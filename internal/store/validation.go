package store

import (
	"errors"
	"fmt"
	"os"
)

// Sentinel errors for dependency validation. Callers can match with
// errors.Is. The 5 rules are sourced from PRD §3.3 and ADR-011 D5.
var (
	// ErrSelfDependency is returned when a feature declares itself as a parent.
	ErrSelfDependency = errors.New("feature cannot depend on itself")
	// ErrDanglingDependency is returned when a parent slug does not exist in the store.
	ErrDanglingDependency = errors.New("dependency references unknown feature")
	// ErrKindConflict is returned when the same parent is declared with conflicting kinds.
	ErrKindConflict = errors.New("dependency kind conflict")
	// ErrSatisfiedByRequiresUpstream is returned when satisfied_by is set on a parent
	// whose state is not upstream_merged (ADR-011 D5).
	ErrSatisfiedByRequiresUpstream = errors.New("satisfied_by is only valid for upstream_merged parents")
	// ErrInvalidDependencyKind is returned when kind is neither "hard" nor "soft".
	ErrInvalidDependencyKind = errors.New("dependency kind must be \"hard\" or \"soft\"")
)

// ValidateDependencies checks the proposed dependency list for `slug`
// against the live store, applying the 5 rules from PRD §3.3:
//
//  1. No self-dependency.
//  2. No dangling refs (every parent must exist in the store).
//  3. No kind conflict (same parent declared twice with different kinds).
//  4. No cycles (global graph including the proposed change).
//  5. satisfied_by is only valid when the parent's state is upstream_merged.
//
// Returns the first violation as a wrapped sentinel error so callers can
// errors.Is-match. To get *all* violations across all features at once,
// use ValidateAllFeatures.
func ValidateDependencies(s *Store, slug string, deps []Dependency) error {
	// Rule 1: self-dependency, plus kind sanity.
	seen := make(map[string]string, len(deps))
	for _, d := range deps {
		if d.Slug == slug {
			return fmt.Errorf("%w: %s", ErrSelfDependency, slug)
		}
		if d.Kind != DependencyKindHard && d.Kind != DependencyKindSoft {
			return fmt.Errorf("%w: parent %s has kind %q", ErrInvalidDependencyKind, d.Slug, d.Kind)
		}
		// Rule 3: kind conflict on duplicate parent.
		if prev, dup := seen[d.Slug]; dup && prev != d.Kind {
			return fmt.Errorf("%w: parent %s declared as both %s and %s", ErrKindConflict, d.Slug, prev, d.Kind)
		}
		seen[d.Slug] = d.Kind
	}

	// Rules 2 + 5: per-parent existence and satisfied_by gate.
	for _, d := range deps {
		parent, err := s.LoadFeatureStatus(d.Slug)
		if err != nil {
			if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%w: %s -> %s", ErrDanglingDependency, slug, d.Slug)
			}
			return fmt.Errorf("load parent %s: %w", d.Slug, err)
		}
		if d.SatisfiedBy != "" && parent.State != StateUpstreamMerged {
			return fmt.Errorf("%w: parent %s state is %q (need upstream_merged)", ErrSatisfiedByRequiresUpstream, d.Slug, parent.State)
		}
	}

	// Rule 4: cycle detection on the global graph including the proposed change.
	graph, err := loadGraphWithOverride(s, slug, deps)
	if err != nil {
		return err
	}
	if cyc, cerr := DetectCycles(graph); cerr != nil {
		_ = cyc // path is already in the wrapped error message
		return cerr
	}
	return nil
}

// ValidateAllFeatures runs the 5 validation rules across every feature in
// the store and returns every violation found, not just the first. Useful
// for bulk health-checks (see M14.4 `tpatch status --dag`). Errors are
// independent (one bad edge does not short-circuit unrelated features).
func ValidateAllFeatures(s *Store) []error {
	feats, err := s.ListFeatures()
	if err != nil {
		return []error{err}
	}
	// Build the index once so we can resolve parents without re-reading status.
	index := make(map[string]FeatureStatus, len(feats))
	graph := make(map[string][]Dependency, len(feats))
	for _, f := range feats {
		index[f.Slug] = f
		graph[f.Slug] = f.DependsOn
	}

	var out []error
	for _, f := range feats {
		seen := make(map[string]string, len(f.DependsOn))
		for _, d := range f.DependsOn {
			if d.Slug == f.Slug {
				out = append(out, fmt.Errorf("%w: %s", ErrSelfDependency, f.Slug))
				continue
			}
			if d.Kind != DependencyKindHard && d.Kind != DependencyKindSoft {
				out = append(out, fmt.Errorf("%w: %s -> %s kind %q", ErrInvalidDependencyKind, f.Slug, d.Slug, d.Kind))
				continue
			}
			if prev, dup := seen[d.Slug]; dup && prev != d.Kind {
				out = append(out, fmt.Errorf("%w: %s -> %s declared as both %s and %s", ErrKindConflict, f.Slug, d.Slug, prev, d.Kind))
			}
			seen[d.Slug] = d.Kind

			parent, ok := index[d.Slug]
			if !ok {
				out = append(out, fmt.Errorf("%w: %s -> %s", ErrDanglingDependency, f.Slug, d.Slug))
				continue
			}
			if d.SatisfiedBy != "" && parent.State != StateUpstreamMerged {
				out = append(out, fmt.Errorf("%w: %s -> %s parent state %q", ErrSatisfiedByRequiresUpstream, f.Slug, d.Slug, parent.State))
			}
		}
	}

	// Single global cycle check — surface once, with the cycle path.
	if _, cerr := DetectCycles(graph); cerr != nil {
		out = append(out, cerr)
	}
	return out
}

// loadGraphWithOverride builds the full feature dependency graph by
// reading every feature's status.json, then substitutes deps for the
// supplied slug (modeling the proposed write before it is persisted).
// Used by cycle detection in ValidateDependencies.
func loadGraphWithOverride(s *Store, slug string, deps []Dependency) (map[string][]Dependency, error) {
	feats, err := s.ListFeatures()
	if err != nil {
		return nil, err
	}
	graph := make(map[string][]Dependency, len(feats)+1)
	for _, f := range feats {
		graph[f.Slug] = f.DependsOn
	}
	graph[slug] = deps
	return graph, nil
}
