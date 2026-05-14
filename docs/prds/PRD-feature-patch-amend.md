# PRD - Feature Patch Amend - `feat-feature-patch-amend`

**Status**: Draft
**Date**: 2026-05-13
**Owner**: Core
**Byline**: T55
**Milestone**: Capture and metadata foundation. Not yet roadmap-committed.
**Depends on**: [PRD-feature-patch-identity-metadata](./PRD-feature-patch-identity-metadata.md). Implementation requires a future `ADR-patch-amendment-policy`.

## Related

- [Patch capture prior art](../state-of-the-art/patch-capture-prior-art-and-hooks.md)
- [Patch identity metadata research](../state-of-the-art/tpatch-metadata-for-patch-identity.md)
- [Research roadmap](../state-of-the-art/research-roadmap.md)
- [WP-001 Feature Slice Gap](../whitepapers/WP-001-feature-slice-gap.md)
- [Feature Layout](../feature-layout.md)
- [Recording Patches](../record.md)
- [ADR-011 feature dependencies](../adrs/ADR-011-feature-dependencies.md)
- [ADR-013 verify freshness overlay](../adrs/ADR-013-verify-freshness-overlay.md)
- [PRD-record-auto-base](./PRD-record-auto-base.md)
- [PRD-record-collision-detection](./PRD-record-collision-detection.md)
- [PRD-reconcile-lock-guard](./PRD-reconcile-lock-guard.md)
- [PRD-tpatch-land](./PRD-tpatch-land.md)
- [PRD-feature-patch-identity-metadata](./PRD-feature-patch-identity-metadata.md)

## Cluster Position

This is PRD 4 of 4 in the capture-and-metadata foundation cluster:

1. `PRD-feature-file-claims`
2. `PRD-record-capture-modes`
3. `PRD-feature-patch-identity-metadata`
4. `PRD-feature-patch-amend`

See [research-roadmap.md](../state-of-the-art/research-roadmap.md) for the
cluster tracker. This PRD builds on the WP-001 / M17 boundary-capture outcomes:
auto-base, collision detection, lock guard, and `land` trailers. Supervisor
acceptance is still required before implementation.

## 0. Claims Audit

This PRD is a proposal. It changes nothing.

| Claim | Evidence |
|---|---|
| `artifacts/post-apply.patch` is the current canonical replay patch and numbered `patches/` files are audit snapshots. | `docs/feature-layout.md`. |
| `tpatch amend` currently edits feature request/dependency metadata; it is not a first-class patch-content amendment command. | `internal/cli/c1.go` amend command. |
| `record` already has a `--force-amend` guard for Git commit rewrites that would orphan downstream references. | `internal/cli/cobra.go` record command; `docs/ROADMAP.md` v0.7.0 row. |
| Verify freshness is invalidated by recipe-touching amend behavior. | `ADR-013-verify-freshness-overlay.md`; `PRD-verify-freshness.md`. |
| Quilt and StGit default to refreshing the managed patch, while Git also supports fixup/squash-forward flows. | `patch-capture-prior-art-and-hooks.md` sections 3.1-3.3 and amendment notes. |

No code, schema, command behavior, or asset text is changed by this PRD.

## Summary

Define first-class patch amendment semantics for "the patch was wrong or
incomplete." tpatch should support both major models seen in prior art:

1. **Refresh/rewrite current patch** - the canonical feature patch is updated to
   the corrected full diff, preserving old generations in metadata.
2. **Append fixup generation** - the correction is recorded as an explicit fixup
   generation that can later be folded into the canonical logical patch.

The recommended v1 is conservative:

- keep `artifacts/post-apply.patch` as the current full diff;
- append patch generation metadata for every patch-content amendment;
- classify amendment kind explicitly;
- make dependency and verify invalidation visible;
- reserve fork/fold names for follow-up PRDs, but keep v1 limited to refresh,
  fixup, and metadata-only amendment.

This PRD depends on a binding amendment ADR before implementation.

## 1. Problem Statement

tpatch can already re-record a feature, and old full-diff snapshots remain under
`patches/`. But the product vocabulary is still ambiguous:

- Did the user refresh the same logical patch?
- Did they add a follow-up fixup that should remain visible?
- Did they only amend intent/metadata?
- Which dependents were recorded against the old patch?
- Which verify result is now stale?

Today, a changed `post-apply.patch` is visible only as "latest bytes changed".
Future middle-pass reconciliation needs stronger provenance: when the canonical
patch moves, downstream dependency pins, patch identity, and structural anchors
must know which generation changed and why.

## 2. Goals / Non-goals

### Goals

1. Define amendment kinds for patch-content and metadata-only changes.
2. Preserve `artifacts/post-apply.patch` as the current full patch.
3. Append patch generation metadata for patch-content amendments.
4. Make dependent invalidation explicit when a parent generation changes.
5. Keep existing `tpatch amend` request/dependency behavior compatible.
6. Avoid confusing `record --force-amend` with patch amendment policy.
7. Provide a CLI path for refresh and fixup classification.

### Non-goals

1. No provider-assisted patch repair in this PRD.
2. No automatic semantic split of one patch into multiple features.
3. No replacement for Git history rewriting tools.
4. No change to `upstream_merged` semantics.
5. No change to land trailer schema.
6. No raw transcript or context storage.
7. No structural fingerprint or vector generation.
8. No fork or fold/squash workflow in v1; those are deferred follow-up PRDs.

## 3. Amendment Model

### 3.1 V1 amendment kinds

| Kind | Meaning | Canonical patch effect | Generation effect |
|---|---|---|---|
| `refresh` | Same logical patch, corrected or completed. | Rewrite `artifacts/post-apply.patch` to the new full diff. | Append generation `kind: amend-refresh`. |
| `fixup` | Follow-up correction that should remain visible until folded. | Rewrite canonical full diff to include the fix. | Append generation `kind: amend-fixup` with `fixup_of_generation`. |
| `metadata-only` | Intent, claims, dependencies, notes, or context changed; patch bytes unchanged. | No patch byte change. | Optional generation or metadata revision depending on identity PRD decision. |

### 3.2 Recommended v1 defaults

- Existing `record <slug>` that changes patch bytes remains allowed and is
  classified as `refresh` for backwards compatibility. Users who want explicit
  amendment intent should use `tpatch feature patch refresh <slug>`.
- Plain changed `record <slug>` defaults to `refresh` and does not require an
  amendment reason. Requiring a reason would break existing record workflows.
- `fixup` requires the explicit `tpatch feature patch fixup <slug> --reason
  "..."` surface because it changes audit interpretation.
- `metadata-only` remains owned by existing `tpatch amend` and future claim
  commands unless patch bytes change.
- Dependents recorded against an older parent generation become stale when the
  parent refreshes or records a fixup.

## 4. User-facing Contract

The broker locked patch-content amendment under the `tpatch feature patch`
namespace:

```bash
tpatch feature patch refresh <slug> [--reason "..."]
tpatch feature patch fixup <slug> --reason "..."
```

These commands are amendment-first user surfaces. They still compose the patch
capture/write path under the hood, but the user's intent is visible in the
command vocabulary instead of being hidden behind another `record` flag.

### 4.1 Refresh current patch

```bash
tpatch feature patch refresh <slug> [--reason "handle nil config"]
```

Behavior:

1. Enter patch-amendment mode with `kind: amend-refresh`.
2. Capture the corrected full feature diff using the normal record capture path.
3. If patch bytes are unchanged, print a no-byte-change note and do not append a
   patch-content generation unless metadata changed.
4. If patch bytes changed, write `artifacts/post-apply.patch`, append the normal
   numbered audit patch, and append a patch generation with
   `kind: amend-refresh`.
5. Invalidate verify freshness as existing patch/recipe drift rules require.
6. Mark dependents pinned to the old generation as stale or broken according to
   dependency policy.

### 4.2 Append fixup generation

```bash
tpatch feature patch fixup <slug> --reason "cover empty response"
```

Behavior:

1. Enter patch-amendment mode with `kind: amend-fixup`.
2. Require `--reason`; refusal happens before patch capture when it is missing.
3. Capture the corrected full diff, not a partial delta.
4. Update `artifacts/post-apply.patch` so reconcile always sees the full current
   feature.
5. Append generation `kind: amend-fixup`.
6. Store `fixup_of_generation` as the previously current generation.
7. Surface the feature as having an unfolded fixup in `status` or `status --json`
   if the implementation adds that read path.

Rationale: tpatch's canonical replay model stays simple, while the generation
manifest preserves the fact that this change was a fixup.

### 4.3 Metadata-only amend

Existing examples:

```bash
tpatch amend <slug> "updated request text"
tpatch amend <slug> --append "additional requirement"
tpatch amend <slug> --depends-on parent:hard
tpatch feature claim add <slug> src/new-scope/
```

Behavior:

- patch bytes unchanged;
- `artifacts/post-apply.patch` unchanged;
- verify freshness unchanged unless recipe/patch bytes drift under existing
  ADR-013 rules;
- patch generation unchanged unless the identity PRD chooses to track metadata
  revisions;
- `status.updated_at` and existing metadata behavior continue as implemented.

### 4.4 Deferred v2 candidate: fork feature from a generation

Possible command:

```bash
tpatch feature patch fork <slug> <new-slug> --from-generation <generation-id>
```

Behavior:

- create a new feature using the selected generation's patch bytes and intent
  summary;
- do not mutate the original feature by default;
- preserve provenance from original slug/generation;
- require explicit dependencies for the new feature.

This can be a follow-up slice if refresh/fixup must ship first.

### 4.5 Deferred v2 candidate: fold fixup

Possible command:

```bash
tpatch feature patch fold <slug> --fixup-generation <generation-id>
```

Because canonical patch bytes already include the fixup after
`tpatch feature patch fixup`, fold is mostly a metadata operation:

- mark the fixup generation as folded;
- clear "unfolded fixup" status;
- optionally append a metadata-only generation/revision.

This can be a follow-up slice.

## 5. Dependency and Verify Semantics

### 5.1 Dependencies

When a parent feature's current patch generation changes:

- children with dependency snapshots pointing at the old parent generation
  surface `parent-generation-stale`;
- hard dependents should be warned or blocked before apply/reconcile depending
  on existing DAG policy;
- soft dependents should warn;
- `--force-amend` on `record` remains the escape hatch for Git rewrite orphan
  detection, not for patch-generation stale checks.

`parent-generation-stale` is a label/overlay, not a lifecycle state. The
amendment ADR may rename it before implementation, but the implementation must
ship one visible dependent-staleness surface.

### 5.2 Verify freshness

Patch-content amendments invalidate verify freshness because either patch hash
or recipe hash changed.

Metadata-only amendments should not invalidate verify unless they touch
`apply-recipe.json`, `artifacts/post-apply.patch`, or fields that ADR-013 says
are freshness inputs.

### 5.3 Reconcile

Reconcile should read the current canonical patch. Historical generations are
audit/provenance, not a replay stack. A future planner may inspect generations
to explain why a patch moved or why a dependency went stale.

## 6. Backwards Compatibility

Existing workflows continue:

- `tpatch amend` keeps its current request/dependency meaning.
- `tpatch record <slug>` keeps recording patch bytes.
- `record --force-amend` keeps its current Git-rewrite dependent-orphan meaning.
- existing feature directories without generation manifests work normally.

The only new behavior should be attached to explicit `tpatch feature patch`
subcommands or to generation metadata written alongside existing record
behavior.

## 7. Implementation Notes

- Do not overload `--force-amend`; it already has a shipped meaning.
- Validate `tpatch feature patch <subverb>` against the allowed v1 values
  `refresh` and `fixup`; reserve `fork` and `fold` for v2.
- Require `--reason` for `fixup`; make it optional for `refresh`.
- Keep plain changed `record <slug>` as implicit `refresh`; do not require
  `tpatch feature patch refresh` for compatibility.
- Store amendment kind in `patch-generations.json`.
- If dependencies are enabled, compare parent generation snapshots before and
  after patch-content amendments.
- Prefer labels/overlays over new lifecycle states.
- Keep `artifacts/post-apply.patch` as full current diff even for fixups.
- Make fold/fork follow-up slices if they would delay refresh/fixup.

## 8. Acceptance Criteria

- `tpatch feature patch refresh <slug>` appends a generation with
  `kind: amend-refresh` when patch bytes change.
- `tpatch feature patch fixup <slug> --reason "..."` appends a generation with
  `kind: amend-fixup` and `fixup_of_generation`.
- `tpatch feature patch fixup <slug>` without `--reason` refuses before patch
  capture.
- Both refresh and fixup update `artifacts/post-apply.patch` to the current full
  diff.
- Existing `tpatch amend` behavior remains compatible.
- `record --force-amend` behavior is unchanged.
- Verify freshness is invalidated for patch/recipe byte changes.
- Dependents recorded against older parent generations surface a stale/broken
  warning or label.
- Metadata-only amend does not change patch bytes.
- No-generation legacy features can still be recorded and amended.
- Docs explain when to refresh, when to fixup, and why fork/fold are deferred.
- Tests cover refresh, fixup, unchanged patch, metadata-only, dependent stale
  detection, verify invalidation, and legacy no-manifest compatibility.

## 9. Open Questions

- What follow-up PRD should own fork/fold?
- Should metadata-only amendments have their own manifest revision number?

## 10. Disputes

None logged.
