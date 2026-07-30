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

Wave α accepted 2026-07-29 at three-way concurrence (19th scoreboard entry). Wave β dispatched 2026-07-29. Wave β implementer executed Slices 1–5 to completion in a single sitting; rev-0 hand-back for three-way review is ready. Wave γ pending Wave β acceptance.

### Wave β implementation — per-slice results (all committed, none pushed)

- **Slice 1 — schema + parity guards** (`639efb2`):
  extended `RecipeOperation` (`internal/workflow/implement.go`) with
  optional `PreimageHash *string` field (pointer type distinguishes
  legacy nil from explicit new-file `""` per PRD §3.3); documented
  `preimage_hash` in all six shipped skill assets (Claude, Copilot,
  Copilot Prompt, Cursor, Windsurf, Generic) with matching JSON
  example + prose bullet; added
  `write-file-safety/preimage-hash-field` required substring anchor
  in `assets/assets_test.go` so schema drift trips the parity guard
  (anti-drift lesson carried forward from Wave α rev-0 F-SEXT-2).
- **Slice 2 — preimage precondition + reject path** (`329f009`):
  new `internal/workflow/writefile_safety.go` with sentinels
  (`ErrWriteFilePreimageMismatch`, `ErrWriteFileLaterTouch`),
  constants (`PreimageHashPrefix = "sha256:"`, `PreimageHashHexLen
  = 64`), helpers (`computeFileSHA256`, `checkWriteFilePreimage`,
  `isLowercaseHex`), and orchestrator
  `runWriteFilePreimagePrecheck` returning
  `PreimagePrecheckResult{Errors, Warnings}`. Wired precheck into
  `ExecuteRecipe` + `DryRunRecipe` in `internal/workflow/recipe.go`
  ahead of any op mutation (ADR-029 D3 all-or-nothing). Malformed
  hash literals (uppercase hex, wrong length, missing `sha256:`
  prefix, embedded whitespace) rejected via `isLowercaseHex`. Legacy
  nil-preimage recipes warn-and-proceed per ADR-029 D4; explicit
  `""` (new-file gate) refuses if the target exists per PRD §3.3;
  populated `sha256:<64hex>` compared byte-exact per ADR-029 D2.
  Diagnostics carry only paths, hashes, and reason codes — no file
  bodies (ADR-029 D8). Apply CLI (`runApplyExecuteChecked` in
  `internal/cli/cobra.go`) now surfaces `result.Warnings` on stderr
  with `⚠` prefix so the legacy advisory + Slice 4 downgrade note
  are operator-visible. 11 test functions in
  `writefile_safety_test.go`; empirical CLI verification of five
  paths (match / mismatch / legacy / new-file-empty-refuses / empty-
  literal-collision) all pass against a live tpatch binary.
- **Slice 3 — later-touch detection** (`c816769`): extended
  `writefile_safety.go` with `laterTouchIndex map[string]string`
  (path → offending slug), `loadLaterFeatureTouches` (RequestedAt
  ordering, alphabetical tie-break for determinism per PRD §5 note
  4), `collectFeatureTouchedPaths` (unions
  `patch-generations.json.touched_paths` per PRD §4.2 preferred
  artifact + `apply-recipe.json` op paths as fallback for early-
  lifecycle features), `checkLaterTouch`. Wave β dispatch tightens
  ADR-029 D6 from warn to REJECT at apply-time to close GH #1
  silent-revert scenarios — this delta is documented in the
  precheck docstring and the CHANGELOG bullet. 6 test functions in
  `writefile_later_touch_test.go` (one locks in that
  `SavePatchGenerations` fixtures need `Refs: &store.GenerationRefs{}`);
  empirical CLI verification: later-touch reject names the culprit
  feature slug.
- **Slice 4 — supersession coupling** (`9af8de8`): added
  `(*PreimagePrecheckResult).appendDrift(msg, superseded,
  superseder)` helper. Calls `IsFeatureSuperseded(s, recipe.Feature)`
  ONCE per recipe (amortized), then routes drift into Errors
  (baseline) or Warnings with downgrade suffix (when superseded).
  Suffix cites PRD §PRD-1-interaction + ADR-029 D7 verbatim. Path
  safety violations NEVER downgrade (security-boundary invariant).
  Inherits Wave α R4 runtime flip so stale-superseder scenarios
  also downgrade the historical target's drift. 6 test functions in
  `writefile_supersession_test.go` including
  `TestSlice4_StaleSupersederStillDowngrades` and
  `TestSlice4_SupersededPathSafetyStillHardReject`. Empirical CLI
  verification: superseded feature's stale-preimage drift becomes
  warning, execution proceeds.
- **Slice 5 — CHANGELOG + status flips** (`0d25e75`): amended
  `CHANGELOG.md` with `### Wave β` sub-section under existing
  `## v0.12.0 — TBD` heading (Wave α bullets untouched); flipped
  `docs/prds/PRD-write-file-recipe-safety.md` and `docs/adrs/
  ADR-029-write-file-recipe-safety.md` line 3 `Proposed` →
  `Accepted`; updated `docs/adrs/README.md` ADR-029 status column;
  updated `docs/ROADMAP.md` Wave β marker 🚧 → ✅ with per-slice
  acceptance summary bullets.

### Files Changed

New files:
- `internal/workflow/writefile_safety.go`
- `internal/workflow/writefile_safety_test.go`
- `internal/workflow/writefile_later_touch_test.go`
- `internal/workflow/writefile_supersession_test.go`

Modified files:
- `internal/workflow/implement.go` (added `PreimageHash *string`)
- `internal/workflow/recipe.go` (wired precheck into
  `ExecuteRecipe` + `DryRunRecipe`)
- `internal/cli/cobra.go` (⚠ stderr warning surface in
  `runApplyExecuteChecked`)
- `assets/assets_test.go` (added `preimage-hash-field` anchor)
- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/windsurf/windsurfrules`
- `assets/workflows/tessera-patch-generic.md`
- `CHANGELOG.md`
- `docs/prds/PRD-write-file-recipe-safety.md`
- `docs/adrs/ADR-029-write-file-recipe-safety.md`
- `docs/adrs/README.md`
- `docs/ROADMAP.md`
- `docs/handoff/CURRENT.md` (this update)

### Test Results

```
ok  	github.com/tesseracode/tesserapatch/assets	0.655s
ok  	github.com/tesseracode/tesserapatch/internal/buildinfo	0.943s
ok  	github.com/tesseracode/tesserapatch/internal/cli	65.969s
ok  	github.com/tesseracode/tesserapatch/internal/gitutil	11.794s
ok  	github.com/tesseracode/tesserapatch/internal/provider	15.349s
ok  	github.com/tesseracode/tesserapatch/internal/safety	2.365s
ok  	github.com/tesseracode/tesserapatch/internal/store	3.527s
ok  	github.com/tesseracode/tesserapatch/internal/tools/studyvalidator	2.974s
ok  	github.com/tesseracode/tesserapatch/internal/workflow	40.188s
ok  	github.com/tesseracode/tesserapatch/tests/integration	2.870s
```

All packages PASS. 23 new `TestSlice[234]_*` functions all green.
`gofmt -l .` empty. `go vet ./...` clean. `go build ./cmd/tpatch`
clean. Side Research md5 preserved:
`b385fe622db9926f48861105239f113e`.

## Next Steps

1. Dispatch internal + supervisor-external reviewers in parallel for Wave β rev-0.
2. User's parallel external pass on rev-0.
3. On three-way APPROVED: archive Wave β, dispatch Wave γ (active-feature-session).

## Blockers

None.

## Context for Next Agent

- HEAD at Wave β dispatch: `a05a918` (main, all 8 Wave α +
  consolidation commits pushed). Wave β commits landed on top,
  NOT pushed: `639efb2` (Slice 1) → `329f009` (Slice 2) →
  `c816769` (Slice 3) → `9af8de8` (Slice 4) → `0d25e75` (Slice 5)
  → this handoff commit.
- Wave α acceptance is on `main` — Wave β freely uses
  `store.DependencyKindSupersedes`, `store.ErrMultipleActiveSuperseders`,
  `IsFeatureSuperseded` from Wave α without re-export or refactor
  (halt condition R3 did not trigger).
- 20 binding carry-forward rules; every Wave β commit carries the
  Copilot co-author trailer (verified with
  `git log -1 --format='%(trailers:key=Co-authored-by)'` per
  commit). Every commit changing the exported recipe-execute
  path cites PRD acceptance criteria + ADR-029 D-clauses in the
  body (Rule 19).
- Two-opinion protocol scoreboard: 19/19 final-acceptance three-way
  concurrence going into Wave β.
- **Non-obvious decisions to know before review**:
  - `PreimageHash *string` (pointer, not plain string) is
    load-bearing: PRD §3.3 needs to distinguish nil (legacy,
    warn) from `""` (explicit new-file gate, refuse if target
    exists). Go's `string` decodes both cases identically.
  - Wave β dispatch tightens ADR-029 D6 later-touch from
    warn-only to REJECT-class AT APPLY TIME. The ADR itself
    still reads "warn" in the current tree (unchanged by Wave β);
    the delta is documented in the `runWriteFilePreimagePrecheck`
    docstring and in the CHANGELOG bullet. If the reviewer flags
    this as ADR drift, that is expected — the dispatch is the
    binding source, not the ADR text.
  - Supersession downgrade is a severity flip, NOT a bypass. Drift
    is still detected, formatted, and delivered; only the routing
    changes (Warnings vs Errors) so execution can proceed. The
    warning suffix carries the superseder slug + PRD/ADR citation
    verbatim for audit.
  - Path-safety violations (from `EnsureSafeRepoPath`) NEVER
    downgrade regardless of supersession — this is a security
    boundary invariant. `TestSlice4_SupersededPathSafetyStillHardReject`
    locks it in.
  - CLI warning surface: `runApplyExecuteChecked` in
    `internal/cli/cobra.go` now prints `result.Warnings` on
    stderr with `⚠` prefix. Before Wave β there was no operator-
    visible surface for advisory-class findings.
  - Pre-existing unstaged changes to `docs/CLUSTERS.md`,
    `docs/state-of-the-art/*`, `docs/whitepapers/*`, and untracked
    `docs/prds/PRD-feature-unapply.md`, `docs/prds/PRD-recurring-
    patches.md`, `docs/whitepapers/WP-004.md`..`WP-007.md` and
    their `.turns.md` companions are UNRELATED to Wave β and were
    deliberately excluded from every Wave β commit via targeted
    `git add`. The supervisor should confirm the diff shows only
    Wave β touches.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`
  (preserved throughout Slices 1–5 + this handoff).

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
