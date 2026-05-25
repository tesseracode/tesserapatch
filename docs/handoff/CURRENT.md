# Current Handoff

## Active Task

- **Task ID**: `adr-025-author`
- **Milestone**: WP-003 — Reconcile safety & middle-pass (T56 cluster)
- **Description**: Author `ADR-025-reconcile-evidence-and-revision-schema` as the single cluster ADR gating WP-003 Wave α implementation.
- **Status**: Review (awaiting reviewer).
- **Assigned**: 2026-05-24.

## Session Summary

Created the paper-only ADR-025 gate for WP-003. The ADR locks the v1 schemas and semantic contracts for `reconcile-evidence.jsonl`, `reconcile-revisions.jsonl`, confirmation-gate state, patch-id auto-confirm behavior, privacy constraints, malformed-file handling, versioning, and cross-artifact `refs` compatibility with ADR-024.

No code, CLI behavior, existing PRDs, CLUSTERS.md, or supervisor LOG entries were changed.

## Current State

- ADR-025 is authored and committed, ready for review.
- WP-003 remains implementation-blocked until reviewer/supervisor acceptance of ADR-025.
- Pre-existing `docs/state-of-the-art/` working-tree modifications were left untouched.

## Files Changed

- `docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md` — new ADR file.
- `docs/handoff/CURRENT.md` — handoff updated to mark ADR-025 ready for review.

## Test Results

Paper-only documentation task; no Go code, assets, CLI behavior, or tests changed.

Validation performed:

- `git diff --check -- docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md` — passed before ADR commit.
- `wc -l docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md` — 413 lines, within requested 250–500 line target.

## Next Steps

1. Reviewer reads ADR-025 against WP-003, PRDs 1–3, ADR-024, and the 2026-05-16 supervisor LOG acceptance entry.
2. If approved, supervisor records verdict in `docs/supervisor/LOG.md` and clears the WP-003 ADR gate.
3. After acceptance, Wave α implementation can start for PRD 1 (`reconcile-verdict-evidence`) and PRD 6 (`reconcile-file-novelty-classifier`).

## Blockers

None for review. Implementation remains intentionally blocked until ADR-025 is accepted.

## Context for Next Agent

- This was a paper-only task. Do not infer any implemented schema or CLI behavior from the ADR until the follow-up implementation commits exist.
- Commit 1 adds only the ADR file. Commit 2 updates only this handoff.
- The ADR deliberately defers structural patch fingerprints, commutation graph, structural middle-pass boundary, reconcile search planner/planner audit artifacts, patch vector index, LLM/transcript persistence, and free-text `--reason`/richer agent context persistence.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`. Always verify after editing CURRENT.md.

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
