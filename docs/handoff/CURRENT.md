# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**GH #13 S9 dispatched: docs, parity, downstream soak and release closeout.**
S0-S8 are accepted. S8 closed durably at
`faeedd38a7b1056945864b1964bc55632eae7416`; that commit's required CI
41774958353 is successful. Its terminal handoff was already archived exactly
in HISTORY under 2026-11-04T04:41:58Z and is not archived a second time.
The revised 18-file S9 implementation is frozen, independent rev-0 source
review is APPROVED and all six guarded formatting/focused commands passed.
This is the reviewed-source checkpoint for the fresh full gate, not final
S9 acceptance. Complete full-gate/native CI and release remain pending.
The exact reviewed source was pushed at
`c18f02545ec11ac7e8653ab8e8814ce9e0758a68`, but native CI **41795299460**
exposed a directly coupled AP prose-hash regression. Acceptance is reopened.
The incomplete old-Mac gate was identity-safely interrupted at 08:03Z with
no owned leftovers. Preserve both failed/interrupted evidence sets; rev-1
must pass independent review and a new complete gate.

The operator authorized a ten-hour overnight window beginning
**2026-11-04T06:20:23Z**, ending **2026-11-04T16:20:23Z**
(November 3 22:20 through November 4 08:20 PST). The deadline is anchored to
the request, not restarted after preparation or clarification.

The single clarification set explicitly authorizes:

1. Finish S9 and everything directly necessary to close GH #13.
2. Tag and publish the planned **v0.18.0** only after independent review,
   complete guarded validation and required native CI pass.
3. Then address **GH #9 only if sufficient time remains for its own complete
   implementation/review/validation/closure cycle**, and only after GH #13
   is closed and CI passes. Otherwise record deferral; do not reduce checks.
4. Reconnect the existing NetBird profile if it drops, solely to restore
   old-Mac access. No credential, account or configuration changes; pause
   if login or elevated privileges are required.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s9`
- **Milestone**: GH #13 / ADR-037 / v0.18.0
- **Description**: S9 docs, nine-region parity, frozen-fixture/adjacent-argument soak and fully validated GH #13 release closeout
- **Status**: Rev-1 in progress - native AP prose-hash regression; acceptance reopened
- **Assigned**: 2026-11-04
- **WAVE_BASE**: `faeedd38a7b1056945864b1964bc55632eae7416`
- **Contract**: accepted PRD rev-7, section 6.0, S9 and ROC-001 through ROC-306; accepted S1-S8 ADRs
- **Owner**: one implementation agent; parent owns tracking, every Go invocation and release; independent source reviewer
- **Deadline**: `2026-11-04T16:20:23Z`

WAVE_BASE = faeedd38a7b1056945864b1964bc55632eae7416

## Session Summary

Fetched origin before dispatch and confirmed clean HEAD=origin/main at the
wave base, no stash, one main worktree and no merge/rebase/cherry-pick/index
lock marker. S8's existing terminal archive matches CURRENT byte-for-byte:
49,279 bytes, SHA256
`ae03ba05a76d720851efe11f7426081716b9d7d6dfe6cc37642bd07afd46d56d`.
The existing HISTORY is 1,105,048 bytes, SHA256
`bcfbdb0581112bb70190efdc9e1d3713143068e7d76c0d6f795b9cc6d1009adc`.
Its prefix must remain unchanged when S9 is eventually archived.

The operator approved conditional GH #9 follow-up, v0.18.0 publication and
bounded existing-profile VPN reconnection. These permissions supersede S8's
expired no-S9/no-release/no-#9 dispatch limits; they do not authorize unrelated
migration, coverage expansion, upstream-Git/rebase research or GH #17 repair.
A fixed-deadline timer is active, with PID/start-identity readiness verified.

Dispatch tracking is pushed at `ce1d962a62091355ac1addda67552466564b8426`.
The single source implementer is `da7756ac-c9cb-4bdc-b037-65a1c88a7a71`,
explicitly selected as OpenAI `gpt-6-astra`. It owns only the paths below and
will return frozen source before any parent-run Go validation or review.

At 2026-11-04T06:50Z the parent prepared a fresh full-history old-Mac
checkout at the dispatch commit and an isolated targeted-validation harness.
All five pinned guard/profiler/shard hashes match the accepted S8 harness;
all nine unchanged fake-backend routing controls pass. Evidence/cache
directories are still absent and no Go has run. Preflight found normal
pressure, load1 1.49, 76% diagnostic free memory and no active Go process.
No VPN reconnection, setting change or application termination was needed.

### Initial handback and bounded amendments

The implementer returned 16 frozen source files with docs/assets, exact-byte
nine-region parity and wrong-input controls, S9 scope admission and the real
record/reconcile/review/accept adjacent-argument soak. It ran structural Python
checks and `git diff --check`, but no Go or gofmt. The initial bytes and hash
manifest are preserved in ignored `initial-handback.tar.gz` / JSON.

Parent inspection confirmed two directly coupled closeout gaps:

- `assets/recipe_authority_parity_test.go` selects v0.17.0 as current and
  deliberately rejects any newer heading. Advance the active-contract
  selection to v0.18.0, retaining strict heading selection and all authority /
  editor-failure disclosure validators and wrong-input controls. Do not solve
  this by exempting the new release or relaxing the parser. Historical
  v0.17.0 bytes remain unchanged; add needed current-release disclosure.
- `RefuseResolverActionWhenCandidatePending` prints
  `tpatch reconcile <slug> --reject-candidate` and a bare acceptance flag,
  while the shipped terminal parser requires `--reject-candidate <slug>` /
  `--accept-candidate <slug>` with no positional args. Correct only this
  guidance, preserve the error/state behavior, and exercise both printed
  commands through the real CLI on fresh pending-candidate fixtures.

These are not new architecture or extra backlog work. The same implementer
may edit the two additional paths below; parent still owns all validation.

The revised handback is frozen at **2026-11-04T07:17:38Z**: 18 files, six Go
files, no implementation blocker reported. Parent verified the complete path
set against the authorized scope, unchanged HISTORY and all historical
v0.17.0-and-earlier changelog bytes. The sole production Go delta is the
confirmed one-line command-guidance correction; error/state logic is unchanged.
The strict release parser/predicates are reported unchanged, with the current
selection and disclosure controls advanced to v0.18.0. Actual runtime proof
and independent review remain outstanding. The implementer is idle/read-only.
Frozen source/archive: `rev0-source-sha256.json`, `rev0-source.tar.gz` and
`rev0-freeze.json` under the ignored S9 evidence directory.

Guarded formatting completed and was reconciled at **07:24:05Z**. The six-file
formatter and whole-repository empty-output check both passed, with separate
61.2-second stable admissions, pressure level 1 and zero new swap-outs.
All four sampled owned identities ended; no blocked marker remains.
Only `assets/operation_candidate_parity_test.go` and the S0 scope guard changed
under gofmt. Parent verified all 18 local pre-format hashes before mirroring
those formatter deltas, then verified the complete formatted manifest.
The independent reviewer receives these exact frozen bytes; the implementer
remains read-only while parent runs focused validation.

Independent reviewer `739406f9-1709-4f8e-a899-c48d8568eb03` was explicitly
selected with OpenAI `gpt-6-astra`. The parent-only focused batch
`roc-s9-focused` started at 07:29:51Z; at 07:31:06Z its PID/start identity and
live guarded asset-test stage were verified. The batch runs assets, ROC0-9
workflow/CLI regressions, vet and build serially, stopping at its first
failed command. No source edits while either review or validation is active.

The independent rev-0 verdict is **APPROVED - source only**. The reviewer
verified all 18 hashes, exact-byte/same-validator parity controls, both real
adjacent-argument soaks, actual suggested-command execution, preserved release
parser/predicates with genuinely newer-version controls, and unchanged
protected boundaries. No significant finding. Returned metadata confirms
the selected OpenAI model. Runtime, native CI and the fresh complete gate
remain separate requirements, not implied by this verdict.

At **2026-11-04T07:51:37Z** the parent audited the completed focused batch:
all six formatting/test/vet/build commands pass with exact argument parity.
The verbose logs contain 160 top-level PASS records and all 20 required
exact markers for the new parity/scope/soak/suggested-command cases and
current-release controls. Each command had a separate 61.2-second admission;
all samples show normal pressure and zero new swap-outs. Maximum within-stage
sampling gap is 2.134 seconds; all 312 sampled owned identities and the
launcher have ended. No blocked marker remains. All 18 local/remote source
hashes still match the independently reviewed formatted manifest.
Evidence: `rev0-focused-audit.json` and `rev0-checks-evidence.tar.gz` under
the ignored S9 directory. The archive SHA256 is
`fe3906daa8abff8de8b02ac7cd24bfdc3207daa755fbbf6ba192cba637ce148f`.

The canonical APPROVED field now records the independently approved,
focused-tested source checkpoint required before the existing mechanical
gate. It does not declare the remaining acceptance workload or release done.
Publish this checkpoint, then keep origin frozen while the fresh 42-command
gate and exact-tip native CI run. Any failure reopens review; do not tag,
close GH #13 or dispatch GH #9 on focused evidence alone.

At **07:53:45Z** the fresh full-gate preparation passed at pushed checkpoint
`c18f02545ec11ac7e8653ab8e8814ce9e0758a68`. The separate full-history checkout
is clean, all 18 reviewed hashes and the protected v0.17.0 tag match, all five
pinned harness/profiler/shard hashes match, and all nine unchanged fake-backend
routing controls pass. Its command inventory is exactly 42 commands /39 tests.
Evidence, caches and runner identity are absent before launch. Bundle SHA256:
`920f641a03e483ed91023b4fed6841a4356030c63413932a2418168600623b10`.
Remote source: `s9-final-c18f025-source`; gate: `s9-full-gate-c18f025`, under
the existing approved old-Mac validation root. Native run **41795299460** is
bound to the same SHA. Both acceptance lanes are now dispatched; results
remain pending. Subsequent progress tracking stays local to freeze origin.
The complete gate started at **07:54:46Z**; at **07:55:10Z** its PID 34822 /
start identity, make child and guarded `01-gofmt` admission were verified.
The native observer is responsive and all five required jobs are in progress
at the same SHA. Attached executions: `roc-s9-full-gate` and
`roc-s9-native-ci`; do not launch duplicate Go work or restart either gate.

### Rev-0 native failure and bounded rev-1

At **08:00:24Z**, Ubuntu job **126458448586** in CI **41795299460**
failed `TestRGAS6DanglingDocDeltaAndSensitivity`: S9's six reviewed skill
bodies differ from the still-pinned S6 expected prose hashes used by
`validateS7APDanglingOwnedSurfaces`. This is a real coupled regression,
not Windows GH #17 debt or an infrastructure waiver. The original focused
ROC0-9 selector did not run this RGA/AP dependency.

Parent preserved the raw Ubuntu log and job/run/SHA metadata, stopped the
incomplete gate at **08:03:15Z** by its freshly verified PID/start identity,
and verified **all 97 sampled/descendant identities gone** at 08:03:16Z.
The existing gate/evidence is interrupted, not passed and not reusable.

Rev-1 is limited to a separately pinned S9 transition over the preserved S6
prose projection, its existing AP-validator hook and corresponding reviewed
current-source provenance. Preserve all historical AP/S6 hashes, semantic
validators and negative controls. Prove missing/extra/drifted/already-projected
inputs fail and that unapproved current body changes still fail through the
actual AP validator. Do not derive expected hashes from live docs at test time.
The existing 18-file handback stays frozen except the S9 CLI tests and, only
if necessary, the narrowly bounded S0 scope guard. Additional authorized files:
`internal/cli/prepare_s7_ap_dangling_test.go` (projection hook only) and
`internal/cli/prepare_pib_golden_test.go` (that AP entry's CurrentSHA256 only).
Do not edit `recipe_authority_s6_doc_delta_test.go` or historical registry
SHA256/History/FreezeCurrent. Report any further required scope before editing.

### Decisions for morning review

| ID | Decision and reason |
|---|---|
| S9-D1 | Complete GH #13 first. GH #9 is the nested-repository wave-close sentinel bug, not upstream-Git research and not a prerequisite for semantic replay. |
| S9-D2 | Treat S9 as contract alignment and evidence, not a new replay architecture. Preserve the accepted default-off gate, explicit acceptance, provider fallback, authority/transaction semantics and no-history-rewrite boundary. |
| S9-D3 | Release v0.18.0 only after source approval, a fresh complete gate and required native CI. Existing Windows full-suite debt remains explicitly under GH #17, not silently declared green. |
| S9-D4 | Conditional GH #9 needs a new recorded wave base and a separate review/full-gate cycle. The preceding full gate took 3h37m26s, before implementation/review/closure overhead; do not spend the remaining window on an unfinishable second wave. |
| S9-D5 | Keep one source writer and parent-controlled guarded validation. Use fresh OpenAI/MAI agents; do not repurpose idle S8 agents or run unguarded Go locally. |
| S9-D6 | Preserve all S5-S8 evidence and use fresh S9 source/cache/evidence paths. Network recovery may reconnect only the existing profile, within the operator's explicit bounds. |
| S9-D7 | Use the existing tag-triggered CI release job, not a new packaging pipeline or early manual publication. Like v0.17.0, this is a source release without binary assets; the existing job publishes only after all required test and observer jobs pass. |
| S9-D8 | Advance the recipe-authority guard's selected current release to v0.18.0, rather than weakening its heading parser or silently treating current guidance as historical. Retain the authority/disclosure rules, hostile controls and all v0.17.0 release bytes. |
| S9-D9 | Fix the confirmed malformed candidate terminal instructions in the pending-resolver error. This is a narrow CLI-text correction to match existing parsing, with real-command regression coverage; no acceptance/state semantics change. |
| S9-D10 | Native CI exposed a preserved AP guard still selecting S6 prose hashes. Add an explicit pinned S9 projection and reviewed current-hash amendment, rather than weakening the guard, overwriting historical oracles or blessing live docs dynamically. |
| S9-D11 | Expand rev-1 focused checks to the actual RGA/AP dependency and provenance/observer guards. Preserve rev-0 failure/interruption evidence and rerun complete acceptance from fresh source/cache/evidence; no partial-pass stitching. |

No new architecture choice has been made. Any later architecture change
requires an ADR and explicit recorded rationale before implementation.

## Current State

S0-S8 are accepted. S8's complete fresh gate passed 42/42 commands and
39/39 uncached test commands with exact parity, normal pressure and zero
new swap-outs. Its reviewed checkpoint was
`ffb032307bc3d6b0fd8744da04e8f1c78884f3d9`; closure faeedd3 changed only
tracking and ADR acceptance evidence. These are baseline facts, not S9 passes.

S9 must update all three SPEC section 7 phase-2 rows together; add ADR-036
D15 P3 and ADR-025 evidence-taxonomy notes; align both confirmation-gate PRDs;
guard the nine precisely delimited PRD/ADR-037 regions; update all six shipped
formats and assets parity; and soak frozen fixtures plus both adjacent-CLI
argument variants. New guards require genuinely failing wrong-input fixtures
through their actual validators, not token/count-only substitutes.

The dedicated Windows S8 lane is blocking and proved 26 required markers,
52 top-level passes and no skips. The separate allowed-failure full suite
has known GH #17 failures and its known panic. Compare failure identities,
not counts alone; one oversized-attribute baseline case has varied between
223 and 224 total failures. No new failure may be waived as baseline debt.

### Ownership and edit boundaries

The implementation agent may edit:

- `SPEC.md`, `CHANGELOG.md`.
- `docs/adrs/ADR-036-recipe-coverage-authority.md`.
- `docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md`.
- `docs/prds/PRD-upstreamed-confirmation-gate.md`.
- `docs/prds/PRD-confirm-upstreamed-human-review-path.md`.
- `assets/assets_test.go`, optional new `assets/operation_candidate_parity_test.go`.
- `assets/recipe_authority_parity_test.go`, solely for the current-release
  selector and corresponding non-weakened controls/disclosure coverage.
- `assets/skills/claude/tessera-patch/SKILL.md`.
- `assets/skills/copilot/tessera-patch/SKILL.md`.
- `assets/skills/cursor/tessera-patch.mdc`.
- `assets/skills/windsurf/windsurfrules`.
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`.
- `assets/workflows/tessera-patch-generic.md`.
- New `internal/workflow/operation_candidate_s9_test.go` and
  `internal/cli/operation_candidate_s9_test.go`, if needed.
- `internal/workflow/operation_candidate_s0_guards_test.go`, solely for a
  justified S9 test-file scope admission with its negative control preserved.
- `internal/workflow/operation_candidate_actions.go`, solely the corrected
  terminal-command guidance in `RefuseResolverActionWhenCandidatePending`.

Parent exclusively owns CURRENT/LOG/ROADMAP/HISTORY, validation artifacts,
commits/pushes/tags/releases and issue state. No same-file parallel writers.
Any required additional edit surface or directly coupled production defect
must be reported before expanding this set. The implementer runs no Go,
gofmt, build, vet, cross-compile or release command and makes no commit.

Preserve production Go behavior, frozen S0 fixtures/goldens, nineteen accepted
AR memo-lifetime insertions, dependency manifests, the frozen ADR index,
existing CI partitions/timeouts/allow-failure semantics, resource guards,
unrelated research, `.wave-close-allowlist` and the existing v0.17.0 tag.
Never set `TPATCH_UPDATE_ROC_S0_GOLDENS`. Approved provenance amendments use
`CurrentSHA256`; baseline `SHA256`, `History` and `FreezeCurrent` stay intact.

## Files Changed

- `docs/handoff/CURRENT.md`: fresh S9 scope replacing the already archived S8 handoff.
- `docs/ROADMAP.md`: S9 in-flight entry and authorized conditional sequencing.
- `docs/supervisor/LOG.md`: authorization, bounds and dispatch decision.

Ignored operational evidence starts at `bin/roc-s9-20261104-0620/`.
The final source handback changes SPEC, CHANGELOG, both named ADRs, both
confirmation PRDs, `assets/assets_test.go`, the new asset parity test, all six
named shipped formats, the S0 scope guard and the new CLI S9 soak test
plus `assets/recipe_authority_parity_test.go` and
`internal/workflow/operation_candidate_actions.go` (18 files; exact paths in
`rev0-source-sha256.json`). No other production Go, dependency, CI or frozen
fixture path changed.

## Test Results

- Baseline closure CI **41774958353**: successful at exact faeedd3.
- Baseline/source/archive/deadline readiness checks: passed before dispatch.
- Isolated old-Mac guard-routing controls: 9/9 passed using fake Python
  backends; source clean, fresh evidence/cache paths absent, no Go executed.
- Initial implementation structural checks and `git diff --check`: passed;
  these do not establish compilation, runtime behavior or acceptance.
- Private native-evidence parser: archived positive, eight rejecting controls
  and the documented 223/224 failure variant pass; no native run is claimed.
- Guarded S9 formatting: **2/2 commands passed**, whole-repository output empty;
  normal pressure, zero new swap-outs and zero owned leftovers.
- Independent rev-0 source review: **APPROVED**, no significant findings.
- Guarded focused batch: **4/4 commands passed** (assets; ROC0-9 workflow/CLI;
  vet; build), **6/6 including formatting**. Exact command/source parity,
  160 top-level PASS records and all 20 required exact markers verified.
- Resource/process audit: pressure 1, zero new swap-outs, separate >=60-second
  admissions, maximum sample gap 2.134 seconds, all 312 sampled identities
  and launcher ended; no blocked marker.
- Fresh full-gate preparation: clean full history, reviewed hashes, pinned
  harness, 9/9 routing controls and exact 42/39 inventory verified; isolated
  cache/evidence absent before launch.
- **S9 fresh full gate and exact-tip native CI 41795299460: dispatched,
  not accepted.** Ubuntu's first Test command failed the RGA S6/AP prose-body
  guard. Old-Mac gate interrupted safely before source revision; no full pass.

Every local/old-Mac Go or gofmt invocation remains under the accepted wrapper:
60 continuous seconds of normal OS pressure, no new swap-outs and load1 <=5;
600-second admission budget; runtime stop on warning/critical pressure, any
swap-out growth, unreliable telemetry or sampling gap >5 seconds. Free-memory
percentage and existing swap occupancy are diagnostic, not admission vetoes.
External Go is observed, not vetoed; all our local/old-Mac Go remains serial.
GitHub native jobs on separate hosts may overlap.

Use the private Go1.26.5 toolchain with GOTOOLCHAIN=local, GOENV=off, empty
GOFLAGS, GOWORK=off, BASH_ENV=/dev/null, GOMAXPROCS=1, -p=1 and advisory
GOMEMLIMIT=3GiB. Tests are uncached with the accepted per-process timeout.
Stop at the first command failure; preserve evidence and blocked markers.
No blind retry, weakened thresholds, shorter partitions or partial-pass stitching.

Acceptance requires the exact fresh full-history, isolated-cache 42-command /
39-test-command gate at a frozen pushed reviewed tip, with
`make wave-close-check WAVE_BASE=faeedd38a7b1056945864b1964bc55632eae7416`,
exact parity, mechanical PASS, no owned leftovers, clean source and unchanged
protected tags. Run the nine unchanged routing controls first. Native CI uses
its existing Go1.26.1 configuration; do not silently relabel it Go1.26.5.

## Next Steps

1. Implement only the bounded rev-1 AP prose-projection/provenance correction.
2. Independently re-review, run expanded guarded focused checks, then publish
   a new checkpoint and run fresh complete gate/exact-tip native CI.
3. Accept/archive S9, push/tag/publish v0.18.0, verify release/CI and close GH #13.
4. Reassess GH #9 against the remaining fixed window; either dispatch a separate
   fully budgeted wave or explicitly defer it. Stop before any unrelated issue.

## Blockers

Rev-0 acceptance is blocked by the coupled AP prose-hash regression above.
All previous passes remain historical evidence only. Rev-1 needs scoped
implementation, independent review, expanded focused validation and fresh
complete/native acceptance. GH #17 remains known separate debt.

## Context for Next Agent

This authorization is active only through 2026-11-04T16:20:23Z. Stop safely
at the deadline, preserve source/evidence and update tracking before handing off.
Do not reset the clock after a restart. The timer is
`roc-s9-overnight-deadline`; its identity is in the ignored deadline JSON.

Existing baseline evidence is under `bin/roc-s8-20261103/`. S8's expired
launchers contain a different checkpoint and deadline; do not reuse them
unmodified. Reuse the accepted guard/profiler bytes, not old results.
The nine parity IDs are the closed set A/B/C1/C2/D1/D2/D3/E/F; the literal
`<ID>` illustration is not a region. Compare exactly enclosed bytes, not
surrounding prose or whitespace-normalized approximations.

The prior upstream-maintenance/rebase research intake remains backlog only,
as recorded in ROADMAP. GH #9 is instead the nested-repository source sentinel
bug; it must preserve allowlists, avoid `.git` internals and prove a false-clean
fixture fails. Do not confuse the two follow-ups.
