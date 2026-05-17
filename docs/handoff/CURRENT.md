# Current Handoff

## Active Task

- **Task ID**: `ADR-024-patch-generation-manifest-boundary`.
- **Milestone**: v0.10.0 Wave beta gate. The ADR must be externally APPROVED before the code implementer for `PRD-feature-patch-identity-metadata` (slice 3 of 4 in the capture-and-metadata foundation cluster) can dispatch.
- **Description**: Author ADR-024 to accept (or revise with rationale) the six Implementation-Gate decisions from `PRD-feature-patch-identity-metadata` §"Implementation Gate" (lines 41-51): (1) append-only generation manifest outside `status.json`; (2) monotonic `generation` integer plus content-addressed `generation_id`; (3) no wall-clock timestamps in the artifact; (4) no historical backfill from `patches/NNN-*.patch` in v1; (5) dependency snapshots by parent generation/hash; (6) `git patch-id --stable` as the only persisted patch-id algorithm. Each decision must include alternatives considered and an explicit consequence statement. The ADR is documentation-only — NO code or test changes in this slice.
- **Status**: Implementation drafted — awaiting external review.
- **Assigned**: 2026-05-16.

## Session Summary

ADR-024 (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md`) authored as the Wave beta gate for `PRD-feature-patch-identity-metadata`. The ADR is documentation-only (no Go, asset, or PRD changes). All six PRD §"Implementation Gate" decisions are accepted as written:

- D1 append-only `artifacts/patch-generations.json` separate from `status.json`;
- D2 monotonic `generation` integer plus 12-hex-prefix content-addressed `generation_id` over a pinned field set;
- D3 no wall-clock timestamps;
- D4 no historical backfill from `patches/NNN-*.patch` in v1;
- D5 dependency snapshots pin parent generation/patch_sha256/recipe_sha256;
- D6 `git patch-id --stable` as the sole persisted patch-id algorithm with required `git_patch_id_algorithm` marker.

Both PRD §9 open questions are resolved: D4 closes "should historical numbered patches be backfilled?" (no in v1); D7 closes "should malformed manifests refuse `record` by default?" (yes — `record` refuses, read-only commands warn, reconcile distrusts identity fields but proceeds). D8 closes the `kind` enum to two writable values in v1 (`record`, `reconcile`); the remaining four (`amend-refresh`, `amend-fixup`, `import`, `manual-metadata`) are reserved-but-unused. D9 locks strict-schema reads, `refs` block empty in v1, and clarifies the manifest tracks canonical patch byte changes only.

The ADR also pins reuse of the `claims.json`-style atomic `.tmp` + rename + fsync pattern (`internal/store/claims.go:290`), so no new I/O abstraction is required in Wave beta implementation.

### Prior session context

v0.9.0 Wave alpha shipped 2026-05-14. The cluster delivered the two-slice capture-and-metadata foundation:

1. **alpha-1** (`PRD-feature-file-claims` v1) — `tpatch feature claim <add|list|remove|clear>` with deterministic `.tpatch/features/<slug>/claims.json` manifest, atomic writes, claim_id-derived stable sort, advisory-only mode. Rev-1 fixed F1 (path-normalization gap in `MatchClaim` for `remove`). External APPROVED at `9d7435b`.
2. **alpha-2** (`PRD-record-capture-modes` v1) — `tpatch record` flags `--all`, `--staged`, `--unstaged`, `--claimed-only` with PRD §3.7 mutex matrix, mode-aware untracked-file policy, refuse-on-overlap diagnostics, and capture-mode provenance in `record.md`. Default `record` behavior byte-identical (pinned regression). Rev-1 fixed F1 (`claim_ids` provenance subset for `--claimed-only --files`). External APPROVED at `5d154cd`.

Both archived to `docs/handoff/HISTORY.md`. `v0.9.0` annotated tag pushed.

## Open Threads (informational, not blocking)

- **Wave alpha pre-existing edge case (alpha-1)**: if a claimed directory is removed from disk between `add` and `remove`, `MatchClaim` cannot reconstruct the trailing-slash and `remove <path>` won't match. Workarounds: remove by claim_id or with explicit trailing slash. Could be addressed when the matcher gains "unconditional trailing-slash variants" probing — not currently scoped to any milestone.
- **Cluster continuation**: `PRD-feature-patch-identity-metadata` (slice 3) and `PRD-feature-patch-amend` (slice 4) remain on the roadmap but require ADR-022 (patch-generation-manifest-boundary) and ADR-023 (patch-amendment-policy) respectively. Those ADR slots are reserved.
- **Keypoint experiment**: structural/RAG/search-layer PRDs (`PRD-structural-patch-fingerprints`, `PRD-structural-anchor-manifest`, `PRD-reconcile-commutation-graph`, `PRD-patch-vector-index`, search planner PRDs) deferred until the marker/keypoint experiment produces results.
- **Provider-routing audit** (committed `18fd668` 2026-05-13): empirical findings on `localhost:4141` documented in `docs/MODEL-ROUTING.md`. `TPATCH_ENABLE_RESPONSES_PROVIDER` should remain unset by default.

## Frozen Regions (in effect across slices)

- Wave D reconcile files: `internal/workflow/reconcile_check_applied.go`, `internal/workflow/reconcile_auto_drop.go`, `internal/cli/reconcile_check_applied.go`, `internal/cli/reconcile_auto_drop.go`
- Provider audit files: `internal/provider/{errors,responses,router}.go`
- Wave alpha alpha-1 surface: `internal/store/claims.go`, `internal/cli/feature_claim.go`
- Wave alpha alpha-2 surface: `internal/cli/record_capture_modes.go`, `internal/gitutil/capture_modes.go`
- The `## Side Research` section in this CURRENT.md (preserve byte-identical across handoff resets; md5 `b385fe622db9926f48861105239f113e`)

## Test Results

ADR-024 authored slice (documentation-only):
- `gofmt -l . | grep -v vendor` → empty
- `go vet ./...` → clean
- ADR-024 file present: `docs/adrs/ADR-024-patch-generation-manifest-boundary.md` (363 lines, 14+ concrete file:line citations).
- No code, test, asset, or PRD changes.

## Files Changed

- `docs/adrs/ADR-024-patch-generation-manifest-boundary.md` (new)
- `docs/handoff/CURRENT.md` (this file — Session Summary, Test Results, Files Changed, Status updated; `## Side Research` section preserved byte-identical, md5 `b385fe622db9926f48861105239f113e`)

`v0.9.0` tag verification (carried from prior session):
- `gofmt -l . | grep -v vendor` → empty
- `go vet ./...` → clean
- `go build ./cmd/tpatch` → succeeds
- `go test ./... -count=1 -race` → all 10 packages green at HEAD

## Blockers

None.

## Context for Next Agent

- The repository is at the freshly-tagged `v0.9.0` baseline. Pick the next slice from the supervisor's queue.
- Process patterns: every commit needs the `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer; use `git -c commit.gpgsign=false commit`; push via `GIT_TERMINAL_PROMPT=0 git -c credential.helper= -c credential.helper='!gh auth git-credential' push origin main`; updates to tracking docs (CURRENT.md, HISTORY.md, LOG.md, ROADMAP.md, CHANGELOG.md) at every phase transition per `AGENTS.md` "Context Preservation Rules".
- The `## Side Research — State-of-the-art middle pass (2026-05-10)` section below is preserved verbatim across handoff resets.

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
