# Current Handoff

## Active Task

- **Task ID**: post-`feat-skill-doc-references-user-visible` / pre-v0.8.1 backlog selection
- **Milestone**: post-v0.8.0 / pre-v0.8.1
- **Description**: Slice `feat-skill-doc-references-user-visible` shipped and archived to `docs/handoff/HISTORY.md`. Next: supervisor selects from the M17-deferred backlog — `m17-wave-a1-followup-ambig-discovery-diag` (LOW), `m17-wave-a-parser-deduplication` (refactor), or the Wave D deferrals (`--check-applied-only`, `--auto-drop-merged`, hotfix-kind auto-drop default) — and optionally opens v0.8.1 around one of those.
- **Status**: Awaiting selection.
- **Assigned**: 2026-05-14 (tracking-close).

## Session Summary

Tracking-only commit closing the skill-doc-references slice. Previous stack tip: rev-3 `47e2888` (CURRENT.md handoff sync, doc-only). This commit (tracking-close) is doc-only on top of that stack: prepends the slice archive entry to `docs/handoff/HISTORY.md`, resets `docs/handoff/CURRENT.md` to a clean post-slice state, and opens the `v0.8.1 (in development)` section in `CHANGELOG.md`. No code, no tests, no `.tpatch/` artifact changes.

## Current State

- Previous stack tip before this commit: rev-3 `47e2888`. v0.8.0 tagged at `29a6732`. No in-flight code regions; worktree expected clean after this commit (only doc files touched).
- All M17 work shipped under v0.8.0. The skill-doc-references slice ships as the first item under v0.8.1.

## Files Changed (this commit)

- `docs/handoff/HISTORY.md` — prepended slice archive entry `# 2026-05-14 — feat-skill-doc-references-user-visible — APPROVED` covering scope, ship-stack table, 4-cycle review history, code anchors, files touched, test results, frozen-code respect, deferred-backlog status.
- `docs/handoff/CURRENT.md` — Active Task / Session Summary / Current State / Files Changed / Test Results / Next Steps / Blockers / Context blocks reset to clean post-slice state. The `## Side Research — State-of-the-art middle pass (2026-05-10)` section below is preserved byte-identical from rev-3.
- `CHANGELOG.md` — new `## v0.8.1 (in development)` section opened above the v0.8.0 header with the skill-doc-references entry under `### Skill assets`.
- SQL `todos` row `feat-skill-doc-references-user-visible` flipped from `pending` → `done`.

## Test Results

Doc-only commit; no tests run beyond smoke gates. Pre-commit gates green: `gofmt -l .` empty; `go build ./cmd/tpatch` OK; `go test ./assets -run TestSkillDocReferencesAreSelfContained -count=1 -v` → 14 sub-tests PASS (8 probes + 6 surfaces). Full `./...` baseline covered by the rev-1 race-clean run (55.077s wall).

## Next Steps

1. Supervisor selects the next backlog item from `m17-wave-a1-followup-ambig-discovery-diag`, `m17-wave-a-parser-deduplication`, or the Wave D deferrals.
2. On selection, supervisor dispatches a new implementer cycle (rewriting Active Task / Session Summary / Current State here, leaving the Side Research section below untouched).
3. Push + (optional) tag are supervisor-owned for the v0.8.0 → v0.8.1 transition; this tracking-close commit is intentionally local.

## Blockers

None.

## Context for Next Agent

- v0.8.0 tag SHA: `29a6732` (annotated; pushed). Last code commit on `main` before this tracking-close: rev-3 `47e2888` (doc-only sync; rev-1 `dd6506a` is the last code-touching commit).
- **Frozen-code regions** remain (touch only with an explicit revision brief):
  - `internal/cli/record_auto*.go` (Wave A1).
  - `internal/cli/record_collision*.go` (Wave B).
  - `internal/workflow/reconcile.go` lines ~196-236 (Wave D phase-1.5) and ~560-700 (Wave A2 lock guard).
  - `internal/workflow/patch_id_detector*.go` (Wave D).
  - `Config.PatchIDDetectorEnabled` default — `false`.
  - ADR-019 trailer schema, ADR-020 inline-minimal skill-doc policy, ADR-021 carve-out scope.
- **Wave A1+A2 are CROSS-COMMIT BOUND** (`1d6179c` ↔ `8fc2e4e`); revert as a unit if needed (`internal/gitutil/gitutil.go:111-115` references `LockState`/`LockDiagnostic` field declarations defined in A2).
- The `TestSkillDocReferencesAreSelfContained` parity guard now pins the inline-minimal policy; do NOT reintroduce any repo-relative `docs/*.md` reference in `assets/`. URL-prefixed forms (`https://`, `http://`, `file://`) are allowed.
- The `## Side Research — State-of-the-art middle pass (2026-05-10)` section below is preserved verbatim from rev-3; it is living research notes and stays in `CURRENT.md` across handoff resets.

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


