# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**GH #13 S9 dispatched: docs, parity, downstream soak and release closeout.**
S0-S8 are accepted. S8 closed durably at
`faeedd38a7b1056945864b1964bc55632eae7416`; that commit's required CI
41774958353 is successful. Its terminal handoff was already archived exactly
in HISTORY under 2026-11-04T04:41:58Z and is not archived a second time.
S9 has no implementation, review or validation result yet.

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
- **Status**: In progress - scope and ownership recorded before source edits
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

### Decisions for morning review

| ID | Decision and reason |
|---|---|
| S9-D1 | Complete GH #13 first. GH #9 is the nested-repository wave-close sentinel bug, not upstream-Git research and not a prerequisite for semantic replay. |
| S9-D2 | Treat S9 as contract alignment and evidence, not a new replay architecture. Preserve the accepted default-off gate, explicit acceptance, provider fallback, authority/transaction semantics and no-history-rewrite boundary. |
| S9-D3 | Release v0.18.0 only after source approval, a fresh complete gate and required native CI. Existing Windows full-suite debt remains explicitly under GH #17, not silently declared green. |
| S9-D4 | Conditional GH #9 needs a new recorded wave base and a separate review/full-gate cycle. The preceding full gate took 3h37m26s, before implementation/review/closure overhead; do not spend the remaining window on an unfinishable second wave. |
| S9-D5 | Keep one source writer and parent-controlled guarded validation. Use fresh OpenAI/MAI agents; do not repurpose idle S8 agents or run unguarded Go locally. |
| S9-D6 | Preserve all S5-S8 evidence and use fresh S9 source/cache/evidence paths. Network recovery may reconnect only the existing profile, within the operator's explicit bounds. |

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
No S9 source file has been changed yet.

## Test Results

- Baseline closure CI **41774958353**: successful at exact faeedd3.
- Baseline/source/archive/deadline readiness checks: passed before dispatch.
- **S9 formatting, tests, independent review, full gate and native CI: not run.**

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

1. Commit/push dispatch tracking, then implement the bounded S9 surface.
2. Freeze source; guarded format/check and independent review, revising findings.
3. Complete focused/affected checks and a fresh full gate plus exact-tip native CI.
4. Accept/archive S9, push/tag/publish v0.18.0, verify release/CI and close GH #13.
5. Reassess GH #9 against the remaining fixed window; either dispatch a separate
   fully budgeted wave or explicitly defer it. Stop before any unrelated issue.

## Blockers

None at dispatch. GH #17 is known separate debt, not silently part of this run.

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
