package workflow

// Slice D of the freshness-overlay design (PRD-verify-freshness §9, §4.4, Q2).
//
// Scope:
//   - `RunVerifyAll` is the additive aggregate orchestrator behind the
//     `tpatch verify --all` flag. It walks every feature in
//     `.tpatch/features/`, topologically ordered (parents first) using
//     the existing `store.TopologicalOrder` Kahn primitive, and either
//     dispatches each post-apply feature through the unchanged
//     single-feature `RunVerify` entry point OR records a
//     `skipped: pre-apply` row at that feature's topo position.
//   - Pre-apply skip is decided BEFORE V0 runs. A feature whose
//     lifecycle state is not in `postApplyVerifyStates` does not produce
//     a 10-check report and does not contribute to a non-zero exit on
//     its own (PRD Q2). The skip row is emitted at the feature's topo
//     position, not at the end (CURRENT.md "deterministic and ordered
//     first in topo").
//   - V3–V9 logic in verify.go is NOT touched. Slice C is closed. This
//     file is additive surface only.
//   - No new state transitions. Verify remains a freshness overlay.

import (
	"fmt"
	"io"
	"sort"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// aggregateSchemaVersion is independent of the per-feature
// `verifySchemaVersion`; both happen to be "1.0" today but the aggregate
// envelope can rev independently of the per-feature shape.
const aggregateSchemaVersion = "1.0"

// AggregateFeatureStatus enumerates the states a single feature row can
// be in within an aggregate run. Frozen vocabulary; harness consumers
// may switch on these strings.
const (
	AggregateStatusPassed  = "passed"  // RunVerify returned verdict=passed.
	AggregateStatusFailed  = "failed"  // RunVerify returned verdict=failed (any check or V0 abort).
	AggregateStatusRefused = "refused" // RunVerify returned a *RefusedError despite the pre-check.
	AggregateStatusSkipped = "skipped" // Pre-apply skip — RunVerify never invoked.
	AggregateStatusError   = "error"   // Pre-flight load failed; RunVerify could not be dispatched.
)

// AggregateFeatureEntry is one row in the `features:` array of a
// `verify --all` run. `Report` is omitted for skipped/error rows where
// no per-feature report exists.
type AggregateFeatureEntry struct {
	Slug           string             `json:"slug"`
	Status         string             `json:"status"`
	Reason         string             `json:"reason,omitempty"`
	LifecycleState store.FeatureState `json:"lifecycle_state,omitempty"`
	Report         *VerifyReport      `json:"report,omitempty"`
}

// AggregateSummary is the four-bucket footer counts. Matches the
// vocabulary in CURRENT.md "Aggregate JSON shape".
type AggregateSummary struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Error   int `json:"error"`
}

// AggregateReport is the full result of `RunVerifyAll`. JSON-serialised
// directly under `tpatch verify --all --json`.
type AggregateReport struct {
	SchemaVersion string                  `json:"schema_version"`
	Features      []AggregateFeatureEntry `json:"features"`
	Summary       AggregateSummary        `json:"summary"`
}

// HasFailures reports whether the aggregate run should produce a
// non-zero exit code per PRD Q7. Pre-apply skips are non-fatal; a
// pre-flight error (e.g. unreadable status.json) is fatal so the
// operator notices.
func (r *AggregateReport) HasFailures() bool {
	if r == nil {
		return false
	}
	return r.Summary.Failed > 0 || r.Summary.Error > 0
}

// isPostApplyState mirrors the package-private `postApplyVerifyStates`
// gate used by RunVerify. Duplicated by intent so verify_all.go has no
// reach into verify.go's V3–V9 internals (Slice C closed).
func isPostApplyState(s store.FeatureState) bool {
	switch s {
	case store.StateApplied,
		store.StateActive,
		store.StateUpstreamMerged,
		store.StateBlocked:
		return true
	}
	return false
}

// RunVerifyAll executes verify across every tracked feature in
// topological order (parents first, ties broken lexicographically by
// slug — same determinism contract as `store.TopologicalOrder`).
//
// Pre-apply features are skipped at their topo position with a
// synthetic row; their RunVerify is NOT invoked, so V0 does not fire
// and no `Verify` record is written to status.json.
//
// Post-apply features dispatch through the unchanged single-feature
// `RunVerify` entry point. opts is forwarded as-is — `--no-write`
// honoured per-feature.
//
// Returns a non-nil report unless the store layer cannot enumerate
// features at all (in which case the error is surfaced to the caller
// for an exit-2 abort).
func RunVerifyAll(s *store.Store, opts VerifyOptions) (*AggregateReport, error) {
	if s == nil {
		return nil, fmt.Errorf("verify --all: nil store")
	}

	feats, err := s.ListFeatures()
	if err != nil {
		return nil, fmt.Errorf("verify --all: list features: %w", err)
	}

	report := &AggregateReport{
		SchemaVersion: aggregateSchemaVersion,
		Features:      make([]AggregateFeatureEntry, 0, len(feats)),
	}

	if len(feats) == 0 {
		return report, nil
	}

	// Build the dep graph using the SAME convention store.TopologicalOrder
	// expects (child → parents). Dangling parents are tolerated by
	// TopologicalOrder; a cycle returns an error and aborts the run.
	graph := make(map[string][]store.Dependency, len(feats))
	stateBySlug := make(map[string]store.FeatureState, len(feats))
	for _, f := range feats {
		graph[f.Slug] = f.DependsOn
		stateBySlug[f.Slug] = f.State
	}

	order, terr := store.TopologicalOrder(graph)
	if terr != nil {
		// Cycle (or other graph error). Fall back to lexicographic
		// order so the report still surfaces every feature with a
		// flagged "error" row at the head; the operator gets the cycle
		// diagnostic too.
		order = make([]string, 0, len(feats))
		for slug := range graph {
			order = append(order, slug)
		}
		sort.Strings(order)
		report.Features = append(report.Features, AggregateFeatureEntry{
			Slug:   "(graph)",
			Status: AggregateStatusError,
			Reason: fmt.Sprintf("dependency graph error: %v", terr),
		})
		report.Summary.Error++
	}

	for _, slug := range order {
		state := stateBySlug[slug]

		// Pre-apply gate — decided BEFORE V0 fires.
		if !isPostApplyState(state) {
			report.Features = append(report.Features, AggregateFeatureEntry{
				Slug:           slug,
				Status:         AggregateStatusSkipped,
				Reason:         fmt.Sprintf("pre-apply (state=%s)", state),
				LifecycleState: state,
			})
			report.Summary.Skipped++
			continue
		}

		rep, runErr := RunVerify(s, slug, opts)
		switch {
		case IsRefused(runErr):
			// Defensive — state slipped between the pre-check above and
			// the inside-RunVerify check. Surface as refused, not
			// failed: same semantics as the pre-apply skip.
			refusedReason := ""
			if runErr != nil {
				refusedReason = runErr.Error()
			}
			lifecycle := state
			if rep != nil {
				lifecycle = rep.LifecycleState
			}
			report.Features = append(report.Features, AggregateFeatureEntry{
				Slug:           slug,
				Status:         AggregateStatusRefused,
				Reason:         refusedReason,
				LifecycleState: lifecycle,
				Report:         rep,
			})
			report.Summary.Skipped++
		case rep == nil:
			// RunVerify bailed before producing any report (e.g. empty
			// slug guard — should not happen here since we filter
			// upstream, but keep the path honest).
			reason := "RunVerify returned no report"
			if runErr != nil {
				reason = runErr.Error()
			}
			report.Features = append(report.Features, AggregateFeatureEntry{
				Slug:           slug,
				Status:         AggregateStatusError,
				Reason:         reason,
				LifecycleState: state,
			})
			report.Summary.Error++
		default:
			// We have a report. Map the verdict — V0 abort returns
			// verdict=failed even when runErr is non-nil; treat that
			// as a failed feature (PRD §5: "every verify failure mode"
			// is a failure for aggregate exit-code purposes).
			status := AggregateStatusPassed
			switch rep.Verdict {
			case "passed":
				status = AggregateStatusPassed
				report.Summary.Passed++
			case "failed":
				status = AggregateStatusFailed
				report.Summary.Failed++
			case "refused":
				status = AggregateStatusRefused
				report.Summary.Skipped++
			default:
				status = AggregateStatusError
				report.Summary.Error++
			}
			entry := AggregateFeatureEntry{
				Slug:           slug,
				Status:         status,
				LifecycleState: rep.LifecycleState,
				Report:         rep,
			}
			if runErr != nil && status != AggregateStatusPassed {
				entry.Reason = runErr.Error()
			}
			report.Features = append(report.Features, entry)
		}
	}

	return report, nil
}

// WriteHumanAggregate renders `report` as a human-readable stream:
// per-feature verdict + per-check rows (reusing `WriteHumanReport`),
// plus a one-line summary footer. Pre-apply skips render as a single
// `slug — skipped: pre-apply (state=X)` row at their topo position.
func (r *AggregateReport) WriteHumanAggregate(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "verify --all (%d feature(s))\n", len(r.Features))
	for i, e := range r.Features {
		switch e.Status {
		case AggregateStatusSkipped:
			fmt.Fprintf(w, "[%d/%d] %s — skipped: %s\n", i+1, len(r.Features), e.Slug, e.Reason)
		case AggregateStatusError:
			fmt.Fprintf(w, "[%d/%d] %s — error: %s\n", i+1, len(r.Features), e.Slug, e.Reason)
		case AggregateStatusRefused:
			reason := e.Reason
			if reason == "" {
				reason = "refused"
			}
			fmt.Fprintf(w, "[%d/%d] %s — refused: %s\n", i+1, len(r.Features), e.Slug, reason)
		default:
			fmt.Fprintf(w, "[%d/%d] ", i+1, len(r.Features))
			if e.Report != nil {
				e.Report.WriteHumanReport(w)
			} else {
				fmt.Fprintf(w, "%s — %s\n", e.Slug, e.Status)
			}
		}
	}
	fmt.Fprintf(w, "Summary: %d passed, %d failed, %d skipped, %d error\n",
		r.Summary.Passed, r.Summary.Failed, r.Summary.Skipped, r.Summary.Error)
}
