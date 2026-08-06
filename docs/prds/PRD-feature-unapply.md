# PRD - Feature Unapply - `feat-feature-unapply`

**Status**: Rev-0 (Cluster G planning)
**Date**: 2026-07-10 (orig. draft U63); 2026-08-05 (Cluster G refresh)
**Owner**: Core
**Byline**: U63; Cluster G refresh
**Milestone**: v0.14.0 candidate
**Depends on**: [PRD-feature-dependencies](./PRD-feature-dependencies.md), [PRD-feature-patch-identity-metadata](./PRD-feature-patch-identity-metadata.md), [PRD-tpatch-land](./PRD-tpatch-land.md). Implementation requires `ADR-032-feature-unapply-state-boundary`.

## Related

- [Feature Layout](../feature-layout.md)
- [Recording Patches](../record.md)
- [Reconcile Workflow](../reconcile.md)
- [Feature Dependencies](../dependencies.md)
- [ADR-011 feature dependencies](../adrs/ADR-011-feature-dependencies.md)
- [ADR-019 tpatch land trailer block schema](../adrs/ADR-019-tpatch-land-trailer-block-schema.md)
- [ADR-024 patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md)
- [ADR-031 rejected feature state data model](../adrs/ADR-031-rejected-feature-state-data-model.md) — D6 deferred post-implementation retirement to this PRD; D10 intentional-non-relationship naming precedent for this PRD's §3.4.
- [ADR-032 feature unapply state boundary](../adrs/ADR-032-feature-unapply-state-boundary.md) — the binding ADR this PRD requires before implementation.
- [PRD-rejected-feature-state](./PRD-rejected-feature-state.md) — v0.13.0 pre-implementation retirement; shipped. Understanding the reject/reopen lifecycle is required reading before §8.2–§8.3 and §11.7 of this PRD.
- [PRD-feature-patch-amend](./PRD-feature-patch-amend.md)
- [PRD-recurring-patches](./PRD-recurring-patches.md)
- [PRD-patch-already-upstream-detector](./PRD-patch-already-upstream-detector.md)

## Implementation Gate

Implementation requires a small binding ADR before code lands:

```text
ADR-032-feature-unapply-state-boundary
```

The ADR should lock:

- `unapplied` as a real `FeatureState`;
- which states satisfy hard dependencies after unapply;
- unapply audit artifact schema;
- no patch-generation writes in v1;
- patch-mode-only v1 scope;
- failure atomicity guarantees;
- composition with `rejected` state (D7 — the deferral from ADR-031 D6);
- command-name pattern (`feature unapply` under `feature` group vs top-level) (D8).

This PRD can be reviewed as paper design before the ADR exists, but implementation
must not start until the ADR is accepted.

## 0. Claims Audit

This PRD is a proposal. It changes nothing.

| Claim | Evidence |
|---|---|
| The canonical replay patch for a feature is `.tpatch/features/<slug>/artifacts/post-apply.patch`; numbered files under `patches/` are audit snapshots, not replay authority. | `docs/feature-layout.md:34-44`, `docs/feature-layout.md:50-67` |
| `record` captures working-tree or commit-range diffs and writes the canonical patch; same-feature duplicate records skip numbered snapshots. | `docs/record.md:1-3`, `docs/record.md:23-37`, `docs/record.md:89-90` |
| Reconcile already uses a reverse-apply check to detect whether a recorded patch is present in the tree. | `docs/reconcile.md:5-14`, `internal/workflow/reconcile.go:205-214` |
| The git utility layer already exposes reverse-apply checks and reverse validation using `git apply --reverse --check`. | `internal/gitutil/gitutil.go:396-435` |
| Reconcile refuses dirty trees, merge conflict markers, and `.orig` / `.rej` leftovers because patch verdicts become unreliable on polluted trees. | `docs/reconcile.md:47-83`, `internal/gitutil/gitutil.go:85-187` |
| Existing feature states do not include an `unapplied` state. | `internal/store/types.go:5-24` |
| Hard dependencies are satisfied only by applied/active/upstream-merged parents; removal with dependents refuses unless `--cascade` is used, and `--force` does not bypass dependency integrity. | `docs/dependencies.md:95-112`, `docs/dependencies.md:236-251`, `ADR-011-feature-dependencies.md:76-90` |
| `remove` deletes the feature directory and all artifacts; it is not a patch-removal operation. | `internal/cli/c1.go:339-399`, `internal/store/store.go:509-521` |
| Feature management subcommands live under `tpatch feature`; current children are `deps`, `claim`, and `patch`. | `internal/cli/feature_deps.go:37-50` |
| `land` writes ordinary Git commits with a locked trailer block; `Tpatch-Feature` is the feature-to-commit binding and is not unique across commits. | `docs/feature-layout.md:90-94`, `ADR-019-tpatch-land-trailer-block-schema.md:21-24`, `ADR-019-tpatch-land-trailer-block-schema.md:64-69` |
| Patch generations track changes to canonical patch bytes and deliberately do not replace `status.json`, `post-apply.patch`, numbered patches, or land trailers. | `PRD-feature-patch-identity-metadata.md:80-88`, `ADR-024-patch-generation-manifest-boundary.md:48-75` |
| **v0.13.0 / Cluster F additions (for §8.2, §11.7 of this PRD)** | |
| `StateRejected` is the eleventh `FeatureState` (value `"rejected"`), introduced in v0.13.0 GH #6. The `ValidFeatureState` closed switch includes it. | `internal/store/types.go:21-30` |
| `RejectableStates` restricts `tpatch reject` to pre-implementation states: `requested`, `analyzed`, `defined`. Post-implementation states (including `implementing`, `applied`, `active`, `reconciling`, `reconciling-shadow`, `blocked`, `upstream_merged`) are refused outright at exit code 3 (ADR-031 D6). `unapplied` — when added — is also post-implementation and therefore implicitly refused. | `internal/store/status.go:108-118` |
| `RejectionStatus` is the live rejection sub-record on `FeatureStatus` (`reason`, `note`, `actor`, `evidence []EvidenceRef`, `rejected_at`, `prior_state`). `RejectionHistoryEntry` records one completed reject→reopen cycle; `FeatureStatus.RejectionHistory` is append-only and grows by exactly one entry per `tpatch reopen`. | `internal/store/status.go:98-106`, `internal/store/status.go:59-96` |
| Rule 7 (`ErrRejectedParent`): dependency-edge creation onto a `rejected` parent is refused for all three edge kinds (`hard`, `soft`, `supersedes`) with exit code 3. The remediation message names `tpatch reopen`. | `internal/store/validation.go:57-70`, `internal/store/validation.go:168-181` |
| `tpatch reject` and `tpatch reopen` are top-level lifecycle verbs. They are intentionally distinct from `tpatch feature <subcommand>` noun-scoped management per ADR-031 D10 Alternative 3. The intentional-non-relationship rationale is documented in PRD-rejected-feature-state §4.1 and ADR-031 D10. | `internal/cli/reject.go:1-40`, `internal/cli/reopen.go:1-40`, `ADR-031-rejected-feature-state-data-model.md:889-985` |
| `tpatch status` excludes `rejected` features from the default listing. `--include-rejected` opts in. The count of hidden rejected features is shown in the footer. | `internal/cli/cobra.go:250`, `internal/cli/cobra.go:397`, `internal/cli/cobra.go:529` |
| ADR-031 D6 explicitly deferred post-implementation retirement ("should never have shipped" command) to a future ADR, naming `PRD-feature-unapply.md` as the potential host. | `docs/adrs/ADR-031-rejected-feature-state-data-model.md:604-630` |

## Summary

Add a conservative `tpatch feature unapply <slug>` command that removes a
feature's current canonical patch from the working tree while preserving the
feature directory, canonical patch, patch-generation history, and audit trail.

V1 is patch-mode only:

```bash
tpatch feature unapply <slug> [--dry-run] [--allow-soft-dependents]
```

Unapply is a working-tree operation, not feature deletion and not Git history
rewriting. It should refuse when safety is unclear and explain why before
mutating anything.

## 1. Problem Statement

tpatch can record, replay, reconcile, land, and track feature patches. Users also
need a safe way to temporarily or permanently remove one feature's changes from
the working tree while retaining the feature as a tracked patch.

Existing primitives partially cover this:

- `git apply -R .tpatch/features/<slug>/artifacts/post-apply.patch` can remove
  cleanly-present patch bytes, but it bypasses tpatch dependency checks, audit
  artifacts, status updates, and preflight diagnostics.
- Reconcile's reverse-apply logic can detect patch presence, but reconcile uses
  the result to classify upstream state, not to intentionally remove local
  changes.
- `tpatch remove <slug>` deletes `.tpatch/features/<slug>/`; it is too
  destructive for "remove this patch from my worktree but keep tracking it."
- `git revert <landed-commit>` may be useful when a feature was landed as a Git
  commit, but it is a separate problem because landed commits include tpatch
  metadata and may not match the current canonical generation.

The missing capability is a first-party, dependency-aware, dry-run-friendly,
audited unapply operation.

## 2. Goals / Non-goals

### Goals

1. Remove a feature's current patch from the working tree when it can be proven
   safe.
2. Preserve `.tpatch/features/<slug>/` by default.
3. Preserve `artifacts/post-apply.patch` as the canonical patch for future
   reapply/reconcile.
4. Write audit artifacts for each successful unapply attempt under
   `artifacts/unapply/`.
5. Refuse by default when child dependents exist.
6. Refuse dirty/conflicted trees unless `--dry-run` is only reporting.
7. Leave the working tree unchanged on failed or partial reverse-apply.
8. Add explicit lifecycle semantics for "recorded patch exists, but is not
   present in this working tree."

### Non-goals

1. No deletion of feature metadata. `tpatch remove` remains the deletion command.
2. No Git history rewriting.
3. No default `git revert`.
4. No landed-commit unapply in v1.
5. No best-effort partial patch removal.
6. No semantic patch surgery or provider-assisted unapply in v1.
7. No changes to `artifacts/post-apply.patch` as canonical replay authority.
8. No patch-generation writes in v1.
9. No automatic cascade unapply of children in v1.
10. No `feature reapply` alias in v1; existing `tpatch apply <slug>` is the
    reapply path.

## 3. Command Name / User-facing Contract

Selected command:

```bash
tpatch feature unapply <slug> [--dry-run] [--allow-soft-dependents]
```

### 3.1 Candidate command evaluation

| Candidate | Verdict | Reason |
|---|---|---|
| `tpatch feature unapply <slug>` | Selected | Describes "remove this patch from the working tree" without implying metadata deletion or Git commit revert. Fits existing `feature` namespace. |
| `tpatch feature drop <slug>` | Rejected | Patch-stack language, but "drop" can imply deleting/abandoning metadata. |
| `tpatch patch drop <slug>` | Rejected | New `patch` top-level namespace and ambiguous "drop patch" semantics. |
| `tpatch remove <slug> --revert` | Rejected | `remove` already means delete feature directory; adding `--revert` would mix metadata deletion with working-tree mutation. |
| `tpatch feature revert <slug>` | Rejected | "Revert" strongly suggests `git revert` or history-level semantics. |

### 3.2 Default behavior

Default invocation:

```bash
tpatch feature unapply auth-timeout
```

Behavior:

1. Load feature status and canonical patch.
2. Run dependency safety checks.
3. Run working-tree safety checks.
4. Verify the canonical patch reverse-applies cleanly without mutation.
5. Preview the reverse patch in a temporary worktree.
6. Snapshot touched files from the real worktree.
7. Apply the reverse patch to the real worktree.
8. If the real apply fails, restore touched files from the snapshot and fail.
9. Write `artifacts/unapply/<attempt-id>/unapply-session.json`.
10. Write `artifacts/unapply/<attempt-id>/reverse.patch`.
11. Update `status.json` with `state: "unapplied"`, `last_command:
    "feature unapply"`, and a note describing the audit artifact.

### 3.3 Dry-run

```bash
tpatch feature unapply auth-timeout --dry-run
```

Dry-run is read-only. It reports:

- feature state and canonical patch SHA;
- files touched by the canonical patch;
- dependency blockers and dependent children;
- working-tree cleanliness / conflict marker status;
- whether reverse-apply would succeed;
- which audit artifacts would be written.

Dirty-tree dry-run exits 0 and reports blockers because it is a report-only
operation. Invalid slug, missing feature, unreadable status, or missing canonical
patch exit non-zero. Dry-run must not modify the worktree, index, or `.tpatch/`.

### 3.4 Intentional naming asymmetry with `reject` / `reopen`

**This section documents an intentional design decision; it is not a placeholder.** The
Cluster F rev-5 precedent (PRD-rejected-feature-state §4.1, ADR-031 D10) should be read
first.

`tpatch feature unapply` sits under the `tpatch feature` noun-scoped subcommand group.
`tpatch reject` and `tpatch reopen` are bare top-level lifecycle verbs. At the CLI surface
this looks asymmetric. The asymmetry is correct and deliberate.

**Why `feature unapply` is under `feature`:**

1. `unapply` is a **working-tree artifact operation**: it reverse-applies the canonical
   patch, writes an audit session under `artifacts/unapply/`, and leaves all feature
   metadata intact. The natural scoping is the feature's artifact namespace — hence `feature
   <noun> <slug>`. This mirrors `feature deps <slug> add/remove` (DAG-edge management) and
   `feature patch <slug>` (patch generation browser): all are noun-scoped per-feature
   management operations, not lifecycle verbs.

2. `tpatch feature` is specifically the group for "operate on a feature's artifacts or
   sub-records" (`internal/cli/feature_deps.go:41-49`, doc comment: "Feature management
   subcommands (`deps`, `claim`, `patch`)"). `unapply` adds a working-tree projection
   operation to this group — consistent with its noun scope.

3. The alternative (`tpatch unapply <slug>` as a top-level verb) would put a working-tree
   mutation at the same level as major lifecycle transitions (`apply`, `reconcile`, `land`).
   The lifecycle verbs advance a feature's state through its primary phases; `unapply` backs
   out a projection that `apply` wrote without deleting the feature or advancing the design
   arc. The asymmetric placement communicates this distinction.

**Why `reject` / `reopen` are NOT under `feature`:**

4. As documented in ADR-031 D10 Alternative 3 (chosen) and PRD-rejected-feature-state §4.1,
   `reject` and `reopen` are first-class lifecycle-state transitions — terminal outcome and
   its reversal. Every other lifecycle-state transition in tpatch (`analyze`, `define`,
   `explore`, `implement`, `apply`, `reconcile`, `land`) is a top-level verb
   (`internal/cli/cobra.go:60-80`, `buildRootCmd`). Placing `reject`/`reopen` under `feature`
   would contradict that convention and import a lifecycle verb into a noun-scoped group.

**Disambiguating cross-references in `--help`:**

Both `tpatch feature unapply --help` and `tpatch apply --help` should cross-reference the
other command for discoverability. Golden cross-reference strings (Cluster F' test-27
precedent — analogous `--help` cross-reference assertions are expected in the implementation
test matrix):

- `tpatch feature unapply --help` footer: `"Use 'tpatch apply <slug>' to reapply a feature that has been unapplied."`
- `tpatch apply --help` footer: `"Use 'tpatch feature unapply <slug>' to remove a feature's patch from the working tree."`
- `tpatch reject --help` footer is governed by ADR-031 D10; it is NOT expected to cross-reference `feature unapply` because the two commands operate on non-overlapping lifecycle domains (pre-implementation vs post-implementation working-tree management).

## 4. Modes

### 4.1 Patch mode (v1 only)

V1 has one mode: reverse-apply `artifacts/post-apply.patch`.

Rules:

- Use strict `git apply --reverse --check` before mutation.
- Do not use `--3way` by default; a 3-way unapply can silently remove or merge
  unrelated edits.
- If the check fails, print the refusal diagnostic and leave the tree unchanged.
- If the check passes, preview in a temporary worktree, snapshot touched files,
  and then run strict `git apply --reverse` in the real tree.
- After mutation, write audit artifacts under `artifacts/unapply/<attempt-id>/`.

Rationale: the canonical patch is tpatch's replay authority. Removing it from
the worktree should be symmetric with replaying it, but stricter because deletion
is risky.

## 5. Dependency Safety

Unapply of a feature with dependents is dangerous: children may rely on files or
symbols the parent introduced.

V1 policy:

- If any child depends on `<slug>`, refuse by default.
- The refusal lists child slugs and dependency kinds.
- Hard dependents always refuse in v1.
- Soft dependents also refuse by default.
- `--allow-soft-dependents` may bypass only soft dependent blockers.
- No `--cascade` unapply in v1. Cascading unapply of children is a follow-up PRD
  because it must define reverse-topological mutation, partial failure recovery,
  and child status transitions.

Soft dependents normally do not gate apply, but unapply is destructive: it
removes code that a soft dependent may opportunistically use. The safer default
is therefore to refuse and require an explicit soft-only override.

This intentionally differs from `remove --cascade`: unapply mutates source files,
so reverse-topological deletion semantics are not enough.

### 5.1 Rule 7 interaction: dependency edges onto an `unapplied` parent

Rule 7 (`ErrRejectedParent`, `internal/store/validation.go:57-70`) currently refuses to
create any dependency edge (hard/soft/supersedes) onto a `rejected` parent. This PRD
adds a parallel question: what should happen when a dependency edge points at an `unapplied`
parent?

**Decision (locked by ADR-032 D2):**

- Creating a new dependency edge onto an `unapplied` parent is **allowed** in v1. The parent
  exists, has a canonical patch, and can be reapplied. The `unapplied` state is reversible
  and transient (unlike `rejected`, which requires an explicit `tpatch reopen`). A hard edge
  onto an unapplied parent is already semantically meaningful: the child cannot apply until
  the parent is applied.

- Attempting to **unapply a feature that has live hard dependents** is **refused** by default
  in v1 (see §5 main policy). This is the safe direction: a child that depended on the
  parent's files should not silently lose those files.

- Attempting to **unapply a feature that has live soft dependents** is refused by default but
  can be bypassed with `--allow-soft-dependents`.

**Symmetric-invariant statement (analog of Cluster F's Rule 7 symmetric invariant):**

> If feature A has a hard dependency on feature B, and B is in state `unapplied`, then A
> cannot be in state `applied` (the dependency gate prevents applying A when its hard parent
> is not `applied`/`active`/`upstream_merged`). Conversely, unapplying B when A is applied
> is refused unless `--allow-soft-dependents` (for soft edges) or there are no hard
> dependents. The two refusals together mean the set of hard-dependency-satisfied features
> is always closed under `unapplied` parents — no applied child can have an unapplied hard
> parent.

This invariant is weaker than Rule 7 (which simply prohibits edges onto a `rejected` parent
entirely), because `unapplied` is reversible. The ADR must confirm whether to add a
validation-time warning when a live hard edge points at an `unapplied` parent to make the
broken state visible at `tpatch status` time.

## 6. Working Tree Safety

Unapply should use the same conservative philosophy as reconcile preflight:
refuse rather than guess.

Before mutating, v1 refuses when:

1. `git status --porcelain` is non-empty.
2. tracked files contain conflict markers.
3. `*.orig` or `*.rej` leftovers exist.
4. the repository is mid-merge, mid-rebase, or mid-cherry-pick.
5. canonical patch is missing or empty.
6. reverse-apply check fails.
7. temporary-worktree preview cannot prove the reverse apply is clean.

Dirty tree refusal is strict even when dirty paths do not overlap the feature. A
future PRD may allow cleanly disjoint dirt after a robust path-set proof, but v1
should not.

## 7. Patch Generation / Audit Artifacts

### 7.1 Audit artifacts

Successful attempts write an attempt directory:

```text
.tpatch/features/<slug>/artifacts/unapply/<attempt-id>/
├── unapply-session.json
└── reverse.patch
```

The reverse patch is deliberately not written under `patches/`. Today
`patches/NNN-*.patch` means historical full feature diff snapshots. A reverse
patch is an operation audit artifact, not a full feature snapshot.

Example `unapply-session.json`:

```json
{
  "version": 1,
  "feature": "auth-timeout",
  "attempt_id": "ua_9cb6578d11c8",
  "mode": "patch",
  "previous_state": "applied",
  "result": "unapplied",
  "canonical_patch_sha256": "64hex...",
  "reverse_patch": "artifacts/unapply/ua_9cb6578d11c8/reverse.patch",
  "touched_paths": ["src/auth.go"],
  "dependency_blockers": [],
  "preflight": {
    "clean_tree": true,
    "conflict_markers": [],
    "leftovers": []
  }
}
```

Refused attempts do not write `unapply-session.json` in v1. A future PRD may add
an explicit `--write-refusal-audit` flag if operators want persistent refusal
logs. This keeps "refused before mutation" commands from dirtying `.tpatch/`.

The session artifact may include wall-clock `attempted_at` if the ADR accepts the
same audit-session timestamp precedent used elsewhere; deterministic fields must
remain stable-sorted.

### 7.2 Patch generations

V1 does **not** append a patch generation for unapply.

Rationale:

- the feature's canonical patch bytes are unchanged;
- patch generations are about canonical patch identity;
- unapply is a working-tree projection event.

Do not overload `manual-metadata` as an unapply run without an ADR.

## 8. Status Semantics

V1 adds a new lifecycle state:

```json
"state": "unapplied"
```

Meaning: the feature remains tracked and has a canonical patch, but tpatch has
successfully removed that patch from the current working tree.

Rules:

- `post-apply.patch` remains unchanged.
- `apply.has_patch` remains true.
- `last_command` becomes `feature unapply`.
- `notes` names the unapply audit session.
- `Verify` is cleared, matching existing patch/recipe drift invalidation behavior
  where the producer clears stale verify state; the next status render should
  derive `never-verified`.
- Hard dependencies are not satisfied by an `unapplied` parent.
- Default `tpatch reconcile` skips `unapplied` features.
- Explicit `tpatch reconcile <slug>` on an unapplied feature is allowed.
- Existing `tpatch apply <slug>` is the reapply path from `unapplied`.

Why not leave `state: applied`: dependency gates would incorrectly treat an
unapplied parent as present. Why not reuse `defined`: the feature has a recorded
canonical patch and audit history, so rolling back to pre-patch lifecycle state
would lose status truth.

### 8.1 Why a lifecycle state, not an apply overlay?

Alternative:

```json
{
  "state": "applied",
  "apply": {
    "has_patch": true,
    "present_in_worktree": false
  }
}
```

Rejected for v1:

- It creates a second truth source: state says "applied" while apply metadata says
  "not present."
- Dependency gates still need to learn the overlay, so the integration burden is
  not actually avoided.
- Status text becomes ambiguous unless every renderer composes state plus overlay.
- Existing state-driven command selection would remain misleading until audited.

Chosen model: `state: "unapplied"` is explicit, user-visible, and keeps lifecycle
truth in one field. The ADR must lock this decision before implementation.

### 8.2 Composition with `rejected` state (v0.13.0 compatibility)

This subsection answers the composition question deferred by ADR-031 D6 ("post-implementation
retirement is explicitly out of scope for Cluster F; deferred to future ADR, potentially
PRD-feature-unapply.md").

**Three options:**

**(A) Parallel independent states** — `unapplied` and `rejected` are both valid
`FeatureState` values but operate on non-overlapping lifecycle segments. A feature can hold
at most one of them at any point. Transition to `unapplied` is only legal from states where
the feature has an applied patch (i.e., from `applied`/`active`/`reconciling`/
`reconciling-shadow`). Transition to `rejected` is only legal from `requested`/`analyzed`/
`defined` (the pre-implementation states). Because the entry conditions are disjoint, no
feature can reach both terminal states without the state machine explicitly allowing a path
between them, which neither ADR-031 nor this PRD proposes.

**(B) Hierarchical** — `rejected` implies `unapplied` first; if a future PRD ever adds
post-implementation rejection, an operator would run `feature unapply` before `reject`.
Or: `unapplied` is treated as a transient station in a compound
`applied → unapplied → rejected → reopened → applied` arc.

**(C) Union** — a single combined lifecycle path where `unapplied` and `rejected` are
stations in the same terminal arc, and commands compose (e.g., a future `reject --from-applied`
flag that internally unapplies then rejects).

**Chosen: Alternative A — parallel independent states.**

Rationale:

1. **The state machines are already disjoint.** `RejectableStates = {requested, analyzed,
   defined}` (`internal/store/status.go:108-118`). A feature cannot be both `applied`
   (required to reach `unapplied`) and one of the pre-implementation states (required to
   reach `rejected`). The boundary is mechanical, not merely conceptual.

2. **No shared data model is needed.** `RejectionStatus` and `RejectionHistoryEntry` carry
   pre-implementation evidence, operator notes, and actor. An `unapply-session.json` carries
   patch-reversal audit and working-tree snapshot info. These are orthogonal schemas; forcing
   one into the other produces a bloated composite record.

3. **ADR-031 D6's language is explicit.** The deferral says "post-implementation *retirement*
   command" — not "post-implementation *rejection* command." `feature unapply` is a
   reversible working-tree operation, not a terminal verdict. Treating it as a station toward
   `rejected` misrepresents its semantics.

4. **Minimizes future debt.** Hierarchical (B) and union (C) both require a future
   implementation to maintain the `unapplied → rejected` path AND the pre-implementation
   `rejected` path as separate entry-condition branches on `tpatch reject`. This adds
   complexity to a command that is already well-specified for its narrower pre-implementation
   scope. Parallel states allow both commands to remain self-contained.

**Consequence:** A feature cannot be directly transitioned from `unapplied` to `rejected`.
If an operator has unapplied a feature and later decides it should be permanently
discarded, they must run `tpatch remove <slug>` (or a future `tpatch retire` command if one
is ever specified). `tpatch reject` will refuse `unapplied` as a source state with exit
code 3, just as it refuses `applied`, `active`, and every other post-implementation state.

This decision is locked by ADR-032 D7. The source-of-deferral citation is ADR-031 D6
(`docs/adrs/ADR-031-rejected-feature-state-data-model.md:539-630`).

### 8.3 Status command filtering for `unapplied` features

v0.13.0 established the pattern of **opt-out filtering** for terminal-ish states:
`rejected` features are hidden from `tpatch status` by default and shown with
`--include-rejected` (`internal/cli/cobra.go:250,397,529`). `FEATURES.md` renders a
`## Rejected` section trailing after the active feature table.

This PRD proposes a **parallel treatment** for `unapplied` features:

**Should `status` hide `unapplied` by default?**

`unapplied` is not a terminal state — it is recoverable with `tpatch apply <slug>`. A
feature that is unapplied is still tracked, still has a canonical patch, and may be actively
worked on in a different branch context. Hiding it by default would reduce discoverability
and make it easy to forget that the feature exists. **Decision: `unapplied` features are
included in the default `tpatch status` listing** (not hidden). They should render with a
distinct `[unapplied]` badge to distinguish them from `[applied]` or `[active]` features.

**`--include-unapplied` flag: not needed.**

Because `unapplied` features are shown by default, no `--include-unapplied` flag is
introduced in v1. This avoids flag proliferation. A future PRD may introduce `--hide-unapplied`
if operator feedback shows that repositories with many unapplied features need a way to
suppress them.

**`next` action for `unapplied` features:**

`tpatch next` should recommend `tpatch apply <slug>` as the next action for an `unapplied`
feature. It should not recommend `tpatch feature unapply` as a next action from any other
state.

**`FEATURES.md` rendering:**

`unapplied` features should appear in a `## Unapplied` table in `FEATURES.md`, placed
after the active/applied tables but before the `## Rejected` table (if present). The
ordering signals that unapplied is an active concern (not retired) while rejected is a
terminal outcome. The `## Unapplied` section header and table schema follow the same
format as other state-specific tables in `FEATURES.md`.

## 9. Lifecycle Integration

Implementation must update every state-aware surface in one slice.

| Surface | Required behavior |
|---|---|
| Feature state enum | Add `StateUnapplied = "unapplied"` and include it in state validation. |
| `tpatch status` text | Render unapplied features as `[unapplied]`; include notes/audit session in verbose detail. |
| `tpatch status --json` | Emit `state: "unapplied"`; no extra overlay is required for clients to detect it. |
| Aggregate status filtering | Include unapplied features in normal status lists because they remain tracked features. |
| `tpatch apply <slug>` | Allow apply from `unapplied` as the v1 reapply path, subject to existing dependency and parent-generation stale gates. On success, state returns to `applied`. |
| `tpatch record <slug>` | Preserve existing empty-capture refusal; do not let a clean tree after unapply overwrite the canonical patch with empty bytes. |
| `tpatch reconcile` aggregate | Skip unapplied features in the default all-feature sweep. |
| `tpatch reconcile <slug>` explicit | Allow explicit reconcile; use the canonical patch and do not infer upstreamed merely because the patch is absent from the working tree. |
| Dependency gates | `unapplied` does not satisfy hard dependents. Children should behave like the parent is not applied. |
| Soft dependents | Warn/refuse on unapply by default; allow bypass only through `--allow-soft-dependents`. |
| Verify/freshness | Clear `Verify` on successful unapply; status should derive `never-verified` unless a future freshness label models unapply. |
| `tpatch remove` / cascade | Existing deletion behavior is unchanged; removing an unapplied feature follows the same dependency rules as any other feature. |
| `tpatch land` | Refuse unapplied features because there are no feature source edits to commit; suggest `tpatch apply <slug>` first. |
| **v0.13.0 surfaces (§8.2 composition)** | |
| `tpatch reject` | Refuse `unapplied` as a source state (exit code 3). `unapplied` is a post-implementation state; `RejectableStates` is `{requested, analyzed, defined}`. No escape-hatch flag. Consistent with ADR-031 D6 (no post-implementation reject). |
| `tpatch reopen` | Not applicable to `unapplied`. `reopen` is the inverse of `reject`; `unapplied` features are not in `state: rejected`. If an unapplied feature should be discarded entirely, the operator must `tpatch remove`. |
| `tpatch feature deps add` / dependency gate | Dependency edges onto an `unapplied` parent are **allowed** (§5.1). Adding a hard edge onto a currently-unapplied parent is legal; the apply-time gate will refuse the child until the parent is reapplied. No Rule-7-analog validation-write refusal is introduced for `unapplied` parents in v1. |
| `tpatch status --include-rejected` | `--include-rejected` does not affect `unapplied` features (§8.3). They appear in the default listing regardless. |
| Docs / skills | Update lifecycle docs and shipped skill assets if they enumerate feature states or recommend `record/apply/reconcile` flows. Parity guard must pass when assets change. |
| Tests | Cover state validation, status text/JSON, apply from unapplied, aggregate reconcile skip, explicit reconcile behavior, dependency unsatisfied behavior, verify clearing, remove unchanged, and land refusal. Also: reject-refused-from-unapplied (exit 3), `--include-rejected` does-not-hide-unapplied, `next` recommends apply. |

## 10. Failure Atomicity

V1 must provide a concrete rollback strategy:

1. Resolve touched paths from the canonical patch.
2. Run strict reverse-apply check in the real tree.
3. Run the same reverse-apply in a temporary worktree at the current `HEAD`.
4. Snapshot touched files from the real working tree, including missing-file
   markers and file modes where possible.
5. Run strict `git apply --reverse` in the real tree.
6. If real apply fails, restore touched files from the snapshot, restore missing
   files to missing, and report failure.
7. Write `.tpatch/` audit artifacts and update `status.json` only after source
   mutation succeeds.

The preflight clean-tree requirement makes races unlikely, but check+apply is not
atomic across filesystem changes. Snapshot/restore closes the remaining gap.

## 11. Interactions

### 11.1 `record`

`record` after unapply is allowed but should be cautious:

- default `record <slug>` on a clean tree after unapply should capture 0 bytes and
  hit existing empty-capture refusal;
- to intentionally retire a feature, users must use a separate retirement/removal
  path, not accidental record;
- re-recording after reapplying the feature follows existing record/refresh
  semantics.

### 11.2 `reconcile`

Unapplied features remain reconcile-eligible when explicitly named:

```bash
tpatch reconcile auth-timeout
```

Default `tpatch reconcile` should not include them in the all-applied/active
sweep because their patch is not currently present in the worktree.

If an unapplied feature is reconciled explicitly, reconcile should use the
canonical patch and may report whether it can be forward-applied to the current
upstream. It must not infer that absence from the working tree means upstreamed.

### 11.3 `land`

`land` should refuse `unapplied` features by default because there are no feature
source edits to commit. It may suggest:

```bash
tpatch apply <slug>
tpatch land <slug>
```

V1 unapply does not commit.

### 11.4 Dependencies

Hard dependents block unapply. Soft dependents block unless
`--allow-soft-dependents` is passed.

An unapplied hard parent does not satisfy child apply/reconcile gates. Status/DAG
rendering should use existing waiting-on-parent semantics unless the ADR adds a
more specific label.

### 11.5 Patch generations

Unapply reads the latest canonical patch and patch-generation metadata for
diagnostics, but it does not write a new generation. The current generation
remains the patch to reapply later.

### 11.6 `remove` / cascade

`remove` remains deletion. It should not grow `--revert`.

If a user wants to unapply and then delete metadata, they must run two explicit
commands:

```bash
tpatch feature unapply <slug>
tpatch remove <slug>
```

This makes both source mutation and metadata deletion separately reviewable.

### 11.7 Interactions with `reject` / `reopen` (v0.13.0)

This subsection documents the composition boundary locked in §8.2 from the command-
interaction perspective.

**`tpatch reject` applied to an `unapplied` feature:**

Refused (exit code 3). `unapplied` is a post-implementation state. `RejectableStates`
is `{requested, analyzed, defined}` (`internal/store/status.go:108-118`); `tpatch reject`
refuses any source state outside this set with the standard state-machine error. No
`--force` flag bypasses this: ADR-031 D6 explicitly closes the post-implementation escape
hatch. Operators who want to permanently retire an unapplied feature should run
`tpatch remove <slug>` (or a future `tpatch retire` if one is specified).

**`tpatch reopen` applied to a `rejected` feature that was previously `unapplied`:**

This case does not arise under §8.2's composition model because a feature cannot be in
state `rejected` if it was ever in state `unapplied`. The entry conditions are disjoint
(see §8.2). If state.json is hand-edited to produce this impossible combination, `tpatch
reopen` should behave exactly as it does for any `rejected` feature: return the feature to
its `PriorState` (the state recorded in `RejectionStatus.PriorState` at reject time). If
`PriorState` is somehow `unapplied` in a corrupted record, the reopen should fail with
exit code 1 (internal error) rather than silently setting an unknown prior state, to avoid
leaving the feature in an unrecoverable configuration.

**`confirm-upstreamed` gate on `unapplied` state:**

`tpatch reconcile confirm-upstreamed` (`internal/cli/cobra.go:2535`) is guarded against
`rejected` features by the ADR-031 D6 defense-in-depth guard. Parallel reasoning applies
to `unapplied`: a feature whose patch is not present in the working tree cannot be
confirmed as upstreamed by examining the working tree. The `confirm-upstreamed` gate should
refuse `unapplied` source state with exit code 3, printing a message suggesting the
operator run `tpatch apply <slug>` first to restore the patch before confirming it is
upstream. The exact guard implementation is deferred to the Cluster G' implementation slice
alongside the defense-in-depth pattern from ADR-031 D6 (`internal/cli/cobra.go:2525-2540`).

## 12. Deferred: landed-commit unapply

`--mode landed-commit` is deferred to a follow-up PRD.

Reasons:

- it requires Git trailer lookup;
- it must match `Tpatch-Patch-SHA` against current canonical bytes;
- it must extract non-tpatch file changes from a commit that likely also touched
  `.tpatch/`;
- it must preserve `.tpatch/` metadata while applying a reverse source patch;
- it must clearly distinguish patch reverse-apply from raw `git revert`.

V1 patch mode is useful without solving this second feature.

## 13. Backwards Compatibility

No existing behavior changes unless `tpatch feature unapply` is invoked.

Existing feature directories without patch generations still work in patch mode
as long as `artifacts/post-apply.patch` exists.

Repositories that do not use the new command see no schema changes except that
future tpatch binaries understand `state: "unapplied"` after the command has
been used.

## 14. Implementation Notes

- Put the command under `featureCmd()` as `feature unapply`.
- Use `--allow-soft-dependents`, not `--force-dependents`.
- Reuse `gitutil.PreflightReconcile` or a sibling preflight with mid-Git-operation
  checks.
- Use `gitutil.FilesInPatch` to enumerate touched paths for dry-run, snapshots,
  and audit.
- Use strict `git apply --reverse --check` before mutating and strict
  `git apply --reverse` for mutation.
- Implement temporary-worktree preview plus touched-file snapshot/restore.
- Write `unapply-session.json` and `reverse.patch` only after successful source
  mutation.
- Do not write under `patches/`.
- Clear `Verify` on successful unapply.
- Update state validation, status rendering, dependency satisfaction, reconcile
  selection, land refusal, docs/skills, and tests in the same implementation
  slice.
- Do not add provider calls.

## 15. Acceptance Criteria

1. `tpatch feature unapply <slug> --dry-run` reports exact touched files,
   dependency blockers, preflight status, reverse-apply viability, and planned
   audit artifact paths without mutating the worktree, index, or `.tpatch/`.
2. Dirty-tree dry-run exits 0 and reports dirty-tree blockers.
3. Invalid slug, missing feature, unreadable status, or missing canonical patch
   dry-run exits non-zero.
4. Clean patch-mode unapply reverse-applies `artifacts/post-apply.patch` and
   removes the feature changes from the working tree.
5. Clean unapply writes
   `artifacts/unapply/<attempt-id>/unapply-session.json`.
6. Clean unapply writes `artifacts/unapply/<attempt-id>/reverse.patch`.
7. No unapply artifact is written under `patches/`.
8. The feature directory remains present.
9. `artifacts/post-apply.patch` remains byte-identical after unapply.
10. `patch-generations.json` remains unchanged after unapply.
11. `status.json` records `state: "unapplied"`, `last_command:
    "feature unapply"`, and a note referencing the audit session.
12. `tpatch status` text renders unapplied features visibly.
13. `tpatch status --json` emits `state: "unapplied"`.
14. `tpatch apply <slug>` works as the v1 reapply path from `unapplied` and
    returns state to `applied` on success.
15. Aggregate `tpatch reconcile` skips unapplied features.
16. Explicit `tpatch reconcile <slug>` on an unapplied feature is allowed and
    does not infer upstreamed merely from absence.
17. Hard child dependents refuse by default and cannot be bypassed in v1.
18. Soft child dependents refuse by default and can be bypassed only with
    `--allow-soft-dependents`.
19. An unapplied parent does not satisfy hard dependency gates.
20. Successful unapply clears `Verify` according to existing invalidation style.
21. Dirty working tree refuses before mutation.
22. Merge conflict markers refuse before mutation.
23. `.orig` / `.rej` leftovers refuse before mutation.
24. Mid-merge, mid-rebase, and mid-cherry-pick states refuse before mutation.
25. Reverse-apply check failure leaves the worktree, index, feature metadata, and
    status unchanged.
26. If real reverse-apply fails after preview/check, touched files are restored
    from the snapshot and `.tpatch/` audit/status writes do not occur.
27. No partial reverse-apply success is reported as success.
28. `tpatch land <slug>` refuses unapplied features and suggests reapply first.
29. `tpatch remove <slug>` behavior is unchanged.
30. Docs and shipped skill assets that enumerate states or lifecycle flows are
    updated; parity guard passes when assets change.
31. Existing features behave exactly as before unless the new command is invoked.

## 16. Open Questions

1. Should a future flag persist refused `unapply-session.json` entries for audit
   trails, or should refusals remain terminal diagnostics only?
2. Should landed-commit mode ship as `--mode landed-commit` or as a separate
   command once the follow-up PRD exists?
3. Should status/DAG rendering add a dedicated `unapplied-parent` label, or is
   existing waiting-on-parent presentation sufficient? (§5.1 notes the ADR should
   confirm whether a validation-time warning is needed for live hard edges onto
   `unapplied` parents.)
4. Should a future `feature reapply` command provide a symmetric UX over existing
   `apply`, or is `tpatch apply <slug>` enough?
5. (Resolved in §8.2) How do `unapplied` and `rejected` compose? → **Parallel
   independent states.** No new question remains open on this topic; ADR-032 D7 locks
   the decision.

## 17. Disputes

- **Lifecycle state**: This PRD recommends a new `unapplied` state. The overlay
  alternative is rejected in §8.1, but the ADR must lock the final boundary.
- **Soft-dependent refusal**: V1 treats destructive unapply more strictly than
  apply ordering. Reviewers may prefer warning-only for soft dependents.
- **Refused audit sessions**: V1 omits persistent refusal sessions to keep refused
  commands side-effect-free. Operators may still want opt-in refusal audit later.
- **Composition with `rejected` (Resolved)**: §8.2 adopts Alternative A (parallel
  independent states). `unapplied` is a post-implementation reversible state;
  `rejected` is a pre-implementation terminal state. They do not overlap by state
  machine construction (`RejectableStates` = {requested, analyzed, defined};
  `unapply` entry requires a feature that is applied/active). No further dispute
  remains on this topic; the ADR must cross-reference ADR-031 D6 as the deferral
  source. Reviewers who prefer Alternative B (hierarchical) or C (union) should
  record their disagreement in the ADR-032 review; this PRD treats the matter
  resolved subject to ADR acceptance.
- **`confirm-upstreamed` guard on `unapplied`**: §11.7 proposes a guard analogous
  to ADR-031 D6's `rejected` defense-in-depth guard. The exact implementation site
  and error message are deferred to Cluster G'. Reviewers may dispute whether the
  guard is needed for `unapplied` (vs. `rejected`) given that `unapplied` is
  reversible without an operator `reopen` step.

