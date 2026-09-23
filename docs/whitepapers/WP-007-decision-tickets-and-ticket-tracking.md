# WP-007 — Decision Tickets and Ticket Tracking

**Status**: Exploring
**Authors**: W65
**Started**: 2026-07-16
**Turn log**: [WP-007-decision-tickets-and-ticket-tracking.turns.md](./WP-007-decision-tickets-and-ticket-tracking.turns.md)

**Related**:
- [Active clusters](../CLUSTERS.md)
- [Whitepaper protocol](./README.md)
- [WP-001 Feature-slice gap and intent-VCS direction](./WP-001-feature-slice-gap.md)
- [WP-002 Capture and metadata foundation](./WP-002-capture-and-metadata-foundation.md)
- [WP-003 Reconcile safety and middle-pass foundation](./WP-003-reconcile-safety-and-middle-pass.md)
- [WP-004 Auto feature dependency suggestions](./WP-004-auto-feature-dependencies.md)
- [WP-005 Spec-driven workflows](./WP-005-spec-driven-workflows.md)
- [WP-006 tpatch substrate and non-Git mode](./WP-006-tpatch-substrate-and-non-git-mode.md)
- [Feature layout](../feature-layout.md)
- [PRD-feature-patch-identity-metadata](../prds/PRD-feature-patch-identity-metadata.md)
- [PRD-feature-unapply](../prds/PRD-feature-unapply.md)
- [PRD-recurring-patches](../prds/PRD-recurring-patches.md)
- [PRD-skill-doc-strategy](../prds/PRD-skill-doc-strategy.md)
- [ADR-011 Feature Dependency DAG](../adrs/ADR-011-feature-dependencies.md)
- [ADR-015 Prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md)
- [ADR-019 tpatch land trailer block schema](../adrs/ADR-019-tpatch-land-trailer-block-schema.md)
- [ADR-024 Patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md)
- [ADR-025 Reconcile evidence and revision schema](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md)
- [ADR-026 Patch amendment policy](../adrs/ADR-026-patch-amendment-policy.md)

## Intake rebaseline *(O54, 2026-09-22)*

This paper remains **Exploring**. Its working recommendation is still negative:
do **not** add decision tickets as a tpatch feature type, and do not infer CLI,
schema, or `.tpatch/features/` work from this paper alone.

Section 4's tesseraworkspaces fit notes remain useful, but they depend on a
local sibling checkout snapshot. Keep Turn 2's corrected local path as the
historical provenance record, yet treat those anchors as local-only inputs that
must be revalidated before any public or implementation-facing proposal.

If a later experiment is authorized, keep it paper-only / hybrid and external to
the patch store unless a future dispatch explicitly decides otherwise.

## 1. Context — why decision tickets are being considered *(W65)*

tpatch's core object is an implementation feature. A feature has request,
analysis, spec, exploration, recipe, patch, reconcile, verification, dependency,
trailer, and generation artifacts. Its identity is anchored by a stable slug and
moving patch bytes. That model is excellent when work eventually produces a
patch.

Some work is earlier than that:

- choose an architecture direction;
- compare options;
- collect evidence;
- decide that a question should be deferred;
- revisit prior reasoning;
- later graduate the result into an ADR, PRD, tpatch feature, or external task.

Those are decision workflows, not implementation feature workflows. The research
question is whether "decision tickets" should become a tpatch feature type, a
better use of existing tpatch planning docs, a separate code/Markdown ticket
project, or a hybrid integration.

Working finding: **a decision ticket is not a tpatch feature** unless and until
it graduates into patch-bearing implementation work. The near-term path should be
hybrid: keep tpatch focused on patch/change identity, use the existing
whitepaper/PRD/ADR process for tpatch's own decisions, and explore a separate
ticket graph/workspace tool that can link to tpatch feature slugs and
generations without living under `.tpatch/features/`.

## 2. Current tpatch decision surfaces *(W65)*

tpatch already has several decision surfaces. They are process artifacts, not a
generic ticket tracker.

| Surface | Current role | Evidence | Fit for decision tickets |
|---|---|---|---|
| Whitepapers | Exploratory, multi-agent problem restatement between PRDs and ADRs. | `docs/whitepapers/README.md:3-19` | Strong for larger research decisions inside this repo. |
| Turn logs | Append-only cross-agent collaboration logs with stable agent IDs and cites. | `docs/whitepapers/README.md:66-102` | Strong for durable reasoning, but per-whitepaper rather than a general ticket graph. |
| PRDs | Specific build proposal that can graduate into ADR + implementation. | `docs/whitepapers/README.md:6-10`, `docs/whitepapers/README.md:104-112` | Good after a decision narrows into a concrete feature. |
| ADRs | Locked decisions after design work. | `AGENTS.md:187-198`, `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:7-24` | Good terminal form, not an exploration queue. |
| CLUSTERS.md | Live dashboard for in-flight PRD clusters and states. | `docs/CLUSTERS.md:1-24` | Good project-state dashboard; not a decision store. |
| Supervisor LOG | Review/decision audit trail after implementation/review cycles. | `AGENTS.md:132-158` | Good verdict history; not a place to work unresolved options. |
| `.tpatch/features/<slug>/` | Patch-bearing implementation unit with canonical replay patch. | `docs/feature-layout.md:1-44`, `SPEC.md:19-48` | Poor fit when no patch exists. |
| Patch generations | Moving patch identity and dependency snapshots. | `PRD-feature-patch-identity-metadata.md:80-115`, `ADR-024-patch-generation-manifest-boundary.md:48-75` | No fit until a canonical patch exists. |
| Land trailers | Git projection of a patch-bearing feature. | `ADR-019-tpatch-land-trailer-block-schema.md:7-24`, `ADR-019-tpatch-land-trailer-block-schema.md:56-71` | Useful link target from decisions, not decision-ticket storage. |

Two existing papers are directly relevant:

- WP-001 warned against inventing new storage objects before proving existing
  primitives are insufficient and made canonical patch authority a non-negotiable
  invariant (`WP-001-feature-slice-gap.md:37-69`).
- WP-005 found that tpatch already has a strong internal process:
  `whitepaper gap study -> PRD cluster -> ADR -> implementation wave -> review`,
  while optional process aids should not duplicate existing docs
  (`WP-005-spec-driven-workflows.md:44-70`, `WP-005-spec-driven-workflows.md:440-459`).

## 3. Wayfinder research brief *(W65)*

I reviewed the public Wayfinder docs and source from
`mattpocock/skills`:

- Docs page: <https://raw.githubusercontent.com/mattpocock/skills/main/docs/engineering/wayfinder.md>
- Skill source: <https://raw.githubusercontent.com/mattpocock/skills/main/skills/engineering/wayfinder/SKILL.md>
- Decision-ticket terminology changeset: <https://raw.githubusercontent.com/mattpocock/skills/main/.changeset/wayfinder-decision-tickets.md>
- Research-subagents changeset: <https://raw.githubusercontent.com/mattpocock/skills/main/.changeset/wayfinder-research-subagents.md>
- Local-markdown tracker changeset: <https://raw.githubusercontent.com/mattpocock/skills/main/.changeset/friendlier-setup-and-local-tickets.md>

Concepts worth borrowing:

1. **Decision tickets are not implementation tickets.** Wayfinder defines a ticket
   as a question whose resolution is a decision, not a build slice. Its docs say
   the map is done when nothing remains to decide before someone builds.
2. **Destination first.** The first action is naming the destination so every
   ticket is scoped against it.
3. **Map as index, ticket as store.** Wayfinder's map only lists decisions and
   links to tickets; each decision lives in exactly one ticket.
4. **Fog vs ticket.** A question becomes a ticket only when it can be stated
   precisely. Vague future uncertainty stays as "not yet specified."
5. **Frontier.** Open, unblocked, unclaimed child tickets form the next actionable
   frontier. Blocking relationships belong in the tracker, not in prose alone.
6. **Ticket types.** Wayfinder distinguishes HITL tickets (grilling/prototype)
   from AFK research tickets, and can use research subagents to burn down
   independent research blockers.
7. **Local Markdown fallback.** If no issue tracker is configured, the wider skill
   set supports local Markdown tickets; a changeset says local tickets are one
   file per ticket under `.scratch/<feature>/issues/<NN>-<slug>.md`.
8. **Names over bare IDs.** User-facing references should use names/titles rather
   than raw issue numbers.

What not to import directly into tpatch:

- A generic issue-tracker map does not need patch bytes, reconcile, land,
  generation IDs, or feature dependency closure.
- Wayfinder's tracker abstraction belongs closer to a ticket/workspace layer than
  to tpatch's patch replay layer.
- The local Markdown ticket layout under `.scratch/` is intentionally not
  `.tpatch/features/`, reinforcing that decision tickets can live outside the
  patch store.

## 4. tesseraworkspaces fit *(local-checkout baseline; revalidate before public proposal)* *(W65)*

The sibling directory is named `../../tesseraspaces` in this checkout, while its
README and module name use `tesseraworkspaces`: `README.md` starts with
`# tesseraworkspaces`, install docs reference
`github.com/jdbencardinop/tesseraworkspaces/cmd/tws`, and `go.mod` declares
`module github.com/jdbencardinop/tesseraworkspaces`
(`../../tesseraspaces/README.md:1-14`, `../../tesseraspaces/go.mod:1-8`).

Current tesseraworkspaces behavior is directly relevant:

- It is a CLI for feature-scoped workspaces with multiple Git worktrees and
  parallel/stacked branches (`../../tesseraspaces/README.md:1-23`).
- It already has a cross-worktree decision channel:
  `tws decide <feature> "<msg>" [--type] [--to]`, `tws decisions show`, and
  `tws decisions ack` (`../../tesseraspaces/README.md:41-57`,
  `../../tesseraspaces/README.md:117-143`).
- Its feature/worktree model covers stacked/divergent branches, multi-repo
  workspaces, inject files, sync, hooks, and agent launch
  (`../../tesseraspaces/README.md:25-115`).

Fit finding:

- tesseraworkspaces is the better home for "create/open a workspace for this
  ticket/branch/agent session" because it already owns worktrees, branch stacks,
  sync, hooks, and agent launch.
- Its existing `tws decide` channel is a lightweight decision broadcast between
  active worktrees, not a full Wayfinder-style decision-ticket graph. A separate
  decision-ticket project could either integrate with it or eventually extend it,
  but should not be confused with the current `tws decide` message stream.
- tpatch can be a link target: a decision ticket may graduate into a tpatch
  feature slug, patch generation, ADR, PRD, or land commit.
- tesseraworkspaces should own workspace-per-ticket behavior if decision tickets
  become executable workspaces; tpatch should remain the patch/change subsystem.

## 5. Option A — Add decision-ticket type inside tpatch *(W65)*

Possible surface:

```bash
tpatch add --kind decision --slug auth-provider-choice "Choose auth provider"
tpatch decision options auth-provider-choice
tpatch decision resolve auth-provider-choice --adr ADR-0XX
```

Possible files:

```text
.tpatch/features/<slug>/
  status.json
  decision.md
  options.md
  evidence.md
  resolution.md
  links.json
```

Pros:

- Reuses tpatch's existing slug and store discovery.
- Keeps decisions near features when a decision later produces a patch.
- Could share `status`, dependency, and skill-install ergonomics.

Cons:

- A decision ticket has no canonical `post-apply.patch`.
- It does not need `record`, `reconcile`, `land`, patch generations, patch IDs,
  or replay verification.
- Storing non-patch tickets under `.tpatch/features/` weakens the invariant that
  a feature is a replayable unit with patch authority.
- Every command must learn "implementation feature vs decision feature" branches.
- It risks repeating the WP-001 mistake: adding new feature-like objects before
  proving naming/UX conventions and existing process are insufficient.

Assessment: **not recommended for v1**. This muddies tpatch's strongest model:
feature identity tied to replayable patch bytes and reconciliation.

## 6. Option B — Use existing tpatch artifacts / whitepapers / PRDs *(W65)*

Current process can already handle many decision workflows:

```text
whitepaper -> PRD -> ADR -> implementation/review
```

Pros:

- Already accepted and used in this repo.
- Strong citation discipline via whitepaper turn logs and claims-audit
  conventions.
- ADRs already lock durable decisions.
- No new schema, CLI, or project.

Cons:

- Not suitable as a general-purpose ticket graph across many small decisions.
- Frontier/blocking relationships are implicit in prose, CLUSTERS, or broker
  routing rather than visible as a tracker graph.
- Less ergonomic for day-to-day agent assignment outside this repo's supervisor
  workflow.
- Whitepapers are too heavy for every small option comparison.

Assessment: **sufficient for tpatch-internal research today**, but not a
replacement for a reusable decision-ticket system.

## 7. Option C — New separate project *(W65)*

A separate code/Markdown ticket-tracking project could model:

- maps;
- decision tickets;
- blocker graph/frontier;
- ticket states;
- confidence / uncertainty;
- options and evidence;
- links to ADRs, PRDs, tpatch features, Git commits, GitHub issues, and
  workspace branches;
- local Markdown storage with future exports to GitHub/GitLab/Linear/Projects.

Pros:

- Keeps tpatch focused on patch identity, replay, record, reconcile, and land.
- Matches Wayfinder's issue-tracker/local-Markdown abstraction better.
- Can integrate with tesseraworkspaces if that project manages per-ticket
  workspaces and agent sessions.
- Can be useful outside tpatch and outside code-patch workflows.

Cons:

- New project overhead.
- Needs its own schema, CLI, tracker adapters, and persistence story.
- Could duplicate parts of GitHub Issues or Linear if scoped too broadly.
- Needs integration standards so links to tpatch are stable and machine-readable.

Assessment: promising long-term, but not enough current repo evidence to create a
project directory now.

## 8. Option D — Hybrid integration *(W65)*

Keep tpatch focused on patch/change identity. Define integration points so an
external decision-ticket system can link to tpatch:

```yaml
links:
  tpatch_features:
    - slug: auth-provider-oauth
      generation: pg_...
  prds:
    - docs/prds/PRD-auth-provider.md
  adrs:
    - docs/adrs/ADR-0XX-auth-provider.md
  workspaces:
    - tws://ticket/auth-provider-choice
```

Potential integration surfaces:

- ticket frontmatter or JSON links to tpatch feature slugs;
- tpatch feature status optionally records `external_ticket_id` later, but does
  not own ticket lifecycle;
- tesseraworkspaces creates workspaces per ticket/map;
- tickets can graduate to ADR/PRD/tpatch feature/external task.

Pros:

- Preserves tpatch's patch-centered model.
- Allows decision tracking to grow in the right abstraction layer.
- Lets existing tpatch whitepapers remain the internal high-ceremony path.
- Enables future tool interoperability without splitting `.tpatch/features/`
  into many object kinds.

Cons:

- Requires agreement on link schema.
- Users need two tools if/when the external project exists.
- Some convenience is lost compared to one `tpatch decision` namespace.

Assessment: **recommended near-term direction**.

## 9. Comparison table *(W65)*

| Option | Fit | Complexity | Agent UX | tpatch scope risk | Integration value | Overall |
|---|---:|---:|---:|---:|---:|---|
| A — tpatch decision feature type | Low | High | Medium | High | Medium | Not v1 |
| B — existing whitepapers/PRDs/ADRs | High for this repo, low generality | Low | Medium | Low | Low | Keep using |
| C — new separate project | High generality | High | High if done well | Low | High | Later candidate |
| D — hybrid links/integration | High | Medium | High | Low | High | Recommended |

## 10. Recommendation *(paper-only; no CLI/schema authorization)* *(W65)*

Do **not** add decision tickets as a tpatch feature type now.

Recommended near-term path:

1. Keep using whitepapers, PRDs, ADRs, CLUSTERS, and supervisor LOG for tpatch's
   own high-ceremony decisions.
2. Define a paper-only "decision ticket" schema outside `.tpatch/features/`,
   inspired by Wayfinder but adapted to local Markdown and tpatch links.
3. Treat tpatch as a linked implementation target, not the owner of decision
   lifecycle.
4. Revisit a separate project only after one or two real decision maps prove the
   local Markdown shape.

Decision tickets are upstream of implementation features. They may graduate into
tpatch features, but they should not be stored as tpatch features before they have
patch authority.

## 11. Smallest credible experiment *(W65)*

Paper-only experiment, no code:

```text
docs/decision-tickets/example-map.md
docs/decision-tickets/example-tickets/
  001-auth-provider-choice.md
  002-oauth-library-research.md
```

or, if we want to mirror Wayfinder's local fallback without creating a permanent
repo convention:

```text
.scratch/decision-tickets/<map-slug>/issues/<NN>-<slug>.md
```

Experiment rules:

- one map has a destination, decisions so far, not-yet-specified, and out-of-scope;
- each ticket has a question, type, blockers, options, evidence links,
  confidence, resolution, and "graduates to" links;
- at least one ticket links to a tpatch feature slug or PRD/ADR;
- no `.tpatch/features/` entries are created;
- no PRD is drafted until the experiment shows a concrete tpatch-owned gap.

Suggested first evaluation question:

> Can a local Markdown decision map reduce broker/session handoff friction without
> duplicating WP turn logs, PRDs, ADRs, or CLUSTERS?

## 12. Open questions for broker *(W65)*

1. Should the smallest experiment use tracked `docs/decision-tickets/` files, or
   untracked `.scratch/` files like Wayfinder's local Markdown fallback?
2. Should this repo dogfood a decision-ticket map for a real upcoming choice, or
   only write a synthetic example?
3. If a separate project is later created, should it live under `tesseracode/` and
   integrate with `tesseraworkspaces`, or be a generic local-ticket CLI?
4. What link fields are stable enough now: tpatch feature slug, generation ID,
   PRD path, ADR path, land commit trailer, GitHub issue URL?
5. Should tpatch ever read `external_ticket_id` from feature metadata, or should
   all links live on the ticket side?
6. Is "confidence" a first-class field, or should uncertainty be represented as
   unresolved options and follow-up tickets?
7. Should decision tickets have HITL/AFK types as Wayfinder does, or is that
   overfitted to one agent toolkit?

## 13. References *(W65)*

### Repo references

| Claim | Citation |
|---|---|
| Whitepapers are exploratory docs between PRDs and ADRs and do not authorize implementation. | `docs/whitepapers/README.md:3-19` |
| Turn logs are append-only cross-agent collaboration logs. | `docs/whitepapers/README.md:66-102` |
| Whitepapers graduate to PRDs/ADRs or close as no action. | `docs/whitepapers/README.md:104-112` |
| CLUSTERS is live state dashboard, not historical log or decision audit trail. | `docs/CLUSTERS.md:1-24` |
| tpatch feature layout centers canonical patch authority. | `docs/feature-layout.md:1-44` |
| tpatch lifecycle states and commands are implementation-feature oriented. | `SPEC.md:19-78` |
| Patch generation manifest ties patch identity metadata together without replacing status, canonical patch, numbered patches, or trailers. | `PRD-feature-patch-identity-metadata.md:80-88` |
| ADR-024 keeps generation history separate from `status.json` and focused on patch identity. | `docs/adrs/ADR-024-patch-generation-manifest-boundary.md:48-75` |
| ADR-019 makes `Tpatch-Feature` the Git commit binding for patch-bearing features. | `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:7-24`, `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md:56-71` |
| WP-005 documents the current whitepaper -> PRD -> ADR -> implementation workflow. | `docs/whitepapers/WP-005-spec-driven-workflows.md:44-70` |
| WP-006 records tpatch's Git/patch substrate assumptions and cautions against broad substrate changes without ADR work. | `docs/whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md:19-39`, `docs/whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md:40-119` |
| tesseraspaces is the local checkout for the tesseraworkspaces module and README. | `../../tesseraspaces/README.md:1-14`, `../../tesseraspaces/go.mod:1-8` |
| tesseraworkspaces already owns feature workspaces, worktrees, agent launch, sync, hooks, and decision broadcasts. | `../../tesseraspaces/README.md:16-23`, `../../tesseraspaces/README.md:41-57`, `../../tesseraspaces/README.md:117-143` |

### External references

| Source | URL | Accessed | Claim used |
|---|---|---:|---|
| Wayfinder docs | <https://raw.githubusercontent.com/mattpocock/skills/main/docs/engineering/wayfinder.md> | 2026-07-16 | Wayfinder maps foggy large efforts as decision tickets on an issue tracker, with a map/index, frontier, fog, ticket types, and local-markdown fallback. |
| Wayfinder skill source | <https://raw.githubusercontent.com/mattpocock/skills/main/skills/engineering/wayfinder/SKILL.md> | 2026-07-16 | Source-level confirmation that the map is an issue, tickets are child issues, tickets are questions, and work-through-map resolves one ticket per session. |
| Wayfinder decision-ticket changeset | <https://raw.githubusercontent.com/mattpocock/skills/main/.changeset/wayfinder-decision-tickets.md> | 2026-07-16 | Confirms "decision ticket" was intentionally introduced to prevent confusion with implementation tickets. |
| Wayfinder research subagents changeset | <https://raw.githubusercontent.com/mattpocock/skills/main/.changeset/wayfinder-research-subagents.md> | 2026-07-16 | Confirms research tickets remain real blockers but can be burned down by `/research` subagents. |
| Local Markdown ticket changeset | <https://raw.githubusercontent.com/mattpocock/skills/main/.changeset/friendlier-setup-and-local-tickets.md> | 2026-07-16 | Confirms local Markdown tickets use one file per ticket under `.scratch/<feature>/issues/<NN>-<slug>.md`. |
| setup-matt-pocock-skills docs | <https://raw.githubusercontent.com/mattpocock/skills/main/docs/engineering/setup-matt-pocock-skills.md> | 2026-07-16 | Confirms tracker configuration is a shared substrate for engineering skills. |
| to-tickets docs | <https://raw.githubusercontent.com/mattpocock/skills/main/docs/engineering/to-tickets.md> | 2026-07-16 | Distinguishes implementation tickets as tracer-bullet slices after a plan/spec is clear. |
