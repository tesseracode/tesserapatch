# Current Handoff

## Status

**Cluster state**: ACCEPTED

S0 and its authorized six-loop test-memory correction are accepted. Independent
review is approved, the complete local gate passes 8/8 and exact-checkpoint
native CI passes all five required jobs. S1 is not dispatched.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s0`
- **Milestone**: GH #13 / ADR-037 and ADR-043
- **Description**: Frozen S0 baseline plus explicitly authorized test-only memory unblock
- **Status**: Complete — independent review, full local gate and native CI accepted
- **Assigned**: 2026-10-11
- **Maintenance authorized**: 2026-10-21
- **Completed**: 2026-10-22
- **WAVE_BASE**: `bbb7a4310b2ebea095890a284997fbbb42535bc7`
- **Pre-maintenance checkpoint**: `89e481c0ee882c1c9668f8b8d2296a48acaa4eed`
- **Reviewed/measured fix**: `4e56e8559ec305f40ad18690eec8353412c546d2`
- **Full-validation checkpoint**: `bdfe3b1b6a1383dcef06dbe3159089ac6c9801c0`
- **Contract**: accepted ADR-037/ADR-043 and PRD rev-7, PRD §8 S0

WAVE_BASE = bbb7a4310b2ebea095890a284997fbbb42535bc7

## Session Summary

S0's thirteen runtime goldens, CLI/summary contracts, both candidate expectation
fixtures and mutation controls pass. Initial independent review findings were
corrected at `cb96eb5`; the same OpenAI reviewer approved that checkpoint.
Native CI passed at `e8a154b`, but two complete local attempts stopped on warning
pressure in the fifth test command, AR rev-21/23/24, after four commands passed.
The retry after approved idle Docker shutdown also stopped. No assertion
failure or swapping was reported; all owned cleanup was verified.

The separately approved isolated probe reproduced the stop without preceding
suite commands: five roots/83 of 85 leaves passed, peak sampled RSS 10.08 GiB,
no package PASS. Static inspection found unscoped transient analyzer caches
across six roots/85 fixtures. All failed attempts remain archived and private.

The operator then authorized the bounded test-only correction. Exactly twelve
lines register the existing four-completed-fixture memo scope in those six
roots and their local validators. Models are local to each synchronous fixture;
no active model crosses a cleanup boundary. The unchanged helper clears fifteen
coupled transient maps, preserving immutable/import/partial-type state.

The S0 scope guard does NOT broadly allowlist the old analyzer test file.
It accepts only its exact frozen bytes plus those twelve insertions. Negative
controls reject missing/extra registrations, parallelism, assertion changes,
unrelated edits and malformed baselines. Original goldens and production are
unchanged. Independent reviewer `e17d29e2-bd48-4003-80a9-c0b61112dc09`,
explicitly selected/runtime-confirmed OpenAI `gpt-5.4`, returns APPROVED for
`89e481c..4e56e85`, with no findings.

The complete gate passes 8/8 at pushed `bdfe3b1`, with fresh source/caches,
capture disabled and the original S0 wave base. All 25 top-level commands/
22 test commands pass with exact parity. Native CI `39728657049` is SUCCESS
at that exact SHA, all five required jobs passed and release skipped.
Subsequent closure changes are tracking-only; validated source is unchanged.

## Current State

At frozen `4e56e85`, all six roots and all 85 original leaves pass with exact
91-path RUN/PASS inventory parity. Peak sampled owned RSS is 1,439,350,784 bytes
(about 1.34 GiB), versus 10.08 GiB observed before the old probe was interrupted.
No completed matched pre-fix heap profile exists; do not claim precise per-cache
attribution or a controlled performance ratio.

The package takes 317.009s and fresh-cache command 347.08s. Less retained
memoization costs recomputation; earlier passing unscoped commands were about
125-129s under different conditions. This is a RAM/runtime tradeoff, not a
speed improvement. All 201 samples show normal pressure and no new swap-outs.
Historical occupied swap is unchanged at 3,323,063,173 bytes; it is NOT zero.
The frozen source stays clean and zero sampled owned processes remain.

The full gate takes 4009.71s with all 2009 samples at normal pressure and no
new swap-outs. Historical occupied swap decreases from 3,289,508,741 to
3,264,342,917 bytes. The corrected group uses about 1.32 GiB sampled peak
RSS inside the full run (317.64s), while the entire suite still peaks around
10.37 GiB in the first broad command. This is a bounded group-memory fix,
not a claim that every test now has a small footprint. All 380 sampled owned
process identities are reconciled; none remain and no extra termination is needed.

## Files Changed

S0 tests/fixtures remain as reviewed under `internal/workflow`, `internal/cli`,
`internal/store` and `internal/workflow/testdata/roc-s0/`.
The maintenance delta is confined to:

- `internal/cli/prepare_s7_ar_guard_test.go`: twelve scope-registration lines.
- `internal/workflow/operation_candidate_s0_guards_test.go`: exact-delta
  maintenance proof and sensitivity controls, not a general test-file exemption.
- `internal/workflow/testdata/roc-s0/README.md`: explicit exception documentation.
- CURRENT, HISTORY, ROADMAP and the supervisor log.

Production Go, all 85 original assertions, cache-helper implementation, analyzer
logic, S0 goldens, SPEC/assets, accepted contracts, dependencies, CHANGELOG,
canonical CI/shards, test timeouts and pressure safeguards are unchanged.

## Test Results

- Formatting and S0/existing cache controls pass:
  workflow 3.783s, CLI 52.089s, store 0.161s.
- Exact monitored AR rev-21/23/24 group: all 91 paths and package PASS;
  317.009s package, 347.08s command, 1.34 GiB sampled peak, no new swap-outs.
- Updated-source vet and CLI build pass.
- Independent follow-up review: APPROVED, no findings.
- Prior S0 native CI `38189486586` and pre-maintenance latest-tip
  `38218488185` passed. They do not substitute for fixed-source CI.
- Full local gate at `bdfe3b1`: **8/8**, no warnings; all 25 top-level
  commands / 22 test commands and all nine routing controls pass.
- All 25 admission/runtime windows replay successfully; no new swap-outs
  or pressure warnings, no owned validation processes remain.
- Exact-checkpoint [native CI 39728657049](https://github.com/tesseracode/tesserapatch/actions/runs/39728657049):
  **SUCCESS**, all five required jobs pass, release skipped. The historical
  allowed-failure Windows full-suite step remains owned by GH #17.

Measurement evidence: `bin/roc-memo-scope-20261021/measurement/evidence/`.
Final gate evidence: `bin/roc-memo-scope-20261021/final-gate/evidence/`.
Hashes and prior failed outcomes are recorded in LOG and the archived handoff.

## Next Steps

No new task is dispatched. S1 requires a separate bounded assignment against
the accepted ADR-037/ADR-043/PRD rev-7 contract. Preserve these S0 baselines
and the adopted resource policy when planning that slice.

## Validation Policy — GH #13 Local Work

On the current 32 GiB Mac, require 60 continuous seconds of normal OS pressure,
no new swap-outs and load1 <=5 before every Go validation command. External Go
is observed rather than vetoed; our own work remains serial (`GOMAXPROCS=1`,
`-p=1`). Percentage is diagnostic. Historical occupied swap alone is not a
veto; no additional-swap tolerance is enabled.

Stop owned work on warning/critical pressure, new swap-outs, invalid/missing
telemetry or a sampling gap over 5s. Use 600s admission waits and stop at first
failure. Preserve the original 22-command partition and 40m package timeouts.
No old-host admission claim or profiler-default/hosted-CI change is implied.

## Blockers

None for S0 or the authorized maintenance fix. No local validation workload
remains running.

## Context for Next Agent

The full pre-maintenance chronological handoff is archived in HISTORY under
2026-10-21. Preserve `bin/roc-s0-20261011/`, all earlier maintenance/older-host
evidence and new `bin/roc-memo-scope-20261021/`. The first isolated probe's
timestamp parser was repaired by reanalyzing retained data, not rerunning tests.
Do not overwrite historical resource stops or count partial passes as complete.

No production replay feature, release/tag or S1 work is authorized. The
versioned changelog must remain unchanged; its parity guard rejects an added
Unreleased heading. v0.17.0's annotated tag remains
`e7c748ada40988fe72f47b638cb37d07e24f0ac3`. Stage explicit file paths only and
retain the required Copilot co-author trailer.
