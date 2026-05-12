# tpatch Middle-pass Synthesis

**Status**: Snapshot synthesis (paper-only; no implementation authorized)
**Date**: 2026-05-10
**Owner**: Core
**Related**: [Patch theory research](patch-theory-and-commutation.md),
[Structural fingerprint research](patch-identity-and-structural-fingerprints.md),
[Metadata sufficiency research](tpatch-metadata-for-patch-identity.md),
[Search-based application research](search-based-patch-application.md),
[Experiment guide](experiment-guide-structural-middle-pass.md),
[Reconcile Workflow](../reconcile.md),
[SPEC §7 reconciliation](../../SPEC.md#7-reconciliation--4-phase-decision-tree),
[PRD-patch-already-upstream-detector](../prds/PRD-patch-already-upstream-detector.md),
[PRD-provider-conflict-resolver](../prds/PRD-provider-conflict-resolver.md)

## Why this doc exists

The research packet points toward a possible middle pass for tpatch:

> a reproducible structural/search planner that sits between deterministic
> hunk-level heuristics and full provider-backed semantic resolution.

This doc synthesizes the research into tpatch-specific language. It is not a
PRD. It names candidate seams and guardrails for future design work.

## Refresh triggers

- A PRD or ADR is opened for structural fingerprints, commutation artifacts, or
  search-based reconcile.
- The current reconcile ladder changes.
- Case-study data shows provider calls are dominated by cases that structural
  search could likely solve.

## 1. Current gap

Current reconcile is intentionally conservative:

1. reverse-apply checks whether the patch is already present;
2. operation-level evaluation checks deterministic recipe operations;
3. provider-semantic analysis asks whether intent is satisfied;
4. forward-apply attempts `git apply --3way`;
5. provider-assisted resolver can handle selected conflicts when enabled.

The gap is between "deterministic operation check failed" and "ask a model to
reason semantically." Many real cases are not semantic mysteries:

- the function moved;
- the file was renamed;
- the same patch was cherry-picked with context drift;
- first-party tpatch metadata can identify the patch generation, base, anchors,
  and declared dependencies before fuzzy algorithms run;
- an embedding search can retrieve the moved or rewritten target chunk;
- two local patches need a different order;
- hunk anchors are weak but structural anchors survive.

Those cases may be solvable with structural fingerprints and bounded search.

## 2. Proposed conceptual seam: phase 2.5

Research-only ladder:

```text
phase 1     reverse-apply
phase 1.5   patch-id upstream detector      (already proposed by PRD)
phase 2     operation-level evaluation
phase 2.5   structural/vector/search planner
phase 3     provider-semantic
phase 3.5   provider-assisted resolver
phase 4     forward-apply preview
```

Phase 2.5 would not replace provider assistance. It would reduce provider calls
and improve provider prompts by producing grounded evidence.

## 3. Middle-pass components

### 3.1 Identity sweep

Purpose: classify cheap identity wins before search.

Inputs:

- canonical `post-apply.patch`,
- upstream commit range,
- current feature store,
- optional patch generations if ADR-015 follow-up work lands,
- first-party identity metadata if a future metadata PRD lands.

Signals:

- exact SHA-256,
- `git patch-id`,
- stable feature slug and moving patch generation,
- base/upper-bound capture metadata,
- declared dependency snapshots,
- token fingerprints,
- structural fingerprints,
- vector retrieval matches.

Outputs:

- `exact_patch_match`,
- `patch_id_match`,
- `structural_match_high`,
- `structural_match_low`,
- `vector_match_high`,
- `vector_match_low`,
- `unknown`.

### 3.2 Structural anchor relocation

Purpose: find where an old hunk/operation should land after upstream movement.

Analogy:

- old hunk context = image patch around a keypoint;
- new upstream = second image;
- code landmarks = keypoints;
- local AST/token/call descriptors = feature descriptors;
- file/order consistency = RANSAC-style geometric verification.

Possible output:

```json
{
  "feature": "dynamic-models",
  "operation": "op-004",
  "old_anchor": "apps/api/src/models.ts:function listModels",
  "new_anchor": "packages/server/src/models/catalog.ts:function listModels",
  "confidence": 0.91,
  "evidence": ["signature", "string-literals", "call-neighborhood", "order-consistency"]
}
```

This would be diagnostic evidence first, not an auto-write contract.

### 3.3 Vector retrieval / RAG evidence

Purpose: retrieve candidate matches before provider-semantic reasoning.

Vector retrieval fits in phase 2.5 when it is used only to rank candidates:

- old patch hunk -> nearest current upstream chunks;
- feature intent -> nearby upstream commits or prior tpatch features;
- conflict shape -> prior accepted resolutions;
- keypoint descriptor -> likely moved symbol/function.

RAG with a generator is different. Retrieval can prepare a smaller, better
grounded phase-3 prompt, but once a provider reasons over retrieved chunks, the
result is provider-assisted and should be audited as such.

Required evidence:

- embedding model and version;
- source text hash for each vector;
- top-k IDs and scores;
- sparse filters that agreed or disagreed with the dense match;
- whether generation was used.

### 3.4 Commutation graph

Purpose: shrink the ordering problem.

Relations:

- `commutes`,
- `depends_on`,
- `conflicts`,
- `unknown`.

Evidence:

- hard feature dependency,
- file/hunk overlap,
- structural write-set overlap,
- empirical `A;B` vs `B;A`,
- test or validation result.

This graph lets tpatch avoid global factorial search. Search only runs on
uncertain connected components.

### 3.5 Search planner

Purpose: choose a candidate order and relocation set for uncertain clusters.

Candidate algorithm ladder:

1. deterministic topological sort when enough edges are known;
2. exhaustive enumeration for tiny clusters;
3. beam search as the likely first practical bounded planner;
4. MCTS or evolutionary search if real clusters exceed beam search's useful
   range;
5. provider/human escalation if all candidates fail safety thresholds.

### 3.6 Audit artifact

Purpose: make non-LLM nondeterminism explainable and reproducible.

Must include:

- base SHA and upstream ref;
- feature set;
- algorithm name and parameters;
- seed if stochastic;
- hard constraints;
- candidate attempts and score vectors;
- vector retrieval model/index metadata when used;
- selected order;
- rejected candidates with failure reasons;
- validation/test results;
- whether any provider call was avoided or still required.

## 4. Safety and product guardrails

Any future phase 2.5 should:

- default to preview/report before mutation;
- run only on clean trees;
- isolate candidate attempts in scratch worktrees;
- use deterministic tie-breaking and persisted random seeds;
- never auto-retire a feature from fuzzy structural evidence alone;
- preserve the provider path as fallback;
- surface uncertainty honestly instead of inventing success-shaped defaults;
- keep exact/patch-id matches separate from structural similarity.

## 5. Candidate future PRDs/ADRs

These are names only, not assignments:

| Candidate | Type | Scope |
|---|---|---|
| `PRD-structural-patch-fingerprints` | PRD | Record and compare token/AST/keypoint fingerprints for feature patches. |
| `PRD-feature-patch-identity-metadata` | PRD | Persist patch generation IDs, patch-id, capture provenance, keypoint manifests, and dependency snapshots. |
| `PRD-patch-vector-index` | PRD | Optional vector index over patch hunks, feature intent, code chunks, and prior resolutions. |
| `PRD-reconcile-commutation-graph` | PRD | Build pairwise relation artifacts for active features during reconcile. |
| `PRD-reconcile-search-planner` | PRD | Run bounded search over uncertain patch-order clusters in isolated worktrees. |
| `ADR-structural-middle-pass-boundary` | ADR | Lock where phase 2.5 sits relative to provider-semantic and provider-resolver phases. |
| `PRD-reconcile-planner-audit-artifacts` | PRD | Persist candidate orders, scores, seeds, and validation results. |

## 6. Suggested first experiment

Before any implementation PRD, run an offline case study on existing tpatch
stress repositories. The full collection protocol is
[`experiment-guide-structural-middle-pass.md`](experiment-guide-structural-middle-pass.md).

Minimal shape:

1. Pick a set of recorded features with known collisions or upstream drift.
2. Compute exact SHA and `git patch-id` identities.
3. Extract simple keypoints using only text/Git:
   - file paths,
   - function-like lines,
   - imports,
   - string literals,
   - branch predicates,
   - nearby token k-grams.
4. Optionally extract AST/CFG/vector representations to measure lift over the
   stdlib/Git-only baseline.
5. Build a pairwise overlap/commutation estimate.
6. Compare estimates against actual apply outcomes in scratch clones.
7. Report precision/recall of:
   - duplicate detection,
   - anchor relocation,
   - order prediction,
   - vector top-k retrieval.

The experiment should answer whether a first PRD is worth writing and whether
stdlib-only text/keypoint fingerprints are enough for v1.

## 7. What this means for the current two-strategy model

tpatch can describe three tiers:

| Tier | Name | Role |
|---|---|---|
| 1 | Deterministic heuristics | Exact/patch-id checks, operation-level checks, direct apply. |
| 2 | Structural/vector/search middle pass | Reproducible fuzzy matching, vector retrieval, relocation, commutation graph, bounded candidate search. |
| 3 | Provider/coding-agent workflow | Semantic intent analysis and conflict resolution when algorithms cannot decide. |

The middle tier should not pretend to understand intent. Its job is to exhaust
the non-semantic evidence before paying the cost and risk of semantic
resolution.

## 8. References

- [Patch theory and commutation](patch-theory-and-commutation.md)
- [Patch identity and structural fingerprints](patch-identity-and-structural-fingerprints.md)
- [Metadata sufficiency research](tpatch-metadata-for-patch-identity.md)
- [Search-based patch application](search-based-patch-application.md)
- [Experiment guide](experiment-guide-structural-middle-pass.md)
- [Reconcile Workflow](../reconcile.md)
- [ADR-015 prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md)
- [PRD-patch-already-upstream-detector](../prds/PRD-patch-already-upstream-detector.md)
- [PRD-provider-conflict-resolver](../prds/PRD-provider-conflict-resolver.md)

## Open questions

- Should phase 2.5 be a reconcile subphase, a separate `tpatch plan-reapply`
  command, or an opt-in flag?
- How much language awareness is acceptable under tpatch's low-dependency
  posture?
- Which first-party metadata belongs in `status.json`, which belongs in
  append-only patch-generation manifests, and which should be recomputed?
- Does vector retrieval belong in core, an optional local tool, or a
  provider-backed mode?
- Which confidence class, if any, may skip provider-semantic analysis?
- Should search operate on feature patches only, apply-recipe operations only,
  or both?
- What artifact shape lets future agents audit a planner result without
  replaying every candidate?

## Disputes

None logged.
