# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**16-hour unattended window (SW) dispatched** by the operator at
2026-11-11T00:41:56Z (baseline check); deadline **2026-11-11T16:41:56Z**.
`WAVE_BASE=c5bc3f8` (`origin/main` at dispatch, CI green). Decisions are
recorded as SW-D* below for the operator's review.

Scope (operator, 2026-11-10/11):

1. **W1 — GH #26 N1**: ADR-013 Amendment 4 D40 (D14 step 4 prefers the
   newest qualifying exact attestation), D41 (already recorded) and D44
   (measure every verify fixture under both selection rules, plus the
   read-only t3code `5ce5d79` reproduction) with tests T1, T2, T5, T6,
   T6b, T7, T8, T11 and T14.
2. **Test-time profiling**: measure which tests are slowest and why;
   propose, change nothing (operator, 2026-11-07).
3. **Validation-run queue**: in the private runner on the validation
   machine (untracked, operator choice).
4. **Private snapshots under `bin/`** (NW-D12): local whole-tree checks.
5. **Release v0.21.0** after N1, only if every gate passes (operator).

Follow-ups if time allows (operator-approved, in order): GH #26 N2 (D42,
D43; not in v0.21.0 unless it fits before the release gate); read-only
rebaseline of GH #18/#19/#20; low-risk profiling fixes in a reviewed wave
(identical coverage only); GH #25 B consolidation-workflow design
(planning only); GH #17 Windows failure inventory (read-only).

Loop per wave: implement → independent review (gpt-6.1-sol, static, no
Go) → local serial validation → push → hosted CI → serial
validation-machine gate at a close commit whose canonical field is
terminal (HK-D9).

## Active Task

- **Task ID**: `sw-gh26-n1`
- **Milestone**: Post-v0.20 backlog — GH #26 N1 / v0.21.0
- **Description**: ADR-013 Amendment 4 D40/D41/D44 with tests; D44
  measurement before release
- **Status**: In progress — N1 committed (`6bb6d13`); code review APPROVED after a tracking fix (`0ee0c99`). D44 measurement done: it blocks under Amendment 4 rev-4 and passes under rev-5 (D44 only, SW-D9); design review rev-5 APPROVED. D44 item 2 (t3code) recorded (SW-D10). `bin/` fix validated (SW-D11), awaiting static review. Still to do: release.
- **Assigned**: 2026-11-11
- **WAVE_BASE**: `c5bc3f875ad80f979b4bf91bb5df0be059e76903`
- **Deadline**: `2026-11-11T16:41:56Z`

WAVE_BASE = c5bc3f875ad80f979b4bf91bb5df0be059e76903

### Operator answers (2026-11-10)

| Item | Answer |
|---|---|
| Window | 16 hours if time allows, unattended, decisions recorded. |
| Release | v0.21.0 after N1, only if every gate passes. |
| Validation-run queue | Private runner on the validation machine (untracked). |
| Follow-ups | N2, rebaseline #18–#20, profiling fixes, GH #25 B design, GH #17 inventory. |
| Spotlight (HK-D5) | The operator adds it later; keep working with the machine as it is. |
| Lane mode on the validation machine | Stays serial until the operator reports the cleanup. |

### Decisions (SW-D*)

| ID | Decision |
|---|---|
| SW-D1 | Window starts at the baseline check (2026-11-11T00:41:56Z); WAVE_BASE `c5bc3f8`; deadline 16 hours later. |
| SW-D2 | Amendment 4 test mapping: T2 is AC-L33 (now also checks `verify --all` equality), T6 is AC-L35, T7 is AC-L34. AC-L34 is renamed `TestACL34_EqualIdentityQualifiersSelectNewestAttestation` and now pins the newer attestation; the acceptance ledger and the PRD row say so. T1, T5, T6b, T8, T11 and T14 (main and control) are new, in `internal/workflow/verify_anchor_selection_test.go`. T6b is new because the existing R7 row pins neither `failed_at` nor that no anchor was resolved. |
| SW-D3 | T6b pins the terminal anchor block (`unavailable`, 0 collected, 0 qualified, no replay anchor) instead of an absent block: the terminal phase always writes one. |
| SW-D4 | D40 and D10 share one trailer test (`expectedAttestationTrailers`), so "exact attestation" cannot mean different things in evidence and in selection. |
| SW-D5 | T8's shim fails only `git read-tree <attestation>^`, so the older landing still qualifies; the mutation "error read as does not qualify" then silently selects it and fails the test. |
| SW-D6 | Local admission did not pass (the development machine was busy). Development test runs moved to the validation machine, in a private development clone under the same 60-second admission. Gates still use the reviewed runner. |
| SW-D7 | The validation-run queue is private and untracked: first in, first out; one worker at a time; an entry past its deadline is recorded expired, not run; the verdict comes from the gate's own verdict line. Its 15 self-tests pass on both machines and seven mutations are caught. It queues gates; it never changes the reviewed runner. |
| SW-D8 | The D44 harness is private and never committed. It runs an oldest-first shadow beside D40 on a separate run context and records only the runs that differ or whose anchor moved. Tests that count Git calls may fail under it (the shadow doubles them); only the recorded differences count. As D44 requires, any difference outside the pinned fixtures blocks, even one shaped like T7 (both exit 0, only the replay anchor moved); it is resolved by revising Amendment 4 and repeating its design review. (Review of `6bb6d13` caught an earlier draft that exempted the T7 shape.) |
| SW-D9 | **D44 item 1 result: blocks under rev-4, passes under rev-5.** 406 recorded runs (252 tests; `internal/workflow` whole, plus 39 cli verify tests). Fourteen runs had a moved anchor. Every pinned run (T1, T5, T14 main, T14 control) matches its pin's exit code, `failed_at`, V7/V8 results, overlap class and promotion, checked by script. Rev-4 still blocks, for three reasons. Six runs of four unpinned fixtures differ exactly as T7 does: AC-L29, AC-L36 and the precondition run of the two rev-2 duplicate-attestation tests, all built by `RelandIdentical`, so every qualifier is an exact attestation of one identity. T7's own run also moves the historical anchor's `commit` and the target's V10 member baseline, which record the same selection. T14 main's run of the child's own verify is implied by its pin but not named. So I revised Amendment 4 to rev-5 instead of declaring a pass: it names all three, and states the A/A rule for harness artifacts; D40, the rest of the criterion and every other pin are unchanged. Nine runs differ without a moved anchor (seven in workflow, two in cli). An A/A control run (the harness with D40 on both sides) reproduces each one with the same report values, apart from commit IDs and temporary paths, which change with every fixture build. The causes are one-shot fault hooks consumed by the shadow, the shadow's `NoWrite`, `verify --all`'s shared shadow context seeing the real run's writes, and a temporary path. The seven tests that fail only under the harness (two Git-call counters, five one-shot hooks) fail identically under A/A. Under rev-5's rules no run blocks. I did not ask the operator, because Amendment 4 (D44) already makes a revision plus a repeated design review the remedy. Design review rev-5 APPROVED it (no findings). |
| SW-D10 | **D44 item 2 (t3code `5ce5d79`, read-only) does not block the release.** I ran the five GH #26 roots with the pre-N1 base (`c5bc3f8`) and N1 (`6bb6d13`), both built from `git archive`; the clone's state hash was unchanged afterwards. `copilot-cli-provider` exits 0 under both, identical apart from the timestamp. The four failing roots exit 2 at `parent-landing-drift` under both. Three have a single qualifier, so D40 cannot change them. For `attention-notifications`, N1 moves the anchor from `75bfc26` (replay `55fd4d7`) to `19785c6` (replay `964529c`), the genuine new base GH #26 names; nothing else in its report changes. All four still block on the closure member `copilot-cli-provider`: its only exact attestation `92e65de` is in none of their anchors' history, so its canonical patch is absent there. That is D42's R14b case, which N2 addresses (better remediation, same verdict). No passing root fails and no verdict changes, so I let the release proceed and left GH #26 open for N2. I posted no comment; a draft is in the morning report. |
| SW-D11 | **`bin/` local checks (NW-D12): match Go's module boundary, plus a private stub.** Tracked part: the S7 partition-owner walk (`walkS7PartitionTestFiles` in `internal/cli/prepare_s7_ci_timeout_guard_test.go`) no longer descends into a nested module, that is a directory below the root that holds a `go.mod` file, because `go test ./...` never runs those tests either. A directory named `go.mod` is not a module; any other stat error fails the walk. I did not skip `bin/` by name: that would also hide a tracked test file placed there. Private part: an untracked, ignored `bin/go.mod` stub (`module tpatch.invalid/private-bin`) makes the whole private tree one nested module, so locally `go vet ./...`, `go test ./...` and the S7 walk skip it. `gofmt -l .` still lists 72 private files under `bin/` (copies of third-party module sources inside old snapshots); removing them deletes private data, so I held that as SW-Q1. Gates are unaffected: the validation machine's clean clone has no `bin/`. |

### Open questions (SW-Q*)

| ID | Question |
|---|---|
| SW-Q1 | May I delete the 72 unformatted third-party files under the private `bin/` snapshots (or the snapshots that hold them), so that local `gofmt -l .` is clean? Nothing has been deleted. |

## Session Summary

- Baseline verified: `HEAD` == `origin/main` == `c5bc3f8`, no tracked
  changes, no stash, one worktree, no in-progress Git operation; main CI
  42359322494 green.
- The HK handoff stays archived in `docs/handoff/HISTORY.md` under
  2026-11-07T08:09:14Z; the IDLE handoff that followed it is replaced here.

## Current State

N1 is committed (`6bb6d13`, pushed) and approved on code review. D14 step 4
now prefers the newest qualifying exact attestation (D40). Steps 1–3, the
reports and the schema are unchanged, and no Git call was added.

D44 item 1 (every verify fixture under both rules) is measured: see SW-D9.
ADR-013 Amendment 4 rev-5 (D44 only) answers it; its design review
APPROVED it. D44 item 2 (the t3code reproduction) is recorded: see SW-D10.

The `bin/` local-check fix (SW-D11) is validated and awaits static review.
Its mutation proofs, run on the validation machine's development clone:

| # | Mutation | Result |
|---|---|---|
| MB1 | Nested modules walked | `TestS7CIFullSuiteTimeoutGuard` fails |
| MB2 | Root module itself treated as nested | `TestS7CIFullSuiteTimeoutGuard` fails |
| MB3 | A directory named `go.mod` read as a module | `TestS7CIFullSuiteTimeoutGuard` fails |

### N1 mutation proofs (validation machine, development clone)

Each mutation was applied privately, the named tests were run, and the
file was restored afterwards.

| # | Mutation | Test that fails |
|---|---|---|
| M1 | Oldest-first restored | T1, T5, T14 main, AC-L34 (T11 still passes, as pinned) |
| M2 | Attestation used without qualification | AC-L33 (T2) |
| M3 | Identity compare skipped when an attestation qualifies | AC-L35 (T6) |
| M4 | D40 runs before evidence classification (ambiguous routed to landed) | T6b |
| M5 | Probe error read as "does not qualify" | T8 |
| M6 | Newest qualifier whenever an exact attestation qualifies | T11 |
| M7 | E6 dropped from eligibility | T14 control |
| M8 | Anchor C skipped when anchor H passes | T5 |
| M9 | Nested verify selects oldest-first (target unchanged) | T14 main (T1 still passes) |
| M10 | Oldest among the exact attestations | AC-L34 (T7) |

## Files Changed

- Tracking: `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`,
  `docs/supervisor/LOG.md`.
- N1 (`6bb6d13`): `internal/workflow/verify_landed.go`,
  `internal/workflow/verify_anchor_selection_test.go` (new),
  `internal/workflow/verify_landed_groupd_test.go`,
  `internal/workflow/acceptance_ledger_test.go`, `SPEC.md`,
  `docs/adrs/ADR-013-verify-freshness-overlay.md`,
  `docs/prds/PRD-verify-freshness.md`,
  `docs/prds/PRD-historical-anchor-relanding.md`.
- Amendment 4 rev-5 (D44 only): `docs/adrs/ADR-013-verify-freshness-overlay.md`,
  `docs/prds/PRD-historical-anchor-relanding.md` (status line).
- `bin/` fix (SW-D11): `internal/cli/prepare_s7_ci_timeout_guard_test.go`.

## Test Results

- Validation machine, development clone at `cf2db23` plus the N1 diff:
  gofmt clean; `go vet ./internal/workflow/` clean. The 25 targeted tests
  pass: `TestA4*`, AC-L33–AC-L37, the rev-2 duplicate-attestation rows and
  the acceptance ledger. All ten mutations are caught.
- Whole `internal/workflow` package (same clone, code and tests synced,
  before the doc edits): `ok` in 1,415 s.
- Doc guards with the final doc edits (same clone; synced bytes checked
  identical): gofmt clean; 33 workflow doc and ledger guards `ok`;
  `./assets` `ok`; 11 cli doc and contract guards `ok`; `go vet ./...`
  clean; `go build ./cmd/tpatch` ok.
- D44 measurement (validation machine; private harness, restored after
  each run, with clone bytes checked identical to `6bb6d13`). A/B run:
  `internal/workflow` exits 1 on the seven harness-only failures; the 39
  cli verify tests `ok`. A/A control on the nine flagged workflow tests
  and two cli tests: the same seven failures, with identical messages;
  cli `ok`. Under rev-5: no blocking run. Evidence SHA-256: A/B workflow
  dump `ebe7a199…`, A/B cli `e14b01e7…`, A/A workflow `be139efa…`,
  A/A cli `d1d2ba11…`.
- Doc guards with the rev-5 ADR/PRD text (validation machine, synced bytes
  checked identical): gofmt clean; workflow doc and ledger guards `ok`;
  `./assets` `ok`; cli doc guards `ok`; `go vet ./...` clean; build ok.
- t3code `5ce5d79` reproduction (read-only; base `c5bc3f8` and N1
  `6bb6d13`, darwin/arm64, from `git archive`): see SW-D10. Binary
  SHA-256 base `7353284f…`, N1 `3c53c08e…`; private summary
  `7ed28cfa…`.
- `bin/` fix (SW-D11; validation machine, development clone, guard file
  checked identical, SHA-256 `bffc4401…`): gofmt clean;
  `go vet ./internal/cli/` clean; `TestS7CIFullSuiteTimeoutGuard` `ok`;
  MB1–MB3 caught. End-to-end, with scratch copies of the operator's local
  failure shapes under an ignored `bin/` (removed afterwards): without the
  stub the guard fails on the loose `bin/` test ("outside internal/cli"),
  and the nested snapshot module is skipped; with the stub it passes and
  `go list ./...` lists no `bin/` package.

## Next Steps

1. `bin/` fix: static review, commit.
2. Release candidate; push; CI; serial gate; release check; tag.

## Blockers

- None.

## Context for Next Agent

- **Old Mac:** reach it as in the archived handoff (host key pinned with
  `HostKeyAlias`; any LAN address stays in private notes). Its private
  runner is serial by default until the operator reports the cleanup.
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Gates:** run the gate at the close commit, whose canonical field is
  terminal (HK-D9). Do not push to `main` while its CI is running; a push
  during a gate no longer fails `[3/8]` or the runner's preparation, but
  the gated commit's CI must already be complete (AGENTS.md).
- **Release guards:** `assets/recipe_authority_parity_test.go:16` pins
  `0.20.0`; the next release moves the pin with the CHANGELOG heading in
  gated source, and `make release-check` runs before `git tag -a`.
- **Reviewers:** OpenAI/MAI models only (gpt-6.1-sol); reviewers never run Go.
- **Private evidence:** `bin/sw-20261111/` (ignored).
