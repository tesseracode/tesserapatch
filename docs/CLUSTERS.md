# Active Clusters

**Purpose**: live state dashboard for in-flight paper clusters, paper-only
backlog items, and accepted planning gates.

**Not** a historical shipping log (use [`docs/ROADMAP.md`](./ROADMAP.md)).
**Not** a review receipt ledger (use
[`docs/supervisor/LOG.md`](./supervisor/LOG.md)).
**Not** a gap-study narrative (use `docs/whitepapers/` and
`docs/state-of-the-art/`).

**Cluster states**:
- **Exploring** — whitepaper in flight; PRDs not yet drafted.
- **Drafting** — PRDs being authored; cross-review cycle active.
- **Accepted** — planning approved; ADR or implementation dispatch still gated.
- **Implementing** — at least one wave in flight.
- **Shipped** — all approved waves shipped; closed to new scope.
- **Closed** — superseded or rejected without shipping.

## Snapshot — 2026-09-22

### Shipped cluster lineage

| Cluster | Source | Status | Notes |
|---|---|---|---|
| WP-001 | [whitepaper](./whitepapers/WP-001-feature-slice-gap.md) | **Shipped** | Graduated 2026-04-28; downstream implementation lineage shipped in v0.8.0. |
| WP-002 | [whitepaper](./whitepapers/WP-002-capture-and-metadata-foundation.md) | **Shipped** | Wave α shipped in v0.9.0; Waves β and γ shipped in v0.10.0; ADR-024 and ADR-026 are live. |
| WP-003 | [whitepaper](./whitepapers/WP-003-reconcile-safety-and-middle-pass.md) | **Shipped** | ADR-025 is accepted/live; all 9 PRDs shipped in the v0.11.0 lineage, followed by v0.11.1 stabilization. No active ADR blocker remains. |
| WP-005 | [whitepaper](./whitepapers/WP-005-spec-driven-workflows.md) | **Graduated / shipped implementation** | Optional preparation workflow graduated to GH #16/#23; v0.16.0 shipped the intent-bundle and archive work. |
| GH #15 | [recipe authority PRD](./prds/PRD-recipe-generation-authority.md) | **Shipped — v0.17.0** | Exact preimage/coverage/evidence authority shipped; no persisted anchors or GH #13 candidate implementation is implied. |

### Paper-only research / backlog

| Artifact | Type | Status | Current constraint | Next bounded step |
|---|---|---|---|---|
| [WP-004 auto feature dependency suggestions](./whitepapers/WP-004-auto-feature-dependencies.md) | Whitepaper | **Approved paper research** (2026-06-25) | Research only; no implementation or automatic PRD graduation is authorized. | If dependency-suggestion planning reopens, use WP-004 as the preferred research seed and revalidate citations first. |
| [WP-006 tpatch substrate and non-Git mode](./whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md) | Whitepaper | **Exploring** | Git-first recommendation only; no substrate interface or native non-Git VCS work is authorized. | Reopen only for narrow init/preflight planning that re-baselines current non-Git `prepare` behavior. |
| [WP-007 decision tickets and ticket tracking](./whitepapers/WP-007-decision-tickets-and-ticket-tracking.md) | Whitepaper | **Exploring** | Recommends against adding decision tickets as a tpatch feature type. | Paper-only external/hybrid map experiment if explicitly requested; no tpatch CLI/schema work. |
| [PRD-recurring-patches](./prds/PRD-recurring-patches.md) | PRD | **Approved (paper design)** | Implementation blocked on `ADR-recurring-patch-metadata-boundary`; v0.17 producer/event obligations still need explicit boundary treatment. | Draft and accept the boundary ADR before any implementation dispatch. |
| [PRD-tpatch-hotfix](./prds/PRD-tpatch-hotfix.md) | PRD | **Draft** | Existing fast-path proposal remains unrouted; this intake does not promote it. | Refresh its baseline and route explicitly if selected. |

### Preserved historical singletons

| Artifact | Disposition |
|---|---|
| [PRD-patch-already-upstream-detector](./prds/PRD-patch-already-upstream-detector.md) | Shipped in M17/v0.8.0; defer-list cleanup in v0.8.1. |
| [PRD-skill-doc-strategy](./prds/PRD-skill-doc-strategy.md) / ADR-020 | Shipped in May 2026; not a pending implementation. |
| [Intent-VCS evaluation](./prds/PRD-intent-version-control-evaluation.md), [Git primitive mapping](./prds/PRD-tpatch-git-primitive-mapping.md), [feature slices](./prds/PRD-feature-slices-and-nested-changes.md) | Superseded exploration preserved under WP-001; not reopened by this snapshot. |

### Accepted sequential planning queue

| Artifact | Status | Gate cleared | Next bounded step |
|---|---|---|---|
| [PRD-reconcile-operation-replay-candidate](./prds/PRD-reconcile-operation-replay-candidate.md) + [ADR-037](./adrs/ADR-037-reconcile-operation-replay-candidate-authority.md) + [ADR-043](./adrs/ADR-043-operation-candidate-capture-evidence.md) | **Accepted rev-7 planning — 2026-09-22** | v0.17.0 shipped GH #15; independent review accepts canonical E identity/gates/recovery follow-up. | Complete the documentation wave's resource-gated mechanical close, then obtain a separate GH #13 implementation assignment. |

### ADR snapshot relevant to active/backlog work

| ADR | Status | Notes |
|---|---|---|
| [ADR-024 patch generation manifest boundary](./adrs/ADR-024-patch-generation-manifest-boundary.md) | **Shipped / live** | WP-002 Wave β boundary; no longer pending. |
| [ADR-025 reconcile evidence and revision schema](./adrs/ADR-025-reconcile-evidence-and-revision-schema.md) | **Accepted / live** | WP-003 cluster ADR; all nine PRDs shipped under it. |
| [ADR-026 patch amendment policy](./adrs/ADR-026-patch-amendment-policy.md) | **Shipped / live** | WP-002 Wave γ amendment policy; no longer pending. |
| [ADR-041 independent capture-event evidence](./adrs/ADR-041-independent-capture-event-evidence.md) | **Accepted rev-1** | GH #15 planning amendment accepted; §7 names the follow-up dependency for GH #13 consumer planning. |
| [ADR-042 ordered recipe no-op proof](./adrs/ADR-042-ordered-recipe-noop-proof.md) | **Accepted** | Shipped in v0.17.0. |
| [ADR-043 operation-candidate capture-evidence identity](./adrs/ADR-043-operation-candidate-capture-evidence.md) | **Accepted** | Operator-selected direction; independent planning review approved 2026-09-22. No runtime implementation. |
| Capture-context privacy boundary (unassigned) | **Deferred** | Historical WP-002 v2 proposal, not an active blocker; any renewed work must reconcile with shipped ADR-027 rather than assume privacy design is absent. |

### Housekeeping notes

- 2026-09-22 — The previous pending-ADR block was stale. `ADR-024`,
  `ADR-025`, and `ADR-026` are already accepted/live, and the next available
  ADR number at intake was **043**, not 024; it is now assigned to the accepted
  GH #13 capture-evidence amendment. The next unused number is **044**.
- 2026-09-22 — `WP-004` is the preferred later planning track for dependency
  suggestions, but that preference does **not** auto-open a PRD.
- 2026-09-22 — Parent/coordinator owns ROADMAP queueing, CURRENT/HISTORY/LOG
  updates, issue actions, and any future implementation dispatch.

## How to update this file

- Keep this file to tables and dated bullets only.
- Mirror shipped state from `docs/ROADMAP.md`; do not use this file as the
  historical source of truth.
- When a paper or PRD stops being a blocker, flip it here instead of leaving it
  in a stale pending bucket.
- Record active planning gates, not full implementation narratives.
