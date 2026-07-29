# ADR-028 — Supersession Edge Model

**Status**: Proposed
**Date**: 2026-07-29
**Owner**: Core
**Cluster**: Replay trustworthiness (GH #1)
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-011](./ADR-011-feature-dependencies.md)
**Blocks**: [PRD-feature-supersession](../prds/PRD-feature-supersession.md) implementation

## Context

GH #1 identifies a feature-graph gap: a newer feature can replace an older feature without requiring it as a prerequisite and without deleting its history. `PRD-feature-supersession` evaluates three models:

1. add `supersedes` as a third edge kind in the existing ADR-011 graph;
2. store a separate `supersedes: []` list;
3. mutate the older feature into a new `superseded` lifecycle state.

ADR-011 already locks the dependency graph into `status.json.depends_on[]`, uses DFS for cycle detection, Kahn's algorithm for deterministic traversal, and represents graph-derived conditions as composable labels rather than states. This ADR locks the minimal extension so the implementation slice does not re-litigate the graph shape.

This ADR is paper-only. It changes no code, schema, CLI behavior, assets, PRD body, handoff, release artifact, or CHANGELOG entry.

## Decision

### D1 — `supersedes` is a third `depends_on[].kind` value

Supersession is represented as:

```json
{
  "depends_on": [
    { "slug": "older-feature", "kind": "supersedes" }
  ]
}
```

The edge is authored on the newer feature and points to the older historical feature. The literal `supersedes` joins the existing `hard` and `soft` kind values.

**Rationale**. This preserves ADR-011 D1's one graph storage surface in `status.json`, avoids a second relationship list, and avoids a lifecycle-state mutation that would break ADR-011 D3's label model.

### D2 — ADR-011 cycle detection applies across all three edge kinds

DFS validation treats `hard`, `soft`, and `supersedes` as directed edges for cycle detection. A reciprocal pair (`X supersedes Y`, `Y supersedes X`) and any longer mixed-kind cycle are invalid.

**Rationale**. ADR-011 D2 already chose DFS because it returns the actionable cycle path. Supersession needs exactly that operator feedback.

### D3 — Kahn traversal remains the deterministic graph traversal primitive

Read-time graph rendering and planning continue to use Kahn's algorithm with the existing deterministic tie-break. Superseded historical features are filtered out of the default effective replay set before hard/soft gate evaluation.

**Rationale**. `supersedes` is a graph relationship and validation edge, not a prerequisite gate. Keeping Kahn's traversal avoids a second graph engine while allowing replay planners to choose effective versus historical views.

### D4 — Supersession labels are derived, composable labels

Supersession adds these derived labels:

- `superseded-by <slug>`
- `active-superseder`
- `stale-superseder`
- `orphan-superseder`

They are never persisted lifecycle states. Within the supersession label group, rendering order is:

```text
[stale-superseder] [orphan-superseder] [superseded-by <slug>] [active-superseder]
```

ADR-011 parent-label order remains unchanged and precedes supersession labels when both apply.

### D5 — One active superseder per historical feature in v1

A historical feature may have at most one active/effective superseder. Multiple active superseders for the same target are a validation conflict. Inactive or unhealthy candidates can be reported, but they do not make multi-replacement valid.

**Rationale**. Default replay needs one effective implementation. Allowing multiple replacements requires ordering and merge semantics that belong to a later PRD.

### D6 — Supersession preserves history and excludes by default

Supersession never deletes or rewrites the older feature's artifacts. It excludes the superseded feature from default replay, default `next` planning, and default effective reconcile when the superseder is active and healthy. Explicit historical inspection remains possible through status/reconcile surfaces defined by the PRD.

### D7 — Hard/soft semantics are not amended

`hard` and `soft` dependency semantics from ADR-011 D4 remain unchanged. `supersedes` does not gate `created_by`, does not satisfy a hard dependency, and does not convert a soft dependency into a warning/error.

### D8 — Superseded historical recipe drift is warning-class by default

When a superseded historical feature has recipe drift, including `write-file` preimage drift from `PRD-write-file-recipe-safety`, default effective replay is protected by excluding the historical feature. Read-only diagnostics may still warn for audit visibility. Drift on the active superseder follows normal severity.

## Consequences

- One graph surface remains authoritative: `status.json.depends_on[]`.
- Existing hard/soft readers must be extended to accept a third kind rather than assuming a two-value enum.
- Effective replay becomes a graph-filtering operation, not a state mutation.
- Operators can preserve old feature artifacts while making the replacement the current implementation.

## Alternatives considered

1. **Separate `supersedes: []` list** — rejected. It duplicates inverse-index, cycle-validation, and rendering code.
2. **New `superseded` lifecycle state** — rejected. It creates state combinations that ADR-011's composable labels intentionally avoid.
3. **Allow multiple active superseders** — rejected for v1. Ambiguous default replay requires a future policy.
4. **Suppress historical drift entirely** — rejected. Warning-class audit output is safer than hiding drift from explicit historical inspection.

## References

- GH #1 — missing `supersedes` relationship and replay trustworthiness.
- `copilot-api/.tpatch/RETROSPECTIVE.md` Part 2 — external empirical evidence, cited by reference.
- [PRD-feature-supersession](../prds/PRD-feature-supersession.md).
- [ADR-011 — Feature Dependency DAG](./ADR-011-feature-dependencies.md) D1-D4.
- [PRD-write-file-recipe-safety](../prds/PRD-write-file-recipe-safety.md).
