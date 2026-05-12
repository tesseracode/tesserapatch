package workflow

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// patchIDDetectorResult is the in-process outcome of one phase-1.5
// patch-id sweep. `Match` is nil when the sweep did not produce a
// deterministic hit (no match, skipped, or tooling failure). Skip
// reasons are surfaced via `Skipped` + `SkipReason` so callers can log
// a one-line hint without flipping the lifecycle outcome (PRD §5.1:
// tooling failure must never become a no-match verdict).
type patchIDDetectorResult struct {
	Match      *store.PatchIDMatch
	Skipped    bool
	SkipReason string
}

// runPatchIDDetector executes the phase-1.5 deterministic patch-already-
// upstream sweep described in PRD-patch-already-upstream-detector §3.1
// and §5.1-5.3. It is *only* invoked when Config.PatchIDDetectorEnabled
// is true — the caller owns the flag-gate so the default-OFF path is a
// pure no-op.
//
// Contract:
//   - Reads upstream.lock.commit as the baseline (the lock-guard
//     preflight at Wave A2 owns validation; phase 1.5 itself does no
//     lock validation).
//   - Walks `git rev-list --no-merges <lockCommit>..<upstreamCommit>`,
//     capped by Config.PatchIDScanLimit (or DefaultPatchIDScanLimit).
//   - On match: returns a populated PatchIDMatch. UpstreamCommit on the
//     ReconcileResult is set by the caller to the earliest matching SHA
//     (PRD §5.3 multi-match policy).
//   - On no-match: returns (nil, nil) with Match=nil, Skipped=false.
//   - On any tooling failure or unreachable baseline: returns Skipped=true
//     with a SkipReason. The caller logs a debug line and falls through
//     to phase 2 (PRD §5.1 fail-soft).
func runPatchIDDetector(s *store.Store, patch, upstreamCommit string, scanLimit int) patchIDDetectorResult {
	if scanLimit <= 0 {
		scanLimit = store.DefaultPatchIDScanLimit
	}

	lock, err := store.LoadUpstreamLock(s)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return patchIDDetectorResult{Skipped: true, SkipReason: "upstream.lock not found — run `tpatch upstream pin` to enable phase 1.5"}
		}
		return patchIDDetectorResult{Skipped: true, SkipReason: fmt.Sprintf("upstream.lock unreadable: %v", err)}
	}
	if lock.Commit == "" {
		return patchIDDetectorResult{Skipped: true, SkipReason: "upstream.lock has no commit baseline — run `tpatch reconcile` once to populate"}
	}
	if upstreamCommit == "" {
		return patchIDDetectorResult{Skipped: true, SkipReason: "upstream tip commit is empty"}
	}

	ourPID, err := gitutil.PatchID(s.Root, patch)
	if err != nil {
		return patchIDDetectorResult{Skipped: true, SkipReason: fmt.Sprintf("git patch-id failed: %v", err)}
	}
	if ourPID == "" {
		return patchIDDetectorResult{Skipped: true, SkipReason: "feature patch produced empty patch-id (no diff content)"}
	}

	shas, err := gitutil.RevListInRange(s.Root, lock.Commit, upstreamCommit)
	if err != nil {
		return patchIDDetectorResult{Skipped: true, SkipReason: fmt.Sprintf("git rev-list %s..%s failed: %v", lock.Commit, upstreamCommit, err)}
	}
	if len(shas) == 0 {
		// Empty range — upstream tip is the baseline. No commits to scan.
		return patchIDDetectorResult{}
	}
	if len(shas) > scanLimit {
		return patchIDDetectorResult{
			Skipped:    true,
			SkipReason: fmt.Sprintf("rev-list count %d exceeds patch_id_scan_limit %d — skipping phase 1.5 (consider --upstream-ref <closer-ref>)", len(shas), scanLimit),
		}
	}

	rangeStr := fmt.Sprintf("%s..%s", lock.Commit, upstreamCommit)

	// `git rev-list` emits newest-first. Walk it that way and remember
	// every match; phase 1.5 picks the earliest (oldest) match as the
	// canonical UpstreamCommit per PRD §5.3.
	var matches []string
	for _, sha := range shas {
		theirPID, perr := gitutil.CommitPatchID(s.Root, sha)
		if perr != nil {
			// Skip individual commit failures but keep walking; a
			// single empty/binary commit shouldn't abort the sweep.
			continue
		}
		if theirPID == ourPID {
			matches = append(matches, sha)
		}
	}
	if len(matches) == 0 {
		return patchIDDetectorResult{}
	}

	// Earliest match = last entry (rev-list is newest-first).
	earliest := matches[len(matches)-1]
	additional := make([]string, 0, len(matches)-1)
	for _, sha := range matches {
		if sha != earliest {
			additional = append(additional, sha)
		}
	}
	return patchIDDetectorResult{
		Match: &store.PatchIDMatch{
			OurPatchID:         ourPID,
			MatchedUpstreamSHA: earliest,
			AdditionalMatches:  additional,
			ScannedRange:       rangeStr,
			ScannedCount:       len(shas),
		},
	}
}
