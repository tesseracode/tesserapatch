package store

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// satisfiedBySHARe matches a 40-character hex commit SHA. This must
// stay byte-identical to the regex enforced by the apply-time
// dependency gate (`internal/workflow.satisfiedBySHA`) so that what
// validation accepts is exactly what the gate accepts. M15-W2 review
// caught a contract drift where validation accepted unique short SHAs
// (any ref `git merge-base --is-ancestor` could resolve) while the
// gate still rejected anything not 40-hex, producing save-now/fail-
// later dependency paths.
var satisfiedBySHARe = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

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
	// ErrSatisfiedByMalformed is returned when satisfied_by is set but
	// is not a 40-character hex commit SHA. Mirrors the contract
	// enforced by the apply-time dependency gate so validation and
	// gate accept exactly the same value space.
	ErrSatisfiedByMalformed = errors.New("satisfied_by must be a 40-character hex commit SHA")
	// ErrSatisfiedBySHANotReachable is returned when satisfied_by points at a SHA
	// that is not an ancestor of HEAD. Closes the deliberate M14.1 limitation
	// where any well-formed hex string was accepted as long as the parent was
	// upstream_merged. The provenance value must correspond to a commit we can
	// actually see from the current branch tip.
	ErrSatisfiedBySHANotReachable = errors.New("satisfied_by SHA not reachable from HEAD")
	// ErrInvalidDependencyKind is returned when kind is not one of
	// "hard", "soft", or "supersedes" (ADR-011 D4 + ADR-028 D1).
	ErrInvalidDependencyKind = errors.New("dependency kind must be \"hard\", \"soft\", or \"supersedes\"")
	// ErrMultipleActiveSuperseders is returned when more than one
	// healthy superseder targets the same historical feature. Enforces
	// PRD-feature-supersession AC-4 + ADR-028 D5: "A historical
	// feature may have at most one active/effective superseder."
	// v0.12.0 rev-1 (F-SEXT-3): write-time rejection replaces the
	// prior first-healthy-wins silent selection.
	ErrMultipleActiveSuperseders = errors.New("multiple active superseders for the same target")
	// ErrRejectedParent is returned when a proposed dependency edge
	// points at a parent feature whose state is `rejected` (Rule 7,
	// v0.13.0 GH #6 — ADR-031 D8 / PRD-rejected-feature-state §5
	// "Symmetric invariant"). Applies to all three edge kinds
	// (hard/soft/supersedes) with no per-kind carve-out, so that
	// whichever order an operator performs the two independent
	// operations (reject the parent then add the edge, or add the
	// edge then reject the parent) the same outcome results: a
	// rejected feature never has live dependents.
	//
	// The sentinel's own text is the leading clause of PRD §8's golden
	// error string, so `fmt.Errorf("%w: ...", ErrRejectedParent, ...)`
	// renders the spec message verbatim (Cluster F' rev-1, F-EXT-1).
	ErrRejectedParent = errors.New("cannot add dependency")
)

// supersederValidationHealthyStates lists the FeatureStates that
// qualify a superseder as "active/effective" for AC-4 / ADR-028 D5
// fan-in validation. Must stay byte-identical with
// workflow.supersederHealthyStates so what validation rejects is
// exactly what reconcile/verify would treat as an authoritative
// exclusion. See internal/workflow/labels.go:supersederHealthyStates.
var supersederValidationHealthyStates = map[FeatureState]struct{}{
	StateApplied:        {},
	StateActive:         {},
	StateUpstreamMerged: {},
}

// supersederValidationBlockedOutcomes lists the ReconcileOutcomes that
// disqualify a superseder from the "healthy" set even when its state is
// applied/active/upstream_merged. Must stay byte-identical with
// workflow.blockedReconcileOutcomes.
var supersederValidationBlockedOutcomes = map[ReconcileOutcome]struct{}{
	ReconcileBlockedRequiresHuman:    {},
	ReconcileBlockedTooManyConflicts: {},
	ReconcileBlocked:                 {},
	ReconcileShadowAwaiting:          {},
}

// supersederIsHealthyForValidation mirrors workflow.supersederIsHealthy
// so store-level fan-in validation and workflow-level reconcile
// suppression treat the same superseder identically. See the F-SEXT-3
// contract in PRD-feature-supersession AC-4 + ADR-028 D5.
func supersederIsHealthyForValidation(f FeatureStatus) bool {
	if _, ok := supersederValidationHealthyStates[f.State]; !ok {
		return false
	}
	if _, blocked := supersederValidationBlockedOutcomes[f.Reconcile.Outcome]; blocked {
		return false
	}
	return true
}

// isAncestor is a package-level hook so unit tests can stub the git
// reachability check without standing up a real repo. Default wires
// straight to gitutil.IsAncestor.
var isAncestor = gitutil.IsAncestor

// rejectionReasonSuffix renders ` (reason=<code>)` when the feature
// carries a rejection record with a reason, and the empty string
// otherwise. Keeps Rule 7's message actionable without exploding when a
// hand-edited status.json sets `state: rejected` with no sub-record.
func rejectionReasonSuffix(f FeatureStatus) string {
	if f.Rejection == nil || f.Rejection.Reason == "" {
		return ""
	}
	return " (reason=" + f.Rejection.Reason + ")"
}

// ValidateDependencies checks the proposed dependency list for `slug`
// against the live store, applying the 7 rules from PRD §3.3 (rules 1-5),
// PRD-feature-supersession AC-4 (rule 6) and
// PRD-rejected-feature-state §5 (rule 7):
//
//  1. No self-dependency.
//  2. No dangling refs (every parent must exist in the store).
//  3. No kind conflict (same parent declared twice with different kinds).
//  4. No cycles (global graph including the proposed change).
//  5. satisfied_by is only valid when the parent's state is upstream_merged.
//  6. At most one active/effective superseder per target.
//  7. No edge — hard, soft, or supersedes — onto a `rejected` parent.
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
		if d.Kind != DependencyKindHard && d.Kind != DependencyKindSoft && d.Kind != DependencyKindSupersedes {
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
		// Rule 7 (v0.13.0 GH #6 — ADR-031 D8 / PRD-rejected-feature-state
		// §5 "Symmetric invariant"): refuse edge creation onto a parent
		// that is currently `rejected`, for every edge kind. The
		// remediation is to reopen the parent — NOT to silently drop the
		// dependency. Classified as a state-machine refusal (exit code 3
		// at the CLI layer, ADR-031 D4 addendum): the input is well
		// formed, but the current state of the store makes the edge
		// invalid right now.
		if parent.State == StateRejected {
			return fmt.Errorf(
				"%w: parent %q is rejected%s; run `tpatch reopen %s` first if this dependency is still needed",
				ErrRejectedParent, d.Slug, rejectionReasonSuffix(parent), d.Slug,
			)
		}
		if d.SatisfiedBy != "" && parent.State != StateUpstreamMerged {
			return fmt.Errorf("%w: parent %s state is %q (need upstream_merged)", ErrSatisfiedByRequiresUpstream, d.Slug, parent.State)
		}
		// satisfied_by must be a well-formed 40-hex SHA AND point at a
		// commit reachable from HEAD. The 40-hex check mirrors the
		// apply-time gate's contract; the reachability check hardens
		// against fabricated-but-well-formed values.
		if d.SatisfiedBy != "" && parent.State == StateUpstreamMerged {
			if !satisfiedBySHARe.MatchString(d.SatisfiedBy) {
				return fmt.Errorf("%w: %s -> %s satisfied_by=%q", ErrSatisfiedByMalformed, slug, d.Slug, d.SatisfiedBy)
			}
			ok, aerr := isAncestor(s.Root, d.SatisfiedBy, "HEAD")
			if aerr != nil {
				return fmt.Errorf("verify satisfied_by reachability for %s -> %s: %w", slug, d.Slug, aerr)
			}
			if !ok {
				return fmt.Errorf("%w: %s -> %s satisfied_by=%s", ErrSatisfiedBySHANotReachable, slug, d.Slug, d.SatisfiedBy)
			}
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

	// Rule 6 (v0.12.0 rev-1 F-SEXT-3 — PRD-feature-supersession AC-4
	// + ADR-028 D5): a historical feature may have at most one
	// active/effective superseder. When the caller's proposed edges
	// include `kind: supersedes -> T`, scan the store for any other
	// feature that also declares `supersedes -> T` and is currently
	// healthy. If found, reject the write with an actionable error
	// naming both superseder slugs. `slug` (the caller) is treated as
	// presumed-healthy for the purposes of this check: even a freshly
	// authored draft edge is enough to make the second-superseder
	// state ambiguous per D5.
	feats, ferr := s.ListFeatures()
	if ferr == nil {
		for _, d := range deps {
			if d.Kind != DependencyKindSupersedes {
				continue
			}
			for _, other := range feats {
				if other.Slug == slug {
					continue
				}
				if !supersederIsHealthyForValidation(other) {
					continue
				}
				for _, od := range other.DependsOn {
					if od.Kind != DependencyKindSupersedes {
						continue
					}
					if od.Slug != d.Slug {
						continue
					}
					return fmt.Errorf(
						"%w: target %s is already superseded by healthy feature %s; refusing to add second superseder %s. "+
							"Resolution: remove the existing supersedes edge on %s or the new one on %s before proceeding (PRD-feature-supersession AC-4 / ADR-028 D5).",
						ErrMultipleActiveSuperseders, d.Slug, other.Slug, slug, other.Slug, slug,
					)
				}
			}
		}
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
			if d.Kind != DependencyKindHard && d.Kind != DependencyKindSoft && d.Kind != DependencyKindSupersedes {
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
			if d.SatisfiedBy != "" && parent.State == StateUpstreamMerged {
				if !satisfiedBySHARe.MatchString(d.SatisfiedBy) {
					out = append(out, fmt.Errorf("%w: %s -> %s satisfied_by=%q", ErrSatisfiedByMalformed, f.Slug, d.Slug, d.SatisfiedBy))
					continue
				}
				ok, aerr := isAncestor(s.Root, d.SatisfiedBy, "HEAD")
				if aerr != nil {
					out = append(out, fmt.Errorf("verify satisfied_by reachability for %s -> %s: %w", f.Slug, d.Slug, aerr))
				} else if !ok {
					out = append(out, fmt.Errorf("%w: %s -> %s satisfied_by=%s", ErrSatisfiedBySHANotReachable, f.Slug, d.Slug, d.SatisfiedBy))
				}
			}
		}
	}

	// Single global cycle check — surface once, with the cycle path.
	if _, cerr := DetectCycles(graph); cerr != nil {
		out = append(out, cerr)
	}

	// v0.12.0 rev-1 F-SEXT-3 — PRD-feature-supersession AC-4 + ADR-028
	// D5: bulk fan-in scan. For each target T, tally the healthy
	// superseders pointing at it; more than one is a validation
	// violation (a data-corruption path since ValidateDependencies now
	// rejects the second edge at write time). Emit one error per
	// conflicted target so `status --dag` surfaces every conflict.
	// Sort the superseder slugs to keep the error message deterministic.
	healthySupersedersByTarget := make(map[string][]string)
	for _, f := range feats {
		if !supersederIsHealthyForValidation(f) {
			continue
		}
		for _, d := range f.DependsOn {
			if d.Kind != DependencyKindSupersedes {
				continue
			}
			healthySupersedersByTarget[d.Slug] = append(healthySupersedersByTarget[d.Slug], f.Slug)
		}
	}
	// Sort target keys so output order is deterministic across runs.
	targetKeys := make([]string, 0, len(healthySupersedersByTarget))
	for t := range healthySupersedersByTarget {
		targetKeys = append(targetKeys, t)
	}
	sort.Strings(targetKeys)
	for _, t := range targetKeys {
		peers := healthySupersedersByTarget[t]
		if len(peers) < 2 {
			continue
		}
		sort.Strings(peers)
		out = append(out, fmt.Errorf(
			"%w: target %s is superseded by %d healthy features [%s]; ADR-028 D5 permits at most one. "+
				"Resolution: remove the extra supersedes edge(s) so exactly one healthy superseder remains.",
			ErrMultipleActiveSuperseders, t, len(peers), strings.Join(peers, ", "),
		))
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
