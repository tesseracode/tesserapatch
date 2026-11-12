# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**TW unattended window in progress.** The operator authorized a 12-hour
unattended window (TW) from 2026-11-12T20:31:32Z to
**2026-11-13T08:31:32Z** (TW-D1). Wave 1 releases **v0.22.0**: GH #26 N2
(approved at `a37e512`, unreleased) plus a refresh of the 52 displaced
`verify_anchored.go` doc cites (SW-D25). Wave 2: a cite guard, the GH #17
prerequisites, the ROCS8 build-once fixture, and a measurement of the S6
shard merge (measure only). GH #25 B is not authorized.

## Active Task

- **Task ID**: tw-window (wave 1: `release-v0.22.0`; wave 2: `tw-wave-2`)
- **Milestone**: Post-v0.21 backlog — GH #26 close, SW-D25, GH #17, SW-D14
- **Description**: Release N2 and the doc-cite refresh as v0.22.0, then
  the cite guard, GH #17 prerequisites, ROCS8 fixture template and S6
  shard measurement
- **Status**: In progress — dispatched
- **Assigned**: 2026-11-12
- **WAVE_BASE** (TW): `dd4c082a7cbf04b61e3d5514c580e10ac11ee9ec`
  (`origin/main` at dispatch; both TW gates use it)

## Session Summary

- **Dispatch (2026-11-12T20:31:32Z).** Baseline verified: `HEAD` ==
  `origin/main` == `dd4c082`, no tracked changes, no stash, one
  worktree, no Git operation in progress; CI 43130446490 green; the
  validation machine idle.
- The SW handoff is archived byte-for-byte in `docs/handoff/HISTORY.md`
  under **2026-11-12T00:48:26Z - SW unattended window closed; v0.21.0
  shipped; GH #26 N2 approved**, with the SW-D* decisions.

### Operator answers (2026-11-12, recorded verbatim in substance)

| ID | Answer |
|---|---|
| TW-A1 | SW-Q1: delete the 72 unformatted third-party files under the private `bin/` snapshots. |
| TW-A2 | Release first: v0.22.0 with GH #26 N2 and the doc-cite refresh, then wave 2. |
| TW-A3 | Wave 2 scope: cite guard, GH #17 prerequisites, ROCS8 fixture fix, S6 shard-merge measurement (measure only). |
| TW-A4 | GH #25 B: not now. |
| TW-A5 | Start now, 12 hours, unattended; record decisions for morning review. |

### Decisions for operator review

| ID | Decision |
|---|---|
| TW-D1 | Window starts at the baseline check, 2026-11-12T20:31:32Z (local and GitHub clocks agree); deadline 2026-11-13T08:31:32Z; WAVE_BASE `dd4c082`. |

## Current State

Dispatched. `main` is at this tracking commit. `v0.21.0` is the latest
release (tag on `fc6e80f`); GH #26 N2 is approved and unreleased.

## Files Changed

This dispatch (tracking files only): `docs/handoff/CURRENT.md`,
`docs/ROADMAP.md`, `docs/supervisor/LOG.md`.

## Test Results

- None yet this window. Baseline: CI 43130446490 (`dd4c082`) green.

## Next Steps

1. SW-Q1 (TW-A1): delete exactly the files `gofmt -l .` lists under the
   ignored `bin/` snapshots; confirm local `gofmt -l .` is clean.
2. Wave 1: refresh the 52 displaced cites (docs only); static review;
   doc guards on the validation machine.
3. Release candidate: CHANGELOG `## v0.22.0`, release pin `0.22.0`,
   versioned heading fixture; review; validate; push; CI; serial gate;
   `make release-check`; tag; publish.
4. Wave 2 on a local branch during gate 1; rebase after the tag; review;
   validate; push; CI; serial gate at the wave-2 close commit.
5. Close: archive this handoff to HISTORY byte-for-byte; IDLE.

## Blockers

- None.

## Context for Next Agent

- **Old Mac:** reach it as in the archived SW handoff (host key pinned
  with `HostKeyAlias`; any LAN address stays in private notes). Its
  private runner is serial by default until the operator reports the
  cleanup.
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Gates:** run the gate at the close commit, whose canonical field is
  terminal (HK-D9). Do not push to `main` while the gated commit's CI is
  running.
- **Release guards:** `assets/recipe_authority_parity_test.go:16` pins
  `0.21.0`; the release candidate moves the pin with the CHANGELOG
  heading in gated source, and `make release-check` runs before
  `git tag -a`.
- **Reviewers:** OpenAI/MAI models only (gpt-6.1-sol); reviewers never run Go.
- **Private evidence:** `bin/tw-20261112/` (ignored); SW evidence in
  `bin/sw-20261111/`.
