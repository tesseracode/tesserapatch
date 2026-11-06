# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**10-hour unattended window dispatched** (operator, 2026-11-06): runs
**2026-11-06T16:37:59Z** to **2026-11-07T02:37:59Z**. v0.19.0 is published
(tag on `68142ca`); the window starts from `fddf4bd`.

Scope, in order (operator-approved):

1. **W0**: fix the AC-L68/AC-L69 partial-clone fixture so `stop()` ends the
   real `git-daemon` (it orphaned one per test run), with a guard that fails
   on the old code; add `make release-check` and the release-candidate rule
   (OV-D18 answer).
2. **D**: record the operator's GH #26 and GH #24 §9 answers in both PRDs;
   draft GH #26's ADR-013 Amendment 4 (design only, no code).
3. **W1**: GH #25 A1 — single recorded child, linear ancestry, behind ADR-013
   Amendment 3 (design → review → implement → review), with the PRD §9.2
   negative matrix and 0 false greens.
4. **W2**: reproduce the arbitration fail-fast leftover state in
   `verify --all`; fix only if it reproduces, with its own review.
5. **R**: v0.20.0 if A1 ships, under the new release-candidate rule.

## Active Task

- **Task ID**: `window-20261106-w0-fixture-leak-release-check`
- **Milestone**: Post-v0.19 backlog — validation hygiene, GH #25 A1, GH #26/#24 decisions
- **Description**: W0 — partial-clone fixture daemon leak + `make release-check`
- **Status**: In progress
- **Assigned**: 2026-11-06
- **WAVE_BASE**: `fddf4bd1483572acbb5208324a376b5d1796e864`
- **Deadline**: `2026-11-07T02:37:59Z`

WAVE_BASE = fddf4bd1483572acbb5208324a376b5d1796e864

## Session Summary

- **Baseline (16:37:59Z):** `HEAD == origin/main == fddf4bd`, no tracked
  changes, no stash, one worktree, no Git operation in progress. CI
  42235898642 green at `fddf4bd`.
- **Previous window:** closed and archived (HISTORY
  `2026-11-06T13:15:03Z`); v0.19.0 published. Its decisions OV-D1–OV-D18
  are in that archive and in `docs/supervisor/LOG.md`.
- **Validation machine hygiene (operator-approved):** 27 orphaned fixture
  `git-daemon` processes were terminated (NW-D4).

### Operator answers (2026-11-06)

| Item | Answer |
|---|---|
| OV-D2 | Accept make's behaviour: a whitespace-only `WAVE_CLOSE_TEST_LANES` on the make command line runs serial. **Correction:** the archived OV-D2 note says a `$(origin …)`/`$(value …)` check could make it fail. That is wrong: make strips the leading spaces before any Makefile code runs, so only a wrapper outside make could see them. |
| OV-D13 | Keep as is: the live S0 call-site guard is the source of truth; no PI-14 row in GH #15's closed migration inventory. |
| OV-D17 | All four: fix the fixture leak in the repo (W0); private runner pre-flight (reap our own orphans, check swap, choose serial when swap is high); automatic serial fallback after a lane resource stop; terminate the 27 orphans now. |
| OV-D18 | Adopt the release-candidate rule and `make release-check`. |
| Retro gate at `fddf4bd` | Asked what it would cover; skipped (NW-D2). |
| GH #26 §9 | Q1 yes (attestation anchor, option A); Q2 newest qualifying; Q3 yes (R14b, option C, `failed_at` unchanged); Q4 Fusion D/C3 after N1–N2 ship; Q5 yes (A3 out of scope); Q6 yes (ADR-013 Amendment 4). |
| GH #24 §9 | Q1 G (opt-in gates), only when a concrete user needs it; Q2 append via gates; Q3 yes; Q4 yes (legacy ungated stays incomplete, migration is GH #19); Q5 a new ADR superseding ADR-039's domain; Q6 yes (keep ADR-029 D7). |

### Decisions for morning review

| ID | Decision |
|---|---|
| NW-D1 | The window starts at the baseline check (16:37:59Z), deadline 02:37:59Z; WAVE_BASE `fddf4bd`. |
| NW-D2 | No separate retro gate at `fddf4bd`. Since the last gate (`2e44569`) only the CHANGELOG, the release-guard pin and tracking changed; local real-CHANGELOG tests and main/tag CI on three OSes covered them. W0's gate runs the full suite on a descendant tree and uses `WAVE_BASE=2e44569`, so its trailer walk also covers `68142ca` and `fddf4bd` (all six post-gate commits already carry the trailer, checked locally). |
| NW-D3 | The validation machine is reached over a LAN address for this window (operator), with the host key pinned to the machine's recorded key. The address stays out of tracked files. |
| NW-D4 | Terminated 27 orphaned fixture daemons: each was ours, parented by `init`, serving a deleted AC-L68/AC-L69 temp directory, and rechecked by PID just before its own `kill`. No test was running. |

## Current State

`main` is at `fddf4bd` (v0.19.0 published; tag on `68142ca`). Nothing in
this window is implemented yet. GH #25 stays open (A1 authorized in this
window; B and C3 not). GH #26 and GH #24 have operator answers; only GH
#26's ADR-013 Amendment 4 draft is authorized, no code.

## Files Changed

- `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`:
  window dispatch.

## Test Results

- None yet in this window. Baseline CI 42235898642 (`fddf4bd`) green.

## Next Steps

1. W0: fixture fix + guard (with a mutation run proving the guard fails on
   the old wrapper launch); `make release-check` + tests + AGENTS.md rule;
   independent review; local validation; push; CI; old-Mac gate with the
   new private pre-flight (`WAVE_BASE=2e44569`, NW-D2).
2. D: PRD answers + ADR-013 Amendment 4 draft; independent review.
3. W1: GH #25 A1 (ADR-013 Amendment 3 first).
4. W2: arbitration fail-fast reproduction.
5. R: v0.20.0 if A1 ships.

## Blockers

- None.

## Context for Next Agent

- **Old Mac:** `ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o
  ConnectTimeout=20 -o ForwardAgent=no -o IdentitiesOnly=yes -i
  ~/.ssh/id_ed25519 jbencardino@macbook-pro.netbird.selfhosted`. For this
  window the operator supplied a LAN address (private notes only); connect
  with `-o HostKeyAlias=macbook-pro.netbird.selfhosted` so the recorded host
  key is still enforced (NW-D3).
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Private evidence:** `bin/ov-20261106/` (ignored).
- **CI pushes:** `.github/workflows/ci.yml` cancels in-progress runs per
  ref, so do not push to `main` while its CI is running. The `release` job
  runs only for `v*` tags.
- **Release guards:** `assets/recipe_authority_parity_test.go:16` pins the
  current CHANGELOG release (`0.19.0`); the next release must move the pin
  together with the CHANGELOG heading, inside the gated commit (OV-D18
  rule).
- **Private evidence for this window:** `bin/nw-20261106/` (ignored).
