# ADR-037 - Reconcile Operation-Replay Candidate Authority

**Status**: Accepted rev-6
**Date**: 2026-09-02
**Owner**: Core
**Issue**: [GH #13](https://github.com/tesseracode/tesserapatch/issues/13) —
`reconcile: implement safe phase-2 operation replay candidate generation`
**Companion**:
[PRD-reconcile-operation-replay-candidate](../prds/PRD-reconcile-operation-replay-candidate.md)
rev-6 — **this ADR is normative where the two documents overlap**
**Depends on**:
[ADR-010](./ADR-010-provider-conflict-resolver.md),
[ADR-011](./ADR-011-feature-dependencies.md),
[ADR-017](./ADR-017-reconcile-lock-guard-and-writer-normalization.md),
[ADR-024](./ADR-024-patch-generation-manifest-boundary.md),
[ADR-025](./ADR-025-reconcile-evidence-and-revision-schema.md),
[ADR-028](./ADR-028-supersession-edge-model.md),
[ADR-029](./ADR-029-write-file-recipe-safety.md),
[ADR-030](./ADR-030-multi-slug-reconcile-derivation-mode.md),
[ADR-031](./ADR-031-rejected-feature-state-data-model.md) D4,
[ADR-033](./ADR-033-resource-capture-boundary.md),
[ADR-034](./ADR-034-rooted-filesystem-inspection-boundary.md),
[ADR-036](./ADR-036-recipe-coverage-authority.md) rev-7, and the accepted
confirmation-gate contract —
[PRD-reconcile-verdict-evidence](../prds/PRD-reconcile-verdict-evidence.md),
[PRD-upstreamed-confirmation-gate](../prds/PRD-upstreamed-confirmation-gate.md),
[PRD-confirm-upstreamed-human-review-path](../prds/PRD-confirm-upstreamed-human-review-path.md)
**Amends**:
1. **`SPEC.md:444-448` only** — and **all three** of its phase-2 rows.
   `→ All already-present → UPSTREAMED` (`:446`) becomes CG-qualified (D2);
   `→ Any conflict → BLOCKED` (`:447`) is a *phase classification*, not a
   forced overall return, so the arm stays **nonterminal** and the pipeline
   continues into phase 3/phase 4 (D3); and `→ Mix → STILL-NEEDED` (`:448`)
   is nonterminal at phase 2, surfacing a candidate **only** at a real phase-4
   textual conflict (D13). rev-2 **removes
   [ADR-010](./ADR-010-provider-conflict-resolver.md) from this row**: the
   sentences that change are `SPEC.md` §7's decision-tree prose, and no ADR-010
   decision is re-specified, weakened or renumbered by them.
2. **The accepted confirmation-gate contract**
   ([PRD-upstreamed-confirmation-gate](../prds/PRD-upstreamed-confirmation-gate.md) §3,
   [PRD-confirm-upstreamed-human-review-path](../prds/PRD-confirm-upstreamed-human-review-path.md))
   — the gate gains **one** new confirming authority class, defined by seven
   independently recomputed conditions CG1-CG7. Every existing confirming
   input, every rejection path and every weaker operation match is unchanged
   (D2).
3. **[ADR-025](./ADR-025-reconcile-evidence-and-revision-schema.md) D1/D3** —
   one reconcile invocation may append **multiple ordered evidence attempts**,
   split into **verdict-bearing** entries (at most one per (invocation, phase),
   each carrying its own **phase-local** `raw_reconcile_verdict`) and
   **supporting** entries (detector and gate observations, at most three per
   phase in the shipped pipeline, with their shipped phase, kind, verdict and
   **firing conditions** unchanged); `attempt_id` idempotency is the mechanism
   that makes the repeat safe (D25).
4. **[ADR-036](./ADR-036-recipe-coverage-authority.md) D15 producer P3** —
   `producer: reconcile-accept` gains **two variants** over the **same**
   closed `producer` enum value and the **same** `schema_version: 1`: the
   existing `resolver-accept` incomplete variant, and a new
   `operation-candidate-accept` complete variant (D30). **No enum value and
   no schema version change.**

**Preserves unchanged**:
[ADR-010](./ADR-010-provider-conflict-resolver.md) in full — its four-phase
order, its D2 shadow-worktree unit of atomicity and its provider-resolution
contract are depended on and preserved, never amended (S1);
[ADR-029](./ADR-029-write-file-recipe-safety.md) D3 all-or-nothing prechecks,
D4 legacy warning, D7 supersession severity;
[ADR-030](./ADR-030-multi-slug-reconcile-derivation-mode.md) D1/D2/D6/D7
multi-slug and `--cumulative-legacy` semantics;
[ADR-031](./ADR-031-rejected-feature-state-data-model.md) D4's exit-code
envelope;
[ADR-033](./ADR-033-resource-capture-boundary.md)'s resource-capture lock and
local contract, which this ADR **does not reuse and does not re-specify**;
[ADR-036](./ADR-036-recipe-coverage-authority.md)'s D3 canonical coverage
schema and strict decoder, D8's no-persisted-anchor scope reduction, and D9's
recompute-at-read rule.

**Blocked by**: GH #15 / ADR-036 implementation **shipped in v0.17.0**. No
GH #13 implementation slice may start before that release exists — the
consumer contract has nothing to consume until `artifacts/recipe-coverage.json`
is written by a real producer (`internal/` contains **no** coverage code
today).

**Target release**: v0.18.0, separate from GH #15's v0.17.0.

## Revision history

| Rev | Date | Change |
|---|---|---|
| rev-0 | 2026-09-01 | Initial proposal. Decisions D1-D34. Reviewed **NEEDS REVISION** (C1-C6, H1-H6, M1-M11 plus optional findings). |
| rev-1 | 2026-09-01 | D2 restated over the shipped confirmation gate's actual behavior and amended with the CG1-CG7 authority class (C1). D9 replaced: the byte-window context expansion is deleted and exact line-record alignment over **all** maximum-cardinality monotone alignments, with unanimity as the authority condition and closed resource limits, takes its place; D9.1 works the motivating fixture to a **success** (C2, C3, M7). D2 and D13 re-sequenced so a candidate is executed and persisted only at a real phase-4 textual conflict with `--resolve` unset (C4). D28 replaced HEAD equality with an ancestor check plus an authoritative per-path CAS (C5). D28's lock model replaced by a new portable kernel-advisory abstraction with no PID metadata and no stale-breaking (C6). D12b retires the legacy evaluator as authority behind a config gate (H1). D4 reduced from sixteen to fifteen achievable gates against ADR-036's strict decoder (H2). D12 given two sanctioned bare-root constructors and `GIT_NO_LAZY_FETCH=1` (H3, M11). D22 pinned at five new terminals, eight terminal actions and exact pair counts, with existing combinations never re-routed (H4). D20b makes candidate-ready `still_needed` bypass `updateFeatureState` (H5). D19 and D20 split immutable identity from mutable `state.json` (H6). D29 is one canonical fourteen-step transaction with step 10 as the first live write and tight exit codes; D29b defines the recovery preflight (M1, M2). D25 defines phase-local verdicts and honest attempt-ID migration (M3, M4). D13 scopes generation isolation to repository content (M5). D7 states the recipe-as-witness role and disposes of `ensure-directory` through ADR-036 completeness (M6, M8). D21 orders the staleness preflight (M9). D30 restates ADR-036 D15 P3 in full (M10). Decisions renumbered to **D1-D36** plus four sub-decisions D9.1, D12b, D20b and D29b (40 decision blocks in total). Reviewed **NEEDS REVISION** (B1-B6, S1-S3 plus ten optional findings); D9/D9.1's alignment mathematics was **approved** and is carried forward. |
| rev-2 | 2026-09-02 | **D4 re-sequenced so every reason code is reachable** (B1): rev-1's combined E4 asked "does the recipe decode **and** own the coverage?" ahead of the presence, hash and decodability gates, so it consumed all four recipe-side failures and reported `recipe-owner-mismatch` for each; rev-2 splits it into E4 readable presence, E5 raw-byte hashes, E6 strict decode and E7 decoded owner **conditional on E6**, with strictly first-match reporting. **D25 corrected against the shipped pipeline** (B2): four `phase-3.5` entries and up to two `phase-4` entries already coexist in one invocation, so the one-attempt-per-phase claim was false; D25 now distinguishes **verdict-bearing** from **supporting** entries and bounds only the former. **D2 gains a typed in-process carrier** (B5): the unexported `phase2AuthorityProof`, minted only by the GH #13 evaluator and passed as a fourth parameter to `applyUpstreamedConfirmationGate`, is what CG1-CG7 consume; it is never serialized and never reconstructed. **D28 and D29 gain a feature-state contract** (B3) requiring `status.State ∈ {applied, blocked}` at derivation, binding `source_feature_state` into D19's identity tuple and D20's schemas, and refusing `candidate-feature-state-changed` at acceptance. **D29 and D30 gain a base-metadata contract** (B4): step 12 sets `status.Apply.BaseCommit = upstream_commit` with a matching staged ADR-024 generation entry, and the six shipped `Apply.BaseCommit` readers are named. **D21 and D28 lock every candidate-state mutation** (B6), including the staleness preflight before phase 1. **S1**: ADR-010 removed from **Amends** and added to **Preserves unchanged**. **S2**: D28's runtime lock support narrowed to linux, darwin and windows; the BSDs move to the unsupported stub until native runtime CI verifies the primitive. **S3**: D28 gains its own known-network-filesystem refusal `lock-filesystem-unsupported` without importing the `rescap` local contract, and states that the CAS and journal — not the lock — are the authority. Optional findings closed: D3 re-attributes the ten-row consumer table to GH #15 PRD §6.14 and corrects the warning count to **seven**; D9 gains a fixed first-match rule order, traceback-free sentinel predicates and an L3 lowered to `1048576` cells / 8 MiB; D15 widens `candidate-not-idempotent`; D19/D23 declare accepted directories immutable audit with `candidate-already-accepted`; D22 becomes a source-derived allowlist with **85** refusing pairs; D28 pins `LockFileEx` to stdlib `syscall`; D3 scopes the fallthrough rule to the phase-2 vocabulary; and D30 quotes GH #15's P3-P7 ownership statement in full. One sub-decision is added — **D7b**, the permitted feature states and the bound `source_feature_state` — so the set is **D1-D36** plus D7b, D9.1, D12b, D20b and D29b (41 decision blocks). | Reviewed **NEEDS REVISION** (R1-R3 plus seven optional findings); D9/D9.1's alignment mathematics, D4's gate order, D28's lock model and D29/D30's acceptance and publication contracts were **not** challenged and are carried forward. |
| rev-3 | 2026-09-02 | **D2 names the entry CG5-CG7 grade, and pins its fields** (R1): the subject is the **new GH #13 phase-2 verdict-bearing `recipe-operation-match` attempt**, whose builder emits `confidence: high`, `match_origin: upstream`, `pre_reconcile_presence: present`, `upstream_commit_refs: [upstreamCommit]` and `upstream_commit == upstreamCommit` only after E1-E15 and the all-present classification both pass; the shipped legacy builder keeps `low` / `unknown` / `[]` (`internal/workflow/reconcile.go:826,828,829,843-845`) and is not modified. The carrier's route is closed: `saveReconcileArtifacts` (`:724`) takes a fourth `proof *phase2AuthorityProof`, all **ten** shipped call sites pass `nil`, and the gate cross-checks the proof against the **exact** verdict-bearing entry it is grading. *(rev-3 said **nine** sites and "about to write"; both corrected in rev-4.)* **D25 corrected against the shipped guards** (R2): the shipped maximum is one verdict-bearing entry plus **three** supporting entries at `phase-3.5` and one plus **one** at `phase-4`; the gate's `manual-review` entry (`:1033-1035`) is **mutually exclusive** with the blocked-only path-restructure (`:947-949`) and blocked-classification (`:998-1000`) entries, so rev-2's "four supporting `phase-3.5` entries" was wrong in every invocation shape. Detector firing conditions are unchanged and candidate-ready forces none of them. **D20b gains a pointer carry-forward contract** (R3): a shared status merge copies the loaded `Reconcile.OperationCandidate` into the new `ReconcileSummary` **before** the wholesale replacement at `:1191-1209`, on every non-candidate arm; only the five explicit candidate transitions may set, modify or clear the pointer; and the carry-forward is not a lifecycle-state change. **D26 restates diagnostic parity by semantic field**, not by detector entry count. **D28 and D29** re-validate the per-path CAS immediately before the first live write (same `candidate-cas-failed`), soften the "last word" wording to a precondition claim, and state that the advisory locks serialize `tpatch` on one host only — external and cross-host mutation after the recheck is not claimed prevented, and the journal and rollback are the recovery answer. **D29 step 8** names the snapshot set, including `artifacts/patch-generations.json` and `artifacts/recipe-provenance.json`. **D3** now names all three amended `SPEC.md` phase-2 bullets. **D22** excludes Cobra's auto-registered `help` flag by name, keeping the count at **85**. **D36** gives every parity block explicit `<!-- parity-block:<ID>:begin/end -->` delimiters over **nine** marked regions. No decision is added or removed: the set stays **D1-D36** plus D7b, D9.1, D12b, D20b and D29b (41 decision blocks). |
| rev-4 | 2026-09-02 | **D2's call-site inventory corrected from nine to TEN** (F1, blocking): `saveReconcileArtifacts` is called at `internal/workflow/reconcile.go:383,432,455,480,524,536,564,570,583,596`; rev-3 omitted `:570`, the phase-3.5 call whose third argument is spelled `phase35` rather than `result`. *(rev-4 called `phase35` "the distinct `*ReconcileResult` returned by `tryPhase35`"; rev-6 corrects that: `tryPhase35` takes `result` as its ninth parameter (`:1363`) and returns that same pointer from all ten of its returns, so `phase35` is a distinct variable name bound to the same object.)* Every nine-call claim is replaced with the ten-site list in parity block A (byte-identically with the PRD), in this table, in D34, in the implementation order and in the references, and `:570` is stated to pass `phase35` and, exactly like every existing and legacy call, a **`nil`** proof. **D2 defines how the new phase-2 verdict-bearing attempt reaches the evidence stream, and forbids emitting two** (opt. 1): the evaluator builds the attempt in the same statement that mints the proof and stages it in one new unexported `phase2Verdict *store.ReconcileEvidence` field on `ReconcileResult` *(superseded by rev-5's ordered `verdictAttempts` list)* (`:19-85`; `attemptedAt` `:74-84` is the shipped precedent, and `encoding/json` ignores unexported fields so `reconcile-session.json` (`:775-776`) is unaffected); `proof != nil` **iff** `result.phase2Verdict != nil`; and `persistReconcileEvidence` (`:766`, defined `:812`) appends the staged attempt **in place of** the entry it would have derived through `evidencePhaseAndKind` (`:1161-1176`). Exactly **one** verdict-bearing `phase-2` entry therefore exists on the authoritative all-present arm (`:451-457`), which D25 shows is the only arm where a collision is possible; the legacy builder is unmodified and runs verbatim everywhere else. **D2's cross-check names the entry it grades, not one it writes** (opt. 2): the gate grades `result.phase2Verdict`, already appended at `:766` before the gate runs at `:771`, and the only line the gate itself appends is its separate supporting `phase-3.5` / `manual-review` entry (`:1033-1035,1069-1070`), unchanged in phase, kind, fields and firing condition. **The positive and anti-duplicate cases are pinned** (opt. 3) by the companion PRD's ROC-290 and ROC-291 and by D2's source guard, extended to the staged attempt's single assignment site and single read site. No decision is added, removed or renumbered: the set stays **D1-D36** plus D7b, D9.1, D12b, D20b and D29b (41 decision blocks), and the nine parity regions are unchanged in identity and count. |
| rev-5 | 2026-09-02 | **D2's staging carrier becomes an ordered list, and candidate-ready becomes a staging arm** (B1, blocking): rev-4's singular `phase2Verdict *store.ReconcileEvidence` could carry one entry on one arm, so the candidate-ready outcome — whose `result.Phase` is `phase-2-operation-candidate` and therefore *also* takes `evidencePhaseAndKind`'s `phase-2` prefix branch (`internal/workflow/reconcile.go:1167-1168`) — would have had the legacy builder derive a **duplicate** `phase-2` verdict entry and would have had **no carrier** for the phase-4 textual-conflict verdict D25's own worked shape requires. D2 replaces the field with one unexported ordered `verdictAttempts []store.ReconcileEvidence` on `ReconcileResult` (`:19-85`), still ignored by `encoding/json` on the `attemptedAt` precedent (`:74-84`), and pins **three** legal configurations: (1) every other and legacy arm stages nothing and the shipped single derived builder runs unchanged; (2) the authoritative all-present arm stages **exactly one** qualifying phase-2 verdict-bearing `recipe-operation-match` attempt, built in the same statement that mints the proof; (3) the candidate-ready arm stages **exactly two** ordered attempts before save — `[1]` `phase-2` / `recipe-operation-match` / `still_needed` / `high` for the independently proved candidate and `[2]` `phase-4` / `forward-apply` / `blocked` / `low` for why ordinary patch replay could not win — and mints **no** proof. `persistReconcileEvidence` (`:766`, defined `:812`) emits the staged list **in place of** the entry it would have derived through `evidencePhaseAndKind` (`:1161-1176`), so `result.Phase` may stay `phase-2-operation-candidate` with no duplication. The biconditional is re-scoped to `proof != nil` **iff configuration 2**, so a proof is **never** paired with candidate-ready, and any illegal configuration is treated as no proof and no staging at all. D2's source guard is widened to **six** axes — single assignment site, single mutation site, single persist read site, no serialization, pairwise-distinct `attempt_id`s and strictly increasing phase order. **D25 names the two routes** a phase-2 finding takes: **staged substitution** on the two arms whose terminal `result.Phase` derives `phase-2`, and **direct append** on the fallthrough arms where no collision is possible; its per-arm table, class table and append-order statement follow. **D24** records that the candidate-ready pair's second attempt reuses the existing `phase-4` / `forward-apply` enum pair at the shipped `:846-849` confidence rung, so the no-new-enum claim covers both staged attempts. **D34** and the implementation order carry the renamed ordered field. **D36** now states the marker-parser mechanism, not just the conclusion: a marker is recognized only when its ID matches the closed nine-value set, so neither this ADR's `<ID>` illustration nor the companion PRD §6.0's is read as an unpaired `begin`. Parity block `A` changes identically on both sides. No decision is added, removed or renumbered: the set stays **D1-D36** plus D7b, D9.1, D12b, D20b and D29b (41 decision blocks), and the nine parity regions are unchanged in identity and count. |
| rev-6 | 2026-09-02 | **D2's `:570` argument claim corrected: `phase35` is a distinct variable *name*, not a distinct object** (blocking): rev-4 and rev-5 said `internal/workflow/reconcile.go:570` passes "the distinct `*ReconcileResult` returned by `tryPhase35`". The shipped source says otherwise — `tryPhase35` (`:569`, defined `:1354-1364`) takes the pipeline's `result` as its **ninth parameter** (`:1363`), mutates it in place and returns **that same pointer** from every one of its ten returns, each literally `return result` (`:1373,1381,1389,1436,1475,1491,1497,1502,1508,1513`). `phase35` and `result` name **one object**; `:570` differs from the other nine call sites in argument spelling only and, like every existing and legacy call, stages no attempt and passes a **`nil`** proof. The corrected statement replaces the old one in parity block A (byte-identically with the companion PRD), in this table's rev-4 row, in D34, in the implementation order and in the references. The ten-site inventory, the proof route, D2's three staging configurations and the CG1-CG7 class are untouched — only the *reason* `:570` was singled out changes. **D25 gains the dual-sink invariant**: every verdict-bearing attempt reaches `reconcile-evidence.jsonl` (via `store.AppendReconcileEvidence`, `:876`) **and** the in-memory `result.Evidence` slice in the same relative order, on **both** routes — staged substitution inherits it from the shipped `:766` / `:880` shape, and the direct-append route must do it explicitly — so `reconcile-session.json`'s serialized `evidence` array (`:775-776`) is never a strict subset of the JSONL; the companion PRD's ROC-208 pins it at runtime. The companion PRD additionally scopes ROC-290's and ROC-281's S0 byte-identity claims to D25's **gate-off** per-arm row, since the gate-on ineligible fallthrough takes the direct-append route and carries an intentional phase-2 evidence delta while still emitting one verdict-bearing entry per phase. No decision is added, removed or renumbered: the set stays **D1-D36** plus D7b, D9.1, D12b, D20b and D29b (41 decision blocks), and the nine parity regions are unchanged in identity and count. |

## Context

### What phase 2 does today

`reconcileFeature` runs a five-slot pipeline
(`internal/workflow/reconcile.go:315`): a viability short-circuit
(`:358`), phase 1 reverse-apply (`:377-386`), a flag-gated phase 1.5 patch-id
sweep (`:388-443`), phase 2 operation-level evaluation (`:445-464`), phase 3
provider-semantic (`:466-488`), and phase 4 forward-apply with a nested
phase 3.5 resolver (`:490-586`, resolver dispatch at `:569`).

Phase 2's entire implementation is `evaluateRecipeOperations`
(`internal/workflow/reconcile.go:611`). It has five structural problems that
this ADR exists to fix.

1. **It inspects the live worktree.** It is called as
   `evaluateRecipeOperations(s.Root, recipe.Operations)`
   (`internal/workflow/reconcile.go:450`) and reads
   `filepath.Join(repoRoot, op.Path)` with `os.ReadFile`
   (`internal/workflow/reconcile.go:617-618,635-636`). There is no
   `git show` / `git cat-file` read anywhere in the phase. Whether the local
   checkout happens to be dirty, mid-rebase or on an unrelated branch changes
   the verdict.
2. **Its matching is substring and whitespace-lossy.** `replace-in-file`
   presence is `strings.Contains(contentStr, op.Replace)`
   (`internal/workflow/reconcile.go:625`); `write-file` presence is
   `strings.TrimSpace(string(content)) == strings.TrimSpace(op.Content)`
   (`internal/workflow/reconcile.go:637`). A first-match substring is not
   an anchor and trimmed equality is not byte equality.
3. **Its outcome vocabulary does not exist.** `SPEC.md:444-448` promises
   `UPSTREAMED` / `BLOCKED` / `STILL-NEEDED`, but the code produces only
   counts and one boolean:
   `result.allPresent = presentCount > 0 && conflictCount == 0 && applicableCount == 0`
   (`internal/workflow/reconcile.go:654`). The `allPresent` arm returns
   `store.ReconcileUpstreamed` (`internal/workflow/reconcile.go:451-457`); the
   conflict arm appends an advisory note and falls through
   (`internal/workflow/reconcile.go:459-462`). So `BLOCKED` is already
   nonterminal in practice and the SPEC text is the thing that is wrong.
4. **Its one terminal arm is not actually terminal.** This is the fact rev-0
   of this ADR got wrong, and it is load-bearing.
   `applyUpstreamedConfirmationGate` (`internal/workflow/reconcile.go:1032`,
   called at `:771`) runs on every `store.ReconcileUpstreamed` result and
   confirms on exactly two inputs: a `patch-id-match` entry carrying a
   non-empty `matched_upstream_sha` (`:1047-1054`), and a `reverse-apply`
   entry at `confidence: high` (`:1055-1060`). Phase 2 can only produce
   `recipe-operation-match` (`:1167-1168`), which confirms nothing, so the
   unconfirmed branch overwrites the outcome with `store.ReconcileBlocked` and
   sets `rejected-upstreamed` (`:1089-1090`), and `persistRevisionPassLog`
   records a `blocked` final state (`:1096-1112`). This is pinned by
   `TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch`
   (`internal/workflow/reconcile_evidence_integration_test.go:459`) over
   `buildOperationUpstreamedCandidateFixture` (`:235`).
5. **It produces no reviewable artifact.** A mixed result — some effects
   already upstream, the rest cleanly insertable — is exactly the case an
   operator wants a diff for, and phase 2 emits nothing but a count line.

The confirmation gate in (4) is **correct** and its motivation is on record:
two real features were lost to false `upstreamed` verdicts
(`docs/prds/PRD-upstreamed-confirmation-gate.md:61-66`). The gate is not an
obstacle to route around; what is missing is a deterministic proof strong
enough to satisfy it, which its own goal 4 explicitly anticipates
(`docs/prds/PRD-upstreamed-confirmation-gate.md:79-80`). D2 supplies exactly
one.

### What GH #15 hands it

ADR-036 rev-7 defines `artifacts/recipe-coverage.json`: a strict, deterministic
sidecar binding the canonical patch and the recipe to a named reference commit,
with orthogonal per-effect axes, per-side observation flags, exact hashes and a
ten-predicate `coverage_status`
(`docs/adrs/ADR-036-recipe-coverage-authority.md:464-543,1038-1059`). Its
decoder already refuses every internally contradictory record
(`:1065-1076`). It deliberately hands GH #13 **no anchor** (D8,
`docs/adrs/ADR-036-recipe-coverage-authority.md:1518-1541`) and states in terms
that completeness is not replay eligibility (D4,
`docs/adrs/ADR-036-recipe-coverage-authority.md:1227-1268`).

That is the correct division. GH #15 proves *how a recipe explains a patch at
its own base*. GH #13 must prove, independently, *whether those same effects
can be reproduced at a different upstream tree* — and it must do so without
promoting any producer claim into authority.

### The case this feature exists for

`docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md`
records a feature that added two arguments to a Go CLI slice while upstream
intentionally deleted the neighbouring arguments. Merge and rebase both
conflict; the semantically correct answer is obvious to a human and stated in
the case study. rev-0's byte-window algorithm could not derive it, which is why
D9 is replaced rather than patched. D9.1 works the fixture end to end.

### What this ADR is not

This is **contract fidelity for the existing phase 2**. It is not a new
command, not a phase 2.5, not a new lifecycle state, and not a general
patch-commutation engine. The four-phase order and the provider/forward-apply
fallback are preserved exactly; every refusal in this document falls **through**
to them.

## Decision

**Six blocks are reproduced byte-identically in the companion PRD** and are
covered by a planning parity check at implementation time: the CG1-CG7
confirmation conditions with the `phase2AuthorityProof` carrier (D2 / PRD
§6.2), the E1-E15 eligibility gate in its reachable order (D4 / §6.5), the
A1-A8 alignment rules with their first-match evaluation order and the L1-L3
limits (D9 / §6.7), the `candidate.json` and `state.json` schemas (D20 /
§6.10), the fourteen-step acceptance transaction (D29 / §6.14), and the
ADR-036 D15 P3 two-variant restatement (D30 / §6.16).

**rev-3 makes the blocks machine-extractable** (D36). Each is delimited in
both documents by `<!-- parity-block:<ID>:begin -->` and
`<!-- parity-block:<ID>:end -->`, and the check compares **only** the bytes
between a matching pair. The six blocks occupy **nine** marked regions — `A`,
`B`, `C1`, `C2`, `D1`, `D2`, `D3`, `E`, `F` — because block C's rules and
limits, and block D's two schemas and enum paragraph, are separated in both
documents by prose that is deliberately not identical.

### D1 - Phase 2 evaluates one resolved upstream commit, never the live worktree

Phase 2 resolves the upstream ref to a single 40-hex commit once per feature
(`gitutil.ResolveRef`, `internal/gitutil/gitutil.go:789`) and performs **every**
eligibility, alignment and classification read against that commit's tree,
through content reads that never touch the checkout
(`gitutil.FileAtCommit`, `internal/gitutil/gitutil.go:803`;
`gitutil.BlobAtTree`, `internal/gitutil/trailers.go:737`).

`s.Root` is read for exactly two things and neither is eligibility:
the `.tpatch/` store artifacts (which are not repository content), and — only
at explicit acceptance (D28) — the compare-and-swap preconditions on the live
paths about to be written.

This retires `internal/workflow/reconcile.go:450`'s `s.Root` argument for
candidate purposes (D12b).

**Rationale.** A verdict computed against an arbitrary working tree is not
reproducible and cannot be re-proved at accept time. Everything downstream —
the candidate ID, the idempotency proof, the accept-time CAS — depends on the
inputs being two named immutable trees.

**Alternatives rejected.** Reading the index instead of the worktree: still not
durable, and ADR-036 D2 already classified `--unstaged` as commit-kind HEAD
rather than minting index authority. Requiring a clean tree: `--allow-dirty`
exists (`internal/cli/cobra.go:2523`) and a flag an operator can set must not
be load-bearing for correctness.

### D2 - Phase 2's outcomes, and one new confirmation authority class

Phase 2 classifies each ADR-036 normalized effect as **present**,
**applicable** or **conflicting** at the resolved upstream commit (D10), then
produces one of four results. **Only the all-present result returns from
phase 2.**

| Phase-2 result | Phase 2 returns | Artifact |
|---|---|---|
| every effect **present**, CG1-CG7 all hold | `store.ReconcileUpstreamed`, **confirmed** | richer evidence only |
| every effect **present**, any of CG1-CG7 fails | `store.ReconcileUpstreamed`, which the shipped gate demotes to `store.ReconcileBlocked` / `rejected-upstreamed` | richer evidence only |
| every effect present or applicable, **at least one applicable**, zero conflicting | **nothing** — an in-memory candidate proof is held for D13 | none yet |
| **any** effect ineligible, unsafe or conflicting | **nothing** — a phase-2 `BLOCKED` classification | nonterminal evidence, pipeline continues |

**The all-present arm is not terminal today.** The Context section states the
source truth: the confirmation gate confirms only on `patch-id-match` with a
`matched_upstream_sha` (`internal/workflow/reconcile.go:1047-1054`) or
high-confidence `reverse-apply` (`:1055-1060`), and demotes everything else
(`:1089-1090`).

<!-- parity-block:A:begin -->
**The gate gains exactly one new confirming authority class, and this PRD and
ADR-037 D2 state it identically.** The gate confirms a `recipe-operation-match`
entry **if and only if all seven** of the following hold, and it learns all
seven from **one typed in-process value**, never from a stored field. This is
**parity block A**.

**The graded entry is named, and it is new.** CG5-CG7 are conditions on
**one** entry: the **new GH #13 phase-2 verdict-bearing
`recipe-operation-match` attempt** minted in this invocation by the GH #13
evaluator (D25 / §6.17). They are not conditions on a legacy
`recipe-operation-match` line, on an entry replayed from an earlier run, or on
anything the shipped builder emits. The new builder emits exactly these values,
and emits them **only** after E1-E15 have all passed and the classification is
all-present:

| Field | New GH #13 phase-2 verdict-bearing entry | Shipped legacy builder (`internal/workflow/reconcile.go:843-845`) |
|---|---|---|
| `schema_version` | `store.ReconcileEvidenceSchemaVersion` — `1`, the shipped constant (`:852`) | the same constant (`:852`) |
| `feature_slug` | the reconciled `slug` (`:853`), which `store.AppendReconcileEvidence` re-stamps and the validator re-checks (`internal/store/reconcile_evidence.go:144,347-348`) | the same slug (`:853`) |
| `attempt_id` | `store.ComputeAttemptID` over the finished entry (`:875`), never hand-written | the same call (`:875`) |
| `upstream_ref` | `result.UpstreamRef` (`:854`) — the resolved ref of this invocation, non-empty | the same field (`:854`) |
| `phase` | `phase-2` | `phase-2` |
| `evidence_kind` | `recipe-operation-match` | `recipe-operation-match` |
| `confidence` | `high` | `low` (`:844`) |
| `match_origin` | `upstream` | `unknown` — the `:826` default, never assigned by this arm |
| `pre_reconcile_presence` | `present` | `present` (`:845`) |
| `upstream_commit_refs` | `[upstreamCommit]`, exactly one element | `[]` — the `:829` empty initializer, never assigned by this arm |
| `upstream_commit` | `upstreamCommit`, the single resolved 40-hex commit — identical to `result.UpstreamCommit` (`:855`), because §6.1 resolves it once | `result.UpstreamCommit`, unchanged |
| `base_commit` | the loaded `status.Apply.BaseCommit` with the shipped `"unknown"` fallback (`:820-823`, written at `:856`) — the accepted base this feature's patch and effects are recorded against, which for an eligible feature is the commit ADR-036's `reference.commit` names and E8 reconstructs. It is **never** `upstream_commit` here: the base becomes the upstream commit only at acceptance step 12 (§6.14). No new reader, writer or gate is introduced for it | the same loaded value (`:820-823,856`) |
| `requires_confirmation` | `false` | `true` (`:828`) |
| `raw_reconcile_verdict` | `upstreamed` | unchanged |
| `reason_code` | `phase-2-operation-level` — the shipped `result.Phase` at `:453`, which `:867` would also have written | `result.Phase`, unchanged (`:867`) |
| `matched_paths` | `result.Conflicts`, exactly as `:861` — empty on this arm, because an all-present classification has no conflict | the same field (`:861`) |
| `matched_operations` | `[]`, exactly as `:862` | the same empty slice (`:862`) |

**Every required field is populated, so the staged entry satisfies the shipped
store validator unmodified.** `validateReconcileEvidence`
(`internal/store/reconcile_evidence.go:343-357`) refuses an empty
`upstream_ref`, `upstream_commit`, `base_commit`, `raw_reconcile_verdict` or
`reason_code`, a `schema_version` other than `1`, a `feature_slug` that is not
the reconciled slug, an empty `attempt_id`, and a nil `matched_paths`,
`matched_operations` or `upstream_commit_refs`. The staged attempts populate
all seventeen required fields and add no field beyond them, so no schema,
decoder or validator change is needed. `attempt_id` remains content-addressed over the
whole normalized entry (`:875`, `internal/store/reconcile_evidence.go:120`) —
pinning the base fields changes **evidence** attempt identity only in the sense
ADR-025 D3 already defines, and changes **candidate** identity not at all:
§6.10 / D19's candidate ID is a separate immutable tuple and is untouched here.

**The legacy builder is not modified.** Any run that does not reach the GH #13
all-present result produces byte-identical evidence to today's, and the
`unknown` / `[]` / `low` triple is exactly why such an entry can never satisfy
CG5-CG7. These fields are an **output** of the proof, never a substitute for
it: an implementation that widens the legacy builder to emit them, rather than
adding the new entry behind the full recomputation, has satisfied nothing.

**The GH #13 evaluator stages an ordered list of verdict-bearing attempts, and
whatever it stages replaces the single derived entry.** Both authoritative arms
substitute; **no arm ever emits a staged attempt and a derived entry in one
invocation**. The carrier is one new **unexported** field on `ReconcileResult`
(`internal/workflow/reconcile.go:19-85`), `verdictAttempts
[]store.ReconcileEvidence`. `attemptedAt` (`:74-84`) is the shipped precedent
for an unexported staging field on that struct, and `encoding/json` ignores
unexported fields, so the whole-struct serialization into
`reconcile-session.json` (`:775-776`) is byte-unaffected by the staging. There
are exactly **three** legal configurations, and every other one is a defect:

| # | Arm | `len(result.verdictAttempts)` | `proof` |
|---|---|---|---|
| 1 | every other arm — the legacy all-present arm (D12b / §6.4), the phase-2 conflict arm, every fallthrough, every phase-1, phase-1.5, phase-3, phase-3.5 and phase-4 outcome, and every invocation with `operation_candidate_enabled` off | `0` | `nil` |
| 2 | the authoritative all-present arm | `1` — the **qualifying** phase-2 entry of the field table above | **non-nil** |
| 3 | the candidate-ready arm | `2` — ordered, phase-2 then phase-4 | `nil` |

- **Substitution, never addition.** When `len(result.verdictAttempts) > 0`,
  `persistReconcileEvidence` (`:766`, defined `:812`) appends the staged list
  **in slice order, in place of** the one entry it would have derived from
  `result.Phase` through `evidencePhaseAndKind` (`:1161-1176`) — at the shipped
  `:766` position and ahead of every supporting entry. The derived entry is
  neither built nor appended. When the list is empty the shipped single
  derivation runs verbatim (`:816-880`) and its bytes are unchanged.
- **Configuration 2 stages exactly one attempt** — the phase-2 verdict-bearing
  `recipe-operation-match` entry of the field table above, every one of whose
  seventeen fields that table pins, `raw_reconcile_verdict: upstreamed` and
  `reason_code: phase-2-operation-level` (the shipped `result.Phase` at `:453`,
  which the shipped builder would also have written through `:867`) included.
  It is built in the **same statement** that mints the proof.
- **Configuration 3 stages exactly two ordered attempts** (table below) before
  `saveReconcileArtifacts` is called, and mints **no proof**. Because it
  substitutes, `result.Phase` may remain `phase-2-operation-candidate` — which
  `evidencePhaseAndKind` maps to `phase-2` / `recipe-operation-match`
  (`:1167-1168`) — **without** producing a duplicate phase-2 line, because the
  derived entry is suppressed.

**Configuration 3's two attempts, in slice order.** Attempt [1] is the
independently proved candidate; attempt [2] is why ordinary patch replay could
not win — the verdict this invocation would have returned had no candidate been
finalized:

| Field | [1] phase-2 candidate attempt | [2] phase-4 textual-conflict attempt |
|---|---|---|
| `schema_version` | `1`, the shipped constant (`:852`) | `1`, the same constant |
| `feature_slug` | the reconciled `slug` (`:853`) | the same slug |
| `attempt_id` | `store.ComputeAttemptID` over the finished entry (`:875`) | the same call, over different bytes, so the two IDs are **distinct** |
| `upstream_ref` | `result.UpstreamRef` (`:854`) | the same ref |
| `phase` | `phase-2` | `phase-4` |
| `evidence_kind` | `recipe-operation-match` | `forward-apply` |
| `raw_reconcile_verdict` | `still_needed` | `blocked` |
| `confidence` | `high` — the finalized-and-validated rung of §6.17's ladder, staged only after §6.8's execution, validation and idempotency proofs all pass | `low` — the shipped `forward-apply`-plus-blocked rung (`:846-849`) |
| `match_origin` | `upstream` — every present effect was recomputed at `upstreamCommit` | `unknown`, the shipped `:826` default for this kind |
| `pre_reconcile_presence` | `present` | `not-checked`, the shipped `:827` default for this kind |
| `upstream_commit_refs` | `[upstreamCommit]`, exactly one element | `[]`, the shipped `:829` initializer |
| `upstream_commit` | `upstreamCommit` | `upstreamCommit` |
| `base_commit` | the loaded `status.Apply.BaseCommit` with the shipped `"unknown"` fallback (`:820-823,856`) — the accepted base the effects are recorded against, never `upstream_commit` | the **same** loaded value; both attempts of one invocation share one base |
| `requires_confirmation` | `true` — no proof was minted, so nothing here is confirmable | `true`, the shipped `:828` default |
| `reason_code` | `phase-2-operation-candidate` | `phase-4-forward-apply-conflicts` on the conflicts arm (`:575`), `phase-4-blocked` on the exhausted arm (`:590`) |
| `matched_paths` | the slash-normalized effect paths | `result.Conflicts`, exactly as `:861` |
| `matched_operations` | the derived operation identifiers | `[]`, exactly as `:862` |

Both attempts populate the same seventeen required fields as the all-present
entry above and satisfy the same validator (`:343-357`); the pair differs only
in the values this table pins, which is also why their content-addressed
`attempt_id`s differ.

Attempt [1] carries `match_origin: upstream` and `pre_reconcile_presence:
present` and is still **never confirmable**: the gate runs only on an
`upstreamed` outcome (`:1033-1035`) and this invocation returns
`store.ReconcileStillNeeded`, and CG1 fails regardless because no proof exists.
Confirmation authority lives in the proof, never in a field value.

**The proof pairs with the one qualifying entry, and with nothing else.**
`proof != nil` **if and only if** configuration 2 holds — that is, if and only
if `result.verdictAttempts` is a one-element list whose single member is the
qualifying phase-2 attempt, matching the field table above on `phase`,
`evidence_kind`, `confidence`, `match_origin`, `pre_reconcile_presence`,
`upstream_commit_refs`, `upstream_commit`, `requires_confirmation` and
`raw_reconcile_verdict`. A proof beside an empty list, beside configuration 3's
two-element list, or beside a one-element list that is not the qualifying
entry is a defect; so is the qualifying entry with no proof. The nine field
values named here are the discriminators; the base fields of the field table —
`schema_version`, `feature_slug`, `attempt_id`, `upstream_ref` and
`base_commit` — carry the shipped values on **every** arm and therefore
distinguish nothing, which is why they are pinned but not compared. **The
candidate-ready arm is never paired with a proof** — it stages two attempts and
mints none, and that is configuration 3, not a broken configuration 2.
`saveReconcileArtifacts` treats **any** illegal configuration as **no proof and
no staging at all**: it discards the staged list, the shipped single derivation
runs, and the gate is handed `nil`.

**Why the attempts travel on the result while the proof does not.** Each
attempt is a `store.ReconcileEvidence` whose destination is `result.Evidence`;
staging them on that same result makes it structurally impossible to pair a
proof minted for one result with an entry built for another, which a second
out-of-band parameter would permit. The **proof** keeps its parameter-only
route and is still never a field of `ReconcileResult`, never serialized and
never rebuilt.

| # | Confirmation condition |
|---|---|
| CG1 | the gate was handed a non-nil `phase2AuthorityProof` minted by this feature's GH #13 phase-2 evaluator **in this invocation** — not read back from a prior run, not rebuilt from an evidence line, and not produced by the legacy evaluator (D12b / §6.4) |
| CG2 | the proof records that every ADR-036 binding passed the E1-E15 gate (D4 / §6.5) with every value **independently recomputed**, never read |
| CG3 | the proof records that every normalized effect of the canonical patch was independently recomputed **present** at the named upstream commit under the D10 / §6.7 classification |
| CG4 | the proof's derivation counts are **zero** candidate operations, **zero** applicable classifications and **zero** conflicting classifications |
| CG5 | the entry being graded is the **single** staged entry of `result.verdictAttempts` — the new GH #13 phase-2 verdict-bearing entry above — and its `match_origin == upstream` (`internal/store/reconcile_evidence.go:63`) |
| CG6 | that same entry's `pre_reconcile_presence == present` (`internal/store/reconcile_evidence.go:73`) |
| CG7 | that same entry's `upstream_commit_refs` is exactly `[upstreamCommit]` — one element, no more — and both the proof's `upstream_commit` and the entry's `upstream_commit` equal that resolved commit |

**The carrier is a typed, unexported, in-process value, and its route is
fixed.** `applyUpstreamedConfirmationGate`
(`internal/workflow/reconcile.go:1032`) is **not** called from the pipeline
directly: it is called from inside `saveReconcileArtifacts` (`:724`, call at
`:771`). The proof therefore reaches it through that function and through no
other channel — not through `ReconcileResult`, not through a package variable,
not through a context value. The staged `verdictAttempts` list above is a
different object with a different route, carries no proof field, and does not
change this: the proof is a parameter and nothing else. Both signatures change:

```go
// Unexported: constructible only inside the GH #13 phase-2 evaluator.
type phase2AuthorityProof struct {
    upstreamCommit   string // 40-hex, the commit E1-E15 and the classifier used
    referenceCommit  string // 40-hex, ADR-036 reference.commit
    coverageSHA256   string // recomputed, not read
    patchSHA256      string // recomputed, not read
    recipeSHA256     string // recomputed over raw on-disk bytes
    sourceFeatureState string // the permitted state bound at derivation (D7b / §6.6a)
    gatesPassed      [15]bool // E1-E15, all true
    effectCount      int
    effectDigest     string // SHA-256 over the canonical JSON of the ordered effect-map proof hashes
    presentCount     int
    applicableCount  int
    conflictCount    int
}

// New unexported field on ReconcileResult (internal/workflow/reconcile.go:19-85),
// staged only by the GH #13 evaluator: one qualifying phase-2 attempt on the
// all-present arm, minted in the same statement as the proof; two ordered
// attempts, phase-2 then phase-4, on the candidate-ready arm; empty everywhere
// else. Unexported, so encoding/json ignores it exactly as it ignores
// attemptedAt (:74-84) and reconcile-session.json (:775-776) is unaffected.
verdictAttempts []store.ReconcileEvidence

// Fourth parameter added. All ten shipped call sites pass nil; only the GH #13
// all-present path passes a minted proof, and only paired with the single
// qualifying entry in result.verdictAttempts.
func saveReconcileArtifacts(
    s *store.Store, slug string, result *ReconcileResult, proof *phase2AuthorityProof,
)

func applyUpstreamedConfirmationGate(
    s *store.Store, slug string, result *ReconcileResult, proof *phase2AuthorityProof,
) []store.ReconcileEvidence
```

Its rules are absolute:

- it is **minted in exactly one place** — the GH #13 evaluator, after E1-E15
  all pass and the classification is all-present — and the type is unexported
  with no exported constructor, no setter and no zero-value path to a
  confirming state. The one qualifying verdict-bearing attempt above is built
  and staged in that **same statement**, so proof and entry are minted together
  or not at all;
- it is **threaded, never stored**. The evaluator hands it to
  `saveReconcileArtifacts`, which passes it to the gate as its fourth argument
  in the same invocation. All **ten** shipped call sites
  (`internal/workflow/reconcile.go:383,432,455,480,524,536,564,570,583,596`)
  pass `nil`, so every non-GH-#13 path is unchanged — including `:570`, whose
  third argument is **named** `phase35` but is the **same `*ReconcileResult`
  object** as the pipeline's `result`, not a second one: `tryPhase35` (`:569`,
  defined `:1354-1364`) takes that `result` as its ninth parameter (`:1363`),
  mutates it in place and returns **that same pointer** from every one of its
  ten returns — each literally `return result`
  (`:1373,1381,1389,1436,1475,1491,1497,1502,1508,1513`) — so `phase35` is a
  distinct **variable name**, not a distinct object, and `:570` differs from
  the other nine call sites in argument spelling only. Exactly like every
  existing and legacy call it stages no attempt and passes a `nil` proof. It is
  **never serialized**,
  never written to `candidate.json`, `state.json`, `status.json`,
  `reconcile-session.json` or `reconcile-evidence.jsonl`, never read back from
  any of them, and never reconstructed from `confidence`, `evidence_kind` or
  any other stored field;
- the gate **recomputes and cross-checks** the proof against the **exact
  verdict-bearing entry it is grading** — the single member of
  `result.verdictAttempts`, which `persistReconcileEvidence` already appended
  to `result.Evidence` at `:766` before the gate runs at `:771` — and against
  the current `result`: that
  entry's `phase`, `evidence_kind`, `confidence`, `match_origin`,
  `pre_reconcile_presence`, `upstream_commit` and `upstream_commit_refs`,
  together with the proof's two commits, three binding hashes, effect count,
  effect digest and three classification counts, must all agree. Any
  disagreement fails CG1-CG7 and takes the shipped unconfirmed branch. The gate
  never grades one entry against a proof minted for another. **The gate does
  not write the entry it grades**: the only line the gate itself appends is its
  own separate **supporting** `phase-3.5` / `manual-review` entry
  (`:1033-1035,1069-1070`), whose shipped phase, kind, fields and firing
  condition are unchanged (D25 / §6.17);
- it is **`nil`** for every legacy caller, every prior-run entry and every
  invocation where the GH #13 evaluator did not run to an all-present result.
  A `nil` proof confirms nothing;
- a source guard pins that no symbol outside the evaluator constructs, copies
  into or mutates the type; that it carries no struct tag, no exported field
  and no marshaller; and that no function other than the two above accepts or
  returns it. A fabricated or smuggled proof is therefore a compile-time or
  guard-time failure rather than a review finding. The **same** guard pins the
  staged list, on **six** axes: `verdictAttempts` has exactly **one**
  assignment site, inside the GH #13 evaluator; exactly **one** mutation site,
  in that same evaluator, with no append, re-slice or element write anywhere
  else; exactly **one** read site, `persistReconcileEvidence`'s substitution;
  it is **never serialized** — unexported, no struct tag, no exported alias, no
  marshaller, and absent from every artifact; the staged entries' `attempt_id`
  values are **pairwise distinct**, which `store.ComputeAttemptID` (`:875`)
  gives for free because the two candidate-ready payloads differ; and the list
  is in **strictly increasing phase order** over ADR-025 D4's phase sequence,
  so configuration 3 is `phase-2` then `phase-4` and never the reverse. No
  production path appends a staged attempt and a derived entry in one
  invocation.

`confidence: high` is a **consequence** of CG1-CG7, not an input to them. An
implementation that confirms on confidence alone, on `evidence_kind` alone, on
any stored flag, or on a proof it rebuilt from persisted bytes has not
satisfied this decision.
<!-- parity-block:A:end -->

**Everything weaker keeps today's behavior.** A legacy operation match, a
replayed entry, an entry from a run whose gate refused, or any entry failing
one of CG1-CG7 still requires human confirmation and still demotes. The gate's
two existing confirming inputs, its rejection arm, its evidence record and
`persistRevisionPassLog` (`internal/workflow/reconcile.go:1096`) are unchanged.

**Rationale.** The gate exists because a false `upstreamed` silently drops
local feature code, and two real features were lost that way
(`docs/prds/PRD-upstreamed-confirmation-gate.md:61-66`). CG1-CG7 do not weaken
it: they replace "a human looked at it" with "a machine re-derived every byte
from two named immutable trees in this process and found nothing left to do",
which is the deterministic fast path that contract's own goal 4 anticipates
(`:79-80`). Without this amendment, GH #13's strongest possible result would
still print `blocked`, and the feature would have no all-present story at all.

**Alternatives rejected.** Bypassing the gate for `recipe-operation-match`:
that is the exact regression the gate was built to prevent. Confirming on
`confidence: high`: confidence is a self-assessment and any future evaluator
could set it. Adding a new `evidence_kind`: it would break ADR-025 D4's closed
enum and every existing aggregator for no gain (D24). **Widening the shipped
`recipe-operation-match` builder to emit `upstream` / `present` /
`[upstreamCommit]` instead of adding a new entry** (considered at rev-3): it
would make every legacy operation match look confirmable, invert the direction
of the proof — fields would become the input rather than the output — and
change shipped evidence bytes for runs that prove nothing. **Carrying the proof
on `ReconcileResult`**: that struct is serialized to `reconcile-session.json`
(`internal/workflow/reconcile.go:775-776`), so the authority would become a
persisted field the next run could read back, which D33's ninth prohibition
forbids outright. **Threading it through a package-level variable or a
context**: both are ambient state that a second goroutine, a test helper or a
future caller can set, and neither is checkable by a source guard the way an
explicit parameter is.

### D3 - Phase-2 `BLOCKED` is a classification, not a return; refusal always falls through

This decision **explicitly amends** the `SPEC.md:444-448` decision tree, whose
phase-2 row reads `→ Any conflict → BLOCKED` and is naturally read as an
overall verdict. It is not. Phase-2 `BLOCKED` records a phase classification
and its evidence, and **returns nothing**; control continues to phase 3 and
phase 4 exactly as `internal/workflow/reconcile.go:459-464` already does.

**The amendment covers all three phase-2 rows, not just the conflict one.**
rev-2 named only `BLOCKED`, which understated the edit S9 has to make: this
feature changes what all three SPEC outcomes mean, and the SPEC has to say so
in one pass or it stays wrong in two places.

| SPEC phase-2 row | Amended reading | Where |
|---|---|---|
| `→ All already-present → UPSTREAMED` (`SPEC.md:446`) | **CG-qualified.** All-present returns `upstreamed` only when the confirmation gate's CG1-CG7 class holds; unconfirmed all-present keeps the shipped demotion to `blocked` / `rejected-upstreamed` | D2 |
| `→ Any conflict → BLOCKED` (`:447`) | **A phase classification, not an overall return** — the pipeline continues into phase 3 and phase 4 | D3 |
| `→ Mix → STILL-NEEDED` (`:448`) | **Nonterminal at phase 2.** A mixed result holds an in-memory proof and returns nothing; it surfaces as candidate-ready `still_needed` **only** at a real phase-4 textual conflict with `--resolve` unset, and any stronger later outcome wins | D13 |

Two consequences are load-bearing:

- **Later evidence is preserved, never overwritten.** A phase-2 blocked
  classification followed by a phase-4 clean forward apply yields *both*
  entries and an overall `reapplied`. Phase 2 may not shadow a better answer
  a later phase can prove (D25).
- **Every phase-2 refusal in this ADR is a fallthrough, not a failure.**
  Missing, malformed, incomplete or stale-marked coverage; an unsupported
  effect; an ambiguous alignment; a multi-slug invocation; an unsupported or
  contended lock platform — each records phase-2 ineligibility and continues.
  `tpatch reconcile` exits with the code its final phase earned, not one this
  phase invented. **The rule is scoped to the phase-2 vocabulary** (companion
  PRD §7.1-§7.3): a terminal candidate action, an acceptance failure and a
  recovery refusal (§7.4) are command refusals that return their own exit code
  and have no later phase to fall through to.

**The converse is equally binding: a warning is never eligibility.** The
ten-row consumer table belongs to **GH #15's PRD §6.14**
(`docs/prds/PRD-recipe-generation-authority.md:2098`, table `:2140-2151`), not
to ADR-036 D13 — D13 is the **six-rung** verify severity ladder
(`docs/adrs/ADR-036-recipe-coverage-authority.md:1745`, ladder `:1755-1766`),
and rev-1 conflated the two. That table has ten rows, **seven** of them
warning-class with exit `0` — missing coverage, incomplete coverage,
`recipe-stale.json`, `canonical-patch-unparseable`, `recipe-undecodable`,
`recipe-owner-mismatch`, and any `object_kind` / `content_kind` of `unknown`.
Phase 2 hard-refuses every one of them, and the companion PRD §6.3 maps each to
the E-gate that refuses it. The default for any missing, malformed, incomplete
or stale-marked input is **nonterminal refusal**, never replay.

**Rationale.** An early design had phase-2 conflicts terminating with
`blocked`, which would have *removed* working phase-4 reapply behavior for
every feature whose recipe drifted. That is a regression dressed as a feature.
The shipped code already gets this right; the SPEC prose is what needs the
amendment.

### D4 - Eligibility is fifteen recomputed gates over ADR-036's own strict decoder

Phase 2 decodes `artifacts/recipe-coverage.json` with **ADR-036 D3's own strict
semantic decoder** — not a private re-implementation and not a weaker one. That
decoder already refuses every internally contradictory record
(`docs/adrs/ADR-036-recipe-coverage-authority.md:1065-1076`), so those
contradictions fail at E1 as decode failures and get no separate gate.

rev-0's `coverage-reason-outstanding`, `effect-axis-unknown` and zero-effect
rows were unreachable for exactly that reason and are removed. What survives is
**recomputation**: any value the decoder cannot check because it is a claim
*about other bytes* is re-derived and compared.

The gate set is closed and ordered; the first failure records its reason and
falls through (D3).

<!-- parity-block:B:begin -->
| # | Gate | Reason code | Why it is reachable |
|---|---|---|---|
| E1 | coverage present, readable, and strict-decodes under ADR-036 D3, **including** its contradiction refusals | `coverage-unusable` | absent, unreadable, malformed or internally contradictory bytes |
| E2 | `coverage.feature` == the requested slug | `coverage-owner-mismatch` | the envelope owner is not checked by the decoder |
| E3 | `coverage_status == complete` | `coverage-incomplete` | a truthfully incomplete record decodes fine |
| E4 | `patch_present` / `recipe_present` recomputed from **readable existence**, matching in **both** directions | `binding-presence-drift` | either file may have appeared or disappeared since publication, and readability may have changed either way |
| E5 | `patch_sha256` over exact raw patch bytes; `recipe_sha256` over exact **raw on-disk** recipe bytes | `binding-hash-drift` | bytes may have changed since publication, decodable or not |
| E6 | the **on-disk** recipe strict-decodes — `recipe_decodable` recomputed by **attempting the decode**, and true | `recipe-undecodable` | the stored flag is a claim about other bytes |
| E7 | **conditional on E6**: the decoded recipe's `feature` == `coverage.feature` | `recipe-owner-mismatch` | the recipe on disk may have changed owner since publication; only a decoded recipe has a readable `feature` |
| E8 | `reference.kind == commit`, 40-hex, the object **reconstructs offline in this repository**, and `preimage_set_sha256` recomputes over that reconstructed tree | `reference-not-reconstructable` | the object may be absent locally |
| E9 | every `effect_sha256` and `patch_fragment_sha256` recomputes equal, and `ordinal` is gapless strict-grammar record order | `effect-binding-drift` | a hand-forged or buggy record can state hashes the decoder cannot verify |
| E10 | every recipe operation is assigned to an effect, is a supported ADR-036 write-file witness, and carries **no** `created_by` (`internal/workflow/implement.go:49`) | `recipe-unsupported` | the on-disk recipe is re-read and re-checked (D7 / §6.6) |
| E11 | `recipe-stale.json` absent | `recipe-stale-marker-present` | a preserved drift marker (`internal/workflow/recipe_autogen.go:184-198`) |
| E12 | supersession authorizes nothing (D32 / §6.19) | `supersession-not-authority` | a superseded feature can carry complete coverage |
| E13 | no hard-blocked parent in the ADR-011 DAG (D32 / §6.19) | `parent-blocked` | a child can carry complete coverage |
| E14 | exactly one slug and `--cumulative-legacy` unset | `derivation-mode-unsupported` | invocation shape |
| E15 | no resolver shadow is staged for this slug (`Reconcile.ShadowPath` empty, `internal/store/types.go:381`) | `shadow-pending` | two staged answers may not coexist (D21 / §6.11) |

**The order is what makes every code reachable, and rev-1's was not.** rev-1
put a combined "the recipe decodes **and** its `feature` matches" gate at E4,
ahead of the presence, hash and decodability gates. That single gate consumed
every recipe-side failure first: a missing recipe, a drifted recipe and an
undecodable recipe all reported `recipe-owner-mismatch`, so
`binding-presence-drift`, `binding-hash-drift` and `recipe-undecodable` were
unreachable on the recipe side. rev-2 splits it into four ordered questions
that a recipe file answers in the only order it can answer them:

1. **E4 — does a readable file exist, on both sides, as the record claims?**
   Readable existence is the ADR-036 D9 collapse of absence and unreadability;
   a mismatch in either direction is drift.
2. **E5 — are its raw bytes the bytes that were bound?** The recipe hash is
   over raw on-disk bytes precisely so this question is answerable before, and
   independently of, decoding.
3. **E6 — do those bytes strict-decode?** This is the only gate that may
   report `recipe-undecodable`.
4. **E7 — does the decoded record name the right owner?** E7 is **conditional
   on E6**: it is evaluated only when E6 succeeded, because an undecodable file
   has no `feature` field to compare. E7 can therefore never shadow E6, and E6
   can never shadow E4 or E5.

**Evaluation is strictly first-match and the reported reason is
deterministic.** A record that would fail several gates reports the
**lowest-numbered** one, always — a fixture failing E3 and E11 reports
`coverage-incomplete`, and a fixture failing E5 and E6 reports
`binding-hash-drift`. No gate is skipped except E7 under the stated condition,
and no gate may be reordered by an implementation.

**Every gate is recomputed, never read.** An implementation that trusts
`coverage_status`, `recipe_decodable`, `patch_present` or any `*_sha256` as
stated has not satisfied the gate that names it.

**E9 and E10 are recomputation defenses.** ADR-036's decoder cannot verify a
hash over a *different* file, and predicate 5's operation assignment is a
producer-side statement. If either fails on a record the decoder accepted, the
record is a lie and phase 2 refuses; the codes are not reachable through any
honest producer. **E7 is a recomputation defense in the same sense** — a
truthful `recipe-owner-mismatch` already makes the record `incomplete` and
refuses at E3, so E7 fires only against a record whose claimed completeness
does not survive re-reading the recipe.
<!-- parity-block:B:end -->

**Rationale.** ADR-036 D4 and D9 require exactly this. Making the gate a closed
ordered table means an implementation cannot satisfy nine of fifteen and claim
the contract — and making every row *reachable* means the table is a
specification rather than a wish list, which is what rev-0's sixteen-row
version was not, and what rev-1's ordering silently undid on the recipe side:
a table whose rows are all present but whose order makes three of them
unreachable is the same defect wearing a different hat.

**Alternatives rejected.** Keeping a `coverage-reason-outstanding` gate "for
defense in depth": a gate that cannot fire cannot be tested, and an untestable
row in an ordered table is worse than no row. Writing a second, more permissive
decoder so those rows become reachable: that weakens ADR-036, which is
forbidden. Keeping rev-1's combined decode-and-owner gate and simply
documenting that three later codes are unreachable: an ordered table whose
documented codes cannot fire is a specification that lies about its own
vocabulary, and the split costs nothing — the recipe file answers the four
questions in that order anyway. Making E7 unconditional: an undecodable recipe
has no `feature` field, so an unconditional E7 either re-decodes (duplicating
E6) or reports a mismatch it did not observe.

### D5 - Both trees are reconstructed offline; producer scope and generation IDs are context only

Phase 2 reconstructs **two** commits: ADR-036's `reference.commit` (the base
the coverage's effects are bound to) and the resolved `upstreamCommit`. Both
must already exist in the local object database. **No lazy fetch, no network,
no ref advance** — every Git subprocess on this path carries
`GIT_NO_LAZY_FETCH=1` (D12). A reference commit that is not present is
`reference-not-reconstructable` and falls through; it is not an invitation to
go get it.

Three fields are explicitly **advisory context, never authority**:

- `coverage.producer` — names which of ADR-036 D15's seven producers wrote the
  record. A `record` record and an `artifact-edit` record get identical
  treatment; both are recomputed identically.
- `coverage.cross_base_status` — producer *scope*, not a verdict.
  `consumer-derivation-required` is a statement that the producer withheld
  cross-base authority, which is precisely the authority this ADR derives for
  itself.
- patch-generation identifiers (`pg_<12hex>`, ADR-024) — diagnostics. They
  prove which run wrote a patch, never which bytes are on disk now.

**Rationale.** ADR-036 D4 states the rule and this decision makes it
operational: a consumer that reads `cross_base_status: reference-tree-only` and
skips its own proof has re-created the failure the adjacent-CLI-args case study
documents (`docs/adrs/ADR-036-recipe-coverage-authority.md:1547-1558`).

### D6 - Candidate persistence is single-slug only in v1

A candidate is created only for an invocation naming **exactly one** slug with
`--cumulative-legacy` unset. Multi-slug invocations
(`internal/cli/cobra.go:2440` dispatches `args` as a slug list) and
`--cumulative-legacy` (`internal/cli/cobra.go:2542`) record phase-2
ineligibility with `derivation-mode-unsupported` and **retain their existing
fallback behavior byte-for-byte**.

ADR-030 D1/D2/D6/D7 are preserved unchanged: canonical `post-apply.patch`
remains authoritative in multi-slug reconcile, `--cumulative-legacy` still
opts into cumulative derivation, still skips the ADR-011 D9 topological
reorder and still skips phase 1.5.

**Rationale.** A cumulative patch is, by construction, a superset of the
previous slug's; deriving insertions from it would attribute another feature's
records to this one. And a multi-slug run's per-feature candidates would
interact — feature B's candidate would need to be derived against a tree that
already contains feature A's, which is a stack-replay problem this ADR
deliberately does not open.

### D7 - The v1 candidate domain is two shapes, and the persisted recipe is a witness

**The persisted ADR-036 recipe is an eligibility and coverage witness, not a
cross-base program.** Its `write-file` operation body is **never** executed
against the upstream tree — a whole-file postimage write is the exact
cross-base hazard ADR-036 D8 documents
(`docs/adrs/ADR-036-recipe-coverage-authority.md:1547-1558`). E10 checks that a
complete record's operations *are* supported witnesses; it does not license
running them.

Under ADR-036 v1 a `complete` record's operations are assigned supported
`write-file` witnesses by construction. `append-file` and `replace-in-file`
carry `operation-not-reclassifiable`
(`docs/adrs/ADR-036-recipe-coverage-authority.md:1141`) and therefore make the
record `incomplete`; `ensure-directory`, any `created_by` operation and any
unknown type fail predicate 5 or predicate 6 the same way. **So none of them
needs an independent candidate-domain refusal**, and this ADR makes no separate
`ensure-directory` claim: such a recipe simply cannot reach E3.

A candidate may only be derived when **every** normalized effect in the
coverage record falls into one of exactly two shapes:

**(a) Exact creation.** `change_kind: add`, `content_kind: text`,
`object_kind: regular`, `new_mode: 100644`, `old_mode: ""`.

**(b) Additive-only modification.** `change_kind: modify`,
`content_kind: text`, `object_kind: regular`,
`old_mode == new_mode == 100644`, and the effect's
`patch_fragment_sha256`-bounded fragment contains **zero** `-` content lines —
every hunk body line is `+` or context.

Presence at the upstream commit is a **classification** (D10), not a domain
condition. rev-0 made "absent upstream" a domain requirement for creations and
then also classified creations three ways, which contradicted itself; a
creation already present upstream with the exact postimage is simply
`absorbed-present`.

Everything else refuses the candidate and falls through:

| Refused | Why |
|---|---|
| `delete`, `rename`, `copy` | reproducing a deletion at another tree requires proving nothing else came to depend on the removed bytes |
| `content_kind: binary` or `none` | no record structure to align |
| `object_kind: executable`, `symlink`, `gitlink` | `write-file` execution is fixed at `0o644` (`internal/workflow/recipe.go:207-211`); the modes are not reproducible today |
| any mode change, including `100644`→`100755` | same |
| a `modify` fragment containing any `-` line | that is a replacement, and replacement requires commutation reasoning D33 forbids |

**Refusal is per-feature, not per-effect.** One unsupported effect refuses the
whole candidate. A partial candidate would silently redefine the feature.

**Vocabulary.** "Operation replay" is GH #13's issue title and is kept for
continuity. What v1 replays is the set of **normalized effects** ADR-036
derived from the canonical patch, re-expressed as newly derived `create-file`
and `insert-by-boundary` operations (D12). No persisted operation body is
executed cross-base.

**Rationale.** The two admitted shapes share one property that makes them safe
without a commutation theory: they only **add** records, so no upstream record
that someone else may now depend on is removed or rewritten. That property is
what lets D9's mapping be a pure alignment question.

**Alternatives rejected.** Admitting `100755` by teaching `write-file` a mode
field: a recipe schema change, which belongs to GH #15's surface and would
change all six skill formats. Admitting deletes with a "nothing references it"
check: that check is a whole-program analysis, not a byte comparison.

### D7b - Candidate derivation is permitted from exactly two feature states, and the state is bound

A candidate proposes to rewrite a feature's realized changes at a new upstream.
That proposition only has meaning for a feature whose changes are supposed to be
in the tree, or were and stopped applying.

**Permitted set.** Derivation requires the feature's current `status.State` to
be one of exactly two `store.FeatureState` values
(`internal/store/types.go:8-37`):

| State | Constant | Why it is permitted |
|---|---|---|
| `applied` | `store.StateApplied` (`:13`) | the feature's changes are supposed to be present; a candidate proposes how to keep them present at a newer upstream |
| `blocked` | `store.StateBlocked` (`:17`) | the feature's changes stopped applying; a candidate proposes how to make them apply again |

Every other state — `requested`, `analyzed`, `defined`, `implementing`,
`active`, `reconciling`, `reconciling-shadow`, `upstream_merged`, `rejected`,
`unapplied` — records the nonterminal refusal
`candidate-feature-state-unsupported` and falls through.

**It is a lifecycle precondition, deliberately outside D4's E-gate set.** D4's
fifteen gates are exactly the recomputations over the coverage record's own
bindings; a feature-state question is not one of them, and adding it as a
sixteenth would make the set mean two different things at once. It is evaluated
in D21's preflight, **before** E1, so a feature in a refused state never reaches
the coverage decoder at all.

**The state is bound into identity.** The permitted state observed at derivation
is recorded as `source_feature_state` in D19's identity tuple, in D20's
`candidate.json`, and in D20's `status.json` pointer, and the three must agree.
Binding it into identity — rather than merely recording it — is what makes
"the feature moved since this candidate was derived" a *different candidate*
rather than a stale field: the same bindings derived at `blocked` and at
`applied` produce two different IDs.

**Acceptance re-checks it.** D29 step 3 requires the current `status.State` to
equal the bound `source_feature_state` **and** to still be permitted; any drift
refuses `candidate-feature-state-changed` with exit `3`, before the journal,
the snapshots and any live write. Successful acceptance sets `store.StateApplied`
(D29 step 12), the same terminal state the resolver accept path reaches
(`internal/workflow/accept.go:123`).

**Generation itself changes no state** (D20b). Binding a state records an
observation; it is never a transition.

**Rationale.** rev-1 had no state precondition at all, so a candidate derived
while a feature was `blocked` could be accepted after an operator had explicitly
rejected the feature (ADR-031), after `tpatch unapply` removed its changes, or
after it was retired as `upstream_merged` — each of which would write code back
into a tree on the strength of a decision that had since been reversed. The
per-path CAS does not catch this: the CAS proves the *bytes* still match the
upstream tree, which they do, and says nothing about whether the feature is
still wanted.

**Alternatives rejected.** Checking the state only at acceptance: a candidate
would be staged, reviewed and diffed for a feature nobody intends to apply,
which wastes the operator's attention and the worktree. Recording the state
without binding it into identity: re-deriving after a state change would reuse
the existing candidate ID and silently overwrite the bound value, which is
exactly the mutable-identity problem D19 exists to prevent. Permitting
`unapplied`: there are no realized changes to reproduce, and reproducing them
would be an `apply`, not a reconcile.

### D8 - Insertion runs are derived from the strict canonical patch against ADR-036's named base

For an additive modification, phase 2 parses the effect's fragment with the
**same** ADR-036 D1 strict grammar that produced the coverage record — not a
second parser, not `strings.Fields`, and not any of the lenient readers ADR-036
D1 retired.

From the fragment it computes **maximal `+` runs**: each maximal consecutive
sequence of `+` lines in the fragment body, in fragment order, one-based within
the effect. Each run yields:

- its exact **insertion records** — the `+` line payloads with the leading `+`
  removed, each keeping its original line terminator byte-for-byte;
- its **base boundary** `b`, an index into the *reference-tree* file's record
  list with `0 ≤ b ≤ len(B)`, meaning "between base record `b-1` and base
  record `b`", computed by walking the fragment's hunk headers and context
  records against the reference-tree file content
  (`gitutil.FileAtCommit(repoRoot, reference.commit, path)`).

The derivation **must** reconcile: replaying every run at its boundary against
the reference-tree file must reproduce the effect's `postimage_sha256` exactly.
If it does not, the patch and the coverage disagree and the candidate refuses
(`derivation-postimage-mismatch`).

A fragment the strict grammar refuses is `fragment-unparseable`; a hunk whose
headers and context do not walk to a record-aligned boundary is
`fragment-boundary-underivable`. Both are recomputation defenses under D4.

**Rationale.** Anchoring to the base the coverage was bound to is the only
place the effect's meaning is defined. Anchoring to the upstream tree directly
would be circular — that is the thing being solved for.

### D9 - Boundaries map into upstream by exact line-record alignment, unanimously or not at all

**rev-0's byte-window context expansion is deleted in full.** It rejected
ordinary repeated bytes, had no decidable termination criterion, and could not
derive the motivating fixture (D9.1). This decision replaces it.

**Records.** A file's bytes are split into **line records**: each record is the
maximal byte run up to and including the next `\n`, and a trailing run with no
`\n` forms a final **no-newline record**. Record equality is **byte-exact**, so
`foo\n`, `foo\r\n` and `foo` are three different records. There is no
normalization anywhere: mixed EOL is simply exact bytes, and a file containing
both `\n` and `\r\n` records is ordinary input, not an error. rev-0's
`anchor-eol-ambiguous` refusal is deleted with the algorithm that needed it.

**Alignment.** Let `B` be the reference file's record list and `U` the upstream
file's record list at `upstreamCommit`. Phase 2 computes the set of **all**
maximum-cardinality monotone exact-record alignments between `B` and `U` — all
longest-common-subsequence traces — by a deterministic dynamic program. **No
tie is selected.**

<!-- parity-block:C1:begin -->
| # | Rule |
|---|---|
| A1 | An alignment is a monotone injective partial map `A` from base record indices to upstream record indices with `B[i] == U[A(i)]` byte-exactly for every `i ∈ dom(A)`, of maximum cardinality `L` |
| A2 | Under one alignment `A`, `left(A)` is `A(i)+1` for the **greatest** matched base index `i < b`, and `right(A)` is `A(j)` for the **least** matched base index `j ≥ b`. Base records the upstream deleted are simply unmatched and are skipped |
| A3 | Sentinels are read off the unanimous bounds A4 establishes, with **no traceback**: `left₀ == 0` **is** the start sentinel — it holds exactly when no optimal alignment matches any base record before `b` — and it is admissible **only** when `b == 0`, otherwise `alignment-left-unanchored`; `right₀ == len(U)` **is** the end sentinel, holding exactly when no optimal alignment matches any base record at or after `b`, and it is admissible **only** when `b == len(B)`, otherwise `alignment-right-unanchored` |
| A4 | The boundary is authoritative **only if every** maximum-cardinality alignment produces the **same** `(left, right)` pair; otherwise `alignment-ambiguous`. The decision is exact, not sampled: with `F[i][j] = LCS(B[0:i], U[0:j])` and `G[i][j] = LCS(B[i:], U[j:])`, the achievable bounds are `left₀ = min{ t : F[b][t] + G[b][t] == L }` and `right₀ = max{ t : F[b][t] + G[b][t] == L }`, and unanimity holds **iff** no pair `(i,j)` with `B[i] == U[j]` and `F[i][j] + 1 + G[i+1][j+1] == L` satisfies either `i < b ∧ j ≥ left₀` or `i ≥ b ∧ j < right₀` |
| A5 | The **gap** is the upstream record slice `U[left : right]`. An **empty** gap classifies **applicable**; a gap **byte-exactly equal** to the boundary's combined insertion records classifies **present**; **any other** gap classifies **conflict** (`boundary-conflict`). There is no global "the insertion bytes occur elsewhere" rule, in either direction |
| A6 | Runs sharing one `(path, b)` are combined in canonical order — ascending effect ordinal, then ascending run ordinal — and are classified as one boundary. Every boundary in one candidate is derived against a **single** upstream snapshot taken before the first classification |
| A7 | The mapped gaps of two **distinct** boundaries in one file must be **strictly** separated: for `b₁ < b₂` the rule is `right₁ < left₂`, and a violation is `alignment-range-overlap`. Strictness is exactly the collapse test — whenever any base record in `[b₁, b₂)` survives upstream, monotonicity already gives `right₁ < left₂`, so the rule can only fire when every base record between the two boundaries was deleted and the two map onto one region, empty or not. Applicable insertions are applied in **descending** upstream byte offset, where a record index is converted to a byte offset by summing the lengths of the preceding upstream records |
| A8 | A no-newline insertion record is admissible **only** at `b == len(B)` and only when the base file's final record ends with `\n` (or the base file is empty); any other shape refuses with `record-terminator-unsupported` |

**The rules are evaluated in one fixed first-match order, and it is part of
parity block C.** rev-1 listed the rules without saying which fires first,
which left the reported reason for a boundary that violates several rules
undefined. The order is:

| Step | Rule | Refusal if it fails |
|---|---|---|
| 1 | L1, L2, L3 resource limits, checked **before** any dynamic program is allocated | `alignment-limit` |
| 2 | A6 same-boundary combination, then A1/A2 construction of `F`, `G`, `L`, `left₀` and `right₀` | — (construction cannot fail once step 1 passed) |
| 3 | A4 unanimity | `alignment-ambiguous` |
| 4 | A3 sentinel legality, read off the now-established `left₀` / `right₀` | `alignment-left-unanchored` / `alignment-right-unanchored` |
| 5 | A8 insertion-record terminator admissibility | `record-terminator-unsupported` |
| 6 | A5 gap classification | `boundary-conflict` (a classification, not an eligibility refusal) |
| 7 | A7 cross-boundary strict separation, over every boundary of one file once all of them have mapped | `alignment-range-overlap` |

**Unanimity is decided before sentinel legality, deliberately.** A3's sentinels
are *derived from* the unanimous bounds, so asking "is this boundary anchored?"
before asking "do all optimal alignments agree?" would require materializing an
alignment to answer. Deciding A4 first means both sentinel questions are the
two DP comparisons `left₀ == 0` and `right₀ == len(U)`, and no traceback is
ever enumerated. When a boundary is ambiguous **and** would also be unanchored,
the reported reason is `alignment-ambiguous`, because step 3 precedes step 4.

**The reported reason is deterministic under multiple failures**: it is always
the earliest step's code, per file and then per boundary in ascending `(path,
b)` order, and the first refusing boundary refuses the whole candidate. Two
runs over the same two trees therefore report the same code for the same
reason, which is what makes the refusal vocabulary testable.

**Why A4's procedure is exact.** For a fixed optimal alignment `A`, the set of
upstream cut positions consistent with `A` at base boundary `b` is exactly the
interval `[left(A), right(A)]`, and a cut `t` is consistent with *some* optimal
alignment iff `F[b][t] + G[b][t] == L`. Therefore `left₀` is the minimum
`left(A)` over all optimal `A` and `right₀` the maximum `right(A)`; and
`left(A) > left₀` for some `A` iff some optimal matched pair `(i,j)` has
`i < b` and `j ≥ left₀`, symmetrically for `right`. So the two negative
conditions in A4 hold iff every optimal alignment yields `(left₀, right₀)`. The
procedure is `O(|B|·|U|)` in time and space and enumerates nothing.

**Why A3's sentinels need no traceback.** Every optimal alignment whose cut at
`b` is `t` matches exactly `F[b][t]` base records before `b`. Once A4 has
established unanimity, every optimal alignment cuts at `left₀ == right₀`, so
"no matched base index `i < b` exists" is exactly `F[b][left₀] == 0`, which is
exactly `left₀ == 0` — the minimum consistent cut can only be `0` when no
prefix match is required. Symmetrically, `right₀ == len(U)` is exactly "no
matched base index `j ≥ b` exists". So both sentinel questions reduce to a
comparison against a bound the procedure has already computed, and A3 never
inspects an alignment.
<!-- parity-block:C1:end -->

**Hard resource limits — closed compiled-in constants, no tuning, no
configuration, no flag and no environment override:**

<!-- parity-block:C2:begin -->
| # | Limit | Value |
|---|---|---|
| L1 | maximum bytes read for either side of one alignment | `4194304` (4 MiB) |
| L2 | maximum records in either record list | `131072` |
| L3 | maximum dynamic-program cells, `(len(B)+1) × (len(U)+1)` | `1048576` |

**L3 is a memory bound, and rev-2 states the budget it bounds.** The A4
procedure needs the **full** `F` and `G` matrices — the unanimity test ranges
over every matched pair `(i, j)`, not over one row — so an implementation holds
two `(len(B)+1) × (len(U)+1)` matrices of `int32` cell counts simultaneously.
At `1048576` cells that is `4 MiB` per matrix and **`8 MiB` for the pair**,
which is the whole dynamic-program budget for one file. Together with L1's two
`4 MiB` record buffers, one alignment's peak working set is bounded above by
**`16 MiB` plus O(records) of slice headers**, and no configuration can raise
it. rev-1's `16777216` cells would have been `128 MiB` of matrices for one
file, which is not a bound anyone would have noticed being hit.
<!-- parity-block:C2:end -->

Exceeding any limit refuses the candidate with `alignment-limit` and falls
through. The limit is a **refusal**, never a truncation, a sampling or a
downgrade — a truncating limit would silently change the alignment and
therefore the answer.

**Alignments are ephemeral.** No artifact carries a record body, a record
index, a byte offset, a line number or any file bytes. This is the direct
discharge of ADR-036 D8's deferral
(`docs/adrs/ADR-036-recipe-coverage-authority.md:1518-1541`), which parked
persisted anchors until GH #13 proved uniqueness and idempotence. GH #13 proves
them and **still does not persist them** (D11).

**Rationale.** Exact line-record alignment is decidable, has no tuning
parameter and no scoring function, and produces identical results on any
machine at any time from the same two trees. Critically, it is *deletion-aware*
in the only way that matters: a base record the upstream removed is simply
unmatched, so a boundary anchors to the nearest **surviving** neighbours
instead of failing. Refusing unless **every** optimal alignment agrees is what
makes "unique" a property of the input rather than of the traceback order.

**Alternatives rejected.** Byte-window expansion (rev-0): rejects ordinary
repeated bytes, has no termination criterion, and cannot bridge deleted
neighbours. Picking one LCS traceback: the answer would depend on DP iteration
order, which is not a property of the two trees. Fuzzy/similarity anchoring
(`git apply -C`-style fuzz): introduces a tolerance knob, and a knob is exactly
what makes a replay unreviewable. A fixed N-record context window: N is a
tuning parameter and its failure mode is a silent wrong match rather than a
refusal. A configurable resource limit: it would make the same two trees
produce different answers on different machines.

### D9.1 - Worked proof: the motivating adjacent-CLI-args fixture

Base records `B` (reference tree), with the feature's two arguments inserted at
boundary `b = 3`:

```
0  args := []string{\n
1  \t"--old-a",\n
2  \t"--old-b",\n
3  }\n
4  return run(args)\n
```

Upstream records `U`, after upstream intentionally deleted both old arguments:

```
0  args := []string{\n
1  }\n
2  return run(args)\n
```

`B[1]` and `B[2]` occur nowhere in `U`, so the only matches are `(0,0)`,
`(3,1)`, `(4,2)` and `L = 3`. At `b = 3`: `F[3][t] = 1` for `t ≥ 1` and
`G[3][t]` is `2, 2, 1, 0` for `t = 0..3`, so `{ t : F[3][t] + G[3][t] == 3 }`
is exactly `{1}` and `left₀ = right₀ = 1`. No optimal pair `(i,j)` has
`i < 3 ∧ j ≥ 1` or `i ≥ 3 ∧ j < 1`, so A4's unanimity holds. The gap `U[1:1]`
is **empty**, so A5 classifies **applicable** and the two feature records are
inserted at upstream record index `1`, producing byte-exactly the case study's
stated correct slice: both feature arguments present, both deleted arguments
still absent.

The `adjacent-between-delete-first` variant, where upstream deletes only
`--old-a`, resolves the same way at `b = 2` to `left₀ = right₀ = 1` and yields
the case study's other stated correct result, retaining upstream's surviving
`--old-b`.

**Why the deleted neighbours are not a problem.** A2 skips unmatched base
records, so the boundary anchors to the nearest surviving neighbours on each
side. Deleted base records contribute nothing to `U`, so they add nothing to
the gap. **Unrelated upstream additions in that region do**, and A5 refuses
them as `boundary-conflict` — which is the discrimination rev-0's global
"postimage elsewhere" rule was reaching for and could not express.

### D10 - Classification is three-state over one immutable snapshot

At a mapped boundary the gap decides the class (A5). For a **creation** effect
the same three states apply at file granularity: an upstream blob byte-equal to
the effect's postimage is **present**, an absent path is **applicable**, and a
path present with different bytes is **conflict** (`creation-conflict`).

**Every operation in one candidate is derived and classified against one
immutable upstream snapshot**, captured once before the first classification
and not re-read during derivation. Operations are never classified against a
partially-mutated intermediate.

**Rationale.** Descending application (A7) is the standard invariant for
offset-based edits and is what makes "derive all, then apply all" equivalent to
"derive and apply one at a time" without re-deriving. Combining same-boundary
runs in a canonical order is what makes the candidate diff byte-identical
across runs, which is what makes the candidate ID content-addressable (D19).

### D11 - Nothing alignment-shaped is persisted as future authority

No candidate artifact contains a record body, a record index, a byte offset, a
line number, a file preimage, a file postimage or any other bytes from which an
alignment could be reconstructed and re-used. The candidate metadata carries
**only**:

- `insertion_sha256` — the digest of the run's exact insertion records;
- `boundary_proof_sha256` — a digest over the derivation's own proof tuple
  (D20);
- the effect and binding hashes it recomputed.

**Those hashes are diagnostic, never authority.** At acceptance (D27) the
alignment is **re-derived from the bound inputs from scratch** and the freshly
computed hashes must equal the stored ones. A matching hash proves the inputs
did not move; it never substitutes for the derivation.

`candidate.patch` is a different object: it is a **reviewable unified diff**
and may contain diff source exactly as `artifacts/post-apply.patch` does. It is
for humans and for `--candidate-diff`; it is not an input to acceptance.

**Rationale.** A persisted alignment is a claim with no expiry. The moment one
exists, some future path will read it instead of re-deriving, and the whole
independence property collapses. Storing only digests makes the cheap path
(compare hashes) *insufficient* by construction — you cannot apply a digest.

### D12 - A typed two-root executor with two sanctioned constructors, and no lazy fetch

**Forbidden for candidate authority**, with reasons:

| Primitive | Anchor | Why it cannot be reused |
|---|---|---|
| `evaluateRecipeOperations` | `internal/workflow/reconcile.go:611` | reads `s.Root` (`:617-618,635-636`); `strings.Contains` presence (`:625`); `TrimSpace` equality (`:637`); `ensure-directory` counted present unconditionally (`:646`); unknown types counted **applicable** (`:648`) — fail-open on exactly the input that must refuse |
| `ExecuteRecipe` | `internal/workflow/recipe.go:90` | collects per-op errors and **continues the loop** (`:102-108`), reporting `Success` only at the end (`:110`) — not all-or-nothing at the operation layer |
| `dryRunOperation` | `internal/workflow/recipe.go:128` | hardcodes `repoRoot := s.Root` (`:129`) |
| `executeOperation` | `internal/workflow/recipe.go:199` | hardcodes `repoRoot := s.Root` (`:200`); `write-file` writes a fixed `0o644` (`:207-211`); `replace-in-file` is first-match and `append-file` unconditional (`:213-242`) |

Candidate work uses **typed roots** so a file operation cannot accidentally
target the live checkout:

```go
type ReferenceRoot struct{ /* reconstructed reference.commit; read-only */ }
type CandidateRoot struct{ /* isolated detached worktree at upstreamCommit */ }
```

**Exactly two sanctioned constructors are the only bare-root boundary:**

```go
func OpenReferenceRoot(repoRoot string, commit string) (*ReferenceRoot, error)
func CreateCandidateRoot(repoRoot, slug, candidateID, commit string) (*CandidateRoot, error)
```

Each constructor, and nothing else in the candidate package, may take a bare
`string` root. Each (1) validates `commit` as 40 lowercase hex and confirms the
object exists **locally**; (2) validates and contains its path with
`safety.EnsureSafeRepoPath` (`internal/safety/safety.go:12`) against the root
it is about to hold; and (3) sets `GIT_NO_LAZY_FETCH=1` — the shipped
`gitutil.NoLazyFetchEnv` constant (`internal/gitutil/trailers.go:41`) — on
**every** Git subprocess it or its returned root spawns.

Every subsequent read or write takes the typed root value, so passing `s.Root`
elsewhere is a compile error rather than a review finding. Store reads
(`.tpatch/` artifacts) go through the existing `*store.Store` and are a
separate, explicitly non-repository surface. Reads of a reconstructed tree stay
inside the ADR-034 rooted-inspection boundary.

**The source guard excludes exactly those two symbols, by name.** It fails on
any other candidate-package function accepting a `string` root, and on any
candidate-path Git invocation without `GIT_NO_LAZY_FETCH=1`, modelled on the
shipped environment guards at
`internal/gitutil/apply_classifier_test.go:319-320` and
`internal/cli/land_rev1_fold_test.go:236-237`. It does **not** exempt a
package, a file, a build tag or "arbitrary calls" — rev-0 left that boundary
undefined, which would have made the guard unenforceable.

**Closed candidate operation kinds** — exactly two, and unknown refuses:

| Kind | Payload | Precondition |
|---|---|---|
| `create-file` | `path`, `content_sha256`, `size` | path **absent** in `CandidateRoot` |
| `insert-by-boundary` | `path`, `run_ordinals`, `insertion_sha256`, `insertion_size`, `boundary_proof_sha256` | the boundary re-derives unanimously; the gap is empty |

**Rationale.** The single highest-risk failure in this feature is a
candidate-generation code path writing into the operator's real tree. A typed
root moves that from "we were careful" to "it does not compile" — but only if
the set of functions allowed to *mint* a root is finite and named, which is
what the two constructors give.

**Alternatives rejected.** Adding a `root string` parameter to the existing
functions: it makes the dangerous call *easier* to write, not harder, and it
does not fix their fail-open semantics. Reusing `write-file`: its fixed
`0o644`, whole-file semantics are exactly the cross-base hazard ADR-036 D8
documents. Allowing lazy fetch: a promisor fetch would make an "offline"
derivation depend on a network round-trip and could silently change which
objects the two trees contain mid-run.

### D12b - The legacy operation evaluator is retired as an authority, behind a config gate

`evaluateRecipeOperations` is **retired as an authority** by this feature.
Behind the `operation_candidate_enabled` config gate:

- it may still run, unchanged, to produce the shipped diagnostic note
  `Operation-level: %d present, %d applicable, %d conflicts`
  (`internal/workflow/reconcile.go:460-461`) when coverage is absent or
  ineligible, with its exact shipped byte output;
- it **never** produces a terminal `upstreamed`, a candidate, a confirmation,
  a state transition, or an evidence entry CG1 admits. The shipped
  `opResult.allPresent` early return
  (`internal/workflow/reconcile.go:451-457`) is removed from the authoritative
  path;
- every ineligible or legacy path records phase-2 evidence with the reason code
  that refused it (D4) and falls through.

**One existing expectation intentionally changes, and no byte identity is
claimed for it.** With the gate **on**,
`TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch`
(`internal/workflow/reconcile_evidence_integration_test.go:459`) no longer
reaches the confirmation gate from phase 2: its legacy fixture has no eligible
ADR-036 coverage, so it mints no `upstreamed` candidate at all and falls
through to earn its later phase's outcome. The gate-on expectation is added
**beside** the existing one, not edited into it. The gate's rejection arm stays
reachable from phase 3 (`internal/workflow/reconcile_test.go:304,354-359`) and
its confirmation arm from phase 1
(`internal/workflow/reconcile_evidence_integration_test.go:413`).

**`operation_candidate_enabled` defaults to `false` in v0.18.0**, so the
shipped baseline stays byte-identical and the S0 goldens remain a usable guard
for the whole wave. Flipping the default is a separate, soak-gated decision.

**Rationale.** Leaving a fail-open evaluator (`:646-647`, `:649-650`) able to
mint a terminal verdict beside a strict one is the worst of both worlds: the weaker
path would decide first for every feature without coverage. Retiring it is the
point of the feature. Doing so under a gate is what keeps the change honest —
a behavior change is disclosed and defaulted off rather than asserted to be
byte-identical.

**Alternatives rejected.** Fixing `evaluateRecipeOperations` in place: its
inputs are persisted operation bodies, which D7 establishes are witnesses, not
cross-base programs. Retiring it with no gate: it would change shipped
behavior for legacy features in the same release that introduces the feature,
with no way to separate the two effects during soak.

### D13 - Execution and persistence happen only at a real phase-4 textual conflict

Phase 2 produces a **proof**, not an artifact. The complete derived operation
set is executed **once**, in a candidate worktree created detached at
`upstreamCommit` under the D18 namespace, only when phase 4 has established
that the ordinary textual path cannot answer:

| Phase-4 arm | Held proof | Result |
|---|---|---|
| `ForwardApplyStrict` (`internal/workflow/reconcile.go:519`) | discarded | shipped `store.ReconcileReapplied` |
| `ForwardApply3WayClean` (`:527`) | discarded | shipped `store.ReconcileReapplied` |
| either arm promoted by `promoteIfMarkers` (`:505-517`) | discarded | shipped `blocked` / `phase-4-live-conflict-markers` |
| `ForwardApply3WayConflicts` with `opts.Resolve` set (`:544`) | discarded, **never persisted** | shipped blocked-by-parent short-circuit (`:552-567`) or `tryPhase35` (`:569`) |
| `ForwardApply3WayConflicts` without `--resolve` (`:574-585`) | valid and safe | finalized, executed, validated and persisted; returns `store.ReconcileStillNeeded` with a candidate-ready action |
| `ForwardApply3WayConflicts` without `--resolve` | absent, unsafe or ineligible | shipped `blocked` / `phase-4-forward-apply-conflicts` |
| exhausted arm (`:588-596`) | valid and safe | finalized as above; returns `store.ReconcileStillNeeded` |
| exhausted arm | absent, unsafe or ineligible | shipped `blocked` / `phase-4-blocked` |

If any operation fails a precondition or a write during finalization, the whole
candidate is abandoned and its worktree is removed. There is no partial
candidate, no "succeeded 3 of 4", and no `RecipeExecResult`-style per-op
accounting as an outcome.

**Finalization is the only locked part of generation, and it revalidates.** The
proof is computed lock-free. Finalizing it **reacquires** the per-feature lock
D21's preflight released before phase 1, takes the global lock only around
worktree mutation, and re-checks the preflight facts it depends on rather than
trusting observations made before the pipeline ran (D28). If either lock is
unavailable the finalization records `candidate-lock-held`, discards the proof
and falls through to the shipped `blocked` answer: a contended repository never
gets a partially staged candidate. Finalization also runs strictly **after**
phase 3 has completed and after phase 4's forward-apply preview
(`internal/workflow/reconcile.go:496`) has classified the arm — it is never
hoisted earlier.

**This is still a phase-2 semantic proof.** It is computed at phase 2, from
phase-2 inputs, and carries phase-2 evidence; it is *surfaced* at the phase-4
conflict boundary because that is the first point at which the cheaper textual
answers are known to have failed. There is **no phase 2.5**, no new phase value
and no reordering. `result.Phase` for the candidate-ready outcome is
`phase-2-operation-candidate`, and its **first** staged evidence attempt
carries `phase: phase-2` while its second carries `phase: phase-4` — the arm
substitutes its staged list for the derived entry, so the phase-2 finding and
the phase-4 textual-conflict finding coexist without duplication (D2, D24,
D25).

**Generation writes no live repository content.** The guarantee is scoped
precisely: not one byte of repository content under `s.Root` outside
`.tpatch/reconcile-candidates/<slug>/`. Four write sets are **expected** and
excluded from that statement — the candidate namespace itself, `.tpatch/locks/`
(D28), the feature's own `.tpatch/` artifacts, `status.json` and
`reconcile-evidence.jsonl` which every reconcile already writes, and
`.git/worktrees/<name>/`, Git's own bookkeeping for
`git worktree add --detach`, which is not repository content. rev-0's
unqualified "no live path is written" was false on all four counts.

**Rationale.** rev-0 made candidate-ready terminal at phase 2, which would have
suppressed a clean phase-4 reapply and a phase-3 provider `upstreamed` — both
strictly better answers than a staged candidate. Gating persistence on an
actual textual conflict means a candidate only ever *replaces* a `blocked`, and
`--resolve` still means what the operator typed. ADR-010 D2 already chose the
shadow worktree as the unit of atomicity for the resolver; this keeps that
isolation model and rejects the sibling's pruning and lifecycle (D18, D21).

**Alternatives rejected.** Terminal at phase 2: deletes working behavior
(above). A phase 2.5: misdescribes the architecture and breaks ADR-025's closed
phase enum (D24). Persisting the candidate at phase 2 and deleting it later if
phase 4 succeeds: a write that is usually undone is a race and an audit-trail
lie.

### D14 - Validation is a postcondition proof over the exact path set

After applying the complete set in the candidate worktree, phase 2 verifies:

1. **Per-effect postconditions.** For each derived operation, the affected file
   in `CandidateRoot` contains the exact insertion records at the re-derived
   boundary (or, for a creation, is byte-identical to the effect's postimage).
2. **Exact path set.** The set of paths differing between `upstreamCommit` and
   the candidate worktree equals **exactly** the derived operations' path set.
   A path more or a path fewer refuses.
3. **Mode fidelity.** Every touched path is a regular `100644` file on both
   sides. Any mode change refuses.
4. **No unmodeled change.** No path outside the derived set differs in content
   or mode.
5. **No conflict markers.** No `<<<<<<<`, `=======` or `>>>>>>>` line appears
   in any touched file — a candidate is a deterministic derivation, so a
   conflict marker means something merged when nothing should have.

**Rationale.** The resolver's validation gates are per-file and provider-shaped
(`internal/workflow/resolver.go:204-210`). A deterministic derivation can prove
something stronger: the exact set of changes, and nothing else.

### D15 - Idempotency is proved by a second full derivation over every effect

The **same** classifier and the **same** derivation are run a second time, with
the candidate worktree's result as the upstream input. The candidate is valid
only when:

- **every effect** — not merely every derived operation — classifies
  **present**;
- the applicable count is **zero** and the conflict count is **zero**;
- the Git tree hash of the candidate result and the byte content of the
  candidate diff are **unchanged** by the second pass.

This is a re-run of the production classifier, not a separate check with its
own logic. An implementation that adds a bespoke "idempotency checker", or that
asserts only over the derived operations while ignoring absorbed effects, has
not satisfied this decision.

**Any second-pass outcome other than all-present refuses.**
`candidate-not-idempotent` is not a nonzero-count code: it covers a second-pass
`boundary-conflict` or `creation-conflict`, a second-pass `alignment-ambiguous`,
`alignment-left-unanchored`, `alignment-right-unanchored` or
`alignment-range-overlap`, a second-pass `alignment-limit` (the candidate result
can be larger than the input that produced it), a second-pass
`record-terminator-unsupported`, a second-pass `derivation-postimage-mismatch`,
a changed tree hash, a changed diff, and anything else the second pass emits.
The rule is stated positively so it cannot be read as a list: **the second pass
must return all-present with zero counts and unchanged bytes**, and the
second-pass code is carried as diagnostic detail rather than as its own
outcome.

**Rationale.** Idempotency is the property that makes a replay reviewable: if
applying the candidate twice differs from applying it once, the derivation
found a place that moves, and no amount of postcondition checking catches that.
ADR-036 D8 named exactly this proof as the precondition for ever persisting an
alignment.

### D16 - Effect absorption is fully accounted; no effect goes unexplained

The candidate diff against `upstreamCommit` **may legitimately omit** effects
already present upstream — that is the point of the mixed case. What it may not
do is lose track of one.

Every effect in the old canonical patch maps to **exactly one** of:

- **absorbed-present** — proved byte-present at the mapped boundary (D10);
- **candidate-applicable** — realized by one or more derived operations.

Any effect that is neither refuses the candidate with `effect-unexplained`. The
mapping is recorded per-effect in the candidate metadata (D20) with the effect
ordinal, the effect hash and the resolving operation ordinals.

**Rationale.** "The diff is smaller than the patch" is either the correct
answer or a silent data loss, and the only thing that distinguishes them is a
total accounting.

### D17 - Candidate generation runs no configured test command by default

Candidate **generation and finalization** execute **no** configured
`test_command` (`internal/workflow/validation.go:60`, set at
`internal/cli/cobra.go:3731-3732`). Phase 2 runs on every reconcile of every
feature; making it execute arbitrary operator-configured commands would turn a
read-only classification into arbitrary code execution.

**Explicit candidate acceptance runs it**, in the candidate worktree, at D29
step 5 — before step 10's first live write — via the existing
`RunTestCommandInShadow` (`internal/workflow/validation.go:273`), the same
primitive the resolver already gates on
(`internal/workflow/resolver.go:206-207`). A non-zero result refuses the
acceptance; the candidate stays pending.

**Optional preview validation** may later be added as an **explicit flag**
only. It is not a default and not implied by `--candidate-diff`.

**Output vocabulary is pinned.** Because generation runs no tests, every
surface describing a generated candidate says **structurally validated** —
never "validated", "verified" or "tested" standing alone. `candidate.json`'s
`result.validated` is fixed at `structural`; the acceptance-time result is
recorded in `state.json`'s `accept_validation`, not in the immutable artifact
(D20).

**Rationale.** The word matters. An operator who reads "validated" beside a
diff will merge it; "structurally validated" is an accurate and deliberately
less reassuring claim.

### D18 - Candidates live in their own namespace, not `.tpatch/shadow`

```
.tpatch/reconcile-candidates/<slug>/<candidate-id>/
    candidate.json          immutable strict metadata (D20)
    state.json              mutable lifecycle (D20)
    candidate.patch         reviewable unified diff
    worktree/               detached worktree at upstreamCommit
    accept-journal.json     present only during an acceptance (D29)
    snapshots/              rollback snapshots (D29)
    staged/                 precomputed artifacts (D29)
.tpatch/reconcile-candidates/<slug>/rejected.jsonl   append-only (D23)
.tpatch/locks/reconcile/<slug>.lock                  advisory lock (D28)
.tpatch/locks/worktree-mutation.lock                 advisory lock (D28)
```

`.tpatch/shadow` (`internal/gitutil/shadow.go:35`) is **not** reused, and
neither is its lifecycle: `CreateShadow` reaps prior shadows on every create
(`internal/gitutil/shadow.go:56,72`), `ResolveShadow` picks newest-wins
(`:115`), and `PruneAllShadows` clears the lot (`:152`). Every one of those is
wrong for a candidate, which must be **stable** while an operator reviews it
and must never be silently replaced by a newer one.

**Every persisted path is repository-relative and slash-normalized.** Every
`path` value in `candidate.json`, `state.json`, `rejected.jsonl`, the status
pointer and every evidence field is produced by `filepath.ToSlash` over a path
already made relative to the repository root, matching the shipped convention
at `internal/gitutil/shadow.go:206,248`. A backslash-separated or absolute path
in any of those artifacts is a refusal, not a platform variation, so the
artifacts are byte-identical across Linux, macOS and Windows.

**Rationale.** Sharing the directory would make the resolver's reaping delete a
candidate an operator was reading, and would make `ResolveShadow`'s
newest-wins rule silently pick between two different kinds of object. And an
artifact whose bytes depend on the host's path separator cannot be
content-addressed (D19).

### D19 - The candidate ID is content-addressed over an immutable pre-execution tuple

```
candidate_id = "rc_" + first 12 lowercase hex of SHA-256(canonical JSON of the identity tuple)
```

The identity tuple is **exactly**: `schema_version`, `feature`,
`source_feature_state` (D7b), `upstream_commit`, `reference_commit`,
`coverage_sha256`, `patch_sha256`, `recipe_sha256`, `derivation_mode`, the
**ascending-sorted** list of derived-operation proof hashes, and the
**ascending-sorted** list of effect-map proof hashes. Each derived-operation proof hash is SHA-256 over the canonical
JSON of `{kind, path, effect_ordinal, effect_sha256, run_ordinals,
insertion_sha256, insertion_size, boundary_proof_sha256}`; each effect-map
proof hash is SHA-256 over the canonical JSON of `{effect_ordinal,
effect_sha256, path, resolution, operation_ordinals, reason_codes}`.

The tuple contains **no** candidate diff hash, **no** candidate tree hash,
**no** status and **no** self-reference to `candidate_id`, and no timestamp.

**Every member is known before the candidate worktree is created.** That is the
decision, not an observation: it is what lets the directory be named with the
ID directly, so there is **no staging directory and no rename**. rev-0's tuple
included `candidate_diff_sha256` and `candidate_tree_sha256`, which are results
of execution, so the ID could not exist until after the worktree it was
supposed to name.

**Rerun and collision.** Identical bindings recompute the identical ID. Before
anything is created, the ID is checked against `rejected.jsonl` (D23); a match
records `candidate-rejected` and falls through. Otherwise, if a directory with
that ID exists:

| Comparison | Result |
|---|---|
| `state.json` says `accepted` | `candidate-already-accepted` — refuse, fall through, touch nothing |
| immutable identity subset differs | `candidate-id-collision` — refuse, never overwrite |
| identity subset equal, `result` integrity fields differ | `candidate-integrity-drift` — refuse, never overwrite |
| both equal, `state.json` is `pending` or `stale` | reuse, **only after** the full D14 + D15 proof re-passes |

`state.json` is **not** part of either identity comparison, so a status
transition can never collide with, invalidate or be mistaken for an identity
change. The `accepted` row is a **lifecycle** check that runs first, not an
identity comparison.

**An accepted candidate directory is immutable audit, and nothing else.** Once
D29 completes, the directory is never reused, re-validated, rejected, pruned or
overwritten: step 14 removes `worktree/`, `snapshots/`, `staged/` and the
journal, and what remains — `candidate.json`, `state.json` at `accepted`,
`candidate.patch` — is a read-only record of what was accepted. Step 14 also
**clears** the `status.json` pointer, so an accepted directory has no live
pointer, D21's preflight ignores it entirely, and it is never marked `stale`
when the upstream moves. A later reconcile that recomputes the same ID records
`candidate-already-accepted` and falls through **before** creating anything;
any changed binding — including `source_feature_state` — mints a different ID
and a new candidate normally, which is the ordinary path after an accept
because acceptance republishes all three bindings (D30).

**Rationale.** This is the third instance of one house pattern — ADR-024's
`pg_<12hex>` and ADR-025 D3's `re_<12hex>` are the siblings — chosen for the
same reasons: short enough to print, content-derived so byte-identical reruns
are equivalent, and prefix-namespaced so an ID's origin is readable. Excluding
execution results is what makes the naming acyclic.

**Alternatives rejected.** Including the diff and tree hashes for
"completeness": it creates the staging-rename cycle, and the integrity those
hashes provide is delivered instead by revalidating them in full at every reuse
and at acceptance. Including `status`: a rejection would mint a new ID, so the
rejection log could never match anything.

### D20 - Immutable metadata and mutable state are two files

`candidate.json` is v1, immutable, strict in both directions, with **no
status**, no timestamp, no source body, no record body, no index, no offset, no
prompt and no secret:

<!-- parity-block:D1:begin -->
```json
{
  "schema_version": 1,
  "candidate_id": "rc_0a1b2c3d4e5f",
  "feature": "fix-model-id-translation",
  "source_feature_state": "blocked",
  "upstream_commit": "<40 lowercase hex>",
  "reference_commit": "<40 lowercase hex>",
  "derivation_mode": "single-slug",
  "bindings": {
    "coverage_sha256": "<64 lowercase hex>",
    "patch_sha256": "<64 lowercase hex>",
    "recipe_sha256": "<64 lowercase hex>"
  },
  "derivation": {
    "effect_count": 3,
    "absorbed_present_count": 1,
    "candidate_applicable_count": 2,
    "conflict_count": 0,
    "operations": [
      {
        "ordinal": 1,
        "kind": "insert-by-boundary",
        "path": "cmd/serve.go",
        "effect_ordinal": 2,
        "effect_sha256": "<64 lowercase hex>",
        "run_ordinals": [1, 2],
        "insertion_sha256": "<64 lowercase hex>",
        "insertion_size": 148,
        "boundary_proof_sha256": "<64 lowercase hex>",
        "classification": "applicable"
      }
    ],
    "effect_map": [
      {
        "effect_ordinal": 1,
        "effect_sha256": "<64 lowercase hex>",
        "path": "cmd/serve.go",
        "resolution": "absorbed-present",
        "operation_ordinals": [],
        "reason_codes": []
      }
    ]
  },
  "result": {
    "candidate_diff_sha256": "<64 lowercase hex>",
    "candidate_tree_sha256": "<64 lowercase hex>",
    "validated": "structural"
  }
}
```
<!-- parity-block:D1:end -->

`result` carries **integrity** fields, not identity: they are revalidated in
full at every reuse and at acceptance, and they are excluded from the ID (D19)
precisely so the ID is computable before the operations run.

`state.json` is v1 and is the **only** mutable file:

<!-- parity-block:D2:begin -->
```json
{
  "schema_version": 1,
  "candidate_id": "rc_0a1b2c3d4e5f",
  "status": "pending",
  "accept_validation": "none"
}
```
<!-- parity-block:D2:end -->

<!-- parity-block:D3:begin -->
Closed enums: `derivation_mode` ∈ {`single-slug`}; `source_feature_state` ∈
{`applied`, `blocked`} (D7b / §6.6a); `kind` ∈ {`create-file`,
`insert-by-boundary`};
`classification` ∈ {`present`, `applicable`, `conflict`}; `resolution` ∈
{`absorbed-present`, `candidate-applicable`}; `validated` ∈ {`structural`};
`status` ∈ {`pending`, `stale`, `rejected`, `accepted`}; `accept_validation` ∈
{`none`, `structural`, `structural+tests`}. Arrays are present and non-null,
sorted by ordinal; unknown fields are refused at decode.
<!-- parity-block:D3:end -->

The two JSON bodies above and this enum paragraph are **parity block D**.

`boundary_proof_sha256` is SHA-256 over the canonical JSON of
`{path, base_boundary_index, upstream_left_index, upstream_right_index,
base_record_count, upstream_record_count, alignment_length,
insertion_record_count, insertion_sha256}`. The tuple is hashed and discarded;
none of its members is ever written to an artifact (D11).

**`status.json` gains one optional pointer**, separate from the resolver's:

```json
"reconcile": {
  "operation_candidate": {
    "candidate_id": "rc_0a1b2c3d4e5f",
    "path": ".tpatch/reconcile-candidates/<slug>/rc_0a1b2c3d4e5f",
    "status": "pending",
    "source_feature_state": "blocked"
  }
}
```

It is `omitempty` so the no-candidate case stays byte-identical to today's
`ReconcileSummary` (`internal/store/types.go:372`), and it is **not**
`ShadowPath` / `ResolveSession` (`internal/store/types.go:381-382`), which stay
resolver-only. It is also the one field of the summary that must **survive** a
later, unrelated reconcile: D20b's shared status merge carries it forward on
every non-candidate arm, and only the five candidate transitions may change
it.

**Rationale.** ADR-025 D10 and ADR-027 D2 already make body-free artifacts a
repository-wide rule; a candidate is the artifact most tempted to carry bodies,
so the prohibition is restated rather than assumed. Splitting immutable
identity from mutable lifecycle is what makes the D19 collision comparison
meaningful: without it, rejecting a candidate would rewrite the very file whose
bytes the collision check compares.

### D20b - A candidate-ready result changes no lifecycle state, bypasses `updateFeatureState`, and the pointer survives every other write

A candidate-ready `store.ReconcileStillNeeded` **must not** flow through
`updateFeatureState` (`internal/workflow/reconcile.go:1181`), whose
`ReconcileStillNeeded` arm forces `status.State = store.StateApplied` and
overwrites `status.Notes` (`:1220-1222`). A feature that was `blocked` before
the reconcile would silently become `applied` because a *reviewable proposal*
was staged.

It persists instead through a dedicated path that **calls**
`saveReconcileArtifacts(s, slug, result, nil)` rather than reimplementing it:
the candidate-ready result reaches the shipped function carrying its two staged
`verdictAttempts` (D2 configuration 3) and a **`nil`** proof, so
`reconcile-session.json` and `reconcile.md` are written by the shipped writer,
the staged list substitutes for the derived entry at `:766`, and the shipped
`:766` → `:771` call order and every detector's own firing condition are
preserved exactly — file-novelty and hunk-overlap still run, the blocked-only
pair still does not, and the confirmation gate returns immediately because the
outcome is not `upstreamed` (`:1033-1035`). What is dedicated is the **status
write**: it sets `status.Reconcile` — including the D20 pointer — and
`status.LastCommand` / `status.UpdatedAt` without calling
`updateFeatureState`, and **leaves `status.State` and `status.Notes` exactly as
loaded**. The outcome is carried by the reconcile summary alone. A
reimplementation of the artifact or evidence writers is a violation of this
decision even where its bytes coincide on one fixture.

No `FeatureState` transition, no `StateReconcilingShadow`
(`internal/store/types.go:16`), no new state. A pending candidate is a
reviewable artifact beside the feature, not a mode the feature is in. The
all-present arm continues to use `updateFeatureState` and the confirmation gate
(D2), unchanged.

**The bypass is necessary but not sufficient: the pointer must also survive
every write that is not about candidates.** `updateFeatureState` does not merge
`status.Reconcile`; it **replaces** it with a freshly constructed
`store.ReconcileSummary` literal built from the current result
(`internal/workflow/reconcile.go:1191-1209`). So the very next ordinary
reconcile of a slug — a phase-1 `upstreamed`, a clean phase-4 `reapplied`, a
phase-4 `blocked`, a gate-off run, a multi-slug run, a run whose preflight
refused with `candidate-lock-held` — would drop `operation_candidate` while the
candidate directory stayed on disk. The result is a pending candidate nobody
can address and a `--accept-candidate` that reports `candidate-absent` beside a
real worktree.

The rule is a **shared status merge**, used by every writer of
`status.Reconcile`: load the status, build the new summary from the result as
today, **copy the loaded `Reconcile.OperationCandidate` into it before
assigning**, then apply that writer's own state and note rules. The ordering is
part of the decision — the carry-forward happens **before** the standard
`ReconcileSummary` replacement, never as a repair afterwards, so no written
value ever lacks the pointer. `updateFeatureState` and the candidate-ready path
call the same helper, so exactly one place in the codebase preserves it.

**The carry-forward is not a lifecycle-state change.** It copies one
`omitempty` sub-object; `status.State`, `status.Notes`, `status.Apply` and
every other field keep today's semantics, and the bypass above is unaffected.

**Exactly five writers may set, modify or clear the pointer** — candidate
finalization (D13), the staleness preflight (D21), rejection (D23), acceptance
(D29 steps 12 and 14) and recovery (D29b) — each under the per-feature lock,
each changing the pointer and the on-disk directory together.

**Carrying the pointer forward does not keep the candidate acceptable.** An
outcome that moves the feature out of D7b's permitted set — a phase-1
`upstreamed` sets `store.StateUpstreamMerged`
(`internal/workflow/reconcile.go:1214-1216`) — leaves the pointer in place so
that `--accept-candidate` can refuse `candidate-feature-state-changed` by name
(D28), and so the operator can still inspect or reject the proposal. Dropping
the pointer would report `candidate-absent` for a directory that exists, which
is a worse answer to the same situation. A source guard
enumerates every `store.ReconcileSummary` construction in production code and
requires each to come from the shared merge or to be on that five-writer
allowlist.

**Rationale.** rev-0 asserted "candidate generation changes no lifecycle state"
while routing the result through the one function that changes it; rev-2 fixed
that and left a second, quieter version of the same defect — the pointer was
written by one path and silently destroyed by another. A pending proposal whose
address disappears on the next unrelated command is worse than no proposal,
because the directory, the worktree and the rejection log all survive it.

**Alternatives rejected.** Re-deriving the pointer from the filesystem on every
read: it makes an untracked directory authoritative over `status.json` and
would resurrect stale or rejected candidates. Making the candidate a
`FeatureState`: that is the lifecycle change this decision exists to avoid.
Repairing the pointer after `updateFeatureState` returns: there is then a
written status without it, and a crash between the two writes loses the
candidate for good.

### D21 - Preflight order, one pending candidate per slug, and no coexistence with resolver shadows

**Preflight ordering.** Before phase 1 and before the phase-1.5 sweep, and for
**every** reconcile invocation and every candidate action naming this slug:

0. **Lock.** Acquire the per-feature reconcile lock
   `.tpatch/locks/reconcile/<slug>.lock` **nonblocking** (D28). Steps 1-3 run
   while it is held, and it is released before phase 1 — before any provider
   call, any forward-apply preview and any other expensive work. On contention
   the preflight performs **no** inspection, **no** mutation and **no** prune:
   a plain reconcile records the nonterminal `candidate-lock-held` and
   continues with derivation disabled; a terminal candidate action refuses with
   `candidate-lock-held` and exit `3`. `Supported == false` and
   `lock-filesystem-unsupported` behave the same way under their own codes.
1. **Recovery check.** Any `accept-journal.json` under
   `.tpatch/reconcile-candidates/<slug>/` refuses the whole command with
   `recovery-required` (D29b). Nothing else in this list runs, nothing is
   pruned, and the lock is released before the refusal returns.
2. **Staleness check.** With no journal present, a **pending** candidate whose
   `upstream_commit`, whose three binding hashes or whose bound
   `source_feature_state` (D7b) no longer matches the current resolution is
   marked `stale` in `state.json`, its `worktree/` is pruned safely
   (`git worktree remove` then directory removal, the `pruneShadowPath` shape
   at `internal/gitutil/shadow.go:309`, in the candidate namespace), its
   immutable `candidate.json` is **retained for audit**, and the `status.json`
   pointer is cleared. **No silent overwrite**, ever. Directories at `accepted`
   or `rejected` have no live pointer, are immutable audit (D19), and are
   **ignored** here.
3. **Feature-state precondition.** D7b's permitted set is checked; a feature
   outside it records `candidate-feature-state-unsupported` and the invocation
   continues with derivation disabled.

**Every candidate-state mutation happens under the lock.** Marking `stale`,
pruning a worktree, setting or clearing the `status.json` pointer, appending to
`rejected.jsonl` and writing `state.json` are read-modify-write sequences on
state a concurrent acceptance also mutates. There is **no unlocked prune, mark,
pointer write or rejection append anywhere** in this ADR. Finalization (D13)
reacquires and revalidates; acceptance (D29) holds the per-feature and global
locks for the whole transaction; rejection (D23) and recovery (D29b) hold the
per-feature lock. Candidate *derivation* holds nothing, because it mutates
nothing.

**A candidate and a resolver shadow may not coexist.** With one pending, the
other's actions refuse with exact remediation naming the pending object and the
commands that clear it:

| Attempted | Existing | Refusal |
|---|---|---|
| `--resolve` / `--accept` / `--reject` / `--shadow-diff` | pending candidate | `candidate-pending` — run `tpatch reconcile <slug> --reject-candidate` or `--accept-candidate` first |
| `--accept-candidate` / `--reject-candidate` / `--candidate-diff` / `--clear-candidate-rejections` | resolver shadow (`Reconcile.ShadowPath` non-empty) | `shadow-pending` — run `tpatch reconcile --accept <slug>` or `--reject <slug>` first |
| candidate derivation | resolver shadow present | E15 ineligibility `shadow-pending`, fall through |

**Rationale.** Ordering recovery ahead of staleness is not cosmetic: pruning a
worktree that a half-finished acceptance journal still references would destroy
the rollback material. Holding the lock across the whole inspect-then-mutate
sequence is equally load-bearing: rev-1 described the staleness prune as
unlocked, which races an in-flight acceptance that is re-materializing the same
worktree at D29 step 5. Releasing the lock before phase 1 rather than holding it
across the pipeline is what stops one slow provider call serializing every
reconcile in the repository, and finalization's revalidation is what makes the
released window safe. Two staged answers to the same question, each with its
own accept path, is how an operator accepts one while reading the other's diff;
mutual exclusion with a named remediation is strictly better than a merge rule.

### D22 - Five new terminal CLI flags, eight terminal actions, additive exclusivity

`tpatch reconcile` gains exactly **five** flags, each taking **exactly one**
slug:

| Flag | Effect |
|---|---|
| `--candidate-diff <slug>` | print the pending candidate's reviewable diff; read-only; no state, no artifact |
| `--accept-candidate <slug>` | run the D27-D30 acceptance transaction |
| `--reject-candidate <slug>` | record the rejection, prune, clear the pointer (D23) |
| `--clear-candidate-rejections <slug>` | truncate this slug's `rejected.jsonl`; nothing else |
| `--resume-candidate-recovery <slug>` | complete or roll back a recorded acceptance journal (D29b) |

The three existing terminals — `--accept`, `--reject`, `--shadow-diff` — are
unchanged. **Eight terminal actions exist in total.** rev-0 said three and six;
both were wrong, because `--clear-candidate-rejections` and
`--resume-candidate-recovery` are terminal actions too.

**Exclusivity is source-derived, not a hand-maintained list.** rev-1 pinned a
fixed count of 55 pairs against a hand-copied list of nine existing flags. That
list omitted `--upstream-ref`, `--timeout`, `--allow-dirty`,
`--allow-stale-lock`, `--max-conflicts` and `--model`, and it would go stale the
moment `reconcileCmd` registered another flag. rev-2 derives the rule from the
command's own flag set:

> A new terminal flag may be combined **only** with a flag on the presentation
> allowlist. Every other flag registered on the reconcile command refuses when
> it is **explicitly set**, tested with `Flags().Changed(name)` so a default
> value is never mistaken for an operator's intent.

| Set | Members | Rule |
|---|---|---|
| presentation allowlist | the root persistent `--path` (`internal/cli/cobra.go:73`) and reconcile's `--format` (`:2543`) | permitted beside a new terminal |
| excluded by name | Cobra's auto-registered `help` flag | **not** behavioral, **not** counted |
| reconcile-local behavioral flags | the other fifteen flags registered at `internal/cli/cobra.go:2520-2543`: `--upstream-ref` (`:2520`), `--timeout` (`:2521`), `--preflight` (`:2522`), `--allow-dirty` (`:2523`), `--allow-stale-lock` (`:2524`), `--resolve` (`:2526`), `--apply` (`:2527`), `--max-conflicts` (`:2528`), `--model` (`:2529`), `--accept` (`:2530`), `--reject` (`:2531`), `--shadow-diff` (`:2532`), `--check-applied-only` (`:2534`), `--auto-drop-merged` (`:2535`), `--cumulative-legacy` (`:2542`) | each refuses when `Changed` |
| new × new unordered pairs | 10 | each refuses |
| existing × existing | unchanged | shipped `validateReconcileFlags` rules and **shipped message bytes** (`internal/cli/cobra.go:3200`, rules `:3211-3218`, strings `:3212,3215,3218`), plus the shipped `--check-applied-only` / `--auto-drop-merged` rule (`:2316-2325`) |

**`help` is excluded explicitly, by name, and rev-3 says so because
`cmd.Flags()` does not return what `reconcileCmd` registered.** Cobra adds
`help` to a command during execution, so a naive enumeration finds sixteen
non-presentation flags, reports `5 × 16 = 80` plus `10` — `90` pairs — and
refuses `--accept-candidate <slug> --help`, which is a request for
documentation, not a conflicting instruction. The subtraction is by flag name
and is asserted against the registered set.

**There is no `--json` flag on this command**; reconcile's presentation flag is
`--format` with values `human` and `json` (`:2543`), and the root command's only
persistent flag is `--path` (`:73`). For the current flag set the refusing pairs
that involve a new flag are `5 × 15 = 75` plus `10`, **85** in total — stated as
a consequence of the rule, never as the rule. The implementation enumerates
`cmd.Flags()`, subtracts the two-member presentation allowlist and `help`, and
the guard asserts the enumeration against the registered set, so a sixteenth
reconcile flag is covered automatically.

**Explicit `--upstream-ref` is refused deliberately.** A terminal candidate
action operates on the `upstream_commit` bound into the candidate (D19).
Honouring an alternative ref would either be ignored — a lie — or would
re-resolve against a commit the candidate never proved, which is the exact
substitution the per-path CAS exists to prevent. `--timeout` and `--model` are
refused for the same reason: there is no provider call and no phase pipeline in
a terminal candidate action for them to govern.

**Existing combinations are never re-routed.** The new rule is a second
validation stage that runs **only when at least one new flag is set**. An
invocation using only shipped flags reaches exactly the shipped code path and
produces exactly the shipped error strings.

**The existing three stay resolver-only.** They keep their exact shipped
meanings and never operate on a candidate. An `--accept <slug>` with a pending
candidate and no shadow refuses by name (D21) rather than guessing.

**Rationale.** Overloading `--accept` to mean "accept whichever staged thing
exists" is the single worst available option: it makes a destructive command's
behavior depend on invisible state. And retroactively routing shipped flag
combinations through a stricter validator would change error bytes operators
and scripts already depend on, for no safety gain — the new flags are the only
thing that needs the new rule. Deriving the exclusivity set from `cmd.Flags()`
rather than from a literal list is what stops the guard rotting: a hand-written
list is a snapshot of the flag set at authoring time, and this ADR's own rev-1
proved that such a snapshot is wrong on the day it is written.

### D23 - Rejection is recorded, and the same ID does not regenerate

`--reject-candidate`, under the per-feature lock for the whole action (D28):

1. appends a rejection record to
   `.tpatch/reconcile-candidates/<slug>/rejected.jsonl` — `candidate_id`,
   `source_feature_state`, `upstream_commit`, `reference_commit` and the three
   binding hashes, no bodies, no timestamp;
2. sets `state.json` to `status: rejected` and prunes `worktree/`, retaining
   the immutable `candidate.json`;
3. clears the `status.json` pointer;
4. **changes no lifecycle state** — the feature is exactly where it was.

**Subsequent reconciles fall through rather than recreating.** D19's
pre-creation check means a later reconcile deriving an ID already in
`rejected.jsonl` records ineligibility `candidate-rejected` and falls through
**before** creating a directory or a worktree. Identical inputs producing an
identical ID means the operator already answered this exact question.

**Changed bindings mint a new ID and a new candidate.** A different upstream
commit, patch, recipe or coverage produces a different ID, which is not in the
rejection log, so a new candidate is created normally.

**Clearing is explicit and unambiguous.** `--clear-candidate-rejections`
truncates `rejected.jsonl` for that slug and does nothing else — no
regeneration, no state change, no pruning. It is a separate terminal flag,
subject to the same exclusivity, so "I want to reconsider" is a typed sentence
rather than an inferred one.

**Rationale.** Without the log, rejecting a candidate and re-running reconcile
regenerates it immediately, which is an infinite loop with extra steps. Doing
the check *before* creation is what keeps the loop from costing a worktree.

### D24 - Evidence uses `phase-2` and `recipe-operation-match`; there is no phase 2.5

Candidate evidence uses the **existing** ADR-025 D4 enum values — `phase`
`phase-2`, `evidence_kind` `recipe-operation-match`
(`internal/store/reconcile_evidence.go:41`,
`internal/workflow/reconcile.go:1167-1168`). **No new phase value, no phase
2.5, no new evidence kind.**

Confidence ladder:

| Phase-2 result | `confidence` |
|---|---|
| all effects present with CG1-CG7 satisfied (D2) | `high` |
| candidate finalized and fully validated (D14 + D15) | `high` |
| conflicting effect | `low` |
| ineligible (any E-gate refusal, D4) | `unknown` |

The all-present entry's remaining fields are pinned by D2's field table —
`match_origin: upstream`, `pre_reconcile_presence: present`,
`upstream_commit_refs: [upstreamCommit]`, `requires_confirmation: false` — and
the shipped legacy builder (`internal/workflow/reconcile.go:843-845`) keeps its
`low` / `unknown` / `[]` triple, so the two entries are distinguishable by
value and not only by provenance.

**The candidate-ready arm's second staged attempt also reuses existing enum
values.** It carries `phase: phase-4`, `evidence_kind: forward-apply` and
`confidence: low` — the shipped `forward-apply`-plus-blocked rung
(`internal/workflow/reconcile.go:846-849`) — with the phase-local verdict
`blocked`; its exact fields are pinned by D2's two-attempt table. That is still
an ADR-025 D4 pair the shipped pipeline already writes, so this decision's "no
new phase value, no new evidence kind" claim covers both staged attempts.

`matched_paths` and `matched_operations` carry slash-normalized paths and
operation identifiers. **No source body, no matched record, no insertion byte**
appears in evidence — ADR-025 D10's privacy boundary and ADR-029 D8's
no-source-bodies rule both apply unchanged.

**Rationale.** A phase 2.5 would be a lie about the architecture: this is
phase 2 doing what `SPEC.md:444-448` always said it did. New enum values would
also break every existing aggregator for no gain. Confidence stays a
*consequence* of the proof, never an input to the D2 gate.

### D25 - A reconcile invocation appends multiple ordered attempts, each with its own phase-local verdict

This decision **amends** ADR-025 D1/D3's "one line is one evidence attempt for
one reconcile verdict" reading
(`docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:70-72`).

Today `persistReconcileEvidence` (`internal/workflow/reconcile.go:766`,
defined `:812`) derives a **single** phase from `result.Phase` via
`evidencePhaseAndKind` (`internal/workflow/reconcile.go:1161-1176`), so a
fallthrough loses the phase-2 finding entirely. The pipeline already appends up
to six entries per invocation (`internal/workflow/reconcile.go:766-771`), so
multi-entry is the shipped shape; what is missing is an **ordered attempt
list** rather than one phase-keyed derivation.

**rev-1 pinned "at most one attempt per (invocation, phase) pair". That is
false against the shipped code; rev-2 corrected the rule but miscounted the
classes. The tree says:**

| Call site | Entry | Phase written | Fires when |
|---|---|---|---|
| `internal/workflow/reconcile.go:766` | `persistReconcileEvidence`, phase derived from `result.Phase` (`:1161-1176`) — or, when the GH #13 evaluator staged any, the whole of `result.verdictAttempts` in slice order **in place of** the derived entry (§6.2 / D2) | any of `phase-1`, `phase-1.5`, `phase-2`, `phase-3`, `phase-3.5`, `phase-4` | any non-empty outcome (`:813-815`) |
| `:767` | file-novelty detector (`internal/workflow/file_novelty.go:115-116`) | `phase-3.5` | any outcome with an upstream commit, a base commit and a canonical patch (`:884-898`) |
| `:768` | hunk-overlap detector (`internal/workflow/hunk_overlap.go:135-136`) | `phase-3.5` | the same, plus a modifying or mixed-additive novelty classification (`:913-923,928-930`) |
| `:769` | path-restructure detector (`internal/workflow/path_restructure.go:118-119`) | `phase-3.5` | **blocked outcomes only** (`:947-949`) |
| `:770` | blocked-classification taxonomy (`internal/workflow/blocked_taxonomy.go:179`) | `phase-4` | **blocked outcomes only** (`:998-1000`) |
| `:771` | the confirmation gate's own entry (`internal/workflow/reconcile.go:1069-1070`) | `phase-3.5`, kind `manual-review` | **`upstreamed` outcomes only** (`:1033-1035`) |

**The shipped maxima are one verdict-bearing entry plus three supporting
entries at `phase-3.5`, and one plus one at `phase-4`.** rev-2 called all four
`phase-3.5` lines supporting; one of them is the main pipeline verdict whenever
`result.Phase` is a phase-3.5 value. And the three supporting ones are never
the same three in every shape, because the gate's entry and the blocked-only
detectors exclude each other — `:769` and `:770` run before the gate can demote
an unconfirmed `upstreamed` to `blocked` (`:1089-1090`), so no invocation
writes both:

| Invocation shape | Supporting `phase-3.5` | Supporting `phase-4` |
|---|---|---|
| ends blocked | file-novelty, hunk-overlap, path-restructure | blocked-classification |
| ends `upstreamed`, confirmed or demoted | file-novelty, hunk-overlap, the gate's `manual-review` | none |
| ends `reapplied` or `still_needed` | file-novelty, hunk-overlap | none |

| Class | Members | Cardinality | `raw_reconcile_verdict` |
|---|---|---|---|
| **verdict-bearing** | the main pipeline result entry (`:766`, defined `:812`), which on the two authoritative arms **is** the staged list — one qualifying GH #13 phase-2 proof entry on the all-present arm, two ordered attempts on the candidate-ready arm — emitted in place of the derived entry rather than beside it; the derived entry itself on every other arm; the final overall-phase entry | **at most one per (invocation, phase)** | the **phase-local** verdict of that phase |
| **supporting** | file-novelty, hunk-overlap, path-restructure, blocked-classification, and the confirmation gate's `manual-review` entry | **at most three per phase in the shipped pipeline**; several may share one phase | **shipped semantics, unchanged** |

- **Verdict-bearing** entries are the ones a reader may treat as "this phase's
  answer". A second verdict-bearing entry for one phase in one invocation is a
  defect — which is why both authoritative arms **replace** the derived `:766`
  entry with their staged list rather than appending beside it (D2).
- **Supporting** entries are detector and gate observations, retained exactly
  as shipped: same phase, same `evidence_kind`, same `confidence`, same
  `raw_reconcile_verdict`, same `reason_code`, and the **same firing
  conditions**. This ADR makes no detector run on an outcome it does not run on
  today and suppresses none that it does. The confirmation gate's entry keeps
  `phase: phase-3.5` and `evidence_kind: manual-review` even when its subject
  is a phase-2 result, because that is what it means today and this ADR does
  not re-key it. No phase-2 proof authority flows through it — D2's authority
  travels in the typed proof parameter, not in an evidence line.
- The phase-local verdict rule is **scoped to verdict-bearing entries only**. A
  supporting entry's verdict field is not a phase verdict:
  `blocked-classification` writes `blocked` because that is its subject, and the
  gate writes `upstreamed` because that is the result it is grading.
- **A candidate-ready invocation returns `store.ReconcileStillNeeded`**, so
  path-restructure and blocked-classification correctly do not fire for it. The
  candidate path neither claims their support nor forces it into existence. Its
  own phase-4 finding is carried by its **second staged verdict-bearing
  attempt**, not by a supporting detector entry.

**Append order is deterministic.** The staged attempts are appended at the
shipped `:766` position, in slice order and ahead of every supporting entry;
then the shipped call order `:766` → `:771` is preserved exactly with every
call site's own firing condition unchanged, so two runs over the same inputs
produce the same lines in the same order.

**Exactly one verdict-bearing entry per phase, on every arm, over two routes.**
The GH #13 evaluator's phase-2 attempt reaches `reconcile-evidence.jsonl` by
one of two routes, decided by a single question: **would `evidencePhaseAndKind`
(`internal/workflow/reconcile.go:1161-1176`) derive `phase-2` from this
invocation's terminal `result.Phase`?**

- **Staged substitution** — yes, it would collide. That happens on exactly two
  arms, the only two whose terminal `result.Phase` takes the `phase-2` prefix
  branch (`:1167-1168`): the all-present return (`phase-2-operation-level`,
  `:451-457`) and the candidate-ready outcome
  (`phase-2-operation-candidate`). Both stage — D2's configurations 2 and 3 —
  so on both the derived entry is **suppressed** and the substitution is
  like-for-like: same call site, same position, one line per phase.
- **Direct append** — no, it cannot collide. On a phase-2 refusal or conflict
  the pipeline falls through, and every terminal arm downstream of phase 2 sets
  a phase-3, phase-3.5 or phase-4 `result.Phase`, so the derived entry is never
  `phase-2`. The evaluator appends its phase-2 verdict-bearing attempt directly
  at phase-2 time, `result.verdictAttempts` stays empty (D2 configuration 1),
  and the shipped derived builder produces the terminal entry verbatim. **No
  arm emits a staged attempt and a derived entry together**; the direct-append
  route is not staging and suppresses nothing.

**Both routes write both sinks, in one order.** A verdict-bearing attempt is
appended to `reconcile-evidence.jsonl` **and** to the in-memory
`result.Evidence` slice, in the same relative order, by whichever route carries
it. Staged substitution gets this from the shipped shape: `:766` is
`result.Evidence = append(result.Evidence, persistReconcileEvidence(...)...)`,
and the substituted function both calls `store.AppendReconcileEvidence`
(`:876`) and returns the entries it appended (`:880`), so the returned slice is
appended to `result.Evidence` in slice order. The **direct-append** route must
do the same explicitly: the evaluator appends its phase-2 entry to the JSONL
and to `result.Evidence` in one step, at phase-2 time, before the pipeline
falls through. Since `ReconcileResult` is serialized whole into
`reconcile-session.json` (`:775-776`), skipping the `result.Evidence` half
would make the session file's `evidence` array a strict subset of the JSONL —
a divergence between two artifacts that are contractually the same ordered
sequence. Neither route may write one sink without the other, and neither may
write them in different orders.

| Arm | Staged attempts | `:766` derives | Verdict-bearing lines by phase |
|---|---|---|---|
| GH #13 all-present, proof minted | **1** — the qualifying phase-2 entry | `phase-2` — **suppressed**, replaced by the staged list | `phase-2`: 1 |
| legacy all-present with `operation_candidate_enabled` **off** (D12b) | 0 | `phase-2`, legacy builder verbatim (`:826,829,843-845`) | `phase-2`: 1 |
| candidate-ready, returning `still_needed` | **2** — phase-2 then phase-4 | `phase-2`, from `phase-2-operation-candidate` — **suppressed**, replaced by the staged list | `phase-2`: 1, `phase-4`: 1 |
| phase-2 refusal or conflict — including, with the gate **on**, every legacy or ineligible all-present result — a later phase answers | 0 | that later phase's entry, unchanged | `phase-2`: 1, appended directly by the evaluator; that later phase: 1 |

**Row 2 is the gate-off arm only.** The legacy derived `phase-2` entry survives
exactly where `result.Phase` is a `phase-2` value and nothing was staged, and
with `operation_candidate_enabled` **on** that never happens: D12b removes the
shipped `opResult.allPresent` early return (`:451-457`) from the authoritative
path, so a legacy or E-gate-ineligible all-present result falls through with
its phase-2 refusal evidence and earns a later phase's outcome — row 4's
**direct-append** route, not row 2's. Row 2 is the byte-identical shipped
behavior the S0 goldens pin, reachable only with the gate off.

A staged list that is not one of D2's three legal configurations — a `phase-2`
staged attempt duplicated, a derived entry appended beside a staged one, a
proof beside the two-attempt list, the two attempts out of phase order, or two
staged entries sharing an `attempt_id` — is a defect and is rejected by D2's
source guard and by `saveReconcileArtifacts`'s configuration check.


**Each verdict-bearing attempt's `raw_reconcile_verdict` is that phase's own
verdict** —
`upstreamed`, `still_needed` or `blocked` — and it may differ from the
invocation's returned outcome. A phase-2 evaluation that proved a candidate
writes `still_needed` on its own entry even when the invocation ultimately
returns `reapplied`; a phase-2 conflict writes `blocked` on its own entry even
when phase 4 later reapplies cleanly. The terminal phase always gets its own,
separate entry with its own verdict. `RawReconcileVerdict` is already an
unconstrained string on the shipped schema
(`internal/store/reconcile_evidence.go:96`), so this needs **no** schema
change.

**CLI display.** `tpatch reconcile review <slug>`
(`internal/cli/cobra.go:2544`) renders every attempt of the latest invocation
in phase order, labelling each verdict-bearing `raw_reconcile_verdict` as that
attempt's **phase-local verdict**, labelling supporting entries as supporting
evidence rather than verdicts, and printing the invocation's **returned
outcome** separately from `status.json:reconcile.outcome`. A rendering that
presents a phase-local verdict as the command's answer, or a supporting entry's
verdict field as a phase verdict, fails the contract.

**Migration is honest.** The new authoritative phase-2 entry carries different
field values from anything a pre-GH-#13 run produced, so it **intentionally
mints a new `attempt_id`**. No byte identity with historical entries is claimed
anywhere. Idempotency is ADR-025 D3's, unchanged: `attempt_id` is
content-addressed, an identical existing ID is a **no-op skip**
(`internal/store/reconcile_evidence.go:166-170`), and the same ID with
different bytes is corruption and refuses with `ErrMalformedEvidence`
(`:171`). That is what makes appending two entries safe across reruns.

**Rationale.** A blocked phase 2 followed by a clean phase 4 is the single most
useful audit trail this system can produce, and today it is discarded. Making
each verdict-bearing attempt carry its *own* verdict rather than the
invocation's is what makes the trail readable: an entry that says `blocked`
beside an overall `reapplied` is the finding, not a contradiction. The
two-class split exists because the alternative — bounding *every* entry at one
per phase — would have required deleting three shipped detectors or re-keying
them onto invented phases, which is a schema change dressed as a cardinality
rule.

**Alternatives rejected.** One attempt per phase, full stop (rev-1): it
contradicts four shipped call sites and would force either detector deletion or
a phase-enum extension. Re-keying the confirmation gate's entry to `phase-2`
when its subject is a phase-2 result: it would change shipped evidence bytes for
every existing gate rejection, and the gate genuinely runs after the pipeline,
not inside phase 2. Adding a `class` field to the evidence schema: the split is
derivable from `evidence_kind`, which is already closed, so a new field would be
redundant state that could disagree with itself.

### D26 - Candidate diagnostics are at least as rich as the phase-4 path, measured in semantic fields

`candidate.json`'s `effect_map` carries per-effect resolution and reason codes
(D20); the phase-2 verdict-bearing entry carries the derivation summary; and
`candidate.patch` is a reviewable diff. Together, a terminal candidate-ready
result must be **at least as diagnostic-rich** as the phase-4 blocked path an
operator would otherwise have reached.

**The comparison is over semantic fields, not evidence-line counts.**

| What an operator can learn | Phase-4 blocked path | Candidate-ready path |
|---|---|---|
| which files are involved | file-novelty and hunk-overlap (`internal/workflow/reconcile.go:767-768`) | the same two entries, unchanged, plus an `effect_map` naming **every** effect and path |
| what happened to each change | not answered per change | `resolution` per effect: `absorbed-present` or `candidate-applicable` |
| why something did not work | one invocation-level `blocked-classification` reason (`:770`) | per-effect `reason_codes`, plus the phase-2 entry's own `reason_code`, `confidence` and counts |
| where the change would land | not answered | `candidate.patch` and the derived operation list |
| what the answer is | one `blocked` verdict | a `still_needed` verdict plus an artifact that can be diffed, accepted or rejected |

**No equality of detector-entry counts is claimed, and none should be.**
path-restructure (`:947-949`) and blocked-classification (`:998-1000`) are
blocked-only, and a candidate-ready invocation is not blocked (D25). Matching
their line count would mean running a blocked-outcome detector on a non-blocked
outcome — a change to shipped detector semantics for every feature in the
repository, made to satisfy a metric nobody reads. "Phase 2 produced a
candidate" without the per-effect mapping still does not satisfy this decision.

**Rationale.** The candidate path *replaces* the phase-4 blocked verdict for
the features it terminates. Terminating with less information would be a net
regression for those features — but "less information" means fewer answered
questions, not fewer JSONL lines, and rev-2's phrasing invited the wrong
measurement.

### D27 - Acceptance is a new path; `AcceptShadow` is never reused

`AcceptOperationCandidate` is new. **`AcceptShadow`
(`internal/workflow/accept.go:82`), `CopyShadowToReal`
(`internal/gitutil/shadow.go:191`) and `RefreshAfterAccept`
(`internal/workflow/refresh.go:45`) are never called on a candidate path.**
The reasons are specific and each is disqualifying on its own:

| Primitive | Disqualifying behavior |
|---|---|
| `AcceptShadow` step 1 | re-applies the **stale original patch** minus resolved paths (`gitutil.ForwardApplyExcluding(s.Root, originalPatch, files)`, `internal/workflow/accept.go:95`). A candidate's entire premise is that the original patch no longer applies at this upstream |
| `AcceptShadow` step 2 | `CopyShadowToReal` is **file-only** (`internal/gitutil/shadow.go:191`) and its own doc concedes a caller must wrap it for atomicity (`:189-191`) |
| `AcceptShadow` overall | **not atomic** — a mid-flight failure returns an error with the shadow still in place and no rollback (`internal/workflow/accept.go:95-131`) |
| `AcceptShadow` scope | **partial** — it accepts only the resolver's `files` list, not the feature's full effect set |
| `RefreshAfterAccept` | writes `post-apply.patch` unconditionally (`internal/workflow/refresh.go:82`) and **deliberately leaves `apply-recipe.json` stale** (`:20-24`) — coverage after it is incomplete by construction (ADR-036 D15 P3) |

`AcceptOperationCandidate` instead revalidates the metadata and every binding,
**re-derives** the alignment and candidate bytes from scratch, re-runs D14 and
D15, runs the D17 test command in the candidate worktree, and requires the
recomputed identity tuple, candidate diff hash and candidate tree hash to equal
the stored ones before the first live write.

**Rationale.** Every one of the five rows above is a property the resolver was
allowed to have because a human reviewed a per-file diff. A candidate's
guarantee is machine-derived, so it must be machine-provable end to end.

### D28 - Acceptance preconditions: portable advisory locks, HEAD ancestry, and a twice-run per-path CAS

#### Locks

**`internal/rescap`'s lock is not reused and its contract is not
re-specified.** That lock is a real nonblocking `flock(2)`
(`internal/rescap/lock_unix.go:63,80-87`) with no PID protocol — rev-0's
"stale lock is one whose owning PID is gone" described a protocol that lock
does not implement — but it is bound to the resource-capture surface: a
`statfs` filesystem preflight (`:64-70`), a scratch root, callers that must
first pass `rescap.EnsureLocalContract` (`internal/rescap/scratch.go:46-48`,
`internal/cli/feature_resource.go:107`), and an outright refusal on every
non-linux/darwin target (`internal/rescap/lock_unsupported.go:1,15,29-31`).

A **new** internal advisory-lock abstraction is added:

```go
const Supported bool                                   // build-target constant
func TryAcquire(path string) (*Handle, error)          // nonblocking, no wait, no retry
func (h *Handle) Release() error                       // closes the descriptor
```

**Runtime-supported targets are `linux`, `darwin` and `windows` in v1 — and
nothing else.**

| Build target | v1 status | Primitive |
|---|---|---|
| `linux`, `darwin` | **runtime-supported**, CI-covered | `flock(2)` with `LOCK_EX \| LOCK_NB` |
| `windows` | **runtime-supported**, CI-covered | `LockFileEx` with `LOCKFILE_EXCLUSIVE_LOCK \| LOCKFILE_FAIL_IMMEDIATELY` |
| `freebsd`, `openbsd`, `netbsd`, `dragonfly`, and every other target | **unsupported stub** | `Supported == false`; `TryAcquire` refuses without touching the filesystem |

rev-1 listed the BSDs as supported. They are not, and the reason is stated
rather than assumed: **a target that compiles is not a target that is
validated.** The repository's CI runs Linux, macOS and Windows
(`AGENTS.md` wave-close gate, three platforms), so those three are the only
targets where the primitive's nonblocking, crash-release and
same-process-reacquire behavior is actually exercised. A BSD build would
compile a `flock` path nobody has run, and a lock that has never been run is
not a lock. The BSDs move out of the stub when — and only when — a native
runtime CI job exists for the target **and** the primitive is verified on it;
until then they degrade exactly like any other unsupported target: phase 2
records `lock-unsupported` and falls through, and `--accept-candidate` refuses
with the same code and exit `3`. §11 carries this as an explicit deferral.

**Windows uses stdlib `syscall`, and adds no dependency.** `LockFileEx` is
resolved through `syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")` in
a `//go:build windows` file, with `syscall.Handle` from the opened `*os.File`.
`golang.org/x/sys/windows` is **not** added: `go.mod` stays at
`github.com/spf13/cobra` plus its `pflag` and `mousetrap` indirects, which is
the minimal-dependency rule this repository already enforces. An implementation
that introduces `x/sys` for this fails the dependency guard.

**The lock refuses on filesystems where it does not mean anything.** The
constructor performs **its own** filesystem-class check before acquiring, and
refuses a known network or remote filesystem with `lock-filesystem-unsupported`:

| Target | Check | Refuses on |
|---|---|---|
| `linux` | `statfs(2)` / `fstatfs(2)` on the lock file's directory, reading `f_type` | NFS, SMB/CIFS, AFS, FUSE network classes, 9P, and any `f_type` not on the known-local allowlist |
| `darwin` | `statfs(2)`, reading `f_fstypename` and the `MNT_LOCAL` flag | any mount that is not `MNT_LOCAL`, and any `f_fstypename` not on the known-local allowlist |
| `windows` | `GetDriveTypeW` on the volume root, plus UNC-path detection | `DRIVE_REMOTE`, `DRIVE_UNKNOWN`, `DRIVE_NO_ROOT_DIR`, and any UNC path |

**Unknown classes refuse.** The check is an allowlist of known-local
filesystems, not a denylist of known-remote ones, so an unrecognized `f_type`,
an unrecognized `f_fstypename` or `DRIVE_UNKNOWN` refuses rather than
proceeding. `lock-filesystem-unsupported` is a **nonterminal** phase-2 refusal
during generation and a state refusal with exit `3` on a terminal candidate
action, exactly like `lock-unsupported`.

**This is not `rescap`'s contract, and no cross-host safety is claimed.** The
check shares a *technique* with `internal/rescap/statfs_linux.go:78-95` and
nothing else: there is no `.tpatch/local` directory, no
`rescap.EnsureLocalContract` precondition (`internal/rescap/scratch.go:46-48`),
no ignore-file requirement, no scratch root and no
`ReasonResourceLockFSUnsupported` reuse. And the check is a **narrowing**, not
a safety proof — `flock` on a local filesystem is a single-host mutex, and this
document does not claim that the lock alone makes anything safe across hosts.
**The authoritative defense against a concurrent or cross-host mutation is the
per-path compare-and-swap and the acceptance journal (§6.14)**, both of which
are content comparisons against named trees and are indifferent to which host
or which process moved the bytes. The lock exists to make same-host contention
explicit and fast to refuse; the CAS exists to make correctness independent of
it.

Properties, each stated because rev-0 got one of them wrong:

- **No PID metadata is written and none is read.** The lock is a kernel-tracked
  association between one open file description and one inode. There is no
  owner record, no heartbeat and no identity to go stale.
- **There is no stale-lock breaking, at any age, by any flag.** The kernel
  releases the lock when the last descriptor closes — including on `SIGKILL`,
  crash and power loss — so a "stale" lock cannot exist. rev-0's
  `candidate-lock-stale-broken` diagnostic is deleted.
  `--allow-stale-lock` (`internal/cli/cobra.go:2524`) governs ADR-017's
  baseline classifier (`internal/gitutil/lock_guard.go:18-23,99-100`) and does
  **not** apply here.
- **Lock-file residue is inert.** An empty `.tpatch/locks/**.lock` left behind
  is never read, never parsed and never a signal.
- **Paths are explicit and safe:** `.tpatch/locks/reconcile/<slug>.lock` and
  `.tpatch/locks/worktree-mutation.lock`, opened `O_CREATE|O_RDWR 0600` under
  the store root after `safety.EnsureSafeRepoPath`. There is no `.tpatch/local`
  directory, no `EnsureLocalContract` precondition and no ignore-file contract;
  the only preflight is this abstraction's **own** filesystem-class refusal
  above, which is not `rescap`'s and imports none of its reason codes.
- **Contention refuses immediately** with `candidate-lock-held` and exit `3` —
  no wait, no retry, no timeout.
- **Acquisition order is fixed — per-feature lock, then global
  worktree-mutation lock — for consistent, explainable behavior, not for
  deadlock prevention.** A nonblocking lock cannot deadlock. rev-0 justified
  the order as deadlock avoidance, which is not a property this primitive
  needs. If the second acquisition fails, the first is **released** before the
  refusal returns, so a failed attempt leaves nothing held.
- **Scope.** Candidate derivation holds neither lock, because it mutates
  nothing. D21's preflight takes the per-feature lock for its inspect-and-prune
  window and releases it before phase 1. Finalization (D13) **reacquires** the
  per-feature lock and **revalidates**, and takes the global lock only around
  `git worktree add` / `git worktree remove`. Acceptance holds both for the
  whole transaction. Rejection (D23) and recovery (D29b) hold the per-feature
  lock. **No candidate-state mutation happens outside a hold** (D21).
- **Unsupported platforms are named, not silent.** With `Supported == false`,
  candidate finalization records a **nonterminal** phase-2 refusal
  `lock-unsupported` and falls through, so reconcile behaves exactly as it does
  today on that platform; `--accept-candidate` refuses with the same named code
  and exit `3`.

#### HEAD ancestry, and the CAS that is the actual authority

rev-0 required `git rev-parse HEAD == upstream_commit`, which is unreachable
whenever an operator's branch carries any commit of its own — which is the
normal case for the only workflow this tool has.

- **Feature state.** The feature's current `status.State` must equal the
  candidate's bound `source_feature_state` **and** must still be in D7b's
  permitted set. Any drift refuses `candidate-feature-state-changed` with both
  states named and exit `3`, before the journal, the snapshots and any live
  write.
- **Ancestry.** `upstream_commit` must be HEAD **or an ancestor of** HEAD
  (`git merge-base --is-ancestor <upstream_commit> HEAD`). Otherwise refuse
  `head-diverged` with both commits named and exit `2`. This is an ordering
  diagnostic, not the safety property.
- **Exact per-path compare-and-swap, which *is* the acceptance authority.** For
  **every** path the candidate touches, the live tree must match the
  `upstreamCommit` tree exactly: a modified path must exist live with
  **byte-equal** content and the same mode as that tree's blob; a created path
  must be **absent** live — an untracked file there refuses with
  `candidate-path-occupied`, because writing it would destroy unversioned work.
  Any mismatch is `candidate-cas-failed`, exit `2`.
- **The CAS runs twice**: at step 6, and again inside step 10 immediately
  before the first live write, over the same path set with the same comparison
  and the same `candidate-cas-failed` refusal (D29).
- **Unrelated dirt outside the candidate path set is allowed.** An operator
  with edits elsewhere is not blocked.

Among the preconditions the CAS is decisive: no earlier check overrides it, no
ancestry result substitutes for it, and no "close enough" tolerance exists.
What it is **not** is a mutual exclusion, and rev-3 states the boundary rather
than leaving "last word" to be read as one:

- the advisory locks serialize **`tpatch` invocations on a single host**. They
  do not constrain an editor, a build tool, a second checkout, a CI runner or a
  process on another host;
- **no claim is made that a non-`tpatch` writer — local or cross-host — is
  prevented from mutating a candidate path after the step-10 recheck.** The
  recheck shrinks that window to the smallest this design can offer; it does
  not close it;
- the answer to that residual case is **recovery, not prevention**: the
  journal and snapshot rollback (D29, D29b) leave a deterministic, explicitly
  resumable state rather than a half-written tree.

**Rationale.** The CAS is what makes "no lock held during review" tolerable:
the candidate can sit pending for days, and acceptance still refuses the moment
any byte it depends on moved — twice, the second time with nothing between the
check and the write. Ancestry is kept only so a diverged branch gets a clear,
early, well-named refusal instead of a per-path CAS failure that reads like
corruption.

**Alternatives rejected.** HEAD equality (rev-0): unreachable in practice, and
strictly weaker than the CAS anyway — HEAD equality does not prove any
individual path still matches, because the tree may be dirty. Reusing
`rescap.AcquireLock`: it would drag the `statfs` preflight, the scratch-root
layout and the `.tpatch/local` contract into reconcile, and would refuse
outright on Windows. A PID-and-rename lock protocol: it has the ABA and
stale-owner problems the kernel lock exists to avoid, and rev-0 attributed
those semantics to a primitive that does not have them. Claiming BSD support on
the strength of a `flock` build tag (rev-1): the BSDs have no CI job here, and a
lock nobody has ever run is not a lock — the honest position is the stub and a
named deferral. Omitting the filesystem-class check because "the CAS is the real
defense" (rev-1): the CAS *is* the real defense, but silently acquiring a lock
that provides no exclusion on NFS invites an operator to believe contention was
checked when it was not; refusing by name is the difference between a
limitation and a trap. Importing `golang.org/x/sys/windows` for `LockFileEx`:
`syscall.NewLazyDLL` is in the standard library and this repository's
minimal-dependency rule is older than this feature.

### D29 - Acceptance is a journaled fourteen-step transaction with rollback

The v1 candidate domain only creates and modifies `100644` files, which is what
makes a snapshot-and-restore journal sufficient.

**This step list is reproduced byte-identically in the companion PRD §6.14.**

<!-- parity-block:E:begin -->
```
 1. Acquire the per-feature reconcile lock, then the global worktree-mutation
    lock, in that fixed order.
 2. Detect any acceptance journal for this slug; a journal present is
    `recovery-required` and the transaction does not start.
 3. Load and strict-decode `candidate.json` and `state.json`; require
    `state.status == pending`, and require the feature's current
    `status.State` to equal the bound `source_feature_state` and to still be a
    permitted state.
 4. Re-run the E1-E15 eligibility gate and re-derive every operation, the
    candidate bytes and the candidate ID from the bound inputs; require the
    recomputed identity tuple, candidate diff hash and candidate tree hash to
    equal the stored values.
 5. Re-materialize the candidate worktree, re-apply the complete operation set
    all-or-nothing, re-run the postcondition and idempotency proofs, and run
    the configured `test_command` in that worktree.
 6. Validate that `upstream_commit` is HEAD or an ancestor of HEAD, then
    validate the exact per-path compare-and-swap on every candidate path.
 7. Precompute and stage into `staged/` the complete residual patch, the new
    recipe, the recipe provenance, the ADR-024 generation entry carrying
    `base_commit = upstream_commit`, and the complete coverage record whose
    `reference.commit` and `preimage_set_sha256` derive from that same commit.
 8. Snapshot into `snapshots/` every live candidate path and every feature
    artifact this transaction will replace — `artifacts/post-apply.patch`,
    `artifacts/apply-recipe.json`, `artifacts/recipe-provenance.json`,
    `artifacts/recipe-coverage.json`, `artifacts/patch-generations.json` and
    `status.json` — plus the pre-transaction `status.Apply` and `status.State`
    values, writing an explicit absent marker for every path the candidate
    creates and for every one of those artifacts that does not yet exist.
 9. Write `accept-journal.json` with the prepared step list, and fsync the
    journal and its directory.
10. Re-validate the exact per-path compare-and-swap on every candidate path,
    refusing `candidate-cas-failed` on any mismatch; then, with nothing else
    between, perform the first live write: apply the candidate's create and
    modify operations to the live worktree.
11. Publish the staged feature artifacts, with the coverage record written
    last.
12. Write `state.json` with `status: accepted`, set `status.Apply.BaseCommit`
    to `upstream_commit` with the matching completion fields, update the
    `status.json` pointer and mark the feature applied.
13. Write the journal's commit marker and fsync it.
14. Clean up: prune the candidate worktree, clear the pointer, and remove
    `snapshots/`, `staged/` and the journal.
```
<!-- parity-block:E:end -->

**Step 10 is the first live write, everywhere.** No live repository path is
written before step 10's write begins — in this ADR, in the companion PRD, or
in the implementation. rev-0's two documents said "before step 4" and "before
step 5", which contradicted each other and both of their own lists.

**Step 10's leading compare-and-swap is a read-only precondition inside the
step**, not a fifteenth step and not a write. It re-proves, with nothing
between it and the mutation, the same per-path equality step 6 proved, and it
refuses with the same `candidate-cas-failed` and the same exit `2`. A refusal
there leaves the journal with no commit marker, so it is recoverable exactly as
any other pre-marker failure (D29b). The step count stays fourteen precisely so
the two documents keep one canonical numbering.

**Step 12's base metadata is the contract, not an implementation detail.**
`status.Apply` (`internal/store/types.go:362-369`) is rewritten so that the
feature's recorded base is the commit its new canonical patch is actually a
diff against:

| Field | Value after a successful acceptance |
|---|---|
| `Apply.BaseCommit` | `upstream_commit` — the commit the CAS was validated against |
| `Apply.CompletedAt` | the acceptance timestamp |
| `Apply.HasPatch` | `true` — the residual patch is published at step 11 |
| `Apply.HasRecipe` | `true` — the regenerated recipe is published at step 11 |
| `status.State` | `store.StateApplied` (`internal/store/types.go:13`) |

The staged ADR-024 generation entry (step 7) carries the **same**
`base_commit`, with `capture.mode: reconcile` and an `upper` consistent with
the reconcile capture, so the manifest and `status.Apply` cannot disagree about
which base this feature is now recorded against. `AppendPatchGenerationForFeature`
(`internal/workflow/patch_generations.go:31`) is the shared appender and takes
`BaseCommit` as an input field, so no new writer is introduced.

**Six shipped readers see the new base on the next reconcile, and that is
intended.** `status.Apply.BaseCommit` is read by `persistReconcileEvidence`
(`internal/workflow/reconcile.go:821`), the file-novelty detector (`:888,899,903`),
the hunk-overlap detector (`:917,924,931,935`), the path-restructure detector
(`:951,965,978`), the blocked-classification taxonomy (`:1005`) and the
confirmation gate itself (`:1037`). Leaving a stale `BaseCommit` behind would
make every one of them classify the next reconcile against a base the feature
no longer has — novelty and overlap would be computed against the pre-accept
tree, and the confirmation gate would stamp a base commit that does not match
its own evidence. Updating it at step 12 is what keeps those six readers
truthful, and §9.11 pins each of them.

**Failure before the step-13 commit marker rolls back both sides.** Every live
candidate path and every feature artifact in step 8's snapshot set —
`artifacts/post-apply.patch`, `artifacts/apply-recipe.json`,
`artifacts/recipe-provenance.json`, `artifacts/recipe-coverage.json`,
`artifacts/patch-generations.json` and `status.json` — is restored from
`snapshots/`, the snapshotted `status.Apply` and `status.State` are restored
with them, and every path recorded with an **absent marker is deleted** —
without the marker, a rollback would leave a created file behind and call it
success. The candidate returns to `status: pending`, with its `status.json`
pointer restored.

**A failure at step 11 or step 12 is a failure, not a warning.** A
coverage-publication failure, a staged-artifact write failure or a
state-transition failure rolls back and returns non-zero. There is no
success-shaped partial acceptance; `AcceptShadow`'s best-effort treatment of
refresh and prune (`internal/workflow/accept.go:110,128-131`) is explicitly
**not** the model.

**Exit codes are tight.** A completed rollback is a state refusal, not an
internal error: `acceptance-rolled-back` and `coverage-publication-failed` exit
`3` under ADR-031 D4's shipped envelope (`internal/cli/reject.go:36-47`), with
the failing step named. Only a **failed rollback** escalates to exit `1`
(`acceptance-rollback-failed`), and it retains the journal so every subsequent
command is `recovery-required`.

**No filesystem-wide atomic-rename claim is made, anywhere.** A multi-file live
mutation is not atomic on any filesystem this tool targets. The guarantee is
"journaled, rollback-capable, and explicitly recoverable", and every surface
says exactly that.

**Rationale.** `AcceptShadow` currently has neither rollback nor recovery
(`internal/workflow/accept.go:95-131`), which is survivable for a
human-reviewed per-file copy and not survivable for a machine-derived
whole-feature transition. Publishing one canonical numbered list in both
documents is what makes "which step failed" a checkable statement.

### D29b - Recovery is detected before phase 1 and resumed only explicitly

A crash between step 9 and step 14 leaves `accept-journal.json`.

**Detection is universal and early.** D21's preflight runs before phase 1 for
every reconcile invocation naming the slug, and before any candidate or
resolver action for it. A journal present refuses the command with
`recovery-required` and exit `3`, and does nothing else — no phase runs, no
staleness pruning, no evidence append.

**`--resume-candidate-recovery <slug>` is the only way out**, and it does not
guess:

| Journal state | Action |
|---|---|
| commit marker **absent** | deterministically roll back from `snapshots/`: restore every snapshotted live path and every snapshotted artifact — `artifacts/post-apply.patch`, `artifacts/apply-recipe.json`, `artifacts/recipe-provenance.json`, `artifacts/recipe-coverage.json`, `artifacts/patch-generations.json` and `status.json`, together with the recorded `status.Apply` and `status.State` — delete every path recorded with an absent marker, set `state.json` back to `pending`, restore the `status.json` candidate pointer, and remove the journal |
| commit marker **present** | the transaction is committed; complete step 14's cleanup — prune the worktree, clear the pointer, remove `snapshots/`, `staged/` and the journal |

There is no implicit recovery-on-next-run, no heuristic, no partial replay and
no inference from timestamps or file presence beyond the marker itself. A
resume that cannot complete leaves the journal in place and exits `1`.

**Rationale.** The commit marker is the only bit that distinguishes "the live
tree still needs restoring" from "the live tree is already correct and only
bookkeeping is left". Guessing that bit from file presence would risk undoing a
completed acceptance.

### D30 - Successful acceptance publishes complete coverage; ADR-036 D15 P3 restated in full

Acceptance publishes coverage through **ADR-036 D15's single shared publication
API**. **This restates ADR-036 D15's producer P3 in full as two variants over
one enum value and one schema version**, and is reproduced byte-identically in
the companion PRD §6.16.

<!-- parity-block:F:begin -->
| Variant | Trigger | Writes | Coverage |
|---|---|---|---|
| `resolver-accept` | `AcceptShadow` → `RefreshAfterAccept` | `artifacts/post-apply.patch` unconditionally (`internal/workflow/refresh.go:82`); **no** recipe regeneration, by design (`:20-24`); an ADR-024 generation entry only when the patch bytes changed (`:93,102`) | **incomplete**, carrying `producer-patch-rewrite` **and** `recipe-not-regenerated` — unchanged from ADR-036 D15 |
| `operation-candidate-accept` | `AcceptOperationCandidate` | the **residual** canonical patch, a regenerated `apply-recipe.json`, truthful `recipe-provenance.json`, an ADR-024 generation entry, and the coverage record — all derived together from one bound snapshot and staged before the first live write | **complete**: no `operation-missing`, no outstanding reason, every effect `represented` |
<!-- parity-block:F:end -->

Both variants use the **same** `producer: reconcile-accept` enum value and the
**same** `capture.mode: reconcile`. The `operation-candidate-accept` variant
sets `reference.kind: commit` and `reference.commit` to the **accepted upstream
commit** — the `upstream_commit` the CAS was validated against — and binds the
**residual** patch, which is the `upstreamCommit` → candidate-result diff
restricted to the candidate path set. `preimage_set_sha256` recomputes over
that same commit's tree, and the ADR-024 generation entry staged beside it
carries `base_commit = upstream_commit`, so `status.Apply.BaseCommit` (D29 step
12), the generation manifest and the coverage reference all name one commit.

**No `producer` enum value is added and `schema_version` stays `1`.** The
variant is a property of the input to the shared API, exactly as ADR-036 D15
requires ("policy differences … are expressed as different inputs to the same
API").

**This discharges one part of one ADR-036 deferral row, and the rest of that
row's context is quoted rather than paraphrased.** The ADR-036 row reads
"Recipe regeneration for `reconcile-accept`, `cycle` and `apply --mode done` …
Reopen when: GH #13 regenerates recipe and coverage from accepted operation
candidates" (`docs/adrs/ADR-036-recipe-coverage-authority.md:3048`). GH #15's
PRD assigns the ownership in full
(`docs/prds/PRD-recipe-generation-authority.md:3674-3678`):

> GH #13 derives its own ephemeral anchors and independently proves uniqueness,
> postcondition identity and idempotent reclassification for each one. It also
> owns regenerating recipe and coverage for features that P3, P4 or P5 left
> explicitly incomplete, and for features whose only record came from P6
> (`canonical-patch-missing`) or P7 (`manual-bound-artifact-edit`) (§6.15).

rev-1 quoted only the "P3, P4 or P5" clause and dropped P6 and P7, which
overstated what D30 closes. The honest accounting, against ADR-036 D15's
seven producers (`docs/adrs/ADR-036-recipe-coverage-authority.md:1960-1966`):

| Producer | GH #15 assigns regeneration to GH #13 | What GH #13 v1 actually does |
|---|---|---|
| P3 `reconcile-accept` (`:1962`) | yes | **discharged for the `operation-candidate-accept` variant only.** The `resolver-accept` variant stays `incomplete` by design |
| P4 `cycle` (`:1963`) | yes | **not addressed.** No gate, no publication path, no matrix row; the deferral table below keeps it open |
| P5 `apply --mode done` (`:1964`) | yes | **not addressed.** Same as P4 |
| P6 `implement` (`:1965`), whose incomplete records carry `canonical-patch-missing` | yes | **not addressed.** Such a record is `incomplete`, so E3 refuses it and the feature falls through |
| P7 `artifact-edit` (`:1966`), whose records carry `manual-bound-artifact-edit` | yes | **not addressed.** Same as P6 |

**GH #13 v1 regenerates coverage in exactly one circumstance: a successful
operation-candidate acceptance.** It is a *by-product* of a proved, accepted,
human-authorized rewrite — never a repair pass. There is no code path in this
document that regenerates a recipe, a patch or a coverage record for a feature
whose inputs are ineligible, and there is deliberately no auto-repair: an
`incomplete` record from P4, P5, P6 or P7 refuses at E3 and the feature falls
through with today's behavior. Closing the remaining four fifths of the
ownership statement needs a regeneration contract of its own, which the
deferral table below carries with its reopen trigger.

**The new canonical patch is a residual.** It excludes effects already absorbed
upstream (D16), because those bytes are no longer this feature's diff against
this upstream. The new recipe and coverage explain that residual
**completely**.

Acceptance also marks the feature applied (`store.StateApplied`,
`internal/store/types.go:13`), rewrites `status.Apply` per D29 step 12, and
records the accepted candidate ID.

**Rationale.** An accept that leaves incomplete coverage behind would make the
feature ineligible for the next candidate — the feature would degrade with each
successful reconcile, which is the opposite of the intent. Stating **both**
variants, including the resolver variant's exact write set and its conditional
generation append, is what stops a future reader inferring that the resolver
path changed.

### D31 - Coverage, artifact and state failure is never warning-only

A failure to publish coverage, to write a staged artifact or to record the
state transition is **returned to the caller and exits non-zero**. It is never
a printed warning beside a success line.

There is **no success-shaped partial acceptance**. Either the transaction
completes and the surface says so, or it rolls back (or enters
`recovery-required`) and the surface says that, with the failing step named and
the exit code from D29's tight mapping. `AcceptShadow`'s current best-effort
treatment of refresh and prune (`internal/workflow/accept.go:110`, `:128-131`)
is explicitly **not** the model.

**Rationale.** ADR-036 D10/D15 already require publication failure to be
surfaced non-zero on every producer event
(`docs/prds/PRD-recipe-generation-authority.md:2691-2697`). Acceptance is the
highest-stakes producer event in the system; it gets the same rule, stated.

### D32 - Concurrency, stack and supersession scope

- **Lock order is deterministic** (feature, then global) and the locks carry no
  ownership metadata; there is no stale-lock breaking (D28).
- **Every candidate-state mutation is under the per-feature lock**, including
  D21's staleness preflight before phase 1; derivation holds nothing and
  finalization reacquires and revalidates (D21, D28).
- **Runtime lock support is linux, darwin and windows only** in v1, and the
  lock refuses a known network or unrecognized filesystem class by name (D28).
  Neither refusal changes any other reconcile behavior.
- **Feature state is bound and re-checked** (D7b): derivation is permitted only
  from `applied` or `blocked`, and acceptance refuses
  `candidate-feature-state-changed` on any drift.
- **The accept-time CAS is the decisive precondition**, run at step 6 and again
  immediately before the first live write: no earlier check overrides it, no
  ancestry result substitutes for it and no "close enough" tolerance exists. It
  is a proof that the world matched when it ran, **not** a mutual exclusion —
  the advisory locks serialize `tpatch` on one host, nothing claims to prevent
  an external or cross-host writer afterwards, and the journal plus rollback
  are the recovery answer (D28, D29).
- **Supersession never grants eligibility.** ADR-029 D7 downgrades superseded
  preimage drift to warning-class
  (`docs/adrs/ADR-029-write-file-recipe-safety.md:74-76`) and ADR-028 excludes
  superseded features from default effective replay
  (`docs/adrs/ADR-028-supersession-edge-model.md:77-88`). A downgraded warning
  is a severity statement; it is not authority, and a superseded feature does
  not become candidate-eligible because of it (E12).
- **A hard-parent-blocked child skips candidate derivation** entirely (E13),
  before any alignment work, and falls through. ADR-011's DAG semantics are
  unchanged and the shipped phase-3.5 blocked-by-parent short-circuit
  (`internal/workflow/reconcile.go:552-567`) is untouched.
- **Multi-slug and `--cumulative-legacy` fall through unchanged** (D6).

### D33 - Hard prohibitions

None of the following is permitted anywhere in this feature, in v1 or as an
implementation shortcut:

1. **No Git history rewrite.** No commit, amend, rebase, reset or ref move.
   Acceptance mutates the worktree and `.tpatch/`; committing stays the
   operator's.
2. **No arbitrary test execution by default.** Only explicit acceptance runs a
   configured `test_command`, and only in the candidate worktree (D17).
3. **No provider calls in phase 2.** Phase 2 is deterministic and offline.
   Provider assistance is phase 3/3.5's job and stays there.
4. **No delete or rename acceptance.** Outside the D7 domain, always.
5. **No phase 2.5 and no general search.** No new phase, no commutation
   engine, no fuzzy matching, no scoring function, no similarity threshold, no
   tie-breaking among optimal alignments.
6. **No network access and no lazy object fetch** (D5, D12).
7. **No tunable limit.** L1-L3 are compiled-in constants with no flag, config
   key or environment override (D9).
8. **No weakening of an existing gate.** Not ADR-036's decoder, not the
   confirmation gate's existing arms, not ADR-029 D7, not ADR-030.
9. **No serialized confirmation authority.** The D2 proof is an in-process
   typed value only; it is never written, never read back and never rebuilt
   from `confidence` or from an evidence line.
10. **No new module dependency.** The Windows lock uses stdlib `syscall`;
    `golang.org/x/sys` is not added (D28).
11. **No unlocked candidate-state mutation** (D21, D28).
12. **No auto-repair of ineligible coverage.** Regeneration happens only as a
    by-product of a successful operation-candidate acceptance (D30).
13. **No silent loss of a candidate pointer.** Every writer of
    `status.Reconcile` carries the loaded `operation_candidate` forward; only
    the five candidate transitions may change it (D20b).
14. **No detector re-tuning.** No shipped evidence detector's firing condition,
    phase or kind changes to make a candidate result look richer (D25, D26).

### D34 - Behavior changes are gated and disclosed

Three shipped behaviors change with this feature, and each is disclosed rather
than asserted away:

1. **The legacy all-present arm stops being authoritative** (D12b). Gated by
   `operation_candidate_enabled`, default `false` in v0.18.0. The affected
   existing expectation is named in D12b and carried as a second, explicit
   gate-on expectation.
2. **The confirmation gate gains an authority class** (D2). It is additive: no
   existing confirming input, rejection path or message changes. **Two
   unexported signatures change** — `applyUpstreamedConfirmationGate` and
   `saveReconcileArtifacts` each gain a fourth `proof *phase2AuthorityProof`
   parameter — a compile-time change with one call site
   (`internal/workflow/reconcile.go:771`) and **ten** call sites
   (`:383,432,455,480,524,536,564,570,583,596`) respectively, all ten passing
   `nil` — `:570` passes `phase35`, which **names the same `*ReconcileResult`
   object** as `result` rather than a second one, because `tryPhase35` (`:569`,
   defined `:1354-1364`) takes that result as its ninth parameter (`:1363`),
   mutates it in place and returns the same pointer from all ten of its returns
   (`:1373,1381,1389,1436,1475,1491,1497,1502,1508,1513`), and passes `nil`
   exactly like the other nine — and no
   behavioral effect when the parameter is `nil`. `ReconcileResult` also gains
   one unexported ordered `verdictAttempts []store.ReconcileEvidence` staging
   field (`:19-85`), which `encoding/json` ignores, so no serialized byte
   changes.
3. **`status.Reconcile` gains a carried-forward field** (D20b). With no
   candidate anywhere the `omitempty` pointer is absent and the serialization
   stays byte-identical; the shared merge changes no other field.

Everything else is byte-identical with the gate off, and byte-identical with
the gate on for any repository whose features have no eligible ADR-036
coverage — which is every repository until v0.17.0 ships and producers run.

**Rationale.** rev-0 claimed the all-present arm was unchanged while proposing
to change it, and claimed default byte identity for evidence entries that
necessarily mint new attempt IDs. A behavior change that is written down and
defaulted off is reviewable; one that is asserted not to exist is not.

### D35 - Implementation is blocked until v0.17.0 ships

No implementation slice may begin before GH #15 ships in v0.17.0. Today
`internal/` contains **zero** coverage code: `recipe-coverage.json`,
`recipe_coverage`, `CoverageProducer` and `reconcile-accept` have no
implementation-side match anywhere in `internal/` or `cmd/`. There is nothing
to consume.

The companion PRD's slice S0 freezes the shipped baseline **before** any
production change, and the slices run **sequentially**, not in parallel — the
whole feature converges on `internal/workflow/reconcile.go` and
`internal/cli/cobra.go`, which `AGENTS.md`'s Parallel-Implementer Discipline
makes a hard trigger for sequential execution.

### D36 - The two documents are parity-checked, not merely consistent, and the blocks are delimited

Six blocks are reproduced **byte-identically** in ADR-037 and the companion
PRD, and a planning parity check at implementation time asserts it: CG1-CG7 with
the `phase2AuthorityProof` carrier (D2 / §6.2), E1-E15 in their reachable order
(D4 / §6.5), A1-A8 with their first-match evaluation order and L1-L3 (D9 /
§6.7), the `candidate.json` / `state.json` schemas including
`source_feature_state` (D20 / §6.10), the fourteen transaction steps (D29 /
§6.14), and the ADR-036 D15 P3 two-variant table (D30 / §6.16).

**rev-3 delimits them.** Each region is wrapped, in both documents, in

```
<!-- parity-block:<ID>:begin -->
…the bytes under parity…
<!-- parity-block:<ID>:end -->
```

and the check extracts **only** the bytes between a matching pair. The rules
are mechanical: the IDs are exactly `A`, `B`, `C1`, `C2`, `D1`, `D2`, `D3`, `E`
and `F`; each appears once as a `begin` and once as an `end`, in that order, in
each document; and a missing marker, a duplicated ID, an unpaired marker, an ID
present in only one document, or any byte difference inside a pair fails the
check. The example above uses the literal placeholder `<ID>`, so it is not
itself a marked region: the parser recognizes a marker only when its ID matches
that closed nine-value set exactly, so it skips this illustration here and the
identical one in the companion PRD §6.0 rather than reading either as an
unpaired `begin`.

**Nine regions carry six blocks** because block C's rule set and its limit
table, and block D's two schemas and enum paragraph, are separated in both
documents by prose that is deliberately **not** identical — the ADR states its
own rationale there, the PRD its own product framing. An inferred boundary that
swallowed that prose would report a parity failure against a contract neither
document breaks, which is exactly the false positive that gets a guard
disabled.

**Rationale.** rev-0's two documents disagreed on the transaction's first live
write, on the flag count and on the terminal set, in each case because the same
contract was written twice in prose. Naming the blocks that must match makes
the divergence mechanically detectable instead of review-detectable; marking
their boundaries makes the check implementable without a heuristic.

## Consequences

### Positive

- The mixed phase-2 case stops being a discarded computation and becomes a
  reviewable, machine-proved artifact — and only where the shipped pipeline was
  going to print `blocked` anyway.
- Phase-2 verdicts become reproducible: two named trees in, the same answer
  out, on any machine at any time, with no tie-breaking and no tunable.
- The strongest phase-2 result can finally satisfy the confirmation gate, so an
  all-present feature can be retired deterministically instead of being demoted
  to `blocked` (D2).
- The motivating adjacent-CLI-args case is derivable (D9.1), which rev-0's
  algorithm could not do.
- `SPEC.md`'s phase-2 contract and the shipped code stop disagreeing.
- A blocked phase 2 no longer hides a clean phase-4 answer — both are recorded,
  each with its own phase-local verdict.
- Acceptance gains rollback and explicit crash recovery, which the resolver
  accept path does not have today, plus a per-path CAS that is reachable on a
  normal branch.
- A successful candidate accept leaves **complete** coverage, so a feature does
  not degrade with each reconcile.
- The typed-root rule with two named constructors makes "candidate code wrote
  the live tree" a compile error, and the guard's boundary is finite.
- The confirmation facts travel in a typed, unexported, in-process value along
  one fixed route — evaluator → `saveReconcileArtifacts` → gate — so "the gate
  confirmed on something it read" is a compile-time impossibility rather than a
  review question, and the entry it grades is named rather than implied (D2).
- A pending candidate stays addressable across every unrelated reconcile
  outcome, because one shared status merge carries its pointer forward and only
  five named transitions may change it (D20b).
- The locks work on Linux, macOS and Windows — the three targets CI actually
  runs — carry no protocol that can rot, and refuse by name where they would
  mean nothing.
- Every candidate-state mutation is inside a lock hold, so a concurrent
  acceptance and a stale-candidate prune cannot interleave (D21).
- Acceptance leaves `status.Apply.BaseCommit`, the ADR-024 manifest and the
  coverage reference naming one commit, so the six shipped base readers stay
  truthful on the next reconcile (D29).
- A candidate is bound to the feature state it was derived from, so a rewrite
  cannot land on a feature whose lifecycle decision was reversed while the
  candidate sat pending (D7b).
- ADR-036 D8's anchor deferral is discharged with a proof rather than a
  persisted claim.

### Costs

- Two new locks on a path that has none today, plus a new build-tagged
  primitive to maintain across linux, darwin, Windows and a stub — and a
  filesystem-class allowlist per supported target that will need extending as
  filesystems appear.
- The BSDs get no candidate path at all until someone stands up native runtime
  CI for them.
- An `O(|B|·|U|)` dynamic program per modified file, with hard refusal limits
  that will reject some genuinely large real files (D9 L1-L3) — and rev-2's
  `1048576`-cell L3 refuses considerably more of them than rev-1's would have,
  which is the price of a stated 8 MiB bound.
- The unanimity rule refuses some boundaries a human would call obvious,
  because some *other* optimal alignment disagrees.
- A new artifact namespace, two new strict schemas, a new ID scheme and a new
  append-only rejection log to maintain and to keep body-free.
- A candidate worktree per pending candidate — real disk, and real
  `git worktree list` noise.
- The mutual-exclusion rule between candidates and resolver shadows is a real
  workflow constraint operators will hit.
- Eight terminal actions on one command is a large surface; the exclusivity
  matrix grows quadratically, and **85** refusing pairs must be tested — and
  every future reconcile flag adds five more.
- Refusing explicit `--upstream-ref` beside a candidate action will surprise
  operators who habitually pass it, and the refusal message has to explain why.
- Binding `source_feature_state` into identity means a state change invalidates
  a pending candidate an operator may have already reviewed.
- The eligibility gate is strict enough that many real features will fall
  through, and the honest answer to "why did I not get a candidate" is a
  fifteen-row table.
- Two shipped behaviors change (D34), one of them requiring a config gate and a
  second expectation for an existing test, and one of them changing an
  unexported function's signature.
- Evidence readers must now distinguish verdict-bearing from supporting entries
  (D25); a consumer that treated every line as a verdict was already wrong, but
  this ADR is where that becomes visible.
- The supporting-entry set depends on the invocation's outcome, so a
  candidate-ready result genuinely carries fewer detector lines than the phase-4
  blocked result it replaces; the parity claim has to be read as semantic
  coverage, not as line count (D26).
- Every writer of `status.Reconcile` now goes through a shared merge, which is
  one more indirection on a hot path and a source guard that will reject an
  otherwise-innocent literal (D20b).
- Acceptance performs the per-path CAS twice, and still cannot promise anything
  about a writer outside `tpatch` — the honest statement is a smaller window
  plus a recoverable journal, not exclusion (D28).
- Implementation is fully blocked on another release.

## Rejected alternatives

| Alternative | Why rejected |
|---|---|
| Byte-window context expansion (rev-0's D9) | Rejects ordinary repeated bytes, has no decidable termination criterion, and cannot bridge upstream-deleted neighbours — so it fails on the very fixture the feature exists for (D9, D9.1) |
| Selecting one LCS traceback | The answer would depend on DP iteration order rather than on the two trees; unanimity or refusal is the only reproducible rule (D9 A4) |
| A global "the insertion bytes occur elsewhere" refusal | Refuses correct derivations whenever the inserted text is ordinary, and still misses the real hazard, which is an unrelated upstream addition *in the gap* (D9 A5) |
| A configurable or environment-tunable alignment limit | The same two trees would produce different answers on different machines (D9 L1-L3) |
| Make phase-2 conflict terminal `blocked` | Removes working phase-4 reapply behavior for every drifted feature; regression dressed as a feature (D3) |
| Terminal candidate-ready at phase 2 (rev-0's D2) | Suppresses a clean phase-4 reapply and a phase-3 provider `upstreamed`, both strictly better answers (D13) |
| Persist the candidate at phase 2 and delete it if phase 4 succeeds | A write that is usually undone is a race and an audit-trail lie (D13) |
| Introduce a phase 2.5 | Misdescribes the architecture, breaks ADR-025's closed phase enum, and adds an aggregator-visible value for zero gain (D24) |
| Bypass the confirmation gate for `recipe-operation-match` | Exactly the regression the gate was built to prevent; two features were lost to it (D2) |
| Confirm on `confidence: high` | Confidence is a self-assessment; any future evaluator could set it (D2) |
| Keep the legacy evaluator's terminal all-present arm beside the strict one | The weaker, fail-open path would decide first for every feature without coverage (D12b) |
| Retire the legacy evaluator with no config gate | Ships a disclosed behavior change and the feature in one release with no way to separate them during soak (D12b, D34) |
| Keep unreachable eligibility gates "for defense in depth" | A gate that cannot fire cannot be tested; an untestable row in an ordered table is worse than no row (D4) |
| Write a second, more permissive coverage decoder so those rows become reachable | Weakens ADR-036, which is forbidden (D4, D33) |
| Reuse `.tpatch/shadow` and `AcceptShadow` | Five independently disqualifying behaviors, from stale-patch re-application to non-atomicity to newest-wins reaping (D18, D27) |
| Reuse `rescap.AcquireLock` | Drags the `statfs` preflight, the scratch-root layout and the `.tpatch/local` contract into reconcile, and refuses outright on Windows (D28) |
| A PID-and-rename lock protocol with stale-lock breaking | Has the ABA and stale-owner problems the kernel lock exists to avoid; rev-0 attributed those semantics to a primitive that does not implement them (D28) |
| Require `HEAD == upstream_commit` at accept | Unreachable on any branch carrying a commit of its own, and strictly weaker than the per-path CAS anyway (D28) |
| Add a `root` parameter to `executeOperation` | Makes the dangerous call easier to write, not harder, and leaves the fail-open semantics intact (D12) |
| Leave the typed-root guard's boundary unstated | An unbounded exemption makes the guard unenforceable; two named constructors make it decidable (D12) |
| Execute the persisted recipe's `write-file` body cross-base | A whole-file postimage write is the exact hazard ADR-036 D8 documents (D7) |
| Persist derived alignments or record indices | Exactly what ADR-036 D8 refused; a persisted alignment will be trusted instead of re-derived (D11) |
| Include the candidate diff and tree hashes in the ID | Creates a staging-rename cycle, since the ID could not exist before the worktree it names (D19) |
| Route a candidate-ready result through `updateFeatureState` | Its `still_needed` arm forces `StateApplied` and overwrites the notes, so staging a proposal would silently change the feature's state (D20b) |
| Retroactively route existing flag combinations through the new validator | Changes shipped error bytes operators and scripts depend on, for no safety gain (D22) |
| Overload `--accept` to mean shadow-or-candidate | Makes a destructive command's behavior depend on invisible state (D22) |
| Regenerate a candidate immediately after rejection | An infinite loop with extra steps (D23) |
| Recover a crashed acceptance implicitly on the next run | Guessing the commit bit from file presence risks undoing a completed acceptance (D29b) |
| Treat a completed rollback as an internal error | It is a state refusal whose cleanup succeeded; ADR-031 D4 already has the right code for it (D29) |
| Run `test_command` during generation | Turns a read-only classification that runs on every reconcile into arbitrary code execution (D17) |
| Trust `coverage_status: complete` as eligibility | Forbidden in terms by ADR-036 D4 (D4, D5) |
| Bundle GH #13 into v0.17.0 | ADR-036 D12 already separated the releases; the consumer cannot be tested before the producer ships (D35) |
| A combined "recipe decodes and owns the coverage" gate (rev-1's E4) | Consumes every recipe-side failure and reports one code for four different problems, making `binding-presence-drift`, `binding-hash-drift` and `recipe-undecodable` unreachable (D4) |
| One evidence attempt per (invocation, phase), full stop (rev-1's D25) | Contradicts four shipped `phase-3.5` call sites and two `phase-4` ones; enforcing it would mean deleting shipped detectors or extending the closed phase enum (D25) |
| Carrying the confirmation facts in the evidence entry or in `confidence` | Anything the gate can read, a future evaluator can write; the authority must be a value only the evaluator can construct and nobody can persist (D2) |
| No feature-state precondition (rev-1) | A candidate derived at `blocked` could be accepted after the feature was rejected, unapplied or retired — the CAS proves the bytes match, not that the feature is still wanted (D7b) |
| Leaving `status.Apply.BaseCommit` at the pre-accept commit | Six shipped readers would classify the next reconcile against a base the feature no longer has, and the ADR-024 manifest would disagree with the published coverage (D29) |
| An unlocked staleness prune (rev-1's D21) | Races an in-flight acceptance that is re-materializing the same worktree at D29 step 5, and destroys rollback material (D21) |
| Claiming BSD lock support from a compiling build tag (rev-1's D28) | A primitive no CI job has ever run is not a validated primitive; the stub plus a named deferral is the honest position (D28) |
| Acquiring the lock on a network filesystem without saying so | Invites an operator to believe contention was checked when advisory locking provides no exclusion there; a named refusal is a limitation, silence is a trap (D28) |
| `golang.org/x/sys/windows` for `LockFileEx` | `syscall.NewLazyDLL` is stdlib, and the minimal-dependency rule predates this feature (D28) |
| A hardcoded list of behavioral flags for the exclusivity rule (rev-1's D22) | A snapshot of the flag set at authoring time; rev-1's own list was already missing six flags on the day it was written (D22) |
| An L3 large enough never to fire | rev-1's `16777216` cells is `128 MiB` of `int32` matrices for one file — a limit nobody would notice reaching, which is not a limit (D9) |

## Implementation order

1. Freeze the shipped baseline: reconcile goldens, `evaluateRecipeOperations`
   display behavior, all three confirmation-gate arms, resolver
   accept/reject/shadow-diff output, `validateReconcileFlags` message bytes,
   evidence JSONL byte-identity, and the `ReconcileSummary` no-candidate
   serialization. Promote both adjacent-CLI-args variants as **success**
   fixtures.
2. Typed `ReferenceRoot` / `CandidateRoot` with the two sanctioned
   constructors, `GIT_NO_LAZY_FETCH=1` on every candidate Git subprocess, the
   tree-hash primitive Git plumbing does not yet have a helper for (no
   `WriteTree`/`hash-object` wrapper exists in `internal/gitutil/`), the
   portable advisory-lock abstraction with its linux/darwin/windows primitives,
   its stub for every other target and its filesystem-class refusal, and the
   candidate namespace.
3. The eligibility gate E1-E15 in D4's order over ADR-036's strict decoder,
   D7b's feature-state precondition, and the `operation_candidate_enabled`
   config gate.
4. Insertion-run derivation, line-record splitting, the A1-A8 alignment rules
   with their first-match evaluation order and the exact `F`/`G` unanimity
   procedure, the L1-L3 limits, and three-state classification — with D9.1's
   fixture as an executable test.
5. The closed two-kind candidate executor, all-or-nothing execution, D14
   validation and the D15 idempotency second pass over every effect.
6. `candidate.json`, `state.json`, `candidate.patch`, `rc_<12hex>` including
   `source_feature_state`, path normalization, the `status.json` pointer, the
   locked preflight order, staleness, the immutable-audit treatment of accepted
   directories, and the rejection log.
7. Phase wiring: the D2 outcome table, the `phase2AuthorityProof` type with its
   single mint site, the fourth parameter on both `saveReconcileArtifacts` and
   the gate with `nil` at all **ten** shipped call sites (`:570` passing
   `phase35`, which names the **same** `*ReconcileResult` object as `result`),
   the unexported ordered `verdictAttempts` staging field with its
   three legal configurations — nothing, one qualifying phase-2 attempt paired
   with the proof, or the candidate-ready phase-2 / phase-4 pair with no proof
   — and `persistReconcileEvidence`'s substitution so exactly one
   verdict-bearing entry per phase is appended, the new phase-2
   verdict-bearing entry's exact fields, the CG1-CG7 authority class and its
   cross-check against the entry it
   grades, the D13 phase-4 proof disposition and
   finalization re-lock, the D20b lifecycle bypass — a dedicated **status**
   write over a `saveReconcileArtifacts(s, slug, result, nil)` call, never a
   reimplementation of it — **and** the shared status merge that carries the
   candidate pointer forward, the verdict-bearing /
   supporting evidence split with unchanged detector firing conditions, and the
   review rendering.
8. The five CLI flags and the source-derived exclusivity (85 pairs for the
   current flag set, with Cobra's `help` excluded by name), with shipped
   existing-only validation untouched.
9. `AcceptOperationCandidate`: locks, feature-state re-check, ancestry, the CAS
   at step 6 and again immediately before step 10's write, the fourteen-step
   transaction, step 8's named snapshot set, step 12's base metadata, rollback,
   recovery, tight exit codes, and coverage publication through ADR-036's
   shared API.
10. Docs, the `SPEC.md` §7 amendment across **all three** phase-2 rows, the
    confirmation-gate authority-class note, the parity-block delimiters and
    their extraction guard, skill-asset parity and soak.

## Deferred decisions and review triggers

Reopen this ADR when any row's trigger fires. Each row is a real deferral with
a stated v1 answer, not an unanswered question.

| Deferred | v1 answer | Reopen when |
|---|---|---|
| Replacement (`-` line) effects | Refused; `effect-replacement-unsupported` | A commutation theory is accepted that can prove a replacement's context survived |
| Delete / rename / copy effects | Refused; outside the D7 domain | A dedicated ADR defines their cross-base semantics |
| Binary effects | Refused; no record structure to align | A binary-delta alignment model is accepted |
| Executable / symlink / gitlink / mode changes | Refused; `write-file` is fixed `0o644` | The recipe schema gains a mode field (GH #15 surface) |
| Non-unanimous alignments | Refused; `alignment-ambiguous` | A tie-breaking rule can be shown safe without a knob, which this ADR does not believe is possible |
| Alignments beyond L1-L3 | Refused; `alignment-limit`, with L3 at `1048576` cells / 8 MiB of matrices | A sub-quadratic exact all-alignment decision procedure is accepted, or a streaming formulation removes the two-matrix requirement; the limits are never raised by configuration |
| Persisted alignments, record indices or offsets | Ephemeral only; digests are diagnostic | A consumer needs persistence **and** the D15 idempotency proof is shipped and soaked |
| Multi-slug candidates | Refused; single-slug only | Stack-aware sequential candidate derivation is designed |
| `--cumulative-legacy` candidates | Refused; ADR-030 preserved | Never without retiring cumulative derivation |
| Preview test execution during generation | None; explicit acceptance only | An explicit opt-in flag is specified with its own security review |
| More than one pending candidate per slug | One; older marked `stale` | Operators demonstrably need to compare candidates across upstreams |
| Candidate + resolver shadow coexistence | Refused with named remediation | A merge semantics for two staged answers is designed |
| Filesystem-wide atomic acceptance | Journal + rollback + explicit recovery | A content-addressed staging store makes single-rename publication possible |
| Automatic crash recovery | Explicit `--resume-candidate-recovery` | Recovery is proved total across every journal step |
| Candidate generation changing lifecycle state | No state change; pointer only | A candidate needs to gate other commands, which a pointer cannot express |
| Flipping `operation_candidate_enabled` on by default | `false` in v0.18.0 | The v0.18.0 soak shows no regression and D12b's disclosed behavior change is accepted on its own evidence |
| A `phase-2.5` evidence value | None; `phase-2` reused | Never — it would misdescribe the architecture |
| Widening the confirmation gate beyond CG1-CG7 | One authority class, seven conditions | A second deterministic proof of equal strength is designed; confidence alone never qualifies |
| Provider assistance inside phase 2 | Forbidden | Never; that is phase 3/3.5 |
| Advisory locking on the BSDs and every other unlisted platform | `Supported == false`, named `lock-unsupported` refusal; v1 runtime support is linux, darwin and windows only | A **native runtime CI job** exists for the target **and** the primitive is verified on it. A successful cross-compilation is explicitly not the trigger |
| Advisory locking on network filesystems | Refused; `lock-filesystem-unsupported`, known-local allowlist | A cross-host mutual-exclusion protocol is designed and reviewed; until then the per-path CAS and the journal are the authority (D28) |
| A non-`tpatch` writer mutating a candidate path between the step-10 CAS recheck and the write | **Not prevented, and not claimed to be** (D28); the recheck minimizes the window and the journal makes the outcome recoverable | The platform offers a usable multi-file transactional primitive, or a content-addressed staging store makes single-rename publication possible |
| Feature states beyond {`applied`, `blocked`} | Refused; `candidate-feature-state-unsupported` (D7b) | A state outside the pair is shown to have realized changes worth reproducing, with its own acceptance semantics |
| Regenerating coverage for P6 (`implement`) and P7 (`artifact-edit`) records | Untouched; such records are `incomplete`, so E3 refuses and the feature falls through | The regeneration contract GH #15's PRD `:3674-3678` assigns to GH #13 is designed; v1 regenerates only as a by-product of a successful accept (D30) |
| `ensure-directory` in the candidate domain | Not modeled; such a recipe cannot reach a `complete` ADR-036 record, so the feature refuses at E3 | A creation genuinely needs an empty directory **and** ADR-036 gains a representation for it |
| Superseded-feature candidates | Ineligible (E12) | ADR-028's supersession model itself changes |
| Hard-parent-blocked child candidates | Skipped (E13) | ADR-011 gains a partial-replay model |
| Recipe/coverage regeneration for `cycle` (P4) and `apply --mode done` (P5) | Untouched; ADR-036's deferral row stays open for both | A publication path for those producers is designed; D30 discharges only the `operation-candidate-accept` variant of P3 |
| Recipe regeneration on the `resolver-accept` path | Unchanged; still `incomplete` with `producer-patch-rewrite` + `recipe-not-regenerated` | `RefreshAfterAccept` gains a recipe regenerator, which is a GH #15-surface change |
| Committing on the operator's behalf | Never | Never |

## References

- `SPEC.md:438-458` — the four-phase decision tree whose phase-2 row D3 amends
  (ADR-010 is depended on and preserved, not amended)
- `internal/workflow/reconcile.go:19-85,74-84,315,383,432,455,480,524,536,564,570,575,583,590,596,445-464,451-457,453,459-462,477-483,496-596,588-596,611-655,646-647,649-650,724,766-772,812,813-815,821,825,826,828,829,843-845,846-849,851-868,857,861,862,867,875,876-880,884-898,888,899,903,913-923,917,924,928-930,931,935,947-949,951,965,978,998-1000,1005,1032,1033-1035,1037,1069-1070,1089-1090,1096-1112,1161-1176,1167-1168,1181-1235,1191-1209`
- `internal/workflow/file_novelty.go:115-116`,
  `internal/workflow/hunk_overlap.go:135-136`,
  `internal/workflow/path_restructure.go:118-119`,
  `internal/workflow/blocked_taxonomy.go:179`
- `internal/workflow/patch_generations.go:31`,
  `internal/store/patch_generations.go:30-67`
- `internal/workflow/recipe.go:90,128,194-195,199,211,223,235-241,242-243`
- `internal/workflow/accept.go:82,95,101,110,123,128`
- `internal/workflow/refresh.go:20-24,45,82,93,102`
- `internal/workflow/recipe_autogen.go:184-198`
- `internal/gitutil/shadow.go:35,56,98,115,132,152,191,206,248,309`
- `internal/gitutil/gitutil.go:652,789,803`
- `internal/gitutil/trailers.go:41,737`
- `internal/gitutil/lock_guard.go:18-23,99-100`
- `internal/gitutil/apply_classifier_test.go:319-320`,
  `internal/cli/land_rev1_fold_test.go:236-237`
- `internal/store/types.go:8-37,13,16,17,59-74,206,362-369,366,372-382`
- `internal/store/reconcile_evidence.go:17,41,63-77,89-107,96,116,120,137,166-175`
- `internal/cli/cobra.go:73,2294-2296,2316-2325,2440,2520-2543,2544,3200,3211-3218,3212,3215,3218,3731-3732`
- `internal/cli/reject.go:36-47`
- `internal/rescap/lock_unix.go:63,64-70,80-87`,
  `internal/rescap/lock_unsupported.go:1,15,29-31`,
  `internal/rescap/scratch.go:46-48`,
  `internal/rescap/statfs_linux.go:78-95`,
  `internal/cli/feature_resource.go:107`
- `go.mod` — cobra only, with pflag and mousetrap indirect; no `golang.org/x/sys`
- `internal/safety/safety.go:12`, `internal/workflow/validation.go:60,273`
- `internal/workflow/reconcile_evidence_integration_test.go:235,413,459-485`,
  `internal/workflow/reconcile_test.go:304,354-359`
- [ADR-010](./ADR-010-provider-conflict-resolver.md) D1, D2, D4, D5 —
  **depended on and preserved unchanged**, not amended (S1)
- [ADR-025](./ADR-025-reconcile-evidence-and-revision-schema.md) D1, D3, D4, D10
- [ADR-028](./ADR-028-supersession-edge-model.md),
  [ADR-029](./ADR-029-write-file-recipe-safety.md) D3, D4, D7, D8
- [ADR-030](./ADR-030-multi-slug-reconcile-derivation-mode.md) D1, D2, D6, D7
- [ADR-031](./ADR-031-rejected-feature-state-data-model.md) D4
- [ADR-034](./ADR-034-rooted-filesystem-inspection-boundary.md) D5, D12
- [ADR-036](./ADR-036-recipe-coverage-authority.md) rev-7 D1, D3, D4, D5, D8,
  D9, D10, D13, D15, D17, and its schema/predicate/decoder anchors
  `:464-543,1038-1059,1065-1076,1141,1227-1268,1518-1541,1547-1558,1745,1755-1766,1958-1966,1962,3048`
- [PRD-recipe-generation-authority](../prds/PRD-recipe-generation-authority.md)
  rev-7 §6.14 (`:2098`, ten-row consumer table `:2140-2151`), §13, and
  `:2691-2697,3674-3678`
- [PRD-upstreamed-confirmation-gate](../prds/PRD-upstreamed-confirmation-gate.md)
  `:57-70,79-80,91-99`
- [PRD-confirm-upstreamed-human-review-path](../prds/PRD-confirm-upstreamed-human-review-path.md)
- [PRD-reconcile-verdict-evidence](../prds/PRD-reconcile-verdict-evidence.md)
- `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md`
  §2, §3, §4.2
