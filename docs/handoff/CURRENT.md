# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-wave-alpha-supersession-implementation`
- **Milestone**: v0.12.0 Wave α — implement `PRD-feature-supersession` + `ADR-028-supersession-edge-model`.
- **Description**: First wave of the v0.12.0 3-wave implementation cluster (supervisor picked Option A 2026-07-29). Implements the third-kind `depends_on[].kind: "supersedes"` edge on the ADR-011 dependency graph, 4 new composable labels, and reconcile suppression of superseded features. Extends (does NOT fork) the ADR-011 D1 storage lane.
- **Status**: Dispatched — implementer in flight.
- **Assigned**: 2026-07-29.

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

## Test Results

Final gate output (2026-07-30):

```
$ gofmt -l .
$ go vet ./...
$ go build ./cmd/tpatch
$ go test -count=1 ./...
ok  	github.com/tesseracode/tesserapatch/assets	0.987s
ok  	github.com/tesseracode/tesserapatch/internal/buildinfo	2.307s
ok  	github.com/tesseracode/tesserapatch/internal/cli	88.000s
ok  	github.com/tesseracode/tesserapatch/internal/gitutil	16.199s
ok  	github.com/tesseracode/tesserapatch/internal/provider	16.574s
ok  	github.com/tesseracode/tesserapatch/internal/safety	5.384s
ok  	github.com/tesseracode/tesserapatch/internal/store	5.672s
ok  	github.com/tesseracode/tesserapatch/internal/tools/studyvalidator	4.537s
ok  	github.com/tesseracode/tesserapatch/internal/workflow	52.171s
ok  	github.com/tesseracode/tesserapatch/tests/integration	4.769s
```

All packages pass; no diagnostics from `gofmt` or `go vet`.

Commits landing (5, all with valid Co-authored-by trailer):
- `48399f4` slice 1: schema + parity guards
- `f8f7766` slice 2: supersedes cycle detection tests
- `195921a` slice 3: 4 composable labels + rendering
- `3f49c36` slice 4: reconcile suppression + V7 supersession-skip
- `4d4bb60` slice 5: CHANGELOG + status flips

## Next Steps

1. Dispatch internal reviewer + supervisor-external reviewer in parallel for Wave α rev-0.
2. On three-way APPROVED: archive Wave α to `docs/handoff/HISTORY.md`, dispatch Wave β (`PRD-write-file-recipe-safety` + ADR-029).
3. Wave γ (`PRD-active-feature-session`) dispatch may proceed in parallel with Wave β if capacity allows (independent code surfaces).

## Blockers

None.

## Context for Next Agent

- HEAD after Wave α implementation: `4d4bb60` (5 commits atop kickoff HEAD `7081c62`).
- **Rule 19 exported-surface citations landed**: Slice 1 (store schema + CLI parser), Slice 3 (workflow label helpers, exported `DeriveSupersessionLabels`, `IsSupersessionLabel`, `StripSupersessionLabels`, `IsFeatureSuperseded`), Slice 4 (workflow reconcile + verify V7 wire-in). Each commit message names the PRD AC + ADR D-clause.
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
  Stale supersession (superseder unhealthy) does NOT skip — the
  historical target remains in the closure so V7's replay-fail
  remediation surfaces normally (ADR-028 D8 warning-class severity).
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
