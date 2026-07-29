# Current Handoff

## Active Task

- **Task ID**: `post-v0.11.2-parallel-streams`
- **Milestone**: Post-v0.11.2 combined roadmap: v0.11.3 stabilization slot (fix issue #2 verify V8 double-apply) + Option C paper draft (`PRD-active-feature-session`) + issue #1 PRD-pair paper drafts (`PRD-feature-supersession` + `PRD-write-file-recipe-safety`). Three parallel streams; then implement issue #1 as v0.12.0, then WP-004.
- **Description**: v0.11.2 shipped 2026-07-29. Two open GH issues classified by supervisor: #1 (supersession + write-file safety) → PRD-pair (two paper PRDs + ADR-028/029 on decision-lock); #2 (verify V8 double-applies) → BUG, v0.11.3 stabilization slot. Combined with user's Option C (PRD-active-feature-session, unlocks ADR-027 F3 downstream). Three parallel work streams share only handoff CURRENT.md; supervisor consolidates.
- **Status**: In Progress (three streams).
- **Assigned**: 2026-07-29.

## Three parallel work streams

### Stream C — Issue #2 fix (v0.11.3 stabilization slot)

**GH Issue**: [#2 verify V8 double-applies equivalent recipe and canonical patch](https://github.com/tesseracode/tesserapatch/issues/2).

**Nature**: Localized bug in `internal/workflow/verify.go:948-985`. V7 applies recipe to shadow, then V8 checks canonical patch against ALREADY-MODIFIED shadow — for correct recipe/patch pairs (equivalent representations of the same change), V8 fails because it's double-applying. Reporter observed on v0.11.1 while migrating `session-search` in `t3code`.

**Classification**: BUG (not PRD, not whitepaper, no new ADR unless fix changes V7/V8 semantics beyond a shadow reset — then small ADR-013 amendment).

**Timing**: v0.11.3 stabilization slot. Ships alongside any other post-v0.11.2 small fixes that surface.

**Fix options** (implementer picks in brief):
- Option A (simplest): reset shadow between V7 and V8.
- Option B: separate shadows for V7 and V8.
- Option C: explicit idempotence/equivalence check contract.

**Dispatch**: full implementer + two-opinion review cycle (rule 14 mandatory). Empirical reproduction required per rule 20.

### Stream A — Option C paper draft (`PRD-active-feature-session`)

**Nature**: Draft `docs/prds/PRD-active-feature-session.md` at `Proposed` status. Unlocks ADR-027 F3 (D1 local-buffer path softness) by pinning the primary local-buffer path.

**Precedent shape**: ADR-027 draft model + Slice 4 doctor PRD draft model (paper-only, three-way review).

**Timing**: Dispatch after Stream C's fix lands (so CURRENT.md handoff doesn't churn). Runs in parallel with Stream B after that.

### Stream B — Issue #1 PRD-pair paper drafts

**GH Issue**: [#1 Add supersession edges and guard write-file recipes against stale reverts](https://github.com/tesseracode/tesserapatch/issues/1).

**Classification**: Two connected-but-distinct gaps → **two PRDs** (not whitepaper — fixes are largely independent):
- `PRD-feature-supersession` + ADR-028 (`supersedes` edge model on ADR-011 graph).
- `PRD-write-file-recipe-safety` + ADR-029 (preimage hash preconditions + later-touch detection).

**Timing**: Dispatch after Stream C's fix lands. Runs in parallel with Stream A.

**Implementation** (deferred): after Streams A+B PRDs three-way APPROVED, sequence supersession first (unlocks "which features to replay") then write-file safety. Target v0.12.0.

## Combined roadmap sequencing

1. **NOW**: Stream C (issue #2 fix) — dispatch first as v0.11.3 stabilization slot.
2. **After Stream C three-way APPROVED**: Ship v0.11.3 following RELEASING.md.
3. **After v0.11.3 shipped**: Dispatch Streams A + B in parallel (paper-only).
4. **After Streams A + B three-way APPROVED**: Archive; kick off supersession implementation as v0.12.0.
5. **After supersession + write-file safety land**: Kick off Option A (WP-004 `auto-feature-dependencies`) as the next major cluster.

## Stream C binding scope (Issue #2 fix)

### Detection + fix

- Read `internal/workflow/verify.go:948-985` in full to understand current V7 + V8 shadow-shared logic.
- Read `internal/workflow/verify_closure_replay_test.go` to understand the happy-path coverage that misses this matrix cell (V8 skipped when recipe present per issue).
- Choose Option A / B / C. Recommend **Option A** (reset shadow between V7 and V8): simplest fix, doesn't change disk footprint or PRD semantics. Document choice in closure summary.

### Test coverage

- Add empirical reproduction test in `internal/workflow/verify_closure_replay_test.go` matching issue's Reproduction scenario:
  - Fixture: applied feature with recipe + canonical post-apply.patch that produces equivalent changes, no hard parents, both replay cleanly against base independently.
  - Assert: `tpatch verify <slug> --no-write` passes BOTH V7 (`recipe_replay_clean`) AND V8 (`post_apply_patch_replay_clean`).
  - This test would have failed on v0.11.1 (per issue) → serves as durable regression guard.

### Optional ADR-013 amendment

- If the fix changes V7/V8 shadow semantics beyond "shadow reset between passes", draft a small D-clause amendment to `docs/adrs/ADR-013-verify-freshness-overlay.md`. Otherwise no ADR needed.

### CHANGELOG

- Add `## v0.11.3 (unreleased) — verify V8 double-apply fix` header at CHANGELOG top.
- Bullet describing the fix.

### Stream C hard constraints (20 binding + 2 v0.11.2-lineage)

Same 20 rules as v0.11.2 doctor cluster close. Especially:
- Rule 9 (behavior-implemented-vs-tested): read verify.go:948-985 verbatim first.
- Rule 15 (trigger-name grep): any `tpatch verify` command mention in the fix must match the actual command shape.
- Rule 18 (structural trailer verification): every commit's trailer passes `git log --format='%(trailers)' <sha>` non-empty.
- Rule 20 (empirical user-workspace reproduction): reproduce the fix scenario BEFORE + AFTER in a synthetic tpatch workspace.

Side Research md5 == `b385fe622db9926f48861105239f113e`.

## Session Summary

v0.11.2 shipped. Two GH issues triaged: #1 → PRD-pair (paper draft), #2 → bug fix (v0.11.3 stabilization slot). Combined with user's Option C (PRD-active-feature-session). Three parallel streams; Stream C dispatches first.

## Next Steps

1. Supervisor: dispatch Stream C (issue #2 fix implementer).
2. After Stream C three-way APPROVED: ship v0.11.3.
3. After v0.11.3 shipped: dispatch Streams A + B in parallel.
4. Consolidate + archive after each stream lands.

## Blockers

None.

## Context for Next Agent

- HEAD at three-stream kickoff: `aec05e4` (v0.11.2 post-release tracking).
- Two GH issues open at kickoff:
  - #1: https://github.com/tesseracode/tesserapatch/issues/1 (supersession + write-file safety) → PRD-pair.
  - #2: https://github.com/tesseracode/tesserapatch/issues/2 (verify V8 double-apply) → v0.11.3 fix.
- 20 binding carry-forward rules.
- Two-opinion protocol scoreboard: 15/15 rev cycles at final concurrence; user-external uniquely blocked/caught in 7 of 15 at rev-0.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

## v0.11.2 release summary

- **Tag**: `v0.11.2` on `origin/v0.11.2` at release commit `3267455`.
- **GH Release**: https://github.com/tesseracode/tesserapatch/releases/tag/v0.11.2 (marked `Latest`; v0.11.1 demoted).
- **Scope**: ~30 commits since v0.11.1 covering 4 doctor waves + F1/F2/F3 folds + LOG updates.
- **CHANGELOG**: `## v0.11.2 — 2026-07-29 — tpatch doctor implementation` graduated from `(unreleased)` header with Wave α/β/γ/δ subsections.
- **RELEASING.md validated**: em-dash-anchored awk extraction worked on first try (validating the v0.11.1 doc fix). 3-artifact lock-step complete.
- **Public CLI additions**: `tpatch doctor [--dry-run] [--fix] [--json] [--check <id>] [--release-metadata <file>]`.
- **Zero code regressions**: full-cluster acceptance sweep 29/29 §6 MET; all pre-cluster tests still pass.

## Doctor cluster closure summary

- **4 waves**, all three-way APPROVED at final acceptance.
- **15 consecutive rev cycles** at three-way concurrence.
- **20 binding carry-forward rules** (up from 17 at cluster kickoff).
- Full snapshot archive in `docs/handoff/HISTORY.md`.

## Open decision for supervisor

Same options as post-v0.11.1 with one new option unlocked (doctor follow-ups):

**Option A — WP-004** (`auto-feature-dependencies`). Draft at `docs/whitepapers/WP-004-auto-feature-dependencies.md`. Continues WP-002 → WP-003 sequence into dependency automation.

**Option B — WP-005** (`spec-driven-workflows`). Draft at `docs/whitepapers/WP-005-spec-driven-workflows.md`. Opens spec-workflow surface.

**Option C — Research roadmap continuation**. Six blocked capture PRDs unlocked by ADR-027. Recommendation: `PRD-active-feature-session` (locks ADR-027 F3 follow-up).

**Option D — Doctor follow-ups** (optional cleanup, not urgent):
- S3-boundary observation from Wave δ rev-1 supervisor-external: mixed-CHANGELOG scope (repo-scoped vs per-tag) — draft small ADR or PRD amendment if the boundary proves important in practice.
- ADR-027 F2 (roadmap naming coord) and F3 (D1 local-buffer path softness) still deferred.

## Carry-forward dispatch rules (20 binding)

See prior CURRENT.md snapshots in HISTORY.md for full text. All 20 rules still binding.

## Session Summary

Doctor implementation cluster CLOSED 2026-07-29 across 4 waves (α+β+γ+δ). v0.11.2 SHIPPED via RELEASING.md's second real-world validation. All 4 SQL doctor todos flipped to `done`. Awaiting next-block decision.

## Next Steps

1. Supervisor: pick Option A, B, C, or D.
2. If Option A/B (WP-004/WP-005): read the WP draft, ask for PRD ordering + wave structure, dispatch first slice.
3. If Option C: recommend `PRD-active-feature-session` first (locks ADR-027 F3).
4. If Option D: small doctor follow-up ADR/PRD amendment.

## Blockers

None.

## Context for Next Agent

- v0.11.2 is the current `Latest` GH Release; v0.11.1 demoted; v0.11.0 further demoted.
- 20 binding carry-forward rules — see HISTORY.md snapshots for full text and lineage.
- Two-opinion protocol scoreboard: 15/15 rev cycles at final concurrence; user-external uniquely blocked/caught in 7 of 15 at rev-0.
- Doctor implementation cluster is the largest single-cluster (in commits) shipped so far — the 4-wave pattern proved scalable for D-clause-organized detection code with mixed read-only + `--fix` semantics.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

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
