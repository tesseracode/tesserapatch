# Current Handoff

## Active Task

- **Task ID**: `wp003-wave-alpha-prd1-prd6-impl-rev1`
- **Milestone**: WP-003 — Reconcile safety & middle-pass (T56 cluster), Wave α revision 1
- **Description**: Address two NEEDS REVISION findings from concurrent external reviews. F1: wire `ClassifyFileNovelty` into the production reconcile path so PRD 6 §6.1/§6.3 are actually met. F2: stop swallowing `AppendReconcileEvidence` errors at `reconcile.go`; surface a warning when evidence writing fails, while preserving verdict semantics (PRD 1 §6.6).
- **Status**: In Progress (rev-1 implementer dispatched).
- **Assigned**: 2026-05-26.
- **Prior rev-0**: Internal APPROVED but both externals NEEDS REVISION (commits `d265a08..d6878a4`). Root cause: rev-0 dispatch brief contained a loose escape hatch ("if integration is risky, defer to Wave β") that overrode PRD 6 acceptance. Carry-forward: dispatch briefs must reference PRD acceptance criteria verbatim and forbid implementer-side deferral.

## Session Summary

Wave α implementation is complete and validated. PRD 1 added strict, deterministic `reconcile-evidence.jsonl` storage with `re_<12hex>` content-addressed attempts, ADR-025 enum validation, malformed sentinel handling, atomic append, refs support, patch-id helper, and required privacy/determinism tests. PRD 6 added a patch-header file-novelty classifier, evidence helper, and boundary tests. A reconcile hook now writes evidence when reconcile writes a verdict, without changing verdict semantics.

Commits landed from `d265a08..HEAD`:

1. `76530a0` — Add reconcile evidence store
2. `a7da04f` — Test reconcile evidence store
3. `31f4d89` — Add file novelty classifier
4. `871a703` — Test file novelty classifier
5. `ccbc217` — Write reconcile evidence during reconcile
6. (this commit) — Update WP-003 wave alpha handoff

## Current State

- ADR-025 D1–D13 implemented for Wave α scope with no intentional deviations.
- ADR-024 sibling patterns preserved: `re_<12hex>` content IDs, `git-patch-id-stable`, strict malformed sentinel, artifact under `.tpatch/features/<slug>/artifacts/`.
- No new `FeatureState` lifecycle states added.
- No new config flag or evidence-write opt-out added.
- File-novelty reconcile integration choice: evidence write hook landed in Wave α; classifier verdict semantics are not used to change outcomes.
- Pre-existing `docs/state-of-the-art/` working-tree modifications remain untouched and uncommitted by this task.

## Files Changed

- `internal/store/reconcile_evidence.go`
- `internal/store/reconcile_evidence_test.go`
- `internal/workflow/file_novelty.go`
- `internal/workflow/file_novelty_test.go`
- `internal/workflow/reconcile.go`
- `docs/handoff/CURRENT.md`

## Test Results

- `gofmt -l .` — no output (clean).
- `go build ./cmd/tpatch` — passed.
- `go test ./...` — passed; summary included `ok github.com/tesseracode/tesserapatch/internal/store (cached)`, `ok github.com/tesseracode/tesserapatch/internal/workflow (cached)`, and all other packages passed.
- `go vet ./...` — passed with no output.
- Privacy assertion: `TestReconcileEvidencePrivacyNoSourceLeak` passes and asserts `SECRET_SOURCE_BODY_DO_NOT_LEAK` is absent from `reconcile-evidence.jsonl`.
- Side Research md5 invariant preserved: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Reviewer should run the ADR-025 D1–D13 schema-drift checklist.
2. Reviewer should spot-check `reconcile-evidence.jsonl` strict reader/writer behavior against ADR-024 malformed-manifest precedent.
3. Wave β can consume file-novelty evidence for confirmation/hunk-overlap logic; no blocker from Wave α.

## Blockers

None.

## Context for Next Agent

- Evidence storage intentionally uses ADR-025's required/optional field set only. File-novelty details are represented through `evidence_kind=file-novelty`, sorted `matched_paths`, `reason_code=<classification>`, confidence, and pre-reconcile presence rather than adding non-ADR top-level fields.
- Reconcile evidence append errors are swallowed in the existing `saveReconcileArtifacts` void-return path to avoid changing reconcile verdict semantics; malformed artifacts still refuse appends through `AppendReconcileEvidence` and are covered by tests.
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
