# Current Handoff

## Active Task

- **Task ID**: `v0.8.1-wave-d-deferrals` (kickoff)
- **Milestone**: v0.8.1 marquee — Wave D detector tails
- **Description**: Land four post-detector deferrals: (1) `tpatch reconcile --check-applied-only` flag (PRD §3.2), (2) `tpatch reconcile --auto-drop-merged` flag (PRD §3.3), (3) ADR-022 documenting the **deferral** of `Config.PatchIDDetectorEnabled` default-on flip, (4) ADR-023 documenting the **deferral** of hotfix-kind auto-drop default. Items 1+2 are CLI/code; items 3+4 are docs-only. Process rule (user-confirmed): ADR only when flipping a default OR changing lifecycle automation; pure CLI surface adds ship without ADR. ADRs use Status: **Accepted** with the decision being "defer to ≥v0.9 pending criteria below" — not "Proposed/undecided".
- **Status**: In progress (implementer for items 1+2 dispatched; ADRs 3+4 drafted in parallel by supervisor).
- **Assigned**: 2026-05-14.

## Session Summary

Opening the v0.8.1 marquee. Latest finalized stack before this kickoff: tracking-close `2f8f681` (skill-doc-refs archive + v0.8.1 changelog opened), rev-3 `47e2888` (handoff sync), rev-1 impl `dd6506a`, v0.8.0 tag at `29a6732`. This handoff write is doc-only and does not advance code; it scopes the slice for the implementer + ADR-writer tracks.

## Current State

- Worktree clean at the tracking-close tip; v0.8.1 CHANGELOG section open with one entry (skill-doc-refs).
- Phase 1.5 detector primitives shipped in v0.8.0 (frozen): `Config.PatchIDDetectorEnabled` default `false`, `Config.PatchIDScanLimit` default 5000, `store.PatchIDMatch` struct, `gitutil.PatchID` / `CommitPatchID` / `RevListInRange`, `internal/workflow/patch_id_detector.go`, reconcile slot at `internal/workflow/reconcile.go` ~196-236.
- Items 1+2 are pure CLI surface adds that **consume** existing `ReconcileSummary.PatchIDMatch` outputs without modifying frozen regions.
- Items 3+4 ADRs are deferral documents; defaults stay where they are (detector off, auto-drop opt-in, hotfix-kind value not yet shipped anywhere).

## Files Changed (this commit — handoff scope only)

- `docs/handoff/CURRENT.md` — Active Task / Session Summary / Current State / Next Steps / Context blocks rewritten for the new slice. Side Research section preserved byte-identical.
- SQL: `v0.8.1-wave-d-deferrals` flipped `pending` → `in_progress`; sub-todos inserted for items 1+2 implementer track and items 3+4 ADR track.

## Test Results

Handoff-only commit; no test gates needed. The implementer commit lands with `gofmt -l .`, `go build ./cmd/tpatch`, and the full `go test ./...` race-clean suite.

## Next Steps

1. Implementer (background) lands items 1+2: cobra flags on `tpatch reconcile`, plumbing into existing reconcile orchestrator without touching frozen detector code, exit-code semantics (0=match, 2=no-match for `--check-applied-only`), `--auto-drop-merged` removal-commit path with trailer preservation, tests for opt-in/opt-out/no-detector matrices, CHANGELOG bullet.
2. Supervisor drafts ADR-022 (detector default-on deferral) and ADR-023 (hotfix auto-drop deferral) in parallel.
3. Sub-agent code-review on items 1+2 commit.
4. External supervisor review on the combined stack.
5. Tracking-close + push + (eventually) v0.8.1 tag once the marquee scope wraps.

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

