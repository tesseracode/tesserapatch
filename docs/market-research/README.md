# Market Research

Strategic / positioning artifacts that sit alongside ADRs, PRDs, and
whitepapers. This folder answers "where does tpatch fit in the
market?" — both today and as the option-space evolves.

## What goes here

| Doc kind | Purpose | Lifecycle |
|---|---|---|
| **Competitive landscape** | Map tpatch against prior-art / competitors using strategy frameworks (PESTEL, SWOT, SMART, Strategy Canvas, Business Model Canvas). | Living. Refresh on triggers documented at the top of each doc. |
| **Per-system deep-dive** *(future)* | Detailed analysis of a single competitor / prior-art system when a quick mention in `competitive-landscape.md` isn't enough. | Living. Refresh on releases / major changes. |
| **Persona / segment analysis** *(future)* | Customer-segment / user-persona descriptions, when concrete enough to inform roadmap. | Living. |
| **Industry report / external-input synthesis** *(future)* | Distillation of an external source (research paper, industry survey, conference talk) into roadmap-relevant bullets. | Snapshot — date the doc, don't refresh. |

## What does **not** go here

- **Decisions** → [`docs/adrs/`](../adrs/)
- **Proposals to build something specific** → [`docs/prds/`](../prds/)
- **Cross-agent debate / problem re-statement** → [`docs/whitepapers/`](../whitepapers/)
- **Roadmap / milestones** → [`docs/ROADMAP.md`](../ROADMAP.md)
- **Operating-workflow contracts** (commits, dependencies, agents) → top-level `docs/<topic>.md`

## Document conventions

- **Living by default.** Each doc carries a `## Refresh triggers`
  section near the top. If a doc shouldn't be refreshed, mark it
  Status: Snapshot and date it explicitly.
- **Header block.** Every doc opens with `Status` /
  `Date` / `Owner` / `Related` lines, matching the ADR / PRD pattern.
- **Cross-link liberally.** Research informs decisions. A market-
  research doc that mentions a tpatch concept should link to the
  authoritative ADR / PRD / SPEC section that defines it. ADRs that
  cite this folder are normal.
- **Filename:** kebab-case. No numeric prefix unless the doc is part
  of a numbered series.
- **Disputes block.** Strategy frameworks involve directional
  scoring. End the doc with a `## Disputes` block left empty until
  a scoring is contested. When contested, log the dispute, resolve
  via discussion, fold the resolution back in.

## Authoring a new doc

1. Read the `competitive-landscape.md` doc end-to-end so your new
   doc doesn't duplicate analysis already there.
2. Pick a kebab-case filename based on the doc kind (e.g.
   `deep-dive-<system>.md`, `persona-<segment>.md`,
   `report-<topic>-<source>.md`).
3. Open with the standard header (Status / Date / Owner / Related).
4. Add an entry in this README's Index below.
5. If the new doc surfaces a finding worth locking, **also** open an
   ADR or PRD — market-research docs are inputs to decisions, not
   substitutes for them.

## Index

| Doc | Status | Last refreshed |
|---|---|---|
| [competitive-landscape.md](competitive-landscape.md) | Living | 2026-05-09 |
| [personas.md](personas.md) | Living | 2026-05-03 |
