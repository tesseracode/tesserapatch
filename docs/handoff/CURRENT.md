# Current Handoff

## Active Task

- **Task ID**: M15-W3-SLICE-D
- **Milestone**: M15 Wave 3 — Verify freshness rollout (final slice)
- **Description**: Finalize the verify-freshness work by adding `tpatch verify --all` topo-ordered aggregate reporting, rolling the §4.4 freshness bullet across all 6 skill formats, extending the `assets/assets_test.go` parity guard with the new anchors, cross-linking `docs/dependencies.md` to verify, and shipping CHANGELOG v0.6.2.
- **Status**: Review (revision-4 in progress) — fourth HIGH finding from external supervisor under fix on local commits ahead of `19271f7`
- **Source PRD**: `docs/prds/PRD-verify-freshness.md` §9 (Slice D row), §4.4 (skill bullet contract)

## Predecessor — Slice C

✅ Approved by external supervisor on `23af23e`. Stack on `origin/main`:

- `32f50c8` — Slice C original (V3–V9 + closure-replay)
- `5892ae0` — revision-1 (V8 runs against closure-replayed baseline when recipe absent + patch present)
- `23af23e` — revision-2 (V8 precondition is file presence, not non-empty content)
- `08ed4e5` — tracking (external verdicts logged + ROADMAP flip)

Full retrospective archived in `docs/handoff/HISTORY.md` under `2026-04-29 — M15-W3-SLICE-C`.

## Scope (per PRD §9 Slice D)

1. **`tpatch verify --all`** — new aggregate runner.
   - Topologically order all features (Kahn, hard-deps first, same primitive used by `tpatch land --all` if applicable; otherwise vendored/inlined locally).
   - Skip pre-apply features (state ∈ {drafted, analyzed, defined, explored, implemented}) per PRD Q2 and emit a one-line `skipped: pre-apply` row for each. Only states with a recorded patch (applied / reconciled / verified) execute V0–V9.
   - Per-feature output: existing single-feature verdict line + checks block, prefixed with the slug.
   - Aggregate footer: counts per verdict, plus an exit code that is non-zero if ANY feature failed (verdict ∈ {failed} or any check.severity=error fails). `--json` emits a list of per-feature reports + an aggregate summary object.
   - No new state transitions; this is a read-only aggregate over existing single-feature verify.

2. **Skill bullet rollout (all 6 surfaces)** — add the §4.4 freshness bullet to:
   - `assets/skills/claude/SKILL.md`
   - `assets/skills/copilot/SKILL.md`
   - `assets/skills/copilot-prompt/tpatch.prompt.md`
   - `assets/skills/cursor/.cursor/rules/tpatch.mdc`
   - `assets/skills/windsurf/.windsurfrules`
   - `assets/skills/generic/AGENT.md`
   The bullet must verbatim match the PRD §4.4 wording (or as close as the surface allows; copy-paste anchor strings into all six files).

3. **Parity guard extension** — `assets/assets_test.go` adds anchor checks for the §4.4 bullet across all 6 surfaces. Pattern matches existing slice rollouts (one anchor substring per file).

4. **`docs/dependencies.md` cross-link** — add a short paragraph pointing readers from the dependency model to verify, since hard-dep semantics now drive V7/V8 closure replay.

5. **`CHANGELOG.md` v0.6.2 entry** — name the verb (`tpatch verify`), call out the freshness overlay (V0–V9 numbered checks), call out the explicit out-of-scope list (no provider calls, no state transitions, no `--all` interaction with shadow). Link to PRD §9 for the slice-by-slice landing.

## Constraints

- DO NOT touch `internal/workflow/verify.go` V3–V9 logic (Slice C closed). Slice D is additive surface only.
- DO NOT touch the closure-replay primitive or shadow lifecycle.
- `verify --all` must reuse the existing single-feature `RunVerify` entrypoint per feature; no separate code path.
- Pre-apply skip must be deterministic and ordered first in topo (i.e., even features with no recorded patch participate in topo order; their skip row appears at their topo position).
- Parity guard must keep the existing anchor checks intact and only ADD new ones.
- Out-of-scope file folders: `docs/whitepapers/`, exploratory PRDs (`PRD-feature-slices-and-nested-changes.md`, `PRD-intent-version-control-evaluation.md`, `PRD-record-auto-base.md`, `PRD-record-collision-detection.md`, `PRD-tpatch-git-primitive-mapping.md`, `PRD-tpatch-land.md`).

## Tests required

- **Aggregate ordering**: 3-feature DAG with one hard-dep chain (A → B → C) and one independent feature D — assert `verify --all` runs A, B, C, D in topo order; insertion order in `.tpatch/features/` must NOT determine output order.
- **Pre-apply skip**: feature in state `defined` shows up in topo position with `skipped: pre-apply` row; does NOT execute V0; does NOT cause a non-zero exit on its own.
- **Aggregate exit code**: at least one failed feature → non-zero exit; all passing → zero exit.
- **Aggregate JSON shape**: `--json` emits `{ features: [...], summary: {passed, failed, skipped, error} }` (or equivalent already established for single-feature; extend rather than break).
- **Malformed-but-present artifact case** (carryover lesson from Slice C external review): include at least one `verify --all` test where one feature has a malformed-but-present artifact (e.g., zero-byte `post-apply.patch` or invalid `apply-recipe.json`) and assert the aggregate correctly reports that feature as failed without poisoning other features in the run.
- **Parity guard**: `go test ./assets/...` green with new anchors; intentionally remove a bullet locally to confirm the guard fails.

## Validation gate (must pass before review dispatch)

1. `gofmt -l .` — empty.
2. `go build ./cmd/tpatch` — success.
3. `go vet ./...` — clean.
4. `go test ./...` — all pass; new tests counted.
5. `tpatch verify --all` smoke run on a fixture repo with at least 3 features.
6. Skill files visually inspected — §4.4 bullet present in all 6.

## Reviewer prompt notes (for Slice D reviewer dispatch)

- **Carry forward the artifact-presence gate lesson from Slice C**: any new precondition probe added in Slice D (e.g., `verify --all` skipping pre-apply, or aggregate JSON shape gating) must be exercised with a malformed-but-present artifact case. Reviewer must explicitly run a 2-cell matrix (well-formed vs malformed-but-present) on every new gate.
- Reviewer should diff the 6 skill files against each other to confirm the bullet is consistent (not just present).
- Reviewer should `gofmt -l .` and confirm no unintended file format changes.

## Out of scope (DO NOT touch)

- V3–V9 logic in `internal/workflow/verify.go` (Slice C is closed; only `verify --all` orchestration above this layer).
- Shadow lifecycle.
- Closure-replay primitive.
- Provider integration (verify is local-only per PRD §3).
- `docs/whitepapers/` and the exploratory PRDs listed in Constraints.
- `tpatch` binary at repo root (untracked artifact).

## Revision-4 (HIGH finding fix)

External supervisor flagged that revision-3's `ListFeatureEntries()`
helper used a 2-branch check on `os.Stat(s.tpatchDir())`:

```go
if _, statErr := os.Stat(s.tpatchDir()); statErr == nil {
    return nil, fmt.Errorf("workspace corruption: ...")
}
return nil, nil  // implicit else
```

Any non-nil non-`ENOENT` stat error (EACCES, EIO, ELOOP, exotic FS
errors) silently fell through to `return nil, nil`, producing a green
empty aggregate and exit 0. Same false-green class as the rev-1/2/3
bugs — fifth occurrence of the pattern, one layer higher.

### Fix

- `internal/store/store.go::ListFeatureEntries` — replace the 2-branch
  stat check with an explicit 3-way switch on `statErr`:
  - `nil` → `.tpatch/` present, `features/` removed → workspace
    corruption error (rev-3 contract preserved verbatim).
  - `errors.Is(statErr, fs.ErrNotExist)` → `.tpatch/` also absent →
    preserve `(nil, nil)` so callers that pre-check workspace init
    are not broken.
  - default → wrap and surface: `checking workspace state at %s: %w`.
- Added `io/fs` import for `fs.ErrNotExist`.

### Tests

- `internal/workflow/verify_all_test.go::TestRunVerifyAll_TpatchDirUnstattable_SurfacesAsError`
  — workflow-level: replace `.tpatch` with a symlink-loop pair so any
  stat/read of `.tpatch` returns ELOOP. Asserts `RunVerifyAll`
  surfaces the error (non-zero exit at the CLI layer).
- `internal/cli/verify_all_test.go::TestVerifyAll_TpatchDirUnstattable_ExitsTwo`
  — CLI-level: same scenario through `tpatch verify --all`. Asserts
  `*ExitCodeError` with `Code == 2` and message references the
  workspace path or underlying FS failure.
- Both tests skip on root (`os.Geteuid() == 0`).

### Empirical note on branch coverage

The literal new switch `default` branch — i.e., `ReadDir(features)`
returns `ENOENT` while `Stat(.tpatch)` returns a non-`ENOENT`
non-nil error — requires an asymmetric FS state where the same
`.tpatch` path component succeeds for one syscall and fails
non-`ENOENT` for the other in a single snapshot. This was empirically
confirmed unreachable on macOS/Linux without TOCTOU races (any
exotic error on `.tpatch` propagates identically to both `Stat` and
`ReadDir`). The integration tests exercise the existing line-285
catch-all path with the same exotic error class (ELOOP) and serve
as regression guards for the broader contract: non-`ENOENT` errors
on `.tpatch/` must produce non-zero exit, never a silent green
empty aggregate. The new `default` branch is correct defensive code
for races and exotic FS scenarios; it is reviewed by inspection.

### Validation

- `gofmt -l .` → empty.
- `go build ./cmd/tpatch` → success.
- `go vet ./...` → clean.
- `go test ./...` → all green; +2 new tests.
- BEFORE/AFTER live repro (rev-3 contract preservation):
  `rm -rf .tpatch/features; tpatch verify --all` → both BEFORE
  (`d390322`) and AFTER produce
  `error: ... workspace corruption: .tpatch/features directory is
  missing` with exit 2. Rev-3 behavior preserved exactly.
- BEFORE/AFTER live repro for the rev-4 branch itself was not
  staged because the asymmetric FS state required is unreachable
  in a deterministic snapshot (see "Empirical note" above). Repro
  attempts using symlink loops at `.tpatch` are intercepted earlier
  by `FindProjectRoot`/`fileExists` (which itself uses `os.Stat`),
  so the CLI exits 2 with a "could not find .tpatch" message before
  `ListFeatureEntries` is reached. The defensive new branch is the
  only correct shape for the helper's contract.

## Revision-3 (HIGH finding fix)

External supervisor flagged that revision-2's `ListFeatureEntries()`
helper still false-greens when `.tpatch/features/` is missing entirely
(e.g. operator does `rm -rf .tpatch/features`). The `os.ReadDir` on the
features dir returns `os.ErrNotExist` and the helper swallows it as
`(nil, nil)`, producing a clean exit-0 empty aggregate. Same false-green
class as the rev-1/rev-2 bugs, one layer higher (workspace-discovery
layer).

Reproduced with `tpatch init` then `rm -rf .tpatch/features` then
`tpatch verify --all --json` against the `e7f8661` binary: exit=0,
`{"features":[],"summary":{0,0,0,0}}`.

### Fix

- `internal/store/store.go::ListFeatureEntries` — split the `ErrNotExist`
  branch on `ReadDir(featuresDir())` into two cases:
  - `.tpatch/` present → workspace corruption: return
    `fmt.Errorf("workspace corruption: .tpatch/features directory is missing")`
    so `RunVerifyAll` propagates and the CLI dispatcher exits 2.
  - `.tpatch/` also absent → preserve `(nil, nil)` so callers that
    pre-check workspace init are not broken.

  Option A was chosen (fix in the store layer alongside the rev-1/rev-2
  fixes) over Option B (stat in `RunVerifyAll`) because `RunVerifyAll`
  is the only caller of `ListFeatureEntries` (verified by grep), so the
  store layer is the canonical home for this enumeration policy and DRY
  is preserved.

- `internal/store/store.go::ListFeatures` — INTENTIONALLY UNCHANGED.
  Other call sites (`feature_deps.go`, `cobra.go`, `status_dag.go`,
  `plan_reconcile.go`, `reconcile.go`, `validation.go`, `store_test.go`)
  rely on graceful empty-handling. Per supervisor instructions.

- `internal/workflow/verify_all.go` — UNCHANGED. The existing wrapper
  `return nil, fmt.Errorf("verify --all: list features: %w", err)`
  surfaces the new error correctly to the CLI dispatcher's exit-2 path.

- `internal/workflow/verify_all_test.go` — new
  `TestRunVerifyAll_FeaturesDirMissing_SurfacesAsWorkspaceCorruption`
  exercising the missing-features-dir scenario at the workflow layer.
  Asserts the error message names "features" and "workspace".

- `internal/cli/verify_all_test.go` — new
  `TestVerifyAll_FeaturesDirMissing_ExitsTwo` exercising the same
  scenario through the cobra root: asserts exit=2 with typed
  `*ExitCodeError` whose Message names "features" and either
  "workspace" or "corruption".

The pre-existing `TestRunVerifyAll_EmptyRepo` already pins the
legitimate-empty case: a tpatch-init repo with `.tpatch/features/`
present but empty (no subdirs) → exit=0, empty aggregate. The new
fix does not regress this path (the `os.ReadDir` succeeds and returns
an empty slice; no error).

### Design decisions pinned

- **Three-state taxonomy**:
  - `.tpatch/` missing → workspace not initialized; preserved
    `(nil, nil)` so existing init pre-checks fire as today.
  - `.tpatch/` present, `.tpatch/features/` missing → **workspace
    corruption** → exit 2 with a clear error.
  - `.tpatch/features/` present but empty → legitimate empty repo →
    exit 0 with empty aggregate (matches today's "no features yet"
    semantics; pinned by `TestRunVerifyAll_EmptyRepo`).
- **Why store layer not workflow layer**: `ListFeatureEntries` has a
  single caller (`RunVerifyAll`); locating the policy in the store
  keeps the rev-1 / rev-2 / rev-3 fixes co-located and avoids a
  caller-side stat dance that would have to be duplicated by any
  future aggregate caller.
- **Why `ListFeatures` untouched**: legacy helper; six call sites
  treat empty-or-missing identically. Changing its semantics is out
  of scope and would risk regressions in unrelated commands.

### BEFORE / AFTER repro (external supervisor scenario)

```
$ tpatch init
$ rm -rf .tpatch/features
$ tpatch verify --all --json

BEFORE (binary built from e7f8661):
  exit=0
  {"schema_version":"1.0","features":[],"summary":{"passed":0,"failed":0,"skipped":0,"error":0}}

AFTER (revision-3):
  exit=2
  error: verify --all aborted: verify --all: list features: workspace corruption: .tpatch/features directory is missing
```

### Files changed (revision-3)

- `internal/store/store.go` (`ListFeatureEntries` ErrNotExist branch
  distinguishes workspace-not-init from workspace-corruption)
- `internal/workflow/verify_all_test.go` (+1 test, missing features dir)
- `internal/cli/verify_all_test.go` (+1 test, missing features dir via
  cobra root)
- `docs/handoff/CURRENT.md` (this section + test-count baseline note)

### Validation gate (revision-3)

- `gofmt -l .` → empty
- `go vet ./...` → clean
- `go build ./cmd/tpatch` → success
- `go test ./...` → all pass; `internal/workflow` and `internal/cli`
  green with the 2 new regressions.

### Test count delta

+2 tests this revision (1 workflow + 1 CLI). Going forward, only the
per-revision delta is recorded — absolute baselines drift across
unrelated work and are not load-bearing for review (per supervisor
note: prior absolute counts in this file were point-in-time snapshots
of mixed scope and should not be reconciled retroactively).

## Revision-2 (HIGH finding fix)

External supervisor flagged that revision-1's `ListFeatureEntries()`
helper still silently dropped any feature whose `status.json` could
not be `os.Stat`-ed for ANY reason (not just ENOENT). A pre-read
`os.Stat(statusPath)` swallowed permission-denied / IO / non-traversable
parent errors, producing the exact same false-green class as the
original Slice D bug — one layer above the JSON-read layer that
revision-1 fixed.

Reproduced locally with a 2-feature repo (`good` healthy + `locked`
with `chmod 000` on its feature dir): rev-1 binary returned exit=0
with `locked` ABSENT from `--json` output and `summary.error=0`.

### Fix

- `internal/store/store.go::ListFeatureEntries` — split the pre-read
  stat into two cases:
  - `errors.Is(statErr, os.ErrNotExist)` → drop silently (it's not a
    feature dir; matches today's behavior for empty dirs and
    non-feature noise).
  - any other stat error → emit a `FeatureEntry{Slug, Err: fmt.Errorf("failed to stat status.json: %w", statErr)}`
    so the existing `verify_all.go` error-row branch surfaces it as a
    `verdict=error` row. `RunVerify` is NOT invoked.

  The `LoadFeatureStatus` error branch below this is unchanged
  (revision-1 contract preserved). The new stat-error path is
  purely additive.
- `internal/workflow/verify_all.go` — unchanged. The existing
  `loadErrBySlug` branch already routes any `Err`-bearing
  `FeatureEntry` to a `Status=error` row; the new stat-failure
  entries flow through the same path.
- `internal/workflow/verify_all_test.go` — new
  `TestRunVerifyAll_StatusJSONUnstattable_SurfacesAsErrorRow`
  exercising the chmod-000 scenario at the workflow layer.
- `internal/cli/verify_all_test.go` — new
  `TestVerifyAll_UnstattableStatusJSON_ExitsTwoAndIncludesFeature`
  exercising the same scenario through the cobra root: asserts
  exit=2 with typed `*ExitCodeError`, both features present in
  `--json`, locked has `status=error` with reason mentioning
  `stat status.json`, `summary.error=1`.

The pre-existing `TestRunVerifyAll_EmptyFeatureDir_SilentlyDropped`
already pins the ENOENT-drop regression — kept as-is.

### Design decisions pinned

- **ENOENT vs other stat errors**: ENOENT is a positive signal that
  the directory is not a feature (no status.json present); silent
  drop matches today's contract. ANY other stat error is ambiguous
  evidence of an attempted feature whose entry we cannot inspect —
  surface it so the operator notices, do not assume "not a feature".
- **Test cleanup approach**: every test that `chmod 000`s a
  directory under `t.TempDir()` registers a `t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })`
  so `t.TempDir`'s recursive `RemoveAll` can re-enter the directory.
- **Root-user skip**: both new tests guard with
  `if os.Geteuid() == 0 { t.Skip("test requires non-root user (root bypasses permission checks)") }`.
  Root would `os.Stat` through the chmod-000 dir and the test would
  spuriously fail.
- **Reason wording**: `failed to stat status.json: <wrapped err>`,
  distinct from the JSON-load branch (`failed to load status.json: <err>`)
  so operators (and the CLI test assertion) can disambiguate the
  two failure layers.

### BEFORE / AFTER repro (external supervisor scenario)

Two-feature repo, `good` (state=applied, valid artifacts), `locked`
(same artifacts then `chmod 000` on the feature dir).

```
BEFORE (binary built from 67730de):
  exit=0, summary={passed:1, failed:0, skipped:0, error:0}
  features=[good]   ← locked is COMPLETELY ABSENT (the rev-2 bug)

AFTER (revision-2):
  exit=2, summary={passed:1, failed:0, skipped:0, error:1}
  features=[good (passed), locked (status=error, reason="failed to load status.json: failed to stat status.json: …: permission denied")]
```

### Files changed (revision-2)

- `internal/store/store.go` (`ListFeatureEntries` distinguishes ENOENT)
- `internal/workflow/verify_all_test.go` (+1 test, chmod-000)
- `internal/cli/verify_all_test.go` (+1 test, chmod-000 via cobra root)
- `docs/handoff/CURRENT.md` (this section + Status block bump)

### Validation gate (revision-2)

- `gofmt -l .` → empty
- `go vet ./...` → clean
- `go build ./cmd/tpatch` → success
- `go test ./...` → all pass; `internal/workflow` and `internal/cli`
  green with the 2 new regressions.

### Test count delta

417 → 419 (+2: 1 workflow + 1 CLI).

## Revision-1 (HIGH finding fix)

External supervisor flagged that the Slice D aggregate enumerator
silently dropped any feature whose `status.json` was malformed or
unreadable, producing a false-green aggregate (exit 0, no row, no
error). Root cause: `RunVerifyAll` seeded the universe via
`store.ListFeatures()`, which `continue`s past load failures. The
`error` row branch in `verify_all.go` was unreachable for this class.

### Fix

- `internal/store/store.go` — added `FeatureEntry` struct + new
  `ListFeatureEntries()` helper. Returns every directory under
  `features/` that contains a `status.json` entry (file OR
  unreadable), pairing each slug with either a loaded `*FeatureStatus`
  or a load `Err`. Existing `ListFeatures()` behavior is intentionally
  unchanged so other call sites (FEATURES.md rendering, dep walkers)
  keep their silent skip-on-broken semantics.
- `internal/workflow/verify_all.go` — `RunVerifyAll` now uses
  `ListFeatureEntries()`. Entries with `Err != nil` are added to the
  topo graph with empty deps (deterministic Kahn lex tie-break) and
  emitted as `Status=error` rows with `reason="failed to load
  status.json: <err>"`. `RunVerify` is NOT invoked for these.
  `Summary.Error++` flips `HasFailures()`, which already drives the
  exit-2 gate.
- `internal/workflow/verify_all_test.go` — three new pinned tests:
  `TestRunVerifyAll_UnreadableStatusJSON_SurfacesAsErrorRow` (the
  external supervisor's exact 2-feature repro), plus two
  edge-semantic pins: `TestRunVerifyAll_EmptyFeatureDir_SilentlyDropped`
  (directory under `features/` with no `status.json` is dropped — same
  as today's non-feature dir treatment) and
  `TestRunVerifyAll_StatusJSONIsDirectory_SurfacesAsErrorRow` (a
  `status.json` that is itself a directory surfaces as an error row,
  not silently dropped, since presence of the entry signals an
  attempted feature).
- `internal/cli/verify_all_test.go` — new
  `TestVerifyAll_UnreadableStatusJSON_ExitsTwoAndIncludesFeature`
  exercising the same scenario through the cobra root: asserts
  exit=2 with typed `*ExitCodeError`, both features present in
  `--json`, bad has `status=error`, `summary.error=1`.

### Design decisions pinned

- **Topo placement of error-row features**: included in the topo graph
  with empty deps (treated as no parents). Kahn's lex tie-break makes
  position deterministic without inventing a separate "appended at
  end" path. Documented in the new comment block in
  `verify_all.go::RunVerifyAll`.
- **Empty feature dir (no status.json at all)**: silently dropped —
  matches today's treatment of any non-feature directory and keeps
  the contract "looks like a feature" = "has a status.json entry".
- **status.json that is itself a directory**: surfaced as an error
  row. `os.ReadFile` on a directory returns an error which falls
  naturally into the load-error branch.

### BEFORE / AFTER repro (external supervisor scenario)

Two-feature repo, `good` (state=applied, valid artifacts), `bad` (status.json overwritten with `{not valid json`).

```
BEFORE (binary built from 19271f7):
  exit=0, summary={passed:1, failed:0, skipped:0, error:0}
  features=[good]   ← bad is COMPLETELY ABSENT (the bug)

AFTER (revision-1):
  exit=2, summary={passed:1, failed:0, skipped:0, error:1}
  features=[bad (status=error, reason="failed to load status.json: …"), good (passed)]
```

### Files changed (revision-1)

- `internal/store/store.go` (+`FeatureEntry`, +`ListFeatureEntries`)
- `internal/workflow/verify_all.go` (`RunVerifyAll` rewired)
- `internal/workflow/verify_all_test.go` (+3 tests)
- `internal/cli/verify_all_test.go` (+1 test)
- `docs/handoff/CURRENT.md` (this section + Status block fix)

### Validation gate (revision-1)

- `gofmt -l .` → empty
- `go vet ./...` → clean
- `go build ./cmd/tpatch` → success
- `go test ./...` → all pass; `internal/workflow` and `internal/cli`
  green with the 4 new regressions.

### Test count delta

413 → 417 (+4: 3 workflow + 1 CLI).

## Files Changed

Slice D landing — additive surface only. V3–V9 logic in
`internal/workflow/verify.go` and the closure-replay primitive untouched.

- `internal/workflow/verify_all.go` (NEW, ~270 lines) — `RunVerifyAll`,
  `AggregateReport` / `AggregateFeatureEntry` / `AggregateSummary`
  types, `WriteHumanAggregate` renderer, `HasFailures` exit-gate
  predicate, `isPostApplyState` mirror of the package-private gate.
- `internal/workflow/verify_all_test.go` (NEW, 7 tests) — topo
  ordering (3-feature hard-dep chain + independent D), pre-apply
  skip, topo-position-for-skip, malformed-but-present (carryover
  Slice C lesson — recipe garbage + zero-byte patch), all-passing
  exit-gate-zero, JSON shape, empty-repo edge case.
- `internal/cli/verify.go` (MODIFIED) — added `--all` flag, arg-shape
  validation, `runVerifyAll` dispatcher. Single-feature path
  unchanged byte-for-byte.
- `internal/cli/verify_all_test.go` (NEW, 5 tests) — empty repo,
  all-passing, malformed-feature exits 2 (carryover lesson at the
  CLI surface), `--all` rejects positional slug, `--json` envelope.
- `assets/skills/claude/tessera-patch/SKILL.md` — §4.4 bullet under
  Phase Ordering.
- `assets/skills/copilot/tessera-patch/SKILL.md` — §4.4 bullet under
  Lifecycle.
- `assets/prompts/copilot/tessera-patch-apply.prompt.md` — §4.4
  bullet under Phase Ordering.
- `assets/skills/cursor/tessera-patch.mdc` — §4.4 bullet under
  Methodology.
- `assets/skills/windsurf/windsurfrules` — §4.4 bullet under
  Methodology.
- `assets/workflows/tessera-patch-generic.md` — §4.4 bullet after
  the 7-Phase Lifecycle block.
- `assets/assets_test.go` — parity-guard extended with two anchors
  (`Verify before composing.` and `tpatch verify --all`). Existing
  16 anchors preserved.
- `docs/dependencies.md` — added "Cross-link: dependencies and
  `tpatch verify`" subsection in Apply-time semantics; explains
  hard-deps drive V7/V8 closure replay (soft-deps do not).
- `CHANGELOG.md` — `v0.6.2` entry naming `tpatch verify`, the V0–V9
  freshness overlay, the four explicit out-of-scope items, and the
  `--all`-not-shadow-interaction nuance. Links the PRD §9 slice plan.

## Test Results

```
gofmt -l .              → empty
go vet ./...            → clean
go build ./cmd/tpatch   → success
go test ./...           → all pass
```

Test count delta: 401 → 413 (+12). Breakdown:

- `internal/workflow/verify_all_test.go`: 7 new tests, all pass.
- `internal/cli/verify_all_test.go`: 5 new tests, all pass.

Parity guard inversion check (bonus per CURRENT.md): temporarily
deleted the `Verify before composing.` line from the cursor surface;
`go test ./assets/...` failed with the documented anchor message
(`missing required anchor [verify-freshness/bullet]: "Verify before
composing."`). Bullet restored, guard green again.

Smoke run on a 3-feature fixture (alpha applied + beta applied +
gamma requested):

```
verify --all (3 feature(s))
[1/3] verify alpha — passed
  ✓ ... (10-check report)
[2/3] verify beta — passed
  ✓ ... (10-check report)
[3/3] gamma — skipped: pre-apply (state=requested)
Summary: 2 passed, 0 failed, 1 skipped, 0 error
exit=0
```

Pre-apply gamma appears at its topo position (last; no parent
edges) and is skipped without invoking V0 — confirmed via the
unit test `TestRunVerifyAll_PreApplySkip` which inspects
`status.json` for an absent `Verify` field.

Revision-2 cumulative status (after both HIGH fixes):

```
gofmt -l .              → empty
go vet ./...            → clean
go build ./cmd/tpatch   → success
go test ./...           → all pass (419 tests; 401 → 413 Slice D → 417 rev-1 → 419 rev-2)
```

## Session Summary

Slice D ships as four additive surface deliverables on top of the
closed Slice C foundation:

1. **Aggregate runner.** `tpatch verify --all` reuses the unchanged
   single-feature `RunVerify` per feature; topological order via
   `store.TopologicalOrder` (Kahn, lex tie-break — same primitive
   the cascade-remove path uses). Pre-apply features (state ∉
   {applied, active, upstream_merged, blocked}) skip at their topo
   position with a synthetic row; V0 is **not** invoked, no
   `Verify` record is written. JSON envelope is
   `{schema_version, features: [...], summary: {passed, failed,
   skipped, error}}`. Exit 2 if any feature failed or errored;
   pre-apply skips alone do not flip the gate.

2. **§4.4 bullet rolled to all 6 skill surfaces** — verbatim
   "Verify before composing." paragraph plus a one-line `verify
   --all` pointer in claude/copilot/copilot-prompt/cursor/
   windsurf/generic. Each surface places the new
   `## Verify (freshness overlay)` section adjacent to the
   lifecycle/phase-ordering block.

3. **Parity-guard extension** — two new anchors
   (`Verify before composing.`, `tpatch verify --all`) added to
   `requiredAnchors` in `assets/assets_test.go`. Existing 16
   anchors preserved.

4. **`docs/dependencies.md` cross-link** — short subsection in
   Apply-time semantics noting that hard deps are also the V7/V8
   closure-replay input; soft deps are not.

5. **`CHANGELOG.md` v0.6.2 entry** — names the verb, the V0–V9
   numbered checks, the four explicit out-of-scope items, and the
   `--all`-vs-shadow non-interaction nuance.

**Carryover lesson exercised** — Slice C external supervisor
flagged that artifact-presence gates need a malformed-but-present
matrix cell. Slice D adds two such tests:
`TestRunVerifyAll_MalformedButPresent_FeatureFailsWithoutPoison`
(workflow level: garbage recipe + zero-byte patch) and
`TestVerifyAll_MalformedFeature_ExitsTwo` (CLI surface: same scenario
through the cobra root). Both assert that the malformed feature
fails AND that healthy neighbours keep their passed verdict.

**Hard constraints honoured.** No edit to V3–V9 logic in
`internal/workflow/verify.go`. No edit to the closure-replay
primitive. No edit to shadow lifecycle. The `tpatch` binary at the
repo root was built for the smoke test then deleted (it's
`.gitignore`d already from the smart-routing work). No
out-of-scope PRD or whitepaper touched.

### Predecessor session summary (Slice C dispatch handoff, retained for context)

Slice C external supervisor verdict came in APPROVED on `23af23e`. Tracking commit `08ed4e5` logged all three external verdicts (Slice C original NEEDS REVISION, rev-1 NEEDS REVISION, rev-2 APPROVED) into `docs/supervisor/LOG.md` and flipped Slice C from ⬜ to ✅ in `docs/ROADMAP.md`. Pushed the full 4-commit Slice C stack + tracking commit to `origin/main`. Archived Slice C to `docs/handoff/HISTORY.md`. This handoff stages Slice D for the next implementer dispatch.

## Next Steps

1. Hold for user direction before dispatching the Slice D implementer (per the established cadence).
2. On dispatch: implementer reads this handoff + PRD §9 Slice D + §4.4 + the Slice C archive in HISTORY.md.
3. After implementer ships, dispatch the sub-agent reviewer; on APPROVED, hand off to user for external supervisor pass.
4. After Slice D APPROVED externally, tag and ship `v0.6.2` (final M15 release).

## Blockers

None.

## Context for Next Agent

- Sub-agent reviewer prior-misses pattern is now 4 cycles deep: Slice A reviewer-1, Slice B reviewer-1, bug-fix reviewer (APPROVED WITH NOTES), Slice C reviewer-1. Slice C rev-1 + rev-2 reviewers broke the streak by following strict matrix instructions. Slice D reviewer prompts should keep the matrix-coverage discipline.
- The `tpatch` binary at repo root is now `.gitignore`d (added 2026-05-01 alongside the smart-routing work) — do NOT add it.
- `4945093` was already pushed before Slice C work; the Slice C stack itself is `32f50c8` → `5892ae0` → `23af23e` → `08ed4e5`.

## Side Work — 2026-05-01 — Smart Endpoint Routing for the copilot-api Proxy

Out-of-band fix dispatched in the same session as the Slice D queue. The
user reported `"This operation was aborted"` 500s when running
`tpatch provider set --preset copilot --model claude-opus-4.6` (and
gpt-5.5). Root cause split: the proxy's
`routes/chat-completions/handler.ts` is missing the `/v1/messages`
dispatch branch (proxy-side bug, owners notified separately) AND
tpatch was hitting `/v1/chat/completions` for Claude regardless of
what the proxy advertised on `/v1/models`.

### What landed

- `internal/provider/router.go` (NEW) — `PickProvider(cfg, *Health)`
  selects Anthropic/Responses/OpenAICompatible based on the model's
  `supported_endpoints`. Scoped to the copilot-api proxy via
  `IsCopilotProxyEndpoint`.
- `internal/provider/responses.go` (NEW) — `ResponsesProvider` for the
  OpenAI Responses API, gated behind
  `TPATCH_ENABLE_RESPONSES_PROVIDER=1` (off by default; flip when the
  upstream proxy fix lands).
- `internal/provider/errors.go` (NEW) — `ProxyUpstreamAbortedError`
  typed error + `IsProxyUpstreamAborted` + `detectProxyAbort` helper.
  Replaces the cryptic "generation returned 500" with a multi-line
  remediation message.
- `internal/provider/provider.go` — `Health` extended with
  `ModelInfo []ModelInfo`; `OpenAICompatible.Check` parses
  `supported_endpoints`. `Generate` calls `detectProxyAbort` on 500.
- `internal/provider/anthropic.go` — empty-token check relaxed to
  `token == "" && !IsCopilotProxyEndpoint(cfg)` in both `Check` and
  `Generate`; the proxy strips inbound `x-api-key`.
- `internal/provider/probe.go` — `Probe()` returns `(*Health, error)`;
  `Reachable` kept as thin wrapper. `IsCopilotProxyEndpoint` broadened
  to accept both `openai-compatible` and `anthropic` Types when URL
  contains `:4141`. Added `setForceCopilotProxy` test hook.
- `internal/cli/cobra.go` — `probedEndpoints` cache changed to
  `map[string]probedResult{health, err}`; `loadAndProbeProvider`
  routes through `PickProvider`. Dropped `AuthEnv: "GITHUB_TOKEN"`
  from the `copilot` preset (proxy strips/replaces auth headers).
- `internal/cli/copilot.go` — `ensureProviderReachable` returns
  `(*Health, error)` so the cache can hold the parsed metadata.
- `.gitignore` — added `/tpatch` rule (anchored to repo root so it
  doesn't shadow `cmd/tpatch/`).
- `docs/adrs/ADR-014-smart-endpoint-routing.md` (NEW) + index entry.
- `docs/harnesses/copilot.md` — replaced the `--auth-env GITHUB_TOKEN`
  example, documented smart routing + the `/responses`-only limitation
  and the `TPATCH_ENABLE_RESPONSES_PROVIDER` opt-in.

### Tests added

- `router_test.go` — 9-case `PickProvider` matrix (Claude/GPT-5.x/GPT-4o,
  on-proxy/off-proxy, nil health, missing model, anthropic-type on
  proxy, responses gate on/off).
- `errors_test.go` — `detectProxyAbort` matrix + `IsProxyUpstreamAborted`
  wrapping tests.
- `responses_test.go` — gate, success path, abort detection, Check.
- `anthropic_test.go` — added `TestAnthropicProxyEmptyTokenIntegration`
  + `TestAnthropicProxyAbortDetected` (use `setForceCopilotProxy`
  hook so they don't bind the privileged :4141 port).
- `provider_test.go` — `TestCheckParsesSupportedEndpoints` +
  `TestCheckMissingSupportedEndpoints`.
- `phase2_test.go` — `TestCopilotPresetNoAuthEnv` pins the dropped
  `GITHUB_TOKEN`.
- `probe_test.go` — extended `TestIsCopilotProxyEndpoint` for the
  broadened type predicate.

### Validation

- `go build ./...` clean.
- `gofmt -l .` clean.
- `go vet ./...` clean.
- `go test ./...` all packages pass (cli 23s, provider 13s, workflow 43s).

### Status of the gpt-5.5 case

`/responses`-only models still surface
`ProxyUpstreamAbortedError` until the upstream proxy team's fix lands.
`ResponsesProvider` is wired but gated; flipping the env var without
the upstream fix will not help. This is documented in
`docs/harnesses/copilot.md` and ADR-014 §"Out of scope".

### Out-of-band — does NOT block Slice D

Slice D queued work above is unaffected. The smart-routing changes
touch `internal/provider/`, `internal/cli/cobra.go` /
`internal/cli/copilot.go`, `.gitignore`, and docs only. Slice D's
edit set is `internal/workflow/verify.go` (additive),
`internal/cli/verify.go` (additive), `assets/skills/*`,
`assets/assets_test.go`, `docs/dependencies.md`, `CHANGELOG.md` — no
overlap.

## Side Work — 2026-05-01 — Interim commit-strategy doc

Exploratory session (market-research scope). User formalized the
two-commit / `Tpatch-Feature`-trailer convention currently in active
use in the `tesseraspaces` repo's `.tpatch/steering/local.md`. Lifted
into `docs/commits.md` as the interim contract until `tpatch land`
ships. Forward-compatible with PRD-tpatch-land §3.4 (single trailer
becomes a four-trailer block; no migration). DOES NOT block Slice D
— docs-only, no code path touched.

Files changed:
- `docs/commits.md` (NEW) — Path B workflow + `Tpatch-Feature` trailer
  contract.
- `docs/handoff/CURRENT.md` — this side-work entry.

Out of scope this session: code, ADRs, PRD edits, `tpatch land`
implementation. The land PRD remains gated on
`PRD-record-auto-base` + `PRD-record-collision-detection`.

## Side Work — 2026-05-02 — ADR-015 prior-art identity mapping + WP-001 §5.2 re-verification

Continuation of the 2026-05-01 market-research session. User
asked for an ADR locking how the relevant Quilt / StGit / hg-MQ /
hg-evolve / jj concepts map onto tpatch's identity model, and an
entry here so the audit doesn't get lost. WP-001 itself stays
untouched per user direction; this entry is the placeholder until
WP-001 is opened for an amendment.

DOES NOT block Slice D — docs-only, no code path touched.

### Files changed (this slice)

- `docs/adrs/ADR-015-prior-art-identity-mapping.md` (NEW) — locks
  D1–D7 (slug = stable / patch-SHA = moving identity duality;
  versioned patches; append-only op log; refresh primitive named;
  unstack on the roadmap; explicit rejection of working-copy-as-
  commit and distributed obsolescence). Status: Accepted as a
  research framework; does not authorize implementation.
- `docs/adrs/README.md` — index entry for ADR-015.
- `docs/handoff/CURRENT.md` — this side-work entry.

### WP-001 §5.2 re-verification (2026-05-01) — to fold into WP-001 later

Post-apply.patch SHA-256 audit run on both case-study trees on
2026-05-01. Numbers below should land in WP-001 §5.2 next time the
whitepaper is opened for an amendment; recording here so they are not
lost in chat history.

**`tesseracode/copilot-api`** — 21 features with patches, **9 in
collision (43%)**, three groups:

| Group size | SHA-256 (12) | Patch size | Shared base commit |
|---|---|---|---|
| 5 | `0925b7da7612` | 14793 B | `ac4fefd337d4` (subject: `feat: implement 5 cosmetic features from tpatch stress test`) |
| 2 | `17e16d557996` | 40771 B | (not audited) |
| 2 | `78084d3de868` | 15454 B | (not audited) |

The 5-feature group is the textbook WP-001 §5.2 failure mode: one
upstream commit landed five features at once, then `record --from
<pre-commit>` captured the same diff into all five feature
directories. Shared `base_commit` already proves it.

**`tesseracode/t3code`** — 17 features with patches, **11 in one
collision group (65%)**:

| Group size | SHA-256 (12) | Patch size | Shared base commit |
|---|---|---|---|
| 11 | `f38312eb6ed5` | 137285 B | `02bcb16d06e8` |

Eleven features (incl. `copilot-cli-provider`,
`copilot-skill-discovery`, `effort-theming`,
`readme-copilot-notice`) share one byte-identical 27-file patch.
Notable: the patch's own embedded `.claude/instructions.md`
literally instructs the agent to `record --from main`, encoding the
failure pattern as guidance.

**Verdict.** WP-001 §5.2 holds. The 137285 B / 11-feature shape on
t3code matches the WP's claim verbatim. The copilot-api collisions
were already implied by the WP and now have concrete numbers.

### Upstream-divergence audit — to fold into WP-001 §5 / a freshness
note later

| Repo | `upstream.lock` | Lock vs upstream HEAD | Branch divergence | Health |
|---|---|---|---|---|
| `tpatch` (this repo) | n/a — IS the upstream | n/a | `0 / 0` vs `origin/main` | ✅ synced |
| `copilot-api` | empty scaffold (no commit / remote / branch fields) | n/a | `47 ahead / 0 behind` `ericc-ch/master`, base `0ea08fe` (v0.7.0) | ✅ ahead, but lock not initialized |
| `t3code` | pinned `9df3c640` (2026-04-17) | `0 ahead / 51 behind` upstream/main | `20 ahead / 24 behind` `pingdotgg/main`, base `ada410bc` (v0.0.21) | ⚠️ 14 days / 51 commits stale, reconcile pending |

Implication for `tpatch` itself: `tpatch reconcile` should detect
the empty-scaffold case (copilot-api) and prompt for `tpatch init
--upstream` rather than silently no-op. Worth a follow-up PRD or a
defensive check in the reconcile entrypoint. Out of scope for this
session.

### Open follow-ups (logged here, not assigned)

1. WP-001 §5.2 amendment to fold the 2026-05-01 audit numbers into
   the canonical claim list.
2. PRD-patch-generation-history — implementation of ADR-015 D2
   (versioned patches, no in-place rewrites).
3. PRD-tpatch-oplog — implementation of ADR-015 D3 (append-only
   `.tpatch/oplog/`, schema, command surface).
4. Defensive check in `tpatch reconcile` for empty `upstream.lock`
   scaffold — gates a clear remediation prompt.
5. PRD/ADR for explicit `tpatch refresh` alias (ADR-015 D4 deferral).
6. PRD/ADR for `tpatch feature unstack` (ADR-015 D5 deferral, gated
   on D2 + D3).

Out of scope this session: any code, the WP-001 §5.2 amendment
itself, the follow-up PRDs above, and any change to `assets/`. The
ADR is a research framework — Status: Accepted does not authorize
implementation; it locks vocabulary for future PRDs / ADRs.

## Side Work — 2026-05-02 — Competitive landscape doc + ADR-015 amendment (gbp-pq)

Continuation of the same exploratory market-research session. User
flagged `gbp pq` (git-buildpackage's patch-queue tool — Debian
ecosystem, ~2007, mature) as a missing prior-art system worth
mapping. Researched it; the round-trip `import` ↔ `export` workflow
is the closest analog to our Path B refresh primitive in any system
researched to date.

DOES NOT block Slice D — docs-only, no code path touched.

### Files changed (this slice)

- `docs/competitive-landscape.md` (NEW, ~340 lines) — strategic
  positioning artifact. Covers:
  - 3-lane competitor set (patch-mgmt, stacked-PR, commercial),
  - extended concept map (ADR-015 table + gbp-pq + new rows),
  - PESTEL refresh,
  - SWOT 4-quadrant (incl. honest weaknesses — empty `upstream.lock`,
    no op log, no versioned history, no battle-tested multi-
    collaborator story),
  - SMART target for v0.7 anchored to WP-001 §5.2 audit numbers,
  - **Strategy Canvas** with 10 axes × 8 systems, scored 0–5,
    showing the four-axis moat (provider-assisted reconcile,
    skill emission, replay recipe, lifecycle phases) where every
    competitor scores 0,
  - "What we take / What we leave / Our edge" tables.

  **Subsequently relocated** to
  `docs/market-research/competitive-landscape.md` and extended
  with a Business Model Canvas — see the next side-work block
  for that follow-up.
- `docs/adrs/ADR-015-prior-art-identity-mapping.md` — `## Amendments`
  block added. gbp-pq fits D1–D7 cleanly; **`--time-machine N`** is
  a new concrete direction (single-repo upstream-search heuristic;
  not distributed obsolescence — D7 still defers that). Logged as
  a v0.8 PRD candidate.
- `docs/handoff/CURRENT.md` — this side-work entry.

### What gbp-pq taught us (one-liner each)

- `Gbp-Pq: Name` per-patch tag = per-patch precedent for ADR-015 D1
  (stable identity that survives renumbering).
- `import` ↔ `export` round-trip = precedent for ADR-015 D4 (Path B
  as the canonical refresh primitive). May motivate `tpatch refresh`
  sugar sooner.
- `--time-machine N` = walk upstream history N commits back to find
  a base where the patch applies. **New PRD candidate** for v0.8.
- DEP-3 header standard = ~15-year precedent for trailer-based
  identity, validates PRD-tpatch-land §3.4.
- "Drop-and-recreate the patch-queue branch" = explicit opposite of
  ADR-015 D2 — both decisions are internally consistent; tpatch's
  trade-off (patches are the truth) holds.

### Open follow-ups added

7. PRD candidate (v0.8 stretch): `tpatch reconcile --time-machine N`
   per gbp-pq's import behaviour. Single-repo only.
8. Cite gbp-pq (DEP-3 + `Gbp-Pq:`) and git-gud (`GG-ID`/`GG-Parent`
   trailers) as prior art in PRD-tpatch-land §3.4 when next opened
   for revision.
9. `docs/market-research/competitive-landscape.md` is a **living
   doc** — refresh triggers documented in §"Refresh triggers".
   When a new system enters the picture, add a column. Don't let
   it bit-rot.

Out of scope this session: any code, the PRDs / ADRs the new doc
points at, and any change to `assets/`.

## Side Work — 2026-05-02 — Market-research folder + Business Model Canvas

Continuation of the same exploratory market-research session. User
asked to (a) add a Business Model Canvas alongside the existing
Strategy Canvas, and (b) move `competitive-landscape.md` into a
dedicated subfolder so future research docs (deep-dives, persona
analyses, industry reports) have a home. Both done.

DOES NOT block Slice D — docs-only, no code path touched.

### Files changed (this slice)

- `docs/market-research/` (NEW DIR).
- `docs/market-research/README.md` (NEW) — folder index. Documents
  what kinds of docs go here (competitive landscape, deep-dives,
  persona analyses, industry reports), what does **not** go here
  (ADRs, PRDs, whitepapers — points at the right home for each),
  doc conventions (living-by-default, kebab-case filenames,
  Status/Date/Owner header, end-with-Disputes-block), and an
  authoring checklist.
- `docs/market-research/competitive-landscape.md` (MOVED FROM
  `docs/competitive-landscape.md`) — relative paths in the
  `**Related**` header updated (now use `../adrs/`,
  `../whitepapers/`, `../prds/`, `../commits.md`,
  `../handoff/CURRENT.md`). New `## 7. Business Model Canvas`
  inserted after `## 6. Strategy Canvas` (paired-canvas flow);
  subsequent sections renumbered to §8–§13. The BMC covers all 9
  Osterwalder blocks: customer segments, value propositions,
  channels, customer relationships, revenue streams (currently
  none — pre-revenue / OSS; hypothetical paths surfaced as
  deferred decisions), key resources, key activities, key
  partnerships, cost structure. Synthesis surfaces three findings
  the Strategy Canvas alone doesn't: **asymmetric cost model**
  (no infra burn vs entire.io), **agent-mediated relationship
  surface as primary UX**, and **monetization is a deferred
  decision, not absent**.
- `docs/adrs/ADR-015-prior-art-identity-mapping.md` — amendment
  block link path updated to point at the new location, and the
  framework name extended to "PESTEL / SWOT / SMART / Strategy
  Canvas / Business Model Canvas".
- `docs/handoff/CURRENT.md` — this side-work entry + a forward
  pointer added to the previous 2026-05-02 entry for traceability.

### Why a folder

Three or four future research docs are easy to imagine (deep-dive
on jj methodology after a major release; deep-dive on entire.io
once their roadmap is public; persona doc when we have enough
case-study data for one; industry report when the next FM
context-window jump lands). Having `competitive-landscape.md`
loose in `docs/` with that future already on the horizon would
have been cluttered. Folder + README index matches the existing
convention from `docs/adrs/`, `docs/prds/`, `docs/whitepapers/`.

Out of scope this session: any code, the PRDs / ADRs the doc
points at, and any change to `assets/`.

## Side Work — 2026-05-03 — Five Forces, personas, WP-001 §5.2 amendment, Pijul, org rename

Continuation of the same exploratory market-research session. User
asked to (a) add **Porter's Five Forces** as the highest-leverage
missing strategy framework, (b) write a **personas / JTBD** doc
for the v0.7 audience commitment (fork maintainers — businesses
with custom implementations, platform teams, security teams
patching CVEs ahead of upstream), (c) **fold the WP-001 §5.2
re-verification numbers** into the whitepaper now (deferred from
2026-05-02), (d) add **Pijul** (and by extension darcs) as
patch-theory references, and (e) update **case-study repo refs**
from `tesserabox/...` to `tesseracode/...` after the org rename.
All five done.

DOES NOT block Slice D — docs-only, no code path touched.

### Files changed (this slice)

- `docs/whitepapers/WP-001-feature-slice-gap.md` — new
  `### Amendment — 2026-05-01 — Re-verification on current
  case-study state` block inserted into §5.2 (between "Asks of
  next agent" and `## 6. Open questions`). Records: A1 = 21
  patches / 9 in 3 collision groups (5/2/2), 5-group base
  `ac4fefd337d4` whose subject literally says
  `feat: implement 5 cosmetic features from tpatch stress test`;
  A2 = 17 patches / 11 in one byte-identical 137 285 B 27-file
  collision (base `02bcb16d06e8`) where `.claude/instructions.md`
  embedded inside the patch instructs the agent to
  `tpatch record --from main` — failure encoded as guidance.
  Headline finding (no row reclassified) preserved at greater
  scale.
- `docs/market-research/competitive-landscape.md`:
  - **§1 Lane D (NEW)** — darcs + Pijul as patch-theory DVCSes
    (out-of-band reference frame).
  - **§2 patch-theory commentary (NEW)** — short paragraph after
    the existing concept-map text explaining why we don't adopt
    patch theory.
  - **§4 Porter's Five Forces (NEW)** — inserted between PESTEL
    (§3) and SWOT. Two forces dominate planning: supplier power
    (LLM provider lock-in) and substitute pressure (regenerate-
    with-agent). Subsequent sections renumbered §4 → §5, …,
    §13 → §14.
  - **Internal cross-refs updated** in five places (Lane B
    "see §7" → §9; BMC intro "Strategy Canvas in §6" → §7;
    BMC §1 "see §8" → §10 — also corrects an off-by-one
    authoring error pointing at "What we take" instead of
    "What we don't take"; cross-refs `(§7)` → `(§9)`; cross-refs
    "§6 axis 4" → "§7 axis 4").
- `docs/market-research/personas.md` (NEW, ~290 lines) — three
  primary personas (Platform Pat / Security Sam / Maintainer
  Mira) with JTBD framing per persona, pain points, how tpatch
  fits, and counter-evidence (where tpatch fits poorly today).
  Adjacent personas table for general devs / distro packagers /
  upstream maintainers / multi-fork enterprise teams. Common
  JTBD synthesis. Cross-references to BMC §8, SMART §6, Five
  Forces §4, WP-001 §5.2, PRD-tpatch-land, `docs/commits.md`.
- `docs/market-research/README.md` — index entry for
  `personas.md`; bumped `competitive-landscape.md` last-refresh
  date to 2026-05-03.
- `docs/adrs/ADR-015-prior-art-identity-mapping.md` — new
  `### 2026-05-03 — Patch-theory DVCSes (darcs, Pijul) added to
  the prior-art set` amendment block. D1–D7 all hold; patch
  theory validates the *direction* of D1 / D2 / D3 / D5 but
  cannot be ported onto a snapshot substrate (SPEC commitment).
- **Org rename** `tesserabox/...` → `tesseracode/...` for
  case-study repo references. Updated in:
  `docs/whitepapers/WP-001-feature-slice-gap.md`,
  `docs/whitepapers/WP-001-feature-slice-gap.turns.md`,
  `docs/handoff/CURRENT.md` (this file),
  `docs/market-research/competitive-landscape.md`,
  `docs/prds/PRD-tpatch-land.md`,
  `docs/commits.md`, and
  `docs/adrs/ADR-015-prior-art-identity-mapping.md`.
  `docs/handoff/HISTORY.md:2044` deliberately preserved — that
  reference is to the tesserapatch project's own module-path
  rename (`github.com/tesseracode/tesserapatch (renamed from
  tesserabox on 2026-04-21)`), a historical record of when the
  rename happened, not a current case-study repo reference.

### Open follow-ups logged (not assigned, not scheduled)

- **`tpatch hotfix` shortcut for Security Sam.** Lifecycle phases
  are heavyweight for "fix this CVE in 30 minutes." Logged as a
  personas-driven backlog candidate.
- **`CVE` field in the trailer block.** Sam-driven request;
  forward-compatible with PRD-tpatch-land §3.4.
- **"Patch already upstream" detector.** Surfaces in two persona
  pain lists (Pat audit + Sam SLA). Reinforces existing follow-up
  in `competitive-landscape.md §9` (gbp-pq's
  auto-detect-applied).
- **Solo-friendly onboarding for Maintainer Mira.** Documentation
  corpus implies enterprise audience; Mira persona surfaces this
  as a real friction.
- **Time-machine reconcile** (gbp-pq inspiration) remains a
  v0.8 PRD candidate (logged twice now: ADR-015 amendment
  2026-05-02; reaffirmed by personas.md).

### Why a full sweep this session

The user's review during 2026-05-02 ↔ 2026-05-03 surfaced four
strategic frameworks worth running before any v0.7 PRD work
starts: PESTEL (macro), SWOT (firm-level), Strategy Canvas
(axis-by-axis differentiation), Business Model Canvas (operating
model). Adding Porter's Five Forces (competitive dynamics)
completes the canonical set; adding personas concretizes BMC §1.
The market-research folder is now self-contained for any future
PRD / roadmap question that turns into a positioning question.

Out of scope this session: any code, the PRDs / ADRs the new
material points at, any change to `assets/`, and any modification
of WP-001 outside the §5.2 amendment block.


---

## Side Work — 2026-05-09 — Cursor+Graphite update + hotfix / patch-already-upstream PRDs

**Files Changed**:

- `docs/market-research/competitive-landscape.md`:
  - **§1 Lane B** — added Graphite row + note that the Cursor
    acquisition (Dec 2025) makes this the production-validated
    hg-evolve-on-Git point of reference. Cross-listed in Lane C.
  - **§1 Lane C** — added Graphite-via-Cursor row alongside
    entire.io. The lane is now "the two commercial threats":
    independent (entire.io) and AI-coding-tool-acquired (Cursor +
    Graphite, ~$300M+ Dec 2025).
  - **§3 PESTEL Political** — new bullet on AI-coding-tool M&A
    consolidation (Cursor's serial acquisitions: Supermaven,
    Koala talent, Graphite). Pattern signals platform-
    consolidation pressure on adjacent OSS tooling.
  - **§4 Five Forces** — `## 4. Porter's Five Forces (refreshed
    2026-05-03; Cursor+Graphite update 2026-05-09)`.
    Substitutes ladder bumped 2 → 4 with Cursor+Graphite "self-
    driving PR" entry as a stronger tier than vanilla
    regenerate-with-agent. Rivalry verdict raised **MEDIUM →
    MEDIUM-HIGH** with two-tier rivalry framing (direct rivals
    in our seam: entire.io; adjacent rivals: Cursor+Graphite as
    well-funded buyer/builder one acquisition pivot away).
    Synthesis table updated.
  - **§5 SWOT Threats** — added Cursor-or-other-AI-coding-tool-
    acquiring-fork-management-capability bullet (entire.io is
    the obvious target). Added "self-driving PR framing taking
    mindshare" cultural threat with the "lifecycle + structured
    intent" counter.
  - **§5 SWOT Opportunities** — added Cursor+Graphite-as-
    closed-source bullet (widens the OSS-positioning seam to a
    two-against-many posture) and Graphite-auto-rebase-as-
    production-validated-prior-art for ADR-015 D3 graduation.
  - **§11 "Our edge"** — refreshed entry 1 to acknowledge
    Cursor+Graphite as an additional Lane-C bundle threat the
    OSS posture moats against.
  - Header refreshed: `Last refresh: 2026-05-09`.
- `docs/market-research/README.md` — bumped competitive-landscape
  last-refresh date 2026-05-03 → 2026-05-09.
- `docs/prds/PRD-tpatch-hotfix.md` (NEW, ~340 lines) — fast-path
  lifecycle for CVEs / time-boxed patches. Adds `tpatch hotfix
  <slug>` (compress analyze/define/explore into one invocation)
  and `tpatch hotfix promote <slug>` (retroactive backfill).
  Schema: `FeatureStatus.Kind {feature, hotfix}` and optional
  `FeatureStatus.CVE`. Trailer-block extension: additive
  `Tpatch-CVE:` line. Backwards compatible via `omitempty`.
  Behind `Config.HotfixEnabled` (default false).
- `docs/prds/PRD-patch-already-upstream-detector.md` (NEW, ~360
  lines) — deterministic `upstream_merged` fast path via
  `git patch-id --stable` sweep over the upstream-since-lock
  range. Slots in as **phase 1.5** between existing phase 1
  (reverse-apply) and phase 2 (operation-level). Conservative
  default (suggest, don't auto-drop); opt-in `--auto-drop-merged`
  flag. New `--check-applied-only` read-only verb. Optional
  `PatchIDMatch` audit field. Heuristic-fallback friendly
  (no provider needed). Behind `Config.PatchIDDetectorEnabled`
  (default false). Hotfix-kind features get auto-drop ON when
  the global flag is also ON (double-gate).

**Out of Scope This Session**:

- Code changes — both PRDs are research-driven proposals; no
  implementation has been authorized.
- `docs/prds/README.md` — already stale (says "No PRDs yet"
  despite 12+ existing PRDs). Adding only the two new entries
  would inconsistent with the rest. Flagged for a holistic
  refresh in a future session.
- ADR-015 amendment — Graphite is hg-evolve-on-Git for the
  stacked-PR seam; the D1–D7 framing already covers hg-evolve.
  No ADR-015 update needed unless / until D3 (op log) graduates
  to a PRD that wants Graphite's auto-rebase as cited prior art.
- WP-001 — not edited (per user direction earlier in the session
  series: "lets not edit WP just now, we can add the ADR…").
- Slice D / verify code paths — explicitly delegated to a
  separate agent in parallel.

**Open Follow-ups Logged (newly named or still parked)**:

- **PRD-tpatch-hotfix Open Question 1** — provider-required vs
  heuristic-fallback for `tpatch hotfix promote`. Default-
  position favours provider-required.
- **PRD-tpatch-hotfix Open Question 4** — `tpatch list --kind`
  default visibility.
- **PRD-patch-already-upstream-detector Open Question 1** — cap
  on the scanned upstream range (5000 commits is a guess).
  Validate against real fork-vs-upstream lag (t3code ~5 days
  behind, copilot-api ahead).
- **PRD-patch-already-upstream-detector Open Question 4** —
  whether to introduce a separate `ReconcileUpstreamedDeterministic`
  outcome enum value or keep distinguishing via `Phase`.
  Default-position: keep existing outcome.
- **`docs/prds/README.md` is stale** — says "No PRDs yet" while
  12+ PRDs exist. Holistic refresh needed; out of scope here.
- **Persona pain-ranking validation.** Logged previous session;
  still parked.
- **Persona JTBD validation against real users.** Logged
  previous session; still parked.
- **`tpatch refresh` alias** — Strategy Canvas axis 4 cheap fix.
  Still on the v0.7.x candidate list.
- **ADR-015 D2 / D3 graduation PRDs** — versioned patch history
  + cross-feature op log. Highest-leverage Strategy Canvas gaps
  (axes 2, 3). Cite Graphite's `gt modify` auto-rebase as
  production-validated prior art when D3 graduates.
- **Time-machine reconcile** (gbp-pq inspiration) — separate
  PRD candidate, complementary to the patch-id detector PRD
  (the detector handles the *match* case; time-machine handles
  the *no-match-but-rebase-finds-a-base* case).

**Why this slice this session**

The user explicitly asked to add Graphite to the competitive
landscape (Cursor acquisition was the trigger) and to draft PRDs
for the two highest-leverage persona counter-evidence items
(`tpatch hotfix` for Security Sam; patch-already-upstream
detector for Pat audit + Sam CVE-drop SLA). Slice D was
delegated to another agent in parallel. The remaining persona
follow-ups (#3 / #4 / #5 / #6 from prior session enumeration)
remain parked.

---

## Side Work — 2026-05-09 — Hotfix PRD §3.4 trailer-block fix (PRD-tpatch-land reviewer note)

**Trigger**: `PRD-tpatch-land.md:377-386` "Cross-PRD note for the
supervisor" flagged `PRD-tpatch-hotfix.md` §3.4 as displaying a
non-canonical trailer block.

**Verified errors**:
- Showed `Tpatch-Slug: <slug>` — redundant with `Tpatch-Feature` (same value).
- Showed `Tpatch-Phase: applied` — not in PRD-tpatch-land §3.4's locked
  four; no prior-art rationale in `competitive-landscape.md §9 / §11`.
- **Omitted** `Tpatch-Recipe-SHA` and `Tpatch-Base-Commit` from the
  canonical four — the more serious bug, since it would have implied
  a partial trailer-block contract.

**Fix applied** to `docs/prds/PRD-tpatch-hotfix.md` §3.4:
- Replaced the example block with the canonical four
  (`Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`,
  `Tpatch-Base-Commit`) + additive `Tpatch-CVE` when set.
- Added explicit ordering rule (`Tpatch-CVE` after the four, before
  the repo-level `Co-authored-by:` trailer per CLAUDE.md rule 8 /
  land §3.4).
- Added "Authoritative emitter" sub-paragraph clarifying `land` does
  not emit `Tpatch-CVE` — the hotfix verb owns it. This is the
  authoritative restatement of land §3.4 "Coordination with
  PRD-tpatch-hotfix".
- Anchored the grep target as `'^Tpatch-CVE:'` (was unanchored).

**Out of scope**: PRD-tpatch-land's "Cross-PRD note for the
supervisor" block (lines 377-386) is now stale — the issue it flags
is fixed. The land PRD's agent owns that block; not edited here. A
supervisor pass during graduation review can drop or shorten it.

---

## Side Work — 2026-05-10 — G55 guardrail PRD review-response pass

**Trigger**: human broker asked G55 to apply CO47's review of
`PRD-record-auto-base.md` and `PRD-record-collision-detection.md`,
then cross-review CO47's `PRD-tpatch-land.md` v2. WP-001,
`PRD-tpatch-land.md`, exploratory PRDs, and implementation code were
kept read-only.

**Edits applied**:
- `docs/prds/PRD-record-auto-base.md`: accepted R2/S1/S4 and added
  market-research grounding. The merge-base fallback path now refuses
  inferred ranges with more than one commit by default, prefers remote
  default branches before hard-coded `main`, and has file:line claims
  audit evidence.
- `docs/prds/PRD-record-collision-detection.md`: accepted S2/S3/S4
  and added market-research grounding. Same-feature duplicate handling
  now treats canonical byte-identical writes as no-ops, skips only the
  numbered audit snapshot, and explicitly skips collision scanning for
  empty patches.

**Cross-review status**:
- CO47's F1-F5 land v2 fixes are confirmed in the current draft:
  land-specific dirty-tree preflight, no new-HEAD write to
  `apply.base_commit`, Pattern A metadata-only out of scope, `--auto`
  flag forwarding, and refreshed citations.
- Hotfix trailer coordination is resolved in the current files:
  `PRD-tpatch-hotfix.md` §3.4 now shows the canonical four trailers
  plus additive `Tpatch-CVE`, and land §3.4 marks the note resolved.
- Minor residual review note for supervisor: `PRD-tpatch-land.md` §6.1
  paraphrases the SMART deliverables as `land` + record collision
  detection + `record --auto`, while `competitive-landscape.md` §6
  currently names `land` + record collision detection + reconcile
  upstream-lock validation guard, with auto-base as the remediation
  mechanism. Align wording before graduation.

---

## Side Work — 2026-05-10 — PRD-reconcile-lock-guard (OX47)

**Trigger**: CO47 (PRD-tpatch-land owner) brokered a request to
draft the third v0.7 ship target named in
`competitive-landscape.md §6 SMART` and `PRD-tpatch-land §6.1`.
Reviewed against reality (SMART text confirmed at lines 475-478;
`PreflightReconcile` confirmed at `internal/gitutil/gitutil.go:117`;
`UpstreamLock` confirmed at `internal/store/types.go:344-350`; no
existing PRD covers the seam). Verdict: worth doing, well-grounded.

**Agent ID picked**: **OX47** (Claude Opus 4.7 Extra-high reasoning),
distinct from CO47 (Claude Opus 4.7 base). Per user direction.

**Files Changed**:

- `docs/prds/PRD-reconcile-lock-guard.md` (NEW, ~830 lines) —
  preflight guard validating that `.tpatch/upstream.lock.commit` is
  reachable from `<lock.Remote>/<lock.Branch>` HEAD before
  `tpatch reconcile` runs. Four lock states (Valid / Empty / Missing
  / Stale + the SKIPPED domain-mismatch case under
  `--upstream-ref`); strict refuse on Stale, warn-and-proceed on
  Empty / Missing (preserves v0.6 init-scaffold compatibility).
  Override flag `--allow-stale-lock` mirrors G55's
  `--allow-collision <reason>` pattern. Wires into existing
  `ReconcilePreflight` per CO47's "no new preflight type"
  constraint. No new data-model objects; sibling-of-WP-001 framing
  in §0.1. Shared `LoadUpstreamLock` parser primitive coordinated
  with `PRD-record-auto-base §5` in §5.1-5.4 (one parser, two
  consumers; lives in `internal/store/upstream_lock.go`).
- `docs/prds/PRD-patch-already-upstream-detector.md §3.1` — added
  one-line precondition cite to `PRD-reconcile-lock-guard`
  (the lock-guard validates the precondition phase 1.5's scan
  range depends on; the detector itself does no lock validation).

**Cross-PRD coordination locked**:

- **PRD-record-auto-base §5** (G55): `LoadUpstreamLock` lives in
  `internal/store/upstream_lock.go`. Whichever PRD ships first
  writes it; the other consumes it. Contract spelled out in
  `PRD-reconcile-lock-guard §5.2`.
- **PRD-tpatch-land §6.1** (CO47): land's positioning of the
  lock-guard PRD as the third v0.7 deliverable
  (implementation-independent of land) is correct. No drift.
  Confirmed in `PRD-reconcile-lock-guard §11` cross-review note.
- **PRD-patch-already-upstream-detector §3.1** (OX47, prior
  session): cites the lock-guard as its precondition. Enforced
  one-line edit to that PRD as part of this slice.
- **PRD-tpatch-hotfix §5.3** (OX47, prior session): hotfix flow
  inherits guard semantics for free; no edit needed there.

**Out of Scope This Session**:

- Code — paper design only.
- ADR — supervisor-discretion at acceptance time per CO47's brief.
- Edits to WP-001, the three exploratory PRDs, or any other
  agent's PRD (per CO47's constraint).
- A new reconcile phase — explicitly disallowed by CO47's brief.
- New data-model objects — explicitly disallowed by CO47's brief.

**Open Questions Logged in the PRD (§9)**:

1. Override-audit persistence — `ReconcileSummary` field vs
   session-artifact-only. Default-position: session-only for v1.
2. `--fetch-before-guard` flag — defer to v0.8+.
3. Override-stacking policy (`--allow-dirty --allow-stale-lock`) —
   default-position Option A (allow stacking, both warnings emit).
4. Warn-vs-refuse default flip — default stays refuse.
5. Whether `--allow-dirty` should auto-suppress lock-guard —
   default-position no.
6. Lock state in `--json` output — default-position yes
   (`"lock_state": "valid|empty|missing|stale|skipped"`).

**Verdict on PRD-tpatch-land §6.1 SMART target wording**

Correct. §6.1 (lines 651-672 of `PRD-tpatch-land.md`) names
PRD-reconcile-lock-guard as deliverable (3),
implementation-independent of `land`, with `--auto` as the
remediation mechanism. Matches both `competitive-landscape.md §6
SMART` and the new PRD's framing. No drift; no edit requested.

**Other findings or risks**

- The override-stacking risk (`--allow-dirty --allow-stale-lock`)
  is the largest UX risk; flagged in PRD §9.3 with explicit
  re-evaluation at v0.7+30 days.
- Backwards-compat for v0.6 init scaffolds is the second risk;
  resolved by treating Empty as warn-only (PRD §6.1).
- A potential follow-up not in this PRD: `tpatch upstream check`
  (SPEC.md:71) is declared but its implementation status was not
  re-verified. The diagnostic falls back to manual recovery if
  the verb is stubbed; PRD §3.4 explicitly handles both cases.
- Cross-review pass by CO47 + G55 will follow before formal
  supervisor acceptance per CO47's brief.

## Side Work — 2026-05-10 — PRD-reconcile-lock-guard revision pass (OX47)

Cross-reviews from CO47 (6 findings) + G55 (3 findings) applied to
`docs/prds/PRD-reconcile-lock-guard.md`. File now 684 lines.

**CO47 required edits applied**: F1 (third-vs-fourth deliverable
framing in §0.1 ¶2 fixed), F2 (WP-001 cite split — §9 graduation
plan + `WP-001-feature-slice-gap.turns.md` Turn 13), F3 ("audit-emit"
→ plain English), F4 (parser tolerance tightened to double-quoted
only).

**CO47 suggestions applied**: F5 (option 3 in stale refusal block
clarifies ref-name difference), F6 (§7.2 leads with sibling-function
`PreflightReconcileWithOverride`, demotes breaking-signature change).

**G55 high-severity findings addressed**:
- #1 writer-normalization: NEW §5.3 requires `updateUpstreamLock`
  fix (split `--upstream-ref` into remote+branch); §4 step 4 adds
  read-side legacy tolerance for v0.4 lock format
  (`branch: upstream/main`).
- #2 `Clean()` conflation: §4 algorithm note + §7.3 pseudocode
  corrected — `Clean()` stays working-tree-only; lock-state gate
  evaluated independently at the cli call site.
- #3 ref-name comparison: §4 step 5 specifies
  `git rev-parse --symbolic-full-name`, not SHA equality.

**Acceptance criteria additions**: §8 #16-#20 cover writer
normalization, legacy-lock tolerance, symbolic-ref-name comparison,
sibling-function coexistence, independent-gate evaluation.

**One factual note for the broker**: CO47's F2 claim that the
WP-001 main file is "~607 lines, line 694 doesn't exist" is wrong
— the file is 727 lines and `:694` references the §9 graduation
plan's reference to T13. CO47's *suggested fix* (split-cite to §9
+ turns.md Turn 13) is more precise and was applied. Original
cite was technically valid but not the most direct.

**Authority question on `PRD-patch-already-upstream-detector.md`**:
CO47 flagged the precondition cite as borderline scope-creep
because that PRD's owner field reads "Core". Clarification: I am
the author of that PRD this session; "Core" reflects no
implementation owner, not a different agent. The cross-cite is a
one-line coordination link, factually correct. Deferring to broker
if revert is preferred.

**No code touched.** `go test ./...` was green at session start.
