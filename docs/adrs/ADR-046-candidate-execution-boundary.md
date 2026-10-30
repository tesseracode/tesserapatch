# ADR-046 — Candidate execution snapshot boundary

**Status**: Proposed — pending independent S4 review and complete validation
**Date**: 2026-10-30  
**Issue**: GH #13 S4  
**Related**: ADR-037 D14-D17, ADR-044, ADR-045

## Context

S4 must re-run the **same** derivation/classifier on the candidate result for
idempotency, but a candidate worktree is not an immutable commit object.
Existing snapshot helpers in `internal/candidate/snapshot.go` are commit-bound
(`SnapshotCommitPathsOffline` + `ReadCommitObjectOffline`) and must remain
commit-authoritative.

## Decision

1. Keep commit snapshot semantics unchanged for `ReferenceRoot`.
2. Add a typed `CandidateRoot.snapshotPaths` reader that captures path presence,
   mode and size from the candidate rooted worktree and reads content through
   rooted regular-file reads only.
3. Refactor derivation internals to a shared `derive` core that accepts a
   typed snapshot provider (`snapshotRoot`), while preserving public
   `Derive(reference, upstream *ReferenceRoot, ...)`.
4. Use that same core for:
   - first-pass derivation (`ReferenceRoot` upstream), and
   - second-pass idempotency (`CandidateRoot` upstream).
5. Do not fabricate commits/refs, and do not weaken commit validation in
   `OpenReferenceRoot`/`CreateCandidateRoot`.

## Consequences

- Idempotency uses production classification logic without a bespoke checker.
- Commit-only patch observation APIs remain commit-only.
- Candidate execution can validate mutable worktree state without introducing a
  third bare-root constructor or live-root string API.

## Alternatives rejected

- **Fabricate a temporary commit from candidate output**: rejected; introduces
  mutable authority through synthetic commit identity and unnecessary ref/object
  handling risk.
- **Build a separate idempotency checker**: rejected; violates ADR-037/PRD
  requirement that second pass reuses production derivation/classification.
- **Loosen `patchobs` commit APIs to read mutable paths**: rejected; would blur
  immutable commit authority boundaries established in S2/S3.
