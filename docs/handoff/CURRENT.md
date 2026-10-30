# Current Handoff

## Status

**Cluster state**: IN PROGRESS

S4's prior acceptance is REOPENED for typed-root/lock conformance gaps found
during S5 prerequisite inspection. S5 has no implementation and is blocked.
Historical acceptance/validation remains archived truthfully; do not rewrite it.
The autonomous window ends approximately 2026-10-30 18:00 UTC / 11:00 PDT.

## Active Task

- **Task ID**: `repair-operation-replay-candidate-s4-boundary`
- **Milestone**: GH #13 / ADR-037 and ADR-043
- **Description**: Repair typed execution boundary and short global-lock scope
- **Status**: In progress — S4 acceptance reopened before S5
- **Assigned**: 2026-10-30
- **WAVE_BASE**: `78e501c5ef0985a79fca177a888d97eb956de4e6`
- **Contract**: accepted ADR-037/PRD rev-7 and ADR-043 through ADR-046
- **Operator log**: `docs/supervisor/OVERNIGHT-2026-10-29.md`
- **Prior reviewed code**: `1bc18c9cd71d5da2d0732921dce5107c8a25ef30`
- **Prior validated checkpoint**: `ed7efb28c0016615b09363b627a22385448b8869`
- **Prior native CI**: `41039157086`

WAVE_BASE = 78e501c5ef0985a79fca177a888d97eb956de4e6

## Session Summary

Post-close parent inspection found that Execute/executeWithPlan accept
repoRoot strings outside the two allowed constructors, and retain a required
global handle across execution for both create and abandon/prune. These
contradict PRD 6.9/6.13 despite prior green review/tests. The source guard
misses forwarded-root arguments. Restore typed inputs and short global holds;
pin feature capability across mutation. Add real contention/lifetime tests
and a negative guard reproducing the missed old signatures. ADR-047 records
the correction choice; do not weaken the accepted two-constructor contract.
No S5 code or phase wiring is authorized until this repair closes.

The remainder records the prior S4 work/evidence, not approval of the gaps.

S4 followed accepted/pushed S3 closure `5d22acf`. Implementation agent
`26ea48b1-7dd4-4141-a2cc-73aac199ce33` and independent reviewer
`c7fa235b-5753-4d0d-a31b-2a4ec0fdb9fc` both ran OpenAI `gpt-5.3-codex`.
Parent owned tracking and broader/full validation; no concurrent local Go
workloads or shared-file implementer overlap.

Draft `a9e3182` passed focused and basic checks. Independent review required
two corrections: second-pass non-present outcomes incorrectly reached
postcondition-failed, and modeled validation retained all file bodies.
`17f76d1` fixed routing and one-path replay/compare/discard; the actual
reapplication regression was red before and green after. Both findings closed.

Parent then identified ON-006: first-pass postconditions must remain distinct
from second-pass idempotency. `1bc18c9` checks exact created/inserted payloads
at derived final positions before second derivation, including shifts from
lower-offset insertions. Unrelated content changes remain unmodeled-change;
genuine second-pass failures remain not-idempotent. The corrupted-payload
regression was red before and green after. Final independent review approved
this code, preserving both earlier fixes.

ADR-046's initial premature Accepted label was restored to Proposed before
review. Its candidate-rooted snapshot direction was independently accepted:
rooted regular-file identity/size/mode checks plus unchanged tree/diff bind
the mutable candidate; commit-reference snapshots remain immutable. It is
now Accepted after full validation. No temporary commit/ref or new bare-root
constructor was introduced.

## Current State

Closed create-file/insert-by-boundary writer; typed path safety; complete-set
execution and prune-on-failure; exact payload/path/mode/marker/model checks;
same production classifier over every original effect; stable tree/diff
idempotency; total absorbed-present/candidate-applicable effect accounting.
Generation runs no configured tests and does not mutate live repository
content outside the four explicitly expected write sets.

Existing S1 primitives borrow caller-held capabilities, but S4's outer API
does not yet prove the required feature lifetime/short global scope. That is
the repair target. Alignment algorithm/limits and S2 default-off gate are unchanged.

No S5 artifacts/identity/state/status pointers, S6 phase wiring, S7 CLI,
acceptance transaction, release or deferred research was implemented.

## Files Changed

- `internal/candidate/execute.go`, `writer.go`, `validation.go`
- `internal/candidate/execute_test.go`, `execute_guard_test.go`
- `internal/candidate/derivation.go`, `snapshot.go`, `alignment.go`,
  `source_guard_test.go`
- `internal/gitutil/recipe_authority_s0_pi12_test.go`
- `internal/workflow/operation_candidate_s0_guards_test.go`
- `docs/adrs/ADR-046-candidate-execution-boundary.md` and tracking

The alignment.go edit adds diagnostic plumbing, not algorithm/limit changes;
its initially unannounced scope addition was recorded and reviewed.

## Test Results

- Initial draft basic checks pass: affected packages 140.89s, vet 9.54s,
  build 0.51s.
- Routing regression red-before: `s4-revfix-failing-before-1`; corrected
  runtime/guards pass (1.75s/7.55s). Corrected affected/vet/build pass
  (137.03s/5.00s/0.48s).
- ON-006 red-before: `s4-on006-failing-before-2`; corrected runtime/guards
  pass (2.94s/8.30s), covering inserted/created corruption, unrelated bytes,
  shifted offsets and retained genuine non-idempotency.
- Final formatting/candidate package (5.399s)/vet (10.31s)/build (0.61s)
  pass. All nine gate routing controls pass.
- Full gate at `ed7efb2`: 8/8 checks, 42/42 commands, 39/39 uncached test
  commands, exact CI parity. Runtime 5,980.91s; peak sampled owned RSS
  7.38 GiB; normal pressure and zero new swap-outs. All 479 owned identities
  reconciled, zero remaining, no cleanup signals needed. Source clean after;
  v0.17.0 unchanged.
- Native CI `41039157086`: all five required jobs, no failed steps; actual
  Windows CLW-001..005 PASS lines verified. Release skipped.

All raw evidence remains under `bin/roc-s4-20261030/`. Both full-gate and
native watcher processes completed. Do not delete prior evidence.

## Next Steps

1. Correct only S4 typed execution/lock ownership and add discriminating tests.
2. Independently review and fully validate the repair with its new WAVE_BASE.
3. Archive/push repaired acceptance before considering S5; preserve unfinished
   work explicitly at the autonomy deadline.

## Validation Policy

Require 60 continuous seconds normal pressure, no new swap-outs and load1
<=5 before each Go/gofmt command. External Go is observed, not vetoed; own
work stays serial, GOMAXPROCS=1 and -p=1. Historical swap/free percentage is
diagnostic. Stop on warning/critical pressure, new swap-outs, unreliable
telemetry or gaps over 5s. Admission budget 600s; uncached tests, 40-minute
per-process timeouts, clean isolated sources/caches. Never enable goldens.

## Blockers

S4 boundary/lock conformance gaps block S5. Prior green gates are not sufficient.

## Context for Next Agent

Preserve S0 goldens, nineteen approved AR memo-lifetime insertions, S1 borrowed
locks/two constructors, S2 default-off eligibility/schema, S3 all-optimal
alignment/limits and S4 phase/lifetime guards. Stage-specific source inventory
transitions must remain finite and mutation-sensitive.
No Unreleased heading, frozen ADR-index churn or release/tag without a
justified cluster close. Upstream Git-strategy research remains deferred.
