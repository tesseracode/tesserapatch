---
mode: agent
description: Apply Tessera Patch features to this repository
tools: terminal, editFiles, readFile
---

# Tessera Patch — Apply Feature

You are applying a Tessera Patch feature to this repository. Follow these steps:

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

## Steps

1. Check feature status: `tpatch status --feature {{slug}}`
2. Read the spec: `.tpatch/features/{{slug}}/spec.md`
3. Read the exploration: `.tpatch/features/{{slug}}/exploration.md`
4. Mark as started: `tpatch apply {{slug}} --mode started`
5. Make the code changes described in the spec
6. Run tests to verify acceptance criteria
7. Record completion: `tpatch apply {{slug}} --mode done`
8. Capture patch: `tpatch record {{slug}}`

## Safety Rules

- Do NOT modify files outside the repository
- Do NOT store secrets in tracked files
- Run the project's test suite before marking done
- Review changes before committing

## Available Commands

`tpatch init`, `tpatch add`, `tpatch status`, `tpatch analyze`, `tpatch define`, `tpatch explore`, `tpatch implement`, `tpatch apply`, `tpatch record`, `tpatch reconcile`, `tpatch provider check`, `tpatch config show|set`, `tpatch cycle`, `tpatch test`, `tpatch next`
