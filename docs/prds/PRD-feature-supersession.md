# PRD - Feature Supersession - `feature-supersession`

**Status**: Proposed
**Date**: 2026-07-29
**Owner**: Core
**Cluster**: Replay trustworthiness (GH #1)
**Depends on**: [ADR-011 — Feature Dependency DAG](../adrs/ADR-011-feature-dependencies.md), [PRD-feature-dependencies](./PRD-feature-dependencies.md)
**Blocks**: [PRD-write-file-recipe-safety](./PRD-write-file-recipe-safety.md) severity policy for superseded historical recipes

## Related

- GH #1 — supersession and `write-file` replay safety.
- `copilot-api/.tpatch/RETROSPECTIVE.md` Part 2 — empirical evidence for the `effort-model-suffix` → `catalog-aware-effort-normalization` replacement and whole-file replay drift. This is an external evidence document; this PRD cites it by reference and does not copy its contents.
- [ADR-011 — Feature Dependency DAG](../adrs/ADR-011-feature-dependencies.md), especially D1-D4.
- [PRD-feature-dependencies](./PRD-feature-dependencies.md) hard/soft operational semantics.
- [PRD-write-file-recipe-safety](./PRD-write-file-recipe-safety.md) for `write-file` recipe drift and preimage preconditions.
- Optional companion ADR: [ADR-028 — Supersession edge model](../adrs/ADR-028-supersession-edge-model.md).

## 0. Meta

### 0.1 Paper-only status

This PRD is **Proposed**. It changes no code, schema, CLI behavior, shipped assets, CHANGELOG entry, or release artifact. It defines the implementation contract for a future slice.

### 0.2 Claims audit

| Claim | Evidence |
|---|---|
| Dependency declarations live in `status.json`, not `feature.yaml`. | ADR-011 D1 lines 22-28. |
| Cycle detection uses DFS and traversal uses Kahn's algorithm. | ADR-011 D2 lines 30-37. |
| Parent-derived labels are composable labels, not lifecycle states. | ADR-011 D3 lines 39-45. |
| Hard dependencies gate apply and `created_by`; soft dependencies do not. | ADR-011 D4 lines 47-55. |
| The existing dependency PRD stores `depends_on` entries with `slug` and `kind` and treats absence as backward-compatible empty dependencies. | `docs/prds/PRD-feature-dependencies.md` §3.1. |
| Current top-level commands referenced here (`status`, `next`, `record`, `reconcile`, `apply`, `verify`) exist. | `internal/cli/cobra.go` command registration and `Use:` strings. |

### 0.3 ADR-011 binding clauses quoted

This PRD extends ADR-011 without invalidating it:

- **D1**: "No new `feature.yaml` schema field. No migration. `depends_on` is written into `.tpatch/features/<slug>/status.json`..." Supersession therefore uses the same `depends_on` storage surface.
- **D2**: "Validation (write-time): `AddDependency` runs a depth-first search... Planning (read-time): `tpatch reconcile` and `tpatch status --dag` use Kahn's algorithm..." Supersession participates in the same graph validation and deterministic traversal.
- **D3**: "A feature's `state` field stays in `{analyzed, defined, explored, implemented, applied, recorded, reconciled}`. Derived labels are computed from the DAG + parent states on every render and may coexist." Supersession adds labels, not states.
- **D4**: "Hard deps gate apply AND `created_by`; soft deps gate neither." Supersession does not alter hard/soft gate behavior.

## 1. Problem statement

The existing feature graph represents prerequisite ordering: feature B may require or prefer feature A. GH #1 identifies a distinct relationship: feature B is the current replacement for feature A. The older feature remains historically valuable, but replaying both by default is wrong because the old feature can reintroduce obsolete behavior or collide with the replacement.

The observed external case is a long-lived customization stack in `copilot-api` where a catalog-aware implementation replaced an older model-suffix effort feature. The relationship is not `hard`: the replacement does not require the older implementation to function. It is not `soft`: the older implementation should be excluded from the effective replay set when the replacement is active. Deleting the older feature would destroy useful audit artifacts. A first-class supersession edge captures this "current implementation replaces historical implementation" fact.

## 2. Goals / Non-goals

### 2.1 Goals

1. Add a first-class `supersedes` relationship between features while preserving both feature histories and artifacts.
2. Reuse ADR-011 `status.json.depends_on[]` storage, cycle detection, and deterministic graph traversal.
3. Exclude superseded features from default effective replay when their replacement is active and healthy.
4. Surface supersession in `status`, `next`, dependency validation, `reconcile`, generated indexes, and machine-readable status output.
5. Detect cycles, reciprocal supersession, orphaned superseders, and multiple active superseders for one historical feature.
6. Provide queries for effective/current features versus historical/superseded features.
7. Keep ADR-011 hard/soft semantics unchanged.
8. Define the interaction with `write-file` drift from PRD-write-file-recipe-safety.

### 2.2 Non-goals

1. No automatic supersession detection in v1. Users or future commands author the edge explicitly.
2. No UI/display polish beyond deterministic labels and JSON fields needed for correctness.
3. No reverse-supersedes or automatic "undo supersession" workflow in v1.
4. No multi-replacement fan-in in v1: one historical feature may have at most one active superseder.
5. No provider-assisted rewrite of obsolete features.
6. No change to patch-generation history or amendment semantics.

## 3. User-facing contract

### 3.1 Locked edge model: Option A

Supersession is a third `depends_on[].kind` value:

```json
{
  "slug": "catalog-aware-effort-normalization",
  "state": "applied",
  "depends_on": [
    { "slug": "effort-model-suffix", "kind": "supersedes" }
  ]
}
```

**Decision**: choose **Option A**. `supersedes` is stored alongside `hard` and `soft` in `status.json.depends_on[]`.

**Rationale**:

- It honors ADR-011 D1 by keeping the graph in one canonical storage surface.
- It minimizes schema drift: no separate `supersedes: []` list and no new lifecycle state.
- ADR-011 D2's DFS cycle detection still works because `supersedes` is a directed edge from newer feature to historical feature. `X supersedes Y` plus `Y supersedes X` is a cycle and is rejected with the same path-producing validation machinery.
- Kahn's traversal still gives deterministic graph order for rendering and validation. Replay planning filters superseded historical nodes out of the default effective set before applying hard/soft gates; it does not reinterpret `supersedes` as a prerequisite gate.
- ADR-011 D3's label model extends cleanly: `superseded-by`, `active-superseder`, `stale-superseder`, and `orphan-superseder` are derived labels, not persisted states.

**Rejected options**:

- **Option B — separate `supersedes: []` list**: rejected because it creates a second edge surface that must duplicate validation, inverse-index generation, and status rendering.
- **Option C — lifecycle mutation to `superseded` state**: rejected because it violates ADR-011 D3's composable-label precedent and loses combinations such as `applied` + `superseded-by replacement` + `write-file-drift-warning`.

### 3.2 Edge direction and terminology

The newer feature declares the edge to the historical feature:

```text
newer-feature --kind=supersedes--> older-feature
```

- **Superseder**: the newer feature carrying the `kind: "supersedes"` edge.
- **Superseded feature**: the older feature named by that edge.
- **Effective feature set**: features included in default replay and `next` planning after superseded historical features are filtered out.
- **Historical feature set**: all features, including superseded ones, preserved for audit and explicit inspection.

### 3.3 Default replay and explicit historical replay

Default replay, `tpatch next`, and default reconcile planning operate on the effective set. If feature `B` actively supersedes feature `A`, `A` is excluded from default replay even if `A` remains in `applied` or `recorded` state.

Explicit historical replay remains possible only through an implementation-defined future flag or debug path. This PRD does not name a new CLI flag. If a future slice adds one, it must clearly mark the run as historical and must not be the default path.

### 3.4 Conflict detection

Validation must report these cases:

1. **Cycle**: `X supersedes Y` and `Y supersedes X`, or any longer cycle that includes `supersedes`, is invalid.
2. **Self-edge**: `X supersedes X` is invalid.
3. **Multiple active superseders**: `X supersedes Y` and `Z supersedes Y` is invalid when both `X` and `Z` are active/effective. v1 forbids active multi-replacement because replay ordering would be ambiguous.
4. **Orphan superseder**: `X supersedes Y` where `Y` no longer exists is a warning label, not a hard failure for read-only commands. Write paths that add the edge must refuse missing targets.
5. **Stale superseder**: the superseder exists but is unhealthy for replay, for example because PRD-write-file-recipe-safety detects recipe drift.

### 3.5 Query contract

Existing `tpatch status [slug]` remains the primary inspection command. A future implementation may add flags or JSON fields, but it must support these query concepts:

- **effective/current view**: excludes superseded historical features by default;
- **historical view**: includes superseded features and renders their supersession labels;
- **per-feature view**: for a superseded feature, shows its active superseder; for a superseder, shows the features it supersedes;
- **machine-readable view**: includes the edge kind and derived labels so scripts can distinguish `hard`, `soft`, and `supersedes`.

No new top-level command is proposed by this PRD.

## 4. Technical design

### 4.1 Schema

`status.json.depends_on[]` accepts a new kind literal:

```json
{ "slug": "<historical-feature-slug>", "kind": "supersedes" }
```

Binding field/value contracts:

- field: `depends_on`
- field: `kind`
- value: `supersedes`
- label values: `superseded-by <slug>`, `active-superseder`, `stale-superseder`, `orphan-superseder`

Backward compatibility: pre-supersession `status.json` files with missing `depends_on` or only `hard`/`soft` entries load unchanged. Unknown `kind` values outside the accepted set remain invalid once this PRD is implemented.

### 4.2 Validation

The graph validator treats `hard`, `soft`, and `supersedes` as directed edges for cycle detection. It applies kind-specific policy after cycle detection:

- `hard` and `soft`: existing ADR-011 semantics unchanged.
- `supersedes`: no prerequisite apply gate, but it affects effective-set filtering and conflict detection.

The inverse index must be able to answer "who supersedes this feature?" without adding a second persisted source of truth. Implementers may derive it in memory or persist derived `dependents` exactly as ADR-011 does for hard/soft dependents, as long as `depends_on[]` remains authoritative.

### 4.3 Label semantics

Supersession labels are composable derived labels. They do not mutate `state`.

| Label | Applied to | Meaning |
|---|---|---|
| `superseded-by <slug>` | superseded historical feature | An active/effective feature `<slug>` replaces this feature, so default replay excludes this feature. |
| `active-superseder` | newer feature | This feature actively supersedes at least one historical feature. |
| `stale-superseder` | newer feature and affected historical edge | The replacement is not healthy for default replay, for example due to recipe drift. |
| `orphan-superseder` | newer feature | The edge names a historical feature that is missing from the store. |

Locked rendering order within supersession labels is severity-first:

```text
[stale-superseder] [orphan-superseder] [superseded-by <slug>] [active-superseder]
```

If ADR-011 parent labels also apply, existing ADR-011 severity order is preserved first; supersession labels follow in the order above.

### 4.4 Command interactions

- `tpatch status [slug]`: renders supersession edges and labels. JSON output includes `kind: "supersedes"` edges and derived labels.
- `tpatch next`: computes the next action from the effective set by default, so superseded historical features are not suggested unless a future explicit historical mode requests them.
- `tpatch apply <slug>`: direct apply of a superseded feature should warn that the feature is historical and excluded from default replay. Default stack replay excludes it.
- `tpatch reconcile [slug...]`: default reconcile planning excludes superseded historical features from replay. Direct reconcile of a superseded feature is allowed for audit/repair but emits a historical-feature warning.
- `tpatch verify`: validation checks may report supersession conflicts and stale superseder labels.
- Generated indexes: any index that lists feature relationships must include `kind: "supersedes"` and effective/historical classification.

### 4.5 Reconcile interaction with write-file safety

PRD-write-file-recipe-safety introduces `preimage_hash` checks and later-touch detection for `write-file` operations. Supersession changes severity for historical features:

1. If a feature is superseded by an active, healthy superseder, its `write-file` drift is expected historical drift. Default replay excludes the feature, so default reconcile does not fail the effective stack because of that drift.
2. Read-only diagnostics may still report the drift as a **warning** on the historical feature for audit visibility.
3. If the superseder is stale or unhealthy, the superseded feature does not automatically become effective again. The graph reports `stale-superseder`, and an operator must choose repair, explicit historical replay, or edge removal in a future workflow.
4. If no active superseder exists, `write-file` drift severity follows PRD-write-file-recipe-safety normally.

This PRD therefore chooses **downgrade-to-warning for superseded historical drift**, not total suppression. The default replay path is protected by exclusion; audit paths remain informative.

## 5. Implementation notes

1. Extend the dependency kind enum to include `supersedes`.
2. Extend graph validation to detect cycles across all edge kinds, then run supersession-specific fan-in checks.
3. Derive an effective set by removing features with exactly one active healthy superseder.
4. Keep hard/soft apply gates unchanged. `supersedes` never satisfies or blocks `created_by`.
5. Ensure status and JSON render both directions: superseder → superseded and superseded → superseder.
6. Add fixtures for legacy status files with no `depends_on` and with hard/soft only.
7. Update shipped skill docs only in the implementation slice if they teach dependency authoring.
8. Keep generated artifacts deterministic: sorted slugs, sorted labels, and stable JSON field order where existing writers require it.

## 6. Acceptance criteria

1. **Cycle detection**: `X supersedes Y` plus `Y supersedes X` is rejected with an actionable cycle path.
2. **Longer mixed cycle**: `X hard-depends-on Y`, `Y supersedes Z`, `Z soft-depends-on X` is rejected by the same graph validator.
3. **Self-edge**: `X supersedes X` is rejected.
4. **Multi-superseder conflict**: `X supersedes Y` plus `Z supersedes Y` reports a conflict when both `X` and `Z` are active/effective.
5. **Default replay filtering**: default replay/next planning excludes `Y` when active healthy `X supersedes Y` exists.
6. **Historical query**: status JSON can distinguish effective features from historical/superseded features.
7. **Label rendering**: `superseded-by <slug>`, `active-superseder`, `stale-superseder`, and `orphan-superseder` render in locked severity order and compose with ADR-011 labels.
8. **Backward compatibility**: pre-supersession `status.json` files with no `depends_on` or only `hard`/`soft` edges still load and behave as before.
9. **ADR-011 unchanged**: hard/soft apply gates, `created_by` rules, and parent-derived labels retain existing behavior.
10. **Write-file drift interaction**: drift on an active superseded historical feature is a warning/audit signal, not an effective-stack replay failure.
11. **Generated indexes**: relationship indexes include `kind: "supersedes"` and effective/historical classification.
12. **Orphan detection**: an existing edge to a missing historical feature produces `orphan-superseder` on read-only commands and write-time refusal for newly added edges.

## 7. Open questions

1. Which exact flag names should expose effective versus historical status views? This PRD defines the query concepts but leaves flag spelling to the implementation slice.
2. Should direct `tpatch apply <superseded>` require an explicit override flag, or is a warning sufficient? Default stack replay must exclude it either way.
3. Should future auto-detection propose supersession edges based on same path/claim ownership, patch IDs, or user review? Deferred.

## 8. Out of scope

- Automatic supersession inference.
- Reverse-supersedes / undo workflows.
- Multi-replacement semantics beyond v1 conflict detection.
- Rich UI graph visualization.
- Provider-assisted migration from obsolete feature to replacement.
- New top-level CLI commands.

## 9. Sources

- GH #1 body, fetched 2026-07-29 with `gh issue view 1 --json body --repo tesseracode/tesserapatch`.
- `copilot-api/.tpatch/RETROSPECTIVE.md` Part 2, cited as external empirical evidence by reference.
- `docs/adrs/ADR-011-feature-dependencies.md` D1-D4.
- `docs/prds/PRD-feature-dependencies.md` §3.1-§3.4.
- `internal/cli/cobra.go` command registration for existing command names.
- `docs/prds/PRD-write-file-recipe-safety.md` for recipe-drift severity coupling.
