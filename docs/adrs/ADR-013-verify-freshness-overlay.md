# ADR-013 — Verify Freshness Overlay

**Status**: Accepted (M15 Wave 3 design — Git-like freshness redesign; PRD: `docs/prds/PRD-verify-freshness.md`) · **Amendment 1 rev-1 proposed 2026-08-12 (v0.15.1 Wave B / GH #8 — landed-evidence semantics; D8–D16 below, AWAITING REVIEW)**
**Date**: 2026-04-27 (original) · 2026-08-12 (Amendment 1)
**Deciders**: Core (M15 Wave 3 design — second revision after re-review); Amendment 1 — v0.15.1 Wave B planning writer
**Supersedes**: ADR-012 (in full — every D1–D7 either replaced, retained, or dropped; see the supersession map below). The first-revision design (commit `8c3d72e`) extended `FeatureState` with a `tested` value; that approach is abandoned. The re-review of `8c3d72e` (findings F1, F2, F3, F4) is the trigger.
**Related**: ADR-010 (M12 resolver), ADR-011 (feature DAG), ADR-019 (`tpatch land` trailer-block schema — Amendment 1's evidence substrate), ADR-016 (`record` auto-base resolution — owner of `apply.base_commit`), ADR-028 (supersession edge model), ADR-032 (feature-unapply state boundary), `docs/prds/PRD-verify-freshness.md` (successor PRD), `docs/prds/PRD-tpatch-land.md` (Amendment 1 co-amended), `docs/prds/PRD-verify-and-tested-state.md` (superseded predecessor PRD), `docs/dependencies.md`, CHANGELOG v0.6.1, CHANGELOG v0.11.3 (GH #2 fix this amendment must not regress)

## Context

v0.6.1 closed M15 Wave 1 + Wave 2 with no new lifecycle states and no new read-only verbs. Wave 3 picks up two backlog items:

- `tpatch verify <slug>` — cheap, machine-checkable health command.
- A persistent answer to "verify last passed against the current world."

The first-revision design (approved on `8c3d72e`) modelled the second item as a new `tested` value on the `FeatureState` enum. An external re-review surfaced two structural problems:

- **F1**: V7/V8 shadow replay ignored the target's hard-parent closure, so verify was structurally meaningless for any non-leaf feature whose parent was locally `applied`.
- **F4**: The design conflated lifecycle (sticky, write-by-explicit-verb) with verification freshness (drift-sensitive). It routed a "parent-state hook" through `LoadFeatureStatus`, a read path, which would have meant a read command silently mutates `.tpatch/`.

The supervisor's binding adjudication: redesign with **Git-like semantics**. Lifecycle is the lifecycle (commits — sticky). Freshness is `git status` for the verify check (derived, read-time). This ADR locks the seven decisions of the rewrite. Operational details (exact JSON shapes, flag set, slice boundaries) live in the PRD; this ADR governs the load-bearing invariants.

ADR-011 (feature DAG) and ADR-010 (provider-assisted resolver) remain **binding context**: this ADR does not amend either, and it explicitly preserves ADR-011 D6 (`Reconcile.Outcome` source-truth guard) and ADR-010 D5 (artifact ownership split).

## Supersession map (relative to ADR-012)

This ADR supersedes ADR-012 in full. The mapping of each prior decision:

| Old decision | Disposition | New decision (this ADR) | Why |
|---|---|---|---|
| **D1** — `tested` as a `FeatureState` enum value (linear forward from `applied`) | **REPLACED** | D1 — verify produces a `Verify` sub-record on `FeatureStatus`; `FeatureState` enum is unchanged | Lifecycle and freshness are separate concerns. Encoding "verify last passed" as a sticky lifecycle state forced read paths to mutate state when parents drifted. The freshness overlay is derived at read time and only the explicit `verify` write verb persists anything. |
| **D2** — `tested` satisfies the hard-dep gate (equivalent to `applied`) | **DROPPED** (question is moot) | D2 — apply gate is pure-lifecycle; satisfaction set remains `{applied, upstream_merged}`; freshness is ignored by the gate | There is no `tested` lifecycle state to satisfy. Freshness is an operator/harness signal, not a gate input. Folding it into the gate would re-introduce the demote-on-read pathology from a different angle. |
| **D3** — producer set: only `verify` writes `tested`; `test` does not; `amend` does not | **REPLACED** (mostly retained) | D3 — `verify` writes the `Verify` sub-record; `amend` invalidates by clearing it; `test` does not write | Same producer-purity intent, but the artefact produced is a freshness record, not a lifecycle transition. `amend` must clear, not preserve, because amend touches recipe/patch hashes. |
| **D4** — `omitempty` round-trip for byte-identical v0.6.1 status.json | **RETAINED** | D4 — the new `Verify` sub-record carries `omitempty` on every nested field; v0.6.1 repos round-trip byte-identical until verify runs once | Backwards-compat invariant unchanged. |
| **D5** — forward/backward state-transition table (`applied → tested`, `tested → applied`, etc.) | **DROPPED** (no transitions to table) | D5 — derived label transitions only: `never-verified` / `verified-fresh` / `verified-stale` / `verify-failed`, recomputed at read time in `ComposeLabels` | No persisted transitions exist under the freshness model. Demotion is replaced by derived staleness; the derivation function is the new D5. |
| **D6** — source-truth alignment: `tested` lives in `status.json`, never inferred from `artifacts/reconcile-session.json` | **RETAINED** | D6 — the `Verify` sub-record lives in `status.json`; never inferred from artifacts | Reuses ADR-011 D6 verbatim. ADR-010 D5 source-truth guard binding. |
| **D7** — verify is read-only on the working tree; uses shadow workspace for apply-simulation | **RETAINED + EXTENDED** | D7 — verify is read-only on the working tree; shadow simulation now includes hard-parent topological closure replay before applying the target's recipe (F1 fix) | Without the closure replay, V7/V8 false-fail any non-leaf feature whose parent is locally `applied`. The closure is a verify-only construct; no other code path replays parent recipes into shadows. |

## Findings the redesign addresses

- **F1** (CRITICAL): V7/V8 shadow replay now includes the hard-parent topological closure. Spelled out in this ADR's D7 + the successor PRD §3.4.3.
- **F4** (CRITICAL): Lifecycle and freshness fully separated. The "parent-state hook" of the prior design is replaced by pure read-time label derivation in `ComposeLabels`. No read path mutates `.tpatch/`.
- **F2** (HIGH, CURRENT.md drift): the old idle CURRENT.md invented a `Tested *TestedRecord` field. Resolved: the new field is `Verify` (a freshness sub-record), explicitly locked in this ADR's D1.
- **F3** (MEDIUM, Slice A drift): the old idle CURRENT.md pulled `--all`, `--shadow`, and skill-anchor regen into Slice A. The successor PRD's slicing reaffirms Slice A as just the cobra shell + V0–V2 + freshness writer skeleton.

---

## Decision

### D1. Verify produces a freshness overlay; lifecycle stays untouched

`tpatch verify <slug>` writes a new sub-record on `FeatureStatus`:

```go
type FeatureStatus struct {
    // … existing fields unchanged …
    Verify *VerifyRecord `json:"verify,omitempty"`
}

type VerifyRecord struct {
    VerifiedAt          time.Time
    Passed              bool
    CheckResults        []VerifyCheckResult
    RecipeHashAtVerify  string
    PatchHashAtVerify   string
    ParentSnapshot      map[string]FeatureState
}
```

The `FeatureState` enum (`internal/store/types.go:9–19`) is **unchanged**. There is no `StateTested`, no new lifecycle value, no new state-write site outside the existing `apply` / `amend` / `reconcile` paths.

**Why A (chosen) — freshness overlay + lifecycle untouched.**

- Mirrors the Git mental model: lifecycle states are commits, freshness is `git status`. Operators already know this distinction.
- Reads stay reads. The label-recomputation loop that derives `verified-fresh` vs `verified-stale` runs in `composeLabelsFromStatus` (`internal/workflow/labels.go:143`) — a pure function over `(child, parents[])` — and writes nothing. F4 is structurally precluded.
- Backwards compat is automatic at the schema level: `Verify *VerifyRecord` with `omitempty` round-trips byte-identical for v0.6.1 fixtures (no `tested` field populated → field omitted).
- Drift detection is free: `recipe_hash_at_verify` + `patch_hash_at_verify` + `parent_snapshot` are exactly the inputs the verify run leaned on. If any drift, the next `ComposeLabels` call derives `verified-stale` without touching disk.

**Why B (rejected) — `StateTested` as a `FeatureState` enum value.**

- This was the first-revision design. Rejected on F4 grounds: conflates two orthogonal axes (lifecycle progress vs verification freshness). Forces every gate consumer to reason about a state whose semantics include "and the world hasn't moved since I last asserted this," which is not a property of any other lifecycle state.
- Demote-on-read problem: keeping a `tested` state correct under upstream drift requires either (a) writes from a read path (rejected by F4) or (b) every reader treating `tested` as "maybe-tested" (which makes the state useless).
- Apply gate becomes ambiguous: does `tested` satisfy hard deps? Either answer creates surprise. Direction A (yes) means a child applied at T1 against `tested` parent finds the parent demoted to `applied` at T2 with no operator action. Direction B (no) makes a strictly stronger state count as weaker, which is UX-hostile. Both directions were extensively argued in the first revision; F4 dissolves the question by removing the state.

**Why C (rejected) — `Verify` as a top-level file under `.tpatch/<slug>/verify/`.**

- Adds a new artifact lifecycle for no benefit. The freshness record is small (sub-1KB typical), tightly bound to `FeatureStatus` (read together by `LoadFeatureStatus`), and never the source of truth for a derived decision other than label composition. A separate file would mean two reads, two writes, two consistency invariants.
- ADR-010 D5 + ADR-011 D6 explicitly establish `status.json` as the canonical source for derived decisions. Adding a sibling artifact for verify-specific data fights that invariant.

**Decision: A (freshness overlay).**

### D2. Apply gate stays pure-lifecycle

`workflow.CheckDependencyGate` (`internal/workflow/dependency_gate.go:79`) accepts hard parents in `{applied, upstream_merged}`. **This is unchanged in Wave 3.** Freshness labels do not extend the satisfaction set.

**Why A (chosen) — gate ignores freshness.**

- Lifecycle gates govern persistence; freshness governs harness composition. This is the Git-like answer: `git checkout` doesn't ask `git status` whether the working tree is clean before allowing the checkout (with `--force`); the user composes the two checks at the harness level.
- Avoids the demote-on-read problem from a different angle: if the gate consulted freshness, a child applied at T1 against a `verified-fresh` parent could find its parent's freshness flipped to `verified-stale` at T2 with no operator action. The retroactive change to gate satisfaction would be invisible to `apply` callers — the same kind of hidden state the F4 redesign exists to preclude.
- Implementation is zero lines: the gate is unchanged, the original "extend `case StateApplied:` with `case StateTested:`" diff from the first revision is dropped entirely.
- The harness composition pattern `tpatch verify parent && tpatch apply child` keeps working at the harness level: the harness reads `verified-fresh` from `tpatch status --json` and decides whether to re-run verify before composing. The CLI does not enforce.

**Why B (rejected) — gate accepts `applied + verified-fresh` as a stronger form.**

- Re-creates the F4 problem. Freshness is read-time-derived; a parent's `verified-fresh` label can flip to `verified-stale` between T1 (child apply) and T2 (the operator re-checks), and neither side has any way to detect the transition without re-running both verify and apply.
- The "stronger" framing is misleading: `verified-fresh` is **dynamically** stronger than `applied` (more checks have passed) but **structurally** the same (same lifecycle position). Gate semantics should be structural, not dynamic.

**Decision: A (gate ignores freshness).**

The first-revision framing — "does `tested` satisfy hard deps?" — is now obsolete. There is no `tested` state to satisfy. The PRD §3.4.6 documents the collapse explicitly so future readers understand the shift.

### D3. `verify` is the unique writer of the freshness record; `amend` invalidates it

Producer set:

- `tpatch verify <slug>` writes the full `Verify` record (`VerifiedAt`, `Passed`, `CheckResults`, hashes, `ParentSnapshot`).
- `tpatch amend <slug>` (recipe-touching) **invalidates** the existing record by setting `Verify.Passed = false` and bumping no other field. Rationale: an amend that rewrites `apply-recipe.json` or `artifacts/post-apply.patch` causes `recipe_hash_at_verify` / `patch_hash_at_verify` to drift, so the next `ComposeLabels` would derive `verified-stale` regardless. We clear `Passed` proactively to make the invalidation visible at write time (operator inspecting `status.json` immediately after amend sees `passed: false`).
- `tpatch amend <slug>` (intent-only — `request.md` / `spec.md` only) leaves `Verify` untouched.
- `tpatch test`, `tpatch apply`, `tpatch reconcile` do **not** write or invalidate `Verify`. (Reconcile's recipe/patch rewrites cause hash drift naturally; the next `ComposeLabels` derives `verified-stale` without an explicit clear.)

**Why verify-only.**

- The freshness record is a machine-checkable claim. `tpatch test` runs a user-configurable `Config.TestCommand` whose contract on side effects is loose; conflating "user's tests passed" with "tpatch's invariants hold" produces false greens.
- A manual flip (`amend --notes "I promise this is verified"`) is rejected for the same reason. Harnesses cannot trust hand-written claims.
- Implicit `verify` after every `apply` inflates apply latency for a benefit the harness can opt into via `apply && verify`.

**Future-work expansion**: if a real harness need surfaces for `tpatch test` to also be a producer (e.g., "verify + tests-green = double-fresh"), revisit in `feat-tested-state-test-producer`. Out of Wave 3.

**Cost**: one new verb to learn, one extra hook on the `amend` write path.

### D4. Backwards-compatibility contract — byte-identical round-trip for v0.6.1 repos

A v0.6.1 repo that never runs `verify` must round-trip every `status.json` byte-identically through v0.6.2 read/write paths. Locked by:

- The `FeatureStatus` schema gains exactly one field: `Verify *VerifyRecord` with `json:"verify,omitempty"`. A `nil` pointer is omitted from JSON output entirely; the v0.6.1 byte-stream is unchanged.
- `FeatureState` enum is unchanged (this ADR explicitly does not extend it). `ReconcileSummary` and `Config` are unchanged.
- The `ReconcileLabel` vocabulary gains four values (`never-verified`, `verified-fresh`, `verified-stale`, `verify-failed`), but labels are **derived at read time** and never persisted to `status.json`. A v0.6.1 round-trip never emits any of the new label strings.

Enforced by a regression fixture: `TestUpgradeFromV0_6_1_NoVerify_BehavesIdentically` — load v0.6.1 fixture, run every v0.6.1 command except `verify`, diff `.tpatch/` against v0.6.1 expected output, fail on any byte difference.

**Cost**: any future change that touches `FeatureStatus` field set must include this regression fixture in its acceptance criteria.

### D5. Derived freshness labels — read-time computation, no persisted transitions

Four labels are added to `ReconcileLabel`. They are **derived** every time `ComposeLabels` (`internal/workflow/labels.go:89`) runs, from the freshness record + the current DAG snapshot. **No persisted state transitions.** The lifecycle never moves as a result of label derivation.

**Derivation function** (input: `FeatureStatus` for the child + `LoadFeatureStatus` for each hard parent; output: exactly one of the four labels):

```
if child.Verify == nil:
    return "never-verified"
if child.Verify.Passed == false:
    return "verify-failed"
// child.Verify.Passed == true; check freshness
if child.Verify.RecipeHashAtVerify != sha256(read(child/apply-recipe.json)):
    return "verified-stale"
if child.Verify.PatchHashAtVerify != sha256(read(child/artifacts/post-apply.patch)):
    return "verified-stale"
for (parent_slug, snapshot_state) in child.Verify.ParentSnapshot:
    parent_now := LoadFeatureStatus(parent_slug).State
    if not satisfies_state_or_better(parent_now, snapshot_state):
        return "verified-stale"
return "verified-fresh"
```

**`satisfies_state_or_better`** rules:

- `applied` snapshot is satisfied by current `applied` or `upstream_merged` (both satisfy the apply gate; the structural guarantee verify leaned on is preserved).
- `upstream_merged` snapshot is satisfied only by current `upstream_merged` (terminal-by-design; transition out is a manual-edit anomaly).
- Pre-apply snapshots (`requested` / `analyzed` / `defined` / `implementing`) are satisfied by current `applied` or `upstream_merged` (the parent has only become more healthy).
- `blocked` / `reconciling` / `reconciling-shadow` snapshots are satisfied only by themselves; any transition invalidates freshness.

**Transitions** (all derived, all read-time, none persisted):

| From label | To label | Trigger |
|------------|----------|---------|
| `never-verified` | `verified-fresh` | `tpatch verify` PASS |
| `never-verified` | `verify-failed` | `tpatch verify` FAIL |
| `verified-fresh` | `verified-stale` | recipe hash drift, patch hash drift, or parent state drift |
| `verified-fresh` | `verify-failed` | next `tpatch verify` FAIL |
| `verified-fresh` | `verified-fresh` | `tpatch verify` PASS again (record overwritten) |
| `verified-stale` | `verified-fresh` | `tpatch verify` PASS (with current world recorded) |
| `verified-stale` | `verify-failed` | `tpatch verify` FAIL |
| `verify-failed` | `verified-fresh` | `tpatch verify` PASS (with current world recorded) |
| `verify-failed` | `verify-failed` | `tpatch verify` FAIL again (record overwritten) |

Note that none of these "transitions" are observed at the `status.json` level except by the `tpatch verify` rewrites; the `verified-fresh ↔ verified-stale` flip in particular happens purely at read time.

The four labels compose orthogonally with M14.3 labels (`waiting-on-parent`, `blocked-by-parent`, `stale-parent-applied`). A child can carry any subset; rendering is `[m143-label, freshness-label]` in `tpatch status --dag`.

**Why this approach over a persisted state machine.**

- The first-revision state machine had to handle the "world moved" case via the parent-state hook, which (under any sound interpretation) required writes from a read path. The freshness overlay sidesteps this entirely.
- Read-time derivation is bounded by O(hard-parents) per `ComposeLabels` call — same cost as the existing M14.3 derivation. No new hot path.

### D6. Source-truth alignment with ADR-011 D6 / ADR-010 D5

The `Verify` sub-record lives in `status.json`; **never inferred from any other artifact**. Reusing ADR-011 D6 wording verbatim:

> Authoritative source for derived reconcile decisions: read `status.Reconcile.Outcome` via `store.LoadFeatureStatus` — never read `artifacts/reconcile-session.json` for DAG decisions. The session artifact is an audit record of one RunReconcile invocation; status.json is the source of current truth post-accept (see ADR-010 D5).

Applied to verify:

- `tpatch verify` reads `status.Reconcile.Outcome` (in V9 of the check list) — **never** `artifacts/reconcile-session.json` or `artifacts/resolution-session.json`.
- The freshness derivation reads `parent.State` via `store.LoadFeatureStatus` — never any reconcile artifact.
- The freshness record is written only to `status.json`. There is no `verify-session.json`, no new file in `artifacts/`, no new entry in `patches/`. Pass/fail per check is in the `--json` report on stdout; the persisted record is the `Verify` sub-record alone.

Enforced by an adversarial test: the verify implementation must not import or read `artifacts/reconcile-session.json` or `artifacts/resolution-session.json` at any code path; any reference in `internal/workflow/verify.go` is a test failure.

### D7. Verify is read-only on the working tree; closure-replay is the only side effect, in the shadow

Verify mutates **only** `status.json` (the `Verify` sub-record). Apply-simulation runs inside an existing `gitutil.CreateShadow` worktree (`internal/gitutil/shadow.go:56`) rooted at the upstream baseline; the shadow is pruned via `defer PruneShadow(...)` before verify exits, regardless of pass/fail.

**Closure-replay is the F1-mandated structural correction.** Before applying the target's recipe in the shadow, verify replays the target's hard-parent topological closure into the shadow, in order. Without this, V7/V8 are structurally meaningless for any non-leaf feature whose parents are locally `applied`: the shadow's baseline is the upstream tip, which does not contain the parent's changes; the target's recipe will fail to apply (op targets reference parent-created files; the patch references parent-modified hunks).

**Algorithm summary** (full spec in PRD §3.4.3):

1. Compute the hard-parent closure (only `DependencyKindHard` edges, transitively).
2. Order via `store.TopologicalOrder` (`internal/store/dag.go:107`) over the hard-only sub-DAG.
3. Skip parents in `upstream_merged` (their changes are already on the baseline).
4. Replay parents in `applied` — load each parent's `apply-recipe.json` and execute its ops in the shadow.
5. **Fail-fast** on first parent in any other state, or on first replay failure: verify aborts the V7/V8 phase, writes `Verify.Passed = false`, and emits `failed_at: "parent-replay"` with the failing parent slug in the JSON output.
6. Apply the target's recipe (V7) and `git apply --check` the target's `post-apply.patch` (V8) against the same shadow.
7. Prune.

**Per-slug shadow lock.** Verify and reconcile both write to `.tpatch/shadow/<slug>-<timestamp>/`. To prevent two concurrent writers, `verify` refuses when the lifecycle state is `reconciling` / `reconciling-shadow`. Per-slug only: verify on slug A while reconcile runs on slug B is allowed.

**Why closure replay is verify-only.** No other code path replays parent closures into shadows. `apply` works against the live tree (parents already applied locally). `reconcile` works against the upstream baseline + the target's own recipe (parents are out of band, by design — see ADR-010 D2). The closure-replay primitive lives in `internal/workflow/verify.go` only. If a future feature needs the same primitive, an ADR amendment factors it out.

**Cost.** O(closure size) shadow operations per verify. Bounded by DAG depth × per-recipe replay cost; comparable to a phase-2 reconcile op-replay pass per parent. Well within the cheap-budget for typical 1–3-deep DAGs.

> **Amended 2026-08-12 by Amendment 1 rev-2 (GH #8).** D7's machinery is
> retained in full — one shadow, topological replay, first-failure fail-fast,
> deferred prune, and the GH #2 reset between the recipe and the patch check.
> Amendment 1 refines three things.
>
> (a) The shadow's **root commit** becomes anchor-dependent: it stays `HEAD`
> for a feature with no landing evidence and becomes the **replay-anchor**
> commit's single parent in landed mode (D9/D14).
> `gitutil.CreateShadow` already accepts an arbitrary commit-ish, so this is a
> parameter change, not a new mechanism.
>
> (b) Step 3 of the algorithm above generalises to "skip members whose content
> is already present on the anchor", decided by the **non-mutating patch
> ladder** of D12/D13 — never by recipe replay and never by whole-file byte
> equality. `upstream_merged` is one case, a landed member is another, and an
> unattributed already-present member is a third.
>
> (c) A second, read-only **current-HEAD** assertion is added for landed
> features and folded into V8 (D11/D12). It allocates **no shadow** and reads
> **neither the working tree nor the real index**: it seeds a temporary index
> from the tree under test (`GIT_INDEX_FILE` + `git read-tree`) and runs
> `git apply --check --reverse --cached`, removing the temp index on every
> exit path. D7's read-only guarantee is therefore strengthened, not
> weakened — measured in A1.1 E23/E24.


## Consequences

**Positive**

- `FeatureState` enum stays single-axis-lifecycle. Every gate consumer reads `state` and gets a structurally meaningful answer.
- Read paths stay reads. F4-class problems are structurally precluded by routing freshness derivation through `composeLabelsFromStatus`.
- Closure replay makes V7/V8 structurally meaningful for non-leaf features. Verify is a useful tool across the entire DAG, not just leaves.
- Apply gate is zero-diff vs v0.6.1 — no behavioural change for existing users.
- D6 source-truth guard preserves ADR-011 D6 / ADR-010 D5 invariants.
- `omitempty` on the new `Verify` field gives byte-identical round-trip until first verify.

**Negative / accepted trade-offs**

- The `FeatureStatus` schema gains a field. Downstream JSON consumers that hard-code v0.6.1 schema-shape need to handle the omitempty case once first verify runs in any feature. CHANGELOG callout.
- Closure replay cost on deep DAGs scales with depth × per-recipe cost. Mitigated by harness pattern (verify parents first; rely on `verified-fresh` labels).
- Two simultaneous label systems (M14.3 + freshness) for `ComposeLabels` consumers to handle. Mitigated by orthogonal composition rules (PRD §3.5).
- Operator confusion: "I ran verify but the lifecycle state didn't change." Mitigated by skill bullet + CHANGELOG copy explaining the freshness-vs-lifecycle distinction.

**Neutral**

- `tpatch test` integration deferred (D3 future-work expansion).
- `tpatch verify --all` deferred to Slice D.
- `--fresh-branch` explicitly out of scope (PRD §0.3).
- Recipe-op JSON schema unchanged. Verify tolerates deletions in shadow replay the same way recipe autogen does.

## Alternatives considered

1. **`StateTested` as a `FeatureState` enum value** — rejected per D1. Conflates lifecycle with freshness; routes mutation through read paths to remain correct under drift.
2. **`Verify` as a top-level file under `.tpatch/<slug>/verify/`** — rejected per D1. Adds artifact lifecycle for no benefit; fights ADR-010 D5 / ADR-011 D6.
3. **Apply gate accepts `applied + verified-fresh` as a stronger form** — rejected per D2. Re-creates the demote-on-read problem at the gate level.
4. **Manual flip via `amend --state tested`** — rejected per D3. No `tested` state exists; the freshness record is machine-checkable only.
5. **Implicit `verify` after every `apply`** — rejected per D3. Inflates apply latency for a benefit the harness can opt into.
6. **`tpatch test` as a producer of the freshness record** — deferred to `feat-tested-state-test-producer`. Conflates user-test-suite-pass with tpatch-invariants-hold.
7. **A new `verify-session.json` artifact** — rejected per D6/D7. Verify writes only `status.json`.
8. **V9 (`reconcile_outcome_consistent`) as block severity** — left as PRD Q1; default warn.
9. **`--shadow` flag on verify** — rejected. V7/V8 already gate shadow allocation on recipe/patch presence.
10. **`--fresh-branch` flag on verify** — out of scope.
11. **Closure replay as a shared helper** — rejected. No other code path needs it; keeping it inside `internal/workflow/verify.go` avoids over-factoring.
12. **Skip-and-continue on parent-replay failure** — rejected per PRD §3.4.3. Skipping makes the V7 result meaningless (target's recipe applied against partial baseline).

## References

- `docs/prds/PRD-verify-freshness.md` — operational detail (second revision)
- `docs/adrs/ADR-011-feature-dependencies.md` — feature DAG contract; this ADR preserves D3 (composable labels), D6 (source-truth guard), D9 (config gate)
- `docs/adrs/ADR-010-provider-conflict-resolver.md` — shadow worktree contract (D2), artifact ownership split (D5)
- `docs/dependencies.md` — user-facing dep reference; will gain a one-paragraph cross-link to verify in Slice D
- CHANGELOG v0.6.1 → "Out of scope for v0.6.1" — names verify and tested as the two deferred items this PRD addresses
- `internal/store/types.go:91` — `FeatureStatus` struct (D1 field-add site)
- `internal/store/types.go:50–60` — `ReconcileLabel` vocabulary (D5 extension site)
- `internal/store/store.go:232` — `LoadFeatureStatus` (D6 source-truth read site; D5 derivation input)
- `internal/store/dag.go:107` — `TopologicalOrder` (D7 closure-replay ordering)
- `internal/store/validation.go:38–44, 101–108` — `satisfiedBySHA` regex + `gitutil.IsAncestor` reachability (V5 reuse)
- `internal/workflow/dependency_gate.go:79` — `CheckDependencyGate` (D2 anchor; explicitly NOT modified)
- `internal/workflow/labels.go:89` — `ComposeLabels` (D5 derivation site)
- `internal/workflow/labels.go:143` — `composeLabelsFromStatus` (D5 pure-function host; freshness derivation lives here)
- `internal/workflow/created_by_gate.go:57` — `checkCreatedByGate` (V3 reuse)
- `internal/gitutil/shadow.go:56` — `CreateShadow` (D7 reuse)
- `internal/gitutil/gitutil.go:828` — `IsAncestor` (V5 reuse; anchor re-validated 2026-08-12 during Amendment 1 — the pre-amendment `:680` citation had drifted)

---
# Amendment 1 (2026-08-12, **rev-2**) — Landed-evidence semantics — v0.15.1 Wave B / GH #8

**Status of this amendment**: proposed rev-2, AWAITING REVIEW. Binding on the
Wave C implementation once accepted. Adds D8–D17. **No prior decision D1–D7 is
reversed**; D7 is *extended* (its shadow mechanic is retained, its root commit
becomes anchor-dependent, and a second **index-isolated, worktree-free**
assertion is added that allocates no shadow).

**Issue**: <https://github.com/tesseracode/tesserapatch/issues/8>
**Co-amended**: `docs/prds/PRD-verify-freshness.md` §3.6 / §4.3.6–4.3.9 / §7.1;
`docs/prds/PRD-tpatch-land.md` §3.8 / §6.2.

**Revision history.**

- **rev-0** — omitted V10 entirely, and judged a landed feature only at
  current `HEAD` with byte-exact post-state predicates.
- **rev-1** — restored the eleven-check schema and introduced dual anchors,
  but (a) ran the current assertion against the **working tree**, (b) left the
  `-C0` hardening optional, (c) let an unavailable historical anchor degrade
  to a skip-and-pass, (d) reused whole-file byte equality and a circular
  reference to replay in parent arbitration, and (e) derived V10's later-touch
  from current byte differences rather than the shipped ADR-029 metadata
  detector.
- **rev-2 (this revision)** — closes all eight rev-1 findings. Anchor C is
  **index-isolated** (D11); the `(0/0)` hardening is **mandatory and blocking**
  (D12); an unavailable historical anchor is **terminal** (D14); the
  attestation candidate and the replay-anchor candidate are **separated**
  (D14); parent arbitration is **non-mutating and patch-ladder-only** (D13);
  V10 uses the **shipped `RequestedAt` + touched-path detector** (D15).

## A1.0 Context — what GH #8 actually is

`tpatch land` (v0.8.0, ADR-019) commits the feature into reachable Git history
while intentionally leaving `status.apply.base_commit` unchanged
(`docs/prds/PRD-tpatch-land.md` §3.6; `internal/cli/land.go:394`). `verify`
allocates its V7/V8 shadow from **current HEAD**
(`internal/workflow/verify.go:1012`, `:1024`). After `land`, HEAD already
contains the feature, and the forward-apply semantics V7/V8 were designed
around stop describing the world.

Measured with the real CLI at `13a885c`; every run reported `checks=11` — the
shipped set is **V0–V10** (`internal/workflow/verify.go:49-71`), V10 appended
last (`internal/workflow/verify.go:288-289`):

| Target recipe op kind | pre-land V7/V8 | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file`      | PASS / PASS | **PASS — false green** (`internal/workflow/verify.go:1290-1294`) | **FAIL — false red** |
| `replace-in-file` | PASS / PASS | **FAIL — false red** (`search text not found`, `internal/workflow/verify.go:1295-1305`) | SKIP |
| `append-file`     | PASS / PASS | **PASS — false green, shadow double-appended** (`internal/workflow/verify.go:1306-1313`) | **FAIL — false red** |

The defect is not V8-only, and the same double-apply hazard applies to a
landed hard parent (`internal/workflow/verify.go:1048-1091`).

## A1.1 Empirical basis (read-only probes, git 2.55.0 / macOS, scratch removed)

E1–E22 were measured for rev-0/rev-1 and still hold. rev-2 adds E23–E33 and
**retracts** one rev-1 over-claim (see E27).

| # | Observation | Consequence |
|---|---|---|
| E1 | `git apply --check --reverse` succeeds at any tree containing equivalent content, including one produced by an unrelated actor. | Reverse-apply is a materialization signal only, never ownership proof. (D10) |
| E2 | `git log --grep '^Tpatch-Feature: <slug>$'` matches a commit whose prose body quotes that line; `%(trailers:…)` returns empty for it. | `--grep` is a prefilter; the parsed block is authority — but see E31. (D10) |
| E3 | Git parses trailers only from the last paragraph; a `--amend` appending prose empties `%(trailers:…)` while raw `%B` keeps the line. | Trailer loss is `malformed`, and only the raw message can classify it. (D10) |
| E4 | `--grep '^Tpatch-Feature: my-slug$'` does not match `my-slug-extended`. | Exact-value slug matching. (D10) |
| E5 | `cherry-pick`/`rebase` copy the trailer block verbatim; SHA and parent change; `Tpatch-Base-Commit` may become unreachable. | Evidence keys on trailer values, never on the landing SHA or its parent identity. (D10/D14) |
| E6 | A single commit can carry several `Tpatch-Feature` values. | Cardinality must be decided. (D10) |
| E7 | `git revert` of a landing does not copy trailers; the landing stays reachable with the content gone. | Reachability ≠ materialization. (D12) |
| E8 | One `git log -z --format=…` yields SHA, `%P`, raw `%B` and all four parsed trailers. | One enumeration per run. (D17) |
| E9 | A landing merged from a side branch is reachable only through the non-first parent. | Full-graph reachability, never `--first-parent`. (D10) |
| E10 | `RecipeFromPatch` emits `{type,path,content}` — **no `preimage_hash`** (`internal/workflow/recipe_autogen.go:114-118`). | Every autogenerated recipe takes the ADR-029 D4 legacy path. (D15) |
| E11 | With a genuine `preimage_hash`, **V10 already fails for an `applied`, un-landed feature**: `expected preimage sha256:5fb14…, observed sha256:fa6dd8…`. | V10's live-tree reference is wrong for any applied feature. (D15, residual) |
| E12 | Same for `preimage_hash: ""` (new-file): after apply the file exists ⇒ `new-file collision`. | Both preimage shapes break post-apply. (D15) |
| E13 | `checkWriteFilePreimage` reads from `repoRoot` — the live working tree (`internal/workflow/writefile_safety.go:108-112`). | V10 must be re-anchored in landed mode. (D15) |
| E14 | Exhaustive enumeration over alphabet `{a,b,X}`, preimages ≤ 7 chars, contents ≤ 5, all 1–2-char `search`/`replace`: the rev-0 `replace-in-file` round-trip predicate gave **204 false reds and 15 933 false greens** across 56 784 decided cases. | The rev-0 predicate is unsound in both directions. |
| E15 | The existential-inverse predicate over **every** occurrence of `R` gave **0 false reds and 0 false greens** over the same 52 416 decided cases, with 4 368 correctly reported **undecidable** (`R == ""` or `S == ""`). | Machine-verified sound on the enumerated domain; retained for diagnostics. |
| E16 | Go semantics: `strings.Replace("abc","","Z",1) == "Zabc"`; `strings.Contains(x,"") == true`. | `search == ""` is a degenerate op; `replace == ""` cannot attest. |
| E17 | Duplicate trailers are observable but ambiguous: two `Tpatch-Patch-SHA` lines yield `aaaa,bbbb`; two `Tpatch-Recipe-SHA` yield `none,cccc`. | Strict cardinality is mandatory; no "take the first". (D10) |
| E18 | Git trailer keys are **case-insensitive**: `tpatch-feature: lowerfeat` is returned by `%(trailers:key=Tpatch-Feature,valueonly)`. | The reader inherits git's case-insensitivity; the contract states it. (D10) |
| E19 | `land` reads the canonical `artifacts/post-apply.patch` for `Tpatch-Patch-SHA` and returns the literal `none` from `readRecipeSHA` on both a read error and `strings.TrimSpace(...) == ""`. | `none` covers absent **and** whitespace-only recipes. (D10) |
| E20 | Pre-land the real CLI reported `checks=11` on every run, with `write_file_preimage_fresh` last. | The shipped schema is eleven checks. (D8) |
| E21 | A landed hard parent + an unlanded applied child passes today only by accident — the parent's `write-file` replay is an idempotent no-op. | Parent arbitration must not rely on op-kind luck. (D13) |
| E22 | `git apply --check --reverse` at `status.apply.base_commit` fails forward while succeeding in reverse at HEAD, reproducing the issue report exactly. | The GH #8 reproduction is baseline anchoring, not generation. (A1.0) |
| **E23** | **A dirty worktree produces a false red.** With the feature reverted in the worktree only (HEAD unchanged), `git apply --check --reverse` **FAILS** while `GIT_INDEX_FILE=<tmp> git read-tree HEAD` + `git apply --check --reverse --cached` **succeeds**. | Anchor C must never read the worktree. (D11) |
| **E24** | A temp index placed under `$(git rev-parse --git-dir)` leaves the **real index byte-identical**, the **worktree byte-identical**, and is **invisible to `git status --porcelain`**; `rm` cleans it up completely. A temp index placed inside the working tree appears as an untracked entry. | Exact placement and cleanup rules. (D11) |
| **E25** | `GIT_INDEX_FILE=<tmp> git read-tree <arbitrary-tree>` followed by `git apply --check --reverse --cached` probes **any** tree with **zero** worktree or real-index mutation. | Anchor selection can probe candidate parents without a checkout. (D14) |
| **E26** | Hardened ladder over trees, 3-hunk patch, 60-line file (`C3` / `C0` / count of `Context reduced to (0/0)` under `LC_ALL=C`): pristine `OK/OK/0`; landing parent `FAIL/FAIL/0`; 10 lines prepended `OK/OK/0`; unrelated edit far away `OK/OK/0`; unrelated edit **2 lines** away `FAIL/OK/0`; unrelated edit **1 line** away `FAIL/OK/**1**`; partial revert of hunk 1, of hunk 2, of hunk 3, and of hunks 1+3 all `FAIL/FAIL/0`; full revert `FAIL/FAIL/0`; degenerate whole-file hunk + header/footer `FAIL/OK/0`; **revert-in-place + identical text pasted at EOF `FAIL/OK/**1**`**; patched file deleted `FAIL/FAIL/0`. | The `(0/0)` count is the exact discriminator for the rev-1 hole. (D12) |
| **E27** | **Retraction.** rev-1 asserted that reverts "fail at every level". Measured, reverts fail at `C3` and `C0`, but the sentence generalised beyond what was tested and is withdrawn. The per-scenario triple in E26 is the only claim rev-2 makes. | No generalised "every level" language. (D12) |
| **E28** | Randomized 220-tree corpus, 3-hunk patch in an 80-line file, hardened rule (`C3` pass ⇒ clean; `C0` pass **and** zero `(0/0)` ⇒ warn; otherwise block): **0 false greens over 69 postimage-absent trees**, **26 false reds over 151 postimage-present trees**, every false red carrying exactly one `(0/0)`. | The hardening is sound-by-measurement; the cost is quantified. (D12) |
| **E29** | The **unhardened** rev-1 rule (any `C0` pass ⇒ warn) leaked **2 false greens over the same 69 absent trees**. | Q14 cannot stay optional. (D12) |
| **E30** | After a re-record + re-land, the newest landing `L2`'s parent tree **already materializes** the current canonical patch (`C3=OK`), so `L2` cannot supply a replay baseline, while the earlier landing `L1`'s parent does **not** (`C3=FAIL`) and therefore can — even though `L1`'s own `Tpatch-Patch-SHA` is stale. | The attestation candidate and the replay-anchor candidate are different objects. (D14) |
| **E31** | A commit whose prose body quotes `Tpatch-Feature: <slug>` yields an empty parsed value; the raw `%B` still contains the line. There is no signal that separates "prose quote" from "trailer block destroyed by an amend". | A conservative reader must classify **both** as `malformed`; the prose false-red is accepted deliberately. (D10) |
| **E32** | `git log --topo-order --reverse -z --format='%H%x1f%P%x1f…%x1f%B'` emits oldest-first records carrying parent count, all four trailers **and** the raw body in a single invocation. `rev-list` cannot emit `%B` and is not needed. | One `git log`, no `rev-list`. (D17) |
| **E33** | Topology: a root landing has **0** parents and `git read-tree <root>^` fails with `fatal: Not a valid object name`; a merge landing has **2** parents and its trailer parses normally. | Root/merge ⇒ `unsupported-topology`, never an implicit `^`. (D16) |

---

## Decision (Amendment 1 rev-2)

### D8. The check set is eleven checks, V0–V10; no identifier changes

`internal/workflow/verify.go:49-71` defines eleven check IDs; V10 is
`write_file_preimage_fresh`, appended last (`:288-289`). Every report shape,
golden example and acceptance row in this amendment describes **eleven** rows
in that order. No check is added or removed: the current-materialization
assertion GH #8 needs is folded into **V8**, whose subject is exactly the
artifact the landing trailer attests.

### D9. Baseline model: dual-anchor landed verification

| Model | Verdict |
|---|---|
| HEAD-only post-state predicates (rev-0) | **Rejected.** For `write-file` — the shape every autogenerated recipe has (E10) — the only HEAD-evaluable predicate is byte equality, which false-reds on any later unrelated edit; weakening it makes V7 an alias of V8. |
| Replay at `status.apply.base_commit` | **Rejected.** Owned by `record`/auto-base (ADR-016), operator-mutable, possibly unreachable, unrelated to the attestation. |
| **Dual anchor** | **Chosen.** |

- **Anchor H (historical) — "are the artifacts internally coherent?"** A
  shadow rooted at the **replay-anchor** commit's single parent (D14). Closure
  arbitration (D13) runs, then the existing machinery: V7 replays the target's
  recipe (`internal/workflow/verify.go:1275`), the shadow is reset to
  `closureBaselineTree`, V8 runs `git apply --check` forward
  (`:1092`, `:1143`, `:1156`), V10 evaluates preimages (D15).
- **Anchor C (current) — "is it still there?"** An **index-isolated**
  assertion at `HEAD` (D11) running the hardened ladder (D12). No shadow, no
  worktree read, no real-index read.

**V7's independent obligation, stated.** V7 at anchor H proves the recipe
still forward-applies to the tree it was authored against, with its closure
reconstructed. That is a different fact from "the canonical patch's postimage
is present at HEAD", which is anchor C's job, and from "the patch applies at
the landing baseline", which is V8's historical half. V7 never aliases V8.

**GH #2 invariant, binding in every mode.** The recipe and the patch are
validated independently against the same baseline tree with a shadow reset
between them (`internal/workflow/verify.go:1092`, `:1143`). Normative:
*any check that may mutate the shadow MUST reset it to `closureBaselineTree`
before the next check runs; V7's result is never an input to V8's tree.*
Anchor C mutates nothing and allocates no shadow, so it cannot disturb this.

**Non-landed features are untouched**: evidence `none` ⇒ shadow at `HEAD`,
V7/V8/V10 byte-for-byte as today.

### D10. Evidence reader: one enumeration, raw **and** parsed, conservative grammar

**Enumeration.** Exactly one
`git log --topo-order --reverse -z --format='%H%x1f%P%x1f<four trailers>%x1f%B'`
invocation per verify run, over commits reachable from the resolved `HEAD`,
**cached and reused for every feature** of a `verify --all` run (E32). Records
arrive **oldest-first**. `rev-list` is **not** used — it cannot emit `%B`.
Never `--first-parent` (E9).

**Conservative raw precedence (rev-2).** `--grep '^Tpatch-Feature: '` may be
used as a cost prefilter, but the classification rule is:

> Any commit whose **raw** message contains a line that is exactly
> `Tpatch-Feature: <slug>` (after trimming trailing ASCII space/tab) for the
> slug under test, but whose **parsed terminal trailer block** does not yield
> that slug, is **`malformed`** — never `none`.

E31 measured that a prose quotation and an amend-destroyed trailer block are
**indistinguishable** from the outside. rev-2 therefore accepts a deliberate
false-red on the prose case rather than risk classifying a destroyed
attestation as "no attestation". This is the conservative direction: the
operator sees a `malformed` diagnostic naming the commit and can amend or
re-land.

**Grammar (normative).**

| Element | Rule |
|---|---|
| Key case | Git matches trailer keys **case-insensitively** (`tpatch-feature:` parses). The reader inherits this and the contract says so. |
| `Tpatch-Feature` cardinality | **Exactly one value.** `land` emits exactly one (`internal/cli/land.go:397-400`). Two or more (E6) ⇒ `malformed`: the sibling SHA trailers cannot be attributed to a slug. |
| `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` / `Tpatch-Base-Commit` | **Exactly one each.** Zero or ≥2 ⇒ `malformed`. No "take the first". |
| Slug match | Exact equality after trimming leading/trailing ASCII space and tab. Never prefix, never substring. |
| Formats | Patch-SHA: 64 lowercase hex. Recipe-SHA: 64 lowercase hex **or** literal `none`. Base-Commit: 40 lowercase hex. Otherwise `malformed`. Follows the ADR-029 D1 precedent enforced by `isLowercaseHex` (`internal/workflow/writefile_safety.go:176`). |
| Reader failure | Git error, unparsable output, or a git below the D17 floor ⇒ **`unavailable`**, a block failure, distinct from `none` and from `malformed`. |

**Artifact-absence precedes digest mismatch.** Validation is ordered:

1. If the member's canonical patch is **absent** from the snapshot, the state
   is `landed-artifacts-absent`-eligible (D13) — it is *not* reported as a
   digest mismatch, because there is no digest to compare.
2. Only when the artifact is **present** is the digest compared. A
   present-but-zero-byte patch hashes to `sha256("")` and must compare equal.
   **Absent ≠ empty.**
3. `Tpatch-Recipe-SHA: none` matches both an absent recipe and a
   whitespace-only one, mirroring `readRecipeSHA`
   (`internal/cli/land.go:1034-1043`).

Because `land` refuses when the embedded `record` would capture nothing, a
landed member with an absent or zero-byte patch is a **corruption or
hand-edit** case; the contract classifies it explicitly rather than implying
it is routine.

**Evidence states — closed set of eight:** `none`, `exact`,
`duplicate-equivalent`, `stale`, `ambiguous`, `malformed`,
`unsupported-topology`, `unavailable`. Only `none` degrades to forward mode;
the other six non-`exact` states are terminal failures.

### D11. Anchor C is index-isolated: a temp index seeded from HEAD, never the worktree

rev-1 ran the current assertion with a bare `git apply --check --reverse`,
which reads the **working tree**. E23 measured the consequence: with the
feature reverted in the worktree only, the check **false-reds** a perfectly
healthy landed feature; the symmetric dirty-index case is equally unsound.

**Normative implementation.**

1. Create a temporary index file at a path under
   `$(git rev-parse --git-dir)` (E24: invisible to `git status`, real index
   untouched) — or, equivalently, under the already-gitignored `.tpatch/local/`
   root (`internal/cli/land_journal.go:31`, `:60`;
   `internal/gitutil/ignore.go:50-51`). It **must not** be created inside the
   tracked working tree.
2. `GIT_INDEX_FILE=<tmp> git read-tree <tree-ish>` where `<tree-ish>` is
   `HEAD` for the current assertion, or an arbitrary candidate tree for the
   D14 anchor probe (E25).
3. `GIT_INDEX_FILE=<tmp> git apply --check --reverse --cached [-C0 --verbose] <patch>`.
4. Remove the temp index on **every** exit path, including every failure path,
   in a deferred cleanup.

**Read-only guarantees (measured, E24).** The real index is byte-identical
before and after; the working tree is byte-identical; `git status --porcelain`
output is unchanged; the temp index never appears as an untracked entry. The
probe therefore satisfies D7's read-only rule without any of rev-1's
worktree coupling.

**The ladder result is cached per (tree, patch) pair** for the duration of a
run, so the same tree is never probed twice.

### D12. The hardened ladder — `(0/0)` blocks, and Q14 is now mandatory

**Accurate statement of what reverse-apply proves.** `git apply --check
--reverse --cached` asserts that the patch's **postimage hunks are present in
the given tree**, matched by content with a line-offset search and a
configurable context requirement. It is **not** a byte-exact tree comparison
and **not** ownership evidence.

**The rule (normative).** All three steps run against the D11 temp index.

| Step | Command | Outcome |
|---|---|---|
| 1 | `git apply --check --reverse --cached <patch>` (default context) | pass ⇒ **materialized, clean** |
| 2 | on step-1 failure: `LC_ALL=C git apply --check --reverse --cached -C0 --verbose <patch>` | pass **and zero** `Context reduced to (0/0)` occurrences ⇒ **materialized, context drift** — the block check **passes** and a `warn`-severity `context-drift` advisory is raised |
| 3 | step 2 passes **but reports one or more** `Context reduced to (0/0)`, **or** step 2 fails | **BLOCK** — `landed-content-absent` |

`LC_ALL=C` is **mandatory** so the `(0/0)` marker is locale-stable.

**Advisory vocabulary.** The ladder's step-2 pass raises the `warn`-severity
advisory code `context-drift`. The full closed advisory set shared with the
PRD is `context-drift`, `later-touch`, `unattributed-materialized` and
`base-commit-unreachable` (the D10 `base_commit_reachable: false` signal);
none of them flips `passed`. Exact JSON shape: `PRD-verify-freshness`
§4.3.6–4.3.9.

**Why `(0/0)` blocks — measured.** E26 shows the marker fires on exactly two
shapes: the rev-1 false-green (revert-in-place plus identical text pasted
elsewhere) and an unrelated edit exactly one line from a hunk. E28/E29
quantify the trade over a randomized 220-tree corpus:

| Rule | false greens (69 absent trees) | false reds (151 present trees) |
|---|---|---|
| rev-1, `(0/0)` ignored | **2** | 0 |
| rev-2, any `(0/0)` blocks | **0** | 26 |

The reviewers' direction was explicit: prefer blocking. A false green tells an
operator a feature is healthy when its content is gone; a false red tells them
to look, and the remediation is a real action — the recorded context genuinely
no longer matches HEAD, so `tpatch record` + `tpatch land` re-attests and
restores a clean pass. **A stronger hunk-local corroboration was considered
and not adopted**, because none was found that could be *proved* on the
measured corpus; inventing an unproven discriminator is exactly what the
reviewers rejected.

**No generalised claims (E27).** rev-1's "fails at every level" sentence is
withdrawn. The only claims rev-2 makes about reverts are the per-scenario
`C3`/`C0`/`(0/0)` triples in E26, which include **four distinct partial-revert
shapes** (hunk 1, hunk 2, hunk 3, hunks 1+3) and the full revert, all
`FAIL/FAIL/0` ⇒ block.

### D13. Parent arbitration is non-mutating and patch-ladder-only

rev-1 decided closure membership partly by "total materialization" language
that circularly referred to replay and reused whole-file byte equality. rev-2
replaces it.

**The presence test for any closure member is the D12 hardened ladder applied
to that member's canonical `post-apply.patch` against the anchor tree, probed
through the D11 temp index.** It is:

- **non-mutating** — no shadow write, no recipe execution, no worktree touch;
- **not** recipe replay — replay is what arbitration *decides about*, so it
  can never be the deciding test;
- **not** whole-file byte equality — that is the rev-0 defect.

**Recipe operation predicates are diagnostics only.** They localise *which*
operation and path a failure concerns, and they feed the D15 write-file
signals. They never certify presence and never, on their own, cause a member
to be skipped. The `write-file` later-touch dimension follows ADR-029 (D15).

**Arbitration table.**

| Member condition | Action |
|---|---|
| `upstream_merged` | **skip** (unchanged, `internal/workflow/verify.go:1062-1064`) |
| superseded by an active superseder | **skip** (unchanged, `internal/workflow/verify.go:976-983`) |
| evidence `exact`/`duplicate-equivalent`, patch present, ladder ⇒ clean or context-drift | **skip**; the member is on the anchor |
| evidence `exact`/`duplicate-equivalent`, patch present, ladder ⇒ block | **fail-fast** `parent-landing-drift` |
| evidence `exact`/`duplicate-equivalent`, patch **absent** or zero-byte, recipe present with ≥1 op | the **recipe** is the sole authority: replay decides, and a replay failure is `parent-landing-drift`. Corruption case (`land` cannot produce it). |
| evidence `exact`/`duplicate-equivalent`, **both** artifacts absent (or recipe zero-op and patch absent) | **fail-fast** `landed-artifacts-absent` — never skipped, never replayed, never assumed materialized |
| evidence `none`, patch present, ladder ⇒ clean or context-drift | **skip**, with a mandatory `warn` `unattributed-materialized` advisory naming the member. Verify explicitly claims **no ownership** of that content. |
| evidence `none`, patch present, ladder ⇒ block | **replay** (unchanged, `internal/workflow/verify.go:1065-1082`) |
| evidence `none`, patch absent | **replay** |
| evidence `stale` / `ambiguous` / `malformed` / `unsupported-topology` / `unavailable` | **fail-fast** `parent-evidence-integrity` |
| `unapplied` (ADR-032) | **fail-fast** `parent-unapplied` |
| `rejected` (ADR-031) | **fail-fast** `parent-rejected` |
| any other state | **fail-fast** (unchanged `default:`) |

**A landed `exact` member additionally contributes its applicable V7/V8 and
V10 results.** Skipping on the ladder settles *presence*; it does not excuse
the member from the V10 metadata evaluation of D15, whose block-class outcome
(a preimage mismatch at the anchor) participates in `parent-landing-drift`
and whose warn-class outcome (a later-touch) is advisory only.

**Revert timing is qualified.** "Reverted" means: the member's canonical patch
fails the D12 ladder **at the anchor tree being built**. A revert that lands
*after* the anchor commit is invisible at anchor H and is caught at anchor C;
a revert that predates the anchor makes the member's content absent from the
anchor and is caught there. The two anchors are evaluated independently and
both are reported.

**`active` is total.** `active` is treated **identically to `applied`**
everywhere in the closure. Today the switch handles only `upstream_merged` and
`applied`, so an `active` hard parent reaches `default:` and fail-fasts
(`internal/workflow/verify.go:1061-1089`) — while `CheckDependencyGate`
accepts both (`internal/workflow/dependency_gate.go:79-81`),
`postApplyVerifyStates` admits `active`
(`internal/workflow/verify.go:127-134`) and `isPostApplyState` does too
(`internal/workflow/verify_all.go:89-97`). Widening the switch is the smallest
change that makes all four sites agree. It is a **deliberate behaviour change
for non-landed features** and carries its own acceptance rows and risk row.

### D14. Attestation candidate and replay-anchor candidate are different objects

rev-1 conflated them, which meant a re-record + re-land could permanently
destroy anchor H. E30 measured exactly that: the new landing's parent already
contains the feature, so it cannot supply a baseline, while the earlier
landing's parent can.

**Attestation candidate** — determines `landing_evidence.state`. It must be a
well-formed, single-`Tpatch-Feature`, exact-slug commit whose three recorded
values match the current artifact snapshot (D10). This is the **authority**
and the only thing `state: "exact"` refers to.

**Replay-anchor candidate** — supplies anchor H's root and nothing else. It
must be:

1. reachable from `HEAD`;
2. carrying exactly one `Tpatch-Feature` value equal to the slug, and a
   parseable terminal trailer block (its **hashes may be stale** — it is not
   an authority);
3. **single-parent** (`%P` cardinality exactly 1);
4. a commit whose **parent tree does not already materialize** the current
   canonical patch, probed with the D11 temp index and the D12 ladder at that
   parent tree (`read-tree <parent-tree>` + `apply --check --reverse --cached`).

**Selection is deterministic**: iterate the single enumeration in its native
`--topo-order --reverse` (oldest-first) order and take the **first** candidate
satisfying 1–4. Tie-break — which cannot arise given a total order but is
stated for completeness — the lexicographically smallest full SHA. **No
broadening**: the search is confined to exact-slug trailer commits; it never
falls back to "any commit that looks like it introduced the paths".

**Ambiguity.** If two or more candidates satisfy 1–4 and their
`git diff <C>^ <C> -- <patch path set>` bytes differ, the anchor is
**ambiguous** and treated as unavailable (terminal, below). If the bytes are
identical, the first in topo order is used and `duplicates` is recorded.

**Unavailability is TERMINAL.** If no candidate satisfies 1–4, the run fails
with `failed_at: "historical-anchor-unavailable"`. V7, V8's historical half
and V10 are reported as **failed-because-unanchored**, not skipped, and the
run **never** passes on anchor C alone. rev-1's skip-and-pass is withdrawn:
an unverifiable historical half is an unverified feature.

**Re-land remediation regains anchor H or fails loudly.** The R5 remediation
for `stale` evidence instructs `tpatch record` + `tpatch land`. After that,
the new landing is the attestation candidate and — per E30 — the *earlier*
landing remains a valid replay anchor, so anchor H is regained. If the
operator's history is such that no candidate qualifies, the run fails with
`historical-anchor-unavailable` rather than silently degrading. Both branches
are pinned by acceptance rows.

**Duplicate-equivalence of attestation candidates** is unchanged in spirit and
now fully specified: path set `P` from `gitutil.FilesInPatchStrict`
(`internal/gitutil/patch_paths_strict.go:253`), sorted byte-wise; if `P` is
empty the candidates are **not comparable** ⇒ `ambiguous` (no broadening);
otherwise compare the raw bytes of
`git diff --no-color --no-ext-diff --no-textconv --binary --no-renames --unified=3 <C>^ <C> -- <P…>`.

### D15. V10 uses the shipped ADR-029 later-touch detector

**Historical half.** In landed mode, each `write-file` op's `preimage_hash` is
evaluated against the **anchor-H closure baseline** — the shadow tree after
closure arbitration and *before* the target's recipe replays. That tree is, by
construction, the preimage the field describes. This resolves E11/E12 for
landed features.

| Case | Outcome |
|---|---|
| `preimage_hash` absent | legacy pass, no re-warn (unchanged, ADR-029 D4, `internal/workflow/verify.go:879-883`) |
| present and matching at the anchor-H baseline | **PASS**, `mode: "historical-anchor"` |
| present and **not** matching at the anchor-H baseline | **FAIL**, block severity — the recipe is stale or destructive relative to its own baseline. Downgraded to `warn` when superseded (unchanged, ADR-029 D7, `internal/workflow/verify.go:862-870`). |
| V2 skipped or failed | **skip**, unchanged reason (`internal/workflow/verify.go:853-861`) |
| anchor H unavailable | **fail** with `historical-anchor-unavailable` (D14) — not a skip, and never a live-tree fallback |

**Current half — later-touch, from metadata, not bytes.** rev-1 derived
later-touch from "the path's content at HEAD differs from the landing's
postimage", which is a byte comparison that fires on the operator's own
manual edits and on unrelated formatting. rev-2 uses the **shipped detector**:

- ordering is by `RequestedAt` — a feature is *later* iff its `RequestedAt` is
  non-empty and strictly greater than the current slug's
  (`internal/workflow/writefile_safety.go:409-442`);
- "touched" is the path-level union of
  `patch-generations.json.touched_paths` (`internal/store/patch_generations.go:52`)
  and the feature's `apply-recipe.json` operation paths
  (`internal/workflow/writefile_safety.go:449-481`);
- the index is `path → first later slug`, alphabetically-first for
  determinism (`internal/workflow/writefile_safety.go:380-388`);
- the per-op predicate is `checkLaterTouch`
  (`internal/workflow/writefile_safety.go:489-498`), and the exported
  record-time entry point is `DetectRecordLaterTouchWarnings`
  (`internal/workflow/writefile_safety.go:571`).

A later-touch hit on a landed feature's `write-file` path raises a
**`warn`-severity `later-touch` advisory** and **never blocks** — ADR-029 D6
("Record-time later-touch detection is warning-class in v1") and D5
("`verify`: stale preimages on effective features fail") together mean the
*preimage* gate blocks and the *later-touch* signal warns. The single
exception, stated explicitly: if the baseline/preimage contract is itself
invalid — a malformed `preimage_hash` per ADR-029 D1, or a mismatch at the
anchor — that is a block on its own terms, independent of any later-touch.

**Parent V10 aggregation.** For each closure member, V10 is evaluated with the
same two halves. The member's **block-class** outcome (preimage mismatch at the
anchor, or a malformed `preimage_hash`) contributes to `parent-landing-drift`
for that member. The member's **warn-class** outcome (later-touch) is
aggregated into the run's advisory list, attributed to the member's slug, and
never affects any verdict. The target's own V10 row reports
`mode: "historical-anchor"` and carries the target's block-class result; parent
results appear only as advisories plus the fail-fast reason.

### D16. Topology

A replay-anchor candidate must have **exactly one** parent. E33 measured that
a root landing has zero parents and `git read-tree <root>^` fails outright,
and that a merge landing has two while its trailer parses normally. Candidates
with 0 or ≥2 parents are classified **`unsupported-topology`**; `^1` is never
used as an approximation. Reachability is full-graph (E9); merge commits are
candidates only if they themselves carry the trailer.

**Rebase / cherry-pick / branch switch / detached HEAD / rewrite.** Trailers
survive rebase and cherry-pick verbatim while SHA and parent change (E5), so
evidence keys on trailer *values*; both classify `exact` with a possibly-false
`base_commit_reachable` advisory. A branch switch that removes the landing
yields `none` ⇒ forward mode. A detached `HEAD` is evaluated identically from
whatever `HEAD` resolves to. A rewrite leaving no reachable landing yields
`none`; one leaving two is decided by the D14 rules.

### D17. Snapshots, invocation accounting, and implementability

**Immutable snapshot.** At the start of a run verify captures, once, for the
target and every closure member: the decoded `FeatureStatus`, and the
**presence flag** plus **raw bytes** of `artifacts/apply-recipe.json` and
`artifacts/post-apply.patch`. Every later stage — evidence digests, V7, V8,
V10, the persisted `VerifyRecord`, the derived labels — consumes **copies from
that snapshot** and never re-reads disk. Empty-present is distinct from absent
at every consumer. Before the report is finalised each snapshotted artifact is
re-read and compared; a difference ⇒ **FAIL** `failed_at: "snapshot-unstable"`
naming the path. Verify never mixes bytes from two points in time.

**Honest invocation budget.**

| Purpose | Invocations |
|---|---|
| Evidence enumeration — `git log --topo-order --reverse -z --format=…` incl. `%P` and `%B` | **1 per run**, cached across `verify --all`. No `rev-list`. |
| Shadow allocation at anchor H | 1 `CreateShadow` (already allocated today; only its commit-ish changes) |
| Temp-index seed | 1 `git read-tree <tree-ish>` **per distinct tree probed** |
| Ladder | 1 `git apply --check --reverse --cached`, plus 1 `-C0 --verbose` when step 1 fails, **per (tree, patch) pair**, memoised |
| Replay-anchor selection | one seed + ladder per candidate examined, in topo order, stopping at the first qualifying candidate |
| Duplicate-equivalence | 1 `git diff` per candidate, **only when ≥2 attestation candidates** |
| `base_commit_reachable` advisory | 1 `git merge-base --is-ancestor` per landed member |

Everything else reuses shipped primitives: `gitutil.HeadCommit`
(`internal/gitutil/gitutil.go:14`), `CreateShadow`/`PruneShadow`
(`internal/gitutil/shadow.go:56`), `gitutil.IsAncestor`
(`internal/gitutil/gitutil.go:828`), `store.TopologicalOrder`
(`internal/store/dag.go:107`), `isFeatureSupersededIn`
(`internal/workflow/verify.go:976`), `sha256Hex`
(`internal/workflow/verify.go:498`), `FilesInPatchStrict`,
`checkWriteFilePreimage` (`internal/workflow/writefile_safety.go:108`),
`loadLaterFeatureTouches` / `checkLaterTouch`
(`internal/workflow/writefile_safety.go:409`, `:489`). **No new store field,
no new artifact, no schema migration, no new dependency, no new check ID.**

New code is one generic reader in `internal/gitutil/` (candidate
`trailers.go`) returning raw **and** parsed records, plus a small temp-index
helper; **policy** stays in `internal/workflow/verify.go` per D7.

**Git floor.** `%(trailers:key=…,valueonly)` needs git ≥ 2.22 and `separator=`
needs ≥ 2.25; verified on 2.55.0. Below the floor the reader **fails** ⇒
evidence `unavailable` ⇒ block, never `none`.

---

## Amendment 1 rev-2 — the `replace-in-file` predicate (diagnostic use only)

Under D13 this predicate no longer decides presence; it localises diagnostics.
It is retained, and its soundness still matters for the messages it produces.

For content `c`, search `S`, replacement `R`:

- `S == ""` ⇒ **unsupported**. `strings.Replace(x,"",R,1)` inserts at the
  start, so the operation is malformed.
- `R == ""` ⇒ **undecidable**. Every `c` admits the preimage `S+c`, so the
  predicate attests nothing; the judgement defers to patch authority.
- otherwise ⇒ **true iff there exists an index `i` at which `R` occurs in `c`
  such that `pre := c[:i] + S + c[i+len(R):]` satisfies
  `strings.Replace(pre, S, R, 1) == c`.** Every occurrence of `R` is tried.

Exhaustive enumeration (alphabet `{a,b,X}`, preimages ≤ 7 chars, contents ≤ 5,
all 1–2-char `S`/`R` plus `R == ""`):

| Predicate | decided | undecidable | false reds | false greens |
|---|---|---|---|---|
| rev-0 round trip | 56 784 | 0 | **204** | **15 933** |
| rev-1/rev-2 existential inverse | 52 416 | 4 368 | **0** | **0** |

The other op kinds, used only for diagnostics and never to certify:
`write-file` ⇒ bytes equal `op.Content`; `append-file` ⇒ content ends with
`op.Content`, and an **empty** `op.Content` is **undecidable** rather than a
vacuous pass; `ensure-directory` ⇒ the path exists and is a directory; unknown
type ⇒ unsupported (unchanged, `internal/workflow/verify.go:1316`).

## Amendment 1 rev-2 — alternatives considered and rejected

1. **HEAD-only post-state predicates (rev-0)** — false-reds every landed
   `write-file` feature after any later edit; V7 aliases V8.
2. **Replay at `status.apply.base_commit`** — operator-mutable, possibly
   unreachable, unrelated to the attestation.
3. **Anchor C against the working tree (rev-1)** — E23: dirty worktree
   produces a false red, and the symmetric dirty-index case is unsound.
4. **Anchor C against the real index** — same class of coupling; also would
   read whatever the operator happened to stage.
5. **Byte-exact tree comparison as the materialization test** — later
   unrelated edits are routine and harmless; byte equality fails all of them.
6. **Default-context reverse-check as the sole test** — 60 false reds in 184
   present trees at rev-1 measurement.
7. **`-C0` with `(0/0)` ignored (rev-1)** — E29: 2 false greens in 69 absent
   trees. Q14 is therefore mandatory, not optional.
8. **A stronger hunk-local corroboration instead of blocking on `(0/0)`** —
   considered and **not adopted**: no discriminator was found that could be
   *proved* on the measured corpus, and the reviewers explicitly rejected
   unproven inventions. Blocking is the proved-safe choice.
9. **Recipe replay as the parent presence test (rev-1)** — circular: replay is
   what arbitration decides about.
10. **Whole-file byte equality as the parent presence test** — the rev-0
    defect, re-imported by rev-1.
11. **Skip-and-pass when anchor H is unavailable (rev-1)** — an unverifiable
    historical half is an unverified feature; D14 makes it terminal.
12. **Using the attestation candidate as the replay anchor (rev-1)** — E30:
    after a re-land, its parent already contains the feature, permanently
    destroying anchor H.
13. **Broadening the anchor search to non-trailer commits** — would invent
    attribution; rejected.
14. **Later-touch from current byte differences (rev-1)** — fires on the
    operator's own edits and on formatting; D15 uses the shipped
    `RequestedAt` + touched-path detector instead.
15. **Blocking on later-touch** — contradicts ADR-029 D6's warning-class rule.
16. **The rev-0 `replace-in-file` round-trip predicate** — 204 false reds,
    15 933 false greens.
17. **Treating `R == ""` as a pass or a fail** — undecidable; must defer.
18. **Reverse-apply success as the landed detector** — E1.
19. **`git patch-id` as the ownership key** — ADR-018 exists because
    byte-identical patches across features are real.
20. **Persisting the landing SHA in `status.json`** — ADR-019 settled it;
    rebase/cherry-pick would stale it exactly when trailers stay correct.
21. **Overwriting `status.apply.base_commit` at land time** — would make every
    landed feature instantly `stale` and would break ADR-016.
22. **Classifying a prose-quoted trailer line as `none` (rev-1's implicit
    reading)** — E31: indistinguishable from a destroyed block; the
    conservative direction is `malformed`.
23. **`rev-list` for enumeration** — cannot emit `%B`; a second call would be
    needed.
24. **`--first-parent` reachability scoping** — misses merged side-branch
    landings.
25. **Approximating a merge landing's anchor as `^1`** — E33.
26. **Reader errors degrading to `none`** — converts an unknown into a claim.
27. **A twelfth check for current materialization** — breaks the frozen check
    vocabulary; the assertion belongs to V8.
28. **A new `verified-landed` freshness label** — the four D5 labels stay
    mutually exclusive and the derivation stays mode-agnostic.
29. **Emitting `freshness_label` in the verify JSON report** — rev-1's samples
    invented it; the shipped `VerifyReport`
    (`internal/workflow/verify.go:139-166`) has no such field. Removed rather
    than added casually.
30. **Leaving the `active`-parent inconsistency documented but unresolved** —
    D13 decides it.
31. **Ownership-only evidence** (accept a reachable slug trailer, demote hash
    mismatches to advisories) — a swapped `post-apply.patch` whose content
    happens to be present would get a green verdict.
32. **An escape flag (`--assume-landed`)** — converts a machine-checkable
    claim into a hand-written one, which D3 already refuses.
33. **Fixing forward-mode V10's live-tree reference here** — out of scope; it
    changes verdicts for features that never landed. Recorded as a residual.

## Amendment 1 rev-2 — consequences

**Positive**

- Anchor C is immune to a dirty worktree or index — the rev-1 CRITICAL.
- The `(0/0)` hardening removes the measured false-green class entirely.
- An unverifiable historical half can no longer pass.
- Parent arbitration is non-mutating, non-circular and uses one test.
- V10's later-touch matches the shipped ADR-029 detector rather than a
  reinvented byte comparison.
- A re-record + re-land recovers anchor H instead of destroying it.
- The evidence grammar is conservative: no destroyed attestation is ever read
  as "no attestation".

**Negative / accepted trade-offs**

- The hardened ladder costs a measured **26 false reds in 151** present trees,
  every one with a real remediation (re-record so the recorded context matches
  HEAD, then re-land).
- `historical-anchor-unavailable` is terminal, so a feature whose landing
  history cannot supply a baseline fails until the operator re-lands.
- `active` closure widening changes verdicts for non-landed features.
- Repos below the git floor fail instead of silently forward-verifying.
- V10 remains wrong for un-landed applied features with a real
  `preimage_hash` (E11/E12) — recorded, not fixed.

**Neutral**

- No lifecycle change, no new `FeatureState`, no new label, no new check ID,
  no new artifact, no store schema change, no `land` behaviour change.

## Amendment 1 rev-2 — references (anchors validated 2026-08-12 at `f9138e6`)

**Contract documents**

- `docs/prds/PRD-verify-freshness.md` §3.6, §4.3.6–4.3.9, §7.1
- `docs/prds/PRD-tpatch-land.md` §3.8, §6.2
- `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md` — the four-trailer
  block D10 reads
- `docs/adrs/ADR-016-record-auto-base-resolution.md` — owner of
  `status.apply.base_commit`
- `docs/adrs/ADR-018-record-collision-detection-signature.md`
- `docs/adrs/ADR-029-write-file-recipe-safety.md` D1/D2/D4/D5/D6/D7/D8 — the
  policy D15 implements
- `docs/adrs/ADR-028-supersession-edge-model.md` D6
- `docs/adrs/ADR-031-rejected-feature-state-data-model.md`,
  `docs/adrs/ADR-032-feature-unapply-state-boundary.md`
- CHANGELOG v0.11.3 — the GH #2 fix D9 must not regress
- `docs/reconcile.md` — why landed remediation must not route to `reconcile`

**Source anchors**

- `internal/workflow/verify.go:49-71` — the eleven frozen check IDs (D8)
- `internal/workflow/verify.go:83` — `verifySchemaVersion`
- `internal/workflow/verify.go:127-134` — `postApplyVerifyStates` (D13)
- `internal/workflow/verify.go:139-166` — `VerifyReport` (no `freshness_label`)
- `internal/workflow/verify.go:288-289` — V10 appended last
- `internal/workflow/verify.go:292-293` — recipe/patch digests
- `internal/workflow/verify.go:310-314` — `--no-write` persistence gate
- `internal/workflow/verify.go:498` — `sha256Hex`
- `internal/workflow/verify.go:853-861` — V10 skip when V2 skipped (D15)
- `internal/workflow/verify.go:862-870` — supersession severity downgrade
- `internal/workflow/verify.go:879-883` — ADR-029 D4 legacy path (D15)
- `internal/workflow/verify.go:927` — `runClosureReplay`
- `internal/workflow/verify.go:976-983` — supersession exclusion (D13)
- `internal/workflow/verify.go:998` — `TopologicalOrder` call
- `internal/workflow/verify.go:1012`, `:1024` — `HeadCommit` / `CreateShadow`
- `internal/workflow/verify.go:1036-1040` — deferred `PruneShadow`
- `internal/workflow/verify.go:1048-1091` — the closure replay loop
- `internal/workflow/verify.go:1061-1089` — the state switch `active` joins
- `internal/workflow/verify.go:1092`, `:1143` — GH #2 snapshot + reset
- `internal/workflow/verify.go:1156-1160` — `git apply --check` (anchor H)
- `internal/workflow/verify.go:1167` — the `run tpatch reconcile` string
  landed mode replaces
- `internal/workflow/verify.go:1275-1282` — 1-based op index convention
- `internal/workflow/verify.go:1284-1318` — `replayOpInShadow` op kinds
- `internal/workflow/verify_all.go:89-97` — `isPostApplyState` admits `active`
- `internal/workflow/writefile_safety.go:108-112` — V10 reads the live tree
- `internal/workflow/writefile_safety.go:176` — `isLowercaseHex`
- `internal/workflow/writefile_safety.go:380-388` — `laterTouchIndex` (D15)
- `internal/workflow/writefile_safety.go:409-442` — `loadLaterFeatureTouches`,
  `RequestedAt` ordering (D15)
- `internal/workflow/writefile_safety.go:449-481` — `collectFeatureTouchedPaths`
- `internal/workflow/writefile_safety.go:489-498` — `checkLaterTouch`
- `internal/workflow/writefile_safety.go:571` — `DetectRecordLaterTouchWarnings`
- `internal/workflow/recipe_autogen.go:114-118` — autogen omits `preimage_hash`
- `internal/workflow/dependency_gate.go:79-81` — the gate accepts `active`
- `internal/store/patch_generations.go:52` — `TouchedPaths` (D15)
- `internal/cli/land.go:392`, `:394`, `:397-400` — trailer production
- `internal/cli/land.go:1034-1043` — `readRecipeSHA`, whitespace → `none`
- `internal/cli/land_journal.go:31`, `:60` — the gitignored `.tpatch/local/`
  precedent for a temp-file home (D11)
- `internal/gitutil/ignore.go:50-51` — `.tpatch/local/` ignore handling
- `internal/store/types.go:290-296` — `VerifyRecord` (unchanged)
- `internal/store/types.go:347` — `ApplySummary.BaseCommit`
- `internal/store/dag.go:107` — `TopologicalOrder`
- `internal/gitutil/gitutil.go:14` — `HeadCommit`
- `internal/gitutil/gitutil.go:828` — `IsAncestor`
- `internal/gitutil/shadow.go:56` — `CreateShadow` accepts any commit-ish
- `internal/gitutil/patch_paths_strict.go:253` — `FilesInPatchStrict`
- `internal/workflow/verify_closure_replay_test.go:275` —
  `TestRunVerify_EquivalentRecipeAndPatchBothPass`, the GH #2 regression
