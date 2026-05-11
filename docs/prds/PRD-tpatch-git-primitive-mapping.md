# PRD — Mapping tpatch Commands to Git Primitives and Extra Semantics

**Status**: Exploratory note
**Date**: 2026-04-27
**Owner**: Core
**Milestone**: Side exploration; not yet roadmap-committed
**Related**: [SPEC.md](../../SPEC.md), [docs/record.md](../record.md), [docs/reconcile.md](../reconcile.md), [docs/feature-layout.md](../feature-layout.md), [docs/prds/PRD-intent-version-control-evaluation.md](PRD-intent-version-control-evaluation.md)

## Summary

This note maps the current `tpatch` command surface to the closest Git primitives and then identifies the extra semantics `tpatch` adds on top. The goal is to separate two questions that often get mixed together:

1. What can Git already do mechanically?
2. What value does tpatch add that is not just a verbose commit message convention?

This mapping is a prerequisite for any future attempt to evolve tpatch into a stronger Git-backed intent system. If we cannot state clearly what Git already covers and where tpatch genuinely extends it, any future "superset of version control" proposal will drift into either reinvention or hand-waving.

## Why this note exists

The project already behaves like a layered system:

- Git stores content history and supports transport, merge, and branch movement.
- tpatch stores feature intent, replay instructions, status, and semantic reconciliation.

Today those layers are coupled operationally but separated in user workflow. The same user may:

- use `tpatch implement` and `tpatch record` to create structured feature artifacts,
- then use `git add`, `git commit`, `git rebase`, and `git push` to move actual repo history.

That separation is workable, but it leaves an important question unanswered: which parts of today's tpatch model are just wrapper UX over Git, and which parts represent genuinely richer change semantics?

## Working thesis

Git gives the project durable storage for content and ancestry. tpatch adds three classes of semantics Git does not natively provide:

1. **Intent semantics** — what problem the change is solving and what acceptance criteria define success.
2. **Replay semantics** — how the change can be re-applied deterministically or reconstructed.
3. **Maintenance semantics** — how the change relates to upstream drift, dependencies, and semantic reconciliation.

The mapping below tests that thesis command by command.

## Mapping: current tpatch commands vs Git primitives

| tpatch command | Closest Git primitive | What Git covers | What tpatch adds |
|---|---|---|---|
| `tpatch init` | `git init` plus repo bootstrap | Creates repo storage and config baseline | Creates `.tpatch/` workspace, installs skill surfaces, scaffolds steering and feature metadata model |
| `tpatch add <description>` | none exact; closest is creating a branch or empty commit draft | Git can create a branch or WIP commit, but not a structured feature request | Creates a stable slug and first-class feature identity before code exists |
| `tpatch analyze <slug>` | none exact | Git has no analysis phase | Stores compatibility/risk analysis tied to the feature |
| `tpatch define <slug>` | commit message drafting, issue text, or design note | Git can store prose, but not phase semantics | Creates acceptance criteria and implementation plan as first-class artifact |
| `tpatch explore <slug>` | ad hoc code reading before commit | Git has no representation for exploration | Captures file-level investigation and intended change surface |
| `tpatch implement <slug>` | editing files / preparing a patch | Git can store resulting diff only | Produces deterministic operation-level replay recipe |
| `tpatch apply <slug>` | applying a patch, cherry-pick, checkout of modified tree | Git can apply content changes mechanically | Replays the recipe with path safety and feature-aware state transitions |
| `tpatch record <slug>` | `git diff`, patch generation | Git can capture diffs and patches | Captures canonical feature patch and updates feature metadata around it |
| `tpatch status` | `git status`, `git branch --show-current`, custom dashboards | Git reports working tree and ref state | Reports feature lifecycle, reconcile status, dependency overlays, and structured maintenance state |
| `tpatch test <slug>` | running tests in shell / CI | Git does not own test semantics | Ties project verification to a feature lifecycle |
| `tpatch reconcile` | merge, rebase, cherry-pick, patch replay | Git can mechanically merge or rebase | Adds intent-aware replay, operation-level inspection, semantic verdicts, and feature-scoped maintenance |
| `tpatch cycle <slug>` | shell script around many git and build commands | Git does not provide lifecycle orchestration | Treats feature evolution as one structured workflow |

## Where the mapping is shallow vs deep

Not every tpatch command is equally distinct from Git.

### Mostly orchestration over Git-adjacent mechanics

These commands are relatively close to operations users already perform around Git:

- `apply`
- `record`
- parts of `status`
- parts of `reconcile`

The difference is mostly that tpatch scopes the operation to a feature and captures structured metadata around it.

### Genuinely new semantic layers

These commands represent capabilities Git does not really model directly:

- `add`
- `analyze`
- `define`
- `explore`
- feature DAG management
- semantic reconcile outcomes

These are not just wrappers over Git commands. They are modeling work that exists before, above, or across Git commits.

## What current tpatch artifacts mean in Git terms

The command mapping is easier to understand when paired with the main artifacts.

| tpatch artifact | Rough Git analogy | Why the analogy breaks down |
|---|---|---|
| `request.md` | issue description or commit intent | Not tied to a single commit; survives across rewrites and reconcile |
| `spec.md` | detailed commit message or design doc | Machine-addressable acceptance contract, not just prose |
| `exploration.md` | review notes or scratchpad | Formalized change-surface grounding |
| `status.json` | combination of refs, notes, and custom state | Git has no built-in feature lifecycle record |
| `apply-recipe.json` | scripted patch application | Git stores patch content, not deterministic operation scripts |
| `artifacts/post-apply.patch` | patch file from `git diff` | Scoped as the canonical feature diff, not just an arbitrary exported patch |
| DAG metadata | commit ancestry or branch dependency | Git ancestry is commit-based, not feature-based or soft/hard dependency-aware |

## What Git can emulate with conventions

A disciplined Git workflow can imitate parts of current tpatch.

Possible Git-only approximations:

- commit messages containing feature contracts,
- trailers or tags linking related commits,
- branch-per-feature naming conventions,
- separate metadata commits,
- patch files stored in the repo,
- issue IDs or custom change IDs grouping commits.

This matters because it sets the burden of proof for any future tpatch expansion. The system must do more than what good Git hygiene could already achieve.

## What Git conventions still do not solve well

Even with strong commit conventions, several tpatch needs remain awkward in plain Git:

| Need | Why Git conventions are not enough |
|---|---|
| Feature-level lifecycle | Commit history does not express `requested`, `defined`, `implementing`, `applied`, `reconciling` cleanly |
| Deterministic operation replay | Commit diffs do not say *how* to re-apply in ordered semantic steps |
| Semantic upstream resolution | Commit ancestry does not answer whether upstream already satisfies intent |
| Feature DAG with soft/hard edges | Commit graph ancestry is not the same as dependency semantics |
| Stable feature identity across rebases | SHAs change; message conventions are too weak for reliable machine semantics |
| Agent-authored phase artifacts | Git stores outputs, not workflow phase contracts |

## Current friction points exposed by the mapping

The mapping makes a few product tensions explicit.

### 1. Content capture and commit capture are still separate

Today `tpatch record` captures the canonical feature patch, but Git commit capture remains an external step. That means the user must still manually bridge:

- feature-scoped capture,
- Git staging,
- Git commit boundaries,
- and metadata persistence.

This is the clearest candidate for a first integration improvement.

### 2. Feature identity is stronger than commit identity, but only inside `.tpatch/`

Within tpatch, a slug is a stable feature identity. Within Git, that identity is only reflected indirectly through conventions. The result is a split-brain model:

- tpatch knows the feature precisely,
- Git history often only hints at it.

### 3. Reconcile acts like a higher-order merge, but Git still owns the visible history outcome

Reconcile already reasons about intent, replay, and semantic drift, which is beyond a plain merge or rebase. But the persistent history still ends up being shaped via ordinary Git operations the user performs afterward.

## Candidate integration opportunities

These are the most plausible short- to medium-term opportunities suggested by the mapping.

### Opportunity A — One-step feature landing

Add a command that combines:

- `record`,
- metadata update,
- Git staging,
- and Git commit creation,

into one user-visible operation.

This would not replace Git. It would reduce one of the biggest seams between tpatch semantics and Git history creation.

### Opportunity B — Stable feature-to-commit binding

Define a first-class mapping from feature slug to Git history projection. Possible implementations include:

- required trailers,
- structured commit metadata,
- metadata commits,
- hidden refs,
- or a tpatch-side graph that points to ordinary commits.

### Opportunity C — Operation-level history

Git commit history answers "what content became visible." It does not answer "what workflow action happened." A separate tpatch operation log could record:

- apply,
- amend,
- record,
- reconcile,
- split,
- absorb,
- land,

as first-class operations distinct from commit objects.

## Evaluation questions

1. Which current tpatch verbs should remain explicit even if Git integration becomes tighter?
2. What is the minimum viable single-operation workflow that improves user experience without obscuring Git history?
3. Should feature-to-commit identity be projected into Git commits directly, or maintained separately in `.tpatch/`?
4. Which future operations would most benefit from an operation log that is separate from commit history?
5. Where does Git remain the right user-facing abstraction, and where should tpatch become primary?

## Recommendation

Treat this note as an architectural map, not a feature proposal.

The best immediate use is to ground future experiments. In particular:

1. use this map to scope a narrow `tpatch land` or `record-and-commit` experiment,
2. use it to judge whether future ideas are really new semantics or just wrapper UX,
3. and use it as a checklist against accidental reinvention of Git.

If the project later decides to move toward a stronger Git-backed intent system, this note should become the baseline comparison document.
