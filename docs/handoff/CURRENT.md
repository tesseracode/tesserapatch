# Current Handoff

## Active Task

- **Task ID**: `feat-amend-dependent-warning` — v0.7.0 implementer
- **Milestone**: v0.7.0 — M15 W3 freshness continuation
- **Status**: Not Started — brief to be drafted before implementer dispatch
- **Assigned**: 2026-05-10

## Just Shipped

- **v0.6.4** — M16 Slice 3 (apply-default-auto + 6-surface skill alignment + parity-anchor strengthening). 4-commit stack `eab2c3c` + `4556387` + `38d13fc` + `477ccc9` + tracking `9bd5fb1`. Tag pushed, ROADMAP M16 ✅, archived to HISTORY.md.
- **v0.7 cluster routing pass** (`7196ae8` + tracking) — opened ADR-016..019 placeholders, slugged M17 with Wave A/B/C/D structure, surfaced 3 supervisor decisions (now resolved — see LOG.md "Supervisor Decisions — v0.7 Cluster Routing — 2026-05-10").

## Resolved Supervisor Decisions (2026-05-10)

1. **PRD-patch-already-upstream-detector**: accepted as accepted-exploratory; slotted into M17 as **Wave D** (default-OFF reconcile fast-path, ships with cluster, user-visible flag flipped on a later v0.8.x point release).
2. **Implementation owner assignment**: deferred to backlog (`backlog-assign-m17-owners`); fresh implementer sub-agent dispatched per Wave at start time.
3. **Claims-audit-table convention**: codified in AGENTS.md as "PRD Authoring — Strongly Encouraged Conventions" (3 conventions, no automated guard, reviewer cross-pass remains the safety net).
4. **Ordering** (bonus): v0.7.0 = `feat-amend-dependent-warning`. M17 boundary-capture cluster ships as **v0.8.0**.

## Background — `feat-amend-dependent-warning`

Continuation of the M15 W3 freshness overlay work (verify-freshness shipped in v0.6.2). The amend-dependent-warning feature warns when a user is about to `git commit --amend` (or otherwise rewrite history) on a commit that has dependent features downstream — preventing silent corruption of the dependency graph.

**Next step before implementer dispatch**: draft a concrete brief covering:
- Scope: which `tpatch` commands/hooks emit the warning (record? apply? a new git pre-amend hook?)
- Detection algorithm: how to identify "this commit is depended on by feature X"
- Output shape: warning text, exit code, override flag (`--force`?)
- Skill-surface impact (if any)
- Test fixtures: amend-on-dependent reproducer

The PRD does not yet exist for this feature — it's a smaller polish item that may not need a full PRD. **Decision required from user**: PRD-first or brief-and-implement?

## M17 — Queued for v0.8.0

| Slice | PRD | ADR placeholder | Wave deps |
|-------|-----|-----------------|-----------|
| A1 — `impl-record-auto-base` | PRD-record-auto-base | ADR-016 | Independent |
| A2 — `impl-reconcile-lock-guard` + writer-norm fix | PRD-reconcile-lock-guard | ADR-017 | Independent (bundles HIGH bug fix at `internal/workflow/reconcile.go:599`) |
| B — `impl-record-collision-detection` | PRD-record-collision-detection | ADR-018 | Wave A1 |
| C — `impl-tpatch-land` | PRD-tpatch-land | ADR-019 | Wave A1 + Wave A2 + Wave B |
| D — `impl-patch-already-upstream-detector` | PRD-patch-already-upstream-detector | TBD | Independent (default-OFF) |

Owner assignment per slice: backlog `backlog-assign-m17-owners`.

## Side Research — State-of-the-art middle pass (2026-05-10)

Paper-only exploratory pass completed for a non-LLM middle layer between
deterministic reconcile heuristics and full provider/coding-agent workflows.
This does **not** change code, schema, CLI behavior, roadmap status, PRDs, or
ADRs.

### Research packet

Created `docs/state-of-the-art/` with docs modeled after the existing market
research / PRD conventions: header block, related links, refresh triggers,
references, open questions, and disputes.

Files:

- `docs/state-of-the-art/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`

### Findings

1. Patch theory is useful as vocabulary for identity, inverse, composition,
   commutation, dependency, and conflict, but tpatch should not claim
   Darcs/Pijul guarantees on top of unified diffs.
2. Patch identity should be treated as a ladder: exact bytes, `git patch-id`,
   token fingerprints, AST/CFG/PDG similarity, behavioral checks, and finally
   provider/human intent judgment.
3. Computer-vision feature matching maps to code relocation: detect salient
   code keypoints, compute local descriptors, match across old/new upstream,
   reject outliers, then attempt relocated apply in a shadow tree.
4. Search-based application should operate only on uncertain patch clusters,
   after deterministic dependency/commutation pre-passes shrink the search
   space.
5. Beam search is the likely first practical non-LLM planner; MCTS and
   evolutionary algorithms remain candidates for larger uncertain clusters.

### Candidate follow-up names

These are research outputs only, not queued roadmap work:

- `PRD-structural-patch-fingerprints`
- `PRD-reconcile-commutation-graph`
- `PRD-reconcile-search-planner`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-planner-audit-artifacts`

## Blockers

None. Awaiting user choice on PRD-first vs brief-and-implement for `feat-amend-dependent-warning`.
