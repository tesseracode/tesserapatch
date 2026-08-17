# Path B Operator Guide

Path B means **agent-authored tpatch artifacts**: the human or coding
agent writes `analysis.md`, `spec.md`, `exploration.md`, and (when
needed) `apply-recipe.json`, then advances phases with `--manual`.

This guide captures field-tested defaults from the TWS and copilot-api
proxy case studies. It is operator guidance, not a new product contract.
The authoritative phase/artifact table remains
[`docs/agent-as-provider.md`](./agent-as-provider.md), and the
authoritative record/land contracts remain [`docs/record.md`](./record.md)
and [`docs/land.md`](./land.md).

## Mental model

tpatch is strongest as **structured patch management for forks and
feature streams**, not as a general code generator.

The value of Path B is:

- every customization gets a stable slug and reason;
- the feature gets a small acceptance/spec surface;
- the patch is recorded as canonical replay input;
- future reconcile has enough intent to decide whether a change is
  upstreamed, still needed, or blocked;
- the agent's codebase knowledge is preserved in `.tpatch/` artifacts
  instead of only in chat.

`--manual` is not a failure mode. Treat it as:

> "I am the provider for this phase."

The CLI still validates that the expected artifact exists and advances
state consistently; you are simply authoring the content yourself.

## Provider triage

Default to Path B when the agent already has codebase context.

Use the configured provider when it is likely to add value:

1. **Try provider `analyze` once** for a new or unfamiliar request. It is
   the cheapest phase and can provide a baseline.
2. If `analyze` returns heuristic boilerplate, empty output, or obviously
   shallow output, **switch to Path B immediately**. Do not burn time
   retrying weak provider phases.
3. Use provider `define` and `explore` only when provider `analyze`
   produced useful feature-specific signal.
4. For non-trivial features, prefer Path B for `implement`. A cold
   provider rarely has enough full-codebase context to generate a
   reliable recipe. Use provider `implement` mainly for smoke-testing
   provider quality, trivial features, or controlled experiments.

Field reports vary by model and repository. Do not encode a permanent
"good model / bad model" list into process docs. Judge the actual
artifact quality in the current repo and switch to Path B quickly when
the provider is not helping.

## Preferred flow: Path B before the Git commit

Use this when the feature is not committed yet.

```bash
tpatch add --slug <clean-slug> "<short description>"

# Author these files by hand:
#   .tpatch/features/<slug>/analysis.md
#   .tpatch/features/<slug>/spec.md
#   .tpatch/features/<slug>/exploration.md

# Optional read-only bundle check; it never advances state.
tpatch prepare <slug> --check

tpatch analyze <slug> --manual
tpatch define <slug> --manual
tpatch explore <slug> --manual

# Mark work as underway, then implement in the working tree.
tpatch apply <slug> --mode started

# edit files, run tests

# Preferred composed commit path:
tpatch land <slug>
```

`tpatch prepare <slug> --check` exits 2 when the intent bundle is incomplete. That is a report result, not a workflow or system failure: the command wrote nothing, changed nothing, and the per-artifact rows say exactly what is missing. Author the missing files and re-run, or continue without it — this check is optional.

`tpatch land` runs the record step, stages the feature path set plus
feature metadata, creates one Git commit, and writes the `Tpatch-Feature`
trailer block. Prefer it when you would otherwise run:

```text
tpatch record <slug> -> git add <paths> -> git commit
```

## Retroactive flow: work already committed

Use this when implementation already landed in Git before the feature
was registered in tpatch.

```bash
tpatch add --slug <clean-slug> "<short description>"

# Author minimal artifacts:
#   analysis.md      -> gap, risk, affected files
#   spec.md          -> testable acceptance criteria
#   exploration.md   -> files changed and why

tpatch analyze <slug> --manual
tpatch define <slug> --manual
tpatch explore <slug> --manual

mkdir -p .tpatch/features/<slug>/artifacts
printf '%s\n' '{"version":1,"operations":[]}' \
  > .tpatch/features/<slug>/artifacts/apply-recipe.json
tpatch implement <slug> --manual

# Pick the narrowest correct base/range.
tpatch record <slug> --auto --files <feature-paths> --regenerate-recipe
# or:
tpatch record <slug> --from <feature-base> --to <feature-tip> --files <feature-paths> --regenerate-recipe

git add .tpatch/features/<slug> .tpatch/FEATURES.md
git commit -m "chore(tpatch): record <slug>"
```

This flow is intentionally more manual than `land`: it is recovery for a
feature that already has a production commit. The important thing is to
record immediately, before another feature's commits make the range
ambiguous.

## Small-feature fast track

For bug fixes, one-file changes, and small refactors, the artifacts can
be short:

- `analysis.md`: what gap exists, risk level, affected files;
- `spec.md`: testable acceptance criteria;
- `exploration.md`: changed files and why.

One paragraph each is often enough when the recorded patch is the
source of truth. Do not skip the artifacts; the small amount of
structure is what makes later reconcile understandable.

## Slugs

Prefer explicit slugs:

```bash
tpatch add --slug fix-model-cache "Fix stale model cache after refresh"
```

Auto-generated slugs are valid, but field use shows they are often long,
awkward, and hard to reference in follow-up commands, commits, and
handoffs. Stable human-readable slugs make every later artifact easier
to review.

## Commit and record discipline

The goal is **one feature boundary per recorded patch**.

Recommended:

- use `tpatch land <slug>` for the normal record + commit flow;
- if code was already committed, record it immediately with
  `--auto` / `--from` / `--to` and `--files`;
- keep production commits feature-scoped where practical;
- commit `.tpatch/` metadata promptly so the feature state travels with
  the branch.

Avoid:

- batching multiple features into one commit and one record range;
- recording from a broad upstream base when only one feature should be
  captured;
- ignoring cross-feature collision warnings.

If you intentionally accept a byte-identical cross-feature patch, use:

```bash
tpatch record <slug> --allow-collision "<clear reason>"
```

The reason is persisted in `record.md`. Treat this as an exception, not
normal workflow.

## Dependency timing

Run dependency validation after exploration and before implementation:

```bash
tpatch feature deps --validate-all
```

Exploration often reveals parent/child relationships that were not
obvious at `add` time. If the feature edits files or symbols created by
another feature, declare the parent before implementation or recording.

Use:

```bash
tpatch feature deps <child> add <parent>:hard
tpatch feature deps <child> add <parent>:soft
```

Prefer `hard` only when the child cannot meaningfully exist without the
parent. Use `soft` for ordering and review hints.

## Current gotchas (corrected against modern tpatch)

Some older field notes are now out of date:

- `--from + --files` **is supported**. Use it to scope committed-range
  captures.
- `--auto + --files` is supported and should be the first recovery
  choice when the branch tracks upstream.
- `record --regenerate-recipe` can regenerate a recipe from the
  captured patch when the placeholder recipe is stale.
- `tpatch land` exists and should usually replace manual
  `record -> git add -> git commit` when the code is still in the
  working tree.

Still true:

- `implement --manual` requires a valid `apply-recipe.json`.
- The phase order is real; use `--manual` to advance through it rather
  than skipping state.
- Collision detection is there to prevent accidental broad captures.

## What not to use tpatch for

tpatch does not replace:

- a test runner;
- a CI system;
- a scheduler;
- an agent task tracker;
- Git review.

It records feature intent, patch bytes, dependency context, and
reconcile evidence so those other systems have something stable to
reason over.

## Open follow-up: task tracking for tpatch development

Field use suggests tpatch's own development could benefit from a small
task-tracking layer. Do not overload Path B guidance with that problem.
If pursued, it should be a separate paper/PRD evaluating whether tpatch
should store implementation tasks under `.tpatch/` for projects that use
tpatch as a development workflow.
