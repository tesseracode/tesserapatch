# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

Rev-4 is implemented and awaiting review. It closes the four numbered rev-3
adjudication items — expression-valued `continue-on-error`, the missing
job-level `if` rule, the incorrect `continue-on-error` visibility comment, and
AVP-141's working-tree scratch file. **No production file changed**: the diff is
one CI comment block, two `internal/intent` test files, one `internal/cli` test
file and this handoff. No production behavior, PRD, ADR or AVP matrix row
moved.

## Active Task

- **Task ID**: `implement-prepare-check`
- **Issue**: [GH #16](https://github.com/tesseracode/tesserapatch/issues/16)
- **Description**: Implement
  `PRD-artifact-validation-and-provenance` rev-5 + ADR-034 rev-2.
- **Status**: Rev-4 implemented, AWAITING REVIEW
- **Assigned**: 2026-08-17
- **WAVE_BASE**: `9a8c1d049bb973ccf377bd9f0fa67d7080d2d773`
- **Release tag**: none assigned; this prerequisite must be reviewed before any
  mutating prepare release planning

## Contract

- Command: `tpatch prepare <slug> --check`.
- Read only: no provider, prompt, lock, filesystem mutation, state transition,
  status timestamp, `FEATURES.md` refresh, or artifact write.
- Required artifacts: `analysis.md`, `spec.md`, `exploration.md`.
- Optional analysis sidecar never affects readiness.
- Full accepted nine-state artifact classification and three-document readiness.
- Constant `provenance: unknown`.
- One held Go 1.26 `*os.Root`; logical root confinement, unsafe/non-regular
  refusal, bounded `MaxArtifactBytes+1` reads, honest instability reporting.
- Accepted `unix || windows` fail-closed platform policy.
- Exact human/JSON/quiet output, exit codes and precedence from the 208-row
  `AVP-001…AVP-208` matrix.
- Existing per-phase `--manual`, `next`, `cycle`, and `apply --mode prepare`
  behavior stays byte-compatible.

## Scope and Discipline

- One sequential implementer owns the wave because CLI registration, output
  contracts, tests and public docs overlap.
- Explicit-path staging only; no broad add/commit.
- Implementation goldens must come from this wave's commit range, not from the
  later mutating-prepare cluster.
- Native Windows behavior is part of acceptance.
- Mutating prepare PRD rev-14 / ADR-035 rev-14 remains blocked.

## Session Summary — rev-4

Sole sequential reviser. **No production file changed**: the diff is one CI
comment block, two `internal/intent` test files, one `internal/cli` test file
and this handoff. Each numbered rev-3 adjudication item is closed below.

1. **Expression-valued `continue-on-error` (the rev-3 hole).** The AVP-175
   YAML subset parser decoded `continue-on-error` to a Go `bool` with
   `value == "true"`, so *every* non-literal form — `${{ true }}`,
   `${{ !false }}`, `${{ vars.SOFT_FAIL }}`, `"true"` — collapsed onto `false`,
   i.e. onto "blocking". A job or blocking step demoted by an expression read
   as gating to the guard while GitHub evaluated it to `true` at run time.
   `workflowStep.ContinueOnError` and `workflowJob.ContinueOnError` are now the
   **raw scalar as written**, captured before the quote and trailing-comment
   normalisation the other keys go through, and
   `classifyContinueOnError` maps it onto a closed four-member set —
   `absent`, `false`, `true`, `non-literal`. Two ownership rules consume it:

   - `requireBlockingContinueOnError` — absent or the exact literal `false`.
     Applied to the `test` **job** and to **every step in it** except the one
     allowed-failure step. Rev-3 only inspected steps whose `run` contained
     `go test`, so `Verify formatting`, `go vet`, `Build` and the two smoke
     steps could be demoted unnoticed; they are now in scope.
   - `requireAllowedFailureContinueOnError` — the exact literal `true`, so the
     GH #17 demotion is stated outright in the file rather than derived from
     an expression a reader would have to evaluate.

   A non-literal is **rejected**, not interpreted: its run-time value comes
   from inputs (`vars.*`, `env.*`, `github.*`) the guard cannot read. The job
   is additionally pinned to **exactly one** `continue-on-error` step, and that
   step must be the Windows full-suite step, so the demotion cannot be
   relocated onto a step the allowed-failure checks (blocking-package absence,
   native-marker absence, ordering, `#17` ownership) would not reach.

2. **Job-level `if`.** `checkCIWindowsGate` now requires the `test` job to
   carry no job-level condition at all. A job-level `if` skips the whole job,
   and a skipped job's conclusion is `skipped`, not `failure` — so
   `needs: test` on the `release` job stays satisfied while no gate inside the
   job ever runs. Platform selection stays per step, through the two exact
   conditions rev-3 pinned; those remain unchanged.

3. **CI comment correctness.** The allowed-failure step's comment claimed the
   failures "stay VISIBLE in this step and in the job log — the step's own
   conclusion and output are what a reader inspects". That is wrong:
   `continue-on-error: true` maps the step's **conclusion** to `success`, so
   the step conclusion, the job conclusion and the run summary all report
   success. The comment now states that the failures are visible **only** in
   the step's log output and that a reader has to open the job log to see the
   inventory.

4. **AVP-141 scratch file.** The sensitivity arm wrote its real untracked
   `root_probe.go` into `.avp141-scratch-<pid>/` at the repository root. That
   path is matched by `git ls-files --others --exclude-standard -- '*.go'`,
   which is exactly the wave-close untracked-source sentinel (`Makefile`
   `[2/8]`), so a run interrupted between the write and `t.Cleanup` left
   source noise the gate would then flag. The scratch now lands under the
   repository's git directory, resolved with
   `git rev-parse --absolute-git-dir` (`avpGitDir`), which no
   `git ls-files --others` query can ever report. The arm still proves the
   property, and proves more of it than before: the file is asserted to exist
   on disk, to be absent from `git ls-files`, to be absent from the sentinel's
   own query (`gitUntrackedGoFiles`), to be absent from the scanned
   population, and to leave the call-site count unmoved.

**Sensitivity arithmetic.** AVP-175 goes from fifteen to **twenty-four**
failing arms. The nine added: `job-level-continue-on-error-expression`,
`job-level-continue-on-error-variable`, `job-level-if-false`,
`blocking-step-continue-on-error-expression`,
`blocking-step-continue-on-error-negation`, `vet-step-demoted`,
`allowed-failure-continue-on-error-expression`,
`allowed-failure-continue-on-error-quoted` and
`allowed-failure-continue-on-error-false-expression`.

Three **accepted variants** are added alongside them —
`job-level-continue-on-error-false`, `blocking-step-continue-on-error-false`
and `job-level-if-empty` — each asserted to still **pass** the guard. Without
them the sensitivity requirement could be satisfied by a guard that rejects
every `continue-on-error` key it sees, which would be sensitive for the wrong
reason: writing the ownership out as the literal `false` is the same statement
as omitting it.

## Session Summary — rev-3

Sole sequential reviser. **No production file changed**: the diff is one CI
comment block, three test files, one test helper file and this handoff. Each
numbered rev-2 adjudication item is closed below.

1. **Green tip (BLOCKER).** `internal/cli/land_rev1_fold_test.go`'s
   `mixed-case` mutation was `strings.ToUpper(v[:8]) + v[8:]`, a no-op
   whenever the fixture SHA's first eight characters were all decimal digits
   (~2.3% per run). The row then stored a *canonical* base commit and asserted
   `land` refuses it, which it correctly does not. The mutation is now
   `mixedCaseBaseCommit`: it uppercases the first `a`-`f` byte, and for an
   all-digit SHA — the only input with no such byte — replaces one digit with
   an uppercase `A`, so the result is 40 characters and non-canonical for
   **every** 40-character lowercase-hex SHA. The table loop additionally
   fails any arm whose mutation equals its input, so no future arm can
   silently become a no-op. `TestRev1Land_MixedCaseMangleIsNeverANoOp` proves
   the property deterministically over 241 subtests: one per `a`-`f` letter at
   each of the 40 positions, plus the all-digit SHA.

2. **CI job-level signal.** AVP-175 now parses job-level fields
   (`parseWorkflowJobs`) as well as steps, and fails when: the `test` job
   carries `continue-on-error: true` (which would make every step inside it,
   including the blocking Windows gate, advisory); the `release` job loses
   `needs: test` (a red test job would still publish a release); or any gating
   step's `if` is not *exactly* `runner.os == 'Windows'` or
   `runner.os != 'Windows'` after whitespace/`${{ }}` normalisation. The exact
   comparison is what rejects `&& false`, event/schedule conjuncts and any
   other alternate truthiness that keeps the text a reviewer greps for while
   disabling the step. The tag-version step is pinned to exactly
   `startsWith(github.ref, 'refs/tags/v') && runner.os != 'Windows'`.

3. **Blocking surface integrity.** `checkBlockingWindowsStep` pins the
   blocking Windows package set **exactly** — `./internal/intent`,
   `./assets`, `./internal/buildinfo`, `./internal/redact`,
   `./internal/safety`, `./internal/testutil`,
   `./internal/tools/studyvalidator`, `./tests/integration` — by extracting
   every `./…` argument from the step and comparing the set, so a package
   cannot be dropped or added unnoticed. It also pins the verbose native
   invocation, the `AVP-176`/`AVP-199` row loop (both rows *and* the
   `--- PASS: TestAVPNativeWindows/${row}` assertion), and a leaf floor of at
   least 6. Every allowed-failure step is checked to contain none of the
   blocking packages, no `-run TestAVPNativeWindows` and no `--- PASS:`
   assertion, generalising the rev-2 `internal/intent`-only check.

   Eight sensitivity arms are added to the existing seven:
   `job-level-continue-on-error`, `blocking-condition-false-conjunct`,
   `blocking-package-removed`, `blocking-package-moved-to-allowed-failure`,
   `native-row-loop-shrunk`, `native-row-loop-removed`, `leaf-floor-zeroed`
   and `release-needs-removed` — fifteen arms in total, each asserted to
   fail the guard.

4. **Remaining source walk.** `countOpenRootCallSites` in
   `internal/cli/prepare_avp2_test.go` no longer walks the working tree and no
   longer exempts `.golden-baseline`. The population is `git ls-files`:
   tracked, non-test `.go` files only, read through
   `trackedProductionGoSources`, and the counter is a pure function over that
   map. `AVP-141/sensitivity` proves a **tracked** extra `os.OpenRoot` call
   site moves the count, that a real untracked working-tree file written
   during the test does **not** enter the population or move the count, and
   that every scanned path is tracked. This mirrors the rev-2 AVP-134 fix and
   removes the last `filepath.WalkDir` source population in the wave.

5. **Tracking truth.** Every "192 failures plus two 10-minute timeouts"
   statement is corrected to the run
   [`32093250847`](https://github.com/tesseracode/tesserapatch/actions/runs/32093250847)
   inventory: **200 failing top-level tests, 283 counting subtests, six
   packages** (`internal/cli`, `internal/gitutil`, `internal/provider`,
   `internal/rescap`, `internal/store`, `internal/workflow`), and **no package
   times out** at `-timeout 20m`. The rev-1 "192" was a truncated count: two
   packages panicked at the default ten-minute timeout, so their remaining
   tests never ran. The rev-1 figure is retained only where it describes that
   historical run, explicitly labelled as superseded. The history statement is
   restated where it appears: **GH #16 added the `windows-latest` row**, and
   `main` was green at `WAVE_BASE` `9a8c1d0` because there was no Windows leg
   at all — not because Windows passed.

6. **Guard notes.** The workflow comments are corrected to the same inventory
   and now state the visibility claim accurately: the allowed-failure results
   are visible **in that step and in the job log** — no claim is made about a
   run summary, which does not carry them. Nothing else in the wave's scope
   was reopened; full Windows remediation stays with GH #17.

## Session Summary — rev-2

Sole sequential reviser. No production behavior changed: the diff is CI,
tests/guards, testdata provenance and tracking. Each numbered rev-1 finding is
closed below.

1. **Blocking Windows signal (BLOCKER).** `.github/workflows/ci.yml` now has
   three test steps instead of one:
   - `Test` (`if: runner.os != 'Windows'`) — the whole repository, blocking, on
     ubuntu-latest and macos-latest exactly as before.
   - `Test (Windows GH #16 surface — blocking)` — runs
     `go test -v -count=1 -run TestAVPNativeWindows ./internal/intent`, then
     **asserts the verbose log contains the `--- PASS:` lines** for
     `AVP-176`/`AVP-199` and at least six leaf assertions, because
     `go test -run` over a pattern that matches nothing exits 0 and would look
     green. It then runs the empirically green package set from run
     `32086471429`: `./internal/intent ./assets ./internal/buildinfo
     ./internal/redact ./internal/safety ./internal/testutil
     ./internal/tools/studyvalidator ./tests/integration`.
   - `Test (Windows full suite — allowed to fail, owned by GH #17)` —
     `go test ./... -count=1 -timeout 20m` with `continue-on-error: true`. The
     200 unrelated top-level failures (283 counting subtests) stay **visible**
     in that step and in the job log; they no longer fail the job, so `main`
     and tagged releases are publishable. The step name carries `#17`, which
     is the issue that owns deleting it.

   `Install binary (smoke test)` is split into a bash leg (non-Windows) and a
   `pwsh` leg (Windows) — the bash form interpolates `go env GOPATH`, which is
   a backslash path on Windows. `Verify tag version` is restored to **both**
   non-Windows legs (`runner.os != 'Windows'`), not ubuntu only; the Windows
   leg is excluded because `make` is absent on the runner, which the comment
   now states.

2. **AVP-175 sensitivity (HIGH).** `checkCIMatrix` keeps the LF-checkout
   assertions and delegates to a new `checkCIWindowsGate`, which parses the
   workflow into steps (`parseWorkflowSteps`, a small YAML-subset parser for
   mapping keys and `|` block scalars) instead of scanning the file for
   substrings — `continue-on-error: true` anywhere in a file otherwise looks
   like it belongs to every step. The guard asserts: the non-Windows full
   suite is blocking; a blocking Windows step exists that names
   `./internal/intent`, runs `-run TestAVPNativeWindows` verbosely and asserts
   the `--- PASS:` lines; every allowed-failure step is ordered **after** it,
   never mentions `internal/intent`, and names GH #17; and the tag-version
   step is not pinned to one matrix entry. The sensitivity fixture has six
   arms — matrix row removed, CRLF checkout restored, `internal/intent` moved
   into the allowed-failure command, `continue-on-error: true` added to the
   blocking step, the native invocation dropped, and the tag check pinned back
   to ubuntu — plus a seventh added by the post-push CI inspection (the
   allowed-failure step name unquoted, so YAML swallows the `#17` reference).
   A non-detecting arm now calls `t.Fatalf` instead of returning an
   error, because returning an error is how this harness signals "sensitive"
   and would launder a hole into a pass.

3. **Tracking truth (MEDIUM).** The CI history statement is corrected below:
   GH #16 **added** the `windows-latest` row; `main` was **green** at
   `WAVE_BASE` `9a8c1d0` because no Windows leg existed; this wave exposed
   the pre-existing Windows failures. GH #17 is the named owner and the
   explicit exit condition for the allowed failure.

4. **AVP-168 derived from production (HIGH).** The test-local
   `realStatusOutcomeMap()` literal is deleted. `deriveStatusOutcomes` drives
   six filesystem conditions (empty, symlink, non-regular, oversize,
   vanished-between-lstat-and-open, permission-denied) through the real
   `Inspect` **twice**: once on `status.json`, which yields the abort code,
   and once on `spec.md`, which yields the structural state the production
   classifier assigns. Both halves are read back out of `Inspect`; nothing is
   asserted from a hand-written expectation. `checkStatusAbortTotality` adds
   the state↔abort correspondence (`status-<state>`, with the one documented
   `present-empty → status-malformed` exception) to the existing totality,
   disjointness and catalog-size checks. Sensitivity arm 1 **crosses two
   probes' production-path inputs** so the derived pairing becomes
   `unreadable → status-oversize`; arm 2 removes the probe that drives the
   oversize branch, so the derivation can no longer observe that outcome.

5. **AVP-134 source scan (MEDIUM).** The working-tree `filepath.WalkDir` with
   its `.golden-baseline` name exemption is deleted. The population is now
   `git ls-files`: tracked, non-test `.go` files only, so a nested worktree,
   an editor scratch copy or any untracked experiment cannot fail the row.
   `checkInspectorImporters` is a pure function over that map, and
   `AVP-134/sensitivity` proves it fails for a **tracked** forbidden importer
   (`internal/workflow/prepare_probe.go`), fails when the one authorized
   importer moves, and that every scanned path is in fact tracked.

6. **Status parity holes (MEDIUM).** `schemaFieldsOf` now **fails** on an
   exported field with no JSON tag (it marshals under its Go name, so it is a
   real document member) and on an embedded field; `schemaShapeFromSources`
   returns an error instead of `t.Fatal`ing, so a mutation that breaks
   extraction counts as a detection. `normalizeSchemaType` records any named
   type it has no rule for and the walk fails rather than silently
   normalizing. `checkNamedStringTypes` asserts every enum the mirror types as
   `string` really is `type X string` upstream and that no allowlist entry is
   dead. Three new sensitivity arms cover the untagged exported field, an
   alias whose underlying type stops being `string`, and a newly introduced
   named type; a fourth asserts the unmutated sources still pass, so the
   harness itself cannot rot into always-failing.

7. **Routing golden rigor (MEDIUM).** Each of AVP-071/072/136/137 now counts
   its matches and fails when a prefix matched no fixture; `routingRowPrefix`
   returns an error for an unknown row instead of an empty prefix that
   silently matched everything. The fixture count is pinned at **12**. The
   unverifiable baseline binary digest is **removed** from
   `testdata/routing-goldens/README.md` with the reason recorded (`go build`
   embeds a build ID from the toolchain, module paths and flags, so a reviewer
   cannot reproduce it and no test could check it). The claims that remain are
   checked by `TestAVPRoutingGoldens/provenance-is-verifiable`: no digest
   claim, both environment variables and the worktree recipe still documented,
   and the manifest table agreeing with the directory in both directions. The
   recipe and `buildCurrentBinary` both use `-trimpath` so baseline and
   current are built the same way; re-recording under `-trimpath` produced
   byte-identical goldens.

8. **Low guard defects (LOW).**
   - The tautological `strings.HasSuffix("internal/intent/avp_native_windows_test.go",
     "_windows_test.go")` is replaced by a real constraint check: `go list`
     reports the package's test files for `GOOS=windows` and for the host, and
     the fixture must appear in the first and not in the second.
   - The character-device row asserted a branch that could not execute
     (`!refused(info) && info.Mode().IsRegular()`). It now pins the actual
     production gate order: `refused(info)` must be false (a char device is
     not a reparse point) and `IsRegular()` must be false, so `capture`
     classifies it through `!pre.Mode().IsRegular()`; the comment says exactly
     that.
   - `internal/intent/status_schema.go` referenced a file name that never
     existed (`status_schema_guard_test.go`); it now points at
     `status_schema_test.go`.
   - AVP-170's sensitivity arm allocates inside the measurement window to
     model the rejected per-capture design; AVP-197's asserts budget
     arithmetic. Neither rebuilds the inspector, and both now say so in a
     dedicated honesty note, so the rows are not cited as measurements of a
     mutated production build. AVP-116 remains a source scan plus the real
     backing-array assertions in `TestAVPRootedBoundaryHonesty/AVP-197`, which
     measure the real `Inspect`.

No test was added outside these findings, and no production file changed in
rev-2.

## Rev-3 Review and Rev-4 Adjudication

**Internal verdict**: NEEDS REVISION
**External verdict**: APPROVED
**Implementation tip**: `a4748a9`
**Tracking tip**: `aa27927`

1. Reject expression-valued `continue-on-error` such as `${{ true }}` on the
   test job or blocking Windows step; allowed-failure ownership must be the
   exact literal `true`, while blocking/job scope permits only absent/false.
2. Require the test job to have no job-level `if`; add a failing
   `job-level-if-false` sensitivity arm.
3. Correct CI comments: the allowed-failure step conclusion is success; only
   the step/job log carries failing test output.
4. Move AVP-141's real untracked scratch under `.git` (or otherwise outside the
   untracked-source sentinel) so an interrupted test cannot leave source noise.

Every other rev-0–rev-3 closure is frozen.

## Rev-2 Review and Rev-3 Adjudication

**Internal verdict**: NEEDS REVISION (stale Windows inventory text only)
**External verdict**: APPROVED WITH NOTES (close blockers)
**Implementation tip**: `40ae5c2`
**Tracking tip**: `b95232e`

1. **Green tip.** Fix the pre-existing mixed-case base-commit test so its
   mutation can never be a no-op when the SHA prefix contains only digits.
2. **CI job-level signal.** AVP-175 must reject job-level
   `continue-on-error`, require `release.needs: test`, and permit only exact
   Windows/non-Windows `if` expressions without hidden false conjuncts.
3. **Blocking surface integrity.** Pin the full blocking Windows package list,
   native row loop and minimum leaf count; fail if packages/leaves move into the
   allowed-failure step.
4. **Remaining source walk.** Convert AVP-141's `os.OpenRoot` call-site scan
   from working-tree `WalkDir`/`.golden-baseline` exemption to tracked files.
5. **Tracking truth.** Replace 192/timeouts with the observed rev-2 inventory:
   200 top-level failures (283 including subtests), six packages, no timeout at
   20 minutes.
6. **Guard cleanup.** Fold AVP-168/134/175 sensitivity refinements and the
   remaining non-blocking Windows/comment/source notes without changing product
   behavior.

The 208-row matrix, native Windows behavior, status mirror, routing goldens and
accepted PRD/ADR errata are frozen.

## Session Summary — rev-1

Sole sequential reviser. Each numbered rev-0 finding is closed below.

1. **Status fidelity (HIGH).** `internal/intent/status_schema.go` adds a local
   mirror of `store.FeatureStatus` and every shape reachable from it
   (`ApplySummary`, `ReconcileSummary`, `PatchIDMatch`, `Dependency`,
   `VerifyRecord`, `RejectionStatus`, `RejectionHistoryEntry`, `EvidenceRef`,
   `DivergenceDetail`, including `time.Time` members). `decodeStatusDocument`
   rejects anything that would not unmarshal into that schema, so a known
   field with the wrong JSON type is now `status-malformed` rather than a
   silently accepted document. Unknown keys stay accepted —
   `DisallowUnknownFields` is deliberately not set, matching every other
   reader. The no-`internal/store` import boundary is retained and asserted
   (AVP-087, AVP-150). `TestAVPStatusSchemaParity` walks
   `internal/store/types.go` + `internal/store/status.go` by AST, compares
   field names, JSON names, `omitempty` flags and normalized types against the
   mirror, and ships a sensitivity fixture that fails on an added field, a
   renamed JSON tag and a changed type.
2. **Platform errata (MEDIUM).** `PRD-artifact-validation-and-provenance` is
   **rev-6 errata** (Accepted status retained) and
   `ADR-034` is **rev-3 errata** (Accepted retained). Both record that
   `syscall.O_NONBLOCK` is undeclared in `syscall` on `js/wasm` and `plan9`,
   so the rev-4 two-half `!windows` / `windows` partition does not compile for
   exactly the `GOOS` set D5 refuses, and three halves — `unix`, `windows`,
   `!(unix || windows)` — are required. AVP-118 and AVP-208 are amended; D5
   and D6 carry the matching text. The added obligation is **unreachability**:
   the platform gate aborts before `os.OpenRoot` and before any name is
   composed. No product behavior changes; the matrix is still 208 rows and the
   guard set still 43. The ADR index row is updated.
3. **Acceptance ledger (HIGH).** `internal/intent/avp_ledger_test.go` maps all
   **208** rows to **224** references across `intent`, `cli` and `assets`.
   References resolve by parsing the test sources (literal `t.Run`, keyed
   table `id:` fields, and `[]string` range literals); `TestAVPGuards/<id>`
   resolves against the live guard registry, which additionally proves the
   guard and its sensitivity fixture exist. The ledger fails on a missing row,
   an undeclared row, a duplicate reference and an unresolvable reference, and
   `TestAcceptanceLedgerResolutionRejectsFalsePositives` proves comments,
   string fixtures, wrong packages, missing subtests and non-runnable
   signatures do not resolve. `TestAcceptanceLedgerMatrixArithmetic` re-derives
   §18.27 (25 categories, the kind table, the 43-row guard predicate and the
   165-row complement) from the rows themselves. No row is satisfied by raw
   source-string presence where the behavior is testable.
4. **43 guards + sensitivity (HIGH).** `avpGuards` registers exactly the 43
   `G`-kind rows; `TestAVPGuards` runs each guard and then its sensitivity
   fixture, failing when the mutated input does **not** break the guard.
   Coverage: forbidden JSON key names and the closed human-label set,
   path-kind absence, content-hash absence, abort shape, advisory
   cardinality, provenance domain, output-byte rule, state/reason/advisory/
   abort catalog totality and bijections, precedence pairs, `FeatureState`
   parity against `internal/store` by AST, lifecycle-line totality, exit
   templates, cap↔message coupling, forbidden readers/mutators/read
   primitives, the fixed scratch buffer, seam shape (three + three methods,
   exactly two adapters), single `os.SameFile` call site, `fs.ValidPath`
   names, the reparse predicate, allocation ceilings, descriptor lifecycle,
   CI matrix parsing, confinement-constant declaration and ordering,
   cross-builds, build-tag allowlist exhaustiveness/disjointness over
   `go tool dist list`, the three `openFlags()` halves, `winsymlink=1`,
   the real-FIFO `O_NONBLOCK` Go-upgrade tripwire, over-claim scans across
   shipped strings and both contract documents, the stdlib proper-subset
   relation, and the derived meta-check itself.
5. **Behavior populations (HIGH).** The fake `FileOps` now maintains a real
   read offset, so `io.ReadFull` produces the genuine `io.EOF` /
   `io.ErrUnexpectedEOF` / `err == nil` taxonomy instead of forced errors, and
   every read records its requested length, capacity and backing array. Added
   coverage: injected capture rows 13/14/15/17/18/19/20/20a/20b/20c, the full
   status ladder including 16a close failure, exact `cap` / `cap+1` /
   whitespace / JSON-object boundaries for both caps, the one-allocation
   ceiling with backing-array identity for all five captures, all thirteen
   aborts (ten reachable end-to-end, three injected), the lifecycle-line
   table, attacker bytes, root lifetime and close precedence, sidecar advisory
   totality over all nine states, deterministic human/JSON/quiet bytes, and
   the compatibility differential through the real `--manual` commands.
6. **Native Windows (HIGH).** `internal/intent/avp_native_windows_test.go` is
   `GOOS`-constrained and contains **no skip of any kind**: junctions are
   created with `cmd /c mklink /J` and the helper `t.Fatal`s when the command
   is unavailable or returns non-zero; symlink, junction, reparse-point
   status, `os.SameFile` identity across a replacement, and a `FILE_TYPE_CHAR`
   handle are all asserted. `TestAVPWindowsSourceGuards` runs on every target
   and pins AVP-175 (CI matrix parses and runs `go test`), AVP-176/199 (no
   `t.Skip` in the fixture path, `mklink /J` retained), AVP-198 (the
   `//go:debug winsymlink=1` directive) and AVP-178 (Windows cross-build).

   **CI defect found and fixed by inspecting the run after push.** The
   `windows-latest` job had never reached its `Test` step: the runner checks
   out with `core.autocrlf=true`, so every text file gains CRLF and
   `gofmt -l .` lists the entire tree, failing `Verify formatting` in ~25s.
   The Windows row was in the matrix and was nonetheless completely unrun —
   the precise failure mode AVP-175 exists to prevent. `.github/workflows/ci.yml`
   now forces `core.autocrlf false` / `core.eol lf` **before**
   `actions/checkout`, and the AVP-175 guard was widened to assert that step
   exists, is gated on the Windows runner, precedes the checkout, and that the
   job runs `go test`. Its sensitivity fixture has two arms: removing
   `windows-latest`, and leaving the row in place while flipping `autocrlf`
   back to `true`.
7. **Pre-change routing goldens (HIGH).** A temporary detached worktree at
   `WAVE_BASE` `9a8c1d0` was created, its binary built, the goldens recorded,
   and the worktree deleted. `internal/cli/testdata/routing-goldens/`
   holds twelve fixtures with a `README.md` recording commit, subject,
   toolchain, binary SHA-256 and the exact recording commands. The current
   binary is rebuilt by the test and compared byte-for-byte. `apply --help`
   is the one authorized delta and is asserted as a bounded change (the
   AVP-010 pointer sentence and nothing else). The capture runs under a
   hermetic environment so a leaked credential cannot turn the heuristic
   `cycle` transcript into a network transcript.
8. **Typed diagnostics (LOW).** `AbortCode` and `Readiness` are named types
   with closed catalogs (`AbortCodes()`, `Valid()`); reason, advisory,
   provenance, role and command constants are exported. The CLI uses the
   constants; `NewAbortReport` panics on a code outside the catalog;
   `abortMessage`, `lifecycleAnnotation`, `reasonCode`, `sidecarAdvisory`,
   `remediation` and `prepareExit` are total switches that panic rather than
   emit a generic fallback. A wrong scratch length is now a **panic** —
   a programming error in the calling package — instead of a false
   `workspace-root-unopenable` abort the operator could neither reproduce nor
   remediate. Abort ↔ message ↔ lifecycle-line ↔ remediation bijections are
   asserted (AVP-095, AVP-101, AVP-153, AVP-181).

`internal/cli/cobra.go` gained one small factoring: `Execute` now delegates to
`execute(rootCmd, io.Writer)` so tests assert the real `error:` envelope and
the real exit code without shelling out. Behavior is unchanged.

## Implementation Errata (both preserved and documented)

1. **Negative-array-length cap guard.** `var _ [MaxArtifactBytes -
   MaxStatusBytes - 1]struct{}` is retained. The documented bare subtraction
   would not reject an inverted cap relationship; the array-length form fails
   to compile the moment the status cap stops being strictly smaller than the
   artifact cap. Preserved from rev-0 at reviewer instruction.
2. **Three `openFlags()` build halves.** `syscall.O_NONBLOCK` does not exist
   in `syscall` on `js/wasm` or `plan9`, so the accepted two-half partition
   does not build for the very targets the platform allowlist refuses. Three
   halves are required. Folded into PRD rev-6 and ADR-034 rev-3 errata
   (finding 2 above) rather than left as an undocumented deviation.

## Current State

- `prepare --check` remains a pure `internal/intent.Inspect` call over the
  three-method rooted seam, with one caller-owned `MaxArtifactBytes+1` scratch
  buffer reused by the status capture and all four artifacts.
- One root is opened after workspace discovery and closed after rendering.
  No writes, provider calls, subprocess calls or lifecycle changes.
- Existing `apply --mode prepare`, manual phase gates, `next` and `cycle` are
  byte-identical to the reconstructed `WAVE_BASE` binary; `apply --help`
  carries only the authorized pointer sentence.
- Known allowlisted untracked research WIP is untouched.
- Rev-3 changed **no production file** either: one comment block in
  `.github/workflows/ci.yml`, four `_test.go` files and this handoff.
- Rev-2 changed **no production file**. `git diff --stat` for rev-2 covers
  `.github/workflows/ci.yml`, six `_test.go` files, one comment line in
  `internal/intent/status_schema.go`, the goldens `README.md` and this
  handoff.

## Files Changed

### Rev-4 (this revision)

Commit: `REV4_COMMIT` (implementation + tracking, single commit).
CI: run `REV4_RUN`.

CI:

- `.github/workflows/ci.yml` — comment block only. The allowed-failure step's
  visibility claim is corrected: `continue-on-error: true` maps the step
  conclusion to `success`, so the failures are visible only in the step/job log
  output, not in the step conclusion, the job conclusion or the run summary.
  No key, condition, command or step ordering changed.

Tests:

- `internal/intent/avp_guard_helpers_test.go` — `workflowStep.ContinueOnError`
  and `workflowJob.ContinueOnError` become the raw scalar (captured as
  `rawValue`, before quote/comment normalisation); new
  `continueOnErrorMode`, `classifyContinueOnError`,
  `requireBlockingContinueOnError`, `requireAllowedFailureContinueOnError`.
- `internal/intent/avp_guards_test.go` — `checkCIWindowsGate` gains the
  job-level `if` rule, the whole-job `continue-on-error` ownership pass, the
  "exactly one advisory step and it is the Windows full-suite step" pin, and
  literal ownership on the allowed-failure step; AVP-175 sensitivity grows to
  24 failing arms plus 3 accepted variants.
- `internal/cli/prepare_avp2_test.go` — AVP-141's sensitivity scratch moves
  under the git directory; new `avp141ScratchName`, `avpGitDir` and
  `gitUntrackedGoFiles` helpers; the arm additionally asserts absence from
  `git ls-files` and from the sentinel's own untracked-source query.

Tracking:

- `docs/handoff/CURRENT.md` — this revision.

### Rev-3

Commits: `54ab8b4` (implementation), plus this tracking commit.
CI: run
[`32096303431`](https://github.com/tesseracode/tesserapatch/actions/runs/32096303431)
at `54ab8b4` — ubuntu ✅, macOS ✅, windows ✅, `release` skipped (not a tag).

CI:

- `.github/workflows/ci.yml` — comment block only: the blocking package set is
  attributed to run `32093250847` and described as pinned exactly by AVP-175;
  the allowed-failure comment carries the corrected 200/283/six-package/no-
  timeout inventory and claims visibility only in the step and the job log.

Tests and guards (no production file changed in rev-3):

- `internal/cli/land_rev1_fold_test.go` — `mixedCaseBaseCommit`, the
  no-op-mutation assertion shared by every arm, and
  `TestRev1Land_MixedCaseMangleIsNeverANoOp` (241 deterministic subtests).
- `internal/cli/prepare_avp2_test.go` — `gitLsFilesFromRepo`,
  `trackedProductionGoSources`, pure `countOpenRootCallSites`, and the new
  `AVP-141/sensitivity` arm.
- `internal/intent/avp_guard_helpers_test.go` — `parseWorkflowJobs`,
  `workflowJob`, `normalizeWorkflowCondition`, `platformCondition`;
  `runsOnWindows` (substring heuristics) removed.
- `internal/intent/avp_guards_test.go` — job-level and exact-condition checks,
  `checkBlockingWindowsStep`, `blockingWindowsPackages`, `nativeWindowsRows`,
  `minimumNativeLeaves`, `nativeRowLoop`, and eight further AVP-175
  sensitivity arms (fifteen in total).

Docs:

- `docs/handoff/CURRENT.md`

### Rev-2

Commits: `36f23b3` (implementation), `69dfe7c` (post-push CI-inspection fix),
plus this tracking commit.

CI:

- `.github/workflows/ci.yml` — blocking Windows GH #16 gate, visible
  `continue-on-error` full-suite step owned by GH #17, per-OS smoke-test legs,
  tag-version verification restored to both non-Windows legs.

Tests and guards (no production file changed in rev-2):

- `internal/intent/avp_guards_test.go` — `checkCIWindowsGate`, six-arm
  AVP-175 sensitivity, production-derived AVP-168 (`statusOutcomeProbes`,
  `deriveStatusOutcomes`, correspondence check), allocation honesty notes.
- `internal/intent/avp_guard_helpers_test.go` — `parseWorkflowSteps`,
  `workflowStep`, `runsOnWindows`.
- `internal/intent/avp_windows_guards_test.go` — `go list`-based GOOS
  constraint check replacing the literal suffix assertion.
- `internal/intent/avp_native_windows_test.go` — character-device row matches
  the real `!IsRegular` gate.
- `internal/intent/avp_source_scans_test.go` — `git ls-files` population,
  `checkInspectorImporters`, `AVP-134/sensitivity`.
- `internal/intent/status_schema_test.go` — error-returning extraction,
  untagged/embedded field rejection, unknown named types, named-alias
  underlying-kind check, four new sensitivity arms.
- `internal/intent/status_schema.go` — stale test filename in a comment.
- `internal/cli/prepare_routing_golden_test.go` — non-vacuous prefix
  matching, pinned fixture count, `provenance-is-verifiable`, `-trimpath`.
- `internal/cli/testdata/routing-goldens/README.md` — digest removed with the
  reason recorded; `-trimpath` recipe; worktree moved outside the repository.

Docs:

- `docs/handoff/CURRENT.md`

### Rev-1 (unchanged by rev-2)

Production:

- `internal/intent/intent.go`, `internal/intent/inspect.go`,
  `internal/intent/render.go`, `internal/intent/status_schema.go` (new),
  `internal/intent/openflags_unix.go`, `internal/intent/openflags_windows.go`,
  `internal/intent/openflags_unsupported.go`
- `internal/cli/prepare.go`, `internal/cli/cobra.go`

Tests:

- `internal/intent/harness_test.go` (new),
  `internal/intent/avp_classification_test.go` (new),
  `internal/intent/avp_status_test.go` (new),
  `internal/intent/avp_rooted_test.go` (new),
  `internal/intent/avp_guards_test.go` (new),
  `internal/intent/avp_guard_helpers_test.go` (new),
  `internal/intent/avp_document_guards_test.go` (new),
  `internal/intent/avp_source_scans_test.go` (new),
  `internal/intent/avp_ledger_test.go` (new),
  `internal/intent/status_schema_test.go` (new),
  `internal/intent/avp_native_windows_test.go` (new),
  `internal/intent/avp_windows_guards_test.go` (new),
  `internal/intent/fifo_tripwire_unix_test.go` (new),
  `internal/intent/fifo_tripwire_other_test.go` (new),
  `internal/intent/inspect_test.go` (removed — superseded)
- `internal/cli/prepare_test.go`, `internal/cli/prepare_avp_test.go` (new),
  `internal/cli/prepare_avp2_test.go` (new),
  `internal/cli/prepare_routing_golden_test.go` (new)
- `internal/cli/testdata/routing-goldens/` (new — 12 fixtures + `README.md`)
- `assets/avp_parity_test.go` (new)

CI:

- `.github/workflows/ci.yml` (LF checkout on `windows-latest`, so the job
  reaches `go test` and the native rows actually execute)

Docs:

- `docs/prds/PRD-artifact-validation-and-provenance.md` (rev-6 errata)
- `docs/adrs/ADR-034-rooted-filesystem-inspection-boundary.md` (rev-3 errata)
- `docs/adrs/README.md` (index row)
- `CHANGELOG.md`, `docs/handoff/CURRENT.md`

## Coverage

- **Acceptance rows**: 208 of 208 mapped, 224 references, zero duplicates.
- **Guards**: 43 of 43 registered, each with a paired sensitivity fixture that
  is asserted to fail the guard.
- **Tests**: 32 top-level AVP/ledger/schema tests and **485** subtests on this
  host (`intent` 21/309, `cli` 10/173, `assets` 1/3). `windows-latest`
  additionally runs `TestAVPNativeWindows` (2 rows, 6 leaf assertions) from
  the `GOOS`-constrained file, which is not compiled on this host.
- **Goldens**: 12 routing fixtures recorded from `WAVE_BASE` `9a8c1d0`
  (`go1.26.5 darwin/arm64`, `go build -trimpath`). The rev-1 binary digest was
  removed in rev-2 as unverifiable; see
  `internal/cli/testdata/routing-goldens/README.md` and
  `TestAVPRoutingGoldens/provenance-is-verifiable`.

## Test Results

### Rev-4

All results below are from **rev-4** on `go1.26.5 darwin/arm64`.

- `gofmt -l .`: clean.
- `go vet ./...`: PASS. `go build ./cmd/tpatch`: PASS.
- `go test -count=1 ./...`: PASS — 14 packages ok, 2 command packages have no
  tests. No skips introduced.
- `go test ./internal/intent -run 'TestAVPGuards/AVP-175' -v`: PASS — the
  guard, its 24 failing sensitivity arms and its 3 accepted variants.
- `go test ./internal/intent -run 'TestAVPWindowsSourceGuards' -v`: PASS — all
  four halves, including `AVP-175-ci-half`.
- `go test ./internal/cli -run 'TestAVPRootLifetimeAndDifferential/AVP-141'
  -v`: PASS — the row and its relocated sensitivity arm.
- Sentinel proof: after the AVP-141 run,
  `git ls-files --others --exclude-standard -- '*.go'` reports no
  `avp141-scratch` path, and `git status --short` shows no scratch entry.
  Negative control: a `.go` file written under the working tree *is* reported
  by that query, while the same file written under the git directory is not —
  which is the failure mode this revision removes.
- Ledger: `rows=208 categories=25 references=224 guards=43` — unchanged; no
  matrix row moved.
- CI at the rev-4 tip: run `REV4_RUN`.

### Rev-3

All results below are from **rev-3** on `go1.26.5 darwin/arm64`.

- `gofmt -l .`: clean.
- `go build ./...`: PASS. `go vet ./...`: PASS.
- `go test -count=1 ./...`: PASS (14 packages; 2 command packages have no
  tests).
- Flake proof: `go test ./internal/cli -count=100 -run
  'TestRev1Land_NonCanonicalBaseCommitIsRefused/mixed-case'`: PASS (100
  independent fixture repositories, 22.3s).
- Determinism proof: `TestRev1Land_MixedCaseMangleIsNeverANoOp`: PASS — 241
  subtests (six letters × 40 positions, plus the all-digit SHA);
  `go test -run 'TestRev1Land_' -v` reports 257 PASS lines in total.
- `go test ./internal/intent -run 'TestAVPGuards/AVP-175'`: PASS — the guard
  and all fifteen sensitivity arms.
- `go test ./internal/cli -run 'TestAVPRootLifetimeAndDifferential/AVP-141'`:
  PASS — the row and its new sensitivity arm.
- Ledger: `rows=208 categories=25 references=224 guards=43` — unchanged.
- `GOOS=windows GOARCH=amd64 go vet ./internal/intent ./internal/cli` and
  `go build ./cmd/tpatch`: PASS.
- CI at the rev-3 tip `54ab8b4`, run
  [`32096303431`](https://github.com/tesseracode/tesserapatch/actions/runs/32096303431):
  **all three matrix legs green**. See "Rev-3 run" below.

### Rev-2

All results below are from **rev-2** on `go1.26.5 darwin/arm64`.

- `gofmt -l .`: clean.
- `go build ./...`: PASS.
- `go vet ./...`: PASS.
- `go test -count=1 ./...`: PASS (14 packages; 2 command packages have no
  tests).
- Targeted: `go test -count=1 ./internal/intent ./internal/cli ./assets`:
  PASS.
- Ledger: `go test -count=1 ./internal/intent -run TestAcceptance`: PASS —
  `rows=208 categories=25 references=224 guards=43`.
- Guards: `go test -count=1 ./internal/intent -run TestAVPGuards`: PASS — 43
  guards, 43 executed sensitivity fixtures.
- Rev-2 targeted guards: `-run 'TestAVPGuards/AVP-175'`,
  `-run 'TestAVPGuards/AVP-168'`, `-run TestAVPSourceScans`,
  `-run TestAVPStatusSchema`, `-run TestAVPWindowsSourceGuards`,
  `-run TestAVPRoutingGoldens`: PASS.
- Asset parity (`TestSkillParityGuard`, `TestSkillDocReferencesAreSelfContained`,
  `TestAVPAssetParity`): PASS.
- `GOOS=windows GOARCH=amd64 go vet ./internal/intent ./internal/cli`: PASS.
- `GOOS=windows GOARCH=amd64 go build ./cmd/tpatch`: PASS.
- `GOOS=wasip1 GOARCH=wasm go build ./cmd/tpatch`: PASS (runtime still
  refuses via the platform gate).
- `GOOS=js GOARCH=wasm` and `GOOS=plan9 GOARCH=amd64 go build ./internal/intent`:
  PASS — the rev-6 errata's buildability claim, verified directly.
- Routing goldens re-recorded from the `WAVE_BASE` binary under the hermetic
  environment produced byte-identical files; rebuilding the current binary
  with `-trimpath` also produced byte-identical goldens.

## Windows CI History and the Rev-2 Gate

### What actually happened (corrected in rev-2)

1. At `WAVE_BASE` `9a8c1d0` the CI matrix was `ubuntu-latest` and
   `macos-latest` only. `main` was **green**: there was no Windows leg at all.
2. **GH #16 added the `windows-latest` row.** For its first runs the job died
   at `Verify formatting` in ~25s, because the runner checks out with
   `core.autocrlf=true` and `gofmt -l .` then lists the whole tree. The row
   existed but ran nothing.
3. `755b31e` forced an LF checkout, and the job reached `go test` for the
   first time. That is when the pre-existing failures in unrelated packages
   became visible. They were exposed by this wave, not caused by it — and the
   branch they were exposed on was green beforehand only because nothing
   Windows had ever run.
4. Rev-2 therefore splits the signal rather than reverting the row.
5. The **authoritative inventory** is the one measured with the suite running
   to completion: run
   [`32093250847`](https://github.com/tesseracode/tesserapatch/actions/runs/32093250847)
   — **200 failing top-level tests, 283 counting subtests**, in **six**
   packages, and **no package times out** at `-timeout 20m`. The rev-1 figure
   of "192 failures plus two 10-minute timeouts" was a *truncated* count: two
   packages aborted at the default per-package timeout, so the run never
   enumerated their remaining tests. Every "192 + timeouts" statement in this
   handoff, in `.github/workflows/ci.yml` and in GH #17 is superseded by the
   figures in this item.

The rev-1 handoff sentence "`windows-latest` was already red on `main` before
this wave" was wrong: there was no `windows-latest` before this wave. This
section replaces it.

### Rev-1 run — first real Windows execution

Run [`32086471429`](https://github.com/tesseracode/tesserapatch/actions/runs/32086471429)
at `755b31e`:

| Job | Result |
|---|---|
| `ubuntu-latest` | ✅ pass |
| `macos-latest` | ✅ pass |
| `windows-latest` | ❌ fail — **but for the first time it ran the suite** |

Package results on the Windows runner:

- **`internal/intent`: PASS (64s)** — this is the hard acceptance criterion.
  `TestAVPNativeWindows` executed on a real runner: `cmd /c mklink /J` created
  a real junction, the junction was refused via `ModeIrregular` with no
  `ModeSymlink` bit, a symlinked `spec.md` was `symlink-refused`, a reparse
  `status.json` aborted `status-symlink-refused`, `os.SameFile` was true for
  an unchanged file and false across a replacement, and a `FILE_TYPE_CHAR`
  handle was refused as non-regular.
- `assets`, `internal/buildinfo`, `internal/redact`, `internal/safety`,
  `internal/testutil`, `internal/tools/studyvalidator`, `tests/integration`:
  PASS.
- `internal/cli`, `internal/gitutil`, `internal/provider`, `internal/rescap`,
  `internal/store`, `internal/workflow`: FAIL. This run reported **192**
  distinct failures, but the number is a floor, not the inventory: `go test`
  ran at the default per-package timeout and `internal/cli`
  (`TestPatchGenerations_RecipeOnlySkipsGeneration`) and `internal/workflow`
  (`TestRev2_UnlandedParentProbeFailureNeverReplays`) each panicked at ten
  minutes, so the remaining tests in those two packages never ran. The
  corrected inventory, measured at `-timeout 20m` with nothing aborting
  early, is **200 top-level / 283 including subtests over six packages with
  no timeouts** — see the rev-2/rev-3 runs below.

**Zero of those failures are AVP, acceptance-ledger or `prepare` rows.**

### Rev-2 gate

- The empirically green set above is the **blocking** Windows step, with the
  native rows run verbosely and their `--- PASS:` lines asserted.
- The remaining failures are **visible** in a `continue-on-error` step whose
  name carries `#17`.
- [GH #17](https://github.com/tesseracode/tesserapatch/issues/17) is the
  explicit owner and the explicit exit condition: it acceptance-requires
  `go test -count=1 ./...` green on `windows-latest`, no package over the
  timeout, and **removal** of `continue-on-error` so the two steps collapse
  back into one blocking `go test ./...`. Until then a tagged release stays
  publishable, and AVP-175 forbids moving `./internal/intent` into the
  allowed-failure step.

### Rev-2 run

Run [`32091305245`](https://github.com/tesseracode/tesserapatch/actions/runs/32091305245)
at `36f23b3`:

| Job | Result |
|---|---|
| `ubuntu-latest` | ✅ success |
| `macos-latest` | ✅ success |
| [`windows-latest`](https://github.com/tesseracode/tesserapatch/actions/runs/32091305245/job/95573840737) | ✅ **success** (9m08s) |
| `release` | skipped (not a tag ref) |

Windows step-by-step: `Check out with LF line endings` ✅ → `checkout` ✅ →
`Set up Go` ✅ → `Verify formatting` ✅ → `go vet` ✅ → `Build` ✅ →
`Test` *skipped* (non-Windows only) → **blocking GH #16 step ✅** →
full-suite allowed-failure step (ran, failures visible, did not fail the job)
→ Windows smoke test ✅ → `Verify tag version` skipped (not a tag).

The blocking step's verbose native output, quoted from the run log:

```
--- PASS: TestAVPNativeWindows (0.07s)
    --- PASS: TestAVPNativeWindows/AVP-176 (0.05s)
        --- PASS: TestAVPNativeWindows/AVP-176/symlink-spec-is-symlink-refused (0.01s)
        --- PASS: TestAVPNativeWindows/AVP-176/junction-artifacts-is-symlink-refused (0.02s)
        --- PASS: TestAVPNativeWindows/AVP-176/status-reparse-point-aborts (0.01s)
        --- PASS: TestAVPNativeWindows/AVP-176/samefile-identity-over-root-lstat-and-file-stat (0.01s)
        --- PASS: TestAVPNativeWindows/AVP-176/char-device-handle-is-not-regular (0.00s)
    --- PASS: TestAVPNativeWindows/AVP-199 (0.02s)
        --- PASS: TestAVPNativeWindows/AVP-199/junction-helper-fails-never-skips (0.02s)
```

Six leaf assertions executed — the step's own `--- PASS:` grep floor. The
blocking package list then passed: `internal/intent` (42.8s), `assets`,
`internal/buildinfo`, `internal/redact`, `internal/safety`,
`internal/testutil`, `internal/tools/studyvalidator`, `tests/integration`.

The allowed-failure step remains fully visible in the log. At `-timeout 20m`
**no package times out** (rev-1's two 10-minute timeout panics are gone), so
the suite runs to completion and reports **200 failing top-level tests, 283
counting subtests**, in `internal/cli` (379.7s), `internal/gitutil` (39.7s),
`internal/provider` (13.6s), `internal/rescap` (14.7s), `internal/store`
(5.9s) and `internal/workflow` (409.4s). The count is higher than rev-1's
truncated 192 precisely because nothing aborts early any more; the population
is the same pre-existing class and is owned by GH #17. `internal/intent` passes inside
this step too.

### Rev-2 follow-up: YAML comment truncation in step names

Inspecting the run surfaced a real defect in the first rev-2 commit. YAML
reads ` #` in an *unquoted* plain scalar as a comment, so
`name: Test (Windows full suite — allowed to fail, owned by GH #17)` was
actually named `Test (Windows full suite — allowed to fail, owned by GH` on
the runner — the issue reference the guard requires was silently dropped by
the workflow engine while the guard's parser still saw it in the file. Both
Windows step names are now **quoted**, the two smoke-test steps have distinct
names, and `parseWorkflowSteps` truncates unquoted plain scalars at ` #` so it
sees exactly what the workflow engine sees. AVP-175 gains a seventh
sensitivity arm that unquotes the allowed-failure name and requires the guard
to fail.

Confirming run
[`32092224021`](https://github.com/tesseracode/tesserapatch/actions/runs/32092224021)
at `69dfe7c` (the wave tip) —
[windows-latest job](https://github.com/tesseracode/tesserapatch/actions/runs/32092224021/job/95576546336),
11m02s, ✅ **success**; ubuntu ✅, macOS ✅, `release` skipped. The step names
now survive YAML intact:

| Step | Conclusion |
|---|---|
| `Test` | skipped (non-Windows only) |
| `Test (Windows GH #16 surface — blocking)` | ✅ success |
| `Test (Windows full suite — allowed to fail, owned by GH #17)` | ran, failures visible, job unaffected |
| `Install binary (smoke test)` | skipped (non-Windows) |
| `Install binary (smoke test, Windows)` | ✅ success |
| `Verify tag version` | skipped (not a tag ref) |

Native rows executed again on the runner (all six leaves PASS, including
`char-device-handle-is-not-regular` under the corrected `!IsRegular` gate).
The allowed-failure step still reports **200 failing top-level tests** across
`internal/cli` (546.4s), `internal/workflow` (589.6s), `internal/gitutil`
(44.6s), `internal/rescap` (16.5s), `internal/provider` (13.7s) and
`internal/store` (7.5s), with `internal/intent`, `assets`,
`internal/buildinfo`, `internal/redact`, `internal/safety`,
`internal/testutil`, `internal/tools/studyvalidator` and `tests/integration`
passing. The interim shape and this inventory were posted to
[GH #17](https://github.com/tesseracode/tesserapatch/issues/17#issuecomment-5322764984).

### Rev-3 run — the green tip

Run [`32096303431`](https://github.com/tesseracode/tesserapatch/actions/runs/32096303431)
at `54ab8b4`:

| Job | Result |
|---|---|
| `ubuntu-latest` | ✅ success |
| `macos-latest` | ✅ success |
| [`windows-latest`](https://github.com/tesseracode/tesserapatch/actions/runs/32096303431/job/95588230519) | ✅ success |
| `release` | skipped (not a tag ref) |

The immediately preceding run
[`32093932654`](https://github.com/tesseracode/tesserapatch/actions/runs/32093932654)
at `b95232e` is the red tip the external review reported: `windows-latest` and
`ubuntu-latest` passed and **`macos-latest` failed** — the mixed-case
base-commit no-op, reproduced by chance on one leg. That is exactly the flake
rev-3 finding 1 removes; the fixture SHA no longer decides whether the row's
mutation mutates anything.

The blocking Windows step's verbose native output, quoted from the rev-3 log:

```
--- PASS: TestAVPNativeWindows (0.06s)
    --- PASS: TestAVPNativeWindows/AVP-176 (0.04s)
        --- PASS: TestAVPNativeWindows/AVP-176/symlink-spec-is-symlink-refused (0.01s)
        --- PASS: TestAVPNativeWindows/AVP-176/junction-artifacts-is-symlink-refused (0.02s)
        --- PASS: TestAVPNativeWindows/AVP-176/status-reparse-point-aborts (0.01s)
        --- PASS: TestAVPNativeWindows/AVP-176/samefile-identity-over-root-lstat-and-file-stat (0.00s)
        --- PASS: TestAVPNativeWindows/AVP-176/char-device-handle-is-not-regular (0.00s)
    --- PASS: TestAVPNativeWindows/AVP-199 (0.01s)
        --- PASS: TestAVPNativeWindows/AVP-199/junction-helper-fails-never-skips (0.01s)
```

Six leaf assertions, the step's own floor. The pinned blocking package set
then passed in full: `internal/intent` (5.5s), `assets`, `internal/buildinfo`,
`internal/redact`, `internal/safety`, `internal/testutil`,
`internal/tools/studyvalidator`, `tests/integration`.

The allowed-failure step reproduces the corrected inventory exactly:
**200 failing top-level tests, 283 counting subtests, zero
`panic: test timed out`**, across six packages — `internal/cli` (564.4s),
`internal/workflow` (615.5s), `internal/gitutil` (46.8s), `internal/rescap`
(16.3s), `internal/provider` (13.6s) and `internal/store` (9.0s) — with
`internal/intent`, `assets`, `internal/buildinfo`, `internal/redact`,
`internal/safety`, `internal/testutil`, `internal/tools/studyvalidator` and
`tests/integration` passing inside that step too. GH #17's body now carries
this inventory.

## Next Steps

1. Reviewer: run the ledger (`go test ./internal/intent -run TestAcceptance`)
   and the guard suite (`-run TestAVPGuards`), and spot-check that each
   sensitivity fixture genuinely breaks its guard.
2. Reviewer: confirm the PRD rev-6 / ADR-034 rev-3 errata are narrow — no
   decision changed, matrix still 208 rows, guard set still 43.
3. Reviewer: inspect the rev-4 CI run recorded above; the `windows-latest`
   **job must be green**, the blocking step must show
   `--- PASS: TestAVPNativeWindows/...` leaf lines, and the allowed-failure
   step must still be present and still list the unrelated failures.
4. Reviewer: check AVP-175's **24** sensitivity arms individually — in
   particular the rev-4 additions
   `job-level-continue-on-error-expression`,
   `job-level-continue-on-error-variable`, `job-level-if-false`,
   `blocking-step-continue-on-error-expression`,
   `blocking-step-continue-on-error-negation`, `vet-step-demoted` and the
   three `allowed-failure-continue-on-error-*` arms — and the **3 accepted
   variants**, which must still pass: `job-level-continue-on-error-false`,
   `blocking-step-continue-on-error-false` and `job-level-if-empty`.
5. Reviewer: confirm AVP-141's sensitivity scratch lands under the git
   directory and that an interrupted run cannot leave an untracked `.go` file
   the wave-close `[2/8]` sentinel would flag.
6. Reviewer: re-run
   `go test ./internal/cli -count=100 -run 'TestRev1Land_NonCanonicalBaseCommitIsRefused/mixed-case'`
   — the rev-2 tip's 2.3%-per-run no-op is what made the review tip red.
7. If approved, run the Wave-Close Checklist and flip the handoff Status.

## Blockers

- No implementation blocker.
- Hard block for every mutating prepare slice until this wave is accepted and
  landed.

## Context for Next Agent

- `internal/intent` still must not import `internal/store`. The status schema
  is mirrored locally on purpose and kept honest by the AST parity guard; if
  `store.FeatureStatus` gains a field, that guard fails until the mirror is
  updated.
- The guard registry is the single source of truth for the 43 `G` rows.
  Adding a matrix row whose Kind contains `G` without registering a guard
  fails AVP-139 and the ledger automatically.
- Routing goldens must never be re-recorded from the current binary. If they
  need refreshing, reconstruct the `WAVE_BASE` binary in a temporary detached
  worktree exactly as the testdata `README.md` documents — **outside** the
  repository, so nothing untracked appears in the working tree.
- The Windows CI shape is a contract, not a convenience. `AVP-175` parses
  `.github/workflows/ci.yml` into **jobs and steps** and fails if the GH #16
  surface stops being blocking, if the `test` job or the blocking step becomes
  continue-on-error, if any gating step's `if` stops being exactly
  `runner.os == 'Windows'` / `runner.os != 'Windows'`, if any blocking package
  leaves the pinned set or appears in an allowed-failure command, if the
  AVP-176/199 row loop or the six-leaf floor is weakened, if the
  allowed-failure step stops naming GH #17, if the tag-version condition
  changes, or if the release job stops declaring `needs: test`. When GH #17 lands, deleting the
  `continue-on-error` step is a deliberate AVP-175 edit, not a silent one.
- **Out of scope, pre-existing (2)**: the Windows test failures above —
  **200 top-level, 283 including subtests, six packages, no timeout at
  `-timeout 20m`** (run 32093250847) — are owned by
  [GH #17](https://github.com/tesseracode/tesserapatch/issues/17): path
  separators, symlink/permission assumptions and runtime cost. The two
  10-minute per-package timeouts seen in rev-1 were an artefact of the default
  timeout and do not recur at 20m.
- **Out of scope, pre-existing (1)**: `GOOS=js GOARCH=wasm go build ./cmd/tpatch`
  fails in `internal/rescap` (`pathopen_unix.go` references
  `syscall.O_NOFOLLOW` under a `!windows`-shaped tag). This reproduces at
  `WAVE_BASE` `9a8c1d0` unchanged and is the same failure class the rev-6
  errata fixes inside `internal/intent`. It is **not** touched by this wave;
  it deserves its own ticket.
- Do not modify the ROADMAP, supervisor LOG, HISTORY, or research WIP. The
  untracked research files in `git status` predate this wave.

## Rev-1 Review and Rev-2 Adjudication

**Internal verdict**: APPROVED
**External verdict**: NEEDS REVISION (CI-scoped blocker)
**Implementation tips**: `2cbccf6`, `755b31e`, `b98fac9`
**Tracking tip**: `49ae6ad`
**Windows backlog**: [GH #17](https://github.com/tesseracode/tesserapatch/issues/17)

1. **Blocking Windows signal.** This wave added `windows-latest`; main was green
   at WAVE_BASE and became red. Split a blocking Windows step that runs the
   empirically green GH #16 surface (including explicit verbose native test)
   from a visible `continue-on-error` full-suite step owned by GH #17. Preserve
   tagged releases.
2. **AVP-175 sensitivity.** Guard that `internal/intent` is in the blocking
   step and absent from the non-blocking step; sensitivity must fail when moved.
3. **Status outcome guard.** Derive AVP-168 from the production classifier,
   never a test-local literal map.
4. **Source-scan robustness.** Scan tracked files or explicit package roots;
   remove `.golden-baseline` name coupling and tolerate unrelated worktrees.
5. **Schema parity holes.** Assert named store types retain string underlying
   kinds and fail on exported untagged fields.
6. **Golden routing rigor.** Require every expected prefix to match at least one
   fixture; eliminate vacuous `continue`. Make baseline binary provenance
   reproducible with `-trimpath` or drop the unverifiable hash.
7. **Windows guard cleanup.** Remove constant-literal suffix check, fix
   character-device comment/assertion, correct stale status-schema test
   filename, and restore tag-version verification on both non-Windows legs.
8. **Tracking truth.** State correctly that GH #16 added the Windows row and
   exposed pre-existing package failures; do not call the pre-wave main branch
   red.

The 208-row ledger, 43 sensitivity pairs, native Windows intent behavior,
status fix, errata and routing goldens remain accepted and must not regress.

## Rev-0 Review and Rev-1 Adjudication

**Internal verdict**: NEEDS REVISION
**External verdict**: NEEDS REVISION
**Writer tip**: `0440337`
**Tracking tip**: `a587fad`

1. **Status schema fidelity (HIGH).** The status ladder must reject any known
   `FeatureStatus` field with the wrong JSON type, not decode only `state`.
   Preserve the core's no-`internal/store` import boundary with a locally
   mirrored/validated shape plus a source-parity guard.
2. **Platform partition erratum (MEDIUM).** `syscall.O_NONBLOCK` is undefined
   on unsupported targets, so buildability requires three compile halves:
   `unix`, `windows`, `!(unix || windows)`. Record PRD/ADR rev-6 errata for
   AVP-118/208 and D5/D6; unsupported still refuses before root opening.
3. **AVP acceptance evidence (HIGH).** Add mechanical traceability for all
   `AVP-001…AVP-208`, genuine behavior coverage by category, and all 43
   guard+sensitivity pairs. A spelling-only ledger is insufficient.
4. **Native Windows hard gate (HIGH).** Add windows-only runtime tests,
   junction creation that fails rather than skips, identity/reparse behavior,
   and CI/guard checks for AVP-175/176/198/199.
5. **Pre-change routing goldens (HIGH).** Reconstruct the `9a8c1d0` binary,
   commit baseline routing fixtures for AVP-136/137, and compare current
   behavior byte-for-byte. Record provenance; do not silently waive the
   prerequisite.
6. **Race/read/allocation coverage (HIGH).** Exercise injected instability,
   EOF taxonomy, exact cap boundaries, one-buffer allocation, lifecycle lines,
   all abort codes and root/close ordering. Fix the fake reader so multi-read
   behavior is real.
7. **Typed diagnostics (LOW).** Export/use closed abort constants, remove
   string-literal codes and generic default fallbacks, and treat wrong scratch
   length as a programming error rather than a false root-open abort.

The negative-array-length cap guard remains correct and must be preserved.
