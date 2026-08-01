package workflow

import (
	"fmt"
	"io"
	"os"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// D10 migration diagnostic
// (PRD-multi-slug-reconcile-canonical-safety §4.10 / ADR-030 D2 §Sunset).
//
// When a default (non-legacy) multi-slug reconcile fails phase 1 on
// slug N and any earlier slug in the invocation touched a subset of
// N's `touched_paths`, the run emits
//
//	hint: prior features may have been recorded cumulatively; retry with --cumulative-legacy (see ADR-030)
//
// The overlap-on-touched-paths check is the exact shape legacy
// cumulative recording produces: an earlier slug's canonical patch
// touches paths that are also (a strict subset of the) paths in a
// later slug's canonical patch. In that shape, reverse-applying the
// later slug's canonical patch will fail because the earlier slug's
// contributions are missing from upstream but expected in the diff.
//
// The hint is advisory:
//   - The phase 1 failure surfaces normally with its own diagnostics.
//   - The exit code is unchanged relative to the phase 1 failure.
//   - The run does not silently retry.
//
// AC-15 of the PRD. False positives cost one line of stderr; false
// negatives leave the operator with the pre-remediation surprise.
//
// migrationDiagHint is factored into a var so tests can substitute a
// buffer sink. Callers pass a nil writer to fall through to os.Stderr.

var migrationDiagHintWriter io.Writer = os.Stderr

// maybeEmitMigrationHint checks the overlap condition and writes the
// D10 hint to the diagnostic sink when it fires. The caller is
// expected to invoke this only when:
//   - opts.CumulativeLegacy is false (default path)
//   - result.Phase indicates phase 1 (reverse-apply) did not succeed,
//     i.e. Phase is neither "" nor "phase-1-reverse-apply" nor
//     "phase-1.5-patch-id-match"
//
// The check consults each earlier slug's `patch-generations.json`
// (via store.LoadPatchGenerations) and asks whether the earlier
// slug's `touched_paths` is a subset of the current slug's
// `touched_paths`. A missing manifest for either slug is treated as
// "no overlap" (fail-soft: never emit the hint on incomplete data).
//
// Returns true iff the hint was emitted, so tests can assert AC-15
// positive and negative.
func maybeEmitMigrationHint(s *store.Store, priorSlugs []string, currentSlug string) bool {
	current, err := latestTouchedPaths(s, currentSlug)
	if err != nil || len(current) == 0 {
		return false
	}
	currSet := make(map[string]struct{}, len(current))
	for _, p := range current {
		currSet[p] = struct{}{}
	}
	for _, prior := range priorSlugs {
		priorPaths, perr := latestTouchedPaths(s, prior)
		if perr != nil || len(priorPaths) == 0 {
			continue
		}
		if isSubset(priorPaths, currSet) {
			fmt.Fprintln(migrationDiagHintWriter, "hint: prior features may have been recorded cumulatively; retry with --cumulative-legacy (see ADR-030)")
			return true
		}
	}
	return false
}

// latestTouchedPaths returns the `touched_paths` field of the latest
// generation in patch-generations.json for `slug`. Empty slice
// (nil, nil) when the manifest is absent or has no generations.
func latestTouchedPaths(s *store.Store, slug string) ([]string, error) {
	m, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		return nil, err
	}
	latest, ok := store.LatestPatchGeneration(m)
	if !ok {
		return nil, nil
	}
	return latest.TouchedPaths, nil
}

// isSubset returns true iff every element of `sub` is present in
// `super`. Empty `sub` is not treated as a subset for the D10
// check — an empty prior-touched-paths set never triggers the hint
// (fail-soft: absence of data is not evidence of cumulative
// recording).
func isSubset(sub []string, super map[string]struct{}) bool {
	if len(sub) == 0 {
		return false
	}
	for _, p := range sub {
		if _, ok := super[p]; !ok {
			return false
		}
	}
	return true
}

// phaseIndicatesReverseApplyFailure reports whether the reconcile
// result phase indicates phase 1 (reverse-apply) did not succeed.
// The success indicators are:
//   - "phase-1-reverse-apply"    → phase 1 succeeded (reconcile is upstreamed)
//   - "phase-1.5-patch-id-match" → phase 1 didn't succeed but phase 1.5 did
//
// Everything else (phase 2, 3, 4, error, empty) means phase 1
// failed and the run kept going or blocked. The D10 hint is
// meaningful precisely in those tail phases.
func phaseIndicatesReverseApplyFailure(phase string) bool {
	switch phase {
	case "", "phase-1-reverse-apply", "phase-1.5-patch-id-match":
		return false
	}
	return true
}
