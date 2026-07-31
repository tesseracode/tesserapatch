# AGENTS.md

## Agent Roles

### Supervisor Agent

Oversees the consolidation project. Picks tasks from the ROADMAP, spawns implementation and review cycles, makes final approval decisions, and maintains the tracking system.

**Owns**: `docs/ROADMAP.md`, `docs/milestones/*.md`, `docs/handoff/HISTORY.md`, `docs/supervisor/LOG.md`

### Implementation Sub-Agent

Executes a bounded task: either planning work (PRD, ADR) or code implementation. Reads the handoff, does the work, updates the handoff.

**Owns**: `cmd/`, `internal/`, `assets/`, `tests/`, `SPEC.md`, `CLAUDE.md`, `AGENTS.md`, `docs/handoff/CURRENT.md`

### Reviewer Sub-Agent

Reviews implementation sub-agent output against SPEC and ROADMAP. Runs the review checklist. Writes a verdict.

**Owns**: `docs/supervisor/LOG.md` (review entries only)

---

## The Cyclic Workflow

The project advances through repeating implementation → review cycles:

```
┌───────────────────────────────────────────────────────────────┐
│                        SUPERVISOR                             │
│                                                               │
│  1. Read ROADMAP → pick next task                             │
│  2. Write task scope to docs/handoff/CURRENT.md               │
│  3. IMPLEMENTATION PHASE                                      │
│     └─ Sub-agent reads handoff, implements, updates handoff   │
│  4. REVIEW PHASE                                              │
│     └─ Sub-agent reviews changes, writes verdict to LOG.md    │
│  5. DECISION                                                  │
│     ├─ APPROVED → archive handoff, mark task complete, → 1    │
│     ├─ APPROVED WITH NOTES → mark complete w/ follow-up, → 1 │
│     ├─ NEEDS REVISION → add feedback to handoff, → 3          │
│     └─ BLOCKED → log blocker, skip to next task, → 1          │
└───────────────────────────────────────────────────────────────┘
```

### Implementation Phase Details

The implementation sub-agent must:

1. Read `docs/handoff/CURRENT.md` — understand the task
2. Read relevant SPEC sections, reference files from other teams if needed
3. Do the work:
   - **If planning**: Write a PRD (`docs/prds/<name>.md`) or ADR (`docs/adrs/<name>.md`)
   - **If coding**: Write Go code, tests, update assets, update docs
4. Verify: `gofmt`, `go test ./...`, `go build ./cmd/tpatch`
5. Update `docs/handoff/CURRENT.md` with:
   - What was done
   - Files changed
   - Test results
   - Remaining issues
   - Context for reviewer

### Review Phase Details

The reviewer sub-agent must:

1. Read the updated `docs/handoff/CURRENT.md`
2. Read the changed files
3. Run the review checklist:
   - [ ] Code compiles: `go build ./cmd/tpatch`
   - [ ] Tests pass: `go test ./...`
   - [ ] Code formatted: `gofmt -l .`
   - [ ] `.tpatch/` artifacts are deterministic and documented
   - [ ] Secrets kept out of tracked files
   - [ ] CLI behavior matches `SPEC.md`
   - [ ] Handoff file is accurate and complete
   - [ ] If assets changed: parity guard passes
   - [ ] No regressions to previously passing functionality
4. Write verdict to `docs/supervisor/LOG.md`

### Supervisor Decision

After the review verdict, the supervisor:

- **APPROVED**: Archives handoff to HISTORY.md, checks off the task in milestones, updates ROADMAP status, picks next task
- **NEEDS REVISION**: Adds specific feedback to CURRENT.md, sends implementation sub-agent back to iterate
- **BLOCKED**: Logs the blocker, moves to the next unblocked task

---

## Handoff File Contract

`docs/handoff/CURRENT.md` must always contain:

```markdown
# Current Handoff

## Active Task
- **Task ID**: M0.1
- **Milestone**: M0 — Bootstrap
- **Description**: [what needs to be done]
- **Status**: [Not Started | In Progress | Review | Complete | Blocked]
- **Assigned**: [date]

## Session Summary
[What was done in this session]

## Current State
[What works, what's partial, known issues]

## Files Changed
[List of files created/modified]

## Test Results
[Build output, test results]

## Next Steps
[Numbered list of what comes next]

## Blockers
[Any blockers]

## Context for Next Agent
[Non-obvious things the next agent needs to know]
```

When a handoff entry is superseded, append it to `docs/handoff/HISTORY.md` with a timestamp header.

---

## Supervisor Log Format

Each entry in `docs/supervisor/LOG.md`:

```markdown
## Review — [Task ID] — [Date]

**Reviewer**: [agent identifier]
**Task**: [description]

### Checklist
- [ ] Compiles
- [ ] Tests pass
- [ ] Formatted
- [ ] Artifacts deterministic
- [ ] Secrets safe
- [ ] Matches SPEC
- [ ] Handoff accurate

### Verdict: [APPROVED | APPROVED WITH NOTES | NEEDS REVISION | BLOCKED]

### Notes
[Specific observations, feedback, or follow-up items]

### Action Taken
[What the supervisor did: archived handoff, assigned next task, sent back for revision]
```

---

## File Ownership

| File/Folder | Owner |
|-------------|-------|
| `SPEC.md` | Implementation, reviewed by Reviewer |
| `CLAUDE.md` | Implementation |
| `AGENTS.md` | Supervisor |
| `docs/ROADMAP.md` | Supervisor |
| `docs/milestones/*.md` | Supervisor |
| `docs/handoff/CURRENT.md` | Implementation (writes), Supervisor (reads, archives) |
| `docs/handoff/HISTORY.md` | Supervisor |
| `docs/supervisor/LOG.md` | Reviewer + Supervisor |
| `docs/adrs/*.md` | Implementation (writes), Reviewer (reviews) |
| `docs/prds/*.md` | Implementation (writes), Reviewer (reviews) |
| `cmd/`, `internal/`, `assets/` | Implementation |
| `tests/` | Implementation |

## Naming Conventions

- Milestones: `M0`, `M1`, `M2`, ...
- Tasks: `M0.1`, `M0.2`, `M1.1`, ...
- ADRs: `ADR-001-<topic>.md`, `ADR-002-<topic>.md`, ...
- PRDs: `PRD-<feature-name>.md`
- Feature slugs in target repos: kebab-case, e.g., `fix-model-id-translation`

## Context Preservation Rules

**These rules are enforced, not aspirational. A task is not complete until the tracking documents reflect its state.**

1. **Update `docs/handoff/CURRENT.md` at every phase transition** — not just at session end. Phases include: starting a new task, finishing a unit of work, switching from implementation to review, discovering a blocker.
2. **Before context gets low**: Write everything you know to the handoff — assume the next agent has zero context beyond what's in the files.
3. **After each task completion**: Archive the old CURRENT.md entry to `docs/handoff/HISTORY.md` immediately. Do not batch.
4. **After each review**: Append the verdict to `docs/supervisor/LOG.md` immediately. Do not batch.
5. **After every milestone transition**: Update the milestone box in `docs/ROADMAP.md` (✅ / ⬜ / 🚧). If you touch a milestone, you touch the roadmap.
6. **Push after every wave close, not just at cluster ship**: A wave is not durably closed until its commits are on `origin/main`. See the Wave-Close Checklist below. Local-only "approved" work is a form of stale tracking — the three-way review record only becomes durable once pushed.
7. **ADR on every architecture decision**: When you evaluate alternatives and pick one, write an ADR in `docs/adrs/`. "Decision by agent consensus" is not a valid answer — future agents need the reasoning.
8. **Plan.md is session-local, tracking docs are repo-global**: Use `plan.md` (in `~/.copilot/session-state/...`) for scratch planning. The moment a decision is made, propagate it into `CURRENT.md`, `ROADMAP.md`, or an ADR — do not leave load-bearing context in session state.
9. **Never leave tracking stale**: If you did work, update tracking *before* stopping. If you stopped because of a blocker, update tracking with the blocker *before* handing off.

### Cadence cheatsheet

| Trigger | Update |
|---------|--------|
| Started a new task | `CURRENT.md` Active Task block |
| Finished analyze/define/explore/implement | `CURRENT.md` Session Summary + Files Changed |
| Ran tests (pass or fail) | `CURRENT.md` Test Results |
| Hit a blocker | `CURRENT.md` Blockers + `LOG.md` if it affects review |
| Made an architecture choice | New `ADR-00N-<topic>.md` |
| Reviewer wrote a verdict | `LOG.md` entry + handoff transition |
| Milestone done | `ROADMAP.md` status flip + milestone file closed |
| Task done | `CURRENT.md` → `HISTORY.md` archive + new `CURRENT.md` for next task |
| **Wave close (three-way APPROVED)** | Run the Wave-Close Checklist below **before dispatching the next wave** |

### Wave-Close Checklist

**Codified 2026-07-31 after v0.12.0 shipped.** Every wave in the Streams A+B, Wave α, Wave β, and Wave γ arc drew a recurring F1 LOW from the user-external verdict: either the handoff `Status` was still stale at close, or the wave's commits sat unpushed while the next wave dispatched against them. The compounding backlog case study is preserved in `docs/handoff/HISTORY.md` under the v0.12.0 wave archives. This checklist bakes the flip-and-push discipline into the protocol so the pattern stops recurring.

Run this **as the last step of every wave close**, before dispatching the next wave or the cluster consolidation:

- [ ] **Handoff `Status` flipped.** `docs/handoff/CURRENT.md` top-of-file `## Status` reflects the wave's terminal state (`APPROVED`, `SHIPPED`, `ACCEPTED`, `IDLE — next TBD`, etc.) — NOT the intermediate `rev-N dispatched` state that was accurate mid-cycle. If the wave is closed, the Status line MUST say so before the next agent reads it.
- [ ] **Supervisor LOG entry prepended.** Every review verdict (internal + external) and every supervisor decision (adjudication, dispatch, close) appears in `docs/supervisor/LOG.md` before the next tool call operates against tracking docs. Do not batch.
- [ ] **ROADMAP status flipped.** The wave's box in `docs/ROADMAP.md` reads ✅ ACCEPTED (or the terminal status for the cluster) with the terminal commit range recorded. Do not leave 🚧 in-flight rows after acceptance.
- [ ] **HISTORY archive appended.** If the wave was the final wave of a cluster, the handoff body is copied to `docs/handoff/HISTORY.md` under a dated header **at consolidation**, not later. Include commit range, test count, review scoreboard, and the pattern-catch summary.
- [ ] **Push to `origin/main` before the next wave dispatches.** `git push origin main` after the consolidation commit. **If the wave ships a tagged release**, run `git tag -a vX.Y.Z -m "..."` **before** pushing, then `git push origin vX.Y.Z` — a committed-and-pushed release without a tag is the same F1 failure class at the tag layer. The three-way review record is not durable until pushed. Compounding unpushed backlogs (see HISTORY.md for the v0.12.0 case study) hide the review scoreboard from every other machine — do not treat this as bookkeeping.
- [ ] **Non-invalidation invariants confirmed.** Any invariants the cluster-lead declared at dispatch (Side Research md5, guarded-file empty-diff sets, Rule 18 trailer, Rule 20 CLI repro) verified one last time at the close commit, not just at the review commit.

If any box is unchecked when the next wave is about to dispatch, **stop and finish the checklist first**. Dispatching against unpushed / stale-Status state is what generated the F1 pattern; the fix is protocol, not memory.

## PRD Authoring — Strongly Encouraged Conventions

These are **strongly encouraged but not enforced** (no automated guard). They graduated from `docs/whitepapers/WP-001-feature-slice-gap.md §3.5` after the v0.7 cluster review surfaced repeated cross-PRD blind spots that a self-audit would have caught.

1. **Claims-audit appendix**: Each load-bearing claim about current behavior should cite a `file:line` (or `file:line-range`) anchor in the authoritative docs (`SPEC.md`, `docs/dependencies.md`, `docs/feature-layout.md`, `docs/agent-as-provider.md`) or in `internal/`/`assets/` source. Reviewers should spot-check that cites land within ±5 lines of current code.

2. **"Could existing primitives do this?" pre-flight**: An exploratory PRD that proposes a new data-model object should include a short section enumerating the existing primitives (current `feature.yaml` fields, existing trailers, existing config keys) and explaining why none can carry the new responsibility.

3. **"Related" header should match what the PRD actually cites**: If a PRD lists Related PRDs/ADRs/whitepapers in its header, every claim about those documents in the body should cite them by name (not by paraphrase).

These conventions are not enforced because (a) PRD shape varies enough that a parser would over-fit, and (b) reviewer cross-pass is the real safety net. A PRD that omits them is still acceptable; a reviewer is free to ask for them at acceptance time.
