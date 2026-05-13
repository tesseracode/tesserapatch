package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// reconcileAutoDropMerged is the post-RunReconcile hook that fires when
// the operator passed `tpatch reconcile --auto-drop-merged`. For every
// result whose verdict is the phase-1.5 patch-id match, the matching
// feature is removed from the DAG (subject to the same rules as
// `tpatch feature remove` per ADR-011 cascade) and a removal commit is
// created whose message preserves the `Tpatch-Slug` trailer always and
// the `Tpatch-CVE` trailer when the original slug encodes a CVE
// identifier.
//
// PRD-patch-already-upstream-detector §3.3.
//
// Behaviour matrix:
//   - phase-1.5 match, no dependents (or DAG flag off): remove the
//     feature directory, refresh FEATURES.md, stage the deletion(s) +
//     FEATURES.md, commit with the trailer block.
//   - phase-1.5 match, has hard/soft dependents and DAG flag on: REFUSE
//     the drop for that slug (mirrors `tpatch feature remove` default
//     behaviour) and surface a hint pointing the operator at
//     `tpatch remove --cascade <slug>`. Other slugs in the batch are
//     processed independently.
//   - non-phase-1.5 outcome: no-op for that slug.
//
// The function returns nil if no auto-drop work fired (the common case
// when the detector did not match) or an aggregated error describing
// any per-slug failure. A failure on one slug does not prevent the
// others from being processed.
func reconcileAutoDropMerged(cmd *cobra.Command, s *store.Store, results []resultDropTarget) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	var errs []string
	for _, r := range results {
		if r.Phase != "phase-1.5-patch-id-match" {
			continue
		}
		if r.Outcome != store.ReconcileUpstreamed {
			continue
		}

		// Cascade gate: refuse silently-destructive removal when the
		// DAG flag is on and downstream dependents exist. This mirrors
		// the default `tpatch feature remove` contract (PRD §3.7,
		// ADR-011 D7).
		if err := checkRemoveDependents(s, r.Slug); err != nil {
			fmt.Fprintf(errOut,
				"auto-drop-merged: refusing to drop %s — %v\n  hint: run `tpatch remove --cascade %s` after a manual review\n",
				r.Slug, err, r.Slug)
			errs = append(errs, fmt.Sprintf("%s: %v", r.Slug, err))
			continue
		}

		trailers := buildAutoDropTrailers(r.Slug)
		subject := fmt.Sprintf("tpatch: drop %s (upstreamed in %s)", r.Slug, abbrevSHA(r.UpstreamCommit))

		if err := s.RemoveFeature(r.Slug); err != nil {
			fmt.Fprintf(errOut, "auto-drop-merged: remove %s: %v\n", r.Slug, err)
			errs = append(errs, fmt.Sprintf("%s: remove: %v", r.Slug, err))
			continue
		}

		// Stage only the dropped feature's tree (its deletion) plus
		// the rewritten .tpatch/FEATURES.md. We intentionally do NOT
		// sweep all of .tpatch/features: in a multi-slug batch the
		// normal reconcile pipeline writes incremental.patch /
		// reconcile-session.json / status.json / reconcile.md into
		// OTHER slugs' artifact dirs before this auto-drop fires, and
		// a broad `git add -A .tpatch/features` would absorb those
		// unrelated artifacts into the removal commit.
		stageArgs := []string{"add", "-A",
			filepath.Join(".tpatch", "features", r.Slug),
			filepath.Join(".tpatch", "FEATURES.md"),
		}
		if stageOut, stageErr := runGitCapture(s.Root, stageArgs...); stageErr != nil {
			fmt.Fprintf(errOut, "auto-drop-merged: stage %s: %v\n%s", r.Slug, stageErr, stageOut)
			errs = append(errs, fmt.Sprintf("%s: stage: %v", r.Slug, stageErr))
			continue
		}

		commitArgs := []string{
			"-c", "commit.gpgsign=false",
			"commit",
			"-m", subject,
			"-m", trailers,
		}
		if commitOut, commitErr := runGitCapture(s.Root, commitArgs...); commitErr != nil {
			fmt.Fprintf(errOut, "auto-drop-merged: commit %s: %v\n%s", r.Slug, commitErr, commitOut)
			errs = append(errs, fmt.Sprintf("%s: commit: %v", r.Slug, commitErr))
			continue
		}

		newHead, _ := headCommitForAutoDrop(s.Root)
		fmt.Fprintf(out, "auto-drop-merged: removed %s, committed %s (upstreamed in %s)\n",
			r.Slug, abbrevSHA(newHead), abbrevSHA(r.UpstreamCommit))
	}

	if len(errs) > 0 {
		return fmt.Errorf("auto-drop-merged: %d error(s): %s", len(errs), strings.Join(errs, "; "))
	}
	return nil
}

// resultDropTarget is the minimal projection of workflow.ReconcileResult
// the auto-drop hook needs. Defining it here keeps the cli package's
// surface clean and avoids importing workflow into a helper file just
// for the field aliases.
type resultDropTarget struct {
	Slug           string
	Phase          string
	Outcome        store.ReconcileOutcome
	UpstreamCommit string
}

// cveSlugPattern matches a CVE identifier embedded in a feature slug or
// title. The PRD example slug is `cve-2026-12345-validate-input`; we
// accept the canonical `CVE-YYYY-N+` form (case-insensitive). Used only
// to derive an optional `Tpatch-CVE` trailer for the auto-drop
// removal-commit message — absence of a match means the trailer is
// omitted, never errored.
var cveSlugPattern = regexp.MustCompile(`(?i)\bcve[- ]?(\d{4})[- ](\d{4,})\b`)

// buildAutoDropTrailers returns the trailer block for the auto-drop
// removal commit. `Tpatch-Slug` is always emitted; `Tpatch-CVE` is
// emitted only when the slug contains a CVE identifier (so the audit
// trail captured in the original feature manifest survives the
// removal). The repo-policy `Co-authored-by` trailer is appended last
// to keep the commit compliant with CLAUDE.md rule 8.
func buildAutoDropTrailers(slug string) string {
	var b strings.Builder
	b.WriteString("Tpatch-Slug: ")
	b.WriteString(slug)
	b.WriteString("\n")
	if m := cveSlugPattern.FindStringSubmatch(slug); m != nil {
		fmt.Fprintf(&b, "Tpatch-CVE: CVE-%s-%s\n", m[1], m[2])
	}
	b.WriteString(coAuthorTrailer)
	b.WriteString("\n")
	return b.String()
}

// headCommitForAutoDrop is a thin wrapper around `git rev-parse HEAD`.
// We intentionally do not import gitutil here — the only consumer is
// the auto-drop progress message, and pulling in gitutil would create a
// noisier dependency footprint for a one-line lookup.
func headCommitForAutoDrop(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
