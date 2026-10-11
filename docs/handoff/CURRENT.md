# Current Handoff

## Status

**Cluster state**: IN PROGRESS

The operator authorizes reproducing and resolving the post-close Ubuntu
reapply failure before GH #13. Previous maintenance acceptance at `12617b1`
is preserved in HISTORY; the later `c492318` run is a separate failing result.
No admission-policy adoption or GH #13 implementation is authorized yet.

## Active Task

- **Task ID**: `fix-linux-canonical-reapply`
- **Milestone**: Bounded correctness follow-up, before GH #13
- **Description**: Reproduce and resolve the S5 canonical-reapply failure on Linux
- **Status**: Investigating and preparing an isolated reproduction
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
OOM or admission timeout. Root cause and reproducibility are not yet established;
do not dismiss the failure as harmless flakiness or weaken the assertion.

## Current State

Trace the fixture and the real execute/reapply path, then reproduce its exact
Git/source state on Linux. Preserve the single captured-recipe-read assertion,
canonical reapply's bypass of recipe-only classification, source-diff/preimage
checks and immutable recipe authority. No unrelated testing optimization.

## Files Changed

Tracking only so far: CURRENT, HISTORY, ROADMAP and the supervisor log.
No production/test change or new Go validation has run.

## Test Results

- Existing failure: native Ubuntu run `38118826831` at `c492318`.
- Existing passing comparison: complete local gate and native CI `38114748869`
  at source-equivalent `12617b1`.
- No new reproduction or correction result claimed.

## Next Steps

1. Isolate the failing fixture and canonical materialization path; establish
   a deterministic reproducer and the smallest complete correction.
2. Add regression and negative coverage without suppressing real refusal.
3. Run focused then affected validation, Linux confirmation, required broader
   checks and independent review under the applicable resource rules.
4. Preserve evidence and tracking at each transition; close/push only after
   actual validation. GH #13 and later profile adoption remain separate.

## Blockers

The Ubuntu failure blocks #13 readiness. The Linux execution route and new
Go-validation admission are not yet established.

## Context for Next Agent

The operator explicitly defers admission-profile adoption until after this fix.
Do not silently reuse the previous final-run pressure-only exception.
Existing local Go-validation admission requires a continuous sixty-second
window at >=80% on the established macOS admission metric, load1 <=5 and no
active Go/compile/link/vet/test tools; own Go work remains serial.
Do not run Go checks before their required admission.

Preserve all previous evidence in `bin/resource-baseline/` and
`bin/maintenance-close-20261010/`, the older host's original checkout/research,
the accepted #13 contracts and the existing v0.17.0 tag.
Only explicit-path staging and required Copilot co-author trailers are allowed.
