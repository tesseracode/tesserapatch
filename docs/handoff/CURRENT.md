# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**S8 DISPATCHED — acceptance transaction and explicit recovery.**
S7 closed at `d65150ce02668d095a0f4dffce9571ba3fd7096b`; fetch confirmed
clean HEAD=origin/main, an empty stash and one main worktree before dispatch.
Its terminal handoff is already archived byte-for-byte in HISTORY under
2026-11-03T13:01:09Z (verified before replacing CURRENT).

The operator authorized a fresh twelve-hour window starting
**2026-11-03T16:55:00Z**, ending **2026-11-04T04:55:00Z**
(November 3, 08:55–20:55 PST). Stop safely at the deadline and ask before any
extension. This task is GH #13 S8 only: no S9, release, unrelated migration,
coverage expansion or upstream-Git research; never start #9.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s8`
- **Milestone**: GH #13 / ADR-037 and ADR-043 through ADR-050
- **Description**: Journaled acceptance, authoritative per-path CAS, residual artifact publication and explicit recovery
- **Status**: In progress — contract review and single-implementer dispatch
- **Assigned**: 2026-11-03
- **WAVE_BASE**: `d65150ce02668d095a0f4dffce9571ba3fd7096b`
- **Contract**: accepted PRD rev-7, sections 6.13–6.16, 7.4 and S8; ADR-044 held-lock amendment
- **Owner**: one implementation agent; parent owns tracking and validation; independent source review before acceptance
- **Deadline**: `2026-11-04T04:55:00Z`

WAVE_BASE = d65150ce02668d095a0f4dffce9571ba3fd7096b

## Session Summary

Verified the pushed S7 closure and recorded the fresh wave base before code.
S8 replaces S7's honest exit-3 acceptance/recovery placeholders with the
accepted fourteen-step transaction. No S8 code or Go validation has run yet.
The parent will preserve every review verdict and phase transition here and
in LOG/ROADMAP, committing tracking as work advances.

## Current State

S0–S7 are accepted. S7's complete gate at `8b84b33` passed 42/42 commands,
39/39 uncached test commands, exact parity and zero swap-outs; native CI
41612473590 completed successfully. Its allowed-failure Windows full-suite
log was not compared at S7 close; do not call that suite green or close #17.
The full S7 record remains in HISTORY, including its review limitations.

S8 requirements:

- Hold feature then global locks throughout acceptance; borrow the existing
  ADR-044 capabilities instead of reacquiring or releasing inside helpers.
- Refuse journals first, strict-decode pending state, recheck source feature
  state, E1–E15 and the complete immutable identity (canonical E plus raw C,
  patch and recipe hashes); rederive and rematerialize, rerunning postcondition,
  idempotency and configured tests before touching live candidate paths.
- Require upstream ancestry, not HEAD equality. Exact content/mode CAS covers
  every candidate path twice, at step 6 and immediately before step 10's first
  live write. Created paths must be absent; unrelated dirt stays untouched.
- Stage residual patch, regenerated recipe, truthful provenance, the shared
  ADR-024 generation entry, E and C before any live write. The generation,
  coverage reference and Apply.BaseCommit must name the same upstream commit.
- Snapshot every touched live path and all seven named feature files:
  patch, recipe, provenance, capture event, coverage, generations and status.
  Preserve exact prior bytes/absence and Apply/State; fsync journal and
  directory before writes. Publish through the shared ADR-036 API, E before C.
- Pre-marker failures restore both sides; step-10 CAS refusal retains a
  recoverable no-marker journal. Completed rollback exits 3; failed rollback
  or failed explicit resume exits 1 and retains recovery evidence. Commit
  marker selects cleanup rather than rollback; ordinary commands never resume.
- Focused runtime/failure/crash tests and mutation-sensitive production guards
  must cover the accepted S8 matrix, including all six Apply.BaseCommit readers,
  E/C one-sided absence and publication order. No golden regeneration.

## Files Changed

- `docs/handoff/CURRENT.md`: fresh S8 scope, deadline and validation discipline.
- `docs/ROADMAP.md`: S8 in-flight row, S7 closure retained.
- `docs/supervisor/LOG.md`: operator authorization and S8 dispatch.

## Test Results

No S8 validation yet. Documentation-only dispatch does not run Go.
No prior partial gate result will be reused as S8 acceptance evidence.

## Validation Policy

Reuse the accepted resource guard for EVERY Go/gofmt command, including
formatting and targeted checks: 60 continuous seconds of normal OS pressure,
zero new swap-outs and load1 <= 5, with a 600-second admission budget.
Free-memory percentage and swap occupancy are diagnostic, not new thresholds.
Stop on warning/critical pressure, any new swap-outs, unreliable telemetry or
sample gaps over 5 seconds. External Go is observed; our own work stays serial.
No local/remote Go overlap, raw unguarded runs or weakened guards.

Use the private Go 1.26.5 toolchain with GOTOOLCHAIN=local, GOENV=off, empty
GOFLAGS, GOWORK=off, BASH_ENV=/dev/null, GOMAXPROCS=1, -p=1,
GOMEMLIMIT=3GiB (advisory budget, not RSS cap), -count=1 and 40m test timeouts.
Stop on the first failure and preserve the true exit status (no tail pipeline
masking). Do not enable TPATCH_UPDATE_ROC_S0_GOLDENS.

After focused/affected tests, vet and build, run a fresh complete gate with
isolated full-history source/caches on the authorized old Mac:
`make wave-close-check WAVE_BASE=d65150ce02668d095a0f4dffce9571ba3fd7096b`.
The existing CI-equivalent partition has 42 top-level commands / 39 test
commands. No shard changes or partial-result stitching. Freeze the pushed
reviewed tip during the gate and check exact-tip native CI, comparing actual
Windows failures against the known baseline. A resource stop is recorded and
put to the operator; no blind retry.

## Next Steps

1. Commit/push the dispatch record; implement S8 with one code owner and a
   Proposed ADR-051 for transaction/recovery implementation decisions.
2. Run guarded focused checks, independent review and fix/review cycles until
   the final code has an explicit approval.
3. Complete the fresh full gate and exact-tip CI, then accept the ADR, archive
   the terminal handoff, update ROADMAP/LOG and push a clean closure.
4. Do not start S9 or #9 in this S8 dispatch.

## Blockers

None at dispatch. The deadline timer is `roc-s8-deadline`.

## Context for Next Agent

The previous S6/S7 gates are finished. Preserve their local ignored evidence
under `bin/roc-s6-20261031/` and remote evidence under the authorized
`tpatch-validation-s5-fBIN5H09` validation root; never overwrite/reuse an old
evidence directory or remove unrelated remote processes/data.
Explicit file-path staging only, required Copilot trailer, no amend/reset/clean.
Keep S0 goldens, nineteen accepted AR memo-lifetime insertions, assets, SPEC,
CI/scripts, dependencies, research, frozen ADR index and v0.17.0 unchanged.
Reviewed test-provenance amendments use CurrentSHA256 while preserving the
original SHA256/History/FreezeCurrent. Record any strictly necessary scope
addition before changing it; S8 may use shared publication/generation APIs,
not new parallel writers or silent changes to resolver acceptance.
