# Current Handoff

## Active Task

- **Task ID**: `wp003-wave-alpha-prd1-prd6-impl-rev2`
- **Milestone**: WP-003 — Reconcile safety & middle-pass (T56 cluster), Wave α revision 2
- **Description**: Address rev-1 external NEEDS REVISION finding F3 by adding the reader-side CLI surface for reconcile evidence. PRD 1 §4 now has human evidence hints and JSON evidence exposure; PRD 6 §6.3 file-novelty evidence is available in JSON output.
- **Status**: Review (awaiting reviewer).
- **Assigned**: 2026-05-26.
- **Prior rev-1**: Internal APPROVED + supervisor-external APPROVED, but user's parallel external NEEDS REVISION on F3 (commits `4fa1394..7c72323`). F1 (production novelty integration) and F2 (malformed-evidence warning) remain fixed and independently verified.

## Session Summary

Rev-2 completed the reader-side evidence surface with Option A (inline evidence bundle):

- `2c20450` — Added `ReconcileResult.Evidence []store.ReconcileEvidence` with `omitempty`; populated it from the evidence entries successfully appended during `saveReconcileArtifacts`; rendered PRD-aligned human `evidence:` hints after each reconcile verdict line; added a production `store.LoadReconcileEvidence` reader in `tpatch status --json` to emit `evidence_artifact` when evidence exists.
- `d4411d2` — Added workflow + CLI tests for JSON evidence exposure, human evidence hints, empty-case `omitempty`, D10 privacy, and status JSON artifact exposure.

## Current State

- Writer surface: complete and validated (rev-0 + rev-1).
- Reader surface: implemented and validated in rev-2.
- Option A chosen: reconcile JSON surfaces the latest evidence bundle inline via `ReconcileResult.Evidence`, limited to entries successfully appended during this reconcile invocation.
- `tpatch status --json` additionally surfaces an `evidence_artifact` repo-relative reference when `reconcile-evidence.jsonl` exists and loads successfully.
- ADR-025 D1–D13 schema/enums unchanged; no new evidence kinds, phases, lifecycle states, config flags, or evidence write opt-outs.
- Verdict semantics unchanged; evidence remains diagnostic only.
- `status.json` / `ReconcileSummary` persisted schema unchanged.

## Files Changed

- `internal/workflow/reconcile.go`
- `internal/workflow/reconcile_evidence_integration_test.go`
- `internal/cli/cobra.go`
- `internal/cli/reconcile_evidence_cli_test.go`
- `docs/handoff/CURRENT.md`

## Test Results

Targeted tests added/updated (6/6 pass):

- `TestReconcileResultJSONExposesEvidence` — pass.
- `TestReconcileResultJSONOmitsEvidenceWhenNoArtifactWritten` — pass.
- `TestReconcileEvidenceReaderOutputPrivacyNoSourceLeak` — pass.
- `TestReconcileHumanOutputEvidenceHint` — pass.
- `TestStatusJSONIncludesEvidenceArtifact` — pass.
- `TestReconcileCLIEvidenceOutputPrivacyNoSourceLeak` — pass.

Validation gates:

- `gofmt -l .` — clean (empty output).
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean.
- `go test ./...` — green across all packages.
- Side Research md5 invariant — `b385fe622db9926f48861105239f113e`.

## Next Steps

1. rev-2 internal review pending.

## Blockers

None.

## Context for Next Agent

- Option A was implemented: inline `ReconcileResult.Evidence` is the primary reconcile JSON reader surface. The field is `omitempty`, preserving byte identity when no evidence is written.
- No scope deferrals: status JSON exposure was also wired via a production `store.LoadReconcileEvidence` reader and emits only `evidence_artifact`, not inline history.
- Human reconcile output deduplicates hints by rendered text and uses PRD-aligned lines such as `evidence: phase-4 forward-apply` and `evidence: file-novelty mixed-additive`.
- D10 privacy assertions cover both human and JSON reader outputs; no source bodies, prompts, transcripts, vectors, or embeddings are surfaced.
- Do NOT modify `docs/supervisor/LOG.md`; reviewer owns the next log entry.

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
