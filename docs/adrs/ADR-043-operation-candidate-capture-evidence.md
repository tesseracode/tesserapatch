# ADR-043 — Operation-candidate capture-evidence identity binding

**Status**: Proposed — operator-selected direction; pending independent review
**Date**: 2026-09-22
**Owner**: Core
**Issue**: [GH #13](https://github.com/tesseracode/tesserapatch/issues/13)
**Companion**:
[ADR-037](./ADR-037-reconcile-operation-replay-candidate-authority.md) rev-7
and
[PRD-reconcile-operation-replay-candidate](../prds/PRD-reconcile-operation-replay-candidate.md)
rev-7
**Depends on**:
[ADR-041](./ADR-041-independent-capture-event-evidence.md) rev-1
**Preserves**:
[ADR-039](./ADR-039-coverage-complete-operation-domain.md),
[ADR-040](./ADR-040-p2-publication-reason-semantics.md),
ADR-036 D16's exact-canonical-origin rule, and the existing no-autorepair /
separate-assignment boundaries

## 1. Problem and bounded choice

ADR-041 §7 left one GH #13 planning decision explicitly open: once
`artifacts/recipe-capture-event.json` (**E**) becomes a required paired input
beside `artifacts/recipe-coverage.json` (**C**), should operation-candidate
identity bind E directly, or should GH #13 only perform a later immutable
recheck that does not enter candidate identity?

The operator selected the **identity-digest** direction for this bounded
follow-up.

This ADR is planning-only. It grants **no** runtime implementation authority by
itself.

## 2. Why existing primitives do not already bind E

Current GH #13 rev-6 identity binds `coverage_sha256`, `patch_sha256` and
`recipe_sha256`.

That is insufficient for E:

- `coverage_sha256` proves the exact bytes of C, not the independent
  capture/reference/event/observation descriptor E.
- ADR-041 deliberately makes E a separate artifact because C alone is not an
  adequate source for those facts.
- A candidate whose immutable identity omits E cannot distinguish "same C,
  different E" from "same C, same E", which weakens stale detection, rejection
  reuse and acceptance rechecks precisely where ADR-041 said GH #13 must
  tighten them.

Therefore the repository has **no existing primitive** that already gives GH #13
the needed immutable E binding.

## 3. Decision

### D1 — Candidate identity binds canonical E content

`candidate.json` and the D19 immutable identity tuple gain:

```text
capture_event_sha256
```

This field is the SHA-256 of the **strict-decoded, canonically re-encoded**
`artifacts/recipe-capture-event.json` bytes defined by ADR-041 §3.2.

It is **not**:

- a raw file hash,
- a generation ID,
- a mutable post-hoc recheck bit,
- a source-body digest,
- or a substitute for offline reconstruction.

### D2 — Canonical digest, not raw file bytes

The digest is computed over ADR-041's canonical JSON encoding of E after strict
decode and validation.

Consequences:

- formatting-only raw E rewrites that preserve exact field values keep the same
  `capture_event_sha256` and therefore the same candidate identity;
- semantic E changes rotate the digest and therefore the candidate identity;
- duplicate members, unknown fields, unsupported versions, invalid typed values
  and other strict-decode failures prevent the digest from existing at all.

### D3 — C keeps its raw-byte binding

`coverage_sha256` remains the exact raw-byte hash of C, unchanged from the
accepted baseline.

The two artifacts therefore play different roles:

- **E** contributes a canonical semantic identity digest.
- **C** contributes the exact published coverage bytes.

Binding both is intentional and non-redundant.

### D4 — Identity binding does not create replay authority by itself

Adding `capture_event_sha256` to identity does **not** mean E alone becomes a
replay proof.

Acceptance and candidate reuse still require:

1. the full paired E/C gate inventory,
2. exact bound-artifact validation,
3. offline reference reconstruction,
4. the existing candidate execution/postcondition/idempotency proofs.

`capture_event_sha256` is an immutable identity input, not a shortcut around
those checks.

## 4. Consequences for GH #13 planning

The selected direction implies the rev-7 amendments in ADR-037 / the companion
PRD must do all of the following coherently:

1. **Eligibility**: E becomes part of the closed fifteen-gate inventory without
   resizing `gatesPassed[15]`.
2. **Proof carrier**: the in-process proof binds canonical E content through
   `captureEventSHA256`.
3. **Schema**: `candidate.json.bindings` gains `capture_event_sha256`.
4. **Staleness / reuse / rejection**: all immutable-binding comparisons expand
   from a triple to a quartet.
5. **Acceptance**: step-7 staging, step-8 snapshots, step-11 publication and
   rollback/recovery all handle E and C together, with **E before C**.

## 5. Exact amendment map

The actual closed parity IDs remain:

```text
A, B, C1, C2, D1, D2, D3, E, F
```

rev-7 changes these IDs:

| Parity ID | Why it changes |
|---|---|
| `A` | proof carrier / `captureEventSHA256` / gate cross-check wording |
| `B` | fifteen-gate paired E/C inventory |
| `D1` | `candidate.json.bindings.capture_event_sha256` |
| `E` | acceptance step 4/7/8/11 now bind, stage, snapshot and publish E with C |
| `F` | `reconcile-accept` publication now describes E-before-C semantics |

rev-7 preserves these IDs byte-identically:

| Parity ID | Why unchanged |
|---|---|
| `C1`, `C2` | alignment math / limits unchanged |
| `D2`, `D3` | `state.json` shape and closed enums unchanged |

Normative ADR-037 / PRD sections amended by this choice:

| Surface | Required effect |
|---|---|
| ADR-037 D4 / PRD §6.5 | paired E/C eligibility without changing the 15-gate cardinality |
| ADR-037 D5/D8/D10/D12 and PRD counterparts | validated E reference/capture/event/availability inputs, never generation or current-parent substitution |
| ADR-037 D19/D20/D21 / PRD §6.10-§6.11 | `capture_event_sha256` in identity/schema/staleness/reuse |
| ADR-037 D29/D29b/D30/D31 / PRD §6.14-§6.16 | E staged/snapshotted/published/rolled back/recovered with C |
| PRD §7.1 | refusal vocabulary for paired E/C gating |
| PRD §9 | appended ROC rows and updated totals |

## 6. Alternatives rejected

### A — Separate immutable recheck-only path

Rejected.

It would prove that GH #13 *noticed* E at acceptance time, but candidate
identity, stale detection, rejection reuse and accepted-audit lookup would all
remain silent about E drift.

That is weaker than the operator-selected direction and reintroduces precisely
the ambiguity ADR-041 §7 asked GH #13 to close.

### B — Raw E file hash

Rejected.

It would make formatting-only or whitespace-only JSON rewrites rotate candidate
identity even when strict-decoded E content is unchanged. The operator selected
the opposite property.

### C — Reuse generation metadata or current parent state

Rejected.

ADR-041 already closed these as non-authoritative substitutes for the paired
descriptor. They remain diagnostics or current-environment facts only.

### D — Persist E bodies or source bodies inside candidate metadata

Rejected.

The planning amendment preserves the repository's existing no-source-bodies
boundary. GH #13 binds E by digest, not by copying payload into a new runtime
artifact.

## 7. Non-effects and preserved boundaries

This ADR does **not**:

- widen ADR-039's complete-operation domain,
- change ADR-040's semantic reason truth,
- change ADR-036 D16's exact canonical-origin rule,
- fold GH #24's wider domain planning into GH #13,
- fold GH #19's manual-adoption work into GH #13,
- authorize runtime code, schema shipping, assets, tests or release actions.

The v0.17.0 producer prerequisite is already shipped and satisfied. After
planning review, runtime implementation is still a **separate assignment**.

