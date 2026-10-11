# Current Handoff

## Status

**Cluster state**: ACCEPTED

Validation maintenance is accepted: independent review has no findings, the
complete mechanical gate passes 8/8, and exact-checkpoint native CI passes
all five required jobs. The calibration history and terminal results are
archived in [HISTORY](HISTORY.md). No next implementation task is dispatched.

## Active Task

- **Task ID**: `calibrate-test-resource-admission`
- **Milestone**: Validation maintenance, before GH #13
- **Description**: Deterministic fixture, bounded analyzer memo lifetime and measured resource policy
- **Status**: Complete — review, final gate and native CI accepted
- **Assigned**: 2026-10-08
- **Completed**: 2026-10-11
- **WAVE_BASE**: `d3454edb72c37bacae9159481cc2e6cd69559459`
- **Reviewed endpoint**: `501175b290fd8a3bcf2883e8a14988ab31dac10a`
- **Validated checkpoint**: `12617b1e4aad992b5a684653caa8501bec6beb95`
- **Archival acceptance checkpoint**: `4910022784c11944528f50238df60c8b2d9cba09`

WAVE_BASE = d3454edb72c37bacae9159481cc2e6cd69559459

## Session Summary

The operator-authorized freeze, independent review, full local gate, native CI
and archival are complete. Reviewer `maintenance-openai-review` returned
APPROVED with no findings; explicitly selected and runtime-confirmed model:
`gpt-5.4`. The verified implementation range is
`d3454edb72c37bacae9159481cc2e6cd69559459..12617b1e4aad992b5a684653caa8501bec6beb95`.
Subsequent closing changes contain only tracking and the final measurement
report, with no reviewed-source or protected-contract drift.
The terminal accepted maintenance range is
`d3454edb72c37bacae9159481cc2e6cd69559459..4910022784c11944528f50238df60c8b2d9cba09`;
this final tracking follow-through records that known range for publication.

The complete local gate runs for 3960.94 seconds with all 25 expected top-level
commands passing, including all 22 unchanged test commands. All 1987 samples
show normal pressure, no new swap-outs and zero occupied swap. Diagnostic
percentage readings span 48-74%; sampled peak owned RSS is about 9.80 GiB,
in the unchanged purge-progress guard. External Go is genuinely concurrent:
322 execution samples contain it, with up to five tools observed.
All 414 sampled owned process identities are reconciled; none remain and
no additional termination is needed.

## Current State

The deterministic artifact fixture uses a real capture-boundary mutation and
real refusal/no-write assertions, with an inert production hook and AQ proof.
Four-fixture scopes release fifteen transient analyzer memo maps only at
independent serial fixture boundaries, preserving immutable baseline/import
state and identity within active scopes. No assertion, canonical shard,
timeout, production redaction/store behavior or installed asset was weakened.

Matched rev-19/rev-20 profiles preserve all eighty original test paths and
reduce RSS from 8.07/7.29 GiB to 1.35/1.16 GiB. Post-GC retained heap falls
from 3106.49/2841.81 MiB to 9.53/11.54 MiB. The tradeoff is recomputation:
observed runtime is 2.4-2.5x and allocation volume 2.8-2.9x baseline.
Background activity differed, so exact timing causality is not controlled.
This is not a speed improvement or proof that the entire suite fits the older Mac.

## Files Changed

Reviewed CLI/test scope: `prepare_publish.go`, `prepare_capture_unstable_test.go`,
`prepare_s6_contract_test.go`, `prepare_s7_aq_recovery_test.go`,
`prepare_s6_cache_scope_test.go` and `prepare_s7_ar_guard_test.go`, all in
`internal/cli/`. Reviewed profiling tools are the four `scripts/profile-*.py`
files. The closing record changes only CURRENT, HISTORY, ROADMAP, the supervisor
log and `maintenance-final-gate.json` in the resource case-study directory.

## Test Results

- Independent review: APPROVED, no findings.
- Exact local checkpoint `12617b1`: mechanical gate 8/8, no warnings;
  formatting, vet, build and all 22 original test commands pass.
- All 25 command shapes match the expected sequence; every admission/runtime
  safety window is replayed successfully from raw telemetry.
- Exact-checkpoint [native CI 38114748869](https://github.com/tesseracode/tesserapatch/actions/runs/38114748869):
  SUCCESS, all five required jobs pass, release skipped.
- Nine private routing controls pass without running real Go; prior focused
  negative/cache controls and eighty-path matched profiles remain preserved.

Final non-identifying report:
`docs/state-of-the-art/case-studies/validation-resources-2026-10/maintenance-final-gate.json`.
Raw final evidence: `bin/maintenance-close-20261010/evidence/`.

## Next Steps

No work is dispatched. GH #13 requires a separate bounded implementation
assignment against its accepted ADR-037/ADR-043/PRD rev-7 contracts.
No additional optimization, old-Mac trial, permanent resource-policy adoption
or release/tag is part of this completed maintenance wave.

## Blockers

None. No validation workload remains running.

## Context for Next Agent

The final run was explicitly authorized to require sixty continuous seconds of
normal OS pressure, no new swap-outs and load1 <=5 before each command, while
allowing external Go and serializing our own work. Runtime warning/critical
pressure, new swap-outs, invalid telemetry or sampling gaps stopped owned work.
Load is an admission condition, not a runtime cap. Historical occupied swap
alone was not a veto; no additional-swap tolerance was enabled.
Default profiler admission remains unchanged; do not infer permanent adoption.

Preserve all private raw evidence under `bin/resource-baseline/` and
`bin/maintenance-close-20261010/`, and the older host's original checkout,
research, isolated toolchain and evidence. Its earlier partial/swap-stop results
remain non-passing. A prior run needed identity-verified manual git-daemon
cleanup; the final gate did not.

Do not churn the pinned ADR index, ADR-041 normative sections or accepted #13
contracts for transient tracking. No feature release was created: v0.17.0
remains at `d1d6c3fff6cdbab4b5bb9f6f7054b2ce9b8667e8`, annotated tag object
`e7c748ada40988fe72f47b638cb37d07e24f0ac3`.
