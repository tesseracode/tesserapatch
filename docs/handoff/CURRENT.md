# Current Handoff

## Status

**Cluster state**: IDLE

The adjacent-conflict / semantic-replay / feature-absorption / verified-reorder
research wave is **Accepted — 2026-08-15** at rev-3. The complete record is in
[`HISTORY.md`](./HISTORY.md). GH #12–#15 remain open planning backlog; no
implementation or architecture decision was authorized.

The queue head returns to the accepted read-only `prepare --check`
implementation.

## Active Task

- **Task ID**: `implement-prepare-check`
- **Description**: Implement the accepted read-only artifact-validation and
  provenance contract — `PRD-artifact-validation-and-provenance` rev-5 +
  `ADR-034` rev-2 — i.e. `tpatch prepare <slug> --check`.
- **Status**: **Not Started — awaiting dispatch**
- **Issue**: none yet — register one at dispatch
- **WAVE_BASE**: none yet — fetch and record fresh `origin/main` immediately
  before dispatch
- **Blocks**: every mutating `prepare` slice

## Current State

- Mutating-prepare planning remains accepted at PRD rev-14 / ADR-035 rev-14.
- `prepare --check` remains unimplemented and is the hard prerequisite.
- Accepted research conclusions:
  - merge and rebase both conflict for the reproduced adjacent hunk; neither is
    a semantic conflict-avoidance mechanism;
  - current reconcile diagnoses the case safely and provider `--resolve` is an
    existing escape;
  - applicable operation candidates are not replayed by phase 2;
  - safe replay is blocked on authoritative recipe generation (GH #15) and
    all-or-nothing eligibility/evidence gates (GH #13);
  - absorption/compaction (GH #12) and verified reorder (GH #14) remain
    separate research fronts.

## Files Changed (research close)

- `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/`
- `docs/state-of-the-art/case-studies/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/whitepapers/WP-001-feature-slice-gap.md`
- `docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md`
- `docs/ROADMAP.md`
- `docs/supervisor/LOG.md`
- `docs/handoff/HISTORY.md`
- `docs/handoff/CURRENT.md`

## Verification

- Git reproduction: three adjacent cases conflict under merge and rebase; one
  separate append case is clean.
- tpatch reproduction: whole-file autogen; blocked/high-confidence
  edit-overlap; applicable-only phase-2 silence; one-shot candidate success;
  non-idempotency, duplicate-anchor and deleted-target hazards.
- Both rev-3 reviewers approved with no findings.
- No production source, test, asset, SPEC, lifecycle state or schema changed.

## Next Steps

1. Register a GitHub issue for `implement-prepare-check`.
2. Record fresh `origin/main` as `WAVE_BASE` in the dispatch brief and here.
3. Declare a non-overlapping file partition; use sequential implementers for
   any shared file.
4. Implement and review the 208-row `prepare --check` contract.
5. Only after it lands may mutating prepare slices dispatch.

## Blockers

- None for `implement-prepare-check`.
- Hard sequencing block for mutating prepare.
- GH #13 replay is blocked on GH #15 recipe-generation authority.

## Context for Next Agent

- Do not let GH #12–#15 preempt `implement-prepare-check` without a fresh
  supervisor dispatch.
- The conflict study is synthetic and proves only its fixtures.
- Do not add an `absorbed` state without first resolving the
  upstream-versus-landed evidence and retention-overlay question.
