# PRD — `tpatch verify` and verification freshness overlay

**Status**: Approved (M15 Wave 3 — APPROVED WITH NOTES at commit 3c122aa; Slice A in implementation. Supersedes `docs/prds/PRD-verify-and-tested-state.md`.) · **§3.6 / §4.3 golden refresh / §4.3.6–4.3.9 / §5 landed rows / §6 Q9–Q16 / §7.1 amended 2026-08-12 for v0.15.1 Wave B (GH #8), **rev-2** — AWAITING REVIEW**
**Date**: 2026-04-27 (original) · 2026-08-12 (landed-feature amendment)
**ADR**: **ADR-013-verify-freshness-overlay.md — REQUIRED before Wave 3 implementation slices ship.** ADR-013 supersedes ADR-012 in full. **ADR-013 Amendment 1 rev-2 (D8–D17) is the binding ADR for §3.6 and §7.1.**
**Owner**: Core
**Milestone**: M15 → Wave 3 (lifecycle / reconcile semantics tranche) · v0.15.1 Wave B (landed-feature amendment; implementation deferred to Wave C)
**Related**: ADR-010 (M12 resolver), ADR-011 (feature DAG), ADR-013 (this PRD's binding ADR), ADR-016 (`record` auto-base resolution), ADR-018 (cross-feature collision signature), ADR-019 (`tpatch land` trailer-block schema), ADR-028 (supersession), **ADR-029 (write-file recipe safety — the policy V10 must honour)**, ADR-031 (rejected state), ADR-032 (unapply state), `docs/prds/PRD-tpatch-land.md` (co-amended §3.8), `docs/prds/PRD-verify-and-tested-state.md` (superseded predecessor), `docs/dependencies.md`, `docs/prds/PRD-feature-dependencies.md`, `docs/reconcile.md`, CHANGELOG v0.6.1, CHANGELOG v0.11.3 (GH #2), GH #8

> **This PRD supersedes [`docs/prds/PRD-verify-and-tested-state.md`](PRD-verify-and-tested-state.md) in full.**
> The predecessor encoded `tested` as a forward `FeatureState` lifecycle value.
> An external re-review of the approved design (commit `8c3d72e`) found two
> structural problems: F1 (verify shadows ignored the parent-replay closure)
> and F4 (the design conflated lifecycle with verification freshness, routing
> state mutation through a read path). The supervisor reopened Wave 3 with a
> binding redesign: a Git-like freshness overlay that keeps lifecycle
> untouched. ADR-013 holds the architecture decisions; this PRD captures the
> _what_ and the _why-now_. The reopening note is appended to
> `docs/supervisor/LOG.md` (2026-04-27 entry).

---

## 0. Meta

### 0.1 Why one PRD covers two backlog items

`feat-verify-command` and `feat-feature-tested-state` were originally scoped together because they share contract surface. The first revision approved on `8c3d72e` collapsed them into a single PRD. A re-review of that revision identified two structural problems (F1 — verify shadows ignored the parent-replay closure; F4 — the design conflated lifecycle with verification freshness and routed state mutation through a read path). The supervisor adjudicated both with a binding redesign:

- Verify produces a **freshness overlay** on `FeatureStatus`, not a new lifecycle state.
- The shadow workspace replays the **hard-parent topological closure** before applying the target's recipe.

This second revision pass rewrites the PRD around those two decisions. Slug-naming convention change: `feat-feature-tested-state` is renamed in spirit to `feat-feature-verify-freshness` (the backlog row keeps its slug; the contract behind it is the freshness overlay).

### 0.2 Architecture decisions to lock in ADR-013

This PRD records the _what_ and the _why-now_. Per repo rule (`AGENTS.md` → "Context Preservation Rules" §6) every architecture choice here is captured in **ADR-013-verify-freshness-overlay.md** before any Wave 3 implementer slice begins. The seven decisions, all rewritten for the freshness model:

1. **D1** — freshness overlay model: verify writes a `Verify` sub-record on `FeatureStatus`. The `FeatureState` enum is unchanged.
2. **D2** — apply gate stays pure-lifecycle. Satisfaction set remains `{applied, upstream_merged}`. Freshness does **not** alter the gate.
3. **D3** — producer set: `verify` writes the freshness record; `amend` invalidates it. `test` is not a producer.
4. **D4** — backwards-compatibility contract: `Verify` is `omitempty`-marshalled; v0.6.1 repos round-trip byte-identical until verify runs.
5. **D5** — derived label transitions: four read-time-derived labels (`never-verified`, `verified-fresh`, `verified-stale`, `verify-failed`). No persisted lifecycle transitions.
6. **D6** — source-truth alignment with ADR-011 D6 / ADR-010 D5: the freshness record lives in `status.json`; never inferred from any artifact.
7. **D7** — `verify` is read-only on the working tree. Apply-simulation runs in a `gitutil` shadow worktree that **first replays the target's hard-parent topological closure** before applying the target's recipe (F1 closure-replay spec).

Any deviation during implementation requires an ADR-013 amendment before the slice merges.

### 0.3 Out of scope (cross-linked)

- `feat-reconcile-code-presence-verdicts` — making reconcile assert that recipe ops are still represented in HEAD. Verify reuses the same shadow primitive but does not change reconcile's verdict set.
- `feat-reconcile-fresh-branch-mode` — running reconcile against a freshly-checked-out upstream branch. `tpatch verify --fresh-branch` is not in scope.
- **`delete-file` recipe op.** The recipe-op JSON schema is frozen (CHANGELOG v0.6.1 Notes). Verify's recipe-replay check tolerates deletions the same way recipe autogen does (skip with stderr note); a real `delete-file` op needs its own ADR.
- **Anything that reads `artifacts/reconcile-session.json`.** Verify reads `status.Reconcile.Outcome` only (ADR-010 D5 + ADR-011 D6).
- **Changes to `tpatch land`'s behavior.** The v0.15.1 Wave B amendment reads
  `land`'s trailer block; it does not change what `land` writes. See
  `docs/prds/PRD-tpatch-land.md` §3.8 for the readers' contract.
- **Lifecycle state changes.** The `FeatureState` enum is not extended. Verify never mutates `state`.

---

## Summary

`tpatch verify <slug>` is a **read-only**, machine-checkable health command that runs every static and apply-simulation check we already know how to run for a single feature, prints a pass/fail report (human and `--json`), and writes a **freshness record** (`Verify` sub-record on `FeatureStatus`) capturing `verified_at`, `passed`, hashes of the recipe + patch as observed at verify time, and a snapshot of every hard parent's lifecycle state. The lifecycle state (`FeatureState`) is **never mutated** by verify.

> **Persistence note (LOG entry `3c122aa` Note 1).** The per-check
> `check_results` array is stdout-only on `--json`; it is **not**
> persisted to the `Verify` record. The persisted shape is the minimal
> set above — see `internal/store/types.go` `VerifyRecord` and §3.4.1.

A derived freshness label (`never-verified` / `verified-fresh` / `verified-stale` / `verify-failed`) is recomputed every time `ComposeLabels` (`internal/workflow/labels.go:89`) runs — purely at read time, from the freshness record + the current DAG snapshot. Drift in the recipe, the patch, or any hard parent's state flips the derived label to `verified-stale` without rewriting `status.json`.

The Git-like analogy: lifecycle states are commits (sticky, persisted, mutated only by explicit verbs); freshness is `git status` for the verify check (derived, read-time, no persistence beyond the last record).

> **Landed-feature amendment (2026-08-12, v0.15.1 Wave B / GH #8, rev-2).**
> After `tpatch land` commits a feature into reachable Git history, the
> HEAD-anchored V7/V8 baseline already contains it and forward-apply semantics
> stop describing the world: `write-file` recipes pass vacuously,
> `replace-in-file` recipes false-fail, `append-file` recipes pass while
> corrupting the shadow, and V8 always fails. **§3.6** defines the landed
> contract — a conservative trailer-grammar evidence reader over one
> raw+parsed `--topo-order --reverse` enumeration; **dual-anchor**
> verification pairing a historical replay at the **replay anchor's** single
> parent with an **index-isolated** current-HEAD materialization ladder whose
> reduced-context hunks **block**; closure arbitration decided solely by that
> non-mutating patch ladder; and V10 anchored historically with a later-touch
> signal taken from the **shipped ADR-029 `RequestedAt` + touched-path
> detector**. **§7.1** is its 118-row executable acceptance matrix. Binding
> ADR: ADR-013 Amendment 1 rev-2 (D8–D17).
>
> The shipped check set is **eleven** checks, V0–V10, with
> `write_file_preimage_fresh` last (`internal/workflow/verify.go:49-71`,
> appended at `:288-289`); §4.3's golden examples are corrected accordingly.

Verify reuses the v0.6.1 primitives end-to-end:

- `store.ValidateDependencies` / `store.ValidateAllFeatures` (M14.1) for dependency hygiene.
- `store.satisfiedBySHA` regex + reachability check (v0.6.1 — `internal/store/validation.go:38–44, 101–108`) re-run for drift since edit time.
- `gitutil.IsAncestor` (`internal/gitutil/gitutil.go:680`) for parent-SHA reachability and `satisfied_by` revalidation.
- `gitutil.CapturePatchScoped` / `CaptureDiffStatScoped` (`internal/gitutil/gitutil.go:216`) for any drift-vs-recorded checks.
- `gitutil.CreateShadow` / `PruneShadow` (`internal/gitutil/shadow.go:56,122`) for apply-simulation in a throwaway worktree.
- `store.TopologicalOrder` (`internal/store/dag.go:107`) for the closure-replay ordering required by V7/V8.

The single schema change is the additive `Verify *VerifyRecord` field on `FeatureStatus`, with `omitempty`. No new file, no new artifact directory, no new `Reconcile.Outcome` value, no new `FeatureState` value.

---

## 1. Problem statement

### 1.1 What's missing today

Through v0.6.1 a feature reaches `applied` after `tpatch apply --mode execute` succeeds and stays there indefinitely. We have no way for an operator (or harness) to ask the cheap, structural question: "is this feature still healthy against the current tree?" The relevant signals are split across:

- **Static:** `spec.md` and `exploration.md` exist and reference real paths.
- **Recipe-shape:** `apply-recipe.json` (if present) parses; deps resolve; op targets resolve.
- **Apply-simulation:** the recipe still re-applies cleanly to a fresh shadow worktree **whose baseline already carries every hard parent's recipe replayed in topological order**.
- **Patch-replay:** `artifacts/post-apply.patch` still applies to that same closure-replayed baseline.
- **Dependency hygiene:** parent slugs exist, no cycles, `satisfied_by` SHAs still 40-hex AND reachable.
- **DAG context:** parent state is one of the apply-gate satisfying values.

Today these checks are scattered: dep validation runs at write time and during `status`; recipe parsing runs during `apply`; patch reverse-apply happens inside `reconcile`. There is no single command that runs them all without side effects, and there is no command that runs the apply-simulation against a baseline whose *parents* have first been replayed.

### 1.2 What the freshness overlay is for

The freshness overlay answers two questions:

1. **Did verify ever pass for this feature against the current world?** — operator/harness asks `verified-fresh ∈ labels`. This is needed because the harness must distinguish "the feature is `applied` and last verify said GREEN against unchanged inputs" from "the feature is `applied` and verify has never run."
2. **Has the world moved since the last verify?** — `verified-fresh` flips to `verified-stale` automatically when any of (a) the recipe hash, (b) the patch hash, (c) any hard parent's lifecycle state has drifted away from what was captured in the freshness record. No write to `status.json` is required to detect this; the derivation runs at read time.

The harness composition pattern is therefore: `tpatch verify parent && tpatch apply child`. The harness reads `verified-fresh` for the parent from `tpatch status --json` and decides whether to re-run verify on the parent before composing. The CLI itself does not enforce — gate-level enforcement is reserved for lifecycle states (see D2).

### 1.3 What `verify` is NOT

- `verify` is **not** `apply`. It does not write to the working tree. Apply-simulation runs in a `gitutil.CreateShadow` worktree that is pruned before exit.
- `verify` is **not** `reconcile`. It does not call the provider, does not run phase 3.5, does not produce a `reconcile-session.json`.
- `verify` is **not** `test`. It does not run `Config.TestCommand`. `tpatch test` remains the project-test runner.
- `verify` does **not** mutate `FeatureState`. Lifecycle is owned by `apply` / `amend` / `reconcile`. The freshness overlay is orthogonal.

---

## 2. Goals / non-goals

### Goals

- One read-only command, `tpatch verify <slug>`, that runs a fixed, ordered list of checks (§3.1) and produces both a human summary and a `--json` shape (§4.3).
- A new `Verify` sub-record on `FeatureStatus` (§3.4.1), produced by `verify`, invalidated by `amend`, never read by gates.
- Four derived freshness labels (§3.4.2), recomputed at read time by `ComposeLabels` (`internal/workflow/labels.go:89`).
- Hard-parent topological closure replay in V7/V8 (§3.4.3), so verify is structurally meaningful for non-leaf features.
- Byte-identical round-trip for v0.6.1 repos that never run `verify` (§3.4.4).
- Reuse of every existing primitive that overlaps a verify check (§3.1) — no new validation logic where the store already has it.

### Non-goals (explicit)

- **Provider calls.** Verify is offline by construction.
- **Verify mutates `FeatureState`.** It does not. There is no demote-on-fail, no promote-on-pass at the lifecycle level.
- **Apply gate consults freshness.** It does not. The gate stays pure-lifecycle (D2).
- **`tpatch test` integration in Wave 3.** `test` is not a producer of the freshness record; deferred to `feat-tested-state-test-producer`.
- **`amend --state tested`.** Manual flip is rejected; `tested` is not a state any more.
- **Code-presence reconcile verdicts.** Out of scope (§0.3).
- **Fresh-branch reconcile mode.** Out of scope (§0.3).
- **Schema additions to recipe-op JSON.** Frozen.
- **A new `verify-session.json` artifact.** Verify writes only `status.json`.

---

## 3. Specification

### 3.1 The check list — exact order, primitives, severity

`tpatch verify <slug>` runs the checks below. Severities:

- **`block`** — failure produces a non-zero exit and `passed=false` in the freshness record. Verify continues running so the operator gets the full picture.
- **`warn`** — failure is reported in the per-check entry but does not flip `passed` to `false`.

V0 aborts early — a feature whose `status.json` cannot be loaded cannot be verified meaningfully (`exit 2 — internal error`, no record written).

| #  | Check id                       | Trigger                            | Severity    | Pass criterion | Reuses |
|----|--------------------------------|------------------------------------|-------------|----------------|--------|
| V0 | `status_loaded`                | always                             | block-abort | `store.LoadFeatureStatus(slug)` returns OK | `internal/store/store.go:232` |
| V1 | `intent_files_present`         | always                             | block       | `spec.md` and `exploration.md` exist on disk under `.tpatch/features/<slug>/` and are non-empty | direct fs |
| V2 | `recipe_parses`                | recipe present                     | block       | `apply-recipe.json` parses through canonical unmarshal with `DisallowUnknownFields` | existing recipe loader |
| V3 | `recipe_op_targets_resolve`    | recipe present                     | block       | every op's `Path` exists OR carries a `created_by` whose parent is a declared **hard** dep currently in `applied`/`upstream_merged` | M14.2 `created_by` semantics, `internal/workflow/created_by_gate.go:57` |
| V4 | `dep_metadata_valid`           | always                             | block       | `store.ValidateDependencies(s, slug, status.DependsOn)` returns nil | `internal/store/validation.go:66` |
| V5 | `satisfied_by_reachable`       | every dep with `satisfied_by` set  | block       | `store.satisfiedBySHARe.MatchString` AND `gitutil.IsAncestor(repoRoot, dep.SatisfiedBy, "HEAD")` true | `internal/store/validation.go:101–108`, `internal/gitutil/gitutil.go:680` |
| V6 | `dependency_gate_satisfied`    | always (gated on `Config.DAGEnabled()`) | warn   | `workflow.CheckDependencyGate(s, slug)` returns nil | `internal/workflow/dependency_gate.go:42` |
| V7 | `recipe_replay_clean`          | recipe present                     | block       | After replaying the **hard-parent topological closure** of the target into a `gitutil.CreateShadow` worktree (§3.4.3), the target's recipe replays cleanly | `gitutil.CreateShadow`, `PruneShadow`, `store.TopologicalOrder`, recipe executor |
| V8 | `post_apply_patch_replay_clean`| `artifacts/post-apply.patch` present | block     | After the same closure replay used by V7, `git apply --check` of `post-apply.patch` succeeds against the closure-replayed shadow | shared shadow with V7; `git apply --check` |
| V9 | `reconcile_outcome_consistent` | `status.Reconcile.Outcome` set     | warn        | `Outcome ∈ {reapplied, upstreamed, still_needed}` | reads `status.Reconcile.Outcome` only — never any artifact (D6) |

#### 3.1.1 Ordering rationale

V0 → V6 are **static**: file checks, struct unmarshals, regex/git ancestor, in-process function calls. V7 and V8 are **dynamic** (shadow worktree + parent closure replay + recipe/patch apply). Static block runs first so a recipe-shape error doesn't waste a shadow allocation. V7 and V8 share a single shadow allocation: the closure is replayed once, the target recipe is applied, then `git apply --check` is run for the patch. V9 is last — informational read of `status.Reconcile.Outcome`.

#### 3.1.2 Remediation messages

Every fail surfaces a one-line remediation:

- V1 → `"file 'spec.md' missing or empty; re-run tpatch define <slug>"`
- V2 → `"apply-recipe.json failed to parse: <err>; fix the recipe or re-run tpatch implement <slug>"`
- V3 → `"recipe op #<n> path '<p>' missing and created_by empty; declare created_by=<parent> or apply <parent>"`
- V4 → wraps the validation sentinel verbatim.
- V5 → `"satisfied_by SHA <sha> for parent <slug> is no longer reachable from HEAD; re-run tpatch amend <slug> --remove-depends-on <parent> --depends-on <parent>"`
- V6 → `"hard parent <slug> in state=<state> (warn-only at verify time)"`
- V7 → `"recipe op #<n> failed in shadow replay: <err>; investigate or re-run tpatch implement <slug>"`
- V7 (parent-replay failure) → `"hard parent <slug> failed to replay in shadow: <err>; re-run tpatch verify <slug> on the parent first"` — see §3.4.3 for the exact JSON shape.
- V8 → `"post-apply.patch no longer applies to closure-replayed baseline; run tpatch reconcile <slug>"`
- V9 → `"reconcile outcome is <outcome>; verify cannot vouch for reconcile health (warn-only)"`

### 3.2 What `verify` writes

Verify is **read-only on the working tree** by ADR-013 D7. It writes exactly one thing, to the store:

1. The `Verify` sub-record on `FeatureStatus` (§3.4.1), including `verified_at`, `passed`, `recipe_hash_at_verify`, `patch_hash_at_verify`, and `parent_snapshot`. `LastCommand = "verify"` and `UpdatedAt` are bumped per existing `store.SaveFeatureStatus` semantics.

The per-check `check_results` array is **NOT persisted** — it is built in-memory and emitted on `--json` stdout only, per LOG entry `3c122aa` Note 1 (the authoritative disposition). The persisted record carries only the minimal field set needed by Slice B's read-time `ComposeLabels` derivation; the full diagnostic array is harness-consumable via `tpatch verify --json` stdout.

It writes **no** new file in `artifacts/`, no new file under `.tpatch/`, no new entry in `patches/`. The shadow worktree it spins up for V7/V8 is pruned before verify exits, regardless of pass/fail.

`FeatureState` is **not** mutated. There is no demote, no promote.

### 3.3 Dependency-gate severity is `warn`, not `block` — rationale

V6 (`dependency_gate_satisfied`) is warn because verify must be useful in two scenarios in which the hard-parent gate would fail despite the feature being structurally healthy:

1. **Pre-apply harness handoff.** A child whose hard parent is still `defined` is structurally fine. The harness wants to know "is this slug ready" before applying the parent.
2. **`upstream_merged` parent without `satisfied_by`.** Verify can detect structural drift via V5; the dep gate's behaviour for `upstream_merged` is to accept regardless. Warn lets V5 do the precise work and V6 echo context.

### 3.4 The verification freshness overlay

#### 3.4.1 The `Verify` sub-record on `FeatureStatus`

A new field is added to `FeatureStatus` (`internal/store/types.go:91`):

```go
type FeatureStatus struct {
    // … existing fields unchanged …
    Verify *VerifyRecord `json:"verify,omitempty"`
}

type VerifyRecord struct {
    VerifiedAt          time.Time           `json:"verified_at"`
    Passed              bool                `json:"passed"`
    RecipeHashAtVerify  string              `json:"recipe_hash_at_verify,omitempty"`
    PatchHashAtVerify   string              `json:"patch_hash_at_verify,omitempty"`
    ParentSnapshot      map[string]FeatureState `json:"parent_snapshot,omitempty"`
}

// VerifyCheckResult is built in-memory and emitted on --json stdout.
// It is NOT persisted to status.json (LOG entry 3c122aa Note 1).
type VerifyCheckResult struct {
    ID          string `json:"id"`
    Severity    string `json:"severity"`     // "block" | "block-abort" | "warn"
    Passed      bool   `json:"passed"`
    Remediation string `json:"remediation,omitempty"`
}
```

`Verify` is `omitempty`-marshalled: a `nil` pointer round-trips byte-identically with v0.6.1 status.json (D4). Hashes are SHA-256 of the canonical bytes of `apply-recipe.json` and `artifacts/post-apply.patch` respectively, computed at verify time. `ParentSnapshot` is keyed by parent slug; values are the parent's `FeatureState` as observed when verify ran.

The persisted record deliberately does **not** carry the per-check array. The full 10-row check results live on the in-memory `VerifyReport` and are emitted on `tpatch verify --json` stdout only (LOG entry `3c122aa` Note 1 — the authoritative disposition). Slice B's `ComposeLabels` derivation reads only the persisted minimal fields.

`Verify` is **not** a state, **not** a `Reconcile.Outcome`, and **not** an enum value on `FeatureState`. It is a freshness overlay.

#### 3.4.2 Derived freshness labels

The `ReconcileLabel` vocabulary (`internal/store/types.go:50–60`) is extended with four derived labels, recomputed every time `ComposeLabels` (`internal/workflow/labels.go:89`) runs:

| Label | Derivation |
|-------|-----------|
| `never-verified`   | `status.Verify == nil` |
| `verify-failed`    | `status.Verify != nil && status.Verify.Passed == false` |
| `verified-fresh`   | `status.Verify != nil && status.Verify.Passed == true` AND `recipe_hash_at_verify` matches `sha256(current apply-recipe.json)` (or both absent) AND `patch_hash_at_verify` matches `sha256(current post-apply.patch)` (or both absent) AND **for every `(parent_slug, parent_state)` in `parent_snapshot`**, the parent's current state satisfies `parent_state` (state-or-better; see below) |
| `verified-stale`   | `status.Verify != nil && status.Verify.Passed == true` AND any of the freshness conditions for `verified-fresh` is violated |

**State-or-better invariant for parent snapshots.** If the snapshot recorded a parent in `applied`, the parent currently being in `applied`, `upstream_merged` is acceptable (both satisfy the apply gate, so the structural guarantee the verify run leaned on is preserved). If the snapshot recorded a parent in `upstream_merged`, the parent currently being in `upstream_merged` is the only acceptable state (`upstream_merged` is terminal-by-design; transitioning out of it would be a manual-edit anomaly). For pre-apply states (`requested`/`analyzed`/`defined`/`implementing`), being in `applied` or `upstream_merged` is acceptable (the parent has only become more healthy). For `blocked` / `reconciling` / `reconciling-shadow`, any current state other than the recorded one invalidates freshness.

The four labels are **mutually exclusive**: exactly one applies to any given `FeatureStatus`. They live alongside the existing M14.3 labels (`waiting-on-parent`, `blocked-by-parent`, `stale-parent-applied`); compositions like `[verified-fresh, waiting-on-parent]` are valid and rendered by `tpatch status --dag`.

#### 3.4.3 V7/V8 hard-parent topological closure replay

This is the F1 closure-replay spec. Without it, V7/V8 are structurally useless for any non-leaf feature whose parent is locally `applied` (most of the DAG): the shadow worktree's baseline does not contain the parent's changes, so the target's recipe will fail to apply (op targets reference parent-created files; the patch references parent-modified hunks).

**Algorithm.**

1. Compute the hard-parent closure of the target slug: starting from `status.DependsOn`, follow only `DependencyKindHard` edges, transitively, until fixed point.
2. Order the closure deterministically using `store.TopologicalOrder` (`internal/store/dag.go:107`) over the hard-only sub-DAG. Parents appear before children.
3. Filter the ordered list:
   - Parents in `upstream_merged` are **skipped** — their changes are already on the shadow's baseline (the upstream tip), so replaying their recipe would be a no-op or cause double-apply errors.
   - Parents in `applied` are **replayed**: load `apply-recipe.json` for the parent, execute its ops in the shadow worktree.
   - Parents in any other state are a **fail-fast condition**: the closure cannot be reconstructed because the parent has no recorded recipe-replay-ready state. Verify aborts with `failed_at: "parent-replay"` and reports the offending parent and state.
4. After all replayable parents have replayed, apply the **target's** recipe in the same shadow. This is the V7 check.
5. After V7 succeeds, `git apply --check` the target's `post-apply.patch` against the same shadow tree. This is the V8 check.
6. Prune the shadow regardless of pass/fail.

**Fail-fast semantics.** The first parent that fails to replay causes verify to abort the V7/V8 phase (V0–V6 and V9 still run / have run). The freshness record is written with `passed=false` and the V7 entry's `remediation` carries the failing parent slug + wrapped error. The `--json` report includes a top-level `failed_at: "parent-replay"` and a `parent_slug` field; the human report includes the same.

**Example fail-fast JSON fragment.**

```json
{
  "verdict": "failed",
  "failed_at": "parent-replay",
  "parent_slug": "button-component",
  "checks": [
    { "id": "recipe_replay_clean", "severity": "block", "passed": false,
      "remediation": "hard parent button-component failed to replay in shadow: op #3 path 'src/button.tsx' already exists; re-run tpatch verify button-component first" }
  ]
}
```

**Why this is verify-only.** No other code path replays parent closures into shadows. `apply` works against the live tree (parent already applied locally). `reconcile` works against the upstream baseline + the target's own recipe (parents are out of band, by design — see ADR-010 D2). The closure-replay primitive is therefore not factored out into a shared helper; it lives in `internal/workflow/verify.go` only. If a future feature needs the same primitive, an ADR amendment factors it out.

**Cost.** O(closure size) shadow operations per verify. For a leaf with no hard parents, V7/V8 spin up the shadow once and apply only the target. For a 5-deep DAG, V7/V8 replay 5 recipes before the target. The cost is bounded by the DAG depth × per-recipe replay cost, comparable to a phase-2 reconcile op-replay pass per parent. Within the "cheap" budget verify is supposed to deliver for typical 1–3-deep DAGs; for pathologically deep DAGs the operator can verify parents first and rely on `verified-fresh` labels for the harness signal.

#### 3.4.4 Backwards-compatibility contract (D4)

A v0.6.1 repo that never runs `verify` must round-trip every `status.json` byte-identically through v0.6.2 read/write paths. Locked by:

- The `FeatureStatus` schema gains exactly one field: `Verify *VerifyRecord`, with `omitempty`. A `nil` pointer is omitted from the JSON output entirely. v0.6.1 fixtures (where the field never gets set) round-trip byte-identical.
- `FeatureState` enum is unchanged. No new value, no new state-write site.
- `ReconcileSummary` is unchanged. No new field.
- `Config` is unchanged. No new field, no new key in `.tpatch/config.yaml`.
- The `ReconcileLabel` vocabulary gains four values, but labels are **derived at read time** — they are never persisted to `status.json`. A v0.6.1 status.json round-trip never emits any of the new label strings.

Enforced by a regression fixture: `TestUpgradeFromV0_6_1_NoVerify_BehavesIdentically` — load v0.6.1 fixture, run every v0.6.1 command except `verify`, diff `.tpatch/` against v0.6.1 expected output, fail on any byte difference.

#### 3.4.5 Concurrency: verify during reconcile

Verify is read-only at the working-tree level, but V7/V8 spin up a shadow worktree. ADR-010 D2 reserves shadows for the M12 resolver, scoped per-slug:

- `tpatch verify <slug>` refuses (`exit 2 — feature is reconciling`) when the feature's lifecycle state is `reconciling` or `reconciling-shadow`.
- Verify creates its own shadow with the existing `<slug>-<timestamp>` naming convention; any prior verify shadow is reaped via the existing `gitutil.PruneAllShadows` semantics.
- The shadow is **always** pruned in a deferred call.

Verify on slug A does not block reconcile on slug B; the lock is per-slug.

#### 3.4.6 Apply gate: pure lifecycle, freshness ignored — D2

The apply gate (`workflow.CheckDependencyGate`, `internal/workflow/dependency_gate.go:79`) accepts hard parents in `{applied, upstream_merged}`. **This is unchanged in Wave 3.** The freshness overlay does not extend the satisfaction set.

The original first-revision PRD argued whether `tested` should join `{applied, upstream_merged}`. Under the freshness-overlay model that question is moot — there is no `tested` state to satisfy. The supervisor's binding adjudication on F4: lifecycle gates govern persistence, freshness governs harness composition. If the apply gate consulted freshness, it would re-create the demote-on-read problem from a different angle (a child applied at T1 against a `verified-fresh` parent could find its parent's freshness flipped to `verified-stale` at T2 with no operator action — a hidden retroactive change to gate satisfaction). Keeping the gate pure-lifecycle is the Git-like answer.

The harness composition pattern `tpatch verify parent && tpatch apply child` keeps working — but the value is harness-level, not gate-level. The harness reads `verified-fresh` from `tpatch status --json` and decides whether to re-run verify on the parent before composing. The CLI itself does not enforce.

#### 3.4.7 Parent-state hook: read-only label recomputation — F4 collapse

In the first-revision design, a "parent-state hook" was inserted into the M14.3 label-recomputation loop and (under one interpretation) was poised to mutate state. The reviewer flagged this as routing a write through `LoadFeatureStatus`, a read path. Under the freshness-overlay model the hook collapses entirely:

- The hook's role is **only** to recompute `verified-fresh` vs `verified-stale` (and the existing `stale-parent-applied`-style M14.3 labels). All four computations are pure functions of `(child.FeatureStatus, parent.FeatureStatus[])`.
- The hook lives in `composeLabelsFromStatus` (`internal/workflow/labels.go:143`), the existing read-time label computer.
- The hook **never** writes `.tpatch/`. There is no demotion edge, no state mutation, nothing persisted. Drift in a parent's state simply causes the next `ComposeLabels` call to derive `verified-stale` instead of `verified-fresh`. The persisted `Verify` record is unchanged until the next `tpatch verify` runs.

ADR-013 D5 captures the derived-label transitions: `never-verified → verified-fresh` (verify PASS), `verified-fresh → verified-stale` (drift), `verified-fresh → verify-failed` (verify FAIL on next run), `verify-failed → verified-fresh` (verify PASS on next run after the underlying issue is fixed). All transitions are observed by the operator via `ComposeLabels`; none are persisted as transitions.

### 3.5 Interaction with M14 reconcile labels

The freshness labels compose orthogonally with the M14.3 derived labels. ADR-011 D3 invariant is preserved: labels are computed at READ time from parent state. The two systems do not interact.

| Lifecycle state | Freshness label | M14.3 label(s) | Renders as |
|-----------------|-----------------|-----------------|------------|
| `applied` | `verified-fresh` | none | `applied [verified-fresh]` |
| `applied` | `verified-stale` | none | `applied [verified-stale]` |
| `applied` | `verify-failed` | `stale-parent-applied` | `applied [verify-failed, stale-parent-applied]` |
| `applied` | `never-verified` | `waiting-on-parent` | `applied [never-verified, waiting-on-parent]` |
| `upstream_merged` | any | n/a (M14.3 does not apply) | `upstream_merged [<freshness>]` |

The compound `EffectiveOutcome()` rule (`internal/store/types.go:192`) is **not extended**. Freshness labels are derived overlays, not `Reconcile.Outcome` values. The compound presentation logic is unchanged.

`amend` invalidates the freshness record (D3): a recipe-touching amend rewrites the recipe bytes, so `recipe_hash_at_verify` no longer matches; the next `ComposeLabels` derives `verified-stale`. Optionally — and this is the implementation hook in Slice B — `amend` may proactively clear `Verify.Passed` to `false` (effectively a `verify-failed` derived label) so the harness sees the invalidation immediately rather than waiting for the next read. ADR-013 D3 records this as the producer-set rule.

---

### 3.6 Landed-feature verification contract — v0.15.1 Wave B / GH #8 (rev-2)

> **Amendment status**: proposed rev-2, 2026-08-12, AWAITING REVIEW. Binding
> ADR: **ADR-013 Amendment 1 rev-2, decisions D8–D17.** Implementation is
> Wave C. Issue: <https://github.com/tesseracode/tesserapatch/issues/8>.
> Co-amended: `docs/prds/PRD-tpatch-land.md` §3.8.
>
> **Revision history.** rev-0 omitted **V10** and judged a landed feature only
> at `HEAD` with byte-exact predicates. rev-1 restored the eleven-check schema
> and introduced dual anchors but ran the current assertion against the
> **working tree**, left the `-C0` hardening optional, let an unavailable
> historical anchor degrade to skip-and-pass, reused whole-file byte equality
> and a circular reference to replay in parent arbitration, and derived
> later-touch from byte differences instead of the shipped ADR-029 detector.
> **rev-2 closes all eight rev-1 findings.** Every behavioural claim below was
> measured; the probe index is ADR-013 §A1.1 E1–E33.

#### 3.6.1 The defect this section closes

`tpatch land` commits a feature into reachable Git history while deliberately
leaving `status.apply.base_commit` untouched (`PRD-tpatch-land` §3.6;
`internal/cli/land.go:394`; ADR-019). V7/V8 allocate their shadow from
**current HEAD** (`internal/workflow/verify.go:1012`, `:1024`), so after
`land` the baseline already contains the feature.

Measured at `13a885c` with the real CLI; every run reported `checks=11` — the
shipped set is **V0–V10** (`internal/workflow/verify.go:49-71`), V10 appended
last (`:288-289`):

| Target recipe op kind | pre-land | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file`      | ✓ / ✓ | **✓ false green** (`internal/workflow/verify.go:1290-1294`) | **✗ false red** |
| `replace-in-file` | ✓ / ✓ | **✗ false red** — `search text not found` (`internal/workflow/verify.go:1295-1305`) | skipped |
| `append-file`     | ✓ / ✓ | **✓ false green, shadow double-appended** (`internal/workflow/verify.go:1306-1313`) | **✗ false red** |

The defect is not V8-only, and the same double-apply hazard applies to a
landed hard **parent** (`internal/workflow/verify.go:1048-1091`).

#### 3.6.2 Evidence reader — one enumeration, raw and parsed, conservative grammar (ADR-013 D10)

**Enumeration.** Exactly **one**
`git log --topo-order --reverse -z --format='%H%x1f%P%x1f<four trailers>%x1f%B'`
invocation per verify run, over commits reachable from the resolved `HEAD`,
**cached and reused for every feature** of a `verify --all` run. Records
arrive **oldest-first**. `rev-list` is **not** used — it cannot emit `%B`.
Never `--first-parent`.

**Conservative raw precedence.** `--grep '^Tpatch-Feature: '` may be used as a
cost prefilter, but the classification rule is:

> Any commit whose **raw** message contains a line that is exactly
> `Tpatch-Feature: <slug>` (after trimming trailing ASCII space/tab) for the
> slug under test, but whose **parsed terminal trailer block** does not yield
> that slug, is **`malformed`** — never `none`.

A prose quotation and an amend-destroyed trailer block are measurably
**indistinguishable** from the outside. rev-2 accepts a deliberate false red on
the prose case rather than read a destroyed attestation as "no attestation".

**Grammar (normative).**

| Element | Rule |
|---|---|
| Key case | Git matches trailer keys **case-insensitively** (`tpatch-feature:` parses). The reader inherits this and the contract states it. |
| `Tpatch-Feature` cardinality | **Exactly one value.** `land` emits exactly one (`internal/cli/land.go:397-400`). Two or more ⇒ `malformed`: the sibling SHA trailers cannot be attributed to a slug. |
| `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` / `Tpatch-Base-Commit` | **Exactly one each.** Zero or ≥2 ⇒ `malformed`. No "take the first"; duplicates are observable and ambiguous. |
| Slug match | Exact equality after trimming leading/trailing ASCII space and tab. Never prefix, never substring. |
| Formats | Patch-SHA 64 **lowercase** hex; Recipe-SHA 64 lowercase hex **or** literal `none`; Base-Commit 40 lowercase hex. Otherwise `malformed`. Follows the ADR-029 D1 precedent enforced by `isLowercaseHex` (`internal/workflow/writefile_safety.go:176`). |
| Reader failure | Git error, unparsable output, or a git below the §3.6.9 floor ⇒ **`unavailable`** — a block failure, distinct from `none` and from `malformed`. |

**Artifact-absence precedes digest mismatch.** Validation is ordered:

1. Canonical patch **absent** from the snapshot ⇒ the member is
   `landed-artifacts-absent`-eligible (§3.6.6). It is **not** reported as a
   digest mismatch — there is no digest to compare.
2. Only when the artifact is **present** is the digest compared. A
   present-but-zero-byte patch hashes to `sha256("")` and must compare equal.
   **Absent ≠ empty.**
3. `Tpatch-Recipe-SHA: none` matches both an absent recipe and a
   whitespace-only one, mirroring `readRecipeSHA`
   (`internal/cli/land.go:1034-1043`).

Because `land` refuses when the embedded `record` would capture nothing, a
landed member with an absent or zero-byte patch is a **corruption or hand-edit**
case; §5 classifies those rows explicitly rather than implying they are routine.

**Evidence states — closed set of eight, total:**

| Candidate population | State | Effect |
|---|---|---|
| no candidate, and no raw-body match | `none` | forward mode — today's behavior |
| exactly one well-formed candidate, all three values match | `exact` | landed mode |
| ≥2 such candidates, byte-equivalent per §3.6.8 | `duplicate-equivalent` | landed mode; `duplicates: n` |
| ≥2 such candidates, not byte-equivalent or not comparable | `ambiguous` | **FAIL** `failed_at: "landing-evidence"` |
| 0 all-match, ≥1 well-formed-but-mismatched | `stale` | **FAIL** `failed_at: "landing-evidence"` |
| cardinality / format / raw-only failures | `malformed` | **FAIL** `failed_at: "landing-evidence"` |
| candidates exist but none has exactly one parent | `unsupported-topology` | **FAIL** `failed_at: "landing-evidence"` |
| reader error | `unavailable` | **FAIL** `failed_at: "landing-evidence"` |

Only `none` degrades to forward mode. The other six non-`exact` states are
terminal.

#### 3.6.3 What each check independently proves in landed mode

| Check | Anchor | Independent obligation |
|---|---|---|
| **V7** `recipe_replay_clean` | historical | the recipe still **forward-applies**, by replay, to the tree it was authored against, with its closure reconstructed. Not derivable from V8 and never an alias of it. |
| **V8** `post_apply_patch_replay_clean` | historical **and** current | (a) at the anchor, `git apply --check` forward succeeds after the GH #2 reset — patch/recipe coherence, independent of V7; (b) at `HEAD`, the canonical patch is still **materialized** per the §3.6.5 ladder, evaluated in an isolated index. Both block-severity, reported separately. |
| **V10** `write_file_preimage_fresh` | historical baseline, plus a metadata-only current signal | each `write-file` op's `preimage_hash` still describes the tree it was recorded against, plus an ADR-029 **warn**-class later-touch derived from `RequestedAt` + touched-path metadata. |

#### 3.6.4 Baseline model — dual-anchor landed verification (ADR-013 D9)

- **Anchor H (historical)** — shadow rooted at the **replay-anchor** commit's
  single parent (§3.6.8). Closure arbitration (§3.6.6) runs, then the
  *existing, unmodified* machinery: V7 replays the recipe, the shadow is reset
  to `closureBaselineTree`, V8 runs `git apply --check` forward, V10 evaluates
  preimages.
- **Anchor C (current)** — an **index-isolated** assertion at `HEAD`
  (§3.6.5). No shadow, no worktree read, no real-index read.

**Implementation delta**: the commit-ish handed to `gitutil.CreateShadow`
(`internal/gitutil/shadow.go:56`, which already accepts any commit-ish) becomes
the replay anchor's parent instead of `gitutil.HeadCommit`
(`internal/workflow/verify.go:1012`, `:1024`), plus the §3.6.5 temp-index
ladder.

**GH #2 (v0.11.3) invariant, binding in every mode.** The recipe and the patch
are validated independently against the same baseline tree, with the shadow
reset to `closureBaselineTree` between them
(`internal/workflow/verify.go:1092`, `:1143`). Normative restatement: *any
check that may mutate the shadow MUST reset it to `closureBaselineTree` before
the next check runs; V7's result is never an input to V8's tree.* Anchor C
mutates nothing and allocates no shadow.

**Non-landed features are untouched**: evidence `none` ⇒ shadow at `HEAD`,
V7/V8/V10 byte-for-byte as today.

**Anchor H unavailability is TERMINAL** — see §3.6.8. rev-1's skip-and-pass is
withdrawn.

#### 3.6.5 Anchor C — index-isolated, with a mandatory `(0/0)` block (ADR-013 D11, D12)

rev-1 ran a bare `git apply --check --reverse`, which reads the **working
tree**. Measured: with the feature reverted in the worktree only (HEAD
unchanged) the check **FAILS** — a false red on a healthy landed feature —
while the isolated form succeeds. The symmetric dirty-index case is equally
unsound.

**Normative implementation.**

1. Create a temporary index at a path under `$(git rev-parse --git-dir)` —
   measured: invisible to `git status --porcelain`, real index byte-identical,
   worktree byte-identical — or, equivalently, under the already-gitignored
   `.tpatch/local/` root (`internal/cli/land_journal.go:31`, `:60`;
   `internal/gitutil/ignore.go:50-51`). It **must not** be created inside the
   tracked working tree; measured, a temp index there appears as an untracked
   entry.
2. `GIT_INDEX_FILE=<tmp> git read-tree <tree-ish>` — `HEAD` for the current
   assertion, or an arbitrary candidate tree for the §3.6.8 anchor probe.
3. `GIT_INDEX_FILE=<tmp> git apply --check --reverse --cached [-C0 --verbose] <patch>`.
4. Remove the temp index on **every** exit path, including every failure path,
   in a deferred cleanup.

Results are **memoised per `(tree, patch)` pair** for the run, so no tree is
probed twice.

**What reverse-apply proves.** It asserts that the patch's **postimage hunks
are present in the given tree**, matched by content with a line-offset search
and a configurable context requirement. It is **not** a byte-exact tree
comparison and **not** ownership evidence.

**The hardened ladder — Q14 is now MANDATORY and blocking.**

| Step | Command (all against the temp index) | Outcome |
|---|---|---|
| 1 | `git apply --check --reverse --cached <patch>` | pass ⇒ **materialized, clean** |
| 2 | on step-1 failure: `LC_ALL=C git apply --check --reverse --cached -C0 --verbose <patch>` | pass **and zero** `Context reduced to (0/0)` ⇒ **materialized, context drift**: the block check **passes** and a `warn` `context-drift` advisory is raised |
| 3 | step 2 passes **but reports ≥1** `Context reduced to (0/0)`, **or** step 2 fails | **BLOCK** — `landed-content-absent` |

`LC_ALL=C` is **mandatory** so the marker is locale-stable.

**Measured basis.** Per-scenario `C3` / `C0` / `(0/0)`-count triples over a
3-hunk patch in a 60-line file:

| Scenario | C3 | C0 | (0/0) | rule |
|---|---|---|---|---|
| pristine landed tip | OK | OK | 0 | clean |
| landing parent (feature absent) | FAIL | FAIL | 0 | block |
| 10 lines prepended | OK | OK | 0 | clean |
| unrelated edit far from any hunk | OK | OK | 0 | clean |
| unrelated edit **2 lines** from a hunk | FAIL | OK | 0 | **warn** |
| unrelated edit **1 line** from a hunk | FAIL | OK | **1** | **block** (accepted false red) |
| partial revert — hunk 1 | FAIL | FAIL | 0 | block |
| partial revert — hunk 2 | FAIL | FAIL | 0 | block |
| partial revert — hunk 3 | FAIL | FAIL | 0 | block |
| partial revert — hunks 1+3 | FAIL | FAIL | 0 | block |
| full revert of all hunks | FAIL | FAIL | 0 | block |
| degenerate whole-file hunk + header/footer | FAIL | OK | 0 | warn |
| **revert-in-place + identical text pasted at EOF** | FAIL | OK | **1** | **block** ✔ closes the rev-1 hole |
| a patched file deleted | FAIL | FAIL | 0 | block |

Randomized 220-tree corpus, 3-hunk patch in an 80-line file:

| Rule | false greens (69 absent trees) | false reds (151 present trees) |
|---|---|---|
| rev-1, `(0/0)` ignored | **2** | 0 |
| rev-2, any `(0/0)` blocks | **0** | 26 |

**Blocking is the chosen trade.** A false green tells an operator a feature is
healthy when its content is gone; a false red tells them to look, and the
remediation is real — the recorded context genuinely no longer matches HEAD,
so `tpatch record` + `tpatch land` re-attests and restores a clean pass
(remediation R2). A stronger hunk-local corroboration was considered and **not
adopted**: none could be *proved* on the measured corpus.

**No generalised claims.** rev-1's "fails at every level" sentence is
withdrawn. The only claims made are the per-scenario triples above, which
include **four distinct partial-revert shapes** and the full revert.

#### 3.6.6 Closure arbitration — non-mutating, patch-ladder-only (ADR-013 D13)

**The presence test for any closure member is the §3.6.5 hardened ladder
applied to that member's canonical `post-apply.patch` against the anchor tree,
probed through the temp index.** It is **non-mutating**, it is **not** recipe
replay (replay is what arbitration decides about, so it can never be the
deciding test), and it is **not** whole-file byte equality.

**Recipe operation predicates are diagnostics only.** They localise *which* op
and path a failure concerns and feed the §3.6.7 write-file signals. They never
certify presence and never on their own cause a member to be skipped.

| Member condition | Action |
|---|---|
| `upstream_merged` | **skip** (unchanged, `internal/workflow/verify.go:1062-1064`) |
| superseded by an active superseder | **skip** (unchanged, `internal/workflow/verify.go:976-983`) |
| evidence `exact`/`duplicate-equivalent`, patch present, ladder ⇒ clean or context-drift | **skip** — the member is on the anchor |
| evidence `exact`/`duplicate-equivalent`, patch present, ladder ⇒ block | **fail-fast** `parent-landing-drift` |
| evidence `exact`/`duplicate-equivalent`, patch **absent** or zero-byte, recipe present with ≥1 op | the **recipe** is the sole authority: replay decides, and a replay failure is `parent-landing-drift`. Corruption case — `land` cannot produce it. |
| evidence `exact`/`duplicate-equivalent`, **both** artifacts absent (or recipe zero-op and patch absent) | **fail-fast** `landed-artifacts-absent` — never skipped, never replayed, never assumed materialized |
| evidence `none`, patch present, ladder ⇒ clean or context-drift | **skip**, with a mandatory `warn` `unattributed-materialized` advisory naming the member. Verify claims **no ownership** of that content. |
| evidence `none`, patch present, ladder ⇒ block | **replay** (unchanged, `internal/workflow/verify.go:1065-1082`) |
| evidence `none`, patch absent | **replay** |
| evidence `stale` / `ambiguous` / `malformed` / `unsupported-topology` / `unavailable` | **fail-fast** `parent-evidence-integrity` |
| `unapplied` | **fail-fast** `parent-unapplied` |
| `rejected` | **fail-fast** `parent-rejected` |
| any other state | **fail-fast** (unchanged `default:`) |

A landed `exact` member additionally contributes its applicable **V7/V8 and
V10** results: skipping on the ladder settles *presence* only, and does not
excuse the member from the §3.6.7 V10 evaluation, whose block-class outcome
participates in `parent-landing-drift` and whose warn-class outcome is
advisory.

**Revert timing is qualified.** "Reverted" means the member's canonical patch
fails the ladder **at the anchor tree being built**. A revert landed *after*
the anchor commit is invisible at anchor H and is caught at anchor C; a revert
predating the anchor makes the content absent from the anchor and is caught
there. The anchors are evaluated independently and both are reported.

**`active` is total.** `active` is treated **identically to `applied`**
everywhere in the closure. Today the switch handles only `upstream_merged` and
`applied`, so an `active` hard parent reaches `default:` and fail-fasts
(`internal/workflow/verify.go:1061-1089`) — while `CheckDependencyGate`
accepts both (`internal/workflow/dependency_gate.go:79-81`),
`postApplyVerifyStates` admits `active`
(`internal/workflow/verify.go:127-134`) and `isPostApplyState` does too
(`internal/workflow/verify_all.go:89-97`). Widening the switch is the smallest
change that makes all four sites agree. **This is a deliberate behaviour change
for non-landed features**, pinned by AC-L74/AC-L75 and carried as a §8 risk row.

**Worked mixed chains.**

- *Target unlanded, P1 landed, P2 applied-unlanded* — anchor is `HEAD`; P1's
  patch passes the ladder there and is skipped; P2's fails and is replayed; the
  target is forward-verified exactly as today.
- *Target landed, P1 applied-unlanded* — anchor is the replay anchor's parent;
  P1's patch fails the ladder there and is replayed; then V7/V8/V10 run at
  anchor H and the ladder runs at anchor C.
- *Target landed, P1 landed but reverted before the anchor* — fail-fast
  `parent-landing-drift` before the target is judged.

#### 3.6.7 V10 — anchor-H preimage plus the shipped ADR-029 later-touch detector (ADR-013 D15)

`checkWriteFilePreimage` reads the target from `repoRoot` — the **live working
tree** (`internal/workflow/writefile_safety.go:108-112`). For an **applied**
feature the live tree holds the *post*-image, so a genuine `preimage_hash`
never matches and an empty preimage collides with the now-existing file; both
were measured. Autogenerated recipes escape only because `RecipeFromPatch`
emits `{type,path,content}` with **no** `preimage_hash`
(`internal/workflow/recipe_autogen.go:114-118`), taking the ADR-029 D4 legacy
path.

**Historical half.** In landed mode V10's reference tree is the **anchor-H
closure baseline** — the shadow after arbitration and *before* the target's
recipe replays.

| Case | Outcome |
|---|---|
| `preimage_hash` absent | legacy pass, no re-warn (unchanged, ADR-029 D4, `internal/workflow/verify.go:879-883`) |
| present and matching at the anchor-H baseline | **PASS**, `mode: "historical-anchor"` |
| present and **not** matching at the anchor-H baseline | **FAIL**, block severity. Downgraded to `warn` when superseded (unchanged, ADR-029 D7, `internal/workflow/verify.go:862-870`) |
| `preimage_hash` malformed per ADR-029 D1 | **FAIL**, block — the preimage contract itself is invalid |
| V2 skipped or failed | **skip**, unchanged reason (`internal/workflow/verify.go:853-861`) |
| anchor H unavailable | **FAIL** with `historical-anchor-unavailable` (§3.6.8) — not a skip, and never a live-tree fallback |

**Current half — later-touch from metadata, not bytes.** rev-1 derived
later-touch from "the path's content at HEAD differs from the landing's
postimage", a byte comparison that fires on the operator's own manual edits and
on unrelated formatting. rev-2 uses the **shipped detector**:

- ordering by `RequestedAt` — a feature is *later* iff its `RequestedAt` is
  non-empty and strictly greater than the current slug's
  (`internal/workflow/writefile_safety.go:409-442`);
- "touched" is the path-level union of `patch-generations.json.touched_paths`
  (`internal/store/patch_generations.go:52`) and the feature's
  `apply-recipe.json` operation paths
  (`internal/workflow/writefile_safety.go:449-481`);
- the index is `path → first later slug`, alphabetically-first for determinism
  (`internal/workflow/writefile_safety.go:380-388`);
- the per-op predicate is `checkLaterTouch`
  (`internal/workflow/writefile_safety.go:489-498`); the exported record-time
  entry point is `DetectRecordLaterTouchWarnings`
  (`internal/workflow/writefile_safety.go:571`).

A later-touch hit on a landed feature's `write-file` path raises a **`warn`
`later-touch` advisory** and **never blocks** — ADR-029 D6 makes later-touch
warning-class while D5 makes stale preimages on effective features fail, so the
*preimage* gate blocks and the *later-touch* signal warns. The single stated
exception: if the baseline/preimage contract is itself invalid — a malformed
`preimage_hash`, or a mismatch at the anchor — that blocks on its own terms,
independent of any later-touch.

**Parent V10 aggregation.** Each closure member is evaluated with the same two
halves. A member's **block-class** outcome contributes to
`parent-landing-drift` for that member. A member's **warn-class** outcome is
aggregated into the run's advisory list, attributed to the member's slug, and
never affects any verdict. The target's own V10 row carries the target's
block-class result and reports `mode: "historical-anchor"`.

#### 3.6.8 Attestation vs replay anchor, topology, duplicates (ADR-013 D14, D16)

rev-1 conflated the two, so a re-record + re-land could permanently destroy
anchor H. Measured: after a re-land, the new landing's parent tree **already
materializes** the current canonical patch (ladder `C3=OK`) and cannot supply a
baseline, while the **earlier** landing's parent does not (`C3=FAIL`) and can —
even though the earlier landing's own hashes are stale.

**Attestation candidate** — determines `landing_evidence.state`. Well-formed,
single-`Tpatch-Feature`, exact-slug, and its three recorded values match the
current snapshot. This is the **authority**; `state: "exact"` refers only to it.

**Replay-anchor candidate** — supplies anchor H's root and nothing else. It
must be:

1. reachable from `HEAD`;
2. carrying exactly one `Tpatch-Feature` value equal to the slug with a
   parseable terminal trailer block — **its hashes may be stale**; it is not an
   authority;
3. **single-parent** (`%P` cardinality exactly 1);
4. a commit whose **parent tree does not already materialize** the current
   canonical patch, probed with the §3.6.5 temp index and ladder at that parent
   tree.

**Selection is deterministic**: iterate the single enumeration in its native
`--topo-order --reverse` (oldest-first) order and take the **first** candidate
satisfying 1–4; final tie-break, the lexicographically smallest full SHA.
**No broadening** — the search never falls back to "any commit that looks like
it introduced these paths".

**Ambiguity.** If two or more candidates satisfy 1–4 and their
`git diff <C>^ <C> -- <P…>` bytes differ, the anchor is ambiguous and treated
as unavailable. If identical, the first in topo order is used.

**Unavailability is TERMINAL.** If no candidate satisfies 1–4 the run fails
with `failed_at: "historical-anchor-unavailable"`; V7, V8's historical half and
V10 are reported **failed-because-unanchored**, not skipped, and the run
**never** passes on anchor C alone.

**Re-land remediation regains anchor H or fails loudly.** After the R5
remediation (`tpatch record` + `tpatch land`), the new landing is the
attestation candidate and the earlier landing remains a valid replay anchor, so
anchor H is regained. If no candidate qualifies, the run fails with
`historical-anchor-unavailable` rather than silently degrading. Both branches
are pinned (AC-L14, AC-L15).

**Topology.** A replay-anchor candidate must have exactly one parent. Measured:
a root landing has zero parents and `git read-tree <root>^` fails with
`fatal: Not a valid object name`; a merge landing has two while its trailer
parses normally. Candidates with 0 or ≥2 parents ⇒ **`unsupported-topology`**;
`^1` is never used as an approximation. Reachability is full-graph; a merge
commit is a candidate only if it itself carries the trailer.

**Duplicate-equivalence of attestation candidates.** Path set `P` from
`gitutil.FilesInPatchStrict`
(`internal/gitutil/patch_paths_strict.go:253`), sorted byte-wise. If `P` is
**empty**, candidates are **not comparable** ⇒ `ambiguous`; never broadened.
Otherwise compare the raw bytes of

```
git diff --no-color --no-ext-diff --no-textconv --binary \
         --no-renames --unified=3 <C>^ <C> -- <P...>
```

**Rebase / cherry-pick / branch switch / detached HEAD / rewrite — total.**
Trailers survive rebase and cherry-pick verbatim while SHA and parent change,
so evidence keys on trailer *values*; both classify `exact`, possibly with
`base_commit_reachable: false` (advisory only). A branch switch that removes
the landing yields `none` ⇒ forward mode. A detached `HEAD` is evaluated
identically from whatever `HEAD` resolves to; the resolved commit is reported.
A rewrite leaving no reachable landing yields `none`; one leaving two is
decided by the rules above.

#### 3.6.9 Snapshots, diagnostics, remediation and implementability (ADR-013 D17)

**Immutable snapshot.** At the start of a run verify captures, once, for the
target and every closure member: the decoded `FeatureStatus`, and the
**presence flag** plus **raw bytes** of `artifacts/apply-recipe.json` and
`artifacts/post-apply.patch`. Every later stage — evidence digests, V7, V8,
V10, the persisted `VerifyRecord`, the derived labels — consumes **copies from
that snapshot** and never re-reads disk. Empty-present is distinct from absent
at every consumer. Before the report is finalised each snapshotted artifact is
re-read and compared; a difference ⇒ **FAIL** `failed_at: "snapshot-unstable"`
naming the path.

**Read-only guarantees.** No worktree mutation, no real-index mutation, no
`status.json` write beyond the existing `Verify` record — and none at all under
`--no-write` (`internal/workflow/verify.go:310-314`). Measured for the anchor-C
probe: the real index is byte-identical, the worktree is byte-identical,
`git status --porcelain` output is unchanged, and the temp index never appears
as an untracked entry. The shadow is still pruned via the existing deferred
call (`internal/workflow/verify.go:1036-1040`); the temp index is removed on
every exit path.

**Remediation must never route a just-landed local feature to `reconcile`.**
The current V8 text is `post-apply.patch no longer applies to
closure-replayed baseline; run tpatch reconcile <slug>`
(`internal/workflow/verify.go:1167`). The forward-mode string is **unchanged**;
landed mode uses its own strings.

Exact strings (Wave C emits these verbatim; `<slug>`, `<n>`, `<path>`, `<sha>`,
`<state>` interpolate):

| # | Condition | Check | Exact remediation |
|---|---|---|---|
| R1 | anchor-C ladder blocks (step 2 failed) | V8 | `landed feature: post-apply.patch postimage is not present at HEAD; landing commit <sha> is reachable but the content is absent — inspect with git diff <sha> HEAD, then re-record and re-land. Do NOT run tpatch reconcile: this is local drift, not upstream drift` |
| R2 | anchor-C ladder blocks on a reduced-context hunk | V8 | `landed feature: post-apply.patch matched at HEAD only with all context discarded at <path>; verify refuses to certify an unanchored match — inspect with git diff <sha> HEAD -- <path>, then re-record so the captured context matches HEAD and re-land` |
| R3 | anchor-C step 1 fails, step 2 passes with no reduced-context hunk | V8 (warn advisory) | `landed feature: post-apply.patch content is present at HEAD but its recorded context has drifted at <path>; a later change touched the surrounding lines — inspect with git diff <sha> HEAD -- <path> and re-record if the feature should absorb it` |
| R4 | anchor-H V7 replay fails | V7 | `landed feature: recipe op #<n> failed to replay at the landing baseline <sha>: <err>; the recipe no longer describes the tree it was authored against — re-run tpatch record <slug> --regenerate-recipe and re-land` |
| R5 | anchor-H V8 forward check fails | V8 | `landed feature: post-apply.patch does not apply at the landing baseline <sha>; the patch and the landing attestation disagree — re-record and re-land` |
| R6 | evidence `stale` | V7 | `landing evidence for <slug> is stale: commit <sha> attests patch-sha=<sha> / recipe-sha=<sha> / base=<sha> but the current artifacts hash differently; re-run tpatch land <slug> to re-attest, or restore the attested artifacts` |
| R7 | evidence `ambiguous` | V7 | `landing evidence for <slug> is ambiguous: <n> reachable commits carry matching trailers with non-equivalent content (<sha>, <sha>, …); resolve the history or re-land so exactly one attestation is current` |
| R8 | evidence `malformed` | V7 | `landing evidence for <slug> is malformed: commit <sha> carries a Tpatch-Feature line that Git does not parse as a trailer, or a duplicated/ill-formed Tpatch-* value; restore the four-trailer block with git commit --amend, or re-land` |
| R9 | evidence `unsupported-topology` | V7 | `landing evidence for <slug> is unusable: commit <sha> has <n> parents and tpatch land emits single-parent commits; verify cannot derive a landing baseline from a root or merge commit — re-land <slug> on a linear commit` |
| R10 | evidence `unavailable` | V7 | `landing evidence for <slug> could not be read: <err>; verify requires git >= 2.25 for trailer enumeration and refuses to guess — upgrade git or report this environment` |
| R11 | **anchor H unavailable (terminal)** | V7 | `landed feature <slug> has no usable landing baseline: every reachable landing commit is a root, a merge, or has a parent that already contains this feature; verify will not certify a landed feature it cannot replay — re-run tpatch record <slug> and tpatch land <slug> to create a fresh single-parent landing` |
| R12 | V10 preimage mismatch at anchor H | V10 | `landed feature: recipe op #<n> <path> expected preimage <sha> at the landing baseline but observed <sha>; the recipe is stale against its own baseline — re-run tpatch record <slug> --regenerate-recipe and re-land` |
| R13 | V10 later-touch (metadata) | V10 (warn advisory) | `later-touch: later feature <slug> touched <path> after <slug> was recorded; replaying this write-file would silently revert it — review before any replay (ADR-029 D5/D6, warning-class)` |
| R14 | parent landed, ladder blocks at the anchor | V7 | `hard parent <slug> landed at <sha> but its canonical patch is not present at the verification baseline; verify <slug> first — do not re-apply it into the shadow` |
| R15 | parent evidence integrity | V7 | `hard parent <slug> has <state> landing evidence; verify <slug> first — replaying or skipping it would validate <target> against an unknown baseline` |
| R16 | parent `unapplied` | V7 | `hard parent <slug> is unapplied; its patch is deliberately absent from the tree — run tpatch apply <slug> before verifying <target>` |
| R17 | parent `rejected` | V7 | `hard parent <slug> is rejected (terminal); remove the hard dependency with tpatch amend <target> --remove-depends-on <slug>, or reopen <slug>` |
| R18 | parent `evidence none` but already present | V7 (warn advisory) | `unattributed-materialized: hard parent <slug> is not landed but its canonical patch is already present at the verification baseline; it was not replayed, and verify makes no claim about what produced it` |
| R19 | both artifacts absent on a landed member | V7 | `landed feature <slug> has neither apply-recipe.json nor post-apply.patch; materialization cannot be proven from an empty artifact set — re-run tpatch record <slug>` |
| R20 | snapshot instability | V7 | `verify aborted: <path> changed while verify was running; re-run tpatch verify <slug> with no concurrent tpatch or editor writes` |

Human report gains two lines above the check list:

```text
verify extra-button — passed
  baseline: historical-anchor @ 6316e46 (landing 54b405d) · current @ 9f2c1ab (isolated index)
  landing evidence: exact @ 54b405d (patch ✓ recipe ✓ base ✓)
  ✓ [block] recipe_replay_clean — replayed at landing baseline
  ✓ [block] post_apply_patch_replay_clean — coherent at baseline; materialized at HEAD
  …
  ✓ [block] write_file_preimage_fresh — preimages fresh at landing baseline
```

**Sticky `verify-failed` clearing is mode-agnostic.** No new freshness label is
added; the four §3.4.2 labels are unchanged and mutually exclusive. A passing
landed run persists the same `VerifyRecord` field set as a passing forward run
(`internal/store/types.go:290-296`), so the read-time derivation takes no mode
input and a feature stuck at `verify-failed` from a pre-fix false red clears on
the first passing run, with no migration and no manual edit.

**Honest invocation budget.**

| Purpose | Invocations |
|---|---|
| Evidence enumeration — one `git log --topo-order --reverse -z --format=…` incl. `%P` and `%B` | **1 per run**, cached across `verify --all`. **No `rev-list`.** |
| Shadow allocation at anchor H | 1 `CreateShadow` — already allocated today; only its commit-ish changes |
| Temp-index seed | 1 `git read-tree <tree-ish>` **per distinct tree probed** |
| Ladder | 1 `git apply --check --reverse --cached`, plus 1 `-C0 --verbose` when step 1 fails, **per `(tree, patch)` pair**, memoised |
| Replay-anchor selection | one seed + ladder per candidate examined, oldest-first, stopping at the first qualifying candidate |
| Duplicate-equivalence | 1 `git diff` per candidate, **only when ≥2 attestation candidates** |
| `base_commit_reachable` advisory | 1 `git merge-base --is-ancestor` per landed member |

New code is one generic reader in `internal/gitutil/` (candidate
`trailers.go`) returning raw **and** parsed records, plus a small temp-index
helper; **policy** stays in `internal/workflow/verify.go` per ADR-013 D7.
Everything else reuses `gitutil.HeadCommit`, `CreateShadow`/`PruneShadow`,
`gitutil.IsAncestor`, `store.TopologicalOrder`, `isFeatureSupersededIn`,
`sha256Hex`, `FilesInPatchStrict`, `checkWriteFilePreimage`,
`loadLaterFeatureTouches` / `checkLaterTouch`. **No new store field, no new
artifact, no schema migration, no new dependency, no new check ID.**

**Git floor.** `%(trailers:key=…,valueonly)` needs git ≥ 2.22 and `separator=`
needs ≥ 2.25; verified on 2.55.0. Below the floor the reader **fails** ⇒
evidence `unavailable` ⇒ block, never `none`.

---
## 4. CLI surface

### 4.1 `tpatch verify` flags

Slice A delivers a minimal flag set; richer flags arrive in Slice D.

| Flag | Default | Slice | Description |
|------|---------|-------|-------------|
| `<slug>` | required | A | The feature to verify. |
| `--json` | false | A | Emit the structured JSON report on stdout. Human report on stderr unless `--quiet`. |
| `--quiet` | false | A | Suppress human report. Combined with `--json`, only JSON is emitted. |
| `--no-write` | false | A | Run all checks but do NOT write the `Verify` record. Pure read-only mode. |
| `--path` | `.` | A | Standard tpatch flag — workspace path. |

**Out of Slice A:**

| Flag | Slice | Description |
|------|-------|-------------|
| `--all` | D | Verify every post-apply feature in topological order; aggregate report. |
| `--shadow` | (rejected) | Force shadow allocation even when no recipe is present. Verify already gates V7/V8 on recipe/patch presence; flag is unnecessary. |
| `--fresh-branch` | (out of scope) | Verify against a freshly-checked-out upstream branch. Belongs to `feat-reconcile-fresh-branch-mode`. |

`--no-promote` / `--no-demote` from the first-revision PRD are **dropped** — there is no lifecycle promotion or demotion to opt out of.

### 4.2 `tpatch amend` interaction

Slice B: `tpatch amend <slug>` with a recipe-touching change invalidates the freshness record. Concretely the `Verify.Passed` bit is set to `false` (or `Verify` is cleared entirely; ADR-013 D3 picks one and locks it). `tpatch amend --state tested` is rejected (no such state exists); the existing v0.6.1 amend flag set is otherwise unchanged. No new flag is added.

### 4.3 `tpatch verify --json` output schema

Slice A. The schema is **frozen at the version field**: every consumer reads `schema_version` and refuses unknown majors.

> **rev-1 golden-example refresh (2026-08-12).** The shipped check set is
> **eleven** rows, V0–V10, with `write_file_preimage_fresh` last
> (`internal/workflow/verify.go:49-71`, appended at `:288-289`). The
> §4.3.1–4.3.5 examples below predate V10 and are corrected here to eleven
> rows. `schema_version` moves `"1.0"` → `"1.1"` for the landed fields of
> §4.3.6–4.3.9; the compatibility guarantee is **additive semantic
> compatibility, not byte identity** — the additive `baseline`,
> `landing_evidence` and `target_mode` objects are emitted for *every*
> feature, including one with no landing evidence.

#### 4.3.1 PASS — green verdict, freshness recorded

```json
{
  "schema_version": "1.0",
  "slug": "extra-button",
  "verified_at": "2026-04-27T18:30:11Z",
  "verdict": "passed",
  "exit_code": 0,
  "checks": [
    { "id": "status_loaded",          "severity": "block-abort", "passed": true, "remediation": "" },
    { "id": "intent_files_present",   "severity": "block",       "passed": true, "remediation": "" },
    { "id": "recipe_parses",          "severity": "block",       "passed": true, "remediation": "" },
    { "id": "recipe_op_targets_resolve","severity": "block",     "passed": true, "remediation": "" },
    { "id": "dep_metadata_valid",     "severity": "block",       "passed": true, "remediation": "" },
    { "id": "satisfied_by_reachable", "severity": "block",       "passed": true, "remediation": "" },
    { "id": "dependency_gate_satisfied","severity": "warn",      "passed": true, "remediation": "" },
    { "id": "recipe_replay_clean",    "severity": "block",       "passed": true, "remediation": "" },
    { "id": "post_apply_patch_replay_clean","severity": "block", "passed": true, "remediation": "" },
    { "id": "reconcile_outcome_consistent","severity": "warn",   "passed": true, "remediation": "" },
    { "id": "write_file_preimage_fresh","severity": "block",     "passed": true, "remediation": "" }
  ],
  "lifecycle_state": "applied",
  "recipe_hash_at_verify": "sha256:7a1b…",
  "patch_hash_at_verify": "sha256:9f24…",
  "parent_snapshot": { "button-component": "applied" }
}
```

#### 4.3.2 NEVER-VERIFIED rendering — when `tpatch status --json` (not verify) reports a feature with no record

Verify itself does not emit a `never-verified` report — every verify run produces a record. This shape is what `tpatch status --json` emits for a feature whose `Verify` is `nil`:

```json
{
  "slug": "extra-button",
  "state": "applied",
  "labels": ["never-verified"]
}
```

#### 4.3.3 STALE — verify last passed but a parent has drifted

Same: not emitted by `verify` (which writes a fresh record). This is `tpatch status --json` after a parent was amended:

```json
{
  "slug": "extra-button",
  "state": "applied",
  "labels": ["verified-stale"],
  "verify": {
    "verified_at": "2026-04-26T10:00:00Z",
    "passed": true,
    "parent_snapshot": { "button-component": "applied" }
  }
}
```

#### 4.3.4 FAIL — block-severity check, freshness records the failure

```json
{
  "schema_version": "1.0",
  "slug": "extra-button",
  "verified_at": "2026-04-27T18:31:02Z",
  "verdict": "failed",
  "exit_code": 2,
  "checks": [
    { "id": "status_loaded",          "severity": "block-abort", "passed": true,  "remediation": "" },
    { "id": "intent_files_present",   "severity": "block",       "passed": true,  "remediation": "" },
    { "id": "recipe_parses",          "severity": "block",       "passed": true,  "remediation": "" },
    { "id": "recipe_op_targets_resolve","severity": "block",     "passed": false,
      "remediation": "recipe op #2 path 'src/extras/button.css' missing and created_by empty; declare created_by=button-component or apply button-component" },
    { "id": "dep_metadata_valid",     "severity": "block",       "passed": true,  "remediation": "" },
    { "id": "satisfied_by_reachable", "severity": "block",       "passed": true,  "remediation": "" },
    { "id": "dependency_gate_satisfied","severity": "warn",      "passed": true,  "remediation": "" },
    { "id": "recipe_replay_clean",    "severity": "block",       "passed": false,
      "remediation": "recipe op #2 failed in shadow replay: open src/extras/button.css: no such file or directory" },
    { "id": "post_apply_patch_replay_clean","severity": "block", "passed": true,  "remediation": "" },
    { "id": "reconcile_outcome_consistent","severity": "warn",   "passed": true,  "remediation": "" },
    { "id": "write_file_preimage_fresh","severity": "block",     "passed": true,  "remediation": "" }
  ],
  "lifecycle_state": "applied"
}
```

#### 4.3.5 PARENT-REPLAY FAIL — closure-replay aborted at a hard parent

```json
{
  "schema_version": "1.0",
  "slug": "extra-button",
  "verified_at": "2026-04-27T18:32:14Z",
  "verdict": "failed",
  "exit_code": 2,
  "failed_at": "parent-replay",
  "parent_slug": "button-component",
  "checks": [
    { "id": "status_loaded",          "severity": "block-abort", "passed": true,  "remediation": "" },
    { "id": "intent_files_present",   "severity": "block",       "passed": true,  "remediation": "" },
    { "id": "recipe_parses",          "severity": "block",       "passed": true,  "remediation": "" },
    { "id": "recipe_op_targets_resolve","severity": "block",     "passed": true,  "remediation": "" },
    { "id": "dep_metadata_valid",     "severity": "block",       "passed": true,  "remediation": "" },
    { "id": "satisfied_by_reachable", "severity": "block",       "passed": true,  "remediation": "" },
    { "id": "dependency_gate_satisfied","severity": "warn",      "passed": true,  "remediation": "" },
    { "id": "recipe_replay_clean",    "severity": "block",       "passed": false,
      "remediation": "hard parent button-component failed to replay in shadow: op #3 path 'src/button.tsx' already exists; re-run tpatch verify button-component first" },
    { "id": "post_apply_patch_replay_clean","severity": "block", "passed": true, "skipped": true,
      "reason": "skipped: parent-replay aborted before V8" },
    { "id": "reconcile_outcome_consistent","severity": "warn",   "passed": true,  "remediation": "" },
    { "id": "write_file_preimage_fresh","severity": "block",     "passed": true,  "remediation": "" }
  ],
  "lifecycle_state": "applied"
}
```

#### 4.3.6 LANDED-PASS — dual-anchor verification green (v0.15.1 Wave B / GH #8, rev-2)

`schema_version` moves `"1.0"` → `"1.1"` (`internal/workflow/verify.go:83`).
The compatibility guarantee is **additive semantic compatibility, not byte
identity**: `baseline`, `landing_evidence` and `target_mode` are emitted for
*every* feature, so a no-evidence report is **not** byte-identical to a `"1.0"`
report. Consumers refuse unknown **majors** (§4.3), so 1.1 is non-breaking by
construction. The `checks` array is **eleven** rows in V0–V10 order in every
shape below.

> **`freshness_label` is not a verify-report field.** The shipped
> `VerifyReport` (`internal/workflow/verify.go:139-166`) has no such member;
> the derived label belongs to `tpatch status --json` (§4.3.2, §4.3.3, §4.5).
> rev-1's samples carried it, as did the pre-implementation §4.3.1/§4.3.4/
> §4.3.5 samples. It has been removed from every **verify** sample rather than
> introduced casually. Adding it would be a deliberate schema change with its
> own row.

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verified_at": "2026-08-12T18:30:11Z",
  "verdict": "passed",
  "exit_code": 0,
  "baseline": {
    "mode": "dual-anchor",
    "current_commit": "9f2c1ab4…",
    "current_probe": "isolated-index",
    "historical_anchor": {
      "state": "available",
      "commit": "6316e465…",
      "replay_anchor_commit": "54b405df…"
    }
  },
  "landing_evidence": {
    "state": "exact",
    "attestation_commit": "54b405df…",
    "candidates": 1,
    "parent_count": 1,
    "patch_present": true,
    "recipe_present": true,
    "patch_sha_match": true,
    "recipe_sha_match": true,
    "base_commit_match": true,
    "base_commit_reachable": true
  },
  "target_mode": "landed",
  "checks": [
    { "id": "status_loaded",                 "severity": "block-abort", "passed": true },
    { "id": "intent_files_present",          "severity": "block",       "passed": true },
    { "id": "recipe_parses",                 "severity": "block",       "passed": true },
    { "id": "recipe_op_targets_resolve",     "severity": "block",       "passed": true },
    { "id": "dep_metadata_valid",            "severity": "block",       "passed": true },
    { "id": "satisfied_by_reachable",        "severity": "block",       "passed": true },
    { "id": "dependency_gate_satisfied",     "severity": "warn",        "passed": true },
    { "id": "recipe_replay_clean",           "severity": "block",       "passed": true, "mode": "historical-anchor" },
    { "id": "post_apply_patch_replay_clean", "severity": "block",       "passed": true, "mode": "dual-anchor",
      "anchor_results": { "historical": "passed", "current": "materialized-clean" } },
    { "id": "reconcile_outcome_consistent",  "severity": "warn",        "passed": true },
    { "id": "write_file_preimage_fresh",     "severity": "block",       "passed": true, "mode": "historical-anchor" }
  ],
  "lifecycle_state": "applied",
  "recipe_hash_at_verify": "sha256:7a1b…",
  "patch_hash_at_verify": "sha256:9f24…",
  "parent_snapshot": { "button-component": "applied" }
}
```

**Field reference.**

| Field | Type | Meaning |
|---|---|---|
| `baseline.mode` | string | `"head-anchored"` (forward) \| `"dual-anchor"` (landed) |
| `baseline.current_commit` | string | the resolved `HEAD`; the anchor-C tree, and the shadow root in forward mode |
| `baseline.current_probe` | string | always `"isolated-index"` — records that anchor C read a temp index seeded by `read-tree`, never the worktree or the real index (§3.6.5) |
| `baseline.historical_anchor.state` | string | `"available"` \| `"unavailable"` \| `"not-applicable"` (forward mode) |
| `baseline.historical_anchor.commit` | string | the anchor tree — the replay anchor's single parent; omitted when unavailable |
| `baseline.historical_anchor.replay_anchor_commit` | string | the selected **replay-anchor** commit (§3.6.8), which may differ from the attestation commit; omitted when unavailable |
| `baseline.historical_anchor.reason` | string | why unavailable; omitted otherwise |
| `landing_evidence.state` | string | closed set of eight — `none` \| `exact` \| `duplicate-equivalent` \| `stale` \| `ambiguous` \| `malformed` \| `unsupported-topology` \| `unavailable` |
| `landing_evidence.attestation_commit` | string | the **authority** commit; omitted when `state` is `none` or `unavailable` |
| `landing_evidence.candidates` | int | attestation candidates examined |
| `landing_evidence.duplicates` | int | equivalent attestations; omitted when < 2 |
| `landing_evidence.parent_count` | int | `%P` cardinality of the attestation commit |
| `landing_evidence.patch_present` / `recipe_present` | bool | snapshot presence flags — evaluated **before** any digest comparison (§3.6.2) |
| `landing_evidence.patch_sha_match` / `recipe_sha_match` / `base_commit_match` | bool | digest/value comparisons; omitted for an artifact that is absent |
| `landing_evidence.base_commit_reachable` | bool | advisory only; `false` never fails on its own |
| `landing_evidence.reason` | string | classification detail for the non-`exact` states |
| `target_mode` | string | `"forward"` \| `"landed"` |
| `checks[].mode` | string | `"forward"` \| `"historical-anchor"` \| `"current-anchor"` \| `"dual-anchor"` — **present on V7, V8 and V10 in every report, including when they are skipped or failed**; **absent on V0–V6 and V9**, which have no anchor |
| `checks[].anchor_results` | object | V8 only: `{"historical": "passed"\|"failed"\|"skipped", "current": "materialized-clean"\|"materialized-context-drift"\|"absent"\|"skipped"}` |
| `advisories` | array | zero or more `{ "code", "severity": "warn", "slug", "path", "message" }` |

#### 4.3.7 LANDED with advisories — context drift and a metadata later-touch

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "passed",
  "exit_code": 0,
  "baseline": {
    "mode": "dual-anchor",
    "current_commit": "9f2c1ab4…",
    "current_probe": "isolated-index",
    "historical_anchor": { "state": "available", "commit": "6316e465…", "replay_anchor_commit": "54b405df…" }
  },
  "landing_evidence": { "state": "exact", "attestation_commit": "54b405df…", "candidates": 1, "parent_count": 1,
                        "patch_present": true, "recipe_present": true,
                        "patch_sha_match": true, "recipe_sha_match": true,
                        "base_commit_match": true, "base_commit_reachable": true },
  "target_mode": "landed",
  "checks": [
    { "id": "recipe_replay_clean",           "severity": "block", "passed": true, "mode": "historical-anchor" },
    { "id": "post_apply_patch_replay_clean", "severity": "block", "passed": true, "mode": "dual-anchor",
      "anchor_results": { "historical": "passed", "current": "materialized-context-drift" } },
    { "id": "write_file_preimage_fresh",     "severity": "block", "passed": true, "mode": "historical-anchor" }
  ],
  "advisories": [
    { "code": "context-drift", "severity": "warn", "slug": "extra-button", "path": "src/extras/button.css",
      "message": "landed feature: post-apply.patch content is present at HEAD but its recorded context has drifted at src/extras/button.css; a later change touched the surrounding lines — inspect with git diff 54b405df HEAD -- src/extras/button.css and re-record if the feature should absorb it" },
    { "code": "later-touch", "severity": "warn", "slug": "extra-button", "path": "src/extras/button.css",
      "message": "later-touch: later feature dark-mode touched src/extras/button.css after extra-button was recorded; replaying this write-file would silently revert it — review before any replay (ADR-029 D5/D6, warning-class)" }
  ],
  "lifecycle_state": "applied"
}
```

The `checks` array is abridged to the three anchored rows for readability; the
emitted array is always eleven rows. **The verdict is `passed`**: the
`context-drift` advisory means step 2 of the ladder passed with **zero**
reduced-context hunks (§3.6.5), and `later-touch` is warning-class per ADR-029
D6. Neither flips `passed`.

#### 4.3.8 LANDED-CONTENT-ABSENT — including the mandatory reduced-context block

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "failed",
  "exit_code": 2,
  "failed_at": "landed-content-absent",
  "baseline": {
    "mode": "dual-anchor",
    "current_commit": "9f2c1ab4…",
    "current_probe": "isolated-index",
    "historical_anchor": { "state": "available", "commit": "6316e465…", "replay_anchor_commit": "54b405df…" }
  },
  "landing_evidence": { "state": "exact", "attestation_commit": "54b405df…", "candidates": 1, "parent_count": 1,
                        "patch_present": true, "recipe_present": true,
                        "patch_sha_match": true, "recipe_sha_match": true,
                        "base_commit_match": true, "base_commit_reachable": true },
  "target_mode": "landed",
  "checks": [
    { "id": "recipe_replay_clean",           "severity": "block", "passed": true,  "mode": "historical-anchor" },
    { "id": "post_apply_patch_replay_clean", "severity": "block", "passed": false, "mode": "dual-anchor",
      "anchor_results": { "historical": "passed", "current": "absent" },
      "remediation": "landed feature: post-apply.patch matched at HEAD only with all context discarded at src/extras/button.css; verify refuses to certify an unanchored match — inspect with git diff 54b405df HEAD -- src/extras/button.css, then re-record so the captured context matches HEAD and re-land" },
    { "id": "write_file_preimage_fresh",     "severity": "block", "passed": true,  "mode": "historical-anchor" }
  ],
  "lifecycle_state": "applied"
}
```

V7 **passes** here: the recipe is still coherent at the landing baseline. That
is V7's independent contribution; the failure is purely "it is no longer
anchored in the tree", which only anchor C can see. The remediation shown is
R2, the reduced-context case — R1 is emitted when step 2 fails outright.

#### 4.3.9 TERMINAL states — evidence integrity and unavailable historical anchor

Evidence `stale` / `ambiguous` / `malformed` / `unsupported-topology` /
`unavailable` all carry `failed_at: "landing-evidence"`. A missing replay
anchor carries `failed_at: "historical-anchor-unavailable"`. In **both**
families V7, V8 and V10 report `passed: false` with `mode` present — they are
**failed-because-unanchored**, not skipped, and the run **never** passes on
anchor C alone.

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "failed",
  "exit_code": 2,
  "failed_at": "historical-anchor-unavailable",
  "baseline": {
    "mode": "dual-anchor",
    "current_commit": "9f2c1ab4…",
    "current_probe": "isolated-index",
    "historical_anchor": {
      "state": "unavailable",
      "reason": "every reachable landing commit is a root, a merge, or has a parent that already materializes this patch"
    }
  },
  "landing_evidence": { "state": "exact", "attestation_commit": "69c70bf7…", "candidates": 2, "parent_count": 1,
                        "patch_present": true, "recipe_present": true,
                        "patch_sha_match": true, "recipe_sha_match": true,
                        "base_commit_match": true, "base_commit_reachable": true },
  "target_mode": "landed",
  "checks": [
    { "id": "recipe_replay_clean",           "severity": "block", "passed": false, "mode": "historical-anchor",
      "remediation": "landed feature extra-button has no usable landing baseline: every reachable landing commit is a root, a merge, or has a parent that already contains this feature; verify will not certify a landed feature it cannot replay — re-run tpatch record extra-button and tpatch land extra-button to create a fresh single-parent landing" },
    { "id": "post_apply_patch_replay_clean", "severity": "block", "passed": false, "mode": "dual-anchor",
      "anchor_results": { "historical": "failed", "current": "materialized-clean" },
      "remediation": "landed feature extra-button has no usable landing baseline: the historical half of this check could not run; see recipe_replay_clean" },
    { "id": "write_file_preimage_fresh",     "severity": "block", "passed": false, "mode": "historical-anchor",
      "remediation": "landed feature extra-button has no usable landing baseline: preimage freshness cannot be evaluated without one; see recipe_replay_clean" }
  ],
  "lifecycle_state": "applied"
}
```

Note `anchor_results.current` is still reported as `materialized-clean` — the
content **is** present at HEAD — and the run still **fails**. That is the point
of D14: an unverifiable historical half is an unverified feature.

**Unified `failed_at` vocabulary — closed set.** Wave C must not emit any other
value; AC-L100 pins it.

| Value | Meaning |
|---|---|
| `parent-replay` | existing — a closure member failed to replay |
| `landing-evidence` | `stale` \| `ambiguous` \| `malformed` \| `unsupported-topology` \| `unavailable` |
| `historical-anchor-unavailable` | no candidate satisfies §3.6.8 conditions 1–4 |
| `landed-content-absent` | the anchor-C ladder blocked (step 2 failed, or a reduced-context hunk was reported) |
| `landed-artifacts-absent` | a landed member has neither artifact (§3.6.6) |
| `landed-baseline-incoherent` | anchor-H V7 or V8 forward check failed |
| `parent-landing-drift` | a landed closure member's patch ladder blocked at the anchor, or its V10 block-class outcome fired |
| `parent-evidence-integrity` | a closure member's evidence is `stale`/`ambiguous`/`malformed`/`unsupported-topology`/`unavailable` |
| `parent-unapplied` | a hard parent is `unapplied` |
| `parent-rejected` | a hard parent is `rejected` |
| `snapshot-unstable` | an artifact changed while verify was running (§3.6.9) |

**Advisory `code` vocabulary — closed set**, all `warn` severity, none of which
flips `passed`: `context-drift`, `later-touch`, `unattributed-materialized`,
`base-commit-unreachable`.

### 4.4 Skill / harness updates

Slice D updates all 6 skill formats with a one-paragraph addition under the "Lifecycle" section:

> **Verify before composing.** When you finish `tpatch apply` and want a cheap, machine-checkable signal that the feature is structurally healthy, run `tpatch verify <slug>`. Verify writes a freshness record on the feature; downstream readers see a `verified-fresh` label until the recipe, patch, or any hard parent's state drifts, at which point the label flips to `verified-stale`. The lifecycle state is never changed by verify — `applied` stays `applied`. Verify is read-only on the working tree. It does **not** run the project's test suite; for that, use `tpatch test`.

Anchor list (parity-guard `assets/assets_test.go` extension):

- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/skills/copilot-prompt/...`
- `assets/skills/cursor/...`
- `assets/skills/windsurf/...`
- `assets/skills/generic/...`

### 4.5 Status rendering

Slice B: `tpatch status` gains the freshness label inline (`applied [verified-fresh]`). `tpatch status --dag` renders the same. `tpatch status --json` emits the `Verify` sub-record when present and the derived `freshness_label` in the labels array.

---

## 5. Edge cases / failure modes

| Case | Handling |
|------|----------|
| `verify <slug>` on a feature that does not exist | `exit 2 — feature not found`. No record write. |
| `verify <slug>` on a feature whose `status.json` is malformed | V0 fails (block-abort); `exit 2 — internal error`. No record write. |
| Recipe absent (`apply-recipe.json` missing) | V2/V3/V7 are skipped; V8 runs against the closure-replayed baseline if patch is present. V1/V4/V5/V6/V9 run. |
| `post-apply.patch` absent | V8 is skipped. |
| Both recipe and patch absent | Verify still runs static checks (V1/V4/V5/V6/V9). Reasonable for `applied`-from-pre-autogen-era features. |
| Hard parent in `defined` (not replayable) when V7 needs to replay | V7 fails with `failed_at: "parent-replay"`; freshness record `passed=false`. |
| Hard parent in `upstream_merged` | Skipped during closure replay (its changes are on the baseline). |
| `verify` during a concurrent `reconcile` on the same slug | Refused per §3.4.5. |
| Verify inside a non-tpatch-init repo | `exit 2 — not a tpatch workspace`. |
| Verify on a child whose parent has cycle drift | V4 fails (block); freshness record `passed=false`. |
| `--no-write` honoured with all checks run | Verify runs read-only on `.tpatch/`; freshness record is not updated. |
| Repo with `Config.FeaturesDependencies = false` | V4 still runs. V5 is a no-op. V6 is a no-op. V7 closure replay still runs (DAG flag does not gate hard-dep traversal). |
| Verify on a feature in `requested`/`analyzed`/`defined`/`implementing` | Refused with `exit 2 — feature is pre-apply, nothing to verify`. No record write. |
| Verify on `blocked` / `upstream_merged` | Allowed; runs all applicable checks; writes the freshness record. The harness can interpret `verified-fresh` on `upstream_merged` as "the feature is retired and the artifacts are still healthy." |

**Landed-feature rows (v0.15.1 Wave B / GH #8 rev-2 — see §3.6 for the contract).**

Artifact-absence rows are stated against a fact `land` itself enforces: `land`
refuses when the embedded `record` would capture nothing, so a landed feature
with an absent or zero-byte `post-apply.patch` is a **corruption or hand-edit**
case, not a normal outcome.

| Case | Handling |
|------|----------|
| **Anchor C isolation** | |
| Landed target, worktree dirty (feature reverted in the worktree only) | Anchor C reads a temp index seeded from `HEAD`, so the ladder is unaffected ⇒ **PASS**. Measured: the rev-1 worktree-based check **false-reds** here. |
| Landed target, index dirty (unrelated paths staged) | Same — the real index is never read. |
| Landed target, worktree contains the feature but `HEAD` does not | Anchor C blocks on the `HEAD` tree, correctly: the content is not committed. The worktree is irrelevant. |
| Any landed run | Real index byte-identical, worktree byte-identical, `git status --porcelain` unchanged, temp index invisible and removed on every exit path. |
| **Anchor C ladder** | |
| Later unrelated edit **far** from any hunk, or pure offset shift | Step 1 passes ⇒ **PASS, no advisory**. |
| Later unrelated edit **2 lines** from a hunk | Step 1 fails, step 2 passes with **zero** reduced-context hunks ⇒ **PASS + `context-drift` warn** (R3). |
| Later unrelated edit **1 line** from a hunk | Step 2 passes but reports `Context reduced to (0/0)` ⇒ **BLOCK** (R2). Accepted false red; measured 26/151 present trees. |
| Revert-in-place + identical postimage text pasted elsewhere in the file | Step 2 passes but reports `(0/0)` ⇒ **BLOCK**. This closes the rev-1 false green (measured 2/69 absent trees under the unhardened rule). |
| Partial revert — any single hunk of a multi-hunk patch, or any combination | Both steps fail ⇒ **BLOCK** (R1). Four distinct shapes measured. |
| Full revert of every hunk | Both steps fail ⇒ **BLOCK** (R1). |
| A patched file deleted at `HEAD` | Both steps fail ⇒ **BLOCK** (R1). |
| Degenerate whole-file hunk with header/footer added | Step 2 passes with zero `(0/0)` ⇒ **PASS + `context-drift` warn**. |
| **Historical anchor** | |
| Landed target, ordinary single-parent landing | Anchor is that landing's parent; V7/V8-historical/V10 run there. |
| Re-record + re-land: newest landing's parent already contains the feature | The newest landing is the **attestation** authority; the **earlier** landing supplies the replay anchor. Measured: the newest landing's parent ladder-passes (disqualified), the earlier one's blocks (qualified). |
| No candidate is single-parent, or every candidate's parent already materializes the patch | **FAIL `historical-anchor-unavailable`** (R11). V7, V8-historical and V10 are **failed-because-unanchored**, not skipped; the run never passes on anchor C alone. |
| Root landing commit (0 parents) | Not a replay-anchor candidate; if it is the only candidate ⇒ `unsupported-topology` for attestation and `historical-anchor-unavailable` for the anchor. `git read-tree <root>^` fails outright. |
| Merge commit carrying the trailer (≥2 parents) | Same — no `^1` approximation. |
| After the R6 re-land remediation | Anchor H is regained (the earlier landing still qualifies), or the run fails with R11. Never a silent degradation. |
| **Recipe op shapes** | |
| Landed target, `write-file` recipe | V7 **replays** at anchor H ⇒ ✓. Today V7 passes vacuously and V8 false-reds. |
| Landed target, `replace-in-file` recipe | V7 replays at anchor H ⇒ ✓. Today this false-fails with `search text not found`. |
| Landed target, `append-file` recipe | V7 replays at anchor H ⇒ ✓, and the shadow is **not** double-appended, because the anchor is the pre-landing tree. |
| Any op-kind predicate (`write-file` bytes, `append-file` suffix, `replace-in-file` existential inverse, `ensure-directory`) | **Diagnostic only.** Never certifies presence, never causes a skip. `replace-in-file` with an empty replacement is **undecidable** and defers to patch authority; `append-file` with empty content is undecidable; `write-file` never certifies by whole-file equality. |
| **Artifact presence** | |
| Landed, recipe present, patch present | Ladder decides presence; V7/V8/V10 all applicable. |
| Landed, recipe present, patch **absent** or zero-byte | Absence is evaluated **before** any digest comparison. The recipe becomes the sole authority for that member; corruption case. |
| Landed, recipe **absent**, patch present | `Tpatch-Recipe-SHA` must be the literal `none`; V7 skips with its existing reason; the canonical patch is the sole authority. |
| Landed, recipe present but **whitespace-only** | `readRecipeSHA` returns `none` (`internal/cli/land.go:1039-1041`), so the trailer must be `none`; V2 fails to parse ⇒ V7/V10 skip; the patch is the sole authority. |
| Landed, recipe present with **zero operations** | V7 has nothing to replay and cannot attest; the patch is the sole authority. The V7 row records `0 op(s)` rather than a vacuous pass. |
| Landed, **both artifacts absent** | **FAIL `landed-artifacts-absent`** (R19). Never treated as materialized. |
| Patch present and **zero bytes** with a trailer attesting `sha256("")` | Presence flag is true, digest matches, evidence `exact`. **Absent ≠ empty.** |
| **Evidence** | |
| Hand-rolled `git commit` with no trailers | Evidence `none` ⇒ forward mode ⇒ today's behavior. |
| Commit whose **prose body** quotes `Tpatch-Feature: <slug>` | **`malformed`** ⇒ FAIL. A deliberate, documented false red: a prose quote and an amend-destroyed trailer block are indistinguishable, and reading a destroyed attestation as "no attestation" is the unsafe direction. |
| Trailer block destroyed by a later `--amend` | **`malformed`** ⇒ FAIL — same rule, and the case it exists to protect. |
| Duplicate `Tpatch-Patch-SHA` / `Recipe-SHA` / `Base-Commit` | **`malformed`** ⇒ FAIL. No "take the first". |
| Two or more `Tpatch-Feature` values on one commit | **`malformed`** ⇒ FAIL. |
| Uppercase hex, wrong length, non-`none` non-hex `Recipe-SHA` | **`malformed`** ⇒ FAIL. |
| Lowercase trailer key (`tpatch-feature:`) | Git matches keys case-insensitively; the commit **is** a candidate. |
| Two reachable attestations, byte-equivalent on the patch's path set | **`duplicate-equivalent`** ⇒ landed mode, `duplicates: 2`. |
| Two reachable attestations, not byte-equivalent | **`ambiguous`** ⇒ FAIL. |
| Canonical patch declares **no** paths | Not comparable ⇒ **`ambiguous`**; never broadened to "all paths". |
| Cherry-picked or rebased landing | `exact`; `base_commit_reachable` may be `false` (advisory only). |
| `git` older than the §3.6.9 floor, or any reader error | **`unavailable`** ⇒ FAIL. Never `none`, never a false green. |
| **Parents** | |
| Landed hard parent, patch ladder clean at the anchor | Skipped — never replayed, so an `append-file` parent is not duplicated. |
| Landed hard parent, patch ladder blocks at the anchor | Fail-fast `parent-landing-drift` (R14) **before** the target is judged. |
| Landed hard parent, patch absent, recipe present | The recipe is the sole authority for that member; a replay failure is `parent-landing-drift`. |
| Landed hard parent, **both artifacts absent** | **FAIL `landed-artifacts-absent`** — not skipped, not replayed. |
| Hard parent with `evidence none` whose patch **ladder-passes** at the anchor | **Skipped** with a mandatory `unattributed-materialized` warn (R18). Verify claims no ownership. |
| Hard parent with `evidence none` whose patch ladder blocks, or which has no patch | **Replayed**, unchanged. |
| Hard parent in `active` | Treated **exactly as `applied`**. Today it fail-fasts through `default:`; this amendment widens the switch. |
| Hard parent in `unapplied` / `rejected` | Fail-fast with the named reason instead of the generic `default:` message. |
| Hard parent in `upstream_merged` / superseded | Skipped, unchanged. |
| Landed parent V10 block-class outcome (preimage mismatch or malformed hash at the anchor) | Contributes to `parent-landing-drift` for that member. |
| Landed parent V10 warn-class outcome (metadata later-touch) | Aggregated into `advisories`, attributed to the member's slug; affects no verdict. |
| Revert of a parent that lands **after** the anchor commit | Invisible at anchor H, caught at anchor C. Both anchors are reported. |
| **Run-level** | |
| `.tpatch/` artifact mutated while verify runs | **FAIL `snapshot-unstable`** (R20) naming the path. |
| `--no-write` on any landed path | All checks run, nothing persists (`internal/workflow/verify.go:310-314`). |
| `verify --all` over a mixed landed/unlanded set | One evidence enumeration total, cached and reused; ladder results memoised per `(tree, patch)`; output ordering unchanged. |

---

## 6. Open questions / decisions

The reviewer-adjudicated questions from the first revision (Q1–Q5) are listed for traceability; their resolutions still hold under the new model except where superseded.

### Q1 — V9 (`reconcile_outcome_consistent`) severity: warn vs block?

**Adjudicated, still binding: warn.** A feature can be structurally healthy while sitting in `Reconcile.Outcome = shadow-awaiting`. Demoting verify on V9 would make `verified-fresh` un-reachable for any feature with a pending reconcile.

### Q2 — `verify --all` on pre-apply slugs

**Adjudicated, still binding: skipped with a per-slug `"skipped: pre-apply state"` reason line in the JSON output, exit 0 if all post-apply slugs pass.** Slice D detail.

### Q3 — `passed` field name

**Adjudicated, still binding: retained.** `severity` carries gating; `passed` carries pass/fail intent.

### Q4 — Apply-gate D2 wording

**Superseded by F4.** The first-revision question "does `tested` satisfy hard deps?" is moot under the freshness-overlay model. D2 is now: apply gate is pure-lifecycle, freshness is ignored. See §3.4.6.

### Q5 — Parent-state hook placement

**Adjudicated, still binding under the new model: lives in `composeLabelsFromStatus` (`internal/workflow/labels.go:143`), the existing read-time label computer. No new hot path. Crucially: read-only — never writes `.tpatch/`.** See §3.4.7 for why this is even more restrictive under the freshness model.

### Q6 — Should `verify` clear `Verify.Passed = false` on amend, or just leave the freshness record stale?

**Decision: clear on amend.** The `Verify` record carries an embedded `recipe_hash_at_verify` and `patch_hash_at_verify`; an amend that rewrites the recipe causes the next `ComposeLabels` to derive `verified-stale` from hash drift alone, even if `Verify.Passed` is left at `true`. So strictly speaking the explicit clear is redundant. We clear anyway to make the invalidation visible at write time (operator inspecting `status.json` immediately after amend sees `passed: false`); without the clear, the record's `passed: true` is technically a live but stale claim. This is documented in ADR-013 D3.

### Q7 — `tpatch verify` exit codes

- `0` — verdict passed; freshness recorded.
- `2` — verdict failed (any block-severity check); also covers V0 abort, refused-state, non-existent slug.
- `1` — reserved for "verify aborted by signal / context cancellation"; no record write.

Stable across slices; documented in `--help`.

### Q8 — Open: does the `tpatch verify` recipe-replay tolerate parent-failure mid-closure by skipping the failed parent and trying the rest?

**No, fail-fast on first parent failure.** Spelled out in §3.4.3. Skipping a failed parent would make the V7 result meaningless (the target's recipe applied against a partial baseline). Re-confirmed at design time; no Slice B reconsideration.

---

### Q9 — Should landed detection use `git patch-id` or content equivalence instead of trailers?

**Decided: no. Trailers only (ADR-013 D8).** Byte-identical patches across
distinct features are a real, *detected* condition — `record`'s cross-feature
collision detector (ADR-018) exists precisely because of it — so content
equivalence cannot attest which feature owns the content. Reverse-apply
success was measured to succeed against a tree produced by an unrelated
actor. Trailers are the only attestation tpatch emits.

### Q10 — Does `land` need to persist the landing commit SHA?

**Decided: no (ADR-013 D8 rejected alternative (d); PRD-tpatch-land §3.8.3).**
ADR-019 already refused it on chicken-and-egg grounds, rebase and cherry-pick
would stale the field exactly when the trailers stay correct, and it would
create a second source of truth against ADR-011 D6 / ADR-010 D5. Evidence is
recomputed from reachable history at every verify run.

### Q11 — Should `status.apply.base_commit` change at land time?

**Decided: no; retained unchanged (`PRD-tpatch-land` §3.6 and §3.8.4).** §3.6.2 of this PRD
validates `Tpatch-Base-Commit` against `status.apply.base_commit`; making
`land` overwrite that field with the new HEAD would make every landed feature
instantly evidence-`stale`, and would break `record` auto-base resolution
(ADR-016). Zero migration is required for existing repos.

### Q12 — Should `verify --all` order landed features differently? — **RESOLVED: no**

**Decided (rev-1): ordering is unchanged, and this is not a residual.** rev-0
left it open. The argument for reordering was cost — landed features skip the
closure replay for already-materialized members — but §3.6.9's accounting
shows the dominant per-run cost (one evidence enumeration) is *shared* and
cached across all features, so reordering saves nothing measurable. Ordering
is part of the observable `--all` contract (§4.1 Slice D) and changing it
would break golden output for a benefit that does not exist. Pinned by
AC-L96.

### Q13 — Should `land` warn when a commit-msg hook appends prose after the trailer block? — **RESOLVED: non-blocking follow-up**

**Decided (rev-1): out of scope for Wave C, and explicitly non-blocking.** A
paragraph after the trailer block makes Git parse **no** trailers, which
§3.6.2 classifies as `malformed` — a block-severity failure with the R7
remediation naming `git commit --amend`. The operator is therefore already
told, precisely, at the next verify. A `land`-side warning would move the
signal earlier but cannot change any verdict, and `land` is behaviour-frozen
by this amendment (`PRD-tpatch-land` §6.2 AC-LD9). Tracked in
`docs/prds/PRD-tpatch-land.md` §3.8.5 as a follow-up, not as a Wave C
obligation.

### Q14 — Should the anchor-C `-C0` step treat reduced-context hunks as not-materialized? — **RESOLVED: yes, mandatory and blocking**

**Decided (rev-2): MANDATORY.** rev-1 left this SHOULD-level; the reviewers
correctly refused. Measured, the unhardened rule leaks **2 false greens over
69 postimage-absent trees**, and the `Context reduced to (0/0)` marker
(emitted under a pinned `LC_ALL=C`) fires on exactly the offending shape. The
hardened rule gives **0 false greens** at a cost of **26 false reds over 151
present trees**, every one carrying a genuine remediation (R2: the recorded
context no longer matches HEAD, so re-record and re-land).

A stronger hunk-local corroboration was considered and **not adopted**: none
could be *proved* on the measured corpus, and shipping an unproven
discriminator is what rev-1 was returned for. Safety over measured false reds
is the explicit choice. Pinned by AC-L23, AC-L24, AC-L25 and AC-L31.

### Q15 — Forward-mode V10 is wrong for un-landed applied features — **SCOPED OUT, tracked**

**Open, deliberately out of scope, justified.** Measured: with a genuine
`preimage_hash`, V10 fails for an `applied` feature that never landed
(`expected preimage sha256:5fb14…, observed sha256:fa6dd8…`), and an empty
`preimage_hash` collides with the now-existing file — because
`checkWriteFilePreimage` reads the **live working tree**
(`internal/workflow/writefile_safety.go:108-112`), which holds the
*post*-image. Autogenerated recipes escape only because `RecipeFromPatch`
omits the field (`internal/workflow/recipe_autogen.go:114-118`).

**Why it stays out of scope.** §3.6.7 fixes this **for landed features** by
re-anchoring V10 at the historical baseline. An un-landed feature has **no
anchor** — there is no landing commit and therefore no tree that the
`preimage_hash` describes — so the defect cannot be fixed by this contract at
all. Fixing it requires either a new artifact recording the preimage tree, or
a policy change to forward-mode V10 that would alter verdicts for features
that never landed. Both need their own issue and PRD.

**Why it does not block.** No row of this amendment depends on forward-mode
V10 being correct, and the landed contract does not make it worse. It is
recorded so a reviewer does not read the landed fix as a claim that V10 is now
correct everywhere.

### Q16 — Should verify emit `freshness_label` in its `--json` report? — **RESOLVED: no**

**Decided (rev-2): no.** The shipped `VerifyReport`
(`internal/workflow/verify.go:139-166`) has no such member; the derived label
belongs to `tpatch status --json` (§4.3.2, §4.3.3, §4.5). rev-1's landed
samples and the pre-implementation §4.3.1/§4.3.4/§4.3.5 samples all carried
it, which would have led an implementer to add a field nobody decided to add.
Every **verify** sample in §4.3 now omits it. Adding it later would be a
deliberate schema change with its own row. Pinned by AC-L97.


## 7. Acceptance criteria (combined verify + freshness ships when…)

- [ ] **ADR-013 merged** before any Wave 3 implementation slice lands.
- [ ] `go build ./...`, `go test ./...`, `gofmt -l .` all clean.
- [ ] `FeatureStatus.Verify *VerifyRecord` field present, `omitempty`-marshalled. v0.6.1 fixtures round-trip byte-identical.
- [ ] `FeatureState` enum is unchanged (no `StateTested`).
- [ ] `tpatch verify <slug>` runs the 10-check sequence in order, with the severities documented in §3.1.
- [ ] V7/V8 replay the **hard-parent topological closure** before applying the target's recipe (§3.4.3). Order is `store.TopologicalOrder` over the hard-only sub-DAG. `upstream_merged` parents are skipped.
- [ ] V7/V8 fail-fast on first parent-replay failure with `failed_at: "parent-replay"` and the failing parent slug in the JSON output.
- [ ] On green, `Verify` record is written with `passed=true`; no lifecycle mutation.
- [ ] On block-severity fail, `Verify` record is written with `passed=false`; no lifecycle mutation.
- [ ] `--no-write` runs all checks but does not write `Verify`.
- [ ] `--json` emits the schema in §4.3 with exact field names; `schema_version: "1.0"` is present.
- [ ] V0 abort produces `exit 2 — internal error` and writes nothing to `status.json`.
- [ ] V4 reuses `store.ValidateDependencies` (no parallel validator).
- [ ] V5 reuses the v0.6.1 `satisfied_by` 40-hex + `gitutil.IsAncestor` reachability contract.
- [ ] V6 reuses `workflow.CheckDependencyGate`; soft parents are silent.
- [ ] V7/V8 spin up a single `gitutil.CreateShadow` worktree, replay the closure, run target recipe + patch check, prune before exit.
- [ ] V8 uses `git apply --check` against the closure-replayed shadow tree.
- [ ] V9 reads `status.Reconcile.Outcome` only — never `artifacts/reconcile-session.json`. Adversarial test pins this.
- [ ] `verify` during in-flight reconcile on the same slug refuses with `exit 2`.
- [ ] `ComposeLabels` derives `never-verified` / `verified-fresh` / `verified-stale` / `verify-failed` per the §3.4.2 table. Composes orthogonally with M14.3 labels.
- [ ] `amend (recipe-touching)` invalidates the freshness record (clears `Verify.Passed`); `amend (intent-only)` does not.
- [ ] `tpatch amend --state tested` is rejected with a "no such state" error.
- [ ] **Apply gate is unchanged.** `CheckDependencyGate` satisfaction set remains `{applied, upstream_merged}`. Test `TestApplyGate_FreshnessIsIgnored` pins this.
- [ ] Skill bullet present in all 6 surfaces; parity guard (`assets/assets_test.go`) green.
- [ ] **Backwards compat:** `TestUpgradeFromV0_6_1_NoVerify_BehavesIdentically` — v0.6.1 fixture, all v0.6.1 commands run except `verify`, resulting `.tpatch/` is byte-identical to v0.6.1 expected.
- [ ] **Source-truth guard:** adversarial test asserts the verify implementation does NOT import or read `artifacts/reconcile-session.json` or `artifacts/resolution-session.json` at any code path.
- [ ] CHANGELOG v0.6.2 callout names `verify` and the freshness overlay with exact contract surface.

---

### 7.1 Acceptance matrix — landed-feature verification (v0.15.1 Wave B / GH #8, rev-2)

**Binding on the Wave C implementation dispatch.** Every row is a distinct,
executable acceptance criterion. **Tier** names where the row is proven:

- **U** — unit test, pure function or abstraction, no repo. Covers the §3.6.6
  diagnostic predicates, the §3.6.2 grammar and classifier over fixture byte
  strings, and anything expressed over the **snapshot abstraction** or the
  **evidence-reader abstraction** (construct the value, assert the output is a
  pure function of it).
- **W** — workflow integration test in `internal/workflow`, real temp Git repo
  + `store.Store`, calling `RunVerify` directly. Where a row must observe,
  count or perturb git behaviour, it uses a **`PATH` git wrapper**: a test-only
  shim script placed first on `PATH` that forwards to the real `git` and can
  log calls, inject output or mutate files between calls. Proven feasible.
  **No production seam, no build tag, no exported hook.**
- **C** — real-CLI test in `internal/cli`, executing the cobra surface
  end-to-end (`tpatch apply` → `record` → `land` → `verify`), asserting
  stdout/stderr/exit code.

`W+C` means the row must be proven at **both** tiers.

#### Group A — the reported defect and the eleven-check schema

| # | Criterion | Tier |
|---|---|---|
| AC-L1 | The issue #8 sequence — `apply --mode done` → `record` → `test` → `verify` — passes **before** `land`: exit 0, and the report contains exactly **eleven** check rows in V0–V10 order. | C |
| AC-L2 | The same feature **after** `tpatch land` passes: exit 0, `target_mode: "landed"`, `landing_evidence.state: "exact"`, `baseline.mode: "dual-anchor"`, still eleven rows. | W+C |
| AC-L3 | The issue's committed-range re-record is decided by the §3.6.2 values, and **both branches** are asserted: (a) byte-identical regenerated artifacts ⇒ evidence stays `exact`, verify passes with no re-land; (b) changed artifacts ⇒ `stale`, FAIL with R6 naming `tpatch land <slug>`, and pass after that re-land. Branch (b) is the reporter's path. | C |
| AC-L4 | A landed **leaf** with no dependencies passes. | W+C |
| AC-L5 | Every report — forward or landed, pass or fail — emits exactly eleven `checks` rows whose `id` values equal the eleven constants at `internal/workflow/verify.go:49-71`, in that order. Golden assertion; no 10-row or V9-last shape survives anywhere. | W |
| AC-L6 | `tpatch verify --no-write` on every AC-L row leaves `.tpatch/`, the real index and the worktree byte-identical (hash the `.tpatch/` tree, `git status --porcelain -z`, `git ls-files --cached -z` before and after). | W+C |

#### Group B — anchor C isolation

| # | Criterion | Tier |
|---|---|---|
| AC-L7 | Anchor C runs against a **temporary index** seeded by `GIT_INDEX_FILE=<tmp> git read-tree <tree>` and `git apply --check --reverse --cached`. Asserted by a `PATH` git wrapper that records the argv of every `apply` call and fails the test if any lacks `--cached`. | W |
| AC-L8 | Landed target with a **dirty worktree** (feature reverted in the worktree only, `HEAD` unchanged) ⇒ **PASS**. The rev-1 worktree-based check false-reds this fixture; the row exists to keep it fixed. | W+C |
| AC-L9 | Landed target with a **dirty index** (unrelated paths staged) ⇒ PASS; the real index is never read. | W |
| AC-L10 | Worktree contains the feature but `HEAD` does not ⇒ anchor C **blocks**. The worktree must not rescue an uncommitted feature. | W |
| AC-L11 | After any landed run — pass or fail — the real index is byte-identical, the worktree is byte-identical, `git status --porcelain -z` is unchanged, and the temp index does **not** appear as an untracked entry. | W+C |
| AC-L12 | The temp index is removed on **every** exit path, including every failure path (`landing-evidence`, `historical-anchor-unavailable`, `landed-content-absent`, `landed-artifacts-absent`, `snapshot-unstable`, every `parent-*`). Asserted by scanning the git dir and `.tpatch/local/` after each fixture. | W |
| AC-L13 | The temp index is created outside the tracked working tree (under the git dir or the gitignored `.tpatch/local/`); a fixture asserts it never shows in `git status`. | W |

#### Group C — the hardened ladder

| # | Criterion | Tier |
|---|---|---|
| AC-L14 | Step 1 (default context) passes ⇒ `anchor_results.current == "materialized-clean"`, no advisory. | W+C |
| AC-L15 | Offset shifts (lines prepended and appended) and an unrelated edit far from any hunk ⇒ step 1 still passes. | W |
| AC-L16 | Unrelated edit **2 lines** from a hunk ⇒ step 1 fails, step 2 passes with **zero** `Context reduced to (0/0)` ⇒ verdict **passed**, `materialized-context-drift`, `context-drift` advisory carrying R3. | W+C |
| AC-L17 | Unrelated edit **1 line** from a hunk ⇒ step 2 passes but reports `(0/0)` ⇒ **BLOCK** with R2. The accepted false red, pinned deliberately. | W |
| AC-L18 | **Revert-in-place + identical postimage text pasted elsewhere in the same file** ⇒ step 2 passes but reports `(0/0)` ⇒ **BLOCK**. This is the rev-1 false green; the row is the regression guard. | W |
| AC-L19 | **Partial revert of hunk 1** of a 3-hunk patch ⇒ both steps fail ⇒ BLOCK with R1. | W |
| AC-L20 | **Partial revert of hunk 2** ⇒ BLOCK. | W |
| AC-L21 | **Partial revert of hunk 3** ⇒ BLOCK. | W |
| AC-L22 | **Partial revert of hunks 1 + 3** (non-adjacent combination) ⇒ BLOCK. | W |
| AC-L23 | **Full revert** of every hunk ⇒ BLOCK. | W+C |
| AC-L24 | A patched file **deleted** at `HEAD` ⇒ BLOCK. | W |
| AC-L25 | Degenerate whole-file hunk with header/footer added ⇒ step 2 passes with zero `(0/0)` ⇒ PASS + `context-drift`. | W |
| AC-L26 | The `-C0` step runs with `LC_ALL=C` in its environment. Asserted by a `PATH` git wrapper that records the environment of each `apply` call. Locale-stability of the `(0/0)` marker depends on it. | W |
| AC-L27 | Ladder results are **memoised per `(tree, patch)` pair**: a closure with the same tree probed twice issues only one pair of `apply` calls. Counted by the wrapper. | W |
| AC-L28 | R1 contains the literal clause `Do NOT run tpatch reconcile`, and no landed-mode remediation contains the token `reconcile` outside that negation. Adversarial. | W |

#### Group D — historical anchor: attestation vs replay anchor

| # | Criterion | Tier |
|---|---|---|
| AC-L29 | Ordinary single-parent landing ⇒ the shadow root is that landing's **parent**, not `HEAD`. Asserted on the commit-ish passed to `CreateShadow`. | W |
| AC-L30 | **Re-record + re-land**: the newest landing is the **attestation** commit (`landing_evidence.attestation_commit`) while the **earlier** landing is the **replay anchor** (`baseline.historical_anchor.replay_anchor_commit`), because the newest landing's parent already ladder-passes. Both fields are emitted and differ. | W+C |
| AC-L31 | The anchor probe uses `read-tree <candidate-parent-tree>` + `apply --cached` and touches **no worktree**: `git status --porcelain -z` is unchanged across the whole selection loop. | W |
| AC-L32 | Anchor selection is deterministic: with several qualifying candidates, repeated runs pick the same one (first in `--topo-order --reverse`, then lexicographically smallest SHA). | U+W |
| AC-L33 | A candidate whose parent tree **already materializes** the current canonical patch is **disqualified** as an anchor, even when it is the attestation commit. | U+W |
| AC-L34 | The anchor search never broadens beyond exact-slug trailer commits — it never falls back to "any commit that introduced these paths". Adversarial: a non-trailer commit that introduced the paths must not be selected. | U+W |
| AC-L35 | **No qualifying candidate ⇒ FAIL `historical-anchor-unavailable`** with R11. V7, V8's historical half and V10 report `passed: false` with `mode` present — **failed-because-unanchored, not skipped**. | W+C |
| AC-L36 | Anchor unavailable while anchor C is **clean** ⇒ the run still **FAILS**. `anchor_results.current` is reported as `materialized-clean` and the verdict is `failed`. This is the rev-1 skip-to-C-pass hole. | W |
| AC-L37 | After the R6 re-land remediation, anchor H is **regained** and the run passes; if the operator's history admits no candidate, it fails with R11. Both branches asserted; neither degrades silently. | C |
| AC-L38 | Two candidates satisfy conditions 1–4 with **differing** `git diff <C>^ <C> -- <P…>` bytes ⇒ the anchor is ambiguous and treated as unavailable ⇒ R11. | W |
| AC-L39 | A **root** landing (0 parents) is never an anchor; `git read-tree <root>^` is never attempted blindly. | U+W |
| AC-L40 | A **merge** landing (≥2 parents) is never an anchor and is never approximated to `^1`. | U+W |

#### Group E — evidence reader, grammar and enumeration

| # | Criterion | Tier |
|---|---|---|
| AC-L41 | No reachable landing and no raw match ⇒ `state: "none"`, `target_mode: "forward"`, shadow at `HEAD`, and V7/V8/V10 verdicts **identical** to the pre-amendment implementation on the same fixture. | W+C |
| AC-L42 | All three values match ⇒ `exact`. | W+C |
| AC-L43 | `Tpatch-Patch-SHA` mismatch ⇒ `stale`, FAIL, `failed_at: "landing-evidence"`, with V8 and V10 also failed and `mode` present. | W+C |
| AC-L44 | `Tpatch-Recipe-SHA` mismatch ⇒ `stale`, FAIL. | W+C |
| AC-L45 | `Tpatch-Base-Commit` mismatch ⇒ `stale`, FAIL. | W |
| AC-L46 | `Tpatch-Recipe-SHA: none` matches an **absent** recipe and a **whitespace-only** recipe, mirroring `readRecipeSHA` (`internal/cli/land.go:1039-1041`). | U+W |
| AC-L47 | **Artifact absence precedes digest mismatch**: with the patch artifact absent, the report sets `patch_present: false` and omits `patch_sha_match` rather than reporting a mismatch. | U+W |
| AC-L48 | Patch **present and zero bytes** ⇒ digest `sha256("")`; a trailer attesting that value ⇒ `exact`. Two distinct fixtures assert **absent ≠ empty**. | U+W |
| AC-L49 | Missing any one of the four trailers ⇒ `malformed`. | U+W |
| AC-L50 | Duplicate `Tpatch-Patch-SHA`, `Recipe-SHA` or `Base-Commit` ⇒ `malformed`; the classifier must not select either duplicate. | U+W |
| AC-L51 | Two or more `Tpatch-Feature` values ⇒ `malformed`. | U+W |
| AC-L52 | A commit whose **raw** body carries an exact `Tpatch-Feature: <slug>` line that Git does **not** parse as a trailer ⇒ **`malformed`**, never `none`. Fixtures cover **both** the amend-destroyed block **and** the prose quotation; the prose false red is asserted as intended behaviour. | U+W |
| AC-L53 | Slug matching is exact after trimming ASCII space/tab: `my-slug` does not match `my-slug-extended`; a value with surrounding spaces does match. | U |
| AC-L54 | A **lowercase** trailer key (`tpatch-feature:`) **is** a candidate, because Git matches keys case-insensitively. | U+W |
| AC-L55 | Uppercase hex, wrong-length hex, or a non-`none` non-hex `Recipe-SHA` ⇒ `malformed`. | U |
| AC-L56 | A git error, unparsable output, or a git below the §3.6.9 floor ⇒ `state: "unavailable"`, FAIL — **never** `none`. Proven with a `PATH` git wrapper that exits non-zero on the enumeration call. | U+W |
| AC-L57 | `base_commit_reachable: false` raises the `base-commit-unreachable` advisory and **does not fail** on its own. | W |
| AC-L58 | The enumeration is exactly **one** `git log --topo-order --reverse -z` invocation per run, carrying `%H`, `%P`, `%B` and all four parsed trailers, **reused for every feature** of a `verify --all` run. Counted by the wrapper. | W |
| AC-L59 | **`rev-list` is never invoked.** Adversarial: the wrapper fails the test on any `rev-list` call. | W |
| AC-L60 | The enumeration yields records **oldest-first**, and anchor selection consumes that order directly. | U+W |
| AC-L61 | The §3.6.9 invocation budget is honoured: no per-candidate `git diff` unless ≥2 attestation candidates; at most one `read-tree` per distinct tree; at most two `apply` calls per `(tree, patch)` pair. Counted by the wrapper. | W |
| AC-L62 | Two reachable attestations, byte-equivalent on the patch's path set ⇒ `duplicate-equivalent`, `duplicates: 2`. | W |
| AC-L63 | Two reachable attestations with differing diffs ⇒ `ambiguous`, FAIL with R7. | W |
| AC-L64 | A canonical patch declaring **no** paths makes attestations incomparable ⇒ `ambiguous`; never broadened to "all paths". | U+W |
| AC-L65 | Cherry-picked and rebased landings ⇒ `exact`, PASS (two fixtures). | W |
| AC-L66 | Branch switch away from the landing ⇒ `none` ⇒ forward mode; branch switch away where equivalent content is present anyway ⇒ still `none`, forward mode, diagnostic states the content is unattributed. | W |
| AC-L67 | Detached `HEAD` is evaluated identically; `baseline.current_commit` reports the resolved commit. History rewritten with no reachable landing ⇒ `none`. | W |
| AC-L68 | A landing reachable only through a merge's **non-first** parent **is** found (full-graph reachability). | W |

#### Group F — closure arbitration and parents

| # | Criterion | Tier |
|---|---|---|
| AC-L69 | The presence test for every closure member is the §3.6.5 **patch ladder** at the anchor tree. Adversarial: a `PATH` wrapper asserts no recipe operation is executed for a member that is ultimately skipped, and that no whole-file byte comparison decides the outcome. | W |
| AC-L70 | A landed member whose patch ladder is clean at the anchor is **skipped** — its recipe is never executed. Asserted by an op-execution counter. | W |
| AC-L71 | A landed member with an `append-file` recipe is skipped and the anchor tree contains its payload exactly **once**. The double-apply regression. | W+C |
| AC-L72 | A landed member with a `replace-in-file` recipe is skipped and the closure does not fail with `search text not found`. | W |
| AC-L73 | A landed member whose patch ladder **blocks** at the anchor ⇒ fail-fast `parent-landing-drift` with R14, **before** the target is judged. | W |
| AC-L74 | An applied hard parent with `evidence none` whose patch **ladder-passes** is **skipped** with a mandatory `unattributed-materialized` advisory (R18); verify claims no ownership. This is the rev-0/rev-1 double-application hole. | W |
| AC-L75 | An applied hard parent with `evidence none` whose ladder **blocks**, or which has no patch, is **replayed**, byte-identically to today. | W+C |
| AC-L76 | Landed member with patch absent and a ≥1-op recipe ⇒ the recipe is the sole authority; a replay failure is `parent-landing-drift`. | W |
| AC-L77 | Landed member with recipe absent, zero-op, or whitespace-only ⇒ the patch ladder is the sole authority. | W |
| AC-L78 | Landed member with **both artifacts absent** ⇒ **FAIL `landed-artifacts-absent`** (R19). Never skipped, never replayed, never assumed materialized. | W |
| AC-L79 | Hard parent with `stale` / `ambiguous` / `malformed` / `unsupported-topology` / `unavailable` evidence ⇒ fail-fast `parent-evidence-integrity` (R15). | W |
| AC-L80 | Hard parent in `unapplied` ⇒ fail-fast `parent-unapplied` (R16), replacing today's generic `default:` message. | W+C |
| AC-L81 | Hard parent in `rejected` ⇒ fail-fast `parent-rejected` (R17). | W |
| AC-L82 | Hard parent in **`active`** is treated exactly as `applied` — skipped or replayed by the same arbitration, never fail-fast. Asserted for a **non-landed** target too, where it changes today's verdict. | W+C |
| AC-L83 | After AC-L82, all four `active` sites agree: `dependency_gate.go:79-81`, `verify.go:127-134`, `verify_all.go:89-97` and the closure switch. Adversarial cross-check. | U |
| AC-L84 | Hard parent in `upstream_merged` is still skipped, byte-identically to today. A superseded parent is still excluded by the existing filter; landed classification does not resurrect it. | W |
| AC-L85 | A revert that lands **after** the anchor commit is invisible at anchor H and is caught at anchor C; a revert predating the anchor is caught at anchor H. Both anchors reported. | W |
| AC-L86 | Parent landed **after** vs **before** the target produces identical verdicts; landing order is never consulted. Closure ordering for an all-unlanded closure is topological and identical to today. | W |
| AC-L87 | Mixed chain — target unlanded, P1 landed, P2 applied-unlanded ⇒ anchor `HEAD`, P1 ladder-skipped, P2 replayed, target forward-verified. | W+C |
| AC-L88 | Mixed chain — target landed, P1 applied-unlanded ⇒ anchor is the replay anchor's parent, P1 replayed there, target judged at both anchors. | W |

#### Group G — V10

| # | Criterion | Tier |
|---|---|---|
| AC-L89 | Landed target, recipe **without** `preimage_hash` (the autogen shape, `internal/workflow/recipe_autogen.go:114-118`) ⇒ V10 passes on the ADR-029 D4 legacy path with no re-warn. | U+W |
| AC-L90 | Landed target, `preimage_hash` **matching the anchor-H closure baseline** ⇒ V10 PASSES with `mode: "historical-anchor"`. **This fixture FAILS today**, because `checkWriteFilePreimage` reads the live working tree. | W+C |
| AC-L91 | Landed target, `preimage_hash: ""` (new-file) with the file **absent** at the anchor-H baseline ⇒ V10 passes. Today it fails with `new-file collision`. | W |
| AC-L92 | Landed target, `preimage_hash` **not** matching at the anchor-H baseline ⇒ V10 **FAILS** at block severity with R12. | W |
| AC-L93 | A **malformed** `preimage_hash` (ADR-029 D1 form violation) ⇒ V10 **FAILS** at block severity regardless of any later-touch state — the preimage contract itself is invalid. | U+W |
| AC-L94 | Later-touch is derived from the **shipped metadata detector**: `RequestedAt` ordering plus the union of `patch-generations.json.touched_paths` and recipe op paths (`internal/workflow/writefile_safety.go:380-388`, `:409-448`, `:449-470`, `:489-499`). Adversarial: a fixture where the path's **bytes** at HEAD differ but **no later feature** touched it must raise **no** `later-touch` advisory. | U+W |
| AC-L95 | A genuine later-touch (a later `RequestedAt` feature recording a touch on the path) ⇒ **`later-touch` warn advisory** (R13) and the verdict is **not** blocked by it. | W+C |
| AC-L96 | Superseded landed feature with a preimage mismatch at the anchor ⇒ severity downgraded to `warn`, unchanged from ADR-029 D7. | W |
| AC-L97 | Anchor H unavailable ⇒ V10 **fails** with `historical-anchor-unavailable`, never skips and never falls back to the live tree. V2 skipped or failed ⇒ V10 skips with its existing reason, unchanged in both modes. | W |
| AC-L98 | **Parent V10 aggregation**: a member's block-class outcome contributes to `parent-landing-drift` for that member; a member's warn-class later-touch appears in `advisories` attributed to the member's slug and affects no verdict. | W |

#### Group H — snapshots, schema, diagnostics and run-level guarantees

| # | Criterion | Tier |
|---|---|---|
| AC-L99 | Classification, V7, V8, V10, the persisted `VerifyRecord` and the derived labels are a **pure function of the §3.6.9 snapshot**. Proven at **U** over the snapshot abstraction with no filesystem access in the unit under test. | U |
| AC-L100 | An artifact mutated **while verify runs** ⇒ **FAIL `snapshot-unstable`** (R20) naming the path. Proven at **U** over the snapshot abstraction and at **W** with a `PATH` git wrapper that rewrites a `.tpatch/` artifact between the enumeration call and a later git call. **No production hook.** | U+W |
| AC-L101 | Every later stage consumes **copies** from the snapshot: an adversarial fixture that changes an artifact after capture but before the final re-read still yields digests computed from the captured bytes. | U |
| AC-L102 | Empty-present and absent artifacts are distinguished at **every** consumer — evidence classification, V7/V8/V10 preconditions, and the persisted hashes. Two fixtures, one per shape. | U+W |
| AC-L103 | `schema_version` is `"1.1"`. A no-evidence report is **semantically** compatible with `"1.0"` — every `"1.0"` key retains name, type and position — but is **not** byte-identical, because `baseline`, `landing_evidence` and `target_mode` are emitted for every feature. The test asserts the additive superset, not byte identity. | W |
| AC-L104 | **No verify report contains `freshness_label`.** Adversarial over every §4.3 shape, forward and landed. Q16. | W |
| AC-L105 | `checks[].mode` is present on V7, V8 and V10 in **every** report — passed, failed **and** skipped — and absent on V0–V6 and V9. Both halves asserted. | W |
| AC-L106 | `failed_at` only ever takes a value from the §4.3.9 closed set (11 values incl. `historical-anchor-unavailable`), and advisory `code` only from `{context-drift, later-touch, unattributed-materialized, base-commit-unreachable}`. Adversarial enumeration. | U+W |
| AC-L107 | Every R1–R20 remediation string is emitted **verbatim** (golden strings). | W |
| AC-L108 | The human report emits `baseline:` and `landing evidence:` above the check list, in that order, naming both anchors and the `isolated index` probe in landed mode. | W+C |
| AC-L109 | A passing landed run persists a `VerifyRecord` with the same field set as a passing forward run (`internal/store/types.go:290-296`) — no new persisted field, `omitempty` round-trip preserved. | W |
| AC-L110 | Sticky clearing is mode-agnostic: a feature at `verify-failed` from a pre-fix false red derives `verified-fresh` after one passing landed run, no migration, no manual edit; and `ComposeLabels` takes no mode input (adversarial: the labels package has no reference to landing evidence). | U+W |
| AC-L111 | `TestRunVerify_EquivalentRecipeAndPatchBothPass` (`internal/workflow/verify_closure_replay_test.go:275`) stays green **unmodified**. The GH #2 non-regression anchor. | W |
| AC-L112 | The GH #2 reset holds at anchor H: after V7 mutates the shadow, the tree hash seen by V8 equals `closureBaselineTree` (`internal/workflow/verify.go:1092`, `:1143`). | W |
| AC-L113 | The shadow is pruned on **every** exit path, including each new failure path. | W |
| AC-L114 | `verify --all` output ordering over a mixed landed/unlanded set is byte-identical to today (Q12 resolved: no reordering). | C |
| AC-L115 | Exit codes are unchanged: `0` pass, `2` any block-severity failure including every new terminal state, `1` reserved. Warn-class advisories never change the exit code. | C |
| AC-L116 | The `replace-in-file` diagnostic predicate is sound on the §3.6.6 exhaustive corpus (0 false reds, 0 false greens over the decided cases), including `c='abb', S='aa', R='b'` ⇒ true and `c='b', S='a', R='a'` ⇒ false; `R == ""` ⇒ undecidable; `S == ""` ⇒ unsupported. | U |
| AC-L117 | Diagnostic predicates never certify: an adversarial fixture where a `write-file` op's content matches byte-for-byte but the canonical patch ladder **blocks** must still FAIL; and one where `append-file` content is empty must report undecidable rather than pass. | U+W |
| AC-L118 | `gofmt -l .` clean, `go build ./cmd/tpatch` clean, `go test ./...` clean, and `make wave-close-check` 8/8 at the Wave C close commit. | C |

**Matrix size: 118 numbered criteria (AC-L1 … AC-L118) across 8 groups** —
A 6, B 7, C 15, D 12, E 28, F 20, G 10, H 20.
A Wave C dispatch that cannot place a row at its stated tier must amend this
section rather than silently re-tier it.

#### 7.1.1 Explicit non-goals for Wave C

- No change to `tpatch land` behaviour, output or refusals
  (`PRD-tpatch-land` §3.8 is a readers' contract; §6.2 AC-LD15 is the guard).
- No new check ID, no new `FeatureState`, no new `ReconcileLabel`, no new
  persisted field, no new artifact, no `.tpatch/` schema migration.
- **No `freshness_label` in the verify report** (Q16).
- No change to forward-mode V7/V8/V10 semantics for features with evidence
  `none` — **except** the `active` closure widening (AC-L82/AC-L83), which is
  a deliberate, separately-pinned behaviour change.
- No fix for forward-mode V10's live-tree reference (Q15) — scoped out with
  justification.
- No provider calls; verify stays offline.
- No auto-healing: verify never invokes `record`, `land` or `apply`.
- No `--all` reordering (Q12).
- No `land`-side trailer-block hook warning (Q13).


---

## 8. Risks and mitigations

| Risk | Mitigation |
|------|------------|
| Operator confusion: "I ran verify but the lifecycle state didn't change." | §1.3 + skill bullet explain that verify writes a freshness record, not a lifecycle transition. CHANGELOG explicit. |
| Closure-replay cost on deep DAGs. | Bounded by DAG depth × per-recipe replay cost. For typical 1–3-deep DAGs the cost is well within the cheap-budget. Operators with deeper DAGs verify parents first; harness reads `verified-fresh` and skips redundant reverify. |
| Closure-replay drift: a parent's `apply-recipe.json` has changed since the parent was last applied locally. | Verify replays the parent's *current* `apply-recipe.json` regardless. If the parent itself is `verified-stale` due to recipe drift, the operator should `tpatch verify <parent>` first; the failing-parent message names the parent explicitly. |
| Freshness record getting out of date silently. | The four-label derivation is read-time; staleness is visible in `tpatch status` immediately. No silent rot. |
| Closure-replay shadow contention with reconcile. | §3.4.5: per-slug lock; verify refused while reconcile in flight on the same slug. |
| Shadow leak on crash. | `defer PruneShadow(...)` at verify entry; `PruneAllShadows` reaps stale shadows from prior crashed runs. |
| `Verify` field break on downstream JSON consumers that hard-code v0.6.1 schema. | `omitempty` means v0.6.1 round-trip is byte-identical until first verify. After first verify, the schema gains one new top-level field; downstream consumers need to handle the omitempty case. CHANGELOG callout. |
| **`active` closure widening (§3.6.6) changes verdicts for non-landed features.** A hard parent in `active` fail-fasts today and will be replayed or skipped after Wave C. | The widening makes four call sites agree that already disagree (`dependency_gate.go:79-81`, `verify.go:127-134`, `verify_all.go:88-96` vs the closure switch); today's behaviour is an oversight, not a decision. Pinned by AC-L73/AC-L74, called out in the CHANGELOG at Wave C, and reversible by narrowing the switch. |
| **The mandatory `(0/0)` block produces measured false reds** — 26 over 151 postimage-present trees, each an unrelated edit within one line of a hunk. | Deliberate: the alternative leaked 2 false greens over 69 absent trees. Every false red carries R2, a real remediation (re-record so the captured context matches HEAD, then re-land). Q14 records the decision and the rejected alternative. |
| **`historical-anchor-unavailable` is terminal**, so a feature whose landing history admits no single-parent candidate with a non-materializing parent fails until re-landed. | It is the only honest outcome: an unverifiable historical half is an unverified feature. R11 names the exact fix, and AC-L37 pins that a re-land recovers the anchor. |
| **Anchor C depends on `git apply --cached` and a temp index.** | Measured read-only: real index byte-identical, worktree byte-identical, `git status` unchanged, temp index invisible and removed on every exit path (AC-L7–AC-L13). |
| **Anchor H is unavailable for root/merge landings and for histories where every landing's parent already contains the feature.** | Reported explicitly as `historical_anchor.state = "unavailable"` with a reason; anchor C still runs at block severity, so no verdict is silently weakened. AC-L12 pins it. |
| **Verify now fails on git below 2.25 instead of silently forward-verifying.** | Deliberate: `unavailable` is a block because degrading to `none` converts an unknown into a positive claim. The floor is documented and the diagnostic (R9) names it. |
| **V10 remains wrong for un-landed applied features with a real `preimage_hash`.** | Pre-existing and measured (Q15); out of this amendment's scope because there is no anchor for an un-landed feature. Recorded rather than implied fixed. |
| Apply gate ignoring freshness misread as "verify is pointless." | The PRD §1.2 + skill bullet emphasise that freshness is a harness signal, not a gate. The harness pattern (`verify parent && apply child`) keeps working. |

---

## 9. Implementation slices (downstream Wave 3 dispatches)

The dispatch contract names four slices. Boundaries below reflect F3's correction of Slice A scope.

### Slice A — Verify command shell + V0–V2 cheap structural checks + freshness writer skeleton

- New file `internal/cli/verify.go` (registered under `cmd/tpatch/main.go`).
- New file `internal/workflow/verify.go` with `RunVerify(s *store.Store, slug string, opts VerifyOptions) (*VerifyReport, error)` and the V0–V2 check functions.
- Cobra wiring: `<slug>` arg, `--json`, `--quiet`, `--no-write`, `--path`. **No `--all`, no `--shadow`.**
- New `VerifyRecord` and `VerifyCheckResult` structs in `internal/store/types.go` with `omitempty` JSON tags.
- New `WriteVerifyRecord` (or equivalent) helper in `internal/store/store.go` that updates `FeatureStatus.Verify` and persists.
- V3–V9 stubbed with `TODO` and a sentinel result that marks them `passed=true, severity=warn` so the report still emits a 10-entry array.
- **No closure replay.** V7/V8 stubs return immediately.
- **No skill anchor regen.** Slice D handles all skill surface changes.
- **No `ComposeLabels` extension.** Slice B integrates the freshness derivation.
- Tests: V0–V2 unit tests; `--json` shape golden test for the green/fail rows V0–V2 produce; `--no-write` honoured.

### Slice B — Freshness derivation + label integration

- `ReconcileLabel` vocabulary gains `LabelNeverVerified`, `LabelVerifiedFresh`, `LabelVerifiedStale`, `LabelVerifyFailed`.
- `composeLabelsFromStatus` (`internal/workflow/labels.go:143`) extended to derive the four labels per §3.4.2. Pure function; no writes.
- `tpatch status` and `tpatch status --dag` render the freshness label inline.
- `tpatch status --json` emits the `Verify` sub-record + the derived label.
- `amend (recipe-touching)` invalidates `Verify.Passed`; `amend (intent-only)` preserves.
- `amend --state tested` rejected with `exit 2 — no such state`.
- Tests: derivation truth-table per §3.4.2; v0.6.1 round-trip byte-identity (no `Verify` set); apply-gate test pinning that freshness does NOT extend the satisfaction set.

### Slice C — V3–V9 implementation incl. closure replay

- V3 implementation reusing M14.2 `created_by` semantics.
- V4 calling `store.ValidateDependencies`.
- V5 calling `gitutil.IsAncestor` per the v0.6.1 satisfied_by contract.
- V6 calling `workflow.CheckDependencyGate` with severity reduced to warn.
- V7 + V8: hard-parent topological closure replay per §3.4.3, target recipe apply, target patch `--check`. Single shadow allocation. `failed_at: "parent-replay"` JSON field on first parent failure.
- V9 reading `status.Reconcile.Outcome` (adversarial test pins no artifact reads).
- Tests: per-check unit tests; closure replay test fixtures (3-deep DAG; parent-fail mid-closure; `upstream_merged` parent skipped); concurrency-with-reconcile refusal; source-truth adversarial test.

### Slice D — `--all`, skill bullets, harness anchors, parity guard, CHANGELOG

- `tpatch verify --all` topo-ordered aggregate report; pre-apply slugs skipped per Q2.
- All 6 skill surfaces gain the §4.4 bullet.
- `assets/assets_test.go` parity guard extended with the new anchors.
- `docs/dependencies.md` cross-link to verify (one-paragraph aside near the apply-time gate section).
- `CHANGELOG.md` v0.6.2 entry naming the verb, the freshness overlay, and the explicit out-of-scope list.
- Tests: parity-guard green; `verify --all` topo-order test; pre-apply skip test.

Each slice is independently dispatchable and shippable.

---

## 10. Cross-cutting impact matrix

| Other feature / surface | Relationship | Notes |
|-------------------------|--------------|-------|
| `feat-feature-dependencies` (M14, shipped) | **independent** | `CheckDependencyGate` unchanged. M14.3 labels unchanged. Freshness labels compose orthogonally via the existing `ComposeLabels` plumbing. |
| `feat-provider-conflict-resolver` (M12, shipped) | **independent** | Verify never calls the resolver. Per-slug shadow lock prevents collision. |
| `tpatch amend` (M13, shipped) | **extends** | Recipe-touching amend invalidates the freshness record. No new flag. |
| `tpatch test` (existing command) | **independent** | Distinct verb; not a producer of the freshness record. |
| `tpatch reconcile` | **independent at the lifecycle level**; **invalidates freshness** | Reconcile rewriting `apply-recipe.json` or `post-apply.patch` causes `recipe_hash_at_verify` / `patch_hash_at_verify` drift; next `ComposeLabels` derives `verified-stale`. |
| `tpatch status` / `--dag` / `--json` | **extends** | Renders freshness label and emits `Verify` sub-record. |
| `assets/assets_test.go` | **extends** | New skill anchor for the verify/freshness bullet. |
| `docs/dependencies.md` | **extends** (one paragraph) | Cross-links verify in the apply-time-gate section. |
| `feat-reconcile-code-presence-verdicts` | **out of scope** | Distinct PRD; reuses some primitives. |
| `feat-reconcile-fresh-branch-mode` | **out of scope** | Distinct PRD. |
| `delete-file` recipe op | **out of scope** | Recipe-op JSON schema frozen. Verify tolerates deletions in shadow replay. |
| `feat-tested-state-test-producer` (future) | **enables** | If `tpatch test` ever joins as a producer of the freshness record, ADR-013 D3 names it as the future-work expansion. |

---

**End of PRD.** Implementation handoff for Slice A will live in `docs/handoff/CURRENT.md` once this PRD + ADR-013 are reviewed and approved.
