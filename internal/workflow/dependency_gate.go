package workflow

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// ErrParentNotApplied is returned by CheckDependencyGate when a child
// feature's hard-dependency parents have not yet reached an apply-gate
// satisfying state ("applied" or "upstream_merged"). Wrappable via
// errors.Is. Soft dependencies are never reported here — they are
// ordering hints only (ADR-011 D4).
var ErrParentNotApplied = errors.New("hard parent dependency not applied")

// satisfiedBySHA matches a 40-character lowercase-or-uppercase hex string.
// PRD §3.2 / ADR-011 D5: `satisfied_by` is provenance, not a runtime guard.
// CheckDependencyGate validates only that, when set, the value looks like
// a commit SHA. We deliberately do NOT run `git merge-base` reachability
// checks in M14.2 — that is documented as a follow-up; ADR-011 D5 treats
// `satisfied_by` as provenance metadata, not as a gate.
var satisfiedBySHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// CheckDependencyGate enforces ADR-011 D4: hard parents must be applied
// or upstream_merged before a child can run apply --mode execute / auto.
// Soft parents are ordering hints only and do NOT gate apply.
//
// When Config.DAGEnabled() is false, this function is a no-op and returns
// nil — pre-M14 behaviour is preserved byte-for-byte (ADR-011 D9).
//
// IMPORTANT: For reconcile-result decisions, callers must read
// status.Reconcile.Outcome — never artifacts/reconcile-session.json (see
// ADR-010 D5). This function reads only FeatureStatus.State and the
// dependency declaration; it does not consult reconcile artifacts.
//
// Returns an error wrapping ErrParentNotApplied with a message listing the
// blocking parent slugs and their current states. Soft parents and hard
// parents already satisfied are excluded from the message.
func CheckDependencyGate(s *store.Store, slug string) error {
	cfg, err := s.LoadConfig()
	if err != nil {
		return err
	}
	if !cfg.DAGEnabled() {
		return nil
	}

	child, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return fmt.Errorf("dependency gate: cannot load feature status for %q: %w", slug, err)
	}
	if len(child.DependsOn) == 0 {
		return nil
	}

	type blocker struct {
		slug  string
		state string
		note  string
	}
	var blockers []blocker

	for _, dep := range child.DependsOn {
		if dep.Kind != store.DependencyKindHard {
			continue
		}
		parent, err := s.LoadFeatureStatus(dep.Slug)
		if err != nil {
			blockers = append(blockers, blocker{
				slug:  dep.Slug,
				state: "<missing>",
				note:  fmt.Sprintf("cannot load parent status: %v", err),
			})
			continue
		}
		switch parent.State {
		case store.StateApplied:
			continue
		case store.StateUpstreamMerged:
			if dep.SatisfiedBy != "" && !satisfiedBySHA.MatchString(dep.SatisfiedBy) {
				blockers = append(blockers, blocker{
					slug:  dep.Slug,
					state: string(parent.State),
					note:  fmt.Sprintf("satisfied_by %q is not a 40-hex commit SHA", dep.SatisfiedBy),
				})
				continue
			}
			// upstream_merged satisfies the gate (PRD §3.2). When
			// satisfied_by is present and well-formed it is treated
			// as provenance only; reachability is intentionally not
			// checked here (ADR-011 D5, M14.2 documented limitation).
			continue
		default:
			blockers = append(blockers, blocker{
				slug:  dep.Slug,
				state: string(parent.State),
			})
		}
	}

	if len(blockers) == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "feature %q has %d unsatisfied hard dependency(ies):", slug, len(blockers))
	for _, p := range blockers {
		fmt.Fprintf(&b, "\n  - %s (state=%s)", p.slug, p.state)
		if p.note != "" {
			fmt.Fprintf(&b, " — %s", p.note)
		}
	}
	b.WriteString("\nRun `tpatch apply <parent>` (or merge upstream) for each blocking parent before retrying.")
	return fmt.Errorf("%w: %s", ErrParentNotApplied, b.String())
}
