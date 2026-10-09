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
- **Description**: Correct the unstable-artifact fixture without weakening real capture/refusal checks
- **Status**: In progress — authorized current-Mac focused 75% experiment dispatched
- **Assigned**: 2026-10-08
- **WAVE_BASE**: `d3454edb72c37bacae9159481cc2e6cd69559459`
- **Frozen source baseline**: `d3454edb72c37bacae9159481cc2e6cd69559459`

WAVE_BASE = d3454edb72c37bacae9159481cc2e6cd69559459

## Session Summary

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

## Test Results

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

1. Await the explicitly authorized current-Mac focused 75% experiment and
   preserve its metrics. No full-partition run or permanent policy change.
2. Retain passing AP evidence from separate fixed-source older-Mac checkout
   `fixture-repo-20261009` at `ba15a05`, explicit 75% and unchanged timeouts.
   `older-mac-fixture-ap-20261009` is mirrored locally; failing baseline preserved.
3. Prior native run `37892216801` at `2156ae8` is SUCCESS; newer source still
   needs independent full validation and hosted confirmation before acceptance.
4. GH #13 implementation remains paused and separately authorized.

## Blockers

The diagnosed fixture failure is corrected and the old-Mac AP observer passes.
The AQ guard integration correction awaits the current focused experiment.
Full local partition and new-source hosted confirmation remain outstanding.
The older Mac is idle; the only dispatched local work is the bounded focused run.
Full-suite resource requirements on the older Mac and the exact 75% boundary
remain unvalidated; no permanent threshold change is adopted.

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
- Follow-up output: `older-mac-ap-followup-20261008`; remote profiler:
  `profile-validation-ap.py` plus `profile-go-output.py`. The prior run's caches
  are copied, not reused in place; copied-cache and tee differences must remain
  explicit when comparing outcomes. Mirror its child-output directory locally.
- Corrected output: `older-mac-ap-capture-20261008`; never overwrite the failed
  follow-up's empty capture or old telemetry. Bootstrap adds `-exec` transport
  to the outer Go command only; inner test budgets and assertions are unchanged.
