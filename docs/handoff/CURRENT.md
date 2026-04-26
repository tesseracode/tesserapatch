# Current Handoff

## Active Task

- **Task ID**: M14.1 — Feature Dependencies data model + validation
- **Milestone**: M14 — Feature Dependencies / DAG (Tranche D, v0.6.0)
- **Status**: Review (ready for code-review sub-agent, completed 2026-04-24)
- **Assigned**: 2026-04-24

### Session Summary (2026-04-24)

Implemented the M14.1 data-model + validation slice, fully gated behind `features_dependencies` (default false). No user-visible behaviour change. All 5 PRD §3.3 validation rules covered with sentinel errors + tests; DFS cycle detection and Kahn topological order pure functions in `internal/store/dag.go`; round-trip byte-identity verified against a pre-M14 `status.json` fixture.

### Files Changed

- `internal/store/types.go` — added `Dependency` struct, kind constants, `DependsOn []Dependency` (omitempty) on `FeatureStatus`, `FeaturesDependencies bool` config field, `Config.DAGEnabled()` helper.
- `internal/store/dag.go` (new) — `DetectCycles`, `TopologicalOrder` (Kahn, deterministic), `Children`, `ErrCycle` sentinel. Pure, no IO. Doc comments enforce the ADR-010 D5 reminder for downstream readers.
- `internal/store/validation.go` (new) — `ValidateDependencies` + `ValidateAllFeatures`; sentinels `ErrSelfDependency`, `ErrDanglingDependency`, `ErrKindConflict`, `ErrSatisfiedByRequiresUpstream`, `ErrInvalidDependencyKind`.
- `internal/store/store.go` — repo `SaveConfig`/`parseYAMLConfig` now round-trip the flat `features_dependencies:` key.
- `internal/store/global.go` — global `renderGlobalYAML` and `mergeConfig` carry the same key (repo-true OR'd into global).
- `internal/store/dag_test.go` (new) — empty graph, isolated node, self-edge, 2-/3-node cycles, linear acyclic, diamond, deterministic topo (50 iters), Kahn cycle error path, `Children` ordering.
- `internal/store/validation_test.go` (new) — positive + negative cases for all 5 rules, plus `ValidateAllFeatures` surfacing all sentinels at once.
- `internal/store/roundtrip_test.go` (new) — pre-M14 fixture byte-identity, empty `depends_on` omit guard, populated `depends_on` round-trip, `Config.FeaturesDependencies` round-trip.
- `docs/handoff/CURRENT.md` — this update.

### Test Results

- `gofmt -l .` → clean
- `go build ./cmd/tpatch` → ok
- `go test ./...` → all packages pass (store 1.6s, cli 5.1s, workflow 12.2s).
- Targeted: `go test ./internal/store -run 'DAG|Cycle|Topo|Children|Validate|Roundtrip|Config_Features' -count=1 -v` → 30 cases, all PASS.

### Implementation choices (M14.1)

- **Config flag shape**: Option A (flat top-level key `features_dependencies: true|false`). Lower risk; works with existing flat YAML parser (`internal/store/store.go:497`). Nested `features:` block deferred — would force a parser rewrite for no semantic gain.
- **Flag wiring scope**: Flag parses + round-trips. No callers gate on it in M14.1 — apply/reconcile wiring lives in M14.2/M14.3.
- **Doc-comment guard**: `Dependency` and DAG types carry an explicit comment that `status.Reconcile.Outcome` is the authoritative reconcile result; `reconcile-session.json` is audit-only (per ADR-010 D5).

### Context

v0.5.3 shipped (`4636878`, `3ac7465`, `8a4af4b`, `6024942`, tag `v0.5.3`). All correctness baselines needed for M14 now in place:

- `workflow.AcceptShadow` is the single accept helper for shadow → real (v0.5.2) and stamps `Reconcile.Outcome=reapplied` (v0.5.3) — M14.3 label composition will read it.
- Resolver and reconcile have clean artifact ownership: `resolution-session.json` (per-file outcomes) vs `reconcile-session.json` (high-level summary).
- Recipe stale guard catches both HEAD and content drift.
- Index-dirty bug on refresh fixed.

No shipped feature currently exposes `depends_on` — M14.1 adds the data model behind `features.dependencies: true` config flag (default false).

### Authoritative docs (read before coding)

1. `docs/adrs/ADR-011-feature-dependencies.md` — **MUST READ**. Locks 9 decisions.
2. `docs/prds/PRD-feature-dependencies.md` — 736-line PRD (APPROVED WITH NOTES). §3.1 data model, §3.5 composable labels, §4.5 precedence, §6 milestone sizing, §7 acceptance criteria. Note §3.4 residual terminology drift — **always defer to ADR-011 + §4.5** when the two conflict.
3. `docs/ROADMAP.md` M14 section — sub-milestone boundaries.

### M14.1 scope (~300 LOC)

**Code additions**:
- `internal/store/types.go`: `Dependency` struct (`slug`, `kind` = `hard|soft`, optional `satisfied_by` for `upstream_merged`) added to `FeatureStatus` as `depends_on []Dependency`.
- `internal/store/dag.go` (new): DFS cycle detection + Kahn topological traversal over the feature set. Pure functions; no IO.
- `internal/store/validation.go` (new): 5 validation rules per PRD §3.3:
  1. No self-dependency.
  2. No cycles.
  3. No dangling refs (every `slug` must exist in the store).
  4. No kind conflict (same parent declared both hard and soft is rejected).
  5. `satisfied_by` only valid when parent state is `upstream_merged`.
- `internal/store/config.go` (or wherever config lives): `features.dependencies` bool flag, default false. All DAG code paths must no-op when flag is off.
- CLI plumbing: no user-visible commands in M14.1. Just make `add`/`status` round-trip the new field when the flag is on.

**Tests**:
- `dag_test.go`: cycle detection (direct self, 2-node, 3-node), topo order determinism (ties broken by slug), empty graph, single node.
- `validation_test.go`: each of 5 rules with positive and negative cases.
- Round-trip: add a feature with `depends_on`, reload from disk, verify equality.
- Feature-flag off: all new code paths bypassed; `status.json` schema unchanged byte-for-byte for pre-M14.1 fixtures.

**Not in M14.1** (belongs to M14.2+):
- Apply gate enforcement.
- `created_by` recipe op.
- Reconcile topological traversal.
- Composable DAG labels.
- `status --dag` output.
- Any of the 6 skill-format updates.

### Suggested approach

1. Read ADR-011 end to end, then PRD §3 and §4.5.
2. Sketch the `Dependency` struct + `FeatureStatus` additions.
3. Write `dag.go` + tests first (pure, fast iteration).
4. Write `validation.go` + tests.
5. Wire the config flag; ensure zero behavior change when flag is off.
6. Round-trip test from existing `status.json` fixtures to prove backward compat.

### Validation required

- `gofmt -l .` clean
- `go build ./cmd/tpatch`
- `go test ./...`

### Guardrails

- No scope creep into M14.2/.3/.4.
- No changes to the recipe JSON schema (that's M14.2 — gated by the parity guard).
- No new external Go dependencies.
- All commits must carry the `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.

### Deferred behind M14.1

- M14.2 — Apply gate + `created_by` recipe op + 6-skill parity-guard rollout (~250 LOC)
- M14.3 — Reconcile topological traversal + composable labels + compound verdict (~500 LOC)
- M14.4 — `status --dag`, skills analyze-phase bullet, `docs/dependencies.md`, tag v0.6.0 (~300 LOC)

### Registered follow-ups (unchanged from C3)

- `feat-ephemeral-mode` — depends on `feat-feature-import` + `feat-delivery-modes`
- `feat-feature-reorder` — depends on `feat-feature-dependencies` (i.e., M14)
- `feat-resolver-dag-context`, `feat-feature-autorebase`, `feat-amend-dependent-warning`
- `feat-skills-apply-auto-default`, `bug-record-roundtrip-false-positive-markdown`, `chore-gitignore-tpatch-binary`
