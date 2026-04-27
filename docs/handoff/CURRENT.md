# Current Handoff

## Active Task

- **Task ID**: M14.4 — Status DAG view + skills/docs rollout + v0.6.0 release cutover
- **Milestone**: M14 — Feature Dependencies / DAG (Tranche D, v0.6.0)
- **Status**: Awaiting user approval to dispatch (M14.3 ✅ + correctness pass ✅ APPROVED 2026-04-26)
- **Estimated size**: ~300 LOC + version bump + tag

### Context

M14.1 ✅ data model + DAG primitives. M14.2 ✅ apply gate + `created_by` schema. M14.3 ✅ reconcile topological traversal + composable labels + compound verdict. **M14 correctness pass ✅** addressed three external-reviewer findings before M14.4:

- **F1** (HIGH, was cutover-blocking): `created_by` apply-time gate now wired in `recipe.go` via new `ErrPathCreatedByParent` sentinel. Hard/soft/missing-from-depends_on classification matches PRD §4.3 contract. Closed the M14.2 gap.
- **F2**: Stale-parent-applied label is cleared after a clean reconcile (shared `attemptedAt` threaded through `saveReconcileArtifacts` + `updateFeatureState`).
- **F3**: Children with intrinsic outcome `ReconcileUpstreamed` suppress all parent-derived labels per ADR-011.

All four sub-milestones still flag-protected. With `features_dependencies: false`, runtime behavior is byte-identical to v0.5.3.

**M14.4 is the user-facing cutover.** Flipping the flag default to `true`, shipping `tpatch status --dag`, rolling label/dep documentation across all 6 skill formats, writing `docs/dependencies.md`, tagging v0.6.0.

This is the first M14 sub-milestone where end users observe new behavior. Dispatch only after explicit user approval.

### Authoritative docs

1. `docs/adrs/ADR-011-feature-dependencies.md` — D1–D9 (locked)
2. `docs/prds/PRD-feature-dependencies.md` — §3.5 (label matrix), §4.5 (precedence), §5 (UX)
3. `docs/ROADMAP.md` — M14.4 line + Tranche D summary
4. M14.1, M14.2, M14.3, correctness-pass closeout entries in `docs/supervisor/LOG.md`

### Scope (5 chunks)

#### Chunk A — `tpatch status --dag` (~120 LOC)

- New `--dag` flag on `status` command in `internal/cli/cobra.go`.
- Renders the dependency DAG for all features, or a single feature's transitive parent + child set if a slug is given.
- Output: ASCII tree (deterministic by slug) showing each feature with state + reconcile outcome + labels (using `EffectiveOutcome()`).
- Hard deps shown with `─►`, soft deps with `┄►`.
- `--format json` for harness consumption (M9 contract).
- Tests: cycle handling (must never hang — protected by `DetectCycles`), empty DAG, single-feature subset, label rendering.

#### Chunk B — Flag default flip (~5 LOC + many test fixtures)

- `internal/store/store.go`: change `features_dependencies` default from `false` to `true`.
- This is the moment the new behavior becomes observable. **Audit every test fixture that asserts byte-identity** — some may need updating to include `labels: []` or topo-ordered output.
- Run full suite. Fix every regression.

#### Chunk C — 6-skill rollout (parity-guard coordinated, ~80 LOC of docs)

Update all 6 skill formats with:
- `dependencies` field documentation (analyze-phase bullet)
- Labels reference (`waiting-on-parent`, `blocked-by-parent`, `stale-parent-applied`)
- Compound verdict (`blocked-by-parent-and-needs-resolution`)
- `created_by` recipe field (now a real gate, not inert)
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
- `created_by` apply-time gate behavior (F1 fix)
- `--cascade` and force semantics (D7)
- `tpatch status --dag` examples
- Migration note: existing v0.5.x projects keep working unchanged unless they add deps.

#### Chunk E — Release cutover

- Bump `version = "0.6.0"` in `internal/cli/cobra.go`.
- New `## 0.6.0 — 2026-MM-DD — Feature Dependencies (Tranche D)` section in `CHANGELOG.md` summarizing M14.1–M14.4 + correctness pass.
- Update `docs/ROADMAP.md`: M14 ✅, Tranche D box closed.
- Tag `v0.6.0` AFTER push, AFTER full validation.

### Strict scope guards (DO NOT do)

- Do NOT skip the parity guard in Chunk C — all 6 skills move atomically.
- Do NOT add new external Go dependencies.
- Do NOT introduce `ReconcileWaitingOnParent` / `ReconcileBlockedByParent` enum values (still ADR-011 D3).
- Do NOT inject parent patches into the M12 resolver (ADR-011 D8 — deferred to v0.7).
- Do NOT add implement-phase heuristic inference of `created_by` (PRD §4.3.1 — separate backlog).
- Do NOT bypass DAG integrity with `--force` (ADR-011 D7 — explicit `--cascade` required).

### Validation gate

```
gofmt -l .
go build ./cmd/tpatch && rm -f tpatch
go test ./...
go test ./assets/...
go test ./internal/cli -run 'StatusDag' -count=1 -v
go test ./internal/workflow -run 'CreatedByGate|PlanReconcile|ComposeLabels|EffectiveOutcome|AcceptShadow|GoldenReconcile|Phase35|Labels' -count=1 -v
go test ./internal/store -run 'Label|Reconcile|DAG|Dependency|Roundtrip' -count=1 -v
```

All M14.1+M14.2+M14.3+correctness-pass tests stay green. Golden reconcile + manual accept regressions stay green.

### Workflow notes

- `tpatch` binary at root is NOT gitignored. After every `go build` run `rm -f tpatch` BEFORE staging. (Recurring slip.)
- Use `git -c commit.gpgsign=false` for commits. Each carries the trailer.
- `git push` takes 60+ seconds on this machine.
- 5–6 logical commits expected (one per chunk + version bump + CHANGELOG).
- Do NOT tag during the implementer's run. Tagging is the supervisor's final closeout action after reviewer APPROVES.

## Session Summary

M14.3 closed out. M14 correctness pass closed out (F1/F2/F3 from external reviewer). Ready to dispatch M14.4 once user green-lights the user-facing cutover.

## Files Changed

See `docs/handoff/HISTORY.md` for the M14.3 + correctness-pass entries.

## Test Results

Correctness pass final: gofmt clean, `go test ./...` green, 11 new tests + full regression green.

## Next Steps

1. **Wait for user approval** to dispatch M14.4.
2. On green-light: dispatch `m14-4-implementer` with this scope.
3. After implementer: dispatch `m14-4-reviewer`.
4. On APPROVED: supervisor bumps version (if not already), updates CHANGELOG, ROADMAP, archives this handoff, tags `v0.6.0`, pushes.

## Blockers

None. M14 is ready for cutover as soon as user authorizes.

## Context for Next Agent

- M14.1+M14.2+M14.3+correctness-pass are all flag-protected. Flipping the flag default in Chunk B is the load-bearing change.
- The PRD §3.4 has residual terminology drift treating labels as enum verdicts. Defer to ADR-011 D6 + PRD §4.5.
- External-reviewer guard: any DAG/label code reads `status.Reconcile.Outcome`, NEVER `artifacts/reconcile-session.json`.
- `created_by` is now a live gate (not inert). The implement-phase auto-inference heuristic from PRD §4.3.1 is separate backlog — authors set the field manually or via skill examples.
