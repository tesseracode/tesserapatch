# PRD - Write-file Recipe Safety - `write-file-recipe-safety`

**Status**: Proposed
**Date**: 2026-07-29
**Owner**: Core
**Cluster**: Replay trustworthiness (GH #1)
**Depends on**: [ADR-024 — Patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md), [ADR-025 — Reconcile evidence and revision schema](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md), [ADR-026 — Patch amendment policy](../adrs/ADR-026-patch-amendment-policy.md)
**Blocks**: implementation of safe default replay with supersession from [PRD-feature-supersession](./PRD-feature-supersession.md)

## Related

- GH #1 — supersession and `write-file` replay safety.
- `copilot-api/.tpatch/RETROSPECTIVE.md` Part 2 — empirical evidence that older whole-file recipes can revert later fixes. This PRD cites the external document by reference and does not copy its contents.
- [PRD-feature-supersession](./PRD-feature-supersession.md) for effective/historical feature semantics.
- [ADR-024 — Patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md) for content-addressed manifest precedent.
- [ADR-025 — Reconcile evidence and revision schema](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md) for strict schema and content-addressed ID precedent.
- [ADR-026 — Patch amendment policy](../adrs/ADR-026-patch-amendment-policy.md) for recipe/patch amendment boundaries.
- Optional companion ADR: [ADR-029 — Write-file recipe safety](../adrs/ADR-029-write-file-recipe-safety.md).

## 0. Meta

### 0.1 Paper-only status

This PRD is **Proposed**. It changes no code, schema, CLI behavior, shipped assets, CHANGELOG entry, or release artifact. It defines a future implementation contract.

### 0.2 Claims audit

| Claim | Evidence |
|---|---|
| `ApplyRecipe` contains top-level `feature` and `operations`. | `internal/workflow/implement.go:42-46`. |
| `RecipeOperation` currently supports `type`, `path`, `content`, `search`, `replace`, and optional `created_by`. | `internal/workflow/implement.go:48-76`. |
| Current operation types are `write-file`, `replace-in-file`, `append-file`, and `ensure-directory`. | `internal/workflow/implement.go:98-100`. |
| The recipe decoder rejects unknown fields. | `internal/workflow/implement.go:81-85`. |
| Patch-generation manifests use content-addressed SHA-256-derived identity and strict schema reads. | ADR-024 D2 and D9; `internal/store/patch_generations.go:30-55, 90-114`. |
| Reconcile evidence uses content-addressed `re_<12hex>` IDs and strict per-line schema. | ADR-025 D2-D3. |
| Current top-level commands referenced here (`record`, `reconcile`, `verify`, `apply`, `status`) exist. | `internal/cli/cobra.go` command registration and `Use:` strings. |

### 0.3 Safeguard classification

GH #1 requested five safeguards. This PRD locks the v1 split:

| Safeguard | v1 decision | Rationale |
|---|---|---|
| 1. Prefer contextual operations | Deferred | It is a generation-policy decision: when is whole-file ownership legitimate? Needs a dedicated policy PRD. |
| 2. Preimage hash preconditions | Mandatory | Directly prevents silent overwrite when the current file differs from the recipe's expected preimage. |
| 3. Later-touch detection | Mandatory | Makes cross-feature path ownership drift visible during record, reconcile, and verify. |
| 4. Cross-feature recipe validation | Deferred | Heavier orchestration over an ordered stack; useful later but not required to block the known silent-revert class. |
| 5. Regeneration guidance | Deferred | UX polish should follow once the refusal/warning signals are reliable. |

## 1. Problem statement

A `write-file` operation is a whole-file snapshot. It is safe when creating a new file or intentionally replacing a file whose previous bytes are known. It is unsafe when replayed over a file that later features have already fixed: the older recipe can restore its old snapshot and silently remove later corrections.

GH #1 reports an external measured case in `copilot-api` where an older catalog-aware effort feature had `write-file` operations for files later touched by streaming abort fixes. The authoritative patch still described the older feature's intent, but the whole-file recipe had become unsafe as a replay mechanism. The future CLI must refuse or warn before such an overwrite happens.

## 2. Goals / Non-goals

### 2.1 Goals

1. Add a preimage hash precondition to `write-file` operations.
2. Refuse `apply --mode execute` when a `write-file` preimage does not match the current file.
3. Treat an empty preimage as the new-file case: write only if the file does not exist.
4. Detect later touches to paths owned by older `write-file` operations during `record`, `reconcile`, and `verify`.
5. Downgrade or suppress historical drift appropriately when PRD-feature-supersession excludes the older feature from default replay.
6. Keep pre-preimage-hash recipes backward-compatible with a warning.
7. Preserve `artifacts/post-apply.patch` as authoritative feature intent; recipe safety guards prevent unsafe execution, not patch history preservation.

### 2.2 Non-goals

1. No v1 ban on `write-file` for existing files.
2. No automatic conversion from `write-file` to `replace-in-file` or patch/hunk operations.
3. No cross-feature proof that the entire ordered stack is safe.
4. No automatic recipe regeneration.
5. No new top-level CLI command.
6. No change to `created_by` semantics from ADR-011.
7. No change to patch-generation manifest schema in this PRD.

## 3. User-facing contract

### 3.1 `preimage_hash` schema

`write-file` operations gain an optional field:

```json
{
  "type": "write-file",
  "path": "src/routes/messages/handler.ts",
  "preimage_hash": "sha256:<64 lowercase hex>",
  "content": "...new complete file bytes..."
}
```

Binding contract:

- Field name: `preimage_hash`.
- Value for existing-file writes: `sha256:<64 lowercase hex>`, computed over the file bytes that existed immediately before the feature's recipe was generated or recorded.
- Value for new-file writes: empty string `""` or omitted only under the documented legacy path. In newly generated v1 recipes, an empty `preimage_hash` means "the file must not exist".
- Applies to operation type `write-file` only.

This follows ADR-024/ADR-025's content-addressed pattern: hashes are deterministic, portable, and compare bytes directly. Unlike `generation_id` or `attempt_id`, `preimage_hash` is not a record ID; it is a precondition.

### 3.2 Record-time behavior

When a recipe is generated or refreshed by `tpatch record <slug>` or an implementation path that writes `apply-recipe.json`, each `write-file` operation records the preimage:

1. If the path exists before the feature's change, store `preimage_hash: "sha256:<hash>"`.
2. If the path does not exist before the feature's change, store `preimage_hash: ""`.
3. If the recorder cannot determine the preimage, it must warn and mark the operation as legacy/unknown rather than inventing a hash.

Record also performs later-touch detection (§4.2). v1 **warns, not refuses**, when a new or refreshed feature's `write-file` covers a path already owned by an older active feature's `write-file`. Warning is chosen to avoid blocking legitimate whole-file replacement before supersession or ownership policy exists. Apply-time preimage mismatch remains a hard refusal.

### 3.3 Apply-time behavior

During `tpatch apply <slug>` or `tpatch apply <slug> --mode execute`, each `write-file` operation is checked before writing:

| Recipe state | Current path state | Result |
|---|---|---|
| `preimage_hash: "sha256:<h>"` | file exists and hash matches `<h>` | write succeeds. |
| `preimage_hash: "sha256:<h>"` | file missing | refuse with recipe drift. |
| `preimage_hash: "sha256:<h>"` | file exists and hash differs | refuse with recipe drift. |
| `preimage_hash: ""` | file does not exist | write succeeds. |
| `preimage_hash: ""` | file exists | refuse with recipe drift/new-file collision. |
| missing `preimage_hash` | any | legacy compatibility path: warn, then apply using current behavior in v1. |

The refusal message must identify the feature, operation index, path, expected hash, observed hash or missing-file state, and remediation category: regenerate or reconcile the recipe before replay.

### 3.4 Reconcile and verify behavior

- `tpatch reconcile [slug...]`: detects later touches to paths owned by older `write-file` operations and reports warnings. If reconcile executes a recipe and the preimage mismatches, apply-time refusal semantics apply.
- `tpatch verify`: adds or extends a V-check that reports stale `write-file` preimages. A stale preimage on an effective feature is an error/failure class. A stale preimage on a superseded historical feature is warning-only per PRD-feature-supersession.
- `tpatch status`: may surface drift labels or summary counts, but detailed detection belongs to reconcile/verify output.

## 4. Technical design

### 4.1 Schema extension

Extend the future `RecipeOperation` schema with:

```go
PreimageHash string `json:"preimage_hash,omitempty"`
```

The field is intentionally on the operation, not in a sidecar, because it is an execution precondition for the operation itself. The implementation must update all strict decoders, skill examples, and parity tests in the code slice; this paper PRD does not modify them.

For new v1 recipes, `preimage_hash` is required for `write-file` semantics even if the JSON field is technically `omitempty` for backward compatibility. Non-`write-file` operations ignore the field and should not emit it.

### 4.2 Later-touch detection

Later-touch detection asks: "Has a feature recorded after this `write-file` touched the same path?" v1 uses path-level detection, not hunk-level proof.

Detection surfaces:

1. **During record**: when recording feature `N`, scan older active/effective features for `write-file` operations that target any path in `N`'s touched paths. Emit a warning such as `warning: later-touch: <path> was whole-file-owned by <older>; this recipe may supersede or invalidate that older write-file`.
2. **During reconcile**: when reconciling a stack or feature, scan later active/effective features for paths owned by older `write-file` operations. Emit warnings before any recipe execution.
3. **During verify**: compare each effective `write-file` preimage to the current closure-replayed baseline expected for that feature. Stale preimages fail effective-feature verification; historical superseded features warn.

The v1 scanner should prefer existing deterministic artifacts where available: `apply-recipe.json` operations, `patch-generations.json.touched_paths`, and canonical patch touched paths. If artifacts disagree, the scanner reports a conservative warning; it must not claim a total stack-safety proof.

### 4.3 Supersession interaction

PRD-feature-supersession defines effective versus historical features. This PRD adopts that severity split:

- If an older feature is superseded by an active healthy replacement, stale `write-file` preimage on the older feature is expected historical drift. Default replay excludes that feature; diagnostics downgrade to warning.
- If the superseder is stale, missing, or conflicted, the older feature's drift is still not silently ignored. The graph reports the supersession problem and the recipe-safety checker reports warning or error based on whether the feature is in the effective replay set.
- If no supersession edge exists, stale preimage on an active/effective feature is an apply-time refusal and verify failure.

This cross-reference is load-bearing: supersession decides which features are effective; recipe safety decides whether an effective `write-file` may execute.

### 4.4 Backward compatibility

Existing recipes lack `preimage_hash`. Because the current decoder rejects unknown fields, the implementation slice must update the strict schema and all parity examples at once. For old recipes:

1. Missing `preimage_hash` on `write-file` is accepted in v1 as legacy.
2. Apply emits a warning that the operation lacks a preimage precondition.
3. Verify reports a warning recommending recipe regeneration.
4. A future major version may promote legacy missing-preimage writes to refusal after an explicit migration PRD.

This avoids breaking all existing features while making unsafe legacy behavior visible.

## 5. Implementation notes

1. Compute SHA-256 over exact file bytes, not normalized text. The display format is `sha256:<hex>`.
2. For new-file writes, record `preimage_hash: ""` and refuse if the target exists at apply time.
3. Stage all precondition checks before mutating any file so a multi-operation recipe does not partially apply after a later mismatch.
4. Sort path warnings by path then feature slug for deterministic output.
5. Include operation index in diagnostics because a recipe can write the same path more than once; a future policy may forbid duplicate writes.
6. Treat unreadable files as precondition failures, not as hash mismatches.
7. Do not store file contents in new safety metadata; hashes and repo-relative paths are sufficient.
8. Keep `created_by` hard-parent semantics unchanged. `created_by` answers "who originated this file"; `preimage_hash` answers "what bytes may I overwrite".
9. Regeneration guidance should be brief in v1 and must not name commands that do not exist unless explicitly proposed by that future UX slice.

## 6. Acceptance criteria

1. **Matching preimage applies**: a `write-file` with `preimage_hash` matching the current file applies successfully.
2. **Mismatched preimage refuses**: a `write-file` with a differing current file hash refuses before writing and reports recipe drift.
3. **Missing existing file refuses**: a `write-file` with non-empty `preimage_hash` refuses when the target file is absent.
4. **New-file empty preimage succeeds**: a `write-file` with `preimage_hash: ""` succeeds when the target path does not exist.
5. **New-file collision refuses**: a `write-file` with `preimage_hash: ""` refuses when the target path already exists.
6. **Atomic precheck**: if any operation precondition fails, no earlier operation in that recipe is written.
7. **Record later-touch warning**: recording a feature whose touched path overlaps an older active feature's `write-file` emits a deterministic warning.
8. **Reconcile later-touch warning**: reconcile reports when a later active/effective feature touched a path owned by an older `write-file` operation.
9. **Verify stale-preimage check**: verify reports stale `write-file` preimages on effective features as failures.
10. **Superseded drift downgrade**: stale preimage on an older feature superseded by an active healthy replacement is warning-only and does not fail default effective replay.
11. **Legacy recipe compatibility**: pre-preimage-hash `write-file` recipes still apply in v1 with a warning, and verify recommends regeneration.
12. **Schema parity**: shipped skill examples and `DecodeApplyRecipeStrict` agree on the `preimage_hash` field once code lands.
13. **No source leakage**: diagnostics include paths and hashes, not file bodies.

## 7. Open questions

1. Should v2 require `write-file` only for created files or declared whole-file ownership? Deferred to the prefer-contextual policy PRD.
2. Should later-touch warnings become apply-time blockers when no supersession edge exists? v1 blocks only on preimage mismatch.
3. Which command should perform recipe regeneration with the best UX? Existing `record` and `reconcile` paths can be mentioned generally, but a dedicated command/flag requires a future PRD.
4. Should duplicate `write-file` operations to the same path in one recipe be invalid? Deferred unless implementation finds a current need.

## 8. Out of scope

- Prefer-contextual generation policy.
- Cross-feature full-stack validation proof.
- Automatic conversion of whole-file writes to contextual operations.
- Auto-regeneration of stale recipes.
- Rich remediation UX beyond concise warnings/errors.
- New top-level CLI commands.

## 9. Sources

- GH #1 body, fetched 2026-07-29 with `gh issue view 1 --json body --repo tesseracode/tesserapatch`.
- `copilot-api/.tpatch/RETROSPECTIVE.md` Part 2, cited as external empirical evidence by reference.
- `internal/workflow/implement.go:42-108` (`ApplyRecipe`, `RecipeOperation`, strict recipe decoder, operation type enum).
- `internal/store/patch_generations.go` content-addressed manifest and strict read pattern.
- `docs/adrs/ADR-024-patch-generation-manifest-boundary.md` D2, D7, D9.
- `docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md` D2-D3.
- `docs/adrs/ADR-026-patch-amendment-policy.md` D5-D6.
- `docs/prds/PRD-feature-supersession.md` for effective/historical severity split.
