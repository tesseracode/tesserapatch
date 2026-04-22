# M6 — Bug Bash Validation

**Status**: ✅ Complete  
**Depends on**: M4, M5

## Tasks

- [x] M6.1 — Create test scripts: `tests/setup-reconciliation-test.sh`, `tests/mock-provider.go`
- [x] M6.2 — Clone `tesseracode/copilot-api` at commit `0ea08febdd7e3e055b03dd298bf57e669500b5c1`
- [x] M6.3 — Run full lifecycle for Feature A (model translation fix): init → add → analyze → define → apply → record
- [x] M6.4 — Run full lifecycle for Feature B (models CLI subcommand): add → analyze → apply → record
- [x] M6.5 — Verify both features work: `bun test` 26/26, `bun run typecheck` clean
- [x] M6.6 — Simulate upstream update: checkout upstream/master, remove Feature B files
- [x] M6.7 — Run `tpatch reconcile` — Feature A → `upstream_merged` (Phase 3), Feature B → `reapplied` (Phase 4)
- [x] M6.8 — Verify target repo healthy after reconciliation: 26/26 tests pass, typecheck clean
- [x] M6.9 — Patch validation: both patches pass `git apply --check` against their baselines
- [x] M6.10 — Document results in `tests/tpatch/BUG-BASH-REPORT.md`

## Bugs Found and Fixed (During Bug Bash)

- [x] BUG-1 (flag ordering): Fixed by migrating to cobra
- [x] BUG-2 (corrupt patches): Fixed with intent-to-add + trailing newline
- [x] BUG-3 (stale recording): Fixed with `--from` flag
- [x] BONUS (context mismatch): Fixed with 3-way merge fallback

## Acceptance Criteria

- Feature A is classified as `upstream_merged` after reconciliation
- Feature B is classified as reapplied (still needed) after reconciliation
- Target repo `bun test` passes after reconciliation
- Target repo `bun run typecheck` passes after reconciliation
- Feature B (`copilot-api models`) still works after reconciliation
- Recorded patch replays cleanly on a fresh clone
- All 6 skill formats pass parity test

## Reference

- Prior bug bash guides: `../tests/gpt/RECONCILIATION-BUG-BASH.md`, `../tests/experimental/RECONCILIATION-BUG-BASH.md`, `../tests/cc/RECONCILIATION-BUG-BASH.md`
- Prior bug bash results: `../tests/gpt/RECONCILIATION-BUG-BASH-REPORT.md`, `../tests/experimental/RECONCILIATION-BUG-BASH-REPORT.md`
- Fork repo: `https://github.com/tesseracode/copilot-api.git`
- Pinned commit: `0ea08febdd7e3e055b03dd298bf57e669500b5c1`
