# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**Gate-time reduction window dispatched (2026-11-04T22:51:49Z, deadline
2026-11-05T10:51:49Z).** The goal is to bring the guarded old-Mac
`make wave-close-check` from the measured 3h38m to at most half
(≤ 6,540 s), with identical coverage. The previous handoff (GH #9, shipped and
closed) is archived in `docs/handoff/HISTORY.md` under **2026-11-04T20:49:04Z -
GH #9 nested-repo sentinel shipped; issue closed**.

WAVE_BASE = 7e0d63fe71756618a6d1478ec35cdf38e1af4897

## Active Task

- **Task ID**: gate-time-reduction
- **Milestone**: Post-v0.18 backlog → Validation-infrastructure follow-ups
- **Description**: Measured experiment, then an opt-in tracked change. Bring
  the old-Mac full gate to ≤ 6,540 s with the same 42 commands and 39 test
  commands, identical argv and `-count=1`, with no memory or CPU starvation.
- **Status**: In progress — dispatched; E1 audit and E2 harness starting
- **Assigned**: 2026-11-04 (operator-approved 12 h window)

## Operator approvals (2026-11-04T22:51Z)

- **Window:** 12 h, full scope. Audit and scratch trials first (E1/E2). If the
  trials meet the target, the tracked opt-in change follows, with review, CI
  and full gates (E3).
- **Admission for the experiment:** amortized. One 60 s admission at start;
  a fresh 60 s admission only if a continuous sample breaches the limits.
  This exception applies to the private scratch harness only. The accepted
  gate protocol stays unchanged unless a reviewed tracked change lands.
- **Lanes:** two. Probe three only if two shows headroom.
- **Load ceiling:** load1 ≤ 5 (the current rule) throughout.

## Plan (from `bin/gh9-20261104/gate-time-analysis/PLAN.md`)

1. **E1, read-only interference audit.** Cross-process hazards between two
   concurrently running test binaries: writes outside `t.TempDir()`, fixed
   ports, global locks, `HOME` or Git global config, golden updates, and
   shared temporary paths. The output is a pair-safety list.
2. **E2, private scratch harness on the old Mac.** Exact argv from the
   accepted command list, longest job first, memory-capped pairing, amortized
   admission, continuous sampler.
   - Negative controls: a colliding fixture must be detected; an injected
     failing test must fail the run with attribution.
   - Two consecutive measured runs, each ≤ 6,540 s.
3. **E3, only if E2 meets the target.**
   - Opt-in lane mode in `scripts/wave-close-test-shards.sh`, defaulting to
     serial, with the parity guards kept.
   - Mutation-sensitive tests.
   - Independent review, native CI, and old-Mac full gates in serial and lane
     modes.

## Decisions (for operator review)

(none yet)

## Session Summary

Dispatch only. Baseline verified at `7e0d63f`:
- HEAD equals origin/main;
- no tracked changes, no stash;
- a single worktree;
- no merge, rebase, cherry-pick or index-lock state.

## Current State

- Serial baselines: S9 `5133c95` at 13,081 s; GH #9 `b8a5f37` at 13,093 s.
  Both PASS.
- No experiment has run yet.

## Files Changed

Tracking only (dispatch): `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`.

## Test Results

None yet.

## Next Steps

1. E1 audit.
2. E2 harness and negative controls.
3. E2 measured runs (×2).
4. E3 go/no-go.

## Blockers

None.

## Context for Next Agent

- **Old Mac:** `ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o
  ConnectTimeout=20 -o ForwardAgent=no -o IdentitiesOnly=yes -i
  ~/.ssh/id_ed25519 jbencardino@macbook-pro.netbird.selfhosted`
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Private evidence:** `bin/gt-20261104/` (ignored).
- **CI pushes:** `.github/workflows/ci.yml` cancels in-progress runs per ref,
  so do not push to `main` while source CI is running.
