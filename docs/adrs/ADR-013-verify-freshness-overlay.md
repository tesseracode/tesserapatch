# ADR-013 — Verify Freshness Overlay

**Status**: Accepted (M15 Wave 3 design — Git-like freshness redesign; PRD: `docs/prds/PRD-verify-freshness.md`) · **Amendment 1 rev-7 (final) proposed 2026-08-12 (v0.15.1 Wave B / GH #8 — landed-evidence semantics; D8–D19 below — the final decision set, AWAITING REVIEW)**
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
6. Apply the target's recipe (V7); then, **per GH #2 / v0.11.3**, reset the shadow to the recorded closure-replayed baseline tree (`resetShadowToTree`, `internal/workflow/verify.go:1142-1153`) before `git apply --check`ing the target's `post-apply.patch` (V8), so recipe and patch are validated **independently against the same baseline** rather than double-applied. *(Amended: the original D7 text said "against the same shadow" — that predates the GH #2 reset and is superseded.)*
7. Prune.

**Per-slug shadow lock.** Verify and reconcile both write to `.tpatch/shadow/<slug>-<timestamp>/`. To prevent two concurrent writers, `verify` refuses when the lifecycle state is `reconciling` / `reconciling-shadow`. Per-slug only: verify on slug A while reconcile runs on slug B is allowed.

**Why closure replay is verify-only.** No other code path replays parent closures into shadows. `apply` works against the live tree (parents already applied locally). `reconcile` works against the upstream baseline + the target's own recipe (parents are out of band, by design — see ADR-010 D2). The closure-replay primitive lives in `internal/workflow/verify.go` only. If a future feature needs the same primitive, an ADR amendment factors it out.

**Cost.** O(closure size) shadow operations per verify. Bounded by DAG depth × per-recipe replay cost; comparable to a phase-2 reconcile op-replay pass per parent. Well within the cheap-budget for typical 1–3-deep DAGs.

> **Amended 2026-08-12 by Amendment 1 rev-7 (GH #8).** D7's machinery is
> retained in full — one shadow, topological replay, first-failure fail-fast,
> deferred prune, and the GH #2 reset between the recipe and the patch check.
> Amendment 1 refines three things.
>
> (a) The shadow's **root commit** becomes anchor-dependent: it stays `HEAD`
> for a feature with no landing evidence and becomes the **replay-anchor**
> commit's single parent tree in landed mode, where that anchor is chosen by
> collecting **every** qualifying candidate and comparing normalized change
> identities before selection (D9/D14/D18).
> `gitutil.CreateShadow` already accepts an arbitrary commit-ish, so this is a
> parameter change, not a new mechanism.
>
> (b) Step 3 of the algorithm above generalises to "skip members whose content
> is already present on the anchor", decided by the **non-mutating patch
> ladder** of D12/D13 — never by recipe replay and never by whole-file byte
> equality. `upstream_merged` is one case, a landed member is another, and an
> unattributed already-present member is a third. Each landed member resolves
> **its own** anchor (D14/D15), so a parent is never judged against a tree that
> already contains its own postimage.
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
- `internal/store/store.go:210-236` — `ListFeatures`, the silent-skip helper
  D17 rejects (skip at `:226`)
- `internal/store/store.go:238-245` — `FeatureEntry` (D17)
- `internal/store/store.go:274-348` — `ListFeatureEntries`, slug-sorted at
  `:343-345`, workspace-corruption error at `:295-306` (D17)
- `internal/store/dag.go:107` — `TopologicalOrder` (D7 closure-replay ordering)
- `internal/store/validation.go:38–44, 101–108` — `satisfiedBySHA` regex + `gitutil.IsAncestor` reachability (V5 reuse)
- `internal/workflow/dependency_gate.go:79` — `CheckDependencyGate` (D2 anchor; explicitly NOT modified)
- `internal/workflow/labels.go:89` — `ComposeLabels` (D5 derivation site)
- `internal/workflow/labels.go:143` — `composeLabelsFromStatus` (D5 pure-function host; freshness derivation lives here)
- `internal/workflow/created_by_gate.go:57` — `checkCreatedByGate` (V3 reuse)
- `internal/gitutil/shadow.go:56` — `CreateShadow` (D7 reuse)
- `internal/gitutil/gitutil.go:828` — `IsAncestor` (V5 reuse; anchor re-validated 2026-08-12 during Amendment 1 — the pre-amendment `:680` citation had drifted)

---
# Amendment 1 (2026-08-12, **rev-7 — final**) — Landed-evidence semantics — v0.15.1 Wave B / GH #8

**Status of this amendment**: proposed **rev-7 (final)**, AWAITING REVIEW. The decision set is **D8–D19** and is closed — no decision is added after D19. Binding on the
Wave C implementation once accepted. Adds D8–D19. **No prior decision D1–D7 is
reversed**; D7 is *extended* (anchor-dependent shadow root, plus an
index-isolated worktree-free assertion that allocates no shadow).

**Issue**: <https://github.com/tesseracode/tesserapatch/issues/8>
**Co-amended**: `docs/prds/PRD-verify-freshness.md` §3.6 / §4.3.6–4.3.9 / §7.1;
`docs/prds/PRD-tpatch-land.md` §3.8 / §6.2.

**Revision history.**

- **rev-0** — omitted V10; judged a landed feature only at `HEAD` with
  byte-exact post-state predicates.
- **rev-1** — restored the eleven-check schema and dual anchors, but ran the
  current assertion against the **working tree**, left the `-C0` hardening
  optional, let an unavailable historical anchor degrade to skip-and-pass,
  reused byte equality and a circular reference to replay in parent
  arbitration, and derived later-touch from byte differences.
- **rev-2** — closed those eight, but left nine residuals: stop-at-first anchor
  selection, parent V10 evaluated at a baseline that may already contain the
  parent's postimage, a snapshot that omitted later-feature metadata,
  overlapping empty/absent patch states, `land` able to emit an invalid
  `Tpatch-Base-Commit`, shallow history misclassified as root topology, anchor
  qualification borrowing the *reverse* ladder, raw duplicate comparison
  rejecting healthy cherry-picks, and Q15 ignoring the shipped recipe
  provenance.
- **rev-3** — closed those nine, but wrote an invalid parent-tree revision
  (`C^{tree}^`), built the inventory on `ListFeatures` (which silently drops
  unreadable features), promised "mutating nothing" for a land refusal that
  cannot keep it, qualified anchors at default context (which can dead-end its
  own remediation), left hunk positions in the duplicate identity (rejecting
  healthy cherry-picks), and allowed lazy promisor fetches during an
  offline-by-construction command.
- **rev-7 (this revision)** — one-line guard fix: §7.1.2 **G5** gains a narrow
  exemption (`derived|--show-object-format|would reject|rejects|fails this row`)
  so the pattern stops flagging the correct affirmative anti-hardcode sentences
  in land rule 18 and AC-LD5 / AC-LD19, while an affirmative "is 40 lowercase
  hex" claim still trips it; **G3**'s class widens to `rev-[0-6]`. No decision,
  row count or citation changed.
- **rev-6 (superseded)** — totality sweep: V10 restored to the main §3.1 check
  table with the GH #2 shadow-reset semantics, the amendment's exact
  (non-neutral) `land` scope stated in all five places, absence-is-never-a-
  mismatch propagated into the producer rules, and the mechanical **AC-L135 /
  §7.1.2** documentation guard added.
- **rev-5 (superseded)** — final contract cleanup: the effective Git floor
  becomes **2.36** (D17) because `GIT_NO_LAZY_FETCH` is mandatory; artifact
  presence short-circuits **before** classification so `exact`/`stale` are
  reachable only from `present-nonempty` and the unreachable
  `exact + absent/empty` arbitration rows are removed (D10/D13); Mode A's
  no-mutation promise is scoped around the mandatory `recoverLand` step (D19);
  the real-filtered-remote partial-clone behaviour is marked a **Wave C
  acceptance gate** rather than a claimed fact (D16); and all authoritative
  prose, labels and acceptance rows are swept for parity.
- **rev-4** — corrected all eight rev-3 findings: `C^` syntax (D14/E43),
  `ListFeatureEntries` inventory (D17), `land` validation split by invocation
  mode (D19), `-C1` forward qualification (D14/E44), hunk-position
  normalization (D18/E45–E46), `GIT_NO_LAZY_FETCH=1` on every object command
  (D11/D16/E47), preflight-before-parent-count ordering (D16), and reconciled
  acceptance rows.

- **rev-3 (superseded)** — closed all nine rev-2 residuals. Anchor candidates are
  **collected exhaustively and qualified by forward apply** (D14); duplicate
  comparison uses a **normalized zero-context change identity** (D18); each
  landed member gets its **own** V10 baseline and unlanded members use
  **`RecipeProvenance.BaseCommit`**, resolving Q15 (D15); the snapshot is a
  **full repository metadata inventory** (D17); patch presence is a **closed
  three-state precedence** (D10); `land` **refuses** an invalid base commit
  (D19); shallow and partial histories get **their own states** (D16); and the
  base-commit hex length is **derived from the repository object format**
  (D10).

## A1.0 Context — what GH #8 actually is

`tpatch land` (v0.8.0, ADR-019) commits the feature into reachable Git history
while intentionally leaving `status.apply.base_commit` unchanged
(`docs/prds/PRD-tpatch-land.md` §3.6; `internal/cli/land.go:394`). `verify`
allocates its V7/V8 shadow from **current HEAD**
(`internal/workflow/verify.go:1012`, `:1024`), so after `land` the baseline
already contains the feature.

Measured with the real CLI at `13a885c`; every run reported `checks=11` — the
shipped set is **V0–V10** (`internal/workflow/verify.go:49-71`), V10 appended
last (`:288-289`):

| Target recipe op kind | pre-land V7/V8 | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file`      | PASS / PASS | **PASS — false green** (`internal/workflow/verify.go:1290-1294`) | **FAIL — false red** |
| `replace-in-file` | PASS / PASS | **FAIL — false red** (`internal/workflow/verify.go:1295-1305`) | SKIP |
| `append-file`     | PASS / PASS | **PASS — false green, shadow double-appended** (`internal/workflow/verify.go:1306-1313`) | **FAIL — false red** |

The defect is not V8-only, and the same double-apply hazard applies to a landed
hard parent (`internal/workflow/verify.go:1048-1091`).

## A1.1 Empirical basis (read-only probes, git 2.55.0 / macOS, scratch removed)

E1–E33 were measured for rev-0…rev-2 and still hold; rev-3 added E34–E42 and rev-4 added E43–E47. The index is **E1–E47**, contiguous, and is closed for this amendment.

| # | Observation | Consequence |
|---|---|---|
| E1 | `git apply --check --reverse` succeeds at any tree containing equivalent content, including one produced by an unrelated actor. | Reverse-apply is a materialization signal only, never ownership proof. (D10) |
| E2 | `git log --grep '^Tpatch-Feature: <slug>$'` matches a commit whose prose body quotes that line; `%(trailers:…)` returns empty for it. | `--grep` is a prefilter; see E31. (D10) |
| E3 | Git parses trailers only from the last paragraph; a `--amend` appending prose empties `%(trailers:…)` while raw `%B` keeps the line. | Trailer loss is `malformed`, and only the raw message can classify it. (D10) |
| E4 | `--grep '^Tpatch-Feature: my-slug$'` does not match `my-slug-extended`. | Exact-value slug matching. (D10) |
| E5 | `cherry-pick`/`rebase` copy the trailer block verbatim; SHA and parent change; `Tpatch-Base-Commit` may become unreachable. | Evidence keys on trailer values, never the landing SHA or parent identity. (D10/D14) |
| E6 | A single commit can carry several `Tpatch-Feature` values. | Cardinality must be decided. (D10) |
| E7 | `git revert` of a landing does not copy trailers; the landing stays reachable with the content gone. | Reachability ≠ materialization. (D12) |
| E8 | One `git log -z --format=…` yields SHA, `%P`, raw `%B` and all four parsed trailers. | One enumeration per run. (D17) |
| E9 | A landing merged from a side branch is reachable only through the non-first parent. | Full-graph reachability, never `--first-parent`. (D10) |
| E10 | `RecipeFromPatch` emits `{type,path,content}` — **no `preimage_hash`** (`internal/workflow/recipe_autogen.go:114-118`). | Autogenerated recipes take the ADR-029 D4 legacy path. (D15) |
| E11 | With a genuine `preimage_hash`, V10 fails for an `applied`, un-landed feature: `expected preimage sha256:5fb14…, observed sha256:fa6dd8…`. | V10's live-tree reference is wrong for any applied feature. (D15 — now resolved by provenance, E42.) |
| E12 | Same for `preimage_hash: ""` (new-file): after apply the file exists ⇒ `new-file collision`. | Both preimage shapes break post-apply. (D15) |
| E13 | `checkWriteFilePreimage` reads from `repoRoot` — the live working tree (`internal/workflow/writefile_safety.go:108-112`). | V10 must be re-anchored. (D15) |
| E14 | Exhaustive enumeration, alphabet `{a,b,X}`: the rev-0 `replace-in-file` round-trip predicate gave **204 false reds and 15 933 false greens** across 56 784 decided cases. | The rev-0 predicate is unsound in both directions. |
| E15 | The existential-inverse predicate over **every** occurrence of `R` gave **0 false reds and 0 false greens** over the same 52 416 decided cases, with 4 368 undecidable (`R == ""` or `S == ""`). | Sound on the enumerated domain; retained for diagnostics. (D18 note) |
| E16 | Go: `strings.Replace("abc","","Z",1) == "Zabc"`; `strings.Contains(x,"") == true`. | `search == ""` degenerate; `replace == ""` cannot attest. |
| E17 | Duplicate trailers are observable but ambiguous: two `Tpatch-Patch-SHA` lines yield `aaaa,bbbb`. | Strict cardinality is mandatory. (D10) |
| E18 | Git trailer keys are **case-insensitive**: `tpatch-feature:` is returned by `%(trailers:key=Tpatch-Feature,valueonly)`. | The reader inherits it; the contract states it. (D10) |
| E19 | `readRecipeSHA` returns `none` on a read error **and** on `strings.TrimSpace(...) == ""` (`internal/cli/land.go:1034-1043`). | `none` covers absent **and** whitespace-only recipes. (D10) |
| E20 | Pre-land the real CLI reported `checks=11` on every run, `write_file_preimage_fresh` last. | Eleven checks. (D8) |
| E21 | A landed hard parent + an unlanded applied child passes today only because the parent's `write-file` replay is an idempotent no-op. | Parent arbitration must not rely on op-kind luck. (D13) |
| E22 | Forward apply fails at HEAD while reverse succeeds; forward succeeds at `status.apply.base_commit`. | GH #8 is baseline anchoring. (A1.0) |
| E23 | **A dirty worktree produces a false red**: with the feature reverted in the worktree only, `git apply --check --reverse` FAILS while `GIT_INDEX_FILE=<tmp> git read-tree HEAD` + `--cached` succeeds. | Anchor C must never read the worktree. (D11) |
| E24 | A temp index under `$(git rev-parse --git-dir)` leaves the real index and worktree byte-identical and is invisible to `git status`; one inside the working tree appears as an untracked entry. | Placement and cleanup rules. (D11) |
| E25 | `GIT_INDEX_FILE=<tmp> git read-tree <arbitrary-tree>` + `git apply --cached` probes any tree with zero worktree or real-index mutation. | Tree probing without a checkout. (D14) |
| E26 | Hardened ladder over trees (`C3`/`C0`/`(0/0)`-count): pristine `OK/OK/0`; landing parent `FAIL/FAIL/0`; 10 lines prepended `OK/OK/0`; unrelated edit far away `OK/OK/0`; edit **2 lines** away `FAIL/OK/0`; edit **1 line** away `FAIL/OK/1`; partial revert of hunk 1, of hunk 2, of hunk 3, of hunks 1+3 all `FAIL/FAIL/0`; full revert `FAIL/FAIL/0`; degenerate whole-file hunk + header/footer `FAIL/OK/0`; **revert-in-place + identical text pasted at EOF `FAIL/OK/1`**; patched file deleted `FAIL/FAIL/0`. | The `(0/0)` count is the discriminator. (D12) |
| E27 | **Retraction.** rev-1's "fails at every level" generalisation is withdrawn; only the E26 triples are claimed. | No generalised language. (D12) |
| E28 | Randomized 220-tree corpus, hardened rule: **0 false greens over 69 postimage-absent trees**, **26 false reds over 151 present trees**, each carrying exactly one `(0/0)`. | Hardening sound by measurement; cost quantified. (D12) |
| E29 | The **unhardened** rule leaked **2 false greens over the same 69 absent trees**. | Q14 cannot be optional. (D12) |
| E30 | After a re-record + re-land, the newest landing's parent tree **already materializes** the canonical patch while the earlier landing's parent does not. | Attestation ≠ replay anchor. (D14) |
| E31 | A prose quotation and an amend-destroyed trailer block both yield an empty parsed value with the raw line still present; nothing separates them. | Both are `malformed`; the prose false red is accepted. (D10) |
| E32 | `git log --topo-order --reverse -z --format='%H%x1f%P%x1f…%x1f%B'` emits oldest-first records with parent count, all four trailers and the raw body in one invocation; `rev-list` cannot emit `%B`. | One `git log`, no `rev-list`. (D17) |
| E33 | A root landing has **0** parents and `git read-tree <root>^` fails with `fatal: Not a valid object name`; a merge landing has **2** and its trailer parses normally. | Topology classification. (D16) |
| **E34** | **Forward vs reverse at a candidate parent.** At `L^` (clean pre-landing tree): `git apply --check --cached` **OK**, `--reverse --cached` **FAIL**. At `L` itself: forward **FAIL**, reverse **OK**. | Qualification asks *"does the patch apply here"* — the **forward** question. The reverse ladder answers the inverted question and is the wrong instrument for anchor selection. (D14) |
| **E35** | **Correction to a rev-3 draft claim.** A candidate parent carrying unrelated drift within the patch's context window does **not** forward-qualify (`forward FAIL`). Forward apply is context-sensitive like any `git apply`. | The contract claims only that forward apply is the *right question*, never that it is immune to context. Such a candidate is honestly disqualified. (D14) |
| **E36** | **Normalization is necessary and sufficient.** Same logical change on two branches with different preimage blobs: raw `-U3` hashes **differ**; `-U0` alone still **differs** (index lines); `-U0` with `^index ` lines removed is **EQUAL**. Observed index lines: `index 58b8997..c3badef 100644` vs `index ed24a75..1b142d1 100644`. | `--unified=0` **and** stripping `^index ` are both required. (D18) |
| **E37** | `-U0 --binary --no-renames` with `^index ` stripped **preserves** `old mode`/`new mode` (4 lines), `GIT binary patch` (1), and the `new file mode` / `deleted file mode` rename split (2). | Normalization does not erase mode, binary or rename semantics. (D18) |
| **E38** | **Shallow boundary is indistinguishable from a root by `%P` alone.** In a `--depth 2` clone the boundary commit reports `parents_in_%P = 0`, exactly like a real root, and `git log` marks it `(grafted)`. The discriminators are `git rev-parse --is-shallow-repository == true` and the boundary SHA appearing in `.git/shallow`; a full repository has no `.git/shallow` file. | rev-2's "0 parents ⇒ root ⇒ `unsupported-topology`" **misclassifies** shallow history with the wrong remediation. (D16) |
| **E39** | `git read-tree <boundary>^` in a shallow clone fails with `fatal: Not a valid object name`, the same error text as a true root. | The error alone cannot classify; the preflight must. (D16) |
| **E40** | **Partial (blobless) clone**: `--is-shallow-repository` is `false`, `remote.origin.promisor` is `true`, `remote.origin.partialclonefilter` is `blob:none`, commits and trees are local and `read-tree HEAD` succeeds; blobs are fetched lazily, so content operations may require the network. | Partial clones are usable but can fail offline; that failure is `history-incomplete`, not `unsupported-topology`. (D16) |
| **E41** | **Object format determines hex length.** `git rev-parse --show-object-format` returns `sha1` with 40-hex commit ids, and a repository created with `--object-format=sha256` returns `sha256` with **64**-hex ids. | The `Tpatch-Base-Commit` length rule must be **derived**, not hardcoded to 40. (D10/D19) |
| **E43** | **`C^{tree}^` is invalid.** `git rev-parse <C>^{tree}^` returns `error: object <tree> is a tree, not a commit`, and `git read-tree <C>^{tree}^` fails with `fatal: Not a valid object name`. The valid forms are `C^`, `C^^{tree}`, `C~1` and `C~1^{tree}`, all of which `read-tree` accepts. | rev-3's parent-tree syntax was wrong; D14 uses `C^`. |
| **E44** | **Forward qualification ladder at a candidate parent tree** (`C3`/`C1`/`C0`): pristine pre-landing parent `OK/OK/OK`; unrelated edit **4 lines** from the hunk `OK/OK/OK`; unrelated edit **2 lines** away `FAIL/**OK**/OK`; unrelated edit **1 line** away `FAIL/FAIL/OK`; 10 lines prepended (pure offset) `OK/OK/OK`; unrelated edit far away `OK/OK/OK`; **tree that already has the feature `FAIL/FAIL/FAIL`**; feature line changed to something else `FAIL/FAIL/FAIL`; feature line deleted `FAIL/FAIL/FAIL`. | `-C1` is the level that keeps healthy parents while still rejecting every already-materialized or modified tree. Default context dead-ends the operator's own remediation; `-C0` is over-permissive. (D14) |
| **E45** | **rev-3's duplicate identity rejects a healthy shifted cherry-pick.** The same payload applied after 5 unrelated lines were prepended yields hunk headers `@@ -10 +10 @@ l9` vs `@@ -15 +15 @@ l9`, so `-U0` minus `index` hashes **differ**. Rewriting every hunk header to a bare `@@` token makes them **equal**. | Hunk positions must be normalized. (D18) |
| **E46** | **The trade is bounded.** Normalizing hunk positions makes two changes with **identical `-`/`+` bodies at different positions** compare equal — measured with a file containing duplicate lines (`-DUP` / `+DUP CHANGED` at line 2 vs line 4 normalize identically). When the removed or added text differs at all, the bodies differ and the identities stay distinct; distinct payload, mode-only change and different path all remain distinct. | The collision requires duplicate line content at the changed lines; documented, not hidden. (D18) |
| **E47** | **Lazy fetch breaks offline verification, and `GIT_NO_LAZY_FETCH=1` fixes it.** With a promisor remote configured and its object physically absent, `git cat-file -p <blob>` **attempts the network** and fails with `fatal: '<url>' does not appear to be a git repository` / `Could not read from remote repository.`; the same command under `GIT_NO_LAZY_FETCH=1` fails immediately and locally with `fatal: Not a valid object name <sha>`. The variable is accepted by git 2.55.0 and does not disturb the normal path. | Every object and materialization command must carry `GIT_NO_LAZY_FETCH=1`; a missing promisor object then classifies as `history-incomplete` without any network. (D16) |
| **E42** | **`RecipeProvenance` already exists and is hash-bound.** `internal/workflow/implement.go:30-34` defines `{base_commit, generated_at, recipe_sha256}`; `implement` writes `artifacts/recipe-provenance.json` at `:220-238` with `BaseCommit = gitutil.HeadCommit(...)` at generation time and `RecipeSHA256 = sha256(recipe bytes as re-read from disk)`. `warnRecipeStale` (`internal/cli/cobra.go:1285-1315`) already consumes both. `RecipeFromPatch` — the autogen path that omits `preimage_hash` (E10) — has **no** provenance writer, so the two travel together: a recipe carrying `preimage_hash` was written by `implement`, which also wrote the provenance. | Forward-mode V10 **does** have an anchor. Q15's "no anchor exists" rationale was wrong and is withdrawn. (D15) |

---

## Decision (Amendment 1 rev-7 — final, D8–D19)

### D8. The check set is eleven checks, V0–V10; no identifier changes

`internal/workflow/verify.go:49-71` defines eleven check IDs; V10 is
`write_file_preimage_fresh`, appended last (`:288-289`). Every report shape,
golden example and acceptance row describes **eleven** rows in that order. No
check is added or removed: the current-materialization assertion folds into
**V8**, whose subject is the artifact the landing trailer attests.

### D9. Baseline model: dual-anchor landed verification

- **Anchor H (historical)** — a shadow rooted at the **replay anchor's** single
  parent tree (D14). Closure arbitration (D13) runs, then the existing
  machinery: V7 replays the recipe (`internal/workflow/verify.go:1275`), the
  shadow is reset to `closureBaselineTree`, V8 runs `git apply --check` forward
  (`:1092`, `:1143`, `:1156`), V10 evaluates preimages (D15).
- **Anchor C (current)** — an **index-isolated** assertion at `HEAD` (D11)
  running the hardened ladder (D12). No shadow, no worktree read, no
  real-index read.

**V7's independent obligation.** V7 at anchor H proves the recipe still
forward-applies to the tree it was authored against, with its closure
reconstructed. That is different from "the canonical patch's postimage is
present at HEAD" (anchor C) and from "the patch applies at the landing
baseline" (V8's historical half). V7 never aliases V8.

**GH #2 invariant, binding in every mode.** The recipe and the patch are
validated independently against the same baseline tree with a shadow reset
between them (`internal/workflow/verify.go:1092`, `:1143`). Normative: *any
check that may mutate the shadow MUST reset it to `closureBaselineTree` before
the next check runs; V7's result is never an input to V8's tree.* Anchor C
mutates nothing.

**Non-landed features are untouched**: evidence `none` ⇒ shadow at `HEAD`,
V7/V8 byte-for-byte as today (V10 changes per D15).

### D10. Evidence reader: one enumeration, raw and parsed, conservative grammar, closed presence states

**Enumeration.** Exactly one
`git log --topo-order --reverse -z --format='%H%x1f%P%x1f<four trailers>%x1f%B'`
invocation per verify run over commits reachable from the resolved `HEAD`,
**cached and reused for every feature** of a `verify --all` run (E32). Records
arrive **oldest-first**. `rev-list` is **not** used — it cannot emit `%B`.
Never `--first-parent` (E9).

**Conservative raw precedence.** Any commit whose **raw** message contains a
line exactly `Tpatch-Feature: <slug>` (after trimming trailing ASCII
space/tab) but whose **parsed terminal trailer block** does not yield that
slug is **`malformed`** — never `none`. E31 measured that a prose quotation
and an amend-destroyed block are indistinguishable; the prose false red is
accepted deliberately rather than risk reading a destroyed attestation as "no
attestation".

**Grammar (normative).**

| Element | Rule |
|---|---|
| Key case | Git matches trailer keys **case-insensitively** (E18). Inherited; stated, not denied. |
| `Tpatch-Feature` cardinality | **Exactly one value.** Two or more (E6) ⇒ `malformed`: sibling SHA trailers cannot be attributed to a slug. |
| `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` / `Tpatch-Base-Commit` | **Exactly one each.** Zero or ≥2 ⇒ `malformed` (E17). No "take the first". |
| Slug match | Exact equality after trimming leading/trailing ASCII space and tab (E4). |
| `Tpatch-Patch-SHA` format | 64 lowercase hex (SHA-256 of the artifact bytes; independent of the repository object format). |
| `Tpatch-Recipe-SHA` format | 64 lowercase hex **or** the literal `none`. |
| `Tpatch-Base-Commit` format | **`N` lowercase hex, where `N` is derived from `git rev-parse --show-object-format`**: 40 for `sha1`, 64 for `sha256` (E41). Hardcoding 40 would reject every valid landing in a SHA-256 repository. If the object format cannot be read, the reader reports `unavailable` rather than guessing. |
| Reader failure | Git error, unparsable output, or a git below the D17 floor ⇒ **`unavailable`**, a block failure, distinct from `none` and `malformed`. |

Lowercase strictness follows the ADR-029 D1 precedent enforced by
`isLowercaseHex` (`internal/workflow/writefile_safety.go:176`).

**Artifact presence — three closed, mutually exclusive states, evaluated
*first*, before any digest comparison and therefore before any `exact` /
`stale` classification. `exact` and `stale` are reachable only from
`present-nonempty`; `absent` and `present-empty` short-circuit to
`landed-artifacts-absent` (terminal).**

| State | Definition | Effect on a landing candidate |
|---|---|---|
| **absent** | the artifact file does not exist in the snapshot | **Terminal `landed-artifacts-absent`.** The candidate cannot be an `exact` **or** `stale` attestation, because no digest exists to compare. **No digest comparison is attempted**, no mismatch is reported, and no downstream arbitration row for this member is reachable. |
| **present-empty** | the file exists and is zero bytes (for the recipe: zero bytes **or** whitespace-only) | For the **patch**: **terminal `landed-artifacts-absent`**, same short-circuit as `absent` — `land` refuses when the embedded `record` would capture nothing, so a landed empty patch is corruption or a hand edit, and it can be neither `exact` nor `stale`. For the **recipe**: the expected trailer value is the literal `none`, mirroring `readRecipeSHA` (E19); the recipe's shape never rescues an absent-or-empty patch. |
| **present-nonempty** | the file exists with ≥1 byte (recipe: ≥1 non-whitespace byte) | the only state in which a digest comparison happens, and the only state in which the **patch** can support `exact` authority. |

The recipe's four shapes are named separately and do not overlap: **absent**,
**present-empty** (zero-byte or whitespace-only ⇒ expected trailer `none`),
**present-nonempty-zero-op** (parses, `operations` is empty ⇒ V7 has nothing
to replay and cannot attest), and **present-nonempty-with-ops**. **Recipe
shape is evaluated only within a `present-nonempty` patch state** — the two
short-circuiting patch states terminate the member before the recipe is
consulted for classification.

**Reachable outcome table — 3 patch states × 4 recipe shapes, mutually
exclusive and total.**

| patch \ recipe | absent | present-empty | present-nonempty-zero-op | present-nonempty-with-ops |
|---|---|---|---|---|
| **absent** | `landed-artifacts-absent` | `landed-artifacts-absent` | `landed-artifacts-absent` | `landed-artifacts-absent` |
| **present-empty** | `landed-artifacts-absent` | `landed-artifacts-absent` | `landed-artifacts-absent` | `landed-artifacts-absent` |
| **present-nonempty** | digest compared; recipe trailer must be `none`; patch ladder is sole authority; V7/V10 skip | digest compared; recipe trailer must be `none`; patch ladder is sole authority; V7/V10 skip | digest compared; V7 records `0 op(s)` and cannot attest; patch ladder is sole authority | digest compared; V7 replays; patch ladder and V7 both apply |

Every cell is reachable and no cell matches two rules. The eight
`landed-artifacts-absent` cells are the corruption/hand-edit region `land`
cannot produce.

**Evidence states — closed set of ten, total:** `none`, `exact`,
`duplicate-equivalent`, `stale`, `ambiguous`, `malformed`,
`unsupported-topology`, **`shallow-history`**, **`history-incomplete`**,
`unavailable`. Only `none` degrades to forward mode; the other eight
non-`exact` states are terminal.

### D11. Anchor C is index-isolated

1. Create a temporary index under `$(git rev-parse --git-dir)` (E24) — or,
   equivalently, under the already-gitignored `.tpatch/local/` root
   (`internal/cli/land_journal.go:31`, `:60`;
   `internal/gitutil/ignore.go:50-51`). **Never** inside the tracked working
   tree.
2. `GIT_NO_LAZY_FETCH=1 GIT_INDEX_FILE=<tmp> git read-tree <commit-or-tree>` —
   `HEAD` for anchor C, `C^` for a candidate-parent probe (D14). Never
   `C^{tree}^` (E43).
3. `GIT_NO_LAZY_FETCH=1 GIT_INDEX_FILE=<tmp> git apply --check [--reverse] --cached [-C1|-C0 --verbose] <patch>`.
4. Remove the temp index on **every** exit path, in a deferred cleanup.

**`GIT_NO_LAZY_FETCH=1` is mandatory on every object and materialization
command** — `read-tree`, `apply`, `diff`, `cat-file`, `merge-base` and the
enumeration itself. Measured (E47): without it, a missing promisor object makes
git **attempt the network**; with it, the same object fails immediately and
locally, which is what lets D16 classify `history-incomplete` deterministically.
Verify is offline by construction (§1.3), so a silent network fetch is a
contract violation, not merely slow.

**Read-only guarantees (E24).** Real index byte-identical, worktree
byte-identical, `git status --porcelain` unchanged, temp index never an
untracked entry. Results are **memoised per `(tree, patch, direction, context)`**
for the run.

### D12. The hardened ladder — `(0/0)` blocks

| Step | Command (all against the temp index) | Outcome |
|---|---|---|
| 1 | `git apply --check --reverse --cached <patch>` | pass ⇒ **materialized, clean** |
| 2 | on failure: `LC_ALL=C git apply --check --reverse --cached -C0 --verbose <patch>` | pass **and zero** `Context reduced to (0/0)` ⇒ **materialized, context drift**: block check passes, `warn` `context-drift` advisory |
| 3 | step 2 passes **but reports ≥1** `(0/0)`, **or** step 2 fails | **BLOCK** — `landed-content-absent` |

`LC_ALL=C` is mandatory. Measured: hardened **0 false greens / 69 absent**
(E28) against the unhardened rule's **2 / 69** (E29), at a cost of **26 false
reds / 151 present**, each carrying remediation R2. A stronger hunk-local
corroboration was considered and **not adopted** — none could be *proved* on
the measured corpus.

**Advisory vocabulary.** The closed set shared with the PRD is
`context-drift`, `later-touch`, `unattributed-materialized`,
`base-commit-unreachable`, **`provenance-unreachable`**; all `warn`, none
flips `passed`.

**No generalised claims (E27).** Only the E26 per-scenario triples are
asserted, including four distinct partial-revert shapes and the full revert.

### D13. Parent arbitration is non-mutating and patch-ladder-only

**The presence test for any closure member is the D12 hardened ladder applied
to that member's canonical `post-apply.patch` against the anchor tree, probed
through the D11 temp index.** It is non-mutating, it is **not** recipe replay
(replay is what arbitration decides about), and it is **not** whole-file byte
equality. Recipe operation predicates are **diagnostics only**: they localise
which op and path a failure concerns and feed the D15 write-file signals, and
they never certify presence.

| Member condition | Action |
|---|---|
| `upstream_merged` | **skip** (unchanged, `internal/workflow/verify.go:1062-1064`) |
| superseded by an active superseder | **skip** (unchanged, `internal/workflow/verify.go:976-983`) |
| evidence `exact`/`duplicate-equivalent`, patch **present-nonempty**, ladder ⇒ clean or context-drift | **skip** |
| evidence `exact`/`duplicate-equivalent`, patch present-nonempty, ladder ⇒ block | **fail-fast** `parent-landing-drift` |
| patch **absent** or **present-empty**, whatever the recipe shape | **fail-fast** `landed-artifacts-absent` — reached **before** any digest comparison and therefore **before** any `exact`/`stale` classification (D10). A slug-bearing candidate on such a member never becomes an attestation. |
| evidence `none`, patch present-nonempty, ladder ⇒ clean or context-drift | **skip** with a mandatory `warn` `unattributed-materialized` advisory. Verify claims **no ownership**. |
| evidence `none`, patch present-nonempty, ladder ⇒ block | **replay** (unchanged, `internal/workflow/verify.go:1065-1082`) |
| evidence `none`, patch absent or present-empty | **replay** |
| evidence `stale` / `ambiguous` / `malformed` / `unsupported-topology` / `shallow-history` / `history-incomplete` / `unavailable` | **fail-fast** `parent-evidence-integrity` |
| `unapplied` (ADR-032) | **fail-fast** `parent-unapplied` |
| `rejected` (ADR-031) | **fail-fast** `parent-rejected` |
| any other state | **fail-fast** (unchanged `default:`) |

A landed member additionally contributes its **V10** result per D15.

**Revert timing.** "Reverted" means the member's canonical patch fails the D12
ladder **at the anchor tree being built**. A revert landed *after* the anchor
commit is invisible at anchor H and is caught at anchor C; a revert predating
the anchor makes the content absent from the anchor and is caught there. Both
anchors are evaluated independently and both are reported.

**`active` is total.** Treated **identically to `applied`** everywhere in the
closure. Today the switch handles only `upstream_merged` and `applied`, so an
`active` hard parent reaches `default:` and fail-fasts
(`internal/workflow/verify.go:1061-1089`) — while `CheckDependencyGate`
accepts both (`internal/workflow/dependency_gate.go:79-81`),
`postApplyVerifyStates` admits `active` (`internal/workflow/verify.go:127-134`)
and `isPostApplyState` does too (`internal/workflow/verify_all.go:89-97`).
Widening the switch is the smallest change that makes all four sites agree,
and it is a **deliberate behaviour change for non-landed features**.

### D14. Anchor candidates are **collected exhaustively** and qualified by **forward** apply

rev-2 said "take the first qualifying candidate" while also requiring
non-equivalent qualifiers to be ambiguous — a contradiction, because
stop-at-first can never observe a second qualifier. rev-3 replaces the
procedure.

**Attestation candidate** — determines `landing_evidence.state`. Well-formed,
single-`Tpatch-Feature`, exact-slug, patch **present-nonempty**, and its three
recorded values match the snapshot (D10). This is the **authority**.

**Replay-anchor candidates — collect, qualify, compare, select.**

1. **Collect.** From the single enumeration, take **every** commit that
   (a) is reachable from `HEAD`, (b) carries exactly one `Tpatch-Feature`
   value equal to the slug with a parseable terminal trailer block, and
   (c) has **exactly one** parent in `%P`. Their own recorded hashes **may be
   stale** — they are not authorities (D14 integrity boundary below).
2. **Qualify by forward apply at `-C1`.** For each collected candidate `C`,
   seed a temp index from **`C^`** — the commit's single parent — and run the
   forward check:

   ```
   GIT_NO_LAZY_FETCH=1 GIT_INDEX_FILE=<tmp> git read-tree C^
   GIT_NO_LAZY_FETCH=1 GIT_INDEX_FILE=<tmp> git apply --check --cached -C1 <canonical patch>
   ```

   `C` **qualifies** iff that forward check succeeds.

   **Syntax is normative (E43).** The revision string is **`C^`**, or
   **`C^^{tree}`** when a tree object is explicitly wanted. `C^{tree}^` is
   **invalid** — measured: `error: object <tree> is a tree, not a commit`,
   and `git read-tree C^{tree}^` fails outright. rev-3 wrote that form; it is
   corrected here and nowhere else in these documents uses it.

   **Context level is `-C1`, measured (E44).** At a pristine pre-landing
   parent all of `-C3`/`-C1`/`-C0` pass. A parent carrying an unrelated edit
   **2 lines** from the hunk fails at default context but **passes at `-C1`**;
   a tree that already contains the feature, one where the feature line was
   changed to something else, and one where it was deleted **all fail at
   `-C1`**. Default context therefore rejects healthy parents — including the
   parent an operator's own re-record-and-re-land remediation produces — while
   `-C1` keeps them and still rejects every already-materialized or otherwise
   modified tree. **`-C0` is rejected**: it discards context entirely, which is
   the same over-permissive setting D12 already refuses for the reverse
   direction.

   **Accepted bound (E44).** A parent carrying an unrelated edit **1 line**
   from the hunk does not qualify even at `-C1`. That is the measured false-red
   boundary, and it is accepted for the same reason D12's `(0/0)` block is:
   the remaining choice is `-C0`, which is not safe.

   Qualification asks *"does the current canonical patch apply here"* — the
   question a replay baseline must answer. The reverse ladder answers the
   inverted question and is the wrong instrument (E34).
3. **Compare.** If **more than one** candidate qualifies, compute the D18
   **normalized change identity** of each and compare. If all identities are
   equal, the candidates are equivalent. If any differ, the anchor is
   **ambiguous** ⇒ terminal (below). No candidate is selected before every
   qualifier has been collected and compared.
4. **Select.** Among equal-identity qualifiers, take the **first in the
   enumeration's native `--topo-order --reverse` (oldest-first) order**; final
   tie-break, the lexicographically smallest full commit id.

**No broadening**: the collection is confined to exact-slug trailer commits.
It never falls back to "any commit that introduced these paths".

**Integrity boundary — what a stale candidate may and may not do.** A
qualifying candidate whose own `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` /
`Tpatch-Base-Commit` no longer match the snapshot supplies **only** a tree to
root the shadow at. It never contributes to `landing_evidence.state`, never
relaxes the attestation requirement, and is never reported as the attestation
commit. The report carries both identities separately
(`landing_evidence.attestation_commit` and
`baseline.historical_anchor.replay_anchor_commit`).

**Unavailability is TERMINAL.** If the collection is empty, or no candidate
qualifies, or the qualifiers are non-equivalent, the run fails with
`failed_at: "historical-anchor-unavailable"`. V7, V8's historical half and V10
report `passed: false` with `mode` present — **failed-because-unanchored**,
not skipped — and the run **never** passes on anchor C alone.

**Re-land remediation regains anchor H.** After `tpatch record` +
`tpatch land`, the newest landing becomes the attestation authority and the
earlier landing remains a qualifying replay anchor (E30), so anchor H is
regained and the run passes. If the operator's history admits no qualifier,
the run fails with R11 rather than degrading. Both branches are pinned.

### D15. V10: per-member baselines, and Q15 resolved via `RecipeProvenance`

rev-2 evaluated V10 at the **target's** anchor-H closure baseline. For a
landed **parent** that baseline may already contain the parent's own
postimage — the parent landed earlier in history — so its `preimage_hash`
would be compared against a tree that already has the write applied. rev-3
gives every member its **own** baseline.

**Per-member V10 baseline selection.**

| Member condition | V10 reference tree |
|---|---|
| the **target**, landed | the target's anchor-H **closure baseline** (the shadow after arbitration, before the target's recipe replays) |
| a **landed** closure member | **that member's own** replay-anchor parent tree, resolved by running D14 steps 1–4 for that member's slug. Never the target's anchor. |
| an **unlanded** member (evidence `none`), or the target in forward mode | **`RecipeProvenance.BaseCommit`** for that member (below) |
| any member whose recipe carries no `preimage_hash` on a given op | ADR-029 D4 legacy path for that op: pass, no re-warn (`internal/workflow/verify.go:879-883`) |

**Q15 is RESOLVED, and rev-2's "no anchor exists" rationale is withdrawn.**
E42 measured that the shipped `RecipeProvenance`
(`internal/workflow/implement.go:30-34`) records `base_commit` — the `HEAD`
the recipe was generated against — together with `recipe_sha256`, and that
`implement` writes it (`:220-238`) on the same path that emits
`preimage_hash`, while the autogen path that omits `preimage_hash` writes no
provenance. The two therefore travel together.

**Exact use of provenance as the forward-mode V10 anchor.** For each
`write-file` op that carries a non-nil `preimage_hash`, the reference tree is
`RecipeProvenance.BaseCommit` **iff all four hold**:

1. **present** — `artifacts/recipe-provenance.json` exists in the snapshot and
   decodes;
2. **well-formed** — `base_commit` is `N` lowercase hex for the repository's
   object format (D10);
3. **reachable** — `gitutil.IsAncestor(root, base_commit, "HEAD")`
   (`internal/gitutil/gitutil.go:828`) is true, or the commit is `HEAD`
   itself. Unreachable ⇒ a `warn` `provenance-unreachable` advisory **and**
   the failure below, because an unreachable baseline cannot be read;
4. **snapshot-consistent** — `recipe_sha256`, when present, equals the sha256
   of the snapshot's `apply-recipe.json` bytes. When the field is absent
   (pre-v0.5.2 sidecars, which `warnRecipeStale` already tolerates,
   `internal/cli/cobra.go:1310-1313`) the binding is weaker; the contract
   accepts it and records `provenance_hash_bound: false` in the report so the
   operator can see the difference.

If a `write-file` op carries a real `preimage_hash` but provenance fails any
of 1–3, verify **FAILS** with `failed_at: "recipe-provenance-unavailable"`.
It **never** falls back to reading the live working tree, which is the
measured E11/E12 false block. Ops without `preimage_hash` remain on the
ADR-029 D4 legacy path regardless of provenance.

**V10 outcomes, per member.**

| Case | Outcome |
|---|---|
| `preimage_hash` absent | legacy pass, no re-warn (ADR-029 D4) |
| present and matching at the member's V10 reference tree | **PASS**, `mode` naming the anchor kind |
| present and **not** matching | **FAIL**, block severity; downgraded to `warn` when superseded (ADR-029 D7, `internal/workflow/verify.go:862-870`) |
| `preimage_hash` malformed per ADR-029 D1 | **FAIL**, block — the preimage contract itself is invalid |
| provenance required but absent / ill-formed / unreachable | **FAIL** `recipe-provenance-unavailable` |
| V2 skipped or failed | **skip**, unchanged reason (`internal/workflow/verify.go:853-861`) |
| the member's own anchor unavailable | **FAIL** `historical-anchor-unavailable` for that member |

**Current-half later-touch — from metadata, never bytes.** The signal is the
shipped detector: `RequestedAt` ordering
(`internal/workflow/writefile_safety.go:409-442`), touched-path union of
`patch-generations.json.touched_paths`
(`internal/store/patch_generations.go:52`) and recipe op paths
(`internal/workflow/writefile_safety.go:449-481`), the `path → first later
slug` index (`:380-388`) and the per-op predicate `checkLaterTouch`
(`:489-498`); `DetectRecordLaterTouchWarnings` (`:571`) is the exported
record-time entry point. A hit raises a **`warn` `later-touch` advisory** and
**never blocks** — ADR-029 D6 is warning-class while D5 makes stale preimages
fail — with one stated exception: a malformed `preimage_hash`, or a mismatch
at the member's reference tree, blocks on its own terms.

**Aggregation into the target verdict.** A member's **block-class** V10
outcome contributes to `parent-landing-drift` for that member and is named in
the fail-fast reason. A member's **warn-class** outcome is aggregated into the
run's `advisories`, attributed to the member's slug, and affects no verdict.
The target's own V10 row carries the target's block-class result and reports
its own `mode`.

### D16. Topology, shallow history and partial clones

**Preflight, once per run, strictly BEFORE any parent-count or topology
classification** — the ordering is normative, because E38 shows a shallow
boundary and a true root are indistinguishable by `%P` alone:

- `git rev-parse --is-shallow-repository` — `true` ⇒ the repository is
  shallow;
- `remote.<name>.promisor` / `remote.<name>.partialclonefilter` — set ⇒ the
  repository is a partial clone (E40).

**Classification.**

| Situation | State | Remediation |
|---|---|---|
| candidate has ≥2 parents | `unsupported-topology` | re-land on a linear commit (R9) |
| candidate has 0 parents **and** the repository is **not** shallow | `unsupported-topology` (a genuine root landing) | R9 |
| candidate has 0 parents **and** the repository **is** shallow, or the candidate SHA appears in `.git/shallow` | **`shallow-history`** | `git fetch --unshallow` (R21) |
| any object needed for enumeration, `read-tree`, `diff` or `apply` is missing locally — detected deterministically because every such command runs under `GIT_NO_LAZY_FETCH=1` (E47) | **`history-incomplete`** | `git fetch --refetch` / restore network access, then re-run (R22) |

**Scope of what is proven, without overclaim.** E47 proves the *mechanism*:
with a promisor remote configured and its object physically removed, the
default run attempts the network while the same run under
`GIT_NO_LAZY_FETCH=1` fails locally and immediately. It does **not** prove the
end-to-end partial-clone path — a `file://` `--filter=blob:none` clone on git
2.55.0 did **not** withhold blobs, so no genuinely missing promisor object
could be produced that way. Constructing one requires a **non-local
transport** (HTTP, SSH or `git daemon`) with `uploadpack.allowFilter=true`, or
a deterministic promisor fixture built by setting `extensions.partialclone`
plus a dead promisor URL and deleting the object. `history-incomplete` is
therefore a **specified behaviour with a proven mechanism and an unproven
end-to-end path**, and its validation is a **Wave C acceptance gate**
(AC-L68 / AC-L69): if the implementation environment cannot reproduce a real
filtered remote, Wave C **cannot claim completion** and must report a blocker
rather than mark those rows passed.

E38 is the reason this matters: in a `--depth 2` clone the boundary commit
reports `parents_in_%P = 0` **exactly like a real root**, and E39 shows
`read-tree <boundary>^` fails with the *same error text* as a true root.
Without the preflight the two are indistinguishable and the operator is told
to re-land when the actual fix is to deepen the clone.

Reachability remains full-graph (E9); merge commits are candidates only if
they themselves carry the trailer, and they are never anchors (E33). A
**CI shallow checkout** is the common case and gets R21, not R9.

### D17. Full metadata inventory snapshot, and honest invocation accounting

rev-2 snapshotted the target and its closure only. The shipped
`loadLaterFeatureTouches` reads **every** feature in the repository — it calls
`s.ListFeatures()` at `internal/workflow/writefile_safety.go:409-442` — so a
closure-only snapshot cannot make ADR-029 ordering deterministic. The
inventory therefore covers the whole repository; **and it is built from
`ListFeatureEntries`, not the `ListFeatures` the shipped detector happens to
use**, because that helper silently drops unreadable features (below).

**The inventory is built from `ListFeatureEntries`, not `ListFeatures`.**
`ListFeatures` (`internal/store/store.go:210-236`) **silently skips** any
feature whose `status.json` fails to load — `continue // skip features without
valid status.json` at `:226`. A snapshot built on it cannot represent an
unreadable feature at all, which is the same false-green class this amendment
exists to close. `ListFeatureEntries`
(`internal/store/store.go:274-348`) exists precisely for this: it returns a
`FeatureEntry{Slug, Status *FeatureStatus, Err error}`
(`internal/store/store.go:238-245`) per feature, sorted by slug
(`internal/store/store.go:344-346`), surfacing stat failures and decode
failures as error rows rather than dropping them.

**The inventory.** At the start of a run, verify captures **once**:

- every `FeatureEntry` from `store.ListFeatureEntries()`, **in the returned
  slug-sorted order**, which is the deterministic enumeration order for the
  whole run;
- for every entry with `Err == nil`: the decoded `FeatureStatus` including
  `RequestedAt`, the **presence state** (D10 three-state) and **raw bytes** of
  `artifacts/apply-recipe.json`, `artifacts/post-apply.patch` and
  `artifacts/recipe-provenance.json`, and the decoded
  `patch-generations.json` `touched_paths` union
  (`internal/store/patch_generations.go:52`);
- for every entry with `Err != nil`: the slug and the error, retained as an
  explicit **`unreadable`** row — never omitted;
- the repository preflight facts of D16 and the object format of D10.

Every later stage — evidence classification, V7, V8, V10, ADR-029 later-touch
ordering, the persisted `VerifyRecord` and the derived labels — consumes
**copies** from this one inventory and never re-reads disk.

**Read-error policy, chosen explicitly and with no silent skip.**

| Unreadable entry is… | Effect |
|---|---|
| the **target**, or any **closure member** | **block** — `failed_at: "inventory-unreadable"`, naming the slug and the underlying error. The run cannot reason about a member it cannot read. |
| any **other** feature | a **`warn` `inventory-unreadable` advisory** naming the slug, **and** the entry is **excluded from ADR-029 later-touch ordering**. This exactly matches the shipped detector, which already skips features whose `RequestedAt` is empty (`internal/workflow/writefile_safety.go:409-442`) — but the exclusion is now *reported* rather than invisible. |

`ListFeatureEntries` itself returns an error when `features/` is missing while
`.tpatch/` exists (`internal/store/store.go:294-302`); verify surfaces that as
a block rather than an empty inventory.

**Instability detection.** Before the report is finalised, every inventory
entry is re-stated — `ListFeatureEntries()` is re-run and each captured
artifact re-read — and compared. A slug appearing, disappearing, or changing
its `Err`-vs-`Status` disposition all count as changes. A feature **added**, **removed** or **changed** during
the run ⇒ **FAIL** `failed_at: "snapshot-unstable"` naming the slug and path.
This covers the concurrent-`tpatch add`/`remove` case a target-only snapshot
missed.

**Cost.** One `ListFeatureEntries` call plus O(features) artifact reads, once.
For a repository with `F` features and a closure of size `K`, that is `F`
status decodes and up to `3F` artifact reads, against `K` before — bounded,
sequential, and reused by `verify --all` for every feature in the run. No git
process is spawned by the inventory itself.

**Honest invocation budget.** rev-2's table omitted tree materialization and
per-member anchor resolution. The complete accounting:

| Purpose | Invocations |
|---|---|
| Repository preflight (`--is-shallow-repository`, `--show-object-format`, promisor config) | **3 per run**, cached. Runs **before** any topology classification (D16). |
| Inventory build (`store.ListFeatureEntries` + artifact reads) | **1 `ListFeatureEntries` per run**; `F` status decodes and up to `3F` artifact reads, cached across `verify --all`. No git invocation. |
| Evidence enumeration (`git log --topo-order --reverse -z --format=…`) | **1 per run**, cached across `verify --all`. **No `rev-list`.** |
| Shadow allocation at anchor H | 1 `CreateShadow` per **anchored member** whose recipe must replay — already allocated today for the target; only its commit-ish changes |
| Tree materialization for a candidate parent | 1 `git read-tree C^` **per distinct parent commit** (the parent id is already in the cached `%P`, so no extra `rev-parse` is needed; `C^^{tree}` is the explicit-tree form when one is wanted) |
| Anchor qualification | 1 forward `git apply --check --cached` **per collected candidate**, memoised per `(tree, patch)` |
| Normalized identity (D18) | 1 `git diff --unified=0 …` **per qualifying candidate, only when ≥2 qualify** |
| Anchor-C ladder | 1 `apply --check --reverse --cached`, plus 1 `-C0 --verbose` on failure, **per `(tree, patch)`**, memoised |
| Per-member anchor resolution (D15) | the collect/qualify/compare loop above, **per landed closure member**, sharing the one enumeration and the tree/apply memo |
| `base_commit_reachable` and provenance reachability | 1 `git merge-base --is-ancestor` per checked commit, memoised |

New code is one generic reader in `internal/gitutil/` (candidate
`trailers.go`) returning raw **and** parsed records, plus a temp-index helper;
**policy** stays in `internal/workflow/verify.go` per D7. Everything else
reuses shipped primitives. **No new store field, no new artifact, no schema
migration, no new dependency, no new check ID.**

**Git floor is 2.36, checked before anything else.** The landed contract's
**effective** floor is set by its strictest mandatory capability:

| Capability | Introduced | Used by |
|---|---|---|
| `%(trailers:key=…,valueonly)` | 2.22 | D10 enumeration |
| `…,separator=` | 2.25 | D10 enumeration |
| `git rev-parse --show-object-format` | 2.29 | D10 / D19 derived commit-id length |
| **`GIT_NO_LAZY_FETCH`** | **2.36** | **D11 / D16 offline discipline — mandatory on every object command** |

The sub-capability rows are recorded as **historical component facts only**.
Because `GIT_NO_LAZY_FETCH` is *mandatory* (D11), a git that lacks it cannot
run this contract offline at all, so **the overall preflight requires ≥ 2.36**
and it runs **before any evidence, object, `log`, `read-tree`, `apply` or
`diff` command**. Below the floor the run reports evidence `unavailable` with
R10 and issues **no** object or log command and **no** network call — only the
`git --version` probe itself. Verified on git 2.55.0; the below-floor gate is
proven with a `PATH` shim that reports `git version 2.30.2`, after which the
only subcommand recorded is `--version`.

### D18. Normalized change identity

rev-2 compared raw `git diff --unified=3` bytes, which rejects healthy
cherry-pick and merge-back histories. E36 measured that the same logical change
on two branches with different preimage blobs produces **different** raw `-U3`
bytes, **different** `-U0` bytes, and **identical** bytes once `^index ` lines
are removed from the `-U0` form.

**The normalized identity of a candidate `C` over path set `P` is:**

```
GIT_NO_LAZY_FETCH=1 git diff --no-color --no-ext-diff --no-textconv \
    --binary --no-renames --unified=0 C^ C -- <P...>
```

post-processed by **exactly two** rules, then hashed with SHA-256:

1. **drop** every line beginning `index `;
2. **rewrite** every hunk header — any line matching
   `^@@ -<range> \+<range> @@.*$` — to the bare token `@@`, discarding both
   line ranges **and** the optional function-context suffix.

Nothing else is altered: the hunk **bodies** (`-`/`+`/` ` lines) keep their
content and order, and `diff --git`, `old mode` / `new mode`,
`new file mode` / `deleted file mode`, `--- ` / `+++ ` and
`GIT binary patch` all survive verbatim.

- `P` is the **canonical patch's declared path set** from
  `gitutil.FilesInPatchStrict` (`internal/gitutil/patch_paths_strict.go:253`),
  sorted byte-wise. **If `P` is empty, candidates are not comparable ⇒
  `ambiguous`.** The path set is never broadened to "all paths".
- `--unified=0` removes context lines, the second source of divergence between
  equivalent changes on different branches.
- `^index ` encodes blob ids of the surrounding file version rather than the
  change itself; E36 measured the divergence
  (`index 58b8997..c3badef 100644` vs `index ed24a75..1b142d1 100644`).
- **Hunk ranges and the function suffix are position metadata, not payload.**
  E45 measured that rev-3's rule rejects a healthy cherry-pick applied after
  five unrelated lines were prepended, purely because the headers read
  `@@ -10 +10 @@ l9` versus `@@ -15 +15 @@ l9`. Rewriting both to `@@` makes
  the identities equal. The function suffix is normalized in the same rule
  because it is derived from surrounding content and varies for the same
  reason.
- **Mode, binary and rename semantics are preserved** — E37 measured that the
  normalized form still carries `old mode`/`new mode`, `GIT binary patch`, and
  the `new file mode` / `deleted file mode` pair `--no-renames` produces for a
  rename. E46 re-measured this after adding the header rewrite: a mode-only
  change normalizes to `diff --git` + `old mode` + `new mode` with no `@@` at
  all, and stays distinct from any payload change.
- **The trade, stated precisely (E46).** Discarding positions means two changes
  whose `-`/`+` **bodies are byte-identical** compare equal even when they sit
  at different line numbers. Measured: in a file containing duplicate lines,
  changing the first `DUP` and changing the second `DUP` both normalize to
  `@@ / -DUP / +DUP CHANGED`. The collision therefore requires **duplicate line
  content at the changed lines**; whenever the removed or added text differs at
  all, the bodies differ and the identities stay distinct — measured for a
  distinct payload, a mode-only change and a different path, all of which
  remained distinct. This trade is accepted because the alternative (rev-3's
  position-sensitive rule) rejects healthy cherry-picks, and because a
  collision here yields `duplicate-equivalent` between two commits that
  genuinely introduce the same text, which is the intended equivalence class.

**Applicability.** The identity is used for (a) duplicate-equivalence of
**attestation** candidates and (b) equivalence of **qualifying replay-anchor**
candidates (D14 step 3). Candidates with 0 or ≥2 parents have no `<C>^` and
are never compared — they are already excluded by D14 step 1(c) and D16.
Cherry-picked and merged-back landings compare **equal** under this identity
and are therefore `duplicate-equivalent` rather than `ambiguous`.

### D19. `land` producer contract: validate the base commit **by invocation mode**

`land` reads `status.Apply.BaseCommit` and interpolates it into the trailer
block with **no validation** (`internal/cli/land.go:394`, `:397-400`). On a
legacy or corrupt status the emitted trailer can be `Tpatch-Base-Commit: `
with an empty value, which the D10 grammar classifies as `malformed` at every
future read. A producer must not create evidence the reader is required to
reject.

**rev-3 promised "refuses before the commit, mutating nothing". That promise
is unkeepable in the embedded-`record` mode** and is withdrawn. `land` runs
`record` as its first step; `record` writes `post-apply.patch`, may regenerate
`apply-recipe.json`, and may write a numbered audit snapshot — all **before**
`land` has any post-record value to inspect. Any refusal after that point
leaves those artifacts on disk. The contract is therefore split by invocation
mode and states the boundary honestly.

**Validity predicate** (shared by both modes). `status.Apply.BaseCommit` is
valid iff it is:

1. **non-empty**;
2. **well-formed** — `N` lowercase hex where `N` derives from
   `git rev-parse --show-object-format` (E41: 40 for `sha1`, 64 for
   `sha256`); the length is derived, never hardcoded;
3. **resolvable** — `GIT_NO_LAZY_FETCH=1 git rev-parse --verify <base>^{commit}`
   succeeds;
4. **reachable from `HEAD`** — `gitutil.IsAncestor`
   (`internal/gitutil/gitutil.go:828`), **unless** the repository is shallow or
   a partial clone (D16), in which case unreachability is a one-line `warn` and
   the landing proceeds, because the object may simply be outside local
   history.

**Mode A — `--no-record` (`internal/cli/land.go:66`, `:98`).** `land` does not
run `record`, so `Apply.BaseCommit` is whatever is already on disk.

**Ordering, stated against the shipped sequence.** `runLand`
(`internal/cli/land.go:76`) already performs, in order: store open and
`LoadFeatureStatus` (`:85`), the `unapplied` refusal (`:89`), the dry-run
branch (`:93`), `landPreflight` (`:110`), `CheckDependencyGate` (`:116`), then
**`recoverLand`** (`:127`, defined at `internal/cli/land_journal.go:437-445`).
Base-Commit validation is inserted **immediately after `recoverLand` returns**
and **before** the pre-record gate, the metadata snapshot and every
`land`-owned mutation.

**`recoverLand` stays first, and it is the one documented exception.** GH #7
made crash recovery mandatory before anything else mutates record, status or
the index, and recovery is not read-only: with a pending journal it may
publish a retained index, compare-and-swap the branch back to the pre-land
commit, and restore the `status.json` preimage. That work belongs to a
**prior, interrupted transaction**, not to this invocation.

The guarantee is therefore stated in two cases:

| Case at command entry | Guarantee on an invalid `BaseCommit` |
|---|---|
| **no pending journal** — `recoverLand` is a no-op | `land` refuses with **R23** having made **no mutation whatsoever**: no commit, no index change, no `status.json` write, no artifact. |
| **pending journal** — `recoverLand` completes or refuses the prior transaction | `recoverLand` **may have mutated** the index, the branch ref and/or `status.json` **while finishing that earlier land**. `land` then refuses with **R23** having made **no NEW mutation for this invocation**. The refusal names the completed recovery so the operator is not surprised. |

**Absolute command-entry immutability is not promised**, because the shipped
recovery-first ordering forbids it and that ordering is correct.

**Mode B — embedded `record` (the default).** The field is `record`'s output,
so it cannot be validated before `record` runs.

- **`record` owns the producer invariant.** `record` must guarantee that it
  writes a valid `Apply.BaseCommit` — non-empty, object-format-correct,
  resolvable and reachable — **before its own first mutation**. `record`
  already resolves the base up front, including the `--auto` path
  (`PRD-record-auto-base` §3.3), so this is a precondition it is positioned to
  enforce; it is stated here as an obligation on `record`, not a new
  computation in `land`.
- **`land` re-validates immediately after `record` returns**, on the reloaded
  status, **before** any `land`-owned mutation — before the `landed at` note
  (`internal/cli/land.go:357`), before staging, before the commit.
- **If validation fails at that point**, `land` refuses with R23. **`record`'s
  artifacts persist**, because `record` completed as an **independent, already
  finished transaction**. The refusal message says so explicitly, and the
  acceptance rows say so too — no row may claim "mutating nothing" for this
  mode. `land` itself has still mutated nothing: no commit, no index change,
  no `landed at` note.

**Why the split rather than pre-validating in Mode B.** Pre-validating the
*old* value would be meaningless: `record` is expected to replace it, so a
stale-but-invalid value would refuse a landing that would have succeeded, and
a stale-but-valid value would not protect against `record` producing an
invalid one. Validating the *actual produced value* is the only check with
content.

**Unchanged.** `status.apply.base_commit` is still **never written by `land`**
— ADR-019 and PRD `land` §3.6 stand; this is a precondition, not a writeback,
and the field remains owned by `record` / auto-base resolution (ADR-016).
Every pre-existing `land` refusal, its ordering, its message and the entire
**successful** path are byte-unchanged.

**Object-format assumption, stated honestly.** tpatch declares no
hash-algorithm assumption elsewhere. Both the producer here and the reader in
D10 derive the accepted length from `git rev-parse --show-object-format` and
fail closed if it cannot be read.

---

## Amendment 1 rev-7 — the `replace-in-file` predicate (diagnostic use only)

Under D13 this predicate does not decide presence; it localises diagnostics.
For content `c`, search `S`, replacement `R`:

- `S == ""` ⇒ **unsupported** (E16: `strings.Replace(x,"",R,1)` inserts at the
  start).
- `R == ""` ⇒ **undecidable** — every `c` admits the preimage `S+c`; the
  judgement defers to patch authority.
- otherwise ⇒ true iff there exists an index `i` at which `R` occurs in `c`
  such that `pre := c[:i] + S + c[i+len(R):]` satisfies
  `strings.Replace(pre, S, R, 1) == c`. Every occurrence of `R` is tried.

| Predicate | decided | undecidable | false reds | false greens |
|---|---|---|---|---|
| rev-0 round trip | 56 784 | 0 | **204** | **15 933** |
| rev-1…rev-3 existential inverse | 52 416 | 4 368 | **0** | **0** |

`write-file` ⇒ bytes equal `op.Content`; `append-file` ⇒ content ends with
`op.Content`, and an **empty** `op.Content` is **undecidable**;
`ensure-directory` ⇒ the path exists and is a directory; unknown type ⇒
unsupported (`internal/workflow/verify.go:1316`). None of these certifies.

## Amendment 1 rev-7 — alternatives considered and rejected

1. **HEAD-only post-state predicates (rev-0)** — false-reds every landed
   `write-file` feature after any later edit; V7 aliases V8.
2. **Replay at `status.apply.base_commit`** — operator-mutable, possibly
   unreachable, unrelated to the attestation.
3. **Anchor C against the working tree (rev-1)** — E23.
4. **Anchor C against the real index** — same coupling class.
5. **Byte-exact tree comparison as the materialization test** — later edits are
   routine and harmless.
6. **Default-context reverse-check as the sole test** — 60 false reds / 184.
7. **`-C0` with `(0/0)` ignored (rev-1)** — E29: 2 false greens / 69.
8. **A stronger hunk-local corroboration instead of blocking on `(0/0)`** —
   not adopted: unprovable on the measured corpus.
9. **Recipe replay as the parent presence test (rev-1)** — circular.
10. **Whole-file byte equality as the parent presence test** — the rev-0
    defect.
11. **Skip-and-pass when anchor H is unavailable (rev-1)** — an unverifiable
    historical half is an unverified feature.
12. **Using the attestation candidate as the replay anchor (rev-1)** — E30.
13. **Stop-at-first anchor selection (rev-2)** — contradicts the ambiguity
    rule it was paired with; D14 collects every qualifier first.
14. **Qualifying anchor candidates with the reverse ladder (rev-2)** — E34:
    the reverse check answers the inverted question and fails at exactly the
    tree a baseline needs to be.
15. **Broadening the anchor search to non-trailer commits** — invents
    attribution.
16. **Raw `--unified=3` duplicate comparison (rev-2)** — E36: rejects healthy
    cherry-pick and merge-back histories.
17. **Keeping `-U0` hunk ranges and the function suffix (rev-3)** — rejected on
    E45: it rejects a healthy cherry-pick applied at a different offset. Both
    are position metadata and are normalized to a bare `@@`; the resulting
    trade is measured and bounded in E46.
18. **Comparing `git patch-id` instead of normalized bytes** — `patch-id`
    ignores mode and binary changes that E37 shows the normalized form
    preserves, and ADR-018 exists because identical patch ids occur across
    features.
19. **Evaluating parent V10 at the target's anchor (rev-2)** — that tree may
    already contain the parent's postimage.
20. **Later-touch from current byte differences (rev-1)** — fires on the
    operator's own edits.
21. **Blocking on later-touch** — contradicts ADR-029 D6.
22. **Reading the live working tree when provenance is unavailable** — E11/E12:
    the measured false block. `recipe-provenance-unavailable` instead.
23. **Declaring Q15 unfixable "because no anchor exists" (rev-2)** — E42
    disproves it; `RecipeProvenance.BaseCommit` is the anchor.
24. **Adding a new provenance artifact for forward-mode V10** — unnecessary;
    the sidecar already exists and is hash-bound.
25. **Trusting provenance without the `recipe_sha256` binding** — accepted only
    for pre-v0.5.2 sidecars that omit the field, and reported as
    `provenance_hash_bound: false` rather than silently.
26. **A closure-only snapshot (rev-2)** — `loadLaterFeatureTouches` reads every
    feature, so ADR-029 ordering would be non-deterministic.
27. **Treating a shallow boundary as a root (rev-2)** — E38: identical `%P`
    and identical `read-tree` error text; the remediation would be wrong.
28. **Hardcoding a 40-hex commit-id rule** — E41: a SHA-256 repository uses 64.
29. **Letting `land` emit an unvalidated `Tpatch-Base-Commit`** — produces
    evidence the reader must reject; D19 refuses before the commit.
30. **Having `land` repair `status.apply.base_commit` itself** — that would
    make `land` a writer of a field ADR-016 owns; it refuses and points at
    `record` instead.
31. **Classifying a prose-quoted trailer line as `none`** — E31.
32. **`rev-list` for enumeration** — cannot emit `%B`.
33. **`--first-parent` reachability scoping** — misses merged side branches.
34. **Approximating a merge landing's anchor as `^1`** — E33.
35. **Reader errors degrading to `none`** — converts an unknown into a claim.
36. **A twelfth check for current materialization** — breaks the frozen
    vocabulary.
37. **A new `verified-landed` freshness label** — the four D5 labels stay
    mutually exclusive.
38. **Emitting `freshness_label` in the verify JSON report** — the shipped
    `VerifyReport` (`internal/workflow/verify.go:139-166`) has no such field.
39. **Leaving the `active`-parent inconsistency unresolved** — D13 decides it.
40. **Ownership-only evidence** — a swapped `post-apply.patch` whose content
    happens to be present would get a green verdict.
41. **An escape flag (`--assume-landed`)** — D3 already refuses hand-written
    claims.
42. **`C^{tree}^` as the parent-tree revision (rev-3)** — invalid syntax,
    measured (E43).
43. **Default-context (`-C3`) forward qualification (rev-3)** — E44: rejects a
    parent carrying an unrelated edit two lines from the hunk, which can
    dead-end the operator's own re-record-and-re-land remediation.
44. **`-C0` forward qualification** — over-permissive; it discards context
    entirely, the same setting D12 refuses in the reverse direction.
45. **`ListFeatures()` for the inventory (rev-3)** — silently skips unreadable
    features (`internal/store/store.go:226`), so an unreadable feature cannot
    be represented at all. `ListFeatureEntries` exists for exactly this.
46. **Allowing lazy promisor fetches during verification** — E47: git attempts
    the network and fails with a remote error instead of a classifiable local
    one, contradicting §1.3's offline-by-construction rule.
47. **Promising "mutating nothing" for the embedded-`record` land refusal
    (rev-3)** — `record` completes as an independent transaction before `land`
    can inspect its output, so the promise is unkeepable. D19 splits the
    contract by invocation mode instead.

## Amendment 1 rev-7 — consequences

**Positive**

- Anchor selection is exhaustive and self-consistent; the rev-2 stop-at-first
  contradiction is gone.
- Qualification asks the forward question, which is the one a baseline must
  answer.
- Healthy cherry-pick and merge-back histories are accepted (E36) without
  weakening mode/binary/rename semantics (E37).
- Every landed member is judged against its own baseline, so a parent's V10 is
  never compared against a tree that already contains its postimage.
- **Q15 is resolved**: forward-mode V10 gains a real, hash-bound anchor.
- Shallow and partial clones get correct classification and correct
  remediation instead of a wrong "re-land" instruction.
- `land` can no longer create evidence the reader must reject.
- The snapshot covers everything ADR-029 ordering reads.

**Negative / accepted trade-offs**

- The inventory is O(features) rather than O(closure); bounded and cached, but
  larger.
- Anchor collection costs one forward `apply --check` per candidate rather than
  stopping early; memoised per tree.
- `recipe-provenance-unavailable` is a new terminal failure for recipes that
  carry `preimage_hash` without a usable sidecar.
- The `(0/0)` block still costs 26 false reds per 151 present trees.
- `historical-anchor-unavailable` remains terminal.
- `active` closure widening changes verdicts for non-landed features.
- `land` gains a refusal path on legacy/corrupt status.

**Neutral**

- No lifecycle change, no new `FeatureState`, no new label, no new check ID,
  no new artifact, no store schema change, and `land`'s **successful** path is
  byte-unchanged.

## Amendment 1 rev-7 — references (anchors validated 2026-08-12 at `785d261`)

**Contract documents**

- `docs/prds/PRD-verify-freshness.md` §3.6, §4.3.6–4.3.9, §7.1
- `docs/prds/PRD-tpatch-land.md` §3.8, §6.2
- `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md`
- `docs/adrs/ADR-016-record-auto-base-resolution.md` — owner of
  `status.apply.base_commit`; D19 refuses rather than writes
- `docs/adrs/ADR-018-record-collision-detection-signature.md`
- `docs/adrs/ADR-029-write-file-recipe-safety.md` D1/D2/D4/D5/D6/D7/D8
- `docs/adrs/ADR-028-supersession-edge-model.md` D6
- `docs/adrs/ADR-031-rejected-feature-state-data-model.md`,
  `docs/adrs/ADR-032-feature-unapply-state-boundary.md`
- CHANGELOG v0.11.3 — the GH #2 fix D9 must not regress
- `docs/reconcile.md`

**Source anchors**

- `internal/workflow/verify.go:49-71` — eleven frozen check IDs (D8)
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
- `internal/workflow/verify.go:1275-1282` — 1-based op index convention
- `internal/workflow/verify.go:1284-1318` — `replayOpInShadow` op kinds
- `internal/workflow/verify_all.go:89-97` — `isPostApplyState` admits `active`
- `internal/workflow/writefile_safety.go:108-112` — V10 reads the live tree
- `internal/workflow/writefile_safety.go:176` — `isLowercaseHex`
- `internal/workflow/writefile_safety.go:380-388` — `laterTouchIndex` (D15)
- `internal/workflow/writefile_safety.go:409-442` — `loadLaterFeatureTouches`,
  `RequestedAt` ordering, `ListFeatures()` over the whole repo (D15/D17)
- `internal/workflow/writefile_safety.go:449-481` — `collectFeatureTouchedPaths`
- `internal/workflow/writefile_safety.go:489-498` — `checkLaterTouch`
- `internal/workflow/writefile_safety.go:571` — `DetectRecordLaterTouchWarnings`
- `internal/workflow/recipe_autogen.go:114-118` — autogen omits `preimage_hash`
- `internal/workflow/implement.go:30-34` — `RecipeProvenance` struct (D15/Q15)
- `internal/workflow/implement.go:220-238` — provenance writer (D15/Q15)
- `internal/workflow/implement.go:99` — `PreimageHash *string` pointer
  semantics
- `internal/cli/cobra.go:1285-1315` — `warnRecipeStale`, the shipped
  provenance consumer incl. the nil-`recipe_sha256` tolerance (D15)
- `internal/workflow/dependency_gate.go:79-81` — the gate accepts `active`
- `internal/store/patch_generations.go:52` — `TouchedPaths` (D15/D17)
- `internal/cli/land.go:66`, `:98` — the `--no-record` flag (D19)
- `internal/cli/land.go:392`, `:394`, `:397-400` — trailer production, the
  unvalidated `BaseCommit` read D19 fixes
- `internal/cli/land.go:1034-1043` — `readRecipeSHA`, whitespace → `none`
- `internal/cli/land_journal.go:31`, `:60` — gitignored `.tpatch/local/`
- `internal/gitutil/ignore.go:50-51` — `.tpatch/local/` ignore handling
- `internal/store/types.go:290-296` — `VerifyRecord` (unchanged)
- `internal/store/types.go:347` — `ApplySummary.BaseCommit`
- `internal/store/dag.go:107` — `TopologicalOrder`
- `internal/gitutil/gitutil.go:14` — `HeadCommit`
- `internal/gitutil/gitutil.go:828` — `IsAncestor` (D15/D19 reachability)
- `internal/gitutil/shadow.go:56` — `CreateShadow` accepts any commit-ish
- `internal/gitutil/patch_paths_strict.go:253` — `FilesInPatchStrict` (D18)
- `internal/workflow/verify_closure_replay_test.go:275` —
  `TestRunVerify_EquivalentRecipeAndPatchBothPass`, the GH #2 regression
