package workflow

import (
	"errors"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// gateTestEnv builds an isolated repo store with the given config flag and
// returns it ready for AddFeature calls.
func gateTestEnv(t *testing.T, dagEnabled bool) *store.Store {
	t.Helper()
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	cfg, _ := s.LoadConfig()
	cfg.FeaturesDependencies = dagEnabled
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	return s
}

// addFeature seeds a feature with the given state and optional deps.
func addFeature(t *testing.T, s *store.Store, slug string, state store.FeatureState, deps []store.Dependency) {
	t.Helper()
	if _, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: "test"}); err != nil {
		t.Fatalf("AddFeature %s: %v", slug, err)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus %s: %v", slug, err)
	}
	st.State = state
	st.DependsOn = deps
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatalf("SaveFeatureStatus %s: %v", slug, err)
	}
}

func TestDependencyGate_FlagOff_PassesEvenWithUnappliedHardParent(t *testing.T) {
	s := gateTestEnv(t, false)
	addFeature(t, s, "parent", store.StateAnalyzed, nil)
	addFeature(t, s, "child", store.StateImplementing, []store.Dependency{
		{Slug: "parent", Kind: store.DependencyKindHard},
	})
	if err := CheckDependencyGate(s, "child"); err != nil {
		t.Fatalf("flag off must be no-op, got: %v", err)
	}
}

func TestDependencyGate_RejectsHardUnapplied(t *testing.T) {
	s := gateTestEnv(t, true)
	addFeature(t, s, "parent", store.StateAnalyzed, nil)
	addFeature(t, s, "child", store.StateImplementing, []store.Dependency{
		{Slug: "parent", Kind: store.DependencyKindHard},
	})
	err := CheckDependencyGate(s, "child")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrParentNotApplied) {
		t.Fatalf("want errors.Is ErrParentNotApplied, got %v", err)
	}
	if !strings.Contains(err.Error(), "parent") {
		t.Fatalf("error must mention parent slug; got: %v", err)
	}
}

func TestDependencyGate_AllowsHardApplied(t *testing.T) {
	s := gateTestEnv(t, true)
	addFeature(t, s, "parent", store.StateApplied, nil)
	addFeature(t, s, "child", store.StateImplementing, []store.Dependency{
		{Slug: "parent", Kind: store.DependencyKindHard},
	})
	if err := CheckDependencyGate(s, "child"); err != nil {
		t.Fatalf("hard parent applied must pass: %v", err)
	}
}

func TestDependencyGate_AllowsHardUpstreamMergedNoSatisfiedBy(t *testing.T) {
	s := gateTestEnv(t, true)
	addFeature(t, s, "parent", store.StateUpstreamMerged, nil)
	addFeature(t, s, "child", store.StateImplementing, []store.Dependency{
		{Slug: "parent", Kind: store.DependencyKindHard},
	})
	if err := CheckDependencyGate(s, "child"); err != nil {
		t.Fatalf("upstream_merged without satisfied_by must pass: %v", err)
	}
}

func TestDependencyGate_AllowsHardUpstreamMergedWithSatisfiedBy(t *testing.T) {
	s := gateTestEnv(t, true)
	addFeature(t, s, "parent", store.StateUpstreamMerged, nil)
	addFeature(t, s, "child", store.StateImplementing, []store.Dependency{
		{
			Slug:        "parent",
			Kind:        store.DependencyKindHard,
			SatisfiedBy: "abc1234567890123456789012345678901234567",
		},
	})
	if err := CheckDependencyGate(s, "child"); err != nil {
		t.Fatalf("upstream_merged with valid satisfied_by must pass: %v", err)
	}
}

func TestDependencyGate_RejectsHardUpstreamMergedBadSatisfiedBy(t *testing.T) {
	s := gateTestEnv(t, true)
	addFeature(t, s, "parent", store.StateUpstreamMerged, nil)
	addFeature(t, s, "child", store.StateImplementing, []store.Dependency{
		{Slug: "parent", Kind: store.DependencyKindHard, SatisfiedBy: "not-a-sha"},
	})
	err := CheckDependencyGate(s, "child")
	if err == nil || !errors.Is(err, ErrParentNotApplied) {
		t.Fatalf("malformed satisfied_by must block; got %v", err)
	}
}

func TestDependencyGate_IgnoresSoftDeps(t *testing.T) {
	s := gateTestEnv(t, true)
	addFeature(t, s, "soft-parent", store.StateAnalyzed, nil)
	addFeature(t, s, "child", store.StateImplementing, []store.Dependency{
		{Slug: "soft-parent", Kind: store.DependencyKindSoft},
	})
	if err := CheckDependencyGate(s, "child"); err != nil {
		t.Fatalf("soft deps must never block apply: %v", err)
	}
}

func TestDependencyGate_MixedDeps(t *testing.T) {
	s := gateTestEnv(t, true)
	addFeature(t, s, "hard-applied", store.StateApplied, nil)
	addFeature(t, s, "hard-pending", store.StateAnalyzed, nil)
	addFeature(t, s, "soft-pending", store.StateAnalyzed, nil)
	addFeature(t, s, "child", store.StateImplementing, []store.Dependency{
		{Slug: "hard-applied", Kind: store.DependencyKindHard},
		{Slug: "hard-pending", Kind: store.DependencyKindHard},
		{Slug: "soft-pending", Kind: store.DependencyKindSoft},
	})
	err := CheckDependencyGate(s, "child")
	if err == nil || !errors.Is(err, ErrParentNotApplied) {
		t.Fatalf("expected ErrParentNotApplied, got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "hard-pending") {
		t.Errorf("error must mention hard-pending: %s", msg)
	}
	if strings.Contains(msg, "hard-applied") {
		t.Errorf("error must NOT mention satisfied hard-applied: %s", msg)
	}
	if strings.Contains(msg, "soft-pending") {
		t.Errorf("error must NOT mention soft-pending: %s", msg)
	}
}

func TestDependencyGate_NoDeps(t *testing.T) {
	s := gateTestEnv(t, true)
	addFeature(t, s, "child", store.StateImplementing, nil)
	if err := CheckDependencyGate(s, "child"); err != nil {
		t.Fatalf("no deps must pass: %v", err)
	}
}
