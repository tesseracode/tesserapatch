package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// tripwireProvider returns a benign "unclear" verdict for the phase-3
// semantic-check prompt and FAILS the test the moment a phase-3.5
// conflict-resolution prompt arrives. Per ADR-011 D6 the resolver MUST
// NOT be asked to fix a child whose hard parent is itself broken.
//
// Phase 3.5 prompts include the literal "# File:" marker per the
// resolver prompt template; phase 3 prompts use "# Recorded Patch".
type tripwireProvider struct {
	t           *testing.T
	phase3Calls int
}

func (p *tripwireProvider) Check(ctx context.Context, cfg provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *tripwireProvider) Generate(ctx context.Context, cfg provider.Config, req provider.GenerateRequest) (string, error) {
	if strings.Contains(req.UserPrompt, "# File:") {
		p.t.Errorf("tripwire: phase 3.5 resolver was invoked — short-circuit failed (blocked-by-parent should have skipped phase 3.5)")
		return "", errors.New("tripwire fired (phase 3.5)")
	}
	p.phase3Calls++
	return `{"decision":"unclear","reasoning":"phase-3 stub"}`, nil
}

// TestReconcile_FlagOn_BlockedByParent_SkipsPhase35 — the load-bearing
// scope-guard test for Chunk C. Sets up a child with a 3-way conflict
// AND a hard parent in `blocked-requires-human` state. With the DAG flag
// on, the reconciler must NOT invoke the provider; it must short-circuit
// to the compound verdict.
func TestReconcile_FlagOn_BlockedByParent_SkipsPhase35(t *testing.T) {
	s, slug := buildConflictFixture(t)

	// Enable the DAG flag.
	cfg, _ := s.LoadConfig()
	cfg.FeaturesDependencies = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// Seed a hard parent in a blocked verdict and wire the child to it.
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "broken-parent", Slug: "broken-parent", Request: "x"}); err != nil {
		t.Fatalf("AddFeature parent: %v", err)
	}
	parent, _ := s.LoadFeatureStatus("broken-parent")
	parent.State = store.StateApplied
	parent.Reconcile.Outcome = store.ReconcileBlockedRequiresHuman
	if err := s.SaveFeatureStatus(parent); err != nil {
		t.Fatalf("save parent: %v", err)
	}
	child, _ := s.LoadFeatureStatus(slug)
	child.DependsOn = []store.Dependency{{Slug: "broken-parent", Kind: store.DependencyKindHard}}
	if err := s.SaveFeatureStatus(child); err != nil {
		t.Fatalf("save child deps: %v", err)
	}

	prov := &tripwireProvider{t: t}
	provCfg := provider.Config{Type: "openai-compatible", BaseURL: "http://x", Model: "m", AuthEnv: "X"}

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", prov, provCfg,
		ReconcileOptions{Resolve: true, Apply: true})
	if err != nil {
		t.Fatalf("RunReconcile: %v", err)
	}
	// The planner pulls broken-parent into the closure; we get two
	// results. We assert on the child entry only.
	var r ReconcileResult
	found := false
	for _, candidate := range results {
		if candidate.Slug == slug {
			r = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("child %q not in results: %+v", slug, results)
	}
	if r.Outcome != store.ReconcileBlockedRequiresHuman {
		t.Errorf("Outcome=%s, want blocked-requires-human (intrinsic) — phase=%s", r.Outcome, r.Phase)
	}
	if !hasLabel(r.Labels, store.LabelBlockedByParent) {
		t.Errorf("result must carry LabelBlockedByParent; got labels=%v", r.Labels)
	}
	if r.Phase != "phase-3.5-skipped-blocked-by-parent" {
		t.Errorf("Phase=%q, want phase-3.5-skipped-blocked-by-parent", r.Phase)
	}

	// Persisted status must agree, and EffectiveOutcome must compose.
	st, _ := s.LoadFeatureStatus(slug)
	if st.Reconcile.Outcome != store.ReconcileBlockedRequiresHuman {
		t.Errorf("status.Reconcile.Outcome=%s, want blocked-requires-human", st.Reconcile.Outcome)
	}
	if !hasLabel(st.Reconcile.Labels, store.LabelBlockedByParent) {
		t.Errorf("status.Reconcile.Labels missing blocked-by-parent; got %v", st.Reconcile.Labels)
	}
	if got := st.Reconcile.EffectiveOutcome(); got != "blocked-by-parent-and-needs-resolution" {
		t.Errorf("EffectiveOutcome=%q, want compound", got)
	}

	// Notes must point at the blocking parent for operator UX.
	joined := strings.Join(r.Notes, " | ")
	if !strings.Contains(joined, "phase 3.5 skipped") {
		t.Errorf("notes should mention skip; got %q", joined)
	}
}

// TestEffectiveOutcome_CompoundComposition — workflow-level read of the
// store helper through the reconcile artifact path. Same as the unit
// test in store/, repeated here as a workflow-package smoke check.
func TestEffectiveOutcome_CompoundComposition(t *testing.T) {
	r := store.ReconcileSummary{
		Outcome: store.ReconcileBlockedRequiresHuman,
		Labels:  []store.ReconcileLabel{store.LabelBlockedByParent},
	}
	if got := r.EffectiveOutcome(); got != "blocked-by-parent-and-needs-resolution" {
		t.Fatalf("got %q, want compound", got)
	}
}

// TestEffectiveOutcome_PassthroughWhenNoCompoundLabels — verifies that
// label combinations OTHER than (needs-human-resolution + blocked-by-
// parent) do not produce the compound string.
func TestEffectiveOutcome_PassthroughWhenNoCompoundLabels(t *testing.T) {
	cases := []store.ReconcileSummary{
		{Outcome: store.ReconcileReapplied, Labels: []store.ReconcileLabel{store.LabelBlockedByParent}},
		{Outcome: store.ReconcileBlockedRequiresHuman, Labels: []store.ReconcileLabel{store.LabelStaleParentApplied}},
		{Outcome: store.ReconcileShadowAwaiting, Labels: []store.ReconcileLabel{store.LabelBlockedByParent}},
	}
	for _, c := range cases {
		if got := c.EffectiveOutcome(); got != string(c.Outcome) {
			t.Errorf("EffectiveOutcome(%+v)=%q, want %q", c, got, c.Outcome)
		}
	}
}
