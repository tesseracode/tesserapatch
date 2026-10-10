# Current Handoff

## Status

**Cluster state**: IN PROGRESS

The research-intake/GH #13 planning wave is durably closed at
`d3454edb72c37bacae9159481cc2e6cd69559459`; its acceptance is preserved in
[HISTORY](HISTORY.md). The operator has paused GH #13 to investigate whether
the inherited resource-admission policy unnecessarily prevents older-Mac tests.
The operator now authorizes a bounded deterministic artifact-unstable fixture
correction and an AP observer repeat with unchanged timeouts. No permanent
threshold change or GH #13 implementation is authorized.

## Active Task

- **Task ID**: `calibrate-test-resource-admission`
- **Milestone**: Validation infrastructure investigation, before GH #13
- **Description**: Investigate test-only analyzer retention and smaller process groups
- **Status**: Separate profiles PASS — measured test-memory optimization candidates; review/closure pending
- **Assigned**: 2026-10-08
- **WAVE_BASE**: `d3454edb72c37bacae9159481cc2e6cd69559459`
- **Frozen source baseline**: `d3454edb72c37bacae9159481cc2e6cd69559459`

WAVE_BASE = d3454edb72c37bacae9159481cc2e6cd69559459

## Session Summary

Separate rev-19 and rev-20 profiles now pass, including all four heap analyses
and exact eighty-path assertion coverage. Package times are 64.391s/57.664s;
peak sampled RSS is 8.07/7.29 GiB. Post-GC sampled retained heap is 3.03/2.78
GiB, while cumulative allocation volume is 52.82/49.15 GiB. Roughly 61% of
allocated bytes are attributed to binding-state constructor/clone flat sites.
All 274 samples show normal pressure/zero swap and owned cleanup is verified.
This identifies concrete test-only optimization targets; it is not a production
memory issue or a claim that 50 GiB of RAM is needed. No usable combined heap
profile exists, so no controlled reduction percentage is available.
No workload remains running on either Mac.

The split-only plan is verified and dispatched: rev-19 and rev-20 individually,
then four heap analyses. The original combined log proves eighty emitted
assertion paths (41 rev-19 subtests, 37 rev-20 subtests, two roots); the new
successful split union must match them exactly. Prior partial results remain
historical. New output is `memory-separate-profile-20261010`, with fresh caches;
source remains clean at `6ff988e` in `memory-split-repo-20261010`.

The operator authorizes rev-19 and rev-20 separately. Use unchanged frozen source
`6ff988e` / Go 1.26.5, fresh private caches and a 30-minute combined budget for
two test processes plus four heap analyses. No combined retry, old-Mac test,
cache implementation or canonical shard change. Retain the same pressure/
swap/load safeguards, external-Go permission and test timeouts.
Compare exact split RUN/PASS case coverage with the prior eighty emitted paths,
without treating that interrupted run as a completed combined heap baseline.

The heap comparison stopped safely on warning-level OS pressure during final
output/profile work of the combined command. Both top-level tests and all 78
subtests emitted PASS (80 matching run/pass paths), but there is no package-pass
event and `combined.pprof` is empty. No split run or analysis started, so no
completed comparison or live-heap attribution exists. The peak sampled owned
RSS sum was about 9.17 GiB; swap remained zero and no external Go competitor
was observed. Owned cleanup and the frozen checkout are verified.
No automatic retry is scheduled. Prefer a separately approved smaller-process-
first measurement before another heavy combined profile.

The bounded comparison is dispatched on isolated `6ff988e`, Go 1.26.5:
combined rev-19/20, rev-19 alone and rev-20 alone, with retained/allocation heap
profiles. Coverage-event controls pass before launch. The three tests and six
pprof analyses share a thirty-minute budget and pressure/swap/load safeguards;
external Go is allowed. No source/cache/CI assertion changes or old-Mac work.
Private output: `memory-split-profile-20261010`; source:
`memory-split-repo-20261010`. Native `38036488303` is now SUCCESS.

The operator approved the three-run rev-19/20 comparison on this Mac with a
30-minute combined budget and pressure/no-new-swap/load safeguards, external
Go allowed. Measure combined versus separate fresh test processes, retaining
Go heap profiles and proving identical run/pass subtest coverage. Source,
assertions, existing test timeouts, caches and canonical shards are not changed.
Profiles and test binaries remain private; the older Mac runs nothing.

The operator requests lower-memory/smaller-process investigation. Read-only
analysis found process-global test caches retaining each mutated source's AST/
type graph/model plus dependent binding/expression references. These caches
have no capacity/lifetime eviction; ordinary GC cannot reclaim reachable entries.
The mechanism is concrete, but per-cache heap attribution is not yet measured.

Combined rev-19/20 previously peaked around 11.10 GiB RSS, whereas an already-
single purge-progress test reached 9.64 GiB. Top-level splits may help but are
not a complete answer for monolithic fixture loops. Preserve existing cache
identity/invalidation/immutable-baseline guards. Next recommended measurement:
combined rev-19/20 versus each test alone on the current Mac, adding heap
profiles without changing assertions, test timeouts, caches or canonical shards.
No Go workload or code change was made during this investigation.

The older-Mac swap-tolerant probe is complete, exit 76: it exceeded the approved
2 GiB occupied-swap growth cap before finishing command 1. After about 30.4
minutes of execution, swap usage was 2.33 GiB (2.07 GiB above the 267.25 MiB
prelaunch baseline). The observed 70.75 MiB overshoot is within a sampling
interval, not permission to raise the cap. All 900 samples still report normal
pressure. Page counters show roughly 6.27 GiB out / 4.26 GiB in since prelaunch;
peak sampled owned RSS is about 8.33 GiB. No assertion failure or timeout is
reported, but zero broad commands completed. All owned processes are gone,
source/checkouts are preserved and evidence is mirrored. No retry is scheduled.
The prior current-Mac complete 22-command pass remains valid.

The older-Mac probe is dispatched on frozen `f80f7e2` and Go 1.26.5 after
forty controls and a harmless real telemetry smoke pass on that host.
Arguments: `--profile broad --admission-policy pressure --max-swap-growth-mib
2048 --max-seconds 3600 --wait-seconds 600`. Only command 1 runs; the actual
prelaunch occupied-swap value is the growth baseline. Keep normal-pressure/
monitoring safeguards, existing load/no-Go admission and test timeouts.
Directories are `swap-probe-repo-20261010` and `swap-probe-broad-20261010`;
all old checkouts/results remain intact. No result is claimed yet.

Forty non-Go controls pass for the explicit bounded swap-tolerant broad profile.
The cap stays relative to the fixed prelaunch swap usage; exact-limit, over-limit,
missing-telemetry, pressure/gap, prelaunch/completion and owned-cleanup cases are
verified. The default still stops on any new swap-outs. Prepare a fresh older-
Mac clone and Go 1.26.5 caches, verify its controls, then dispatch one probe.

The operator selected an older-Mac probe of the first broad command only,
allowing at most 2 GiB occupied swap growth from prelaunch while pressure stays
normal, within a 60-minute total budget. Keep original test timeouts, serial
execution, admission quiet minute/load/no-Go and default strict policies.
Current old-host swap is 267.25 MiB and inactive during a short read-only sample;
historical occupied swap is not itself a problem. Record rates/counters and
stop owned work on pressure warning/critical, growth beyond the cap or unreliable
monitoring. No full older-host partition or permanent policy change is authorized.
Native CI `38013960284` has now passed all five jobs at `075eccf`.

The complete concurrent-work experiment **passes all 22 commands** at frozen
`4e6de31` / Go 1.26.5, with identical shard arguments and unchanged assertions/
timeouts. Every pressure/swap/load admission window and completion sample is
verified. All 1875 samples show normal pressure and zero swapping; percentage
readings are 55-75%. Peak sampled owned RSS sum is about 11.17 GiB. External Go
work is actually present (up to nine tools; 793/1118 execution samples), so this
is not an idle-machine-only result. Total recording spans about 62.1 minutes.

The frozen checkout is clean. Post-run reconciliation identified one owned
orphan git-daemon and terminated only that verified PID; no recorded experiment
process remains. Other sessions were untouched. The requested experiment is
complete; independent review, final wave-close work and current hosted CI
confirmation remain. No permanent default or older-host policy has changed.

The concurrency-aware run is dispatched on frozen `4e6de31` with Go 1.26.5
and fresh isolated caches. Use `--profile partition --admission-policy pressure
--allow-external-go --wait-seconds 600`. The previous strict/partial runs are
unchanged. Both copies pass 35 controls; a harmless prelaunch probe refused
load1 5.42, demonstrating that the CPU condition remains active. This full run
will wait for the proper minute before starting tests.
Directories are `pressure-concurrent-repo-20261009` and
`pressure-concurrent-full-20261009` under the owned `bin/resource-baseline/`.

Thirty-five non-Go controls pass for explicit external-Go permission.
The flag changes only Go-presence admission/prelaunch checks: pressure, swap,
load, timing, ownership and existing test timeouts remain guarded. New telemetry
records external Go counts separately. Prepare a fresh isolated checkout/cache
for the one current-host run; no Go/test source changed.

The operator explicitly authorizes one current-Mac full pressure-policy run
allowing external Go processes. Add a pressure-only opt-in; retain the complete
60-second normal-pressure/no-new-swap/load<=5 window, runtime resource stops,
own serial execution and isolated frozen source/caches. External Go activity
is measured, not automatically blocked or terminated. Strict defaults and all
test assertions/timeouts stay unchanged. No older-Mac retry is authorized.
Prior native `38007045267` at `d765eab` has passed.

Both pressure-policy runs have finished. The current 32 GiB host passed
commands 1-10, then timed out waiting for a quiet minute before command 11.
Pressure stayed normal with zero swapping; other Go activity occupied 218 of
292 final admission samples, leaving at most 49.930 continuous qualifying
seconds. Load was within the limit during that window. Peak sampled owned
RSS sum was about 11.06 GiB in command 4. Twelve commands remain unexecuted.
The older host stopped on new swap-outs during command 1, as recorded below.
The specified two-host experiment and comparison are complete; full acceptance
is not. No workloads or automatic retries remain running.

The older 16 GiB experiment stopped safely during command 1 on new swap-outs:
0 -> 174,734 pages, with 683.25 MiB swap used. OS pressure still read normal;
the independent swapping guard triggered correctly. Peak sampled owned RSS sum
was about 6.69 GiB. No test assertion failure is reported, but **0/22 commands
completed**; early package passes do not constitute partition coverage.
Owned Go processes are gone, the frozen checkout is clean, and raw evidence is
mirrored/hash-verified locally. No retry is authorized. The current 32 GiB
run's final result is recorded above; the older stop remains unchanged.

Both experiment checkouts are clean at
`2cba06811ce86025a11f4f7a1c31f4a5e7b6fda9`; Go 1.26.5 is matched and
runtime/cache override checks are clear. Thirty non-Go controls and real
pressure-safety smokes pass on both hosts. Dispatch one full partition per host
with `--profile partition --admission-policy pressure --wait-seconds 600`.
Current-host competing Go work may delay admission; no unrelated process will
be stopped. No result is claimed until each run terminates.

Thirty non-Go controls pass for opt-in pressure policy, including fail-sensitive
admission and runtime checks, prelaunch/completion sampling and owned-process
cleanup. CLI/runtime/test Go code and all shard commands are unchanged.
Prepare separate frozen checkouts and fresh build/module caches on both hosts
so neither previous evidence nor the older Mac's original research is changed.

The operator authorizes one full pressure-based experiment on each Mac. Add an
explicit opt-in mode: normal OS pressure and no new swap-outs for 60 seconds,
with unchanged load1 <=5/no-Go/serial requirements; percentage is diagnostic
only. Stop the owned workload on warning/critical pressure, new swap-outs or
unreliable telemetry. Keep all twenty-two commands and original test timeouts.
Use matching source/Go 1.26.5 and fresh isolated caches on both hosts, preserving
all existing worktrees and evidence. No permanent policy change or automatic
retry follows either outcome. Prior native run `37979067094` has passed.

The full monitored attempt at `d0376d6` completed its first command successfully:
all selected packages pass, including the earlier AQ failures. The second
command never started because its 600-second admission wait remained at 63-71%,
below both 80% and 75%, with no active Go tools. All samples showed normal
pressure and zero swap. The passing command peaked near 9.22 GiB summed RSS
on this 32 GiB Mac. This is **1/22**, not a complete partition pass. No further
run is scheduled; any different admission policy requires an explicit decision.

The operator now explicitly requests the complete test partition with resource
monitoring on this Mac. Dispatch the exact existing twenty-two serial commands,
retaining all selectors/assertions, original 40-minute package timeouts and
fresh >=80%/load<=5/no-Go/60-second admission before each command. Admission
wait is capped at 600 seconds; stop at the first failure. Focused 75% experiments
do not silently change the full-suite rule. Preflight is 78%, so the first window
may block before any test. The old Mac remains idle.
New evidence directory: `current-mac-full-partition-20261009`.

The repeat at `299841d` passes all four identical commands. Focused CLI tests
take 105.792s (107.32s command). The 79% preflight rose to 80-81% for test
admission, with test execution later reaching 79%. Vet/build admit at 79-80%;
the original 80% predicate would not qualify their recorded windows. The test
window itself qualifies under both policies. No sample reached exactly 75%.
All 176 samples show normal pressure/zero swap; peak sampled owned RSS sum
about 2.92 GiB. No full-suite or permanent threshold decision follows this result.

The operator requested another resource check and the focused trial if around
75%. This Mac now reports 79%, normal pressure, zero swap and no Go tools, with
load1 2.44. Dispatch the identical bounded focused workload to new evidence
directory `current-mac-focused-lowheadroom-20261009`, changing only the output
location from the prior runner. Prior raw measurements remain unchanged.
The older Mac is reachable; no test is started there. Fixed-source native CI
`37955807665` at `d91c204` has completed SUCCESS.

The current-Mac focused experiment has passed at `611522c`: formatting,
fixture/AP/AQ tests (128.349s package), vet and build. This validates the latest
AQ guard integration and mutation-sensitive controls. Actual admission stayed
84-87%, with normal pressure and zero swap; it does not demonstrate that the
75% exception was needed or validate operation below 80%. Peak sampled owned RSS
sum was about 2.93 GiB. The bounded experiment is complete; no full partition
is automatically started and no permanent policy change is made.

The operator now authorizes a bounded 75% focused experiment on the current
32 GiB Mac: formatting, fixture/AP/AQ checks and mutation controls, vet/build,
with original load/process/60-second safeguards and unchanged test budgets.
The workload cap is 30 minutes; the full 22-command partition is not dispatched.
Preflight is 86%, so the original rule may also qualify. Record actual conditions
and do not credit the lower threshold with a pass that occurs above 80%.
Prior native run `37892216801` has completed SUCCESS, so it no longer blocks a
push; this older-source CI is not proof for the latest guard correction.

The operator accepted the concrete diagnostic finding and assigned its correction.
Add a nil-by-default internal hook after a successful rooted read, before the
existing post-read descriptor check. The fixture appends one real byte there,
requiring the unmodified size checks and real CLI route to report instability.
Remove the probabilistic busy writer/retries; preserve human/JSON/dry-run
assertions and prove no command writes remain after restoring fixture mutation.
Compiled-source negative controls must reject lost synchronization and bypassed
capture checks. Test timeouts, production classification rules and redaction stay
unchanged. Historical investigation evidence below remains valid.

The user asked why 80% is required, what peak resource use was recorded, and
whether a measured comparison can make testing viable on the older Mac.
The user selected an instrumented baseline only, with candidate selection
after measurements, on the separately reachable older machine. The operator
also explicitly authorized a separate checkout under its `tpatch/bin/` and
an isolated Go 1.26.5 download, leaving the existing checkout untouched.
After the 80% baseline was admission-blocked, the operator explicitly approved
a 75% experiment and asked what startup/background processes consume resources.
Use a 30-minute pilot budget, not an unrestricted full-suite attempt.
The operator has now authorized the recommended unchanged AP-only repeat under
quieter background conditions. Keep the same experimental admission and all
test timeouts/assertions; preserve nested stdout to expose timeout diagnostics.

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
- Pre-follow-up sampling shows lower background load and media-analysis memory.
  The earlier nested CLI process used about 402 CPU-seconds over 479 observed
  seconds. PIB-464 enters a shared contract-baseline builder; inspect the actual
  timeout stack before assigning cause or proposing optimization.
- Repeat only AP with a 30-minute budget and the original 60-second admission
  rule at experimental 75%. Copy the prior isolated caches into a fresh run
  directory and record their provenance; old evidence/caches remain unchanged.
- A private byte-preserving tee will retain nested overlay-test stdout, including
  timeout diagnostics currently omitted from the observer's final error.
  No source, selector, assertion or test-timeout change is authorized.
- AP repeat reproduced the inner timeout despite quieter background activity.
  All 298 samples show normal pressure/zero swap; max owned RSS sum about
  1.70 GiB. Nested CLI used about 399 CPU-seconds over 478 sampled seconds.
- Its nested-output capture failed: Go prepends GOROOT/bin to the test PATH,
  bypassing the tee. Empty capture is recorded explicitly, not treated as success.
- Corrected instrumentation adds top-level `-exec` only to restore the tee PATH,
  preserving the test binary arguments. Capture must include a nested AP run
  event or the diagnostic run is explicitly incomplete. One corrected attempt
  is capped at 900 seconds; combined with the prior ~623 seconds, workload
  remains within the original 1800-second diagnostic allowance.
- Corrected capture succeeded: 315 nested events reveal PIB-461 fails after
  11.84s because forty attempts do not produce the intended artifact identity
  race. The last dry-run response is success/planned. This assertion failure
  happens before the later observer timeout; a larger timeout cannot fix it.
- At the later PIB-464 timeout, the stack is inside the same unstable-artifact
  fixture, invoked through the shared S6 baseline, publishing/redacting a
  3,225,600-byte artifact while its concurrent writer is active. The unsynchronized
  fixture is not reliably inducing its intended condition on this host.
- All 297 corrected-run samples show normal pressure, no swap, a 76% admission/
  execution metric and load1 1.40-3.50. Peak sampled owned RSS sum is about
  1.75 GiB. No external or surviving Go process was observed.
- Combined diagnostic follow-up workload is 1243.824 seconds, below 1800.
  No more runs are scheduled. No Go test source or production behavior changed.
- Tests, assertions, selectors, serial settings and timeouts remain unchanged.
- Capture continuous measurements and durable summaries in the new experiment;
  distinguish sampled RSS sums from exact physical footprint/high-water marks.

## Files Changed

- `internal/cli/prepare_publish.go`: nil-by-default post-read synchronization
  callback, error propagation, and unchanged real descriptor/path checks.
- `internal/cli/prepare_s6_contract_test.go`: one-byte deterministic fixture,
  real refusal assertions, mutation count and whole-workspace preservation.
- `internal/cli/prepare_capture_unstable_test.go`: stable/mutated/error controls,
  CLI presentation/snapshot checks and actual compiled-source negative controls.
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
- `scripts/profile-go-output.py`: private byte-preserving capture of nested
  observer stdout; all other Go commands are exec pass-through.
- `scripts/profile-test-exec.py`: transport bootstrap correcting Go's PATH
  rewrite, with a negative regression fixture and mandatory capture validation.
- `older-mac-ap-followup.json` beside the other aggregate reports: repeated
  timeout, resource measurements and explicit missing-diagnostic disclosure.
- `older-mac-ap-diagnosis.json`: successfully captured first assertion failure,
  timeout stack chain, exact resource measurements, evidence hashes and limits.
- `older-mac-fixture-ap-pass.json`: fixed-source AP pass, actual nested row
  results, unchanged budgets, resource bounds and evidence hashes.
- `internal/cli/prepare_s7_aq_recovery_test.go`: extend existing nil-seam proof
  to the new capture hook, with initializer/assignment/escape/signature negatives.
- `local-fixture-suite-first-shard.json`: failed first full-suite shard and
  its larger measured memory peak on the 32 GiB current Mac.
- `current-mac-focused-75-pass.json`: validated AQ correction, selected test
  results, actual above-80% admission conditions and resource bounds.
- `current-mac-focused-lowheadroom-pass.json`: identical-command repeat,
  admission-window replay comparison, observed 79-81% conditions and limits.
- `current-mac-full-partition-admission-stop.json`: first broad command PASS,
  second-command admission block, 9.22 GiB measured RSS peak and interpretation limits.
- `pressure-older-full-resource-stop.json`: actual swapping-triggered safety
  stop on the older Mac, interrupted coverage and measured resource limits.
- `pressure-two-host-comparison.json`: ten passing current-host commands,
  competing-Go admission block, per-command log hashes, larger analyzer RSS
  peak and comparison limits against the interrupted older-host workload.
- `pressure-concurrent-full-pass.json`: all twenty-two exact commands passing,
  per-stage provenance, real external-Go presence, resource bounds and cleanup.
- `older-mac-bounded-swap-stop.json`: capped swap-growth probe, two-way paging
  counters, interrupted coverage, verified cleanup and interpretation limits.
- `test-memory-investigation.json`: verified retention paths, cache-correctness
  constraints, measured shard peaks and proposed bounded split/heap comparison.
- `memory-split-profile-resource-stop.json`: warning-pressure stop after emitted
  combined test passes, empty profile, unavailable comparison and evidence hashes.
- `memory-separate-profile-pass.json`: complete separate test/profile results,
  exact coverage union, retained versus allocated heap and source-attributed hotspots.

## Test Results

**Current result**: the concurrent-work pressure-policy run passes all 22 commands
at `4e6de31`, with verified logs and original test budgets. Focused fixture/AQ
checks, mutation controls, vet/build and earlier source-equivalent hosted CI
also pass. Latest hosted `38013960284` is pending macOS. Earlier partial/failed
events below remain historical evidence, not the current partition result.

The new Go fixture correction is not yet validated. Formatting is applied;
the local original-policy runner will check formatting, focused fixture/AP
controls (including source mutations), vet and build, stopping on first failure.
Prior measurements below refer to the unchanged, failing baseline only.
First targeted run at `60f40cb` passed formatting/direct capture/mutation controls
but failed the exact-one-mutation composition assertion: normal prepare reads
before and after acquisition. Corrected the fixture to arm only after real
authority acquisition for mutating routes; dry-run is armed immediately.
The exact-one/refusal/snapshot controls are unchanged. A fresh targeted run is
required; the first failure remains in `fixture-local-20261009`.
The corrected `ba15a05` run now passes formatting, targeted CLI tests (74.507s),
vet and build, each after a fresh original-policy 80% admission minute.
Mutation controls actually compiled broken source and observed the specific
expected assertion failures. Old-Mac AP and the full local shard partition are
dispatched independently on their respective hosts; no result is claimed yet.
The older-Mac AP result is now **PASS** at exact `ba15a05`: 444.71s command,
360.457s observer package, 296.176s nested CLI. Captured PIB-461/464/469 all pass;
244 nested events have zero failures. All 242 resource samples show normal
pressure/zero swap at 77%; peak owned RSS sum about 1.73 GiB. No Go processes
remain on that host. Full local partition at source-equivalent `c360dc9` remains
pending, as does new-source hosted confirmation.
Full-partition shard 1 failed two AQ analyzer checks: new callback unresolved.
No later shard ran. The coupled test-only correction proves the hook is inert
in production before classifying it; it retains exact call inventory and all
forbidden-gate controls. New negative fixtures reject activated/escaped/mistyped
hooks. The first-shard peak was about 8.37 GiB RSS sum, much higher than AP-only,
with normal pressure and no swap on this 32 GiB host.
The guard correction is committed at `6984043`, but its validation exhausted
600 seconds before formatting: 289 samples at 77-78%, load1 1.66-4.24, no Go
tools. No command ran; no corrected-guard pass is claimed. Preserve
`fixture-aq-guard-20261009`; no automatic retry or local threshold relaxation.

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
Eighteen non-Go profiler controls now pass after adding AP-only command parity,
isolated cache-copy provenance and failure-sensitive byte/exit-status capture.
Source and all Go test assertions/timeouts remain unchanged.
The AP diagnostic follow-up failed with the same timeout. Its resource samples
are retained, but nested output was not captured. Nineteen profiler controls
pass after modeling the actual Go PATH rewrite and requiring captured AP events.
The corrected bootstrap now has real-Go evidence: capture verification passes
and nested output is retained on both hosts. The test still fails. PIB-461's
fixture assertion precedes the inner timeout in PIB-464; the stack confirms the
same fixture on a publication/redaction path. Command time 557.40s; exit 1.
This diagnostic success is not a validation pass or threshold approval.

## Next Steps

1. Retain the completed separate profiles and choose a bounded test-only
   cache-lifetime or binding-copy optimization before changing implementation.
   Preserve cache identity/invalidation and mutation-sensitive correctness guards.
   Do not claim the empty combined profile as a live-heap baseline.
2. Retain the completed two-host comparison and all partial outcomes.
   The older-host swap stop needs a separate workload/resource decision, not
   an automatic retry or a claim that every 16 GiB machine is inadequate.
3. Retain passing AP evidence from separate fixed-source older-Mac checkout
   `fixture-repo-20261009` at `ba15a05`, explicit 75% and unchanged timeouts.
   `older-mac-fixture-ap-20261009` is mirrored locally; failing baseline preserved.
4. Prior native `38036488303` at `cfe16c6` is SUCCESS. Preserve active
   `38076313773` at `880345a` before the pending result push.
   Independent review/final closure and permanent policy adoption remain separate.
5. GH #13 implementation remains paused and separately authorized.

## Blockers

The diagnosed fixture failure is corrected and the old-Mac AP observer passes.
The AQ guard correction passes focused validation and fixed-source native CI.
The complete current-host partition now passes with external Go allowed and
pressure/swap/load safeguards retained. No local experiment is running.
Independent review, latest hosted confirmation and final wave closure remain.
The older capped-swap probe has also stopped on its growth limit and is not a
pass. No validation workload is running on either Mac. Optional old-host resource
optimization is separate from accepting the validated current-host fixture fixes.
Separate profiling now measures large retained typed-model/binding state and
substantial binding-copy allocation churn. No memory optimization has yet been
implemented or assigned. The combined heap profile remains unavailable, and
7-8 GiB individual RSS does not establish full-suite viability on the older Mac.
No workload is currently running on either Mac.
The older outcome is a measured resource stop, not permission to weaken safety.
Full-suite resource requirements on the older Mac and the exact 75% boundary
remain unvalidated; no permanent threshold change is adopted.

## Context for Next Agent

- Successful full run: `pressure-concurrent-full-20261009`, exact source
  `4e6de31` in `pressure-concurrent-repo-20261009`. Retain all raw evidence.
- One experiment-owned orphan git-daemon required explicit post-run cleanup.
  Its ownership was established from recorded parent/start-time data; do not
  generalize this into permission to terminate other sessions' processes.
- Completed two-host experiments used exact `2cba068` in separate frozen clones,
  not the mutable primary worktree. Directories:
  `pressure-current-repo-20261009`, `pressure-older-repo-20261009`,
  `pressure-current-full-20261009`, `pressure-older-full-20261009`.
- Keep raw process inventories private. Older-host summary/samples/logs are
  already mirrored and verified; do not copy its potentially large caches.
- Legacy percent admission remains the default. Pressure mode's safety stop
  (exit 76) is not a test failure; missing telemetry remains explicit failure.
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
- Follow-up output: `older-mac-ap-followup-20261008`; remote profiler:
  `profile-validation-ap.py` plus `profile-go-output.py`. The prior run's caches
  are copied, not reused in place; copied-cache and tee differences must remain
  explicit when comparing outcomes. Mirror its child-output directory locally.
- Corrected output: `older-mac-ap-capture-20261008`; never overwrite the failed
  follow-up's empty capture or old telemetry. Bootstrap adds `-exec` transport
  to the outer Go command only; inner test budgets and assertions are unchanged.
