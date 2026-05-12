# Patch Identity and Structural Fingerprints

**Status**: Snapshot research (paper-only; no implementation authorized)
**Date**: 2026-05-10
**Owner**: Core
**Related**: [PRD-patch-already-upstream-detector](../prds/PRD-patch-already-upstream-detector.md),
[PRD-record-collision-detection](../prds/PRD-record-collision-detection.md),
[Feature Layout](../feature-layout.md),
[Reconcile Workflow](../reconcile.md),
[ADR-015 prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md)

## Why this doc exists

tpatch already has two identity extremes:

- exact patch bytes, useful for record-time collision detection;
- provider-semantic judgment, useful when upstream rewrites the same intent in a
  different way.

This doc researches the middle: algorithms that can say "these patches or code
regions are probably the same" using text, tokens, syntax trees, graph
structure, vector retrieval, and computer-vision-style feature matching.

## Refresh triggers

- A PRD proposes fuzzy duplicate detection, structural upstream-merged
  detection, or anchor relocation.
- tpatch adds a parser abstraction, a tree-sitter integration, or a language
  server based reconcile phase.
- Binary/reverse-engineering similarity tooling becomes relevant to supported
  target repositories.

## 1. Executive findings

1. **Patch identity is a ladder, not a boolean.** Exact bytes, `git patch-id`,
   token fingerprints, AST edit scripts, CFG/PDG similarity, tests, and
   provider judgment answer different identity questions.
2. **Git already has a strong middle primitive.** `git patch-id` ignores line
   numbers and is documented as "reasonably stable" and useful for likely
   duplicate commits. It is the right deterministic baseline for upstream
   cherry-pick detection.
3. **Diff algorithms are anchor choices.** Myers optimizes edit length;
   patience and histogram find unique/rare lines that humans recognize as
   anchors. That same idea can feed patch relocation.
4. **Computer vision maps surprisingly well.** Detect salient code keypoints,
   describe their local structure, match descriptors across old/new upstream,
   reject outliers, then relocate patch hunks by the surviving transform.
5. **Reverse engineering adds a graph/vector lesson.** Binary diffing and
   decompiler tooling compare functions using control-flow structure,
   constants, calls, and vector signatures. Source patch identity can borrow
   the same multi-feature scoring without using an LLM.
6. **Vector retrieval / RAG is a distinct middle layer.** Dense embeddings can
   retrieve likely-equivalent patches, hunks, functions, or prior resolutions.
   Retrieval alone can stay below the provider tier; RAG that asks a generator
   to reason over retrieved context belongs at the boundary between phase 2.5
   evidence gathering and phase 3 provider-semantic analysis.

## 2. Identity ladder

| Level | Name | Answers | Good for | Not enough for |
|---|---|---|---|---|
| 0 | Byte identity | Are the patch files identical? | Current collision PRD; same-feature dedup. | Cherry-picks with context drift or whitespace. |
| 1 | Normalized patch identity | Are the textual changes likely the same? | `git patch-id`; upstream cherry-picks. | Refactors or semantic reimplementations. |
| 2 | Token fingerprint | Do the same token windows recur? | Local duplicate regions; moved blocks. | Deep structural equivalence. |
| 3 | AST edit identity | Are the same syntax nodes inserted/deleted/updated/moved? | Refactors, moved functions, renamed variables. | Dynamic behavior or generated code. |
| 4 | CFG/PDG identity | Are control/data-flow shapes similar? | Function-level structural matching. | Expensive and language/tool dependent. |
| 5 | Vector retrieval identity | Do learned embeddings retrieve the same or nearest-equivalent patch/code region? | Candidate discovery, prior-resolution lookup, provider-context reduction. | Opaque scores, model drift, privacy, false positives. |
| 6 | Behavioral identity | Do tests/properties remain equivalent? | High confidence for a feature. | Incomplete tests; slow. |
| 7 | Intent identity | Does the feature intent remain satisfied? | Provider/human semantic decisions. | Determinism and cost. |

tpatch should use lower levels as conservative fast paths and higher levels as
confidence signals. Only exact and patch-id style matches should auto-produce
strong verdicts without user confirmation.

## 3. Diff algorithms as identity primitives

### Myers

Myers' O(ND) algorithm computes a shortest edit script between two sequences.
It is foundational for text diffs and is the default family behind many diff
tools. Strength: compact minimal edits. Weakness: minimal edit scripts are not
always the most stable anchors for code that moved or reordered blocks.

### Git diff variants

Git exposes several diff algorithms:

- `myers`: default greedy diff algorithm.
- `minimal`: spends extra time to minimize diff.
- `patience`: uses unique lines as anchors.
- `histogram`: extends patience to low-occurrence common elements.
- `anchored`: asks Git to preserve caller-specified anchor lines.

For tpatch, the important framing is that diff algorithms choose anchors and
therefore shape patch identity. A hunk generated by Myers may fail after a
refactor where a patience/histogram anchor could still find the region.

### `git patch-id`

Git documents patch-id as a sum of SHA-1s of file diffs with line numbers
ignored. It is "reasonably stable" and "reasonably unique", and its main use
case is likely duplicate commits. This directly supports the
patch-already-upstream detector PRD.

### Token and document fingerprints

Winnowing (used in plagiarism detection systems such as MOSS) hashes token
k-grams and keeps selected fingerprints from sliding windows. It is robust to
local insertions/deletions because matches are local rather than global.

For tpatch:

- tokenize code or config text;
- normalize whitespace/comments where safe;
- compute k-gram fingerprints;
- compare fingerprint overlap by file/function;
- use matches as weak anchors, not authoritative identity.

## 4. Syntax-aware differencing

AST differencing tools compare syntax trees rather than lines. GumTree is the
main current reference: it advertises syntax-aware diffs across multiple
languages and cites its ASE 2014 paper "Fine-grained and accurate source code
differencing", plus newer work on hyperparameter optimization and scalable
source differencing.

Useful AST identity concepts:

- **Node kind sequence**: e.g. function -> if -> call -> return.
- **Subtree hash**: normalized hash of an AST region.
- **Edit script**: insert/delete/update/move nodes.
- **Move detection**: a moved function can remain "same" even when line
  anchors fail.
- **Rename tolerance**: variable and parameter renames can be normalized when
  local binding structure is unchanged.

Potential tpatch signature:

```text
structural_patch_id =
  hash(language, file_role, sorted(normalized_ast_edits), stable_symbol_edges)
```

This should start as a confidence score, not as an upstream-merged verdict.

## 5. Computer-vision analogy

Computer vision feature matching typically works as:

1. detect keypoints (corners, blobs, lines, distinctive regions);
2. describe each keypoint with a local descriptor;
3. match descriptors across images;
4. reject ambiguous matches using ratio tests;
5. reject geometrically inconsistent matches using RANSAC;
6. estimate a transform between images.

For code:

| Vision concept | Code analogue |
|---|---|
| Keypoint | Function signature, exported symbol, uncommon string, error message, branch predicate, loop shape, type declaration, route name, config key. |
| Descriptor | Normalized AST neighborhood, token k-grams around the point, call names, constants, parent/child syntax kinds, local CFG shape. |
| Descriptor distance | Jaccard/cosine/Hamming/tree-edit distance over normalized features. |
| Ratio test | Accept a match only if the best target is much better than the second-best target. |
| RANSAC | Keep only matches consistent with a file/AST/order transform; reject outliers. |
| Homography/transform | File rename, line offset, function move, wrapper insertion, module split. |

This gives tpatch a non-LLM relocation strategy:

```text
old patch anchors -> keypoint descriptors -> match in new upstream
                  -> reject outliers -> estimate relocation
                  -> attempt relocated hunk/operation apply
```

The biggest value is not "prove patches are identical"; it is "find where the
old patch probably belongs now".

## 6. Source and binary similarity

Reverse-engineering and malware-analysis tools compare compiled code even when
bytes differ:

- **CFG matching** compares basic blocks and control-flow edges.
- **Call graph/context matching** uses who calls whom and imported APIs.
- **Constants/strings matching** uses stable literals as anchors.
- **Vector signatures** summarize function properties for scalable lookup.
- **Decompiler IR** gives a normalized representation above raw opcodes.

Ghidra is a broad SRE framework with disassembly, decompilation, graphing, and
scripting. BSim-style binary similarity uses the vector-signature idea: encode
function structure into a searchable signature and rank candidates.

For tpatch source patches, the source-level analogue is:

```text
function_signature_vector = [
  syntax node histogram,
  normalized call names,
  literal hashes,
  branch/loop counts,
  exported/imported symbol edges,
  token fingerprints
]
```

This can support language-agnostic "same function, moved or slightly edited"
matching without a provider.

## 7. Vector embeddings and RAG retrieval

Vector retrieval sits above symbolic/keypoint algorithms and below full
provider reasoning.

The important distinction:

- **Vector retrieval without generation** is a phase-2.5 evidence mechanism.
  It embeds patches, hunks, functions, keypoint descriptors, and intent docs,
  then retrieves nearest neighbors from the current upstream or historical
  feature store.
- **RAG with generation** crosses into provider territory. Retrieval still
  reduces cost and improves grounding, but once a generator interprets the
  retrieved material, the verdict is no longer a non-LLM algorithmic result.

For tpatch, useful vectorized objects include:

| Object | Example input | Retrieval use |
|---|---|---|
| Feature intent | `request.md`, `spec.md`, `exploration.md` summary. | Find prior features or upstream commits with similar intent. |
| Patch hunk | Normalized hunk header, removed/added lines, nearby context. | Find where a hunk moved or whether upstream absorbed a similar edit. |
| Code chunk | Function/class/module chunk from old base and new upstream. | Match moved/rewritten code regions. |
| Keypoint descriptor | Symbol name, token k-grams, literals, AST neighborhood. | Hybrid search with symbolic anchors. |
| Resolution artifact | Prior conflict, accepted resolution, validation result. | Retrieve examples for future provider/human resolution. |

The best first design is **hybrid retrieval**:

1. sparse/symbolic filters first: path, language, symbol, string literal,
   patch-id, token fingerprint;
2. dense vector retrieval second: embeddings over patch/code/intent chunks;
3. structural consistency check third: matched chunks must agree with file,
   symbol, order, or keypoint constraints;
4. provider prompt grounding last, only if an automatic decision is still
   unsafe.

This avoids treating opaque vector distance as truth. Dense retrieval should
return candidates and evidence, not final lifecycle verdicts.

### Fit in the tpatch model

Vector retrieval can support three middle-pass jobs:

1. **Candidate discovery.** "This hunk no longer applies; which current
   upstream chunks look nearest to the old target?"
2. **Prior-art lookup.** "Have we resolved a similar patch conflict before?"
3. **Provider context reduction.** "If phase 3 is needed, send the top-k
   retrieved chunks/resolutions instead of the whole repo."

It should not, by itself:

- mark a feature `upstream_merged`;
- drop a patch;
- rewrite a patch in the real tree;
- claim semantic equivalence.

### Required audit metadata

Any vector/RAG experiment or future implementation should persist:

- embedding model name and version;
- tokenizer/chunking strategy;
- vector dimension and distance metric;
- normalization rules;
- corpus boundaries and excluded paths;
- index type and parameters, e.g. exact scan vs HNSW/ANN;
- top-k retrieved IDs and scores;
- hash of the source text used to produce each vector;
- whether generation was used after retrieval.

This is mandatory because embedding scores are model-relative. A score from one
model/index is not comparable to a score from another.

### Risks

- **False confidence.** Nearest vectors can be "near" for irrelevant reasons.
- **Embedding drift.** Model upgrades can change neighbors and thresholds.
- **Privacy leakage.** Vectors and retrieved snippets may leak source code or
  secrets; treat them as sensitive artifacts unless proven otherwise.
- **Overlapping with provider tier.** RAG is not provider-free if a generator
  makes the final judgment.
- **Index staleness.** Upstream movement invalidates stale vectors unless they
  are tied to a commit SHA and regenerated or versioned.

## 8. Proposed tpatch confidence classes

| Class | Evidence | Suggested behavior |
|---|---|---|
| `exact_patch_match` | Same SHA-256 bytes. | Safe for collision refusal/same-feature dedup. |
| `patch_id_match` | Same `git patch-id`. | Safe candidate for deterministic upstream-merged fast path. |
| `structural_match_high` | Strong AST/token/keypoint match plus consistent relocation. | Use to attempt shadow apply or reduce provider context; do not auto-drop. |
| `structural_match_low` | Some anchors match but ambiguity remains. | Use for diagnostics and provider prompt grounding. |
| `vector_match_high` | Dense retrieval top-k agrees with symbolic/structural filters and has a clear margin over alternatives. | Candidate discovery and provider-context reduction; still not auto-drop. |
| `vector_match_low` | Dense retrieval finds plausible but ambiguous neighbors. | Diagnostics only. |
| `behavioral_match` | Tests pass and feature-specific assertions pass after candidate apply. | Strong but still not proof of upstream absorption. |
| `intent_match` | Provider/human says intent is satisfied. | Existing semantic tier. |

## 9. Risks

- **False positives are expensive.** Misidentifying two different patches as
  the same can retire or reorder the wrong feature.
- **Language support can sprawl.** AST/CFG/PDG matching is only as good as the
  parser and normalization per language.
- **Generated code breaks assumptions.** Large generated files often have many
  repeated anchors and weak structural meaning.
- **Similarity thresholds rot.** Scores need calibration on real tpatch
  case-study repos, not generic academic benchmarks.
- **Structural identity is not semantic identity.** Similar code can implement
  different behavior; different code can implement the same behavior.
- **Vector identity is not proof.** Embeddings are useful for retrieval and
  ranking, not for irreversible lifecycle transitions.

## 10. References

- Git `patch-id` documentation:
  https://git-scm.com/docs/git-patch-id
- Git `diff` documentation, diff algorithms and moved-line detection:
  https://git-scm.com/docs/git-diff
- Eugene W. Myers, "An O(ND) Difference Algorithm and Its Variations",
  Algorithmica, 1986.
- Jean-Remy Falleri et al., "Fine-grained and accurate source code
  differencing", ASE 2014. GumTree project:
  https://github.com/GumTreeDiff/gumtree
- Saul Schleimer, Daniel S. Wilkerson, Alex Aiken, "Winnowing: Local
  Algorithms for Document Fingerprinting", SIGMOD 2003.
- Lian Li et al., "Deckard: Scalable and Accurate Tree-Based Detection of Code
  Clones", ICSE 2007.
- David G. Lowe, "Distinctive Image Features from Scale-Invariant Keypoints",
  IJCV 2004.
- Martin A. Fischler and Robert C. Bolles, "Random Sample Consensus", CACM
  1981.
- Z. Feng et al., "CodeBERT: A Pre-Trained Model for Programming and Natural
  Languages", EMNLP Findings 2020.
- D. Guo et al., "GraphCodeBERT: Pre-training Code Representations with Data
  Flow", ICLR 2021.
- Jeff Johnson, Matthijs Douze, Herve Jegou, "Billion-scale similarity search
  with GPUs", IEEE Transactions on Big Data, 2019. FAISS:
  https://github.com/facebookresearch/faiss
- Ghidra project overview, disassembly/decompilation/graphing/scripting:
  https://ghidra-sre.org/

## Open questions

- What target languages matter first for structural matching in tpatch's real
  user repos?
- Should structural fingerprints be stored per feature at record time, or
  generated lazily during reconcile?
- Which confidence class is allowed to short-circuit provider calls?
- Can keypoint extraction be useful with only stdlib and Git, or does useful
  AST matching require optional external tools?
- How should generated files and vendored code be excluded from fingerprints?
- Are local embedding models good enough for patch retrieval, or does useful
  vector search require a hosted model?
- Should vector indexes be committed, regenerated, or stored outside the repo?

## Disputes

None logged.
