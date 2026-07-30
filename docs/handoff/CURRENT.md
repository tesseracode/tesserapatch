# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-wave-beta-writefile-safety-implementation`
- **Milestone**: v0.12.0 Wave β — implement `PRD-write-file-recipe-safety` + `ADR-029-write-file-recipe-safety`.
- **Description**: Second wave of the v0.12.0 3-wave sequential cluster. Adds `preimage_hash` field + later-touch detection to `write-file` recipe ops, protecting against silently reverting later fixes. Couples to Wave α supersession via PRD-feature-supersession §4.5 (superseded features downgrade write-file drift).
- **Status**: Rev-1 dispatched — 2 BLOCKING + 1 MEDIUM + 2 LOW folded in from rev-0 dual review split.
- **Assigned**: 2026-07-29.

## Wave β scope (locked)

**Read these first, in order**:

1. `docs/prds/PRD-write-file-recipe-safety.md` (233 lines, 13 acceptance criteria) — the spec.
2. `docs/adrs/ADR-029-write-file-recipe-safety.md` (108 lines, D1–D8) — the locked model. `sha256:<hex>` deliberately distinguished from `pg_/re_/rr_<12hex>` record identities.
3. `docs/prds/PRD-feature-supersession.md` §4.5 + Wave α acceptance in `docs/handoff/HISTORY.md` — the coupling contract. Wave α now correctly excludes stale-superseded features from replay; Wave β must downgrade write-file drift on features suppressed by supersession.
4. Prior Wave α diff (`7081c62..e5e0091`) for pattern reference — especially `internal/store/validation.go` (write-time rejection pattern from Slice R3) and `internal/workflow/labels.go` (severity-ordered emit + IsSupersessionLabel prefix-match pattern).

**What ships**:

- New `preimage_hash: sha256:<hex>` field on `write-file` recipe ops. Raw `sha256:` prefix (NOT truncated like `pg_/re_/rr_`).
- Preimage-hash precondition check before write-file execution: reject if the current file content hash does not match `preimage_hash`.
- Later-touch detection: if the target file has been touched by a later feature (via commit graph or manifest), reject with actionable message.
- Schema addition ripples through 6 shipped skill assets — parity guard (`TestSkillRecipeSchemaMatchesCLI` or equivalent) MUST update in the same commit (Slice 1 anti-drift lesson).
- Supersession coupling: superseded features (via Wave α's `isFeatureSupersededIn`) downgrade write-file drift severity per PRD-feature-supersession §4.5.
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
   - When the recipe op belongs to a superseded feature (via `isFeatureSupersededIn`), downgrade preimage-hash mismatch + later-touch drift severity per PRD-feature-supersession §4.5. Do NOT bypass the checks entirely — downgrade to warning-with-note instead of hard-reject.
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

Wave α accepted 2026-07-29 at three-way concurrence (19th scoreboard entry). Wave β dispatched 2026-07-29. Wave β implementer executed Slices 1–5 to completion in a single sitting; rev-0 hand-back triggered a dual review split (internal BLOCKED, supervisor-external APPROVED-with-info). Supervisor adjudicated with verbatim ADR-029 D6 + PRD §7.2 contract text at `e8d351f`, siding with the internal reading, and dispatched rev-1 to fold in the 2 BLOCKING + 1 MEDIUM + 2 LOW findings. Rev-1 executed Slices R1–R6 in a single sitting; ready for the second three-way review.

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
  Note (F-L1): the Slice 1 commit BODY described the schema field
  using a plain `string` type; the shipped code uses `*string` per
  PRD §3.3. Doc-only discrepancy; no commit rewrite (Rule 18
  immutability); recorded here for reviewer context.
- **Slice 2 — preimage precondition + reject path** (`329f009`):
  new `internal/workflow/writefile_safety.go` with sentinels
  (`ErrWriteFilePreimageMismatch`, `ErrWriteFileLaterTouch`),
  constants (`PreimageHashPrefix = "sha256:"`, `PreimageHashHexLen
  = 64`), helpers (`computeFileSHA256`, `checkWriteFilePreimage`,
  `isLowercaseHex`), and orchestrator
  `runWriteFilePreimagePrecheck` returning
  `PreimagePrecheckResult{Errors, Warnings}`. Wired precheck into
  `ExecuteRecipe` + `DryRunRecipe` in `internal/workflow/recipe.go`
  ahead of any op mutation (ADR-029 D3 all-or-nothing).
- **Slice 3 — later-touch detection** (`c816769`) — REV-1 REVISED:
  the rev-0 implementation tightened ADR-029 D6 from warn to REJECT
  at apply time. Rev-1 Slice R1 reverted that tightening to match
  the Accepted contract; detection stays intact and the operator
  still sees the drift, but execution proceeds. See the R1 entry
  below for details.
- **Slice 4 — supersession coupling** (`9af8de8`): added
  `(*PreimagePrecheckResult).appendDrift(msg, superseded,
  superseder)` helper. Calls `IsFeatureSuperseded(s, recipe.Feature)`
  ONCE per recipe (amortized), then routes drift into Errors
  (baseline) or Warnings with downgrade suffix (when superseded).
  Suffix cites PRD-feature-supersession §4.5 + ADR-029 D7. Path
  safety violations NEVER downgrade (security-boundary invariant).
  6 test functions in `writefile_supersession_test.go`.
- **Slice 5 — CHANGELOG + status flips** (`0d25e75`): amended
  `CHANGELOG.md` with `### Wave β` sub-section under existing
  `## v0.12.0 — TBD` heading (Wave α bullets untouched); flipped
  `docs/prds/PRD-write-file-recipe-safety.md` and `docs/adrs/
  ADR-029-write-file-recipe-safety.md` line 3 `Proposed` →
  `Accepted`.

### Wave β rev-1 — per-slice results (all committed, none pushed)

- **Slice R1 (F-B1 revert)** (`ec98499`): apply-time later-touch
  reverted to warning-class per ADR-029 D6 verbatim + PRD §7.2
  verbatim. Split the drift router: `appendDrift` still routes
  preimage-mismatch (Errors if not superseded, Warnings if
  superseded); new `appendLaterTouchWarn` ALWAYS routes to Warnings
  and carries the Slice 4 downgrade suffix when the feature is
  superseded so audit-trail uniformity across the two drift
  classes is preserved. Renamed and reworked the Slice 3 tests:
  `TestSlice3_LaterTouchWarnsAndProceeds` replaces the old
  `TestSlice3_LaterTouchDetectsAndRefuses`; new positive
  `TestSlice3_LaterTouchWarnsWithoutSupersession` locks the non-
  superseded warning path (no downgrade suffix). All 6 Slice 4
  supersession tests still green. Empirical CLI check: `apply older
  --mode execute` prints the `⚠ recipe drift` warning naming the
  later feature, exits 0, and the file IS overwritten.
- **Slice R2 (F-B2 AC-7)** (`2b64176`): record-time later-touch
  warning wired into `internal/cli/cobra.go` recordCmd RunE after
  `AppendPatchGenerationForFeature`. New detector
  `DetectRecordLaterTouchWarnings(s, slug)` in `writefile_safety.go`
  scans older active features whose `apply-recipe.json` owns a
  write-file at paths the newly-recorded feature touched. Warnings
  emit on stderr with `⚠` prefix, sorted deterministically by path
  (PRD §5 note 4) with alphabetical-first older slug per path.
  5 regression tests in `writefile_record_later_touch_test.go`.
  Empirical: record CLI prints `⚠ later-touch warning: [newer]
  touches src/a.txt which is whole-file-owned by older active
  feature "older"...`, exit 0.
- **Slice R3 (F-B2 AC-8)** (`7597ddd`): reconcile-time later-touch
  warning attached to each older (write-file owner) feature's
  `ReconcileResult.Notes`. New helper
  `DetectReconcileLaterTouchWarningsByOwner(s, slugs) map[owner]
  []string` in `writefile_safety.go` shares the detection primitives
  with R2 (no duplicated logic). Wired into `RunReconcile` after the
  supersession filter builds the effective slug set, before the per-
  slug reconcile loop. Warnings prepend to `result.Notes` so they
  appear alongside the Wave α historical-feature warning. 6 tests in
  `writefile_reconcile_later_touch_test.go`, including a Rule 20
  end-to-end proof via `setupGitRepo` + `RunReconcile`.
- **Slice R4 (F-B2 AC-9)** (`d50a852`): new V-check
  `CheckWriteFilePreimageFresh = "write_file_preimage_fresh"` (V10)
  in `internal/workflow/verify.go`. `checkWriteFilePreimageFresh`
  helper scans each write-file op's `preimage_hash` against the
  current on-disk file, reusing the Slice 2 `checkWriteFilePreimage`
  primitive so verify and apply agree on outcome by construction.
  Deliberately bypasses `replayRecipeOpsInShadow` — that shadow
  root has no `.tpatch/` and cannot host a preimage check that needs
  the CURRENT real on-disk file. Effective feature + stale preimage
  → SeverityBlock failure (PRD §7.2 v1 rule "v1 blocks only on
  preimage mismatch"). Superseded feature → SeverityWarn downgrade
  per ADR-029 D7 + Slice 4 supersession-controls-severity coupling.
  Frozen check-count grows from 10 → 11 (schema-additive; existing
  switches unaffected). `stubChecksAfterAbort` extended to include
  V10 stub. Existing verify tests updated to expect 11 entries.
  4 regression tests in `writefile_verify_preimage_test.go`.
- **Slice R5 (F-M1)** (`4b5f8e5`): sentinel-error wrap-and-return via
  `errors.Is`. `PreimagePrecheckResult` gains twin fields
  `WrappedErrors []error` + `WrappedWarnings []error`; the string
  `Errors`/`Warnings` retain existing shape for backward compat.
  `appendDrift` wraps ErrWriteFilePreimageMismatch;
  `appendLaterTouchWarn` wraps ErrWriteFileLaterTouch. On the D7
  supersession downgrade the drift class stays preimage-mismatch
  (severity-only flip). 4 regression tests in
  `writefile_sentinel_wrap_test.go` exercise: effective preimage-
  mismatch, effective later-touch, superseded preimage downgrade,
  and sentinel identity distinctness.
- **Slice R6 (F-L2 + doc refresh)** (this commit): replaced the
  `§PRD-1-interaction` shorthand with the literal anchor
  `PRD-feature-supersession §4.5` across Go source, Go tests, the
  CHANGELOG Wave β bullets, and the ROADMAP. Runtime warning suffix
  now reads `... per PRD-feature-supersession §4.5 / ADR-029 D7`.
  The heading text at that anchor is "Reconcile interaction with
  write-file safety". Also appended a `#### Wave β rev-1
  amendments` sub-section to the CHANGELOG under
  `## v0.12.0 — TBD` documenting all six slices; Wave α bullets
  untouched.

### Files Changed

New files (rev-1):
- `internal/workflow/writefile_record_later_touch_test.go`
- `internal/workflow/writefile_reconcile_later_touch_test.go`
- `internal/workflow/writefile_verify_preimage_test.go`
- `internal/workflow/writefile_sentinel_wrap_test.go`

Modified files (rev-1):
- `internal/workflow/writefile_safety.go` (R1 router split, R2/R3
  detectors, R5 wrapped-error fields + wrap in routers, R6 anchor
  fix)
- `internal/workflow/writefile_later_touch_test.go` (R1 warn-and-
  proceed rewrite)
- `internal/workflow/writefile_supersession_test.go` (R6 anchor
  assertion update)
- `internal/cli/cobra.go` (R2 record-time warning wire-up)
- `internal/workflow/reconcile.go` (R3 later-touch attachment to
  ReconcileResult.Notes)
- `internal/workflow/verify.go` (R4 V10 const, helper, RunVerify
  wire-up, stubChecksAfterAbort extension)
- `internal/workflow/verify_test.go` (R4 check-count 10→11)
- `CHANGELOG.md` (R6 anchor fix + rev-1 amendments sub-section)
- `docs/ROADMAP.md` (R6 anchor fix)
- `docs/handoff/CURRENT.md` (this update)

### Test Results

Full-suite pass at rev-1:

```
ok  	github.com/tesseracode/tesserapatch/assets	1.035s
ok  	github.com/tesseracode/tesserapatch/internal/buildinfo	2.010s
ok  	github.com/tesseracode/tesserapatch/internal/cli	148.209s
ok  	github.com/tesseracode/tesserapatch/internal/gitutil	20.335s
ok  	github.com/tesseracode/tesserapatch/internal/provider	21.277s
ok  	github.com/tesseracode/tesserapatch/internal/safety	4.684s
ok  	github.com/tesseracode/tesserapatch/internal/store	4.297s
ok  	github.com/tesseracode/tesserapatch/internal/tools/studyvalidator	10.664s
ok  	github.com/tesseracode/tesserapatch/internal/workflow	104.557s
ok  	github.com/tesseracode/tesserapatch/tests/integration	9.289s
```

19 new `TestSliceR[1-5]_*` regression tests all green (5 + 6 + 4 + 4
= 19). Baseline was 806 top-level PASSes at rev-0; rev-1 total is
comfortably above the +10-15 gate target. `gofmt -l .` empty. `go
vet ./...` clean. `go build ./cmd/tpatch` clean. Wave α files
`internal/workflow/labels.go` and `internal/store/validation.go`
unmodified end-to-end since dispatch (`git diff --stat
e8d351f..HEAD -- <files>` shows 0). Side Research md5 preserved:
`b385fe622db9926f48861105239f113e`.

## Next Steps

1. Dispatch internal + supervisor-external rev-1 reviewers in parallel.
2. On three-way APPROVED: archive Wave β, dispatch Wave γ (active-feature-session).

## Blockers

**RESOLVED — supervisor adjudication 2026-07-29: rev-1 dispatched.**

**Supervisor decision on the D6/§7.2 split**: internal-reviewer reading is authoritative. External read ADR-029 D6 alone as silent on apply-time later-touch, but PRD §7.2 verbatim states "v1 blocks only on preimage mismatch" — the combined ADR+PRD contract is unambiguous. Slice 3's warn→reject tightening is Rule 19-class scope beyond Accepted contract.

**Rev-1 chosen approach**: hybrid of internal Options 1 + 3, with F-B2 addressed by IMPLEMENTING the missing ACs (not scoping the PRD down). Rationale: AC-7/8/9 are v1 contract; skipping them would defer scope originally locked in Wave β.

### Rev-1 scope

**F-B1 fix (BLOCKING)** — revert Slice 3 apply-time later-touch refusal to warning-class:
- File: `internal/workflow/writefile_safety.go:268-270` (current refusal path).
- Change: emit warning via `appendDrift` route, do NOT halt execution. Detection stays.
- Preserve Slice 4's supersession-downgrade routing (superseded → further downgraded / silenced per PRD-feature-supersession §4.5).
- Update `writefile_later_touch_test.go` regression assertions from "reject" to "warn-and-proceed"; add new positive test locking apply-time-warn semantics.
- Rule 19 clause to cite: ADR-029 D6 + PRD §7.2.

**F-B2 fix (BLOCKING)** — implement missing PRD ACs 7 + 8 + 9:
- **AC-7 (record later-touch warning)**: extend `internal/workflow/record*.go` to detect when a newly-recorded feature's touched paths overlap an older active feature's `write-file` operation. Emit deterministic warning naming both features + shared path. Regression test.
- **AC-8 (reconcile later-touch warning)**: extend `internal/workflow/reconcile.go` to report when a later active/effective feature touched a path owned by an older `write-file` op. Warning-class per PRD §5:129. Reuse Slice 3's detector where possible. Regression test.
- **AC-9 (verify stale-preimage check)**: extend verify V-checks (`internal/workflow/verify.go`) to add a stale-preimage check on `write-file` ops. Effective-feature stale preimage = failure-class; superseded-feature stale preimage = warning-class (per D7 + Slice 4 pattern). The `replayRecipeOpsInShadow` bypass at `verify.go:1148` needs to route through `ExecuteRecipe` or use a dedicated stale-check helper. Regression tests for both severity classes.

**F-M1 fix (MEDIUM)** — sentinel errors `ErrWriteFilePreimageMismatch` + `ErrWriteFileLaterTouch`:
- Currently declared but never returned; `errors.Is` claim in docstring/CHANGELOG is false.
- Choose: either (a) actually wrap returned errors so `errors.Is` works and add regression test; or (b) delete the sentinels and fix docstring/CHANGELOG claim.
- Prefer (a) — sentinels are useful downstream for callers matching drift types.

**F-L1 fix (LOW)** — Slice 1 commit-body signature vs shipped `*string`:
- Doc-only fix, address at rev-1 handoff refresh.

**F-L2 fix (LOW)** — `§PRD-1-interaction` shorthand:
- Replace with real anchor `PRD-write-file-recipe-safety §5.7` (or the exact section title). Grep all Wave β commits + CHANGELOG + skill assets.

### Rev-1 slice plan

1. **Slice R1**: F-B1 revert apply-time later-touch to warn-class. Update regression tests. (Smallest, most contained.)
2. **Slice R2**: F-B2 AC-7 record-command later-touch warning.
3. **Slice R3**: F-B2 AC-8 reconcile-command later-touch warning.
4. **Slice R4**: F-B2 AC-9 verify stale-preimage check.
5. **Slice R5**: F-M1 sentinel-error wrap-and-return.
6. **Slice R6**: CHANGELOG rev-1 amendment + F-L1/L2 doc corrections + handoff refresh. PRD + ADR remain at Accepted (rev-1 closes the contract-vs-code gap; no need to revert to Proposed).

### Rev-1 validation gates

- Full gate set (gofmt / vet / build / test suite).
- Baseline 806 top-level PASS at rev-0. Rev-1 total MUST be ≥ 806 + 10-15 (F-B1 warn/positive + AC-7 + AC-8 + AC-9 effective/superseded + F-M1 errors.Is).
- Parity guard test still passes (schema stable, no field changes in rev-1).
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.
- Rule 18 trailer on every commit.

Wave α behavior non-invalidation still confirmed (rev-0 did not touch Wave α files; rev-1 must not either).

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
