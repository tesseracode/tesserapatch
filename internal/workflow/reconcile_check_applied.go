package workflow

import (
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// CheckAppliedOnly runs phases 1 and 1.5 of the reconcile pipeline ONLY for
// the given slug, against the given upstream ref. It writes NO artifacts and
// does NOT mutate FeatureStatus / status.json / reconcile-session.json. This
// is the read-only sweep that drives `tpatch reconcile --check-applied-only`
// (PRD-patch-already-upstream-detector §3.2).
//
// Detector-off override: when forceDetector is true, phase 1.5 runs even if
// Config.PatchIDDetectorEnabled is false. This is the explicit per-invocation
// opt-in for `--check-applied-only`; the persisted default stays OFF.
//
// Returns the populated ReconcileResult exactly as RunReconcile would have
// returned it for phases 1 / 1.5. Under --check-applied-only the normal
// reconcile preflight (lock-guard + clean-tree-at-upstream baseline) is
// skipped by design, so phase-1 reverse-apply reads the LIVE working tree
// rather than a verified upstream state — which means phase-1 success is
// NOT upstream-scoped evidence here (the user may simply be sitting on
// their feature branch with the patch already applied). Phase 1.5
// patch-id sweep is therefore the SOLE authoritative upstream-merged
// signal under this command: it owns Outcome and Phase. Phase-1 success
// is recorded only as a diagnostic note for operator visibility.
//
// In the NORMAL reconcile pipeline (internal/workflow/reconcile.go),
// phase-1 success is legitimately upstream-merged evidence because the
// preflight has already guaranteed tree state — that behaviour is
// unchanged.
func CheckAppliedOnly(s *store.Store, slug, upstreamRef string, forceDetector bool) (*ReconcileResult, error) {
	upstreamCommit, err := gitutil.ResolveRef(s.Root, upstreamRef)
	if err != nil {
		return nil, err
	}

	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return nil, err
	}

	patch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		// Reuse the same error wording RunReconcile uses (without
		// the incremental.patch fallback — the detector requires
		// the canonical patch anyway, per reconcile.go ~206).
		return nil, err
	}

	result := &ReconcileResult{
		Slug:           slug,
		Title:          status.Title,
		UpstreamRef:    upstreamRef,
		UpstreamCommit: upstreamCommit,
	}

	// Phase 1: live-tree reverse-apply check. Under --check-applied-only
	// the preflight is skipped, so reverse-apply against the working
	// tree is NOT upstream-scoped evidence — it only tells us the
	// patched content is already in the tree (which may be the user's
	// feature branch). Phase 1.5 patch-id sweep is the authoritative
	// upstream-merged signal here. We still record a phase-1 success
	// as a diagnostic note for operator visibility, but it does NOT
	// contribute to Outcome or Phase.
	if reverseOK, _ := gitutil.ReverseApplyCheck(s.Root, patch); reverseOK {
		result.Notes = append(result.Notes,
			"phase 1 reverse-apply: working tree already contains the patched content (not an upstream-merged signal under --check-applied-only; see phase 1.5)")
	}

	cfg, _ := s.LoadConfig()
	detectorOn := cfg.PatchIDDetectorEnabled || forceDetector
	if !detectorOn {
		result.Outcome = store.ReconcileStillNeeded
		result.Phase = "phase-1.5-skipped-detector-disabled"
		result.Notes = append(result.Notes,
			"phase 1.5 not run: patch_id_detector_enabled=false and --check-applied-only invoked without override semantics")
		return result, nil
	}

	if strings.TrimSpace(patch) == "" {
		result.Outcome = store.ReconcileStillNeeded
		result.Phase = "phase-1.5-skipped-no-patch"
		result.Notes = append(result.Notes, "phase 1.5 skipped: empty post-apply.patch")
		return result, nil
	}

	det := runPatchIDDetector(s, patch, upstreamCommit, cfg.PatchIDScanLimit)
	switch {
	case det.Match != nil:
		result.Outcome = store.ReconcileUpstreamed
		result.Phase = "phase-1.5-patch-id-match"
		result.UpstreamCommit = det.Match.MatchedUpstreamSHA
		result.PatchIDMatch = det.Match
		result.Notes = append(result.Notes,
			"Patch-id sweep matched upstream commit "+truncateCommit(det.Match.MatchedUpstreamSHA))
	case det.Skipped:
		result.Outcome = store.ReconcileStillNeeded
		result.Phase = "phase-1.5-skipped"
		result.Notes = append(result.Notes, "phase 1.5 skipped: "+det.SkipReason)
	default:
		result.Outcome = store.ReconcileStillNeeded
		result.Phase = "phase-1.5-no-match"
		result.Notes = append(result.Notes, "phase 1.5 found no upstream commit with a matching patch-id")
	}
	return result, nil
}
