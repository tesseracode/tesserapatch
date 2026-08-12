package gitutil

// Landing-evidence primitives — v0.15.1 Wave C / GH #8.
//
// ADR-013 Amendment 1 D10/D11/D12/D14/D16/D17/D18 and
// PRD-verify-freshness §3.6.2/§3.6.5/§3.6.8/§3.6.9 specify a git-side
// reader that this file implements. Policy stays in
// internal/workflow/verify*.go per ADR-013 D7; everything here is a
// mechanical git primitive:
//
//   - the ≥2.36 version floor probe (the only command issued below it),
//   - the three-call repository preflight (object format, shallowness,
//     promisor configuration),
//   - ONE `git log --topo-order --reverse -z` enumeration returning raw
//     bodies AND parsed trailers (`rev-list` cannot emit `%B`),
//   - an index-isolated temp-index prober for `read-tree` + `apply
//     --check --cached`,
//   - the normalized change identity (`--unified=0`, `^index ` dropped,
//     hunk headers rewritten to a bare `@@`).
//
// Offline discipline (D11/D16, measured as E47): EVERY git invocation in
// this file carries `GIT_NO_LAZY_FETCH=1`, so a missing promisor object
// fails locally and immediately instead of reaching the network.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// NoLazyFetchEnv is the mandatory offline-discipline environment entry
// (ADR-013 D11; git ≥ 2.36). Exported so callers/tests can assert it.
const NoLazyFetchEnv = "GIT_NO_LAZY_FETCH=1"

// Git floor for the landed-evidence contract (ADR-013 D17). The floor is
// set by the strictest MANDATORY capability, `GIT_NO_LAZY_FETCH` (2.36),
// not by the trailer-format capabilities (2.22/2.25/2.29) which are
// recorded as historical component facts.
const (
	GitFloorMajor = 2
	GitFloorMinor = 36
)

// CLocaleEnv is the mandatory deterministic-diagnostics entry for every
// evidence command (v0.15.1 Wave C rev-3, adjudication finding P1).
//
// Rev-2 forced `LC_ALL=C` only on the `-C0` ladder step, so every OTHER
// classified probe — `read-tree`, `apply`, `diff`, `cat-file`, `log` —
// inherited the ambient locale. A translated diagnostic then fails the
// missing-object test and degrades a genuine `history-incomplete` (R22)
// into `unavailable` (R10), and makes the malformed-patch grammar
// unmatchable. Classification is only sound if the text is fixed.
const CLocaleEnv = "LC_ALL=C"

// evidenceEnv returns the environment for every git command issued by
// this file. `GIT_NO_LAZY_FETCH=1` and `LC_ALL=C` are appended LAST, in
// that order, so they win over both the inherited environment and any
// caller-supplied extra.
func evidenceEnv(extra ...string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, extra...)
	env = append(env, NoLazyFetchEnv, CLocaleEnv)
	return env
}

// evidenceCmd builds a git command rooted at dir with the offline
// environment applied.
func evidenceCmd(dir string, extraEnv []string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = evidenceEnv(extraEnv...)
	return cmd
}

// runEvidenceGit runs a git command and returns stdout, stderr and the
// error. stderr is returned verbatim so callers can classify missing
// promisor objects (D16).
func runEvidenceGit(dir string, extraEnv []string, args ...string) (string, string, error) {
	cmd := evidenceCmd(dir, extraEnv, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// ── Git version floor ────────────────────────────────────────────────────

// GitVersion is a parsed `git --version` result.
type GitVersion struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

// AtLeast reports whether v is at least major.minor.
func (v GitVersion) AtLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

// String renders the parsed version for diagnostics.
func (v GitVersion) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

var gitVersionRe = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// ReadGitVersion probes `git --version`. This is the ONLY git command a
// run may issue before the floor check succeeds (ADR-013 D17).
func ReadGitVersion(repoRoot string) (GitVersion, error) {
	out, stderr, err := runEvidenceGit(repoRoot, nil, "--version")
	if err != nil {
		return GitVersion{}, fmt.Errorf("git --version: %v: %s", err, strings.TrimSpace(stderr))
	}
	raw := strings.TrimSpace(out)
	m := gitVersionRe.FindStringSubmatch(raw)
	if m == nil {
		return GitVersion{Raw: raw}, fmt.Errorf("git --version: unparsable output %q", raw)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	return GitVersion{Major: major, Minor: minor, Patch: patch, Raw: raw}, nil
}

// ── Repository preflight ─────────────────────────────────────────────────

// RepoFacts carries the once-per-run preflight facts of ADR-013 D16.
// It is built by EXACTLY three git invocations and is consumed BEFORE
// any parent-count or topology classification, because a shallow
// boundary and a true root are indistinguishable by `%P` alone (E38).
type RepoFacts struct {
	ObjectFormat    string          // "sha1" | "sha256"
	CommitIDHexLen  int             // 40 for sha1, 64 for sha256 — derived, never hardcoded
	Shallow         bool            // `git rev-parse --is-shallow-repository`
	PartialClone    bool            // remote.<name>.promisor / partialclonefilter set
	ShallowBoundary map[string]bool // commit ids listed in .git/shallow
	GitDir          string          // absolute `git rev-parse --git-dir`
}

// ReadRepoFacts performs the D16 preflight. Three git calls:
// `rev-parse --show-object-format`, `rev-parse --is-shallow-repository`,
// and `config --get-regexp` for the promisor keys. The `.git/shallow`
// membership set is read from the filesystem (no git process).
func ReadRepoFacts(repoRoot string) (RepoFacts, error) {
	facts := RepoFacts{ShallowBoundary: map[string]bool{}}

	// One `rev-parse` answers all three structural questions, so the
	// preflight stays inside its documented budget while still probing
	// the git dir the isolated index lives under (E24).
	out, stderr, err := runEvidenceGit(repoRoot, nil, "rev-parse",
		"--path-format=absolute", "--show-object-format", "--is-shallow-repository", "--git-dir")
	if err != nil {
		return facts, fmt.Errorf("git rev-parse (preflight): %v: %s", err, strings.TrimSpace(stderr))
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		return facts, fmt.Errorf("git rev-parse (preflight): unparsable output %q", strings.TrimSpace(out))
	}
	facts.ObjectFormat = strings.TrimSpace(lines[0])
	switch facts.ObjectFormat {
	case "sha1":
		facts.CommitIDHexLen = 40
	case "sha256":
		facts.CommitIDHexLen = 64
	default:
		return facts, fmt.Errorf("git rev-parse --show-object-format: unknown object format %q", facts.ObjectFormat)
	}
	facts.Shallow = strings.TrimSpace(lines[1]) == "true"
	facts.GitDir = strings.TrimSpace(lines[2])

	// `config --get-regexp` exits 1 with no output when nothing matches;
	// that is "not a partial clone", not an error.
	promisor, _, cfgErr := runEvidenceGit(repoRoot, nil, "config", "--get-regexp", `^remote\..*\.(promisor|partialclonefilter)$`)
	if cfgErr == nil && strings.TrimSpace(promisor) != "" {
		facts.PartialClone = true
	}

	// Shallow-boundary membership: read `<git-dir>/shallow` when present.
	// A worktree's `.git` may be a file pointing at the real git dir, so
	// resolve through `--git-common-dir` only when the cheap path misses.
	for _, candidate := range shallowFileCandidates(repoRoot) {
		data, readErr := os.ReadFile(candidate)
		if readErr != nil {
			continue
		}
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				facts.ShallowBoundary[line] = true
			}
		}
		break
	}

	return facts, nil
}

func shallowFileCandidates(repoRoot string) []string {
	out := []string{filepath.Join(repoRoot, ".git", "shallow")}
	_ = out
	// `.git` as a file (linked worktree / submodule) — follow the gitdir
	// pointer without spawning git.
	if data, err := os.ReadFile(filepath.Join(repoRoot, ".git")); err == nil {
		line := strings.TrimSpace(string(data))
		if strings.HasPrefix(line, "gitdir:") {
			dir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(repoRoot, dir)
			}
			out = append(out, filepath.Join(dir, "shallow"))
			// linked worktrees keep `shallow` in the common dir
			out = append(out, filepath.Join(filepath.Dir(filepath.Dir(dir)), "shallow"))
		}
	}
	return out
}

// ── Commit enumeration ───────────────────────────────────────────────────

// Trailer keys read by the landing-evidence contract (ADR-013 D10).
// Git matches trailer keys case-insensitively (E18); the constants are
// the canonical spellings `land` emits.
const (
	TrailerFeature    = "Tpatch-Feature"
	TrailerPatchSHA   = "Tpatch-Patch-SHA"
	TrailerRecipeSHA  = "Tpatch-Recipe-SHA"
	TrailerBaseCommit = "Tpatch-Base-Commit"
)

// trailerValueSeparator is the in-field separator requested from git for
// multi-valued trailers. RS (0x1e) cannot appear in a trailer value that
// git itself parsed from a commit message line.
const trailerValueSeparator = "\x1e"

// fieldSeparator (US, 0x1f) separates the record's fields; records are
// NUL-terminated by `-z`.
const fieldSeparator = "\x1f"

// CommitRecord is one commit from the single enumeration: raw body AND
// parsed terminal-trailer values, so the D10 conservative raw precedence
// rule can be applied (a raw `Tpatch-Feature: <slug>` line whose parsed
// block does not yield the slug is `malformed`, never `none`).
type CommitRecord struct {
	SHA      string
	Parents  []string
	RawBody  string
	Trailers map[string][]string
}

// ParentCount is the `%P` cardinality (E33: 0 for a root or a shallow
// graft boundary, ≥2 for a merge).
func (c CommitRecord) ParentCount() int { return len(c.Parents) }

// TrailerValues returns the parsed values for key (case-insensitive).
func (c CommitRecord) TrailerValues(key string) []string {
	if c.Trailers == nil {
		return nil
	}
	return c.Trailers[strings.ToLower(key)]
}

// EnumerateCommitTrailers runs the ONE enumeration of ADR-013 D10/D17:
//
//	git log --topo-order --reverse -z --format='%H%x1f%P%x1f<4 trailers>%x1f%B'
//
// over commits reachable from the resolved HEAD. Records arrive
// oldest-first. `rev-list` is deliberately NOT used — it cannot emit
// `%B`. `--first-parent` is never used (E9): a landing reachable only
// through a merge's non-first parent must still be found.
func EnumerateCommitTrailers(repoRoot string) ([]CommitRecord, error) {
	format := strings.Join([]string{
		"%H",
		"%P",
		trailerFormat(TrailerFeature),
		trailerFormat(TrailerPatchSHA),
		trailerFormat(TrailerRecipeSHA),
		trailerFormat(TrailerBaseCommit),
		"%B",
	}, "%x1f")

	stdout, stderr, err := runEvidenceGit(repoRoot, nil,
		"log", "--topo-order", "--reverse", "-z", "--format="+format)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		// An empty repository (no commits yet) is not a reader failure.
		if strings.Contains(msg, "does not have any commits yet") ||
			strings.Contains(msg, "unknown revision or path not in the working tree") ||
			strings.Contains(msg, "bad default revision") {
			return nil, nil
		}
		return nil, fmt.Errorf("git log enumeration: %v: %s", err, msg)
	}

	keys := []string{TrailerFeature, TrailerPatchSHA, TrailerRecipeSHA, TrailerBaseCommit}
	var out []CommitRecord
	for _, raw := range strings.Split(stdout, "\x00") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		fields := strings.Split(raw, fieldSeparator)
		if len(fields) < 7 {
			return nil, fmt.Errorf("git log enumeration: malformed record with %d fields", len(fields))
		}
		rec := CommitRecord{
			SHA:      strings.TrimSpace(fields[0]),
			RawBody:  strings.Join(fields[6:], fieldSeparator),
			Trailers: map[string][]string{},
		}
		if p := strings.Fields(fields[1]); len(p) > 0 {
			rec.Parents = p
		}
		for i, key := range keys {
			field := fields[2+i]
			var values []string
			for _, v := range strings.Split(field, trailerValueSeparator) {
				v = strings.Trim(v, " \t\r\n")
				if v == "" {
					continue
				}
				values = append(values, v)
			}
			rec.Trailers[strings.ToLower(key)] = values
		}
		out = append(out, rec)
	}
	return out, nil
}

func trailerFormat(key string) string {
	// `separator=%x1e` is honoured by git ≥ 2.25; the floor is 2.36.
	return fmt.Sprintf("%%(trailers:key=%s,valueonly,separator=%%x1e)", key)
}

// RawBodyHasTrailerLine reports whether the commit's RAW message carries
// a line exactly `<key>: <value>` after trimming trailing ASCII space and
// tab. This is the D10 conservative raw-precedence probe: a match here
// with no matching parsed value means `malformed`, never `none`.
//
// Key comparison is case-insensitive, mirroring git's own trailer key
// matching (E18).
func RawBodyHasTrailerLine(body, key, value string) bool {
	wantKey := strings.ToLower(key)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, " \t\r")
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(line[:idx])) != wantKey {
			continue
		}
		if strings.Trim(line[idx+1:], " \t") == value {
			return true
		}
	}
	return false
}

// ── Index-isolated probing ───────────────────────────────────────────────

// TempIndex is an isolated git index used to probe arbitrary trees
// without touching the real index or the working tree (ADR-013 D11,
// measured as E23/E24/E25).
type TempIndex struct {
	repoRoot string
	path     string
}

// NewTempIndex allocates an isolated index file under dir, which MUST be
// outside the tracked working tree. Callers pass the gitignored
// `.tpatch/local/` root (or the git dir).
func NewTempIndex(repoRoot, dir string) (*TempIndex, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("temp index: mkdir %s: %w", dir, err)
	}
	f, err := os.CreateTemp(dir, "verify-index-*")
	if err != nil {
		return nil, fmt.Errorf("temp index: create: %w", err)
	}
	path := f.Name()
	_ = f.Close()
	// git refuses to read a zero-byte index file; remove it and let
	// `read-tree` create it fresh at the reserved name.
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("temp index: reserve %s: %w", path, err)
	}
	return &TempIndex{repoRoot: repoRoot, path: path}, nil
}

// Path is the absolute path of the isolated index file.
func (t *TempIndex) Path() string { return t.path }

// Env is the `GIT_INDEX_FILE` entry callers add to a git invocation.
func (t *TempIndex) Env() []string { return []string{"GIT_INDEX_FILE=" + t.path} }

// Close removes the isolated index. Safe to call more than once; callers
// must defer it on EVERY exit path (ADR-013 D11 step 4).
func (t *TempIndex) Close() error {
	if t == nil || t.path == "" {
		return nil
	}
	err := os.Remove(t.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// `git apply --cached` may leave a lock file behind on failure.
	_ = os.Remove(t.path + ".lock")
	return nil
}

// ReadTree seeds the isolated index from a commit-ish or tree-ish.
// Valid revision forms include `HEAD`, `<sha>`, `<sha>^` and
// `<sha>^^{tree}`; `<sha>^{tree}^` is INVALID and is never used (E43).
func (t *TempIndex) ReadTree(rev string) error {
	_, stderr, err := runEvidenceGit(t.repoRoot, t.Env(), "read-tree", rev)
	if err != nil {
		return fmt.Errorf("git read-tree %s: %v: %s", rev, err, strings.TrimSpace(stderr))
	}
	return nil
}

// ApplyCheckOptions configures one `git apply --check --cached` probe.
type ApplyCheckOptions struct {
	PatchPath string
	Reverse   bool
	// Context, when non-nil, is passed as `-C<n>`. nil means git's
	// default context.
	Context *int
	// Verbose adds `--verbose`, which is what surfaces the
	// `Context reduced to (n/m)` lines the D12 ladder counts.
	Verbose bool
}

// ApplyCheckResult is one probe outcome.
//
// ExitCode distinguishes an ANSWER from a FAILURE: `git apply --check`
// exits 1 when the patch legitimately does not apply, and 128 (or
// anything else) when git could not carry the probe out at all — a
// missing object, a broken repository, a usage error. Rev-0 collapsed
// both into `OK == false`, so an unrunnable probe was reported as "the
// content is absent" / "no candidate qualified" (v0.15.1 rev-1
// adjudication finding 3).
type ApplyCheckResult struct {
	OK               bool
	ExitCode         int
	Stderr           string
	ZeroContextHunks int
	Args             []string
}

// ApplyAnswered reports whether the probe produced a patch-level verdict
// rather than an execution failure.
func (r ApplyCheckResult) ApplyAnswered() bool {
	return ApplyProbeAnswered(r.ExitCode, r.OK, r.Stderr)
}

var zeroContextRe = regexp.MustCompile(`Context reduced to \(0/0\)`)

// ApplyCheck runs `git apply --check [--reverse] --cached [-C<n>]
// [--verbose] <patch>` against the isolated index. It NEVER touches the
// working tree or the real index.
func (t *TempIndex) ApplyCheck(opts ApplyCheckOptions) ApplyCheckResult {
	args := []string{"apply", "--check", "--cached"}
	if opts.Reverse {
		args = append(args, "--reverse")
	}
	if opts.Context != nil {
		args = append(args, fmt.Sprintf("-C%d", *opts.Context))
	}
	if opts.Verbose {
		args = append(args, "--verbose")
	}
	args = append(args, opts.PatchPath)

	// `LC_ALL=C` is applied unconditionally by evidenceEnv, so every
	// probe — not just the `-C0` step — has matchable diagnostics.
	_, stderr, err := runEvidenceGit(t.repoRoot, t.Env(), args...)
	res := ApplyCheckResult{
		OK:               err == nil,
		Stderr:           stderr,
		ZeroContextHunks: len(zeroContextRe.FindAllString(stderr, -1)),
		Args:             args,
	}
	if err != nil {
		res.ExitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		}
	}
	return res
}

// IntPtr is a small helper for ApplyCheckOptions.Context.
func IntPtr(n int) *int { return &n }

// ── Missing-object classification ────────────────────────────────────────

// missingObjectPatterns are the LOCAL failure forms git produces under
// `GIT_NO_LAZY_FETCH=1` when an object is genuinely absent (E47). The
// network form (`does not appear to be a git repository` /
// `Could not read from remote repository`) is deliberately NOT in this
// set: seeing it means the offline discipline was violated.
var missingObjectPatterns = []string{
	"not a valid object name",
	"unable to read",
	"could not read object",
	"object file",
	"missing blob",
	"missing object",
	"bad object",
	"promisor",
	"unable to parse object",
	"fatal: failed to read object",
	// `git apply --cached` reports a blob it cannot read as
	// `error: failed to read <path>`; a genuine content mismatch is
	// reported as `patch does not apply` instead, so the two are
	// distinguishable.
	"failed to read ",
}

// IsMissingObjectError reports whether stderr names a locally-missing
// object. Used to classify `history-incomplete` (ADR-013 D16).
func IsMissingObjectError(stderr string) bool {
	low := strings.ToLower(stderr)
	for _, p := range missingObjectPatterns {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

// ── Apply-probe answer classification (rev-3) ────────────────────────────
//
// `git apply --check` distinguishes three outcomes:
//
//   - exit 0  — the patch applies. An ANSWER.
//   - exit 1  — the patch does not apply. Every ordinary conflict lands
//     here: measured under `LC_ALL=C`, `already exists in index`,
//     `does not exist in index`, `patch does not apply` and their
//     working-tree equivalents ALL exit 1. An ANSWER, decided by the
//     exit code alone — no stderr grammar is consulted.
//   - exit 128 — git refused the input. This is the ONLY exit where the
//     diagnostic matters, and it is admitted as an answer only when the
//     whole diagnostic is a malformed-PATCH complaint.
//
// Rev-2 accepted any non-answer exit whose stderr merely CONTAINED a
// broad fragment such as `already exists`, `new file` or `deleted
// file`. A wrapper failure, a signalled process or a translated
// diagnostic could therefore be promoted to a patch answer (R5/R11) —
// the rev-3 P1 finding. The grammar below is anchored, C-locale only
// (guaranteed by CLocaleEnv), and every non-empty line must match.

// malformedPatchLineRes are the anchored `git apply` diagnostics that
// describe MALFORMED PATCH INPUT. Measured on git 2.55.0 under
// `LC_ALL=C`:
//
//	empty / non-diff input     → error: No valid patches in input (allow with "--allow-empty")
//	truncated or garbage hunk  → error: corrupt patch at ../p.patch:5
//	fragment with no header    → error: patch fragment without header at ../p.patch:1: @@ -1,3 +1,3 @@
//	corrupt binary payload     → error: corrupt binary patch at ../p.patch:6: NOTBASE85
//	                             error: No valid patches in input (allow with "--allow-empty")
//
// `corrupt patch at line N` and `patch with only garbage at line N` are
// the same message family emitted by other git versions; both are
// anchored here so an older git is classified identically.
var malformedPatchLineRes = []*regexp.Regexp{
	regexp.MustCompile(`^error: No valid patches in input \(allow with "--allow-empty"\)$`),
	regexp.MustCompile(`^error: No valid patches in input$`),
	regexp.MustCompile(`^error: corrupt patch at line [0-9]+$`),
	regexp.MustCompile(`^error: corrupt patch at .+:[0-9]+$`),
	regexp.MustCompile(`^error: patch with only garbage at line [0-9]+$`),
	regexp.MustCompile(`^error: patch fragment without header at .+:[0-9]+: .*$`),
	regexp.MustCompile(`^error: patch fragment without header at line [0-9]+: .*$`),
	regexp.MustCompile(`^error: corrupt binary patch at .+:[0-9]+: .*$`),
	regexp.MustCompile(`^error: corrupt binary patch at line [0-9]+: .*$`),
}

// applyInformationalLineRes are non-diagnostic lines `git apply
// --verbose` prints. They never carry a verdict, so they may accompany a
// malformed-patch complaint but can never satisfy the "at least one
// recognised diagnostic" requirement on their own.
var applyInformationalLineRes = []*regexp.Regexp{
	regexp.MustCompile(`^Checking patch .*\.\.\.$`),
}

// IsMalformedPatchDiagnostic reports whether stderr is WHOLLY a
// malformed-patch complaint: at least one recognised diagnostic line,
// and no unrecognised non-empty line. A `fatal:` line, a missing-object
// line, a wrapper's own message or anything unknown makes the whole
// diagnostic unrecognised — mixed output is never an answer.
func IsMalformedPatchDiagnostic(stderr string) bool {
	diagnostics := 0
	for _, raw := range strings.Split(stderr, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if matchesAny(line, malformedPatchLineRes) {
			diagnostics++
			continue
		}
		if matchesAny(line, applyInformationalLineRes) {
			continue
		}
		return false
	}
	return diagnostics > 0
}

func matchesAny(line string, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// ApplyProbeAnswered decides whether an apply probe produced a
// domain-level verdict rather than an execution failure.
//
// Exactly one exit code other than 0 and 1 can be an answer — 128, and
// only when the diagnostic is wholly a malformed-patch complaint that is
// neither a missing-object nor a network failure. Every other exit
// (negative for a signalled or unstartable process, 2, 126, 127, 129+)
// is a FAILURE regardless of what it printed: a wrapper that echoes a
// git-looking line must never be promoted to a patch answer.
func ApplyProbeAnswered(exitCode int, ok bool, stderr string) bool {
	if ok {
		return true
	}
	if exitCode == 1 {
		return true
	}
	if exitCode != 128 {
		return false
	}
	if IsMissingObjectError(stderr) || IsNetworkFetchError(stderr) {
		return false
	}
	return IsMalformedPatchDiagnostic(stderr)
}

// IsNetworkFetchError reports whether stderr looks like git reached (or
// tried to reach) the network. Any occurrence is a contract violation of
// the offline discipline and is surfaced rather than swallowed.
func IsNetworkFetchError(stderr string) bool {
	low := strings.ToLower(stderr)
	return strings.Contains(low, "does not appear to be a git repository") ||
		strings.Contains(low, "could not read from remote repository") ||
		strings.Contains(low, "unable to access")
}

// ── Normalized change identity ───────────────────────────────────────────

var hunkHeaderRe = regexp.MustCompile(`^@@ -\S+(?: \+\S+)? @@.*$`)

// NormalizedChangeIdentity computes the D18 identity of commit over the
// declared path set:
//
//	git diff --no-color --no-ext-diff --no-textconv --binary \
//	    --no-renames --unified=0 <C>^ <C> -- <paths...>
//
// post-processed by exactly two rules — drop every line beginning
// `index `, and rewrite every hunk header to the bare token `@@` — then
// SHA-256 over the remainder.
//
// An empty path set is a caller error: candidates are not comparable and
// the caller must classify `ambiguous` (D18).
func NormalizedChangeIdentity(repoRoot, commit string, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("normalized identity: empty path set")
	}
	args := []string{
		"diff", "--no-color", "--no-ext-diff", "--no-textconv",
		"--binary", "--no-renames", "--unified=0",
		commit + "^", commit, "--",
	}
	args = append(args, paths...)
	stdout, stderr, err := runEvidenceGit(repoRoot, nil, args...)
	if err != nil {
		return "", fmt.Errorf("git diff (normalized identity) for %s: %v: %s", commit, err, strings.TrimSpace(stderr))
	}
	return sha256Hex([]byte(NormalizeDiffBytes(stdout))), nil
}

// NormalizeDiffBytes applies the two D18 post-processing rules. Exported
// for unit testing over fixture bytes (AC-L42/AC-L43/AC-L44/AC-L133).
func NormalizeDiffBytes(diff string) string {
	lines := strings.Split(diff, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "index ") {
			continue
		}
		if hunkHeaderRe.MatchString(line) {
			out = append(out, "@@")
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ── Object reads ─────────────────────────────────────────────────────────

// BlobAtTree returns the bytes of path inside treeish. `found` is false
// when the path does not exist there; err is non-nil only for a genuine
// git failure (which the caller classifies, e.g. `history-incomplete`).
func BlobAtTree(repoRoot, treeish, path string) (data []byte, found bool, stderr string, err error) {
	spec := treeish + ":" + path
	out, errOut, runErr := runEvidenceGit(repoRoot, nil, "cat-file", "blob", spec)
	if runErr != nil {
		low := strings.ToLower(errOut)
		// git's "does not exist in" / "exists on disk, but not in" forms
		// are the path-absent answer, not a reader failure.
		if strings.Contains(low, "does not exist in") ||
			strings.Contains(low, "exists on disk, but not in") ||
			strings.Contains(low, "path ") && strings.Contains(low, "not in") {
			return nil, false, errOut, nil
		}
		return nil, false, errOut, fmt.Errorf("git cat-file blob %s: %v: %s", spec, runErr, strings.TrimSpace(errOut))
	}
	return []byte(out), true, errOut, nil
}

// ResolveRevOffline resolves a revision under the offline discipline.
// Used for `<commit>^{commit}` producer validation (ADR-013 D19).
func ResolveRevOffline(repoRoot, rev string) (string, string, error) {
	out, stderr, err := runEvidenceGit(repoRoot, nil, "rev-parse", "--verify", rev)
	if err != nil {
		return "", stderr, fmt.Errorf("git rev-parse --verify %s: %v: %s", rev, err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(out), stderr, nil
}

// IsAncestorOffline is `git merge-base --is-ancestor` under the offline
// discipline. Distinct from IsAncestor so the landed contract's
// invocations all carry `GIT_NO_LAZY_FETCH=1` (AC-L129, AC-LD22).
func IsAncestorOffline(repoRoot, ancestor, descendant string) (bool, error) {
	cmd := evidenceCmd(repoRoot, nil, "merge-base", "--is-ancestor", ancestor, descendant)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %v: %s",
		ancestor, descendant, err, strings.TrimSpace(stderr.String()))
}

// IsLowercaseHexOfLen reports whether s is exactly n lowercase hex
// nibbles. The length is always DERIVED from the repository object
// format by callers (40 for sha1, 64 for sha256 — E41); this helper
// never assumes one.
func IsLowercaseHexOfLen(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// HeadCommitOffline resolves HEAD under the offline discipline. Distinct
// from HeadCommit so every command the landed contract issues carries
// `GIT_NO_LAZY_FETCH=1` (AC-L129).
func HeadCommitOffline(repoRoot string) (string, error) {
	out, stderr, err := runEvidenceGit(repoRoot, nil, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %v: %s", err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(out), nil
}

// RunOfflineGitIn runs a git command in dir (typically a shadow
// worktree) under the offline discipline, returning combined stderr.
func RunOfflineGitIn(dir string, args ...string) (string, string, error) {
	return runEvidenceGit(dir, nil, args...)
}

// OfflineGitResult is a structured git invocation outcome.
//
// ExitCode is the discriminator between an ANSWER and a FAILURE, the
// same split ApplyCheckResult makes: `git apply --check` exits 1 when
// the patch legitimately does not apply, and 128 (or anything else)
// when git could not carry the command out. ExitCode is 0 on success
// and -1 when the process could not be started or was signalled.
type OfflineGitResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// Answered reports whether the command produced a domain-level verdict
// (success, the documented exit-1 "does not apply", or a patch-level
// diagnostic) rather than an execution failure.
func (r OfflineGitResult) Answered() bool {
	return ApplyProbeAnswered(r.ExitCode, r.Err == nil, r.Stderr)
}

// OK reports a clean success.
func (r OfflineGitResult) OK() bool { return r.Err == nil }

// RunOfflineGitInResult is RunOfflineGitIn with the exit code retained,
// so callers can separate an answer from a failure instead of
// string-matching status text (v0.15.1 Wave C rev-2, finding 3).
func RunOfflineGitInResult(dir string, args ...string) OfflineGitResult {
	stdout, stderr, err := runEvidenceGit(dir, nil, args...)
	res := OfflineGitResult{Stdout: stdout, Stderr: stderr, Err: err}
	if err != nil {
		res.ExitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		}
	}
	return res
}
