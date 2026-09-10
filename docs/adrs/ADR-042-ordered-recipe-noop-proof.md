# ADR-042 — Ordered recipe no-op proof

**Status**: Accepted — independent clarification/correction review approved 2026-09-09; runtime validation pending

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
   Track the gate's specific inputs: `.tpatch/config.yaml` and, when the
   dependency flag is on, the owning feature's `status.json`. If a prefix may
   change a needed input, the dependent append/replacement effect becomes
   unproved before execution; initial metadata must not certify its later
   success. This is conservative dependency tracking, not projected metadata
   authorization or whole-worktree simulation. Exact unchanged metadata
   writes and byte-neutral operations do not invalidate the gate; lexical,
   symlink and hardlink aliases participate in the input tracking.
4. **Identify aliases safely.** Normalize lexical paths; resolve internal
   symlinks through existing ancestors; use `os.SameFile` for existing
   hardlinks. Equivalent proven absent paths share a group, including paths
   through existing internal directory symlinks. Keep `EnsureSafeRepoPath`;
   additionally check resolved containment for the proof. Physical escapes
   in the examined prefix are hard path-safety errors, never downgraded.
   Dangling/cyclic/unresolvable aliases are hard containment refusals, not
   assumed absent or unrelated, even under initial empty-gate authority.
   Known `ENOTDIR` failures cannot mutate a group. Cross-path structural
   changes for initially absent groups make optional proof unavailable,
   rather than revoking their original write permissions.
5. **Bound lifetime and bytes.** Process/release one alias group at a time.
   Retain only operation-index verdicts, previews and path/identity metadata
   across groups; do not retain file-body snapshots across groups. Operation
   strings remain immutable/shared. The **8,388,608-byte (8 MiB)** limit is
   for newly materialized or constructed projection bodies, **not** an
   exact-postimage size or eligibility limit. Unchanged files are compared
   byte-for-byte with 32 KiB scratch space, including initial checks and
   runtime skip rechecks. Initial hash gates stream the digest independently;
   hash equality never substitutes for exact bytes or grants authority
   beyond the original gate. Bounded materialization consumes at most
   limit+1 bytes, and append/replacement growth is checked before allocation.
   A full overwrite can reference its existing immutable operation string
   without allocating another body, even when that string exceeds 8 MiB.
   Transient read/string/replacement copies can coexist, so this is not a
   claim that total process memory is 8 MiB. Metadata is linear in recipe
   size; alias grouping/comparison can be quadratic. Size-only projection
   uncertainty can be recovered by a deterministic originally permitted full
   overwrite. It cannot erase unresolved aliases, other I/O uncertainty,
   changed gate inputs or known write failure. No
   success-shaped fallback exists for postimage-only permission; ordinary
   originally authorized writes still execute without the optimization.
6. **Recheck at use.** Just before a planned skip, resolve/check the target
   again and stream exact bytes. Divergence never falsely reports
   a skip. Original write permission permits ordinary execution **only after
   current physical containment is proved**; unresolved/dangling/cyclic
   topology is a hard path-safety refusal, including when superseded.
   A proven-contained ordinary missing or different target retains its
   original fallback behavior. Without original write permission,
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
- An exact 8,388,609-byte postimage on a read-only target skips under empty,
  mismatching and matching gates, preserving D14 accounting and timestamps.
  Same-length changed bytes, trailing bytes and truncation fail the same
  streaming comparison and production precheck; equality supplies no new
  write permission.
- Deliberately oversized materialization/append/replacement outputs remain
  bounded by the production proof helpers. An authorized full overwrite
  restores size-only knowledge; failed restoring operations and unresolved
  aliases do not. Ordinary authorized large writes remain executable.
- Runtime dangling external symlinks refuse without creating a destination,
  irrespective of initial authority or supersession. Ordinary contained
  missing/different targets retain authorized or warned fallback.
- Changed config/status inputs, through lexical and physical aliases, cannot
  certify a restoring `created_by` effect: dry-run and execution refuse
  before unrelated writes or metadata changes. Unchanged metadata and
  operations that do not use `created_by` remain positive controls.

Implementation: `internal/workflow/recipe.go`, `writefile_safety.go`,
`recipe_prefix_precheck.go`; focused tests:
`internal/workflow/recipe_authority_s5_apply_test.go`.
Independent rev-1 review approves the clarification and correction.
Go validation remains with the coordinator; this acceptance does not itself
accept the runtime unit or S5.
