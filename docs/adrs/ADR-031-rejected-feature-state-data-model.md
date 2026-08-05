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
- **D8** *(rev-1 fold, F-INT-5)*: Dependency edge creation guard against rejected parents — can a new
  `depends_on` edge (`hard`/`soft`/`supersedes`) be created pointing at an already-`rejected` parent?
- **D9** *(rev-1 fold, F-INT-6)*: Actor provenance mechanism for `rejected_by`/`reopened_by`

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

**Consequences**: Cluster F' implements `evidence []string` on `FeatureStatus` — **superseded by the D3
addendum below; the shipped contract is `evidence []EvidenceRef{Path, SHA256}`, not a bare string
slice** (rev-3 fold, F1) — via a repeatable `--evidence` cobra flag on both `reject` and `reopen`, and a
path-resolution helper shared by both commands' validation step.

#### D3 addendum — evidence integrity via content-hash snapshot (rev-2 fold, F-INT-1 BLOCKING — replaces rev-1's path-restriction approach)

> **Why rev-1's approach is retracted, for future-agent readability**: rev-1 attempted to solve F-INT-1
> by *restricting* which paths could be cited as evidence — forbidding `analysis.md`/`spec.md`/
> `exploration.md`/`implementation.md` and admitting anything under `.tpatch/features/<slug>/artifacts/`
> on the premise that artifact files were "append-only-by-convention... no phase command overwrites an
> existing artifact under a fixed name." **That premise is false**, and rev-2's dual review (both
> internal and external, independently) found the counter-evidence: `RunAnalyze` writes
> `artifacts/analysis.json` via `s.WriteArtifact` (`internal/workflow/workflow.go:90`, truncating —
> `WriteArtifact` → `writeFile` → `os.WriteFile`, `internal/store/store.go:442-453,785-790`);
> `RunImplement` writes `artifacts/apply-recipe.json` the same way on every re-run
> (`internal/workflow/implement.go:194,209`); and `artifacts/post-apply.patch` is rewritten by no fewer
> than four call sites (`internal/cli/cobra.go:794,1398`, `internal/cli/phase2.go:158`,
> and the reconcile-refresh path) — this repository's *own documentation* says so explicitly:
> `docs/feature-layout.md:36` states "`tpatch record` writes this file on every invocation, **overwriting
> the previous contents**." Restricting evidence to "the artifacts directory" therefore does not achieve
> immutability at all — it only relocates the same overwrite hazard rev-0's `analysis.md`/`spec.md`/
> `exploration.md` finding identified. Rather than chase an ever-growing exclusion list of "which
> specific filenames happen to be safe this release," this addendum is rewritten to adopt the
> content-hash approach internal review originally recommended at rev-0 and rev-1 (previously overridden
> here on machinery-cost grounds) — evidence *integrity is verified*, not assumed from path location.

**Evidence integrity rule** (new validation + storage shape, both `reject` and `reopen`):

- Each `evidence` entry is stored as a structured object, not a bare path string:
  ```json
  {"path": ".tpatch/features/foo/artifacts/apply-recipe.json", "sha256": "<64-hex>"}
  ```
- The `sha256` is computed by the CLI, at `tpatch reject` time, over the resolved file's raw byte
  content — **after** path-safety validation (below) has passed, so a hash is never computed for a
  rejected/unsafe path.
- This rule applies **uniformly to every admissible evidence path** — feature-directory files, files
  under `artifacts/`, and git-tracked repo-root files alike. There is **no path-class exemption or
  forbidden-filename list**: rev-1's exclusion of `analysis.md`/`spec.md`/`exploration.md`/
  `implementation.md` is dropped, and `apply-recipe.json`/`post-apply.patch` are likewise now
  admissible. Any file's mutability is handled the same way — by hashing it — rather than by trying to
  enumerate which filenames are "safe."
- `reject` fails (validation error) if any evidence file cannot be hashed: the path does not exist, is
  not a regular file, is a symlink resolving outside the repository root, or is unreadable (permission
  error). This is the same failure class rev-1's admissibility check occupied, just re-armed against
  "can we read and hash these bytes right now" instead of "is this filename on an allowlist."
- **Encoding** (rev-3 fold, F4/external L2): `sha256` is the lowercase-hex ASCII encoding of the raw
  SHA-256 digest — `^[0-9a-f]{64}$` — matching Go's `encoding/hex.EncodeToString` default (lowercase),
  not uppercase hex and not base64. This is pinned explicitly to eliminate implementer ambiguity.

**Path safety** (F-INT-3, unchanged from rev-1, still enforced *before* hashing): evidence paths are
rejected outright if they are absolute, contain a `..` traversal segment, resolve (after symlink
evaluation) to a location outside the repository root, or resolve to a non-regular file. Every accepted
path is normalized (`filepath.Clean`, forward slashes), deduplicated after normalization, and the stored
`evidence` list is sorted by path (`sort.Strings` on the `path` field) for deterministic serialization
(CLAUDE.md rule 4).

**Reopen-time integrity check** — three alternatives considered:

**Alternative 1 (chosen)**: On `reopen`, for each historical evidence entry, **first re-run the F-INT-3
path-safety checks** (canonicalize, verify the resolved path is still inside the repository root, verify
it is still a regular file, verify no symlink escape) before attempting to hash anything; **then**, only
if path safety still passes, recompute the SHA-256 and compare against the hash recorded at rejection
time. If the path safety re-check fails, or the hash differs, or the file is missing/unreadable, **warn**
(non-fatal) and record `evidence_integrity: "divergent"` — with a `divergent_reason` per affected entry
(see taxonomy below) — on the new `history[]` entry the reopen creates; do not block the reopen itself.
If path safety re-passes and every hash matches, record `evidence_integrity: "verified"` (or omit the
field entirely, treated as verified by absence).

> **Rev-3 fold (F-INT-R2-2 HIGH)**: rev-2's addendum specified divergence handling only for
> hash-mismatch and missing-file. It did not say what happens when an evidence path's **file kind**
> changes between reject-time and reopen-time — e.g. a historical evidence path that was a regular file
> at rejection is later replaced by a directory, a device/socket, or (most importantly) a symlink that
> now resolves outside the repository root. Rehashing an externally-escaping symlink at reopen time would
> itself violate F-INT-3 path safety — the reopen-time check must never read bytes from outside the
> repo, even to compute a divergence hash. Rev-3 closes this gap: **path safety is re-run at reopen time,
> before any hashing is attempted**, and a path that fails it is recorded as divergent with a
> distinguishing reason rather than either (a) silently attempting to hash it (unsafe) or (b) blocking
> the reopen outright (contradicts the chosen non-blocking Alternative 1 behavior).

**`divergent_reason` taxonomy** (new, rev-3 fold, F-INT-R2-2 HIGH): each divergent evidence element in a
`history[]` entry carries exactly one of:

| `divergent_reason` | Meaning | Hash attempted? |
|---|---|---|
| `hash-mismatch` | Path safety re-check passed; still a regular in-repo file; content hash differs from the value recorded at rejection. | Yes |
| `missing` | Path no longer resolves to any file (deleted). | No |
| `non-regular` | Path now resolves to something that is not a regular file (directory, device, socket) — was a regular file at rejection. | No |
| `path-safety-failed-at-reopen` | Path safety re-check itself fails at reopen time (e.g. now an absolute path, a `..`-escaping path, or a symlink resolving outside the repository root) though it passed at rejection time. **No hash is ever attempted in this case** — the file is never read. | No |
| `unreadable` | Path safety re-check passed and the file is still a regular in-repo file, but it cannot be opened/read (permission error). | No |

Pros: the operator's file edit (or deletion, or kind-change) already happened in the past and cannot be
un-done by `tpatch reopen` refusing to run — blocking would strand the feature in `rejected` with no way
forward except manually editing `status.json`, which this PRD/ADR pair otherwise avoids requiring. The
divergence record itself becomes the auditable signal: a future reader of `history[]` sees exactly which
reopens had trustworthy evidence, which did not, and *why* (via `divergent_reason`) — which is arguably
*more* informative than a hard failure that gives no persistent trace of the problem having occurred.

Cons: an operator could still reopen (and thus effectively "un-reject", i.e. resume implementation work)
on top of evidence that no longer supports the original rejection review, without an enforced pause.

**Alternative 2**: Reject the `reopen` call outright (validation error, no state change) if any
historical evidence entry's hash no longer matches current file content, or if its file kind has changed.

Pros: strictly stronger integrity guarantee — an operator cannot proceed past a reopen without first
either restoring the original evidence file or explicitly acknowledging the mismatch through some other
command.

Cons: this is the "stranding" failure mode Alternative 1's Cons section warns against: the original
evidence file drift is very often *itself* the reason an operator wants to reopen (e.g. "the analysis
this rejection cited turned out to have been edited/rewritten since, let's re-examine") — refusing the
reopen precisely when evidence has diverged actively blocks the operator from investigating the
divergence via the tool itself, forcing an out-of-band `status.json` edit that undermines the very
audit trail this feature exists to preserve.

**Alternative 3**: Require an explicit `--force` (or `--acknowledge-divergence`) flag on `reopen` when
divergence is detected, otherwise behave like Alternative 2.

Pros: a middle ground — divergence is not silently waved through (Alternative 1's Con), but the operator
is not permanently stuck (Alternative 2's Con) since a flag unblocks them.

Cons: adds a second reopen code path and a second flag surface for a warning-class condition; Alternative
1 already achieves "not silently waved through" via the persistent `evidence_integrity: "divergent"`
record (now with a `divergent_reason`) in `history[]` — a flag adds friction without adding information
the record doesn't already capture. Deferred as unnecessary machinery unless a future ADR finds
Alternative 1's non-blocking warning insufficient in practice.

**Chosen: Alternative 1** (warn + record `evidence_integrity: divergent` with a `divergent_reason`,
non-blocking).

**Rationale**: This matches this ADR's own general philosophy elsewhere (D6: no escape-hatch machinery
until an operator need is demonstrated; D8: symmetric fail-loudly only where the *forward* operation
itself would be unsound, not merely suspicious) — a reopen is not, by itself, unsound merely because some
cited evidence changed; it is the operator's own call whether stale evidence still justifies a reopen,
and the tool's job is to make that staleness visible and permanently recorded, not to adjudicate it.

**Missing-file / non-regular / path-safety-failure vs. hash-mismatch handling**: if a historical evidence
path no longer exists at all (deleted, not merely edited), or now resolves to a non-regular file, or now
fails the F-INT-3 path-safety re-check, the reopen-time check cannot (and in the path-safety-failure case
must not attempt to) compute a hash to compare, and records `evidence_integrity: "divergent"` with the
matching `divergent_reason` from the taxonomy above — there is no separate "missing" *state* distinct
from "divergent", only a distinct *reason* recorded alongside it, since from an audit-record perspective
"cannot verify" and "verified different" carry the same practical consequence (the historical evidence
claim can no longer be confirmed), while the reason still tells a future reader *why* verification
failed. If an operator deletes a file and later re-creates it — as a regular, in-repo, path-safe file —
with byte-identical content, the hash comparison naturally resolves to `"verified"`: the hash-match check
already subsumes this case without any special-cased "was it deleted and restored" logic.

**Consequences (addendum)**: Cluster F' implements evidence as `[]EvidenceRef{Path, SHA256 string}` on
`FeatureStatus` (replacing rev-1's planned `[]string`), a shared hashing helper invoked by `reject`
(hash-at-write-time) and `reopen` (hash-at-verify-time, re-running F-INT-3 path safety before every
hash attempt per the rev-3 fold above), and the path-safety/normalization/dedup/sort pipeline unchanged
from rev-1 but now gating hash computation rather than an admissibility allowlist.
PRD §9 gains tests for: evidence mutated before reopen (divergence recorded), evidence deleted before
reopen (divergence recorded), evidence unchanged before reopen (no divergence flag), and a
previously-forbidden path (`artifacts/apply-recipe.json`) now accepted as evidence.

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
existing codebase has never used a generic state-machine abstraction for its 10 existing states and
introducing one now, solely to add an 11th, is disproportionate.

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
`tpatch verify`/lint check, and returns a distinct `ExitCodeError` code for "wrong source state or
dependency-graph refusal" (exit `3`, D4 addendum) versus "input validation failure" (exit `2`: missing
evidence / invalid reason / missing note) — see the D4 addendum below for the full, locked classification
and the general principle it states.

#### D4 addendum — exit codes locked (rev-1 fold, F-INT-4 HIGH; corrected rev-2, F-INT-4 residuals)

Rev-0 left the exact exit-code values as "to be assigned at implementation time," which internal review
correctly flagged as a PRD/ADR contradiction risk: PRD §8's JSON envelope examples all used a bare
`"exit_code": 1` for every error class, which would collapse the very distinction this decision exists to
make. This ADR now locks the codes so the PRD's envelope examples (§8) and Cluster F' implementation
cannot drift apart:

**General principle** (rev-2 addition, to prevent the classification drift rev-1 left in three places):
exit `2` is **pre-mutation input validation** — the operator supplied something malformed or unresolvable
*before* the store's current state is even consulted (schema-level: missing/unreadable evidence file,
invalid `--reason` enum value, missing/empty `--note`, evidence path-safety violation). Exit `3` is
**post-validation state-machine refusal** — the input was well-formed, but the *current state of the
store* (this feature's own state, another feature's dependency on it, or another feature's own state)
makes the requested transition invalid right now (wrong source state, dependents block a reject,
dependency-edge-creation blocked by a rejected parent, already-rejected, reopening a non-rejected
feature, confirm-upstreamed refused on a rejected source). Apply this principle consistently across
every `reject`/`reopen`/dependency-editing/`confirm-upstreamed` code path — see the table below and the
corrections at D6 and D8 that this principle drove.

| Code | Meaning | Example triggers |
|---|---|---|
| `0` | Success | `reject`/`reopen` completed and wrote `status.json`. |
| `1` | Unexpected internal error | Filesystem I/O failure, unhandled panic recovery, store load failure unrelated to the slug itself. Matches the existing legacy generic-error convention every other command already uses. |
| `2` | Validation error (pre-mutation input validation) | Missing or unreadable evidence path (cannot be hashed — D3 addendum), invalid `--reason` enum value, missing/empty `--note`, evidence path-safety violation (D3 addendum: absolute path, `..` traversal, symlink escape, non-regular file). |
| `3` | State-transition error (post-validation state-machine refusal) | Wrong source state (rejecting from `implementing` or later — D6); rejecting a feature that has live dependents (PRD §5); rejecting an already-`rejected` feature; reopening a non-`rejected` feature; creating a dependency edge (`hard`/`soft`/`supersedes`, via `tpatch feature deps <slug> add` or `tpatch amend --depends-on`) onto a `rejected` parent (D8); invoking `tpatch reconcile confirm-upstreamed` against a `rejected` feature (D6 defense-in-depth guard). |

This reuses the `ExitCodeError` convention (`internal/cli/exit_error.go:1-34`) exactly as Alternative 2
proposed. Codes `2` and `3` are new, purpose-specific values, distinguished from `tpatch verify`'s
existing exit-2 contract simply by being scoped to their own commands' own callers — `ExitCodeError`
already supports per-command code assignment (it is a plain `{Code int; Message string}` pair,
`internal/cli/exit_error.go:11-14`), so no collision with `verify`'s own exit-2 usage is possible; each
command's own exit-2 value means whatever that command's own contract says it means, same as any other
Unix tool's per-command exit-code convention.
> **Rev-2 correction**: rev-1's text here additionally claimed exit codes `2`/`3` were "scoped to
> `reject`/`reopen` only." That claim is retracted — D8 requires exit `3` from dependency-*editing*
> commands (`tpatch feature deps <slug> add`, `tpatch amend --depends-on`) refusing an edge onto a
> `rejected` parent, and D6's defense-in-depth guard requires exit `3` from `tpatch reconcile
> confirm-upstreamed` refusing a `rejected` source — neither of those is `reject` or `reopen` itself.
> The exit-code *meanings* (validation vs. state-transition) are shared across every command this PRD/ADR
> pair touches; only the *command surface* differs per triggering scenario, per the table above.
`SPEC.md`'s exit-code documentation (Cluster F' scope) should state this scoping explicitly to avoid an
operator assuming a single global exit-code enum across all `tpatch` subcommands.

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

> **Rev-1 rewrite (F-INT-2 BLOCKING)**: rev-0's D6 proposed `tpatch reconcile confirm-upstreamed` /
> `tpatch reconcile audit-retirement` as the "separate post-implementation retirement semantics" this
> decision defers to. Internal review correctly identified two independent problems with that framing:
> (1) it is **semantically backwards** — `confirm-upstreamed` asserts "an implementation of this feature
> already exists upstream," the exact opposite verdict from `rejected`'s "this should never have been
> implemented at all"; routing a rejection through it would misrecord *why* the feature has no local
> patch. (2) it is **empirically unguarded** — `saveConfirmUpstreamedStatus`
> (`internal/cli/cobra.go:2554-2562`) unconditionally sets `status.State = store.StateUpstreamMerged`
> with no check of the feature's prior state at all:
> ```go
> // internal/cli/cobra.go:2554-2562
> func saveConfirmUpstreamedStatus(s *store.Store, status *store.FeatureStatus, info *confirmUpstreamedTransition) error {
>     status.Reconcile.Outcome = store.ReconcileUpstreamed
>     status.Reconcile.ReviewVerdict = "confirmed-upstreamed"
>     status.Reconcile.UpstreamCommit = info.UpstreamCommit
>     status.State = store.StateUpstreamMerged
>     ...
> }
> ```
> A `rejected` feature that somehow acquired a confirming revision (e.g. operator error, or a future
> automation bug) could be silently flipped to `upstream_merged` — permanently erasing the rejection
> record's meaning with no source-state guard in the way. Both problems are corrected below by **removing
> the escape-hatch proposal entirely** rather than patching it, since GH #6 itself explicitly allows this
> resolution (see Alternative 1 and Chosen, unchanged in spirit from rev-0 but now correctly scoped).

**Alternative 1**: No escape hatch — `reject` is refused unconditionally from `implementing` onward
(`implementing`, `applied`, `active`, `reconciling`, `reconciling-shadow`, `blocked`,
`upstream_merged`), full stop. Post-implementation retirement of an already-shipped feature is **out of
scope for Cluster F entirely** — neither `reject` nor any existing command is asked to solve it here.

Pros: keeps `reject`'s scope tight and unambiguous — it is a **pre-implementation** lifecycle terminal,
full stop, matching GH #6's own framing that this issue is "related to, but distinct from, #4" and that
#4 "concerns a rejected/confirmed **upstreamed reconciliation candidate**," not a general lifecycle
outcome. No new interaction surface between `reject` and any reconcile-family machinery
(`internal/workflow/reconcile.go`, `internal/workflow/retirement_audit.go`) needs to be designed, tested,
or maintained. Crucially, this alternative also does **not** claim `confirm-upstreamed` is a valid
substitute path — it makes no claim about *any* existing command being the answer, closing the semantic
mismatch F-INT-2 identified.

Cons: an operator who discovers post-implementation that a feature should never have been applied (e.g.
an already-`applied`-but-never-`active` feature whose premise collapses) has no single-command path in
this release; they must go through whatever the *existing* recovery path already is (manual revert /
`remove` / a future dedicated command), which this ADR does not change or improve.

**Alternative 2**: `reject` gains a `--force-post-implementation` (or similarly gated) flag that, when
set on an `implementing`/`applied`/`active` feature, also triggers the retirement-audit path
`confirm-upstreamed` uses (`AuditRetirement`, `internal/workflow/retirement_audit.go:29`), effectively
merging `reject` and post-implementation retirement into one command with two modes.

Pros: a single command surface (`reject`) for the operator to remember regardless of feature state.

Cons: **directly reproduces the exact conflation GH #6 explicitly asks to avoid, and additionally
asserts the wrong verdict.** GH #6's own §Distinction-from-related-concepts section draws the line
precisely at "rejected-upstreamed (#4): review verdict about an upstreamed reconciliation candidate, not
a general feature lifecycle outcome" — merging that verdict-producing machinery into `reject`'s own flag
set would make `reject` simultaneously claim "do not implement this" (its own verdict) AND "an
implementation already exists upstream" (`AuditRetirement`'s precondition, `retirement_audit.go:38`
requires `Reconcile.Outcome == store.ReconcileUpstreamed`) — two verdicts that cannot both be true of the
same feature. It would also require `reject` to read/write `ReconcileSummary` fields (today touched only
by `reconcile`-family commands), doubling its blast radius for a use case GH #6 itself frames as
belonging to a different issue.

**Alternative 3**: A separate, new command (e.g. `tpatch retire <slug>`, or the scope currently sketched
by the untracked WIP `docs/prds/PRD-feature-unapply.md`) handles "mark an implemented/applied feature as
should-never-have-shipped," entirely independent of `reject`. `reject` itself stays exactly as
Alternative 1 describes (refused unconditionally past `implementing`).

Pros: preserves `reject`'s narrow pre-implementation scope (like Alternative 1) while still leaving a
door open for a future, explicitly-scoped post-implementation "this shouldn't have shipped" command,
without polluting `reject`'s own flag surface or reusing a semantically-mismatched existing verb.

Cons: that command does not exist today and is not requested by GH #6 (which only asks that
`reject` "refuse rejection from states where source changes are already applied **unless the command
also performs a safe retirement/audit, or provide separate pre-implementation and post-implementation
retirement semantics**" — GH #6 itself offers the "separate semantics" branch as an explicitly
acceptable resolution, which is exactly Alternative 1's no-escape-hatch shape). Designing a whole new
command is out of scope for this PRD/ADR pair and would be premature without that command's own PRD/ADR
pair settling its own data model.

**Chosen**: **Alternative 1** (no escape hatch; `reject` is refused unconditionally past `implementing`;
post-implementation retirement is explicitly out of scope for Cluster F).

**Rationale**: This is the "separate pre-implementation and post-implementation retirement semantics"
branch GH #6 §Expected-semantics item 9 itself offers as an acceptable resolution
(verbatim: "...or provide separate pre-implementation and post-implementation retirement semantics").
Rev-0's error was treating `confirm-upstreamed` as if it were that "separate semantics" — it is not: it
answers a different question ("did upstream already absorb this?") than rejection's question ("should
this ever be implemented?"). Rev-1 corrects this by making **no claim at all** about which mechanism (if
any) eventually serves post-implementation "should never have shipped" retirement — that is deferred, in
full, to future work. A future PRD — potentially `PRD-feature-unapply.md` (currently untracked WIP in
this repository, not yet scoped or accepted) — may address retirement of already-implemented features,
but doing so is explicitly **not** this ADR's decision to make, and Cluster F' must not build toward it
implicitly. See §4 below for the full orthogonality argument this decision rests on, now reinforced by a
concrete defense-in-depth requirement (see the mutating-reconcile guard below).

**Defense-in-depth guard (new, F-INT-2 fix #3; guard placement corrected rev-2)**: independent of
`reject`'s own scope, every *existing* mutating reconcile command that can transition a feature toward
`upstream_merged` — today `tpatch reconcile confirm-upstreamed`, and any future retirement variant —
MUST refuse to operate on a feature whose `status.State == store.StateRejected`, returning a
state-transition error (exit code `3`, D4 addendum — refusing confirm-upstreamed on a rejected feature
is "the current state makes this transition invalid," not a malformed-input case) before **any**
mutation, including audit-log appends.

> **Rev-2 correction on guard placement**: rev-1 placed this guard at `saveConfirmUpstreamedStatus`'s
> call site (`internal/cli/cobra.go:2554-2562`). Internal review traced the call chain one level up and
> found this is too late: `applyConfirmUpstreamedTransition` (`internal/cli/cobra.go:2503-2552`) —
> `saveConfirmUpstreamedStatus`'s only caller — **appends a `ReconcileRevision` via
> `store.AppendReconcileRevision`** (`internal/cli/cobra.go:2535`) *before* it ever calls
> `saveConfirmUpstreamedStatus` (`internal/cli/cobra.go:2554`). A guard placed only inside
> `saveConfirmUpstreamedStatus` would correctly prevent the `status.json` mutation, but a false
> confirm-upstreamed audit-log entry would already have been permanently appended to the append-only
> reconcile-revision log by the time the guard fires — itself a form of silent corruption of the audit
> trail this ADR exists to protect. The corrected guard location is the **first statement inside
> `applyConfirmUpstreamedTransition`** (`internal/cli/cobra.go:2503`), before even the crash-recovery
> idempotency check (`isConfirmedViaReviewTransition`, `internal/cli/cobra.go:2496-2498`, invoked at
> line 2511) — that early-return branch also calls `saveConfirmUpstreamedStatus` directly without going
> through the append path, so the guard must precede it too, not just the append call. Concretely:
> `applyConfirmUpstreamedTransition(s, status, info)` checks `status.State == store.StateRejected` as
> its very first action and returns a state-transition `ExitCodeError` (exit `3`) immediately, before
> either the crash-recovery branch or the `AppendReconcileRevision` call can run.

This is not new PRD-level design scope (it does not add a decision to make); it is a Cluster F'
**implementation task**, mirroring the existing guard style already present in `AuditRetirement`'s own
precondition check (`internal/workflow/retirement_audit.go:38`). Concretely: if
`applyConfirmUpstreamedTransition` is ever invoked against a `rejected` feature — which should never
happen through the normal `reconcile` flow, since a `rejected` feature has no recipe/patch to reconcile —
it must fail loudly, before any append or save, rather than silently overwriting the rejection record or
polluting the audit log, closing the exact hole F-INT-2 found in `saveConfirmUpstreamedStatus`'s
unconditional `status.State` assignment **and** the append-before-guard ordering hole rev-2 found on top
of it.

**Consequences**: Cluster F' implements `reject`'s state check as an unconditional refusal for every
state at or past `implementing` — no flag, no exception — AND adds the defense-in-depth guard above as
the first statement of `applyConfirmUpstreamedTransition`, before its crash-recovery branch and before
its `AppendReconcileRevision` call. PRD §9 test 24 (F-INT-2 defense-in-depth verification) must assert
both `status.json` **and** the reconcile-revision log are byte-identical/unchanged after the refusal. If
a genuine future need for a post-implementation "should never have shipped" command emerges, it is
Alternative 3's separate command, decided by its own future, dedicated PRD/ADR pair — explicitly **not**
this one.

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

### D8: Dependency edge creation guard against rejected parents *(rev-1 fold, F-INT-5 HIGH)*

> Internal review found an asymmetry: rev-0 correctly refused to *reject* a feature that other features
> already depend on (PRD §5, kept), but said nothing about the reverse direction — creating a *new*
> dependency edge whose parent is already `rejected`. Verified empirically:
> `internal/store/validation.go`'s `ValidateDependencies` (lines 113-210) implements exactly 6 rules
> (self-dependency, dangling ref, kind conflict, cycle detection, `satisfied_by`-requires-
> `upstream_merged`, and Rule 6 — `ErrMultipleActiveSuperseders`, lines 169-207, at most one healthy
> superseder per target) and has no rule today that inspects a parent's state for anything other than the
> `satisfied_by` case. A `depends_on` edge can be freely created today pointing at a feature in *any*
> state, rejected or not. Rev-0's PRD also suggested, as remediation, "reject the dependent too" — internal
> review proved this unsound: `dependentEdges` (`internal/cli/feature_deps.go:170-186`) returns dependents
> regardless of the dependent's own current state, so a "dependent" a rejected-parent's rejection would
> supposedly cascade to could itself already be `rejected`, `applied`, or anything else — "reject it too"
> is not a well-formed operation in general.
>
> **Rev-2 CLI-surface correction**: rev-1's text below cited `define --depends-on` as an edge-creation
> surface. That command/flag **does not exist**. The actual edge-creation surfaces, per
> `internal/cli/feature_deps.go:3-18`'s own header comment, are `tpatch feature deps <slug> add
> <parent>[:kind]` and the equivalent `tpatch amend --depends-on <parent>[:kind]` flag (both documented
> as routing through the same `ValidateDependencies` call, so this decision's rule change applies to both
> surfaces identically with no per-command duplication).

**Alternative 1**: Refuse edge creation for all three dependency kinds (`hard`, `soft`, `supersedes`)
whenever the proposed parent's `State == StateRejected`, as a new rule in `ValidateDependencies`.

Pros: fully symmetric with the existing reject-refused-if-dependents-exist rule (PRD §5) — a `rejected`
feature can neither gain new dependents nor be created *as* a dependent's parent after the fact. Single
enforcement point (`ValidateDependencies`), reused by every command that calls it today (`tpatch feature
deps <slug> add`, `tpatch amend --depends-on`, and any future dependency-editing surface).

Cons: an operator wanting to record "this is soft-ordered after that (now rejected) work, for historical
sequencing only, no functional gate" has no way to express that relationship at all.

**Alternative 2**: Refuse only for `hard` and `supersedes` kinds; permit `soft` edges onto a rejected
parent, since `soft` dependencies are documented as ordering-only hints with no apply-time gate
(`docs/dependencies.md`'s distinction between hard "conditions to satisfy" and soft "ordering only, no
gate at apply time").

Pros: preserves a narrow escape valve for the "historical sequencing only" use case Alternative 1's Con
identifies, without weakening the functional gates (`hard`, `supersedes`) that actually block `apply`/
reconcile behavior.

Cons: introduces a state-dependent branch in `ValidateDependencies` keyed on both `Kind` and the parent's
`State` simultaneously — more surface area to test and explain than a flat rule; also, semantically odd:
why would an operator want to *order after* a rejected feature at all, since ordering existed to satisfy
constraints implied by that feature's own progress, which no longer applies once it is rejected?

**Alternative 3**: Warn-only (print a warning, do not refuse) for all three kinds, mirroring a
"soft guardrail" philosophy some CLIs use for likely-but-not-certain mistakes.

Pros: never blocks an operator's workflow, even in edge cases this ADR's authors haven't anticipated.

Cons: contradicts the "fail loudly" default this PRD/ADR pair chooses everywhere else (PRD §5's
dependents-exist-on-reject default, D1's determinism framing) — a warning is trivially missed in scripted/
CI invocations, and GH #6's own §Acceptance-criteria language ("append-only audit history," "explicit and
deliberate") reads as favoring hard gates over advisory-only warnings for state-machine-adjacent
decisions.

**Chosen**: **Alternative 1** (refuse edge creation for all three kinds — `hard`, `soft`, `supersedes` —
whenever the parent is `rejected`).

**Rationale**: Symmetry with the PRD §5 reject-refused-if-dependents-exist rule is the load-bearing
argument: if a `rejected` feature cannot exist *with* existing dependents, it should not be possible to
*create* a new dependent after the fact either — both directions protect the same invariant ("a
`rejected` feature has no live dependency relationships, full stop"), and enforcing only one direction
would leave the other as a silent gap an operator could stumble into by re-ordering their two CLI calls
(reject first vs. add-edge first) and getting a different, order-dependent outcome. This matches this
ADR's own fail-loudly default (D6, D2) rather than introducing Alternative 2's per-kind carve-out, which
this ADR judges unmotivated absent a concrete operator need surfacing post-implementation.

**Consequences**: Cluster F' adds a 7th rule to `ValidateDependencies`
(`internal/store/validation.go:113-210`) — "no dependency edge (`hard`/`soft`/`supersedes`) may be
created pointing at a parent whose `State == StateRejected`" — returning the same wrapped-sentinel-error
convention the existing 6 rules use, mapped to exit code `3` (D4 addendum, state-transition class) at the
CLI layer regardless of which surface (`tpatch feature deps <slug> add`, `tpatch amend --depends-on`, or
any future edge-editing command) triggers it. PRD §9 gains explicit both-order tests
(reject-then-add-edge, add-edge-then-reject-parent) crossed with all three edge kinds.

### D9: Actor provenance mechanism for `rejected_by`/`reopened_by` *(rev-1 fold, F-INT-6 HIGH)*

> Rev-0's PRD cited `internal/store/reconcile_revision.go`'s `ReconcileRevision` struct as an "existing
> actor-attribution precedent" this ADR could follow. Internal review verified this citation is false:
> `ReconcileRevision` (`internal/store/reconcile_revision.go:47-61`) has fields for revision sequencing
> and commit metadata but **no actor field and no timestamp field at all** — there is no existing
> precedent in this codebase for attributing a `FeatureStatus` mutation to a specific human or automation
> identity. D9 makes that decision from scratch.

**Alternative 1**: Precedence chain — `--actor <string>` CLI flag, else `TPATCH_ACTOR` environment
variable, else `git config user.email` (read from the target repo, matching how this codebase already
shells out to `git` for other metadata, e.g. `gitutil.IsAncestor`), else the literal string `"unknown"`
if none of the above resolve.

Pros: works identically in interactive and CI/scripted invocations (the flag/env-var forms need no git
identity configured at all); degrades gracefully to a clearly-labeled `"unknown"` rather than silently
guessing; consistent with this codebase's existing "explicit flag beats implicit environment beats
config-file default" pattern used elsewhere for provider configuration
(`docs/dependencies.md`'s `features_dependencies` config-then-flag-override shape is a loose precedent
for layered configuration resolution, even though it is not itself an actor-attribution case).

Cons: `git config user.email` can be stale, shared (a CI runner's bot email), or simply wrong for the
actual human triggering the action if they never configured it locally.

**Alternative 2**: Derive the actor from the git commit committer identity of the commit that introduces
the `reject`/`reopen` change (e.g. read `git var GIT_COMMITTER_IDENT` at commit time and back-fill it
after the fact).

Pros: ties the actor field to an identity that is cryptographically/structurally tied to an actual commit,
similar in spirit to how Rule 18's trailer convention ties authorship to a commit trailer.

Cons: **conflates two different mechanisms.** Rule 18's `Co-authored-by`/`Copilot-Session` trailers
(`CLAUDE.md`, `AGENTS.md`) attribute a *commit*, not a *feature-state mutation* — `reject`/`reopen` write
`status.json` at invocation time, before any commit necessarily exists (an operator may run `tpatch
reject` and not commit for hours, or never commit directly, letting a separate process do so). There is
no reliable way for the CLI, at the moment it runs, to know what commit (if any) will eventually carry
this change, making this alternative operationally unworkable as the *primary* mechanism, though it may
be a reasonable secondary correlation someone builds later by joining commit trailers to `rejected_at`
timestamps — that joining logic is out of scope here.

**Alternative 3**: Auto-derive from the OS username (`os/user.Current()`) or hostname
(`os.Hostname()`) if no flag/env var is set, skipping the `git config` lookup entirely.

Pros: works even in environments with no git identity configured at all (e.g. a bare checkout with no
`user.email` set).

Cons: privacy/accuracy concerns — an OS username or hostname is frequently an autogenerated CI runner
name (`runner-abc123`) or a shared account name with no relationship to the actual human/agent making the
decision, and leaking local OS usernames into a tracked, committed `status.json` file is a mild but
avoidable information-disclosure smell this ADR should not introduce without a concrete need.

**Chosen**: **Alternative 1** (precedence chain: `--actor` flag > `TPATCH_ACTOR` env var >
`git config user.email` > `"unknown"` literal fallback).

**Rationale**: This is the only alternative that (a) has a well-defined value at the exact moment
`reject`/`reopen` execute (no dependency on a future, not-yet-created commit, unlike Alternative 2), and
(b) never silently fabricates an identity from ambient OS state (unlike Alternative 3) — it either
resolves to something the operator explicitly provided/configured, or honestly reports `"unknown"`.
Explicitly out of scope: deriving from git commit committer identity (Alternative 2, orthogonal to Rule
18's trailer convention, which attributes commits, not status mutations) and auto-derivation from OS
username/hostname (Alternative 3, privacy concern).

**Consequences**: Cluster F' implements a single shared actor-resolution helper used identically by both
`reject` (`rejected_by`) and `reopen` (appended as the `history[]` entry's actor field, D5), with unit
tests for each precedence tier and the final `"unknown"` fallback (PRD §9).

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
      // internal/workflow/reconcile.go:64-65
      // RetirementAudit exposes the cleanup audit triggered after upstreamed
      // confirmation. Runtime/display only; status.json remains lifecycle truth.
      RetirementAudit *RetirementAuditReport `json:"retirement_audit,omitempty"`
  }
  ```

  > **Rev-1 correction (F-EXT-2 LOW)**: rev-0 quoted a fabricated second comment line ("...when the
  > retirement audit ran as part of this call."). The verbatim comment at
  > `internal/workflow/reconcile.go:64-65` is quoted above; its actual second line — "Runtime/display
  > only; status.json remains lifecycle truth." — is itself independent, additional evidence for this
  > section's own orthogonality argument: the Go source comment states in its own words that
  > `RetirementAudit` is not persisted and that `status.json` (i.e. `store.FeatureStatus`) is the
  > lifecycle source of truth, exactly the distinction this ADR draws between the two mechanisms.

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

- **Orthogonality-by-construction needs an explicit guard, not just an absence of shared fields**
  (rev-1 addition, F-INT-2; guard placement corrected rev-2). Internal review found that
  `saveConfirmUpstreamedStatus` (`internal/cli/cobra.go:2554-2562`) sets
  `status.State = store.StateUpstreamMerged` unconditionally, with no check of the feature's prior
  state — and rev-2 further found that its caller, `applyConfirmUpstreamedTransition`
  (`internal/cli/cobra.go:2503-2552`), appends a `ReconcileRevision` audit entry
  (`internal/cli/cobra.go:2535`) even earlier, before `saveConfirmUpstreamedStatus` ever runs — meaning
  that, absent D6's defense-in-depth guard at the correct (earlier) location, a `rejected` feature that
  somehow acquired a confirming revision could have both its audit log polluted and its rejection
  silently overwritten, collapsing the very orthogonality this section claims. The two data shapes not
  sharing fields is necessary but not sufficient; D6's guard at the top of
  `applyConfirmUpstreamedTransition` is what makes the orthogonality enforced rather than merely
  descriptive.

- **If future work wants to unify these two mechanisms** (e.g. a shared "why did this feature not ship"
  query surface spanning both `rejected` features and `upstream_merged`-with-retirement-audit features),
  **that is a separate ADR** — not this one. This ADR's scope is limited to making `rejected` a correct,
  orthogonal addition to the existing state machine; it explicitly does not attempt, and should not be
  read as attempting, any such unification.

## 5. Migration Path

- **Pre-v0.13.0 `status.json` files**: no `rejected`-related field is present. They are treated exactly
  as they are today — `state` is one of the existing 10 values, and `ValidFeatureState` continues to
  accept them unchanged (this ADR only *adds* an 11th valid value; it removes none). No auto-migration
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
  --json`, plus `reject --json`/`reopen --json` shapes) mean a new test matrix (PRD §9, 26 items) and a
  new documentation surface (`SPEC.md` state table, `docs/feature-layout.md` field list,
  `docs/dependencies.md` — if a rejected feature is ever a dependency parent, see open question below).
- `FeatureStatus` grows by 8 fields (7 top-level + `history[]`), all `omitempty`, so the common case
  (never-rejected feature) has effectively zero marginal read/parse cost, but the struct's total field
  count and documentation burden increases.

**Open questions to defer** (to the implementation cluster or a future ADR, not this one):
- Whether a future retirement command for already-implemented features (Alternative 3 of D6, possibly
  `PRD-feature-unapply.md`) should share any data-model fields with `rejected`'s — deferred entirely to
  that future PRD/ADR pair, not decided here.
- Whether the `--force-post-implementation`-style override rejected in D6 Alternative 2 should ever be
  reconsidered once a dedicated post-implementation retirement command exists and its own semantics are
  settled — explicitly not this ADR's call.

(The rev-0 draft of this section left the `rejected_by` actor-derivation mechanism, the exact `reject`/
`reopen` exit-code integers, and the rejected-parent-as-dependency question open; those are now resolved
by D9, the D4 addendum, and D8 respectively, and are no longer open questions.)

## 7. Implementation Notes (for the F' cluster)

**Files to touch (best guess)**:
- `internal/store/types.go` — add `FeatureState` value `StateRejected`; add `RejectionReason` type +
  constants + `ValidRejectionReason`; extend `FeatureStatus` with the 8 new fields (D1); add
  `HistoryEntry` struct (D5).
- `internal/store/status.go` (or wherever `LoadFeatureStatus`/`SaveFeatureStatus` load/save the struct —
  currently `internal/store/store.go`) — no structural change expected beyond what the `FeatureStatus`
  field additions already cover, since (D1) the save path is reused unchanged.
- `internal/store/validation.go` — extend `ValidateDependencies` (`internal/store/validation.go:113-210`,
  which today implements 6 rules including Rule 6 `ErrMultipleActiveSuperseders` at
  `internal/store/validation.go:169-207`) with the new D8 rule refusing edge creation onto a `rejected`
  parent, for all three edge kinds (`hard`, `soft`, `supersedes`).
- `internal/cli/cobra.go` (or a new dedicated `internal/cli/reject.go` file, matching this codebase's
  convention of one file per medium-sized command cluster, e.g. `feature_deps.go`, `land.go`,
  `reconcile_check_applied.go`) — add `rejectCmd()`/`reopenCmd()`, register both on `buildRootCmd`
  (`internal/cli/cobra.go:60-80`); extend `statusCmd()` with `--include-rejected` and the default
  exclusion filter (`internal/cli/cobra.go:226-457`); extend `nextAction` (`internal/cli/phase2.go:400`)
  with a `case store.StateRejected:` arm; add the dependents-fail-loudly check (reusing/adapting
  `checkRemoveDependents`'s pattern, `internal/cli/feature_deps.go:430-447`); add the `rejected`-state
  precondition check to `applyCmd` (`internal/cli/cobra.go:634-693`) and the reconcile entrypoint
  (`internal/cli/cobra.go:1887`, `reconcileCmd`); add the D6 defense-in-depth guard as the first
  statement of `applyConfirmUpstreamedTransition` (`internal/cli/cobra.go:2503`) — **not**
  `saveConfirmUpstreamedStatus`'s call site (`internal/cli/cobra.go:2554-2562`), since
  `applyConfirmUpstreamedTransition` appends a `ReconcileRevision` at line 2535, before
  `saveConfirmUpstreamedStatus` ever runs — refusing to mutate a `rejected` feature before any append.
- `internal/cli/feature_deps.go` and `internal/cli/c1.go` (`amend --depends-on`) — both edge-creation
  surfaces (`tpatch feature deps <slug> add <parent>[:kind]` and `tpatch amend --depends-on
  <parent>[:kind]`, `internal/cli/feature_deps.go:3-18`) route through `ValidateDependencies`, so the D8
  rule change above is automatically enforced at both surfaces with no separate per-command logic.
- `internal/cli/` — new shared helper(s) for evidence content-hashing (D3 addendum: SHA-256 computation
  at `reject`/`reopen` time, path-safety checks, normalization/dedup/sort) and for actor resolution
  precedence (D9: `--actor` flag / `TPATCH_ACTOR` env / `git config user.email` / `"unknown"` fallback
  chain) — both reused identically by `reject` and `reopen`.
- `internal/store/store.go` — extend `RefreshFeaturesIndex` (`internal/store/store.go:677-695`) with the
  distinct "Rejected" trailing section and default-exclusion from the main table.
- `assets/` — no skill-text change is anticipated to be strictly required for the parity guard
  (`assets_test.go` checks that skill docs mention *current CLI commands*; whether `reject`/`reopen`
  need explicit mention is an implementation-time call, but if any skill enumerates the full command
  list by name, it must be updated in lockstep or the parity guard will fail).
- `SPEC.md` — add `rejected` to the Feature States table (§3), add `reject`/`reopen` to the CLI Commands
  table (§4), document the locked exit-code contract (D4 addendum) alongside the existing `verify`
  exit-2 precedent, and note that `reconcile confirm-upstreamed` refuses `rejected` sources (D6 guard).
- `docs/feature-layout.md` — document the 8 new `status.json` fields under "State & debug files".
- `docs/dependencies.md` — cross-reference the "fail loudly on rejected parent" behavior (PRD §5) and
  the new D8 edge-creation guard if it materially changes any documented dependents-gate example.

**Test files** (from PRD §9, now expanded per F-INT-8): new `internal/cli/reject_test.go` (or similarly
named) covering all enumerated scenarios (happy paths, validation errors with exit code 2, state-
transition errors with exit code 3, dependency-order symmetry both directions × 3 edge kinds, evidence
containment/canonicalization, actor precedence, multiple/unbounded reopen cycles, direct
`confirm-upstreamed`-on-`rejected` refusal); a new `internal/store` round-trip fixture test for
pre-v0.13.0 byte-identity (D7); a new `internal/store/validation_test.go` case for the D8 rejected-parent
edge-creation guard; extensions to existing `internal/cli` status/next/FEATURES.md render tests for the
inclusion/exclusion behavior.

**Do NOT touch**: `internal/workflow/reconcile.go`, `internal/workflow/retirement_audit.go` — those
remain confirm-upstreamed's (PRD-#4's) territory, orthogonal per §4 above. `reject`/`reopen` must not
read or write `ReconcileSummary` in any form. The one narrow exception is the D6 defense-in-depth guard
itself, which lives in `internal/cli/cobra.go` at `confirm-upstreamed`'s own call site (a precondition
check on the *existing* command), not inside `reconcile.go`/`retirement_audit.go`'s internals — it does
not change what those files do, only gates one of their callers.
