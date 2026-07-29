package store

import (
	"errors"
	"strings"
	"testing"
)

// v0.12.0 Wave α — supersession validation tests.
//
// Coverage matrix (PRD-feature-supersession AC-1..AC-4, AC-8, AC-9;
// ADR-028 D1, D2):
//
//   - accept-third-kind: a well-formed `supersedes` edge validates
//     cleanly (AC-8 backward compatibility with the schema and
//     ADR-011 D1 storage lane).
//   - self-supersedes: rejected as ErrSelfDependency (AC-3).
//   - reciprocal supersession: X supersedes Y + Y supersedes X is
//     rejected by the existing DFS cycle detector (AC-1, ADR-028 D2).
//   - mixed longer cycle: X hard→ Y, Y supersedes Z, Z soft→ X is
//     rejected by the same detector (AC-2, ADR-028 D2 — cycle
//     detection spans all three kinds).
//   - invalid kind: `wobble` remains rejected (guards the allow-list).
//   - kind conflict: `supersedes` participates in the same
//     ErrKindConflict rule (parent declared as both hard and
//     supersedes on the same edge list).

func TestValidateDependencies_AcceptsSupersedes(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{
		"older": StateApplied,
		"newer": StateApplied,
	})
	deps := []Dependency{{Slug: "older", Kind: DependencyKindSupersedes}}
	if err := ValidateDependencies(s, "newer", deps); err != nil {
		t.Fatalf("`supersedes` should validate on a clean graph, got %v", err)
	}
}

func TestValidateDependencies_SupersedesSelfEdgeRejected(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{"alpha": StateApplied})
	err := ValidateDependencies(s, "alpha", []Dependency{{Slug: "alpha", Kind: DependencyKindSupersedes}})
	if !errors.Is(err, ErrSelfDependency) {
		t.Fatalf("want ErrSelfDependency for `supersedes` self-edge, got %v", err)
	}
}

func TestValidateDependencies_SupersedesReciprocalCycle(t *testing.T) {
	// PRD AC-1: X supersedes Y + Y supersedes X is a cycle.
	s := newStoreWith(t, map[string]FeatureState{
		"alpha": StateApplied,
		"beta":  StateApplied,
	})
	st, _ := s.LoadFeatureStatus("alpha")
	st.DependsOn = []Dependency{{Slug: "beta", Kind: DependencyKindSupersedes}}
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	err := ValidateDependencies(s, "beta", []Dependency{{Slug: "alpha", Kind: DependencyKindSupersedes}})
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("want ErrCycle for reciprocal supersedes, got %v", err)
	}
	if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Fatalf("cycle error should surface the actionable path, got %v", err)
	}
}

func TestValidateDependencies_MixedKindCycle(t *testing.T) {
	// PRD AC-2 / ADR-028 D2: cycles must be detected across all
	// three kinds. Graph: X hard→ Y, Y supersedes Z, Z soft→ X.
	s := newStoreWith(t, map[string]FeatureState{
		"x": StateApplied,
		"y": StateApplied,
		"z": StateApplied,
	})
	// x hard-depends-on y.
	x, _ := s.LoadFeatureStatus("x")
	x.DependsOn = []Dependency{{Slug: "y", Kind: DependencyKindHard}}
	if err := s.SaveFeatureStatus(x); err != nil {
		t.Fatal(err)
	}
	// y supersedes z.
	y, _ := s.LoadFeatureStatus("y")
	y.DependsOn = []Dependency{{Slug: "z", Kind: DependencyKindSupersedes}}
	if err := s.SaveFeatureStatus(y); err != nil {
		t.Fatal(err)
	}
	// Propose z soft-depends-on x → closes the ring.
	err := ValidateDependencies(s, "z", []Dependency{{Slug: "x", Kind: DependencyKindSoft}})
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("want ErrCycle for mixed-kind ring x→y→z→x, got %v", err)
	}
}

func TestValidateDependencies_SupersedesLinearChainAllowed(t *testing.T) {
	// Happy path: successive replacements form a chain, not a cycle.
	// v3 supersedes v2, v2 supersedes v1. Add v3 → v2 last.
	s := newStoreWith(t, map[string]FeatureState{
		"v1": StateApplied,
		"v2": StateApplied,
		"v3": StateApplied,
	})
	v2, _ := s.LoadFeatureStatus("v2")
	v2.DependsOn = []Dependency{{Slug: "v1", Kind: DependencyKindSupersedes}}
	if err := s.SaveFeatureStatus(v2); err != nil {
		t.Fatal(err)
	}
	err := ValidateDependencies(s, "v3", []Dependency{{Slug: "v2", Kind: DependencyKindSupersedes}})
	if err != nil {
		t.Fatalf("linear supersedes chain must validate, got %v", err)
	}
}

func TestValidateDependencies_UnknownKindRejected(t *testing.T) {
	// Regression guard: the allow-list is closed. Adding
	// `supersedes` must NOT open the door to arbitrary strings.
	s := newStoreWith(t, map[string]FeatureState{
		"parent": StateApplied,
		"child":  StateRequested,
	})
	err := ValidateDependencies(s, "child", []Dependency{{Slug: "parent", Kind: "wobble"}})
	if !errors.Is(err, ErrInvalidDependencyKind) {
		t.Fatalf("want ErrInvalidDependencyKind for arbitrary kind, got %v", err)
	}
}

func TestValidateDependencies_SupersedesVsHardKindConflict(t *testing.T) {
	// The same parent declared twice with different kinds — one
	// `hard`, one `supersedes` — must be rejected as a kind conflict.
	s := newStoreWith(t, map[string]FeatureState{
		"parent": StateApplied,
		"child":  StateRequested,
	})
	deps := []Dependency{
		{Slug: "parent", Kind: DependencyKindHard},
		{Slug: "parent", Kind: DependencyKindSupersedes},
	}
	err := ValidateDependencies(s, "child", deps)
	if !errors.Is(err, ErrKindConflict) {
		t.Fatalf("want ErrKindConflict for hard+supersedes on same parent, got %v", err)
	}
}

// TestDetectCycles_SupersedesReciprocal exercises the pure DAG
// primitive directly to lock the ADR-011 D2 property that DFS
// cycle detection is edge-kind-agnostic. The graph carries no
// hard/soft edges — only supersedes — so a failure here proves a
// regression in the DFS kind neutrality, not the wrapper.
func TestDetectCycles_SupersedesReciprocal(t *testing.T) {
	g := map[string][]Dependency{
		"a": {{Slug: "b", Kind: DependencyKindSupersedes}},
		"b": {{Slug: "a", Kind: DependencyKindSupersedes}},
	}
	cycle, err := DetectCycles(g)
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("want ErrCycle on reciprocal supersedes graph, got %v", err)
	}
	if len(cycle) < 2 {
		t.Fatalf("expected cycle path, got %v", cycle)
	}
}

// TestDetectCycles_MixedKindsClean verifies the happy path: a
// three-node DAG mixing all three kinds must not produce a cycle.
func TestDetectCycles_MixedKindsClean(t *testing.T) {
	g := map[string][]Dependency{
		"child":       {{Slug: "parent", Kind: DependencyKindHard}, {Slug: "older", Kind: DependencyKindSupersedes}},
		"parent":      {{Slug: "grandparent", Kind: DependencyKindSoft}},
		"older":       nil,
		"grandparent": nil,
	}
	cycle, err := DetectCycles(g)
	if err != nil {
		t.Fatalf("clean mixed-kind DAG must not cycle, got %v (cycle=%v)", err, cycle)
	}
	if len(cycle) != 0 {
		t.Fatalf("expected empty cycle path, got %v", cycle)
	}
}

// v0.12.0 rev-1 F-SEXT-3 — PRD-feature-supersession AC-4 + ADR-028 D5:
// a historical feature may have at most one active/effective superseder.
// The write-time validator MUST reject a second `supersedes -> Y` edge
// when Y already has a healthy peer superseder.
//
// Fixture: `target` is applied. `existing` (applied) already supersedes
// `target`. Proposing that `newcomer` also supersede `target` must
// error with ErrMultipleActiveSuperseders and name both superseder
// slugs.
func TestValidateDependencies_MultipleActiveSupersedersRejected(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{
		"target":   StateApplied,
		"existing": StateApplied,
		"newcomer": StateApplied,
	})
	// Wire the existing supersedes edge on disk.
	existing, err := s.LoadFeatureStatus("existing")
	if err != nil {
		t.Fatalf("load existing: %v", err)
	}
	existing.DependsOn = []Dependency{{Slug: "target", Kind: DependencyKindSupersedes}}
	if err := s.SaveFeatureStatus(existing); err != nil {
		t.Fatalf("save existing: %v", err)
	}

	// Propose the second supersedes edge.
	err = ValidateDependencies(s, "newcomer", []Dependency{
		{Slug: "target", Kind: DependencyKindSupersedes},
	})
	if !errors.Is(err, ErrMultipleActiveSuperseders) {
		t.Fatalf("want ErrMultipleActiveSuperseders on second healthy superseder, got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "existing") || !strings.Contains(msg, "newcomer") || !strings.Contains(msg, "target") {
		t.Fatalf("error message must name both superseders and the shared target; got %q", msg)
	}
}

// TestValidateDependencies_MultipleSupersedersOneStale — when only ONE
// of the two candidate superseders is healthy, the write validator
// must accept the second edge. Stale peers do not participate in the
// AC-4 / D5 conflict (they cannot displace the historical target on
// their own).
func TestValidateDependencies_MultipleSupersedersOneStale(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{
		"target":         StateApplied,
		"stale-existing": StateRequested, // draft → unhealthy
		"newcomer":       StateApplied,
	})
	stale, err := s.LoadFeatureStatus("stale-existing")
	if err != nil {
		t.Fatalf("load stale-existing: %v", err)
	}
	stale.DependsOn = []Dependency{{Slug: "target", Kind: DependencyKindSupersedes}}
	if err := s.SaveFeatureStatus(stale); err != nil {
		t.Fatalf("save stale-existing: %v", err)
	}

	if err := ValidateDependencies(s, "newcomer", []Dependency{
		{Slug: "target", Kind: DependencyKindSupersedes},
	}); err != nil {
		t.Fatalf("second superseder must be accepted when the existing peer is stale; got %v", err)
	}
}

// TestValidateAllFeatures_MultipleActiveSupersedersFlagged — bulk
// validation must flag on-disk corruption where two healthy features
// point supersedes at the same target. Locks the read-time fan-in
// scan behavior added in v0.12.0 rev-1.
func TestValidateAllFeatures_MultipleActiveSupersedersFlagged(t *testing.T) {
	s := newStoreWith(t, map[string]FeatureState{
		"target": StateApplied,
		"peer-a": StateApplied,
		"peer-b": StateApplied,
	})
	for _, slug := range []string{"peer-a", "peer-b"} {
		st, err := s.LoadFeatureStatus(slug)
		if err != nil {
			t.Fatalf("load %s: %v", slug, err)
		}
		st.DependsOn = []Dependency{{Slug: "target", Kind: DependencyKindSupersedes}}
		if err := s.SaveFeatureStatus(st); err != nil {
			t.Fatalf("save %s: %v", slug, err)
		}
	}
	errs := ValidateAllFeatures(s)
	found := false
	for _, e := range errs {
		if errors.Is(e, ErrMultipleActiveSuperseders) {
			msg := e.Error()
			if strings.Contains(msg, "peer-a") && strings.Contains(msg, "peer-b") && strings.Contains(msg, "target") {
				found = true
				break
			}
			t.Fatalf("ErrMultipleActiveSuperseders error must name all superseders and the target; got %q", msg)
		}
	}
	if !found {
		t.Fatalf("expected ErrMultipleActiveSuperseders in bulk errors; got %v", errs)
	}
}
