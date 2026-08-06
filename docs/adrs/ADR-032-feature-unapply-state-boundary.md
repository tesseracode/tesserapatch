# ADR-032 — Feature Unapply State Boundary

**Status**: Proposed
**Date**: 2026-08-05 (Cluster G planning)
**Author**: Cluster G planning agent
**Related**:
- [SPEC.md](../../SPEC.md) — feature lifecycle state table
- [docs/feature-layout.md](../feature-layout.md) — artifact layout
- [docs/dependencies.md](../dependencies.md) — hard/soft/supersedes edge semantics
- [docs/adrs/ADR-011-feature-dependencies.md](ADR-011-feature-dependencies.md) — dependency gate
- [docs/adrs/ADR-024-patch-generation-manifest-boundary.md](ADR-024-patch-generation-manifest-boundary.md) — no patch-gen writes boundary
- [docs/adrs/ADR-031-rejected-feature-state-data-model.md](ADR-031-rejected-feature-state-data-model.md) — D6 deferral source; D10 naming precedent
- [docs/prds/PRD-feature-unapply.md](../prds/PRD-feature-unapply.md) — paper design this ADR gates
- [docs/prds/PRD-feature-dependencies.md](../prds/PRD-feature-dependencies.md)
- [docs/prds/PRD-feature-patch-identity-metadata.md](../prds/PRD-feature-patch-identity-metadata.md)
- [docs/prds/PRD-tpatch-land.md](../prds/PRD-tpatch-land.md)

---

## Purpose

This ADR locks the implementation-blocking design decisions for `tpatch feature unapply`
before any code lands. The PRD (PRD-feature-unapply.md) can be reviewed as paper design
before this ADR is accepted; implementation must not start until this ADR reaches
**Accepted** status via the standard review workflow.

The eight decision points (D1–D8) cover the state model, dependency semantics, audit
artifact schema, scope constraints, failure invariants, and two cross-cutting concerns
added by the Cluster G refresh: composition with the shipped `rejected` state (D7,
deferred from ADR-031 D6) and command-name pattern (D8).

---

## 0. Claims Audit

| Claim | Evidence |
|---|---|
| `StateRejected` is the eleventh `FeatureState`; `ValidFeatureState` closed switch includes all 11 values. | `internal/store/types.go:21-30` |
| `RejectableStates` restricts `tpatch reject` to `{requested, analyzed, defined}`; all post-implementation states are refused at exit code 3 per ADR-031 D6. | `internal/store/status.go:108-118` |
| `RejectionStatus` (live record) and `RejectionHistoryEntry` (completed-cycle audit) are the only new schema objects introduced by v0.13.0. No shared schema with unapply artifact is required. | `internal/store/status.go:59-106` |
| ADR-031 D6 explicitly deferred post-implementation retirement command, naming `PRD-feature-unapply.md` as potential host. | `docs/adrs/ADR-031-rejected-feature-state-data-model.md:604-630` |
| ADR-031 D10 locked top-level verbs for lifecycle state transitions (`reject`/`reopen`); `tpatch feature` group reserved for noun-scoped per-feature management (`deps`, `claim`, `patch`). | `docs/adrs/ADR-031-rejected-feature-state-data-model.md:889-974`, `internal/cli/feature_deps.go:41-49` |
| Patch generations (ADR-024) deliberately do not include working-tree projection events (record of canonical patch bytes only). | `docs/adrs/ADR-024-patch-generation-manifest-boundary.md:48-75` |
| `gitutil.PreflightReconcile` and reverse-apply check infrastructure already exist. | `internal/gitutil/gitutil.go:396-435`, `internal/gitutil/gitutil.go:85-187` |
| `tpatch status --include-rejected` pattern (opt-in flag to show terminal-ish hidden state) is established. | `internal/cli/cobra.go:250,397,529` |
| `confirm-upstreamed` transition is guarded against `rejected` source state at entry (ADR-031 D6 defense-in-depth). | `internal/cli/cobra.go:2525-2540` |
| `FeatureStatus.Verify` is cleared on certain state transitions (e.g., patch drift invalidation). Unapply should parallel this. | `internal/store/types.go` (Verify field), `internal/cli/cobra.go` (verify-clearing call sites) |

---

## D1: `unapplied` as a real `FeatureState`

### Question

Should `unapplied` be implemented as a real `FeatureState` constant, or as an overlay
(a boolean field on the apply sub-record while state remains `"applied"`)?

### Alternatives

**Alternative 1 (Overlay)**: Keep `state: "applied"` and add
`apply.present_in_worktree: false` when the patch is removed.

Pros:
- No new state added to the closed `ValidFeatureState` switch.
- Existing callers that check `state == StateApplied` continue to work.

Cons:
- Two truth sources: state says "applied" while apply metadata says "not present."
  Dependency gates, reconcile, land, and status must all compose state + overlay — the
  integration burden is not avoided, only relocated.
- Ambiguous status text: every renderer must compose state + overlay to produce accurate
  output. A client that reads `state: "applied"` and stops is silently wrong.
- `tpatch next` and aggregate reconcile sweep can no longer use state alone as a routing
  key; both require special-cased overlay inspection.
- Future clients (CI scripts, other agents) cannot distinguish `applied` from
  `applied-but-not-in-tree` without reading an optional metadata field.

**Alternative 2 (Derived/computed)**: Expose `unapplied` only in `tpatch status` output
as a derived label (like `ReconcileLabel`), computed from a combination of state and
artifact presence checks. Not persisted.

Pros:
- No schema change.

Cons:
- Requires reading canonical patch bytes and running a reverse-apply check on every
  `status` invocation to determine derivation — expensive and non-deterministic in a dirty
  tree.
- Cannot be used by dependency gates at all (gates are called during `apply`, not at
  status time).
- Breaks the invariant that `status.json` is the authoritative source of lifecycle truth.

**Alternative 3 (Real `FeatureState` — PRD's recommendation)**: Add
`StateUnapplied FeatureState = "unapplied"` as the 12th value; extend
`ValidFeatureState` closed switch; use it as the authoritative lifecycle state.

Pros:
- Single source of truth: `status.json:state` alone determines all gate decisions.
- Dependency gates, reconcile selection, land refusal, and next-action routing are all
  driven by the same field without overlay composition.
- Explicit, user-visible, and unambiguous in `tpatch status --json` output for all
  downstream consumers.
- Matches the precedent of every other lifecycle outcome in the codebase (blocked,
  upstream_merged, rejected — all real states).

Cons:
- One more value in the closed switch; all `switch state { }` exhaustiveness checks must
  include it. This is a good forcing function: every state-aware surface is audited at
  compile time.

### Decision: **Alternative 3**

`unapplied` is a real `FeatureState`. Implementation adds `StateUnapplied = "unapplied"`
as the 12th value and extends `ValidFeatureState`. Every `switch state { ... }` in
`internal/store/types.go`, `internal/cli/cobra.go`, and `internal/workflow/` that handles
`StateApplied` or `StateActive` must explicitly handle `StateUnapplied` or document the
omission with a code comment.

### Consequences

Positive: single truth source; dependency gates, reconcile, land, status, and next are all
driven by the same field.

Negative: implementation must audit every state-aware switch. Tooling that hard-codes the
list of 11 states in v0.13.0 must be updated to 12.

---

## D2: Which states satisfy hard dependencies after unapply

### Question

Currently `applied`, `active`, and `upstream_merged` satisfy a hard dependency edge.
After a parent feature is unapplied (`state: "unapplied"`), does it still satisfy its
children's hard dependency gates?

### Alternatives

**Alternative 1 (Unapplied satisfies)**: Treat `unapplied` as a variant of `applied` for
dependency satisfaction purposes. A child can apply when its hard parent is `unapplied`.

Pros: simpler gate — one fewer special case.

Cons: directly contradicts the semantic of `unapplied`. An unapplied parent means its code
is *not* present in the working tree. A child that depends on that code cannot safely apply.
If the child's apply runs on a tree without the parent's files, it will likely fail or
produce incorrect results. The dependency gate exists to prevent exactly this error.

**Alternative 2 (Unapplied does NOT satisfy — PRD's recommendation)**: `unapplied` is not
in the set of dependency-satisfying states. Children whose hard parents are `unapplied`
cannot apply; they receive a `waiting-on-parent` label (or a new `parent-unapplied` label
if D2 chooses to add one).

Pros: correct. The parent's code is absent. A child that needs that code cannot proceed.
Mirrors the `rejected` treatment (rejected parents do not satisfy; in fact, rejected
parents refuse edge-creation entirely via Rule 7).

Cons: a child that was successfully applied and whose parent is then unapplied enters a
broken state (applied child, unapplied parent). This is detectable and should be surfaced
as a DAG warning on `tpatch status`.

**Alternative 3 (Satisfies for the child's already-applied state only)**: Once the child
is applied, subsequent unapply of the parent does not retroactively break the child's
status. The gate applies only at new-apply time.

Pros: avoids the broken-state scenario.

Cons: hides dangerous state; the child's code depends on files that no longer exist in
the tree. This is the worst outcome — a silent lie.

### Decision: **Alternative 2**

`unapplied` does NOT satisfy hard dependency edges. The set of satisfying states remains
`{applied, active, upstream_merged}`. This is the correct semantic: the parent's files are
absent from the working tree.

**Broken-state handling (implementation note)**: when `tpatch status` or DAG validation
detects that a feature is in state `applied`/`active` but has a hard parent in state
`unapplied`, it should emit a DAG warning similar to the broken-parent label introduced in
ADR-011. The exact label name (`parent-unapplied` or reuse of `blocked-by-parent`) is an
implementation decision deferred to Cluster G'.

**Unapply refusal when live hard dependents exist (see PRD §5)**: remains. The gate prevents
the most common path to the broken state. DAG warnings handle the race (parent unapplied
while child was applied independently in a complex concurrent workflow).

### Consequences

Positive: dependency semantics are consistent and unambiguous. Unapply of a parent is
visibly blocking to children.

Negative: it is possible to reach a state where a child is `applied` but its hard parent
is `unapplied`. This is detectable and surfaced as a DAG warning; it is the operator's
responsibility to resolve it (either reapply the parent or unapply the child).

---

## D3: Unapply audit artifact schema (`unapply-session.json`)

### Question

What is the exact wire schema for `unapply-session.json`? Lessons from Cluster F' rev-0
(F-INT-1 BLOCKING wire-schema) require the schema to be locked byte-for-byte before
implementation, not discovered during review.

### Alternatives

**Alternative 1 (Minimal schema)**: Only mandatory fields; optional fields omitted with
`omitempty`.

**Alternative 2 (Fixed-envelope schema — PRD §7.1 baseline)**: Full fixed-envelope with
all standard fields required; optional metadata added in a structured sub-object.

**Alternative 3 (Shared envelope with rejection schema)**: Reuse `RejectionStatus` field
names where they overlap (e.g., `actor`, `note`).

### Decision: **Alternative 2 — fixed-envelope schema**

The schema below is the **binding wire contract**. Implementation must produce this
byte-for-byte (stable-sorted keys within objects). The `version` field enables future
migrations.

```json
{
  "version": 1,
  "feature": "<slug>",
  "attempt_id": "<ua_hex12>",
  "mode": "patch",
  "attempted_at": "<RFC3339 timestamp>",
  "previous_state": "<FeatureState string>",
  "result": "unapplied",
  "actor": "<actor string>",
  "canonical_patch_sha256": "<64-hex lowercase>",
  "reverse_patch": "artifacts/unapply/<attempt-id>/reverse.patch",
  "touched_paths": ["<repo-relative path>", ...],
  "dependency_blockers": [],
  "preflight": {
    "clean_tree": true,
    "conflict_markers": [],
    "leftovers": []
  }
}
```

Field constraints:

| Field | Type | Required | Notes |
|---|---|---|---|
| `version` | int | yes | Always `1` for this schema version |
| `feature` | string | yes | Feature slug, as recorded in store |
| `attempt_id` | string | yes | `ua_` + 12 lowercase hex chars (crypto random) |
| `attempted_at` | string | yes | RFC3339, UTC, matching existing audit timestamp pattern |
| `mode` | string | yes | `"patch"` in v1; reserved for future `"landed-commit"` |
| `previous_state` | string | yes | `FeatureState` string before unapply |
| `result` | string | yes | `"unapplied"` on success; field not written on refusal (refused attempts do not write the file in v1) |
| `actor` | string | yes | Same precedence chain as `tpatch reject`: `--actor` > `TPATCH_ACTOR` > `git config user.email` > `"unknown"` |
| `canonical_patch_sha256` | string | yes | Lowercase hex SHA-256 of `post-apply.patch` bytes at attempt time |
| `reverse_patch` | string | yes | Relative path from feature dir root to `reverse.patch` |
| `touched_paths` | []string | yes | Sorted alphabetically; repo-relative forward-slash paths |
| `dependency_blockers` | []string | yes | Empty array `[]` when none; slugs of blocking dependents |
| `preflight` | object | yes | See sub-fields below |
| `preflight.clean_tree` | bool | yes | `true` when no `git status --porcelain` output; `false` is only possible on `--dry-run` reports |
| `preflight.conflict_markers` | []string | yes | Paths with conflict markers; empty array when none |
| `preflight.leftovers` | []string | yes | `.orig`/`.rej` file paths; empty array when none |

The `dependency_blockers` and `preflight.*` arrays use empty JSON arrays `[]`, never
`null`, for schema-stable round-tripping (Cluster F' F-INT-1 lesson: `null` vs `[]`
divergence caused schema-stability failures during review).

Alternative 3 (shared RejectionStatus fields) is rejected: `actor` is the only potential
overlap, and the two schemas serve orthogonal purposes. Coupling them would make future
independent evolution difficult.

### Consequences

Positive: exact schema is locked; Cluster G' implementer has zero ambiguity; reviewer can
write wire-schema assertions before seeing code.

Negative: any future extension requires a `"version": 2` migration path. This is the
same trade-off accepted for `RejectionHistoryEntry` in ADR-031 D3.

---

## D4: No patch-generation writes in v1

### Question

Should `tpatch feature unapply` append a new patch generation to `patch-generations.json`?

### Alternatives

**Alternative 1 (Write a generation)**: Record a new generation entry noting that the
canonical patch bytes are unchanged but the working-tree state changed.

Cons: ADR-024 locks patch generations to canonical patch byte changes. Adding a generation
for a working-tree operation overloads the schema with two semantically different event
types. This was explicitly excluded in PRD §7.2 and ADR-024.

**Alternative 2 (No generation write — PRD's recommendation)**: V1 does not write a patch
generation on unapply. The working-tree projection event is captured in
`unapply-session.json` exclusively.

Pros: clean boundary; patch generations remain exclusively about canonical patch identity.

**Alternative 3 (Generation with `event: "unapply"` type)**: Add a new generation type
`unapply` that records the state transition without canonical patch changes.

Cons: introduces a second meaning for "generation" — semantic overloading. Clients that
iterate generations to compute patch identity must now filter by type. ADR-024 D1 chose
content-hash identity for a reason; adding non-content-change entries breaks the
one-entry-one-change contract.

### Decision: **Alternative 2**

No patch-generation writes in v1. The boundary set by ADR-024 is preserved.

### Consequences

Positive: ADR-024 boundary intact; patch generation tooling is unaffected.

Negative: the unapply/reapply history is not visible in `tpatch feature patch <slug>` output.
This is acceptable in v1; a future PRD may add a separate `tpatch feature unapply history
<slug>` subcommand that reads `artifacts/unapply/` directory.

---

## D5: Patch-mode-only v1 scope

### Question

Should v1 implement only patch-mode unapply, or also landed-commit unapply?

### Alternatives

**Alternative 1 (Patch mode + landed-commit mode)**: Both modes in v1.

Cons: landed-commit unapply requires Git trailer lookup, `Tpatch-Patch-SHA` verification,
non-tpatch file extraction from commits, and disambiguation from raw `git revert`. PRD §12
enumerates 5 distinct blockers. The added complexity doubles the v1 scope.

**Alternative 2 (Patch mode only — PRD's recommendation)**: V1 is patch mode only.
Landed-commit mode is deferred to a follow-up PRD.

Pros: clean scope; each mode can be independently reviewed and specified.

**Alternative 3 (Landed-commit only)**: Impractical — most features are not landed.

### Decision: **Alternative 2**

V1 is patch mode only. `--mode landed-commit` is reserved as a flag name for the follow-up
PRD; if a user passes it in v1, the command should error with exit code 2 and a message
pointing to the documentation.

### Consequences

Positive: bounded implementation scope; clear follow-up task.

Negative: users who have landed features and want to unapply them must use `git revert`
directly in v1. The PRD §12 deferral list is the forward path.

---

## D6: Failure atomicity guarantees

### Question

What atomicity invariants must `tpatch feature unapply` provide, and what happens on
partial failure?

### Alternatives

**Alternative 1 (Best-effort)**: Apply the reverse patch and report success or partial
success. Do not restore on failure.

Cons: partial reverse-apply leaves the working tree in a corrupted state with no clear
path to recovery. This is unacceptable for a tool that claims to be safe.

**Alternative 2 (Check-before-mutate with snapshot/restore — PRD's recommendation)**:
Before any mutation: (a) run `git apply --reverse --check` in the real tree; (b) run the
same check in a temporary worktree; (c) snapshot touched files from the real tree.
Mutate only if checks pass. If real mutation fails, restore snapshots.

Pros: provides a concrete rollback path; no partial state is reported as success.

**Alternative 3 (Transaction via git stash)**: Stash the entire working tree, apply the
reverse patch on top, pop the stash, cherry-pick the reverse patch back.

Cons: stash-pop conflicts are possible and would produce a broken state worse than the
original. Stash semantics interact badly with untracked files.

### Decision: **Alternative 2**

The implementation must follow the 7-step protocol in PRD §10:

1. Resolve touched paths from canonical patch.
2. Run `git apply --reverse --check` in the real tree.
3. Run the same check in a temporary worktree at current HEAD.
4. Snapshot touched files from real tree (include missing-file markers and file modes).
5. Run `git apply --reverse` in the real tree.
6. If step 5 fails: restore snapshots, restore missing files to missing, and report failure.
7. Write `.tpatch/` audit artifacts and update `status.json` **only after** step 5 succeeds.

**Invariants:**

- If `unapply-session.json` exists in `artifacts/unapply/<attempt-id>/`, the working-tree
  reverse-apply completed successfully at the time it was written.
- If the command exits non-zero, `status.json` retains its previous state.
- If the command exits non-zero, `artifacts/unapply/` has no new partial artifacts.
- No partial reverse-apply success is reported as success.

### Consequences

Positive: deterministic audit trail; `unapply-session.json` presence implies success.

Negative: snapshot/restore is not atomic across filesystem operations; a SIGKILL between
step 5 and step 7 can leave the working tree mutated but `status.json` untouched. This is
a known gap inherited from the same constraint on `tpatch apply`. The check+snapshot
protocol makes this window as small as possible.

---

## D7: Composition with `rejected` state

*This is the decision point deferred by ADR-031 D6 to this ADR.*

Source of deferral: `docs/adrs/ADR-031-rejected-feature-state-data-model.md:604-630`:
> "Whether a future retirement command for already-implemented features (Alternative 3 of
> D6, possibly `PRD-feature-unapply.md`) should share any data-model fields with
> `rejected`'s — deferred entirely to future work."

### Question

How do the proposed `unapplied` state and the shipped `rejected` state compose in the
lifecycle? Can a feature be in both states? Is there a transition path between them?

### Alternatives

**(A) Parallel independent states** — `unapplied` and `rejected` are valid `FeatureState`
values that operate on non-overlapping lifecycle segments. Entry to `unapplied` requires a
post-implementation source state (`applied`/`active`/`reconciling`/`reconciling-shadow`).
Entry to `rejected` is restricted to pre-implementation source states
(`requested`/`analyzed`/`defined`). The entry conditions are mechanically disjoint; no
feature can reach both states without an explicit path the state machine does not provide.

**(B) Hierarchical** — `rejected` implies `unapplied` first (a future "post-implementation
reject" would require unapply as a prerequisite step), or `unapplied` is a transient
station in a compound `applied → unapplied → rejected → reopened → applied` arc.

**(C) Union** — `unapplied` and `rejected` are stations in the same terminal arc; future
commands compose them (e.g., `reject --from-applied` that internally unapplies then
rejects).

### Decision: **Alternative A — parallel independent states**

Rationale:

1. **State machine already enforces disjointness.** `RejectableStates = {requested,
   analyzed, defined}` (`internal/store/status.go:108-118`). A feature cannot be both
   in a post-implementation state (required to reach `unapplied`) and in a
   pre-implementation state (required to reach `rejected`). The mutual exclusion is
   mechanical, not aspirational.

2. **No shared schema required.** `RejectionStatus` carries pre-implementation evidence,
   operator rejection intent, and reopen history. `unapply-session.json` carries working-
   tree reverse-apply audit. They are orthogonal schemas (see D3). Sharing fields would
   create an entangled data model with no benefit.

3. **ADR-031 D6's language is precise.** The deferred scope is "post-implementation
   retirement (should never have shipped)" — a terminal verdict on an implemented feature.
   `feature unapply` is a reversible working-tree operation, not a terminal verdict. The
   deferral does not imply the two must converge; it leaves the door open for a future
   dedicated retirement command. This ADR confirms the door stays open but does not walk
   through it.

4. **Hierarchical (B) and union (C) add implementation complexity with no current
   operator demand.** Both require an `unapplied → rejected` transition path that
   `tpatch reject` currently refuses (ADR-031 D6 explicitly closes the post-
   implementation escape hatch). Adding that path here would reopen a door ADR-031 D6
   deliberately closed, without the operator-demand evidence that justified the original
   closure scope.

**Explicit rule (implementation must enforce):**

`tpatch reject` refuses `unapplied` as a source state with exit code 3, exactly as it
refuses `applied`, `active`, `reconciling`, `reconciling-shadow`, `blocked`, and
`upstream_merged`. The refusal message should read: `"cannot reject: feature is
unapplied (post-implementation); use 'tpatch remove <slug>' to discard the feature"`.

`tpatch reopen` has no interaction with `unapplied` features (reopen is the inverse of
reject; an unapplied feature is not rejected). If a hand-edited `status.json` sets
`state: "unapplied"` alongside a live `Rejection` sub-record, `tpatch reopen` should
fail with exit code 1 (internal error: inconsistent state) rather than silently proceeding.

**Cross-reference to PRD:** PRD-feature-unapply.md §8.2 documents the same decision with
more prose context.

### Consequences

Positive: both commands remain self-contained and independently extensible. No coupling
between `reject` command logic and `unapply` command logic.

Negative: operators who want to "permanently retire" a feature that has been applied and
unapplied have no single-step command in v1. They must `tpatch remove <slug>`. A future
`tpatch retire` command could wrap both `feature unapply` + logical deletion, but that
is out of scope for this ADR.

---

## D8: Command-name pattern (`feature unapply` under `feature` group vs. top-level)

*This decision parallels ADR-031 D10, which chose top-level verbs for `reject`/`reopen`.
This ADR documents the opposite decision and explains why both are correct.*

Source of naming convention: ADR-031 D10 Alternative 3 (accepted), documented in
PRD-rejected-feature-state §4.1. The `tpatch feature` group is for "noun-scoped per-
feature management" (`internal/cli/feature_deps.go:41-49`).

### Question

Should the new command be `tpatch feature unapply <slug>` (under the `feature` subcommand
group) or `tpatch unapply <slug>` (top-level lifecycle verb)?

### Alternatives

**Alternative 1 (`tpatch unapply <slug>` — top-level verb)**: Consistent with other
lifecycle verbs (`apply`, `record`, `reconcile`, `land`).

Pros: uniform surface; all lifecycle-affecting commands are at the top level.

Cons:
- `unapply` is not a lifecycle phase advancement like `apply`, `reconcile`, or `land`. It
  is a working-tree artifact management operation: it reverses a patch projection and writes
  an audit artifact. The semantic is closer to `feature patch` (browse patch generations)
  than to `apply` (advance lifecycle phase).
- Placing it at top level alongside `apply` implies a symmetry that does not exist: `apply`
  advances lifecycle state; `unapply` reverses a working-tree projection but does not
  advance the design arc.
- ADR-031 D10 Alternative 3's rationale specifically defines top-level verbs as "lifecycle-
  state transitions" (`analyze`, `define`, `explore`, `implement`, `apply`, `reconcile`,
  `land`, `reject`, `reopen`). `unapply` transitions `applied → unapplied` and
  `unapplied → [next apply]`, which look like a lifecycle transition. But the key
  distinction is that the `feature` group houses operations whose primary subject is a
  feature's artifact namespace (deps, claim, patch, and now unapply-of-patch); the
  lifecycle verbs advance or query a feature's design arc.

**Alternative 2 (`tpatch feature unapply <slug>` — noun-scoped under `feature`)**: Sits
under the existing `feature` group alongside `feature deps`, `feature claim`, and
`feature patch`.

Pros:
- Consistent with the `feature` group's established scope: working-tree and sub-record
  artifact operations.
- Clear distinction from the top-level lifecycle verbs: `apply` advances lifecycle state;
  `feature unapply` removes an artifact projection.
- PRD §3.4 documents the intentional asymmetry with a 4-point rationale; the `--help` cross-
  reference strings make the relationship to `apply` discoverable.
- No new top-level name collision risk (cf. `reconcile --reject` vs `reject` issue that
  drove ADR-031 D10).

Cons:
- Asymmetric with `apply`/`reapply` at the surface level. Operators who search for
  "how do I undo apply" may look for `tpatch unapply` first.
- Adds a second verb pattern to the codebase (top-level lifecycle verbs + noun-scoped
  feature management verbs). PRD §3.4 cross-reference strings mitigate discoverability.

**Alternative 3 (`tpatch feature remove-patch <slug>` or another compound name)**:
Avoids the `apply`/`unapply` pairing entirely.

Cons: `remove-patch` is a weaker name. `feature unapply` reads clearly as "the inverse of
applying the feature." The pairing is a feature, not a liability.

### Decision: **Alternative 2 — `tpatch feature unapply <slug>` under `feature` group**

Rationale:

1. The `feature` group already houses `feature patch`, which is an artifact-management
   operation on the canonical patch. `feature unapply` is a working-tree projection removal
   operation on the same artifact. Same group, same noun-scoped pattern.

2. ADR-031 D10 explicitly defined the `feature` group's scope as "noun-scoped per-feature
   management" and the top-level verbs as "lifecycle-state transitions." While `unapply`
   does transition state (`applied → unapplied`), its primary action is working-tree
   artifact management (reverse-apply, snapshot, audit). The state transition is a
   side-effect of the artifact operation, not the primary intent.

3. The asymmetry with top-level `apply` is acceptable and documented. PRD §3.4 cross-
   reference strings and `--help` footers make the relationship discoverable. The Cluster F'
   test-27 precedent mandates golden string assertions for both `feature unapply --help`
   and `tpatch apply --help` to ensure cross-references survive refactoring.

**Contrast with ADR-031 D10's opposite decision:** D10 chose top-level for `reject`/`reopen`
because they are "first-class lifecycle-state transitions — terminal outcome and its reversal"
with no working-tree mutation. `feature unapply` mutates the working tree as its primary
action; the state transition is a recording of what happened. The surface asymmetry between
"working-tree operation → `feature` group" and "pure state transition → top level" is the
correct and principled distinction.

### Consequences

Positive: `feature` group remains coherent; no new top-level name collisions.

Negative: discoverability cost for operators who think "apply ↔ unapply". Mitigated by
`tpatch apply --help` cross-reference string added in the implementation test matrix.

---

## Implementation Notes (for Cluster G')

The following notes are for the implementation cluster that executes after this ADR is
accepted. They are not decisions; they are forward-guidance collected during planning.

1. **Add `StateUnapplied = "unapplied"` at `internal/store/types.go`** after `StateRejected`.
   Extend `ValidFeatureState` closed switch. Audit every `switch state { }` that handles
   `StateApplied`; each must either handle `StateUnapplied` explicitly or document why it
   falls through.

2. **Dependency satisfying set** (`internal/workflow/` apply gate): `StateUnapplied` must NOT
   be added to the set. When a hard parent is `unapplied`, the child's apply must refuse with
   `waiting-on-parent` or a new `parent-unapplied` label (D2 note). Confirm label name with
   ADR-011 precedent.

3. **`tpatch reject` source-state check** (`internal/cli/reject.go`): After adding
   `StateUnapplied`, the existing `IsRejectableState` check at the state-machine gate will
   correctly refuse it because `unapplied` is not in `RejectableStates`. Verify the error
   message rendered includes the D7 guidance message.

4. **`confirm-upstreamed` guard** (`internal/cli/cobra.go:2525-2540`): Add `StateUnapplied`
   to the guard analogously to the `StateRejected` guard. Exit code 3; suggest `tpatch apply
   <slug>` first.

5. **`tpatch status` filtering** (§8.3 decision): `unapplied` features appear in the default
   listing. No `--include-unapplied` flag. Add `[unapplied]` rendering badge. Update
   `FEATURES.md` template to add `## Unapplied` section.

6. **`tpatch next`**: Add `unapplied` to the next-action table recommending `tpatch apply
   <slug>`.

7. **`tpatch land`**: Add `StateUnapplied` to the `land` refusal gate (PRD §11.3). Exit
   code 3; suggest `tpatch apply <slug>` first.

8. **`unapply-session.json` schema**: Implement exactly the D3 wire schema. Use
   `json.Marshal` with a stable-sorted struct (not `map[string]interface{}`). The `attempt_id`
   format is `ua_` + 12 lowercase hex chars (`crypto/rand` + `encoding/hex`).

9. **`reverse.patch`**: Write under `artifacts/unapply/<attempt-id>/reverse.patch`. Not under
   `patches/` (D4). Derive by capturing the reverse diff from the snapshot/restore cycle.

10. **Snapshot/restore discipline** (D6): the snapshot must include file permissions and
    missing-file markers. A file that the patch deletes (new file in the feature patch →
    file should be absent after unapply) must be snapshotted as "absent" and restored to
    absent if rollback is needed.

11. **`--help` cross-reference strings** (D8, PRD §3.4): Both `tpatch feature unapply --help`
    and `tpatch apply --help` must include the golden cross-reference strings from PRD §3.4
    as long-description footer text. The implementation test matrix must include assertions
    analogous to Cluster F' test 27.

12. **`tpatch reopen` guard on impossible state**: If `status.json` somehow has
    `state: "unapplied"` with a non-nil `Rejection` sub-record, `tpatch reopen` should
    exit 1 with an internal-error message rather than proceeding (D7 consequence).

13. **Assets parity guard**: Any changes to lifecycle state enumeration in assets must pass
    `assets/assets_test.go`. Plan for state-count 11→12 in the parity guard.

---

## Negative Consequences Summary

| Decision | What breaks / what's deferred |
|---|---|
| D1 (real state) | Every `switch state {}` must handle `StateUnapplied`; 11→12 state count breaks asset guards until updated. |
| D2 (does not satisfy) | Applied children of an unapplied parent enter a detectable broken state. Operator must reapply parent or unapply child. |
| D3 (fixed schema) | Future schema extensions require a `version: 2` migration path. |
| D4 (no patch-gen write) | `feature patch <slug>` history does not show unapply/reapply events. |
| D5 (patch-mode only) | Users with landed features cannot unapply them in v1. |
| D6 (snapshot/restore atomicity) | A SIGKILL between mutation and status-write leaves working tree mutated but `status.json` stale. |
| D7 (parallel states) | No single-step "retire applied feature" command exists; operators use `tpatch remove`. |
| D8 (`feature unapply`) | Discoverability cost for operators who expect `tpatch unapply`; mitigated by `--help` cross-references. |

---

## Test Matrix Outline (Minimum Coverage for Cluster G')

The test matrix below mirrors the PRD §15 acceptance criteria and is broken out per
decision point to help reviewers verify coverage without reading the full test file.

| # | Decision | Test scenario | Expected |
|---|---|---|---|
| 1 | D1 | `ValidFeatureState("unapplied")` | `true` |
| 2 | D1 | `ValidFeatureState` with all 11 existing values | still `true` |
| 3 | D1 | `switch state` exhaustiveness (compile-time) | zero `default: return err` paths that don't mention `StateUnapplied` in a comment |
| 4 | D2 | Apply child when hard parent is `unapplied` | refused; waiting-on-parent |
| 5 | D2 | `tpatch status` with applied child and unapplied hard parent | DAG warning emitted |
| 6 | D2 | Apply child when hard parent is `applied` → unapply parent → child status | DAG warning; child can no longer re-run apply |
| 7 | D3 | Successful unapply writes `unapply-session.json` with all required fields | all fields present, correct types, stable-sorted |
| 8 | D3 | `attempt_id` format | `ua_` + 12 lowercase hex chars |
| 9 | D3 | `canonical_patch_sha256` is lowercase 64-hex | regex `^[0-9a-f]{64}$` |
| 10 | D3 | `dependency_blockers: []` (not `null`) when no blockers | `[]` |
| 11 | D3 | `preflight.conflict_markers: []` (not `null`) when clean | `[]` |
| 12 | D4 | `patch-generations.json` unchanged after unapply | byte-identical before/after |
| 13 | D5 | `tpatch feature unapply <slug> --mode landed-commit` | exit 2, error message |
| 14 | D6 | Reverse-apply check fails → no mutation, no artifact, status unchanged | working tree identical, `status.json` unchanged |
| 15 | D6 | Successful unapply → status records `unapplied` | `status.json:state == "unapplied"` |
| 16 | D6 | Successful unapply → `artifacts/post-apply.patch` byte-identical | SHA matches pre-unapply |
| 17 | D7 | `tpatch reject` from `unapplied` source state | exit 3, message mentions `tpatch remove` |
| 18 | D7 | `tpatch reopen` from `unapplied` source state (no rejection record) | refused or no-op; no state mutation |
| 19 | D7 | `confirm-upstreamed` from `unapplied` source state | exit 3, suggests `tpatch apply` first |
| 20 | D8 | `tpatch feature unapply --help` contains apply cross-reference | golden string present |
| 21 | D8 | `tpatch apply --help` contains `feature unapply` cross-reference | golden string present |
| 22 | (PRD §3.3) | `--dry-run` on dirty tree | exit 0, reports dirty-tree blockers, no mutation |
| 23 | (PRD §5) | Hard dependent exists → unapply refused | exit 3, lists blocker slug |
| 24 | (PRD §5) | Soft dependent exists, no flag → unapply refused | exit 3 |
| 25 | (PRD §5) | Soft dependent exists, `--allow-soft-dependents` → unapply proceeds | success |
| 26 | (PRD §5.1) | Dependency edge creation onto `unapplied` parent | allowed (no Rule-7-analog) |
| 27 | (PRD §8.3) | `tpatch status` default listing includes `unapplied` feature | appears with `[unapplied]` badge |
| 28 | (PRD §8.3) | `--include-rejected` does not affect unapplied visibility | unapplied features still shown |
| 29 | (PRD §11.3) | `tpatch land <slug>` from `unapplied` | exit 3, suggests `tpatch apply` |
| 30 | (PRD §14) | `tpatch apply <slug>` from `unapplied` returns state to `applied` | success, `status.json:state == "applied"` |

Cluster G' implementation cluster may extend this list. The 30-item baseline establishes
minimum coverage for rev-0 review.
