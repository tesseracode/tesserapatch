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

## You Are the Provider

Every LLM phase has two paths:

- **Path A — CLI-driven**: `tpatch <phase> <slug>` — configured provider generates the artifact.
- **Path B — Agent-authored**: author the artifact yourself under `.tpatch/features/<slug>/`, then `tpatch <phase> <slug> --manual` to advance feature state without calling the provider.

You are the provider when no provider is configured, the provider returns empty/truncated/insufficient output (common with implement — 1-op stubs, ensure-directory-only, truncated JSON), or you have more context than it does. Path B is normal, not exceptional — do not wait for a better recipe.

Phase → artifact → state contract (the `--manual` flag validates this):

| phase | artifact | advances state to |
|---|---|---|
| analyze | `analysis.md` | `analyzed` |
| define | `spec.md` | `defined` |
| explore | `exploration.md` | `defined` |
| implement | `artifacts/apply-recipe.json` (JSON-validated) | `implementing` |

## apply-recipe.json schema

```json
{
  "version": 1,
  "operations": [
    { "type": "ensure-directory", "path": "src/feature/" },
    { "type": "write-file", "path": "src/a.ts", "content": "export const x = 1;\n" },
    { "type": "replace-in-file", "path": "src/b.ts",
      "search": "export * from \"./legacy\";\n",
      "replace": "export * from \"./legacy\";\nexport * from \"./feature/a\";\n" },
    { "type": "append-file", "path": "src/changelog.md",
      "content": "\n- added feature/a\n" }
  ]
}
```

Semantics:

- Ops: `ensure-directory`, `write-file { path, content }`, `replace-in-file { path, search, replace }`, `append-file { path, content }`. No `delete-file` / `rename-file` yet — use Path B + `git rm` for deletes.
- `replace-in-file.search` is a **literal string match, not a regex**. Paste the exact text, include surrounding lines for uniqueness.
- `replace-in-file` replaces exactly one occurrence per op. Emit multiple ops to replace several copies.
- All `path` values are repo-relative. `../`, absolute paths, or symlinks outside the repo abort `apply --mode execute` (`EnsureSafeRepoPath`).
- Operations execute in order; later ops may depend on earlier ops.

## Patch vs recipe — mental model

- `artifacts/post-apply.patch` — authoritative git diff. **The patch captures intent.**
- `artifacts/apply-recipe.json` — deterministic script targeting a specific upstream snapshot.

When they disagree (e.g. the recipe's `replace-in-file` can no longer find its anchor because upstream edited the line), trust the patch. Regenerate the recipe afterward.

## If reconcile returns 3WayConflicts

1. **Never pop the stash.** It holds your pre-reconcile tree.
2. Restore only the tpatch metadata so you can see the feature's intent:
   `git checkout stash@{0}^3 -- .tpatch/`
3. Read `.tpatch/features/<slug>/spec.md` (intent), `.tpatch/features/<slug>/artifacts/post-apply.patch` (diff), and the new upstream version of each conflicted file.
4. Hand-author a resolution that preserves **both** intents.
5. `tpatch apply <slug> --mode done && tpatch record <slug>`.

Provider-assisted automation of this playbook is tracked in `docs/adrs/ADR-010-provider-conflict-resolver.md` (v0.5.0 headline).
