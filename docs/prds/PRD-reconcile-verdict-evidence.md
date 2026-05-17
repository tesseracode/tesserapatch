# PRD - Reconcile Verdict Evidence - `feat-reconcile-verdict-evidence`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: Implementation requires `ADR-025-reconcile-evidence-and-revision-schema`. WP-002 Wave beta must reach acceptance before implementation.

## Related

- [Reconcile Workflow](../reconcile.md)
- [Feature Layout](../feature-layout.md)
- [Patch-already-upstream detector](./PRD-patch-already-upstream-detector.md)
- [t3code v0.0.23 local notes](../state-of-the-art/case-studies/t3code-upstream-v0.0.23-2026-05/local-notes.md)
- [WP-003 Reconcile Safety and Middle-pass](../whitepapers/WP-003-reconcile-safety-and-middle-pass.md)
- [WP-002 Capture and metadata foundation](../CLUSTERS.md#wp-002--capture--metadata-foundation-t55-cluster) - cross-cluster prerequisite: Wave beta must accept the patch-generation manifest boundary before this PRD is implemented.
- [T55 patch identity metadata PRD](./PRD-feature-patch-identity-metadata.md)
- [Research roadmap](../state-of-the-art/research-roadmap.md)

## Cluster Position

WP-003 dependency tree:

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

Implementation waves: alpha = PRDs 1 + 6; beta = PRDs 2, 3, 7; gamma = PRDs
4, 5, 8, 9. See [WP-003](../whitepapers/WP-003-reconcile-safety-and-middle-pass.md).

## 0. Claims Audit

This PRD is a proposal. It changes nothing.

| Claim | Evidence |
|---|---|
| Reconcile currently exposes coarse outcomes such as `upstreamed`, `reapplied`, `still_needed`, and `blocked`. | `internal/store/types.go` `ReconcileOutcome`; `docs/reconcile.md` phase list. |
| `ReconcileSummary` persists `Outcome`, upstream ref/commit, labels, resolver fields, and optional `PatchIDMatch`, but not a general evidence bundle. | `internal/store/types.go` `ReconcileSummary`. |
| Patch-id matches already preserve match evidence in a narrow typed field. | `internal/store/types.go` `PatchIDMatch`. |
| The t3code study showed that machine verdicts needed later review correction. | `local-notes.md` in the imported t3code case study. |

No code, schema, or command behavior is changed by this PRD.

## Summary

Persist a small, reviewable evidence bundle for each reconcile verdict so an
operator or reviewer can answer: what phase produced this verdict, what evidence
did it rely on, where did that evidence come from, and what should be checked
before trusting it?

The evidence bundle is a foundation. It does not change verdict semantics by
itself. Later PRDs use it to gate `upstreamed`, record revision passes, validate
case studies, and classify blocked outcomes.

## 1. Problem Statement

The current status model can say "this feature is upstreamed" or "this feature
is blocked", but it cannot explain enough about the evidence behind that claim.
The t3code study exposed the risk:

- `session-search` was working in the fork before reconcile, then was dropped
  because an `upstreamed` verdict was trusted.
- `copilot-skill-controls` was classified as upstreamed because recipe search
  strings matched sibling scaffolding, not the feature intent.
- Reviewers had to reconstruct what happened from commit history and manual
  searches rather than from tpatch-owned evidence.

This PRD fills the audit gap without deciding whether a verdict is right.

## 2. Goals / Non-goals

### Goals

1. Persist machine-readable evidence for every non-empty reconcile verdict.
2. Separate raw machine verdict, evidence source, confidence, and final feature
   state.
3. Preserve enough path/hash/operation metadata for review without storing raw
   source snippets.
4. Make `upstreamed` evidence auditable before any local patch is retired.
5. Support future classifiers for file novelty, hunk overlap, blocked taxonomy,
   and path restructure detection.

### Non-goals

1. No new reconcile verdicts in this PRD.
2. No confirmation gate behavior; that is `PRD-upstreamed-confirmation-gate`.
3. No provider prompt or transcript persistence.
4. No vector embeddings or source-code bodies.
5. No auto-repair, auto-reapply, or auto-retire behavior.

## 3. Proposed Artifact

Add a per-feature artifact:

```text
.tpatch/features/<slug>/artifacts/reconcile-evidence.jsonl
```

Each line is one reconcile evidence attempt:

```json
{
  "schema_version": 1,
  "feature_slug": "session-search",
  "attempt_id": "sha256:...",
  "upstream_ref": "upstream/main",
  "upstream_commit": "b83e9c9...",
  "base_commit": "9df3c64...",
  "raw_reconcile_verdict": "upstreamed",
  "phase": "phase-2",
  "evidence_kind": "recipe-operation-match",
  "confidence": "low",
  "matched_paths": ["apps/web/src/..."],
  "matched_operations": ["op-3", "op-4"],
  "match_origin": "unknown",
  "upstream_commit_refs": [],
  "pre_reconcile_presence": "present",
  "requires_confirmation": true,
  "reason_code": "match-origin-unknown"
}
```

Field rules:

- `attempt_id` is content-derived from slug, upstream commit, base commit, raw
  verdict, phase, and normalized evidence entries.
- `confidence` uses `high`, `medium`, `low`, or `unknown`.
- `match_origin` uses `upstream`, `fork`, `sibling-feature`, `unknown`, or
  `mixed`.
- `pre_reconcile_presence` uses `present`, `absent`, `unknown`, or
  `not-checked`.
- `upstream_commit_refs` is required for high-confidence upstreamed evidence.
- Raw source bodies, prompt transcripts, and embeddings are forbidden in v1.

`patch-generations.json` from the T55 cluster tracks patch identity for replay
and dependency reasoning. `reconcile-evidence.jsonl` tracks reconcile verdict
evidence for review and revision passes. They are sibling per-feature artifacts,
and their schemas must not drift; a future shared `refs` field may link evidence
attempts to patch generations.

### 3.1 Enums

`phase` values:

- `phase-1`
- `phase-1.5`
- `phase-2`
- `phase-3`
- `phase-3.5`
- `phase-4`

`evidence_kind` values are extensible. V1 starts with:

- `reverse-apply`
- `patch-id-match`
- `recipe-operation-match`
- `provider-semantic`
- `forward-apply`
- `file-novelty` (added by PRD 6)
- `hunk-overlap` (added by PRD 7)
- `blocked-classification` (added by PRD 8)
- `path-restructure` (added by PRD 9)
- `manual-review`
- `unknown`

## 4. User-facing Contract

Default human output gains a short evidence hint:

```text
session-search: upstreamed-candidate (low confidence)
  evidence: phase-2 recipe-operation-match
  review: confirmation required before retirement
```

JSON output includes the latest evidence bundle or a reference to the artifact.

## 5. Implementation Notes

- `status.json` remains the current-truth record. The evidence artifact is audit
  detail and must not replace `ReconcileSummary`.
- Existing `PatchIDMatch` can be copied into an evidence entry rather than
  duplicated by hand-coded output.
- The artifact should be deterministic: stable key order per JSONL line, sorted
  arrays, no wall clock fields beyond existing status timestamps.
- Evidence writes should occur only when reconcile writes a verdict.
- Writes must be atomic: write a temporary file, fsync when supported by the
  existing store pattern, then rename into place.
- A corrupt line must not make `status.json` unreadable; readers should surface
  an explicit evidence-artifact error and continue to load current feature state.

## 6. Acceptance Criteria

1. A reconcile run that writes a non-empty outcome also writes
   `artifacts/reconcile-evidence.jsonl`.
2. The artifact records raw verdict, phase, evidence kind, confidence, upstream
   ref/commit, and enough path/hash/operation metadata for review.
3. No raw source snippets, provider transcripts, or embeddings are persisted.
4. Existing status round-trips remain backward-compatible when the feature has no
   evidence artifact.
5. Patch-id matches populate evidence from the existing `PatchIDMatch` data.
6. Corrupt evidence artifacts fail with an explicit warning/error and do not
   prevent `status.json` from loading.
7. Evidence writes are atomic and never leave a partially written final file on
   successful return.
8. Re-running the same reconcile evidence write produces byte-identical JSONL
   lines, including stable key order and sorted arrays.
9. Unknown future `schema_version` values are rejected with a forward-compatible
   error that preserves the raw file.
10. Privacy tests assert that raw source content, provider transcripts, prompt
    text, and vector embeddings do not appear in the evidence artifact.
11. The `phase` and `evidence_kind` values are validated against the v1 enum
    sets, while allowing explicitly marked future extension handling.

## 7. Open Questions

- Should low-confidence evidence change the default terminal output color or
  only the wording?

## Disputes

None yet.
