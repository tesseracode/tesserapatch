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

**Mechanical gate**: `make wave-close-check` runs 8 programmatic checks (working tree clean, untracked source-code sentinel, HEAD pushed, trailer parses, CURRENT.md Status line not stale, gofmt clean, vet/build clean, `go test -count=1 ./...` clean). Human items (LOG prepended, ROADMAP flipped, HISTORY archived, non-invalidation invariants) remain manual — the target prints them as a reminder banner. Codified 2026-08-02 after Cluster A external challenge #2 flagged that the checklist above is protocol-only with no verifier. **Test-suite check added 2026-08-04 (Cluster E F1)**: prior to this, the gate reported PASS while `go test -count=1 ./...` exited 1 at Cluster D HEAD `1bc2a25` — correctness was the one dimension the gate didn't check. `[8/8]` runs the full suite with `-count=1` to disable the build cache, so the check is deterministic even when source is unchanged between gate invocations.

### Parallel-Implementer Discipline

**Codified 2026-08-02 after the v0.12.1 cross-implementer entanglement postmortem** (`docs/handoff/HISTORY.md` → 2026-07-31 → v0.12.1 archive). During v0.12.1, three implementers ran in parallel against overlapping surfaces (all three touched `internal/cli/cobra.go`). One implementer (PRD-#3 Slice 1+2 at `d930963`) used `git commit -a` which swept in another implementer's uncommitted PRD-#4 production code, mis-attributing it. A second implementer (GH #5) independently caught it and split the commit — but the risk of undetected entanglement in a future parallel dispatch is unacceptable.

**Rules for parallel implementers** (two or more implementers running against a shared file surface — the failure class was catalyzed with three implementers in v0.12.1, but two is sufficient to reproduce it, and F-2 external adjudication at Cluster C rev-1 tightened the trigger accordingly):

1. **`git add <path>` mandatory. `git commit -a`, `git add .`, `git add -A`, and directory-scope `git add <dir>/` all forbidden** when a shared surface is in play. Only file-path or hunk-scoped (`git add -p`) adds are permitted. Every implementer stages by explicit path, and the paths must appear in the implementer's dispatch brief so the supervisor can verify at review time that no cross-implementer surface was touched. The v0.12.1 `d930963` entanglement was catalyzed by `git commit -a` specifically, but the root failure class is **stage-by-glob in a shared worktree**, so the forbidden list closes every glob-shaped vector, not just the `-a` flag.
2. **Cluster lead declares the shared-surface set** at dispatch. If more than one implementer will touch `cobra.go`, `reconcile.go`, `verify.go`, etc., the brief for each implementer lists (a) the specific functions / helpers that implementer owns and (b) an explicit "do not touch" list of siblings.
3. **Reviewers scope by function name, not commit boundary.** When entanglement is possible, reviewers verify each ticket's scope by identifying the specific new/modified functions in the diff — not by trusting the commit's labeled scope. This is the review-side counter-measure that caught the v0.12.1 case.
4. **When entanglement is detected post-hoc**, the fix is `git rebase -i` reword or split, not a follow-up commit. The trailer + attribution must be correct on the archived commit; a "fix" commit that attributes later doesn't repair the historical record. See the v0.12.1 rebase-rewrite of `2934521`→`ba3b3b3` for the pattern.
5. **Sequential fallback**: if the cluster lead can not confidently declare a non-overlapping surface partition at dispatch, the implementers run sequentially, not in parallel. Parallel speed is not worth silent attribution errors. **Same-file overlap is a hard trigger for sequential execution** — F-EXT-3 (Cluster C rev-1 external, upgraded to rev-2) confirmed that even explicit-path `git add shared.go` stages *every* implementer's changes to that file, so a shared-worktree parallel dispatch where two implementers modify the same file is always unsafe. Partition must be non-overlapping at the **file** level, not just the function level. If two implementers need to touch `cobra.go`, they run sequentially — no exception.

This addendum subsumes v0.12.1 PRD-#4 F-3 (the "parallel-implementer process fix" follow-up finding).

## Cluster State — Canonical Field for Mechanical Gate

**Codified 2026-08-02 after Cluster C rev-1 external F-EXT-2** empirically demonstrated that scanning the `## Status` section for terminal tokens false-passes whenever the section contains historical mentions of a prior shipped cluster. The current `## Status` in `docs/handoff/CURRENT.md` at Cluster C rev-1 contained both `v0.12.1 SHIPPED` (historical, correct) and `Housekeeping cluster dispatch pending` (current, mid-cycle), and the gate false-passed because `SHIPPED` matched the allowlist regardless of context.

Fix: `docs/handoff/CURRENT.md` MUST contain **exactly one** canonical single-line field near the top of `## Status`:

```markdown
**Cluster state**: <TOKEN>
```

Where `<TOKEN>` is one of the terminal-state allowlist tokens: `IDLE`, `SHIPPED`, `APPROVED`, `ACCEPTED`, or one of the mid-cycle tokens (denylist): `IN PROGRESS`, `REV-N DISPATCHED`, `AWAITING REVIEW`. The mechanical gate at `Makefile` `wave-close-check` parses **only this field** — it does not scan the rest of the Status section. At wave close, the token must be a terminal-state token; if it is mid-cycle, the gate FAILs. The narrative prose below the canonical field is human-readable context and does not affect the gate.

Convention: the field lives on its own line at the top of `## Status`, above any historical context blocks. When a new cluster dispatches, the field is **replaced in place** with a mid-cycle token; at wave close it is **replaced in place** with a terminal token before the gate is run. **Never append a second field** — the gate rejects duplicates (Cluster C rev-2 external F-EXT-NEW-1 empirically demonstrated that `grep -m1` on multiple fields false-passes on the earliest match, so rev-3 tightened the parse to require exactly one).

**Selecting `WAVE_BASE`** for the trailer-check range at `[4/8]`: the default `$(git describe --tags --abbrev=0)..HEAD` works when the wave is the first cluster after a tag. For subsequent waves in the same release cycle (e.g., Cluster D shipping between v0.12.1 and v0.13.0), the cluster lead should invoke the gate with `WAVE_BASE=<immediate-pre-cluster-ancestor>` — the commit SHA of `origin/main` at the moment the cluster's first implementer was dispatched.

Concrete recipe (run **before** the first implementer dispatches, and record the SHA in the cluster's dispatch brief and in `CURRENT.md`):

```sh
git fetch origin
git rev-parse origin/main   # record this SHA as WAVE_BASE
```

Then at wave close: `make wave-close-check WAVE_BASE=<sha>`. This scopes the trailer walk to exactly this cluster's commits and prevents pollution from prior waves already reviewed. Without recording the SHA pre-dispatch, `origin/main` may advance and the value cannot be derived reliably (Cluster C rev-3 external adjudication).

## PRD Authoring — Strongly Encouraged Conventions

These are **strongly encouraged but not enforced** (no automated guard). They graduated from `docs/whitepapers/WP-001-feature-slice-gap.md §3.5` after the v0.7 cluster review surfaced repeated cross-PRD blind spots that a self-audit would have caught.

1. **Claims-audit appendix**: Each load-bearing claim about current behavior should cite a `file:line` (or `file:line-range`) anchor in the authoritative docs (`SPEC.md`, `docs/dependencies.md`, `docs/feature-layout.md`, `docs/agent-as-provider.md`) or in `internal/`/`assets/` source. Reviewers should spot-check that cites land within ±5 lines of current code.

2. **"Could existing primitives do this?" pre-flight**: An exploratory PRD that proposes a new data-model object should include a short section enumerating the existing primitives (current `feature.yaml` fields, existing trailers, existing config keys) and explaining why none can carry the new responsibility.

3. **"Related" header should match what the PRD actually cites**: If a PRD lists Related PRDs/ADRs/whitepapers in its header, every claim about those documents in the body should cite them by name (not by paraphrase).

These conventions are not enforced because (a) PRD shape varies enough that a parser would over-fit, and (b) reviewer cross-pass is the real safety net. A PRD that omits them is still acceptable; a reviewer is free to ask for them at acceptance time.
