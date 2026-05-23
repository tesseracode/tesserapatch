# Current Handoff

## Active Task

- **Task ID**: `wave-gamma-patch-amend-impl`
- **Milestone**: v0.10.0 Wave γ — `PRD-feature-patch-amend` implementation (WP-002 cluster slice 4 of 4).
- **Description**: Implement `tpatch feature patch refresh` and `tpatch feature patch fixup` per the binding contract in `docs/adrs/ADR-026-patch-amendment-policy.md` (D1–D10 + the "Wave γ Implementation Contract" appendix IC1–IC6).
- **Status**: Review (awaiting reviewer).
- **Assigned**: 2026-05-22.

### Binding contract

- `docs/adrs/ADR-026-patch-amendment-policy.md` — D1–D10 are not re-openable. The Wave γ Implementation Contract appendix (IC1–IC6) is also binding.
- `docs/prds/PRD-feature-patch-amend.md` — product surface and acceptance criteria.
- `docs/adrs/ADR-024-patch-generation-manifest-boundary.md` — Wave β manifest contract (D1–D9). Frozen for Wave γ except for the D10 transition (`amend-refresh`/`amend-fixup` become writable).
- `docs/adrs/ADR-013-verify-freshness-overlay.md` — verify-cache invalidation inputs (cited by ADR-026 D6).
- `docs/adrs/ADR-011-feature-dependencies.md` — hard/soft dependency policy (cited by ADR-026 D5).

### Landing order (binding, per IC1)

Wave γ landed in the required IC1 order:

1. **Schema-extension commit** — `df35ab7bf2d1f2490b4d7d204a2cbb9249d34050`
2. **Kind-enum commit** — `2de7242f4aa9a3b593912f87565f317fcbf0ab8e`
3. **CLI surface commit** — `b125b0b40d52a0daee9cc53439475d9dce840e07`

### Frozen regions (per IC4)

The following surfaces were preserved for Wave γ except for the explicit extension points:

- `internal/store/patch_generations.go` — only schema fields, validation for those fields, and writable-kind hook/classification call were touched. Version stayed v1; `DisallowUnknownFields` and `ErrMalformedManifest` classification path were not relaxed.
- `internal/store/claims.go` — not edited.
- `internal/cli/cobra.go:897-905` and `internal/cli/cobra.go:1415` — `--force-amend` behavior/flag were not edited.
- `internal/gitutil/capture_modes.go` — not edited.
- `internal/workflow/patch_generations.go:31` — malformed-manifest swallow path not edited.

### Out of scope for Wave γ

- `fork` / `fold` subverbs (PRD §4.4–4.5; deferred to v2 per ADR-026 D7).
- Metadata-only amend manifest revisions (ADR-026 D9 answers NO for v1).
- WP-003 reconcile cluster work (ADR-025 reserved-but-unwritten).
- Privacy gating on `--reason` (deferred to `ADR-capture-context-privacy-boundary`; ADR-026 D2 is advisory-only).
- Schema version bump (D4 keeps v1).

### Quality gates

All required gates passed after the three commits:

1. `gofmt -l .` — zero output.
2. `go vet ./...` — clean.
3. `go build ./cmd/tpatch` — success.
4. `go test ./... -count=1 -race` — all packages green.
5. `go test ./assets/...` — parity guard green.
6. `go test ./internal/store -run 'TestPatchGenerations' -count=1` — Wave γ contract test and existing Wave β patch-generation fixtures green.
7. Test declaration count: 612 before Wave γ, 624 after Wave γ.

## Session Summary

Wave γ implementation complete and ready for review. Three commits landed in IC1 order:

1. `df35ab7bf2d1f2490b4d7d204a2cbb9249d34050` — schema-extension commit.
   - Files: `internal/store/patch_generations.go`, `internal/store/patch_generations_wavegamma_test.go`.
   - Added `reason` and `fixup_of_generation` as strict v1 known fields with validation for fixup reason and prior-generation target resolution.
   - Added golden Wave γ fixture round-trip, strict unknown-field, missing/empty reason, missing target, and Wave β no-migration round-trip tests.
2. `2de7242f4aa9a3b593912f87565f317fcbf0ab8e` — kind-enum/classifier commit.
   - Files: `internal/store/patch_generation_kinds.go`, `internal/store/patch_generation_kinds_test.go`, `internal/store/patch_generations.go`.
   - Made `amend-refresh` and `amend-fixup` writable while keeping `import` and `manual-metadata` read-only.
   - Added `ClassifyPlainRecordKind` / `ClassifyPatchGenerationKind` and table tests for no-prior record, same-byte no-op, changed plain record → refresh, explicit refresh, and explicit fixup.
3. `b125b0b40d52a0daee9cc53439475d9dce840e07` — CLI/status/assets commit.
   - Files: `internal/cli/feature_patch.go`, `internal/cli/feature_patch_test.go`, `internal/cli/feature_deps.go`, `internal/cli/cobra.go`, `internal/cli/status_dag.go`, `internal/cli/patch_generations_test.go`, `internal/workflow/patch_generations.go`, `internal/workflow/parent_generation_stale.go`, `internal/store/types.go`, and all six shipped skill assets plus `assets/assets_test.go`.
   - Added `tpatch feature patch refresh <slug> [--reason]` and `tpatch feature patch fixup <slug> --target <generation_id> --reason`.
   - Routed plain `record <slug>` through the classifier so later byte-changing records append `kind: amend-refresh`.
   - Added `parent-generation-stale` status/status-JSON overlay and hash-input verify freshness staleness coverage.

IC satisfaction proof:

- **IC1**: Commit order is schema (`df35ab7`) → enum/classifier (`2de7242`) → CLI/status/assets (`b125b0b`).
- **IC2**: `internal/store/patch_generations_wavegamma_test.go` covers all five required contract assertions, including Wave β byte-identical round-trip.
- **IC3**: `internal/store/patch_generation_kinds.go` is the single classification source; `internal/workflow/patch_generations.go` routes record/refresh/fixup writes through it.
- **IC4**: Frozen regions were left untouched outside allowed extension points; no edits to `claims.go`, `capture_modes.go`, or the Wave β malformed-manifest swallow path.
- **IC5**: All six skill assets mention `tpatch feature patch refresh`, `tpatch feature patch fixup`, and `parent-generation-stale`; `go test ./assets/...` passed.
- **IC6**: Reviewer checklist inputs are present here; commit sequence, tests, classifier, frozen-region statement, asset test, and Wave β fixture no-migration result are documented.

## Current State

- Wave γ implementation is complete on local `main` and awaiting reviewer dispatch.
- Existing unrelated working-tree edits under `docs/state-of-the-art/` were present before this task and were not included in the Wave γ commits.
- No blockers are known.

## Files Changed

Committed Wave γ files:

- `internal/store/patch_generations.go`
- `internal/store/patch_generations_wavegamma_test.go`
- `internal/store/patch_generation_kinds.go`
- `internal/store/patch_generation_kinds_test.go`
- `internal/store/types.go`
- `internal/workflow/patch_generations.go`
- `internal/workflow/parent_generation_stale.go`
- `internal/cli/feature_patch.go`
- `internal/cli/feature_patch_test.go`
- `internal/cli/feature_deps.go`
- `internal/cli/cobra.go`
- `internal/cli/status_dag.go`
- `internal/cli/patch_generations_test.go`
- `assets/assets_test.go`
- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/windsurf/windsurfrules`
- `assets/workflows/tessera-patch-generic.md`

## Test Results

- Pre-Wave γ baseline count: 612 `func Test...` declarations.
- Post-Wave γ count: 624 `func Test...` declarations.
- `go test ./internal/store -count=1` after commit 1: passed.
- `go test ./internal/store -count=1` after commit 2: passed.
- `go test ./... -count=1` before commit 3: passed.
- Final quality gates:
  - `gofmt -l .` — zero output.
  - `go vet ./...` — clean.
  - `go build ./cmd/tpatch` — success.
  - `go test ./assets/...` — passed.
  - `go test ./internal/store -run 'TestPatchGenerations' -count=1` — passed.
  - `go test ./... -count=1 -race` — passed.

## Next Steps

1. Dispatch Wave γ reviewer with the IC6 checklist.
2. Reviewer should verify the three-commit sequence and frozen-region boundaries.
3. On approval, supervisor can push local `main` for external review.

## Blockers

None.

## Context for Next Agent

- The three Wave γ commits are local on `main`; do not reorder or squash them before IC6 review.
- `record --force-amend` remains Git-rewrite orphan-only; it was not used as a refresh/fixup shortcut.
- `parent-generation-stale` is status-rendered/advisory and derived from latest child dependency snapshots versus current parent patch generation metadata.
- Side Research section is preserved byte-identical; verify via md5 `b385fe622db9926f48861105239f113e` after any CURRENT.md edit.

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
