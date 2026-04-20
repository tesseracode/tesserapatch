# Current Handoff

## Active Task

- **Task ID**: B1 — PRD-agent-as-provider-skills
- **Milestone**: Tranche B1 (v0.4.3 "Stand-In Agent, Part 1")
- **Status**: Complete — shipped as v0.4.3
- **Next**: Tranche B continues with B3/B4 (recipe template ops) or ADR work for B6/B7/B8

## Session Summary

Shipped v0.4.3. Surfaced the "agent-as-provider" Path B pattern (which
emerged from v0.4.2 live stress testing) as a first-class workflow in
both the CLI and the shipped skills.

### What landed

- **`--manual` / `--skip-llm` flag** on `analyze`, `define`, `explore`,
  `implement`. Advances feature state without calling the provider,
  after validating that the expected artifact exists at the canonical
  path (and is valid JSON for `implement`). Refuses otherwise, names
  the missing file. Writes an audit note to `status.json`.
- **All 6 skill formats rewritten** — Claude (canonical),
  Copilot skill, Copilot prompt, Cursor, Windsurf, Generic — now teach
  Path A (CLI) and Path B (agent-authored) as equal peers. Added:
  "You Are the Provider", `apply-recipe.json` schema with literal
  search semantics, patch-vs-recipe mental model, 3WayConflicts
  playbook (`git checkout stash@{0}^3 -- .tpatch/`, never pop).
  Parity guard: 10 → 16 anchor phrases.
- **`docs/agent-as-provider.md`** — long-form companion with worked
  recipe examples and 3WayConflicts resolution walkthrough.
- **ADR-010 `provider-conflict-resolver`** — locks shape of B8
  (v0.5.0 headline): phase 3.5 in reconcile, shadow worktree,
  per-file provider call, validation gate, `--apply`/`--accept`/
  `--reject` flags.
- **PRD `agent-as-provider-skills`** — full scope doc for this slice.

## Deferred

- `reconcile --manual` flag (PRD §7.2). Agent-driven reconcile path
  uses `apply --mode done` + `record` against new upstream; a manual
  reconcile flag was not needed for the B1 workflow. Defer until
  demand emerges.

## Files Changed

### Created
- `docs/adrs/ADR-010-provider-conflict-resolver.md`
- `docs/prds/PRD-agent-as-provider-skills.md`
- `docs/agent-as-provider.md`
- `internal/store/manual.go`

### Modified
- `internal/cli/cobra.go` — `--manual`/`--skip-llm` on 4 phases,
  version bump 0.4.2 → 0.4.3.
- `internal/cli/phase2_test.go` — 4 new tests.
- `assets/skills/claude/tessera-patch/SKILL.md` — canonical rewrite.
- `assets/skills/copilot/tessera-patch/SKILL.md` — +agent-as-provider
  block.
- `assets/prompts/copilot/tessera-patch-apply.prompt.md` — same.
- `assets/skills/cursor/tessera-patch.mdc` — same.
- `assets/skills/windsurf/windsurfrules` — same.
- `assets/workflows/tessera-patch-generic.md` — same.
- `assets/assets_test.go` — 6 new required anchors (total 16).
- `CHANGELOG.md` — v0.4.3 section.

## Test Results

- `gofmt -l .` — clean.
- `go build ./cmd/tpatch` — ok.
- `go test ./...` — all packages pass. Parity guard green on all 6
  skill formats with 16 required anchors.

## Next Steps

1. Tag and push v0.4.3.
2. User smoke-tests `--manual` flow against a real fork.
3. Next tranche candidates (pick based on feedback):
   - B3/B4: `feat-recipe-template-ops` + migration (backwards-compat).
   - ADR-006: tool-use design (gating B7 + B8).
   - B5: prompt anti-hallucination stopgap.

## Blockers

None.

## Context for Next Agent

- Parity guard (`go test ./assets/...`) must stay green; any skill
  edit that drops one of the 16 anchor phrases breaks the build.
- `internal/store/manual.go` is the single source of truth for the
  phase → artifact → state → last_command mapping. Skills and docs
  mirror this table; if it changes, update in lockstep.
- `reconcile --manual` deliberately unimplemented; see Deferred above.
- Skills are embedded via `go:embed`; users must re-run
  `tpatch init --harness <foo>` (or re-download the binary and
  reinstall skills) to get the new content in their harness.
