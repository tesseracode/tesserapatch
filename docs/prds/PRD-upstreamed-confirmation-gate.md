# PRD - Upstreamed Confirmation Gate - `feat-upstreamed-confirmation-gate`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: [PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md). Implementation requires `ADR-025-reconcile-evidence-and-revision-schema`.

## Related

- [Reconcile Workflow](../reconcile.md)
- [Patch-already-upstream detector](./PRD-patch-already-upstream-detector.md)
- [t3code local notes](../state-of-the-art/case-studies/t3code-upstream-v0.0.23-2026-05/local-notes.md)
- [WP-003 Reconcile Safety and Middle-pass](../whitepapers/WP-003-reconcile-safety-and-middle-pass.md)

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
| `StateUpstreamMerged` and `ReconcileUpstreamed` already exist. | `internal/store/types.go`. |
| Dependencies can store `satisfied_by` when a parent reaches `upstream_merged`. | `internal/store/types.go` `Dependency.SatisfiedBy`. |
| The t3code study found two false-positive `upstreamed` verdicts in one reconcile. | t3code `local-notes.md`. |
| Existing patch-id match metadata can name an upstream commit, but operation/provider verdicts may not. | `PatchIDMatch.MatchedUpstreamSHA`; `ReconcileSummary` has no general upstream commit refs. |

No code, schema, command behavior, or asset text is changed by this PRD.

## Summary

Treat `upstreamed` as a candidate verdict until it is confirmed with evidence
that the feature intent exists upstream. Confirmation, not raw reconcile output,
is what may retire a local feature or satisfy child dependencies.

## 1. Problem Statement

An incorrect `upstreamed` verdict is more dangerous than an incorrect `blocked`
verdict. A false `blocked` costs manual work. A false `upstreamed` can silently
drop local feature code.

The t3code study shows both failure modes:

- `session-search` was present before reconcile and dropped by a false
  `upstream_merged` transition.
- `copilot-skill-controls` was not fully implemented, but the reconciler
  matched sibling scaffolding and called the feature upstreamed.

Both cases needed human correction after the fact. The default workflow should
make that confirmation explicit before retirement.

## 2. Goals / Non-goals

### Goals

1. Prevent automatic retirement on low-confidence or unconfirmed `upstreamed`
   verdicts.
2. Require evidence that the upstream tree contains the feature intent, not just
   matching strings.
3. Record who or what confirmed/rejected the candidate verdict.
4. Preserve existing deterministic fast paths by allowing high-confidence
   patch-id matches to be confirmed cheaply.
5. Keep confirmation reviewable in `.tpatch/`.

### Non-goals

1. No semantic equivalence engine in this PRD.
2. No forced provider call for every `upstreamed` candidate.
3. No change to feature deletion semantics beyond the confirmation gate.
4. No new dependency model; this consumes existing dependency fields.

## 3. User-facing Contract

When reconcile believes a feature is upstreamed, it emits:

```text
session-search: upstreamed-candidate (confirmation required)
  evidence: phase-2 recipe-operation-match, confidence=low
  next: tpatch reconcile confirm-upstreamed session-search --upstream-commit <sha>
        or tpatch reconcile reject-upstreamed session-search --reason-code false-positive-upstreamed
```

Proposed commands:

```bash
tpatch reconcile confirm-upstreamed <slug> --upstream-commit <sha> [--evidence <attempt-id>]
tpatch reconcile reject-upstreamed <slug> --reason-code <code> [--evidence <attempt-id>]
```

Rules:

- Confirmation requires an evidence attempt.
- Confirmation requires at least one upstream commit reference unless the
  evidence kind is a deterministic patch-id match with a stored
  `matched_upstream_sha`.
- Rejection leaves the feature in a non-retired state and records the rejection
  for the revision-pass log.
- Raw reconcile output alone cannot update hard dependents as satisfied.

## 4. State Semantics

V1 should distinguish:

| Concept | Meaning |
|---|---|
| `raw_reconcile_verdict=upstreamed` | Machine candidate. Not enough to retire. |
| `review_verdict=confirmed-upstreamed` | Human or high-confidence deterministic confirmation. |
| `review_verdict=rejected-upstreamed` | Candidate was false positive or inconclusive. |
| `final_feature_state=upstream_merged` | Local feature is safely retired after confirmation. |

V1 storage:

- `review_verdict` becomes a new field on `status.json:reconcile`
  (`ReconcileSummary`) under ADR-025.
- `final_feature_state` reuses existing `status.json:state`; no parallel final
  state field is introduced.
- The revision-pass log stores the correction event that explains why
  `review_verdict` changed.

## 5. Implementation Notes

- Patch-id matches auto-confirm when `matched_upstream_sha` is reachable from
  the target upstream range.
- Patch-id matches require manual confirmation when reachability cannot be
  proven.
- Operation-level matches should default to low or medium confidence unless
  match origin is proven upstream.
- Provider-semantic matches should still require evidence references; the model
  response alone is not a commit reference.
- The confirmation gate should be compatible with future `--auto-drop-merged`
  behavior by making that flag mean "auto-confirm only high-confidence evidence."

## 6. Acceptance Criteria

1. A non-confirmed `upstreamed` verdict is displayed as `upstreamed-candidate`.
2. A candidate does not retire local patches, update child `satisfied_by`, or
   mark final state `upstream_merged` until confirmed.
3. Confirming records evidence ID, upstream commit ref, and final state change.
4. Rejecting records a review verdict and leaves the feature available for
   reapply, repair, or manual implementation.
5. Existing repositories without evidence artifacts remain readable.
6. Rejecting an upstreamed candidate writes `review_verdict=rejected-upstreamed`
   and does not mutate `status.json:state` to `upstream_merged`.
7. Rejecting requires an enumerated reason code and appends a revision-pass log
   entry.
8. Reachable patch-id matches can auto-confirm; unreachable or missing upstream
   SHAs require manual confirmation.

## 7. Open Questions

- Should the confirmation commands live under `tpatch reconcile`, `tpatch
  feature`, or both as aliases?

## Disputes

None yet.
