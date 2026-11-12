# PRD - Historical-anchor selection after a genuine re-landing

**Status**: Direction accepted — operator answered §9 (2026-11-06): options
A and C, newest qualifying exact attestation, D after N1–N2, A3 out of
scope. Independent review APPROVED at rev-3 (2026-11-06,
`docs/supervisor/LOG.md`). N0 is ADR-013 Amendment 4, ACCEPTED
2026-11-07 (design review rev-4 APPROVED). The operator scheduled N1 for
the next window; it was implemented on 2026-11-11 and approved on code
review. Its D44 measurement found differences the rev-4 pins did not name,
so Amendment 4 rev-5 (D44 only) answers them; its design review APPROVED
it on 2026-11-11. N1 shipped in v0.21.0. The operator scheduled N2 on
2026-11-11; it was implemented that day and approved on code review, and
is not yet released. No schema change is authorized.
**Date**: 2026-11-06
**Owner**: Core
**Issue**: [GH #26](https://github.com/tesseracode/tesserapatch/issues/26)
**Related**: [ADR-013](../adrs/ADR-013-verify-freshness-overlay.md) (D13, D14, E30, Amendment 2 D20–D27), [ADR-019](../adrs/ADR-019-tpatch-land-trailer-block-schema.md), [ADR-024](../adrs/ADR-024-patch-generation-manifest-boundary.md),
[PRD-overlap-verification-consolidation](PRD-overlap-verification-consolidation.md) (§2.3 Fusion, options A3/C3), [SPEC.md](../../SPEC.md) §"Landed-feature verification", [docs/ROADMAP.md](../ROADMAP.md) (GH #26 row); GH #25.

---

## 1. Summary

A child feature lands. Its hard parent later changes and lands again. The
child is then genuinely unapplied, re-applied on the parent's new landing
and landed again. Its patch and recipe bytes do not change. `tpatch verify
<child>` still exits 2 with `failed_at: parent-landing-drift` and tells the
operator to verify the parent first. That advice does not help.

The cause is the historical-anchor rule. Verify selects the child's replay
anchor as the **oldest** qualifying landing, which is the child's first
landing, made before the parent changed. It then probes the parent's
**current** patch at that old tree. The parent's current content is not
there, so the parent ladder blocks.

This PRD proposes an attestation-preferred anchor rule (option A), a
truthful refusal for children that really are older than their parent's
current landing (option C), and a separate slice for the Fusion
member-level explanation that GH #25 Q9 folded into this work (option D).
It recommends A + C first, then D.

## 2. Problem

### 2.1 Observed (GH #26, reproduced on v0.18.0)

At t3code `5ce5d79`, four independently recorded roots exit 2 at
`recipe_replay_clean` / `failed_at: parent-landing-drift`, while their
current-anchor result is `materialized-clean` (GH #26 body). The ROADMAP
records the same result on v0.18.0 (2026-11-04, read-only; GH9-D13).

For `attention-notifications`:

| Evidence | Commit |
|---|---|
| Exact attestation (base trailer = recorded base) | `964529c` |
| Genuine new parent/base | `19785c6` |
| Selected historical base | `75bfc26` |
| Selected replay landing | `55fd4d7` |

An independent Git proof (GH #26) shows that at the new base `19785c6` the
corrected parent reverses cleanly and the child forward-applies with no
residual paths.

### 2.2 Root cause

1. **Evidence is bound to the current base.** A candidate attests exactly
   only if its patch, recipe **and base-commit** trailers all equal the
   recorded state (`internal/workflow/verify_landed.go:865-879`). Here
   only `964529c` attests.
2. **The replay anchor is not.** Anchor candidates are qualified by forward
   apply alone, with no trailer binding (ADR-013 D14 steps 1–2,
   `docs/adrs/ADR-013-verify-freshness-overlay.md:702-716`;
   `internal/workflow/verify_landed.go:1242-1277`). Both landings qualify,
   because the bytes are identical.
3. **Selection is oldest-first.** Among equal-identity qualifiers the first
   in `git log --topo-order --reverse` order wins (D14 step 4,
   `ADR-013:749-751`; `internal/gitutil/trailers.go:302`;
   `internal/workflow/verify_landed.go:1308-1318`). That is `55fd4d7`,
   whose parent is `75bfc26`.
4. **Members are probed at the target's anchor.** Closure arbitration runs
   each landed hard parent's current patch through the ladder at the
   child's anchor tree (`internal/workflow/verify_anchored.go:483`,
   `:695`). At `75bfc26` the parent's current postimage is absent, so the
   ladder blocks and R14 is returned
   (`internal/workflow/verify_anchored.go:705-706`).
5. **R14 cannot be satisfied.** R14 says "verify `<parent>` first"
   (`internal/workflow/verify_landed.go:1536-1538`). The parent is healthy;
   verifying it changes nothing.

### 2.3 Why the rule is oldest-first

D14 chose oldest-first on purpose. After a re-record and re-land **on top
of** an existing landing, the newest landing's parent already contains the
patch, so only the earlier landing's parent can serve as a replay baseline
(E30, `ADR-013:462`, `:772-775`). Rejected alternative 12 forbids using the
attestation as the replay anchor for this reason (`ADR-013:1225`).

E30 is a case where the attestation's parent does **not** pass the forward
check. GH #26 is a case where it does. The fix must keep E30's behaviour.

### 2.4 Why existing recovery paths fail

- **Fixup** refuses an empty capture (`internal/cli/feature_patch.go:98`).
- **Generations** from ordinary record, refresh, fixup and landing do not
  append when the patch digest equals the latest
  (`internal/store/patch_generation_kinds.go:47-49`;
  `internal/workflow/patch_generations.go:97-99`), so a genuine re-land
  with identical bytes leaves no generation trace. Since v0.18 there is one
  same-digest exception: ADR-051's reconcile operation-candidate acceptance
  appends when an accepted candidate changes the binding tuple
  (`internal/workflow/patch_generations.go:42-45`, `:97-105`;
  `ADR-051:142-147`). It gives no re-landing recovery. A plain re-land never
  passes through candidate acceptance; staging a reconcile candidate only to
  obtain a generation would be the forged generation trace the issue rules
  out; and D14 anchor selection reads only landing trailers and forward
  qualification, never generations
  (`internal/workflow/verify_landed.go:1242-1277`).
- **No baseline override** exists, and the issue rules out fake patch
  changes, forged generation manifests and rewritten history.

### 2.5 The folded Fusion case (GH #25 Q9)

In the Fusion stack (provider → packaging → SDK → Fusion), a landed hard
ancestor's postimage is absent at the target's fixed anchor because an
intermediate member legitimately evolved it
(`PRD-overlap-verification-consolidation.md` §2.3). The same arbitration
branch returns R14 (`verify_anchored.go:705-706`). The root cause differs
from §2.2: the anchor is correct, but the member check does not consider
recorded evolution by other closure members. GH #25 deferred the
explanation (C3) and the authority change (A3) to this work.

## 3. Goals and non-goals

**Goals**

- G1. A genuinely re-landed child whose recorded base is the parent's
  current landing verifies against that base.
- G2. A child that really is older than its parent's current landing fails
  with a truthful remediation that names both commits and does not
  recommend verifying the parent.
- G3. Keep E30: a re-land on top of an existing landing still anchors at
  the earlier landing.
- G4. Keep every negative guard: the current-anchor half (anchor C), the
  ambiguity rule, D10 presence, terminal evidence and V10.
- G5. Plan the Fusion member-level explanation without granting it
  authority.

**Non-goals**

- No new trailer, generation kind or history rewrite.
- No relaxation of `-C1` qualification or of the identity compare.
- No pass from historical replay alone (anchor C stays mandatory).
- No change to attestation (`landing_evidence`) semantics.
- A3 (member-level **authority**) is out of scope; see §9 Q5.

## 4. Could existing primitives do this?

| Primitive | Can it carry the binding? |
|---|---|
| `Tpatch-Base-Commit` trailer | Yes. It already binds the exact attestation to `status.apply.base_commit` (§2.2.1). Option A reuses it. |
| `patch-generations.json` | No. Ordinary producers never append identical bytes, the ADR-051 exception is unreachable from a re-land, and anchor selection never reads generations (§2.4). |
| `status.apply.base_commit` | Yes, through the attestation; read-only. |
| ADR-013 D14 identity compare | Yes. It already proves equal-identity qualifiers describe the same change. |
| W2 provenance replay (Amendment 2 D22) | Yes for option D: the same temp-index replay can explain a member's absence at the anchor instead of at `HEAD`. |

No new data object is needed for A or C.

## 5. Options

### A. Attestation-preferred anchor selection

Change D14 step 4 only:

> Among equal-identity qualifiers, if the **exact attestation commit** is a
> qualifier, select it. Otherwise select the first in oldest-first order,
> as today.

- E30 is unaffected: there the attestation does not qualify.
- Rejected alternative 12 stays rejected: the attestation is used only when
  it passes the same forward check as any other candidate.
- Collection, `-C1` qualification and the identity compare are unchanged;
  non-equivalent qualifiers are still terminal.
- `duplicate-equivalent` evidence (several exact attestations): see §9 Q2.
- Reporting already separates `attestation_commit` from
  `replay_anchor_commit` (`ADR-013:757-763`); in the fixed case they become
  equal.

**Risk:** the newer anchor tree carries more unrelated history. That only
matters if the child's recipe replay or a member ladder depends on it, and
both are already required to succeed at whichever anchor is chosen.

### B. Generation-bound landing observation

Record each landing commit as a generation-side observation, so anchor
selection can bind to the generation that landed. This needs a new record
kind or a post-land write, which ADR-019 avoided (circular write) and
ADR-024 bounds. It also does not help existing repositories. **Not
recommended** unless A proves insufficient.

### C. Truthful refusal (R14 split)

When a landed member's ladder blocks at the target's anchor, check whether
the member's attestation commit is an ancestor of that anchor (via the
existing gateway).

- If it is **not**, the parent's current landing is not in the history of
  the child's baseline (the child was applied before it, or on a branch
  that never contained it; non-ancestry alone does not prove which —
  ADR-013 Amendment 4 design review, 2026-11-06). Return a new remediation (working name **R14b**) naming the
  child's replay anchor and the parent's attestation, stating that
  verifying the parent will not help, and recommending a genuine
  unapply/re-apply and re-land of the child on the parent's current
  landing.
- If it is, keep R14.

`failed_at` stays `parent-landing-drift`; only the remediation changes.
Option C alone already satisfies the issue's "actionable, truthful
refusal" alternative; with A it covers the cases A does not resolve.

### D. Fusion member-level explanation (C3)

When the member ladder blocks and the member **is** an ancestor of the
anchor (R14 case), run Amendment 2's provenance replay with the target's
anchor tree in place of `HEAD`:

1. Seed an isolated index from the member's **intact** landing
   (attestation) tree.
2. Reverse-**check** the member's captured patch there without applying
   it. This proves the member's postimage is present at the seed. A check
   Git answers negatively → `unexplained-content-loss` (no descendant can
   explain a postimage that never landed); a check Git could not run →
   `invalid-or-missing-evidence`. Neither is ever an explanation.
3. Apply, in landing order, the captured patches of the closure members
   that descend from the member and landed at or before the anchor.
4. Compare the result with the anchor tree on the member's paths.

This mirrors D22, which seeds the target's landing tree and only
reverse-checks the target patch
(`internal/workflow/verify_overlap.go:523-549`). Removing the member's
patch at the seed would be wrong: the anchor tree still carries every
member hunk that no descendant rewrote, so a partial evolution could never
compare equal. Report the class in a member-level field of
`overlap_assessment` (schema bump) and branch the remediation, as C2 did.
Explanation only: `failed_at`, verdict and exit code do not change.

A3 (letting a proven explanation pass the member check) changes the D13
table and is a separate decision (§9 Q5).

## 6. Recommendation and slice plan

1. **N0 — ADR-013 Amendment 4** (Amendment 3 is reserved for GH #25 A1,
   OV-D4): D14 step 4 attestation preference; R14b; the E-series
   measurements below. **Drafted 2026-11-06** (D40–D45, measurements
   A4-E1–A4-E6); **ACCEPTED 2026-11-07** at rev-4, after a design review
   that added tests T11–T16 and D44's pinned pass criterion.
2. **N1 — option A** plus its tests.
3. **N2 — option C** plus its tests.
4. **N3 — option D** (C3), after N1–N2 ship, with its own design review.
5. **Reproduction:** re-run the t3code `5ce5d79` commands (GH #26) on the
   N1 build, read-only, and record the four roots' results.

## 7. Test plan (mutation-sensitive)

Each fixture must fail under the named mutation.

| # | Fixture | Expected | Mutation that must fail it |
|---|---|---|---|
| T1 | Child landed; parent changed and re-landed; child genuinely unapplied, re-applied and re-landed with identical bytes. | Pass; replay anchor = attestation. | Revert to oldest-first. |
| T2 | E30: re-record and re-land on top of an existing landing. | Pass; replay anchor = earlier landing. | Use the attestation unconditionally. |
| T3 | Parent re-landed; child **not** re-landed. | Exit 2, `parent-landing-drift`, R14b naming both commits. | Drop the ancestry check (R14 text returns). |
| T4 | Parent's ladder blocks and its attestation **is** an ancestor of the anchor. | R14 unchanged. | Always emit R14b. |
| T5 | T1, plus a later unrecorded edit at `HEAD` on the child's path. | Exit 2 at the current anchor. | Skip anchor C when anchor H passes. |
| T6 | One exact attestation that qualifies, plus a stale-trailer landing of the same slug that also forward-qualifies at `-C1` but has a non-equivalent normalized identity. | Terminal `historical-anchor-unavailable` (D14 ambiguity). | Skip the identity compare when the attestation qualifies. |
| T6b | Two exact attestations with non-equivalent identities. | Evidence `ambiguous` routes to the terminal landing-evidence phase before any anchor is selected: `failed_at: landing-evidence`, R7 (`internal/workflow/verify_landed.go:927-937`, `:1593-1594`; `internal/workflow/verify_anchored.go:110-111`, `:167`). Option A never runs. | Let option A choose among exact attestations before evidence classification. |
| T7 | `duplicate-equivalent` attestations (per Q2's answer). | The chosen rule. | The other rule. |
| T8 | The forward check cannot run (simulated Git failure) on the attestation candidate. | Reader-failure state, not a silent fallback to oldest-first. | Treat the error as "does not qualify". |
| D1 | Option D (N3): a descendant rewrites one of a landed member's two hunks; the other hunk is unchanged at the target's anchor. | Member-level `recorded-descendant-evolution`; verdict, `failed_at` and exit code unchanged. | Reverse-apply (remove) the member's patch at the seed instead of reverse-checking it. |

T1–T4 also run under `verify --all`, and the per-feature reports must
match the single runs (existing aggregate-equality pattern).

## 8. Compatibility and release

- A changes which tree some already-passing runs replay at. Under
  equal-identity qualification both trees accept the same change, so a
  previously passing run can only fail if the newer tree breaks the
  child's recipe replay or a member ladder. N1 must measure this on the
  repository's own fixtures and on the t3code reproduction before release.
- C changes remediation text only. D adds a field and bumps the report
  schema (additive).
- Skill assets that describe verify output must stay in parity
  (`CLAUDE.md` rule 10).
- Release: a minor version per slice that changes behaviour (N1, N3).

## 9. Questions for the operator

Operator answers, 2026-11-06 (recorded in `docs/handoff/CURRENT.md`):

1. **Approve option A** (attestation preferred when it qualifies) as the
   primary fix? Recommended: yes. **Answer: yes.**
2. **`duplicate-equivalent`:** with several exact attestations, prefer the
   newest qualifying one (ADR-019's "most recent") or keep oldest-first
   among them? Recommended: newest qualifying. **Answer: newest
   qualifying.**
3. **Approve R14b** (option C) with `failed_at` unchanged? Recommended: yes.
   **Answer: yes.**
4. **Fusion explanation (D/C3):** schedule after N1–N2, or defer?
   Recommended: after. **Answer: after N1–N2 ship.**
5. **A3 (authority):** keep out of scope? Recommended: yes, until D has
   run on real stacks. **Answer: yes.**
6. **Numbering:** call the semantics ADR-013 Amendment 4? Recommended: yes.
   **Answer: yes.**

## 10. Claims audit

| # | Claim | Anchor | Verified |
|---|---|---|---|
| 1 | Exact attestation needs patch, recipe and base trailers to match. | `internal/workflow/verify_landed.go:865-879` | yes |
| 2 | Anchor candidates are collected by slug and qualified by forward apply only. | `internal/workflow/verify_landed.go:1242-1277`; `ADR-013:702-716` | yes |
| 3 | Enumeration is `git log --topo-order --reverse` (oldest-first). | `internal/gitutil/trailers.go:302`; `ADR-013:464` | yes |
| 4 | Selection takes the first qualifier, tie-break by smallest id. | `internal/workflow/verify_landed.go:1308-1318`; `ADR-013:749-751` | yes |
| 5 | Members are ladder-probed at the target's anchor; a block returns R14. | `internal/workflow/verify_anchored.go:483`, `:695`, `:705-706` | yes |
| 6 | R14 recommends verifying the parent first. | `internal/workflow/verify_landed.go:1536-1538` | yes |
| 7 | E30 and rejected alternative 12. | `ADR-013:462`, `:772-775`, `:1225` | yes |
| 8 | Attestation and replay anchor are reported separately. | `ADR-013:757-763` | yes |
| 9 | Fixup refuses an empty capture. | `internal/cli/feature_patch.go:98` | yes |
| 10 | Ordinary producers never append a generation for an identical digest; the one exception is ADR-051 reconcile candidate acceptance with a changed binding tuple. | `internal/store/patch_generation_kinds.go:47-49`; `internal/workflow/patch_generations.go:42-45`, `:97-105`; `ADR-051:142-147` | yes |
| 14 | Exact attestations with non-equivalent identities are `ambiguous` and terminal before anchor selection (R7). | `internal/workflow/verify_landed.go:927-937`, `:1593-1594`; `internal/workflow/verify_anchored.go:110-111`, `:167` | yes |
| 15 | W2's replay seeds the landing tree and reverse-checks, never removes, the target patch. | `internal/workflow/verify_overlap.go:523-549` | yes |
| 11 | ADR-019: re-landed slugs are not unique; consumers pick the most recent. | `ADR-019:65-69` | yes |
| 12 | GH #26 still reproduces on v0.18.0. | `docs/ROADMAP.md` GH #26 row | yes |
| 13 | GH #25 Q9 folds A3/C3 into this work. | `docs/handoff/CURRENT.md` "Operator approvals" (Q9) | yes |
