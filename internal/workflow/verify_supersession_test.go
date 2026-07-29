package workflow

import (
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// setSupersedes wires a supersedes edge on `superseder` pointing at
// `target`, mirroring setHardDeps but with kind=supersedes.
func setSupersedes(t *testing.T, s *store.Store, superseder, target string) {
	t.Helper()
	st, err := s.LoadFeatureStatus(superseder)
	if err != nil {
		t.Fatalf("load %s: %v", superseder, err)
	}
	st.DependsOn = append(st.DependsOn, store.Dependency{
		Slug: target,
		Kind: store.DependencyKindSupersedes,
	})
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatalf("save %s: %v", superseder, err)
	}
}

// v0.12.0 Wave α (ADR-028 D6, PRD-feature-supersession §5.1 + AC-6):
// V7's hard-parent closure replay must skip parents that are superseded
// by an active healthy superseder. In this fixture the leaf hard-depends
// on `mid`, but `newer` supersedes `mid` while healthy — V7 must NOT
// replay mid's recipe, so a broken mid recipe (search string missing)
// does not fail the leaf's V7.
func TestRunVerify_ClosureReplay_SupersededParentSkipped(t *testing.T) {
	s := setupVerifyFeature(t, "scratch")

	// root: clean write-file.
	setApplied(t, s, "root", ApplyRecipe{Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/root.txt", Content: "root\n"},
	}})

	// mid: a `replace-in-file` against a missing path. If replayed
	// in the shadow this would fail V7 with a "hard parent mid
	// failed to replay" error. But mid is superseded → skipped.
	setApplied(t, s, "mid", ApplyRecipe{Operations: []RecipeOperation{
		{Type: "replace-in-file", Path: "src/non-existent.txt", Search: "x", Replace: "y"},
	}})
	setHardDeps(t, s, "mid", []string{"root"})

	// newer: healthy superseder for mid (state=applied). Note the
	// DAG's edge-kind-agnostic cycle detector (ADR-011 D2) accepts
	// this because supersedes-from-newer to mid is not a cycle
	// with mid's hard-parent (mid → root, no back-edge to newer).
	setApplied(t, s, "newer", ApplyRecipe{Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/newer.txt", Content: "newer\n"},
	}})
	setSupersedes(t, s, "newer", "mid")

	// leaf: hard-depends on mid, applied.
	setApplied(t, s, "leaf", ApplyRecipe{Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/leaf.txt", Content: "leaf\n"},
	}})
	setHardDeps(t, s, "leaf", []string{"mid"})

	writeIntent(t, s, "leaf")
	writeVerifyRecipe(t, s, "leaf", ApplyRecipe{Feature: "leaf", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/leaf-final.txt", Content: "final\n"},
	}})

	report, err := RunVerify(s, "leaf", VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerify: %v", err)
	}

	v7 := findCheck(t, report, CheckRecipeReplayClean)
	if !v7.Passed || v7.Skipped {
		t.Errorf("V7 should pass — superseded hard parent 'mid' must be skipped from closure replay; got %+v (FailedAt=%q ParentSlug=%q)",
			v7, report.FailedAt, report.ParentSlug)
	}
	if report.FailedAt == "parent-replay" {
		t.Errorf("FailedAt must NOT be 'parent-replay' — superseded parent should be silently skipped; got %q", report.FailedAt)
	}
}

// v0.12.0 Wave α (ADR-028 D8): when the superseder is STALE (unhealthy),
// V7 must NOT skip the historical parent — the superseder isn't
// authoritative enough to displace it. A stale superseder over a broken
// mid should surface the underlying V7 replay failure, not mask it.
func TestRunVerify_ClosureReplay_StaleSupersederDoesNotSkipParent(t *testing.T) {
	s := setupVerifyFeature(t, "scratch")

	setApplied(t, s, "root", ApplyRecipe{Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/root.txt", Content: "root\n"},
	}})

	// mid: broken recipe (replace against missing path).
	setApplied(t, s, "mid", ApplyRecipe{Operations: []RecipeOperation{
		{Type: "replace-in-file", Path: "src/non-existent.txt", Search: "x", Replace: "y"},
	}})
	setHardDeps(t, s, "mid", []string{"root"})

	// newer: STALE superseder — no MarkFeatureState → state=draft
	// (not applied/active/upstream_merged), so supersederIsHealthy()
	// returns false and the target is NOT skipped from V7.
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "newer", Slug: "newer", Request: "x"}); err != nil {
		t.Fatalf("AddFeature newer: %v", err)
	}
	setSupersedes(t, s, "newer", "mid")

	setApplied(t, s, "leaf", ApplyRecipe{Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/leaf.txt", Content: "leaf\n"},
	}})
	setHardDeps(t, s, "leaf", []string{"mid"})

	writeIntent(t, s, "leaf")
	writeVerifyRecipe(t, s, "leaf", ApplyRecipe{Feature: "leaf", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/leaf-final.txt", Content: "final\n"},
	}})

	report, err := RunVerify(s, "leaf", VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerify: %v", err)
	}
	v7 := findCheck(t, report, CheckRecipeReplayClean)
	if v7.Passed {
		t.Errorf("V7 must FAIL — stale superseder must not mask broken hard parent; got %+v", v7)
	}
	if report.FailedAt != "parent-replay" {
		t.Errorf("FailedAt must be 'parent-replay' when broken hard parent surfaces; got %q", report.FailedAt)
	}
}
