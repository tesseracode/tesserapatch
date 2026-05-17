# PRD - Reconcile Blocked Verdict Taxonomy - `feat-reconcile-blocked-taxonomy`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: [PRD-reconcile-hunk-overlap-detector](./PRD-reconcile-hunk-overlap-detector.md). Implementation requires `ADR-025-reconcile-evidence-and-revision-schema`.

## Related

- [Reconcile Workflow](../reconcile.md)
- [PRD-reconcile-file-novelty-classifier](./PRD-reconcile-file-novelty-classifier.md)
- [PRD-reconcile-hunk-overlap-detector](./PRD-reconcile-hunk-overlap-detector.md)
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
| Reconcile has a generic `blocked` outcome and newer blocked variants for resolver paths. | `internal/store/types.go` `ReconcileOutcome`. |
| Current operator docs describe blocked troubleshooting but not a full taxonomy of additive, shifted, deleted, or restructure cases. | `docs/reconcile.md`. |
| The t3code study split blocked features into clean reapply, relocation, and true conflict. | t3code `summary.md` and `local-notes.md`. |

No code or schema is changed by this PRD.

## Summary

Replace generic "blocked" presentation with a review taxonomy that separates
clean-additive false positives, shifted-context relocation, target deletion,
structural conflict, dependency blockage, and unknown failures.

## 1. Problem Statement

`blocked` is too broad to drive a good next action. In the first case study,
most blocked verdicts were not true conflicts. They were either clean-additive
or minor relocation cases.

Operators and agents need to know which type of blocked they are seeing before
choosing between reapply, relocate, provider help, dependency cleanup, or manual
rewrite.

## 2. Goals / Non-goals

### Goals

1. Define stable blocked subcategories.
2. Derive subcategories from file novelty, hunk overlap, dependency labels, and
   path restructure evidence.
3. Preserve existing lifecycle states while improving output and JSON evidence.
4. Give each category a recommended next action.

### Non-goals

1. No automatic repair.
2. No new top-level lifecycle states in v1.
3. No replacement for provider-assisted resolution.
4. No broad structural search planner.
5. No auto-accept of `clean-additive`; that belongs in a follow-up PRD if the
   classifier proves safe enough.

## 3. Taxonomy

| Category | Meaning | Recommended next action |
|---|---|---|
| `clean-additive` | Patch creates new files or non-overlapping additions. | Reapply or accept deterministic apply. |
| `shifted-context` | Target still exists; context moved without edit overlap. | Try relocation/anchor matching. |
| `edit-overlap` | Upstream edited the same region. | Human/provider resolution. |
| `target-deleted` | Patch target no longer exists. | Check path restructure or rewrite. |
| `structural-conflict` | Upstream moved/restructured the feature's path domain. | Manual rewrite or path migration. |
| `dependency-blocked` | Parent state prevents meaningful reconcile. | Reconcile/repair parent first. |
| `validation-blocked` | Apply seemed possible but validation failed. | Inspect tests/typecheck. |
| `unknown-blocked` | Insufficient evidence. | Manual review. |

When multiple categories apply, v1 uses this deterministic precedence:

```text
dependency-blocked > validation-blocked > target-deleted >
structural-conflict > edit-overlap > shifted-context >
clean-additive > unknown-blocked
```

## 4. User-facing Contract

Example:

```text
desktop-managed-environments-connections: blocked (structural-conflict)
  evidence: path-prefix restructure apps/desktop/src -> apps/desktop/{app,backend,electron}
  next: manual rewrite or path migration
```

JSON output should expose both the raw outcome and the blocked category:

```json
{
  "outcome": "blocked",
  "blocked_category": "shifted-context",
  "recommended_action": "try-relocation"
}
```

## 5. Implementation Notes

- Store blocked category as evidence metadata, not as a new lifecycle state.
- Programmatic decisions should still read raw outcome plus labels separately.
- Categories must be deterministic and sorted when multiple apply; v1 can expose
  a primary category plus secondary evidence.

## 6. Acceptance Criteria

1. Generic blocked output is enriched with a deterministic category when
   supporting evidence exists.
2. Unknown cases remain `unknown-blocked` rather than pretending certainty.
3. JSON output exposes raw outcome, category, and recommended action.
4. Existing status files remain readable without category evidence.
5. Multi-category cases choose the primary category using the documented
   precedence order and include secondary evidence separately.

## 7. Open Questions

- Should category names become persisted enum values, or stay evidence-only?

## Disputes

None yet.
