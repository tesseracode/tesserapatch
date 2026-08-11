# Current Handoff

## Status

**Cluster state**: REV-0 DISPATCHED

Cluster H′ rev-0 implementation is dispatched from the corrected and
re-gated Cluster H planning close.

## Active Task

- **Task ID**: Cluster H′ rev-0
- **Milestone**: v0.15.0 typed feature resources and capture adapters
- **Description**: Implement the Accepted Cluster H PRD and ADR-033
  end-to-end.
- **Status**: In Progress
- **Assigned**: 2026-08-11
- **WAVE_BASE**: `46c984b`
- **Target release**: v0.15.0

## Session Summary

Cluster H planning closed with an 8/8 amended wave-close after folding the
post-close review's real-date and citation-anchor notes. A single sequential
implementer is now assigned to Cluster H′ rev-0.

Binding authorities:

- `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
- `docs/adrs/ADR-033-resource-capture-boundary.md`

## Current State

No Cluster H′ production code has landed yet. Baseline `46c984b` is pushed,
clean for tracked files, and passes the full suite. Rev-0 must implement:

- deterministic resource declarations and capture wire artifacts;
- ignored-file and logical Git-metadata capture;
- the trusted Dolt `dolt-diff-summary-v1` adapter;
- Linux/macOS local locking and bounded process cleanup;
- `feature resource add|list|remove|clear|trust-dolt|capture|diff`;
- `record --resources` two-domain staging/publication;
- shared redaction, path/ignore gates, docs and tests.

## Files Changed

- `docs/ROADMAP.md`
- `docs/handoff/CURRENT.md`
- `docs/supervisor/LOG.md`

## Test Results

- Amended Cluster H planning wave-close: PASS 8/8 at `46c984b`.
- Cluster H′ implementation validation: pending.
- Side Research md5:
  `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Implement the Accepted 120-clause / 189-row contract sequentially.
2. Run `gofmt`, targeted tests, `go test ./...`, and
   `go build ./cmd/tpatch`.
3. Update this handoff to AWAITING REVIEW with exact files, counts and
   residuals; push the explicit-path commit.
4. Run independent internal and external rev-0 reviews.
5. Tag v0.15.0 only after the implementation wave is accepted.

## Blockers

None.

## Context for Next Agent

- Accepted papers are binding; architecture changes require a new ADR.
- Implementation WAVE_BASE is `46c984b`; planning WAVE_BASE `f04dec7` is
  historical only.
- One implementer owns all shared surfaces; do not launch parallel writers.
- Stage and commit explicit file paths only.
- Do not touch pre-existing untracked PRDs, whitepapers or case-study files.
- `generic-command` is intentionally out of scope; Dolt is the only v1
  external adapter.
- Resources are audit sidecars, never canonical patch or lifecycle truth.
- Preserve ADR-027 privacy: scanned raw bytes remain bounded in memory and
  are never persisted.
- A post-reap observer `ECHILD` is expected secondary completion and must not
  alter the already-final process classification.

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
