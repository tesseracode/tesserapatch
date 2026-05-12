package workflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// Patch-already-upstream detector (PRD-patch-already-upstream-detector,
// M17 Wave D). The detector is default-OFF gated on
// Config.PatchIDDetectorEnabled. These tests cover:
//
//  1. Default-OFF preservation: with the flag false, reconcile never
//     emits phase-1.5 and PatchIDMatch never appears on the result.
//  2. Detector primitive matches an upstream cherry-pick.
//  3. Detector primitive returns Skipped (not no-match) on missing lock.
//  4. Detector primitive returns no match when the range is empty.
//  5. Detector primitive returns Skipped when the range exceeds the cap.
//  6. End-to-end reconcile fast-path: flag on, phase 1 cannot
//     reverse-apply, phase 1.5 short-circuits to ReconcileUpstreamed
//     with Phase="phase-1.5-patch-id-match" and a populated
//     PatchIDMatch payload. Phases 2/3/4 are skipped.
//  7. Config YAML parser round-trips the new keys.

// writeLock overwrites .tpatch/upstream.lock with a minimal fixture.
func writeLock(t *testing.T, root, remote, branch, commit string) {
	t.Helper()
	body := []byte(
		"remote: \"" + remote + "\"\n" +
			"branch: \"" + branch + "\"\n" +
			"commit: \"" + commit + "\"\n" +
			"url: \"\"\n",
	)
	path := filepath.Join(root, ".tpatch", "upstream.lock")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write upstream.lock: %v", err)
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
	return strings.TrimSpace(string(out))
}

// TestPatchIDDetector_DefaultOffNoOp verifies that with the flag off
// (the package default), reconcile of a feature whose patch matches an
// upstream cherry-pick falls through phase 1.5 entirely. The verdict
// must come from another phase and PatchIDMatch must be nil. This is
// the load-bearing back-compat guarantee for the M17 Wave D rollout.
func TestPatchIDDetector_DefaultOffNoOp(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Confirm the default is OFF.
	cfg, _ := s.LoadConfig()
	if cfg.PatchIDDetectorEnabled {
		t.Fatalf("default config must keep PatchIDDetectorEnabled false; got true")
	}

	s.AddFeature(store.AddFeatureInput{Title: "Add greeting", Request: "Add greeting file"})
	s.MarkFeatureState("add-greeting", store.StateApplied, "apply", "")
	patch := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	s.WriteArtifact("add-greeting", "post-apply.patch", patch)

	// Apply the patch to the working tree so phase 1 reverse-apply
	// succeeds — the default-OFF assertion is "phase != phase-1.5",
	// not "must reach phase 4".
	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "absorb greeting upstream")

	results, err := RunReconcile(context.Background(), s, []string{"add-greeting"}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]
	if strings.HasPrefix(got.Phase, "phase-1.5") {
		t.Fatalf("default-OFF must not reach phase-1.5; got phase=%s", got.Phase)
	}
	if got.PatchIDMatch != nil {
		t.Fatalf("default-OFF must not populate PatchIDMatch; got %+v", got.PatchIDMatch)
	}
}

// TestPatchIDDetector_MatchPrimitive verifies the detector primitive
// finds a match when the upstream range contains a commit with an
// equal `git patch-id --stable`.
func TestPatchIDDetector_MatchPrimitive(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	baseline := git(t, tmpDir, "rev-parse", "HEAD")

	// Make an upstream-side commit that introduces greeting.txt.
	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "upstream absorbs greeting")
	tipCommit := git(t, tmpDir, "rev-parse", "HEAD")

	// The "feature" patch — same diff content the upstream commit
	// introduces. `git patch-id --stable` should agree.
	patch := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`

	writeLock(t, tmpDir, "origin", "main", baseline)

	det := runPatchIDDetector(s, patch, tipCommit, 0)
	if det.Skipped {
		t.Fatalf("detector unexpectedly skipped: %s", det.SkipReason)
	}
	if det.Match == nil {
		t.Fatalf("expected patch-id match, got none")
	}
	if det.Match.MatchedUpstreamSHA != tipCommit {
		t.Fatalf("expected matched SHA %s, got %s", tipCommit, det.Match.MatchedUpstreamSHA)
	}
	if det.Match.ScannedCount != 1 {
		t.Fatalf("expected scanned_count=1, got %d", det.Match.ScannedCount)
	}
	if det.Match.OurPatchID == "" {
		t.Fatalf("OurPatchID must be populated")
	}
	if !strings.Contains(det.Match.ScannedRange, baseline) {
		t.Fatalf("ScannedRange %q must include baseline %s", det.Match.ScannedRange, baseline)
	}
}

// TestPatchIDDetector_NoMatchPrimitive verifies that a range populated
// with non-matching commits returns Match=nil and Skipped=false.
func TestPatchIDDetector_NoMatchPrimitive(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	baseline := git(t, tmpDir, "rev-parse", "HEAD")

	// Upstream commits something unrelated.
	os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("unrelated\n"), 0o644)
	gitAdd(t, tmpDir, "other.txt")
	gitCommit(t, tmpDir, "unrelated change")
	tipCommit := git(t, tmpDir, "rev-parse", "HEAD")

	writeLock(t, tmpDir, "origin", "main", baseline)

	patch := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	det := runPatchIDDetector(s, patch, tipCommit, 0)
	if det.Skipped {
		t.Fatalf("detector unexpectedly skipped: %s", det.SkipReason)
	}
	if det.Match != nil {
		t.Fatalf("expected no match, got %+v", det.Match)
	}
}

// TestPatchIDDetector_MissingLock verifies the detector skips (not
// "no-match") when upstream.lock is absent. The skip reason must
// nudge the operator toward `tpatch upstream pin`.
func TestPatchIDDetector_MissingLock(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	// store.Init creates upstream.lock as scaffolding (empty commit).
	// Delete it to simulate a pre-Wave-A2 repo or operator who removed it.
	os.Remove(filepath.Join(tmpDir, ".tpatch", "upstream.lock"))

	tipCommit := git(t, tmpDir, "rev-parse", "HEAD")
	det := runPatchIDDetector(s, "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -0,0 +1 @@\n+x\n", tipCommit, 0)
	if !det.Skipped {
		t.Fatalf("expected Skipped=true on missing lock; got %+v", det)
	}
	if det.Match != nil {
		t.Fatalf("Skipped run must not produce a Match; got %+v", det.Match)
	}
	if !strings.Contains(det.SkipReason, "upstream.lock") {
		t.Fatalf("SkipReason should mention upstream.lock; got %q", det.SkipReason)
	}
}

// TestPatchIDDetector_EmptyLockCommit verifies that a scaffolded
// upstream.lock with no commit baseline (the `tpatch init` default)
// produces a Skip with a hint, not a no-match.
func TestPatchIDDetector_EmptyLockCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	// Default-init lock has empty commit field; that's already the
	// state after store.Init. Confirm via load:
	lock, _ := store.LoadUpstreamLock(s)
	if lock.Commit != "" {
		t.Fatalf("test fixture invariant broken: expected empty commit field, got %q", lock.Commit)
	}
	tipCommit := git(t, tmpDir, "rev-parse", "HEAD")
	det := runPatchIDDetector(s, "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -0,0 +1 @@\n+x\n", tipCommit, 0)
	if !det.Skipped {
		t.Fatalf("expected Skipped on empty commit baseline; got %+v", det)
	}
}

// TestPatchIDDetector_EmptyRange verifies that when the lock baseline
// equals the upstream tip, the detector reports no match (no commits
// to scan) without skipping. This is the "freshly-pinned upstream"
// fast-path: phase 1.5 has nothing to do but reconcile should still
// proceed to phase 2.
func TestPatchIDDetector_EmptyRange(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	head := git(t, tmpDir, "rev-parse", "HEAD")
	writeLock(t, tmpDir, "origin", "main", head)

	det := runPatchIDDetector(s, "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -0,0 +1 @@\n+x\n", head, 0)
	if det.Skipped {
		t.Fatalf("empty range should not skip; got SkipReason=%q", det.SkipReason)
	}
	if det.Match != nil {
		t.Fatalf("empty range should produce no match; got %+v", det.Match)
	}
}

// TestPatchIDDetector_ScanLimitExceeded verifies the cap from PRD §5.2.
// Range exceeding the per-call limit produces a Skip with a hint.
func TestPatchIDDetector_ScanLimitExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	baseline := git(t, tmpDir, "rev-parse", "HEAD")

	// Three trivial upstream commits.
	for i := 0; i < 3; i++ {
		gitCommit(t, tmpDir, "noise")
	}
	tip := git(t, tmpDir, "rev-parse", "HEAD")
	writeLock(t, tmpDir, "origin", "main", baseline)

	det := runPatchIDDetector(s, "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -0,0 +1 @@\n+x\n", tip, 1)
	if !det.Skipped {
		t.Fatalf("expected Skipped when range exceeds cap; got %+v", det)
	}
	if !strings.Contains(det.SkipReason, "patch_id_scan_limit") {
		t.Fatalf("SkipReason should mention patch_id_scan_limit; got %q", det.SkipReason)
	}
}

// TestPatchIDDetector_FlagOnIntegrationMatch is the end-to-end Wave D
// contract: with the flag on, when phase 1 cannot reverse-apply and
// the patch-id sweep matches an upstream commit, reconcile emits
// Outcome=ReconcileUpstreamed via Phase="phase-1.5-patch-id-match"
// without touching phases 2/3/4 and without invoking the provider.
func TestPatchIDDetector_FlagOnIntegrationMatch(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	// Enable the flag for this test.
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	baseline := git(t, tmpDir, "rev-parse", "HEAD")

	// Upstream-side cherry-pick: commit a file that the feature
	// patch will also describe (same content).
	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "upstream absorbed greeting")
	upstreamTip := git(t, tmpDir, "rev-parse", "HEAD")

	// Then upstream removes the file in a follow-up commit, so the
	// recorded "+Hello World" patch can no longer reverse-apply
	// against HEAD (the file is gone). Patch-id of the absorbing
	// commit still matches our patch-id — that's the phase-1.5 win.
	os.Remove(filepath.Join(tmpDir, "greeting.txt"))
	git(t, tmpDir, "add", "-A")
	gitCommit(t, tmpDir, "upstream later removed greeting")
	tip := git(t, tmpDir, "rev-parse", "HEAD")

	writeLock(t, tmpDir, "origin", "main", baseline)

	s.AddFeature(store.AddFeatureInput{Title: "Add greeting", Request: "Add greeting"})
	s.MarkFeatureState("add-greeting", store.StateApplied, "apply", "")
	patch := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	s.WriteArtifact("add-greeting", "post-apply.patch", patch)

	// Sanity-check the fixture: phase 1 must NOT succeed here.
	if ok, _ := gitutil.ReverseApplyCheck(tmpDir, patch); ok {
		t.Fatalf("test fixture invariant broken: reverse-apply should fail (file no longer in HEAD)")
	}

	// Sanity-check: the upstream-tip walk must include the absorbing commit.
	shas, err := gitutil.RevListInRange(tmpDir, baseline, tip)
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	found := false
	for _, sha := range shas {
		if sha == upstreamTip {
			found = true
		}
	}
	if !found {
		t.Fatalf("test fixture invariant broken: absorbing commit %s not in range", upstreamTip)
	}

	// nil provider — phase 1.5 must run without the provider.
	results, err := RunReconcile(context.Background(), s, []string{"add-greeting"}, tip, nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]
	if got.Outcome != store.ReconcileUpstreamed {
		t.Fatalf("expected ReconcileUpstreamed, got %s; notes=%v", got.Outcome, got.Notes)
	}
	if got.Phase != "phase-1.5-patch-id-match" {
		t.Fatalf("expected phase-1.5-patch-id-match, got %s", got.Phase)
	}
	if got.PatchIDMatch == nil {
		t.Fatalf("expected PatchIDMatch payload, got nil")
	}
	if got.PatchIDMatch.MatchedUpstreamSHA != upstreamTip {
		t.Fatalf("expected matched SHA %s, got %s", upstreamTip, got.PatchIDMatch.MatchedUpstreamSHA)
	}
	if got.UpstreamCommit != upstreamTip {
		t.Fatalf("UpstreamCommit should be the matching SHA per PRD §3.1; got %s", got.UpstreamCommit)
	}

	// status.json must reflect the new state + carry PatchIDMatch.
	status, err := s.LoadFeatureStatus("add-greeting")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != store.StateUpstreamMerged {
		t.Fatalf("expected state upstream_merged, got %s", status.State)
	}
	if status.Reconcile.PatchIDMatch == nil {
		t.Fatalf("status.Reconcile.PatchIDMatch must be persisted on phase-1.5 verdict")
	}
	if status.Reconcile.PatchIDMatch.MatchedUpstreamSHA != upstreamTip {
		t.Fatalf("persisted matched SHA mismatch")
	}
}

// TestPatchIDDetector_FlagOnNoMatchFallsThrough verifies that when the
// flag is on but the patch-id sweep does not match, reconcile proceeds
// to the existing phases 2-4 unchanged.
func TestPatchIDDetector_FlagOnNoMatchFallsThrough(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	baseline := git(t, tmpDir, "rev-parse", "HEAD")
	// Unrelated upstream churn — no patch-id match possible.
	os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("unrelated\n"), 0o644)
	gitAdd(t, tmpDir, "other.txt")
	gitCommit(t, tmpDir, "unrelated upstream commit")
	tip := git(t, tmpDir, "rev-parse", "HEAD")
	writeLock(t, tmpDir, "origin", "main", baseline)

	s.AddFeature(store.AddFeatureInput{Title: "Models", Request: "Add models.txt"})
	s.MarkFeatureState("models", store.StateApplied, "apply", "")
	patch := `diff --git a/models.txt b/models.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/models.txt
@@ -0,0 +1 @@
+gpt-4o
`
	s.WriteArtifact("models", "post-apply.patch", patch)

	results, err := RunReconcile(context.Background(), s, []string{"models"}, tip, nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}
	got := results[0]
	if strings.HasPrefix(got.Phase, "phase-1.5") {
		t.Fatalf("phase-1.5 should not produce a verdict for unrelated upstream; got phase=%s", got.Phase)
	}
	if got.PatchIDMatch != nil {
		t.Fatalf("PatchIDMatch must be nil when phase 1.5 does not match")
	}
}

// TestPatchIDDetector_PrefersCanonicalOverIncremental is the rev-1
// regression for M17 Wave D. Per PRD-patch-already-upstream-detector
// §5.1: when both `post-apply.patch` and `incremental.patch` exist,
// phase-1.5 MUST run against the canonical post-apply form. The
// incremental form may match a partial absorption that isn't a real
// merge — passing it to the detector causes a false-positive retire.
//
// Fixture: feature has post-apply.patch (extra.txt + greeting.txt) and
// incremental.patch (greeting.txt only). Upstream absorbs greeting.txt
// only. Pre-fix, phase-1.5 would see incremental and match. Post-fix,
// phase-1.5 sees canonical (multi-file), does NOT match, and falls
// through to phase 2.
func TestPatchIDDetector_PrefersCanonicalOverIncremental(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	baseline := git(t, tmpDir, "rev-parse", "HEAD")

	// Upstream absorbs greeting.txt only (matches incremental, NOT canonical).
	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "upstream absorbs greeting (partial)")
	_ = git(t, tmpDir, "rev-parse", "HEAD") // upstream tip of the partial-absorb commit; matched by incremental but NOT canonical

	// Then upstream removes greeting.txt so phase-1 reverse-apply on
	// the incremental form cannot trivially succeed via the working tree.
	os.Remove(filepath.Join(tmpDir, "greeting.txt"))
	git(t, tmpDir, "add", "-A")
	gitCommit(t, tmpDir, "upstream later removed greeting")
	tip := git(t, tmpDir, "rev-parse", "HEAD")

	writeLock(t, tmpDir, "origin", "main", baseline)

	s.AddFeature(store.AddFeatureInput{Title: "Add greeting and extra", Request: "Add two files"})
	s.MarkFeatureState("add-greeting-and-extra", store.StateApplied, "apply", "")

	// Canonical post-apply.patch: TWO files (extra.txt + greeting.txt).
	canonical := `diff --git a/extra.txt b/extra.txt
new file mode 100644
index 0000000..d00491f
--- /dev/null
+++ b/extra.txt
@@ -0,0 +1 @@
+extra
diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	// Incremental.patch: greeting.txt only — IDENTICAL to upstream's
	// absorbing commit. Pre-fix this is what the detector saw → match.
	incremental := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	s.WriteArtifact("add-greeting-and-extra", "post-apply.patch", canonical)
	s.WriteArtifact("add-greeting-and-extra", "incremental.patch", incremental)

	// Sanity: the two artifacts have different SHA-256s — i.e. this
	// is genuinely the divergence case the PRD warns about.
	if canonical == incremental {
		t.Fatal("test fixture invariant: canonical and incremental must differ")
	}

	results, err := RunReconcile(context.Background(), s, []string{"add-greeting-and-extra"}, tip, nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]

	// The load-bearing assertion: phase-1.5 must NOT fire here.
	if got.Outcome == store.ReconcileUpstreamed && got.Phase == "phase-1.5-patch-id-match" {
		t.Fatalf("regression: phase-1.5 matched the incremental subset; canonical detector contract broken (PRD §5.1). got=%+v", got)
	}
	if strings.HasPrefix(got.Phase, "phase-1.5") {
		t.Fatalf("phase-1.5 should not produce a verdict; got phase=%s", got.Phase)
	}
	if got.PatchIDMatch != nil {
		t.Fatalf("PatchIDMatch must be nil when canonical does not match; got %+v", got.PatchIDMatch)
	}

	// Persisted status must NOT carry a phase-1.5 PatchIDMatch.
	status, err := s.LoadFeatureStatus("add-greeting-and-extra")
	if err != nil {
		t.Fatal(err)
	}
	if status.Reconcile.PatchIDMatch != nil {
		t.Fatalf("status.Reconcile.PatchIDMatch must not be persisted on canonical-no-match path; got %+v", status.Reconcile.PatchIDMatch)
	}
}

// TestPatchIDDetector_CanonicalMatchesEvenWhenIncrementalDiffers is the
// positive companion to the rev-1 regression: when the canonical
// post-apply.patch itself matches an upstream commit, phase-1.5 still
// fires correctly even if a divergent incremental.patch is also present.
// This guards against an over-correction that would skip phase-1.5
// whenever incremental.patch exists.
func TestPatchIDDetector_CanonicalMatchesEvenWhenIncrementalDiffers(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	baseline := git(t, tmpDir, "rev-parse", "HEAD")

	// Upstream absorbs greeting.txt — matches the canonical patch below.
	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "upstream absorbed greeting")
	upstreamTip := git(t, tmpDir, "rev-parse", "HEAD")

	// Upstream removes greeting.txt so phase-1 can't trivially succeed.
	os.Remove(filepath.Join(tmpDir, "greeting.txt"))
	git(t, tmpDir, "add", "-A")
	gitCommit(t, tmpDir, "upstream later removed greeting")
	tip := git(t, tmpDir, "rev-parse", "HEAD")

	writeLock(t, tmpDir, "origin", "main", baseline)

	s.AddFeature(store.AddFeatureInput{Title: "Add greeting", Request: "Add greeting"})
	s.MarkFeatureState("add-greeting", store.StateApplied, "apply", "")

	canonical := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	// Divergent incremental — content the detector MUST NOT use.
	incremental := `diff --git a/unrelated.txt b/unrelated.txt
new file mode 100644
index 0000000..0000001
--- /dev/null
+++ b/unrelated.txt
@@ -0,0 +1 @@
+other
`
	s.WriteArtifact("add-greeting", "post-apply.patch", canonical)
	s.WriteArtifact("add-greeting", "incremental.patch", incremental)

	results, err := RunReconcile(context.Background(), s, []string{"add-greeting"}, tip, nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}
	got := results[0]
	if got.Phase != "phase-1.5-patch-id-match" {
		t.Fatalf("expected phase-1.5-patch-id-match using canonical patch, got %s; notes=%v", got.Phase, got.Notes)
	}
	if got.Outcome != store.ReconcileUpstreamed {
		t.Fatalf("expected ReconcileUpstreamed, got %s", got.Outcome)
	}
	if got.PatchIDMatch == nil || got.PatchIDMatch.MatchedUpstreamSHA != upstreamTip {
		t.Fatalf("expected PatchIDMatch on upstreamTip %s; got %+v", upstreamTip, got.PatchIDMatch)
	}
}

// TestConfigParserRoundTripsPatchIDKeys is a minimal byte-identity guard
// for the new flat YAML keys. Default values (false / 0) must NOT be
// emitted by SaveConfig so pre-Wave-D fixtures round-trip identically.
func TestConfigParserRoundTripsPatchIDKeys(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	if cfg.PatchIDDetectorEnabled || cfg.PatchIDScanLimit != 0 {
		t.Fatalf("freshly-init config must have zero-value patch-id keys; got %+v", cfg)
	}

	// Flip on and save.
	cfg.PatchIDDetectorEnabled = true
	cfg.PatchIDScanLimit = 100
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !got.PatchIDDetectorEnabled {
		t.Fatalf("PatchIDDetectorEnabled did not round-trip")
	}
	if got.PatchIDScanLimit != 100 {
		t.Fatalf("PatchIDScanLimit did not round-trip; got %d", got.PatchIDScanLimit)
	}
}
