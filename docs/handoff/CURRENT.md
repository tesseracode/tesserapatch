# Current Handoff

## Active Task

- **Task ID**: `wp003-wave-alpha-prd1-prd6-impl-rev1`
- **Milestone**: WP-003 — Reconcile safety & middle-pass (T56 cluster), Wave α revision 1
- **Description**: Address two NEEDS REVISION findings from concurrent external reviews. F1: wire `ClassifyFileNovelty` into the production reconcile path so PRD 6 §6.1/§6.3 are actually met. F2: stop swallowing `AppendReconcileEvidence` errors at `reconcile.go`; surface a warning when evidence writing fails, while preserving verdict semantics (PRD 1 §6.6).
- **Status**: Review (awaiting reviewer).
- **Assigned**: 2026-05-26.
- **Prior rev-0**: Internal APPROVED but both externals NEEDS REVISION (commits `d265a08..d6878a4`). Root cause: rev-0 dispatch brief contained a loose escape hatch ("if integration is risky, defer to Wave β") that overrode PRD 6 acceptance. Carry-forward: dispatch briefs must reference PRD acceptance criteria verbatim and forbid implementer-side deferral.

## Session Summary

Rev-1 implementation is complete, committed, and validated. Both external findings are addressed surgically without changing ADR-025 schema/enums, lifecycle states, `ReconcileSummary`, config flags, or verdict semantics.

Rev-1 commits from `4fa1394..HEAD`:

1. `a1dcaf2` — Address reconcile evidence review findings
2. (this commit) — Update WP-003 Wave α rev-1 handoff for re-review

Finding resolution:

- **F1 fixed**: `saveReconcileArtifacts` now calls file-novelty evidence persistence after the phase evidence write (`internal/workflow/reconcile.go:518`, `internal/workflow/reconcile.go:560-563`). `persistFileNoveltyEvidence` reads canonical `post-apply.patch`, requires upstream/base commit anchors, calls `ClassifyFileNovelty`, builds `FileNoveltyEvidence`, recomputes `ComputeAttemptID`, and appends an independent `evidence_kind: "file-novelty"` JSONL line (`internal/workflow/reconcile.go:668-692`). Integration tests read `reconcile-evidence.jsonl` from disk and assert `mixed-additive` plus `all-new-files` classifications (`internal/workflow/reconcile_evidence_integration_test.go:18`, `internal/workflow/reconcile_evidence_integration_test.go:41`).
- **F2 fixed**: phase evidence append errors are captured (`internal/workflow/reconcile.go:663-665`), file-novelty append errors are captured (`internal/workflow/reconcile.go:690-692`), and `warnReconcileEvidenceAppendError` emits an explicit malformed-artifact warning mentioning the slug when `errors.Is(err, store.ErrMalformedEvidence)` (`internal/workflow/reconcile.go:695-701`). `TestReconcileWarnsOnMalformedEvidenceArtifact` verifies non-error verdict semantics, warning emission, slug mention, and writer refusal preserving the malformed file (`internal/workflow/reconcile_evidence_integration_test.go:64`).

## Current State

- ADR-025 D1–D13 schema/enum surface is unchanged.
- No new `FeatureState` lifecycle states added.
- No `ReviewVerdict` field added to `ReconcileSummary`.
- No new config flag or evidence-write opt-out added.
- File-novelty evidence remains diagnostic only and does not change reconcile verdict semantics.
- Evidence warnings are privacy-safe: warning text includes the slug/error class only, not source bodies, prompts, transcripts, vectors, or embeddings.
- Pre-existing `docs/state-of-the-art/` working-tree modifications remain untouched and uncommitted by this task.

## Files Changed

- `internal/workflow/reconcile.go`
- `internal/workflow/reconcile_evidence_integration_test.go`
- `docs/handoff/CURRENT.md`

## Test Results

Required validation run directly:

- `gofmt -l .` — no output (clean).
- `go vet ./...` — no output (passed).
- `go build ./cmd/tpatch` — passed.
- `go test ./...` — passed; captured summary/final line: `ok  github.com/tesseracode/tesserapatch/tests/integration  (cached)`.
- Targeted integration tests also passed: `go test ./internal/workflow -run 'TestReconcileWritesFileNoveltyEvidence|TestReconcileWarnsOnMalformedEvidenceArtifact'`.
- Side Research md5 invariant preserved: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Reviewer should re-run the required validation commands and inspect `internal/workflow/reconcile.go` for F1/F2 closure.
2. Reviewer should confirm file-novelty evidence remains a separate JSONL entry and does not alter reconcile outcome semantics.
3. Supervisor can push after review approval; this implementer did not push.

## Blockers

None.

## Context for Next Agent

- `persistFileNoveltyEvidence` intentionally skips silently when canonical patch or commit anchors are unavailable because PRD 6 file novelty is diagnostic evidence, not a verdict gate.
- The new warning indirection is intentionally minimal and test-only swappable; production default writes to stderr, matching existing reconcile warning style.
- Pre-existing `docs/state-of-the-art/` modifications are not part of Wave α.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`. Always verify after editing CURRENT.md.

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
