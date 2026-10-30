# ADR-048 — Candidate artifact integrity and lifecycle authority (S5 draft)

**Status**: Proposed (implementation draft for GH #13 S5; not accepted)  
**Date**: 2026-10-30  
**Related**: ADR-037, ADR-043, ADR-044, ADR-046, ADR-047

## Context

S5 introduces immutable `candidate.json`, mutable `state.json`, deterministic
`rc_<12hex>` identity, lock-first preflight ordering, and rejected-candidate
audits. The contract requires strict decode/encode, pre-execution identity, and
integrity rechecks that never rely on mutable labels.

## Proposed decisions

1. **Identity authority**: compute `candidate_id` from the D19 tuple only
   (including `source_feature_state`, sorted operation/effect proof hash lists).
   Execution result fields (`candidate_diff_sha256`, `candidate_tree_sha256`)
   are excluded from identity and compared only as integrity.
2. **Effect authority**: `effect_sha256` remains the ADR-036 descriptor hash,
   never substituted with fragment hashes.
3. **Boundary authority**: compute `boundary_proof_sha256` from the full tuple,
   persist only the digest (never offsets/counts/record bodies).
4. **Tree integrity hash semantics (conservative)**: because S4 currently
   returns Git tree object IDs (40-hex SHA-1 in this repository) while D1 names
   `candidate_tree_sha256`, derive the stored field as
   `SHA-256("git-tree-object-id:" + <tree-object-id>)`. This avoids relabeling
   SHA-1 as SHA-256 while preserving deterministic integrity checks.
5. **Lifecycle preflight**: enforce lock → recovery-check → stale-check →
   feature-state precondition under the per-feature lock; stale transition keeps
   immutable audit files and clears the status pointer only after safe prune.
6. **Rejected log authority**: `rejected.jsonl` lines contain metadata only
   (`candidate_id`, source state, commits, four binding hashes), no timestamp
   and no source/diff bodies.

## Non-goals in this draft

- No S6 reconcile-phase wiring or shared merge integration.
- No CLI flag wiring, acceptance transaction, recovery-resume transaction, or
  release workflow changes.
