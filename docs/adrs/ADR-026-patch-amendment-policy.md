# ADR-026 — Patch amendment policy

**Status**: Accepted
**Date**: 2026-05-19
**Supersedes**: —
**Related**: [PRD-feature-patch-amend](../prds/PRD-feature-patch-amend.md), [ADR-024-patch-generation-manifest-boundary](./ADR-024-patch-generation-manifest-boundary.md), [ADR-013-verify-freshness-overlay](./ADR-013-verify-freshness-overlay.md), [ADR-011-feature-dependencies](./ADR-011-feature-dependencies.md), [WP-002-capture-and-metadata-foundation](../whitepapers/WP-002-capture-and-metadata-foundation.md) §4

> Note: ADR-025 (`ADR-025-reconcile-evidence-and-revision-schema`) is the WP-003 reconcile-cluster's reserved slot (still unwritten as of this ADR's acceptance). ADR-026 is the WP-002 capture-and-metadata-foundation cluster's Wave γ gate. The two are unrelated but adjacent in number.

## Context

`PRD-feature-patch-amend` is slice 4 of 4 in the WP-002 capture-and-
metadata foundation cluster (`docs/prds/PRD-feature-patch-amend.md:26-33`).
It introduces first-class patch amendment vocabulary for an already-recorded
feature: refresh the same logical patch, append an explicit fixup, or amend
metadata without touching canonical patch bytes (`docs/prds/PRD-feature-patch-amend.md:54-73`).

Wave β shipped the generation manifest boundary under ADR-024. That boundary
made `.tpatch/features/<slug>/artifacts/patch-generations.json` the append-only
patch-byte audit trail, wrote only `kind: record|reconcile`, and reserved
`amend-refresh|amend-fixup|import|manual-metadata` as read-compatible future
values (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:253-277`).
It also locked strict v1 reads: unknown fields and unknown enum values are
malformed-manifest errors (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:279-315`).

Wave γ needs the reserved amendment values to become writable without reopening
Wave β. The PRD deliberately left policy gaps around plain `record` defaults,
no-byte-change refresh, fixup targeting, dependent staleness, verify freshness,
metadata-only audit, and command namespace (`docs/prds/PRD-feature-patch-amend.md:124-137`,
`docs/prds/PRD-feature-patch-amend.md:248-273`, `docs/prds/PRD-feature-patch-amend.md:330-333`).
WP-002 row 3 requires this ADR before Wave γ implementation (`docs/whitepapers/WP-002-capture-and-metadata-foundation.md:217-226`).

This ADR is docs-only. It changes no code, test, asset, or PRD body. It locks
implementation policy so the Wave γ code path extends the existing manifest,
status, dependency, and verify contracts instead of inventing parallel state.

## Decisions

### D1 — Plain `record <slug>` byte change classification is hybrid

The first generation for a feature is `kind: record` when no prior `record` or
`reconcile` generation exists. Every later bytes-changing plain
`tpatch record <slug>` invocation is `kind: amend-refresh`.

This reconciles PRD §3.2's compatibility claim that plain changed `record`
defaults to refresh (`docs/prds/PRD-feature-patch-amend.md:124-136`) with
ADR-024 D8's Wave β write boundary (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:253-277`).
The first write is creation, not amendment. Subsequent writes change the
canonical patch of an existing feature and therefore are amendments.

The user-facing command remains compatible: users do not need the new namespace
to keep using `record`. The audit story becomes honest: the manifest records
that a later plain `record` refreshed an existing canonical patch.

### D2 — `--reason` persists on the generation entry

Amendment reason text lives in a per-generation `reason` field in
`patch-generations.json`. The field is a string. It is optional at the schema
level, mandatory when `kind: amend-fixup`, and present for `kind: amend-refresh`
only when the user passed `--reason`.

`record.md` can render or summarize the reason for human reading, but the
machine-owned source for amendment reason is the generation entry. This matches
the PRD's explicit fixup reason surface (`docs/prds/PRD-feature-patch-amend.md:171-190`)
and its optional refresh reason surface (`docs/prds/PRD-feature-patch-amend.md:152-169`).

Privacy gating is not part of v1. `--reason` is advisory tracked metadata in
this ADR. Rich context retention remains deferred to the future
`ADR-capture-context-privacy-boundary`, as WP-002 says for free-text/richer
context (`docs/whitepapers/WP-002-capture-and-metadata-foundation.md:82-86`,
`docs/whitepapers/WP-002-capture-and-metadata-foundation.md:217-224`).

### D3 — No-byte-change refresh exits 0 and appends nothing

`tpatch feature patch refresh <slug>` with no canonical patch byte change exits
0, prints a stderr note equivalent to `no patch byte change; refresh skipped`,
and does not append a generation.

This mirrors the PRD refresh behavior (`docs/prds/PRD-feature-patch-amend.md:158-169`)
and Wave β's same-bytes append-skip boundary for patch-byte history
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:279-292`).
A no-byte refresh is a successful no-op, not a failure and not a metadata-only
manifest revision.

If metadata changed through a separate metadata command, D9 owns the audit
boundary. Patch-byte generation history remains unchanged.

### D4 — Fixups target a generation by `generation_id`

`patch-generations.json` gains `generations[].fixup_of_generation`. The field
value is a `generation_id` string such as `pg_abc123def456`.

The field is optional at the schema level and present only for
`kind: amend-fixup`; when `kind: amend-fixup`, it is mandatory. The value is the
previously current generation at fixup capture time, matching the PRD behavior
(`docs/prds/PRD-feature-patch-amend.md:171-190`).

The top-level schema version stays `version: 1`. ADR-024 D9 remains strict on
unknown fields (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:279-315`);
this ADR registers `fixup_of_generation` as a known v1 per-generation field for
Wave γ readers and writers. The content-addressed identifier is robust under
any future generation renumbering; the monotonic integer is not the fixup
anchor.

### D5 — Dependent staleness is a status overlay label

The visible dependent-staleness surface is the overlay label
`parent-generation-stale` in existing `status` and `status --json` output. It
is not a lifecycle state, not a new `status --stale` subcommand, and not a new
persisted `status.json` field.

Detection compares each child generation snapshot
`dependencies[].parent_generation` and `dependencies[].parent_patch_sha256`
against the parent's current `current_generation` and latest `patch_sha256` in
`patch-generations.json`. ADR-024 D5 defines the dependency snapshot fields
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:157-187`); the current
schema exposes the same names (`internal/store/patch_generations.go:30-75`).

Soft-dependency dependents warn. Hard-dependency dependents block on apply and
reconcile according to ADR-011's hard/soft policy
(`docs/adrs/ADR-011-feature-dependencies.md:47-56`). This follows PRD §5.1's
label/overlay requirement and hard/soft split (`docs/prds/PRD-feature-patch-amend.md:250-264`).
Legacy parents without `patch-generations.json` produce no staleness signal;
readers treat the parent generation as unknown rather than broken.

### D6 — Patch-content amendments invalidate verify freshness by hash inputs

Patch-content amendments invalidate verify freshness when they change an
ADR-013 freshness input. The exact inputs are:

1. `artifacts/post-apply.patch` content SHA-256, represented by
   `patch_hash_at_verify` in ADR-013's derivation (`docs/adrs/ADR-013-verify-freshness-overlay.md:147-164`).
2. `apply-recipe.json` content SHA-256, represented by `recipe_hash_at_verify`
   in the same derivation (`docs/adrs/ADR-013-verify-freshness-overlay.md:147-164`).
3. Base or parent drift already handled by ADR-013's parent-snapshot and closure
   replay rules (`docs/adrs/ADR-013-verify-freshness-overlay.md:166-189`,
   `docs/adrs/ADR-013-verify-freshness-overlay.md:210-230`).

`amend-refresh` and bytes-changing `amend-fixup` trigger invalidation because
the canonical patch SHA-256 changes and the recipe SHA-256 changes if the
recipe is regenerated. Metadata-only amend does not invalidate verify freshness
unless it touches one of those ADR-013 inputs.

Not freshness inputs: `record.md`, `claims.json`, request text, dependency
request fields, status labels, and status request metadata. PRD §5.2 says
metadata-only amendments preserve freshness unless they touch `apply-recipe.json`,
`artifacts/post-apply.patch`, or ADR-013-named inputs (`docs/prds/PRD-feature-patch-amend.md:266-273`).

### D7 — Command namespace is final for v1

The v1 patch-content amendment namespace is exactly:

```bash
tpatch feature patch refresh <slug> [--reason "..."]
tpatch feature patch fixup <slug> --reason "..."
```

No aliases ship. There is no `tpatch refresh`, no `tpatch amend --patch`, and
no `tpatch record --amend-kind`. The existing `tpatch amend` command keeps its
metadata-only meaning (`docs/prds/PRD-feature-patch-amend.md:192-212`,
`docs/prds/PRD-feature-patch-amend.md:281-292`).

The locked v1 subverb set under `tpatch feature patch` is `{refresh, fixup}`.
Any other subverb refuses with a diagnostic equivalent to
`unknown subverb; v1 supports refresh|fixup`. `fork` and `fold` remain v2-
deferred per PRD §4.4-4.5 (`docs/prds/PRD-feature-patch-amend.md:213-246`).
Plain `record <slug>` remains a compatible implicit refresh path per D1.

### D8 — `record --force-amend` stays Git-rewrite orphan-only

`record --force-amend` is not a refresh shortcut, not a fixup shortcut, and not
a bypass for `parent-generation-stale` policy. It remains only the existing
escape hatch for Git-rewrite orphan detection.

The shipped implementation detects the classic `git commit --amend` shape and
refuses when downstream features reference the rewritten SHA; `--force-amend`
bypasses that gate with a warning (`internal/cli/cobra.go:889-907`). The flag
help text names the same boundary: bypass the dependent-orphan gate when
recording on top of an amended commit (`internal/cli/cobra.go:1415`).

This matches the v0.7.0 roadmap entry for `feat-amend-dependent-warning`
(`docs/ROADMAP.md:338-350`), the capture-modes BC promise
(`docs/prds/PRD-record-capture-modes.md:294-305`), and PRD §6's compatibility
line (`docs/prds/PRD-feature-patch-amend.md:281-292`). Amendment commands do
not consume `--force-amend`.

### D9 — Metadata-only amend does not append patch generations in v1

Metadata-only amend writes no `patch-generations.json` generation in v1. It can
update `status.updated_at`, request fields, dependency declarations, claim
metadata, and other existing metadata-owned artifacts, but it does not append
`kind: manual-metadata`.

The reason is identity boundary. ADR-024 D9 defines the manifest as patch-byte
history: recipe-only and same-bytes revisions do not append, and a separate
metadata-revision history is a future PRD concern
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:279-292`).
PRD §4.3 says metadata-only amend leaves `artifacts/post-apply.patch` unchanged
and defers generation behavior to the identity decision (`docs/prds/PRD-feature-patch-amend.md:192-212`).
This ADR's identity decision is no metadata generation.

Metadata audit for v1 lives in `claims.json`, `status.json`, request files, and
normal repository diffs. A richer metadata revision log requires a separate v2
ADR. `manual-metadata` remains reserved-but-unused.

### D10 — Wave β D8 transitions `amend-refresh` and `amend-fixup` to writable

ADR-024 D8's reserved amendment values are now writable for Wave γ.
`amend-refresh` is written by `tpatch feature patch refresh <slug>` and by the
D1 subsequent bytes-changing plain `record <slug>` path. `amend-fixup` is
written by `tpatch feature patch fixup <slug> --reason "..."`.

`import` and `manual-metadata` remain reserved-but-unused. Readers keep
accepting all six v1 `kind` values (`record`, `reconcile`, `amend-refresh`,
`amend-fixup`, `import`, `manual-metadata`), as ADR-024 D8 and the current
reader already enumerate (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:253-277`,
`internal/store/patch_generations.go:153-161`).

This is the controlled extension Wave β anticipated. No Wave β schema change
required. ADR-024 D9's strict-on-unknown contract still holds because the
values are known and this ADR registers the new Wave γ fields before writers
emit them.

## Alternatives Considered

### D1 — Plain `record` classification

- **Always keep `kind: record` for plain `record`** — rejected. It preserves the
  cleanest Wave β boundary but hides the fact that a later invocation changed
  an existing canonical patch. It contradicts PRD §3.2's BC-compatible refresh
  default (`docs/prds/PRD-feature-patch-amend.md:124-136`).
- **Always switch changed plain `record` to `kind: amend-refresh`** — rejected.
  It misclassifies the first patch generation as an amendment when no canonical
  patch existed yet.
- **Hybrid first-record then amend-refresh** — accepted. It preserves creation
  semantics and makes later patch-byte changes honest amendments.

### D2 — Reason storage

- **Generation field only** — accepted. It gives readers one schema-owned place
  to find amendment intent.
- **`record.md` body only** — rejected. Human prose is not a stable machine
  contract and cannot drive future fold/fixup UX.
- **Both as independent sources** — rejected. Dual source creates drift. Human
  rendering can mirror the generation source.

### D3 — No-byte refresh

- **Silent skip** — rejected. Operators need a visible signal that the explicit
  refresh command found no patch-byte change.
- **Non-zero no-op exit** — rejected. The requested end state already holds;
  scripts should not treat an unchanged refresh as failure.
- **Exit 0 with stderr note and no append** — accepted. It mirrors Wave β skip
  semantics and keeps patch-byte history clean.

### D4 — Fixup target field

- **Integer `generation` number** — rejected. It is compact but brittle under
  future pruning, import, or renumbering policy.
- **String `generation_id`** — accepted. It is content-addressed and already the
  manifest's equivalence/audit primitive from ADR-024 D2
  (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:77-105`).
- **Both string and integer** — rejected. Redundant anchors can disagree and
  create repair policy before any value exists.

### D5 — Dependent-staleness surface

- **Overlay label in `status` / `status --json`** — accepted. It matches
  ADR-011's derived-label model (`docs/adrs/ADR-011-feature-dependencies.md:39-45`)
  and PRD §5.1's label/overlay instruction (`docs/prds/PRD-feature-patch-amend.md:250-264`).
- **Separate `status --stale` subcommand** — rejected. It splits status truth
  and makes stale dependents invisible to the primary operator view.
- **Persisted `status.json` field** — rejected. Parent staleness is derived from
  current parent generation and child snapshot, so persistence would go stale.

### D6 — Verify invalidation

- **Invalidate on every amendment artifact change** — rejected. It treats
  prose/status metadata as if it changed replay semantics.
- **Invalidate only on patch SHA** — rejected. Recipe regeneration changes verify
  replay semantics even when patch bytes happen to stay stable.
- **Use ADR-013 freshness inputs exactly** — accepted. Patch SHA, recipe SHA,
  and parent/base drift are the authoritative verify inputs.

### D7 — Namespace

- **Top-level amendment aliases** — rejected. They compete with existing `record`
  and `amend` meanings and expand the public surface before implementation.
- **`tpatch amend --patch`** — rejected. It overloads the metadata-only command
  PRD §6 explicitly preserves (`docs/prds/PRD-feature-patch-amend.md:281-292`).
- **`tpatch feature patch refresh|fixup` only** — accepted. The broker-locked
  namespace is explicit and bounded (`docs/prds/PRD-feature-patch-amend.md:138-150`).

### D8 — `--force-amend` reuse

- **Reuse `--force-amend` for refresh/fixup** — rejected. The shipped flag has a
  Git-rewrite orphan meaning anchored in code (`internal/cli/cobra.go:889-907`).
- **Keep it separate** — accepted. Amendment policy gets explicit commands;
  Git rewrite orphan bypass remains unchanged.

### D9 — Metadata-only generation

- **Append `kind: manual-metadata`** — rejected. It converts a patch-byte
  identity manifest into a general metadata log and reopens ADR-024 D9.
- **No manifest append in v1** — accepted. Metadata-only changes have existing
  metadata-owned artifacts and repo diffs; richer metadata revisions require a
  separate ADR.

### D10 — Enum transition

- **Leave amendment kinds read-only** — rejected. Wave γ cannot ship refresh or
  fixup without writing the reserved values.
- **Bump manifest to v2** — rejected. The enum values are already known to v1
  readers; only writer policy changes.
- **Transition `amend-refresh` and `amend-fixup` to writable** — accepted. It is
  the planned Wave β forward-compat path.

## Consequences

- Wave γ implementers extend the existing `patch-generations.json` writer with
  `kind: amend-refresh`, `kind: amend-fixup`, optional `reason`, and fixup-only
  `fixup_of_generation`.
- Plain `record <slug>` remains a supported BC path. First generation writes
  `record`; subsequent bytes-changing generations write `amend-refresh`.
- `tpatch feature patch refresh` and `fixup` are the only new v1 amendment
  subverbs. `fork` and `fold` remain follow-up PRDs.
- Wave β readers remain strict. They must know the new fields before Wave γ
  writers emit them; no permissive unknown-field behavior is introduced.
- Verify, status, and reconcile consume amendment outcomes through existing
  inputs: patch/recipe hashes for freshness, generation snapshots for parent
  staleness, and current canonical `post-apply.patch` for replay.
- Metadata-only amendment stays outside patch-generation history in v1. If
  operators need metadata revisions, that is a new ADR and not a Wave γ side
  effect.
- `record --force-amend` remains separate from patch amendment and continues to
  mean only Git-rewrite dependent-orphan bypass.

## Wave γ Implementation Contract

This appendix narrows implementation freedom to minimize reader/writer drift
when Wave γ lands. It is binding on the Wave γ implementer; it does not
re-open any D1–D10 decision.

### IC1 — Landing order (reader before writer)

Wave γ must land in this commit order, and each step must include its own
tests before the next step starts:

1. **Schema-extension commit**: Extend the `PatchGeneration` struct with the
   `reason` string field (D2) and `fixup_of_generation` string field (D4).
   Register both as known fields in the strict v1 reader. Land the
   "Wave γ contract test" (see IC2) at the same commit. **No writer changes
   in this commit.** Existing manifests must still load byte-identically.
2. **Kind-enum commit**: Make `amend-refresh` and `amend-fixup` writable in
   the kind enum (D10). Add the `classifyRecordKind` helper (see IC3). **No
   CLI surface in this commit.**
3. **CLI surface commit(s)**: Add `tpatch feature patch refresh` and
   `tpatch feature patch fixup` subverbs (D7). Wire dependent-staleness
   overlay surface (D5) and verify-freshness invalidation inputs (D6).

Any out-of-order landing — for example, a CLI surface commit before the
schema extension — is grounds for review rejection on its own. The
reviewer checklist must verify the commit sequence matches IC1.

### IC2 — Wave γ contract test (golden fixture tripwire)

The schema-extension commit MUST add `internal/store/patch_generations_wavegamma_test.go`
covering, at minimum:

1. A golden v1 manifest fixture containing one `record` generation, one
   `amend-refresh` generation with a populated `reason`, and one
   `amend-fixup` generation with both `reason` and `fixup_of_generation`
   populated, pointing at a real prior `generation_id`.
2. Round-trip assertion: load → re-serialize → byte-identical to fixture.
3. Strict-on-unknown assertion: a copy of the fixture with one unknown
   field added must fail to load under ADR-024 D9.
4. Reason-mandatory assertion: an `amend-fixup` generation with missing or
   empty `reason` must fail to load (D2 makes `reason` mandatory for
   fixup).
5. Fixup-target assertion: an `amend-fixup` generation whose
   `fixup_of_generation` does not resolve to a prior `generation_id` in
   the same manifest must fail validation.

This test is the canonical tripwire for reader/writer skew. Any future
schema change that would invalidate it must update the fixture and the
strict-field list in the same commit.

### IC3 — Single-source classifier for D1

D1's hybrid rule (first generation = `record`; subsequent bytes-changing
plain `record <slug>` = `amend-refresh`) MUST live in exactly one function
in `internal/store` (suggested name: `ClassifyPlainRecordKind`). All
writers — plain `record`, `feature patch refresh`, `feature patch fixup` —
that emit `record`/`amend-refresh`/`amend-fixup` MUST route through that
helper or its sibling for explicit refresh/fixup paths. The helper MUST
have a table-driven test enumerating every transition:

- No prior generations → `record`
- Prior generations, identical patch bytes → no append (D3 no-op)
- Prior generations, changed patch bytes, plain `record` → `amend-refresh`
- Prior generations, explicit refresh path → `amend-refresh`
- Prior generations, explicit fixup path → `amend-fixup`

The classifier is the audit-trail's single point of truth. Drift here
silently corrupts replay.

### IC4 — Frozen regions

The following Wave α and Wave β surfaces are frozen for Wave γ. The
implementer brief must cite them by file:line and the reviewer must
confirm none were edited outside the explicit extensions listed above:

- `internal/store/patch_generations.go` — manifest v1 schema. Wave γ
  extends `PatchGeneration` fields per IC1 step 1 but MUST NOT bump the
  `version` constant, MUST NOT relax `DisallowUnknownFields`, and MUST
  NOT alter the `ErrMalformedManifest` classification path (Wave β rev-2
  contract).
- `internal/store/claims.json` writer — file-claims (Wave α) is not on
  the amendment path. No edits.
- `internal/cli/cobra.go:889-907` and `:1415` — `record --force-amend`
  Git-rewrite orphan-detection branch. D8 binds: no behavior change.
- `internal/gitutil/` capture modes (Wave α) — no edits.

### IC5 — Skill-asset parity

The shipped skill prompts must reference `tpatch feature patch refresh`
and `tpatch feature patch fixup` at the same commit those subverbs ship
(IC1 step 3). The existing `assets/assets_test.go` parity guard catches
omissions if the skill prompts and the CLI register surface drift. The
Wave γ implementer brief must require running `go test ./assets/...`
before claiming completion.

### IC6 — Reviewer checklist additions

In addition to the standard checklist, the Wave γ reviewer must verify:

- [ ] Commit sequence matches IC1 (reader → enum → CLI).
- [ ] `patch_generations_wavegamma_test.go` exists and covers all five
  IC2 assertions.
- [ ] `ClassifyPlainRecordKind` (or equivalent) is the only call site
  classifying `record` vs `amend-refresh` for plain `record <slug>`.
- [ ] IC4 frozen regions are unedited outside the IC1-listed extension
  points.
- [ ] `go test ./assets/...` passes (IC5 parity guard).
- [ ] Existing manifests on disk (Wave β fixtures) still load
  byte-identically — no migration required.

## References

- `docs/prds/PRD-feature-patch-amend.md` — Wave γ product contract.
- `docs/adrs/ADR-024-patch-generation-manifest-boundary.md` — Wave β manifest contract this ADR transitions D8 for.
- `docs/adrs/ADR-013-verify-freshness-overlay.md` — verify-cache invalidation inputs cited by D6.
- `docs/adrs/ADR-011-feature-dependencies.md` — hard/soft dependency policy cited by D5.
- `docs/whitepapers/WP-002-capture-and-metadata-foundation.md` §4 — cluster ADR plan row 3.
- `internal/cli/cobra.go` — `record --force-amend` shipped behavior cited by D8.
- `internal/store/patch_generations.go` — current v1 manifest field names and accepted kind enum cited by D5/D10.
- `docs/prds/PRD-record-capture-modes.md` — BC promise preserving `--force-amend` meaning.
- `docs/ROADMAP.md` — v0.7.0 `feat-amend-dependent-warning` entry anchoring `--force-amend` origin.
