# Tessera Patch — GitHub Copilot Skill

## Overview

Tessera Patch customizes open-source projects through natural-language patches while maintaining upstream compatibility.

## Invocation

`tpatch` is a compiled Go binary on PATH. Invoke it directly — do NOT wrap it:

- ✓ `tpatch <command>`
- ✗ `npx tpatch …` (not a Node package)
- ✗ `npm run tpatch …` (not an npm script)
- ✗ `python -m tpatch …` (not a Python module)

Always run from the repository root (where `.tpatch/` exists). Do not `cd` to speculative paths — use the current working directory.

## Phase Ordering

```
requested    → tpatch analyze    → analyzed
analyzed     → tpatch define     → defined
defined      → tpatch explore    → defined (exploration.md enriched)
defined      → tpatch implement  → implementing (apply-recipe.json ready)
implementing → tpatch apply --mode execute                          → applied
             OR tpatch apply --mode started / edit / --mode done    → applied
applied      → tpatch record     → active
active       → tpatch reconcile  → active | upstream_merged | blocked
```

Never skip a phase. Never go backwards without `tpatch reconcile`.

## Before You Run Anything

1. `tpatch status <slug>` — see current state and last command.
2. `tpatch next <slug>` — get the exact next command (add `--format harness-json` for structured output).
3. Only then proceed. Do not guess the next phase from file presence.
4. Run tpatch record <slug> BEFORE git commit. If you already committed, use tpatch record <slug> --from <base> — a clean working tree without --from is refused.
5. Run tpatch reconcile only on a CLEAN working tree at the target upstream state. Commit or stash first; reconcile refuses dirty trees, conflict markers, and .orig/.rej leftovers. See docs/reconcile.md for the workflow patterns.

## CLI Commands

- `tpatch init` — Initialize workspace
- `tpatch add <description>` — Create feature request
- `tpatch status` — Feature dashboard
- `tpatch analyze <slug>` — Run analysis
- `tpatch define <slug>` — Generate acceptance criteria
- `tpatch explore <slug>` — Explore codebase
- `tpatch implement <slug>` — Generate apply recipe
- `tpatch apply <slug>` — Execute recipe or record session
- `tpatch record <slug>` — Capture patches
- `tpatch reconcile` — Reconcile with upstream
- `tpatch provider check` — Validate provider
- `tpatch config show|set` — Manage config
- `tpatch cycle <slug>` — Run analyze→define→explore→implement→apply→record in sequence (batch or `--interactive`)
- `tpatch test <slug>` — Run the configured `test_command` and record the outcome
- `tpatch next <slug>` — Emit the next action for a feature (`--format harness-json` for structured consumption)

## Lifecycle

analyse → define → explore → implement → test → record → reconcile

## Data Model

Features are tracked in `.tpatch/features/<slug>/` with `status.json`, `request.md`, `analysis.md`, `spec.md`, and `artifacts/`.

## Reconciliation

4-phase decision tree: reverse-apply → operation-level → provider-semantic → forward-apply.

Outcomes: `upstream_merged`, `reapplied`, `blocked`.
