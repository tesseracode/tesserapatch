# State-of-the-art Research

**Status**: Living research index
**Date**: 2026-05-10
**Owner**: Core
**Related**: [SPEC](../../SPEC.md), [Reconcile Workflow](../reconcile.md),
[Recording Patches](../record.md),
[ADR-015 prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md),
[PRD-patch-already-upstream-detector](../prds/PRD-patch-already-upstream-detector.md),
[PRD-record-collision-detection](../prds/PRD-record-collision-detection.md)

## Why this folder exists

This folder captures technical state-of-the-art research for the gap between
tpatch's deterministic heuristics and its full provider-backed / coding-agent
workflow. It is intentionally paper-only: these docs do not authorize code,
schema, CLI, or roadmap changes.

The motivating question is:

> What non-LLM or weakly-nondeterministic algorithms can help tpatch identify,
> order, relocate, and apply feature patches when byte-level replay is not
> enough, but a full LLM pass is too expensive or too nondeterministic?

## What goes here

| Doc kind | Purpose | Lifecycle |
|---|---|---|
| Patch-theory notes | Mathematical vocabulary for patch identity, commutation, dependency, conflict, and ordering. | Living; refresh when new patch-based VCS prior art changes the model. |
| Structural identity notes | Algorithms for deciding whether two patches or code regions are "the same" beyond byte identity, including vector/RAG retrieval. | Living; refresh when new source/binary differencing or retrieval techniques become relevant. |
| Metadata sufficiency notes | Audits whether tpatch-controlled metadata can make patch identity/dependency resolution easier before fallback algorithms run. | Living; refresh when feature/patch schema changes. |
| Capture-context notes | Researches how patches, intent, file scope, and agent/IDE/Git context should be captured before record/reconcile. | Living; refresh when capture or recipe provenance changes. |
| Research roadmap | Durable tracker for exploratory fronts and candidate PRD/ADR sequencing independent of implementation handoff. | Living; refresh when research is added or promoted. |
| Search/planning notes | Algorithms for choosing application order and candidate relocations without asking an LLM. | Living; refresh when a candidate planner PRD is opened. |
| Experiment guides | Collection protocols for case studies that can validate or falsify the research. | Snapshot per experiment design; refresh when the data schema changes. |
| Case-study datasets | Recorded reconcile experiments that test the structural middle-pass hypotheses against real upstream transitions. | Living index; append one folder per study. |
| Synthesis notes | tpatch-specific bridge from the research into possible future PRDs/ADRs. | Snapshot per research pass, then superseded by PRDs/ADRs. |

## What does not go here

- **Decisions** -> [`docs/adrs/`](../adrs/)
- **Specific build proposals** -> [`docs/prds/`](../prds/)
- **Market/competitor positioning** -> [`docs/market-research/`](../market-research/)
- **Cross-agent disagreement logs** -> [`docs/whitepapers/`](../whitepapers/)
- **Current implementation contract** -> [`SPEC.md`](../../SPEC.md)

## Document conventions

- Each doc opens with `Status` / `Date` / `Owner` / `Related` lines.
- Each doc has a `## Refresh triggers` section near the top.
- Research docs may suggest candidate PRDs or ADRs, but must label them as
  candidates.
- Claims about current tpatch behavior should link to the authoritative docs
  or PRDs that define that behavior.
- External sources should be named in `## References`; URLs are included when
  stable and public.
- End with `## Open questions` and `## Disputes`.
- Filename: kebab-case. No numeric prefix unless the doc becomes a numbered
  series.

## Index

| Doc | Status | Scope |
|---|---|---|
| [patch-theory-and-commutation.md](patch-theory-and-commutation.md) | Snapshot research | Patch algebra, commutation, dependency, conflict, and ordering vocabulary. |
| [patch-identity-and-structural-fingerprints.md](patch-identity-and-structural-fingerprints.md) | Snapshot research | Diff algorithms, patch equivalence, CV-style keypoints, AST/CFG/PDG, vector/RAG retrieval, and binary similarity. |
| [tpatch-metadata-for-patch-identity.md](tpatch-metadata-for-patch-identity.md) | Snapshot research | Current metadata sufficiency audit and candidate first-party metadata to carry for easier identity/dependency resolution. |
| [patch-capture-context-research-brief.md](patch-capture-context-research-brief.md) | Research brief | Prompt and preserved context for researching Quilt-style file claims, IDE/coding-agent/Git hooks, and privacy-safe agent context capture. |
| [patch-capture-prior-art-and-hooks.md](patch-capture-prior-art-and-hooks.md) | Snapshot research | Quilt, StGit, Git hooks/trailers, Entire checkpoints, Aider Git workflow, and tpatch capture implications. |
| [research-roadmap.md](research-roadmap.md) | Living tracker | Durable research front and recommended PRD/ADR sequencing independent of `docs/handoff/CURRENT.md`. |
| [search-based-patch-application.md](search-based-patch-application.md) | Snapshot research | Non-LLM search/planning strategies for patch ordering and relocation. |
| [storage-substrate-and-versioned-data.md](storage-substrate-and-versioned-data.md) | Snapshot research | Storage substrate and versioned-data prior art; concludes authoritative tpatch state should remain tracked files, with indexes/caches as derived projections only. |
| [experiment-guide-structural-middle-pass.md](experiment-guide-structural-middle-pass.md) | Snapshot guide | Case-study protocol for collecting keypoints, k-grams, AST/vector data, apply outcomes, and evaluation metrics. |
| [case-studies/](case-studies/) | Living dataset index | Recorded structural middle-pass case studies and imported experiment artifacts. |
| [tpatch-middle-pass-synthesis.md](tpatch-middle-pass-synthesis.md) | Snapshot synthesis | A possible future "structural/search planner" seam for tpatch. |
