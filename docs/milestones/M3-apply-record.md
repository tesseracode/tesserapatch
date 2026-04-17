# M3 — Apply & Record

**Status**: ⬜ Not started  
**Depends on**: M2

## Tasks

- [ ] M3.1 — Implement `apply-recipe.json` format — `write-file`, `append-file`, `replace-in-file`, `ensure-directory` operations
- [ ] M3.2 — Implement recipe validation — operation type checks, path safety via `ensureSafeRepoPath()`
- [ ] M3.3 — Implement `tpatch implement <slug>` — call provider to generate apply recipe, save to `artifacts/apply-recipe.json`
- [ ] M3.4 — Implement `tpatch apply <slug> --mode prepare` — generate agent packet, pre-apply context
- [ ] M3.5 — Implement `tpatch apply <slug> --mode started` — record session start, operator notes
- [ ] M3.6 — Implement `tpatch apply <slug> --mode done` — record post-apply state, validation status, conflicts
- [ ] M3.7 — Implement recipe execution — apply `apply-recipe.json` operations with path traversal protection
- [ ] M3.8 — Implement `tpatch record <slug>` — capture git diff (tracked + untracked files), excluding `.tpatch/` and skill artifacts
- [ ] M3.9 — Implement untracked file capture in patches (port from GPT's `CapturePatch()`)
- [ ] M3.10 — Write tests for recipe parsing, execution, patch capture

## Acceptance Criteria

- `tpatch implement` produces a valid `apply-recipe.json` with reviewed operations
- `tpatch apply --mode prepare/started/done` creates proper session artifacts
- `tpatch record` captures a complete patch including new files
- Patch excludes `.tpatch/`, `.claude/skills/`, `.github/skills/`, `.github/prompts/`, `.cursor/rules/`
- Path traversal attempts in recipes are rejected
- All operations are logged in `apply-session.json`

## Reference

- Port apply recipe format from `../experimental/src/core/workflow.ts` (`applyFeature`)
- Port patch capture from `../gpt/internal/gitutil/git.go` (`CapturePatch`)
- Port apply session lifecycle from `../gpt/internal/tpatch/store.go` (`RecordApplyCompletion`)
