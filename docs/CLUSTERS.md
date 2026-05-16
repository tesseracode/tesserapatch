# Active Clusters

**Purpose**: live state dashboard for in-flight PRD clusters. Tells you "where
are we right now" — what's accepted, what's implementing, what's blocked,
which ADRs are pending.

**Not** a historical shipping log (that's [`docs/ROADMAP.md`](./ROADMAP.md)).
**Not** a decision audit trail (that's
[`docs/supervisor/LOG.md`](./supervisor/LOG.md)). **Not** a gap-study
narrative (those are `docs/whitepapers/`).

This document is updated by the broker / supervisor at every cluster state
transition — new PRD accepted, ADR assigned, wave kickoff, wave ship,
implementation blocker.

**Cluster states**:

- **Exploring** — whitepaper in flight; PRDs not yet drafted.
- **Drafting** — PRDs being authored; cross-review cycle active.
- **Accepted** — supervisor LOG entry approving the cluster; ADRs pending.
- **Implementing** — at least one wave in flight.
- **Shipped** — all waves shipped; closed to new work (file a follow-up
  cluster instead).
- **Closed** — superseded or rejected without shipping.

---

## WP-001 — Feature-slice gap & intent-VCS direction

**Status**: Shipped (graduated 2026-04-28, T16)
**Whitepaper**: [`docs/whitepapers/WP-001-feature-slice-gap.md`](./whitepapers/WP-001-feature-slice-gap.md)
**Graduated to**: v0.7 cluster (`tpatch-land`, `record-auto-base`, `record-collision-detection`, `reconcile-lock-guard`), shipped as M17 / v0.8.0 (2026-05-12, tag `29a6732`).

The original whitepaper. Documented the boundary-capture gap surfaced by
Cases A1 (copilot-api) and A2 (t3code). Closed at Turn 16 with the
four-PRD graduation. No remaining work.

**Headline finding (T13 ratified)**: no data-model gap — the failure was
recording boundaries, not splitting content.

---

## WP-002 — Capture & metadata foundation *(T55 cluster)*

**Status**: **Implementing** (Wave α shipping; Waves β/γ blocked on ADRs)
**Whitepaper**: **PENDING** — to be authored by a fresh agent (cluster summary lift from supervisor LOG entry)
**Supervisor acceptance**: 2026-05-13 (see `docs/supervisor/LOG.md`)
**Cluster ADR plan**: three slugged ADRs (numbered when implementer drafts):
  - `ADR-capture-context-privacy-boundary` (deferred; only required for v2 `--reason` / context-persistence work)
  - `ADR-patch-generation-manifest-boundary` (**pending**; required before Wave β)
  - `ADR-patch-amendment-policy` (**pending**; required before Wave γ)

### PRDs

| # | PRD | Wave | State |
|---|---|---|---|
| 1 | [`PRD-feature-file-claims`](./prds/PRD-feature-file-claims.md) | α | **Shipping** — v0.9.0-alpha-1 shipped 2026-05-13 + rev-1 fix 2026-05-13 |
| 2 | [`PRD-record-capture-modes`](./prds/PRD-record-capture-modes.md) | α | **Shipping** — v0.9.0-alpha-2 shipped 2026-05-14 + rev-1 fix 2026-05-14 |
| 3 | [`PRD-feature-patch-identity-metadata`](./prds/PRD-feature-patch-identity-metadata.md) | β | **Blocked** on `ADR-patch-generation-manifest-boundary` |
| 4 | [`PRD-feature-patch-amend`](./prds/PRD-feature-patch-amend.md) | γ | **Blocked** on Wave β + `ADR-patch-amendment-policy` |

### Implementation order
- **Wave α** (parallel, no internal deps): PRDs 1 + 2. **Both shipping** as v0.9.0 alpha tags.
- **Wave β** (depends on Wave α + ADR): PRD 3 (`patch-generations.json`).
- **Wave γ** (depends on Wave β + ADR): PRD 4 (refresh/fixup/metadata-only).

### Cross-cluster relationships
- Downstream consumer: WP-003 PRD 1 (reconcile-evidence) coordinates artifact schema with PRD 3 (`patch-generations.json`) to prevent drift.
- ADR-021/022/023 slot numbers were **not** taken by this cluster (used by unrelated work); T55 cluster uses **named** slugs.

### Blockers
- ADR-patch-generation-manifest-boundary unwritten — blocks Wave β start.
- ADR-patch-amendment-policy unwritten — blocks Wave γ start.

---

## WP-003 — Reconcile safety & middle-pass *(T56 cluster)*

**Status**: **Accepted** (T56 revision complete; supervisor LOG entry pending)
**Whitepaper**: [`docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md`](./whitepapers/WP-003-reconcile-safety-and-middle-pass.md)
**Origin**: t3code v0.0.23 case study (first structural middle-pass study, false-positive `upstreamed` verdicts on `session-search` and `copilot-skill-controls`).
**Cluster ADR plan**: single cluster ADR — `ADR-024-reconcile-evidence-and-revision-schema` (covers PRDs 1, 2, 3; PRDs 4–9 ship under the same ADR).

### Cross-cluster prerequisite

**WP-002 Wave β must reach acceptance before WP-003 PRD 1 implementation
can start.** Both clusters define per-feature evidence artifacts
(`patch-generations.json` vs `reconcile-evidence.jsonl`); their schemas
must not drift.

### PRDs (dependency tree, not flat list)

```
1 (verdict-evidence) ──┬── 2 (upstreamed-confirmation-gate) ── 4 (retirement-state-audit)
                       │
                       ├── 3 (revision-pass-log) ── 5 (study-validation)
                       │
                       └── 6 (file-novelty-classifier) ── 7 (hunk-overlap-detector) ── 8 (blocked-verdict-taxonomy) ── 9 (path-restructure-detector)
```

| # | PRD | Wave (proposed) | State |
|---|---|---|---|
| 1 | [`PRD-reconcile-verdict-evidence`](./prds/PRD-reconcile-verdict-evidence.md) | α | Approved (cluster keystone; blocked on ADR-024 + WP-002 Wave β acceptance) |
| 2 | [`PRD-upstreamed-confirmation-gate`](./prds/PRD-upstreamed-confirmation-gate.md) | β | Approved |
| 3 | [`PRD-reconcile-revision-pass-log`](./prds/PRD-reconcile-revision-pass-log.md) | β | Approved |
| 4 | [`PRD-reconcile-retirement-state-audit`](./prds/PRD-reconcile-retirement-state-audit.md) | γ | Approved |
| 5 | [`PRD-reconcile-study-validation`](./prds/PRD-reconcile-study-validation.md) | γ | Approved |
| 6 | [`PRD-reconcile-file-novelty-classifier`](./prds/PRD-reconcile-file-novelty-classifier.md) | α | Approved (blocked on ADR-024) |
| 7 | [`PRD-reconcile-hunk-overlap-detector`](./prds/PRD-reconcile-hunk-overlap-detector.md) | β | Approved |
| 8 | [`PRD-reconcile-blocked-verdict-taxonomy`](./prds/PRD-reconcile-blocked-verdict-taxonomy.md) | γ | Approved |
| 9 | [`PRD-reconcile-path-restructure-detector`](./prds/PRD-reconcile-path-restructure-detector.md) | γ | Approved |

### Blockers
- `ADR-024` unwritten — blocks Wave α start.
- WP-002 Wave β unwritten — blocks PRD 1 implementation even if `ADR-024` ships first.

---

## Singletons (not part of a cluster)

These are PRDs that exist in `docs/prds/` but aren't part of an active cluster.

| PRD | State | Notes |
|---|---|---|
| [`PRD-tpatch-hotfix`](./prds/PRD-tpatch-hotfix.md) | Drafted (OX47) | Sibling fast-path verb; trailer-block coordinated with `PRD-tpatch-land`. Awaiting routing. |
| [`PRD-patch-already-upstream-detector`](./prds/PRD-patch-already-upstream-detector.md) | Drafted (OX47, unsolicited) | Post-M14 research; phase-1.5 detector. **Shipped as M17 Wave D** (v0.8.0). Defer-list cleanup landed v0.8.1. |
| [`PRD-skill-doc-strategy`](./prds/PRD-skill-doc-strategy.md) + ADR-020 | Shipped | feat-skill-doc-references-user-visible shipped 2026-05-14 rev-1. |

### Exploratory PRDs superseded by WP-001 (do not edit)

- [`PRD-intent-version-control-evaluation`](./prds/PRD-intent-version-control-evaluation.md)
- [`PRD-tpatch-git-primitive-mapping`](./prds/PRD-tpatch-git-primitive-mapping.md)
- [`PRD-feature-slices-and-nested-changes`](./prds/PRD-feature-slices-and-nested-changes.md)

These remain in place as historical exploration; WP-001 is listed in their
"Supersedes" header. One-way link.

---

## Pending ADRs (cross-cluster view)

| ADR slug or number | Cluster | Blocks | Owner |
|---|---|---|---|
| `ADR-capture-context-privacy-boundary` | WP-002 (deferred to v2) | Free-text reason persistence, agent-context retention | Implementer of v2 claims work; not currently blocking |
| `ADR-patch-generation-manifest-boundary` | WP-002 Wave β | `PRD-feature-patch-identity-metadata` implementation | Implementer of Wave β |
| `ADR-patch-amendment-policy` | WP-002 Wave γ | `PRD-feature-patch-amend` implementation | Implementer of Wave γ |
| `ADR-024-reconcile-evidence-and-revision-schema` | WP-003 (entire cluster) | All 9 WP-003 PRDs (Wave α onward) | Implementer of Wave α |

ADRs 021, 022, 023 are **already used** by unrelated work (land global
metadata carve-out, detector default deferral, hotfix auto-drop deferral).
The next available numbered slot is **024**.

---

## How to update this file

- When a PRD is **drafted**: add row, state = "Draft."
- When a cluster is **accepted** (supervisor LOG entry posted): flip
  cluster header to "Accepted"; link the LOG entry.
- When a PRD enters **implementation**: flip per-PRD state to "Implementing"
  or "Shipping" with the slice tag.
- When a wave **ships**: flip per-PRD state to "Shipped." If the entire
  cluster shipped, flip cluster header to "Shipped" and reference the
  ROADMAP.md milestone.
- When an **ADR is drafted**: move from "pending" → "drafted." Update the
  Pending ADRs table.
- When a **cross-cluster blocker** is introduced or cleared: update the
  blocker line on both clusters involved.

Avoid prose updates. Tables and dated bullets only. The narrative belongs
in whitepapers and supervisor LOG.
