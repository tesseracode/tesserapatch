# Current Handoff

## Status

**Cluster state**: APPROVED

GH #13 S1 is dispatched after accepted S0. The upstream Git-strategy research
remains queued; the operator selected backlog-only research and S1 next.
No strategy policy, full research deliverable or research cluster was adopted.
The S1 production implementation is independently approved and its required
native CI passes. The newer test-only sharding proposal below is not published.
S1 acceptance is still blocked on the complete local gate, which stopped on
warning OS memory pressure. This is not a shipped slice.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s1`
- **Milestone**: GH #13 / ADR-037 and ADR-043
- **Description**: S1 closure — test-only memory-bounded validation maintenance
- **Status**: Static review and all six pilots passed; preparing full 39-shard/native validation
- **Assigned**: 2026-10-29
- **WAVE_BASE**: `5120dc9395769fe8148b61836e546729b66cb4dc`
- **Contract**: ADR-037/PRD rev-7 as narrowly amended by accepted ADR-044; PRD §8 S1, §6.9, §6.13
- **Prior accepted S0 close**: `6ab392996467662572b155574a0509a6269cae8f`
- **Unaccepted first implementation**: `252e4940f04b183a90c0a6a2f6ef7cf4efeb8888`
- **Revised review checkpoint**: `f5df7852a39a476808a9146f65970a809b7ea44c`
- **Accepted amendment checkpoint**: `8b22f99263827a53b5b66fb534e86921563a5d9f`
- **Current frozen code checkpoint**: `3054b84e9ea175ac6a959d32d012788e584918df`
- **Published validation checkpoint**: `ddc59b70042ef3fdca299963292fc5e42149a7d6`

WAVE_BASE = 5120dc9395769fe8148b61836e546729b66cb4dc

## Active Maintenance — memory-bounded test shards

- **Task ID**: `maintain-memory-bounded-test-shards`
- **Maintenance base**: `56311c4fa13fa45b28ebb00a86ef3021d70c7b04`
- **Authorization**: operator requested implementation and measurement of the
  finer-sharding proposal. S1 code stays frozen; this is not S2.
- **Baseline**: clean HEAD equals fetched origin/main; no stashes, one main
  worktree. Tracking-tip CI `40927388746` passes.

The previous successful gate's largest processes were the broad shard
(10.37 GiB), purge observer (9.80), AR revisions 16-18 (9.15), and purge guard
(9.08). The purge observer executes the same purge guard through its existing
marker overlay, so fixing the guard's fixture lifetime can improve both
without changing observer assertions.

Propose 39 serial test commands: retain all existing AR/observer coverage,
split revisions 16/17/18 into separate processes, and partition the broad
S6/S7/AVP families into smaller groups. New family commands must cover all
packages, not just CLI: static inspection found matching names in intent,
gitutil, store and other packages. Keep a complementary catch-all command.

For oversized individual tests, reuse the existing four-fixture memo lifetime.
The purge guard has flat loops rather than leaf subtests; provide a completion
callback for those loops without moving or dropping assertions. Keep the
six prior memo-scope changes and extend the exact-delta guard only for the
new named loops. Preserve the stricter S0 golden-capture boundary.

Bounded surfaces: CI, the canonical shard script, profiler and its tests,
S7 CI partition guards, S6 test-cache helper, the named AR test loops, finite
S0/S1 scope guards, and tracking. No production, contract, assets, golden,
dependency or resource-admission changes. Measure the proposal before claiming
lower peak memory or adopting it as successful full validation.
The draft touches eight bounded test/tooling files. All 40 profiler unit tests
pass. The first admitted formatter run caught misplaced new test declarations;
their placement is corrected before retrying. That failed log is preserved
under `bin/roc-sharding-20261029/shard-proposal-format-20261029/`.
Draft checkpoint `7a1052d` formats successfully. Admitted controls pass the
workflow/S0 package, including the nineteen-line exact-delta guard, but expose
two stale CI-test assumptions: the reorder fixture's formerly adjacent
commands and the former two all-package invocations. Correct those anchors
and validate each of the now-seventeen all-package invocations against its
exact command/step contract, including explicit non-CLI-coverage negatives.
No memory result or proposal acceptance is claimed yet.
Revision `31fd46e` now passes all selected workflow/CLI controls: S0 byte/scope
preservation, cache lifecycle, shard ownership, exact CI/script parity, AVP
root inventory and native Windows registration. Fresh admission completed
without resource stops. Freeze this code for independent review and measure
the purge guard/observer, revisions 16/17/18 and the new catch-all separately.
Independent reviewer: `72126da2-3640-4113-b76e-7724dc2109e3`,
`roc-memory-shard-review`, explicitly requested OpenAI `gpt-5.3-codex`
(runtime confirmation pending). It is read-only and must not run Go.
Parent pilot shell `roc-sharding-memory-pilots` runs the six selections
serially from `bin/roc-sharding-20261029/proposal-rev1-source`, with separate
fresh admission and evidence for each. Do not start another Go workload.
The eventual full-gate harness is prepared in
`bin/roc-sharding-20261029/full-gate/`; its nine routing controls pass, but
the full gate is not configured or running yet.
The independent review now APPROVES static code at
`31fd46ece44806c63eefb76a0760a0a504e05269`, with no significant issues.
Runtime metadata confirms `gpt-5.3-codex`. Parent-owned memory pilots remain
the next evidence; no measured reduction, native or full-gate pass is implied.
The first pilot now passes: purge guard sampled combined RSS falls from
9,750,872,064 bytes (9.08 GiB, prior successful gate) to 1,665,761,280 bytes
(1.55 GiB), about 83% lower. OS-reported maximum RSS is 1,884,553,216 bytes
(1.76 GiB). Command time is 186.00s versus the prior 78.62s; these are
recorded runs, not a controlled speed benchmark (the new run observed up to
four external Go processes). The assertions pass without new swap-outs.

The observer pilot was then stopped by the unchanged guard on 21,682 new
system swap-out pages, despite normal pressure samples. It reached
1,630,289,920 bytes sampled owned RSS (1.52 GiB) before stopping; that is
not a completed-run peak or a pass. Up to three external Go processes were
observed, but the system-wide swapping cannot be attributed from this data.
Fail-fast prevented revisions 16/17/18 and catch-all from starting.
All thirteen sampled owned process identities are gone after cleanup.

Evidence: `bin/roc-sharding-20261029/shard-pilot-purge-20261029/` and
`shard-pilot-purge-observer-20261029/`. No validators remain active. Preserve
both records and obtain a stable, quieter memory window before continuing
the five remaining pilots; do not weaken the no-new-swap rule. The draft
remains local, statically approved but not adopted as a successful full gate.
The operator has now requested a fresh continuous 60-second pressure check
and automatic continuation if it qualifies. The 16:09 snapshot is normal
with about 9.8 GiB free RAM; this snapshot alone is not admission.
The clean pilot checkout still equals reviewed `31fd46e`; only tracking
has changed in the parent. Resume observer, AR16, AR17, AR18 and catch-all
serially using fresh `*-resume1-20261029` evidence stages. The successful
purge pilot is not repeated. Shell `roc-sharding-remaining-pilots` owns this
sequence, with output in `bin/roc-sharding-20261029/remaining-pilots-resume1.log`.
Every command retains its own fresh 60-second admission, 600-second admission
budget and immediate resource/failure stop. No duplicate Go workload.
All five resumed pilots now pass at unchanged `31fd46e`, completing all six
pilots. Each execution stays at normal memory pressure with zero new swap-outs.
Admission takes 61.6-163.2s as needed for a full qualifying minute. All 225
sampled owned process identities from the resumed sequence are gone.

| Pilot | Sampled combined peak RSS | Command seconds |
|-------|---------------------------|-----------------|
| Purge guard (earlier completed pilot) | 1.55 GiB | 186.00 |
| Purge observer | 1.74 GiB | 191.42 |
| AR16 | 0.47 GiB | 15.13 |
| AR17 | 1.10 GiB | 71.70 |
| AR18 | 1.11 GiB | 100.50 |
| New catch-all | 1.43 GiB | 838.30 |

The directly comparable purge/observer baselines were 9.08/9.80 GiB.
The old combined AR16-18 process peaked at 9.15 GiB. The new catch-all excludes
the fifteen extracted family shards, so its peak is not a measurement of
their combined coverage. These six results do not establish a memory ceiling
or a pass for the complete 39-command partition.

With static review, preservation controls and pilots passing, prepare the
reviewed proposal for native CI and the full admitted gate. The canonical
APPROVED field records code/proposal approval only; S1 acceptance still needs
all 39 test commands and native CI. No production/test code changes follow
the frozen review checkpoint.

## Session Summary

The documentation-only research intake is complete and archived in HISTORY.
Intake CI `40834229022` passed all five required jobs. The user's clarified
support-branch scenario and backlog-then-S1 decision are published at `5120dc9`,
verified clean and equal to fetched origin/main before S1 dispatch.

Preflight reads confirm the two typed-root constructor signatures, no-lazy-fetch
contract, candidate namespace, nonblocking lock/filesystem rules and current
S0 scope guard. Existing Windows CI does not imply every new package's tests
are blocking; native S1 lock assertions need explicit required-job coverage.
The first S1 implementation is present in the worktree but not accepted.
Implementation was initially assigned to `roc-s1-implementation`,
agent `6c67ce6f-1058-4f46-bd48-8c663eafe87c`, with OpenAI `gpt-5.3-codex`
explicitly requested and now confirmed by runtime metadata.
It owned code/tests and necessary native-CI registration, not tracking or Git
commits. After its connection failure the parent owns all implementation.
The admitted command runner is `bin/roc-s1-20261029/validate.py`, with separate
S1 evidence/cache paths and the unchanged pressure/swap/load policy.
Parent review identified held-root/cleanup ownership gaps, inherited Git
environment redirection, lock path/type issues, a non-crash crash test,
capture/source-guard weaknesses and a committed-tree-only hash primitive.
The full finding details are prepended to LOG. Return corrections to the
same implementer rather than dispatch a competing agent on these files.
The first pass and findings are committed locally at `252e494`, not pushed.
The same implementer has returned revised code and admitted focused passes:
held roots, identity/environment checks, safer locks, actual termination tests,
stronger scope/envelope guards and a worktree-tree hashing primitive.
No independent approval, full suite or native Windows runtime pass is claimed.
Independent review was dispatched under agent
`e7b49e9c-874f-4fa0-935b-852cf6a0ec1e` against frozen `f5df785`,
with OpenAI `gpt-5.3-codex` explicitly selected. Parent integration shell
`roc-s1-parent-integration` waited for resource admission without running
Go. Its saved stage is `s1-parent-s0-ci-integration`; it later timed out.
Independent review by `e7b49e9c-874f-4fa0-935b-852cf6a0ec1e` confirms runtime
OpenAI `gpt-5.3-codex` and returns NEEDS REVISION: non-composable lock ownership,
half-closed prune failure cleanup, weaker tree-hash environment, and incomplete
root/Git source-inventory guards. Parent also found unreleased lock handles on
Prune's success path. Full findings are in the latest LOG entry.

The first finding crosses the accepted fixed constructor signatures and
transaction hold scope. Ask for a narrow ownership/API contract clarification
before inventing implicit reentrancy or another root constructor.
The parent integration check stopped after its 600-second admission budget
without launching Go. This is not a test failure; retain its logs.
The operator selected `amend-capabilities`. The ADR-044 draft made create/prune
borrow explicit live feature/global handles while the caller owns acquisition,
transaction-wide hold and release. Both named constructors remain; only the
candidate constructor's required parameters and prune contract change.
Proposal checkpoint `9bd80fa` was submitted for same-context read-only review
by `e7b49e9c-874f-4fa0-935b-852cf6a0ec1e`. Approval of the paper alone
does not close the implementation findings.

The proposal is now independently APPROVED (OpenAI `gpt-5.3-codex` runtime
reconfirmed), and ADR-044 is Accepted. The narrow ADR-037/PRD signature and
ownership clauses are updated. All nine paired parity blocks and existing
matrix row identities remain unchanged; the frozen ADR index is untouched.
Implement the accepted API and remaining findings without reopening other
decisions. No revised-code acceptance is claimed by the paper approval.
The existing implementer `6c67ce6f-1058-4f46-bd48-8c663eafe87c` attempted
that contract and the outstanding cleanup/environment/source-guard
corrections before its connection failure.
The implementer has now failed on model-request network timeouts. Its partial
code edits survived and no validation process remains active. The parent resumes
directly without reset/cleanup or another implementation agent.
Last admitted stage `s1r3-gofmt-20261029a` failed on misplaced declarations;
repair that syntax before any new test run. No post-amendment pass is claimed.
The parent repaired syntax; formatting passes. First recovered candidate/lock
tests pass the lock package but fail two outdated candidate expectations.
Corrections now assert refusal of replaced residue/foreign data preservation
and explicit null Git-config overrides. The S0 capture/stage split and
alternative-root/Git-command guards are also tightened before the next run.
Recovered candidate/lock packages pass. Wider checks exposed stale tree-env/
Windows-row expectations and the original AVP-141 three-opener inventory.
The dependent inventory is updated to pin the original three prepare sites
plus the two S1 sites, with moved/unregistered negative controls.
The CI guard's ignored-bin clone scan requires a clean isolated checkout for
the rerun, not deletion of evidence or changes to the scanner.
Recovery checkpoint `9198c0ac5e7dac1fac76902f0863662e37af9f82` is saved locally.
The clean-checkout focused integration now passes all five selected packages,
including S0, AVP-141 and the CI guards, without golden capture.
A final lifetime check identified that copying an exported Handle could give
one descriptor independent borrow counters. The parent is correcting this
with shared private state and a copy/release regression, including CLW-005.
This is part of ADR-044's existing exclusion obligation, not a new API.
The shared-state fix is saved at `f6bae35`. Its clean-checkout formatting and
complete candidate/candidatelock/gitutil/workflow packages pass. Return this
frozen revision to the existing reviewer; no code acceptance is implied.
That review closes composition, cleanup, source guards and copied-handle
lifetime, but returns NEEDS REVISION for inherited `GIT_CONFIG_PARAMETERS`.
The parent is scrubbing it and strengthening both subprocess envelope tests.
The missing scrub is implemented. Both strengthened tests fail against the
pre-fix helper specifically for inherited `GIT_CONFIG_PARAMETERS`; positive
revalidation and independent closure of this correction are next.
Positive formatting, complete candidate/gitutil tests and S0/CLI integration
now pass at `3054b84`. Return only the residual correction to the retained
reviewer; previously closed findings remain closed unless new evidence appears.
The reviewer now APPROVES the residual correction at `3054b84`, closing the
last code finding. Updated vet/build pass. Publish the reviewed checkpoint
with this tracking transition for native CI and a clean-source full gate.
Do not push additional tracking-only commits while its exact-tip CI runs.
Published `ddc59b7` was fetched equal to origin/main, with v0.17.0 unchanged.
Native CI `40917207659` and the isolated full local gate are running against
that exact checkpoint. The gate has passed mechanical checks 1-5, including
all 19 wave trailers; Go validation has begun admission. This progress record
is committed locally only while CI is in flight, avoiding cancellation.
The local gate has now stopped on warning OS memory pressure during its first
test command, after mechanical checks 1-7 passed. No assertion failure is
reported in the captured output; zero complete test shards are credited.
There were no new swap-outs and owned-process reconciliation finds no survivors.
Do not relax admission or blindly repeat the same load; more runtime headroom
is needed before a full retry. Native CI continues independently.
Native CI `40917207659` has now completed successfully at `ddc59b7`: all five
required jobs pass, including Linux/macOS suites, both observer jobs and
Windows. Release is correctly skipped. No validation processes remain active.
The previously local progress records can now be pushed without cancelling
that completed exact-checkpoint run; no S1 acceptance is implied.

## Current State — bounded S1 requirements

1. `OpenReferenceRoot(repoRoot string, commit string)` and
   `CreateCandidateRoot(repoRoot, slug, candidateID, commit string,
   featureLock, worktreeMutationLock *candidatelock.Handle)` are the
   only candidate-package bare-root constructors. Validate lowercase 40-hex
   commits and local object presence; keep subsequent repository IO typed.
2. Every candidate/reference Git subprocess uses `GIT_NO_LAZY_FETCH=1`;
   hold the ADR-034 containment boundary for reads and candidate writes.
3. Add the missing tree-hash primitive and candidate-namespace worktree
   create/prune, without reusing the shadow APIs or changing live Git history.
4. New advisory-lock abstraction: nonblocking TryAcquire/Release, supported
   on Linux/macOS/Windows, filesystem allowlist with unknown/remote refusal,
   no PID/heartbeat/stale-breaking, inert lock-file residue. Other targets
   refuse without filesystem mutation. Windows uses stdlib, not x/sys.
5. Preserve feature-before-global lock order and locked candidate mutations;
   failed acquisition releases earlier holds. Scope is primitives, not later
   acceptance or candidate-state implementation.
   Per ADR-044, create/prune borrow mandatory live feature/global handles;
   they never reacquire or release caller-owned locks.
6. Add focused positive/negative runtime and inventory tests, including native
   Windows contention/release/crash behavior in a required CI path.

S2 eligibility and config flags, S3 alignment, candidate execution/schemas,
phase/CLI wiring and publication/acceptance/recovery are not part of S1.
Public SPEC/assets remain unchanged. Contract changes are limited to the
explicitly accepted ADR-044 ownership/signature clarification.

## S0 preservation and scope transition

S0 remains accepted. Keep all frozen goldens and behavior assertions unchanged,
including phase-2 legacy rejection and candidate expectations marked future-only.
Transition the stage-specific no-production guard explicitly to narrow S1
surfaces; do not expand it to arbitrary internal files. Golden capture must
continue checking the original production baseline. Preserve the prior
exact-twelve-line cache-lifetime exception and its negative controls.

## Files Changed

Tracking: CURRENT, HISTORY, ROADMAP and supervisor LOG.
First implementation: `internal/candidate/{root,root_test,source_guard_test}.go`,
new `internal/candidatelock/` platform implementations/tests,
`internal/gitutil/tree_hash{,_test}.go`,
`internal/workflow/operation_candidate_s0_guards_test.go`, and
`.github/workflows/ci.yml` required Windows lock step.
Revision adds candidate read flags/Unix tests, platform filesystem/Unix lock
safety tests, and `internal/cli/prepare_s7_platform_guard_test.go` native-CI
registration controls. It also updates S0 capture routing in
`internal/workflow/operation_candidate_s0_test.go` without changing goldens.
Recovery also adds `internal/gitutil/hardened_env.go` and updates the directly
dependent `internal/cli/prepare_avp2_test.go` exact root inventory.
The approved documentation amendment adds ADR-044 and changes only the
enumerated ADR-037/PRD clauses; the parent owns those contract/tracking files.
Only one implementation context edits these surfaces; explicit file staging
and the required Copilot co-author trailer remain mandatory.

## Test Results

- S0: independently approved, full local 8/8 gate and native CI passed;
  closing-tip CI `39735631640` passed at `6ab3929`.
- Research intake: `40834229022` passed at `7231514`; subsequent intake
  decision changes are tracking-only.
- First-pass focused candidate/lock/tree/S0-scope checks and CLI build pass.
  Windows candidate/lock test binaries cross-compile. Native runtime CI has
  not run; cross-compilation is not evidence of Windows lock correctness.
- Earlier failed implementation iterations are retained in the admitted runner
  logs. Parent review has six bounded findings; focused passes do not close them.
- Revision passes focused candidate/lock, tree-hash, S0/native-CI guard tests,
  vet, CLI build and Windows cross-compilation. Existing integration guards
  and independent review still need to confirm the correction.
- Independent review of `f5df785`: NEEDS REVISION (two high, two medium findings).
  Focused passes do not establish the missing lifecycle/identity invariants.
- Parent S0/CI integration: admission timeout after 600.72s, before Go starts.
  No integration pass or assertion failure is claimed.
- ADR-044 proposal review is APPROVED; the contract amendment is accepted.
  A non-Go comparison confirms all nine parity blocks and matrix row identities
  are unchanged. Implementation validation remains separate.
- Recovered clean integration at `9198c0a`: PASS, stage
  `s1-recovered-clean-integration-20261029`, all five selected packages.
  Admission 128.54s, command 33.24s; no resource stop. The later shared-state
  correction remains to be validated. All earlier failure evidence is retained.
- Copy-lifetime negative control: the new regression fails at pre-fix `9198c0a`
  with "a handle copy closed a borrowed descriptor". This isolated negative
  disables test vet only to observe the runtime failure despite the old
  copied-mutex warning; ordinary validation retains vet. Stage
  `s1-handle-copy-negative-20261029` preserves the expected failure.
- Corrected `f6bae35`: clean `gofmt -l .`; all tests in candidate (1.644s),
  candidatelock (0.167s), gitutil (10.689s), workflow (111.458s) pass, uncached
  and serial. Evidence: `s1-capability-format-check-20261029`,
  `s1-capability-affected-packages-20261029`; command 130.71s, no resource stop.
- Remaining local checks at `f6bae35` all pass: CLI integration (3.715s),
  race candidate/locks (2.808s/1.200s), vet, CLI build, Windows candidate/lock
  cross-compilation. Evidence stages `s1-capability-{cli-integration,race,vet,
  build,windows-compile}-20261029`; each freshly admitted, no resource stops.
  Cross-compilation is not native Windows execution.
- Independent code review of `f6bae35`: NEEDS REVISION for one medium
  inherited `GIT_CONFIG_PARAMETERS` gap; prior lifetime/cleanup/guard findings
  closed. Retained reviewer runtime reconfirmed OpenAI `gpt-5.3-codex`.
- `s1-config-parameters-negative-20261029`: both real subprocess envelope
  regressions fail at the intended unsafe-variable assertion with the pre-fix
  helper. The inherited input is an inert `core.autocrlf=false` setting; no
  injected command is needed to prove the guards detect the gap.
- Corrected `3054b84`: formatting clean; candidate (1.619s), gitutil (10.322s)
  and targeted S0/CI workflow/CLI integration (4.748s/3.704s) pass.
  Stages `s1-config-parameters-{format-check,packages,integration}-20261029`;
  each freshly admitted, no golden capture or resource stop.
- Updated vet and CLI build at `3054b84` pass in stages
  `s1-config-parameters-{vet,build}-20261029`, without resource stops.
- Independent residual review APPROVED at `3054b84`; retained OpenAI
  `gpt-5.3-codex` runtime confirmed. Together with the prior explicit closures
  this leaves no open code finding, but native/full validation is still pending.
- Full gate running: shell `roc-s1-full-gate-ddc59b7`, runner
  `bin/roc-s1-20261029/full-gate/run-gate.py`, evidence in its `evidence/`
  directory. Fresh isolated source/caches, original shard hash
  `7e9bebd164e73bdbadd4e8363b6be31d974b1e4133b6f086fc1c84d218eb6f10`,
  all nine wrapper routing controls pass; golden capture explicitly disabled.
  Checks 1-5 pass, not a complete gate result.
- Native CI running: https://github.com/tesseracode/tesserapatch/actions/runs/40917207659
  at `ddc59b7`. Watch shell `roc-s1-native-ci-ddc59b7`; output retained at
  `bin/roc-s1-20261029/native-ci-ddc59b7.log`. No native PASS claim yet.
- The initial CI watcher exited on `error connecting to api.github.com`,
  not a failed job. A fresh API query confirms all five jobs remain running
  with no failed step reported. Monitoring resumes in shell
  `roc-s1-native-ci-ddc59b7-retry1`, log
  `bin/roc-s1-20261029/native-ci-ddc59b7-retry1.log`; the original log and
  unchanged CI run are preserved. The full local gate continues separately.
- Final local gate at `ddc59b7`: RESOURCE-BLOCKED in stage `04-go` (the first
  broad test shard), after 809.63s. Checks 1-7 pass; 3/25 top-level commands
  and 0/22 complete test commands pass. Captured package output passes assets,
  buildinfo, candidate and candidatelock before the CLI package is interrupted;
  stderr is empty. Peak sampled owned RSS 3,850,108,928 bytes; pressure changed
  from normal to warning. No external Go was sampled, swap-out delta is zero,
  historical swap occupancy remains, and owned survivors are zero.
  Source remains clean and v0.17.0 unchanged. The raw failed-run evidence stays
  in `bin/roc-s1-20261029/full-gate/evidence/`; do not reuse/overwrite it.
- Native Windows job `123600508248` is successful. Its saved log
  `bin/roc-s1-20261029/native-windows-ddc59b7.log` contains actual
  `TestCandidateLockNativeWindows/CLW-001` through `CLW-005` PASS lines,
  including crash release and borrowed/copy release exclusion.
  Ubuntu observers also pass; remaining CI jobs were still running at this
  checkpoint. The whole CI run is not yet claimed green.
- Final CI result: `40917207659` SUCCESS at `ddc59b7`, all five required jobs
  successful with no failed steps; release skipped. The watcher completed
  successfully. Full local validation remains incomplete because of its
  resource stop, regardless of this native success.

## Next Steps

1. Publish the reviewed, pilot-validated proposal for required native CI, then
   run the full admitted gate at that exact checkpoint. The pilot sequence has
   exited with no owned survivors; do not rerun the completed pilots separately.
2. Complete all 39 shards, formatting, vet/build and native validation before
   S1 acceptance. The fifteen extracted family shards still need execution
   in the complete partition; six successful pilots are not the full gate.
3. Keep WAVE_BASE `5120dc9`, unchanged admission/stop policy and all failed-run
   evidence. Publish only a reviewed validation checkpoint; no S1 acceptance
   until the complete gate succeeds, and no automatic S2 dispatch.

## Validation Policy — GH #13

On the current 32 GiB Mac, require 60 continuous seconds of normal OS pressure,
no new swap-outs and load1 <=5 before every Go validation command. External
Go is observed, not vetoed; own work is serial (`GOMAXPROCS=1`, `-p=1`).
Percentage is diagnostic; historical occupied swap alone is not a veto.
Stop on warning/critical pressure, new swap-outs, invalid/missing telemetry
or gaps over 5s. Use 600s admission waits and preserve test selectors/timeouts.
Use isolated full-gate source/caches, reconcile owned processes and retain logs.

## Blockers

No open S1 production-code finding; its required native CI passes.
The newer test-only proposal `31fd46e` has static approval and all six memory
pilots pass without runtime pressure or new swap-outs. Its full 39-command/
native validation remains pending, so S1 is not accepted.
Do not dispatch a later slice or claim acceptance until they complete.

## Context for Next Agent

Backlog item `research-upstream-git-strategies` remains in ROADMAP. Its clarified
scenario is long-lived library/SDK compatibility lines with independent patch/
security releases. Public GitLab/GitVersion sources support that mechanism;
the exact "Lib Flow" name is unverified. This research is not an S1 prerequisite.

Preserve all S0/memory-fix evidence, no-Unreleased changelog constraint and
v0.17.0 tag. Do not infer permission to change Git branch strategy or use a
central external patch-store backend from the research proposal.
