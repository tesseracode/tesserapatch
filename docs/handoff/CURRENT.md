# Current Handoff

## Status

**Cluster state**: IN PROGRESS

GH #13 S2 is explicitly assigned. S1 and the 39-shard validation maintenance
remain accepted. No S3+, candidate execution, phase wiring or deferred
upstream Git-strategy research is authorized.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s2`
- **Milestone**: GH #13 / ADR-037 and ADR-043
- **Description**: S2 eligibility, paired evidence recomputation, offline
  reconstruction and default-off configuration
- **Status**: Production code approved; correcting three dependent source-inventory registrations
- **Assigned**: 2026-10-29
- **WAVE_BASE**: `bd95a237cbf5b5894f21c19c27bb1d628808db8f`
- **Contract**: accepted ADR-037/PRD rev-7, ADR-043 and ADR-044
- **Prior accepted S1 validation checkpoint**: `24be224e46cb0569de3488bf466467e9c252fd8b`
- **Frozen S2 review checkpoint**: `2c462a3086f9dbb89f322a19494f6d474960944b`

WAVE_BASE = bd95a237cbf5b5894f21c19c27bb1d628808db8f

## Session Summary

The operator requested S2. Preflight confirms clean HEAD equals fetched
origin/main at `bd95a23`, no stashes and one main worktree. S1 closing-tip
CI `40987744660` was still running at dispatch and has now passed.
S1's independently reviewed code and exact-checkpoint CI/full gate at
`24be224` are already accepted; the complete history is archived.

Read PRD S2 and the latest paired C/E eligibility block B. Existing production
strict decoders, capture-event pair validation and coverage reconstruction
live in `internal/workflow/`; use them rather than weakening or duplicating
decoders. A whole pair validator checks later conditions too, so it cannot
be invoked early if doing so shadows prescribed first-match E1-E15 reasons.
The draft adds an unexported ordered eligibility evaluator with a fifteen-bit
in-process result, but no all-present confirmation proof or serialized schema.
It reads repository opt-in/state before bound artifacts, reuses strict decoders
and canonical E identity, reconstructs bounded images through S1 typed roots,
and reuses pure coverage assignment/simulation helpers only at their proper
gate. The evaluator is not called by the reconcile pipeline yet.

Read-only ReferenceRoot methods reuse patchobs's reconstruction core with a
candidate-specific hardened environment. Existing producer readers retain
their original capture environment; no global Git environment policy changes.
The result retains hashes/state only, not reconstructed body arrays or roots.
Repository config emits the new key only for explicit true, preserving
gate-off YAML bytes. The S2 scope allowlist extends the stage boundary only;
S1 and golden-capture permissions remain unchanged.
Draft `d5f921e` passes initial focused workflow/S0/S2, candidate-envelope and
config tests. Before broader validation, strengthen upstream reconstruction:
checking only the tree object misses an absent affected blob and accepts an
annotated tag's object ID. The typed root now checks affected object availability
through bounded metadata plumbing, without retaining upstream bodies or adding
another image-size limit. Also normalize inherited Git variable names before
scrubbing, since Windows names are case-insensitive. Regressions must fail
against the earlier draft; no new API/phase behavior is added.

Independent S2 reviewer: `dee26d79-d708-4b4c-8a3d-84c4478c5f0a`,
explicitly requested OpenAI `gpt-5.3-codex` (confirm runtime on return).
It is read-only and runs no Go. Parent shell `roc-s2-review-validation`
owns formatting, full affected packages, vet and build in sequence.
Clean-checkout `gofmt -l .` now passes. No duplicate validation is allowed.
The eventual full-gate harness is prepared in `bin/roc-s2-20261029/full-gate/`;
its nine routing controls pass, but it is not configured or running yet.
Independent review now APPROVES frozen `2c462a3`, with no significant issues.
Runtime metadata confirms OpenAI `gpt-5.3-codex`. The canonical APPROVED field
records code-review status only; S2 is not accepted until validation completes.
Broader affected-package validation finds three legacy source inventories
needing explicit S2 transition: the new candidate reference file, the moved
strict-normalizer call owner, and the eligibility evaluator's readonly
coverage-consumer role. Patchobs and store pass; candidate/gitutil/workflow
report these inventory failures. There is no resource stop; vet/build do not
start after the failed command.

Correct only those registrations with omission/writer/unknown-reader negative
controls. Keep the evaluator outside the pure-core allowlist, retain exactly
eleven strict normalizer calls, and preserve the original S1/capture scope.
Production code remains frozen; return the test-only correction for narrow
independent review after admitted validation.

## Current State — bounded S2 scope

1. Gate-off remains inert. Add `operation_candidate_enabled`, default false,
   without wiring candidate behavior into the reconcile pipeline.
2. Feature state must be exactly applied or blocked before E1.
3. Implement the fifteen gates in PRD section 6.5's fixed order, with E4 and
   E11's specified sub-order and deterministic first-failure reasons.
   The accepted paired-evidence block B controls the current numbering.
4. Reuse ADR-036's own strict coverage/recipe decoders and ADR-041's capture
   event decoder/canonical encoder. E binds the exact raw C hash; canonical
   E encoding supplies its identity digest, never a fabricated historical proof.
5. Recompute readable presence, raw patch/recipe hashes, reference/preimage
   facts, patch/effect observations, event facts and parent exclusions.
   Do not trust generations, labels or current parent state as capture evidence.
6. Reconstruct required trees offline with explicit no-fetch checks, using
   the accepted S1 boundaries; no candidate writes or live recipe execution.
7. Preserve supersession, hard-blocked-parent, invocation-shape and pending
   shadow refusals. Keep S0 golden/capture and S1 source/lifetime guards intact
   while making the finite S2 scope transition explicit.

S3 alignment/classification, S4 execution, candidate identity/artifact schemas,
publication/lifecycle transactions, S6 phase/evidence wiring and public
docs/assets changes remain out of scope.

## Files Changed

Production draft: `internal/workflow/operation_candidate_eligibility.go`,
`internal/candidate/reference.go`, shared reconstruction/environment
plumbing in `internal/patchobs/{gitread,reconstruct}.go`, and repository
config in `internal/store/{types,store}.go`.
Directly coupled hardening: case-insensitive routing/config scrubbing in
`internal/gitutil/hardened_env.go`, with its existing test file extended.
Tests: new S2 workflow runtime/source guards and candidate reference tests;
store roundtrip regression; finite S2 stage permissions in the S0 scope guard.
Dependent inventory tests also register `reference.go`, the relocated strict
normalizer call, and the evaluator's readonly consumer functions; no pure-core
or general publication exemption is added.
Tracking: CURRENT, ROADMAP and supervisor LOG. No CLI/phase/asset edits.

## Test Results

- Initial admitted formatting passes (`s2-draft-format-20261029`).
  The initial focused suite will cover all
  eighteen gate reasons, first-match/sub-check order, all refused lifecycle
  states, canonical E identity, malformed/unreadable inputs, no live writes,
  captured parent exclusion, no-fetch/environment and config roundtrip behavior.
- `d5f921e` focused suite PASS: workflow 8.771s, candidate 1.018s, store
  0.502s; stage `s2-draft-focused-20261029`, fresh admission, no resource stop.
  Upstream-object/case-variant follow-ups are not yet validated.
- The new upstream-tag, missing-blob and case-insensitive-environment
  regressions fail against the earlier draft at their intended assertions,
  stage `s2-offline-negative-20261029`. This is a deliberate negative control,
  not a current-code result. Formatting of the corrections passes; preserve
  that failure evidence and run the corrected affected packages next.
- Corrected source is frozen at `2c462a3`; broader positive checks and
  independent review are pending. The initial focused pass and intentional
  negative controls do not establish S2 acceptance.
- The affected-package run has three inventory assertion failures only:
  candidate production-file list, gitutil strict-normalizer call owner, and
  workflow coverage-consumer registration. Patchobs/store pass; other runtime
  assertions report no failure. Stage `s2-review-affected-packages-20261029`
  has no resource stop. Vet/build remain unstarted after fail-fast.
- Prior S1 gate: 8/8 checks, 42/42 top-level commands including 39/39 tests.
  Native CI `40975085201` passed all five required jobs and actual Windows
  CLW-001 through CLW-005 at `24be224`.
- S1 closing-tip CI `40987744660` at `bd95a23` is SUCCESS.

## Next Steps

1. Await the existing parent-owned `roc-s2-review-validation` checks at
   independently approved `2c462a3`; do not launch duplicate Go work.
2. Resolve any findings and complete formatting/vet/build, full 39-command
   and native validation before acceptance.
3. Keep dispatch/progress durable without cancelling S1 closing-tip CI.
   Do not start S3 automatically.

## Validation Policy — GH #13

Require 60 continuous seconds of normal OS pressure, no new swap-outs and
load1 <=5 before every Go validation command. External Go is observed, not
vetoed; own work is serial (`GOMAXPROCS=1`, `-p=1`). Percentage is diagnostic;
historical occupied swap alone is not a veto. Stop on warning/critical
pressure, new swap-outs, unreliable telemetry or gaps over 5s. Retain 600s
admission budgets, `-count=1`, 40-minute test-process timeouts, clean isolated
source/caches and owned-process reconciliation. The canonical partition has
39 test commands; do not revert to historical 22-command recipes.

## Blockers

No prior-CI blocker remains. S2 independent review and broader validation are
active; no acceptance or publication is authorized by focused passes alone.

## Context for Next Agent

S1 create/prune borrow caller-held feature/global lock capabilities; caller
acquires feature then global and releases global then feature. Handle copies
share borrow state. Do not reintroduce internal acquisition/release or another
bare-root constructor.

Preserve S0 goldens, the original strict capture boundary and the nineteen
explicit AR memo-scope insertions. Do not add an Unreleased heading, alter
the frozen ADR index, create a release/tag or reopen accepted architecture.
v0.17.0 remains tag object `e7c748ada40988fe72f47b638cb37d07e24f0ac3`,
targeting `d1d6c3fff6cdbab4b5bb9f6f7054b2ce9b8667e8`.

All prior raw measurements remain under ignored `bin/` evidence directories;
do not delete them. Current full-gate measured peak was 6.99 GiB, not the
smaller six-pilot peak. Research `research-upstream-git-strategies` stays deferred.
