# ADR-013 — Verify Freshness Overlay

**Status**: Accepted (M15 Wave 3 design — Git-like freshness redesign; PRD: `docs/prds/PRD-verify-freshness.md`) · **Amendment 1 rev-7 (final) ACCEPTED 2026-08-12 (v0.15.1 Wave B / GH #8 — landed-evidence semantics; D8–D19 below)** · **Amendment 2 ACCEPTED 2026-11-06 (GH #25 C1–C2 — read-only overlap assessment; D20–D27 below)** · **Amendment 3 ACCEPTED 2026-11-06 (GH #25 A1 — the current anchor explained by one recorded child; D28–D35 below; shipped in v0.20.0)** · **Amendment 4 ACCEPTED 2026-11-07 (rev-4; GH #26 attestation-preferred anchor selection and the R14 split; D40–D45 below; design review rev-4 APPROVED; slice N1 (D40, D41, D44) implemented 2026-11-11 and approved on code review; its D44 measurement found differences rev-4's pins did not name, so rev-5 (2026-11-11, D44 only) answers them, design review rev-5 APPROVED; N1 shipped in v0.21.0; slice N2 (D42, D43) scheduled and implemented 2026-11-11, approved on code review, not yet released)**
**Date**: 2026-04-27 (original) · 2026-08-12 (Amendment 1) · 2026-11-06 (Amendment 2) · 2026-11-06 (Amendment 3) · 2026-11-06 (Amendment 4 draft) · 2026-11-07 (Amendment 4 rev-2 to rev-4, accepted) · 2026-11-11 (Amendment 4 rev-5, D44 only)
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

At the pre-amendment baseline, `workflow.CheckDependencyGate` accepted hard
parents in `{applied, upstream_merged}`. The freshness overlay does not extend
the satisfaction set or become a gate input. Wave C's separately disclosed
`active`-totality change now admits `active` at
`internal/workflow/dependency_gate.go:97-98`.

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
6. Apply the target's recipe (V7); then, **per GH #2 / v0.11.3**, reset the shadow to the recorded closure-replayed baseline tree (`resetShadowToTree`, `internal/workflow/verify.go:952-960`) before `git apply --check`ing the target's `post-apply.patch` (V8), so recipe and patch are validated **independently against the same baseline** rather than double-applied. *(Amended: the original D7 text said "against the same shadow" — that predates the GH #2 reset and is superseded.)*
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
- `internal/workflow/dependency_gate.go:42` — `CheckDependencyGate` entry
  (D2 anchor; freshness remains absent from the gate)
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
(`internal/workflow/verify_anchored.go:439-464`), so after `land` the baseline
already contains the feature.

Measured with the real CLI at `13a885c`; every run reported `checks=11` — the
shipped set is **V0–V10** (`internal/workflow/verify.go:49-70`), V10 appended
last (`internal/workflow/verify.go:313`):

| Target recipe op kind | pre-land V7/V8 | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file`      | PASS / PASS | **PASS — false green** (`internal/workflow/verify.go:1003-1007`) | **FAIL — false red** |
| `replace-in-file` | PASS / PASS | **FAIL — false red** (`internal/workflow/verify.go:1008-1018`) | SKIP |
| `append-file`     | PASS / PASS | **PASS — false green, shadow double-appended** (`internal/workflow/verify.go:1019-1025`) | **FAIL — false red** |

The defect is not V8-only, and the same double-apply hazard applies to a landed
hard parent (`internal/workflow/verify_anchored.go:621-756`).

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

`internal/workflow/verify.go:49-70` defines eleven check IDs; V10 is
`write_file_preimage_fresh`, appended last
(`internal/workflow/verify.go:313`). Every report shape,
golden example and acceptance row describes **eleven** rows in that order. No
check is added or removed: the current-materialization assertion folds into
**V8**, whose subject is the artifact the landing trailer attests.

### D9. Baseline model: dual-anchor landed verification

- **Anchor H (historical)** — a shadow rooted at the **replay anchor's** single
  parent tree (D14). Closure arbitration (D13) runs, then the existing
  machinery: V7 replays the recipe (`internal/workflow/verify.go:988-995`), the
  shadow is reset to `closureBaselineTree`, V8 runs `git apply --check` forward
  (`internal/workflow/verify_anchored.go:476`,
  `internal/workflow/verify_anchored.go:547-553`,
  `internal/workflow/verify_anchored.go:556-580`), V10 evaluates preimages
  (D15).
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
between them (`internal/workflow/verify_anchored.go:476`,
`internal/workflow/verify_anchored.go:547-553`). Normative: *any
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
| `upstream_merged` | **skip** (unchanged, `internal/workflow/verify_anchored.go:653-655`) |
| superseded by an active superseder | **skip** (unchanged, `internal/workflow/verify_anchored.go:389-391`) |
| evidence `exact`/`duplicate-equivalent`, patch **present-nonempty**, ladder ⇒ clean or context-drift | **skip** |
| evidence `exact`/`duplicate-equivalent`, patch present-nonempty, ladder ⇒ block | **fail-fast** `parent-landing-drift` |
| patch **absent** or **present-empty**, whatever the recipe shape | **fail-fast** `landed-artifacts-absent` — reached **before** any digest comparison and therefore **before** any `exact`/`stale` classification (D10). A slug-bearing candidate on such a member never becomes an attestation. |
| evidence `none`, patch present-nonempty, ladder ⇒ clean or context-drift | **skip** with a mandatory `warn` `unattributed-materialized` advisory. Verify claims **no ownership**. |
| evidence `none`, patch present-nonempty, ladder ⇒ block | **replay** (unchanged, `internal/workflow/verify_anchored.go:721-753`) |
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

**`active` is total.** It is treated **identically to `applied`** everywhere
in the closure. Before Wave C, the switch handled only `upstream_merged` and
`applied`, so an `active` hard parent reached `default:` and failed fast.
Wave C widened the switch (`internal/workflow/verify_anchored.go:653-666`) to
match `CheckDependencyGate`
(`internal/workflow/dependency_gate.go:97-98`),
`postApplyVerifyStates` admits `active` (`internal/workflow/verify.go:125-135`)
and `isPostApplyState` (`internal/workflow/verify_all.go:90-94`). The widening
was the smallest change that made all four sites agree and is a **deliberate
behaviour change for non-landed features**.

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
   **Amended by Amendment 4 D40**: when any qualifier is an exact attestation
   (D10's trailer test), select only among those and take the **newest**;
   oldest-first applies only when no qualifier is an exact attestation.

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
| any member whose recipe carries no `preimage_hash` on a given op | ADR-029 D4 legacy path for that op: pass, no re-warn (`internal/workflow/verify_anchored.go:847-850`) |

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
| present and **not** matching | **FAIL**, block severity; downgraded to `warn` when superseded (ADR-029 D7, `internal/workflow/verify_anchored.go:796-800`, `internal/workflow/verify_anchored.go:889-893`) |
| `preimage_hash` malformed per ADR-029 D1 | **FAIL**, block — the preimage contract itself is invalid |
| provenance required but absent / ill-formed / unreachable | **FAIL** `recipe-provenance-unavailable` |
| V2 skipped or failed | **skip**, unchanged reason (`internal/workflow/verify_anchored.go:791-792`) |
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
unsupported (`internal/workflow/verify.go:1030`). None of these certifies.

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
    Narrowed by Amendment 4 D40: an exact attestation that passes the same
    `-C1` forward qualification is preferred; an unqualified attestation is
    still never used.
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
    `VerifyReport` (`internal/workflow/verify.go:137-182`) has no such field.
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

## Amendment 1 rev-7 — references (verify-family anchors revalidated 2026-08-13 at `15560af`)

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

- `internal/workflow/verify.go:49-70` — eleven frozen check IDs (D8)
- `internal/workflow/verify.go:81` — `verifySchemaVersion`
- `internal/workflow/verify.go:125-135` — `postApplyVerifyStates` (D13)
- `internal/workflow/verify.go:137-182` — `VerifyReport` (no `freshness_label`)
- `internal/workflow/verify.go:313` — V10 appended last
- `internal/workflow/verify.go:323-324` — recipe/patch digests
- `internal/workflow/verify.go:347-357` — `--no-write` persistence gate
- `internal/workflow/verify.go:566-571` — `sha256Hex`
- `internal/workflow/verify_anchored.go:791-792` — V10 skip when V2 skipped (D15)
- `internal/workflow/verify_anchored.go:796-800`,
  `internal/workflow/verify_anchored.go:889-893` — supersession severity downgrade
- `internal/workflow/verify_anchored.go:847-850` — ADR-029 D4 legacy path (D15)
- `internal/workflow/verify_anchored.go:342-590` — `runAnchoredClosure`
- `internal/workflow/verify_anchored.go:389-391` — supersession exclusion (D13)
- `internal/workflow/verify_anchored.go:404` — `TopologicalOrder` call
- `internal/workflow/verify_anchored.go:439-464` — `HeadCommit` / `CreateShadow`
- `internal/workflow/verify_anchored.go:464` — deferred `PruneShadow`
- `internal/workflow/verify_anchored.go:621-756` — the closure replay loop
- `internal/workflow/verify_anchored.go:653-666` — the state switch `active` joins
- `internal/workflow/verify_anchored.go:476`,
  `internal/workflow/verify_anchored.go:547-553` — GH #2 snapshot + reset
- `internal/workflow/verify_anchored.go:556-580` — `git apply --check` (anchor H)
- `internal/workflow/verify_anchored.go:576` — the `run tpatch reconcile` string
- `internal/workflow/verify.go:988-995` — 1-based op index convention
- `internal/workflow/verify.go:997-1031` — `replayOpInShadow` op kinds
- `internal/workflow/verify_all.go:90-94` — `isPostApplyState` admits `active`
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
- `internal/workflow/dependency_gate.go:97-98` — the gate accepts `active`
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

---

# Amendment 2 (2026-11-06) — Read-only overlap assessment — GH #25 slices C1–C2

**Status of this amendment**: ACCEPTED for implementation by the operator's
GH #25 answers (2026-11-06, PRD
`docs/prds/PRD-overlap-verification-consolidation.md` §8 Q1–Q12). Adds
**D20–D27**. **No prior decision is reversed.** D12 (the `(0/0)` block), D13
(arbitration), D14 (anchor collection), D17 (one inventory, one
enumeration) and the persisted record (D3, `internal/store/types.go`
`VerifyRecord`) are unchanged. The D12 advisory vocabulary stays closed:
the assessment is a separate report field, not an advisory.

**Issue**: <https://github.com/tesseracode/tesserapatch/issues/25>
**Numbering**: the PRD's draft slice plan called Option A's semantic
amendment "Amendment 2". The operator ordered C before A (Q1), so C takes
Amendment 2 and A1's widening of anchor C, when it is drafted, becomes
Amendment 3.

## A2.0 Context

A parent feature creates a file; a hard-dependent child later rewrites it.
Both land with `exact` evidence. `tpatch verify <parent>` then fails at
`landed-content-absent`, because anchor C (D11/D12) correctly finds the
parent's postimage missing at `HEAD`. Verify cannot say why, and its R1
remediation tells the operator to re-record and re-land the parent, which is
wrong for this case. Amendment 2 adds an explanation that is computed in the
same run and never changes the verdict.

## Decision (Amendment 2, D20–D27)

**Design review.** Rev-1 of this amendment (2026-11-06) was judged UNSOUND by
an independent design review: a HEAD round-trip proved only that the
descendants' hunks are invertible, not that nothing else touched the paths;
the composition reread artifact files instead of the captured bytes; the
nested verify followed the live `HEAD`, pruned the descendant's existing
shadow worktrees and mutated shared evidence; E4 skipped non-overlapping
intermediates; and R26 could name a descendant for a Git failure. Rev-2,
below, closes each of those.

### D20. `overlap_assessment` is an explanation-only report field

`tpatch verify --json` gains an optional top-level object
`overlap_assessment` (omitted when no assessment ran). The report
`schema_version` moves `1.1` → `1.2` (additive). The object has:

| Field | Meaning |
|---|---|
| `class` | One of the four D22 classes. |
| `verdict_effect` | Always the constant `"none"`, so a reader cannot mistake the field for authority. |
| `detail` | A deterministic one-line explanation. |
| `failed_step` | Present only when a procedure step decided the class: `target-paths`, `candidates`, `eligibility`, `order`, `baseline`, `replay` or `compare`. |
| `descendants` | The candidates considered: `slug`, `attestation_commit` (when known), `evidence` (its landing-evidence state), and `failed_rule` + `detail` when that candidate is ineligible. |

The assessment **never** changes `verdict`, `exit_code`, `failed_at`,
`parent_slug`, any check's `passed`/`skipped`/`severity`/`anchor_results`,
the advisories, or the persisted `VerifyRecord`. It is **never written to
`status.json`** and no other command (apply, land, reconcile, status,
amend) reads it. The only check-row text it may change is the V8
remediation (D26). `verify --all` reports it inside each entry's embedded
report; aggregate counts and the aggregate exit code are unchanged (Q12).

### D21. Trigger: only a blocked anchor-C ladder on a landed target

The assessment runs only inside `foldAnchorC`
(`internal/workflow/verify_anchored.go`), after the target's own anchor-C
ladder has run, and never inside a nested E6 verify:

- `ladder.Blocked` → the full D22 procedure;
- `ladder.ContextDrift` → `class: context-only-drift` with no candidates
  (the existing R3 advisory is unchanged);
- every other outcome — materialized clean, a probe that could not run, a
  terminal evidence state, absent artifacts, an unavailable historical
  anchor, a static block failure, forward mode — emits **no** field.

The field is dropped (omitted) when the run's inventory re-statement
reports instability (D17) or when `HEAD` no longer resolves to the run's
captured commit after the assessment, so an assessment is never reported
against a state the run did not hold. Dropping a blocked ladder's
assessment restores the R1/R2 remediation it replaced (D26). A
`context-only-drift` assessment replaced no remediation, so dropping it
leaves V8's remediation as the run computed it — for example a
historical-anchor failure's text (code review rev-1 F4).

### D22. The closed class vocabulary and the procedure

Four classes: `recorded-descendant-evolution`, `context-only-drift`,
`unexplained-content-loss`, `invalid-or-missing-evidence`. For a blocked
ladder the class is decided by these steps, in order; the first step that
decides wins. `H` is the run's captured `HEAD` commit (D17) and `T` is the
target's attestation commit.

1. **Target paths** (`target-paths`). The target's canonical-patch paths
   are read with the strict parser
   (`gitutil.PathsAffectedByPatchStrict`) from the run's captured
   inventory bytes. A parse failure or an empty set →
   `unexplained-content-loss`. This call and step 2's are direct consumers
   of the GH #15 strict authority, registered by name in its parser
   call-site inventory (`TestRGAS0PatchParserCallsiteInventory`); they
   parse nothing themselves.
2. **Candidates** (`candidates`). A candidate is a feature, other than the
   target, that reaches the target through a chain of `hard` `depends_on`
   edges in the run inventory (Q4) **and** whose canonical patch touches at
   least one target path (E2/E3). A descendant whose patch is absent or
   empty changed nothing and is not a candidate; one whose patch cannot be
   read or parsed is a candidate that fails E2. No candidate →
   `unexplained-content-loss`. Edge-less features, soft edges, `supersedes`
   edges and later-touch advisories never make a candidate.
3. **Eligibility** (`eligibility`). Every candidate must pass, in this
   order:
   - **E1** — landing evidence `exact` with artifacts present.
     `duplicate-equivalent` does not qualify (Q11).
   - **E2** — its canonical patch parses (paths were needed in step 2).
   - **E4** — every hard edge on every chain from the candidate to the
     target is bound (Q3). For each chain member `X` (the candidate and
     every intermediate, overlapping or not): `X`'s captured
     `patch-generations.json` parses and validates, has at least one
     generation, its latest generation's `patch_sha256` equals the SHA-256
     of `X`'s captured canonical patch, and for each hard parent `Y` of `X`
     on such a chain the latest generation records a `hard` dependency
     whose `parent_patch_sha256` equals the SHA-256 of `Y`'s captured
     canonical patch. A missing manifest is a legacy feature and fails E4
     with that reason.
   - **E5** — its attestation commit differs from `T`, descends from `T`,
     and is an ancestor of `H`. If Git cannot answer either ancestry query,
     the step could not run: the class is `invalid-or-missing-evidence`
     with `failed_step: eligibility`, no descendant carries
     `failed_rule: E5`, and R26's execution form applies (D26; code review
     rev-1 F3).
   - **E6** — its own verify, run in the same run context under D23, has
     verdict `passed`. A persisted or earlier result is never used.
   Any ineligible candidate → `invalid-or-missing-evidence`, with every
   candidate listed in slug order and each failure named.
4. **Order** (`order`). The candidates are sorted by ancestry of their
   attestation commits. If any pair is not ordered by ancestry (parallel
   landings joined by a merge, or two candidates sharing one commit) →
   `unexplained-content-loss`; merge topologies are refused in v1 (Q2).
5. **Baseline** (`baseline`). A temp index (D11) is seeded from `T`, and
   the target's captured patch must pass the plain full-context reverse
   check there: the recorded postimage was present at its own landing. If
   it is not → `unexplained-content-loss` (no descendant can explain a
   postimage that never landed, so R1/R2 stay the answer).
6. **Replay** (`replay`). In landing order, each candidate's captured
   patch is applied forward to that index with full context, limited to
   the target paths (`git apply --cached --include=<literal path>`).
7. **Compare** (`compare`). The index must equal `H` on every target
   path. This binds provenance: `H`'s bytes on the target's paths are
   exactly `T`'s bytes evolved by the recorded descendants and nothing
   else, so a non-tpatch edit, a partial revert, or a landing commit that
   carried more than its patch on those paths is never explained.

   A step that answers "does not apply" or "differs" →
   `unexplained-content-loss` naming the step and, where relevant, the
   candidate. A Git command that could not run (an execution failure, a
   missing object) → `invalid-or-missing-evidence` naming the step.
8. Otherwise → `recorded-descendant-evolution`, with the descendants in
   landing order.

No step accepts `-C0`, `(0/0)` or any reduced-context match. No permutation
search is attempted. A candidate patch that renames a target path out of
the target set is not followed, so the compare step fails closed
(`unexplained-content-loss`).

### D23. Same-run nested verify is hermetic and read-only

E6 calls the ordinary single-feature pipeline with `NoWrite`. The nested
call:

- shares the run's immutable captures (D17) — inventory, enumeration,
  preflight and the probe memo tables — so the run's instability
  re-statement covers it;
- resolves every `HEAD` reference to `H`, the run's captured commit, rather
  than the live `HEAD`;
- works on a private deep copy of the landing-evidence memo, discarded
  afterwards, so no evidence state it writes can change how a later
  `verify --all` entry is classified;
- allocates its shadow worktrees under a run-owned name
  (`<slug>.overlap-<run nonce>`), so it never lists, reuses or prunes a
  shadow that belongs to the descendant or to another run;
- runs with the assessment disabled, so it cannot recurse;
- saves, clears and restores the run context's pending arbitration state
  around the call, so neither run sees the other's advisories or member
  baselines;
- is memoized per slug for the rest of the run.

### D24. Git surface

The procedure uses two new methods on the isolated temp index, both
through the offline evidence runner (`GIT_NO_LAZY_FETCH=1`, `LC_ALL=C`):
`git apply --cached --whitespace=nowarn [--include=<path>]... <patch>` and
`git diff-index --cached --quiet --ignore-submodules=none <rev> -- <paths>`,
plus the existing `read-tree` and `apply --check --cached --reverse`.
Gitlinks always compare: a `submodule.<name>.ignore` setting in
`.git/config` or `.gitmodules` cannot hide a changed submodule commit
(code review rev-1 F1). Every patch the
procedure applies or checks is the run's **captured** inventory bytes,
written to a run-owned temporary file in the D11 directory — never the
artifact file — so bytes that change mid-run cannot enter the assessment.
Every commit it names is `T`, `H` or a captured attestation commit, never
the symbolic `HEAD`. The temp index and the temporary patch files are
removed on every exit path. The working tree, the real index and
`.tpatch/` are never written (AC-6).

The same rule holds for the ordinary pipeline's own probes (code review
rev-1 F2): the anchor-C ladder at `HEAD`, the closure members' ladders,
D14 forward qualification and the historical V8 shadow apply check all
stage the run's captured bytes instead of passing the artifact path to
Git, and their memo keys include the bytes' SHA-256. A nested E6 verify
shares those memos with the run, so an artifact rewritten mid-run can
enter no probe and no memo entry that the nested verify or a later
`verify --all` entry reuses. Output is unchanged whenever the artifacts do
not change during the run.

### D25. Determinism

Same repository state → byte-identical `overlap_assessment` (AC-5).
Candidates are sorted by slug, then by landing order on success; `detail`
strings contain slugs, commit SHAs, rule names and repository-relative
paths only — never raw Git stderr, temp-file paths, the run nonce or
wall-clock time.

### D26. Remediation branches on the class (C2, Q8)

For a blocked ladder the V8 remediation that R1/R2 would carry is chosen by
class:

- `recorded-descendant-evolution` → **R25**: names the descendants and their
  attestation commits, states that verify still requires the feature's own
  postimage at `HEAD`, and says not to re-land or reconcile the feature to
  restore it.
- `invalid-or-missing-evidence` with an ineligible descendant → **R26**:
  names the first ineligible descendant and its rule, and says to verify or
  repair it first.
- `invalid-or-missing-evidence` from a step that could not run → **R26**'s
  execution form: names the step, states that no descendant was judged, and
  says to fix the repository or tool failure and re-run verify before
  deciding whether to re-land.
- `unexplained-content-loss` → R1/R2, unchanged.

The human report prints one line after the advisories,
`  ~ overlap assessment: <class> (explanation only; verdict unchanged)`,
followed by one line per descendant.

### D27. What this amendment does not do

It does not implement Option A (no verdict ever turns green), Option B
(consolidation), member-level explanation (C3, folded into GH #26, Q9), or
the warn-class downgrade for superseded originals (Q6). It adds no trailer
key, no evidence writer and no persisted field (PRD §9.3 M1–M5). It does not
change how the ordinary (non-nested) pipeline resolves `HEAD` or allocates
shadows. Its only change to the ordinary pipeline is D24's captured-bytes
staging, which changes no output for an artifact that is stable during the
run.

## Amendment 2 — consequences

- A blocked landed parent whose content was rewritten by a recorded,
  verified, generation-bound hard descendant now says so, and stops telling
  the operator to re-land it.
- A blocked run pays for one nested verify per candidate plus one temp-index
  replay. Unblocked runs pay nothing.
- Readers may still treat a class as authority. `verdict_effect: "none"`,
  the unchanged exit code and the R25 text exist to prevent that.
- A concurrent writer that moves `HEAD` and restores it within one run can
  still make the target's own ladder and the assessment observe different
  trees; the post-assessment `HEAD` check catches every movement that is not
  restored. Ordinary verify has the same exposure today.

# Amendment 3 (2026-11-06, ACCEPTED) — The current anchor explained by one recorded child — GH #25 A1

**Status of this amendment**: ACCEPTED (2026-11-06; independent design
review round 1 UNSOUND, round 2 APPROVED). The operator authorized slice A1 for
the 2026-11-06T16:37:59Z window, on the GH #25 answers (PRD
`docs/prds/PRD-overlap-verification-consolidation.md` §8: Q2 ancestry only,
merges refused; Q3 a generation snapshot is required; Q4 hard edges only;
Q5 ladder step 1 only; Q11 `duplicate-equivalent` may not explain; Q12).
Binding since design review round 2. Adds
**D28–D37**; D38–D39 stay unused. Revision 2 answers design review round 1
(UNSOUND, F1–F4): D36 pins every `HEAD`-dependent read to the captured
commit (F1), P10 refuses merge topologies (F2), P9 requires the child's
own current anchor to be full-context clean (F3), and D37 drains the
closure arbitration's pending state on its fail-fast path (F4).

**Issue**: <https://github.com/tesseracode/tesserapatch/issues/25>

**Changes to prior decisions.** Anchor C's meaning widens (D28). D20's
constant `verdict_effect` gains a second value, D12's closed advisory set
gains one code, and D26's R25 gains a reason clause (D32). D27's "no
verdict ever turns green" is narrowed to everything outside D30.
D36 replaces the run's remaining live-`HEAD` reads with the captured
commit, and D37 fixes how D13's arbitration hands over its pending state.
Unchanged: D12's `(0/0)` rule, D13, D14, D17, D21's trigger and drop rules,
D22's procedure, D23, D24, D25, ADR-019, and the shape of the persisted
`VerifyRecord`.

## A3.0 Context

After Amendment 2, the PRD §2.1 minimal case — a parent lands, its hard
child rewrites one of the parent's lines and lands — reports
`recorded-descendant-evolution`, but verify still exits 2 at
`landed-content-absent` (AC-1). AC-2 asks that the parent pass in exactly
that case, with the historical half still passing on its own and a
mandatory advisory naming the child. A1 is the minimal slice: one direct
hard child, ordered by ancestry. Chains (A2), member-level explanation
(A3, folded into GH #26) and consolidation (B) stay out.

## Decision (Amendment 3, D28–D35)

### D28. Anchor C: present at `HEAD`, or explained by one recorded child

The V8 current half passes when the target's postimage is present at
`HEAD` (D11/D12, unchanged), **or** when D30's conditions hold. In the
second case the half reports the new anchor result
`explained-by-descendant`, never `materialized-clean`, so a reader can
always tell which claim was made.

### D29. The proof is D22's forward composition, not the PRD's reverse sketch

The PRD §6.1 sketch reverse-applies the descendants from `HEAD` and then
checks the target. A1 uses Amendment 2's D22 procedure unchanged instead,
because it proves more: step 5 shows the target's postimage was present at
its own landing commit `T` (the PRD's step 5), and step 7 shows `HEAD`'s
bytes on every target path equal `T`'s bytes evolved by the recorded
descendants' captured patches and nothing else (the PRD's step 6). A
reverse round-trip from `HEAD` proves only that the hunks invert there; it
cannot see an unrecorded edit outside them, which is why Amendment 2 rev-1
was rejected. A1 adds no Git surface.

### D30. Promotion conditions

The V8 current half is promoted only when **all** of these hold. When any
fails, the run is exactly what Amendment 2 reports.

- **P1. Class.** `overlap_assessment.class` is
  `recorded-descendant-evolution`: D22 steps 1–8 succeeded, with E1, E2,
  E4, E5 and E6 for every candidate.
- **P2. Not dropped.** The assessment survived D21's two drop rules: the
  inventory re-statement found no instability, and `HEAD` still resolves
  to the run's captured commit. Promotion therefore runs after both checks
  (D33), never inside `foldAnchorC`.
- **P3. Not nested.** A nested E6 verify never assesses (D21/D23), so it
  can never promote: the explaining child must pass its own verify
  natively, as the PRD requires for A1. This holds by construction: a
  child whose own anchor C is red because its own child rewrote it
  cannot be built end to end, since that grandchild's closure
  arbitration already fails on the rewritten child. A unit guard runs the
  nested verify of a promotable target directly and requires it to stay
  blocked with no assessment.
- **P4. One descendant.** The assessment lists exactly one descendant.
  Several overlapping descendants, ordered or not, stay explanation-only
  (A2).
- **P5. A direct child.** That descendant has a `hard` `depends_on` edge
  naming the target in the run inventory. A descendant that reaches the
  target only through intermediates stays explanation-only (A2), even
  though E4 bound every edge.
- **P6. Exact target.** The target's own landing evidence is `exact`. A
  `duplicate-equivalent` target is not promoted, mirroring Q11 for
  descendants.
- **P7. Historical half passed.** V8's `anchor_results.historical` is
  `passed` — not `skipped`, not `failed`.
- **P8. Nothing else failed.** Anchor C's block is the run's only failure:
  `failed_at` is `landed-content-absent` and was set by the anchor-C fold,
  and no other non-skipped `block` or `block-abort` check failed. If the
  run is red for another reason it stays red, and `failed_at` keeps
  pointing at the first failure.
- **P9. The child is clean with full context.** The child's own nested
  verify (E6) reported `anchor_results.current: materialized-clean`.
  E6 accepts a child whose ladder passed only with reduced context (a
  `context-drift` advisory); that child's own hunks are present but its
  surroundings changed after it landed, so its paths are not proven to be
  exactly what it recorded. Such a child can still explain the target's
  block (Amendment 2), but it cannot pass the target's check.
- **P10. No merge in `T..H`.** `git rev-list --merges --max-count=1
  T..H` at the captured `H` lists nothing. GH #25 Q2 refuses merge
  topologies in v1: a child that reaches `H` through a merge's second
  parent, and an unrelated merge anywhere in `T..H`, both refuse. A Git
  error refuses (fail-closed). This is the only Git query promotion adds;
  it reads the captured commit, never the live `HEAD`.

"Linear ancestry" (PRD slice plan) is E5 for the single descendant (its
attestation commit differs from `T`, descends from `T` and is an ancestor
of `H`) together with P10. Revision 1 allowed merges elsewhere in `T..H`
on the strength of step 7's byte compare; design review round 1 (F2)
rejected that, because Q2 refuses merge topologies outright and the
byte compare does not show which side of a merge an edit came from.

### D31. What promotion changes

- **V8.** `passed: true`, `skipped: false`, `anchor_results.current:
  explained-by-descendant`, `anchor_results.historical` unchanged
  (`passed`), and no remediation (a passing check carries none).
- **`failed_at`.** Cleared, together with `parent_slug` (P8 guarantees
  that anchor C set it).
- **Advisory.** One `warn` advisory, code `explained-by-descendant`,
  `slug` the target, appended after every other advisory. Its message
  (R27) names the child, the child's attestation commit and the target's
  landing commit, and says not to re-land or reconcile the target and to
  change those paths through the child.
- **`overlap_assessment.verdict_effect`.** `current-anchor-passed`
  instead of `none`. The rest of the object is unchanged.
- **Verdict.** Recomputed from the checks as always, so a promoted run
  exits 0 and its persisted `VerifyRecord` has `passed: true`.

### D32. Vocabulary, schema and text

- The report `schema_version` moves `1.2` → `1.3`. The change is
  additive: one new `anchor_results.current` value, one new
  `verdict_effect` value and one new advisory code. Consumers that refuse
  unknown majors are unaffected.
- D12's closed advisory set gains `explained-by-descendant`, still
  `warn`, still unable to flip `passed` on its own. It exists only next to
  a promoted V8.
- **R25** (a `recorded-descendant-evolution` assessment that is not
  promoted) now states why the explanation cannot pass the check: several
  descendants (P4), a descendant that is not a direct child (P5), the
  target's evidence (P6), the historical half (P7), the child's own
  context drift (P9), a merge in `T..H` or a merge query that could not
  answer (P10), or another failure in the run (P8). It keeps naming the
  descendants and keeps "do NOT re-land or reconcile".
- The human line reads `(explains the current anchor; V8 passes)` for a
  promoted assessment and stays `(explanation only; verdict unchanged)`
  otherwise.

### D33. Where promotion runs

`RunVerify` promotes after the instability re-statement and the
`HEAD`-still-captured check, and before the verdict is computed. The fold
records whether P4–P7 held and whether it set `failed_at`; the promotion
step checks P1, P2 and P8 on the finished report. `verify --all` runs the
same pipeline per entry, so a promoted entry is the same report on its own
and under `--all` (D37 keeps an earlier entry's arbitration state out of
a later one). The fold checks P1, P4–P7 and P9 in that order, then P10,
and records the first reason that fails; R25 states it. Amendment 2's boundary stays: only verify reads the
assessment (`TestOverlapAssessmentHasNoConsumers`); other commands see
only the verdict and the persisted record.

### D34. Persisted record and other readers

The persisted `VerifyRecord` keeps its shape. A promoted target records
`passed: true`, so its freshness label becomes `verified-fresh`. That label
already ignores later `HEAD` changes until verify runs again; a change to
the child (for example a re-record without a re-land, N6) turns the target
red on the next verify (E4), not before. `verify --all` counts a promoted
entry as passed (Q12 kept C's counts unchanged; A1 changes the verdict on
purpose). PRD §9.3 M1–M5 hold: no trailer key, no evidence writer, no
`status.json` field, and the record never claims a historical-anchor pass
that was not computed in the same run (M3).

### D35. What this amendment does not do

No chains or multiple descendants (A2). No member-level or Fusion
explanation (A3; GH #26 Amendment 4). No consolidation (B). No change to
D22's classes or steps, to E1–E6, or to the advisory severity. Revision 1
also excluded any change to how the ordinary pipeline resolves `HEAD`;
D36 withdraws that exclusion.

### D36. The whole run reads the captured `HEAD`

Design review round 1 (F1) found that D21's drop check compares only the
start and the end of the run: `HEAD` can move to another commit and back
(A→B→A) in between. Before A1 a run that read B in one check stayed red
or produced only an explanation; with promotion it could turn green on a
mixture of two commits. So every `HEAD`-dependent read of the ordinary
pipeline now uses the commit captured when the run context is built:

- V4's ancestry adapter answers the validator's `HEAD` with the captured
  commit.
- V5's `satisfied_by` reachability and the blocked-ladder diagnostics
  already use `headRev()`, which now returns the captured commit whenever
  one was captured, and `HEAD` only when the capture failed (an empty or
  broken repository keeps the shipped handling).
- The landing-evidence trailer enumeration lists commits reachable from
  the captured commit (`EnumerateCommitTrailersAt`), so a commit added
  while the run starts is never evidence for it.

D21's end-of-run check stays: it still drops an assessment when `HEAD`
did not come back. With D36, a run that observed A→B→A has read only A.

### D37. Arbitration state is drained on both paths

D13's closure arbitration collects member advisories and member baselines
on the run context. Its fail-fast path returned before they were cleared,
so under `verify --all` the next entry inherited the previous entry's
member advisories (F4; the arbitration fail-fast leak from the backlog).
The dynamic phase now takes and clears both immediately after the
arbitration returns, whichever path it took. This is a pre-existing
defect, fixed here because promotion makes cross-entry state
verdict-relevant.

## Amendment 3 — tests (PRD §9, mutation-sensitive)

| # | Case | Expected | Mutation it catches |
|---|---|---|---|
| A3-T1 (AC-2) | Minimal case | Parent exits 0; V8 current `explained-by-descendant`, historical `passed`; `failed_at` empty; one `explained-by-descendant` advisory naming the child and its commit; `verdict_effect: current-anchor-passed`; the persisted record passes | No promotion; promotion without the advisory; `failed_at` kept |
| A3-T2 (AC-3) | Child verify | Unchanged, exit 0, no assessment | — |
| A3-T3 (AC-5/AC-6) | Same state twice | Byte-identical JSON; no write to the tree, index, `.tpatch/` or worktrees | — |
| A3-T4 (P4) | Two direct children, ordered: the first edits the parent's path outside its hunks, the second rewrites a parent line (each passes its own verify) | `recorded-descendant-evolution`, exit 2, R25 names the P4 reason | Promotion without the single-descendant check |
| A3-T5 (P5) | Grandchild through a non-overlapping intermediate | `recorded-descendant-evolution`, exit 2, R25 names the P5 reason | Promotion without the direct-edge check |
| A3-T6 (P3) | The nested verify of the minimal case's promotable parent, run directly | Blocked, no assessment, no promotion | Assessment or promotion inside a nested verify |
| A3-T7 (P2) | `HEAD` moves during the run; inventory changes during the run | Assessment dropped, exit 2, R1/R2 or R20 | Promotion before the drop checks |
| A3-T8 (P1/P4–P9) | The promotion conditions and the promotion step on synthetic inputs: each condition broken alone | No promotion, and the condition's own reason | Each condition removed |
| A3-T8b (P8) | Minimal case plus a malformed `recipe-coverage.json` (V11 fails at `block`) | Exit 2, R25 names the P8 reason | Promotion without checking the rest of the run |
| A3-T9 (N-matrix) | N1, N2, N5, N6, N7, N9, N12, N19, parallel landings, E5 execution failure | Unchanged: exit 2, Amendment 2's class | Promotion ignoring the class |
| A3-T10 | `verify --all` on the minimal case | Both pass; each entry equals its single run | Nested or cross-entry state leaking into promotion |
| A3-T11 (P9) | The child drifted in context on its own second path; it passes its own verify with `context-drift` | Parent exit 2, R25 names the P9 reason | Promotion without P9 |
| A3-T12 (P10) | The child reaches `HEAD` as a merge's second parent; separately, an unrelated merge after the child | Exit 2, R25 names the P10 merge reason | Promotion without P10 |
| A3-T13 (P10) | The merge query fails (Git shim) | Exit 2, R25 names the P10 query reason | Treating a failed query as merge-free |
| A3-T14 (D36) | `HEAD` moved after the run context was captured; a dependency satisfied only at the moved commit | V4 and V5 fail at the captured commit (controls at the moved commit pass) | V4 or V5 reading the live `HEAD` |
| A3-T15 (D36) | `HEAD` moves right after `rev-parse HEAD` (Git shim) | The trailer enumeration never lists the moved commit | Enumerating the live `HEAD` |
| A3-T16 (D37) | `verify --all` where one entry's arbitration records a member advisory and then fails fast; the next entry verifies right after | Every entry equals its single run | Draining the pending state only on the success path |
| A3-T17 (D34) | Minimal case with persistence | `status.json` records `passed: true` | Persisting before the promotion |

## Amendment 3 — consequences

- The GH #25 minimal case passes, with an advisory that tells the operator
  why and what not to do.
- A composition bug is now a false green, not a misleading explanation.
  P1–P8, the N-matrix and the mutants exist to prevent that; the scope is
  the smallest case D22 already proves byte-exactly.
- A child that deletes or wholly rewrites the target's file is an
  explained evolution: the target passes because `HEAD` is exactly what
  the recorded child made of it.
- A target now depends on its child's evidence: changing the child without
  re-landing it turns the target red. This is the coupling the PRD
  accepted (§6.1 risks).
- Any merge after the target's landing keeps it red until A2 or a later
  slice decides merge topologies (P10). This is deliberately conservative.
- A run reads one commit throughout (D36). A `HEAD` that moves and does
  not come back still drops the assessment (D21); one that comes back is
  harmless because no check read the intermediate commit.

---

# Amendment 4 (2026-11-06, ACCEPTED 2026-11-07) — Attestation-preferred anchor selection and the R14 split — GH #26

**Status of this amendment**: ACCEPTED 2026-11-07 at rev-4, after the
design review the operator requested on 2026-11-07 APPROVED it
(`docs/supervisor/LOG.md`); the operator scheduled slice N1 for the next
window. N1 was implemented on 2026-11-11 (`internal/workflow/verify_landed.go`,
tests in `internal/workflow/verify_anchor_selection_test.go`) and its code
review approved it. Its D44 measurement found differences that rev-4's
pins did not name; revision 5 answers them, and its design review
APPROVED it on 2026-11-11. N1 shipped in v0.21.0. The operator
scheduled N2 on 2026-11-11 and it was implemented that day
(`internal/workflow/verify_anchored.go`, remediation strings in
`internal/workflow/verify_landed.go`, tests in
`internal/workflow/verify_r14b_test.go`); its code review approved it and
it is not yet released. The text below keeps the conditions it was proposed under. The operator's answers to
`docs/prds/PRD-historical-anchor-relanding.md` §9 (2026-11-06) approve its
semantics: option A (Q1), the newest qualifying exact attestation (Q2),
R14b with `failed_at` unchanged (Q3), A3 out of scope (Q5) and this
numbering (Q6). It becomes binding only after an independent design review
approves it and the operator schedules the slices: N1 implements D40, D41
and D44; N2 implements D42 and D43. Option D (the Fusion member-level
explanation, C3) follows N1–N2 (Q4) and needs its own amendment text and
design review; nothing here decides it.

**Revision 2 (2026-11-07)** answers design review rev-1 (NEEDS REVISION,
`docs/supervisor/LOG.md`): a shallow repository's "no" is undetermined
(F1, D43, T16); Amendment 3's promotion inputs follow D40 and are measured
(F2, D44, D45, T14); selection and ancestry aggregation are pinned by tests
that separate the wrong implementations (F3, D40, D42, T11, T12, T15);
forward-mode targets keep R14 (F4, D42, T13); code citations refreshed
(F5). T11–T16 are added here, not in the PRD.

**Revision 3 (2026-11-07)** answers design review rev-2 (NEEDS REVISION;
rev-1 F1, F4 and F5 closed): D43 is stated over the aggregate, so one yes
proves ancestry even when another query failed (rev-2 F1); D44 pins each
allowed difference with both rules' expected results, including T5's
red-to-red change of `failed_at` (rev-2 F2); T14 specifies its commit graph
and bindings, and its control passes E1, E4 and E5 so that only E6 can keep
it red (rev-2 F3).

**Revision 4 (2026-11-07)** answers design review rev-3 (NEEDS REVISION;
rev-2 F1 and F3 closed, no new false green): the oldest-first pins now
follow the code, where an arbitration failure skips only V8's historical
half while the current anchor and the overlap assessment still run (rev-3
F1); T14's E6 mutation is detected by the control's class and failing
rule, not its exit code, because P9 keeps it red either way (rev-3 F2).

**Revision 5 (2026-11-11)** answers N1's D44 measurement
(`docs/handoff/CURRENT.md`, SW-D9) and changes only D44. Every pinned
fixture matched its pin's verdict, `failed_at`, V7/V8 results and overlap
class, and no run changed its verdict without a moved anchor. The
measurement still blocked under rev-4, for three reasons that rev-5 now
names: (a) six runs of four unpinned fixtures differ exactly as T7 does:
AC-L29, AC-L36 and the two rev-2 duplicate-attestation tests. All four are
built by `RelandIdentical`, so every qualifier is an exact attestation of
one identity. (b) T7's own run also differs in two fields that rev-4's
"nothing else differs" did not name: the historical anchor's `commit` and
the target's own V10 member baseline. Both record the same selection as
the replay anchor. (c) T14 main also runs the child C's own verify. Its
change was implied, since E6 reads that verify, but not pinned. Rev-5 also
states how a difference in a run whose anchors did not move is told apart
from a harness artifact: by an A/A control. D40, the rest of the pass
criterion and every other pin are unchanged.

**Issue**: <https://github.com/tesseracode/tesserapatch/issues/26>
**Numbering**: Amendment 3 is reserved for GH #25 A1 (the anchor-C
widening, OV-D4) and is drafted after this text. So that neither draft is
renumbered, Amendment 3 takes **D28–D39** and this amendment takes
**D40–D45**; numbers Amendment 3 does not use stay unused. Measurements
are labelled **A4-E1…A4-E6**, so they cannot collide with Amendment 1's
closed E1–E47 index.

**Changes to prior decisions.** D14 step 4's selection rule changes for one
case: an exact attestation that qualifies (D40). Rejected alternative 12 is
narrowed, not reversed (D41). D13's R14 is split on ancestry (D42–D43).
Every other decision — D10 presence, D12, D14 steps 1–3, D15, D17, D18,
Amendment 2 — is unchanged.

## A4.0 Context

A child feature lands. Its hard parent later changes and lands again. The
child is genuinely unapplied, re-applied on the parent's new landing and
landed again with identical patch and recipe bytes. `tpatch verify <child>`
still exits 2 with `failed_at: parent-landing-drift` and R14 ("verify the
parent first"), which cannot help (GH #26; PRD §2.1, reproduced on
v0.18.0).

Both of the child's landings forward-qualify at `-C1` and have equal D18
identities, so D14 step 4 picks the **oldest**, the landing made before the
parent changed. D13 then probes the parent's **current** patch at that old
tree, where it is absent, and the member ladder blocks (PRD §2.2). Oldest-
first exists for E30: after a re-land on top of an existing landing, the
newest landing's parent already contains the patch and cannot be a replay
baseline. In GH #26 the newest landing's parent does qualify.

## A4.1 Empirical basis (scratch repository, git 2.55.0, macOS; removed afterwards)

Plain Git, reproducing D14's probes: the forward check is
`GIT_INDEX_FILE=<tmp> git read-tree C^` then
`git apply --check --cached -C1`; the member ladder's first rung is the
full-context `--reverse` check.

| ID | Observation | Consequence |
|---|---|---|
| A4-E1 | The child landed at `L1`; the parent changed and re-landed; the child was genuinely unapplied, re-applied and re-landed at `L2` with identical patch bytes. The child's patch forward-qualifies at `-C1` at both `L1^` and `L2^`, and the two landings' D18 normalized identities are equal. | Both are equal-identity qualifiers; today's step 4 picks `L1`. (D40) |
| A4-E2 | The parent's current canonical patch fails the full-context reverse check at `L1^` and passes it at `L2^`. | At the oldest-first anchor the member ladder blocks (R14); at the attestation it does not. (D40) |
| A4-E3 | E30 restated: after a further landing on top while the child stays applied, the child's patch does not forward-qualify at that landing's parent. | Such an attestation is not a qualifier, so D40 never selects it. (D41) |
| A4-E4 | The parent's re-landing commit is not an ancestor of `L1^` (`git merge-base --is-ancestor` exit 1) and is an ancestor of `L2^` (exit 0). | Ancestry separates "the parent's current landing is not in the history of the child's baseline" from the R14 case. Non-ancestry does not prove the baseline is older: on a divergent branch the two commits are incomparable. (D42) |
| A4-E5 | A descendant rewrites the parent's line and a later child lands on top. The parent's current patch blocks at the child's anchor, and the parent's landing **is** an ancestor of that anchor. | The R14 (Fusion) case; R14 stays. (D42) |
| A4-E6 | `git merge-base --is-ancestor` naming an object the repository lacks exits 128 (`fatal: Not a valid commit name …`), neither 0 nor 1. | An ancestry query can fail to answer; that must not read as "not an ancestor". (D43) |

## Decision (Amendment 4, D40–D45)

### D40. D14 step 4 prefers a qualifying exact attestation

D14 step 4 becomes:

> 4. **Select.** Let *A* be the qualifiers from step 3 that are **exact
>    attestation commits** of the feature: commits whose patch, recipe and
>    base-commit trailers all match the snapshot, the test D10 already
>    applies for `landing_evidence` (one commit for `exact`, each duplicate
>    for `duplicate-equivalent`;
>    `internal/workflow/verify_landed.go:885-891`, `:900-948`). If *A* is
>    non-empty, select its **newest** member: the last in the enumeration's
>    `--topo-order --reverse` order. Otherwise take the first qualifier in
>    that order, as before. Final tie-break unchanged.

- **Steps 1–3 are unchanged** (`internal/workflow/verify_landed.go:1259-1318`).
  Every candidate is collected and qualified, and identities are compared
  across all qualifiers, before any selection. Non-equivalent qualifiers
  stay terminal even when an attestation qualifies (PRD T6).
  Non-equivalent exact attestations are `ambiguous` evidence and terminal
  before anchor selection runs (PRD T6b).
- **A probe that cannot run** still aborts resolution
  (`internal/workflow/verify_landed.go:1282-1287`); it is never read as
  "does not qualify", so it can never fall back to oldest-first silently
  (PRD T8).
- **Only exact attestations are preferred.** When *A* is non-empty the
  selection is among *A* only: a later qualifier that is not an exact
  attestation (for example a landing whose recipe trailer predates the
  snapshot but whose change is equivalent) never displaces an earlier
  exact attestation (T11).
- **Merge landings** are never collected (step 1(c)), so a merge-landing
  attestation is never preferred.
- **No new Git call.** *A* is computed from the run's single snapshot and
  enumeration (D17).
- **Members follow the same rule.** Anchor resolution is shared and
  memoised per slug, so each landed closure member's anchor, and V10's
  member baselines (D15), use D40 too.
- **Reporting is unchanged.** `landing_evidence.attestation_commit` keeps
  its D10 meaning (for `duplicate-equivalent`, the first exact match), and
  `baseline.historical_anchor.replay_anchor_commit` names the selected
  landing. For `exact` with a qualifying attestation they are equal. No
  field is added and the report schema does not change.
- **Why the newest exact attestation (Q2).** ADR-019 tells trailer
  consumers to pick the most recent of a re-landed slug's commits
  (`ADR-019-tpatch-land-trailer-block-schema.md:64-69`). Every member of
  *A* carries the recorded base trailer and passed the same forward check,
  so the newest is the latest landing that still accepts the recorded
  patch.

### D41. Rejected alternative 12 is narrowed, not reversed

Alternative 12 rejected using the attestation as the replay anchor
**unconditionally**, because its parent may already contain the patch
(E30). D40 uses an attestation only after it passes the same `-C1` forward
qualification as every other candidate. When it does not (E30, A4-E3),
selection is oldest-first exactly as before. Alternative 12 carries a
pointer to D40 since acceptance (2026-11-07).

### D42. R14 splits on ancestry (R14b)

In closure arbitration (D13), when a landed hard member's ladder blocks at
the target's anchor tree
(`internal/workflow/verify_anchored.go:740-742`), verify asks, through the
existing offline ancestry gateway
(`internal/workflow/verify_gitgate.go:73`), whether **any** of the member's
exact attestation commits (one for `exact`, each duplicate for
`duplicate-equivalent`) equals the target's anchor commit or is an
ancestor of it. The anchor is the replay landing's parent: the tree the
target was applied on.

- **None is** → **R14b**. It names the target's replay anchor commit and
  the member's reported attestation commit, states that no exact landing
  of the member is in the history of the target's replay baseline (the
  target was applied before that landing, or on a branch that never
  contained it) and that verifying the member will not help, and
  recommends genuinely unapplying the target,
  re-applying it on the member's current landing, and recording and landing
  it again. Under D40 that new landing's parent qualifies and becomes the
  anchor (A4-E1, A4-E2).
- **At least one is** → R14, unchanged: the member's content landed in the
  anchor's history and is absent there (the Fusion case, A4-E5).
- **Landed targets only.** The split applies only when the target is
  verified in landed mode, where the anchor tree is its historical replay
  anchor (`internal/workflow/verify_anchored.go:481-495`). A target verified
  in forward mode (evidence `none`) arbitrates at the captured `HEAD`, has
  no replay anchor to name, and keeps R14 unchanged with no ancestry query
  (T13).
- **Aggregation.** Every exact attestation of the member is queried, not
  only the reported `attestation_commit` (T12). The outcome depends only on
  the set of answers, never on their order: any **yes** → R14; otherwise
  any undetermined answer (D43) → R14b's undetermined form; otherwise
  (every answer a determined **no**) → R14b. A yes may stop the remaining
  queries because it decides the outcome whatever they answer; an
  undetermined answer may not (T15).

`failed_at` stays `parent-landing-drift`. Verdict, exit code, checks,
advisories and the persisted record are unchanged; only the remediation
text changes.

### D43. An ancestry query that cannot answer makes no ancestry claim

A query Git cannot answer (A4-E6: an exit status other than 0 or 1,
including a missing object under `GIT_NO_LAZY_FETCH=1`) supplies **no
answer**: it is read as neither yes nor no. D42's predicate is existential
(*some* exact landing of the member is in the anchor's history), so a yes
from another query proves it whatever the failed query would have said,
and R14 stands (T15). When no query answered yes and at least one could not
answer, verify emits **R14b's undetermined form**: it names the member and
the target's replay anchor, states that verify could not determine whether
any exact landing of the member is in the anchor's history and therefore
judged neither case, and says to restore the repository's history (for
example `git fetch --refetch`, or unshallow) and run verify again. R14
needs at least one yes and R14b's ancestry claim needs every query to have
answered a determined no; neither is ever inferred from a query that did
not answer. `failed_at` stays `parent-landing-drift` (Q3). This mirrors
Amendment 2's R26 execution form (D26): a query that could not run is never
reported as an answer.

**Shallow repositories.** In a shallow repository (the run's preflight
`git rev-parse --is-shallow-repository`, `internal/gitutil/trailers.go:153`),
a **no** (exit 1) is undetermined too, and is aggregated as one (D42):
traversal stops at the graft boundary, so Git can answer no for a landing that the full history places
in the anchor's history. A **yes** stays decisive, because the path Git
found exists. The repository-level flag is used, not a per-commit boundary
test, because the cut can lie anywhere between the two commits (T16).

### D44. Measure before release (N1)

D40 changes the replay tree of a run whose qualifying exact attestation is
not the oldest qualifier, for the target and for every landed closure
member and nested child that shares anchor resolution (D45). Equal identity
means both trees accept the same change, so such a run can newly fail only
if the newer tree breaks the target's recipe replay (V7, V8's historical
half) or a member's ladder or V10 baseline, and can newly pass or be
promoted only through those same inputs. Before N1 is released:

1. every verify fixture in the repository runs under both selection rules.
   For each run the measurement records the verdict, `failed_at`, the
   remediation, `anchor_results` (historical and current),
   `baseline.historical_anchor.replay_anchor_commit` of the target and
   each landed member, Amendment 3's promotion
   (`explained-by-descendant`) and the nested E6 verdicts;
2. GH #26's t3code `5ce5d79` reproduction is re-run read-only on the N1
   build and the four roots' results are recorded.

**Pass criterion.** A difference between the two rules is allowed only when
(a) a selected replay anchor (the target's, a member's or a nested
child's) moved from the oldest qualifier to a newer exact attestation, and
(b) the fixture is pinned below and each rule's result equals its pin. A
run whose anchors did not move must be identical under both rules. Every
other difference blocks N1, in either direction: a green that turns red; a
red that turns green, or a new promotion, without a moved anchor; or a
changed verdict, anchor result, `failed_at` or remediation that no pin
names. A blocking difference is resolved by revising this amendment and
repeating its design review, not by the N1 code review alone.

**Harness artifacts (rev-5).** Running both rules on one fixture needs a
shadow run beside the real one, and the shadow can change what the real
run sees. It consumes a fault that a test injects only once. It never
writes, while the real run persists. In `verify --all` it can also observe
the real run's writes. A difference in a run whose anchors did not move
counts as such an artifact, not as a rule difference, only when an A/A
control reproduces it. The A/A control is the same harness with D40 on both
sides; it must record a difference for the same test, feature, mode and call
number, with each side's report equal to the measured run's, field for
field, apart from commit IDs and temporary paths, which change with every
fixture build. Any other such difference blocks. A test that fails only under the harness must fail the
same way in the A/A control.

| Fixture | Oldest-first (today) | D40 |
|---|---|---|
| T1 | Exit 2, `parent-landing-drift`, R14. Arbitration fails before the historical half replays (`internal/workflow/verify_anchored.go:513-519`, `:669-674`): V7 failed, V8's historical half `skipped`. The current anchor is still evaluated (`:276-280`): `materialized-clean`. | Exit 0. V7 passed; V8 historical `passed`, current `materialized-clean`; replay anchor = the attestation. |
| T5 | Exit 2, `parent-landing-drift`, R14. V7 failed; V8 failed with historical `skipped`, current `absent`; the overlap assessment runs (`:311-328`) and finds no recorded descendant (`unexplained-content-loss`). | Exit 2, `landed-content-absent`. V7 passed; V8 failed with historical `passed`, current `absent`; the same assessment. |
| T7 | Exit 0; replay anchor = the older exact attestation. | Exit 0; replay anchor = the newer. The historical anchor's `commit` and the target's own V10 member baseline record the same selection and move with it (rev-5). Nothing else differs. |
| T7-class (rev-5) | AC-L29 (two identical re-landings, so three qualifiers), AC-L36 (one identical re-landing, verified three times) and the precondition run of `newDuplicateAttestationFixture` (`TestRev2_DuplicateAttestationGenericDiffFailureIsUnavailable`, `TestRev2_DuplicateAttestationMissingObjectIsHistoryIncomplete`). `RelandIdentical` re-lands the same change with the same trailer block, so every qualifier is an exact attestation of one identity. As T7, at the oldest. | As T7, at the newest. |
| T14 main | Exit 2, `parent-landing-drift`: the target's arbitration fails at T1, where P2's patch is absent. V7 failed; V8 failed with historical `skipped`, current `absent`. The assessment runs: C passes E1, E2, E4 and E5 and fails E6 (its nested verify also selects C1 and fails), so the class is `invalid-or-missing-evidence`. No promotion. | Exit 0, promoted: V8 historical `passed`, current `explained-by-descendant`; `failed_at` empty. |
| T14 main, C's own verify (rev-5) | Exit 2, `parent-landing-drift`: C's anchor is C1, where P2's patch is absent. V7 failed; V8 historical `skipped`, current `materialized-clean`. This is the verify E6 reads. | Exit 0; replay anchor = C2. V7 passed; V8 historical `passed`, current `materialized-clean`. |
| T14 control | As T14 main. | Exit 2, `landed-content-absent`. V8 failed with historical `passed`, current `absent`; class `invalid-or-missing-evidence`, with E6 the child's only failing rule. |

T11 needs no pin: its exact attestation is also its oldest qualifier, so
both rules select it and its runs must be identical.

### D45. What this amendment does not do

- No option B (generation-bound landing observation), no option D (C3
  member-level explanation, after N1–N2 under its own amendment) and no A3
  (member-level authority, out of scope).
- No new trailer, generation kind, persisted field or report field; no
  report schema change.
- No change to `landing_evidence` semantics, `-C1` qualification, the
  identity compare, anchor C, D10 presence or V10's rule.
- **Amendment 3's conditions are unchanged, but two of their inputs follow
  D40.** P7 reads V8's historical half, which replays at the D40-selected
  anchor, and E6/P9 read the explaining child's own nested verify, whose
  anchor and member ladders use D40 too. A run that is red today can
  therefore be promoted to `explained-by-descendant` after N1 when the
  target or the explaining child is a genuine GH #26 re-landing. That is
  intended — each input equals what a single verify of that feature
  reports — and D44 measures it; T14 pins it with a negative control.

## Amendment 4 — tests (PRD §7, mutation-sensitive)

Each fixture must fail under the named mutation. T1–T4 also run under
`verify --all`, and each embedded report must equal its single run.

| # | Fixture | Expected | Mutation that must fail it | Decision |
|---|---|---|---|---|
| T1 | GH #26: child genuinely unapplied, re-applied and re-landed with identical bytes after the parent re-landed. | Pass; replay anchor = attestation. | Oldest-first restored. | D40 |
| T2 | E30: a further landing on top of an existing landing. | Pass; replay anchor = the earlier landing. | Attestation used without qualification. | D41 |
| T3 | Parent re-landed; child not re-landed. | Exit 2, `parent-landing-drift`, R14b naming both commits. | Ancestry query dropped (R14 always). | D42 |
| T4 | Member ladder blocks and the member's attestation is an ancestor of the anchor. | R14 unchanged. | R14b always. | D42 |
| T5 | T1 plus a later unrecorded edit at `HEAD` on the child's path. | Exit 2 at the current anchor. | Anchor C skipped when anchor H passes. | D45 |
| T6 | A qualifying exact attestation plus a stale-trailer landing that forward-qualifies with a non-equivalent identity. | Terminal `historical-anchor-unavailable`. | Identity compare skipped when the attestation qualifies. | D40 |
| T6b | Two exact attestations with non-equivalent identities. | `failed_at: landing-evidence`, R7; D40 never runs. | D40 runs before evidence classification. | D40 |
| T7 | `duplicate-equivalent`, no hard parent: two exact attestations, both qualifying. | Replay anchor = the newer. | Oldest among *A*. | D40 |
| T8 | The forward check cannot run on the attestation candidate. | Reader-failure state, as today. | The error read as "does not qualify". | D40 |
| T10 | Divergent branch: the target was applied and landed on a branch that never contained the member's current landing, then merged (the member landing reachable only through a merge's non-first parent, `internal/gitutil/trailers.go:286-289`). | R14b; its text claims only that no exact member landing is in the baseline's history, never that the baseline is older. | R14b text restored to "predates". | D42 |
| T9 | T3 with D42's ancestry query failing (simulated Git failure). | R14b's undetermined form; `failed_at` unchanged. | The failure read as "not an ancestor" (R14b's claim) or as "ancestor" (R14). | D43 |
| T11 | An `exact` attestation qualifies; a later single-parent landing of the slug with a stale recipe trailer forward-qualifies with an equal identity. | Replay anchor = the exact attestation. | The newest qualifier selected whenever *A* is non-empty. | D40 |
| T12 | `duplicate-equivalent` member: the reported `attestation_commit` is not in the target anchor's history; the other exact attestation is. | R14 unchanged. | Only `attestation_commit` queried (R14b). | D42 |
| T13 | Forward-mode target (evidence `none`) whose landed hard parent's patch is reverted at `HEAD`. | R14 unchanged; no ancestry query runs. | The split applied in forward mode (R14b naming a replay anchor). | D42 |
| T14 | Amendment 3 combined. Hard chain P → T → C (T is the target; C, its one recorded child, has T as hard parent; T has P). The patch bytes of T and C never change. P lands (P1); T lands (T1); C lands (C1), rewriting lines T added. C and T are then unapplied, P re-lands with a changed patch that touches only its own lines (P2), T is re-applied and re-lands (T2), and C is re-applied and re-lands (C2). Each re-application records a new base, so T2 and C2 are the only exact attestations. C's latest generation binds T's unchanged patch, so E4 holds without any re-record refreshing it; the fixture writes both manifests explicitly. Control: identical, except that C is re-applied by a commit with no `Tpatch-Feature` trailer and C2 lands on top of it, so (as in A4-E3) C2's parent already contains C's patch, C2 does not qualify and C's anchor falls back to C1, where P2's patch is absent. Both cases first assert E1 (C `exact`), E4, E5 (C2 descends from T2 and is an ancestor of `H`), the target's clean historical half (P7), and that C's own verify reports its current anchor `materialized-clean`. | Main: under D40 the child's nested verify passes and the target is promoted (`explained-by-descendant`). Control: the target stays red; class `invalid-or-missing-evidence`, with E6 the child's only failing rule. | Nested E6 anchor selected oldest-first (the main case stays red), or E6 dropped from eligibility (the control's class becomes `recorded-descendant-evolution`; P9 still keeps it red, because a failed nested verify never reports its current anchor clean, `internal/workflow/verify_overlap.go:601-614`, so the control asserts the class and failing rule, not the exit code). | D40, D45 |
| T15 | Member with two exact attestations whose ancestry queries answer {failure, yes} and {failure, no}, each in both enumeration orders. | {failure, yes} → R14; {failure, no} → R14b's undetermined form; identical in both orders. | The first answer decides, or querying stops at the first failure. | D42, D43 |
| T16 | Shallow clone: the member's exact landing and the target's anchor are both present, with their parents, above the graft boundary, but the history joining them is cut (the landing is reachable from `HEAD` by a shorter path), so `--is-ancestor` exits 1. Control: the same history unshallowed. | Shallow: R14b's undetermined form. Control: R14. | A shallow **no** read as determined (R14b's claim). | D43 |

## Amendment 4 — consequences

- A genuinely re-landed child (GH #26) verifies against the base it was
  landed on.
- A child that really is older than its parent's current landing gets a
  remediation that names both commits and an action that works.
- Some passing runs replay at a newer tree; D44 measures them before
  release.
- A shallow repository gets R14b's undetermined form instead of an
  ancestry claim it cannot support.
- Cost: D40 adds no Git call; D42 adds at most one ancestry query per exact
  attestation of a member (stopping at the first yes), and only when the
  target is in landed mode and that member's ladder has already blocked.
