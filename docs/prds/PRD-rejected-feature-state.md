# PRD: Rejected Feature State

**Status**: Proposed
**Author**: Copilot (Cluster F planning phase)
**Date**: 2026-08-05
**Related**: GH #6, PRD-confirm-upstreamed-human-review-path (orthogonal), ADR-031-rejected-feature-state-data-model (data model), ADR-024-patch-generation-manifest-boundary (migration precedent)
**Refresh triggers**: state enum extension, evidence field format changes, reason code additions

> **Naming note**: GH #6's dispatch brief (`docs/handoff/CURRENT.md`, `docs/supervisor/LOG.md`) and the
> task brief that produced this PRD both refer to the companion ADR as "ADR-028". At planning time
> `docs/adrs/ADR-028-supersession-edge-model.md` (Accepted, 2026-07-29, GH #1 cluster) already occupies
> that number. This PRD and its companion ADR use the next free slot, **ADR-031**, to avoid a filename
> and identifier collision. No other content changes; every cross-reference in this document points at
> `ADR-031-rejected-feature-state-data-model.md`.

## 1. Problem

GH #6 §Summary states, verbatim:

> tpatch has no supported lifecycle outcome for a requested feature whose premise is disproven by
> measurement before implementation.
>
> The current choices are all misleading or destructive:
>
> - leave it `requested`, which keeps it in the actionable backlog forever;
> - advance it through implementation/apply with an empty or fabricated patch;
> - remove it, which deletes the evidence and loses traceability;
> - overwrite request prose with `REJECTED`, which preserves evidence but remains machine-invisible.
>
> This is related to, but distinct from, #4. That issue concerns a rejected/confirmed **upstreamed
> reconciliation candidate**. This issue concerns a normal feature request rejected as not-a-bug or
> no-longer-needed before implementation.

This is a measurement-first-engineering gap. tpatch's current state machine (`SPEC.md` §3, `internal/store/types.go:9-19`)
has exactly one terminal state that isn't a healthy live/merged outcome: `blocked` — and `blocked` is
explicitly a *temporary* failure ("Failed; needs manual intervention", `SPEC.md` §3 Feature States table),
not a decision. There is no state that means "we investigated, and the correct engineering outcome is
**do not implement this**." Every available workaround destroys either the evidence (`remove`) or the
backlog signal (leaving it `requested` forever), or falsifies machine-readable state (hand-editing
`request.md` prose, which `tpatch status`/`tpatch next` cannot see).

## 2. Motivating Example

GH #6 §Concrete reproduction, verbatim:

> Repository: `tesseracode/copilot-api`
>
> Feature:
>
> ```text
> claude-developer-instruction-preservation
> ```
>
> Initial premise:
>
> > OpenAI `developer` instructions are lost when Claude Chat requests are converted to Anthropic Messages.
>
> Measurement showed the premise was wrong at current HEAD:
>
> - the assumed Chat→Anthropic converter has no production caller;
> - Claude `/v1/chat/completions` passes the original Chat payload upstream;
> - live Sonnet 4.6, Opus 4.8, and Haiku 4.5 probes all honored:
>   - developer-only instructions;
>   - system-only instructions;
>   - mixed instructions;
>   - both conflict orders;
>   - a later developer message after an assistant turn.
>
> The correct engineering outcome was **no source change**. The probe evidence is valuable and should
> prevent future agents from re-proposing the same false fix.
>
> `tpatch` offered no lifecycle transition for that outcome. The workaround was:
>
> ```bash
> tpatch amend claude-developer-instruction-preservation \
>   "REJECTED after live measurement: ..."
> ```
>
> and hand-authored:
>
> ```text
> analysis.md
> spec.md
> exploration.md
> artifacts/live-role-probe.md
> ```
>
> The feature still remains `state: requested`, so status/backlog tooling continues to present it as
> pending work.

This is the PRD's real-world anchor: the live-role-probe evidence (`artifacts/live-role-probe.md`) is
exactly the kind of artifact a `rejected` state must preserve and surface, not the kind a `remove` or a
silent `requested` backlog entry can represent.

## 3. Proposed Behavior

GH #6 §Expected semantics states a rejected feature should have the following 9 properties. Each is
transcribed verbatim below, with PRD-precision sub-bullets added where GH #6 leaves an implementation
choice open.

### 3.1

> 1. Preserve the complete feature directory and append-only audit history.

- `reject` and `reopen` MUST NOT delete, truncate, or overwrite `request.md`, `analysis.md`, `spec.md`,
  `exploration.md`, `record.md`, or anything under `artifacts/` or `patches/`.
- The only files `reject`/`reopen` write are `status.json` (new/updated fields, §6) and the
  `FEATURES.md` index refresh already performed by every other status-mutating command
  (`internal/store/store.go:363` doc comment: "SaveFeatureStatus writes status.json for a feature and
  refreshes FEATURES.md").

### 3.2

> 2. Require human-readable rationale and at least one evidence reference.

- `note` (free text) and at least one `evidence` path are hard requirements at the CLI layer, validated
  before any write occurs (fail-closed — no partial state).

### 3.3

> 3. Record timestamp, actor/provenance, prior state, reason code, and optional related feature/issue.

- Fields: `rejected_at` (RFC3339), `rejected_by` (actor string), `prior_state` (the `FeatureState` value
  immediately before rejection), `reason` (closed enum, §6), `related` (optional slug or `GH#N`
  reference).

### 3.4

> 4. Be excluded from default actionable backlog and `next` implementation guidance.

- `tpatch status` (no flags) excludes `rejected` features from the default listing; `tpatch next` on a
  rejected slug prints the rejection reason/evidence/reopen command instead of proposing `analyze`
  (§7).

### 3.5

> 5. Appear distinctly in `status` and `FEATURES.md`.

- `--include-rejected` (or `--all`, see §7) surfaces them in `status` with a `rejected` state column;
  `FEATURES.md` renders a distinct trailing section (§7).

### 3.6

> 6. Not participate in apply/reconcile by default.

- `tpatch apply <slug>` and `tpatch reconcile <slug>` both refuse on a `rejected` feature with a clear,
  actionable error (§7). No escape hatch is introduced by this PRD; see ADR-031 D6 for whether one is
  ever warranted.

### 3.7

> 7. Remain queryable for architecture history and duplicate detection.

- `status --json --include-rejected` and `FEATURES.md` are the two supported query surfaces. No new
  query subcommand is proposed; existing `tpatch status --feature <slug>` detail view already works for
  any state and needs no change beyond rendering the new fields.

### 3.8

> 8. Support an explicit evidence-linked reopen transition back to `requested` if circumstances change.

- `tpatch reopen <slug> --evidence <path> --note <text>` transitions `rejected → requested`. The
  reopen call is itself evidence-linked and audited (§6 `history[]`), matching the rejection's own
  evidentiary bar rather than being a bare state flip.

### 3.9

> 9. Refuse rejection from states where source changes are already applied unless the command also
>    performs a safe retirement/audit, or provide separate pre-implementation and post-implementation
>    retirement semantics.

- This PRD picks the **separate semantics** branch: `reject` is a **pre-implementation** lifecycle
  terminal only. It is allowed from `requested`, `analyzed`, `defined`, `explored` (§5). It is refused
  outright — no escape hatch — from `implementing`, `applied`, `active`, `reconciling`,
  `reconciling-shadow`, `blocked`, `upstream_merged`. Post-implementation retirement already has a
  dedicated, evidence-linked mechanism: `tpatch reconcile confirm-upstreamed` +
  `tpatch reconcile audit-retirement` (`SPEC.md` §4 Phase 1 table;
  `docs/prds/PRD-confirm-upstreamed-human-review-path.md` §2-3). Building a second retirement path inside
  `reject` would duplicate that machinery and blur the orthogonality GH #6 itself calls out (§10). See
  ADR-031 D6 for the full rationale.

## 4. CLI Shape

Two candidate forms were evaluated:

- **(a)** `tpatch reject <slug> --reason <code> --evidence <path> [--note <text>] [--related <ref>]`
- **(b)** `tpatch feature state <slug> rejected --reason <code> ...`

**Chosen: form (a).**

Rationale:

- Every existing lifecycle-phase transition in tpatch is a top-level verb subcommand:
  `analyze`, `define`, `explore`, `implement`, `apply`, `record`, `land`, `reconcile`, `amend`, `remove`,
  `next` are all registered directly on the root command (`internal/cli/cobra.go:60-80`,
  `buildRootCmd`). `reject` (and `reopen`) is exactly this kind of transition — it belongs alongside
  them, not nested.
- The existing `tpatch feature <subcommand>` group (`internal/cli/feature_deps.go:41-49`, `featureCmd`)
  is reserved for **noun-scoped per-feature management** (`deps`, `claim`, `patch`), not lifecycle-state
  transitions. Its own doc comment explains the `feature deps <slug> add <parent>` ordering choice
  precisely because "the natural CLI verb is `feature deps <slug> add <parent>`... the slug comes before
  the action" (`internal/cli/feature_deps.go:52-56`) — i.e. `feature <noun> <slug> <verb>`, not
  `feature <verb-like-state> <slug>`. Retrofitting a bare state name (`rejected`) as a sub-verb under
  `feature state` does not match that group's established shape and would read as a generic escape
  hatch around lifecycle verbs — precisely the kind of thing `amend --state` was deliberately reserved
  against (`internal/cli/c1.go:276-284`, `validateAmendStateFlag`: "Lifecycle states are owned by other
  verbs (add/analyze/define/explore/implement/apply/reconcile)").
- Form (a) also composes more naturally with `--json` (matching `reconcile confirm-upstreamed --json`,
  `SPEC.md` §4) and gives a single, greppable verb for tooling/skills to reference.

Reverse transition:

```bash
tpatch reopen <slug> --evidence <path> --note <text>
```

`reopen` is a sibling top-level verb, symmetric with `reject`, following the same registration
convention.

## 5. State Machine

```
                    ┌──────────┐
        ┌──────────►│ rejected │◄────────────┐
        │           └────┬─────┘             │
        │ reject          │ reopen            │ reject
        │                 ▼                   │
   ┌─────────┐      ┌───────────┐             │
   │requested│─────►│  (back to │─────────────┘
   └────┬────┘       requested) │
        │           └───────────┘
        ▼
   ┌─────────┐  reject   ┌──────────┐
   │analyzed │──────────►│ rejected │
   └────┬────┘           └──────────┘
        ▼
   ┌─────────┐  reject   ┌──────────┐
   │ defined │──────────►│ rejected │
   └────┬────┘           └──────────┘
        ▼
   ┌─────────┐  reject   ┌──────────┐
   │ explored│──────────►│ rejected │
   └────┬────┘           └──────────┘
        ▼
  implementing ──X (reject refused)
        ▼
    applied   ──X (reject refused)
        ▼
    active    ──X (reject refused)
        ▼
  reconciling / reconciling-shadow ──X (reject refused)
        ▼
    blocked   ──X (reject refused)
        ▼
 upstream_merged ──X (reject refused)
```

Note: `explored` is not itself a distinct `FeatureState` value today — `explore` output lands under the
`defined` state per the phase model (`SPEC.md` §3 lists `defined` immediately preceding
`implementing`, and `internal/cli/phase2.go:400-...` `nextAction`'s `StateDefined` case dispatches on
whether `exploration.md` exists yet). This PRD treats "before `implementing` has produced any recipe or
patch" as the allowed-rejection boundary — i.e. `requested`, `analyzed`, `defined` (whether or not
`exploration.md` exists under it) are all reject-eligible; `implementing` onward is not.

**Allowed rejections** (no code side effects, since no apply recipe has executed): from `requested`,
`analyzed`, `defined` (pre- or post-`exploration.md`).

**Refused rejections**: from `implementing`, `applied`, `active`, `reconciling`, `reconciling-shadow`,
`blocked`, `upstream_merged`. See ADR-031 §"D6 — post-implementation escape hatch" for why this PRD does
not add one; the short version is that `tpatch reconcile confirm-upstreamed` /
`tpatch reconcile audit-retirement` already own the post-implementation retirement path
(`docs/prds/PRD-confirm-upstreamed-human-review-path.md` §2), and duplicating that machinery under
`reject` would violate the orthogonality GH #6 itself demands (§10).

**Reopen**: `rejected → requested` only. Append-only: the prior rejection record (`reason`, `evidence`,
`note`, `rejected_at`, `rejected_by`, `prior_state`) is never deleted — it is pushed onto `history[]`
(§6) and a fresh `requested` cycle begins. Reopen itself requires its own `--evidence`/`--note` pair,
appended to the same `history[]` array, so "why did we reopen this" is as auditable as "why did we
reject this".

### What happens to a rejected feature that other features depend on?

Three options were considered:

1. **Fail loudly and refuse the reject** — if any other feature's `depends_on` (hard or soft) lists this
   slug as a parent, `tpatch reject` errors out before writing anything, listing the dependent(s) and
   their kinds, and pointing at how to resolve (remove the edge, or reject the dependent too).
2. **Warn and proceed** — print a warning listing dependents but still perform the rejection.
3. **Require `--force`** — same warning, but require an explicit flag to proceed.

**Chosen: Option 1 (fail loudly and refuse).** This mirrors the existing, already-shipped precedent for
`tpatch remove`: `checkRemoveDependents` (`internal/cli/feature_deps.go:430-447`) refuses removal
outright when dependents exist, regardless of `--force` ("`--force` is a TTY-confirmation override; it
does NOT bypass DAG integrity" — same file, doc comment on `removeCmd`, `internal/cli/c1.go:341,358-361`).
A feature being rejected is at least as disruptive to its dependents as a feature being deleted (its
parent's work is now formally "will never be implemented" rather than merely gone from disk), so the bar
for silently proceeding should be at least as high. Options 2 and 3 are recorded as alternatives in §12.

## 6. Required Fields

All new fields live on `FeatureStatus` (`internal/store/status.json`), gated by `state: rejected` (a
tenth value added to the existing nine-value `FeatureState` enum, `internal/store/types.go:9-19`).

| Field | Type | Validation |
|---|---|---|
| `state` | `FeatureState` enum, top-level | Must be `"rejected"`. Existing `ValidFeatureState` switch (`internal/store/types.go:21-27`) gains this arm. |
| `reason` | closed enum string | One of: `not-a-bug`, `premise-disproved`, `obsolete`, `out-of-scope`, `unsafe`, `duplicate`, `superseded`. Any other value is a validation error (see ADR-031 D2 for closed-vs-open rationale). |
| `evidence` | `[]string` (list of paths) | At least one entry required. Each path must exist, resolved relative to the feature directory `.tpatch/features/<slug>/` (falls back to repo-root-relative if not found there — see ADR-031 D3 for the exact resolution order). A path that resolves to neither location is a validation error. |
| `note` | `string` | Required, non-empty after trim. Free-form rationale. |
| `rejected_at` | `string` (RFC3339) | Set by the CLI at write time; not operator-supplied. |
| `rejected_by` | `string` (actor) | Derived the same way other provenance fields are today (see `ReconcileRevision` actor precedent in `internal/store/reconcile_revision.go`); operator-overridable via `--actor` only if an equivalent flag already exists elsewhere in the CLI — otherwise defaults to a fixed `"operator"`/environment-derived string, decided at implementation time. |
| `prior_state` | `FeatureState` enum | Captured automatically as whatever `state` was immediately before this `reject` call. |
| `related` | `string`, optional | Free-form: a feature slug or a `GH#N` reference. Not validated against the store (a related feature may not exist in this repo, e.g. a cross-repo GH issue). |
| `history` | `[]HistoryEntry`, append-only | Populated on `reopen` (and on every subsequent `reject`/`reopen` cycle). Each entry snapshots the fields above (`reason`, `evidence`, `note`, `rejected_at`, `rejected_by`, `prior_state`, `related`, plus the reopen's own `reopened_at`/`reopened_by`/`reopen_note`/`reopen_evidence`) so no rejection record is ever overwritten — only appended past. Matches GH #6 §7's "append-only" requirement (§8 acceptance criteria: "Reopen is explicit and append-only"). |

## 7. Integration Semantics

- **`tpatch status`**: rejected features are excluded from the default (no-flag) listing and from the
  default JSON envelope's `features[]` array. A new `--include-rejected` boolean flag opts them back in
  (additive to the existing `--json`/`--verbose`/`--feature`/`--dag` flags on `statusCmd`,
  `internal/cli/cobra.go:226,441-444`). Today `statusCmd` has **no state-based filtering at all** — every
  feature returned by `s.ListFeatures()` (`internal/cli/cobra.go:236`) is printed in both the text loop
  (`internal/cli/cobra.go:362`) and the JSON `rendered` slice (`internal/cli/cobra.go:308-334`); this PRD
  is the first time `status` gains a state-based exclusion. The JSON envelope gains
  `rejection: {reason, evidence, note, rejected_at, rejected_by, prior_state, related}` (omitted/absent
  for non-rejected features) alongside the existing per-feature fields.
- **`tpatch next`**: for a `rejected` slug, prints the rejection reason, evidence list, and the exact
  `tpatch reopen <slug> ...` command — it does NOT propose `tpatch analyze` or any other forward-phase
  action. This is a new terminal case added to the `nextAction` state switch
  (`internal/cli/phase2.go:400`, which today starts its `switch status.State` at
  `case store.StateRequested:` on line 410 and has no `rejected` arm because the state doesn't exist
  yet).
- **`FEATURES.md` renderer**: `RefreshFeaturesIndex` (`internal/store/store.go:677-695`) currently emits
  one flat table (`| Slug | Title | State | Compatibility |`, `internal/store/store.go:687-693`) with no
  state-based branching. This PRD adds a distinct trailing section, e.g.:

  ```markdown
  ## Rejected

  | Slug | Reason | Evidence | Note |
  |------|--------|----------|------|
  | `claude-developer-instruction-preservation` | premise-disproved | `artifacts/live-role-probe.md` | Claude Chat already preserves developer instructions |
  ```

  rendered after the main table, sourced from the same `features` slice already loaded by
  `RefreshFeaturesIndex` — no new store call. Rejected rows are also removed from the main table's
  default view (mirroring the `status` default-exclusion above) and only appear in the new section.
- **`tpatch apply` / `tpatch reconcile`**: both refuse by default on a `rejected` feature with a clear
  error (e.g. `feature "<slug>" is rejected (reason=<code>); run` `tpatch reopen <slug>` `to resume, or
  see` `<evidence>` `for rationale`). Today neither `applyCmd` (`internal/cli/cobra.go:634-693`) nor the
  reconcile entrypoint checks `status.State` against a terminal/refused list before proceeding — this
  PRD adds that check as a new precondition, matching the fail-fast style. Any future escape-hatch
  behavior for either command is explicitly deferred to ADR-031 §D6 and is NOT specified by this PRD.
- **`tpatch remove`**: `removeCmd` (`internal/cli/c1.go:341-395`) does not inspect `status.State` at all
  today — it removes a feature in *any* state (subject only to the dependents gate,
  `internal/cli/feature_deps.go:430-447`, and the `--cascade`/confirmation flow). This PRD does **not**
  change that: a `rejected` feature is removable exactly like any other feature, with the same dependents
  gate applying. Rejection intentionally does not add extra remove-protection beyond what any other
  state already has — `rejected` preserves evidence by not being auto-deleted, not by becoming
  un-removable. An operator who explicitly wants to delete a rejected feature's directory (e.g. after
  exporting its evidence elsewhere) retains that ability, same as today.

## 8. JSON Envelope

`tpatch reject --json` success:

```jsonc
{
  "slug": "claude-developer-instruction-preservation",
  "state": "rejected",
  "prior_state": "requested",
  "reason": "premise-disproved",
  "evidence": ["artifacts/live-role-probe.md"],
  "note": "Claude Chat already preserves developer instructions",
  "rejected_at": "2026-08-05T00:00:00Z",
  "rejected_by": "operator",
  "related": null
}
```

`tpatch reject --json` error (missing evidence example):

```jsonc
{
  "error": "evidence required: at least one --evidence path must be supplied",
  "slug": "claude-developer-instruction-preservation",
  "exit_code": 1
}
```

`tpatch reject --json` error (invalid reason code):

```jsonc
{
  "error": "invalid reason \"not-really-a-code\": must be one of not-a-bug, premise-disproved, obsolete, out-of-scope, unsafe, duplicate, superseded",
  "slug": "claude-developer-instruction-preservation",
  "exit_code": 1
}
```

`tpatch reject --json` error (wrong state, e.g. `applied`):

```jsonc
{
  "error": "cannot reject feature \"x\" from state \"applied\": reject is only valid from requested, analyzed, or defined. Use `tpatch reconcile confirm-upstreamed` / `tpatch reconcile audit-retirement` for post-implementation retirement.",
  "slug": "x",
  "state": "applied",
  "exit_code": 1
}
```

`tpatch status --json` addition (per-feature, only present when `state == "rejected"`):

```jsonc
{
  "slug": "claude-developer-instruction-preservation",
  "state": "rejected",
  "rejection": {
    "reason": "premise-disproved",
    "evidence": ["artifacts/live-role-probe.md"],
    "note": "Claude Chat already preserves developer instructions",
    "rejected_at": "2026-08-05T00:00:00Z",
    "rejected_by": "operator",
    "prior_state": "requested",
    "related": null
  }
}
```

`tpatch reopen --json` success:

```jsonc
{
  "slug": "claude-developer-instruction-preservation",
  "state": "requested",
  "reopened_at": "2026-08-06T00:00:00Z",
  "reopened_by": "operator",
  "reopen_note": "Upstream converter regained a caller in v1.4; re-evaluating.",
  "reopen_evidence": ["artifacts/upstream-v1.4-caller-diff.md"],
  "history_entries": 1
}
```

## 9. Tests-to-Write List

The implementation cluster (Cluster F') must write, at minimum:

1. Reject from `requested` — happy path.
2. Reject from `analyzed`, `defined`, `defined`-with-`exploration.md` (the three pre-implementation
   sub-states) — happy path per intermediate state.
3. Reject with missing evidence file → error, no state change (status.json byte-identical before/after
   the failed call).
4. Reject with invalid reason code → error, no state change.
5. Reject on non-existent slug → error.
6. Reject on already-rejected feature → error (PRD decision: error, not idempotent no-op — see §12).
7. Reject on `implementing`/`applied`/`active`/`reconciling`/`blocked`/`upstream_merged` state → error
   in every case (escape hatch, if any, is tested separately under a future ADR/PRD, not this one).
8. Dependency effects: reject A while B declares `depends_on: [{slug: A, kind: hard}]` (or `soft`) →
   error, no state change, dependent(s) listed in the error message.
9. Reopen from `rejected` → `requested`; verify `history[]` gains exactly one new entry and the prior
   rejection record (`reason`, `evidence`, `note`, `rejected_at`, `rejected_by`, `prior_state`) is
   byte-identical to what it was pre-reopen inside that history entry.
10. Reopen from any non-`rejected` state → error.
11. `status` (no flags) excludes rejected features from both text and `--json` output;
    `--include-rejected` shows them in both.
12. `next` on a rejected slug prints reopen guidance (reason, evidence list, exact reopen command) and
    does NOT emit an `analyze`/`define`/etc. `HarnessTask`.
13. `FEATURES.md` render includes a distinct "Rejected" section for rejected features and excludes them
    from the main table.
14. `apply <rejected-slug>` → error, no file mutation, no recipe execution attempted.
15. `reconcile <rejected-slug>` (and bare `reconcile` sweeping all features) → rejected features are
    skipped/errored per-slug without aborting a multi-slug sweep.
16. JSON envelope shape validation for `reject --json`, `reopen --json`, and `status --json`'s
    `rejection` sub-object (field presence/absence, RFC3339 timestamp format, enum closure for
    `reason`).

## 10. Distinction from Related Concepts

GH #6 §Distinction from related concepts, verbatim:

> - **remove**: destructive cleanup; rejected must preserve history.
> - **supersedes** (#1): one implementation replaces another; rejected may have no replacement.
> - **upstream_merged**: implementation exists upstream; rejected means no implementation should be
>   applied.
> - **blocked**: work may continue after a blocker is resolved; rejected is a terminal decision unless
>   explicitly reopened.
> - **rejected-upstreamed** (#4): review verdict about an upstreamed reconciliation candidate, not a
>   general feature lifecycle outcome.

Per-concept confirmation of no scope overlap:

- **`remove`**: `RemoveFeature` (`internal/store/store.go:659-670`) deletes the feature directory
  outright. `reject` writes fields onto the surviving `status.json` and touches nothing else — the two
  are mutually exclusive operations on disjoint data (delete vs. annotate), and §7 above confirms
  `remove` is unmodified by this PRD.
- **`supersedes` (#1)**: `DependencyKindSupersedes` (`internal/store/types.go:299`) is a *replacement*
  edge between two features, one of which is expected to live on. `rejected` carries no such expectation
  — a rejected feature may have zero replacements, and this PRD adds no new dependency-edge kind.
- **`upstream_merged`**: `StateUpstreamMerged` (`internal/store/types.go:18`) means the feature's
  intent *was* implemented, just not by tpatch's own patch. `rejected` is the opposite claim — the
  correct outcome is that *no implementation should exist at all*. The two states are mutually exclusive
  terminal outcomes and this PRD adds `rejected` as a tenth, disjoint enum value, not a variant of
  `upstream_merged`.
- **`blocked`**: `StateBlocked` (`internal/store/types.go:17`) is explicitly temporary ("Failed; needs
  manual intervention", `SPEC.md` §3) — work is expected to resume once the blocker clears, with no
  new evidence/reason schema attached. `rejected` is a terminal decision that only reverses via the
  explicit, evidence-linked `reopen` transition (§5, §6) — a materially higher bar than simply clearing
  whatever blocked a `blocked` feature.
- **`rejected-upstreamed` (#4)**: this is a `Reconcile.ReviewVerdict` string value
  (`internal/store/types.go:318`) describing one possible human review outcome of an *already-attempted*
  reconciliation on an *already-implemented* feature (`docs/prds/PRD-confirm-upstreamed-human-review-path.md`
  §2-3). It lives on `ReconcileSummary`, a sub-object of `FeatureStatus` that only ever gets populated
  after `tpatch reconcile` has run. `rejected` (this PRD) is a top-level `FeatureState` reached
  *pre-implementation*, before any recipe, patch, or reconciliation exists. See ADR-031 §4 for the full
  data-model-level orthogonality argument.

## 11. Migration

- Existing `feature.yaml`/`status.json` files with no `rejection`-related fields are unaffected: the
  `state` enum simply never contains `"rejected"` for them, and `omitempty` on every new field means
  they continue to round-trip byte-identical (same pattern as every prior additive field —
  `internal/store/types.go:207-215` documents this exact contract for `DependsOn`).
- Existing features with `state: requested` (or `analyzed`/`defined`) that an operator now wants to mark
  rejected: the operator runs `tpatch reject <slug> --reason <code> --evidence <path> --note <text>`
  exactly as they would for a newly-discovered rejection. No bulk/batch migration command is proposed.
- No auto-migration of any kind. No existing feature is silently reclassified as `rejected` by this
  change; the state transition is always an explicit, operator-invoked `reject` call.

## 12. Alternatives Considered

- **CLI form (a) vs (b)**: form (b) (`tpatch feature state <slug> rejected ...`) was rejected as a
  mismatch with both the top-level-verb convention (§4) and the `feature` group's existing noun-scoped
  shape (`internal/cli/feature_deps.go:41-56`).
- **Dependency-effect strictness**: fail/refuse (chosen, §5) vs. warn-and-proceed vs. require `--force`.
  Warn-and-proceed was rejected because it would let a rejection silently strand dependents in a state
  their own tooling can't detect until they next run `status --dag`. Require-`--force` was rejected for
  consistency with `remove`'s explicit precedent that `--force` never bypasses DAG integrity
  (`internal/cli/c1.go:358-361`) — extending a force-bypass to `reject` (which is arguably more
  consequential, since it declares "this will never be implemented") would be an inconsistent, weaker
  guarantee than the one `remove` already enforces.
- **Rejection idempotency**: error (chosen, §9 item 6) vs. no-op-success on re-`reject`-of-`rejected`.
  Error was chosen because a second `reject` call on an already-rejected feature almost always signals
  an operator mistake (wrong slug, or forgetting the feature was already handled) rather than a genuine
  intent to re-record a rejection; an operator who wants to update the reason/evidence should `reopen`
  first (creating an audited history entry) and then `reject` again with the corrected fields, keeping
  every rejection's provenance intact rather than silently overwriting it.

## 13. Claims-Audit Appendix (mandatory per WP-001 §3.5)

Every load-bearing claim about current behavior in this PRD is anchored below. All citations verified
against commit `8574ff3` (Cluster F planning `WAVE_BASE`); the working HEAD at drafting time, `c6af5ce`,
differs from `8574ff3` only in `docs/handoff/CURRENT.md` and `docs/supervisor/LOG.md` (tracking-doc-only
diff, confirmed via `git diff --stat 8574ff3 HEAD`), so every source citation below applies unchanged at
both commits.

| Claim | Source | Verified at commit |
|---|---|---|
| Current `FeatureState` enum has exactly 9 values, none of which is `rejected`. | `internal/store/types.go:9-19` | `8574ff3` |
| `ValidFeatureState` is a closed switch over those 9 values. | `internal/store/types.go:21-27` | `8574ff3` |
| `FeatureStatus` (status.json) has no rejection-related fields today (`ID`, `Slug`, `Title`, `State`, `Compatibility`, `RequestedAt`, `UpdatedAt`, `LastCommand`, `Notes`, `Apply`, `Reconcile`, `DependsOn`, `Verify`). | `internal/store/types.go:188-230` | `8574ff3` |
| `tpatch status` applies no state-based filter — every feature from `ListFeatures()` is printed in both text and JSON paths. | `internal/cli/cobra.go:236` (load), `internal/cli/cobra.go:308-334` (JSON render loop), `internal/cli/cobra.go:362` (text render loop) | `8574ff3` |
| `statusCmd` registers `--json`, `--verbose`, `--feature`, and a DAG flag, but no rejection/inclusion flag. | `internal/cli/cobra.go:441-444` | `8574ff3` |
| `nextAction`'s state switch starts at `case store.StateRequested:` and has no terminal/rejected arm. | `internal/cli/phase2.go:400,410` | `8574ff3` |
| `RefreshFeaturesIndex` renders one flat table with columns Slug/Title/State/Compatibility and no state-based branching. | `internal/store/store.go:677-695` (table header/rows at `687-693`) | `8574ff3` |
| `removeCmd` does not inspect `status.State` before removing; it only gates on dependents via `checkRemoveDependents` and confirmation/`--force`. | `internal/cli/c1.go:341-395` | `8574ff3` |
| `checkRemoveDependents` refuses removal when dependents exist regardless of `--force`, citing "PRD §3.7" / ADR-011 D7. | `internal/cli/feature_deps.go:430-447` | `8574ff3` |
| `applyCmd` dispatches on `--mode` with no precondition check against `status.State`. | `internal/cli/cobra.go:634-693` | `8574ff3` |
| All lifecycle-phase transitions (`analyze`, `define`, `explore`, `implement`, `apply`, `record`, `land`, `reconcile`, `amend`, `remove`, `next`) are registered as top-level subcommands on `buildRootCmd`. | `internal/cli/cobra.go:60-80` | `8574ff3` |
| The `feature` subcommand group is reserved for noun-scoped per-feature management (`deps`, `claim`, `patch`), not lifecycle-state verbs, and its own doc comment explains the `<noun> <slug> <verb>` ordering choice. | `internal/cli/feature_deps.go:41-56` | `8574ff3` |
| `amend --state` is deliberately reserved (exit code 2) because "Lifecycle states are owned by other verbs". | `internal/cli/c1.go:276-284` (`validateAmendStateFlag`) | `8574ff3` |
| `DependencyKindSupersedes` is a distinct dependency-edge kind from `hard`/`soft`, used for replacement semantics (GH #1), unrelated to feature-level state. | `internal/store/types.go:296-299` | `8574ff3` |
| `Reconcile.ReviewVerdict` (including the `"rejected-upstreamed"` literal referenced by GH #4) is a free string on `ReconcileSummary`, a sub-object of `FeatureStatus` populated only after reconciliation runs. | `internal/store/types.go:313-318` | `8574ff3` |
| `RetirementAudit` (the post-implementation retirement mechanism GH #6 §9 alludes to) is a field on `workflow.ReconcileResult`, a runtime reconcile output — not a field on the persistent `store.FeatureStatus`. | `internal/workflow/reconcile.go:19,64-66` | `8574ff3` |
| `SaveFeatureStatus` is documented as the single writer that both persists `status.json` and refreshes `FEATURES.md`, the pattern this PRD's `reject`/`reopen` reuse. | `internal/store/store.go:363` | `8574ff3` |
| The `omitempty`/byte-identity contract for additive `FeatureStatus` fields (used here as migration precedent, §11) is explicitly documented for the `DependsOn` field. | `internal/store/types.go:207-215` | `8574ff3` |

18 claims audited (≥8 required).
