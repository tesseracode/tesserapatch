# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**S8 REV-5 IN PROGRESS — native CI exposed additional regressions.**
The published cba4423 checkpoint is not accepted. The prepared complete
gate has not run; fix forward and obtain fresh review/native evidence first.
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
- **Status**: In progress — repairing native CI lock-path/fixture failures and investigating the AQ observer timeout
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
Rev-1 affected validation stopped on its first command: the new live-Git
mutation-control fixture passes a newline-terminated SHA to `git update-ref`,
which rejects it. This does not establish that the revised acceptance path
passes. Source remains frozen until the ongoing review finishes so the next
revision can address the full findings set together.
The reviewer initially approved rev-1 source, confirming all four original
findings closed and the raw live-Git oracle stronger. That approval was
explicitly superseded by **NEEDS REVISION** after inspecting the runtime
failure: trim the captured commit ID locally, not the shared raw-output
helper. Rev-2 is limited to that fixture repair; final re-review still required.
Rev-2 returned with exactly the local `strings.TrimSpace` repair. The parent
compared all 32 frozen source/ADR files to formatted rev-1 and verified that
single-line delta. Guarded formatting passed without changing any bytes.
The same reviewer is checking the repair for a final source verdict while
the fresh rev-2 affected batch runs; no prior failure evidence is overwritten.
Final rev-2 verdict is **APPROVED — current source**. The reviewer verified
the fixture repair, retained all four prior closures and reaffirmed that the
raw live-Git oracle is stronger. This is source approval only: runtime,
native Windows and complete-gate evidence remain required before acceptance.
The rev-2 full workflow run reached an existing S7 privacy/source guard and
failed after 811.890s. Its type-checker includes both sides of the new S8
platform splits, producing duplicate declarations and undefined Windows
syscalls on macOS. This is a directly coupled analyzer regression, not a
production compiler or resource failure. Rev-3 may update the affected
test-only source selection and mutation controls, preserving all actual
privacy assertions and any frozen provenance fields. No production behavior
change or platform skip is authorized by this diagnosis.
Rev-3 returned frozen: `s7_an_privacy_test.go` now selects files with
`build.Context.MatchFile`, following the existing CLI analyzer pattern.
New controls exercise platform/tags, malformed constraints, wrong-platform
inclusion, missing active backend/root and active privacy sinks through the
existing validator. The only other delta is its S8 test-file allowlist
admission in `operation_candidate_s0_guards_test.go`. No production, golden
or provenance-byte changes are reported; validation and re-review are pending.
The parent verified the 33-file manifest against rev-2: only those two test
files differ; production and ADR bytes are identical. Guarded formatting
passed, with two formatter outputs mirrored after local frozen-hash checks.
The same reviewer is checking the fix while targeted analyzer regressions
precede the fresh full affected batch.
Final rev-3 source verdict is **APPROVED**. The reviewer confirmed build
selection propagates errors, negative controls reach the existing privacy
validator (including an active-platform sink), and the S8 admission leaves
earlier scope sets unchanged. Prior production/oracle closures stand.
Runtime/native/full-gate validation still blocks acceptance.
Rev-3 targeted analyzer checks and the entire workflow package now pass.
The next CLI command failed `TestReconcileAutoDropMerged_Match`: its
otherwise clean repository acquired untracked `.tpatch/locks/`, and the
normal clean-tree gate refused reconciliation. Rev-4 is scoped to the
early journal-preflight dependency and regression controls. Diagnose and
remove the unintended mutation; do not relax the clean-tree gate, delete
lock files to hide it, or let existing journals bypass early refusal.
Rev-4 returned frozen. The implementer confirmed early `TryAcquire` created
the persistent lock files; the preflight now scans read-only, while held-lock
transaction rechecks remain unchanged. The bounded delta is recovery.go,
its tests, CLI S8 tests and a minimal Proposed ADR-051 explanation. Controls
cover filesystem preservation, unsafe/present journals, post-probe journal
creation, invalid handles and clean CLI preflight. No Go tools ran in revision.
The parent verified exactly those four deltas in the 33-file manifest.
Guarded formatting/routing controls pass; one formatter output was mirrored
after local frozen-hash checks. The same reviewer is checking probe authority,
fail-closed paths and the unchanged held-lock recheck while guarded probe,
CLI, candidate/lock, vet and build commands run serially.
Final rev-4 source verdict is **APPROVED**. The reviewer confirmed the
probe neither acquires locks nor writes files, rejects unsafe namespaces
and present journals without trusting their contents, preserves early CLI
ordering, and retains authoritative held-lock rechecks. Earlier closures
stand. Runtime/native/full-gate validation still blocks acceptance.
The rev-4 guarded batch has now passed all seven commands: both formatting
steps, changed-probe workflow regressions, affected CLI, full candidate/lock
packages, vet and CLI build. All stages retained normal pressure and zero
new swap-outs; no failure marker or owned command remains. The exact 33
reviewed source/ADR files are byte-unchanged. Publish this checkpoint with
explicit staging, then verify exact-tip native CI before the complete gate.
Reviewed checkpoint **`cba4423ecacc6caec7addc12641e228835072ee0`** is pushed.
All 36 staged source/ADR/tracking paths were explicit and verified; the
trailer parses, HEAD equaled origin/main, the tree was clean and v0.17.0
unchanged. Exact-tip native CI is **41718498683**; Windows job
**126205486285** has passed formatting/vet/build but runtime remains pending.
Keep origin/main frozen at this checkpoint during CI/gate. Tracking-only
progress commits may remain local until closure so they do not cancel the
exact-tip CI or advance the gate's origin check; push them with the closure
before any future wave. No production changes are authorized without review.

The old Mac's fresh `s8-final-source` and `s8-full-gate` are prepared at this
checkpoint, with byte-identical accepted harness/profile/shard scripts and
empty evidence/caches. The nine Python routing controls initially had a
wrong-CWD setup error before creating fixtures or running Go; rerunning the
unchanged controls from the prepared source passed all nine. No full gate
has started, and no guard was altered.
Native CI 41718498683 finished **FAILED**. Linux and macOS both fail the
existing cumulative downstream soak because ordinary reconciliation leaves
`.tpatch/locks/reconcile/maintained-route.lock`, which later `land` refuses.
Windows reports success only because its full-suite step is allowed-failure:
actual failures rise from 224 to 226, adding two S8 fixture failures (held
directory rename denied; reading an actively locked file denied). The
existing single panic is unchanged. macOS AQ registration observation also
times out; the Linux observer job passes. Do not label that timeout flaky or
raise/skip its checks without diagnosis.

Rev-5 is bounded to the remaining ordinary-path lock side effect and genuine
Windows controls, with read-only AQ diagnosis before any extra scope. Keep
lock/journal/land guards intact, preserve all logs, and obtain new independent
approval. The old prepared cba4423 gate/source remain untouched and unrun.

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
- Rev-3 adds `internal/workflow/s7_an_privacy_test.go` for platform-aware
  source selection and mutation controls, plus its single S8 scope admission
  in the already modified `operation_candidate_s0_guards_test.go`.

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

Rev-1 formatting passed under the same guards. The first affected command
(`workflow -count=1 -timeout 40m -failfast`) failed after 36.573s in
`TestROCS8LiveGitIdentityDetectsIndexRefsAndReflogChanges` at line 225:
`git update-ref` rejects its newline-terminated restore SHA. CLI, candidate/
lock, vet and build did not run. Pressure remained normal, new swap-outs zero;
this is not a resource stop. Owned command processes are gone.
`-failfast` is diagnostic only; the final CI-equivalent gate remains unchanged.
Source, manifests, scripts and evidence are retained under local ignored
`bin/roc-s8-20261103/` and remote `s8-rev1-source`/`s8-validation/rev1`.
The preserved remote/local `03-go.json` SHA256 matches:
`a59ad5d0afb49c383f030b008e3422f753dfb0696f254d68198ebfab281bf6b7`.

Rev-2: nine unchanged routing controls pass; guarded formatting passes.
Fresh full workflow, focused CLI S7/S8/reconcile, full candidate/lock, vet and
build commands run serially with stop-first-failure in `s8-validation/rev2`,
against a clean full-history base plus the exact frozen source overlay.
No runtime result yet; previous partial results are not acceptance evidence.
Rev-2 subsequently failed at
`TestS7PIB420PureGeneratorRetryConstructionGuardAndWrongInput`
(`s7_an_privacy_test.go:351`): duplicate `operationCandidateSyncDirectory`
and `operationCandidateReadFlags` declarations plus undefined Windows syscalls
show that the source type-check includes mutually exclusive platform files.
The workflow command stopped after 811.890s; CLI/candidate/lock/vet/build
remain unrun. Pressure was normal and new swap-outs zero. Failed evidence
and marker remain in `s8-validation/rev2`, mirrored locally; matching
`03-go.json` SHA256:
`6c10d3e9b7c8f57a508d7d5d773de9fd9ef9ee029ccaa57774945d438dcd16aa`.

Rev-3: nine unchanged routing controls and guarded formatting pass.
`TestS7Workflow`, `TestS7PIB420/446` and `TestROCS8ScopeTransition`
run first, followed by full workflow, focused CLI S7/S8/reconcile,
candidate/lock, vet and build. The fresh serial batch in
`s8-validation/rev3` stops at the first failure; previous evidence is retained.
Rev-3 results: targeted analyzer/scope checks PASS (8.062s); full workflow
PASS (1062.653s). CLI stops after 71.212s at
`TestReconcileAutoDropMerged_Match` (`reconcile_check_applied_test.go:166`):
untracked `.tpatch/locks/` causes the clean-tree refusal. Candidate/lock,
vet and build remain unrun. Pressure normal, new swap-outs zero, owned
commands ended. Evidence is mirrored locally and retained remotely; matching
failed `05-go.json` SHA256:
`4a60151615eeff9532928337a7240df73a431b2b4a6fbcfd6c4ac4019bd01cb9`.

Rev-4: nine unchanged routing controls and guarded formatting pass.
The fresh `s8-validation/rev4` batch starts with changed-probe workflow
regressions, then the full existing CLI S7/S8/reconcile selector,
candidate/lock packages, vet and build, stopping at the first failure.
Unchanged workflow cases will all run again in the complete fresh gate;
rev-3's partial evidence will never be stitched into acceptance.
Rev-4 final results: workflow probe/inventory checks PASS (6.375s), affected
CLI PASS (83.927s), candidate PASS (25.413s), candidatelock PASS (0.290s),
`go vet ./...` PASS and CLI build PASS. Seven of seven guarded commands
(including formatting) passed, normal pressure and zero new swap-outs each.
All remote/local stage hashes match; `blocked.json` is absent and owned
commands are gone. Evidence remains under `s8-validation/rev4` and
`bin/roc-s8-20261103/rev4-evidence/evidence`.
No native Windows result or complete S8 gate result exists yet.
At cba4423, native formatting/vet/build pass on all three OSes, but native
runtime does not. Linux/macOS:
`TestRGAS6CumulativeDownstreamSoak` fails at
`recipe_authority_s6_soak_test.go:321` on the persistent reconcile lock path.
Windows adds `TestROCS8WindowsDirectoryFlushRejectsReplacedPath`
(`operation_candidate_recovery_test.go:57`, rename denied before sync) and
`TestROCS8JournalProbeDoesNotReplaceHeldRecheck`
(`operation_candidate_recovery_test.go:522`, locked-file read denied).
All 224 baseline Windows failures and its known panic remain.
macOS observer `TestS7ObservedAQRegistrationAuthority` times out at 510.67s:
PIB-487 started without PASS, PIB-498 never ran. Preserve and diagnose.
Logs/comparison live under ignored `bin/roc-s8-20261103/s8-cba4423-*`.
The matching Linux AQ observer passed in 66.209s. A single diagnostic now
runs the unchanged cba4423 AQ observer on a fresh isolated old-Mac clone
under the accepted serial/resource policy. Evidence is separate at
`s8-validation/ci-cba4423-aq`; shell `roc-s8-ci-aq-diagnostic`.
This is not a native-CI retry or proof of flakiness. Rev-5 formatting/Go
checks must wait for it to finish; no overlapping Go workload is permitted.

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

1. Fix the remaining ordinary reconcile lock-path regression and Windows
   fixture assumptions without weakening production guards or native proof.
2. Diagnose the AQ observer timeout; no timeout/partition/memo-policy change
   is authorized merely to manufacture green. Review and validate rev-5.
3. Complete the fresh full gate and exact-tip CI, then accept the ADR, archive
   the terminal handoff, update ROADMAP/LOG and push a clean closure.
4. Do not start S9 or #9 in this S8 dispatch.

## Blockers

Native CI fails on the ordinary lock-path regression, two Windows fixture
assumptions and the macOS AQ observer timeout. Fresh source approval/native
evidence and a complete gate are still required; ADR-051 remains Proposed.
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
