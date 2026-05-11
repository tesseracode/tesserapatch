# Current Handoff

## Active Task

- **Task ID**: v0.7-cluster-routing-pass (paper-only)
- **Milestone**: pre-M17 routing
- **Status**: Not Started
- **Assigned**: 2026-05-10

## Just Shipped

**v0.6.4** — M16 Slice 3 (apply-default-auto + 6-surface skill alignment + parity-anchor strengthening). 4-commit stack landed (`eab2c3c` + `4556387` + `38d13fc` + `477ccc9`), tag pushed, ROADMAP M16 flipped ✅, archived to HISTORY.md.

## Background

After v0.6.4, the next planned work is **v0.7.0**. There are two competing scopes:

1. **`feat-amend-dependent-warning`** — supervisor's earlier plan (continuation of M15 W3 freshness work).
2. **v0.7 boundary-capture cluster** — multi-agent paper-design from 2026-05-10 (4 PRDs, accepted in LOG.md):
   - PRD-record-auto-base (Wave A)
   - PRD-reconcile-lock-guard (Wave A, bundles HIGH bug fix at `internal/workflow/reconcile.go:600-604`)
   - PRD-record-collision-detection (Wave B, depends on auto-base)
   - PRD-tpatch-land (Wave C, gated on auto-base + collision-detection)
   - Plus PRD-patch-already-upstream-detector (sibling exploratory, placement TBD)

User's stated preference: **`feat-amend-dependent-warning` before the boundary-capture cluster**. Final ordering still requires explicit confirmation after this routing pass.

## This Task — Paper-Only Routing Pass

Per the suggesting agent's brief (logged in `~/.copilot/session-state/.../checkpoints/027` and earlier turns), the v0.7 cluster needs paper routing **before** any code lands. **No implementation. No ADR bodies. No PRD edits.**

### Acceptance Criteria

1. **Open 4 ADR placeholders** in `docs/adrs/` (next numbers: ADR-016, 017, 018, 019). Each placeholder contains ONLY:
   - Title
   - `Status: Draft (placeholder)`
   - `Owner: TBD (assign during implementation)`
   - `Locks in: <one-paragraph from supervisor entry>`
   - `Source PRD: <relative path>`
   - `Sections to write at implementation time:` Decision drivers, Decision, Consequences, Alternatives considered.

   Suggested mapping (verify against supervisor LOG entry before locking):
   - ADR-016 — record auto-base resolution algorithm (PRD-record-auto-base)
   - ADR-017 — reconcile lock-guard semantics + writer-normalization (PRD-reconcile-lock-guard)
   - ADR-018 — record collision-detection signature scheme (PRD-record-collision-detection)
   - ADR-019 — tpatch-land trailer-block schema (PRD-tpatch-land)

2. **Slug M17** in `docs/ROADMAP.md` covering the v0.7 cluster with Wave A/B/C slice rows. Reference the supervisor acceptance entry by date. **Do not assign owners** — that's a pending supervisor decision.

3. **Append `## v0.7 Cluster — Queued` section to CURRENT.md** AFTER this routing pass closes (i.e. in the next task's CURRENT.md), surfacing:
   - 3 pending supervisor decisions (PRD-detector placement; implementation owner assignment; AGENTS.md claims-audit-table convention).
   - The queued task IDs (already in SQL — see `todos` table where `id LIKE 'impl-%' OR id LIKE 'adr-%'`).

4. **Append routing entry to `docs/supervisor/LOG.md`** titled `## Queue — v0.7 Cluster Routing — 2026-05-10` documenting: ADR numbers opened, milestone slugged (M17), tasks queued, supervisor decisions pending. **This is a routing entry, not a review verdict.**

5. **Verify HIGH bug still present** at `internal/workflow/reconcile.go:600-604` (`branch: %s` interpolated with full ref). If independently fixed, note drift in the routing entry. (Per PRD-reconcile-lock-guard §5.3, this fix bundles into Wave A guard implementation — DO NOT fix standalone.)

### Constraints

- No edits to `internal/`, `assets/`, or `cmd/`.
- No edits to the 4 cluster PRDs, WP-001, exploratory PRDs.
- No ADR bodies (placeholders only).
- No PRD movement (PRD-patch-already-upstream-detector stays where it is).
- No owner assignments.
- No `go test` / `go build` runs needed (paper-only).

## SQL Todos

The 14 v0.7 cluster todos are already queued (`impl-record-auto-base`, `impl-reconcile-lock-guard`, `impl-record-collision-detection`, `impl-tpatch-land`, plus 4 `adr-*` and 3 `decision-*` and the routing pass itself, with full Wave A/B/C dependency graph in `todo_deps`).

After this routing pass: bring `v0.7-cluster-decide-vs-amend-dependent` to the user for the ordering call.

## Files to Touch

- `docs/adrs/ADR-016-*.md` (create)
- `docs/adrs/ADR-017-*.md` (create)
- `docs/adrs/ADR-018-*.md` (create)
- `docs/adrs/ADR-019-*.md` (create)
- `docs/ROADMAP.md` (add M17 row)
- `docs/supervisor/LOG.md` (prepend routing entry)
- `docs/handoff/CURRENT.md` (rewrite once routing closes — handoff for whichever v0.7.0 scope is chosen next)

## Blockers

None.
