# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

**S8 REV-1 UNDER REVIEW — acceptance transaction and explicit recovery.**
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
- **Status**: Review — rev-1 frozen under same-reviewer re-review and guarded affected checks
- **Assigned**: 2026-11-03
- **WAVE_BASE**: `d65150ce02668d095a0f4dffce9571ba3fd7096b`
- **Contract**: accepted PRD rev-7, sections 6.13–6.16, 7.4 and S8; ADR-044 held-lock amendment
- **Owner**: one implementation agent; parent owns tracking and validation; independent source review before acceptance
- **Deadline**: `2026-11-04T04:55:00Z`

WAVE_BASE = d65150ce02668d095a0f4dffce9571ba3fd7096b

## Session Summary

Verified the pushed S7 closure and recorded the fresh wave base before code.
Dispatch tracking is pushed at `6880b1f`. A single OpenAI implementation agent
(`09aaa781-2665-4fdc-a1c8-7619210a364e`) owns S8 code and Proposed ADR-051.
Parent owns CURRENT/LOG/ROADMAP/HISTORY and every Go invocation, avoiding shared
file overlap. S8 replaces the acceptance/recovery placeholders with the
accepted fourteen-step transaction. Validation had not started at dispatch.

The old Mac's new `s8-source` is a clean full-history checkout at the dispatch
commit; `s8-validation/rev0` uses the byte-identical accepted gate wrapper and
profiler. Preflight: pressure 1, load1 2.19, 76% diagnostic free, 51GiB disk
free, exact private Go1.26.5, no active Go workload. No user app was stopped.

Initial implementation returned at 2026-11-03T17:58Z: acceptance/recovery,
rooted transaction I/O, shared generation/publication staging, CLI wiring,
runtime/crash tests and mutation-sensitive guards are written. No Go command
has run yet; `git diff --check` passes. Files are frozen while the parent
formats and validates the exact snapshot remotely. Proposed ADR-051 records
the design. Two review questions remain explicit: same-byte generation/base
conflicts currently fail closed, and native Windows directory-sync durability
is not yet established. Neither is silently accepted as a contract change.
Independent OpenAI reviewer `1eadd48b-81a1-448c-b7b2-9c38d9c4518b`
is examining the full frozen change, including these questions and possible
constant-folding gaps in the new evidence-literal guards. The implementer is
idle/read-only until review and validation results return.

Rev-0 independent verdict: **NEEDS REVISION**. Findings: an ordinary-directory
replacement at `.accept-prune` can be mistaken for owned cleanup data; CLI
journal refusal is later than dirty/preflight/check-only early returns; a
reachable ordinary producer/ancestor case hits the same-byte generation/base
conflict; and the new evidence guard misses constant concatenation/aliases.
No native Windows directory-flush defect was established statically; native
evidence is still required. S8 is not source-approved or accepted.
The operator explicitly selected `candidate-only-generation-amendment`:
append a generation for operation-candidate acceptance when the normalized
base/recipe/capture/upper binding tuple changes despite identical patch bytes.
Do not rewrite historical entries, add a producer/kind/schema, or change any
other producer's same-patch skip. The shared appender remains the only writer;
an identical candidate binding tuple is still a no-op. ADR-051 and a narrow
ADR-024 D9 cross-reference will document this approved contract exception.
The real record/ancestor/conflict reproduction must pass after implementation.
Rev-1 returned at 2026-11-03T19:24Z, frozen without Go execution. It adds
persisted pre-relocation directory/registration identities, early shared CLI
journal refusal, the existing constant-folding scanner and the approved
candidate-only generation exception with normal-flow tests. The implementer
diagnosed rev-0's Git assertion as `reflog --all --single-worktree` including
disposable detached-worktree logs; its replacement snapshots raw live
index/HEAD/refs/reflogs with real mutation controls. This diagnosis/assertion
change still requires runtime confirmation and independent review, not waiver.
Native Windows identity/directory-sync tests are now explicit.
Rev-1 guarded formatting passed for the exact 32-file snapshot: 30 Go files
formatted, empty repository formatting check, eight formatter outputs mirrored
after local frozen-hash checks. The unchanged nine routing controls pass.
The same reviewer is rechecking the full revision, not just the prior list.

## Current State

S0–S7 are accepted. S7's complete gate at `8b84b33` passed 42/42 commands,
39/39 uncached test commands, exact parity and zero swap-outs; native CI
41612473590 completed successfully. Its allowed-failure Windows full-suite
log was not compared at S7 close. During S8 setup, the parent retrieved job
125837939993: 224 top-level failures and one panic line, with no added or
removed failure names versus the validated S6 `ee96551` log. This is the S8
comparison baseline, retained in ignored `bin/roc-s8-20261103/`.
Do not call that suite green or close #17. The full S7 record remains in
HISTORY, including its review limitations.

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
- `docs/adrs/ADR-051-operation-candidate-acceptance-recovery.md`: Proposed
  transaction, journal, publication and recovery implementation decisions.
- `internal/candidate/acceptance.go`, `artifact_root.go`, `source_guard_test.go`:
  held-capability rematerialization/cleanup and corresponding boundary guards.
- `internal/cli/operation_candidate.go`, `operation_candidate_s7_test.go`,
  `operation_candidate_s8_test.go`: terminal acceptance/recovery integration.
- `internal/workflow/operation_candidate_*`: acceptance, transaction,
  publication, recovery, platform file/sync boundaries and focused tests.
- `internal/workflow/patch_generations.go`, `recipe_coverage_publish.go`:
  private staged inputs to the existing shared APIs.
- `internal/workflow/recipe_authority_s0_source_guards_test.go`,
  `recipe_authority_s4_publication_guards_test.go`, `reconcile.go`: dependent
  caller inventories and early recovery refusal.
- Authorized revision scope adds the shared early journal check in
  `internal/cli/cobra.go`, any necessary platform directory-identity helpers,
  and the narrowly approved ADR-024 D9 amendment. No S9 public-doc expansion.

## Test Results

Guarded rev-0 formatting passed on the old Mac: `gofmt -w` on the explicit
25-file Go list, followed by empty `gofmt -l .`. Both commands used unchanged
60-second admission/runtime guards. The 26-file source snapshot was hashed
before transfer; ten formatted files were mirrored back only after verifying
that the local snapshot remained unchanged. Focused runtime checks and
independent review are next. No prior partial gate result is reused.
The first guarded focused command (`workflow -run '^TestROCS8'`) failed after
501.989s: `TestROCS8AcceptAncestorDirtyTreeAndReeligibility` reported live
index/ref/reflog mutation at line 232. The batch stopped immediately; CLI and
candidate/lock commands did not run. This is a test assertion failure, not a
resource stop. Preserve rev-0 evidence and diagnose which invariant changed;
do not weaken the assertion to manufacture a pass.

Rev-1 formatting passed under the same guards. Full affected workflow tests,
focused CLI S7/S8/reconcile tests, full candidate/lock tests, vet and build
are running in that order. Each command has its own unchanged admission/runtime
guard; the batch stops at the first failure. `-failfast` is used only for this
diagnostic batch, not the final CI-equivalent gate. No local Go runs overlap.
Source, manifests, scripts and evidence are retained under local ignored
`bin/roc-s8-20261103/` and remote `s8-rev1-source`/`s8-validation/rev1`.

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

1. Keep the formatted rev-1 source frozen while checks and review run.
2. Complete guarded affected checks, independent review and fix/review cycles until
   the final code has an explicit approval.
3. Complete the fresh full gate and exact-tip CI, then accept the ADR, archive
   the terminal handoff, update ROADMAP/LOG and push a clean closure.
4. Do not start S9 or #9 in this S8 dispatch.

## Blockers

The generation-contract decision is now approved, but its implementation and
the other rev-0 defects still require validation and independent re-review.
The deadline remains 2026-11-04T04:55:00Z; timer `roc-s8-deadline`.

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
