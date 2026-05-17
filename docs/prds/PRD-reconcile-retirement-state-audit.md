# PRD - Reconcile Retirement State Audit - `feat-reconcile-retirement-state-audit`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: [PRD-upstreamed-confirmation-gate](./PRD-upstreamed-confirmation-gate.md). Implementation requires `ADR-025-reconcile-evidence-and-revision-schema`.

## Related

- [Feature dependencies PRD](./PRD-feature-dependencies.md)
- [Reconcile Workflow](../reconcile.md)
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
| `LabelDependentBroken` exists as a derived overlay for stale dependency/base SHA reachability. | `internal/store/types.go`. |
| `Dependency.SatisfiedBy` stores a SHA when a parent reaches `upstream_merged`. | `internal/store/types.go`. |
| The t3code review found `toast-close-button` genuinely upstreamed but still associated with stale `dependent-broken` cleanup. | t3code `local-notes.md`. |

No code or schema is changed by this PRD.

## Summary

After a feature is confirmed as upstreamed or retired, run a focused audit that
checks whether dependent features, `satisfied_by` SHAs, stale base commits, and
derived labels still make sense.

## 1. Problem Statement

Correctly identifying a feature as upstreamed does not guarantee the surrounding
metadata is clean. The t3code study showed a separate cleanup problem:
`toast-close-button` was genuinely absorbed upstream, but reviewers still saw
stale dependency/status metadata.

Retirement should be treated as a two-step process:

1. Confirm the feature can be retired.
2. Audit the graph and status overlays that retirement affects.

## 2. Goals / Non-goals

### Goals

1. Detect stale dependency or base SHA references after feature retirement.
2. Surface children whose labels change because a parent became upstreamed.
3. Warn when `dependent-broken` remains after a confirmed upstream absorption.
4. Provide a safe audit report before any fix-up writes occur.
5. Feed revision-pass logs with cleanup-needed actions.

### Non-goals

1. No automatic dependency rewrites in v1.
2. No deletion of feature directories.
3. No replacement for dependency validation.
4. No new dependency edge types.

## 3. User-facing Contract

Proposed command:

```bash
tpatch reconcile audit-retirement <slug> [--json]
```

Behavior is read-only. Example output:

```text
retirement audit: toast-close-button
  feature state: upstream_merged
  dependent metadata: stale satisfied_by f6e4378
  children affected: 2
  action: cleanup-needed
```

The audit also runs automatically after `confirm-upstreamed`. Automatic audit
prints findings and may append `cleanup-needed` revision-pass entries, but never
mutates dependency or status metadata.

## 4. Audit Checks

The audit should check:

1. Feature state and raw/review evidence agree that retirement was confirmed.
2. `Dependency.SatisfiedBy` SHAs are reachable from the current HEAD.
3. Child features derive expected labels after parent retirement.
4. `dependent-broken` labels are either justified or cleared by current state.
5. The feature has a revision-pass log entry for the retirement action.

## 5. Implementation Notes

- The audit should reuse existing label composition rather than persist new
  label fields.
- `feat-reconcile-audit-retirement-fixer` is a follow-up backlog item for any
  future fixer. This PRD is read-only only.

## 6. Acceptance Criteria

1. The audit reports stale `satisfied_by` or base SHAs after retirement.
2. The audit identifies child features affected by the retired parent.
3. The audit can run without mutating feature state.
4. JSON output is stable enough for case-study validation.
5. `confirm-upstreamed` runs the audit automatically after confirmation and
   prints any cleanup-needed findings.
6. No mutation path is included in v1.

## 7. Open Questions

- Which cleanup findings should become hard blockers for future auto-drop flows?

## Disputes

None yet.
