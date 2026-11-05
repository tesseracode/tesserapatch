# Current Handoff

## Status

**Cluster state**: APPROVED

**Lane-mode adoption + F-V18-1: code APPROVED (review r3), gates next.**
The operator approved GT-D27 on 2026-11-05: accepted wave closes use lane
mode with one 60 s pressure admission per gate. This window makes lanes the
Makefile default, fixes F-V18-1, and drafts the GH #25 PRD while waiting on
gates. The previous gate-time handoff (decisions GT-D1…D30) is archived in
`docs/handoff/HISTORY.md` under **2026-11-05T08:43:11Z - Gate-time
reduction: lane mode shipped opt-in**; its lane gate addendum is in commit
`4382c33`. APPROVED is set at the code commit, before the gates, as in
GT-D28 (LW-D7).

- **Window:** 2026-11-05T18:27:19Z → deadline **2026-11-06T00:27:19Z**.
- **WAVE_BASE = 4382c33ec414da18fe45b4c4bc2011bbdee1674f**
  (`make wave-close-check WAVE_BASE=4382c33ec414da18fe45b4c4bc2011bbdee1674f`).
- **Baseline verified at dispatch:** HEAD == origin/main == `4382c33`, no
  tracked changes, no stash, one worktree, no merge/rebase/cherry-pick
  state, CI idle. (Correction: the dispatch text said nine untracked
  research files were allowlisted. Those files were integrated in `0ca98db`
  on 2026-09-22; there were no untracked files at dispatch, and
  `.wave-close-allowlist` has no entries.)

## Active Task

- **Task ID**: wave-close-lanes-adoption-and-fv18
- **Milestone**: Post-v0.18 backlog — validation infrastructure (GT-D27 adoption) + F-V18-1
- **Description**: A — make `WAVE_CLOSE_TEST_LANES=2` the wave-close default
  (`0` selects serial) and codify the lane protocol; B — fix F-V18-1 (the
  `docs/agent-as-provider.md` recipe example is rejected by the strict
  decoder) with a strict-decode guard, filed and closed as a GitHub issue;
  C — GH #25 planning PRD draft (Draft only, written during gate waits);
  D — native CI plus the first accepted lane-mode wave close.
- **Status**: Review APPROVED (r3); local validation r3 PASS; committing,
  then CI and the old-Mac lane gate
- **Assigned**: 2026-11-05

## Operator approvals (2026-11-05, single question set)

- **GT-D27 adopted:** accepted wave closes use lane mode, with one 60 s
  pressure admission per gate.
- **Scope:** A, B and C (GH #25 draft only).
- **Makefile default:** flip to lanes; `WAVE_CLOSE_TEST_LANES=0` runs serial.
- **Fallback:** if external pressure stops a lane gate, wait for calm,
  retry lanes once, then fall back to serial.
- **F-V18-1:** file it as a GitHub issue and close it with the fix.

## Session Summary

- Dispatch: baseline verified, approvals and plan recorded, ROADMAP row 🚧,
  LOG dispatch entry (`b41a073`).
- B: F-V18-1 filed as **GH #27**. The `"version": 1` field is removed from
  the `docs/agent-as-provider.md` recipe example. `assets/assets_test.go`
  now strict-decodes that example too (`TestDocRecipeSchemaMatchesCLI`,
  sharing one helper with the skill guard), and
  `TestRecipeSchemaExampleGuardSensitivity` proves the helper fails when the
  version field returns, the feature is dropped, an op type is renamed, or
  the example is removed.
- A: `WAVE_CLOSE_TEST_LANES ?= 2` is the Makefile default; check 8 runs the
  serial script only for `0` and prints the selected mode. The static guard
  now requires the default (`validateS7WaveCloseLaneDefault`), and the new
  behavioral `TestS7WaveCloseCheckModeSelection` runs check 8 under `make`
  with stub runners for default, explicit, serial, environment, empty,
  unknown-count and failing-runner cases, plus eight mutation fixtures.
  AGENTS.md, REVIEW-PLAYBOOK §1.8 and the lane script header describe lanes
  as the default with the approved fallback.
- C: GH #25 PRD draft started in the background (planning only).
- Review r1 (independent, OpenAI model, no Go): NEEDS REVISION with two
  MEDIUM findings: the lane count was word-split (`2 ` ran lanes), and both
  guards read past the wave-close-check rule. Both were fixed forward (quoted
  argv via `set --`; new `s7WaveCloseCheckRule` bounds the guards), with
  new failure-proving fixtures. Re-review r2 found F3 (MEDIUM): a later
  rule naming `wave-close-check` would override the guarded recipe; fixed
  by counting every rule that names it and refusing `include`/`$(eval`,
  with five redefinition fixtures in both guards. Re-review r3 APPROVED;
  local validation r3 PASS.
- C: the GH #25 draft (`docs/prds/PRD-overlap-verification-consolidation.md`,
  Draft, planning only) passed an independent claims check (r1 NEEDS
  REVISION with 2 HIGH + 3 MEDIUM, fixed; r2 APPROVED WITH NOTES, LOW cite
  fixed). It recommends C (read-only assessment) first, then A for the
  single-child case behind an ADR-013 amendment, with B as a runbook, and
  lists twelve open questions for the operator. Committed separately from
  the code.

### Decisions (for operator review)

- **LW-D1, window and base.** The window starts at the baseline check
  (18:27:19Z) and ends 6 h later. WAVE_BASE is `4382c33`, origin/main at
  dispatch.
- **LW-D2, values of the switch.** `WAVE_CLOSE_TEST_LANES` defaults to `2`.
  `0` selects the serial script; every other value, including an explicit
  empty value or one with surrounding spaces, fails check 8 closed (the
  count reaches the lane script as one quoted argument, and the lane script
  rejects anything but `2`). An explicit empty value no longer means serial,
  so an old command line cannot silently pick a mode.
- **LW-D3, local validation.** As GT-D10: local free memory sits near 67%,
  under the 80% rule, so local Go runs use the accepted protocol (60 s of
  pressure level normal, no new swap-outs, load1 ≤ 5, no Go process) in a
  clean temporary clone. Full gates run on the old Mac.
- **LW-D4, gate harness.** The private old-Mac harness is a copy of the
  accepted gate-time lane harness with three changes: it runs
  `make wave-close-check WAVE_BASE=<base>` with no lane argument (so the
  gate exercises the new default), scrubs `WAVE_CLOSE_TEST_LANES` and the
  make-flag variables from the child environment, and requires the
  `(lanes: WAVE_CLOSE_TEST_LANES=2)` banner. Tool hashes are pinned again.
- **LW-D5, no CHANGELOG entry.** The opt-in commit had none, and lane mode
  is validation infrastructure, not shipped CLI behavior. The F-V18-1 doc
  fix is recorded on GH #27. The next release can mention both.
- **LW-D6, local test scope.** Locally: gofmt, the targeted assets and
  wave-close tests, the full `assets` package, `go vet ./...`, `go build`,
  and negative controls (old doc and old Makefile must fail the new
  guards). The full `internal/cli` suite is left to CI and the old-Mac lane
  gate, which run it anyway.
- **LW-D7, APPROVED before the gates.** As GT-D28: check `[5/8]` needs a
  terminal token at the HEAD it checks, and the gate must run on the pushed
  code commit, so that commit carries APPROVED (review r3 APPROVED, local
  validation r3 PASS). If CI or the lane gate fails, the state returns to
  IN PROGRESS in a tracking commit and the fix goes forward.
- **LW-D8, three fix-forward rounds.** All three review findings were
  accepted as written (word-split count, unbounded guards, overriding
  rules). The rule check also refuses `include` and `$(eval`, which the
  reviewer did not ask for, because both can redefine the target out of the
  guard's sight. Exotic make metaprogramming beyond that is out of scope.

## Current State

- A and B committed with review APPROVED (r3) and local validation r3 PASS.
  Next: push when the dispatch CI run is done, then CI and the old-Mac lane
  gate at the pushed commit.
- **Known risk:** the hosted observer AM–AO uses about 88% of its 4 m child
  budget in serial mode (ROADMAP "Observer child-budget headroom").

## Files Changed

- Dispatch: `docs/handoff/CURRENT.md`, `docs/supervisor/LOG.md`,
  `docs/ROADMAP.md`.
- A: `Makefile`, `internal/cli/wave_close_lanes_test.go`,
  `internal/cli/prepare_s7_ci_timeout_guard_test.go`,
  `scripts/wave-close-test-lanes.sh`, `AGENTS.md`,
  `docs/REVIEW-PLAYBOOK.md`.
- B: `docs/agent-as-provider.md`, `assets/assets_test.go`.
- C: `docs/prds/PRD-overlap-verification-consolidation.md` (new, Draft).

## Test Results

- **Local validation r1 PASS** (clean clone, admitted 18:37:20Z, done
  18:39:47Z): gofmt clean; targeted assets guards; targeted
  `TestS7WaveClose(ShardParity|LaneParity|CheckModeSelection|LaneRunner|RecordingSwitchDiscovery)`
  (128.1 s); full `assets` package; `go vet ./...`; `go build`. Negative
  controls failed as required and for the intended reason: the old doc
  fails `TestDocRecipeSchemaMatchesCLI` with `unknown field "version"`; the
  old Makefile fails `TestS7WaveCloseCheckModeSelection` (default ran
  `serial`) and `TestS7WaveCloseShardParity` (old block).
- **Local validation r3 PASS** (after the review fixes; done 18:51:57Z):
  same steps as r1. Every new subtest ran and passed, including the
  `lane-count-unquoted`, `check-8-moved-to-sibling-target` and five
  redefinition fixtures in both guards; the same negative controls failed
  as required. (r2 was stopped before completion because F3 changed the
  code under test.)
- Last accepted gates: serial PASS at `6b3a4ac`
  (14,326.9 s); evidence-only lane PASS at `6df4ced` (6,329.6 s); CI
  42011960057 (`4382c33`) green.

## Next Steps

1. Push the A+B commit (`c03da36`) and the C commit together once the
   dispatch CI run (`b41a073`) has finished; watch CI.
2. D: lane gate on the old Mac at the pushed HEAD, with the default
   command and no lane argument (fallback per approvals).
3. Closeout: close GH #27, HISTORY archive, IDLE CURRENT, LOG, ROADMAP ✅,
   push, CI.

## Blockers

- None.

## Context for Next Agent

- **Old Mac:** `ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o
  ConnectTimeout=20 -o ForwardAgent=no -o IdentitiesOnly=yes -i
  ~/.ssh/id_ed25519 jbencardino@macbook-pro.netbird.selfhosted`
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Private evidence:** `bin/gt-20261104/` and `bin/lw-20261105/` (ignored).
- **CI pushes:** `.github/workflows/ci.yml` cancels in-progress runs per ref,
  so do not push to `main` while source CI is running.
