package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// v0.12.0 Wave α (ADR-028 D6, PRD-feature-supersession AC-5, AC-11):
// when a feature is superseded by an active healthy superseder,
// RunReconcile with an empty slug set (default effective replay)
// must silently omit the superseded feature from the reconcile
// sweep.
func TestReconcileDefaultSet_ExcludesSupersededFeature(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Older (soon-to-be superseded) feature.
	s.AddFeature(store.AddFeatureInput{Title: "Add older greeting", Request: "older"})
	s.MarkFeatureState("add-older-greeting", store.StateApplied, "apply", "")

	// Newer feature that supersedes the older one.
	s.AddFeature(store.AddFeatureInput{Title: "Add newer greeting", Request: "newer"})
	s.MarkFeatureState("add-newer-greeting", store.StateApplied, "apply", "")

	// Wire the supersedes edge on the newer feature's status.
	newerStatus, err := s.LoadFeatureStatus("add-newer-greeting")
	if err != nil {
		t.Fatal(err)
	}
	newerStatus.DependsOn = []store.Dependency{
		{Slug: "add-older-greeting", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(newerStatus); err != nil {
		t.Fatal(err)
	}

	// Create a file so the greetings have a persisted change.
	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "add greeting")

	// Default effective replay: empty slug list — should sweep applied
	// features but silently drop the superseded one.
	results, err := RunReconcile(context.Background(), s, nil, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}

	seen := map[string]bool{}
	for _, r := range results {
		seen[r.Slug] = true
	}
	if seen["add-older-greeting"] {
		t.Errorf("default reconcile set should exclude superseded feature add-older-greeting; got results=%v", results)
	}
	if !seen["add-newer-greeting"] {
		t.Errorf("default reconcile set should include active superseder add-newer-greeting; got results=%v", results)
	}
}

// v0.12.0 Wave α (ADR-028 D6, PRD-feature-supersession §3.3, AC-11):
// when the caller explicitly names a superseded feature on the CLI,
// RunReconcile reconciles it (audit/repair path) but prepends a
// historical-feature warning note to the ReconcileResult.
func TestReconcileExplicitSlug_SupersededEmitsHistoricalWarning(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	s.AddFeature(store.AddFeatureInput{Title: "Add older greeting", Request: "older"})
	s.MarkFeatureState("add-older-greeting", store.StateApplied, "apply", "")

	s.AddFeature(store.AddFeatureInput{Title: "Add newer greeting", Request: "newer"})
	s.MarkFeatureState("add-newer-greeting", store.StateApplied, "apply", "")

	newerStatus, err := s.LoadFeatureStatus("add-newer-greeting")
	if err != nil {
		t.Fatal(err)
	}
	newerStatus.DependsOn = []store.Dependency{
		{Slug: "add-older-greeting", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(newerStatus); err != nil {
		t.Fatal(err)
	}

	// Create a patch artifact so reconcile can operate.
	patch := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	s.WriteArtifact("add-older-greeting", "post-apply.patch", patch)
	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "add greeting")

	// Explicit direct reconcile of the superseded feature.
	results, err := RunReconcile(context.Background(), s, []string{"add-older-greeting"}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d (%v)", len(results), results)
	}
	if results[0].Slug != "add-older-greeting" {
		t.Fatalf("expected slug add-older-greeting, got %s", results[0].Slug)
	}

	foundWarning := false
	for _, note := range results[0].Notes {
		if strings.Contains(note, "historical-feature warning") &&
			strings.Contains(note, "add-newer-greeting") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected historical-feature warning note referencing superseder; got Notes=%v", results[0].Notes)
	}
}

// v0.12.0 rev-1 Internal F1 runtime flip (PRD §4.5.3 + ADR-028 D6/D8):
// when the superseder is stale (unhealthy), the historical target
// STAYS EXCLUDED from the default replay set. Previously this test
// asserted the historical remained IN the set — that runtime behavior
// contradicted PRD §4.5.3 clause 3 and the composeSupersessionLabels
// docstring, both of which lock exclusion regardless of superseder
// health. The rev-1 flip aligns the runtime with the accepted
// PRD/ADR contract; the `stale-superseder` label continues to render
// as the operator-visible signal that the replacement needs repair.
func TestReconcileDefaultSet_ExcludesFeatureWhenSupersederStale(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	s.AddFeature(store.AddFeatureInput{Title: "Add older greeting", Request: "older"})
	s.MarkFeatureState("add-older-greeting", store.StateApplied, "apply", "")

	// Superseder in an unhealthy state: draft (not applied/active).
	s.AddFeature(store.AddFeatureInput{Title: "Add newer greeting", Request: "newer"})
	// Do NOT mark applied — leave in draft (default state after AddFeature).

	newerStatus, err := s.LoadFeatureStatus("add-newer-greeting")
	if err != nil {
		t.Fatal(err)
	}
	newerStatus.DependsOn = []store.Dependency{
		{Slug: "add-older-greeting", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(newerStatus); err != nil {
		t.Fatal(err)
	}

	// A bystander applied feature keeps the default effective set
	// non-empty so RunReconcile has something to sweep after the
	// stale-supersession filter excludes the older target.
	s.AddFeature(store.AddFeatureInput{Title: "Add bystander", Request: "bystander"})
	s.MarkFeatureState("add-bystander", store.StateApplied, "apply", "")

	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hi\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "add greeting")

	results, err := RunReconcile(context.Background(), s, nil, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range results {
		seen[r.Slug] = true
	}
	// Stale superseder STILL excludes the historical target — PRD §4.5.3.
	if seen["add-older-greeting"] {
		t.Errorf("stale superseder must exclude historical target from default replay (PRD §4.5.3 / ADR-028 D6); got results=%v", results)
	}
	// Bystander must remain — the filter is targeted.
	if !seen["add-bystander"] {
		t.Errorf("bystander applied feature must remain in default replay set; got results=%v", results)
	}
}

// v0.12.0 rev-1 Internal F1 positive regression: an ORPHAN superseder
// (the superseder's own supersedes edge names a missing target) must
// NOT cause target-side exclusion because there is no target-side
// participant to exclude. This test differentiates orphan from stale:
// stale still excludes the historical target (previous test), orphan
// does not participate in the target-side exclusion decision.
//
// PRD §4.5 clarifies orphan is a label on the SUPERSEDER (naming a
// missing target); it does not put anything else into the effective
// set. The check is that the ORPHAN's own existence + supersedes-to-
// ghost does not cascade into excluding some unrelated feature.
func TestReconcileDefaultSet_OrphanSupersederDoesNotExcludeUnrelated(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// unrelated: applied and NOT the target of any supersedes edge.
	s.AddFeature(store.AddFeatureInput{Title: "unrelated", Request: "unrelated"})
	s.MarkFeatureState("unrelated", store.StateApplied, "apply", "")

	// orphan-newer: healthy superseder whose supersedes edge names a
	// missing target `ghost-target`. Its orphan label attaches to the
	// SUPERSEDER only — no exclusion effect on `unrelated`.
	s.AddFeature(store.AddFeatureInput{Title: "orphan newer", Request: "orphan"})
	s.MarkFeatureState("orphan-newer", store.StateApplied, "apply", "")
	orphanSt, err := s.LoadFeatureStatus("orphan-newer")
	if err != nil {
		t.Fatal(err)
	}
	orphanSt.DependsOn = []store.Dependency{
		{Slug: "ghost-target", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(orphanSt); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hi\n"), 0o644)
	gitAdd(t, tmpDir, "greeting.txt")
	gitCommit(t, tmpDir, "add greeting")

	results, err := RunReconcile(context.Background(), s, nil, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range results {
		seen[r.Slug] = true
	}
	if !seen["unrelated"] {
		t.Errorf("unrelated feature must remain in default replay set — orphan supersession does not exclude bystanders; got results=%v", results)
	}
}

// v0.12.0 rev-1 Internal F1 positive regression: assert
// IsFeatureSuperseded returns true for a STALE superseder now that the
// runtime is aligned with PRD §4.5.3. Complements the reconcile-level
// exclusion test by locking the primitive helper's flipped semantics.
func TestIsFeatureSuperseded_StaleSupersederReturnsTrue(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// target: healthy applied historical feature.
	s.AddFeature(store.AddFeatureInput{Title: "target", Request: "t"})
	s.MarkFeatureState("target", store.StateApplied, "apply", "")

	// stale-newer: unhealthy (draft state) superseder pointing at
	// target. Per rev-1 runtime flip, IsFeatureSuperseded(s, target)
	// must return true and name the superseder.
	s.AddFeature(store.AddFeatureInput{Title: "stale newer", Request: "s"})
	staleSt, err := s.LoadFeatureStatus("stale-newer")
	if err != nil {
		t.Fatal(err)
	}
	staleSt.DependsOn = []store.Dependency{
		{Slug: "target", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(staleSt); err != nil {
		t.Fatal(err)
	}

	superseder, superseded := IsFeatureSuperseded(s, "target")
	if !superseded {
		t.Fatalf("IsFeatureSuperseded(target) must return true for STALE superseder (rev-1 PRD §4.5.3 flip); got (%q, %v)", superseder, superseded)
	}
	if superseder != "stale-newer" {
		t.Fatalf("IsFeatureSuperseded(target) must name the stale superseder; got %q", superseder)
	}
}
