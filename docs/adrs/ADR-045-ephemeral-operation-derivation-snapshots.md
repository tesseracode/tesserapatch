# ADR-045 — Ephemeral operation derivation snapshots

**Status**: Proposed — implementation choice pending independent S3 review
**Date**: 2026-10-29
**Issue**: GH #13
**Related**: ADR-037 sections D8-D12, ADR-043, ADR-044 and the companion
PRD sections 6.6-6.7 and 8 S3

## Context

The accepted classifier requires one immutable upstream snapshot before any
classification. It also bounds one alignment's buffers and dynamic program.
S1 already supplies typed immutable commit roots; S2 proves paired eligibility
without retaining file bodies or exposing a mutable execution plan.

S3 needs exact patch-derived insertion payloads, reference boundaries and
three-state classifications. It must not depend on a mutable worktree, pick
one LCS trace, retain all upstream bodies without a bound, or serialize
record bodies/offsets as a purported replay proof.

## Decision

1. Keep derivation/classification in the candidate package behind typed
   ReferenceRoot inputs. No third bare-root constructor or pipeline wiring.
   Inputs carry validated effect projections, not persisted recipe bodies.
2. Freeze the required upstream path/object identities from its immutable
   commit before classifying. Load immutable objects by captured identity,
   one file at a time with the accepted L1 bound for alignment. A later
   body read is not a new snapshot or a promotion of unobserved evidence.
3. Reuse the strict normalized patch grammar and its existing exact hunk
   walker for insertion-run extraction; do not create a lenient second parser.
   Replaying the extracted runs must match the bound postimage digest.
4. Allocate exactly two int32 F/G matrices after L1-L3 checks for that file.
   No traceback or sampled alignment is an authority. Preserve the accepted
   per-file/per-boundary first-match ordering and canonical run combination.
5. Plans, insertion payloads, record indices and byte offsets are ephemeral.
   No new wire schema or publication is introduced in S3. S4 may consume
   applicable operations; S5 must build explicit metadata-only artifacts.
6. Presence includes the required regular-file mode. An upstream nonregular
   object or wrong-mode path cannot be called present solely because some
   bytes match. Preserve the existing conflict/refusal vocabulary; do not
   invent a permissive conversion into a create or overwrite operation.

## Complexity and matrix-budget proof

The initial implementation rescanned all optimal pairs per boundary, yielding
O(K*n*m); independent review correctly rejected that capacity gap. No new
boundary-count limit is added.

After canonical combination there are at most n+1 distinct boundaries.
Compute each boundary's minimum/maximum optimal cut first, in O(K*(m+1)),
bounded by the matrix cell count. Then scan optimal pairs once and derive
prefix maximum matched-upstream indices and suffix minimum indices. A4 is
equivalently `prefixMax[b] < left[b] && suffixMin[b] >= right[b]`.

No extra DP arrays are allocated: after every requested cut is captured,
reuse F's last column for prefix maxima and G's first column for suffix
minima. The optimal-pair predicate reads F[i][j] only for j<m and
G[i+1][j+1] only for j+1>=1, so it never observes those overwritten cells.
L was saved before reuse. This also covers empty sides. The matrices are
one-use after indexing; classification/refusal still proceeds in sorted
boundary order. Alignment work stays O((n+1)*(m+1)), with the original two
int32 matrices and no third full matrix or hidden additional limit. Input
parsing and canonical run sorting retain their separate input-size costs.

## Alternatives

- **Re-read paths from the live worktree during classification**: rejected;
  classifications could refer to different states and escape the typed roots.
- **Retain every upstream body eagerly**: rejected; the number of affected
  paths would multiply retained memory beyond the per-file alignment budget.
- **Select one LCS traceback**: rejected by the accepted contract; ties cannot
  silently grant authority.
- **Execute the persisted write-file recipe**: rejected by ADR-036/037; it is
  an eligibility witness, not a cross-base program.

## Boundaries

This decision implements, rather than replaces, A1-A8 and L1-L3. It changes no
config defaults, lock contract, confirmation proof, candidate wire format,
acceptance transaction, CLI phase ordering or release target.
The exact helpers and tests must be reviewed with S3. Resource measurements
and full/native validation remain required before this ADR is accepted.
