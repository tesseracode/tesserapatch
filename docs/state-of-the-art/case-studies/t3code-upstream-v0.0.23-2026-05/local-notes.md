# Local Notes — t3code v0.0.23 Reconciliation Study

**Status**: Local tpatch analysis notes
**Date**: 2026-05-15
**Owner**: Core
**Related**: [Case summary](summary.md),
[case-study index](../README.md),
[experiment guide](../../experiment-guide-structural-middle-pass.md),
[research roadmap](../../research-roadmap.md)

## Refresh triggers

- The source study corrects its post-review labels or counters.
- A reconciliation PRD consumes these notes as implementation evidence.
- Another upstream-transition study confirms or contradicts these signals.

## Why this note exists

The files in this directory preserve the imported case-study artifacts. This
note records tpatch-side interpretation after the reconciliation review so raw
study data and product conclusions stay separate.

The most important revision is that `upstreamed` is no longer a reliable success
signal in this study. A follow-up reviewer found both `upstreamed` verdicts were
false positives, but with different failure modes: one retired working fork code,
and the other misclassified missing feature work as upstream adoption.

## Post-review delta

| Feature | Original reconcile result | Review result | Follow-up action |
|---|---|---|---|
| `session-search` | `upstreamed` | False positive; no upstream keybinding, component, or ChatView state existed. A reviewer verified the fork implementation *was* present before reconcile and was dropped by the `upstream_merged` transition. | Re-applied and re-recorded as an applied patch. |
| `copilot-skill-controls` | `upstreamed` | False positive with a different shape; pre-reconcile had only discovery scaffolding, not the enable/disable/reload feature intent. | Implemented server-side plumbing and recorded as applied; web UI toggle remains future work. |
| `toast-close-button` | Already upstream-merged before reconcile | Confirmed genuine upstream absorption, but still showed `dependent-broken` with stale SHA `f6e4378` in the reviewed state. | No feature work needed; dependency/status cleanup remains. |

The aggregate artifacts now represent the corrected post-review state:

| Field | Value |
|---|---|
| Total feature records | 25 |
| Applied features | 15 |
| Upstreamed before this reconcile | 1 |
| False-positive upstreamed verdicts in this reconcile | 2 |
| Requested / not implemented | 5 |
| True blocked conflicts | 2 |
| Typecheck | 13/13 packages passing |

## Reconciliation tracking improvements

The revision pass would have been easier if every reconcile verdict carried a
small evidence bundle:

| Field | Why it matters |
|---|---|
| `raw_reconcile_verdict` | Keeps the original machine verdict available after review corrections. |
| `review_verdict` | Distinguishes confirmed, false-positive, false-negative, and inconclusive outcomes. |
| `final_feature_state` | Separates the state after human repair from the original reconcile result. |
| `evidence_kind` | Records whether the verdict came from patch bytes, recipe operation matches, file novelty, hunk overlap, provider output, or manual review. |
| `matched_paths` / `matched_operations` | Lets reviewers inspect exactly what convinced the reconciler. |
| `pre_reconcile_presence` | Shows whether the feature existed in the fork before reconcile, so reviewers can detect dropped working code. |
| `upstream_commit_refs` | Required before calling a feature upstreamed; otherwise sibling fork code can masquerade as upstream adoption. |
| `match_origin` | Distinguishes code introduced by upstream from code introduced by earlier fork patches or sibling features. |
| `confidence` | Gives low-confidence `upstreamed` and `blocked` verdicts a manual-review queue instead of a terminal state. |
| `action_taken` | Captures re-applied, re-recorded, implemented, retired, skipped, or deferred revision outcomes. |
| `dependency_state_after_action` | Catches cases like `toast-close-button`, where the feature is correct but stale dependency metadata still needs cleanup. |
| `validation_refs` | Links typecheck/test results, patch bytes, affected file counts, and reviewer notes to the correction. |

Two status boundaries should be explicit:

1. `upstreamed` should be a candidate verdict until human confirmation verifies
   the feature intent exists upstream. It should not retire local patches by
   default.
2. `blocked` should not mean one terminal state. It should split into
   clean-additive false positive, shifted-context relocation, target-deleted,
   and true structural conflict.

## Revision-pass checklist candidate

1. Verify every `upstreamed` verdict against the actual feature intent, not only
   recipe search strings.
2. Confirm each `upstreamed` match was introduced by upstream, not by sibling
   fork code already present in the worktree.
3. Diff the pre-reconcile snapshot against the post-reconcile result for every
   retired feature to detect working fork code that was dropped.
4. For every `blocked` verdict, classify the failure as additive, shifted
   context, target deleted, or structural conflict.
5. Record the review decision and follow-up action per feature before updating
   aggregate counts.
6. Cross-check dependency state for confirmed upstreamed features; true feature
   absorption can still leave stale `dependent-broken` records.
7. Cross-check `study.json`, `metrics.json`, and `features.jsonl` after the
   revision pass.

## Data-quality notes

The imported `summary.md` includes a superseded section saying operation-level
upstream absorption worked for `session-search` and `copilot-skill-controls`.
Treat the later "CRITICAL" section and the post-review action log as the
authoritative interpretation.

The post-review action log is also slightly imprecise for `session-search`: the
feature was not absent from the pre-reconcile fork. It was present in the
`pre-reconcile-v0.0.23` snapshot, dropped by the post-reconcile merge, then
restored by the fix commit. The accurate diagnosis is a false-positive
`upstream_merged` transition, not a never-implemented feature.

The artifacts also expose why future studies should separate raw reconciler
counts, post-review ground-truth labels, and final feature states. Those counts
answer different questions and should not be summed without checking which
phase they represent.

## Approved paper PRDs

- [PRD-reconcile-verdict-evidence](../../../prds/PRD-reconcile-verdict-evidence.md):
  persist evidence bundles for reconcile verdicts.
- [PRD-upstreamed-confirmation-gate](../../../prds/PRD-upstreamed-confirmation-gate.md):
  require confirmation before retiring a local feature as upstreamed.
- [PRD-reconcile-revision-pass-log](../../../prds/PRD-reconcile-revision-pass-log.md):
  add a structured correction log for false positives, false negatives, and
  follow-up actions.
- [PRD-reconcile-retirement-state-audit](../../../prds/PRD-reconcile-retirement-state-audit.md):
  verify dependency/status cleanup after confirmed upstream absorption or
  feature retirement.
- [PRD-reconcile-study-validation](../../../prds/PRD-reconcile-study-validation.md):
  validate case-study counters across raw verdicts, review labels, and final
  states.
- [PRD-reconcile-file-novelty-classifier](../../../prds/PRD-reconcile-file-novelty-classifier.md):
  classify additive/new-file patches before generic blocked handling.
- [PRD-reconcile-hunk-overlap-detector](../../../prds/PRD-reconcile-hunk-overlap-detector.md):
  separate true edit overlap from shifted context.
- [PRD-reconcile-blocked-verdict-taxonomy](../../../prds/PRD-reconcile-blocked-verdict-taxonomy.md):
  split generic blocked verdicts into actionable categories.
- [PRD-reconcile-path-restructure-detector](../../../prds/PRD-reconcile-path-restructure-detector.md):
  detect path-prefix restructures that explain likely true conflicts.

## Open questions

- Should the confirmation gate live in the CLI workflow, the supervisor
  checklist, or both?
- Should `upstreamed` require an upstream commit reference in v1, or allow a
  lower-confidence manual-review queue when the commit is unknown?

## Disputes

None yet.
