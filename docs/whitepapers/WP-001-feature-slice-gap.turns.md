# WP-001 — Turn Log

Append-only. Newest turn at the bottom. See
[`README.md`](./README.md#turn-log-protocol) for the entry format and rules.

---

## Turn 1 — CO47 — 2026-04-27

**Responding to**: external prompt (review of three exploratory PRDs)
**Type**: review

Findings against the three PRDs, ordered by severity:

1. **Major — op-log claim understates what ships.** The intent-VCS PRD
   (`PRD-intent-version-control-evaluation.md:93,258,311-321`) and mapping
   PRD (`:184-194`) treat operation history as future work, but per-feature
   op history already exists: `patches/NNN-<label>.patch`
   (`docs/feature-layout.md:51-67`), `apply-session.json`,
   `resolution-session.json` (`docs/agent-as-provider.md:240-256`), and
   `status.json:notes`. The real gap is *cross-feature, repo-wide* op
   history.
2. **Major — mapping PRD is incomplete.** `PRD-tpatch-git-primitive-mapping.md:42-58`
   omits `amend`, `remove [--cascade]`, `feature deps`, `replay`,
   `upstream check`, `provider check`, `config`, and the entire reconcile
   shadow flow (`SPEC.md:50-76`, `docs/agent-as-provider.md:199-291`).
   These are exactly the semantics the companion PRD says are missing.
3. **Major — slices PRD never engages with `created_by` + hard-child
   features as a pre-existing slicing mechanism.** `created_by`
   (`docs/agent-as-provider.md:102-108`, `docs/dependencies.md:114-138`)
   plus `depends_on:hard` (`docs/dependencies.md:36-46`) already form an
   ordered, individually-reconcilable parent-child contract.
4. **Major — `tpatch land` is not differentiated from `tpatch cycle`.**
   `cycle <slug>` already ships (`SPEC.md:73`); neither PRD compares them.
5. Moderate — patch-authority conflict in proposed slice layout
   (`PRD-feature-slices-and-nested-changes.md:130-144` vs
   `docs/feature-layout.md:34-44`).
6. Moderate — feature identity is overstated as a gap; slug is already
   stable, only the feature-to-Git-commit mapping is split-brain.
7. Minor — "Patch B" vs "Path B" naming
   (`PRD-intent-version-control-evaluation.md:61` vs
   `docs/agent-as-provider.md:1-13`).

**Recommended next experiment**: paper-only retroactive decomposition of
**one** historical oversized feature using `depends_on:hard` + `created_by`
only. If it works, the slices PRD collapses; if it doesn't, the specific
pains define the slice object.

**Asks of next agent**:
- Independent audit; flag agreements and disagreements.

---

## Turn 2 — G55 — 2026-04-27

**Responding to**: external prompt (same task)
**Type**: review

No major findings. Three medium and one low.

1. **Medium — operation layer overstated.** The recipe is treated as a
   durable op-level representation, but `docs/feature-layout.md`,
   `docs/record.md`, and `docs/agent-as-provider.md` make the **patch**
   the authority. The recipe is a snapshot-targeting executable projection
   and can be absent / stale / lossy to regenerate. PRDs should say tpatch
   has "replay aids" + "feature-scoped patch authority", not yet a full
   operation-log layer.
2. **Medium — mapping PRD omits feature-graph commands.**
   `tpatch feature deps`, `tpatch amend --depends-on`,
   `tpatch remove --cascade` (per `docs/dependencies.md` and
   `internal/cli/feature_deps.go`) are among the strongest examples of
   what tpatch adds beyond Git and must be in the baseline mapping.
3. **Medium — slices PRD identifies the patch-authority ambiguity but
   still proposes a fixture with two slices each owning their own patch
   and recipe** (`PRD-feature-slices-and-nested-changes.md`). The
   "paper design first" recommendation should be a **hard prerequisite**,
   not a recommendation.
4. **Low — stale link.** `PRD-feature-slices-and-nested-changes.md`
   references `PRD-tpatch-git-command-mapping.md`; actual file is
   `PRD-tpatch-git-primitive-mapping.md`.

Doc drift noted: `agent-as-provider.md` says `created_by` is "currently
inert"; `dependencies.md` + code (`created_by_gate.go`) treat it as a
**live gate**. Treated `dependencies.md` + code as authoritative for this
review.

**Recommended next experiment**: smallest possible `tpatch land <slug>`
spike — no slices, no hidden refs, no new op log. Bridge `record` and
Git commit safely. Dry-run that prints exactly what would be staged and
committed. Add machine-readable feature binding (e.g. `Tpatch-Feature:`
trailer). Success = fewer record-before-commit mistakes, reliable
feature-to-commit mapping, no surprise hidden state, compatibility with
both reconcile patterns in `docs/reconcile.md`. Slices remain paper
design until this proves the larger feature-boundary problem is real and
unsolved.

**Asks of next agent**:
- Compare with Turn 1; surface convergences and divergences.

---

## Turn 3 — CO47 — 2026-04-27

**Responding to**: Turn 2
**Type**: review

Convergence with G55:

- Mapping PRD incompleteness — **same finding, same omitted commands**
  (`feature deps`, `amend --depends-on`, `remove --cascade`).
- Slices PRD steps into the patch-authority trap too early.
- Next experiment is a `land` spike, not slice implementation.

Complementary (not contradictory):

- Op-log gap. Turn 1 attacked the **omission** (existing per-feature op
  trail isn't credited); Turn 2 attacked the **claim** (recipe is not
  durable op layer). Both true; combined: the gap is **cross-feature,
  repo-wide** history; per-feature op history already ships in narrower
  form.
- Turn 2 caught real items Turn 1 missed: `created_by` doc drift
  (`agent-as-provider.md` "inert" vs `dependencies.md` + code "live gate")
  and the stale PRD link.
- Turn 1 caught items Turn 2 didn't surface: slice ≈ hard-child +
  `created_by` + thin parent contract; `land` vs `cycle` overlap.

Process observation: both reviews independently found the **same**
mapping incompleteness and the **same** patch-authority blind spot. That
is a structural defect in how the exploratory PRDs were written — they
did not audit their own claims against the docs in their own "Related"
header. Future exploratory PRDs should ship with a one-page "claims
audit" appendix.

**Asks of next agent (G55)**:
- Confirm or refine the working-draft hypothesis in WP-001 §2.
- Confirm canonical-patch-authority is the right invariant to pin first.

---

## Turn 4 — G55 — 2026-04-27

**Responding to**: Turn 3
**Type**: agreement (with refinement)

Confirms: next step is a paper-only gap study. Key hypothesis to test —
`depends_on:hard` child features + `created_by` already cover most
"feature slice" use cases.

Pinned invariant: **canonical patch authority** — one replayable unit
must have one clear `post-apply.patch` source of truth. No slice or
sub-unit design proceeds without a written answer to this for the
parent/child case.

Refines op-log point: *per-feature audit trail exists; repo-wide
operation history with recovery semantics does not*. This becomes the
durable phrasing for §2.

**Asks of next agent**:
- Lift the hypothesis + invariant + op-log refinement into WP-001 §2 as
  the **working draft** (not yet ratified). Leave §2 ratification for a
  later turn that closes the open questions in §6.

---

## Turn 5 — CO47 — 2026-04-27

**Responding to**: Turn 4
**Type**: proposal

Drafted §3 (CO47 view) of WP-001. Key positions taken there:

- The residual slice gap probably resolves to either a `containment`
  edge kind or a UX convention on top of unchanged primitives — **not**
  a new storage object. Cites in §3.1.
- The intent-VCS framing bundles a (largely-closed) data-model gap with
  a (still-open) UX gap. `land` is a UX-gap experiment, not a step
  toward an intent-aware VCS. §3.2.
- G55's T4 op-log phrasing is the correct restatement and should land
  verbatim in §2 once ratified. §3.3.
- `land` scope: `record` + safe staging + one commit + `Tpatch-Feature:`
  trailer. `cycle` owns phase orchestration; they compose, don't
  overlap. §3.4.
- Process recommendation: future exploratory PRDs ship with a one-page
  claims-audit appendix and a "could this be done with existing
  primitives?" pre-flight. §3.5.

§2 ("Agreed re-statement") deliberately **not** updated. Per the
turn-log rules, that section only changes when a turn produces an
agreement; G55 has not yet had a turn after §3 was drafted.

**Asks of next agent (G55)**:

1. Read WP-001 §3 in full (it cites file:line ranges; verify any you
   doubt).
2. Fill WP-001 §4 with G55's view, with bylines and cites in the same
   shape as §3. Disagreement is welcome; do not paper over.
3. Take explicit positions on the three asks at the end of §3.6.
4. If §4 produces an agreement on any sub-point with §3, write it into
   §2 in the same edit and cite the turn number.
5. Append your turn (T6) to this log following the format in
   `README.md`.

## Turn 7 — CO47 — 2026-04-27

**Responding to**: Turn 6
**Type**: agreement

Answers G55's two asks from T6.

**Ask 1 — §7 sequencing.** Reorder, not parallel. The decomposition case
study must run **before** the `land` paper-spike, because the table the
case study produces determines what `land` even needs to project (one
commit per parent? per child? a mix?). Running them in parallel risks
`land` solving the wrong shape, which is exactly the failure mode the
exploratory PRDs already hit. Updated §7 to make this explicit (Step A
→ Step B, no overlap).

**Ask 2 — containment as a third `depends_on.kind`.** Confirmed: no.
G55's §4.3 reasoning is correct and CO47 retracts the "(a) edge kind"
half of §3.1's two-option phrasing. Concrete reason: `kind` is an
**apply-gate semantic axis** — `hard` is a gate, `soft` is a hint, both
answer the question "what does the apply path do at this edge?". A
containment relation answers a different question entirely ("is this
child part of a larger product intent?") and has no apply-gate meaning.
Encoding it as a third `kind` would force every consumer of `kind`
(apply, reconcile, status, label overlay, removal cascade) to handle a
value that is semantically inert to it — pure schema pollution. If a
first-class containment relation is ever needed, it lives in a
**separate field or edge type** (e.g. `contained_by` at the feature
level), not inside `depends_on[]`.

Both points lifted into §2 with `(T7)` markers.

§3.1's "(a) `containment` edge kind" wording is now superseded by the §2
agreement; CO47 will leave §3 unedited per the append-only spirit but
notes this here for future readers.

**No remaining open ask between CO47 and G55** at this point. The
whitepaper is blocked on a human-broker decision: which historical
oversized feature is Step A's subject. Without that selection, Step A
cannot start, Step B cannot start, and §5 (backlog triage) cannot
proceed because the gap re-statement is not yet ratified against any
concrete case.

**Asks of next agent (human broker)**:

1. Pick the historical oversized feature for Step A. Anchor by either
   (a) feature slug if it lives in a real `.tpatch/features/<slug>/`
   directory, or (b) a description plus enough commit/PR refs that the
   next executing agent can locate the source material.
2. Optionally constrain Step A's depth (e.g. "produce only the gap
   table; no recipe-level decomposition yet" vs "decompose fully into
   2–3 child features with proposed `depends_on` edges").
3. Decide whether the `created_by` doc-fix in `agent-as-provider.md`
   (already agreed in §2/§4.6) should be done by CO47, G55, or routed
   as a separate task — it is the one action item from this whitepaper
   that does not depend on Step A.

## Turn 9 — jbencardino (human broker) — 2026-04-27

**Responding to**: Turn 8
**Type**: steering

Surveyed three candidate sources before fixing Step A's subject. Findings
and a counter-proposal that **augments** G55's selection rather than
replacing it.

### Sources surveyed (read-only)

1. `tesseracode/copilot-api/.tpatch/features/` — G55's selection lives here
   as upstream Git commits `f831904` + `f6e9076`. Confirmed: the three
   matching feature directories (`per-generation-thinking`,
   `internal-suffix-resolution`, `anthropic-beta-1m-detection`) are all
   in `state=requested` with **no** `post-apply.patch` yet. So G55's case
   is a *recovery from raw Git commits* problem — features that were
   never recorded as separate tpatch units.
2. `tesseracode/t3code/.tpatch/features/` — 21 features, mostly `applied`.
   **Eleven of them share the byte-identical `post-apply.patch`**
   (md5 `f491eb4d…`, 137285 bytes). Verified by `md5sum` over the patch
   files. This is a textbook canonical-patch-authority breakdown: those
   eleven features each claim to "be" the entire fork-vs-upstream diff.
3. `multiwebsiteshowroom/docs/` — forward-looking design docs only; no
   `.tpatch/features/` directory. Useful for future case studies, but
   not for Step A's "retroactively decompose a real oversized feature"
   purpose.

### Why both copilot-api and t3code matter

These are **two different failure modes of the same underlying gap**, and
they bracket it neatly:

- **copilot-api (G55's pick)** — *boundary capture from Git into tpatch*.
  Real work happened, real Git commits exist, no tpatch artifacts
  separating the concerns. Question: can the existing primitives
  (`record --from`, child features, `created_by`) reverse-engineer two
  safe canonical patches without hand-slicing?
- **t3code (this turn's pick)** — *boundary capture from working tree
  into tpatch*. Eleven features were recorded, but every recording
  captured the wrong scope (almost certainly `record --from
  upstream/main` over a multi-feature branch), producing eleven
  identical patches. Reconcile against any one of them would replay
  the entire fork. Question: at what point should tpatch have refused,
  warned, or guided the operator into `--from <feature-base>` instead
  of `--from upstream/main`?

Per the human-broker hint: this is almost certainly a **UX-gap**
manifestation, not a data-model gap. The slices PRD would not have
prevented either failure. What might have prevented them:

- earlier `record` guardrails that detect "this patch is identical to
  the patch of N other features" and refuse,
- `land` (Step B) creating a per-feature commit boundary at the moment
  intent is captured, eliminating the after-the-fact `--from` guess,
- explicit per-feature baseline tracking (`status.json:apply.base_commit`
  already exists per `docs/feature-layout.md:86`) being **enforced** on
  `record`, not just recorded.

If Step A studies only one of the two, the gap table will be lopsided.

### Proposal for Step A scope

Pair the two cases. Both are paper-only, cheap, and complementary:

- **Case A1 — copilot-api recovery** (G55's pick, unchanged in scope).
  Two concerns inside `f6e9076` + one inside `f831904`. Question: can
  current primitives produce three safe canonical feature patches?
- **Case A2 — t3code patch collision**. The eleven features sharing
  `md5:f491eb4d…`. Question: at what point in the workflow should the
  tooling have caught this? Is the fix an existing-primitive guardrail
  (e.g. `record` rejects identical patches across features) or a new
  primitive?

Step A's gap table should be one combined table covering both cases,
with a per-case column so the pattern is visible. The constraint stays:
only the *true data-model gap* category may reopen the
slice/containment question.

### Step A depth and the doc-fix routing

To answer T7's other two asks while I'm here:

- **Depth**: gap table only for now. No proposed `depends_on` edges, no
  recipe-level decomposition. A clean classification per case is more
  valuable than speculative re-modelling, given the UX-gap hypothesis.
- **`created_by` doc-fix routing**: route as a separate task (not
  CO47, not G55 as part of WP-001). It's a single doc PR against
  `agent-as-provider.md:102-107` that should track the live gate
  language in `docs/dependencies.md:127-138`. Keep it out of WP-001's
  graduation path.

### Out-of-scope for this turn

- Showroom-driven forward-looking case studies. Park for later.
- Any of the other 8 t3code features that don't share the colliding
  patch hash — they're real but not where the pain shows.
- The 7 copilot-api features that *do* have content
  (`health-endpoint`, `hide-internal-models`,
  `log-model-display-name`, `model-vendor-filter`,
  `native-payload-sanitization`, `startup-model-count`,
  `three-tier-routing`, `three-tier-routing-tests`). Several of these
  also share suspicious patch sizes (≈14793b for many,
  `three-tier-routing` notably larger at 34328b). Worth a follow-up,
  not Step A.

**Asks of next agent (CO47 to confirm or push back, then G55 to ratify
or amend)**:

1. Accept Case A1 + Case A2 as the paired Step A subject? If so, lift
   the pairing into WP-001 §7 Step A and update §2 with a one-line
   note that Step A is now grounded in **two** complementary cases.
2. Confirm "gap table only, no decomposition" depth.
3. Confirm separate-task routing for the `created_by` doc-fix (so
   WP-001 stops carrying it as an asterisked agreement).

## Turn 10 — jbencardino (human broker) — 2026-04-27

**Responding to**: own T9
**Type**: context (corrections + new evidence)

T9 made two factual mistakes about the candidate sources and missed a
substantial body of pre-existing tracking. Correcting before either
agent acts.

### Correction 1 — copilot-api Path-B features now have patches

T9 said the three Path-B features (`per-generation-thinking`,
`internal-suffix-resolution`, `anthropic-beta-1m-detection`) had no
`post-apply.patch`. They do now (recording finished after T8). All
three are `state=applied`, base
`ad9eef73f1b646697dbcc52dc9982f3aded8189a`, patch size 15914 bytes.

But the more important fact: **G55's three features share an
identical patch byte-for-byte** (md5 `df5be1df72bf12c599e7b9a902bf5c12`).
Verified by `md5sum` over the actual files.

In fact, the entire copilot-api `.tpatch/features/` directory has only
**four distinct patch contents across eleven features**:

| md5 prefix | feature count | features |
|---|---|---|
| `df5be1df…` | 3 | `anthropic-beta-1m-detection`, `internal-suffix-resolution`, `per-generation-thinking` |
| `2825515d…` | 5 | `health-endpoint`, `hide-internal-models`, `log-model-display-name`, `model-vendor-filter`, `startup-model-count` |
| `f68dd4a2…` | 2 | `native-payload-sanitization`, `three-tier-routing-tests` |
| `c9c5baf4…` | 1 | `three-tier-routing` |

This means T9's framing was wrong: copilot-api (Case A1) is **not** a
"recovery from raw Git commits" case. It is the **same failure mode**
as the t3code 11-feature collision (Case A2), just at a different scale
(8 of 11 features collide here vs 11 of 21 there). Both repos exhibit
boundary-capture-from-working-tree-into-tpatch.

That actually **strengthens** the pairing — not as two complementary
failure modes, but as the same failure reproducing across two
independent repos and two independent operators (different sessions,
different upstreams). The signal is robust.

### Correction 2 — `multiwebsiteshowroom` is for demo candidates, not historical case studies

T9 dismissed showroom for the right reason but the wrong description.
It does have a structured `apps/web/fixtures/showroomProjects.ts` file
listing candidate forks for tpatch demos (e.g. `t3code-copilot-integration`).
That fixture is **forward-looking demo material**, not retrospective
artifacts. Right call for Step A; wrong reason given. Keep it parked.

### New evidence — the read-only backlog already tracks much of this

The repo has a tracked SQLite mirror at
`.tpatch-backlog/backlog.db` (gitignored as data, but its existence is
known and it is read-only the way agents should treat it). Sweeping it
with `sqlite3` reveals **161 todos**, 59 pending, and at least seven
items directly inside the WP-001 problem space:

| ID | Status | Why it matters to Step A / WP-001 |
|---|---|---|
| `feat-feature-decomposition` | pending | "Support splitting a large feature into sub-features / epics" — literally the slice question. |
| `feat-feature-import` | pending | "Reverse-engineer existing fork changes into tpatch features" — literally what would fix Case A1's *original* shape (before recording). |
| `feat-noncontiguous-feature-commits` | pending | Per-feature commit ledger so non-contiguous features re-derive cleanly — directly relevant to Step B (`land`). |
| `feat-record-auto-base` | pending | `tpatch record --auto`: infer `--from` from `upstream.lock` / merge-base. **Would have prevented both A1 and A2 collisions.** |
| `feat-record-dedup-patches` | pending | Skip numbered snapshot if byte-identical to previous — would not have prevented the collision but would have surfaced it. |
| `feat-feature-amend` / `feat-feature-removal` / `feat-feature-reorder` / `feat-feature-standalonify` | pending | DAG-shape-mutation primitives that bound any slice/containment design space. |
| `feat-agent-collision-detection`, `feat-feature-file-claim`, `feat-parallel-feature-workflows` | pending | Concurrency + ownership primitives; orthogonal to slices but adjacent to canonical-patch-authority. |

Plus the bug/doc trio from your earlier session:

- `bug-record-files-incompatible-with-from` (high) — `--files` rejected with `--from`; `--to`/`--commit-range` never shipped.
- `doc-skills-record-flags` (medium) — skill-file doc update for `--files` + `--exclude`.
- `feat-record-scoped-files` (done, partial-shipping marker).

**Implication for Step A:** the gap table must not be drafted in a
vacuum. Each row should map (where applicable) to an **existing
backlog ID**, so we can see at a glance which gaps are already
recognised, which are tracked-but-deferred, and which are genuinely
new. Without that anchoring, Step A risks producing a parallel slate
that duplicates planning that already exists.

### Revised Step A scope

T9 proposed: Case A1 (copilot-api) + Case A2 (t3code), gap-table only.
T10 keeps that and adds:

1. **Pre-step (one agent only):** read-only sweep of
   `.tpatch-backlog/backlog.db` for every todo whose ID matches
   `feat-record%`, `feat-feature%`, `feat-recipe%`, `feat-noncontiguous%`,
   `feat-agent-collision%`, `feat-parallel%`, `bug-record%`. Produce a
   shortlist (the table above is a starting point but not exhaustive).
   Output goes into a **§5.1 backlog index** appended to WP-001 before
   the gap table itself, so the gap table can cite IDs.
2. **Step A executor:** when classifying each pain into *solved by
   primitive / awkward UX / missing Git projection / missing recovery
   history / true data-model gap*, also tag with the existing backlog
   ID(s) the pain maps to, or `(new)` if none.
3. **Output:** one combined gap table covering A1 + A2, with backlog
   anchors. Nothing else (no edge proposals, no recipe decomposition,
   no Step B preview).

### Updated asks for the agents

Replaces the asks at the end of T9 (which still stand for items 2 and 3
in unchanged form):

1. **G55** to do the pre-step backlog sweep (read-only) since CO47 just
   did the source-repo survey. Hands the §5.1 backlog index back to
   CO47 to merge with the gap-table draft.
2. **CO47** to ratify or push back on this revised Step A scope; if
   accepted, lift the pairing + backlog-anchoring requirement into
   WP-001 §7 Step A and update §2.
3. **Both** to confirm:
   - Step A depth: **gap table only**, no decomposition, no proposed
     edges. (Unchanged from T9.)
   - `created_by` doc-fix: **separate task**, not WP-001. (Unchanged
     from T9.)

## Turn 12 — CO47 — 2026-04-28

**Responding to**: Turn 11
**Type**: proposal

Drafted WP-001 §5.2 (Step A gap table for Cases A1 + A2), anchored to
G55's §5.1 backlog index per T11.

Verifications before drafting:

- T11's hash-group correction (10 of 11 copilot-api features in repeated
  groups, not 8) re-verified by `md5sum`; tracked in §5.2 evidence base.
  T10 was wrong; T11 is right.
- Spot-check on `tesseracode/copilot-api`: the three `df5be1df…` features
  (Path-B set) all share `base_commit ad9eef73f1b…` and all touch the
  same 4 files (`src/lib/model-mapping.{ts,test.ts}`,
  `src/routes/messages/handler.ts`,
  `src/services/copilot/forward-native-messages.ts`); none has
  `depends_on` set despite obvious topical overlap.
- t3code `f491eb4d…` collision touches ≥21 files across `apps/server/`,
  `apps/web/`, and `.claude/`; the 11 colliding slugs are thematically
  unrelated. No plausible "this is one feature" reading.

§5.2 produces an 11-row gap table. Headline result:

> **No row in either case requires a new data-model object.** The fixes
> live in `record` UX (prevention + detection), in the `land`
> Git-projection that Step B proposes, and in modest extensions of
> existing recovery primitives. Row 9 — `feat-feature-decomposition`,
> the closest thing tpatch already has to "slices" — would not have
> prevented either case.

Classification breakdown: 6 awkward-UX rows, 1 missing-Git-projection
row (Step B precondition), 2 missing-recovery-history rows, 0
true-data-model-gap rows. If G55 ratifies row 11, §2's working-draft
hypothesis collapses into a confirmed finding and WP-001 can graduate
into one or more PRDs.

Two probably-new backlog IDs flagged in §5.2 rows 6 and 10:

- `feat-feature-resplit` (or extension of `feat-feature-import`):
  re-derive distinct canonical patches from already-recorded but
  collided features.
- A `tpatch patches --collisions` view (extension of
  `feat-patches-subcommand`): cross-feature collision detection.

CO47 is **not** filing these as backlog entries from this turn. T10
already routed backlog edits as a separate decision; G55's third ask in
§5.2 is whether to file them or defer as scope notes on existing
anchors.

§2 not updated yet. Per the turn-log rules, §2 only changes when a
turn produces an inter-agent agreement. T12 is a unilateral CO47 draft
of §5.2; G55 must ratify (or amend) row 11 before §2 graduates.

**Asks of next agent (G55)**:

1. Sanity-check rows 1–5 (CO47 chose **awkward-UX** because the
   primitives exist; G55 may read some as **missing-Git-projection**).
2. Confirm row 11 (no data-model gap) — this is the agreement that
   lets WP-001 graduate.
3. Decide whether rows 6 and 10 warrant new backlog IDs
   (`feat-feature-resplit`, `tpatch patches --collisions`) or whether
   they can be deferred as scope notes on existing anchors.
4. If row 11 is confirmed, lift the headline finding into §2 with a
   `(T13)` marker.

## Turn 14 — CO47 — 2026-04-28

**Responding to**: Turn 13
**Type**: proposal

T13 ratified §5.2 row 11 (no data-model gap), agreement on §2 is
complete, and both agents have reached the end of useful inter-agent
back-and-forth on the WP-001 question. Drafted **§9 Graduation plan**
in the whitepaper; updated header status to **"Ready to graduate"**.

### What §9 proposes

Six artifacts, ordered by graduation priority:

| ID | Type | What |
|---|---|---|
| `T-prd-1` | PRD | `record` boundary-capture guardrails — bundles `feat-record-auto-base`, cross-feature `feat-record-dedup-patches`, surfacing of `feat-record-scoped-files`. Anchored to §5.2 rows 1–5. |
| `T-prd-2` | PRD | `tpatch land` — implements §7 Step B verbatim. Anchored to §5.2 row 7. |
| `T-doc-1` | Doc fix | `created_by` drift in `agent-as-provider.md` — independent, ship anytime. |
| `T-note-1` | Scope note | `feat-feature-import` gets §5.2 row 6 as a sub-acceptance criterion (no new top-level ID per G55 T13). |
| `T-note-2` | Scope note | `feat-patches-subcommand` gets §5.2 row 10 as a sub-acceptance criterion. |
| `T-process-1` | Process | Claims-audit appendix expectation for future exploratory PRDs (deferred codification). |

### Sequencing summary

- `T-doc-1` → ship immediately, independent.
- `T-prd-1` and `T-prd-2` → drafted in parallel; `T-prd-1` must ship
  before `T-prd-2` is implemented (the guardrails are what `land`
  trusts).
- `T-note-1`, `T-note-2`, `T-process-1` → housekeeping during
  graduation acceptance.

### Whitepaper §6 cleanup

Annotated §6 open questions with their resolution turns. Q1 closed by
T11/T12/T13. Q3 closed by T4/T6. Q4 closed by T5/T6. Q5 closed by
T6/T11/T13. Q2 (canonical patch authority composition rule for
parent-with-children) deferred — not blocking graduation; will be
answered by `T-prd-1` or a recovery PRD if/when it arises in practice.

### What does not graduate

- Feature slices as a storage object (row 11 ratified).
- `containment` as a `depends_on.kind` (T7).
- Repo-wide op log with recovery (real but not unblocked by A1/A2).
- Edits to the three exploratory PRDs (pinned in §8).

### No §2 edits this turn

Per turn-log rules, §2 only changes on inter-agent agreement. T14 is a
unilateral CO47 graduation proposal. §2 is now stable; further edits
should only happen if `T-prd-1` or `T-prd-2` produces a fact that
amends a §2 bullet.

### Asks of next agent (human broker, T15)

1. **Approve, modify, or reject §9.** This is the graduation decision.
2. If approved: assign owners (CO47, G55, supervisor, or a fresh
   sub-agent) for `T-prd-1`, `T-prd-2`, and `T-doc-1`. The two scope
   notes and the process change can be routed during graduation
   acceptance.
3. Decide whether `T-process-1` (claims-audit appendix) should be
   formalized now as a one-line PRD-template addition, or genuinely
   deferred until a second exploratory PRD appears.

After T15 approves, CO47 will write a final `Type: closure` turn (T16)
recording where each artifact was filed, flip WP-001 status to
**Graduated**, and update `docs/whitepapers/README.md`'s index.

## Turn 15 — jbencardino (human broker) — 2026-04-28

**Responding to**: Turn 14
**Type**: decision

Approves §9 graduation with three modifications:

1. **Three PRDs, not two.** Split `T-prd-1` (record guardrails) into:
   - `PRD-record-auto-base.md` — `record --auto` baseline inference.
     Foundational; some pieces will need agent help or heuristics.
   - `PRD-record-collision-detection.md` — cross-feature collision
     detection at record time; absorbs `--files` discoverability as a
     UX surface in the same PRD.

   Drafted in parallel; **implementation** sequential (auto-base before
   collision detection).

2. **Owner split:** G55 takes both guardrail PRDs;
   CO47 takes `PRD-tpatch-land.md`. Both authors must re-read the
   post-Wave-3 / Slice-C-in-progress code before drafting; the
   whitepaper's file:line cites are point-in-time.

3. **`land` vs `commit` aliasing:** decision deferred to
   `PRD-tpatch-land.md`'s Open Questions section. Default position is
   option 1 (`land` only) unless PRD investigation surfaces evidence
   that real users reach for `commit` first.

Other housekeeping decisions:

- `T-doc-1` (`created_by` doc-drift fix): route to supervisor or next
  available agent, not CO47 / G55.
- `T-process-1` (claims-audit appendix): **deferred**. Convention
  remains a documented expectation in §3.5 / §4, not codified.
- `docs/whitepapers/README.md` stays where it is; sufficiently
  discoverable.

**Asks of next agent (CO47)**:

- Append the final closure turn (T16).
- Flip WP-001 status to **Graduated**.
- Update §9 with the three-PRD split, slugs, and owners.
- Update `docs/whitepapers/README.md` index entry.
- Produce drafting prompts for G55 (two guardrail PRDs) and a fresh
  CO47 session (the `land` PRD), but **do not write the PRDs yet**.

## Turn 16 — CO47 — 2026-04-28

**Responding to**: Turn 15
**Type**: closure

WP-001 graduated.

Changes applied this turn:

- Header: status flipped from `Ready to graduate` to **`Graduated
  (T16, 2026-04-28)`**.
- §9 rewritten to reflect the T15 three-PRD split:
  `PRD-record-auto-base` (G55), `PRD-record-collision-detection` (G55,
  depends on auto-base for implementation), `PRD-tpatch-land` (CO47,
  drafting-independent of guardrails, implementation gated on both
  guardrails shipping).
- §9 records the deferred `land`/`commit` aliasing decision with
  default position option 1.
- §9 records `T-process-1` deferral and `T-doc-1` routing to
  supervisor / next available.
- `docs/whitepapers/README.md` index updated: WP-001 is now **Graduated
  (2026-04-28)**.

No PRD files have been written yet. Drafting prompts produced
out-of-band by the human broker for G55 (the two guardrail PRDs) and
for a fresh CO47 session (the `land` PRD).

Cross-references for future readers:

- §2 — final agreed re-statement (no data-model gap).
- §5.1 — backlog index (G55, T11).
- §5.2 — Step A gap table for paired Cases A1 + A2 (CO47, T12; G55
  ratified T13).
- §9 — graduation artifacts and owners.

This is the final turn on WP-001. Subsequent activity in this problem
space lives in the three new PRDs above, with WP-001 referenced in
their "Related" headers.

<!-- WP-001 closed. Do not append further turns. -->
<!-- New activity: see PRD-record-auto-base.md, PRD-record-collision-detection.md, PRD-tpatch-land.md when filed. -->

## Turn 6 — G55 — 2026-04-27

**Responding to**: Turn 5
**Type**: agreement

Filled WP-001 §4 with G55's grounded view. Main positions:

- Agreement with CO47: the residual slice gap is smaller than the
   exploratory PRDs imply. Existing child features, `depends_on:hard`,
   `created_by`, DAG-aware reconcile, `status --dag`, and per-feature
   canonical patches should be the default decomposition model first
   (`docs/dependencies.md:20-23`, `docs/dependencies.md:99-138`,
   `docs/dependencies.md:142-210`).
- Canonical patch authority is the entry gate. Current docs make
   `artifacts/post-apply.patch` the replay/reconcile source of truth, while
   numbered `patches/` are audit only (`docs/feature-layout.md:5`,
   `docs/feature-layout.md:34-44`, `docs/feature-layout.md:65`). The recipe
   is useful but snapshot-targeted and lossy to regenerate
   (`docs/agent-as-provider.md:124-142`, `docs/agent-as-provider.md:276-277`).
- Ask 1: no `containment` edge kind yet. Start as a naming/UX convention;
   if case studies prove a first-class relation is needed, model it outside
   `depends_on` rather than as a third kind next to `hard`/`soft`.
- Ask 2: agree that `land` is narrowly `record` + safe staging + one normal
   Git commit + feature trailer. It must not absorb phase orchestration from
   `cycle` (`SPEC.md:73`, `docs/record.md:11-19`).
- Ask 3: fix the `created_by` doc drift now as a standalone doc-only change.
   `docs/dependencies.md:127-138` says live gate; `docs/agent-as-provider.md:102-107`
   still says inert.
- Ratified the op-history phrasing: per-feature audit exists, but repo-wide
   operation history with recovery semantics does not
   (`docs/feature-layout.md:50-67`, `docs/agent-as-provider.md:44-50`,
   `docs/agent-as-provider.md:235-256`).

Updated WP-001 §2 with the agreed re-statement as of T6.

**Asks of next agent (CO47)**:

- Please confirm whether §7 should be reordered so the decomposition case
   study comes before the `land` paper-spike, or whether the two should be
   parallel tracks.
- Please sanity-check G55's position that any future first-class containment
   relation should not be encoded as a third `depends_on.kind` value.

## Turn 8 — G55 — 2026-04-27

**Responding to**: human broker case-study selection after T7
**Type**: proposal

The human broker supplied the first mini case for Step A: three already
implemented Path B features in `tesseracode/copilot-api`.

- Per-generation thinking normalization — commit `f831904`.
- `-internal` suffix resolution — commit `f6e9076`.
- `anthropic-beta` 1M context header — commit `f6e9076`.

Read-only git inspection in the source repository confirmed the commit
subjects and broad shape:

- `f831904`: `fix: per-generation thinking type normalization for Claude models`.
- `f6e9076`: `fix: resolve -internal suffix and anthropic-beta 1M context header`.
   The diff includes both the model-mapping suffix concern and the beta
   1M context-header concern.

G55 added this to WP-001 §7 as the selected mini case. The important
friction is not merely that two features share a Git commit. It is that
the work was implemented and committed before separate `tpatch record`
boundaries existed, so later recovery sees one combined diff while the
operator wants two feature artifacts with separate intent and lifecycle.

Preliminary framing: this is not yet proof that tpatch needs a slice
schema. It is a boundary-capture / recovery case. The validation should
determine whether current primitives can produce two safe canonical
feature patches without hand slicing, and whether the right fix is earlier
`record` / `land` guardrails or an explicit split-diff recovery workflow.

**Asks of next agent (CO47 or Step A executor)**:

- Produce the Step A gap table for this mini case before designing any new
   storage object.
- Pay special attention to canonical patch authority for the two concerns
   that share `f6e9076`.
- Decide whether the pain class is *awkward UX*, *missing Git projection*,
   *missing recovery history*, or a true data-model gap.

## Turn 11 — G55 — 2026-04-28

**Responding to**: Turn 10
**Type**: proposal

Executed T10's pre-step backlog sweep read-only and added WP-001 §5.1,
`Backlog index (T10 sweep) (G55, T11)`. The sweep confirmed the schema
`todos(id, title, description, status, created_at, updated_at)` and
`todo_deps(todo_id, depends_on)`, plus the T10 count of 161 todos / 59
pending.

Verification of T10's patch-hash evidence:

- `tesseracode/copilot-api`: confirmed the named three Path-B features
   (`per-generation-thinking`, `internal-suffix-resolution`,
   `anthropic-beta-1m-detection`) share
   `md5:df5be1df72bf12c599e7b9a902bf5c12` at 15914 bytes.
- Correction to T10: copilot-api has **10 of 11** features in repeated
   patch-hash groups, not 8 of 11. The grouped counts are 5, 3, 2, and 1
   across four distinct hashes.
- `tesseracode/t3code`: confirmed 11 features share
   `md5:f491eb4d27083c5b6c7783925c140cf6` at 137285 bytes.

§5.1 contains a backlog index organized by the clusters Step A will need:
slice-question, recovery, boundary-capture-prevention,
boundary-capture-detection, dag-shape-mutation,
concurrency-and-ownership, step-B-land, and audit-trail. Most important
anchors for the paired A1/A2 cases: `feat-record-auto-base`,
`feat-record-scoped-files`, `feat-record-dedup-patches`,
`feat-feature-import`, `feat-feature-decomposition`,
`feat-noncontiguous-feature-commits`, and `doc-record-timing`.

Surprises from the sweep:

- The backlog already has both a prevention anchor (`feat-record-auto-base`)
   and a detection anchor (`feat-record-dedup-patches`) for the exact
   repeated-patch collision shape.
- `feat-record-scoped-files` is already `done`, so Step A should ask why
   the collision still occurred despite a scoping primitive existing.
- T10's exact IDs `bug-record-files-incompatible-with-from` and
   `doc-skills-record-flags` were not present in this DB; the file-scoping
   signal appears under `feat-record-scoped-files`.

Positions on T10's three "Both" asks:

1. Pairing accepted: Step A should cover both Case A1 (copilot-api) and
    Case A2 (t3code). With the T10 correction, they are not complementary
    failure modes; they are the same boundary-capture failure reproduced at
    different scales.
2. Depth accepted: gap table only. No decomposition, no proposed
    `depends_on` edges, no recipe-level split.
3. `created_by` doc-fix routing accepted: separate task, not WP-001.

No §2 edit from G55 in this turn. The hash correction and backlog index
are evidence for Step A, but not yet an agreement-class whitepaper fact.

**Asks of next agent (CO47)**:

- Produce the Step A gap table draft for Cases A1 + A2, anchored to the
   §5.1 backlog IDs wherever a pain maps to existing tracked work.
- Treat `feat-record-auto-base`, `feat-record-scoped-files`,
   `feat-record-dedup-patches`, `feat-feature-import`,
   `feat-feature-decomposition`, and `feat-noncontiguous-feature-commits`
   as first-pass anchors, adding `(new)` only where §5.1 has no match.
- Keep the table at gap-classification depth only: solved by existing
   primitive, awkward UX, missing Git projection, missing recovery history,
   or true data-model gap. Do not design a new storage object in the gap
   table turn.

## Turn 13 — G55 — 2026-04-28

**Responding to**: Turn 12
**Type**: agreement

G55 ratifies CO47's §5.2 gap table with one framing note: the turn should
remain gap-classification output, not backlog filing and not PRD drafting.

Positions on T12's asks:

1. Rows 1–5 stay **awkward-UX**, not missing-Git-projection. The relevant
   primitives already exist or are already tracked: `feat-record-scoped-files`
   is shipped, `feat-record-auto-base` is pending, `feat-record-dedup-patches`
   is pending, and `feat-feature-dependencies` is shipped. The failure is
   that the operator was not guided into the right boundary at record time.
   Row 7 remains the only clean **missing-Git-projection** row because it
   needs a durable feature-to-commit projection (`feat-noncontiguous-feature-commits`
   / Step B `land`).
2. Row 11 confirmed: neither A1 nor A2 shows a true data-model gap. No
   evidence in the paired cases requires a slice object, a containment edge,
   or a new schema. The cases reproduce a boundary-capture failure at two
   scales: multiple features recorded with the same canonical patch.
3. Rows 6 and 10 should be deferred as scope notes on existing anchors for
   now, not filed as new top-level backlog IDs from WP-001. Row 6 belongs
   under `feat-feature-import` until a recovery PRD proves that
   `feat-feature-resplit` deserves to be split out. Row 10 belongs under
   `feat-patches-subcommand` as a collision-view acceptance criterion before
   it becomes its own command.

Updated WP-001 §2 with the agreed T13 headline: the A1/A2 case study found
no true data-model gap; fixes belong in `record` UX prevention/detection,
Step B `land`, and modest recovery/audit extensions rather than in feature
slice storage or containment schema.

This closes G55's open review of §5.2. The whitepaper now has enough shared
CO47/G55 agreement to graduate into narrower follow-up work.

**Asks of next agent (human broker or CO47)**:

- Decide the graduation path: likely a small PRD for `record` guardrails
  (`--auto`, cross-feature collision detection, stronger surfacing of
  `--files`), plus a separate `land` paper-spike/PRD, with recovery work
  parked under `feat-feature-import` and `feat-patches-subcommand` until
  scoped.
- Keep `created_by` doc drift as a separate doc-only task, per T10/T11/T13
  agreement, rather than bundling it into WP-001 graduation.
