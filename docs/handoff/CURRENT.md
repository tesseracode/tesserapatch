# Current Handoff

## Status

**Cluster state**: ACCEPTED

Cluster H planning is accepted at rev-13. No implementation wave has been
dispatched and no release tag was created.

## Active Task

- **Task ID**: Cluster H
- **Milestone**: Typed feature resources and capture adapters planning
- **Description**: Define the v1 product and architecture contract for
  explicit non-Git/ignored resources and deterministic capture adapters.
- **Status**: Complete
- **Assigned**: 2026-08-10
- **Accepted**: 2026-08-19
- **WAVE_BASE**: `f04dec7`
- **Accepted writer tip**: `650b44f`

## Session Summary

Accepted:

- `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
- `docs/adrs/ADR-033-resource-capture-boundary.md`

The final contract contains 120 acceptance clauses and a 189-row ADR test
matrix. Internal review returned APPROVED WITH NOTES and external review
returned APPROVED.

## Current State

Cluster H is planning-only and complete. The accepted v1 design provides:

- a separate typed `resources.json` declaration domain;
- explicit ignored-file and logical Git-metadata resources;
- a Dolt `dolt-diff-summary-v1` adapter;
- deterministic immutable capture batches plus an atomic current pointer;
- no raw resource persistence and no raw `.git/**` capture;
- no resource authority over canonical patches or feature lifecycle;
- Linux/macOS locking and bounded adapter-process cleanup.

The final non-blocking review note is an implementation clarification:
`cmd.Wait()` may reap after signaling before the non-reaping observer returns.
That observer may then report `ECHILD`; this is expected secondary completion
and must not alter the already-final classification.

## Files Changed

- `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
- `docs/adrs/ADR-033-resource-capture-boundary.md`
- `docs/ROADMAP.md`
- `docs/handoff/CURRENT.md`
- `docs/handoff/HISTORY.md`
- `docs/supervisor/LOG.md`

## Test Results

- `AC-1` through `AC-120`: contiguous and fully represented.
- ADR Test Matrix rows 1 through 189: contiguous, complete coverage.
- Four resource IDs, batch digest and directory digest recomputed.
- Six PRD/ADR JSON blocks byte-identical.
- Side Research md5:
  `b385fe622db9926f48861105239f113e`.
- Rule 18 trailer verified on all 32 pre-consolidation commits.
- Guarded WIP and `.wave-close-allowlist` unchanged.

## Next Steps

1. Before dispatching Cluster H′, fetch `origin` and record the then-current
   `origin/main` commit as the new implementation `WAVE_BASE`.
2. Dispatch a sequential implementation wave from the Accepted PRD and
   ADR-033.
3. Implement store/wire foundations before CLI and adapter execution, then
   run the accepted 189-row matrix through the normal dual-review loop.
4. Tag only when the separately reviewed implementation release closes.

## Blockers

None.

## Context for Next Agent

- Accepted papers are binding; architecture changes require a new ADR.
- Planning range: `f04dec7..650b44f`; do not reuse `f04dec7` as Cluster H′'s
  implementation base.
- `generic-command` is intentionally out of scope; Dolt is the only v1
  external adapter.
- Resources are audit sidecars, never canonical patch or lifecycle truth.
- Preserve ADR-027 privacy: scanned raw bytes remain bounded in memory and
  are never persisted.
- Preserve the post-reap observer `ECHILD` clarification above when
  implementing the process finalizer.

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
