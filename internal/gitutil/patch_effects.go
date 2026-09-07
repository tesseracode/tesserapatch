// The one authoritative strict normalized effect grammar
// (GH #15 / ADR-036 D1, PRD-recipe-generation-authority §6.1).
//
// Every production consumer that claims a file path or an effect kind
// derives it from NormalizePatchEffects, either directly or through a
// thin adapter that projects the normalized effect set. Two projections
// ship over this grammar and are deliberately different:
//
//   - FilesInPatchStrict — the B-SIDE path list. Its path/order projection is
//     frozen (PRD §6.1.2, PI-12): `land`, `refresh` and `verify_landed`
//     receive exactly the paths they received before this grammar existed.
//     The shared grammar may add fail-closed malformed/path-safety refusals;
//     widening the returned path set would silently broaden a landed file set
//     and two verify scopes. That freeze is the
//     one reason a compatibility parse mode exists: the frozen list
//     de-duplicates a repeated destination in first-seen order, so it
//     parses with duplicate destinations ALLOWED while every
//     effect-authority caller parses with them REFUSED.
//   - PathsAffectedByPatchStrict — the BOTH-SIDE union, which additionally
//     carries every rename/copy SOURCE. Unapply reverse-applies a patch, so
//     a rename's source must be snapshotted and restored; projecting that
//     scope onto the b-side list would recreate a file at a path nobody
//     snapshotted (PRD §6.1.1).
//
// The grammar refuses rather than degrades. Contradictory headers,
// unsupported quoting, an a-side/b-side mismatch with no rename or copy
// corroboration, and any operand that is not a repo-relative path all
// return an error and a nil result. A fail-soft path list is not effect
// authority.
//
// Record boundaries are the grammar's own recognized record starts, never
// a substring scan (ADR-036 D3). Inside a valid unified-diff hunk every
// body line carries a `+`, `-` or space prefix, so an added line whose
// content is `diff --git a/x b/y` is on the wire as `+diff --git a/x b/y`
// and cannot open a record. A bare line-start token outside a valid hunk
// body is, by definition, not inside one, and the grammar parses it as a
// new record. patch_fragment_sha256 reuses exactly those offsets.

package gitutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ChangeKind is what happened to the path. It is read from the record
// header, which exists whether or not any tree was observed, so it is
// never `unknown` (ADR-036 D3).
type ChangeKind string

const (
	ChangeKindAdd    ChangeKind = "add"
	ChangeKindModify ChangeKind = "modify"
	ChangeKindDelete ChangeKind = "delete"
	ChangeKindRename ChangeKind = "rename"
	ChangeKindCopy   ChangeKind = "copy"
)

// ContentKind is how the content is expressed. `text` is a positive claim
// about bytes, so a producer that read no side may not make it.
type ContentKind string

const (
	ContentKindText    ContentKind = "text"
	ContentKindBinary  ContentKind = "binary"
	ContentKindNone    ContentKind = "none"
	ContentKindUnknown ContentKind = "unknown"
)

// ObjectKind is what the object is. It is selected from one named extant
// side of the immutable observation, and is `unknown` when that side was
// not observed (ADR-036 D3).
type ObjectKind string

const (
	ObjectKindRegular    ObjectKind = "regular"
	ObjectKindExecutable ObjectKind = "executable"
	ObjectKindSymlink    ObjectKind = "symlink"
	ObjectKindGitlink    ObjectKind = "gitlink"
	ObjectKindUnknown    ObjectKind = "unknown"
)

// Git file modes. These are the only values a normalized effect records;
// "" means the side is either proven absent or unobserved, and which of
// the two is read from the side's Observed flag.
const (
	ModeRegular    = "100644"
	ModeExecutable = "100755"
	ModeSymlink    = "120000"
	ModeGitlink    = "160000"
)

// PatchEffect is one normalized file-level effect of a unified diff.
//
// The three kind axes are orthogonal and decided independently: a binary
// rename is rename+binary+regular, a symlink delete is delete+text+symlink,
// an executable rename is rename+text+executable.
//
// A bare parse (NormalizePatchEffects with no observation) fills the axes
// truthfully for a producer that looked at no tree: ChangeKind is definite,
// ContentKind is binary only when a grammar stanza proves it and unknown
// otherwise, ObjectKind is unknown, both modes are "" and both Observed
// flags are false. ResolveEffectObservation upgrades them from observed
// tree/filesystem data; the headers only corroborate.
type PatchEffect struct {
	// Ordinal is the one-based position of this effect's record in the
	// grammar's parse of the patch, counted from the first byte with no
	// gaps.
	Ordinal int

	ChangeKind  ChangeKind
	ContentKind ContentKind
	ObjectKind  ObjectKind

	// Path is the canonical (b-side / destination) repo-relative path.
	Path string
	// OldPath is non-empty exactly when ChangeKind is rename or copy.
	OldPath string

	// OldMode and NewMode come from the immutable observation, not from
	// the headers. "" means proven-absent or unobserved.
	OldMode string
	NewMode string

	// HeaderOldMode and HeaderNewMode are what the record's own headers
	// declared. They corroborate the observation and are never a
	// substitute for it: a producer that read neither side has verified
	// nothing about the object it holds.
	HeaderOldMode string
	HeaderNewMode string

	// BinaryStanza records that the grammar itself proved binary content,
	// through a `GIT binary patch` stanza or a `Binary files ... differ`
	// line. It stands on its own even when no side was observed.
	BinaryStanza bool

	PreimageObserved bool
	PreimagePresent  bool
	PreimageSHA256   string

	PostimageObserved bool
	PostimagePresent  bool
	PostimageSHA256   string

	// FragmentStart and FragmentEnd are the record's exact byte range in
	// the patch: [FragmentStart, FragmentEnd).
	FragmentStart int
	FragmentEnd   int
	// FragmentSHA256 is the SHA-256 of those exact raw bytes. Original
	// line endings, no-newline markers and binary stanzas are retained
	// verbatim; nothing is normalized, trimmed or re-encoded.
	FragmentSHA256 string
}

// ExtantSides reports which sides this effect's ChangeKind requires to
// exist: an add needs its postimage, a delete its preimage, and a modify,
// rename or copy both (ADR-036 D3).
func (e PatchEffect) ExtantSides() (pre, post bool) {
	switch e.ChangeKind {
	case ChangeKindAdd:
		return false, true
	case ChangeKindDelete:
		return true, false
	default:
		return true, true
	}
}

// NormalizePatchEffects parses a unified diff into the ordered normalized
// effect set. It is the single authority for path and effect kind.
//
// Whitespace-only input is not an error: it legitimately touches nothing.
// Non-blank input carrying zero recognized `diff --git` records is
// refused, because an empty scope means "everything" to git.
//
// Arbitrary records that name the SAME canonical destination path are
// refused. Git's adjacent delete+add representation of a real object-type
// transition is the one exception: it is coalesced into one modify effect.
func NormalizePatchEffects(patch string) ([]PatchEffect, error) {
	return normalizePatchEffects(patch, effectParseOptions{})
}

// effectParseOptions carries the ONE deliberate deviation from authority
// parsing. It is unexported: no caller outside this file may choose to
// weaken the grammar.
type effectParseOptions struct {
	// allowDuplicateDestinations preserves the frozen PI-12 projection,
	// which de-duplicates a repeated destination in first-seen order
	// (PRD §6.1.2). It is set by FilesInPatchStrict and by nothing else.
	allowDuplicateDestinations bool
}

// normalizePatchEffects is the shared parse both modes run. Authority mode
// coalesces real typechanges and refuses every other duplicate destination;
// PI-12 compatibility mode preserves the historical first-seen projection.
func normalizePatchEffects(patch string, opts effectParseOptions) ([]PatchEffect, error) {
	lines := splitPatchLines(patch)
	starts := patchRecordStarts(lines)
	if len(starts) == 0 {
		if strings.TrimSpace(patch) != "" {
			return nil, fmt.Errorf("patch contains no `diff --git` header but is not empty (%d byte(s)); refusing to treat it as touching nothing", len(patch))
		}
		return nil, nil
	}

	effects := make([]PatchEffect, 0, len(starts))
	for k, startLine := range starts {
		endLine := len(lines)
		if k+1 < len(starts) {
			endLine = starts[k+1]
		}
		effect, err := normalizeOneRecord(lines, startLine, endLine)
		if err != nil {
			return nil, fmt.Errorf("unparseable diff header %q: %w", lines[startLine].text, err)
		}
		effect.Ordinal = k + 1
		effect.FragmentStart = lines[startLine].start
		effect.FragmentEnd = len(patch)
		if k+1 < len(starts) {
			effect.FragmentEnd = lines[starts[k+1]].start
		}
		sum := sha256.Sum256([]byte(patch[effect.FragmentStart:effect.FragmentEnd]))
		effect.FragmentSHA256 = hex.EncodeToString(sum[:])
		effects = append(effects, effect)
	}
	if opts.allowDuplicateDestinations {
		return effects, nil
	}
	return coalesceTypeChanges(patch, effects)
}

// coalesceTypeChanges folds Git's two-record representation of a file type
// change into one semantic modify effect. Plain `git diff` emits a delete
// record followed immediately by an add record for regular↔symlink and
// file↔gitlink transitions. Every other repeated destination remains a hard
// refusal.
func coalesceTypeChanges(patch string, effects []PatchEffect) ([]PatchEffect, error) {
	out := make([]PatchEffect, 0, len(effects))
	seenDestination := make(map[string]int, len(effects))
	for i := 0; i < len(effects); i++ {
		effect := effects[i]
		originalOrdinal := effect.Ordinal
		if i+1 < len(effects) && isTypeChangePair(effect, effects[i+1]) {
			add := effects[i+1]
			effect.ChangeKind = ChangeKindModify
			effect.HeaderNewMode = add.HeaderNewMode
			effect.BinaryStanza = effect.BinaryStanza || add.BinaryStanza
			if effect.BinaryStanza {
				effect.ContentKind = ContentKindBinary
			}
			effect.FragmentEnd = add.FragmentEnd
			sum := sha256.Sum256([]byte(patch[effect.FragmentStart:effect.FragmentEnd]))
			effect.FragmentSHA256 = hex.EncodeToString(sum[:])
			i++
		}
		if first, duplicate := seenDestination[effect.Path]; duplicate {
			return nil, fmt.Errorf("records %d and %d both describe destination path %q; refusing to collapse two effects on one path",
				first, originalOrdinal, effect.Path)
		}
		seenDestination[effect.Path] = originalOrdinal
		effect.Ordinal = len(out) + 1
		out = append(out, effect)
	}
	return out, nil
}

func isTypeChangePair(deleted, added PatchEffect) bool {
	if deleted.ChangeKind != ChangeKindDelete ||
		added.ChangeKind != ChangeKindAdd ||
		deleted.Path != added.Path {
		return false
	}
	oldKind := ObjectKindForMode(deleted.HeaderOldMode)
	newKind := ObjectKindForMode(added.HeaderNewMode)
	if oldKind == ObjectKindUnknown || newKind == ObjectKindUnknown {
		return false
	}
	// Git treats 100644 and 100755 as the same regular-file object type and
	// emits their transition as one old-mode/new-mode record, never as a
	// delete+add pair.
	oldRegular := oldKind == ObjectKindRegular || oldKind == ObjectKindExecutable
	newRegular := newKind == ObjectKindRegular || newKind == ObjectKindExecutable
	if oldRegular && newRegular {
		return false
	}
	return oldKind != newKind
}

// FilesInPatchStrict returns the b-side path of every file entry in a
// unified diff, decoding Git's C-quoting byte-correctly.
//
// Handled: quoted and unquoted paths, paths containing spaces, tabs,
// newlines and octal-escaped bytes, renames, copies, mode-only entries,
// binary entries, new and deleted files.
//
// Refused (error, nil slice — never a partial or empty scope):
//
//   - non-blank input containing zero recognized `diff --git` records;
//   - a header whose a-side OR b-side operand is malformed;
//   - a quoted operand using an escape Git does not emit;
//   - an operand without a valid `a/` / `b/` prefix, or with an empty
//     path;
//   - contradictory headers, including differing sides with no rename or
//     copy corroboration;
//   - an operand that is not a repo-relative path;
//   - a header that no corroborating line can disambiguate.
//
// Whitespace-only input is NOT an error: it legitimately touches
// nothing. Callers that derive a write scope MUST use this function or
// PathsAffectedByPatchStrict.
//
// PI-12 contract: this projection returns the b-side path ONLY, in
// first-seen order, de-duplicated. Rename and copy SOURCES are not in it
// and may not be added — PathsAffectedByPatchStrict carries the rollback
// scope instead (PRD §6.1.2).
//
// It is also the ONE caller that parses with duplicate destinations
// allowed. Its five shipped consumers received a de-duplicated first-seen
// list before this grammar existed and must keep receiving exactly that;
// refusing here would change a frozen result. Nothing else in the module
// may make that choice — an effect-level consumer that met a duplicate
// destination would have to pick a record, and picking one silently
// discards the other.
func FilesInPatchStrict(patch string) ([]string, error) {
	effects, err := normalizePatchEffects(patch, effectParseOptions{allowDuplicateDestinations: true})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, effect := range effects {
		if effect.Path == "" || seen[effect.Path] {
			continue
		}
		seen[effect.Path] = true
		out = append(out, effect.Path)
	}
	return out, nil
}

// PathsAffectedByPatchStrict returns the sorted, unique union of every
// path a patch touches on either side: each effect's canonical path plus
// its old_path for rename and copy effects. It preserves the rollback
// scope PathsAffectedByPatch provides today and refuses, with an error,
// every input FilesInPatchStrict refuses.
//
// It parses in AUTHORITY mode, so a repeated destination is refused
// rather than collapsed. Nothing in its accepted contract asks for the
// PI-12 de-duplication: its three call sites derive a snapshot, a diff
// scope and a reverse-apply scope, and each of those has to know which
// effect it is undoing. The union it returns is de-duplicated across the
// two SIDES of one effect set, which is a different statement.
//
// Callers derive a snapshot, a diff scope or a reverse-apply scope from
// this, so they MUST fail closed on the error rather than continue with a
// short list.
func PathsAffectedByPatchStrict(patch string) ([]string, error) {
	effects, err := NormalizePatchEffects(patch)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, effect := range effects {
		add(effect.Path)
		switch effect.ChangeKind {
		case ChangeKindRename, ChangeKindCopy:
			add(effect.OldPath)
		}
	}
	sort.Strings(out)
	return out, nil
}

// patchLine is one line of a patch with its exact byte range, so record
// boundaries and fragment digests are computed on raw bytes rather than
// on a re-joined approximation of them.
type patchLine struct {
	start int
	end   int
	text  string
}

func splitPatchLines(patch string) []patchLine {
	var out []patchLine
	for i := 0; i < len(patch); {
		nl := strings.IndexByte(patch[i:], '\n')
		if nl < 0 {
			out = append(out, patchLine{start: i, end: len(patch), text: patch[i:]})
			break
		}
		out = append(out, patchLine{start: i, end: i + nl + 1, text: patch[i : i+nl]})
		i += nl + 1
	}
	return out
}

// patchRecordStarts returns the line indexes the grammar recognizes as
// file-record starts.
//
// The scan tracks hunk-body and binary-stanza state so a `diff --git`
// sequence appearing inside a hunk body or a binary stanza is never
// mistaken for a boundary. This is the structural reason ADR-036 D3 can
// close the embedded-token case instead of merely warning about it.
func patchRecordStarts(lines []patchLine) []int {
	const (
		stateHeader = iota
		stateHunkBody
		stateBinaryStanza
	)
	var starts []int
	state := stateHeader
	oldRemaining, newRemaining := 0, 0

	for idx := 0; idx < len(lines); idx++ {
		text := lines[idx].text
		switch state {
		case stateHunkBody:
			if oldRemaining <= 0 && newRemaining <= 0 {
				// The hunk is complete. Only a trailing
				// `\ No newline at end of file` marker may follow
				// before the next structural line.
				if strings.HasPrefix(text, `\`) {
					continue
				}
				state = stateHeader
				idx--
				continue
			}
			switch {
			case strings.HasPrefix(text, `\`):
				// No-newline marker: annotates the previous line
				// and consumes no budget.
			case strings.HasPrefix(text, "+"):
				newRemaining--
			case strings.HasPrefix(text, "-"):
				oldRemaining--
			case strings.HasPrefix(text, " ") || text == "" || text == "\r":
				oldRemaining--
				newRemaining--
			default:
				// The hunk-line prefix discipline is violated, so
				// this line is not inside a valid hunk body.
				state = stateHeader
				idx--
				continue
			}
		case stateBinaryStanza:
			// A `GIT binary patch` stanza is terminated by a blank
			// line. Its base85 payload lines contain no spaces, so
			// they cannot look like a record header.
			if strings.TrimSpace(text) == "" {
				state = stateHeader
			}
		default:
			switch {
			case strings.HasPrefix(text, "diff --git "):
				starts = append(starts, idx)
			case strings.HasPrefix(text, "@@ "):
				_, oldLen, _, newLen, ok := parseUnifiedHunkRange(text)
				if !ok {
					break
				}
				oldRemaining, newRemaining = oldLen, newLen
				state = stateHunkBody
			case strings.TrimRight(text, "\r") == "GIT binary patch":
				state = stateBinaryStanza
			}
		}
	}
	return starts
}

// parseUnifiedHunkRange reads `@@ -<oldStart>[,<oldLen>] +<newStart>[,<newLen>] @@`.
// It is the grammar's own hunk-budget reader; the workflow hunk-overlap
// projection keeps its separate range parser but no longer reads paths.
func parseUnifiedHunkRange(header string) (oldStart, oldLen, newStart, newLen int, ok bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[1], "-") || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, 0, 0, false
	}
	oldStart, oldLen, ok1 := parseHunkRangeToken(strings.TrimPrefix(fields[1], "-"))
	newStart, newLen, ok2 := parseHunkRangeToken(strings.TrimPrefix(fields[2], "+"))
	return oldStart, oldLen, newStart, newLen, ok1 && ok2
}

func parseHunkRangeToken(token string) (int, int, bool) {
	parts := strings.SplitN(token, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	length := 1
	if len(parts) == 2 {
		length, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}
	return start, length, true
}

// recordHeaders is the corroborating metadata of one file record.
type recordHeaders struct {
	newFileMode     string
	deletedFileMode string
	oldMode         string
	newMode         string
	indexMode       string

	renameFrom string
	renameTo   string
	hasRename  bool
	copyFrom   string
	copyTo     string
	hasCopy    bool

	minusPath string
	plusPath  string
	hasMinus  bool
	hasPlus   bool

	binary bool
}

// scanRecordHeaders collects the metadata lines between a record start and
// its first hunk. Everything the change axis needs is declared there.
func scanRecordHeaders(lines []patchLine, start, end int) recordHeaders {
	var h recordHeaders
	for j := start + 1; j < end; j++ {
		text := strings.TrimRight(lines[j].text, "\r")
		if strings.HasPrefix(text, "@@") {
			break
		}
		switch {
		case strings.HasPrefix(text, "new file mode "):
			h.newFileMode = strings.TrimSpace(strings.TrimPrefix(text, "new file mode "))
		case strings.HasPrefix(text, "deleted file mode "):
			h.deletedFileMode = strings.TrimSpace(strings.TrimPrefix(text, "deleted file mode "))
		case strings.HasPrefix(text, "old mode "):
			h.oldMode = strings.TrimSpace(strings.TrimPrefix(text, "old mode "))
		case strings.HasPrefix(text, "new mode "):
			h.newMode = strings.TrimSpace(strings.TrimPrefix(text, "new mode "))
		case strings.HasPrefix(text, "rename from "):
			h.renameFrom = strings.TrimPrefix(text, "rename from ")
			h.hasRename = true
		case strings.HasPrefix(text, "rename to "):
			h.renameTo = strings.TrimPrefix(text, "rename to ")
			h.hasRename = true
		case strings.HasPrefix(text, "copy from "):
			h.copyFrom = strings.TrimPrefix(text, "copy from ")
			h.hasCopy = true
		case strings.HasPrefix(text, "copy to "):
			h.copyTo = strings.TrimPrefix(text, "copy to ")
			h.hasCopy = true
		case strings.HasPrefix(text, "index "):
			h.indexMode = indexLineMode(text)
		case strings.HasPrefix(text, "--- "):
			h.minusPath = strings.TrimPrefix(text, "--- ")
			h.hasMinus = true
		case strings.HasPrefix(text, "+++ "):
			h.plusPath = strings.TrimPrefix(text, "+++ ")
			h.hasPlus = true
		case text == "GIT binary patch":
			h.binary = true
		case strings.HasPrefix(text, "Binary files ") && strings.HasSuffix(text, " differ"):
			h.binary = true
		}
	}
	return h
}

// indexLineMode reads the trailing mode of `index <old>..<new> <mode>`.
// Git writes it only when the mode is unchanged, which is exactly when no
// `old mode`/`new mode` pair is present.
func indexLineMode(text string) string {
	fields := strings.Fields(text)
	if len(fields) < 3 {
		return ""
	}
	return fields[len(fields)-1]
}

// normalizeOneRecord turns one recognized record into a normalized effect.
// It refuses rather than guessing at every shape whose interpretation is
// not complete.
func normalizeOneRecord(lines []patchLine, start, end int) (PatchEffect, error) {
	header := strings.TrimPrefix(strings.TrimRight(lines[start].text, "\r"), "diff --git ")
	h := scanRecordHeaders(lines, start, end)

	change, err := recordChangeKind(h)
	if err != nil {
		return PatchEffect{}, err
	}
	if err := checkDevNullCorroboration(change, h); err != nil {
		return PatchEffect{}, err
	}

	path, oldPath, err := resolveRecordPaths(header, change, h)
	if err != nil {
		return PatchEffect{}, err
	}
	if err := ensureRepoRelativePatchPath(path); err != nil {
		return PatchEffect{}, fmt.Errorf("b-side path %q: %w", path, err)
	}
	if oldPath != "" {
		if err := ensureRepoRelativePatchPath(oldPath); err != nil {
			return PatchEffect{}, fmt.Errorf("a-side path %q: %w", oldPath, err)
		}
	}

	headerOld, headerNew := recordHeaderModes(change, h)
	if err := validateHeaderMode(headerOld); err != nil {
		return PatchEffect{}, err
	}
	if err := validateHeaderMode(headerNew); err != nil {
		return PatchEffect{}, err
	}

	effect := PatchEffect{
		ChangeKind:    change,
		ContentKind:   ContentKindUnknown,
		ObjectKind:    ObjectKindUnknown,
		Path:          path,
		OldPath:       oldPath,
		HeaderOldMode: headerOld,
		HeaderNewMode: headerNew,
		BinaryStanza:  h.binary,
	}
	if h.binary {
		// A stanza marker is positive evidence and stands on its own,
		// so this branch is reachable even when no side was observed.
		effect.ContentKind = ContentKindBinary
	}
	return effect, nil
}

func recordChangeKind(h recordHeaders) (ChangeKind, error) {
	switch {
	case h.newFileMode != "" && h.deletedFileMode != "":
		return "", fmt.Errorf("header declares both a new file mode and a deleted file mode")
	case h.hasRename && h.hasCopy:
		return "", fmt.Errorf("header declares both a rename and a copy")
	case h.newFileMode != "":
		if h.hasRename || h.hasCopy {
			return "", fmt.Errorf("header declares a new file alongside a rename or copy")
		}
		return ChangeKindAdd, nil
	case h.deletedFileMode != "":
		if h.hasRename || h.hasCopy {
			return "", fmt.Errorf("header declares a deleted file alongside a rename or copy")
		}
		return ChangeKindDelete, nil
	case h.hasRename:
		if h.renameFrom == "" || h.renameTo == "" {
			return "", fmt.Errorf("rename is missing its `rename from` or `rename to` operand")
		}
		return ChangeKindRename, nil
	case h.hasCopy:
		if h.copyFrom == "" || h.copyTo == "" {
			return "", fmt.Errorf("copy is missing its `copy from` or `copy to` operand")
		}
		return ChangeKindCopy, nil
	default:
		return ChangeKindModify, nil
	}
}

// checkDevNullCorroboration refuses a `---`/`+++` pair that contradicts the
// change axis the same record declares.
func checkDevNullCorroboration(change ChangeKind, h recordHeaders) error {
	minusIsNull := h.hasMinus && isDevNullOperand(h.minusPath)
	plusIsNull := h.hasPlus && isDevNullOperand(h.plusPath)
	switch change {
	case ChangeKindAdd:
		if h.hasMinus && !minusIsNull {
			return fmt.Errorf("a new file declares a preimage side %q", h.minusPath)
		}
		if plusIsNull {
			return fmt.Errorf("a new file declares a /dev/null postimage side")
		}
	case ChangeKindDelete:
		if h.hasPlus && !plusIsNull {
			return fmt.Errorf("a deleted file declares a postimage side %q", h.plusPath)
		}
		if minusIsNull {
			return fmt.Errorf("a deleted file declares a /dev/null preimage side")
		}
	default:
		if minusIsNull || plusIsNull {
			return fmt.Errorf("a %s effect declares a /dev/null side", change)
		}
	}
	return nil
}

func isDevNullOperand(field string) bool {
	return strings.TrimSpace(stripDiffTimestamp(field)) == "/dev/null"
}

// resolveRecordPaths determines the canonical path and, for rename and
// copy effects, the old path. Both operands are validated before either
// is used; the corroborating rename/copy lines are consulted only for the
// shapes the two-operand header cannot decide on its own.
func resolveRecordPaths(header string, change ChangeKind, h recordHeaders) (path, oldPath string, err error) {
	a, b, ok, perr := parseDiffGitOperands(header)
	if perr != nil {
		return "", "", perr
	}

	switch change {
	case ChangeKindRename, ChangeKindCopy:
		from, to := h.renameFrom, h.renameTo
		if change == ChangeKindCopy {
			from, to = h.copyFrom, h.copyTo
		}
		oldPath, err = decodeDiffOperandField(from)
		if err != nil {
			return "", "", fmt.Errorf("%s source operand: %w", change, err)
		}
		path, err = decodeDiffOperandField(to)
		if err != nil {
			return "", "", fmt.Errorf("%s destination operand: %w", change, err)
		}
		if ok && (a.Path != oldPath || b.Path != path) {
			return "", "", fmt.Errorf("header operands %q/%q contradict the %s operands %q/%q",
				a.Path, b.Path, change, oldPath, path)
		}
		if path == "" || oldPath == "" {
			return "", "", fmt.Errorf("%s operand resolves to an empty path", change)
		}
		return path, oldPath, nil
	}

	if !ok {
		// An ambiguous unquoted header with no rename or copy
		// corroboration cannot be interpreted completely. Falling back
		// to `+++` here would mint a change kind the record never
		// declared.
		return "", "", fmt.Errorf("no unambiguous path in the header or its corroborating lines")
	}
	if a.Path != b.Path {
		return "", "", fmt.Errorf("header sides %q and %q differ with no rename or copy corroboration", a.Path, b.Path)
	}
	return b.Path, "", nil
}

// recordHeaderModes reports the modes the record's own headers declared.
// They corroborate the immutable observation and never replace it.
func recordHeaderModes(change ChangeKind, h recordHeaders) (oldMode, newMode string) {
	switch change {
	case ChangeKindAdd:
		return "", h.newFileMode
	case ChangeKindDelete:
		return h.deletedFileMode, ""
	default:
		oldMode, newMode = h.oldMode, h.newMode
		if oldMode == "" {
			oldMode = h.indexMode
		}
		if newMode == "" {
			newMode = h.indexMode
		}
		return oldMode, newMode
	}
}

// validateHeaderMode refuses a mode field that is not six octal digits.
// Values outside the permitted set classify as ObjectKindUnknown rather
// than refusing, because the observation — not the header — is the mode
// authority.
func validateHeaderMode(mode string) error {
	if mode == "" {
		return nil
	}
	if len(mode) != 6 {
		return fmt.Errorf("file mode %q is not six octal digits", mode)
	}
	for i := 0; i < len(mode); i++ {
		if mode[i] < '0' || mode[i] > '7' {
			return fmt.Errorf("file mode %q is not six octal digits", mode)
		}
	}
	return nil
}

// ObjectKindForMode classifies a Git file mode. An unrecognized or empty
// mode is `unknown`, never a guess.
func ObjectKindForMode(mode string) ObjectKind {
	switch mode {
	case ModeRegular:
		return ObjectKindRegular
	case ModeExecutable:
		return ObjectKindExecutable
	case ModeSymlink:
		return ObjectKindSymlink
	case ModeGitlink:
		return ObjectKindGitlink
	default:
		return ObjectKindUnknown
	}
}

// ensureRepoRelativePatchPath refuses an operand that is not a
// repo-relative path. Path escape is decided by the grammar so that no
// downstream consumer has to re-derive it before a snapshot or a write.
func ensureRepoRelativePatchPath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if strings.ContainsRune(p, 0) {
		return fmt.Errorf("path contains a NUL byte")
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) {
		return fmt.Errorf("path is absolute")
	}
	// Only the `C:/` / `C:\` shape is drive-rooted. A bare colon is a
	// legal byte in a POSIX filename, so testing for one alone would
	// refuse paths Git happily records.
	if len(p) >= 3 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') && isASCIILetter(p[0]) {
		return fmt.Errorf("path is drive-rooted")
	}
	for _, segment := range strings.Split(p, "/") {
		switch segment {
		case "":
			return fmt.Errorf("path contains an empty segment")
		case ".", "..":
			return fmt.Errorf("path contains a %q segment and escapes the repository", segment)
		}
	}
	return nil
}

func isASCIILetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
