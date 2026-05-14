# Current Handoff

## Active Task

- **Task ID**: provider-model-routing-audit
- **Milestone**: post v0.8.1
- **Description**: Verify live copilot-api model routing against `localhost:4141`, preserve a curl-based matrix suite, and document which endpoint/body shape tpatch should use per model family.
- **Status**: Complete.
- **Assigned**: 2026-05-13.

## Session Summary

Live curl audit against `http://localhost:4141` completed for the current
copilot-api model catalog. The proxy returned 43 models, including 22
user-pickable chat models. All 22 succeeded through the same default route
tpatch uses today:

- Claude models advertising `/v1/messages` -> `/v1/messages` with Anthropic
  Messages payloads.
- All other chat models, including GPT-5.2/5.4/5.5 and Gemini, ->
  `/v1/chat/completions` with OpenAI Chat Completions payloads.

Important live finding: several GPT-5.x models advertise `/responses`, but the
local proxy returns `404 Not Found` for both `/responses` and `/v1/responses`.
Keeping `TPATCH_ENABLE_RESPONSES_PROVIDER` unset is therefore the correct
default; GPT-5.x works through chat completions on this proxy.

## Current State

- v0.8.1 shipped; this handoff now records a post-release provider audit.
- `tests/scripts/model-routing-matrix.sh` is the durable curl suite for
  re-running the local proxy matrix. It discovers `/models`, prints catalog
  metadata, tests tpatch default routes, and can optionally probe advertised
  endpoint variants, payload combinations, and SSE.
- `docs/MODEL-ROUTING.md` captures the observed proxy contract and the current
  guidance: route Claude to `/v1/messages`, route GPT/Gemini chat models to
  `/v1/chat/completions`, and keep the experimental responses provider gated
  until the proxy serves `/responses`.

## Files Changed

- `tests/scripts/model-routing-matrix.sh`
- `docs/MODEL-ROUTING.md`
- `tests/integration/provider_model_matrix_test.go`
- `internal/provider/router.go`
- `internal/provider/responses.go`
- `internal/provider/errors.go`
- `docs/handoff/CURRENT.md`

## Test Results

- `tests/scripts/model-routing-matrix.sh --stream --combos` -> all 22
  user-pickable chat models returned 200 with extracted text on the tpatch
  default route; Claude SSE and GPT chat SSE returned valid event streams;
  `/responses` SSE returned 404.
- `tests/scripts/model-routing-matrix.sh --full` -> advertised `/responses`
  and `/v1/responses` routes returned 404 for GPT-5.x models; advertised/chat
  and messages routes returned 200.
- `go test ./internal/provider ./tests/integration` -> pass.
- `TPATCH_LIVE_PROVIDER=1 go test ./tests/integration -run TestLiveLocalProxy -v`
  -> pass; live `PickProvider(...).Generate(...)` smoke returned `TPATCH_OK`
  for `claude-sonnet-4.6`, `gpt-5.5`, `gpt-5.4`, `gpt-5-mini`,
  `gemini-2.5-pro`, `gpt-4.1`, and `gpt-4o`.
- `go test ./...` -> pass.
- `go build ./cmd/tpatch` -> pass.

## Next Steps

1. Keep `TPATCH_ENABLE_RESPONSES_PROVIDER` unset for the copilot-api proxy until a live rerun shows `/responses` (or `/v1/responses`) returning usable text.
2. If the proxy starts serving `/responses`, rerun `tests/scripts/model-routing-matrix.sh --full --stream --combos` before flipping the provider gate.
3. Supervisor picks the next slice from `docs/ROADMAP.md` after this audit is reviewed/archived.

## Blockers

None.

## Context for Next Agent

- v0.8.1 tag SHA: see `git show v0.8.1`. v0.8.0 tag SHA: `29a6732`.
- Provider audit finding: current localhost:4141 advertises `/responses` for
  GPT-5.x but returns 404 for both `/responses` and `/v1/responses`; chat
  completions works for `gpt-5.2`, `gpt-5.2-codex`, `gpt-5.3-codex`,
  `gpt-5.4`, `gpt-5.4-mini`, `gpt-5.5`, and `gpt-5-mini`.
- **Frozen-code regions** (touch only with an explicit revision brief):
  - `internal/cli/record_auto*.go` (Wave A1).
  - `internal/cli/record_collision*.go` (Wave B).
  - `internal/workflow/reconcile.go` lines ~196-236 (Wave D phase-1.5) and ~560-700 (Wave A2 lock guard).
  - `internal/workflow/patch_id_detector*.go` (Wave D).
  - `Config.PatchIDDetectorEnabled` default — `false` (deferral documented in ADR-022).
  - `Manifest.Kind == "hotfix"` auto-drop default — deferred (ADR-023).
  - ADR-019 trailer schema, ADR-020 inline-minimal skill-doc policy, ADR-021 carve-out scope.
- **Wave A1+A2 are CROSS-COMMIT BOUND** (`1d6179c` ↔ `8fc2e4e`); revert as a unit if needed.
- The `TestSkillDocReferencesAreSelfContained` parity guard pins the inline-minimal policy; do NOT reintroduce any repo-relative `docs/*.md` reference in `assets/`. URL-prefixed forms (`https://`, `http://`, `file://`) are allowed.
- v0.8.1 `--check-applied-only` exit-success is now **phase-1.5-only** under `CheckAppliedOnly` (`internal/workflow/reconcile_check_applied.go`); phase-1 reverse-apply is diagnostic-only because that command skips the normal reconcile preflight. The normal reconcile pipeline (`internal/workflow/reconcile.go`) is unchanged — its preflight still legitimizes phase-1 as upstream-merged evidence in that context.
- The `## Side Research — State-of-the-art middle pass (2026-05-10)` section below is preserved verbatim across handoff resets; it is living research notes.

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
