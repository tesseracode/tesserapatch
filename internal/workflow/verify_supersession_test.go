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

// v0.12.0 rev-1 Internal F1 runtime flip (PRD §4.5.3 + ADR-028 D6/D8):
// V7's hard-parent closure replay skips a superseded parent whether
// its superseder is healthy or STALE. Previously this test asserted
// that a stale superseder must NOT mask a broken parent — that runtime
// contradicted the composeSupersessionLabels docstring and PRD §4.5.3
// clause 3, both of which lock exclusion regardless of superseder
// health. The rev-1 flip aligns V7 with the paper contract; drift on
// the historical stays warning-class per ADR-028 D8, and the
// `stale-superseder` label continues to surface the replacement's
// health separately.
func TestRunVerify_ClosureReplay_StaleSupersederSkipsParent(t *testing.T) {
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
	// returns false. Per PRD §4.5.3 clause 3 (rev-1 runtime flip),
	// V7 STILL skips the target — the historical drift becomes
	// warning-class rather than a V7 failure.
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
	if !v7.Passed || v7.Skipped {
		t.Errorf("V7 should pass — stale superseder still excludes historical parent (rev-1 PRD §4.5.3); got %+v (FailedAt=%q ParentSlug=%q)",
			v7, report.FailedAt, report.ParentSlug)
	}
	if report.FailedAt == "parent-replay" {
		t.Errorf("FailedAt must NOT be 'parent-replay' — superseded parent should be skipped whether the superseder is healthy or stale; got %q", report.FailedAt)
	}
}
