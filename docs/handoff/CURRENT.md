# Current Handoff

## Status

**Cluster state**: ACCEPTED

The Linux canonical-reapply failure is resolved and accepted. The unchanged
original test and deterministic regressions pass 1000 repetitions; the complete
local gate passes 8/8; exact-checkpoint native CI passes all five required jobs.
No next task is dispatched. Permanent admission-policy adoption and GH #13
remain separate assignments.

## Active Task

- **Task ID**: `fix-linux-canonical-reapply`
- **Milestone**: Bounded correctness follow-up, before GH #13
- **Description**: Preserve Git's same-stat content checks in private scoped-diff indices
- **Status**: Complete — corrected, validated and accepted
- **Assigned**: 2026-10-11
- **Completed**: 2026-10-11
- **WAVE_BASE**: `c4923185ba6fe01b4558b138e41f30fab0015762`
- **Regression checkpoint**: `0c38c15`
- **Production correction**: `25cbf747d73e282a4994dd22bf4d3e3cea882907`
- **Validated checkpoint**: `608714bc13d87178b5141306b5c9f7bcc5e41c1e`
- **Archival acceptance checkpoint**: `d55183c5b2d2d5755a84f609dc33615796c4e806`

WAVE_BASE = c4923185ba6fe01b4558b138e41f30fab0015762

## Session Summary

Native run `38118826831` exposed the original line-222 error in
`TestRGAS5CLIUsesCapturedRecipeAndBypassesCanonicalReapply`:
`reapply produced no source diff against the committed unapplied baseline`.
The failure was reproduced on Linux without changing that test.

`DiffFromCommitForPaths` copied index bytes into a newly timestamped file.
The newer timestamp could suppress Git's racy-entry content check, falsely
reporting no difference for same-stat source edits. The correction reads
metadata and bytes from the same open effective-index descriptor and preserves
mtime on the private copy before invoking Git. Timestamp errors fail explicitly;
the real index, literal scope and materialization checks remain intact.

Both new normal-index and redirected-index guards fail before the correction.
Afterward, they and the original unchanged CLI test pass 1000 repetitions.
Full gitutil/workflow packages, selected CLI consumers, formatting, vet and
build pass on Linux. Direct bounded review reports no findings; no independent
subagent review is claimed.

The final mechanical gate and native CI both pass at exact checkpoint
`608714b`. Subsequent closing edits are tracking/report-only and do not change
the validated implementation or protected contracts.
The terminal accepted range is
`c4923185ba6fe01b4558b138e41f30fab0015762..d55183c5b2d2d5755a84f609dc33615796c4e806`;
this final tracking follow-through records those known endpoints for publication.

## Current State

The original correctness blocker is resolved. There is no broad testing
optimization, new replay behavior, assertion weakening or recipe-authority
change in this fix. GH #13 has not started.

An added Unreleased changelog heading was rejected by the existing versioned
parity guard during the first full attempt. It was removed, not exempted:
CHANGELOG is byte-for-byte identical to the wave base, and all assets validators
are unchanged. The fix remains documented here and in the case-study report.

## Files Changed

- `internal/gitutil/gitutil.go`: preserve the effective index's mtime in the
  private scoped-diff copy, with metadata/bytes read from the same descriptor.
- `internal/gitutil/diff_index_test.go`: deterministic same-stat regressions
  for normal and redirected indices, including real-index preservation.
- Tracking: CURRENT, HISTORY, ROADMAP and the supervisor log.
- Evidence report:
  `docs/state-of-the-art/case-studies/validation-resources-2026-10/linux-reapply-index-timestamp.json`.

CHANGELOG, original CLI tests, assets, SPEC, CI/shard definitions and dependencies
have no net changes in this wave.

## Test Results

- Original unmodified CLI failure and both deterministic guards reproduce
  on unfixed Linux source; no compile/format failure is credited as reproduction.
- Corrected guards and original CLI test: 1000 repetitions pass.
- Affected gitutil/workflow packages, S5/apply/unapply consumers, formatting,
  vet and CLI build pass; restored-changelog assets tests pass.
- Full gate at `608714b`: **8/8**, no warnings; exact parity and success for
  all **25 top-level commands / 22 test commands**.
- All 25 safety windows replay successfully. Across 1931 samples, pressure is
  normal and swapping remains zero; the run takes 3842.77s. All 375 sampled
  owned identities are reconciled, with none left and no extra termination.
- Exact-checkpoint [native CI 38164854684](https://github.com/tesseracode/tesserapatch/actions/runs/38164854684):
  **SUCCESS**, all five required jobs pass, release skipped. The pre-existing
  allowed-failure Windows full-suite step remains separately owned by GH #17.

The initial misplaced test declaration, root-container permission failures
and first full attempt's changelog parity failures remain preserved as failed
attempts. They do not count toward final passing coverage.

## Next Steps

No implementation task is dispatched. The operator may separately adopt a
validation admission policy, then assign GH #13 S0's frozen regression baseline
under the accepted ADR-037/ADR-043/PRD rev-7 contract. Do not start production
replay changes before S0 or silently make this task's profile the default.

## Blockers

None for this correction. No local validation workload remains running.

## Context for Next Agent

The operator approved a pressure-based exception only for this fix: sixty
continuous seconds of normal host pressure, no new swap-outs and load1 <=5
before each Go command, external Go allowed, own work serial, with runtime
pressure/swap/telemetry stops. Permanent defaults remain unchanged.

Docker Desktop was started with approval; task containers are removed.
Container verification uses an unprivileged identity for permission fixtures.
Host Docker-client RSS is not guest Go memory. The memory-heavy final partition
ran on the 32 GiB Mac, not Docker's approximately 4 GiB VM.

Preserve raw evidence under `bin/reapply-linux-20261011/`, including both
`final-gate/evidence/` (failed) and `final-gate-retry/evidence/` (passed).
Preserve prior maintenance/older-host evidence and check existing results
before restarting any work after a CLI error.

The assets parity guard rejects an Unreleased heading beside the selected
versioned contract. Keep unshipped maintenance notes in tracking until an
authorized release transition. No release/tag was created: v0.17.0 remains
at `d1d6c3fff6cdbab4b5bb9f6f7054b2ce9b8667e8`, annotated tag object
`e7c748ada40988fe72f47b638cb37d07e24f0ac3`.
