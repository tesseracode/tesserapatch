# PRD - Reconcile Hunk Overlap Detector - `feat-reconcile-hunk-overlap-detector`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: [PRD-reconcile-file-novelty-classifier](./PRD-reconcile-file-novelty-classifier.md). Implementation requires `ADR-025-reconcile-evidence-and-revision-schema`.

## Related

- [Patch identity and structural fingerprints](../state-of-the-art/patch-identity-and-structural-fingerprints.md)
- [Search-based patch application](../state-of-the-art/search-based-patch-application.md)
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
| The t3code study found blocked false positives where hunks had shifted insertion points rather than true overlaps. | t3code `summary.md` and `local-notes.md`. |
| Current status can store conflicts but not hunk-level overlap evidence. | `internal/store/types.go` `ReconcileSummary`. |
| The experiment guide records hunk-level units. | `experiment-guide-structural-middle-pass.md`. |

No code or schema is changed by this PRD.

## Summary

Compare feature patch hunks against upstream diff hunks to decide whether a
blocked apply is likely a true edit conflict or only shifted context. This is a
deterministic middle-pass signal, not a semantic engine.

## 1. Problem Statement

When upstream changes near a patch's context lines, `git apply` may fail even if
the feature remains valid. Humans can often see that the insertion point moved.
The first case study found this pattern repeatedly.

Before invoking an LLM, tpatch can ask a cheaper question: did upstream actually
edit the same lines or surrounding hunk region that the feature edits?

## 2. Goals / Non-goals

### Goals

1. Map feature patch hunks to path and approximate line ranges in the old base.
2. Map upstream changes between old base and target upstream.
3. Classify overlap as none, context-only, edit-overlap, target-deleted, or
   unknown.
4. Feed blocked taxonomy and relocation planning with hunk-level evidence.

### Non-goals

1. No AST or CFG analysis.
2. No automatic relocation in v1.
3. No semantic equivalence guarantee.
4. No vector index.

## 3. Detector Contract

Input:

- feature patch hunks;
- upstream diff from base commit to target upstream;
- path novelty results.

Output evidence:

```json
{
  "evidence_kind": "hunk-overlap",
  "classification": "context-only",
  "hunks": [
    {
      "path": "src/providers.ts",
      "feature_hunk": "@@ -42,6 +42,7 @@",
      "overlap": "none",
      "upstream_hunks_nearby": 1
    }
  ]
}
```

Classifications:

- `none`
- `context-only`
- `edit-overlap`
- `target-deleted`
- `path-moved`
- `unknown`

## 4. Reconcile Behavior

- `none` or `context-only` should downgrade a generic blocked verdict to a
  relocation candidate.
- `edit-overlap` should remain a stronger conflict signal.
- `target-deleted` and `path-moved` should feed the path restructure detector.
- The nearby window defaults to +/- 3 lines, matching Git's default context
  size. This value is tunable by implementation if later studies justify it.

## 5. Implementation Notes

- Use Git's diff ranges and patch headers first; do not parse language syntax.
- Store hunk IDs or hashes rather than raw hunk bodies where possible.
- Keep output deterministic by sorting paths and hunks.
- This detector should be safe in `--no-provider` mode.
- Complexity is O(N*M) line-range intersection where N is feature hunks and M is
  upstream hunks for the relevant paths. That is acceptable for v1 because real
  studies are expected to stay below N < 100 and M < 1000.

## 6. Acceptance Criteria

1. Reconcile can emit hunk overlap evidence for modified paths.
2. Non-overlapping modified paths are distinguished from true edit overlap.
3. The detector output can explain why a blocked feature is a relocation
   candidate.
4. No raw source bodies are persisted.
5. The default nearby window is +/- 3 lines and appears in JSON output.

## 7. Open Questions

- Should overlap be line-range-only in v1, or include context-line hashes?

## Disputes

None yet.
