# WP-006 — Turn Log

## Turn 1 - V64 - 2026-07-10

**Responding to**: human broker research prompt
**Type**: research

Investigated whether `tpatch init` should require or initialize Git and whether
tpatch should support non-Git repositories with its own change-tracking
substrate. Initial conclusion: split the work. A small PRD should cover
near-term Git preflight / explicit init UX / metadata-only command gating, while
tpatch-native non-Git change tracking is architecture-sized and needs a
whitepaper/ADR before any PRD. The recommended stance is Git-first:
metadata-only mode may track intent/planning artifacts, but `record`, `land`,
`reconcile`, patch-id detection, and freshness checks must require Git and fail
early with clear diagnostics.

**Asks of next agent / broker**:
- Decide whether to graduate the near-term PRD
  `PRD-git-init-and-substrate-preflight.md`.
- Resolve the default `tpatch init` behavior outside Git: refuse, prompt, or
  explicit `--git=skip` only.
- Decide whether substrate metadata should use nested `substrate.type` or a flat
  `substrate_type` key in v1.

## Turn 2 - O54 - 2026-09-22

**Responding to**: documentation-only research intake rebaseline
**Type**: review

Rebased this paper as **still Exploring**. The July conclusion remains the same:
Git-first is the only current directional recommendation, and any future work
must stay much narrower than a new substrate abstraction or tpatch-native VCS.

The important current-state correction is that the repository now ships newer
operator guidance around non-Git behavior, including `prepare`'s documented
handling of non-worktree and unusable-Git workspaces. That means the July
code-line inventory below should be treated as an exploratory baseline, not a
silent override of current shipped behavior. If a PRD is reopened later, it
must explicitly reconcile the paper with the current non-Git `prepare` contract
and keep the scope to bounded init/preflight planning.

No interface work, no native non-Git change-tracking substrate, and no CLI
implementation are authorized by this turn.
