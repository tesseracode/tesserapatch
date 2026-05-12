// Package cli — tests for `tpatch land` (M17 Wave C / PRD-tpatch-land).
//
// Each PRD §6 acceptance row is exercised here. Test names cite the
// PRD acceptance number in the leading comment so the supervisor /
// reviewer can map them in one pass.
package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// gitCommitOnly stages and commits the current working-tree edits with
// gpgsign disabled. Returns the new HEAD SHA (40 chars).
func gitCommitOnly(t *testing.T, dir, message string) string {
	t.Helper()
	for _, args := range [][]string{
		{"add", "."},
		{"-c", "commit.gpgsign=false", "commit", "-q", "-m", message},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	return gitHead(t, dir)
}

// gitLastCommitMsg returns the most recent commit's full body (subject
// + body + trailers) for a repo at dir.
func gitLastCommitMsg(t *testing.T, dir string) string {
	t.Helper()
	c := exec.Command("git", "log", "-1", "--pretty=%B")
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %s: %v", out, err)
	}
	return string(out)
}

// gitPorcelain returns `git status --porcelain` output for dir.
func gitPorcelain(t *testing.T, dir string) string {
	t.Helper()
	c := exec.Command("git", "status", "--porcelain")
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %s: %v", out, err)
	}
	return string(out)
}

// setupLandFixture builds a small feature with one tracked file
// modification + one untracked file, returns (tmpDir, slug, baseHead).
// Caller takes it from there. The feature is added but NOT yet
// recorded, so callers can exercise pre-flight + record-from-land.
func setupLandFixture(t *testing.T) (tmpDir, slug, baseHead string) {
	t.Helper()
	tmpDir = t.TempDir()
	gitInitTestRepo(t, tmpDir)
	// Seed a tracked file + a placeholder so we have non-trivial
	// history for the embedded record step to diff against.
	if err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(tmpDir, "src", "tracked.txt"), []byte("v1\n"), 0o644)
	for _, args := range [][]string{
		{"add", "."},
		{"-c", "commit.gpgsign=false", "commit", "-q", "-m", "seed"},
	} {
		c := exec.Command("git", args...)
		c.Dir = tmpDir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	baseHead = gitHead(t, tmpDir)

	if _, _, code := runCmd("init", "--path", tmpDir); code != 0 {
		t.Fatalf("init failed")
	}
	// Commit the scaffolding `tpatch init` writes (.claude/, .cursor/,
	// .github/, .windsurfrules, .tpatch/ skeleton). In real-world
	// usage operators check these in once; doing so in the fixture
	// keeps land's "extras" check honest about FEATURE-touched paths
	// rather than tooling scaffolding noise.
	for _, args := range [][]string{
		{"add", "."},
		{"-c", "commit.gpgsign=false", "commit", "-q", "-m", "tpatch scaffolding"},
	} {
		c := exec.Command("git", args...)
		c.Dir = tmpDir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	baseHead = gitHead(t, tmpDir)

	if _, _, code := runCmd("add", "--path", tmpDir, "Extra button"); code != 0 {
		t.Fatalf("add failed")
	}
	slug = "extra-button"

	// Working-tree edits the embedded record step will capture:
	// modify tracked.txt and add a fresh untracked file.
	os.WriteFile(filepath.Join(tmpDir, "src", "tracked.txt"), []byte("v1\nv2\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "src", "feature.txt"), []byte("hello feature\n"), 0o644)
	return
}

// TestLand_Success_OneCommit_FourTrailers — PRD §6 ac.1, ac.2, ac.3,
// ac.6, ac.7. Happy path: land produces exactly one commit, with the
// four trailers in the documented order, and leaves a clean tree.
func TestLand_Success_OneCommit_FourTrailers(t *testing.T) {
	tmpDir, slug, base := setupLandFixture(t)

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code != 0 {
		t.Fatalf("land failed: stdout=%q stderr=%q", stdout, stderr)
	}

	// ac.1: exactly one commit advance.
	newHead := gitHead(t, tmpDir)
	if newHead == base {
		t.Fatalf("HEAD did not advance (ac.1)")
	}
	parents := exec.Command("git", "rev-list", "--count", base+".."+newHead)
	parents.Dir = tmpDir
	if out, err := parents.CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected exactly 1 commit, got %q (err=%v) (ac.1)", out, err)
	}

	// ac.2 + ac.3 + ac.7: trailer block present, in order.
	msg := gitLastCommitMsg(t, tmpDir)
	want := []string{
		"Tpatch-Feature: " + slug,
		"Tpatch-Patch-SHA: ",
		"Tpatch-Recipe-SHA: ",
		"Tpatch-Base-Commit: ",
	}
	lastIdx := -1
	for _, w := range want {
		idx := strings.Index(msg, w)
		if idx < 0 {
			t.Errorf("commit message missing trailer %q (ac.2/ac.3); msg=%q", w, msg)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("trailer %q out of order (ac.3); msg=%q", w, msg)
		}
		lastIdx = idx
	}

	// Co-authored-by trailer required by repo policy (CLAUDE.md
	// rule 8 / PRD §3.4 "Repo-level trailers").
	if !strings.Contains(msg, "Co-authored-by: Copilot") {
		t.Errorf("missing Co-authored-by trailer; msg=%q", msg)
	}

	// ac.6: working tree clean post-land.
	if got := gitPorcelain(t, tmpDir); strings.TrimSpace(got) != "" {
		t.Errorf("working tree not clean after land: %q (ac.6)", got)
	}

	// ac.7: feature ↔ commit binding via git log --grep.
	g := exec.Command("git", "log", "--grep", "^Tpatch-Feature: "+slug+"$", "--pretty=%H")
	g.Dir = tmpDir
	out, _ := g.CombinedOutput()
	if !strings.Contains(string(out), newHead) {
		t.Errorf("git log --grep did not return landing commit; out=%q (ac.7)", out)
	}
}

// TestLand_DryRun_NoMutation — PRD §6 ac.11. --dry-run must print the
// contract and exit 0 without mutating index, working tree, or .tpatch/.
func TestLand_DryRun_NoMutation(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)

	// Capture pre-state of .tpatch/features/<slug>/ + working tree.
	preStatus := readArtifact(t, tmpDir, slug, "post-apply.patch")
	prePorcelain := gitPorcelain(t, tmpDir)
	preHead := gitHead(t, tmpDir)

	stdout, _, code := runCmdWithError("land", "--path", tmpDir, slug, "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run should exit 0, got %d", code)
	}
	for _, marker := range []string{
		"DRY RUN: tpatch land " + slug,
		"Pre-flight:",
		"Embedded record:",
		"Staging (path set):",
		"Commit:",
		"Tpatch-Feature: " + slug,
		"Tpatch-Patch-SHA:",
		"Tpatch-Recipe-SHA:",
		"Tpatch-Base-Commit:",
		"Post-conditions if you re-run without --dry-run:",
	} {
		if !strings.Contains(stdout, marker) {
			t.Errorf("dry-run output missing %q; got=%q", marker, stdout)
		}
	}

	// No mutation.
	if got := gitPorcelain(t, tmpDir); got != prePorcelain {
		t.Errorf("dry-run mutated working tree:\n  pre:  %q\n  post: %q", prePorcelain, got)
	}
	if got := gitHead(t, tmpDir); got != preHead {
		t.Errorf("dry-run advanced HEAD: %s -> %s", preHead, got)
	}
	if got := readArtifact(t, tmpDir, slug, "post-apply.patch"); got != preStatus {
		t.Errorf("dry-run mutated post-apply.patch")
	}
}

// TestLand_Refuses_NonexistentFeature — PRD §6 ac.12 / §3.2 #1.
func TestLand_Refuses_NonexistentFeature(t *testing.T) {
	tmpDir := t.TempDir()
	gitInitTestRepo(t, tmpDir)
	if _, _, code := runCmd("init", "--path", tmpDir); code != 0 {
		t.Fatalf("init failed")
	}
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, "no-such-slug")
	if code == 0 {
		t.Fatalf("expected refusal for missing feature; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "no-such-slug") {
		t.Errorf("refusal should mention slug; stderr=%q", stderr)
	}
}

// TestLand_Refuses_ConflictMarkers — PRD §6 ac.12 / §3.2 #2.
func TestLand_Refuses_ConflictMarkers(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	// Inject a conflict marker into a tracked file.
	os.WriteFile(filepath.Join(tmpDir, "src", "tracked.txt"),
		[]byte("ours\n<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> their\n"), 0o644)

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code == 0 {
		t.Fatalf("expected refusal for conflict markers; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "conflict markers") {
		t.Errorf("refusal should mention conflict markers; stderr=%q", stderr)
	}
}

// TestLand_Refuses_MergeLeftovers — PRD §6 ac.12 / §3.2 #3.
func TestLand_Refuses_MergeLeftovers(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	os.WriteFile(filepath.Join(tmpDir, "stale.orig"), []byte("leftover\n"), 0o644)

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code == 0 {
		t.Fatalf("expected refusal for .orig leftover; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "merge leftover") {
		t.Errorf("refusal should mention merge leftover; stderr=%q", stderr)
	}
}

// TestLand_Refuses_MidMerge — PRD §6 ac.12 / §3.2 #4.
func TestLand_Refuses_MidMerge(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	// Simulate mid-merge by creating .git/MERGE_HEAD.
	os.WriteFile(filepath.Join(tmpDir, ".git", "MERGE_HEAD"), []byte("dead\n"), 0o644)

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code == 0 {
		t.Fatalf("expected refusal for mid-merge; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "mid-merge") {
		t.Errorf("refusal should mention mid-merge; stderr=%q", stderr)
	}
}

// TestLand_NotRefused_DirtyOrUntrackedInScope — PRD §6 ac.12 explicit
// non-refusal: dirty/untracked files in the feature's path set must
// NOT cause a refusal. The happy-path test already covers this
// implicitly; this test asserts the explicit non-refusal contract.
func TestLand_NotRefused_DirtyOrUntrackedInScope(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	// Working tree is dirty (modified tracked.txt) and has an
	// untracked file (feature.txt). Both are in scope. Land must
	// proceed without refusal.
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code != 0 {
		t.Fatalf("land must NOT refuse for dirty/untracked in-scope files; stderr=%q", stderr)
	}
}

// TestLand_RefusesExtras_StrictByDefault — PRD §6 ac.4 + §3.3 step 4b.
// A working-tree edit OUTSIDE the feature's path set refuses without
// --allow-extra-paths.
func TestLand_RefusesExtras_StrictByDefault(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	// Make an unrelated edit OUTSIDE the slug's recorded scope.
	// We narrow the embedded record to src/feature.txt only via
	// --files, then leave src/tracked.txt dirty as the extra.
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--files", "src/feature.txt")
	if code == 0 {
		t.Fatalf("expected refusal for extra path; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "outside the feature's path set") {
		t.Errorf("refusal should name the path-set scope; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "src/tracked.txt") {
		t.Errorf("refusal should list the extra path; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "--allow-extra-paths") {
		t.Errorf("refusal should mention --allow-extra-paths escape hatch; stderr=%q", stderr)
	}
	// No commit was created.
	c := exec.Command("git", "log", "--grep", "^Tpatch-Feature:", "--pretty=%H")
	c.Dir = tmpDir
	out, _ := c.CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("no Tpatch-Feature commit should exist after refusal; got=%q", out)
	}
}

// TestLand_AllowExtraPaths_Stages — PRD §6 ac.4 + §3.3 step 4a. With
// --allow-extra-paths the extra files are staged and a one-line note
// appears per file.
func TestLand_AllowExtraPaths_Stages(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "src/feature.txt", "--allow-extra-paths")
	if code != 0 {
		t.Fatalf("land --allow-extra-paths failed: %s", stderr)
	}
	if !strings.Contains(stderr, "staging extra path src/tracked.txt") {
		t.Errorf("expected note about staging extra path; stderr=%q", stderr)
	}
	// The commit should include both the in-scope and extra paths.
	c := exec.Command("git", "show", "--name-only", "--pretty=", "HEAD")
	c.Dir = tmpDir
	out, _ := c.CombinedOutput()
	got := string(out)
	if !strings.Contains(got, "src/tracked.txt") {
		t.Errorf("extra path src/tracked.txt missing from commit; got=%q", got)
	}
}

// TestLand_AutoFromMutuallyExclusive — PRD §6 ac.14. --auto and --from
// are mutually exclusive; the embedded record refuses verbatim.
func TestLand_AutoFromMutuallyExclusive(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--auto", "--from", "HEAD")
	if code == 0 {
		t.Fatalf("expected refusal for --auto + --from")
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("expected mutual-exclusion diagnostic; stderr=%q", stderr)
	}
}

// TestLand_NoRecord_RequiresExistingPatch — PRD §3.1 / §3.7. --no-record
// without a prior post-apply.patch refuses cleanly.
func TestLand_NoRecord_RequiresExistingPatch(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
	if code == 0 {
		t.Fatalf("expected refusal for --no-record without prior patch; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "no recorded post-apply.patch") {
		t.Errorf("expected diagnostic about missing patch; stderr=%q", stderr)
	}
}

// TestLand_BaseCommitUnchanged — PRD §6 ac.5. status.json:apply.base_commit
// must NOT be overwritten with the new HEAD by land. It remains the
// value the embedded record step wrote (the lower bound of the captured
// range; for a working-tree capture, the HEAD at record time).
func TestLand_BaseCommitUnchanged(t *testing.T) {
	tmpDir, slug, baseHead := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	newHead := gitHead(t, tmpDir)
	if newHead == baseHead {
		t.Fatalf("HEAD did not advance; cannot test ac.5")
	}
	// Read status.json directly.
	statusPath := filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json")
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	// status.Apply.BaseCommit should be baseHead (record's HEAD at
	// capture time), NOT newHead.
	if !strings.Contains(body, baseHead) {
		t.Errorf("status.json should retain base_commit=%s (the pre-land HEAD); body=%q", baseHead, body)
	}
	if strings.Contains(body, "\"base_commit\": \""+newHead+"\"") {
		t.Errorf("status.json must NOT carry the new HEAD as base_commit (ac.5); body=%q", body)
	}
}

// TestLand_StatusNotesUpdated — PRD §3.6. After success, status.notes
// records a "landed at <ts>" entry. The new HEAD's SHA is NOT written
// (F2 — chicken-and-egg).
func TestLand_StatusNotesUpdated(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	newHead := gitHead(t, tmpDir)
	statusPath := filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json")
	raw, _ := os.ReadFile(statusPath)
	body := string(raw)
	if !strings.Contains(body, "landed at ") {
		t.Errorf("status.notes should contain 'landed at <ts>'; body=%q", body)
	}
	// Defence-in-depth: ensure the new commit's SHA didn't sneak
	// into notes (F2 fix).
	if strings.Contains(body, "\"notes\": \""+newHead) {
		t.Errorf("notes must NOT embed the new landing SHA (F2); body=%q", body)
	}
}

// TestLand_Subject_FromMessage — PRD §3.4 subject precedence #1.
func TestLand_Subject_FromMessage(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--message", "feat: add extra button"); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	msg := gitLastCommitMsg(t, tmpDir)
	if !strings.HasPrefix(msg, "feat: add extra button\n") {
		t.Errorf("expected --message subject; got=%q", msg)
	}
}

// TestLand_Subject_FallbackDerivation — PRD §3.4 subject precedence #4.
// With no --message, no spec.md, and an empty request.md, the fallback
// "feat(<slug>): apply tpatch feature" is used.
func TestLand_Subject_FallbackDerivation(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	// Wipe request.md so the derivation falls through to the
	// fallback. spec.md was never written by `add`.
	reqPath := filepath.Join(tmpDir, ".tpatch", "features", slug, "request.md")
	os.WriteFile(reqPath, []byte(""), 0o644)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	msg := gitLastCommitMsg(t, tmpDir)
	expected := "feat(" + slug + "): apply tpatch feature"
	if !strings.HasPrefix(msg, expected+"\n") {
		t.Errorf("expected fallback subject %q; got %q", expected, msg)
	}
}

// TestLand_TrailerSHAs_DeterministicAndAccurate — PRD §3.4 semantics.
// Tpatch-Patch-SHA must equal sha256(post-apply.patch); Tpatch-Recipe-SHA
// must be "none" when no recipe is present.
func TestLand_TrailerSHAs_DeterministicAndAccurate(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	patch := readArtifact(t, tmpDir, slug, "post-apply.patch")
	if patch == "" {
		t.Fatalf("post-apply.patch was not written")
	}
	wantSHA := sha256Hex([]byte(patch))
	msg := gitLastCommitMsg(t, tmpDir)
	if !strings.Contains(msg, "Tpatch-Patch-SHA: "+wantSHA) {
		t.Errorf("Tpatch-Patch-SHA mismatch; want=%s msg=%q", wantSHA, msg)
	}
	// record's recipe autogen wrote apply-recipe.json, so the
	// trailer must carry its sha256 (not "none"). Re-derive the
	// expected hash from the on-disk artifact.
	recipePath := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	recipeRaw, rerr := os.ReadFile(recipePath)
	if rerr != nil {
		// If autogen ever stops emitting a recipe in this fixture,
		// the trailer must be "none" instead. Either is valid by
		// PRD §3.4.
		if !strings.Contains(msg, "Tpatch-Recipe-SHA: none") {
			t.Errorf("Tpatch-Recipe-SHA should be 'none' when no recipe; msg=%q", msg)
		}
	} else {
		wantRecipe := sha256Hex(recipeRaw)
		if !strings.Contains(msg, "Tpatch-Recipe-SHA: "+wantRecipe) {
			t.Errorf("Tpatch-Recipe-SHA mismatch; want=%s msg=%q", wantRecipe, msg)
		}
	}
}

// TestLand_ReRecordRoundTrip — PRD §6 ac.8. After land,
// `tpatch record <slug> --from <previous-HEAD>` re-captures a patch
// byte-identical to the patch produced by the embedded record step.
func TestLand_ReRecordRoundTrip(t *testing.T) {
	tmpDir, slug, baseHead := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	originalPatch := readArtifact(t, tmpDir, slug, "post-apply.patch")
	if originalPatch == "" {
		t.Fatalf("no post-apply.patch produced")
	}

	// Re-record with --from <pre-land HEAD> against the now-landed
	// commit. The patch should be byte-identical (ac.8).
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--from", baseHead, "--lenient"); code != 0 {
		t.Fatalf("re-record failed: %s", stderr)
	}
	rePatch := readArtifact(t, tmpDir, slug, "post-apply.patch")
	if rePatch != originalPatch {
		t.Errorf("re-record patch differs (ac.8 round-trip)\n--- original ---\n%s\n--- re-recorded ---\n%s",
			originalPatch, rePatch)
	}
}

// TestLand_PreCommitHookFailureRecoverable — PRD §3.7 / §7.6. If the
// pre-commit hook fails, land surfaces the hook output and leaves the
// staged index intact for the operator to recover.
func TestLand_PreCommitHookFailureRecoverable(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	// Install a pre-commit hook that always fails with a stderr
	// signature we can grep for.
	hookPath := filepath.Join(tmpDir, ".git", "hooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\necho HOOK_REJECTED >&2\nexit 1\n"
	os.WriteFile(hookPath, []byte(hook), 0o755)

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code == 0 {
		t.Fatalf("expected non-zero exit when pre-commit hook fails")
	}
	if !strings.Contains(stderr, "HOOK_REJECTED") {
		t.Errorf("hook output should be surfaced verbatim; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "--no-record") {
		t.Errorf("recovery hint should mention --no-record retry; stderr=%q", stderr)
	}
	// PRD §3.7: staged index intact. `git diff --cached` should be
	// non-empty.
	c := exec.Command("git", "diff", "--cached", "--name-only")
	c.Dir = tmpDir
	out, _ := c.CombinedOutput()
	if strings.TrimSpace(string(out)) == "" {
		t.Errorf("staged index should be intact for recovery; got empty git diff --cached")
	}
}

// TestLand_NoRecord_RetryAgainstStagedIndex — PRD §3.7. After a
// pre-commit hook is removed, --no-record re-runs the commit step
// against the existing staged index.
func TestLand_NoRecord_RetryAgainstStagedIndex(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	// Land once successfully so a post-apply.patch exists.
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("initial land failed: %s", stderr)
	}
	// Make a trivial extra-touch and run --no-record + --allow-extra-paths.
	os.WriteFile(filepath.Join(tmpDir, "src", "tracked.txt"), []byte("v3\n"), 0o644)
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--no-record", "--allow-extra-paths", "--message", "amend extra")
	if code != 0 {
		t.Fatalf("--no-record retry failed: %s", stderr)
	}
	msg := gitLastCommitMsg(t, tmpDir)
	if !strings.Contains(msg, "Tpatch-Feature: "+slug) {
		t.Errorf("retry commit missing trailer; msg=%q", msg)
	}
}

// TestLand_HelpMentionsContract — PRD §6 ac.16 (skill files cross-link
// land). Cheap sanity check that the help string carries the trailer
// block contract for users running `tpatch land --help`.
func TestLand_HelpMentionsContract(t *testing.T) {
	out, _, code := runCmdWithError("land", "--help")
	if code != 0 {
		t.Fatalf("--help should exit 0")
	}
	for _, want := range []string{
		"Tpatch-Feature:",
		"Tpatch-Patch-SHA:",
		"Tpatch-Recipe-SHA:",
		"Tpatch-Base-Commit:",
		"--allow-extra-paths",
		"--no-record",
		"--dry-run",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--help missing %q", want)
		}
	}
}

// TestLand_Refuses_CrossFeatureCollision — PRD §6 ac.13 (Wave B
// integration). A cross-feature byte-identical patch refusal in the
// embedded record step is surfaced verbatim by land.
func TestLand_Refuses_CrossFeatureCollision(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	if _, _, code := runCmd("init", "--path", tmpDir); code != 0 {
		t.Fatal("init failed")
	}
	// First feature recorded with the canonical fixture range.
	if _, _, code := runCmd("add", "--path", tmpDir, "First"); code != 0 {
		t.Fatal("add first failed")
	}
	if _, stderr, code := recordSameContent(t, tmpDir, "first", baseSha); code != 0 {
		t.Fatalf("baseline record failed: %s", stderr)
	}
	// Second feature: try to land with the same base — record will
	// detect a byte-identical canonical patch collision.
	if _, _, code := runCmd("add", "--path", tmpDir, "Second"); code != 0 {
		t.Fatal("add second failed")
	}
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, "second",
		"--from", baseSha)
	if code == 0 {
		t.Fatalf("expected collision refusal from embedded record; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "byte-identical") {
		t.Errorf("expected byte-identical collision diagnostic; stderr=%q", stderr)
	}
	// No commit should have landed.
	c := exec.Command("git", "log", "--grep", "^Tpatch-Feature: second$", "--pretty=%H")
	c.Dir = tmpDir
	out, _ := c.CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("no commit should have landed on collision; out=%q", out)
	}
}

// TestLand_Refuses_HardParent — PRD §6 ac.15 / §3.2 refusal #7. With
// the features_dependencies flag on and an unsatisfied hard parent,
// `tpatch land <child>` must refuse with the same diagnostic the
// apply-time gate emits (workflow.CheckDependencyGate /
// internal/workflow/dependency_gate.go), exit non-zero, create no
// commit, and leave the working tree (HEAD + tracked content) at the
// pre-land state. Mirrors TestApplyExecute_BlockedByHardDep_FlagOn but
// fired through `land`.
func TestLand_Refuses_HardParent(t *testing.T) {
	tmpDir, childSlug, baseHead := setupLandFixture(t)

	// Add a sibling parent feature; child gets a hard dep on it.
	// The parent is left in StateAnalyzed — the gate (ADR-011 D4 /
	// docs/dependencies.md:96-112) requires applied / upstream_merged.
	if _, _, code := runCmd("add", "--path", tmpDir, "Parent feature"); code != 0 {
		t.Fatal("add parent failed")
	}
	parentSlug := "parent-feature"

	s, err := store.Open(tmpDir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg, _ := s.LoadConfig()
	cfg.FeaturesDependencies = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	parent, _ := s.LoadFeatureStatus(parentSlug)
	parent.State = store.StateAnalyzed
	if err := s.SaveFeatureStatus(parent); err != nil {
		t.Fatalf("SaveFeatureStatus parent: %v", err)
	}
	child, _ := s.LoadFeatureStatus(childSlug)
	child.DependsOn = []store.Dependency{
		{Slug: parentSlug, Kind: store.DependencyKindHard},
	}
	if err := s.SaveFeatureStatus(child); err != nil {
		t.Fatalf("SaveFeatureStatus child: %v", err)
	}

	// Snapshot HEAD + tracked file content before invoking land.
	preHead := gitHead(t, tmpDir)
	if preHead != baseHead {
		t.Fatalf("setup invariant: preHead %q != baseHead %q", preHead, baseHead)
	}
	preTracked, _ := os.ReadFile(filepath.Join(tmpDir, "src", "tracked.txt"))

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, childSlug)
	if code == 0 {
		t.Fatalf("expected non-zero exit when hard parent blocks land; stderr=%q", stderr)
	}

	// Same diagnostic as the apply-time gate (verbatim from
	// workflow.CheckDependencyGate). The error message must name the
	// blocking parent slug and use the canonical wrapper string.
	if !strings.Contains(stderr, "hard parent dependency not applied") {
		t.Errorf("refusal must use the gate's canonical wrapper string; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, parentSlug) {
		t.Errorf("refusal must name blocking parent %q; stderr=%q", parentSlug, stderr)
	}
	if !strings.Contains(stderr, "unsatisfied hard dependency") {
		t.Errorf("refusal must explain unsatisfied hard dependency; stderr=%q", stderr)
	}

	// No commit was created — git rev-parse HEAD unchanged.
	if got := gitHead(t, tmpDir); got != preHead {
		t.Errorf("HEAD must be unchanged after refusal; pre=%s post=%s", preHead, got)
	}
	g := exec.Command("git", "log", "--grep", "^Tpatch-Feature: "+childSlug+"$", "--pretty=%H")
	g.Dir = tmpDir
	out, _ := g.CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("no Tpatch-Feature commit should exist after refusal; out=%q", out)
	}

	// Working tree unchanged — the gate fires before staging /
	// status.notes mutation in land.go (PRD §3.2 #7 + §3.7).
	gotTracked, _ := os.ReadFile(filepath.Join(tmpDir, "src", "tracked.txt"))
	if string(gotTracked) != string(preTracked) {
		t.Errorf("tracked file mutated despite refusal; pre=%q post=%q", preTracked, gotTracked)
	}
	// status.notes must NOT have been written (gate runs at line
	// 112; notes mutation is at line 169 in land.go).
	afterChild, _ := s.LoadFeatureStatus(childSlug)
	if strings.Contains(afterChild.Notes, "landed at ") {
		t.Errorf("status.notes must not record 'landed at' on a refusal; got=%q", afterChild.Notes)
	}
}

// TestLand_DoesNotStageUnrelatedDirtyMetadata — Wave C rev-2 Finding 1.
// PRD §3.3 step 3: `.tpatch/upstream.lock` and `.tpatch/FEATURES.md`
// are swept into the feature commit ONLY if the embedded record step
// modified them. Operator-driven dirty drift on these globals must
// remain in the working tree, not silently absorbed.
func TestLand_DoesNotStageUnrelatedDirtyMetadata(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)

	lockPath := filepath.Join(tmpDir, ".tpatch", "upstream.lock")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf(".tpatch/upstream.lock should exist after init: %v", err)
	}
	pre, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("reading upstream.lock: %v", err)
	}
	sentinel := append([]byte{}, pre...)
	sentinel = append(sentinel, []byte("sentinel-unrelated-lock-drift\n")...)
	if err := os.WriteFile(lockPath, sentinel, 0o644); err != nil {
		t.Fatalf("writing sentinel: %v", err)
	}

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code != 0 {
		t.Fatalf("land should succeed (operator drift on globals must not refuse): stderr=%q", stderr)
	}

	// PRD §3.3 step 3 / ADR-021: pin the exact stderr note string so
	// future refactors can't quietly drop the visibility note that
	// makes the carve-out auditable.
	wantNote := `note: leaving .tpatch/upstream.lock dirty (operator drift outside feature scope; not staged)`
	if !strings.Contains(stderr, wantNote) {
		t.Errorf("stderr must contain canonical carve-out note %q; stderr=%q", wantNote, stderr)
	}

	c := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
	c.Dir = tmpDir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff-tree: %s: %v", out, err)
	}
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, f := range files {
		if f == ".tpatch/upstream.lock" {
			t.Errorf("commit must NOT include .tpatch/upstream.lock when record did not touch it; commit files=%v", files)
		}
	}
	hasFeature := false
	for _, f := range files {
		if f == "src/feature.txt" {
			hasFeature = true
		}
	}
	if !hasFeature {
		t.Errorf("commit must still include legitimate feature paths (src/feature.txt); commit files=%v", files)
	}

	porcelain := gitPorcelain(t, tmpDir)
	if !strings.Contains(porcelain, ".tpatch/upstream.lock") {
		t.Errorf("operator's unrelated dirty drift on upstream.lock must remain in working tree; porcelain=%q", porcelain)
	}
	post, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("re-reading upstream.lock: %v", err)
	}
	if !bytes.Contains(post, []byte("sentinel-unrelated-lock-drift")) {
		t.Errorf("sentinel must still be on disk after land; post=%q", post)
	}
}

// TestLand_DryRun_CarvesOutGlobalMetadata — Wave C rev-4 Finding 1.
// PRD §3.5 + §3.6: the dry-run surface must classify drifted global
// metadata under the "Carved-out global metadata" section, NOT under
// "Outside path set", and the post-condition footer must be qualified
// to feature scope. Pre-fix, runLandDryRun called classifyExtras
// directly on the raw dirty set so drifted globals printed under the
// extras-refusal block and the footer promised an unqualified clean
// tree.
func TestLand_DryRun_CarvesOutGlobalMetadata(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)

	// Run an initial land so artifacts (post-apply.patch,
	// status.json) exist and the dry-run path-set computation has
	// something to enumerate. Mirrors TestLand_DryRun_NoMutation.
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("initial land failed: %s", stderr)
	}

	// Drift .tpatch/upstream.lock with a sentinel.
	lockPath := filepath.Join(tmpDir, ".tpatch", "upstream.lock")
	pre, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("reading upstream.lock: %v", err)
	}
	sentinel := append([]byte{}, pre...)
	sentinel = append(sentinel, []byte("sentinel-dryrun-lock-drift\n")...)
	if err := os.WriteFile(lockPath, sentinel, 0o644); err != nil {
		t.Fatalf("writing sentinel: %v", err)
	}

	stdout, _, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record", "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run should exit 0, got %d; stdout=%q", code, stdout)
	}

	// New section heading present.
	carveHeading := "Carved-out global metadata (left dirty in working tree, NOT staged):"
	if !strings.Contains(stdout, carveHeading) {
		t.Errorf("dry-run output missing carve-out section heading %q; got=%q", carveHeading, stdout)
	}

	// Drifted .tpatch/upstream.lock must appear UNDER the carve-out
	// section, NOT under the extras-refusal section.
	extrasHeading := "Outside path set (would refuse without --allow-extra-paths):"
	carveIdx := strings.Index(stdout, carveHeading)
	extrasIdx := strings.Index(stdout, extrasHeading)
	lockIdx := strings.Index(stdout, ".tpatch/upstream.lock")
	if carveIdx < 0 || lockIdx < 0 {
		t.Fatalf("expected carve-out section + upstream.lock listing in dry-run; carveIdx=%d lockIdx=%d", carveIdx, lockIdx)
	}
	// If the extras section is present at all, upstream.lock must
	// not be listed under it. Detect by checking whether the
	// upstream.lock substring falls in the extras window.
	if extrasIdx >= 0 && extrasIdx < lockIdx && lockIdx < carveIdx {
		t.Errorf("upstream.lock listed under extras instead of carve-out; stdout=%q", stdout)
	}

	// Postcondition footer is qualified to feature scope and the
	// unqualified version is gone.
	wantFooter := "Working tree will be clean w.r.t. feature scope."
	if !strings.Contains(stdout, wantFooter) {
		t.Errorf("dry-run output must contain qualified post-condition %q; got=%q", wantFooter, stdout)
	}
	if strings.Contains(stdout, "Working tree will be clean.\n") {
		t.Errorf("dry-run output must NOT contain unqualified post-condition; got=%q", stdout)
	}

	// Carve-out qualifier line about <N> global metadata file(s).
	wantQualifier := "global metadata file(s) will remain dirty with a stderr note"
	if !strings.Contains(stdout, wantQualifier) {
		t.Errorf("dry-run output must contain carve-out qualifier %q; got=%q", wantQualifier, stdout)
	}
}

// TestLand_NoRecord_LeavesCleanWorkingTree — Wave C rev-2 Finding 2.
// PRD §3.6 + §6 ac: after a successful land (with-record OR --no-record)
// the working tree must be clean. The bug was that on the --no-record
// path, status.json was saved AFTER the path-set was computed, so its
// freshly-dirty state was not staged.
func TestLand_NoRecord_LeavesCleanWorkingTree(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)

	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("initial land failed: %s", stderr)
	}
	if got := gitPorcelain(t, tmpDir); strings.TrimSpace(got) != "" {
		t.Fatalf("working tree should be clean after initial land; porcelain=%q", got)
	}

	// Sleep just over a second so the second land produces a
	// different "landed at <RFC3339-second>" notes string and
	// therefore a real status.json content change. Without this,
	// timestamp collision can mask the bug being regressed against.
	time.Sleep(1100 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(tmpDir, "src", "tracked.txt"), []byte("v1\nv2\nv3\n"), 0o644); err != nil {
		t.Fatalf("writing tracked.txt: %v", err)
	}

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--no-record", "--message", "amend")
	if code != 0 {
		t.Fatalf("land --no-record failed: %s", stderr)
	}

	porcelain := gitPorcelain(t, tmpDir)
	if strings.TrimSpace(porcelain) != "" {
		t.Errorf("working tree must be clean after --no-record land (PRD §3.6); porcelain=%q", porcelain)
	}
}
