# ADR-049 — Phase-2 authority and lifecycle integration

**Status**: Proposed — S6 implementation decisions, independent review pending
**Date**: 2026-10-31
**Related**: ADR-037, ADR-043, ADR-044, ADR-048

## Context

Accepted S5 owns actual candidate preparation, execution and publication.
S6 must connect it to the existing pipeline without holding locks across a
provider call, losing a pending pointer, or turning persisted evidence into
confirmation authority. The accepted PRD S6 checklist and §6.2/§6.11/§6.17
remain the contract; no new wire schema or later-stage action is introduced.

## Decisions

1. **Separate readonly observation from publication authority.** Phase2 uses
   fresh S2 eligibility and S3 typed immutable-root derivation. It retains only
   bounded binding/classification metadata through phase3, not a locked S5
   preparation or execution payload. The locked S5 factory is used only at
   phase4 finalization, after a real eligible textual-conflict arm.
2. **Keep one feature hold through candidate-ready persistence.** Extract a
   caller-held finalization core behind the existing S5 acquiring wrapper,
   following ADR-044's borrow discipline. S6 acquires once, prepares and
   revalidates against the readonly phase2 snapshot, executes through the S5
   core, stages evidence and writes the merged summary before releasing.
   Global holds remain limited to worktree creation/pruning. Releasing between
   candidate publication and its summary write could resurrect a pointer
   concurrently cleared by rejection.
3. **Keep the proof's route parameter-only.** The GH13 evaluator invokes
   saveReconcileArtifacts itself; it never returns a phase2AuthorityProof.
   Save and the confirmation gate are the only functions accepting that type.
   Bind the proof to its Store/root and result invocation as well as the
   prescribed independently recomputed facts. Gate checks use the already
   appended qualifying entry and fresh readonly facts, never reconstructed
   authority from an evidence label. An operation-count observation, if
   needed from the opaque S3 plan, is readonly metadata rather than authority.
4. **Distinguish initialization, mutation and observation of staging.** One
   evaluator multi-assignment mints/stages the all-present entry and proof.
   The candidate-ready branch has the one append mutation, in that same
   evaluator, and mints no proof. The guard treats that append assignment as
   the mutation, not a second initialization. Persist copies the staged list
   at its sole observation site and validates the three legal configurations
   before substitution; invalid staging takes the unchanged legacy builder.
   The actual supporting-detector calls and their firing conditions stay put.
5. **No false retirement on new-authority publication failure.** If staged
   authority or its confirmation evidence cannot be appended, the new path
   cannot remain upstreamed merely because a legacy warning-only write
   returned early. Any denial bookkeeping is local to save and grants no
   authority. Scope this safeguard to attempted new authority/staging so
   gate-off legacy behavior stays unchanged.
6. **Reuse the existing summary merge.** Build the same result-derived summary
   through store.CarryForwardOperationCandidate before replacement. Ordinary
   persistence retains its State/Notes rules; candidate-ready persistence
   changes neither. Explicit locked candidate transitions remain the only
   pointer writers.
7. **Checked new-path persistence and late recovery.** The shipped artifact
   writer returns its evidence/session/markdown/log errors; legacy callers
   retain their existing ignore-and-warn behavior, while new authority and
   candidate-ready callers propagate a publication error. A valid pending
   candidate is retained when later presentation/status persistence fails.
   updateFeatureState likewise returns load/save errors: the new all-present
   caller checks them, while existing callers retain their ignored-return
   behavior. A failed atomic status replacement cannot report successful
   retirement.
   Recovery discovered by finalization remains the terminal recovery sentinel,
   never an ordinary phase2 refusal.
8. **Retain observations when stronger answers win.** A discarded readonly
   candidate observation is appended to both evidence sinks before the winning
   arm is saved. It is phase-local still_needed with medium confidence, not
   the high-confidence claim reserved for completed candidate validation.
   It creates no worktree or confirmation proof and does not use staged
   substitution.
9. **Latest-invocation review.** Human review selects the evidence IDs recorded
   in the latest reconcile-session, resolves them against the validated
   evidence log and displays them in phase order. The returned outcome is
   labelled separately; --all explicitly labels historical append order.
   The existing revision JSON envelope is unchanged.

## Implementation constraints

Preflight still acquires before recovery/metadata inspection and releases before
phase1. Compute any needed live binding snapshot under that hold, after the
recovery check; unusable input cannot become a fabricated canonical hash or
derivation authority. Candidate-ready persistence calls the shipped artifact/
evidence writer with nil proof and bypasses only updateFeatureState.

No S7 terminal flags, S8 acceptance/recovery transactions, S9 assets/release,
new persisted evidence fields or weakening of the legacy confirming inputs.
Source and runtime negative controls must cover proof escape, malformed
staging, swapped result identity, stronger-arm precedence and pointer lifetime.
