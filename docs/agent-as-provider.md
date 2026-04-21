# Agent as Provider — Path B Workflow

> Companion to the shipped skill files. Read this once, then rely on
> the per-format skills for day-to-day invocation.

Tessera Patch has two equally-supported execution paths for every LLM
phase (`analyze`, `define`, `explore`, `implement`):

- **Path A — CLI-driven.** The configured provider generates the
  artifact: `tpatch <phase> <slug>`.
- **Path B — Agent-authored.** You (the agent) author the artifact by
  hand under `.tpatch/features/<slug>/`, then advance state with
  `tpatch <phase> <slug> --manual`.

Path B is **normal, not exceptional**. Prefer it whenever:

1. No provider is configured in this repository or in global config.
2. The configured provider returns empty / truncated / insufficient
   output. In practice this happens most often with `implement` — the
   LLM returns a 1-operation stub, an `ensure-directory`-only recipe,
   or truncated JSON.
3. You have strictly more context than the provider (you are mid-task,
   have already read the relevant files, and know what needs to
   change).
4. The provider output would disagree with intent you have already
   captured in `spec.md` / `exploration.md`.

Do not wait for a "better" recipe. Author the artifact and move on.

## The phase → artifact → state contract

The `--manual` flag validates this table. If the expected artifact is
missing at the canonical path (or, for `implement`, is not valid JSON),
the command refuses with a diagnostic pointing at the exact file; state
does **not** advance.

| phase       | artifact                                    | advances state to |
|-------------|---------------------------------------------|-------------------|
| `analyze`   | `.tpatch/features/<slug>/analysis.md`       | `analyzed`        |
| `define`    | `.tpatch/features/<slug>/spec.md`           | `defined`         |
| `explore`   | `.tpatch/features/<slug>/exploration.md`    | `defined`         |
| `implement` | `.tpatch/features/<slug>/artifacts/apply-recipe.json` | `implementing` |

On success, `status.json.notes` records:

```
Phase advanced manually (--manual); artifact authored at <path>
```

so the audit trail distinguishes Path B transitions from provider
output.

## apply-recipe.json schema (authoritative)

The recipe is the deterministic script that `tpatch apply --mode execute`
replays against the current upstream snapshot.

```json
{
  "version": 1,
  "operations": [
    { "type": "ensure-directory", "path": "src/feature/" },
    { "type": "write-file",
      "path": "src/feature/index.ts",
      "content": "export const x = 1;\n" },
    { "type": "replace-in-file",
      "path": "src/registry.ts",
      "search": "export * from \"./legacy\";\n",
      "replace": "export * from \"./legacy\";\nexport * from \"./feature\";\n" },
    { "type": "append-file",
      "path": "src/changelog.md",
      "content": "\n- added feature/\n" }
  ]
}
```

Operation semantics:

- `ensure-directory` — `mkdir -p`. Idempotent.
- `write-file` — creates or overwrites the whole file. Use for new
  files or full rewrites.
- `replace-in-file` — locates the first occurrence of `search` and
  substitutes `replace`. Errors if `search` is absent.
  - `search` is a **literal string match, not a regex**. Paste the
    exact text, including leading/trailing whitespace. Escape quotes
    and backslashes per JSON rules, not regex rules.
  - Include surrounding lines for uniqueness — one-line anchors
    collide.
  - Exactly one occurrence is replaced per op. To replace several
    copies, emit several ops; each targets the next occurrence as the
    prior op has already rewritten the file.
- `append-file` — appends `content` to an existing file. Errors if
  the file is missing.

There is no `delete-file` or `rename-file` op in the current schema.
To delete or rename a file, use Path B: `apply --mode started`,
`git rm <path>` (or `git mv`), `apply --mode done`, `record`. Richer
op support is tracked in `feat-recipe-schema-expansion`.

Path safety:

- All `path` values are repo-relative.
- `../` traversal, absolute paths, and symlinks that escape the repo
  abort `apply --mode execute` via `EnsureSafeRepoPath`. This is
  enforced per-operation, not just on the recipe as a whole.

Ordering:

- Operations execute sequentially. Later operations can assume earlier
  operations succeeded (e.g. `ensure-directory` before `write-file`
  into that directory, or `write-file` before `replace-in-file` on the
  same file).

## Patch vs recipe — mental model

`.tpatch/features/<slug>/artifacts/` contains two representations of
the same change:

| file | role |
|------|------|
| `post-apply.patch` | authoritative `git diff`. **The patch captures intent.** |
| `apply-recipe.json` | deterministic script targeting a specific upstream snapshot. |

When they disagree — e.g. a `replace-in-file` anchor is no longer
present because upstream edited the surrounding lines — **trust the
patch**, regenerate the recipe. The patch is what `tpatch reconcile`
evaluates against new upstream, what `tpatch record` writes on every
capture, and what survives a feature being rebuilt from scratch.

The recipe is a performance optimisation: it lets `apply --mode
execute` run without invoking a provider on a clean snapshot. It is
not the source of truth.

## The 3WayConflicts playbook

When `tpatch reconcile` returns `3WayConflicts` for a feature, the
phase-4 three-way merge detected textual conflicts between your
feature's diff and the new upstream. The stash created by reconcile
holds your **pre-reconcile** tree (index, working tree, and — crucially
— the `.tpatch/` metadata). Do not pop it.

1. **Never pop the stash.** If you `git stash pop`, you roll the
   repository back to pre-reconcile state and lose the new upstream
   you were trying to reconcile against.
2. Restore only the tpatch metadata so you can see the feature's
   recorded intent without disturbing the working tree:

   ```
   git checkout stash@{0}^3 -- .tpatch/
   ```

   The `^3` parent of a stash commit is the staged-files tree; stash
   created by tpatch includes `.tpatch/` there specifically for this
   flow.
3. Read the feature's intent:
   - `.tpatch/features/<slug>/spec.md` — what the feature is supposed
     to do.
   - `.tpatch/features/<slug>/exploration.md` — which files and
     symbols are load-bearing.
   - `.tpatch/features/<slug>/artifacts/post-apply.patch` — the exact
     diff applied last time.
4. Read the **new** upstream version of each conflicted file (they
   are in the working tree now).
5. Hand-author a resolution that preserves **both** intents — yours
   and the upstream change that caused the conflict. Do not blindly
   prefer one side; reconcile exists to merge.
6. Finish the feature via Path B:

   ```
   tpatch apply <slug> --mode done
   tpatch record <slug>
   ```

   This captures a fresh `post-apply.patch` against the new upstream
   and writes a new `apply-recipe.json`.

### Worked example

Given conflict on `src/registry.ts`:

- Feature `add-telemetry` added `export * from "./telemetry";` after
  `export * from "./legacy";`.
- Upstream renamed `./legacy` to `./deprecated`.

Resolution: edit the new upstream file to add
`export * from "./telemetry";` after the renamed
`export * from "./deprecated";` line. Both intents preserved.

## Provider-assisted automation (coming)

ADR-010 (`docs/adrs/ADR-010-provider-conflict-resolver.md`) locks the
design for v0.5.0's headline: automating the 3WayConflicts playbook
via the configured provider. Phase 3.5 in reconcile will run per-file
provider calls with `spec.md` + `exploration.md` + the conflicted file
as context, validate the proposed resolution, optionally run repo
tests, and surface a report with `--apply` / `--accept` / `--reject`.

Until then, the playbook above is the supported path.
