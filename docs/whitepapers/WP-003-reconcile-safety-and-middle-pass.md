# WP-003 - Reconcile Safety and Middle-pass Foundation

**Status**: Accepted paper cluster (implementation not authorized)
**Authors**: T56
**Started**: 2026-05-16
**Related**:
- [WP-001 Feature-slice gap](./WP-001-feature-slice-gap.md)
- [Active clusters](../CLUSTERS.md)
- [Reconcile workflow](../reconcile.md)
- [t3code v0.0.23 case-study notes](../state-of-the-art/case-studies/t3code-upstream-v0.0.23-2026-05/local-notes.md)
- [T55 patch identity metadata PRD](../prds/PRD-feature-patch-identity-metadata.md)

## 1. Context

The first structural middle-pass case study,
`t3code-upstream-v0.0.23-2026-05`, showed two separate reconcile gaps:

1. False-positive `upstreamed` verdicts are unsafe. `session-search` was working
   in the fork before reconcile, then was dropped by a false-positive
   `upstream_merged` transition. `copilot-skill-controls` was not fully
   implemented, but the reconciler matched sibling scaffolding and treated the
   feature as upstreamed. Both cases required human repair.
2. Generic `blocked` verdicts are too coarse. Thirteen of fifteen blocked
   verdicts were false positives: nine reapplied cleanly, four needed only
   insertion-point relocation, and only two were true structural conflicts tied
   to an upstream directory restructure.

The gap is not "replace reconcile with an LLM." It is a narrower safety and
evidence gap: tpatch needs durable evidence for why a verdict fired, explicit
confirmation before feature retirement, structured revision-pass records, and
cheap middle-pass classifiers before broad structural/search algorithms.

## 2. Cluster overview

WP-003 consists of nine PRDs. PRD 1 is the schema keystone. It branches into
confirmation, revision logging, and narrow middle-pass classification:

```text
1 PRD-reconcile-verdict-evidence
|-- 2 PRD-upstreamed-confirmation-gate
|   `-- 4 PRD-reconcile-retirement-state-audit
|-- 3 PRD-reconcile-revision-pass-log
|   `-- 5 PRD-reconcile-study-validation
`-- 6 PRD-reconcile-file-novelty-classifier
    `-- 7 PRD-reconcile-hunk-overlap-detector
        `-- 8 PRD-reconcile-blocked-verdict-taxonomy
            `-- 9 PRD-reconcile-path-restructure-detector
```

The first branch prevents unsafe retirement. The second makes review corrections
auditable. The third promotes only the narrow deterministic middle-pass signals
the t3code study supports today.

## 3. Cross-cluster prerequisites

WP-002 Wave beta must reach acceptance before WP-003 PRD 1 implementation can
start. WP-002's `PRD-feature-patch-identity-metadata` defines
`patch-generations.json`; WP-003 PRD 1 defines `reconcile-evidence.jsonl`.
Those are sibling per-feature artifacts. `patch-generations.json` tracks patch
identity for replay and dependency reasoning. `reconcile-evidence.jsonl` tracks
verdict evidence for review and revision passes. The schemas must not drift,
and they may eventually share a `refs` field for cross-artifact links.

This prerequisite does not block paper review of WP-003. It blocks
implementation so both clusters can align artifact boundaries before code lands.

## 4. ADR plan

Implementation requires one cluster ADR:

```text
ADR-025-reconcile-evidence-and-revision-schema
```

The ADR covers PRDs 1, 2, and 3 directly: verdict evidence,
upstreamed-confirmation storage, and revision-pass log semantics. PRDs 4-9 ship
under the same ADR because they write or consume the same evidence and revision
records. The Wave alpha implementer drafts ADR-025 before PRD 1 implementation.

## 5. Privacy anchor

WP-003 uses the same privacy boundary as the T55 capture-and-metadata cluster:
the deferred `ADR-capture-context-privacy-boundary`. Until that ADR says
otherwise, WP-003 artifacts may store paths, hashes, operation IDs, enum reason
codes, verdicts, upstream commit refs, counts, and validation references. They
must not persist raw source bodies, provider transcripts, prompt text, or vector
embeddings.

This shared anchor keeps the two clusters aligned: capture metadata, patch
identity metadata, reconcile evidence, and revision logs should all obey the
same no-source-bodies / no-transcripts / no-vectors default.

## 6. Waves / implementation order

Wave alpha is foundational and parallel-safe after ADR-025 exists and WP-002
Wave beta has reached acceptance:

| Wave | PRDs | Gate |
|---|---|---|
| alpha | PRD 1 `reconcile-verdict-evidence`; PRD 6 `reconcile-file-novelty-classifier` | ADR-025 + WP-002 Wave beta acceptance |
| beta | PRD 2 `upstreamed-confirmation-gate`; PRD 3 `reconcile-revision-pass-log`; PRD 7 `reconcile-hunk-overlap-detector` | Wave alpha |
| gamma | PRD 4 `reconcile-retirement-state-audit`; PRD 5 `reconcile-study-validation`; PRD 8 `reconcile-blocked-verdict-taxonomy`; PRD 9 `reconcile-path-restructure-detector` | Wave beta |

The broad structural/search backlog remains deferred until at least one more
upstream-transition reconcile case study exists:

- `PRD-structural-patch-fingerprints`
- `PRD-reconcile-commutation-graph`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-search-planner`
- `PRD-reconcile-planner-audit-artifacts`
- `PRD-patch-vector-index`
