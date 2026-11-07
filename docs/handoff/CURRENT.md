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
- **Status**: In progress — CI passed at `ac65af4`; serial gate running
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
| HK-D4 | Private runner (untracked): the preparation step now requires the gated commit to be contained in `origin/main` (`git merge-base --is-ancestor`, any non-zero status refuses) and checks it out detached, instead of requiring `origin/main` to equal it; the run itself already pinned only the checked-out commit. Serial is the default; lanes run only when explicitly requested with the exact value `1`, so an empty or other value stays serial. The stub harness gained three cases (serial by default; `yes` and empty stay serial) and fails three of fifteen against the previous runner; the ancestry steps were exercised against a fresh clone (equal and behind accepted, unpushed refused). Previous copies are kept beside the new ones on the machine. |
| HK-D5 | Spotlight: a `.metadata_never_index` marker in the validation folder is **not honoured** on the validation machine's macOS (a probe file in the folder was still indexed after 90 s), so the marker was removed again. Renaming the folder with `.noindex` would break every pinned path. The working option is adding the folder to System Settings → Spotlight → Privacy on that machine, an operator action (an OS setting; not done by the agent). |
| HK-D6 | No CHANGELOG entry in HK: the file has no Unreleased section, and the next release entry will describe the gate and limit changes. |
| HK-D7 | Amendment 4 is marked ACCEPTED once design review rev-4 APPROVED: its own acceptance condition was an approving independent design review plus the operator scheduling the slices, and the operator's answer scheduled N1 for the next window. N2 is recorded as not yet scheduled. |
| HK-D8 | The local package step for `./internal/cli` is the targeted test set, not one unsharded `go test ./internal/cli` process: that process ran 60 minutes without a failure and was stopped by its own limit inside the AR observer (`TestS7ARPurgeProgressGuard`), as the unsharded package is known to exceed a single-process limit (the reason for the 22-process partition). The whole package is covered by the hosted CI shards and the serial validation-machine gate, as in W1. |

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
- Amendment 4 design review rev-2: **NEEDS REVISION** (rev-1 F1, F4, F5
  closed; three new blocking MEDIUM findings, no false green). Rev-3 makes
  D43 aggregate-aware, pins each allowed D44 difference with both rules'
  results (including T5), and gives T14 an explicit graph whose control
  fails E6 only.
- Amendment 4 design review rev-3: **NEEDS REVISION** (one blocking
  MEDIUM: oldest-first pins did not match the code's fail-fast shape; one
  LOW). Rev-4 corrects the pins.
- Amendment 4 design review rev-4: **APPROVED** (all findings closed, no
  false green). Amendment 4 ACCEPTED (HK-D7); PRD and ROADMAP updated.
- HK1 code review (`9aaf136`): **APPROVED**, no findings.
- Private runner relaxed and serial made the default (HK-D4); Spotlight
  marker tried and removed (HK-D5).

## Current State

`origin/main` is `ac65af4` (HK1 code, Amendment 4 rev-2 to rev-4 and
ACCEPTED, local validation record). HK1 is reviewed APPROVED, passes local
validation and hosted CI; the serial validation-machine gate at `ac65af4`
is running.

## Files Changed

- Tracking: `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`,
  `docs/supervisor/LOG.md`.
- HK1: `Makefile`, `internal/cli/wave_close_pushed_test.go` (new),
  `internal/cli/prepare_s7_registration_test.go`, `AGENTS.md`.
- Design: `docs/adrs/ADR-013-verify-freshness-overlay.md`,
  `docs/prds/PRD-historical-anchor-relanding.md`.

## Test Results

- Local (`9aaf136`): `TestWaveCloseHeadPushedAncestry`,
  `TestGH9WaveCloseSentinelMakefileWiring`,
  `TestS7ObservedRegistrationWrongInputs` pass; the new test fails on the
  pre-HK recipe.
- Local (`5bbdb2d`, `dev-go.sh` admission each run): gofmt on tracked Go
  files clean; `go vet` on all repository packages ok; `go build
  ./cmd/tpatch` ok. The unsharded `./internal/cli` process was stopped by
  its own 60-minute limit with no failure (HK-D8).
- Hosted CI run 42336508316 at `ac65af4`: success (ubuntu, macOS, Windows
  test jobs; S7 observers on ubuntu and macOS).
- Validation machine: serial gate at `ac65af4` with `WAVE_BASE=7a8f00b`
  started 2026-11-07T05:00:03Z (serial by default, HK-D4; preparation
  passed the new ancestor check). Result pending.
- Private runner stub harness: 15/15 locally and on the machine; 3/15 fail
  against the previous runner.

## Next Steps

1. Local serial validation, push, CI, serial gate with
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
