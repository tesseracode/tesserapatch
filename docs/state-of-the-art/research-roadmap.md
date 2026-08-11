# State-of-the-art Research Roadmap

**Status**: Living research tracker
**Date**: 2026-05-11
**Owner**: Core
**Related**: [State-of-the-art index](README.md),
[Middle-pass synthesis](tpatch-middle-pass-synthesis.md),
[Patch capture prior art](patch-capture-prior-art-and-hooks.md),
[Patch capture brief](patch-capture-context-research-brief.md)

## Why this doc exists

`docs/handoff/CURRENT.md` belongs to the active implementation/supervisor flow
and may be overwritten when implementation work resumes. This roadmap is the
durable research front for exploratory work that is not yet roadmap-queued.

Use this file to preserve:

- active research fronts;
- candidate PRD/ADR queues;
- recommended sequencing;
- prompts for future research agents;
- unresolved research questions.

This file does **not** authorize implementation.

## Refresh triggers

- A research doc is added, superseded, or promoted into a PRD/ADR.
- The implementation handoff changes and would otherwise drop research context.
- A new exploratory front begins.

## 1. Active research fronts

| Front | Status | Primary docs | Next output |
|---|---|---|---|
| Structural middle pass | Narrow PRDs approved | `patch-theory-and-commutation.md`, `patch-identity-and-structural-fingerprints.md`, `search-based-patch-application.md`, `tpatch-middle-pass-synthesis.md`, `case-studies/README.md` | Keep broad structural/search work in backlog pending more studies. |
| RAG/vector retrieval | Snapshot complete | `patch-identity-and-structural-fingerprints.md`, `tpatch-middle-pass-synthesis.md` | `PRD-patch-vector-index` after privacy boundary. |
| Experiment guide | First study promoted | `experiment-guide-structural-middle-pass.md`, `case-studies/README.md` | Run at least one more upstream-transition study before broad structural/search PRDs. |
| Reconcile safety/tracking | Foundation PRDs approved | `case-studies/t3code-upstream-v0.0.23-2026-05/local-notes.md`, `../whitepapers/WP-003-reconcile-safety-and-middle-pass.md` | Draft ADR-025 before implementation; WP-002 Wave beta gates PRD 1. |
| First-party metadata | Foundation PRD drafted | `tpatch-metadata-for-patch-identity.md` | Review identity metadata PRD; patch-generation boundary ADR next. |
| Patch capture/context | Foundation PRDs drafted | `patch-capture-context-research-brief.md`, `patch-capture-prior-art-and-hooks.md` | Review/accept capture privacy ADR, amendment ADR, and drafted PRDs before implementation. |

## 2. Recorded case-study signal

The first recorded study is
[`t3code-upstream-v0.0.23-2026-05`](case-studies/t3code-upstream-v0.0.23-2026-05/),
imported under [`case-studies/`](case-studies/). It covers a private fork of
public upstream `pingdotgg/t3code` moving from a `v0.0.21` lock to `v0.0.23`
across 25 feature records and 53 hunks.

The strongest signal is that current `blocked` verdicts are overly
conservative: 13 of 15 blocked features were false positives. Nine reapplied
cleanly, four needed insertion-point relocation, and only two were true
conflicts caused by upstream directory restructure. The post-review revision
adds a higher-severity signal: both `upstreamed` verdicts in this reconcile were
false positives, which would have silently dropped or skipped local feature work
if trusted. A follow-up reviewer further clarified that `session-search` was
working in the pre-reconcile fork, then was dropped by an incorrect
`upstream_merged` transition and re-applied later.

The study supports a narrow middle-pass queue before heavier AST/vector work:

1. Evidence bundles and confirmation gates for `upstreamed` verdicts.
2. Structured revision-pass logs that preserve raw reconcile verdict, review
   verdict, final state, evidence, action taken, and validation refs.
3. Pre/post reconcile retirement audits to catch dropped working fork code.
4. Dependency/status cleanup checks after confirmed upstream absorption.
5. File-level novelty classification for additive/new-file patches.
6. Hunk-level overlap detection between upstream diff hunks and feature hunks.
7. Better blocked-verdict taxonomy: clean-additive, shifted-context,
   target-deleted, and structural-conflict.
8. Path-prefix restructure detection to identify likely true conflicts early.

See the study's
[`local-notes.md`](case-studies/t3code-upstream-v0.0.23-2026-05/local-notes.md)
for tpatch-side recommendations on reconciliation tracking and revision passes.

The promoted PRD cluster is now approved on paper:

1. [PRD-reconcile-verdict-evidence](../prds/PRD-reconcile-verdict-evidence.md)
2. [PRD-upstreamed-confirmation-gate](../prds/PRD-upstreamed-confirmation-gate.md)
3. [PRD-reconcile-revision-pass-log](../prds/PRD-reconcile-revision-pass-log.md)
4. [PRD-reconcile-retirement-state-audit](../prds/PRD-reconcile-retirement-state-audit.md)
5. [PRD-reconcile-study-validation](../prds/PRD-reconcile-study-validation.md)
6. [PRD-reconcile-file-novelty-classifier](../prds/PRD-reconcile-file-novelty-classifier.md)
7. [PRD-reconcile-hunk-overlap-detector](../prds/PRD-reconcile-hunk-overlap-detector.md)
8. [PRD-reconcile-blocked-verdict-taxonomy](../prds/PRD-reconcile-blocked-verdict-taxonomy.md)
9. [PRD-reconcile-path-restructure-detector](../prds/PRD-reconcile-path-restructure-detector.md)

## 3. Recommended PRD/ADR sequence

### Foundation: capture and privacy

1. `ADR-capture-context-privacy-boundary`
2. `ADR-patch-amendment-policy`
3. [`PRD-feature-file-claims`](../prds/PRD-feature-file-claims.md) - drafted
4. [`PRD-record-capture-modes`](../prds/PRD-record-capture-modes.md) - drafted
5. [`PRD-feature-patch-amend`](../prds/PRD-feature-patch-amend.md) - drafted
6. `PRD-active-feature-session`
7. `PRD-record-context-summary`
8. `PRD-agent-event-log`
9. `PRD-git-hook-capture-guards`
10. `ADR-capture-metadata-branch`

### Foundation: patch identity metadata

1. [`PRD-feature-patch-identity-metadata`](../prds/PRD-feature-patch-identity-metadata.md) - drafted
2. `ADR-patch-generation-manifest-boundary`
3. `PRD-dependency-version-snapshots`
4. `PRD-recipe-operation-identity`
5. `PRD-structural-anchor-manifest`

### Reconcile safety and narrow middle-pass PRDs

1. [`PRD-reconcile-verdict-evidence`](../prds/PRD-reconcile-verdict-evidence.md) - approved
2. [`PRD-upstreamed-confirmation-gate`](../prds/PRD-upstreamed-confirmation-gate.md) - approved
3. [`PRD-reconcile-revision-pass-log`](../prds/PRD-reconcile-revision-pass-log.md) - approved
4. [`PRD-reconcile-retirement-state-audit`](../prds/PRD-reconcile-retirement-state-audit.md) - approved
5. [`PRD-reconcile-study-validation`](../prds/PRD-reconcile-study-validation.md) - approved
6. [`PRD-reconcile-file-novelty-classifier`](../prds/PRD-reconcile-file-novelty-classifier.md) - approved
7. [`PRD-reconcile-hunk-overlap-detector`](../prds/PRD-reconcile-hunk-overlap-detector.md) - approved
8. [`PRD-reconcile-blocked-verdict-taxonomy`](../prds/PRD-reconcile-blocked-verdict-taxonomy.md) - approved
9. [`PRD-reconcile-path-restructure-detector`](../prds/PRD-reconcile-path-restructure-detector.md) - approved

### Deferred broad structural/search backlog

1. `PRD-structural-patch-fingerprints`
2. `PRD-reconcile-commutation-graph`
3. `ADR-structural-middle-pass-boundary`
4. `PRD-reconcile-search-planner`
5. `PRD-reconcile-planner-audit-artifacts`
6. `PRD-patch-vector-index`

Backlog gate: wait for at least one more upstream-transition reconcile case
study before drafting these. The current t3code study is enough for safety
tracking, file novelty, hunk overlap, blocked taxonomy, and path restructure
detection, but not enough to lock broader fingerprint/vector/search planner
architecture.

## 4. Recommended immediate path

1. Write `ADR-capture-context-privacy-boundary`.
   - Decide raw transcripts vs summaries vs hashes/references.
   - Decide what can be committed, gitignored, or stored on a separate branch.
2. Write `ADR-patch-amendment-policy`.
   - Decide when corrections rewrite the canonical patch, append fixups, fork,
     fold/squash, or only amend metadata.
   - Define how amendments affect dependent features and patch generations.
3. Review [`PRD-feature-file-claims`](../prds/PRD-feature-file-claims.md).
   - Confirms Quilt-style explicit path claims and advisory vs strict behavior.
4. Review [`PRD-record-capture-modes`](../prds/PRD-record-capture-modes.md).
   - Confirms `--staged`, `--unstaged`, `--all`, `--claimed-only`, and
     combinations with committed-range modes.
5. Review [`PRD-feature-patch-identity-metadata`](../prds/PRD-feature-patch-identity-metadata.md).
   - Confirms patch-generation metadata, capture-mode provenance, claim IDs,
     dependency snapshots, and future anchor/vector slots.
6. Review [`PRD-feature-patch-amend`](../prds/PRD-feature-patch-amend.md).
   - Confirms refresh/fixup/fork/fold vocabulary after the amendment ADR.
7. Only after those, design agent/IDE/Git hook capture.

Rationale: privacy, amendment semantics, and explicit scope should be settled
before tpatch starts capturing richer agent context.

## 5. Research prompts

### Continue patch-capture research

> Read `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`,
> `docs/state-of-the-art/patch-capture-context-research-brief.md`,
> `docs/record.md`, and `docs/feature-layout.md`. Produce a PRD-ready outline
> for tpatch capture improvements. Separate manual scope claims, Git capture
> modes, active feature sessions, agent event logs, IDE hooks, Git hooks, and
> context privacy boundaries. Keep recommendations local-first and do not
> authorize implementation.

### Start PRD drafting

> Read `docs/state-of-the-art/research-roadmap.md` and the referenced research
> docs. Draft or review the next PRD/ADR only; do not implement code. The first
> PRD set now exists: `PRD-feature-file-claims`,
> `PRD-record-capture-modes`, `PRD-feature-patch-identity-metadata`, and
> `PRD-feature-patch-amend`. Preferred next docs are
> `ADR-capture-context-privacy-boundary` and `ADR-patch-amendment-policy`, or
> review/acceptance updates for the four drafted PRDs.

> Post-v0.14.0 candidate registered 2026-08-10:
> `PRD-feature-resource-claims-and-capture-adapters` +
> `ADR-033-resource-capture-boundary`. It extends shipped file claims and
> capture modes to explicit ignored/logical-Git/external resources (for
> example Dolt exports) while keeping resource diffs as sidecars and raw
> `.git/**` prohibited. Review ADR-027, WP-006, and
> `storage-substrate-and-versioned-data.md` before drafting.

### Run the structural experiment

> Read `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`.
> During a real reconcile/upstream case study, collect data under
> `docs/state-of-the-art/case-studies/<study-id>/` using the prescribed JSONL
> schemas. Do not change algorithms unless a PRD authorizes implementation.

## 5. Promotion rules

Before a research output becomes a PRD/ADR:

- cite the source research doc;
- state whether it changes CLI behavior, schema, assets, or docs only;
- define privacy and determinism boundaries;
- define migration/backwards-compatibility behavior;
- include test fixtures if it affects code;
- update this roadmap to mark the research item promoted.

## Open questions

- Should this roadmap be owned by the Supervisor Agent or a separate research
  owner?
- Should case-study artifacts live in this repo long term, or move to external
  datasets once they grow?
- Should the research packet be versioned by milestone once PRDs start landing?

## Disputes

None logged.
