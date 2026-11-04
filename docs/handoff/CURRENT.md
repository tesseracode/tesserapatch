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
- **Status**: In progress — E1 audit done; controls C1 and C2 behaved as
  intended; E2 run 1 running since 23:10:07Z; E3 opt-in drafted and validated
  locally, uncommitted until E2 meets the target (2026-11-04T23:30Z)
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

- **GT-D1, experiment checkpoint `85a6763`.** The scratch clone is the
  dispatch commit `85a6763`, which only changes docs over WAVE_BASE
  `7e0d63f`. Every test input is identical.
- **GT-D2, admission semantics in the scratch harness.**
  - One 60 s `PressureWindow` admission at start (pressure normal, no new
    swap-outs, load1 ≤ 5, external Go allowed).
  - A continuous monitor then samples every 1.5 s. Each launch needs a fresh
    healthy sample and an unbroken healthy record since the last admission,
    so every launch is still preceded by at least 60 continuous healthy
    seconds. The measurement is continuous rather than taken in an idle gap.
  - Any breach forces a fresh 60 s qualifying window before the next launch.
  - Load above 5 while commands run is recorded and blocks new launches until
    requalified, but does not stop running commands. The serial gate itself
    peaked at 5.63 during execution.
  - The accepted runtime stops are unchanged: pressure ≠ normal, new
    swap-outs, sample gap > 5 s, telemetry failure, and the deadline.
- **GT-D3, static two-lane partition.** A tracked shell change must be
  deterministic and reviewable, so it gets a fixed partition, not a runtime
  scheduler.
  - **Lane A:** shards 1, 13, 17, 21, 22, 26, 29. About 5,220 s, with a peak
    of 2.16 GiB.
  - **Lane B:** the other 32 shards. About 5,218 s, and it holds all five
    shards ≥ 2.5 GiB, so heavy shards never overlap. The estimated combined
    peak is about 7 GiB.
  - Each lane runs its shards in script order. The preflight (gofmt, vet,
    build) runs alone first.
  - E2 measures this exact plan through the harness `--plan` option, not the
    dynamic scheduler.
- **GT-D4, timing scope.** The harness calls the pinned toolchain directly,
  with the gate's environment and fresh caches per run. Make and the non-Go
  checks `[1/8]`–`[5/8]` are excluded; they take seconds.
- **GT-D5, checkout integrity after every command.** Any non-ignored change
  fails the run. Changes to ignored files are recorded only.
- **GT-D6, negative controls.** They run on local-only control commits that
  are never pushed.
  - **C1:** a rendezvous collision pair in shards 17 and 18. It must fail
    with attribution and overlapping intervals.
  - **C2:** a stray write into `internal/cli`. It must fail as a checkout
    mutation even though the test exits 0.
- **GT-D7, E2 acceptance.** Each run must meet all of these:
  - PASS, with wall time ≤ 6,540 s including admission waits;
  - the same 42 argv;
  - per-package `ok` sets equal to the serial baseline;
  - pressure always normal and no new swap-outs;
  - load1 ≤ 5 at every launch.

  Two consecutive passing runs are required.
- **GT-D8, E1 fold-in (supersedes GT-D3's lane lists).** The audit
  (`bin/gt-20261104/e1-audit.md`, private) found no pair that must never run
  together in normal mode, plus two conditions:
  - Golden recording (`TPATCH_RECORD_ROUTING_GOLDENS`,
    `TPATCH_RECORD_PREPARE_PIB_GOLDENS`, `TPATCH_UPDATE_ROC_S0_GOLDENS`),
    `TPATCH_LIVE_PROVIDER`, `GIT_DIR`, `GIT_WORK_TREE` and `GIT_INDEX_FILE`
    must be unset. The harness scrubs them; the tracked runner refuses to
    start if any is set.
  - Shard 01's store test creates and removes a scratch directory under
    `internal/store/` while `./...` shards walk the tree. The plan therefore
    starts lane B with `./internal/cli`-only shards for longer than shard 01
    takes (about 3,718 s against 2,784 s).

  Final plan: **lane A** shards 1, 2, 3, 4, 6, 13, 17, 22, 26, 29 (about
  5,222 s, peak 2.16 GiB); **lane B** shards 18–21, 23–25, 27, 28, 30–39,
  then 5, 7–12, 14–16 (about 5,216 s, all five ≥ 2.5 GiB shards).
- **GT-D9, tracked opt-in shape (E3).** `WAVE_CLOSE_TEST_LANES ?=` in the
  Makefile. Empty keeps the serial script; `2` runs
  `scripts/wave-close-test-lanes.sh 2`; any other value fails check 8. The
  runner copies the serial script's preamble and `go test` lines verbatim,
  checks that its static partition covers every serial shard exactly once,
  and fails closed when it does not. `scripts/wave-close-test-shards.sh` is
  unchanged. This keeps the plan's "lane mode in the shard script" intent
  without touching the CI-parity source of truth.
- **GT-D10, where local validation runs.** Local free memory stays near 67%,
  under the 80% rule, so local runs use the accepted protocol instead:
  60 s of pressure level normal, no new swap-outs, load1 ≤ 5 and no Go
  process. They run in a clean temporary clone with only the E3 files
  overlaid, because the main checkout's git-ignored `bin/` scratch (old
  source copies) breaks `go vet ./...` and the repository-walking guards.
- **GT-D11, docs-only push cancelled CI.** Pushing `21be56f` cancelled CI
  run 41929031542 for `85a6763`. Both commits change only docs, so run
  41930585027 on `21be56f` covers both trees. No further pushes while a CI
  run is in progress.
- **GT-D12, external load during a measured run (decided before run 1's
  result was known).** GT-D7 is unchanged: wall time includes admission
  waits. If a run misses 6,540 s and the miss is no larger than the time
  launches were held for load from processes outside the gate, the run is
  recorded as inconclusive. It neither counts toward nor breaks the
  two-run streak. Run 1 hit this at 23:20Z: the Microsoft AutoUpdate helper
  used about 315% CPU and pushed load1 to 8.8 while the gate's own test
  binary used under 10%.

## Session Summary

- **E1 audit:** done; see GT-D8.
- **Controls (on local-only fixture commits):**
  - C1 collision pair: the run FAILED with shards 17 and 18 both exit 1, both
    attributed, intervals overlapping (61.2–216.4 s).
  - C2 stray write: both commands exited 0, yet the run FAILED with
    `?? internal/cli/gt-stray.txt` as a checkout mutation.
  - Both control leftovers were removed; the control clone is clean.
- **E2 run 1:** launched 23:10:07Z on the scratch clone at `85a6763` with the
  GT-D8 plan.
- **E3 draft (uncommitted):** `scripts/wave-close-test-lanes.sh`, the
  Makefile opt-in, new guards in `internal/cli/wave_close_lanes_test.go`, and
  updated parity checks in `prepare_s7_ci_timeout_guard_test.go`. In a clean
  clone: gofmt clean, targeted guards PASS, `go vet ./...` and
  `go build ./cmd/tpatch` PASS.

Dispatch baseline verified at `7e0d63f`:
- HEAD equals origin/main;
- no tracked changes, no stash;
- a single worktree;
- no merge, rebase, cherry-pick or index-lock state.

## Current State

- Serial baselines: S9 `5133c95` at 13,081 s; GH #9 `b8a5f37` at 13,093 s.
  Both PASS.
- Controls C1 and C2 behaved as intended. E2 run 1 in progress.

## Files Changed

- Committed: tracking only (`docs/handoff/CURRENT.md`, `docs/ROADMAP.md`,
  `docs/supervisor/LOG.md`).
- Uncommitted E3 draft: `scripts/wave-close-test-lanes.sh` (new),
  `Makefile`, `internal/cli/prepare_s7_ci_timeout_guard_test.go`,
  `internal/cli/wave_close_lanes_test.go` (new).

## Test Results

- E3 draft, clean clone of `21be56f` plus the four files, after a 60 s
  GT-D10 window: `gofmt -l .` empty;
  `go test -p=1 ./internal/cli -count=1 -run
  '^Test(GH9WaveClose.*|S7WaveClose.*|S7CIFullSuiteTimeoutGuard|S7ARPermanentBlockClaimsGuard)$'`
  ok (48.8 s); `go vet ./...` ok; `go build ./cmd/tpatch` ok.
- Every new guard fixture is rejected, including a sequential runner (caught
  by the lane rendezvous), a dropped preamble, a masked lane status, a
  missing refusal and a drifted partition.

## Next Steps

1. Analyse E2 run 1; launch run 2 if it passes.
2. If E2 meets GT-D7: AGENTS.md paragraph, independent review, commit and
   push E3, native CI, then old-Mac gates in lane and serial modes.
3. Otherwise: go/no-go report with the measured numbers, E3 left
   uncommitted.

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
