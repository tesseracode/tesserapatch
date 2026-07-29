# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-wave-alpha-supersession-implementation`
- **Milestone**: v0.12.0 Wave α — implement `PRD-feature-supersession` + `ADR-028-supersession-edge-model`.
- **Description**: First wave of the v0.12.0 3-wave implementation cluster (supervisor picked Option A 2026-07-29). Implements the third-kind `depends_on[].kind: "supersedes"` edge on the ADR-011 dependency graph, 4 new composable labels, and reconcile suppression of superseded features. Extends (does NOT fork) the ADR-011 D1 storage lane.
- **Status**: Rev-1 dispatched — 4 findings fold-in (2 HIGH, 2 MEDIUM).
- **Assigned**: 2026-07-29.

## Rev-1 fold-in (from Wave α rev-0 dual review)

Wave α rev-0 landed at `480f90a` (Slices 1–5 + handoff). Internal review APPROVED WITH NOTES at `0aa6b81`; supervisor-external NEEDS REVISION at `4dc6c5d`. Four findings must land before rev-1 acceptance:

**F-SEXT-1 (HIGH) — missing `<slug>` suffix on `superseded-by` label**.
PRD §4.1:154-159 + §4.3:178 + ADR-028 D4:58 all lock the render format `superseded-by <slug>`. Current code emits bare `superseded-by` in both text (`internal/cli/status_dag.go:398-400`) and JSON (`internal/cli/status_dag.go:494-497`). The docstring at `internal/store/types.go:157` even acknowledges the render layer is supposed to add the slug — the code just doesn't.
**Fix**: extend `DeriveSupersessionLabels` (or the render sites) to emit `superseded-by <slug>` where `<slug>` is the active superseder slug. When multiple healthy superseders exist AFTER F-SEXT-3 rejection lands, this becomes moot (validation refuses). During the transition, pick the first healthy superseder deterministically (sorted slug) to keep tests stable.
**Test**: add regression test emitting text + JSON status for a superseded feature and asserting the slug is present.

**F-SEXT-2 (HIGH) — label render order alphabetical, not PRD-locked severity order**.
PRD §4.3:184-188 + ADR-028 D4:63-67 lock the order `[stale-superseder] [orphan-superseder] [superseded-by <slug>] [active-superseder]`. Current code (`internal/workflow/labels.go:546`) sorts alphabetically, producing `active-superseder, orphan-superseder, stale-superseder` (inverted for two of four positions).
**Fix**: replace the alphabetical sort with an explicit severity-ordered emit. Suggest a small ordered slice of the 4 labels (stale, orphan, superseded-by, active) and emit in that order when present. Preserve alphabetical sort for OTHER label families if they exist — only supersession labels need the fixed order.
**Test**: regression test asserting exact label order in text output.

**F-SEXT-3 / Internal F2 (MEDIUM) — multi-active-superseder not detected (AC-4 / ADR-028 D5)**.
`isFeatureSupersededIn` (`internal/workflow/labels.go:419-436`) returns first-healthy-wins; `composeSupersessionLabels` set-dedupes to a single `superseded-by`; validation has no fan-in scan. Two healthy superseders + one target currently produces zero errors/warnings/conflict labels. PRD AC-4 + ADR-028 D5 require REJECTION at write-time.
**Fix**: add a write-time validation pass that scans `depends_on[]` fan-in for each feature — if more than one healthy (non-stale, non-orphan) superseder points at the same target, `Save`/`Accept` MUST reject with an actionable error naming both superseders. This is a validation-layer rejection, not a label-composition change.
**Test**: regression test constructing two healthy superseders → one target and asserting `Accept` (or the write path) errors with the AC-4/D5 message.

**Internal F1 (MEDIUM) — stale-superseder docs↔runtime contradiction**.
`composeSupersessionLabels` docstring (`internal/workflow/labels.go:456-458`) + PRD §4.5.3 both claim "stale → historical stays excluded". But `isFeatureSupersededIn` returns `false` for stale, so `RunReconcile` default-set filter + V7 `runClosureReplay` supersession-skip both KEEP the historical target IN the effective set. Two tests currently LOCK the contradictory runtime: `TestReconcileDefaultSet_KeepsFeatureWhenSupersederStale` + `TestRunVerify_ClosureReplay_StaleSupersederDoesNotSkipParent`.
**Decision — runtime flip, not paper reconciliation**. PRD §4.5.3 is `Accepted`; the ADR-028 lock chain is a stronger contract than the current runtime, and the docstring already agrees with the PRD. Flip runtime: for stale superseders, treat historical target as EXCLUDED from default-set replay (matching docstring + PRD). Update the two locking tests to assert the new (correct) behavior. Add a positive regression test for the flipped semantics.
**Rule 19 note**: `RunReconcile` and V7 are shipped exported surface — cite PRD §4.5.3 + ADR-028 D-clause in the commit.

## Rev-1 slice plan

1. **Slice R1 — F-SEXT-1 slug suffix** (labels + status render + regression test). Smallest, most contained.
2. **Slice R2 — F-SEXT-2 severity order** (labels emit + regression test).
3. **Slice R3 — F-SEXT-3 multi-active-superseder rejection** (validation-layer + regression test). Adds new error path — ensure error message is actionable per ADR-020 inline-remediation style.
4. **Slice R4 — Internal F1 runtime flip** (labels + reconcile + verify V7; UPDATE two existing locking tests; ADD positive regression test).
5. **Slice R5 — CHANGELOG amendment + handoff update**. Amend `## v0.12.0 — TBD` entry to reflect rev-1 corrections. Cite ADR-011 D1–D4 non-invalidation preserved (no fork).

## Rev-1 validation gates

Same as rev-0 (gofmt, vet, build, full test suite). PLUS:

- After Slice R3, expect the two rev-0 locking tests for stale supersession to be UPDATED (not deleted). They must assert the corrected behavior.
- After Slice R4, expect at least 2 new regression tests for stale-supersession positive path.
- Full test count MUST be >= rev-0 count + 4-6 (F-SEXT-1 + F-SEXT-2 + F-SEXT-3 + 2 stale positive).
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.
- Rule 18 trailer on every commit (structural parse via `git interpret-trailers`).

## Wave α scope (locked)

**Read these first, in order**:

1. `docs/prds/PRD-feature-supersession.md` (259 lines, 12 acceptance criteria) — the spec.
2. `docs/adrs/ADR-028-supersession-edge-model.md` (109 lines, D1–D8) — the locked model.
3. `docs/adrs/ADR-011-feature-dependencies.md` (D1–D4) — what you are extending. D1 storage lane (`status.json.depends_on[]`) is authoritative. D2 DFS cycle detection extends cleanly to the third edge kind. D3 composable-derived-labels pattern is what the 4 new labels attach to. D4 hard/soft semantics unchanged.

**What ships**:

- New third `kind: "supersedes"` value on `depends_on[]` entries, alongside existing `hard` and `soft`. Directed edge: `X supersedes Y` means X replaces Y.
- Reconcile suppression: superseded features are excluded from replay by default (both hard-parent replay in verify V7 and the general reconcile pass).
- 4 new composable labels on the ADR-011 D3 pattern: `superseded-by`, `active-superseder`, `stale-superseder`, `orphan-superseder`.
- Cycle detection extends via ADR-011 D2's existing DFS (X supersedes Y + Y supersedes X = cycle).
- `depends_on[].kind` schema addition ripples through skill assets — parity guards must update in the same commit.

**What does NOT ship in Wave α**:

- `write-file` recipe safety (`preimage_hash`, later-touch detection) → Wave β.
- Active feature session lane (`tpatch session` command group, `.tpatch/local/capture/`) → Wave γ.

## Cross-wave coordination

- **Wave β depends on Wave α**: `PRD-write-file-recipe-safety §PRD-1-interaction` says superseded features suppress/downgrade `write-file` drift on the parent verify path. Wave β cannot dispatch until Wave α acceptance.
- **Wave γ is independent** of Waves α/β. It touches `internal/cli/init.go`, new `tpatch session` command group, and a new `.tpatch/local/capture/` storage lane.

## Implementer directives (Wave α)

**Approach**: sequential slices per PRD acceptance criteria groupings. Recommended shape:

1. **Slice 1 — Schema + parity guards** (foundation, must land first):
   - Add `kind: "supersedes"` as a third valid value on `depends_on[]` in `internal/store/status.go` (or wherever the type lives).
   - Update `TestSkillRecipeSchemaMatchesCLI` / any dependency-schema parity guards in the SAME commit (Slice 1 anti-drift lesson from doctor Wave β).
   - Update the 6 shipped skill assets (`assets/*/SKILL.md` + prompt/recipe templates) so any documentation of `depends_on[].kind` mentions the new value.
2. **Slice 2 — Graph + cycle detection extension**:
   - Extend ADR-011 D2 DFS cycle detection to include supersedes edges (X supersedes Y + Y supersedes X = cycle).
   - Add unit tests for the mixed hard/soft/supersedes cycle cases.
3. **Slice 3 — 4 composable labels**:
   - Add `superseded-by`, `active-superseder`, `stale-superseder`, `orphan-superseder` to the ADR-011 D3 label composition pattern.
   - `tpatch status` / label rendering must surface these.
4. **Slice 4 — Reconcile suppression**:
   - `internal/workflow/reconcile.go` and verify V7 (`runClosureReplay` in `internal/workflow/verify.go`) exclude superseded features from the hard-parent closure by default.
   - Add regression tests for both paths.
5. **Slice 5 — CHANGELOG + docs**:
   - Add `## v0.12.0 — TBD` header + Wave α entry.
   - Flip `PRD-feature-supersession.md` and `ADR-028` status from `Proposed` → `Accepted`.
   - Update `docs/ROADMAP.md` v0.12.0 entry.

**Binding carry-forward rules** (all 20):

- **Rule 15**: any `tpatch` command referenced in docs must exist in `internal/cli/cobra.go`.
- **Rule 18**: every commit MUST carry a parseable `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer (verify via `git log -1 --format='%(trailers)'`).
- **Rule 19**: any diff touching exported `store`/`workflow`/`cli` surface MUST be justified by a PRD/ADR clause. Wave α touches store schema (kind value) + workflow (reconcile + verify) — both are PRD-scoped, but call out the exact clause in each commit message.
- **Rule 20**: for any user-facing CLI behavior change (label rendering, reconcile output, verify remediation strings), reproduce empirically in a scratch repo before shipping. Rigor extension optional but recommended for the reconcile-suppression regression tests.
- Full 20-rule list carry-forward from v0.11.3 close in HISTORY.md snapshots.

**Validation gates** (must pass before handoff back for review):

- `gofmt -l .` (empty output).
- `go vet ./...` (clean).
- `go build ./cmd/tpatch` (clean).
- `go test ./...` (full suite pass; currently 99 tests as of v0.11.3).
- Parity guard test (`assets_test.go` or `TestSkillRecipeSchemaMatchesCLI`) must pass with schema addition.
- Side Research md5 preserved: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)` = `b385fe622db9926f48861105239f113e`.

## Wave sequencing plan (v0.12.0)

1. **Wave α — Supersession** (this task): PRD-feature-supersession + ADR-028. Foundation for graph model.
2. **Wave β — Write-file safety** (after α acceptance): PRD-write-file-recipe-safety + ADR-029. Depends on α for reconcile-suppression interaction.
3. **Wave γ — Active-feature-session** (after β acceptance, OR parallel with β if capacity allows): PRD-active-feature-session. Independent code surface.

Ship v0.12.0 when Waves α+β+γ all acceptance-approved. Doctor cluster's 4-wave pattern (v0.11.2) is the proven template.

## Non-blocking follow-ups deferred

- ADR-027 F2 (LOW): PRD-ide-capture-hooks naming coord.
- Doctor S3-boundary (LOW): mixed-CHANGELOG scope documentation.
- Stream B ADR-029 informational nit: literalize `TestSkillRecipeSchemaMatchesCLI` symbol in parity contract prose (post-v0.12.0 docs polish).

## Carry-forward dispatch rules (20 binding)

Same 20 rules as v0.11.3 close. See prior CURRENT.md snapshots in HISTORY.md for full text. Rule 20 rigor extension pattern (detached-worktree-at-pre-fix + test-copy) remains optional.

## Session Summary

v0.11.3 shipped 2026-07-29 (Stream C closed GH #2). Streams A + B closed at three-way APPROVED 2026-07-29 (paper-only PRD/ADR drafts). Supervisor picked Option A 2026-07-29 — v0.12.0 3-wave sequential cluster kicked off with Wave α (supersession) implementer dispatched.

**Wave α implementation session — 2026-07-29 → 2026-07-30 (5 slices landed)**

- **Slice 1 (`48399f4`)** — schema + parity guards. Added
  `store.DependencyKindSupersedes = "supersedes"` constant and four new
  `ReconcileLabel` constants (`LabelSupersededBy`, `LabelActiveSuperseder`,
  `LabelStaleSuperseder`, `LabelOrphanSuperseder`) in `internal/store/types.go`.
  Broadened `ValidateDependencies` + `ValidateAllFeatures` kind allow-lists in
  `internal/store/validation.go`. Broadened `parseDepSpec` in
  `internal/cli/feature_deps.go` to accept `<parent>:supersedes` shorthand.
  Updated all six shipped skill assets (Claude, Copilot, Copilot Prompt,
  Cursor, Windsurf, Generic) with a third `supersedes` bullet under
  "Edge kinds". Parity guards pass in the same commit (Slice 1
  anti-drift lesson honored).
- **Slice 2 (`f8f7766`)** — cycle detection extension. Confirmed ADR-011
  D2 `DetectCycles` is edge-kind-agnostic by construction (uses `dep.Slug`
  without inspecting `Kind`), so ZERO code change to the DFS primitive.
  Added `internal/store/supersedes_test.go` with 9 regression tests
  covering AC-1 (reciprocal cycle), AC-2 (mixed hard/soft/supersedes
  cycle), AC-3 (self-supersession), AC-8 (unknown-kind rejection),
  kind-conflict guard, linear chain happy path, and direct DFS primitive
  properties.
- **Slice 3 (`195921a`)** — four composable derived labels via ADR-011
  D3. Added `composeSupersessionLabels`, `IsSupersessionLabel`,
  `StripSupersessionLabels`, `stripDerivedLabels`, `supersederIsHealthy`,
  and exported render helper `DeriveSupersessionLabels` to
  `internal/workflow/labels.go`. Wired supersession-strip into 3
  persistence call sites (`accept.go:160`, `reconcile.go:376`,
  `reconcile.go:574`) via the `stripDerivedLabels` chain so freshness
  + supersession labels stay out of persisted `Reconcile.Labels`.
  Updated `renderNodeLine` / `renderNodeLineWithFreshness` /
  `writeDAGJSON` in `internal/cli/status_dag.go` to merge supersession
  labels into text + JSON output. Added
  `internal/workflow/labels_supersession_test.go` with 6 unit tests.
  **Rule 20 empirical verification**: scratch repo with
  `add-newer-feature supersedes add-older-feature` correctly rendered
  `superseded-by`, `active-superseder`, `orphan-superseder`, and
  `stale-superseder` in `tpatch status` output.
- **Slice 4 (`3f49c36`)** — reconcile suppression. Added
  `IsFeatureSuperseded` (exported) + `isFeatureSupersededIn` (pre-loaded
  features variant) helpers to `internal/workflow/labels.go`.
  `RunReconcile` now filters superseded-by-healthy features from the
  default effective replay set (empty slug input); when an explicit
  slug names a superseded feature, it is reconciled for audit/repair
  but a historical-feature warning note is prepended to the
  `ReconcileResult` (PRD §3.3). V7 `runClosureReplay` in
  `internal/workflow/verify.go` pre-loads all features once and skips
  superseded parents from the BFS closure — their recipes are not
  replayed in the shadow. Added 3 reconcile regression tests
  (`internal/workflow/reconcile_supersession_test.go`) and 2 V7
  regression tests (`internal/workflow/verify_supersession_test.go`)
  covering both happy-path suppression and the ADR-028 D8 stale
  superseder must-not-mask contract.
- **Slice 5 (`4d4bb60`)** — CHANGELOG + docs. Added
  `## v0.12.0 — TBD — feature supersession (Wave α)` header (em-dash
  U+2014) to `CHANGELOG.md` with Wave α entry describing schema,
  labels, and reconcile suppression, citing
  `PRD-feature-supersession` and `ADR-028-supersession-edge-model`.
  Flipped both PRD-feature-supersession.md and
  ADR-028-supersession-edge-model.md status from `Proposed` →
  `Accepted`. Added a `v0.12.0 — feature supersession Wave α
  implementation landed 🚧 in-flight` section to `docs/ROADMAP.md`
  above the existing planning-artifacts entry, describing Wave α
  results and the Wave β + Wave γ deferrals. Flipped ADR-028 status
  column in `docs/adrs/README.md` from `Proposed` → `Accepted`.

**Wave α rev-1 fold-in session — 2026-07-29 (5 slices landed atop `d21b4b4`)**

- **Slice R1 (`5e6515d`)** — F-SEXT-1 `superseded-by <slug>` composite.
  `composeSupersessionLabels` target-side scan now collects the
  healthy peer superseder slugs, sorts them, and emits the composite
  `ReconcileLabel("superseded-by " + slugs[0])` per PRD-feature-
  supersession §4.1 (binding label-value contract) + §4.3:178 +
  ADR-028 D4:58. `IsSupersessionLabel` widened to prefix-match on
  `"superseded-by "` so persistence strip (`StripSupersessionLabels`)
  stays sound regardless of the appended slug. Existing tests
  updated (`TestComposeLabels_ActiveSuperseder`,
  `TestComposeLabels_StaleSuperseder`) via new
  `hasSupersededByFor` / `hasAnySupersededBy` helpers. Three new
  tests: `TestComposeLabels_SupersededByCarriesSlug` (workflow),
  `TestStatusDag_SupersededByCarriesSlugText` (cli),
  `TestStatusDag_SupersededByCarriesSlugJSON` (cli).
- **Slice R2 (`84c873a`)** — F-SEXT-2 severity-first render order.
  `DeriveSupersessionLabels` replaces the alphabetical `sort.Slice`
  with an explicit severity-ordered emit — `[stale-superseder,
  orphan-superseder, superseded-by <slug>, active-superseder]` —
  per PRD §4.3:184-188 + ADR-028 D4:63-67. Added
  `appendLabelPreserveOrder` helper in `internal/cli/status_dag.go`
  so both `renderNodeLineWithFreshness` and `writeDAGJSON` merge
  supersession labels while preserving the fixed order (other label
  families keep their alphabetical sort via `appendLabel`). Two new
  tests: `TestDeriveSupersessionLabels_SeverityOrder` (workflow),
  `TestStatusDag_SupersessionLabelsSeverityOrder` (cli — asserts
  positional order in the text output).
- **Slice R3 (`a7f0222`)** — F-SEXT-3 multi-active-superseder
  rejection. New sentinel `store.ErrMultipleActiveSuperseders`,
  plus a helper `supersederIsHealthyForValidation` mirroring the
  workflow-side `supersederIsHealthy` byte-identically (comments
  note the byte-identity contract). `ValidateDependencies` now
  scans the store for existing healthy peers pointing supersedes at
  the same target and rejects the write with an ADR-020-style
  actionable message naming both superseder slugs + the shared
  target + a resolution hint. `ValidateAllFeatures` adds a bulk
  fan-in pass (deterministic ordering) to surface on-disk
  corruption via `tpatch status --dag`. Three new tests:
  `TestValidateDependencies_MultipleActiveSupersedersRejected`,
  `TestValidateDependencies_MultipleSupersedersOneStale`,
  `TestValidateAllFeatures_MultipleActiveSupersedersFlagged`.
- **Slice R4 (`4a7ea4f`)** — Internal F1 stale-supersession
  runtime flip. `isFeatureSupersededIn` now returns true for both
  healthy AND stale superseders per PRD §4.5.3 clause 3 + ADR-028
  D6/D8; docstrings on `IsFeatureSuperseded` +
  `isFeatureSupersededIn` + the V7 closure comment updated to
  describe the flipped semantics. The two rev-0 locking tests are
  RENAMED + assertion-inverted, NOT deleted:
  `TestReconcileDefaultSet_KeepsFeatureWhenSupersederStale` →
  `..._ExcludesFeatureWhenSupersederStale` (adds a bystander
  applied feature so the "empty default set" guard does not fire),
  and `TestRunVerify_ClosureReplay_StaleSupersederDoesNotSkipParent`
  → `..._StaleSupersederSkipsParent` (V7 now passes instead of
  failing with FailedAt=parent-replay). Two new positive
  regression tests: `TestReconcileDefaultSet_
  OrphanSupersederDoesNotExcludeUnrelated` (differentiates orphan
  from stale — orphan attaches only to the superseder side and
  does NOT cascade into excluding a bystander) and
  `TestIsFeatureSuperseded_StaleSupersederReturnsTrue` (locks the
  primitive helper's flipped semantics directly). ADR-011 D1-D4
  non-invalidation preserved (storage lane, DFS, Kahn, hard/soft
  gates all untouched).
- **Slice R5** — CHANGELOG amendment + handoff update. Amended
  the `## v0.12.0 — TBD` block to describe the four rev-1
  corrections (F-SEXT-1/2/3 + Internal F1) alongside the original
  Wave α bullets, keeping the em-dash contract. This handoff
  section refreshed with Session Summary + Files Changed + Test
  Results + Next Steps. Side Research md5 preserved.

## Files Changed

**New source files**:
- `internal/store/supersedes_test.go` (Slice 2)
- `internal/workflow/labels_supersession_test.go` (Slice 3)
- `internal/workflow/reconcile_supersession_test.go` (Slice 4)
- `internal/workflow/verify_supersession_test.go` (Slice 4)

**Modified source files**:
- `internal/store/types.go` — `DependencyKindSupersedes` constant + 4
  `ReconcileLabel` constants.
- `internal/store/validation.go` — kind allow-list broadened, error
  message updated.
- `internal/cli/feature_deps.go` — `parseDepSpec` accepts `:supersedes`.
- `internal/workflow/labels.go` — supersession label derivation,
  strip helpers, `IsFeatureSuperseded`/`isFeatureSupersededIn`.
- `internal/workflow/accept.go` — persist path uses `stripDerivedLabels`.
- `internal/workflow/reconcile.go` — default-set filter + explicit-slug
  historical warning; persist paths use `stripDerivedLabels`.
- `internal/workflow/verify.go` — V7 `runClosureReplay` skips
  superseded parents from the BFS closure.
- `internal/cli/status_dag.go` — supersession labels merged into
  DAG text + JSON output.

**Modified skill assets** (Slice 1):
- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/windsurf/windsurfrules`
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`
- `assets/workflows/tessera-patch-generic.md`

**Modified docs** (Slice 5):
- `CHANGELOG.md`
- `docs/prds/PRD-feature-supersession.md` (Proposed → Accepted)
- `docs/adrs/ADR-028-supersession-edge-model.md` (Proposed → Accepted)
- `docs/ROADMAP.md`
- `docs/adrs/README.md`

**Rev-1 fold-in — modified/new files** (Slices R1-R5, atop `d21b4b4`):

- `internal/workflow/labels.go` — R1 composite `superseded-by <slug>`
  emit + `IsSupersessionLabel` prefix match; R2 severity-first
  emit in `DeriveSupersessionLabels`; R4 `isFeatureSupersededIn`
  returns true for stale, docstrings updated. `strings` import added.
- `internal/workflow/labels_supersession_test.go` — R1 helpers
  (`hasSupersededByFor`, `hasAnySupersededBy`) + updated existing
  tests; new tests `TestComposeLabels_SupersededByCarriesSlug` (R1)
  and `TestDeriveSupersessionLabels_SeverityOrder` (R2).
- `internal/cli/status_dag.go` — R2 `appendLabelPreserveOrder`
  helper + both render sites use it for supersession labels.
- `internal/cli/status_dag_supersession_test.go` (new) — R1 text +
  JSON regression tests + R2 severity-order regression test.
- `internal/store/validation.go` — R3 `ErrMultipleActiveSuperseders`
  sentinel, `supersederIsHealthyForValidation` helper (byte-identical
  with workflow), fan-in check in `ValidateDependencies` +
  `ValidateAllFeatures`. `sort` + `strings` imports added.
- `internal/store/supersedes_test.go` — R3 three new regression tests
  appended.
- `internal/workflow/verify.go` — R4 V7 closure comment revised.
- `internal/workflow/reconcile_supersession_test.go` — R4 rename +
  invert of the stale-supersession locking test; +1 orphan-vs-stale
  positive test.
- `internal/workflow/verify_supersession_test.go` — R4 rename +
  invert of the V7 stale-supersession locking test.
- `CHANGELOG.md` — R5 amended `## v0.12.0 — TBD` block with a new
  "Rev-1 corrections" nested list documenting F-SEXT-1/2/3 +
  Internal F1.
- `docs/handoff/CURRENT.md` — R5 handoff refresh (this file).

## Test Results

Rev-1 final gate output (2026-07-29):

```
$ gofmt -l .
$ go vet ./...
$ go build ./cmd/tpatch
$ go test -count=1 ./...
ok  	github.com/tesseracode/tesserapatch/assets	0.538s
ok  	github.com/tesseracode/tesserapatch/internal/buildinfo	0.656s
ok  	github.com/tesseracode/tesserapatch/internal/cli	76.539s
ok  	github.com/tesseracode/tesserapatch/internal/gitutil	14.738s
ok  	github.com/tesseracode/tesserapatch/internal/provider	15.541s
ok  	github.com/tesseracode/tesserapatch/internal/safety	1.309s
ok  	github.com/tesseracode/tesserapatch/internal/store	5.472s
ok  	github.com/tesseracode/tesserapatch/internal/tools/studyvalidator	2.441s
ok  	github.com/tesseracode/tesserapatch/internal/workflow	51.450s
ok  	github.com/tesseracode/tesserapatch/tests/integration	4.188s
```

All packages pass; no diagnostics from `gofmt` or `go vet`.

Top-level `--- PASS` count via `go test -count=1 -v ./... | grep -c
'^--- PASS'`: **783** (rev-0 baseline: 773 → delta +10 tests). New
tests: R1 = 3 (one workflow, two CLI), R2 = 2 (one workflow, one
CLI), R3 = 3 (all store), R4 = 2 positive (both workflow). The two
rev-0 locking tests still exist under their inverted names and
assertions (renamed, not deleted, per the rev-1 dispatch protocol):
`TestReconcileDefaultSet_ExcludesFeatureWhenSupersederStale` and
`TestRunVerify_ClosureReplay_StaleSupersederSkipsParent`.

Rev-0 commits (5, all with valid Co-authored-by trailer):
- `48399f4` slice 1: schema + parity guards
- `f8f7766` slice 2: supersedes cycle detection tests
- `195921a` slice 3: 4 composable labels + rendering
- `3f49c36` slice 4: reconcile suppression + V7 supersession-skip
- `4d4bb60` slice 5: CHANGELOG + status flips

Rev-1 commits (5, all with valid Co-authored-by trailer):
- `5e6515d` slice R1: F-SEXT-1 superseded-by label carries slug suffix
- `84c873a` slice R2: F-SEXT-2 supersession labels render in severity-first order
- `a7f0222` slice R3: F-SEXT-3 reject multiple healthy superseders at write time
- `4a7ea4f` slice R4: Internal F1 stale-supersession runtime flip
- (this commit) slice R5: CHANGELOG amendment + handoff update

## Next Steps

1. Dispatch internal + supervisor-external rev-1 reviewers in parallel.
2. On three-way APPROVED: archive Wave α to `docs/handoff/HISTORY.md`, dispatch Wave β (`PRD-write-file-recipe-safety` + ADR-029).
3. Wave γ (`PRD-active-feature-session`) dispatch may proceed in parallel with Wave β if capacity allows (independent code surfaces).

## Blockers

None.

## Context for Next Agent

- HEAD after Wave α rev-1 fold-in: (this commit) atop `4a7ea4f`
  (which sits atop rev-0 tip `4d4bb60` → dispatch HEAD `d21b4b4`).
  Rev-1 landed 5 sequential commits (R1-R5), one per slice.
- **Rev-1 Rule 19 exported-surface citations landed**: R1
  (composite `superseded-by <slug>` label value contract), R2
  (`DeriveSupersessionLabels` render order), R3
  (`store.ErrMultipleActiveSuperseders` sentinel + validation),
  R4 (`isFeatureSupersededIn` / `IsFeatureSuperseded` behavior
  flip touching `RunReconcile` + `runClosureReplay`). Each commit
  message names the PRD AC + ADR D-clause.
- **Design decision — first-sorted-slug determinism (R1)**: when a
  superseded target has multiple healthy peers, the composite label
  emit chose `slugs[0]` after `sort.Strings`. This case is only
  transiently reachable because R3 rejects the multi-healthy write
  path at `ValidateDependencies` time; it exists to keep the label
  well-defined on pre-existing on-disk fan-in that predates R3.
- **Design decision — presumed-healthy caller semantics (R3)**:
  `ValidateDependencies` treats the caller feature `slug` as
  presumed-healthy (its own reconcile state is not consulted).
  A fresh draft `A supersedes X` write is rejected if any OTHER
  healthy peer already supersedes `X`. This aligns with the
  intent of AC-4/D5 — write-time prevention of ambiguity — and
  avoids a chicken-and-egg where the caller must already be
  applied+verified before validation can approve.
- **Design decision — health-check duplication (R3)**: the
  `internal/store` package cannot import `internal/workflow`
  (cycle), so `supersederIsHealthyForValidation`,
  `supersederValidationHealthyStates`, and
  `supersederValidationBlockedOutcomes` are locally duplicated
  in `internal/store/validation.go`. A comment marks the
  byte-identity contract with `internal/workflow/labels.go`
  `supersederIsHealthy` — any change to one MUST be mirrored in
  the other. Long-term cleanup: promote to a shared
  `internal/reconcilestate` package (not done here to keep the
  fold-in surgical).
- **Design decision — stale-supersession runtime flip rationale
  (R4, PRD §4.5.3 clause 3, ADR-028 D6/D8)**: the rev-0
  implementation returned `false` from `isFeatureSupersededIn`
  for stale peers, so both `RunReconcile` default set and V7
  closure REPLAYED the historical target. This contradicted the
  docstring at `labels.go:456-458` ("stale → historical stays
  excluded") + PRD §4.5.3 clause 3 (Accepted status). The
  supervisor decision was RUNTIME FLIP (not paper reconciliation):
  the flipped rule is that supersession excludes the historical
  target regardless of whether the superseder is healthy OR
  stale — the operator-visible signal is the `stale-superseder`
  label that renders on the superseder side telling the operator
  the replacement needs repair. Orphan superseders (dep points
  to a nonexistent slug) are separate — they attach only to the
  superseder side and do NOT cascade into target exclusion,
  because there is no target relationship to honor. R4's new
  `TestReconcileDefaultSet_OrphanSupersederDoesNotExcludeUnrelated`
  locks that boundary.
- **Design decision — RunReconcile explicit-slug treatment (PRD §3.3, AC-11)**:
  when the caller directly names a superseded feature on the CLI, we
  DO reconcile it (audit/repair path) but PREPEND a historical-feature
  warning note to the `ReconcileResult.Notes` slice. Alternative
  interpretations considered but rejected: (a) silently skip the
  explicit slug (would violate the "audit/repair path always available"
  clause); (b) hard-error on the explicit slug (would require a CLI
  flag override). Warning-note path is the least surprising and
  matches the ADR-028 D6 default-vs-explicit distinction.
- **Design decision — V7 supersession skip semantics (ADR-028 D6, D8)**:
  in `runClosureReplay`, the BFS loop pre-loads all features once
  (`allFeatures`) and calls `isFeatureSupersededIn(all, curr)` per
  candidate parent. When true, the parent is skipped AND its own
  hard-parent chain is NOT enqueued — the ancestor chain is only
  relevant because of the excluded parent, so its transitive parents
  should not be replayed either. If some independent hard-dep path
  re-queues the same ancestor, it will be picked up via that path.
  With the R4 runtime flip, this skip now fires for stale
  supersession as well — operator sees `stale-superseder` on the
  replacement, historical target's V7 replay is suppressed.
- **Design decision — ADR-011 D2 cycle detection**: verified
  `DetectCycles` in `internal/store/dag.go` is edge-kind-agnostic
  (walks `dep.Slug` without branching on `dep.Kind`). Zero code
  change was required; Slice 2 is a regression-test-only commit
  locking this property. If a future wave adds edge-kind-specific
  cycle rules (e.g. soft edges skipping the cycle check), the DAG
  primitive would need a rework — currently NONE do.
- **Design decision — persist-side label strip pattern**: introduced
  internal helper `stripDerivedLabels(labels)` that chains
  `StripFreshnessLabels` → `StripSupersessionLabels` and is called
  at all three persist sites (`accept.go`, `reconcile.go` two
  places). This keeps `Reconcile.Labels` on disk free of derived
  labels, matching the ADR-011 D3 contract that derived labels
  are ALWAYS re-composed at read time.
- **Skill asset parity guard** (`assets/assets_test.go`): my additive
  `supersedes` bullet does not touch any required-anchor list, so
  `TestSkillParityGuard`, `TestSkillRecipeSchemaMatchesCLI`, and
  `TestSkillDocReferencesAreSelfContained` all pass unchanged.
- **CLI dep parser** (`internal/cli/feature_deps.go:parseDepSpec`):
  now accepts `<parent>:supersedes` shorthand. Verified empirically
  via `tpatch feature deps <slug> add <parent>:supersedes` end-to-end
  during Slice 3 Rule 20 verification.
- **Slice 5 status flip**: only ADR-028 was flipped in
  `docs/adrs/README.md`. ADR-011 remained Accepted (unchanged).
  PRD-feature-supersession and ADR-028 both went Proposed →
  Accepted.
- **Wave β (`PRD-write-file-recipe-safety`) prep**: its
  `§PRD-1-interaction` clause says superseded features suppress
  or downgrade `write-file` drift on the parent verify path. The
  Wave α hooks are in place — the `isFeatureSupersededIn` helper
  in `internal/workflow/labels.go` is the natural extension point.
- **Side Research md5 invariant preserved**:
  `b385fe622db9926f48861105239f113e` — verified after every edit to
  this file.
- **Untouched pre-existing dirty tree** (belongs to supervisor, not
  Wave α): `docs/CLUSTERS.md`, `docs/state-of-the-art/*`,
  `docs/whitepapers/README.md`, and several untracked whitepaper /
  PRD drafts (`WP-004`..`WP-007`, `PRD-feature-unapply`,
  `PRD-recurring-patches`). These predate the task dispatch.
- Status field intentionally left as `Dispatched` per handoff protocol —
  supervisor flips to `Review` at consolidation.

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
