# tpatch Metadata for Patch Identity and Dependency Resolution

**Status**: Snapshot research (paper-only; no implementation authorized)
**Date**: 2026-05-10
**Owner**: Core
**Related**: [Feature Layout](../feature-layout.md),
[Feature Dependencies](../dependencies.md),
[ADR-015 prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md),
[PRD-record-auto-base](../prds/PRD-record-auto-base.md),
[PRD-record-collision-detection](../prds/PRD-record-collision-detection.md),
[PRD-tpatch-land](../prds/PRD-tpatch-land.md),
[Patch identity research](patch-identity-and-structural-fingerprints.md),
[Experiment guide](experiment-guide-structural-middle-pass.md)

## Why this doc exists

The structural/vector/search research assumes tpatch may need fuzzy algorithms
when upstream does not carry tpatch metadata. But tpatch controls its own
feature store, recipe format, commit trailers, and future artifact layout. That
means the first question should be:

> What metadata can tpatch carry forward so patch identity, dependency, and
> ordering are obvious for tpatch-aware repos, leaving fuzzy algorithms as a
> fallback for non-tpatch upstreams?

This doc audits the current metadata and names candidate additions. It is not a
schema proposal; any persisted shape still needs PRD/ADR work.

## Refresh triggers

- `PRD-record-auto-base`, `PRD-record-collision-detection`, or
  `PRD-tpatch-land` ships and changes persisted metadata.
- A PRD opens for patch generations, vector indexes, structural fingerprints,
  or commutation graphs.
- A real case study shows the proposed metadata is insufficient or noisy.

## 1. Executive findings

1. **Current tpatch metadata is already good for explicit lifecycle and DAG
   reasoning.** `status.json` has stable slug/state, `apply.base_commit`,
   `depends_on`, `satisfied_by`, reconcile outcome/labels, and verify freshness
   hashes.
2. **Current metadata is weak for moving patch identity.** The canonical patch
   is overwritten in place; prior patch hashes/generations are not preserved
   until ADR-015 D2 is implemented.
3. **Current dependency metadata lacks version pins in the implemented schema.**
   The dependency PRD discussed `since`, but the live `Dependency` shape carries
   only `slug`, `kind`, and `satisfied_by`.
4. **Recipe operations have useful but narrow provenance.** `created_by` helps
   explain parent-created paths, but operations lack stable IDs, anchor IDs,
   read/write sets, or structural fingerprints.
5. **`land` trailers are the strongest near-term bridge into Git.** The planned
   `Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`, and
   `Tpatch-Base-Commit` trailers make ordinary commits carry replay identity.
6. **The best next metadata layer is a small first-party patch identity
   manifest.** It should record patch generation, patch hashes, capture bounds,
   touched paths, dependency snapshots, operation IDs, and optional keypoint /
   fingerprint manifests.

## 2. Current metadata inventory

| Surface | Current fields | Helps with | Gap |
|---|---|---|---|
| Feature directory | `request.md`, `analysis.md`, `spec.md`, `exploration.md`, `record.md` | Intent, rationale, file-level exploration. | Mostly prose; not normalized for machine identity. |
| `status.json` identity | `id`, `slug`, `title`, `state`, timestamps, `last_command` | Stable feature identity and lifecycle. | No explicit feature UUID/change-id separate from slug; slug rename/split semantics need care. |
| `status.apply` | `base_commit`, timestamps, `has_patch`, `has_recipe` | Replay baseline and artifact presence. | No capture mode, upper bound, pathspecs, patch SHA, patch-id, or generation number. |
| `status.depends_on` | `slug`, `kind`, `satisfied_by` | Hard/soft DAG and upstream-merged provenance. | No parent patch-generation/version pin in live schema. |
| `status.reconcile` | `upstream_ref`, `upstream_commit`, `outcome`, shadow fields, labels | Last reconcile verdict and parent overlays. | Not a full candidate/attempt history. |
| `status.verify` | `recipe_hash_at_verify`, `patch_hash_at_verify`, `parent_snapshot` | Freshness/staleness detection. | Snapshot stores parent states, not parent patch hashes/generations. |
| `apply-recipe.json` | `feature`, `operations[]`; op has `type`, `path`, `content/search/replace`, `created_by` | Deterministic operation-level apply; parent-created path hints. | No op ID, anchor, read/write set, content hash, structural descriptor, or parent version. |
| `artifacts/post-apply.patch` | canonical current full diff | Replay authority. | Overwritten in place; no intrinsic metadata beyond diff content. |
| `patches/NNN-*.patch` | audit snapshots | Historical full diffs. | Not canonical generations; no manifest ties them to feature state. |
| `apply-session.json` | slug, base commit, validation notes | Apply audit. | Not authoritative for identity/dependencies. |
| Future `land` trailers | `Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit` | Git-visible feature-to-commit mapping and patch/recipe hash anchors. | Not shipped yet; no structural anchors or dependency graph. |

## 3. Is the current metadata enough?

### Enough today

Current metadata is sufficient for:

- stable feature naming by slug;
- hard/soft dependency traversal;
- fail-fast apply when a hard parent is missing;
- topological reconcile ordering;
- upstream-merged parent provenance through `satisfied_by`;
- verify freshness across recipe/patch hash drift;
- exact patch collision detection via computed SHA-256;
- planned `git patch-id` upstream-detection by computing patch-id on demand.

### Not enough for the middle pass

Current metadata is not enough for:

- answering "which version of parent did this child depend on?";
- preserving prior canonical patch generations after reconcile/record rewrites;
- mapping a hunk to stable anchors across upstream movement;
- deciding whether two operations commute without re-parsing patches;
- comparing structural fingerprints without recomputing everything;
- creating a reproducible vector index without a source-text manifest;
- explaining why a search planner chose one order over another.

The missing data is not necessarily complex. Most of it can be generated at
record/verify time while tpatch still has the old base, new patch, and feature
intent in hand.

## 4. Candidate first-party metadata to carry forward

### 4.1 Patch generation identity

Purpose: preserve moving identity instead of overwriting it.

| Field | Meaning |
|---|---|
| `patch_generation` | Monotonic integer or content-addressed generation ID. |
| `patch_sha256` | SHA-256 of canonical `post-apply.patch` bytes. |
| `git_patch_id` | Git patch-id of the canonical patch. |
| `recipe_sha256` | SHA-256 of `apply-recipe.json` bytes or `none`. |
| `base_commit` | Lower bound used to produce the patch. |
| `upper_commit` | Upper bound for committed-range captures, or `working-tree`. |
| `capture_mode` | `working-tree`, `from`, `commit-range`, `auto`, `reconcile`. |
| `pathspecs` | User-supplied scoping paths, if any. |

Fit: a patch-generation manifest, not necessarily `status.json`, because
history should be append-only.

### 4.2 Dependency version pins

Purpose: make parent drift obvious without fuzzy inference.

| Field | Meaning |
|---|---|
| `parent_patch_sha256` | Parent canonical patch hash when the edge was declared or child was recorded. |
| `parent_patch_generation` | Parent generation ID if generation manifests exist. |
| `parent_recipe_sha256` | Parent recipe hash at child record time. |
| `satisfied_by` | Already exists: upstream commit that absorbed parent. |
| `satisfied_patch_id` | Optional patch-id of upstream absorption commit, when known. |

Fit: additive fields on `depends_on[]` or a separate dependency snapshot inside
the patch-generation manifest. The latter avoids mutating old edges when a child
is re-recorded.

### 4.3 Operation IDs and read/write sets

Purpose: enable commutation and ordering checks without reparsing prose.

| Field | Meaning |
|---|---|
| `op_id` | Stable operation ID within a recipe generation. |
| `reads` | Paths/symbols/anchors this operation expects. |
| `writes` | Paths/symbols/anchors this operation changes. |
| `anchor_ids` | Stable anchors/keypoints associated with the operation. |
| `created_by` | Already exists: parent slug that originated the target. |
| `depends_on_ops` | Optional intra-feature operation ordering constraints. |

Fit: future recipe schema change, guarded by ADR because apply-recipe JSON is a
shipped contract and strict JSON validation exists.

### 4.4 Structural anchor manifest

Purpose: make relocation easier before vector/RAG or provider fallback.

| Field | Meaning |
|---|---|
| `anchor_id` | Stable ID for this anchor within a patch generation. |
| `kind` | `function`, `class`, `import`, `route`, `config-key`, `literal`, `branch`, `loop`, `hunk-context`. |
| `path` | File path at record time. |
| `symbol` | Nearest symbol/name if any. |
| `line_range` | Original line range, advisory only. |
| `context_hash` | Hash of nearby normalized text. |
| `token_fingerprints` | Winnowed token fingerprints around the anchor. |
| `ast_fingerprint` | Optional parser-derived structural hash. |
| `confidence` | Extraction confidence, not identity confidence. |

Fit: `artifacts/anchors.json` or a generation-scoped manifest. Prefer
generation-scoped so anchors can be compared across rewrites.

### 4.5 Commutation/dependency relation manifest

Purpose: cache pairwise facts computed during record/verify/reconcile.

| Field | Meaning |
|---|---|
| `left_slug`, `right_slug` | Feature pair. |
| `left_generation`, `right_generation` | Patch versions the relation applies to. |
| `relation` | `commutes`, `depends_on`, `conflicts`, `unknown`. |
| `evidence` | `file-disjoint`, `hard-dep`, `shared-anchor`, `empirical-tree-hash`, etc. |
| `computed_at_base` | Commit SHA where the relation was evaluated. |

Fit: separate planner artifact, not status.json. Relations are base-dependent
and should expire when either generation or base changes.

### 4.6 Vector index manifest

Purpose: make embeddings reproducible and auditable.

- model name/version;
- chunking rules;
- source text hash;
- vector dimension/metric;
- index type/parameters;
- excluded paths;
- top-k retrieval results;
- commit SHA of indexed corpus.

Fit: manifest in git, raw vector index outside git by default. Vectors should
be treated as sensitive.

## 5. Recommended metadata layering

| Layer | Persistence | Why |
|---|---|---|
| `status.json` | Current truth only | Keep lifecycle, current apply/reconcile/verify summaries, current deps. Avoid turning it into history. |
| Patch-generation manifest | Append-only | Preserve moving patch identity, capture bounds, hashes, and dependency snapshots. |
| Anchor/fingerprint manifest | Generation-scoped | Anchors are only meaningful for the patch bytes they describe. |
| Relation/planner artifact | Base-scoped | Commutation/search results depend on target upstream base. |
| Vector manifest | Base/index-scoped | Embedding results depend on model, corpus, and index parameters. |
| Git trailers | Commit-scoped | Make feature-to-commit and patch hash visible in ordinary Git history. |

This split avoids overloading `status.json` while still letting tpatch-aware
repos carry rich identity metadata forward.

## 6. Priority recommendation

If the project chooses to design metadata before building fuzzy algorithms, the
recommended order is:

1. **Patch generation manifest** — implements ADR-015 D2 and gives every later
   algorithm stable moving identities.
2. **Dependency version snapshots** — lets children know exactly which parent
   generation they were built against.
3. **Operation IDs + read/write sets** — unlocks cheap commutation/order checks.
4. **Structural anchor manifest** — unlocks relocation and hybrid retrieval.
5. **Relation/planner artifacts** — cache base-dependent pairwise facts.
6. **Vector index manifest** — optional, sensitive, and model-dependent.

## 7. How this changes the fallback story

For tpatch-aware downstreams:

1. Read patch-generation metadata.
2. Check exact SHA / patch-id / parent generation snapshots.
3. Use declared read/write sets and dependency edges.
4. Use structural anchors/fingerprints.
5. Search uncertain clusters.
6. Use vector retrieval if enabled.
7. Escalate to provider/human.

For upstreams that do **not** adopt tpatch:

1. Fall back to Git commit history and `git patch-id`.
2. Extract text/keypoint/AST/vector evidence from source.
3. Search/relocate in shadow worktrees.
4. Escalate to provider/human when evidence is insufficient.

So yes: richer first-party metadata should be the happy path; the complicated
algorithms are the interoperability fallback.

## 8. Candidate follow-up docs

Names only:

| Candidate | Scope |
|---|---|
| `PRD-feature-patch-identity-metadata` | Patch generation manifest, patch SHA, patch-id, base/upper bounds, capture mode. |
| `ADR-patch-generation-manifest-boundary` | Lock what belongs in status vs append-only generation manifests. |
| `PRD-dependency-version-snapshots` | Parent patch/recipe generation snapshots on dependency edges. |
| `PRD-recipe-operation-identity` | Stable operation IDs, read/write sets, and anchor IDs. |
| `PRD-structural-anchor-manifest` | Keypoints, token fingerprints, optional AST fingerprints per patch generation. |
| `PRD-reconcile-commutation-graph` | Base-scoped pairwise relation artifact. |

## References

- [Feature Layout](../feature-layout.md)
- [Feature Dependencies](../dependencies.md)
- [ADR-015 prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md)
- [PRD-record-auto-base](../prds/PRD-record-auto-base.md)
- [PRD-tpatch-land](../prds/PRD-tpatch-land.md)
- [Patch identity and structural fingerprints](patch-identity-and-structural-fingerprints.md)

## Open questions

- Should the first patch-generation manifest live under each feature directory
  or under a repo-wide `.tpatch/patches/<slug>/` namespace as ADR-015 sketches?
- Should dependency version snapshots be stored directly on `depends_on[]`, or
  only in generation manifests to avoid rewriting old edges?
- Can operation read/write sets be inferred reliably from current recipe ops,
  or do agents need to emit them explicitly?
- Which anchors can be extracted with only stdlib/Git and still be useful?
- Should `git_patch_id` be persisted at record time or computed on demand?

## Disputes

None logged.
