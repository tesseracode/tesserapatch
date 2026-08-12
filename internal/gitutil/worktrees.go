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

// FilterPathsExcludingNestedWorktrees returns `paths` with every entry
// that names a registered nested linked worktree (or anything inside
// one) removed. Order is preserved and the input slice is not mutated.
//
// Discovery failure is fail-closed: the wrapped
// ErrNestedWorktreeDiscovery is returned so callers refuse rather than
// operating on an unfiltered path set.
//
// Callers that use the result as a diff scope MUST treat
// "non-empty input filtered down to empty" as "produce nothing", never
// as "no scope, diff everything" — see DiffFromCommitForPaths.
func FilterPathsExcludingNestedWorktrees(repoRoot string, paths []string) ([]string, error) {
	prefixes, err := NestedWorktreePrefixes(repoRoot)
	if err != nil {
		return nil, err
	}
	return FilterNestedWorktreePaths(paths, prefixes), nil
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
// by `git worktree list --porcelain -z`, byte-for-byte.
//
// The NUL-delimited `-z` shape (Git 2.36+) is the ONLY shape tpatch
// accepts. A worktree path may contain any byte except NUL — newlines,
// tabs and trailing whitespace included — and only NUL termination can
// delimit it losslessly.
//
// GH #7 rev-2: the newline-delimited fallback that used to run on
// pre-2.36 Git is gone, not merely stricter. Two independent defects
// made it unsafe:
//
//  1. It is intrinsically ambiguous. A worktree path containing a
//     newline whose continuation happens to be shaped like a valid
//     attribute — `locked x`, `bare`, `HEAD <sha>` — parses as a
//     well-formed record, so no amount of structural validation can
//     tell it apart from a real one. The nested worktree then silently
//     escapes the filter and lands back in a capture as a gitlink.
//  2. Deciding that `-z` is unsupported required classifying Git's
//     stderr, and bare `usage:` text is emitted for several failures
//     that have nothing to do with an unknown switch.
//
// Every failure — unknown switch, usage error, broken repository, git
// missing — is therefore fail-closed with actionable guidance. tpatch
// never runs plain `git worktree list --porcelain`.
func listRegisteredWorktreePaths(repoRoot string) ([]string, error) {
	out, stderr, err := runGitStreams(repoRoot, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf(
			"git worktree list --porcelain -z failed: %s\n"+
				"tpatch requires Git 2.36 or newer: the NUL-delimited porcelain shape is the only one that "+
				"delimits worktree paths losslessly, and byte-safe nested-worktree exclusion depends on it. "+
				"The newline-delimited shape is intrinsically ambiguous (a path containing a newline can parse "+
				"as a well-formed record), so tpatch will not fall back to it",
			detail)
	}
	return parseWorktreeListNUL(out)
}

const worktreeKey = "worktree "

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
//
// Output that yields zero worktree records is treated as malformed: a
// working repository always reports at least its own main worktree, so
// an empty parse means we cannot prove the absence of nested
// worktrees.
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
