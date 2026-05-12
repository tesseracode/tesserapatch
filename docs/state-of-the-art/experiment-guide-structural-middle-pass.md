# Experiment Guide — Structural Middle-pass Case Study

**Status**: Snapshot guide (paper-only; no implementation authorized)
**Date**: 2026-05-10
**Owner**: Core
**Related**: [tpatch middle-pass synthesis](tpatch-middle-pass-synthesis.md),
[Patch identity and structural fingerprints](patch-identity-and-structural-fingerprints.md),
[Patch theory and commutation](patch-theory-and-commutation.md),
[Search-based patch application](search-based-patch-application.md),
[Reconcile Workflow](../reconcile.md),
[Feature Layout](../feature-layout.md)

## Why this guide exists

The research packet proposes a middle layer between deterministic patch replay
and provider/coding-agent reconciliation. This guide defines the first
case-study protocol: what to collect, how to represent it, and which metrics
decide whether the middle layer is worth turning into PRDs.

The guide intentionally allows both cheap stdlib/Git-only extraction and richer
optional extraction (AST, CFG, embeddings). The experiment should measure the
lift from each tier instead of assuming the most complex tier is necessary.

## Refresh triggers

- A real case study starts and finds missing fields.
- A PRD chooses a concrete fingerprint/vector/search implementation.
- The target stress repositories or ground-truth labels change.

## 1. Questions the experiment must answer

1. Can exact SHA and `git patch-id` detect enough duplicate/upstreamed patches
   to justify phase 1.5?
2. Can text/keypoint fingerprints relocate moved patch targets without AST or
   embeddings?
3. Do AST/CFG features materially improve relocation or duplicate detection?
4. Do vector embeddings improve top-k candidate discovery beyond symbolic
   filters?
5. Can pairwise commutation estimates predict safe patch order better than
   stable slug order?
6. Which evidence should reduce provider context, and which evidence is safe
   enough to avoid provider calls entirely?

## 2. Unit of study

Collect data at five levels:

| Unit | Meaning |
|---|---|
| Study | One repository plus one upstream transition or stress fixture. |
| Feature | One `.tpatch/features/<slug>/` directory and its canonical patch. |
| Patch hunk | One file/range edit inside `post-apply.patch`. |
| Code chunk | A function/class/module/config block in old base or new upstream. |
| Candidate attempt | One apply/relocation/order attempt in a scratch tree. |

## 3. Study selection

Prefer cases with known hard outcomes:

- exact cross-feature collisions;
- patches that upstream absorbed;
- patches that reapply cleanly;
- patches that fail direct apply but are manually relocatable;
- features with known dependencies;
- clusters where order matters.

For every study, record:

| Field | Required? | Notes |
|---|---|---|
| `study_id` | Yes | Kebab-case, e.g. `copilot-api-upstream-bump-2026-05`. |
| `repo_url` | Yes | Public URL or internal label. Do not store secrets. |
| `repo_kind` | Yes | Public, private, synthetic, fixture. |
| `old_base_commit` | Yes | Commit used when feature patches were recorded. |
| `new_upstream_commit` | Yes | Target commit for replay/reconcile. |
| `feature_slugs` | Yes | Slugs included in the experiment. |
| `known_outcome_source` | Yes | Manual review, prior bug bash, test fixture, or current reconcile verdict. |
| `test_command` | Optional | Command used for finalist validation. |
| `languages` | Optional | Detected or declared major languages. |
| `excluded_paths` | Yes | Vendored/generated/secrets/build-output paths excluded from extraction. |

## 4. Recommended output layout

For a future tracked case study, keep summaries in git and large/generated
artifacts outside git unless explicitly approved.

```text
docs/state-of-the-art/case-studies/<study-id>/
  README.md
  study.json
  features.jsonl
  patches.jsonl
  hunks.jsonl
  keypoints.jsonl
  fingerprints.jsonl
  retrieval-results.jsonl
  commutation.jsonl
  apply-attempts.jsonl
  metrics.json
  summary.md
```

If raw vectors are produced, store a manifest in git and keep the vector index
outside git:

```text
vectors.manifest.json
external/
  <study-id>.faiss       # not committed by default
  <study-id>.npy         # not committed by default
```

## 5. Core JSONL records

### 5.1 `features.jsonl`

One record per feature.

```json
{
  "study_id": "copilot-api-upstream-bump-2026-05",
  "slug": "dynamic-models",
  "state": "active",
  "base_commit": "abc123",
  "patch_sha256": "sha256:...",
  "git_patch_id": "...",
  "files_touched": ["src/models.ts"],
  "insertions": 42,
  "deletions": 8,
  "dependencies": [{"slug": "provider-core", "kind": "hard"}],
  "intent_paths": {
    "request": ".tpatch/features/dynamic-models/request.md",
    "spec": ".tpatch/features/dynamic-models/spec.md",
    "exploration": ".tpatch/features/dynamic-models/exploration.md"
  },
  "ground_truth": {
    "label": "reapplies_with_relocation",
    "source": "manual-review",
    "notes": "Function moved from models.ts to catalog.ts"
  }
}
```

### 5.2 `hunks.jsonl`

One record per patch hunk.

```json
{
  "study_id": "copilot-api-upstream-bump-2026-05",
  "slug": "dynamic-models",
  "hunk_id": "dynamic-models:h001",
  "old_path": "src/models.ts",
  "new_path": "src/models.ts",
  "old_range": {"start": 120, "lines": 14},
  "new_range": {"start": 120, "lines": 22},
  "context_before_hash": "sha256:...",
  "context_after_hash": "sha256:...",
  "removed_hash": "sha256:...",
  "added_hash": "sha256:...",
  "nearest_symbol": "listModels",
  "change_kind": ["insert-branch", "add-call"],
  "raw_patch_sha256": "sha256:..."
}
```

### 5.3 `keypoints.jsonl`

One record per salient code marker from old base, new upstream, or a patch.

```json
{
  "study_id": "copilot-api-upstream-bump-2026-05",
  "source": "new_upstream",
  "source_commit": "def456",
  "path": "packages/server/src/models/catalog.ts",
  "keypoint_id": "kp-000123",
  "kind": "function",
  "name": "listModels",
  "line": 88,
  "language": "typescript",
  "descriptor": {
    "tokens_hash": "sha256:...",
    "string_literals": ["models", "provider"],
    "calls": ["fetchModels", "translateModelId"],
    "branch_predicates": ["provider != nil"],
    "parent_kinds": ["module", "exported-function"],
    "nearby_kgrams": ["sha256:...", "sha256:..."]
  }
}
```

### 5.4 `fingerprints.jsonl`

One record per computed fingerprint. Use multiple records for different
methods so methods can be compared independently.

```json
{
  "study_id": "copilot-api-upstream-bump-2026-05",
  "subject_id": "dynamic-models:h001",
  "subject_kind": "hunk",
  "method": "token-winnowing",
  "parameters": {"k": 7, "window": 5, "normalization": "lowercase-space-comments"},
  "fingerprints": ["sha256:...", "sha256:..."],
  "fingerprint_count": 18
}
```

### 5.5 `retrieval-results.jsonl`

One record per query method/top-k result set.

```json
{
  "study_id": "copilot-api-upstream-bump-2026-05",
  "query_id": "dynamic-models:h001",
  "query_kind": "hunk",
  "method": "hybrid-symbolic-vector",
  "model": "codebert-base",
  "index": {"type": "hnsw", "metric": "cosine", "commit": "def456"},
  "top_k": [
    {
      "target_id": "kp-000123",
      "target_path": "packages/server/src/models/catalog.ts",
      "score": 0.91,
      "rank": 1,
      "symbolic_agreement": ["symbol", "string-literal", "call-neighborhood"]
    }
  ],
  "ground_truth_rank": 1
}
```

### 5.6 `commutation.jsonl`

One record per pairwise relation.

```json
{
  "study_id": "copilot-api-upstream-bump-2026-05",
  "left_slug": "provider-core",
  "right_slug": "dynamic-models",
  "relation": "depends_on",
  "evidence": ["hard-dependency", "right-edits-symbol-created-by-left"],
  "empirical": {
    "left_then_right": "applies",
    "right_then_left": "fails",
    "same_tree_hash": false
  }
}
```

### 5.7 `apply-attempts.jsonl`

One record per candidate replay/relocation/order attempt.

```json
{
  "study_id": "copilot-api-upstream-bump-2026-05",
  "attempt_id": "attempt-00042",
  "base_commit": "def456",
  "order": ["provider-core", "dynamic-models"],
  "relocations": [
    {
      "slug": "dynamic-models",
      "hunk_id": "dynamic-models:h001",
      "target_path": "packages/server/src/models/catalog.ts",
      "target_anchor": "listModels",
      "source": "hybrid-symbolic-vector",
      "confidence": 0.91
    }
  ],
  "result": "applies-clean",
  "tree_hash": "git-tree:...",
  "conflicts": [],
  "tests": {"ran": true, "passed": true, "command": "go test ./..."},
  "score_vector": {
    "strict_apply": 1,
    "conflicts": 0,
    "anchor_survival": 0.91,
    "patch_id_preservation": 0.74,
    "tests": 1,
    "churn": 12
  }
}
```

## 6. Extraction tiers

Run tiers cumulatively. Each tier should report its own metrics so the study
can answer whether extra complexity buys accuracy.

### Tier 0 — Git and patch metadata

Required for every study.

Collect:

- exact patch SHA-256;
- `git patch-id`;
- file list;
- insertions/deletions;
- hunk ranges and context hashes;
- status/dependency metadata;
- direct apply and 3-way apply results.

### Tier 1 — Text keypoints

Stdlib/Git-friendly. No parser required.

Extract:

- path and extension;
- import/export lines;
- function-like/class-like lines by regex;
- route/config keys;
- string literals;
- error messages;
- branch predicates (`if`, `switch`, `case`, ternary);
- loop headers;
- nearby token k-grams;
- rare lines using patience/histogram-style frequency.

### Tier 2 — Token fingerprints

Normalize:

- whitespace;
- comments, when safe;
- string-literal content either preserved or hashed;
- identifiers optionally normalized for a separate variant.

Compute:

- k-grams for `k in {5, 7, 9}`;
- winnowed fingerprints with multiple windows;
- Jaccard overlap by hunk/function/file.

### Tier 3 — AST features

Optional, parser-dependent.

Collect:

- syntax node kind histograms;
- subtree hashes;
- nearest symbol/function/class;
- call names;
- parameter counts;
- import/export graph;
- AST edit script if a tool such as GumTree is used;
- move/rename signals.

### Tier 4 — CFG/PDG features

Optional and likely language-specific.

Collect:

- basic block count;
- branch/loop count;
- call graph edges;
- data-flow edges when available;
- normalized control-flow shape hash.

### Tier 5 — Vector embeddings / RAG retrieval

Optional but important to test.

Collect:

- chunking method: hunk, function, class, file window, intent doc section;
- embedding model and version;
- tokenizer and max token length;
- vector dimension;
- distance metric;
- index type and parameters;
- source text hash for each embedded chunk;
- top-k retrieval results for each query;
- whether retrieval was sparse-only, dense-only, or hybrid.

Do not treat vector results as ground truth. Measure whether they improve
candidate ranking and provider-context reduction.

## 7. Metrics

### Duplicate/upstreamed detection

- precision/recall for exact SHA;
- precision/recall for `git patch-id`;
- precision/recall for token/AST/vector similarity thresholds;
- false positive list with root cause.

### Anchor relocation

- top-1 accuracy;
- top-3 / top-5 recall;
- mean reciprocal rank (MRR);
- relocation apply success rate;
- conflict reduction versus direct apply.

### Commutation/order prediction

- pairwise relation accuracy;
- topological-order success rate;
- search attempts needed;
- best score by algorithm;
- provider/human escalations avoided.

### Vector retrieval lift

Compare:

1. symbolic-only retrieval;
2. dense-only retrieval;
3. hybrid symbolic+dense retrieval;
4. hybrid plus AST/CFG reranking.

Report top-k recall, MRR, false positives, runtime, and index size.

### End-to-end planner value

- features reapplied without provider;
- features correctly escalated;
- provider context size reduction;
- tests passed after accepted candidate;
- manual review time saved, if measurable.

## 8. Ground-truth labels

Allowed labels:

| Label | Meaning |
|---|---|
| `exact_duplicate` | Same canonical patch bytes as another feature. |
| `patch_id_duplicate` | Same `git patch-id`, bytes differ. |
| `upstream_absorbed` | Upstream independently contains the feature. |
| `reapplies_clean` | Direct apply succeeds on new upstream. |
| `reapplies_with_relocation` | Apply succeeds after anchor relocation. |
| `order_dependent` | At least one patch order succeeds and another fails/differs. |
| `true_conflict` | Needs human/provider resolution. |
| `not_same` | Similar-looking candidate is not the same feature. |
| `unknown` | No reliable label yet. |

Every non-`unknown` label needs a source: manual review, test fixture, existing
reconcile verdict, upstream commit evidence, or accepted resolution.

## 9. Safety and privacy

- Do not collect secrets.
- Exclude `.env`, credentials, build outputs, vendored directories, generated
  blobs, and any path restricted by policy.
- Treat embeddings as sensitive unless proven otherwise; they can leak source
  information through nearest-neighbor attacks or model inversion.
- Commit summaries, manifests, hashes, and metrics by default. Commit raw code
  excerpts or vector indexes only when the repository owner explicitly allows
  it.
- Record excluded paths so metrics are interpretable.

## 10. Minimum viable study

The smallest useful study includes:

1. `study.json`
2. `features.jsonl`
3. `hunks.jsonl`
4. Tier 0 patch metadata
5. Tier 1 text keypoints
6. direct apply / 3-way apply attempts
7. manual ground-truth labels for at least:
   - one exact duplicate or known non-duplicate,
   - one clean reapply,
   - one relocation case,
   - one conflict or order-dependent case.
8. `summary.md` answering:
   - what worked with stdlib/Git only;
   - what failed;
   - which optional tier should be tested next.

## 11. Exit criteria for opening a PRD

Open a PRD only if the case study shows at least one of:

- `git patch-id` catches meaningful upstream absorption with near-zero false
  positives;
- text/keypoint relocation improves apply success over direct `git apply`;
- AST/CFG features materially improve top-k relocation over text keypoints;
- vector retrieval materially improves candidate ranking or reduces provider
  context;
- search over uncertain clusters beats stable slug order on order-dependent
  cases.

If none of those hold, close the research as informative but do not build the
middle pass yet.

## References

- [Patch identity and structural fingerprints](patch-identity-and-structural-fingerprints.md)
- [Search-based patch application](search-based-patch-application.md)
- [tpatch middle-pass synthesis](tpatch-middle-pass-synthesis.md)

## Open questions

- Which stress repositories should be the first official studies?
- Should raw case-study data live in this repo, an external artifact store, or
  session-local files until summarized?
- Which embedding model is acceptable for a local/offline experiment?
- Should AST extraction use external tools during research even if core tpatch
  stays zero-dep?
- Who provides ground-truth labels for ambiguous semantic cases?

## Disputes

None logged.
