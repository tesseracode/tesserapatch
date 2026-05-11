package store

import (
	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// FeatureRef identifies which feature references a SHA and via which
// field. `Kind` is one of "satisfied_by" or "base_commit". The owning
// feature is the one whose feature.yaml/status.json carries the
// reference — for `satisfied_by` that is the CHILD feature (the one
// whose dependency edge is annotated with the parent's commit), not
// the upstream-merged parent.
//
// See feat-amend-dependent-warning (v0.7.0): used by both
// `tpatch status` (post-hoc dependent-broken label) and `tpatch
// record` (pre-amend orphaning gate).
type FeatureRef struct {
	Feature string // owning feature slug
	Kind    string // "satisfied_by" | "base_commit"
	SHA     string // 40-hex commit SHA
}

// FeatureRefKind constants. Kept as plain strings (not a typed enum)
// because they show up directly in JSON output via the
// `tpatch status --json` `broken_refs[].kind` field.
const (
	FeatureRefKindSatisfiedBy = "satisfied_by"
	FeatureRefKindBaseCommit  = "base_commit"
)

// CollectDependentSHAs walks every feature in the store and returns a
// map from SHA → list of features that reference it (and via which
// field). The returned map is keyed by the LITERAL SHA value as it
// appears on disk (no normalization — callers comparing against
// reachability checks should pass the same value through to
// `git merge-base --is-ancestor`).
//
// SHAs that are empty strings are skipped (a feature with no
// `apply.base_commit` recorded has nothing to break). Cache lifetime
// is the duration of the call — callers that want to reuse the map
// across multiple lookups within a single CLI invocation should hold
// onto the returned value.
func CollectDependentSHAs(s *Store) (map[string][]FeatureRef, error) {
	feats, err := s.ListFeatures()
	if err != nil {
		return nil, err
	}
	out := make(map[string][]FeatureRef)
	for _, f := range feats {
		if sha := f.Apply.BaseCommit; sha != "" {
			out[sha] = append(out[sha], FeatureRef{
				Feature: f.Slug,
				Kind:    FeatureRefKindBaseCommit,
				SHA:     sha,
			})
		}
		for _, d := range f.DependsOn {
			if d.SatisfiedBy == "" {
				continue
			}
			out[d.SatisfiedBy] = append(out[d.SatisfiedBy], FeatureRef{
				Feature: f.Slug,
				Kind:    FeatureRefKindSatisfiedBy,
				SHA:     d.SatisfiedBy,
			})
		}
	}
	return out, nil
}

// IsAmendBreaking returns the FeatureRef list that would be orphaned
// if `prevHead` is rewritten away (i.e. is no longer reachable from
// the new HEAD). An empty list means the amend is safe with respect
// to recorded dependent references.
//
// The function is a pure lookup — reachability is the caller's
// responsibility (the brief amend-detection in the record command
// already knows the prevHead SHA via the reflog and just needs to
// know whether anyone depends on it).
func IsAmendBreaking(prevHead string, dependents map[string][]FeatureRef) []FeatureRef {
	if prevHead == "" || len(dependents) == 0 {
		return nil
	}
	refs, ok := dependents[prevHead]
	if !ok {
		return nil
	}
	// Defensive copy so callers can mutate the slice without aliasing
	// the cached map.
	out := make([]FeatureRef, len(refs))
	copy(out, refs)
	return out
}

// CollectBrokenRefs walks the store and returns every dependent
// reference whose SHA is no longer reachable from HEAD. Used by
// `tpatch status` to compute the `dependent-broken` label.
//
// Reachability is checked via gitutil.IsAncestor (delegating to
// `git merge-base --is-ancestor`). The same SHA is checked at most
// once per call (de-duped via the dependents map).
//
// Returns broken refs grouped by owning feature slug, so callers can
// emit one diagnostic line per affected feature even when several
// references break simultaneously.
func CollectBrokenRefs(s *Store) (map[string][]FeatureRef, error) {
	dependents, err := CollectDependentSHAs(s)
	if err != nil {
		return nil, err
	}
	if len(dependents) == 0 {
		return nil, nil
	}
	byFeature := make(map[string][]FeatureRef)
	for sha, refs := range dependents {
		ok, aerr := gitutil.IsAncestor(s.Root, sha, "HEAD")
		if aerr != nil {
			// Treat a real git failure (bad SHA, repo missing) as
			// "broken" — the same SHA cannot be a valid ancestor of
			// HEAD if git can't resolve it. Surface it under the
			// owning feature(s) so the user sees something rather
			// than silently dropping the diagnostic.
			ok = false
		}
		if ok {
			continue
		}
		for _, r := range refs {
			byFeature[r.Feature] = append(byFeature[r.Feature], r)
		}
	}
	return byFeature, nil
}
