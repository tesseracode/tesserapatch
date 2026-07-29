# Current Handoff

## Active Task

- **Task ID**: `v0.11.2-release`
- **Milestone**: v0.11.2 release ship — tpatch doctor implementation cluster CLOSED. Following `RELEASING.md` end-to-end (its second real-world validation).
- **Description**: Doctor implementation cluster (Waves α+β+γ+δ) is CLOSED at three-way APPROVED. Full-cluster 29/29 §6 MET. Ready to ship v0.11.2 following the RELEASING.md three-artifact lock-step (CHANGELOG graduate → annotated tag → gh release create --verify-tag --notes-file --latest).
- **Status**: In Progress (release ship).
- **Assigned**: 2026-07-29.

## Doctor cluster closure summary

**All 4 waves shipped**:
- **Wave α** ✅ CLOSED 2026-07-27 (scaffold + D1 + D2 + D8). §6.1-§6.7 + §6.20-§6.29 MET.
- **Wave β** ✅ CLOSED 2026-07-28 (D3 + D7). §6.8, §6.9, §6.18, §6.19 MET.
- **Wave γ** ✅ CLOSED 2026-07-28 (D4 + D5, F1 folded to δ). §6.10-§6.13 MET.
- **Wave δ** ✅ CLOSED 2026-07-29 (D6 + F1 fold-in + F2 close + F3 pre-ship). §6.14-§6.17 MET.

**Full-cluster acceptance sweep: 29/29 §6 MET**.

**Two-opinion protocol**: 15 consecutive rev cycles at three-way concurrence. User-external uniquely blocked/caught in 7 of 15 at rev-0. Supervisor-external uniquely caught F-EXT-1 in Wave α.

**20 binding carry-forward rules** (up from 17 at cluster kickoff; Rules 19 loader-caller-tracing + 20 empirical-user-workspace-reproduction added).

## v0.11.2 release scope (deltas since v0.11.0 → v0.11.2)

- **v0.11.1** (2026-07-23): Stabilization cluster (Slices 1-4) + ADR-027 acceptance + storage-substrate research doc.
- **v0.11.2** (this release): tpatch doctor implementation (D1-D8) + F1 behavior-change disclosure on `tpatch reconcile review list`.

CHANGELOG `## v0.11.2 (unreleased) — tpatch doctor Wave α` header already has Wave α + β + γ + δ subsections. Ship prep: graduate `(unreleased)` → `— 2026-07-29`, adjust header to cover the full cluster.

## Ship steps (following RELEASING.md)

**Step 1**: Graduate CHANGELOG `(unreleased)` header to dated release header.
- Current: `## v0.11.2 (unreleased) — tpatch doctor Wave α`
- Target: `## v0.11.2 — 2026-07-29 — tpatch doctor implementation`
- Verify all Wave α/β/γ/δ subsections + F1 behavior-change bullet + F2 fix bullet are present.
- Commit + push.

**Step 2**: Annotated tag `v0.11.2`.
- `git tag -a v0.11.2 -m "v0.11.2 — <short scope>"`
- `git push origin v0.11.2`

**Step 3**: `gh release create v0.11.2` with `--notes-file` extracted from CHANGELOG.
- Use the em-dash-anchored awk pattern per RELEASING.md's updated guidance:
  `awk '/^## v0\.11\.2 —/,/^## v0\.11\.1 —/' CHANGELOG.md | sed '$d'`
- `--verify-tag`, `--latest`.

**Post-release**: verify `gh release list --limit 3` shows v0.11.2 as Latest. Update this handoff Status to Complete.

## Options after v0.11.2 ships

Same options as post-v0.11.1 (from HISTORY.md post-v0.11.1 handoff snapshot):

**Option A — WP-004** (`auto-feature-dependencies`). Draft at `docs/whitepapers/WP-004-auto-feature-dependencies.md`. Continues WP-002 → WP-003 sequence.

**Option B — WP-005** (`spec-driven-workflows`). Draft at `docs/whitepapers/WP-005-spec-driven-workflows.md`. Opens spec-workflow surface.

**Option C — Research roadmap continuation**. Six blocked capture PRDs unlocked by ADR-027 acceptance. Recommendation: `PRD-active-feature-session` (locks ADR-027 F3 follow-up).

**Option D — Post-v0.11.2 doctor follow-ups**: address the LOW-severity S3-boundary observation (mixed-CHANGELOG scope) if it proves important in practice. Draft an ADR or PRD amendment. Not urgent.

## Carry-forward dispatch rules (20 binding)

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
11. (Slice 2 F1) Flag-surface accuracy claims MUST account for cobra persistent-flag inheritance.
12. (Wave β F3) Privacy tests seed secrets into title/slug/path metadata, NOT file content.
13. (Wave β schema-lock) Briefs say "no persisted-schema additions outside what binding ADRs explicitly authorize" — enumerate which fields/clauses.
14. Two-opinion external review protocol (supervisor + user-parallel) MANDATORY. 15/15 rev cycles has caught HIGH BLOCKERs or confirmed fixes.
15. (γ-1 F1) When PRD names a command/event as trigger, verify the command/event actually exists in production code BEFORE wiring implementation.
16. (Slice 1 anti-drift lesson) When a docs-vs-code drift finding is fixed, add or extend a parity guard test that decodes/validates the docs artifact against the code ground-truth when feasible.
17. (Slice 4 / totality generalization) Docs totality claims ("only X", "the full list is Y") MUST be verified against ALL layers of the production model.
18. (Doctor Wave α F-EXT-1) Internal reviewer checklists MUST include structural trailer verification (`git interpret-trailers --parse`), not text-grep.
19. (Doctor Wave γ F1) Reviewers MUST trace exported loader callers via grep before accepting store/workflow/cli diffs as internal refactor. Shipped-CLI-surface callers → §6 criterion + CHANGELOG bullet + test.
20. (Doctor Wave δ F2) Reviewer briefs for user-facing CLI checks MUST include an "empirically reproduce in a user-workspace scenario" step: build the binary, initialize a NON-tpatch repo, run the check, verify output is actionable and not noisy.

## Non-blocking follow-ups deferred

- **ADR-027 F2** (LOW): PRD-ide-capture-hooks naming coord.
- **ADR-027 F3** (LOW): D1 local-buffer path softness.
- **Doctor S3-boundary** (LOW): mixed-CHANGELOG scope documentation.

## Session Summary

Doctor implementation cluster CLOSED at three-way APPROVED across 4 waves (α+β+γ+δ). 29/29 §6 MET. F3 pre-ship fix landed as supervisor-direct one-line guard. v0.11.2 ready to ship following RELEASING.md.

## Next Steps

1. Supervisor: execute RELEASING.md 3-step ship for v0.11.2.
2. After v0.11.2 shipped: archive this CURRENT.md, open post-v0.11.2 decision handoff.

## Blockers

None.

## Context for Next Agent

- HEAD at v0.11.2 ship prep: `17417c6` (F3 pre-ship fix + LOG closure).
- Doctor cluster archived to HISTORY.md 2026-07-29 (4 waves + F1 fold-in + F2 close + F3 pre-ship).
- 20 binding carry-forward rules. Rules 19 + 20 both graduated from candidate this cluster.
- Two-opinion protocol continues to earn its keep (7 of 15 rev cycles user-external uniquely caught real production findings).
- v0.11.1 shipped via same RELEASING.md process — validated end-to-end (with awk em-dash fix landed alongside). v0.11.2 is the second real-world exercise.
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
