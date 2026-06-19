# Current Handoff

## Active Task

- **Task ID**: `wp003-wave-beta-prd2-prd3-prd7-impl-rev1`
- **Milestone**: WP-003 Wave β rev-1 (PRDs 2 `upstreamed-confirmation-gate`, 3 `reconcile-revision-pass-log`, 7 `reconcile-hunk-overlap-detector`).
- **Description**: Rev-1 fix-pass over rev-0 (HEAD `d8774a7`). Three independent reviews returned NEEDS REVISION. Implementer must close 7 consolidated findings (F1–F7 below) without reopening any ADR-025 D1–D13 schema lock, ADR-024 capture/metadata non-drift, or D10 privacy invariants. No new lifecycle states. No new config flags. Keep `Outcome`, `ReviewVerdict`, and persisted schema additions strictly within ADR-025 D8.
- **Status**: In Progress (rev-1).
- **Assigned**: 2026-05-30.

## Rev-1 findings (binding scope — close all 7)

**F1 (HIGH BLOCKING)** — PRD 2 §6.1 display contract. `internal/cli/cobra.go:1868` prints `result.Outcome` directly. When `ReviewVerdict == "rejected-upstreamed"` (set at `internal/workflow/reconcile.go:825`), human output MUST render `[upstreamed-candidate]` (or PRD-exact phrasing) instead of `[blocked]`. JSON output may keep `outcome=blocked` + `review_verdict=rejected-upstreamed` (operators reconstruct from those two fields). Add CLI test asserting `[upstreamed-candidate]` appears in human output for the rejected-gate case AND that JSON keeps both fields intact (byte-identity template at `reconcile_evidence_integration_test.go:513-529`).

**F2 (HIGH BLOCKING)** — PRD 3 §5 (lines 159-161) corrupt_entries contract on `tpatch reconcile review list` surface. Current `internal/cli/cobra.go` (review list path) calls strict `LoadReconcileRevisions` at `internal/store/reconcile_revision.go:27-43` which aborts on first malformed line. PRD verbatim: "A bad JSONL line in the middle of the file is reported with line number. Human output skips unreadable trailing summaries; `--json` returns a structured `corrupt_entries` array and exits non-zero." Required:
  - Add a lenient loader (e.g. `LoadReconcileRevisionsLenient`) that returns `(valid []ReconcileRevision, corrupt []CorruptEntry{Line int, Error string}, err error)` — does NOT abort on first malformed line, accumulates corrupt-line metadata, returns ALL valid entries (before AND after the corrupt line).
  - Update `review list` CLI path to use the lenient loader.
  - Human output: print valid entries, then a `corrupted entries: line N: <error>` summary; skip unreadable trailing summaries.
  - JSON output: emit `{ "revisions": [...], "corrupt_entries": [{"line": N, "error": "..."}] }` envelope and exit non-zero when corrupt_entries is non-empty.
  - Keep strict `AppendReconcileRevision` writer semantics unchanged (writer refuses on malformed pre-existing file — that's a separate concern).
  - Tests: malformed JSONL with valid entries on both sides → list returns ALL valid entries + structured corrupt_entries + non-zero exit (JSON mode).

**F3 (MEDIUM)** — PRD 3 privacy test re-seed. `internal/store/reconcile_revision_test.go:54-71` (`TestReconcileRevisionPrivacyNoSourceLeak`) and `internal/workflow/reconcile_evidence_integration_test.go:200,450` seed secrets into file CONTENT, but D10 privacy is about persisted-artifact content. Re-seed plausible secret-leak vectors into feature title, slug, and path metadata (mirror gate test at `:245`). Assert revision JSONL + evidence artifact do not contain the seeded secret string from any of those vectors.

**F4 (MEDIUM)** — PRD 7 §6.5: hunk-overlap evidence default `nearby-window=3` (encoded at `internal/workflow/hunk_overlap.go:117`) must be asserted in marshaled JSON output. Extend a hunk-overlap test in `reconcile_evidence_integration_test.go` and/or `internal/cli/reconcile_evidence_cli_test.go` to assert the string `nearby-window=3` (or canonical encoding) appears in evidence-line JSON for the default-window case.

**F5 (LOW)** — PRD 2 §6.5 backward-compat. Mirror Wave α carry-forward template `TestStatusLoadsWhenEvidenceArtifactAbsent` (`internal/cli/reconcile_evidence_carryforward_test.go`). Add tests asserting: (a) reading a `ReconcileSummary` with empty `ReviewVerdict` works; (b) reconcile run with no pre-existing `reconcile-evidence.jsonl` and no `reconcile-revisions.jsonl` succeeds and creates files lazily.

**F6 (MEDIUM)** — PRD 2 §6.2 state non-mutation. Extend `TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch` (`reconcile_evidence_integration_test.go:435`): after reconcile, reload `status.json` from disk (not via in-memory `result`), assert persisted `State` is NOT `upstream_merged`. The workflow code at `internal/workflow/reconcile.go:825` does `finalState = StateBlocked`, but no disk-reload assertion exists.

**F7 (LOW)** — PRD 2 §6.3 revision-log linkage. Extend `TestUpstreamedConfirmationGateKeepsConfirmedReverseApply` (or analogous): load `reconcile-revisions.jsonl` from disk, assert the persisted revision entry includes the evidence attempt ID (non-empty) AND the upstream commit ref. Linkage logic lives at `internal/workflow/reconcile.go:827-833` and `:810-846` (`persistRevisionPassLog`).

## Carry-forward dispatch rules (binding for rev-1 brief)

11. Distinguish "behavior implemented" from "behavior tested". Reviewers MUST read the production code path first ("does this acceptance criterion actually have a code path?"), THEN check tests. F8/F2-user lesson: my supervisor-external accepted F4 as a test gap; user-external read the production code and discovered there's no `corrupt_entries` envelope at all. Same PRD line, different severity.
12. PRD §6 lines like "displayed as X" or "appears in JSON output" or "returns a structured X array and exits non-zero" are binding test contracts AND production-behavior contracts. Brief them as both.
13. Privacy tests MUST seed secrets into plausible exfiltration vectors (title, slug, path metadata) — NOT just file content.
14. State-mutation contracts MUST be verified by reloading from disk (`store.LoadStatus`), not by checking runtime fields on the `result` value.
15. ReconcileSummary persisted schema is governed by ADR-025 D8. Brief should say "no persisted-schema additions outside what ADR-025 explicitly authorizes" — NOT "schema LOCKED" (rev-0 wording was over-broad; D8 already authorized `ReviewVerdict`).

## Prior Wave α reference

- Writer: `internal/store/reconcile_evidence.go`, `internal/workflow/file_novelty.go`, `internal/workflow/reconcile.go` persistence helpers (`persistReconcileEvidence`, `persistFileNoveltyEvidence`, `warnReconcileEvidenceAppendError`).
- Reader: `ReconcileResult.Evidence` inline field; `evidenceArtifactRef` in `internal/cli/cobra.go:1701-1707`; status JSON `evidence_artifact` runtime field; human `evidence:` hint at `cobra.go:1849-1851`; deduplication at `:1714-1726`.
- Test patterns: `internal/workflow/reconcile_evidence_integration_test.go`, `internal/cli/reconcile_evidence_cli_test.go`, `internal/cli/reconcile_evidence_carryforward_test.go`. Reuse `cliEvidenceFixture` harness.

## Wave β scope (binding for implementer)

Read PRDs in order:
1. `docs/prds/PRD-upstreamed-confirmation-gate.md` — PRD 2. Adds a confirmation gate before issuing `upstreamed` verdict; uses evidence artifact (Wave α surface).
2. `docs/prds/PRD-reconcile-revision-pass-log.md` — PRD 3. Adds per-attempt revision log via the evidence schema (uses ADR-025 D3-D5 revision shape).
3. `docs/prds/PRD-reconcile-hunk-overlap-detector.md` — PRD 7. Hunk-overlap detector — depends on PRD 6 file-novelty (Wave α).

ADR-025 binding contracts (no drift):
- D1–D5: evidence schema.
- D6–D9: revision shape and attempt-id semantics.
- D10: privacy (no source bodies/transcripts/prompts/vectors).
- D11: malformed-artifact handling.
- D12–D13: byte-identity contracts.

Cross-cluster: ADR-024 capture/metadata is binding. No drift in `patch-generations.json` schema vs `reconcile-evidence.jsonl`.

## Carry-forward dispatch rules (do not strip from briefs)

1. Briefs MUST reference PRD acceptance criteria verbatim. No escape hatches like "defer to next wave" or "if integration is risky, skip" — escape hatches cause regressions (rev-0 Wave α F1 root cause).
2. Briefs MUST enumerate any policy-ADR opt-out contracts in scope.
3. External reviewer briefs MUST sweep ALL PRD §6 acceptance criteria, not just the rev's stated findings (rev-1 Wave α F3 root cause).
4. Internal reviewer checklist MUST include flag-off counter-scenarios for any new enforcement.
5. `gofmt -l .` MUST be run directly, never piped (returns exit 1 on empty input through grep).
6. Two-opinion external review protocol (supervisor + user-parallel) confirmed for every rev — caught real regressions in EVERY Wave α revision.

## Session Summary

Wave β rev-0 implemented in three commits:

1. `e45ccdc` — added ADR-025 `reconcile-revisions.jsonl` store schema, strict reader/writer, deterministic `rr_<12hex>` IDs, malformed sentinel, and `ReconcileSummary.review_verdict`.
2. `34b2bba` — added workflow confirmation gate, revision-pass persistence for gate decisions, hunk-overlap detector after file-novelty, and workflow tests for PRDs 2/3/7.
3. `1e99a9f` — added `tpatch reconcile --format json`, human evidence hints for gate/hunk overlap, `tpatch reconcile review add/list`, and CLI tests.

## Current State

- PRD 2: upstreamed candidates now pass through a confirmation gate. High-confidence reverse-apply / patch-id evidence stays `upstreamed` with `review_verdict=confirmed-upstreamed`; unconfirmed operation/provider candidates downgrade to `blocked` with recorded gate evidence and revision entry.
- PRD 3: `reconcile-revisions.jsonl` is append-only, deterministic, strict on malformed input, and surfaced through `ReconcileResult.Revisions` plus `tpatch reconcile review add/list`.
- PRD 7: modified/mixed-additive files run a deterministic line-range hunk-overlap pass after file-novelty. Evidence uses existing ADR-025 fields (`evidence_kind=hunk-overlap`, classification in `reason_code`, sanitized hunk IDs in `matched_operations`).
- No `schema_version` bump. No `FeatureState` additions. No config flags. No `patch-generations.json` / ADR-024 changes.

## Files Changed

- `internal/store/types.go`
- `internal/store/reconcile_revision.go`
- `internal/store/reconcile_revision_test.go`
- `internal/workflow/reconcile.go`
- `internal/workflow/hunk_overlap.go`
- `internal/workflow/reconcile_evidence_integration_test.go`
- `internal/workflow/reconcile_test.go`
- `internal/cli/cobra.go`
- `internal/cli/reconcile_evidence_cli_test.go`

## Test Results

Validation gates (all green, run directly):

- `gofmt -l .` — clean (empty output).
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean.
- `go test ./...` — green across all packages.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

New / updated targeted coverage:

- PRD 2: `TestUpstreamedConfirmationGateKeepsConfirmedReverseApply`, `TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch`, updated `TestReconcilePhase3_ProviderAssistedUpstreamed`, `TestReconcileJSONSurfacesConfirmationGateAndRevision`.
- PRD 3: `TestReconcileRevisionRoundTripAndStableID`, `TestReconcileRevisionMalformedLineAndWriterRefusal`, `TestReconcileRevisionPrivacyNoSourceLeak`, `TestRevisionPassLogAppendedForConfirmationGate`, `TestReconcileReviewAddListJSON`.
- PRD 7: `TestHunkOverlapEvidenceForModifiedPath`, `TestHunkOverlapSkippedForAllNewFiles`, `TestReconcileHumanOutputHunkOverlapHint`.
- Byte/privacy/carry-forward: `TestReconcileResultJSONOmitsWaveBetaFieldsWhenNoGateRevisionOrOverlap` plus existing Wave α absent/malformed evidence status tests remain green.

## Next Steps

1. Wave β internal review pending.

## Blockers

None.

## Context for Next Agent

- Confirmation gate evidence is encoded as `evidence_kind=manual-review` with `matched_operations=["confirmation-gate"]` and reason codes `confirmed-upstreamed` / `missing-upstream-commit-ref`. This intentionally avoids adding a new `confirmation-gate` evidence kind because ADR-025 D4 closes the v1 evidence-kind enum.
- Hunk-overlap evidence does not add new JSONL fields; it preserves ADR-025 D2 by storing classification in `reason_code`, paths in `matched_paths`, the default window as `nearby-window=3`, and hunk range IDs in `matched_operations` (no source bodies).
- Revision entries are written for confirmation-gate decisions and by `tpatch reconcile review add`. General non-gate reconcile attempts do not emit revision entries, preserving `omitempty` byte-identity for no-review/no-gate scenarios.
- No PRD acceptance criteria intentionally deferred.

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
