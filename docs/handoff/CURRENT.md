# Current Handoff

## Active Task

- **Task ID**: B1 — PRD-agent-as-provider-skills, in progress
- **Milestone**: v0.4.3 (skills + `--manual` flag), shipped ahead of B2 (conflict resolver, v0.5.0).
- **Status**: Step 1 of 5 complete (`--manual` flag + tests). Step 2 (skills rewrite) is the next chunk.
- **Related**: `docs/adrs/ADR-010-provider-conflict-resolver.md`, `docs/prds/PRD-agent-as-provider-skills.md`

## Session Summary

Post-v0.4.2 live stress test on a real fork (t3code) produced strong signal that the biggest unblocker right now is not a CLI behaviour — it is the **skills gap**: shipped skills do not teach coding agents what to do when the provider underperforms. Provider quote: *"the biggest gap in the skills isn't a CLI behavior — it's that the skill doesn't teach agents what to do when the LLM provider fails or is not available."*

Tranche B is now split into two workstreams:

- **B1** (v0.4.3, cheap+high-impact): rewrite skills with Path A/Path B symmetry, inline `apply-recipe.json` schema, 3WayConflicts playbook, patch-vs-recipe mental model, add `--manual`/`--skip-llm` flag to every phase command.
- **B2** (v0.5.0 headline): provider-assisted conflict resolver, architectural shape locked in ADR-010.

This session delivered:

1. **ADR-010** — shape of the conflict resolver (phase 3.5, shadow worktree, per-file provider calls, validation gates, accept/reject UX, DERIVED artifact refresh, no heuristic fallback).
2. **PRD-agent-as-provider-skills** — full scope of B1 including the `--manual` flag contract.
3. **`--manual` / `--skip-llm` flag implementation** — step 1 of the PRD's §10.

## Files Changed (this B1 step)

New files:
- `docs/adrs/ADR-010-provider-conflict-resolver.md`
- `docs/prds/PRD-agent-as-provider-skills.md`
- `internal/store/manual.go` — `ManualPhase(phase)` map + `Store.AdvanceStateManually(slug, phase)`.

Modified:
- `internal/cli/cobra.go`:
  - `analyzeCmd / defineCmd / exploreCmd / implementCmd` — early-return branch when `isManualFlag(cmd)` is true.
  - New helpers at end of file: `addManualFlag`, `isManualFlag`, `runManualPhase`.
- `internal/cli/phase2_test.go`:
  - `TestManualAnalyzeAdvancesState`
  - `TestManualRefusesMissingArtifact`
  - `TestManualImplementValidatesJSON` (covers JSON validation gate + success path)
  - `TestManualSkipLLMAlias`

## Test Results

```
$ gofmt -l .
(clean)

$ go build ./...
(clean)

$ go test ./...
ok  .../assets              (cached)
ok  .../internal/cli        1.899s
ok  .../internal/gitutil    1.070s
ok  .../internal/provider   (cached)
ok  .../internal/safety     (cached)
ok  .../internal/store      0.506s
ok  .../internal/workflow   1.460s
```

Four new tests added, all pass. No regressions.

## Next Steps (rest of PRD §10)

2. **Skills rewrite** — draft Claude SKILL.md as canonical. Sections required (§5.1–5.5 of PRD):
   - "You are the provider" framing at top
   - Per-phase Path A / Path B symmetric layout for analyze/define/explore/implement/reconcile
   - Inline `apply-recipe.json` schema (4 ops, literal-search semantics, path safety)
   - 3WayConflicts playbook (`git checkout stash@{0}^3 -- .tpatch/`, never pop stash)
   - Patch-vs-recipe mental model
   Then propagate adapted versions to: Copilot SKILL.md, Copilot prompt file, Cursor `.mdc`, Windsurf rules, Generic workflow.
3. **Parity guard** — add 6 new anchors in `assets/assets_test.go`:
   - `provider-fallback/you-are-the-provider`
   - `recipe-schema/ops-table`
   - `recipe-schema/literal-search`
   - `conflict-playbook/checkout-stash`
   - `conflict-playbook/never-pop`
   - `patch-vs-recipe/intent-vs-snapshot`
4. **`docs/agent-as-provider.md`** — longer-form companion. Skills link to it. Exhaustive recipe examples + 3WayConflicts playbook with sample conflict output.
5. **Release**: bump `version` to `0.4.3`. CHANGELOG entry. Single commit with co-author trailer. Tag `v0.4.3`. Push.

## Blockers

None.

## Context for Next Agent

- PRD-agent-as-provider-skills is the source of truth for what the skills must contain. Read it first; everything else is subordinate.
- ADR-010 is already accepted — when writing the 3WayConflicts playbook, reference it by name so readers can follow the trail from "here is what you do manually today" to "here is what v0.5.0 will automate".
- `--manual` flag contract is already in code (`internal/store/manual.go`). The phase→artifact→state map is the source of truth — skills MUST NOT invent different paths:
  - analyze → `analysis.md` → StateAnalyzed
  - define → `spec.md` → StateDefined
  - explore → `exploration.md` → StateDefined
  - implement → `artifacts/apply-recipe.json` → StateImplementing (JSON-validated)
- Parity guard (`assets/assets_test.go` `requiredAnchors`) currently has 10 entries. The PRD adds 6 more, bringing total to 16. `strings.Contains` check across all six files.
- `reconcile --manual` is DEFERRED (PRD §7.2). Accept the flag but print a pointer to the conflict resolver docs — the manual reconcile path uses `apply` + `record` against a new upstream, not `reconcile`.
- The ADR-010 document outlines the long-term automation. Skills should teach the manual playbook NOW so users can unblock themselves without waiting for B2.
- Version bump + tag + push happens only AFTER all skills land + parity guard passes + docs land.
