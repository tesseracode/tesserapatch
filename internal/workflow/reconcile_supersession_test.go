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

// v0.12.0 Wave α (ADR-028 D6, PRD-feature-supersession AC-5):
// when the superseder itself is unhealthy (blocked/removed), the
// historical target should REMAIN in the default replay set — the
// superseder is not authoritative enough to displace it. The
// `stale-superseder` render label surfaces the anomaly separately.
func TestReconcileDefaultSet_KeepsFeatureWhenSupersederStale(t *testing.T) {
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
	// Stale superseder does NOT displace the historical target.
	if !seen["add-older-greeting"] {
		t.Errorf("stale superseder must NOT exclude historical target; got results=%v", results)
	}
}
