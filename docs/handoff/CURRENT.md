# Current Handoff

## Status

**Cluster state**: IDLE

**GH #9 shipped and CLOSED/completed (2026-11-04T20:47:37Z); no task is
assigned.** The complete GH #9 handoff, including decisions GH9-D1…D18,
full validation evidence and the GH #25/#26 v0.18 reproduction, is archived
in `docs/handoff/HISTORY.md` under **2026-11-04T20:49:04Z - GH #9
nested-repo sentinel shipped; issue closed**:
- archive block: 23,131 bytes, SHA256
  `c98ad684cfcb064ccd9730a49c5b9722f48e518fd5d6ccb5c393174f62de7abe`;
- HISTORY after the append: 1,193,330 bytes, SHA256
  `37d1d943c4ebcd036d51f29a2e8ed65ce95f5234f1349877252c786d5fdeb829`;
- the prior 1,170,199-byte prefix is unchanged.

The next implementation needs a new explicit operator window and a fresh
WAVE_BASE.

## Active Task

- **Task ID**: none — awaiting operator assignment
- **Milestone**: Post-v0.18 backlog (`docs/ROADMAP.md` → "Current Backlog — Post-v0.18.0")
- **Description**: —
- **Status**: Idle
- **Assigned**: —

## Session Summary

The operator-approved GH #9 window ran from 2026-11-04T16:16:08Z, with its
deadline at 2026-11-05T04:16:08Z.
- **Implementation.** `[2/8]` was moved into
  `scripts/wave-close-untracked-sentinel.sh` (source `b8a5f37`).
- **Review.** The independent review (OpenAI model) APPROVED it after one
  MEDIUM finding was fixed forward.
- **Validation passed.**
  - guarded focused checks `rev1`;
  - native source CI 41873614423;
  - tracking CI 41885082226, after rerunning one macOS job that never got a
    runner;
  - the fresh guarded old-Mac full gate (42/42 commands, 13,093 s).
- **Closure.** GH #9 was closed as completed with a factual closing comment.
- **During the waits, read-only:**
  - the GH #25/#26 reproduction on v0.18.0, where both still reproduce;
  - a new unfiled finding, F-V18-1;
  - a proposal-only plan to halve old-Mac gate time.

### Decisions awaiting operator review (full text in the HISTORY archive)

- **D1:** latest-main health was taken from CI 41844293141, because the
  dispatch-tip macOS jobs were cancelled for runner capacity.
- **D2:** the `[2/8]` logic moved into a POSIX-sh script; the pathspecs stay
  in the Makefile.
- **D3:** the nested-repository model treats `.git` as if absent: `find`
  with `.git` pruned, plus a throwaway index.
- **D4:** Git errors and ambiguous names fail closed.
- **D5:** allowlist entries are never shell-globbed (`set -f`).
- **D6:** the Go sentinel tests are skipped on Windows (POSIX `sh` tooling).
- **D7:** no CHANGELOG entry and no release (gate tooling only).
- **D8:** the scratch index has `splitIndex`, `fsmonitor` and
  `untrackedCache` off.
- **D9:** the closed GH #13 ROC stage fence was frozen to `bbb7a43..064bc38`
  with a sensitivity test. It had diffed the live tree, which would have
  rejected any later change.
- **D10:** no ADR.
- **D11:** the opaque boundary record for a nested repository is replaced by
  the files inside it (review R1).
- **D12:** tracking pushes were held while source CI ran, because the
  workflow cancels in-progress runs.
- **D13:** the GH #25/#26 reproduction ran during the validation wait.
- **D14:** no external posting; drafts are kept for the operator.
- **D15:** F-V18-1 is recorded, not fixed (out of scope).
- **D16:** the gate-time work is a plan only; no experiment ran.
- **D17:** only the macOS job that never got a runner was rerun.
- **D18:** the issue was closed before the closeout tracking push.

## Current State

- **GH #9:** CLOSED/completed. `[2/8]` runs
  `sh scripts/wave-close-untracked-sentinel.sh <18 pathspecs>`.
- **GH #13:** CLOSED/completed. v0.18.0 is published; tag object
  `648afeeee5fcacffdf871177ad022406074b51c3` unchanged.
- **GH #25/#26:** OPEN. Both still reproduce on v0.18.0; the drafted comments
  have not been posted.
- **F-V18-1:** unfiled. The `"version": 1` in the
  `docs/agent-as-provider.md` recipe example is rejected by the strict
  decoder (`recipe-undecodable`), and `apply --mode execute` refuses such a
  recipe.
- **GH #17:** OPEN. The full Windows suite is still known-red, with
  unchanged audits.
- **Live inventory:** 11 open issues, 15 closed issues, zero open PRs.

## Files Changed

Closeout tracking only: `docs/handoff/CURRENT.md`,
`docs/handoff/HISTORY.md`, `docs/supervisor/LOG.md`, `docs/ROADMAP.md`.

## Test Results

No new Go validation was needed for the closeout, which changed tracking
only. GH #9 evidence and final audits are in `bin/gh9-20261104/` (ignored)
and summarized in the HISTORY archive.

## Next Steps

1. Operator: review decisions GH9-D1…D18.
2. Operator: decide whether to post the GH #25/#26 v0.18 comments and
   whether to file F-V18-1. Drafts are in
   `bin/gh9-20261104/repro-v018/DRAFTS.md`.
3. Operator: accept, revise or reject the gate-time experiment plan
   (`bin/gh9-20261104/gate-time-analysis/PLAN.md`; ROADMAP "Validation-
   infrastructure follow-ups").
4. Any next implementation needs a new explicit window and a fresh
   WAVE_BASE.

## Blockers

None.

## Context for Next Agent

The old-Mac harness requires the exact line `=== Mechanical gate: PASS ===`,
so `[2/8]` must stay OK (not WARN) on a normal checkout. Source inside an
untracked nested repository now WARNs unless allowlisted.
`.github/workflows/ci.yml` cancels in-progress runs per ref, so do not push
to `main` while source CI is running (GH9-D12).
The accepted guarded resource protocol is unchanged:
- 60 s admission before every Go run: normal memory pressure, no new
  swap-outs, load1 ≤ 5;
- serial Go 1.26.5.
