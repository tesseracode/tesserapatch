# Current Handoff

## Active Task

- **Task ID**: (none — Tranche D closed at v0.6.0)
- **Milestone**: Awaiting next milestone selection
- **Status**: Idle — awaiting supervisor/user direction
- **Assigned**: 2026-04-26

## Session Summary

**M14 Tranche D shipped as v0.6.0** (tagged 2026-04-26). All sub-milestones APPROVED:

- M14.1 data model + DAG primitives ✅
- M14.2 schema + dep-state apply gate ✅
- M14.3 reconcile topo + composable labels + compound verdict ✅
- M14 correctness pass (F1/F2/F3 from external review) ✅
- M14 C5 fix-pass (reconcile-path label suppression + dry-run downgrade) ✅
- M14.4 user-facing cutover (status --dag, dep-CLI, flag flip, 6-skill rollout, docs, release) ✅

`v0.6.0` tag pushed. `features_dependencies: true` is now the default. `tpatch feature deps add/remove`, `amend --depends-on`, `remove --cascade`, and `tpatch status --dag` are all live. `created_by` is a real apply-time gate (not inert). PRD §4.3 dry-run downgrade in effect.

See `docs/handoff/HISTORY.md` (top entry) for the full M14.4 archive — chunk breakdown, files changed, test results, and operator notes.

## Current State

- HEAD = `v0.6.0` (post-closeout commits push the archive + tag)
- Build clean, full test suite green, parity guard holds
- ROADMAP M14 ✅, Tranche D box closed
- No active in-flight work

## Files Changed

This handoff transition only:
- `docs/handoff/CURRENT.md` (this file — reset for next milestone)
- `docs/handoff/HISTORY.md` (M14.4 archive prepended)
- `docs/ROADMAP.md` (M14.4 ✅, Tranche D closed — already landed in `f2d0d1b`)

## Test Results

Last green validation gate ran during M14.4 (see HISTORY for full output). Reviewer re-ran the gate as part of APPROVED verdict on commit `0779ab5`.

## Next Steps

1. **Supervisor / user**: pick the next milestone from `docs/ROADMAP.md`. Candidates per ROADMAP:
   - M15+ Future items (review the list)
   - Pending follow-ups (separate backlog):
     - `feat-satisfied-by-reachability` (M14.1 deliberate limitation: no `git merge-base` check on `satisfied_by` SHAs)
     - Implement-phase auto-inference of `created_by` per PRD §4.3.1 (separate from M14.4)
     - Address any v0.6.0 field-feedback issues that surface
2. Once a milestone is selected, write the active-task block above and dispatch.

## Blockers

None.

## Context for Next Agent

- **Default flip is now LIVE.** `features_dependencies: true` by default. Existing v0.5.x repos get the new behavior on first run unless they set `features_dependencies: false` in `.tpatch/config.yaml`.
- **`tpatch` binary at the repo root is NOT gitignored.** A bare `tpatch` ignore would shadow `cmd/tpatch/`. After every `go build ./cmd/tpatch` run `rm -f tpatch` BEFORE staging. Recurring slip across the entire M14 series.
- **Commit trailer mandatory**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`. Use `git -c commit.gpgsign=false`.
- **Source-truth guard (ADR-011 D6)**: all DAG/label/status code reads `status.Reconcile.Outcome` via `store.LoadFeatureStatus`, NEVER `artifacts/reconcile-session.json`. M14.3 has an adversarial test pinning this — do not regress.
- **`--force` is NOT a DAG-integrity bypass** (PRD §3.7, ADR-011 D7). Only `--cascade` opts into removing a feature with downstream dependents.
- **Skill parity guard** (`assets/assets_test.go`) enforces required CLI-command anchors and the recipe-op JSON schema. Treat it as a real reviewer.
- **`git push` is slow** (60+ seconds on the typical operator machine). Use long initial_wait when pushing.
