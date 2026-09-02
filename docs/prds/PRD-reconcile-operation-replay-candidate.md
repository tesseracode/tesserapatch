# PRD - Reconcile Operation-Replay Candidate

**Status**: Accepted rev-6
**Date**: 2026-09-02
**Owner**: Core
**Issue**: [GH #13](https://github.com/tesseracode/tesserapatch/issues/13) —
`reconcile: implement safe phase-2 operation replay candidate generation`
**Architecture**:
[ADR-037 - Reconcile Operation-Replay Candidate Authority](../adrs/ADR-037-reconcile-operation-replay-candidate-authority.md)
rev-6 — **normative where the two documents overlap**
**Depends on**:
[ADR-010](../adrs/ADR-010-provider-conflict-resolver.md),
[ADR-011](../adrs/ADR-011-feature-dependencies.md),
[ADR-017](../adrs/ADR-017-reconcile-lock-guard-and-writer-normalization.md),
[ADR-024](../adrs/ADR-024-patch-generation-manifest-boundary.md),
[ADR-025](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md),
[ADR-028](../adrs/ADR-028-supersession-edge-model.md),
[ADR-029](../adrs/ADR-029-write-file-recipe-safety.md),
[ADR-030](../adrs/ADR-030-multi-slug-reconcile-derivation-mode.md),
[ADR-031](../adrs/ADR-031-rejected-feature-state-data-model.md) D4 exit-code
envelope,
[ADR-033](../adrs/ADR-033-resource-capture-boundary.md),
[ADR-034](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md),
[ADR-036](../adrs/ADR-036-recipe-coverage-authority.md) rev-7,
[PRD-recipe-generation-authority](./PRD-recipe-generation-authority.md) rev-7,
[PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md)
(Approved),
[PRD-upstreamed-confirmation-gate](./PRD-upstreamed-confirmation-gate.md)
(Approved), and
[PRD-confirm-upstreamed-human-review-path](./PRD-confirm-upstreamed-human-review-path.md)
(Accepted)
**Amends**: the accepted confirmation-gate contract
([PRD-upstreamed-confirmation-gate](./PRD-upstreamed-confirmation-gate.md) §3,
[PRD-confirm-upstreamed-human-review-path](./PRD-confirm-upstreamed-human-review-path.md))
by adding **one** new confirming authority class (§6.2), all three of
`SPEC.md:444-448`'s phase-2 rows (§6.3),
[ADR-025](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md)
D1/D3's verdict cardinality (§6.17), and ADR-036 D15's producer P3 into two
variants over one enum value (§6.16).
**Preserves unchanged**:
[ADR-010](../adrs/ADR-010-provider-conflict-resolver.md)'s four-phase order,
shadow-worktree atomicity model and provider-resolution contract — rev-2
**removes ADR-010 from the amended set**, because the phase-2 terminality
sentence that changes lives in `SPEC.md` §7, not in ADR-010, and nothing in
ADR-010's own decisions is re-specified here.
**Blocked by**: GH #15 shipped in **v0.17.0**. No implementation slice starts
before that release exists.
**Target release**: v0.18.0, separate from GH #15's v0.17.0

## 0. Revision history

| Rev | Date | Change |
|---|---|---|
| rev-0 | 2026-09-01 | Initial proposal. Contract §6.1-§6.16, vocabulary §7, ten sequential slices §8, 212-row acceptance matrix §9 (I 46, C 64, G 40, U 56, S 6), deferrals §11, claims audit §12. Reviewed **NEEDS REVISION** (C1-C6, H1-H6, M1-M11 plus optional findings). |
| rev-1 | 2026-09-01 | **Confirmation gate corrected and amended**: rev-0 asserted the all-present arm was behaviorally unchanged; the shipped gate demotes an operation-match `upstreamed` to `blocked` / `rejected-upstreamed` (`internal/workflow/reconcile.go:1047-1060,1088-1091`, pinned by `internal/workflow/reconcile_evidence_integration_test.go:459`), so this revision states that source truth and adds one strictly-qualified new confirming authority class (§6.2, C1). **Byte-window anchor expansion deleted and replaced** by exact line-record alignment over the set of *all* maximum-cardinality monotone alignments, with unanimity as the authority condition, gap classification, closed resource limits and an `alignment-limit` refusal; the motivating adjacent-CLI-args fixture is now pinned to candidate **success** (§6.7, C2/C3, M7). **Candidate persistence moved behind an actual phase-4 textual conflict** — phase 2 computes an in-memory proof only, phase 3 always runs, and strict/3-way-clean phase 4 keeps shipped `reapplied` (§6.2, C4). **Acceptance HEAD equality replaced** by an ancestor check plus authoritative per-path CAS (§6.14, C5). **Lock contract replaced** by a new portable kernel-advisory abstraction with no PID metadata and no stale-breaking (§6.13, C6). **Legacy evaluator retired as authority** behind a config gate (§6.4, H1); gate order made achievable against ADR-036's strict decoder, from sixteen gates to fifteen (§6.5, H2); typed roots given two sanctioned constructors (§6.9, H3); flag inventory pinned at five new terminals and eight terminal actions (§6.12, H4); candidate-ready `still_needed` bypasses `updateFeatureState` (§6.11, H5); candidate identity reduced to an immutable pre-execution tuple with a separate mutable `state.json` (§6.10, H6); recovery preflight, one canonical transaction list, phase-local evidence verdicts, generation-scope definition, recipe-as-witness vocabulary, `ensure-directory` disposition, staleness preflight, the full ADR-036 D15 P3 restatement and `GIT_NO_LAZY_FETCH=1` all specified (M1-M11). Matrix rebuilt to **253** contiguous rows (I 51, C 77, G 61, U 56, S 8). Reviewed **NEEDS REVISION** (B1-B6, S1-S3 plus ten optional findings); the alignment mathematics of §6.7 was **approved** and is carried forward unchanged. |
| rev-2 | 2026-09-02 | **Eligibility order made reachable** (B1): E4's combined decode-and-owner gate shadowed every later recipe-drift code, so the recipe gates are split and re-sequenced — presence (E4), raw hashes (E5), strict decode (E6) and, **conditional on E6 only**, decoded-recipe owner (E7); §6.3's mapping, §7.1's vocabulary order, §9.3 and parity block B follow. **Evidence cardinality corrected** (B2): the shipped pipeline already appends four `phase-3.5` entries and up to two `phase-4` entries per invocation (`internal/workflow/reconcile.go:766-771`), so the one-attempt-per-phase claim was false; §6.17 now splits **verdict-bearing** from **supporting** entries and pins at most one verdict-bearing attempt per (invocation, phase) while supporting detector entries stay unlimited, ordered and semantically unchanged. **Acceptance gains a feature-state contract** (B3): generation requires `status.State ∈ {applied, blocked}`, `source_feature_state` is bound into the immutable identity tuple and the status pointer, and acceptance refuses `candidate-feature-state-changed` on any drift. **Acceptance gains a base-metadata contract** (B4): successful acceptance sets `status.Apply.BaseCommit = upstream_commit` with a matching staged ADR-024 generation entry, and the coverage reference and preimage set derive from the same commit. **Confirmation facts gain an in-process carrier** (B5): an unexported typed `phase2AuthorityProof`, minted only by the GH #13 evaluator and passed as a new parameter to `applyUpstreamedConfirmationGate`, is what CG1-CG7 consume; it is never serialized and never reconstructed. **Every candidate-state mutation is locked** (B6): the staleness preflight now runs under the per-feature lock before phase 1, and no prune happens unlocked. ADR-010 removed from **Amends** (S1); runtime lock support narrowed to linux/darwin/windows with BSD deferred to the unsupported stub (S2); the lock constructor gains its own network-filesystem refusal `lock-filesystem-unsupported` without importing the `rescap` local contract (S3). Ten optional findings closed: warning-state count corrected to **seven**, the ten-row consumer table re-attributed to GH #15 PRD §6.14, source anchors corrected, `candidate-not-idempotent` widened, accepted-candidate directories declared immutable audit, A1-A8 given a fixed first-match evaluation order with traceback-free sentinel predicates, L3 lowered to a memory-bounded `1048576` cells, flag exclusivity made source-derived (**85** refusing pairs), `LockFileEx` pinned to stdlib `syscall`, fallthrough scoped to the phase-2 vocabulary, and GH #15's P3-P7 regeneration ownership quoted in full. Matrix rebuilt to **280** contiguous rows (I 54, C 83, G 70, U 64, S 9). Reviewed **NEEDS REVISION** (R1-R3 plus seven optional findings); the alignment mathematics of §6.7 and the acceptance/coverage model remain **approved** and are carried forward unchanged. |
| rev-3 | 2026-09-02 | **The phase-2 evidence entry CG5-CG7 grade is now named and its fields are pinned** (R1): CG5-CG7's subject is the **new GH #13 phase-2 verdict-bearing `recipe-operation-match` attempt**, whose builder sets `confidence: high`, `match_origin: upstream`, `pre_reconcile_presence: present`, `upstream_commit_refs: [upstreamCommit]` and `upstream_commit == upstreamCommit` **only** after the full E1-E15 and all-present proof; the shipped legacy builder keeps `unknown` / `[]` / `low` (`internal/workflow/reconcile.go:843-845`). The proof's plumbing is defined end to end: `saveReconcileArtifacts` (`:724`) gains a fourth `proof *phase2AuthorityProof` parameter, all **ten** shipped call sites pass `nil`, and only the GH #13 all-present path passes the minted value through to `applyUpstreamedConfirmationGate` (`:771`); the gate cross-checks the proof against **that exact entry** and nothing is reconstructed from stored bytes (§6.2, parity block A). *(rev-3 said **nine** call sites and omitted `:570`; corrected in rev-4.)* **Evidence taxonomy corrected against the shipped guards** (R2): the shipped maximum is **one verdict-bearing entry plus three supporting detector entries** at `phase-3.5` and **one verdict-bearing plus one supporting** at `phase-4`; the confirmation gate's supporting entry is **mutually exclusive** with the blocked-only path-restructure and blocked-classification entries (`:769-771`, guards `:947-949,998-1000,1033-1035`). Candidate-ready neither claims nor forces path-restructure or blocked-classification support — detector semantics are unchanged and its diagnostic richness is carried by `candidate.json`, the effect map, `candidate.patch` and the phase-2 verdict entry, by **semantic field**, not by detector line count (§6.17). **The candidate pointer survives every unrelated status write** (R3): a shared status merge carries the loaded `Reconcile.OperationCandidate` forward **before** the standard `ReconcileSummary` replacement on every non-candidate arm of `updateFeatureState` (`:1181,1191-1209`), and only the five explicit candidate transitions — finalize, stale, reject, accept, recovery — may set, modify or clear it; a source guard and a runtime row pin it (§6.11). Optional findings closed: acceptance re-validates the per-path CAS immediately before the first live write with the same `candidate-cas-failed`, and the "last word" / cross-host wording is softened to what the locks actually provide (§6.14); §9.13's addition arithmetic is corrected against the real delta; the `SPEC.md` §7 amendment now names **all three** phase-2 bullets and S9 edits all three; Cobra's auto-registered `help` flag is explicitly excluded from the source-derived behavioral flag set, keeping the count at **85** (§6.12); the rollback snapshot set names `patch-generations.json` and `recipe-provenance.json` beside the patch, recipe, coverage and status artifacts and the created-path absent markers (§6.14 step 8); every parity block gains explicit `<!-- parity-block:<ID>:begin -->` / `<!-- parity-block:<ID>:end -->` delimiters so the guard extracts only marked regions (§6.0). Matrix rebuilt to **289** contiguous rows (I 57, C 85, G 72, U 66, S 9). |
| rev-4 | 2026-09-02 | **The `saveReconcileArtifacts` call-site inventory is corrected from nine to TEN** (F1, blocking): the shipped function is called at `internal/workflow/reconcile.go:383,432,455,480,524,536,564,570,583,596`, and rev-3 omitted `:570` — the phase-3.5 call, whose third argument is spelled `phase35` rather than `result`. *(rev-4 called `phase35` "the distinct `*ReconcileResult` returned by `tryPhase35`, not the pipeline's `result`"; rev-6 corrects that: `tryPhase35` takes `result` as its ninth parameter (`:1363`) and returns that same pointer from all ten of its returns, so `phase35` is a distinct variable name bound to the same object.)* Every nine-call claim is replaced with the ten-site list in parity block A (byte-identically on both sides), in this table, in §0.3's R1 row, in §8's S6 slice, in ROC-282 and in §12's claims audit; `:570` is stated to pass `phase35` and, exactly like every existing and legacy call, a **`nil`** proof, so only the GH #13 all-present path ever passes a minted proof. **The new phase-2 verdict-bearing attempt replaces the legacy derived entry rather than joining it** (opt. 1): the evaluator builds it in the same statement that mints the proof and stages it in one new unexported `phase2Verdict *store.ReconcileEvidence` field on `ReconcileResult` (`:19-85`, with `attemptedAt` `:74-84` as the shipped precedent); `proof != nil` **iff** `result.phase2Verdict != nil`; *(rev-4's singular field and its unconditional biconditional are superseded by rev-5's ordered `verdictAttempts` list and its configuration-2-only pairing.)* and `persistReconcileEvidence` (`:766`) emits the staged attempt **in place of** the entry it would have derived through `evidencePhaseAndKind` (`:1161-1176`), so **exactly one** verdict-bearing `phase-2` entry exists on the authoritative all-present arm, in the shipped `:766` position — the legacy builder is untouched and still runs verbatim on every non-authoritative arm (§6.2, §6.17). **The gate grades an entry it does not write** (opt. 2): the cross-check is against "the exact verdict-bearing entry **it is grading**", already appended at `:766` before the gate runs at `:771`; the only line the gate itself appends is its separate supporting `phase-3.5` / `manual-review` entry (`:1033-1035,1069-1070`). **Positive and anti-duplicate cases pinned** (opt. 3): ROC-290 and ROC-291 in the new §9.14, plus §6.2's source guard extended to the staged attempt's single assignment site and single read site. The two prose references to the delimiter template in §0 and §0.3 are normalized from a literal `X` to `<ID>`, matching ADR-037 and §6.0's own template, so the marker inventory is one-sided nowhere and ROC-278's symmetry claim holds. No other contract changes: no decision, gate, limit, transaction step, flag count or parity region is added, removed or renumbered, and only region `A` changes — identically on both sides. Matrix rebuilt to **291** contiguous rows (I 58, C 85, G 73, U 66, S 9). |
| rev-5 | 2026-09-02 | **The singular staging field is replaced by an ordered list, and the candidate-ready arm becomes a staging arm** (B1, blocking): rev-4's `phase2Verdict *store.ReconcileEvidence` could carry only one entry and only on the all-present arm, so candidate-ready — whose `result.Phase` is `phase-2-operation-candidate` and therefore *also* takes `evidencePhaseAndKind`'s `phase-2` prefix branch (`internal/workflow/reconcile.go:1167-1168`) — would have had the legacy builder derive a **duplicate** `phase-2` verdict entry and would have had **no carrier at all** for its required phase-4 textual-conflict verdict. rev-5 replaces the field with one unexported ordered `verdictAttempts []store.ReconcileEvidence` on `ReconcileResult` (`:19-85`), still ignored by `encoding/json` on the `attemptedAt` precedent (`:74-84`), and pins **three** legal configurations: (1) every other arm stages nothing and the shipped single derived builder runs unchanged; (2) the authoritative all-present arm stages **exactly one** qualifying phase-2 verdict-bearing `recipe-operation-match` attempt, paired biconditionally with the minted `phase2AuthorityProof`; (3) the candidate-ready arm stages **exactly two** ordered attempts before save — `[1]` phase-2 / `recipe-operation-match` / `still_needed` / `high` for the independently proved candidate and `[2]` phase-4 / `forward-apply` / `blocked` / `low` for why ordinary patch replay could not win — and mints **no** proof. `persistReconcileEvidence` (`:766`) emits the staged list **in place of** the entry it would have derived through `evidencePhaseAndKind` (`:1161-1176`), so `result.Phase` may stay `phase-2-operation-candidate` with no duplication; the biconditional is now proof-non-nil **iff** configuration 2, so a proof is **never** paired with candidate-ready; and any illegal configuration is treated as no proof and no staging at all. The source guard is widened to **six** axes — single assignment site, single mutation site, single persist read site, no serialization, pairwise-distinct `attempt_id`s and strictly increasing phase order — and §6.17 / D25 name the two routes by which a phase-2 finding reaches the JSONL: **staged substitution** on the two arms whose terminal `result.Phase` derives `phase-2`, and **direct append** on the fallthrough arms, where no collision is possible. Parity block `A` and D25 change identically on both sides; §6.17's example, per-arm table and confidence ladder, ROC-038, ROC-175, ROC-208, ROC-209, ROC-210, ROC-211, ROC-221, ROC-281, ROC-283, ROC-290 and ROC-291 follow, and ROC-292 / ROC-293 pin the candidate-ready positive and defect cases. **§9's preamble ID range is corrected** (B2): it stopped at `ROC-289` while the matrix already ran to `ROC-291`; it now reads `ROC-001` … `ROC-293`, recounted after rev-5's additions. **Every remaining "entry the gate is about to write / being written" is replaced** (B3) by "the exact verdict-bearing entry **it is grading**" — in §8's S6 slice and ROC-038, joining parity block A's rev-4 correction — with the accompanying statement that the only line the gate appends is its own separate supporting `phase-3.5` / `manual-review` entry (`:1033-1035,1069-1070`). §6.0 gains ADR-037 D36's carve-out that the literal `<ID>` template is **not** a marked region, and both documents now state that the marker parser recognizes only the closed nine-ID set, so neither illustration is read as an unpaired `begin`. No other contract changes: no decision, gate, limit, transaction step, flag count or parity region is added, removed or renumbered, and only region `A` changes — identically on both sides. Matrix rebuilt to **293** contiguous rows (I 59, C 85, G 74, U 66, S 9). |
| rev-6 | 2026-09-02 | **The `:570` argument claim is corrected: `phase35` is a distinct variable *name*, not a distinct object** (blocking): rev-4 and rev-5 described `internal/workflow/reconcile.go:570`'s third argument as "the distinct `*ReconcileResult` returned by `tryPhase35`". The shipped source says otherwise — `tryPhase35` (`:569`, defined `:1354-1364`) takes the pipeline's `result` as its **ninth parameter** (`:1363`), mutates it in place, and returns **that same pointer** from every one of its ten returns, each literally `return result` (`:1373,1381,1389,1436,1475,1491,1497,1502,1508,1513`). `phase35` and `result` therefore name **one object**, and `:570` differs from the other nine call sites in argument spelling only; like every existing and legacy call it stages no attempt and passes a **`nil`** proof. The corrected statement replaces the old one in parity block A (byte-identically on both sides), in this table's rev-4 row, in §0.3's R1 row, in §0.4's F1 row, in §8's S6 slice, in ROC-282 and in §12's claims audit, whose phase-3.5 row is rewritten against the real signature and returns; the same correction lands in ADR-037's revision history, parity block A, D34, implementation order and references. The ten-site inventory, the proof route, the three staging configurations and the CG1-CG7 class are untouched — only the *reason* `:570` was singled out changes. **Three adjacent notes are folded in.** (1) ROC-290's "byte-identical to the S0 golden" is scoped to the **gate-off** arm (§6.17 per-arm table row 2) alone; with the gate **on**, a legacy or ineligible all-present result takes row 4's direct-append route and carries an **intentional** phase-2 evidence delta against S0 while still emitting exactly one verdict-bearing entry per phase, and a fixture asserting S0 byte identity there fails. (2) ROC-281's legacy `low` / `unknown` / `[]` entry and its S0 byte identity are likewise scoped to `operation_candidate_enabled` **off**; the gate-on ineligible fallthrough uses the direct phase-2 append of row 4. (3) §6.17 and ADR-037 D25 now pin that **both** routes write **both** sinks in one order — every verdict-bearing attempt reaches `reconcile-evidence.jsonl` (via `store.AppendReconcileEvidence`, `:876`) **and** `result.Evidence` in the same relative order, which staged substitution inherits from the shipped `:766` / `:880` shape and the direct-append route must do explicitly, so `reconcile-session.json`'s serialized `evidence` array (`:775-776`) never becomes a strict subset of the JSONL; ROC-208 is amended to pin it at runtime, and the omit-the-`result.Evidence`-half implementation fails that row. No contract, decision, gate, limit, transaction step, flag count or parity region is added, removed or renumbered; no new matrix row is required. Matrix stays at **293** contiguous rows (I 59, C 85, G 74, U 66, S 9). |

## 0.1 Review response — rev-0 findings

Every rev-0 finding, the adjudication applied, and where rev-1 discharges it.
**Every row is a change to this document or to ADR-037; none is deferred.**

| # | rev-0 finding | rev-1 resolution |
|---|---|---|
| C1 | "The all-present arm is behaviorally unchanged" is false — authoritative all-present evidence does not satisfy the shipped confirmation gate | §6.2 states the source truth (`applyUpstreamedConfirmationGate` confirms only on `patch-id-match` with a `matched_upstream_sha`, or high-confidence `reverse-apply`) with the integration-test citation, adds the three confirmation PRDs to **Depends on**, and amends the gate with **one** new authority class requiring seven independently recomputed conditions. Goals 3/12, ADR-037 D2/D24 and the matrix follow |
| C2 | The byte-window expansion algorithm rejects ordinary repeated bytes and has no decidable termination | §6.7 deletes it. Boundaries map by **exact line-record alignment** over all maximum-cardinality monotone alignments, with unanimity as the authority condition |
| C3 | The algorithm cannot solve its own motivating fixture (adjacent CLI args with upstream deletions) | §6.7's worked proof maps the fixture to a **unanimous empty gap** and an applicable insertion; §8 S0 pins it as a **success** fixture with both case-study variants |
| C4 | Terminal candidate-ready at phase 2 suppresses working phase-4 reapply | §6.2: phase 2 computes an in-memory proof and returns nothing; phase 3 always runs; strict / 3-way-clean phase 4 keeps shipped `reapplied` and discards the proof; only a real phase-4 textual conflict with `opts.Resolve` unset finalizes and persists |
| C5 | `HEAD == upstream_commit` is unreachable on normal branches | §6.14: `upstream_commit` must be HEAD **or an ancestor of** HEAD (`head-diverged` otherwise), and the authority is the exact per-path CAS against the `upstreamCommit` tree |
| C6 | The PID / stale-breaking lock protocol contradicts the cited kernel lock | §6.13 defines a **new** portable abstraction (`flock` on Unix/BSD, `LockFileEx` on Windows, build-tagged unsupported elsewhere) with no PID metadata, no stale-breaking, inert residue, and a named `lock-unsupported` refusal. `internal/rescap`'s lock and local contract are **not** reused |
| H1 | The legacy evaluator retains terminal authority | §6.4 retires it: behind the config gate it produces diagnostic counts only and never a terminal `upstreamed` or a candidate. The affected existing expectation is named, and no default byte identity is claimed where behavior intentionally changes |
| H2 | Gate order contains rows ADR-036's strict decoder makes unreachable | §6.5: fifteen ordered gates. Internally contradictory `complete` records fail **first** at E1 as `coverage-unusable`; `coverage-incomplete` is E3; the outstanding-reason, unknown-axis and empty-effect codes are removed as reachable refusals and survive only where they are genuine recomputation defenses |
| H3 | The typed-root rule has no defined bare-root boundary | §6.9 defines `OpenReferenceRoot` and `CreateCandidateRoot` as the **only** two sanctioned bare-root constructors; they set `GIT_NO_LAZY_FETCH=1` and validate commit and path. The source guard excludes exactly those two symbols |
| H4 | The flag count and exclusivity set are wrong and would retroactively re-route existing flags | §6.12: **five** new terminals, **eight** terminal actions, exact pinned pair counts. Existing-only combinations keep shipped validation and shipped message bytes; the new rule is a second stage that fires only when a new flag is set |
| H5 | Candidate-ready `still_needed` would rewrite the feature's lifecycle state | §6.11: it **bypasses** `updateFeatureState` and persists through a dedicated path that preserves `status.State` and `status.Notes` exactly |
| H6 | `candidate_id` mixes execution results and status into identity, forcing a staging rename | §6.10: the ID hashes only the immutable **pre-execution** tuple, so it is computable before worktree creation; `candidate.json` is immutable and status-free; a separate `state.json` carries lifecycle |
| M1 | Recovery had no defined detection point | §6.15: every reconcile invocation and every candidate action checks the journal **before phase 1**, refuses `recovery-required`, and resumes only under `--resume-candidate-recovery`, deterministically by commit marker |
| M2 | The PRD and ADR transaction lists disagreed, and "first live write" was mis-numbered | §6.14's fourteen-step list is reproduced byte-identically in ADR-037 D29; step 10 is the first live write everywhere; state and artifact failure roll back and are never warnings |
| M3 | A fallthrough attempt's verdict was undefined | §6.17: each attempt carries its **phase-local** `raw_reconcile_verdict`; the terminal phase gets its own entry; `tpatch reconcile review` renders both, labelled |
| M4 | rev-0 implied evidence byte identity across the change | §6.17: the new authoritative entry intentionally mints a **new** `attempt_id`; identical future attempts dedupe; same ID with different bytes is corruption |
| M5 | "No live path is written" was unscoped | §6.8: the guarantee is over **repository content**; `.tpatch/` artifacts and `.git/worktrees` metadata are expected writes |
| M6 | "Operation replay" implied replaying persisted operation bodies | §7.0 and §6.6: the persisted recipe is an **eligibility and coverage witness**; v1 replays normalized **effects** |
| M7 | Overlap, EOL and idempotency rules were tied to the deleted algorithm | §6.7 restates them over mapped gaps and exact bytes; §6.8 defines idempotency as a second derivation classifying **every** effect present with tree and diff unchanged |
| M8 | `ensure-directory` was claimed as an independent E-gate refusal | §6.6: it is already caught by ADR-036 completeness; no separate claim is made |
| M9 | Staleness had no preflight ordering | §6.11: the staleness and recovery preflight runs before phase 1 and phase 1.5, subject to no active journal |
| M10 | The ADR-036 D15 P3 amendment was stated only in part | §6.16 restates **both** variants in full, including the operation-candidate variant's producer enum, capture mode, reference and residual patch |
| M11 | Lazy fetch was not disabled | §6.9: `GIT_NO_LAZY_FETCH=1` on every candidate and reference-tree Git subprocess, with a test source guard |
| opt. | `acceptance-rolled-back` was an internal error | §7.4: it is a state refusal with a completed rollback and exits `3`; only a failed rollback is exit `1` |
| opt. | Coverage-publication failure had loose error semantics | §6.16 / §7.4: it rolls back, exits `3`, and has no warning-class rendering |
| opt. | Metadata path normalization was unstated | §6.10: every persisted path is repository-relative and `filepath.ToSlash`-normalized |
| opt. | Citations and output claims drifted | §12 re-verified at `e76e0f7`; §6.8's vocabulary rule restated |

## 0.2 Review response — rev-1 findings

rev-1's alignment mathematics (§6.7, parity block C) was **approved** and is
carried forward unchanged except for the evaluation-order and limit
corrections named below. Every remaining rev-1 finding, the adjudication
applied, and where rev-2 discharges it. **Every row is a change to this
document or to ADR-037; none is deferred.**

| # | rev-1 finding | rev-2 resolution |
|---|---|---|
| B1 | The gate order makes recipe drift codes unreachable — E4 required the recipe to decode *and* to own the coverage, so `binding-presence-drift`, `binding-hash-drift` and `recipe-undecodable` could never fire on the recipe side | §6.5 splits and re-sequences the recipe gates: E4 readable presence in both directions, E5 raw-byte hashes, E6 strict decode, E7 decoded-recipe owner **conditional on E6 alone**. E8-E15 keep their rev-1 meanings at their new positions. §6.3's mapping, §7.1's ordering, §9.3's rows and ADR-037 D4 (parity block B) are updated together, and a new row pins that no gate is shadowed by an earlier one |
| B2 | Evidence cardinality ignores existing same-phase detector entries — the shipped pipeline already writes four `phase-3.5` entries and can write two `phase-4` entries in one invocation | §6.17 replaces the one-attempt-per-phase claim with a **verdict-bearing / supporting** split: at most one verdict-bearing attempt per (invocation, phase); supporting detector entries (`file-novelty`, `hunk-overlap`, `path-restructure`, `blocked-classification`, the confirmation gate's `manual-review`) are unlimited, ordered and keep their shipped phase, kind and verdict semantics. The "exactly two attempts" rows are corrected |
| B3 | Acceptance has no permitted feature-state rule | §6.6a, §6.10, §6.11 and §6.14: generation requires `status.State` ∈ {`applied`, `blocked`} using the shipped `store.FeatureState` spellings (`internal/store/types.go:8-37`); `source_feature_state` is bound into the immutable identity tuple, `candidate.json` and the status pointer; acceptance requires the current state to equal the bound one and to still be permitted, and otherwise refuses `candidate-feature-state-changed`, exit `3`, with no writes |
| B4 | Acceptance has no `Apply.BaseCommit` rule | §6.14 steps 7 and 12, and §6.16: successful acceptance sets `status.Apply.BaseCommit = upstream_commit` (`internal/store/types.go:362-369`) together with `CompletedAt`, `HasPatch` and `HasRecipe`, and stages an ADR-024 generation entry with `base_commit = upstream_commit` and reconcile-consistent `upper` / `capture` before the first live write. Coverage `reference.commit` and `preimage_set_sha256` derive from the same commit, and the six shipped `status.Apply.BaseCommit` readers are pinned to observe the new base |
| B5 | CG2-CG4 lack an in-process carrier — nothing in the shipped gate signature can carry a recomputation fact | §6.2 defines the unexported typed `phase2AuthorityProof`, minted only by the GH #13 evaluator after E1-E15 and the all-present classification, and passed **as a new parameter** to `applyUpstreamedConfirmationGate` (`internal/workflow/reconcile.go:1032`, called at `:771`) in the same invocation. It is never serialized, never read back from `status.json` or evidence, and never reconstructed from `confidence`; it is `nil` for every legacy caller |
| B6 | Stale-candidate cleanup can race acceptance outside the feature lock | §6.11 and §6.13: the staleness preflight acquires the per-feature lock nonblocking **before phase 1**, inspects the journal, pointer and bindings, performs any stale mutation or prune, and releases before any provider work. Finalization reacquires and revalidates; acceptance holds the feature and global locks for the whole transaction. Contention refuses the candidate path safely and there is no unlocked prune anywhere |
| S1 | ADR-010 is listed as amended although nothing in it changes | The header's **Amends** list drops ADR-010; it stays in **Depends on** and gains an explicit **Preserves unchanged** entry. Only `SPEC.md` §7's phase-2 terminality row is amended (§6.3). ADR-037's header and `docs/adrs/README.md` are corrected the same way |
| S2 | The lock platform table claims BSD support that no CI target validates | §6.13: **runtime-supported targets are `linux`, `darwin` and `windows` in v1**. Every BSD and every other target compiles against the unsupported stub, so phase 2 falls through with `lock-unsupported` and `--accept-candidate` refuses. Cross-compilation is explicitly **not** claimed as runtime validation |
| S3 | "No `statfs` preflight" over-promises: an advisory lock on a network filesystem is not cross-host safe | §6.13: the **new** lock constructor performs its own known-network/remote filesystem refusal `lock-filesystem-unsupported` — `statfs`/`fstatfs` on linux and darwin, `GetDriveTypeW` on Windows — and refuses on unknown or remote classes. The per-path CAS and the acceptance journal remain the authoritative defense; no cross-host safety is claimed for the lock alone, and no `rescap` local/ignore contract is imported |
| opt. 1 | Warning-state count wrong | §6.3 and §12: **seven** warning-class exit-`0` rows, not five (`docs/prds/PRD-recipe-generation-authority.md:2140-2151`) |
| opt. 2 | The ten-row consumer table was attributed to ADR-036 D13 | §6.3 and ADR-037 D3 attribute it to **GH #15 PRD §6.14** (`:2098`, table `:2140-2151`). ADR-036 D13 is the **six-rung** verify severity ladder (`docs/adrs/ADR-036-recipe-coverage-authority.md:1745`, ladder `:1755-1766`) |
| opt. 3 | Source anchors drifted for the reconcile flag block and the recipe switches | §12: flag block `internal/cli/cobra.go:2520-2543`; `--cumulative-legacy` `:2542`; both recipe switch defaults `internal/workflow/recipe.go:194-195,242-243`; the fail-open increment `internal/workflow/reconcile.go:649-650`; unconditional `ensure-directory` presence `:646-647` |
| opt. 4 | `validateReconcileFlags` anchors drifted | §6.12, §9.9 and §12: definition `internal/cli/cobra.go:3200`, rule body `:3211-3218`, the three shipped message strings `:3212,3215,3218` |
| opt. 5 | `candidate-not-idempotent` was under-specified | §6.8 and §7.3: **any** second-pass result other than all-present refuses — including a second-pass alignment refusal, an `alignment-limit`, an unanchored boundary or any other refusal code |
| opt. 6 | An accepted candidate directory's disposition was unstated | §6.10 and §6.11: an `accepted` directory is **immutable audit only** — never reused, never rejected, never overwritten — and its pointer is cleared on success. Re-deriving the same identity afterwards records `candidate-already-accepted` and falls through unless the bindings changed |
| opt. 7-8 | A1-A8 had no evaluation order, A3's sentinel definition implied a traceback, and multi-failure reasons were nondeterministic | §6.7 adds a fixed **first-match** evaluation order (limits → construction → unanimity → sentinel legality → terminator → gap classification → cross-boundary disjointness), restates A3's sentinels as the DP predicates `left₀ == 0` and `right₀ == len(U)` under an established unanimity, and pins the reported reason to the earliest refusing rule |
| opt. 9 | L3 was not memory-bounded | §6.7: L3 is **`1048576`** dynamic-program cells, so the two `int32` matrices the A4 procedure needs occupy at most **8 MiB**, and the total alignment budget is stated with `alignment-limit` as the only response |
| opt. 10 | The fixed 55-pair exclusivity is fragile and did not cover `--upstream-ref` | §6.12 makes the rule **source-derived**: the new terminals combine only with the presentation allowlist (`--path`, `internal/cli/cobra.go:73`; `--format`, `:2543`) and refuse every other registered reconcile flag that is explicitly set, including `--upstream-ref`, `--allow-dirty`, `--allow-stale-lock`, `--max-conflicts`, `--model` and `--timeout`. The recomputed current set is 15 behavioral flags, so **85** refusing pairs involve a new flag |
| opt. 11 | The Windows lock implied a new dependency | §6.13: `LockFileEx` is bound through stdlib `syscall` / `syscall.NewLazyDLL` in a `windows`-tagged file. `go.mod` stays at cobra + pflag + mousetrap; `golang.org/x/sys` is not added |
| opt. 12 | "Every refusal falls through" was unscoped | §1, §6.3 and §7: the fallthrough rule is scoped to the **phase-2 refusal vocabulary** (§7.1-§7.3 generation-time codes). Terminal candidate-action, acceptance and recovery refusals (§7.4) are command refusals with exit codes and do **not** fall through |
| opt. 13 | The GH #15 ownership quote was partial | §6.16 and §11 quote `docs/prds/PRD-recipe-generation-authority.md:3674-3678` in full, including P6 and P7, and state that GH #13 v1 regenerates **only** on a successful operation-candidate accept and auto-repairs no ineligible input |
| further | The confirmation gate's own evidence entry was unclassified | §6.17 classifies the gate's shipped `phase-3.5` / `manual-review` entry (`internal/workflow/reconcile.go:1069-1070`) as **supporting**, retained unchanged, while the phase-2 proof entry is verdict-bearing |
| further | Staleness ordering and accepted audit directories | §6.11: the staleness check runs before phase 1 and phase 1.5; an `accepted` directory with no live pointer is ignored by the preflight |
| further | Candidate-ready finalization ordering | §6.2: finalization stays conflict-gated — it happens after phase 3 and after phase 4's forward-apply preview, only on the arms that were going to return `blocked` |
| further | P3 variant table and transaction shape | §6.16's variant table and §6.14's fourteen steps are unchanged except for the new base-commit and feature-state fields |

## 0.3 Review response — rev-2 findings

rev-2's eligibility order, alignment mathematics, acceptance transaction,
coverage publication, lock model and feature-state binding were **not**
challenged and are carried forward unchanged except where a row below names an
edit. Every rev-2 finding, the adjudication applied, and where rev-3 discharges
it. **Every row is a change to this document or to ADR-037; none is deferred.**

| # | rev-2 finding | rev-3 resolution |
|---|---|---|
| R1 | CG5-CG7 name fields the new phase-2 verdict entry never explicitly emits — the conditions read as if some entry already carried `match_origin: upstream`, `pre_reconcile_presence: present` and a populated `upstream_commit_refs`, while the only shipped `recipe-operation-match` builder sets `unknown` / `[]` / `low`; and the proof had no defined route from the evaluator to the gate, because the gate is called from inside `saveReconcileArtifacts` | §6.2 names the subject explicitly: CG5-CG7 grade the **new GH #13 phase-2 verdict-bearing `recipe-operation-match` attempt**, whose builder sets `confidence: high`, `match_origin: upstream`, `pre_reconcile_presence: present`, `upstream_commit_refs: [upstreamCommit]` and `upstream_commit == upstreamCommit`, and only after E1-E15 and the all-present classification have both passed. The shipped legacy builder (`internal/workflow/reconcile.go:843-845`) is untouched and keeps `low` / `unknown` / `[]`. The plumbing is stated in full: `saveReconcileArtifacts` (`:724`) takes a fourth `proof *phase2AuthorityProof`, all **ten** shipped call sites (`:383,432,455,480,524,536,564,570,583,596`) pass `nil` — `:570`'s third argument being spelled `phase35`, though it names the same object as `result` (rev-6) — and only the GH #13 all-present path passes the minted proof through to `applyUpstreamedConfirmationGate` (`:771`). *(rev-3 enumerated nine sites and missed `:570`; rev-4 corrects the count and the list everywhere.)* The gate cross-checks the proof against **that exact entry**, and no proof is ever reconstructed from stored bytes. New rows ROC-281, ROC-282 and ROC-283 pin the positive fields, the plumbing and the anti-reconstruction guard |
| R2 | The evidence model calls four phase-3.5 entries supporting although one is the main verdict, and the confirmation entry is mutually exclusive with the blocked/path cases | §6.17 corrects the taxonomy and every count: the shipped maximum at `phase-3.5` is **one verdict-bearing entry plus three supporting detector entries**, and at `phase-4` **one verdict-bearing plus one supporting**. The gate's `manual-review` entry requires an `upstreamed` outcome at `:771` (`:1033-1035`) while path-restructure (`:769`, guard `:947-949`) and blocked-classification (`:770`, guard `:998-1000`) require a blocked one, so the gate entry never coexists with either. Candidate-ready does not claim or force path-restructure or blocked-classification support: both are blocked-only and a candidate-ready invocation returns `store.ReconcileStillNeeded`. Detector semantics are unchanged, and the candidate's diagnostic richness is delivered by `candidate.json`, its `effect_map`, `candidate.patch` and the phase-2 verdict entry — measured by **semantic field**, never by detector line count. ROC-208, ROC-211, ROC-219, ROC-220 and ROC-221 are corrected and ROC-284 is added |
| R3 | Ordinary `updateFeatureState` paths can erase a pending candidate pointer — it replaces `status.Reconcile` wholesale (`:1191-1209`) | §6.11 adds the carry-forward contract: a **shared status merge** copies the loaded `Reconcile.OperationCandidate` into the newly built `ReconcileSummary` **before** it is assigned, on **every** non-candidate arm, so a pending candidate stays addressable through a later phase-1, phase-1.5, provider or phase-4 outcome, a gate-off run, a multi-slug run and a lock refusal. Only the five explicit candidate transitions — finalize, stale, reject, accept and recovery — may set, modify or clear the pointer, and the pointer and the on-disk directory stay aligned. The carry-forward is **not** a lifecycle-state change: `status.State` and `status.Notes` are untouched by it and D20b's bypass is unaffected. A source guard over every `store.ReconcileSummary` construction and rows ROC-285, ROC-286 and ROC-287 pin it |
| opt. 1 | Acceptance's CAS is described as the "last word" and reads as if it made acceptance authoritative across hosts | §6.14: the CAS is re-validated over **every** candidate path immediately before the first live write, refusing with the same `candidate-cas-failed`; the advisory locks serialize `tpatch` invocations **on one host only**; and no surface claims that an external or cross-host writer mutating a path after the recheck is prevented. The journal and rollback are named as the **recovery** answer for that case, not a prevention claim. ROC-288 pins the recheck and the absence of the claim |
| opt. 2 | §9.13's rev-2 addition itemization sums to twenty-nine against a real delta of twenty-seven | The totals section (§9.14 at rev-3, renumbered §9.15 when rev-4's rows became §9.14) corrects the itemization to the real delta: four carrier rows, not five, and five lock rows, not six — the two extra were rev-1 rows **corrected** in rev-2, not added. rev-3's own delta is itemized the same way and re-checked against the totals table |
| opt. 3 | The `SPEC.md` amendment names only the conflict row | §6.3 names **all three** phase-2 bullets the amendment rewrites: the all-present arm becomes a **CG-qualified** confirmation rather than an unconditional `UPSTREAMED`; the conflict arm is a **nonterminal classification**, not an overall return; and the mixed arm surfaces a candidate **only at a phase-4 textual conflict**, otherwise yielding to the stronger later outcome. §8's S9 edits all three |
| opt. 4 | The source-derived flag enumeration would sweep in Cobra's auto-registered `help` flag | §6.12: `help` is excluded **by name** from the behavioral set, because Cobra registers it during execution rather than at `reconcileCmd` construction and it is not an operator behavior switch. The behavioral set stays exactly the fifteen flags registered at `internal/cli/cobra.go:2520-2542`, so the count remains `5 × 15 = 75` plus `10`, **85**. ROC-289 pins the exclusion, and a build that counts `help` reports `90` and fails |
| opt. 5 | The rollback snapshot list was not exhaustive | §6.14 step 8 names the set: `artifacts/post-apply.patch`, `apply-recipe.json`, `recipe-provenance.json`, `recipe-coverage.json`, `patch-generations.json` and `status.json`, plus the pre-transaction `status.Apply` / `status.State` values and an explicit absent marker for every created path and every artifact that does not exist yet. ROC-250 and ROC-251 are corrected to the same list |
| opt. 6 | The parity blocks had no machine-extractable boundaries | §6.0 defines the delimiters: every block is wrapped in `<!-- parity-block:<ID>:begin -->` / `<!-- parity-block:<ID>:end -->` in **both** documents, the guard compares **only** the bytes between a matching pair, and a missing, duplicated or unpaired marker fails the check. The enclosed bytes are unchanged and remain identical; the nine marked regions are A, B, C1, C2, D1, D2, D3, E and F |
| opt. 7 | "At least as rich as the phase-4 path" was measured in detector entries | §6.17 restates the claim over **semantic fields** — every effect, its path, its resolution and its reason codes, plus the phase-2 verdict entry's own fields — and explicitly disclaims equality of evidence line counts, which would require forcing blocked-only detectors to run on a non-blocked outcome |

## 0.4 Review response — rev-3 findings

rev-3's confirmation-gate authority class CG1-CG7, evidence taxonomy,
candidate-pointer carry-forward, alignment mathematics, gate order, lock model
and acceptance / publication contracts were **not** challenged and are carried
forward unchanged except where a row below names an edit. Every rev-3 finding,
the adjudication applied, and where rev-4 discharges it. **Every row is a
change to this document or to ADR-037; none is deferred.**

| # | rev-3 finding | rev-4 resolution |
|---|---|---|
| F1 (blocking) | The proof-plumbing claim enumerates **nine** `saveReconcileArtifacts` call sites and omits `internal/workflow/reconcile.go:570`. The shipped function has **ten**, and `:570` is the one whose third argument is `phase35` rather than `result` (`:569-571`), so "all nine shipped call sites pass `nil`" is both under-counted and silent about the one call that differs | Every nine-call claim is replaced with the **ten** production call sites `internal/workflow/reconcile.go:383,432,455,480,524,536,564,570,583,596` — in parity block A (byte-identically on both sides), in §0's revision history, in §0.3's R1 row, in §8's S6 slice, in ROC-282 and in §12's claims audit, and in ADR-037's revision history, D34, implementation order and references. `:570` is named explicitly: it passes `phase35` and — exactly like every existing and legacy call — a **`nil`** proof, and it stages no attempt. *(rev-4 also called `phase35` "the distinct `*ReconcileResult` returned by `tryPhase35`"; rev-6 corrects that to a distinct variable name bound to the same object, since `tryPhase35` mutates and returns the `result` it receives.)* Only the GH #13 all-present path passes a minted proof |
| opt. 1 *(rev-4; superseded by rev-5 B1)* | The new phase-2 verdict-bearing attempt had no defined relationship to the entry `persistReconcileEvidence` derives at `:766`, so an implementation could emit **both** and write two verdict-bearing `phase-2` lines — which §6.17 itself calls a defect | §6.2 (parity block A) fixes the route and the substitution. The evaluator builds the attempt in the **same statement** that mints the proof and stages it on the result in one new unexported field `phase2Verdict *store.ReconcileEvidence` (`:19-85`), the shipped unexported `attemptedAt` (`:74-84`) being the precedent and `encoding/json`'s treatment of unexported fields the reason `reconcile-session.json` (`:775-776`) is unaffected. The pair is all-or-nothing — `proof != nil` **iff** `result.phase2Verdict != nil`, either alone being treated as no proof at all — and when it is present `persistReconcileEvidence` appends the staged attempt **in place of** the derived entry, so exactly one verdict-bearing `phase-2` line exists, in the shipped `:766` position and append order. Legacy and non-authoritative arms stage nothing and keep the legacy builder verbatim. §6.17 adds the per-arm table showing the collision is possible on exactly one arm — the all-present return (`:451-457`) — which is the arm that substitutes. The route on `ReconcileResult` was chosen over a second `saveReconcileArtifacts` parameter because it makes pairing a proof minted for one result with an entry built for another structurally impossible; the **proof** keeps its parameter-only route and is still never a `ReconcileResult` field |
| opt. 2 *(rev-4; completed by rev-5 B3, which cleared the two remaining occurrences)* | The gate was said to cross-check the proof against "the exact verdict-bearing entry it is **about to write**", but the gate does not write that entry: it is appended at `:766`, before the gate runs at `:771`; the only line the gate writes is its own supporting `manual-review` entry | Parity block A now reads "the exact verdict-bearing entry **it is grading**" — `result.phase2Verdict`, already in `result.Evidence` — and states explicitly that the gate does **not** write the entry it grades: the only line it appends is its separate supporting `phase-3.5` / `manual-review` entry (`:1033-1035,1069-1070`), unchanged in phase, kind, fields and firing condition. §6.17 and D25 carry the same statement |
| opt. 3 *(rev-4; extended by rev-5's ROC-292 and ROC-293)* | Neither the positive shape nor the anti-duplicate case was pinned by a matrix row or by a source guard | §9.14 adds **ROC-290** (positive: exactly one verdict-bearing `phase-2` line, carrying ROC-281's pinned fields, in the `:766` position, with the legacy `low` / `unknown` / `[]` derivation absent and the gate grading rather than writing it) and **ROC-291** (`G`: the `legacy-phase2-entry-appended-beside-staged-attempt`, `staged-attempt-appended-without-substitution`, `staged-attempt-without-proof`, `proof-without-staged-attempt` and `phase2-verdict-assigned-outside-evaluator` wrong-input fixtures, each failing the same validator the correct configuration passes; *rev-5 renames the last of those to `verdict-attempts-assigned-outside-evaluator`, after the ordered field that replaced the singular one, and adds `proof-paired-with-candidate-ready-pair`*). §6.2's source guard is extended to the staged attempt's **one** assignment site and **one** read site, and ROC-283 is corrected so the permitted unexported carrier is distinguished from the forbidden exported one |

## 0.5 Review response — rev-4 findings

rev-4's ten-site proof route, the CG1-CG7 authority class, the evidence
taxonomy, the candidate-pointer carry-forward, the alignment mathematics, the
gate order, the lock model and the acceptance / publication contracts were
**not** challenged and are carried forward unchanged except where a row below
names an edit. Every rev-4 finding, the adjudication applied, and where rev-5
discharges it. **Every row is a change to this document or to ADR-037; none is
deferred.**

| # | rev-4 finding | rev-5 resolution |
|---|---|---|
| B1 (blocking) | Candidate-ready stages no attempt while its public phase still maps to phase-2, so the legacy builder derives a **duplicate** `phase-2` verdict entry and the arm has **no carrier** for the phase-4 textual-conflict verdict its own worked example requires. rev-4's single-valued `phase2Verdict` could not express two attempts, and its biconditional bound a proof to any staged attempt | The carrier becomes one unexported **ordered** `verdictAttempts []store.ReconcileEvidence` field on `ReconcileResult` (`internal/workflow/reconcile.go:19-85`), still ignored by `encoding/json` on the `attemptedAt` precedent (`:74-84`), and §6.2 (parity block A) pins **three** legal configurations. **Configuration 1** — every other and legacy arm — stages nothing and runs the shipped single derived builder unchanged. **Configuration 2** — the authoritative all-present arm — stages exactly **one** qualifying phase-2 verdict-bearing `recipe-operation-match` attempt, built in the same statement that mints the proof; `persistReconcileEvidence` (`:766`) emits the staged list **in place of** its derived entry, and the gate grades that exact entry and appends only its own supporting `manual-review` line. **Configuration 3** — candidate-ready — stages exactly **two** ordered attempts before save: `[1]` `phase-2` / `recipe-operation-match` / `still_needed` / `high` / `match_origin: upstream` / `pre_reconcile_presence: present` / `upstream_commit_refs: [upstreamCommit]` / `requires_confirmation: true` / `reason_code: phase-2-operation-candidate`, and `[2]` `phase-4` / `forward-apply` / `blocked` / `low` / the shipped `:826-829` defaults / `reason_code: phase-4-forward-apply-conflicts` or `phase-4-blocked`. Because configuration 3 substitutes, `result.Phase` may remain `phase-2-operation-candidate` with **no** duplicate line. The biconditional is re-scoped: `proof != nil` **iff configuration 2** — candidate-ready therefore has a `nil` proof beside two staged attempts, and a proof beside the two-attempt list is a defect. **No arm emits staged and derived together**, and any illegal configuration is treated as no proof and no staging at all. §6.2's source guard is widened to **six** axes: one assignment site, one mutation site, one persist read site, no serialization of any kind, pairwise-distinct `attempt_id`s, and strictly increasing phase order. §6.17 and D25 name the two routes a phase-2 finding can take — **staged substitution** on the two arms whose terminal `result.Phase` derives `phase-2` (`:1167-1168`), and **direct append** on the fallthrough arms where no collision is possible — and their examples, per-arm table and confidence ladder follow. ROC-038, ROC-175, ROC-208, ROC-209, ROC-210, ROC-211, ROC-221, ROC-281, ROC-283, ROC-290 and ROC-291 are corrected, and **ROC-292** (positive: two ordered attempts with exact fields, derived entry absent, proof `nil`, distinct IDs) and **ROC-293** (`G`: unsuppressed derivation, missing phase-4 attempt, out-of-phase order, shared `attempt_id`, invocation-outcome verdict) are added |
| B2 | §9's preamble says the matrix runs `ROC-001` … `ROC-289` while rev-4's own §9.14 added ROC-290 and ROC-291 | The preamble now reads `ROC-001` … **`ROC-293`**, recounted **after** rev-5's two additions rather than before them, and §9.15's totals, kind counts and delta itemization are recomputed against the same number: 293 rows, I 59, C 85, G 74, U 66, S 9 |
| B3 | Two gate descriptions still say the graded entry is the one the gate is **about to write** / **being written**, although the gate never writes it | Every remaining occurrence is replaced with "the exact verdict-bearing entry **it is grading**": §8's S6 slice and ROC-038 join parity block A's rev-4 correction, and each now also states that the only line the gate itself appends is its **separate supporting** `phase-3.5` / `manual-review` entry (`:1033-1035,1069-1070`), unchanged in phase, kind, fields and firing condition. The only surviving "about to write" strings are the historical citations inside §0's and §0.3-§0.4's rev-3 / rev-4 audit rows, which quote the superseded wording on purpose |
| opt. (carve-out) | §6.0 illustrates the delimiter with a literal `<ID>` template but, unlike ADR-037 D36, never says the template is not itself a region — so a naive marker parser would read it as an unpaired `begin` | §6.0 gains the carve-out explicitly, and **both** documents now state the mechanism rather than just the conclusion: the marker parser recognizes a marker **only** when its ID matches the closed nine-value set `A`, `B`, `C1`, `C2`, `D1`, `D2`, `D3`, `E`, `F`, so it skips the `<ID>` illustration in §6.0 and the identical one in D36. A parser that accepts an arbitrary ID would fail the check on a document that breaks nothing |

## 0.6 Review response — rev-5 findings

rev-5's ordered `verdictAttempts` carrier, its three legal configurations, the
CG1-CG7 authority class, the ten-site proof route, the evidence taxonomy, the
candidate-pointer carry-forward, the alignment mathematics, the gate order, the
lock model and the acceptance / publication contracts were **not** challenged
and are carried forward unchanged except where a row below names an edit. Every
rev-5 finding, the adjudication applied, and where rev-6 discharges it. **Every
row is a change to this document or to ADR-037; none is deferred.**

| # | rev-5 finding | rev-6 resolution |
|---|---|---|
| blocker | Both documents describe `internal/workflow/reconcile.go:570`'s third argument as `phase35`, "the **distinct** `*ReconcileResult` returned by `tryPhase35`", "rather than the pipeline's `result`". That is wrong about the shipped source: `tryPhase35` takes the pipeline's `result` as its **ninth parameter** and mutates and returns the **same pointer**, so `phase35` is a distinct variable *name*, not a distinct object — and the wording implies `:570` is materially different from the other nine call sites when it is not | The source truth is stated everywhere the claim appears: `tryPhase35` (`:569`, defined `internal/workflow/reconcile.go:1354-1364`) takes `result` as its ninth parameter (`:1363`), mutates it in place and returns **that same pointer** from every one of its ten returns, each literally `return result` (`:1373,1381,1389,1436,1475,1491,1497,1502,1508,1513`); `phase35` is therefore a distinct **variable name** bound to the **same object**, and `:570` differs from the other nine sites in argument spelling only. Like every existing and legacy call it passes a **`nil`** proof and stages no attempt; only the GH #13 all-present path passes a minted proof. Updated in **parity block A** (byte-identically on both sides), in §0's rev-4 and rev-6 rows, in §0.3's R1 row, in §0.4's F1 row, in §8's S6 slice, in ROC-282 and in §12's claims audit — whose phase-3.5 row is rewritten against the real signature and returns — and in ADR-037's revision history, parity block A, D34, implementation order and references. Nothing else about the plumbing changes: the ten-site inventory, the proof's parameter-only route and the three staging configurations stand |
| note 1 | ROC-290 asserted that a gate-off legacy all-present run **and** a gate-on fallthrough are each "byte-identical to the S0 golden". Only the first is: the gate-on arm's phase-2 line is the evaluator's directly appended entry, which S0 does not contain | ROC-290 scopes the S0 byte-identity assertion to the **gate-off** arm alone (§6.17's per-arm table row 2). The gate-on legacy or E-gate-ineligible all-present result takes row 4's **direct-append** route: it retains **one verdict-bearing entry per phase**, but its `phase-2` line is an **intentional** evidence delta against S0, not a byte-identical match. A fixture asserting S0 byte identity on the gate-on arm now fails the row, and so does one asserting that arm emits no `phase-2` line |
| note 2 | ROC-281's "in the same repository a run that does not reach the all-present result still emits the shipped legacy `recipe-operation-match` entry at `low` / `unknown` / `[]` … byte-identical to the S0 golden" is unscoped, and reads as if it held with the gate on | ROC-281 scopes that clause to `operation_candidate_enabled` **off** — §6.17's per-arm table row 2 — and states the alternative explicitly: with the gate **on**, an ineligible or legacy all-present result falls through and its `phase-2` line is the evaluator's directly appended entry (row 4), so the S0 byte-identity claim belongs to the gate-off arm and to nothing else |
| note 3 | Neither document says whether the **direct-append** route writes the in-memory `result.Evidence` slice as well as `reconcile-evidence.jsonl`. Since `ReconcileResult` is serialized whole into `reconcile-session.json`, an implementation that wrote only the JSONL would make the two artifacts diverge | §6.17 and ADR-037 D25 pin the dual-sink invariant for **both** routes: every verdict-bearing attempt is appended to `reconcile-evidence.jsonl` (via `store.AppendReconcileEvidence`, `internal/workflow/reconcile.go:876`) **and** to `result.Evidence`, in the same relative order. Staged substitution inherits this from the shipped shape — `:766` appends `persistReconcileEvidence`'s returned entries (`:880`) to `result.Evidence` — and the direct-append route must do it explicitly at phase-2 time. `reconcile-session.json`'s serialized `evidence` array (`:775-776`) is therefore never a strict subset of the JSONL, and neither route may write one sink without the other or in a different order. **ROC-208 is amended** to pin it at runtime — the existing row already discriminates the two routes, so no new row is required — and a direct-append implementation that omits the `result.Evidence` half fails it |

## 1. Summary

`tpatch reconcile`'s phase 2 is supposed to answer one question deterministically
and for free: *are this feature's changes already in the new upstream, and if
only some of them are, what would it take to reproduce the rest?*

Today it answers neither half well. It inspects whatever happens to be checked
out (`internal/workflow/reconcile.go:450`), matches by substring and
whitespace-trimmed equality (`internal/workflow/reconcile.go:625,637`), produces
a single boolean (`internal/workflow/reconcile.go:654`), and when the answer is
"some yes, some no" it prints a count line and throws the computation away
(`internal/workflow/reconcile.go:459-462`).

GH #15 / ADR-036 rev-7 has just made a real answer possible. Its
`artifacts/recipe-coverage.json` binds the canonical patch and the recipe to a
named reference commit with strict, recomputable, per-effect hashes, and it
deliberately hands this consumer **no** anchor
(`docs/adrs/ADR-036-recipe-coverage-authority.md:1518-1541`) and **no**
authority (`docs/adrs/ADR-036-recipe-coverage-authority.md:1227-1268`).

This PRD specifies the consumer. Phase 2 independently recomputes every ADR-036
binding, evaluates a **single resolved upstream commit**, and — for a bounded
domain of purely additive text effects — derives and proves an
**all-or-nothing reviewable candidate** in memory. The candidate is executed,
validated and persisted only when a later phase proves the ordinary textual
path cannot answer, and it is then held for explicit human acceptance through a
journaled, rollback-capable transaction that publishes complete coverage on
success.

Three properties define the scope:

- **It is contract fidelity, not a new feature surface.** No new command, no
  phase 2.5, no new lifecycle state. The four-phase order and the
  provider/forward-apply fallback are preserved exactly.
- **Every phase-2 refusal falls through.** Ineligible coverage, an unsupported
  effect, an ambiguous alignment, a multi-slug run — each records phase-2
  evidence and continues into phase 3/4. Phase-2 `BLOCKED` is a phase
  classification, never a forced overall return. The rule is scoped to the
  **phase-2 refusal vocabulary** (§7.1-§7.3): a terminal candidate action, an
  acceptance refusal and a recovery refusal (§7.4) are command refusals with
  their own exit codes, and they do not fall through to anything.
- **No producer claim is ever trusted.** `coverage_status: complete`,
  `producer: record`, `cross_base_status` and every verify warning are context.
  Authority is recomputed, or it does not exist.

## 2. Problem

### 2.1 Phase 2 grades against whatever is checked out

`reconcileFeature` calls `evaluateRecipeOperations(s.Root, recipe.Operations)`
(`internal/workflow/reconcile.go:450`). Inside, every read is
`os.ReadFile(filepath.Join(repoRoot, op.Path))` with `repoRoot == s.Root`
(`internal/workflow/reconcile.go:617-618,635-636`). There is no `git show`, no
`git cat-file` and no tree read anywhere in the phase.

The consequences are not theoretical. `--allow-dirty` exists precisely to let
an operator reconcile with local modifications
(`internal/cli/cobra.go:2523`), and its own help text concedes "verdicts may be
wrong". A verdict that depends on the checkout cannot be re-proved later, which
means it cannot back a write.

### 2.2 Substring presence and trimmed equality are not authority

- `replace-in-file` presence is `strings.Contains(contentStr, op.Replace)`
  (`internal/workflow/reconcile.go:625`) — a first-match substring anywhere in
  the file, with no uniqueness requirement.
- `write-file` presence is
  `strings.TrimSpace(string(content)) == strings.TrimSpace(op.Content)`
  (`internal/workflow/reconcile.go:637`) — leading and trailing whitespace
  differences are silently equal.
- `ensure-directory` is counted **present** unconditionally
  (`internal/workflow/reconcile.go:646-647`), with no directory check at all.
- Unknown operation types are counted **applicable**
  (`internal/workflow/reconcile.go:649-650`) — the fail-open default on exactly the
  input that should refuse. `append-file` is a real recipe type
  (`internal/workflow/implement.go:50`) that phase 2 does not handle, so it
  lands here.

By contrast the recipe executor's own switches fail **closed** on unknown types
(`internal/workflow/recipe.go:194-195,242-243`). Phase 2 is the outlier.

### 2.3 The SPEC's phase-2 vocabulary does not exist in code

`SPEC.md:444-448` specifies:

```
Phase 2: Operation-Level Evaluation (deterministic, no LLM)
    → All already-present → UPSTREAMED
    → Any conflict → BLOCKED
    → Mix → STILL-NEEDED
```

The code produces `operationEvalResult{allPresent, hasConflicts, presentCount,
applicableCount, conflictCount}` (`internal/workflow/reconcile.go:603-609`) and
one derived boolean (`internal/workflow/reconcile.go:654`). Only the
`allPresent` arm has an outcome (`store.ReconcileUpstreamed`,
`internal/workflow/reconcile.go:452`). The conflict arm appends a note and falls
through (`internal/workflow/reconcile.go:459-462`), and the "mix" arm has no
representation at all.

**The shipped fallthrough is correct and the SPEC prose is wrong.** Making
phase-2 conflicts terminal would delete working phase-4 reapply behavior for
every feature whose recipe drifted. §6.3 amends the SPEC rather than the code.

### 2.4 The most valuable phase-2 result is the one that is discarded

A feature where three of five effects are already upstream and the other two
insert cleanly is the exact case an operator wants a diff for. Today it produces
`Operation-level: 3 present, 2 applicable, 0 conflicts`
(`internal/workflow/reconcile.go:460-461`) and nothing else — and only when
`hasConflicts` is true, so a clean mix prints nothing whatsoever.

### 2.5 The resolver accept path cannot be reused

`AcceptShadow` (`internal/workflow/accept.go:82`) has five properties that are
each independently disqualifying for a machine-derived candidate:

1. Step 1 re-applies the **stale original patch** minus the resolved paths
   (`internal/workflow/accept.go:95`). A candidate exists precisely because that
   patch no longer applies.
2. Step 2 is a **file-only copy** (`gitutil.CopyShadowToReal`,
   `internal/gitutil/shadow.go:191`), whose own doc comment says a caller must
   wrap it to get atomicity (`internal/gitutil/shadow.go:189-191`).
3. It is **not atomic and has no rollback**: a mid-flight failure returns an
   error with the shadow still in place
   (`internal/workflow/accept.go:95-131`).
4. Its scope is **partial** — it accepts the resolver's `files` list, not the
   feature's effect set.
5. `RefreshAfterAccept` (`internal/workflow/refresh.go:45`) writes
   `post-apply.patch` unconditionally (`internal/workflow/refresh.go:82`) and
   **deliberately leaves `apply-recipe.json` stale**
   (`internal/workflow/refresh.go:20-24`), which is why ADR-036 D15 classes P3
   as an inherently incomplete-coverage producer.

The shadow **lifecycle** is equally wrong for a candidate: `CreateShadow` reaps
priors on every create (`internal/gitutil/shadow.go:56,72`), `ResolveShadow` is
newest-wins (`internal/gitutil/shadow.go:115`), and `PruneAllShadows` clears the
lot (`internal/gitutil/shadow.go:152`). A candidate must be stable while a human
reads it.

### 2.6 A fallthrough loses the phase-2 finding entirely

`persistReconcileEvidence` (`internal/workflow/reconcile.go:766`, defined
`:812`) derives one phase from `result.Phase` through `evidencePhaseAndKind`
(`internal/workflow/reconcile.go:1161-1176`). When phase 2 classifies a
conflict and phase 4 later reapplies cleanly, the persisted evidence says
`phase-4` and the phase-2 finding is gone.

The pipeline already appends up to six entries per invocation
(`internal/workflow/reconcile.go:766-771`) and the JSONL writer already
de-duplicates by content-addressed `attempt_id`
(`internal/store/reconcile_evidence.go:166-171`), so multi-entry is the shipped
shape. **Several of those entries already share a phase**: file-novelty
(`internal/workflow/file_novelty.go:115-116`), hunk-overlap
(`internal/workflow/hunk_overlap.go:135-136`), path-restructure
(`internal/workflow/path_restructure.go:118-119`) and the confirmation gate
(`internal/workflow/reconcile.go:1069-1070`) all write `phase-3.5`, and
blocked-classification (`internal/workflow/blocked_taxonomy.go:179`) writes
`phase-4` beside whatever `persistReconcileEvidence` derived. What is missing
is an ordered attempt list and a stated distinction between the entries that
carry a phase's verdict and the entries that support it (§6.17).

### 2.7 The all-present arm is not terminal today — the confirmation gate demotes it

This is the fact rev-0 got wrong, and it changes the shape of the whole
all-present story.

`applyUpstreamedConfirmationGate` (`internal/workflow/reconcile.go:1032`, called
at `:771`) runs on **every** `store.ReconcileUpstreamed` result. It confirms on
exactly two inputs:

- an evidence entry with `evidence_kind: patch-id-match` **and** a non-empty
  `matched_upstream_sha` (`internal/workflow/reconcile.go:1047-1054`);
- an evidence entry with `evidence_kind: reverse-apply` at
  `confidence: high` (`internal/workflow/reconcile.go:1055-1060`).

A `recipe-operation-match` entry — the *only* kind phase 2 can produce
(`internal/workflow/reconcile.go:1167-1168`) — confirms nothing. So the
unconfirmed branch fires: `result.Outcome` is overwritten with
`store.ReconcileBlocked` and `result.ReviewVerdict` becomes
`rejected-upstreamed` (`internal/workflow/reconcile.go:1089-1091`), and
`persistRevisionPassLog` records `final_feature_state: blocked`
(`internal/workflow/reconcile.go:1096-1112`).

This is pinned by an integration test, not inferred:
`TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch`
(`internal/workflow/reconcile_evidence_integration_test.go:459`) builds an
all-present operation fixture (`buildOperationUpstreamedCandidateFixture`,
`:235`) and asserts `outcome == blocked`, `review_verdict ==
rejected-upstreamed`, gate reason `missing-upstream-commit-ref`, and a
persisted `blocked` feature state.

The gate is correct and its motivation is documented: a false `upstreamed`
silently drops local feature code, and two real features were lost that way
(`docs/prds/PRD-upstreamed-confirmation-gate.md:57-70`). What is missing is an
authority class strong enough to satisfy it deterministically. §6.2 adds
exactly one, with seven independently recomputed conditions.

### 2.8 There is no coverage code to consume yet

A search of `internal/` and `cmd/` for `recipe-coverage.json`, `recipe_coverage`,
`CoverageProducer`, `RecipeCoverage` and `reconcile-accept` returns **zero**
matches. ADR-036 exists only as documentation. Every claim this PRD makes about
coverage is a claim about a **contract to be shipped in v0.17.0**, not about
code in the tree, and §8's slices are blocked accordingly.

## 3. Goals

1. Phase 2 evaluates exactly one resolved upstream commit and never inspects the
   live worktree for eligibility, alignment or classification.
2. Phase 2 independently recomputes every ADR-036 binding and refuses on
   anything a producer merely asserts.
3. The all-present case reaches terminal `upstreamed` **without human
   confirmation only** when a new, strictly-qualified authority class is
   satisfied (§6.2); every weaker or legacy operation match keeps today's
   confirmation requirement, including its `blocked` / `rejected-upstreamed`
   demotion.
4. A proven candidate is surfaced as terminal `still_needed` with a
   **reviewable, machine-proved artifact** — but only where the shipped
   pipeline would otherwise have returned a phase-4 textual `blocked`, and
   never when `--resolve` was explicitly requested.
5. Every other case is a **nonterminal refusal** that preserves the existing
   provider and forward-apply behavior byte-for-byte.
6. Candidate derivation is bounded, decidable and free of tuning parameters:
   exact creations and additive-only text modifications, exact line-record
   alignment, unanimous-or-refuse.
7. Candidate execution is all-or-nothing in an isolated worktree, validated by
   postconditions plus a full second-pass idempotency proof.
8. Candidate alignments are ephemeral; no artifact carries a record body, a
   record index, an offset or a source body.
9. Acceptance is a new, journaled, rollback-capable, explicitly recoverable
   transaction that never reuses the resolver accept path.
10. Successful acceptance publishes **complete** coverage through ADR-036's
    shared API, so a feature does not degrade with each reconcile.
11. Phase-2 evidence and terminal-phase evidence coexist in phase order, each
    carrying its own phase-local verdict, and no shipped detector's firing
    condition, phase or kind changes.
12. Candidate diagnostics are at least as rich as the phase-4 path they replace,
    measured in **semantic fields** — every effect, its path, its resolution and
    its reason codes — not in evidence-line counts.
13. A pending candidate stays addressable: its `status.json` pointer is carried
    forward through every reconcile outcome that is not an explicit candidate
    transition, and pointer and directory never disagree.
14. The legacy operation evaluator is retired as an authority: with the feature
    gate on it produces diagnostics only, and it can never mint a terminal
    verdict or a candidate.

## 4. Non-goals

1. A new command, a phase 2.5, or any new lifecycle state.
2. General patch commutation, fuzzy matching, similarity scoring or search.
3. Replacement (`-` line), delete, rename, copy or binary effects.
4. Executable, symlink, gitlink or any mode-changing effect.
5. Multi-slug or `--cumulative-legacy` candidates.
6. Provider calls anywhere in phase 2.
7. Test execution during candidate generation.
8. Any Git history mutation — no commit, amend, rebase, reset or ref move.
9. Persisted alignments, record indices or offsets of any kind.
10. Filesystem-wide atomic acceptance, or any claim of it.
11. Changing ADR-036's coverage schema, its `producer` enum or its
    `schema_version`.
12. Changing ADR-029's supersession severity or ADR-030's multi-slug semantics.
13. Weakening the confirmation gate, ADR-036's strict decoder, or any existing
    refusal.
14. Re-routing existing reconcile flag combinations through a stricter
    validator, or changing their shipped error message bytes.

## 5. Existing primitives pre-flight

Every row is a primitive that could plausibly carry part of this contract, and
the exact reason it cannot.

| Primitive | What it already proves | Why it is insufficient |
|---|---|---|
| `evaluateRecipeOperations` (`internal/workflow/reconcile.go:611`) | A three-way present/applicable/conflict count exists in shape | Reads `s.Root` (`:617-618,635-636`); substring presence (`:625`); trimmed equality (`:637`); unconditional `ensure-directory` present (`:646-647`); unknown types counted applicable (`:649-650`) |
| `ExecuteRecipe` (`internal/workflow/recipe.go:90`) | Ordered execution with a preimage precheck | Collects per-op errors and **continues** (`:102-108`); not all-or-nothing at the operation layer |
| `dryRunOperation` / `executeOperation` (`internal/workflow/recipe.go:128,199`) | Path-safe operation execution | Hardcode `repoRoot := s.Root` (`:129,200`); fixed `0o644` write (`:211`); first-match replace (`:223`) and unconditional append (`:235-241`); both switches fail closed only at `:194-195,242-243` |
| `ApplyRecipe` / `RecipeOperation` (`internal/workflow/implement.go:43,49`) | Four persisted operation types plus `preimage_hash` | No anchor, no uniqueness claim, no boundary concept; ADR-036 D8 persists none |
| ADR-036 `recipe-coverage.json` | Strict effect enumeration bound to a named base | Explicitly **not** replay eligibility (ADR-036 D4); withholds cross-base authority by design |
| `contextual_hint` (ADR-036 D3) | Advisory additive-text hint | Advisory only; never changes `coverage_status` or `cross_base_status` |
| `AcceptShadow` (`internal/workflow/accept.go:82`) | An accept transition exists | Stale-patch reapply, file-only copy, no atomicity, no rollback, partial scope (§2.5) |
| `CopyShadowToReal` (`internal/gitutil/shadow.go:191`) | Safe per-file shadow→real copy | Files only; its own doc defers atomicity to the caller |
| `RefreshAfterAccept` (`internal/workflow/refresh.go:45`) | Post-accept patch regeneration | Leaves the recipe stale by design (`:20-24`); produces incomplete coverage by construction |
| `CreateShadow` / `ResolveShadow` / `PruneAllShadows` (`internal/gitutil/shadow.go:56,115,152`) | Detached worktree creation and cleanup | Reap-on-create, newest-wins and clear-all are all wrong for a stable reviewable candidate |
| `.tpatch/upstream.lock` guard (`internal/gitutil/lock_guard.go:99-100`) | Baseline-staleness classification | A staleness classifier, **not a mutex**; `--allow-stale-lock` can bypass it |
| `rescap.AcquireLock` (`internal/rescap/lock_unix.go:63`) | A real nonblocking `flock(2)` with no PID protocol | Bound to the resource-capture contract: it runs a `statfs` filesystem preflight (`internal/rescap/lock_unix.go:64-70`), creates the lock inside a scratch root, and its callers must first pass `rescap.EnsureLocalContract` (`internal/cli/feature_resource.go:107`, `internal/rescap/scratch.go:48`). It also refuses outright on every non-linux/darwin target (`internal/rescap/lock_unsupported.go:29-31`). Reconcile needs the same kernel primitive under a different, portable contract (§6.13) |
| `AppendReconcileEvidence` (`internal/store/reconcile_evidence.go:137`) | Append-only JSONL with content-addressed idempotency (`:166-171`) | The caller derives one phase from `result.Phase` (`internal/workflow/reconcile.go:1161-1176`), so a fallthrough loses phase 2 |
| ADR-025 evidence enums (`internal/store/reconcile_evidence.go:27-77`) | Closed `phase` / `evidence_kind` / `confidence` / `match_origin` / `pre_reconcile_presence` vocabularies | Already contain `phase-2`, `recipe-operation-match`, `upstream` and `present`; nothing new is needed, which is why no new value is added |
| `applyUpstreamedConfirmationGate` (`internal/workflow/reconcile.go:1032`) | A gate that refuses to retire a feature on weak evidence | Its confirming set is exactly `patch-id-match` with a `matched_upstream_sha` (`:1047-1054`) and high-confidence `reverse-apply` (`:1055-1060`); a `recipe-operation-match` entry is demoted to `blocked` (`:1089-1090`). Its signature `(s, slug, result)` carries **no** channel for a recomputation fact, and its own evidence entry is `phase-3.5` / `manual-review` (`:1069-1070`). It needs a **new authority class** and a typed carrier, not a bypass (§6.2) |
| `updateFeatureState` (`internal/workflow/reconcile.go:1181`) | The single state/summary persistence point | Its `ReconcileStillNeeded` arm forces `status.State = store.StateApplied` and overwrites `status.Notes` (`:1220-1222`), which a candidate must not do; and it **replaces** `status.Reconcile` with a fresh literal (`:1191-1209`) rather than merging, so any field it does not know about is destroyed on the next reconcile. It needs a shared merge, not a second writer (§6.11) |
| `store.ReconcileSummary` (`internal/store/types.go:372-382`) | An `omitempty` container that already carries optional, phase-specific sub-objects (`ShadowPath`, `PatchIDMatch`, `Labels`) | Every one of those is written by the same invocation that computes it; none has to survive a *later* unrelated invocation, so nothing in the shipped code preserves a field across a rebuild (§6.11) |
| `validateReconcileFlags` (`internal/cli/cobra.go:3200`, rules `:3211-3218`) | Terminal-flag mutual exclusion | Knows three terminals; five more need an additive second stage that leaves the shipped messages (`:3212,3215,3218`) untouched, and its rule set is a literal list rather than an enumeration of the command's flags |
| `RunTestCommandInShadow` (`internal/workflow/validation.go:273`) | Bounded test execution in a shadow | Correct primitive, wrong default — running it during generation would execute operator commands on every reconcile |
| `gitutil.FileAtCommit` / `BlobAtTree` (`internal/gitutil/gitutil.go:803`, `internal/gitutil/trailers.go:737`) | Checkout-free blob reads | Exactly what phase 2 needs and does not currently use |
| `gitutil.ResolveRef` / `RevParse` (`internal/gitutil/gitutil.go:789,854`) | Ref → commit resolution | Present and sufficient |
| `gitutil.NoLazyFetchEnv` (`internal/gitutil/trailers.go:41`) | A shipped `GIT_NO_LAZY_FETCH=1` constant with a test guard precedent (`internal/gitutil/apply_classifier_test.go:319-320`) | Applied only on the trailer/land surfaces today; §6.9 extends the same constant and the same guard shape to every candidate Git subprocess |
| Git tree-hash helper | — | **Does not exist.** No `WriteTree`, `mktree` or `hash-object` wrapper anywhere in `internal/gitutil/`; the §6.8 idempotency proof needs one built |
| `safety.EnsureSafeRepoPath` (`internal/safety/safety.go:12`) | Path containment against one root | Takes a bare `string` root, so it cannot prevent passing the *wrong* root — that needs a type (§6.9) |
| ADR-034 rooted inspection (`internal/rescap`, `os.Root`) | Hostile-tree-safe reads under one held root | The model to follow for reference/candidate reads; not itself a two-root discipline |
| `store.ApplySummary` (`internal/store/types.go:362-369`) | `BaseCommit`, `CompletedAt`, `HasPatch`, `HasRecipe` already exist and are the recorded base for the whole reconcile surface | Nothing writes them on a reconcile path today, and six shipped readers (`internal/workflow/reconcile.go:821,888,917,951,1005,1037`) would silently classify against a stale base after an accept (§6.14) |
| `AppendPatchGenerationForFeature` (`internal/workflow/patch_generations.go:31`) | The shared ADR-024 appender, taking `BaseCommit`, `Upper` and `Capture` as inputs (`internal/store/patch_generations.go:30-67`) | Exactly the right primitive; it just needs to be called with `base_commit = upstream_commit` from a staged, pre-live-write position (§6.14 step 7) |
| `store.FeatureState` (`internal/store/types.go:8-37`) | Twelve closed lifecycle states with a validator | Nothing binds an observed state into an artifact, so a candidate derived at `blocked` cannot detect that the feature became `rejected` before acceptance (§6.6a) |
| `internal/rescap`'s statfs classifier (`internal/rescap/statfs_linux.go:78-95`) | A real filesystem-class check with a named refusal | Bound to `ReasonResourceLockFSUnsupported` and the resource-capture contract; the technique is reusable, the contract is not (§6.13) |

No existing object can carry this responsibility. The six structural gaps —
a typed root so the live tree cannot be targeted, a stable candidate namespace
with its own lifecycle, a portable reconcile mutex that is not bound to the
resource-capture contract, an in-process channel for a recomputation fact that
neither the confirmation gate's `(s, slug, result)` signature nor
`saveReconcileArtifacts`'s can carry, a bound feature state so acceptance can
detect lifecycle drift, and a status merge that preserves a pending candidate's
pointer across unrelated reconciles — are what §6.9, §6.10, §6.13, §6.2, §6.6a
and §6.11 add.

## 6. Product contract

**ADR-037 rev-6 is normative wherever these sections and its decisions
overlap.** Six blocks are reproduced **byte-identically** in both documents and
are covered by a planning parity check at implementation time:

| Parity block | This PRD | ADR-037 | Marked regions |
|---|---|---|---|
| A — confirmation-gate conditions CG1-CG7 and the `phase2AuthorityProof` carrier | §6.2 | D2 | `A` |
| B — eligibility gates E1-E15 | §6.5 | D4 | `B` |
| C — alignment rules A1-A8, their evaluation order and limits L1-L3 | §6.7 | D9 | `C1`, `C2` |
| D — `candidate.json` / `state.json` schemas and closed enums | §6.10 | D20 | `D1`, `D2`, `D3` |
| E — acceptance transaction steps 1-14 | §6.14 | D29 | `E` |
| F — ADR-036 D15 P3 two-variant restatement | §6.16 | D30 | `F` |

### 6.0 Parity blocks are delimited, and the guard reads only what is delimited

rev-2 named the six blocks in prose and left the parity check to infer where
each one starts and ends. rev-3 marks them. In **both** documents every parity
region is wrapped in a matched pair of HTML comments:

```
<!-- parity-block:<ID>:begin -->
…the bytes under parity…
<!-- parity-block:<ID>:end -->
```

The rules are mechanical:

- the guard extracts **only** the bytes strictly between a matching
  `begin` / `end` pair and compares them across the two documents; text outside
  a marked region is never compared, so ordinary prose may legitimately differ;
- the marker IDs are exactly `A`, `B`, `C1`, `C2`, `D1`, `D2`, `D3`, `E` and
  `F` — **nine** marked regions carrying the six logical blocks, because block
  C's rules and limits and block D's two schemas and enum paragraph are
  separated in both documents by prose that is deliberately not identical;
- **the template above is not a region.** It uses the literal placeholder
  `<ID>`, which is not one of the nine IDs, and the marker parser recognizes a
  marker **only** when its ID matches that closed set exactly. The parser
  therefore skips this section's illustration in both documents — as it skips
  ADR-037 D36's identical illustration — and neither counts as a `begin`
  without an `end`. A parser that accepts an arbitrary ID would read the
  template as an unpaired marker and fail the check on a document that breaks
  nothing;
- each ID appears **exactly once** as a `begin` and once as an `end`, in that
  order, in each document. A missing marker, a duplicated ID, an unpaired
  marker, an ID present in one document and not the other, or any byte
  difference inside a pair fails the check;
- the enclosed bytes are what the two documents must agree on. rev-3 changes
  the contents of `A` and `E`, identically on both sides, and leaves the other
  seven regions unchanged; rev-5 changes only `A`, again identically on both
  sides.

**Why the regions are finer-grained than the blocks.** Block D's claim has
always been scoped to "the two JSON bodies and this enum paragraph"; the
sentences between them differ by design, and an inferred block boundary that
swallowed them would have reported a parity failure against a contract neither
document breaks. Marking the regions makes the claim exactly as wide as it was
always meant to be.

### 6.1 One resolved upstream commit, never the live worktree

Phase 2 resolves the upstream ref once per feature to a 40-hex commit
(`gitutil.ResolveRef`, `internal/gitutil/gitutil.go:789`) and performs every
eligibility, alignment and classification read against that commit's tree
through `gitutil.FileAtCommit` (`internal/gitutil/gitutil.go:803`) or
`gitutil.BlobAtTree` (`internal/gitutil/trailers.go:737`).

`s.Root` is read for exactly two purposes, neither of which is eligibility:
`.tpatch/` store artifacts, and — only inside explicit acceptance (§6.14) — the
compare-and-swap preconditions on the paths about to be written.

A dirty worktree, a detached HEAD, an in-progress rebase or an
`--allow-dirty` invocation changes nothing about the phase-2 classification.

### 6.2 Phase-2 outcomes and the confirmation-gate amendment

Phase 2 classifies each ADR-036 normalized effect as **present**,
**applicable** or **conflicting** at the resolved upstream commit (§6.7), then
produces one of four results. **Only the all-present result returns from
phase 2.**

| Phase-2 result | Phase 2 returns | Candidate |
|---|---|---|
| every effect present, CG1-CG7 all hold | `store.ReconcileUpstreamed`, **confirmed** | none |
| every effect present, any of CG1-CG7 fails | `store.ReconcileUpstreamed`, which the shipped gate demotes to `store.ReconcileBlocked` / `rejected-upstreamed` | none |
| every effect present or applicable, ≥1 applicable, 0 conflicting | nothing — an **in-memory** candidate proof is held | not yet |
| any ineligible, unsafe or conflicting effect | nothing — phase-2 `BLOCKED` classification | none |

**The all-present arm is not terminal today.** §2.7 states the source truth:
`applyUpstreamedConfirmationGate` (`internal/workflow/reconcile.go:1032`)
confirms only on `patch-id-match` with a non-empty `matched_upstream_sha`
(`:1047-1054`) or high-confidence `reverse-apply` (`:1055-1060`), so a
`recipe-operation-match` result is overwritten with `store.ReconcileBlocked`
and `rejected-upstreamed` (`:1089-1090`). That behavior is pinned by
`TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch`
(`internal/workflow/reconcile_evidence_integration_test.go:459`) over
`buildOperationUpstreamedCandidateFixture` (`:235`).

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

**Everything weaker keeps today's behavior.** A legacy `recipe-operation-match`
entry, a persisted entry from an earlier invocation, an entry from a run whose
gate refused, or any entry failing one of CG1-CG7 still requires human
confirmation and still demotes to `blocked` / `rejected-upstreamed`. The gate's
other two confirming inputs, its rejection arm, its evidence record and
`persistRevisionPassLog` (`internal/workflow/reconcile.go:1096`) are unchanged.

**This is a strict widening of a safety gate, so it is stated as new
authority.** The confirmation gate exists because two real features were lost
to false `upstreamed` verdicts
(`docs/prds/PRD-upstreamed-confirmation-gate.md:61-66`). CG1-CG7 replace "a
human looked at it" with "a machine re-derived every byte from two named
immutable trees in this process, and found nothing left to do" — which is the
one substitute the accepted contract's own goal 4 anticipates for a
deterministic fast path (`docs/prds/PRD-upstreamed-confirmation-gate.md:79-80`).

**Where a candidate may be persisted.** The held proof survives phase 3
untouched and is disposed of at phase 4:

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

Three consequences are load-bearing:

- **Candidate persistence is gated by an actual textual conflict.** A feature
  whose patch still forward-applies gets the shipped `reapplied` answer, which
  is strictly stronger than a staged candidate. Finalization therefore runs
  **after** phase 3 has completed and **after** phase 4's forward-apply preview
  (`internal/workflow/reconcile.go:496`) has classified the arm — it is not
  hoisted earlier for convenience.
- **A candidate never suppresses a stronger answer.** Neither a provider
  `upstreamed` at phase 3 (`:477-483`) nor a clean phase-4 reapply can be
  overtaken by a held proof, because the proof is only consulted on the arms
  that were going to return `blocked`.
- **`--resolve` is honored.** An operator who asked for provider resolution
  gets provider resolution; no two staged answers coexist (§6.11).

**Finalization is the only locked part of generation.** The held proof is
computed lock-free; finalizing it takes the per-feature reconcile lock and,
around worktree mutation only, the global lock (§6.13). If either lock is
unavailable the finalization records `candidate-lock-held`, discards the proof
and falls through to the shipped `blocked` answer — a contended repository
never gets a partially staged candidate.

**This is still a phase-2 semantic proof.** It is computed at phase 2, from
phase-2 inputs, with phase-2 evidence, and it is merely *surfaced* at the
phase-4 conflict boundary because that is the first point at which the cheaper
textual answers are known to have failed. There is **no phase 2.5**, no new
phase value and no reordering: `result.Phase` for the candidate-ready outcome
is `phase-2-operation-candidate`, and its **first** staged evidence attempt
carries `phase: phase-2` while its second carries `phase: phase-4` — the arm
substitutes its staged list for the derived entry, so the phase-2 finding and
the phase-4 textual-conflict finding coexist without duplication (§6.2, §6.17).

### 6.3 Phase-2 refusal is always a fallthrough, and a warning is never eligibility

Every **phase-2** refusal in this document records phase-2 evidence and
**continues**. `tpatch reconcile` exits with the code its final phase earned.
The rule is scoped precisely: it covers the phase-2 vocabulary of §7.1, §7.2
and §7.3. The terminal candidate actions, acceptance failures and recovery
refusals of §7.4 are **not** in scope — they are command refusals that return
their own exit code and have no later phase to fall through to.

**`SPEC.md:444-448` is amended, and the amendment covers all three phase-2
arms.** The SPEC's phase-2 decision tree states three outcomes; every one of
them is re-specified, and S9 edits all three in one pass:

| SPEC phase-2 row today | Amended reading |
|---|---|
| `→ All already-present → UPSTREAMED` (`SPEC.md:446`) | **CG-qualified confirmation.** All-present is a *candidate* for `UPSTREAMED`; it returns `upstreamed` only when the confirmation gate's CG1-CG7 authority class holds (§6.2). Unconfirmed all-present remains the shipped demotion to `blocked` / `rejected-upstreamed` |
| `→ Any conflict → BLOCKED` (`:447`) | **A phase classification, not an overall return.** The pipeline continues to phase 3 and phase 4, exactly as `internal/workflow/reconcile.go:459-464` already does |
| `→ Mix → STILL-NEEDED` (`:448`) | **Nonterminal at phase 2.** A mixed result holds an in-memory candidate proof and returns nothing; it surfaces as a candidate-ready `still_needed` **only** at a real phase-4 textual conflict with `--resolve` unset (§6.2). Every stronger later outcome — a phase-3 provider `upstreamed`, a clean phase-4 reapply — wins, and the proof is discarded |

The SPEC edit lands in implementation slice S9, not in this planning revision.
**ADR-010 is not amended by any of the three**: the sentences that change live
in `SPEC.md` §7, and ADR-010's four-phase order, shadow-worktree atomicity
model and provider-resolution contract are preserved verbatim.

**The ten-row consumer table belongs to GH #15's PRD §6.14**
(`docs/prds/PRD-recipe-generation-authority.md:2098`, table `:2140-2151`), not
to ADR-036 D13 — D13 is the **six-rung** verify severity ladder
(`docs/adrs/ADR-036-recipe-coverage-authority.md:1745`, ladder `:1755-1766`).
That table has ten rows, **seven** of them warning-class with exit `0`:
missing coverage, incomplete coverage, `recipe-stale.json`,
`canonical-patch-unparseable`, `recipe-undecodable`, `recipe-owner-mismatch`,
and any `object_kind` / `content_kind` of `unknown`. Phase 2 hard-refuses all
ten. The mapping below reproduces those ten rows and adds one eleventh —
coverage that is present but unreadable, which ADR-036 D13 routes to rung 1 as
`recipe-coverage-malformed` rather than treating it as absent:

| GH #15 §6.14 state | Verify | Phase 2 gate |
|---|---|---|
| coverage missing | `warn`, exit `0` | E1, refuse, fall through |
| coverage present but unreadable | `block`, exit `2` | E1, refuse, fall through |
| coverage malformed | `block`, exit `2` | E1, refuse, fall through |
| any `object_kind: unknown` / `content_kind: unknown` effect | `warn`, exit `0` | E1 — a `complete` record carrying one does not decode |
| coverage `incomplete` | `warn`, exit `0` | E3, refuse, fall through |
| `recipe-owner-mismatch` | `warn`, exit `0` | E3 for a truthful record, then **E7** as the recomputation defense |
| presence flag contradicted either direction | `block`, exit `2` | **E4**, refuse, fall through |
| `canonical-patch-unparseable` | `warn`, exit `0` | E3 for a truthful record, then **E9** as the recomputation defense |
| `recipe-undecodable` | `warn`, exit `0` | **E6**, refuse, fall through |
| envelope owner mismatch | `block`, exit `2` | E2, refuse, fall through |
| `recipe-stale.json` present | `warn`, exit `0` | E11, refuse, fall through |

**Verify's exit code is never consulted.** Phase 2 does not run verify and does
not read its verdict; it recomputes the same facts itself. An implementation
that gates on a verify exit status has inverted the dependency.

### 6.4 The legacy operation evaluator is retired as an authority

`evaluateRecipeOperations` (`internal/workflow/reconcile.go:611`) is **retired
as an authority** by this feature. Behind the `operation_candidate_enabled`
config gate:

- it may still run, unchanged, to produce the shipped diagnostic note
  `Operation-level: %d present, %d applicable, %d conflicts`
  (`internal/workflow/reconcile.go:460-461`) when coverage is absent or
  ineligible. Its counters keep their exact shipped byte output;
- it **never** produces a terminal `upstreamed`, a candidate, a confirmation, a
  state transition or an evidence entry that CG1 admits. The shipped
  `opResult.allPresent` early return (`internal/workflow/reconcile.go:451-457`)
  is removed from the authoritative path;
- every ineligible or legacy path records phase-2 evidence with the reason code
  that refused it (§6.5, §7) and falls through to phase 3/4.

**One existing expectation intentionally changes, and no byte identity is
claimed for it.** With the gate **on**,
`TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch`
(`internal/workflow/reconcile_evidence_integration_test.go:459`) no longer
reaches the confirmation gate from phase 2, because its legacy fixture has no
eligible ADR-036 coverage and therefore mints no `upstreamed` candidate at all;
the fixture falls through and earns its later phase's outcome. Slice S7 adds
the gate-on expectation beside the existing one rather than editing it in
place. The gate's rejection arm stays reachable from phase 3
(`TestReconcilePhase3_ProviderAssistedUpstreamed`,
`internal/workflow/reconcile_test.go:304`) and its confirmation arm from
phase 1 (`TestUpstreamedConfirmationGateKeepsConfirmedReverseApply`,
`internal/workflow/reconcile_evidence_integration_test.go:413`).

**`operation_candidate_enabled` defaults to `false` in v0.18.0.** With the gate
off, every reconcile path — including the legacy all-present arm — is
byte-identical to the shipped behavior, which is what makes the S0 goldens a
usable guard for the whole wave. Flipping the default is a separate,
soak-gated decision (§10, §11).

### 6.5 Independent eligibility: fifteen gates, closed and ordered

Coverage is decoded with **ADR-036 D3's own strict semantic decoder**, not a
private re-implementation and not a weaker one. That decoder already refuses
every internally contradictory record — `complete` beside a non-empty
`reasons`, a non-`represented` disposition, a non-empty `reason_codes`, an
empty `effects` array, `object_kind: unknown`, `content_kind: unknown`,
`patch_present: false`, `recipe_present: false`, `recipe_decodable: false`, an
unobserved required side or a non-`commit` reference, and the converse
`incomplete` record with all of those satisfied
(`docs/adrs/ADR-036-recipe-coverage-authority.md:1065-1076`).

**Those contradictions therefore fail at E1, as decode failures, and have no
separate gate.** rev-0's `coverage-reason-outstanding`, `effect-axis-unknown`
and zero-effect refusals were unreachable rows for exactly this reason; they
are removed from the reachable vocabulary. What survives is recomputation: a
value the decoder cannot check because it is a claim *about other bytes* is
re-derived and compared.

The gate set is closed and ordered. The first failing gate records its reason
and falls through. **This is parity block B.**

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

### 6.6 The bounded v1 candidate domain, and what the persisted recipe is for

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
unknown type fail predicate 5 or predicate 6 the same way. **So E10 raises
`recipe-unsupported` only against a record whose claimed completeness does not
survive re-reading the recipe** — it makes no independent claim about
`ensure-directory` (M8), and it is not a second, weaker domain test.

A candidate is derived only when **every** normalized effect is one of exactly
two shapes.

**(a) Exact creation.** `change_kind: add`, `content_kind: text`,
`object_kind: regular`, `new_mode: 100644`, `old_mode: ""`.

**(b) Additive-only modification.** `change_kind: modify`,
`content_kind: text`, `object_kind: regular`,
`old_mode == new_mode == 100644`, and the effect's fragment contains **zero**
`-` content lines.

Presence at the upstream commit is a **classification** (§6.7), not a domain
condition: a creation whose path already exists upstream with the effect's
exact postimage is `absorbed-present`, and one that exists with different bytes
is `creation-conflict`.

Every other shape refuses the whole candidate and falls through:

| Refused shape | Reason code |
|---|---|
| `delete` / `rename` / `copy` | `effect-change-kind-unsupported` |
| `content_kind: binary` or `none` | `effect-content-kind-unsupported` |
| `object_kind: executable` / `symlink` / `gitlink` | `effect-object-kind-unsupported` |
| any mode change, including `100644`→`100755` | `effect-mode-change-unsupported` |
| a `modify` fragment carrying any `-` line | `effect-replacement-unsupported` |

**Refusal is per-feature, not per-effect.** One unsupported effect refuses the
candidate. A partial candidate would silently redefine the feature.

**Vocabulary.** "Operation replay" is GH #13's issue title and is kept for
continuity. What v1 actually replays is the set of **normalized effects**
ADR-036 derived from the canonical patch, re-expressed as newly derived
`create-file` and `insert-by-boundary` operations (§6.9). No persisted
operation body is executed cross-base, in v1 or as an implementation shortcut.

### 6.6a The permitted feature states, and the state bound into the candidate

A candidate is a proposal to rewrite a feature's realized changes at a new
upstream. That only means something for a feature whose changes are supposed
to be in the tree, or were and stopped applying.

**Generation precondition.** Candidate derivation requires the feature's
current `status.State` to be in the exact set

```
{ store.StateApplied ("applied"), store.StateBlocked ("blocked") }
```

using the shipped `store.FeatureState` spellings
(`internal/store/types.go:8-37`). Any other state — `requested`, `analyzed`,
`defined`, `implementing`, `active`, `reconciling`, `reconciling-shadow`,
`upstream_merged`, `rejected` or `unapplied` — records the nonterminal phase-2
refusal `candidate-feature-state-unsupported` and falls through.

**It is a lifecycle precondition, not an ADR-036 binding gate**, so it is
evaluated in the §6.11 preflight **before** E1 rather than as a sixteenth
E-gate: E1-E15 are exactly the closed set of recomputations over the coverage
record's own bindings, and a feature-state question is not one of them. The
distinction is testable — a feature in a refused state never reaches E1, and a
feature in a permitted state is graded by E1-E15 alone.

**The state is bound into the candidate.** The permitted state observed at
derivation is recorded as `source_feature_state` in three places, and the three
must agree:

- the **immutable identity tuple** (§6.10), so a candidate derived while the
  feature was `blocked` and a candidate derived while it was `applied` are
  different candidates with different IDs;
- `candidate.json`'s top level, as an immutable metadata field;
- the `status.json` `operation_candidate` pointer, so a reader can see which
  state the pending proposal was derived against without opening the artifact.

**Acceptance re-checks it.** §6.14 step 3 requires the feature's **current**
`status.State` to equal the bound `source_feature_state` **and** to still be in
the permitted set. Any drift — including `blocked` → `rejected`, `applied` →
`unapplied`, either → `upstream_merged`, and any transition into
`reconciling-shadow` — refuses `candidate-feature-state-changed`, exits `3`,
and writes nothing: the refusal happens before the journal, before the
snapshots and before any live write.

**Successful acceptance sets `applied`.** §6.14 step 12 marks the feature
`store.StateApplied` (`internal/store/types.go:13`), which is the same terminal
state the resolver accept path reaches (`internal/workflow/accept.go:123`).

**Generation itself changes no state** (§6.11). Binding a state is a record of
what was observed, never a transition.

**Rejected and unapplied are refused deliberately.** A `rejected` feature has
an operator decision against it (ADR-031), an `unapplied` feature has no
realized changes to reproduce, an `upstream_merged` feature is retired, and a
`reconciling-shadow` feature already has a staged answer (E15 refuses that one
twice over). Each of these is a case where a machine-derived rewrite would be
answering a question nobody asked.

### 6.7 Insertion-run derivation, exact line-record alignment and classification

rev-0's byte-window context expansion is **deleted in full**. It rejected
ordinary repeated bytes, had no decidable termination, and could not solve the
fixture this feature exists for. It is replaced by exact line-record alignment.

**Records.** A file's bytes are split into **line records**: each record is the
maximal byte run up to and including the next `\n`, and a trailing run with no
`\n` forms a final **no-newline record**. Record equality is **byte-exact**, so
`foo\n`, `foo\r\n` and `foo` are three different records. There is no
normalization anywhere: mixed EOL is simply exact bytes, and a file containing
both `\n` and `\r\n` records is ordinary input, not an error.

**Runs and base boundaries.** Each effect's fragment is parsed with the **same**
ADR-036 D1 strict grammar that produced the coverage record. Each additive
canonical hunk yields its maximal consecutive `+` line sequences as **runs**,
one-based in fragment order. Each run carries:

- its exact **insertion records** — the `+` payloads with the leading `+`
  removed, each keeping its original terminator byte-for-byte;
- its **base boundary** `b`, an index into the reference file's record list
  with `0 ≤ b ≤ len(B)`, meaning "between base record `b-1` and base record
  `b`", derived by walking the fragment's hunk headers and context records
  against the reference-tree file (`gitutil.FileAtCommit(repoRoot,
  reference.commit, path)`).

**Reconciliation is mandatory.** Replaying every run at its boundary against
the reference-tree file must reproduce the effect's `postimage_sha256` exactly.
Otherwise: `derivation-postimage-mismatch`.

**Alignment.** Let `B` be the reference file's record list and `U` the
upstream file's record list at `upstreamCommit`. Phase 2 computes the set of
**all** maximum-cardinality monotone exact-record alignments between `B` and
`U` — every longest-common-subsequence trace — by a deterministic dynamic
program. **No tie is selected.** The following eight rules are **parity block
C**.

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

**Creations** classify at file granularity on the same three-state rule: an
upstream blob byte-equal to the effect postimage is **present**, an absent path
is **applicable**, and a present path with different bytes is
`creation-conflict`.

**Hard resource limits — closed constants, no tuning, no configuration.** These
three constants are **parity block C** as well; they are compiled-in literals
with no flag, no config key and no environment override.

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
through. The limit is a refusal, never a truncation, a sampling or a
downgrade, and L1-L3 are checked **before** the matrices are allocated (step 1
of the evaluation order above), so an over-large file never allocates.

**Alignments are ephemeral.** No artifact carries a record body, a record
index, a byte offset, a line number or any file bytes. At acceptance the
alignment is **re-derived from the bound inputs from scratch**; stored proof
hashes are compared as a staleness check and are **diagnostic only** — a digest
cannot be applied.

#### 6.7.1 Worked proof: the motivating adjacent-CLI-args fixture

`docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md`
is the case this feature exists for, and rev-0's algorithm could not solve it.
Under §6.7 it succeeds.

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
`i < 3 ∧ j ≥ 1` or `i ≥ 3 ∧ j < 1`, so A4's unanimity holds. The gap
`U[1:1]` is **empty**, so A5 classifies **applicable**, and the two feature
records are inserted at upstream record index `1`:

```
args := []string{
	"--feature-x",
	"--feature-y",
}
return run(args)
```

That is byte-exactly the case study's stated correct resolution
(`summary.md` §2). The `adjacent-between-delete-first` variant, where upstream
deletes only `--old-a`, resolves the same way at `b = 2` to `left₀ = right₀ =
1` and yields the case study's other stated correct result, retaining
upstream's surviving `--old-b`.

**Why the deleted neighbours are not a problem.** A2 skips unmatched base
records, so the boundary anchors to the nearest **surviving** neighbours on
each side. Deleted base records contribute nothing to `U`, so they add nothing
to the gap. **Unrelated upstream additions in that region do**, and A5 refuses
them as `boundary-conflict` — which is the discrimination rev-0's global
"postimage elsewhere" rule was reaching for and could not express.

### 6.8 Candidate execution, validation, idempotency and absorption

**Execution is all-or-nothing** in a candidate worktree created detached at
`upstreamCommit` under §6.10's namespace. Any precondition or write failure
abandons the whole candidate and removes its worktree.

**Postcondition validation**, after one application of the complete set:

1. per-effect postconditions hold at the re-derived boundary (or byte-identical
   postimage for a creation);
2. the set of paths differing from `upstreamCommit` equals **exactly** the
   derived operations' path set;
3. every touched path is a regular `100644` file on both sides;
4. no path outside the derived set differs in content or mode;
5. no `<<<<<<<`, `=======` or `>>>>>>>` line appears in any touched file.

**Idempotency** is proved by running the **same** derivation and classifier a
second time with the candidate result as the upstream input. Required: **every
effect** classifies **present** — not merely every derived operation — the
applicable and conflict counts are **zero**, and the Git tree hash and
candidate diff bytes are **unchanged**. This is a re-run of the production
classifier, not a bespoke checker.

**Any second-pass outcome other than all-present refuses.** `candidate-not-idempotent`
covers every one of them, not just a nonzero applicable count: a second-pass
`boundary-conflict` or `creation-conflict` classification, a second-pass
`alignment-ambiguous`, `alignment-left-unanchored`, `alignment-right-unanchored`
or `alignment-range-overlap` refusal, a second-pass `alignment-limit` (the
candidate result can be larger than the upstream input that produced it), a
second-pass `record-terminator-unsupported`, a second-pass
`derivation-postimage-mismatch`, a changed tree hash, a changed diff, or any
other refusal code the second pass emits. The rule is stated positively so it
cannot be read as a list: **the second pass must return all-present with zero
counts and unchanged bytes, and anything else is `candidate-not-idempotent`,
with the second-pass code carried as the diagnostic detail.**

**Absorption accounting.** The candidate diff may omit effects already present
upstream — that is the point. Every old canonical effect must map to exactly
one of `absorbed-present` or `candidate-applicable`. Anything else refuses with
`effect-unexplained`.

**Generation writes no live repository content.** The guarantee is scoped
precisely: candidate generation writes **zero** bytes of repository content
under `s.Root` outside `.tpatch/reconcile-candidates/<slug>/`. Four write sets
are **expected** and are excluded from that statement:

- `.tpatch/reconcile-candidates/<slug>/…` — the candidate namespace itself;
- `.tpatch/locks/…` — the §6.13 lock files;
- the feature's own `.tpatch/` artifacts, `status.json` and
  `reconcile-evidence.jsonl`, which every reconcile already writes;
- `.git/worktrees/<name>/` — Git's own bookkeeping for
  `git worktree add --detach`, which is not repository content.

**Test execution.** Generation runs **no** configured `test_command`
(`internal/workflow/validation.go:60`, set at
`internal/cli/cobra.go:3731-3732`). Explicit acceptance runs it in the candidate
worktree before any live mutation via `RunTestCommandInShadow`
(`internal/workflow/validation.go:273`); a non-zero result refuses the
acceptance and leaves the candidate pending. Optional preview validation may
later be added **only** as an explicit flag.

**Output vocabulary.** Because generation runs no tests, every surface
describing a generated candidate says **structurally validated** — never
"validated", "verified" or "tested" standing alone. `candidate.json`'s
`result.validated` is fixed at `structural`; an acceptance test run is recorded
in `state.json`'s `accept_validation`, not in the immutable artifact.

### 6.9 Typed roots, sanctioned constructors and no lazy fetch

**Forbidden for candidate authority**, each for a stated reason:
`evaluateRecipeOperations` (live-root and fail-open,
`internal/workflow/reconcile.go:611-655`), `ExecuteRecipe` (continues past
per-op errors, `internal/workflow/recipe.go:102-110`), `dryRunOperation` and
`executeOperation` (hardcoded `s.Root`, fixed `0o644`, first-match replace,
unconditional append, `internal/workflow/recipe.go:129,200,207-242`).

Candidate file IO accepts only:

```go
type ReferenceRoot struct{ /* reconstructed reference.commit; read-only */ }
type CandidateRoot struct{ /* isolated detached worktree at upstreamCommit */ }
```

**Two sanctioned constructors are the only bare-root boundary in the candidate
package:**

```go
func OpenReferenceRoot(repoRoot string, commit string) (*ReferenceRoot, error)
func CreateCandidateRoot(repoRoot, slug, candidateID, commit string) (*CandidateRoot, error)
```

Each constructor, and nothing else, may take a bare `string` root. Each:

1. validates `commit` as 40 lowercase hex and confirms the object exists
   **locally**;
2. validates and contains its path with `safety.EnsureSafeRepoPath`
   (`internal/safety/safety.go:12`) against the root it is about to hold;
3. sets `GIT_NO_LAZY_FETCH=1` — the shipped `gitutil.NoLazyFetchEnv` constant
   (`internal/gitutil/trailers.go:41`) — on **every** Git subprocess it or its
   returned root spawns.

Every subsequent read or write takes the typed root value. Passing `s.Root` to
anything else in the candidate package is a compile error, not a review
finding. Store reads go through `*store.Store` and are an explicitly separate,
non-repository surface.

**The source guard excludes exactly those two constructors, by name.** §9.11's
inventory guard fails on any candidate-package function other than
`OpenReferenceRoot` and `CreateCandidateRoot` that accepts a `string` root, and
on any candidate-path Git invocation that does not carry
`GIT_NO_LAZY_FETCH=1`. It does not exclude "arbitrary calls", a package, a file
or a build tag — two symbols, named.

Closed candidate operation kinds — **exactly two, unknown refuses**:

| Kind | Payload | Precondition |
|---|---|---|
| `create-file` | `path`, `content_sha256`, `size` | path absent in `CandidateRoot` |
| `insert-by-boundary` | `path`, `run_ordinals`, `insertion_sha256`, `insertion_size`, `boundary_proof_sha256` | the boundary re-derives unanimously; the gap is empty |

### 6.10 Namespace, identity, artifacts and paths

```
.tpatch/reconcile-candidates/<slug>/<candidate-id>/
    candidate.json          immutable strict metadata
    state.json              mutable lifecycle
    candidate.patch         reviewable unified diff
    worktree/               detached worktree at upstreamCommit
    accept-journal.json     present only during an acceptance
    snapshots/              rollback snapshots
    staged/                 precomputed artifacts
.tpatch/reconcile-candidates/<slug>/rejected.jsonl
.tpatch/locks/reconcile/<slug>.lock
.tpatch/locks/worktree-mutation.lock
```

`.tpatch/shadow` (`internal/gitutil/shadow.go:35`) and its lifecycle
(reap-on-create `:56,72`, newest-wins `:115`, clear-all `:152`) are **not**
reused.

**Every persisted path is repository-relative and slash-normalized.** Every
`path` value in `candidate.json`, `state.json`, `rejected.jsonl`, the status
pointer and every evidence field is produced by `filepath.ToSlash` over a path
already made relative to the repository root, matching the shipped convention
at `internal/gitutil/shadow.go:206,248`. A backslash-separated or absolute
path in any of those artifacts is a refusal, not a platform variation, and the
artifacts are therefore byte-identical across Linux, macOS and Windows.

**Identity is immutable and pre-execution.**

```
candidate_id = "rc_" + first 12 lowercase hex of SHA-256(canonical JSON of the identity tuple)
```

The identity tuple is **exactly** `schema_version`, `feature`,
`source_feature_state`, `upstream_commit`, `reference_commit`,
`coverage_sha256`, `patch_sha256`, `recipe_sha256`, `derivation_mode`, the
**ascending-sorted** list of derived-operation proof hashes, and the
**ascending-sorted** list of effect-map proof hashes. Each derived-operation
proof hash is SHA-256 over the canonical JSON of `{kind, path, effect_ordinal,
effect_sha256, run_ordinals, insertion_sha256, insertion_size,
boundary_proof_sha256}`; each effect-map proof hash is SHA-256 over the
canonical JSON of `{effect_ordinal, effect_sha256, path, resolution,
operation_ordinals, reason_codes}`.

`source_feature_state` is the permitted `store.FeatureState` observed at
derivation (§6.6a), so the same bindings derived while the feature was
`blocked` and while it was `applied` are two different candidates with two
different IDs. It is known before the worktree exists, like every other member.

The tuple contains **no** candidate diff hash, **no** candidate tree hash,
**no** status, **no** timestamp and **no** self-reference to `candidate_id`.
Every member is known **before** the candidate worktree is created, which is
the property that lets the directory be named with the ID directly. **There is
no staging directory and no rename.**

`boundary_proof_sha256` is SHA-256 over the canonical JSON of
`{path, base_boundary_index, upstream_left_index, upstream_right_index,
base_record_count, upstream_record_count, alignment_length,
insertion_record_count, insertion_sha256}`. The tuple is hashed and discarded;
none of its members is ever written to an artifact.

**`candidate.json` v1** — immutable, strict both directions, no status, no
timestamp, no source body, no record body, no index and no offset:

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
full at every reuse and at acceptance, and they are excluded from the ID
precisely so that the ID is computable before the operations run.

**`state.json` v1** — the only mutable file, carrying the whole lifecycle:

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

**Rerun and collision.** Identical bindings recompute the identical ID. Before
anything is created, the derived ID is checked against `rejected.jsonl`
(§6.18); a match records `candidate-rejected` and falls through. Otherwise, if
a directory with that ID exists:

| Comparison | Result |
|---|---|
| `state.json` says `accepted` | `candidate-already-accepted` — refuse, fall through, touch nothing |
| immutable identity subset differs | `candidate-id-collision` — refuse, never overwrite |
| identity subset equal, `result` integrity fields differ | `candidate-integrity-drift` — refuse, never overwrite |
| both equal, `state.json` is `pending` or `stale` | reuse, **only after** the full §6.8 validation and idempotency proof re-passes |

`state.json` is **not** part of either identity comparison, so a status
transition can never collide with, invalidate or be mistaken for an identity
change. The `accepted` row above is a **lifecycle** check that runs first, not
an identity comparison.

**An accepted candidate directory is immutable audit, and nothing else.** Once
§6.14 completes, the directory is never reused, never re-validated, never
rejected, never pruned and never overwritten: `worktree/`, `snapshots/`,
`staged/` and the journal are removed by step 14, and what remains —
`candidate.json`, `state.json` at `accepted`, `candidate.patch` — is a
read-only record of what was accepted. The `status.json` pointer is **cleared**
on success (§6.14 step 14), so an accepted directory has no live pointer and
the §6.11 preflight ignores it entirely: it is not a pending candidate, it does
not gate any command, and it is not marked `stale` when the upstream moves.

**Re-deriving the same identity after acceptance falls through.** If a later
reconcile derives an ID whose directory is already `accepted`, it records
`candidate-already-accepted` and falls through **before** creating anything —
the operator already accepted this exact answer, and re-staging it would ask
the same question twice. Changed bindings — a new upstream commit, a new
residual patch, a regenerated recipe, a new coverage record, or a different
`source_feature_state` — mint a **different** ID and a new candidate normally,
which is the ordinary path after a successful accept because acceptance
republishes all three bindings (§6.16).

**`candidate.patch`** is a reviewable unified diff and may contain diff source
exactly as `artifacts/post-apply.patch` does. It is for humans and
`--candidate-diff`; it is **not** an input to acceptance.

**Status pointer.** `ReconcileSummary` (`internal/store/types.go:372`) gains one
optional `operation_candidate` object — `candidate_id`, `path`, `status`,
`source_feature_state` — with `omitempty` so the no-candidate serialization
stays byte-identical. It is **not** `ShadowPath` / `ResolveSession`
(`internal/store/types.go:381-382`), which stay resolver-only.

### 6.11 Lifecycle: preflight, state preservation and coexistence

**Preflight ordering.** Before phase 1 and before the phase-1.5 sweep, and for
**every** reconcile invocation and every candidate action naming this slug, the
pipeline runs the candidate preflight in this order:

0. **Lock.** Acquire the per-feature reconcile lock
   `.tpatch/locks/reconcile/<slug>.lock` **nonblocking** (§6.13). Everything in
   steps 1-3 happens while it is held, and it is released before phase 1 —
   that is, before any provider call, any forward-apply preview and any other
   expensive work. On contention the preflight performs **no** inspection,
   **no** mutation and **no** prune: a plain reconcile records the nonterminal
   `candidate-lock-held` and continues into phase 1 with candidate derivation
   disabled for this invocation, and a terminal candidate action refuses with
   `candidate-lock-held` and exit `3`. On a build target with
   `Supported == false` the same applies under `lock-unsupported`.
1. **Recovery check.** Any `accept-journal.json` under
   `.tpatch/reconcile-candidates/<slug>/` refuses the whole command with
   `recovery-required` (§6.15). Nothing else in this list runs, nothing is
   pruned, and the lock is released before the refusal returns.
2. **Staleness check.** With no journal present, a **pending** candidate whose
   `upstream_commit`, whose three binding hashes or whose bound
   `source_feature_state` no longer matches the current resolution is marked
   `stale` in `state.json`, its `worktree/` is pruned safely (the
   `pruneShadowPath` shape at `internal/gitutil/shadow.go:309`, in the
   candidate namespace), its immutable `candidate.json` is **retained for
   audit**, and the `status.json` pointer is cleared. **No silent overwrite.**
   Directories at `accepted` or `rejected` have no live pointer and are
   **ignored** here — they are immutable audit (§6.10), never marked stale and
   never pruned by the preflight.
3. **Feature-state precondition.** The feature's current `status.State` must be
   in the §6.6a permitted set for candidate derivation to be attempted at all;
   otherwise the invocation records `candidate-feature-state-unsupported` and
   continues with derivation disabled.

**Every candidate-state mutation happens under the lock.** That is the whole
point of step 0: marking a candidate `stale`, pruning its worktree, clearing or
setting the `status.json` pointer, appending to `rejected.jsonl` and writing
`state.json` are all read-modify-write sequences on state that a concurrent
acceptance also mutates. There is **no unlocked prune anywhere** in this
document — not in the preflight, not in finalization, not in rejection and not
in recovery. Finalization (§6.2) **reacquires** the per-feature lock and
**revalidates** the preflight facts it depends on rather than trusting what the
earlier, now-released, hold observed; acceptance (§6.14) holds the per-feature
lock and the global worktree-mutation lock for the whole transaction.

**The lock is held for inspection, not for the pipeline.** Holding it across
phase 3's provider calls would serialize every reconcile in the repository
behind one network round trip. Releasing it after step 3 and reacquiring it at
finalization is what keeps the window short, and revalidation is what makes the
gap safe.

**Candidate persistence changes no lifecycle state.** A candidate-ready
`store.ReconcileStillNeeded` **bypasses `updateFeatureState`**
(`internal/workflow/reconcile.go:1181`) entirely, because that function's
`ReconcileStillNeeded` arm forces `status.State = store.StateApplied` and
overwrites `status.Notes` (`:1220-1222`).

It persists instead through a dedicated path that:

- **calls `saveReconcileArtifacts(s, slug, result, nil)`** — it does not
  reimplement it. The candidate-ready result reaches the shipped function with
  its two staged `verdictAttempts` (§6.2 configuration 3) and a **`nil`**
  proof, so `reconcile-session.json` and `reconcile.md` are written by the
  shipped writer, the staged list substitutes for the derived entry at `:766`,
  and the shipped `:766` → `:771` call order and each detector's own firing
  condition are preserved exactly — file-novelty and hunk-overlap still run,
  the blocked-only pair still does not, and the confirmation gate still returns
  immediately because the outcome is not `upstreamed` (`:1033-1035`). A
  reimplementation that duplicates the artifact writers, reorders the call
  sequence or skips the supporting detectors fails this contract even if the
  bytes happen to match on one fixture;
- appends the ordered evidence attempts through that same call (§6.17);
- sets `status.Reconcile` — including the new `operation_candidate` pointer —
  and `status.LastCommand` / `status.UpdatedAt` **without** calling
  `updateFeatureState`;
- **leaves `status.State` and `status.Notes` exactly as loaded.**

The bypass is therefore scoped to the **state and summary writer**
(`internal/workflow/reconcile.go:1181`), not to artifact or evidence
persistence: only the status write is dedicated, and it goes through the same
shared status merge below.

So a feature that was `blocked` stays `blocked`, one that was `applied` stays
`applied`, and the outcome is carried by the reconcile summary alone. No
`FeatureState` transition, no `StateReconcilingShadow`
(`internal/store/types.go:16`), no new state. The all-present arm continues to
use `updateFeatureState` and the confirmation gate, unchanged.

**The pointer survives every unrelated status write, and that needs its own
rule.** `updateFeatureState` does not merge: it **replaces** `status.Reconcile`
with a freshly constructed `store.ReconcileSummary` literal
(`internal/workflow/reconcile.go:1191-1209`) built entirely from the current
`ReconcileResult`. Without a rule, the next ordinary reconcile of a slug with a
pending candidate would silently drop `operation_candidate` while leaving the
candidate directory on disk — a pending candidate nobody can address, and a
`--accept-candidate` that reports `candidate-absent` beside a real worktree.

The contract is a **shared status merge**, used by every writer of
`status.Reconcile`:

1. load the current status;
2. build the new `ReconcileSummary` from the result, exactly as today;
3. **before** assigning it, copy the **loaded** `Reconcile.OperationCandidate`
   into the new summary;
4. assign, then apply whatever state and note rules that writer has.

Step 3 is the whole rule, and its position matters: the carry-forward happens
**before** the standard `ReconcileSummary` replacement, not as a repair
afterwards, so no window exists in which the pointer is absent from the value
about to be written. `updateFeatureState` and the candidate-ready path both
call the same helper, so there is exactly **one** place in the codebase where
the pointer is preserved and exactly one place a reviewer has to check.

**The carry-forward is not a lifecycle-state change.** It copies one
`omitempty` sub-object and touches nothing else: `status.State`,
`status.Notes`, `status.Apply` and every other summary field keep the semantics
they have today, and the §6.11 bypass above is unaffected. A feature with a
pending candidate that reconciles to `blocked` is still recorded `blocked`.

**Exactly five writers may set, modify or clear the pointer**, and every one of
them is an explicit candidate transition under the per-feature lock:

| Transition | Writer | Effect on the pointer |
|---|---|---|
| finalize | candidate finalization (§6.2) | **sets** it to the newly persisted `pending` candidate |
| stale | staleness preflight (§6.11 step 2) | **clears** it, after marking `state.json` `stale` and pruning `worktree/` |
| reject | `--reject-candidate` (§6.18) | **clears** it, after appending to `rejected.jsonl` |
| accept | `--accept-candidate` (§6.14 steps 12 and 14) | updates it at step 12, **clears** it at step 14 |
| recovery | `--resume-candidate-recovery` (§6.15) | restores the `pending` pointer on rollback, or completes step 14's clear on a committed journal |

**Every other outcome carries the pointer forward untouched**, including: a
phase-1 reverse-apply `upstreamed`, a phase-1.5 patch-id `upstreamed`, a
phase-3 provider verdict, a clean phase-4 `reapplied`, a phase-4 `blocked`, a
run with `operation_candidate_enabled` off, a multi-slug or
`--cumulative-legacy` invocation that never evaluates the candidate path, and a
run whose preflight refused with `candidate-lock-held`, `lock-unsupported` or
`lock-filesystem-unsupported`. In each case the pending candidate remains
addressable by `--candidate-diff`, `--accept-candidate` and
`--reject-candidate` afterwards.

**Carrying the pointer forward is not the same as keeping the candidate
acceptable.** An outcome that moves the feature out of §6.6a's permitted set —
a phase-1 `upstreamed` transitions it to `store.StateUpstreamMerged`
(`internal/workflow/reconcile.go:1214-1216`) — leaves the pointer in place and
lets `--accept-candidate` refuse `candidate-feature-state-changed` with both
states named (§6.14 step 3). That is the designed answer: the operator is told
why the staged proposal no longer applies, and can still inspect it with
`--candidate-diff` or discard it with `--reject-candidate`. Dropping the
pointer instead would report `candidate-absent` for a directory that plainly
exists, which is the failure mode this rule removes.

**Pointer and directory stay aligned.** The invariant is two-way: a pointer
must name a directory whose `state.json` says `pending`, and a `pending`
directory must have a pointer. The five transitions above are the only places
either side changes, and they change both sides under the same lock hold. A
source guard enumerates every construction of a `store.ReconcileSummary` in
production code and requires each one to come from the shared merge or to be on
the five-writer allowlist; anything else fails the guard.

**One pending candidate per slug**, enforced by the staleness check above.

**Candidate and resolver shadow may not coexist:**

| Attempted | Existing | Refusal |
|---|---|---|
| `--resolve` / `--accept` / `--reject` / `--shadow-diff` | pending candidate | `candidate-pending` + "run `tpatch reconcile <slug> --reject-candidate` or `--accept-candidate` first" |
| `--accept-candidate` / `--reject-candidate` / `--candidate-diff` / `--clear-candidate-rejections` | resolver shadow | `shadow-pending` + "run `tpatch reconcile --accept <slug>` or `--reject <slug>` first" |
| candidate derivation | resolver shadow present | E15 ineligibility `shadow-pending`, fall through |

### 6.12 CLI surface and the exact flag inventory

**Five** new terminal flags, each taking **exactly one** slug:

| Flag | Behavior |
|---|---|
| `--candidate-diff <slug>` | print the pending candidate's reviewable diff; read-only; no state, no artifact |
| `--accept-candidate <slug>` | run the §6.14 acceptance transaction |
| `--reject-candidate <slug>` | record the rejection, prune, clear the pointer (§6.18) |
| `--clear-candidate-rejections <slug>` | truncate this slug's `rejected.jsonl`; does nothing else |
| `--resume-candidate-recovery <slug>` | complete or roll back a recorded acceptance journal (§6.15) |

**Three** existing terminal flags are unchanged: `--accept`, `--reject`,
`--shadow-diff`. **Eight terminal actions exist in total.**

**Exclusivity is source-derived, not a hand-maintained list.** rev-1 pinned a
fixed count of 55 pairs against a hand-copied list of nine existing flags,
which was wrong twice over: it omitted `--upstream-ref`, `--allow-dirty`,
`--allow-stale-lock`, `--max-conflicts`, `--model` and `--timeout`, and it
would silently go stale the moment `reconcileCmd` registered another flag.
rev-2 derives the rule from the command's own flag set:

> A new terminal flag may be combined **only** with a flag on the presentation
> allowlist. Every other flag registered on the reconcile command refuses when
> it is **explicitly set**, tested with `Flags().Changed(name)` so that a
> default value is never mistaken for an operator's intent.

| Set | Members | Rule |
|---|---|---|
| presentation allowlist | the root persistent `--path` (`internal/cli/cobra.go:73`) and reconcile's `--format` (`:2543`) | permitted beside a new terminal |
| excluded by name | Cobra's auto-registered `help` flag | **not** a behavioral flag and **not** counted |
| reconcile-local behavioral flags | every other flag registered at `internal/cli/cobra.go:2520-2543`: `--upstream-ref` (`:2520`), `--timeout` (`:2521`), `--preflight` (`:2522`), `--allow-dirty` (`:2523`), `--allow-stale-lock` (`:2524`), `--resolve` (`:2526`), `--apply` (`:2527`), `--max-conflicts` (`:2528`), `--model` (`:2529`), `--accept` (`:2530`), `--reject` (`:2531`), `--shadow-diff` (`:2532`), `--check-applied-only` (`:2534`), `--auto-drop-merged` (`:2535`), `--cumulative-legacy` (`:2542`) | each refuses when `Changed` |
| new × new unordered pairs | 10 | each refuses |

**`help` is excluded explicitly, by name.** `cmd.Flags()` does not return the
flag set that `reconcileCmd` registered: Cobra adds `help` to a command during
execution, so a naive enumeration would find **sixteen** non-presentation flags
and report `5 × 16 = 80` plus `10`, that is `90` refusing pairs — and would
refuse `--accept-candidate <slug> --help`, which is not a conflict but a
request for the command's own documentation. `help` is therefore subtracted
from the enumeration alongside the presentation allowlist, and the subtraction
is by flag **name**, asserted against the registered set so that it cannot
silently start matching something else.

**There is no `--json` flag on this command.** The reconcile command's
presentation flag is `--format` with values `human` and `json`
(`internal/cli/cobra.go:2543`); the root command's only persistent flag is
`--path` (`:73`). The allowlist names exactly what exists.

**Recomputed counts, for the current flag set.** Fifteen behavioral flags are
registered today — sixteen enumerated members at `internal/cli/cobra.go:2520-2543`
minus `--format`, with `help` excluded as above — so the refusing pairs that
involve a new flag are `5 × 15 = 75` new×existing plus `10` new×new, **85** in
total. The count is stated as a consequence of the source-derived rule, not as
the rule: the implementation enumerates `cmd.Flags()`, subtracts the two-member
presentation allowlist and `help`, and the test asserts the enumeration against
the registered set, so adding a sixteenth reconcile flag updates the guard
automatically instead of silently escaping it. An implementation that hardcodes
a list of behavioral flag names fails §9.9's inventory guard, and one that
counts `help` reports `90` and fails it too.

**Explicit `--upstream-ref` is refused, and this is deliberate.** A terminal
candidate action operates on the `upstream_commit` **bound into the candidate**
(§6.10). Accepting an operator's alternative ref beside `--accept-candidate`
would either be ignored — which is a lie — or would re-resolve against a
different commit than the one the candidate proved, which is the exact
substitution the per-path CAS exists to prevent. Refusing at validation, before
anything is read, is the only honest answer. `--timeout` and `--model` are
refused for the same reason: there is no provider call and no phase pipeline in
a terminal candidate action for them to govern.

**Existing combinations are never re-routed.** The new rule is a second
validation stage that runs **only when at least one new flag is set**. An
invocation using only shipped flags reaches exactly the shipped
`validateReconcileFlags` path (`internal/cli/cobra.go:3200`, rules at
`:3211-3218`), produces exactly the shipped error strings (`:3212,3215,3218`),
and is covered by an S0 golden. `--apply` still requires `--resolve` with its
shipped message.

**The existing three stay resolver-only.** `--accept`, `--reject` and
`--shadow-diff` keep their exact shipped meanings and never touch a candidate.
`--accept <slug>` with a pending candidate and no shadow refuses by name.

Exit codes follow the shipped ADR-031 D4 envelope (`internal/cli/reject.go:36-47`):
`2` for pre-mutation validation refusals (flag misuse, absent or ineligible
candidate, failed CAS, diverged HEAD), `3` for post-validation state-machine
refusals (coexistence, lock contention, `recovery-required`, a rolled-back
acceptance), `1` only for unexpected internal errors.

### 6.13 Locks: a new portable kernel-advisory abstraction

**`internal/rescap`'s lock is not reused and its contract is not
re-specified.** That lock is bound to the resource-capture surface: a `statfs`
filesystem preflight (`internal/rescap/lock_unix.go:64-70`), a scratch root,
and callers that must first pass `rescap.EnsureLocalContract`
(`internal/rescap/scratch.go:48`, `internal/cli/feature_resource.go:107`). It
also refuses on every non-linux/darwin target
(`internal/rescap/lock_unsupported.go:29-31`). Reconcile needs the same kernel
primitive under a different, portable contract.

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
  crash and power loss — so a "stale" lock cannot exist. `--allow-stale-lock`
  (`internal/cli/cobra.go:2524`) governs ADR-017's baseline classifier
  (`internal/gitutil/lock_guard.go:99-100`) and does **not** apply here.
- **Lock-file residue is inert.** An empty `.tpatch/locks/**.lock` left behind
  is never read, never parsed and never a signal. It is not cleaned up on a
  timer and its presence means nothing.
- **Paths are explicit and safe:** `.tpatch/locks/reconcile/<slug>.lock` and
  `.tpatch/locks/worktree-mutation.lock`, opened `O_CREATE|O_RDWR 0600` under
  the store root after `safety.EnsureSafeRepoPath`. There is no `.tpatch/local`
  directory, no `EnsureLocalContract` precondition and no ignore-file contract;
  the only preflight is this abstraction's **own** filesystem-class refusal
  above, which is not `rescap`'s and does not import its reason codes.
- **Contention refuses immediately** with `candidate-lock-held` and exit `3`.
  There is no wait, no retry and no timeout.
- **Acquisition order is fixed — per-feature lock, then global
  worktree-mutation lock — for consistent, explainable behavior, not for
  deadlock prevention.** A nonblocking lock cannot deadlock. If the second
  acquisition fails, the first is released before the refusal returns, so a
  failed attempt leaves nothing held.

**Scope — every candidate-state mutation is inside a hold.**

| Phase of work | Locks held | Why |
|---|---|---|
| §6.11 preflight (before phase 1): journal inspection, staleness marking, worktree prune, pointer clear | per-feature | read-modify-write on candidate state that acceptance also mutates (B6) |
| phases 1-4, including every provider call | none | the preflight releases before expensive work; nothing candidate-shaped is mutated here |
| §6.2 finalization: derive-to-disk, worktree create, artifact write, pointer set | per-feature for the whole finalization; global **only** around `git worktree add` / `git worktree remove` | reacquired and **revalidated** — the preflight's observations are not trusted across the released window |
| §6.14 acceptance | per-feature **and** global, for the whole transaction | steps 1-14 are one read-modify-write over live paths and artifacts |
| §6.18 rejection, §6.15 recovery | per-feature | both mutate `state.json`, the pointer and the worktree |

**There is no unlocked prune, mark, pointer write or rejection append
anywhere.** A candidate action that cannot take the lock refuses; a plain
reconcile that cannot take it disables candidate work for that invocation and
continues (§6.11 step 0). Candidate *derivation* — reading two trees and
computing an alignment — holds nothing, because it mutates nothing.

**Unsupported platforms are named, not silent.** With `Supported == false`,
candidate finalization records a **nonterminal** phase-2 refusal
`lock-unsupported` and falls through, so reconcile behaves exactly as it does
today on that platform; `--accept-candidate` refuses with the same named code
and exit `3`.

### 6.14 Acceptance is a new journaled transaction

`AcceptOperationCandidate` is new. **`AcceptShadow`
(`internal/workflow/accept.go:82`), `CopyShadowToReal`
(`internal/gitutil/shadow.go:191`) and `RefreshAfterAccept`
(`internal/workflow/refresh.go:45`) are never called on a candidate path**, for
the five reasons in §2.5.

**Preconditions replacing rev-0's HEAD equality.** rev-0 required
`git rev-parse HEAD == upstream_commit`, which is unreachable whenever an
operator's branch carries any commit of its own.

- **Feature state.** The feature's current `status.State` must equal the
  candidate's bound `source_feature_state` **and** must still be in §6.6a's
  permitted set. Any drift refuses `candidate-feature-state-changed` with both
  states named and exit `3`, before the journal, before the snapshots and
  before any live write.
- **Ancestry.** `upstream_commit` must be HEAD **or an ancestor of** HEAD
  (`git merge-base --is-ancestor <upstream_commit> HEAD`). Otherwise refuse
  `head-diverged` with both commits named and exit `2`. This is an ordering
  diagnostic, not the safety property.
- **Exact per-path compare-and-swap, which *is* the acceptance authority.** For
  **every** path the candidate touches, the live tree must match the
  `upstreamCommit` tree exactly: a modified path must exist live with
  **byte-equal** content and the same mode as that tree's blob; a created path
  must be **absent** live — an untracked file there refuses with
  `candidate-path-occupied` and destroys nothing. Any mismatch is
  `candidate-cas-failed`, exit `2`.
- **The CAS runs twice: at step 6, and again immediately before the first live
  write.** The second pass covers exactly the same path set with exactly the
  same comparison and the same `candidate-cas-failed` refusal, and it runs
  inside step 10 before a single byte is written, so the window between proving
  the tree and mutating it is as small as the design permits.
- **Unrelated dirt is allowed.** Uncommitted edits outside the candidate path
  set neither block acceptance nor are touched by it.

Among the preconditions the CAS is decisive: no earlier check overrides it, no
ancestry result substitutes for it, and there is no tolerance of any kind. What
it is **not** is a mutual exclusion. It proves the tree matched at the two
moments it ran, and the honest scope of the surrounding machinery is:

- the advisory locks (§6.13) serialize **`tpatch` invocations on one host**.
  They are kernel-advisory and they do not constrain the operator's editor, a
  build tool, a second checkout, a CI runner or a process on another host;
- **no surface in this document claims that a writer outside `tpatch` — local
  or cross-host — is prevented from mutating a candidate path after the step-10
  recheck.** Such a write is possible, and saying otherwise would be a
  guarantee this design cannot keep;
- what covers that case is **recovery, not prevention**: the acceptance journal
  and the snapshot rollback (§6.15) leave a deterministic, explicitly resumable
  state rather than a half-written tree, and the post-transaction artifacts
  name the exact commit and path set the acceptance was computed against.

**The transaction — fourteen ordered steps. This is parity block E, reproduced
byte-identically in ADR-037 D29.**

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
written before step 10's write begins, in this document, in ADR-037 or in the
implementation. Step 10's leading compare-and-swap is a **read-only
precondition inside the step**, not a fifteenth step and not a write: it
re-proves, immediately before the mutation, the same per-path equality step 6
proved, and it refuses with the same `candidate-cas-failed` and the same exit
`2`. A refusal there leaves the journal on disk with no commit marker, so the
transaction is recoverable exactly as any other pre-marker failure is (§6.15).

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
with them, and every path recorded with an absent marker is **deleted**. The
candidate returns to `status: pending`. A failure at step 11 or step 12 — a coverage-publication
failure, a staged-artifact write failure or a state-transition failure — is a
**failure**: it rolls back and returns non-zero. It is never a printed warning
beside a success line, and there is no success-shaped partial acceptance.
`AcceptShadow`'s best-effort refresh and prune (`internal/workflow/accept.go:110,128-131`)
are explicitly not the model.

**Exit codes are tight.** A completed rollback is a state refusal, not an
internal error: `acceptance-rolled-back` and `coverage-publication-failed` exit
`3` with the failing step named. Only a **failed rollback** is exit `1`
(`acceptance-rollback-failed`), and it retains the journal, so every subsequent
command is `recovery-required`.

**No filesystem-wide atomic-rename claim is made anywhere.** The guarantee is
"journaled, rollback-capable, explicitly recoverable", and every surface says
exactly that.

### 6.15 Recovery is explicit and deterministic

A crash between step 9 and step 14 leaves `accept-journal.json`.

**Detection is universal and early.** §6.11's preflight runs before phase 1 for
every reconcile invocation naming the slug, and before any candidate or
resolver action for it. A journal present refuses the command with
`recovery-required` and exit `3`, and does nothing else — no phase runs, no
staleness pruning, no evidence append.

**`--resume-candidate-recovery <slug>` is the only way out**, and it does not
guess. It runs under the per-feature lock for the whole resume (§6.13), because
restoring snapshots and removing the journal are candidate-state mutations like
any other:

| Journal state | Action |
|---|---|
| commit marker **absent** | deterministically roll back from `snapshots/`: restore every snapshotted live path and every snapshotted artifact — `artifacts/post-apply.patch`, `artifacts/apply-recipe.json`, `artifacts/recipe-provenance.json`, `artifacts/recipe-coverage.json`, `artifacts/patch-generations.json` and `status.json`, together with the recorded `status.Apply` and `status.State` — delete every path recorded with an absent marker, set `state.json` back to `pending`, restore the `status.json` candidate pointer, and remove the journal |
| commit marker **present** | the transaction is committed; complete steps 14's cleanup — prune the worktree, clear the pointer, remove `snapshots/`, `staged/` and the journal |

There is no implicit recovery-on-next-run, no heuristic, no partial replay and
no inference from timestamps or file presence beyond the marker itself. A
resume that cannot complete leaves the journal in place and exits `1`.

### 6.16 Coverage publication on acceptance

Acceptance publishes through **ADR-036 D15's single shared publication API**.
**This restates ADR-036 D15's producer P3 in full as two variants over one enum
value and one schema version, and is parity block F.**

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
restricted to the candidate path set. The `preimage_set_sha256` is recomputed
over that same commit's tree, and the ADR-024 generation entry staged beside it
carries `base_commit = upstream_commit`, so `status.Apply.BaseCommit`
(§6.14 step 12), the generation manifest and the coverage reference all name
one commit.

**No `producer` enum value is added and `schema_version` stays `1`.** The
variant is a property of the input to the shared API, exactly as ADR-036 D15
requires.

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
overstated what §6.16 closes. The honest accounting, against ADR-036 D15's
seven producers (`docs/adrs/ADR-036-recipe-coverage-authority.md:1960-1966`):

| Producer | GH #15 assigns regeneration to GH #13 | What GH #13 v1 actually does |
|---|---|---|
| P3 `reconcile-accept` (`:1962`) | yes | **discharged for the `operation-candidate-accept` variant only.** The `resolver-accept` variant stays `incomplete` by design |
| P4 `cycle` (`:1963`) | yes | **not addressed.** No gate, no publication path, no matrix row; §11 keeps it deferred |
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
ownership statement needs a regeneration contract of its own, which §11 carries
as a deferral with its reopen trigger.

**The residual is smaller on purpose.** It excludes effects already absorbed
upstream (§6.8), because those bytes are no longer this feature's diff against
this upstream, and the new recipe and coverage explain what remains
**completely** — which is what stops a feature degrading with each successful
reconcile.

### 6.17 Evidence

Candidate evidence uses the **existing** ADR-025 D4 values — `phase: phase-2`,
`evidence_kind: recipe-operation-match`
(`internal/store/reconcile_evidence.go:41`,
`internal/workflow/reconcile.go:1167-1168`). **No new phase value, no phase 2.5,
no new evidence kind.**

| Phase-2 result | `confidence` |
|---|---|
| all effects present with CG1-CG7 satisfied | `high` |
| candidate finalized and fully validated | `high` |
| conflicting effect | `low` |
| any E-gate ineligibility | `unknown` |

The all-present entry's remaining fields are pinned by §6.2's field table —
`match_origin: upstream`, `pre_reconcile_presence: present`,
`upstream_commit_refs: [upstreamCommit]` and `requires_confirmation: false` —
while the shipped legacy builder keeps its `low` / `unknown` / `[]` triple
(`internal/workflow/reconcile.go:826,829,843-845`). The two entries are
therefore distinguishable by value, not only by provenance. A candidate-ready
invocation additionally stages a **second** attempt at `phase: phase-4`,
`evidence_kind: forward-apply`, `confidence: low` — the shipped
`forward-apply`-plus-blocked rung (`:846-849`) — carrying the phase-local
verdict `blocked`; its exact fields are pinned by §6.2's two-attempt table.
That is still an existing ADR-025 D4 enum pair; no new phase and no new kind is
introduced by it.

**Multiple ordered attempts per invocation, split into verdict-bearing and
supporting entries.** This amends ADR-025 D1/D3's one-attempt-per-verdict
reading (`docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:70-72`).

rev-1 claimed "at most one attempt per (invocation, phase) pair". **That is
false against the shipped code**, and the counter-example is not hypothetical:
one invocation already appends up to four lines carrying `phase-3.5` and up to
two carrying `phase-4`. rev-2 corrected the rule but miscounted the classes —
it called all four `phase-3.5` lines supporting, although one of them is the
main pipeline verdict, and it treated the confirmation gate's entry as
coexisting with the blocked-only detectors, which the shipped guards forbid.
rev-3 states the shipped shape exactly.

| Call site | Entry | Phase written | Fires when |
|---|---|---|---|
| `internal/workflow/reconcile.go:766` | `persistReconcileEvidence`, phase derived from `result.Phase` (`:1161-1176`) — or, when the GH #13 evaluator staged any, the whole of `result.verdictAttempts` in slice order **in place of** the derived entry (§6.2 / D2) | any of `phase-1`, `phase-1.5`, `phase-2`, `phase-3`, `phase-3.5`, `phase-4` | any non-empty outcome (`:813-815`) |
| `:767` | file-novelty detector (`internal/workflow/file_novelty.go:115-116`) | `phase-3.5` | any outcome with an upstream commit, a base commit and a canonical patch (`:884-898`) |
| `:768` | hunk-overlap detector (`internal/workflow/hunk_overlap.go:135-136`) | `phase-3.5` | the same, plus a modifying or mixed-additive novelty classification (`:913-923,928-930`) |
| `:769` | path-restructure detector (`internal/workflow/path_restructure.go:118-119`) | `phase-3.5` | **blocked outcomes only** (`:947-949`) |
| `:770` | blocked-classification taxonomy (`internal/workflow/blocked_taxonomy.go:179`) | `phase-4` | **blocked outcomes only** (`:998-1000`) |
| `:771` | the confirmation gate's own entry (`internal/workflow/reconcile.go:1069-1070`) | `phase-3.5`, kind `manual-review` | **`upstreamed` outcomes only** (`:1033-1035`) |

**The shipped maxima, per invocation:**

| Phase | Verdict-bearing | Supporting | Total lines |
|---|---|---|---|
| `phase-3.5` | at most 1 — the main entry when `result.Phase` is a phase-3.5 value | at most **3** | **4** |
| `phase-4` | at most 1 — the main entry when `result.Phase` is a phase-4 value | at most **1** (blocked-classification) | **2** |

**The three supporting `phase-3.5` entries are not always the same three, and
the gate's entry is mutually exclusive with the blocked-only pair.** The gate
runs at `:771` only while the outcome is still `store.ReconcileUpstreamed`
(`:1033-1035`); path-restructure (`:769`) and blocked-classification (`:770`)
run only on a blocked outcome. Since `:769` and `:770` execute **before** the
gate can demote an unconfirmed `upstreamed` to `blocked` (`:1089-1090`), no
invocation writes both:

| Invocation shape | Supporting `phase-3.5` entries | Supporting `phase-4` entries |
|---|---|---|
| ends blocked | file-novelty, hunk-overlap, path-restructure | blocked-classification |
| ends `upstreamed` (confirmed or demoted by the gate) | file-novelty, hunk-overlap, the gate's `manual-review` | none |
| ends `reapplied` or `still_needed` | file-novelty, hunk-overlap | none |

A claim of "four supporting `phase-3.5` entries" is therefore wrong in every
shape; the correct statement is **one verdict-bearing entry plus at most three
supporting ones**.

**Two classes of entry.**

| Class | Members | Cardinality | `raw_reconcile_verdict` |
|---|---|---|---|
| **verdict-bearing** | the main pipeline result entry (`:766`, defined `:812`), which on the two authoritative arms **is** the staged list — one qualifying GH #13 phase-2 proof entry on the all-present arm, two ordered attempts on the candidate-ready arm — emitted in place of the derived entry rather than beside it; the derived entry itself on every other arm; the final overall-phase entry | **at most one per (invocation, phase)** | the **phase-local** verdict of that phase |
| **supporting** | file-novelty, hunk-overlap, path-restructure, blocked-classification, and the confirmation gate's `manual-review` entry | **unordered in principle, at most three per phase in the shipped pipeline**; several may share one phase | **shipped semantics, unchanged** |

- **Verdict-bearing** entries are the ones a reader may treat as "this phase's
  answer". Exactly one may exist per (invocation, phase): a second phase-2
  verdict-bearing entry in one invocation is a defect — which is why both
  authoritative arms **replace** the derived `:766` entry with their staged
  list instead of appending beside it (§6.2).
- **Supporting** entries are detector and gate observations. They are retained
  exactly as shipped — same phase, same `evidence_kind`, same `confidence`,
  same `raw_reconcile_verdict`, same `reason_code`, and **the same firing
  conditions**. Nothing in this document makes a detector run on an outcome it
  does not run on today, and nothing suppresses one it does. The confirmation
  gate's entry keeps `phase: phase-3.5` and `evidence_kind: manual-review`
  (`internal/workflow/reconcile.go:1069-1070`) even when its subject is a
  phase-2 result, because that is what it means today and this PRD does not
  re-key it. Nothing about `phase-2` proof authority flows through it.
- The phase-local verdict rule is **scoped to verdict-bearing entries only**. A
  supporting entry's verdict field is not a phase verdict and is not read as
  one: `blocked-classification` writes `blocked` because that is its subject,
  and the confirmation gate writes `upstreamed` because that is the result it
  is grading.

**A candidate-ready invocation is the worked example.** Phase 2 proves a
candidate, phase 3 declines, phase 4 conflicts textually, and the candidate is
finalized, so the invocation returns `store.ReconcileStillNeeded`. Its
`result.Phase` is `phase-2-operation-candidate`, so `evidencePhaseAndKind`
(`:1161-1176`) *would* derive a second `phase-2` / `recipe-operation-match`
entry (`:1167-1168`) — which is exactly why this arm stages a list and
substitutes rather than appending:

1. **verdict-bearing**, staged attempt [1]: `phase: phase-2`,
   `evidence_kind: recipe-operation-match`,
   `raw_reconcile_verdict: still_needed`, `confidence: high` — the phase-2
   attempt, appended first at the shipped `:766` position;
2. **verdict-bearing**, staged attempt [2]: `phase: phase-4`,
   `evidence_kind: forward-apply`, `raw_reconcile_verdict: blocked`,
   `confidence: low`, `reason_code: phase-4-forward-apply-conflicts` — the
   textual-conflict finding that explains why ordinary patch replay could not
   win, appended immediately after [1], still at `:766`. **The derived entry
   is suppressed**, so no third verdict-bearing line and no duplicate `phase-2`
   line exists;
3. **supporting**, `phase: phase-3.5` — file-novelty and hunk-overlap, in that
   fixed order, whenever their shipped preconditions hold;
4. **no** path-restructure entry and **no** phase-4 blocked-classification
   entry: both are blocked-only (`:947-949`, `:998-1000`) and this invocation
   is not blocked.

Attempt [2] is the terminal phase's own verdict-bearing entry. Without the
staged list the arm had **no carrier** for it at all: `:766` derives from
`result.Phase`, which is a `phase-2` value here, so the phase-4 finding would
have been lost and the phase-2 finding duplicated.

**Candidate-ready neither claims nor forces the blocked-only detectors.** A
candidate replaces a `blocked` answer with a `still_needed` one, so the two
detectors that only speak about blocked outcomes legitimately fall silent.
Forcing them to run would mean either lying to `ClassifyBlockedVerdict` about
the outcome or duplicating its entry under a non-blocked verdict — both are
changes to shipped detector semantics, and this PRD makes neither.

A plain fallthrough (a phase-2 `blocked` classification followed by a clean
phase-4 reapply) takes the **direct-append** route below instead: a
verdict-bearing phase-2 entry carrying `blocked`, appended by the evaluator at
phase-2 time; file-novelty and hunk-overlap as supporting entries; and the
shipped derived terminal entry carrying `reapplied`. Its
`result.verdictAttempts` is empty, so nothing is substituted and the shipped
builder runs verbatim. A fallthrough that ends **blocked** at phase 4
additionally gets path-restructure and blocked-classification, which is the
four-line / two-line maximum above.

**Append order is deterministic.** The staged attempts are appended at the
shipped `:766` position, in slice order and ahead of every supporting entry;
then the shipped call order `:766` → `:771` is preserved exactly, with each
call site's own firing condition unchanged. Two runs over the same inputs
produce the same lines in the same order, which is what makes the JSONL
diffable.

**Exactly one verdict-bearing entry per phase, on every arm, over two routes.**
The GH #13 evaluator's phase-2 attempt reaches `reconcile-evidence.jsonl` by
one of two routes, and which one is decided by a single question: **would
`evidencePhaseAndKind` (`internal/workflow/reconcile.go:1161-1176`) derive
`phase-2` from this invocation's terminal `result.Phase`?**

- **Staged substitution** — yes, it would collide. That happens on exactly two
  arms, because they are the only two whose terminal `result.Phase` takes the
  `phase-2` prefix branch (`:1167-1168`): the all-present return
  (`phase-2-operation-level`, `:451-457`) and the candidate-ready outcome
  (`phase-2-operation-candidate`). Both stage (§6.2 configurations 2 and 3), so
  on both the derived entry is **suppressed** and the substitution is
  like-for-like: same call site, same position, one line per phase.
- **Direct append** — no, it cannot collide. On a phase-2 refusal or conflict
  the pipeline falls through, and every terminal arm downstream of phase 2 sets
  a phase-3, phase-3.5 or phase-4 `result.Phase`, so the derived entry is never
  `phase-2`. The evaluator appends its phase-2 verdict-bearing attempt
  directly at phase-2 time, `result.verdictAttempts` stays empty (§6.2
  configuration 1), and the shipped derived builder produces the terminal
  entry verbatim. **No arm emits a staged attempt and a derived entry
  together**; the direct-append route is not staging and does not suppress
  anything.

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
| legacy all-present with `operation_candidate_enabled` **off** (D12b / §6.4) | 0 | `phase-2`, legacy builder verbatim (`:826,829,843-845`) | `phase-2`: 1 |
| candidate-ready, returning `still_needed` | **2** — phase-2 then phase-4 | `phase-2`, from `phase-2-operation-candidate` — **suppressed**, replaced by the staged list | `phase-2`: 1, `phase-4`: 1 |
| phase-2 refusal or conflict — including, with the gate **on**, every legacy or ineligible all-present result — a later phase answers | 0 | that later phase's entry, unchanged | `phase-2`: 1, appended directly by the evaluator; that later phase: 1 |

**Row 2 is the gate-off arm only.** The legacy builder's derived `phase-2`
entry survives exactly where `result.Phase` is still a `phase-2` value without
GH #13 having staged anything, and with `operation_candidate_enabled` **on**
that never happens: the shipped `opResult.allPresent` early return
(`:451-457`) is removed from the authoritative path (§6.4), so a legacy or
E-gate-ineligible all-present result falls through with its phase-2 refusal
evidence and earns a later phase's outcome — row 4's **direct-append** route,
not row 2's. Row 2 is therefore the byte-identical shipped behavior the S0
goldens pin, and it is reachable only with the gate off.

A staged list that is not one of §6.2's three legal configurations — a
`phase-2` staged attempt duplicated, a derived entry appended beside a staged
one, a proof beside the two-attempt list, the two attempts out of phase order,
or two staged entries sharing an `attempt_id` — is a defect and is rejected by
§6.2's source guard and by `saveReconcileArtifacts`'s configuration check.


**Idempotency and corruption are ADR-025 D3's, unchanged.** An identical
existing `attempt_id` is a **no-op skip**
(`internal/store/reconcile_evidence.go:166-170`); the same `attempt_id` with
different bytes is corruption and refuses with `ErrMalformedEvidence` (`:171`).
Because `attempt_id` is content-addressed, two supporting entries sharing a
phase never collide unless their payloads are byte-identical, in which case the
dedupe is correct.

Each verdict-bearing attempt's `raw_reconcile_verdict` is that **phase's own**
verdict — `upstreamed`, `still_needed` or `blocked` — and it may differ from
the invocation's returned outcome. A phase-2 evaluation that proved a candidate
writes `still_needed` on its own entry even when the invocation ultimately
returns `reapplied`; a phase-2 conflict writes `blocked` on its own entry even
when phase 4 later reapplies cleanly. The terminal phase always gets its own,
separate verdict-bearing entry. `RawReconcileVerdict` is already an
unconstrained string on the shipped schema
(`internal/store/reconcile_evidence.go:96`), so this needs no schema change —
only the ordered attempt list that substitutes for `evidencePhaseAndKind`'s
single derivation from `result.Phase` on the two staging arms
(`internal/workflow/reconcile.go:1161-1176`, §6.2), plus the evaluator's
direct append on the fallthrough arms.

**CLI display.** `tpatch reconcile review <slug>`
(`internal/cli/cobra.go:2544`) renders every attempt of the latest invocation
in phase order, labelling each verdict-bearing `raw_reconcile_verdict` as that
attempt's **phase-local verdict**, labelling supporting entries as supporting
evidence rather than verdicts, and printing the invocation's **returned
outcome** separately from `status.json:reconcile.outcome`. A rendering that
presents a phase-local verdict as the command's answer, or a supporting
entry's verdict field as a phase verdict, fails the contract.

**Migration is honest.** The new authoritative phase-2 entry carries different
field values from anything a pre-GH-#13 run produced, so it **intentionally
mints a new `attempt_id`**. No byte identity with historical entries is
claimed anywhere. Idempotency is ADR-025 D3's, unchanged: an identical existing
`attempt_id` is a **no-op skip**
(`internal/store/reconcile_evidence.go:166-170`), and the same ID with
different bytes refuses with `ErrMalformedEvidence`
(`internal/store/reconcile_evidence.go:171`).

**No source bodies in evidence** — ADR-025 D10 and ADR-029 D8 apply unchanged.
`matched_paths` and `matched_operations` carry slash-normalized paths and
operation identifiers only.

**Diagnostic parity is measured in semantic fields, not in evidence lines.** A
terminal candidate-ready result must be at least as diagnostic-rich as the
phase-4 blocked path it replaces. The comparison is over **what an operator can
learn**, not over how many JSONL lines were appended:

| Question an operator asks | Phase-4 blocked path | Candidate-ready path |
|---|---|---|
| which files are involved? | file-novelty and hunk-overlap entries (`:767-768`) | the same two entries, unchanged, plus `candidate.json`'s `effect_map`, which names **every** effect and its path |
| what happened to each change? | not answered per change — the detectors classify the patch, not the effects | `effect_map`'s `resolution` per effect: `absorbed-present` or `candidate-applicable` |
| why did something not work? | `blocked-classification`'s reason code for the invocation (`:770`) | the per-effect `reason_codes`, plus the phase-2 verdict-bearing entry's own `reason_code`, `confidence` and counts |
| where would the change land? | not answered | `candidate.patch`, a reviewable unified diff, and the derived operation list |
| what is the exact answer? | one `blocked` verdict | a `still_needed` verdict plus an artifact that can be diffed, accepted or rejected |

**No claim of equal detector-entry counts is made, and none should be.**
path-restructure and blocked-classification are blocked-only
(`internal/workflow/reconcile.go:947-949,998-1000`), so a candidate-ready
invocation does not — and must not — emit them. Matching their line count would
require running a blocked-outcome detector on a non-blocked outcome, which
changes shipped detector semantics for every feature, not just this one. The
contract is the field coverage above; a candidate result that carries counts
but no per-effect mapping fails it.

### 6.18 Rejection semantics

`--reject-candidate`, under the per-feature lock for the whole action (§6.13),
(1) appends `candidate_id`, `source_feature_state`, `upstream_commit`,
`reference_commit` and the three binding hashes to `rejected.jsonl` — no bodies,
no timestamp; (2) sets `state.json` to `status: rejected` and prunes
`worktree/`, retaining the immutable `candidate.json`; (3) clears the
`status.json` pointer; (4) **changes no lifecycle state**. A candidate at
`accepted` is not rejectable: it refuses `candidate-not-pending` (§6.10).

**A rejected ID does not regenerate.** §6.10's pre-creation check means a later
reconcile deriving an ID already in `rejected.jsonl` records
`candidate-rejected` and falls through **before** creating any directory or
worktree. Identical inputs producing an identical ID means the operator already
answered this exact question.

**Changed bindings mint a new ID** and a new candidate, normally.

**Reconsidering is explicit.** `--clear-candidate-rejections <slug>` truncates
the log and does nothing else — no regeneration, no state change, no pruning.
There is no inferred retry.

### 6.19 Scope, supersession and hard prohibitions

- **Supersession never grants eligibility.** ADR-029 D7's downgrade to
  warning-class (`docs/adrs/ADR-029-write-file-recipe-safety.md:74-76`) and
  ADR-028's exclusion from default effective replay
  (`docs/adrs/ADR-028-supersession-edge-model.md:77-88`) are severity and scope
  statements, not authority. E12 refuses regardless.
- **A hard-parent-blocked child skips candidate derivation** (E13) and falls
  through, before any alignment work. ADR-011 semantics are unchanged, and the
  shipped phase-3.5 blocked-by-parent short-circuit
  (`internal/workflow/reconcile.go:552-567`) is untouched.
- **Single-slug only in v1**; multi-slug and `--cumulative-legacy` fall through
  unchanged (E14, ADR-030 preserved).
- **Hard prohibitions**: no Git history rewrite; no default arbitrary test
  execution; no provider calls in phase 2; no delete or rename acceptance; no
  phase 2.5; no general search, fuzzy matching, scoring or similarity
  threshold; no network access and no lazy object fetch.

## 7. Failure and refusal vocabulary

### 7.0 What "operation replay" means here

GH #13's issue title says *operation replay*, and that vocabulary is kept. What
v1 replays is the set of **normalized effects** ADR-036 derived from the
canonical patch, re-expressed as newly derived `create-file` and
`insert-by-boundary` operations. The persisted `apply-recipe.json` is an
**eligibility and coverage witness** whose `write-file` bodies are never
executed against the upstream tree (§6.6). Every code below is read against
that meaning.

Every code is closed and machine-readable. Each row states exactly one class.
**Every phase-2 code in §7.1, §7.2 and §7.3 is nonterminal**: it records
evidence and falls through to phase 3/4. §7.4's codes are **terminal command
refusals** — they belong to candidate actions, acceptance and recovery, they
return an exit code, and they do not fall through, because at that point there
is no pipeline left to fall through to.

### 7.1 Phase-2 eligibility, lifecycle and platform refusals (§6.5, §6.6a, §6.13)

Listed in evaluation order: the preflight preconditions first, then E1-E15.

| Code | Class | Behavior |
|---|---|---|
| `candidate-lock-held` | phase-2 refusal (preflight) | the per-feature lock is held elsewhere; no inspection, no mutation, derivation disabled for this invocation |
| `lock-unsupported` | phase-2 refusal (preflight) | the build target has no advisory-lock primitive (§6.13) |
| `lock-filesystem-unsupported` | phase-2 refusal (preflight) | the lock path is on a known network/remote or unrecognized filesystem class (§6.13) |
| `recovery-required` | **terminal**, see §7.4 | an acceptance journal is present; the whole command refuses |
| `candidate-feature-state-unsupported` | phase-2 refusal (preflight) | `status.State` is outside {`applied`, `blocked`} (§6.6a) |
| `coverage-unusable` | phase-2 refusal, **E1** | absent, unreadable, malformed or internally contradictory coverage — including every ADR-036 D3 contradiction refusal |
| `coverage-owner-mismatch` | phase-2 refusal, **E2** | `coverage.feature` ≠ requested slug |
| `coverage-incomplete` | phase-2 refusal, **E3** | `coverage_status` ≠ `complete` |
| `binding-presence-drift` | phase-2 refusal, **E4** | either presence flag contradicted by recomputed readable existence, either direction |
| `binding-hash-drift` | phase-2 refusal, **E5** | recomputed patch or raw on-disk recipe hash differs |
| `recipe-undecodable` | phase-2 refusal, **E6** | the recipe's bytes are present and hash-correct but do not strict-decode |
| `recipe-owner-mismatch` | phase-2 refusal, **E7** (conditional on E6) | the decoded recipe's `feature` ≠ `coverage.feature`; a recomputation defense |
| `reference-not-reconstructable` | phase-2 refusal, **E8** | non-`commit` kind, malformed hex, absent local object, or `preimage_set_sha256` mismatch |
| `effect-binding-drift` | phase-2 refusal, **E9** | any `effect_sha256` / `patch_fragment_sha256` / ordinal mismatch; a recomputation defense |
| `recipe-unsupported` | phase-2 refusal, **E10** | an operation is unassigned, is not a supported witness, or carries `created_by`; a recomputation defense |
| `recipe-stale-marker-present` | phase-2 refusal, **E11** | `recipe-stale.json` on disk |
| `supersession-not-authority` | phase-2 refusal, **E12** | superseded feature; the ADR-029 D7 downgrade grants nothing |
| `parent-blocked` | phase-2 refusal, **E13** | hard-blocked parent in the DAG |
| `derivation-mode-unsupported` | phase-2 refusal, **E14** | multi-slug or `--cumulative-legacy` |
| `shadow-pending` | phase-2 refusal, **E15** | a resolver shadow is staged for this slug |

**Removed in rev-1 because ADR-036's strict decoder makes them unreachable**:
`coverage-reason-outstanding` (a `complete` record cannot carry a reason),
`effect-axis-unknown` (a `complete` record cannot carry an `unknown` axis), and
the zero-effect arm of `effect-binding-drift` (predicate 1 requires at least
one effect). All three now fail at E1 as `coverage-unusable`.

**Reordered in rev-2 so that every recipe-side code is reachable**: the
recipe's presence (E4), raw-byte hash (E5), decodability (E6) and decoded owner
(E7) are four ordered questions instead of rev-1's single combined E4, which
consumed all four and reported `recipe-owner-mismatch` for every one of them
(§6.5).

### 7.2 Domain, derivation and alignment refusals (§6.6, §6.7)

| Code | Class | Behavior |
|---|---|---|
| `effect-change-kind-unsupported` | phase-2 refusal | delete, rename or copy |
| `effect-content-kind-unsupported` | phase-2 refusal | binary or `none` |
| `effect-object-kind-unsupported` | phase-2 refusal | executable, symlink or gitlink |
| `effect-mode-change-unsupported` | phase-2 refusal | any mode transition |
| `effect-replacement-unsupported` | phase-2 refusal | a `modify` fragment carrying a `-` line |
| `fragment-unparseable` | phase-2 refusal | the ADR-036 D1 strict grammar refuses the fragment; a recomputation defense |
| `fragment-boundary-underivable` | phase-2 refusal | hunk headers and context do not walk to a record-aligned base boundary |
| `record-terminator-unsupported` | phase-2 refusal | a no-newline insertion record away from the base file's end, or a base final record the derivation would have to rewrite |
| `derivation-postimage-mismatch` | phase-2 refusal | replaying runs at their base boundaries does not reproduce `postimage_sha256` |
| `alignment-limit` | phase-2 refusal | L1, L2 or L3 exceeded; a refusal, never a truncation |
| `alignment-left-unanchored` | phase-2 refusal | no surviving matched base record before `b`, and `b ≠ 0` |
| `alignment-right-unanchored` | phase-2 refusal | no surviving matched base record at or after `b`, and `b ≠ len(B)` |
| `alignment-ambiguous` | phase-2 refusal | the maximum-cardinality alignments do not agree on `(left, right)` |
| `alignment-range-overlap` | phase-2 refusal | two distinct boundaries' mapped gaps overlap |
| `boundary-conflict` | phase-2 **BLOCKED** classification | the gap is neither empty nor byte-exactly the combined insertion records |
| `creation-conflict` | phase-2 **BLOCKED** classification | a creation's upstream path exists with bytes differing from the effect postimage |

**Removed in rev-1 with the byte-window algorithm**: `anchor-not-found`,
`anchor-ambiguous`, `anchor-disagreement`, `anchor-upstream-drift`,
`anchor-one-sided`, `anchor-eol-ambiguous`, `postimage-elsewhere` and
`creation-path-occupied`. `anchor-range-overlap` is renamed
`alignment-range-overlap`. There is no EOL refusal at all: mixed line endings
are exact bytes (§6.7), and `candidate-path-occupied` survives only as the
**acceptance-time** untracked-collision code in §7.4.

### 7.3 Candidate integrity refusals (§6.8, §6.10)

| Code | Class | Behavior |
|---|---|---|
| `effect-unexplained` | candidate refusal | an effect is neither absorbed-present nor candidate-applicable |
| `candidate-postcondition-failed` | candidate refusal | a per-effect postcondition does not hold |
| `candidate-path-set-mismatch` | candidate refusal | the differing path set ≠ the derived path set |
| `candidate-mode-drift` | candidate refusal | a touched path is not regular `100644` on both sides |
| `candidate-unmodeled-change` | candidate refusal | a path outside the derived set differs |
| `candidate-conflict-marker` | candidate refusal | a conflict marker in a touched file |
| `candidate-not-idempotent` | candidate refusal | the second pass returns anything other than all-present with zero counts and unchanged tree and diff bytes — including a second-pass conflict, alignment refusal, `alignment-limit` or terminator refusal, whose code is carried as diagnostic detail (§6.8) |
| `candidate-id-collision` | refusal | recomputed ID matches an on-disk candidate whose immutable identity subset differs |
| `candidate-integrity-drift` | refusal | identity subset equal but the stored `result` integrity fields differ |
| `candidate-already-accepted` | phase-2 refusal | the recomputed ID names a directory whose `state.json` is `accepted`; immutable audit, never reused or rejected (§6.10) |
| `candidate-metadata-malformed` | refusal | `candidate.json` or `state.json` does not strict-decode |
| `candidate-path-not-normalized` | refusal | a persisted path is absolute or not `filepath.ToSlash`-normalized |
| `candidate-rejected` | phase-2 refusal | the derived ID is already in `rejected.jsonl`; checked before anything is created |

### 7.4 Coexistence, acceptance and recovery refusals (§6.11-§6.15)

**None of these falls through.** Each is a terminal command refusal that
returns the stated exit code.

| Code | Class | Exit |
|---|---|---|
| `candidate-pending` | state-machine refusal | `3` |
| `shadow-pending` | state-machine refusal | `3` |
| `candidate-absent` | validation refusal | `2` |
| `candidate-not-pending` | state-machine refusal | `3` |
| `candidate-already-accepted` | state-machine refusal | `3` |
| `candidate-feature-state-changed` | state-machine refusal | `3` |
| `head-diverged` | validation refusal | `2` |
| `candidate-cas-failed` | validation refusal | `2` |
| `candidate-path-occupied` | validation refusal | `2` |
| `candidate-binding-changed` | validation refusal | `2` |
| `candidate-test-failed` | validation refusal | `2` |
| `candidate-lock-held` | state-machine refusal | `3` |
| `lock-unsupported` | state-machine refusal | `3` |
| `lock-filesystem-unsupported` | state-machine refusal | `3` |
| `recovery-required` | state-machine refusal | `3` |
| `coverage-publication-failed` | transaction failure, **rolled back** | `3` |
| `acceptance-rolled-back` | transaction failure, **rolled back** | `3` |
| `acceptance-rollback-failed` | rollback itself failed; journal retained | `1` |

**`candidate-feature-state-changed` is a state refusal, not a validation
refusal.** The candidate is well-formed and its bindings still recompute; what
changed is the feature's position in its own lifecycle, which is exactly the
state-machine class ADR-031 D4 gives exit `3`. It is checked at §6.14 step 3,
so nothing is journaled, snapshotted or written when it fires.

**Several codes appear in both §7.1 and here, with different classes.**
`candidate-lock-held`, `lock-unsupported` and `lock-filesystem-unsupported` are
nonterminal during generation — a reconcile still runs its phases and earns its
own outcome — and terminal on a candidate action, where there is nothing else
to do. `shadow-pending` and `candidate-already-accepted` behave the same way.
The class is a property of the surface, not of the code.

**Exit `1` is reserved for the one case that is genuinely an internal error.**
rev-0 gave `acceptance-rolled-back` and `coverage-publication-failed` exit `1`;
they are state refusals whose rollback completed, so under ADR-031 D4's shipped
envelope (`internal/cli/reject.go:36-47`) they exit `3` and name the failing
transaction step. Only a rollback that could not complete escalates to `1`,
and it leaves the journal so the next command is `recovery-required`.

**Neither publication failure nor rollback has a warning-class rendering,
deliberately.** A coverage-publication failure is the command's own error
(§6.16), mirroring ADR-036 D10's rule for every governed producer event, and it
never appears beside a success line.

**`head-diverged` replaces rev-0's `head-moved`.** The check it names is
ancestry, not equality (§6.14), and the code says so.

**The step-10 CAS recheck introduces no new code.** It refuses with the same
`candidate-cas-failed` (or `candidate-path-occupied` for an occupied created
path) and the same exit `2` as the step-6 pass, because it is the same
comparison over the same path set; only the moment differs. A distinct code
would imply a distinct failure mode, and there is none.

## 8. Implementation slices

**Sequential, not parallel.** Every slice converges on
`internal/workflow/reconcile.go` and `internal/cli/cobra.go`. `AGENTS.md`'s
Parallel-Implementer Discipline makes same-file overlap a hard trigger for
sequential execution, so these slices run one at a time with explicit-path
staging.

**S0 must complete before any production edit.**

### S0 - Frozen baseline

- Freeze current reconcile goldens for all four phases, including the exact
  `Operation-level: %d present, %d applicable, %d conflicts` note
  (`internal/workflow/reconcile.go:460-461`) and the
  `All recipe operations already present in upstream` note (`:454`).
- Freeze `reconcile-session.json` and `reconcile-evidence.jsonl` byte-identity
  for a representative feature in each of the phase-1, phase-1.5, phase-2
  all-present, phase-3, phase-3.5 and phase-4 outcomes, **with
  `operation_candidate_enabled` off**.
- Freeze the confirmation gate's three existing behaviors verbatim: the
  phase-1 confirmation arm
  (`TestUpstreamedConfirmationGateKeepsConfirmedReverseApply`,
  `internal/workflow/reconcile_evidence_integration_test.go:413`), the phase-3
  rejection arm (`TestReconcilePhase3_ProviderAssistedUpstreamed`,
  `internal/workflow/reconcile_test.go:304`) and the phase-2 rejection arm
  (`TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch`, `:459`).
  The third is the **one** expectation §6.4 intentionally changes with the gate
  on; S0 records both the gate-off (byte-identical) and gate-on (new)
  expectations explicitly, and no default byte identity is claimed for it.
- Freeze the `ReconcileSummary` serialization for a feature with **no**
  candidate, so the `omitempty` pointer cannot change it.
- Freeze `--accept` / `--reject` / `--shadow-diff` output and the
  `validateReconcileFlags` error strings (`internal/cli/cobra.go:3212,3215,3218`)
  **byte-for-byte**, as the guard for §6.12's no-re-routing rule.
- Freeze a multi-slug fixture and a `--cumulative-legacy` fixture whose behavior
  must be byte-identical through the whole wave.
- Promote the adjacent-CLI-args case study
  (`docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/`)
  into a candidate fixture in **both** its `delete-all` and
  `between-delete-first` variants. §6.7.1 pins them as candidate **successes**,
  not refusals: each must derive one applicable insertion, preserve the feature
  arguments, preserve upstream's deletions, and — for the second variant —
  retain upstream's surviving `--old-b`.
- Add failing contract tests before any refactor.

### S1 - Typed roots, locks and the missing Git primitives

- Add `ReferenceRoot` / `CandidateRoot` and the two sanctioned constructors
  `OpenReferenceRoot` / `CreateCandidateRoot` (§6.9); no other candidate
  function accepts a bare `string` root.
- Wire `gitutil.NoLazyFetchEnv` (`internal/gitutil/trailers.go:41`) into every
  candidate and reference-tree Git subprocess, with the environment guard
  modelled on `internal/gitutil/apply_classifier_test.go:319-320`.
- Add the tree-hash primitive `internal/gitutil/` does not have (no `WriteTree`,
  `mktree` or `hash-object` wrapper exists today).
- Add the §6.13 advisory-lock abstraction: `flock` on linux/darwin,
  `LockFileEx` via stdlib `syscall.NewLazyDLL` on Windows, build-tagged
  unsupported stub on **every** other target including the BSDs. No PID
  metadata, no stale-breaking, no `rescap` local contract, and no new module
  dependency.
- Add the lock constructor's own filesystem-class refusal
  `lock-filesystem-unsupported` — `statfs`/`fstatfs` on linux, `statfs` plus
  `MNT_LOCAL` on darwin, `GetDriveTypeW` plus UNC detection on Windows — as a
  known-local allowlist that refuses unknown and remote classes.
- Add candidate-namespace worktree create/prune that does **not** call
  `CreateShadow`, `ResolveShadow`, `PruneShadow` or `PruneAllShadows`, and that
  is only ever reachable while the per-feature lock is held.
- Add the source-inventory guards for live-root calls, lazy fetch and the
  module dependency set (§9.11).

### S2 - Eligibility

- Consume the v0.17.0 ADR-036 D3 strict decoder directly; do **not** write a
  private copy and do **not** write a weaker one.
- The E1-E15 gate in the §6.5 order, each with its own reason code, with E7
  evaluated **only** when E6 succeeded and with strict first-match reporting.
- The §6.6a feature-state precondition, evaluated in the preflight before E1.
- Offline dual-tree reconstruction with an explicit no-fetch assertion.
- The `operation_candidate_enabled` config gate, default `false`.

### S3 - Derivation, alignment and classification

- Insertion-run extraction over the ADR-036 D1 strict grammar; line-record
  splitting with byte-exact record equality and no normalization.
- Base-boundary computation and the mandatory postimage reconciliation.
- The A1-A8 alignment rules, their **fixed first-match evaluation order** and
  the L1-L3 limits (§6.7), including the exact `F`/`G` unanimity decision
  procedure and the traceback-free sentinel predicates.
- The `1048576`-cell L3 bound, checked before any matrix is allocated, with the
  8 MiB two-matrix budget asserted.
- Three-state classification over one immutable snapshot; canonical
  same-boundary combination; gap-disjointness; descending application order.
- The §6.7.1 worked fixture as an executable test in both variants.

### S4 - Candidate execution and validation

- The closed two-kind candidate writer/action registry.
- All-or-nothing execution in the candidate worktree.
- Postcondition, path-set, mode, unmodeled-change and conflict-marker
  validation.
- The second-pass idempotency proof re-running the production classifier over
  **every effect**.
- Absorption accounting.
- The §6.8 generation-scope guard, with the four expected write sets excluded
  explicitly.

### S5 - Artifacts, identity and lifecycle

- `rc_<12hex>` identity over the immutable pre-execution tuple **including
  `source_feature_state`**; strict `candidate.json` and `state.json`;
  `candidate.patch`; `filepath.ToSlash` normalization of every persisted path.
- The `ReconcileSummary` `operation_candidate` pointer, carrying
  `source_feature_state`, with `omitempty` byte parity.
- The §6.11 preflight ordering under the per-feature lock (lock, recovery,
  staleness, feature state), safe worktree pruning, the immutable-audit
  treatment of `accepted` and `rejected` directories, `rejected.jsonl` with the
  pre-creation check, and the coexistence refusals.

### S6 - Phase wiring, lifecycle preservation and evidence

- The §6.2 outcome table: phase-2 returns only the all-present arm; the
  candidate proof is held in memory; phase 3 always runs; phase 4 disposes of
  the proof per the arm table; finalization reacquires the per-feature lock and
  revalidates.
- The unexported `phase2AuthorityProof` type, its single mint site in the GH #13
  evaluator, and the fourth parameter on **both** `saveReconcileArtifacts`
  (`internal/workflow/reconcile.go:724`) and
  `applyUpstreamedConfirmationGate` (`:1032`, called at `:771`), with `nil` at
  all **ten** shipped call sites
  (`:383,432,455,480,524,536,564,570,583,596`) — `:570` passing `phase35`,
  which **names the same `*ReconcileResult` object** as `result`, because
  `tryPhase35` (`:569`) takes that result as its ninth parameter (`:1363`) and
  returns the same pointer from all ten of its returns
  (`:1373,1381,1389,1436,1475,1491,1497,1502,1508,1513`), and passing a `nil`
  proof and staging no attempt exactly like the other nine — and the source
  guard against any other constructor, marshaller or carrier.
- The unexported `verdictAttempts []store.ReconcileEvidence` staging field on
  `ReconcileResult` (`:19-85`), its single assignment and mutation sites in the
  evaluator, its single read site in `persistReconcileEvidence`, the three
  legal configurations and the proof pairing over configuration 2, and the
  `:766` substitution so **exactly one** verdict-bearing entry per phase is
  appended per invocation — one attempt on the all-present arm, two ordered
  attempts on the candidate-ready arm, and the direct-append route on the
  fallthrough arms.
- The new phase-2 verdict-bearing `recipe-operation-match` entry with §6.2's
  exact field values, minted only after E1-E15 and the all-present
  classification, and the candidate-ready pair's exact phase, kind, verdict,
  confidence and reason-code values, with the shipped legacy builder
  (`:843-845`) untouched.
- The CG1-CG7 confirmation authority class inside
  `applyUpstreamedConfirmationGate`, consuming that typed value, cross-checking
  it against the exact verdict-bearing entry **it is grading** — never one it
  writes — added beside the two existing confirming inputs and weakening
  neither.
- The dedicated candidate persistence path that **calls**
  `saveReconcileArtifacts(s, slug, result, nil)` with the two staged attempts
  rather than reimplementing the `:766` → `:771` sequence, bypasses
  `updateFeatureState` for the status write only, and preserves
  `status.State` / `status.Notes` (§6.11).
- The shared status merge that carries the loaded
  `Reconcile.OperationCandidate` forward before the `ReconcileSummary`
  replacement (`:1191-1209`), used by every writer of `status.Reconcile`, plus
  the source guard over every `store.ReconcileSummary` construction.
- The ordered evidence list with the verdict-bearing / supporting split,
  phase-local verdicts on verdict-bearing entries only, the five shipped
  supporting entries retained byte-for-byte **with their firing conditions
  unchanged**, and the `tpatch reconcile review` rendering that labels the two
  classes differently.
- Retire the legacy evaluator as authority; keep its diagnostic note bytes.

### S7 - CLI

- `--candidate-diff`, `--accept-candidate`, `--reject-candidate`,
  `--clear-candidate-rejections`, `--resume-candidate-recovery`.
- The source-derived second-stage exclusivity: enumerate `cmd.Flags()`,
  subtract the `--path` / `--format` presentation allowlist **and Cobra's
  auto-registered `help`**, refuse every other registered reconcile flag that
  is `Changed`. For the current flag set that is 10 new×new plus 75
  new×existing pairs, **85** in total, asserted against the enumerated set
  rather than a hardcoded list, with the shipped existing-only validation and
  message bytes untouched (`internal/cli/cobra.go:3200`, `:3211-3218`,
  `:3212,3215,3218`).
- Exit-code mapping per §6.12 and §7.4.
- The gate-on expectation for
  `TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch`, added beside
  the gate-off golden rather than editing it.

### S8 - Acceptance and recovery

- `AcceptOperationCandidate` with both locks, the feature-state re-check, the
  ancestry check and the authoritative per-path CAS.
- The fourteen-step transaction (§6.14) with step 10's CAS recheck immediately
  before the first live write, staged artifacts including the ADR-024
  generation entry at `base_commit = upstream_commit`, step 8's named snapshot
  set — patch, recipe, provenance, coverage, `patch-generations.json` and
  `status.json` — with absent markers and the pre-transaction `status.Apply` /
  `status.State`, fsynced journal, rollback, and the tight `3` / `1` exit
  split.
- Step 12's base metadata: `Apply.BaseCommit`, `Apply.CompletedAt`,
  `Apply.HasPatch`, `Apply.HasRecipe` and `store.StateApplied`, with the six
  shipped `status.Apply.BaseCommit` readers pinned against the new value.
- `--resume-candidate-recovery`'s commit-marker-driven resume or rollback.
- Coverage publication through ADR-036's shared API with the
  `operation-candidate-accept` variant (§6.16).
- Non-zero surfacing of every publication, artifact and state failure.

### S9 - Docs, parity and soak

- The `SPEC.md` §7 phase-2 amendment across **all three** rows in one pass:
  `→ All already-present → UPSTREAMED` becomes CG-qualified (`SPEC.md:446`),
  `→ Any conflict → BLOCKED` becomes a phase classification (`:447`), and
  `→ Mix → STILL-NEEDED` becomes nonterminal at phase 2, surfacing a candidate
  only at a phase-4 conflict (`:448`).
- ADR-036 D15 P3 variant note; ADR-025 verdict-bearing / supporting note;
  confirmation-gate authority-class note in
  `docs/prds/PRD-upstreamed-confirmation-gate.md` and
  `docs/prds/PRD-confirm-upstreamed-human-review-path.md`.
- The parity-block extraction guard over the nine marked regions (§6.0).
- Skill assets and the `assets_test.go` parity guard.
- Downstream soak on the frozen fixtures and both adjacent-CLI-args variants.

**Public assets and `SPEC.md` updates are implementation work (S9), not planning
edits.** No asset, SPEC or skill file is touched by this planning revision.

## 9. Acceptance matrix

Kinds: `I` integration/runtime, `C` failure/concurrency, `G` semantic guard,
`U` unit/schema, `S` security/privacy.

**Every `G` row runs the same validator the production path runs and feeds it a
semantically wrong input, named in the Observable column.** A fixture that only
asserts a token is present, a count matches or a string appears does not satisfy
a semantic guard and does not belong in this matrix.

IDs are contiguous `ROC-001` … `ROC-293`.

### 9.1 Phase-2 evaluation boundary, outcomes and terminality

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-001 | I | Upstream ref resolved once | A reconcile of one slug resolves `--upstream-ref` to a single 40-hex commit and every subsequent eligibility, alignment and classification read is issued against that commit; a second resolution during the same feature fails the row |
| ROC-002 | G | No live-worktree read in phase 2 | Wrong-input fixture `phase2-live-root-read` (a candidate code path handed `s.Root` where a `ReferenceRoot`/`CandidateRoot` is required) fails the same root-typing validator that the two-root configuration passes |
| ROC-003 | I | Dirty tree and detached HEAD change nothing | The same feature and same upstream commit yield a byte-identical phase-2 classification with a clean tree, with unrelated uncommitted edits under `--allow-dirty` (`internal/cli/cobra.go:2523`), and with HEAD detached at an unrelated commit |
| ROC-004 | I | Phase 2 returns only the all-present arm | A candidate-ready feature produces **no** return from phase 2: execution reaches phase 3, then phase 4, and the proof is carried in memory |
| ROC-005 | G | Phase 2 does not persist a candidate | Wrong-input fixture `phase2-persists-candidate` (a phase-2 arm writing under `.tpatch/reconcile-candidates/` before phase 4 disposes of the proof) fails the same persistence-boundary validator |
| ROC-006 | I | Clean phase 4 keeps `reapplied` | A feature with a held candidate proof whose patch applies strictly (`internal/workflow/reconcile.go:519-526`) or via a clean 3-way merge (`:527-538`) returns `store.ReconcileReapplied` and persists no candidate; the proof is discarded on both arms |
| ROC-007 | G | A candidate never suppresses a clean reapply | Wrong-input fixture `candidate-preempts-phase4-clean` (a pipeline returning candidate-ready while phase 4 would have reapplied) fails the same proof-disposition validator |
| ROC-008 | I | Live conflict markers keep shipped blocked | With unresolved markers in the live tree, `promoteIfMarkers` (`:505-517`) still returns `blocked` / `phase-4-live-conflict-markers` and the held proof is discarded |
| ROC-009 | I | Phase-4 blocked arms finalize the candidate | A held proof plus `ForwardApply3WayConflicts` without `--resolve` (`:574-585`), and a held proof reaching the exhausted `phase-4-blocked` arm (`:588-596`), each return `store.ReconcileStillNeeded` with a candidate-ready action and a persisted candidate in place of the shipped `blocked` |
| ROC-010 | C | No proof means shipped blocked | With no held proof, both phase-4 blocked arms produce byte-identical output to the S0 goldens |
| ROC-011 | I | `--resolve` suppresses candidate persistence | A candidate-ready feature reconciled with `--resolve` (`internal/workflow/reconcile.go:544`) persists **no** candidate, reaches `tryPhase35` (`:569`) or the blocked-by-parent short-circuit (`:552-567`), still records phase-2 evidence naming the suppressed proof, and leaves the shipped `--resolve --apply` path byte-identical to the S0 golden |
| ROC-012 | G | `--resolve` suppression is not silence | Wrong-input fixture `resolve-drops-phase2-evidence` (a `--resolve` run that skips the phase-2 evidence attempt entirely) fails the same evidence-obligation validator |
| ROC-013 | I | Provider `upstreamed` at phase 3 is not overtaken | A held proof plus a phase-3 provider `upstreamed` (`:477-483`) returns the phase-3 answer; no candidate is persisted |
| ROC-014 | U | Phase order unchanged | The phase sequence observed for a feature that exercises every phase matches the S0 golden ordering: viability, 1, 1.5, 2, 3, 4 with 3.5 nested (`internal/workflow/reconcile.go:358,377,388,445,466,490,569`) |
| ROC-015 | I | Phase 1 and phase 1.5 still short-circuit | A reverse-applying patch returns at phase 1, and with the patch-id detector enabled a phase-1.5 match returns there; phase 2 never runs in either case |
| ROC-016 | C | Conflicting effect is nonterminal | A conflicting effect records a phase-2 BLOCKED classification and **continues**; the reconcile then reaches phase 3 and phase 4 and returns whatever they earn |
| ROC-017 | I | Nonterminal BLOCKED preserves a clean phase-4 answer | A feature with a phase-2 conflict whose patch forward-applies cleanly returns `store.ReconcileReapplied`, not `blocked` |
| ROC-018 | G | Phase-2 BLOCKED is not an overall return | Wrong-input fixture `phase2-blocked-terminates` (a pipeline returning `store.ReconcileBlocked` from the phase-2 conflict arm) fails the same phase-terminality validator that the fallthrough configuration passes |
| ROC-019 | C | No provider call in phase 2 | With a provider configured, a phase-2 run that proves a candidate issues **zero** provider requests; the first provider request in any trace belongs to phase 3 or later |
| ROC-020 | G | Phase-2 offline determinism | Wrong-input fixture `phase2-provider-assisted-classification` (a classification arm consulting the provider) fails the same offline-determinism validator |
| ROC-021 | C | No network during phase 2 | With every network syscall failed at the seam, phase-2 derivation completes normally against local objects |
| ROC-022 | U | Config gate default off | With `operation_candidate_enabled` unset, every reconcile outcome, note, artifact and evidence line is byte-identical to the S0 goldens |
| ROC-023 | G | The gate is not partially applied | Wrong-input fixture `gate-off-still-persists-candidate` (a gate-off run creating a candidate namespace entry) fails the same feature-gate validator |

### 9.2 The confirmation gate and the new authority class

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-024 | C | Shipped gate demotion reproduced | With the gate off, `buildOperationUpstreamedCandidateFixture` (`internal/workflow/reconcile_evidence_integration_test.go:235`) still yields `blocked` / `rejected-upstreamed` with reason `missing-upstream-commit-ref`, exactly as `:459` asserts today |
| ROC-025 | I | CG1-CG7 satisfied confirms | An all-present feature whose seven conditions all hold returns `store.ReconcileUpstreamed` with `review_verdict: confirmed-upstreamed`, `StateUpstreamMerged`, and a revision entry linking the gate evidence attempt |
| ROC-026 | C | CG1 rejects replayed and legacy entries | A `recipe-operation-match` entry read back from a prior invocation's `reconcile-evidence.jsonl`, and one produced by `evaluateRecipeOperations` (`internal/workflow/reconcile.go:611`), each fail to confirm whatever their counters say |
| ROC-027 | C | CG2 rejects an ungated record | An all-present derivation whose coverage failed any of E1-E15 does not confirm |
| ROC-028 | C | CG3 rejects a read presence claim | A derivation that took `present` from the coverage record instead of recomputing it at the upstream commit does not confirm |
| ROC-029 | C | CG4 rejects a nonzero count | One applicable or one conflicting classification anywhere in the feature does not confirm |
| ROC-030 | C | CG5 and CG6 reject weak origins | `match_origin` of `fork`, `sibling-feature`, `mixed` or `unknown` (`internal/store/reconcile_evidence.go:63-68`), and `pre_reconcile_presence` of `not-checked`, `absent` or `unknown` (`:73-77`), each independently fail to confirm |
| ROC-031 | C | CG7 rejects a missing or wrong ref | An empty `upstream_commit_refs`, or one naming a commit other than the resolved `upstreamCommit`, does not confirm |
| ROC-032 | G | Confidence alone does not confirm | Wrong-input fixture `high-confidence-confirms` (a gate arm confirming a `recipe-operation-match` entry on `confidence: high` without CG1-CG7) fails the same confirmation-authority validator |
| ROC-033 | G | The gate is widened, never weakened | Wrong-input fixture `gate-accepts-any-operation-match` (a gate arm confirming every `recipe-operation-match`) fails the same validator, and the two shipped confirming inputs still confirm |
| ROC-034 | I | Existing confirming inputs unchanged | `patch-id-match` with a `matched_upstream_sha` (`:1047-1054`) and high-confidence `reverse-apply` (`:1055-1060`) still confirm, byte-identically to the S0 goldens |
| ROC-035 | I | Rejection arm unchanged | An unconfirmed `upstreamed` still writes the gate's `manual-review` evidence, sets `rejected-upstreamed`, persists `blocked` state, and appends the revision entry (`:1096-1112`) |
| ROC-036 | S | Gate evidence carries no metadata secret | The confirmation evidence for a candidate-confirmed feature contains no configured metadata secret, matching the assertion at `internal/workflow/reconcile_evidence_integration_test.go:483-485` |
| ROC-037 | U | The proof is a typed in-process parameter with one mint site | `applyUpstreamedConfirmationGate` (`internal/workflow/reconcile.go:1032`) takes a fourth parameter of the unexported proof type, reached only through `saveReconcileArtifacts`'s own fourth parameter (`:724`, ROC-282), and the type has no exported constructor, setter, exported field or JSON tag; a signature without the parameter fails the row at compile time, and wrong-input fixture `proof-constructed-outside-evaluator` (a second construction site anywhere in `internal/`) fails the same source-inventory guard that the single evaluator mint site passes |
| ROC-038 | C | A nil proof confirms nothing, and a disagreeing proof does not either | Every legacy caller passes `nil`, and a `recipe-operation-match` result reaching the gate with a `nil` proof takes the shipped unconfirmed branch with `rejected-upstreamed` and `blocked`, byte-identically to the S0 golden; a non-nil proof whose `upstream_commit`, binding hash, effect count, effect digest or classification count disagrees with the result — or whose values disagree with the `match_origin`, `pre_reconcile_presence`, `upstream_commit`, `upstream_commit_refs` or `confidence` of the exact verdict-bearing entry **it is grading**, the single member of `result.verdictAttempts` already appended at `:766` — also fails CG1-CG7 |
| ROC-039 | S | The proof is never serialized | After a CG1-CG7 confirmation, no field of the proof type appears in `candidate.json`, `state.json`, `status.json`, `reconcile-evidence.jsonl` or any log line; a marshaller reachable from the type fails the row |
| ROC-040 | G | The proof cannot be reconstructed from stored bytes | Wrong-input fixtures `proof-rebuilt-from-evidence` (a gate arm assembling a proof from a persisted `recipe-operation-match` line) and `proof-rebuilt-from-confidence` (a gate arm synthesising one because `confidence: high`) each fail the same confirmation-authority validator that the evaluator-minted value passes |

### 9.3 Independent eligibility recomputation

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-041 | C | E1 coverage absent | No `recipe-coverage.json` yields `coverage-unusable` and a fallthrough; verify's `warn`/exit `0` grading is irrelevant to the decision |
| ROC-042 | C | E1 coverage present but unreadable | A coverage file whose read fails at the file-access seam yields `coverage-unusable` with the **actual** read cause named, never "not found" |
| ROC-043 | U | E1 strict decode refuses unknown fields, nulls and duplicates | Coverage carrying one unknown member, a null array, a null scalar or a duplicate JSON member each refuse independently as `coverage-unusable`; the same record without them decodes |
| ROC-044 | U | E1 absorbs the contradiction refusals | `complete` beside a non-empty `reasons`, a non-`represented` disposition, a non-empty `reason_codes`, an empty `effects`, `content_kind: unknown`, `object_kind: unknown`, `patch_present: false`, `recipe_present: false`, `recipe_decodable: false`, an unobserved required side and a non-`commit` reference each fail at **E1** as `coverage-unusable`, not at a later gate; and neither `coverage-reason-outstanding` nor `effect-axis-unknown` appears anywhere in the shipped reason enum |
| ROC-045 | G | The decoder is reused, not re-implemented | Wrong-input fixture `private-lenient-coverage-decoder` (a consumer-local decoder that accepts one contradictory `complete` record) fails the same decoder-ownership validator that the ADR-036 decoder passes |
| ROC-046 | C | E2 envelope owner mismatch | `coverage.feature` naming another slug is `coverage-owner-mismatch`, even when every hash recomputes |
| ROC-047 | C | E3 incomplete coverage | `coverage_status: incomplete` refuses with `coverage-incomplete` |
| ROC-048 | G | E3 completeness is not eligibility | Wrong-input fixture `complete-status-authorizes-replay` (an eligibility path short-circuiting on `coverage_status: complete` and skipping E4-E15) fails the same eligibility validator |
| ROC-049 | C | E7 recipe owner mismatch | An on-disk recipe that decodes but whose `feature` differs from `coverage.feature` is `recipe-owner-mismatch`, recomputed by decoding the file rather than reading predicate 3; a record whose recipe does **not** decode reports `recipe-undecodable` at E6 instead, never this code |
| ROC-050 | C | E4 presence drift in both directions | `patch_present: true` beside a missing patch, `recipe_present: false` beside a readable recipe, a recipe that becomes unreadable and one that becomes readable are all `binding-presence-drift` |
| ROC-051 | C | E5 patch hash drift | One byte changed in `post-apply.patch` is `binding-hash-drift` |
| ROC-052 | C | E5 recipe raw-byte hash | The recipe hash is recomputed over **raw on-disk bytes**, so a semantically equivalent reserialization with different bytes drifts |
| ROC-053 | C | E6 recipe undecodable | A present, hash-correct recipe that does not strict-decode is `recipe-undecodable`, regardless of `recipe_decodable: true` in the record |
| ROC-054 | G | E6 decodability is recomputed, not read | Wrong-input fixture `trusts-recipe-decodable-flag` (an eligibility path reading the stored flag instead of attempting the decode) fails the same recomputation validator |
| ROC-055 | C | E8 reference refusals | A non-`commit` `reference.kind`, a well-formed 40-hex commit absent from the local object database (with **no** fetch attempted), and a reconstructable commit whose recomputed `preimage_set_sha256` differs each refuse with `reference-not-reconstructable` |
| ROC-056 | C | E9 fragment hash drift | A hand-forged record whose `patch_fragment_sha256` does not recompute over the bound patch refuses with `effect-binding-drift`, even though the decoder accepted it |
| ROC-057 | C | E9 ordinal gap | Ordinals `1, 2, 4` refuse; `1, 2, 3` in strict-grammar record order pass |
| ROC-058 | C | E10 unsupported recipe shapes | An operation carrying `created_by` (`internal/workflow/implement.go:49`), one assigned to no effect, and one that is not a supported write-file witness each refuse with `recipe-unsupported` |
| ROC-059 | C | E11 stale marker present | `recipe-stale.json` on disk refuses with `recipe-stale-marker-present`, despite verify's `warn`/exit `0` |
| ROC-060 | G | Warning class is never eligibility | Wrong-input fixture `verify-warn-implies-eligible` (an eligibility path gating on a verify exit status instead of recomputing) fails the same eligibility validator on every warning-class state of §6.3 |
| ROC-061 | U | Gate order is the stated order, and is strictly first-match | A record failing two gates reports the **lowest-numbered** gate's code: a fixture failing E3 and E11 reports `coverage-incomplete`, one failing E5 and E6 reports `binding-hash-drift`, and one failing E4 and E7 reports `binding-presence-drift`; an implementation that reorders any gate fails the row |
| ROC-062 | G | Every gate is recomputed | Wrong-input fixture `eligibility-reads-stored-hashes` (a gate comparing stored values to each other rather than to recomputed ones) fails the same recomputation validator |
| ROC-063 | C | E4 and E5 are reachable on the recipe side | A record claiming `recipe_present: true` beside an absent recipe, and one claiming `recipe_present: false` beside a readable recipe, each report `binding-presence-drift` — **not** `recipe-owner-mismatch` and **not** `recipe-undecodable`; and a readable recipe whose raw bytes differ from `recipe_sha256` reports `binding-hash-drift` even when it decodes cleanly and its `feature` matches |
| ROC-064 | C | E6 precedes E7, and E7 is conditional | A recipe that is present and hash-correct but does not strict-decode reports `recipe-undecodable`; E7 is not evaluated for it at all, and no fixture can make an undecodable recipe report `recipe-owner-mismatch` |
| ROC-065 | G | No gate shadows a later gate, and every code is reachable | Wrong-input fixture `combined-recipe-decode-and-owner-gate` (rev-1's single E4 asking both questions at once) fails the same gate-reachability validator: under it the `binding-presence-drift`, `binding-hash-drift` and `recipe-undecodable` fixtures above all report `recipe-owner-mismatch`, while the ordered configuration reports three distinct codes; and each of the fifteen E-gate codes has a fixture reporting exactly it, so a code with no reaching fixture fails the row |

### 9.4 Candidate domain, the recipe-witness role and insertion-run derivation

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-066 | I | Exact creation admitted | An `add` / `text` / `regular` / `100644` effect derives one `create-file` operation when its path is absent upstream |
| ROC-067 | I | Creation already upstream is absorbed | The same effect whose upstream blob equals the postimage classifies `absorbed-present`, not a refusal |
| ROC-068 | C | Creation with different upstream bytes conflicts | The same effect whose upstream path exists with different bytes classifies conflict with `creation-conflict` |
| ROC-069 | I | Additive modification admitted | A `modify` / `text` / `regular` / `100644`→`100644` effect with zero `-` lines derives `insert-by-boundary` operations |
| ROC-070 | C | Replacement refused | A `modify` fragment containing one `-` line refuses with `effect-replacement-unsupported` |
| ROC-071 | U | Delete, rename and copy refused | `change_kind: delete`, `rename` (including a pure rename with no content change) and `copy` each refuse with `effect-change-kind-unsupported` |
| ROC-072 | U | Binary and `none` content refused | `content_kind: binary` and the mode-only `content_kind: none` each refuse with `effect-content-kind-unsupported` |
| ROC-073 | U | Executable, symlink and gitlink refused | `object_kind: executable`, `symlink` and `gitlink` each refuse with `effect-object-kind-unsupported` |
| ROC-074 | U | Mode change refused | `100644`→`100755` refuses with `effect-mode-change-unsupported` even when the content change is purely additive |
| ROC-075 | G | Refusal is per-feature, not per-effect | Wrong-input fixture `partial-candidate-on-mixed-domain` (a candidate derived from the supported subset of a feature containing one unsupported effect) fails the same domain validator |
| ROC-076 | G | The persisted write-file body is never executed | Wrong-input fixture `candidate-executes-persisted-write-file` (a candidate operation whose payload is the recipe's whole-file postimage) fails the same operation-derivation validator that newly derived `create-file`/`insert-by-boundary` operations pass; in the passing configuration no derived operation payload is byte-traceable to a recipe operation body |
| ROC-077 | U | Legacy operation kinds need no separate claim | A recipe carrying `ensure-directory`, `append-file` (`internal/workflow/implement.go:50`) or `replace-in-file` outside a proven exact-postimage case cannot reach a `complete` ADR-036 record — the latter two carry `operation-not-reclassifiable` — so the feature refuses at E3 with `coverage-incomplete`; a fixture asserting a dedicated candidate code for any of the three fails the row |
| ROC-078 | G | Unknown type must not be treated as applicable | Wrong-input fixture `unknown-op-counted-applicable` (the shipped `default: result.applicableCount++` behavior at `internal/workflow/reconcile.go:649-650`) fails the same candidate-domain validator |
| ROC-079 | U | Maximal run extraction | A fragment with `+`, context, `+`, `+`, context, `+` yields exactly three runs with one-based fragment-order ordinals |
| ROC-080 | U | Run records preserve terminators | Insertion records preserve each line's original terminator byte-for-byte; a normalizing extractor fails the row |
| ROC-081 | I | Postimage reconciliation succeeds | Replaying every run at its base boundary against the reference-tree file reproduces `postimage_sha256` exactly |
| ROC-082 | C | Postimage reconciliation fails | A patch and coverage that disagree refuse with `derivation-postimage-mismatch` |
| ROC-083 | G | Derivation uses the ADR-036 strict grammar | Wrong-input fixture `derivation-uses-lenient-parser` (run extraction through a `strings.Fields`-class reader rather than the strict grammar) fails the same parser-ownership validator |
| ROC-084 | C | Unparseable fragment and underivable boundary refuse | A fragment the strict grammar refuses yields `fragment-unparseable`; a hunk whose headers and context do not walk to a record-aligned base boundary yields `fragment-boundary-underivable` |
| ROC-085 | S | No source bodies leave derivation | Neither `candidate.json`, `state.json`, the evidence entry nor any log line emitted during derivation contains a context line, an insertion line or a file body |

### 9.5 Line-record alignment, unanimity, gaps and classification

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-086 | U | Record splitting is exact | `foo\n`, `foo\r\n` and `foo` split into three distinct, byte-unequal records, and a file not ending in `\n` yields a final no-newline record; a splitter that equates any two records or silently terminates the last fails the row |
| ROC-087 | U | Mixed EOL is ordinary input | A file containing both `\n` and `\r\n` records aligns normally and raises **no** EOL refusal; a build carrying an `anchor-eol-ambiguous`-class code fails the row |
| ROC-088 | I | Unanimous alignment maps a boundary | A boundary whose optimal alignments all agree maps to one `(left, right)` and classifies |
| ROC-089 | I | **Motivating fixture, delete-all variant** | The adjacent-CLI-args base with upstream deleting both old arguments maps `b = 3` to `left₀ = right₀ = 1` with an empty gap, classifies **applicable**, and produces the case study's stated correct slice — both feature arguments present, both deleted arguments still absent |
| ROC-090 | I | **Motivating fixture, between-delete-first variant** | The same fixture with upstream deleting only `--old-a` maps `b = 2` to an empty gap and produces the case study's other stated correct slice, retaining upstream's surviving `--old-b` |
| ROC-091 | G | The fixture is a success, not a refusal | Wrong-input fixture `adjacent-args-refuses` (a mapper that refuses the motivating case because its immediate base neighbours were deleted) fails the same alignment validator that §6.7.1's configuration passes |
| ROC-092 | I | Deleted neighbours are skipped, not fatal | A boundary whose immediately adjacent base records were deleted upstream still anchors to the nearest surviving matched neighbours |
| ROC-093 | C | Unrelated upstream addition in the gap conflicts | The same boundary with an unrelated upstream record inserted in that region classifies conflict with `boundary-conflict` |
| ROC-094 | G | No global elsewhere rule | Wrong-input fixture `postimage-elsewhere-refusal` (a mapper refusing because the insertion bytes also occur somewhere else upstream) fails the same classification validator; the correct configuration classifies on the mapped gap alone |
| ROC-095 | U | Gap classification is total | `left == right` classifies **applicable**; a gap byte-exactly equal to the combined insertion records classifies **present**; a gap differing by one byte, one record or one ordering classifies **conflict** |
| ROC-096 | C | Whitespace-different bytes are not present | A gap differing only in leading or trailing whitespace classifies **conflict**, never present |
| ROC-097 | G | Trimmed equality is not presence | Wrong-input fixture `trimspace-equality-presence` (the shipped `strings.TrimSpace` comparison at `internal/workflow/reconcile.go:637`) fails the same classification validator |
| ROC-098 | G | Substring containment is not presence | Wrong-input fixture `contains-substring-presence` (the shipped `strings.Contains` check at `internal/workflow/reconcile.go:625`) fails the same classification validator |
| ROC-099 | U | All optimal alignments are computed | A fixture with two distinct maximum-cardinality alignments exercises both; an implementation that materializes one traceback fails the row |
| ROC-100 | C | Ambiguous bounds refuse | A fixture whose optimal alignments disagree on `(left, right)` refuses with `alignment-ambiguous` |
| ROC-101 | G | No tie is selected | Wrong-input fixture `alignment-picks-first-lcs` (a mapper resolving a tie by preferring the lexicographically or DP-order-first traceback) fails the same unanimity validator |
| ROC-102 | U | Unanimity decision is exact | The `F`/`G` procedure of A4 accepts every fixture whose alignments provably agree and refuses every fixture where a matched pair violates either half of the condition |
| ROC-103 | U | Repeated records do not refuse by themselves | A base and upstream file containing many byte-identical records still maps when the alignment is unanimous; a mapper refusing on repetition alone fails the row |
| ROC-104 | U | File-edge boundaries use sentinels | `b == 0` with no matched base record before it maps with `left = 0`, and `b == len(B)` with no matched base record at or after it maps with `right = len(U)` |
| ROC-105 | C | Sentinel away from a file edge refuses | A mid-file boundary with no surviving matched base record before it refuses with `alignment-left-unanchored`, and one with none at or after it refuses with `alignment-right-unanchored` |
| ROC-106 | U | Same-boundary runs combine canonically | Two runs at one `(path, b)` concatenate in ascending effect-ordinal then run-ordinal order, producing byte-identical output across runs |
| ROC-107 | U | Multi-boundary file maps independently | Two distinct boundaries in one file each map to their own disjoint gap and both classify |
| ROC-108 | C | Non-strictly-separated gaps refuse | Two distinct boundaries that collapse onto one upstream region because every base record between them was deleted refuse with `alignment-range-overlap` — in **both** the non-empty-gap and the **empty-gap** shape, since the empty-gap shape would otherwise leave two insertions at one offset with no ordering rule; a mapper using non-strict `right₁ ≤ left₂` passes the non-empty case and fails this row |
| ROC-109 | U | Descending application order | Distinct offsets apply in descending upstream byte offset; an ascending implementation produces a different result on a two-offset fixture and fails the row |
| ROC-110 | U | One immutable snapshot | Every boundary in one candidate is derived against a snapshot taken before the first classification; a re-read mid-derivation fails the row |
| ROC-111 | U | No-newline insertion at the file end | An insertion run whose last record has no terminator maps at `b == len(B)` when the base file ends with `\n` |
| ROC-112 | C | No-newline insertion elsewhere refuses | The same run at any other boundary, or against a base whose final record has no terminator, refuses with `record-terminator-unsupported` |
| ROC-113 | C | L1-L3 limits refuse before allocation | A side exceeding `4194304` bytes or `131072` records, and a pair whose `(len(B)+1) × (len(U)+1)` exceeds `1048576`, each refuse with `alignment-limit`; the refusal is observed **before** either dynamic-program matrix is allocated, nothing is truncated and no partial candidate is produced |
| ROC-114 | G | The limits are not tunable | Wrong-input fixture `alignment-limits-configurable` (a limit sourced from config, an environment variable or a flag) fails the same limit-constant validator that the compiled-in literals pass |
| ROC-115 | U | L3 is a stated memory bound | At the L3 ceiling the two `int32` dynamic-program matrices the A4 procedure allocates total at most `8 MiB`, and one alignment's peak working set stays under the stated `16 MiB` plus O(records) budget; a build whose matrices are wider than `int32` cells, or that allocates a third full matrix, fails the row |
| ROC-116 | G | No fuzzy, scored or fixed-window matching | Wrong-input fixtures `similarity-scored-alignment` (a mapper accepting a best-scoring near match) and `fixed-context-window-N` (a mapper deciding on a fixed number of surrounding records) each fail the same alignment validator |
| ROC-117 | U | Alignment is deterministic across runs | Two derivations from the same two trees produce identical bounds, identical classifications and identical operation ordinals |
| ROC-118 | U | Rule evaluation is fixed first-match and the reason is deterministic | A boundary that violates several rules reports the earliest step's code in the §6.7 order: a fixture that is both over L3 and ambiguous reports `alignment-limit`; one that is both ambiguous and would be left-unanchored reports `alignment-ambiguous`; one that is both unanchored and gap-conflicting reports `alignment-left-unanchored`. A file with two refusing boundaries reports the lower `(path, b)` boundary's code, and two runs over the same trees report the identical code and boundary |
| ROC-119 | G | Unanimity is decided before sentinel legality, and sentinels need no traceback | Wrong-input fixture `sentinel-checked-before-unanimity` (a mapper answering "is this boundary anchored?" from one materialized alignment before A4 runs) fails the same alignment-ordering validator, and reports `alignment-left-unanchored` where the ordered configuration reports `alignment-ambiguous`; in the passing configuration the start sentinel is observed exactly when `left₀ == 0` and the end sentinel exactly when `right₀ == len(U)`, with no traceback enumerated |

### 9.6 Typed roots, lazy fetch and candidate execution

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-120 | U | Typed roots are structural | Candidate file IO accepts `ReferenceRoot` or `CandidateRoot` and no bare `string` root; `OpenReferenceRoot` and `CreateCandidateRoot` are the only candidate-package functions taking one, `.tpatch/` reads go through `*store.Store`, and a signature taking `string` elsewhere fails the row at compile time |
| ROC-121 | G | The guard excludes symbols, not scopes | Wrong-input fixture `third-bare-root-constructor` (a new candidate-package helper taking a `string` root) fails the same source-inventory guard that the two sanctioned constructors pass |
| ROC-122 | C | Constructors validate their commit | A malformed, non-hex or locally absent commit refuses at the constructor, before any read |
| ROC-123 | S | Constructors validate their path | A candidate root path escaping the store root is refused by `safety.EnsureSafeRepoPath` (`internal/safety/safety.go:12`) at construction |
| ROC-124 | I | `GIT_NO_LAZY_FETCH=1` on every candidate Git call | Every Git subprocess issued through either root carries `GIT_NO_LAZY_FETCH=1`, asserted through a PATH wrapper in the shape of `internal/gitutil/apply_classifier_test.go:319-320` |
| ROC-125 | G | Lazy fetch cannot be reintroduced | Wrong-input fixture `candidate-git-without-no-lazy-fetch` (a candidate-path Git invocation built without the environment) fails the same environment-inventory guard |
| ROC-126 | C | A promisor-absent object refuses, never fetches | With a partial clone whose reference object is absent, the run refuses with `reference-not-reconstructable` and issues no network call |
| ROC-127 | G | `evaluateRecipeOperations` is not candidate authority | Wrong-input fixture `candidate-reuses-evaluate-recipe-operations` (candidate classification delegating to `internal/workflow/reconcile.go:611`) fails the same authority-boundary validator |
| ROC-128 | G | `ExecuteRecipe` is not candidate authority | Wrong-input fixture `candidate-reuses-execute-recipe` (candidate execution delegating to `internal/workflow/recipe.go:90`, which continues past per-op errors at `:102-108`) fails the same authority-boundary validator |
| ROC-129 | G | The live-root operation helpers are not candidate authority | Wrong-input fixtures `candidate-reuses-execute-operation` (`internal/workflow/recipe.go:199`, hardcoding `s.Root` at `:200` and `0o644` at `:207-211`) and `candidate-reuses-dry-run-operation` (`:128-129`) each fail the same authority-boundary validator |
| ROC-130 | U | Closed candidate op kinds | Exactly `create-file` and `insert-by-boundary` decode and execute; a third kind refuses |
| ROC-131 | G | Unknown candidate kind fails closed | Wrong-input fixture `candidate-registry-open-default` (a writer registry whose default arm executes or skips instead of refusing) fails the same registry-totality validator |
| ROC-132 | I | Candidate worktree is detached at upstream | The worktree is created detached at `upstreamCommit` under `.tpatch/reconcile-candidates/<slug>/<id>/worktree/` |
| ROC-133 | C | All-or-nothing on precondition failure | One failing precondition abandons the whole candidate and removes the worktree; no partial candidate and no per-op success accounting survives |
| ROC-134 | C | All-or-nothing on write failure | A forced write failure on the last operation abandons the whole candidate |
| ROC-135 | S | Generation mutates no live repository content | With repository-tracked content hashed before and after, candidate generation changes **zero** bytes outside the four expected write sets of §6.8; a guard asserting a fifth exempted set, or omitting `.git/worktrees`, fails the row |
| ROC-136 | G | Generation live-mutation guard | Wrong-input fixture `generation-writes-live-tree` (a generation path writing repository content under `s.Root`) fails the same live-mutation validator |
| ROC-137 | C | Candidate worktree does not disturb shadows | Creating a candidate does not reap, resolve or prune any `.tpatch/shadow` worktree (`internal/gitutil/shadow.go:56,115,152`) |
| ROC-138 | S | Path safety per root | A candidate operation whose path escapes its root is refused by `safety.EnsureSafeRepoPath` applied against **that** root, not the live root |

### 9.7 Validation, idempotency and effect absorption

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-139 | I | Complete set applied once and verified | The derived set is applied exactly once in the candidate worktree; every operation's exact insertion records are present at the re-derived boundary and every creation is byte-identical to its effect postimage; a re-application during validation fails the row |
| ROC-140 | C | Postcondition failure refuses | A forced byte difference at one boundary refuses with `candidate-postcondition-failed` |
| ROC-141 | C | Path-set mismatch refuses both ways | A path differing from upstream that is not in the derived set, and a derived path that does not differ from upstream, each refuse with `candidate-path-set-mismatch` |
| ROC-142 | C | Mode drift refuses | A touched path landing as anything other than regular `100644` refuses with `candidate-mode-drift` |
| ROC-143 | C | Unmodeled change refuses | Any content or mode difference outside the derived set refuses with `candidate-unmodeled-change` |
| ROC-144 | C | Conflict marker refuses | A `<<<<<<<`, `=======` or `>>>>>>>` line in a touched file refuses with `candidate-conflict-marker` |
| ROC-145 | I | Idempotency second pass is all-present | Re-running the derivation with the candidate result as upstream classifies **every effect** present with zero applicable and zero conflict, and leaves the Git tree hash and the candidate diff bytes unchanged |
| ROC-146 | G | Idempotency covers effects, not just operations | Wrong-input fixture `idempotency-checks-operations-only` (a second pass asserting only the derived operations, ignoring absorbed effects) fails the same idempotency validator |
| ROC-147 | C | Non-idempotent candidate refuses on every second-pass outcome | A second pass yielding one applicable operation, one `boundary-conflict`, an `alignment-ambiguous`, an `alignment-limit`, a `record-terminator-unsupported`, a changed tree hash or a changed diff each refuse with `candidate-not-idempotent`, carrying the second-pass code as diagnostic detail; a build that treats any non-all-present second pass as success fails the row |
| ROC-148 | G | Idempotency reuses the production classifier | Wrong-input fixture `bespoke-idempotency-checker` (a separate check with its own comparison logic rather than a second run of the production classifier) fails the same idempotency validator |
| ROC-149 | I | Absorbed effects legitimately omitted | A candidate diff omitting effects already present upstream is valid, and each such effect maps to `absorbed-present` |
| ROC-150 | C | Unexplained effect refuses | An effect that is neither absorbed-present nor candidate-applicable refuses with `effect-unexplained` |
| ROC-151 | G | Absorption accounting is total | Wrong-input fixture `effect-map-omits-absorbed` (an effect map listing only realized operations) fails the same accounting validator |
| ROC-152 | C | Generation runs no test command | With `test_command` configured (`internal/cli/cobra.go:3731-3732`) and a sentinel command that writes a marker file, candidate generation leaves the marker absent |
| ROC-153 | G | No default test execution | Wrong-input fixture `generation-runs-test-command` (a generation path invoking `RunTestCommandInShadow`, `internal/workflow/validation.go:273`) fails the same execution-boundary validator |
| ROC-154 | I | Acceptance runs the test command in the candidate worktree | With `test_command` configured, `--accept-candidate` runs it at step 5, **before** step 10's first live write; the marker appears and the live tree is still untouched at that point |
| ROC-155 | U | Structural wording without tests | Every surface describing a generated candidate says **structurally validated**; the words "validated", "verified" or "tested" standing alone fail the row |
| ROC-156 | U | `validated` and `accept_validation` split | `candidate.json`'s `result.validated` is fixed at `structural`; the acceptance-time result lives in `state.json`'s `accept_validation`, and any other placement refuses at decode |

### 9.8 Namespace, identity, artifacts and lifecycle

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-157 | U | Namespace layout | Artifacts land under `.tpatch/reconcile-candidates/<slug>/<candidate-id>/` with `candidate.json`, `state.json`, `candidate.patch`, `worktree/`; nothing is written under `.tpatch/shadow` (`internal/gitutil/shadow.go:35`) |
| ROC-158 | G | Shadow namespace not reused | Wrong-input fixture `candidate-under-shadow-dir` (a candidate materialized under `.tpatch/shadow`) fails the same namespace validator |
| ROC-159 | I | Resolver pruning does not reap candidates | A resolver run that reaps prior shadows (`internal/gitutil/shadow.go:56,72`) leaves every pending candidate worktree intact |
| ROC-160 | U | Candidate ID format | The ID matches `rc_[0-9a-f]{12}` exactly |
| ROC-161 | U | Candidate ID inputs | Changing any one identity-tuple member — slug, `source_feature_state`, upstream commit, reference commit, coverage/patch/recipe hash, derivation mode, any derived-operation proof hash or any effect-map proof hash — changes the ID; changing nothing keeps it; in particular the same bindings derived at `applied` and at `blocked` produce two different IDs |
| ROC-162 | U | Candidate ID excludes results, status and time | Changing only `candidate_diff_sha256`, only `candidate_tree_sha256`, or only `state.json`'s `status` leaves the ID unchanged; two derivations separated in time produce the identical ID; and the directory is created with its final name with no staging rename anywhere in generation |
| ROC-163 | G | Identity does not include execution results | Wrong-input fixture `id-includes-diff-hash` (an identity tuple carrying the candidate diff or tree hash) fails the same identity validator, because the ID would no longer be computable before execution |
| ROC-164 | G | No circular staging rename | Wrong-input fixture `candidate-staged-then-renamed` (a generation path creating a temporary directory and renaming it once the ID is known) fails the same namespace validator |
| ROC-165 | I | Identical rerun reuses after full validation | A rerun with identical bindings reuses the existing candidate **only after** the full §6.8 validation and idempotency proof re-passes; a reuse that skips them fails the row |
| ROC-166 | C | Collision, integrity drift and status changes | A recomputed ID matching an on-disk candidate whose immutable identity subset differs refuses with `candidate-id-collision`; identity subset equal but a differing stored `result` field refuses with `candidate-integrity-drift`; neither overwrites anything; and flipping `state.json` from `pending` to `rejected` and re-deriving produces the same ID and neither code |
| ROC-167 | U | Candidate metadata strict decode | Unknown fields, nulls, duplicate members and out-of-enum values each refuse with `candidate-metadata-malformed`; a `status` member anywhere in `candidate.json` refuses at decode; and two derivations from identical inputs produce byte-identical, timestamp-free `candidate.json` |
| ROC-168 | S | Artifacts carry no bodies, indices or offsets | No context line, insertion line, file body, byte offset, record index or line number appears in `candidate.json`, `state.json` or `rejected.jsonl`; only digests |
| ROC-169 | G | Alignment persistence guard | Wrong-input fixture `candidate-json-carries-record-indices` (a metadata schema with a boundary or record-index field) fails the same artifact-schema validator |
| ROC-170 | U | Persisted paths are normalized and portable | Every persisted `path` is repository-relative and `filepath.ToSlash`-normalized, an absolute or backslash-separated path refuses with `candidate-path-not-normalized`, and the same fixture derived on Linux, macOS and Windows produces byte-identical `candidate.json` and `state.json` |
| ROC-171 | I | `candidate.patch` is reviewable diff source | The patch artifact is a unified diff and may contain source lines, exactly as `artifacts/post-apply.patch` does |
| ROC-172 | C | `candidate.patch` is not an acceptance input | Corrupting `candidate.patch` does not change acceptance behavior: acceptance re-derives from the bound inputs and refuses or proceeds on those grounds alone |
| ROC-173 | U | Status pointer is separate, `omitempty` and merge-preserved | `ReconcileSummary` (`internal/store/types.go:372`) gains `operation_candidate` carrying `candidate_id`, `path`, `status` and `source_feature_state`; a feature with no candidate serializes byte-identically to the S0 golden; `ShadowPath`/`ResolveSession` (`:381-382`) are untouched; and every production writer of the summary obtains it from the shared status merge that carries the loaded pointer forward (§6.11, ROC-285) |
| ROC-174 | I | Candidate persistence changes no lifecycle state | A feature that was `blocked` before a candidate-ready reconcile is still `blocked` after it, with `status.Notes` byte-identical; the same holds for `applied` |
| ROC-175 | G | `updateFeatureState` is bypassed, and nothing else is reimplemented | Wrong-input fixtures `candidate-routes-through-update-feature-state` (a candidate-ready result reaching `internal/workflow/reconcile.go:1181`, whose `ReconcileStillNeeded` arm forces `StateApplied` at `:1220-1222`) and `candidate-path-reimplements-evidence-persistence` (a dedicated path that writes `reconcile-session.json`, `reconcile.md` and the evidence lines itself instead of calling `saveReconcileArtifacts(s, slug, result, nil)`, thereby losing the `:766` → `:771` order or a supporting detector) each fail the same lifecycle-preservation validator; the passing configuration calls the shipped function with the two staged attempts and a `nil` proof and dedicates only the status write (§6.11) |
| ROC-176 | I | Recovery preflight precedes phase 1, under the lock | With an acceptance journal present, a plain `tpatch reconcile <slug>` refuses with `recovery-required` before phase 1 runs; the per-feature lock was held for the inspection and is released before the refusal returns; no phase output, no evidence line and no pruning occurs, and the staleness path does not run |
| ROC-177 | I | Staleness preflight precedes phase 1 and phase 1.5 | With no journal, a changed upstream commit — or a changed binding hash, or a changed `source_feature_state` — marks the prior **pending** candidate `stale`, prunes its worktree, retains its `candidate.json` and clears the pointer, all before phase 1 and before the phase-1.5 sweep |
| ROC-178 | G | No silent overwrite | Wrong-input fixture `candidate-overwritten-in-place` (a new derivation replacing the prior `candidate.json` bytes) fails the same lifecycle validator |
| ROC-179 | C | Candidate blocks resolver actions | With a pending candidate, `--resolve`, `--accept`, `--reject` and `--shadow-diff` each refuse with `candidate-pending` and exact remediation naming both clearing commands |
| ROC-180 | C | Shadow blocks candidate actions | With a resolver shadow present, `--accept-candidate`, `--reject-candidate`, `--candidate-diff` and `--clear-candidate-rejections` each refuse with `shadow-pending` and exact remediation |
| ROC-181 | C | Shadow blocks candidate derivation | With a resolver shadow present, phase 2 records E15 `shadow-pending` and falls through; no candidate is created |
| ROC-182 | G | Coexistence is refused, not merged | Wrong-input fixture `candidate-and-shadow-coexist` (both staged for one slug with both accept paths reachable) fails the same coexistence validator |
| ROC-183 | C | Rejected ID is checked before creation | A derivation whose ID is already in `rejected.jsonl` records `candidate-rejected` and falls through **without** creating a directory or a worktree |
| ROC-184 | G | No inferred retry | Wrong-input fixture `reject-then-regenerate` (a reconcile that recreates a rejected ID without an explicit clear) fails the same rejection-durability validator |
| ROC-185 | I | `--clear-candidate-rejections` does exactly one thing | It truncates the slug's `rejected.jsonl` and changes no state, no pointer, no artifact and no candidate |
| ROC-186 | U | Permitted feature states, checked outside the E-gate set | A feature at `applied` and one at `blocked` (`internal/store/types.go:13,17`) each reach E1; every other `store.FeatureState` — `requested`, `analyzed`, `defined`, `implementing`, `active`, `reconciling`, `reconciling-shadow`, `upstream_merged`, `rejected`, `unapplied` — records `candidate-feature-state-unsupported` and falls through **without reaching E1**; and wrong-input fixture `feature-state-as-sixteenth-gate` (an eligibility table carrying a state row inside E1-E15) fails the same gate-inventory validator, which asserts the closed set is exactly the fifteen coverage-binding recomputations |
| ROC-187 | U | The bound state is recorded in three places, and binding it changes nothing | `source_feature_state` appears in the identity tuple, in `candidate.json` and in the `status.json` pointer, and the three agree; a candidate whose pointer and artifact disagree refuses at decode; and a candidate-ready reconcile of a `blocked` feature leaves `status.State` at `blocked` with `status.Notes` byte-identical |
| ROC-188 | C | Feature-state drift refuses acceptance; a matching state accepts to applied | With a candidate bound at `blocked`, transitioning the feature to `rejected`, to `unapplied`, to `upstream_merged`, to `reconciling-shadow` or to `applied` and then running `--accept-candidate` refuses with `candidate-feature-state-changed`, exit `3`, naming both states — and no journal, snapshot, staged artifact or live path is written. With the state unchanged the acceptance succeeds and leaves `status.State` at `store.StateApplied` (`internal/store/types.go:13`), whether the bound state was `applied` or `blocked` |
| ROC-189 | C | An accepted directory is immutable audit, and its identity does not regenerate | After acceptance, the directory retains `candidate.json`, `state.json` at `accepted` and `candidate.patch`, has no `worktree/`, `staged/`, `snapshots/` or journal, has no `status.json` pointer, is never marked `stale` by a later preflight, and refuses `--reject-candidate` with `candidate-not-pending`; a reconcile that recomputes the same ID records `candidate-already-accepted` and falls through **before** creating a directory or worktree, while any changed binding — including `source_feature_state` — mints a different ID and a new candidate normally |
| ROC-190 | G | An accepted candidate is not reused | Wrong-input fixture `accepted-candidate-revalidated-and-restaged` (a preflight that re-validates an `accepted` directory and republishes its pointer) fails the same lifecycle validator |

### 9.9 CLI surface and flag exclusivity

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-191 | U | Exactly five new terminal flags | `--candidate-diff`, `--accept-candidate`, `--reject-candidate`, `--clear-candidate-rejections` and `--resume-candidate-recovery` are registered; a sixth, or a missing one, fails the row |
| ROC-192 | U | Eight terminal actions total | The terminal set is exactly the five new flags plus `--accept`, `--reject` and `--shadow-diff` |
| ROC-193 | U | New-flag pair exclusivity | All **10** unordered pairs drawn from the five new flags refuse |
| ROC-194 | G | The exclusivity set is source-derived | Wrong-input fixture `hardcoded-behavioral-flag-list` (a validator carrying a literal list of existing flag names instead of enumerating `cmd.Flags()` minus the `--path` / `--format` allowlist and `help`) fails the same flag-inventory guard; adding a sixteenth reconcile flag to the fixture makes the hardcoded build pass where the enumerated build refuses |
| ROC-195 | U | New-versus-existing exclusivity | All **75** pairs of one new flag with one of the fifteen reconcile-local behavioral flags registered at `internal/cli/cobra.go:2520-2542` refuse when that flag is `Changed`, including explicit `--upstream-ref`, `--allow-dirty`, `--allow-stale-lock`, `--max-conflicts`, `--model` and `--timeout`; **85** refusing pairs involve a new flag in total, with Cobra's auto-registered `help` excluded from the behavioral set (ROC-289) |
| ROC-196 | U | Presentation flags are permitted, and a default is not an explicit set | `--path` (`internal/cli/cobra.go:73`), `--format` (`:2543`) and `--help` each combine with every new terminal flag without refusing, and `--format json` renders the terminal action's output; `--accept-candidate <slug>` alone succeeds even though `--upstream-ref`, `--timeout` and `--format` all carry defaults, while the same invocation with `--upstream-ref` passed explicitly refuses, proving the rule tests `Flags().Changed` rather than emptiness |
| ROC-197 | G | Existing combinations are not re-routed | Wrong-input fixture `existing-flags-through-new-validator` (a build routing an all-shipped-flag invocation through the new stage) fails the same message-parity validator that the shipped path passes |
| ROC-198 | U | Shipped messages are byte-identical | `--accept --reject`, `--accept --resolve` and `--apply` without `--resolve` produce byte-identical strings to the S0 goldens, emitted by the shipped `validateReconcileFlags` (`internal/cli/cobra.go:3200`, rules `:3211-3218`, strings `:3212,3215,3218`) |
| ROC-199 | U | Each new flag takes exactly one slug | Zero slugs and two slugs each refuse at validation with exit `2` |
| ROC-200 | I | `--candidate-diff` prints and mutates nothing | It emits the reviewable diff, writes no artifact, changes no state and leaves `candidate.json` and `state.json` byte-identical |
| ROC-201 | I | `--reject-candidate` records and prunes | It appends to `rejected.jsonl`, sets `state.json` `status: rejected`, prunes the worktree, clears the pointer and changes no lifecycle state |
| ROC-202 | C | Resolver terminals never touch a candidate | With a pending candidate and no shadow, `--accept <slug>` refuses with `candidate-pending` and never falls back to the candidate, and `--shadow-diff <slug>` refuses rather than printing the candidate's diff |
| ROC-203 | I | Resolver flag behavior unchanged | `--accept`, `--reject` and `--shadow-diff` against a real shadow produce output byte-identical to the S0 goldens |
| ROC-204 | U | Exit-code mapping | Flag misuse, an absent or ineligible candidate, a failed CAS and a diverged HEAD exit `2`; coexistence, lock contention, `lock-unsupported`, `lock-filesystem-unsupported`, `candidate-already-accepted`, `candidate-feature-state-changed`, `recovery-required`, `coverage-publication-failed` and `acceptance-rolled-back` exit `3`; only `acceptance-rollback-failed` and unexpected internal errors exit `1` |
| ROC-205 | G | A rolled-back acceptance is not an internal error | Wrong-input fixture `rolled-back-exits-one` (a completed rollback returning exit `1`) fails the same exit-envelope validator against ADR-031 D4 (`internal/cli/reject.go:36-47`) |

### 9.10 Evidence multiplicity, verdicts and content

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-206 | U | Phase and kind reuse existing enums | Candidate evidence carries `phase: phase-2` and `evidence_kind: recipe-operation-match` (`internal/store/reconcile_evidence.go:41`); no new enum value is added |
| ROC-207 | G | No phase 2.5 | Wrong-input fixture `phase-2.5-evidence-value` (an evidence entry carrying a `phase-2.5` value) fails the same closed-enum validator that ADR-025 D4's six values pass |
| ROC-208 | I | Two ordered verdict-bearing attempts, by the route each arm uses, in both sinks | A candidate-ready outcome writes exactly two **verdict-bearing** attempts in `reconcile-evidence.jsonl` — `phase-2` first, `phase-4` second, both from `result.verdictAttempts` and both at the shipped `:766` position — and a phase-2 blocked classification followed by a phase-4 reapply also writes exactly two, the phase-2 one appended directly by the evaluator and the terminal one derived by the shipped builder from an empty staged list; **on both routes every verdict-bearing attempt reaches both sinks in the same relative order** — `reconcile-evidence.jsonl` via `store.AppendReconcileEvidence` (`internal/workflow/reconcile.go:876`) and the in-memory `result.Evidence` slice — so `reconcile-session.json`'s serialized `evidence` array (`:775-776`) is line-for-line and order-for-order the same sequence as the JSONL for that invocation; in both cases they sit beside whichever supporting entries that outcome's shipped guards admit (file-novelty and hunk-overlap in both, and no blocked-only entry in either); a row asserting exactly two lines in the file fails, one asserting a fixed supporting count independent of the outcome fails, one asserting the candidate-ready pair came from the derived builder fails, and a direct-append implementation that writes the JSONL line but omits the `result.Evidence` append — leaving the session file's array a strict subset of the JSONL — fails the row |
| ROC-209 | U | Phase-local verdicts, with the candidate-ready pair's exact values | The candidate-ready run's staged attempt [1] carries `phase: phase-2`, `evidence_kind: recipe-operation-match`, `raw_reconcile_verdict: still_needed` and `confidence: high`, while staged attempt [2] carries `phase: phase-4`, `evidence_kind: forward-apply`, `raw_reconcile_verdict: blocked`, `confidence: low` and `reason_code: phase-4-forward-apply-conflicts` (`phase-4-blocked` on the exhausted arm); a phase-2 conflict followed by a clean phase 4 writes `blocked` then `reapplied`; a run whose attempts all carry the invocation's returned outcome — `still_needed` on both candidate-ready lines — fails the row; and all of them round-trip through the shipped `RawReconcileVerdict` field (`internal/store/reconcile_evidence.go:96`) with no schema-version bump |
| ROC-210 | C | Phase-2 finding survives the terminal phase, on both routes | The phase-2 attempt is still present and readable after the terminal attempt is appended — as staged attempt [1] beside [2] on a candidate-ready run, and as the directly appended entry beside the derived terminal entry on a fallthrough; the shipped single-`result.Phase` derivation (`internal/workflow/reconcile.go:1161-1176`) fails the row on both |
| ROC-211 | U | At most one **verdict-bearing** attempt per (invocation, phase) | A run producing two verdict-bearing phase-2 attempts fails the row, including a candidate-ready run whose derived `phase-2` entry is appended beside staged attempt [1]; a candidate-ready run producing one `phase-2` plus one `phase-4` verdict-bearing attempt **passes**, because they are different phases; a run producing four `phase-3.5` lines — **one verdict-bearing plus three supporting** — and two `phase-4` lines — one verdict-bearing plus one supporting — **passes**, because that is the shipped shape (`internal/workflow/reconcile.go:766-771`); a fixture that classifies all four `phase-3.5` lines as supporting fails the row |
| ROC-212 | I | Review renders both layers | `tpatch reconcile review` prints every attempt in phase order with its verdict labelled **phase-local**, and prints the invocation's returned outcome separately |
| ROC-213 | G | A phase-local verdict is not the answer | Wrong-input fixture `review-prints-phase-verdict-as-outcome` (a renderer presenting the phase-2 verdict as the command's result) fails the same rendering validator |
| ROC-214 | C | New attempt ID is expected, not a regression | The new authoritative phase-2 entry mints a **different** `attempt_id` from any pre-GH-#13 entry; a test asserting byte identity with a historical entry fails the row |
| ROC-215 | C | Attempt-ID idempotency and corruption | Rerunning an identical reconcile appends no duplicate line (`internal/store/reconcile_evidence.go:166-170`), and a forged line sharing an `attempt_id` with a different payload refuses with `ErrMalformedEvidence` (`:171`) |
| ROC-216 | U | Confidence ladder | An all-present result satisfying CG1-CG7 and a finalized candidate are `high`; a conflicting effect is `low`; every E-gate ineligibility is `unknown` |
| ROC-217 | S | No source bodies in evidence | No evidence field carries a context line, an insertion line or a file body; `matched_paths` and `matched_operations` carry slash-normalized paths and identifiers only |
| ROC-218 | G | Evidence privacy boundary | Wrong-input fixture `evidence-carries-alignment-records` (an evidence entry embedding the surviving matched records) fails the same privacy validator |
| ROC-219 | I | Diagnostic parity is by semantic field | A terminal candidate result answers every question §6.17's parity table lists — each effect's path, its resolution, its reason codes, the derived operations and a reviewable `candidate.patch` — so its field coverage meets or exceeds the phase-4 blocked path's; and a fixture asserting an equal **count** of detector evidence entries fails the row, because path-restructure and blocked-classification are blocked-only (`internal/workflow/reconcile.go:947-949,998-1000`) and a candidate-ready outcome is not blocked |
| ROC-220 | G | Candidate result is not diagnostically thinner, and no detector is re-tuned | Wrong-input fixtures `candidate-evidence-summary-only` (a candidate result carrying counts but no per-effect mapping) and `blocked-detectors-forced-on-candidate` (a candidate-ready path invoking the blocked-only detectors so the line counts match) each fail the same diagnostic-parity validator, the second because it changes shipped detector semantics rather than adding candidate diagnostics |
| ROC-221 | U | Supporting entries are unchanged, correctly bounded and deterministically ordered | For a blocked reconcile reaching phase 3.5, the file-novelty (`internal/workflow/file_novelty.go:115-116`), hunk-overlap (`hunk_overlap.go:135-136`) and path-restructure (`path_restructure.go:118-119`) entries — and, for an `upstreamed` reconcile, the confirmation-gate entry (`reconcile.go:1069-1070`) in place of path-restructure — keep their shipped `phase`, `evidence_kind`, `confidence`, `raw_reconcile_verdict` and `reason_code` byte-for-byte against the S0 golden; **three** supporting `phase-3.5` entries plus **one** supporting `phase-4` entry is the shipped maximum and **passes** the cardinality validator; the gate's entry stays classified supporting at `phase-3.5` / `manual-review` and carries no phase-2 authority; and two identical reconciles emit the same lines in the same order — the staged verdict-bearing attempts in slice order at the shipped `internal/workflow/reconcile.go:766` position, then the shipped call order `:766` → `:771` with every firing condition unchanged |
| ROC-222 | G | A supporting verdict is not a phase verdict | Wrong-input fixture `supporting-entry-rendered-as-phase-verdict` (a renderer labelling `blocked-classification`'s `raw_reconcile_verdict` as the phase-4 verdict) fails the same rendering validator that the two-class configuration passes |

### 9.11 Locks, acceptance transaction, coverage publication and recovery

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-223 | U | Lock abstraction is new, with exact paths | The reconcile locks are `.tpatch/locks/reconcile/<slug>.lock` and `.tpatch/locks/worktree-mutation.lock`, opened under the store root after path validation through the new abstraction; a call reaching `rescap.AcquireLock` (`internal/rescap/lock_unix.go:63`) or `rescap.EnsureLocalContract` (`internal/rescap/scratch.go:48`) from a reconcile path fails the row |
| ROC-224 | G | The rescap contract is not re-imposed | Wrong-input fixture `reconcile-lock-requires-local-contract` (a reconcile lock path running the `statfs` preflight or the `.tpatch/local` gate) fails the same lock-contract validator |
| ROC-225 | I | Both locks acquired in order | Acceptance acquires the per-feature reconcile lock, then the global worktree-mutation lock, in that fixed order |
| ROC-226 | C | Second-acquisition failure releases the first | With the global lock already held elsewhere, a failed acceptance leaves the per-feature lock **released**; a probe acquires it immediately afterwards |
| ROC-227 | C | Held lock refuses immediately; a crash releases it | A second acceptance while the first holds either lock refuses with `candidate-lock-held` and exit `3` without waiting or retrying, and a process killed while holding a lock leaves the next acquisition succeeding with no unlock step and no cleanup |
| ROC-228 | U | No PID metadata and inert residue | The lock file is zero-length after acquisition and a pre-existing empty lock file from an earlier run neither blocks nor signals anything; a build writing or parsing owner data fails the row |
| ROC-229 | G | No stale-lock breaking exists | Wrong-input fixture `stale-lock-broken-by-age-or-pid` (a lock path that breaks a lock on age, on a missing PID, or on any flag) fails the same lock-model validator |
| ROC-230 | C | `--allow-stale-lock` does not apply | `--allow-stale-lock` (`internal/cli/cobra.go:2524`) governs only ADR-017's baseline guard (`internal/gitutil/lock_guard.go:99-100`) and does not weaken either mutex |
| ROC-231 | I | The staleness preflight runs under the lock, and releases before phase 1 | With a probe holding `.tpatch/locks/reconcile/<slug>.lock`, a reconcile's preflight performs no inspection, no stale marking and no prune, records `candidate-lock-held` and continues into phase 1 with derivation disabled; releasing the probe and re-running performs the stale marking normally; and a reconcile that reaches a phase-3 provider call holds neither lock at that moment, so a concurrent probe acquires the per-feature lock during the call |
| ROC-232 | G | There is no unlocked candidate mutation, and finalization revalidates | Wrong-input fixture `unlocked-stale-prune` (a preflight marking `stale`, pruning `worktree/` or clearing the pointer without holding the per-feature lock) fails the same lock-coverage guard, which enumerates every write to `state.json`, the pointer, `rejected.jsonl` and the candidate worktree and asserts each is dominated by a hold; and mutating candidate state between the preflight's release and phase-4 finalization makes finalization observe the change and refuse rather than act on the stale observation |
| ROC-233 | C | Remote and unknown filesystems refuse | With the lock directory on a filesystem class the constructor does not recognize as local — an NFS/SMB `f_type` on linux, a non-`MNT_LOCAL` mount on darwin, `DRIVE_REMOTE` or a UNC path on Windows — acquisition refuses with `lock-filesystem-unsupported`, nonterminal during generation and exit `3` on a candidate action; an unrecognized `f_type`, an unrecognized `f_fstypename` and `DRIVE_UNKNOWN` refuse the same way, and a build implementing the check as a known-remote denylist rather than a known-local allowlist fails the row |
| ROC-234 | G | The rescap reason codes are not reused | Wrong-input fixture `reconcile-lock-emits-rescap-reason` (a reconcile lock path returning `ReasonResourceLockFSUnsupported` or requiring an ignore-file contract) fails the same lock-contract validator; the reconcile path has its own code and no `.tpatch/local` precondition |
| ROC-235 | U | Windows uses stdlib syscall, adds no dependency, and claims no cross-host safety | The Windows lock resolves `LockFileEx` through `syscall.NewLazyDLL` in a `//go:build windows` file, and `go.mod`'s require set stays exactly `github.com/spf13/cobra` with the `pflag` and `mousetrap` indirects; a build importing `golang.org/x/sys` fails the row; and no surface, doc line or artifact field says the advisory lock makes acceptance safe across hosts — the stated authority is the per-path CAS and the journal |
| ROC-236 | C | Unsupported platform is named at both surfaces | On a build target with `Supported == false` — which in v1 is every target other than linux, darwin and windows, including every BSD — the preflight and candidate finalization record `lock-unsupported` and fall through so the reconcile returns exactly its shipped outcome, and `--accept-candidate` refuses with the same code and exit `3` |
| ROC-237 | G | The resolver accept primitives are never reached | Wrong-input fixtures `candidate-accept-calls-accept-shadow` (`internal/workflow/accept.go:82`), `candidate-accept-copies-shadow-files` (`internal/gitutil/shadow.go:191`) and `candidate-accept-refreshes-after-accept` (`internal/workflow/refresh.go:45`, which leaves the recipe stale at `:20-24`) each fail the same acceptance-boundary validator |
| ROC-238 | C | No stale-patch application | Acceptance never invokes `gitutil.ForwardApplyExcluding` (`internal/workflow/accept.go:95`) or any textual patch application against the live tree |
| ROC-239 | I | Ancestor HEAD is accepted | With HEAD several commits ahead of the bound `upstream_commit` and every candidate path still matching the upstream tree, acceptance proceeds |
| ROC-240 | C | Diverged HEAD refuses | With `upstream_commit` not an ancestor of HEAD, acceptance refuses with `head-diverged`, naming both commits, exit `2` |
| ROC-241 | G | HEAD equality is not required | Wrong-input fixture `accept-requires-head-equals-upstream` (an acceptance path demanding `HEAD == upstream_commit`) fails the same precondition validator that the ancestor configuration passes |
| ROC-242 | C | Content and mode CAS | A candidate-modified path whose live bytes differ from the `upstreamCommit` blob by one byte, and one whose live mode differs, each refuse with `candidate-cas-failed` |
| ROC-243 | C | Created-path collision refuses | An untracked file at a candidate-created path refuses with `candidate-path-occupied` and destroys nothing |
| ROC-244 | I | Unrelated dirt is allowed | Uncommitted edits outside the candidate path set do not block acceptance and are untouched by it |
| ROC-245 | G | CAS is the acceptance authority, and it is re-run | Wrong-input fixtures `accept-skips-cas-on-ancestor-head` (an acceptance path treating a passing ancestry check as sufficient) and `accept-skips-cas-recheck` (a path that validates at step 6 only and writes at step 10 without re-validating) each fail the same precondition validator that the two-pass configuration passes |
| ROC-246 | C | Binding change refuses | A coverage, patch or recipe byte changed since derivation refuses with `candidate-binding-changed` |
| ROC-247 | I | Alignment re-derived at accept | Acceptance recomputes the alignment and candidate bytes from the bound inputs and compares the identity tuple, diff hash and tree hash; a path that trusts the stored proof hashes alone fails the row |
| ROC-248 | G | Stored proofs are not authority | Wrong-input fixture `accept-trusts-stored-proof-hashes` (acceptance skipping re-derivation because the stored hashes match) fails the same acceptance-authority validator |
| ROC-249 | U | Staging precedes the first live write | Every staged artifact — residual patch, recipe, provenance, generation entry, coverage record — exists under `staged/` before step 10 begins |
| ROC-250 | U | Snapshots cover the named set and carry absent markers | `snapshots/` holds every live candidate path plus `artifacts/post-apply.patch`, `artifacts/apply-recipe.json`, `artifacts/recipe-provenance.json`, `artifacts/recipe-coverage.json`, `artifacts/patch-generations.json` and `status.json`, together with the pre-transaction `status.Apply` and `status.State`; every path the candidate creates and every one of those artifacts that does not yet exist carries an explicit absent marker; a missing member fails the row |
| ROC-251 | C | Rollback restores both sides | A forced failure after step 10 restores modified paths from snapshots, **deletes** every path recorded absent, restores `artifacts/post-apply.patch`, `artifacts/apply-recipe.json`, `artifacts/recipe-provenance.json`, `artifacts/recipe-coverage.json`, `artifacts/patch-generations.json` and `status.json` — including the `operation_candidate` pointer and the pre-transaction `status.Apply` / `status.State` — and leaves `state.json` at `pending` |
| ROC-252 | C | Ordinary rollback exits 3 | A rolled-back acceptance returns `acceptance-rolled-back` with the failing step named and exit `3`; no success line is printed |
| ROC-253 | C | Failed rollback exits 1 and retains the journal | With the rollback itself forced to fail, the command returns `acceptance-rollback-failed`, exits `1`, and the next command refuses with `recovery-required` |
| ROC-254 | C | Publication, artifact and state failure all roll back | A forced coverage-publication failure at step 11 rolls back and returns `coverage-publication-failed` with exit `3`; a forced staged-artifact write failure and a forced state-transition failure each roll back and exit non-zero; there is no warning-class rendering and no success-shaped partial acceptance anywhere |
| ROC-255 | C | Crash leaves a journal and blocks everything | A process kill mid live-write leaves `accept-journal.json`; every subsequent reconcile and candidate action for the slug refuses with `recovery-required` and exit `3` |
| ROC-256 | I | Resume rolls back without a commit marker | `--resume-candidate-recovery` on a journal with no commit marker restores every snapshot, deletes every absent-marked path, returns the candidate to `pending` and removes the journal |
| ROC-257 | I | Resume completes with a commit marker | The same command on a journal carrying the commit marker finishes step 14's cleanup and does **not** roll back |
| ROC-258 | G | Recovery is never implicit | Wrong-input fixture `journal-auto-resumed-on-next-run` (a reconcile that completes or rolls back a journal without the explicit flag) fails the same recovery validator |
| ROC-259 | U | No atomicity claim | No surface, doc line or artifact field claims filesystem-wide atomicity; the guarantee printed is journaled, rollback-capable and explicitly recoverable |
| ROC-260 | I | Coverage published through the shared API | Successful acceptance publishes through ADR-036 D15's single publication entry point with `producer: reconcile-accept` and `capture.mode: reconcile` |
| ROC-261 | U | P3 variant without schema change | The published record is `coverage_status: complete`, the `producer` enum gains no value, `schema_version` stays `1`, `reference.kind` is `commit`, `reference.commit` equals the `upstream_commit` the CAS validated against, and `preimage_set_sha256` recomputes over that same commit's tree |
| ROC-262 | I | Residual patch excludes absorbed effects | The new canonical patch is the `upstreamCommit` → candidate-result diff restricted to the candidate path set, and the new recipe and coverage explain it with no `operation-missing` and no outstanding reason |
| ROC-263 | C | The resolver variant is unchanged | An `AcceptShadow` acceptance still publishes the incomplete `resolver-accept` variant with `producer-patch-rewrite` and `recipe-not-regenerated`, byte-identically to the S0 golden |
| ROC-264 | I | Accepted feature is eligible again | Reconciling the accepted feature against a later upstream passes E1-E15 on the freshly published coverage, so the feature does not degrade |
| ROC-265 | I | Step 12 rewrites the apply base, and the manifest agrees with it | After a successful acceptance, `status.Apply.BaseCommit` (`internal/store/types.go:366`) equals the accepted `upstream_commit`, `Apply.CompletedAt` is set, `Apply.HasPatch` and `Apply.HasRecipe` are `true`, and `status.State` is `store.StateApplied`; the ADR-024 entry staged at step 7 and published at step 11 carries the same `base_commit` with `capture.mode: reconcile`, appended through the shared `AppendPatchGenerationForFeature` (`internal/workflow/patch_generations.go:31`) rather than a new writer; and a forced mismatch between the three — status, manifest and coverage `reference.commit` — refuses the transaction and rolls back |
| ROC-266 | I | The six base readers observe the new base | The next reconcile's `persistReconcileEvidence` (`internal/workflow/reconcile.go:821`), file-novelty (`:888,899,903`), hunk-overlap (`:917,924,931,935`), path-restructure (`:951,965,978`), blocked-classification (`:1005`) and confirmation-gate (`:1037`) reads all stamp the accepted `upstream_commit`; a fixture where any of them still reports the pre-accept base fails the row |
| ROC-267 | G | A stale base is a defect, and rollback restores the apply block | Wrong-input fixture `accept-leaves-stale-base-commit` (an acceptance that publishes the residual patch and coverage but leaves `status.Apply.BaseCommit` unchanged) fails the same base-consistency validator, and its next-reconcile novelty and overlap classifications differ from the correct configuration's; and a forced failure after step 12 restores the pre-transaction `status.Apply` and `status.State` from `snapshots/` along with the artifacts, leaving no partially updated base |

### 9.12 Scope, security and source-derived inventory guards

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-268 | C | Superseded feature is ineligible | A superseded feature with otherwise-complete coverage refuses with `supersession-not-authority`; ADR-029 D7's downgrade (`docs/adrs/ADR-029-write-file-recipe-safety.md:74-76`) grants nothing |
| ROC-269 | G | Supersession severity is not authority | Wrong-input fixture `superseded-warning-implies-eligible` (an eligibility path treating the warning-class downgrade as a grant) fails the same eligibility validator |
| ROC-270 | C | Hard-parent-blocked child skips | A child whose parent is hard-blocked refuses with `parent-blocked` and falls through before any alignment work; ADR-011 behavior is unchanged |
| ROC-271 | C | Phase-3.5 blocked-by-parent unchanged | The shipped short-circuit (`internal/workflow/reconcile.go:552-567`) produces output byte-identical to the S0 golden |
| ROC-272 | I | Multi-slug falls through unchanged | A multi-slug invocation (`internal/cli/cobra.go:2440`) records `derivation-mode-unsupported` and produces output byte-identical to the S0 golden |
| ROC-273 | I | `--cumulative-legacy` falls through unchanged | Output is byte-identical to the S0 `--cumulative-legacy` golden, with ADR-030 D6's reorder skip and D7's phase-1.5 skip intact |
| ROC-274 | G | Live-root call inventory | Wrong-input fixture `new-live-root-call-in-candidate-path` (a newly added candidate-path call reaching `s.Root` for repository content) fails the same source-inventory guard that the shipped candidate package passes |
| ROC-275 | G | Candidate writer/action registry inventory | Wrong-input fixture `unregistered-candidate-writer` (a production writer of a candidate artifact that maps to no registry entry) fails the same registry-inventory guard |
| ROC-276 | G | Accept-transaction write inventory | Wrong-input fixture `unjournaled-accept-write` (a live write inside the acceptance transaction with no snapshot and no journal step) fails the same transaction-inventory guard |
| ROC-277 | S | No Git history mutation | Across generation, acceptance and rejection, `git reflog` and every ref are unchanged; no commit, amend, rebase, reset or ref move occurs |
| ROC-278 | U | Cross-document parity blocks are delimited and byte-identical | The nine marked regions `A`, `B`, `C1`, `C2`, `D1`, `D2`, `D3`, `E` and `F` carrying §6's six parity blocks are byte-identical between this PRD and ADR-037 rev-6; the guard extracts **only** the bytes between a matching `<!-- parity-block:<ID>:begin -->` / `<!-- parity-block:<ID>:end -->` pair, so prose outside a region may differ; a missing, duplicated, unpaired or one-sided marker fails the row; and the implementation's transaction step identifiers match steps 1-14 one-for-one |
| ROC-279 | U | Docs, assets and SPEC parity | The `SPEC.md` §7 phase-2 amendment covers **all three** rows — `→ All already-present → UPSTREAMED` as CG-qualified (`SPEC.md:446`), `→ Any conflict → BLOCKED` as a phase classification (`:447`) and `→ Mix → STILL-NEEDED` as nonterminal, surfacing only at a phase-4 conflict (`:448`) — and the ADR-036 D15 P3 variant note, the ADR-025 verdict-bearing / supporting note, the confirmation-gate authority-class note and every shipped skill asset describing reconcile phases agree with this contract; `assets_test.go`'s parity guard passes; a SPEC edit touching only the conflict row fails the row |
| ROC-280 | U | Full-suite regression | `gofmt -l .` is clean, `go build ./cmd/tpatch` succeeds and `go test -count=1 ./...` passes at every slice boundary |

### 9.13 Proof plumbing, evidence taxonomy, pointer persistence and the acceptance recheck

These rows are rev-3's additions. Each extends an earlier section — §9.2, §9.8,
§9.9, §9.10 or §9.11 — and is numbered here so the matrix stays contiguous
without renumbering 280 reviewed rows.

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-281 | I | The new phase-2 entry's exact fields, and the gate accepting them | A feature that passes E1-E15 and recomputes **every** effect present at `upstreamCommit` writes one verdict-bearing `phase-2` / `recipe-operation-match` entry carrying `confidence: high`, `match_origin: upstream`, `pre_reconcile_presence: present`, `upstream_commit_refs` equal to exactly `[upstreamCommit]` (one element), `upstream_commit == upstreamCommit`, `requires_confirmation: false` and `raw_reconcile_verdict: upstreamed`; the confirmation gate confirms **on that entry** — the substituted `:766` entry **it is grading**, which the gate does not itself write — and the invocation returns `store.ReconcileUpstreamed` with `review_verdict: confirmed-upstreamed`; in the same repository, and **only** with `operation_candidate_enabled` **off** (§6.17's per-arm table row 2), a legacy run that does not reach the all-present result still emits the shipped legacy `recipe-operation-match` entry at `low` / `unknown` / `[]` (`internal/workflow/reconcile.go:826,829,843-845`), byte-identical to the S0 golden — that byte-identity claim is scoped to the gate-off arm alone, because with the gate **on** a legacy or ineligible all-present result instead falls through and its `phase-2` line is the evaluator's directly appended entry (row 4's direct-append route), an intentional delta; any one of the seven field values differing, or the gate-off legacy entry changing, fails the row |
| ROC-282 | U | The proof is a fourth parameter on both functions, `nil` everywhere else | `saveReconcileArtifacts` (`internal/workflow/reconcile.go:724`) and `applyUpstreamedConfirmationGate` (`:1032`) each take `proof *phase2AuthorityProof` as a fourth parameter; all **ten** shipped call sites (`:383,432,455,480,524,536,564,570,583,596`) pass `nil`, including `:570`, whose third argument is the variable `phase35` — the **same `*ReconcileResult` object** as `result`, since `tryPhase35` takes it as its ninth parameter (`internal/workflow/reconcile.go:1363`) and returns that same pointer from all ten of its returns (`:1373,1381,1389,1436,1475,1491,1497,1502,1508,1513`) — so `:570` stages no attempt and passes `nil` exactly like the other nine, and a fixture asserting `:570` receives a second, distinct result object fails the row; the only non-nil argument anywhere in `internal/` originates in the GH #13 evaluator's all-present arm; a signature missing the parameter fails at compile time, and any other production caller passing non-nil fails the source-inventory guard |
| ROC-283 | G | The proof is not smuggled through stored or ambient state | Wrong-input fixtures `proof-carried-on-reconcile-result` (the proof as an **exported** `ReconcileResult` field, which `internal/workflow/reconcile.go:775-776` serializes into `reconcile-session.json`), `proof-in-package-variable` and `proof-in-context-value` each fail the same carrier-boundary validator that the explicit fourth parameter passes; the unexported `verdictAttempts []store.ReconcileEvidence` staging field is the one permitted `ReconcileResult` carrier and the validator distinguishes it by name, by its unexported-ness, by its single assignment and mutation sites and by the absence of any struct tag, exported alias or marshaller; in the passing configuration no proof field reaches any artifact or log line |
| ROC-284 | I | Supporting-entry maxima and mutual exclusion | A blocked invocation writes at most one verdict-bearing plus **three** supporting `phase-3.5` entries (file-novelty, hunk-overlap, path-restructure) and at most one verdict-bearing plus **one** supporting `phase-4` entry (blocked-classification), and **no** gate entry; an `upstreamed` invocation writes file-novelty, hunk-overlap and the gate's `manual-review` and **no** path-restructure or blocked-classification (`:947-949`, `:998-1000`, `:1033-1035`); a candidate-ready invocation returns `store.ReconcileStillNeeded` and writes neither blocked-only entry; a fixture asserting four supporting `phase-3.5` entries, or asserting a blocked-only entry beside a candidate-ready outcome, fails the row |
| ROC-285 | I | A pending candidate survives every unrelated outcome | With a pending candidate for the slug, a later reconcile ending phase-1 `upstreamed`, phase-1.5 `upstreamed`, phase-3 provider-decided, phase-4 `reapplied`, phase-4 `blocked`, gate-off, multi-slug, `--cumulative-legacy`, or refused at the preflight with `candidate-lock-held` / `lock-unsupported` / `lock-filesystem-unsupported`, leaves `status.Reconcile.operation_candidate` naming the same `candidate_id`, `path`, `status` and `source_feature_state`; `--candidate-diff`, `--accept-candidate` and `--reject-candidate` all still resolve it afterwards; and `status.State` and `status.Notes` follow that outcome's shipped rules, unaffected by the carry-forward |
| ROC-286 | G | A status writer that drops or late-repairs the pointer fails | Wrong-input fixtures `status-merge-drops-candidate-pointer` (the shipped wholesale `store.ReconcileSummary` replacement at `internal/workflow/reconcile.go:1191-1209` with no carry-forward) and `pointer-repaired-after-write` (a writer that saves the status and restores the pointer afterwards) each fail the same pointer-preservation validator that the shared merge passes, the second because the carry-forward must precede the replacement rather than repair it |
| ROC-287 | C | Only five transitions change the pointer, and it stays aligned with the directory | Finalization sets the pointer; the staleness preflight, rejection and step-14 cleanup clear it; acceptance updates it at step 12 and clears it at step 14; recovery restores or clears it by commit marker — and every other production construction of `store.ReconcileSummary` is rejected by the same source guard. After each transition the invariant holds both ways: a live pointer names a directory whose `state.json` is `pending`, and no `pending` directory lacks a pointer; a fixture leaving either side alone fails |
| ROC-288 | C | The CAS is re-validated immediately before the first live write | A candidate path mutated after step 6 and before step 10 — changed content, changed mode, or a newly created untracked file at a candidate-created path — refuses at step 10's recheck with `candidate-cas-failed` (`candidate-path-occupied` for the occupied-create case) **before any live byte is written**, leaves the journal without a commit marker and returns the candidate to `pending`; and no CLI string, doc line or artifact field claims the advisory locks prevent an external or cross-host writer — the surfaces name the journal and rollback as the recovery answer |
| ROC-289 | U | Cobra's `help` is excluded from the behavioral set | The enumeration subtracts `--path`, `--format` and Cobra's auto-registered `help`, leaving exactly the fifteen behavioral flags of `internal/cli/cobra.go:2520-2542` and **85** refusing pairs; `--accept-candidate <slug> --help` prints the command's help instead of refusing; a build that leaves `help` in the behavioral set computes `90` pairs and fails the row |

### 9.14 The verdict-attempt substitution

These four rows are rev-4's and rev-5's additions. They extend §9.2 and §9.10
and are numbered here so the matrix stays contiguous without renumbering 289
reviewed rows.

| ID | Kind | Case | Observable |
|---|---|---|---|
| ROC-290 | I | The authoritative all-present arm writes **one** verdict-bearing `phase-2` line, and it is the staged attempt | A feature that mints a proof appends **exactly one** verdict-bearing `phase-2` / `recipe-operation-match` entry to `reconcile-evidence.jsonl`, in the shipped `:766` position and ahead of every supporting entry; it carries ROC-281's seven pinned field values plus `reason_code: phase-2-operation-level`; `len(result.verdictAttempts) == 1`; the legacy `low` / `unknown` / `[]` derivation (`internal/workflow/reconcile.go:826,829,843-845`) is **absent** from that invocation; the confirmation gate grades that same entry and appends only its own separate supporting `phase-3.5` / `manual-review` line (`:1033-1035,1069-1070`); `reconcile-session.json` is byte-unchanged by the staging because `verdictAttempts` is unexported (`:775-776`); and in the same repository a **gate-off** legacy all-present run (§6.17's per-arm table row 2) still produces exactly one verdict-bearing entry per phase, **byte-identical to the S0 golden** — the S0 byte-identity assertion is scoped to that gate-off row and to nothing else. With the gate **on**, a legacy or E-gate-ineligible all-present result takes row 4's **direct-append** route: it likewise produces exactly one verdict-bearing entry per phase, but its `phase-2` line is the evaluator's directly appended refusal entry, an **intentional** evidence delta against S0 rather than a byte-identical match, and its terminal derived entry is unchanged. A fixture asserting S0 byte identity on the gate-on arm fails the row, as does one asserting the gate-on arm emits no `phase-2` line at all. Two verdict-bearing `phase-2` lines, a missing one, or a gate that appends the graded entry itself fails the row |
| ROC-291 | G | Duplicated, unpaired or smuggled verdict attempts fail the same guards | Wrong-input fixtures `legacy-phase2-entry-appended-beside-staged-attempt` (both the derived and the staged entry appended in one invocation), `staged-attempt-appended-without-substitution` (the staged list appended ahead of `:766` while `persistReconcileEvidence` still derives its own), `staged-attempt-without-proof` and `proof-without-staged-attempt` (configuration 2's biconditional broken in either direction), `proof-paired-with-candidate-ready-pair` (a proof minted beside the two-attempt list) and `verdict-attempts-assigned-outside-evaluator` (a second assignment or mutation site for `verdictAttempts`) each fail the same validators the correct configuration passes — the one-verdict-bearing-entry-per-(invocation, phase) validator for the first two, `saveReconcileArtifacts`'s three-configuration check for the next three, and §6.2's source guard for the last. In the passing configuration `verdictAttempts` has exactly one assignment site, one mutation site and one read site, and no `phase2AuthorityProof` field reaches any artifact |
| ROC-292 | I | The candidate-ready arm writes **two** ordered verdict-bearing attempts and no duplicate | A candidate-ready invocation returns `store.ReconcileStillNeeded` with `result.Phase == phase-2-operation-candidate` and `len(result.verdictAttempts) == 2`; `reconcile-evidence.jsonl` carries, in slice order at the shipped `:766` position and ahead of every supporting entry, attempt [1] `phase-2` / `recipe-operation-match` / `still_needed` / `high` / `match_origin: upstream` / `pre_reconcile_presence: present` / `upstream_commit_refs: [upstreamCommit]` / `requires_confirmation: true` / `reason_code: phase-2-operation-candidate`, then attempt [2] `phase-4` / `forward-apply` / `blocked` / `low` / `match_origin: unknown` / `pre_reconcile_presence: not-checked` / `upstream_commit_refs: []` / `reason_code: phase-4-forward-apply-conflicts` — `phase-4-blocked` when the exhausted arm (`:588-596`) finalized it — with `matched_paths` equal to `result.Conflicts` and `matched_operations` empty (`:861-862`); the derived `phase-2` entry `evidencePhaseAndKind` would have produced from `phase-2-operation-candidate` (`:1167-1168`) is **absent**; the proof is `nil` and the gate never runs, because the outcome is not `upstreamed` (`:1033-1035`); and the two `attempt_id` values differ. A third verdict-bearing line, a missing phase-4 attempt, a duplicate `phase-2` line, or a run that returns `still_needed` on both attempts fails the row |
| ROC-293 | G | Candidate-ready staging defects fail the same validators | Wrong-input fixtures `candidate-ready-derived-entry-not-suppressed` (the derived `phase-2` entry appended beside staged attempt [1]), `candidate-ready-stages-only-phase2` (attempt [2] omitted, so the terminal textual-conflict verdict has no carrier and the substitution silently drops it), `candidate-ready-attempts-out-of-phase-order` (`phase-4` staged before `phase-2`), `candidate-ready-attempts-share-attempt-id` (two staged entries with one content-addressed ID, which `internal/store/reconcile_evidence.go:166-171` would silently dedupe to one line) and `candidate-ready-phase4-attempt-carries-invocation-outcome` (attempt [2] carrying `still_needed` instead of its phase-local `blocked`) each fail the same validators the correct two-attempt configuration passes — the one-verdict-bearing-entry-per-(invocation, phase) validator for the first, `saveReconcileArtifacts`'s three-configuration check for the next three, and the phase-local-verdict validator for the last |

### 9.15 Matrix totals

| Kind | Count |
|---|---|
| `I` integration/runtime | 59 |
| `C` failure/concurrency | 85 |
| `G` semantic guard | 74 |
| `U` unit/schema | 66 |
| `S` security/privacy | 9 |
| **Total** | **293** |

**Change against rev-0's 212 rows.** Removed in rev-1: the eight byte-window
anchor rows (`anchor-not-found`, `anchor-ambiguous`, `anchor-disagreement`,
`anchor-upstream-drift`, `anchor-one-sided`, `anchor-eol-ambiguous`,
`postimage-elsewhere`, full-file expansion), the two unreachable-gate rows
(`coverage-reason-outstanding`, `effect-axis-unknown`), the zero-effect row, the
`creation-path-occupied` domain row, and the HEAD-equality row. Added in rev-1:
13 confirmation-gate rows (§9.2), 31 line-record alignment rows including both
motivating-fixture variants (§9.5), 13 lock-model rows (§9.11), the phase-4
proof-disposition rows (§9.1), the lifecycle-preservation and preflight-ordering
rows (§9.8), the phase-local-verdict rows (§9.10), the flag-count and
message-parity rows (§9.9), the lazy-fetch and constructor rows (§9.6), the
exit-envelope rows, and the path-normalization and cross-platform rows.

**Change against rev-1's 253 rows.** `280 − 253 = 27`: twenty-seven rows added,
none removed, twenty corrected rather than replaced. rev-2's own itemization
summed to twenty-nine, because it counted two **corrected** rev-1 rows as
additions; the arithmetic below is the corrected one, and it sums to
`4 + 3 + 3 + 6 + 2 + 2 + 5 + 2 = 27`:

| Added in rev-2 | Rows |
|---|---|
| `phase2AuthorityProof` carrier rows (§9.2, B5) — ROC-037 to ROC-040 | 4 |
| gate-order reachability rows (§9.3, B1), including the `combined-recipe-decode-and-owner-gate` fixture | 3 |
| the L3 memory-budget row and two alignment evaluation-order rows (§9.5, opt. 7-9) | 3 |
| feature-state and accepted-audit lifecycle rows (§9.8, B3 and opt. 6) | 6 |
| source-derived exclusivity rows (§9.9, opt. 10) | 2 |
| verdict-bearing / supporting evidence rows (§9.10, B2) | 2 |
| lock preflight, filesystem-class, dependency and cross-host rows (§9.11, B6/S2/S3, opt. 11) | 5 |
| base-metadata rows (§9.11, B4) | 2 |
| **Total** | **27** |

rev-2's twenty corrections renumbered the E-gate references (§9.3), restated
the exactly-two-attempts rows as verdict-bearing (§9.10), lowered L3 (§9.5),
widened `candidate-not-idempotent` (§9.7), added `source_feature_state` to the
identity and pointer rows (§9.8), replaced the 55-pair count with 85 (§9.9),
and extended the platform and exit-code rows (§9.11).

**Change against rev-2's 280 rows.** `289 − 280 = 9`: nine rows added
(ROC-281 … ROC-289, all in §9.13), none removed, and **sixteen** corrected in
place. The additions sum to `3 + 1 + 3 + 1 + 1 = 9`:

| Added in rev-3 | Rows |
|---|---|
| CG evidence fields and proof plumbing (R1) — ROC-281, ROC-282, ROC-283 | 3 |
| evidence taxonomy maxima and mutual exclusion (R2) — ROC-284 | 1 |
| candidate-pointer persistence (R3) — ROC-285, ROC-286, ROC-287 | 3 |
| acceptance CAS recheck and the absent cross-host claim (opt. 1) — ROC-288 | 1 |
| Cobra `help` exclusion (opt. 4) — ROC-289 | 1 |
| **Total** | **9** |

The sixteen rev-3 corrections are ROC-037 and ROC-038 (the proof's route, and
the entry it is cross-checked against), ROC-173 (the pointer's
merge-preserved persistence), ROC-194, ROC-195 and ROC-196 (`help` excluded
from the behavioral set, `--help` permitted beside a terminal), ROC-208,
ROC-211, ROC-219, ROC-220 and ROC-221 (verdict-bearing versus supporting
counts, outcome-dependent supporting sets, and diagnostics by semantic field),
ROC-245 (the CAS recheck), ROC-250 and ROC-251 (the named snapshot set), and
ROC-278 and ROC-279 (delimited parity regions, and all three SPEC rows).
ROC-174, ROC-175, ROC-186, ROC-187 and every other rev-2 row is carried forward
**unchanged** — the pointer contract is additive to them, not a restatement.

**Kind deltas for rev-3**: `I` 54 → 57 (ROC-281, ROC-284, ROC-285); `C` 83 → 85
(ROC-287, ROC-288); `G` 70 → 72 (ROC-283, ROC-286); `U` 64 → 66 (ROC-282,
ROC-289); `S` unchanged at 9.

**Change against rev-3's 289 rows.** `291 − 289 = 2`: two rows added
(ROC-290 and ROC-291, both in §9.14), none removed, and **three** corrected in
place. The additions sum to `1 + 1 = 2`:

| Added in rev-4 | Rows |
|---|---|
| the authoritative arm's single verdict-bearing `phase-2` entry, and the gate grading rather than writing it | 1 |
| the anti-duplicate, carrier-pairing and single-assignment-site guards | 1 |
| **Total** | **2** |

The three rev-4 corrections are ROC-282 (ten call sites, with `:570` named and
its `phase35` argument stated), ROC-281 (the graded entry is the substituted
`:766` entry, not a second line) and ROC-283 (the carrier boundary now also
covers the staged attempt's single assignment site). Every other row is carried
forward **unchanged**.

**Kind deltas for rev-4**: `I` 57 → 58 (ROC-290); `G` 72 → 73 (ROC-291); `C`,
`U` and `S` unchanged at 85, 66 and 9.

**Change against rev-4's 291 rows.** `293 − 291 = 2`: two rows added
(ROC-292 and ROC-293, both in §9.14), none removed, and **eleven** corrected in
place. The additions sum to `1 + 1 = 2`:

| Added in rev-5 | Rows |
|---|---|
| the candidate-ready arm's two ordered staged attempts, their exact fields and the suppressed derived entry — ROC-292 | 1 |
| the candidate-ready staging-defect fixtures: unsuppressed derivation, a missing phase-4 attempt, out-of-phase order, a shared `attempt_id` and an invocation-outcome verdict — ROC-293 | 1 |
| **Total** | **2** |

The eleven rev-5 corrections are ROC-038 and ROC-281 (the graded entry is the
single member of `result.verdictAttempts`, and the gate grades rather than
writes it), ROC-208, ROC-209, ROC-210 and ROC-211 (the two verdict-bearing
attempts, their route per arm, the candidate-ready pair's exact phase-local
values, and one-per-(invocation, phase) across two phases), ROC-221 (the staged
attempts are appended in slice order at the shipped `:766` position), ROC-283
(the permitted carrier is the unexported `verdictAttempts` list, with its
single assignment, mutation and read sites), ROC-175 (the dedicated
candidate-ready path **calls** `saveReconcileArtifacts(s, slug, result, nil)`
rather than reimplementing it, pinned by the added
`candidate-path-reimplements-evidence-persistence` wrong-input fixture),
ROC-290 (the surviving legacy `phase-2` derivation is the **gate-off** arm —
§6.17's per-arm table row 2 — while a gate-on legacy or ineligible all-present
result takes row 4's direct-append route) and ROC-291 (the staging-defect
fixture set, whose assignment-site fixture is now named
`verdict-attempts-assigned-outside-evaluator` — rev-4 called it
`phase2-verdict-assigned-outside-evaluator`, after the singular field this
revision replaced — and which gains `proof-paired-with-candidate-ready-pair`
for the biconditional re-scoped to configuration 2). ROC-282, ROC-284 …
ROC-289 and every other row are carried forward **unchanged**.

**Kind deltas for rev-5**: `I` 58 → 59 (ROC-292); `G` 73 → 74 (ROC-293); `C`,
`U` and `S` unchanged at 85, 66 and 9.

**rev-6 adds no row and removes none.** The matrix stays at **293** contiguous
rows — `I` 59, `C` 85, `G` 74, `U` 66, `S` 9 — because every rev-6 change is a
correction or a tightening of a row that already exists. Three rows are
corrected: **ROC-282** (the `:570` argument is the variable `phase35` bound to
the **same** `*ReconcileResult` object as `result`, since `tryPhase35` mutates
and returns the pointer it receives, so `:570` differs from the other nine call
sites in spelling only, and a fixture asserting a second distinct result object
fails), **ROC-290** (the S0 byte-identity assertion is scoped to the gate-off
per-arm row 2, while the gate-on direct-append arm carries an intentional
phase-2 evidence delta and still emits one verdict-bearing entry per phase) and
**ROC-281** (the legacy `low` / `unknown` / `[]` entry and its S0 byte identity
are scoped to `operation_candidate_enabled` **off**). One row is **amended
rather than duplicated**: **ROC-208** now also pins the dual-sink invariant —
every verdict-bearing attempt reaches `reconcile-evidence.jsonl` and
`result.Evidence` in the same relative order on **both** routes, so
`reconcile-session.json`'s `evidence` array (`internal/workflow/reconcile.go:775-776`)
is never a strict subset of the JSONL. ROC-208 already discriminates the staged
and direct-append routes, so no new row is required and no ID is renumbered.
Every other row is carried forward **unchanged**.

**Kind deltas for rev-6**: none. `I` 59, `C` 85, `G` 74, `U` 66, `S` 9 — all
unchanged.

**Every added `G` row names a wrong-input fixture
and the production validator it must fail**, which is the standard §9's
preamble sets.

## 10. Rollout and release

- **Target `v0.18.0`**, separate from GH #15's `v0.17.0`. ADR-036 D12 already
  separated them and this PRD does not reopen that.
- **Implementation is blocked until v0.17.0 ships.** Today `internal/` and
  `cmd/` contain **zero** matches for `recipe-coverage.json`, `recipe_coverage`,
  `CoverageProducer`, `RecipeCoverage` or `reconcile-accept`. There is nothing
  to consume.
- **Slices run sequentially** (§8). Every slice touches
  `internal/workflow/reconcile.go` and most touch `internal/cli/cobra.go`;
  `AGENTS.md`'s Parallel-Implementer Discipline makes same-file overlap a hard
  trigger for sequential execution, and staging is explicit-path only.
- **S0 freezes the baseline before any production edit.** Multi-slug,
  `--cumulative-legacy`, resolver accept/reject/shadow-diff, the shipped
  `validateReconcileFlags` message bytes and the no-candidate
  `ReconcileSummary` serialization must stay byte-identical through the entire
  wave.
- **`operation_candidate_enabled` ships `false`.** With the gate off, every
  reconcile path — including the legacy all-present arm the feature retires
  (§6.4) — is byte-identical to v0.17.0. The one expectation that changes with
  the gate **on** is named in §6.4 and carried as a second, explicit
  expectation in S0/S7 rather than an edit to the existing one.
- **Default posture is unchanged behavior even with the gate on.** A repository
  with no ADR-036 coverage — which is every repository until v0.17.0 lands and
  producers run — refuses at E1 and falls through, producing today's outcomes.
- **Unsupported platforms degrade to today's behavior.** Runtime support is
  `linux`, `darwin` and `windows` only. On every other target — including every
  BSD — the preflight and candidate finalization record `lock-unsupported` and
  fall through (§6.13), and on a supported target whose `.tpatch/locks`
  directory is on a network or unrecognized filesystem they record
  `lock-filesystem-unsupported` the same way. Nothing else changes on those
  targets, and no BSD claim is made on the strength of a successful
  cross-compilation.
- **Soak** on the frozen fixtures plus both adjacent-CLI-args variants before
  release authorization. **Flipping the default is a separate decision** with
  its own soak evidence, taken no earlier than the release after v0.18.0.

## 11. Deferred decisions and review triggers

Each row is a real deferral with a stated v1 answer.

| Deferred | v1 answer | Reopen when |
|---|---|---|
| Replacement (`-` line) effects | Refused; `effect-replacement-unsupported` | An accepted commutation theory can prove a replacement's context survived |
| Delete / rename / copy effects | Refused; `effect-change-kind-unsupported` | A dedicated ADR defines their cross-base semantics |
| Binary effects | Refused; `effect-content-kind-unsupported` | A binary-delta alignment model is accepted |
| Executable / symlink / gitlink / mode changes | Refused; `write-file` is fixed `0o644` (`internal/workflow/recipe.go:207-211`) | The recipe schema gains a mode field on GH #15's surface |
| Non-unanimous alignments | Refused; `alignment-ambiguous` | A tie-breaking rule can be shown to be safe under review, which this PRD does not believe is possible without a knob |
| Alignments beyond L1-L3 | Refused; `alignment-limit`, with L3 at `1048576` cells / 8 MiB of matrices | A sub-quadratic exact all-alignment decision procedure is accepted, or a streaming formulation removes the two-matrix requirement; the limits are never raised by configuration |
| Persisted alignments and record indices | Ephemeral only; digests are diagnostic | A consumer needs persistence **and** the §6.8 idempotency proof has shipped and soaked |
| Multi-slug candidates | Refused; single-slug only | Stack-aware sequential candidate derivation is designed |
| `--cumulative-legacy` candidates | Refused; ADR-030 preserved | Never without retiring cumulative derivation |
| Preview test execution during generation | None; acceptance only | An explicit opt-in flag is specified with its own security review |
| More than one pending candidate per slug | One; older marked `stale` | Operators demonstrably need to compare candidates across upstreams |
| Candidate + resolver shadow coexistence | Refused with named remediation | A merge semantics for two staged answers is designed |
| Filesystem-wide atomic acceptance | Journal + rollback + explicit recovery | A content-addressed staging store makes single-rename publication possible |
| Automatic crash recovery | Explicit `--resume-candidate-recovery` | Recovery is proved total across every journal step |
| Candidate generation changing lifecycle state | No state change; `omitempty` pointer only | A candidate must gate other commands, which a pointer cannot express |
| Flipping `operation_candidate_enabled` on by default | `false` in v0.18.0 | The v0.18.0 soak shows no regression and the retired legacy arm's behavior change is accepted on its own evidence |
| A `phase-2.5` evidence value | None; `phase-2` reused | Never — it would misdescribe the architecture |
| Widening the confirmation gate beyond CG1-CG7 | One authority class, seven conditions | A second deterministic proof of equal strength is designed; confidence alone never qualifies |
| Provider assistance inside phase 2 | Forbidden | Never; that is phase 3/3.5 |
| Advisory locking on the BSDs and every other unlisted platform | `Supported == false`, named `lock-unsupported` refusal; v1 runtime support is linux, darwin and windows only | A **native runtime CI job** exists for the target **and** the primitive is verified on it. A successful cross-compilation is explicitly not the trigger |
| Advisory locking on network filesystems | Refused; `lock-filesystem-unsupported`, known-local allowlist | A cross-host mutual-exclusion protocol is designed and reviewed; until then the per-path CAS and the journal are the authority, and the lock claims nothing across hosts |
| A writer outside `tpatch` mutating a candidate path between the step-10 CAS recheck and the write | **Not prevented, and not claimed to be.** The recheck minimizes the window; the journal and rollback make the outcome recoverable | The platform offers a usable multi-file transactional primitive, or a content-addressed staging store makes single-rename publication possible |
| Feature states beyond {`applied`, `blocked`} | Refused; `candidate-feature-state-unsupported` (§6.6a) | A state outside the pair is shown to have realized changes worth reproducing, with its own acceptance semantics |
| Regenerating coverage for P6 (`implement`) and P7 (`artifact-edit`) records | Untouched; such records are `incomplete`, so E3 refuses and the feature falls through | The regeneration contract GH #15's PRD `:3674-3678` assigns to GH #13 is designed; GH #13 v1 regenerates only as a by-product of a successful operation-candidate accept |
| Regenerating coverage for features GH #15 left incomplete | Out of scope here; those features are simply ineligible | A separate regeneration contract is accepted (ADR-036 §13 names GH #13 as a candidate owner) |
| Recipe/coverage regeneration for `cycle` (P4) and `apply --mode done` (P5) | Untouched; ADR-036's deferral row stays open for both | A publication path for those producers is designed; GH #13 discharges only the `operation-candidate-accept` variant of P3 |
| Recipe regeneration on the `resolver-accept` path | Unchanged; still `incomplete` with `producer-patch-rewrite` + `recipe-not-regenerated` | `RefreshAfterAccept` gains a recipe regenerator, which is a GH #15-surface change |
| `ensure-directory` in the candidate domain | Not modeled; a recipe carrying one cannot reach a `complete` ADR-036 record, so the feature refuses at E3 | A creation genuinely needs an empty directory **and** ADR-036 gains a representation for it |
| Committing on the operator's behalf | Never | Never |

## 12. Claims-audit appendix

Every load-bearing claim about current behavior, with a `file:line` anchor
re-verified against the working tree at `e76e0f7`. rev-2 re-verified every row
and corrected the eleven marked **(rev-2)**; rev-3 re-verified every row again,
corrected the two detector-guard ranges it had wrong, and added the nine marked
**(rev-3)** — the plumbing, builder-default and detector-guard anchors the three
rev-2 findings turn on. rev-4 re-verified every row a third time, **corrected
the call-site row from nine sites to ten**, and added the three marked
**(rev-4)** — the staging-field precedent, the phase derivation and the
phase-3.5 call's separately named argument, which are the anchors F1 and opt. 1
turn on. rev-5 re-verified every row a fourth time, changed none, and added the
six marked **(rev-5)** — the shipped `forward-apply` confidence rule, the entry
literal's field sources, the content-addressed `attempt_id`, the single-entry
return, and the two `result.Phase` values the substitution turns on. rev-6
re-verified every row a fifth time and **rewrote one**: the phase-3.5
argument row, which rev-4 had recorded as a distinct `*ReconcileResult`, now
records the shipped truth — `tryPhase35` takes the pipeline's `result` as its
ninth parameter, mutates it in place and returns that same pointer from all ten
of its returns, so `phase35` is a distinct **variable name** bound to the same
object. Every other row is unchanged and no row is added.

| Claim | Source |
|---|---|
| `reconcileFeature` is the per-feature phase machine | `internal/workflow/reconcile.go:315` |
| `RunReconcile` is the multi-slug driver | `internal/workflow/reconcile.go:129` |
| Phase order is viability, 1, 1.5, 2, 3, 4 with 3.5 nested | `internal/workflow/reconcile.go:358,377-386,388-443,445-464,466-488,490-586,569` |
| Phase 2 is called with the live root | `internal/workflow/reconcile.go:450` |
| Phase 2 reads the worktree with `os.ReadFile` | `internal/workflow/reconcile.go:617-618,635-636` |
| `replace-in-file` presence is a substring check | `internal/workflow/reconcile.go:625` |
| `write-file` presence is trimmed equality | `internal/workflow/reconcile.go:637` |
| `ensure-directory` is counted present unconditionally | `internal/workflow/reconcile.go:646-647` **(rev-2)** |
| Unknown operation types are counted **applicable** | `internal/workflow/reconcile.go:649-650` **(rev-2)** |
| `allPresent` is the only phase-2 outcome signal | `internal/workflow/reconcile.go:654` |
| The all-present arm returns `ReconcileUpstreamed` and returns | `internal/workflow/reconcile.go:451-457` |
| The conflict arm appends a note and falls through | `internal/workflow/reconcile.go:459-462` |
| `operationEvalResult` carries counts only | `internal/workflow/reconcile.go:603-609` |
| Phase 3's provider `upstreamed` arm returns terminally | `internal/workflow/reconcile.go:477-483` |
| Phase 4 previews the forward apply from the live root | `internal/workflow/reconcile.go:496` |
| `promoteIfMarkers` promotes a clean verdict to blocked | `internal/workflow/reconcile.go:505-517` |
| The strict and 3-way-clean arms return `reapplied` | `internal/workflow/reconcile.go:519-538` |
| The 3-way-conflict arm branches on `opts.Resolve` | `internal/workflow/reconcile.go:539-586` |
| The blocked-by-parent short-circuit precedes the resolver | `internal/workflow/reconcile.go:552-567` |
| The non-`--resolve` conflict arm returns `blocked` | `internal/workflow/reconcile.go:574-585` |
| The exhausted arm returns `phase-4-blocked` | `internal/workflow/reconcile.go:588-596` |
| SPEC promises `UPSTREAMED` / `BLOCKED` / `STILL-NEEDED` at phase 2 | `SPEC.md:444-448` |
| `ReconcileOutcome` constants and their JSON values | `internal/store/types.go:59-74` |
| The twelve `FeatureState` constants, with `StateApplied` and `StateBlocked` the two this feature permits | `internal/store/types.go:8-37`, `StateApplied` `:13`, `StateBlocked` `:17`, `StateReconcilingShadow` `:16` **(rev-2)** |
| `ApplySummary` carries `CompletedAt`, `BaseCommit`, `HasPatch`, `HasRecipe` | `internal/store/types.go:362-369`, `BaseCommit` `:366` |
| The confirmation gate reads `status.Apply.BaseCommit` for its evidence stamp | `internal/workflow/reconcile.go:1037` |
| `persistReconcileEvidence` reads it too | `internal/workflow/reconcile.go:821` |
| The file-novelty detector reads it | `internal/workflow/reconcile.go:888,899,903` |
| The hunk-overlap detector reads it | `internal/workflow/reconcile.go:917,924,931,935` |
| The path-restructure detector reads it | `internal/workflow/reconcile.go:951,965,978` |
| The blocked-classification taxonomy reads it | `internal/workflow/reconcile.go:1005` |
| The gate's own evidence entry is `phase-3.5` / `manual-review` | `internal/workflow/reconcile.go:1069-1070` |
| `applyUpstreamedConfirmationGate` takes `(s, slug, result)` today and returns evidence | `internal/workflow/reconcile.go:1032` |
| It is called from inside `saveReconcileArtifacts`, not from the pipeline | `internal/workflow/reconcile.go:724`, call at `:771` **(rev-3)** |
| `saveReconcileArtifacts` has exactly **ten** call sites; nine pass `(s, slug, result)` and `:570` passes `(s, slug, phase35)`, a different variable name bound to the same object | `internal/workflow/reconcile.go:383,432,455,480,524,536,564,570,583,596`, `phase35` produced at `:569` **(rev-4; rev-3 listed nine and omitted `:570`; "same object" added in rev-6)** |
| `ReconcileResult` is serialized whole into `reconcile-session.json` | `internal/workflow/reconcile.go:775-776` **(rev-3)** |
| `ReconcileResult` already carries an **unexported** staging field, `attemptedAt`, which `encoding/json` ignores and which is shared between `saveReconcileArtifacts` and `updateFeatureState` | `internal/workflow/reconcile.go:19-85`, `attemptedAt` `:74-84` **(rev-4)** |
| `tryPhase35` receives the pipeline's `result` as its **ninth parameter**, mutates it in place and returns **that same pointer** from every one of its ten returns — each literally `return result` — so the phase-3.5 arm's `phase35` is a distinct **variable name** bound to the **same** `*ReconcileResult` object, not a distinct object, and `:570` differs from the other nine call sites in argument spelling only | `internal/workflow/reconcile.go:1354-1364` (parameter `:1363`), returns `:1373,1381,1389,1436,1475,1491,1497,1502,1508,1513`, bound and passed at `:569-571` **(rev-6; rev-4 wrongly described `phase35` as a distinct result)** |
| `evidencePhaseAndKind` derives one phase from `result.Phase`, mapping a `phase-2` prefix to `phase-2` / `recipe-operation-match` | `internal/workflow/reconcile.go:1161-1176`, `phase-2` arm `:1167-1168` **(rev-4)** |
| The evidence builder's defaults are `unknown` origin, `not-checked` presence, `requires_confirmation: true` and an empty ref list | `internal/workflow/reconcile.go:826,827,828,829` **(rev-3)** |
| The `recipe-operation-match` arm sets only `confidence: low` and `presence: present` | `internal/workflow/reconcile.go:843-845` **(rev-3)** |
| The `forward-apply` arm lowers confidence to `low` only when the outcome is `blocked`, and otherwise leaves the `medium` default | `internal/workflow/reconcile.go:825,846-849` **(rev-5)** |
| The entry literal takes `raw_reconcile_verdict` from `result.Outcome`, `matched_paths` from `result.Conflicts`, `matched_operations` as empty, and `reason_code` from `result.Phase` | `internal/workflow/reconcile.go:851-868`, verdict `:857`, paths `:861`, operations `:862`, reason `:867` **(rev-5)** |
| `attempt_id` is content-addressed over the finished entry, so two entries differing anywhere get different IDs | `internal/workflow/reconcile.go:875`, `internal/store/reconcile_evidence.go:120,166-171` **(rev-5)** |
| The store validator requires `schema_version == 1`, a `feature_slug` equal to the reconciled slug, a non-empty `attempt_id`, a non-empty `upstream_ref` / `upstream_commit` / `base_commit` / `raw_reconcile_verdict` / `reason_code`, and non-nil `matched_paths` / `matched_operations` / `upstream_commit_refs`; `AppendReconcileEvidence` re-stamps `feature_slug` before validating | `internal/store/reconcile_evidence.go:343-357`, re-stamp `:144` **(rev-5)** |
| `persistReconcileEvidence` appends exactly one entry and returns a one-element slice | `internal/workflow/reconcile.go:876-880` **(rev-5)** |
| The shipped all-present arm sets `result.Phase = "phase-2-operation-level"`, which the `phase-2` prefix branch maps to `phase-2` | `internal/workflow/reconcile.go:453`, branch `:1167-1168` **(rev-5)** |
| The non-`--resolve` conflict arm and the exhausted arm set `phase-4-forward-apply-conflicts` and `phase-4-blocked` respectively, which become the entry's `reason_code` | `internal/workflow/reconcile.go:575,590`, via `:867` **(rev-5)** |
| `persistReconcileEvidence` fires on any non-empty outcome | `internal/workflow/reconcile.go:813-815` **(rev-3)** |
| The file-novelty guard needs an outcome, an upstream commit, a base commit and a canonical patch | `internal/workflow/reconcile.go:884-898` **(rev-3)** |
| The hunk-overlap guard adds a modifying / mixed-additive novelty classification | `internal/workflow/reconcile.go:913-923,928-930` **(rev-3)** |
| The path-restructure detector fires **only** on blocked outcomes | `internal/workflow/reconcile.go:947-949` **(rev-3)** |
| The blocked-classification taxonomy fires **only** on blocked outcomes | `internal/workflow/reconcile.go:998-1000` **(rev-3)** |
| The confirmation gate fires **only** on `upstreamed` outcomes | `internal/workflow/reconcile.go:1033-1035` **(rev-3)** |
| `updateFeatureState` replaces `status.Reconcile` with a fresh literal rather than merging | `internal/workflow/reconcile.go:1191-1209` **(rev-3)** |
| `patch-generations.json` and `recipe-provenance.json` live under the feature's `artifacts/` directory | `internal/store/patch_generations.go:22,86-87`, `internal/cli/cobra.go:1294` **(rev-3)** |
| The file-novelty entry is `phase-3.5` | `internal/workflow/file_novelty.go:115-116` |
| The hunk-overlap entry is `phase-3.5` | `internal/workflow/hunk_overlap.go:135-136` |
| The path-restructure entry is `phase-3.5` | `internal/workflow/path_restructure.go:118-119` |
| The blocked-classification entry is `phase-4` | `internal/workflow/blocked_taxonomy.go:179` |
| `AppendPatchGenerationForFeature` is the shared ADR-024 appender taking `BaseCommit`, `Upper` and `Capture` | `internal/workflow/patch_generations.go:31`, `internal/store/patch_generations.go:30-67` |
| The root command's only persistent flag is `--path` | `internal/cli/cobra.go:73` |
| Reconcile's presentation flag is `--format`, and there is no `--json` on the command | `internal/cli/cobra.go:2543` |
| `rescap`'s statfs refusal code is `ReasonResourceLockFSUnsupported` | `internal/rescap/statfs_linux.go:78-95` |
| `go.mod` requires only cobra, with pflag and mousetrap indirect | `go.mod` |
| `ReconcileSummary` carries `ShadowPath` / `ResolveSession` | `internal/store/types.go:372,381-382` |
| `ReconcileOptions.Resolve` gates phase 3.5 | `internal/workflow/reconcile.go:89-93` |
| The confirmation gate runs on every `upstreamed` result | `internal/workflow/reconcile.go:771,1032-1035` |
| It confirms on `patch-id-match` with a `matched_upstream_sha` | `internal/workflow/reconcile.go:1047-1054` |
| It confirms on high-confidence `reverse-apply` | `internal/workflow/reconcile.go:1055-1060` |
| An unconfirmed `upstreamed` is overwritten with `blocked` | `internal/workflow/reconcile.go:1089-1090` **(rev-2)** |
| The gate's reason code when no upstream ref exists | `internal/workflow/reconcile.go:1042` |
| A rejected candidate persists `blocked` in the revision log | `internal/workflow/reconcile.go:1096-1112` |
| The phase-2 operation-match demotion is pinned by a test | `internal/workflow/reconcile_evidence_integration_test.go:459-485` |
| Its all-present fixture builder | `internal/workflow/reconcile_evidence_integration_test.go:235` |
| The phase-1 confirmation arm is pinned by a test | `internal/workflow/reconcile_evidence_integration_test.go:413` |
| The phase-3 rejection arm is pinned by a test | `internal/workflow/reconcile_test.go:304,354-359` |
| The confirmation gate's motivating data loss | `docs/prds/PRD-upstreamed-confirmation-gate.md:61-66` |
| Its goal of a cheap deterministic confirmation | `docs/prds/PRD-upstreamed-confirmation-gate.md:79-80` |
| `updateFeatureState` writes the summary and the state | `internal/workflow/reconcile.go:1181-1238` |
| Its `still_needed` arm forces `StateApplied` and a note | `internal/workflow/reconcile.go:1220-1222` |
| `ExecuteRecipe` collects per-op errors and continues | `internal/workflow/recipe.go:90,102-110` |
| `dryRunOperation` hardcodes `s.Root` | `internal/workflow/recipe.go:128-129` |
| `executeOperation` hardcodes `s.Root` | `internal/workflow/recipe.go:199-200` |
| `write-file` execution uses a fixed `0o644` mode | `internal/workflow/recipe.go:207-211` |
| Replace is first-match and append is unconditional | `internal/workflow/recipe.go:213-242` |
| Both operation switches fail closed on unknown types | `internal/workflow/recipe.go:194-195,242-243` **(rev-2)** |
| `RecipeOperation` has four types and `CreatedBy` | `internal/workflow/implement.go:49-50` |
| `AcceptShadow` is the resolver accept transition | `internal/workflow/accept.go:82` |
| Step 1 re-applies the original patch minus resolved paths | `internal/workflow/accept.go:95` |
| Step 2 copies shadow files onto the real tree | `internal/workflow/accept.go:101` |
| Refresh and prune failures are treated as non-fatal | `internal/workflow/accept.go:110,128-131` |
| Accept marks the feature applied | `internal/workflow/accept.go:123` |
| `CopyShadowToReal` defers atomicity to the caller | `internal/gitutil/shadow.go:189-191` |
| `RefreshAfterAccept` writes `post-apply.patch` unconditionally | `internal/workflow/refresh.go:82` |
| It deliberately leaves `apply-recipe.json` stale | `internal/workflow/refresh.go:20-24` |
| Its generation append is conditional on changed bytes | `internal/workflow/refresh.go:93,102` |
| The shadow directory constant | `internal/gitutil/shadow.go:35` |
| `CreateShadow` reaps prior shadows on create | `internal/gitutil/shadow.go:56,72` |
| Shadow worktrees are created detached at a commit | `internal/gitutil/shadow.go:98` |
| `ResolveShadow` is newest-wins | `internal/gitutil/shadow.go:115` |
| `PruneAllShadows` clears every shadow for a slug | `internal/gitutil/shadow.go:152` |
| Shadow paths are `filepath.ToSlash`-normalized | `internal/gitutil/shadow.go:206,248` |
| `pruneShadowPath` removes the worktree then the directory | `internal/gitutil/shadow.go:309` |
| Reconcile flag definitions: sixteen flags, three of them terminal | `internal/cli/cobra.go:2520-2543` **(rev-2)** |
| `--allow-dirty` concedes verdicts may be wrong | `internal/cli/cobra.go:2523` |
| `--allow-stale-lock` bypasses the baseline guard | `internal/cli/cobra.go:2524` |
| `--check-applied-only` and `--auto-drop-merged` are mutually exclusive | `internal/cli/cobra.go:2316-2325,2534-2535` |
| `--cumulative-legacy` opts into ADR-030 D2 behavior | `internal/cli/cobra.go:2542` **(rev-2)** |
| `reconcile review` is a registered subcommand | `internal/cli/cobra.go:2544` |
| `validateReconcileFlags` enforces three-terminal exclusivity | `internal/cli/cobra.go:3200`, rules `:3211-3218`, message strings `:3212,3215,3218` **(rev-2)** |
| Multi-slug args are dispatched as a slug list | `internal/cli/cobra.go:2440` |
| `test_command` is set through `tpatch config set` | `internal/cli/cobra.go:3731-3732` |
| `ValidationConfig.TestCommand` is the consumed field | `internal/workflow/validation.go:60` |
| `RunTestCommandInShadow` runs the gate in a shadow | `internal/workflow/validation.go:273` |
| The resolver gates on it at ADR-010 D4 gate 4 | `internal/workflow/resolver.go:204-210` |
| Evidence is appended by six persist calls per invocation | `internal/workflow/reconcile.go:766-771` |
| `evidencePhaseAndKind` derives one phase from `result.Phase` | `internal/workflow/reconcile.go:1161-1176` **(rev-2)** |
| `phase-2` maps to `recipe-operation-match` | `internal/workflow/reconcile.go:1167-1168`, `internal/store/reconcile_evidence.go:41` |
| `raw_reconcile_verdict` is an unconstrained string field | `internal/store/reconcile_evidence.go:96` |
| `match_origin` closed values | `internal/store/reconcile_evidence.go:61-68` |
| `pre_reconcile_presence` closed values | `internal/store/reconcile_evidence.go:70-77` |
| Evidence is append-only JSONL | `internal/store/reconcile_evidence.go:17,116,137` |
| Identical `attempt_id` with identical bytes is a no-op | `internal/store/reconcile_evidence.go:166-170` |
| Identical `attempt_id` with different bytes is malformed | `internal/store/reconcile_evidence.go:171` |
| `attempt_id` is content-addressed | `internal/store/reconcile_evidence.go:120`, `docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:112-122` |
| ADR-025 D1 frames one line as one attempt for one verdict | `docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:70-72` |
| ADR-025 D4 closes the phase and kind enums | `docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md:134-146` |
| ADR-017's guard is a baseline-staleness classifier, not a mutex | `internal/gitutil/lock_guard.go:18-23,99-100` |
| `rescap.AcquireLock` is a nonblocking `flock` with no PID protocol | `internal/rescap/lock_unix.go:63,80-87` |
| It runs a `statfs` filesystem preflight first | `internal/rescap/lock_unix.go:64-70` |
| Its callers must first pass `EnsureLocalContract` | `internal/rescap/scratch.go:46-48`, `internal/cli/feature_resource.go:107` |
| It refuses on every non-linux/darwin target | `internal/rescap/lock_unsupported.go:1,15,29-31` |
| `GIT_NO_LAZY_FETCH=1` is a shipped constant | `internal/gitutil/trailers.go:41` |
| Its environment is asserted by a PATH-wrapper guard | `internal/gitutil/apply_classifier_test.go:319-320`, `internal/cli/land_rev1_fold_test.go:236-237` |
| `EnsureSafeRepoPath` validates containment against one root | `internal/safety/safety.go:12` |
| `FileAtCommit` reads a blob without checkout | `internal/gitutil/gitutil.go:803` |
| `BlobAtTree` reads a blob and reports found-ness | `internal/gitutil/trailers.go:737` |
| `ResolveRef` and `RevParse` resolve refs to commits | `internal/gitutil/gitutil.go:789,854` |
| A preserved drifted recipe writes a stale marker | `internal/workflow/recipe_autogen.go:184-198` |
| Feature artifacts live under `<feature>/artifacts/` | `internal/store/store.go:787-789` |
| Exit `2` is pre-mutation validation, exit `3` a state refusal, exit `1` an internal error | `internal/cli/reject.go:36-47` |
| ADR-036 D3 is the canonical coverage schema | `docs/adrs/ADR-036-recipe-coverage-authority.md:464-543` **(rev-2)** |
| ADR-036 D3's ten completeness predicates | `docs/adrs/ADR-036-recipe-coverage-authority.md:1038-1059` **(rev-2)** |
| ADR-036 D3's contradictory-record decode refusals | `docs/adrs/ADR-036-recipe-coverage-authority.md:1065-1076` **(rev-2)** |
| ADR-036 D4 states completeness is not replay eligibility | `docs/adrs/ADR-036-recipe-coverage-authority.md:1227-1268` |
| `operation-not-reclassifiable` covers `append-file` and `replace-in-file` | `docs/adrs/ADR-036-recipe-coverage-authority.md:1141` |
| ADR-036 D8 persists no anchor and defers to GH #13 | `docs/adrs/ADR-036-recipe-coverage-authority.md:1518-1541` |
| ADR-036 D8 names the adjacent-args case as the proof they differ | `docs/adrs/ADR-036-recipe-coverage-authority.md:1547-1558` |
| ADR-036 D15 P3 is `reconcile-accept` with incomplete coverage | `docs/adrs/ADR-036-recipe-coverage-authority.md:1962` |
| ADR-036 defers recipe regeneration for `reconcile-accept` to GH #13 | `docs/adrs/ADR-036-recipe-coverage-authority.md:3048` |
| GH #15's PRD enumerates the consumer boundary | `docs/prds/PRD-recipe-generation-authority.md:2098-2145` |
| GH #15's PRD §6.14 owns the ten-row consumer table, **seven** of whose rows are warning-class with exit `0` | `docs/prds/PRD-recipe-generation-authority.md:2098`, table `:2140-2151` **(rev-2)** |
| ADR-036 D13 is the **six-rung** verify severity ladder, not that table | `docs/adrs/ADR-036-recipe-coverage-authority.md:1745`, ladder `:1755-1766` **(rev-2)** |
| GH #15 assigns GH #13 regeneration ownership for P3, P4, P5, P6 and P7 | `docs/prds/PRD-recipe-generation-authority.md:3674-3678` **(rev-2)** |
| ADR-036 D15's seven producers P1-P7 | `docs/adrs/ADR-036-recipe-coverage-authority.md:1958-1966` **(rev-2)** |
| GH #15's PRD lists what GH #13 must not assume | `docs/prds/PRD-recipe-generation-authority.md:3645-3680` |
| ADR-029 D7 downgrades superseded drift to warning-class | `docs/adrs/ADR-029-write-file-recipe-safety.md:74-76` |
| ADR-028 excludes superseded features from default replay | `docs/adrs/ADR-028-supersession-edge-model.md:77-88` |
| ADR-030 D2/D6/D7 define `--cumulative-legacy` semantics | `docs/adrs/ADR-030-multi-slug-reconcile-derivation-mode.md:35-42,65-76` |
| ADR-010 D2 makes the shadow worktree the unit of atomicity | `docs/adrs/ADR-010-provider-conflict-resolver.md:31-40` |
| The adjacent-args case study's correct resolutions | `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md` §2 |
| No coverage implementation exists in the tree | zero matches for `recipe-coverage.json` / `recipe_coverage` / `CoverageProducer` / `reconcile-accept` under `internal/` and `cmd/` |
| No Git tree-hash helper exists | no `WriteTree` / `mktree` / `hash-object` wrapper under `internal/gitutil/` |
| No per-feature reconcile mutex exists | no lock acquisition in the reconcile `RunE` (`internal/cli/cobra.go:2303-2519`) |
| No `LockFileEx` binding exists | zero matches for `LockFileEx` under `internal/` |
| No `golang.org/x/sys` dependency exists | zero matches for `golang.org/x/sys` and `NewLazyDLL` under `internal/` and in `go.mod` |

## 13. Context for the implementation wave

The implementer inheriting this PRD must not assume:

- that ADR-036 coverage exists in code — it does not, and slice S0 runs against
  a tree where E1 refuses for every feature;
- that the all-present arm is terminal today — the confirmation gate demotes it
  (§2.7), and only CG1-CG7 changes that;
- that `confidence: high` on an operation match is a confirmation;
- that the confirmation facts can travel through anything except the typed
  in-process `phase2AuthorityProof` parameter, threaded evaluator →
  `saveReconcileArtifacts` → gate — not through `ReconcileResult`, not through
  evidence, not through `status.json`, not through a package variable and not
  through `confidence` (§6.2);
- that CG5-CG7 grade any entry other than the **new** GH #13 phase-2
  verdict-bearing entry, or that the shipped legacy builder may be widened to
  emit that entry's field values (§6.2);
- that one reconcile invocation writes at most one evidence entry per phase —
  it already writes up to four `phase-3.5` lines (one verdict-bearing plus
  three supporting) and up to two `phase-4` lines (one plus one), and only
  **verdict-bearing** entries are one-per-phase (§6.17);
- that the confirmation gate's supporting entry can coexist with the
  blocked-only path-restructure or blocked-classification entries, or that a
  candidate-ready outcome emits either of them (§6.17);
- that diagnostic parity means an equal number of detector evidence lines — it
  is measured in semantic fields (§6.17);
- that a supporting detector or gate entry may be read as a phase verdict;
- that the gate's own `phase-3.5` / `manual-review` entry becomes a phase-2
  entry, or carries phase-2 authority;
- that a feature in any state may get a candidate — only `applied` and
  `blocked` may, and the observed state is bound into the ID (§6.6a);
- that acceptance may proceed when the feature's state moved since derivation;
- that acceptance may leave `status.Apply.BaseCommit` pointing at the old base,
  or may publish a generation entry whose `base_commit` differs from it (§6.14);
- that an `accepted` candidate directory may be reused, re-validated, rejected,
  pruned or marked stale (§6.10);
- that the staleness preflight, a prune, a pointer write or a rejection append
  may happen without the per-feature lock (§6.11);
- that a BSD build's successful compilation means the lock works there (§6.13);
- that an advisory lock alone makes acceptance safe across hosts, or that the
  step-10 CAS recheck prevents an external or cross-host writer — it shrinks the
  window, and the journal plus rollback are the recovery answer (§6.14);
- that the step-6 CAS may stand in for the step-10 recheck;
- that `golang.org/x/sys` may be added for `LockFileEx`;
- that E1-E15 may be reordered, or that E7 may be evaluated for a recipe E6
  refused;
- that a terminal candidate action may accept `--upstream-ref` or any other
  reconcile-local behavioral flag, or that the exclusivity set may be a
  hardcoded list of flag names (§6.12);
- that a second-pass refusal other than a nonzero count is anything but
  `candidate-not-idempotent` (§6.8);
- that GH #13 v1 repairs P4, P5, P6 or P7 coverage — it does not, and it
  regenerates only on a successful operation-candidate accept (§6.16);
- that `coverage_status: complete`, `producer`, `cross_base_status` or a
  patch-generation ID carries authority;
- that a verify `warn` with exit `0` is a grant;
- that phase-2 `BLOCKED` may return — it may not, and a terminal phase-2 blocked
  arm is a regression against the shipped fallthrough;
- that a candidate may be persisted at phase 2 — persistence is gated on an
  actual phase-4 textual conflict with `--resolve` unset (§6.2);
- that a candidate may preempt a clean phase-4 reapply or a phase-3 provider
  answer;
- that the persisted recipe's `write-file` body may be executed cross-base;
- that a byte-window, a context window, a fuzz factor, a similarity score or a
  configurable limit may appear anywhere in the alignment;
- that one optimal alignment may be selected when several exist;
- that `evaluateRecipeOperations`, `ExecuteRecipe`, `dryRunOperation` or
  `executeOperation` may be reused, extended with a root parameter, or
  "fixed in place";
- that `rescap.AcquireLock`, `rescap.EnsureLocalContract` or a PID-based lock
  protocol may be used for reconcile;
- that a lock may be broken because it looks stale;
- that `AcceptShadow`, `CopyShadowToReal` or `RefreshAfterAccept` may be reused
  for candidate acceptance;
- that acceptance may require `HEAD == upstream_commit`, or that a passing
  ancestry check substitutes for the per-path CAS;
- that `.tpatch/shadow` may host a candidate;
- that `updateFeatureState` may be reused for a candidate-ready result;
- that any writer of `status.Reconcile` may replace the summary without
  carrying the loaded `operation_candidate` pointer forward first, or that a
  pointer may be repaired after the write rather than merged before it (§6.11);
- that any path other than finalize, stale, reject, accept and recovery may set,
  modify or clear that pointer;
- that a candidate ID may include an execution result, a status, or anything
  unknown before the worktree exists;
- that a stored proof hash may substitute for re-derivation at accept time;
- that an alignment, a record index or an offset may be persisted "just for
  diagnostics";
- that a candidate may be partial when one effect is unsupported;
- that a test command may run during generation;
- that acceptance may be filesystem-atomic, or may be described as such;
- that a completed rollback is an internal error;
- that a coverage-publication failure may be printed as a warning beside a
  success line;
- that existing flag combinations may be re-routed through the new validator,
  or that their message bytes may change;
- that `SPEC.md`, the skill assets or any public surface may be edited before
  slice S9.

Implementation starts only after this PRD and ADR-037 rev-6 are accepted
**and** GH #15 has shipped in v0.17.0.
