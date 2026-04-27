# Current Handoff

## Active Task
- **Task ID**: M14.4
- **Milestone**: M14 — Feature Dependencies / DAG (Tranche D, v0.6.0)
- **Description**: User-facing cutover for the feature-dependency DAG. Seven chunks (A–G): `tpatch status --dag`, default flip, dep-management verbs, status-time validation, 6-skill rollout, `docs/dependencies.md`, release commit.
- **Status**: **Implementation complete — awaiting reviewer**
- **Assigned**: closed 2026-04-26

## Session Summary

All 7 chunks landed on `main` (not yet pushed at write time of this file; push will be the next action). Six logical commits (A+D combined). v0.6.0 NOT tagged — that is supervisor's closeout.

| Chunk | Title | Commit | Headline |
|-------|-------|--------|----------|
| A + D | `tpatch status --dag` + status-time DAG validation | `d1aca5f` | ASCII (`─►` hard / `┄►` soft) + `--json`, scoped + full, cycle-safe; `ValidateAllFeatures` warnings inline on plain `tpatch status`. Reads `status.Reconcile.Outcome` only (ADR-010 D5). 9 new tests. |
| C     | Dep-management verbs                              | `ca23b35` | `tpatch feature deps [<slug> [add\|remove] <parent>[:hard\|:soft]] \| --validate-all`; `tpatch amend --depends-on / --remove-depends-on` (deps-only mode skips request.md rewrite); `tpatch remove --cascade` reverse-topo + `ErrInteractiveRequired` for non-TTY without `--force`; **`--force` does not bypass dep-integrity** (PRD §3.7 / ADR-011 D7). 9 tests + non-TTY pipe helper (because `/dev/null` is a char device on macOS). |
| B     | Flag default flip                                 | `5d5f594` | `parseYAMLConfig` defaults `features_dependencies` to `true` when absent; `Init()` template writes the explicit `true`. Two byte-identity tests (apply gate-off, accept-shadow labels-nil) updated to opt out explicitly. Roundtrip test inverted. |
| E     | 6-skill rollout                                   | `97a994f` | All six shipped skill formats updated atomically with the dependency surface; `created_by` description reframed from "inert" → live apply-time gate. Parity guard (`assets_test.go`) holds. |
| F     | `docs/dependencies.md`                            | `e0a7d47` | Single user-facing reference (~270 LOC): edge model, declaration, validation, apply gate, `created_by` op-level gate (with dry-run downgrade), reconcile labels, compound verdict, `status --dag` examples, `--cascade` contract, migration, out-of-scope list. |
| G     | Release cutover                                   | `f2d0d1b` | Version `0.5.3` → `0.6.0`; new `## v0.6.0 — 2026-04-26 — Feature Dependencies (Tranche D)` CHANGELOG section; M14 box flipped 🔨 → ✅ in ROADMAP; obsolete "Feature dependency DAG" bullet pruned from M15+ Future. **NO TAG.** |

## Current State

`main` carries `f2d0d1b`, six commits ahead of `origin/main`. Build is clean, full test suite is green, parity guard holds, scoped DAG/label/dependency tests are all green. Ready for reviewer.

## Files Changed

Created:
- `internal/cli/status_dag.go` — ASCII tree + JSON renderer for `tpatch status --dag`.
- `internal/cli/status_dag_test.go` — 9 tests (chunks A + D).
- `internal/cli/feature_deps.go` — `feature deps` command tree, `applyAmendDependsOn`, `runRemoveWithCascade`, `collectSubtree`, sentinel `ErrHasDependents` + `ErrInteractiveRequired`.
- `internal/cli/feature_deps_test.go` — 9 tests (chunk C).
- `internal/cli/test_helpers_test.go` — `openDevNull()` non-TTY pipe helper.
- `docs/dependencies.md` — user reference (chunk F).

Modified:
- `internal/cli/cobra.go` — `featureCmd()` registered on root; `--dag` flag wired onto status; status-time `ValidateAllFeatures` warnings; `version` bumped to `0.6.0`.
- `internal/cli/c1.go` — `amendCmd` gained `--depends-on` / `--remove-depends-on` (deps-only mode); `removeCmd` gained `--cascade` + integrity gate.
- `internal/cli/dependency_gate_apply_test.go` — `TestApplyExecute_FlagOff_*` opts out of the new default.
- `internal/store/store.go` — `parseYAMLConfig` defaults `features_dependencies: true`; `Init()` template writes the explicit `true`.
- `internal/store/types.go` — `Config.FeaturesDependencies` doc updated.
- `internal/store/roundtrip_test.go` — `TestConfig_FeaturesDependenciesRoundtrip` inverted (default-true + explicit-false opt-out).
- `internal/workflow/accept_labels_test.go` — `TestAcceptShadow_FlagOff_LabelsRemainNil` opts out explicitly.
- `assets/skills/claude/tessera-patch/SKILL.md` — `created_by` paragraph reframed; new "Feature dependencies (v0.6.0+)" section.
- `assets/skills/copilot/tessera-patch/SKILL.md` — same.
- `assets/skills/cursor/tessera-patch.mdc` — same.
- `assets/skills/windsurf/windsurfrules` — same.
- `assets/workflows/tessera-patch-generic.md` — same.
- `assets/prompts/copilot/tessera-patch-apply.prompt.md` — same.
- `CHANGELOG.md` — new v0.6.0 section.
- `docs/ROADMAP.md` — M14 box flipped to ✅; M14.4 line expanded with chunk-level breakdown and commit shas; obsolete M15+ "Feature dependency DAG" bullet removed.

## Test Results

```
$ gofmt -l .
(clean)

$ go build ./cmd/tpatch && rm -f tpatch
ok

$ go test ./...
ok  github.com/tesseracode/tesserapatch/assets        0.441s
?   github.com/tesseracode/tesserapatch/cmd/tpatch    [no test files]
ok  github.com/tesseracode/tesserapatch/internal/cli  4.968s
ok  github.com/tesseracode/tesserapatch/internal/gitutil   (cached)
ok  github.com/tesseracode/tesserapatch/internal/provider  (cached)
ok  github.com/tesseracode/tesserapatch/internal/safety    (cached)
ok  github.com/tesseracode/tesserapatch/internal/store     (cached)
ok  github.com/tesseracode/tesserapatch/internal/workflow  (cached)

$ go test ./assets/... -count=1
ok  github.com/tesseracode/tesserapatch/assets        0.371s
    (TestAllSkillFilesExist + TestSkillRecipeSchemaMatchesCLI both green
     across all 6 formats; TestSkillParityGuard implicit via build.)

$ go test ./internal/cli      -run 'StatusDag'                       -count=1   ok 1.073s
$ go test ./internal/workflow -run 'CreatedByGate|PlanReconcile|ComposeLabels|EffectiveOutcome|AcceptShadow|GoldenReconcile|Phase35|Labels' -count=1   ok 5.551s
$ go test ./internal/store    -run 'Label|Reconcile|DAG|Dependency|Roundtrip'   -count=1   ok 0.358s
```

## Next Steps

1. Reviewer runs the standard checklist (`AGENTS.md` review phase) against the six commits `d1aca5f..f2d0d1b`.
2. If APPROVED, supervisor:
   - Tags `v0.6.0` on `f2d0d1b`.
   - Archives this handoff to `docs/handoff/HISTORY.md`.
   - Picks the next milestone (M15+ from ROADMAP).
3. If NEEDS REVISION, the implementer reads the LOG.md verdict and iterates here.

## Blockers

None.

## Context for Next Agent

- **Tag is supervisor work, not implementer work.** The release commit deliberately omits a tag. Operator instruction was explicit on this point.
- **`tpatch` binary at the repo root is NOT gitignored.** Always `rm -f tpatch` after `go build ./cmd/tpatch` — this is a recurring slip that has bitten earlier sessions.
- **Source-truth guard (ADR-010 D5):** all DAG / label / status code reads `status.Reconcile.Outcome` via `store.LoadFeatureStatus` — never `artifacts/reconcile-session.json`. The M14.3 adversarial test pins this; do not regress.
- **`--force` is NOT a DAG-integrity bypass.** It only suppresses the TTY confirm prompt on `remove`. Only `--cascade` may opt into removing a feature with downstream dependents. PRD §3.7 / ADR-011 D7. The chunk-C tests pin this.
- **Default-flip compatibility:** v0.5.3-byte-identity behaviour is recoverable per-repo via `features_dependencies: false` in `.tpatch/config.yaml`. Two existing tests demonstrate the opt-out path (`TestApplyExecute_FlagOff_BypassesDependencyGate`, `TestAcceptShadow_FlagOff_LabelsRemainNil`).
- **Skill parity guard.** `assets/assets_test.go` enforces required CLI-command anchors and the recipe-op JSON schema. Adding new content to skills is safe; removing required anchors breaks the guard. The chunk-E rollout used the parity guard as the green-light signal.
- **`/dev/null` is a char device on macOS** — `canPromptForConfirmation` returns true for it. `internal/cli/test_helpers_test.go::openDevNull()` returns an `os.Pipe()` write-end-closed pipe to simulate non-TTY stdin. Reuse it.
- **Amend deps-only mode:** when `--depends-on` / `--remove-depends-on` is set with only the slug arg and no piped stdin, `amend` skips the request.md rewrite path. Don't accidentally re-couple them.
- **`store.Init()` refuses if `.tpatch/` already exists** — the validate-all-on-init style test in chunk C instead asserts that `feature deps --validate-all` runs cleanly post-init. Use the same shape for follow-up tests.
