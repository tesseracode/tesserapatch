package workflow

// Derived-artifact refresh after a successful `tpatch reconcile --accept`.
//
// The accept flow produces a new working-tree state:
//
//	[new upstream commit] + [feature's non-conflict hunks (applied cleanly)]
//	                      + [phase-3.5 resolved files (copied from shadow)]
//
// That new state is what future reconciles should start from, so the
// feature's derived artifacts must be refreshed to reflect it:
//
//   - artifacts/post-apply.patch — regenerated as `git diff <upstreamCommit>`
//     restricted to the files the ORIGINAL post-apply.patch touched. Other
//     working-tree dirt (untracked files, unrelated edits) is excluded.
//   - patches/NNN-reconcile.patch — numbered snapshot of the new
//     post-apply.patch, serves as the audit trail of what changed.
//
// What this function does NOT refresh (deferred; callers should emit a
// note pointing users at `tpatch record` or an ADR-scoped followup):
//
//   - artifacts/apply-recipe.json — regenerating the op-level recipe
//     from a raw diff is lossy. Left stale for now; `tpatch record`
//     remains the authoritative path.
//   - artifacts/incremental.patch — reconcile.go already consumes
//     post-apply.patch as the fallback, so staleness here is cosmetic.

import (
	"errors"
	"fmt"
	"os"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// RefreshAfterAccept regenerates the feature's derived artifacts after
// the working tree has been updated via `reconcile --accept`.
// originalPatch is the pre-accept contents of post-apply.patch, used
// to determine which file paths to restrict the new diff to.
// upstreamCommit is the ref the regenerated diff is taken against.
//
// On error the caller should treat derived artifacts as potentially
// stale but should NOT roll back the working-tree changes — those are
// already reflected in the new state. The safe recovery path is to
// re-run `tpatch record` which uses the same plumbing.
func RefreshAfterAccept(s *store.Store, slug, upstreamCommit, originalPatch string) (retErr error) {
	// GH #7 rev-2/rev-3: the ORIGINAL patch may carry stale
	// nested-worktree gitlink entries recorded by a pre-fix tpatch, and
	// Git C-quotes any of those whose path contains a space plus a
	// control byte, a quote, a backslash or a newline. Strict parsing is
	// mandatory here: the fail-soft scanner silently dropped quoted
	// headers, and an empty touched-path set means "diff everything" to
	// Git — which is how a worktree-only patch broadened the refresh to
	// unrelated working-tree dirt.
	//
	// Order is load-bearing: strict parse, then discovery, then
	// filtering, and only then the first write. A failure in any of the
	// three leaves the feature directory byte-identical.
	originalFiles, err := gitutil.FilesInPatchStrict(originalPatch)
	if err != nil {
		return fmt.Errorf("refresh: cannot determine which paths the current post-apply.patch touches: %w", err)
	}
	files, err := gitutil.FilterPathsExcludingNestedWorktrees(s.Root, originalFiles)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	var newPatch string
	if len(originalFiles) > 0 && len(files) == 0 {
		// Every path the original patch touched was a nested worktree.
		// Regenerate nothing rather than broadening to a full-tree
		// diff, which is what an empty scope would mean to Git.
		newPatch = ""
	} else {
		newPatch, err = gitutil.DiffFromCommitForPaths(s.Root, upstreamCommit, files)
		if err != nil {
			return fmt.Errorf("refresh: regenerate post-apply.patch: %w", err)
		}
	}

	// P3: the immutable observation is taken here, against the accepted
	// upstream commit the refresh diffs from, and BEFORE the canonical
	// patch write below (ADR-036 D2).
	//
	// It binds `newPatch` — the bytes about to be written — not the
	// pre-refresh `originalPatch`. The event P3 owns is the patch WRITE,
	// so an observation of the bytes being replaced would describe a
	// state the producer is in the act of ending. `originalPatch` keeps
	// its one remaining role above: it is the pre-write context the
	// refresh scope is derived from.
	//
	// `PatchPresent` is true even when `newPatch` is empty. The file is
	// about to exist, which is a different statement from "no canonical
	// patch was readable"; a patch that is present and semantically empty
	// is recorded as such by the observation's own record-level reason.
	//
	// The reference is the accepted upstream commit, and postimages come
	// from the working tree the accept just produced — which is exactly
	// the state `newPatch` was diffed out of.
	obs := patchobs.ObserveAndEmit(patchobs.Input{
		Producer:     patchobs.ProducerReconcileAccept,
		RepoRoot:     s.Root,
		Slug:         slug,
		Patch:        newPatch,
		PatchPresent: true,
		Capture: patchobs.CaptureDescriptor{
			Mode:      patchobs.CaptureModeReconcile,
			Pathspecs: files,
			ClaimIDs:  []string{},
		},
		PreimageRef: upstreamCommit,
	})
	// PI-3: the strict refusal is raised HERE, before the first bound
	// write, so a patch whose effects nobody can derive never reaches
	// post-apply.patch, the numbered snapshot, or the generation append
	// that re-parses the same bytes downstream.
	if perr := obs.PreflightError(); perr != nil {
		return fmt.Errorf("refresh: regenerated post-apply.patch is unreadable, refusing to bind it: %w", perr)
	}

	// post-apply.patch is the source of truth for future reconciles.
	publication := ObserveCoveragePublication(s, obs)
	if err := s.WriteArtifactAtomic(slug, "post-apply.patch", newPatch); err != nil {
		return fmt.Errorf("refresh: write post-apply.patch: %w", err)
	}
	publication.Events.PatchRewritten = true
	defer func() {
		coverage, coverageErr := PublishCoverage(s, publication)
		retErr = ReportCoverageStatus(nil, coverage, errors.Join(retErr, coverageErr))
	}()

	// Audit snapshot into patches/. The label "reconcile" matches the
	// ADR-010 design doc so future tooling can filter by it.
	patchName, err := s.WritePatch(slug, "reconcile", newPatch)
	if err != nil {
		return fmt.Errorf("refresh: write numbered reconcile patch: %w", err)
	}

	if newPatch != originalPatch {
		baseCommit := ""
		if st, serr := s.LoadFeatureStatus(slug); serr == nil {
			baseCommit = st.Apply.BaseCommit
		}
		auditPatch := ""
		if patchName != "" {
			auditPatch = "patches/" + patchName
		}
		if _, err := AppendPatchGenerationForFeature(s, slug, PatchGenerationInput{
			Kind:       "reconcile",
			Patch:      newPatch,
			AuditPatch: auditPatch,
			BaseCommit: baseCommit,
			Upper: store.GenerationUpper{
				Kind:   "reconcile-result",
				Ref:    upstreamCommit,
				Commit: upstreamCommit,
			},
			Capture: store.GenerationCapture{
				Mode:      "reconcile",
				Pathspecs: files,
				ClaimIDs:  []string{},
			},
			AllowMalformedManifest: true,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: patch-generations.json append after reconcile failed: %v\n", err)
		}
	}

	return nil
}
