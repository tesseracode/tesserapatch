# Current Handoff

## Active Task

- **Task ID**: `v0.10.0-beta-patch-identity-metadata`.
- **Milestone**: v0.10.0 Wave beta. Slice 3 of 4 in the capture-and-metadata foundation cluster. Gated on **ADR-024-patch-generation-manifest-boundary** (externally APPROVED 2026-05-16 at `dfffe70`).
- **Description**: Implement `PRD-feature-patch-identity-metadata` v1 — the append-only `.tpatch/features/<slug>/artifacts/patch-generations.json` manifest, per ADR-024 D1–D9. New code in `internal/store/` (manifest types, schema-v1 strict parse, atomic IO, `generation_id` derivation) and `internal/cli/cobra.go` (append from `record`, `reconcile`; malformed-manifest policy per D7). Reuse `internal/gitutil/patch_id.go` `PatchID` for the `git_patch_id` field (D6). Snapshot dependencies from `store.Dependency` at child record time (D5). No CHANGELOG ship yet — that comes at v0.10.0 cluster closeout.
- **Status**: Review.
- **Assigned**: 2026-05-17.

## Session Summary

Wave beta implementation is complete and ready for reviewer pass.

Implemented the ADR-024 / PRD-feature-patch-identity-metadata v1 manifest at `artifacts/patch-generations.json`:

- strict schema-version-1 load/validate with unknown-field refusal;
- deterministic append-only `generation` plus `pg_<12hex>` `generation_id` hashing over the ADR-024 D2 field set with sorted pathspecs / claim IDs;
- atomic `.tmp` + fsync + rename + parent-dir fsync writes;
- required `git_patch_id_algorithm: "git-patch-id-stable"` and `gitutil.PatchID` population;
- no wall-clock timestamp fields;
- latest-only `canonical_patch` pointer (prior entries are cleared on append);
- dependency snapshots from live `status.depends_on` plus parent manifest latest generation/hash when present;
- empty v1 `refs` block.

Record integration now refuses malformed current-feature manifests before artifact writes, appends after successful canonical-byte changes, skips same-latest-byte / recipe-only generations, mirrors alpha-2 provenance values for capture mode/pathspecs/claim IDs, and keeps same-feature audit-snapshot dedup behavior. Status emits a non-fatal one-line warning for malformed manifests.

Reconcile integration appends `kind: "reconcile"` generations from the shadow-accept derived-artifact refresh path when refreshed canonical bytes differ, but ignores malformed manifests so reconcile itself is not blocked (ADR-024 D7 distrust policy).

## Current State

All ADR-024 D1–D9 decisions are represented in code. Existing repositories with no manifest create generation 1 on the next non-empty record without scanning historical `patches/NNN-*.patch` files. Reserved writer kinds remain refused by the append helper; reserved read kinds remain accepted by schema validation.

## Files Changed

- `internal/store/patch_generations.go` (new, 312 lines) — manifest types, strict load/save, validation, generation-id hashing, append semantics, SHA helpers.
- `internal/store/patch_generations_test.go` (new, 139 lines) — store round-trip, strict schema, algorithm marker, generation-id determinism/collision tests.
- `internal/workflow/patch_generations.go` (new, 130 lines) — shared record/reconcile generation builder and dependency snapshot logic.
- `internal/workflow/refresh.go` (modified, 93 lines) — reconcile refresh appends `kind: reconcile` when canonical bytes change.
- `internal/workflow/refresh_test.go` (modified, 258 lines) — reconcile refresh asserts generation append.
- `internal/cli/cobra.go` (modified, 2497 lines) — record/status wiring and record upper-bound mapping.
- `internal/cli/patch_generations_test.go` (new, 240 lines) — CLI coverage for first record, changed re-record, claim IDs, dependencies, same-bytes skip, recipe-only skip, malformed refusal, no-manifest compatibility, and record→reconcile→record smoke.
- `docs/handoff/CURRENT.md` (this update; `## Side Research` preserved byte-identical).

## Test Results

Required validation completed:

- `gofmt -l . | grep -v vendor` → empty
- `go vet ./...` → clean
- `go build ./cmd/tpatch` → succeeds
- `go test ./... -count=1 -race` → all packages green:
  - `assets` ok
  - `cmd/tpatch` no test files
  - `internal/buildinfo` ok
  - `internal/cli` ok
  - `internal/gitutil` ok
  - `internal/provider` ok
  - `internal/safety` ok
  - `internal/store` ok
  - `internal/workflow` ok
  - `tests/integration` ok

Test count after this slice: 608 `func Test...` declarations. Net new patch-generation coverage: 18 new test functions plus one augmented reconcile-refresh test.

Side Research md5 verified before handoff edit: `b385fe622db9926f48861105239f113e`. Re-verify after edit before commit.

## Next Steps

1. Reviewer sub-agent should inspect the implementation against ADR-024 D1–D9 and PRD §8.
2. If approved, supervisor archives this handoff and advances to Wave gamma / patch amendment policy work.

## Blockers

None.

## Context for Next Agent

- `record` appends after recipe autogen so `recipe_sha256` reflects an apply recipe generated or regenerated by the same record run.
- Same-byte duplicate records still skip numbered `patches/NNN-record.patch`; if no manifest exists yet, the first post-PRD same-byte record creates generation 1 with empty `audit_patch` because there is no current generation to compare against.
- Reconcile append currently lives in `workflow.RefreshAfterAccept`, the path that rewrites `artifacts/post-apply.patch` and writes `patches/NNN-reconcile.patch` after shadow accept / auto-apply. Pure upstream-merged verdict paths do not rewrite canonical bytes and do not append.
- The `## Side Research — State-of-the-art middle pass (2026-05-10)` section below must remain byte-identical; expected md5 is `b385fe622db9926f48861105239f113e`.

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
