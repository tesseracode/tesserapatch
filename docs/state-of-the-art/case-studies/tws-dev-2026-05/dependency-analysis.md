# Dependency Analysis - tws-dev-2026-05

**Status**: Historical aggregate development-study analysis
**Date**: 2026-05 baseline; intake rebaselined 2026-09-22
**Owner**: Core
**Refresh trigger**: Correct imported aggregate metadata or reopen WP-004
planning.
**Limitations**: Uses `study.json`, `metrics.json`, `summary.md`, and
`local-notes.md` only. No `features.jsonl`, `hunks.jsonl`, or `patches.jsonl`
were imported here, so this folder is not compatible with the normal
reconcile-study validator.

**Study type**: greenfield development workflow, not upstream reconciliation.
Do not evaluate `upstreamed`, `blocked`, or reconcile false positives from this
study. The imported snapshot contains aggregate study files, not per-feature
`features.jsonl` or hunk records, so this analysis uses the imported summaries,
metrics, and local notes only.

## Executive summary

This study strongly reinforces WP-004's core assumption: dependency suggestions
are most useful during feature registration and exploration, before a human has
finished declaring the graph. The strongest signal is not same-file overlap; it
is workflow timing plus explicit review discipline. The steering rule to validate
dependencies after analyze, define, and explore repeatedly found links that were
not obvious at registration time.

It also reinforces a WP-003-adjacent lesson without being a reconcile study:
runtime smoke checks are valuable evidence. The project caught three real bugs
through end-to-end smoke tests that static checks alone would not have made
actionable.

Duration provenance remains unresolved: `summary.md` reports the
2026-05-23..2026-05-27 date window, while `study.json` separately reports
`duration_days: 7`. This intake preserves both instead of choosing a synthetic
"true" duration.

The dependency tables below are qualitative planning input, not automatic-edge
evidence.

## Feature boundary quality

| Observed slice | Boundary assessment | Evidence |
|---|---|---|
| `user-defined-workdir` -> `auto-detect-repos-workspaces` -> `lightweight-worktree-handler` -> `keep-track-of-stacked-diffs-and-dependencies` | Clean parent/child chain | The summary names this as the natural DAG chain that made implementation order obvious. |
| `keep-track-of-stacked-diffs-and-dependencies` | Boundary expanded during implementation | Local notes say it was scoped for linear stacks, but DAG support came naturally from Kahn's algorithm. This is acceptable scope growth, but should be recorded as either an amended feature or an explicit subfeature if it changes downstream contracts. |
| `tmux-free-mode` | Boundary changed from flag addition to default behavior shift | The feature moved from "add `--no-tmux`" to changing the default execution mode. That is a behavior-boundary expansion and should trigger dependency re-check plus stronger validation. |
| `archive-worktree` | Clean implementation-time refinement | The archived-vs-missing distinction came from `git worktree list --porcelain` behavior. This is a feature-specific refinement rather than an infrastructure mix. |
| `cross-worktree-agent-context` | Good simplification; possible parent/child boundary | The original idea was a messaging system, then narrowed to extending `decisions.yaml` with a `to` field. If another feature consumes the `to` field, the field extension should be the parent and targeted messaging should be the child. |
| `auto-read-decisions-hooks` | Correctly amended rather than split | `tpatch amend --append` captured `--all` and `auto_hooks` scope additions. The notes explicitly say this was a scope extension rather than a separate child feature. |
| Bundled features such as quiet fetch, clean output, validation, and push | Boundary was too coarse at commit level | Four record collisions came from multiple features sharing a commit. The features may be conceptually distinct, but the commit/patch boundary made artifact attribution weaker. |

## Observed dependency relationships

| Child | Parent candidate | Suggested kind | Evidence |
|---|---|---:|---|
| `auto-detect-repos-workspaces` | `user-defined-workdir` | hard | The named chain starts with configured workdir support, then auto-detection. Auto-detect depends on the existence of the workspace-root resolution model. |
| `lightweight-worktree-handler` | `auto-detect-repos-workspaces` | hard | Worktree handling needs a resolved repo/workspace location before it can create feature-scoped worktrees. |
| `keep-track-of-stacked-diffs-and-dependencies` | `lightweight-worktree-handler` | hard | Stack DAG behavior builds on feature-scoped worktree handling. |
| `divergent-stack-sync` | `keep-track-of-stacked-diffs-and-dependencies` | hard | The wrong-branch incident happened during divergent stack smoke testing; divergent sync depends on the stack graph foundation. |
| `archive-worktree` | worktree lifecycle/delete feature parent | hard or soft | The feature distinguishes archived vs missing worktrees through `git worktree list --porcelain`; it likely depends on prior worktree lifecycle commands, but the imported snapshot does not name the exact parent. |
| `cross-worktree-agent-context` | stack/dependency foundation plus decisions context | hard multi-parent candidate | It sends context across worktrees and reuses decisions metadata. The stack/worktree side and decisions metadata side are separate evidence points. |
| `auto-read-decisions-hooks` | targeted/decision messaging parent | hard or amended parent | Local notes say it was amended after `--all` and `auto_hooks` additions; if treated as a new child, it should depend on the decision-reading/hook foundation. |
| `tmux-free-mode` | tmux session management | hard | A feature that flips tmux from default to opt-in depends on the existing tmux execution/session model. |

## Missed or questionable dependencies

| Candidate | Assessment | Why |
|---|---|---|
| Dependencies discovered after explore | likely missed at registration | The steering rule explicitly found new links after exploration. This is exactly the WP-004 suggestion window. |
| Bundled-commit features | weak artifact attribution | Same patch/commit evidence would over-suggest dependencies among features that were merely bundled for workflow convenience. |
| Scope-extension features amended with `tpatch amend` | not a new edge by default | The `auto-read-decisions-hooks` example shows that amend can be the right model when scope grows inside the same conceptual feature. |
| Same-command or same-package features | false-positive risk | A CLI project naturally reuses command wiring, config, tests, and docs paths. Same-area overlap should usually be `soft` or `none` unless a parent-created path or explicit API use exists. |

## Multi-parent cases

| Child candidate | Parent A evidence | Parent B evidence | Classification |
|---|---|---|---|
| `cross-worktree-agent-context` | Worktree/stack side: sends context across feature worktrees. | Decisions side: implementation simplified to `decisions.yaml` with a `to` field. | Legitimate merge-point candidate; each edge needs separate evidence. |
| `divergent-stack-sync` | Stack DAG parent: divergent behavior needs dependency graph ordering. | Sync/rebase parent: behavior is exercised through sync and rebase validation. | Likely legitimate multi-parent if the source graph declared both foundations. |
| `auto-read-decisions-hooks` | Decision read-tracking/hook behavior. | Targeted messaging or `--all` expansion. | Could be multi-parent, but the study notes say amend was better than a child feature for this case. |

## Hard, soft, and none classification

| Signal | Recommended kind | False-positive risk |
|---|---|---|
| Explicit `depends_on` already declared | hard or soft as declared | Low; confirmation should preserve the source graph. |
| Steering-discovered prerequisite after explore | usually hard | Low-to-medium; the human/agent found the edge while narrowing implementation. |
| Parent-created path or command/API introduced by parent | hard | Low when backed by operation/path evidence. |
| Same commit from bundled development | none by default | High; bundling caused record collisions and is workflow convenience, not dependency proof. |
| Same CLI command file, docs file, config file, or test harness | soft or none | Medium-to-high; greenfield CLI projects naturally share central files. |
| Shared feature prefix or same release window | soft | Medium; useful for ranking, unsafe for hard gates. |

## WP-004 impact

The tiered model still fits, but this study makes the registration-time flow more
important than the earlier upstream-transition corpus did.

| WP-004 point | Finding |
|---|---|
| Tier 0 explicit links | Strongly reinforced. The graph had 18 edges, max depth 4, zero cycles, and 12 validation runs. Existing declarations remain the source of truth. |
| Tier 1 deterministic artifacts | Useful but incomplete. Record artifacts document touched paths, but bundled commits and missing per-feature snapshot files limit attribution. |
| Tier 2 heuristic locality | Useful during explore, but should bias soft. Same-area command/config/docs edits are common in a new CLI. |
| New evidence kind | Add `phase-discovered-dependency`: a dependency candidate discovered during analyze/define/explore, with phase and reviewer/agent note refs. |
| New evidence kind | Add `bundled-commit-overlap`: a negative or cautionary signal for same-patch collisions caused by deliberate bundling. |
| Suggestion timing | Run suggestions at `define` and `explore`, not only `record`; the study found dependencies before implementation was done. |
| First PRD | `PRD-feature-dependency-suggestions` remains the right first PRD, but should include greenfield Path B workflows and phase-triggered suggestions. |
| Tpatch-Depends-On trailers | Reinforced but not urgent. Commit separation was clean, yet dependency review happened through tpatch metadata rather than Git-only review. |
| Persisted evidence | Strengthened. The case-study aggregate lacks per-edge evidence, which makes later dependency auditing harder. |
| LLM-assisted classification | Strengthened as a reviewer/classifier, not generator. The agent/user found semantic edges during exploration where path math alone would be weak. |
| Static/symbol index | Still later-tier. It would help command/API dependencies, but greenfield evidence shows explicit phase-time review is a cheaper first slice. |

## WP-003 impact

This is not a reconcile case, so it does not evaluate reconcile verdicts,
`upstreamed`, `blocked`, retirement, or false positives. It does reinforce the
WP-003 evidence boundary:

| WP-003 point | Finding |
|---|---|
| `validation_refs` | Should include runtime and smoke checks, not only static test commands. This study recorded smoke tests per feature and three bugs caught by smoke tests. |
| Raw machine evidence vs review decision vs final state | Reinforced. Provider output was unreliable, heuristic artifacts were overwritten, and manual artifacts became the practical source of truth. These should be tracked separately when studying workflow outcomes. |
| Evidence privacy | Reinforced. Store slugs, paths, operation IDs, hashes, counts, commands, exit summaries, and validation refs; do not store source bodies, transcripts, vectors, or secrets. |
| Missing evidence kind | Add `workflow-validation-ref` for development workflow checks such as branch check, smoke test, and post-agent cwd verification. |

## Recommended documentation updates

1. Extend WP-004's experiment plan to include this greenfield Path B study as the
   first development-workflow corpus.
2. Add phase-triggered suggestions to the first auto-deps PRD: run at define and
   explore, not just record.
3. Add a negative evidence kind for bundled commits so collision/same-patch
   evidence does not become a false hard dependency.
4. Keep WP-003 reconcile claims separate: this study only supports validation
   evidence and evidence separation, not reconcile verdict behavior.
