# M5 — Skill System

**Status**: ⬜ Not started  
**Depends on**: M1

## Tasks

- [ ] M5.1 — Port CC's SKILL.md content (939 lines with decision trees, editable sections, lifecycle walkthroughs)
- [ ] M5.2 — Create Copilot skill format (`.github/skills/tessera-patch/SKILL.md`)
- [ ] M5.3 — Create Copilot prompt companion (`.github/prompts/tessera-patch-apply.prompt.md`)
- [ ] M5.4 — Create Cursor rules format (`.cursor/rules/tessera-patch.mdc`)
- [ ] M5.5 — Create Windsurf rules format (`.windsurfrules`)
- [ ] M5.6 — Create generic markdown workflow (`.tpatch/workflows/tessera-patch-generic.md`)
- [ ] M5.7 — Create `assets/embed.go` with `go:embed` directives for all 6 formats
- [ ] M5.8 — Update `tpatch init` to install all 6 skill formats into target repos
- [ ] M5.9 — Create `assets_test.go` parity guard — verify all 6 formats mention current CLI commands
- [ ] M5.10 — Ensure all skill formats document the unified CLI contract and 4-phase reconciliation

## Acceptance Criteria

- `tpatch init` installs all 6 skill formats into the target repo
- All formats teach the same methodology (7-phase lifecycle, `.tpatch/` structure, feature tracking)
- `assets_test.go` passes — all formats mention `init`, `add`, `analyze`, `define`, `implement`, `apply`, `record`, `reconcile`, `status`
- Formats are adapted to each harness's native convention (frontmatter for Claude, instructions for Copilot, rules for Cursor, etc.)

## Reference

- Port SKILL.md content from `../cc/src/skill/SKILL.md` (most comprehensive)
- Port adapter formats from `../cc/src/skill/adapters/` (Copilot, Cursor, Windsurf, Generic)
- Port embed system from `../gpt/assets/embed.go`
- Port parity test from `../gpt/assets/assets_test.go`
- Port Copilot prompt from `../gpt/assets/prompts/copilot/tessera-patch-apply.prompt.md`
