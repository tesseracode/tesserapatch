# ADR-042 — Ordered recipe no-op proof

**Status**: Proposed — implementation clarification pending independent review

**Date**: 2026-09-09

**Clarifies**: ADR-036 D7/D14 and ADR-029 D3/D4/D7

**Scope**: bounded S5 apply/dry-run correctness; no coverage-domain, schema,
replay-eligibility or producer-policy change

## Problem

An initial-tree `already-present` result is not a permanent skip instruction.
Starting at `B`, two operations authorized by initial `hash(B)` that write
`A`, then `B`, must finish at `B`. Caching the second operation's initial
postimage equality instead leaves `A` while reporting two successes.

Conversely, an empty gate (or mismatching expected hash) accepted **only**
because initial bytes equal `B` does not authorize writing `B` after an
earlier operation changes it. A runtime-only refusal would discover this
predictable failure after mutation, violating ADR-029 D3.

## Clarification

1. **Preserve initial authorization.** Every existing preimage check still
   runs against the initial tree before mutation. Exact expected hash, or
   explicit-empty gate plus absent target, grants the original write
   permission for this invocation. Earlier outputs grant no new permission.
   Missing expected targets, unreadable targets and malformed gates retain
   their initial refusals; equality cannot bypass gate grammar.
2. **Separate permission from witness.** Exact initial postimage alone grants
   only a no-write exemption. Before mutation, project the relevant ordered
   prefix and compare exact bytes at each candidate's own sequential position.
   For originally authorized writes, equality permits skipping; inequality
   retains ordinary execution, even if the sequential preimage differs.
   For postimage-only candidates, an invalidated or unprovable witness is
   drift before **any** recipe operation executes.
3. **Respect operation order and failure semantics.** Project in-place
   writes, append, first replacement (including empty search), and directory
   operations. Reuse the executor's replacement helper and `created_by`
   validator. Known failed operations do not change the image; a failed
   restoring operation cannot establish a witness. Non-effect and restoring
   prefixes are allowed, not blanket-rejected as overlaps. Legacy omitted
   gates keep their original warning and ordinary writes, never new skips.
4. **Identify aliases safely.** Normalize lexical paths; resolve internal
   symlinks through existing ancestors; use `os.SameFile` for existing
   hardlinks. Equivalent proven absent paths share a group, including paths
   through existing internal directory symlinks. Keep `EnsureSafeRepoPath`;
   additionally check resolved containment for the proof. Physical escapes
   in the examined prefix are hard path-safety errors, never downgraded.
   Dangling/cyclic/unresolvable aliases are not assumed absent or unrelated.
   Known `ENOTDIR` failures cannot mutate a group. Cross-path structural
   changes for initially absent groups make optional proof unavailable,
   rather than revoking their original write permissions.
5. **Bound lifetime and bytes.** Process/release one alias group at a time.
   Retain only operation-index verdicts, previews and path/identity metadata
   across groups; do not retain file-body snapshots across groups. Operation
   strings remain immutable/shared. Each read/projected image has an
   **8,388,608-byte (8 MiB)** logical limit; bounded reads consume at most
   limit+1 bytes, and append/replacement growth is checked before allocation.
   Transient read/string/replacement copies can coexist, so this is not a
   claim that total process memory is 8 MiB. Metadata is linear in recipe
   size; alias grouping/comparison can be quadratic. Unknown/oversized
   images remain unproved through that group's remaining prefix. No
   success-shaped fallback exists for postimage-only permission; ordinary
   originally authorized writes still execute without the optimization.
6. **Recheck at use.** Just before a planned skip, resolve/check the target
   again and compare bounded exact bytes. Divergence never falsely reports
   a skip. Original write permission permits ordinary execution; without it,
   divergence is drift and stops effective execution, with no unauthorized
   fallback write. This is not a filesystem transaction or protection
   against concurrent mutation between a check and use; external changes
   and unexpected I/O failures cannot promise rollback of an executed prefix.
7. **Preserve severity and accounting.** Upfront/runtime drift uses the
   existing supersession router and its audit-not-safety disclaimer.
   Explicit superseded drift can therefore still warn and write, by existing
   policy, not newly inferred permission. Later-touch warnings still run
   independently. A proved/rechecked no-write increments both `Applied`
   and `Skipped` and retains D14's exact message. Dry-run uses ordered
   previews for relevant same-target groups without writing; unrelated
   operation previews and their existing limitations remain unchanged.

## Alternatives rejected

- Cached initial equality: wrong `B → A → B` result and false accounting.
- Sequential preimage reauthorization: rejects previously authorized
  recipes or incorrectly grants permission from earlier outputs.
- Runtime-only checks: predictable late refusal, or unauthorized fallback.
- Blanket overlap invalidation: wrongly refuses harmless/restoring prefixes.
- Whole-worktree/body simulator: unnecessary retained bytes and topology
  scope; this proof is ephemeral and local to relevant alias groups.

## Focused acceptance

- Matching-initial-preimage `B → A → B → B` succeeds with one final skip;
  gated/legacy write, append, replace and lexical/hardlink/symlink aliases
  cannot preserve an invalid cached skip.
- Empty/mismatching-gate postimage-only invalidation refuses all writes,
  including unrelated earlier operations; superseded drift warns instead.
- Unchanged/restoring prefixes retain no-write success; failed search,
  directory or `created_by` operations are not simulated as successful writes.
- Initially absent aliases can be created and restored under their original
  empty-gate authority; missing expected targets/read failures/malformed
  gates and unsafe paths cannot be rescued by a prefix.
- Injected execution divergence cannot silently skip or gain write authority.
- Deliberately oversized reads/append/replacement outputs are rejected by
  the same production proof helpers; ordinary authorized large writes remain
  executable. Unsafe and unprovable alias inputs exercise the real resolver.

Implementation: `internal/workflow/recipe.go`, `writefile_safety.go`,
`recipe_prefix_precheck.go`; focused tests:
`internal/workflow/recipe_authority_s5_apply_test.go`.
Formatting only at implementation handoff; Go validation and independent
review belong to the coordinator. This proposal does not itself accept S5.
