# Current Handoff

## Status

**Cluster state**: IN PROGRESS

GH #13 S0 is dispatched from the clean, pushed, native-green `bbb7a43`
baseline. S0 freezes existing reconciliation behavior and candidate expectation
fixtures before any production edit. S1 and later slices are not dispatched.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s0`
- **Milestone**: GH #13 / ADR-037 and ADR-043
- **Description**: S0 — frozen reconcile, evidence, confirmation, CLI and legacy baselines
- **Status**: In progress — inventorying existing fixtures and golden helpers
- **Assigned**: 2026-10-11
- **WAVE_BASE**: `bbb7a4310b2ebea095890a284997fbbb42535bc7`
- **Contract**: accepted ADR-037/ADR-043 and PRD rev-7, especially PRD §8 S0

WAVE_BASE = bbb7a4310b2ebea095890a284997fbbb42535bc7

## Session Summary

The operator approved recording #13's admission policy and starting S0.
Fresh verification finds HEAD equal to fetched origin/main at `bbb7a43`,
no tracked/untracked changes, no stash and one main worktree. Latest native
CI `38170635445` is SUCCESS at that exact closing commit.
The completed reapply-correction handoff is archived unchanged in HISTORY.

Read the accepted rev-7 plan and its no-production-before-S0 boundary.
Historical "implementation undispatched" wording in frozen planning documents
is superseded by this dated live dispatch, not silently rewritten in the
accepted contract. No production or public-contract change has been made.

## Current State

S0 must cover the following before review:

1. Current phase outcomes and exact operation-level notes.
2. Gate-off session JSON and evidence JSONL for phases 1, 1.5, 2 all-present,
   3, 3.5 and 4, with complete byte-level golden checks.
3. Existing confirmation-gate behavior: phase-1 confirmation, phase-3 rejection
   and phase-2 rejection; keep future gate-on expectations explicitly distinct.
4. ReconcileSummary serialization with no candidate.
5. Existing accept/reject/shadow-diff output and reconcile flag-error bytes.
6. Multi-slug and cumulative-legacy baseline behavior.
7. Both adjacent-CLI-argument fixtures, with their required future candidate
   success outputs preserving upstream deletions and the surviving old argument.
8. Mutation-sensitive regressions and scope guards proving that protected
   baseline changes fail, without shipping candidate runtime behavior.

Reuse existing helpers and deterministic fixture conventions. Record any
necessary normalization explicitly; do not erase semantic evidence fields,
weaken assertions or claim future candidate execution already works.

## Validation Policy — GH #13 Local Work

The operator-approved pressure-based policy is now explicit for #13 validation
on the current 32 GiB Mac. Carry it forward in #13 slice handoffs; it is not
a claim that the older Mac is qualified and does not change the profiler's
default CLI flags or the checked-in hosted-CI commands.

- Before every Go validation command, require sixty continuous seconds of
  normal OS pressure, no new swap-outs and load1 <=5.
- External Go is permitted and observed, not a machine-wide veto. Our own
  Go workload remains serial (`GOMAXPROCS=1`, package parallelism `-p=1`).
- The memory percentage is diagnostic only. Historical occupied swap alone
  is not a veto; no additional-swap tolerance is enabled.
- Stop owned work on warning/critical pressure, new swap-outs, missing/invalid
  telemetry or a sampling gap exceeding five seconds. Load is an admission
  condition, not a runtime cap.
- Use the existing 600-second per-command admission wait and stop on the
  first validation/admission/resource failure. Preserve logs and exact cause.
- Preserve canonical test selectors/order and timeouts. Use focused checks
  first, affected packages next, then required full validation and wave closure.
- Isolate source, output and caches for broad runs; reconcile owned process
  identities afterward. Never terminate another session to manufacture admission.

## Files Changed

Dispatch tracking only: CURRENT, HISTORY, ROADMAP and the supervisor log.
S0 implementation is limited to test helpers, test fixtures/goldens and tracking.
Production Go, SPEC, assets, accepted ADR/PRD semantics, dependencies, CI/shards
and CHANGELOG remain untouched.

## Test Results

- Prior latest-tip native CI `38170635445`: SUCCESS at `bbb7a43`.
- Prior correction: full 8/8 gate and native CI at `608714b`, with all
  25 top-level commands / 22 test commands passed.
- No new S0 Go validation or fixture-generation result is claimed yet.

## Next Steps

1. Inventory current tests, normalization helpers and exact PRD S0 obligations.
2. Build frozen runtime/CLI baselines and both motivating fixture variants.
3. Prove regression sensitivity and no production replay surface was added.
4. Validate, review and close S0 with the explicit wave base; do not start S1
   until S0 is accepted and a subsequent slice is assigned.

## Blockers

None at dispatch. Any unresolved fixture/contract ambiguity must be recorded
before implementation choices are made.

## Context for Next Agent

The feature's config gate, proof carrier, locks, candidate schemas, alignment,
publication and acceptance logic belong to later slices, not S0. Keep default
runtime behavior unchanged. Phase-2 legacy all-present is not confirmed today;
confidence labels and stored metadata cannot substitute for future typed proof.
Candidate persistence must eventually wait for real phase-4 conflict, but S0
only pins the required expectations and current baseline.

Preserve prior raw evidence and fixes, including Git index mtime preservation.
No release/tag is authorized. The assets parity guard rejects an Unreleased
heading beside its selected versioned changelog contract; do not add one.
Stage explicit file paths and retain the required Copilot co-author trailer.
