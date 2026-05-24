# Current Handoff

## Active Task

- **Task ID**: `wave-gamma-patch-amend-impl-rev2`
- **Milestone**: v0.10.0 Wave γ — `PRD-feature-patch-amend` rev-2 revision after external NEEDS REVISION verdict on rev-1 stack `5ea7a01..cf02c05`.
- **Description**: Address external reviewer's F3 finding (MEDIUM regression introduced by rev-1: the new `parent-generation-stale` gate ignores the existing `features_dependencies` config opt-out, breaking the documented flag-off rollout contract). F1 and F2 fixes from rev-1 remain accepted and unchanged.
- **Status**: Ready to dispatch.
- **Assigned**: 2026-05-23.

### Finding to address (F3 — MEDIUM, BINDING)

**F3 — `parent-generation-stale` gate ignores `features_dependencies` config opt-out**

**Contract**: The original ADR-011 dependency gate is flag-guarded. `internal/workflow/recipe.go:34` documents the contract: "When Config.FeaturesDependencies is false the gate is a no-op". Locked by existing flag-off regression tests:

- `TestApplyExecute_FlagOff_BypassesDependencyGate` (`internal/cli/dependency_gate_apply_test.go:103-106`)
- `TestDependencyGate_FlagOff_PassesEvenWithUnappliedHardParent` (`internal/workflow/dependency_gate_test.go:21`)

The `features_dependencies` flag is the user-controllable rollout switch for dependency enforcement. Any new dependency-related gate MUST honor it.

**rev-1 drift**:
- `checkParentGenerationStaleGate` (`internal/cli/cobra.go:855`) does NOT consult config.
- Called unconditionally at `internal/cli/cobra.go:712` and `:812` (both apply paths), immediately after the flag-gated `CheckDependencyGate` calls at `:707` and `:808`.
- Called unconditionally at `internal/cli/cobra.go:847` (reconcile path, via `checkParentGenerationStaleForReconcile`).
- Helper in `internal/workflow/parent_generation_stale.go` never inspects config.
- New tests in `internal/cli/apply_reconcile_stale_dep_test.go` set `cfg.FeaturesDependencies = true` (line 28). Flag-off path uncovered.

**External reviewer's manual repro**: With `cfg.FeaturesDependencies = false` and a child holding a stale hard-parent snapshot, `apply child` still refused with `parent-generation-stale: apply refused...` and did not execute the recipe. Per the existing flag-off contract this should have been a no-op gate; apply should have proceeded.

### rev-2 required fix

**Implementation**:
1. Gate `parent-generation-stale` enforcement behind `cfg.FeaturesDependencies` for both apply and reconcile. Match the existing `CheckDependencyGate` pattern (early-return no-op when flag is false).
2. **Cleanest location**: inside `checkParentGenerationStaleGate` itself (`internal/cli/cobra.go:855`). Load config at the top of the function; if `!cfg.FeaturesDependencies`, return nil immediately. Both apply and reconcile call sites inherit the gate without duplication.
3. Do NOT silently drop the warning. If the flag is off, the gate is a true no-op (no warning, no refusal) — matching the `CheckDependencyGate` semantics.

**Tests**:
1. Add `TestApplyParentGenerationStaleFlagOffBypassesGate` (or matching test name pattern) alongside `internal/cli/apply_reconcile_stale_dep_test.go`. Setup: `cfg.FeaturesDependencies = false`, child with stale hard parent. Assert: apply executes successfully and writes the recipe output (the regression repro the external reviewer ran).
2. Add `TestReconcileParentGenerationStaleFlagOffBypassesGate` with same setup for reconcile.
3. Both tests must NOT depend on stderr being empty if some other path emits warnings — focus the assertion on "apply/reconcile proceeded and recipe ran" / "reconcile completed cleanly".

### F1 + F2 frozen (no changes from rev-1)

The rev-1 fixes for F1 and F2 are accepted and must NOT regress:

- `feature patch fixup` keeps no `--target`, auto-derives `fixup_of_generation`, mandatory `--reason`, refuses empty manifests.
- `parent-generation-stale` enforcement for hard deps still refuses apply/reconcile with the same diagnostic.
- Soft-dep warning behavior unchanged.

The only change in behavior is: when `features_dependencies = false`, the gate becomes a no-op (matching `CheckDependencyGate`).

### IC4 frozen regions (unchanged from rev-0/rev-1)

All previously frozen regions remain frozen:

- `internal/store/patch_generations.go` — no version/strict-on-unknown/`ErrMalformedManifest` edits.
- `internal/store/claims.go:263` (`LoadClaims`) + `:294` (`SaveClaims`).
- `internal/cli/cobra.go:897-905` + `:1415` (`--force-amend`).
- `internal/gitutil/capture_modes.go:137/:182/:328`.
- `internal/workflow/patch_generations.go:31`.

### Out of scope for rev-2

- Any change to F1 or F2 behavior beyond the flag-gating.
- Any change to D1–D10 implementation.
- Schema version bump.
- CHANGELOG entry.

### Quality gates

1. `gofmt -l .` — zero output (run DIRECTLY).
2. `go vet ./...` — clean.
3. `go build ./cmd/tpatch` — succeeds.
4. `go test ./... -count=1 -race` — all green.
5. `go test ./assets/...` — parity guard passes.
6. Existing flag-off tests (`TestApplyExecute_FlagOff_BypassesDependencyGate`, `TestDependencyGate_FlagOff_PassesEvenWithUnappliedHardParent`) still pass — confirms behavioral consistency with the existing dependency gate.
7. Existing rev-1 hard/soft tests still pass — confirms no regression to flag-on path.

### Manual CLI verification

Reproduce the external reviewer's regression and confirm it's fixed:

```bash
# Setup: child with stale hard parent + features_dependencies=false
# Action: tpatch apply child --mode execute
# Expected before rev-2: refused with "parent-generation-stale: apply refused"
# Expected after rev-2: succeeds, recipe runs

# Same for reconcile
```

Also confirm flag-on behavior still refuses (rev-1 contract):

```bash
# Setup: child with stale hard parent + features_dependencies=true
# Expected: apply/reconcile refuse with parent-generation-stale diagnostic
```

### Reviewer checklist additions for rev-2

- [ ] `checkParentGenerationStaleGate` consults `cfg.FeaturesDependencies` and returns no-op when false.
- [ ] Two new tests cover flag-off bypass for apply and reconcile.
- [ ] Existing flag-off tests (`TestApplyExecute_FlagOff_BypassesDependencyGate`, `TestDependencyGate_FlagOff_PassesEvenWithUnappliedHardParent`) still green.
- [ ] Existing rev-1 hard/soft tests still green.
- [ ] IC4 frozen regions unedited.
- [ ] Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.

### Commit shape

DO NOT amend/rebase rev-0 or rev-1 commits. Land rev-2 as a single additive commit on top of `3bb76a8`:

`fix(cli): gate parent-generation-stale enforcement behind features_dependencies (Wave γ rev-2 F3)`

Or split into two if cleaner: one for the gate, one for the tests. Single commit preferred for an atomic regression fix.

### Session Summary — rev-1 (for context)

rev-1 stack on `main`: `5ea7a01` (F1) → `85f4abe` (F2) → `cf02c05` (handoff). External APPROVED F1, accepted F2 hard/soft enforcement, flagged F3 as a new regression introduced by F2's unconditional gate. Internal reviewer for rev-1 missed F3 — they verified the flag-on path but didn't check flag-off semantics.

### Session Summary — rev-0 (for context, archived)

rev-0 stack on `main`: `df35ab7..7a1326c`. External flagged F1 + F2. Both fixed in rev-1.

## Session Summary

External NEEDS REVISION on rev-1: one new finding (F3 MEDIUM, regression introduced by rev-1's F2 fix). Verdict logged to LOG.md. rev-2 dispatch brief in place. rev-1 commits stay on `main`; rev-2 lands additive.

## Current State

- rev-1 stack (`5ea7a01..cf02c05`) on `main`. F1 + F2 fixes accepted by external reviewer.
- F3 regression: `parent-generation-stale` gate ignores `features_dependencies` config. rev-2 brief dispatched above.
- No blockers.

## Files Changed

Rev-1 committed files:

- `internal/cli/feature_patch.go`
- `internal/cli/feature_patch_test.go`
- `internal/workflow/parent_generation_stale.go`
- `internal/cli/cobra.go`
- `internal/cli/apply_reconcile_stale_dep_test.go`

## Test Results

- Before rev-1 count: 624 `func Test...` declarations.
- After rev-1 count: 630 `func Test...` declarations.
- Targeted tests: `go test ./internal/cli -run 'TestFeaturePatch|TestApply.*ParentGenerationStale|TestReconcile.*ParentGenerationStale' -count=1` — passed.
- F2 targeted tests: `go test ./internal/cli -run 'TestApply.*ParentGenerationStale|TestReconcile.*ParentGenerationStale' -count=1` — passed.
- Wave β byte-identical fixture guard: `go test ./internal/store -run TestPatchGenerationsWaveGammaExistingWaveBetaManifestRoundTrip -count=1` — passed.
- Final quality gates:
  - `gofmt -l .` — zero output.
  - `go vet ./...` — clean.
  - `go build ./cmd/tpatch` — success.
  - `go test ./assets/...` — passed.
  - `go test ./... -count=1 -race` — passed.

## Next Steps

1. Dispatch Wave γ rev-1 reviewer with the F1/F2 checklist.
2. Reviewer should verify the two additive commits and IC4 frozen-region boundaries.
3. On approval, supervisor can push local `main` for external review.

## Blockers

None.

## Context for Next Agent

- The two Wave γ rev-1 commits are local on `main`; do not reorder or squash them before review.
- `record --force-amend` remains Git-rewrite orphan-only; it was not used as a refresh/fixup shortcut.
- `parent-generation-stale` remains a derived overlay; rev-1 only adds hard/soft enforcement/warnings at apply and reconcile entry points.
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
