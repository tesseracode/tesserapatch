package workflow

import (
	"reflect"
	"sort"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// v0.12.0 Wave α — supersession label composition tests.
//
// Coverage matrix (PRD-feature-supersession AC-6, AC-7, AC-12; ADR-028
// D4). Each test asserts the read-time overlay attaches on the correct
// side of a supersedes edge (superseder vs target) and composes with
// the existing M14.3 + freshness sets.

func hasLabelInSlice(labels []store.ReconcileLabel, want store.ReconcileLabel) bool {
	for _, l := range labels {
		if l == want {
			return true
		}
	}
	return false
}

// hasSupersededByFor returns true when labels contain the composite
// `superseded-by <superseder>` token per PRD §4.1 binding contract
// (v0.12.0 rev-1 F-SEXT-1).
func hasSupersededByFor(labels []store.ReconcileLabel, superseder string) bool {
	want := string(store.LabelSupersededBy) + " " + superseder
	for _, l := range labels {
		if string(l) == want {
			return true
		}
	}
	return false
}

// hasAnySupersededBy returns true when labels contain any
// `superseded-by …` token (bare or composite).
func hasAnySupersededBy(labels []store.ReconcileLabel) bool {
	prefix := string(store.LabelSupersededBy)
	for _, l := range labels {
		s := string(l)
		if s == prefix {
			return true
		}
		if len(s) > len(prefix)+1 && s[:len(prefix)+1] == prefix+" " {
			return true
		}
	}
	return false
}

func labelSlice(t *testing.T, s *store.Store, slug string) []store.ReconcileLabel {
	t.Helper()
	got, err := ComposeLabels(s, slug)
	if err != nil {
		t.Fatalf("ComposeLabels(%s): %v", slug, err)
	}
	// Ensure caller sees a stable order regardless of set-iteration
	// randomness.
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	return got
}

// TestComposeLabels_ActiveSuperseder — a healthy superseder with a
// valid target gets `active-superseder`; the target gets
// `superseded-by`.
func TestComposeLabels_ActiveSuperseder(t *testing.T) {
	s := planTestEnv(t, true)
	addPlanFeature(t, s, "older", nil)
	addPlanFeature(t, s, "newer", []store.Dependency{
		{Slug: "older", Kind: store.DependencyKindSupersedes},
	})
	// Both features healthy for default replay.
	setParentState(t, s, "older", store.StateApplied, "", "")
	setParentState(t, s, "newer", store.StateApplied, "", "")

	newer := labelSlice(t, s, "newer")
	if !hasLabelInSlice(newer, store.LabelActiveSuperseder) {
		t.Fatalf("newer must carry active-superseder, got %v", newer)
	}
	if hasLabelInSlice(newer, store.LabelStaleSuperseder) {
		t.Fatalf("newer is healthy — must not carry stale-superseder, got %v", newer)
	}
	if hasLabelInSlice(newer, store.LabelOrphanSuperseder) {
		t.Fatalf("newer's target exists — must not carry orphan-superseder, got %v", newer)
	}

	older := labelSlice(t, s, "older")
	if !hasSupersededByFor(older, "newer") {
		t.Fatalf("older must carry composite `superseded-by newer` (PRD §4.1 / F-SEXT-1), got %v", older)
	}
	if hasLabelInSlice(older, store.LabelStaleSuperseder) {
		t.Fatalf("newer is healthy — older must not carry stale-superseder, got %v", older)
	}
}

// TestComposeLabels_SupersededByCarriesSlug — v0.12.0 rev-1 F-SEXT-1:
// the `superseded-by` label must render as the composite
// `superseded-by <slug>` per PRD-feature-supersession §4.1 binding
// label-value contract + ADR-028 D4:58. The historical target of a
// healthy superseder carries the composite token with the superseder's
// slug appended, not the bare literal.
func TestComposeLabels_SupersededByCarriesSlug(t *testing.T) {
	s := planTestEnv(t, true)
	addPlanFeature(t, s, "target", nil)
	addPlanFeature(t, s, "replacer", []store.Dependency{
		{Slug: "target", Kind: store.DependencyKindSupersedes},
	})
	setParentState(t, s, "target", store.StateApplied, "", "")
	setParentState(t, s, "replacer", store.StateApplied, "", "")

	got := labelSlice(t, s, "target")
	want := store.ReconcileLabel("superseded-by replacer")
	if !hasLabelInSlice(got, want) {
		t.Fatalf("target must carry composite %q (F-SEXT-1), got %v", want, got)
	}
	if hasLabelInSlice(got, store.LabelSupersededBy) {
		t.Fatalf("bare literal `superseded-by` must not be emitted alongside the composite; got %v", got)
	}
}

// TestComposeLabels_OrphanSuperseder — a superseder pointing at a
// missing target gets `orphan-superseder`. No `active-superseder`
// because no target exists.
func TestComposeLabels_OrphanSuperseder(t *testing.T) {
	s := planTestEnv(t, true)
	// Only `newer` exists; the `ghost` target is missing.
	addPlanFeature(t, s, "newer", []store.Dependency{
		{Slug: "ghost", Kind: store.DependencyKindSupersedes},
	})
	setParentState(t, s, "newer", store.StateApplied, "", "")

	got := labelSlice(t, s, "newer")
	if !hasLabelInSlice(got, store.LabelOrphanSuperseder) {
		t.Fatalf("dangling supersedes target must yield orphan-superseder, got %v", got)
	}
	if hasLabelInSlice(got, store.LabelActiveSuperseder) {
		t.Fatalf("no valid target — active-superseder must not appear, got %v", got)
	}
}

// TestComposeLabels_StaleSuperseder — a superseder whose reconcile
// outcome is terminally blocked gets `stale-superseder` (on itself);
// the target ALSO gets `stale-superseder` (in place of superseded-by)
// so operators see the replacement is broken but default replay still
// excludes the historical feature per ADR-028 D6/D8.
func TestComposeLabels_StaleSuperseder(t *testing.T) {
	s := planTestEnv(t, true)
	addPlanFeature(t, s, "older", nil)
	addPlanFeature(t, s, "newer", []store.Dependency{
		{Slug: "older", Kind: store.DependencyKindSupersedes},
	})
	setParentState(t, s, "older", store.StateApplied, "", "")
	setParentState(t, s, "newer", store.StateApplied, store.ReconcileBlockedRequiresHuman, "")

	newer := labelSlice(t, s, "newer")
	if !hasLabelInSlice(newer, store.LabelStaleSuperseder) {
		t.Fatalf("unhealthy newer must carry stale-superseder, got %v", newer)
	}
	// active-superseder still applies — the target exists — even
	// though the superseder itself is unhealthy.
	if !hasLabelInSlice(newer, store.LabelActiveSuperseder) {
		t.Fatalf("newer has a real target — active-superseder should still apply, got %v", newer)
	}

	older := labelSlice(t, s, "older")
	if !hasLabelInSlice(older, store.LabelStaleSuperseder) {
		t.Fatalf("target of unhealthy superseder must carry stale-superseder, got %v", older)
	}
	if hasAnySupersededBy(older) {
		t.Fatalf("target of unhealthy superseder must not carry superseded-by, got %v", older)
	}
}

// TestComposeLabels_NoSupersessionEdges_NoSupersessionLabels — a plain
// feature graph with only hard/soft edges must not surface any of the
// four supersession labels (regression guard for the M14.3 pre-Slice-3
// contract).
func TestComposeLabels_NoSupersessionEdges_NoSupersessionLabels(t *testing.T) {
	s := planTestEnv(t, true)
	addPlanFeature(t, s, "parent", nil)
	addPlanFeature(t, s, "child", []store.Dependency{
		{Slug: "parent", Kind: store.DependencyKindHard},
	})
	setParentState(t, s, "parent", store.StateApplied, "", "")

	child := labelSlice(t, s, "child")
	for _, l := range []store.ReconcileLabel{
		store.LabelSupersededBy,
		store.LabelActiveSuperseder,
		store.LabelStaleSuperseder,
		store.LabelOrphanSuperseder,
	} {
		if hasLabelInSlice(child, l) {
			t.Fatalf("plain hard-dep graph must not carry %s, got %v", l, child)
		}
	}
}

// TestDeriveSupersessionLabels_SeverityOrder — v0.12.0 rev-1 F-SEXT-2:
// PRD §4.3:184-188 + ADR-028 D4:63-67 lock the supersession-group
// render order as severity-first
// `[stale-superseder] [orphan-superseder] [superseded-by <slug>] [active-superseder]`.
// This test constructs a graph where all four labels are active on one
// superseder feature and asserts the exact positional order returned
// by DeriveSupersessionLabels.
func TestDeriveSupersessionLabels_SeverityOrder(t *testing.T) {
	s := planTestEnv(t, true)
	// `target-healthy` — exists so `active-superseder` fires on the
	// combo node.
	addPlanFeature(t, s, "target-healthy", nil)
	setParentState(t, s, "target-healthy", store.StateApplied, "", "")

	// `combo` — the node under test. It supersedes the healthy target
	// (yields active-superseder) AND supersedes a ghost target (yields
	// orphan-superseder). Its OWN state is draft so it is unhealthy →
	// stale-superseder on itself. A peer `peer` supersedes `combo`
	// (target side of the graph) — peer is applied → yields
	// composite `superseded-by peer`.
	addPlanFeature(t, s, "combo", []store.Dependency{
		{Slug: "target-healthy", Kind: store.DependencyKindSupersedes},
		{Slug: "ghost", Kind: store.DependencyKindSupersedes},
	})
	// Do not mark combo applied → superseder unhealthy → stale.

	addPlanFeature(t, s, "peer", []store.Dependency{
		{Slug: "combo", Kind: store.DependencyKindSupersedes},
	})
	setParentState(t, s, "peer", store.StateApplied, "", "")

	comboStatus, err := s.LoadFeatureStatus("combo")
	if err != nil {
		t.Fatal(err)
	}
	got := DeriveSupersessionLabels(s, comboStatus)

	want := []store.ReconcileLabel{
		store.LabelStaleSuperseder,
		store.LabelOrphanSuperseder,
		store.ReconcileLabel("superseded-by peer"),
		store.LabelActiveSuperseder,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("severity-first order violation (F-SEXT-2 / PRD §4.3):\n got: %v\nwant: %v", got, want)
	}
}

// TestStripSupersessionLabels_RemovesOnlyFour verifies the strip
// helper's scope is exactly the four Wave α labels.
func TestStripSupersessionLabels_RemovesOnlyFour(t *testing.T) {
	in := []store.ReconcileLabel{
		store.LabelWaitingOnParent,
		store.LabelSupersededBy,
		store.LabelActiveSuperseder,
		store.LabelStaleSuperseder,
		store.LabelOrphanSuperseder,
		store.LabelBlockedByParent,
		store.LabelVerifiedFresh, // freshness label — should survive this strip
	}
	got := StripSupersessionLabels(in)
	want := []store.ReconcileLabel{
		store.LabelWaitingOnParent,
		store.LabelBlockedByParent,
		store.LabelVerifiedFresh,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StripSupersessionLabels: got %v, want %v", got, want)
	}
}

// TestComposeLabels_SupersessionLabelsNotPersisted — after RunReconcile
// via stripDerivedLabels, none of the four supersession labels may
// appear in the persisted `Reconcile.Labels` slice. Exercises the
// contract that persistence sites always chain freshness+supersession
// strips.
func TestComposeLabels_SupersessionLabelsNotPersisted(t *testing.T) {
	// Labels the compose function might return.
	composed := []store.ReconcileLabel{
		store.LabelWaitingOnParent,
		store.LabelActiveSuperseder,
		store.LabelStaleSuperseder,
		store.LabelSupersededBy,
		store.LabelOrphanSuperseder,
		store.LabelVerifiedFresh,
	}
	got := stripDerivedLabels(composed)
	want := []store.ReconcileLabel{store.LabelWaitingOnParent}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stripDerivedLabels must remove freshness + supersession: got %v, want %v", got, want)
	}
}
