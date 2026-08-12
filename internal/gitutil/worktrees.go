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
	"errors"
	"fmt"
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
		return nil, err
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
		return nil, nil, NestedWorktreeDiscoveryError(repoRoot, err)
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
// than performed blind.
func NestedWorktreeDiscoveryError(repoRoot string, err error) error {
	return fmt.Errorf(
		"%w: cannot determine whether linked Git worktrees are nested under %s: %v\n"+
			"Refusing to capture: an unnoticed nested worktree would be recorded as a gitlink and pull unrelated state into the feature patch.\n"+
			"Fix the Git error above (for example `git worktree prune`, or repair `.git`), then retry.",
		ErrNestedWorktreeDiscovery, repoRoot, err)
}

// listRegisteredWorktreePaths returns the raw worktree paths reported
// by `git worktree list --porcelain`.
//
// The NUL-delimited `-z` shape is preferred because it is the only
// unambiguous form (a worktree path may contain any byte except NUL,
// newlines included). `-z` requires Git 2.36+; on older Git the flag is
// rejected, so we transparently retry the newline-delimited form. Both
// shapes tolerate spaces and other special characters, because the
// path is everything after the `worktree ` key — it is never split on
// whitespace.
func listRegisteredWorktreePaths(repoRoot string) ([]string, error) {
	out, zErr := runGit(repoRoot, "worktree", "list", "--porcelain", "-z")
	if zErr == nil {
		return parseWorktreeListPorcelain(out, true)
	}
	plain, err := runGit(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list --porcelain: %v (NUL-delimited attempt: %v)", err, zErr)
	}
	return parseWorktreeListPorcelain(plain, false)
}

// parseWorktreeListPorcelain extracts the `worktree <path>` values from
// `git worktree list --porcelain` output. When nul is true the input is
// NUL-delimited (`-z`); otherwise it is newline-delimited.
//
// Any output that yields zero worktree records is treated as malformed
// and returned as an error: a working repository always reports at
// least its own main worktree, so an empty parse means we cannot prove
// the absence of nested worktrees.
func parseWorktreeListPorcelain(out string, nul bool) ([]string, error) {
	sep := "\n"
	if nul {
		sep = "\x00"
	}
	var paths []string
	for _, field := range strings.Split(out, sep) {
		if !nul {
			field = strings.TrimSuffix(field, "\r")
		}
		if !strings.HasPrefix(field, "worktree ") {
			continue
		}
		p := strings.TrimPrefix(field, "worktree ")
		if strings.TrimSpace(p) == "" {
			return nil, fmt.Errorf("git worktree list --porcelain: empty path in record %q", field)
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("git worktree list --porcelain: no worktree records in output %q", truncateForError(out))
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
		if strings.TrimSpace(wt) == "" {
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
func normalizeRepoRelPath(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
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
