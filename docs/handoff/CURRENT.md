# Current Handoff

## Active Task

- **Task ID**: `m17-wave-c-tpatch-land` — M17 Wave C `tpatch land` implementation (PRD-tpatch-land)
- **Milestone**: M17 — boundary-capture cluster, v0.8.0
- **Status**: Wave A shipped. Wave B + Wave D rev-1 implemented (awaiting review). **Wave C implemented (awaiting review)**.
- **Assigned**: 2026-05-13

## Wave C — Just Implemented (2026-05-13)

**Status**: Implementation complete on a working branch / staged commits, awaiting sub-agent reviewer + external supervisor.

**Summary**: New `tpatch land <slug>` flagship verb that composes (record → safe path-set staging → one Git commit) with the locked four-trailer block (`Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`) followed by the repo `Co-authored-by:` trailer. Per `docs/prds/PRD-tpatch-land.md` and `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md`.

**Files changed**:
- `internal/cli/land.go` — new (~530 lines). `landCmd()`, `runLand()`, `runLandDryRun()`, `landPreflight()` (PRD §3.2 refusals: conflict markers, `*.orig`/`*.rej` leftovers, mid-merge state, hard-parent gate), `embedRecord()` (re-executes a fresh `buildRootCmd()` with `record` args — preserves all record semantics including Wave A1 auto-base and Wave B collision detection, verbatim diagnostics), `computePathSet()` + `dirtyPaths()` (uses `git status --porcelain --untracked-files=all` to expand untracked dirs) + `classifyExtras()`, `stagePathSet()` (`git add --intent-to-add` then `git add`), `deriveSubject()` (precedence: `--message` > spec.md H1 > first non-empty request.md line > fallback), trailer hashing helpers.
- `internal/cli/cobra.go` — registered `landCmd()` in `buildRootCmd()` between `recordCmd()` and `reconcileCmd()`. **No other change.** Wave A1/A2/B/D regions untouched.
- `internal/cli/land_test.go` — new (~22KB, 19 tests). One test per PRD §6 acceptance row plus subject-derivation, trailer-SHA accuracy, re-record round-trip, pre-commit hook recovery hint, and cross-feature collision refusal (consumes `setupRecordRangeFixture` from `record_range_scoped_test.go`).
- `assets/skills/{claude,copilot,cursor,windsurf}/...`, `assets/workflows/tessera-patch-generic.md`, `assets/prompts/copilot/tessera-patch-apply.prompt.md` — appended a one-sentence pointer to rule-4 ("Or run `tpatch land <slug>` to compose record + safe-stage + one Git commit (with a `Tpatch-Feature: <slug>` trailer block — see docs/land.md) in a single verb.") and added `tpatch land` to each per-file command list (markdown table row for claude; bullet for the others). The rule-4 anchor `tpatch record <slug> BEFORE git commit` is preserved verbatim.
- `assets/assets_test.go` — added `"tpatch land"` to `requiredCommands`. Parity guard now enforces presence of the new verb in all six skill assets.
- `CHANGELOG.md` — `### Wave C` subheading under `## v0.8.0 (in development)`, inserted above `### Wave D`.
- `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md` — fleshed out from placeholder to Accepted. Documents the four-trailer schema, the `Tpatch-CVE` reservation for hotfix, the no-overwrite rule for `apply.base_commit`, and four rejected alternatives.

**Test results** (last run, this session):

```
ok  	github.com/tesseracode/tesserapatch/assets	0.535s
?   	github.com/tesseracode/tesserapatch/cmd/tpatch	[no test files]
ok  	github.com/tesseracode/tesserapatch/internal/buildinfo	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/cli	30.069s
ok  	github.com/tesseracode/tesserapatch/internal/gitutil	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/provider	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/safety	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/store	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/workflow	(cached)
```

All 19 `TestLand_*` tests pass. Parity guard green. `gofmt` and `go build ./cmd/tpatch` clean.

### Notable / drift notes

- **PRD §3.4 vs §6 ac.3 trailer-order mismatch**: §3.4 explicitly orders Feature → Patch-SHA → Recipe-SHA → Base-Commit. §6 ac.3 reorders them in prose ("Patch-SHA, Recipe-SHA, Base-Commit, Feature") but is internally inconsistent. Implementation followed §3.4 literally; ADR-019 locks that ordering.
- **`apply.base_commit` ownership (PRD F2)**: `land` writes `status.notes = "landed at <ts>"` BEFORE staging (so the change rides in the commit and the post-condition "working tree clean" holds). It does NOT overwrite `apply.base_commit` — that field stays owned by `record` / auto-base. The feature↔commit binding lives only in the `Tpatch-Feature` trailer (chicken-and-egg: a commit cannot embed its own SHA in tracked content). ADR-019 documents this.
- **Untracked-directory expansion in path set**: `git status --porcelain` collapses an untracked directory to a single `?? path/` entry. The first iteration of `dirtyPaths` saw `?? .tpatch/features/` and incorrectly classified it as an extra (because the strict prefix check requires `.tpatch/features/<slug>/`). Fixed by passing `--untracked-files=all` to expand into individual files.
- **`embedRecord` strategy**: Builds a fresh `buildRootCmd()` and calls `Execute()` with `["record", "--path", repoRoot, slug, ...]`. This is the most surgical way to compose without duplicating record's collision/auto-base logic. Re-parses the persistent `--path` flag cleanly.
- **`abbrevSHA` collision**: `internal/cli/status_dag.go:539` already exports `abbrevSHA(sha string) string` returning 7 chars; reused (initial draft had a duplicate definition that broke the build).

### What was NOT touched (per brief DO-NOT list)

- `internal/cli/record_collision*.go` and the collision wiring in `recordCmd` — Wave B, untouched. `embedRecord` invokes record via cobra `Execute()`, never reaching into its internals.
- `internal/cli/record_auto.go`, `internal/cli/record_range_scoped.go` — Wave A1, read-only references in tests.
- `internal/workflow/reconcile.go` lock-guard region (~lines 560-700) — Wave A2, untouched.
- `internal/workflow/patch_id_detector*.go` and the phase-1.5 block in `reconcile.go` (~lines 196-236) — Wave D, untouched. `Config.PatchIDDetectorEnabled` default still `false`.
- `docs/state-of-the-art/` (untracked user research) — preserved.
- "Side Research" section below — preserved verbatim.

### PRD §6 acceptance criteria → test mapping

| PRD §6 ac | Test name |
|-----------|-----------|
| ac.1 success / one commit | `TestLand_Success_OneCommit_FourTrailers` |
| ac.2 dry-run no mutation | `TestLand_DryRun_NoMutation` |
| ac.3 four-trailer block | `TestLand_Success_OneCommit_FourTrailers` + `TestLand_TrailerSHAs_DeterministicAndAccurate` |
| ac.4 (preflight refusals) | `TestLand_Refuses_*` (conflict markers, orig-rej, mid-merge, hard-parent) |
| ac.5 base-commit not overwritten | `TestLand_BaseCommitUnchanged` |
| ac.6 working tree clean | `TestLand_Success_OneCommit_FourTrailers` (asserts clean) |
| ac.7 status notes | `TestLand_StatusNotesUpdated` |
| ac.8 extras strict | `TestLand_Refuses_ExtrasStrict` / `TestLand_AllowsExtras_WithFlag` |
| ac.9 dirty/untracked in-scope OK | `TestLand_AcceptsInScopeDirty` |
| ac.10 subject derivation | `TestLand_SubjectFromMessage` / `TestLand_SubjectFromSpecH1` / `TestLand_SubjectFallback` |
| ac.11 --no-record requires patch | `TestLand_NoRecord_RequiresPatch` |
| ac.12 --auto/--from mutex | `TestLand_AutoFromMutex` |
| ac.13 re-record round-trip | `TestLand_ReRecord_Roundtrip` |
| ac.14 pre-commit hook | `TestLand_PreCommitHookRecovery` |
| ac.15 --no-record retry | `TestLand_NoRecord_RetriesStagedIndex` |
| ac.16 help mentions contract | `TestLand_HelpMentionsContract` |
| ac.17 cross-feature collision | `TestLand_Refuses_CrossFeatureCollision` |

## Wave D rev-1 fix (2026-05-12)

External supervisor flagged a Medium false-positive: phase-1.5 was
fed the wrong patch artifact. The legacy reconcile loader at
`internal/workflow/reconcile.go:166-169` reads `incremental.patch`
first and only falls back to `post-apply.patch` (correct for
phases 2/3/4 / GAP 4 multi-feature derivation). That same `patch`
variable was being passed to `runPatchIDDetector`, violating
PRD-patch-already-upstream-detector §5.1 which mandates the
canonical `post-apply.patch` for the patch-id sweep.

### Reproducer
Feature with canonical `post-apply.patch` adding two files
(extra.txt + greeting.txt) and `incremental.patch` adding only
greeting.txt. Upstream absorbs only greeting.txt. With the flag on,
pre-fix code wrongly emitted `[upstreamed] (phase-1.5-patch-id-match)`
and persisted `patch_id_match` because phase-1.5 saw the incremental
form.

### Fix scope (surgical)

- `internal/workflow/reconcile.go` — phase-1.5 block now loads
  `artifacts/post-apply.patch` separately via `s.ReadFeatureFile` and
  passes that to `runPatchIDDetector`. If the canonical artifact is
  missing or whitespace-only, phase-1.5 fail-soft skips with a
  one-line note (`"phase 1.5 skipped: no canonical post-apply.patch
  artifact"`) and reconcile falls through to phase 2. The legacy
  `patch` variable used by phases 2/3/4 is unchanged. Wave A2
  lock-guard region (~lines 560-700) untouched.
- `internal/workflow/patch_id_detector_test.go` — added two regression
  tests:
  - `TestPatchIDDetector_PrefersCanonicalOverIncremental`: dual-artifact
    divergence (canonical=multi-file, incremental=subset matching
    upstream). Asserts `Outcome != ReconcileUpstreamed`,
    `Phase != "phase-1.5-patch-id-match"`, `PatchIDMatch == nil`,
    persisted `status.Reconcile.PatchIDMatch == nil`. Verified to
    FAIL on pre-fix code, PASS after.
  - `TestPatchIDDetector_CanonicalMatchesEvenWhenIncrementalDiffers`:
    positive companion — canonical matches upstream while
    incremental is unrelated. Asserts phase-1.5 still fires correctly
    (guards against an over-correction that would skip phase-1.5
    whenever incremental.patch exists). Verified to FAIL pre-fix
    (would reach phase 4) and PASS after.
- `CHANGELOG.md` — added "Rev-1 fix" bullet under `### Wave D`.
- `docs/handoff/CURRENT.md` — this update.

### What was NOT touched (per brief DO-NOT list)

- `docs/state-of-the-art/` (untracked user research) — preserved.
- "Side Research" section below — preserved verbatim.
- Wave A2 lock-guard region of `reconcile.go` (~lines 560-700) and
  `internal/gitutil/lock_guard.go` — untouched.
- Wave B record path (`internal/cli/record_collision*.go`,
  collision wiring in `cobra.go`) — untouched.
- `Config.PatchIDDetectorEnabled` default — still `false`.
- Phase ordering — phase-1.5 stays strictly between phase 1 and
  phase 2.
- Deferred CLI flags (`--check-applied-only`, `--auto-drop-merged`)
  — still deferred, out of scope for rev-1.

### Verification (rev-1)

- `gofmt -l .` clean.
- `go build ./cmd/tpatch` clean.
- `go vet ./...` clean.
- `go test ./internal/workflow -run TestPatchIDDetector_ -count=1 -v`:
  all 11 tests PASS (9 pre-existing + 2 new regression).
- Pre-fix sanity check: with the reconcile.go fix stashed, both new
  regression tests FAIL as designed; restored after.
- `go test ./assets -run TestSkillParityGuard -count=1 -v`: PASS
  (no skill changes).
- `go test ./...`: all packages green (workflow ~46s, cli ~40s).
- Default-OFF preservation: `TestPatchIDDetector_DefaultOffNoOp`
  still passes — no behaviour change when
  `PatchIDDetectorEnabled=false`.

---

## Wave D summary (2026-05-11) — original landing, superseded by rev-1 above

Phase-1.5 deterministic patch-already-upstream detector landed,
default-OFF behind `Config.PatchIDDetectorEnabled` (PRD §6). Files
touched:

- `internal/store/types.go` — added `Config.PatchIDDetectorEnabled`,
  `Config.PatchIDScanLimit`, `DefaultPatchIDScanLimit = 5000`,
  `PatchIDMatch` struct, `ReconcileSummary.PatchIDMatch *PatchIDMatch`
  (omitempty).
- `internal/store/store.go` — flat YAML parser learns
  `patch_id_detector_enabled` and `patch_id_scan_limit`; `SaveConfig`
  emits the keys only when non-default (preserves pre-Wave-D byte
  identity for fixtures).
- `internal/gitutil/patch_id.go` — new file. Primitives `PatchID`,
  `CommitPatchID`, `RevListInRange` wrap `git patch-id --stable` and
  `git rev-list --no-merges`.
- `internal/workflow/patch_id_detector.go` — new file. `runPatchIDDetector`
  reads `upstream.lock`, computes our patch-id, walks the range, applies
  PRD §5.3 multi-match policy (earliest match wins), enforces the
  scan-limit cap (PRD §5.2), fails soft on tooling errors (PRD §5.1).
- `internal/workflow/reconcile.go` — fast-path slotted between phase 1
  (reverse-apply) and phase 2 (operation-level). Gated on
  `cfg.PatchIDDetectorEnabled`; on match sets `Outcome=ReconcileUpstreamed`,
  `Phase="phase-1.5-patch-id-match"`, `UpstreamCommit = matched SHA`,
  `PatchIDMatch = audit payload`, skips phases 2/3/4.
- `internal/workflow/patch_id_detector_test.go` — new file. Coverage:
  default-OFF no-op, primitive match, primitive no-match, missing-lock
  Skip, empty-baseline Skip, empty-range no-match, scan-limit Skip,
  flag-on integration match (provider=nil) with `status.json`
  persistence assertion, flag-on no-match falls through, config
  parser round-trip for the new keys.
- `CHANGELOG.md` — Wave D entry under v0.8.0 in-development; explicitly
  flags default-OFF gating and lists deferred CLI surfaces.

### PRD acceptance criteria — what shipped and what deferred

Shipped: PRD §3.1 (phase-1.5 default behaviour), §3.4 JSON
`patch_id_match` block, §4 schema additions, §5.1 fail-soft semantics,
§5.2 scan-limit cap, §5.3 multi-match policy, §5.6 heuristic-fallback
friendliness, §6 default-OFF rollout, §9 validation plan items 1-4 and
7-9.

Deferred to follow-up backlog (not in this commit, called out in
CHANGELOG): PRD §3.2 `--check-applied-only` CLI verb/flag, PRD §3.3
`--auto-drop-merged` CLI flag, PRD §3.3 hotfix-kind auto-drop default
gating. Rationale: the deterministic primitive + reconcile fast-path
is the load-bearing M17 Wave D contract; the user-facing flags layer
on top and can ship in v0.8.1+ without invalidating the core. Brief
permits this scoping ("if you add a CLI flag, ..."). No skill files
were touched (no new CLI flag → parity guard unaffected).

### Verification

- `gofmt -l .` clean.
- `go build ./cmd/tpatch` clean.
- `go test ./...` green across all packages.
- New `TestPatchIDDetector_*` suite (9 tests) passes; new
  `TestConfigParserRoundTripsPatchIDKeys` passes.
- Skill parity guard test (`go test ./assets`) still cached/green —
  no CLI flag changes, no skill asset changes.

### Default-OFF preservation — explicit confirmation

`TestPatchIDDetector_DefaultOffNoOp` asserts that a freshly-init
config keeps `PatchIDDetectorEnabled` false and that reconcile does
*not* enter phase 1.5 nor populate `PatchIDMatch`. All pre-existing
reconcile tests (`TestReconcilePhase1_*`, `TestReconcilePhase4_*`,
labels tests, etc.) continue to pass without modification — none of
them enable the flag, so the new fast-path is silently skipped and
the original phase ladder runs unchanged.

## Wave Status

| Slice | Task ID | PRD | Status | Depends |
|-------|---------|-----|--------|---------|
| A1 | `impl-record-auto-base` | PRD-record-auto-base | ✅ shipped (`1d6179c` + rev-1 `4484e04`) | — |
| A2 | `impl-reconcile-lock-guard` | PRD-reconcile-lock-guard | ✅ shipped (`8fc2e4e`) | — |
| **B** | `impl-record-collision-detection` | PRD-record-collision-detection | 🟡 **implemented, awaiting review** | A1 (recovery hints reference `--auto`) |
| **C** | `impl-tpatch-land` | PRD-tpatch-land | 🟡 **implemented, awaiting review** | A1, A2, B |
| **D** | `impl-patch-already-upstream-detector` | PRD-patch-already-upstream-detector | 🟡 **implemented, awaiting review** (default-OFF) | independent (default-OFF) |

**Coordination notes for parallel B + D dispatch**:
- Same checkout, same working tree — same risk pattern as Wave A's parallel dispatch.
- Recommend: dispatch B and D in parallel but with explicit "do not touch the other's territory" lists in their briefs.
- B touches `internal/cli/cobra.go` record path + `internal/gitutil/` (collision-signature primitive); D touches reconcile workflow (`internal/workflow/reconcile.go`) + `internal/store/types.go` (config flag `PatchIDDetectorEnabled`).
- Minimal overlap — main shared file is `cobra.go` if D adds a `--no-patch-id-detector` flag. Risk lower than Wave A's A1↔A2 because they share fewer surfaces.

## Just Shipped — M17 Wave A

**M17 Wave A (A1 + A2)** — APPROVED WITH NOTES, archived to HISTORY 2026-05-11. Ship stack: `1d6179c` (A1 v0) + `8fc2e4e` (A2) + `6d67b41` (verdicts) + `4484e04` (A1 rev-1) + `63a0373` (rev-1 verdict). External one revision on A1 (zero-diff false-green + lock-fallback policy); both addressed in `4484e04`. A2 clean. One non-blocking external follow-up captured as backlog `m17-wave-a1-followup-ambig-discovery-diag`. Cross-commit binding (A1 ↔ A2) accepted; HISTORY notes the revert must move both as a unit.

## Wave B — Just Implemented (2026-05-11)

**Status**: Implementation complete on `main`, awaiting sub-agent reviewer + external supervisor.

**Summary**: Cross-feature byte-identical canonical patch collision detection at `tpatch record` time, per `docs/prds/PRD-record-collision-detection.md`. Refuses by default, overridable with `--allow-collision "<reason>"`. Same-feature re-record with unchanged bytes is treated as deduplication (numbered audit snapshot skipped, canonical artifact rewritten in place).

**Files changed**:
- `internal/gitutil/collision.go` — new `PatchSignature(patch) (sha256Hex, bytes)` primitive. Tiny stdlib-only helper. Reusable by future `tpatch patches --collisions` (PRD §7).
- `internal/gitutil/collision_test.go` — deterministic / distinct-bytes / empty-string coverage for the primitive.
- `internal/cli/record_collision.go` — `scanCanonicalPatchCollisions(store, slug, patch)` enumerates `.tpatch/features/*/artifacts/post-apply.patch`, applies the length → SHA-256 → byte-for-byte ladder (PRD §4), splits matches into same-feature vs cross-feature buckets. `printCollisionRefusal` writes the PRD §3.1 diagnostic and tailors recovery hints per capture mode (PRD §5).
- `internal/cli/cobra.go` — wired the scan into `recordCmd` AFTER empty-patch handling and BEFORE `WriteArtifact`; added `--allow-collision` flag; same-feature dedup skips `WritePatch`; `generateRecordMD` extended to persist the override reason under a "Collision Override" section.
- `internal/cli/record_collision_test.go` — 9 acceptance tests covering each PRD §8 row (cross-feature refusal, allow-collision override, same-feature dedup, changed-bytes appends, ≥3 colliders recommendation, missing artifact ignored, empty capture skips scan, working-tree + from-mode recovery hints).
- `internal/cli/record_auto_test.go` — existing `TestRecordAuto_AutoEqualsFromExplicit` updated to use `--allow-collision` for its deliberate byte-equivalence assertion (the test records the same bytes under two slugs to verify `--auto` == `--from`; collision detection would otherwise refuse it).
- `CHANGELOG.md` — `### Wave B` subheading under unreleased v0.8.0.
- `docs/record.md` — "Cross-feature collision detection (v0.8.0)" section with the refusal example and dedup semantics.

**Test results**:
- `gofmt -l .` clean
- `go build ./cmd/tpatch` clean
- `go test ./...` all green (full suite)
- `go test ./assets -run TestSkillParityGuard` green (no CLI command surface changes; flag-only addition does not require skill updates — the parity guard inspects `requiredCommands` and `requiredAnchors`, none of which mention `--allow-collision`)

**Notable**:
- The collision diagnostic is printed directly to `cmd.ErrOrStderr()`, then a short error is returned. Same pattern as the rest of `recordCmd` (`Execute()` in `cobra.go` will additionally print `error: <err>` from `os.Stderr`). The user sees the long diagnostic once and the short error once.
- `--allow-collision <reason>` requires a non-empty trimmed reason. Empty / whitespace-only reasons fail the override and the refusal stands. This matches the PRD §3.1 contract ("the reason is required to proceed").
- The scan helper lives in `internal/cli/` (not `internal/gitutil/`) because it depends on `store.Store` for feature enumeration. The brief allowed either layer; only the byte signature primitive landed in gitutil to satisfy the "collision-signature primitive" line of the brief.
- One pre-existing test (`TestRecordAuto_AutoEqualsFromExplicit`) deliberately records the same patch bytes under two slugs. It was updated to pass `--allow-collision` with a self-describing reason rather than restructure the assertion — the test still validates byte equality, which is the PRD §6 row 6 acceptance for Wave A1.
- No changes to `internal/workflow/reconcile.go`, no changes to `Config.PatchIDDetectorEnabled` — Wave D territory left untouched.
- `internal/cli/record_auto.go` not modified.

**For the reviewer (start here)**:
1. `internal/cli/cobra.go` recordCmd block from `// Determine capture mode once` through the WritePatch/dedup branch — that is the wiring. Confirm refusal happens before `WriteArtifact` (it does: scan runs immediately after `empty capture` handling and before any artifact write).
2. `internal/cli/record_collision.go` — verify PRD §4 algorithm (length → SHA-256 → bytes), missing-file skip, and the recovery-hint switch per capture mode (PRD §5).
3. `internal/cli/record_collision_test.go` — each PRD §8 acceptance row has a named test.
4. The two diagnostic paths: refusal (no artifact write) vs override (stderr warn + record.md persist).

## Tagging Decision (Open)

Wave A alone is partial v0.8.0. Two paths:
1. **Tag `v0.8.0-alpha.1` after Wave A push** — early-adopter signal, enables progress visibility, follows precedent of mid-milestone alphas. Cost: pre-release tag count grows; minor versioning bookkeeping.
2. **Defer tagging until Wave A+B+C+D complete** — single clean `v0.8.0` release. Cost: longer dark period; if Waves B/C/D are large, no incremental ship signal.

Recommendation: defer (option 2). Wave A is internal-facing infrastructure; user-facing value lands with Wave C (`tpatch land`). Tag once at v0.8.0 unless we hit a long pause between waves.

## Dispatch Plan (proposed)

1. Push current 5-commit stack (`1d6179c` + `8fc2e4e` + `6d67b41` + `4484e04` + `63a0373`) to `origin/main`. No tag.
2. Dispatch `m17-wave-b-impl` background agent with `PRD-record-collision-detection.md` as authoritative brief.
3. Dispatch `m17-wave-d-impl` background agent with `PRD-patch-already-upstream-detector.md` as authoritative brief + reminder of default-OFF gating.
4. Each implementer completes → sub-agent reviewer → external supervisor → push. Same pattern as Wave A.
5. After both B and D ship: dispatch `m17-wave-c-impl` (depends on A1+A2+B; D independent, can have shipped or be in flight).
6. After all 4 waves ship + follow-ups assessed: tag `v0.8.0`, archive cluster to HISTORY.

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

### Candidate follow-up names

These are research outputs only, not queued roadmap work:

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



## Blockers

None. Awaiting user direction on:
- Wave A push (5 commits, no tag) — user can drive or supervisor can execute on confirmation.
- Wave B + D parallel dispatch — user OK with same pattern as Wave A?

