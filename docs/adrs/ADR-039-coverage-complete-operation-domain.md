# ADR-039 - Conservative v1 Coverage-Complete Operation Domain

**Status**: Accepted operator policy decision (2026-09-08)
**Owner**: Core
**Issue**: [GH #15](https://github.com/tesseracode/tesserapatch/issues/15), S3
**Amends**: ADR-036 D3's admissible present-state proof and D5's operation domain
**Companion**: PRD-recipe-generation-authority, S3
**Follow-up**: [GH #24](https://github.com/tesseracode/tesserapatch/issues/24)

## Context

S3 implementation exposed two conflicts in Accepted ADR-036 rev-7:
D5 permits an exact-postimage replacement exception, but later categorically
excludes replacement recipes from complete v1 coverage. D3's cross-base table
also has no complete branch for replacement-only recipes or creations without
an explicit preimage gate.

The operator selected: "Conservative v1 (recommended): complete only for
preimage-bearing write-file operations", and requested a separate planning
task for the remaining broader domain. This addendum records the current
decision; GH #24 owns any future widening. S3 implementation acceptance and
resource-gated validation remain tracked in CURRENT, not asserted by this
policy's acceptance.

## Decision

The admissible v1 complete-operation domain is exactly:

```text
type == "write-file" AND preimage_hash is present and non-null
```

The preimage value must still satisfy the existing recipe grammar and
precondition rules. An explicit empty value gates creation; an existing-file
write normally carries its exact expected hash. This domain is necessary,
not sufficient: all ten ADR-036 completeness predicates still apply.

Every assigned `append-file`, every assigned `replace-in-file` (including
an exact-postimage case), and every assigned `write-file` that omits its
preimage gate is outside this domain. Such an operation contributes
`operation-not-reclassifiable` to each assigned effect, and the record is
incomplete. Here that code means no **admissible v1** present-state proof,
not a claim that an exact mathematical transformation is impossible.
The pure simulator may still measure whether an excluded operation reproduces
the observed postimage; that success does not satisfy the no-write
reclassification requirement.

An admissible gated write whose actual precondition fails remains a simulation
failure, not an unsupported-operation classification. Surplus or unassigned
operations keep the existing record-level reasons; do not invent an effect
assignment merely to attach an operation-domain reason. All other applicable
reason, observation and safety rules remain unchanged.

This supersedes D5's exact-replacement completeness exception for v1.
Preserved manual/provider recipes and legacy ungated write-file recipes
remain byte-identical, and their existing execution behavior is unchanged.
S3 does not publish artifacts, manufacture provenance, alter consumer behavior
or authorize replay.

The existing D3 cross-base cases are now total over admissible complete
records:

| Record | cross_base_status |
|---|---|
| Incomplete | `unsupported` |
| Complete, with at least one existing-file write | `consumer-derivation-required` |
| Complete, exclusively explicit-empty-gated creations | `reference-tree-only` |

No wire field, schema version, enum value or reason code is added.
The canonical schema and ten-predicate blocks remain unchanged. This is a
narrow qualification of admissible v1 reclassification, not an alternative
completeness predicate or a new authority label.

## Consequences and follow-up

S3 can finish without returning a contract-adjudication error for ordinary
legacy/replacement recipes: it emits truthful incomplete records instead.
The unsupported complete shapes are not silently assigned replay scope.

GH #24 is a non-blocking planning task to write an addendum, ADR and/or PRD
for broader exact replacement and ungated-write support. It must define
admissible proofs, total cross-base classification, required data carriers,
backward compatibility and mutation-sensitive acceptance cases before any
implementation. It is distinct from GH #19's historical provenance adoption
and must account for the accepted GH #13 consumer boundary.

## Verification requirements

Test exact replacement, append, ungated creation and ungated existing-file
writes as incomplete, preserving recipe bytes and execution semantics.
Keep gated creation/existing-file positive controls, precondition-mismatch
negative controls, exact reason allocation, and no-publication/no-consumer
guards. A wrong-input fixture that drops the gate or admits replacement
must fail the same v1 completeness validator.
