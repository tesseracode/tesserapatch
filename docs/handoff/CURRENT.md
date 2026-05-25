# Current Handoff

## Active Task

- **Task ID**: `wp003-wave-alpha-prd1-prd6-impl`
- **Milestone**: WP-003 — Reconcile safety & middle-pass (T56 cluster), Wave α
- **Description**: Implement PRD 1 (`reconcile-verdict-evidence`) and PRD 6 (`reconcile-file-novelty-classifier`) in parallel under ADR-025 D1–D13. Add `reconcile-evidence.jsonl` writer + reader, `file-novelty` evidence_kind classifier, atomic write/round-trip, malformed-handling sentinel, and privacy assertion tests.
- **Status**: In Progress (implementer dispatched).
- **Assigned**: 2026-05-25.

## Session Summary

ADR-025 cleared internal + external review with zero findings (commits `06b013e..7b76c01`). WP-003 cluster gate locked. Dispatched Wave α implementer with parallel scope: PRD 1 (evidence schema writer/reader, foundation) + PRD 6 (file-novelty classifier, consumer of the schema).

## Current State

- ADR-025 is the locked cluster gate; Wave α implementation can begin.
- No code changes yet for Wave α.
- Pre-existing `docs/state-of-the-art/` working-tree modifications remain untouched.

## Files Changed

(none yet for Wave α — implementer will populate)

## Test Results

(pending Wave α implementation)

## Next Steps

1. Implementer writes evidence writer/reader in `internal/store/` honoring ADR-025 D1–D5, D10, D11, D12.
2. Implementer wires file-novelty classifier in `internal/workflow/` honoring PRD 6 §3.
3. Implementer adds round-trip, byte-identical determinism, privacy assertion, and malformed-handling tests.
4. Internal reviewer applies schema-drift checklist vs ADR-024 sibling and privacy hard-assertion check.

## Blockers

None.

## Context for Next Agent

- ADR-025 is paper-locked. All schema/enum decisions must come from D1–D13 verbatim.
- Cross-cluster non-drift vs ADR-024 is a HARD requirement: same path layout, same `git-patch-id-stable` algorithm marker, same content-addressing convention (12-hex), same writer-refuses/reader-warns malformed pattern with `errors.Is`-compatible sentinel.
- PRD 6 is a CONSUMER of PRD 1's writer. The implementer can land PRD 1 first, then PRD 6, even though waves are "parallel" — they share a foundation.
- Privacy boundary (D10) is a HARD constraint with a dedicated test obligation: no source bodies, no transcripts, no prompts, no vectors, no embeddings in the JSONL artifact.
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
