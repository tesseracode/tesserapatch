# Current Handoff

## Active Task

- **Task ID**: Tranche C3 / v0.5.3 — shadow accept accounting fixes (✅ **3/3 items landed on main; release task is supervisor's**)
- **Status**: ✅ Implementation + regression test landed on `origin/main`. Tag + CHANGELOG + version bump deferred to supervisor (per agent guardrails).
- **Blocks**: M14.1 — M14.3 reads `status.Reconcile.Outcome` for ADR-011 D6 label composition. C3 clears the baseline.
- **Previous**: Tranche C2 / v0.5.2 shipped ✅ — archived in `HISTORY.md`

### C3 scope — external reviewer surfaced 3 follow-ups on v0.5.2 shadow flow

All verified by code inspection:

| ID | Severity | Finding | Status |
|---|---|---|---|
| c3-separate-resolution-artifact | 🔴 Silent correctness (manual-accept regression) | Resolver writes `ResolveResult` (with `outcomes[]`) to `artifacts/reconcile-session.json`; reconcile.go:398 `saveReconcileArtifacts` overwrites with `ReconcileResult` (no outcomes); `loadResolvedFiles` reads outcomes → errors "no resolved files recorded". Fix: split into `resolution-session.json` (resolver) + `reconcile-session.json` (reconcile summary) | ✅ `4636878` |
| c3-manual-accept-regression-test | 🟡 Missing coverage | End-to-end shadow-awaiting → manual accept test. Counterpart to `TestGoldenReconcile_ResolveApplyTruthful` but for the manual path. Would have caught both other C3 findings in v0.5.2 | ✅ `8a4af4b` |
| c3-accept-stamps-reconcile-outcome | 🟡 Internal consistency (M14.3 blocker) | `AcceptShadow` marks `State=applied` but leaves `Reconcile.Outcome=shadow-awaiting`. M14.3 label composition (ADR-011 D6) reads `Reconcile.Outcome` — stale outcome → wrong DAG labels | ✅ `3ac7465` |

### Session Summary — 2026-04-24 — C3 fix pass complete

Resumed the partial C3 run (resolver-session split + CLI reader already staged)
and completed the three outstanding deliverables:

- **Split artifact fully landed** (`4636878`): `internal/workflow/resolver.go`
  (`persistSession`), `internal/cli/cobra.go` (`loadResolvedFiles` +
  `shadow-diff`), `resolver_test.go`, and the Notes string in
  `reconcile.go:tryPhase35` all point at `resolution-session.json`. Drift
  audit updated the matching copy in 5 skill/prompt/workflow assets plus
  `docs/agent-as-provider.md` and `docs/prds/PRD-provider-conflict-resolver.md`.
  CHANGELOG, HISTORY, ADR-010, and M12 milestone are left historical.
- **AcceptShadow now stamps Outcome** (`3ac7465`):
  `clearShadowPointerAndStamp` signature extended to `(s, slug, sessionID, phase)`;
  sets `Reconcile.Outcome = ReconcileReapplied` and `Reconcile.AttemptedAt`.
  Auto-apply path unchanged externally (outer `updateFeatureState` still writes
  the same value on top); manual `reconcile --accept` now leaves a truthful
  `Outcome=reapplied` in status.json.
- **Regression test** (`8a4af4b`): `TestGoldenReconcile_ManualAcceptFlow`
  in `internal/workflow/golden_reconcile_test.go` drives
  `RunReconcile(Resolve:true)` → parses `resolution-session.json` inline
  (mirroring `loadResolvedFiles`) → calls `workflow.AcceptShadow` → asserts
  merged content on disk, `State=applied`, `Reconcile.Outcome=reapplied`,
  `ShadowPath` cleared, shadow directory pruned. Guards all three C3 fixes
  together.

### Commits (pushed to `origin/main`)

- `4636878` — fix(workflow): split resolver artifact into resolution-session.json
- `3ac7465` — fix(workflow): AcceptShadow stamps Reconcile.Outcome=reapplied
- `8a4af4b` — test(reconcile): end-to-end shadow-awaiting → manual accept regression

### Test results

```
ok  	github.com/tesseracode/tesserapatch/assets
?   	github.com/tesseracode/tesserapatch/cmd/tpatch		[no test files]
ok  	github.com/tesseracode/tesserapatch/internal/cli
ok  	github.com/tesseracode/tesserapatch/internal/gitutil
ok  	github.com/tesseracode/tesserapatch/internal/provider
ok  	github.com/tesseracode/tesserapatch/internal/safety
ok  	github.com/tesseracode/tesserapatch/internal/store
ok  	github.com/tesseracode/tesserapatch/internal/workflow
```

`gofmt -l .` clean; `go build ./cmd/tpatch` succeeds.

### Files changed (drift audit — resolver context only)

Assets: `assets/skills/copilot/tessera-patch/SKILL.md`,
`assets/skills/cursor/tessera-patch.mdc`,
`assets/skills/windsurf/windsurfrules`,
`assets/workflows/tessera-patch-generic.md`,
`assets/prompts/copilot/tessera-patch-apply.prompt.md`
(Claude SKILL.md was already updated by the prior sub-agent).

Docs: `docs/agent-as-provider.md`,
`docs/prds/PRD-provider-conflict-resolver.md`.

Intentionally left historical: `CHANGELOG.md`, `docs/handoff/HISTORY.md`,
`docs/supervisor/LOG.md`, `docs/adrs/ADR-010-*.md`,
`docs/milestones/M12-*.md`, `docs/milestones/M4-reconciliation.md`
(the latter refers to the classical phase-4 reconcile summary, which
legitimately still writes to `reconcile-session.json`).

### Next Steps

1. **Supervisor**: run the code-review sub-agent on the three C3 commits.
2. **Supervisor**: tag `v0.5.3`, bump version string, and add the
   v0.5.3 heading to `CHANGELOG.md` (implementation agent was explicitly
   instructed not to do any of these three).
3. **Supervisor**: unblock M14.1 once the review verdict lands.

### Artifact naming (locked: Option A)

- `artifacts/resolution-session.json` — resolver-owned, per-file `Outcomes[]`
- `artifacts/reconcile-session.json` — reconcile-owned, high-level `ReconcileResult` (unchanged external contract)

### Deferred behind v0.5.3

- M14.1 Data model + validation (~300 LOC)
- M14.2 Apply gate + `created_by` + 6-skill rollout (~250 LOC)
- M14.3 Reconcile topo + composable labels + compound verdict (~500 LOC)
- M14.4 `status --dag` + skills + release v0.6.0 (~300 LOC)

M14.3 will extend `workflow.AcceptShadow` (with the C3-stamped outcome) for the `blocked-by-parent-and-needs-resolution` compound verdict. C2+C3 correctness baselines are prerequisites.

### Registered follow-ups (not in any tranche yet)

- `feat-ephemeral-mode` — one-shot add-feature with no tracking artifacts; depends on `feat-feature-import` + `feat-delivery-modes`
- `feat-feature-reorder` — flip parent-child in DAG; depends on `feat-feature-dependencies`
- `feat-resolver-dag-context` — parent-patch to M12 resolver
- `feat-feature-autorebase` — auto-rebase child on parent drift
- `feat-amend-dependent-warning` — stale-parent-* labels
- `feat-skills-apply-auto-default` — 6 skills still reference `--mode prepare/execute/done`
- `bug-record-roundtrip-false-positive-markdown` — `--lenient` fallback shipped; live repro pending
- `chore-gitignore-tpatch-binary` — trivial; bundle into next release
