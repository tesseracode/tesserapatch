# Current Handoff

## Status

**Cluster state**: REV-2 DISPATCHED

Rev-1 closes every rev-0 implementation finding and is accepted in substance,
but external review found a Windows CI release blocker and narrow guard-quality
issues. Rev-2 is dispatched. No mutating prepare mode is authorized.

## Active Task

- **Task ID**: `implement-prepare-check`
- **Issue**: [GH #16](https://github.com/tesseracode/tesserapatch/issues/16)
- **Description**: Implement
  `PRD-artifact-validation-and-provenance` rev-5 + ADR-034 rev-2.
- **Status**: Rev-2 dispatched after split rev-1 verdict
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

## Files Changed

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
- **Tests**: 32 top-level AVP/ledger/schema tests and 479 subtests on this
  host (`intent` 21/304, `cli` 10/172, `assets` 1/3). `windows-latest`
  additionally runs `TestAVPNativeWindows` (2 rows, 5 leaf assertions) from
  the `GOOS`-constrained file, which is not compiled on this host.
- **Goldens**: 12 routing fixtures recorded from `WAVE_BASE` `9a8c1d0`
  (binary SHA-256 `c06c205cc8a819aa8bb4e10eb8542c4b5174793920cfcec56b1b57d2d8388de5`,
  `go1.26.5 darwin/arm64`).

## Test Results

- `gofmt -l .`: clean.
- `go build ./...`: PASS.
- `go vet ./...`: PASS.
- `go test -count=1 ./...`: PASS (14 packages; 2 command packages have no
  tests).
- Targeted: `go test -count=1 ./internal/intent/ ./internal/cli/ ./assets/`:
  PASS.
- Asset parity (`TestSkillParityGuard`, `TestSkillDocReferencesAreSelfContained`,
  `TestAVPAssetParity`): PASS.
- `GOOS=windows GOARCH=amd64 go vet ./internal/intent ./internal/cli`: PASS.
- `GOOS=windows GOARCH=amd64 go build ./cmd/tpatch`: PASS.
- `GOOS=wasip1 GOARCH=wasm go build ./cmd/tpatch`: PASS (runtime still
  refuses via the platform gate).
- `GOOS=js GOARCH=wasm` and `GOOS=plan9 GOARCH=amd64 go build ./internal/intent`:
  PASS — the rev-6 errata's buildability claim, verified directly.
- Routing goldens re-recorded from the `WAVE_BASE` binary under the hermetic
  environment produced byte-identical files.

## CI Observation After Push (recorded, not acted on beyond scope)

Run [`32086471429`](https://github.com/tesseracode/tesserapatch/actions/runs/32086471429)
at `755b31e`:

| Job | Result |
|---|---|
| `ubuntu-latest` | ✅ pass |
| `macos-latest` | ✅ pass |
| `windows-latest` | ❌ fail — **but for the first time it ran the suite** |

`windows-latest` now clears `Verify formatting`, `go vet` and `Build`, and
reaches `Test`. Package results on the Windows runner:

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
  `internal/store`, `internal/workflow`: FAIL — **192 distinct pre-existing
  Windows failures**, plus a 10-minute per-package timeout in `internal/cli`
  (`TestPatchGenerations_RecipeOnlySkipsGeneration`) and `internal/workflow`
  (`TestRev2_UnlandedParentProbeFailureNeverReplays`).

**Zero of the 192 failures are AVP, acceptance-ledger or `prepare` rows.**
They are Windows path/symlink/permission assumptions in code this wave does
not touch, which had never executed on Windows because the job always died at
formatting in ~25 seconds. Fixing them is a separate, sizeable ticket and is
explicitly out of this wave's authorized surface; raising the per-package
timeout was likewise left alone, because it would only remove noise from a job
that still fails for unrelated reasons. `windows-latest` was already red on
`main` before this wave — the change is that its redness is now *true* instead
of *vacuous*.

## Next Steps

1. Reviewer: run the ledger (`go test ./internal/intent -run TestAcceptance`)
   and the guard suite (`-run TestAVPGuards`), and spot-check that each
   sensitivity fixture genuinely breaks its guard.
2. Reviewer: confirm the PRD rev-6 / ADR-034 rev-3 errata are narrow — no
   decision changed, matrix still 208 rows, guard set still 43.
3. Reviewer: inspect CI; `windows-latest` must now pass `Verify formatting`,
   reach `Test`, and execute `TestAVPNativeWindows` with real assertions.
   Before this wave the job died at formatting and ran nothing.
4. If approved, run the Wave-Close Checklist and flip the handoff Status.

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
  worktree exactly as the testdata `README.md` documents.
- **Out of scope, pre-existing (2)**: the 192 Windows test failures above.
  Recommend a dedicated ticket: "make the non-`intent` packages pass on
  windows-latest", covering path separators, symlink/permission assumptions
  and the two per-package 10-minute timeouts.
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
