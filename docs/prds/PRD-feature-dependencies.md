# PRD — Stacked Feature Dependencies (DAG) — `feat-feature-dependencies`

**Status**: Draft (v2 — revised after rubber-duck review)
**Date**: 2026-04-24
**ADR**: **ADR-011-feature-dependencies.md — REQUIRED before M14.1 coding starts.**
**Owner**: Core
**Milestone**: M14 (new — this PRD defines it)

## 0. Meta

### 0.1 Architecture decisions to lock in ADR-011

This PRD records the _what_ and the _why-now_. Per repo rule (`AGENTS.md` → "Context Preservation Rules" §6), every architecture choice here must be captured in ADR-011 before any M14 implementation begins. The ADR itself is the first task of M14.1 and is **not** the author of this PRD's responsibility — but it must exist before code lands.

Decisions the ADR is required to record and justify:

1. **Canonical storage.** `depends_on` lives exclusively in the existing `status.json` (Option A in §3.1). No new `feature.yaml` file. Alternative considered: dual-file with migration. Rejected for file-surface minimisation and zero-migration rollout.
2. **Cycle detection algorithm.** DFS with WHITE/GREY/BLACK colouring. Alternative: Tarjan's SCC. Rejected — we need the path for the error message, not just a yes/no.
3. **Traversal algorithm.** Kahn's topological sort with alphabetical tie-break. Alternative: DFS postorder. Rejected per §4.4 rationale (ready-set maps 1:1 to operator-facing frontier).
4. **`waiting-on-parent` is a derived label, not a persisted state.** Composable with any `FeatureState`. Alternative: new persisted state. Rejected — produces drift sources and a second write path.
5. **`created_by` gating is hard-only.** Soft parents emit warnings, never errors. Alternative: all-kinds gating. Rejected — violates the soft/hard split defined in §3.2.
6. **`upstream_merged` satisfies hard dependencies.** A parent whose changes now live in pristine upstream is by construction "already applied." Codified in the §3.2 truth table.
7. **Remove requires explicit `--cascade` when dependents exist.** Silent orphaning is forbidden; `--force` alone is insufficient (see §3.7).
8. **M12 resolver does NOT receive parent-patch context in v0.6.** The DAG planner sequences resolver invocations across the stack, but each shadow-resolution call sees only its own feature. Parent-aware resolution is deferred to `feat-resolver-dag-context` (§9).

Any deviation from the above during implementation requires an ADR amendment before merging.

---

## Summary

Today every feature under `.tpatch/features/<slug>/` is treated as an independent patch against the upstream base commit. In practice, features stack: feature B edits code that feature A introduces, or B's recipe assumes symbols A defined. The CLI has no way to express this, which has produced real, repeatable failures in the v0.4.2 stress test (below).

This PRD defines a first-class dependency graph: a `depends_on` block on each feature, hard vs. soft semantics, fail-fast apply, topological reconcile, DAG-aware `status`, and a new `waiting-on-parent` transient state. It also locks a recipe-level `created_by` hint so "file not found" errors triggered by missing parents become actionable instead of mysterious.

It does **not** ship auto-rebase, per-dep version pinning, or delivery-mode stack semantics — those are acknowledged cross-cutting concerns pushed to named follow-ups.

---

## 1. Problem statement

### Live evidence (v0.4.2 stress test, 2026-04-22)

A real three-feature DAG was observed while building against `tesseracode/copilot-api`:

```
copilot-cli-provider           (root — creates CopilotProvider.ts)
├── copilot-dynamic-models     (HARD  — modifies CopilotProvider.ts)
└── effort-theming             (SOFT  — conditions on `provider === "copilot"`)
```

Three concrete failures followed from tpatch having no concept of dependencies:

1. **Phantom "file not found".** `copilot-dynamic-models` has `replace-in-file` ops targeting `CopilotProvider.ts`. Applied on a pristine checkout the recipe fails with a generic filesystem error — nothing distinguishes "missing target" from "parent feature not yet applied."
2. **Wrong reconcile verdict.** If upstream merges `copilot-cli-provider` but `copilot-dynamic-models` is reconciled first (or alone), reconcile reports `blocked-upstream-drift` for the child when the true state is *waiting for the parent's reconcile to land*. Operators then chase a phantom drift.
3. **Fragile ordering workaround.** The only way to ship today is to `git rebase -i` the fork until commits are in dependency order, then `tpatch record` each feature over a commit range that happens to match the logical boundary. One reorder mistake silently corrupts every downstream patch.

### Why this matters now

The conflict-resolver (M12, v0.5.0) made single-feature reconcile good enough to trust. The next real-world blocker is **feature composition**. Every other open backlog item — decomposition, standalonify, delivery modes, parallel workflows, patch-compat — either depends on a dep graph or would be half-baked without one. Evidence from the stress test is strong enough to move this from "needs a PRD" to "is the next implementation milestone after Tranche C ships."

### Why not "just rebase and record"

The rebase workaround is a project-management hack, not a product feature. It:
- leaves logical dependency information only in the user's head,
- breaks silently the moment commits move around,
- gives the reconciler no way to compose verdicts across a stack,
- cannot express soft dependencies (informational-only) at all.

A declarative DAG fixes all four.

---

## 2. Goals / non-goals

### Goals

- Declarative `depends_on` block on every feature, validated on write.
- Hard vs. soft dependency semantics that diverge *only* where it matters (apply fail-fast, reconcile verdict composition).
- Topological traversal in `reconcile` and `apply`, with deterministic ordering on ties.
- Cycle detection with a clear, actionable error (refusal, not silent).
- `tpatch status --dag` / `tpatch status --dag --json` surfaces the graph and ready-to-apply frontier.
- New `waiting-on-parent` outcome distinguishes dep-gated features from genuinely blocked ones.
- Recipe `created_by` hint turns phantom "file not found" into "will be created by parent X; apply X first."
- Skills declare dependencies during `analyze`.

### Non-goals (explicit)

- **Auto-rebase on parent drift.** If A's patch changes after B's record, we warn and mark B as requiring reconcile — we do *not* auto-edit B. Tracked as `feat-feature-autorebase`.
- **Per-dep version pinning.** `depends_on[].since` is captured (see §3) but not enforced in v1; consumed later by `feat-patch-compatibility`.
- **Delivery-mode stack semantics** (ship whole chain as one PR, build-artifact from tip, etc.). This PRD specifies the data model the delivery layer will consume; the delivery behaviour itself is out of scope (owned by `feat-delivery-modes`).
- **Auto-decomposition of oversized features.** Inverse problem; see `feat-feature-decomposition`.
- **Commit-range capture (`record --from/--to`).** Stress-test wrinkle B is intentionally split off into `feat-record-scoped-files` — it is useful independently and would balloon this PRD.
- **Parallel reconcile across independent DAG branches.** Sequential in v1; logged as `feat-parallel-feature-workflows`.

---

## 3. User-facing contract

### 3.1 Schema: `depends_on` block

**Canonical storage: `status.json`** (the file that `docs/feature-layout.md` already names as authoritative machine state). No new top-level file. This is Option A from the review — justification and rejected alternative recorded in §0.1 and ADR-011.

Rationale:

- `status.json` is already the only file tpatch commands write to for machine state; extending it adds zero new parse paths.
- Zero migration surface: legacy features that predate M14 simply have no `depends_on` key. Readers treat that as the empty list.
- One source of truth collapses the reviewer's "yaml vs json" footgun entirely — there is no mirror, so there is nothing to drift.
- `tpatch add --depends-on …` and `tpatch amend --depends-on …` write directly to `status.json`. No yaml parser is added to the binary.

```json
{
  "slug": "copilot-dynamic-models",
  "state": "requested",
  "depends_on": [
    { "slug": "copilot-cli-provider", "kind": "hard", "since": "copilot-cli-provider@post-apply:abc123" }
  ],
  "dependents": []
}
```

For reference, the parent in this stress-test DAG (`copilot-cli-provider`) has the inverse shape — empty `depends_on`, non-empty `dependents`:

```json
{
  "slug": "copilot-cli-provider",
  "state": "applied",
  "depends_on": [],
  "dependents": ["copilot-dynamic-models", "effort-theming"]
}
```

Note that `dependents` lists **children** (features that declare this one as a parent), never siblings and never the feature itself. It is derived; see below.

Field reference:

| Field | Type | Required | Rationale |
|---|---|---|---|
| `slug` | string | yes | Target feature slug. Must exist under `.tpatch/features/`. Self-reference is rejected at validate time. |
| `kind` | `"hard"` \| `"soft"` | yes | Drives apply fail-fast and reconcile verdict composition. No default: forces the author to think about it. |
| `since` | string | no | Pointer to the parent's recorded baseline at the moment the dep was declared. Format: `<parent-slug>@post-apply:<sha>` where the sha is the content-hash of the parent's `artifacts/post-apply.patch`. v1 records it; does not yet enforce it. v2 (patch-compat) uses it to detect parent drift. |

`dependents` is the inverse edge, computed by the store on refresh. It is never authored by hand.

**Backward-compat invariant (locked):**

> A feature whose `status.json` has no `depends_on` key, or an empty `depends_on: []`, is semantically identical to a pre-v0.6 feature with no dependencies. All lifecycle verbs (`apply`, `reconcile`, `status`, `amend`, `remove`) behave exactly as in v0.5.1 for such features.

This is enforced by a dedicated regression test in §7 acceptance: a repo upgraded from v0.5.1 with N features, none declaring deps, must apply/reconcile/status/amend/remove **byte-identically** to v0.5.1 after the v0.6.0 upgrade (excluding the `depends_on`/`dependents` keys added to `status.json` on next write, which default to absent/empty).

**Rollout safety — atomic release.** M14 is shipped as **a single atomic v0.6.0 release**. There is no intermediate "schema-only v0.5.2" that ships the data model without the planner. The whole DAG behavior is additionally feature-flagged behind the config key `features.dependencies` (default `false` until M14.4 lands), so trunk builds between M14.1 and M14.4 remain production-safe — the flag is flipped to `true` in the same commit that cuts v0.6.0. Acceptance (§7) verifies both that the flag gates all new behavior and that the flag default flips exactly once.

### 3.2 Semantics — canonical truth table

This table is the **single source of truth** for dep semantics across lifecycle verbs. All prose in §§4.3, 4.5, 4.6, 8 defers to it. Any disagreement between prose and this table is a bug in the prose.

**Legend:**

- Parent state: value of the parent's `FeatureState` (see `internal/store/types.go`).
- A hard dep is **satisfied** iff parent state ∈ `{applied, active, upstream_merged, reapplied}`.
  - `reapplied` is not a distinct persisted state today; it is the transient outcome `ReconcileReapplied` that leaves the parent in `applied` at rest. We include it in the satisfaction set for the brief window during a reconcile sweep where the planner evaluates children mid-flight (§4.5 step 3).
- A soft dep never gates; it only annotates.
- `-` means "no effect" (current behavior unchanged).
- `W` = warning on stderr (non-fatal); `E` = error (exit 2); `L` = derived label surfaced in `status` only.

**Table 3.2-A: hard dependency × parent state × verb**

| Parent state | `apply` | `apply --dry-run` | `reconcile` | `remove parent` | `amend parent` | `status` |
|---|---|---|---|---|---|---|
| `requested` | E `ErrHardDepUnmet` | E `ErrHardDepUnmet` (downgrade to W with `--force`) | child emits `waiting-on-parent`, skipped | E `ErrHasDependents` unless `--cascade` | W `stale-parent-*` labels on child (see §3.7) | L `waiting-on-parent` on child |
| `analyzed` | E `ErrHardDepUnmet` | E same | child `waiting-on-parent` | E same | W same | L same |
| `defined` | E `ErrHardDepUnmet` | E same | child `waiting-on-parent` | E same | W same | L same |
| `implementing` | E `ErrHardDepUnmet` | E same | child `waiting-on-parent` | E same | W same | L same |
| `applied` | ✅ proceed | ✅ proceed | ✅ run child reconcile | E same (even though satisfied — dependents still exist) | W on recipe/patch amend (§3.7) | ✅ no label |
| `active` | ✅ proceed | ✅ proceed | ✅ run child reconcile | E same | W same | ✅ no label |
| `upstream_merged` | ✅ proceed (parent changes live in upstream; `created_by` stays truthful) | ✅ proceed | ✅ run child reconcile | allowed — parent can be removed once upstream merges it and dependents are decoupled; dep validation re-checks | W only on `apply-recipe.json`/`post-apply.patch` (spec/request changes inert for `upstream_merged`) | ✅ no label |
| `reapplied` (transient during sweep) | n/a (sweep only) | n/a | ✅ run child reconcile | n/a | n/a | ✅ no label |
| `reconciling` (active sweep in flight) | E `ErrHardDepUnmet` (parent not yet satisfied) | E same | child waits until parent finishes this sweep | E `ErrHasDependents` (cannot remove during sweep) | E — amend refused during in-flight reconcile | L `waiting-on-parent` on child |
| `reconciling-shadow` (ADR-010) | E `ErrHardDepUnmet` | E same | child emits `waiting-on-parent` (parent awaits human accept/reject; §4.8) | E same | E — amend refused while shadow pending | L `waiting-on-parent` on child (distinct from `blocked-by-parent`) |
| `blocked` (generic) | E `ErrHardDepUnmet` | E same | child emits `blocked-by-parent` | E same | W same (discouraged; authors should resolve block first) | L `blocked-by-parent` on child |
| `blocked-requires-human` (ADR-010 resolver verdict, surfaced on parent) | E `ErrHardDepUnmet` | E same | child emits `blocked-by-parent` | E same | E | L `blocked-by-parent` on child |

**Table 3.2-B: soft dependency × parent state × verb**

| Parent state | `apply` | `apply --dry-run` | `reconcile` | `remove parent` | `amend parent` | `status` |
|---|---|---|---|---|---|---|
| any | ✅ proceed, single INFO line noting the soft edge | ✅ proceed, INFO line | ✅ run child reconcile independently; if soft parent's verdict is `blocked-*` and the child's recipe text references the parent's created symbols (heuristic string match), emit W — never flip the verdict | allowed; child loses the soft annotation, no other state change | - | L dashed edge rendered; never excluded from ready-frontier |

**Key invariants locked by this table:**

1. **Soft deps never produce errors on any verb.** They are informational edges. `created_by` gating therefore applies to **hard deps only** — a `created_by` pointing at a soft parent emits W at `apply`/`implement`, never E. This resolves the contradiction the reviewer caught in v1 §§3.2/4.3.
2. **`upstream_merged` satisfies hard deps.** The parent's changes now exist on the baseline, so a child recipe referring to parent-created paths behaves identically to a child whose parent is still `applied`. `created_by` messaging stays truthful ("will be created by parent X" — X did create them; they now happen to be upstream).
3. **`reconciling-shadow` yields `waiting-on-parent`, not `blocked-by-parent`.** The resolver is mid-flight, not failed. The distinction is load-bearing for `status --dag` UX (§4.7) — operators need to know "wait" vs. "go fix it."
4. **A child's own state transitions are never driven by parent state changes.** `waiting-on-parent` is a label rendered by `status`/`reconcile`, not a persisted `FeatureState`. See §3.5.

`--force` is the single escape hatch for `apply` and is explicit: it downgrades the gate from E to W and proceeds.

### 3.3 New / changed commands

- `tpatch feature add --depends-on <slug>[:hard|:soft] [...]` — declarative add. Kind defaults to `hard` if omitted (forcing the explicit-soft case). Writes to `status.json`.
- `tpatch feature deps <slug>` — read-only print of the dep block.
- `tpatch feature deps <slug> add <parent>[:hard|:soft]` / `remove <parent>` — edit the block. Validates cycles + existence. Writes to `status.json` and re-derives `dependents` across the store.
- `tpatch amend <slug> --depends-on <...>` — per M13 amend surface, same validation as `deps add`/`remove`.
- `tpatch status --dag` — render the whole graph as ASCII.
- `tpatch status --dag --json` — the same, machine-readable, for harnesses / CI. **Reuses the existing `--json` flag** already present on `tpatch status` — we do NOT introduce `--format json`, to keep flag surface consistent across commands.
- `tpatch apply <slug>` — new error class (§3.4) when hard deps unmet.
- `tpatch reconcile [slug...]` — topological traversal; see §4.
- `tpatch reconcile <slug> --isolated` — escape hatch that reconciles *this slug only*, ignoring deps. For debugging, not default.

**No `tpatch graph` command.** v1 of this PRD proposed a separate `tpatch graph <slug>` for scoped rendering. Dropped: `tpatch status --dag <slug>` (passing a slug argument to the DAG view) provides the same scoped-closure rendering without a new top-level verb. One less surface to document, lint, and parity-guard. Any future "rich graph viz" (e.g. DOT output, HTML) should be a fresh PRD, not a stub command baked in here.

### 3.4 New error / verdict surface

- Apply error: `ErrHardDepUnmet` — exit 2. Message shape:
  ```
  tpatch apply: feature 'copilot-dynamic-models' requires parent feature(s):
    - copilot-cli-provider (state: applied)     ← would proceed
    - some-other-parent    (state: requested)   ← BLOCKING
  Apply the blocking parent(s) first, or re-run with --force.
  ```
- Reconcile verdict: `ReconcileWaitingOnParent`. Populated on `ReconcileSummary`, with a new field `waiting_on: []string` listing unsatisfied parent slugs.
- Remove error: `ErrHasDependents` — exit 2 when `tpatch feature remove <parent>` is invoked and any other feature hard-depends on `<parent>`. Message shape:
  ```
  tpatch feature remove: feature 'copilot-cli-provider' has dependents:
    - copilot-dynamic-models (kind: hard)
    - effort-theming         (kind: soft)
  Remove them first, or pass --cascade to remove the whole subtree.
  ```
- Recipe op error: when an op's target path does not exist and the op declares `created_by: <parent>` (§4.3) **and** `<parent>` is a declared `hard` dep, the error becomes `ErrPathCreatedByParent` instead of `ErrPathNotFound`, with message `"path %s will be created by parent feature %s; apply %s first"`. Recipe dry-run treats this as a non-fatal warning iff the parent is in the child's `depends_on:hard`.
  - **Soft parents do not produce this error.** If `created_by` names a soft parent and the target is missing, we fall through to the existing `ErrPathNotFound` with an **additional W line** ("note: op declares created_by=<soft-parent>; soft deps do not gate apply"). This preserves the soft/hard split locked in §3.2.
  - If `created_by` names a feature that is **not in `depends_on` at all**, `implement` rejects the recipe with a validation error at recipe-write time (§4.3.1), before any `apply` runs.

### 3.5 State machine changes and the `waiting-on-parent` label

**No new persisted state.** `FeatureState` on disk is unchanged. v1 of this PRD already rejected a first-class `waiting-on-parent` state; the review flagged that the v1 rule was too narrow — it only covered pre-apply states, yet the M12 use case (ADR-010) produces a child in `applied` whose parent is in `reconciling-shadow`, and that child very much needs to be surfaced as "waiting."

**Redefined rule (v2):**

> `waiting-on-parent` and `blocked-by-parent` are **derived status labels**, computed from the per-parent state categorization, not from the child's own `FeatureState`. They are **distinct, composable predicates** covering disjoint parent-state categories:
>
> - A child is labelled `waiting-on-parent` iff **any** of its hard parents is in a **transient / in-progress** state — i.e. `∈ {requested, analyzed, defined, implementing, reconciling, reconciling-shadow}` (plus any future transient state). The parent is not done yet, but it has not terminally failed either.
> - A child is labelled `blocked-by-parent` iff **any** of its hard parents is in a **terminal-failure** state — i.e. `∈ {blocked, blocked-requires-human, blocked-upstream-drift, blocked-too-many-conflicts, blocked-by-parent}`. The parent is stuck and needs human action.
> - A child is **fully satisfied** (no parent-derived label) iff **every** hard parent is in the satisfied set `{applied, active, upstream_merged, reapplied}`.
>
> The two labels are **orthogonal to the child's own state** — a feature can simultaneously be `applied` AND `waiting-on-parent`, e.g. when its parent enters `reconciling-shadow` during an upstream sync.

Labels are computed as a pure function: `(child_state, parent_states) → label_set`. There is no write path, no timestamp, no new file. `status` and `reconcile` recompute on every read.

**Labels are composable.** A child with one parent in `reconciling-shadow` and another in `blocked-requires-human` carries **both** `waiting-on-parent` and `blocked-by-parent` simultaneously. They are not mutually exclusive: the predicates address disjoint parent subsets, and each predicate fires independently when any parent falls into its category.

**Parent-state × child-label matrix** (per individual parent; compose via OR across the child's hard-parent set):

| Parent state | Contributes to `waiting-on-parent`? | Contributes to `blocked-by-parent`? | Soft-parent contribution |
|---|---|---|---|
| `applied` / `active` / `upstream_merged` / `reapplied` | no | no | (none — satisfied) |
| `requested` / `analyzed` / `defined` / `implementing` | **yes** | no | (none — soft parents never contribute a label) |
| `reconciling` | **yes** | no | (none) |
| `reconciling-shadow` (ADR-010) | **yes** | no | (none) |
| `blocked` | no | **yes** | (none) |
| `blocked-requires-human` (ADR-010 verdict) | no | **yes** | (none) |
| `blocked-upstream-drift` / `blocked-too-many-conflicts` | no | **yes** | (none) |

Labels are computed bottom-up from roots. Both labels may appear on the same child simultaneously; neither masks the other. Rendering order in `status --dag` is fixed: `[blocked-by-parent] [waiting-on-parent]` when both are present, so the more severe condition is read first, but both remain visible. Soft parents never contribute a label regardless of their state.

**What this does NOT change:**

- Internal state transitions. The state machine diagram is untouched.
- Child's own reconcile verdict. A child in `applied` with a shadow-stuck parent is still `applied` at rest; it just renders with a `waiting-on-parent` badge in `status --dag`.
- Any existing workflow that does not read the label. `status` without `--dag` shows each feature's state as before, with the label appended only when present.

`status --dag` SHOWS the label; internal state transitions do NOT change because of it. This keeps the state machine simple and keeps the label idempotent (recomputed on every read; never persisted, never out-of-sync).

### 3.6 Skills (all 6 harnesses)

One-paragraph addition to each skill's `analyze` section, plus a cross-link in `implement`:

> **Declare dependencies.** If this feature's implementation plan touches files, symbols, or code paths that are not present on a pristine upstream checkout (because an earlier tpatch feature creates them), declare the parent in `depends_on` before running `tpatch implement`. Use `hard` when the recipe would fail without the parent, `soft` when the feature would apply but degrade semantically. Edit with `tpatch feature deps <slug> add <parent>:hard`.

Parity guard (`assets/assets_test.go`) must assert this bullet exists in all 6 surfaces (claude SKILL, copilot SKILL, copilot prompt companion, cursor `.mdc`, windsurf rules, generic workflow).

### 3.7 Downstream invalidation policy

When a parent is **amended** (M13) or **removed**, children are affected. v1 of this PRD left the behavior vague; this section makes it fully deterministic.

#### 3.7.1 Amend policy

When `tpatch amend <parent> ...` modifies a tracked file on a parent that has dependents, the following is emitted and recorded:

| Parent file changed | Hard children | Soft children | Behavior |
|---|---|---|---|
| `request.md` | no-op | no-op | Request text is human context only; does not affect downstream. |
| `spec.md` | `stale-parent-spec` label | `stale-parent-spec` label | Informational; does NOT block child apply or reconcile. Rendered by `status --dag`. |
| `apply-recipe.json` | `stale-parent-recipe` label + W on child's next `apply` + W on `status` | label only | Recipe shape change means child's `created_by` assumptions may be invalid. Reconcile may repath. |
| `artifacts/post-apply.patch` | `stale-parent-patch` label + `since` mismatch flagged | label only | Child's recorded `since` is now known-mismatched against parent's new patch hash. v1 surfaces this; `feat-patch-compatibility` will make it actionable. |

All three `stale-parent-*` labels are computed the same way as `waiting-on-parent` (derived, orthogonal to state), and coexist with it. A child can simultaneously be `applied + waiting-on-parent + stale-parent-recipe`.

#### 3.7.2 Remove policy

`tpatch feature remove <parent>` follows strict rules when dependents exist:

| Invocation | Has dependents? | Outcome |
|---|---|---|
| `remove <parent>` | no | proceeds as today |
| `remove <parent>` | yes | E `ErrHasDependents` listing each dependent + its kind |
| `remove <parent> --force` | yes | **Still** E `ErrHasDependents`. `--force` bypasses TTY confirmation, not dep-graph integrity. |
| `remove <parent> --cascade` | yes (TTY) | single confirmation prompt listing the full subtree; on Y, removes children in **reverse-topological order** (leaves first), then the parent; emits per-feature summary |
| `remove <parent> --cascade` | yes (non-TTY) | E — same existing rule for `remove` in non-interactive mode; requires `--cascade --force` to proceed without prompt |
| `remove <parent> --cascade --force` | yes (non-TTY) | proceeds without prompt; emits per-feature summary |

Reverse-topological ordering guarantees: when we delete child C first, parent P is still intact, so any post-remove hooks or audit steps on C see a coherent graph. Removing P first would leave C pointing at a missing parent during its own removal, triggering `ErrUnknownDep` mid-cascade.

Soft dependents are included in the cascade count and listed in the error, and the `--cascade` semantics treat them identically to hard dependents for removal (if you're pulling out a feature, its soft dependents should still learn about it). Hard and soft dependents alike are removed under `--cascade` or the operation is refused — there is no "drop soft edges only" mode in v1; see §9 for the deferred alternative.

These rules extend the existing M13 remove command. M13's TTY-confirm rule is preserved; the `--cascade` rule composes atop it.

---

## 4. Technical design

### 4.1 Data model

New type in `internal/store/types.go`:

```go
type DepKind string

const (
    DepHard DepKind = "hard"
    DepSoft DepKind = "soft"
)

type Dependency struct {
    Slug  string  `json:"slug"`
    Kind  DepKind `json:"kind"`
    Since string  `json:"since,omitempty"`
}
```

`FeatureStatus` gains:

```go
DependsOn  []Dependency `json:"depends_on,omitempty"`
Dependents []string     `json:"dependents,omitempty"` // derived, never hand-authored
```

Both fields live **only** in `status.json` (see §3.1, Option A). No yaml parser is introduced. Missing key = empty list (backward-compat invariant, §3.1).

On every `SaveFeatureStatus` / `MarkFeatureState` we re-derive the `Dependents` field for every feature in the store via `RefreshFeaturesIndex`. Cost is linear in feature count; the stress test's three-feature DAG is nowhere near a concern, and no real project will exceed a few hundred.

### 4.2 Validation (write-time)

Invoked from `feature add`, `feature deps add`, and a one-shot `tpatch feature deps --validate-all` (also run as part of `init` sanity):

1. **Slug exists.** Unknown parents rejected with `ErrUnknownDep`.
2. **Self-reference rejected.** `depends_on[].slug == self.Slug` → `ErrSelfDep`.
3. **Duplicate edges rejected.** Two entries with the same `slug` — **regardless of kind** — are a config error. An edge is defined by its target slug; changing kind is an `amend`, not a second edge. (v1 of the PRD only rejected identical `(slug, kind)` pairs, which accidentally allowed "A:hard and A:soft" to coexist — a contradiction.)
4. **Cycle detection.** DFS colouring (WHITE/GREY/BLACK). On a back-edge to GREY, error names the full cycle path: `A → B → C → A`.
5. **Kind value.** Only `hard` / `soft`; unknown → `ErrUnknownDepKind`.

`since` is accepted verbatim in v1; format validated syntactically (`<slug>@post-apply:<hex>`) but not cross-checked against the parent's actual patch hash until `feat-patch-compatibility` ships.

### 4.3 Recipe-level `created_by`

Stress-test wrinkle A. Included in this PRD — it's too tightly coupled to the dep semantics to defer.

`RecipeOperation` gains:

```go
CreatedBy string `json:"created_by,omitempty"` // parent slug that creates this op's Path
```

Semantics during `ExecuteRecipe` / `DryRunRecipe` (hard-only gating, per §3.2):

- If `created_by == ""` → current behaviour.
- If `created_by` is set and the parent is in this feature's `depends_on` as **`hard`** and the target exists → proceed normally.
- If `created_by` is set, names a `hard` dep, and target does not exist → emit `ErrPathCreatedByParent` referencing the named parent. `apply --force` override still bypasses.
- If `created_by` is set and names a **soft** dep → target-existence is **not** gated. If the target is missing, fall through to existing `ErrPathNotFound` with an advisory W line ("note: op declares created_by=<parent>; soft deps do not gate apply"). See §3.2 Table 3.2-B.
- If `created_by` is set but names a feature **not** in `depends_on` at all → validation error at `implement` time ("recipe references a file created by feature X but X is not in depends_on"); nudges the author to declare it explicitly.

`tpatch implement` is updated to suggest `created_by` population when the provider / heuristic output includes `replace-in-file` ops whose `Search` text cannot be found in the pristine checkout but *is* present in a parent feature's `post-apply.patch`. Low-cost heuristic; explicit opt-out via `--no-created-by-infer`. Inference only suggests hard parents (soft parents wouldn't gate anyway).

### 4.3.1 Recipe schema rollout — coordinated change

Adding `created_by` to `RecipeOperation` is a coordinated schema change, not just a Go-struct edit. The `assets/assets_test.go` parity guard uses `json.DisallowUnknownFields` on the canonical recipe schema, so every skill surface that ships example recipes must learn the field in the same release.

**All of the following must land together in M14.2** (not spread across milestones):

1. **Go struct field:** `CreatedBy string \`json:"created_by,omitempty"\`` added to `RecipeOperation` in `internal/tpatch/`. Field is optional; `omitempty` ensures pre-v0.6 recipes round-trip unchanged.
2. **Recipe JSON examples in all 6 skill surfaces** updated:
   - `assets/skills/claude/...` (example recipes under skill docs)
   - `assets/skills/copilot/...`
   - `assets/skills/copilot-prompt/...`
   - `assets/skills/cursor/...`
   - `assets/skills/windsurf/...`
   - `assets/skills/generic/...`
   Each gets at least one example op demonstrating correct `created_by` usage with a hard-dep example (mirror of the §1 stress-test DAG).
3. **`docs/agent-as-provider.md`** gains a new sub-section "Declaring parent-created paths" explaining:
   - The `created_by` field shape.
   - The hard-dep requirement (soft deps emit W, not E — matches §3.2).
   - The implement-time cross-check error shape.
   - A worked example with the stress-test DAG.
4. **`assets/assets_test.go` parity-guard updates:**
   - New required-anchor assertion: every recipe example in `assets/skills/*` that contains a `replace-in-file` op against a path that does not exist on pristine checkout must include a `created_by` field. (Tested by grepping the shipped examples; keeps drift from creeping in.)
   - New positive unmarshal test: a recipe JSON carrying `created_by` is accepted by the canonical unmarshal path even with `DisallowUnknownFields` in effect. Asserts the struct field tag is correct and present.
   - New negative test: a recipe JSON carrying `created_bye` (typo) still fails, confirming `DisallowUnknownFields` is still on.
5. **CHANGELOG v0.6.0 callout** explicitly naming `created_by` as a new recipe-schema field, with the hard-only gating semantics spelled out so downstream tooling (CI, custom harnesses) knows to update their recipe validators.

**Milestone placement:** all five items land in M14.2. M14.1 (data model + validation) intentionally does NOT touch recipe schema — if M14.2 slips, M14.1 can still ship the dep data model without the recipe-schema churn, but M14.2 ships as an atomic bundle.

### 4.4 DAG resolution algorithm

**Kahn's algorithm** with deterministic tie-breaking on slug (alphabetical). Chosen over DFS-postorder because:

- The traversal order *is* the operator-facing order (apply order, reconcile order). Kahn's ready-set model maps 1:1 to "features ready to apply right now" — the exact question `status --dag` answers.
- It naturally surfaces cycles at input (ready-set empty with remaining nodes) rather than mid-recursion.
- Incremental updates (a single feature changes state) re-run in O(V+E) on the in-memory graph, which is trivial.

Pseudocode:

```
sort features by slug ascending
in_degree[v] = count of v's hard parents (soft edges are ignored for ordering)
ready = { v : in_degree[v] == 0 }
order = []
while ready not empty:
    pick = min(ready)                   # deterministic
    order.append(pick); remove from ready
    for child of pick (hard edges only):
        in_degree[child] -= 1
        if in_degree[child] == 0: add to ready
if |order| < |features|: return ErrCycle(remaining)
```

**Soft edges are intentionally not part of the ordering.** A soft parent failing should not block the child — that's the entire point of the soft/hard split.

### 4.5 Reconcile integration

`tpatch reconcile` (no slugs) and `tpatch reconcile <slug>` both funnel through a single DAG planner:

1. Compute transitive hard closure (focus set).
2. Kahn-order the focus set.
3. For each feature in order, apply the **precedence rules** below. There is no "skip to next on blocked parent" shortcut — the child's own verdict is always computed first so intrinsic drift is never masked.
4. After the full sweep, any child whose own verdict was clean but whose parent state produced a derived label is reported with the label(s) attached (see §3.5 for label composition).

**Precedence rules (locked — these define the behavior; the matrix below is a reference view, not a competing source of truth):**

1. **Always compute the child's own reconcile verdict first**, running the full 4-phase reconcile, independent of parent state. This preserves the v0.4.2 truthful-error theme: operators must see their actual problems, not a hidden-behind-parent label.
2. If the child's own verdict is any `blocked-*` value (its own drift, its own validation failure, its own phase-1/2/4 failure) → **emit the child's own verdict unchanged**. Child-intrinsic drift is never masked by parent state; parent status may be echoed in `notes` but does not flip the verdict.
3. If the child's own verdict is clean (`reapplied` / `upstreamed` / `already-active`) → **overlay parent-derived labels** from §3.5 (`waiting-on-parent`, `blocked-by-parent`) as additional status on the same entry. The verdict stays clean; the labels flag the contextual situation.
4. If the child's own verdict is `3WayConflicts` (i.e. the child needs phase 3.5 provider-assisted resolution) **and** any hard parent is in a `blocked-*` state → **skip phase 3.5 resolution** and emit the compound verdict `blocked-by-parent-and-needs-resolution`, with guidance `"resolve parent <slug> first, then retry: tpatch reconcile <child-slug>"`. Rationale: running the resolver against a child whose parent is itself unresolved wastes provider budget and produces churn; human action is required at the parent first.
5. If the child's own verdict is `3WayConflicts` and no parent is blocked (parents only transient or satisfied) → proceed with phase 3.5 normally; overlay `waiting-on-parent` only if a hard parent is transient.

```text
compute_child_verdict(c) -> v
switch v:
  case blocked-*:                  emit v; done.
  case clean (reapplied/upstreamed/already-active):
                                   emit v + §3.5 label set for c's hard parents.
  case 3WayConflicts:
      if any hard parent in blocked-* set:
                                   emit blocked-by-parent-and-needs-resolution;
                                   skip phase 3.5; done.
      else:
                                   run phase 3.5; re-evaluate v;
                                   overlay waiting-on-parent if any hard parent transient.
```

**Multi-parent verdict composition — reference matrix** (deterministic consequence of the precedence rules above):

| Child's own outcome | Hard-parent set | Emitted child verdict |
|---|---|---|
| any child `blocked-*` (intrinsic drift / validation / phase 4 failure) | any | child's own `blocked-*` verdict (unchanged; parent status echoed in notes only) |
| child clean (`reapplied` / `upstreamed` / `already-active`) | all satisfied | child's own clean verdict, no overlay |
| child clean | some parents transient, none blocked | child's own clean verdict **+** `waiting-on-parent` label |
| child clean | some parents blocked, none transient | child's own clean verdict **+** `blocked-by-parent` label |
| child clean | mix of transient AND blocked | child's own clean verdict **+** both `waiting-on-parent` AND `blocked-by-parent` labels (§3.5 composition) |
| child `3WayConflicts` | all satisfied or transient only | run phase 3.5; post-3.5 verdict applies (+ `waiting-on-parent` if any parent transient) |
| child `3WayConflicts` | any parent blocked | `blocked-by-parent-and-needs-resolution` (phase 3.5 skipped) |
| any soft parent failed, no hard issue | — | child's own verdict; soft failures echoed in notes only, never flip the verdict |

`waiting_on` in `ReconcileSummary` always lists **every** unsatisfied hard parent (transient or blocked), even when the child's own verdict is emitted with no parent-derived label, so operators see the full picture.

### 4.6 Apply integration

`tpatch apply <slug>` checks hard-dep gate *before* any side effects, per Table 3.2-A:

```go
for _, dep := range status.DependsOn {
    if dep.Kind != DepHard { continue }
    pstate := store.ReadFeatureStatus(dep.Slug).State
    // Satisfied set defined in §3.2 canonical truth table.
    if !isHardDepSatisfied(pstate) {
        return ErrHardDepUnmet{Parent: dep.Slug, ParentState: pstate}
    }
}
```

where `isHardDepSatisfied(s) == s ∈ {applied, active, upstream_merged, reapplied}`.

`--force` bypasses with a one-line stderr warning. Recipe execution then proceeds as today; `created_by` ops (§4.3) pick up slack for fine-grained target-existence errors.

### 4.7 Rendering (`status --dag`)

ASCII tree, roots first (in-degree 0 on hard edges), children indented. Soft edges rendered dashed and *after* hard children to avoid confusing the eye. Derived labels (`waiting-on-parent`, `blocked-by-parent`, `stale-parent-*`) are appended after the state badge when present.

```
$ tpatch status --dag
copilot-cli-provider                 [active]
├── copilot-dynamic-models           [applied] [waiting-on-parent]      hard
┆   ┄┄ effort-theming                [implementing]                     soft  ← copilot-cli-provider
standalone-feature                   [active]
```

`tpatch status --dag --json` (reuses the existing `--json` flag — no new `--format json`):

```json
{
  "nodes": [
    { "slug": "copilot-cli-provider", "state": "active", "labels": [], "compatibility": "compatible" },
    { "slug": "copilot-dynamic-models", "state": "applied", "labels": ["waiting-on-parent"], "compatibility": "compatible" }
  ],
  "edges": [
    { "from": "copilot-dynamic-models", "to": "copilot-cli-provider", "kind": "hard" }
  ],
  "ready": ["copilot-dynamic-models"],
  "cycles": [],
  "waiting_on_parent": ["copilot-dynamic-models"]
}
```

**Edge direction is locked:** `edge.from` is the **child** (dependent); `edge.to` is the **parent** (dependency). This matches the data-flow reading "from X, which depends on, to Y." Clients must not reverse. The direction is documented in `docs/dependencies.md` (M14.4) and asserted in a golden-output test (§7).

`status --dag <slug>` (with a slug argument) renders a **scoped** view: the focus feature plus its transitive closure upward (ancestors) and downward (descendants). This replaces the dropped v1 `tpatch graph <slug>` command. Machine output identical shape, with `"focus": "<slug>"` added.

Status labels in JSON are always an array. `labels: []` when no derived label applies. Multiple labels can coexist (e.g. `["waiting-on-parent", "stale-parent-recipe"]`) per §3.5 and §3.7.

### 4.8 Interaction with M12 (provider-assisted conflict resolver)

The resolver does **not** need to see the dep chain for its core shadow-resolution work — it operates on a single feature's conflicted files. However, the DAG planner (§4.5) is the right place to sequence `--resolve` across a stack: if the root's shadow is `awaiting`, children must wait for accept/reject before their own reconcile runs. We add no new flags to `reconcile --resolve`; the sequencing is implicit from the planner.

**Locked decision: v0.6 does NOT pass parent-patch context to the M12 resolver.** Each shadow-resolution call sees only the feature's own conflicted files. Rationale:

- The resolver contract (ADR-010) is single-feature; changing it is out of scope for this PRD.
- Parent-aware resolution would require a second API shape and a second prompt template, multiplying skill-asset surface at the moment we're adding dep semantics everywhere else.
- The current planner sequencing (hard parent must exit `reconciling-shadow` before child runs) already produces correct outcomes for the stress-test DAG; we have no evidence that provider-side parent awareness is the bottleneck.

Follow-up: tracked as **`feat-resolver-dag-context`** (§9) — adds parent-patch context to the resolver prompt and API, with its own ADR and eval plan. Not a blocker for v0.6.

---

## 5. Open questions & decisions

### Q1 — Hard vs soft dependencies?

**DECIDED: both, with the split locked as §3.2.** The stress test produced concrete evidence that `copilot-dynamic-models` (hard) and `effort-theming` (soft) need divergent behaviour. A one-dimensional model would either force false failures on soft deps or hide real failures on hard deps. No default kind (authors must choose).

### Q2 — Auto-rebase on parent drift?

**DEFERRED** to `feat-feature-autorebase`. v1 detects drift (via `since` mismatch once `feat-patch-compatibility` lands) and *warns*, marking the child as needing reconcile. Auto-editing a child's recipe / patch to match a shifted parent is a large, risky feature that deserves its own PRD. The `since` field is recorded in v1 so follow-up work has the data it needs.

### Q3 — Per-dep version pinning (`B requires A@revX`)?

**DEFERRED** to `feat-patch-compatibility`. The `since` scalar on each `Dependency` is the entire data-plane surface we need — the enforcement layer (hashing, mismatch reporting, range expressions) is a separate concern that overlaps with upstream.lock. v1 stores `since`, does not read it.

### Q4 — Delivery-mode stack handling (ship whole chain vs. tip)?

**DEFERRED** to `feat-delivery-modes`. This PRD stops at "the DAG is knowable and traversable." How `tpatch deliver` / `tpatch upstream pr` uses it — squash, stacked PRs, build a single artifact at tip — is a delivery-layer decision that should not be rushed into the dep-system PRD. Acceptance criterion on that future PRD: must consume `DependsOn` and refuse incoherent stacks.

### Q5 — Cycle UX / dependency drop?

**DECIDED: refuse cycles, atomic drop, explicit remove cascade (see §3.7.2).**

- Cycles are a bug in the data model; `tpatch feature deps add` rejects them at write time with the full path. There is no `--allow-cycle` escape hatch.
- `tpatch feature deps <slug> remove <parent>` is atomic — removes the edge from `status.json` and re-derives `dependents` across the store. If the removal leaves a child in a state where its recipe still references parent-created paths, validation flags it on next `implement` (via the `created_by` cross-check, §4.3). We do not scan recipe bodies on remove.
- `tpatch feature remove <slug>` follows the full policy in §3.7.2. Summary: dependents → E `ErrHasDependents` unless `--cascade`; `--force` alone does NOT bypass the dep check (force is for TTY prompt, not graph integrity); non-TTY + `--cascade` requires `--force` to skip the confirm prompt.

### Q6 — Does this subsume `feat-feature-decomposition`?

**Neither subsumes.** Decomposition is the *producer* of deps (split one feature into A→B→C); this PRD is the *consumer* (model, enforce, visualise). Decomposition is blocked-by this PRD: it cannot emit a coherent multi-feature split without a dep graph to declare the split in. Tracked in §10.

---

## 6. Rollout plan

Target: M14, tranche D1. 4 sub-milestones, roughly linear.

### M14.1 — Data model + validation (~300 LOC + tests)

- Scope: ADR-011 written and merged FIRST (per §0.1, repo rule from AGENTS.md). Then: `Dependency` struct in `status.json` schema, `RefreshFeaturesIndex` computes `Dependents`, write-time validation (all 5 rules from §4.2), cycle DFS, config flag `features.dependencies` wired (default `false`).
- Dependencies: none. Uses existing store plumbing.
- Tests: status.json round-trip with and without `depends_on` key (backward-compat), cycle detection across 2/3/5-node graphs, self-ref/unknown/duplicate-regardless-of-kind rejection, `Dependents` stays in sync after every mutation, legacy-no-deps regression fixture.

### M14.2 — Apply gate + recipe `created_by` coordinated schema rollout (~250 LOC + tests)

- Scope: `ErrHardDepUnmet`, `--force` override, `RecipeOperation.CreatedBy`, `ErrPathCreatedByParent` (hard-only), soft-parent W-not-E path, implement-time cross-check, heuristic inference with `--no-created-by-infer`, **all 5 items from §4.3.1 coordinated rollout** (struct field + 6 skill examples + agent-as-provider doc + parity guard + CHANGELOG).
- Dependencies: M14.1 landed.
- Tests: the three stress-test failure modes as regression fixtures; hard-vs-soft `created_by` gating tests; parity-guard positive/negative tests for `created_by`; all 6 skill recipe examples parse under `DisallowUnknownFields`.

### M14.3 — Reconcile topological traversal (~500 LOC + tests)

(Estimate bumped from 350 → 500 per review: the multi-parent verdict composition matrix in §4.5, the child-own-drift precedence rule, and M12 shadow sequencing are each non-trivial and each need dedicated test coverage.)

- Scope: Kahn planner, `ReconcileWaitingOnParent` / `ReconcileBlockedByParent` outcomes, M12 shadow-awaiting sequencing, `--isolated` escape hatch, multi-parent verdict composition, child-own-drift precedence.
- Dependencies: M14.1 landed. Independent of M14.2 but benefits from its tests.
- Tests: golden scenarios (see §7). Specifically the stress-test DAG rerun end-to-end against a staged upstream; multi-parent diamond with mixed verdicts; child-with-own-drift-and-blocked-parent (verdict must be child's own, not `blocked-by-parent`).

### M14.4 — `status --dag`, amend/remove cascade, skill updates, docs, flag flip (~300 LOC + tests)

(Estimate bumped from 200 → 300: the amend/remove cascade behavior (§3.7) and the scoped `status --dag <slug>` view each add non-trivial code and test surface.)

- Scope: ASCII + JSON renderers (including derived-label surfacing), scoped `status --dag <slug>` replacing v1's `graph` command, `--cascade` on `remove`, amend-emits-`stale-parent-*`-labels wiring, skill bullet in all 6 surfaces, parity-guard extension, `docs/feature-layout.md` + `docs/reconcile.md` + `docs/agent-as-provider.md` updates, new `docs/dependencies.md`, **flip `features.dependencies` default to `true`** in the v0.6.0 cut commit.
- Dependencies: M14.1–M14.3.
- Tests: `assets_test.go` parity extended; render golden-output tests for three graph shapes (chain, fork, diamond); edge-direction assertion in JSON output; amend/remove cascade tests (see §7); scoped `status --dag <slug>` tests.

### v0.6.0 cut

M14.4 tagged as v0.6.0 with CHANGELOG entry headlining the DAG support, the three stress-test failure modes now caught, and the explicit non-goals list.

---

## 7. Acceptance criteria (v1 ships when…)

- [ ] **ADR-011 merged** before any M14 code lands (§0.1).
- [ ] `go build ./...`, `go test ./...`, `gofmt -l .` all clean.
- [ ] `status.json` with `depends_on` round-trips through store without loss; the `Dependents` derivation stays consistent across every state transition.
- [ ] Write-time validation rejects: unknown parent, self-reference, duplicate edges **regardless of kind**, invalid kind, cycles (2, 3, and 5-node cases tested).
- [ ] `tpatch apply <child>` on pristine checkout fails with `ErrHardDepUnmet` naming the parent; `--force` bypasses with a stderr warning; exit codes stable.
- [ ] Recipe op with `created_by` pointing at a **hard** dep and missing target emits `ErrPathCreatedByParent` with the parent slug; dry-run downgrades to warning when parent is a declared hard dep.
- [ ] Recipe op with `created_by` pointing at a **soft** dep and missing target emits the ordinary `ErrPathNotFound` plus an advisory W line — **not** `ErrPathCreatedByParent`.
- [ ] `implement` rejects recipes whose `created_by` names an undeclared parent.
- [ ] `tpatch reconcile` with no slugs processes features in Kahn order, alphabetical on ties; child of blocked parent reports `blocked-by-parent`, not `blocked-upstream-drift`.
- [ ] `--isolated` escape hatch ignores the dep graph and reports exactly today's single-feature behaviour.
- [ ] `tpatch status --dag` renders ASCII with the stress-test DAG shape shown in §4.7; `tpatch status --dag --json` (reusing the existing `--json` flag — not `--format json`) emits the structure shown in §4.7, with `edge.from` = child and `edge.to` = parent **explicitly asserted** by a golden-output test.
- [ ] `tpatch status --dag <slug>` renders the scoped view (ancestors + descendants); no separate `tpatch graph` command exists.
- [ ] `tpatch feature deps add/remove` mutates `status.json` atomically, re-derives `Dependents`, and runs full validation on every write.
- [ ] `tpatch amend --depends-on` is recognized and validated identically to `feature deps add/remove`.
- [ ] All 6 skill surfaces carry the dep-declaration bullet; `assets_test.go` parity guard green.
- [ ] **Recipe schema rollout (§4.3.1):**
   - `created_by` unmarshal test passes with `DisallowUnknownFields` on.
   - Typo `created_bye` still fails `DisallowUnknownFields` (confirms the guard is still in effect).
   - All 6 skill example recipes include at least one `created_by`-bearing op, validated by parity test.
   - `docs/agent-as-provider.md` has a "Declaring parent-created paths" section asserted by a docs-anchor test.
- [ ] **Backward-compat regression (§3.1 invariant):**
   - `TestUpgradeFromV0_5_1_NoDeps_BehavesIdentically`: a repo fixture with N features (chosen N=3), none declaring deps, applies/reconciles/status-es/amends/removes byte-identically (modulo the new `depends_on: []` / `dependents: []` keys appearing in `status.json` only on next write) to v0.5.1 output.
- [ ] **Feature flag (§3.1 rollout safety):**
   - `TestFeatureFlagOff_AllDAGBehaviorInert`: with `features.dependencies: false`, DAG planner, derived labels, `--dag`, `created_by` gating, cascade remove are all disabled (commands behave as v0.5.1).
   - `TestFeatureFlagOn_AllDAGBehaviorActive`: with `features.dependencies: true`, all new behavior is active.
   - `TestV060CutFlipsFlag`: the v0.6.0 release commit sets the default to `true` in exactly one place.
- [ ] **Amend + remove invalidation (§3.7):**
   - `TestAmendRecipe_WarnsOnHardChildren`: amending parent's `apply-recipe.json` adds `stale-parent-recipe` label to hard children and emits a W on `status`.
   - `TestAmendSpec_LabelsChildrenStale`: amending parent's `spec.md` adds `stale-parent-spec` label to children (both hard and soft).
   - `TestAmendRequest_NoOp`: amending parent's `request.md` produces no child-side label.
   - `TestStatusDag_ShowsStalenessLabels`: rendered output includes `[stale-parent-recipe]` etc. after an amend.
   - `TestRemoveWithHardDependents_ErrorsWithoutCascade`: `feature remove` with dependents returns `ErrHasDependents`.
   - `TestRemoveWithForceAlone_StillRefused`: `--force` alone does NOT bypass the dep check.
   - `TestRemoveWithCascade_DeletesInReverseTopoOrder`: `--cascade` removes leaves first, parent last; per-feature summary emitted.
   - `TestRemoveWithCascade_NonTTYRequiresForce`: non-TTY + `--cascade` without `--force` → refused.
- [ ] **Waiting-on-parent as derived label (§3.5):**
   - `TestWaitingOnParent_LabelComposesWithAppliedState`: a child in `applied` whose hard parent is in `reconciling-shadow` renders with `[applied] [waiting-on-parent]` in `status --dag`, and the child's persisted `FeatureState` is still `applied` (no state write occurred).
   - `TestLabelsCompose_BlockedAndWaitingCoexist`: a child with one blocked parent and one transient (awaiting) parent carries **both** `blocked-by-parent` AND `waiting-on-parent` labels simultaneously (§3.5 composability).
- [ ] **Multi-parent verdict composition (§4.5):**
   - `TestChildOwnDriftNotMaskedByBlockedParent`: a child with intrinsic drift AND a blocked parent reports the child's own drift, NOT `blocked-by-parent`.
   - `TestDiamond_MixedParentVerdicts`: diamond DAG with one blocked and one clean parent, child's own verdict clean → emitted child verdict stays clean, with `blocked-by-parent` label overlaid; `waiting_on` lists all unsatisfied parents.
   - `TestChildNeedsPhase3_5_BlockedParent_CompoundVerdict`: child's own verdict is `3WayConflicts` AND a hard parent is in `blocked-*` → phase 3.5 is skipped and the compound verdict `blocked-by-parent-and-needs-resolution` is emitted with parent-first guidance (§4.5 rule 4).
- [ ] **Orphan detection on `status`:**
   - `TestStatus_SurfacesManualEditDrift`: manual edit of `status.json` that introduces an unknown-dep slug is surfaced by `tpatch status` (not only by workflow commands) with a clear error.
- [ ] **Golden scenarios** (fixtures under `tests/dependencies/golden/`):
   1. **Chain replay.** A → B → C on pristine checkout; `apply C` refuses until A and B applied. After full apply, all three in `applied`. `status --dag` matches expected.
   2. **Stress-test DAG.** Exact shape from §1 reproduced; every failure mode resolves: child gets actionable error instead of "file not found"; reconcile surfaces `blocked-by-parent` when root is blocked; soft edge does not gate anything.
   3. **Diamond.** A → B, A → C, {B,C} → D. Kahn order stable across runs. D waits on B *and* C.
   4. **Cycle.** `feature deps add` refuses with the full cycle path, `status.json` not written.
   5. **Soft-only dep.** Child applies cleanly with soft parent absent; warning logged; no verdict flip.
   6. **`upstream_merged` satisfies hard.** Parent marked `upstream_merged`; child `apply` proceeds; `status --dag` shows no `waiting-on-parent` on child.
- [ ] CHANGELOG v0.6.0 section; version bumped; tagged.

---

## 8. Risks & mitigations

| Risk | Mitigation |
|---|---|
| **Dep graph staleness when upstream merges a parent.** Parent goes to `upstream_merged`; child's recipe still references parent-created paths, but those paths now exist in pristine upstream. | Reconcile treats `upstream_merged` as a **satisfied** hard-parent state — codified in §3.2 Table 3.2-A and §4.6's `isHardDepSatisfied()`. `created_by` messaging stays truthful: the path *was* created by the parent — just now it lives upstream. Docs call this out. Golden test (§7) asserts the behavior. |
| **User edits `status.json` by hand and orphans a dependent.** Dep validation only runs on tpatch-driven writes. | `tpatch feature deps --validate-all` runs at startup of every workflow command (analyze/define/explore/implement/apply/reconcile) **and on every `status` invocation** (including bare `status` with no flags), so manual-edit drift is surfaced on the exact command users reach for when things look wrong. Failure surfaces a clear message pointing at the offending file. Cheap (O(V+E)). |
| **Cycles introduced by bulk edits outside the CLI.** Same class as above. | Same validator catches it before any side effects. If a cycle exists at startup, workflow commands refuse until resolved; `status` reports the cycle explicitly. |
| **`created_by` heuristic infers a false parent.** Provider emits a `replace-in-file` whose `Search` coincidentally appears in a parent's patch. | `created_by` inference is advisory: always printed for operator review at `implement` time; `--no-created-by-infer` disables it. Manual edits survive re-runs. |
| **Performance for large DAGs.** | Kahn is O(V+E). The store already iterates all features on index refresh. No new hot path. If any project ever sees >1000 features we have bigger problems. |
| **Skill drift (one of six surfaces forgets the dep bullet or the `created_by` example).** | Parity guard extended in M14.2 and M14.4 checks both the dep bullet text and a `created_by` example op in all six surfaces. Ratchets: cannot regress without flipping the test. |
| **`since` format confusion once `feat-patch-compatibility` lands.** | v1 stores `since` but does not interpret it; documented explicitly as forward-compat metadata. The compat PRD owns the semantic upgrade. |
| **Feature flag left enabled in trunk before M14.4 completes.** | Flag default is `false` through M14.1–M14.3. A single commit in M14.4 flips it to `true` as part of the v0.6.0 cut. `TestV060CutFlipsFlag` (§7) asserts the flip is atomic and in one place. |

---

## 9. Out of scope / future work

- `feat-feature-autorebase` — auto-modify child recipe/patch when parent drifts. Consumes this PRD's `since` field.
- `feat-patch-compatibility` — enforce `since` constraints; per-dep version ranges.
- `feat-record-scoped-files` — `tpatch record --from <sha> --to <sha>` for bounded commit-range capture. Stress-test wrinkle B, split for independent shipping.
- `feat-delivery-modes` — stacked-PR, build-artifact-from-tip, diff-only shipping. Consumes this PRD's topological order.
- `feat-parallel-feature-workflows` — reconcile independent DAG branches concurrently.
- `feat-feature-decomposition` — split an oversized feature into a dep chain. Blocked-by this PRD.
- `feat-feature-standalonify` — inverse: fold a chain back into one feature. Blocked-by this PRD.
- `tpatch upstream cherry-pick-parent` — convenience for "upstream adopted only the root; update the child's `since` and reconcile." Natural follow-up.
- **`feat-resolver-dag-context`** — pass parent-patch context to the M12 provider-assisted conflict resolver (§4.8). Requires its own ADR and eval plan. Not a blocker for v0.6; v0.6 keeps the resolver single-feature and sequences invocations at the planner level. Motivating question: does showing the resolver "here's what parent A did to this file" improve child-shadow quality? Needs data from v0.6 usage before we commit.
- **Audit-patch DAG view.** Per-feature `patches/NNN-*.patch` remain authoritative in v0.6; there is no combined stack-audit patch. If operators want "show me the whole stack's diff at HEAD," that's a fresh PRD (`feat-stack-audit-patch`) and lives after v0.6 ships.
- **`feature remove --orphan-soft`** — allow dropping soft-dep edges without cascading delete of soft dependents. Deferred — the simpler remove semantics in v0.6 (no flag / `--cascade` / `--force` for TTY only) are sufficient; revisit if users request a way to decouple soft dependents without removing them.

---

## 10. Cross-cutting impact matrix

| Other feature / surface | Relationship | Notes |
|---|---|---|
| `feat-feature-decomposition` | **blocked-by** | Cannot emit multi-feature split without a dep graph. Depends on §3.1 schema + §4.2 validation. |
| `feat-feature-standalonify` | **blocked-by** | "Fold dep chain back into one feature" is meaningless without a graph model. |
| `feat-parallel-feature-workflows` | **enables** | Independent DAG branches are the unit of parallelism. This PRD ships sequential; parallel is a pure follow-up once the planner exists. |
| `feat-noncontiguous-feature-commits` | **pairs-with** | Commit-range recording makes sense exactly when features are stacked. Same real-world user need surfaced them together. |
| `feat-record-scoped-files` | **pairs-with** | Stress-test wrinkle B, split off from this PRD. Same stress test motivates both. |
| `feat-patch-compatibility` | **pairs-with** | Consumes `since` field. Should ship close to (not inside) this PRD. |
| `feat-delivery-modes` | **cross-cutting** | Delivery semantics over a DAG: stacked PRs, single artifact, diff-only. Data model here; behaviour there. |
| `feat-richer-operation-types` | **pairs-with** | `created_by` is a first operation-level metadata field; future op types (rename, delete-region) will likely need similar hints. This PRD sets the precedent. |
| `feat-provider-conflict-resolver` (M12, shipped) | **consumes** | Phase 3.5 sequencing across a DAG is handled by §4.5; no flag changes to `--resolve`. Parent-context extension deferred to `feat-resolver-dag-context` (§9). |
| `feat-feature-amend` (M13, shipped) | **extends** | Amend gains the `--depends-on` flag and the downstream-invalidation policy (§3.7.1). `stale-parent-*` labels added to `status --dag` after amend. |
| `tpatch feature remove` (M13, shipped) | **extends** | Remove gains `--cascade` (§3.7.2). `--force` no longer bypasses dep-graph integrity. |
| `tpatch status` (existing command) | **extends** | `--dag` / `--dag --json` (reuses existing `--json` flag) / `--dag <slug>` scoped view; surfaces derived labels; runs full dep validation on every invocation (§8 orphan risk). |
| `tpatch status --json` | **extends** | Per-feature entries gain `labels: []` array (may contain `waiting-on-parent`, `blocked-by-parent`, `stale-parent-*`). Backward-compat: readers that don't know the key are unaffected (additive change). |
| `docs/feature-layout.md` | **extends** | Adds a "Dependencies" section documenting `status.json` `depends_on` / `dependents` fields. Clarifies that `status.json` is the single source of truth (no new `feature.yaml`). |
| `docs/agent-as-provider.md` | **extends** | New "Declaring parent-created paths" section explaining `created_by` semantics and the hard-only gating rule. |
| `docs/reconcile.md` | **extends** | New section on DAG-aware reconcile, verdict composition, and M12 shadow sequencing. |
| `docs/dependencies.md` | **new** | End-user facing guide for declaring and debugging feature dependencies. |
| `patches/NNN-*.patch` audit trail | **unchanged** | Per-feature `patches/` directory remains authoritative for per-feature audit history. No combined stack audit in v0.6 (see §9 follow-up). |
| `assets/assets_test.go` parity guard | **extends** | New required anchors: dep-declaration bullet in all 6 surfaces; `created_by` example op in all 6 recipe examples; `created_by` unmarshal positive/negative tests. |

---

**End of PRD.** Implementation task list will live at `docs/milestones/M14-feature-dependencies.md`, written once this PRD is accepted.
