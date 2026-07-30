# ADR-029 — Write-file Recipe Safety

**Status**: Accepted
**Date**: 2026-07-29 (Proposed) / 2026-07-29 (Accepted — v0.12.0 Wave β implementation)
**Owner**: Core
**Cluster**: Replay trustworthiness (GH #1)
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-024](./ADR-024-patch-generation-manifest-boundary.md), [ADR-025](./ADR-025-reconcile-evidence-and-revision-schema.md), [ADR-026](./ADR-026-patch-amendment-policy.md)
**Blocks**: [PRD-write-file-recipe-safety](../prds/PRD-write-file-recipe-safety.md) implementation

## Context

GH #1 reports that older `write-file` recipes can silently restore whole-file snapshots and remove later fixes. `PRD-write-file-recipe-safety` classifies five possible safeguards and selects two for v1: preimage hash preconditions and later-touch detection.

The current recipe schema is operation-based and strict. `ApplyRecipe` contains `operations[]`, and `RecipeOperation` supports `write-file`, `replace-in-file`, `append-file`, and `ensure-directory` with optional `created_by` (`internal/workflow/implement.go:42-108`). ADR-024 and ADR-025 provide the content-addressed schema precedent: hashes and canonical JSON are used for durable identity and drift detection rather than prose or timestamps.

This ADR is paper-only. It changes no code, schema, CLI behavior, assets, PRD body, handoff, release artifact, or CHANGELOG entry.

## Decision

### D1 — `write-file` operations carry `preimage_hash`

Future `write-file` operations include a `preimage_hash` field:

```json
{
  "type": "write-file",
  "path": "relative/path",
  "preimage_hash": "sha256:<64 lowercase hex>",
  "content": "new file bytes"
}
```

For new-file writes, the emitted value is `""`, meaning the target path must not exist at apply time. Missing `preimage_hash` is reserved for legacy recipes and accepted in v1 with a warning.

### D2 — Hash input is exact file bytes

`preimage_hash` is SHA-256 over exact preimage bytes, displayed as `sha256:<64 lowercase hex>`. Writers do not normalize line endings, encodings, permissions, or JSON string escaping before hashing.

**Rationale**. Whole-file overwrite safety is byte-level. Any normalization would let an overwrite proceed against bytes different from the captured preimage.

### D3 — Apply prechecks are all-or-nothing

Before executing a recipe, apply checks every `write-file` precondition. If any precondition fails, no operation from that recipe is written.

Refusal cases:

- expected hash present, file missing;
- expected hash present, file hash differs;
- empty preimage, file exists;
- unreadable target needed for a precondition.

### D4 — Legacy missing-preimage recipes warn in v1

A v1 reader accepts existing `write-file` recipes that lack `preimage_hash`, emits a warning, and applies using current behavior. Verify reports a warning recommending regeneration. Promoting missing preimages to refusal requires a future migration PRD or major-version policy.

**Rationale**. Strict immediate refusal would break existing feature histories. Warning preserves compatibility while making the risk visible.

### D5 — Later-touch detection is mandatory and path-level in v1

Record, reconcile, and verify must detect when a later active/effective feature touches a path covered by an older `write-file` operation. v1 detection is path-level, not hunk-level and not a total stack-safety proof.

- `record`: warns when the new feature touches a path already whole-file-owned by an older active feature.
- `reconcile`: warns when later active/effective features touched paths owned by older `write-file` operations.
- `verify`: stale preimages on effective features fail; stale preimages on superseded historical features warn.

### D6 — Later-touch record behavior warns, not refuses

Record-time later-touch detection is warning-class in v1. Apply-time preimage mismatch is refusal-class.

**Rationale**. Some later touches are intentional replacements or pending supersession edges. Warning avoids blocking legitimate authoring while the preimage gate prevents unsafe execution.

### D7 — Supersession controls severity for historical features

If ADR-028/PRD-feature-supersession marks an older feature as superseded by an active healthy replacement, stale `write-file` preimages on the older feature are warning-class audit signals and do not fail default effective replay. The active superseder remains subject to normal refusal/failure semantics.

### D8 — No source bodies in diagnostics

Diagnostics may include feature slug, operation index, repo-relative path, expected hash, observed hash, missing/existing state, and reason code. They must not include file contents.

**Rationale**. Hashes and paths are enough to explain drift while avoiding unnecessary source-body duplication in metadata or logs.

## Consequences

- `preimage_hash` becomes a binding recipe schema field for future generated `write-file` operations.
- The implementation slice must update strict recipe decoding, skill examples, parity tests, and apply precheck ordering in one change set.
- Legacy recipes remain runnable but noisy.
- The recipe is no longer allowed to silently overwrite a later-touched effective file when its preimage differs.

## Alternatives considered

1. **Prefer contextual operations first** — deferred. It is a generation-policy question and does not itself protect existing whole-file recipes.
2. **Cross-feature stack validation first** — deferred. It is heavier and should build on preimage and later-touch signals.
3. **Immediate refusal for legacy missing preimages** — rejected for v1 compatibility.
4. **Sidecar preimage manifest** — rejected. The precondition belongs to the operation that performs the overwrite.
5. **Hash normalized text** — rejected. Replay safety is byte-exact.

## References

- GH #1 — `write-file` recipes can silently revert later fixes.
- `copilot-api/.tpatch/RETROSPECTIVE.md` Part 2 — external empirical evidence, cited by reference.
- [PRD-write-file-recipe-safety](../prds/PRD-write-file-recipe-safety.md).
- [PRD-feature-supersession](../prds/PRD-feature-supersession.md).
- [ADR-024 — Patch generation manifest boundary](./ADR-024-patch-generation-manifest-boundary.md).
- [ADR-025 — Reconcile evidence and revision schema](./ADR-025-reconcile-evidence-and-revision-schema.md).
- [ADR-026 — Patch amendment policy](./ADR-026-patch-amendment-policy.md).
- `internal/workflow/implement.go:42-108`.
