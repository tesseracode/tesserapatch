# Current Handoff

## Active Task

- **Task ID**: (none — between tasks)
- **Milestone**: WP-003 Wave β eligible (PRDs 2 `upstreamed-confirmation-gate`, 3 `reconcile-revision-pass-log`, 7 `reconcile-hunk-overlap-detector`). Wave α (PRDs 1 + 6) shipped 2026-05-26.
- **Description**: Awaiting supervisor selection of next task. Candidates:
  1. **WP-003 Wave β kickoff** (PRDs 2 + 3 + 7) — Wave α prerequisite is satisfied; ADR-025 already covers β PRDs.
  2. **Wave α test-coverage carry-forwards** (F1/F2/F3 from rev-2 supervisor-external) — small focused hotfix to add absent-evidence load test, malformed-evidence load test, and stronger privacy-vector tests.
  3. Other backlog item per `docs/CLUSTERS.md`.
- **Status**: Not Started.

## Session Summary

WP-003 Wave α (PRDs 1 + 6) shipped at HEAD `a5faf91`:

- **rev-0** (`d265a08..d6878a4`) — writer side: evidence store, file-novelty classifier, reconcile persistence hook. Helper tests passed but reader/integration shipped weak.
- **rev-1** (`4fa1394..7c72323`) — F1 (production novelty integration) + F2 (malformed-evidence warning) fixed after both externals returned NEEDS REVISION on rev-0.
- **rev-2** (`6a8deba..8d4665f`) — reader-side CLI surface (Option A inline `Evidence` field on `ReconcileResult`, `evidence_artifact` reference on status JSON, human `evidence:` hint) after user's parallel external caught F3 reader-side gap that supervisor's rev-1 external missed.
- **Close-out** (`a5faf91`) — internal APPROVED + both externals' verdicts (mine NEEDS REVISION → APPROVED WITH NOTES, user's APPROVED) recorded.

Final supervisor decision: APPROVED WITH NOTES — Wave α ships; three test-coverage findings carried forward as future cleanup (production code is correct by design, tests don't prove all PRD §6 criteria).

## Current State

- Wave α PRDs 1 + 6: shipped.
- Wave β prerequisite cleared.
- ADR-025 D1–D13 schema lock preserved through all 3 revs.
- D10 privacy preserved.

## Carry-forward test-coverage items (from rev-2 supervisor-external)

These are NOT rev-3 blockers. They are tracked here so a future hotfix or Wave β prep can pick them up:

- **F1 (HIGH)** — Add test: feature with `ReconcileSummary` in `status.json` but NO `reconcile-evidence.jsonl` → `LoadFeatureStatus` succeeds + `tpatch status --json` works. Verifies PRD 1 §6 lines 208-209.
- **F2 (MEDIUM)** — Extend `TestReconcileWarnsOnMalformedEvidenceArtifact` (or add sibling) to call `LoadFeatureStatus` + `tpatch status --json` after seeding malformed artifact. Verifies PRD 1 §6 lines 211-212.
- **F3 (LOW)** — Strengthen privacy tests by seeding secrets into plausible leak vectors (file path containing secret string; feature title with embedded secret); assert evidence fields don't surface them. Current tests seed secrets into file content, which is not a plausible leak vector.

Production code is correct (`store.go:345` `LoadFeatureStatus` does not depend on `LoadReconcileEvidence`; `cobra.go:1701-1707` `evidenceArtifactRef` returns empty string on error). Tests just don't prove it.

## Files Changed

(this handoff transition only — see `docs/handoff/HISTORY.md` for Wave α file deltas.)

- `docs/supervisor/LOG.md` — both rev-2 external verdicts + supervisor decision recorded.
- `docs/CLUSTERS.md` — PRDs 1 + 6 marked Shipped; WP-003 status flipped to Wave α SHIPPED.
- `docs/handoff/HISTORY.md` — Wave α rev-2 archived.
- `docs/handoff/CURRENT.md` — reset for next task.

## Test Results

Wave α final state (HEAD `a5faf91`): all gates green.
- `gofmt -l .` clean.
- `go vet ./...` clean.
- `go build ./cmd/tpatch` clean.
- `go test ./...` green.

## Next Steps

Supervisor to pick next task from CLUSTERS.md. See "Active Task" candidates above.

## Blockers

None.

## Context for Next Agent

- Two-opinion external review protocol (supervisor + user-parallel) has caught a real regression in every Wave α revision. Continue this pattern for Wave β.
- External review briefs MUST require a sweep of ALL PRD §6 acceptance criteria — not just the rev's stated findings. This lesson is from rev-1 (supervisor external missed F3 because it scoped to F1+F2). Rev-2's brief baked the lesson in and it worked.
- Dispatch briefs MUST NOT contain escape hatches that override PRD acceptance criteria. (Rev-0 lesson: "if integration is risky, defer to Wave β" allowed implementer to ship the classifier as dead library code.)
- ADR-025 schema lock is binding for all 9 PRDs in WP-003. Any β/γ PRD touching evidence schema must check D1–D13 first.

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
