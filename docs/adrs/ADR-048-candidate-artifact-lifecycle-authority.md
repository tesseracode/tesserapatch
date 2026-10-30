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
7. **Preparation mint site**: keep preparation private to workflow, where the
   existing strict E1-E15 evaluator lives. Recompute eligibility and retain its
   exact patch/effect/descriptor snapshot; accept no caller-provided eligibility,
   binding digest map or execution-success callback. Candidate helpers expose
   derivation metadata and typed structural revalidation, not a second
   eligibility authority. This avoids copying decoders or creating a package
   cycle merely to preserve the draft's unsafe public preparation constructor.
8. **Continuous finalization hold**: under one borrowed feature capability,
   revalidate recovery/coexistence, current eligibility/state/bindings, rejected
   IDs and the existing candidate before creation. Keep global holds limited
   to worktree add/remove. Bind reuse to the actual rooted candidate's metadata
   and the same preparation; re-run production structural validation.
9. **Failure consistency**: inventory pending candidate metadata under the
   feature hold, not just the status pointer. Orphan/incomplete publication must
   disable derivation, never become an invisible pending candidate. Prune before
   terminal stale/rejected state changes; detect and finish safe interrupted
   pointer cleanup or refuse explicitly. A failed operation is not success,
   and this generation lifecycle does not introduce an acceptance journal.
10. **Index-independent worktree proof**: verify detached HEAD/registration
    before and after structural validation. Build a materialized worktree tree
    through the existing fresh private index, then compare that immutable tree
    directly with the bound upstream for both path-set and patch bytes.
    Never compare the mutable candidate index with its worktree. This avoids
    relying on staging/cache flags: staged unmodeled changes remain visible,
    while merely staging the correct modeled result changes no proof.
    Before adding files to that private index, reuse the existing conversion
    policy against the same seeded index with a hardened read environment.
    Refuse applicable filters and byte-changing attributes/configuration;
    no filter runs before refusal. Disable stat/fsmonitor shortcuts for the
    materializing add. A Git object tree is not a raw-byte proof without this
    conversion boundary.
    On POSIX, force core.fileMode=true only for this private add so executable
    changes cannot be suppressed by repository configuration. Windows has no
    POSIX executable bit and retains Git's logical tracked-mode handling.
    Likewise observe actual symlinks on POSIX. Disable sparse-index/checkout
    and case-folding shortcuts for the private materialization so hidden or
    renamed paths cannot inherit logical baseline entries. These are command-
    local settings, not repository configuration changes.

## Non-goals in this draft

- No S6 reconcile-phase wiring or shared merge integration.
- No CLI flag wiring, acceptance transaction, recovery-resume transaction, or
  release workflow changes.
