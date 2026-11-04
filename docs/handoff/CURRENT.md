# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**GH #9 implementation window open.** The operator approved a 12-hour window
on 2026-11-04 for GH #9 (wave-close sentinel misses source inside nested
repositories), with an optional read-only v0.18 reproduction of GH #25/#26
only if GH #9 is complete and time remains. GH #13 is CLOSED/completed and
v0.18.0 is published; neither is reopened by this window.

## Active Task

- **Task ID**: `gh9-nested-repo-sentinel`
- **Milestone**: Post-v0.18 reliability — GH #9
- **Description**: Make wave-close `[2/8]` see source files inside untracked nested Git repositories, preserving allowlist behavior, never descending into `.git/`, with a failure-proving fixture
- **Status**: In progress — dispatched; reproduction and implementation next
- **Assigned**: 2026-11-04
- **Window**: 2026-11-04T16:16:08Z → **2026-11-05T04:16:08Z** (12 hours, operator-approved; no silent extension)
- **WAVE_BASE**: `064bc381dedd07fb2125ef1a75e639c01a841555`

Scope: GH #9 only, then optionally GH #25/#26 read-only reproduction on the
latest release. Out of scope: GH #17, migration/coverage contracts
(GH #18-22/#24), research tracks, and the gate-time reduction experiment
(recorded in ROADMAP as a follow-up after GH #9, not started here).

## Decisions (for operator review)

- **GH9-D1 — Latest-main health at dispatch.** CI 41859120038 at `064bc38`
  had its two macOS jobs cancelled by GitHub ("job was not acquired by Runner
  of type hosted even after multiple attempts", capacity notice, zero steps
  executed). `064bc38` changes only four tracking documents relative to
  `0fc232d`, whose CI 41844293141 passed all five required jobs. Latest-main
  source health is therefore taken from 41844293141; the capacity-cancelled
  run is not re-run, and this dispatch push supersedes it. GH #9's own source
  commit must still receive a complete five-job native CI pass.

## Session Summary

GH #13 was closed as completed at 2026-11-04T15:15:52Z after the operator's
account change. The post-v0.18 backlog was reconciled and archived at
15:32:06Z (ROADMAP "Current Backlog — Post-v0.18.0"). The operator then
approved this 12-hour GH #9 window and asked for a follow-up backlog item to
halve old-Mac full-gate time without starving the machine.

Dispatch baseline verified at 16:22:12Z: HEAD = origin/main = `064bc38`,
clean tree, no stash, one worktree, no merge/rebase/cherry-pick/index lock,
zero untracked files.

## Current State

- GH #9: OPEN, in progress in this window.
- GH #13: CLOSED/completed; v0.18.0 (release 433986008) published, tag object
  `648afeeee5fcacffdf871177ad022406074b51c3` unchanged.
- GH #17: OPEN; full Windows suite remains known-red; not in scope.
- Current `[2/8]` (Makefile:79-110) enumerates candidates and allowlist
  expansions only through `git ls-files --others`; Git reports an untracked
  nested repository as one opaque `dir/` entry, so source inside it is invisible.

## Files Changed

Tracking only so far: `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`.

## Test Results

None yet for GH #9. All local/old-Mac Go and gofmt runs will use the accepted
guarded wrappers (60 s normal-pressure admission, no new swap-outs,
load1 <= 5, serial private Go 1.26.5 with the accepted environment).

## Next Steps

1. Reproduce the false-clean `[2/8]` result with a nested-repository fixture.
2. Implement a filesystem-aware supplement that is deterministic, POSIX-sh,
   allowlist-preserving and never descends into `.git/`.
3. Add mutation-sensitive Go fixtures; guarded format/focused checks.
4. Independent review (OpenAI model), then push, native CI and the fresh
   guarded old-Mac `make wave-close-check WAVE_BASE=064bc381dedd07fb2125ef1a75e639c01a841555`.
5. Close GH #9 only after review, native CI and gate pass; then, time
   permitting, read-only GH #25/#26 reproduction on v0.18.

## Blockers

None.

## Context for Next Agent

The old-Mac harness (`run-gate.py`) runs `make wave-close-check` and requires
the exact line `=== Mechanical gate: PASS ===`; `[2/8]` must therefore stay
clean (not WARN) on a normal checkout. The shard-parity guard
(`internal/cli/prepare_s7_ci_timeout_guard_test.go`) requires `'scripts/**'`
exactly once in the Makefile and no recursive `$(MAKE)` in `wave-close-check`.
Private evidence for this window goes under `bin/gh9-20261104/` (ignored).
