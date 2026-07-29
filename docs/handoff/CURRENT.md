# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-implementation-kickoff-decision`
- **Milestone**: All three post-v0.11.3 paper streams landed. Ready for v0.12.0 implementation cluster kickoff.
- **Description**: Streams A + B closed at three-way APPROVED (2026-07-29). All planning artifacts for GH #1 (supersession + write-file safety) and ADR-027 F3 closure (active-feature-session) are drafted at `Proposed` status. Awaiting supervisor decision on v0.12.0 implementation cluster shape.
- **Status**: Awaiting next-block decision.
- **Assigned**: 2026-07-29.

## Available planning artifacts for v0.12.0

Three PRDs + two ADRs at `Proposed` status, ready for implementation:

- **`docs/prds/PRD-active-feature-session.md`** (Stream A, 500 lines, 25 acceptance criteria) — locks ADR-027 F3 to `.tpatch/local/capture/`; new `tpatch session {start,stop,list,summarize,purge}` command group; `tpatch init` `.gitignore` amendment; six-mandate refusal contract.
- **`docs/prds/PRD-feature-supersession.md`** (Stream B, 259 lines, 12 acceptance criteria) — third-kind `depends_on[].kind: "supersedes"` edge; 4 new composable labels; reconcile suppression of superseded features.
- **`docs/prds/PRD-write-file-recipe-safety.md`** (Stream B, 233 lines, 13 acceptance criteria) — `preimage_hash: <sha256>` field on `write-file` ops + later-touch detection.
- **`docs/adrs/ADR-028-supersession-edge-model.md`** — locks Cluster 1 edge model with D1-D8.
- **`docs/adrs/ADR-029-write-file-recipe-safety.md`** — locks preimage hash schema + later-touch detection contract with D1-D8.

## v0.12.0 implementation cluster shape — supervisor decision needed

Recommended sequencing (matches Doctor cluster's 4-wave pattern):

**Option A (Recommended) — Sequential 3-wave v0.12.0 cluster**:
1. **Wave α — Supersession** (`PRD-feature-supersession` + `ADR-028`). Foundation for graph model; unlocks "which features replay" question. Extends ADR-011 D1 storage. Estimated ~4-6 commits + fixtures.
2. **Wave β — Write-file safety** (`PRD-write-file-recipe-safety` + `ADR-029`). Depends on Wave α (supersession suppresses write-file drift on superseded features per PRD 2 §3). `preimage_hash` schema addition drifts skill assets; `TestSkillRecipeSchemaMatchesCLI` guard must update in same commit. Estimated ~4-6 commits + fixtures.
3. **Wave γ — Active-feature-session** (`PRD-active-feature-session`). Independent of Waves α/β. First wave to require `tpatch init` `.gitignore` amendment. Refusal-path test coverage is the entire safety margin (doctor Wave β D3 refusal fixtures are the recommended template). Estimated ~6-8 commits + fixtures (larger surface: new command group + init amendment + storage lane).

**Option B — Parallel dispatch of Waves α+β+γ**: All three touch disjoint code surfaces except Stream A's `tpatch init` amendment (which doesn't collide with Stream B's `depends_on[].kind` addition or `preimage_hash` schema addition). Feasible but adds parallel-review complexity. Trade-off: faster wall-clock, more supervisor consolidation work.

**Option C — Ship Waves α+β as v0.12.0, defer Wave γ**: Waves α+β close GH #1 completely. Wave γ (active-feature-session) unlocks the ADR-027 F3 F3 path but doesn't have downstream capture PRDs implemented yet (agent-event-log, record-context-summary, ide-capture-hooks, git-hook-capture-guards all still deferred). Consider deferring Wave γ to v0.13.0 with those downstream PRDs for cluster coherence.

**Option D — WP-004 next** (user's original Option A): defer v0.12.0 implementation; kick off WP-004 (`auto-feature-dependencies`) as next major cluster. Streams A + B PRDs sit at `Proposed` until later.

Supervisor default recommendation: **Option A** (sequential 3-wave). Matches doctor cluster's proven scaling pattern. If wall-clock matters more than protocol discipline, consider Option B.

## Non-blocking follow-ups deferred

- ADR-027 F2 (LOW): PRD-ide-capture-hooks naming coord.
- Doctor S3-boundary (LOW): mixed-CHANGELOG scope documentation.
- Stream B ADR-029 informational nit: literalize `TestSkillRecipeSchemaMatchesCLI` symbol in parity contract prose (post-v0.12.0 docs polish).

## Carry-forward dispatch rules (20 binding)

Same 20 rules as v0.11.3 close. See prior CURRENT.md snapshots in HISTORY.md for full text. Rule 20 rigor extension pattern (detached-worktree-at-pre-fix + test-copy) remains optional.

## Session Summary

v0.11.3 shipped 2026-07-29 (Stream C closed GH #2). Streams A + B closed at three-way APPROVED 2026-07-29 (paper-only PRD/ADR drafts). Ready for v0.12.0 implementation cluster kickoff — supervisor decision needed on Options A/B/C/D.

## Next Steps

1. Supervisor: pick Option A, B, C, or D.
2. If Option A: dispatch Wave α (supersession) implementer.
3. If Option B: dispatch Waves α+β+γ implementers in parallel background.
4. If Option C: dispatch Wave α + Wave β sequentially; defer Wave γ to v0.13.0.
5. If Option D: read WP-004 draft; ask for PRD ordering + wave structure; dispatch first slice.

## Blockers

None.

## Context for Next Agent

- HEAD at v0.12.0 kickoff: `442fd4f` + supervisor decision LOG entry pending commit.
- Doctor cluster + v0.11.2 shipped; v0.11.3 shipped 2026-07-29 (Stream C).
- Streams A + B closed at three-way APPROVED 2026-07-29 (paper-only).
- 20 binding carry-forward rules. Rule 20 rigor extension pattern optional.
- Two-opinion protocol scoreboard: 17/17 rev cycles at final concurrence; user-external uniquely blocked/caught in 7 of 17 at rev-0 + one process-level F1 (handoff-stale) this consolidation.
- All 5 PRDs/ADRs from Streams A+B are `Proposed` and ready for implementation cluster.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

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
