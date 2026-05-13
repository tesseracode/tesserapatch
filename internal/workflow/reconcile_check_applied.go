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
// returned it for phases 1 / 1.5. Phase-1 reverse-apply success is itself
// upstream-merged evidence: it sets Outcome=ReconcileUpstreamed and
// Phase="phase-1-reverse-apply". Phase 1.5 is then run regardless and may
// UPGRADE the verdict to the more specific Phase="phase-1.5-patch-id-match"
// with the matched upstream SHA in UpstreamCommit / PatchIDMatch. If phase
// 1.5 does not match (or is skipped), a prior phase-1 hit still stands as
// the verdict. The caller distinguishes match vs no-match by Outcome, not
// by Phase.
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

	phase1Hit := false
	if reverseOK, _ := gitutil.ReverseApplyCheck(s.Root, patch); reverseOK {
		result.Outcome = store.ReconcileUpstreamed
		result.Phase = "phase-1-reverse-apply"
		result.Notes = append(result.Notes, "Patch is already present in upstream (reverse-apply succeeded)")
		phase1Hit = true
	}

	cfg, _ := s.LoadConfig()
	detectorOn := cfg.PatchIDDetectorEnabled || forceDetector
	if !detectorOn {
		if !phase1Hit {
			result.Outcome = store.ReconcileStillNeeded
			result.Phase = "phase-1.5-skipped-detector-disabled"
			result.Notes = append(result.Notes,
				"phase 1.5 not run: patch_id_detector_enabled=false and --check-applied-only invoked without override semantics")
		} else {
			result.Notes = append(result.Notes,
				"phase 1.5 not run: patch_id_detector_enabled=false (phase 1 already matched)")
		}
		return result, nil
	}

	if strings.TrimSpace(patch) == "" {
		if !phase1Hit {
			result.Outcome = store.ReconcileStillNeeded
			result.Phase = "phase-1.5-skipped-no-patch"
		}
		result.Notes = append(result.Notes, "phase 1.5 skipped: empty post-apply.patch")
		return result, nil
	}

	det := runPatchIDDetector(s, patch, upstreamCommit, cfg.PatchIDScanLimit)
	switch {
	case det.Match != nil:
		// Phase-1.5 match upgrades the verdict to the more specific
		// phase string and pins the matched upstream SHA.
		result.Outcome = store.ReconcileUpstreamed
		result.Phase = "phase-1.5-patch-id-match"
		result.UpstreamCommit = det.Match.MatchedUpstreamSHA
		result.PatchIDMatch = det.Match
		result.Notes = append(result.Notes,
			"Patch-id sweep matched upstream commit "+truncateCommit(det.Match.MatchedUpstreamSHA))
	case det.Skipped:
		if !phase1Hit {
			result.Outcome = store.ReconcileStillNeeded
			result.Phase = "phase-1.5-skipped"
		}
		result.Notes = append(result.Notes, "phase 1.5 skipped: "+det.SkipReason)
	default:
		if !phase1Hit {
			result.Outcome = store.ReconcileStillNeeded
			result.Phase = "phase-1.5-no-match"
		}
		result.Notes = append(result.Notes, "phase 1.5 found no upstream commit with a matching patch-id")
	}
	return result, nil
}
