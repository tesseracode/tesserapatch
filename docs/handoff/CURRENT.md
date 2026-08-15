# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

The adjacent-hunk conflict, semantic replay, feature absorption and verified
reorder investigation **rev-3 errata** closes the rev-2 review notes and awaits
re-review. The work is research only: no command, state, schema or
implementation is authorized.

## Active Task

- **Task ID**: `adjacent-conflict-and-absorption-research`
- **Description**: Reproduce a Git conflict caused by a feature addition beside
  intentional upstream deletions; compare merge/rebase/current tpatch behavior;
  map absorbed-feature and reparent/reorder requests to shipped surfaces and
  existing research; formalize gaps.
- **Status**: Rev-3 errata complete — awaiting review
- **Assigned**: 2026-08-15
- **WAVE_BASE**: `33826d8` (origin/main immediately before research commit
  `b19ea6a`)
- **Issues**:
  - [GH #13](https://github.com/tesseracode/tesserapatch/issues/13) —
    ADR-010 phase-2 fidelity and safe operation candidate generation
  - [GH #12](https://github.com/tesseracode/tesserapatch/issues/12) —
    absorbed-feature compaction tiers
  - [GH #14](https://github.com/tesseracode/tesserapatch/issues/14) —
    verified reparent/reorder through commutation
  - [GH #15](https://github.com/tesseracode/tesserapatch/issues/15) —
    anchored/preimage-complete recipe generation prerequisite
- **Prior next task**: `implement-prepare-check` remains Not Started and returns
  to the front of the queue after this research closes.

## Rev-1 Review and Rev-2 Adjudication

**Internal verdict**: APPROVED
**External verdict**: NEEDS REVISION
**Reviewed tip**: `9a4ad48`

1. Corrected the delete-first expected tree so surviving `--old-b` remains.
2. Anchored phase-2 fidelity to authoritative SPEC §7 as well as ADR-010 D1;
   documented the missing BLOCKED/STILL-NEEDED branches and applicable-only
   evidence silence.
3. Added GH #15 for anchored/preimage-complete recipe generation; GH #13 is
   blocked on it because every current autogen recipe is a whole-file write and
   no non-test writer populates `preimage_hash`.
4. Made candidate replay all-or-nothing over the complete operation/patch
   coverage set.
5. Qualified external-upstream adoption evidence and replaced “restore” with
   “implement” in the replay ticket.
6. Added structural-recipe reconcile, second-apply, duplicate-anchor and
   deleted-target resurrection assertions to the tracked tpatch script.
7. Cross-linked GH #12 and GH #14 on shared dependency disposition.

## Rev-2 Review and Rev-3 Errata

**Internal verdict**: APPROVED
**External verdict**: APPROVED WITH NOTES
**Reviewed tip**: `95cf86e`

- Made the Git fixture hermetic against missing clone identity and global commit
  signing configuration.
- Recorded that SPEC's phase-2 conflict/mixed terminal outcomes are safe only
  for GH #15-authoritative recipes; legacy/ineligible recipes must retain the
  later evidence path.
- Required terminal phase-2 outcomes to preserve matched operation/path
  evidence.
- Clarified issue ownership: GH #15 generates authoritative recipes; GH #13
  consumes them and independently enforces eligibility/all-or-nothing replay.

## Rev-0 Review and Rev-1 Adjudication

**Internal verdict**: NEEDS REVISION
**External verdict**: NEEDS REVISION
**Reviewed tip**: `b19ea6a`

1. Corrected swapped issue identities: GH #13 is operation replay, GH #12 is
   absorption/compaction, GH #14 is reorder.
2. Reframed replay as ADR-010 D1 contract fidelity plus reviewed candidate
   generation, not automatically safe deterministic execution.
3. Added hard gates for idempotency, anchor uniqueness, intentional deletion
   versus missing creation target, preimage authority, and lossy delete/rename
   recipes.
4. Distinguished external `upstream_merged` from this fork's landed baseline;
   qualified `--auto-drop-merged`; dispositioned `unapplied`.
5. Documented the manual reorder composition (`unapply` → `apply` → patch
   refresh/fixup → dependency rewire) and the equivalence/transaction gap.
6. Restored WP-001's historical read-only backlog rows and moved issue links to
   a post-snapshot note.
7. Added a tracked self-validating tpatch trial; session files are no longer
   load-bearing evidence.
8. Narrowed Git conclusions to the tested fixture/default behavior and restored
   the required `Current State`/`WAVE_BASE` handoff fields.
9. Corrected the delete-first expected tree, pinned SPEC §7's missing phase-2
   branches, added all-or-nothing coverage, and split recipe-generation
   fidelity into GH #15.

## Session Summary

### Git reproduction

The tracked disposable-repository script tests four Go argument-list shapes:

| Feature shape | Upstream shape | Merge | Rebase |
|---|---|---|---|
| add after adjacent arguments | delete both adjacent arguments | conflict | conflict |
| add before adjacent arguments | delete both adjacent arguments | conflict | conflict |
| add between adjacent arguments | delete one adjacent argument | conflict | conflict |
| separate `append(args, ...)` | delete both old arguments | clean | clean |

Conclusion for this fixture under default Git behavior: rebase does not avoid
the conflict; it replays the feature commit onto the same changed base. Merge
versus rebase is branch-history policy, not a semantic-conflict avoidance
choice here.

### Current tpatch experiment

- `tpatch record` captured the one-file change but autogenerated a whole-file
  `write-file` recipe.
- `tpatch reconcile` safely returned `blocked`,
  `phase-4-forward-apply-conflicts`, high-confidence `edit-overlap`, and
  `human-or-provider-resolution`.
- A hand-authored anchor-based `replace-in-file` recipe produced the desired
  candidate tree once: deleted upstream arguments stayed deleted; feature
  arguments were present.
- Reconcile still blocked with that applicable structural recipe because phase
  2 recognizes `allPresent` only; it does not execute or report applicable-only
  operations.
- Repeating that recipe duplicates the arguments; duplicated anchors select
  the first match; missing whole-file targets can be resurrected; delete/rename
  autogeneration is lossy. Automatic replay is therefore unsafe today.

## Current State

### Existing surface mapping

- Provider-backed `reconcile --resolve` already supplies semantic resolution in
  a shadow worktree.
- ADR-010 D1 and SPEC §7 specify operation-level behavior beyond shipped
  phase-2's `allPresent` fast path.
- `upstream_merged` records external-upstream adoption evidence (with a
  documented human-review residual); local baseline landing is a distinct axis
  handled by `land`/landed verification.
- `feature unapply` is an existing patch-absent/full-history tier.
- `reconcile --auto-drop-merged` applies only to opted-in phase-1.5 patch-id
  matches; `remove` supplies general complete deletion. There is no
  intent/stub compaction tier.
- `feature deps add/remove` supports atomic metadata edits and cycle checks, but
  does not prove patch-order equivalence or refresh patch/base provenance.
- Manual reorder can compose unapply/apply, patch refresh/fixup and dependency
  rewiring, but is not transactional and proves no equivalence.
- Existing research already names `feat-feature-autorebase`,
  `feat-feature-reorder`, `feat-feature-standalonify`, commutation graphs and
  search planning; the new issues bind the user cases to those backlogs.

## Files Changed

- `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md`
- `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/reproduce.sh`
- `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/reproduce-tpatch.sh`
- `docs/state-of-the-art/case-studies/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/whitepapers/WP-001-feature-slice-gap.md`
- `docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md`
- `docs/handoff/CURRENT.md`
- `docs/ROADMAP.md`
- `docs/supervisor/LOG.md`

## Verification

- Tracked reproduction:
  `adjacent-after/before/between` each produce merge=1 and rebase=1;
  `separate-append` produces merge=0 and rebase=0.
- Current tpatch binary built successfully for the disposable trial.
- Recorded whole-file recipe reconcile: blocked/edit-overlap.
- Structural recipe dry-run and execute: success; desired final argument list.
- Safety fixtures: second replay duplicates arguments; duplicate anchors choose
  the first match; whole-file replay recreates a missing target.
- Applicable-only structural reconcile is tracked: it falls through to phase 4
  without a phase-2 operation note.
- No production source, test, asset, SPEC, lifecycle state or schema changed.

## Next Steps

1. Internal and external review of the empirical claims and issue boundaries.
2. Fold corrections, then archive this research handoff.
3. Restore `implement-prepare-check` as the active Not Started task.
4. Treat GH #12–#14 as research/planning backlog; none may preempt the accepted
   `prepare --check` implementation without a new supervisor dispatch.

## Blockers

None for research review.

## Context for Next Agent

- Debugging outputs remain available in
  `files/repro-adjacent-args-output.txt` and
  `files/repro-tpatch-intent-replay-output.txt`.
- The tracked `reproduce-tpatch.sh` now carries the load-bearing trial; session
  outputs remain debugging detail only.
- The synthetic fixture is evidence for an operation-candidate gap,
  not evidence that every textual conflict is semantically independent.
- Do not create a new `absorbed` state without first disproving
  `upstream_merged` plus a retention overlay.
