package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestCheckAppliedOnly_DetectorOnMatch verifies the read-only sweep
// returns a phase-1.5 verdict and writes no artifacts when the detector
// is configured ON and a patch-id match exists upstream.
//
// PRD-patch-already-upstream-detector §3.2.
func TestCheckAppliedOnly_DetectorOnMatch(t *testing.T) {
	tmpDir, slug, tip := setupCheckAppliedFixture(t, true /* upstreamAbsorb */)

	s, _ := store.Init(tmpDir)
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	s.SaveConfig(cfg)

	mustWriteAppliedFeature(t, s, slug)
	writeLockForCheck(t, tmpDir)

	res, err := CheckAppliedOnly(s, slug, tip, false /* forceDetector */)
	if err != nil {
		t.Fatalf("CheckAppliedOnly: %v", err)
	}
	if res.Phase != "phase-1.5-patch-id-match" {
		t.Fatalf("expected phase-1.5-patch-id-match, got %s; notes=%v", res.Phase, res.Notes)
	}
	if res.Outcome != store.ReconcileUpstreamed {
		t.Fatalf("expected ReconcileUpstreamed, got %s", res.Outcome)
	}
	assertNoArtifactsWritten(t, s, slug)
}

// TestCheckAppliedOnly_DetectorOnNoMatch verifies the no-match path
// short-circuits at phase 1.5 with no artifacts.
func TestCheckAppliedOnly_DetectorOnNoMatch(t *testing.T) {
	tmpDir, slug, tip := setupCheckAppliedFixture(t, false /* upstreamAbsorb */)

	s, _ := store.Init(tmpDir)
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	s.SaveConfig(cfg)

	mustWriteAppliedFeature(t, s, slug)
	writeLockForCheck(t, tmpDir)

	res, err := CheckAppliedOnly(s, slug, tip, false)
	if err != nil {
		t.Fatalf("CheckAppliedOnly: %v", err)
	}
	if res.Phase == "phase-1.5-patch-id-match" {
		t.Fatalf("expected no match, got phase=%s", res.Phase)
	}
	if res.Outcome == store.ReconcileUpstreamed {
		t.Fatalf("no-match must not yield ReconcileUpstreamed; got %s", res.Outcome)
	}
	assertNoArtifactsWritten(t, s, slug)
}

// TestCheckAppliedOnly_ForceDetectorWhenDisabled verifies the
// per-invocation override: with `Config.PatchIDDetectorEnabled=false`
// (the persisted default), passing `forceDetector=true` still runs
// phase 1.5 and surfaces the match. This is the read-only opt-in
// semantic for `--check-applied-only`.
func TestCheckAppliedOnly_ForceDetectorWhenDisabled(t *testing.T) {
	tmpDir, slug, tip := setupCheckAppliedFixture(t, true)

	s, _ := store.Init(tmpDir)
	// Confirm the persisted default really is OFF — load-bearing for
	// the override semantic under test.
	cfg, _ := s.LoadConfig()
	if cfg.PatchIDDetectorEnabled {
		t.Fatalf("test fixture invariant: detector default must be off")
	}
	mustWriteAppliedFeature(t, s, slug)
	writeLockForCheck(t, tmpDir)

	res, err := CheckAppliedOnly(s, slug, tip, true /* forceDetector */)
	if err != nil {
		t.Fatalf("CheckAppliedOnly: %v", err)
	}
	if res.Phase != "phase-1.5-patch-id-match" {
		t.Fatalf("force-detector should yield phase-1.5 match even with config off; got phase=%s notes=%v",
			res.Phase, res.Notes)
	}

	// Also confirm forceDetector=false on the same fixture skips it.
	res2, err := CheckAppliedOnly(s, slug, tip, false)
	if err != nil {
		t.Fatalf("CheckAppliedOnly (no force): %v", err)
	}
	if strings.HasPrefix(res2.Phase, "phase-1.5-patch-id-match") {
		t.Fatalf("without force-detector and config off, phase 1.5 must NOT match; got phase=%s", res2.Phase)
	}
}

// setupCheckAppliedFixture builds a git repo where a feature patch
// adds greeting.txt. When upstreamAbsorb is true the upstream commits
// greeting.txt then removes it (so reverse-apply fails but patch-id
// matches the absorbing commit). When false, upstream commits an
// unrelated file (no patch-id match).
//
// Returns: tmpDir, slug, upstream tip commit SHA.
func setupCheckAppliedFixture(t *testing.T, upstreamAbsorb bool) (string, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Pin baseline before any upstream-side commits.
	baseline := git(t, tmpDir, "rev-parse", "HEAD")
	_ = baseline

	if upstreamAbsorb {
		os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
		gitAdd(t, tmpDir, "greeting.txt")
		gitCommit(t, tmpDir, "upstream absorbs greeting")
		os.Remove(filepath.Join(tmpDir, "greeting.txt"))
		git(t, tmpDir, "add", "-A")
		gitCommit(t, tmpDir, "upstream later removed greeting")
	} else {
		os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("unrelated\n"), 0o644)
		gitAdd(t, tmpDir, "other.txt")
		gitCommit(t, tmpDir, "unrelated upstream change")
	}
	tip := git(t, tmpDir, "rev-parse", "HEAD")
	return tmpDir, "add-greeting", tip
}

// mustWriteAppliedFeature creates the feature directory and writes the
// canonical post-apply.patch describing a single-file greeting addition.
func mustWriteAppliedFeature(t *testing.T, s *store.Store, slug string) {
	t.Helper()
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Add greeting", Request: "Add greeting file"}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	s.MarkFeatureState(slug, store.StateApplied, "apply", "")
	patch := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
}

// writeLockForCheck pins upstream.lock to the very first commit
// (HEAD~N) so the rev-list walk includes the absorbing commit. We
// resolve the actual SHA via `git rev-list --max-parents=0 HEAD`
// (root commit) to avoid hard-coding HEAD~ arithmetic.
func writeLockForCheck(t *testing.T, repoRoot string) {
	t.Helper()
	root := git(t, repoRoot, "rev-list", "--max-parents=0", "HEAD")
	writeLock(t, repoRoot, "origin", "main", root)
}

// assertNoArtifactsWritten verifies the read-only contract: no
// reconcile-session.json, no reconcile.md, and feature state is
// untouched (still "applied", not "upstream_merged").
func assertNoArtifactsWritten(t *testing.T, s *store.Store, slug string) {
	t.Helper()
	for _, name := range []string{"reconcile-session.json", "reconcile.md"} {
		if _, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", name)); err == nil {
			t.Fatalf("--check-applied-only must not write artifacts/%s", name)
		}
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus: %v", err)
	}
	if st.State != store.StateApplied {
		t.Fatalf("--check-applied-only must not mutate feature state; got %q want %q", st.State, store.StateApplied)
	}
	if st.Reconcile.Outcome != "" {
		t.Fatalf("--check-applied-only must not write status.Reconcile; got outcome=%q", st.Reconcile.Outcome)
	}
}

// setupPhase1HitFixture builds a git repo where the working tree
// contains greeting.txt with the same content the feature patch adds,
// so phase-1 `git apply -R --check` succeeds. The flag picks whether
// the absorbing upstream commit's patch-id ALSO matches the feature
// patch (phase 1.5 match) or not.
//
// Returns: tmpDir, slug, upstream tip SHA.
func setupPhase1HitFixture(t *testing.T, alsoPhase15Match bool) (string, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	baseline := git(t, tmpDir, "rev-parse", "HEAD")

	if alsoPhase15Match {
		// Single-file commit whose patch-id equals the feature patch-id.
		os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
		gitAdd(t, tmpDir, "greeting.txt")
		gitCommit(t, tmpDir, "upstream absorbs greeting")
	} else {
		// Multi-file commit — same greeting.txt content (so reverse-apply
		// still succeeds) but a different patch-id (extra noise.txt
		// content in the same commit).
		os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
		os.WriteFile(filepath.Join(tmpDir, "noise.txt"), []byte("noise\n"), 0o644)
		gitAdd(t, tmpDir, "greeting.txt")
		gitAdd(t, tmpDir, "noise.txt")
		gitCommit(t, tmpDir, "upstream absorbs greeting bundled with noise")
	}
	tip := git(t, tmpDir, "rev-parse", "HEAD")
	_ = baseline
	return tmpDir, "add-greeting", tip
}

// TestCheckAppliedOnly_Phase1Hit_AlsoPhase15Match — regression for
// external review F2: when phase-1 reverse-apply hits AND the detector
// is on AND phase-1.5 also matches, the result is Upstreamed with the
// MORE SPECIFIC phase-1.5 phase string and a populated PatchIDMatch.
// Notes carry both the phase-1 and phase-1.5 signals.
func TestCheckAppliedOnly_Phase1Hit_AlsoPhase15Match(t *testing.T) {
	tmpDir, slug, tip := setupPhase1HitFixture(t, true /* alsoPhase15Match */)

	s, _ := store.Init(tmpDir)
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	s.SaveConfig(cfg)
	mustWriteAppliedFeature(t, s, slug)
	writeLockForCheck(t, tmpDir)

	res, err := CheckAppliedOnly(s, slug, tip, false)
	if err != nil {
		t.Fatalf("CheckAppliedOnly: %v", err)
	}
	if res.Outcome != store.ReconcileUpstreamed {
		t.Fatalf("outcome=%s notes=%v", res.Outcome, res.Notes)
	}
	if res.Phase != "phase-1.5-patch-id-match" {
		t.Fatalf("expected phase-1.5 upgrade; phase=%s notes=%v", res.Phase, res.Notes)
	}
	if res.PatchIDMatch == nil {
		t.Fatalf("expected PatchIDMatch populated on phase-1.5 upgrade")
	}
	joined := strings.Join(res.Notes, "\n")
	if !strings.Contains(joined, "working tree already contains the patched content") {
		t.Errorf("expected phase-1 diagnostic note in result.Notes:\n%s", joined)
	}
	if !strings.Contains(joined, "Patch-id sweep matched") {
		t.Errorf("expected phase-1.5 match note in result.Notes:\n%s", joined)
	}
}

// TestCheckAppliedOnly_Phase1Hit_Phase15NoMatch — regression for
// external review F3 (rev-2): phase-1 reverse-apply hits, detector is
// on, but phase-1.5 finds no patch-id match. Under --check-applied-only
// the preflight is skipped, so phase-1 success is NOT upstream-scoped
// evidence — phase-1.5 owns the verdict. Outcome must be
// ReconcileStillNeeded and Phase must be "phase-1.5-no-match". The
// phase-1 diagnostic note still appears for operator visibility.
func TestCheckAppliedOnly_Phase1Hit_Phase15NoMatch(t *testing.T) {
	tmpDir, slug, tip := setupPhase1HitFixture(t, false /* alsoPhase15Match */)

	s, _ := store.Init(tmpDir)
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	s.SaveConfig(cfg)
	mustWriteAppliedFeature(t, s, slug)
	writeLockForCheck(t, tmpDir)

	res, err := CheckAppliedOnly(s, slug, tip, false)
	if err != nil {
		t.Fatalf("CheckAppliedOnly: %v", err)
	}
	if res.Outcome != store.ReconcileStillNeeded {
		t.Fatalf("phase-1.5 no-match under --check-applied-only must yield ReconcileStillNeeded; got %s notes=%v", res.Outcome, res.Notes)
	}
	if res.Phase != "phase-1.5-no-match" {
		t.Fatalf("expected Phase=phase-1.5-no-match; got %s notes=%v", res.Phase, res.Notes)
	}
	if res.PatchIDMatch != nil {
		t.Fatalf("PatchIDMatch must be nil when phase-1.5 did not match; got %+v", res.PatchIDMatch)
	}
	joined := strings.Join(res.Notes, "\n")
	if !strings.Contains(joined, "working tree already contains the patched content") {
		t.Errorf("expected phase-1 diagnostic note still present for operator visibility; notes=\n%s", joined)
	}
}

// TestCheckAppliedOnly_Phase1Hit_DetectorOff — regression for external
// review F3 (rev-2): phase-1 reverse-apply hits, detector is OFF on
// disk, and forceDetector=false. Phase 1.5 is the sole upstream-scoped
// signal, so when it's not run the result must be ReconcileStillNeeded
// with Phase="phase-1.5-skipped-detector-disabled". The phase-1
// diagnostic note still appears.
func TestCheckAppliedOnly_Phase1Hit_DetectorOff(t *testing.T) {
	tmpDir, slug, tip := setupPhase1HitFixture(t, true)

	s, _ := store.Init(tmpDir)
	cfg, _ := s.LoadConfig()
	if cfg.PatchIDDetectorEnabled {
		t.Fatalf("test invariant: detector default must be off")
	}
	mustWriteAppliedFeature(t, s, slug)
	writeLockForCheck(t, tmpDir)

	res, err := CheckAppliedOnly(s, slug, tip, false /* forceDetector */)
	if err != nil {
		t.Fatalf("CheckAppliedOnly: %v", err)
	}
	if res.Outcome != store.ReconcileStillNeeded {
		t.Fatalf("detector-off under --check-applied-only must yield ReconcileStillNeeded; got %s", res.Outcome)
	}
	if res.Phase != "phase-1.5-skipped-detector-disabled" {
		t.Fatalf("expected Phase=phase-1.5-skipped-detector-disabled; got %s notes=%v", res.Phase, res.Notes)
	}
	joined := strings.Join(res.Notes, "\n")
	if !strings.Contains(joined, "working tree already contains the patched content") {
		t.Errorf("expected phase-1 diagnostic note still present for operator visibility; notes=\n%s", joined)
	}
}
