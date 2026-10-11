# Current Handoff

## Status

**Cluster state**: APPROVED

Independent review approves the frozen maintenance implementation with no
findings. Final mechanical verification is running at pushed `12617b1`; the durable
closing result remains pending. APPROVED describes the review, not a completed
final gate.
The previous calibration handoff is archived verbatim in [HISTORY](HISTORY.md)
under **2026-10-10 — Validation maintenance and resource calibration**.
GH #13 implementation remains unstarted.

## Active Task

- **Task ID**: `calibrate-test-resource-admission`
- **Milestone**: Validation maintenance, before GH #13
- **Description**: Close the reviewed fixture, analyzer-memory and profiling work
- **Status**: Review approved; final monitored gate and exact-checkpoint CI running
- **Assigned**: 2026-10-08
- **WAVE_BASE**: `d3454edb72c37bacae9159481cc2e6cd69559459`
- **Reviewed endpoint**: `501175b290fd8a3bcf2883e8a14988ab31dac10a`

WAVE_BASE = d3454edb72c37bacae9159481cc2e6cd69559459

## Session Summary

The operator authorized scope freeze, independent review, final verification,
archival and closing push. Read-only reviewer `maintenance-openai-review`
(`17c28f96-b00b-45f8-a7fa-c32a60dfed68`) returned APPROVED with no findings;
the explicitly selected model and runtime metadata both identify `gpt-5.4`.
The verdict is recorded in the supervisor log.

Published checkpoint `12617b1e4aad992b5a684653caa8501bec6beb95` is running
the complete mechanical gate in an isolated full-history clone. Checks 1-6
pass; the serialized vet/build/test pipeline is underway with real telemetry.
Exact-checkpoint native CI is `38114748869`. Preserve this dispatch record
locally but do not cancel that CI with another push before its completion.

The accumulated handoff was archived byte-for-byte, with SHA-256
`f47efb6ccbe5f0283b7d0e840db54466d6690746235909ce8f242aba983fe9e3`.
Earlier failed, blocked and partial experiments remain historical evidence,
not current success claims. Four generated Python bytecode files were verified
against the corresponding script filenames and removed; their creator was not
confirmed. All raw measurement evidence is preserved.

Private final-gate routing controls pass nine non-Go tests for stdout/stderr
fidelity, empty formatter output, format findings, first-failure stopping,
mechanical precondition failure, telemetry/admission failure, unexpected faults,
unchanged test arguments and nested-process admission bypass. This is preparation,
not Go validation or a mechanical gate certificate.

## Current State

The maintenance changes provide a deterministic real artifact mutation/refusal
fixture using an inert capture hook, strengthen its AQ authority proof, bound
fifteen transient analyzer memo maps to four completed independent serial
fixtures, and add explicit opt-in resource measurement policies.
Assertions, immutable baseline/import state, canonical shard selection/order,
timeouts, production redaction/store semantics and installed assets are unchanged.

Matched rev-19/rev-20 profiles retain all eighty original test paths.
Sampled owned RSS falls from 8.07/7.29 GiB to 1.35/1.16 GiB, and post-GC
retained heap from 3106.49/2841.81 MiB to 9.53/11.54 MiB.
The cost is recomputation: observed package time is approximately 2.4-2.5x and
cumulative allocation volume 2.8-2.9x baseline. External Go activity differed,
so exact timing causality is not controlled. This is not a speed improvement
or proof that the entire suite now fits the older Mac.

No further optimization, older-Mac experiment, permanent admission-default
change, release/tag or GH #13 implementation is authorized in this closure.

## Files Changed

Reviewed production/test scope: `internal/cli/prepare_publish.go`,
`prepare_capture_unstable_test.go`, `prepare_s6_contract_test.go`,
`prepare_s7_aq_recovery_test.go`, `prepare_s6_cache_scope_test.go` and
`prepare_s7_ar_guard_test.go` in the same directory.

Reviewed tooling: `scripts/profile-validation.py`,
`scripts/profile-validation-test.py`, `scripts/profile-go-output.py` and
`scripts/profile-test-exec.py`. Public aggregate reports remain under
`docs/state-of-the-art/case-studies/validation-resources-2026-10/`.

This phase changes only CURRENT, HISTORY, ROADMAP and the supervisor log.
Operational final-gate helpers/caches/output are private under ignored
`bin/maintenance-close-20261010/`, outside its clean source clone.

## Test Results

- The complete unchanged 22-command partition passed at `4e6de31` with real
  external Go activity, normal pressure and no swapping.
- Current cache-scoped source passes focused correctness/negative controls,
  rev-19/rev-20 profiles, vet and build; all eighty profile paths match.
- Native CI [38084490647](https://github.com/tesseracode/tesserapatch/actions/runs/38084490647)
  passed all five required jobs at source-equivalent `a4baf17`; release skipped.
- Nine private routing controls pass without invoking real Go.
- Final `make wave-close-check` is running at `12617b1`, with checks 1-6 passed;
  exact-checkpoint native CI `38114748869` is running.
  No earlier partial/partition-only run substitutes for this final gate.

## Next Steps

1. Await the running gate at `12617b1` in
   `bin/maintenance-close-20261010/repo`. Its exact command is
   `make wave-close-check WAVE_BASE=d3454edb72c37bacae9159481cc2e6cd69559459`.
   Shell session: `maintenance-final-gate-20261010`; private evidence:
   `bin/maintenance-close-20261010/evidence/`. Do not start a duplicate run.
2. Preserve the running gate's policy:
   with fresh admission before each top-level formatter/vet/build/test command.
   The operator-approved current-Mac profile requires sixty continuous seconds
   of normal OS pressure, no new swap-outs and load1 <=5; external Go is observed
   rather than vetoed. Own validation is serial. Stop on first failure, warning/
   critical pressure, new swap-outs, missing telemetry or an excessive sampling
   gap. Historical occupied swap alone is not a veto; no extra-swap tolerance
   is enabled. This is approval for this final run, not a permanent default.
3. Reconcile owned process identities after completion, preserve raw evidence,
   and await exact-checkpoint native CI without cancelling it with another push.
4. Record actual results, complete archival/terminal tracking and push closure.
   Verify source invariants and unchanged v0.17.0 tag. GH #13 needs a separate
   bounded implementation assignment after maintenance closes.

## Blockers

None from independent review. Complete final gate/CI results remain unverified.
If either blocks, record the exact outcome before stopping;
do not weaken safeguards or claim acceptance.

## Context for Next Agent

Default profiler admission remains unchanged. The approved pressure-based final
run does not authorize removing load/swap/runtime monitoring, parallelizing our
own Go commands or changing test selectors/timeouts. Use the explicit wave base,
not last-tag defaults or the previous research-wave base.

Preserve `bin/resource-baseline/` and the older host's original checkout,
research, isolated toolchain and evidence. No old-host workload is running.
A prior successful partition required identity-verified manual cleanup of an
experiment-owned git-daemon; do not claim automatic reaping on normal exit.

Do not change the pinned ADR index, ADR-041 normative sections or accepted
ADR-037/ADR-043/PRD rev-7 contracts for transient tracking. The latest release
remains v0.17.0 at `d1d6c3fff6cdbab4b5bb9f6f7054b2ce9b8667e8`, annotated tag
object `e7c748ada40988fe72f47b638cb37d07e24f0ac3`.
