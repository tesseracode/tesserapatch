# Current Handoff

## Status

**Cluster state**: IN PROGRESS

The accepted read-only `tpatch prepare <slug> --check` implementation is
dispatched as one sequential wave. No mutating prepare mode is authorized.

## Active Task

- **Task ID**: `implement-prepare-check`
- **Issue**: [GH #16](https://github.com/tesseracode/tesserapatch/issues/16)
- **Description**: Implement
  `PRD-artifact-validation-and-provenance` rev-5 + ADR-034 rev-2.
- **Status**: In Progress — implementation dispatch
- **Assigned**: 2026-08-17
- **WAVE_BASE**: `9a8c1d049bb973ccf377bd9f0fa67d7080d2d773`
- **Release tag**: none assigned; this prerequisite must be reviewed before any
  mutating prepare release planning

## Contract

- Command: `tpatch prepare <slug> --check`.
- Read only: no provider, prompt, lock, filesystem mutation, state transition,
  status timestamp, `FEATURES.md` refresh, or artifact write.
- Required artifacts: `analysis.md`, `spec.md`, `exploration.md`.
- Optional analysis sidecar never affects readiness.
- Full accepted nine-state artifact classification and three-document readiness.
- Constant `provenance: unknown`.
- One held Go 1.26 `*os.Root`; logical root confinement, unsafe/non-regular
  refusal, bounded `MaxArtifactBytes+1` reads, honest instability reporting.
- Accepted `unix || windows` fail-closed platform policy.
- Exact human/JSON/quiet output, exit codes and precedence from the 208-row
  `AVP-001…AVP-208` matrix.
- Existing per-phase `--manual`, `next`, `cycle`, and `apply --mode prepare`
  behavior stays byte-compatible.

## Scope and Discipline

- One sequential implementer owns the wave because CLI registration, output
  contracts, tests and public docs overlap.
- Explicit-path staging only; no broad add/commit.
- Implementation goldens must come from this wave's commit range, not from the
  later mutating-prepare cluster.
- Native Windows behavior is part of acceptance.
- Mutating prepare PRD rev-14 / ADR-035 rev-14 remains blocked.

## Current State

- Planning contracts are accepted; no check implementation exists at dispatch.
- Research GH #12–#15 is accepted/parked and may not preempt this task.
- Tracked worktree is clean at dispatch; known allowlisted untracked research
  WIP remains untouched.

## Files Changed (dispatch)

- `docs/handoff/CURRENT.md`
- `docs/ROADMAP.md`
- `docs/supervisor/LOG.md`

## Verification

- Fresh `origin/main` baseline recorded before dispatch.
- GH #16 contains the accepted scope and sequencing block.
- No source change yet.

## Next Steps

1. Extract the accepted PRD/ADR contract into implementation slices.
2. Implement rooted inspection and readiness derivation.
3. Wire CLI/output/exit behavior.
4. Add matrix-derived tests, platform builds and public-surface guards.
5. Update SPEC/docs/assets without authorizing mutation.
6. Run full validation and dual review.

## Blockers

- None for the read-only check implementation.
- Hard block for every mutating prepare slice until this wave is accepted and
  landed.

## Context for Next Agent

- Read PRD §3–§6, §10–§18 and ADR-034 D1–D18 before editing.
- Do not reuse ADR-034 as a write precedent.
- The research issues are GH #12 absorption, #13 replay, #14 reorder, #15 recipe
  generation; all stay out of this implementation.
