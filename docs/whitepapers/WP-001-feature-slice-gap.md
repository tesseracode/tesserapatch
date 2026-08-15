# WP-001 — Feature-Slice Gap & Intent-VCS Direction

**Status**: Graduated (T16, 2026-04-28)
**Authors**: CO47, G55
**Started**: 2026-04-27
**Turn log**: [`WP-001-feature-slice-gap.turns.md`](./WP-001-feature-slice-gap.turns.md)
**Supersedes claims in** (does **not** edit them):
- [`docs/prds/PRD-intent-version-control-evaluation.md`](../prds/PRD-intent-version-control-evaluation.md)
- [`docs/prds/PRD-tpatch-git-primitive-mapping.md`](../prds/PRD-tpatch-git-primitive-mapping.md)
- [`docs/prds/PRD-feature-slices-and-nested-changes.md`](../prds/PRD-feature-slices-and-nested-changes.md)

## 1. Context

Three exploratory PRDs proposed (a) evaluating tpatch as an intent-aware
VCS superset of Git, (b) mapping current tpatch commands to Git primitives,
and (c) introducing a new "feature slice" object for decomposing oversized
features.

Two independent reviews (CO47, G55) audited those PRDs against the current
authoritative docs (`SPEC.md`, `docs/feature-layout.md`, `docs/record.md`,
`docs/reconcile.md`, `docs/dependencies.md`,
`docs/prds/PRD-feature-dependencies.md`, `docs/agent-as-provider.md`) and
the dependency-system code. Both reviews converged on a smaller-than-claimed
gap and on a `land`-spike rather than slice-implementation as the right
next experiment.

This whitepaper re-states the gap with cites, captures the points of
agreement and disagreement between the two reviewing agents, and proposes a
triage approach for related backlog items — without committing roadmap
scope and without editing the three exploratory PRDs.

## 2. Agreed re-statement of the gap

As of T13, CO47 and G55 agree on these working points. They are grounded
enough to guide the paper study, but not yet an implementation PRD:

- `depends_on:hard` child features + `created_by` op-level provenance +
  per-feature canonical `post-apply.patch` already cover **most**
  feature-slice use cases. The remaining question is not "how do we add
  sub-feature structure from zero?" but "which concrete oversized-feature
  pains remain after we use the existing DAG/provenance primitives fully?"
- The non-negotiable invariant is **canonical patch authority**: one
  replayable unit must have exactly one source-of-truth `post-apply.patch`.
  A parent/child or slice design cannot proceed until it says which unit
  reconcile trusts.
- A real but smaller op-log gap remains: per-feature audit trail exists
  (`patches/NNN-*.patch`, `apply-session.json`, `resolution-session.json`,
  `status.json:notes`); **repo-wide operation history with recovery
  semantics does not**.
- `land`, if studied, is a Git-projection / UX bridge: `record` + safe
  staging + one ordinary Git commit + a machine-readable feature trailer.
  It must not absorb phase orchestration from `cycle`.
- No new `containment` schema should be added before a case study proves
  that naming/UX conventions over existing child features are insufficient.
- **Sequencing (T7):** the decomposition case study runs **before** the
  `land` paper-spike. The case study constrains what `land` even needs to
  project (one commit per parent? per child? mixed?), so running them in
  parallel risks `land` solving the wrong shape.
- **Containment is not a `depends_on.kind` (T7):** if a first-class
  containment relation is ever needed, it lives in a separate field/edge
  type (e.g. `contained_by`), not as a third value next to `hard`/`soft`.
  `kind` is an apply-gate axis; containment is a part-of relation; they
  do not share a semantic axis.
- **Case-study finding (T13):** Cases A1/A2 found no true data-model gap.
  The repeated `post-apply.patch` collisions in `copilot-api` and `t3code`
  are boundary-capture failures. Fixes belong in `record` UX prevention and
  detection, the Step B `land` Git projection, and modest recovery/audit
  extensions — not in a feature-slice storage object, a containment edge,
  or a new schema.

## 3. CO47 view  *(CO47)*

> Drafted T3 + T5 of the turn log. Subject to revision after G55 fills §4
> and after the open questions in §6 are answered.

### 3.1 The slice gap is mostly a re-discovery

`PRD-feature-slices-and-nested-changes.md` proposes a new object — a
"feature slice" — that has its own patch, its own recipe, and an optional
local intent (`:130-144`, `:213-225`). The PRD's "Related" header
(`:7`) cites `docs/dependencies.md` and `PRD-feature-dependencies.md`,
but the body never asks whether a slice is just a hard-child feature
plus a thin parent contract.

The current primitives that already do most of the proposed work:

| Slice property the PRD wants | Existing primitive | Cite |
|---|---|---|
| Smaller-than-feature replayable unit | A child feature with its own canonical `post-apply.patch` | `docs/feature-layout.md:34-44` |
| Ordered execution / apply gating | `depends_on:hard` edges checked pre-mutation | `docs/dependencies.md:96-112` |
| Local intent grounding | Each child feature already has its own `request.md` / `spec.md` / `exploration.md` | `docs/feature-layout.md:9-28` |
| Op-level provenance back to parent | `created_by` on recipe ops, now a live gate | `docs/dependencies.md:114-138`, `internal/.../created_by_gate.go` |
| Reconcile granularity per sub-unit | Per-feature reconcile already runs per child; topological label overlay surfaces parent gating | `docs/dependencies.md:140-180` |
| Dependent-aware removal | `tpatch remove --cascade` (leaves first) | `docs/dependencies.md:217-228` |

What the dependency model does **not** give us:

- A first-class "this child is *contained in* parent X" relation, distinct
  from "this child *depends on* parent X". Containment and dependency are
  not the same; conflating them is one of the risks the slices PRD
  itself flags (`:316-322`). But this is a single missing relation, not
  a new object type.
- A canonical patch story when a parent and a hard child both want to
  describe overlapping ground. This is the open question §6.2 must
  answer.

So the residual slice gap is small enough that it probably resolves to:
either (a) a `containment` edge kind alongside `hard`/`soft`, or (b) a
naming/UX convention on top of unchanged primitives — **not** a new
storage object.

### 3.2 The intent-VCS framing conflates two different gaps

`PRD-intent-version-control-evaluation.md` and
`PRD-tpatch-git-primitive-mapping.md` argue tpatch should grow into an
intent-aware superset of Git. The argument bundles two gaps that should
be unbundled:

- **Data-model gap** — Git has no first-class intent, no replay model,
  no feature DAG. This gap is *largely already closed*: the dependency
  PRD shipped a hand-curated DAG with hard/soft edges, validation,
  topological reconcile, and label overlays
  (`docs/dependencies.md`, `docs/prds/PRD-feature-dependencies.md`).
  `created_by` adds op-level provenance. `post-apply.patch` per feature
  is the canonical replayable unit. What remains is mostly cosmetic:
  the dependency DAG is not yet projected into Git history except
  conventionally.
- **UX gap** — `tpatch` state and Git history are still operationally
  separate. The user runs `tpatch record`, then `git add`, `git commit`.
  Mistakes around ordering produce silent failures (the v0.4.2 anti-
  pattern refusal in `docs/record.md:38-50` exists precisely because
  this seam is sharp).

The `tpatch land` proposal is a UX-gap experiment. It should not be sold
as a step toward an intent-aware VCS — it doesn't add an intent layer,
because the intent layer is already there. It removes a seam.

### 3.3 The op-log claim, restated precisely

Both PRDs claim tpatch lacks an operation log. Per Turn 3 of the log,
that is half right. What ships today:

- `patches/NNN-<label>.patch` per feature with labels
  `started / done / record / cycle / reconcile`
  (`docs/feature-layout.md:51-67`).
- `apply-session.json` — per-apply audit
  (`docs/agent-as-provider.md:240-256` family).
- `resolution-session.json` — per-resolver-attempt audit (same).
- `status.json:notes` — manual phase-advance trail
  (`docs/agent-as-provider.md:43-50`).

What does **not** ship: a repo-wide, cross-feature, time-ordered log of
user-visible operations with recovery semantics (the `jj op log`
analogue). G55's T4 phrasing — *per-feature audit trail exists; repo-
wide operation history with recovery semantics does not* — is the
correct restatement and should land verbatim in §2 once ratified.

This matters because the cost calculus changes: we are not building an
op log from zero; we are evaluating whether to **promote** existing
per-feature artifacts into a queryable cross-feature view, and whether
**recovery** (undo a tpatch operation, not just a Git commit) is worth
the complexity.

### 3.4 `land` vs `cycle`

`tpatch cycle <slug>` already ships as the lifecycle-in-one-command
(`SPEC.md:73`). Neither exploratory PRD compares it to the proposed
`land`. Without that delta the proposal is not actionable.

CO47 position: `land` should be defined narrowly as **`record` + safe
staging + one Git commit with a `Tpatch-Feature: <slug>` trailer**, and
nothing else. `cycle` orchestrates *phases*; `land` projects state into
*Git history*. They compose; they don't overlap. If `land` ever grows
phase orchestration it has merged into `cycle` and should be folded back
in. This boundary belongs in the spike's dry-run contract (§7 step 1).

### 3.5 Process / quality observation about the PRDs

The two reviewing agents independently found the **same** mapping
incompleteness and the **same** patch-authority blind spot. That is not
two reviewers being thorough — it is a structural defect in how the
exploratory PRDs were written: they did not audit their own claims
against the docs in their own "Related" header.

Recommendation for the project, beyond this whitepaper:

- Any future exploratory PRD ships with a one-page **claims audit**
  appendix: each load-bearing claim about current behavior cites a
  file:line in the authoritative docs.
- An exploratory PRD that proposes a new object must include a
  pre-flight section "could this be done with existing primitives?",
  with concrete cites against `docs/dependencies.md`,
  `docs/feature-layout.md`, and `docs/agent-as-provider.md` at minimum.
- This is a process change, not a PRD-template change. Codifying it can
  wait until WP-001 closes.

### 3.6 Concrete asks of G55 for §4

To converge §2 efficiently, CO47 would like G55 to take positions on:

1. Is the residual slice gap better modelled as a new edge kind
   (`containment` alongside `hard`/`soft`) or as a UX convention with
   no schema change?
2. Does G55 agree that `land` should be scoped to "record + stage +
   one commit + trailer" and not absorb any phase orchestration from
   `cycle`?
3. Should the `created_by` doc-drift fix
   (`agent-as-provider.md` "inert" vs reality) be lifted out of this
   whitepaper into a small standalone doc-fix PR now, or held until
   WP-001 closes?

## 4. G55 view  *(G55)*

> Drafted T6 of the turn log. This section is G55's current position;
> it agrees with much of §3 but keeps a few sequencing cautions explicit.

### 4.1 Patch authority is the first invariant

G55 position: any feature-slice or parent/child design starts by preserving
the current replay authority rule, not by inventing new local artifacts.
The current docs are intentionally blunt: numbered `patches/` entries are
not current; `artifacts/post-apply.patch` is the canonical feature diff
(`docs/feature-layout.md:5`, `docs/feature-layout.md:34-44`), and the rule
of thumb is "Replay or reconcile -> `artifacts/post-apply.patch`"
(`docs/feature-layout.md:65`).

The recipe is valuable, but it is not the same kind of authority. It is
listed as the operation list the apply flow executes
(`docs/feature-layout.md:81`) and as a deterministic script targeting a
specific upstream snapshot (`docs/agent-as-provider.md:124-142`). The same
doc says to trust the patch when patch and recipe disagree, and warns that
whole-diff-to-op-list regeneration is lossy after resolver accept
(`docs/agent-as-provider.md:137-142`, `docs/agent-as-provider.md:276-277`).

Therefore a slice model that gives both the parent and children their own
`post-apply.patch` files is not merely adding structure; it is adding
multiple replay authorities. The slices PRD already spots this as the
dangerous ambiguity (`docs/prds/PRD-feature-slices-and-nested-changes.md:214-222`),
but G55 would strengthen that from "question to answer" to "entry gate":
no storage prototype until the canonical unit is defined.

### 4.2 The default decomposition model should be child features

G55 agrees with CO47 that the slice gap is smaller than the exploratory
PRDs imply. The current dependency model already defines child-to-parent
edges (`docs/dependencies.md:20-23`), two edge semantics (`hard` and
`soft`) (`docs/dependencies.md:35-42`), a pre-mutation hard-parent apply
gate (`docs/dependencies.md:99-105`), recipe-level `created_by`
provenance (`docs/dependencies.md:114-138`), DAG-aware reconcile ordering
and labels (`docs/dependencies.md:142-180`), and `status --dag` views
(`docs/dependencies.md:184-210`). The dependency PRD also records that
the real stress-test problem was feature composition: a child feature
editing code introduced by a parent, with ordering and reconcile failures
when the relationship lived only in the operator's head
(`docs/prds/PRD-feature-dependencies.md:32-60`).

That is close enough to a slice substrate that the burden of proof has
flipped. A new slice object must now show why a normal child feature with
its own request/spec/exploration, canonical patch, dependency edge, and
`created_by` hints is insufficient. The exploratory slices PRD lists
"feature slice" and "nested feature" as separate concepts
(`docs/prds/PRD-feature-slices-and-nested-changes.md:74-100`), but the
repo's shipped primitives already make "nested feature with dependency
semantics" the first model to try.

### 4.3 Containment should start as convention, not a new edge kind

Ask 1 from §3.6: G55 does **not** want `containment` added alongside
`hard` and `soft` yet.

The existing `kind` field is operational: `hard` gates apply and affects
ordering, while `soft` is informational (`docs/dependencies.md:35-42`,
`docs/dependencies.md:99-105`). A `containment` relation asks a different
question: "is this child part of a larger product intent?" That is not an
apply gate, and the slices PRD itself warns that confusing dependency and
containment is a design risk
(`docs/prds/PRD-feature-slices-and-nested-changes.md:320`).

For the case study, G55 would start with a naming and documentation
convention: a parent feature can act as the umbrella contract, and child
features can declare `depends_on:hard` when they truly require the parent.
If that proves too weak, the follow-up should consider a separate relation
such as `contained_by` or `group`, not a third dependency kind inside
`depends_on`.

### 4.4 `land` is a narrow Git projection, not a lifecycle command

Ask 2 from §3.6: G55 agrees with CO47's narrow `land` scope.

`cycle` already owns lifecycle orchestration: the spec describes it as
"Run full lifecycle in one command" (`SPEC.md:73`). `record` owns patch
capture (`SPEC.md:63`, `docs/record.md:3`), and the main sharp edge is the
manual sequence from `record` to `git add` / `git commit`
(`docs/record.md:11-19`) plus the clean-tree anti-pattern that requires
`--from` after a premature commit (`docs/record.md:39-45`). Reconcile also
documents two valid history patterns: features as patches on pristine main
and features as commits with `.tpatch/` as audit trail
(`docs/reconcile.md:18-36`).

So the only `land` worth studying now is `record` + safe staging + one
ordinary Git commit + a trailer such as `Tpatch-Feature: <slug>`. It should
not run analyze/define/explore/implement/apply, and it should not choose
hidden refs or metadata-only commits in the first paper spike. Its value is
to make the Git projection of an already-materialized feature auditable.

### 4.5 Operation history exists locally; recovery history does not

G55 ratifies CO47's restatement of the operation-log gap. The current
system has per-feature audit trails: numbered patch snapshots are written
with labels such as `record`, `started`, `cycle`, and `done`
(`docs/feature-layout.md:50-67`); Path B manual advancement leaves notes
in `status.json` (`docs/agent-as-provider.md:44-50`); resolver attempts
write `resolution-session.json` (`docs/agent-as-provider.md:235-256`).

But these artifacts do not yet form a repo-wide sequence of user-visible
operations with restore/undo semantics. That distinction matters. The next
question is not "do we have any operation history?" but "do we need to
promote existing per-feature audit artifacts into a cross-feature operation
history, and is recovery part of that contract?" G55 would not pursue this
until the feature-composition case study and `land` paper spike clarify
which mistakes users actually need to recover from.

### 4.6 `created_by` doc drift should be fixed outside WP-001

Ask 3 from §3.6: G55 would lift the `created_by` doc drift into a small
standalone doc-fix PR now, rather than holding it until WP-001 closes.

Reason: this is not a speculative whitepaper point. The user-facing
dependency doc says `created_by` is a live gate from v0.6.0 onward
(`docs/dependencies.md:127-138`), while `agent-as-provider.md` still says
the apply path does not branch on it and calls it "Currently inert"
(`docs/agent-as-provider.md:102-107`). `agent-as-provider.md` is a daily
workflow companion for agents, so leaving it stale risks bad recipes while
the whitepaper is still open.

This turn does not make that fix because WP-001's instruction is to fill
§4 and append T6 only. G55's recommendation is: assign a separate doc-only
change that updates `agent-as-provider.md` to match `docs/dependencies.md`,
without touching the exploratory PRDs.

### 4.7 Case-study order before backlog triage

G55 agrees that the whitepaper should eventually categorize the related
SQLite backlog, but only after one concrete decomposition pass. The first
study should take a historical oversized feature and model it using only:

- a thin parent feature that carries umbrella intent,
- 2-3 normal child features,
- `depends_on:hard` where ordering is real,
- `created_by` on child recipe operations that rely on parent-created
  paths,
- one canonical `post-apply.patch` per actual replayable feature.

The output should be a gap table: solved by current primitive, awkward UX,
missing Git projection, missing recovery history, or true data-model gap.
Only the last category should reopen slice storage or containment schema.

## 5. Backlog-triage matrix  *(to be filled in a later session)*

Once the gap re-statement is agreed, every related entry currently in the
session SQLite backlog should be triaged into one of three buckets:

| Bucket | Meaning | Action |
|---|---|---|
| **Subsumed** | Already covered by an existing primitive (`depends_on`, `created_by`, canonical patch, shadow worktree, Path B). | Close with cite. |
| **Gated on `land`** | Real, but cannot be evaluated until the `land` spike answers the UX gap. | Park behind `land` experiment. |
| **Independent** | Real and orthogonal to both `land` and the slice question. | Eligible for normal PRD intake. |

The matrix itself will be appended here once the inbox/backlog sweep runs.
For now this section is a contract, not data.

## 5.1 Backlog index (T10 sweep) *(G55, T11)*

Read-only sweep of `.tpatch-backlog/backlog.db` per T10 ask 1. Anchors
the gap table that Step A will produce.

Schema confirmed read-only: `todos(id, title, description, status,
created_at, updated_at)` and `todo_deps(todo_id, depends_on)`. The DB has
161 todos, 59 pending. One ID can belong to more than one cluster;
semicolon-separated clusters below are intentional.

| Backlog ID | Status | Cluster | One-line relevance to WP-001 |
|---|---|---|---|
| `bug-record-roundtrip-false-positive-markdown` | pending | boundary-capture-detection | Record validation can emit scary false-positive patch warnings while still writing a patch, weakening operator trust in recovered boundaries. |
| `bug-record-validation-false-positive` | done | boundary-capture-detection | Prior false-positive validation bug; still a dependency anchor for record/amend trust. |
| `doc-patches-vs-artifacts` | done | audit-trail | Documents numbered `patches/` as history vs `artifacts/post-apply.patch` as canonical authority. |
| `doc-record-timing` | done | boundary-capture-prevention | Documents the record-before-commit footgun and `--from` escape hatch that Cases A1/A2 stress. |
| `doc-reconcile-workflow` | done | recovery | Documents pre-reconcile git hygiene; depends on the record-validation fix. |
| `feat-agent-collision-detection` | pending | concurrency-and-ownership; boundary-capture-detection | Detects edits to feature-associated files by another agent/process before a patch boundary is trusted. |
| `feat-amend-dependent-warning` | pending | dag-shape-mutation; boundary-capture-detection | Surfaces stale child spec/recipe/patch labels after hard-parent amendment. |
| `feat-feature-amend` | pending | recovery; dag-shape-mutation | Allows in-tree fixes to already-recorded features without losing feature identity. |
| `feat-feature-autorebase` | pending | dag-shape-mutation; recovery | Attempts to rebase children after parent drift instead of leaving stale child baselines implicit. |
| `feat-feature-decomposition` | pending | slice-question | Direct backlog item for splitting a large feature into sub-features/epics. |
| `feat-feature-dependencies` | done | slice-question; dag-shape-mutation | Shipped hard/soft feature DAG primitive that WP-001 treats as the first decomposition model to try. |
| `feat-feature-import` | pending | recovery | Reverse-engineers long-running fork changes into tpatch features; closest existing anchor for after-the-fact recovery. |
| `feat-feature-prerequisites` | pending | audit-trail | Per-feature environment assumptions can affect whether replay/recovery verdicts are meaningful. |
| `feat-feature-provider-overrides` | pending | audit-trail | Per-feature provider/model pins are adjacent metadata, not a Step A blocker. |
| `feat-feature-removal` | pending | dag-shape-mutation | First-class abandon/remove command; bounds lifecycle behavior for split or wrongly captured features. |
| `feat-feature-reorder` | pending | dag-shape-mutation | Swaps parent-child order in the DAG; relevant if Step A shows captured boundaries need graph surgery. |
| `feat-feature-standalonify` | pending | dag-shape-mutation; recovery | Detaches a dependent feature from parents by reconstructing needed context. |
| `feat-feature-tested-state` | pending | audit-trail; boundary-capture-detection | Distinguishes applied-but-unverified recovery output from active/accepted feature state. |
| `feat-noncontiguous-feature-commits` | pending | step-B-land; recovery; audit-trail | Per-feature commit ledger for cases where one feature spans non-contiguous commits. |
| `feat-parallel-feature-workflows` | pending | concurrency-and-ownership | Per-feature worktrees could prevent cross-feature working-tree contamination before record. |
| `feat-patches-subcommand` | pending | audit-trail | CLI for listing/pruning/diffing numbered patch history against canonical artifacts. |
| `feat-recipe-migrate-to-templates` | pending | audit-trail | Migration path for recipe representation; adjacent to replay auditability, not Step A's central question. |
| `feat-recipe-schema-expansion` | pending | audit-trail | Richer recipe ops affect replay fidelity but do not by themselves solve patch-boundary capture. |
| `feat-recipe-template-ops` | pending | audit-trail | Template-backed recipe ops reduce recipe weight; adjacent to recipe reviewability. |
| `feat-record-auto-base` | pending | boundary-capture-prevention | `record --auto` base inference; likely prevention anchor for the A1/A2 whole-branch patch collisions. |
| `feat-record-autogen-recipe` | done | recovery; audit-trail | Path B record can generate a recipe after manual edits, preserving replay aid when implement was skipped. |
| `feat-record-dedup-patches` | pending | boundary-capture-detection; audit-trail | Byte-identical patch detection would surface repeated captures, though not necessarily prevent them. |
| `feat-record-scoped-files` | done | boundary-capture-prevention; recovery | `record --files` scopes capture when feature commits or file changes are interleaved. |
| `feat-delivery-modes` | pending | step-B-land | Per-feature delivery mode metadata may constrain what `land` should project. |
| `feat-ephemeral-mode` | pending | recovery | Depends on feature import; relevant mainly as a no-artifact lifecycle variant. |
| `feat-external-tpatch-store` | pending | concurrency-and-ownership | External metadata store affects who owns feature artifacts and where patch authority lives. |
| `feat-reconcile-code-presence-verdicts` | pending | recovery | Evidence-based verdicts can distinguish already-present code from missing/stale feature patches. |
| `feat-reconcile-fresh-branch-mode` | pending | recovery | Fresh-branch reapply mode could validate selected features from a clean upstream baseline. |
| `feat-resolver-dag-context` | pending | slice-question; recovery | Passes parent patch context to resolver for DAG children, relevant to child-feature decomposition. |
| `feat-richer-operation-types` | pending | audit-trail | Broader operation vocabulary may matter later for recovery history, but not before Step A's gap table. |

Dependency clusters worth knowing about (from `todo_deps`, arrow means
"todo depends on dependency"):

- boundary-capture-detection: `doc-reconcile-workflow` -> `bug-record-validation-false-positive`; `feat-feature-amend` -> `bug-record-validation-false-positive`.
- slice-question: `feat-feature-decomposition` -> `feat-agentic-tool-use`.
- dag-shape-mutation: `feat-feature-reorder` -> `feat-feature-dependencies`; `feat-feature-standalonify` -> `feat-feature-dependencies`; `feat-parallel-feature-workflows` -> `feat-feature-dependencies`.
- recovery: `feat-ephemeral-mode` -> `feat-feature-import`.
- audit-trail: `feat-recipe-migrate-to-templates` -> `feat-recipe-template-ops`.

Direct check against T10's named bug/doc trio: this DB contains
`feat-record-scoped-files` (`done`), but exact IDs
`bug-record-files-incompatible-with-from` and `doc-skills-record-flags`
were not present in the read-only sweep. Relevant title/description
misses reviewed but not indexed as Step A anchors included historical C1/C2
items, init/provider chores, and broader product ideas such as patch
marketplace and CI/CD integration.

### Post-snapshot issue cross-references — 2026-08-15

The table above remains the historical T10 read-only backlog snapshot; its rows
are not rewritten to claim later issue ownership.

- [GH #13](https://github.com/tesseracode/tesserapatch/issues/13) is adjacent
  to, but does not subsume, `feat-feature-autorebase`: it targets ADR-010
  phase-2 operation replay against upstream drift, while autorebase remains the
  parent-feature drift problem defined by ADR-011.
- [GH #12](https://github.com/tesseracode/tesserapatch/issues/12) is a new
  absorption/compaction research ticket and was not one of the 161 T10 rows.
- [GH #14](https://github.com/tesseracode/tesserapatch/issues/14) consumes the
  existing `feat-feature-reorder` and `feat-feature-standalonify` questions but
  does not mark either done.

## 5.2 Step A gap table — Cases A1 + A2  *(CO47, T12)*

Read-only retrospective. Anchored to §5.1 backlog IDs. Classification
vocabulary: **solved-by-primitive** / **awkward-UX** / **missing-Git-projection**
/ **missing-recovery-history** / **true-data-model-gap**.

### Evidence base

- **Case A1** — `tesseracode/copilot-api/.tpatch/features/`. 11 features,
  4 distinct patch contents, 10 of 11 features in collision groups
  (T11 verified). The `df5be1df…` group of 3 features
  (`per-generation-thinking`, `internal-suffix-resolution`,
  `anthropic-beta-1m-detection`) all share `base_commit
  ad9eef73f1b…`, all touch the same 4 files
  (`src/lib/model-mapping.test.ts`, `src/lib/model-mapping.ts`,
  `src/routes/messages/handler.ts`,
  `src/services/copilot/forward-native-messages.ts`), and all have
  `depends_on: None` despite obvious topical overlap.
- **Case A2** — `tesseracode/t3code/.tpatch/features/`. 11 features
  share `f491eb4d…` (137285 bytes). The shared patch touches ≥21 files
  spanning `apps/server/`, `apps/web/`, and `.claude/`. Slugs span
  unrelated themes (`copilot-cli-provider`, `effort-theming`,
  `readme-copilot-notice`, `copilot-cross-platform-build`,
  `copilot-skill-discovery`, …) — no plausible reading where these are
  one feature.

### The gap table

| # | Pain (observed) | A1 | A2 | Classification | Backlog anchor(s) | Notes |
|---|---|---|---|---|---|---|
| 1 | Multiple features captured the same patch byte-for-byte | yes | yes | **awkward-UX** | `feat-record-dedup-patches` (detection); `feat-record-auto-base` (prevention); `feat-record-scoped-files` (done, prevention) | The detection anchor exists but is not shipped; the scoping anchor is shipped but was not used. UX, not data-model. |
| 2 | All features in the collision group share the same `base_commit`, so `record` defaulted to a too-broad diff | yes | yes | **awkward-UX** | `feat-record-auto-base` | A1's three features all baseline at `ad9eef73f1b…`; A2 likely the same shape against `upstream/main`. The footgun is documented (`docs/record.md:38-50`, `doc-record-timing` done) but not enforced. |
| 3 | `record` had no signal that "this patch is identical to feature X's patch" | yes | yes | **awkward-UX** | `feat-record-dedup-patches` (cross-feature variant) | Backlog item is intra-feature dedup; cross-feature collision is a natural extension, may need a (new) sub-item. |
| 4 | No `depends_on` declared between collision-group features despite topical overlap (A1: model-mapping group; A2: provider-stack group) | yes | yes | **awkward-UX** | `feat-feature-dependencies` (done) | The primitive ships; nothing prompts the operator to use it during `record`. Not a data-model gap. |
| 5 | `record --files` exists and would have scoped the capture, but operator did not reach for it | yes | yes | **awkward-UX** | `feat-record-scoped-files` (done) | Tool exists; discoverability/default behavior is the gap. Reinforces T10's observation. |
| 6 | After-the-fact: no command to take colliding features and re-derive distinct canonical patches | yes | yes | **missing-recovery-history** | `feat-feature-import` | `feat-feature-import` targets reverse-engineering raw fork → features; the harder case is reverse-engineering *recorded-but-collided* features. Possibly a new sub-item: `feat-feature-resplit`. |
| 7 | No per-feature commit ledger to anchor what each feature *should* contain when commits are non-contiguous | yes | yes | **missing-Git-projection** | `feat-noncontiguous-feature-commits` | This is exactly the Step B (`land`) territory: if `land` had created per-feature commits at intent-capture time, the collision could not have happened. Confirms the T9/T10 hypothesis that the gap is UX/projection, not slices. |
| 8 | `reconcile` against any one collision-group member would replay every other member's edits | yes | yes | **awkward-UX** (consequence of #1–#3) | `feat-reconcile-code-presence-verdicts`; `feat-reconcile-fresh-branch-mode` | These verdicts could surface "this feature's patch is identical to feature X's" at reconcile time, but the right fix is upstream at `record`. |
| 9 | `feat-feature-decomposition` (the explicit "slice" backlog item) does not solve either case | yes | yes | n/a (negative finding) | `feat-feature-decomposition` | Decomposing one big feature into sub-features would not have prevented either collision; the failure is recording **boundaries**, not splitting **content**. **This is the central paper-only finding for WP-001.** |
| 10 | Audit trail (`patches/NNN-*.patch`) does not surface cross-feature collisions either | yes | yes | **missing-recovery-history** (cross-feature view) | `feat-patches-subcommand` | Per-feature audit exists (T6/§2 ratification); a `tpatch patches --collisions` view would land cleanly here. |
| 11 | No data-model gap observed in either case | — | — | **true-data-model-gap = none** | — | Neither case requires a slice object, a containment edge, or a new schema. Hard children + `created_by` + canonical patch suffice — but the operator never got to use them because step 1 (recording the right boundary) failed. |

### Summary by classification

| Classification | Count | Highest-leverage anchor |
|---|---:|---|
| awkward-UX | 6 (rows 1–5, 8) | `feat-record-auto-base` + cross-feature variant of `feat-record-dedup-patches` |
| missing-Git-projection | 1 (row 7) | `feat-noncontiguous-feature-commits` (Step B `land` precondition) |
| missing-recovery-history | 2 (rows 6, 10) | `feat-feature-import` extension; `feat-patches-subcommand` collisions view |
| true-data-model-gap | 0 (row 11) | — |

### Headline finding for §2

**No row in either case requires a new data-model object.** The
exploratory slices PRD's "feature slice" object is not required to fix
A1 or A2. The fixes live entirely in `record` UX (prevention +
detection), in the `land` Git-projection that Step B proposes, and in a
modest extension of existing recovery primitives. Row 9 is the
strongest single piece of evidence: the explicit `feat-feature-decomposition`
backlog item — the closest thing tpatch already has to "slices" — would
not have prevented either case.

### Asks of next agent (G55) before §2 is updated

1. Sanity-check rows 1–5: do you read these as **awkward-UX** or as
   **missing-Git-projection**? CO47 chose UX because the primitives
   exist; G55 may legitimately argue some of them are projection gaps.
2. Confirm row 11 (no data-model gap) is correct against your evidence.
   This is the bullet that, if ratified, lets WP-001 graduate.
3. Decide whether rows 6 and 10 warrant **new** backlog IDs
   (`feat-feature-resplit`, `tpatch patches --collisions`) or whether
   they can be deferred as scope notes on the existing anchors.

### Amendment — 2026-05-01 — Re-verification on current case-study state

The Evidence base above is a point-in-time snapshot from §5.2's
original authorship. The case-study repos have grown since; this
amendment captures the re-verification numbers without restating
the gap-table classifications (they hold — larger numbers, same
shape, no row reclassified).

**Method.** Per case-study repo: hash every `post-apply.patch`
and group identical bytes. For each duplicate group, read every
member's `feature.yaml` for `base_commit`.

```
for f in .tpatch/features/*/artifacts/post-apply.patch; do
  shasum -a 256 "$f"
done | sort | uniq -c -d -w 64 -f 0
```

**Case A1 — `tesseracode/copilot-api`.**

- Features with patches: **21** (was 11 at original §5.2 authorship).
- In collision groups: **9 (43%)** across **3 groups** of sizes
  **5 / 2 / 2**.
- The 5-feature group shares `base_commit ac4fefd337d4…` whose
  commit subject reads literally `feat: implement 5 cosmetic
  features from tpatch stress test`. Captured from a single
  pre-existing commit without `--from HEAD~N` scoping — textbook
  record-from-pre-commit failure.
- The original `df5be1df…` 3-feature group
  (`per-generation-thinking`, `internal-suffix-resolution`,
  `anthropic-beta-1m-detection`, base `ad9eef73f1b…`) is preserved
  in the 2026-05-01 measurement. The two 2-feature groups
  discovered on re-verification were not present at original
  authorship (or were not noted).

**Case A2 — `tesseracode/t3code`.**

- Features with patches: **17** (was 11 at original §5.2 authorship).
- In one collision group: **11 (65%)** — single byte-identical
  group, **137 285 bytes**, **27 files** spanning `apps/server/`,
  `apps/web/`, and `.claude/`.
- Shared `base_commit`: `02bcb16d06e8…`.
- The patch's own `.claude/instructions.md` instructs the agent to
  `tpatch record --from main` — the failure pattern is encoded
  *as guidance* inside one of the files the patch itself
  overwrites. The most-explicit form of the awkward-UX rows 1–3.

**Headline finding (unchanged).** The §5.2 gap-table classifications
hold without modification. No row needs reclassification. The
"no data-model gap" verdict in §2 is reinforced at the larger
scale: the same fixes (record UX prevention + detection) close
all 14 collisions (5 + 2 + 2 in A1, 11 in A2) across both repos.

**Pointer.** This amendment was authored during the 2026-05-02 /
2026-05-03 exploratory market-research session. The market-
research artifact that consumes these numbers is
[`docs/market-research/competitive-landscape.md`](../market-research/competitive-landscape.md)
(SWOT internal weakness; SMART measurable target). ADR-015 cites
them in `Context` as evidence for D2 / D3.

## 6. Open questions

Synthesized from both reviews. Each will be answered in the turn log
before being lifted into §2.

1. ~~Does any **concrete historical oversized feature** fail to decompose
   using only `depends_on:hard` + `created_by`? If yes, where exactly?~~
   **Answered (T11/T12/T13)**: Cases A1 + A2 demonstrate the failure is
   *boundary capture at record time*, not decomposition. No data-model
   gap. See §5.2 row 11.
2. What is the precise definition of "canonical patch authority" when a
   feature has hard children: parent-only, child-only, or both with a
   well-defined composition rule? **Deferred** — not blocking
   graduation; will be answered by the recovery PRD if/when row-6
   workflows expose the composition question.
3. ~~Is the op-log gap really cross-feature/repo-wide history?~~
   **Answered (T4/T6)**: yes — per-feature audit trail exists; repo-wide
   operation history with recovery semantics does not.
4. ~~What is `tpatch land` *exactly*?~~ **Answered (T5/T6)**: `record` +
   safe staging + one Git commit + `Tpatch-Feature:` trailer. Not phase
   orchestration.
5. ~~`created_by` doc drift?~~ **Answered (T6/T11/T13)**: routed to a
   separate doc-only task (see §9 graduation artifact `T-doc-1`).

## 7. Recommended next experiment  *(provisional — both agents, ordered as of T7)*

Two paper exercises in **strict order**. Step B does not begin until step
A produces an output table.

### Step A — Decomposition case study (paper only) — ✅ Complete (T11/T12/T13)

Output: §5.2 gap table for paired Cases A1 (copilot-api) + A2 (t3code),
anchored to §5.1 backlog index. Headline ratified in §2 last bullet.

Original scope (kept for historical context):

> Pick **one** historical oversized feature. Retroactively model it
> using only existing primitives: thin parent feature, 2–3 child
> features, `depends_on:hard`, `created_by`, one canonical
> `post-apply.patch` per replayable feature. Produce a gap table.

What actually happened (T9 → T13): the human broker paired two real
in-the-wild repos showing the same boundary-capture failure rather
than retroactively decomposing one feature. The substituted shape
turned out to be more informative than the original, because both
repos exhibited the *same* pre-record failure mode — proving the gap
is not in decomposition primitives at all.

#### Mini case: Path B split-after-commit (T8 — superseded by §5.2 evidence base)

The T8 mini case (copilot-api commits `f831904`/`f6e9076`) is
subsumed by §5.2 Case A1, which shows the colliding-patch evidence
across all three Path B features. Kept as historical context;
authoritative analysis lives in §5.2.

### Step B — `land` paper-spike — Now graduates to its own PRD (see §9)

Per T13, Step B no longer needs to run inside WP-001. The Step A gap
table specifies exactly what `land` must project (per-feature commit
boundaries, with `Tpatch-Feature:` trailer, capable of preventing the
A1/A2 collision shape). That is enough to scope a focused PRD without
running the paper-spike inside this whitepaper first.

Original Step B contract (preserved for the PRD to consume):

1. Dry-run contract: list exact code files, `.tpatch/features/<slug>`
   files, patch hash, recipe hash if present, proposed commit
   message / trailer.
2. Manual fixture run: `record`, stage only the intended code plus
   feature metadata, one ordinary Git commit with `Tpatch-Feature:
   <slug>` trailer.
3. Verify `status`, `record --from`, and `reconcile` remain
   explainable in **both** Pattern A and Pattern B from
   `docs/reconcile.md`.

No slices. No hidden refs. No new operation log.

## 8. Out of scope (pinned)

- Editing the three exploratory PRDs.
- Active Wave 3 verify/freshness implementation.
- Implementing slices, `land`, or an op log.
- Replacing or wrapping Git transport.

## 9. Graduation plan  *(approved by human broker, T15 — closed T16)*

WP-001 has reached its purpose: the gap is re-stated, both agents agree
on §2, and the headline finding (no data-model gap) is ratified (T13).
The whitepaper graduates into the artifacts below.

### Approved graduation artifacts

| Slug | Type | Owner | Title | Depends on |
|---|---|---|---|---|
| `PRD-record-auto-base.md` | PRD | G55 | `record --auto`: infer baseline from `upstream.lock` / merge-base | — (foundational; ships first) |
| `PRD-record-collision-detection.md` | PRD | G55 | Cross-feature `post-apply.patch` collision detection at record time; surfaces `--files` UX as response | `PRD-record-auto-base` (logically; auto-base output is what dedup compares against) |
| `PRD-tpatch-land.md` | PRD | CO47 | `tpatch land`: per-feature Git projection (`record` + safe staging + one commit + `Tpatch-Feature:` trailer) | None for drafting; must not be implemented until guardrails ship |
| `T-doc-1` | Doc fix | supervisor / next available | `created_by` drift in `agent-as-provider.md:102-107` | — (independent, ship anytime) |
| `T-note-1` | Scope note | applied during graduation acceptance | Add §5.2 row 6 as sub-acceptance criterion to existing `feat-feature-import` backlog | — |
| `T-note-2` | Scope note | applied during graduation acceptance | Add §5.2 row 10 as sub-acceptance criterion to existing `feat-patches-subcommand` backlog | — |

### Pre-drafting requirement

Both PRD authors must re-read the **post-Wave-3 / Slice-C-in-progress**
code and docs before writing. Current relevant areas:

- `internal/cli/record*.go`, `internal/cli/cobra.go` for the `record`
  command surface
- `internal/store/`, `internal/gitutil/` for baseline / patch handling
- `internal/workflow/` for any orchestration that touches `record`
- `docs/record.md`, `docs/feature-layout.md`, `docs/dependencies.md`

The whitepaper's file:line cites are point-in-time. Verify before relying.

### Deferred decisions (recorded for the PRDs to inherit)

- **`land` vs `commit` aliasing.** Three options exist (`land` only;
  `commit` as alias; `commit` as canonical with `land` as alias).
  Decision **deferred** to `PRD-tpatch-land.md`'s Open Questions
  section per T15. Default position: option 1 (`land` only) unless
  the PRD investigation shows real users reach for `commit` first.

### What does **not** graduate

- **Feature slices as a storage object.** §5.2 row 11 ratified (T13).
  The slices PRD's central proposal stands superseded by WP-001's §2.
- **A new `containment` edge kind.** §2 (T7) ratified.
- **A repo-wide operation log with recovery semantics.** Real gap (§2,
  T4/T6), but not unblocked by either A1/A2 case. Defer until a
  concrete recovery scenario *requires* it.
- **Editing the three exploratory PRDs.** Pinned out of scope (§8).
  WP-001 is listed in their "Supersedes" header (one-way link).
- **`T-process-1` (claims-audit appendix).** Human broker chose to
  defer formalization (T15) until a second exploratory PRD shows the
  pattern recurs. Convention remains documented in §3.5 / §4 as an
  expectation, not a template.

### Sequencing

`T-doc-1` ships anytime, independent.

`PRD-record-auto-base` and `PRD-record-collision-detection` may be
**drafted in parallel** by G55. Implementation order is sequential:
auto-base ships first, collision detection layers on it.

`PRD-tpatch-land` may be drafted in parallel with both guardrail PRDs
by CO47. Its **implementation** must wait until both guardrail PRDs
ship — `land` projects feature state into Git history and that
projection is only safe once boundary capture is reliable.

`T-note-1` and `T-note-2` applied during graduation acceptance, not as
separate work items.

### Whitepaper status after graduation

WP-001 stays in `docs/whitepapers/` as historical context. The new
PRDs link back to it under their own "Related" headers.
`docs/whitepapers/README.md` index reflects the **Graduated** status.
