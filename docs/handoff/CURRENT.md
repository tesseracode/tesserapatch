# Current Handoff

## Active Task

- **Task ID**: `feat-skill-doc-references-user-visible`
- **Milestone**: post-v0.8.0 / pre-v0.8.1
- **Description**: Implement PRD-skill-doc-strategy / ADR-020. Remove all `docs/land.md` and `docs/reconcile.md` repo-relative references from the six shipped skill surfaces and replace them with concise inline action snippets per ac.3. Add `TestSkillDocReferencesAreSelfContained` parity guard per ac.4. No `.tpatch/` migration; no new CLI flags.
- **Status**: In Progress (implementer dispatched)
- **Assigned**: 2026-05-14

## Session Summary

v0.8.0 shipped: tag `v0.8.0` annotated at `29a6732` (CHANGELOG release-flip on top of tracking-close `e79c7d9`). Pushed to `origin`. M17 cluster archive landed in HISTORY at `e79c7d9`. Now starting the skill-doc-references slice with PRD/ADR-020 already approved at `2e0b791`.

## Current State

- `main` at `29a6732` (release-flip commit). Tag `v0.8.0` pushed to `origin`.
- Worktree clean. Untracked: `.dbg/` (local artifacts).
- All M17 work landed; no in-flight code regions.
- Ready surfaces for this slice (12 references total — see PRD §5.1 table):
  - `assets/skills/claude/tessera-patch/SKILL.md` (lines 68-69)
  - `assets/skills/copilot/tessera-patch/SKILL.md` (lines 43-44)
  - `assets/prompts/copilot/tessera-patch-apply.prompt.md` (lines 50-51)
  - `assets/skills/cursor/tessera-patch.mdc` (lines 40-41)
  - `assets/skills/windsurf/windsurfrules` (lines 34-35)
  - `assets/workflows/tessera-patch-generic.md` (lines 38-39)
- Parity guard target: `assets/assets_test.go` (existing `skillFiles` table at lines 12-30).

## Files Changed (this slice — planned)

- 6 shipped skill surfaces above.
- `assets/assets_test.go` — new `TestSkillDocReferencesAreSelfContained` (negative regex check on the same `skillFiles` table).

## Test Results

- v0.8.0 tag baseline: `go test ./assets -run TestSkillParityGuard` PASS (0.916s); `go build ./cmd/tpatch` OK; `gofmt -l .` clean.

## Next Steps

1. Implementer landed → sub-agent reviewer → external supervisor review.
2. On approval, tracking close + push.
3. Pick next backlog item (Wave D deferrals to v0.8.1 / parser dedup / a1-followup).

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


