# Current Handoff

## Status

**Cluster state**: IN PROGRESS

GH #13 S1 is dispatched after accepted S0. The upstream Git-strategy research
remains queued; the operator selected backlog-only research and S1 next.
No strategy policy, full research deliverable or research cluster was adopted.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s1`
- **Milestone**: GH #13 / ADR-037 and ADR-043
- **Description**: S1 — typed roots, advisory locks and missing offline Git primitives
- **Status**: Recovering directly from surviving ADR-044 edits after agent connection failure
- **Assigned**: 2026-10-29
- **WAVE_BASE**: `5120dc9395769fe8148b61836e546729b66cb4dc`
- **Contract**: ADR-037/PRD rev-7 as narrowly amended by accepted ADR-044; PRD §8 S1, §6.9, §6.13
- **Prior accepted S0 close**: `6ab392996467662572b155574a0509a6269cae8f`
- **Unaccepted first implementation**: `252e4940f04b183a90c0a6a2f6ef7cf4efeb8888`
- **Revised review checkpoint**: `f5df7852a39a476808a9146f65970a809b7ea44c`
- **Accepted amendment checkpoint**: `8b22f99263827a53b5b66fb534e86921563a5d9f`

WAVE_BASE = 5120dc9395769fe8148b61836e546729b66cb4dc

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
Implementation is assigned to `roc-s1-implementation`,
agent `6c67ce6f-1058-4f46-bd48-8c663eafe87c`, with OpenAI `gpt-5.3-codex`
explicitly requested and now confirmed by runtime metadata.
It owns code/tests and necessary native-CI registration, not tracking or Git
commits. Use this existing agent for same-task follow-up; do not duplicate work.
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
Independent review is now active under agent
`e7b49e9c-874f-4fa0-935b-852cf6a0ec1e` against frozen `f5df785`,
with OpenAI `gpt-5.3-codex` explicitly selected. Parent integration shell
`roc-s1-parent-integration` is waiting for resource admission, not yet running
Go. Its saved stage is `s1-parent-s0-ci-integration`. Do not start duplicate
work or interpret the admission wait as a test result.
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
The existing implementer `6c67ce6f-1058-4f46-bd48-8c663eafe87c` is now
applying that contract and the outstanding cleanup/environment/source-guard
corrections. Preserve its work and await its result; do not start a duplicate
implementation or parent Go workload while it owns admitted checks.
The implementer has now failed on model-request network timeouts. Its partial
code edits survived and no validation process remains active. The parent resumes
directly without reset/cleanup or another implementation agent.
Last admitted stage `s1r3-gofmt-20261029a` failed on misplaced declarations;
repair that syntax before any new test run. No post-amendment pass is claimed.

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

## Next Steps

1. Implement accepted ADR-044 with the remaining cleanup, tree-environment
   and source-guard findings, using the existing implementer/admitted runner.
2. Run focused admitted checks, required native-platform coverage, affected
   tests and full closure validation; review independently before acceptance.
3. Update tracking at each transition; do not dispatch S2 automatically.

## Validation Policy — GH #13

On the current 32 GiB Mac, require 60 continuous seconds of normal OS pressure,
no new swap-outs and load1 <=5 before every Go validation command. External
Go is observed, not vetoed; own work is serial (`GOMAXPROCS=1`, `-p=1`).
Percentage is diagnostic; historical occupied swap alone is not a veto.
Stop on warning/critical pressure, new swap-outs, invalid/missing telemetry
or gaps over 5s. Use 600s admission waits and preserve test selectors/timeouts.
Use isolated full-gate source/caches, reconcile owned processes and retain logs.

## Blockers

ADR-044 is accepted. Its implementation and the remaining code findings
still block S1.
No publication/acceptance or later slice until corrections, independent review
and native runtime validation complete.

## Context for Next Agent

Backlog item `research-upstream-git-strategies` remains in ROADMAP. Its clarified
scenario is long-lived library/SDK compatibility lines with independent patch/
security releases. Public GitLab/GitVersion sources support that mechanism;
the exact "Lib Flow" name is unverified. This research is not an S1 prerequisite.

Preserve all S0/memory-fix evidence, no-Unreleased changelog constraint and
v0.17.0 tag. Do not infer permission to change Git branch strategy or use a
central external patch-store backend from the research proposal.
