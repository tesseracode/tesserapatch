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
- **Status**: In progress — accepted-contract implementation preflight
- **Assigned**: 2026-10-29
- **WAVE_BASE**: `5120dc9395769fe8148b61836e546729b66cb4dc`
- **Contract**: accepted ADR-037/PRD rev-7; PRD §8 S1, §6.9, §6.13
- **Prior accepted S0 close**: `6ab392996467662572b155574a0509a6269cae8f`

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
No S1 production changes or new validation results are claimed yet.

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

Dispatch tracking only so far: CURRENT, HISTORY, ROADMAP and supervisor LOG.
S1 code is planned in new internal candidate/lock files plus a bounded Git tree
primitive, guard tests and directly necessary native-CI test registration.
Only one implementation context edits these surfaces; explicit file staging
and the required Copilot co-author trailer remain mandatory.

## Test Results

- S0: independently approved, full local 8/8 gate and native CI passed;
  closing-tip CI `39735631640` passed at `6ab3929`.
- Research intake: `40834229022` passed at `7231514`; subsequent intake
  decision changes are tracking-only.
- No new S1 tests, builds or runtime-support claims yet.

## Next Steps

1. Implement and test the accepted S1 primitives without later-slice scope.
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

None confirmed at dispatch. If contract ambiguities affect root ownership,
filesystem safety or lock lifetime, record them and ask before changing a
locked architectural decision.

## Context for Next Agent

Backlog item `research-upstream-git-strategies` remains in ROADMAP. Its clarified
scenario is long-lived library/SDK compatibility lines with independent patch/
security releases. Public GitLab/GitVersion sources support that mechanism;
the exact "Lib Flow" name is unverified. This research is not an S1 prerequisite.

Preserve all S0/memory-fix evidence, no-Unreleased changelog constraint and
v0.17.0 tag. Do not infer permission to change Git branch strategy or use a
central external patch-store backend from the research proposal.
