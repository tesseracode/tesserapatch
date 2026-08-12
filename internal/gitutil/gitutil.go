// Package gitutil provides git operations: diff, patch capture, reverse-apply, head commit.
package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// HeadCommit returns the current HEAD commit hash.
func HeadCommit(repoRoot string) (string, error) {
	out, err := runGit(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// FirstCommit returns the root (initial) commit hash on the current
// branch, i.e. the earliest ancestor of HEAD. Used as a stable
// repository-identity signal that survives HEAD advancing (unlike
// HeadCommit) and is deterministic across clones (unlike remote
// URLs or worktree paths). Returns an error on repos with no
// commits.
func FirstCommit(repoRoot string) (string, error) {
	out, err := runGit(repoRoot, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-list --max-parents=0 HEAD: %w", err)
	}
	// A repository can have multiple root commits (rare — orphan
	// branch merges). Take the first line — deterministic across
	// runs because git rev-list emits in newest-first order.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("git rev-list --max-parents=0 HEAD: no root commit")
}

// RecentCommit is a one-line summary of a commit, used to suggest
// candidate --from base refs when `tpatch record` captures an empty
// diff (almost always because the user committed before recording).
type RecentCommit struct {
	SHA     string // short SHA
	When    string // "2 hours ago"
	Subject string // commit subject line
}

// RecentCommits returns up to `limit` recent commits on HEAD, newest
// first. Used by the record command to give the user concrete --from
// candidates in the "you committed before recording" diagnostic. Never
// returns an error — a bare repo / shallow clone / first commit case
// simply yields a shorter list.
func RecentCommits(repoRoot string, limit int) []RecentCommit {
	if limit <= 0 {
		limit = 10
	}
	// Use an ASCII unit separator between fields so commit subjects
	// containing tabs or pipes do not break parsing.
	sep := "\x1f"
	format := "%h" + sep + "%ar" + sep + "%s"
	out, err := runGit(repoRoot, "log", fmt.Sprintf("-n%d", limit), "--pretty=format:"+format)
	if err != nil {
		return nil
	}
	var result []RecentCommit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 3)
		if len(parts) != 3 {
			continue
		}
		result = append(result, RecentCommit{SHA: parts[0], When: parts[1], Subject: parts[2]})
	}
	return result
}

// IsWorkingTreeDirty reports whether there are unstaged or untracked
// changes in the repo. Used by the record empty-capture diagnostic to
// distinguish the "nothing changed" case from the "you committed
// already" case.
//
// GH #7 (rev-0 user-external note): registered nested linked worktrees
// are subtracted, because capture deliberately excludes them. Counting
// one as dirt made an empty capture report "working tree is dirty, but
// no textual diff was produced — possibly mode-only or binary changes"
// when the real story was "the only dirt is a worktree we filtered
// out", and it suppressed the correct `--from` recovery guidance.
//
// The NUL-delimited porcelain shape is used so paths arrive
// byte-for-byte, with `--untracked-files=all` so an untracked
// directory is not collapsed to a parent path that the nested-worktree
// filter cannot classify.  If worktree discovery fails, the raw answer
// is returned rather than claiming a clean tree.
func IsWorkingTreeDirty(repoRoot string) bool {
	out, err := runGit(repoRoot, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return false
	}
	nested, nestedErr := NestedWorktreePrefixes(repoRoot)
	if nestedErr != nil {
		return strings.TrimSpace(out) != ""
	}
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}
		x, y := entry[0], entry[1]
		p := entry[3:]
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			// Consume the rename/copy origin field.
			i++
		}
		if p == "" || PathUnderNestedWorktree(p, nested) {
			continue
		}
		return true
	}
	return false
}

// IsPathTracked reports whether `path` (relative to repoRoot) is
// tracked by git. A missing path or any git error returns false so
// callers can treat "not tracked" as the conservative default.
func IsPathTracked(repoRoot, path string) bool {
	out, err := runGit(repoRoot, "ls-files", "--", path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

// ReconcilePreflight is the preflight report returned by
// PreflightReconcile. The Reconcile phase MUST NOT run when any of the
// four fields is non-empty, unless the user passes --allow-dirty.
//
// Rationale (see A10 doc-reconcile-workflow): a dirty tree or lingering
// conflict markers silently corrupt reverse/forward apply verdicts —
// reconcile reads file bytes, not git trees, so a `<<<<<<<` line inside
// a source file looks exactly like any other context line to `git apply
// --check`. We hard-refuse instead of guessing.
type ReconcilePreflight struct {
	// UnstagedFiles lists `git status --porcelain` entries with their
	// status code, e.g. " M apps/server/src/foo.ts".
	UnstagedFiles []string
	// UntrackedFiles lists files present in the tree but ignored by
	// git (separate from modified-tracked files).
	UntrackedFiles []string
	// MergeMarkerFiles lists paths that still contain `<<<<<<< `,
	// `=======`, or `>>>>>>> ` conflict markers.
	MergeMarkerFiles []string
	// LeftoverFiles lists *.orig and *.rej files — the classic "I
	// aborted a merge but forgot to clean up" footprint.
	LeftoverFiles []string
	// LockState classifies `.tpatch/upstream.lock` for the reconcile
	// command. Populated only by PreflightReconcileWithOverride;
	// PreflightReconcile leaves it at LockStateUnknown.
	// See PRD-reconcile-lock-guard §3.1.
	LockState LockState
	// LockDiagnostic carries the fields needed to format a stale-lock
	// refusal block or a skipped/empty/missing note. Zero-valued when
	// LockState is Valid, Unknown, or otherwise has nothing to report.
	LockDiagnostic LockDiagnostic
}

// Clean reports whether the preflight found zero violations.
func (p ReconcilePreflight) Clean() bool {
	return len(p.UnstagedFiles) == 0 &&
		len(p.UntrackedFiles) == 0 &&
		len(p.MergeMarkerFiles) == 0 &&
		len(p.LeftoverFiles) == 0
}

// PreflightReconcile inspects the working tree for the four conditions
// that make reconcile verdicts unreliable. It is read-only — it never
// modifies files. See ReconcilePreflight for the contract.
func PreflightReconcile(repoRoot string) (ReconcilePreflight, error) {
	var p ReconcilePreflight

	// git status --porcelain: first two columns are the status code,
	// remainder is the path. We split tracked-modified from untracked.
	out, err := runGit(repoRoot, "status", "--porcelain")
	if err != nil {
		return p, fmt.Errorf("git status: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		code, path := line[:2], strings.TrimSpace(line[3:])
		if code == "??" {
			p.UntrackedFiles = append(p.UntrackedFiles, path)
		} else {
			p.UnstagedFiles = append(p.UnstagedFiles, line)
		}
	}

	// Conflict markers. `git grep -lE '^<<<<<<< |^=======$|^>>>>>>> '`
	// scans tracked files only, which is what we want — untracked
	// noise is already reported above.
	if mark, _ := runGit(repoRoot, "grep", "-lE", "^<<<<<<< |^=======$|^>>>>>>> "); strings.TrimSpace(mark) != "" {
		for _, f := range strings.Split(strings.TrimSpace(mark), "\n") {
			p.MergeMarkerFiles = append(p.MergeMarkerFiles, f)
		}
	}

	// *.orig and *.rej leftovers anywhere in the tree (walk, cheap).
	_ = filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		// Skip .git/ entirely.
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".orig") || strings.HasSuffix(name, ".rej") {
			rel, rerr := filepath.Rel(repoRoot, path)
			if rerr != nil {
				rel = path
			}
			p.LeftoverFiles = append(p.LeftoverFiles, rel)
		}
		return nil
	})

	sort.Strings(p.UnstagedFiles)
	sort.Strings(p.UntrackedFiles)
	sort.Strings(p.MergeMarkerFiles)
	sort.Strings(p.LeftoverFiles)
	return p, nil
}

// CaptureDiffStat returns `git diff --stat` output for the full tree.
func CaptureDiffStat(repoRoot string) (string, error) {
	return CaptureDiffStatScoped(repoRoot, nil)
}

// CaptureDiffStatScoped returns `git diff --stat` output narrowed to
// `pathspecs`. Empty pathspecs reproduces the historical full-tree
// behaviour byte-for-byte. Used by `record --files <pathspec>...` so
// that record.md and post-apply-diff.txt stay scoped to the same set
// the captured patch covers (M15-W2 review F2: previously the patch
// was scoped but the diffstat metadata was not, leaking cross-feature
// edits into per-feature artifacts).
//
// GH #7 rev-1: the diffstat carries the SAME nested-worktree
// `:(exclude,literal)` pathspecs as the patch capture. Without them, a
// pre-existing intent-to-add or staged gitlink for a nested registered
// worktree — residue from a pre-fix tpatch run or from an operator's
// own `git add` — was filtered out of `post-apply.patch` but still
// rendered into `post-apply-diff.txt`. Discovery failure is
// fail-closed here too.
func CaptureDiffStatScoped(repoRoot string, pathspecs []string) (string, error) {
	nestedExcludes, _, err := nestedWorktreeCaptureFilters(repoRoot)
	if err != nil {
		return "", err
	}
	args := []string{"diff", "--stat"}
	if len(nestedExcludes) > 0 || len(pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, nestedExcludes...)
		args = append(args, pathspecs...)
	}
	out, err := runGit(repoRoot, args...)
	if err != nil {
		return "", err
	}
	return out, nil
}

// CapturePatch captures a unified diff including tracked modifications and untracked new files.
// It excludes .tpatch/, .claude/skills/, .github/skills/, .github/prompts/, .cursor/rules/.
func CapturePatch(repoRoot string) (string, error) {
	return CapturePatchScoped(repoRoot, nil)
}

// CapturePatchScoped is like CapturePatch but, when pathspecs is
// non-empty, narrows the diff to those pathspecs (passed verbatim to
// `git diff -- <pathspec>...`). Pathspecs may be plain paths, globs,
// or git's `:(...)` magic forms. Empty pathspecs reproduces the
// historical full-tree capture byte-for-byte.
func CapturePatchScoped(repoRoot string, pathspecs []string) (string, error) {
	excludePatterns := []string{
		":(exclude).tpatch",
		":(exclude).claude/skills",
		":(exclude).github/skills",
		":(exclude).github/prompts",
		":(exclude).cursor/rules",
		":(exclude).windsurfrules",
	}

	// GH #7: registered linked worktrees nested under repoRoot are
	// excluded from the diff as well as from the intent-to-add pass
	// below. Discovery failure is fail-closed — capturing blind would
	// fold an agent harness checkout into the feature patch as a
	// mode-160000 gitlink.
	//
	// GH #7 rev-2 (F4): discovery runs EXACTLY ONCE per capture and
	// both of its products are threaded through, so this helper cannot
	// observe two different answers and presents a single failure
	// window, entirely before any caller-visible write.
	nestedExcludes, nestedPrefixes, err := nestedWorktreeCaptureFilters(repoRoot)
	if err != nil {
		return "", err
	}
	excludePatterns = append(excludePatterns, nestedExcludes...)

	skipPrefixes := []string{".tpatch/", ".claude/skills/", ".github/skills/", ".github/prompts/", ".cursor/rules/", ".windsurfrules"}

	// Stage untracked files with --intent-to-add so they appear in git diff
	untrackedFiles, err := listUntrackedFilesWithPrefixes(repoRoot, pathspecs, nestedPrefixes)
	if err != nil {
		return "", err
	}
	var stagedNewFiles []string
	for _, file := range untrackedFiles {
		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(file, prefix) || file == strings.TrimSuffix(prefix, "/") {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		// Stage as intent-to-add (makes new files visible to git diff)
		if _, err := runGit(repoRoot, "--literal-pathspecs", "add", "--intent-to-add", "--", file); err != nil {
			for _, staged := range stagedNewFiles {
				runGit(repoRoot, "--literal-pathspecs", "reset", "--", staged)
			}
			return "", fmt.Errorf("git add --intent-to-add %q: %w", file, err)
		}
		stagedNewFiles = append(stagedNewFiles, file)
	}

	// Capture unified diff (now includes tracked changes AND intent-to-add new files).
	// Order: `git diff -- <excludes...> <pathspecs...>`. Excludes come
	// first so a user-supplied positive pathspec like `src/` does not
	// re-include files under the always-excluded directories.
	args := append([]string{"diff", "--"}, excludePatterns...)
	if len(pathspecs) > 0 {
		args = append(args, pathspecs...)
	}
	patch, err := runGit(repoRoot, args...)
	if err != nil {
		// When the caller supplied explicit pathspecs, surface the git
		// error (e.g. `fatal: Invalid pathspec magic ...`). Empty
		// pathspecs preserves the historical "tolerate transient diff
		// failure → empty patch" behaviour the unscoped capture path
		// has always relied on (M15-W2 review F3: silent error
		// swallowing misled `--files` users with a generic "0 bytes"
		// diagnostic).
		if len(pathspecs) > 0 {
			// Best-effort cleanup of any intent-to-add markers we
			// staged before the diff failed; ignore secondary errors
			// because the primary error from git is the useful signal.
			for _, file := range stagedNewFiles {
				runGit(repoRoot, "--literal-pathspecs", "reset", "--", file)
			}
			return "", fmt.Errorf("git diff failed for pathspecs %v: %w", pathspecs, err)
		}
		patch = ""
	}

	// Unstage the intent-to-add files to leave the working tree clean
	for _, file := range stagedNewFiles {
		runGit(repoRoot, "--literal-pathspecs", "reset", "--", file)
	}

	return normalizePatchTail(patch), nil
}

// CapturePatchFromCommits captures the diff between two commits, excluding tpatch artifacts.
// Equivalent to CapturePatchFromCommitsScoped(repoRoot, fromRef, toRef, nil) — preserved
// as a thin wrapper so existing callers keep their byte-for-byte output.
func CapturePatchFromCommits(repoRoot, fromRef, toRef string) (string, error) {
	return CapturePatchFromCommitsScoped(repoRoot, fromRef, toRef, nil)
}

// CapturePatchFromCommitsScoped captures the diff between two commits and, when
// pathspecs is non-empty, narrows the diff to those pathspecs (passed verbatim
// to `git diff <from> <to> -- <pathspec>...`). Pathspecs may be plain paths,
// globs, or git's `:(...)` magic forms.
//
// Note: committed-range capture intentionally does NOT consult `git ls-files
// --others`. The working tree is irrelevant to a range diff — only the
// committed snapshots at fromRef and toRef matter — so untracked files are
// never included regardless of working-tree state.
//
// Exclude patterns (.tpatch/, .claude/skills/, etc.) come before user-supplied
// pathspecs so a positive pathspec like `src/` does not re-include files under
// the always-excluded directories (mirrors CapturePatchScoped).
func CapturePatchFromCommitsScoped(repoRoot, fromRef, toRef string, pathspecs []string) (string, error) {
	excludePatterns := []string{
		":(exclude).tpatch",
		":(exclude).claude/skills",
		":(exclude).github/skills",
		":(exclude).github/prompts",
		":(exclude).cursor/rules",
		":(exclude).windsurfrules",
	}
	args := append([]string{"diff", fromRef, toRef, "--"}, excludePatterns...)
	if len(pathspecs) > 0 {
		args = append(args, pathspecs...)
	}
	out, err := runGit(repoRoot, args...)
	if err != nil {
		if len(pathspecs) > 0 {
			return "", fmt.Errorf("git diff failed for pathspecs %v: %w", pathspecs, err)
		}
		return "", err
	}
	return normalizePatchTail(out), nil
}

// normalizePatchTail returns patch with exactly one trailing newline,
// or "" when the patch is effectively empty (only whitespace).
//
// Root cause of bug-record-roundtrip-false-positive-markdown: the
// previous implementation called strings.TrimSpace on the whole patch
// before re-appending a single "\n". TrimSpace strips ALL trailing
// whitespace bytes, not just the trailing newline — so when the very
// last line of a `git diff` ends with content that has trailing
// whitespace (e.g. a markdown blockquote line `+> ` whose space after
// `>` is a deliberate soft-break, or any added line ending in a space
// or tab), the trailing space was eaten along with the final newline.
// Re-appending "\n" then produced a patch whose final hunk line was
// `+>` instead of `+> `, which no longer matches the on-disk file —
// causing `git apply --reverse --check` to (correctly!) reject it as
// "patch does not apply" and `tpatch record` to emit a misleading
// "patch does not round-trip against working tree" warning. Worse,
// the corrupted patch was also persisted to patches/NNN-record.patch
// and would not replay cleanly later.
//
// The fix preserves every byte of every content line and only
// normalizes the trailing newline count to exactly one. We still
// collapse a wholly-whitespace capture to "" so the upstream
// "0 bytes — nothing to record" diagnostic keeps firing.
func normalizePatchTail(patch string) string {
	if strings.TrimSpace(patch) == "" {
		return ""
	}
	return strings.TrimRight(patch, "\n") + "\n"
}

// ValidatePatch runs `git apply --check` to verify a patch is well-formed and can be applied.
// It checks against the given repoRoot (which should be at the target baseline).
// Returns nil if valid. Tries strict first, then 3-way if strategy is "3way".
func ValidatePatch(repoRoot, patch, strategy string) error {
	if patch == "" {
		return fmt.Errorf("empty patch")
	}
	// Strict check first
	cmd := exec.Command("git", "apply", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	if cmd.Run() == nil {
		return nil
	}
	if strategy == "3way" || strategy == "" {
		cmd = exec.Command("git", "apply", "--3way", "--check", "-")
		cmd.Dir = repoRoot
		cmd.Stdin = strings.NewReader(patch)
		if cmd.Run() == nil {
			return nil
		}
	}
	return fmt.Errorf("patch validation failed: patch cannot be applied cleanly")
}

// ReverseApplyCheck tests if a patch can be reverse-applied (already present in the tree).
func ReverseApplyCheck(repoRoot, patch string) (bool, error) {
	cmd := exec.Command("git", "apply", "--reverse", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	err := cmd.Run()
	return err == nil, nil
}

// ValidatePatchReverse runs `git apply --reverse --check` against the
// current working tree. This is the correct semantic for record-time
// validation: the patch was just applied, so the working tree contains
// its result. A successful reverse-apply proves the recorded patch
// round-trips against what is on disk — i.e. it is well-formed and
// describes the changes accurately.
//
// Compare with ValidatePatch (forward `git apply --check`) which is
// correct for reconcile/rebase-time validation against an upstream
// baseline that does NOT yet contain the patch.
//
// Returns nil on success. On failure, surfaces git's stderr so users
// can see the precise reason (line-ending mismatch, binary file
// without index, untracked-file collision, etc).
func ValidatePatchReverse(repoRoot, patch string) error {
	if patch == "" {
		return fmt.Errorf("empty patch")
	}
	cmd := exec.Command("git", "apply", "--reverse", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("patch does not round-trip against working tree: %s", msg)
	}
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return fmt.Errorf("patch does not round-trip against working tree: %s", msg)
	}
	return nil
}

// ForwardApplyCheck tests if a patch can be applied cleanly.
// Tries strict apply first, then falls back to 3-way merge check.
func ForwardApplyCheck(repoRoot, patch string) (bool, error) {
	// Try strict apply first
	cmd := exec.Command("git", "apply", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	if cmd.Run() == nil {
		return true, nil
	}
	// Fall back to 3-way merge check (handles context mismatches)
	cmd = exec.Command("git", "apply", "--3way", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	return cmd.Run() == nil, nil
}

// ForwardApplyVerdict is what phase-4 of reconcile now consumes. It
// distinguishes a clean re-apply from a 3-way merge that will leave
// conflict markers in the tree — the latter used to masquerade as
// "reapplied" because `git apply --3way --check` returns 0 whenever
// the 3-way machinery *could attempt* the merge, even if the final
// files contain conflict markers.
type ForwardApplyVerdict int

const (
	// ForwardApplyStrict means `git apply --check` (without --3way)
	// succeeds. Safe to auto-apply.
	ForwardApplyStrict ForwardApplyVerdict = iota
	// ForwardApply3WayClean means strict failed but a real 3-way merge
	// in an isolated worktree completes without conflict markers.
	ForwardApply3WayClean
	// ForwardApply3WayConflicts means the 3-way merge runs but leaves
	// conflict markers — the user must resolve them. ConflictFiles on
	// the ForwardApplyPreview lists the affected paths.
	ForwardApply3WayConflicts
	// ForwardApplyBlocked means neither strict nor 3-way can even
	// attempt the apply.
	ForwardApplyBlocked
)

// ForwardApplyPreview is the structured result of PreviewForwardApply.
// Verdict is always set; ConflictFiles is non-nil only when Verdict ==
// ForwardApply3WayConflicts. Stderr carries git's diagnostic output for
// the final attempt and is surfaced in reconcile notes.
type ForwardApplyPreview struct {
	Verdict       ForwardApplyVerdict
	ConflictFiles []string
	Stderr        string
}

// PreviewForwardApply gives an authoritative phase-4 verdict without
// mutating repoRoot. The algorithm:
//  1. Strict `git apply --check` — if it passes, return ForwardApplyStrict.
//  2. Create a temporary linked worktree at HEAD (`git worktree add --detach`).
//  3. Actually run `git apply --3way` in the worktree.
//  4. Scan the worktree for conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`).
//     - No markers + apply exit 0 ⇒ ForwardApply3WayClean.
//     - Markers present        ⇒ ForwardApply3WayConflicts (+ file list).
//     - Apply failed outright  ⇒ ForwardApplyBlocked.
//  5. Remove the worktree.
//
// If the worktree provisioning fails (e.g. bare repo, permissions),
// PreviewForwardApply falls back to the looser strict/--3way --check
// pair and marks the verdict ForwardApply3WayClean conservatively —
// logging the fallback reason in Stderr so callers can report it.
func PreviewForwardApply(repoRoot, patch string) (ForwardApplyPreview, error) {
	if patch == "" {
		return ForwardApplyPreview{Verdict: ForwardApplyBlocked, Stderr: "empty patch"}, nil
	}

	// Phase 4a: strict check.
	strict := exec.Command("git", "apply", "--check", "-")
	strict.Dir = repoRoot
	strict.Stdin = strings.NewReader(patch)
	if strict.Run() == nil {
		return ForwardApplyPreview{Verdict: ForwardApplyStrict}, nil
	}

	// Phase 4b: linked worktree at HEAD for a real 3-way attempt.
	wt, cleanup, wtErr := mkPreviewWorktree(repoRoot)
	if wtErr != nil {
		// Degraded path: without an isolated worktree we cannot prove
		// that a 3-way merge would be clean. `git apply --3way --check`
		// returns 0 even for merges that will leave conflict markers
		// (that's the original bug). Prefer a HONEST Blocked verdict
		// with a clear reason over an optimistic 3WayClean; reconcile
		// callers can surface the reason and the user can investigate.
		return ForwardApplyPreview{
			Verdict: ForwardApplyBlocked,
			Stderr:  fmt.Sprintf("worktree preview unavailable (%v); cannot verify 3-way merge cleanliness — refusing to guess", wtErr),
		}, nil
	}
	defer cleanup()

	apply := exec.Command("git", "apply", "--3way", "-")
	apply.Dir = wt
	apply.Stdin = strings.NewReader(patch)
	var applyErr strings.Builder
	apply.Stderr = &applyErr
	applyExit := apply.Run()

	markers := scanConflictMarkers(wt)
	stderr := strings.TrimSpace(applyErr.String())

	switch {
	case applyExit == nil && len(markers) == 0:
		return ForwardApplyPreview{Verdict: ForwardApply3WayClean, Stderr: stderr}, nil
	case len(markers) > 0:
		return ForwardApplyPreview{
			Verdict:       ForwardApply3WayConflicts,
			ConflictFiles: markers,
			Stderr:        stderr,
		}, nil
	default:
		return ForwardApplyPreview{Verdict: ForwardApplyBlocked, Stderr: stderr}, nil
	}
}

// mkPreviewWorktree provisions a detached linked worktree at HEAD and
// returns its path plus a cleanup func. Safe to call concurrently
// because each invocation uses a unique temp directory.
func mkPreviewWorktree(repoRoot string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "tpatch-preview-*")
	if err != nil {
		return "", nil, err
	}
	add := exec.Command("git", "worktree", "add", "--detach", "-q", dir, "HEAD")
	add.Dir = repoRoot
	var addErr strings.Builder
	add.Stderr = &addErr
	if err := add.Run(); err != nil {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("git worktree add: %v: %s", err, strings.TrimSpace(addErr.String()))
	}
	cleanup := func() {
		rm := exec.Command("git", "worktree", "remove", "--force", dir)
		rm.Dir = repoRoot
		_ = rm.Run()
		os.RemoveAll(dir)
	}
	return dir, cleanup, nil
}

// scanConflictMarkers walks the worktree looking for files that contain
// `<<<<<<<` at the start of a line (the canonical git merge marker).
// Returns repo-relative paths sorted alphabetically.
// ScanConflictMarkers walks root looking for files that contain git
// conflict markers (`<<<<<<<` and `>>>>>>>` on line starts). Skips
// `.git`, files larger than 5MB (binary-ish), and any read errors.
// Returns repo-relative paths, sorted. Safe to call on the main
// working tree as a defensive last-line check; reconcile uses it to
// detect a conflict-markers-but-reapplied false positive.
func ScanConflictMarkers(root string) []string {
	return scanConflictMarkers(root)
}

func scanConflictMarkers(root string) []string {
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 5*1024*1024 { // skip > 5MB binaries/assets
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if bytesHasLine(data, "<<<<<<<") && bytesHasLine(data, ">>>>>>>") {
			rel, relErr := filepath.Rel(root, p)
			if relErr == nil {
				out = append(out, rel)
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// HasConflictMarkers reports whether data contains both `<<<<<<<`
// and `>>>>>>>` at the start of any line — the canonical signature of
// an unresolved git merge. Exported so the phase-3.5 validation gate
// can check a single file's in-memory content without walking a tree.
func HasConflictMarkers(data []byte) bool {
	return bytesHasLine(data, "<<<<<<<") && bytesHasLine(data, ">>>>>>>")
}

// bytesHasLine reports whether data contains prefix at the start of any
// line. Avoids allocating a string for large files.
func bytesHasLine(data []byte, prefix string) bool {
	if len(data) == 0 || len(prefix) == 0 {
		return false
	}
	p := []byte(prefix)
	// Start of file.
	if len(data) >= len(p) && bytesEq(data[:len(p)], p) {
		return true
	}
	for i := 0; i+1+len(p) <= len(data); i++ {
		if data[i] == '\n' && bytesEq(data[i+1:i+1+len(p)], p) {
			return true
		}
	}
	return false
}

func bytesEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ForwardApply applies a patch. Uses 3-way merge if strict apply fails.
func ForwardApply(repoRoot, patch string) error {
	cmd := exec.Command("git", "apply", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	// Retry with 3-way merge
	cmd = exec.Command("git", "apply", "--3way", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply failed: %s: %w", string(out), err)
	}
	return nil
}

// FetchUpstream fetches from a remote ref.
func FetchUpstream(repoRoot, remote string) error {
	_, err := runGit(repoRoot, "fetch", remote)
	return err
}

// DiffBetween returns the diff between two refs.
func DiffBetween(repoRoot, fromRef, toRef string) (string, error) {
	return runGit(repoRoot, "diff", fromRef, toRef)
}

// ResolveRef resolves a ref to its commit hash.
func ResolveRef(repoRoot, ref string) (string, error) {
	out, err := runGit(repoRoot, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// FileAtCommit returns the contents of a repo-relative path as it
// existed at a given commit. Used by phase 3.5 to build ConflictInput
// slices (base/theirs). If the file did not exist at that commit, the
// returned bytes are nil and the error is nil — callers treat a missing
// side as empty for three-way reconciliation purposes (git does the
// same). Any other git failure is returned verbatim.
func FileAtCommit(repoRoot, commit, relPath string) ([]byte, error) {
	// `git show <commit>:<path>` prints the blob to stdout.
	cmd := exec.Command("git", "show", commit+":"+relPath)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		// "exists on disk, but not in <commit>" or "does not exist in
		// <commit>" manifest as non-zero exit — treat as absent.
		if _, ok := err.(*exec.ExitError); ok {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

// IsAncestor reports whether `ancestor` is an ancestor of `descendant`
// in the git history rooted at repoRoot. Wraps
//
//	git merge-base --is-ancestor <ancestor> <descendant>
//
// which exits 0 when ancestor is reachable, 1 when it is not, and any
// other non-zero exit on a real git failure (bad ref, repo missing,
// etc.). Mapped as: exit 0 -> (true, nil); exit 1 -> (false, nil);
// otherwise (false, err).
func IsAncestor(repoRoot, ancestor, descendant string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = repoRoot
	// Capture stderr so a real failure surfaces a useful message.
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %s", ancestor, descendant, strings.TrimSpace(stderr.String()))
	}
	return false, err
}

// RevParse resolves `ref` to a 40-character commit SHA via
// `git rev-parse <ref>`. Returns ("", nil) on a clean exit-128 (ref
// does not exist — typical for `HEAD@{1}` on a fresh clone with no
// reflog), and an error on any other git failure. The empty-string
// "missing ref" path is exposed because some callers
// (feat-amend-dependent-warning record gate, v0.7.0) treat a missing
// reflog as "skip the gate" rather than a hard error.
func RevParse(repoRoot, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		// `--verify --quiet` returns exit 1 when the ref does not
		// exist. Treat that as "missing" rather than an error so
		// callers can branch on "no signal available".
		if exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("git rev-parse %s: %s", ref, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return "", err
}

// MergeBase returns the merge-base (common ancestor) of two commits.
// Used as the "base" side of the three-way reconciliation triple. If
// no common ancestor exists (disjoint histories), the returned commit
// is empty and err is non-nil.
func MergeBase(repoRoot, a, b string) (string, error) {
	out, err := runGit(repoRoot, "merge-base", a, b)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// FilesInPatch returns the unique set of repo-relative file paths
// touched by a unified diff, parsed from `diff --git a/<path> b/<path>`
// headers. Paths are returned in first-seen order so the output is
// stable.
//
// It is deliberately FAIL-SOFT: it splits each `diff --git` header on
// the first ` b/` and silently skips any header it cannot split, which
// includes every Git C-quoted path (spaces plus a control byte, a
// quote, a backslash, or a newline). Callers that derive a WRITE SCOPE
// from a patch must NOT use it — an unparseable header would degrade to
// an empty scope, and an empty scope means "everything" to `git diff`.
// Those callers use FilesInPatchStrict instead
// (workflow.RefreshAfterAccept, cli.computePathSet).
//
// The remaining fail-soft callers are advisory only and intentionally
// stay on this function (GH #7 rev-3 F2 audit):
//
//   - workflow.touchedPathsFromPostApplyPatch — feeds the D10
//     migration-hint suppression check, which is documented fail-soft
//     ("absence of data is not evidence of cumulative recording");
//   - workflow.AppendPatchGenerationForFeature — fills the advisory
//     `touched` audit field in patch-generations.json.
//
// Neither drives a write, a diff scope, or a staging decision, so a
// dropped quoted path understates a hint or an audit list rather than
// widening what tpatch touches.
func FilesInPatch(patch string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		// `diff --git a/<p1> b/<p2>` — we take the b-side since new
		// files have /dev/null on the a-side. Handle quoted paths
		// loosely; git doesn't quote unless the path needs it.
		parts := strings.SplitN(line, " b/", 2)
		if len(parts) != 2 {
			continue
		}
		p := strings.TrimSpace(parts[1])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// ForwardApplyExcluding applies a patch but skips the listed
// repo-relative paths entirely. Uses `git apply --3way` with
// `--exclude=<path>` so that non-excluded hunks land cleanly (or via
// 3-way merge) while the excluded paths are left untouched for a
// subsequent explicit overwrite step (e.g., CopyShadowToReal after
// phase 3.5). Returns an error only if the non-excluded portion of
// the patch fails to apply.
func ForwardApplyExcluding(repoRoot, patch string, excludePaths []string) error {
	args := []string{"apply", "--3way"}
	for _, p := range excludePaths {
		args = append(args, "--exclude="+p)
	}
	args = append(args, "-")
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply (excluding %d path(s)) failed: %s: %w",
			len(excludePaths), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// DiffFromCommitForPaths returns `git diff <commit> -- <paths...>`
// against the working tree. Empty paths slice means the full diff.
// Before running the diff, untracked paths in the provided list are
// marked via `git add -N` (intent-to-add) so they appear in the diff
// output; this mirrors how `tpatch record` handles newly created
// files. Used by the reconcile --accept derived-refresh flow to
// regenerate post-apply.patch after the accepted resolution.
//
// IMPORTANT: the intent-to-add markers are written to a *temporary*
// index (via GIT_INDEX_FILE), not the user's real .git/index. This
// guarantees that callers do not leak intent-to-add entries into the
// user's working state after the function returns — a bug from prior
// versions that left `git status` dirty after reconcile --accept /
// refresh. The temp file is removed before return.
//
// GH #7 rev-2: nested registered linked worktrees are excluded here
// too. Discovery runs FIRST, before the temp index is created or any
// `git add -N` is issued, so a discovery failure cannot leave a
// half-mutated temp index behind — and, at the caller layer, cannot
// happen after an artifact has already been written. Caller paths that
// name a nested worktree are dropped before the intent-to-add pass,
// and `:(exclude,literal)` pathspecs are appended to the diff itself so
// index residue cannot re-admit the worktree.
//
// When every caller-supplied path is a nested worktree the result is an
// EMPTY diff, never a broadened full-tree diff: silently widening the
// scope would be a worse failure than returning nothing.
//
// The historical global `--literal-pathspecs` flag is replaced by the
// per-pathspec `:(literal)` form, which has identical semantics for the
// caller's paths while leaving pathspec magic enabled so the exclude
// entries are honoured.
func DiffFromCommitForPaths(repoRoot, commit string, paths []string) (string, error) {
	nestedExcludes, nestedPrefixes, err := nestedWorktreeCaptureFilters(repoRoot)
	if err != nil {
		return "", err
	}
	scoped := len(paths) > 0
	if scoped {
		paths = FilterNestedWorktreePaths(paths, nestedPrefixes)
		if len(paths) == 0 {
			// The caller asked only for nested-worktree paths. Return
			// nothing rather than falling through to a full-tree diff.
			return "", nil
		}
	}

	var env []string
	if len(paths) > 0 {
		tmpIdx, err := os.CreateTemp("", "tpatch-idx-*")
		if err != nil {
			return "", fmt.Errorf("create temp index: %w", err)
		}
		tmpPath := tmpIdx.Name()
		_ = tmpIdx.Close()
		defer os.Remove(tmpPath)

		// Seed from Git's effective index. Linked worktrees store `.git`
		// as a file and keep their index under the common repository's
		// worktrees/ directory; GIT_INDEX_FILE may redirect it again.
		indexOut, indexErr := runGit(repoRoot, "rev-parse", "--git-path", "index")
		if indexErr != nil {
			return "", fmt.Errorf("resolve effective git index: %w", indexErr)
		}
		realIndex := strings.TrimSpace(indexOut)
		if !filepath.IsAbs(realIndex) {
			realIndex = filepath.Join(repoRoot, realIndex)
		}
		data, readErr := os.ReadFile(realIndex)
		if readErr != nil {
			return "", fmt.Errorf("read effective git index %q: %w", realIndex, readErr)
		}
		if werr := os.WriteFile(tmpPath, data, 0o644); werr != nil {
			return "", fmt.Errorf("seed temp index: %w", werr)
		}

		env = append(os.Environ(), "GIT_INDEX_FILE="+tmpPath)

		// Intent-to-add against the TEMP index. Non-fatal: the
		// intent-to-add only helps NEW files; if git refuses (e.g.,
		// file doesn't exist), the diff call below will still see
		// tracked-file changes.
		addArgs := append([]string{"--literal-pathspecs", "add", "-N", "--"}, paths...)
		addCmd := exec.Command("git", addArgs...)
		addCmd.Dir = repoRoot
		addCmd.Env = env
		_, _ = addCmd.CombinedOutput()
	}
	args := []string{"diff", commit}
	if len(paths) > 0 || len(nestedExcludes) > 0 {
		args = append(args, "--")
		args = append(args, nestedExcludes...)
		for _, p := range paths {
			args = append(args, ":(literal)"+p)
		}
	}
	diffCmd := exec.Command("git", args...)
	diffCmd.Dir = repoRoot
	if env != nil {
		diffCmd.Env = env
	}
	out, err := diffCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return "", err
	}
	result := strings.TrimSpace(string(out))
	if result != "" {
		result += "\n"
	}
	return result, nil
}

// DeriveIncrementalPatch computes the diff that only contains one feature's changes,
// given the cumulative patches for the previous features and the current feature.
// prevCumulativePatch = everything up to (but not including) this feature.
// currentCumulativePatch = everything up to and including this feature.
// Returns only this feature's changes (the delta).
//
// PRD-multi-slug-reconcile-canonical-safety §4.4 D4 / ADR-030 D3
// (v0.12.1): the delta-worktree pipeline MUST NOT emit `.git/**`
// paths. Plain `diff -ruN prevDir currDir` on two independent
// `git clone --no-checkout` clones deterministically diffs
// `.git/logs/HEAD`, `.git/logs/refs/...`, and binary `.git/index`
// because reflog writes and index bytes carry wall-clock timestamps
// that differ between clones. Two enforcement layers:
//
//  1. Diff-boundary exclusion — `diff --exclude=.git` at the subprocess
//     invocation. Portable across GNU/BSD diff on macOS and Linux.
//  2. Post-diff `.git/**` filter — reject any hunk whose file header
//     references `.git/`, `.git\`, or the exact path `.git`. Defense
//     in depth against future GNU/BSD/busybox variance in
//     `--exclude` semantics.
//
// INV-3/INV-6 of PRD §5.
func DeriveIncrementalPatch(repoRoot, baseCommit, prevCumulativePatch, currentCumulativePatch string) (string, error) {
	// Create temp dirs
	tmpDir, err := os.MkdirTemp("", "tpatch-incremental-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	prevDir := filepath.Join(tmpDir, "prev")
	currDir := filepath.Join(tmpDir, "curr")

	// Clone base state into both dirs
	for _, dir := range []string{prevDir, currDir} {
		if _, err := runGit(".", "clone", "--no-checkout", repoRoot, dir); err != nil {
			return "", fmt.Errorf("clone failed: %w", err)
		}
		if _, err := runGit(dir, "checkout", baseCommit); err != nil {
			return "", fmt.Errorf("checkout %s failed: %w", baseCommit, err)
		}
	}

	// Apply previous features' cumulative patch to prevDir
	if prevCumulativePatch != "" {
		cmd := exec.Command("git", "apply", "--3way", "-")
		cmd.Dir = prevDir
		cmd.Stdin = strings.NewReader(prevCumulativePatch)
		cmd.Run() // best-effort
	}

	// Apply current features' cumulative patch to currDir
	if currentCumulativePatch != "" {
		cmd := exec.Command("git", "apply", "--3way", "-")
		cmd.Dir = currDir
		cmd.Stdin = strings.NewReader(currentCumulativePatch)
		cmd.Run() // best-effort
	}

	// Diff the two: this gives only the incremental changes for this
	// feature. `--exclude=.git` fires the D4 diff-boundary exclusion
	// so reflog entries, index bytes, and other repo-internals cannot
	// enter the returned patch text (PRD §4.4 / ADR-030 D3).
	cmd := exec.Command("diff", "-ruN", "--exclude=.git", prevDir, currDir)
	out, _ := cmd.Output()
	result := string(out)

	// Fix paths: replace temp dir paths with relative paths
	result = strings.ReplaceAll(result, prevDir+"/", "a/")
	result = strings.ReplaceAll(result, currDir+"/", "b/")

	// Defense-in-depth post-filter: even with `--exclude=.git` at the
	// diff invocation, drop any residual `.git/**` file stanzas so a
	// GNU/BSD variance in exclusion semantics or a hostile future
	// helper cannot leak repo internals downstream. Refer to PRD
	// §4.4 D4 second-fallback for the exact contract.
	result = stripGitInternalFileStanzas(result)

	trimmed := strings.TrimSpace(result)
	if trimmed != "" {
		trimmed += "\n"
	}
	return trimmed, nil
}

// stripGitInternalFileStanzas removes any `Only in …/.git`, `diff -ruN`,
// `Binary files`, or unified-diff stanza whose file path starts with
// `.git/` or is exactly `.git`. Defensive post-filter for
// DeriveIncrementalPatch (PRD-multi-slug-reconcile-canonical-safety
// §4.4 D4 / ADR-030 D3 second layer).
//
// The parser is line-oriented: it walks the input a stanza at a time
// where a stanza starts at `diff -`, `Only in `, `Binary files `,
// `--- ` (after a blank line), or `diff --git ` and ends at the next
// stanza boundary or EOF. If any header inside the stanza references
// `.git/**` or `.git`, the whole stanza is dropped.
func stripGitInternalFileStanzas(patch string) string {
	if patch == "" {
		return ""
	}
	// Fast path: no `.git` reference anywhere — return as-is.
	if !strings.Contains(patch, ".git") {
		return patch
	}
	lines := strings.Split(patch, "\n")
	var out []string
	i := 0
	isStanzaStart := func(line string) bool {
		switch {
		case strings.HasPrefix(line, "diff --git "),
			strings.HasPrefix(line, "diff -"),
			strings.HasPrefix(line, "Only in "),
			strings.HasPrefix(line, "Binary files "),
			strings.HasPrefix(line, "--- "):
			return true
		}
		return false
	}
	for i < len(lines) {
		line := lines[i]
		if !isStanzaStart(line) {
			out = append(out, line)
			i++
			continue
		}
		// Collect the stanza: current line + all following lines
		// until the next stanza boundary or EOF.
		start := i
		i++
		for i < len(lines) && !isStanzaStart(lines[i]) {
			i++
		}
		stanza := lines[start:i]
		if stanzaReferencesGitInternal(stanza) {
			continue
		}
		out = append(out, stanza...)
	}
	return strings.Join(out, "\n")
}

// stanzaReferencesGitInternal returns true when any header inside the
// stanza mentions the on-disk `.git/` repo-internals path.
func stanzaReferencesGitInternal(stanza []string) bool {
	if len(stanza) == 0 {
		return false
	}
	for _, line := range stanza {
		if headerPathIsGitInternal(line) {
			return true
		}
	}
	return false
}

// headerPathIsGitInternal returns true iff `line` is a patch/diff
// header that references `.git/**` or the exact path `.git`. It
// recognises the four header shapes tpatch produces or consumes:
//
//	diff --git a/<path> b/<path>
//	diff -ruN a/<path> b/<path>            (generic unified)
//	--- a/<path>                            or /dev/null
//	+++ b/<path>                            or /dev/null
//	Only in <dir>: .git                     (GNU diff summary)
//	Binary files a/<path> and b/<path> differ
//
// The path is extracted per shape and normalised (leading a/ or b/
// stripped) before the `.git` check.
func headerPathIsGitInternal(line string) bool {
	switch {
	case strings.HasPrefix(line, "diff --git "):
		// diff --git a/foo b/foo
		rest := strings.TrimPrefix(line, "diff --git ")
		fields := strings.Fields(rest)
		for _, f := range fields {
			if pathIsGitInternal(stripABPrefix(f)) {
				return true
			}
		}
	case strings.HasPrefix(line, "diff -"):
		// diff -ruN prevDir/foo currDir/foo  or  diff -ruN a/foo b/foo
		fields := strings.Fields(line)
		for _, f := range fields {
			if pathIsGitInternal(stripABPrefix(f)) {
				return true
			}
		}
	case strings.HasPrefix(line, "--- "):
		p := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
		p = strings.SplitN(p, "\t", 2)[0]
		if p != "/dev/null" && pathIsGitInternal(stripABPrefix(p)) {
			return true
		}
	case strings.HasPrefix(line, "+++ "):
		p := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
		p = strings.SplitN(p, "\t", 2)[0]
		if p != "/dev/null" && pathIsGitInternal(stripABPrefix(p)) {
			return true
		}
	case strings.HasPrefix(line, "Only in "):
		// Only in /path/to/dir: .git   OR   Only in /path/to/dir/.git: HEAD
		rest := strings.TrimPrefix(line, "Only in ")
		if idx := strings.LastIndex(rest, ": "); idx >= 0 {
			dir := rest[:idx]
			leaf := strings.TrimSpace(rest[idx+2:])
			if leaf == ".git" || strings.Contains(dir, "/.git") || strings.HasSuffix(dir, "/.git") || strings.Contains(dir, "/.git/") {
				return true
			}
		}
	case strings.HasPrefix(line, "Binary files "):
		fields := strings.Fields(line)
		for _, f := range fields {
			if pathIsGitInternal(stripABPrefix(f)) {
				return true
			}
		}
	}
	return false
}

func stripABPrefix(p string) string {
	if strings.HasPrefix(p, "a/") {
		return strings.TrimPrefix(p, "a/")
	}
	if strings.HasPrefix(p, "b/") {
		return strings.TrimPrefix(p, "b/")
	}
	return p
}

// pathIsGitInternal returns true when `p` is exactly `.git`, or
// starts with `.git/`, or contains `/.git/`, or ends with `/.git`.
// Backslash variants are also recognised to defeat Windows-style
// paths that may still hit the check on cross-platform hosts.
func pathIsGitInternal(p string) bool {
	if p == "" {
		return false
	}
	if p == ".git" || p == ".git/" || p == ".git\\" {
		return true
	}
	if strings.HasPrefix(p, ".git/") || strings.HasPrefix(p, ".git\\") {
		return true
	}
	if strings.Contains(p, "/.git/") || strings.Contains(p, "\\.git\\") {
		return true
	}
	if strings.HasSuffix(p, "/.git") || strings.HasSuffix(p, "\\.git") {
		return true
	}
	return false
}

// SymbolicRef returns the symbolic target of `ref`, e.g. given
// "refs/remotes/origin/HEAD" it returns "refs/remotes/origin/main".
// Returns ("", nil) when the ref is missing or not symbolic (exit 1
// from `git symbolic-ref --quiet`), and a wrapped error on real
// failure. Used by record --auto to discover the default branch of a
// remote without guessing `main` vs `master`.
func SymbolicRef(repoRoot, ref string) (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--quiet", ref)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("git symbolic-ref %s: %s", ref, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return "", err
}

// CommitCountInRange returns the number of commits in `<base>..<tip>`
// via `git rev-list --count`. Used by record --auto to count how many
// commits are ahead of the resolved baseline so the decision line can
// surface it and so the merge-base safety gate can refuse multi-commit
// fallback ranges.
func CommitCountInRange(repoRoot, base, tip string) (int, error) {
	out, err := runGit(repoRoot, "rev-list", "--count", base+".."+tip)
	if err != nil {
		return 0, err
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(out), "%d", &n); err != nil {
		return 0, fmt.Errorf("parse rev-list --count: %v", err)
	}
	return n, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), fmt.Errorf("git %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return string(out), err
	}
	return string(out), nil
}
