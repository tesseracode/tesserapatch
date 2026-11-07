# Current Handoff

## Status

**Cluster state**: IDLE

**Post-v0.20.0 housekeeping wave (HK) accepted; idle.** The operator's
review answers of 2026-11-07 are implemented: ADR-013 Amendment 4 (GH #26)
passed its design review at rev-4 and is **ACCEPTED** (slice N1 scheduled
for the next window, N2 not scheduled); `make wave-close-check` `[3/8]`
accepts a pushed `HEAD` that `origin/main` has moved past; the hosted
AM–AO, AP and AQ observer limits are doubled; the private validation
runner uses the same ancestor rule and runs serial by default. HK1
(`9aaf136`) passed code review, hosted CI 42336508316 and 42341338991, and
a serial validation-machine gate at the close commit `41f2538` with
`WAVE_BASE=7a8f00b` (42/42 commands, 10,912.7 s, wave-close-check 8/8).
No release was authorized; the CHANGELOG entry waits for the next release
(HK-D6).

## Active Task

- **Task ID**: none (idle)
- **Milestone**: Post-v0.20 backlog
- **Description**: Awaiting the operator's next window
- **Status**: Idle — HK accepted at `41f2538`
- **Assigned**: 2026-11-07
- **WAVE_BASE** (HK, closed): `7a8f00bccfeb1e89f947c1c24590788541860691`

## Session Summary

- **A4R**: ADR-013 Amendment 4 design review rev-1 to rev-3 NEEDS
  REVISION (shallow repositories, the Amendment 3 interaction, aggregation
  of ancestry answers, forward mode, release pins), rev-4 APPROVED.
  Amendment 4 ACCEPTED (`5bbdb2d`); the PRD and ROADMAP say N1 next window.
- **HK1** (`9aaf136`): `[3/8]` ancestor check with a behavioural test that
  fails on the old recipe; AGENTS.md note on pushing during a gate;
  AM–AO 8/12 m, AP and AQ 16/20 m hosted limits with a mutation fixture.
  Code review APPROVED, no findings.
- **Private runner** (untracked, validation machine): ancestor rule in
  preparation, serial by default, stub harness 15/15 (3/15 fail on the
  previous runner).
- **Gates:** the first attempt at `ac65af4` was refused at `[5/8]` because
  the handoff still read `IN PROGRESS` (HK-D9, no Go ran); the close
  commit `41f2538` passed.

The full HK handoff is archived **byte-for-byte** in
`docs/handoff/HISTORY.md` under
**2026-11-07T08:09:14Z - Post-v0.20.0 housekeeping wave (HK) accepted**:

- Archive block (header plus terminal body): 12,315 bytes,
  SHA256 `d6784717aedcf200b56fbce7a41dbe6731632dd296ca8f2b1e7901974eef675d`.
- Prior HISTORY prefix: 1,299,878 bytes, SHA256
  `3c0fdcf2f1a94c9ecb4511a748e1c6159cc8b23bc9108495173177c047e19f5c`, preserved unchanged.
- HISTORY after this archive: 1,312,193 bytes, SHA256
  `1b511c3915b27148f33e6243ecf98491e7f4225861b29dadcf1e8bb4f5590eba`.

### Decisions for operator review

Full rationale is in the archived handoff and `docs/supervisor/LOG.md`.

| ID | Decision |
|---|---|
| HK-D1 | WAVE_BASE `7a8f00b`, `origin/main` at dispatch. |
| HK-D2 | The ancestor rule also changes the tracked `[3/8]` check, not only the private runner; ahead, diverged or unanswerable still fails. |
| HK-D3 | Double the inner limit of each hosted observer category whose child used more than half of it (AM–AO, AP, AQ); others unchanged. |
| HK-D4 | Private runner: ancestor rule in preparation, detached checkout of the gated commit; serial unless lanes are requested with exactly `1`. |
| HK-D5 | Spotlight ignores the folder marker on the validation machine; the exclusion needs a System Settings → Spotlight → Privacy entry (operator action). |
| HK-D6 | No CHANGELOG entry until the next release. |
| HK-D7 | Amendment 4 ACCEPTED on design review rev-4 APPROVED plus the operator's scheduling answer (N1 next window). |
| HK-D8 | The local `./internal/cli` step is the targeted set; the unsharded package process exceeds 60 minutes and is covered by CI shards and the gate. |
| HK-D9 | Gate at the close commit (terminal canonical field), not mid-cycle; it ran in parallel with the tracking-only close commit's CI. |

Open for the operator: the Spotlight setting on the validation machine
(HK-D5); when the validation machine is cleaned up, so lane mode can be
retried there.

## Current State

Idle. `main` is at this tracking commit; `v0.20.0` is the latest release
(tag on `495ca59`). HK changes are unreleased.

GH #26: Amendment 4 ACCEPTED; N1 is scheduled for the next window, N2 is
not scheduled. GH #25 stays open (B and C3 not authorized); GH #24 has an
approved PRD with operator answers and no implementation authorization.

## Files Changed

This close (tracking files only): `docs/handoff/CURRENT.md`,
`docs/handoff/HISTORY.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`.

## Test Results

- Hosted CI 42336508316 (`ac65af4`) and 42341338991 (`41f2538`): green on
  every job.
- Validation-machine gate at `41f2538`, `WAVE_BASE=7a8f00b`: **PASS
  (serial)**, 42/42, 10,912.7 s, wave-close-check 8/8.

## Next Steps

1. Check this tracking commit's main CI.
2. Next window candidates: GH #26 N1 (scheduled by the operator), the
   validation-run queue and test-time profiling backlog rows, GH #24,
   GH #25 B/C3 (not authorized).

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
- **Private evidence:** `bin/nw-20261106/` (ignored).
