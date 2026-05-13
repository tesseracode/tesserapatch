# Current Handoff

## Active Task

- **Task ID**: `v0.8.0-tag-and-backlog`
- **Milestone**: post-M17 / pre-v0.8.1
- **Description**: M17 boundary-capture cluster complete and externally approved end-to-end (Waves A1+A2, B, C+rev1+2+3+4, D+rev1). Next operator action: tag v0.8.0 at `origin/main` HEAD (`34815e8`), then pick from the M17-deferred backlog (see below) or move to the next milestone.
- **Status**: Ready for tag
- **Assigned**: 2026-05-12

## Session Summary

M17 cluster archived to `docs/handoff/HISTORY.md` (single dated entry covering Waves B, C revs 0-4, and D + rev-1 with full review history). ROADMAP M17 cluster header + Wave B / C / D rows flipped to ✅ with externally-approved-2026-05-12 annotations. SQL todos for the in-progress Wave C revisions marked done.

This is a tracking-only commit — NO code changes.

## Current State

- Worktree clean. Last commit on `main`: `34815e8` (Wave C rev-4 sub-agent verdict log) — externally approved.
- v0.8.0 NOT yet tagged. Awaiting supervisor tag dispatch.
- All M17 PRDs marked complete in their respective files; ADR-019, ADR-020, ADR-021 all Accepted.
- Untracked: `.dbg/` (local artifacts; gitignored / safe to leave).

## Files Changed (this commit)

- `docs/handoff/CURRENT.md` — reset for post-M17 (this file)
- `docs/handoff/HISTORY.md` — prepended M17 final archive entry (Waves B + C + D + revs)
- `docs/ROADMAP.md` — Waves B / C / D ✅; M17 cluster header ✅; v0.8.0 milestone marked awaiting tag
- (No code changes.)

## Test Results

- Last full suite run on `34815e8`: `go test -timeout 180s ./...` passed end-to-end (cli ≈37s, workflow ≈30s, gitutil ≈14s, …).
- Race-detector run on `34815e8`: passed.

## Next Steps

1. Tag v0.8.0 at `34815e8` (supervisor decision; expected next action).
2. Push tag.
3. Pick from the M17-deferred backlog or next milestone:
   - `m17-wave-a1-followup-ambig-discovery-diag` (LOW) — surface candidate refs when ambiguous discovery refuses post-unusable-lock fallback.
   - `m17-wave-a-parser-deduplication` (refactor; pair with the `store → gitutil` import-cycle break).
   - `feat-skill-doc-references-user-visible` (PRD-skill-doc-strategy + ADR-020 now approved at `2e0b791`).
   - PRD-patch-already-upstream-detector §3.2 `--check-applied-only` / §3.3 `--auto-drop-merged` CLI flags + hotfix-kind auto-drop default (Wave D deferrals → v0.8.1+).

## Blockers

None.

## Context for Next Agent

- v0.8.0 covers all of M17 (Waves A1+A2, B, C+rev1-4, D+rev1) plus the v0.7.0-superset Wave A landed on top of the v0.7.0 baseline.
- **Wave A1+A2 are CROSS-COMMIT BOUND** (`1d6179c` ↔ `8fc2e4e`); they must be reverted as a unit if needed (`internal/gitutil/gitutil.go:111-115` references `LockState`/`LockDiagnostic` field declarations defined in A2).
- **Frozen-code regions** remain (touch only with an explicit revision brief):
  - `internal/cli/record_auto*.go` (Wave A1)
  - `internal/cli/record_collision*.go` (Wave B)
  - `internal/workflow/reconcile.go` lines ~196-236 (Wave D phase-1.5) and ~560-700 (Wave A2 lock guard)
  - `internal/workflow/patch_id_detector*.go` (Wave D)
  - `Config.PatchIDDetectorEnabled` default — `false`
  - ADR-019 trailer schema, ADR-021 carve-out scope
- The "Side Research — State-of-the-art middle pass" section below is preserved verbatim from before this commit; it is living research notes and stays in `CURRENT.md` across handoff resets.

---

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


