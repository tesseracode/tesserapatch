# Structural Middle-pass Case Studies

**Status**: Living case-study index
**Date**: 2026-05-15
**Owner**: Core
**Related**: [Experiment guide](../experiment-guide-structural-middle-pass.md),
[tpatch middle-pass synthesis](../tpatch-middle-pass-synthesis.md),
[Research roadmap](../research-roadmap.md)

## Refresh triggers

- A new upstream-reconcile study is captured.
- A study's ground-truth labels are corrected after manual review.
- A PRD/ADR consumes a study finding as implementation evidence.

## Index

| Study | Transition | Records | Key signal |
|---|---|---|---|
| [`t3code-upstream-v0.0.23-2026-05`](t3code-upstream-v0.0.23-2026-05/) | `tesseracode/t3code` fork from upstream `v0.0.21` lock to `v0.0.23` | 25 features, 53 hunks, 5 patch summaries | Both `upstreamed` verdicts were false positives, and 13 of 15 `blocked` verdicts were false positives. See [local notes](t3code-upstream-v0.0.23-2026-05/local-notes.md). |
| [`adjacent-cli-args-conflict-2026-08`](adjacent-cli-args-conflict-2026-08/) | Synthetic Go CLI argument addition versus neighboring upstream deletion | 4 asserted Git variants + tracked tpatch replay/safety trial | Merge and rebase conflict identically for the adjacent fixtures; an anchor-based operation produces a clean candidate, but current reconcile does not replay applicable operations and current recipe semantics require additional safety gates. |
| [`copilot-api-cumulative-verify-2026-08`](copilot-api-cumulative-verify-2026-08/) | `tesseracode/copilot-api` long-lived cumulative fork at `e2d7ce4` | 56 features, exact v0.15.1 verify/doctor reproduction, 38 own-base patch probes | Repository gates pass while verify reports 0 pass / 53 fail / 3 skip; 29 of 38 V8 failures replay on their recorded base, and four recent V10 failures are missing manual provenance rather than measured stale hashes. |

## Import notes

The imported artifacts preserve the study's original `.tpatch/case-studies/`
layout: `study.json`, `features.jsonl`, `hunks.jsonl`, `patches.jsonl`,
`metrics.json`, and `summary.md`.

The t3code study stores metadata, hashes, paths, feature labels, and aggregate
outcomes. It does not store raw source bodies. The 2026-05-15 import includes
the post-review action log plus local corrections: `session-search` was working
before reconcile, incorrectly retired, and then re-applied;
`copilot-skill-controls` was implemented server-side after its false upstreamed
verdict; and `toast-close-button` was confirmed as already upstreamed but still
needs stale dependency/status cleanup.

The adjacent-argument study is a synthetic reproduction rather than an imported
production dataset. Its script creates disposable repositories and stores no
source outside the fixture itself.

The copilot-api cumulative-verification study records only aggregate results,
artifact metadata and source-contract findings. Raw downstream source bodies
and full verifier output remain outside this repository.

## Candidate PRD signals

The first study now has an approved paper PRD cluster:

1. [Reconcile verdict evidence](../../prds/PRD-reconcile-verdict-evidence.md)
2. [Upstreamed confirmation gate](../../prds/PRD-upstreamed-confirmation-gate.md)
3. [Reconcile revision-pass log](../../prds/PRD-reconcile-revision-pass-log.md)
4. [Reconcile retirement state audit](../../prds/PRD-reconcile-retirement-state-audit.md)
5. [Reconcile study validation](../../prds/PRD-reconcile-study-validation.md)
6. [Reconcile file novelty classifier](../../prds/PRD-reconcile-file-novelty-classifier.md)
7. [Reconcile hunk overlap detector](../../prds/PRD-reconcile-hunk-overlap-detector.md)
8. [Reconcile blocked verdict taxonomy](../../prds/PRD-reconcile-blocked-verdict-taxonomy.md)
9. [Reconcile path restructure detector](../../prds/PRD-reconcile-path-restructure-detector.md)

## Open questions

- How many additional studies are enough before promoting these signals into
  implementation PRDs?
- Should future studies separate raw reconcile verdicts, post-review labels,
  and final feature states in distinct files?

## Disputes

None yet.
