# PRD — Evaluating tpatch as an Intent-Aware Version Control Superset

**Status**: Exploratory note
**Date**: 2026-04-27
**Owner**: Core
**Milestone**: Side exploration; not yet roadmap-committed
**Related**: [SPEC.md](../../SPEC.md), [docs/feature-layout.md](../feature-layout.md), [docs/record.md](../record.md), [docs/reconcile.md](../reconcile.md), [docs/agent-as-provider.md](../agent-as-provider.md)

## Summary

This note explores a larger direction for Tessera Patch: instead of treating `tpatch` as a workflow that sits beside Git, evaluate whether it should evolve into an intent-aware change system that uses Git as a storage and transport substrate while adding first-class models for feature intent, replay semantics, agent actions, and higher-order history.

The working hypothesis is:

> Git is strong at recording content history, distribution, and merge mechanics. tpatch is already strong at recording feature intent, replay instructions, and semantic reconciliation. A useful long-term direction may be to combine these into a single user-facing operation model where the user thinks in features, slices, intent, and reconciliation, while Git remains an underlying compatibility layer.

This is not a proposal to replace Git in the short term. It is an evaluation plan for a possible next layer on top of Git, similar in spirit to how `jj` rethinks workflows while remaining Git-compatible.

## Why this note exists

Today the project already has two overlapping systems:

1. Git manages file history, commits, branches, transport, and merge mechanics.
2. tpatch manages feature intent, request/spec/exploration artifacts, canonical feature patches, replay recipes, and reconcile semantics.

That split is pragmatic, but it creates friction:

- Users run a tpatch lifecycle and then separately decide how to stage and commit the resulting code and metadata.
- A single Git commit may contain multiple features, while a single tpatch feature may span many edits and commits.
- Git commit messages can describe why something changed, but they do not enforce structured feature intent, replay steps, dependency semantics, or reconcile outcomes.
- Git diff is primarily file and hunk oriented; tpatch increasingly reasons in terms of features, operations, dependencies, and semantic drift.

This note asks whether tpatch should remain a sidecar forever, or whether it should gradually become a more opinionated, intent-aware layer over version control.

## Core hypothesis

The long-term opportunity is not merely to add more metadata to Git commits. It is to define a richer unit of change with at least three layers:

| Layer | Current primary owner | What it represents |
|---|---|---|
| Content layer | Git | The byte-level code changes, tree state, commit graph, transport, merge, rebase |
| Operation layer | tpatch recipe and reconcile machinery | The ordered steps by which the change can be replayed, validated, or merged |
| Intent layer | tpatch request/spec/exploration/status artifacts | Why the change exists, what problem it solves, what acceptance criteria it must satisfy |

Git already owns the first layer extremely well. tpatch already owns the second and third layers better than Git does. The evaluation question is whether the user-facing workflow should eventually treat all three as one coherent system.

## What tpatch already has that fits this direction

The project is already closer to an intent-aware VCS than it may appear at first glance.

| Existing tpatch concept | Closest VCS analogy | Why it matters |
|---|---|---|
| `request.md`, `spec.md`, `exploration.md` | Commit message plus design note, but structured | Records intent and acceptance criteria as first-class artifacts rather than prose in a commit body |
| `artifacts/post-apply.patch` | Canonical patch snapshot | Separates current feature diff from audit history and from transient working tree state |
| `apply-recipe.json` | Deterministic replay script | Adds an operation-level representation that Git does not have |
| `status.json` | State/ref metadata | Tracks lifecycle, reconcile outcome, timestamps, and machine-readable feature facts |
| `record` | Snapshot capture | Already models the idea that feature state should be captured independently of commits |
| `reconcile` | Semantic merge/rebase layer | Goes beyond mechanical patch application and asks whether intent is already satisfied |
| Feature DAG (`depends_on`, `created_by`, `satisfied_by`) | Dependency graph over changes | Adds relations between changes that Git does not represent directly |
| Shadow worktrees | Isolated merge/apply sandbox | Similar to staging alternative histories without mutating the real tree |
| Path B / agent-authored artifacts | Manual change authoring with machine state | Already treats the agent as a structured change producer, not just a commit author |

Put differently: Git gives the repo a history of trees. tpatch already gives the repo a history of feature intent, replayability, and semantic maintenance.

## What Git already covers well

Any expansion of tpatch should start by acknowledging what Git already does better than a new bespoke system is likely to do for a long time.

| Git strength | Why we should keep using it |
|---|---|
| Content-addressed storage | Efficient, battle-tested, widely interoperable |
| Commit graph and refs | Excellent model for ancestry, branch movement, and transport |
| Diff, merge, rebase, cherry-pick | Mature mechanical history-editing primitives |
| Remotes and forge interoperability | Essential for collaboration; not something tpatch should reimplement lightly |
| Blame, bisect, tags, release tooling | Important ecosystem advantages |
| Existing IDE/tool integration | Editors, CI, code hosts, and build tools already assume Git |

This makes a Git-backed or Git-colocated model more realistic than a clean-sheet repository backend.

## Where Git is not enough for tpatch's goals

Git can store intent in commit messages, branch naming, trailers, tags, or conventions. That is useful, but it is not the same as a first-class intent model.

| Limitation in plain Git | Why it matters for tpatch |
|---|---|
| Commit messages are free-form | They do not give machine-checked acceptance criteria, replay steps, or semantic states |
| One commit can mix many features | Hard to recover feature intent boundaries after the fact |
| One feature can span many commits | Git has no native grouping for "these commits are one feature contract" |
| Diffs are file/hunk centric | Git does not express change as ordered operations or feature steps |
| Rename/move history is heuristic | The reasoning behind a move or semantic split is not preserved |
| Merge/rebase are mechanical | They do not answer whether upstream now satisfies the feature's intent |
| No first-class dependency graph between features | DAG semantics today live entirely outside Git |
| No operation log over user intent | Git records content history, not "what structured workflow action happened and why" |

In principle, a disciplined team could emulate parts of tpatch with commit message conventions and tags. In practice, the structure would be weaker, more fragile, and much harder for agents to consume reliably.

## Comparison point: why `jj` is relevant

`jj` is a useful reference because it demonstrates that a tool can rethink user workflows while remaining Git-compatible.

Relevant concepts from `jj`:

| `jj` concept | Why it is relevant here |
|---|---|
| Git-backed repositories | Suggests a path where tpatch can remain compatible with Git storage and remotes |
| Colocated workflow | Shows that one tool can provide the primary UX while Git remains present for tooling compatibility |
| Operation log | Shows value in recording user-visible operations separately from commit history |
| Mutable working-copy revision model | Suggests a friendlier workflow than forcing every logical step into a visible Git commit |
| Automatic import/export with Git | Suggests a model where the user experiences one system while compatibility is maintained underneath |

The important lesson is not "copy `jj`." The useful lesson is: a Git-backed system can legitimately expose a higher-level model than raw commits and branches.

## The missing piece: a better change unit

One recurring pain in real use is that features can become too large. Git's answer is "split into more commits," but that does not always solve the tpatch problem, because the feature's intent, recipe, dependency semantics, and reconcile history may still logically belong together.

This suggests the project may need a unit smaller than a feature but richer than a plain commit.

Working names:

| Candidate unit | Rough meaning |
|---|---|
| Feature slice | A coherent part of a feature with its own patch/recipe but still grouped under a parent feature |
| Change step | A replayable, inspectable step in a feature plan |
| Intent commit | A Git-compatible commit projection of a richer tpatch change unit |

The central requirement is atomic friendliness:

- large features should be decomposable into smaller units,
- those units should remain tied back to the parent feature intent,
- each unit should still be replayable and reconcilable,
- and users should not have to hand-maintain that grouping in ad hoc commit messages.

## Candidate system model

The simplest useful target model is:

1. Git remains the underlying content history and transport layer.
2. tpatch becomes the primary user-facing change workflow.
3. A single tpatch operation can perform several lower-level storage actions while presenting one user-visible action.
4. Feature intent, operation steps, and code content are linked by stable identifiers instead of loose commit-message conventions.

That could look like this from the user's point of view:

```text
tpatch land extra-button
  -> capture content changes for the feature or slice
  -> capture/update intent metadata
  -> project both into Git-compatible history
  -> leave the repo in a clean, explainable state
```

The user sees one operation. Under the hood, the system may create:

- one code commit,
- one metadata commit,
- a hidden ref,
- or a single combined commit with structured metadata,

depending on the model we choose.

## Design options to evaluate

### Option A — Better sidecar over Git

tpatch remains a sidecar, but gains a first-class "record and commit" flow.

Possible properties:

- `tpatch` stages the right code files automatically,
- `tpatch` updates `.tpatch/` metadata automatically,
- `tpatch` creates the Git commit(s) automatically,
- the user triggers one command instead of separate `tpatch` then `git` steps.

Pros:

- Lowest migration risk.
- Preserves today's mental model.
- Gives immediate UX gains.

Cons:

- Still fundamentally commit-centric underneath.
- Harder to represent feature slices, operation logs, or richer change graphs cleanly.

### Option B — Git-backed intent VCS

tpatch becomes the primary change manager, with Git as backend and compatibility surface.

Possible properties:

- stable tpatch change IDs separate from Git commit SHAs,
- explicit operation log,
- automatic import/export to Git,
- support for feature grouping and slicing above raw commits.

Pros:

- Best fit for the long-term "superset of VCS" idea.
- More room for intent-aware workflows and agent-native history.

Cons:

- Larger conceptual jump.
- Higher implementation and support cost.
- Greater risk of fighting existing Git tooling semantics.

### Option C — Intent graph over Git commits

Keep Git commits as primary content objects, but add a first-class tpatch graph that groups them into features, slices, dependencies, and semantic states.

Possible properties:

- Git commits remain normal and visible,
- tpatch adds a separate graph of feature objects and slice objects,
- each slice points to one or more commits,
- feature history can survive rebases and commit reshaping better than commit-message conventions alone.

Pros:

- Strong compatibility story.
- Easier incremental adoption.
- Cleaner bridge from today's model.

Cons:

- Dual-model complexity.
- Some operations may feel redundant if both commit graph and feature graph must be kept in sync.

## A possible data model extension

This is not a final proposal. It is a frame for evaluation.

| Object | Purpose |
|---|---|
| Feature | Top-level user-visible unit of intent and maintenance |
| Slice | Optional child unit for atomic portions of a large feature |
| Operation record | One user-visible action: apply, amend, reconcile, split, land, absorb, etc. |
| Content projection | The Git commit(s) or patch snapshot corresponding to the operation |
| Intent projection | Request/spec/exploration and structured rationale linked to the operation |

The interesting difference from plain Git is that the system would track not only "what changed" but also:

- why it changed,
- how it was intended to be applied,
- what feature or slice it belongs to,
- what operation created or modified it,
- and how it should be reconciled as upstream evolves.

## Where current tpatch still falls short

Even though tpatch already has many of the right building blocks, several gaps remain before it could credibly act as a higher-level change system.

| Gap | Why it matters |
|---|---|
| No first-class "land/commit with tpatch" user operation | Users still manually bridge tpatch lifecycle and Git commit workflow |
| No first-class feature slices | Large features still become awkward units |
| No operation log independent of content history | The system cannot yet answer "what workflow action happened" as clearly as it answers "what patch exists" |
| Weak mapping between features and Git commits | Today the relationship is mostly conventional, not enforced |
| No stable identity for sub-feature changes across rebases | Git SHAs alone are insufficient for intent grouping |
| Diff is still mostly patch-centered | Operation-level and intent-level diffs are not yet first-class views |
| No multi-backend abstraction beyond Git-centric assumptions | Harder to grow into "later other VCS operations" cleanly |

## Evaluation plan

This note is intentionally a planning document, not a final product proposal. The goal is to structure exploration before backlog commitments.

### Phase 0 — Map the current system more explicitly

Deliverable:

- an appendix or follow-up note mapping every major tpatch command to the closest Git primitive and to the extra semantics tpatch adds.

Questions:

- Which commands are already acting like VCS operations?
- Which metadata files are already functioning like an intent history?
- Which case-study pains are really Git pains versus tpatch pains?

### Phase 1 — Prototype a single visible "land" operation

Goal:

- test the narrowest version of the idea without changing the full model.

Candidate prototype:

- one command that captures the feature patch,
- updates `.tpatch/` metadata,
- stages only the intended files,
- and creates Git history in one user-visible action.

What to measure:

- fewer user mistakes around `record` timing,
- cleaner feature-to-commit correspondence,
- whether separate metadata commits help or confuse.

### Phase 2 — Evaluate feature slices

Goal:

- test whether a sub-feature unit solves the large-feature problem better than "just make more commits."

Questions:

- does a slice need its own `post-apply.patch` and `apply-recipe.json`?
- is a slice purely local to a parent feature, or independently reconcilable?
- how are dependencies expressed between slices and features?

### Phase 3 — Evaluate an operation log

Goal:

- determine whether the system needs a first-class operation log similar in spirit to `jj op log`.

Questions:

- should tpatch record workflow operations independently of content commits?
- do users need undo/restore at the operation level?
- can this stay purely in `.tpatch/`, or must it project into Git refs/commits?

### Phase 4 — Decide architectural direction

Decision point:

- remain a stronger sidecar,
- become a Git-backed intent VCS layer,
- or build an intent graph over normal Git commits.

At this point the project should only commit roadmap scope after at least one user-facing prototype and one review cycle grounded in real usage.

## Suggested evaluation criteria

Any future proposal in this area should be judged against the following:

| Criterion | Why it matters |
|---|---|
| Git compatibility | Existing repos, CI, forges, and tooling must keep working |
| Better reasoning over change intent | Must deliver more than commit-message conventions already can |
| Better handling of large features | Should make big work easier to split and maintain |
| Replay and reconcile quality | Must improve, or at least not weaken, current tpatch strengths |
| Low operator confusion | One user-visible action should not create opaque hidden state |
| Recoverability | Users must be able to audit, undo, and repair mistakes |
| Agent ergonomics | The model must be easier, not harder, for agents to follow deterministically |

## Risks

| Risk | Why it is serious |
|---|---|
| Reinventing too much of Git badly | The project could lose the advantages of existing tooling |
| Dual-history confusion | Users may struggle if Git history and tpatch history disagree |
| Metadata overgrowth | Too much structure can make normal work feel heavy |
| Hidden magic around commits | One visible command must still leave an auditable result |
| Overfitting to one backend | The model should be Git-first today without becoming Git-locked forever |

## Open questions

1. What should the minimal user-visible operation be: `land`, `commit`, `record-and-commit`, or something else?
2. Should metadata live in ordinary commits, hidden refs, parallel commits, or only `.tpatch/` tracked content?
3. Does a feature slice need to be independently reconcilable, or only replayable within its parent feature?
4. Should the system privilege commit compatibility first, or operation-log clarity first?
5. Which current case-study problems would actually disappear if `tpatch` owned the commit step?
6. What is the smallest experiment that would prove this direction is worth further roadmap investment?

## Recommendation

Do not treat this as a near-term rewrite of Git integration. Treat it as a research direction with one narrow product experiment first.

The best first experiment is likely:

1. add one user-visible `tpatch` command that captures feature code and metadata and produces the necessary Git history automatically,
2. keep Git as the primary storage and compatibility surface,
3. defer feature slices and operation logs until the single-operation workflow proves valuable,
4. use the result to decide whether the project should stay a stronger sidecar or move toward a true Git-backed intent system.

That path gives the project a concrete way to test the core idea without prematurely committing to a full VCS redesign.

## Follow-up candidates

If this exploration survives review, likely follow-ups are:

- a backlog item for a single-operation `tpatch land` or equivalent,
- a design note mapping current tpatch commands to Git primitives and extra semantics,
- a prototype note for feature slices or nested features,
- and a later ADR if the project decides to formalize an operation log or a Git-colocated model.
