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
- **Status**: Authorized test-only six-loop cache-lifetime fix in progress
- **Assigned**: 2026-10-11
- **WAVE_BASE**: `bbb7a4310b2ebea095890a284997fbbb42535bc7`
- **Contract**: accepted ADR-037/ADR-043 and PRD rev-7, especially PRD §8 S0
- **Full-validation checkpoint**: `e8a154bd954815cefccdacd160b70f183ead4eb4`
- **Pre-maintenance checkpoint**: `89e481c0ee882c1c9668f8b8d2296a48acaa4eed`
- **Memo-lifetime checkpoint**: `4e56e8559ec305f40ad18690eec8353412c546d2`

WAVE_BASE = bbb7a4310b2ebea095890a284997fbbb42535bc7

## Session Summary

**2026-10-21 update**: the operator authorizes the recommended bounded test-only
cache-lifetime fix. Fresh baseline is clean/pushed `89e481c`, no stash, one
worktree; native CI `38218488185` passes. Add only existing four-fixture scope
registrations to the six AR rev-21/23/24 roots/callbacks. Preserve every
assertion and the helper's existing lifetime contract.

S0's old-test-file exception must verify an exact twelve-line insertion against
the frozen baseline, not expand its generic allowlist. Add sensitivity controls
for missing/extra registrations, unrelated edits, assertion drift and parallel
execution. Production, goldens, analyzer internals, helper implementation,
canonical grouping/timeouts and pressure safeguards remain untouched.
Validate the exact six-root/85-leaf group and report runtime/memory tradeoffs
before full closure. The original S0 wave base remains `bbb7a43`.

The twelve registrations are implemented using the existing rev-19/rev-20
pattern. Each new validator builds and consumes its model locally, returns
only an error, and is invoked once per nonparallel child fixture. The helper
clears after four completed children and at root cleanup, not during active
analysis. Source comparison proves all other old analyzer-test bytes unchanged.
The S0 exception derives those exact insertions from the pinned baseline and
rejects missing/extra scope calls, parallelism, assertion drift and unrelated
edits. Formatting and focused S0/cache-lifecycle controls pass
(workflow 3.783s, CLI 52.089s, store 0.161s). The exact heavy group's
coverage/memory run and independent follow-up review are next.
The correction is frozen at `4e56e85`; request a same-S0 follow-up review of
the exact maintenance delta and run the unchanged group with all 91 expected
RUN/PASS paths (six roots plus 85 leaves), fresh caches and original safeguards.
Do not claim a controlled speed ratio against the old interrupted probe.

The operator approved recording #13's admission policy and starting S0.
Fresh verification finds HEAD equal to fetched origin/main at `bbb7a43`,
no tracked/untracked changes, no stash and one main worktree. Latest native
CI `38170635445` is SUCCESS at that exact closing commit.
The completed reapply-correction handoff is archived unchanged in HISTORY.

Read the accepted rev-7 plan and its no-production-before-S0 boundary.
Historical "implementation undispatched" wording in frozen planning documents
is superseded by this dated live dispatch, not silently rewritten in the
accepted contract. No production or public-contract change has been made.

Inventory identifies reusable reverse-apply/legacy-operation/conflict/novelty
fixtures, the existing scripted provider and real Cobra test runner.
New S0 scaffolding covers thirteen runtime cases, exact legacy CLI terminal/
error bytes, no-candidate summary serialization, source-scope and comparison
sensitivity, and both motivating candidate expectation variants.
All thirteen runtime goldens are captured, and normal S0 comparisons plus
sensitivity tests pass across workflow/CLI/store, including a two-repeat run.
The phase-3.5 fixture now uses the semantic provider's actual `decision` field.
Its recapture and the final two-repeat comparison pass. Source and public
contracts remain byte-identical to the baseline outside test/tracking files.
Implementation checkpoint: `7d7febde8afff602064d6e31a156d139ab14ccc8`.
Review is bounded to `bbb7a43..7d7febd`, using an explicitly selected OpenAI
reviewer. The reviewer must remain read-only and run no unadmitted Go commands.
Reviewer `e17d29e2-bd48-4003-80a9-c0b61112dc09` (runtime-confirmed `gpt-5.4`)
returns NEEDS REVISION: restrict capture scope to explicit S0 files so shared
fixture helpers cannot drift; distinguish the applicable-only/mixed/plain
fixture inputs with explicit classification assertions and mutation controls.
Both corrections are implemented: the allowlist names only the five S0 test
files plus this fixture directory/tracking; existing shared builders are denied.
Runtime fixtures now assert recipe presence and real evaluator counts before
fallthrough, with missing-recipe/dropped-operation negative controls.
Goldens and production code are unchanged. Reformatting and a full two-repeat
S0 comparison pass (workflow 6.625s, CLI 1.869s, store 0.183s). Return the
bounded fixes to the same reviewer before final acceptance.
Follow-up review at `cb96eb5a7710941b0d5643e45ac52e10f7ad4686` is APPROVED
with both findings closed and no residual notes. The reviewer remains
runtime-confirmed `gpt-5.4`. Updated-source vet/build pass; final full gate
and native CI remain pending.
The complete gate at `e8a154b` is stopped: checks 1-7 and four test commands
pass, then warning-level OS pressure stops `TestS7ARRev(21|23|24)`.
No assertion failure is reported; zero swapping persists. The controller
terminates one observed owned git-daemon during reconciliation and verifies
zero remaining owned processes. Source and prior evidence remain unchanged.
Native `38189486586` is now SUCCESS, with all five required jobs passed.
The operator approves stopping idle Docker Desktop after a zero-active-container
check, then retrying the full local gate. Shutdown is verified; post-stop
pressure is normal, load1 2.97, swap zero and no Go tools active.
Retry the same pushed/native-green `e8a154b` source with new caches/evidence;
do not weaken safeguards or count the first attempt as complete.
The retry is complete with the same resource stop: checks 1-7 and 4/22 test
commands pass, then warning pressure stops AR rev-21/23/24. No assertion
failure or swapping is reported. The 1933.22s run records 959 samples and
peak sampled owned RSS of 9,842,114,560 bytes; the interrupted command's final
sample is 7,914,389,504 bytes. The controller explicitly terminates one retained
owned git-daemon and verifies zero remaining owned processes.
Docker shutdown was not sufficient to complete the local gate. Do not start
another identical full retry without addressing the resource problem.
The operator now authorizes a read-only heavy-shard investigation and at most
one monitored isolated probe. Static inspection finds six roots/85 table
fixtures with whole-package mutated type models and no scoped memo cleanup.
The same group already peaked around 9.59/10.20 GiB in earlier passing gates;
its analyzer/cache code is unchanged. A single unchanged group run with JSON
events can correlate fixture progress and memory samples. No code/grouping/
timeout/safeguard change is authorized, and no full-gate success is implied.

The one authorized probe is now complete: it also stops on warning pressure,
without any preceding suite commands. Five test roots and 83/85 leaf fixtures
pass; rev-21 static `scheduled-loop-limit-fails-closed` is active at the stop,
and `scheduled-state-cap-fails-closed` never starts. No assertion failure,
package pass or full coverage is reported. Peak sampled RSS is about 10.08 GiB,
growing across roots from about 2.78 GiB; this is consistent with the unscoped
cache retention found statically, not exact per-cache heap attribution.
Swap stays zero, source stays clean and no sampled owned processes remain.
The private timestamp correlator needed a fractional-second parsing correction;
only retained evidence was reanalyzed, with no second probe.

The legacy gate-on expectation is not a new confirmation: the legacy fixture
has no eligible coverage and must fall through. A recipe-absent test-only
oracle pins its later phase's outcome; it does not implement the future gate.

## Current State

The reviewed S0 implementation covers:

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

Tracking: CURRENT, HISTORY, ROADMAP and the supervisor log.
New test files: `internal/workflow/operation_candidate_s0_test.go`,
`operation_candidate_s0_guards_test.go`, `operation_candidate_s0_fixtures_test.go`
in the same package, and `internal/cli/operation_candidate_s0_test.go` /
`internal/store/operation_candidate_s0_test.go`.
Fixture data/documentation: `internal/workflow/testdata/roc-s0/`.
Private monitored command runner: ignored `bin/roc-s0-20261011/`.
S0 implementation remains limited to test helpers, fixtures/goldens and tracking.
The authorized maintenance exception touches only
`internal/cli/prepare_s7_ar_guard_test.go` (six scope pairs) and
`internal/workflow/operation_candidate_s0_guards_test.go` (exact-delta proof/
sensitivity controls), plus directly related fixture README/tracking.
Production Go, SPEC, assets, accepted ADR/PRD semantics, dependencies, CI/shards
and CHANGELOG remain untouched.

## Test Results

- Prior latest-tip native CI `38170635445`: SUCCESS at `bbb7a43`.
- Prior correction: full 8/8 gate and native CI at `608714b`, with all
  25 top-level commands / 22 test commands passed.
- Initial formatting stopped on a misplaced new test declaration, before any
  tests ran. Its scope is corrected and formatting passes.
- Corrected formatting passes. Initial capture records twelve of thirteen
  runtime fixtures, then fails because the intended clean-threeway fixture
  used adjacent conflicting edits. Separate those edits while retaining the
  context mismatch; preserve the expected clean-threeway outcome and retry.
  That partial capture was superseded by the passing comparisons below.
- The separated clean-threeway fixture captures successfully. Full normal
  comparisons pass (workflow 2.908s, CLI 1.107s, store 0.158s), then all S0
  tests pass twice (5.999s / 1.954s / 0.442s), including decoder controls.
- Final valid-provider fixture capture and normal `-count=2` S0 comparison
  pass: workflow 5.580s, CLI 1.761s, store 0.069s. No golden-update environment
  is enabled during comparison.
- Full workflow/store packages pass (117.346s/2.766s), and vet/build pass.
  The initial review's two findings were addressed and closed below.
- Both review corrections pass all S0 checks twice (6.625s/1.869s/0.183s)
  with ordinary golden comparison. Follow-up review is APPROVED.
- Updated-source vet and CLI build pass; all nine routing controls pass.
- The full gate at `e8a154b` passes checks 1-7 and 4/22 test commands,
  then stops on warning pressure during command five. No assertion failure.
  Across 986 samples swap stays zero; 268 owned identities are reconciled,
  one owned git-daemon is terminated and none remain.
- Native `38189486586`: SUCCESS at `e8a154b`, all five required jobs pass,
  release skipped.
- The approved retry after Docker shutdown again passes checks 1-7 and
  4/22 test commands, then stops on warning pressure in AR rev-21/23/24.
  No assertion failure or swap; 236 owned identities reconciled, one retained
  git-daemon explicitly terminated, zero remaining processes.
- The single isolated unchanged-shard probe stops at warning pressure in
  216.92s total (60.22s admission); five roots/83 leaves pass, zero assertions
  fail, and two leaves are incomplete/unstarted. Peak sampled RSS 10.08 GiB;
  zero swap, no owned processes remain. It is not a passing shard certificate.
- Authorized six-loop fix: formatting and S0/existing cache controls pass
  (workflow 3.783s, CLI 52.089s, store 0.161s). Heavy-group/full-gate results
  remain unverified; no memory-reduction claim yet.

## Next Steps

1. Preserve the completed native CI result and the original resource-stop
   evidence at `bin/roc-s0-20261011/final-gate/evidence/`.
2. Preserve the second stopped run at
   `bin/roc-s0-20261011/final-gate-retry/evidence/`. No local Go run remains.
   The authorized investigation/probe is complete; its evidence is at
   `bin/roc-s0-20261011/heavy-shard-probe/evidence/`.
3. Implement the now-authorized bounded lifetime correction for the six loops.
   Reuse the unchanged helper after confirming independent lifetimes. Keep the
   exact-delta S0 exception narrow and mutation-sensitive, then validate/review
   before a fresh full gate.
4. Preserve existing failure evidence; do not alter production or accepted
   contracts to make a baseline test pass.
5. Validate, review and close S0 with the explicit wave base; do not start S1
   until S0 is accepted and a subsequent slice is assigned.

## Blockers

No review/native CI findings remain. Both full local attempts stopped on
warning pressure in the fifth test command, including the approved retry after
idle Docker shutdown. Full local validation is incomplete; S1 stays undispatched.
The test-only lifetime fix is now authorized; canonical-shard and safeguard
changes remain forbidden. Full local validation is still incomplete. The new
scope exception/call sites require validation and independent review.

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
