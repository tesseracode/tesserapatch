# PRD - Reconcile Revision-pass Log - `feat-reconcile-revision-pass-log`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: [PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md). Implementation requires `ADR-024-reconcile-evidence-and-revision-schema`.

## Related

- [Reconcile Workflow](../reconcile.md)
- [t3code local notes](../state-of-the-art/case-studies/t3code-upstream-v0.0.23-2026-05/local-notes.md)
- [PRD-upstreamed-confirmation-gate](./PRD-upstreamed-confirmation-gate.md)
- [PRD-reconcile-retirement-state-audit](./PRD-reconcile-retirement-state-audit.md)
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
| Current `status.json` tracks current reconcile state, not a structured correction history. | `internal/store/types.go` `FeatureStatus` and `ReconcileSummary`. |
| The t3code study required post-review corrections for false-positive upstreamed verdicts. | t3code `summary.md` and `local-notes.md`. |
| Case-study counts drifted before post-review correction. | t3code `local-notes.md` data-quality notes. |

No code or schema is changed by this PRD.

## Summary

Add a structured revision-pass log so review corrections are first-class records
instead of prose in summaries. The log links raw machine verdicts to review
decisions, actions taken, final states, and validation results.

## 1. Problem Statement

Reconciliation is iterative. A raw run may say `blocked` or `upstreamed`; a
reviewer may then confirm, reject, reapply, implement missing pieces, or defer.
Today those steps are captured ad hoc in prose or commit messages. That makes it
hard to reconstruct why counts changed.

The t3code case study needed a post-review action log, feature label fixes, and
metric corrections. Future studies should not require archaeology.

## 2. Goals / Non-goals

### Goals

1. Record per-feature review corrections after reconcile.
2. Preserve raw verdict, review verdict, final state, and action taken.
3. Link corrections to evidence attempts and validation commands.
4. Support both feature-local review and aggregate case-study validation.
5. Make revision passes repeatable by agents without losing prior decisions.

### Non-goals

1. No implementation of the repair itself.
2. No replacement for Git history or feature `status.json`.
3. No raw source-code diff bodies in the log.
4. No requirement that every repo produce formal case-study artifacts.

## 3. Proposed Artifact

Add a per-feature artifact:

```text
.tpatch/features/<slug>/artifacts/reconcile-revisions.jsonl
```

One JSON object per correction:

```json
{
  "schema_version": 1,
  "feature_slug": "session-search",
  "evidence_attempt": "sha256:...",
  "raw_reconcile_verdict": "upstreamed",
  "review_verdict": "false-positive-upstreamed",
  "final_feature_state": "applied",
  "action_taken": "reapplied-and-recorded",
  "reason_code": "false-positive-upstreamed",
  "validation_refs": [
    {"kind": "test-command", "value": "bun run typecheck", "result": "pass"}
  ]
}
```

Allowed `review_verdict` values:

- `confirmed`
- `false-positive`
- `false-negative`
- `inconclusive`
- `deferred`

Allowed `action_taken` values:

- `none`
- `confirmed-retired`
- `reapplied`
- `reapplied-and-recorded`
- `implemented`
- `deferred`
- `skipped`
- `cleanup-needed`

Allowed `reason_code` values include:

- `false-positive-upstreamed`
- `false-negative-blocked`
- `missing-upstream-commit-ref`
- `match-origin-sibling-feature`
- `dropped-pre-reconcile-code`
- `cleanup-needed`
- `manual-review`

Free-text notes are deferred to `ADR-capture-context-privacy-boundary`; v1 uses
enumerated reason codes only.

## 4. User-facing Contract

Proposed commands:

```bash
tpatch reconcile review add <slug> --verdict false-positive --action reapplied-and-recorded --reason-code false-positive-upstreamed
tpatch reconcile review list <slug> [--json]
```

The command is audit-only. It records what happened; it does not perform the
repair.

## 5. Implementation Notes

- The log should be append-only unless a future ADR defines rewrite policy.
- Entries should link to evidence attempt IDs when available.
- If the entry changes the feature state, the state change still belongs in
  `status.json`; the log explains why.
- JSONL is preferred because review corrections are naturally append events.
- A bad JSONL line in the middle of the file is reported with line number. Human
  output skips unreadable trailing summaries; `--json` returns a structured
  `corrupt_entries` array and exits non-zero.
- List output deduplicates by `(feature_slug, evidence_attempt, review_verdict,
  action_taken)` and keeps the last valid entry by file order.
- Supersedure is explicit: a later entry may include `supersedes_entry_id`; old
  entries remain in the file.
- `PRD-reconcile-retirement-state-audit` is the canonical producer of
  `action_taken=cleanup-needed`.

## 6. Acceptance Criteria

1. A reviewer can record a correction with raw verdict, review verdict, action,
   final state, and validation refs.
2. The log is deterministic and contains no raw source bodies or transcripts.
3. `--json` output can summarize the latest review decision for a feature.
4. Revision-pass entries can be consumed by the study validator PRD.
5. Corrupt JSONL entries are reported without losing valid entries before or
   after the corrupt line.
6. Superseded entries remain readable and are omitted from latest-summary output
   unless `--all` is requested.
7. V1 entries use enumerated reason codes and do not persist free-text notes.

## 7. Open Questions

- Should aggregate revision summaries live repo-wide, or be derived from
  per-feature logs?

## Disputes

None yet.
