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
- **Status**: In progress — E2 run 1 FAILED (shard 31 observer budget under
  overlap, GT-D18); run 2 with the solo-tail plan running remotely since
  00:23:49Z, all shards so far exit 0, observable again after a NetBird
  outage (GT-D22); E3 rev-2 (GT-D19–D21) validated locally and sent for
  re-review, uncommitted until E2 meets the target (2026-11-05T01:15Z)
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
- **GT-D13, how the lane-mode old-Mac gate runs.** Private tooling under
  `bin/gt-20261104/lane-gate/`; the accepted serial gate tools are not
  modified.
  - `run-gate-lanes.py` runs
    `make wave-close-check WAVE_BASE=… WAVE_CLOSE_TEST_LANES=2` with the
    serial gate's environment plus `GOMEMLIMIT=3GiB`, which matches E2 and
    the serial `run-full-gate` launcher. It scrubs the seven variables the
    lane script refuses, as well as GOOS, GOARCH and the gate nonce.
  - `gate-tool-lanes.py` is the `go`/`gofmt` wrapper. It allows at most two
    concurrent commands and applies the GT-D2 amortized admission against
    the runner's single 1.5 s monitor. Any failure stops the other lane at
    its next command.
  - The runner passes only if:
    - the gate passes, announcing lane mode;
    - the 42 commands match the expected multiset, and the three
      preflight commands match in order;
    - lanes actually overlapped;
    - stage clocks are consistent;
    - no runtime stop, block marker or leftover process occurred;
    - the source is clean afterwards and the v0.17.0 tag is unchanged.
  - Amortized admission is a proposed protocol change, so a lane-gate PASS
    is evidence only. The serial gate under the accepted per-command
    protocol remains the authority for the E3 commit.
- **GT-D14, cross-process clock.** On both Macs, `/usr/bin/python3` 3.9.6
  starts `time.monotonic()` near zero in every process. A freshness check
  comparing a wrapper's clock with the monitor's therefore always passed.
  The local end-to-end smoke test exposed this; in-process unit controls
  could not. Every cross-process timestamp now uses the boot-relative
  `CLOCK_MONOTONIC`. A control fails if the wrapper's clock disagrees with a
  child process, and the runner fails if any stage interval falls outside
  its own gate interval. The E2 harness is unaffected because it times
  everything within one process.
- **GT-D15, shard-01 isolation becomes a barrier (review finding 1).**
  Shard 01 creates and removes an `intent-archive-shell-*` scratch
  directory under `internal/store/` while the `./...` shards walk the tree.
  The E3 draft kept them apart only by timing: lane B's CLI prefix was
  expected to outlast shard 01 by about 934 s. The lane runner now
  enforces it. Lane A records shard 01's status atomically, then continues
  only if it passed. Lane B runs its CLI prefix, then waits for that record
  before any `./...` shard. If shard 01 failed, or lane A exits without a
  record, lane B stops and the run fails.
  - Rejected: a three-phase schedule with a global wait, because lane A
    would idle for about 934 s.
  - E2 keeps the run-1 plan so that both runs measure the same schedule.
    `analyze-e2.py` reports how many seconds the barrier would have held
    lane B and adds them to the projected wall time.
- **GT-D16, cancellation stops the whole process tree (review finding 2).**
  The draft's trap killed only the two lane shells. `go test` and its test
  binaries survived an interrupted gate. The trap now lists every
  descendant of the runner with `ps -A -o pid= -o ppid=` before signalling
  any, sends TERM to all of them, waits, and removes the temporary
  directory. A behavioural fixture checks that grandchild processes are
  gone after SIGTERM.
- **GT-D17, recording-switch discovery parses Go (review finding 3).** The
  draft matched only double-quoted names, so a raw string or a
  concatenation hid a new `TPATCH_RECORD_*`/`TPATCH_UPDATE_*` switch from
  the refusal-list guard. Discovery now parses every `.go` file under
  `cmd/`, `internal/` and `tests/` and folds string constants: raw and
  interpreted literals, `+` concatenation, parentheses and package-level
  constants. It fails closed on any constant fragment that contains a
  switch prefix but is not a complete name.
- **GT-D18, E2 run 1 is a FAIL; three observer shards move to a serial
  tail.** Shard 31 (`TestS7ObservedAMThroughAORegistrationAuthority`)
  exited 1 at 00:07:08Z after 303 s with "observed registration missing
  out-of-band markers (timed out)". GT-D12 does not apply: a test failure
  is a FAIL, not an inconclusive miss.
  - Root cause: the hosted observer tests run a child `go test` under a
    fixed wall-clock budget (`s7ObservedHostedBudgets`,
    `internal/cli/prepare_s7_registration_test.go`). For AM-AO that budget
    is 4 m. In the serial baseline the whole test took 264.6 s, so the
    child used about 88% of its budget. Two-lane overlap slowed shards by
    ×1.02–1.18 (shard 31 by ×1.13; likely the shared turbo and thermal
    budget, since load stayed at 2–2.7 on 8 threads, pressure was normal
    and there were no swap-outs). That exceeded the budget.
  - Decision: an observer shard whose serial child run uses more than half
    of its fixed budget runs alone, after both lanes have passed. That is
    shards 31 (about 88% of 4 m), 32 (AP, about 65% of 8 m) and 33 (AQ,
    about 83% of 8 m). Shards 17 and 29 move from lane A to lane B to
    rebalance.
  - Model: lane A 4,705 s, lane B 4,657 s, tail 1,076 s, preflight 82 s;
    makespan 5,863 s plus admission. Lane B reaches its first `./...` shard
    at about 3,241 s and shard 01 ends at about 2,866 s, so the GT-D15
    barrier should not hold.
  - The other child-`go` budget, `TestPrepareArtifactUnstableMutationControls`
    (3 m outer, 2 m inner, in shard 01), passed under overlap in run 1. It is
    monitored, not moved.
  - Rejected: raising the test budgets. That changes test semantics and is
    outside this window. The ~10–15% serial headroom of AM-AO is a latent
    flake risk even in serial mode; it is recorded as a backlog note in
    `docs/ROADMAP.md`.
  - E2 runs 2 and 3 use this plan
    (`bin/gt-20261104/plan-2lane-tail.json`). The private tool mirrors the
    GT-D15 barrier and the tail, and logs when each shard becomes ready, so
    holds are measured from ready to launch.
- **GT-D19, cancellation escalates to KILL (rev-1 review R1-F1).** TERM
  alone left TERM-ignoring descendants running (for example
  `TestRunawayChildIsBoundedToCapPlusOne` uses `trap '' TERM`). After TERM
  and CONT, the runner polls for up to 10 s. It then freezes the whole tree
  again from scratch, so a survivor is stopped too and cannot start a new
  process between the listing and the kill, and sends KILL to everything
  from either freeze that is still alive. Zombies count as exited.
- **GT-D20, cancellation also finds reparented and escaped descendants
  (rev-1 review R1-F2).** The PPID scan misses a process whose parent has
  already exited, such as the setsid escapee in
  `buildEscapingDoltCLIFixture`. Before starting the lanes, the runner
  opens a marker file in its temporary directory on descriptor 9, and every
  descendant inherits it. Cancellation stops the union of the PPID tree and
  every process that holds the marker, listed with `lsof` or, on Linux,
  `/proc/*/fd`. Only the runner's descendants can hold the descriptor, so
  no unrelated process is signalled.
  - Rejected: an exported environment token. macOS 27 `ps -E` does not
    show another process's environment, which was checked directly.
  - Rejected: `set -m` process groups. dash refuses job control without a
    tty (the CI case), and zsh cannot set it.
  - Rejected: signalling the runner's own process group. It is shared with
    the caller (make, or the Go test binary in the fixtures).
  - Residual: a descendant that closes descriptor 9 and also loses its
    parent link is not found. Under Ctrl-C the serial gate misses the same
    setsid escapees, and the repository's escapees exit on their own after
    a bounded hold.
- **GT-D21, switch discovery resolves constants by scope and package
  (rev-1 review R1-F3).** The const map was per directory, so `package
  a_test` could overwrite a constant of `package a` with the same name and
  hide a switch.
  - Files are grouped by directory and package name.
  - A reference is bound as the compiler would bind it. The parser's
    resolver binds same-file references, including function-local
    constants and parameters or variables that shadow a constant. A
    reference it leaves unresolved folds to that package's package-level
    constants.
  - A package-level name declared in several build-tagged files folds to
    every declared value, in either file order. Concatenation takes every
    combination; more than 64 values fails the scan closed.
  - Changed from the first plan, which made such a name unfoldable. An
    unfoldable operand would silently hide `part + "UPDATE_X"`, because the
    remaining run carries no prefix.
  - Found during validation: name-only resolution across the whole package
    folded local variables in `internal/cli` (for example `pkg + "\x00" +
    rel`) into 216 values. Lexical binding fixes that, and a fixture with
    shadowing locals guards it.
  - Rejected: `go/types` type-checking of every package. It is too heavy for
    a guard test and needs source imports of the whole module.
  - `ast.Object` is deprecated, but it is the only scope information
    available without type-checking.
- **GT-D22, NetBird logged out mid-run; E2 continues remotely, observed
  later (2026-11-05T00:5xZ).** The local NetBird daemon reports
  `NeedsLogin`, so `macbook-pro.netbird.selfhosted` no longer resolves. The
  run-2 watcher exhausted its retries. Run 2 was started with `nohup` on the
  old Mac and keeps running; its evidence stays in `$G/run2`.
  - NetBird login is an interactive SSO step for the operator; I do not
    attempt it.
  - A local watcher polls NetBird. When the old Mac is reachable again, it
    fetches run 2's state. Run 3 is launched only if run 2 passed and the
    remaining window still covers a full run plus margin.
  - Until then, E3 rev-2 proceeds locally (validation, smoke, re-review).
    GT-D7 cannot be met without old-Mac access, so E3 stays uncommitted.
  - **Resolved 01:07:45Z without operator action.** NetBird reconnected on
    its own (`Management: Connected`). Run 2 had kept running throughout:
    at 01:08:51Z every completed shard had exited 0, lane B launched each
    shard as soon as the previous one finished, and the monitor had not
    stopped the run. A new watcher tolerates ssh failures instead of
    exiting on them.

## Session Summary

- **E1 audit:** done; see GT-D8.
- **Controls (on local-only fixture commits):**
  - C1 collision pair: the run FAILED with shards 17 and 18 both exit 1, both
    attributed, intervals overlapping (61.2–216.4 s).
  - C2 stray write: both commands exited 0, yet the run FAILED with
    `?? internal/cli/gt-stray.txt` as a checkout mutation.
  - Both control leftovers were removed; the control clone is clean.
- **E2 run 1:** launched 23:10:07Z on the scratch clone at `85a6763` with the
  GT-D8 plan. External load (XprotectService, syspolicyd, Microsoft
  AutoUpdate; load1 peaked at 21.9) held lane B from 23:20:53Z to 23:28:11Z,
  a 438 s hold. Lane A's shard 01 kept running but was starved of CPU.
- **Lane-gate tooling (GT-D13):**
  - 16 wrapper controls PASS, and each of 8 mutations fails at least one
    control.
  - Local end-to-end smoke with a fake toolchain and the real profiler,
    lane script and admission:
    - pass case: PASS, 42/42 parity, 4.7 s of lane overlap;
    - shard 3 failing: FAIL, and the other lane stopped after 9 stages.
  - The smoke test found the GT-D14 clock defect, which is now fixed.
- **E2 run 1 FAILED** at 00:07:08Z: shard 31 exceeded its 4 m observer
  child budget under two-lane overlap (GT-D18). Per-package `ok` for every
  other completed shard. Shard 01's `internal/cli` took 1,556 s of its
  40 m timeout.
- **E2 run 2** launched 00:23:49Z with the GT-D18 solo-tail plan. The first
  attempt was refused because run 1's build step left an ignored `tpatch`
  binary in the scratch clone; the tool's clean-checkout check worked as
  intended. The binary was removed before relaunch.
- **E3 rev-1** (GT-D15–D17) in a clean clone: gofmt clean, `go vet
  ./internal/cli` ok, targeted guards ok (69 s), all 6 runner mutations
  caught, and a regex-scanner mutation fails the discovery test. Lane-gate
  smoke with the rev-1 script: PASS, 42/42, no shard-01 overlap.
- **Rev-1 review** (`gpt-6.1-sol`): NEEDS REVISION, 3 MEDIUM (GT-D19–D21).
- **E3 rev-2** (GT-D18–D21) in a clean clone of `54177df` plus the four
  files: gofmt clean, `go vet ./internal/cli` ok, targeted guards ok
  (120 s). Mutation evidence:
  - all 12 runner mutations are caught, including KILL escalation removed,
    escapees not tracked, the solo tail running after a lane failure, a
    masked solo status, and solo shards moved into lane B;
  - the no-`lsof`/no-`/proc` refusal is exercised on macOS;
  - 4 discovery mutations each fail the discovery test: name-only
    resolution, package keyed by directory only, first declaration wins,
    and no value cap.
- **Lane-gate smoke with the rev-2 script:** PASS, 42/42, exact parity, no
  shard-01 overlap, clean source. The call log shows the three solo shards
  starting after every lane command ended, one at a time, in serial order.
  Two earlier smoke attempts failed on harness setup, not on the runner: I
  copied the real Makefile into the stub repo, and I ran without `python3
  -B`. Both are recorded in the private evidence.
- **NetBird logged out** around 00:50Z and reconnected on its own at
  01:07:45Z (GT-D22). Run 2 continued throughout. Lane B's shards are
  within about 3% of their estimates, which projects completion around
  02:02Z (about 5,920 s, target 6,540 s).
- **Analyzer** (`bin/gt-20261104/analyze-e2.py`, private) now measures
  holds from `ready` to `launch` and checks the GT-D18 solo tail (no
  overlap, after every lane shard) and the GT-D15 barrier (module-walking
  shards start only after shard 01 exits 0). On the fake-toolchain smoke,
  the clean run reports no violations, and copies mutated to break each
  rule are each caught.
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
- Controls C1 and C2 behaved as intended. E2 run 1 FAILED (GT-D18); run 2
  running remotely, all completed shards exit 0 (GT-D22 resolved). GT-D7
  now needs runs 2 and 3 to pass consecutively.
- E3 rev-2 passes local guards, mutation checks and the lane-gate smoke;
  awaiting re-review.

## Files Changed

- Committed: tracking only (`docs/handoff/CURRENT.md`, `docs/ROADMAP.md`,
  `docs/supervisor/LOG.md`).
- Uncommitted E3 draft: `scripts/wave-close-test-lanes.sh` (new),
  `Makefile`, `internal/cli/prepare_s7_ci_timeout_guard_test.go`,
  `internal/cli/wave_close_lanes_test.go` (new).

## Test Results

- E3 rev-2, clean clone of `54177df` plus the four files, each run after a
  60 s GT-D10 window: `gofmt -l .` empty; `go vet ./internal/cli` ok;
  `go test -p=1 ./internal/cli -count=1 -run
  '^Test(GH9WaveClose.*|S7WaveClose.*|S7CIFullSuiteTimeoutGuard|S7ARPermanentBlockClaimsGuard|S7WaveCloseShardParity)$'`
  ok (120.3 s). The first run failed on the real repo (GT-D21, 216-value
  fold) and was fixed before this result.
- E3 rev-1 draft (superseded), clean clone of `21be56f`: `gofmt -l .` empty;
  `go test -p=1 ./internal/cli -count=1 -run
  '^Test(GH9WaveClose.*|S7WaveClose.*|S7CIFullSuiteTimeoutGuard|S7ARPermanentBlockClaimsGuard)$'`
  ok (48.8 s); `go vet ./...` ok; `go build ./cmd/tpatch` ok.
- Every new guard fixture is rejected, including a sequential runner (caught
  by the lane rendezvous), a dropped preamble, a masked lane status, a
  missing refusal and a drifted partition.

## Next Steps

1. E3 rev-2 re-review (same reviewer, no Go runs).
2. When run 2 exits (projected about 02:02Z): fetch and analyse it; launch
   run 3 if it passed and the window still fits a full run.
3. If E2 meets GT-D7: AGENTS.md paragraph, commit and push E3, native CI,
   then old-Mac gates in lane and serial modes.
4. Otherwise: go/no-go report with the measured numbers, E3 left
   uncommitted.

## Blockers

- None. The NetBird outage (about 00:50Z to 01:07:45Z) resolved without
  operator action.

## Context for Next Agent

- **Old Mac:** `ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o
  ConnectTimeout=20 -o ForwardAgent=no -o IdentitiesOnly=yes -i
  ~/.ssh/id_ed25519 jbencardino@macbook-pro.netbird.selfhosted`
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Private evidence:** `bin/gt-20261104/` (ignored).
- **CI pushes:** `.github/workflows/ci.yml` cancels in-progress runs per ref,
  so do not push to `main` while source CI is running.
