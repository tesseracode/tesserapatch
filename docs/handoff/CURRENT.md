# Current Handoff

## Status

**Cluster state**: ACCEPTED

**W0, D and W1 (GH #25 A1) are accepted; v0.20.0 publication is pending
tag CI.** The validated source `7b690b2` passed independent review, CI
42278212760 (all five jobs) and a complete serial validation-machine gate
with `WAVE_BASE=fddf4bd` (42/42 commands, 10,868.0 s, evidence verifier
PASS). `7b690b2` already carries the v0.20.0 CHANGELOG section and the
release-guard pin (release-candidate rule); this release commit changes
only tracking files. Do not claim shipment until the tag CI and the
automatic GitHub release succeed.

The operator's 10-hour unattended window runs **2026-11-06T16:37:59Z** to
**2026-11-07T02:37:59Z**. The operator authorized v0.20.0 if A1 shipped.

## Active Task

- **Task ID**: `release-single-child-current-anchor-v0-20-0`
- **Milestone**: Post-v0.19 backlog — GH #25 A1 / v0.20.0
- **Description**: Publish the accepted window work as v0.20.0, verify
  main and tag CI and the GitHub release, then close the window
- **Status**: Release consolidation committed — `make release-check`,
  annotated tag and pushes next
- **Assigned**: 2026-11-06
- **WAVE_BASE**: `fddf4bd1483572acbb5208324a376b5d1796e864`
- **Validated source**: `7b690b2e5cf92c7007c7f21175ecf0b7b05e27a9`
- **Deadline**: `2026-11-07T02:37:59Z`

WAVE_BASE = fddf4bd1483572acbb5208324a376b5d1796e864

## Session Summary

- **W0** (`583f7dc`): the AC-L68/AC-L69 partial-clone fixture starts the
  real `git-daemon` and stops it (it orphaned two per run), with a guard
  that fails on the old launch; `make release-check` enforces the
  release-candidate rule before tagging. Review round 3 APPROVED.
- **D** (`d6d2008`): the operator's GH #26 and GH #24 answers recorded in
  both PRDs; ADR-013 Amendment 4 (GH #26 N0) drafted, PROPOSED, no code.
  Design review round 2 APPROVED.
- **W1** (`2354df9`, `e142bfd`, closed at `7b690b2`): GH #25 A1 under
  ADR-013 Amendment 3 (ACCEPTED). A single recorded direct hard child with
  exact evidence, a clean historical half, a full-context-clean child and
  a merge-free `T..H` passes V8's current half as
  `explained-by-descendant`; schema 1.3. Every `HEAD` read in a verify run
  uses the captured commit (D36), and the arbitration fail-fast leak (W2)
  is fixed (D37). Design round 1 UNSOUND → round 2 APPROVED; code review
  round 1 NEEDS REVISION (a `verify --all` persistence-error defect and
  Windows shim rows) → round 2 APPROVED. 19/19 manual mutants killed.
- **Gates:** W0's gate at `6eb98a0` failed at serial preparation after a
  lane resource stop because `7b690b2` was pushed during the run; no test
  failed (NW-D23, NW-D24). W1's gate at `7b690b2` covered the whole window:
  serial PASS, 42/42, 10,868.0 s, wave-close-check 8/8, no fixture daemon
  left afterwards.

The full window handoff is archived **byte-for-byte** in
`docs/handoff/HISTORY.md` under
**2026-11-06T23:21:12Z - 10-hour window: W0, D and W1 (GH #25 A1) accepted**:

- Archive block (header plus terminal body): 24,591 bytes,
  SHA256 `cbff1f032c6898f6aeb5807c2ed9fa10fd63415324fb278ae384ab35c6f164d5`.
- Prior HISTORY prefix: 1,275,287 bytes, SHA256
  `b893a6815ccc93ea5c792b55c2f42eab46472da019feaaa55a758a95f2ff79e5`,
  preserved unchanged.
- HISTORY after this archive: 1,299,878 bytes, SHA256
  `3c0fdcf2f1a94c9ecb4511a748e1c6159cc8b23bc9108495173177c047e19f5c`.

Do not archive this window again; append a separate entry only if needed
after publication.

### Decisions for morning review

Full rationale is in the archived handoff and `docs/supervisor/LOG.md`.

| ID | Decision |
|---|---|
| NW-D1 | The window started at the baseline check (16:37:59Z); WAVE_BASE `fddf4bd`. |
| NW-D2 | No retro gate at `fddf4bd`; a later gate on a descendant tree covers it. |
| NW-D3 | The validation machine was reached over a LAN address with its recorded host key pinned; the address stays out of tracked files. |
| NW-D4 | Terminated 27 orphaned fixture daemons on the validation machine after per-PID checks. |
| NW-D5 | Terminated 84 orphaned fixture daemons on the dev machine after the same checks. |
| NW-D6 | The make variable is `RELEASE` (not `VERSION`, already used for ldflags), read from the environment. |
| NW-D7 | `release-check` accepts the gate's terminal Cluster-state tokens and runs by hand before `git tag -a`. |
| NW-D8 | The fixture starts the real `git-daemon` binary rather than killing a process group (Windows build). |
| NW-D9 | Private runner pre-flight: one runner at a time, reap only our own orphans, start serial when swap is high. |
| NW-D10 | After a lane resource stop, fall back to serial at once; AGENTS.md updated to match. |
| NW-D11 | ADR-013 numbering: Amendment 3 = GH #25 A1, Amendment 4 = GH #26. |
| NW-D12 | Local whole-tree checks are scoped to repository files (private `bin/` snapshots break them); backlog row added. |
| NW-D13 | Cluster state set to `APPROVED` in place before each gate, back to `IN PROGRESS` for the next wave. |
| NW-D14 | A1 F1: pin every `HEAD`-dependent read to the captured commit (D36) instead of adding re-checks. |
| NW-D15 | A1 F2: refuse promotion when any merge is in `T..H` or the query fails (P10). |
| NW-D16 | A1 F3: the child's own V8 must be `materialized-clean` (P9). |
| NW-D17 | W2 (arbitration fail-fast leak) folded into W1 as D37. |
| NW-D18 | P3 (nested verify never promotes) holds by construction, with a unit guard. |
| NW-D19 | The P8 fixture uses a malformed `recipe-coverage.json` (V11 fails after anchor C). |
| NW-D20 | Fixed the pre-existing `verify --all` persistence-error count in W1: a pass whose record write failed is now `error`. |
| NW-D21 | The two shell-shim A1 rows skip on Windows; portable gitutil rows cover the pinning; an unresolvable explicit rev now fails. |
| NW-D22 | W1's gate used the window's `WAVE_BASE=fddf4bd`. |
| NW-D23 | W0's gate was not rerun; W1's gate on a tree containing W0 covers it. |
| NW-D24 | Never push while a validation-machine gate runs (its serial fallback needs `origin/main` == the gated commit). |

Open for the operator: ADR-013 Amendment 4 (GH #26 N0) is PROPOSED and
needs acceptance before any code; NW-D20 (a pre-existing defect fixed
inside an A1 wave); NW-D24 (a new push rule while a gate runs). The
validation machine started this gate with 1,752M of swap in use and no
gate running, so it went straight to serial (see the OV-D17 discussion).

## Current State

All window work is accepted. Remaining: tag v0.20.0, push main and the tag,
verify both CI runs and the automatic release, then close the window
(`Cluster state` → `IDLE`).

GH #25 stays open: C1–C2 and A1 shipped; B and C3 are not authorized.
GH #26 has an ADR-013 Amendment 4 draft (PROPOSED) and GH #24 an approved
PRD with operator answers; neither has implementation authorization.

## Files Changed

Release consolidation only (tracking files):

- `docs/handoff/CURRENT.md`: this release handoff.
- `docs/handoff/HISTORY.md`: the window archive.
- `docs/ROADMAP.md`: W0, A1 and the arbitration row accepted; release
  pending.
- `docs/supervisor/LOG.md`: gate, wave close and release entries.

The v0.20.0 CHANGELOG section and release-guard pin are in the gated
commit `7b690b2`.

## Test Results

- CI 42278212760 (`7b690b2`): green on all five jobs.
- Validation-machine gate at `7b690b2`, `WAVE_BASE=fddf4bd`: **PASS
  (serial)**; 42/42 top-level commands, 39 timed test commands, 0
  concurrent test seconds, 10,868.0 s, evidence verifier PASS,
  wave-close-check 8/8.
- `make release-check RELEASE=v0.20.0 GATED=7b690b2…`: run before the tag
  (result in LOG).

## Next Steps

1. `make release-check RELEASE=v0.20.0 GATED=7b690b2e5cf92c7007c7f21175ecf0b7b05e27a9`.
2. `git tag -a v0.20.0`; push main and the tag.
3. Verify main CI, tag CI, the tagged install version and the GitHub
   release; then close the window (`IDLE`) by `2026-11-07T02:37:59Z`.

## Blockers

- None.

## Context for Next Agent

- **Old Mac:** reach it as in the archived handoff (host key pinned with
  `HostKeyAlias`; any LAN address stays in private notes).
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Gates and pushes:** do not push to `main` while its CI is running
  (in-progress runs are cancelled per ref) or while a validation-machine
  gate is running (NW-D24).
- **Release guards:** `assets/recipe_authority_parity_test.go:16` pins
  `0.20.0`; the next release moves the pin with the CHANGELOG heading in
  gated source, and `make release-check` runs before `git tag -a`.
- **Private evidence for this window:** `bin/nw-20261106/` (ignored).
