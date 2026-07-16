# Current Handoff

## Active Task

- **Task ID**: `post-v0.11.0-decision`
- **Milestone**: v0.11.0 released 2026-07-16 (WP-003 cluster complete). Awaiting supervisor decision on next work block: WP-004, WP-005, or research roadmap continuation.
- **Description**: v0.11.0 shipped at `1c63d1d` with tag `v0.11.0` pushed to `origin/v0.11.0`. CHANGELOG.md updated with full WP-003 cluster summary. Now open for next cluster kickoff. WP-004 (`auto-feature-dependencies`) and WP-005 (`spec-driven-workflows`) both have drafted whitepapers ready for PRD ordering + wave structure decisions.
- **Status**: Awaiting next-phase dispatch (no active implementer).
- **Assigned**: 2026-07-16.

## v0.11.0 release notes

- **Tag**: `v0.11.0` on `origin/main` at commit `1c63d1d`.
- **Scope**: 59 commits since v0.10.0, ~10,183 insertions.
- **CHANGELOG**: Full WP-003 cluster summary (all 9 PRDs, 4 waves).
- **Public CLI additions**:
  - `tpatch reconcile audit-retirement <slug> [--json]`
  - `tpatch reconcile confirm-upstreamed <slug> [--json]`
  - `tpatch reconcile review add`
  - `tpatch reconcile review list [--json]`
- **Dev-only tool**: `internal/tools/studyvalidator/` (not in public CLI).
- **New persisted artifacts** (under ADR-025 D1-D13):
  - `.tpatch/features/<slug>/artifacts/reconcile-evidence.jsonl`
  - `.tpatch/features/<slug>/artifacts/reconcile-revisions.jsonl`
- **Zero** schema drift, new lifecycle states, or new user-facing enforcement config flags.

## WP-003 cluster closure summary

### Ship totals (on `origin/main`)
- **9 PRDs** across 4 waves:
  - **Wave α** (v0.9.0 alpha-2 approved 2026-05-26): PRD 1 `reconcile-verdict-evidence`, PRD 6 `reconcile-file-novelty-classifier`.
  - **Wave β** (approved 2026-06-28): PRD 2 `upstreamed-confirmation-gate`, PRD 3 `reconcile-revision-pass-log`, PRD 7 `reconcile-hunk-overlap-detector`.
  - **Wave γ-1** (approved 2026-07-10): PRD 4 `reconcile-retirement-state-audit`, PRD 5 `reconcile-study-validation`, PRD 8 `reconcile-blocked-verdict-taxonomy`.
  - **Wave γ-2** (approved 2026-07-16): PRD 9 `reconcile-path-restructure-detector`.
- **ADR-025** (`reconcile-evidence-and-revision-schema`) governs entire cluster. Zero schema drift.
- **Zero** new lifecycle states across 9 PRDs (evidence-metadata pattern preferred over persisted enum).
- **Two-opinion external review protocol** validated: caught HIGH BLOCKERs in α rev-0, β rev-0, γ-1 rev-0.
- **15 process rules** codified in dispatch-brief carry-forwards.

### Deltas since v0.10.0 (unreleased)
All 9 WP-003 PRDs + WP-002 Wave β/γ (already in v0.10.0 CHANGELOG) landed after v0.10.0 tag. v0.11.0 would bundle:
- WP-003 Wave α: reconcile evidence artifact + revision-pass writer + file-novelty classifier.
- WP-003 Wave β: upstreamed confirmation gate + revision-pass log + hunk-overlap detector.
- WP-003 Wave γ-1: retirement-state-audit + study-validator (dev-only tool) + blocked-verdict-taxonomy.
- WP-003 Wave γ-2: path-restructure detector.
- New CLI subcommand: `tpatch reconcile confirm-upstreamed <slug>` (Wave γ-1 rev-1 Path A).
- New internal tool: `internal/tools/studyvalidator/` (dev-only, not in public CLI).

## Open decision for supervisor

v0.11.0 shipped. Pick next work block:

**Option A — Kick off WP-004 (`auto-feature-dependencies`)**. Existing draft at `docs/whitepapers/WP-004-auto-feature-dependencies.md`. Continues the WP-002 → WP-003 sequence into dependency automation.

**Option B — Kick off WP-005 (`spec-driven-workflows`)**. Existing draft at `docs/whitepapers/WP-005-spec-driven-workflows.md`. Opens spec-workflow surface (potentially higher user-visible impact).

**Option C — Research roadmap continuation**. Return to `docs/state-of-the-art/research-roadmap.md` for exploratory items (structural fingerprints, commutation graph, search planner, vector index). Feeds future PRDs but no immediate user-facing shipment.

**Option D — Small quality/UX work**. Address any deferred v0.11.x follow-ups (e.g., PatchIDDetectorEnabled flag flip decision, WP-004 blockers, or a v0.11.1 patch release for any post-release issues discovered).

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
14. Two-opinion external review protocol (supervisor + user-parallel) MANDATORY. 6/6 rev cycles has caught HIGH BLOCKERs or confirmed fixes.
15. (γ-1 F1) When PRD names a command/event as trigger, verify the command/event actually exists in production code BEFORE wiring implementation.

## Session Summary

WP-003 cluster closure recorded. γ-2 rev-0 archived 2026-07-16. Awaiting next-block decision (release vs new WP).

## Next Steps

1. Supervisor: pick Option A, B, C, or D above.
2. If Option A (v0.11.0):
   - Audit `git log v0.10.0..HEAD` for full release scope.
   - Draft CHANGELOG v0.11.0 entry covering all WP-003 waves.
   - Tag `v0.11.0`, push tag.
   - Archive CURRENT.md, open next-wave kickoff.
3. If Option B or C:
   - Read the WP draft.
   - Ask (as WP-003 did) for PRD ordering + wave structure.
   - Dispatch first implementer slice.

## Blockers

None.

## Context for Next Agent

- WP-003 was a 9-PRD, 4-wave cluster with a strict ADR-025 schema lock. That template (single cluster ADR + wave-sliced implementation + two-opinion external review) worked well and should be the default for future multi-PRD clusters.
- `docs/handoff/HISTORY.md` contains full per-wave snapshots for WP-002 β/γ, WP-003 α, β, γ-1, γ-2. Reference these for pattern reuse.
- 15 carry-forward dispatch rules live above. Every future implementer/reviewer brief must incorporate applicable rules.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`. Verify before/after any CURRENT.md edits.

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
