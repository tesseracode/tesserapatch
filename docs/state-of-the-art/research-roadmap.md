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
| Structural middle pass | Snapshot complete | `patch-theory-and-commutation.md`, `patch-identity-and-structural-fingerprints.md`, `search-based-patch-application.md`, `tpatch-middle-pass-synthesis.md` | PRD/ADR queue when ready. |
| RAG/vector retrieval | Snapshot complete | `patch-identity-and-structural-fingerprints.md`, `tpatch-middle-pass-synthesis.md` | `PRD-patch-vector-index` after privacy boundary. |
| Experiment guide | Snapshot complete | `experiment-guide-structural-middle-pass.md` | First case study under `case-studies/<study-id>/`. |
| First-party metadata | Snapshot complete | `tpatch-metadata-for-patch-identity.md` | `PRD-feature-patch-identity-metadata` + boundary ADR. |
| Patch capture/context | In progress | `patch-capture-context-research-brief.md`, `patch-capture-prior-art-and-hooks.md` | Capture privacy ADR, file-claims PRD, capture-modes PRD. |

## 2. Recommended PRD/ADR sequence

### Foundation: capture and privacy

1. `ADR-capture-context-privacy-boundary`
2. `ADR-patch-amendment-policy`
3. `PRD-feature-file-claims`
4. `PRD-record-capture-modes`
5. `PRD-feature-patch-amend`
6. `PRD-active-feature-session`
7. `PRD-record-context-summary`
8. `PRD-agent-event-log`
9. `PRD-git-hook-capture-guards`
10. `ADR-capture-metadata-branch`

### Foundation: patch identity metadata

1. `PRD-feature-patch-identity-metadata`
2. `ADR-patch-generation-manifest-boundary`
3. `PRD-dependency-version-snapshots`
4. `PRD-recipe-operation-identity`
5. `PRD-structural-anchor-manifest`

### Middle-pass algorithms

1. `PRD-structural-patch-fingerprints`
2. `PRD-reconcile-commutation-graph`
3. `ADR-structural-middle-pass-boundary`
4. `PRD-reconcile-search-planner`
5. `PRD-reconcile-planner-audit-artifacts`
6. `PRD-patch-vector-index`

## 3. Recommended immediate path

1. Write `ADR-capture-context-privacy-boundary`.
   - Decide raw transcripts vs summaries vs hashes/references.
   - Decide what can be committed, gitignored, or stored on a separate branch.
2. Write `ADR-patch-amendment-policy`.
   - Decide when corrections rewrite the canonical patch, append fixups, fork,
     fold/squash, or only amend metadata.
   - Define how amendments affect dependent features and patch generations.
3. Write `PRD-feature-file-claims`.
   - Model Quilt-style explicit path/symbol claims.
   - Define advisory vs strict behavior.
4. Write `PRD-record-capture-modes`.
   - Define `--staged`, `--unstaged`, `--all`, `--claimed-only`, and
     combinations with `--from`.
5. Write `PRD-feature-patch-identity-metadata`.
   - Add capture-mode and claim/context IDs to patch generations.
6. Only after those, design agent/IDE/Git hook capture.

Rationale: privacy, amendment semantics, and explicit scope should be settled
before tpatch starts capturing richer agent context.

## 4. Research prompts

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
> docs. Draft the next PRD/ADR only; do not implement code. Preferred first doc:
> `ADR-capture-context-privacy-boundary`, followed by
> `ADR-patch-amendment-policy`, then `PRD-feature-file-claims`.

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
