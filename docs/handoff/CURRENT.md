# Current Handoff

## Active Task

- **Task ID**: `wave-gamma-patch-amend-impl`
- **Milestone**: v0.10.0 Wave γ — `PRD-feature-patch-amend` implementation (WP-002 cluster slice 4 of 4).
- **Description**: Implement `tpatch feature patch refresh` and `tpatch feature patch fixup` per the binding contract in `docs/adrs/ADR-026-patch-amendment-policy.md` (D1–D10 + the new "Wave γ Implementation Contract" appendix IC1–IC6). This is the implementation phase of Wave γ; ADR-026 is APPROVED both internally and externally.
- **Status**: Ready to dispatch.
- **Assigned**: 2026-05-22.

### Binding contract

- `docs/adrs/ADR-026-patch-amendment-policy.md` — D1–D10 are not re-openable. The Wave γ Implementation Contract appendix (IC1–IC6) is also binding.
- `docs/prds/PRD-feature-patch-amend.md` — product surface and acceptance criteria.
- `docs/adrs/ADR-024-patch-generation-manifest-boundary.md` — Wave β manifest contract (D1–D9). Frozen for Wave γ except for the D10 transition (`amend-refresh`/`amend-fixup` become writable).
- `docs/adrs/ADR-013-verify-freshness-overlay.md` — verify-cache invalidation inputs (cited by ADR-026 D6).
- `docs/adrs/ADR-011-feature-dependencies.md` — hard/soft dependency policy (cited by ADR-026 D5).

### Landing order (binding, per IC1)

Wave γ MUST land as a sequence of commits in this exact order. Each step ships with its own tests before the next step starts:

1. **Schema-extension commit** — Extend `PatchGeneration` with `reason` (D2) and `fixup_of_generation` (D4). Register both as known fields in the strict v1 reader. Add `internal/store/patch_generations_wavegamma_test.go` (IC2) covering all five tripwire assertions. **NO writer changes.** Existing manifests still load byte-identically.
2. **Kind-enum commit** — Flip `amend-refresh` and `amend-fixup` from reserved to writable in the kind enum (D10). Add `ClassifyPlainRecordKind` (IC3) with its table-driven test. **NO CLI surface.**
3. **CLI surface commit(s)** — Add `tpatch feature patch refresh` and `tpatch feature patch fixup` subverbs (D7). Wire `parent-generation-stale` overlay surface (D5) into `status`/`status --json`. Wire verify-freshness invalidation per D6. Update shipped skill assets per IC5 so the parity guard passes.

The reviewer checklist will verify the commit sequence matches IC1. Out-of-order landing is grounds for rejection on its own.

### Frozen regions (per IC4)

The following surfaces are frozen for Wave γ. The implementer MUST NOT edit them outside the explicit extension points listed in the landing order:

- `internal/store/patch_generations.go` — manifest v1 schema. Wave γ extends `PatchGeneration` per IC1 step 1 but MUST NOT bump the `version` constant, relax `DisallowUnknownFields`, or alter the `ErrMalformedManifest` classification path (`internal/store/patch_generations.go:24-28` + `:101-104`).
- `internal/store/claims.go` — Wave α file-claims reader/writer at `LoadClaims` (`internal/store/claims.go:263`) and `SaveClaims` (`internal/store/claims.go:294`). Not on the amendment path. No edits.
- `internal/cli/cobra.go:897-905` (`--force-amend` orphan-detection branch) and `internal/cli/cobra.go:1415` (`--force-amend` flag registration). D8 binds: no behavior change.
- `internal/gitutil/capture_modes.go` — Wave α capture entry points at `CaptureStagedPatch` (`:137`), `CaptureUnstagedPatch` (`:182`), and `ValidateStagedPatch` (`:328`). No edits.
- `internal/workflow/patch_generations.go:31` — Wave β rev-2 narrow swallow keyed on `store.ErrMalformedManifest`. No edits.

### Out of scope for Wave γ

- `fork` / `fold` subverbs (PRD §4.4–4.5; deferred to v2 per ADR-026 D7).
- Metadata-only amend manifest revisions (ADR-026 D9 answers NO for v1).
- WP-003 reconcile cluster work (ADR-025 reserved-but-unwritten).
- Privacy gating on `--reason` (deferred to `ADR-capture-context-privacy-boundary`; ADR-026 D2 is advisory-only).
- Schema version bump (D4 keeps v1).

### Quality gates

Before claiming completion, the implementer MUST:

1. Run `gofmt -l .` and confirm zero output.
2. Run `go vet ./...` and confirm clean.
3. Run `go build ./cmd/tpatch` and confirm success.
4. Run `go test ./... -count=1 -race` and confirm all green.
5. Run `go test ./assets/...` and confirm parity guard passes (IC5).
6. Confirm the Wave γ contract test (`patch_generations_wavegamma_test.go`) exists and passes.
7. Confirm Wave β fixtures load byte-identically (IC6 no-migration assertion).
8. Update `docs/handoff/CURRENT.md` Session Summary block with: commit shas per IC1 step, files changed, test counts, and explicit confirmation that IC1–IC6 are satisfied.

### Reviewer checklist additions (per IC6)

In addition to the standard `AGENTS.md` checklist, the reviewer MUST verify:

- [ ] Commit sequence matches IC1 (schema → enum → CLI).
- [ ] `patch_generations_wavegamma_test.go` exists and covers all five IC2 assertions.
- [ ] `ClassifyPlainRecordKind` (or equivalent named helper) is the only call site classifying `record` vs `amend-refresh` for plain `record <slug>` writes.
- [ ] IC4 frozen regions are unedited outside the IC1-listed extension points.
- [ ] `go test ./assets/...` passes (IC5).
- [ ] Existing Wave β manifests on disk still load byte-identically — no migration required (IC6).
- [ ] Side Research md5 invariant preserved: `b385fe622db9926f48861105239f113e`.

### Drift-mitigation rationale (background, not binding)

The external reviewer for ADR-026 flagged that the principal Wave γ risk is reader/writer drift on the strict v1 manifest. The IC1–IC6 appendix exists specifically to convert that implicit risk into reviewable artifacts:

- IC1 (landing order) forces the strict reader to know `reason` and `fixup_of_generation` before any writer emits them.
- IC2 (golden fixture tripwire) breaks first on any reader/writer skew.
- IC3 (single-source classifier) prevents silent corruption of the D1 hybrid-kind audit trail.
- IC4 (frozen regions) names the Wave α + Wave β surfaces that are NOT on the amendment path.
- IC5 (skill-asset parity) catches doc-skew at build time.
- IC6 (reviewer checklist additions) operationalizes IC1–IC5 at review time.

## Session Summary

ADR-026 internal + external review cycle complete (commit `b40b042` records the external APPROVED verdict). Drafted "Wave γ Implementation Contract" appendix (IC1–IC6) on top of the approved ADR to convert the reviewer's residual drift risk into binding, reviewable artifacts. Prepared this Wave γ kickoff brief embedding IC1–IC6 as the implementer's binding contract.

## Current State

- v0.10.0 cluster: Wave α shipped (v0.9.0 at `9267026`), Wave β complete (on `main`, no release tag yet), Wave γ ready to dispatch with full ADR + IC contract in place.
- ADR-026 status: Accepted, APPROVED internally and externally; appendix IC1–IC6 added post-approval as binding implementation guardrails (no D1–D10 changes).

## Files Changed

- `docs/adrs/ADR-026-patch-amendment-policy.md` — appended "Wave γ Implementation Contract" appendix (IC1–IC6) between `## Consequences` and `## References`.
- `docs/handoff/CURRENT.md` — this rewrite (Wave γ kickoff brief).

## Test Results

Docs-only edits; baseline holds at 612 `func Test...` declarations across ten packages green under `go test ./... -count=1 -race`.

## Next Steps

1. Commit ADR-026 appendix + CURRENT.md rewrite.
2. Push to origin.
3. Dispatch Wave γ implementer with reference to this CURRENT.md (binding contract embedded).
4. After implementer lands all three IC1 commits, dispatch sub-agent reviewer (using IC6 checklist additions).
5. On internal APPROVED, push and surface for external review.

## Blockers

None.

## Context for Next Agent

- ADR-026 D1–D10 are immutable; IC1–IC6 are the binding implementation guardrails layered on top.
- The reviewer's "Wave γ contract test" (IC2) is the single most important tripwire — it MUST land in the schema-extension commit, not later.
- IC3's `ClassifyPlainRecordKind` is the single source of truth for D1's hybrid kind rule. Multiple call sites classifying `record` vs `amend-refresh` is a review-blocker.
- IC4 frozen regions include the Wave β rev-2 `ErrMalformedManifest` sentinel; do not touch the classification path or the narrow swallow.
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
