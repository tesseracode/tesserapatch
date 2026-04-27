package workflow

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// C5 F1 regression: when a reconcile produces ReconcileUpstreamed
// (phase-1 reverse-apply, phase-2 op-level all-present, phase-3 provider
// semantic), saveReconcileArtifacts previously called composeLabelsAt
// which re-loaded child status FROM DISK — where the OLD outcome still
// lives. As a result, parent-derived labels would be composed against
// the pre-reconcile baseline and persisted alongside the freshly-
// upstreamed verdict. ADR-011: a retired child surfaces no parent
// labels. The guard added in C5 short-circuits label composition when
// result.Outcome is in childRetiredOutcomes.
//
// Tests below exercise the persistence path that every RunReconcile
// completion calls — saveReconcileArtifacts then updateFeatureState —
// asserting BOTH status.json and artifacts/reconcile-session.json have
// empty Labels for upstreamed outcomes, regardless of which phase set
// the verdict.

// seedRetiredChildScaffolding wires a child whose persisted status
// still claims a stale waiting-on-parent label and an OLD upstreamed
// outcome. The parent is in StateAnalyzed so that, were labels
// composed from disk, waiting-on-parent would re-fire.
func seedRetiredChildScaffolding(t *testing.T) *store.Store {
	t.Helper()
	s := planTestEnv(t, true)
	addPlanFeature(t, s, "parent", nil)
	addPlanFeature(t, s, "child", []store.Dependency{
		{Slug: "parent", Kind: store.DependencyKindHard},
	})
	// Parent in a state that would normally yield waiting-on-parent.
	setParentState(t, s, "parent", store.StateAnalyzed, "", "")
	child, _ := s.LoadFeatureStatus("child")
	child.Reconcile.AttemptedAt = "2025-01-01T00:00:00Z"
	child.Reconcile.Outcome = store.ReconcileBlocked // OLD outcome
	child.Reconcile.Labels = []store.ReconcileLabel{store.LabelWaitingOnParent}
	if err := s.SaveFeatureStatus(child); err != nil {
		t.Fatalf("seed child: %v", err)
	}
	return s
}

// assertNoLabelsPersisted reads BOTH status.json and
// artifacts/reconcile-session.json for the child and asserts neither
// carries any reconcile labels.
func assertNoLabelsPersisted(t *testing.T, s *store.Store, slug string) {
	t.Helper()
	got, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus: %v", err)
	}
	if len(got.Reconcile.Labels) != 0 {
		t.Fatalf("status.json: expected empty Labels for upstreamed child, got %v", got.Reconcile.Labels)
	}
	data, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "reconcile-session.json"))
	if err != nil {
		t.Fatalf("read reconcile-session.json: %v", err)
	}
	var session struct {
		Labels []store.ReconcileLabel `json:"labels"`
	}
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		t.Fatalf("unmarshal session artifact: %v", err)
	}
	if len(session.Labels) != 0 {
		t.Fatalf("reconcile-session.json: expected empty Labels for upstreamed child, got %v", session.Labels)
	}
	// Defense-in-depth: the JSON should not even contain the labels key
	// (omitempty must drop it). This catches accidental allocation of
	// an empty slice that would leak into the artifact.
	if strings.Contains(data, "\"labels\"") {
		t.Fatalf("reconcile-session.json must omit labels key for upstreamed child; got %s", data)
	}
}

func TestRunReconcile_Phase1ReverseApply_UpstreamedClearsLabels(t *testing.T) {
	s := seedRetiredChildScaffolding(t)
	result := &ReconcileResult{
		Slug:    "child",
		Outcome: store.ReconcileUpstreamed,
		Phase:   "phase-1-reverse-apply",
	}
	saveReconcileArtifacts(s, "child", result)
	updateFeatureState(s, "child", result)
	assertNoLabelsPersisted(t, s, "child")
}

func TestRunReconcile_Phase2OperationLevel_UpstreamedClearsLabels(t *testing.T) {
	s := seedRetiredChildScaffolding(t)
	result := &ReconcileResult{
		Slug:    "child",
		Outcome: store.ReconcileUpstreamed,
		Phase:   "phase-2-operation-level",
	}
	saveReconcileArtifacts(s, "child", result)
	updateFeatureState(s, "child", result)
	assertNoLabelsPersisted(t, s, "child")
}

func TestRunReconcile_Phase3ProviderSemantic_UpstreamedClearsLabels(t *testing.T) {
	s := seedRetiredChildScaffolding(t)
	result := &ReconcileResult{
		Slug:    "child",
		Outcome: store.ReconcileUpstreamed,
		Phase:   "phase-3-provider-semantic",
	}
	saveReconcileArtifacts(s, "child", result)
	updateFeatureState(s, "child", result)
	assertNoLabelsPersisted(t, s, "child")
}

// Control: a non-upstreamed outcome must STILL produce labels through
// the same persistence path. This guards against an over-broad C5 fix
// that would suppress all label composition.
func TestRunReconcile_NonUpstreamedOutcome_StillProducesLabels(t *testing.T) {
	s := seedRetiredChildScaffolding(t)
	result := &ReconcileResult{
		Slug:    "child",
		Outcome: store.ReconcileBlockedRequiresHuman,
		Phase:   "phase-4-forward-apply",
	}
	saveReconcileArtifacts(s, "child", result)
	updateFeatureState(s, "child", result)

	got, err := s.LoadFeatureStatus("child")
	if err != nil {
		t.Fatalf("LoadFeatureStatus: %v", err)
	}
	if !hasLabel(got.Reconcile.Labels, store.LabelWaitingOnParent) {
		t.Fatalf("expected waiting-on-parent label for non-retired outcome, got %v", got.Reconcile.Labels)
	}
}
