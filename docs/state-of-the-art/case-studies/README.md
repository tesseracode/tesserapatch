# Case Studies — Structural Middle-pass and Workflow Evidence

**Status**: Living case-study index
**Date**: 2026-05-15
**Owner**: Core
**Related**: [Experiment guide](../experiment-guide-structural-middle-pass.md),
[tpatch middle-pass synthesis](../tpatch-middle-pass-synthesis.md),
[Research roadmap](../research-roadmap.md)

## Refresh triggers

- A new reconcile or historical workflow study is captured.
- A study's ground-truth labels are corrected after manual review.
- A PRD/ADR consumes a study finding as implementation evidence.

## Index

| Study | Study type | Records | Key signal |
|---|---|---|---|
| [`t3code-upstream-v0.0.23-2026-05`](t3code-upstream-v0.0.23-2026-05/) | Upstream-transition reconcile study | 25 features, 53 hunks, 5 patch summaries, [local notes](t3code-upstream-v0.0.23-2026-05/local-notes.md), and a [dependency analysis](t3code-upstream-v0.0.23-2026-05/dependency-analysis.md) addendum | Both `upstreamed` verdicts were false positives, and 13 of 15 `blocked` verdicts were false positives. The dependency addendum is qualitative planning input, not new runtime proof of hard edges. |
| [`tws-dev-2026-05`](tws-dev-2026-05/) | Aggregate historical development-workflow study | [`study.json`](tws-dev-2026-05/study.json), [`metrics.json`](tws-dev-2026-05/metrics.json), [`summary.md`](tws-dev-2026-05/summary.md), [dependency analysis](tws-dev-2026-05/dependency-analysis.md), and [local notes](tws-dev-2026-05/local-notes.md) only | Early dependency discovery and smoke-test evidence were useful during greenfield Path B work, but the import has no `features.jsonl` / `hunks.jsonl` / `patches.jsonl`, so it is not a normal reconcile-validator corpus. |
| [`adjacent-cli-args-conflict-2026-08`](adjacent-cli-args-conflict-2026-08/) | Synthetic reconcile reproduction | 4 asserted Git variants + tracked tpatch replay/safety trial | Merge and rebase conflict identically for the adjacent fixtures; an anchor-based operation produces a clean candidate, but current reconcile does not replay applicable operations and current recipe semantics require additional safety gates. |
| [`copilot-api-cumulative-verify-2026-08`](copilot-api-cumulative-verify-2026-08/) | Long-lived cumulative verification study | 56 features, exact v0.15.1 verify/doctor reproduction, 38 own-base patch probes | Repository gates pass while verify reports 0 pass / 53 fail / 3 skip; 29 of 38 V8 failures replay on their recorded base, and four recent V10 failures are missing manual provenance rather than measured stale hashes. |

## Import notes

Imported studies do **not** all have the same record layout. This index keeps
that distinction explicit.

The t3code study stores metadata, hashes, paths, feature labels, and aggregate
outcomes. It does not store raw source bodies. The 2026-05-15 import includes
the post-review action log plus local corrections: `session-search` was working
before reconcile, incorrectly retired, and then re-applied;
`copilot-skill-controls` was implemented server-side after its false upstreamed
verdict; and `toast-close-button` was confirmed as already upstreamed but still
needs stale dependency/status cleanup.

The TWS study is intentionally **not** a reconcile import. It is an aggregate
historical development-workflow snapshot: summary + metrics + local notes +
derived dependency analysis. No per-feature `features.jsonl`, `hunks.jsonl`, or
`patches.jsonl` were imported here, so the normal reconcile-study validator does
not apply. The summary date window and `study.json.duration_days` also disagree;
both are preserved as historical evidence rather than silently normalized.

The adjacent-argument study is a synthetic reproduction rather than an imported
production dataset. Its script creates disposable repositories and stores no
source outside the fixture itself.

The copilot-api cumulative-verification study records only aggregate results,
artifact metadata and source-contract findings. Raw downstream source bodies
and full verifier output remain outside this repository.

## Candidate PRD signals

The first study graduated to the nine-PRD WP-003 cluster, now shipped as recorded
in [ROADMAP](../../ROADMAP.md). These are historical consumers of its evidence,
not nine newly authorized implementation tasks:

1. [Reconcile verdict evidence](../../prds/PRD-reconcile-verdict-evidence.md)
2. [Upstreamed confirmation gate](../../prds/PRD-upstreamed-confirmation-gate.md)
3. [Reconcile revision-pass log](../../prds/PRD-reconcile-revision-pass-log.md)
4. [Reconcile retirement state audit](../../prds/PRD-reconcile-retirement-state-audit.md)
5. [Reconcile study validation](../../prds/PRD-reconcile-study-validation.md)
6. [Reconcile file novelty classifier](../../prds/PRD-reconcile-file-novelty-classifier.md)
7. [Reconcile hunk overlap detector](../../prds/PRD-reconcile-hunk-overlap-detector.md)
8. [Reconcile blocked verdict taxonomy](../../prds/PRD-reconcile-blocked-verdict-taxonomy.md)
9. [Reconcile path restructure detector](../../prds/PRD-reconcile-path-restructure-detector.md)

The TWS aggregate study is a WP-004 planning input only. It strengthens
phase-triggered dependency suggestions, bundled-commit caution evidence, and
workflow-validation refs; it does **not** add new WP-003 reconcile verdict
proof on its own.

## Open questions

- How many additional studies are enough before promoting these signals into
  implementation PRDs?
- Should future studies separate raw reconcile verdicts, post-review labels,
  and final feature states in distinct files?

## Disputes

None yet.
