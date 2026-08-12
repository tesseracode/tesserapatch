// Nested linked-worktree discovery and filtering (GH #7).
//
// Agent harnesses routinely provision linked Git worktrees *inside*
// the target repository (`.claude/worktrees/agent-*`, `.worktrees/…`).
// Git reports such a directory to `git ls-files --others` and
// `git status --porcelain` as an ordinary untracked path, so every
// tpatch capture surface would happily `git add --intent-to-add` it —
// which turns the whole checkout into a single `mode 160000` gitlink
// entry in `post-apply.patch` and drags unrelated agent state into the
// feature artifacts.
//
// This file is the single authority for "is this path inside a
// registered linked worktree nested under the target repository?".
// Every capture / staging surface routes through NestedWorktreePrefixes
// plus the PathUnderNestedWorktree membership primitive so the
// classification cannot drift between callers.
//
// Deliberate non-goals — these MUST keep working exactly as before:
//
//   - ordinary directories (including ones named like worktree roots);
//   - real submodules and any intentionally tracked gitlink;
//   - unrelated nested Git repositories that are NOT registered linked
//     worktrees of this repository;
//   - linked worktrees registered OUTSIDE the target repository root
//     (the long-standing "run tpatch from a linked worktree" flows).

package gitutil

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNestedWorktreeDiscovery marks the fail-closed class: tpatch could
// not prove whether linked Git worktrees are nested under the target
// repository. Capture surfaces that otherwise degrade a Git failure to
// a warning (`apply --mode done`, the `cycle` record step) MUST treat
// this class as fatal — capturing blind is precisely the GH #7 bug.
var ErrNestedWorktreeDiscovery = errors.New("nested linked-worktree discovery failed")

// NestedWorktreePrefixes returns normalized, repo-relative,
// forward-slash path prefixes for every registered linked worktree
// that lives strictly *under* repoRoot. The target repository's own
// worktree root is never returned, and neither is any worktree that
// resolves outside repoRoot.
//
// The result is sorted and de-duplicated. A nil slice means "this
// repository has no nested linked worktrees" — it never means
// "discovery failed": discovery failures are returned as an error so
// callers fail closed instead of silently capturing agent state.
func NestedWorktreePrefixes(repoRoot string) ([]string, error) {
	paths, err := listRegisteredWorktreePaths(repoRoot)
	if err != nil {
		return nil, NestedWorktreeDiscoveryError(repoRoot, err)
	}
	return nestedWorktreePrefixes(repoRoot, paths), nil
}

// PathUnderNestedWorktree reports whether a repo-relative path is a
// nested linked worktree root itself or any descendant of one.
//
// Matching is segment-boundary exact: with a prefix of `agent`, the
// path `agent-other` is NOT a match, while `agent` and `agent/x` are.
// Trailing slashes (which `git ls-files --others` and
// `git status --porcelain` both emit for un-recursed directories) and
// a leading `./` are normalized away before comparison.
func PathUnderNestedWorktree(p string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return false
	}
	np := normalizeRepoRelPath(p)
	if np == "" {
		return false
	}
	for _, prefix := range prefixes {
		if prefix == "" {
			continue
		}
		if np == prefix || strings.HasPrefix(np, prefix+"/") {
			return true
		}
	}
	return false
}

// FilterNestedWorktreePaths returns the subset of paths that are not
// inside any nested linked worktree, preserving input order. The
// original slice is never mutated.
func FilterNestedWorktreePaths(paths, prefixes []string) []string {
	if len(prefixes) == 0 {
		return paths
	}
	keep := make([]string, 0, len(paths))
	for _, p := range paths {
		if PathUnderNestedWorktree(p, prefixes) {
			continue
		}
		keep = append(keep, p)
	}
	return keep
}

// nestedWorktreeCaptureFilters discovers the nested linked worktrees of
// repoRoot once and returns both filter shapes a capture site needs:
// `:(exclude,literal)` pathspecs to append to a `git diff` invocation,
// and the raw prefixes for in-process path membership tests.
//
// The `literal` magic keyword is required: a worktree directory name
// may legitimately contain glob metacharacters (`[`, `*`, `?`), which
// a bare `:(exclude)` pattern would interpret.
//
// Discovery failure is returned as an actionable, fail-closed error.
func nestedWorktreeCaptureFilters(repoRoot string) (excludes, prefixes []string, err error) {
	prefixes, err = NestedWorktreePrefixes(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		excludes = append(excludes, ":(exclude,literal)"+p)
	}
	return excludes, prefixes, nil
}

// NestedWorktreeDiscoveryError wraps a `git worktree list` failure with
// the operator-facing guidance that capture is being refused rather
// than performed blind. Already-wrapped errors pass through unchanged
// so the guidance block is never duplicated.
func NestedWorktreeDiscoveryError(repoRoot string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNestedWorktreeDiscovery) {
		return err
	}
	return fmt.Errorf(
		"%w: cannot determine whether linked Git worktrees are nested under %s: %v\n"+
			"Refusing to capture: an unnoticed nested worktree would be recorded as a gitlink and pull unrelated state into the feature patch.\n"+
			"Fix the Git error above (for example `git worktree prune`, or repair `.git`), then retry.",
		ErrNestedWorktreeDiscovery, repoRoot, err)
}

// runGitStreams runs git in dir and returns stdout and stderr
// separately so a failure can be classified (e.g. "unknown switch"
// versus a genuine repository error) instead of being flattened into
// one opaque message.
func runGitStreams(dir string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// listRegisteredWorktreePaths returns the raw worktree paths reported
// by `git worktree list --porcelain`, byte-for-byte.
//
// The NUL-delimited `-z` shape (Git 2.36+) is the only unambiguous
// form: a worktree path may contain any byte except NUL, newlines and
// trailing whitespace included, and only NUL termination can delimit
// it losslessly. It is therefore tried first.
//
// When `-z` is rejected as an unknown switch — i.e. the repository's
// Git predates 2.36 — the newline-delimited form is parsed STRICTLY:
// records are structurally validated against Git's known attribute
// keys, C-quoted paths are decoded, and anything ambiguous (an
// unrecognised continuation line, an unterminated or malformed quote)
// is refused rather than guessed. Guessing is what would let a
// newline-bearing worktree path slip past the filter and back into a
// capture.
//
// Any `-z` failure that is NOT an unknown-switch usage error is
// propagated as-is: a broken repository must fail closed, not silently
// re-route through the weaker legacy shape.
func listRegisteredWorktreePaths(repoRoot string) ([]string, error) {
	out, stderr, err := runGitStreams(repoRoot, "worktree", "list", "--porcelain", "-z")
	if err == nil {
		return parseWorktreeListNUL(out)
	}
	if !isUnknownSwitchError(stderr) {
		return nil, fmt.Errorf("git worktree list --porcelain -z: %v: %s", err, strings.TrimSpace(stderr))
	}
	legacy, legacyStderr, legacyErr := runGitStreams(repoRoot, "worktree", "list", "--porcelain")
	if legacyErr != nil {
		return nil, fmt.Errorf("git worktree list --porcelain: %v: %s", legacyErr, strings.TrimSpace(legacyStderr))
	}
	return parseWorktreeListLegacy(legacy)
}

// isUnknownSwitchError reports whether Git rejected an option it does
// not know, which is how a pre-2.36 Git answers `worktree list -z`.
// Git prints `error: unknown switch \`z'` (short form) or
// `error: unknown option \`...'` followed by the usage block, and
// exits with the usage status.
func isUnknownSwitchError(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "unknown switch") ||
		strings.Contains(s, "unknown option") ||
		strings.Contains(s, "usage: git worktree list")
}

// parseWorktreeListNUL extracts worktree paths from the NUL-delimited
// porcelain shape. Each attribute is NUL-terminated and each record is
// closed by an extra NUL.
//
// The value after the exact `worktree ` key is taken VERBATIM: no
// trimming of any kind. A directory legitimately named `agent ` (with
// a trailing space) or `agent\t` must round-trip byte-for-byte,
// otherwise the derived prefix stops matching the path Git reports for
// the same directory elsewhere and the worktree slips back into
// capture.
func parseWorktreeListNUL(out string) ([]string, error) {
	var paths []string
	for _, field := range strings.Split(out, "\x00") {
		if !strings.HasPrefix(field, worktreeKey) {
			continue
		}
		p := field[len(worktreeKey):]
		if p == "" {
			return nil, fmt.Errorf("git worktree list --porcelain -z: empty path in record %q", field)
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("git worktree list --porcelain -z: no worktree records in output %q", truncateForError(out))
	}
	return paths, nil
}

const worktreeKey = "worktree "

// worktreeAttrKeys is the set of non-`worktree` keys Git emits in a
// `worktree list --porcelain` record. Presence of a value is allowed
// for every key (`locked`/`prunable` carry an optional reason), which
// keeps the validator forward-compatible with new valued attributes
// while still catching a stray path continuation line.
var worktreeAttrKeys = map[string]bool{
	"HEAD":     true,
	"branch":   true,
	"bare":     true,
	"detached": true,
	"locked":   true,
	"prunable": true,
}

// parseWorktreeListLegacy strictly parses the newline-delimited
// porcelain shape emitted by Git versions without `-z`.
//
// Contract:
//
//   - A record begins with a `worktree ` line and ends at a blank line
//     (Git terminates every record, including the last, with one).
//   - Every other line in a record MUST be a known attribute key,
//     optionally followed by a space and a value. An unrecognised line
//     is treated as the continuation of a path containing a newline —
//     unrepresentable in this shape — and refused.
//   - A path beginning with `"` MUST be a well-formed, fully
//     terminated Git C-quoted string; it is decoded to raw bytes.
//     Malformed quoting is refused, never silently taken literally.
//   - An unquoted path is preserved byte-for-byte, INCLUDING trailing
//     spaces and tabs. `\r` is not stripped: Git does not emit CRLF
//     here, so a trailing `\r` is a legitimate path byte.
func parseWorktreeListLegacy(out string) ([]string, error) {
	lines := strings.Split(out, "\n")
	// A trailing "\n" produces one empty tail element; drop exactly
	// that one so it is not mistaken for a record separator.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	var paths []string
	inRecord := false
	for i, line := range lines {
		if line == "" {
			inRecord = false
			continue
		}
		if strings.HasPrefix(line, worktreeKey) {
			if inRecord {
				return nil, legacyAmbiguityError(i, line, "a second `worktree` line inside an unterminated record")
			}
			p, err := decodeLegacyWorktreePath(line[len(worktreeKey):])
			if err != nil {
				return nil, legacyAmbiguityError(i, line, err.Error())
			}
			paths = append(paths, p)
			inRecord = true
			continue
		}
		if !inRecord {
			return nil, legacyAmbiguityError(i, line, "attribute line outside any worktree record")
		}
		key := line
		if sp := strings.IndexByte(line, ' '); sp >= 0 {
			key = line[:sp]
		}
		if !worktreeAttrKeys[key] {
			return nil, legacyAmbiguityError(i, line,
				"unrecognised line; it is indistinguishable from the continuation of a worktree path containing a newline")
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("git worktree list --porcelain: no worktree records in output %q", truncateForError(out))
	}
	return paths, nil
}

func legacyAmbiguityError(lineNo int, line, why string) error {
	return fmt.Errorf(
		"git worktree list --porcelain (newline-delimited, pre-Git-2.36 fallback) cannot be parsed unambiguously at line %d (%q): %s.\n"+
			"Upgrade Git to 2.36 or newer so tpatch can use the NUL-delimited `git worktree list --porcelain -z` shape, or move the offending worktree to a path without unusual characters",
		lineNo+1, truncateForError(line), why)
}

// decodeLegacyWorktreePath returns the raw bytes of a path field from
// the newline-delimited shape. A leading `"` selects Git's C-quoting;
// everything else is verbatim.
func decodeLegacyWorktreePath(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("empty worktree path")
	}
	if value[0] != '"' {
		return value, nil
	}
	return unquoteCStyle(value)
}

// unquoteCStyle decodes a Git `quote_c_style` string. The input must
// start and end with `"` with nothing after the closing quote, and may
// contain only the escapes Git emits: \a \b \f \n \r \t \v \" \\ and
// one-to-three-digit octal. Anything else is malformed.
func unquoteCStyle(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' {
		return "", fmt.Errorf("malformed C-quoted path: missing opening quote")
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		switch {
		case c == '"':
			if i != len(s)-1 {
				return "", fmt.Errorf("malformed C-quoted path: trailing bytes after the closing quote")
			}
			return b.String(), nil
		case c != '\\':
			b.WriteByte(c)
			i++
		default:
			i++
			if i >= len(s) {
				return "", fmt.Errorf("malformed C-quoted path: escape at end of input")
			}
			e := s[i]
			i++
			switch e {
			case 'a':
				b.WriteByte(0x07)
			case 'b':
				b.WriteByte(0x08)
			case 'f':
				b.WriteByte(0x0c)
			case 'n':
				b.WriteByte(0x0a)
			case 'r':
				b.WriteByte(0x0d)
			case 't':
				b.WriteByte(0x09)
			case 'v':
				b.WriteByte(0x0b)
			case '"', '\\':
				b.WriteByte(e)
			case '0', '1', '2', '3', '4', '5', '6', '7':
				val := int(e - '0')
				digits := 1
				for digits < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7' {
					val = val*8 + int(s[i]-'0')
					i++
					digits++
				}
				if val > 0xFF {
					return "", fmt.Errorf("malformed C-quoted path: octal escape out of range")
				}
				b.WriteByte(byte(val))
			default:
				return "", fmt.Errorf("malformed C-quoted path: unknown escape %q", "\\"+string(e))
			}
		}
	}
	return "", fmt.Errorf("malformed C-quoted path: unterminated quote")
}

// nestedWorktreePrefixes maps absolute worktree paths onto normalized
// repo-relative prefixes, dropping repoRoot itself and every worktree
// that resolves outside repoRoot.
func nestedWorktreePrefixes(repoRoot string, worktreePaths []string) []string {
	rootAbs := resolvePathForCompare(repoRoot)
	seen := make(map[string]bool, len(worktreePaths))
	var out []string
	for _, wt := range worktreePaths {
		if wt == "" {
			continue
		}
		wtAbs := resolvePathForCompare(wt)
		if wtAbs == rootAbs {
			continue
		}
		rel, err := filepath.Rel(rootAbs, wtAbs)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == ".." || strings.HasPrefix(rel, "../") || path.IsAbs(rel) {
			continue
		}
		norm := normalizeRepoRelPath(rel)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	sort.Strings(out)
	return out
}

// resolvePathForCompare renders p as an absolute, symlink-resolved
// path so it can be compared against Git's output, which always prints
// physical paths (on macOS `/var/...` resolves to `/private/var/...`,
// which is exactly the shape `t.TempDir()` produces).
//
// When p does not exist — e.g. a prunable worktree whose directory was
// deleted — the deepest existing ancestor is resolved and the missing
// tail re-attached, so a symlinked parent still compares correctly.
func resolvePathForCompare(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	dir := abs
	var tail []string
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		tail = append([]string{filepath.Base(dir)}, tail...)
		dir = parent
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(append([]string{resolved}, tail...)...)
		}
	}
}

// normalizeRepoRelPath renders a repo-relative path in the canonical
// comparison form: forward slashes, no leading `./`, no trailing `/`,
// no redundant separators. The repository root itself normalizes to
// the empty string.
//
// It performs NO whitespace normalization. Leading and trailing spaces
// and tabs are legitimate path bytes on every platform tpatch targets;
// trimming them would derive a prefix that no longer matches the path
// Git reports for the same directory in `ls-files` / `status`, which
// is exactly how a `agent ` worktree would slip back into a capture.
func normalizeRepoRelPath(p string) string {
	p = filepath.ToSlash(p)
	if p == "" {
		return ""
	}
	c := path.Clean(p)
	if c == "." || c == "/" {
		return ""
	}
	return strings.TrimPrefix(c, "/")
}

func truncateForError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
