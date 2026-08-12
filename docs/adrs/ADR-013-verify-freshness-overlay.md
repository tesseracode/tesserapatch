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

> **Amended 2026-08-12 by Amendment 1 rev-1 (GH #8).** D7's machinery is
> retained in full — one shadow, topological replay, first-failure fail-fast,
> deferred prune, and the GH #2 reset between the recipe and the patch check.
> Amendment 1 refines three things. (a) The shadow's **root commit** becomes
> anchor-dependent: it stays `HEAD` for a feature with no landing evidence and
> becomes the selected landing commit's single parent `L^` in landed mode
> (D9). `gitutil.CreateShadow` already accepts an arbitrary commit-ish, so
> this is a parameter change, not a new mechanism. (b) Step 3 of the algorithm
> above generalises to "skip members whose content is already present on the
> anchor", of which `upstream_merged` is one case, a landed member is another,
> and an unattributed already-present member is a third (D12). (c) A second,
> read-only **current-HEAD** assertion is added for landed features and folded
> into V8 (D11); it allocates no shadow and does not disturb D7's single-shadow
> rule.


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
# Amendment 1 (2026-08-12, **rev-1**) — Landed-evidence semantics — v0.15.1 Wave B / GH #8

**Status of this amendment**: proposed rev-1, AWAITING REVIEW. Binding on the
Wave C implementation once accepted. Adds D8–D16. **No prior decision D1–D7
is reversed**; D7 is *extended* (its shadow mechanic is retained but the
shadow's **root commit** becomes anchor-dependent, and its closure-membership
rule is refined by D12).

**Issue**: <https://github.com/tesseracode/tesserapatch/issues/8>
**Co-amended**: `docs/prds/PRD-verify-freshness.md` §3.6 / §4.3.6–4.3.9 / §7.1;
`docs/prds/PRD-tpatch-land.md` §3.8 / §6.2.

**rev-0 → rev-1**: rev-0 was returned NEEDS REVISION by both reviewers.
Its two structural errors were (a) omitting **V10**
(`write_file_preimage_fresh`) and therefore describing a 10-check schema that
does not exist, and (b) judging a landed feature **only** at current `HEAD`
with byte-exact post-state predicates, which false-reds on any later
unrelated edit and cannot give V7 a role independent of V8. rev-1 replaces
the baseline model (D9), rebuilds the evidence reader (D10), defines V10
(D13), and makes every state total. Every change below is grounded in a
measured probe recorded in A1.1.

## A1.0 Context — what GH #8 actually is

`tpatch land` (v0.8.0, ADR-019) commits the feature into reachable Git
history while intentionally leaving `status.apply.base_commit` unchanged
(`docs/prds/PRD-tpatch-land.md` §3.6; `internal/cli/land.go:394`). `verify`
allocates its V7/V8 shadow from **current HEAD**
(`internal/workflow/verify.go:1012`, `:1024`). After `land`, HEAD therefore
already contains the feature, and the forward-apply semantics V7 and V8 were
designed around no longer describe the world.

Measured with the real CLI on this repo at `13a885c` (throwaway repos, since
removed). The check set is **eleven** checks, V0–V10
(`internal/workflow/verify.go:50-71`), and every run below reported
`checks=11`:

| Target recipe op kind | pre-land V7/V8 | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file`      | PASS / PASS | **PASS — false green** (`os.WriteFile` overwrites unconditionally, `internal/workflow/verify.go:1290-1294`) | **FAIL — false red** |
| `replace-in-file` | PASS / PASS | **FAIL — false red** (`search text not found`, `internal/workflow/verify.go:1295-1305`) | SKIP (V7 failed) |
| `append-file`     | PASS / PASS | **PASS — false green, and the shadow is silently corrupted by a double append** (`internal/workflow/verify.go:1306-1313`) | **FAIL — false red** |

Three consequences fix the scope:

1. **The bug is not V8-only.** The reporter saw `V7 ✓ / V8 ✗` because their
   recipe was `write-file` shaped.
2. **"It would replay cleanly" is not a materialization test** — the
   `write-file` false green is exactly that shortcut.
3. **The same failure hits a landed hard *parent*.** Closure replay
   (`internal/workflow/verify.go:1048-1091`) re-executes a landed parent's
   recipe on top of already-materialized content.

## A1.1 Empirical basis (read-only probes, git 2.55.0 / macOS, scratch removed)

Every load-bearing claim was executed before being written. rev-1 adds E10–E22.

| # | Observation | Consequence |
|---|---|---|
| E1 | `git apply --check --reverse` succeeds at *any* tree containing equivalent content, including one produced by an unrelated actor. | Reverse-apply is a **materialization** signal only, never ownership proof. (D10) |
| E2 | `git log --grep '^Tpatch-Feature: <slug>$'` also matches a commit whose *prose body* quotes that line; `%(trailers:key=…,valueonly)` returns empty for it. | `--grep` is a prefilter; `%(trailers:…)` is authority. (D10) |
| E3 | Git parses trailers only from the **last** paragraph. A `--amend` appending prose makes `%(trailers:…)` return empty while the raw `%B` still contains the line. | Trailer loss is `malformed`, and **only the raw message can classify it**. (D10) |
| E4 | `--grep '^Tpatch-Feature: my-slug$'` does not match `my-slug-extended`; the unanchored form matches both. | Exact-value slug matching. (D10) |
| E5 | `cherry-pick` and `rebase` copy the trailer block verbatim; SHA and parent change; `Tpatch-Base-Commit` may become unreachable. | Evidence must not key on the landing SHA or its parent identity. (D10) |
| E6 | A single commit can carry several `Tpatch-Feature` values. | Cardinality must be decided, not assumed. (D10) |
| E7 | `git revert` of a landing does **not** copy trailers; the landing stays reachable with the content gone. | Reachability ≠ materialization. (D11/D14) |
| E8 | `git log -z --format='%H%x1f%P%x1f%B%x1f%(trailers:key=…,valueonly,separator=…)…'` yields SHA, **parent list**, **raw message** and all four trailer values in **one** invocation. | One enumeration per run; parent cardinality is free. (D16) |
| E9 | A landing merged from a side branch is reachable through the non-first parent. | Reachability is full-graph, never `--first-parent`. (D10) |
| **E10** | `RecipeFromPatch` emits `{type,path,content}` only — **no `preimage_hash`** (`internal/workflow/recipe_autogen.go:114-118`). Every autogenerated recipe therefore takes the ADR-029 D4 legacy path and V10 passes without asserting anything. | V10's real bite is on provider/hand-authored recipes. (D13) |
| **E11** | With a genuine `preimage_hash`, **V10 already FAILS for an `applied`, un-landed feature**: `expected preimage sha256:5fb14…, observed sha256:fa6dd8…`. The preimage is the *pre*-apply content; the live tree holds the *post*-apply content. | V10's live-tree reference is wrong for any applied feature. Pre-existing; see D13 and the residual. |
| **E12** | Same for `preimage_hash: ""` (new-file): after apply the file exists → `new-file collision` FAIL. | Both preimage shapes break post-apply. (D13) |
| **E13** | `checkWriteFilePreimage` reads from `repoRoot` — the **live working tree** — not the shadow (`internal/workflow/writefile_safety.go:108-112`). | V10 is a live-tree check today; landed mode must re-anchor it. (D13) |
| **E14** | Reverse-check context ladder, 3-hunk patch in a 60-line file: offset shift (10 lines prepended/appended) → `C3=OK C1=OK C0=OK`; unrelated edit far away → all OK; unrelated edit **2 lines** from a hunk → `C3=FAIL C1=OK C0=OK`; **1 line** away → `C3=FAIL C1=FAIL C0=OK`; partial revert, full revert, feature line further modified, patched file deleted → **FAIL at all three**. | Reverse-apply is **offset-tolerant but context-sensitive**. Default `-C3` false-reds a healthy landed feature whose neighbour line moved. (D11) |
| **E15** | Randomized adversarial run, 400 trees: `-C0` reverse-check gave **0 false greens over 216 postimage-absent trees** and **0 false reds over 184 postimage-present trees**; default `-C3` gave **60 false reds / 184**. | `-C0` is the accurate operationalisation of "hunk presence modulo context and offset". (D11) |
| **E16** | Measured hole: with the feature reverted *in place* and the identical postimage text pasted elsewhere in the same file, `-C0` **succeeds**. `--verbose` (under a pinned locale) prints `Hunk #1 succeeded at 41 (offset -27 lines).` and `Context reduced to (0/0) to apply fragment at 41`. | A real, bounded `-C0` limitation, and it is *detectable*. Recorded, not hidden. (D11) |
| **E17** | Exhaustive enumeration over alphabet `{a,b,X}`, preimages ≤ 7 chars, contents ≤ 5 chars, all 1–2-char `search`/`replace`: the rev-0 `replace-in-file` predicate `Replace(Replace(c,R,S,1),S,R,1)==c` gave **204 false reds and 15 933 false greens** across 56 784 decided cases (e.g. false red `c='abb', S='aa', R='b'`; false green `c='b', S='a', R='a'`). | The rev-0 predicate is unsound in both directions. Replaced. (D12) |
| **E18** | The rev-1 predicate — *existential inverse over **every** occurrence of `R` in `c`* — gave **0 false reds and 0 false greens** over the same 52 416 decided cases, with 4 368 correctly reported **undecidable** (`R == ""` or `S == ""`). | Machine-verified sound and complete on the enumerated domain. (D12) |
| **E19** | Go semantics: `strings.Replace("abc","","Z",1) == "Zabc"`; `strings.Contains(x,"") == true`. | `search == ""` is a degenerate op; `replace == ""` cannot attest. (D12) |
| **E20** | Duplicate trailers are observable but ambiguous: two `Tpatch-Patch-SHA` lines yield `aaaa,bbbb`; two `Tpatch-Recipe-SHA` yield `none,cccc`. A naive "take the first" parser silently picks a convenient duplicate. | Strict cardinality is mandatory. (D10) |
| **E21** | Git trailer keys are **case-insensitive**: `tpatch-feature: lowerfeat` is returned by `%(trailers:key=Tpatch-Feature,valueonly)`. | The reader inherits git's case-insensitivity; the contract states it rather than pretending otherwise. (D10) |
| **E22** | Topology: a root landing commit has **0** parents and `git rev-parse <root>^` fails with `fatal: ambiguous argument`; a merge commit carrying a trailer has **2** parents and its trailer parses normally. | An implicit `^` is not total. Single-parent is a hard requirement. (D9/D15) |

---

## Decision (Amendment 1 rev-1)

### D8. The check set is eleven checks, V0–V10, and this amendment changes none of the identifiers

`internal/workflow/verify.go:50-71` defines eleven check IDs. V10 is
`write_file_preimage_fresh`, appended last in `RunVerify`
(`internal/workflow/verify.go:288-289`), after V9. Every report shape, golden
example, JSON sample and acceptance row in this amendment and in
`PRD-verify-freshness` describes **eleven** rows in that order.

This amendment introduces **no new check ID and removes none**. The
current-materialization assertion GH #8 requires is folded into **V8**, whose
subject — the canonical `post-apply.patch` — is exactly the artifact the
landing trailer attests. Adding a twelfth check was considered and rejected
(rejected alternative 17): it would break the frozen vocabulary comment at
`internal/workflow/verify.go:49` and every consumer that switches on it.

### D9. Baseline model: **dual-anchor landed verification**

rev-0 judged a landed target only at current `HEAD`. rev-1 rejects that.
Three models were evaluated.

| Model | Verdict |
|---|---|
| **(a) HEAD-only, post-state predicates** (rev-0) | **Rejected.** For `write-file` — the shape *every autogenerated recipe* has (E10) — the only HEAD-evaluable predicate is byte equality, which false-reds on any later unrelated edit to the file (E14, and measured directly: after prepending four comment lines the recipe content and the HEAD file are `DIFFERENT`). Weakening it to "the patch's hunks are present" makes V7 a silent alias of V8, which is the reviewer finding rev-1 must close. V10 is worse: its reference is the live tree (E13), where a landed feature's preimage never matches (E11/E12). |
| **(b) Replay at `status.apply.base_commit`** | **Rejected.** Owned by `record` / auto-base resolution (ADR-016) and moved by the operator; may be unreachable after a rewrite or in a shallow clone; carries no relationship to the attested landing. |
| **(c) Dual anchor: historical replay at the landing commit's parent, plus a current-HEAD materialization assertion** | **CHOSEN.** |

**The chosen model, stated normatively.**

Landed verification has **two anchors**, answering two different questions.

- **Anchor H (historical) — "are the artifacts internally coherent?"**
  A shadow rooted at `L^`, the single parent of the selected landing commit
  `L` (D15). Into it, the closure is replayed under D12 arbitration; then the
  **existing, unmodified** machinery runs: V7 replays the target's recipe
  (`replayRecipeOpsInShadow`, `internal/workflow/verify.go:1275`), the shadow
  is reset to `closureBaselineTree`, and V8 runs `git apply --check` forward
  (`internal/workflow/verify.go:1092`, `:1143`, `:1156`). V10 evaluates
  `preimage_hash` against the closure baseline (D13).
- **Anchor C (current) — "is it still there?"**
  At current `HEAD`, the canonical `post-apply.patch` must still be
  materialized, asserted by the reverse-check ladder of D11.

**Why this is right and (a) is not.** rev-0 collapsed two questions into one
check and therefore had to choose between false reds and vacuity. Separating
them gives each check an independent, statable job:

- V7 at anchor H proves the recipe still forward-applies to the tree it was
  authored against, with its closure. That is **not** derivable from V8 and
  is not an alias of it.
- V8 has two independent obligations — forward coherence at anchor H, and
  current materialization at anchor C. Both are block-severity; both are
  reported separately.
- V10 at anchor H is evaluated against the exact tree its `preimage_hash`
  describes, which is the only place the field is meaningful.
- Later unrelated commits cannot touch anchor H at all, so they cannot
  false-red the historical half; and anchor C's ladder is offset-tolerant
  (E14/E15), so they cannot false-red the current half either.

**Implementation delta.** Anchor H changes exactly one thing in the shipped
code path: the commit passed to `gitutil.CreateShadow`
(`internal/gitutil/shadow.go:56`, which already accepts an arbitrary
commit-ish) becomes `L^` instead of `gitutil.HeadCommit`
(`internal/workflow/verify.go:1012`, `:1024`). Anchor C adds one or two
`git apply --check --reverse` invocations. Model (a) would have required a
whole new predicate engine.

**GH #2 invariant, preserved verbatim and generalised.** The v0.11.3 fix —
snapshot the closure-replayed baseline, then reset the shared shadow back to
that tree before the patch check, so the recipe and the patch are validated
*independently against the same baseline* — is binding at anchor H exactly as
today, and the normative restatement applies to every mode: *any check that
may mutate the shadow MUST reset it to `closureBaselineTree` before the next
check runs; V7's result is never an input to V8's tree.* Anchor C is
read-only and allocates no shadow.

**Non-landed features are untouched.** Evidence `none` ⇒ the shadow is rooted
at `HEAD` and V7/V8/V10 behave byte-for-byte as they do today.

**Anchor H unavailability is explicit, never silent.** If no candidate
qualifies under D15, V7 and V8's historical half are **skipped with the
reason** `skipped: no single-parent landing anchor available`, the report
carries `historical_anchor: {"state":"unavailable","reason":…}`, and anchor C
still runs at block severity. This is degradation with a named cause, not a
fallback to pass: the block-severity materialization assertion is unaffected.

### D10. Evidence reader: one enumeration, raw **and** parsed, strict grammar

**Enumeration.** Exactly **one** `git log` invocation per verify run
(cached and reused for every feature of a `verify --all` run) over commits
reachable from the resolved `HEAD`, in `--topo-order`, emitting per commit:
`%H`, `%P`, the raw message `%B`, and `%(trailers:key=…,valueonly,separator=…)`
for the four ADR-019 keys, NUL-delimited (E8). No `--first-parent` (E9).
`--grep '^Tpatch-Feature: '` is permitted as a cost prefilter **only if** the
raw body is still retained for matched commits; because E3 requires
classifying commits whose trailer block was destroyed, the enumeration must
also retain any commit whose **raw body** contains a `Tpatch-Feature:` line
even when the parsed value is empty.

**Raw retention is mandatory.** E3 and E20 both show that parsed-only output
cannot distinguish "no attestation" from "destroyed attestation". The reader
returns both; the classifier reads both.

**Grammar (normative).**

| Element | Rule |
|---|---|
| Key case | Git matches trailer keys **case-insensitively** (E21). The reader inherits this and the contract says so; it does not claim a case-sensitive match it cannot implement. |
| `Tpatch-Feature` cardinality | **Exactly one value.** `land` emits exactly one (`internal/cli/land.go:397-400`) and ADR-019 admits no other producer. Zero ⇒ not a candidate (or `malformed` if the raw body carries the line). Two or more (E6) ⇒ `malformed`: the sibling SHA trailers cannot be attributed to a specific slug. |
| `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` / `Tpatch-Base-Commit` cardinality | **Exactly one each.** Zero or ≥2 ⇒ `malformed` (E20). No "take the first" and no duplicate selection. |
| Slug match | Exact string equality after trimming leading/trailing ASCII space and tab (E4). Never prefix, never substring. |
| `Tpatch-Patch-SHA` format | 64 **lowercase** hex. Anything else ⇒ `malformed`. |
| `Tpatch-Recipe-SHA` format | 64 lowercase hex **or** the literal `none`. Anything else ⇒ `malformed`. |
| `Tpatch-Base-Commit` format | 40 lowercase hex. Anything else ⇒ `malformed`. |
| Reader failure | Git invocation error, unparsable output, or a git that does not support the format ⇒ evidence state **`unavailable`**, a **block** failure. It **never** degrades to `none`. |

Lowercase strictness follows the ADR-029 D1 precedent already enforced by
`isLowercaseHex` (`internal/workflow/writefile_safety.go:176`).

**Value validation** against the run's artifact snapshot (D14):

| Trailer | Compared with |
|---|---|
| `Tpatch-Patch-SHA` | `sha256` of the snapshot's `post-apply.patch` bytes. **Presence-aware**: artifact *absent* ⇒ there is no digest and any attested value is a mismatch; artifact *present and zero-byte* ⇒ the digest is `sha256("")` and must compare equal. Absent ≠ empty. |
| `Tpatch-Recipe-SHA` | `sha256` of the snapshot's `apply-recipe.json` bytes, **except** that an absent artifact **or** one that is whitespace-only expects the literal `none` — mirroring `readRecipeSHA` (`internal/cli/land.go:1034-1043`), which returns `none` on both a read error and `strings.TrimSpace(...) == ""`. |
| `Tpatch-Base-Commit` | `status.apply.base_commit` from the snapshot (`internal/store/types.go:347`) — **not** the landing commit's parent, because rebase and cherry-pick rewrite the parent while copying the trailer (E5). Unreachability of that commit (`gitutil.IsAncestor`, `internal/gitutil/gitutil.go:828`) is the advisory `base_commit_reachable: false` and never fails on its own. |

**Evidence states — closed set, total:**
`none`, `exact`, `duplicate-equivalent`, `stale`, `ambiguous`, `malformed`,
`unsupported-topology`, `unavailable`.

| Candidate population | State | Effect |
|---|---|---|
| no candidate, and no raw-body-only match | `none` | forward mode; today's behavior |
| exactly one well-formed candidate, all three values match | `exact` | landed mode |
| ≥2 such candidates, byte-equivalent per D15 | `duplicate-equivalent` | landed mode; `duplicates: n` |
| ≥2 such candidates, not byte-equivalent or not comparable | `ambiguous` | **FAIL** `failed_at: "landing-evidence"` |
| 0 all-match, ≥1 well-formed-but-mismatched | `stale` | **FAIL** `failed_at: "landing-evidence"` |
| only cardinality/format/raw-only failures | `malformed` | **FAIL** `failed_at: "landing-evidence"` |
| candidates exist but none has exactly one parent | `unsupported-topology` | **FAIL** `failed_at: "landing-evidence"` |
| reader error | `unavailable` | **FAIL** `failed_at: "landing-evidence"` |

**Reverse-apply is never ownership proof (E1).** Anchor C runs only behind an
`exact` / `duplicate-equivalent` evidence commit whose `Tpatch-Patch-SHA`
equals the digest of the very bytes being reverse-checked.

### D11. Anchor C — the current-materialization ladder, and what it can and cannot prove

**Accurate statement of what reverse-apply proves.** `git apply --check
--reverse` asserts that the patch's **postimage hunks are present**, matched
by content with a line-offset search and a configurable context requirement.
It is **not** a byte-exact tree comparison, and it is **not** ownership
evidence. rev-0's "byte-exact" language was wrong.

**The ladder (normative).**

1. `git apply --check --reverse <patch>` at default context. Success ⇒
   **materialized, clean**. V8's anchor-C half passes with
   `mode: "already-materialized"` and no advisory.
2. Only on failure: `git apply --check --reverse -C0 <patch>`. Success ⇒
   **materialized with context drift**. V8's anchor-C half passes at block
   severity and the report carries a **`warn`-severity advisory** naming the
   affected paths and telling the operator to inspect. Failure ⇒ **FAIL**
   `landed-content-absent` (block).

**Why the ladder and not default-only.** E14/E15: default context produced
**60 false reds in 184 postimage-present trees**, because an unrelated edit
within three lines of a hunk breaks context matching. `-C0` produced **0
false reds and 0 false greens** across the same randomized corpus while still
failing every partial revert, full revert, in-place modification and file
deletion.

**Advisory vocabulary.** The ladder's step-2 pass raises the `warn`-severity
advisory code `context-drift`. The full closed advisory set shared with the
PRD is `context-drift`, `later-touch`, `unattributed-materialized`,
`base-commit-unreachable`; none of them flips `passed`, and
`base-commit-unreachable` is the D10 `base_commit_reachable: false` signal.
The exact JSON shape is `PRD-verify-freshness` §4.3.6–4.3.9.

**Measured limitation, recorded not hidden (E16).** `-C0` can succeed when the
feature was reverted *in place* and the identical postimage text exists
verbatim elsewhere in the same file, because the offset search relocates the
hunk. Three things bound it: (i) it requires a deliberate revert-plus-paste of
identical text; (ii) it surfaces as step 2, i.e. a `warn` advisory, never a
silent clean pass; (iii) V7 at anchor H is an independent corroboration that
this shape does not produce. **Hardening named for Wave C**: run step 2 with
`--verbose` under a pinned `LC_ALL=C` and treat `Context reduced to (0/0)` on
*every* hunk as not-materialized — measured to fire exactly on the E16 shape.
This is a SHOULD, not a MUST, because it depends on parsing human-readable
git output; the residual is recorded in the PRD.

### D12. Closure arbitration: replay iff the content is not already on the anchor

**Governing invariant, unchanged in spirit from rev-0 and now total:** a
closure member is replayed **iff** its content is not already present on the
anchor being built. Arbitration runs at whichever anchor the shadow is rooted
at — `L^` in landed mode, `HEAD` in forward mode.

| Member condition | Action |
|---|---|
| `upstream_merged` | **skip** (unchanged, `internal/workflow/verify.go:1062-1064`) |
| superseded by an active superseder | **skip** (unchanged, `internal/workflow/verify.go:976-983`) |
| `applied` / `active`, evidence `exact`/`duplicate-equivalent`, **and** total materialization (D14) holds at the anchor | **skip** |
| `applied` / `active`, evidence `exact`/`duplicate-equivalent`, materialization does **not** hold | **fail-fast** `parent-landing-drift` |
| `applied` / `active`, evidence `none`, **content already present at the anchor** | **skip**, with a `warn`-severity `unattributed-materialized` advisory naming the member. Verify explicitly claims **no ownership** of that content. |
| `applied` / `active`, evidence `none`, content absent at the anchor | **replay** (unchanged, `internal/workflow/verify.go:1065-1082`) |
| `applied` / `active`, evidence `stale` / `ambiguous` / `malformed` / `unsupported-topology` / `unavailable` | **fail-fast** `parent-evidence-integrity` |
| `unapplied` (ADR-032) | **fail-fast** `parent-unapplied` |
| `rejected` (ADR-031) | **fail-fast** `parent-rejected` |
| any other state | **fail-fast** (unchanged `default:`) |

**The `evidence none` + already-present row is a rev-1 correction.** rev-0
replayed such members unconditionally, which re-creates the double-apply
defect for exactly the `append-file` shape GH #8 is about. The advisory is
mandatory: skipping without saying so would let verify silently assume it
knows why the content is there.

**`active` is decided here, not left dangling.** `active` is treated
**identically to `applied`** throughout the closure. Today it is not: the
switch handles only `upstream_merged` and `applied`, so an `active` hard
parent reaches `default:` and fail-fasts
(`internal/workflow/verify.go:1061-1089`) — while `CheckDependencyGate`
accepts `applied` **and** `active`
(`internal/workflow/dependency_gate.go:79-81`), `postApplyVerifyStates`
admits `active` (`internal/workflow/verify.go:127-134`), and `verify --all`
does too (`internal/workflow/verify_all.go:89-97`). rev-0 documented the
inconsistency and declined to resolve it; the reviewers correctly refused a
dangling note. **Decision: widen the closure switch to accept `active`
wherever it accepts `applied`.** This is a deliberate behavior change for
non-landed features as well, it is the smallest change that makes the four
call sites agree, and it carries its own acceptance rows and its own risk
note in the PRD.

**Landing order is never consulted.** Classification is per-member against the
current graph, not a temporal ordering; an order-dependent rule would be
unstable under rebase (E5). Closure ordering itself is unchanged
(`store.TopologicalOrder`, `internal/store/dag.go:107`, driven from
`internal/workflow/verify.go:998`).

### D13. V10 in landed mode: anchor-H preimage, ADR-029-consistent later-touch warning

`checkWriteFilePreimage` reads the target from `repoRoot` — the **live
working tree** (E13, `internal/workflow/writefile_safety.go:108-112`). For an
**applied** feature the live tree holds the *post*-image, so a genuine
`preimage_hash` never matches (E11) and an empty preimage collides with the
now-existing file (E12). Autogenerated recipes escape this only because they
omit the field entirely and take the ADR-029 D4 legacy path (E10).

**Decision.** In landed mode V10's reference tree is the **anchor-H closure
baseline** — the shadow tree after closure arbitration and *before* the
target's recipe replays. That tree is, by construction, the preimage the
`preimage_hash` describes.

| Case | V10 landed outcome |
|---|---|
| `preimage_hash` absent | legacy pass, no re-warn (unchanged, ADR-029 D4, `internal/workflow/verify.go:879-883`) |
| `preimage_hash` present and matching at the anchor-H baseline | **PASS**, `mode: "historical-anchor"` |
| `preimage_hash` present and **not** matching at the anchor-H baseline | **FAIL** at block severity — the recipe is genuinely stale or destructive relative to its own baseline. Downgraded to `warn` when the feature is superseded (unchanged, ADR-029 D7, `internal/workflow/verify.go:862-870`). |
| the same path's content at current `HEAD` differs from the landing's postimage | **`warn`-severity later-touch advisory** on the V10 row, per ADR-029 D5/D6 (later-touch detection is mandatory and warning-class). **Never a block on its own.** |
| anchor H unavailable | **skip**, reason `skipped: landed V10 requires a historical anchor`, plus the later-touch advisory when computable. Never falls back to the live tree, because that is the E11/E12 false block. |
| V2 skipped or failed | **skip**, unchanged reason (`internal/workflow/verify.go:853-861`) |

This is exactly the ADR-029 policy the reviewers asked for: **no automatic
block merely because a landed `write-file` no longer matches its preimage**
(that is now a warn-class later-touch signal), and **no false green for an
actually stale or destructive recipe** (a preimage mismatch at its own
baseline still blocks).

**Out of scope, recorded:** E11/E12 mean V10 is also wrong for an
**un-landed applied** feature with a real `preimage_hash`. That is a shipped
defect independent of landing; this amendment cannot fix it, because there is
no anchor to evaluate against and changing forward-mode V10 would alter
verdicts for features that never landed. It is recorded as a residual with
its measured evidence.

### D14. Total materialization, and one immutable snapshot per run

**Total materialization** of a feature at a tree means **every applicable
assertion passes**, not the cheapest one:

- if a recipe is present with ≥1 operation, the recipe's **anchor-H** replay
  (V7) must succeed **and**
- if a canonical patch is present, the **anchor-C** ladder (D11) must succeed.

| Artifacts | Materialization requirement |
|---|---|
| recipe present (≥1 op) + patch present | both, as above |
| recipe absent, or present with zero operations, or whitespace-only | the **canonical patch** ladder is required and is the sole authority |
| patch absent (or the recipe is the only artifact) | the recipe replay is required and is the sole authority |
| **both absent** | materialization is **not provable** ⇒ the member **fails** (`landed-artifacts-absent`). It is never treated as materialized. |

rev-0 allowed a landed parent to be skipped on a recipe check alone; rev-1
requires the conjunction. This is what "no double application" needs in order
to be safe.

**Snapshot.** At the start of a run, verify takes an **immutable snapshot**
for the target and every closure member: the decoded `FeatureStatus`, the
**presence flag** and **raw bytes** of `artifacts/apply-recipe.json` and
`artifacts/post-apply.patch`. Evidence digests, V7, V8, V10, the persisted
`VerifyRecord` and the derived labels all read from that one snapshot — never
from a second read of disk. **Empty-present is distinct from absent** at
every consumer. Before the report is finalised, each snapshotted artifact is
re-read and compared; any difference ⇒ **FAIL** `failed_at:
"snapshot-unstable"` with the mutated path named. Verify never mixes bytes
from two points in time.

### D15. Landing-commit topology and the historical anchor

**Single parent required.** A candidate is usable as the historical anchor
only if `%P` lists **exactly one** parent. E22 measured both failure shapes:
a root landing has zero parents and `git rev-parse <root>^` fails outright,
and a merge landing has two, so an implicit `^` is not total. Candidates with
0 or ≥2 parents are classified **`unsupported-topology`** — not silently
approximated to `^1`. A merge-shaped candidate is honestly unsupported rather
than guessed at.

**Anchor selection — deterministic.** Among well-formed, all-values-match,
single-parent candidates, walk the single enumeration's `--topo-order`
**oldest-first** and select the first whose parent tree does **not** already
materialize the canonical patch (D14 anchor-C ladder run at that parent). If
several remain tied, the oldest in topo order wins; the final tie-break is the
lexicographically smallest full SHA. If no candidate qualifies, anchor H is
`unavailable` (D9).

**Duplicate-equivalence — implementable, no broadening.** Let `P` be the
canonical patch's declared path set, obtained from the strict parser
(`gitutil.FilesInPatchStrict`, `internal/gitutil/patch_paths_strict.go:253`),
sorted byte-wise. If `P` is **empty**, candidates are **not** comparable ⇒
`ambiguous`; the path set is never broadened to "all paths". Otherwise, for
each candidate `C` compute

```
git diff --no-color --no-ext-diff --no-textconv --binary \
         --no-renames --unified=3 <C>^ <C> -- <P...>
```

captured as raw bytes. Candidates are `duplicate-equivalent` iff every such
byte string is identical. Any candidate that is not single-parent, or whose
diff cannot be produced, makes the set `ambiguous`. The reported commit is the
selected anchor (above); `duplicates` is the candidate count.

**Rebase / cherry-pick / branch switch / detached HEAD / rewrite — total.**
Trailers survive rebase and cherry-pick verbatim while SHA and parent change
(E5); evidence therefore keys on trailer *values*, never on the landing SHA or
its parent identity, and both cases classify `exact` with a possibly-false
`base_commit_reachable`. A branch switch that removes the landing from
reachability yields `none` ⇒ forward mode. A detached `HEAD` is evaluated
identically from whatever `HEAD` resolves to; the resolved commit is reported
as `baseline.commit`. A history rewrite that leaves no reachable landing
yields `none`. A rewrite that leaves *two* reachable landings is decided by
the duplicate-equivalence rule above.

### D16. Implementability and honest invocation accounting

rev-0 claimed "one `git log`-family invocation per run" while also requiring
per-candidate diffs and ancestry checks. That accounting was contradictory.
The honest budget:

| Purpose | Invocations |
|---|---|
| Evidence enumeration (raw + parsed + `%P`, whole closure, all features) | **1 per run**, cached across `verify --all` |
| Shadow allocation | 1 `CreateShadow` (already allocated today; only its commit-ish changes) |
| Anchor-C ladder | 1, or 2 when step 1 fails, **per landed member** |
| Duplicate-equivalence diff | 1 `git diff` per candidate, **only when ≥2 candidates** |
| `base_commit_reachable` advisory | 1 `git merge-base --is-ancestor` per landed member |
| Anchor-H parent materialization probe during D15 selection | 1 ladder run per candidate examined |

Everything else reuses shipped primitives: `gitutil.HeadCommit`
(`internal/gitutil/gitutil.go:14`), `CreateShadow`/`PruneShadow`,
`gitutil.IsAncestor`, `store.TopologicalOrder`, `isFeatureSupersededIn`
(`internal/workflow/verify.go:976`), `sha256Hex`
(`internal/workflow/verify.go:498`), `FilesInPatchStrict`,
`checkWriteFilePreimage` (`internal/workflow/writefile_safety.go:108`), and
`os`/`strings` for the D12 predicate. **No new store field, no new artifact,
no schema migration, no new dependency.**

The trailer reader is a new generic helper in `internal/gitutil/` (candidate
`trailers.go`) returning raw **and** parsed records; **policy** —
classification, anchor selection, mode arbitration — stays in
`internal/workflow/verify.go` per D7.

**Git floor.** `%(trailers:key=…,valueonly)` needs git ≥ 2.22 and
`separator=` needs ≥ 2.25; verified on 2.55.0. Below the floor the reader
**fails**, yielding evidence `unavailable` (a block), **not** `none`. rev-0's
"degrade to `none`" was wrong: it turns an unknown into a claim.

---

## Amendment 1 rev-1 — the `replace-in-file` predicate (D12 detail)

Landed-mode recipe assertions run at anchor H by **replaying** the recipe, so
the exhaustively-verified predicate below is required only where a
post-state judgement is unavoidable: the D12 "content already present at the
anchor" arbitration and the D14 recipe-only materialization case.

**Predicate (rev-1).** For content `c`, search `S`, replacement `R`:

- if `S == ""` ⇒ **undecidable / unsupported op**. `strings.Replace(x,"",R,1)`
  inserts at the start (E19); such an operation is malformed.
- if `R == ""` ⇒ **undecidable**. Every `c` admits a preimage `S+c`, so the
  predicate attests nothing; the judgement **defers to patch authority**
  (anchor C).
- otherwise: **true iff there exists an index `i` at which `R` occurs in `c`
  such that `pre := c[:i] + S + c[i+len(R):]` satisfies
  `strings.Replace(pre, S, R, 1) == c`.** Iterate every occurrence of `R`.

**Verification.** Exhaustive enumeration over alphabet `{a,b,X}`, preimages up
to 7 characters, contents up to 5, all 1–2-character `S`/`R` plus `R == ""`:

| Predicate | decided | undecidable | false reds | false greens |
|---|---|---|---|---|
| rev-0 `Replace(Replace(c,R,S,1),S,R,1)==c` | 56 784 | 0 | **204** | **15 933** |
| rev-1 existential inverse | 52 416 | 4 368 | **0** | **0** |

The other op kinds, when a post-state judgement is unavoidable:
`write-file` ⇒ bytes equal `op.Content`; `append-file` ⇒ content ends with
`op.Content`, and an **empty** `op.Content` is undecidable (it attests
nothing) rather than a vacuous pass; `ensure-directory` ⇒ the path exists and
is a directory; unknown type ⇒ unsupported (unchanged,
`internal/workflow/verify.go:1316`).

---

## Amendment 1 rev-1 — alternatives considered and rejected

1. **HEAD-only post-state predicates (rev-0's D9/D10)** — rejected per D9.
   False-reds every landed `write-file` feature after any later edit to the
   file, and cannot give V7 a role independent of V8.
2. **Replay at `status.apply.base_commit`** — rejected per D9(b).
3. **Byte-exact tree comparison as the materialization test** — rejected.
   E14/E15 show later unrelated edits are routine and harmless; byte equality
   makes every one of them a failure.
4. **Default-context reverse-check as the sole materialization test** —
   rejected: 60 false reds in 184 postimage-present trees (E15).
5. **`-C0` reverse-check as the sole materialization test** — rejected as
   *sole*: E16 is a real hole. Used only as step 2 of the ladder, behind
   trailer evidence and corroborated by anchor H.
6. **The rev-0 `replace-in-file` round-trip predicate** — rejected: 204 false
   reds and 15 933 false greens (E17).
7. **Treating `R == ""` as a pass or a fail** — rejected both ways: it is
   undecidable and must defer to patch authority (E19).
8. **Reverse-apply success as the landed detector** — rejected on E1.
9. **`git patch-id` as the ownership key** — rejected: ADR-018 exists because
   byte-identical patches across features are real.
10. **Persisting the landing SHA in `status.json`** — rejected; ADR-019
    settled it, and rebase/cherry-pick would stale the field exactly when the
    trailers stay correct.
11. **Overwriting `status.apply.base_commit` at land time** — rejected: it is
    ADR-019's chicken-and-egg, it would make every landed feature instantly
    `stale`, and it would break ADR-016.
12. **Idempotent-overwrite shortcut for `write-file`** — rejected; it *is* the
    GH #8 false green.
13. **A new `verified-landed` freshness label** — rejected; the four D5 labels
    stay mutually exclusive and the derivation stays mode-agnostic.
14. **`--first-parent` reachability scoping** — rejected on E9.
15. **Approximating a merge landing's anchor as `^1`** — rejected on E22;
    `unsupported-topology` is honest, a guess is not.
16. **Reader errors degrading to evidence `none`** (rev-0) — rejected: it
    converts an unknown into a positive claim. `unavailable` is a block.
17. **Adding a twelfth check for current materialization** — rejected per D8;
    it breaks the frozen check vocabulary. The assertion belongs to V8.
18. **Leaving the `active`-parent inconsistency documented but unresolved**
    (rev-0) — rejected by both reviewers; D12 decides it.
19. **Ownership-only evidence** (accept a reachable slug trailer and demote
    hash mismatches to advisories) — rejected. An operator or a bad reconcile
    that replaces `post-apply.patch` with a different patch whose content
    happens to be present would get a green verdict. The strictness is paid
    for with a one-command remediation (`tpatch land <slug>` re-attests).
20. **An escape flag (`--assume-landed`)** — rejected; it converts a
    machine-checkable claim into a hand-written one, which D3 already refuses.
21. **Claiming byte-identical no-evidence JSON output under schema 1.1**
    (rev-0) — rejected as dishonest: the additive `baseline` /
    `landing_evidence` / `target_mode` fields are emitted for every feature,
    so the guarantee is **additive semantic compatibility**, not byte
    identity.
22. **Fixing forward-mode V10's live-tree reference in this amendment** —
    rejected as out of scope; it changes verdicts for features that never
    landed. Recorded as a residual with measured evidence (E11/E12).

## Amendment 1 rev-1 — consequences

**Positive**

- `verify` becomes correct for the `land`-first workflow across all three
  mutating recipe op kinds, for the target and for hard parents.
- The `write-file` and `append-file` **false greens** and the
  `replace-in-file` **false red** are all closed.
- Later unrelated edits no longer fail a healthy landed feature (E14/E15),
  while partial and full reverts are still caught.
- V7, V8 and V10 each have a statable, independent job.
- V10 stops being meaningless for landed features, and the ADR-029 later-touch
  policy is honoured rather than contradicted.
- The `active`-parent inconsistency across four call sites is resolved.
- Evidence classification is total: eight closed states, with `unavailable`
  and `unsupported-topology` replacing rev-0's silent degradations.

**Negative / accepted trade-offs**

- Two anchors mean the shadow is rooted at `L^` in landed mode, so a landed
  feature whose landing parent is unreachable loses the historical half. This
  is explicit (`historical_anchor.state = "unavailable"`), never silent.
- The anchor-C ladder inherits the E16 `-C0` limitation; bounded, surfaced as
  a warn, and named for hardening.
- `active` closure widening is a behavior change for non-landed features.
- Verify depends on `land`'s trailer schema; ADR-019 already requires an ADR
  for any schema break.
- Repos below the git floor now **fail** instead of silently forward-verifying.
- V10 remains wrong for un-landed applied features with a real
  `preimage_hash` (E11/E12) — a pre-existing defect this amendment records
  but does not fix.

**Neutral**

- No lifecycle change, no new `FeatureState`, no new label, no new check ID,
  no new artifact, no store schema change, no `land` behavior change.

## Amendment 1 rev-1 — references (anchors validated 2026-08-12 at `4fdc18e`)

**Contract documents**

- `docs/prds/PRD-verify-freshness.md` §3.6, §4.3.6–4.3.9, §7.1
- `docs/prds/PRD-tpatch-land.md` §3.8, §6.2
- `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md` — the four-trailer
  block D10 reads and the "`apply.base_commit` is never overwritten" lock
- `docs/adrs/ADR-016-record-auto-base-resolution.md` — owner of
  `status.apply.base_commit`; why D9(b) is rejected
- `docs/adrs/ADR-018-record-collision-detection-signature.md` — why
  `git patch-id` cannot be an ownership key
- `docs/adrs/ADR-029-write-file-recipe-safety.md` D1/D2/D4/D5/D6/D7/D8 — the
  policy D13 must remain consistent with
- `docs/adrs/ADR-028-supersession-edge-model.md` D6 — the existing closure
  exclusion D12 composes with
- `docs/adrs/ADR-031-rejected-feature-state-data-model.md`,
  `docs/adrs/ADR-032-feature-unapply-state-boundary.md`
- CHANGELOG v0.11.3 — the GH #2 fix D9 must not regress
- `docs/reconcile.md` — why landed remediation must not route to `reconcile`

**Source anchors**

- `internal/workflow/verify.go:49-71` — the eleven frozen check IDs (D8)
- `internal/workflow/verify.go:83` — `verifySchemaVersion` (D8/schema)
- `internal/workflow/verify.go:127-134` — `postApplyVerifyStates` (D12)
- `internal/workflow/verify.go:139-166` — `VerifyReport` (additive fields)
- `internal/workflow/verify.go:288-289` — V10 appended last in `RunVerify`
- `internal/workflow/verify.go:292-293` — recipe/patch digests (D10, D14)
- `internal/workflow/verify.go:310-314` — `--no-write` persistence gate
- `internal/workflow/verify.go:498` — `sha256Hex`
- `internal/workflow/verify.go:879-883` — ADR-029 D4 legacy path (D13)
- `internal/workflow/verify.go:853-861` — V10 skip when V2 skipped (D13)
- `internal/workflow/verify.go:862-870` — supersession severity downgrade (D13)
- `internal/workflow/verify.go:927` — `runClosureReplay`
- `internal/workflow/verify.go:976-983` — supersession exclusion (D12)
- `internal/workflow/verify.go:998` — `TopologicalOrder` call (D12)
- `internal/workflow/verify.go:1012`, `:1024` — `HeadCommit` / `CreateShadow`
  (D9: only the commit-ish changes)
- `internal/workflow/verify.go:1036-1040` — deferred `PruneShadow`
- `internal/workflow/verify.go:1048-1091` — the closure replay loop (D12)
- `internal/workflow/verify.go:1061-1089` — the state switch `active` must
  join (D12)
- `internal/workflow/verify.go:1092`, `:1143` — GH #2 snapshot + reset (D9)
- `internal/workflow/verify.go:1156-1160` — `git apply --check` (anchor H)
- `internal/workflow/verify.go:1167` — the `run tpatch reconcile` string
  landed mode replaces
- `internal/workflow/verify.go:1275-1282` — 1-based op index convention
- `internal/workflow/verify.go:1284-1318` — `replayOpInShadow` op kinds
- `internal/workflow/verify_all.go:89-97` — `isPostApplyState` admits `active`
- `internal/workflow/writefile_safety.go:108-112` — V10 reads the live tree
  (E13)
- `internal/workflow/writefile_safety.go:176` — `isLowercaseHex` precedent
- `internal/workflow/recipe_autogen.go:114-118` — autogen omits
  `preimage_hash` (E10)
- `internal/workflow/dependency_gate.go:79-81` — the gate accepts `active`
- `internal/cli/land.go:392`, `:394`, `:397-400` — trailer production (D10)
- `internal/cli/land.go:1034-1043` — `readRecipeSHA`, incl. the whitespace →
  `none` rule (D10)
- `internal/store/types.go:290-296` — `VerifyRecord` (unchanged)
- `internal/store/types.go:347` — `ApplySummary.BaseCommit` (D10)
- `internal/store/dag.go:107` — `TopologicalOrder`
- `internal/gitutil/gitutil.go:14` — `HeadCommit`
- `internal/gitutil/gitutil.go:828` — `IsAncestor`
- `internal/gitutil/shadow.go:56` — `CreateShadow` accepts any commit-ish (D9)
- `internal/gitutil/patch_paths_strict.go:253` — `FilesInPatchStrict` (D15)
- `internal/workflow/verify_closure_replay_test.go:275` —
  `TestRunVerify_EquivalentRecipeAndPatchBothPass`, the GH #2 regression
