# Current Handoff

## Status

**Cluster state**: IN PROGRESS

The research-intake/GH #13 planning wave is durably closed at
`d3454edb72c37bacae9159481cc2e6cd69559459`; its acceptance is preserved in
[HISTORY](HISTORY.md). The operator has paused GH #13 to investigate whether
the inherited resource-admission policy unnecessarily prevents older-Mac tests.
Only an instrumented baseline is authorized; a relaxed candidate is undecided.

## Active Task

- **Task ID**: `calibrate-test-resource-admission`
- **Milestone**: Validation infrastructure investigation, before GH #13
- **Description**: Measure baseline resource use before choosing any admission change
- **Status**: In progress — instrumented older-host baseline dispatched
- **Assigned**: 2026-10-08
- **WAVE_BASE**: `d3454edb72c37bacae9159481cc2e6cd69559459`
- **Frozen source baseline**: `d3454edb72c37bacae9159481cc2e6cd69559459`

WAVE_BASE = d3454edb72c37bacae9159481cc2e6cd69559459

## Session Summary

The user asked why 80% is required, what peak resource use was recorded, and
whether a measured comparison can make testing viable on the older Mac.
The user selected an instrumented baseline only, with candidate selection
after measurements, on the separately reachable older machine. The operator
also explicitly authorized a separate checkout under its `tpatch/bin/` and
an isolated Go 1.26.5 download, leaving the existing checkout untouched.

Direct inspection confirms the threshold was an admission policy, not a
measured suite memory requirement. The Makefile has eight correctness/durability
checks; Git/document checks 1-5 are distinct from formatting (6), vet/build (7)
and the complete fresh 22-invocation test partition (8). The shard script
already serializes packages and Go execution with `-p=1` and `GOMAXPROCS=1`.
Observer tests intentionally launch child `go test` processes.

Prior logs sampled before commands, not throughout execution. Historical
admission observations survive in LOG/HISTORY, but no trustworthy peak process-
tree RSS, runtime pressure maximum or runtime process-count maximum was recorded.
The temporary helpers/logs were removed at the prior wave's completed cleanup.

## Current State

- Current host: 32 GiB RAM, ten physical/logical CPUs; Go version file 1.26.5.
- Older host: 16 GiB RAM, four physical/eight logical CPUs, Intel; Go 1.27.1.
- Older-host read-only samples: admission metric 81-84%, zero swap used,
  load1 above 5. This is not test-generated load or a runtime peak measurement.
- Its checkout is at `75d80f0`, with untracked historical research; do not
  overwrite, clean or fast-forward it across those files.
- Need a clean isolated baseline checkout and an explicitly matched toolchain
  before a comparison can isolate resource-policy effects. Setup now complete:
  isolated source is exact `d3454ed`; isolated Go 1.26.5 has its official checksum.
- The profiler runs outside the frozen checkout, preserving its clean source.
  Only formatting, serial vet/build and the exact 22-command partition are
  profiled. Git/document gates 1-5 are not repeated as a new close certification.
- Latest older-host non-Go smoke reports 78% on the admission metric. A bounded
  attempt will retain evidence even if no Go command qualifies.
- Keep >=80% admission, load1 <=5, no competing Go/test tools and a fresh
  continuous 60-second window. No relaxed policy or artificial stress authorized.
- Tests, assertions, selectors, serial settings and timeouts remain unchanged.
- Capture continuous measurements and durable summaries in the new experiment;
  distinguish sampled RSS sums from exact physical footprint/high-water marks.

## Files Changed

- CURRENT, ROADMAP and supervisor LOG: investigation scope and discovered
  limitations.
- `scripts/profile-validation.py` and its non-Go controls: optional profiler
  of stages 6-8, not a new mechanical-close certificate. It retains the existing
  admission conditions, profiles the exact shard command partition, and preserves
  raw evidence, stage logs and summaries. No CLI runtime, Go test-source, CI,
  existing shard-script or admission-policy changes.

## Test Results

Eight non-Go profiler controls pass, with negative admission/telemetry/source-
parity fixtures and owned-child cleanup. Real non-Go telemetry/timing smokes
pass on both Macs. Go validation is now dispatched only through the original
admission gate; no Go success is claimed yet. Prior acceptance remains valid:
local gate 8/8 at `bd13671`, exact-checkpoint native run `37871315522` SUCCESS,
terminal tracking pushed at `d3454ed`. That evidence contains no resource profile.

## Next Steps

1. Await the bounded older-host baseline, retaining raw measurements, stage
   logs, workload identity, cache/toolchain context and its summary.
2. Run only if the existing admission rule qualifies. If it cannot, report
   admission-blocked without inventing test resource requirements.
3. Select any relaxed experiment only after discussing the baseline results.
   GH #13 implementation remains paused and separately authorized.

## Blockers

The older checkout/source and compiler differ from the validated baseline;
the authorized isolated setup avoids changing them. Admission still requires
a qualifying 80%/load/no-Go minute; no lower threshold is authorized.

## Context for Next Agent

- No permanent threshold adjustment, synthetic memory pressure, termination of
  other users' processes, release, tag or #13 runtime implementation is authorized.
- Keep old research and accepted ADR-043/ADR-037/PRD rev-7 unchanged.
- The prior ADR index and frozen ADR-041 sections are guarded snapshots; do not
  edit their historical status wording for this infrastructure investigation.
- Access details are session-local, not repository documentation.
- `-count=1` forces test execution; compiled Go artifacts may still be cached.
  A warm/cold-cache difference must not be attributed to admission thresholds.
- Keep the new `bin/resource-baseline/` evidence and isolated setup. The older
  raw run is `older-mac-20261008`; mirror summary and samples locally at completion.
