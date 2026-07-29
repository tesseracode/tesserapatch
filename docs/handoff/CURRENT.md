# Current Handoff

## Active Task

- **Task ID**: `streams-a-and-b-parallel-paper-prds`
- **Milestone**: Post-v0.11.3 parallel PRD-drafting streams. Streams A + B dispatched in parallel; supervisor consolidates on landing.
- **Description**: v0.11.3 shipped 2026-07-29 (Stream C closed GH #2). Streams A + B now unblocked; both are paper-only PRD drafts with no code changes. Runnable in parallel because they touch different files (Stream A: single new PRD; Stream B: two new PRDs) and no shared production code. Only shared touchpoint: this handoff file — supervisor consolidates when both land.
- **Status**: In Progress (two parallel streams).
- **Assigned**: 2026-07-29.

## Stream A binding scope — `PRD-active-feature-session`

**GH Reference**: user Option C. Unlocks ADR-027 F3 (D1 local-buffer path softness).

**Deliverable**: `docs/prds/PRD-active-feature-session.md` at `Proposed` status.

**Precedent shape**: ADR-027 draft model + Slice 4 doctor PRD draft model. Both paper-only, three-way review, no code implementation this slice.

**What the PRD must decide**:

The ADR-027 D-clauses established the privacy-restrictive boundary for future capture-context features but left three softness gaps for downstream PRDs:

- **F3 (LOW, deferred)**: D1 local-buffer path is intentionally soft — implementer left the choice open between `.git/tpatch/capture/`, OS user-cache location, and `.tpatch/local/capture/`-style paths. PRD-active-feature-session locks this.
- Adjacent: what constitutes an "active feature session" boundary — when does a session start, stop, and get promoted from local buffer to committed context summary?

**Concrete scope** (implementer expands):
1. Session lifecycle: start/stop triggers; per-feature vs per-tpatch-command scope.
2. Local-buffer storage: canonical path for the D1 local lane (fold ADR-027 F3).
3. Session-to-summary promotion: what triggers a `record` to consume a local buffer and produce a committed summary? What's the redaction contract on that boundary (mirror ADR-027 D3)?
4. CLI surface: any new commands? Any new flags on `tpatch record` / `tpatch analyze` / etc.?
5. Privacy invariants (mirror ADR-027 D2 + D10): what content can flow from active session → local buffer → committed summary? What's explicitly forbidden?
6. Acceptance criteria (§6.1-§6.N): idempotence, dry-run defaults, per-check failure isolation.

**Non-scope for this PRD**:
- Actual agent event log implementation (deferred to `PRD-agent-event-log`).
- IDE capture hooks (deferred to `PRD-ide-capture-hooks`).
- Git hook capture guards (deferred to `PRD-git-hook-capture-guards`).
- Metadata branch storage (deferred to `ADR-capture-metadata-branch`).

**Structure suggestion** (adjust): §0 Meta → §1 Problem → §2 Goals/Non-goals → §3 User-facing contract → §4 Session lifecycle → §5 Implementation notes → §6 Acceptance criteria → §7 Open questions → §8 Out of scope → §9 Sources.

**Hard constraints for Stream A** (subset of 20 binding rules):
1. Paper-only: no code changes; no `internal/` / `cmd/` touches.
2. ADR-027 D-clauses binding (no invalidation of D2/D10 privacy).
3. Status = `Proposed` (not Accepted).
4. Cite ADR-027 F3 verbatim and lock the D1 path.
5. Explicit non-scope declaration for the five deferred capture PRDs.
6. Rule 8 (display-string contracts): if the PRD specifies filenames, directory paths, or CLI flag names, those become contracts for the implementation slice.
7. Rule 15 (trigger-name grep): any `tpatch <command>` referenced in the PRD must exist in `internal/cli/cobra.go`.
8. Rule 17 (totality claims): avoid "only X is supported" totality claims without verification against ALL layers of the production model.
9. Rule 18 (structural trailer verification): every commit's trailer passes structurally.
10. Side Research md5 == `b385fe622db9926f48861105239f113e`.

**Suggested target size**: 300-500 lines (comparable to ADR-027 or `PRD-tpatch-land`).

## Stream B binding scope — Issue #1 PRD pair

**GH Reference**: [Issue #1](https://github.com/tesseracode/tesserapatch/issues/1). Empirical evidence in `copilot-api/.tpatch/RETROSPECTIVE.md` Part 2.

**Deliverables** (two PRDs, both at `Proposed` status):

### PRD 1: `docs/prds/PRD-feature-supersession.md`

**What it decides**:

Extends the ADR-011 feature dependency graph with a third-kind edge: `supersedes` (currently only `hard` + `soft`).

- Model: is `supersedes` a third edge kind on the same graph (ADR-011 D1 storage), a separate edge type, or a lifecycle-state mutation? Lock the choice.
- Semantics per the issue: preserve both histories; exclude the superseded feature from replay by default when its replacement is active; surface in `status`, `next`, dependency validation, reconcile, generated indexes; detect conflicting states, cycles, multiple active superseders; allow queries for effective/current vs historical/superseded features.
- Composable label semantics (mirror ADR-011 D3): `superseded-by`, `stale-superseder`, etc.
- Interaction with reconcile: if the superseded feature has a stale recipe (see PRD 2), does supersession disable replay AT ALL, or downgrade drift severity?

**Non-scope for PRD 1**:
- Automatic supersession detection (deferred).
- UI/display polish (deferred to a later slice).

### PRD 2: `docs/prds/PRD-write-file-recipe-safety.md`

**What it decides**:

Adds safeguards for `write-file` operations to prevent silent-revert-of-later-fixes per issue's 5 requested safeguards:

1. Prefer contextual operations: `write-file` reserved for created-by-feature files or explicitly declared whole-file ownership.
2. Preimage hash preconditions: store expected preimage hash; refuse to overwrite when current file differs.
3. Later-touch detection: during record/reconcile/validation, detect when a later feature touches a path owned by an older `write-file` op.
4. Cross-feature recipe validation: validate the effective ordered feature stack, not each recipe independently.
5. Regeneration guidance: actionable commands to regenerate stale recipes while preserving `post-apply.patch` as authoritative intent.

**Decide** which are v1 mandatory (recommend: 2 preimage hash + 3 later-touch detection) vs v1+ deferred (recommend: 1 prefer-contextual is a policy decision needing more study; 4 cross-feature validation is heavier).

**Interaction with PRD 1**: supersession disables replay for superseded features → write-file drift never fires for those. Cross-reference both PRDs.

**Optional matching ADRs** (draft alongside if the decision surface warrants a separate lock):
- **ADR-028** (`supersession-edge-model`): locks the graph-model decision from PRD 1.
- **ADR-029** (`write-file-recipe-safety`): locks the schema decisions (preimage hash field, later-touch detection contract).

Precedent: ADR-024 + ADR-026 pattern (PRD + adjacent ADR).

**Hard constraints for Stream B**:
1. Paper-only: no code changes.
2. Two PRDs are separate files but MAY cross-reference.
3. Status = `Proposed` for both.
4. Cite ADR-011 D1/D3/D4 (dependency graph model) verbatim; do not invalidate.
5. Cite the empirical retrospective in `copilot-api/.tpatch/RETROSPECTIVE.md` Part 2.
6. Rules 8, 15, 17, 18 apply (same as Stream A).
7. If drafting ADR-028 / ADR-029: same status = `Proposed`, cite PRD as motivation.
8. Side Research md5 == `b385fe622db9926f48861105239f113e`.

## Streams A + B collision avoidance

- Stream A file: `docs/prds/PRD-active-feature-session.md` (new).
- Stream B files: `docs/prds/PRD-feature-supersession.md` + `docs/prds/PRD-write-file-recipe-safety.md` (both new). Optional: `docs/adrs/ADR-028-supersession-edge-model.md` + `docs/adrs/ADR-029-write-file-recipe-safety.md`.
- Shared handoff: `docs/handoff/CURRENT.md`. Both streams add closure summaries at the end; supervisor consolidates.
- Shared parity: `docs/adrs/README.md` if ADR-028/029 land. Both would append. Supervisor merges if collision.

## Combined roadmap sequencing (after Streams A + B)

1. Streams A + B three-way review each.
2. After A + B APPROVED: archive; kick off implementation.
3. Implement supersession first (unlocks "which features to replay") + write-file safety second → target v0.12.0.
4. After v0.12.0: Option A (WP-004 `auto-feature-dependencies`) as next major cluster.

## Carry-forward dispatch rules (20 binding)

Same 20 rules as v0.11.3 close. See prior CURRENT.md snapshots in HISTORY.md for full text. Rule 20 rigor extension pattern (detached-worktree-at-pre-fix + test-copy) documented as optional stronger application — not a new rule.

## Non-blocking follow-ups deferred

- ADR-027 F2 (LOW): PRD-ide-capture-hooks naming coord.
- ADR-027 F3 (LOW): D1 local-buffer path softness — **Stream A locks this**.
- Doctor S3-boundary (LOW): mixed-CHANGELOG scope documentation.

## Session Summary

v0.11.3 shipped 2026-07-29 (Stream C closed GH #2). Streams A + B now dispatched in parallel for paper-only PRD drafting. After both APPROVED, implementation as v0.12.0.

## Next Steps

1. Supervisor: dispatch Streams A + B in parallel.
2. After each stream three-way APPROVED: archive; consolidate handoff.
3. Sequencing: implementation of Streams A + B PRDs targets v0.12.0.
4. After v0.12.0: Option A (WP-004) as next major cluster.

## Blockers

None.

## Context for Next Agent

- HEAD at Streams A + B kickoff: `84a2f88` (v0.11.3 release commit + LOG closure).
- v0.11.3 tag on `origin/v0.11.3`; GH Release marked `Latest`; GH #2 closed.
- 20 binding carry-forward rules; Rule 20 rigor extension pattern optional but recommended.
- Two-opinion protocol scoreboard: 16/16 rev cycles at final concurrence; user-external uniquely blocked/caught in 7 of 16 at rev-0.
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
