# Patch Capture and Agent Context Research Brief

**Status**: Research brief (paper-only; no implementation authorized)
**Date**: 2026-05-11
**Owner**: Core
**Related**: [Recording Patches](../record.md),
[Feature Layout](../feature-layout.md),
[Patch identity metadata research](tpatch-metadata-for-patch-identity.md),
[Middle-pass synthesis](tpatch-middle-pass-synthesis.md),
[Experiment guide](experiment-guide-structural-middle-pass.md),
[SPEC §3 lifecycle](../../SPEC.md#3-core-workflow--the-7-phase-lifecycle),
[SPEC §4 CLI](../../SPEC.md#4-cli-commands)

## Why this doc exists

The first research pass studied how to identify, order, relocate, and reapply
patches after they already exist. This brief opens the upstream question:

> How should tpatch capture a feature patch in the first place, including the
> user's intent, exact file scope, agent context, recipe provenance, and Git
> history boundaries?

Current `tpatch record` can capture working-tree diffs or commit-range diffs.
That is necessary, but it may not be sufficient when several agents, IDEs, Git
operations, and human edits all contribute to a feature. This research front
should study how prior tools capture patch scope and developer context, then
translate that into possible tpatch PRDs.

## Refresh triggers

- A PRD opens for file claims, active-feature sessions, IDE hooks, Git hooks, or
  agent context capture.
- `tpatch record`, `apply-recipe.json`, feature layout, or skill surfaces change
  capture semantics.
- A case study shows `record` captured too much, too little, or the right bytes
  with insufficient context.

## 1. Current tpatch baseline

Current authoritative docs:

- `docs/record.md`: `tpatch record <slug>` captures unstaged modifications plus
  untracked files by default; `--from <base>` captures committed work after the
  fact.
- `docs/feature-layout.md`: `artifacts/post-apply.patch` is the canonical
  current feature diff; `patches/NNN-*.patch` are historical full-diff audit
  snapshots.
- `SPEC.md`: the lifecycle is
  `analyse -> define -> explore -> implement -> test -> record -> reconcile`.
- `apply-recipe.json`: records deterministic operations from implement, but does
  not yet carry stable operation IDs, read/write sets, capture telemetry, or
  full agent context.

Important current limitation:

> tpatch knows what changed at record time, but not always why each file belongs
> to the feature, which agent/user action introduced it, which context was used
> to decide it, or whether unrelated edits leaked into the diff.

## 2. Prior-art targets to research

### 2.1 Quilt-style explicit file claiming

Quilt treats patches as the primary working material. The Debian manpage says
files must be added to the patch before modification; `quilt refresh` compares
current files to the saved backups for those claimed files.

Research questions:

- Should tpatch support a Quilt-like `claim` step, e.g.
  `tpatch claim <slug> path...`, before edits happen?
- Should claimed files constrain what `record` is allowed to capture?
- Should claims be advisory by default and strict with a flag?
- Can explicit file claims coexist with the current default of capturing
  unstaged and untracked files?
- Should claims support path, glob, symbol, or operation-level scope?

Possible tpatch translation:

```text
tpatch claim feature-a src/foo.go docs/foo.md
edit files
tpatch record feature-a --claimed-only
```

### 2.2 Git index and commit-boundary capture

Git already has a rich capture surface:

- staged vs unstaged changes;
- interactive hunk staging;
- branch and merge-base boundaries;
- commit messages and trailers;
- pre-commit, prepare-commit-msg, post-commit, post-checkout, post-merge hooks.

Research questions:

- Should tpatch read the Git index as the feature boundary?
- Should `tpatch record` support `--staged`, `--unstaged`, `--all`, and
  `--claimed-only` modes?
- Can Git trailers connect commits back to feature slugs and patch hashes before
  `tpatch land` exists?
- Should tpatch ever install hooks automatically? Current safety posture
  suggests opt-in only.
- Which hook should warn that a commit contains edits not assigned to any active
  feature?

### 2.3 IDE hooks

IDE integrations can observe context that Git cannot:

- open files, active editor, selected region;
- save/rename/delete events;
- diagnostics and quick-fix events;
- active test/debug sessions;
- file-to-feature assignment in a UI;
- chat or command IDs from an embedded coding assistant.

Research questions:

- What minimal IDE hook API would help tpatch without making core depend on a
  specific IDE?
- Should IDEs write an append-only local event log under `.tpatch/`?
- Should event logs be committed, ignored, summarized, or stored outside the
  repo?
- What privacy controls are required for editor selections and chat context?
- Can an IDE hook improve recipe provenance without storing full transcripts?

### 2.4 Coding-agent hooks

Coding agents can emit structured provenance while they work:

- active feature slug;
- user request and subtask ID;
- tool calls and command summaries;
- file reads/writes;
- generated operation IDs;
- tests run and results;
- confidence/uncertainty notes;
- prompt/context references.

Research questions:

- Should tpatch define a generic "agent event" schema that any coding agent can
  append to?
- Should agents update `apply-recipe.json` incrementally or write a separate
  provenance log that `record` later compiles?
- What is the smallest useful context artifact: summaries and references, not
  raw prompts?
- How can agents mark "this edit belongs to feature X" before the final diff?
- How should tpatch handle multiple agents touching the same repo concurrently?

### 2.5 Entire / full-agent-context capture

The user clarified that Entire is <https://entire.io/> and the linked
<https://github.com/entireio/cli> repository. Initial findings are captured in
[`patch-capture-prior-art-and-hooks.md`](patch-capture-prior-art-and-hooks.md).

Research questions:

- What artifacts does Entire persist, and at what granularity?
- Does it store raw conversation, summarized memory, tool traces, file diffs,
  embeddings, or task graphs?
- How does it handle privacy, secrets, retention, replay, and branch changes?
- Which ideas map cleanly to tpatch's local-first, secret-by-reference model?

Key verified Entire ideas:

- Git hooks plus agent hooks capture sessions as work happens.
- User commits stay clean while metadata is stored on `entire/checkpoints/v1`.
- Commits link to context through an `Entire-Checkpoint` trailer.
- Agent integrations expose session IDs, transcript refs, prompts, modified
  files, token usage, tool calls, subagents, and resume commands.
- Security/privacy is a first-class problem because transcripts may contain
  prompts, tool interactions, file contents, secrets, and PII.

## 3. Hypothesis for tpatch

tpatch likely wants **layered capture**, not one universal capture mode:

| Layer | Capture source | Role |
|---|---|---|
| Intent | `tpatch add`, define/spec docs, agent summary | Why this feature exists and what success means. |
| Scope claim | User/agent/IDE claimed paths or symbols | What should belong to the feature before edits happen. |
| Edit telemetry | IDE or agent events | How files changed during the work session. |
| Recipe provenance | `apply-recipe.json` plus future op IDs/read/write sets | What deterministic operations were intended. |
| Git boundary | staged/unstaged diff, commit range, trailers | What bytes changed and how they map to history. |
| Patch artifact | canonical `post-apply.patch` | Replay authority. |
| Context summary | local summaries/references to prompts, commands, tests | Enough context to audit without storing raw sensitive data. |

The safest product shape is probably:

1. keep current `record` as the zero-setup fallback;
2. add explicit file/symbol claims for high-precision capture;
3. add optional IDE/agent event logs for richer provenance;
4. add optional Git hooks that warn, never silently mutate;
5. compile all sources into a patch-generation manifest at record time.

## 4. Required research output

The next research agent should produce:

1. `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`
   - Quilt, StGit, Git index/hunk staging, Git hooks, Entire, IDE hooks, coding-agent
     context systems, and full-agent-context capture prior art.
2. Updates to this brief if Entire changes or another relevant context system
   is identified.
3. Candidate PRD/ADR list with priority and boundaries.
4. A small capture model matrix:
   - manual explicit claim;
   - inferred from Git;
   - inferred from IDE;
   - emitted by coding agent;
   - reconstructed from conversation/context.
5. Risks and privacy/security constraints.

## 5. Prompt for a future research agent

Use this prompt verbatim or adapt it:

> We are working in the `tpatch` repo. This is a docs-only research task; do not
> change code or schemas. Read `docs/state-of-the-art/README.md`,
> `docs/state-of-the-art/patch-capture-context-research-brief.md`,
> `docs/record.md`, `docs/feature-layout.md`, `SPEC.md`,
> `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`, and
> `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`.
>
> Research how patch-management tools and coding environments capture patches
> before replay/reconciliation. Include Quilt's `new/add/refresh` model, Git
> staged/unstaged/hunk capture, commit trailers and Git hooks, StGit or similar
> patch-stack tools, IDE extension hooks, coding-agent event logs, and systems
> that capture full agent context. The user specifically mentioned `"Entire"`;
> first verify whether this is a specific tool/product/framework. If unclear,
> state that and compare adjacent systems such as Continue, Cursor, Copilot
> coding-agent/workspace-style flows, Aider, and OpenHands/OpenDevin-style
> agents.
>
> Goal: identify what tpatch should capture at feature-creation and
> feature-record time so recipes and patches carry intent, precise scope,
> provenance, and enough context for future reconcile. Propose a layered model
> for manual claims, IDE/agent hooks, Git hooks, recipe provenance, patch
> generation manifests, and privacy-safe context summaries. Keep recommendations
> local-first and secret-by-reference. End with candidate PRDs/ADRs, open
> questions, references, and disputes.

## 6. Existing research context not to lose

The current state-of-the-art packet already produced these research docs:

- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`
- `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`
- `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`

Existing candidate follow-ups:

| Candidate | Captures |
|---|---|
| `PRD-feature-patch-identity-metadata` | Patch generation IDs, patch SHA, patch-id, recipe SHA, capture provenance. |
| `ADR-patch-generation-manifest-boundary` | What belongs in `status.json` vs append-only manifests vs recomputed artifacts. |
| `PRD-dependency-version-snapshots` | Parent patch/recipe generation snapshots. |
| `PRD-recipe-operation-identity` | Stable operation IDs, read/write sets, anchor IDs. |
| `PRD-structural-anchor-manifest` | Keypoints, token fingerprints, optional AST fingerprints. |
| `PRD-structural-patch-fingerprints` | Structural duplicate/similarity fingerprints. |
| `PRD-patch-vector-index` | Optional vector/RAG retrieval over patches, hunks, intent, code chunks. |
| `PRD-reconcile-commutation-graph` | Pairwise `commutes` / `depends_on` / `conflicts` / `unknown` facts. |
| `PRD-reconcile-search-planner` | Bounded non-LLM search over uncertain patch clusters. |
| `ADR-structural-middle-pass-boundary` | Phase 2.5 boundary between deterministic and provider workflows. |
| `PRD-reconcile-planner-audit-artifacts` | Persist planner attempts, scores, seeds, selected orders, validation. |

Potential new capture-front PRDs/ADRs:

| Candidate | Captures |
|---|---|
| `PRD-feature-file-claims` | Quilt-style explicit path/symbol claiming before record. |
| `PRD-record-capture-modes` | `--staged`, `--unstaged`, `--all`, `--claimed-only`, and commit-boundary capture semantics. |
| `PRD-active-feature-session` | Current active feature slug and session state for agents/IDEs. |
| `PRD-agent-event-log` | Generic append-only local event schema for coding agents. |
| `PRD-ide-capture-hooks` | Optional editor integrations for save/rename/diagnostic/test events. |
| `PRD-git-hook-capture-guards` | Opt-in hooks that warn when commits contain unassigned or stale feature edits. |
| `ADR-capture-context-privacy-boundary` | What raw context can be stored, summarized, ignored, or referenced by hash. |
| `ADR-capture-metadata-branch` | Whether tpatch should use a separate metadata branch like Entire. |
| `PRD-record-context-summary` | Privacy-safe summaries of commands, tests, diagnostics, and prompt/context references. |

## References

- Quilt manual: <https://manpages.debian.org/testing/quilt/quilt.1.en.html>
- [Recording Patches](../record.md)
- [Feature Layout](../feature-layout.md)
- [Patch identity metadata research](tpatch-metadata-for-patch-identity.md)
- [Middle-pass synthesis](tpatch-middle-pass-synthesis.md)

## Open questions

- Should file claims be strict by default, advisory by default, or only used by
  opt-in record modes?
- Should tpatch store raw agent/IDE event logs, derived summaries, or both?
- Where should event logs live if they may contain sensitive context?
- Should Git hooks be installed by `tpatch init`, offered as opt-in, or left as
  documented snippets?
- How should tpatch attribute edits when multiple agents or humans modify the
  same claimed file?

## Disputes

None logged.
