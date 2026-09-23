# WP-007 — Turn Log

## Turn 1 - W65 - 2026-07-16

**Responding to**: human broker research prompt
**Type**: research

Initial research pass for decision tickets and ticket tracking. I read the
current tpatch process surfaces (`docs/CLUSTERS.md`, `docs/whitepapers/README.md`,
`AGENTS.md`, `docs/handoff/CURRENT.md`, and the supervisor log enough to
understand operational flow), recent whitepapers WP-001 / WP-004 / WP-005 /
WP-006, recent PRDs for patch identity, recurring patches, feature unapply, and
skill docs, and ADRs 011, 015, 019, 020, 024, 025, and 026.

External review covered the Wayfinder docs and skill source in
`mattpocock/skills`. The most important concepts are: decision tickets are
questions whose resolution is a decision, not implementation slices; a map is an
index, not a store; fog becomes tickets only when the question is precise; the
frontier is open/unblocked/unclaimed tickets; research can be AFK while
grilling/prototype tickets are HITL; local Markdown fallback uses one file per
ticket under `.scratch/.../issues/`.

The sibling `../tesseraworkspaces` directory was not present in this checkout,
and `https://api.github.com/repos/tesseracode/tesseraworkspaces` returned 404
when checked on 2026-07-16, so the tesseraworkspaces fit section is conceptual
rather than source-grounded.

Recommendation: do not add decision tickets as a tpatch feature type. A decision
ticket has no canonical patch, does not need record/reconcile/land, and would
muddy `.tpatch/features/` as a patch-bearing store. Keep using
whitepapers/PRDs/ADRs for tpatch's own high-ceremony decisions, and explore a
hybrid external/local ticket graph that links to tpatch slugs, patch generations,
PRDs, ADRs, and workspaces without tpatch owning ticket lifecycle.

**Asks of next agent / broker**:
- Decide whether the smallest experiment should use tracked
  `docs/decision-tickets/` files or untracked `.scratch/` files.
- Pick one real upcoming decision to dogfood as a decision-ticket map, or ask for
  a synthetic example only.
- If tesseraworkspaces exists elsewhere, provide its path or repository so the
  fit section can be grounded in actual code/docs.
- Decide whether link metadata should live on the ticket side only, or whether a
  future tpatch feature may store an `external_ticket_id`.

## Turn 2 - W65 - 2026-07-16

**Responding to**: broker correction on tesseraworkspaces path
**Type**: revision

The broker clarified that `../tesseraworkspaces` is not the local sibling path;
the matching checkout is `../../tesseraspaces` from this repo's `tpatch/`
directory, while that repo's README/module use `tesseraworkspaces`.

Updated WP-007 §4 from a conceptual-only tesseraworkspaces fit section to a
source-grounded one. The local repo is a CLI for feature-scoped workspaces with
multiple Git worktrees, stacked/divergent branches, agent launch, sync, hooks,
and a `tws decide` / `tws decisions` cross-worktree decision broadcast channel.
The updated finding is that tesseraworkspaces is a strong owner for
workspace-per-ticket behavior, but its current decision channel is lighter than a
Wayfinder-style decision-ticket graph.

**Asks of next agent / broker**:
- Decide whether a future decision-ticket experiment should integrate with
  existing `tws decide` messages, or stay separate until a full ticket graph is
  proven.

## Turn 3 - O54 - 2026-09-22

**Responding to**: documentation-only research intake rebaseline
**Type**: review

Rebased this paper as **still Exploring**. The recommendation is unchanged: do
**not** add decision tickets as a tpatch feature type, and do not treat this
paper as authorization for a new CLI namespace or feature-store schema.

Turn 2's correction about the local tesseraworkspaces checkout path remains
important, but it should now be read as **local-only provenance** for the fit
discussion, not as a public-proof claim that the same checkout or repository
state is generally available. If this topic becomes active again, revalidate the
local workspace anchors before turning them into a public proposal.

The near-term path stays paper-only: whitepapers/PRDs/ADRs for tpatch's own
decisions, plus a possible external or hybrid decision-map experiment if the
broker explicitly opens one.
