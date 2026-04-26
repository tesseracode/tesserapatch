# Current Handoff

## Active Task

- **Task ID**: M14.4 — Status DAG view + skills/docs rollout + v0.6.0 release cutover
- **Milestone**: M14 — Feature Dependencies / DAG (Tranche D, v0.6.0)
- **Status**: Awaiting user approval to dispatch (M14.3 ✅ APPROVED 2026-04-26)
- **Estimated size**: ~300 LOC + version bump + tag

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

M14.3 closed out. Ready to dispatch M14.4 once user green-lights the user-facing cutover.

## Files Changed (M14.3 — closed)

See `docs/handoff/HISTORY.md` for the full M14.3 entry.

## Test Results

M14.3 final: gofmt clean, `go test ./...` green, 24 new tests + full regression green.

## Next Steps

1. **Wait for user approval** to dispatch M14.4.
2. On green-light: dispatch `m14-4-implementer` with this scope.
3. After implementer: dispatch `m14-4-reviewer`.
4. On APPROVED: supervisor bumps version (if not already), updates CHANGELOG, ROADMAP, archives this handoff, tags `v0.6.0`, pushes.

## Blockers

None. M14.4 is ready to start as soon as the user authorizes the user-facing cutover.

## Context for Next Agent

- M14.1+M14.2+M14.3 are all flag-protected. Flipping the flag default in Chunk B is the load-bearing change.
- The PRD §3.4 has residual terminology drift treating labels as enum verdicts. Defer to ADR-011 D6 + PRD §4.5.
- External-reviewer guard remains: any DAG/label code reads `status.Reconcile.Outcome`, NEVER `artifacts/reconcile-session.json`.
- `created_by` is persisted but inert. Population from the implement phase is a separate backlog item, NOT part of M14.4.
