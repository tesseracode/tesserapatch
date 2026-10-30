# ADR-047: Typed Candidate Execution Ownership

- Status: Proposed
- Date: 2026-10-30
- Deciders: Candidate execution maintainers
- Related: ADR-044, ADR-046, PRD §6.9, PRD §6.13

## Context

S4 execution originally accepted a bare `repoRoot string` and a caller-provided
global worktree-mutation lock handle in `Execute`/`executeWithPlan`. That shape
allowed non-constructor APIs to forward bare-root authority and encouraged
holding global mutation capability across the whole apply/validate path.

PRD §6.9 limits bare-root construction authority to two sanctioned constructors:
`OpenReferenceRoot` and `CreateCandidateRoot`. PRD §6.13 requires global lock
ownership only around worktree add/remove windows, not across expensive
execution/validation.

## Decision

1. `Execute`/`executeWithPlan` derive repository identity from typed
   `ReferenceRoot` inputs, not from a string root parameter.
2. `Execute` accepts only `slug`, `candidateID`, and caller-held `featureLock`
   capability in addition to typed roots/patch/effects (no global-lock argument).
3. Execution borrows the feature lock for the entire mutation/validation/cleanup
   span so copied handles cannot release it mid-execution.
4. Execution acquires the global worktree-mutation lock nonblocking only:
   - before `CreateCandidateRoot` (add window),
   - before `Prune` on failure (remove window),
   and releases it outside those windows.
5. If prune-time global acquisition fails, execution returns a combined failure,
   closes retained root handles, and does not report success.
6. Private feature/global/abandon helpers also consume the existing typed
   reference, candidate or held-root capability; they accept no bare root
   strings. Root identity is checked before lock acquisition. Slug and ID
   validation precedes any mutation.

## Consequences

- Non-constructor execution APIs no longer transport bare-root authority.
- Lock lifetime now matches PRD phase boundaries: feature capability spans the
  execution phase; global capability is windowed around add/remove only.
- Failure reporting preserves both primary mutation/validation cause and cleanup
  failure cause for reviewer diagnostics.
