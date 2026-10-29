# Current Handoff

## Status

**Cluster state**: IDLE

Documentation-only intake is complete; the research idea is queued in ROADMAP.
S0 remains accepted at closing commit `6ab3929`; S1 is paused for the
operator's requested scheduling decision, not blocked by a demonstrated
technical dependency on this research.

## Active Task

- **Task ID**: `research-upstream-git-strategies-intake`
- **Milestone**: Paper-only backlog intake, before the S1 scheduling decision
- **Description**: Capture research on long-lived downstream patches, upstream bases and Git branch history
- **Status**: Intake complete — awaiting operator sequencing decision
- **Assigned**: 2026-10-29
- **Intake base**: `6ab392996467662572b155574a0509a6269cae8f`
- **Accepted S0 WAVE_BASE**: `bbb7a4310b2ebea095890a284997fbbb42535bc7`

## Session Summary

The operator requests a durable investigation proposal, then a pause to choose
between a full research deliverable before S1 and backlog-only deferral with
S1 next. The task does not authorize starting either branch of that choice.
The starting checkout is clean, main equals fetched origin/main at `6ab3929`.
The prior accepted S0 handoff is archived verbatim in HISTORY.

Prior Copilot CLI sessions confirm a bounded paper-intake-first practice:
record evidence/status and separate exploration from implementation. The
whitepaper protocol reserves ADRs for decisions, not untested hypotheses.
This item will live as a source-backed research-backlog brief in ROADMAP.
No full whitepaper, new cluster, ADR or implementation is being dispatched.
The durable item is `research-upstream-git-strategies` in
[ROADMAP](../ROADMAP.md#research-backlog--upstream-git-and-patch-management-strategies--2026-10-29).
It contains current-state anchors, a topology/release matrix, verified
prior-art leads, candidate strategies, proposed deliverables and two sequencing
options. S1 has not started.

## Current State

The research must distinguish rewriting Git commits/refs from updating
tpatch's canonical patch/generation artifacts. It must not assume every
reconcile rewrites Git history or every parent/meta-repo layout provides an
independent external patch store. Verify capabilities from current code and
locked contracts; preserve the operator's observations as problem reports.

Selected primary sources cover Quilt, StGit, TopGit, gbp pq, Git workflows,
git-flow, GitHub flow, Jujutsu's operation log and Git range-diff. This is
orientation, not a complete survey or an adopted default. The gbp pq distinction
between durable patch history and disposable queue history is a promising lead,
not a selected architecture. "lib-flow" still needs an exact reference.

No independent central-store/meta-repo backend was established: reviewed
`Store.Root` and `--path` behavior couples metadata to the target workspace.
Research separately the meta-repo orchestrator, per-target stores, vendored
content/submodules and a genuinely external shared store. WP-006 keeps
tpatch-native/non-Git version control exploratory.

## Files Changed

Tracking only: ROADMAP, CURRENT, HISTORY and supervisor LOG.
Production/tests, S0 goldens and guard logic, public SPEC/assets, accepted
ADRs/PRDs, dependencies, CHANGELOG and Git branch/tag policy are unchanged.

## Test Results

No Go workload is needed for this tracking-only intake. Prior S0 acceptance,
gate/CI results and evidence remain unchanged; this is not a new gate pass.

## Next Steps

1. Pause for the operator to choose full paper research before S1, with later
   explicit cluster triage, or keep the proposal queued and assign S1 next.
2. Clarify the intended "lib-flow" project when a reference is available;
   do not treat the unresolved name as a research result.
3. Do not start either path, adopt a branching strategy or create an ADR
   without that scheduling decision.

## Validation Policy — carry forward for GH #13

On the current 32 GiB Mac, require 60 continuous seconds of normal OS pressure,
no new swap-outs and load1 <=5 before every Go validation command. External
Go is observed, not vetoed; own work is serial (`GOMAXPROCS=1`, `-p=1`).
Percentage is diagnostic; historical occupied swap alone is not a veto.
Stop on warning/critical pressure, new swap-outs, invalid/missing telemetry
or gaps over 5s; use 600s admission waits and preserve the original partition/
timeouts. No such workload is scheduled for this intake.

## Blockers

No technical blocker at intake. The next activity requires the requested
operator decision. The exact reference meant by "lib-flow" is not yet identified;
record that uncertainty rather than silently substituting another workflow.

## Context for Next Agent

S0 is complete, not reopened. Preserve prior measurement evidence and the
v0.17.0 tag. If S1 is selected later, fetch and record its actual pre-dispatch
wave base; the old suggested `6ab3929` will no longer be the latest commit
after this intake. Do not create an Unreleased changelog entry.

S0's scope guard currently allows only its explicit test files and tracking
documents. A future standalone research paper/index or S1 production work
requires an explicit, reviewed scope transition; do not quietly expand the
guard for this documentation-only proposal.
