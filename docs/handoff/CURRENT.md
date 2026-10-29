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
- **Status**: Revised checkpoint frozen at f5df785 for independent review/integration checks
- **Assigned**: 2026-10-29
- **WAVE_BASE**: `5120dc9395769fe8148b61836e546729b66cb4dc`
- **Contract**: accepted ADR-037/PRD rev-7; PRD §8 S1, §6.9, §6.13
- **Prior accepted S0 close**: `6ab392996467662572b155574a0509a6269cae8f`
- **Unaccepted first implementation**: `252e4940f04b183a90c0a6a2f6ef7cf4efeb8888`
- **Revised review checkpoint**: `f5df7852a39a476808a9146f65970a809b7ea44c`

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

## Current State — bounded S1 requirements

1. `OpenReferenceRoot(repoRoot string, commit string)` and
   `CreateCandidateRoot(repoRoot, slug, candidateID, commit string)` are the
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
6. Add focused positive/negative runtime and inventory tests, including native
   Windows contention/release/crash behavior in a required CI path.

S2 eligibility and config flags, S3 alignment, candidate execution/schemas,
phase/CLI wiring and publication/acceptance/recovery are not part of S1.
Public SPEC/assets and accepted contract semantics remain unchanged.

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

## Next Steps

1. Independently review the revised snapshot against the six findings and the
   accepted S1 contract; verify existing S0/CI guards before publication.
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

Revision validation and independent review remain outstanding. No S1 acceptance
or later-slice dispatch until actual contract fidelity and native runtime
coverage are established.

## Context for Next Agent

Backlog item `research-upstream-git-strategies` remains in ROADMAP. Its clarified
scenario is long-lived library/SDK compatibility lines with independent patch/
security releases. Public GitLab/GitVersion sources support that mechanism;
the exact "Lib Flow" name is unverified. This research is not an S1 prerequisite.

Preserve all S0/memory-fix evidence, no-Unreleased changelog constraint and
v0.17.0 tag. Do not infer permission to change Git branch strategy or use a
central external patch-store backend from the research proposal.
