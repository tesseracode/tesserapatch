# M1 — Core Store & Init

**Status**: ✅ Complete
**Depends on**: M0

## Tasks

- [x] M1.1 — Implement `.tpatch/` data model types (`status.json`, `config.yaml`, `upstream.lock`, feature folder structure)
- [x] M1.2 — Implement store layer (`Init()`, `Open()`, `AddFeature()`, `LoadFeature()`, `SaveStatus()`, `ListFeatures()`)
- [x] M1.3 — Implement `tpatch init` — create `.tpatch/` scaffold, `config.yaml`, `FEATURES.md`, `upstream.lock`, `steering/` directory
- [x] M1.4 — Implement `tpatch add` — create feature folder, `request.md`, `status.json`, update `FEATURES.md`
- [x] M1.5 — Implement `tpatch status` — list features, show states, support `--feature`, `--json`, `--verbose`
- [x] M1.6 — Implement `tpatch config show|set` — read/write `config.yaml` values
- [x] M1.7 — Implement slug generation with tests (kebab-case, dedup, length limits)
- [x] M1.8 — Implement `ensureSafeRepoPath()` in `internal/safety/` with tests
- [x] M1.9 — Write unit tests for store operations (init, add, list, status)

## Acceptance Criteria

- `tpatch init --path /tmp/test` creates the full `.tpatch/` scaffold
- `tpatch add "Change button color" --path /tmp/test` creates feature folder with `request.md` and `status.json`
- `tpatch status --path /tmp/test` lists features with correct states
- `tpatch config show --path /tmp/test` displays config
- `ensureSafeRepoPath()` rejects path traversal attempts (unit tested)
- All operations are deterministic and produce human-readable files

## Reference

- Port store logic from `../gpt/internal/tpatch/store.go`
- Port path safety from `../experimental/src/core/workflow.ts` (`ensureSafeRepoPath`)
- Use CC's `.tpatch/` structure enrichments: `steering/`, `upstream.lock`
- Use YAML for config (not JSON) — more human-readable
