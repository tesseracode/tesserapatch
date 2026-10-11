# Current Handoff

## Status

**Cluster state**: IN PROGRESS

The operator authorizes reproducing and resolving the post-close Ubuntu
reapply failure before GH #13. Previous maintenance acceptance at `12617b1`
is preserved in HISTORY; the later `c492318` run is a separate failing result.
No permanent admission-policy adoption or GH #13 implementation is authorized.
The operator subsequently approved a pressure-based exception for this fix only.

## Active Task

- **Task ID**: `fix-linux-canonical-reapply`
- **Milestone**: Bounded correctness follow-up, before GH #13
- **Description**: Reproduce and resolve the S5 canonical-reapply failure on Linux
- **Status**: Original Linux failure and deterministic guards reproduced; correction next
- **Assigned**: 2026-10-11
- **WAVE_BASE**: `c4923185ba6fe01b4558b138e41f30fab0015762`

WAVE_BASE = c4923185ba6fe01b4558b138e41f30fab0015762

## Session Summary

The fresh baseline is clean on main, HEAD equals fetched origin/main at
`c492318`, with no stash and one worktree. Native CI
[38118826831](https://github.com/tesseracode/tesserapatch/actions/runs/38118826831)
fails `test (ubuntu-latest)` in
`TestRGAS5CLIUsesCapturedRecipeAndBypassesCanonicalReapply`.
At `recipe_authority_s5_cli_test.go:222`, canonical reapply returns:
`reapply produced no source diff against the committed unapplied baseline`.
The other four required jobs pass. No production/test source changed between
the fully passing `12617b1` and failing `c492318`.

The failure reaches canonical reapply's materialization check, not a reported
OOM or admission timeout. Linux reproduction now establishes the index-copy
timestamp mechanism; do not suppress the materialization assertion.

A Git-only reproducer now confirms that copying an index with a newer
timestamp can suppress Git's racy-entry content check and hide a same-stat
source change. Preserving the original timestamp restores the exact diff
without changing the real index. Both deterministic Go guards fail on Linux,
and the original unmodified CLI test reproduces the same line-222 error in
a capped 1000-repeat fail-fast run (17.270s package time before failure).
The private reproducer is `bin/reapply-linux-20261011/reproduce-index-copy.py`.
Deterministic Go regressions cover normal and redirected effective indices.

## Current State

Trace the fixture and the real execute/reapply path, then reproduce its exact
Git/source state on Linux. Preserve the single captured-recipe-read assertion,
canonical reapply's bypass of recipe-only classification, source-diff/preimage
checks and immutable recipe authority. No unrelated testing optimization.

## Files Changed

Tracking: CURRENT, HISTORY, ROADMAP and the supervisor log.
`internal/gitutil/diff_index_test.go`: deterministic copied-index regression cases.
No production correction has been applied yet.

## Test Results

- Existing failure: native Ubuntu run `38118826831` at `c492318`.
- Existing passing comparison: complete local gate and native CI `38114748869`
  at source-equivalent `12617b1`.
- Git-only same-stat/index-copy reproducer passes its expected empty-versus-real
  diff controls on the current Mac; real index remains unchanged.
- Linux/arm64, Go 1.26.5: both new index regression cases fail exactly as
  expected against unchanged production code.
- The original CLI test reproduces the exact Ubuntu failure during a bounded
  repeated run; execution stops at the first failure.
- The initial formatting check stopped on a misplaced new test declaration.
  Its placement is corrected in a focused test file; no Go test/compile ran
  in that failed attempt. The private log is retained.

## Next Steps

1. Preserve the source index timestamp in the private diff copy, using metadata
   and bytes from the same open source file.
2. Rerun the deterministic guards and original CLI repetitions without
   suppressing real refusal or changing the original fixture.
3. Run focused then affected validation, Linux confirmation, required broader
   checks and independent review under the applicable resource rules.
4. Preserve evidence and tracking at each transition; close/push only after
   actual validation. GH #13 and later profile adoption remain separate.

## Blockers

The Ubuntu failure blocks #13 readiness until the correction validates.
Docker Desktop is running; all completed task containers were removed.
The completed Linux reproductions and source files survived the request error.

## Context for Next Agent

The operator approved a new exception specifically for this fix: a continuous
sixty-second normal-pressure/no-new-swap/load1<=5 window, with external Go
allowed and own work serial. Runtime pressure, swap and telemetry stops remain.
Default admission and permanent adoption are unchanged. Do not reuse the
exception for another task. Docker execution is restricted to this task's
container/checkout; do not modify other containers or the original worktree
from inside it.

Preserve all previous evidence in `bin/resource-baseline/` and
`bin/maintenance-close-20261010/`, the older host's original checkout/research,
the accepted #13 contracts and the existing v0.17.0 tag.
Only explicit-path staging and required Copilot co-author trailers are allowed.
