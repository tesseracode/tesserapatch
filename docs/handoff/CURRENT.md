# Current Handoff

## Status

**Cluster state**: IN PROGRESS

The research-intake/GH #13 planning wave is durably closed at
`d3454edb72c37bacae9159481cc2e6cd69559459`; its acceptance is preserved in
[HISTORY](HISTORY.md). The operator has paused GH #13 to investigate whether
the inherited resource-admission policy unnecessarily prevents older-Mac tests.
The operator now authorizes a bounded 75% pilot with CPU/memory process
attribution. No permanent threshold change or GH #13 implementation is authorized.

## Active Task

- **Task ID**: `calibrate-test-resource-admission`
- **Milestone**: Validation infrastructure investigation, before GH #13
- **Description**: Measure a bounded 75% pilot and attribute older-host CPU/memory use
- **Status**: Blocked — AP observer timed out; pilot result recorded, follow-up decision pending
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
After the 80% baseline was admission-blocked, the operator explicitly approved
a 75% experiment and asked what startup/background processes consume resources.
Use a 30-minute pilot budget, not an unrestricted full-suite attempt.

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
  attempt retained evidence but no command qualified: the 600-second run ended
  with exit 75 before formatting.
- Its 288 admission samples span 598.640 seconds: free metric 76-80%, load1
  2.60-17.39, OS pressure normal throughout, zero swap and zero Go processes.
  Total system process counts were 670-694; these are not test process counts.
- No sample met all the original conditions. Replaying a 75% or 70% memory
  threshold against the same trace yields a 189.186-second qualifying window
  with the other conditions unchanged. This is not an executed relaxed run
  or proof of runtime headroom. The operator has now selected the 75% pilot.
- Pilot admission is >=75%, load1 <=5, no competing Go/test tools and a fresh
  continuous 60-second window. The tool's default remains 80%; no permanent
  policy change or artificial stress is authorized.
- Bound the pilot to 30 minutes plus cleanup: formatting, vet, build, existing
  AR rev-11..15 shard and AP observer shard. A partial/budget-limited run is not
  a full-suite pass. Preserve all selected test commands and timeouts.
- Add top RSS and interval-CPU consumers with parent/start-time information.
  Keep inventories local, with no argv/environment/full executable path capture.
- The 75% pilot completed with exit 1: formatting/vet/build and AR rev-11..15
  passed; AP observer timed out with incomplete PIB-464 and missing later markers.
  No automatic retry is running. The 30-minute experiment budget was not reached.
- All 510 samples showed normal pressure and zero swap. Load1 was 1.97-3.83;
  admission occurred at 78-79%, execution at 77-79%. Largest sampled owned RSS
  sum was about 1.72 GiB, with seven owned processes/six Go tools at peak.
  No external Go competitor was observed; no Go process remained afterward.
- Background process attribution is retained privately. Sustained post-boot
  work is observable, but neither a leak nor the timeout's cause is established.
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
  existing shard-script or permanent admission-policy changes.
- `docs/state-of-the-art/case-studies/validation-resources-2026-10/older-mac-admission.json`:
  durable aggregate, raw evidence digests, hypothetical-window results and limits.
- `docs/state-of-the-art/case-studies/validation-resources-2026-10/older-mac-75-pilot.json`:
  non-identifying pilot measurements and the failing observer diagnostic.

## Test Results

Eight non-Go profiler controls pass, with negative admission/telemetry/source-
parity fixtures and owned-child cleanup. Real non-Go telemetry/timing smokes
pass on both Macs. Baseline exits 75 after the 600-second admission budget.
Zero Go/formatting stages executed; no runtime peak or test pass can be claimed.
Raw summary/samples are retained on both hosts with matching SHA-256 digests.
Prior acceptance remains valid:
local gate 8/8 at `bd13671`, exact-checkpoint native run `37871315522` SUCCESS,
terminal tracking pushed at `d3454ed`. That evidence contains no resource profile.

The extended profiler passes fifteen non-Go controls, including explicit-only
75% admission, unchanged load/process checks, exact pilot commands, CPU counter/
PID-reuse handling, mandatory budget, pre-launch timeout refusal and cleanup of
an owned child in a separate process session. Two-sample real older-host
attribution succeeds. New process-level raw evidence stays local and untracked.
The pilot result is **validation-failed**: four stages passed, AP observer failed.
Stage real times were 1.89s / 91.98s / 3.07s / 104.47s / 550.35s.
The failure is a nested observer timeout under existing budgets, not the
overall pilot cap. Raw stage logs, summary and samples are retained on both
hosts. This is neither full-suite coverage nor a passing lower-threshold result.

## Next Steps

1. Discuss a controlled follow-up of the unchanged AP observer; do not automatically
   repeat, raise timeouts, alter assertions or adopt a permanent threshold.
2. Preserve native run `37884132310` at `5eaccd6` before the pending documentation
   push. Raw evidence is already retained on both hosts; inventories remain private.
3. GH #13 implementation remains paused and separately authorized.

## Blockers

The original 80% attempt could not start. The 75% pilot executed but AP timed out.
Memory pressure remained normal in the samples, with no swapping; the cause of
the timeout remains unresolved. A permanent 75% rule is not justified by this
failed pilot or by measurements taken only at 77-79%.

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
  raw run is `older-mac-20261008`; summary and samples are mirrored locally.
- The separate pilot is `older-mac-75-20261008`, using the versioned remote
  `profile-validation-75.py`. Pre-pilot attribution is retained privately in
  `prepilot-attribution-20261008.json` on both hosts. Do not publish inventories.
- A local private `process-attribution-summary.json` accompanies the pilot's
  mirrored evidence. CPU averages use matched PID/start-time cumulative counters,
  not averages conditioned on appearing in a top-process ranking.
