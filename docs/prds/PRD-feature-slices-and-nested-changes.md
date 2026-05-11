# PRD — Exploring Feature Slices and Nested Change Units

**Status**: Exploratory note
**Date**: 2026-04-27
**Owner**: Core
**Milestone**: Side exploration; not yet roadmap-committed
**Related**: [PRD-intent-version-control-evaluation.md](PRD-intent-version-control-evaluation.md), [PRD-tpatch-git-primitive-mapping.md](PRD-tpatch-git-primitive-mapping.md), [SPEC.md](../../SPEC.md), [docs/feature-layout.md](../feature-layout.md), [docs/dependencies.md](../dependencies.md), [docs/reconcile.md](../reconcile.md)

## Summary

This note explores whether `tpatch` needs a unit smaller than a feature but richer than a plain Git commit. The motivating problem is practical rather than theoretical: some real features become too large to reason about as one patch, one recipe, one reconcile target, or one maintenance unit.

The hypothesis is:

> Large features should be decomposable into smaller, replayable, inspectable units that stay tied to the parent feature's intent, rather than forcing users to choose between one oversized feature and a set of unrelated Git commits.

This note does not propose an implementation yet. It frames the problem, lists candidate models, and defines the evaluation questions for a future experiment.

## Why this note exists

The current `tpatch` model treats the feature as the primary unit of:

- request capture,
- acceptance criteria,
- replay artifacts,
- dependency semantics,
- and reconcile maintenance.

That is usually the right level of abstraction. The problem appears when a feature becomes too large or too internally diverse:

- multiple logically separate sub-changes live under one feature,
- the recipe becomes long and harder to trust,
- the patch becomes broad and harder to inspect,
- and later reconcile may need to understand which part of the feature is drifting.

Git's default answer is "split it into multiple commits." That helps content history, but it does not solve the `tpatch` problem on its own, because the feature's intent and reconcile semantics may still belong together.

## The core problem

Today the project has a strong top-level unit, the feature, but no explicit middle layer between:

- a whole feature, and
- a raw Git commit or hunk.

That leaves several cases awkward:

| Scenario | Why the current model is awkward |
|---|---|
| One feature has three clearly separable sub-parts | one patch and one recipe flatten the internal structure |
| One part of a feature is already upstreamed but the rest is not | reconcile still reasons about the parent feature as one unit |
| A feature needs staged rollout or partial acceptance | feature is too coarse; commit is too content-centric |
| An agent can implement one portion confidently but wants to defer another | there is no first-class sub-feature identity |
| A feature spans refactors plus behavior change | one patch may hide the more semantic boundaries |

## Why Git commits are not enough

Commits are useful subunits, but they do not fully solve the problem `tpatch` is trying to solve.

| Git commit strength | Why it helps | Why it is still insufficient |
|---|---|---|
| atomic content snapshot | good for review and history | does not carry first-class feature intent |
| easy to split and reorder | helps with large changes | commit boundaries often reflect editing flow, not product intent |
| can be stacked | approximates sub-features | no explicit parent feature contract |
| commit message can explain rationale | some human context | not a structured replay or reconcile model |

In other words, commits help with atomicity, but they do not automatically give the project a better semantic maintenance unit.

## Candidate concepts

Three terms are useful for exploration. They are intentionally provisional.

| Candidate term | Meaning | Why it may help |
|---|---|---|
| Feature slice | A coherent child unit of a parent feature, with its own change boundary | preserves parent feature identity while allowing smaller units |
| Nested feature | A child feature that inherits or references parent intent | useful when the sub-unit is independently meaningful |
| Intent commit | A projection of a richer unit into Git-compatible history | bridges semantic unit and Git storage |

These are not equivalent.

### Feature slice

A feature slice is the most conservative extension.

Properties:

- always belongs to one parent feature,
- may have its own patch and recipe,
- may or may not have its own request/spec fragments,
- is not necessarily independently user-visible outside the parent feature.

This is the best fit if the problem is mostly atomicity and maintainability.

### Nested feature

A nested feature is a stronger concept.

Properties:

- has a parent-child relationship with another feature,
- may carry its own lifecycle and reconcile semantics,
- may be independently reviewable or deployable,
- may later be promoted to a top-level feature.

This is attractive if sub-units are often product-meaningful in their own right, but it risks turning the model into "features all the way down."

### Intent commit

An intent commit is not necessarily a new stored object. It is a way of saying that a richer semantic unit may need a deliberate projection into Git history.

Properties:

- stable semantic identity above the raw commit SHA,
- can survive commit reshaping better than commit-message conventions alone,
- may be the projection target for a slice or nested feature.

This is attractive if the project moves further toward the intent-aware VCS direction.

## Candidate models to evaluate

### Option A — Feature slices under a parent feature

Model:

- the top-level feature remains the primary unit of intent,
- optional child slices capture smaller replayable parts,
- the parent feature groups slices and remains the main user-facing contract.

Possible structure:

```text
.tpatch/features/<feature>/
  status.json
  request.md
  spec.md
  slices/
    001-layout/
      status.json
      artifacts/post-apply.patch
      artifacts/apply-recipe.json
    002-wireup/
      status.json
      artifacts/post-apply.patch
      artifacts/apply-recipe.json
```

Pros:

- keeps one parent feature contract,
- gives atomic units for replay and review,
- likely easier to add incrementally.

Cons:

- introduces a second hierarchy inside feature storage,
- requires clear rules for parent versus slice ownership of artifacts,
- reconcile semantics may get more complex quickly.

### Option B — Nested features with explicit parent-child links

Model:

- child units are just features with parent links,
- the existing feature model is reused instead of inventing a new object type,
- grouping semantics live in relationships rather than in a separate slice schema.

Pros:

- reuse of existing feature primitives,
- strong flexibility,
- easier to expose in status and DAG tooling using existing patterns.

Cons:

- risks overloading feature semantics,
- could make simple repositories feel too structurally heavy,
- may blur the distinction between dependency and containment.

### Option C — Keep features flat, project sub-units only into Git

Model:

- `tpatch` features remain flat,
- atomicity is handled by Git commits or hidden projections,
- the feature links to multiple commits but does not own sub-feature objects.

Pros:

- simpler `tpatch` data model,
- easier Git compatibility story.

Cons:

- weakest support for reconcile-at-subunit-level,
- may leave too much burden on commit conventions,
- does not fully solve the need for smaller semantic maintenance units.

## Questions a slice model must answer

Any serious design in this area will need precise answers to the following.

### 1. What owns intent?

Options:

- only the parent feature owns `request.md` and `spec.md`,
- slices own smaller local specs,
- or parent and slice both own scoped intent artifacts.

Tradeoff:

- too much local intent duplicates the parent contract,
- too little local intent makes slices hard to reason about independently.

### 2. What owns replay artifacts?

Questions:

- does each slice get its own `post-apply.patch` and `apply-recipe.json`?
- does the parent keep an aggregate patch and recipe?
- if both exist, which one is canonical for reconcile?

This is the most dangerous ambiguity. The current system works partly because [docs/feature-layout.md](../feature-layout.md) makes the canonical patch authority explicit.

### 3. What is the reconcile unit?

Options:

- reconcile parent only,
- reconcile each slice independently,
- or support both.

Tradeoff:

- parent-only keeps the model simpler but loses granularity,
- per-slice reconcile could surface much better maintenance signals but may be substantially more complex.

### 4. How do dependencies work?

Questions:

- can slices depend on other slices?
- can a feature depend on a slice in another feature?
- is slice ordering containment-only, while inter-feature DAG remains feature-level?

Recommendation for early exploration:

- keep inter-feature dependencies at feature level first,
- treat slice ordering as local-to-parent until a stronger use case appears.

### 5. How does Git projection work?

Questions:

- should each slice map to one commit, many commits, or no explicit commit boundary?
- if slices are rebased or squashed, how does the parent feature track them?
- does the user see slices directly in Git history, or only through `tpatch` views?

## Constraints for a good solution

Any slice or nested-change design should preserve the current strengths of `tpatch`.

| Constraint | Why it matters |
|---|---|
| Canonical patch authority must remain clear | avoids replay and reconcile ambiguity |
| Agent workflows must stay deterministic | extra structure cannot become vague or hand-maintained |
| Large-feature support must improve materially | otherwise the model is just more metadata |
| Backward compatibility must be understandable | existing feature repos should not become ambiguous |
| Reconcile semantics must remain explainable | slices should not create opaque maintenance behavior |

## A cautious evaluation path

This area is easy to overdesign. The best path is incremental.

### Phase 0 — Study real oversized features

Before designing the object model, gather concrete examples:

- features that felt too large to maintain as one unit,
- features that were split across many commits but still one logical contract,
- features where only one sub-part drifted during reconcile.

The first design should be grounded in those cases, not in general desire for hierarchy.

### Phase 1 — Model slices on paper only

Deliverable:

- one design note with 2 or 3 concrete examples of a feature decomposed into candidate slices,
- exact answers for ownership of intent, patch authority, and reconcile unit.

No code yet.

### Phase 2 — Small prototype in one repo fixture

Possible narrow experiment:

- one parent feature,
- two local slices,
- each with its own patch and recipe,
- no cross-slice dependencies beyond local order,
- no Git projection changes yet.

Goal:

- learn whether the extra structure makes review and reconcile meaningfully easier.

### Phase 3 — Decide whether Git projection is needed

Only after slices prove useful should the project decide whether:

- slices need explicit Git linkage,
- slices should map to commits,
- or the feature graph can stay mostly independent from Git history.

## Risks

| Risk | Why it matters |
|---|---|
| Hierarchy overload | simple features may become burdened by unnecessary structure |
| Confusion between dependency and containment | parent-child and depends-on are not the same relation |
| Patch authority ambiguity | multiple patches per feature can make replay unsafe or unclear |
| Harder agent ergonomics | more objects can mean more ways to get state wrong |
| Reconcile explosion | per-slice maintenance may multiply surface area too much |

## Open questions

1. Is the right new unit a slice, a nested feature, or just a stronger commit projection?
2. Should slices be optional and invisible for small features, or part of the core model?
3. Does each slice need its own lifecycle, or only a local ordering state?
4. Should reconcile operate at parent level, slice level, or both?
5. What is the smallest slice model that meaningfully improves large-feature workflows without breaking the current simplicity of `tpatch`?

## Recommendation

Do not jump directly to nested features as a core product direction. Start with the narrower concept of feature slices.

Why:

- it addresses the immediate large-feature and atomicity problem,
- it is easier to keep local to a parent feature,
- and it avoids prematurely turning containment into another form of DAG dependency.

The best next step is not implementation. It is a case-study-driven paper design that answers three questions clearly:

1. what is the canonical patch authority when slices exist,
2. what is the reconcile unit,
3. and how much Git projection is actually needed.

If the answers remain simple after that exercise, the idea is worth prototyping. If not, the project should prefer a lighter Git-projection improvement instead of a richer slice model.
