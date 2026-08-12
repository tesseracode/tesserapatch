# Current Handoff

## Status

**Cluster state**: REV-0 DISPATCHED

v0.15.1 Wave B is dispatched for GitHub issue #8 contract planning.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V7/V8 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: In Progress
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Session Summary

Wave A / GH #7 is accepted, 8/8 gated and closed on GitHub.

Issue #8 reproduces on current `main`:

- V7/V8 pass before land;
- V8 fails after land because the shadow starts at current HEAD, where the
  canonical patch is already materialized;
- full committed-range re-record from the original base does not fix it;
- a landed leaf with no parents reproduces, distinguishing it from GH #2.

## Current State

Root cause is baseline/land semantics, not generation or closure replay:

- `verify.go` allocates the shadow from current HEAD;
- `land` commits the feature overlay into HEAD and intentionally leaves
  `status.apply.base_commit` unchanged;
- V8 forward-checks the already-materialized patch and fails;
- reverse-check succeeds at HEAD, while forward-check succeeds at the
  recorded feature base.

No planning or implementation amendment has landed yet.

## Files Changed

- `docs/ROADMAP.md`
- `docs/handoff/CURRENT.md`
- `docs/supervisor/LOG.md`

## Test Results

- Current-main GH #8 reproduction: confirmed.
- GH #2 v0.11.3 regression remains green.
- Wave B planning validation: pending.
- Side Research md5:
  `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Amend verify/land/ADR-013 contracts with alternatives and a binding choice.
2. Define landing evidence, baseline selection, V7/V8 behavior, drift and
   diagnostics.
3. Build an executable acceptance matrix including landed parents and
   operation kinds.
4. Run independent internal/external planning review until accepted.
5. Dispatch Wave C implementation from a fresh base.

## Blockers

None.

## Context for Next Agent

- Do not implement verify code in Wave B.
- Reverse-apply success is evidence of materialization, not proof of which
  feature/commit owns it.
- Landing trailers include feature slug, patch SHA, recipe SHA and base
  commit; evaluate how reachability, rebases/cherry-picks and multiple matches
  affect authority.
- Preserve independent V7 and V8 checks and GH #2's reset-between-checks fix.
- Consider landed target and landed hard-parent behavior separately.
- Verification remains read-only; no status/index/worktree mutation.
- Stage explicit paths only; no parallel writers; do not touch WIP docs.

## Side Research — State-of-the-art middle pass (2026-05-10)

Paper-only exploratory pass completed for a non-LLM middle layer between
deterministic reconcile heuristics and full provider/coding-agent workflows.
This does **not** change code, schema, CLI behavior, roadmap status, PRDs, or
ADRs.

### Research packet

Created `docs/state-of-the-art/` with docs modeled after the existing market
research / PRD conventions: header block, related links, refresh triggers,
references, open questions, and disputes.

Files:

- `docs/state-of-the-art/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`
- `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`
- `docs/state-of-the-art/patch-capture-context-research-brief.md`
- `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`
- `docs/state-of-the-art/research-roadmap.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`

### Findings

1. Patch theory is useful as vocabulary for identity, inverse, composition,
   commutation, dependency, and conflict, but tpatch should not claim
   Darcs/Pijul guarantees on top of unified diffs.
2. Patch identity should be treated as a ladder: exact bytes, `git patch-id`,
   token fingerprints, AST/CFG/PDG similarity, behavioral checks, and finally
   provider/human intent judgment.
3. Computer-vision feature matching maps to code relocation: detect salient
   code keypoints, compute local descriptors, match across old/new upstream,
   reject outliers, then attempt relocated apply in a shadow tree.
4. Search-based application should operate only on uncertain patch clusters,
   after deterministic dependency/commutation pre-passes shrink the search
   space.
5. Beam search is the likely first practical non-LLM planner; MCTS and
   evolutionary algorithms remain candidates for larger uncertain clusters.
6. Vector retrieval / RAG fits as a distinct middle layer: dense retrieval can
   rank likely patch/hunk/code-region matches below full provider reasoning,
   while generation over retrieved context still belongs to the provider tier.
7. The experiment guide defines collection formats for feature metadata, hunks,
   keypoints, fingerprints, retrieval results, commutation relations,
   candidate apply attempts, metrics, and ground-truth labels.
8. First-party tpatch metadata should be the happy path for tpatch-aware repos:
   current metadata is good for lifecycle/DAG reasoning, but future patch
   generations, dependency version snapshots, operation IDs/read-write sets,
   structural anchors, relation artifacts, and vector manifests would make
   identity and ordering easier before fuzzy fallback.
9. A new patch-capture research brief preserves this PRD/ADR queue and defines
   the next front: Quilt-style explicit file claims, Git index/hook boundaries,
   IDE hooks, coding-agent event logs, and privacy-safe agent context capture.
10. Entire is verified as a concrete prior-art target. Its model uses Git hooks,
    agent hooks, commit trailers, a separate `entire/checkpoints/v1` metadata
    branch, shadow checkpoints, full transcript/session storage, redaction, and
    optional checkpoint remotes. tpatch should borrow the Git-native linking
    pattern but default toward summaries/references over raw transcripts.
11. `docs/state-of-the-art/research-roadmap.md` is now the durable exploratory
    tracker so research can advance independently if `docs/handoff/CURRENT.md`
    is reassigned to implementation work.
12. Amendment models differ by tool: Quilt/StGit usually refresh the managed
    patch, Git supports both amend and fixup/squash-forward workflows, Aider
    favors small commits plus undo, and Entire preserves context links around
    rewrites. tpatch likely needs canonical-current patch plus append-only
    generations, with explicit amend/fixup/fold/fork semantics.

### PRD drafts promoted from research (2026-05-13)

The first capture/metadata foundation PRDs were drafted as paper-only planning
docs:

- `docs/prds/PRD-feature-file-claims.md`
- `docs/prds/PRD-record-capture-modes.md`
- `docs/prds/PRD-feature-patch-identity-metadata.md`
- `docs/prds/PRD-feature-patch-amend.md`

`docs/state-of-the-art/research-roadmap.md` is updated to point at these drafts.
The remaining gate before implementation is review/acceptance of the queued
capture privacy and amendment-policy ADRs plus PRD review.

### Candidate follow-up names

These are research outputs only, not queued roadmap work. Four items below now
have draft PRDs as noted above.

- `PRD-structural-patch-fingerprints`
- `PRD-feature-patch-identity-metadata`
- `PRD-dependency-version-snapshots`
- `PRD-recipe-operation-identity`
- `PRD-structural-anchor-manifest`
- `PRD-patch-vector-index`
- `PRD-reconcile-commutation-graph`
- `PRD-reconcile-search-planner`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-planner-audit-artifacts`
- `PRD-feature-file-claims`
- `PRD-record-capture-modes`
- `ADR-patch-amendment-policy`
- `PRD-feature-patch-amend`
- `PRD-active-feature-session`
- `PRD-agent-event-log`
- `PRD-ide-capture-hooks`
- `PRD-git-hook-capture-guards`
- `ADR-capture-context-privacy-boundary`
- `ADR-capture-metadata-branch`
- `PRD-record-context-summary`
