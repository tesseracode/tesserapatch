# Whitepapers

Exploratory, multi-agent collaboration documents that sit **between** PRDs and
ADRs.

| Doc type | Purpose | Lifecycle |
|---|---|---|
| Whitepaper | Grounded re-statement of a problem before any commitment. May be co-authored by multiple agents in a back-and-forth. | Lives in `docs/whitepapers/`. Either graduates into one or more PRDs/ADRs, or is closed as "no action". |
| PRD | Proposal to build a specific thing. | `docs/prds/`. Graduates into ADR + implementation. |
| ADR | Locked decision. | `docs/adrs/`. |

A whitepaper is the right format when:

- two or more agents disagree or have non-overlapping mental models of a problem,
- existing PRDs make claims that need re-grounding against current code/docs,
- the gap or scope of a future feature is not yet clear enough to write a PRD.

A whitepaper is **not** a PRD. It does not commit roadmap scope. It does not
authorize implementation work.

## Naming

`WP-NNN-<short-slug>.md` — sequentially numbered. The companion turn log is
`WP-NNN-<short-slug>.turns.md`.

## Authorship and bylines

Whitepapers are typically written by more than one agent. Each substantive
section carries a byline:

```markdown
### Section title  *(CO47)*
```

When agents agree on a re-statement, it goes under an `## Agreed` heading
with no byline. When they disagree, both views stay in the document side by
side under their bylines — the whitepaper does not paper over disagreement.

### Human / broker turns *(optional)*

The human acting as broker between agents (or any human contributor) **may**
take a turn at any time. Human turns are first-class — same format, same
append-only rule — but they are never required. Use them to:

- steer the whitepaper (scope, priorities, what to cut),
- supply external context the agents cannot infer from the repo,
- override or pin a contested point so the agents stop circling,
- declare a turn closed and ask for a graduation step (PRD, ADR, closure).

Conventions for human turns:

- Byline is `human` (lowercase) or a stable handle the human chooses,
  used consistently across turns.
- `Type:` should be one of `steering`, `context`, `decision`, `question`,
  or `closure` — distinct from agent turn types so they are easy to scan.
- Human turns may edit `## Agreed` directly when they are of `Type: decision`;
  agent turns may only edit `## Agreed` when the change reflects an
  agreement reached *between agents* in the log.
- Human turns may also explicitly hand the next turn to a named agent in
  `Asks of next agent`, which the broker then routes.

A whitepaper with zero human turns is normal and fine. A whitepaper where
the human takes every other turn is also fine — the protocol doesn't
prescribe a cadence.

## Turn-log protocol

Every whitepaper has a companion `.turns.md` file. This is the cross-agent
collaboration log. Append-only. Newest turn at the bottom.

Why a tracked Markdown file and **not** the session SQLite db: the per-session
SQLite mirror lives in `.tpatch-backlog/` which is **gitignored**
(see repo `.gitignore`). It does not survive across sessions, agents, or
checkpoints, so it cannot carry cross-agent state. The turn log must be
tracked in git.

### Turn entry format

````markdown
## Turn N — <agent-id> — <ISO date>

**Responding to**: <Turn M | initial review | external prompt>
**Type**: review | proposal | counter | agreement | open-question

<body — concise, with file/line cites where relevant>

**Asks of next agent** (optional):
- ...
````

Rules:

1. Append-only. Never edit a prior turn (typo fixes excepted; mark with
   `[edited <date> by <agent>]`).
2. One turn per substantive contribution. Don't bundle.
3. Always cite repo files by path (and line range when possible) for any
   factual claim about current behavior.
4. When a turn produces an agreed re-statement, the agent who wrote the
   turn also edits the whitepaper's `## Agreed` section to reflect it,
   referencing the turn number.
5. Agents are identified by stable IDs (e.g. `CO47`, `G55`). When a new
   agent joins a whitepaper, they introduce themselves in their first turn.

## Graduation

When a whitepaper produces a concrete proposal, the originating agent
opens a PRD in `docs/prds/` that links back to the whitepaper. The
whitepaper itself stays in place as historical context.

When a whitepaper concludes "no action" or "subsumed by existing
primitives", the final turn is marked `Type: closure` and the whitepaper
header is updated to **Status: Closed**.

## Index

| ID | Title | Status |
|---|---|---|
| WP-001 | Feature-slice gap & intent-VCS direction | **Graduated** (2026-04-28) |
