# Current Handoff

## Active Task

- **Task ID**: M14 correctness pass (3 findings) — fix-pass before M14.4
- **Milestone**: M14 — Feature Dependencies / DAG (Tranche D, v0.6.0)
- **Status**: Complete — all 3 findings landed, ready for review (2026-04-26)
- **Estimated size**: ~190 LOC + 11 tests, no version bump (final: ~520 LOC including doc/comments + 11 tests)

### Three findings (all flag-gated, byte-identical when flag off)

1. **F1 (HIGH, cutover-blocking)**: Wire `created_by` apply-time gate. Today
   `RecipeOperation.CreatedBy` is parsed but inert. Per PRD §4.3 + ADR-011 D4
   it must gate `replace-in-file` / `append-file` against missing targets when
   the named parent is hard. Soft parents emit a warning. Validation error
   when `created_by` names a feature not in `depends_on`. New file
   `internal/workflow/created_by_gate.go` + sentinel `ErrPathCreatedByParent`
   + 7 regression tests.
2. **F2 (MEDIUM)**: `RunReconcile` persists `stale-parent-applied` labels
   computed against the OLD child `AttemptedAt`, then overwrites the
   timestamp with `time.Now()`. Result: child appears stale against itself.
   Fix: thread one shared `attemptedAt` through `saveReconcileArtifacts` →
   `updateFeatureState`, compose labels using it as the staleness baseline.
   2 regression tests.
3. **F3 (MEDIUM)**: `ComposeLabels` keeps emitting parent-derived labels
   for children whose own outcome is `ReconcileUpstreamed`. Per ADR-011 the
   child is being retired; surfacing `waiting-on-parent` is misleading. Fix:
   early return in `ComposeLabels` when `status.Reconcile.Outcome ==
   ReconcileUpstreamed`. 2 regression tests.

### Strict scope guards (DO NOT)

- DO NOT bump version, update CHANGELOG, or tag.
- DO NOT touch skill formats (M14.4 work).
- DO NOT add `tpatch status --dag` (M14.4 work).
- DO NOT add new `ReconcileOutcome` enum values (ADR-011 D3).
- DO NOT consult `artifacts/reconcile-session.json` from any new code path.

### Context

M14.1 ✅ data model + DAG primitives. M14.2 ✅ apply gate (inert until flag on). M14.3 ✅ reconcile topological traversal + composable labels + compound verdict (inert until flag on). All three landed flag-protected — runtime behavior with `features_dependencies: false` is **byte-identical to v0.5.3**.

**M14.4 is the user-facing cutover.** Flipping the flag default to `true`, shipping the `tpatch status --dag` view, rolling label/dep documentation across all 6 skill formats, writing `docs/dependencies.md`, and tagging v0.6.0.

This is the first M14 sub-milestone where end users observe new behavior. Dispatch only after explicit user approval.

### Authoritative docs

1. `docs/adrs/ADR-011-feature-dependencies.md` — D1–D9 (locked)
2. `docs/prds/PRD-feature-dependencies.md` — §3.5 (label matrix), §4.5 (precedence), §5 (UX)
3. `docs/ROADMAP.md` — M14.4 line + Tranche D summary
4. M14.1, M14.2, M14.3 closeout entries in `docs/supervisor/LOG.md`

### Scope (5 chunks)

#### Chunk A — `tpatch status --dag` (~120 LOC)

- New `--dag` flag on `status` command in `internal/cli/cobra.go`.
- Renders the dependency DAG for all features in the project, or a single feature's transitive parent + child set if a slug is given.
- Output: ASCII tree (deterministic by slug) showing each feature with state + reconcile outcome + labels (using `EffectiveOutcome()`).
- Hard deps shown with `─►`, soft deps with `┄►`.
- `--format json` for harness consumption (M9 contract).
- Tests: cycle handling (should never hang — already protected by `DetectCycles`), empty DAG, single-feature subset, label rendering.

#### Chunk B — Flag default flip (~5 LOC + many test fixtures)

- `internal/store/store.go`: change `features_dependencies` default from `false` to `true`.
- This is the moment the new behavior becomes observable. **Audit every test fixture that asserts byte-identity** — some may need updating to include `labels: []` or topo-ordered output.
- Run full suite. Fix every regression.

#### Chunk C — 6-skill rollout (parity-guard coordinated, ~80 LOC of docs)

Update all 6 skill formats with:
- `dependencies` field documentation (analyze-phase bullet)
- Labels reference (`waiting-on-parent`, `blocked-by-parent`, `stale-parent-applied`)
- Compound verdict (`blocked-by-parent-and-needs-resolution`)
- `tpatch status --dag` mention

Files (all 6 in lockstep):
- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch.md`
- `assets/skills/copilot-prompt/tessera-patch.prompt.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/windsurf/tessera-patch.md`
- `assets/skills/generic/tessera-patch.md`

`assets/assets_test.go` parity guard MUST pass after all 6 are updated.

Also: `docs/agent-as-provider.md` — if it covers reconcile-time agent behavior, add labels section.

#### Chunk D — `docs/dependencies.md` (~150 LOC)

User-facing reference doc:
- What dependencies are (hard vs soft)
- How to declare them (YAML examples)
- Validation rules (cycles, dangling, self-ref, etc.)
- Label semantics + matrix (lifted from PRD §3.5)
- Compound verdict explanation
- `--cascade` and force semantics (D7)
- `tpatch status --dag` examples
- Migration note: existing v0.5.x projects keep working unchanged unless they add deps.

#### Chunk E — Release cutover

- Bump `version = "0.6.0"` in `internal/cli/cobra.go`.
- New `## 0.6.0 — 2026-MM-DD — Feature Dependencies (Tranche D)` section in `CHANGELOG.md` summarizing M14.1–M14.4.
- Update `docs/ROADMAP.md`: M14 ✅, Tranche D box closed.
- Tag `v0.6.0` AFTER push, AFTER full validation.

### Strict scope guards (DO NOT do)

- Do NOT skip the parity guard in Chunk C — all 6 skills must move atomically.
- Do NOT add new external Go dependencies.
- Do NOT introduce `ReconcileWaitingOnParent` / `ReconcileBlockedByParent` enum values (still ADR-011 D3).
- Do NOT inject parent patches into the M12 resolver (ADR-011 D8 — deferred to v0.7).
- Do NOT populate `created_by` from the implement phase (separate backlog).
- Do NOT bypass DAG integrity with `--force` (ADR-011 D7 — explicit `--cascade` required).

### Validation gate

```
gofmt -l .
go build ./cmd/tpatch && rm -f tpatch
go test ./...
go test ./assets/...
go test ./internal/cli -run 'StatusDag' -count=1 -v
go test ./internal/workflow -run 'PlanReconcile|ComposeLabels|EffectiveOutcome|AcceptShadow|GoldenReconcile|Phase35|Labels' -count=1 -v
go test ./internal/store -run 'Label|Reconcile|DAG|Dependency|Roundtrip' -count=1 -v
```

All M14.1+M14.2+M14.3 tests stay green. Golden reconcile + manual accept regressions stay green.

### Workflow notes

- `tpatch` binary at root is NOT gitignored. After every `go build` run `rm -f tpatch` BEFORE staging. (Recurring slip — supervisor has tripped 3 times this session.)
- Use `git -c commit.gpgsign=false` for commits. Each carries the trailer.
- `git push` takes 60+ seconds on this machine.
- 5–6 logical commits expected (one per chunk + version bump + CHANGELOG).
- Do NOT tag during the implementer's run. Tagging is the supervisor's final closeout action after reviewer APPROVES.

## Session Summary

M14 correctness pass complete. Three findings landed in three logical
commits, all flag-protected:

  - F1 (cbe2873): `created_by` apply-time gate wired into recipe.go
    (`replace-in-file` / `append-file` only). New sentinel
    `ErrPathCreatedByParent`. Soft deps emit warning + fall through.
    7 regression tests.
  - F2 (071c5ed): one shared `attemptedAt` timestamp threaded through
    `saveReconcileArtifacts` → `updateFeatureState` so persisted
    `Labels` reflect the AttemptedAt about to be written. New
    `composeLabelsAt(s, slug, asOf)` helper; `ComposeLabels` refactored
    to delegate to `composeLabelsFromStatus(s, child)`. 2 regression
    tests.
  - F3 (cc95cbb): early return in `composeLabelsFromStatus` for
    children whose own outcome is in `childRetiredOutcomes`
    (currently only `ReconcileUpstreamed`). 2 regression tests.

Validation gate: `gofmt` clean, `go build ./cmd/tpatch` green,
`go test ./...` green, all targeted regression suites green
(workflow, store, cli, assets parity). M14.1 / M14.2 / M14.3
adversarial tripwires
(`TestComposeLabels_ReadsStatusJsonNotSessionArtifact`,
`TestReconcile_FlagOn_BlockedByParent_SkipsPhase35`) stay green.

## Files Changed (M14 fix-pass)

  - internal/workflow/created_by_gate.go          (new, F1)
  - internal/workflow/created_by_gate_test.go     (new, F1)
  - internal/workflow/recipe.go                   (F1: signatures + gate wiring)
  - internal/cli/cobra.go                         (F1: 2 call sites)
  - internal/cli/phase2.go                        (F1: 1 call site)
  - internal/workflow/reconcile.go                (F2: shared attemptedAt)
  - internal/workflow/labels.go                   (F2 helper extraction + F3 retired-outcomes)
  - internal/workflow/labels_freshness_test.go    (new, F2)
  - internal/workflow/labels_upstreamed_test.go   (new, F3)

## Test Results

  gofmt -l .                                                  → clean
  go build ./cmd/tpatch                                       → ok
  go test ./...                                               → all packages ok
  go test ./internal/workflow -run 'CreatedByGate|ComposeLabels|RunReconcile|GoldenReconcile|Phase35|Labels|AcceptShadow|PlanReconcile|Recipe' → ok
  go test ./internal/store -run 'Label|Reconcile|DAG|Dependency|Roundtrip' → ok
  go test ./internal/cli -run 'DependencyGate|Apply'                       → ok
  go test ./assets/...                                                     → ok

## Next Steps

1. Reviewer dispatched to verify the three commits against the
   PRD §4.3 contract and the regression test set.
2. On APPROVED → archive this handoff, then user may green-light
   M14.4 (status DAG view + skill rollout + v0.6.0 cutover).

## Blockers

None.

## Context for Next Agent

  - All three fixes are flag-gated: with `features_dependencies: false`
    (current default) behaviour is byte-identical to v0.5.3.
  - F1 changes the public signatures of `workflow.DryRunRecipe` and
    `workflow.ExecuteRecipe` from `(repoRoot, recipe)` to `(s, recipe)`.
    Three internal call sites updated; no external consumers.
  - F2 adds an unexported `attemptedAt` field to `ReconcileResult`.
    Unexported, so encoding/json ignores it — no schema impact.
  - F3 currently treats only `ReconcileUpstreamed` as "retired". If a
    future enum value (e.g. `ReconcileObsolete`) lands, add it to
    `childRetiredOutcomes`.
  - Implement-phase heuristic inference of `created_by` is still a
    separate backlog item per PRD §4.3.1 (NOT included here).
