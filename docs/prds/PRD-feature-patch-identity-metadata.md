# PRD - Feature Patch Identity Metadata - `feat-feature-patch-identity-metadata`

**Status**: Draft
**Date**: 2026-05-13
**Owner**: Core
**Byline**: T55
**Milestone**: Capture and metadata foundation. Not yet roadmap-committed.
**Depends on**: [PRD-feature-file-claims](./PRD-feature-file-claims.md), [PRD-record-capture-modes](./PRD-record-capture-modes.md). Implementation requires a future `ADR-patch-generation-manifest-boundary`.

## Related

- [Patch identity metadata research](../state-of-the-art/tpatch-metadata-for-patch-identity.md)
- [Patch identity and structural fingerprints](../state-of-the-art/patch-identity-and-structural-fingerprints.md)
- [Middle-pass synthesis](../state-of-the-art/tpatch-middle-pass-synthesis.md)
- [Experiment guide](../state-of-the-art/experiment-guide-structural-middle-pass.md)
- [Research roadmap](../state-of-the-art/research-roadmap.md)
- [WP-001 Feature Slice Gap](../whitepapers/WP-001-feature-slice-gap.md)
- [Feature Layout](../feature-layout.md)
- [Recording Patches](../record.md)
- [ADR-015 prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md)
- [PRD-record-auto-base](./PRD-record-auto-base.md)
- [PRD-record-collision-detection](./PRD-record-collision-detection.md)
- [PRD-reconcile-lock-guard](./PRD-reconcile-lock-guard.md)
- [PRD-tpatch-land](./PRD-tpatch-land.md)
- [PRD-patch-already-upstream-detector](./PRD-patch-already-upstream-detector.md)

## Cluster Position

This is PRD 3 of 4 in the capture-and-metadata foundation cluster:

1. `PRD-feature-file-claims`
2. `PRD-record-capture-modes`
3. `PRD-feature-patch-identity-metadata`
4. `PRD-feature-patch-amend`

See [research-roadmap.md](../state-of-the-art/research-roadmap.md) for the
cluster tracker. This PRD builds on the WP-001 / M17 boundary-capture outcomes:
auto-base, collision detection, lock guard, and `land` trailers. Supervisor
acceptance is still required before implementation.

## Implementation Gate

This PRD has the widest blast radius in the cluster. Before implementation, a
binding ADR must accept or revise these decisions:

- append-only generation manifest outside `status.json`;
- monotonic `generation` plus content-addressed `generation_id`;
- no wall-clock timestamps;
- no historical backfill from `patches/NNN-*.patch` in v1;
- dependency snapshots by parent generation/hash;
- `git patch-id --stable` as the only persisted patch-id algorithm.

## 0. Claims Audit

This PRD is a proposal. It changes nothing.

| Claim | Evidence |
|---|---|
| `artifacts/post-apply.patch` is the current canonical replay input and is overwritten by `record`. | `docs/feature-layout.md` "Canonical vs. audit trail". |
| Numbered files under `patches/` are historical full-diff snapshots, not replay input. | `docs/feature-layout.md` lines describing `patches/NNN-*.patch`. |
| `status.apply` currently stores `base_commit`, `has_patch`, and `has_recipe`, but not capture mode, patch SHA, patch-id, upper bound, generation, or claims. | `internal/store/types.go` `ApplySummary`. |
| `VerifyRecord` stores recipe and patch hashes at verify time, but not an append-only patch generation history. | `internal/store/types.go` `VerifyRecord`. |
| Dependency edges currently carry `slug`, `kind`, and `satisfied_by`, but no parent patch generation pin. | `internal/store/types.go` `Dependency`. |
| The phase-1.5 upstream detector can compute `git patch-id` on demand; a stored patch-id would be a cache/audit field, not a precondition. | `PRD-patch-already-upstream-detector.md`; M17 roadmap notes. |

No code, schema, command behavior, or asset text is changed by this PRD.

## Summary

Add append-only patch generation metadata so tpatch can answer:

- which canonical patch bytes represented this feature at each record/amend
  point?
- what capture mode produced those bytes?
- which claims, pathspecs, base commits, upper commits, dependencies, patch
  hashes, recipe hashes, and patch-ids were in force?
- which future structural anchor, commutation, or vector artifacts correspond to
  this exact patch generation?

Recommended v1 artifact:

```text
.tpatch/features/<slug>/artifacts/patch-generations.json
```

This file is an append-only manifest rewritten deterministically as a stable
JSON array. It does not replace `status.json`, `artifacts/post-apply.patch`,
`patches/NNN-*.patch`, or land trailers. It ties them together.

## 1. Problem Statement

tpatch has a current-truth model:

- `status.json` says where the feature is now;
- `artifacts/post-apply.patch` is the current patch;
- `patches/NNN-*.patch` preserve historical full-diff snapshots;
- `record.md` explains the latest record run;
- `land` trailers bind a Git commit to current patch/recipe/base hashes.

That is enough for today's lifecycle. It is not enough for future patch identity
and dependency reasoning:

- prior canonical patch generations are not machine-addressable;
- children cannot pin "I depended on parent generation 3";
- two record runs with the same bytes cannot be identified without rescanning;
- `git patch-id` has to be recomputed every time and is not tied to an audit
  event;
- capture mode and claim IDs are not normalized;
- structural anchors, vectors, and relation graphs have no generation-scoped
  parent object to reference.

The fix should be small and first-party. tpatch controls record, reconcile,
verify, land, feature dependencies, and future claims. It should carry enough
metadata forward so fuzzy structural algorithms are fallback tools, not the
first resort for tpatch-aware repositories.

## 2. Goals / Non-goals

### Goals

1. Add an append-only patch generation manifest per feature.
2. Record patch SHA-256 and `git patch-id` for each non-empty canonical patch
   generation.
3. Record recipe SHA-256 when an `apply-recipe.json` exists.
4. Record capture mode, base commit, upper bound, pathspecs, and claim IDs.
5. Record touched paths and file count for quick filtering.
6. Snapshot dependency versions when a generation is recorded.
7. Provide stable references for future anchor, fingerprint, commutation, and
   vector manifests.
8. Keep `status.json` focused on current lifecycle truth.
9. Keep raw vectors, prompts, transcripts, and source excerpts out of v1.

### Non-goals

1. No structural fingerprint extraction in this PRD.
2. No vector index creation in this PRD.
3. No operation-ID or read/write-set recipe schema change in this PRD.
4. No migration that rewrites historical `patches/` into full generations.
5. No change to `artifacts/post-apply.patch` as canonical replay input.
6. No change to land trailer schema.
7. No provider dependency.

## 3. Persisted Artifact

Recommended path:

```text
.tpatch/features/<slug>/artifacts/patch-generations.json
```

Top-level shape:

```json
{
  "version": 1,
  "feature": "model-picker",
  "current_generation": 3,
  "generations": [
    {
      "generation": 3,
      "generation_id": "pg_9cb6578d11c8",
      "kind": "record",
      "patch_sha256": "64hex...",
      "git_patch_id": "40hex-or-64hex...",
      "git_patch_id_algorithm": "git-patch-id-stable",
      "recipe_sha256": "64hex...",
      "canonical_patch": "artifacts/post-apply.patch",
      "audit_patch": "patches/003-record.patch",
      "base_commit": "full-sha",
      "upper": {
        "kind": "working-tree",
        "ref": "working-tree",
        "commit": ""
      },
      "capture": {
        "mode": "working-tree-all",
        "pathspecs": ["src/models/"],
        "claim_ids": ["8f31c0a19b2d"]
      },
      "touched_paths": ["src/models/provider.go"],
      "dependencies": [
        {
          "slug": "base-provider",
          "kind": "hard",
          "parent_generation": 2,
          "parent_patch_sha256": "64hex..."
        }
      ],
      "refs": {
        "anchors": "",
        "fingerprints": "",
        "relations": "",
        "vector_manifest": ""
      }
    }
  ]
}
```

The JSON should be stable:

- generations sorted by `generation`;
- arrays sorted unless command order is semantically important;
- no wall-clock timestamps in v1;
- no absolute paths;
- no raw source snippets;
- no raw prompts or transcripts.

## 4. Field Contract

### 4.1 Generation identity

| Field | Meaning |
|---|---|
| `generation` | Monotonic integer within a feature. Starts at 1 and preserves order. |
| `generation_id` | Stable content-derived ID, using a prefix plus a digest of feature, generation, patch SHA, recipe SHA, base, upper, and capture metadata. |
| `kind` | `record`, `amend-refresh`, `amend-fixup`, `reconcile`, `import`, or `manual-metadata`. |
| `patch_sha256` | SHA-256 of canonical patch bytes for this generation. |
| `git_patch_id` | Output of `git patch-id --stable` for deterministic duplicate detection. |
| `git_patch_id_algorithm` | Literal algorithm marker, v1 `git-patch-id-stable`. |
| `recipe_sha256` | SHA-256 of `apply-recipe.json`, or empty when absent. |
| `canonical_patch` | Current replay file for the latest generation. Historical entries point to the canonical path only when it still matches the generation. |
| `audit_patch` | Numbered patch file that preserves this generation's full diff when available. |

The ID model is both monotonic and content-addressed: `generation` is the
ordering primitive, while `generation_id` is the equivalence/audit primitive. If
a new capture would produce an existing `generation_id` with identical payload,
classify it as a no-op. If the same ID maps to different payload bytes, refuse as
a hash/schema collision rather than silently appending.

### 4.2 Capture bounds

| Field | Meaning |
|---|---|
| `base_commit` | Lower bound used to produce the patch. |
| `upper.kind` | `working-tree`, `index`, `commit`, `range`, or `reconcile-result`. |
| `upper.ref` | Human label such as `HEAD`, `working-tree`, or explicit `--to`. |
| `upper.commit` | Resolved commit when applicable; empty for working-tree-only captures. |
| `capture.mode` | Normalized mode from `PRD-record-capture-modes.md`. |
| `capture.pathspecs` | Normalized explicit `--files`, if any. |
| `capture.claim_ids` | Claims active in `--claimed-only`, if any. |

### 4.3 Dependency snapshot

Each generation should snapshot dependency versions:

| Field | Meaning |
|---|---|
| `slug` | Parent slug. |
| `kind` | `hard` or `soft`. |
| `satisfied_by` | Existing upstream absorption commit when present. |
| `satisfied_patch_id` | Optional patch-id of the upstream absorption commit when known. |
| `parent_generation` | Parent patch generation at child record time, when available. |
| `parent_patch_sha256` | Parent canonical patch hash at child record time. |
| `parent_recipe_sha256` | Parent recipe hash at child record time, when available. |

This does not replace `status.depends_on`; it records the exact parent version a
child was recorded against.

### 4.4 Future artifact refs

`refs` are empty in v1 but reserve generation-scoped targets:

| Ref | Future owner |
|---|---|
| `anchors` | `PRD-structural-anchor-manifest` |
| `fingerprints` | `PRD-structural-patch-fingerprints` |
| `relations` | `PRD-reconcile-commutation-graph` |
| `vector_manifest` | `PRD-patch-vector-index` |

The ref value should be a relative path under the feature directory or another
explicitly approved metadata location.

## 5. Command Interactions

### 5.1 `record`

Every successful non-empty `record` that changes canonical patch bytes should
append a generation entry.

Same-feature duplicate records with unchanged patch bytes must align with
`PRD-record-collision-detection.md` section 3.2: skip the numbered audit
snapshot and skip a patch-generation entry. A future metadata-revision PRD can
track no-byte-change capture metadata changes separately.

### 5.2 `record --regenerate-recipe`

If the patch bytes are unchanged but the recipe changes, do not append a patch
generation in v1. Existing recipe stale/verify mechanisms remain the source of
truth for recipe drift. A future recipe-revision PRD can add a nested recipe
history without overloading patch generations.

### 5.3 `reconcile`

If reconcile changes canonical patch bytes, append a `kind: "reconcile"`
generation. If reconcile only marks `upstream_merged`, do not append a patch
generation; the patch bytes did not change.

### 5.4 Phase-1.5 upstream detector

The detector may use stored `git_patch_id` as a cache when:

1. `patch_sha256` matches the live `artifacts/post-apply.patch`;
2. `git_patch_id` is non-empty;
3. the detector implementation still validates command semantics against the
   current upstream range.

Stored patch-id must not become an unchecked source of truth.

### 5.5 `land`

`land` trailers continue to carry commit-scoped identity. When a generation
manifest exists, `land` can validate that `Tpatch-Patch-SHA` and
`Tpatch-Recipe-SHA` match the current generation before committing.

### 5.6 `verify`

`verify` may read the current generation to avoid recomputing hashes, but must
fall back to live bytes and must detect drift when the manifest is missing or
stale.

## 6. Backwards Compatibility

Existing repositories have no patch generation manifest. That means "unknown
generation history", not corruption.

On first successful post-PRD `record`, tpatch can create generation 1 from the
live canonical patch and current metadata. It should not attempt to reconstruct
older generations from `patches/NNN-*.patch` unless a future migration PRD
defines the audit guarantees.

If the manifest is malformed, commands that rely on it should surface an error
or a clear warning according to the command's risk:

- `record` can refuse before writing more metadata;
- read-only status can warn;
- reconcile should not trust stale identity fields.

## 7. Implementation Notes

- Use existing SHA-256 helpers where available; otherwise centralize hashing.
- Compute `git_patch_id` through `git patch-id --stable`, not a custom patch-id
  implementation and not Git's unstable patch-id mode.
- Normalize path order from parsed patch files.
- Keep schema validation strict; unknown future fields should be either ignored
  only under a versioned extension policy or refused.
- Write the manifest atomically with the same style as other feature artifacts.
- Avoid storing timestamps unless an ADR explicitly approves nondeterministic
  metadata in this artifact.
- Do not store embeddings, raw source chunks, or transcript text here.

## 8. Acceptance Criteria

- First successful non-empty `record` creates `artifacts/patch-generations.json`.
- A changed re-record appends a new generation with a new generation number.
- Each generation records patch SHA-256, `git patch-id --stable`, the patch-id
  algorithm marker, base commit, upper bound, capture mode, touched paths, and
  any explicit pathspecs.
- Each generation has both a monotonic `generation` integer and a
  content-addressed `generation_id`.
- When claims are used, generation metadata records claim IDs.
- When dependencies exist, generation metadata snapshots parent slug, kind,
  satisfied_by, and available parent patch/recipe identity.
- Same-feature duplicate records with unchanged patch bytes skip both numbered
  audit snapshot and patch-generation append.
- Patch-unchanged recipe-only changes do not append patch generations in v1.
- Malformed manifests produce clear diagnostics.
- Existing repositories without the manifest continue to work.
- `status.json` remains current truth and is not turned into an append-only log.
- Tests cover generation creation, append, same-patch skip, recipe-only skip,
  dependency snapshot, claim IDs, malformed manifest, and no-manifest
  compatibility.

## 9. Open Questions

- Should historical numbered patches be backfilled into generation entries?
- Should malformed manifests refuse `record` by default?

## 10. Disputes

None logged.
