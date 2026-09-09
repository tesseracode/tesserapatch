# ADR-040 - P2 Publication Reasons Preserve Semantic Truth

**Status**: Accepted operator policy decision (2026-09-09)
**Owner**: Core
**Issue**: GH #15, S4
**Amends**: ADR-036 D15's P2 writing-event wording and the corresponding
PRD-recipe-generation-authority producer-output qualification
**Preserves**: ADR-036 D3 reason conditions, D16 origin proof and ADR-039's domain

## Context

Independent S4 review identified a conflict for a patch-writing P2 event.
A preserved, compactly encoded gated write-file recipe can still cover,
simulate and reclassify the new patch exactly while failing D16's total
canonical-byte equality. P2 preserves it and records the stale marker.

D15's "always pair" wording would require `producer-patch-rewrite` and
`recipe-not-regenerated`, while D3 expressly forbids those codes when the
preserved recipe still explains the patch exactly. The operator selected
`semantic-reasons`: retain D3's truthful conditions rather than manufacture
reasons for an origin-formatting mismatch.

## Decision

The rewrite-reason pair is raised only when a successful patch rewrite leaves
a non-regenerated recipe unable to exactly cover, simulate and reclassify the
patch. Both codes occur together when applicable; neither is added merely
because D16's raw canonical-byte comparison fails.

For a P2 patch-writing event with a semantically exact but non-D16 recipe:

- Preserve the recipe's exact bytes and existing provenance; fabricate no
  origin proof or provenance repair.
- Preserve P2's existing stale-marker behavior. Coverage is incomplete
  because `recipe-stale-marker-present` applies, together with any other
  genuinely applicable conditions.
- Do not add `producer-patch-rewrite` or `recipe-not-regenerated` when their
  semantic raising condition is false.
- Do not redefine semantic explanation to fail merely because a global
  stale marker exists. Explanation, origin proof and marker status are
  separate facts.

This narrows D15's unconditional "always pair" interpretation to the condition
already defined by D3. It also qualifies PRD producer-output wording that
equates every non-D16 P2 result with a recipe that no longer explains the
patch. Incomplete status/reason reporting remains mandatory and exact.

The category-(c) checkpoint remains separate: it writes no patch, recipe,
provenance, generation or state. It repairs coverage only, using the existing
semantic completeness rules; no rewrite reason is applicable to that event.
D16 remains the prerequisite for an origin/provenance claim, not a reason
to invent a failed semantic predicate.

No wire field, enum, reason code, canonical predicate or complete-operation
class changes. ADR-039 remains in force, and GH #24 remains out of S4 scope.
No consumer/replay authorization is introduced.

## Verification requirements

Use paired real P2 writing-event fixtures: formatting-only mismatch preserves
semantic explanation and has no rewrite codes; actual semantic drift has
both codes. Assert unchanged recipe/provenance bytes, truthful stale-marker
and coverage status, and exact sorted reasons. Retain the independent
coverage-only checkpoint and non-D16 writing-event refusal controls for
invalid publication inputs.

The operator decision resolves policy. Independent review and resource-gated
implementation validation remain required before S4 acceptance.
