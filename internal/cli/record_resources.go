// `tpatch record --resources` two-domain semantics
// (PRD-feature-resource-claims-and-capture-adapters §11).
//
// The Git-side capture and the resource-domain publication are two
// separate atomic domains. Staging is ephemeral in-memory metadata
// only; publishing is the same §7.3 steps 3-4 a standalone `capture`
// would run, using the identical content-addressed batch_id.
//
// Ordering, exactly:
//
//  1. zero-resource preflight — refuses before touching Git and before
//     lock acquisition;
//  2. stage — acquire the per-slug flock, run the lock-gated orphan
//     sweeps, compute every result and the candidate batch_id in
//     memory, and stop before any tracked write;
//  3. Git-side capture — record's existing, unmodified capture-mode
//     dispatch, completely unaffected by step 2's outcome;
//  4. publish, gated on Git success — a tracked resource write is never
//     attempted before Git succeeds.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/rescap"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// recordResourcesRetryHint is the exact remediation the partial-domain
// refusal prints. `capture` re-stages and republishes and is safe to
// re-run because batch_id is content-addressed.
const recordResourcesRetryHint = "Retry with `tpatch feature resource capture %s` — this re-stages and republishes and is safe to re-run."

// runRecordWithResources wraps record's existing RunE with the
// two-domain ordering. When --resources is absent it is a pure
// pass-through: the Git-side behaviour is byte-identical.
func runRecordWithResources(cmd *cobra.Command, args []string, gitSideRecord func(*cobra.Command, []string) error) error {
	withResources, _ := cmd.Flags().GetBool("resources")
	if !withResources {
		return gitSideRecord(cmd, args)
	}
	slug := args[0]

	ctx, err := openResourceContext(cmd, slug)
	if err != nil {
		return resourceExit(err)
	}
	// Step 1: zero declared resources refuses immediately, before
	// touching Git and before lock acquisition.
	if len(ctx.Manifest.Resources) == 0 {
		return resourceExit(rescap.Internal(rescap.ReasonNoResourcesDeclared,
			"record --resources: %s has no declared resources", slug))
	}

	if err := rescap.EnsureLocalContract(ctx.Store.Root, slug); err != nil {
		return resourceExit(err)
	}
	lock, err := rescap.AcquireLock(rescap.ScratchRoot(ctx.Store.Root, slug), ctx.Store.Root)
	if err != nil {
		return resourceExit(err)
	}
	// The flock is held across staging, the Git-side capture, and the
	// publish: this is one invocation of record --resources, not two.
	defer func() { _ = lock.Release() }()

	engine := rescap.NewEngine(ctx.Store, slug)
	engine.Diagnostics = append(engine.Diagnostics, rescap.SweepLocalOrphans(ctx.Store.Root, slug, "")...)
	engine.Diagnostics = append(engine.Diagnostics, ctx.Store.SweepTrackedTempArtifacts(slug)...)

	scratch, scratchErr := rescap.EphemeralScratch(ctx.Store.Root, slug)
	if scratchErr != nil {
		return resourceExit(scratchErr)
	}
	defer engine.RemoveScratch(scratch)

	// Step 2: stage into memory. A staging failure is NOT propagated
	// yet — the Git-side capture runs regardless, and the partial-domain
	// outcome is only reported once Git's own result is known.
	staged, stageErr := engine.Stage(ctx.Manifest.Resources, scratch)

	// Step 3: the Git-side capture, unmodified.
	if gitErr := gitSideRecord(cmd, args); gitErr != nil {
		// Git failed: the in-memory candidate is simply discarded,
		// never written anywhere, regardless of its own outcome.
		if stageErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"record --resources: the resource domain also failed to stage (%v); its candidate batch was discarded\n", stageErr)
		}
		return gitErr
	}

	// Step 4: publish, gated on Git success.
	if stageErr != nil {
		return resourceExit(partialDomain(slug, stageErr))
	}
	outcome, pubErr := engine.Publish(staged)
	if pubErr != nil {
		engine.WriteLocalDiagnostics(scratch)
		return resourceExit(partialDomain(slug, pubErr))
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	return reportCapture(cmd.OutOrStdout(), asJSON, slug, staged.Batch, outcome, false)
}

// partialDomain builds the resource-domain-incomplete refusal. It is
// exit 1 regardless of the underlying reason's own code, because the
// canonical patch *did* record successfully — only the sidecar domain
// is incomplete.
func partialDomain(slug string, cause error) error {
	reason := cause.Error()
	if r := rescap.AsRefusal(cause); r != nil {
		reason = r.Error()
	}
	if p, ok := cause.(*store.PublicationError); ok {
		reason = p.Error()
	}
	return &rescap.Refusal{
		Reason: rescap.ReasonResourceDomainIncomplete,
		Code:   rescap.ExitInternal,
		Detail: fmt.Sprintf(
			"canonical patch recorded successfully; resource capture did not complete: %s. "+recordResourcesRetryHint,
			reason, slug),
		Cause: cause,
	}
}
