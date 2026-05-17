# ADR-024 — Patch generation manifest boundary

**Status**: Accepted
**Date**: 2026-05-16

> **Note**: this ADR-024 covers the WP-002 cluster's patch-generation manifest
> boundary (`PRD-feature-patch-identity-metadata`). It is distinct from
> `ADR-025-reconcile-evidence-and-revision-schema`, which covers the WP-003
> reconcile-cluster's evidence/revision schema. The two ADRs are unrelated
> but adjacent in number for historical reasons (see broker's 2026-05-16
> supervisor LOG renumbering decision).

**Source PRD**: [PRD-feature-patch-identity-metadata.md](../prds/PRD-feature-patch-identity-metadata.md) §"Implementation Gate" (lines 41-51), §3, §4, §5, §6, §7, §9
**Related ADRs**: ADR-011 (feature dependency DAG — dependency-snapshot field set inherits from this), ADR-013 (verify freshness overlay — parent-snapshot precedent), ADR-018 (record collision detection — same-bytes-skip semantics), ADR-019 (`tpatch land` trailer schema — patch SHA / recipe SHA relationship to commit trailers)

## Context

`PRD-feature-patch-identity-metadata` (slice 3 of 4 in the
capture-and-metadata foundation cluster) proposes an append-only per-
feature manifest at
`.tpatch/features/<slug>/artifacts/patch-generations.json` so future
identity, dependency, and structural-middle-pass work has a stable
generation-scoped object to reference. PRD §"Implementation Gate"
conditions implementation on a binding ADR that accepts (or revises) six
decisions; §9 leaves two further open questions.

Blast radius is wide. The manifest sits next to `status.json`
(`internal/store/types.go:234` `ApplySummary`) and `claims.json`
(`internal/store/claims.go:290`); its boundary against both must be
explicit. It borrows identity primitives already produced ad-hoc by
`verify` (`internal/store/types.go:191` `VerifyRecord`) and the
the phase-1.5 detector (`PRD-patch-already-upstream-detector` /
`internal/workflow/patch_id_detector.go` + `internal/gitutil/patch_id.go`). The dependency-snapshot field set extends
the ADR-011 `Dependency` struct (`internal/store/types.go:221`) with
parent-version fields that today are not persisted anywhere. And
`tpatch land` trailers (ADR-019) already commit-bind patch/recipe SHAs,
so the manifest must coexist with — not contradict — those trailers.

This ADR is the Wave beta gate. It does not change code, schema, asset
text, or CLI behavior; it locks the shape of the artifact the v0.10.0
Wave beta implementer will produce. All six PRD §"Implementation Gate"
decisions are accepted as written. Both §9 open questions are resolved.
Three additional boundary questions (`kind` enum closure, `refs` block,
schema versioning) are settled to remove ambiguity before code dispatch.

## Decisions

### D1 — Append-only generation manifest as a separate file

Generations live in `artifacts/patch-generations.json`, a per-feature
file outside `status.json`, as a stable JSON object with `version`,
`feature`, `current_generation`, and `generations[]` sorted by
`generation`.

**Decision drivers**.

- **`status.json` is on the hot read path** — `ApplySummary` and
  `Verify *VerifyRecord` (`internal/store/types.go:174,234`) load on
  every command; folding unbounded history into the same document
  inflates every read and forces every existing fixture to be
  regenerated.
- **Separate-file precedent exists** — `claims.json` (alpha-1)
  demonstrated that a co-located feature artifact with its own version
  field, atomic write, and schema validation is a low-cost extension
  of the store (`internal/store/claims.go:290`).

**Alternatives considered**. (a) Embed `generations[]` inside
`status.json` — rejected: bloats every read path and couples lifecycle
truth to audit history. (b) Spread per-generation files under
`artifacts/generations/NNN.json` — rejected: trades one atomic write
for N with no compensating benefit.

**Consequence**. One new path under `.tpatch/features/<slug>/artifacts/`.
`status.json` byte-identity is preserved for repositories that have not
yet recorded a post-PRD generation. Pruning and rotation are deferred.

### D2 — Monotonic `generation` plus content-addressed `generation_id`

Each entry carries both identifiers. `generation` is a 1-based monotonic
integer scoped to the feature (the ordering primitive). `generation_id`
is `"pg_" + first 12 hex chars of SHA-256(canonical JSON of: feature
slug | generation | patch_sha256 | recipe_sha256 | base_commit |
upper.commit | upper.kind | upper.ref | capture.mode |
sorted(capture.pathspecs) | sorted(capture.claim_ids))` — the
equivalence/audit primitive.

**Decision drivers**.

- **Ordering and equivalence are different operations** — equivalence
  must survive prune/rotate; ordering must not collide across
  concurrent records.
- **PRD §4.1 lines 225-229 mandate duality** — same-payload re-capture
  must classify as no-op; same-ID different-payload must refuse rather
  than silently append.

**Alternatives considered**. (a) Monotonic integer only — rejected:
equivalence degrades to field-by-field comparison, the brittle scan
PRD §1 calls out as the current gap. (b) Content-addressed only —
rejected: sorting by hash is meaningless; "previous generation"
becomes O(N).

**Consequence**. Implementers ship a canonical JSON serializer stable
under field-order/platform variation; the same helper backs the
strict-schema reader (D9). Future ADRs that add manifest fields must
state whether each new field enters the `generation_id` hash input.

### D3 — No wall-clock timestamps in the manifest

The manifest contains no `recorded_at`-style fields. Ordering is solely
by `generation`; time-bearing provenance uses Git object identity
(`base_commit`, `upper.commit`).

**Decision drivers**.

- **Determinism is load-bearing** — timestamped artifacts
  (`VerifyRecord.VerifiedAt`, `ApplySummary.*At`) are cordoned from
  byte-identity tests today; generation history is meant to be diffed
  across CI runs.
- **PRD §3 line 204 is explicit** — "no wall-clock timestamps in v1".

**Alternatives considered**. (a) `recorded_at` as `omitempty` for
debugging — rejected: makes the write path non-deterministic. (b)
Logical sequence counter within reconcile session — rejected:
`generation` already serves that role.

**Consequence**. Fixtures use straight byte comparison; `time.Now()`
must not appear in the manifest write path.

### D4 — No historical backfill from `patches/NNN-*.patch` in v1

The first post-PRD `record` creates `generation: 1` from live state.
tpatch does not scan `patches/NNN-*.patch` to reconstruct prior
generations; existing numbered snapshots remain audit-only per
`docs/feature-layout.md`.

**Decision drivers**.

- **Numbered patches lack the metadata for a faithful entry** —
  capture mode, claim IDs, upper-bound `kind`/`ref`/`commit`,
  dependency parent SHAs, and the recipe hash *at that record time*
  are not recoverable from a raw diff. Backfilling with blanks pollutes
  the D2 equivalence primitive.
- **PRD §6 lines 327-330 documents this stance**, conditioning any
  future migration on an audit-guarantee contract that does not yet
  exist.

**Alternatives considered**. (a) Best-effort backfill with empty fields
— rejected: blurs the trustworthy/untrustworthy boundary the manifest
exists to create. (b) Backfill `patch_sha256` and `git_patch_id` only —
rejected: partial schemas force readers to carry per-field "is this
complete?" logic forever.

**Consequence**. Existing repositories see `current_generation: 1`
after their first post-upgrade record. This links to D7's resolution
of PRD §9's first open question.

### D5 — Dependency snapshots pin parent generation and hashes

Each generation's `dependencies[]` entry uses the PRD §4.3 field set:
`slug`, `kind`, `satisfied_by`, optional `satisfied_patch_id`,
`parent_generation`, `parent_patch_sha256`, and `parent_recipe_sha256`
when a parent recipe exists. The live `status.depends_on` (ADR-011 /
`internal/store/types.go:221`) remains the source of truth for the
*current* edge; the snapshot is the *historical* parent version the
child was captured against.

**Decision drivers**.

- **ADR-011 does not version parents** — the `Dependency` struct only
  carries `Slug`, `Kind`, `SatisfiedBy`; a child has no way to express
  "I was captured against parent generation 2, not 3", the gap PRD §1
  lines 105-110 calls out.
- **Snapshot vs. reference is the standard pattern** — `VerifyRecord`
  does the same for the parent's *state literal* (ADR-013); generations
  extend that to parent *identity*.

**Alternatives considered**. (a) Snapshot only `parent_generation` —
rejected: meaningful only relative to a manifest that may be pruned,
malformed, or absent. (b) Extend `Dependency` itself — rejected:
pollutes a live-state struct with historical fields and forces every
existing `status.json` fixture to be regenerated.

**Consequence**. Parent state is frozen at child-record time;
subsequent parent re-records do not retroactively rewrite the
snapshot. Missing parents record empty hashes and zero
`parent_generation`; strict readers must tolerate those documented
zero values.

### D6 — `git patch-id --stable` as the sole persisted patch-id algorithm

The only value ever written to `git_patch_id` in v1 is the output of
`git patch-id --stable`. The marker `git_patch_id_algorithm:
"git-patch-id-stable"` is a required literal; no other value parses
in v1.

**Decision drivers**.

- **The phase-1.5 detector already commits to `--stable`**
  (`PRD-patch-already-upstream-detector`; implementation in
  `internal/workflow/patch_id_detector.go` and
  `internal/gitutil/patch_id.go` invokes `git patch-id --stable`
  directly). A manifest value that could be either mode would silently
  disagree with the detector cache (PRD §5.4).
- **The algorithm marker future-proofs the schema** — a future
  structural-fingerprint ID adds a *new* field; the marker keeps v1
  stable distinguishable from later variants without re-parsing.

**Alternatives considered**. (a) Persist patch-id without the marker —
rejected: the marker is one string per generation. (b) Define a
tpatch-custom patch-id — rejected: duplicates Git tooling and
disagrees with the detector implementation
(`internal/gitutil/patch_id.go`) without compensating gain.

**Consequence**. Implementers reuse the existing `git patch-id --stable`
helper (or centralize one; PRD §7) and reject any manifest whose marker
differs from the v1 literal (see D9).

### D7 — Malformed manifest: `record` refuses, status warns, reconcile distrusts identity fields

Resolves PRD §9's second open question.

- **`record`**: refuses to write a new generation if the existing
  manifest is unparseable, fails schema validation (D9), or has
  internal inconsistencies (`current_generation` not matching the max
  in `generations[]`; duplicate `generation` integers; duplicate
  `generation_id` with differing payload). Non-zero exit with a
  diagnostic pointing at the manifest path. No `--force` repair flag
  ships in v1.
- **Read-only `status`, `list`, `verify`**: warn and continue. The
  manifest is treated as "unknown generation history", not corruption.
- **`reconcile`**: does not trust identity fields from a malformed
  manifest, but reconcile itself proceeds, recomputing from live bytes
  as today.

**Decision drivers**.

- **Asymmetric blast radius** — `record` is the only writer; refusing
  there is the chokepoint that prevents cascading corruption.
  Read-only commands have no write side-effect to protect.
- **PRD §6 lines 333-337 sketches this asymmetry directly**.

**Alternatives considered**. (a) Warn-everywhere — rejected: a `record`
that proceeds against a malformed manifest writes a generation whose
`current_generation` bookkeeping is already broken. (b) Refuse-everywhere
— rejected: blocks `tpatch status` and `tpatch verify` on a manifest
they do not own and cannot repair.

**Consequence**. A single `ValidateManifest` helper returns
`(parsed, nil)` or `(zero, err)`; writers refuse on err, readers warn.
A future ADR may introduce `--force` or a `tpatch patches repair`
surface when operator experience surfaces malformation.

### D8 — `kind` enum closure for v1: only `record` and `reconcile` are writable

The PRD §4.1 enum has six values. v1 ships two writers: `record` (per
PRD §5.1, §5.2) and `reconcile` (per PRD §5.3). The other four
(`amend-refresh`, `amend-fixup`, `import`, `manual-metadata`) are
**reserved-but-unused-in-v1**: readers accept them as valid, no v1
code path emits them.

**Decision drivers**.

- **`amend-*` is gated on `PRD-feature-patch-amend`** (slice 4) and
  its companion ADR; emitting those kinds in v1 would pre-bind that
  future decision.
- **`import` and `manual-metadata` have no producing command today** —
  reserving them is cheap and prevents v2 from breaking v1 readers.

**Alternatives considered**. (a) Ship only `record`; defer `reconcile`
writes — rejected: reconcile is the dominant source of canonical-patch
byte changes on long-lived features (see ADR-017). (b) Ship all six
kinds — rejected: pre-binds `amend-*` and `import` semantics to
whatever Wave beta guesses ahead of their own ADRs.

**Consequence**. Wave beta tests cover `record` and `reconcile`
writers only. Future ADRs add their own writers and supersede this
enum closure by reference.

### D9 — Strict schema validation; `refs` reserved but empty; manifest tracks patch-byte changes only

`version: 1` at the top level. v1 readers validate strictly: unknown
top-level fields, unknown per-generation fields, and unknown values for
closed enums (`kind`, `upper.kind`, `git_patch_id_algorithm`) cause a
parse error and trip D7. The `refs` block exists on every generation
with exactly the four keys `anchors`, `fingerprints`, `relations`,
`vector_manifest`; all four are empty strings in v1 (each future
PRD per PRD §4.4 lands populated values under a `version: 2`
versioned-extension ADR). The manifest tracks **canonical patch byte
changes only** — recipe-only revisions (PRD §5.2) and same-bytes
duplicate records (PRD §5.1, inheriting ADR-018 /
`PRD-record-collision-detection` §3.2) do not append; a separate
metadata-revision history is a future PRD's concern.

**Decision drivers**.

- **Strict reads are the cheap path to forward compatibility** —
  PRD §7 line 345 asks for this explicitly; v1 picks refusal.
- **Reserved-but-empty `refs` slots prevent shape churn** — every
  generation already carries the four ref keys, so middle-pass
  producers fill in values rather than mutating the schema shape (and
  the D2 hash input).
- **`version: 1` is the safety net** — v2 with new fields increments
  this integer; v1 refusal on mismatch catches premature cross-version
  mixing.

**Alternatives considered**. (a) Permissive reads — rejected: ignored
fields become silent drift in the D2 equivalence primitive. (b) Omit
`refs` until populated — rejected: forces every future ref-owning PRD
to mutate the schema shape.

**Consequence**. The manifest is written using the **same atomic
`.tmp` + `rename` + fsync pattern as `claims.json`**
(`internal/store/claims.go:290`); no new I/O abstraction is needed.
Strict reads can use Go's `json.Decoder` with `DisallowUnknownFields`
plus explicit enum checks.

## Consequences

Aggregating D1-D9, the Wave beta implementer can rely on:

- one new path under `.tpatch/features/<slug>/artifacts/` written via
  the `claims.json`-style atomic pattern (D1, D9);
- two identifiers per generation, with `generation_id` derived from a
  fixed field set (D2) and `git_patch_id` always
  `git patch-id --stable` (D6);
- no time-source plumbing (D3) and no migration scanner over
  `patches/` (D4);
- a dependency-snapshot shape (D5) that extends — not replaces — the
  ADR-011 `Dependency` struct;
- two writable `kind` values plus four reserved-but-unused (D8);
- strict-schema reads (D9) with the D7 writer-refuses /
  reader-warns / reconcile-distrusts split.

Permanently ruled out without a follow-up ADR: embedding generation
history inside `status.json`; wall-clock timestamps; historical
backfill from `patches/NNN-*.patch`; persisted non-`--stable` patch-id
values; permissive reads in v1; `--force` overrides on malformed-
manifest refusal; recipe-only or same-bytes generation appends.

Deferred to Wave gamma or beyond: `amend-refresh` / `amend-fixup`
writers (`PRD-feature-patch-amend` and companion ADR); `import` and
`manual-metadata` writers; populated
`refs.{anchors,fingerprints,relations,vector_manifest}` (four future
PRDs per PRD §4.4); prune/rotation; recipe-only metadata-revision
history (PRD §5.2); a repair surface for malformed manifests.

## Open questions deferred

- **Versioned-extension policy for v2**. v1 refuses unknown fields
  (D9); v2 needs an explicit "accept unknown under prefix X" or
  "additive-only" policy. Owned by whichever PRD first needs to
  extend — most likely `PRD-structural-patch-fingerprints` or
  `PRD-structural-anchor-manifest`.
- **Whether a separate metadata-revision history is warranted**. If
  operators turn out to revise recipes frequently without changing
  patch bytes, a future PRD owns that; explicitly out of scope here.
- **Audit guarantees for any future `patches/`-backfill migration**.
  D4 closes v1's door; reopening is owned by a future migration PRD
  that defines best-effort fields and reader guards.

## References

- [PRD-feature-patch-identity-metadata.md](../prds/PRD-feature-patch-identity-metadata.md) §"Implementation Gate", §3, §4, §5, §6, §7, §9
- [PRD-record-collision-detection.md](../prds/PRD-record-collision-detection.md) §3.2 (same-bytes-skip semantics inherited by PRD §5.1)
- [PRD-record-capture-modes.md](../prds/PRD-record-capture-modes.md) (capture-mode field source for the D2 hash input)
- ADR-011 (`Dependency` struct extended by D5)
- ADR-013 (`VerifyRecord` parent-snapshot pattern extended by D5)
- ADR-018 (refuse-with-override pattern referenced by D7)
- ADR-019 (`tpatch land` four-trailer schema coexisting with D6)
- [PRD-patch-already-upstream-detector.md](../prds/PRD-patch-already-upstream-detector.md) (locked `git patch-id --stable` as the project-wide patch-id algorithm; cited by D6)
- `internal/workflow/patch_id_detector.go` (phase-1.5 detector using `git patch-id --stable`; D6 manifest value must match this implementation)
- `internal/gitutil/patch_id.go` (`git patch-id --stable` helper invocation; D6 reuse target)
- `internal/store/types.go:174,191` (`Verify *VerifyRecord` / `VerifyRecord`; hash and parent-snapshot precedents for D2/D5/D6)
- `internal/store/types.go:221` (`Dependency` struct extended by D5)
- `internal/store/types.go:234` (`ApplySummary`; D1 boundary against `status.json`)
- `internal/store/claims.go:290` (`SaveClaims` atomic write pattern reused by D1/D9)
- `docs/feature-layout.md` (`patches/NNN` audit-trail boundary cited by D4)
