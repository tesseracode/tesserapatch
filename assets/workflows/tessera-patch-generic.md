# Tessera Patch — Generic Workflow

## Overview

Tessera Patch customizes forked open-source projects using natural-language patches while maintaining upstream compatibility.

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

## 7-Phase Lifecycle

1. **Analyse** (`tpatch analyze <slug>`) — Assess compatibility and impact
2. **Define** (`tpatch define <slug>`) — Generate acceptance criteria and plan
3. **Explore** (`tpatch explore <slug>`) — Identify affected files and minimal changeset
4. **Implement** (`tpatch implement <slug>`) — Generate apply recipe, make changes
5. **Test** — Run project tests, verify acceptance criteria
6. **Record** (`tpatch record <slug>`) — Capture patches (tracked + untracked)
7. **Reconcile** (`tpatch reconcile`) — Re-evaluate when upstream updates

## Quick Start

```bash
tpatch init --path /path/to/fork
tpatch add "Your feature description"
tpatch analyze <slug>
tpatch define <slug>
tpatch explore <slug>
# Make changes...
tpatch apply <slug> --mode done
tpatch record <slug>
tpatch status
```

## CLI Commands

- `tpatch init` — Initialize workspace
- `tpatch add` — Create feature request
- `tpatch status` — Feature dashboard
- `tpatch analyze` — Run analysis
- `tpatch define` — Generate spec
- `tpatch explore` — Explore codebase
- `tpatch implement` — Generate recipe
- `tpatch apply` — Execute/record session
- `tpatch record` — Capture patches
- `tpatch reconcile` — Reconcile with upstream
- `tpatch provider check` — Validate provider
- `tpatch config show|set` — Manage config
- `tpatch cycle <slug>` — Full lifecycle in one command (add `--interactive` for prompts)
- `tpatch test <slug>` — Run the configured `test_command` and record the result
- `tpatch next <slug>` — Emit the next action (use `--format harness-json` for structured output)

## Reconciliation

When upstream releases a new version, run `tpatch reconcile`. The 4-phase decision tree determines each feature's fate:

1. **Reverse-apply** — Is the patch already in upstream? → UPSTREAMED
2. **Operation-level** — Check individual recipe operations → partial detection
3. **Provider-assisted** — LLM semantic check → structural differences
4. **Forward-apply** — Can the patch be re-applied? → REAPPLIED or BLOCKED
