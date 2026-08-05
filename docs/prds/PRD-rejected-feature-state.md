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

- `tpatch reopen <slug> --note <string> [--evidence <path>...]` transitions `rejected → requested`. The
  reopen call is itself evidence-linked and audited (§6 `history[]`), matching the rejection's own
  evidentiary bar rather than being a bare state flip.

### 3.9

> 9. Refuse rejection from states where source changes are already applied unless the command also
>    performs a safe retirement/audit, or provide separate pre-implementation and post-implementation
>    retirement semantics.

- This PRD picks the **separate semantics** branch: `reject` is a **pre-implementation** lifecycle
  terminal only. It is allowed from `requested`, `analyzed`, `defined`, `explored` (§5). It is refused
  outright — no escape hatch — from `implementing`, `applied`, `active`, `reconciling`,
  `reconciling-shadow`, `blocked`, `upstream_merged`.
  > **Rev-1 correction (F-INT-2 BLOCKING)**: rev-0 named `tpatch reconcile confirm-upstreamed` +
  > `tpatch reconcile audit-retirement` as the "separate post-implementation semantics" this branch
  > defers to. That framing was wrong on two counts, both caught by internal review: (1) semantically,
  > `confirm-upstreamed` asserts "an implementation already exists upstream," the opposite verdict from
  > rejection's "this should never be implemented" — it is not a retirement mechanism for rejected work,
  > it is a *different* reconciliation outcome entirely. (2) empirically, `saveConfirmUpstreamedStatus`
  > (`internal/cli/cobra.go:2554-2562`) sets `status.State = store.StateUpstreamMerged` with **no
  > source-state guard**, so routing rejection through it would risk silently overwriting a rejection
  > record rather than safely retiring it. Post-implementation retirement of an already-implemented
  > feature is therefore **out of scope for Cluster F entirely**, not solved by an existing command. A
  > future, dedicated PRD/ADR pair — potentially `docs/prds/PRD-feature-unapply.md` (currently an
  > untracked draft in this repository) — may address it; this PRD makes no claim about which mechanism,
  > if any, eventually does. See ADR-031 D6 for the full rationale and the defense-in-depth guard added
  > to `confirm-upstreamed` itself as a result.
  >
  > **Rev-2 correction (F-INT-2 BLOCKING, guard placement)**: rev-1's guard was specified at
  > `saveConfirmUpstreamedStatus`. Rev-2 dual review traced the call chain and found
  > `applyConfirmUpstreamedTransition` (`internal/cli/cobra.go:2503-2552`) appends a `ReconcileRevision`
  > entry (`internal/store/store.go` via `AppendReconcileRevision`, called at `cobra.go:2535`) **before**
  > calling `saveConfirmUpstreamedStatus` (`cobra.go:2554-2562`). A guard placed only in the callee would
  > let a false audit revision get appended to a `rejected` feature before the guard ever fires. The
  > guard therefore moves to the **first statement of `applyConfirmUpstreamedTransition`**
  > (`cobra.go:2503`), ahead of both the append and the crash-recovery idempotency branch
  > (`isConfirmedViaReviewTransition` at `cobra.go:2511`, which also reaches
  > `saveConfirmUpstreamedStatus` directly). See ADR-031 D6 for the corrected call-chain trace.

## 4. CLI Shape

Two candidate forms were evaluated:

- **(a)** `tpatch reject <slug> --reason <code> --note <string> [--evidence <path>...] [--actor <string>]`
- **(b)** `tpatch feature state <slug> rejected --reason <code> ...`

**Chosen: form (a).**

> **Rev-2 correction (F-INT-CLI)**: the flag list above is corrected from rev-1's
> `--reason <code> --evidence <path> [--note <text>] [--related <ref>]`, which listed `--note` as
> optional. §6 requires `note` non-empty (validation error if missing), so `--note` is a **required**
> flag, not bracketed as optional; `--actor` is added explicitly (optional, §6's actor-precedence
> subsection) since it is a real, named flag in the precedence chain, not merely an implementation
> detail. `--related` remains available as an optional flag (omitted from the headline signature here
> for brevity, consistent with §6's field table, which is the authoritative field list) but was
> previously the *only* optional flag shown — rev-2 fixes the signature to show the flags that actually
> matter to invocation correctness: required `--reason`/`--note`, repeatable optional `--evidence`,
> optional `--actor`.

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
tpatch reopen <slug> --note <string> [--evidence <path>...] [--actor <string>]
```

`reopen` is a sibling top-level verb, symmetric with `reject`, following the same registration
convention. `--note` is required here too (all lifecycle mutations this PRD introduces get an operator
note, per GH #6's append-only-audit requirement — a reopen with no stated rationale would be exactly as
under-audited as a rejection with no stated rationale).

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
`blocked`, `upstream_merged`. See ADR-031 §"D6 — post-implementation escape hatch" for the full
rationale.
> **Rev-1 correction (F-INT-2 BLOCKING)**: rev-0 stated that `tpatch reconcile confirm-upstreamed` /
> `tpatch reconcile audit-retirement` already own the post-implementation retirement path and that
> `reject` defers to them. That is incorrect: `confirm-upstreamed` asserts the opposite verdict
> ("implementation already exists upstream," transitioning to `upstream_merged`) from rejection's
> "should never be implemented," and empirically `saveConfirmUpstreamedStatus`
> (`internal/cli/cobra.go:2554-2562`) has no guard preventing it from being invoked against a `rejected`
> feature at all. Post-implementation rejection is therefore **out of scope for Cluster F**, full stop —
> not solved by any existing command — and is deferred to a future PRD/ADR (see §3.9 above). As a
> defense-in-depth measure (not new PRD scope, but a Cluster F' implementation task), `confirm-upstreamed`
> itself gains a guard refusing to mutate a `rejected` feature (§7 below, ADR-031 D6).

**Reopen**: `rejected → requested` only. Append-only: the prior rejection record (`reason`, `evidence`,
`note`, `rejected_at`, `rejected_by`, `prior_state`) is never deleted — it is pushed onto `history[]`
(§6) and a fresh `requested` cycle begins. Reopen itself requires its own **mandatory** `--note`
(matching F-INT-CLI's requirement that every lifecycle mutation carries an operator rationale) and **may
include** additional `--evidence` (optional — if supplied, content-hashed exactly like at `reject` time
and appended to the same `history[]` entry). Reopen cycles are **unbounded** — an operator may reject and
reopen the same feature any number of times; each cycle appends its own `history[]` entry rather than
replacing the previous one (ADR-031 D5).
> **Rev-3 correction (F-INT-R2-3 HIGH)**: rev-2 text here said reopen "requires its own
> `--evidence`/`--note` pair," which contradicted §4's CLI signature (`--evidence` bracketed/optional)
> and §8's zero-evidence reopen examples. Rev-3 picks the **REOPEN-EVIDENCE-OPTIONAL** contract: `--note`
> is mandatory (the operator's stated reason for reopening is itself sufficient audit), `--evidence` is
> optional. A brief alternative — REOPEN-EVIDENCE-REQUIRED, mirroring `reject`'s own mandatory-evidence
> rule — was considered and rejected: reopening because "new context arrived" (e.g. an upstream caller
> reappeared, a policy changed) doesn't always correspond to a citable file, and forcing an operator to
> manufacture a throwaway evidence file just to satisfy the flag would degrade evidence quality overall
> rather than improve audit rigor.

### What happens to a rejected feature that other features depend on?

Three options were considered:

1. **Fail loudly and refuse the reject** — if any other feature's `depends_on` (hard or soft) lists this
   slug as a parent, `tpatch reject` errors out before writing anything, listing the dependent(s) and
   their kinds, and pointing at how to resolve (remove the offending edge from the dependent's own
   `depends_on` list).
   > **Rev-1 correction (F-INT-5 HIGH)**: rev-0's remediation text additionally suggested "or reject the
   > dependent too." Internal review proved this unsound: `dependentEdges`
   > (`internal/cli/feature_deps.go:170-186`) returns dependents regardless of the dependent's own
   > current state, so a listed dependent could itself already be `rejected`, `applied`, or in any other
   > state — "reject it too" is not a well-formed remediation in general and is removed from this PRD.
   > The only remediation this PRD specifies is removing the dependency edge itself.
2. **Warn and proceed** — print a warning listing dependents but still perform the rejection.
3. **Require `--force`** — same warning, but require an explicit flag to proceed.

**Chosen: Option 1 (fail loudly and refuse).** This mirrors the existing, already-shipped precedent for
`tpatch remove`: `checkRemoveDependents` (`internal/cli/feature_deps.go:430-447`) refuses removal
outright when dependents exist, regardless of `--force` ("`--force` is a TTY-confirmation override; it
does NOT bypass DAG integrity" — same file, doc comment on `removeCmd`, `internal/cli/c1.go:341,358-361`).
A feature being rejected is at least as disruptive to its dependents as a feature being deleted (its
parent's work is now formally "will never be implemented" rather than merely gone from disk), so the bar
for silently proceeding should be at least as high. Options 2 and 3 are recorded as alternatives in §12.

### Symmetric invariant: edge creation onto a rejected parent *(new, rev-1 fold, F-INT-5 HIGH)*

The above answers "can a rejected feature keep its existing dependents?" (no). Internal review found the
reverse direction was left open in rev-0: **can a *new* dependency edge be created pointing at a parent
that is already `rejected`?** Verified empirically that `ValidateDependencies`
(`internal/store/validation.go:113-210`, 6 existing rules, rev-2 correction of rev-1's "5 rules"/
`113-160` citation) has no rule today that would prevent this for any of the three
edge kinds (`hard`, `soft`, `supersedes`).

This PRD adds the symmetric invariant: **edge creation is refused if the proposed parent's `state` is
`rejected`**, for all three edge kinds, with no per-kind carve-out (ADR-031 D8 evaluates and rejects a
soft-only exception). The error message names the offending parent slug and its rejection reason, and
points at `tpatch reopen <parent-slug>` as the remediation if the dependency is still needed. This closes
the gap so that, regardless of which order an operator performs two independent operations (reject the
parent, then try to add an edge to it — or add the edge first, then try to reject the parent) the same
outcome results: a `rejected` feature never has live dependents, whichever order the two calls happen in.

## 6. Required Fields

All new fields live on `FeatureStatus` (`internal/store/status.json`), gated by `state: rejected` (an
eleventh value added to the existing ten-value `FeatureState` enum, `internal/store/types.go:9-19`).

| Field | Type | Validation |
|---|---|---|
| `state` | `FeatureState` enum, top-level | Must be `"rejected"`. Existing `ValidFeatureState` switch (`internal/store/types.go:21-27`) gains this arm. |
| `reason` | closed enum string | One of: `not-a-bug`, `premise-disproved`, `obsolete`, `out-of-scope`, `unsafe`, `duplicate`, `superseded`. Any other value is a validation error, exit code `2` (see ADR-031 D2 for closed-vs-open rationale; ADR-031 D4 addendum for the exit-code lock-in). |
| `evidence` | `[]EvidenceRef{path, sha256}` (list of hash-verified objects), normalized + deduplicated + sorted by `path` | At least one entry required. Each path must pass **path-safety** checks and be readable as a regular file so its content can be hashed (new, rev-2 fold, F-INT-1 BLOCKING + F-INT-3 HIGH — see below); a path failing either check is a validation error, exit code `2`. `sha256` is the lowercase-hex ASCII encoding of the raw SHA-256 digest (`^[0-9a-f]{64}$`, rev-3 fold, F4) — not uppercase hex, not base64. |
| `note` | `string` | Required, non-empty after trim. Free-form rationale. Missing or empty (after trim) is a validation error, exit code `2`. |
| `rejected_at` | `string` (RFC3339) | Set by the CLI at write time; not operator-supplied. |
| `rejected_by` | `string` (actor) | Resolved via a precedence chain, not operator-supplied ad hoc (rev-1 fold, F-INT-6 HIGH — see below). |
| `prior_state` | `FeatureState` enum | Captured automatically as whatever `state` was immediately before this `reject` call. |
| `related` | `string`, optional | Free-form: a feature slug or a `GH#N` reference. Not validated against the store (a related feature may not exist in this repo, e.g. a cross-repo GH issue). |
| `history` | `[]HistoryEntry`, append-only | Populated on `reopen` (and on every subsequent `reject`/`reopen` cycle). Each entry snapshots the fields above (`reason`, `evidence`, `note`, `rejected_at`, `rejected_by`, `prior_state`, `related`, plus the reopen's own `reopened_at`/`reopened_by`/`reopen_note`/`reopen_evidence`, and an `evidence_integrity` field with a per-element `divergent_reason` when divergent — see below) so no rejection record is ever overwritten — only appended past. Matches GH #6 §7's "append-only" requirement (§8 acceptance criteria: "Reopen is explicit and append-only"). Reopen/reject cycles are unbounded (ADR-031 D5). |

### Evidence integrity via content-hash snapshot *(rev-2 fold, F-INT-1 BLOCKING — replaces rev-1's path-restriction approach)*

> **Retraction notice**: rev-1's PRD text here restricted evidence to `.tpatch/features/<slug>/artifacts/`
> files, other non-forbidden feature-dir files, and git-tracked repo-root files, on the premise that
> `analysis.md`/`spec.md`/`exploration.md`/`implementation.md` were the *only* mutable files an operator
> could accidentally cite, and that `artifacts/` files were safe because "no phase command overwrites an
> existing artifact under a fixed name." **That premise is false.** Rev-2 dual review found
> `artifacts/analysis.json` (`internal/workflow/workflow.go:90`), `artifacts/apply-recipe.json`
> (`internal/workflow/implement.go:194,209`), and `artifacts/post-apply.patch`
> (`internal/cli/cobra.go:794,1398`, `internal/cli/phase2.go:158`, and the reconcile-refresh path) are
> all rewritten via `s.WriteArtifact` → `os.WriteFile`, a truncating write
> (`internal/store/store.go:442-453,785-790`). `docs/feature-layout.md:36` states this outright for
> `post-apply.patch`: "`tpatch record` writes this file on every invocation, **overwriting the previous
> contents**." Restricting evidence to "the artifacts directory" does not achieve immutability — it
> relocates the same hazard. This subsection is rewritten to verify evidence integrity via content
> hashing instead of assuming it from path location.

Internal review found rev-0's evidence-resolution rule permitted `analysis.md`, `spec.md`, and
`exploration.md` as evidence paths — but those exact files are **overwritten in place** on every
subsequent `tpatch analyze`/`define`/`explore` re-run (`internal/workflow/workflow.go:90-97` for
`analysis.md`, `:151-155` for `spec.md`, `:196-200` for `exploration.md`, each via
`s.WriteFeatureFile`). If a feature is rejected citing `analysis.md` as evidence, then later reopened and
re-analyzed, the historical rejection record would silently point at content that no longer reflects what
was actually reviewed at rejection time — violating GH #6 §1's "append-only audit history" guarantee at
the evidence layer, even though the `history[]` array itself is append-only at the *field* layer. Rev-2
resolves this by *detecting* drift rather than *forbidding* the paths where it can occur.

**Content-hash rule**:
- Each `evidence` entry is stored as `{path, sha256}`, not a bare path string.
- The CLI computes `sha256` at `tpatch reject` time from the resolved file's raw byte content — **after**
  path-safety validation below has passed.
- This rule applies **uniformly to every evidence path** — feature-directory files (including
  `analysis.md`/`spec.md`/`exploration.md`/`implementation.md`, now admissible), `artifacts/` files
  (including `analysis.json`/`apply-recipe.json`/`post-apply.patch`, now admissible), and git-tracked
  repo-root files alike. **There is no forbidden-filename list and no path-class exemption in rev-2** —
  every file's mutability risk is handled identically, by hashing it, not by an allowlist/denylist of
  filenames.
- `reject` fails (validation error, exit code `2`) if any evidence file cannot be hashed: the path does
  not exist, is not a regular file, is a symlink resolving outside the repository root, or is unreadable.

**Path-safety rule** (unchanged from rev-1, enforced *before* hashing):
- Absolute paths are rejected.
- Paths containing a `..` traversal segment (post-`filepath.Clean`) are rejected.
- Symlinks that resolve outside the repository root are rejected.
- Non-regular files (directories, sockets, devices, etc.) are rejected.
- All accepted paths are normalized (`filepath.Clean`, forward-slash separators) before being written to
  `status.json`, deduplicated after normalization, and sorted by `path` (`sort.Strings` semantics) for
  stable, deterministic serialization (CLAUDE.md rule 4).

**Reopen-time integrity check**: on `reopen`, for each historical `evidence` entry the CLI first
**re-runs the path-safety rule above** (canonicalize, confirm still inside the repo root, confirm still
a regular file, confirm no symlink escape) and, only if that re-check passes, recomputes the SHA-256 of
the entry's `path` and compares it against the hash recorded at rejection time.
- If path safety re-passes **and** the hash matches: the new `history[]` entry the reopen creates omits
  `evidence_integrity` (equivalent to `"verified"`).
- Otherwise, the reopen **still proceeds** (non-blocking) and the new `history[]` entry records
  `evidence_integrity: "divergent"` together with a **`divergent_reason`** *(new, rev-3 fold,
  F-INT-R2-2 HIGH)* per affected evidence element, drawn from a closed taxonomy:
  - `hash-mismatch` — path safety re-passed, still a regular in-repo file, but content hash differs.
  - `missing` — the path no longer resolves to any file.
  - `non-regular` — the path now resolves to something that is not a regular file (a directory, device,
    or socket) though it was a regular file at rejection time.
  - `path-safety-failed-at-reopen` — the path-safety re-check itself fails at reopen time (e.g. now
    resolves to an absolute path, a `..`-escaping path, or a symlink resolving outside the repository
    root) even though it passed at rejection time. **No hash is ever attempted in this case** — the
    file's bytes are never read, so a divergence entry here can never itself become an F-INT-3 violation.
  - `unreadable` — path safety re-passed and the file is still a regular in-repo file, but it cannot be
    opened (permission error).
  - A file that was deleted and later recreated with byte-identical content, at the same
    path-safety-passing location, naturally resolves back to `"verified"` via the hash comparison — no
    separate "was it deleted and restored" tracking is needed.

This is a deliberate choice (ADR-031 D3 addendum discusses two stricter alternatives — blocking the
reopen outright, or requiring a `--force`/acknowledgment flag — and explains why both were rejected in
favor of the non-blocking warn-and-record approach): the operator's past file edit, deletion, or
kind-change cannot be undone by refusing the reopen, and the persistent `evidence_integrity`/
`divergent_reason` record is itself the durable audit signal a stricter block would not add much beyond.
> **Rev-3 correction (F-INT-R2-2 HIGH)**: rev-2's text here read "if every hash matches (or the file is
> unreadable/missing... resolving identically)" — placing unreadable/missing evidence in the *same*
> branch as a verified match, contradicting the "divergent" branch immediately below it that also
> claimed unreadable/missing files. That was a genuine contradiction, not just imprecise wording:
> unreadable and missing files belong in exactly one branch (divergent), never both. This rewrite fixes
> it and additionally closes the file-kind-change gap rev-2 left open (a historical evidence path that
> was a regular file at rejection could be replaced by a directory, device, socket, or an
> externally-escaping symlink by reopen time — the last of which must never be hashed, since doing so
> would itself violate F-INT-3 path safety).

A deferred alternative — restricting evidence to a fixed set of "safe" locations (rev-1's approach) —
was tried and retracted (see the retraction notice above); ADR-031 D3 addendum records the full
rev-1-vs-rev-2 comparison for future-agent readability.

### Actor resolution precedence *(new, rev-1 fold, F-INT-6 HIGH)*

`rejected_by` (and the `history[]` entry's `reopened_by`) is resolved via a fixed precedence chain, not
free-form operator input:

1. `--actor <string>` CLI flag, if provided.
2. Else the `TPATCH_ACTOR` environment variable, if set and non-empty.
3. Else `git config user.email` read from the target repository, if configured.
4. Else the literal string `"unknown"`.

This does **not** derive from the git commit committer identity of whatever commit eventually carries
the change (that is orthogonal to Rule 18's `Co-authored-by`/`Copilot-Session` trailer convention, which
attributes commits, not status mutations) and does **not** auto-derive from the OS username or hostname
(privacy). Rev-0 incorrectly cited `internal/store/reconcile_revision.go`'s `ReconcileRevision` struct as
an existing actor-attribution precedent to follow; internal review verified that struct
(`internal/store/reconcile_revision.go:47-61`) has **no actor field and no timestamp field at all** — the
citation was false and is removed. See ADR-031 D9 for the full alternatives analysis.

## 7. Integration Semantics

- **`tpatch status`**: rejected features are excluded from the default (no-flag) listing and from the
  default JSON envelope's `features[]` array. A new `--include-rejected` boolean flag opts them back in
  (additive to the existing `--json`/`--verbose`/`--feature`/`--dag` flags on `statusCmd`,
  `internal/cli/cobra.go:226,441-444`). Today `statusCmd` has **no state-based filtering at all** — every
  feature returned by `s.ListFeatures()` (`internal/cli/cobra.go:236`) is printed in both the text loop
  (`internal/cli/cobra.go:362`) and the JSON `rendered` slice (`internal/cli/cobra.go:308-334`); this PRD
  is the first time `status` gains a state-based exclusion. The JSON envelope gains
  `rejection: {reason, evidence: [{path, sha256}, ...], note, rejected_at, rejected_by, prior_state,
  related}` (omitted/absent for non-rejected features) alongside the existing per-feature fields.
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
  error and exit code `3` (state-transition refusal, per ADR-031 D4's exit-code principle) (e.g.
  `feature "<slug>" is rejected (reason=<code>); run` `tpatch reopen <slug>` `to resume, or see`
  `<evidence>` `for rationale`). Today neither `applyCmd` (`internal/cli/cobra.go:634-693`) nor the
  reconcile entrypoint checks `status.State` against a terminal/refused list before proceeding — this
  PRD adds that check as a new precondition, matching the fail-fast style. Any future escape-hatch
  behavior for either command is explicitly deferred to ADR-031 §D6 and is NOT specified by this PRD.
- **`tpatch reconcile confirm-upstreamed` mutating-reconcile guard** *(rev-2 fold, F-INT-2 BLOCKING —
  guard placement corrected)*: independent of the general `apply`/`reconcile` refusal above,
  `confirm-upstreamed` specifically — and any future retirement-family reconcile variant — MUST guard
  against a `rejected` source state before mutating `status.State`, and the guard must run before **any**
  mutation, not just the final state write. Verified empirically that `applyConfirmUpstreamedTransition`
  (`internal/cli/cobra.go:2503-2552`) appends a `ReconcileRevision` entry (`cobra.go:2535`) **before**
  calling `saveConfirmUpstreamedStatus` (`cobra.go:2554-2562`, which today sets
  `status.State = store.StateUpstreamMerged` unconditionally with no prior-state check at all). A guard
  placed only in `saveConfirmUpstreamedStatus` (rev-1's placement) would let the false audit-revision
  append happen first. The guard therefore belongs at the **first statement of
  `applyConfirmUpstreamedTransition`** — ahead of both the append and the crash-recovery idempotency
  branch (`isConfirmedViaReviewTransition`, `cobra.go:2511`) — and must return a state-transition error
  (exit code `3`) before either mutation occurs. This is a Cluster F' **implementation task** (adding a
  precondition check at that call site), not new PRD-level design scope; see ADR-031 D6 for the full
  rationale and call-chain trace.
- **`tpatch remove`**: `removeCmd` (`internal/cli/c1.go:341-395`) does not inspect `status.State` at all
  today — it removes a feature in *any* state (subject only to the dependents gate,
  `internal/cli/feature_deps.go:430-447`, and the `--cascade`/confirmation flow). This PRD does **not**
  change that: a `rejected` feature is removable exactly like any other feature, with the same dependents
  gate applying. Rejection intentionally does not add extra remove-protection beyond what any other
  state already has — `rejected` preserves evidence by not being auto-deleted, not by becoming
  un-removable. An operator who explicitly wants to delete a rejected feature's directory (e.g. after
  exporting its evidence elsewhere) retains that ability, same as today.

## 8. JSON Envelope

> **Rev-1 correction (F-INT-4 HIGH)**: rev-0 used a bare `"exit_code": 1` for every error example below,
> which internal review flagged as a direct contradiction of this PRD's own multi-class error taxonomy
> (validation vs. state-transition). Every example below now uses the locked exit-code scheme from
> ADR-031's D4 addendum: `0` success, `1` unexpected internal error, `2` validation error, `3`
> state-transition error.
>
> **Rev-2 correction (F-INT-1 BLOCKING + F-INT-4 HIGH)**: two residual issues from rev-1 are fixed here.
> First, every `evidence`/`reopen_evidence` array below now shows `{path, sha256}` objects, not bare
> path strings, matching §6's content-hash redraft; the rev-1 "forbidden mutable artifact" error example
> is replaced with an "evidence cannot be hashed" example, since the admissibility restriction it
> illustrated is retracted. Second, the dependents-exist and `confirm-upstreamed`-refusal examples are
> corrected to exit code `3` (state-transition) — ADR-031 D4's general principle classifies both as
> post-validation checks against current store/graph state, not pre-mutation input validation, so
> labeling them "validation" in rev-1 was itself the contradiction rev-2 review caught.

`tpatch reject --json` success (exit code `0`):

```jsonc
{
  "slug": "claude-developer-instruction-preservation",
  "state": "rejected",
  "prior_state": "requested",
  "reason": "premise-disproved",
  "evidence": [
    {"path": "artifacts/live-role-probe.md", "sha256": "3b1c...e4f2"}
  ],
  "note": "Claude Chat already preserves developer instructions",
  "rejected_at": "2026-08-05T00:00:00Z",
  "rejected_by": "operator",
  "related": null,
  "exit_code": 0
}
```

`tpatch reject --json` error — missing evidence (validation error, exit code `2`):

```jsonc
{
  "error": "evidence required: at least one --evidence path must be supplied",
  "slug": "claude-developer-instruction-preservation",
  "exit_code": 2
}
```

`tpatch reject --json` error — invalid reason code (validation error, exit code `2`):

```jsonc
{
  "error": "invalid reason \"not-really-a-code\": must be one of not-a-bug, premise-disproved, obsolete, out-of-scope, unsafe, duplicate, superseded",
  "slug": "claude-developer-instruction-preservation",
  "exit_code": 2
}
```

`tpatch reject --json` error — missing/empty `--note` (validation error, exit code `2`, new example
per F-INT-8):

```jsonc
{
  "error": "note required: --note must be a non-empty rationale string",
  "slug": "claude-developer-instruction-preservation",
  "exit_code": 2
}
```

`tpatch reject --json` error — evidence cannot be hashed (validation error, exit code `2`, rev-2 fold,
F-INT-1 BLOCKING — replaces rev-1's "forbidden mutable artifact" example, which is retracted along with
the admissibility restriction; see §6 above):

```jsonc
{
  "error": "evidence path \"artifacts/missing-probe.md\" could not be read: no such file or directory",
  "slug": "claude-developer-instruction-preservation",
  "exit_code": 2
}
```

`tpatch reject --json` error — wrong state, e.g. `applied` (state-transition error, exit code `3`):

```jsonc
{
  "error": "cannot reject feature \"x\" from state \"applied\": reject is only valid from requested, analyzed, or defined",
  "slug": "x",
  "state": "applied",
  "exit_code": 3
}
```

`tpatch reject --json` error — dependents exist (state-transition error, exit code `3`, new example per
F-INT-8; rev-2 confirms this is exit `3` — a store/graph-state check, not input validation — per
ADR-031 D4's general principle):

```jsonc
{
  "error": "cannot reject feature \"x\": 1 dependent feature(s) still reference it: y (kind=hard)",
  "slug": "x",
  "dependents": [{"slug": "y", "kind": "hard"}],
  "exit_code": 3
}
```

`tpatch status --json` addition (per-feature, only present when `state == "rejected"`):

```jsonc
{
  "slug": "claude-developer-instruction-preservation",
  "state": "rejected",
  "rejection": {
    "reason": "premise-disproved",
    "evidence": [
      {"path": "artifacts/live-role-probe.md", "sha256": "3b1c...e4f2"}
    ],
    "note": "Claude Chat already preserves developer instructions",
    "rejected_at": "2026-08-05T00:00:00Z",
    "rejected_by": "operator",
    "prior_state": "requested",
    "related": null
  }
}
```

`tpatch reopen --json` success, no evidence divergence (exit code `0`):

```jsonc
{
  "slug": "claude-developer-instruction-preservation",
  "state": "requested",
  "reopened_at": "2026-08-06T00:00:00Z",
  "reopened_by": "operator",
  "reopen_note": "Upstream converter regained a caller in v1.4; re-evaluating.",
  "reopen_evidence": [
    {"path": "artifacts/upstream-v1.4-caller-diff.md", "sha256": "9a02...c771"}
  ],
  "history_entries": 1,
  "exit_code": 0
}
```

`tpatch reopen --json` success, evidence divergence detected (exit code `0` — non-blocking per D3's
Alternative 1, rev-2 fold, F-INT-1 BLOCKING; `divergent_reason` per-element, rev-3 fold, F-INT-R2-2
HIGH):

```jsonc
{
  "slug": "claude-developer-instruction-preservation",
  "state": "requested",
  "reopened_at": "2026-08-06T00:00:00Z",
  "reopened_by": "operator",
  "reopen_note": "Re-evaluating after the linked artifact was edited post-rejection.",
  "reopen_evidence": [
    {"path": "artifacts/upstream-v1.4-caller-diff.md", "sha256": "9a02...c771"}
  ],
  "history_entries": 1,
  "evidence_integrity": "divergent",
  "divergence_detail": [
    {"path": "artifacts/upstream-v1.4-caller-diff.md", "divergent_reason": "hash-mismatch"}
  ],
  "exit_code": 0
}
```

`tpatch reopen --json` success, note-only, zero `--evidence` supplied (exit code `0` — rev-3 fold,
F-INT-R2-3 HIGH, REOPEN-EVIDENCE-OPTIONAL contract):

```jsonc
{
  "slug": "claude-developer-instruction-preservation",
  "state": "requested",
  "reopened_at": "2026-08-06T00:00:00Z",
  "reopened_by": "operator",
  "reopen_note": "Policy changed; the original rationale no longer applies. No new artifact to attach.",
  "reopen_evidence": [],
  "history_entries": 1,
  "exit_code": 0
}
```

`tpatch reject --json` error — new dependency edge created onto a `rejected` parent (state-transition
error, exit code `3`, new example per F-INT-5 HIGH; this error is emitted by whichever command creates
the edge — `tpatch feature deps <slug> add <parent>` or `tpatch amend <slug> --depends-on <parent>`,
rev-2 correction: not `define --depends-on`, which does not exist — see ADR-031 D8):

```jsonc
{
  "error": "cannot add dependency: parent \"claude-developer-instruction-preservation\" is rejected (reason=premise-disproved); run `tpatch reopen claude-developer-instruction-preservation` first if this dependency is still needed",
  "slug": "y",
  "parent": "claude-developer-instruction-preservation",
  "kind": "hard",
  "exit_code": 3
}
```

`tpatch reconcile confirm-upstreamed --json` error — refused on a `rejected` feature (state-transition
error, exit code `3`, rev-2 fold, F-INT-2 BLOCKING + F-INT-4 HIGH — rev-1 mislabeled this a "validation"
case; it is a wrong-source-state refusal per ADR-031 D4's general principle):

```jsonc
{
  "error": "cannot confirm-upstreamed feature \"claude-developer-instruction-preservation\": feature is rejected (reason=premise-disproved); run `tpatch reopen claude-developer-instruction-preservation` first if this is no longer accurate",
  "slug": "claude-developer-instruction-preservation",
  "state": "rejected",
  "exit_code": 3
}
```

## 9. Tests-to-Write List

The implementation cluster (Cluster F') must write, at minimum:

1. Reject from `requested` — happy path.
2. Reject from `analyzed`, `defined`, `defined`-with-`exploration.md` (the three pre-implementation
   sub-states) — happy path per intermediate state.
3. Reject with missing evidence file → error, no state change (status.json byte-identical before/after
   the failed call), exit code `2`.
4. Reject with invalid reason code → error, no state change, exit code `2`.
5. Reject on non-existent slug → error.
6. Reject on already-rejected feature → error (PRD decision: error, not idempotent no-op — see §12).
7. Reject on `implementing`/`applied`/`active`/`reconciling`/`reconciling-shadow`/`blocked`/
   `upstream_merged` state → error in every case, exit code `3` (escape hatch, if any, is tested
   separately under a future ADR/PRD, not this one — note `reconciling-shadow` was missing from this
   list in rev-0 and is added here per F-INT-8).
8. Dependency effects: reject A while B declares `depends_on: [{slug: A, kind: hard}]` (or `soft`) →
   error, no state change, dependent(s) listed in the error message, exit code `3`.
9. Reopen from `rejected` → `requested`; verify `history[]` gains exactly one new entry and the prior
   rejection record (`reason`, `evidence`, `note`, `rejected_at`, `rejected_by`, `prior_state`) is
   byte-identical to what it was pre-reopen inside that history entry.
10. Reopen from any non-`rejected` state → error, exit code `3`.
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

**Additional tests, rev-1 fold (F-INT-8 MEDIUM + F-INT-1/3/5/6 follow-through)**:

17. Reject with missing/empty `--note` (empty string or whitespace-only after trim) → error, no state
    change, exit code `2`.
18. Reopen validation failures: reopen with missing evidence file → error, no state change; reopen with
    empty/missing `--note` → error; reopen from a non-`rejected` state (covered by item 10, expanded here
    to assert exit code `3` explicitly and that no `history[]` entry is appended on failure).
    > **Rev-2 split (F-INT-4 HIGH)**: item 18's sub-cases are not all the same exit-code class. "Missing
    > evidence file" and "empty/missing `--note`" are pre-mutation input validation → exit code `2`.
    > "Reopen from a non-`rejected` state" is a post-validation state-machine refusal → exit code `3`.
    > Item 25 below is corrected to reflect this split; rev-1's item 25 incorrectly folded all of item 18
    > into the exit-`3` bucket.
19. Multiple reopen cycles: reject → reopen → reject → reopen (at least 3 full cycles) → verify
    `history[]` grows by exactly one entry per cycle, unbounded, with no entry ever overwritten or
    truncated (ADR-031 D5 chooses unbounded).
20. Evidence containment/canonicalization edge cases (F-INT-3 HIGH): reject each of — absolute path
    (error), `..`-traversal path (error), symlink resolving outside the repo root (error), a directory
    path instead of a regular file (error), two evidence paths that normalize to the same path after
    `filepath.Clean` (deduplicated to one entry, no error), evidence paths supplied out of sorted order
    (persisted in sorted order regardless of input order).
21. Evidence integrity via content-hash (rev-2 fold, F-INT-1 BLOCKING — **replaces rev-1's admissibility
    tests**, which asserted `analysis.md`/`spec.md`/`exploration.md`/`implementation.md` were forbidden;
    that restriction is retracted in rev-2, see §6):
    - Reject citing `artifacts/apply-recipe.json` (previously forbidden under rev-1's restriction) as
      evidence → success; `sha256` recorded matches the file's content at reject time.
    - Reject with evidence → mutate the evidence file's content → reopen → verify the reopen succeeds
      (non-blocking) and the new `history[]` entry records `evidence_integrity: "divergent"` with
      `divergent_reason: "hash-mismatch"`.
    - Reject with evidence → delete the evidence file → reopen → verify the reopen succeeds and the new
      `history[]` entry records `evidence_integrity: "divergent"` with `divergent_reason: "missing"`.
    - Reject with evidence → no change to the file → reopen → verify the new `history[]` entry omits
      `evidence_integrity` (or records it as absent/`"verified"`), i.e. no false-positive divergence.
    - Reject with evidence → delete the evidence file → recreate it with byte-identical content → reopen
      → verify no divergence is recorded (the hash comparison subsumes the delete-then-recreate-identical
      case, per §6's design).
    - Reject citing an evidence file that cannot be hashed (missing, non-regular, symlink escaping the
      repo root, unreadable) → error, exit code `2`, no state change.
    - **File-kind-change tests, rev-3 fold, F-INT-R2-2 HIGH** (replaces rev-2's untested gap for
      divergence classes other than hash-mismatch/missing):
      - Reject with a regular-file evidence path → replace it with a symlink resolving outside the repo
        root → reopen → verify `evidence_integrity: "divergent"` with `divergent_reason:
        "path-safety-failed-at-reopen"`, and verify (via test instrumentation / a spy on the hashing
        helper) that **no read/hash attempt was made** against the escaping symlink target.
      - Reject with a regular-file evidence path → replace it with a directory → reopen → verify
        `divergent_reason: "non-regular"`.
      - Reject with a regular-file evidence path → make it unreadable (permission change) → reopen →
        verify `divergent_reason: "unreadable"`.
22. Dependency-order symmetry (F-INT-5 HIGH), both orders × all three edge kinds (`hard`, `soft`,
    `supersedes`): (a) reject parent P, then attempt to create a new edge from Q onto P → error, exit
    code `3`, no edge created; (b) create an edge from Q onto P first, then attempt to reject P → error,
    exit code `3`, no state change (already covered by item 8 for the `reject`-side case; this item adds
    the `soft`/`supersedes` kinds and the edge-creation-first ordering).
23. Actor precedence (F-INT-6 HIGH): `--actor` flag present → used verbatim, overriding everything else;
    no flag but `TPATCH_ACTOR` set → env value used; no flag/env but `git config user.email` configured
    → git value used; none of the three present → literal `"unknown"`; explicit precedence-order test
    asserting flag beats env beats git-config beats fallback when more than one is present
    simultaneously.
24. Direct `tpatch reconcile confirm-upstreamed <rejected-slug>` invocation (F-INT-2 BLOCKING defense-in-
    depth guard) → error, `status.State` remains `rejected`, rejection record fields unchanged, exit
    code `3`.
    > **Rev-2 addition**: also assert the feature's `ReconcileRevision` log
    > (`internal/store/reconcile_revision.go`) is unchanged after the refusal — not just `status.json`.
    > This closes the gap rev-2 review found: the append happens before the state check in rev-1's guard
    > placement, so a test that only checks `status.json` would miss a false revision entry sneaking in.
25. Exit-code assertions per validation class, exhaustively:
    - Exit code `2` (pre-mutation input validation): every scenario in items 3, 4, 17, 20, and the final
      sub-bullet of item 21 (evidence cannot be hashed), plus item 18's "missing evidence file" /
      "empty-or-missing `--note`" sub-cases.
    - Exit code `3` (post-validation state-machine refusal): every scenario in items 6, 7, 8, 10, 22, 24,
      plus item 18's "reopen from a non-`rejected` state" sub-case.
    - Exit code `0` (success, including non-blocking divergence): the happy-path scenarios (1, 2, 9,
      11-13, 19, 21's mutate/delete/no-change/recreate-identical/file-kind-change sub-bullets, and item
      26 below).
    > **Rev-2 correction (F-INT-4 HIGH)**: rev-1's item 25 assigned exit `3` to every item-18 case
    > uniformly. That was wrong — item 18 mixes a validation class (missing evidence/note) with a
    > state-transition class (wrong source state for reopen). This item is split accordingly, matching
    > item 18's rev-2 correction note above and ADR-031 D4's general principle (exit `2` = determinable
    > without consulting current store state; exit `3` = requires consulting it).

**Additional tests, rev-3 fold (F-INT-R2-3 HIGH); test 26 wording corrected + 26b added, rev-4 fold (F1
internal MEDIUM)**:

26. Reopen with zero `--evidence` supplied, only `--note` — verify success (exit code `0`); the new
    `history[]` entry has an empty/absent `reopen_evidence` list and `reopen_note` recorded exactly as
    supplied; **and**, critically, the *historical* evidence from the original rejection (which always
    exists — `reject` requires at least one evidence entry per §6, and the history entry retains its
    `{path, sha256}` refs) is still recomputed and verified per §6's reopen-time integrity check. In the
    clean case (historical evidence unmutated), verification passes silently and no `evidence_integrity`
    field is emitted. Confirms the REOPEN-EVIDENCE-OPTIONAL contract (§3.9, §5): only `--note` is
    mandatory for `reopen` — but historical-evidence verification is unconditional and runs regardless of
    whether new `--evidence` is attached.
    > **Rev-4 correction (F-INT internal MEDIUM)**: rev-3's wording here said "no `evidence_integrity`
    > required because nothing to diverge against," which was substantively wrong — historical evidence
    > from the original rejection always exists and is always re-verified on reopen, independent of
    > whether the operator attaches *new* evidence. The corrected wording above states plainly that the
    > absence of `evidence_integrity` in this scenario reflects a clean *verification result* against the
    > historical entry, not the absence of anything to verify. Test 26b immediately below locks in the
    > distinguishing case (historical evidence diverges but no new evidence was attached) so the two
    > integrity paths — "did the historical evidence hold up?" vs. "was new evidence attached?" — are
    > never conflated.
26b. Note-only reopen with mutated historical evidence (new, rev-4 fold, F1). Setup: `reject` citing
    evidence file `X`; mutate `X`'s content on disk; `reopen` with `--note "context revised"` and **no**
    `--evidence`. Verify: the reopen still succeeds (non-blocking, exit code `0`); the new `history[]`
    entry records `evidence_integrity: "divergent"` with `divergence_detail: [{path: X,
    divergent_reason: "hash-mismatch"}]` — even though zero new `--evidence` paths were supplied. This
    proves historical-evidence verification runs unconditionally on every reopen, orthogonal to whether
    the operator attaches new evidence in the same call.

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
  terminal outcomes and this PRD adds `rejected` as an eleventh, disjoint enum value, not a variant of
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
  rejected: the operator runs `tpatch reject <slug> --reason <code> --note <string> --evidence <path>`
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
| Current `FeatureState` enum has exactly 10 values, none of which is `rejected`. | `internal/store/types.go:9-19` | `8574ff3` |
| `ValidFeatureState` is a closed switch over those 10 values. | `internal/store/types.go:21-27` | `8574ff3` |
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
| `RetirementAudit` (the post-implementation retirement mechanism GH #6 §9 alludes to) is a field on `workflow.ReconcileResult`, a runtime reconcile output — not a field on the persistent `store.FeatureStatus`. Verbatim comment: "RetirementAudit exposes the cleanup audit triggered after upstreamed confirmation. Runtime/display only; status.json remains lifecycle truth." | `internal/workflow/reconcile.go:19,64-65` | `8574ff3` |
| `SaveFeatureStatus` is documented as the single writer that both persists `status.json` and refreshes `FEATURES.md`, the pattern this PRD's `reject`/`reopen` reuse. | `internal/store/store.go:363` | `8574ff3` |
| The `omitempty`/byte-identity contract for additive `FeatureStatus` fields (used here as migration precedent, §11) is explicitly documented for the `DependsOn` field. | `internal/store/types.go:207-215` | `8574ff3` |
| `analyze`/`define`/`explore` overwrite `analysis.md`/`spec.md`/`exploration.md` in place on every re-run via `s.WriteFeatureFile`, making path-location alone an unreliable immutability signal for rejection evidence (F-INT-1 BLOCKING). | `internal/workflow/workflow.go:90-97,151-155,196-200` | `8574ff3` |
| `WriteArtifact` writes every `artifacts/*` file (including `analysis.json`, `apply-recipe.json`, `post-apply.patch`) via `writeFile`, which calls the truncating `os.WriteFile` — proving rev-1's "artifacts/ is safe by directory location" premise false and motivating the rev-2 content-hash redesign (F-INT-1 BLOCKING). | `internal/store/store.go:442-453,785-790` | `8574ff3` |
| `docs/feature-layout.md` documents that `tpatch record` overwrites `post-apply.patch` on every invocation, corroborating the `WriteArtifact` truncation finding from the doc side. | `docs/feature-layout.md:36` | `8574ff3` |
| `applyConfirmUpstreamedTransition` appends a `ReconcileRevision` (via `store.AppendReconcileRevision`) before calling `saveConfirmUpstreamedStatus`, so a guard placed only in the callee would let a false audit revision get appended to a `rejected` feature first (F-INT-2 BLOCKING, guard-placement correction). | `internal/cli/cobra.go:2503,2510,2535,2554-2562` | `8574ff3` |
| `dependentEdges` returns a slug's dependents regardless of the dependent's own current state (no state filter), proving "reject the dependent too" is not a well-formed general remediation (F-INT-5 HIGH). | `internal/cli/feature_deps.go:170-186` | `8574ff3` |
| `ValidateDependencies` implements exactly 6 rules (self-dep, dangling ref, kind conflict, cycle detection, `satisfied_by`-requires-`upstream_merged`, and Rule 6 `ErrMultipleActiveSuperseders`) and has no rule today preventing edge creation onto a `rejected` parent (F-INT-5 HIGH, motivates new 7th D8 rule). | `internal/store/validation.go:113-210` (Rule 6 at `169-207`) | `8574ff3` |
| The real edge-creation CLI surfaces are `tpatch feature deps <slug> add <parent>[:kind]` and `tpatch amend <slug> --depends-on <parent>[:kind]` — `define --depends-on` does not exist as a flag anywhere in the codebase (F-INT-DEPS, rev-2 correction of a rev-1 fabricated citation). | `internal/cli/feature_deps.go:1-19,57-69` | `8574ff3` |
| `ReconcileRevision` has no actor field and no timestamp field at all, so it is **not** a valid actor-attribution precedent (rev-0's citation of it was false, corrected per F-INT-6 HIGH). | `internal/store/reconcile_revision.go:47-61` | `8574ff3` |

26 claims audited (≥8 required).

