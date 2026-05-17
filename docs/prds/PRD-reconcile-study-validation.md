# PRD - Reconcile Study Validation - `feat-reconcile-study-validation`

**Status**: Approved
**Date**: 2026-05-15
**Owner**: Core
**Byline**: T56
**Milestone**: Reconcile safety and middle-pass foundation. Not yet roadmap-committed.
**Depends on**: [PRD-reconcile-revision-pass-log](./PRD-reconcile-revision-pass-log.md). Implementation requires `ADR-025-reconcile-evidence-and-revision-schema`.

## Related

- [Structural middle-pass experiment guide](../state-of-the-art/experiment-guide-structural-middle-pass.md)
- [Case-study index](../state-of-the-art/case-studies/README.md)
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
| The experiment guide defines study, feature, hunk, patch, and candidate-attempt units. | `experiment-guide-structural-middle-pass.md`. |
| The imported t3code study uses `study.json`, `features.jsonl`, `hunks.jsonl`, `patches.jsonl`, `metrics.json`, and `summary.md`. | `case-studies/t3code-upstream-v0.0.23-2026-05/`. |
| The first import had count drift that required correction and notes. | t3code `local-notes.md`. |

No code or schema is changed by this PRD.

## Summary

Add validation for reconcile case-study artifacts so raw verdict counts,
post-review labels, final states, and aggregate metrics cannot drift silently.

## 1. Problem Statement

The first case study was valuable, but its counters changed as reviewers found
false positives and the reconciliation agent took follow-up actions. That is
normal. The problem is that raw verdicts, ground truth, and final states answer
different questions but can look like one set of totals.

Before using studies as PRD evidence, tpatch should validate that each counter
has a defined phase and that aggregate files agree with record-level files.

## 2. Goals / Non-goals

### Goals

1. Validate case-study JSON and JSONL shapes.
2. Cross-check `study.json`, `metrics.json`, and `features.jsonl` counts.
3. Distinguish raw reconcile verdicts, post-review ground truth, and final
   feature states.
4. Detect missing revision-pass entries for corrected verdicts.
5. Produce stable JSON output for research review.

### Non-goals

1. No requirement that normal tpatch users produce case studies.
2. No source-code content validation.
3. No automatic correction of counters.
4. No full experiment runner.

## 3. Proposed Tooling

V1 is a dev-only test helper under an `internal/tools/` package, not a public
CLI surface. The helper may expose a small command for maintainers, but it is
not part of `SPEC.md` until at least two repositories produce comparable case
studies.

`<path>` points at a case-study folder containing:

- `study.json`
- `features.jsonl`
- `hunks.jsonl`
- `patches.jsonl`
- `metrics.json`
- `summary.md`

The public CLI candidate remains backlog:

```bash
tpatch case-study validate <path> [--json]
```

## 4. Validation Rules

Minimum v1 checks:

1. Every JSON and JSONL record parses.
2. `study_id` is consistent across all machine-readable files.
3. Feature counts in `study.json` match the number of `features.jsonl` rows.
4. Aggregate ground-truth counts in `metrics.json` match record-level
   `ground_truth` values.
5. Every false-positive or false-negative ground-truth label has either a
   revision-pass entry or a documented notes reference.
6. Raw verdict counts are not compared directly to final state counts unless the
   metric declares which phase it represents.

`documented notes reference` means a `local-notes.md` file in the case-study
directory, following the t3code convention. Inline prose in `summary.md` alone
is not enough for v1 validation once this helper exists.

## 5. Implementation Notes

- Validation should be stdlib-only.
- Do not require access to the target repo; validate the study artifact only.
- Warnings are acceptable for prose-only discrepancies; parse failures and count
  contradictions are errors.
- The validator should be used before promoting a study finding into a PRD.

## 6. Acceptance Criteria

1. The validator reports malformed JSON/JSONL with filename and line number.
2. The validator catches mismatched aggregate counts.
3. The validator distinguishes raw verdicts from post-review labels and final
   states.
4. The validator can run on the t3code study artifacts.
5. The first implementation is dev-only under `internal/tools/` and does not add
   a public CLI namespace.
6. Missing `local-notes.md` is a warning for old studies and an error for new
   studies that record corrected verdicts.

## 7. Open Questions

- Should future studies require a machine-readable revision log, or allow prose
  notes when the study predates the log PRD?

## Disputes

None yet.
