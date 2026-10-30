# Current Handoff

## Status

**Cluster state**: IDLE

GH #13 S2 eligibility is ACCEPTED. S0/S1 and memory-bounded validation remain
accepted. No local validation/watch process remains active. S3 and deferred
upstream Git-strategy research are not assigned. No release/tag was created.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s2`
- **Milestone**: GH #13 / ADR-037 and ADR-043
- **Description**: Ordered eligibility over paired capture evidence, offline
  reconstruction and a default-off repository config gate
- **Status**: Complete — idle pending explicit next assignment
- **Assigned**: 2026-10-29
- **Accepted validation checkpoint**: `8d3b9ef196b597695595076e4a55f086836b8b04`
- **WAVE_BASE**: `bd95a237cbf5b5894f21c19c27bb1d628808db8f`
- **Contract**: ADR-037/PRD rev-7, ADR-043 and ADR-044

## Session Summary

S2 adds the default-off `operation_candidate_enabled` repository setting and
an unexported eligibility evaluator. Applied/blocked state is required before
the fifteen ordered gates. Existing strict decoders and canonical E identity
are reused; presence, raw hashes, offline reference facts, projections,
witnesses, captured parent exclusions and metadata conditions are recomputed
with deterministic first-failure reasons.

S1 ReferenceRoot gains read-only checks/reconstruction over local commits.
Affected upstream object availability is verified without retaining upstream
bodies or imposing a new body-size limit. Existing producer read environments
remain unchanged; candidate reads use the hardened envelope, including
case-insensitive routing/config scrubbing.

The evaluator is not wired into reconcile and is not an all-present
confirmation proof. It retains no reconstructed bodies or root handles in its
result. No alignment, candidate execution, lifecycle/schema publication,
public CLI behavior or release work was added.

The full chronology is archived in `docs/handoff/HISTORY.md` under the
2026-10-29 S2 acceptance entry.

## Current State

- Production `2c462a3`, exact inventory correction `b189770` and exact
  Config-schema extension `a5af08e` are independently APPROVED.
- The default-off flag is omitted when false; opt-in is repository-local.
  Gate-off config bytes and the legacy reconcile pipeline remain unchanged.
- The result's `[15]bool` records eligibility only. Do not treat it as the
  later all-present proof or execute persisted recipe bodies cross-base.
- S1 still permits only two bare-root constructors. Candidate create/prune
  borrow caller-held feature/global capabilities; no lock reacquisition,
  release, transfer or third constructor was introduced.
- Readonly coverage-consumer registration is separate from the pure core.
  The strict normalizer's relocated owner is registered with its unchanged
  eleven-call total. S0 capture and S1 scope permissions remain strict.
- PIB-477 admits exactly the new bool/omitempty Config field. Removing it
  reproduces the prior schema digest; other schema changes are not exempt.

## Files Changed

S2 evaluator/runtime/source guards in `internal/workflow/`; read-only reference
methods/tests in `internal/candidate/`; shared bounded reconstruction plumbing
in `internal/patchobs/`; case-insensitive hardened Git filtering in
`internal/gitutil/`; repository config and roundtrip tests in `internal/store/`.
Dependent candidate/parser/coverage/schema inventories were transitioned
explicitly with negative controls. Closure changes only tracking.

## Test Results

- **Exact checkpoint**: `8d3b9ef196b597695595076e4a55f086836b8b04`.
- **Local gate**: PASS, 8/8 checks, 42/42 top-level commands, all 39 uncached
  test commands and exact parity. Formatting, vet and build pass.
- **Native CI**: [41000838573](https://github.com/tesseracode/tesserapatch/actions/runs/41000838573)
  SUCCESS, all five required jobs with no failed steps; release skipped.
  Actual Windows CLW-001 through CLW-005 PASS lines verified.
- **Focused coverage**: eighteen eligibility refusal codes, fixed gate and
  sub-check ordering, all disallowed lifecycle states, strict malformed/
  unreadable inputs, canonical E identity, no-capture, parent exclusions,
  no live writes, offline/missing-object and environment regressions.
- **Full run**: 6,086.41s (about 101 minutes), 6.89 GiB peak sampled owned RSS,
  normal pressure and zero new swap-outs. Historical swap occupancy remains.
- **Cleanup**: 578 sampled identities reconciled; one owned git-daemon needed
  identity-verified SIGTERM; final owned survivors zero. Tested source clean.
- **Evidence**: `bin/roc-s2-20261029/full-gate-retry1/evidence/summary.json`,
  adjacent logs, native Windows log and focused/negative stages.
  Preserve the failed initial CI and interrupted first gate; its interruption
  sidecar explains why its original summary did not reach normal finalization.

## Next Steps

1. Remain idle pending an explicit next assignment. S3 is not dispatched.
2. Before another slice, read the accepted contracts, scope CURRENT, fetch
   and record a new wave base, and deliberately transition finite inventories.
3. Keep the 39-command partition and resource safeguards. Do not weaken
   source/schema guards merely to accommodate new files or fields.

## Validation Policy — GH #13

Require 60 continuous seconds of normal OS pressure, no new swap-outs and
load1 <=5 before every Go validation command. External Go is observed, not
vetoed; own work is serial (`GOMAXPROCS=1`, `-p=1`). Percentage is diagnostic;
historical occupied swap alone is not a veto. Stop on warning/critical
pressure, new swap-outs, unreliable telemetry or gaps over 5s. Retain 600s
admission budgets, `-count=1`, 40-minute test-process timeouts, clean isolated
full-gate source/caches, exact CI parity and owned-process reconciliation.

## Blockers

None for completed S2. No subsequent slice is assigned.

## Context for Next Agent

S0 goldens, assets, public SPEC, CHANGELOG, dependencies, allowlist, accepted
ADR/PRD bytes and reconcile/CLI wiring are unchanged from S2 wave base.
Do not add an Unreleased heading or casually edit the frozen ADR index.

v0.17.0 remains tag object `e7c748ada40988fe72f47b638cb37d07e24f0ac3`,
targeting `d1d6c3fff6cdbab4b5bb9f6f7054b2ce9b8667e8`.
All earlier raw experiments remain preserved under ignored `bin/`.

Research `research-upstream-git-strategies` remains deferred. No branching
policy, full research deliverable, Git replacement or central patch-store
backend was adopted.
