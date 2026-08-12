# ADR-013 — Verify Freshness Overlay

**Status**: Accepted (M15 Wave 3 design — Git-like freshness redesign; PRD: `docs/prds/PRD-verify-freshness.md`) · **Amendment 1 proposed 2026-08-12 (v0.15.1 Wave B / GH #8 — landed-evidence semantics; D8–D14 below, AWAITING REVIEW)**
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

> **Amended 2026-08-12 by Amendment 1 (GH #8).** D7's mechanics are retained
> in full: the shadow is still rooted at current `HEAD`, replay is still
> topological, fail-fast is still first-failure, and the prune is still
> deferred. What Amendment 1 refines is (a) **which** closure members are
> replayed — D11 removes landed-and-materialized members for the same reason
> `upstream_merged` members are already removed — and (b) **how** the target
> is judged when it is itself landed (D10). Step 3 of the algorithm above
> should be read as "skip parents whose content is already on the baseline",
> of which `upstream_merged` is one case and locally landed is the other.


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

# Amendment 1 (2026-08-12) — Landed-evidence semantics — v0.15.1 Wave B / GH #8

**Status of this amendment**: proposed, AWAITING REVIEW. Binding on the Wave C
implementation once accepted. Adds D8–D14. **No prior decision D1–D7 is
reversed**; D7 is *extended* (its "shadow is rooted at current HEAD" mechanic
is retained and its closure-membership rule is refined by D11).

**Issue**: <https://github.com/tesseracode/tesserapatch/issues/8>
**Co-amended**: `docs/prds/PRD-verify-freshness.md` §3.6 / §4.3.6–4.3.8 / §7.1;
`docs/prds/PRD-tpatch-land.md` §3.8 / §6.2.

## A1.0 Context — what GH #8 actually is

`tpatch land` (v0.8.0, ADR-019) commits the feature into reachable Git
history while intentionally leaving `status.apply.base_commit` unchanged
(`docs/prds/PRD-tpatch-land.md` §3.6; `internal/cli/land.go:394`). `verify`
allocates its V7/V8 shadow from **current HEAD**
(`internal/workflow/verify.go:1012`, `:1024`). After `land`, HEAD therefore
already contains the feature, and the forward-apply semantics that V7 and V8
were designed around no longer describe the world.

Empirical matrix reproduced on this repo's `main` at `13a885c` with the real
CLI (throwaway repos, since removed):

| Target recipe op kind | pre-land V7/V8 | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file`      | PASS / PASS | **PASS — false green** (`os.WriteFile` overwrites unconditionally, `internal/workflow/verify.go:1290-1294`) | **FAIL — false red** |
| `replace-in-file` | PASS / PASS | **FAIL — false red** (`search text not found`, `internal/workflow/verify.go:1295-1305`) | SKIP (V7 failed) |
| `append-file`     | PASS / PASS | **PASS — false green, and the shadow is silently corrupted by a double append** (`internal/workflow/verify.go:1306-1313`) | **FAIL — false red** |

Three consequences fix the scope of this amendment:

1. **The bug is not V8-only.** The issue reporter observed `V7 ✓ / V8 ✗`
   because their recipe was `write-file`-shaped. `replace-in-file` fails V7;
   `append-file` passes V7 only by corrupting the shadow. Any fix that
   special-cases V8 is incomplete.
2. **An "it would replay cleanly" test is not a materialization test.** The
   `write-file` false green is precisely the idempotent-overwrite shortcut.
   D10 forbids it.
3. **The same failure applies to a landed hard *parent*.** Closure replay
   (`internal/workflow/verify.go:1048-1091`) re-executes a landed parent's
   recipe on top of already-materialized content. D11 fixes that.

## A1.1 Empirical basis (read-only probes, git 2.55.0, macOS)

Every Git claim below was executed before being written down. The scratch
repos were removed; the observations are recorded here because the design
depends on them.

| # | Observation | Consequence for the design |
|---|---|---|
| E1 | `git apply --check --reverse` of a feature's patch succeeds at *any* tree containing equivalent content, including a commit authored by an unrelated actor. | Reverse-apply is a **materialization** signal only. It is never ownership proof. (D8) |
| E2 | `git log --grep '^Tpatch-Feature: <slug>$'` also matches a commit whose *prose body* quotes that line; `%(trailers:key=Tpatch-Feature,valueonly)` correctly returns empty for it. | `--grep` is a cheap prefilter. `%(trailers:…)` is the authority. (D8) |
| E3 | Git parses trailers only from the **last** paragraph. A `--amend` that appends a prose paragraph after the block makes `%(trailers:…)` return empty even though the content is unchanged. | Trailer loss is `malformed`, distinct from `none`. (D12) |
| E4 | `--grep '^Tpatch-Feature: my-slug$'` does **not** match `Tpatch-Feature: my-slug-extended`; the unanchored form matches both. | Slug matching is exact-value, never prefix. (D8) |
| E5 | `cherry-pick` and `rebase` copy the trailer block **verbatim**; the commit SHA and the parent change, and `Tpatch-Base-Commit` may name a commit that is no longer reachable. | Evidence must not key on the landing commit's SHA or parent. (D8) |
| E6 | A single commit can carry **several** `Tpatch-Feature` trailer values (squash landing). | The parser is multi-value. (D8) |
| E7 | `git revert` of a landing commit does **not** copy trailers, and the landing commit stays reachable. Evidence present, content gone. | Evidence reachability alone is not materialization. Both are required. (D10/D12) |
| E8 | `git log -z --format='%H%x1f%(trailers:key=…,valueonly,separator=%x2c)…'` enumerates SHA + all four trailer values in **one** invocation, NUL-record / `0x1f`-field delimited. | The whole design costs one extra `git log` per verify run and needs no new dependency. (D14) |
| E9 | A landing merged from a side branch is reachable through the non-first parent; `git log --grep … HEAD` finds it, `--first-parent` may not. | Reachability is full-graph, not first-parent. (D8) |

## Decision (Amendment 1)

### D8. Landing evidence is trailer-derived, reachability-scoped, and value-validated

A feature `S` is **landed** for verification purposes iff a commit reachable
from the *current* `HEAD` carries a well-formed `tpatch land` trailer block
naming `S`, and that block's recorded hashes still describe `S`'s current
canonical artifacts.

**Candidate enumeration.** Walk commits reachable from `HEAD` (full graph, not
`--first-parent` — E9). `--grep '^Tpatch-Feature: '` is permitted as a
prefilter for cost only; the authoritative value is the parsed
`%(trailers:key=Tpatch-Feature,valueonly)` list (E2). A commit is a candidate
for `S` iff that list contains `S` under **exact string equality after
trimming ASCII space/tab** — never prefix, never substring (E4, E6).

**Well-formedness.** A candidate must carry all four ADR-019 trailers
(`Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`,
`Tpatch-Base-Commit`). A commit that matched the prefilter but whose parsed
trailer list is empty, or that is missing any of the four, is **malformed**.

**Value validation** against the feature's *current* canonical bytes:

| Trailer | Validated against | Match rule |
|---|---|---|
| `Tpatch-Patch-SHA`   | `sha256(artifacts/post-apply.patch)` — the same bytes `land` hashes (`internal/cli/land.go:392`) and the same digest `verify` already computes (`internal/workflow/verify.go:498`, consumed at `:293`) | byte-equal hex |
| `Tpatch-Recipe-SHA`  | `sha256(artifacts/apply-recipe.json)`, or the literal `none` when the recipe is absent (`internal/cli/land.go:1034-1043`) | byte-equal hex, or both `none` |
| `Tpatch-Base-Commit` | `status.apply.base_commit` (`internal/store/types.go:347`) — **not** the landing commit's parent | byte-equal SHA |

`Tpatch-Base-Commit` is validated against `status.apply.base_commit` and not
against the landing commit's parent precisely because rebase and cherry-pick
rewrite the parent while preserving the trailer (E5). If the recorded base is
no longer reachable from HEAD (`gitutil.IsAncestor`,
`internal/gitutil/gitutil.go:828`), that is reported as the advisory
`base_commit_reachable: false` and is **not** on its own an evidence failure —
verify does not claim to know why history was rewritten.

**Classification** of each candidate: `exact` (all three values match),
`divergent` (well-formed, ≥1 value mismatched), `malformed` (as above).

**Selection — deterministic, total:**

| Candidate population | `landing_evidence.state` | Verdict effect |
|---|---|---|
| no candidates | `none` | **Non-landed semantics** — V7/V8 behave exactly as they do today. This is the only fallback path, and it is the pre-existing behavior. |
| exactly one `exact` | `exact` | landed semantics (D10/D11) |
| ≥2 `exact`, and every one of them introduces a byte-identical change over its own parent restricted to the canonical patch's path set | `duplicate-equivalent` | landed semantics; `duplicates: <n>` recorded. Tie-break for the reported commit: latest in `git rev-list --topo-order HEAD` order, then lexicographically greatest SHA. |
| ≥2 `exact` that are not byte-equivalent in that sense | `ambiguous` | **FAIL**, `failed_at: "landing-evidence"` |
| 0 `exact`, ≥1 `divergent` | `stale` | **FAIL**, `failed_at: "landing-evidence"` |
| only `malformed` | `malformed` | **FAIL**, `failed_at: "landing-evidence"` |

**Merge topology.** A merge commit is a candidate only if it itself carries the
trailer (i.e. a squash-merge landing). Verify makes no claim about which merge
parent contributed which bytes; presence is settled by the materialization
check at the baseline (D10), never by topology inference.

**Reverse-apply is never ownership proof (E1).** The reverse check in D10 runs
*only* behind an `exact`/`duplicate-equivalent` evidence commit whose
`Tpatch-Patch-SHA` equals the digest of the very bytes being reverse-checked.

**Rejected alternatives.** (a) *`git patch-id` equivalence as the ownership
key* — rejected: identical content across features is a real, detected
condition (ADR-018 cross-feature collision detection), so `patch-id` cannot
attest ownership. (b) *Reverse-apply success alone* — rejected on E1. (c)
*`--first-parent` scoping* — rejected on E9: it would false-negative every
merged side-branch landing. (d) *Keying on the landing commit's SHA* —
rejected on E5: rebase/cherry-pick rewrite it while the attestation stays
valid, and ADR-019 already refuses to persist it.

### D9. Baseline model: HEAD-anchored closure with landed-materialization arbitration

Three models were evaluated for what tree V7/V8 must be judged against.

| Model | Verdict | Why |
|---|---|---|
| **(a) Current HEAD, with landed members recognised as already on the baseline** | **CHOSEN** | Always resolvable; a single tree the whole closure can share; keeps validating against the world the operator actually has. |
| (b) Replay at the original `status.apply.base_commit` | rejected | That field is owned by `record` / auto-base resolution (ADR-016) and moves under the operator; it may be unreachable after a rebase or in a shallow clone; and it validates the past — a feature landed and then reverted still "passes forever". It also has no single value across a closure: parent and child have different bases, so one shadow cannot host both. |
| (c) Replay at the selected landing commit's parent (`L^`) | rejected | Undefined for a multi-parent squash landing; rewritten by rebase; per-member (each closure member has a different `L^`), so again no shared baseline; and it carries model (b)'s "validates the past" flaw. Retained only as a *diagnostic* value, never as the tree verify judges against. |

**The chosen model, stated normatively.**

1. The shadow is created at **current HEAD**
   (`gitutil.HeadCommit` → `gitutil.CreateShadow`,
   `internal/workflow/verify.go:1012`, `:1024`). Unchanged mechanic; D7 stands.
2. Before any replay, verify performs **one** evidence pass (D8, one `git log`
   per run — E8) and classifies every closure member, including the target.
   Classification is computed once, up front, and is not re-evaluated during
   replay; replay order therefore cannot change any classification.
3. The **effective replay set** is the existing hard-parent topological closure
   (`store.TopologicalOrder`, `internal/store/dag.go:107`, driven from
   `internal/workflow/verify.go:998`) minus every member that is already on
   the baseline. D11 defines membership exactly. The unifying rule:
   **a locally landed feature is to the baseline what an `upstream_merged`
   feature is** — already there, therefore never replayed
   (cf. the existing `upstream_merged` skip at
   `internal/workflow/verify.go:1062-1064`).
4. If the **target** has no landing evidence, V7/V8 run in **forward mode**,
   byte-for-byte as they do today.
5. If the target has authoritative landing evidence, V7/V8 run in
   **materialized mode** (D10).

**Why the model stays meaningful after later commits.** HEAD advances and the
baseline advances with it. An unrelated later commit is invisible to the
checks. A later commit that *overlaps* the feature's paths changes the baseline
content — and materialized mode detects exactly that as drift. Models (b) and
(c) are structurally blind to it, which is the deeper reason they are rejected.

**GH #2 invariant, preserved and generalised.** The v0.11.3 fix — snapshot the
closure-replayed baseline, then reset the shared shadow back to that tree
before the patch check, so the recipe and the patch are validated
*independently against the same baseline*
(`internal/workflow/verify.go:1092`, `:1143`; CHANGELOG v0.11.3) — is binding
in **both** modes. Normative restatement: *any check that may mutate the shadow
MUST reset it to `closureBaselineTree` before the next check runs; V7's result
must never be an input to V8's tree.* Materialized mode is non-mutating, so the
reset is a no-op there, but the rule is stated uniformly so the regression that
pins it cannot be weakened by mode.

### D10. Landed target: materialized mode for V7 and V8

When the target's `landing_evidence.state ∈ {exact, duplicate-equivalent}`:

**V7 (`recipe_replay_clean`) — post-state predicates, no execution.**
The recipe's operations are **not executed**. Ops are grouped by `op.Path` in
recipe order, and the **final** op on each path is asserted against the
baseline tree with a decidable post-state predicate:

| Final op on the path | Materialized iff |
|---|---|
| `write-file` | the file exists and its bytes are **byte-identical** to `op.Content` |
| `append-file` | the file exists and its content **ends with** `op.Content` (strict suffix when `op.Content` is non-empty; a trivially-empty content passes) |
| `replace-in-file` | the **op-inverse round trip** holds: with `c` the file content, `strings.Replace(strings.Replace(c, R, S, 1), S, R, 1) == c`, and (`R == ""` or `strings.Contains(c, R)`) |
| `ensure-directory` | the path exists and is a directory |
| unknown type | fail (unchanged from `internal/workflow/verify.go:1316`) |

The `replace-in-file` predicate is the general form: it asserts *there exists a
preimage that this exact operation would turn into the current content*. It is
correct for the ordinary case, for `search ⊂ replace` (insert-around-existing)
recipes, and for files where `search` legitimately occurs more than once — all
of which a naive `!contains(search) && contains(replace)` rule would
mis-classify.

**The idempotent-overwrite shortcut is forbidden.** It is never sufficient for
V7 that an operation *would succeed if replayed*. `write-file` demands byte
equality, not writability. This is the exact defect that made the GH #8
reporter's V7 green (A1.0).

**Accepted bound, stated explicitly.** Non-final operations on the same path
are not independently asserted; they are subsumed by the terminal predicate and
by V8's byte-exact check. V7 materialized mode is the *op-level localisation*
signal that produces actionable per-op diagnostics; V8 is the byte-exact
authority. This bound is recorded rather than hidden.

**V8 (`post_apply_patch_replay_clean`) — reverse check at the same baseline.**
`git apply --check --reverse <artifacts/post-apply.patch>` against the baseline
tree, after the mandatory D9 reset. Passing means the canonical patch's
postimage is present. This is sound **only** because it is gated behind D8: the
evidence commit's `Tpatch-Patch-SHA` equals `sha256` of the very bytes being
reverse-checked, which is what closes the E1 gap. In addition every path
declared by the patch must resolve (reuse the strict path parser,
`internal/gitutil/patch_paths_strict.go`).

**Outcomes.** PASS ⇒ per-check `mode: "already-materialized"`. FAIL ⇒
`materialization-drift` (V7, naming the 1-based op index, path, and which
predicate failed — same index convention as
`internal/workflow/verify.go:1275-1282`) or `landed-content-absent` (V8).
Skip rules for absent recipe / absent patch are unchanged
(`internal/workflow/verify.go:932-937`, `:1130-1141`).

**Remediation must never send a just-landed local feature to `reconcile`.**
The current V8 failure text is
`post-apply.patch no longer applies to closure-replayed baseline; run tpatch reconcile <slug>`
(`internal/workflow/verify.go:1167`). `reconcile` is the *upstream-drift* verb
(ADR-010; `docs/reconcile.md`): for a feature the operator landed minutes ago
there is no upstream drift, the advice is unactionable, and reconcile may
rewrite the canonical patch — destroying the artifact the landing trailer
attests to. Landed-mode remediation therefore points at inspection
(`git show`, `git diff`), at `tpatch record`/`tpatch land` re-attestation, or
at `tpatch apply` re-materialisation. Exact strings are locked in
PRD-verify-freshness §3.6.6.

### D11. Hard-parent closure: landed, applied-unlanded, and retired parents are three different things

Per closure member other than the target, evaluated once during the D9 step-2
classification pass:

| Member condition | Closure action | Rationale |
|---|---|---|
| `upstream_merged` | **skip** (unchanged, `internal/workflow/verify.go:1062-1064`) | already on the baseline |
| superseded by an active superseder | **skip** (unchanged, `internal/workflow/verify.go:976-983`, ADR-028 D6) | excluded from the effective set |
| `applied`, evidence `exact`/`duplicate-equivalent`, **and** materialized at the baseline | **skip** (new) | landed is locally what `upstream_merged` is remotely; replaying would double-apply |
| `applied`, evidence `exact`/`duplicate-equivalent`, **not** materialized at the baseline | **fail-fast** `parent-landing-drift` | replaying would silently repair a broken baseline and produce a success-shaped result |
| `applied`, evidence `none` | **replay** (unchanged, `internal/workflow/verify.go:1065-1082`) | its content is genuinely absent from the baseline |
| `applied`, evidence `stale` / `ambiguous` / `malformed` | **fail-fast** `parent-evidence-integrity` | neither skipping nor replaying is defensible: skipping validates the target against *some other version* of the parent; replaying may double-apply |
| `active` | **fail-fast, unchanged** | Today's switch (`internal/workflow/verify.go:1061-1089`) handles only `upstream_merged` and `applied`; `active` reaches `default:`. This differs from `CheckDependencyGate`, which accepts both (`internal/workflow/dependency_gate.go:80`). It is a **pre-existing** inconsistency that this amendment deliberately does **not** widen — doing so would change verdicts for features unrelated to landing. Separate decision, separate PRD. |
| `unapplied` (ADR-032) | **fail-fast** `parent-unapplied` | its patch is deliberately absent from the tree; the closure cannot be reconstructed. Today this falls into the generic `default:` branch (`internal/workflow/verify.go:1083-1089`); the contract names it |
| `rejected` (ADR-031) | **fail-fast** `parent-rejected` | terminal pre-implementation; no recipe to replay |
| any other state | **fail-fast** (unchanged `default:`) | |

**Governing invariant — no double application.** A member is replayed **iff**
its content is not already on the baseline. Landed-and-materialized ⇒ on the
baseline ⇒ never replayed. This is what makes `append-file` parents safe: a
replayed landed `append-file` parent silently duplicates its payload (A1.0).

**Landing order is irrelevant.** Whether a parent landed before or after the
target changes nothing, because classification is per-member against *current*
HEAD, never against a temporal ordering. Any order-dependent rule would be
unstable under rebase (E5). Closure ordering itself is unchanged: topological,
parents before children.

**Mixed chains compose.** Target unlanded + parent P1 landed + parent P2
applied-unlanded ⇒ baseline HEAD already carries P1, P2 is replayed, then the
target's recipe is forward-applied. Target landed + parent unlanded ⇒ P is
replayed into the shadow and the target is then judged in materialized mode
against that post-replay baseline. A conflict between the two surfaces as
either the existing parent-replay fail-fast or as target drift — both honest
failures, neither a success-shaped fallback.

### D12. Drift and evolution: exactly one fallback edge, and it is `none`

| Situation | Evidence | Materialized | Verdict |
|---|---|---|---|
| later **unrelated** commit | `exact` | yes | PASS |
| later overlapping commit, feature content still satisfies D10 | `exact` | yes | PASS |
| later overlapping commit that changes feature content | `exact` | no | **FAIL** `materialization-drift` |
| **full revert** of the landing commit (E7) | `exact` (still reachable) | no | **FAIL** `landed-content-reverted` |
| **partial revert** | `exact` | partly | **FAIL** `materialization-drift`, first failing op named |
| `--amend` appending prose after the trailer block (E3) | `malformed` | yes | **FAIL** `evidence-integrity` — deliberately *not* `none`: content is present but the attestation was destroyed |
| re-`record` after landing (patch bytes change) | `stale` | — | **FAIL** `evidence-stale` |
| `record --regenerate-recipe` after landing | `stale` | — | **FAIL** `evidence-stale` |
| new patch generation recorded | `stale` (patch SHA) | — | **FAIL** `evidence-stale` |
| re-`land` after re-`record` (two landings, both current) | `duplicate-equivalent` or `ambiguous` | yes | PASS if byte-equivalent per D8; else **FAIL** |
| `tpatch feature unapply` on the target | n/a | n/a | **refused** — `StateUnapplied` is outside `postApplyVerifyStates` (`internal/workflow/verify.go:127-134`); unchanged |
| re-`apply` after unapply, no re-land | `stale` or `exact` per hashes | yes | decided in landed mode; no special case |
| branch switch to a branch without the landing | `none` | no | **non-landed semantics** (forward mode) — status quo |
| branch switch to a branch without the landing but with equivalent content present | `none` | yes | **non-landed semantics**, forward mode fails. Correct: verify has no authority to claim the feature owns unattributed content (E1). Diagnostics say so explicitly instead of guessing. |
| detached HEAD | evaluated identically — reachability is from whatever `HEAD` resolves to | — | no special case; the resolved baseline commit is reported |
| merge brings the landing in (E9) | `exact` | yes | PASS |
| cherry-pick (E5) | `exact`; `base_commit_reachable` may be `false` | yes | PASS |
| rebase rewrote the landing (E5) | `exact` on the rewritten commit | yes | PASS |
| history rewritten so no landing is reachable | `none` | maybe | **non-landed semantics** |

**The single fallback edge.** Verification falls back to non-landed semantics
**iff** `landing_evidence.state == none`. `stale`, `ambiguous` and `malformed`
are terminal failures with `failed_at: "landing-evidence"`. There is **no
success-shaped fallback** for ambiguous or contradictory evidence: verify never
"tries the other mode and passes". This is the load-bearing safety property of
the amendment — every other rule here is a refinement of it.

### D13. Diagnostics: read-only, mode-explicit, and no new freshness label

Verify remains **read-only** (D7 unchanged): no worktree mutation, no index
mutation, no `status.json` write beyond the existing `Verify` record — and none
at all under `--no-write` (`internal/workflow/verify.go:310-314`). Evidence
enumeration is `git log` / `git rev-list` plumbing, which is read-only, and it
runs against the repo root, not the shadow. The shadow is still created and
pruned via the existing deferred call (`internal/workflow/verify.go:1036-1040`).

Both the human report and `--json` state (i) the baseline mode and commit,
(ii) the landing-evidence state and, when selected, the evidence commit and
which of the three trailer values matched, and (iii) per check whether it ran
forward or already-materialized. Exact shapes in PRD-verify-freshness
§4.3.6–4.3.8.

`--json` grows additive `omitempty` fields and `schema_version` moves
`"1.0"` → `"1.1"` (`internal/workflow/verify.go:83`). A minor bump, not a
major: PRD §4.3 tells consumers to refuse unknown **majors**, so existing
consumers keep working unchanged.

**No new freshness label.** `never-verified` / `verified-fresh` /
`verified-stale` / `verify-failed` (D5) are unchanged and remain mutually
exclusive. A passing landed-mode run writes the same `VerifyRecord` shape as a
passing forward-mode run — same `Passed`, same hashes, same parent snapshot
(`internal/store/types.go:290-296`) — so the read-time derivation in
`ComposeLabels` is **mode-agnostic** and needs no change. Sticky
`verify-failed` therefore clears the ordinary way: the next passing run
overwrites `Verify.Passed`, which includes the GH #8 case where the *only*
thing that changed is that the run now uses landed semantics. A
`verified-landed` label was considered and **rejected**: the freshness
vocabulary describes staleness, not which baseline mode executed, and adding a
fifth value would break the four-labels-mutually-exclusive invariant D5
depends on.

### D14. Implementability: one new generic Git primitive, policy stays in verify

- **Trailer reading** is a generic Git capability with no existing helper
  anywhere in `internal/` (searched: no `interpret-trailers`, no
  `%(trailers`). A single reader belongs in `internal/gitutil/` (candidate
  `trailers.go`) exposing "commits reachable from `<rev>` with their parsed
  `Tpatch-*` trailer values", implemented as **one** `git log -z --format=…`
  invocation per verify run (E8) parsed with `bytes.Split`. `status`,
  `doctor` and `reconcile` are plausible future consumers, which is why it is
  not buried in `verify.go`.
- **Policy** — candidate selection, classification, mode arbitration — stays
  in `internal/workflow/verify.go`, consistent with D7's "closure replay is
  verify-only; do not factor out without an ADR amendment".
- **Everything else reuses shipped primitives**: `gitutil.HeadCommit` /
  `CreateShadow` / `PruneShadow`, `gitutil.IsAncestor`
  (`internal/gitutil/gitutil.go:828`), `store.TopologicalOrder`
  (`internal/store/dag.go:107`), `isFeatureSupersededIn`
  (`internal/workflow/verify.go:976`), `sha256Hex`
  (`internal/workflow/verify.go:498`), the strict patch-path parser
  (`internal/gitutil/patch_paths_strict.go`), `os`/`strings` for the D10
  predicates (same primitives `replayOpInShadow` already uses,
  `internal/workflow/verify.go:1284-1318`).
- **No new store field, no new artifact, no schema migration.**
- **Git floor.** `%(trailers:key=…,valueonly)` requires git ≥ 2.22 and
  `separator=` requires git ≥ 2.25; verified working on 2.55.0. Below the
  floor the reader yields no parsed trailers, which resolves to evidence
  `none` — i.e. today's behavior, a false *red*, never a false green. Wave C
  should emit a one-line `warn`-severity advisory in that case rather than
  silently degrade.

## Amendment 1 — alternatives considered and rejected

1. **Shadow rooted at `status.apply.base_commit`** — D9(b). Operator-owned and
   drifting field; may be unreachable; validates the past; no single value
   across a closure.
2. **Shadow rooted at the landing commit's parent `L^`** — D9(c). Undefined for
   squash landings; rewritten by rebase; per-member; validates the past.
3. **Reverse-apply success as the landed detector** — rejected on E1. Retained
   only as a materialization signal *behind* trailer evidence.
4. **`git patch-id` as the ownership key** — rejected: ADR-018 exists precisely
   because byte-identical patches occur across features.
5. **Persisting the landing commit SHA in `status.json`** — rejected; see
   D8 rejected-alternative (d) and PRD-tpatch-land §3.8.3. ADR-019 already
   settled this; rebase/cherry-pick would stale the field exactly when the
   trailers stay correct, and it would create a second source of truth against
   ADR-011 D6 / ADR-010 D5.
6. **Changing `land` to overwrite `status.apply.base_commit` with the new
   HEAD** — rejected: it is the chicken-and-egg ADR-019 already refused, it
   would make every landed feature instantly evidence-`stale` under D8, and it
   would break `record` auto-base resolution (ADR-016).
7. **Idempotent-overwrite shortcut for `write-file` in materialized mode** —
   rejected; it *is* the GH #8 false green.
8. **Special-casing V8 only** — rejected; `replace-in-file` fails V7 and
   `append-file` corrupts the shadow (A1.0).
9. **A new `verified-landed` freshness label** — rejected per D13.
10. **`--first-parent` reachability scoping** — rejected on E9.
11. **Skipping closure members with stale/ambiguous evidence** — rejected per
    D11: it manufactures a success-shaped result from contradictory evidence.
12. **Letting verify auto-heal (invoke `record`/`land` on failure)** — rejected:
    verify is read-only by D7, and self-healing a *verification* command
    destroys the signal it exists to produce.
13. **`schema_version` major bump to `"2.0"`** — rejected: every new field is
    additive and `omitempty`.
14. **Inferring which merge parent contributed the landed bytes** — rejected as
    over-claiming Git semantics; presence is settled by the materialization
    predicate at the baseline, not by topology.
15. **Ownership-only evidence: treat a reachable slug trailer as sufficient and
    demote hash mismatches to advisories** — rejected, and this is the closest
    call in the amendment. Under that model, `stale` evidence would still enter
    materialized mode and the §3.6.3 predicates would run against the
    *current* artifacts, so a re-`record` after landing would pass with a
    "re-land to re-attest" note instead of failing. It is more permissive and
    superficially friendlier. It is rejected because the ownership token would
    then attest artifacts other than the ones being checked: an operator (or a
    bad reconcile) that replaces `post-apply.patch` with a different patch
    whose content happens to be present at HEAD would receive a **green**
    verdict claiming the feature is healthy. That is precisely a success-shaped
    result derived from contradictory evidence. The chosen model refuses, and
    pays for the strictness with a one-command remediation (`tpatch land
    <slug>` re-attests) rather than with a silent pass.
16. **An escape flag (`--ignore-landing-evidence` / `--assume-landed`)** —
    rejected. A flag that lets the operator assert what the tool could not
    verify converts a machine-checkable claim into a hand-written one, which
    D3 already refuses for the freshness record itself. Re-attesting with
    `tpatch land` is the supported path and is one command.

## Amendment 1 — consequences

**Positive**

- `verify` becomes correct for the `land`-first workflow that `tpatch land`
  has shipped since v0.8.0, across all three mutating recipe op kinds.
- The `write-file` and `append-file` **false greens** are closed — a stronger
  guarantee than the issue asked for.
- Landed hard parents stop being double-applied into the shadow.
- One unified rule (`landed ≈ upstream_merged, locally`) replaces what would
  otherwise be two parallel skip mechanisms.
- Remediation stops mis-routing local bookkeeping problems into the provider-
  assisted upstream reconcile path.

**Negative / accepted trade-offs**

- One extra `git log` per verify run (bounded, single invocation, E8).
- V7 materialized mode asserts only the terminal op per path (D10 accepted
  bound); V8 remains the byte-exact authority.
- Verify now depends on `tpatch land`'s trailer schema. Mitigated: the schema
  is locked by ADR-019 and a schema break there already requires a new ADR.
- Repos on git < 2.25 degrade to today's behavior (D14), which is a false red,
  not a false green.
- Features landed by a hand-rolled `git commit` without trailers get evidence
  `none` and keep today's behavior. This is intentional: attribution cannot be
  invented.

**Neutral**

- No lifecycle change, no new `FeatureState`, no new label, no new artifact,
  no store schema change, no `land` behavior change.
- Wave C implementation is a `verify`-side change plus one `gitutil` reader.

## Amendment 1 — references (anchors validated 2026-08-12 at `13a885c`)

**Contract documents**

- `docs/prds/PRD-verify-freshness.md` §3.6 — the operational contract this
  amendment governs; §4.3.6–4.3.8 JSON shapes; §7.1 acceptance matrix
- `docs/prds/PRD-tpatch-land.md` §3.8 — landing-evidence readers' contract;
  §6.2 land-side acceptance rows
- `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md` — the four-trailer
  block D8 reads, and the "`apply.base_commit` is never overwritten by land"
  lock D8 depends on
- `docs/adrs/ADR-016-record-auto-base-resolution.md` — owner of
  `status.apply.base_commit`; the reason D9(b) is rejected
- `docs/adrs/ADR-018-record-collision-detection-signature.md` — byte-identical
  patches across features are a real condition; the reason `git patch-id` is
  rejected as an ownership key
- `docs/adrs/ADR-028-supersession-edge-model.md` D6 — the existing closure
  exclusion D11 composes with
- `docs/adrs/ADR-031-rejected-feature-state-data-model.md`,
  `docs/adrs/ADR-032-feature-unapply-state-boundary.md` — the two terminal
  states D11 names explicitly
- `docs/adrs/ADR-029-write-file-recipe-safety.md` — `write-file`
  `preimage_hash` semantics that V10 already enforces alongside D10
- CHANGELOG v0.11.3 — the GH #2 fix D9 must not regress
- `docs/reconcile.md` — why landed-mode remediation must not route to
  `tpatch reconcile`

**Source anchors**

- `internal/workflow/verify.go:83` — `verifySchemaVersion` (D13 bump site)
- `internal/workflow/verify.go:127-134` — `postApplyVerifyStates` (D12
  `unapplied` refusal)
- `internal/workflow/verify.go:139-166` — `VerifyReport` (D13 additive fields)
- `internal/workflow/verify.go:293` — patch-hash computation reused by D8
- `internal/workflow/verify.go:310-314` — `--no-write` persistence gate (D13)
- `internal/workflow/verify.go:498` — `sha256Hex` (D8 digest reuse)
- `internal/workflow/verify.go:927` — `runClosureReplay` (D9/D10/D11 host)
- `internal/workflow/verify.go:976-983` — supersession exclusion (D11 row)
- `internal/workflow/verify.go:1012` / `:1024` — `HeadCommit` / `CreateShadow`
  (D9 baseline anchor, retained)
- `internal/workflow/verify.go:1036-1040` — deferred `PruneShadow` (D13)
- `internal/workflow/verify.go:1062-1064` — `upstream_merged` skip (the
  precedent D11 generalises)
- `internal/workflow/verify.go:1065-1082` — `applied` replay branch (D11)
- `internal/workflow/verify.go:1083-1089` — `default:` fail-fast (D11 names
  `unapplied` / `rejected` out of it)
- `internal/workflow/verify.go:1092` / `:1143` — GH #2 baseline snapshot +
  pre-V8 reset (D9 invariant)
- `internal/workflow/verify.go:1156-1160` — `git apply --check` (D10 reverse
  variant site)
- `internal/workflow/verify.go:1167` — the `run tpatch reconcile` remediation
  D10 replaces in landed mode
- `internal/workflow/verify.go:1275-1282` — 1-based op index convention D10
  reuses for diagnostics
- `internal/workflow/verify.go:1284-1318` — `replayOpInShadow` op kinds; the
  `write-file` / `replace-in-file` / `append-file` behaviors A1.0 measured
- `internal/cli/land.go:392` / `:394` / `:397-400` — patch SHA, base commit,
  trailer block construction (D8 evidence producer)
- `internal/cli/land.go:1034-1043` — `readRecipeSHA`, including the `none`
  sentinel D8 must accept
- `internal/store/types.go:290-296` — `VerifyRecord` (D13: unchanged)
- `internal/store/types.go:343-347` — `ApplySummary.BaseCommit` (D8 validation
  target)
- `internal/store/dag.go:107` — `TopologicalOrder` (D9 ordering)
- `internal/gitutil/gitutil.go:14` — `HeadCommit`
- `internal/gitutil/gitutil.go:828` — `IsAncestor` (D8 `base_commit_reachable`)
- `internal/gitutil/shadow.go:56` — `CreateShadow` (accepts an arbitrary
  commit-ish; D9 keeps passing `HEAD`)
- `internal/gitutil/patch_paths_strict.go` — strict patch path parsing (D10)
- `internal/workflow/verify_closure_replay_test.go:275` —
  `TestRunVerify_EquivalentRecipeAndPatchBothPass`, the GH #2 regression D9
  must keep green
