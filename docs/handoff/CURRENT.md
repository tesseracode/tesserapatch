# Current Handoff

## Status

**Cluster state**: APPROVED

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
- **Status**: Approved (three review rounds) and locally validated; push, CI and validation-machine gate pending
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
- **W0 implemented:**
  - **Fixture leak.** `git daemon` runs `git-daemon` as a child (macOS adds
    an `xcrun` shim too), so the fixture's `Kill()` ended only the wrapper.
    The fixture now starts `$(git --exec-path)/git-daemon` directly with a
    `--pid-file` (NW-D8). The new guard
    `TestPartialCloneFixtureDaemonStopEndsDaemon` checks that the recorded
    daemon PID is the started process and that the port refuses connections
    after `stop()`. Both mutants fail it: the old wrapper launch (PID
    mismatch, port still open) and a `stop()` without `Kill`/`Wait` (port
    still open).
  - **`make release-check`.** `scripts/release-check.sh` runs nine
    fail-fast checks on committed bytes (see the new AGENTS.md checklist
    item). Live runs against real history: v0.19.0 with GATED `2e44569`
    fails `[4/9]` and lists `CHANGELOG.md` and the pin file (the OV-D18
    case); v0.18.0 with GATED `5133c95` passes `[1/9]`–`[8/9]` and fails
    `[9/9]` because the tag exists.
  - **Local dev machine:** 84 orphaned fixture daemons from earlier local
    test runs were terminated (NW-D5).
- **W0 review round 1 (gpt-6.1-sol): NEEDS REVISION**, two MEDIUM findings,
  both fixed:
  - **F1:** `[4/9]` filtered with `:(exclude,literal)` pathspecs, and
    `GIT_LITERAL_PATHSPECS=1` turns those into plain file names, so the diff
    came back empty and the check passed (reproduced on the real v0.19.0
    case).
  - **F2:** a literal exclude of a tracking file also hides files under a
    directory that replaces it (`docs/ROADMAP.md/extra.go`).
  - **Fix:** `[4/9]` now lists every changed path with
    `git diff --no-renames --name-only` and no pathspecs, and compares each
    path exactly against the four tracking names. `--no-renames` is
    load-bearing again: with rename detection, a source file renamed onto a
    tracking name would list only the tracking name.
- **W0 review round 2: APPROVED WITH NOTES; round 3: APPROVED.** The
  round 2 LOW: the `tracking-prefix-allowed` mutant's inline `case` did not
  parse under macOS `/bin/sh`, so a syntax error killed it, not the case it
  targets. Fixed with parenthesized patterns. The mutation test now also
  runs `sh -n` on every mutant and fails with "mutant does not parse"; with
  the old mutant restored it failed exactly that way. Round 3 closed the
  LOW with no further findings.
- **Makefile placement:** the `release-check` target had landed inside the
  `wave-close-check` comment block; it now sits right after `all:` (no
  behaviour change).
- **D done (own commit after its own review):** the GH #26 and GH #24 PRDs
  record the operator's §9 answers, and ADR-013 gains a PROPOSED
  Amendment 4 draft for GH #26 N0 (D40–D45, T1–T10). Design review
  (gpt-6.1-sol) round 1 NEEDS REVISION, F1 (MEDIUM): the R14b text claimed
  the replay baseline "predates" the member's landing, but non-ancestry
  also covers divergent branches. Fixed (the text now says whether an
  exact landing is in the baseline's history; new test row T10); round 2
  APPROVED. Amendment 4 stays PROPOSED until the operator accepts it.
- **Validation-host automation (private, OV-D17 answer):** a runner
  pre-flight and automatic serial fallback, kept outside the repository
  (NW-D9, NW-D10).

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
| NW-D5 | Terminated 84 orphaned fixture daemons on the local dev machine after the same checks (ours, parented by `init`, deleted AC-L68/AC-L69 base path). |
| NW-D6 | The make variable is `RELEASE`, not `VERSION`: the Makefile already uses `VERSION ?= $(shell git describe …)` for build ldflags. The recipe reads `RELEASE` and `GATED` from the environment (make exports command-line variables), so values are never parsed as shell syntax. |
| NW-D7 | `release-check` accepts the gate's terminal Cluster-state tokens (SHIPPED, APPROVED, ACCEPTED, IDLE), so the two tools share one rule; past release tag commits used ACCEPTED, APPROVED and SHIPPED. It runs by hand before `git tag -a`, not inside CI, whose release job only starts after the tag is pushed. |
| NW-D8 | The fixture starts the real `git-daemon` binary instead of killing a process group: process groups need `Setpgid`, which does not compile on Windows, and the test file must still build there. |
| NW-D9 | Private runner pre-flight on the validation host: refuse if a gate runner is active; reap only our own orphaned fixture daemons (ours, parented by `init`, `git-daemon` basename, serving a deleted AC-L68/AC-L69 base path, row rechecked just before the signal); choose serial when swap use is above a limit set just above the highest value seen at the start of a passing run, or cannot be read. Swap use alone did not predict the two lane stops (both started below that value), so the limit only catches the clearly-worse case. A reap failure warns and does not block. |
| NW-D10 | After a lane resource stop, the private runner falls back to serial at once instead of retrying lanes first (operator OV-D17: "automatic serial fallback after a lane resource stop"). The stopped attempt is renamed, not deleted. AGENTS.md's "retry lanes once" sentence is updated to match. A lane stop at the deadline, or any non-resource failure, is a final FAIL. |
| NW-D11 | ADR-013 numbering: Amendment 3 is reserved for GH #25 A1 (decisions D28–D39), which comes later in this window; GH #26's draft takes Amendment 4 (D40–D45) and labels its measurements A4-E1–A4-E6, so neither draft renumbers the other whichever is accepted first. |
| NW-D12 | Local validation runs on repository files only. The ignored `bin/` directory holds private evidence, including old source snapshots with `.go` files. `gofmt -l .`, `go vet ./...` and `TestS7CIFullSuiteTimeoutGuard`'s filesystem walk all read them and fail locally (`TestS7ARExitSixRouteGuard` found again in a snapshot). None of them exist in CI or in the validation machine's clean checkout, where the unscoped commands still run. So locally: gofmt on `git ls-files -co --exclude-standard`, vet on `go list ./...` minus `/bin/`, and that one guard left to CI and the gate. Backlog row added (keep private snapshots from looking like Go source). |
| NW-D13 | Cluster state for the W0 gate: W0 and D are both approved, so the single `**Cluster state**` field is set to `APPROVED` in place before W0's gate, as OV-D10 prescribes (the gate's `[5/8]` rejects a mid-cycle token, and the gate runs at the tip, which also carries D's docs). When W1's ADR draft is committed, the field goes back to `IN PROGRESS` in place, and W1's close commit sets `APPROVED` again. |

## Current State

`origin/main` is at `edd72a3` (window dispatch; v0.19.0 tag on `68142ca`).
W0 is approved by independent review, passed local validation and is
committed locally as `583f7dc`; push (after `edd72a3`'s CI finishes), CI
and the validation-machine gate follow. D is approved and committed on its
own. GH #25 stays open
(A1 authorized in this window; B and C3 not). GH #26 and GH #24 have
operator answers; only GH #26's ADR-013 Amendment 4 draft is authorized,
no code.

## Files Changed

- `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`:
  window dispatch (`edd72a3`) and W0 tracking.
- W0 (`583f7dc`): `internal/workflow/verify_landed_partialclone_test.go`
  (fixture fix + guard); `scripts/release-check.sh` (new);
  `internal/cli/release_check_test.go` (new); `Makefile` (`release-check`
  target); `AGENTS.md` (Wave-Close Checklist release item, NW-D10
  lane-stop sentence).
- D (approved, own commit): `docs/prds/PRD-historical-anchor-relanding.md`,
  `docs/prds/PRD-coverage-operation-domain.md`,
  `docs/adrs/ADR-013-verify-freshness-overlay.md`.

## Test Results

- Baseline CI 42235898642 (`fddf4bd`) green.
- W0 targeted (dev machine): the fixture guard plus AC-L68/AC-L69 pass, and
  both fixture mutants fail the guard. `TestReleaseCheck*` after the review
  fix: 66 PASS, 0 SKIP — 37 script cases, 23 script mutants each failing
  its named case, and 3 Makefile-wiring mutants (shell-pasted values,
  swapped arguments, script not invoked) run under the real `make`. New for
  F1/F2: cases for `GIT_LITERAL_PATHSPECS=1`, a directory replacing a
  tracking file, and a tracking-name prefix (`docs/ROADMAP.md.orig`);
  mutants restoring the pathspec filter (killed by both new cases),
  restoring rename detection, and allowing prefix matches. After the
  round 2 LOW fix: still 66 PASS, 0 SKIP, and every mutant passes `sh -n`.
- W0 local validation (dev machine, serial, 60 s admission): gofmt clean on
  all 730 tracked and unignored Go files; workflow targeted (fixture guard,
  AC-L68/AC-L69, `TestROCS*`) ok in 159 s; cli targeted (`TestReleaseCheck*`,
  `TestGH9*`, `TestS7WaveClose*`, `TestS7MemoryShardCoverageSensitivity`)
  ok in 144 s; `go vet` on the 22 repository packages ok; `go build
  ./cmd/tpatch` ok; 0 orphaned fixture daemons afterwards.
  `TestS7CIFullSuiteTimeoutGuard` was left to CI and the gate (NW-D12).
- Private runner harness (dev machine, stub tools): 12 cases pass; all 10
  orchestrator mutants are killed.

## Next Steps

1. W0: push; CI; validation-machine gate with the new private pre-flight
   (`WAVE_BASE=2e44569`, NW-D2).
2. D: done (approved; Amendment 4 PROPOSED, awaiting the operator).
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
