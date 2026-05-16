# PRD - Reconcile Path Restructure Detector - `feat-reconcile-path-restructure-detector`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: [PRD-reconcile-blocked-verdict-taxonomy](./PRD-reconcile-blocked-verdict-taxonomy.md). Implementation requires `ADR-024-reconcile-evidence-and-revision-schema`.

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
| The t3code study's true conflicts correlated with upstream directory restructure. | t3code `summary.md` and `local-notes.md`. |
| Reconcile currently does not persist path-prefix restructure evidence. | `internal/store/types.go` `ReconcileSummary`. |
| The research packet identifies structural anchors and path movement as middle-pass signals. | `patch-identity-and-structural-fingerprints.md`; `tpatch-middle-pass-synthesis.md`. |

No code or schema is changed by this PRD.

## Summary

Detect likely upstream path-prefix restructures before treating failures as
ordinary hunk conflicts. This gives operators a clear reason when a feature's
old path domain has moved or split.

## 1. Problem Statement

The t3code study's true conflicts were not random line conflicts. Upstream
restructured a desktop source tree from one layout into multiple subdirectories.
Features that added files and IPC channels to the old layout needed a real
rewrite.

Path restructure detection is cheaper than AST or vector matching and can
explain a class of true conflicts early.

## 2. Goals / Non-goals

### Goals

1. Detect directory-prefix moves, splits, and deletions between base and target
   upstream.
2. Mark feature paths affected by those restructures.
3. Feed the blocked taxonomy with `structural-conflict` or `path-moved`
   evidence.
4. Suggest candidate new prefixes when Git rename/copy evidence is strong.

### Non-goals

1. No automatic path migration in v1.
2. No AST-based move detection.
3. No guarantee that suggested prefixes are semantically correct.
4. No vector search.

## 3. Detector Contract

Input:

- feature patch paths;
- upstream diff name-status between base and target;
- optional Git rename/copy detection output.

Output evidence:

```json
{
  "evidence_kind": "path-restructure",
  "classification": "prefix-split",
  "old_prefix": "apps/desktop/src/",
  "candidate_prefixes": [
    "apps/desktop/src/app/",
    "apps/desktop/src/backend/",
    "apps/desktop/src/electron/"
  ],
  "affected_feature_paths": ["apps/desktop/src/ManagedEnvironment.ts"]
}
```

Classifications:

- `none`
- `prefix-move`
- `prefix-split`
- `target-deleted`
- `mixed`
- `unknown`

Threshold defaults:

- `prefix-split`: at least 3 files moved to at least 2 distinct new prefixes.
- `prefix-move`: at least 5 files moved to one new prefix.

The thresholds are tunable; v1 should expose them in evidence output so later
studies can judge whether they are too sensitive.

## 4. Reconcile Behavior

- Prefix restructure evidence should turn a generic blocked verdict into
  `structural-conflict` or `target-deleted`.
- Candidate prefixes should be hints only.
- Provider-assisted resolution may use the evidence to reduce context, but this
  PRD does not require provider integration.

## 5. Implementation Notes

- Start with Git name-status and path-prefix counts.
- Use thresholds to avoid over-reporting tiny path churn as restructure.
- Do not persist source snippets.
- Cap candidate prefix output at 5 entries, sorted by support count descending,
  then path ascending.

## 6. Acceptance Criteria

1. The detector reports when feature paths fall under an upstream-renamed or
   split prefix.
2. The blocked taxonomy can consume restructure evidence.
3. Output includes old prefix, candidate prefixes, affected paths, and
   confidence.
4. The detector can run without language parsers or a provider.
5. Candidate prefix output is capped at 5 entries and deterministically sorted.
6. Prefix-move and prefix-split thresholds use the documented defaults unless an
   explicit configuration changes them.

## 7. Open Questions

- Should candidate prefixes be allowed to seed a future relocation planner?

## Disputes

None yet.
