# Current Handoff

## Active Task

- **Task ID**: `v0.11.1-slice-1-asset-cli-parity`
- **Milestone**: v0.11.1 stabilization — Slice 1
- **Description**: Close asset/CLI parity findings: apply-recipe schema drift in shipped skills, unsupported fixup `--target` guidance, and stale `verify` V3-V9 help text.
- **Status**: Review (implementation complete; awaiting supervisor review)
- **Assigned**: 2026-07-17.

## Session Summary

v0.11.1 Slice 1 implemented on top of `430aab6`. All six shipped skill/prompt/workflow surfaces now show the canonical `ApplyRecipe` JSON shape, no longer document the rejected `feature patch fixup --target` flag, and describe `tpatch verify` as running V0-V9 real checks. `internal/cli/verify.go` help/comment text now reflects post-Slice-C behavior without changing verify execution logic. `CHANGELOG.md` has a v0.11.1 unreleased stabilization entry.

The asset recipe parity guard was updated to decode examples into `workflow.ApplyRecipe` and require top-level `feature`, so the guard now enforces the same schema as `internal/workflow/implement.go:42`.

## Current State

Slice 1 code/docs are ready for review. No `docs/reconcile.md`, release-ops, draft doctor PRD, or ADR-027 follow-up work was touched. Pre-existing unrelated uncommitted research/whitepaper docs remain in the worktree and were intentionally left unstaged/out of scope.

## Slice 1 closure summary

### Finding 1 — HIGH — apply-recipe schema drift closed

- Fixed all six recipe examples to remove unsupported `version` and add required top-level `feature`:
  - `assets/workflows/tessera-patch-generic.md:128`
  - `assets/prompts/copilot/tessera-patch-apply.prompt.md:101`
  - `assets/skills/copilot/tessera-patch/SKILL.md:116`
  - `assets/skills/cursor/tessera-patch.mdc:111`
  - `assets/skills/claude/tessera-patch/SKILL.md:139`
  - `assets/skills/windsurf/windsurfrules:105`
- Guard aligned with ground truth: `assets/assets_test.go:255`, `assets/assets_test.go:277`, `assets/assets_test.go:286`.
- Test result: `go test ./assets/...` PASS (`ok github.com/tesseracode/tesserapatch/assets 2.326s`).

### Finding 2 — HIGH — unsupported fixup `--target` guidance removed

- Removed `--target <generation_id>` and documented manifest-derived target selection at all six surfaces:
  - `assets/workflows/tessera-patch-generic.md:61`
  - `assets/prompts/copilot/tessera-patch-apply.prompt.md:47`
  - `assets/skills/copilot/tessera-patch/SKILL.md:68`
  - `assets/skills/copilot/tessera-patch/SKILL.md:82`
  - `assets/skills/cursor/tessera-patch.mdc:57`
  - `assets/skills/claude/tessera-patch/SKILL.md:65`
  - `assets/skills/windsurf/windsurfrules:51`
- CLI behavior unchanged: `internal/cli/feature_patch.go:45` defines only `--reason`; `internal/cli/feature_patch_test.go:114` still asserts `--target` is rejected.
- Test result: full `go test ./...` PASS, including `internal/cli` (`115.216s`).

### Finding 3 — MEDIUM — `verify` help/comment staleness closed

- Updated CLI comment/help text to state all V0-V9 checks execute as real checks, with documented precondition skips where applicable:
  - `internal/cli/verify.go:20`
  - `internal/cli/verify.go:52`
- Shipped skill command summaries also now say `tpatch verify` runs V0-V9 checks:
  - `assets/workflows/tessera-patch-generic.md:94`
  - `assets/prompts/copilot/tessera-patch-apply.prompt.md:77`
  - `assets/skills/copilot/tessera-patch/SKILL.md:65`
  - `assets/skills/cursor/tessera-patch.mdc:76`
  - `assets/skills/claude/tessera-patch/SKILL.md:315`
  - `assets/skills/windsurf/windsurfrules:70`
- Evidence read before edit: `internal/workflow/verify_slice_c_test.go:3` covers V3-V6/V9; `internal/workflow/verify_closure_replay_test.go:3` covers V7/V8.
- Test result: full `go test ./...` PASS.

## Files Changed

- `CHANGELOG.md`
- `assets/assets_test.go`
- `assets/workflows/tessera-patch-generic.md`
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/windsurf/windsurfrules`
- `internal/cli/verify.go`
- `docs/handoff/CURRENT.md`

## Test Results

- `go test ./assets/...` — PASS (`ok github.com/tesseracode/tesserapatch/assets 2.326s`).
- `gofmt -l .` — PASS (no output).
- `go vet ./...` — PASS (no output).
- `go build ./cmd/tpatch` — PASS.
- `go test ./...` — PASS (`internal/cli` 115.216s; all other packages ok/cached or no test files).

## Next Steps

1. Supervisor dispatches internal review for v0.11.1 Slice 1.
2. If approved, archive this handoff and proceed to Slice 2 — reconcile docs refresh.
3. Keep Slice 3 release-ops cleanup and Slice 4 `PRD-tpatch-doctor` draft deferred until supervisor dispatch.
4. Do not take ADR-027 F2/F3 follow-ups in this slice.

## Blockers

None.

## Context for Next Agent

- Side Research md5 invariant remains `b385fe622db9926f48861105239f113e`; verify after any future edit with `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.
- Pre-existing unrelated uncommitted docs/research/whitepaper files are present in the worktree from before Slice 1. Do not stage them with Slice 1 commits.
- The parity guard now rejects stale `version` recipe examples because it decodes into `workflow.ApplyRecipe` with `DisallowUnknownFields`.
- `feature patch fixup` target selection remains implementation-derived from the current patch-generation manifest; no CLI `--target` flag should be reintroduced without a new tested contract.

## Carry-forward dispatch rules (all 15 binding for any future implementation)

1. (Wave α) Briefs MUST self-audit against binding ADRs.
2. (Wave α) Briefs naming policy ADRs MUST enumerate config-flag opt-out contracts.
3. (Wave α) Internal reviewer checklist MUST include flag-off counter-scenarios.
4. (Wave α) `gofmt -l .` direct — never piped.
5. (Wave α) Drift self-audit during release prep.
6. (Wave α rev-0) Briefs MUST NOT contain escape hatches that override PRD acceptance.
7. (Wave α rev-1) External reviewer briefs MUST sweep ALL PRD §6 acceptance criteria.
8. (Wave β F1) Briefs MUST enumerate per-PRD §6 display-name contracts as binding production + test contracts.
9. (Wave β F2) Reviewer briefs MUST distinguish "behavior implemented" from "behavior tested". Read production code FIRST.
10. (Wave β F6) State-mutation contracts MUST be verified by reloading from disk.
11. (Wave β F7) Cross-artifact linkage contracts MUST be verified by loading persisted JSONL.
12. (Wave β F3) Privacy tests seed secrets into title/slug/path metadata, NOT file content.
13. (Wave β schema-lock) Briefs say "no persisted-schema additions outside what binding ADRs explicitly authorize" — enumerate which fields/clauses.
14. Two-opinion external review protocol (supervisor + user-parallel) MANDATORY. 7/7 rev cycles has caught HIGH BLOCKERs or confirmed fixes.
15. (γ-1 F1) When PRD names a command/event as trigger, verify the command/event actually exists in production code BEFORE wiring implementation.

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
