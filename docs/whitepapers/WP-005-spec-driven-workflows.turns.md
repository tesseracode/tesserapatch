# WP-005 — Turn Log

## Turn 1 - R58 - 2026-06-25

**Responding to**: human broker research prompt
**Type**: research

Researched OpenSpec's OPSX workflow and GitHub Spec Kit's spec-driven
development workflow against tpatch's current whitepaper/PRD/ADR/cluster/review
process. Initial conclusion: tpatch should not adopt either workflow wholesale.
The promising overlap is narrower: optional task companions for complex PRDs,
OpenSpec-style behavior deltas for selected proposal docs, and a lightweight
cross-artifact consistency pass before implementation/review. OpenSpec is the
closer philosophical match for brownfield, durable, incremental specs; Spec Kit
is more useful as a template/checklist library for agent task breakdown and
quality gates.

**Asks of next agent / broker**:
- Confirm whether "GitHub Spec" should mean GitHub Spec Kit for future turns, or
  whether another GitHub-internal/public spec workflow should be compared.
- Decide whether the next pass should pressure-test one candidate experiment
  against a recent PRD, without creating or editing PRDs yet.
- Review whether WP-005 should remain a concise exploratory brief or invite a
  second agent to add a counter-view before any graduation step.

## Turn 2 - CO76 - 2026-08-13

**Responding to**: Turn 1 + human broker prepare/SDD council prompt
**Type**: agreement

I am CO76, joining WP-005 to record the requested second-opinion council.
The pass used five independent lenses (Contrarian, First Principles,
Expansionist, Outsider and Executor), five anonymous peer reviews and a
chairman synthesis.

The advisor round genuinely split:

- two favored shipping `prepare` immediately as orchestration;
- two rejected it as misleading state sugar;
- one argued for specifying it now but blocking mutation on trustworthy
  validation/provenance.

Peer review converged on the middle position: three reviewers selected the
First Principles response as strongest and two selected the Contrarian. All
five identified the Expansionist's immediate "Intent Engine" productization as
the largest blind spot because it ignored empty-file acceptance, Path A/B
asymmetry, overwrite protection and partial publication.

The agreed disposition is:

1. `prepare` is a valid product seam, not an implementation authorization.
2. The first slice is read-only `prepare --check`, built on validation repaired
   in the individual phase primitives rather than duplicated in an
   orchestrator.
3. A mutating Path A/Path B bundle waits for non-destructive overwrite,
   provenance, all-or-nothing publication and migration rules.
4. No new lifecycle state: completion remains `defined`; full-bundle
   preparation is optional and does not make exploration mandatory globally.
5. Internal SDD aids remain optional; downstream SDD is encouraged but never
   enforced. Patch maintenance against upstream remains the product.
6. This paper supplies sufficient prior art for two ordered PRDs:
   artifact-validation/provenance first, prepare-intent-bundle second and
   blocked. No ADR exists until a persistent representation is chosen.

Repo grounding confirms the risk is current, not hypothetical:
`AdvanceStateManually` requires the Markdown path but validates content only
for the JSON implement artifact (`internal/store/manual.go:25-31,45-79`);
Path A analysis writes a structured `analysis.json` sidecar consumed by define
(`internal/workflow/workflow.go:88-121`); and `next` distinguishes two
`defined` substates from `exploration.md` presence
(`internal/cli/phase2.go:409-466`).

The whitepaper's `## Agreed` section and product-facing disposition were
updated in this turn. Status remains `Exploring` until the first graduating PRD
is drafted.

**Asks of next agent**:
- Draft `PRD-artifact-validation-and-provenance.md` with a claims audit over
  manual validation, Path A sidecars, notes/provenance and `next` behavior.
- Keep `PRD-prepare-intent-bundle.md` blocked until the prerequisite contract
  is accepted.
- Do not create an ADR or implementation milestone during the first PRD.

## Turn 3 - CO76 - 2026-08-13

**Responding to**: Turn 2 internal/external review
**Type**: agreement

Turn 2 remains unchanged. This turn records the review corrections and
supersedes its final "do not create an ADR during the first PRD" instruction
with the narrower rule below.

**Council traceability.** The lens-to-position mapping was:

- Executor and Expansionist: ship orchestration now;
- Contrarian and Outsider: reject the current proposal as misleading sugar;
- First Principles: specify now, repair validation/provenance first, and defer
  mutation.

Anonymous peer review selected First Principles as strongest three times and
Contrarian twice. That is why the chairman adopted validation-first planning
even though only one advisor initially occupied the middle position.

**Review clarifications accepted.**

1. Current free-text notes cannot prove per-artifact provenance:
   `MarkFeatureState` overwrites the one `Notes` field on each phase
   (`internal/store/store.go:384-400`). The first PRD therefore renders
   provenance `unknown` until an accepted representation proves otherwise. It
   may propose that representation; if it selects one, an ADR is required
   before acceptance for implementation.
2. "All-or-nothing" includes `analysis.md`, `spec.md`, `exploration.md`,
   structured sidecars and the final `status.json` transition. Current Path A
   phase functions write artifacts and state incrementally
   (`internal/workflow/workflow.go:88-105,155-200`), so prepare cannot simply
   call them sequentially and claim atomicity. Validation logic should be
   shared; publication needs its own transaction boundary.
3. Existing primitives are insufficient in a bounded way: individual
   `--manual` commands are one-artifact transitions; `cycle` continues through
   implement/apply/record and `--skip-execute` stops after implement; `next`
   emits one action and infers exploration from file presence
   (`internal/cli/phase2.go:26-145,409-466`). This is the required
   "could existing primitives do this?" pre-flight for the prepare PRD.
4. Slice 1 is advisory. Existing `defined` features are not invalidated, and
   `next`/`cycle` routing does not change merely because a document is
   classified thin or provenance is unknown. Any later routing change must be
   an explicit PRD behavior delta.
5. WP-004 is a guarded local draft, so its broken clean-checkout link was
   removed rather than staging unrelated work. The tracked whitepaper index is
   refreshed alongside this turn.

The `## Agreed` and §6.2 text now carry these constraints. WP-005 remains
`Exploring`; the next task is still the validation/provenance PRD, not
implementation.

**Asks of next agent**:
- Treat `unknown` provenance and routing compatibility as acceptance
  requirements, not open-ended prose.
- Include artifacts, sidecars and `status.json` in the transaction analysis.
- If the PRD chooses a persistent representation, write the ADR before asking
  reviewers to accept that choice for implementation.
