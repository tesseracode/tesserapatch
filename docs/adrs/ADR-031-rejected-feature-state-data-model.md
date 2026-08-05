# ADR-031: Rejected Feature State Data Model

**Status**: Proposed
**Date**: 2026-08-05
**Deciders**: Copilot (Cluster F planning phase)
**Related**: GH #6, PRD-rejected-feature-state, PRD-confirm-upstreamed-human-review-path (orthogonal, cite specific lines — §4)

> **Numbering note**: the Cluster F dispatch brief (`docs/handoff/CURRENT.md`, `docs/supervisor/LOG.md`)
> and its originating task both call this ADR "ADR-028". At planning time
> `docs/adrs/ADR-028-supersession-edge-model.md` (Accepted, 2026-07-29) already occupies that slot — it
> covers GH #1's supersession edge model, an unrelated decision. This document takes the next free
> number, **ADR-031** (`docs/adrs/ADR-029-write-file-recipe-safety.md` and
> `docs/adrs/ADR-030-multi-slug-reconcile-derivation-mode.md` are also already occupied). Every
> cross-reference to "this ADR" elsewhere (PRD-rejected-feature-state.md, future Cluster F' work) should
> read ADR-031.

## 1. Context

PRD-rejected-feature-state §1-3 establishes the problem: tpatch's feature lifecycle has no terminal
state meaning "investigated, correct outcome is do-not-implement." Every current option destroys either
evidence (`remove`), backlog signal (permanent `requested`), or machine-readability (prose edits via
`amend`). GH #6 asks for a new `rejected` terminal state with a required reason code, evidence
reference(s), audit provenance, and an explicit evidence-linked reopen path.

This ADR makes the **data-model-level** decisions the PRD depends on but does not itself lock in:
where the rejection record physically lives, how closed the `reason` enum is, how `evidence` is
shaped and resolved, how the state machine is formalized (including exit codes), how `reopen`/history
is structured, whether a post-implementation escape hatch exists, and how backward compatibility with
pre-v0.13.0 `status.json` files is guaranteed.

## 2. Decision Points

- **D1**: Where does rejection state live? (`feature.yaml`/`status.json` field extension vs. sidecar
  file vs. append to a `history.json`)
- **D2**: Reason enum shape (closed enum vs. open string)
- **D3**: Evidence field format (single vs. list, path resolution order)
- **D4**: State machine formalization (transition graph, exit codes)
- **D5**: Reopen mechanism (append-only history vs. re-state replay; bounded vs. unbounded reopens)
- **D6**: Post-implementation escape hatch (does `reject` on `applied` state ever succeed? If yes, via
  what mechanism?)
- **D7**: Backward compatibility for pre-v0.13.0 `status.json`

## 3. Decisions

### D1: Rejection state location

**Alternative 1**: Extend `status.json`'s `FeatureStatus` struct — add `reason`, `evidence`, `note`,
`rejected_at`, `rejected_by`, `prior_state`, `related`, `history[]` fields directly alongside `State`.

Pros: single source of truth; `state` and its rejection metadata are read/written atomically by the same
`SaveFeatureStatus` call that already refreshes `FEATURES.md` (`internal/store/store.go:363`); no new
file to keep in sync; matches how every other lifecycle sub-record already lives on `FeatureStatus`
(`Apply`, `Reconcile`, `Verify`, `DependsOn` — `internal/store/types.go:200-230`).

Cons: mixes append-only audit data (`history[]`) with the otherwise mostly-current-value shape of
`status.json`; a growing `history[]` array on a repeatedly reject/reopen'd feature inflates every read
of `status.json`, which is on tpatch's hot path (every command loads it).

**Alternative 2**: Sidecar `.tpatch/features/<slug>/rejection.json` (and/or `history.json`).

Pros: separates append-only audit from the mostly-mutable-current-value `status.json`; keeps
`status.json` small and stable regardless of how many reject/reopen cycles a feature accumulates;
mirrors the existing `artifacts/patch-generations.json` precedent, which was deliberately kept out of
`status.json` for exactly this reason — "`status.json` is on the hot read path... folding unbounded
history into the same document inflates every read and forces every existing fixture to be [rewritten]"
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md`, D1 rationale).

Cons: a new file per feature to create, document (`docs/feature-layout.md`), and keep round-trip-safe;
two files (`status.json` + `rejection.json`) must agree on `state` — `status.json.state == "rejected"`
must imply `rejection.json` exists and vice versa, which is a new cross-file invariant to validate on
every load; adds a new artifact type to the parity guard's mental model even though `assets_test.go`
itself doesn't need to change (no skill text describes individual artifact files by name).

**Alternative 3**: Append to a feature-level `history.json` (create if absent) as a generic append-only
log, of which rejection/reopen would be one entry kind among others.

Pros: reuses a single generic append-only primitive if one already existed for other purposes, avoiding
a rejection-specific file.

Cons: **no such file exists today.** A repo-wide search
(`grep -rn "history.json" internal/ docs/` at `8574ff3`) returns zero matches — there is no existing
generic per-feature history log to append to. This alternative would require inventing a brand-new,
schema-generic audit-log primitive from scratch specifically to serve one feature (rejection), which is
strictly more new surface than Alternative 2's purpose-built sidecar, for no reuse benefit. The closest
existing append-only precedent, `reconcile-revisions.jsonl`
(`internal/store/reconcile_revision.go`, referenced throughout
`docs/prds/PRD-confirm-upstreamed-human-review-path.md`), is reconcile-scoped, not generic, and adding
rejection entries to it would violate the very orthogonality (§4) this ADR is required to preserve.

**Chosen**: **Alternative 1** (extend `FeatureStatus` directly), with one refinement: `history[]` only
ever grows on `reopen`, not on `reject` — the *current* rejection (if any) lives in the top-level
`reason`/`evidence`/`note`/`rejected_at`/`rejected_by`/`prior_state`/`related` fields, and a *past*
rejection is only pushed onto `history[]` at the moment it is superseded by a `reopen` call. A feature
that has never been rejected, or has been rejected exactly once and never reopened, carries zero
`history[]` growth — the array is empty (and `omitempty`) for the overwhelming majority of features.
Only features that cycle through reject→reopen→reject repeatedly accumulate `history[]` entries, and
that is an intentionally rare, deliberate-operator-action path, not a per-command write.

**Rationale**: CLAUDE.md rule 4 ("Prefer deterministic artifacts in `.tpatch/` over hidden session
state") favors keeping the authoritative state in the one place every command already loads
(`status.json`) rather than introducing a second file whose absence-vs-presence is itself a new
invariant to get wrong. PRD-confirm-upstreamed-human-review-path's own closest analog —
`ReconcileSummary` (`Reconcile.Outcome`, `Reconcile.ReviewVerdict`, etc.) — already lives directly on
`FeatureStatus` (`internal/store/types.go:313-318`), not in a sidecar, and that PRD explicitly treats
`status.json` as "the source of current truth post-accept" (ADR-010 D5, cited at
`docs/dependencies.md`'s "Authoritative source" section). Alternative 2's `patch-generations.json`
precedent is a poor match here because that artifact is genuinely unbounded and high-churn (one entry
per apply generation); a rejection record, by contrast, changes only on the rare, deliberate
`reject`/`reopen` actions — the "hot read path bloat" argument that justified pulling generations out of
`status.json` does not apply with anywhere near the same force to a handful of rejection fields.

**Consequences**: Cluster F' extends `internal/store/types.go`'s `FeatureStatus` struct with the eight
new fields from PRD §6 (all `omitempty` except `state`/`reason`/`evidence`/`note` when
`state == "rejected"`), updates `docs/feature-layout.md`'s `status.json` field list, and does NOT
introduce any new file under `.tpatch/features/<slug>/`.

### D2: Reason enum shape

**Alternative 1**: Closed enum in code — a `RejectionReason` type with a fixed `const` block
(`not-a-bug`, `premise-disproved`, `obsolete`, `out-of-scope`, `unsafe`, `duplicate`, `superseded`) and a
`ValidRejectionReason` switch, exactly mirroring `FeatureState`/`ValidFeatureState`
(`internal/store/types.go:5-27`).

Pros: matches the existing enum-validation convention in this codebase (`FeatureState`,
`DependencyKind*`, `CompatibilityStatus`); typo-proof at write time; `FEATURES.md` and `status --json`
consumers get a stable, finite vocabulary to build tooling against (e.g. dashboards grouping rejections
by reason).

Cons: adding a new reason code later requires a code change + release, not just an operator convention
update.

**Alternative 2**: Open string with a recommended-values doc (like `git commit --type` conventions in
some tools) — any non-empty string accepted, with the 7 GH #6 values documented as suggestions.

Pros: zero code changes needed to add a new reason in the field; maximal operator flexibility.

Cons: defeats the purpose of a reason *code* — `status --json` consumers (dashboards, the FEATURES.md
renderer, future duplicate-detection tooling per GH #6 §Expected-semantics item 7) can no longer rely on
a finite set; typos (`"premise_disproved"` vs `"premise-disproved"`) silently fragment the data with no
validation to catch them; directly contradicts this codebase's established pattern of closed enums for
every other state-like field (`FeatureState`, `DependencyKind*`).

**Alternative 3**: Closed enum in code, but with an explicit escape value (e.g. `other` or `custom:<text>`)
that always validates, deferring to `note` for detail.

Pros: gives a pressure-release valve for reasons the 7-item enum doesn't anticipate, without opening the
field to arbitrary strings.

Cons: an `other`/`custom` value is functionally identical to an open string for classification purposes
(every consumer still has to special-case it), while adding a second code path (parse `custom:<text>`)
for no real gain over just widening the real enum in a future PR when a genuine 8th case appears.

**Chosen**: **Alternative 1** (closed enum in code).

**Rationale**: Consistency with the existing `FeatureState`/`DependencyKind*` pattern
(`internal/store/types.go:5-27,296-299`) — this codebase's established convention is closed enums with
compile-time constants and a `Valid*` switch, not open strings, for every other state-classifying field.
GH #6's own proposed reason list (`not-a-bug`, `premise-disproved`, `obsolete`, `out-of-scope`, `unsafe`,
`duplicate`, `superseded`) is deliberately exhaustive-feeling and was arrived at by the issue author
already; extending it later (Alternative 1's stated con) is a small, low-risk PR — this repo already
has precedent for additive enum growth (e.g. `StateReconcilingShadow` was added to `FeatureState`
alongside the existing values, per ADR-010's M12 shadow-resolve work) with no migration hazard, since
`omitempty`/enum-widening is backward compatible by construction (an old binary simply never writes the
new value; a new binary reading an old file simply never sees it).

**Consequences**: Cluster F' adds a `RejectionReason` type + 7 constants + `ValidRejectionReason` switch
to `internal/store/types.go`, and `tpatch reject --reason <code>` validates against it before any write.

### D3: Evidence field format

**Alternative 1**: Single string field (`evidence: <path>`), one evidence reference per rejection.

Pros: simplest possible shape; matches the `--evidence <path>` singular flag GH #6's own example command
uses.

Cons: real-world rejections often accumulate multiple evidence artifacts (GH #6's own motivating example
already has 4 hand-authored files: `analysis.md`, `spec.md`, `exploration.md`,
`artifacts/live-role-probe.md`) — a single-string field would force operators to pick one "primary" file
and lose the others from the machine-readable record, undermining GH #6 §Expected-semantics item 7
("remain queryable for architecture history and duplicate detection").

**Alternative 2**: List of strings (`evidence: []string`), one or more paths, `--evidence` flag
repeatable.

Pros: matches the real shape of the motivating example; `--evidence` as a repeatable flag is a familiar
CLI convention already used elsewhere in this codebase for list-shaped input (e.g. `amend
--depends-on <parent> --depends-on <parent2>`, `docs/dependencies.md` "Declaring dependencies" §
`tpatch amend extra-button ... --depends-on button-component --depends-on polish-css:soft`).

Cons: slightly more validation surface (each list entry must resolve, not just one).

**Alternative 3**: Structured list of `{path, kind}` objects (e.g. `kind: "probe" | "analysis" |
"external-link"`), allowing evidence to be typed.

Pros: could distinguish "this is a live-measurement probe" from "this is a URL to an external tracker"
at the schema level.

Cons: GH #6 does not ask for typed evidence — its own reproduction just lists file paths
(`artifacts/live-role-probe.md`); adding a `kind` taxonomy here is speculative schema growth with no
concrete consumer identified in either the issue or the PRD's integration semantics (§7); the existing
`ValidationRef{Kind, Value, Result}` pattern (`internal/store/reconcile_revision.go:44-49`) already shows
what "we actually needed a typed ref" looks like when a real consumer (the reconcile revision log)
required it — no analogous consumer is identified for rejection evidence in this PRD.

**Chosen**: **Alternative 2** (list of plain path strings), with resolution order: first relative to the
feature directory (`.tpatch/features/<slug>/`), falling back to relative to the repository root if not
found there. A path resolving to neither location is a validation error at `reject`/`reopen` time.

**Rationale**: Matches the real evidentiary shape GH #6 itself demonstrates (multiple artifacts per
rejection) without inventing unrequested structure (Alternative 3). The repeatable-flag CLI convention
already exists in this codebase for exactly this shape of input
(`docs/dependencies.md`'s `--depends-on` repeatable-flag precedent), so `--evidence <path>` repeated N
times composes naturally and needs no new flag-parsing idiom. The dual-resolution-order (feature-dir
first, repo-root fallback) matches how the motivating example's evidence
(`artifacts/live-role-probe.md`) is feature-dir-relative, while still allowing a rejection to point at a
repo-level document (e.g. a shared research note under `docs/state-of-the-art/`) without forcing a copy
into the feature directory.

**Consequences**: Cluster F' implements `evidence []string` on `FeatureStatus`, a repeatable
`--evidence` cobra flag on both `reject` and `reopen`, and a path-resolution helper shared by both
commands' validation step.

### D4: State machine formalization

**Alternative 1**: No new formal transition table — `reject`/`reopen` each hard-code their own
allowed-from-state check inline, similar to how individual commands today informally rely on artifact
presence (`exploration.md` existing or not) rather than a declared transition graph.

Pros: minimal new code; matches the loosely-declarative style of `nextAction`'s existing switch
(`internal/cli/phase2.go:400`).

Cons: `reject`, `reopen`, and any future code path that needs to reason about "is this feature
reject-eligible" (e.g. a future `tpatch verify` check, or a linter) would each have to duplicate the
same list of allowed source states, risking drift.

**Alternative 2**: A small formal transition table (`map[FeatureState]bool` or an explicit
`RejectableStates` slice) shared by `reject`'s validation and any future consumer, with a distinct,
documented exit code for "refused: wrong source state" vs. "refused: validation error (missing
evidence/bad reason)".

Pros: single source of truth for "which states can be rejected"; matches the existing
`ExitCodeError` convention (`internal/cli/exit_error.go`) already used to distinguish `tpatch verify`'s
exit 2 from generic exit 1 — this ADR proposes the same treatment for `reject`/`reopen` refusals so
scripts can distinguish "your reason code was wrong" from "this feature can't be rejected right now"
programmatically.

Cons: one more small shared symbol to maintain; marginal added indirection versus Alternative 1's inline
checks.

**Alternative 3**: Fully generic state-machine engine (declared transition graph with guards, evaluated
by a shared dispatcher used by every phase command, not just `reject`/`reopen`).

Pros: would unify `reject`/`reopen` with the rest of the phase lifecycle's implicit transitions under one
mechanism.

Cons: massive scope increase — every existing phase command (`analyze`, `define`, `explore`, `implement`,
`apply`) would need to be retrofitted onto the new engine to make it non-redundant, which is explicitly
out of scope for this PRD/ADR pair (planning-only, no code) and unrelated to GH #6's actual ask; the
existing codebase has never used a generic state-machine abstraction for its 9 existing states and
introducing one now, solely to add a 10th, is disproportionate.

**Chosen**: **Alternative 2** (small formal transition table + reused `ExitCodeError` convention).

**Rationale**: `ExitCodeError` (`internal/cli/exit_error.go:1-34`) already exists precisely for this
purpose — distinguishing a binding non-1 exit code for a specific command from the legacy generic exit-1
default — and is already used by `tpatch verify` (per its own doc comment, "currently `tpatch verify`
per PRD-verify-freshness §6 Q7"). Reusing rather than inventing a second convention keeps `reject`/
`reopen` consistent with the one precedent this codebase already has for "this refusal is categorically
different from a generic error." A small explicit table (Alternative 2) is proportionate to a single new
enum value and avoids Alternative 3's scope creep, while still preventing the drift risk Alternative 1
would introduce once more than one call site needs to ask "is this feature reject-eligible."

**Consequences**: Cluster F' defines the reject-eligible state set (`requested`, `analyzed`, `defined`)
as a shared symbol in `internal/store`, uses it from both `reject`'s validation and (if needed) a future
`tpatch verify`/lint check, and returns a distinct `ExitCodeError` code for "wrong source state" versus
plain `error` for "validation failure" (missing evidence / invalid reason / dependents blocking) —
exact code numbers to be assigned at implementation time, documented in `SPEC.md` alongside the existing
exit-code contract for `verify`.

### D5: Reopen mechanism

**Alternative 1**: Append-only `history[]` array (as decided under D1), unbounded number of reopen
cycles.

Pros: matches GH #6 §Expected-semantics item 8 ("explicit evidence-linked reopen") and §7's acceptance
criterion ("Reopen is explicit and append-only") directly; no artificial cap to hit in a legitimately
long-lived feature that gets reconsidered multiple times over a project's life.

Cons: a pathological feature reject/reopen-looped many times accumulates an ever-growing `history[]`,
though — per D1 — this is an intentionally rare, deliberate-operator-action path, not a per-command
write, so unbounded growth here is not comparable to unbounded growth on a hot-write path like
`patches/NNN-*.patch` (`docs/feature-layout.md`'s own "when `patches/` exceeds six files... a one-line
reminder" precedent shows this codebase already tolerates unbounded-but-rare growth without a hard cap).

**Alternative 2**: Full re-state replay — `reopen` doesn't append a history entry at all; it simply
clears the rejection fields and resets `state` to `requested`, relying on `git log`/external tooling to
recover the old rejection rationale from a prior commit of `status.json` if needed.

Pros: simplest possible implementation — no `history[]` field needed at all.

Cons: **directly contradicts GH #6 §Expected-semantics item 1** ("Preserve the complete feature
directory and **append-only audit history**") and §7's acceptance criterion ("Reopen is explicit and
**append-only**"). Also silently assumes `status.json` is tracked in git and that operators reliably
commit it after every tpatch command — neither is guaranteed (the file lives under `.tpatch/`, which
per `SPEC.md` §9 is explicitly excluded from patch artifacts, and nothing in the current codebase
requires committing `.tpatch/` state after every command). This alternative is a non-starter against
GH #6's explicit ask and is included only to document why it was considered and discarded.

**Alternative 3**: Bounded reopen count (e.g. cap at N reopens, refuse the N+1th with an error directing
the operator to `remove` and re-`add` instead).

Pros: caps worst-case `history[]` growth to a known bound.

Cons: introduces an arbitrary, unmotivated limit with no basis in GH #6 (which specifies no such cap) or
in any existing tpatch convention (no other append-only structure in this codebase — `patches/`,
`reconcile-revisions.jsonl` — enforces a hard reopen-style cap; `patches/` only nudges with a soft
reminder, never a refusal). A hard cap would also create a confusing failure mode where a legitimately
long-lived, frequently-revisited feature suddenly can't be reopened for reasons unrelated to its actual
state.

**Chosen**: **Alternative 1** (append-only `history[]`, unbounded).

**Rationale**: GH #6 §7's acceptance criteria are explicit and binding: "Reopen is explicit and
append-only." Alternative 2 directly violates that criterion and is discarded outright. Alternative 3's
bound is unmotivated by anything in GH #6, the PRD, or existing codebase precedent (`docs/feature-layout.md`'s
`patches/` growth-reminder precedent shows this codebase's established pattern for "this could grow
large" is a soft nudge, not a hard refusal) — Cluster F' should follow that same soft-nudge precedent if
`history[]` growth ever becomes a real operator concern, rather than pre-emptively capping a field GH #6
never asked to be capped.

**Consequences**: Cluster F' implements `history []HistoryEntry` with no enforced maximum length; if a
future cluster observes `history[]` growing large in practice (mirroring the `patches/` precedent), a
soft print-a-reminder nudge — not a hard cap — is the natural next step, and is out of scope for this
ADR.

### D6: Post-implementation escape hatch

**Alternative 1**: No escape hatch — `reject` is refused unconditionally from `implementing` onward
(`implementing`, `applied`, `active`, `reconciling`, `reconciling-shadow`, `blocked`,
`upstream_merged`), full stop. Post-implementation retirement remains exclusively the domain of
`tpatch reconcile confirm-upstreamed` / `tpatch reconcile audit-retirement`.

Pros: keeps `reject`'s scope tight and unambiguous — it is a **pre-implementation** lifecycle terminal,
full stop, matching GH #6's own framing that this issue is "related to, but distinct from, #4" and that
#4 "concerns a rejected/confirmed **upstreamed reconciliation candidate**," not a general lifecycle
outcome. No new interaction surface between `reject` and the retirement-audit machinery
(`internal/workflow/retirement_audit.go`) needs to be designed, tested, or maintained.

Cons: an operator who discovers post-implementation that a feature should never have been applied (e.g.
an already-`applied`-but-never-`active` feature whose premise collapses) has no single-command path;
they must go through whatever the *existing* recovery path already is (manual revert / `remove` /
future retirement tooling), which this ADR does not change or improve.

**Alternative 2**: `reject` gains a `--force-post-implementation` (or similarly gated) flag that, when
set on an `implementing`/`applied`/`active` feature, also triggers the same retirement-audit path
`confirm-upstreamed` uses (`AuditRetirement`, `internal/workflow/retirement_audit.go:29`), effectively
merging `reject` and post-implementation retirement into one command with two modes.

Pros: a single command surface (`reject`) for the operator to remember regardless of feature state.

Cons: **directly reproduces the exact conflation GH #6 explicitly asks to avoid.** GH #6's own
§Distinction-from-related-concepts section draws the line precisely at "rejected-upstreamed (#4): review
verdict about an upstreamed reconciliation candidate, not a general feature lifecycle outcome" — merging
that verdict-producing machinery into `reject`'s own flag set would make `reject` simultaneously a
pre-implementation lifecycle terminal AND a reconciliation-verdict command, the very ambiguity §4 of
this ADR is written to foreclose. It would also require `reject` to understand
`ReconcileSummary.{Outcome,ReviewVerdict}` state (today read only by `reconcile`-family commands,
`internal/cli/cobra.go:2107-2138`), doubling its blast radius for a use case GH #6 itself frames as
belonging to a different issue.

**Alternative 3**: A separate, new command (e.g. `tpatch retire <slug>` or an extension to
`confirm-upstreamed`'s sibling command set) handles "mark an implemented/applied feature as
should-never-have-shipped," entirely independent of `reject`. `reject` itself stays exactly as
Alternative 1 describes (refused unconditionally past `implementing`).

Pros: preserves `reject`'s narrow pre-implementation scope (like Alternative 1) while still leaving a
door open for a future, explicitly-scoped post-implementation "this shouldn't have shipped" command,
without polluting `reject`'s own flag surface.

Cons: that command does not exist today and is not requested by GH #6 (which only asks that
`reject` "refuse rejection from states where source changes are already applied **unless the command
also performs a safe retirement/audit, or provide separate pre-implementation and post-implementation
retirement semantics**" — GH #6 itself offers the "separate semantics" branch as an explicitly
acceptable resolution, which is exactly Alternative 1). Designing a whole new command is out of scope
for this PRD/ADR pair and would be premature without a concrete driving use case.

**Chosen**: **Alternative 1** (no escape hatch; `reject` is refused unconditionally past `implementing`).

**Rationale**: This is the "separate pre-implementation and post-implementation retirement semantics"
branch GH #6 §Expected-semantics item 9 itself offers as an acceptable resolution
(verbatim: "...or provide separate pre-implementation and post-implementation retirement semantics").
Post-implementation retirement is not a gap tpatch has — it already exists and was purpose-built by
PRD-confirm-upstreamed-human-review-path: `tpatch reconcile confirm-upstreamed` mutates
`status.Reconcile.{Outcome,ReviewVerdict,UpstreamCommit}` and `status.State` "under the same guardrails
as automated confirmation" and "runs the retirement audit established by
PRD-reconcile-retirement-state-audit" (`docs/prds/PRD-confirm-upstreamed-human-review-path.md` §Summary,
2nd paragraph). Extending `reject` to also drive that same audit (Alternative 2) would create two
command surfaces that both claim ownership of retiring an implemented feature, contradicting the
single-writer discipline this codebase already enforces elsewhere (e.g. "Only the `verify` and `amend`
(recipe-touching) verbs may rewrite [`Verify`]" — `internal/store/types.go:220-230`). See §4 below for
the full orthogonality argument this decision rests on.

**Consequences**: Cluster F' implements `reject`'s state check as an unconditional refusal for every
state at or past `implementing` — no flag, no exception. If a genuine future need for a
post-implementation "should never have shipped" command emerges, it is Alternative 3's separate command,
decided by a future, dedicated ADR — explicitly **not** this one.

### D7: Backward compatibility for pre-v0.13.0 `status.json`

**Alternative 1**: All new fields (`reason`, `evidence`, `note`, `rejected_at`, `rejected_by`,
`prior_state`, `related`, `history`) are `omitempty`-tagged and simply absent from every pre-v0.13.0
file; a pre-v0.13.0 file loads with zero values for all of them and `state` never equals `"rejected"`
for such a file (since the value didn't exist to write before this change).

Pros: zero migration code required; matches this codebase's established `omitempty` discipline for every
prior additive field (`DependsOn`, `Verify`, `Labels`, `PatchIDMatch` — all documented `omitempty` for
exactly this reason, e.g. `internal/store/types.go:207-215`'s explicit byte-identity requirement for
`DependsOn`).

Cons: none identified specific to this field set — this is the default, low-risk path every prior
additive change in this codebase has taken.

**Alternative 2**: A migration hint/fallback, similar to the D10 `patch-generations.json` absent-file
fallback precedent from Cluster D Item 1 — e.g. `tpatch status`/`tpatch reject` prints a one-time notice
the first time it encounters a pre-v0.13.0 file that predates the `rejected` state, suggesting operators
review their `blocked`/stale-`requested` backlog for candidates.

Pros: could nudge operators toward adopting the new state on existing backlog items.

Cons: unmotivated by anything in GH #6 or the PRD — there is no schema *ambiguity* to resolve (unlike
the Cluster D Item 1 case, which existed because `patch-generations.json` could be legitimately absent
*or* malformed and the reader needed to distinguish those); a `rejected`-state field set is either
present (only ever written by this version forward) or absent (every prior file), with no fallback
decision to make. Adding a proactive notice here would be scope creep beyond what backward-compat
strictly requires.

**Alternative 3**: A version-gated behavior flag (like `Config.FeaturesDependencies` /
`Config.DAGEnabled()`, `docs/dependencies.md` "Available: v0.6.0+. Default: on. Toggle via
`features_dependencies: true|false`") that must be explicitly enabled before `reject`/`reopen` work at
all, defaulting to off for pre-existing repos.

Pros: gives operators an explicit opt-in switch, matching the DAG feature's rollout precedent.

Cons: the DAG feature needed a flag because its *default apply-time gating behavior* was a breaking
change to existing repos' `apply` semantics the moment it was turned on (`docs/dependencies.md`'s
"Migration from v0.5.x" section is explicit: "apply behaviour matches v0.5.3 exactly" only when the flag
stays default/off). `reject`/`reopen` introduce **no new default-on gating behavior for existing
features** — a pre-v0.13.0 repo's existing features are simply never in `state: rejected` (nothing
retroactively changes their behavior), and the new commands are opt-in by construction (an operator must
explicitly invoke `tpatch reject`). There is no equivalent breaking-default-behavior risk to gate behind
a config flag.

**Chosen**: **Alternative 1** (plain `omitempty`, no fallback hint, no feature flag).

**Rationale**: This mirrors every prior additive `FeatureStatus` field in this codebase exactly —
`DependsOn`'s own doc comment states the byte-identity requirement explicitly ("`omitempty` is
load-bearing — when the flag is OFF and no deps are declared, status.json must round-trip byte-for-byte
identical to pre-M14.1 fixtures," `internal/store/types.go:207-215`) and `Verify`'s doc comment states
the same for its own rollout ("The pointer is `omitempty`-marshalled so v0.6.1 fixtures that never run
verify round-trip byte-identical," `internal/store/types.go:220-224`). Unlike the DAG feature
(Alternative 3's precedent), `reject`/`reopen` change no default behavior for features that never call
them — there is nothing to gate. Unlike the D10 patch-generations fallback (Alternative 2's precedent),
there is no ambiguous-vs-malformed distinction to resolve for an absent rejection field set — absence
unambiguously means "never rejected."

**Consequences**: Cluster F' tags every new `FeatureStatus` field `omitempty`, adds a round-trip fixture
test asserting a pre-v0.13.0 `status.json` (no rejection fields) parses and re-serializes byte-identical
(matching the existing round-trip test pattern in `internal/store/roundtrip_test.go`), and introduces no
new config flag.

## 4. Orthogonality with PRD-#4 (confirm-upstreamed)

This is the section GH #6, the PRD, and the Cluster F dispatch brief all treat as load-bearing: `rejected`
(this ADR) and confirm-upstreamed's retirement machinery (PRD-#4) must operate on **orthogonal data**,
not two views of the same mechanism.

- **`RetirementAudit` is a runtime reconciliation output, not persistent feature state.** It is defined
  as a field on `workflow.ReconcileResult`:

  ```go
  // internal/workflow/reconcile.go:19
  type ReconcileResult struct {
      ...
      // internal/workflow/reconcile.go:64-66
      // RetirementAudit exposes the cleanup audit triggered after upstreamed
      // confirmation, when the retirement audit ran as part of this call.
      RetirementAudit *RetirementAuditReport `json:"retirement_audit,omitempty"`
  }
  ```

  `ReconcileResult` is the in-memory return value of one `RunReconcile`/`confirm-upstreamed` invocation —
  it is not itself persisted to `status.json`. The *effects* of a retirement audit (cleanup revisions)
  are appended to the reconcile-revision log via `AppendRetirementCleanupRevisions`
  (`internal/workflow/retirement_audit.go:147`), a structure entirely separate from `FeatureStatus`.

- **`store.FeatureStatus` is the persistent feature state** (`internal/store/types.go:188-230`) — the
  one object every command loads via `LoadFeatureStatus`/`SaveFeatureStatus`. Its `Reconcile
  ReconcileSummary` sub-object (`internal/store/types.go:313-...`) is the *persisted* record of the most
  recent reconcile attempt's outcome/verdict — but note carefully: `ReconcileSummary` is populated only
  **after** `tpatch reconcile` has run at least once, and it describes a reconciliation verdict about
  already-existing patched code, never a pre-implementation lifecycle decision.

- **The two mechanisms operate on orthogonal data**:
  - `rejected` (this ADR): a **top-level `FeatureState` enum value**, reached from
    `requested`/`analyzed`/`defined` — i.e. **before** any recipe or patch exists
    (`ApplySummary.HasRecipe`/`HasPatch` are both still false). It carries its own dedicated fields
    (`reason`, `evidence`, `note`, etc.) that have no equivalent anywhere in `ReconcileSummary`.
  - `RetirementAudit`/`confirm-upstreamed` (PRD-#4): operates exclusively on features already in
    `implementing`/`applied`/`active`/`reconciling` territory, where `Reconcile.Outcome` has already been
    computed by an actual reconcile pass against upstream Git history. Its retirement-confirmed check
    explicitly requires `status.State == store.StateUpstreamMerged` (not `rejected`) plus
    `Reconcile.Outcome == store.ReconcileUpstreamed`
    (`internal/workflow/retirement_audit.go:38`, cited verbatim in
    `docs/prds/PRD-confirm-upstreamed-human-review-path.md`'s own Claims Audit table).
  - No code path reads `FeatureStatus.rejected*` fields to decide a reconcile verdict, and no code path
    reads `Reconcile.Outcome`/`ReviewVerdict` to decide reject-eligibility (D4's reject-eligible state
    set is drawn purely from `State`, never from `Reconcile`). The two data shapes never intersect.

- **This orthogonality is by design, not by accident of naming.** GH #6 itself states: "This is related
  to, but distinct from, #4. That issue concerns a rejected/confirmed **upstreamed reconciliation
  candidate**. This issue concerns a normal feature request rejected as not-a-bug or no-longer-needed
  before implementation." D6 above (no post-implementation escape hatch inside `reject`) is the concrete
  mechanism that keeps this orthogonality real rather than aspirational: `reject` never touches
  `Reconcile.*`, and `confirm-upstreamed`/`AuditRetirement` never touch the new rejection fields.

- **If future work wants to unify these two mechanisms** (e.g. a shared "why did this feature not ship"
  query surface spanning both `rejected` features and `upstream_merged`-with-retirement-audit features),
  **that is a separate ADR** — not this one. This ADR's scope is limited to making `rejected` a correct,
  orthogonal addition to the existing state machine; it explicitly does not attempt, and should not be
  read as attempting, any such unification.

## 5. Migration Path

- **Pre-v0.13.0 `status.json` files**: no `rejected`-related field is present. They are treated exactly
  as they are today — `state` is one of the existing 9 values, and `ValidFeatureState` continues to
  accept them unchanged (this ADR only *adds* a 10th valid value; it removes none). No auto-migration
  runs.
- **Post-v0.13.0 CLI reading pre-v0.13.0 files**: silent OK, no warning printed. This matches D7's
  Alternative 1 choice and the codebase's established `omitempty` backward-compat convention (see D7
  rationale citations).
  the D10 `patch-generations.json` absent-file fallback pattern (Cluster D Item 1) is **not** invoked
  here — that pattern exists to distinguish "absent" from "malformed" for a file whose *presence* is
  itself ambiguous without a version marker. Rejection fields have no such ambiguity: absence
  unambiguously means "this feature has never been rejected." No fallback-hint code is warranted (D7
  Alternative 2, rejected above).
- **Post-v0.13.0 CLI writing**: the new fields are written only when `tpatch reject` (or `tpatch
  reopen`) is explicitly invoked on a given slug. No other command (`analyze`, `define`, `apply`, etc.)
  writes any rejection-related field, ever.

## 6. Consequences

**Positive**:
- Measurement-first-engineering evidence (like the motivating example's `live-role-probe.md`) is
  preserved and becomes machine-discoverable, closing the exact gap GH #6 raises.
- Backlog signal is cleaned: `tpatch status`'s default view stops presenting permanently-`requested`
  features that were actually already resolved as "do not implement."
- No new file format, no new config flag, no breaking change to any existing command's default output
  for repos that never call `reject`.

**Negative**:
- New CLI surface (`reject`, `reopen`) and new JSON envelope fields (`rejection` sub-object on `status
  --json`, plus `reject --json`/`reopen --json` shapes) mean a new test matrix (PRD §9, 16 items) and a
  new documentation surface (`SPEC.md` state table, `docs/feature-layout.md` field list,
  `docs/dependencies.md` — if a rejected feature is ever a dependency parent, see open question below).
- `FeatureStatus` grows by 8 fields (7 top-level + `history[]`), all `omitempty`, so the common case
  (never-rejected feature) has effectively zero marginal read/parse cost, but the struct's total field
  count and documentation burden increases.

**Open questions to defer** (to the implementation cluster or a future ADR, not this one):
- Exact `rejected_by` actor-derivation mechanism (environment variable? git config user.name? a new
  `--actor` flag echoing existing conventions elsewhere?) — D6/PRD §6 leaves this to Cluster F'
  implementation detail, since no existing tpatch command has a strong precedent for actor attribution
  beyond timestamps.
- Whether a `rejected` parent feature should be allowed as a *soft* dependency parent for ordering-only
  purposes (the PRD's chosen "fail loudly" default in §5 covers *any* dependent, hard or soft — this ADR
  does not revisit that PRD-level choice, but a future refinement distinguishing hard-vs-soft dependents'
  treatment of a rejected parent is left open).
- Exact numeric exit codes for `reject`/`reopen` refusals (D4) — this ADR mandates *that* a distinct
  `ExitCodeError` is used, not the specific integer value(s); Cluster F' should pick values that don't
  collide with `tpatch verify`'s existing exit-2 contract and document them in `SPEC.md`.

## 7. Implementation Notes (for the F' cluster)

**Files to touch (best guess)**:
- `internal/store/types.go` — add `FeatureState` value `StateRejected`; add `RejectionReason` type +
  constants + `ValidRejectionReason`; extend `FeatureStatus` with the 8 new fields (D1); add
  `HistoryEntry` struct (D5).
- `internal/store/status.go` (or wherever `LoadFeatureStatus`/`SaveFeatureStatus` load/save the struct —
  currently `internal/store/store.go`) — no structural change expected beyond what the `FeatureStatus`
  field additions already cover, since (D1) the save path is reused unchanged.
- `internal/cli/cobra.go` (or a new dedicated `internal/cli/reject.go` file, matching this codebase's
  convention of one file per medium-sized command cluster, e.g. `feature_deps.go`, `land.go`,
  `reconcile_check_applied.go`) — add `rejectCmd()`/`reopenCmd()`, register both on `buildRootCmd`
  (`internal/cli/cobra.go:60-80`); extend `statusCmd()` with `--include-rejected` and the default
  exclusion filter (`internal/cli/cobra.go:226-457`); extend `nextAction` (`internal/cli/phase2.go:400`)
  with a `case store.StateRejected:` arm; add the dependents-fail-loudly check (reusing/adapting
  `checkRemoveDependents`'s pattern, `internal/cli/feature_deps.go:430-447`); add the `rejected`-state
  precondition check to `applyCmd` (`internal/cli/cobra.go:634-693`) and the reconcile entrypoint
  (`internal/cli/cobra.go:1887`, `reconcileCmd`).
- `internal/store/store.go` — extend `RefreshFeaturesIndex` (`internal/store/store.go:677-695`) with the
  distinct "Rejected" trailing section and default-exclusion from the main table.
- `assets/` — no skill-text change is anticipated to be strictly required for the parity guard
  (`assets_test.go` checks that skill docs mention *current CLI commands*; whether `reject`/`reopen`
  need explicit mention is an implementation-time call, but if any skill enumerates the full command
  list by name, it must be updated in lockstep or the parity guard will fail).
- `SPEC.md` — add `rejected` to the Feature States table (§3), add `reject`/`reopen` to the CLI Commands
  table (§4), document the new exit-code contract (D4) alongside the existing `verify` exit-2 precedent.
- `docs/feature-layout.md` — document the 8 new `status.json` fields under "State & debug files".
- `docs/dependencies.md` — cross-reference the "fail loudly on rejected parent" behavior (PRD §5) if it
  materially changes any documented dependents-gate example.

**Test files** (from PRD §9): new `internal/cli/reject_test.go` (or similarly named) covering all 16
enumerated scenarios; a new `internal/store` round-trip fixture test for pre-v0.13.0 byte-identity (D7);
extensions to existing `internal/cli` status/next/FEATURES.md render tests for the inclusion/exclusion
behavior.

**Do NOT touch**: `internal/workflow/reconcile.go`, `internal/workflow/retirement_audit.go` — those are
confirm-upstreamed's (PRD-#4's) territory, orthogonal per §4 above. `reject`/`reopen` must not read or
write `ReconcileSummary` in any form.
