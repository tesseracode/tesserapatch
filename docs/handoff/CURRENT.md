# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**Post-v0.20.0 housekeeping wave (HK) dispatched** by the operator's review
answers at 2026-11-07T02:30Z. The operator is present; this is not an
unattended window. v0.20.0 is published (tag on `495ca59`); the wave starts
from `7a8f00b`. No release is authorized.

Scope (operator answers, 2026-11-07):

1. **A4R**: independent design review of ADR-013 Amendment 4 (GH #26) now;
   implementation slice N1 is for the next window, not this wave.
2. **HK1** (tracked, one reviewed wave):
   - fix ADR-013's stale status line (Amendment 3 is ACCEPTED; Amendment 4
     per the review outcome);
   - `make wave-close-check` `[3/8]` accepts a `HEAD` that is contained in
     `origin/main` instead of requiring equality, with an AGENTS.md note
     that pushing during a gate run can still affect it (HK-D2);
   - raise the hosted observer limits for AM–AO, AP and AQ (NW-D25 answer,
     HK-D3);
   - backlog rows: a queue for validation runs of several versions while
     `main` advances; a test-time profiling task; serial gates on the
     validation machine until the operator's cleanup.
3. **Private** (untracked, validation machine): the gate runner's
   `origin/main` check becomes an ancestor check; serial is the default
   there; the validation folder is excluded from Spotlight if the marker
   works.

Close: independent code review, local serial validation, push, CI and a
serial validation-machine gate with `WAVE_BASE=7a8f00b`.

## Active Task

- **Task ID**: `post-v0-20-housekeeping`
- **Milestone**: Post-v0.20 operator review — GH #26 design review, gate
  and CI hygiene
- **Description**: Amendment 4 design review; ADR status fix; `[3/8]`
  ancestor check; hosted observer limits; backlog rows
- **Status**: In progress — tracking dispatched
- **Assigned**: 2026-11-07
- **WAVE_BASE**: `7a8f00bccfeb1e89f947c1c24590788541860691`

WAVE_BASE = 7a8f00bccfeb1e89f947c1c24590788541860691

### Operator answers (2026-11-07)

| Item | Answer |
|---|---|
| ADR-013 Amendment 4 (GH #26) | Design review now, N1 in the next window. |
| NW-D20 (`verify --all` record-write fix) | Accept as shipped. |
| NW-D24 (no push during a gate) | "Relax the runner to an ancestor check, but still put a note that pushing could affect the gate runs"; also add a mechanism to queue validation runs for different versions even if `origin/main` advances. |
| NW-D25 (hosted observer limits) | Small reviewed wave raising the hosted limits; also queue a task to time the tests that take longest and check whether that comes from too many tests running, unoptimized tests, or genuinely needed computation. |
| Lane mode on the validation machine | Keep serial for now; the operator will report when the machine is cleaned up. |
| ADR-013 status line | Fix it. |
| Spotlight on the validation folder | Exclude it. |

### Decisions (HK-D*)

| ID | Decision |
|---|---|
| HK-D1 | WAVE_BASE is `7a8f00b`, `origin/main` at dispatch. |
| HK-D2 | The ancestor check must also change `make wave-close-check` `[3/8]`, not only the private runner: `[3/8]` requires `HEAD == origin/main`, and the gate runs it in its own checkout, so a push during a gate would still fail it there. The new rule: `HEAD` must be contained in `origin/main` (pushed). A `HEAD` ahead of or diverged from `origin/main` still fails, and so does an ancestry query that cannot answer. |
| HK-D3 | New hosted limits: double the inner limit of each category whose child has used more than half of it in any recorded environment, keeping the 4-minute outer margin. AM–AO 8 m inner / 12 m outer (was 4/8; old-Mac serial process 268–274 s), AP 16/20 (was 8/12; 371–374 s), AQ 16/20 (was 8/12; 444–449 s old Mac, killed at 480 s on hosted macOS). AR (527 s of 37 m) and AS–AX (≤ 51 s of 4 m) are unchanged. Cost: a hung child in those three categories takes up to 20 minutes longer to fail. |

## Session Summary

- v0.20.0 shipped (see the release entry in `docs/supervisor/LOG.md`); the
  window handoff is archived in `docs/handoff/HISTORY.md` under
  2026-11-06T23:21:12Z. The release handoff that followed it is replaced
  here, as at the previous window start.
- Operator answers recorded above; HK dispatched.
- HK1 code committed at `9aaf136` (`[3/8]` ancestor check with a
  behavioural test, AGENTS.md note, AM–AO/AP/AQ limits). Targeted tests
  passed locally, and the new test fails on the pre-HK `[3/8]` recipe
  (behind-advanced-main case).
- Amendment 4 design review rev-1: **NEEDS REVISION** (F1–F4 MEDIUM, F5
  LOW; no false green). Rev-2 answers all five and fixes ADR-013's status
  line (Amendment 3 ACCEPTED); rev-2 is back with the reviewer.

## Current State

`origin/main` is `7a8f00b` (v0.20.0 tag on `495ca59`). Close-commit main
CI 42319994055 passed all five test jobs. Nothing implemented yet in HK.

## Files Changed

- Tracking: `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`,
  `docs/supervisor/LOG.md`.

## Test Results

- None yet for HK.

## Next Steps

1. Dispatch the Amendment 4 design review (independent, gpt-6.1-sol).
2. Implement HK1 with mutation-sensitive guards; private runner changes on
   the validation machine.
3. Code review, local serial validation, push, CI, serial gate with
   `WAVE_BASE=7a8f00b`, close.

## Blockers

- None.

## Context for Next Agent

- **Old Mac:** reach it as in the archived handoff (host key pinned with
  `HostKeyAlias`; any LAN address stays in private notes). Run gates serial
  (`WAVE_CLOSE_TEST_LANES=0`) until the operator reports the cleanup.
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Pushes:** do not push to `main` while its CI is running (in-progress
  runs are cancelled per ref). Until HK lands, also not while a
  validation-machine gate runs (NW-D24).
- **Release guards:** `assets/recipe_authority_parity_test.go:16` pins
  `0.20.0`; `make release-check` runs before any `git tag -a`.
- **Reviewers:** OpenAI/MAI models only (gpt-6.1-sol); reviewers never run Go.
