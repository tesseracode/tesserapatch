package workflow

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestSliceR3_ReconcileLaterTouchWarns — PRD-write-file-recipe-safety
// AC-8 + §4.2 "During reconcile": across an effective replay set, an
// older feature that owns a write-file at path P and a newer feature
// that touched P should surface a deterministic warning owned by the
// OLDER feature (the write-file operator). ADR-029 D6 warning-class;
// reconcile does not refuse (PRD §7.2).
func TestSliceR3_ReconcileLaterTouchWarns(t *testing.T) {
	s := slice2Store(t)

	// older owns a write-file on src/a.txt.
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("older")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "older", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "older\n"},
	})

	// newer touched the same path (later-touch).
	slice3Feature(t, s, "newer", "2026-06-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("newer")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "newer\n"},
	})

	warnings := DetectReconcileLaterTouchWarnings(s, []string{"older", "newer"})
	if len(warnings) == 0 {
		t.Fatalf("expected a later-touch warning across the effective set; got none")
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "[older]") {
		t.Errorf("warning should be owned by the older write-file feature; got: %s", joined)
	}
	if !strings.Contains(joined, "\"newer\"") {
		t.Errorf("warning should name the newer touching feature; got: %s", joined)
	}
	if !strings.Contains(joined, "src/a.txt") {
		t.Errorf("warning should name the shared path; got: %s", joined)
	}
	if !strings.Contains(joined, "PRD-write-file-recipe-safety §4.2") {
		t.Errorf("warning should cite PRD §4.2; got: %s", joined)
	}
	if !strings.Contains(joined, "ADR-029 D6") {
		t.Errorf("warning should cite ADR-029 D6; got: %s", joined)
	}
}

// TestSliceR3_ReconcileByOwnerRoutesToOlder — the by-owner variant
// (consumed by RunReconcile to attach warnings to each per-feature
// ReconcileResult.Notes) keys the map by the OLDER (owner) slug, not
// the newer touching slug.
func TestSliceR3_ReconcileByOwnerRoutesToOlder(t *testing.T) {
	s := slice2Store(t)
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("older")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "older", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "older\n"},
	})

	slice3Feature(t, s, "newer", "2026-06-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("newer")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "newer\n"},
	})

	byOwner := DetectReconcileLaterTouchWarningsByOwner(s, []string{"older", "newer"})
	if _, ok := byOwner["older"]; !ok {
		t.Fatalf("expected warning to be keyed by owner \"older\"; got keys: %v", keys(byOwner))
	}
	if _, ok := byOwner["newer"]; ok {
		t.Errorf("newer touching feature should NOT own the warning; keys: %v", keys(byOwner))
	}
	if len(byOwner["older"]) == 0 {
		t.Errorf("expected non-empty warning slice for owner \"older\"")
	}
}

// TestSliceR3_ReconcileSortedByOwnerThenPath — determinism: multiple
// owners with overlaps yield warnings sorted by owner alphabetically,
// then by path alphabetically (PRD §5 note 4).
func TestSliceR3_ReconcileSortedByOwnerThenPath(t *testing.T) {
	s := slice2Store(t)

	// Two older owners: alpha owns src/a.txt, bravo owns src/b.txt.
	slice3Feature(t, s, "alpha", "2026-01-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("alpha")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "alpha", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "a\n"},
	})

	slice3Feature(t, s, "bravo", "2026-02-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("bravo")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "bravo", []RecipeOperation{
		{Type: "write-file", Path: "src/b.txt", PreimageHash: ptr(""), Content: "b\n"},
	})

	// newer touches both paths.
	slice3Feature(t, s, "newer", "2026-06-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("newer")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "x\n"},
		{Type: "write-file", Path: "src/b.txt", PreimageHash: ptr(""), Content: "y\n"},
	})

	warnings := DetectReconcileLaterTouchWarnings(s, []string{"bravo", "alpha", "newer"})
	if len(warnings) < 2 {
		t.Fatalf("expected 2 warnings; got %d: %v", len(warnings), warnings)
	}
	// alpha comes first alphabetically → its warning first.
	if !strings.Contains(warnings[0], "[alpha]") {
		t.Errorf("first warning should be owned by alpha; got: %s", warnings[0])
	}
	if !strings.Contains(warnings[1], "[bravo]") {
		t.Errorf("second warning should be owned by bravo; got: %s", warnings[1])
	}
}

// TestSliceR3_ReconcileNoOverlapNoWarning — negative direction: an
// effective set without overlapping write-file / touched-path pairs
// yields zero warnings.
func TestSliceR3_ReconcileNoOverlapNoWarning(t *testing.T) {
	s := slice2Store(t)

	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("older")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "older", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "older\n"},
	})

	slice3Feature(t, s, "newer", "2026-06-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("newer")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/unrelated.txt", PreimageHash: ptr(""), Content: "unrelated\n"},
	})

	warnings := DetectReconcileLaterTouchWarnings(s, []string{"older", "newer"})
	if len(warnings) != 0 {
		t.Errorf("expected zero warnings when paths don't overlap; got: %v", warnings)
	}
}

// TestSliceR3_ReconcileOlderNotInSetSkipped — an older feature that is
// NOT in the effective slug set (e.g. excluded by supersession
// filtering upstream in RunReconcile) does not generate warnings.
func TestSliceR3_ReconcileOlderNotInSetSkipped(t *testing.T) {
	s := slice2Store(t)

	// older exists in the store but is not in the effective set.
	slice3Feature(t, s, "older", "2026-01-01T00:00:00Z")
	st, _ := s.LoadFeatureStatus("older")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "older", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "older\n"},
	})

	slice3Feature(t, s, "newer", "2026-06-01T00:00:00Z")
	st, _ = s.LoadFeatureStatus("newer")
	st.State = store.StateApplied
	s.SaveFeatureStatus(st)
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "newer\n"},
	})

	// Call with ONLY newer in the effective set. older is excluded.
	warnings := DetectReconcileLaterTouchWarnings(s, []string{"newer"})
	if len(warnings) != 0 {
		t.Errorf("expected zero warnings when older is not in effective set; got: %v", warnings)
	}
}

func keys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestSliceR3_ReconcileWireupAttachesWarningToOwnerNotes — Rule 20
// empirical proof-of-wire-up: RunReconcile end-to-end must attach the
// per-owner AC-8 warning to the OLDER (write-file owner) feature's
// ReconcileResult.Notes, without refusing execution (D6 warn-class).
// Mirrors the setupGitRepo / RunReconcile pattern from reconcile_test.go.
func TestSliceR3_ReconcileWireupAttachesWarningToOwnerNotes(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// older: write-file at src/a.txt, applied.
	s.AddFeature(store.AddFeatureInput{Title: "older", Request: "older"})
	s.MarkFeatureState("older", store.StateApplied, "apply", "")
	os.MkdirAll(filepath.Join(tmpDir, "src"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "src/a.txt"), []byte("older\n"), 0o644)
	gitAdd(t, tmpDir, "src/a.txt")
	gitCommit(t, tmpDir, "older applied")
	olderPatch := `diff --git a/src/a.txt b/src/a.txt
new file mode 100644
index 0000000..0000001
--- /dev/null
+++ b/src/a.txt
@@ -0,0 +1 @@
+older
`
	s.WriteArtifact("older", "post-apply.patch", olderPatch)
	slice3WriteRecipe(t, s, "older", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "older\n"},
	})
	// Bump older's RequestedAt below newer.
	os1, _ := s.LoadFeatureStatus("older")
	os1.RequestedAt = "2026-01-01T00:00:00Z"
	s.SaveFeatureStatus(os1)

	// newer: also touches src/a.txt, applied.
	s.AddFeature(store.AddFeatureInput{Title: "newer", Request: "newer"})
	s.MarkFeatureState("newer", store.StateApplied, "apply", "")
	os.WriteFile(filepath.Join(tmpDir, "src/a.txt"), []byte("newer\n"), 0o644)
	gitAdd(t, tmpDir, "src/a.txt")
	gitCommit(t, tmpDir, "newer applied")
	newerPatch := `diff --git a/src/a.txt b/src/a.txt
index 0000001..0000002 100644
--- a/src/a.txt
+++ b/src/a.txt
@@ -1 +1 @@
-older
+newer
`
	s.WriteArtifact("newer", "post-apply.patch", newerPatch)
	slice3WriteRecipe(t, s, "newer", []RecipeOperation{
		{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "newer\n"},
	})
	ns, _ := s.LoadFeatureStatus("newer")
	ns.RequestedAt = "2026-06-01T00:00:00Z"
	s.SaveFeatureStatus(ns)

	// Run reconcile across both.
	results, err := RunReconcile(context.Background(), s, []string{"older", "newer"}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("RunReconcile failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Find each per slug.
	var olderResult, newerResult *ReconcileResult
	for i := range results {
		switch results[i].Slug {
		case "older":
			olderResult = &results[i]
		case "newer":
			newerResult = &results[i]
		}
	}
	if olderResult == nil || newerResult == nil {
		t.Fatalf("missing per-slug results: %+v", results)
	}
	// Older (owner) MUST carry the later-touch warning in its Notes.
	joined := strings.Join(olderResult.Notes, "\n")
	if !strings.Contains(joined, "later-touch warning") {
		t.Errorf("expected older's Notes to contain later-touch warning; got:\n%s", joined)
	}
	if !strings.Contains(joined, "src/a.txt") || !strings.Contains(joined, "\"newer\"") {
		t.Errorf("older's warning should name src/a.txt and \"newer\"; got:\n%s", joined)
	}
	// Newer MUST NOT carry the warning (it is the toucher, not the
	// owner; per-owner attachment routes only to the write-file owner).
	nJoined := strings.Join(newerResult.Notes, "\n")
	if strings.Contains(nJoined, "later-touch warning") {
		t.Errorf("newer should not carry the later-touch warning; got:\n%s", nJoined)
	}
	// Reconcile MUST NOT refuse based on later-touch — D6 warn-class.
	// Outcome != blocked for reasons other than an explicit error.
	if olderResult.Outcome == store.ReconcileBlocked && strings.Contains(strings.Join(olderResult.Notes, "\n"), "Error:") {
		t.Errorf("older reconcile blocked with error, expected non-blocked warn-class flow; notes=%v", olderResult.Notes)
	}
}
