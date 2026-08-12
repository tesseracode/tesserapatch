// Capture-mode helpers (PRD-record-capture-modes v1).
//
// CaptureStagedPatch, CaptureUnstagedPatch, StagedUnstagedOverlap and
// ValidateStagedPatch back the new `--staged` / `--unstaged` modes on
// `tpatch record`. They live alongside the existing
// CapturePatchScoped / CapturePatchFromCommitsScoped helpers and
// reuse the same exclude-pathspec list so installed skill surfaces
// stay out of every capture mode.
//
// Refusal policy (empty patches, overlap diagnostics) lives in the
// CLI layer; these helpers return raw patches and structured
// summaries.

package gitutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runGitEnv runs git with extra environment variables (e.g.
// `GIT_INDEX_FILE=...`). Output is discarded; only the error is
// returned. Used by ValidateStagedPatch for the temp-index handshake.
func runGitEnv(dir string, extraEnv []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// runGitEnvStdin runs git with extra environment variables and a
// stdin payload (the patch bytes). Used by ValidateStagedPatch's
// temp-index `git apply --cached --check` invocation.
func runGitEnvStdin(dir string, extraEnv []string, stdin string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// runGitStdin runs git with a stdin payload but no env overrides.
// Used as the fallback live-index --cached --check path.
func runGitStdin(dir, stdin string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewBufferString(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// captureModeExcludes is the per-call copy of the always-excluded
// pathspec list. It mirrors the literal list in CapturePatchScoped so
// staged/unstaged captures inherit the same .tpatch/ and installed-
// skill exclusions byte-for-byte.
func captureModeExcludes() []string {
	return []string{
		":(exclude).tpatch",
		":(exclude).claude/skills",
		":(exclude).github/skills",
		":(exclude).github/prompts",
		":(exclude).cursor/rules",
		":(exclude).windsurfrules",
	}
}

// captureModeSkipPrefixes is used when filtering untracked files for
// --unstaged capture. The list mirrors CapturePatchScoped's
// skipPrefixes so plain untracked files inside reserved areas are
// never staged with --intent-to-add.
var captureModeSkipPrefixes = []string{
	".tpatch/",
	".claude/skills/",
	".github/skills/",
	".github/prompts/",
	".cursor/rules/",
	".windsurfrules",
}

// shouldSkipCapturePath reports whether the given repo-relative path
// falls under one of the always-excluded reserved areas.
func shouldSkipCapturePath(path string) bool {
	for _, prefix := range captureModeSkipPrefixes {
		if strings.HasPrefix(path, prefix) || path == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}
	return false
}

// StagedDirtySummary captures structural counts used by
// `tpatch record --staged` for the `dirty_state` provenance line and
// for the unrelated-unstaged note printed to stderr.
type StagedDirtySummary struct {
	// StagedPaths is the count of distinct paths included in the
	// staged-only patch (HEAD → index).
	StagedPaths int
	// UnrelatedUnstagedPaths is the count of paths that have unstaged
	// edits but DO NOT appear in the staged patch. These trigger an
	// advisory note line but never a refusal.
	UnrelatedUnstagedPaths int
}

// UnstagedDirtySummary mirrors StagedDirtySummary for `--unstaged`.
type UnstagedDirtySummary struct {
	UnstagedPaths        int
	UnrelatedStagedPaths int
}

// CaptureStagedPatch captures the diff from `HEAD` to the index, with
// the same exclude pathspecs as CapturePatchScoped. When pathspecs is
// non-empty the diff is further narrowed.
//
// New files appear in the patch only when they are represented in the
// index (i.e. the user ran `git add <file>` already); plain untracked
// files are never staged here.
//
// The returned StagedDirtySummary is best-effort path-count metadata
// computed off `git diff --name-only` invocations. Refusal logic
// (empty patch, staged∩unstaged overlap) lives in the CLI layer; this
// helper simply returns the bytes and the counts.
func CaptureStagedPatch(repoRoot string, pathspecs []string) (string, StagedDirtySummary, error) {
	var summary StagedDirtySummary

	nestedExcludes, _, err := nestedWorktreeCaptureFilters(repoRoot)
	if err != nil {
		return "", summary, err
	}

	args := append([]string{"diff", "--cached", "HEAD", "--"}, captureModeExcludes()...)
	args = append(args, nestedExcludes...)
	if len(pathspecs) > 0 {
		args = append(args, pathspecs...)
	}
	patch, err := runGit(repoRoot, args...)
	if err != nil {
		if len(pathspecs) > 0 {
			return "", summary, fmt.Errorf("git diff --cached failed for pathspecs %v: %w", pathspecs, err)
		}
		return "", summary, fmt.Errorf("git diff --cached failed: %w", err)
	}

	staged, err := stagedNameOnly(repoRoot, pathspecs)
	if err != nil {
		return "", summary, err
	}
	summary.StagedPaths = len(staged)
	unstaged, err := unstagedNameOnly(repoRoot, pathspecs)
	if err != nil {
		return "", summary, err
	}
	stagedSet := stringSet(staged)
	for _, p := range unstaged {
		if !stagedSet[p] {
			summary.UnrelatedUnstagedPaths++
		}
	}
	return normalizePatchTail(patch), summary, nil
}

// CaptureUnstagedPatch captures the diff from the index to the
// working tree, with the same exclude pathspecs as CapturePatchScoped.
// Plain untracked files (not in the index) are temporarily staged
// with `git add --intent-to-add` so `git diff` surfaces them as new
// additions, then unstaged again at the end.
//
// The returned UnstagedDirtySummary records the count of paths
// included in the unstaged patch plus the count of UNRELATED staged
// paths (paths the user has staged on top of HEAD that do not appear
// in the unstaged patch). Unrelated staged paths trigger an advisory
// note line but never a refusal; overlap is the CLI layer's
// responsibility via StagedUnstagedOverlap.
func CaptureUnstagedPatch(repoRoot string, pathspecs []string) (string, UnstagedDirtySummary, error) {
	var summary UnstagedDirtySummary

	nestedExcludes, _, err := nestedWorktreeCaptureFilters(repoRoot)
	if err != nil {
		return "", summary, err
	}

	untrackedFiles, err := listUntrackedFiles(repoRoot, pathspecs)
	if err != nil {
		return "", summary, err
	}

	var stagedNewFiles []string
	for _, file := range untrackedFiles {
		if shouldSkipCapturePath(file) {
			continue
		}
		if _, err := runGit(repoRoot, "--literal-pathspecs", "add", "--intent-to-add", "--", file); err != nil {
			for _, staged := range stagedNewFiles {
				runGit(repoRoot, "--literal-pathspecs", "reset", "--", staged)
			}
			return "", summary, fmt.Errorf("git add --intent-to-add %q: %w", file, err)
		}
		stagedNewFiles = append(stagedNewFiles, file)
	}

	// Index → worktree diff (i.e. `git diff` without --cached). The
	// intent-to-add markers we just placed surface untracked files
	// here as new additions.
	diffArgs := append([]string{"diff", "--"}, captureModeExcludes()...)
	diffArgs = append(diffArgs, nestedExcludes...)
	if len(pathspecs) > 0 {
		diffArgs = append(diffArgs, pathspecs...)
	}
	patch, err := runGit(repoRoot, diffArgs...)
	if err != nil {
		for _, file := range stagedNewFiles {
			runGit(repoRoot, "--literal-pathspecs", "reset", "--", file)
		}
		if len(pathspecs) > 0 {
			return "", summary, fmt.Errorf("git diff failed for pathspecs %v: %w", pathspecs, err)
		}
		return "", summary, fmt.Errorf("git diff failed: %w", err)
	}

	// Unstage intent-to-add markers so the working tree is restored.
	for _, file := range stagedNewFiles {
		runGit(repoRoot, "--literal-pathspecs", "reset", "--", file)
	}

	// Count paths for provenance / advisory note. Note: for the path
	// count we re-list unstaged names AFTER the intent-to-add markers
	// were removed (so plain untracked additions show up only if they
	// were intent-to-add staged during the diff). We re-stage and
	// re-reset around stagedNameOnly to keep the live tree untouched.
	staged, err := stagedNameOnly(repoRoot, pathspecs)
	if err != nil {
		return "", summary, err
	}
	unstaged, err := unstagedNameOnly(repoRoot, pathspecs)
	if err != nil {
		return "", summary, err
	}
	// Include untracked files in the unstaged path count (they would
	// have appeared in the patch via intent-to-add). We list them
	// post-reset to mirror the "what the user has, minus the index"
	// set.
	untracked, err := untrackedFiltered(repoRoot, pathspecs)
	if err != nil {
		return "", summary, err
	}
	uniqUnstaged := stringSet(unstaged)
	for _, p := range untracked {
		uniqUnstaged[p] = true
	}
	summary.UnstagedPaths = len(uniqUnstaged)

	stagedSet := stringSet(staged)
	for p := range stagedSet {
		if !uniqUnstaged[p] {
			summary.UnrelatedStagedPaths++
		}
	}

	return normalizePatchTail(patch), summary, nil
}

// StagedUnstagedOverlap computes three disjoint path slices used by
// the record CLI to decide whether to refuse (overlap) or emit a
// note-only diagnostic (unrelated edits in the "other" tier):
//
//   - overlap: paths that have BOTH staged and unstaged edits;
//   - unrelatedStaged: paths with staged edits but no unstaged edits;
//   - unrelatedUnstaged: paths with unstaged edits but no staged edits.
//
// "Unstaged" here folds in plain untracked files (filtered by skip
// prefixes and pathspecs). All path lists are repo-relative and
// forward-slash normalized as `git diff --name-only` produces them.
// The returned slices are sorted for stable diagnostic ordering.
func StagedUnstagedOverlap(repoRoot string, pathspecs []string) (overlap, unrelatedStaged, unrelatedUnstaged []string, err error) {
	staged, err := stagedNameOnly(repoRoot, pathspecs)
	if err != nil {
		return nil, nil, nil, err
	}
	unstaged, err := unstagedNameOnly(repoRoot, pathspecs)
	if err != nil {
		return nil, nil, nil, err
	}
	untracked, err := untrackedFiltered(repoRoot, pathspecs)
	if err != nil {
		return nil, nil, nil, err
	}

	stagedSet := stringSet(staged)
	unstagedSet := stringSet(unstaged)
	for _, p := range untracked {
		unstagedSet[p] = true
	}

	for p := range stagedSet {
		if unstagedSet[p] {
			overlap = append(overlap, p)
		} else {
			unrelatedStaged = append(unrelatedStaged, p)
		}
	}
	for p := range unstagedSet {
		if !stagedSet[p] {
			unrelatedUnstaged = append(unrelatedUnstaged, p)
		}
	}
	sortStrings(overlap)
	sortStrings(unrelatedStaged)
	sortStrings(unrelatedUnstaged)
	return overlap, unrelatedStaged, unrelatedUnstaged, nil
}

// ValidateStagedPatch confirms a staged-only patch applies cleanly
// against a temporary index seeded from HEAD. The temp-index path
// avoids interrogating the live working tree (whose state for staged
// validation is irrelevant) and side-steps the "patch is already
// applied" trap that direct `git apply --cached --check` against the
// live index would hit.
//
// If the temp-index handshake cannot be set up (e.g. `git read-tree`
// fails for an unrelated reason), the helper falls back to a direct
// `git apply --cached --check` against the live index. The fallback
// is the PRD-allowed second choice; it is never a silent downgrade to
// a working-tree apply check.
func ValidateStagedPatch(repoRoot, patch string) error {
	if strings.TrimSpace(patch) == "" {
		return fmt.Errorf("validate: empty staged patch")
	}

	tmp, err := os.CreateTemp("", "tpatch-staged-index-*")
	if err == nil {
		tmpPath := tmp.Name()
		_ = tmp.Close()
		defer os.Remove(tmpPath)

		// Seed the temp index from HEAD. Use GIT_INDEX_FILE so the
		// live index is never touched.
		if err := runGitEnv(repoRoot, []string{"GIT_INDEX_FILE=" + tmpPath}, "read-tree", "HEAD"); err == nil {
			if err := runGitEnvStdin(repoRoot, []string{"GIT_INDEX_FILE=" + tmpPath}, patch, "apply", "--cached", "--check"); err == nil {
				return nil
			} else {
				return fmt.Errorf("staged patch fails temp-index --cached --check: %w", err)
			}
		}
		// fallthrough to live-index fallback if read-tree failed.
	}

	if err := runGitStdin(repoRoot, patch, "apply", "--cached", "--check"); err != nil {
		return fmt.Errorf("staged patch fails --cached --check: %w", err)
	}
	return nil
}

// FilterPathsByClaimedDirs takes a list of repo-relative paths
// (forward-slash form, no leading slash) and returns the subset that
// matches any of the supplied claim values. A claim that ends with
// `/` matches any path strictly inside that directory; a claim
// without `/` matches the path verbatim. This is the bulk-filter
// shape used by `--claimed-only`; for single-operand lookups use
// store.MatchClaim (which carries different normalization semantics).
func FilterPathsByClaimedDirs(paths []string, claimValues []string) []string {
	if len(claimValues) == 0 {
		return nil
	}
	var fileClaims []string
	var dirClaims []string
	for _, c := range claimValues {
		if strings.HasSuffix(c, "/") {
			dirClaims = append(dirClaims, c)
		} else {
			fileClaims = append(fileClaims, c)
		}
	}
	fileSet := stringSet(fileClaims)
	var out []string
	for _, p := range paths {
		if fileSet[p] {
			out = append(out, p)
			continue
		}
		for _, d := range dirClaims {
			if strings.HasPrefix(p, d) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// PathspecsForClaims renders a list of claim values into git
// pathspecs suitable for `git diff -- <pathspecs>`. Directory claims
// (trailing-slash) are emitted as the bare directory name so git
// expands them recursively; file claims are emitted verbatim.
func PathspecsForClaims(claimValues []string) []string {
	out := make([]string, 0, len(claimValues))
	for _, c := range claimValues {
		out = append(out, strings.TrimSuffix(c, "/"))
	}
	return out
}

// IntersectPathspecs returns the elements of `explicit` that also
// match (verbatim or under-dir) one of the claim values. Used when
// `--files` and `--claimed-only` are both supplied: the resulting
// pathspec set is the intersection. Empty result means the user's
// explicit filter has no overlap with the claim set, which the CLI
// treats as a refusal condition.
func IntersectPathspecs(explicit []string, claimValues []string) []string {
	if len(explicit) == 0 || len(claimValues) == 0 {
		return nil
	}
	var fileClaims []string
	var dirClaims []string
	for _, c := range claimValues {
		if strings.HasSuffix(c, "/") {
			dirClaims = append(dirClaims, c)
		} else {
			fileClaims = append(fileClaims, c)
		}
	}
	fileSet := stringSet(fileClaims)
	var out []string
	for _, raw := range explicit {
		// Normalize the explicit pathspec to a comparable form: drop
		// any trailing slash for set lookup, keep it for prefix match.
		bare := strings.TrimSuffix(raw, "/")
		if fileSet[bare] {
			out = append(out, raw)
			continue
		}
		matched := false
		for _, d := range dirClaims {
			if bare == strings.TrimSuffix(d, "/") || strings.HasPrefix(bare+"/", d) {
				out = append(out, raw)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// Also allow the converse: explicit is a directory and a
		// claim file sits inside it. We accept the explicit pathspec
		// in that case so git's diff still narrows to it; intersection
		// for filtering happens via git itself.
		if strings.HasSuffix(raw, "/") || isLikelyDir(bare) {
			for _, fc := range fileClaims {
				if strings.HasPrefix(fc, bare+"/") {
					out = append(out, raw)
					break
				}
			}
		}
	}
	return out
}

func isLikelyDir(p string) bool {
	// Conservative heuristic only used as a tie-breaker in
	// IntersectPathspecs: no extension and no slash terminator likely
	// names a directory.
	return !strings.Contains(filepath.Base(p), ".")
}

// stagedNameOnly returns the list of distinct paths that differ
// between HEAD and the index, scoped by the supplied pathspecs (if
// any). The exclude pathspecs match the diff-capture excludes so
// reserved-area entries never appear in overlap diagnostics.
func stagedNameOnly(repoRoot string, pathspecs []string) ([]string, error) {
	nestedExcludes, _, err := nestedWorktreeCaptureFilters(repoRoot)
	if err != nil {
		return nil, err
	}
	args := append([]string{"diff", "--cached", "--name-only", "HEAD", "--"}, captureModeExcludes()...)
	args = append(args, nestedExcludes...)
	if len(pathspecs) > 0 {
		args = append(args, pathspecs...)
	}
	out, err := runGit(repoRoot, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff --cached --name-only failed: %w", err)
	}
	return splitNonEmptyLines(out), nil
}

// unstagedNameOnly returns the list of distinct TRACKED paths that
// differ between the index and the working tree. Untracked files are
// listed separately via untrackedFiltered.
func unstagedNameOnly(repoRoot string, pathspecs []string) ([]string, error) {
	nestedExcludes, _, err := nestedWorktreeCaptureFilters(repoRoot)
	if err != nil {
		return nil, err
	}
	args := append([]string{"diff", "--name-only", "--"}, captureModeExcludes()...)
	args = append(args, nestedExcludes...)
	if len(pathspecs) > 0 {
		args = append(args, pathspecs...)
	}
	out, err := runGit(repoRoot, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only failed: %w", err)
	}
	return splitNonEmptyLines(out), nil
}

// untrackedFiltered lists untracked files filtered by the reserved-
// area skip-prefixes and the supplied pathspecs.
func untrackedFiltered(repoRoot string, pathspecs []string) ([]string, error) {
	files, err := listUntrackedFiles(repoRoot, pathspecs)
	if err != nil {
		return nil, err
	}
	var keep []string
	for _, file := range files {
		if shouldSkipCapturePath(file) {
			continue
		}
		keep = append(keep, file)
	}
	return keep, nil
}

// listUntrackedFiles is the single untracked-discovery entry point for
// every capture mode (default/manual Path B via CapturePatchScoped,
// --unstaged via CaptureUnstagedPatch, and the overlap diagnostics via
// untrackedFiltered). Registered linked worktrees nested under
// repoRoot are subtracted here — before any caller can hand the path
// to `git add --intent-to-add` — so no capture surface can turn an
// agent harness checkout into a `mode 160000` gitlink (GH #7).
//
// The subtraction is applied regardless of pathspecs, so explicitly
// naming the worktree via `record --files <worktree>` cannot re-admit
// it. Discovery failure is safety-relevant and fails closed.
func listUntrackedFiles(repoRoot string, pathspecs []string) ([]string, error) {
	nested, err := NestedWorktreePrefixes(repoRoot)
	if err != nil {
		return nil, NestedWorktreeDiscoveryError(repoRoot, err)
	}
	args := []string{"-c", "core.quotePath=false", "ls-files", "--others", "--exclude-standard", "-z"}
	if len(pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, pathspecs...)
	}
	out, err := runGit(repoRoot, args...)
	if err != nil {
		return nil, fmt.Errorf("git ls-files --others failed: %w", err)
	}
	var files []string
	for _, file := range strings.Split(out, "\x00") {
		if file == "" {
			continue
		}
		if PathUnderNestedWorktree(file, nested) {
			continue
		}
		files = append(files, file)
	}
	return files, nil
}

func splitNonEmptyLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func stringSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

func sortStrings(s []string) {
	// Local insertion sort to avoid pulling sort into the import set
	// for a tiny helper. Test slices are at most a handful of paths.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
