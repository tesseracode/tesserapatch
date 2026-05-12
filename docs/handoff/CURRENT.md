# Current Handoff

## Active Task

- **Task ID**: `m17-wave-bd-parallel-dispatch` — M17 Wave B + Wave D (v0.8.0 boundary-capture cluster, second + third waves; can ship in parallel)
- **Milestone**: M17 — boundary-capture cluster, v0.8.0
- **Status**: Ready to dispatch. Wave A (A1 + A2) shipped 2026-05-11 (unreleased, bundled into v0.8.0). Waves B + D are independent of each other and can ship as parallel background implementers. Wave C waits for B.
- **Assigned**: 2026-05-11 (immediately after Wave A external approval)

## Wave Status

| Slice | Task ID | PRD | Status | Depends |
|-------|---------|-----|--------|---------|
| A1 | `impl-record-auto-base` | PRD-record-auto-base | ✅ shipped (`1d6179c` + rev-1 `4484e04`) | — |
| A2 | `impl-reconcile-lock-guard` | PRD-reconcile-lock-guard | ✅ shipped (`8fc2e4e`) | — |
| **B** | `impl-record-collision-detection` | PRD-record-collision-detection | ⬜ **ready to dispatch** | A1 (recovery hints need `--auto`) |
| C | `impl-tpatch-land` | PRD-tpatch-land | ⬜ blocked on A1+A2+B | A1, A2, B |
| **D** | `impl-patch-already-upstream-detector` | PRD-patch-already-upstream-detector | ⬜ **ready to dispatch** | independent (default-OFF) |

**Coordination notes for parallel B + D dispatch**:
- Same checkout, same working tree — same risk pattern as Wave A's parallel dispatch.
- Recommend: dispatch B and D in parallel but with explicit "do not touch the other's territory" lists in their briefs.
- B touches `internal/cli/cobra.go` record path + `internal/gitutil/` (collision-signature primitive); D touches reconcile workflow (`internal/workflow/reconcile.go`) + `internal/store/types.go` (config flag `PatchIDDetectorEnabled`).
- Minimal overlap — main shared file is `cobra.go` if D adds a `--no-patch-id-detector` flag. Risk lower than Wave A's A1↔A2 because they share fewer surfaces.

## Just Shipped — M17 Wave A

**M17 Wave A (A1 + A2)** — APPROVED WITH NOTES, archived to HISTORY 2026-05-11. Ship stack: `1d6179c` (A1 v0) + `8fc2e4e` (A2) + `6d67b41` (verdicts) + `4484e04` (A1 rev-1) + `63a0373` (rev-1 verdict). External one revision on A1 (zero-diff false-green + lock-fallback policy); both addressed in `4484e04`. A2 clean. One non-blocking external follow-up captured as backlog `m17-wave-a1-followup-ambig-discovery-diag`. Cross-commit binding (A1 ↔ A2) accepted; HISTORY notes the revert must move both as a unit.

## Tagging Decision (Open)

Wave A alone is partial v0.8.0. Two paths:
1. **Tag `v0.8.0-alpha.1` after Wave A push** — early-adopter signal, enables progress visibility, follows precedent of mid-milestone alphas. Cost: pre-release tag count grows; minor versioning bookkeeping.
2. **Defer tagging until Wave A+B+C+D complete** — single clean `v0.8.0` release. Cost: longer dark period; if Waves B/C/D are large, no incremental ship signal.

Recommendation: defer (option 2). Wave A is internal-facing infrastructure; user-facing value lands with Wave C (`tpatch land`). Tag once at v0.8.0 unless we hit a long pause between waves.

## Dispatch Plan (proposed)

1. Push current 5-commit stack (`1d6179c` + `8fc2e4e` + `6d67b41` + `4484e04` + `63a0373`) to `origin/main`. No tag.
2. Dispatch `m17-wave-b-impl` background agent with `PRD-record-collision-detection.md` as authoritative brief.
3. Dispatch `m17-wave-d-impl` background agent with `PRD-patch-already-upstream-detector.md` as authoritative brief + reminder of default-OFF gating.
4. Each implementer completes → sub-agent reviewer → external supervisor → push. Same pattern as Wave A.
5. After both B and D ship: dispatch `m17-wave-c-impl` (depends on A1+A2+B; D independent, can have shipped or be in flight).
6. After all 4 waves ship + follow-ups assessed: tag `v0.8.0`, archive cluster to HISTORY.

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

### Candidate follow-up names

These are research outputs only, not queued roadmap work:

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



## Blockers

None. Awaiting user direction on:
- Wave A push (5 commits, no tag) — user can drive or supervisor can execute on confirmation.
- Wave B + D parallel dispatch — user OK with same pattern as Wave A?

