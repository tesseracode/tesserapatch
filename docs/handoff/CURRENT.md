# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-wave-beta-writefile-safety-implementation`
- **Milestone**: v0.12.0 Wave β — implement `PRD-write-file-recipe-safety` + `ADR-029-write-file-recipe-safety`.
- **Description**: Second wave of the v0.12.0 3-wave sequential cluster. Adds `preimage_hash` field + later-touch detection to `write-file` recipe ops, protecting against silently reverting later fixes. Couples to Wave α supersession via §PRD-1-interaction (superseded features downgrade write-file drift).
- **Status**: Dispatched — implementer in flight.
- **Assigned**: 2026-07-29.

## Wave β scope (locked)

**Read these first, in order**:

1. `docs/prds/PRD-write-file-recipe-safety.md` (233 lines, 13 acceptance criteria) — the spec.
2. `docs/adrs/ADR-029-write-file-recipe-safety.md` (108 lines, D1–D8) — the locked model. `sha256:<hex>` deliberately distinguished from `pg_/re_/rr_<12hex>` record identities.
3. `docs/prds/PRD-feature-supersession.md` §PRD-1-interaction + Wave α acceptance in `docs/handoff/HISTORY.md` — the coupling contract. Wave α now correctly excludes stale-superseded features from replay; Wave β must downgrade write-file drift on features suppressed by supersession.
4. Prior Wave α diff (`7081c62..e5e0091`) for pattern reference — especially `internal/store/validation.go` (write-time rejection pattern from Slice R3) and `internal/workflow/labels.go` (severity-ordered emit + IsSupersessionLabel prefix-match pattern).

**What ships**:

- New `preimage_hash: sha256:<hex>` field on `write-file` recipe ops. Raw `sha256:` prefix (NOT truncated like `pg_/re_/rr_`).
- Preimage-hash precondition check before write-file execution: reject if the current file content hash does not match `preimage_hash`.
- Later-touch detection: if the target file has been touched by a later feature (via commit graph or manifest), reject with actionable message.
- Schema addition ripples through 6 shipped skill assets — parity guard (`TestSkillRecipeSchemaMatchesCLI` or equivalent) MUST update in the same commit (Slice 1 anti-drift lesson).
- Supersession coupling: superseded features (via Wave α's `isFeatureSupersededIn`) downgrade write-file drift severity per PRD §PRD-1-interaction.
- CHANGELOG `## v0.12.0 — TBD` amendment (Wave α bullets already present; append Wave β bullets).
- Status flip: `PRD-write-file-recipe-safety` + `ADR-029` from `Proposed` → `Accepted`.

**What does NOT ship in Wave β**:

- `prefer-contextual` heuristic (deferred to v1+).
- Cross-feature validation of preimage hashes (v1+).
- Regeneration guidance (v1+).
- Active-feature-session lane → Wave γ.

## Wave β slice plan (locked)

1. **Slice 1 — Schema + parity guards** (foundation).
   - Add `preimage_hash` field to write-file recipe op struct.
   - Update `TestSkillRecipeSchemaMatchesCLI` in the SAME commit.
   - Update the 6 shipped skill assets so write-file recipe documentation mentions `preimage_hash`.
2. **Slice 2 — Preimage precondition + reject path**.
   - Implement raw sha256 computation + comparison at recipe-execute time.
   - Reject with actionable ADR-020-style message when hash mismatches.
   - Regression tests: match, mismatch, missing-field (backward-compat) cases.
3. **Slice 3 — Later-touch detection**.
   - Detect if the target file has been touched by a later-feature commit (see PRD for detection method).
   - Reject with actionable message naming the later feature.
   - Regression tests for both directions (touched / not-touched).
4. **Slice 4 — Supersession coupling** (Wave α interaction).
   - When the recipe op belongs to a superseded feature (via `isFeatureSupersededIn`), downgrade preimage-hash mismatch + later-touch drift severity per PRD §PRD-1-interaction. Do NOT bypass the checks entirely — downgrade to warning-with-note instead of hard-reject.
   - Regression tests for both directions (superseded / not).
5. **Slice 5 — CHANGELOG amendment + PRD/ADR flips**.
   - Amend `## v0.12.0 — TBD` entry to append Wave β bullets (do NOT touch Wave α bullets).
   - Flip PRD-write-file-recipe-safety + ADR-029 `Proposed → Accepted`.
   - Update ROADMAP v0.12.0 Wave β status marker.

## Wave β validation gates

Same as Wave α (gofmt, vet, build, full test suite). Baseline: 129 top-level PASS at Wave α acceptance. Wave β total MUST be ≥ 129 + 6-10 (schema + reject + later-touch + coupling + backward-compat).

**Additional gates**:
- Parity guard test MUST update in Slice 1 (anti-drift lesson from Wave α rev-0 F-SEXT-2 partial + doctor Wave β F1).
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.

## Carry-forward dispatch rules (20 binding, all apply)

Same 20 rules as Wave α close. Highlights for Wave β:

- **Rule 15**: no new `tpatch` command — Wave β is validation/execution changes only.
- **Rule 18**: every commit MUST carry parseable `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.
- **Rule 19**: recipe-execute path is shipped surface. Any behavior change must cite PRD/ADR clause in commit message.
- **Rule 20**: reproduce empirically — preimage mismatch reject + later-touch reject + supersession downgrade paths ALL need empirical verification (Wave α rev-0 F-SEXT-1/2 were display-contract bugs missed BECAUSE they weren't reproduced empirically at rev-0).

## Session Summary

Wave α accepted 2026-07-29 at three-way concurrence (19th scoreboard entry). Wave β dispatched 2026-07-29. Wave γ pending Wave β acceptance.

## Next Steps

1. Wave β implementer executes Slices 1–5.
2. Wave β implementer updates handoff at each phase transition.
3. Supervisor dispatches internal + supervisor-external reviewers in parallel.
4. User's parallel external pass on rev-0.
5. On three-way APPROVED: archive Wave β, dispatch Wave γ (active-feature-session).

## Blockers

None.

## Context for Next Agent

- HEAD at Wave β dispatch: `763b926` (Wave α rev-1 dual review) + this consolidation commit pushing to `origin/main`.
- Wave α acceptance is on `main` — Wave β can freely depend on `store.DependencyKindSupersedes`, `store.ErrMultipleActiveSuperseders`, `isFeatureSupersededIn` semantics from Wave α.
- 20 binding carry-forward rules. Rule 20 rigor extension pattern optional but recommended for empirical CLI checks (Wave α rev-0 taught that display-contract bugs get missed without it).
- Two-opinion protocol scoreboard: 19/19 final-acceptance three-way concurrence.
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
