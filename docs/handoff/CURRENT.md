# Current Handoff

## Active Task

- **Task ID**: `wave-gamma-patch-amend-impl-rev1`
- **Milestone**: v0.10.0 Wave γ — `PRD-feature-patch-amend` rev-1 revision after external NEEDS REVISION verdict on rev-0 stack `df35ab7..7a1326c`.
- **Description**: Address external reviewer's two findings (F1 HIGH `fixup --target` off-contract; F2 MEDIUM `parent-generation-stale` not enforced on apply/reconcile for hard dependents). Both are real ADR-026 violations — the rev-0 implementation matched the supervisor's kickoff brief but the brief drifted from ADR-026 D4 on F1. ADR is binding; rev-1 must match ADR-026 verbatim.
- **Status**: Ready to dispatch.
- **Assigned**: 2026-05-22.

### Findings to address (both BINDING per ADR-026)

**F1 — HIGH — `fixup` CLI surface contradicts ADR-026 D4 + D7**

ADR-026 D4 (`docs/adrs/ADR-026-patch-amendment-policy.md:93-94`): `fixup_of_generation` value is "the previously current generation at fixup capture time."
ADR-026 D7: subverbs are `{refresh, fixup}` with `--reason` mandatory for fixup; no `--target` flag is part of the locked surface.
PRD §4.2 (`docs/prds/PRD-feature-patch-amend.md:171-190`): same binding semantics — auto-target the previously-current generation.

**rev-0 implementation drift**:
- `internal/cli/feature_patch.go` registers `--target` and makes it mandatory.
- The handler writes whatever `generation_id` the user supplied (only validates the ID exists in the manifest).
- `internal/cli/feature_patch_test.go` encodes the off-contract surface.

**rev-1 fix**:
1. Remove the `--target` flag from `feature patch fixup`.
2. Derive `fixup_of_generation` automatically: it MUST equal the manifest's `current_generation` at fixup capture time (the generation_id of the entry whose `generation` integer matches `current_generation`).
3. Refuse fixup when the manifest has no prior generations (clear diagnostic; mirrors D3 semantics).
4. `--reason <text>` remains MANDATORY for fixup per D2.
5. Update `internal/cli/feature_patch_test.go`:
   - Drop tests asserting `--target` mandatory behavior.
   - Add a test that fixup with no `--target` flag succeeds and sets `fixup_of_generation` to the previously-current generation.
   - Add a test confirming `--target` is no longer a registered flag (or that passing it errors).
   - Add a test that fixup on a feature with no prior generations refuses with a clear diagnostic.

**F2 — MEDIUM — `parent-generation-stale` not enforced on apply/reconcile for hard dependents**

ADR-026 D5 (`docs/adrs/ADR-026-patch-amendment-policy.md:122-126`): "Soft-dependency dependents warn. Hard-dependency dependents block on apply and reconcile according to ADR-011's hard/soft policy."
ADR-011 (`docs/adrs/ADR-011-feature-dependencies.md:47-56`): hard/soft dependency policy.

**rev-0 implementation drift**: `internal/workflow/parent_generation_stale.go` exists and the overlay surfaces in `status`/`status --json`. But apply and reconcile do NOT consult the helper. Hard stale dependents proceed silently.

**rev-1 fix**:
1. Thread `ParentGenerationStale` detection into `apply` and `reconcile` paths.
2. For **hard** dependencies: when a dependent feature is `parent-generation-stale` against a hard parent, `apply` and `reconcile` MUST refuse with a clear diagnostic naming the stale parent generation_id and current parent generation_id. Suggest `tpatch feature patch refresh <parent>` or `tpatch reconcile <child>` as remediation.
3. For **soft** dependencies: warn only, do not refuse. Existing soft-dep warning patterns in the codebase are the model — match the warning style.
4. Tests: add explicit coverage for both hard-blocks-with-diagnostic and soft-warns-with-message in `internal/cli/feature_patch_test.go` or a new `apply_stale_dep_test.go` / `reconcile_stale_dep_test.go` as appropriate. Both `apply` and `reconcile` need coverage.

### Binding contract (UNCHANGED from rev-0)

- `docs/adrs/ADR-026-patch-amendment-policy.md` — D1–D10 + IC1–IC6 appendix.
- `docs/prds/PRD-feature-patch-amend.md` — product surface and acceptance criteria.
- `docs/adrs/ADR-024-patch-generation-manifest-boundary.md` — Wave β manifest contract; D1–D9 frozen.
- `docs/adrs/ADR-013-verify-freshness-overlay.md` — verify-cache invalidation inputs (D6).
- `docs/adrs/ADR-011-feature-dependencies.md` — hard/soft dependency policy (D5).

### Frozen regions (IC4) — STILL FROZEN

All IC4 frozen regions from rev-0 remain frozen for rev-1:

- `internal/store/patch_generations.go` — manifest v1 schema. Wave γ extensions from rev-0 are accepted; no further version/strict-on-unknown/`ErrMalformedManifest` edits.
- `internal/store/claims.go:263` + `:294`.
- `internal/cli/cobra.go:897-905` + `:1415` (`--force-amend`).
- `internal/gitutil/capture_modes.go:137/:182/:328`.
- `internal/workflow/patch_generations.go:31`.

### Out of scope for rev-1

- Any change to D1, D2, D3, D6, D8, D9, or D10 implementations from rev-0.
- Schema version bump (still v1).
- New CLI surfaces beyond removing `--target` from fixup.
- CHANGELOG entry.

### Quality gates (UNCHANGED)

Before claiming completion:

1. `gofmt -l .` — zero output (run gofmt directly, NOT piped through `grep -v '^$'`).
2. `go vet ./...` — clean.
3. `go build ./cmd/tpatch` — succeeds.
4. `go test ./... -count=1 -race` — all green.
5. `go test ./assets/...` — parity guard passes.
6. Existing Wave β fixtures still load byte-identically.
7. Manual CLI verification:
   - `tpatch feature patch fixup <slug> --reason 'why'` succeeds (no `--target` needed) and writes `fixup_of_generation` equal to previously-current generation_id.
   - `tpatch feature patch fixup <slug> --target <id>` is rejected (flag not registered) OR `--target` accepted no-op for BC if desired (NOT desired — drop it).
   - `tpatch apply <child>` refuses when child is `parent-generation-stale` and the dep is hard; warns and proceeds when dep is soft.
   - `tpatch reconcile <child>` same.

### Reviewer checklist additions for rev-1

In addition to the standard `AGENTS.md` checklist:

- [ ] F1: `--target` flag removed from `feature patch fixup`; `fixup_of_generation` auto-derived from manifest `current_generation`.
- [ ] F1: Tests assert auto-target behavior and absence of `--target` flag.
- [ ] F1: Fixup on a feature with no prior generations refuses cleanly.
- [ ] F2: `apply` consults `ParentGenerationStale` helper; hard-stale dependents refuse with diagnostic; soft-stale warns.
- [ ] F2: `reconcile` same hard/soft split.
- [ ] F2: Explicit test coverage for both hard-refuse and soft-warn on both `apply` and `reconcile`.
- [ ] IC4 frozen regions unedited (no regression).
- [ ] Side Research md5 invariant preserved: `b385fe622db9926f48861105239f113e`.

### Session Summary — rev-0 (for context, not new work)

The rev-0 stack is on `main` at `df35ab7..7a1326c`:

- `df35ab7` — schema extension + tripwire test (IC1 step 1, IC2)
- `2de7242` — kind enum + `ClassifyPatchGenerationKind` (IC1 step 2, IC3)
- `b125b0b` — CLI + stale overlay + skill assets (IC1 step 3, IC5)
- `7a1326c` — implementer handoff

rev-1 must NOT amend, rebase, or rewrite these commits. It must land as **new commits on top** of `7a1326c`, additive in the same conventional-commit style. Suggested commit shape:

1. `fix(cli): drop --target from feature patch fixup; auto-derive fixup_of_generation (rev-1 F1)`
2. `feat(cli): enforce parent-generation-stale on apply/reconcile per hard/soft policy (rev-1 F2)`

Optionally one combined commit if cleanly atomic, but two separate commits per finding are preferred for review clarity.

### History of prior section (superseded)

[previous rev-0 dispatch brief archived implicitly; see Session Summary]

## Session Summary

External NEEDS REVISION verdict received. Both findings confirmed against ADR-026 verbatim. F1 root cause was a supervisor-brief drift (kickoff prompt said `fixup --target` contradicting D4). Logged verdict to LOG.md. Ready to dispatch rev-1 implementer.

rev-0 stack (`df35ab7..7a1326c`) remains on `main`; rev-1 lands additive commits on top — no amend/rewrite.

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
