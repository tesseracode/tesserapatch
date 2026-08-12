# PRD — `tpatch verify` and verification freshness overlay

**Status**: Approved (M15 Wave 3 — APPROVED WITH NOTES at commit 3c122aa; Slice A in implementation. Supersedes `docs/prds/PRD-verify-and-tested-state.md`.) · **§3.6 / §4.3 golden refresh / §4.3.6–4.3.9 / §5 landed rows / §6 Q9–Q18 / §7.1 amended and ACCEPTED 2026-08-12 for v0.15.1 Wave B (GH #8), rev-7**
**Date**: 2026-04-27 (original) · 2026-08-12 (landed-feature amendment)
**ADR**: **ADR-013-verify-freshness-overlay.md — REQUIRED before Wave 3 implementation slices ship.** ADR-013 supersedes ADR-012 in full. **ADR-013 Amendment 1 rev-7 (final, D8–D19) is the binding ADR for §3.6 and §7.1.**
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
- **Changes to `tpatch land`'s behavior — with exactly one specified
  exception.** The v0.15.1 Wave B amendment is primarily a **readers'**
  contract over `land`'s trailer block
  (`docs/prds/PRD-tpatch-land.md` §3.8.1–§3.8.5) and changes nothing `land`
  writes on a healthy workspace. **The one exception, mandated by ADR-013
  Amendment 1 D19 and specified in `PRD-tpatch-land` §3.8.6, is a new producer
  refusal**: `land` refuses (R23) when `status.apply.base_commit` is not valid
  for the reader grammar, so an unreadable `Tpatch-Base-Commit` can never be
  emitted. Beyond that refusal, `land`'s successful path, its pre-existing
  refusals, its trailer set and its status writes are unchanged, and no new
  status metadata is added.
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

> **Landed-feature amendment (2026-08-12, v0.15.1 Wave B / GH #8, rev-7).**
> After `tpatch land` commits a feature into reachable Git history, the
> HEAD-anchored V7/V8 baseline already contains it and forward-apply semantics
> stop describing the world: `write-file` recipes pass vacuously,
> `replace-in-file` recipes false-fail, `append-file` recipes pass while
> corrupting the shadow, and V8 always fails. **§3.6** defines the landed
> contract — a conservative trailer-grammar evidence reader over one
> raw+parsed `--topo-order --reverse` enumeration with a **closed three-state
> artifact presence precedence** and an object-format-derived commit-id
> length; **dual-anchor** verification pairing a historical replay at a
> **replay anchor chosen by collecting every candidate and qualifying it with
> a forward apply** against an **index-isolated** current-HEAD ladder whose
> reduced-context hunks **block**; closure arbitration decided solely by that
> non-mutating patch ladder; **per-member V10 baselines** with
> `RecipeProvenance.BaseCommit` anchoring forward mode (Q15 resolved); a
> **full repository metadata inventory**; and explicit **shallow** /
> **partial-clone** states. **§7.1** is its 135-row executable acceptance
> matrix. Binding ADR: ADR-013 Amendment 1 rev-7 (final, D8–D19).
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

> **The shipped sequence is eleven checks, V0 … V10, and V10 is last**
> (`internal/workflow/verify.go:49-71`; V10 appended at `:288-289`). V10
> (`write_file_preimage_fresh`) was added in v0.12.0 Wave β Slice R4 under
> `PRD-write-file-recipe-safety` / ADR-029 and is included in the table below.
> **Superseded**: any statement elsewhere in this document that describes the
> sequence as ten checks, or names V9 as the final check, is historical and is
> labelled as such where it survives. §7.1.2 G1/G2 enforce this mechanically.

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
| V8 | `post_apply_patch_replay_clean`| `artifacts/post-apply.patch` present | block     | After the same closure replay used by V7, `git apply --check` of `post-apply.patch` succeeds against the **closure-replayed baseline tree**. Per **GH #2 / v0.11.3**, when a recipe was applied in V7 the one shared shadow is **reset to `closureBaselineTree`** first (`resetShadowToTree`, `internal/workflow/verify.go:1142-1153`, `:1227-1229`), so recipe and patch are checked **independently against the same baseline** rather than double-applied | one shadow allocation shared with V7, **reset between them**; `git apply --check` |
| V9 | `reconcile_outcome_consistent` | `status.Reconcile.Outcome` set     | warn        | `Outcome ∈ {reapplied, upstreamed, still_needed}` | reads `status.Reconcile.Outcome` only — never any artifact (D6) |
| V10 | `write_file_preimage_fresh`   | recipe present and parsed (skipped with `skipped: V2 (recipe_parses) skipped or failed` otherwise) | block, **downgraded to `warn` when the feature is superseded by an active superseder** | every `write-file` op's `preimage_hash` still matches the current on-disk file; `preimageLegacyWarn` (no `preimage_hash`, ADR-029 D4 legacy path) and `preimageSkip` (not a write-file) do not fail the check | `checkWriteFilePreimage` (`internal/workflow/writefile_safety.go:108-112`), `IsFeatureSuperseded`; check body at `internal/workflow/verify.go:852-905` |

#### 3.1.1 Ordering rationale

V0 → V6 are **static**: file checks, struct unmarshals, regex/git ancestor, in-process function calls. V7 and V8 are **dynamic** (shadow worktree + parent closure replay + recipe/patch apply). Static block runs first so a recipe-shape error doesn't waste a shadow allocation. V7 and V8 share a **single** shadow allocation: the closure is replayed once, the target recipe is applied for V7, and then — per **GH #2 / v0.11.3** — the shadow is **reset to the closure-replayed baseline tree** before `git apply --check` runs for V8, so an equivalent recipe/patch pair is validated **independently against the same baseline** instead of being applied twice on top of itself. V9 follows — an informational read of `status.Reconcile.Outcome`.

**V10 is last.** `write_file_preimage_fresh` is appended after V9
(`internal/workflow/verify.go:288-289`). It is placed last because it is a
post-hoc integrity assertion over the parsed recipe that needs neither the
shadow nor V9's outcome, and because its severity is resolved late: it is
`block` normally and **`warn` when `IsFeatureSuperseded` reports an active
superseder** (ADR-029 D7). It is skipped when V2 skipped or failed, since it
must iterate a parsed recipe.

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
- V10 → the joined per-op `checkWriteFilePreimage` rejection messages, `; `-separated. When the feature is superseded the joined string is suffixed with `" (downgraded to warn: superseded by \"<superseder>\" per ADR-029 D7 + PRD-feature-supersession §4.5 \"Reconcile interaction with write-file safety\")"` (`internal/workflow/verify.go:896-905`).

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

The persisted record deliberately does **not** carry the per-check array. The full **eleven-row** check results (V0–V10, `internal/workflow/verify.go:49-71`) live on the in-memory `VerifyReport` and are emitted on `tpatch verify --json` stdout only (LOG entry `3c122aa` Note 1 — the authoritative disposition). Slice B's `ComposeLabels` derivation reads only the persisted minimal fields.

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
4. After all replayable parents have replayed, record the resulting
   **closure-replayed baseline tree** and apply the **target's** recipe in the
   shadow. This is the V7 check.
5. **GH #2 / v0.11.3 — reset between V7 and V8.** If V7 applied a recipe, the
   shadow is reset to the recorded closure-replayed baseline tree
   (`resetShadowToTree`, `internal/workflow/verify.go:1142-1153`) **before**
   `git apply --check` runs the target's `post-apply.patch`. V8 therefore
   validates the patch **independently, against the same baseline V7 used** —
   not against a tree that already contains V7's recipe output. Without the
   reset an equivalent recipe/patch pair double-applies and V8 false-reds.
   This invariant is preserved verbatim by the landed contract in §3.6.
6. Prune the shadow regardless of pass/fail.

**Fail-fast semantics.** The first parent that fails to replay causes verify to abort the V7/V8 phase (V0–V6, V9 and V10 still run / have run). The freshness record is written with `passed=false` and the V7 entry's `remediation` carries the failing parent slug + wrapped error. The `--json` report includes a top-level `failed_at: "parent-replay"` and a `parent_slug` field; the human report includes the same.

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
### 3.6 Landed-feature verification contract — v0.15.1 Wave B / GH #8 (rev-7)

> **Amendment status**: proposed **rev-7 (final)**, 2026-08-12, AWAITING
> REVIEW. Binding ADR: **ADR-013 Amendment 1 rev-7, decisions D8–D19
> (final set — no decision is added after D19).** Implementation is
> Wave C. Issue: <https://github.com/tesseracode/tesserapatch/issues/8>.
> Co-amended: `docs/prds/PRD-tpatch-land.md` §3.8.
>
> **Revision history.** rev-0 omitted **V10** and judged a landed feature only
> at `HEAD`. rev-1 introduced dual anchors but read the **working tree**, left
> the `(0/0)` hardening optional, let an unavailable anchor skip-and-pass,
> and reused byte equality in parent arbitration. rev-2 closed those but left
> nine residuals. **rev-3 closes all nine**: anchor candidates are collected
> exhaustively and qualified by **forward** apply (§3.6.8); duplicates use a
> **normalized zero-context identity** (§3.6.8); each landed member gets its
> **own** V10 baseline and unlanded members use **`RecipeProvenance.BaseCommit`**,
> resolving Q15 (§3.6.7); the snapshot is a **full repository inventory**
> (§3.6.9); patch presence is a **closed three-state precedence** (§3.6.2);
> **shallow and partial clones** get their own states (§3.6.8); the base-commit
> hex length is **derived from the object format** (§3.6.2); and `land`
> **refuses** an invalid base commit (`PRD-tpatch-land` §3.8.6). Every claim
> was measured; the probe index is ADR-013 §A1.1 **E1–E47**.

#### 3.6.1 The defect this section closes

`tpatch land` commits a feature into reachable Git history while deliberately
leaving `status.apply.base_commit` untouched (`PRD-tpatch-land` §3.6;
`internal/cli/land.go:394`; ADR-019). V7/V8 allocate their shadow from
**current HEAD** (`internal/workflow/verify.go:1012`, `:1024`).

Measured at `13a885c`; every run reported `checks=11` — the shipped set is
**V0–V10** (`internal/workflow/verify.go:49-71`), V10 appended last
(`:288-289`):

| Target recipe op kind | pre-land | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file`      | ✓ / ✓ | **✓ false green** (`internal/workflow/verify.go:1290-1294`) | **✗ false red** |
| `replace-in-file` | ✓ / ✓ | **✗ false red** (`internal/workflow/verify.go:1295-1305`) | skipped |
| `append-file`     | ✓ / ✓ | **✓ false green, shadow double-appended** (`internal/workflow/verify.go:1306-1313`) | **✗ false red** |

The defect is not V8-only, and the same hazard applies to a landed hard
**parent** (`internal/workflow/verify.go:1048-1091`).

#### 3.6.2 Evidence reader — enumeration, grammar, and closed presence states (ADR-013 D10)

**Enumeration.** Exactly one
`git log --topo-order --reverse -z --format='%H%x1f%P%x1f<four trailers>%x1f%B'`
per verify run over commits reachable from the resolved `HEAD`, **cached and
reused for every feature** of a `verify --all` run. Records arrive
**oldest-first**. `rev-list` is **not** used — it cannot emit `%B`. Never
`--first-parent`.

**Conservative raw precedence.** Any commit whose **raw** message contains a
line exactly `Tpatch-Feature: <slug>` (after trimming trailing ASCII
space/tab) but whose **parsed terminal trailer block** does not yield that slug
is **`malformed`** — never `none`. A prose quotation and an amend-destroyed
block are measurably indistinguishable; the prose false red is accepted
deliberately.

**Grammar (normative).**

| Element | Rule |
|---|---|
| Key case | Git matches trailer keys **case-insensitively**; inherited and stated. |
| `Tpatch-Feature` cardinality | **Exactly one value**; two or more ⇒ `malformed`. |
| `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` / `Tpatch-Base-Commit` | **Exactly one each**; zero or ≥2 ⇒ `malformed`. No "take the first". |
| Slug match | Exact equality after trimming leading/trailing ASCII space and tab. |
| `Tpatch-Patch-SHA` format | 64 lowercase hex — SHA-256 of artifact bytes, independent of the repository object format. |
| `Tpatch-Recipe-SHA` format | 64 lowercase hex **or** the literal `none`. |
| `Tpatch-Base-Commit` format | **`N` lowercase hex, `N` derived from `git rev-parse --show-object-format`** — 40 for `sha1`, **64 for `sha256`** (measured). Hardcoding 40 would reject every valid landing in a SHA-256 repository. If the format cannot be read ⇒ `unavailable`. |
| Reader failure | Git error, unparsable output, or a git below the §3.6.9 floor ⇒ **`unavailable`**, a block failure distinct from `none` and `malformed`. |

**Artifact presence — three closed, mutually exclusive states, evaluated
*first*, before any digest comparison and therefore before any `exact` /
`stale` classification. `exact` and `stale` are reachable only from
`present-nonempty`.**

| State | Definition | Effect on a landing candidate |
|---|---|---|
| **absent** | the file does not exist in the inventory | **Terminal `landed-artifacts-absent`.** The candidate can be neither `exact` **nor** `stale`, because no digest exists to compare. **No digest comparison is attempted**, no mismatch is reported, and no downstream arbitration row for this member is reachable. |
| **present-empty** | patch: zero bytes. Recipe: zero bytes **or** whitespace-only. | **Patch**: **terminal `landed-artifacts-absent`**, the same short-circuit as `absent` — `land` refuses when the embedded `record` would capture nothing, so a landed empty patch is corruption or a hand edit, and it can be neither `exact` nor `stale`. **Recipe**: the expected trailer value is the literal `none`, mirroring `readRecipeSHA` (`internal/cli/land.go:1039-1041`); the recipe's shape never rescues an absent-or-empty patch. |
| **present-nonempty** | patch: ≥1 byte. Recipe: ≥1 non-whitespace byte. | The **only** state in which a digest comparison happens, and therefore the only state from which `exact` or `stale` is reachable. |

The recipe's four shapes are named separately and never overlap: **absent**,
**present-empty** (⇒ expected trailer `none`), **present-nonempty-zero-op**
(parses, `operations` empty ⇒ V7 has nothing to replay and cannot attest), and
**present-nonempty-with-ops**. **Recipe shape is consulted only within a
`present-nonempty` patch state** — the two short-circuiting patch states
terminate the member before the recipe affects classification.

**Reachable outcome table — 3 patch states × 4 recipe shapes, mutually
exclusive and total.**

| patch \ recipe | absent | present-empty | present-nonempty-zero-op | present-nonempty-with-ops |
|---|---|---|---|---|
| **absent** | `landed-artifacts-absent` | `landed-artifacts-absent` | `landed-artifacts-absent` | `landed-artifacts-absent` |
| **present-empty** | `landed-artifacts-absent` | `landed-artifacts-absent` | `landed-artifacts-absent` | `landed-artifacts-absent` |
| **present-nonempty** | digest compared; trailer must be `none`; patch ladder sole authority; V7/V10 skip | digest compared; trailer must be `none`; patch ladder sole authority; V7/V10 skip | digest compared; V7 records `0 op(s)` and cannot attest; patch ladder sole authority | digest compared; V7 replays; patch ladder **and** V7 both apply |

Every cell is reachable and no cell matches two rules. The eight
`landed-artifacts-absent` cells are the corruption/hand-edit region `land`
cannot produce.

**Evidence states — closed set of ten, total:**

| Candidate population / condition | State | Effect |
|---|---|---|
| no candidate and no raw-body match | `none` | forward mode — today's behavior |
| exactly one well-formed candidate, patch **present-nonempty**, all three values match | `exact` | landed mode |
| ≥2 such candidates with equal normalized identity (§3.6.8) | `duplicate-equivalent` | landed mode; `duplicates: n` |
| ≥2 such candidates with differing identities, or incomparable | `ambiguous` | **FAIL** `landing-evidence` |
| 0 all-match, ≥1 well-formed-but-mismatched, patch **present-nonempty** | `stale` | **FAIL** `landing-evidence` |
| cardinality / format / raw-only failures | `malformed` | **FAIL** `landing-evidence` |
| every candidate has ≥2 parents, or 0 parents in a **non-shallow** repository | `unsupported-topology` | **FAIL** `landing-evidence` |
| the repository is shallow and a candidate sits on the graft boundary | **`shallow-history`** | **FAIL** `landing-evidence` |
| a needed object is missing in a partial clone, or a promisor fetch fails | **`history-incomplete`** | **FAIL** `landing-evidence` |
| reader error, or the object format cannot be read | `unavailable` | **FAIL** `landing-evidence` |

Only `none` degrades to forward mode; the other eight non-`exact` states are
terminal.

#### 3.6.3 What each check independently proves in landed mode

| Check | Anchor | Independent obligation |
|---|---|---|
| **V7** `recipe_replay_clean` | historical | the recipe still **forward-applies**, by replay, to the tree it was authored against, with its closure reconstructed. Never an alias of V8. |
| **V8** `post_apply_patch_replay_clean` | historical **and** current | (a) at the anchor, `git apply --check` forward succeeds after the GH #2 reset; (b) at `HEAD`, the canonical patch is still **materialized** per the §3.6.5 ladder, in an isolated index. Both block-severity, reported separately. |
| **V10** `write_file_preimage_fresh` | **per-member** baseline (§3.6.7) | each `write-file` op's `preimage_hash` still describes the tree it was recorded against, plus an ADR-029 **warn**-class later-touch from `RequestedAt` + touched-path metadata. |

#### 3.6.4 Baseline model — dual-anchor landed verification (ADR-013 D9)

- **Anchor H (historical)** — shadow rooted at the **replay anchor's** single
  parent tree (§3.6.8). Closure arbitration (§3.6.6) runs, then the existing
  machinery: V7 replays, the shadow is reset to `closureBaselineTree`, V8 runs
  `git apply --check` forward, V10 evaluates preimages.
- **Anchor C (current)** — an **index-isolated** assertion at `HEAD` (§3.6.5).

**Implementation delta**: the commit-ish handed to `gitutil.CreateShadow`
(`internal/gitutil/shadow.go:56`) becomes the replay anchor's parent tree
instead of `gitutil.HeadCommit` (`internal/workflow/verify.go:1012`, `:1024`),
plus the §3.6.5 temp-index ladder.

**GH #2 (v0.11.3) invariant, binding in every mode.** The recipe and the patch
are validated independently against the same baseline tree, with the shadow
reset to `closureBaselineTree` between them
(`internal/workflow/verify.go:1092`, `:1143`). Normative: *any check that may
mutate the shadow MUST reset it to `closureBaselineTree` before the next check
runs; V7's result is never an input to V8's tree.*

**Non-landed features are untouched** for V7/V8: evidence `none` ⇒ shadow at
`HEAD`, byte-for-byte as today. V10 changes per §3.6.7.

**Anchor H unavailability is TERMINAL** — §3.6.8.

#### 3.6.5 Anchor C — index-isolated, with a mandatory `(0/0)` block (ADR-013 D11, D12)

**Normative implementation.**

1. Create a temporary index under `$(git rev-parse --git-dir)` — measured:
   invisible to `git status --porcelain`, real index byte-identical, worktree
   byte-identical — or under the gitignored `.tpatch/local/` root
   (`internal/cli/land_journal.go:31`, `:60`;
   `internal/gitutil/ignore.go:50-51`). **Never** inside the tracked working
   tree.
2. `GIT_NO_LAZY_FETCH=1 GIT_INDEX_FILE=<tmp> git read-tree <commit-or-tree>` —
   `HEAD` for anchor C, **`C^`** for a candidate-parent probe (§3.6.8).
   **Never `C^{tree}^`**: measured, `git rev-parse C^{tree}^` returns
   `error: object <tree> is a tree, not a commit` and `git read-tree C^{tree}^`
   fails outright. The valid forms are `C^`, `C^^{tree}`, `C~1`, `C~1^{tree}`.
3. `GIT_NO_LAZY_FETCH=1 GIT_INDEX_FILE=<tmp> git apply --check [--reverse] --cached [-C1|-C0 --verbose] <patch>`.
4. Remove the temp index on **every** exit path, in a deferred cleanup.

**`GIT_NO_LAZY_FETCH=1` is mandatory on every object and materialization
command** — `read-tree`, `apply`, `diff`, `cat-file`, `merge-base` and the
enumeration. Measured: with a promisor remote configured and its object
absent, `git cat-file -p <blob>` **attempts the network** and fails with
`fatal: '<url>' does not appear to be a git repository`; the same command under
`GIT_NO_LAZY_FETCH=1` fails immediately and locally with
`fatal: Not a valid object name <sha>`. Verify is offline by construction
(§1.3), so a silent network fetch is a contract violation, and the local
failure is what lets §3.6.8 classify `history-incomplete` deterministically.

Results are memoised per `(tree, patch, direction, context)`.

Measured: with the feature reverted in the worktree only, the rev-1
worktree-based check **FAILS** while the isolated form passes.

**What reverse-apply proves.** The patch's **postimage hunks are present in the
given tree**, matched by content with a line-offset search and a configurable
context requirement. **Not** byte-exact, **not** ownership.

**The hardened ladder — `(0/0)` blocks.**

| Step | Command (all against the temp index) | Outcome |
|---|---|---|
| 1 | `git apply --check --reverse --cached <patch>` | pass ⇒ **materialized, clean** |
| 2 | on failure: `LC_ALL=C git apply --check --reverse --cached -C0 --verbose <patch>` | pass **and zero** `Context reduced to (0/0)` ⇒ **materialized, context drift**: block check passes, `warn` `context-drift` advisory |
| 3 | step 2 passes **but reports ≥1** `(0/0)`, **or** step 2 fails | **BLOCK** — `landed-content-absent` |

`LC_ALL=C` is **mandatory**.

**Measured basis**, 3-hunk patch in a 60-line file — `C3` / `C0` / `(0/0)`:

| Scenario | C3 | C0 | (0/0) | rule |
|---|---|---|---|---|
| pristine landed tip | OK | OK | 0 | clean |
| landing parent (feature absent) | FAIL | FAIL | 0 | block |
| 10 lines prepended; unrelated edit far away | OK | OK | 0 | clean |
| unrelated edit **2 lines** from a hunk | FAIL | OK | 0 | **warn** |
| unrelated edit **1 line** from a hunk | FAIL | OK | **1** | **block** (accepted false red) |
| partial revert — hunk 1 / hunk 2 / hunk 3 / hunks 1+3 | FAIL | FAIL | 0 | block |
| full revert | FAIL | FAIL | 0 | block |
| degenerate whole-file hunk + header/footer | FAIL | OK | 0 | warn |
| **revert-in-place + identical text pasted at EOF** | FAIL | OK | **1** | **block** ✔ |
| patched file deleted | FAIL | FAIL | 0 | block |

Randomized 220-tree corpus:

| Rule | false greens (69 absent) | false reds (151 present) |
|---|---|---|
| rev-1, `(0/0)` ignored | **2** | 0 |
| rev-2/rev-3, any `(0/0)` blocks | **0** | 26 |

Blocking is the chosen trade; each false red carries remediation R2. A
stronger hunk-local corroboration was considered and **not adopted** — none
could be *proved* on the measured corpus.

**No generalised claims.** Only the per-scenario triples above are asserted.

#### 3.6.6 Closure arbitration — non-mutating, patch-ladder-only (ADR-013 D13)

**The presence test for any closure member is the §3.6.5 hardened ladder over
that member's canonical `post-apply.patch` against the anchor tree, probed
through the temp index.** Non-mutating; **not** recipe replay; **not**
whole-file byte equality. Recipe op predicates are **diagnostics only** — they
localise which op/path a failure concerns and never certify presence.

| Member condition | Action |
|---|---|
| `upstream_merged` | **skip** (unchanged, `internal/workflow/verify.go:1062-1064`) |
| superseded by an active superseder | **skip** (unchanged, `internal/workflow/verify.go:976-983`) |
| evidence `exact`/`duplicate-equivalent`, patch **present-nonempty**, ladder ⇒ clean or context-drift | **skip** |
| evidence `exact`/`duplicate-equivalent`, patch present-nonempty, ladder ⇒ block | **fail-fast** `parent-landing-drift` |
| patch **absent** or **present-empty**, whatever the recipe shape | **fail-fast** `landed-artifacts-absent` — reached **before** any digest comparison and therefore before any `exact`/`stale` classification (§3.6.2). A slug-bearing candidate on such a member never becomes an attestation. |
| evidence `none`, patch present-nonempty, ladder ⇒ clean or context-drift | **skip** with a mandatory `warn` `unattributed-materialized` advisory. No ownership claimed. |
| evidence `none`, patch present-nonempty, ladder ⇒ block | **replay** (unchanged, `internal/workflow/verify.go:1065-1082`) |
| evidence `none`, patch absent or present-empty | **replay** |
| evidence `stale` / `ambiguous` / `malformed` / `unsupported-topology` / `shallow-history` / `history-incomplete` / `unavailable` | **fail-fast** `parent-evidence-integrity` |
| `unapplied` | **fail-fast** `parent-unapplied` |
| `rejected` | **fail-fast** `parent-rejected` |
| any other state | **fail-fast** (unchanged `default:`) |

A landed member additionally contributes its **V10** result per §3.6.7.

**Revert timing.** "Reverted" means the member's canonical patch fails the
ladder **at the anchor tree being built**. A revert landed *after* the anchor
commit is invisible at anchor H and is caught at anchor C; a revert predating
the anchor is caught at anchor H. Both anchors are reported.

**`active` is total.** Treated **identically to `applied`** everywhere in the
closure. Today the switch handles only `upstream_merged` and `applied`, so an
`active` hard parent reaches `default:` and fail-fasts
(`internal/workflow/verify.go:1061-1089`) — while `CheckDependencyGate`
accepts both (`internal/workflow/dependency_gate.go:79-81`),
`postApplyVerifyStates` admits `active` (`internal/workflow/verify.go:127-134`)
and `isPostApplyState` does too (`internal/workflow/verify_all.go:89-97`).
**This is a deliberate behaviour change for non-landed features**, pinned by
AC-L86/AC-L87 and carried as a §8 risk row.

**Worked mixed chains.**

- *Target unlanded, P1 landed, P2 applied-unlanded* — anchor `HEAD`; P1's patch
  ladder-passes there and is skipped; P2's blocks and is replayed; the target
  is forward-verified as today.
- *Target landed, P1 applied-unlanded* — anchor is the target's replay-anchor
  parent tree; P1's patch blocks there and is replayed; V7/V8/V10 run at anchor
  H and the ladder runs at anchor C.
- *Target landed, P1 landed but reverted before the anchor* — fail-fast
  `parent-landing-drift` before the target is judged.

#### 3.6.7 V10 — per-member baselines, and Q15 resolved via `RecipeProvenance` (ADR-013 D15)

rev-2 evaluated V10 at the **target's** anchor-H baseline. For a landed
**parent** that tree may already contain the parent's own postimage, so its
`preimage_hash` would be compared against a tree that already has the write
applied. rev-3 gives every member its **own** baseline.

| Member condition | V10 reference tree |
|---|---|
| the **target**, landed | the target's anchor-H **closure baseline** (shadow after arbitration, before the target's recipe replays) |
| a **landed** closure member | **that member's own** replay-anchor parent tree, resolved by running §3.6.8 for that member's slug. **Never** the target's anchor. |
| an **unlanded** member (evidence `none`), or the target in forward mode | **`RecipeProvenance.BaseCommit`** for that member |
| any op with no `preimage_hash` | ADR-029 D4 legacy path: pass, no re-warn (`internal/workflow/verify.go:879-883`) |

**Q15 is RESOLVED; rev-2's "no anchor exists" rationale is withdrawn.**
Measured: the shipped `RecipeProvenance`
(`internal/workflow/implement.go:30-34`) records `base_commit` — the `HEAD` the
recipe was generated against — plus `recipe_sha256`; `implement` writes
`artifacts/recipe-provenance.json` (`:220-238`) on the same path that emits
`preimage_hash`, while the autogen path that omits `preimage_hash`
(`internal/workflow/recipe_autogen.go:114-118`) writes no provenance. The two
travel together, and `warnRecipeStale` (`internal/cli/cobra.go:1285-1315`)
already consumes both.

**Exact use.** For each `write-file` op carrying a non-nil `preimage_hash`, the
reference tree is `RecipeProvenance.BaseCommit` **iff all four hold**:

1. **present** — `artifacts/recipe-provenance.json` exists in the inventory and
   decodes;
2. **well-formed** — `base_commit` is `N` lowercase hex for the repository's
   object format (§3.6.2);
3. **reachable** — `gitutil.IsAncestor(root, base_commit, "HEAD")`
   (`internal/gitutil/gitutil.go:828`) is true, or it is `HEAD` itself.
   Unreachable ⇒ a `warn` `provenance-unreachable` advisory **and** the failure
   below;
4. **inventory-consistent** — `recipe_sha256`, when present, equals the sha256
   of the inventory's `apply-recipe.json` bytes. When absent (pre-v0.5.2
   sidecars, which `warnRecipeStale` already tolerates,
   `internal/cli/cobra.go:1310-1313`) the binding is weaker; the contract
   accepts it and reports `provenance_hash_bound: false`.

If an op carries a real `preimage_hash` but provenance fails 1–3, verify
**FAILS** with `failed_at: "recipe-provenance-unavailable"` (R24). It **never**
falls back to reading the live working tree — the measured false block that
made `checkWriteFilePreimage` (`internal/workflow/writefile_safety.go:108-112`)
fail for every applied feature.

**V10 outcomes, per member.**

| Case | Outcome |
|---|---|
| `preimage_hash` absent | legacy pass, no re-warn (ADR-029 D4) |
| present and matching at the member's reference tree | **PASS**, `mode` naming the anchor kind |
| present and **not** matching | **FAIL**, block; downgraded to `warn` when superseded (ADR-029 D7, `internal/workflow/verify.go:862-870`) |
| `preimage_hash` malformed per ADR-029 D1 | **FAIL**, block — the preimage contract itself is invalid |
| provenance required but absent / ill-formed / unreachable | **FAIL** `recipe-provenance-unavailable` |
| V2 skipped or failed | **skip**, unchanged reason (`internal/workflow/verify.go:853-861`) |
| the member's own anchor unavailable | **FAIL** `historical-anchor-unavailable` for that member |

**Later-touch — metadata, never bytes.** The shipped detector: `RequestedAt`
ordering (`internal/workflow/writefile_safety.go:409-442`), touched-path union
of `patch-generations.json.touched_paths`
(`internal/store/patch_generations.go:52`) and recipe op paths
(`:449-481`), the `path → first later slug` index (`:380-388`) and the per-op
predicate `checkLaterTouch` (`:489-498`);
`DetectRecordLaterTouchWarnings` (`:571`) is the exported record-time entry.
A hit raises a **`warn` `later-touch` advisory** and **never blocks** —
ADR-029 D6 is warning-class while D5 makes stale preimages fail. Single stated
exception: a malformed `preimage_hash`, or a mismatch at the member's own
reference tree, blocks on its own terms.

**Aggregation into the target verdict.** A member's **block-class** V10 outcome
contributes to `parent-landing-drift` for that member and is named in the
fail-fast reason. A member's **warn-class** outcome is aggregated into
`advisories`, attributed to the member's slug, and affects no verdict. The
target's own V10 row carries the target's block-class result and its own `mode`.

#### 3.6.8 Anchor collection, forward qualification, normalized identity, topology (ADR-013 D14, D16, D18)

**Attestation candidate** — determines `landing_evidence.state`. Well-formed,
single-`Tpatch-Feature`, exact-slug, patch **present-nonempty**, and its three
recorded values match the inventory. This is the **authority**.

**Replay-anchor candidates — collect, qualify, compare, select.** rev-2 said
"take the first qualifier" while also requiring non-equivalent qualifiers to
be ambiguous, which is a contradiction: stop-at-first can never observe a
second qualifier.

1. **Collect.** From the single enumeration, take **every** commit that is
   reachable from `HEAD`, carries exactly one `Tpatch-Feature` value equal to
   the slug with a parseable terminal trailer block, and has **exactly one**
   parent in `%P`. Their own hashes **may be stale**.
2. **Qualify by FORWARD apply at `-C1`.** For each candidate `C`:

   ```
   GIT_NO_LAZY_FETCH=1 GIT_INDEX_FILE=<tmp> git read-tree C^
   GIT_NO_LAZY_FETCH=1 GIT_INDEX_FILE=<tmp> git apply --check --cached -C1 <canonical patch>
   ```

   `C` qualifies iff that **forward** check succeeds. The revision string is
   **`C^`** (or `C^^{tree}` when a tree object is explicitly wanted); `C^{tree}^`
   is invalid and is never used (§3.6.5).

   **Why forward.** Measured: at a clean pre-landing tree forward passes and
   reverse fails; at the landing itself forward fails and reverse passes.
   Qualification asks *"does the current canonical patch apply here"* — the
   question a replay baseline must answer.

   **Why `-C1`.** Measured ladder at candidate parent trees:

   | Candidate parent tree | `-C3` | `-C1` | `-C0` |
   |---|---|---|---|
   | pristine pre-landing parent | OK | OK | OK |
   | unrelated edit 4 lines from the hunk | OK | OK | OK |
   | unrelated edit **2 lines** from the hunk | **FAIL** | **OK** | OK |
   | unrelated edit **1 line** from the hunk | FAIL | **FAIL** | OK |
   | 10 lines prepended (pure offset) | OK | OK | OK |
   | unrelated edit far away | OK | OK | OK |
   | **tree that already has the feature** | FAIL | **FAIL** | **FAIL** |
   | feature line changed to something else | FAIL | **FAIL** | **FAIL** |
   | feature line deleted | FAIL | **FAIL** | **FAIL** |

   Default context rejects a healthy parent whose neighbourhood moved by two
   lines — which can **dead-end the operator's own re-record-and-re-land
   remediation**, since the re-recorded patch spans the drift. `-C1` keeps
   every healthy parent in the table while still rejecting all three
   already-materialized or otherwise-modified trees. **`-C0` is rejected**: it
   discards context entirely, the same over-permissive setting §3.6.5 refuses
   in the reverse direction.

   **Accepted bound.** A parent carrying an unrelated edit **1 line** from the
   hunk does not qualify even at `-C1`. That is the measured false-red
   boundary; the only alternative is `-C0`, which is not safe.
3. **Compare.** If **more than one** candidate qualifies, compute each one's
   **normalized change identity** (below) and compare. All equal ⇒ equivalent.
   Any differ ⇒ **ambiguous** anchor ⇒ terminal. No selection happens before
   every qualifier has been collected and compared.
4. **Select.** Among equal-identity qualifiers, the **first in the
   enumeration's native `--topo-order --reverse` (oldest-first) order**; final
   tie-break, the lexicographically smallest full commit id.

**No broadening** — the collection never falls back to "any commit that
introduced these paths".

**Integrity boundary.** A qualifying candidate whose own hashes no longer match
the inventory supplies **only a tree**. It never contributes to
`landing_evidence.state`, never relaxes the attestation requirement, and is
never reported as the attestation commit. The report carries both separately:
`landing_evidence.attestation_commit` and
`baseline.historical_anchor.replay_anchor_commit`.

**Unavailability is TERMINAL.** Empty collection, no qualifier, or
non-equivalent qualifiers ⇒ `failed_at: "historical-anchor-unavailable"`
(R11). V7, V8's historical half and V10 report `passed: false` with `mode`
present — **failed-because-unanchored**, not skipped — and the run **never**
passes on anchor C alone.

**Re-land regains anchor H.** After `tpatch record` + `tpatch land`, the newest
landing is the attestation authority and the earlier landing remains a
qualifying replay anchor (measured), so anchor H is regained and the run
passes. If no qualifier exists, the run fails with R11 rather than degrading.

**Normalized change identity.** For candidate `C` over path set `P`:

```
GIT_NO_LAZY_FETCH=1 git diff --no-color --no-ext-diff --no-textconv \
    --binary --no-renames --unified=0 C^ C -- <P...>
```

post-processed by **exactly two** rules, then hashed with SHA-256:

1. **drop** every line beginning `index `;
2. **rewrite** every hunk header — any line matching
   `^@@ -<range> \+<range> @@.*$` — to the bare token `@@`, discarding both
   line ranges **and** the optional function-context suffix.

Nothing else is altered: hunk **bodies** keep their content and order, and
`diff --git`, `old mode` / `new mode`, `new file mode` / `deleted file mode`,
`--- ` / `+++ ` and `GIT binary patch` all survive verbatim.

- `P` is the canonical patch's declared path set from
  `gitutil.FilesInPatchStrict` (`internal/gitutil/patch_paths_strict.go:253`),
  sorted byte-wise. **If `P` is empty, candidates are not comparable ⇒
  `ambiguous`.** Never broadened to "all paths".
- Measured: the same logical change on two branches with different preimage
  blobs gives **different** raw `-U3` bytes, **different** `-U0` bytes, and
  **identical** bytes once `^index ` lines are removed. Both `--unified=0` and
  the `index` strip are required.
- Measured: rule 1 alone is **not** enough. A healthy cherry-pick applied after
  five unrelated lines were prepended yields headers `@@ -10 +10 @@ l9` versus
  `@@ -15 +15 @@ l9`, so the identities **differ** — rev-3 rejected that
  history. Rewriting both headers to `@@` makes them **equal**. Hunk ranges and
  the function suffix are position metadata, not payload.
- Measured: the normalized form **preserves** `old mode`/`new mode`,
  `GIT binary patch`, and the `new file mode` / `deleted file mode` pair that
  `--no-renames` emits for a rename. A mode-only change normalizes to
  `diff --git` + `old mode` + `new mode` with no `@@` at all and stays distinct
  from any payload change.
- **The trade, stated precisely.** Discarding positions means two changes whose
  `-`/`+` **bodies are byte-identical** compare equal even at different line
  numbers. Measured: in a file with duplicate lines, changing the first `DUP`
  and changing the second `DUP` both normalize to `@@ / -DUP / +DUP CHANGED`.
  The collision therefore requires **duplicate line content at the changed
  lines**; whenever the removed or added text differs at all the bodies differ
  and the identities stay distinct — measured for a distinct payload, a
  mode-only change and a different path. The trade is accepted because the
  alternative rejects healthy cherry-picks, and because a collision yields
  `duplicate-equivalent` between commits that genuinely introduce the same
  text.
- **Cherry-picked and merged-back landings compare EQUAL** under this identity
  and are therefore `duplicate-equivalent`, not `ambiguous`.
- Candidates with 0 or ≥2 parents have no `<C>^` and are never compared; they
  are already excluded at step 1 and by the topology rules below.

**Topology, shallow history and partial clones.** A **preflight** runs once per
run, **strictly before any parent-count or topology classification** — the
ordering is normative, because a shallow boundary and a true root are
indistinguishable by `%P` alone: `git rev-parse
--is-shallow-repository`, `git rev-parse --show-object-format`, and the
`remote.<name>.promisor` / `partialclonefilter` config.

| Situation | State | Remediation |
|---|---|---|
| candidate has ≥2 parents | `unsupported-topology` | R9 — re-land on a linear commit |
| candidate has 0 parents **and** the repository is **not** shallow | `unsupported-topology` (genuine root landing) | R9 |
| candidate has 0 parents **and** the repository **is** shallow, or the candidate appears in `.git/shallow` | **`shallow-history`** | **R21 — `git fetch --unshallow`** |
| a needed object is missing locally — detected deterministically because every object and materialization command runs under `GIT_NO_LAZY_FETCH=1` (§3.6.5) | **`history-incomplete`** | **R22 — restore network access / `git fetch --refetch`, then re-run** |

Measured: in a `--depth 2` clone the boundary commit reports
`parents_in_%P = 0` **exactly like a real root** and is marked `(grafted)`, and
`read-tree <boundary>^` fails with the *same* `fatal: Not a valid object name`
text as a true root. Without the preflight the two are indistinguishable and
the operator is told to re-land when the fix is to deepen the clone. A **CI
shallow checkout** is the common case and gets R21, not R9. A partial
(blobless) clone reports `is-shallow=false`, `promisor=true`, and has commits
and trees locally. Because every command carries `GIT_NO_LAZY_FETCH=1`, a
missing object fails **locally and immediately** — measured
`fatal: Not a valid object name <sha>` — instead of the network error the
default produces, and classifies as `history-incomplete`.

**Scope of what is proven, without overclaim.** The *mechanism* is measured:
with a promisor remote configured and its object physically removed, the
default run **attempts the network** while the same run under
`GIT_NO_LAZY_FETCH=1` fails locally. The **end-to-end partial-clone path is
not** — a `file://` `--filter=blob:none` clone on git 2.55.0 did **not**
withhold blobs, so no genuinely missing promisor object could be produced that
way. Producing one requires a **non-local transport** (HTTP, SSH or
`git daemon`) with `uploadpack.allowFilter=true`, or a deterministic promisor
fixture (`extensions.partialclone` + a dead promisor URL + object deletion).
`history-incomplete` is therefore a **specified behaviour with a proven
mechanism and an unproven end-to-end path**; **AC-L68 and AC-L69 are Wave C
acceptance gates** — if the environment cannot reproduce a real filtered
remote, Wave C **cannot claim completion** and must report a blocker rather
than mark those rows passed.

**Rebase / cherry-pick / branch switch / detached HEAD / rewrite — total.**
Trailers survive rebase and cherry-pick verbatim while SHA and parent change,
so evidence keys on trailer *values*; both classify `exact`, possibly with
`base_commit_reachable: false` (advisory). A branch switch removing the landing
yields `none` ⇒ forward mode. A detached `HEAD` is evaluated identically from
whatever `HEAD` resolves to. A rewrite leaving no reachable landing yields
`none`; one leaving two is decided by the rules above.

#### 3.6.9 Inventory, diagnostics, remediation and implementability (ADR-013 D17)

**Full repository metadata inventory, built from `ListFeatureEntries`.**
`loadLaterFeatureTouches` reads **every** feature
(`internal/workflow/writefile_safety.go:409-442`), so a closure-only snapshot
cannot make ADR-029 ordering deterministic.

**`ListFeatures` is not usable for this.** It **silently skips** any feature
whose `status.json` fails to load — `continue // skip features without valid
status.json` (`internal/store/store.go:210-236`, the skip at `:226`) — so an
unreadable feature cannot be represented at all, which is the same false-green
class this amendment exists to close. `ListFeatureEntries`
(`internal/store/store.go:274-348`) exists for exactly this purpose: it returns
`FeatureEntry{Slug, Status *FeatureStatus, Err error}`
(`internal/store/store.go:238-245`) per feature, **sorted by slug**
(`internal/store/store.go:344-346`), surfacing stat and decode failures as
error rows.

At the start of a run verify captures **once**:

- every `FeatureEntry` from `store.ListFeatureEntries()`, **in the returned
  slug-sorted order** — the deterministic enumeration order for the whole run;
- for every entry with `Err == nil`: the decoded `FeatureStatus` including
  `RequestedAt`; the **presence state** (§3.6.2) and **raw bytes** of
  `artifacts/apply-recipe.json`, `artifacts/post-apply.patch` and
  `artifacts/recipe-provenance.json`; and the decoded
  `patch-generations.json` `touched_paths` union
  (`internal/store/patch_generations.go:52`);
- for every entry with `Err != nil`: the slug and the error, retained as an
  explicit **`unreadable`** row — **never omitted**;
- the §3.6.8 preflight facts and the object format.

Every later stage — evidence classification, V7, V8, V10, ADR-029 later-touch
ordering, the persisted `VerifyRecord` and the derived labels — consumes
**copies** from this one inventory and never re-reads disk.

**Read-error policy, explicit, no silent skip.**

| Unreadable entry is… | Effect |
|---|---|
| the **target**, or any **closure member** | **block** — `failed_at: "inventory-unreadable"` naming the slug and the underlying error |
| any **other** feature | a **`warn` `inventory-unreadable` advisory** naming the slug, **and** exclusion from ADR-029 later-touch ordering — matching the shipped detector, which already skips features whose `RequestedAt` is empty, but now **reported** rather than invisible |

`ListFeatureEntries` itself errors when `features/` is missing while `.tpatch/`
exists (`internal/store/store.go:294-302`); verify surfaces that as a block
rather than an empty inventory.

**Instability detection.** Before the report is finalised,
`ListFeatureEntries()` is re-run and every captured artifact re-read and
compared. A feature **added**, **removed**, or **changed** — including a slug
flipping between an `Err` row and a `Status` row — ⇒ **FAIL** `failed_at:
"snapshot-unstable"` naming the slug and path.

**Cost.** One `ListFeatureEntries` call plus O(features) artifact reads, once:
`F` status decodes and up to `3F` artifact reads, against `K` before. Bounded,
sequential, reused by `verify --all`, and spawning no git process.

**Read-only guarantees.** No worktree mutation, no real-index mutation, no
`status.json` write beyond the existing `Verify` record — and none at all under
`--no-write` (`internal/workflow/verify.go:310-314`). Measured for the
anchor-C and anchor-qualification probes: the real index is byte-identical, the
worktree is byte-identical, `git status --porcelain` is unchanged, and the temp
index never appears as an untracked entry. The shadow is pruned via the
existing deferred call (`internal/workflow/verify.go:1036-1040`).

**Remediation must never route a just-landed local feature to `reconcile`.**
The current V8 text is `post-apply.patch no longer applies to
closure-replayed baseline; run tpatch reconcile <slug>`
(`internal/workflow/verify.go:1167`). The forward-mode string is **unchanged**.

Exact strings (Wave C emits these verbatim):

| # | Condition | Check | Exact remediation |
|---|---|---|---|
| R1 | anchor-C ladder blocks (step 2 failed) | V8 | `landed feature: post-apply.patch postimage is not present at HEAD; landing commit <sha> is reachable but the content is absent — inspect with git diff <sha> HEAD, then re-record and re-land. Do NOT run tpatch reconcile: this is local drift, not upstream drift` |
| R2 | anchor-C ladder blocks on a reduced-context hunk | V8 | `landed feature: post-apply.patch matched at HEAD only with all context discarded at <path>; verify refuses to certify an unanchored match — inspect with git diff <sha> HEAD -- <path>, then re-record so the captured context matches HEAD and re-land` |
| R3 | anchor-C step 1 fails, step 2 passes with no reduced-context hunk | V8 (warn) | `landed feature: post-apply.patch content is present at HEAD but its recorded context has drifted at <path>; a later change touched the surrounding lines — inspect with git diff <sha> HEAD -- <path> and re-record if the feature should absorb it` |
| R4 | anchor-H V7 replay fails | V7 | `landed feature: recipe op #<n> failed to replay at the landing baseline <sha>: <err>; the recipe no longer describes the tree it was authored against — re-run tpatch record <slug> --regenerate-recipe and re-land` |
| R5 | anchor-H V8 forward check fails | V8 | `landed feature: post-apply.patch does not apply at the landing baseline <sha>; the patch and the landing attestation disagree — re-record and re-land` |
| R6 | evidence `stale` | V7 | `landing evidence for <slug> is stale: commit <sha> attests patch-sha=<sha> / recipe-sha=<sha> / base=<sha> but the current artifacts hash differently; re-run tpatch land <slug> to re-attest, or restore the attested artifacts` |
| R7 | evidence `ambiguous` | V7 | `landing evidence for <slug> is ambiguous: <n> reachable commits carry matching trailers with non-equivalent normalized changes (<sha>, <sha>, …); resolve the history or re-land so exactly one attestation is current` |
| R8 | evidence `malformed` | V7 | `landing evidence for <slug> is malformed: commit <sha> carries a Tpatch-Feature line that Git does not parse as a trailer, or a duplicated/ill-formed Tpatch-* value; restore the four-trailer block with git commit --amend, or re-land` |
| R9 | evidence `unsupported-topology` | V7 | `landing evidence for <slug> is unusable: commit <sha> has <n> parents and tpatch land emits single-parent commits; verify cannot derive a landing baseline from a root or merge commit — re-land <slug> on a linear commit` |
| R10 | evidence `unavailable` | V7 | `landing evidence for <slug> could not be read: <err>; verify requires git >= 2.36 (trailer enumeration >= 2.22/2.25, object-format probe >= 2.29, and GIT_NO_LAZY_FETCH >= 2.36 for offline object access) and refuses to guess — upgrade git to 2.36 or newer, or report this environment` |
| R11 | anchor H unavailable (terminal) | V7 | `landed feature <slug> has no usable landing baseline: no reachable single-parent landing commit has a parent that the current canonical patch applies to, or the qualifying candidates describe different changes; verify will not certify a landed feature it cannot replay — re-run tpatch record <slug> and tpatch land <slug> to create a fresh single-parent landing` |
| R12 | V10 preimage mismatch at the member's reference tree | V10 | `recipe op #<n> <path> expected preimage <sha> at baseline <sha> but observed <sha>; the recipe is stale against its own baseline — re-run tpatch record <slug> --regenerate-recipe and re-land` |
| R13 | V10 later-touch (metadata) | V10 (warn) | `later-touch: later feature <slug> touched <path> after <slug> was recorded; replaying this write-file would silently revert it — review before any replay (ADR-029 D5/D6, warning-class)` |
| R14 | parent landed, ladder blocks at the anchor | V7 | `hard parent <slug> landed at <sha> but its canonical patch is not present at the verification baseline; verify <slug> first — do not re-apply it into the shadow` |
| R15 | parent evidence integrity | V7 | `hard parent <slug> has <state> landing evidence; verify <slug> first — replaying or skipping it would validate <target> against an unknown baseline` |
| R16 | parent `unapplied` | V7 | `hard parent <slug> is unapplied; its patch is deliberately absent from the tree — run tpatch apply <slug> before verifying <target>` |
| R17 | parent `rejected` | V7 | `hard parent <slug> is rejected (terminal); remove the hard dependency with tpatch amend <target> --remove-depends-on <slug>, or reopen <slug>` |
| R18 | parent `evidence none` but already present | V7 (warn) | `unattributed-materialized: hard parent <slug> is not landed but its canonical patch is already present at the verification baseline; it was not replayed, and verify makes no claim about what produced it` |
| R19 | both artifacts absent or empty on a landed member | V7 | `landed feature <slug> has no usable apply-recipe.json or post-apply.patch; materialization cannot be proven from an absent or empty artifact set — re-run tpatch record <slug>` |
| R20 | inventory instability | V7 | `verify aborted: <slug>/<path> changed while verify was running; re-run tpatch verify <slug> with no concurrent tpatch or editor writes` |
| R21 | evidence `shallow-history` | V7 | `landing evidence for <slug> is incomplete: this is a shallow clone and commit <sha> sits on the graft boundary, so its parent is not available locally — run git fetch --unshallow (or increase --depth) and re-run verify` |
| R22 | evidence `history-incomplete` | V7 | `landing evidence for <slug> could not be completed: an object required to read the landing baseline is missing from this partial clone — restore network access to the promisor remote, or run git fetch --refetch, and re-run verify` |
| R24 | provenance required but unusable | V10 | `recipe op #<n> <path> carries a preimage_hash but artifacts/recipe-provenance.json is <absent|malformed|unreachable>; verify will not evaluate a preimage against the live working tree — re-run tpatch implement <slug> to regenerate the recipe and its provenance` |

`R23` is reserved for the `land`-side producer refusal and is specified in
`PRD-tpatch-land` §3.8.6.

Human report gains two lines above the check list:

```text
verify extra-button — passed
  baseline: historical-anchor @ 6316e46 (replay anchor 54b405d) · current @ 9f2c1ab (isolated index)
  landing evidence: exact @ 71a0c9d (patch ✓ recipe ✓ base ✓)
  ✓ [block] recipe_replay_clean — replayed at landing baseline
  ✓ [block] post_apply_patch_replay_clean — coherent at baseline; materialized at HEAD
  …
  ✓ [block] write_file_preimage_fresh — preimages fresh at their own baselines
```

**Sticky `verify-failed` clearing is mode-agnostic.** No new freshness label;
the four §3.4.2 labels are unchanged and mutually exclusive. A passing landed
run persists the same `VerifyRecord` field set as a passing forward run
(`internal/store/types.go:290-296`), so the read-time derivation takes no mode
input and a feature stuck at `verify-failed` from a pre-fix false red clears on
the first passing run.

**Honest invocation budget** — rev-2's table omitted tree materialization and
per-member anchor resolution:

| Purpose | Invocations |
|---|---|
| Repository preflight (`--is-shallow-repository`, `--show-object-format`, promisor config) | **3 per run**, cached; runs **before** any topology classification |
| Inventory build (`store.ListFeatureEntries` + artifact reads) | **1 `ListFeatureEntries` per run**; `F` status decodes, up to `3F` artifact reads; cached across `verify --all`; **no git process** |
| Evidence enumeration (`git log --topo-order --reverse -z --format=…`) | **1 per run**, cached across `verify --all`. **No `rev-list`.** |
| Shadow allocation at anchor H | 1 `CreateShadow` per anchored member whose recipe must replay |
| Tree materialization for a candidate parent | 1 `git read-tree C^` **per distinct parent commit** — the parent id is already in the cached `%P`, so no extra `rev-parse` is needed |
| Anchor qualification | 1 **forward** `git apply --check --cached -C1` **per collected candidate**, memoised per `(tree, patch)` |
| Normalized identity | 1 `git diff --unified=0 …` **per qualifying candidate, only when ≥2 qualify**; post-processing is in-process |
| Anchor-C ladder | 1 `apply --check --reverse --cached`, plus 1 `-C0 --verbose` on failure, per `(tree, patch)`, memoised |
| Per-member anchor resolution | the collect/qualify/compare loop **per landed closure member**, sharing the one enumeration and the tree/apply memo |
| Reachability (`base_commit`, provenance) | 1 `git merge-base --is-ancestor` per checked commit, memoised |
| Offline discipline | **every** git invocation above carries `GIT_NO_LAZY_FETCH=1`; none of them may reach the network |

New code is one generic reader in `internal/gitutil/` (candidate
`trailers.go`) returning raw **and** parsed records, plus a temp-index helper;
**policy** stays in `internal/workflow/verify.go` per ADR-013 D7. Everything
else reuses shipped primitives. **No new store field, no new artifact, no
schema migration, no new dependency, no new check ID.**

**Git floor is 2.36, checked before anything else.** The **effective** floor
is set by the strictest mandatory capability:

| Capability | Introduced | Used by |
|---|---|---|
| `%(trailers:key=…,valueonly)` | 2.22 | §3.6.2 enumeration |
| `…,separator=` | 2.25 | §3.6.2 enumeration |
| `git rev-parse --show-object-format` | 2.29 | §3.6.2 / §3.8.6 derived commit-id length |
| **`GIT_NO_LAZY_FETCH`** | **2.36** | **§3.6.5 offline discipline — mandatory on every object command** |

The first three rows are **historical component facts only**. Because
`GIT_NO_LAZY_FETCH` is *mandatory*, a git that lacks it cannot run this
contract offline at all, so the **overall preflight requires ≥ 2.36** and runs
**before any evidence, object, `log`, `read-tree`, `apply` or `diff`
command**. Below the floor the run reports evidence `unavailable` with **R10**
and issues **no** object or log command and **no** network call — only the
`git --version` probe. Verified on git 2.55.0; the below-floor gate is proven
with a `PATH` shim reporting `git version 2.30.2`, after which the only
recorded subcommand is `--version`.

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
#### 4.3.6 LANDED-PASS — dual-anchor verification green (v0.15.1 Wave B / GH #8, rev-7)

`schema_version` moves `"1.0"` → `"1.1"` (`internal/workflow/verify.go:83`).
The guarantee is **additive semantic compatibility, not byte identity**:
`baseline`, `landing_evidence` and `target_mode` are emitted for *every*
feature. Consumers refuse unknown **majors** (§4.3), so 1.1 is non-breaking.
The `checks` array is **eleven** rows in V0–V10 order in every shape below.

> **`freshness_label` is not a verify-report field.** The shipped
> `VerifyReport` (`internal/workflow/verify.go:139-166`) has no such member;
> the derived label belongs to `tpatch status --json` (§4.3.2, §4.3.3, §4.5).
> Every **verify** sample omits it (Q16).

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verified_at": "2026-08-12T18:30:11Z",
  "verdict": "passed",
  "exit_code": 0,
  "repository": {
    "object_format": "sha1",
    "commit_id_hex_len": 40,
    "shallow": false,
    "partial_clone": false
  },
  "baseline": {
    "mode": "dual-anchor",
    "current_commit": "9f2c1ab4…",
    "current_probe": "isolated-index",
    "historical_anchor": {
      "state": "available",
      "commit": "6316e465…",
      "replay_anchor_commit": "54b405df…",
      "candidates_collected": 2,
      "candidates_qualified": 1
    }
  },
  "landing_evidence": {
    "state": "exact",
    "attestation_commit": "71a0c9d3…",
    "candidates": 1,
    "parent_count": 1,
    "patch_presence": "present-nonempty",
    "recipe_presence": "present-nonempty-with-ops",
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
    { "id": "write_file_preimage_fresh",     "severity": "block",       "passed": true, "mode": "historical-anchor",
      "member_baselines": { "extra-button": "6316e465…", "button-component": "0a91be22…" } }
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
| `repository.object_format` | string | `git rev-parse --show-object-format` |
| `repository.commit_id_hex_len` | int | 40 for `sha1`, 64 for `sha256` — the **derived** `Tpatch-Base-Commit` length rule (§3.6.2) |
| `repository.shallow` / `partial_clone` | bool | the §3.6.8 preflight facts |
| `baseline.mode` | string | `"head-anchored"` (forward) \| `"dual-anchor"` (landed) |
| `baseline.current_commit` | string | the resolved `HEAD` — anchor C's tree, and the shadow root in forward mode |
| `baseline.current_probe` | string | always `"isolated-index"` — anchor C read a temp index seeded by `read-tree`, never the worktree or the real index |
| `baseline.historical_anchor.state` | string | `"available"` \| `"unavailable"` \| `"not-applicable"` |
| `baseline.historical_anchor.commit` | string | the anchor **tree's** commit — the replay anchor's single parent |
| `baseline.historical_anchor.replay_anchor_commit` | string | the selected replay anchor, which **may differ** from the attestation commit |
| `baseline.historical_anchor.candidates_collected` / `candidates_qualified` | int | the D14 collect-then-qualify counts; `qualified > 1` implies the identities were compared and found equal |
| `baseline.historical_anchor.reason` | string | why unavailable; omitted otherwise |
| `landing_evidence.state` | string | closed set of ten — `none` \| `exact` \| `duplicate-equivalent` \| `stale` \| `ambiguous` \| `malformed` \| `unsupported-topology` \| `shallow-history` \| `history-incomplete` \| `unavailable` |
| `landing_evidence.attestation_commit` | string | the **authority** commit; omitted when `state` is `none` or `unavailable` |
| `landing_evidence.candidates` | int | attestation candidates examined |
| `landing_evidence.duplicates` | int | equivalent attestations; omitted when < 2 |
| `landing_evidence.parent_count` | int | `%P` cardinality of the attestation commit |
| `landing_evidence.patch_presence` | string | `absent` \| `present-empty` \| `present-nonempty` (§3.6.2) |
| `landing_evidence.recipe_presence` | string | `absent` \| `present-empty` \| `present-nonempty-zero-op` \| `present-nonempty-with-ops` |
| `landing_evidence.patch_sha_match` / `recipe_sha_match` / `base_commit_match` | bool | digest/value comparisons; **omitted entirely** when the corresponding presence state is `absent` or `present-empty` |
| `landing_evidence.base_commit_reachable` | bool | advisory only |
| `landing_evidence.reason` | string | classification detail for non-`exact` states |
| `target_mode` | string | `"forward"` \| `"landed"` |
| `checks[].mode` | string | `"forward"` \| `"historical-anchor"` \| `"current-anchor"` \| `"dual-anchor"` \| `"provenance-anchor"` — **present on V7, V8 and V10 in every report, including when skipped or failed**; **absent on V0–V6 and V9** |
| `checks[].anchor_results` | object | V8 only: `{"historical": "passed"\|"failed"\|"skipped", "current": "materialized-clean"\|"materialized-context-drift"\|"absent"\|"skipped"}` |
| `checks[].member_baselines` | object | V10 only: `slug → baseline commit`, one entry per evaluated member, proving each landed member used **its own** anchor (§3.6.7) |
| `checks[].provenance_hash_bound` | bool | V10 only, forward/provenance mode: `false` when the sidecar predates `recipe_sha256` |
| `advisories` | array | `{ "code", "severity": "warn", "slug", "path", "message" }` |

**Mode presence rule.** V7, V8 and V10 always carry `mode`, even when
`skipped` or `failed`. V0–V6 and V9 never carry it. Consumers may rely on both
halves.

#### 4.3.7 LANDED with advisories — context drift, metadata later-touch, unattributed parent

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "passed",
  "exit_code": 0,
  "repository": { "object_format": "sha1", "commit_id_hex_len": 40, "shallow": false, "partial_clone": false },
  "baseline": {
    "mode": "dual-anchor", "current_commit": "9f2c1ab4…", "current_probe": "isolated-index",
    "historical_anchor": { "state": "available", "commit": "6316e465…", "replay_anchor_commit": "54b405df…",
                           "candidates_collected": 3, "candidates_qualified": 2 }
  },
  "landing_evidence": { "state": "duplicate-equivalent", "attestation_commit": "71a0c9d3…", "candidates": 2,
                        "duplicates": 2, "parent_count": 1,
                        "patch_presence": "present-nonempty", "recipe_presence": "present-nonempty-with-ops",
                        "patch_sha_match": true, "recipe_sha_match": true,
                        "base_commit_match": true, "base_commit_reachable": true },
  "target_mode": "landed",
  "checks": [
    { "id": "recipe_replay_clean",           "severity": "block", "passed": true, "mode": "historical-anchor" },
    { "id": "post_apply_patch_replay_clean", "severity": "block", "passed": true, "mode": "dual-anchor",
      "anchor_results": { "historical": "passed", "current": "materialized-context-drift" } },
    { "id": "write_file_preimage_fresh",     "severity": "block", "passed": true, "mode": "historical-anchor",
      "member_baselines": { "extra-button": "6316e465…", "button-component": "0a91be22…" } }
  ],
  "advisories": [
    { "code": "context-drift", "severity": "warn", "slug": "extra-button", "path": "src/extras/button.css",
      "message": "landed feature: post-apply.patch content is present at HEAD but its recorded context has drifted at src/extras/button.css; a later change touched the surrounding lines — inspect with git diff 71a0c9d3 HEAD -- src/extras/button.css and re-record if the feature should absorb it" },
    { "code": "later-touch", "severity": "warn", "slug": "extra-button", "path": "src/extras/button.css",
      "message": "later-touch: later feature dark-mode touched src/extras/button.css after extra-button was recorded; replaying this write-file would silently revert it — review before any replay (ADR-029 D5/D6, warning-class)" },
    { "code": "unattributed-materialized", "severity": "warn", "slug": "shared-tokens",
      "message": "unattributed-materialized: hard parent shared-tokens is not landed but its canonical patch is already present at the verification baseline; it was not replayed, and verify makes no claim about what produced it" }
  ],
  "lifecycle_state": "applied"
}
```

`checks` is abridged to the three anchored rows; the emitted array is always
eleven. **The verdict is `passed`**: `context-drift` means step 2 of the ladder
passed with **zero** reduced-context hunks, `later-touch` is warning-class per
ADR-029 D6, and `unattributed-materialized` is a no-ownership note. None flips
`passed`. `candidates_qualified: 2` records that two replay anchors qualified
and their normalized identities compared **equal** before selection.

#### 4.3.8 LANDED-CONTENT-ABSENT — the mandatory reduced-context block

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "failed",
  "exit_code": 2,
  "failed_at": "landed-content-absent",
  "repository": { "object_format": "sha1", "commit_id_hex_len": 40, "shallow": false, "partial_clone": false },
  "baseline": {
    "mode": "dual-anchor", "current_commit": "9f2c1ab4…", "current_probe": "isolated-index",
    "historical_anchor": { "state": "available", "commit": "6316e465…", "replay_anchor_commit": "54b405df…",
                           "candidates_collected": 1, "candidates_qualified": 1 }
  },
  "landing_evidence": { "state": "exact", "attestation_commit": "71a0c9d3…", "candidates": 1, "parent_count": 1,
                        "patch_presence": "present-nonempty", "recipe_presence": "present-nonempty-with-ops",
                        "patch_sha_match": true, "recipe_sha_match": true,
                        "base_commit_match": true, "base_commit_reachable": true },
  "target_mode": "landed",
  "checks": [
    { "id": "recipe_replay_clean",           "severity": "block", "passed": true,  "mode": "historical-anchor" },
    { "id": "post_apply_patch_replay_clean", "severity": "block", "passed": false, "mode": "dual-anchor",
      "anchor_results": { "historical": "passed", "current": "absent" },
      "remediation": "landed feature: post-apply.patch matched at HEAD only with all context discarded at src/extras/button.css; verify refuses to certify an unanchored match — inspect with git diff 71a0c9d3 HEAD -- src/extras/button.css, then re-record so the captured context matches HEAD and re-land" },
    { "id": "write_file_preimage_fresh",     "severity": "block", "passed": true,  "mode": "historical-anchor",
      "member_baselines": { "extra-button": "6316e465…" } }
  ],
  "lifecycle_state": "applied"
}
```

V7 **passes**: the recipe is still coherent at the landing baseline. The
failure is purely "it is no longer anchored in the tree", which only anchor C
can see. The string shown is R2 (reduced-context); R1 is emitted when step 2
fails outright.

#### 4.3.9 TERMINAL states — evidence integrity, unavailable anchor, provenance, shallow

Evidence `stale` / `ambiguous` / `malformed` / `unsupported-topology` /
`shallow-history` / `history-incomplete` / `unavailable` all carry
`failed_at: "landing-evidence"`. A missing replay anchor carries
`historical-anchor-unavailable`. An unusable recipe provenance carries
`recipe-provenance-unavailable`. In **every** terminal family V7, V8 and V10
report `passed: false` with `mode` present — **failed-because-unanchored**, not
skipped — and the run **never** passes on anchor C alone.

```json
{
  "schema_version": "1.1",
  "slug": "extra-button",
  "verdict": "failed",
  "exit_code": 2,
  "failed_at": "historical-anchor-unavailable",
  "repository": { "object_format": "sha1", "commit_id_hex_len": 40, "shallow": true, "partial_clone": false },
  "baseline": {
    "mode": "dual-anchor", "current_commit": "9f2c1ab4…", "current_probe": "isolated-index",
    "historical_anchor": {
      "state": "unavailable",
      "candidates_collected": 2,
      "candidates_qualified": 0,
      "reason": "no collected candidate's parent tree accepts a forward apply of the current canonical patch"
    }
  },
  "landing_evidence": { "state": "exact", "attestation_commit": "69c70bf7…", "candidates": 2, "parent_count": 1,
                        "patch_presence": "present-nonempty", "recipe_presence": "present-nonempty-with-ops",
                        "patch_sha_match": true, "recipe_sha_match": true,
                        "base_commit_match": true, "base_commit_reachable": true },
  "target_mode": "landed",
  "checks": [
    { "id": "recipe_replay_clean",           "severity": "block", "passed": false, "mode": "historical-anchor",
      "remediation": "landed feature extra-button has no usable landing baseline: no reachable single-parent landing commit has a parent that the current canonical patch applies to, or the qualifying candidates describe different changes; verify will not certify a landed feature it cannot replay — re-run tpatch record extra-button and tpatch land extra-button to create a fresh single-parent landing" },
    { "id": "post_apply_patch_replay_clean", "severity": "block", "passed": false, "mode": "dual-anchor",
      "anchor_results": { "historical": "failed", "current": "materialized-clean" },
      "remediation": "landed feature extra-button has no usable landing baseline: the historical half of this check could not run; see recipe_replay_clean" },
    { "id": "write_file_preimage_fresh",     "severity": "block", "passed": false, "mode": "historical-anchor",
      "remediation": "landed feature extra-button has no usable landing baseline: preimage freshness cannot be evaluated without one; see recipe_replay_clean" }
  ],
  "lifecycle_state": "applied"
}
```

`anchor_results.current` is still `materialized-clean` — the content **is**
present at HEAD — and the run still **fails**. That is the point: an
unverifiable historical half is an unverified feature.

**Unified `failed_at` vocabulary — closed set of thirteen.** Wave C must not
emit any other value; AC-L114 pins it.

| Value | Meaning |
|---|---|
| `parent-replay` | existing — a closure member failed to replay |
| `landing-evidence` | `stale` \| `ambiguous` \| `malformed` \| `unsupported-topology` \| `shallow-history` \| `history-incomplete` \| `unavailable` |
| `historical-anchor-unavailable` | no collected candidate qualifies, or qualifiers are non-equivalent (§3.6.8) |
| `recipe-provenance-unavailable` | a `write-file` op carries `preimage_hash` but its provenance is absent, ill-formed or unreachable (§3.6.7) |
| `landed-content-absent` | the anchor-C ladder blocked (step 2 failed, or ≥1 reduced-context hunk) |
| `landed-artifacts-absent` | a landed member has no usable artifact per the §3.6.2 presence states |
| `landed-baseline-incoherent` | anchor-H V7 or V8 forward check failed |
| `parent-landing-drift` | a landed member's patch ladder blocked at its anchor, or its V10 block-class outcome fired |
| `parent-evidence-integrity` | a member's evidence is any terminal non-`exact` state |
| `parent-unapplied` | a hard parent is `unapplied` |
| `parent-rejected` | a hard parent is `rejected` |
| `snapshot-unstable` | the inventory changed while verify was running (§3.6.9) |
| `inventory-unreadable` | the target's or a closure member's artifacts could not be read (§3.6.9) |

**Advisory `code` vocabulary — closed set of five**, all `warn`, none of which
flips `passed`: `context-drift`, `later-touch`, `unattributed-materialized`,
`base-commit-unreachable`, `provenance-unreachable`.

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
**Landed-feature rows (v0.15.1 Wave B / GH #8 rev-7 — see §3.6 for the contract).**

Artifact rows are stated against a fact `land` enforces: it refuses when the
embedded `record` would capture nothing, so a landed feature with an absent or
zero-byte `post-apply.patch` is a **corruption or hand-edit** case.

| Case | Handling |
|------|----------|
| **Anchor C isolation** | |
| Landed target, worktree dirty (feature reverted in the worktree only) | Anchor C reads a temp index seeded from `HEAD` ⇒ **PASS**. Measured: the rev-1 worktree-based check false-reds here. |
| Landed target, index dirty (unrelated paths staged) | Same — the real index is never read. |
| Worktree contains the feature but `HEAD` does not | Anchor C blocks on the `HEAD` tree, correctly. |
| Any landed run | Real index byte-identical, worktree byte-identical, `git status` unchanged, temp index invisible and removed on every exit path. |
| **Anchor C ladder** | |
| Later unrelated edit **far** from any hunk, or pure offset shift | Step 1 passes ⇒ **PASS, no advisory**. |
| Later unrelated edit **2 lines** from a hunk | Step 1 fails, step 2 passes with **zero** `(0/0)` ⇒ **PASS + `context-drift` warn** (R3). |
| Later unrelated edit **1 line** from a hunk | Step 2 passes but reports `(0/0)` ⇒ **BLOCK** (R2). Accepted false red; measured 26/151. |
| Revert-in-place + identical postimage text elsewhere in the file | Step 2 passes but reports `(0/0)` ⇒ **BLOCK**. Closes the rev-1 false green (2/69). |
| Partial revert — hunk 1, hunk 2, hunk 3, or hunks 1+3 | Both steps fail ⇒ **BLOCK** (R1). Four shapes measured. |
| Full revert; a patched file deleted | Both steps fail ⇒ **BLOCK** (R1). |
| Degenerate whole-file hunk with header/footer added | Step 2 passes with zero `(0/0)` ⇒ **PASS + `context-drift`**. |
| **Anchor collection and qualification** | |
| One landing, ordinary single-parent | Collected, forward-qualifies at its parent tree, selected. |
| Re-record + re-land, two landings | **Both** collected. The newest landing's parent already contains the feature so it **does not** forward-qualify; the earlier one does. Attestation = newest, replay anchor = earlier. Measured. |
| Two qualifying candidates with **equal** normalized identity (cherry-pick or merge-back) | Both collected and compared; the topo-oldest is selected. `candidates_qualified: 2`. |
| Two qualifying candidates with **differing** normalized identity | **FAIL `historical-anchor-unavailable`** (R11). The rev-2 stop-at-first rule could never observe the second qualifier. |
| Candidate parent carries unrelated drift inside the patch's context window | Does **not** forward-qualify — honestly disqualified, because a replay there would not be meaningful. Measured. |
| No candidate qualifies | **FAIL `historical-anchor-unavailable`** (R11); V7/V8-historical/V10 all `passed: false` with `mode`; the run never passes on anchor C alone. |
| **Topology, shallow, partial** | |
| Root landing (0 parents) in a **non-shallow** repository | `unsupported-topology` ⇒ FAIL with R9. |
| Merge commit carrying the trailer (≥2 parents) | `unsupported-topology` ⇒ FAIL with R9. No `^1` approximation. |
| **Shallow clone**, candidate on the graft boundary (0 parents in `%P`) | **`shallow-history`** ⇒ FAIL with **R21 (`git fetch --unshallow`)**. Measured: a `--depth 2` boundary reports 0 parents exactly like a root, and `read-tree <boundary>^` fails with the same text. |
| **CI shallow checkout** (`--depth 1`) verifying a landed feature | `shallow-history` + R21 — not R9. This is the common real-world case. |
| Partial (blobless) clone, objects available locally | Works: `is-shallow=false`, commits and trees local. |
| Partial clone, promisor fetch fails or an object is missing | **`history-incomplete`** ⇒ FAIL with **R22**. |
| SHA-256 repository | `Tpatch-Base-Commit` must be **64** lowercase hex; the length is derived from `git rev-parse --show-object-format`. Measured. A hardcoded 40 would reject every valid landing. |
| **Recipe op shapes** | |
| Landed target, `write-file` / `replace-in-file` / `append-file` recipe | V7 **replays** at anchor H ⇒ ✓. Today `write-file` passes vacuously, `replace-in-file` false-fails, `append-file` double-appends. |
| Any op-kind predicate | **Diagnostic only** — never certifies, never causes a skip. `replace-in-file` with empty replacement is undecidable; `append-file` with empty content is undecidable; `write-file` never certifies by whole-file equality. |
| **Artifact presence (closed, mutually exclusive)** | |
| Patch **absent** | **Terminal `landed-artifacts-absent`** (R19), reached **before** any digest comparison. Never a mismatch. `patch_sha_match` is omitted from the report. Recipe shape is irrelevant. |
| Patch **present-empty** (zero bytes) | **Terminal `landed-artifacts-absent`** (R19) by the same short-circuit — `land` refuses to produce it, so this is corruption or a hand edit. Recipe shape is irrelevant. |
| Patch **present-nonempty** | The **only** state in which the digest is compared, and the only one from which `exact` or `stale` is reachable. Recipe shape is evaluated only here. |
| Recipe **absent** (patch present-nonempty) | Expected trailer `none`; V7 skips; the patch is sole authority. |
| Recipe **present-empty** (zero bytes or whitespace-only; patch present-nonempty) | Expected trailer `none` (`readRecipeSHA`, `internal/cli/land.go:1039-1041`); V2 fails to parse ⇒ V7/V10 skip; the patch is sole authority. |
| Recipe **present-nonempty-zero-op** (patch present-nonempty) | Parses, no operations ⇒ V7 has nothing to replay and **cannot attest**; the V7 row records `0 op(s)` rather than a vacuous pass; the patch is sole authority. |
| **Evidence** | |
| Hand-rolled `git commit` with no trailers | `none` ⇒ forward mode ⇒ today's behavior. |
| Commit whose **prose body** quotes `Tpatch-Feature: <slug>` | **`malformed`** ⇒ FAIL. A deliberate, documented false red — indistinguishable from an amend-destroyed block. |
| Trailer block destroyed by a later `--amend` | **`malformed`** ⇒ FAIL — the case the rule protects. |
| Duplicate `Tpatch-Patch-SHA` / `Recipe-SHA` / `Base-Commit`; ≥2 `Tpatch-Feature` values | **`malformed`** ⇒ FAIL. No "take the first". |
| Uppercase hex, wrong length for the object format, non-`none` non-hex `Recipe-SHA` | **`malformed`** ⇒ FAIL. |
| Lowercase trailer key (`tpatch-feature:`) | Git matches keys case-insensitively; the commit **is** a candidate. |
| Two attestations with equal normalized identity | **`duplicate-equivalent`**, `duplicates: 2`. |
| Two attestations with differing identity | **`ambiguous`** ⇒ FAIL (R7). |
| Canonical patch declares **no** paths | Not comparable ⇒ **`ambiguous`**; never broadened. |
| Cherry-picked or rebased landing | `exact`; `base_commit_reachable` may be `false` (advisory only). |
| `git` below the floor, unreadable object format, or any reader error | **`unavailable`** ⇒ FAIL (R10). Never `none`. |
| **Parents** | |
| Landed hard parent, patch ladder clean at the anchor | Skipped — never replayed, so an `append-file` parent is not duplicated. |
| Landed hard parent, ladder blocks at the anchor | Fail-fast `parent-landing-drift` (R14) **before** the target is judged. |
| Landed hard parent, patch **absent** or **present-empty** | **FAIL `landed-artifacts-absent`** regardless of recipe shape — the recipe never rescues an absent-or-empty patch. |
| Landed hard parent, no usable artifact | **FAIL `landed-artifacts-absent`**. |
| Hard parent, `evidence none`, patch **ladder-passes** at the anchor | **Skipped** with a mandatory `unattributed-materialized` warn (R18). No ownership claimed. |
| Hard parent, `evidence none`, ladder blocks or no patch | **Replayed**, unchanged. |
| Hard parent in `active` | Treated **exactly as `applied`**. Today it fail-fasts through `default:`; this amendment widens the switch. |
| Hard parent in `unapplied` / `rejected` | Fail-fast with the named reason. |
| Hard parent in `upstream_merged` / superseded | Skipped, unchanged. |
| **Landed hard parent V10** | Evaluated at **that parent's own** replay-anchor parent tree — never the target's anchor, which may already contain the parent's postimage. `member_baselines` in the report proves it. |
| Landed parent V10 block-class outcome | Contributes to `parent-landing-drift` for that member. |
| Landed parent V10 warn-class later-touch | Aggregated into `advisories` under the member's slug; affects no verdict. |
| **V10 / provenance** | |
| Unlanded applied feature, op with `preimage_hash`, provenance present + well-formed + reachable + hash-bound | Evaluated at `RecipeProvenance.BaseCommit` ⇒ correct. **This fixture fails today**, because `checkWriteFilePreimage` reads the live working tree. |
| Same, provenance sidecar predates `recipe_sha256` | Accepted with `provenance_hash_bound: false` reported. |
| Same, provenance **absent / ill-formed / unreachable** | **FAIL `recipe-provenance-unavailable`** (R24). Never falls back to the live tree. |
| Op with **no** `preimage_hash` (the autogen shape) | ADR-029 D4 legacy path: pass, no re-warn. Autogen writes no provenance either, so the two are consistent. |
| Malformed `preimage_hash` per ADR-029 D1 | **FAIL** block regardless of later-touch state. |
| Path byte-changed at HEAD but **no later feature** touched it | **No** `later-touch` advisory — the signal is metadata, not bytes. |
| **Run-level** | |
| Concurrent `tpatch add` / `remove` / artifact edit during a run | **FAIL `snapshot-unstable`** (R20) naming the slug and path. Detected because the inventory covers **all** features, not just the closure. |
| Unreadable `status.json` on an **unrelated** feature | `warn` advisory; excluded from later-touch ordering (matching the shipped best-effort detector). |
| Unreadable artifacts on the **target or a closure member** | **FAIL `inventory-unreadable`**. |
| `--no-write` on any landed path | All checks run, nothing persists (`internal/workflow/verify.go:310-314`). |
| `verify --all` over a mixed landed/unlanded set | One evidence enumeration and one inventory total, reused; ladder and tree probes memoised; output ordering unchanged. |

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
signal earlier but cannot change any verdict, and this amendment adds
**exactly one** `land`-side behaviour change — the §3.8.6 / D19 Base-Commit
refusal — and no other (`PRD-tpatch-land` §6.2 AC-LD15, AC-LD21). Tracked in
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

### Q15 — Forward-mode V10 anchor — **RESOLVED: `RecipeProvenance.BaseCommit`**

**Decided (rev-3): resolved, not scoped out.** rev-2 declared this unfixable
"because an un-landed feature has no anchor". That rationale is **withdrawn**:
the anchor already exists and ships.

Measured: `RecipeProvenance` (`internal/workflow/implement.go:30-34`) records
`base_commit` — the `HEAD` the recipe was generated against — together with
`recipe_sha256`; `implement` writes `artifacts/recipe-provenance.json`
(`:220-238`) on the same code path that emits `preimage_hash`, while the
autogen path that omits `preimage_hash`
(`internal/workflow/recipe_autogen.go:114-118`) writes no provenance. The two
therefore travel together, and `warnRecipeStale`
(`internal/cli/cobra.go:1285-1315`) already consumes both, including the
nil-`recipe_sha256` tolerance for pre-v0.5.2 sidecars.

**Resolution.** §3.6.7 uses `RecipeProvenance.BaseCommit` as the V10 reference
tree for every un-landed member and for the target in forward mode, gated on
four conditions — present, well-formed for the repository object format,
reachable from `HEAD`, and inventory-consistent via `recipe_sha256` when that
field exists. When an op carries a real `preimage_hash` but provenance fails
any of the first three, verify **FAILS** with
`recipe-provenance-unavailable` (R24) and **never** reads the live working
tree, which is the measured false block. Ops without `preimage_hash` stay on
the ADR-029 D4 legacy path. Pinned by AC-L96 – AC-L101.

**What remains open** is narrower and is tracked as Q17: recipes that carry a
`preimage_hash` but were produced before the provenance sidecar existed now
fail closed rather than silently mis-verifying. That is the intended
direction, and the remediation (`tpatch implement <slug>`) is one command.

### Q16 — Should verify emit `freshness_label` in its `--json` report? — **RESOLVED: no**

**Decided (rev-2): no.** The shipped `VerifyReport`
(`internal/workflow/verify.go:139-166`) has no such member; the derived label
belongs to `tpatch status --json` (§4.3.2, §4.3.3, §4.5). rev-1's landed
samples and the pre-implementation §4.3.1/§4.3.4/§4.3.5 samples all carried
it, which would have led an implementer to add a field nobody decided to add.
Every **verify** sample in §4.3 now omits it. Adding it later would be a
deliberate schema change with its own row. Pinned by AC-L97.


### Q17 — Open (non-blocking): pre-provenance recipes that carry `preimage_hash`

**Open, non-blocking, tracked.** A recipe generated by a provider before the
`recipe-provenance.json` sidecar existed can carry a real `preimage_hash` with
no provenance. Under §3.6.7 that combination now **fails closed** with
`recipe-provenance-unavailable` rather than silently evaluating the preimage
against the live working tree — which is the measured false block it replaces.
The remediation is a single command (`tpatch implement <slug>` regenerates
both the recipe and its provenance), and the population is bounded to recipes
authored in that window. A migration that back-fills provenance from
`status.apply.base_commit` was considered and **not adopted**: that field is
owned by `record`/auto-base (ADR-016), moves under the operator, and is not
hash-bound to the recipe bytes, so back-filling would manufacture an
attestation nobody made. **This does not block acceptance**; it is recorded so
Wave C sizes the failure population rather than discovering it.

### Q18 — Hunk-position normalization — **RESOLVED: normalize, with a measured trade**

**Decided (rev-4): hunk ranges and the function-context suffix are
normalized.** rev-3 left them in place and called the residual conservative.
Measured, it is not: a healthy cherry-pick applied after five unrelated lines
were prepended yields `@@ -10 +10 @@ l9` versus `@@ -15 +15 @@ l9` and is
rejected outright. §3.6.8 therefore rewrites every hunk header to a bare `@@`.

**The trade is measured and bounded**: two changes whose `-`/`+` bodies are
byte-identical compare equal even at different line numbers, which requires
**duplicate line content at the changed lines** (measured with a file
containing repeated `DUP` lines). Whenever the removed or added text differs at
all, the bodies differ and the identities stay distinct — verified for a
distinct payload, a mode-only change and a different path. A collision yields
`duplicate-equivalent` between commits that genuinely introduce the same text,
which is the intended equivalence class. Pinned by AC-L133.

## 7. Acceptance criteria (combined verify + freshness ships when…)

- [ ] **ADR-013 merged** before any Wave 3 implementation slice lands.
- [ ] `go build ./...`, `go test ./...`, `gofmt -l .` all clean.
- [ ] `FeatureStatus.Verify *VerifyRecord` field present, `omitempty`-marshalled. v0.6.1 fixtures round-trip byte-identical.
- [ ] `FeatureState` enum is unchanged (no `StateTested`).
- [ ] `tpatch verify <slug>` runs the check sequence in order, with the severities documented in §3.1. *(Historical Wave 3 acceptance row, written before V10 shipped; the current sequence is eleven checks V0–V10 — `internal/workflow/verify.go:49-71`.)*
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
### 7.1 Acceptance matrix — landed-feature verification (v0.15.1 Wave B / GH #8, rev-7)

**Binding on the Wave C implementation dispatch.** **Tier** names where a row
is proven:

- **U** — unit test, pure function or abstraction, no repo. Covers the §3.6.6
  diagnostic predicates, the §3.6.2 grammar and classifier over fixture bytes,
  and anything expressed over the **inventory** or **evidence-reader**
  abstractions.
- **W** — workflow integration in `internal/workflow`, real temp Git repo +
  `store.Store`, calling `RunVerify` directly. Where a row must observe, count
  or perturb git behaviour it uses a **`PATH` git wrapper** — a test-only shim
  first on `PATH` that forwards to the real `git` and can log argv and
  environment, inject errors, or mutate files between calls (proven feasible).
  **No production seam, no build tag, no exported hook.**
- **C** — real-CLI test in `internal/cli`, end-to-end through the cobra
  surface.

`W+C` means both tiers.

#### Group A — the reported defect and the eleven-check schema

| # | Criterion | Tier |
|---|---|---|
| AC-L1 | The issue #8 sequence passes **before** `land`: exit 0, exactly **eleven** check rows in V0–V10 order. | C |
| AC-L2 | The same feature **after** `land` passes: `target_mode: "landed"`, `landing_evidence.state: "exact"`, `baseline.mode: "dual-anchor"`, eleven rows. | W+C |
| AC-L3 | The committed-range re-record is decided by the §3.6.2 values, and **both branches** asserted: byte-identical artifacts ⇒ `exact`, passes with no re-land; changed artifacts ⇒ `stale`, FAIL with R6, passes after the re-land. | C |
| AC-L4 | A landed **leaf** with no dependencies passes. | W+C |
| AC-L5 | Every report — forward or landed, pass or fail — emits exactly eleven `checks` rows whose `id` values equal the constants at `internal/workflow/verify.go:49-71`, in order. No shorter or differently-terminated sequence shape survives anywhere in this PRD, the ADR or the land PRD — enforced mechanically by **AC-L135 / §7.1.2 G1 and G2**. Golden assertion plus the docs guard. | W |
| AC-L6 | `--no-write` on every AC-L row leaves `.tpatch/`, the real index and the worktree byte-identical. | W+C |

#### Group B — anchor C isolation

| # | Criterion | Tier |
|---|---|---|
| AC-L7 | Every anchor-C and qualification `apply` call carries `--cached` against a `read-tree`-seeded temp index. Asserted by a `PATH` wrapper recording argv; any `apply` without `--cached` fails the test. | W |
| AC-L8 | Landed target with a **dirty worktree** (feature reverted in the worktree only) ⇒ **PASS**. The rev-1 check false-reds this fixture. | W+C |
| AC-L9 | Landed target with a **dirty index** ⇒ PASS; the real index is never read. | W |
| AC-L10 | Worktree contains the feature but `HEAD` does not ⇒ anchor C **blocks**. | W |
| AC-L11 | After any landed run the real index is byte-identical, the worktree is byte-identical, `git status --porcelain -z` is unchanged, and the temp index is not an untracked entry. | W+C |
| AC-L12 | The temp index is removed on **every** exit path, including each terminal state. Asserted by scanning the git dir and `.tpatch/local/`. | W |
| AC-L13 | The temp index is created outside the tracked working tree. | W |
| AC-L129 | **Every** git invocation issued by verify — enumeration, `read-tree`, `apply`, `diff`, `merge-base`, `cat-file` — carries `GIT_NO_LAZY_FETCH=1`. Asserted by a `PATH` git wrapper recording each call's environment; any call missing it fails the test. | W |
| AC-L135 | **Totality guard — forbidden-phrase regex sweep over the three authoritative documents.** A docs test runs the regex set below over `docs/prds/PRD-verify-freshness.md`, `docs/prds/PRD-tpatch-land.md` and `docs/adrs/ADR-013-verify-freshness-overlay.md` and requires **zero** hits on any line that is not explicitly prefixed `historical`, `superseded`, `rejected`, `withdrawn` or `pre-rev-5`. See §7.1.2 for the exact regex table, the whitelist rule and the per-document section list. Paired with `PRD-tpatch-land` §6.2 **AC-LD23**. | U |
| AC-L134 | **Git-floor preflight.** With a `PATH` shim reporting `git version 2.30.2`, the run reports `landing_evidence.state: "unavailable"` with **R10** verbatim, and the shim's call log contains **only** `--version` — no `log`, `read-tree`, `apply`, `diff`, `cat-file`, `merge-base`, and no network call. A shim reporting `2.36.0` proceeds normally. Two fixtures. | W+C |

#### Group C — the hardened ladder

| # | Criterion | Tier |
|---|---|---|
| AC-L14 | Step 1 passes ⇒ `anchor_results.current == "materialized-clean"`, no advisory. | W+C |
| AC-L15 | Offset shifts and an unrelated edit far from any hunk ⇒ step 1 passes. | W |
| AC-L16 | Unrelated edit **2 lines** from a hunk ⇒ step 2 passes with **zero** `(0/0)` ⇒ **passed**, `materialized-context-drift`, `context-drift` advisory (R3). | W+C |
| AC-L17 | Unrelated edit **1 line** from a hunk ⇒ `(0/0)` reported ⇒ **BLOCK** (R2). The accepted false red, pinned deliberately. | W |
| AC-L18 | **Revert-in-place + identical postimage text elsewhere in the file** ⇒ `(0/0)` ⇒ **BLOCK**. The rev-1 false-green regression guard. | W |
| AC-L19 | **Partial revert of hunk 1** of a 3-hunk patch ⇒ both steps fail ⇒ BLOCK (R1). | W |
| AC-L20 | **Partial revert of hunk 2** ⇒ BLOCK. | W |
| AC-L21 | **Partial revert of hunk 3** ⇒ BLOCK. | W |
| AC-L22 | **Partial revert of hunks 1 + 3** (non-adjacent) ⇒ BLOCK. | W |
| AC-L23 | **Full revert** ⇒ BLOCK. | W+C |
| AC-L24 | A patched file **deleted** at `HEAD` ⇒ BLOCK. | W |
| AC-L25 | Degenerate whole-file hunk with header/footer ⇒ step 2 passes with zero `(0/0)` ⇒ PASS + `context-drift`. | W |
| AC-L26 | The `-C0` step runs with `LC_ALL=C`. Asserted by a `PATH` wrapper recording the call environment. | W |
| AC-L27 | Ladder results are memoised per `(tree, patch, direction, context)`; the same tree probed twice issues one pair of calls. | W |
| AC-L28 | R1 contains the literal `Do NOT run tpatch reconcile`, and no landed-mode remediation contains `reconcile` outside that negation. Adversarial. | W |

#### Group D — anchor collection, forward qualification, normalization

| # | Criterion | Tier |
|---|---|---|
| AC-L29 | **All** candidates are collected before any selection: with three slug-bearing single-parent landings reachable, `candidates_collected == 3` in the report even though the first one qualifies. Pins that rev-2's stop-at-first is gone. | W |
| AC-L30 | Qualification uses a **forward** `git apply --check --cached -C1` seeded by `git read-tree C^`. Asserted by a `PATH` wrapper: qualification calls must **not** carry `--reverse`, must carry `-C1`, and the `read-tree` revision must be `C^` (or `C^^{tree}`) — **never** `C^{tree}^`, which is invalid. | W |
| AC-L130 | `git rev-parse C^{tree}^` and `git read-tree C^{tree}^` both fail; `C^`, `C^^{tree}`, `C~1` and `C~1^{tree}` all resolve and `read-tree` accepts them. Pins the rev-3 syntax defect. | U+W |
| AC-L131 | The `-C1` qualification ladder matches the §3.6.8 measured table row for row: pristine, 4-lines-away, 2-lines-away, 1-line-away, pure offset, far-away, already-materialized, feature-line-changed and feature-line-deleted. Nine fixtures; the 2-lines-away row must pass **only** at `-C1`, and all three materialized/modified rows must fail at `-C1`. | W |
| AC-L132 | **Remediation-loop fixture**: a landed feature whose neighbourhood drifted two lines, then re-recorded over the drift and re-landed. Default-context qualification rejects the surviving candidate and dead-ends R11; `-C1` qualifies it and the run passes. Pins that the contract does not send an operator into an unresolvable loop. | W+C |
| AC-L31 | Measured semantics hold: at a clean pre-landing parent tree forward passes and reverse fails; at the landing itself forward fails and reverse passes. Both directions asserted on one fixture. | W |
| AC-L32 | A candidate parent carrying unrelated drift **inside** the patch's context window does **not** qualify, and the report says so rather than claiming immunity. | W |
| AC-L33 | **Re-record + re-land**: both landings are collected; the newest **fails** qualification (its parent already contains the feature) and the earlier **qualifies**. `attestation_commit` ≠ `replay_anchor_commit` in the report. | W+C |
| AC-L34 | Two qualifying candidates with **equal** normalized identity (cherry-pick then merge-back) ⇒ selection proceeds, `candidates_qualified: 2`, topo-oldest chosen. | W |
| AC-L35 | Two qualifying candidates with **differing** normalized identity ⇒ **FAIL `historical-anchor-unavailable`** (R11). Not silently resolved by picking one. | W |
| AC-L36 | Selection is deterministic across repeated runs: first in `--topo-order --reverse`, then lexicographically smallest commit id. | U+W |
| AC-L37 | The anchor search never broadens beyond exact-slug trailer commits; a non-trailer commit that introduced the same paths is never selected. Adversarial. | U+W |
| AC-L38 | A stale-hash qualifying candidate supplies **only** a tree: it never appears as `attestation_commit`, never changes `landing_evidence.state`, and never relaxes the attestation requirement. | W |
| AC-L39 | **No qualifier ⇒ FAIL `historical-anchor-unavailable`** with R11; V7, V8-historical and V10 all report `passed: false` **with `mode` present** — failed, not skipped. | W+C |
| AC-L40 | Anchor unavailable while anchor C is **clean** ⇒ the run still **FAILS**; `anchor_results.current` is `materialized-clean` and the verdict is `failed`. | W |
| AC-L41 | After the R6 re-land remediation anchor H is **regained** and the run passes; where history admits no qualifier it fails with R11. Both branches; neither degrades silently. | C |
| AC-L42 | Normalized identity is exactly `git diff --no-color --no-ext-diff --no-textconv --binary --no-renames --unified=0 <C>^ <C> -- <P…>` with **only** `^index ` lines removed, SHA-256 over the remainder. Asserted by a `PATH` wrapper on the argv. | W |
| AC-L43 | Measured normalization behaviour: the same logical change on two branches with different preimage blobs gives **different** raw `-U3` bytes, **different** `-U0` bytes, and **identical** bytes after the `index` strip. All three asserted on one fixture. | W |
| AC-L44 | Normalization **preserves** `old mode`/`new mode`, `GIT binary patch`, and the `new file mode` / `deleted file mode` rename split; hunk **bodies** keep content and order. Three fixtures. A mode-only change normalizes with no `@@` line at all and stays distinct from any payload change. | W |
| AC-L133 | **Offset-normalized duplicate trade, both directions.** (a) A cherry-pick applied after unrelated lines were prepended — headers `@@ -10 +10 @@ l9` vs `@@ -15 +15 @@ l9` — compares **equal** after the hunk-header rewrite and **unequal** without it. (b) Two changes with byte-identical `-`/`+` bodies at different positions in a file with duplicate lines compare **equal** — the documented trade. (c) A distinct payload, a mode-only change and a different path all stay **distinct**. | W |
| AC-L45 | An **empty** canonical path set makes candidates incomparable ⇒ `ambiguous`; never broadened to "all paths". Adversarial. | U+W |

#### Group E — evidence reader, grammar, presence, topology, shallow

| # | Criterion | Tier |
|---|---|---|
| AC-L46 | No reachable landing and no raw match ⇒ `none`, `target_mode: "forward"`, shadow at `HEAD`, V7/V8 verdicts **identical** to the pre-amendment implementation. | W+C |
| AC-L47 | All three values match with patch `present-nonempty` ⇒ `exact`. | W+C |
| AC-L48 | `Tpatch-Patch-SHA` mismatch ⇒ `stale`, FAIL, `landing-evidence`, V8 and V10 also failed with `mode` present. | W+C |
| AC-L49 | `Tpatch-Recipe-SHA` mismatch ⇒ `stale`. `Tpatch-Base-Commit` mismatch ⇒ `stale`. | W |
| AC-L50 | `Tpatch-Recipe-SHA: none` matches an **absent** recipe and a **present-empty** (whitespace-only) recipe. | U+W |
| AC-L51 | **Presence precedes digest**: patch `absent` ⇒ the report emits `patch_presence: "absent"` and **omits** `patch_sha_match` entirely; no mismatch is reported. | U+W |
| AC-L52 | Patch **present-empty** ⇒ **terminal `landed-artifacts-absent`** reached **before** any digest comparison, with the corruption framing; neither `exact` nor `stale` is reachable. Distinct fixture from `absent`, which behaves identically by the same short-circuit. | U+W |
| AC-L53 | The four recipe shapes are distinguished: `absent`, `present-empty`, `present-nonempty-zero-op`, `present-nonempty-with-ops`; the zero-op case records `0 op(s)` rather than a vacuous V7 pass. **Adversarial exclusivity check over the full 3×4 cross-product** of §3.6.2's table: all twelve cells are exercised and each maps to exactly one outcome; the eight cells whose patch state is absent or present-empty **can never** be classified `exact` or `stale` and all yield `landed-artifacts-absent`; and no cell is unclassified or matches two rules. | U+W |
| AC-L54 | Missing any of the four trailers, or a duplicate of any, or ≥2 `Tpatch-Feature` values ⇒ `malformed`. | U+W |
| AC-L55 | A commit whose **raw** body carries an exact `Tpatch-Feature: <slug>` line that Git does not parse as a trailer ⇒ **`malformed`**, never `none`. Fixtures for **both** the amend-destroyed block **and** the prose quotation; the prose false red is asserted as intended. | U+W |
| AC-L56 | Slug matching is exact after trimming; `my-slug` ≠ `my-slug-extended`. A **lowercase** trailer key is still a candidate. | U |
| AC-L57 | `Tpatch-Base-Commit` length is **derived** from `git rev-parse --show-object-format`: a **SHA-256 repository** accepts 64 hex and rejects 40; a SHA-1 repository does the inverse. Two fixtures. A hardcoded 40 fails this row. | U+W |
| AC-L58 | A git error, unparsable output, or an unreadable object format ⇒ `unavailable`, FAIL — never `none`. Proven with a `PATH` wrapper that exits non-zero. | U+W |
| AC-L59 | `base_commit_reachable: false` raises `base-commit-unreachable` and **does not fail** on its own. | W |
| AC-L60 | The enumeration is exactly **one** `git log --topo-order --reverse -z` per run carrying `%H`, `%P`, `%B` and all four trailers, **reused for every feature** of `verify --all`. Counted by the wrapper. | W |
| AC-L61 | **`rev-list` is never invoked.** Adversarial: the wrapper fails the test on any `rev-list` call. | W |
| AC-L62 | Records are consumed **oldest-first** and anchor selection uses that order directly. | U+W |
| AC-L63 | The §3.6.9 invocation budget is honoured, including **tree materialization**: 3 preflight calls, 1 enumeration, ≤1 `read-tree` per distinct tree, ≤1 forward `apply` per collected candidate, ≤2 ladder calls per `(tree, patch)`, `git diff` only when ≥2 qualify. Counted by the wrapper. | W |
| AC-L64 | Root landing (0 parents) in a **non-shallow** repository ⇒ `unsupported-topology`, R9. | U+W |
| AC-L65 | Merge landing (≥2 parents) ⇒ `unsupported-topology`, R9; never approximated to `^1`. | U+W |
| AC-L66 | **Shallow clone**: a candidate on the graft boundary reports 0 parents in `%P` yet classifies **`shallow-history`** with **R21 (`git fetch --unshallow`)**, not `unsupported-topology`/R9. Fixture uses a real `--depth 1` clone. | W+C |
| AC-L67 | The shallow discriminator is the preflight (`git rev-parse --is-shallow-repository` and/or `.git/shallow` membership), not the parent count — asserted by a fixture where a **true root** in a full repo still yields `unsupported-topology`. The preflight is proven to run **before** any parent-count branch, via `PATH`-wrapper call ordering. | W |
| AC-L68 | **Wave C acceptance gate.** **Partial (blobless) clone** built against a **real filtered remote** — a non-local transport (HTTP, SSH or `git daemon`) with `uploadpack.allowFilter=true`, or a deterministic promisor fixture — with objects available ⇒ verification proceeds normally (`repository.partial_clone: true`). If the environment cannot construct such a remote, Wave C **must report a blocker** and may not mark this row passed. | W |
| AC-L69 | **Wave C acceptance gate.** The same real filtered remote with a **genuinely missing promisor object** ⇒ **`history-incomplete`** with **R22**, and **no network call attempted**: the failure must be the local `fatal: Not a valid object name` form, never the `does not appear to be a git repository` form the default produces. Wave B proved only the *mechanism* on a synthetic promisor repo; the end-to-end path is unproven and this row is where it is closed. Same blocker rule as AC-L68. | W |
| AC-L70 | A landing reachable only through a merge's **non-first** parent **is** found. Cherry-picked and rebased landings ⇒ `exact`. | W |
| AC-L71 | Branch switch away from the landing ⇒ `none` ⇒ forward mode; equivalent content present anyway ⇒ still `none` and the diagnostic says the content is unattributed. Detached `HEAD` and history-rewrite rows included. | W |

#### Group F — closure arbitration and parents

| # | Criterion | Tier |
|---|---|---|
| AC-L72 | The presence test for every member is the §3.6.5 **patch ladder** at the anchor tree. Adversarial: no recipe operation is executed for a member that is ultimately skipped, and no whole-file byte comparison decides the outcome. | W |
| AC-L73 | A landed member whose ladder is clean is **skipped** — its recipe is never executed. Op-execution counter. | W |
| AC-L74 | A landed member with an `append-file` recipe is skipped and the anchor tree contains its payload exactly **once**. | W+C |
| AC-L75 | A landed member with a `replace-in-file` recipe is skipped without `search text not found`. | W |
| AC-L76 | A landed member whose ladder **blocks** ⇒ fail-fast `parent-landing-drift` (R14) **before** the target is judged. | W |
| AC-L77 | An applied hard parent with `evidence none` whose patch **ladder-passes** is **skipped** with a mandatory `unattributed-materialized` advisory (R18). | W |
| AC-L78 | An applied hard parent with `evidence none` whose ladder **blocks**, or which has no patch, is **replayed**, byte-identically to today. | W+C |
| AC-L79 | Landed member with patch **absent** or **present-empty** ⇒ **`landed-artifacts-absent`**, **regardless of recipe shape**. Four fixtures across the recipe shapes; none may reach a recipe-is-sole-authority outcome. Replaces the rev-4 row that contradicted §3.6.2's short-circuit. | W |
| AC-L80 | Landed member with patch **present-nonempty** and recipe absent / present-empty / zero-op ⇒ the patch ladder is sole authority and V7/V10 skip with their existing reasons. | W |
| AC-L81 | Landed member with **no usable artifact** under the §3.6.2 presence states ⇒ **FAIL `landed-artifacts-absent`** (R19). Never skipped, never replayed. | W |
| AC-L82 | Hard parent with any terminal non-`exact` evidence state — including `shallow-history` and `history-incomplete` — ⇒ fail-fast `parent-evidence-integrity` (R15). | W |
| AC-L83 | Hard parent in `unapplied` ⇒ `parent-unapplied` (R16), replacing today's generic `default:` message. | W+C |
| AC-L84 | Hard parent in `rejected` ⇒ `parent-rejected` (R17). | W |
| AC-L85 | Hard parent in `upstream_merged` is skipped byte-identically to today; a superseded parent stays excluded by the existing filter. | W |
| AC-L86 | Hard parent in **`active`** is treated exactly as `applied` — skipped or replayed by the same arbitration, never fail-fast. Asserted for a **non-landed** target too, where it changes today's verdict. | W+C |
| AC-L87 | After AC-L86, all four `active` sites agree: `dependency_gate.go:79-81`, `verify.go:127-134`, `verify_all.go:89-97`, and the closure switch. Adversarial cross-check. | U |
| AC-L88 | A revert landing **after** the anchor commit is invisible at anchor H and caught at anchor C; one predating the anchor is caught at anchor H. Both anchors reported. | W |
| AC-L89 | Parent landed **after** vs **before** the target yields identical verdicts; closure ordering for an all-unlanded closure is topological and identical to today. | W |
| AC-L90 | Mixed chain — target unlanded, P1 landed, P2 applied-unlanded ⇒ anchor `HEAD`, P1 ladder-skipped, P2 replayed, target forward-verified. | W+C |
| AC-L91 | Mixed chain — target landed, P1 applied-unlanded ⇒ anchor is the target's replay-anchor parent tree, P1 replayed there, target judged at both anchors. | W |

#### Group G — V10 per-member baselines and provenance

| # | Criterion | Tier |
|---|---|---|
| AC-L92 | **Each landed member's V10 uses its OWN replay-anchor parent tree**, never the target's. Asserted on `checks[].member_baselines`: a landed parent's baseline commit differs from the target's, and the parent's `preimage_hash` is compared at a tree that does **not** already contain the parent's postimage. This is the rev-2 defect. | W+C |
| AC-L93 | A landed parent whose postimage **is** present at the target's anchor still passes V10, because it is evaluated at its own baseline. Regression guard for the same defect. | W |
| AC-L94 | Landed target, `preimage_hash` matching at the target's anchor-H closure baseline ⇒ V10 PASSES, `mode: "historical-anchor"`. **Fails today** — `checkWriteFilePreimage` reads the live working tree. | W+C |
| AC-L95 | Landed target, `preimage_hash: ""` (new-file) with the file absent at the anchor baseline ⇒ V10 passes. Today it fails with `new-file collision`. | W |
| AC-L96 | **Unlanded** applied feature, op with `preimage_hash`, provenance present + well-formed + reachable + hash-bound ⇒ V10 evaluated at `RecipeProvenance.BaseCommit`, `mode: "provenance-anchor"`. **Fails today.** Q15. | W+C |
| AC-L97 | Provenance sidecar **without** `recipe_sha256` (pre-v0.5.2) ⇒ accepted, with `provenance_hash_bound: false` in the report. | U+W |
| AC-L98 | Provenance `recipe_sha256` **mismatching** the inventory's recipe bytes ⇒ treated as inventory-inconsistent, not silently trusted. | U+W |
| AC-L99 | Provenance **absent**, **ill-formed** (wrong hex length for the object format), or **unreachable** from `HEAD`, with a real `preimage_hash` present ⇒ **FAIL `recipe-provenance-unavailable`** (R24). Three fixtures. | U+W |
| AC-L100 | Verify **never** reads the live working tree for a preimage comparison in any mode. Adversarial: a fixture whose worktree would make the check pass must still fail. | W |
| AC-L101 | Ops **without** `preimage_hash` (the autogen shape, `internal/workflow/recipe_autogen.go:114-118`) stay on the ADR-029 D4 legacy path — pass, no re-warn — **and** the absence of a provenance sidecar on that path is not an error. | U+W |
| AC-L102 | A **malformed** `preimage_hash` (ADR-029 D1 form violation) ⇒ FAIL block regardless of any later-touch state. | U+W |
| AC-L103 | Later-touch is derived from the **shipped metadata detector** (`RequestedAt` + `patch-generations.touched_paths` ∪ recipe op paths). Adversarial: a path whose **bytes** differ at HEAD but which **no later feature touched** raises **no** advisory. | U+W |
| AC-L104 | A genuine later-touch ⇒ **`later-touch` warn** (R13); the verdict is not blocked by it. | W+C |
| AC-L105 | Superseded landed feature with a preimage mismatch ⇒ severity downgraded to `warn` (ADR-029 D7). V2 skipped/failed ⇒ V10 skips with its existing reason. | W |
| AC-L106 | **Parent V10 aggregation**: a member's block-class outcome contributes to `parent-landing-drift`; its warn-class later-touch appears in `advisories` under the member's slug and affects no verdict. | W |

#### Group H — inventory, schema, diagnostics, run-level

| # | Criterion | Tier |
|---|---|---|
| AC-L107 | The inventory is built from **`store.ListFeatureEntries()`**, not `ListFeatures()`, and covers **every** feature — status, `RequestedAt`, the three artifacts' presence states and raw bytes, and `patch-generations.touched_paths`. Asserted on a repo with features outside the closure that participate in ADR-029 ordering, **and** by a `PATH`/call assertion that `ListFeatures` is not the enumeration source. | W |
| AC-L108 | Classification, V7, V8, V10, the persisted `VerifyRecord` and the derived labels are a **pure, deterministic function of the inventory**: two runs over identical inventory values produce byte-identical reports, and the unit under test performs no filesystem access. | U |
| AC-L109 | Every later stage consumes **copies**: an artifact changed after capture but before the final re-read still yields digests computed from the captured bytes. | U |
| AC-L110 | A feature **added**, **removed**, **changed**, or **flipping between an `Err` row and a `Status` row** during a run ⇒ **FAIL `snapshot-unstable`** (R20) naming the slug and path. Four fixtures; the add/remove and `Err`-flip cases are the ones `ListFeatures` could not see. Proven at **U** over the inventory abstraction and at **W** with a `PATH` wrapper mutating `.tpatch/` between git calls. **No production hook.** | U+W |
| AC-L111 | An unreadable `status.json` is **represented in the inventory as an `Err` row, never dropped**. On an **unrelated** feature ⇒ `warn` `inventory-unreadable` advisory naming the slug **and** exclusion from ADR-029 later-touch ordering; on the **target or a closure member** ⇒ **FAIL `inventory-unreadable`**. Three fixtures (unrelated, target, closure member); the unrelated case additionally asserts the slug appears in `advisories` — a silent skip fails this row. | U+W |
| AC-L112 | Inventory enumeration order is deterministic — the slug-sorted order `ListFeatureEntries()` returns (`internal/store/store.go:344-346`) — and the report is stable across runs. | U |
| AC-L113 | `schema_version` is `"1.1"`; a no-evidence report is **semantically** compatible with `"1.0"` — every `"1.0"` key retains name, type and position — but **not** byte-identical. The test asserts the additive superset. | W |
| AC-L114 | `failed_at` only takes a value from the §4.3.9 closed set of **thirteen**, and advisory `code` only from the closed set of **five**. Adversarial enumeration. | U+W |
| AC-L115 | **No verify report contains `freshness_label`** (Q16). Adversarial over every §4.3 shape. | W |
| AC-L116 | `checks[].mode` is present on V7, V8 and V10 in **every** report — passed, failed **and** skipped — and absent on V0–V6 and V9. | W |
| AC-L117 | Every R1–R22 and R24 remediation string is emitted **verbatim** (golden strings). **R23 is `land`-owned** and is pinned by `PRD-tpatch-land` §6.2 AC-LD18/AC-LD18a, including the Mode-B retained-artifacts note. | W+C |
| AC-L118 | The human report emits `baseline:` and `landing evidence:` above the check list, naming both anchors, the replay anchor when it differs from the attestation, and the `isolated index` probe. | W+C |
| AC-L119 | A passing landed run persists a `VerifyRecord` with the same field set as a passing forward run — no new persisted field, `omitempty` round-trip preserved. | W |
| AC-L120 | Sticky clearing is mode-agnostic; `ComposeLabels` takes no mode input (adversarial: the labels package has no reference to landing evidence). | U+W |
| AC-L121 | `TestRunVerify_EquivalentRecipeAndPatchBothPass` (`internal/workflow/verify_closure_replay_test.go:275`) stays green **unmodified**. GH #2 non-regression. | W |
| AC-L122 | The GH #2 reset holds at anchor H: after V7 mutates the shadow the tree hash seen by V8 equals `closureBaselineTree`. | W |
| AC-L123 | The shadow is pruned on **every** exit path, including each new terminal state. | W |
| AC-L124 | `verify --all` ordering is byte-identical to today (Q12), and the enumeration plus inventory are built **once** and reused across features. | C |
| AC-L125 | Exit codes unchanged: `0` pass, `2` any block-severity failure including every terminal state, `1` reserved. Warn advisories never change the exit code. | C |
| AC-L126 | The `replace-in-file` diagnostic predicate is sound on the §3.6.6 exhaustive corpus (0 false reds, 0 false greens), including `c='abb', S='aa', R='b'` ⇒ true and `c='b', S='a', R='a'` ⇒ false; `R == ""` undecidable; `S == ""` unsupported. | U |
| AC-L127 | Diagnostic predicates never certify: a `write-file` op whose content matches byte-for-byte while the patch ladder **blocks** must still FAIL; `append-file` with empty content reports undecidable rather than passing. | U+W |
| AC-L128 | `gofmt -l .` clean, `go build ./cmd/tpatch` clean, `go test ./...` clean, and `make wave-close-check` 8/8 at the Wave C close commit. | C |

**Matrix size: 135 numbered criteria (AC-L1 … AC-L135) across 8 groups** —
A 6, B 10, C 15, D 21, E 26, F 20, G 15, H 22. AC-L129–AC-L135 are appended to
the groups they belong to (B, D, D, D, D, B, B respectively) rather than to a
new group, so the group totals above are authoritative. **AC-L135** is the
documentation totality guard specified in §7.1.2.

**Wave C acceptance gates.** AC-L68 and AC-L69 require a **real filtered
remote**. Wave B proved only the offline *mechanism*; if the implementation
environment cannot construct such a remote, Wave C must report a **blocker**
and may not mark those rows passed.
A Wave C dispatch that cannot place a row at its stated tier must amend this
section rather than silently re-tier it.

#### 7.1.2 AC-L135 totality guard — the exact regex set

The guard is a **unit** test over document bytes, not a workflow or CLI test.
It reads exactly three files and applies exactly these patterns
(case-insensitive, per line):

| # | Regex | Why it is forbidden |
|---|---|---|
| G1 | `V9\s+is\s+last` | The shipped sequence ends at **V10** (`internal/workflow/verify.go:288-289`). |
| G2 | `\bten[- ]check\b|\b10[- ]check\b|\b10[- ]row\b|exactly ten\b` | The sequence is **eleven** checks, V0–V10. |
| G3 | `Amendment 1 rev-[0-6]\b|proposed rev-[0-6]\b` | The amendment is **rev-7 (final)**; only the revision-history and rejected-alternatives narrative may name earlier revisions. The character class is widened by one on every revision, so a stale current-amendment label cannot survive a bump. |
| G4 | `land[’']?s? behaviou?r is unchanged|behaviou?r-frozen|behaviou?r-neutral` | The amendment adds **one** producer refusal (§3.8.6 / D19); a blanket unchanged/neutral claim is false. |
| G5 | `40[- ](lowercase[- ])?hex|hardcode[sd]? 40|fixed 40`, **scoped to lines mentioning `Tpatch-Base-Commit`, `base_commit` or `BaseCommit`**, and **exempt when the same line also matches `derived|--show-object-format|would reject|rejects|fails this row`** | That length is **object-format-derived** (40 `sha1` / 64 `sha256`). The exemption is deliberately narrow: it clears only sentences that *forbid* the hardcode by naming the derivation or its consequence — land rule 18's "a reader that hardcodes 40 **rejects** every valid landing in a SHA-256 repository" and AC-LD5 / AC-LD19's "a hardcoded 40 **fails this row**" — while an affirmative claim still trips the pattern because it names neither the derivation nor a rejection — the optional `lowercase` alternation exists precisely so the pre-rev-5 rule-6 phrasing (`… is 40 lowercase hex`) is caught, which a bare `40[- ]hex` pattern misses. Deliberately *not* applied to `satisfied_by`, whose shipped regex really is 40-hex (`internal/store/validation.go:22`) — see §8 residual. |
| G6 | `absent.{0,60}mismatch|mismatch.{0,60}absent|any attested value mismatch`, **affirmative voice only** (a line that also matches `never|not a mismatch|rather than a mismatch|no mismatch` is exempt) | An absent or empty patch short-circuits to **`landed-artifacts-absent`**; it never mismatches a digest. |
| G7 | `exact` and (`absent`\|`present-empty`) within 40 characters, **affirmative voice only** (exempt when the line also matches `neither|cannot|never|not reachable|no row`) | `exact` is reachable only from **present-nonempty**. |
| G8 | `mutat(ing|es) nothing`, **scoped to the §3.8.6 Base-Commit-refusal context** — the line must also mention `base_commit`/`BaseCommit`, `R23`, `recoverLand`, `Mode A`, `Mode B` or `journal` | Mode B retains `record`'s artifacts and Mode A with a pending journal permits `recoverLand`'s mutation, so an unqualified "mutating nothing" is false there. Two true claims are deliberately **out of pattern**: `land --dry-run` really does mutate nothing (AC-LD14, pre-existing §3.5 contract), and anchor C's temp index really does mutate nothing (D11) — neither line mentions the refusal context. |
| G9 | `freshness_label` inside a fenced block that also contains `"checks"` | `VerifyReport` has no such field (§4.3, Q16). |
| G10 | `E1–E4[0-6]\b` | The empirical index is **E1–E47**, closed. |

**Whitelist rules.** A hit is permitted **only** under one of these four
mechanically checkable conditions:

1. **Marked line** — the line, after stripping list/quote/table punctuation,
   begins with `historical`, `superseded`, `rejected`, `withdrawn` or
   `pre-rev-5`.
2. **Marked block** — the line sits under a Markdown heading, or under a
   bolded label paragraph, matching
   `Revision history|Alternatives considered|Retraction`, and before the next
   heading. This covers ADR-013's `**Revision history.**` list and its
   rejected-alternatives block, which are paragraphs rather than headings.
3. **Negated or quoted** — the same line also matches
   `\bnot\b|\bnever\b|\bno\b|cannot|neither|rather than|only when|only from`,
   so a sentence that *forbids* the phrase does not trip the guard that
   enforces it. Some correct sentences forbid a phrase **affirmatively**, by
   stating the consequence rather than by negating; those are cleared by the
   per-pattern exemptions in the table above (G5's
   `derived|--show-object-format|would reject|rejects|fails this row`, G6's and
   G7's voice exemptions), not by this rule.
4. **Self-reference** — the line is a row of the G-table in this section, or
   names `AC-L135`.

The test asserts the whitelist by prefix and block, not by proximity, so a
stale claim cannot hide next to a historical one. Scope qualifiers in the table
above (G5's base-commit scope, G8's `land` scope) are part of the pattern, not
of the whitelist.

**Sections the guard must cover** (asserted non-empty so the test cannot pass
by reading the wrong file): this PRD §2, §3.1, §3.1.1, §3.4.3, §3.6, §4.3, §5,
§6, §7.1; `PRD-tpatch-land` header, §3.8.2, §3.8.6, §6.2; ADR-013 header, D8–D19
and the references block.

#### 7.1.1 Explicit non-goals for Wave C

- No change to `tpatch land`'s **successful** path, output or refusal messages
  beyond the single new `Tpatch-Base-Commit` precondition
  (`PRD-tpatch-land` §3.8.6, AC-LD18–AC-LD20).
- No new check ID, no new `FeatureState`, no new `ReconcileLabel`, no new
  persisted field, no new artifact, no `.tpatch/` schema migration.
- **No `freshness_label` in the verify report** (Q16).
- No change to forward-mode V7/V8 semantics for features with evidence `none`
  — **except** the `active` closure widening (AC-L86/AC-L87), a deliberate,
  separately-pinned behaviour change.
- **Forward-mode V10 does change** for ops that carry `preimage_hash`: it moves
  from the live working tree to `RecipeProvenance.BaseCommit` (Q15). This is
  the fix, not a regression, and is pinned by AC-L96–AC-L101.
- No back-fill migration for pre-provenance recipes (Q17).
- No stripping of the `-U0` function-context suffix (Q18).
- No provider calls; verify stays offline.
- No auto-healing: verify never invokes `record`, `land` or `apply`.
- No `--all` reordering (Q12); no `land`-side hook warning (Q13).


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
| **The inventory is O(features), not O(closure).** | `loadLaterFeatureTouches` already reads every feature, so a narrower snapshot could not make ADR-029 ordering deterministic. Cost is `F` status decodes plus up to `3F` artifact reads once per run, reused by `verify --all` (AC-L107, AC-L112). |
| **Anchor collection costs one forward `apply --check` per candidate** instead of stopping at the first qualifier. | Required for correctness — stop-at-first can never observe a second, non-equivalent qualifier (AC-L29, AC-L35). Memoised per `(tree, patch)`. |
| **`recipe-provenance-unavailable` is a new terminal failure** for recipes carrying `preimage_hash` without a usable sidecar. | It replaces a measured false block that fired for *every* applied feature. Remediation is one command; the affected population is bounded and tracked as Q17 (AC-L99). |
| **Forward-mode V10 changes behaviour** for ops with `preimage_hash`: the reference tree moves from the live working tree to `RecipeProvenance.BaseCommit`. | That is the fix for a measured defect, not a regression, and ops without `preimage_hash` are untouched on the ADR-029 D4 legacy path (AC-L96, AC-L101). |
| **`land` gains a refusal path** on legacy/corrupt status. | Fail-closed at production time beats an unreadable trailer at every future read; the successful path is byte-unchanged (`PRD-tpatch-land` §3.8.6, AC-LD18, AC-LD21). |
| **Anchor H is unavailable for root/merge landings and for histories where every landing's parent already contains the feature.** | Reported explicitly as `historical_anchor.state = "unavailable"` with a reason; anchor C still runs at block severity, so no verdict is silently weakened. AC-L12 pins it. |
| **Verify now fails on git below 2.36 instead of silently forward-verifying.** | Deliberate: `unavailable` is a block because degrading to `none` converts an unknown into a positive claim. The floor is set by the mandatory `GIT_NO_LAZY_FETCH` (§3.6.9), the preflight runs before any object command, and R10 names the exact version and the reason. |
| **`satisfied_by` is validated by a 40-hex-only regex (`internal/store/validation.go:22`), so V5 cannot accept a SHA-256 repository.** | **Pre-existing and out of scope for this amendment.** The landed contract derives `Tpatch-Base-Commit`'s length from `git rev-parse --show-object-format` (§3.6.2, land rule 18), but V5's shipped regex is unchanged and remains 40-hex. Noted here so a reviewer does not read the two as contradictory; a SHA-256 fix for V5 needs its own ticket. |
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
- V3–V9 stubbed with `TODO` and a sentinel result that marks them `passed=true, severity=warn` so the report still emits a stable-shaped array. *(Historical Slice A note, written before V10 shipped; the current set is eleven rows V0–V10 — `internal/workflow/verify.go:49-71`.)*
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
