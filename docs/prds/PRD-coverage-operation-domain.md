# PRD - Widening the coverage-complete operation domain

**Status**: Direction accepted, deferred — operator answered §9
(2026-11-06): option G, but only when a concrete user needs complete
coverage for replace or append. No slice is scheduled; this PRD authorizes
no implementation, schema expansion, consumer integration or release.
Independent review APPROVED at rev-2 (2026-11-06,
`docs/supervisor/LOG.md`). The current conservative domain (ADR-039) stays
in force until the new ADR (Q5) is written and accepted.
**Date**: 2026-11-06
**Owner**: Core
**Issue**: [GH #24](https://github.com/tesseracode/tesserapatch/issues/24)
**Related**: [ADR-036](../adrs/ADR-036-recipe-coverage-authority.md) (D3, D5, D16), [ADR-039](../adrs/ADR-039-coverage-complete-operation-domain.md),
[PRD-recipe-generation-authority](PRD-recipe-generation-authority.md) (§6.5, RGA-230–232), [SPEC.md](../../SPEC.md) §"Recipe generation authority", §"Operation replay candidates"; GH #13, GH #15, GH #19.

---

## 1. Summary

Recipe coverage is `complete` only when every operation in the recipe is a
`write-file` that carries a `preimage_hash` (ADR-039). Exact replacements,
appends and legacy ungated writes are executed as before but always yield
`incomplete` with `operation-not-reclassifiable`.

GH #24 asks whether that domain can be widened soundly. This draft
evaluates each excluded shape, defines what an admissible present-state
proof must be, proposes a total `cross_base_status` table for every shape
that could be admitted, and lists the decisions the operator must take.

**Drafter's recommendation:** keep the v1 domain for every shape that has
no gate the executor enforces. Widen only through an explicit, opt-in,
executor-enforced gate on the operation itself (§5, option G), and keep
GH #19's provenance adoption separate.

## 2. Current state

- **Domain.** `type == "write-file" AND preimage_hash is present`
  (`docs/adrs/ADR-039-coverage-complete-operation-domain.md:27-31`;
  `internal/workflow/recipe_coverage_simulation.go:126-128`).
- **Predicates.** Completeness needs all ten ADR-036 D3 predicates;
  predicate 9 is exact simulation from the immutable preimage and
  predicate 10 is "reclassifying that simulated result reports every
  operation already-present and writes no byte"
  (`docs/adrs/ADR-036-recipe-coverage-authority.md:1164-1169`;
  `internal/workflow/recipe_coverage.go:110-125`).
- **Simulation is not reclassification.** ADR-039 states that an excluded
  operation may still simulate exactly, and that this does not satisfy the
  no-write reclassification requirement (`ADR-039:44-46`).
- **Cross-base table** (`ADR-039:60-67`;
  `internal/workflow/recipe_coverage.go:379-400`): incomplete →
  `unsupported`; complete with at least one existing-file write →
  `consumer-derivation-required`; complete with only explicit-empty-gated
  creations → `reference-tree-only`. `cross_base_status` is producer scope,
  never consumer permission (`ADR-036:1194-1213`).
- **Execution.** `write-file` overwrites (`internal/workflow/recipe.go:253-257`);
  its gate is enforced by the apply precheck, which permits a write only
  when authorized, ungated, **or the feature is superseded**
  (`internal/workflow/recipe_prefix_precheck.go:283`). For a superseded
  feature ADR-029 D7 downgrades a preimage mismatch from refusal to a
  warning with a note, and execution proceeds
  (`recipe_prefix_precheck.go:293`;
  `internal/workflow/writefile_safety.go:283-293`). ADR-036 keeps D7 in
  full (`ADR-036:1614-1615`). Gate enforcement is therefore unconditional
  only for features that are not superseded.
  `replace-in-file` replaces the first match of an existing file
  (`recipe.go:259-273`); `append-file` opens with `O_APPEND` and always
  appends (`recipe.go:275-287`). Neither carries a hash gate.
- **Pinned boundary.** `TestRGAS3ConservativeReclassificationDomain`
  (`internal/workflow/recipe_authority_s3_simulation_test.go:144-218`) and
  RGA-230–232 (`docs/prds/PRD-recipe-generation-authority.md:3292-3294`).
- **GH #13 consumer.** Eligibility requires `coverage_status: complete`
  (`internal/workflow/operation_candidate_eligibility.go:148-153`) and
  then derives replay from effects, independently (SPEC §"Operation replay
  candidates").

## 3. What an admissible present-state proof is

Predicate 10 must not be satisfiable by restating predicate 9. This draft
proposes that a proof is **admissible** only if all of these hold:

1. **Operation-local.** It is decided from the operation's own fields and
   the target's present bytes, existence and mode. It does not read the
   coverage record, a parent's body, history or a stored label.
2. **Executor-enforced.** For a feature that is not superseded, the apply
   path refuses the same operation when the proof's precondition fails. A
   proof the executor ignores is a statement about the capture, not about
   execution. A superseded feature keeps ADR-029 D7's documented
   downgrade: the mismatch is still detected and reported, as a warning,
   and execution proceeds (§2). The criterion accepts exactly that
   enforcement and no weaker one.
3. **Trichotomous and exact.** For any present state it answers exactly
   one of *already present*, *applicable* or *conflict*, by exact byte or
   hash comparison.
4. **Non-vacuous.** *Already present* and *applicable* are disjoint for the
   operation's own data (for example preimage ≠ postimage).

Gated `write-file` meets all four, with (2) holding in the D7 form above.
**Rejected** alternative: using the
coverage effect's whole-file hashes as the gate. It is exact and
non-vacuous, but fails (1) and (2): the executor would still replace the
first match or append again on any other content.

## 4. Shape-by-shape evaluation

| Shape | Proof available today? | Why |
|---|---|---|
| Ungated `write-file`, existing target | No | Content equality gives *already present*, but nothing separates *applicable* from *conflict*; the executor overwrites any content. |
| Ungated `write-file`, new target | No | The executor overwrites a file that appeared since capture; no explicit-empty gate. |
| `replace-in-file`, exact postimage | No | The operation carries no postimage or preimage; absence of the search text does not prove the replacement happened. |
| `replace-in-file`, general | No | First-match semantics; no uniqueness proof. |
| `append-file` | No | The same bytes may already end the file for unrelated reasons; appending is not idempotent. |
| Mixed sets | Inherits the weakest member | Predicate 10 is over every operation. |

None of the excluded shapes can be admitted without a new carrier. That
confirms ADR-039's reading of `operation-not-reclassifiable`: there is no
admissible proof, not that the transform is inexact.

## 5. Options

### K. Keep the v1 domain (status quo)

No change. Excluded shapes stay `incomplete`. Lowest risk; GH #13 sees no
new eligible features.

### G. Opt-in operation gates (recommended if widening is wanted)

Add optional, explicit hash fields to the operation, enforced by the apply
precheck exactly as `preimage_hash` is today:

- `replace-in-file`: `preimage_hash` and `postimage_hash` over the whole
  target. Present = postimage → *already present*; present = preimage and
  the search text occurs exactly once → *applicable*; anything else →
  *conflict* (refused).
- `append-file`: the same two fields. This is the only way to support
  append without assuming idempotency: the gate, not the operation, decides.
- Ungated `write-file`: no new field; the existing `preimage_hash` is the
  gate. Legacy recipes stay ungated and incomplete.

**Supersession.** The new gates inherit `write-file`'s ADR-029 D7
behaviour unchanged: for a superseded feature a gate mismatch is detected,
reported as a warning with the supersession note, and execution proceeds,
through the same `out.superseded` branch
(`recipe_prefix_precheck.go:283`, `:293`). Hard-refusing superseded drift
for the new gates only, or skipping their check, would each need an ADR
amending D7, which this PRD does not propose (§9 Q6).

Recipes without the new fields keep today's bytes and behaviour (RGA-232).
Only producers may emit gated forms; preserved manual/provider recipes stay
byte-identical (ADR-036 D16). This is a recipe schema change and needs an
ADR.

### M. Explicit migration of legacy recipes

An operator command that replaces a preserved legacy recipe with the fresh
gated derivation. That is provenance adoption, which is GH #19's scope; it
must not be folded into this work.

## 6. Proposed `cross_base_status` table (option G)

Evaluated per record; first matching row wins.

| # | Record | `cross_base_status` |
|---|---|---|
| 1 | Incomplete | `unsupported` |
| 2 | Complete; any effect modifies a pre-existing file (gated `write-file`, gated `replace-in-file` or gated `append-file`) | `consumer-derivation-required` |
| 3 | Complete; every effect is a creation by a `write-file` with an explicit empty `preimage_hash` | `reference-tree-only` |
| 4 | Anything else complete | Contract error (no record is published), as today (`recipe_coverage.go:399`) |

Mixed sets fall to row 2 whenever any member modifies an existing file, so
widening never makes a record more permissive than its least gated member.
`contextual_hint` stays advice only and never changes this table
(`ADR-036:1187-1193`).

## 7. Acceptance matrix (sketch)

Each case needs a fixture that fails under the named wrong input.

| # | Case | Expected | Wrong input that must fail |
|---|---|---|---|
| A1 | Ungated legacy write | `incomplete`, `operation-not-reclassifiable`, bytes preserved | Admit `write-file` without a gate |
| A2 | Gated replace, present = postimage | Reclassified *already present*, no write | Decide from search-text absence |
| A3 | Gated replace, present = preimage, search text twice | Conflict, refused at apply | Drop the uniqueness check |
| A4 | Gated append, present = postimage | *Already present*, no write | Append unconditionally |
| A5 | Gated append, present = preimage + unrelated tail | Conflict | Accept a suffix match |
| A6 | Ungated replace or append | Unchanged behaviour, `incomplete` | Use coverage-effect hashes as the gate |
| A7 | Mixed: gated creation + gated replace | `consumer-derivation-required` | Take the most permissive member |
| A8 | Gate in recipe, precheck bypassed | Refused at apply | Enforce the gate only in coverage |
| A9 | Exact simulation, no gate | `incomplete` | Use predicate 9's success as predicate 10 |
| A10 | Superseded feature; gated replace or append whose present bytes match neither preimage nor postimage | Drift reported as a warning with the supersession note; execution proceeds (ADR-029 D7), exactly as for a gated `write-file` | Hard-refuse superseded drift for the new gates, or skip their check when superseded |

## 8. Impact on other contracts

- **GH #13.** More features could pass the `coverage-complete` gate, but
  E1–E15 and the effect-derived replay stay the only authority. A gated
  replace or append is an existing-file effect, so its records are
  `consumer-derivation-required`, the same class the consumer already
  handles. No consumer change is needed for G; a consumer test must show
  that a G record is never treated as authorization.
- **GH #19.** Unchanged. Option M is its scope.
- **Verify/doctor (S5).** A record that was `incomplete` (rung 3, warn)
  could become `complete` (rung 6) only after re-recording with a gated
  derivation; existing features keep their current rows.
- **Release.** G is a minor release with a recipe schema addition, an ADR
  superseding ADR-039's domain, and asset parity for any skill that
  documents recipe operations.

## 9. Questions for the operator

Operator answers, 2026-11-06 (recorded in `docs/handoff/CURRENT.md`):

1. **Widen at all?** K (no change) or G (opt-in gates)? Recommended: G, but
   only when a concrete user needs complete coverage for replace/append.
   **Answer: G, only when a concrete user needs it.** Until then K stays in
   effect.
2. **Append support** via gates (G), or never? Recommended: via gates.
   **Answer: via gates.**
3. **Admissibility criteria (§3):** accept the four conditions as the
   definition? Recommended: yes. **Answer: yes.**
4. **Legacy ungated recipes:** stay incomplete, with migration left to
   GH #19? Recommended: yes. **Answer: yes.**
5. **ADR shape:** a new ADR superseding ADR-039's domain, rather than
   another ADR-036 addendum? Recommended: new ADR. **Answer: new ADR**,
   written when G is scheduled.
6. **Supersession:** keep ADR-029 D7's warning downgrade for the new gates,
   identical to gated `write-file`? Recommended: yes; changing it needs
   its own ADR amending D7. **Answer: yes.**

## 10. Claims audit

| # | Claim | Anchor | Verified |
|---|---|---|---|
| 1 | v1 domain is gated `write-file`. | `ADR-039:27-31`; `internal/workflow/recipe_coverage_simulation.go:126-128` | yes |
| 2 | Predicates 9 and 10. | `ADR-036:1164-1169` | yes |
| 3 | Completeness requires exact simulation and all-already-present. | `internal/workflow/recipe_coverage.go:110-125` | yes |
| 4 | Simulation success does not satisfy reclassification. | `ADR-039:44-46` | yes |
| 5 | Cross-base table and its contract error. | `ADR-039:60-67`; `internal/workflow/recipe_coverage.go:379-400` | yes |
| 6 | Cross-base is producer scope, not permission. | `ADR-036:1194-1213` | yes |
| 7 | Execution of the three operations. | `internal/workflow/recipe.go:253-287` | yes |
| 8 | Gate enforcement in the apply precheck, permissive when the feature is superseded. | `internal/workflow/recipe_prefix_precheck.go:283`, `:293` | yes |
| 11 | ADR-029 D7 downgrades superseded preimage drift to a warning; ADR-036 keeps D7. | `internal/workflow/writefile_safety.go:283-293`; `ADR-036:1614-1615` | yes |
| 9 | Pinned v1 boundary tests. | `internal/workflow/recipe_authority_s3_simulation_test.go:144-218`; `PRD-recipe-generation-authority.md:3292-3294` | yes |
| 10 | GH #13 requires complete coverage. | `internal/workflow/operation_candidate_eligibility.go:148-153` | yes |
