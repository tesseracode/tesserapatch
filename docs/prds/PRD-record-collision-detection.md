# PRD — Record Collision Detection — `feat-record-dedup-patches`

**Status**: Draft  
**Date**: 2026-04-30  
**Owner**: G55  
**Milestone**: Record boundary-capture guardrails, post-WP-001  
**Depends on**: [PRD-record-auto-base.md](./PRD-record-auto-base.md) for implementation sequencing. Collision detection can be drafted independently, but strict refusal should ship after `record --auto` gives users the preferred recovery path.

## Related

- [WP-001 Feature Slice Gap](../whitepapers/WP-001-feature-slice-gap.md) — graduated source paper; especially §5.2 rows 3 and 5 and §9.
- [Feature Layout](../feature-layout.md) — canonical patch vs audit-trail contract.
- [Recording Patches](../record.md) — current recovery guidance, to be updated after implementation.
- [Personas](../market-research/personas.md) — Platform Pat's audit pressure explicitly calls WP-001 collisions a comfort blocker.
- [Competitive Landscape](../market-research/competitive-landscape.md) — SMART v0.7 collision target, gbp-pq upstream-applied detection prior art, and §12 versioned-history gap this PRD does not replace.
- Backlog: `feat-record-dedup-patches`, `feat-record-scoped-files`, `feat-patches-subcommand`, `feat-feature-import`.

## 0. Claims Audit

This PRD was drafted after re-reading current record and store behavior:

| Claim | Evidence |
|---|---|
| `recordCmd()` refuses empty working-tree captures, but explicit empty ranges preserve the old success/no-op behavior. | `internal/cli/cobra.go:862-896` |
| `recordCmd()` writes `artifacts/post-apply.patch` on every successful non-empty capture. | `internal/cli/cobra.go:899-902` |
| `recordCmd()` appends `patches/NNN-record.patch` through `Store.WritePatch` on every successful non-empty capture. | `internal/cli/cobra.go:903-915` |
| `Store.NextPatchNumber` and `Store.WritePatch` are append-only and do not compare patch bytes. | `internal/store/store.go:459-490` |
| There is no current cross-feature collision scan before `post-apply.patch` is overwritten. | The current write block goes directly from capture/empty handling to `WriteArtifact` and `WritePatch` (`internal/cli/cobra.go:862-915`). |
| `docs/feature-layout.md` makes `artifacts/post-apply.patch` canonical and `patches/NNN-*.patch` audit-only. | `docs/feature-layout.md:32-67` |
| `--files`, `--from`, `--to`, and `--commit-range` already exist, so recovery hints should point to shipped primitives, not hypothetical ones. `--auto` is the only planned prerequisite from the paired PRD. | `internal/cli/cobra.go:815-855`, `internal/cli/cobra.go:996-1002` |

No implementation has been changed by this PRD.

## Summary

Add byte-identical canonical patch collision detection to `tpatch record`. Before writing a newly captured `artifacts/post-apply.patch`, record computes a content hash and compares it with every other feature's canonical patch. If another feature already has the same bytes, record refuses by default and prints targeted recovery guidance: narrow the range with `--files`, choose the correct base with `--from` or `--auto`, cap with `--to`, or split/import the feature if the wrong shape has already landed.

Same-feature duplicate records are not cross-feature collisions. They should skip the numbered audit snapshot when the canonical patch bytes did not change, while still keeping `artifacts/post-apply.patch` current.

Market grounding: Platform Pat's persona explicitly says WP-001 collisions would be noticed immediately and must be closed before Pat is a comfortable user. The competitive landscape's SMART target requires zero new post-v0.7 collisions, while its gbp-pq prior-art row shows adjacent tools already detect patches that have effectively been applied elsewhere. This PRD stays narrower: exact byte identity across tpatch canonical patches, not semantic `git patch-id` or upstream-applied detection.

## 1. Motivation

WP-001 established that the repeated patch hashes in the A1/A2 case studies were not a feature-slice storage failure. They were a boundary-capture failure that should have been caught at record time:

- `copilot-api`: 10 of 11 features were in repeated canonical patch groups; three Path-B features shared `md5:df5be1df72bf12c599e7b9a902bf5c12`.
- `t3code`: 11 features shared `md5:f491eb4d27083c5b6c7783925c140cf6`, size 137285 bytes.

In both cases, the operator ended up with multiple feature directories that appeared distinct but carried the same replay authority. Once that happens, downstream reconcile, dependency, and future land/commit workflows have no reliable way to know which feature the patch represents.

The CLI should detect this exact shape while the operator still has context.

## 2. Goals / Non-Goals

### Goals

- Detect byte-identical `artifacts/post-apply.patch` collisions across different feature slugs at record time.
- Refuse cross-feature collisions by default, with an explicit override for rare legitimate duplicates.
- Treat same-feature identical re-records as deduplication, not errors.
- Surface recovery hints based on the capture mode that produced the collision.
- Keep `artifacts/post-apply.patch` canonical and `patches/NNN-*.patch` audit-only.
- Provide enough structured output for tests and future harnesses to assert collision details.

### Non-Goals

- Semantic or fuzzy duplicate detection. v1 is exact byte identity only.
- Recovering already-corrupted feature sets automatically. That belongs to `feat-feature-import` or a future resplit/recovery PRD.
- Replacing `--files`, `--from`, `--to`, or `--commit-range`.
- Adding repo-wide operation-log semantics.
- Implementing `tpatch patches --collisions`. This PRD should leave a clean seam for that backlog item, but record-time prevention is first.

## 3. User-Facing Contract

### 3.1 Default Policy

Recommendation: **refuse exact cross-feature collisions by default**.

Rationale: a byte-identical canonical patch across distinct feature slugs almost always means at least one feature boundary is wrong. False positives are possible but rare, and the cost of silently accepting the collision is high: the feature store now has multiple identities for the same replay authority.

Default refusal example:

```text
Error: recorded patch for "dynamic-models" is byte-identical to existing feature patch(es):
  - copilot-cli-provider  sha256=ab12cd34... bytes=42118 files=12

This usually means the record range is too broad.
Try one of:
  tpatch record dynamic-models --auto --files <feature-paths>
  tpatch record dynamic-models --from <feature-base> --files <feature-paths>
  tpatch record dynamic-models --from <base> --to <feature-tip>

To accept an intentional duplicate, rerun with:
  tpatch record dynamic-models --allow-collision "<reason>"
```

`--allow-collision <reason>` is required to proceed. The reason is written to `record.md` and printed to stderr. It is not a new machine-state field in v1.

### 3.2 Same-Feature Deduplication

If the new patch bytes equal the current slug's existing `artifacts/post-apply.patch` and no other feature collides:

- treat the canonical write as a no-op when bytes are identical. If the implementation still calls `WriteArtifact`, it rewrites identical bytes and does not change semantics; no separate "refresh" is required;
- skip the numbered audit snapshot (`patches/NNN-record.patch`);
- continue the normal metadata/status updates;
- print a one-line note:

```text
record: no content change since current artifacts/post-apply.patch; skipping numbered audit snapshot
```

This absorbs the original `feat-record-dedup-patches` behavior without weakening cross-feature collision detection.

### 3.3 Warn vs Refuse

The PRD considered three policies:

| Policy | Behavior | Assessment |
|---|---|---|
| Warn only | Print collision but still write by default | Too weak for the A1/A2 failure. Operators already miss record-time warnings in long agent sessions. |
| Refuse by default with override | Stop the write unless `--allow-collision <reason>` is supplied | Recommended. Prevents store corruption while preserving an explicit escape hatch. |
| Refuse always | No override | Too rigid; legitimate duplicate patches can exist in test fixtures, demonstrations, or staged migrations. |

The implementation should choose the middle policy.

## 4. Detection Algorithm

Run after patch capture and empty-patch handling, before writing the new canonical patch.

0. If the proposed patch string is empty, skip collision scanning entirely. Empty-patch handling is upstream of this PRD: working-tree empty captures refuse, while explicit empty committed ranges keep the existing no-op behavior.
1. Normalize only by using the exact patch string that record is about to write. Do not trim, reorder, or reserialize. `CapturePatchScoped` and `CapturePatchFromCommitsScoped` already return canonical newline-terminated strings.
2. Compute `sha256` and byte length for the proposed patch.
3. Enumerate `.tpatch/features/*/artifacts/post-apply.patch`.
4. For each existing canonical patch:
   - skip missing files;
   - read bytes;
   - compare length and SHA-256;
   - if both match, compare bytes directly before reporting a collision.
5. If the matching slug equals the current slug, classify it as `same_feature_duplicate`.
6. If any matching slug differs from the current slug, classify as `cross_feature_collision`.
7. On `cross_feature_collision` without `--allow-collision`, refuse before writing any feature artifact or status update.
8. On `same_feature_duplicate` only, skip `WritePatch` but continue metadata updates.
9. On no collision, preserve existing write behavior.

The scan is O(number of features) and uses small patch files already on disk. No index is required for v1.

## 5. Recovery Hints

The refusal message should tailor hints to capture mode:

- **Working-tree capture**: tell the user they may have multiple feature edits in the working tree. Suggest `--files <paths>` or splitting the working tree before recording.
- **`--from` without `--files`**: say the range may include unrelated commits. Suggest `--files`, `--to`, or `--commit-range`.
- **`--commit-range` without `--files`**: suggest narrowing by pathspec if the range intentionally includes multiple feature commits.
- **`--auto` without `--files`**: say auto-base found a base but cannot infer feature ownership. Suggest rerunning with `--files`.
- **Collision with many existing features**: recommend stopping and using future import/resplit tooling rather than stamping more feature directories.

Every refusal should include the equivalent explicit range when known. That keeps the diagnostic teachable instead of mysterious.

## 6. Backwards Compatibility

Existing repositories are not scanned or migrated until `record` runs. Existing collisions remain readable and reconcilable exactly as before.

New records change behavior only when they would write a byte-identical canonical patch to another feature. That is a deliberate fail-closed change. The escape hatch is explicit:

```bash
tpatch record <slug> --allow-collision "intentional duplicate for fixture"
```

If maintainers decide this is too strict for the first release, an acceptable staged rollout is:

1. Ship warning-only behind `record_collision_policy: warn` for one minor release.
2. Flip the default to `refuse` when `record --auto` is available and documented.

The recommended end state remains default refusal.

## 7. Implementation Notes

- Add collision scanning near the current `recordCmd()` write block, after validation that `patch != ""` and before `WriteArtifact` / `WritePatch`.
- Keep the collision helper in a small internal function so future `tpatch patches --collisions` can reuse it or port it without re-learning the edge cases.
- Do not compare numbered files under `patches/`; those are audit snapshots and may intentionally repeat. Only canonical `artifacts/post-apply.patch` participates.
- Include file count from `countPatchFiles(patch)` and hash prefix in the diagnostic.
- If `--json` is ever added to `record`, collision details should expose `{slug, path, sha256, bytes, files}`. Do not add a new JSON flag in this PRD.
- Update `docs/record.md` to show the new refusal and its recovery commands.

## 8. Acceptance Criteria

- Recording a patch byte-identical to another feature's `artifacts/post-apply.patch` refuses before writing any artifact for the current feature.
- The refusal lists all colliding slugs, hash prefix, byte count, and file count.
- `--allow-collision <reason>` permits the write and records the reason in `record.md`.
- Re-recording the same feature with unchanged patch bytes does not append a numbered `patches/NNN-record.patch`.
- Re-recording the same feature with changed patch bytes appends the numbered patch as today.
- Collision scan ignores missing `post-apply.patch` files and does not inspect `patches/` history.
- Empty patch results skip the collision scan and preserve the existing empty-capture / empty-range behavior.
- Working-tree, `--from`, `--commit-range`, and `--auto` modes all run the same scan.
- Recovery hints mention `--files` and either `--from` or `--auto` as appropriate.
- Existing range/scoped record tests remain green.
- New tests cover: cross-feature refusal, override, same-feature dedup, multiple colliders, missing artifact ignored, and collision before write.
- Docs and skill assets are updated if user-facing text changes; parity guard passes if assets change.

## 9. Open Questions

- Should the override reason become structured metadata later, perhaps in `status.json` or a `record-collisions.json` audit artifact?
- Should the first implementation expose a read-only `tpatch patches --collisions` scan, or should that remain strictly under `feat-patches-subcommand`?
- Should collision detection ever compare `incremental.patch`, or is canonical `post-apply.patch` sufficient by definition?
- Should near-duplicate detection use patch file lists as a warning-only future extension, or would that create too much noise?
