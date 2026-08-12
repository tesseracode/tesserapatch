# PRD — `tpatch verify` and verification freshness overlay

**Status**: Approved (M15 Wave 3 — APPROVED WITH NOTES at commit 3c122aa; Slice A in implementation. Supersedes `docs/prds/PRD-verify-and-tested-state.md`.) · **§3.6 / §4.3 golden refresh / §4.3.6–4.3.9 / §5 landed rows / §6 Q9–Q15 / §7.1 amended 2026-08-12 for v0.15.1 Wave B (GH #8), **rev-1** — AWAITING REVIEW**
**Date**: 2026-04-27 (original) · 2026-08-12 (landed-feature amendment)
**ADR**: **ADR-013-verify-freshness-overlay.md — REQUIRED before Wave 3 implementation slices ship.** ADR-013 supersedes ADR-012 in full. **ADR-013 Amendment 1 rev-1 (D8–D16) is the binding ADR for §3.6 and §7.1.**
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

> **Landed-feature amendment (2026-08-12, v0.15.1 Wave B / GH #8, rev-1).**
> After `tpatch land` commits a feature into reachable Git history, the
> HEAD-anchored V7/V8 baseline already contains it and forward-apply semantics
> stop describing the world: `write-file` recipes pass vacuously,
> `replace-in-file` recipes false-fail, `append-file` recipes pass while
> corrupting the shadow, and V8 always fails. **§3.6** defines the landed
> contract — a strict trailer-grammar evidence reader over one raw+parsed
> enumeration, **dual-anchor** verification (historical replay at the landing
> commit's single parent, plus a current-HEAD materialization ladder), closure
> arbitration that replays a member iff its content is absent from the anchor,
> and anchor-anchored V10 semantics consistent with ADR-029. **§7.1** is its
> 106-row executable acceptance matrix. Binding ADR: ADR-013 Amendment 1 rev-1
> (D8–D16).
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
### 3.6 Landed-feature verification contract — v0.15.1 Wave B / GH #8 (rev-1)

> **Amendment status**: proposed rev-1, 2026-08-12, AWAITING REVIEW. Binding
> ADR: **ADR-013 Amendment 1 rev-1, decisions D8–D16.** Implementation is
> Wave C. Issue: <https://github.com/tesseracode/tesserapatch/issues/8>.
> Co-amended: `docs/prds/PRD-tpatch-land.md` §3.8.
>
> **rev-0 → rev-1.** rev-0 was returned NEEDS REVISION by both reviewers for
> two structural errors: it omitted **V10** (`write_file_preimage_fresh`) and
> therefore described a check set that does not exist, and it judged a landed
> feature only at current `HEAD` with byte-exact post-state predicates, which
> false-reds on any later unrelated edit and leaves V7 with no role
> independent of V8. rev-1 replaces the baseline model (§3.6.4), rebuilds the
> evidence reader (§3.6.2), defines V10 (§3.6.6) and makes every state total.
> Every behavioural claim below was measured; the probe index is ADR-013
> §A1.1 E1–E22.

#### 3.6.1 The defect this section closes

`tpatch land` (v0.8.0) commits a feature into reachable Git history while
deliberately leaving `status.apply.base_commit` untouched (§3.6 of
`PRD-tpatch-land`; `internal/cli/land.go:394`; ADR-019). V7/V8 allocate their
shadow from **current HEAD** (`internal/workflow/verify.go:1012`, `:1024`), so
after `land` the baseline already contains the feature and the forward-apply
semantics of §3.4.3 no longer describe the world.

Measured on `main` at `13a885c` with the real CLI. Every run reported
`checks=11` — the shipped set is **V0–V10**
(`internal/workflow/verify.go:49-71`), V10 appended last in `RunVerify`
(`internal/workflow/verify.go:288-289`):

| Target recipe op kind | pre-land | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file`      | ✓ / ✓ | **✓ false green** — `os.WriteFile` overwrites unconditionally (`internal/workflow/verify.go:1290-1294`) | **✗ false red** |
| `replace-in-file` | ✓ / ✓ | **✗ false red** — `search text not found` (`internal/workflow/verify.go:1295-1305`) | skipped |
| `append-file`     | ✓ / ✓ | **✓ false green, shadow silently double-appended** (`internal/workflow/verify.go:1306-1313`) | **✗ false red** |

The reporter saw `V7 ✓ / V8 ✗` because their recipe was `write-file` shaped.
**The defect is not V8-only**, and the same double-apply hazard applies to a
landed hard **parent** (`internal/workflow/verify.go:1048-1091`).

#### 3.6.2 Landing evidence — one enumeration, raw and parsed, strict grammar (ADR-013 D10)

A feature is **landed** iff a commit reachable from current `HEAD` carries a
well-formed `tpatch land` trailer block naming it, whose recorded values still
describe its current canonical artifacts.

**Enumeration — exactly one `git log` per run.** Over commits reachable from
the resolved `HEAD`, in `--topo-order`, emitting per commit `%H`, `%P`, the
**raw message** `%B`, and `%(trailers:key=…,valueonly,separator=…)` for the
four ADR-019 keys, NUL-delimited. The result is **cached and reused for every
feature** of a `verify --all` run. Never `--first-parent`: a landing merged
from a side branch is reachable only through a non-first parent.

**Raw retention is mandatory.** `git log --grep '^Tpatch-Feature: <slug>$'`
also matches a commit whose *prose body* quotes that line, and a `--amend`
that appends a paragraph after the trailer block makes Git parse **no**
trailers while the raw body still contains the line. Parsed-only output
therefore cannot distinguish "no attestation" from "destroyed attestation".
`--grep` is permitted as a cost prefilter only if the raw body is retained for
every commit whose body contains a `Tpatch-Feature:` line.

**Grammar (normative).**

| Element | Rule |
|---|---|
| Key case | Git matches trailer keys **case-insensitively** (`tpatch-feature:` is returned by `%(trailers:key=Tpatch-Feature…)`). The reader inherits this; the contract states it rather than claiming a match it cannot implement. |
| `Tpatch-Feature` cardinality | **Exactly one value.** `land` emits exactly one (`internal/cli/land.go:397-400`) and ADR-019 admits no other producer. Zero parsed values with a slug-bearing raw body ⇒ `malformed`. Two or more ⇒ `malformed`: the sibling SHA trailers cannot be attributed to a specific slug. |
| `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` / `Tpatch-Base-Commit` cardinality | **Exactly one each.** Zero or ≥2 ⇒ `malformed`. Duplicates are observable (`aaaa,bbbb`) and a "take the first" parser would silently pick a convenient one; that is forbidden. |
| Slug match | Exact string equality after trimming leading/trailing ASCII space and tab. Never prefix (`my-slug` ≠ `my-slug-extended`), never substring. |
| `Tpatch-Patch-SHA` format | 64 **lowercase** hex; else `malformed`. |
| `Tpatch-Recipe-SHA` format | 64 lowercase hex **or** the literal `none`; else `malformed`. |
| `Tpatch-Base-Commit` format | 40 lowercase hex; else `malformed`. |
| Reader failure | Git error, unparsable output, or a git below the §3.6.9 floor ⇒ evidence **`unavailable`**, a block failure. It **never** degrades to `none`. |

Lowercase strictness follows the ADR-029 D1 precedent already enforced by
`isLowercaseHex` (`internal/workflow/writefile_safety.go:176`).

**Value validation** against the run's artifact snapshot (§3.6.7):

| Trailer | Compared with |
|---|---|
| `Tpatch-Patch-SHA` | `sha256` of the snapshot's `post-apply.patch` bytes. **Presence-aware**: artifact *absent* ⇒ no digest exists and any attested value mismatches; artifact *present and zero-byte* ⇒ the digest is `sha256("")` and must compare equal. **Absent ≠ empty.** |
| `Tpatch-Recipe-SHA` | `sha256` of the snapshot's `apply-recipe.json` bytes, **except** that an absent artifact **or** a whitespace-only one expects the literal `none` — mirroring `readRecipeSHA` (`internal/cli/land.go:1034-1043`), which returns `none` on a read error and on `strings.TrimSpace(...) == ""`. |
| `Tpatch-Base-Commit` | `status.apply.base_commit` (`internal/store/types.go:347`) — **not** the landing commit's parent, because rebase and cherry-pick rewrite the parent while copying the trailer verbatim. Unreachability of the recorded base (`gitutil.IsAncestor`, `internal/gitutil/gitutil.go:828`) is the advisory `base_commit_reachable: false` and never fails on its own. |

**Evidence states — closed set of eight, total:**

| Candidate population | State | Effect |
|---|---|---|
| no candidate and no raw-body-only match | `none` | forward mode — today's behavior, unchanged |
| exactly one well-formed candidate with all three values matching | `exact` | landed mode |
| ≥2 such candidates, byte-equivalent per §3.6.8 | `duplicate-equivalent` | landed mode; `duplicates: n` |
| ≥2 such candidates, not byte-equivalent or not comparable | `ambiguous` | **FAIL** `failed_at: "landing-evidence"` |
| 0 all-match, ≥1 well-formed-but-mismatched | `stale` | **FAIL** `failed_at: "landing-evidence"` |
| cardinality / format / raw-only failures | `malformed` | **FAIL** `failed_at: "landing-evidence"` |
| candidates exist but none has exactly one parent | `unsupported-topology` | **FAIL** `failed_at: "landing-evidence"` |
| reader error | `unavailable` | **FAIL** `failed_at: "landing-evidence"` |

**Reverse-apply is never ownership proof.** `git apply --check --reverse`
succeeds against *any* tree containing equivalent content, including one
produced by an unrelated actor. §3.6.5 uses it as a *materialization* signal
only, and only behind an evidence commit whose `Tpatch-Patch-SHA` equals the
digest of the very bytes being reverse-checked.

#### 3.6.3 What each check independently proves in landed mode

| Check | Anchor | Independent obligation |
|---|---|---|
| **V7** `recipe_replay_clean` | historical (`L^`) | the recipe still **forward-applies** to the tree it was authored against, after closure arbitration. Not derivable from V8. |
| **V8** `post_apply_patch_replay_clean` | historical **and** current | (a) at `L^`, `git apply --check` forward succeeds after the GH #2 reset — patch/recipe coherence, independent of V7; (b) at `HEAD`, the canonical patch is still **materialized** per the §3.6.5 ladder. Both block-severity, reported separately. |
| **V10** `write_file_preimage_fresh` | historical baseline | each `write-file` op's `preimage_hash` still describes the tree the op was recorded against, plus an ADR-029 later-touch **warn** when `HEAD` has moved on. |

rev-0 gave V7 a post-state predicate at `HEAD` that, for the `write-file`
shape every autogenerated recipe has, either false-reds on unrelated edits or
collapses into V8. The two-anchor split is what restores an independent role.

#### 3.6.4 Baseline model — dual-anchor landed verification (ADR-013 D9)

Three models were evaluated; ADR-013 D9 carries the full comparison.

| Model | Verdict |
|---|---|
| HEAD-only post-state predicates (rev-0) | **rejected** — false-reds every landed `write-file` feature after any later edit to the file; V7 aliases V8; V10's live-tree reference is unusable |
| replay at `status.apply.base_commit` | **rejected** — operator-owned and drifting, may be unreachable, unrelated to the attestation |
| **dual anchor** | **chosen** |

- **Anchor H (historical)** — shadow rooted at `L^`, the **single** parent of
  the selected landing commit. Closure arbitration (§3.6.6) runs, then the
  *existing, unmodified* machinery: V7 replays the target's recipe, the shadow
  is reset to `closureBaselineTree`, V8 runs `git apply --check` forward,
  V10 evaluates preimages against the closure baseline.
- **Anchor C (current)** — at `HEAD`, read-only, no shadow: the canonical
  patch must still be materialized per the §3.6.5 ladder.

**Implementation delta**: the commit-ish handed to `gitutil.CreateShadow`
(`internal/gitutil/shadow.go:56`, which already accepts any commit-ish)
becomes `L^` instead of `gitutil.HeadCommit`
(`internal/workflow/verify.go:1012`, `:1024`), plus one or two
`git apply --check --reverse` calls.

**GH #2 (v0.11.3) invariant, binding in every mode.** The recipe and the patch
are validated **independently against the same baseline tree**, with the
shadow reset to `closureBaselineTree` between them
(`internal/workflow/verify.go:1092`, `:1143`). Normative restatement: *any
check that may mutate the shadow MUST reset it to `closureBaselineTree`
before the next check runs; V7's result is never an input to V8's tree.*
Anchor C allocates no shadow and is read-only, so it cannot disturb this.

**Non-landed features are untouched**: evidence `none` ⇒ shadow at `HEAD`,
V7/V8/V10 byte-for-byte as today.

**Anchor H unavailable** (no candidate qualifies under §3.6.8) ⇒ V7 and V8's
historical half are **skipped** with reason
`skipped: no single-parent landing anchor available`, the report carries
`historical_anchor: {"state": "unavailable", …}`, and **anchor C still runs at
block severity**. Explicit degradation with a named cause — not a fallback to
pass.

#### 3.6.5 Anchor C — the materialization ladder, stated accurately (ADR-013 D11)

**What reverse-apply actually proves.** `git apply --check --reverse` asserts
that the patch's **postimage hunks are present**, matched by content with a
line-offset search and a configurable context requirement. It is **not** a
byte-exact tree comparison and **not** ownership evidence. rev-0's
"byte-exact" language was wrong.

**The ladder.**

1. `git apply --check --reverse <patch>` at default context. Success ⇒
   **materialized, clean**; V8's anchor-C half passes, no advisory.
2. On failure only: `git apply --check --reverse -C0 <patch>`. Success ⇒
   **materialized with context drift**; V8's anchor-C half passes at block
   severity **and** a `warn`-severity advisory names the affected paths and
   tells the operator to inspect. Failure ⇒ **FAIL** `landed-content-absent`.

**Why the ladder.** Measured on a 3-hunk patch in a 60-line file: an offset
shift (10 lines prepended or appended) and an unrelated edit far from a hunk
pass at every context level; an unrelated edit **2 lines** from a hunk fails
at default context but passes at `-C0`; partial revert, full revert, further
modification of the feature line, and deletion of a patched file fail at every
level. Over 400 randomized trees, `-C0` gave **0 false greens in 216
postimage-absent trees** and **0 false reds in 184 postimage-present trees**,
while default context gave **60 false reds / 184**.

**Measured limitation, recorded not hidden.** `-C0` can succeed when the
feature was reverted *in place* and the identical postimage text exists
verbatim elsewhere in the same file. Three things bound it: it requires a
deliberate revert-plus-paste of identical text; it surfaces at step 2, i.e.
as a `warn` advisory, never a silent clean pass; and V7 at anchor H is an
independent corroboration that this shape does not produce. **Hardening
(SHOULD, Wave C)**: run step 2 with `--verbose` under a pinned `LC_ALL=C` and
treat `Context reduced to (0/0)` on *every* hunk as not-materialized —
measured to fire exactly on this shape. It is a SHOULD because it parses
human-readable git output; the residual is recorded in §6 Q14.

#### 3.6.6 Closure arbitration, total materialization, and V10 (ADR-013 D12, D13, D14)

**Governing invariant.** A closure member is replayed **iff** its content is
not already present on the anchor being built.

| Member condition | Action |
|---|---|
| `upstream_merged` | **skip** (unchanged, `internal/workflow/verify.go:1062-1064`) |
| superseded by an active superseder | **skip** (unchanged, `internal/workflow/verify.go:976-983`) |
| `applied` / `active`, evidence `exact`/`duplicate-equivalent`, **total materialization holds** at the anchor | **skip** |
| `applied` / `active`, evidence `exact`/`duplicate-equivalent`, materialization does **not** hold | **fail-fast** `parent-landing-drift` |
| `applied` / `active`, evidence `none`, **content already present** at the anchor | **skip**, with a mandatory `warn` `unattributed-materialized` advisory naming the member. Verify explicitly claims **no ownership** of that content. |
| `applied` / `active`, evidence `none`, content absent | **replay** (unchanged, `internal/workflow/verify.go:1065-1082`) |
| `applied` / `active`, evidence `stale` / `ambiguous` / `malformed` / `unsupported-topology` / `unavailable` | **fail-fast** `parent-evidence-integrity` |
| `unapplied` | **fail-fast** `parent-unapplied` |
| `rejected` | **fail-fast** `parent-rejected` |
| any other state | **fail-fast** (unchanged `default:`) |

The `evidence none` + already-present row is a rev-1 correction: rev-0
replayed such members unconditionally, which re-creates the double-apply
defect for exactly the `append-file` shape GH #8 is about.

**`active` is decided, not deferred.** `active` is treated **identically to
`applied`** throughout the closure. Today it is not: the switch handles only
`upstream_merged` and `applied`, so an `active` hard parent reaches `default:`
and fail-fasts (`internal/workflow/verify.go:1061-1089`) — while
`CheckDependencyGate` accepts `applied` **and** `active`
(`internal/workflow/dependency_gate.go:79-81`), `postApplyVerifyStates`
admits `active` (`internal/workflow/verify.go:127-134`) and `verify --all`
does too (`internal/workflow/verify_all.go:89-97`). Widening the switch is the
smallest change that makes all four call sites agree. **This is a deliberate
behaviour change for non-landed features too**; it carries its own acceptance
rows (AC-L57, AC-L58) and its own risk row in §8.

**Total materialization** means **every applicable assertion passes**, never
the cheapest one:

| Artifacts on the member | Requirement |
|---|---|
| recipe present with ≥1 op **and** patch present | **both** the recipe replay (V7-equivalent) **and** the §3.6.5 patch ladder |
| recipe absent, zero-operation, or whitespace-only | the **canonical patch** ladder is required and is the sole authority |
| patch absent, recipe present with ≥1 op | the recipe replay is required and is the sole authority |
| **both absent** | materialization is **not provable** ⇒ the member **fails** `landed-artifacts-absent`. Never treated as materialized. |

rev-0 let a landed parent be skipped on a recipe check alone; rev-1 requires
the conjunction, which is what "no double application" needs to be safe.

**Post-state predicates** are needed only where a replay is unavailable — the
"content already present at the anchor" arbitration and the recipe-only
materialization case:

| Op | Materialized iff |
|---|---|
| `write-file` | the file exists and its bytes equal `op.Content` |
| `append-file` | the file exists and its content **ends with** `op.Content`; an **empty** `op.Content` is **undecidable** (it attests nothing) rather than a vacuous pass |
| `replace-in-file` | the **existential inverse** holds — see below |
| `ensure-directory` | the path exists and is a directory |
| unknown type | unsupported (unchanged, `internal/workflow/verify.go:1316`) |

**The `replace-in-file` existential inverse.** For content `c`, search `S`,
replacement `R`:

- `S == ""` ⇒ **unsupported**. `strings.Replace(x,"",R,1)` inserts at the
  start, so such an operation is malformed.
- `R == ""` ⇒ **undecidable**. Every `c` admits the preimage `S+c`, so the
  predicate attests nothing; the judgement **defers to patch authority**
  (anchor C).
- otherwise ⇒ **true iff there exists an index `i` at which `R` occurs in `c`
  such that `pre := c[:i] + S + c[i+len(R):]` satisfies
  `strings.Replace(pre, S, R, 1) == c`.** Every occurrence of `R` is tried.

Verified by exhaustive enumeration (alphabet `{a,b,X}`, preimages ≤ 7 chars,
contents ≤ 5, all 1–2-char `S`/`R` plus `R == ""`):

| Predicate | decided | undecidable | false reds | false greens |
|---|---|---|---|---|
| rev-0 `Replace(Replace(c,R,S,1),S,R,1)==c` | 56 784 | 0 | **204** | **15 933** |
| rev-1 existential inverse | 52 416 | 4 368 | **0** | **0** |

**V10 (`write_file_preimage_fresh`) in landed mode.**
`checkWriteFilePreimage` reads the target from `repoRoot` — the **live working
tree** (`internal/workflow/writefile_safety.go:108-112`). For an **applied**
feature the live tree holds the *post*-image, so a genuine `preimage_hash`
never matches, and an empty preimage collides with the now-existing file. Both
were measured. Autogenerated recipes escape only because `RecipeFromPatch`
emits `{type,path,content}` with **no** `preimage_hash`
(`internal/workflow/recipe_autogen.go:114-118`), taking the ADR-029 D4 legacy
path.

In landed mode V10's reference tree is the **anchor-H closure baseline** — the
shadow after closure arbitration and *before* the target's recipe replays.
That tree is, by construction, the preimage the field describes.

| Case | Outcome |
|---|---|
| `preimage_hash` absent | legacy pass, no re-warn (unchanged, ADR-029 D4, `internal/workflow/verify.go:879-883`) |
| present and matching at the anchor-H baseline | **PASS**, `mode: "historical-anchor"` |
| present and **not** matching at the anchor-H baseline | **FAIL** at block severity — the recipe is genuinely stale or destructive relative to its own baseline. Downgraded to `warn` when superseded (unchanged, ADR-029 D7, `internal/workflow/verify.go:862-870`). |
| the path's content at current `HEAD` differs from the landing's postimage | **`warn`-severity later-touch advisory** on the V10 row, per ADR-029 D5/D6. **Never a block on its own.** |
| anchor H unavailable | **skip**, reason `skipped: landed V10 requires a historical anchor`, plus the later-touch advisory when computable. Never falls back to the live tree. |
| V2 skipped or failed | **skip**, unchanged reason (`internal/workflow/verify.go:853-861`) |

This is the ADR-029 policy the contract must honour: **no automatic block
merely because a landed `write-file` no longer matches its preimage** — that
is now a warn-class later-touch signal — and **no false green for an actually
stale or destructive recipe** — a preimage mismatch at its own baseline still
blocks.

**Landing order is never consulted.** Classification is per-member against the
current graph; an order-dependent rule would be unstable under rebase. Closure
ordering is unchanged (`store.TopologicalOrder`, `internal/store/dag.go:107`,
driven from `internal/workflow/verify.go:998`).

**Worked mixed chains.**

- *Target unlanded, P1 landed, P2 applied-unlanded* — anchor is `HEAD`; P1 is
  already materialized there and is skipped; P2 is replayed; the target is
  forward-verified exactly as today.
- *Target landed, P1 applied-unlanded* — anchor H is `L^`; P1's content is
  absent there, so it is replayed; then V7/V8/V10 run at anchor H and the
  ladder runs at anchor C.
- *Target landed, P1 landed but reverted* — fail-fast `parent-landing-drift`
  before the target is judged.

#### 3.6.7 Immutable snapshots and read-only guarantees (ADR-013 D14)

At the start of a run verify takes **one immutable snapshot** for the target
and every closure member: the decoded `FeatureStatus`, and the **presence
flag** plus **raw bytes** of `artifacts/apply-recipe.json` and
`artifacts/post-apply.patch`. Evidence digests, V7, V8, V10, the persisted
`VerifyRecord` and the derived labels all read from that one snapshot — never
from a second read of disk. **Empty-present is distinct from absent** at every
consumer.

Before the report is finalised each snapshotted artifact is re-read and
compared; any difference ⇒ **FAIL** `failed_at: "snapshot-unstable"` naming
the mutated path. Verify never mixes bytes from two points in time.

Verify remains **read-only**: no worktree mutation, no index mutation, no
`status.json` write beyond the existing `Verify` record — and none at all
under `--no-write` (`internal/workflow/verify.go:310-314`). Evidence
enumeration is `git log` plumbing against the repo root; the anchor-C ladder
is `git apply --check`, which writes nothing. The shadow is still pruned via
the existing deferred call (`internal/workflow/verify.go:1036-1040`).

#### 3.6.8 Topology, anchor selection and duplicate-equivalence (ADR-013 D15)

**Single parent required.** A candidate is usable as the historical anchor
only if `%P` lists **exactly one** parent. A root landing has zero parents and
`git rev-parse <root>^` fails outright; a merge landing has two. Candidates
with 0 or ≥2 parents are classified **`unsupported-topology`** — never
approximated to `^1`. A merge-shaped candidate is honestly unsupported rather
than guessed at.

**Anchor selection — deterministic.** Among well-formed, all-values-match,
single-parent candidates, walk the single enumeration's `--topo-order`
**oldest-first** and select the first whose parent tree does **not** already
materialize the canonical patch (the §3.6.5 ladder run at that parent). Ties
resolve to the oldest in topo order, then to the lexicographically smallest
full SHA. If no candidate qualifies, anchor H is `unavailable` (§3.6.4).

**Duplicate-equivalence — implementable, no broadening.** Let `P` be the
canonical patch's declared path set from the strict parser
(`gitutil.FilesInPatchStrict`, `internal/gitutil/patch_paths_strict.go:253`),
sorted byte-wise. If `P` is **empty**, candidates are **not comparable** ⇒
`ambiguous`; the path set is never broadened to "all paths". Otherwise, for
each candidate `C`:

```
git diff --no-color --no-ext-diff --no-textconv --binary \
         --no-renames --unified=3 <C>^ <C> -- <P...>
```

captured as raw bytes. Candidates are `duplicate-equivalent` iff every such
byte string is identical. A candidate that is not single-parent, or whose diff
cannot be produced, makes the set `ambiguous`. The reported commit is the
selected anchor; `duplicates` is the candidate count.

**Rebase / cherry-pick / branch switch / detached HEAD / rewrite — total.**
Trailers survive rebase and cherry-pick verbatim while the SHA and parent
change, so evidence keys on trailer *values*, never on the landing SHA or its
parent identity; both classify `exact`, possibly with
`base_commit_reachable: false`. A branch switch that removes the landing from
reachability yields `none` ⇒ forward mode. A detached `HEAD` is evaluated
identically from whatever `HEAD` resolves to; the resolved commit is reported
as `baseline.commit`. A rewrite leaving no reachable landing yields `none`;
one leaving two is decided by the duplicate-equivalence rule above.

#### 3.6.9 Diagnostics, remediation and implementability (ADR-013 D13, D16)

**Remediation must never route a just-landed local feature to `reconcile`.**
The current V8 text is `post-apply.patch no longer applies to
closure-replayed baseline; run tpatch reconcile <slug>`
(`internal/workflow/verify.go:1167`). `reconcile` is the *upstream-drift*
verb: for a feature landed minutes ago there is no upstream drift, the advice
is unactionable, and reconcile may rewrite the canonical patch — destroying
the artifact the landing trailer attests. The forward-mode string is
**unchanged**; landed mode uses its own strings.

Exact strings (Wave C emits these verbatim; `<slug>`, `<n>`, `<path>`,
`<sha>`, `<state>` interpolate):

| # | Condition | Check | Exact remediation |
|---|---|---|---|
| R1 | anchor-C ladder step 2 fails | V8 | `landed feature: post-apply.patch postimage is not present at HEAD; landing commit <sha> is reachable but the content is absent — inspect with git diff <sha> HEAD, then re-record and re-land. Do NOT run tpatch reconcile: this is local drift, not upstream drift` |
| R2 | anchor-C step 1 fails, step 2 passes | V8 (warn advisory) | `landed feature: post-apply.patch content is present at HEAD but its recorded context has drifted at <path>; a later change touched the surrounding lines — inspect with git diff <sha> HEAD -- <path> and re-record if the feature should absorb it` |
| R3 | anchor-H V7 replay fails | V7 | `landed feature: recipe op #<n> failed to replay at the landing baseline <sha>: <err>; the recipe no longer describes the tree it was authored against — re-run tpatch record <slug> --regenerate-recipe and re-land` |
| R4 | anchor-H V8 forward check fails | V8 | `landed feature: post-apply.patch does not apply at the landing baseline <sha>; the patch and the landing attestation disagree — re-record and re-land` |
| R5 | evidence `stale` | V7 | `landing evidence for <slug> is stale: commit <sha> attests patch-sha=<sha> / recipe-sha=<sha> / base=<sha> but the current artifacts hash differently; re-run tpatch land <slug> to re-attest, or restore the attested artifacts` |
| R6 | evidence `ambiguous` | V7 | `landing evidence for <slug> is ambiguous: <n> reachable commits carry matching trailers with non-equivalent content (<sha>, <sha>, …); resolve the history or re-land so exactly one attestation is current` |
| R7 | evidence `malformed` | V7 | `landing evidence for <slug> is malformed: commit <sha> carries a Tpatch-Feature line that Git does not parse as a trailer, or a duplicated/ill-formed Tpatch-* value; restore the four-trailer block with git commit --amend, or re-land` |
| R8 | evidence `unsupported-topology` | V7 | `landing evidence for <slug> is unusable: commit <sha> has <n> parents and tpatch land emits single-parent commits; verify cannot derive a landing baseline from a root or merge commit — re-land <slug> on a linear commit` |
| R9 | evidence `unavailable` | V7 | `landing evidence for <slug> could not be read: <err>; verify requires git >= 2.25 for trailer enumeration and refuses to guess — upgrade git or report this environment` |
| R10 | anchor H unavailable | V7 (skip reason) | `skipped: no single-parent landing anchor available (every reachable landing commit's parent already contains this feature)` |
| R11 | V10 preimage mismatch at anchor H | V10 | `landed feature: recipe op #<n> <path> expected preimage <sha> at the landing baseline but observed <sha>; the recipe is stale against its own baseline — re-run tpatch record <slug> --regenerate-recipe and re-land` |
| R12 | V10 later-touch | V10 (warn advisory) | `later-touch: <path> has changed at HEAD since <slug> landed at <sha>; the recipe would overwrite work recorded after this feature — review before any replay (ADR-029 D5/D6, warning-class)` |
| R13 | parent landed, not materialized | V7 | `hard parent <slug> landed at <sha> but its content is not present at the verification baseline; verify <slug> first — do not re-apply it into the shadow` |
| R14 | parent evidence integrity | V7 | `hard parent <slug> has <state> landing evidence; verify <slug> first — replaying or skipping it would validate <target> against an unknown baseline` |
| R15 | parent `unapplied` | V7 | `hard parent <slug> is unapplied; its patch is deliberately absent from the tree — run tpatch apply <slug> before verifying <target>` |
| R16 | parent `rejected` | V7 | `hard parent <slug> is rejected (terminal); remove the hard dependency with tpatch amend <target> --remove-depends-on <slug>, or reopen <slug>` |
| R17 | parent `evidence none` but already present | V7 (warn advisory) | `unattributed-materialized: hard parent <slug> is not landed but its content is already present at the verification baseline; it was not replayed, and verify makes no claim about what produced it` |
| R18 | both artifacts absent on a landed member | V7 | `landed feature <slug> has neither apply-recipe.json nor post-apply.patch; materialization cannot be proven from an empty artifact set — re-run tpatch record <slug>` |
| R19 | snapshot instability | V7 | `verify aborted: <path> changed while verify was running; re-run tpatch verify <slug> with no concurrent tpatch or editor writes` |

Human report gains two lines above the check list:

```text
verify extra-button — passed
  baseline: historical-anchor @ 6316e46 (landing 54b405d) · current @ 9f2c1ab
  landing evidence: exact @ 54b405d (patch ✓ recipe ✓ base ✓)
  ✓ [block] recipe_replay_clean — replayed at landing baseline
  ✓ [block] post_apply_patch_replay_clean — coherent at baseline; materialized at HEAD
  …
  ✓ [block] write_file_preimage_fresh — preimages fresh at landing baseline
```

**Sticky `verify-failed` clearing is mode-agnostic.** No new freshness label is
added; the four §3.4.2 labels are unchanged and remain mutually exclusive. A
passing landed run persists the same `VerifyRecord` field set as a passing
forward run (`internal/store/types.go:290-296`), so the read-time derivation
takes no mode input and a feature stuck at `verify-failed` from a pre-fix
false red clears on the first passing run under this contract, with no
migration and no manual edit.

**Implementability and honest invocation accounting.** rev-0 claimed "one
`git log`-family invocation per run" while also requiring per-candidate diffs
and ancestry checks; that accounting was contradictory. The honest budget:

| Purpose | Invocations |
|---|---|
| Evidence enumeration (raw + parsed + `%P`, whole closure, all features) | **1 per run**, cached across `verify --all` |
| Shadow allocation | 1 `CreateShadow` — already allocated today; only its commit-ish changes |
| Anchor-C ladder | 1, or 2 when step 1 fails, **per landed member** |
| Duplicate-equivalence diff | 1 `git diff` per candidate, **only when ≥2 candidates** |
| `base_commit_reachable` advisory | 1 `git merge-base --is-ancestor` per landed member |
| Anchor-selection parent probe | 1 ladder run per candidate examined |

New code is one generic reader in `internal/gitutil/` (candidate
`trailers.go`) returning raw **and** parsed records; **policy** stays in
`internal/workflow/verify.go` per ADR-013 D7. Everything else reuses
`gitutil.HeadCommit`, `CreateShadow`/`PruneShadow`, `gitutil.IsAncestor`,
`store.TopologicalOrder`, `isFeatureSupersededIn`, `sha256Hex`,
`FilesInPatchStrict`, `checkWriteFilePreimage`
(`internal/workflow/writefile_safety.go:108`) and `os`/`strings`. **No new
store field, no new artifact, no schema migration, no new dependency, no new
check ID.**

**Git floor.** `%(trailers:key=…,valueonly)` needs git ≥ 2.22 and
`separator=` needs ≥ 2.25; verified on 2.55.0. Below the floor the reader
**fails** ⇒ evidence `unavailable` ⇒ block. rev-0's "degrade to `none`" was
wrong: it converts an unknown into a positive claim.

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
  "freshness_label": "verified-fresh",
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
  "lifecycle_state": "applied",
  "freshness_label": "verify-failed"
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
  "lifecycle_state": "applied",
  "freshness_label": "verify-failed"
}
```
#### 4.3.6 LANDED-PASS — dual-anchor verification green (v0.15.1 Wave B / GH #8, rev-1)

`schema_version` moves `"1.0"` → `"1.1"` (`internal/workflow/verify.go:83`).
Every new field is **additive and `omitempty` where it can be**, and the
compatibility guarantee is **additive semantic compatibility, not byte
identity**: `baseline`, `landing_evidence` and `target_mode` are emitted for
*every* feature, so a no-evidence report is **not** byte-identical to a
`"1.0"` report. Consumers refuse unknown **majors** (§4.3), so 1.1 is
non-breaking by construction. The `checks` array is **eleven** rows in
V0–V10 order in every shape below.

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verified_at": "2026-08-12T18:30:11Z",
  "verdict": "passed",
  "exit_code": 0,
  "baseline": {
    "mode": "dual-anchor",
    "commit": "9f2c1ab4…",
    "historical_anchor": { "state": "available", "commit": "6316e465…", "landing_commit": "54b405df…" }
  },
  "landing_evidence": {
    "state": "exact",
    "commit": "54b405df…",
    "candidates": 1,
    "parent_count": 1,
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
  "freshness_label": "verified-fresh",
  "recipe_hash_at_verify": "sha256:7a1b…",
  "patch_hash_at_verify": "sha256:9f24…",
  "parent_snapshot": { "button-component": "applied" }
}
```

**Field reference.**

| Field | Type | Meaning |
|---|---|---|
| `baseline.mode` | string | `"head-anchored"` (forward mode) \| `"dual-anchor"` (landed mode) |
| `baseline.commit` | string | the resolved `HEAD` — the anchor-C commit; also the shadow root in forward mode |
| `baseline.historical_anchor.state` | string | `"available"` \| `"unavailable"` \| `"not-applicable"` (forward mode) |
| `baseline.historical_anchor.commit` | string | `L^`, the shadow root in landed mode; omitted when unavailable |
| `baseline.historical_anchor.landing_commit` | string | the selected `L`; omitted when unavailable |
| `baseline.historical_anchor.reason` | string | why unavailable; omitted otherwise |
| `landing_evidence.state` | string | closed set of eight — `none` \| `exact` \| `duplicate-equivalent` \| `stale` \| `ambiguous` \| `malformed` \| `unsupported-topology` \| `unavailable` |
| `landing_evidence.commit` | string | selected landing commit; omitted when `state == "none"` or `"unavailable"` |
| `landing_evidence.candidates` | int | total candidate commits examined |
| `landing_evidence.duplicates` | int | equivalent landings; omitted when < 2 |
| `landing_evidence.parent_count` | int | `%P` cardinality of the selected candidate |
| `landing_evidence.patch_sha_match` / `recipe_sha_match` / `base_commit_match` | bool | trailer-vs-snapshot comparisons (§3.6.2) |
| `landing_evidence.base_commit_reachable` | bool | advisory only; `false` never fails on its own |
| `landing_evidence.reason` | string | reader/classification detail for the non-`exact` states |
| `target_mode` | string | `"forward"` \| `"landed"` |
| `checks[].mode` | string | `"forward"` \| `"historical-anchor"` \| `"current-anchor"` \| `"dual-anchor"` — **present on V7, V8 and V10 in every report, including when they are skipped**; **absent on V0–V6 and V9**, which have no anchor |
| `checks[].anchor_results` | object | V8 only: `{"historical": "passed"\|"failed"\|"skipped", "current": "materialized-clean"\|"materialized-context-drift"\|"absent"\|"skipped"}` |
| `advisories` | array | zero or more `{ "code", "severity": "warn", "slug", "path", "message" }` — the warn-class signals of §3.6.5 step 2, §3.6.6 later-touch and `unattributed-materialized` |

**Mode presence rule (explicit, per rev-0 finding).** V7, V8 and V10 always
carry `mode`, even when `skipped` — a skipped check must still say which
anchor it would have used. V0–V6 and V9 never carry `mode`. Consumers may
rely on both halves of that rule.

#### 4.3.7 LANDED with advisories — content present, context drifted, path later-touched

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "passed",
  "exit_code": 0,
  "baseline": {
    "mode": "dual-anchor",
    "commit": "9f2c1ab4…",
    "historical_anchor": { "state": "available", "commit": "6316e465…", "landing_commit": "54b405df…" }
  },
  "landing_evidence": { "state": "exact", "commit": "54b405df…", "candidates": 1, "parent_count": 1,
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
      "message": "later-touch: src/extras/button.css has changed at HEAD since extra-button landed at 54b405df; the recipe would overwrite work recorded after this feature — review before any replay (ADR-029 D5/D6, warning-class)" }
  ],
  "lifecycle_state": "applied",
  "freshness_label": "verified-fresh"
}
```

The `checks` array is abridged to the three anchored rows for readability; the
emitted array is always eleven rows. **The verdict is `passed`**: both
advisories are warn-class by §3.6.5 and ADR-029 D5/D6, and neither flips
`passed`.

#### 4.3.8 LANDED-CONTENT-ABSENT — evidence present, content is not

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "failed",
  "exit_code": 2,
  "failed_at": "landed-content-absent",
  "baseline": {
    "mode": "dual-anchor",
    "commit": "9f2c1ab4…",
    "historical_anchor": { "state": "available", "commit": "6316e465…", "landing_commit": "54b405df…" }
  },
  "landing_evidence": { "state": "exact", "commit": "54b405df…", "candidates": 1, "parent_count": 1,
                        "patch_sha_match": true, "recipe_sha_match": true,
                        "base_commit_match": true, "base_commit_reachable": true },
  "target_mode": "landed",
  "checks": [
    { "id": "recipe_replay_clean",           "severity": "block", "passed": true,  "mode": "historical-anchor" },
    { "id": "post_apply_patch_replay_clean", "severity": "block", "passed": false, "mode": "dual-anchor",
      "anchor_results": { "historical": "passed", "current": "absent" },
      "remediation": "landed feature: post-apply.patch postimage is not present at HEAD; landing commit 54b405df is reachable but the content is absent — inspect with git diff 54b405df HEAD, then re-record and re-land. Do NOT run tpatch reconcile: this is local drift, not upstream drift" },
    { "id": "write_file_preimage_fresh",     "severity": "block", "passed": true,  "mode": "historical-anchor" }
  ],
  "lifecycle_state": "applied",
  "freshness_label": "verify-failed"
}
```

Note that V7 **passes** here: the recipe is still coherent at the landing
baseline. That is the independent fact V7 contributes; the failure is purely
"it is no longer in the tree", which only anchor C can see.

#### 4.3.9 EVIDENCE-INTEGRITY FAIL — stale / ambiguous / malformed / unsupported-topology / unavailable

All five states share `failed_at: "landing-evidence"`. V7 carries the
diagnostic. V8 and V10 are **skipped** with
`reason: "skipped: landing evidence integrity failed"` and their `mode` field
present, so the report never implies the patch or the preimages were checked.

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "failed",
  "exit_code": 2,
  "failed_at": "landing-evidence",
  "baseline": {
    "mode": "dual-anchor",
    "commit": "9f2c1ab4…",
    "historical_anchor": { "state": "unavailable", "reason": "landing evidence integrity failed" }
  },
  "landing_evidence": {
    "state": "stale",
    "commit": "54b405df…",
    "candidates": 1,
    "parent_count": 1,
    "patch_sha_match": false,
    "recipe_sha_match": true,
    "base_commit_match": true,
    "base_commit_reachable": true,
    "reason": "Tpatch-Patch-SHA attests 201d2f3a… but artifacts/post-apply.patch hashes to 8c11ba90…"
  },
  "target_mode": "landed",
  "checks": [
    { "id": "recipe_replay_clean",           "severity": "block", "passed": false, "mode": "historical-anchor",
      "remediation": "landing evidence for extra-button is stale: commit 54b405df attests patch-sha=201d2f3a… / recipe-sha=33649b3a… / base=09a30d22… but the current artifacts hash differently; re-run tpatch land extra-button to re-attest, or restore the attested artifacts" },
    { "id": "post_apply_patch_replay_clean", "severity": "block", "passed": true, "skipped": true,
      "mode": "dual-anchor", "reason": "skipped: landing evidence integrity failed" },
    { "id": "write_file_preimage_fresh",     "severity": "block", "passed": true, "skipped": true,
      "mode": "historical-anchor", "reason": "skipped: landing evidence integrity failed" }
  ],
  "lifecycle_state": "applied",
  "freshness_label": "verify-failed"
}
```

**Unified `failed_at` vocabulary — closed set.** Wave C must not emit any
other value, and AC-L86 pins it:

| Value | Meaning |
|---|---|
| `parent-replay` | existing — a closure member failed to replay |
| `landing-evidence` | `stale` \| `ambiguous` \| `malformed` \| `unsupported-topology` \| `unavailable` |
| `landed-content-absent` | anchor-C ladder failed at both steps |
| `landed-artifacts-absent` | a landed member has neither artifact (§3.6.6) |
| `landed-baseline-incoherent` | anchor-H V7 or V8 forward check failed |
| `parent-landing-drift` | a landed closure member is not materialized at the anchor |
| `parent-evidence-integrity` | a closure member's evidence is `stale`/`ambiguous`/`malformed`/`unsupported-topology`/`unavailable` |
| `parent-unapplied` | a hard parent is `unapplied` |
| `parent-rejected` | a hard parent is `rejected` |
| `snapshot-unstable` | an artifact changed while verify was running (§3.6.7) |

**Advisory `code` vocabulary — closed set**, all `warn` severity, none of
which flips `passed`: `context-drift`, `later-touch`,
`unattributed-materialized`, `base-commit-unreachable`.

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
**Landed-feature rows (v0.15.1 Wave B / GH #8 rev-1 — see §3.6 for the contract).**

Artifact-absence rows are stated against a fact `land` itself enforces:
`land` refuses when the embedded `record` would capture nothing, so a landed
feature with an absent or empty `post-apply.patch` is a **corruption or
hand-edit** case, not a normal outcome. The rows below say so rather than
implying the state is routine.

| Case | Handling |
|------|----------|
| **Target checks** | |
| Landed target, `write-file` recipe, content present | V7 replays at anchor H → ✓. Anchor C ladder step 1 → ✓. Today V7 passes vacuously and V8 false-reds. |
| Landed target, `replace-in-file` recipe | V7 replays at anchor H → ✓. Today this false-fails with `search text not found`. |
| Landed target, `append-file` recipe | V7 replays at anchor H → ✓, and the shadow is **not** double-appended, because anchor H is the pre-landing tree. |
| Landed target, later unrelated edit **far** from any hunk | Anchor H untouched; anchor C step 1 ✓. **PASS, no advisory.** |
| Landed target, later unrelated edit **within** a hunk's context | Anchor C step 1 fails, step 2 (`-C0`) ✓ ⇒ **PASS + `context-drift` advisory (warn)**. Measured: default context alone would false-red 60/184 such trees. |
| Landed target, full revert | Anchor C fails at both steps ⇒ **FAIL `landed-content-absent`**. Reachability of the landing is never materialization. |
| Landed target, partial revert | Same — measured to fail at every context level. |
| Landed target, a patched file deleted | Same. |
| **Artifact presence** | |
| Landed, recipe present, patch absent | Evidence: any attested `Tpatch-Patch-SHA` mismatches an absent artifact ⇒ **`stale`** ⇒ FAIL. This is the corruption case; `land` cannot produce it. |
| Landed, recipe present, patch present but **zero bytes** | Digest is `sha256("")`. If the trailer attests that value, evidence is `exact` and the anchor-C ladder on an empty patch trivially succeeds; V7 at anchor H remains the block-severity authority. Absent ≠ empty. |
| Landed, recipe **absent**, patch present | `Tpatch-Recipe-SHA` must be the literal `none` (mirroring `readRecipeSHA`); V7 skips with its existing reason, and the canonical patch is the **sole** materialization authority (§3.6.6). |
| Landed, recipe present but **whitespace-only** | `readRecipeSHA` returns `none` (`internal/cli/land.go:1039-1041`), so the trailer must be `none`; V2 fails to parse ⇒ V7/V10 skip with their existing reasons; the patch is the sole authority. |
| Landed, recipe present with **zero operations** | V7 has nothing to replay and cannot attest; the canonical patch is the sole authority. The V7 row records `0 op(s)` rather than a vacuous pass. |
| Landed, **both artifacts absent** | **FAIL `landed-artifacts-absent`.** Materialization is not provable from an empty artifact set, and this is never treated as materialized. Corruption/hand-edit case. |
| **Evidence** | |
| Hand-rolled `git commit` with no trailers | Evidence `none` ⇒ forward mode ⇒ today's behavior. Attribution is never invented. |
| Trailer block destroyed by a later `--amend` (prose paragraph appended) | Raw body still carries the line, parsed value is empty ⇒ **`malformed`** ⇒ FAIL. Explicitly not `none`. |
| Duplicate `Tpatch-Patch-SHA` / `Recipe-SHA` / `Base-Commit` | **`malformed`** ⇒ FAIL. No "take the first"; duplicates are observable and ambiguous. |
| Two or more `Tpatch-Feature` values on one commit | **`malformed`** ⇒ FAIL: sibling SHA trailers cannot be attributed to a slug. |
| Uppercase hex, wrong length, or a non-`none` non-hex `Recipe-SHA` | **`malformed`** ⇒ FAIL. |
| Lowercase trailer key (`tpatch-feature:`) | Git matches keys case-insensitively; the reader inherits that and the commit **is** a candidate. |
| Root landing commit (0 parents) | **`unsupported-topology`** ⇒ FAIL. `git rev-parse <root>^` fails outright; no implicit `^`. |
| Merge commit carrying the trailer (≥2 parents) | **`unsupported-topology`** ⇒ FAIL. No `^1` approximation. |
| Landing reachable only through a merge's **non-first** parent | Found — reachability is full-graph, never `--first-parent`. |
| Two reachable landings, byte-equivalent on the patch's path set | **`duplicate-equivalent`** ⇒ landed mode, `duplicates: 2`. |
| Two reachable landings, not byte-equivalent | **`ambiguous`** ⇒ FAIL. No mode is retried. |
| Canonical patch declares **no** paths | Candidates are not comparable ⇒ **`ambiguous`**. The path set is never broadened to "all paths". |
| Cherry-picked or rebased landing | `exact` (trailers copied verbatim); `base_commit_reachable` may be `false`, which is advisory only. |
| Every reachable landing's parent already contains the feature | Anchor H **`unavailable`** — V7 and V8's historical half skip with a named reason; anchor C still runs at block severity. |
| `git` older than the §3.6.9 floor, or any reader error | Evidence **`unavailable`** ⇒ FAIL. Never `none`, never a false green. |
| **Parents** | |
| Landed hard parent, totally materialized | Skipped from the closure — never replayed, so an `append-file` parent is not duplicated. |
| Landed hard parent, recipe absent, patch materialized | Skipped: the canonical patch is the sole authority for that member. |
| Landed hard parent, **both artifacts absent** | **FAIL `landed-artifacts-absent`** — not skipped, not replayed. |
| Landed hard parent, reverted | Fail-fast `parent-landing-drift` **before** the target is judged. |
| Hard parent with `evidence none` whose content is **already present** at the anchor | **Skipped**, with a mandatory `unattributed-materialized` warn advisory. Verify claims no ownership. rev-0 replayed it, re-creating the double-apply defect. |
| Hard parent with `evidence none` and content absent | Replayed, unchanged. |
| Hard parent in `active` | Treated **exactly as `applied`** (§3.6.6). Today it fail-fasts through `default:`; this amendment widens the switch. |
| Hard parent in `unapplied` / `rejected` | Fail-fast with the named reason instead of the generic `default:` message. |
| Hard parent in `upstream_merged` / superseded | Skipped, unchanged. |
| **Run-level** | |
| `.tpatch/` artifact mutated while verify runs | **FAIL `snapshot-unstable`** naming the path. Verify never mixes bytes from two points in time. |
| `--no-write` on any landed path | All checks run, nothing persists (`internal/workflow/verify.go:310-314`). No worktree, index or `status.json` mutation on any code path. |
| `verify --all` over a mixed landed/unlanded set | One evidence enumeration total, cached and reused; per-feature output ordering unchanged. |

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

### Q14 — Open (non-blocking): should the anchor-C `-C0` step parse `git apply --verbose` output?

**Open, non-blocking, SHOULD-level.** §3.6.5 records a measured `-C0`
limitation: with the feature reverted *in place* and the identical postimage
text present verbatim elsewhere in the same file, `-C0` succeeds. Running
step 2 with `--verbose` under a pinned `LC_ALL=C` and treating
`Context reduced to (0/0)` on *every* hunk as not-materialized was measured to
fire exactly on that shape. It is left SHOULD rather than MUST because it
parses human-readable, translatable git output. Wave C may implement it; if
it does, AC-L47 covers it, and if it does not, the limitation is bounded by
three independent factors named in §3.6.5. **This does not block acceptance.**

### Q15 — Open (non-blocking): forward-mode V10 is wrong for un-landed applied features

**Open, out of scope, tracked.** Measured: with a genuine `preimage_hash`,
V10 fails for an `applied` feature that never landed
(`expected preimage sha256:5fb14…, observed sha256:fa6dd8…`), and an empty
`preimage_hash` collides with the now-existing file — because
`checkWriteFilePreimage` reads the **live working tree**
(`internal/workflow/writefile_safety.go:108-112`), which holds the
*post*-image. Autogenerated recipes escape only because `RecipeFromPatch`
omits the field (`internal/workflow/recipe_autogen.go:114-118`). §3.6.6 fixes
this **for landed features** by re-anchoring V10 at the historical baseline;
there is no anchor for an un-landed feature, and changing forward-mode V10
would alter verdicts for features that never landed. Needs its own issue and
PRD. **Does not block this amendment**; recorded so a reviewer does not read
the landed fix as a claim that V10 is now correct everywhere.


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
### 7.1 Acceptance matrix — landed-feature verification (v0.15.1 Wave B / GH #8, rev-1)

**Binding on the Wave C implementation dispatch.** Every row is a distinct,
executable acceptance criterion. **Tier** names where the row is proven:

- **U** — unit test, pure function or abstraction, no repo. Covers the §3.6.6
  predicates, the §3.6.2 grammar over fixture byte strings, the §3.6.2
  classifier over a synthetic candidate list, and anything expressed over the
  §3.6.7 **snapshot abstraction** (construct two snapshot values, assert the
  output is a pure function of them).
- **W** — workflow integration test in `internal/workflow`, real temp Git repo
  + `store.Store`, calling `RunVerify` directly. Where a row needs the
  evidence reader to observe something specific, it is proven with a **`PATH`
  git wrapper**: a test-only shim script placed first on `PATH` that forwards
  to the real `git` and can inject output, an error exit, or a mutation
  between calls. No production seam, no build tag, no exported hook.
- **C** — real-CLI test in `internal/cli`, executing the cobra surface
  end-to-end (`tpatch apply` → `record` → `land` → `verify`), asserting
  stdout/stderr/exit code.

`W+C` means the row must be proven at **both** tiers. Group letters are not
part of the identifier.

#### Group A — the reported defect and the eleven-check schema

| # | Criterion | Tier |
|---|---|---|
| AC-L1 | The issue #8 sequence — `apply --mode done` → `record` → `test` → `verify` — passes **before** `land`: V7 ✓, V8 ✓, V10 ✓, exit 0, and the report contains exactly **eleven** check rows in V0–V10 order. | C |
| AC-L2 | The same feature **after** `tpatch land` passes: exit 0, `target_mode: "landed"`, `landing_evidence.state: "exact"`, `baseline.mode: "dual-anchor"`, still eleven rows. | W+C |
| AC-L3 | The issue's committed-range re-record (`record --from <base> --to HEAD --files … --regenerate-recipe`) is decided by the §3.6.2 hashes, and **both branches are asserted**: (a) byte-identical regenerated artifacts ⇒ evidence stays `exact`, verify passes with no re-land; (b) changed artifacts ⇒ evidence `stale`, verify FAILs with the R5 string naming `tpatch land <slug>`, and passes after that re-land. Branch (b) is the reporter's actual path. | C |
| AC-L4 | A landed **leaf** with no dependencies passes. Pins that the fix is not closure-dependent. | W+C |
| AC-L5 | Every report — forward or landed, pass or fail — emits exactly eleven `checks` rows whose `id` values equal the eleven constants at `internal/workflow/verify.go:50-71`, in that order. Golden assertion. | W |
| AC-L6 | `tpatch verify --no-write` on every AC-L row leaves `.tpatch/`, the index and the worktree byte-identical, asserted by hashing the whole `.tpatch/` tree plus `git status --porcelain -z` and `git ls-files --cached -z` before and after. | W+C |

#### Group B — anchor H: historical replay and recipe op shapes

| # | Criterion | Tier |
|---|---|---|
| AC-L7 | Landed target, `write-file` recipe: V7 **replays** at anchor H and passes; the shadow root is `L^`, not `HEAD`. Asserted on the commit-ish passed to `CreateShadow`. | W |
| AC-L8 | Landed target, `replace-in-file` recipe: V7 passes. **This is the case that false-fails today with `search text not found`.** | W+C |
| AC-L9 | Landed target, `append-file` recipe: V7 passes and the shadow contains the payload exactly **once**. Asserted on file content, not just verdict. | W+C |
| AC-L10 | Landed target, recipe that genuinely no longer applies at `L^` (hand-edited op): V7 FAILs with R3, `failed_at: "landed-baseline-incoherent"`. | W |
| AC-L11 | Anchor H is the selected landing commit's **single** parent; a candidate with 0 or ≥2 parents never contributes an anchor. | U+W |
| AC-L12 | When no candidate qualifies, `historical_anchor.state == "unavailable"` with a reason, V7 and V8's historical half **skip** with R10, and anchor C still runs at **block** severity. Explicit degradation, not a pass. | W |
| AC-L13 | `write-file` post-state predicate (used only where replay is unavailable): bytes equal `op.Content` ⇒ true; one byte differs ⇒ false. | U |
| AC-L14 | `append-file` post-state predicate: strict suffix ⇒ true; payload present but not at the end ⇒ false; **empty `op.Content` ⇒ undecidable**, never a vacuous pass. | U |
| AC-L15 | `replace-in-file` existential inverse: the exhaustive corpus of §3.6.6 (alphabet `{a,b,X}`, preimages ≤ 7, contents ≤ 5, all 1–2-char `S`/`R`) yields **0 false reds and 0 false greens** over the decided cases. Includes the specific rev-0 counterexamples `c='abb', S='aa', R='b'` (must be true) and `c='b', S='a', R='a'` (must be false). | U |
| AC-L16 | `replace-in-file` with **repeated replacement text** (`R` occurring several times in `c`) is decided by trying **every** occurrence, not the first. | U |
| AC-L17 | `replace-in-file` with `R == ""` ⇒ **undecidable**, judgement defers to anchor C; with `S == ""` ⇒ **unsupported op**. Neither is a pass and neither is a block on its own. | U |
| AC-L18 | `ensure-directory` predicate: directory present ⇒ true, absent ⇒ false. Unknown op type ⇒ unsupported, message unchanged from `internal/workflow/verify.go:1316`. | U |

#### Group C — anchor C: the materialization ladder

| # | Criterion | Tier |
|---|---|---|
| AC-L19 | Landed target, pristine: ladder step 1 passes ⇒ `anchor_results.current == "materialized-clean"`, no advisory. | W+C |
| AC-L20 | Later unrelated edit **far** from any hunk, and pure offset shifts (lines prepended and appended): step 1 still passes. **PASS, no advisory.** | W |
| AC-L21 | Later unrelated edit **within** a hunk's default context window: step 1 fails, step 2 (`-C0`) passes ⇒ verdict **passed**, `anchor_results.current == "materialized-context-drift"`, and a `context-drift` warn advisory with the R2 string. This is the rev-0 false red. | W+C |
| AC-L22 | **Full revert** of the landing: both ladder steps fail ⇒ **FAIL `landed-content-absent`** with R1. The landing commit is still reachable — reachability is never materialization. | W+C |
| AC-L23 | **Partial revert** (one hunk of a multi-hunk patch): both steps fail ⇒ FAIL. | W |
| AC-L24 | A patched file **deleted** at HEAD: both steps fail ⇒ FAIL. | W |
| AC-L25 | The feature's own changed line further modified at HEAD: both steps fail ⇒ FAIL. | W |
| AC-L26 | Anchor C is **read-only**: it allocates no shadow and runs only `git apply --check`. Asserted by hashing the worktree and index around the ladder. | W |
| AC-L27 | The R1 string contains the literal clause `Do NOT run tpatch reconcile`, and no landed-mode remediation string contains the token `reconcile` outside that negation. Adversarial. | W |
| AC-L28 | Measured `-C0` limitation is not silently green: with the feature reverted in place and identical postimage text elsewhere in the file, the outcome is step-1-fail / step-2-pass ⇒ a **`context-drift` warn advisory is emitted**, never a clean pass. If Wave C implements the Q14 `--verbose` hardening, the same fixture must instead FAIL. | W |

#### Group D — evidence reader, grammar and cardinality

| # | Criterion | Tier |
|---|---|---|
| AC-L29 | No reachable landing ⇒ `state: "none"`, `target_mode: "forward"`, shadow rooted at `HEAD`, and V7/V8/V10 verdicts **identical** to the pre-amendment implementation on the same fixture. | W+C |
| AC-L30 | All three trailer values match ⇒ `exact`. | W+C |
| AC-L31 | `Tpatch-Patch-SHA` mismatch (re-record after landing) ⇒ `stale`, FAIL, `failed_at: "landing-evidence"`, V8 **and V10** skipped with `reason: "skipped: landing evidence integrity failed"` and their `mode` present. | W+C |
| AC-L32 | `Tpatch-Recipe-SHA` mismatch (`--regenerate-recipe` after landing) ⇒ `stale`, FAIL. | W+C |
| AC-L33 | `Tpatch-Base-Commit` mismatch (a later `record` moved `status.apply.base_commit`) ⇒ `stale`, FAIL. | W |
| AC-L34 | `Tpatch-Recipe-SHA: none` with an **absent** recipe ⇒ match. With a **whitespace-only** recipe ⇒ also match, mirroring `readRecipeSHA` (`internal/cli/land.go:1039-1041`). | U+W |
| AC-L35 | Patch artifact **present and zero bytes** ⇒ the compared digest is `sha256("")`; a trailer attesting that value ⇒ `exact`. Patch artifact **absent** ⇒ any attested value mismatches ⇒ `stale`. **Absent ≠ empty** is asserted by two distinct fixtures. | U+W |
| AC-L36 | Missing any one of the four trailers ⇒ `malformed`. | U+W |
| AC-L37 | **Duplicate** `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA` or `Tpatch-Base-Commit` ⇒ `malformed`. Adversarial: the classifier must not select either duplicate. | U+W |
| AC-L38 | **Two or more** `Tpatch-Feature` values on one commit ⇒ `malformed`. | U+W |
| AC-L39 | A commit whose **prose body** quotes `Tpatch-Feature: <slug>` with no trailer block ⇒ **not** a candidate (`--grep` is a prefilter, `%(trailers:…)` is authority). | U+W |
| AC-L40 | A commit whose **raw body** carries a `Tpatch-Feature: <slug>` line that Git does **not** parse as a trailer (prose paragraph appended by `--amend`) ⇒ **`malformed`**, not `none`. Requires the reader to retain the raw message; this row fails if the reader is parsed-only. | U+W |
| AC-L41 | Slug matching is exact after trimming ASCII space/tab: `my-slug` does not match `my-slug-extended`, and a value with surrounding spaces does match. | U |
| AC-L42 | A **lowercase** trailer key (`tpatch-feature:`) **is** a candidate, because Git matches trailer keys case-insensitively. The contract states the inherited behaviour rather than claiming a case-sensitive match. | U+W |
| AC-L43 | Uppercase hex, wrong-length hex, or a non-`none` non-hex `Recipe-SHA` ⇒ `malformed`. | U |
| AC-L44 | A git invocation error, unparsable output, or a git below the §3.6.9 floor ⇒ `state: "unavailable"`, **FAIL**. It must **never** produce `none`. Proven with a `PATH` git wrapper that exits non-zero on the enumeration call. | U+W |
| AC-L45 | `base_commit_reachable: false` (base rewritten out of history) is reported, raises the `base-commit-unreachable` advisory, and **does not fail** on its own. | W |
| AC-L46 | The evidence enumeration issues **exactly one** `git log`-family invocation per run and **reuses it for every feature** of a `verify --all` run. Proven with a `PATH` git wrapper that counts `log` invocations. | W |
| AC-L47 | The §3.6.9 invocation budget is honoured: no per-candidate `git diff` unless ≥2 candidates exist, and at most two `git apply --check` calls per landed member. Counted by the same wrapper. If Wave C implements the Q14 hardening, one extra `--verbose` call per failing member is permitted and counted. | W |

#### Group E — topology, duplicates and history rewrites

| # | Criterion | Tier |
|---|---|---|
| AC-L48 | A **root** landing commit (0 parents) ⇒ `unsupported-topology`, FAIL with R8. No implicit `^`. | U+W |
| AC-L49 | A **merge** commit carrying the trailer (≥2 parents) ⇒ `unsupported-topology`, FAIL with R8. No `^1` approximation. | U+W |
| AC-L50 | A landing reachable only through a merge's **non-first** parent **is** found. Pins full-graph reachability. | W |
| AC-L51 | Two reachable landings whose `git diff <C>^ <C> -- <P…>` bytes are identical ⇒ `duplicate-equivalent`, PASS, `duplicates: 2`; the reported commit is the deterministically selected anchor. | W |
| AC-L52 | Two reachable landings whose diffs differ ⇒ `ambiguous`, FAIL with R6. No mode is retried. | W |
| AC-L53 | A canonical patch declaring **no** paths makes candidates incomparable ⇒ `ambiguous`. The path set is never broadened to "all paths". Adversarial. | U+W |
| AC-L54 | Anchor selection is deterministic: over a fixture with several qualifying candidates, repeated runs select the same commit (topo-oldest, then lexicographically smallest SHA), and a candidate whose parent already materializes the patch is skipped. | U+W |
| AC-L55 | **Cherry-picked** landing (trailers copied, SHA and parent rewritten) ⇒ `exact`, PASS. | W |
| AC-L56 | **Rebased** landing ⇒ `exact`, PASS. | W |
| AC-L57 | Branch switch away from the landing ⇒ `none` ⇒ forward mode. Branch switch away where equivalent content is present anyway ⇒ still `none`, forward mode, and the diagnostic states the content is unattributed. No pass claimed. | W |
| AC-L58 | Detached `HEAD` is evaluated identically; `baseline.commit` reports the resolved commit. | W |
| AC-L59 | History rewritten so no landing is reachable ⇒ `none` ⇒ forward mode. | W |

#### Group F — closure arbitration, parents and `active`

| # | Criterion | Tier |
|---|---|---|
| AC-L60 | A landed, totally-materialized hard parent is **skipped** — its recipe is never executed. Asserted by an op-execution counter, not by verdict alone. | W |
| AC-L61 | A landed hard parent with an `append-file` recipe is skipped and the anchor tree contains its payload exactly **once**. The double-apply regression. | W+C |
| AC-L62 | A landed hard parent with a `replace-in-file` recipe is skipped and the closure does not fail with `search text not found`. | W |
| AC-L63 | An applied-but-**unlanded** hard parent whose content is **absent** at the anchor is still replayed, byte-identically to today. | W+C |
| AC-L64 | An applied hard parent with `evidence none` whose content is **already present** at the anchor is **skipped**, and an `unattributed-materialized` warn advisory (R17) names it. Verify claims no ownership. This is the rev-0 double-application hole. | W |
| AC-L65 | **Total materialization** is a conjunction: a landed parent with a recipe **and** a patch is skipped only if the recipe replay **and** the patch ladder both hold; satisfying only one is not sufficient. | W |
| AC-L66 | Landed parent with recipe absent / zero-op / whitespace-only ⇒ the canonical patch ladder is the **sole** authority and is required. | W |
| AC-L67 | Landed parent with patch absent and a ≥1-op recipe ⇒ the recipe replay is the sole authority and is required. | W |
| AC-L68 | Landed member with **both artifacts absent** ⇒ **FAIL `landed-artifacts-absent`** (R18). Never skipped, never replayed, never treated as materialized. | W |
| AC-L69 | Landed hard parent whose landing was **reverted** ⇒ fail-fast `parent-landing-drift` (R13) **before** the target is evaluated; the target receives no passing verdict. | W |
| AC-L70 | Hard parent with `stale` / `ambiguous` / `malformed` / `unsupported-topology` / `unavailable` evidence ⇒ fail-fast `parent-evidence-integrity` (R14). Neither skipped nor replayed. | W |
| AC-L71 | Hard parent in `unapplied` ⇒ fail-fast `parent-unapplied` (R15), replacing today's generic `default:` message (`internal/workflow/verify.go:1083-1089`). | W+C |
| AC-L72 | Hard parent in `rejected` ⇒ fail-fast `parent-rejected` (R16). | W |
| AC-L73 | Hard parent in **`active`** is treated exactly as `applied` in the closure — skipped or replayed by the same arbitration, never fail-fast. **This is a deliberate behaviour change**; the row also asserts it for a **non-landed** target, where it changes today's verdict. | W+C |
| AC-L74 | After the AC-L73 widening, all four `active` call sites agree: `CheckDependencyGate` (`internal/workflow/dependency_gate.go:79-81`), `postApplyVerifyStates` (`internal/workflow/verify.go:127-134`), `isPostApplyState` (`internal/workflow/verify_all.go:89-97`) and the closure switch. Adversarial cross-check. | U |
| AC-L75 | Hard parent in `upstream_merged` is still skipped, byte-identically to today (`internal/workflow/verify.go:1062-1064`). | W |
| AC-L76 | A superseded hard parent is still excluded by the existing filter (`internal/workflow/verify.go:976-983`); landed classification does not resurrect it. | W |
| AC-L77 | Parent landed **after** the target and parent landed **before** the target produce identical verdicts. Landing order is never consulted. | W |
| AC-L78 | Closure ordering for an all-unlanded closure is topological and identical to today. Golden-order assertion. | W |
| AC-L79 | Mixed chain — target unlanded, P1 landed, P2 applied-unlanded ⇒ anchor `HEAD`, P1 skipped, P2 replayed, target forward-verified. Matches the §3.6.6 worked example. | W+C |
| AC-L80 | Mixed chain — target landed, P1 applied-unlanded ⇒ anchor `L^`, P1 replayed there, target judged at both anchors. | W |

#### Group G — V10

| # | Criterion | Tier |
|---|---|---|
| AC-L81 | Landed target, recipe **without** `preimage_hash` (the autogen shape, `internal/workflow/recipe_autogen.go:114-118`) ⇒ V10 passes on the ADR-029 D4 legacy path with no re-warn. | U+W |
| AC-L82 | Landed target, `preimage_hash` **matching the anchor-H closure baseline** ⇒ V10 PASSES with `mode: "historical-anchor"`. **This fixture FAILS today**, because `checkWriteFilePreimage` reads the live working tree (`internal/workflow/writefile_safety.go:108-112`) which holds the post-image. | W+C |
| AC-L83 | Landed target, `preimage_hash: ""` (new-file) with the file **absent** at the anchor-H baseline ⇒ V10 passes. Today the same fixture fails with `new-file collision`. | W |
| AC-L84 | Landed target, `preimage_hash` **not** matching at the anchor-H baseline ⇒ V10 **FAILS** at block severity with R11. No false green for a genuinely stale or destructive recipe. | W |
| AC-L85 | Landed target whose write-file path has changed at `HEAD` since landing ⇒ a **`later-touch` warn advisory** (R12) is emitted and the verdict is **not** blocked by it. This is the ADR-029 D5/D6 warning-class policy. | W+C |
| AC-L86 | A landed feature whose write-file no longer matches its preimage **at HEAD** but does at the anchor ⇒ **PASS + later-touch advisory**, never an automatic block. Pins the exact requirement that a landed write-file must not block merely on a HEAD-side preimage difference. | W |
| AC-L87 | Superseded landed feature with a preimage mismatch at the anchor ⇒ severity downgraded to `warn`, unchanged from ADR-029 D7 (`internal/workflow/verify.go:862-870`). | W |
| AC-L88 | Anchor H unavailable ⇒ V10 **skips** with `skipped: landed V10 requires a historical anchor` and `mode` present; it **never** falls back to the live tree. | W |
| AC-L89 | V2 skipped or failed ⇒ V10 skips with its existing reason (`internal/workflow/verify.go:853-861`), unchanged in both modes. | W |
| AC-L90 | Landed **parent** V10: the same anchor-H semantics apply to closure members; a parent's preimage mismatch at the anchor participates in `parent-landing-drift`, and a parent's HEAD-side later-touch is advisory only. | W |

#### Group H — snapshots, schema, diagnostics and run-level guarantees

| # | Criterion | Tier |
|---|---|---|
| AC-L91 | Classification, V7, V8, V10, the persisted `VerifyRecord` and the derived labels are a **pure function of the §3.6.7 snapshot**. Proven at **U** by constructing two snapshot values and asserting identical outputs, with no filesystem access in the unit under test. | U |
| AC-L92 | An artifact mutated **while verify runs** ⇒ **FAIL `snapshot-unstable`** (R19) naming the path. Proven at **W** with a `PATH` git wrapper that rewrites a `.tpatch/` artifact between the enumeration call and a later git call. No production seam. | U+W |
| AC-L93 | Empty-present and absent artifacts are distinguished at **every** consumer — evidence digest, V7/V8/V10 preconditions, and the persisted hashes. Two fixtures, one per shape. | U+W |
| AC-L94 | `schema_version` is `"1.1"`. A no-evidence report is **semantically** compatible with `"1.0"` — every `"1.0"` key retains its name, type and position — but is **not** byte-identical, because `baseline`, `landing_evidence` and `target_mode` are emitted for every feature. The test asserts the additive superset, not byte identity. | W |
| AC-L95 | `checks[].mode` is present on V7, V8 and V10 in **every** report including when they are `skipped`, and absent on V0–V6 and V9. Both halves asserted. | W |
| AC-L96 | `failed_at` only ever takes a value from the §4.3.9 closed set, and advisory `code` only from `{context-drift, later-touch, unattributed-materialized, base-commit-unreachable}`. Adversarial enumeration. | U+W |
| AC-L97 | Every R1–R19 remediation string is emitted **verbatim** (golden strings). | W |
| AC-L98 | The human report emits `baseline:` and `landing evidence:` above the check list, in that order, naming both anchors in landed mode. | W+C |
| AC-L99 | A passing landed run persists a `VerifyRecord` with the same field set as a passing forward run (`internal/store/types.go:290-296`) — no new persisted field, `omitempty` round-trip preserved. | W |
| AC-L100 | Sticky clearing is mode-agnostic: a feature at `verify-failed` from a pre-fix false red derives `verified-fresh` after one passing landed run, with no migration and no manual edit; and `ComposeLabels` takes no mode input (adversarial: the labels package has no reference to landing evidence). | U+W |
| AC-L101 | `TestRunVerify_EquivalentRecipeAndPatchBothPass` (`internal/workflow/verify_closure_replay_test.go:275`) stays green **unmodified**. The GH #2 non-regression anchor. | W |
| AC-L102 | The GH #2 reset holds at anchor H: after V7 mutates the shadow, the tree hash seen by V8 equals `closureBaselineTree` (`internal/workflow/verify.go:1092`, `:1143`). | W |
| AC-L103 | The shadow is pruned on **every** exit path, including each new failure path (`landing-evidence`, `landed-artifacts-absent`, `snapshot-unstable`, every `parent-*`). | W |
| AC-L104 | `verify --all` output ordering over a mixed landed/unlanded set is byte-identical to today (Q12 resolved: no reordering). | C |
| AC-L105 | Exit codes are unchanged: `0` pass, `2` any block-severity failure including every new evidence failure, `1` reserved. Warn-class advisories never change the exit code. | C |
| AC-L106 | `gofmt -l .` clean, `go build ./cmd/tpatch` clean, `go test ./...` clean, and `make wave-close-check` 8/8 at the Wave C close commit. | C |

**Matrix size: 106 numbered criteria (AC-L1 … AC-L106) across 8 groups** —
A 6, B 12, C 10, D 19, E 12, F 21, G 10, H 16.
Tier distribution is recorded per row; a Wave C dispatch that cannot place a
row at its stated tier must amend this section rather than silently re-tier it.

#### 7.1.1 Explicit non-goals for Wave C

- No change to `tpatch land` behaviour, output or refusals
  (`PRD-tpatch-land` §3.8 is a readers' contract; §6.2 AC-LD9 is the guard).
- No new check ID, no new `FeatureState`, no new `ReconcileLabel`, no new
  persisted field, no new artifact, no `.tpatch/` schema migration.
- No change to forward-mode V7/V8 semantics for features with evidence `none`
  — **except** the `active` closure widening (AC-L73/AC-L74), which is a
  deliberate, separately-pinned behaviour change.
- No fix for forward-mode V10's live-tree reference (Q15) — out of scope,
  tracked.
- No provider calls; verify stays offline.
- No auto-healing: verify never invokes `record`, `land` or `apply`.
- No `--all` reordering (Q12, resolved).
- The Q14 `--verbose` hardening is SHOULD, not MUST.


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
| **Anchor-C `-C0` step can pass when the feature was reverted in place and identical text exists elsewhere in the file.** | Measured and bounded: it requires a deliberate revert-plus-paste; it surfaces as a `context-drift` **warn advisory**, never a clean pass; V7 at anchor H is an independent corroboration; and Q14 names a `--verbose` hardening that fires exactly on that shape. |
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
