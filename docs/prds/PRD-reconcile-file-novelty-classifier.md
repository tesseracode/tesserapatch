# PRD - Reconcile File Novelty Classifier - `feat-reconcile-file-novelty-classifier`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: [PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md). Implementation requires `ADR-025-reconcile-evidence-and-revision-schema`.

## Related

- [Reconcile Workflow](../reconcile.md)
- [Structural middle-pass synthesis](../state-of-the-art/tpatch-middle-pass-synthesis.md)
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
| Reconcile currently has a forward-apply phase that may classify failures as blocked. | `docs/reconcile.md`. |
| The t3code study found many blocked verdicts were actually additive/new-file or clean reapply cases. | t3code `summary.md` and `local-notes.md`. |
| `ApplySummary` records whether a feature has a patch but not file novelty categories. | `internal/store/types.go` `ApplySummary`. |

No code or schema is changed by this PRD.

## Summary

Classify feature patch paths as new-file, modified-existing-file, deleted-file,
or mixed before treating a failed apply as blocked. Additive/new-file features
should enter a safer review path than true conflicting edits.

## 1. Problem Statement

In the t3code study, many false `blocked` verdicts came from patches that were
mostly additive: new files, new provider entries, and schema extensions. A
middle-pass does not need ASTs or embeddings to identify the simplest case:
"this patch creates paths that upstream did not touch."

Without file novelty, tpatch collapses additive features and destructive edits
into the same blocked bucket.

## 2. Goals / Non-goals

### Goals

1. Detect whether each feature patch path is new, existing, deleted, or moved in
   the target upstream tree.
2. Mark all-new-file patches as clean-additive candidates when path safety
   passes.
3. Record file novelty evidence for later blocked taxonomy and review.
4. Avoid provider calls for obvious additive non-overlap cases when safe.

### Non-goals

1. No AST parsing.
2. No symbol-level relocation.
3. No auto-commit or auto-record behavior.
4. No attempt to prove semantic correctness of new files.

## 3. Classifier Contract

Input:

- feature patch file list and file status from `post-apply.patch`;
- old base commit;
- target upstream commit;
- current worktree path state.

Output evidence:

```json
{
  "evidence_kind": "file-novelty",
  "paths": [
    {"path": "src/CopilotDriver.ts", "feature_action": "create", "upstream_state": "absent"},
    {"path": "src/providers.ts", "feature_action": "modify", "upstream_state": "present"}
  ],
  "classification": "mixed-additive"
}
```

Classifications:

- `all-new-files`
- `mixed-additive`
- `modifies-existing-files`
- `deletes-or-renames`
- `unknown`

Boundary cases:

- Create + modify in one feature is `mixed-additive`.
- Any rename or delete action is `deletes-or-renames`.
- Binary patches use the same action-based classification rules as text patches;
  file type does not create a separate v1 category.
- Generated or ignored paths are classified by patch action, not by path type.

## 4. Reconcile Behavior

This PRD should not immediately mark a feature applied. It should change
diagnostics and evidence:

- `all-new-files` with no upstream path conflict becomes a high-confidence
  clean-additive candidate.
- `mixed-additive` proceeds to hunk-overlap detection for modified paths.
- path collisions remain blocked or require human review.

## 5. Implementation Notes

- Use Git diff metadata and patch headers before reading file bodies.
- Classify by patch action (`create`, `modify`, `delete`, `rename`), not by path
  type or language.
- Sort path entries deterministically.
- Store results in the verdict evidence artifact, not as a new lifecycle state.
- The classifier should run before expensive provider phases when data is
  available.

## 6. Acceptance Criteria

1. Reconcile can report file novelty categories for a feature patch.
2. New-file-only patches with no target path collision are not described as
   generic blocked failures.
3. File novelty evidence is available in JSON output.
4. No raw file contents are persisted.
5. Create+modify, rename, and binary patch boundary cases are covered by tests
   or fixtures.

## 7. Open Questions

- Should `all-new-files` be allowed to bypass provider analysis in v1, or only
  lower the severity of a blocked verdict?

## Disputes

None yet.
