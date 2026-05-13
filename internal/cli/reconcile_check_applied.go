package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// runReconcileCheckAppliedOnly implements `tpatch reconcile
// --check-applied-only <slug>` (PRD-patch-already-upstream-detector
// §3.2). The command is read-only: it runs phase 1 (reverse-apply)
// and phase 1.5 (patch-id sweep) for the named slug and prints a
// reconcile-summary line. Phases 2/3/4 are unconditionally skipped,
// no artifacts are written, and `status.json` / `reconcile-session.json`
// are left untouched.
//
// Detector-off override: phase 1.5 runs even when
// `Config.PatchIDDetectorEnabled` is false. The flag is an explicit
// per-invocation user opt-in; the persisted default stays OFF (frozen
// per ADR-022 deferral track).
//
// Exit codes:
//   - 0 — phase-1.5 patch-id sweep matched
//     (Phase == "phase-1.5-patch-id-match")
//   - 2 — no phase-1.5 match (any other terminal phase, including
//     phase-1 reverse-apply hits or detector skips)
//
// Other failures (unreadable patch, missing slug, git plumbing errors)
// surface as ordinary CLI errors and collapse to exit 1 via the
// existing root command path.
func runReconcileCheckAppliedOnly(cmd *cobra.Command, s *store.Store, slugs []string, upstreamRef string) error {
	if len(slugs) != 1 {
		return fmt.Errorf("reconcile --check-applied-only requires exactly one <slug> argument")
	}
	slug := slugs[0]

	// Force the detector ON for this single invocation regardless of
	// Config.PatchIDDetectorEnabled. The flag is the user's explicit
	// opt-in; we do not mutate persisted config.
	result, err := workflow.CheckAppliedOnly(s, slug, upstreamRef, true /* forceDetector */)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Checked %s against %s (read-only)\n", slug, upstreamRef)
	fmt.Fprintf(out, "  - %s [%s] (%s) %s\n", result.Slug, result.Outcome, result.Phase, result.Title)
	for _, note := range result.Notes {
		fmt.Fprintf(out, "    %s\n", note)
	}
	if result.PatchIDMatch != nil {
		fmt.Fprintf(out, "    matched-upstream-sha: %s\n", result.PatchIDMatch.MatchedUpstreamSHA)
		fmt.Fprintf(out, "    scanned-range:        %s\n", result.PatchIDMatch.ScannedRange)
		fmt.Fprintf(out, "    scanned-count:        %d\n", result.PatchIDMatch.ScannedCount)
	}

	if result.Phase == "phase-1.5-patch-id-match" {
		return nil
	}
	return &ExitCodeError{Code: 2, Message: "no phase-1.5 patch-id match"}
}
