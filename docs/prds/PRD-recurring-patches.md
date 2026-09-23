# PRD - Recurring Patches - `feat-recurring-patches`

**Status**: Approved (paper design; implementation gated on ADR-recurring-patch-metadata-boundary)
**Date**: 2026-06-26
**Owner**: Core
**Byline**: Q62
**Milestone**: TBD
**Depends on**: [PRD-feature-patch-identity-metadata](./PRD-feature-patch-identity-metadata.md), [PRD-feature-patch-amend](./PRD-feature-patch-amend.md). Implementation requires `ADR-recurring-patch-metadata-boundary`.

## Related

- [WP-001 Feature-slice gap](../whitepapers/WP-001-feature-slice-gap.md)
- [WP-002 Capture and metadata foundation](../whitepapers/WP-002-capture-and-metadata-foundation.md)
- [WP-003 Reconcile safety and middle-pass](../whitepapers/WP-003-reconcile-safety-and-middle-pass.md)
- [WP-004 Auto feature dependency suggestions](../whitepapers/WP-004-auto-feature-dependencies.md)
- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md)
- [Feature Layout](../feature-layout.md)
- [Recording Patches](../record.md)
- [Reconcile Workflow](../reconcile.md)
- [Feature Dependencies](../dependencies.md)
- [ADR-024 patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md)
- [ADR-041 independent capture-event evidence](../adrs/ADR-041-independent-capture-event-evidence.md)
- [ADR-042 ordered recipe no-op proof](../adrs/ADR-042-ordered-recipe-noop-proof.md)
- [PRD-feature-file-claims](./PRD-feature-file-claims.md)
- [PRD-record-capture-modes](./PRD-record-capture-modes.md)
- [PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md)
- [PRD-recipe-generation-authority](./PRD-recipe-generation-authority.md)
- [PRD-tpatch-land](./PRD-tpatch-land.md)

## 0. Claims Audit

This PRD is a proposal. It changes nothing.

| Claim | Evidence |
|---|---|
| A tpatch feature already has one canonical replay patch, and numbered patches are audit snapshots, not replay input. | `docs/feature-layout.md:34-44`, `docs/feature-layout.md:50-67` |
| `record` overwrites the canonical patch and skips same-feature duplicate audit snapshots when bytes are unchanged. | `docs/record.md:1-3`, `docs/record.md:89-90` |
| Patch generations already track repeated patch-byte changes for one feature, including `record`, `amend-refresh`, `amend-fixup`, and `reconcile` kinds. | `internal/store/patch_generations.go:30-55`, `internal/store/patch_generation_kinds.go:5-15`, `internal/workflow/patch_generations.go:28-63` |
| `tpatch feature patch refresh` and `fixup` already expose explicit patch-amendment commands, but they do not record recurrence intent. | `internal/cli/feature_patch.go:14-24`, `internal/cli/feature_patch.go:27-65` |
| Feature claims already model expected feature paths as deterministic advisory scope metadata, and `record --claimed-only` can use them to restrict capture. | `internal/store/claims.go:1-12`, `docs/prds/PRD-feature-file-claims.md:191-234`, `internal/cli/record_capture_modes.go:136-182` |
| `verify` persists only a freshness overlay in `status.json`; it does not have a recurring-churn concept today. | `internal/store/types.go:174-190`, `internal/cli/verify.go:13-42` |
| `reconcile` reads the canonical patch and requires clean-tree preflight; reconcile evidence is designed as audit detail separate from current status. | `docs/reconcile.md:5-14`, `docs/reconcile.md:47-83`, `docs/prds/PRD-reconcile-verdict-evidence.md:187-199` |
| `land` keeps `Tpatch-Feature: <slug>` as the stable feature-to-commit binding and does not change trailer schema for patch generations. | `docs/feature-layout.md:90-94`, `docs/prds/PRD-tpatch-land.md:343-373` |
| The dependency DAG is explicit and hand-curated; tpatch should not infer hidden recurring relationships by default. | `docs/dependencies.md:15-17`, `docs/dependencies.md:20-45` |
| The active dashboard treats recurring patches as a paper-only backlog item, not an active implementation cluster, and adjacent shipped work is capture/metadata plus reconcile safety. | `docs/CLUSTERS.md`, `docs/ROADMAP.md` |

Read-only backlog sweep note: `.tpatch-backlog/backlog.db` has adjacent pending items such as `feat-ci-cd-integration`, `feat-dependabot-bot`, `feat-feature-amend`, `feat-reconcile-metadata-refresh`, and recipe template work, but no explicit recurring-patch or scheduled-recurring feature model.

## Implementation Gate

Implementation requires a small binding ADR before code lands:

```text
ADR-recurring-patch-metadata-boundary
```

The ADR should lock:

- recurrence metadata location and schema;
- no-scheduler boundary;
- deterministic/no-wall-clock metadata policy;
- verify and reconcile interpretation rules;
- no `land` trailer extension;
- whether future generation-level recurrence refs require an ADR-024 amendment.

This PRD may be approved as paper design, but implementation must not start until
`ADR-recurring-patch-metadata-boundary` exists and is accepted. The ADR is
required because this PRD introduces a new persisted per-feature artifact and
cross-command interpretation rules; it is not merely a docs-only convention.

## v0.17 readiness review

This PRD stays **approved as paper design only** after the 2026-09-22 intake.
The design approval is historical; it is **not** an implementation dispatch.

Current readiness constraints to preserve before any code work starts:

1. **Boundary ADR still required.** No implementation begins until
   `ADR-recurring-patch-metadata-boundary` is drafted and accepted.
2. **Shipped producer/event obligations must be inherited, not bypassed.**
   The v0.17-era producer work now governs seven producers plus independent
   capture/event (`E`/`C`) obligations (`ADR-041`, `ADR-042`, and the shipped
   v0.17 notes in `docs/ROADMAP.md`). A future recurring-patches
   implementation must map onto those contracts rather than inventing a
   recurrence-only publication path.
3. **No same-byte generation => no event assumption.** This PRD already treats
   same-byte recurrence attempts as no generation append. Future implementation
   must not smuggle in a new "recurrence run happened" event on those no-op
   paths without the boundary ADR explicitly deciding it.
4. **No scheduler grant.** External scheduling remains out of scope; nothing in
   this PRD grants cron/workflow/bot ownership to tpatch.
5. **No replay-authority grant.** Recurrence metadata explains bounded churn; it
   does not independently authorize apply/replay behavior outside the existing
   `record`, `feature patch refresh`, `verify`, `reconcile`, and `land`
   contracts.

## Summary

Recurring patches should be modeled as **one stable feature with repeated patch generations**, not as a new feature slug for every recurrence.

The gap is that recurrence intent is currently implicit. A user can repeatedly run `tpatch record <slug>` or `tpatch feature patch refresh <slug>`, and `patch-generations.json` can record the byte history. But tpatch cannot yet say: "this feature is expected to recur, only these paths are expected to churn, these recurrence reasons are legitimate, and verify/reconcile should distinguish expected recurring churn from unrelated drift."

Recommended v1: add deterministic, reviewable **recurrence metadata attached to the feature**, plus an explicit feature-local recurrence run list that points at patch generations. Scheduling remains external.

## 1. Problem Statement

Some feature patches are not one-off changes. Examples:

- generated timestamps in code or docs;
- version banners;
- generated metadata rotation;
- periodic compatibility shims;
- files that record current upstream, build, or release information.

Today these can be forced into existing commands:

```bash
tpatch feature patch refresh build-info
tpatch record build-info --claimed-only
tpatch land build-info
```

That preserves the feature slug and patch history, but the intent is invisible. A reviewer cannot tell whether the latest `amend-refresh` is legitimate recurring churn or a suspicious broad refresh that hid drift. Reconcile cannot distinguish "expected generated-file update under this feature" from unrelated upstream movement. Verify can detect patch hash drift, but not whether the drift is inside declared recurring paths.

The goal is not to make noisy generated changes disappear. A recurring patch is legitimate only when it has:

1. stable feature identity;
2. expected paths or claims;
3. a clear enumerated reason to recur;
4. reviewable patch generations;
5. no silent auto-application by default.

## 2. Goals / Non-goals

### Goals

1. Model recurring updates as repeated generations under the same feature slug.
2. Add deterministic recurrence metadata that makes recurring intent explicit.
3. Require expected paths and/or claim IDs before recurrence metadata is valid.
4. Let `record`, `feature patch refresh`, `verify`, `reconcile`, and `land` consume recurrence metadata without changing their core authority model.
5. Keep scheduling outside tpatch v1.
6. Keep recurrence metadata reviewable, enum-heavy, and free of raw prompts, transcripts, source bodies, or uncontrolled notes.

### Non-goals

1. No scheduler, daemon, cron installer, GitHub Actions generator, or bot in v1.
2. No silent auto-application by default.
3. No new feature per recurrence.
4. No template/generator recipe engine in v1.
5. No broad "generated files are always safe" exemption.
6. No change to `artifacts/post-apply.patch` as canonical replay authority.
7. No change to the `land` trailer block.
8. No new dependency edge kind.

## 3. Design Options Evaluated

| Option | Verdict | Why |
|---|---|---|
| A - No new feature | Insufficient alone | Existing record/refresh/generation primitives cover patch history, but recurrence intent and expected churn remain implicit. |
| B - Recurrence metadata on feature | Recommended | Smallest explicit model: stable feature identity plus expected paths, reason code, cadence/trigger intent, and last generation linkage. |
| C - Recurring patch command | Defer | Useful UX later, but a large command surface would imply tpatch owns running recurrences. |
| D - Template / generator recipe | Defer | Best reproducibility story, but too complex before recurrence metadata proves useful. |
| E - External scheduler integration | Use as boundary | Cron/GitHub Actions/wrapper bots may invoke tpatch, but tpatch should only expose deterministic metadata and idempotent commands. |

## 4. User-facing Contract

V1 should start with metadata and status/reporting, not automation.

V1 command surface:

```bash
tpatch feature recurring mark <slug> \
  --reason-code generated-timestamp \
  --kind manual \
  --claim-id 8f31c0a19b2d \
  --path docs/build-info.md

tpatch feature recurring status [<slug>] [--json]
tpatch feature recurring unmark <slug>
```

`mark` records recurrence metadata only. It does not run `record`, mutate source files, refresh a patch, land a commit, or install a scheduler.

Claims are preferred over literal paths because `claims.json` is the shipped
feature-scope primitive. `--path` remains available as a fallback for legacy
features without claims or for an initial mark before claims are added.

Recurring updates continue to use existing write paths:

```bash
tpatch record build-info --claimed-only
tpatch feature patch refresh build-info
tpatch land build-info
```

The implementation may later add a convenience alias:

```bash
tpatch feature recurring run <slug>
```

But v1 should not include `run` unless it is defined as an explicit wrapper around existing commands with dry-run output and no scheduler semantics.

## 5. Data Model / Metadata

Recommended artifact:

```text
.tpatch/features/<slug>/recurrence.json
```

Recurrence is user-declared feature metadata, not a generated patch artifact.
The location comparison:

| Location | Verdict | Reason |
|---|---|---|
| `.tpatch/features/<slug>/recurrence.json` | Recommended | Sits beside `claims.json`, `status.json`, request/spec files, and other feature identity metadata. It is easy to review and does not imply patch-byte derivation. |
| `.tpatch/features/<slug>/artifacts/recurrence.json` | Rejected for v1 | `artifacts/` is already associated with generated patch/recipe/diff outputs. Recurrence intent is declared metadata, not derived output. |
| `status.json.recurrence` | Rejected for v1 | Would widen lifecycle state into a metadata bucket and make every status read parse recurrence policy. |
| `claims.json` extension | Rejected | Claims express expected scope; recurrence expresses why and when that scope should be refreshed. Coupling them would blur responsibilities. |
| `patch-generations.json.refs.recurrence` | Deferred | Generation-level links may be useful later, but changing the generation schema requires ADR-024 amendment. |

Top-level shape:

```json
{
  "version": 1,
  "feature": "build-info",
  "recurrence": {
    "kind": "manual",
    "reason_code": "generated-timestamp",
    "expected_paths": ["docs/build-info.md"],
    "expected_claim_ids": ["8f31c0a19b2d"],
    "cadence_hint": "monthly",
    "scheduler": "external",
    "review_policy": "manual-required",
    "last_run_generation": 7,
    "runs": [
      {
        "generation": 7,
        "generation_id": "pg_9cb6578d11c8",
        "reason_code": "generated-timestamp",
        "expected_scope_match": "matched"
      }
    ]
  }
}
```

Field rules:

| Field | Rule |
|---|---|
| `version` | Required schema version. |
| `feature` | Must match the feature slug. |
| `kind` | Enum: `manual`, `external-scheduled`, `on-release`, `on-upstream-change`. This is intent, not execution authority. `external-scheduled` means scheduled outside tpatch. |
| `reason_code` | Enum: `generated-timestamp`, `version-banner`, `generated-metadata`, `compatibility-shim`, `upstream-build-info`, `release-info`, `other`. `other` has no free-text companion in v1. |
| `expected_paths` | Stable-sorted repo-relative paths. No absolute paths, no `.tpatch/`, no installed skill surfaces. |
| `expected_claim_ids` | Stable-sorted claim IDs from `claims.json`; preferred over `expected_paths`. At least one of `expected_paths` or `expected_claim_ids` is required. |
| `cadence_hint` | Optional enum: `none`, `daily`, `weekly`, `monthly`, `per-release`, `per-upstream-change`. It is descriptive only; no next-run time is stored. |
| `scheduler` | Enum: `none`, `external`. V1 never stores cron expressions, workflow YAML, URLs, tokens, or provider-specific secrets. |
| `review_policy` | V1 writes `manual-required`. Reserved future value: `auto-open-pr`. |
| `last_run_generation` | Optional integer equal to the highest `runs[].generation`. No timestamp. |
| `runs` | Stable-sorted recurrence run records. Only generations listed here count as recurrence runs in v1. |
| `runs[].generation` | Existing patch generation number. Must exist in `patch-generations.json`. |
| `runs[].generation_id` | Existing patch generation ID. Must match the referenced generation when present. |
| `runs[].reason_code` | Reason code for this run. Defaults to the top-level reason code when omitted by the user-facing command, but is persisted explicitly. |
| `runs[].expected_scope_match` | Enum: `matched`, `paths-exceeded`, `claims-missing`, `unknown`. Successful v1 recurring writes should append only `matched`; other values may appear in diagnostics or future imported metadata. |

The artifact is deterministic:

- stable JSON indentation and key order;
- stable-sorted arrays;
- no wall-clock timestamps;
- no absolute paths;
- no raw source snippets;
- no prompts, transcripts, provider responses, cron expressions, tokens, URLs, or uncontrolled free-text notes.

### 5.1 Recurrence history semantics

V1 uses an explicit `runs[]` list. A patch generation is considered a recurrence
run only when it appears in `runs[]`; non-recurring corrections to the same
feature are normal patch generations but are not recurrence runs.

Rules:

- `runs[]` is stable-sorted by `generation`.
- `last_run_generation` is a denormalized convenience field and must equal the
  highest generation in `runs[]`; if `runs[]` is empty, it must be omitted or 0.
- A same-byte recurrence attempt appends no patch generation and no run entry.
- A bytes-changing refresh outside expected scope must not append a `matched` run.
- Historical generations before `recurrence.json` exists are not backfilled in v1
  unless a future migration PRD defines the audit guarantees.

### 5.2 Patch-generation linkage

When a recurring update changes patch bytes, the normal patch generation is still appended under `patch-generations.json`.

V1 should avoid adding a new patch-generation kind. The update remains:

- `record` for first capture;
- `amend-refresh` for changed repeated records or explicit refresh;
- `reconcile` when reconcile refreshes the canonical patch.

If an implementation needs a generation-level marker, add a bounded `refs.recurrence` or `recurrence_run` field only after an ADR updates the generation manifest boundary. Until then, `recurrence.json:runs[]` can point at existing generations without rewriting historical generation entries.

## 6. Interactions

### 6.1 `record`

For a feature with recurrence metadata:

1. `record` continues to capture patches exactly as today.
2. `record --claimed-only` is the preferred path when recurrence metadata has `expected_claim_ids`.
3. If captured touched paths are outside `expected_paths` and outside referenced claims, human output should warn.
4. In a future strict mode, unexpected paths may refuse, but v1 should remain warn-only unless `--claimed-only` already refuses through existing claim semantics.
5. A changed recurring update appends the normal patch generation under the same feature.

Same-byte recurrence attempts should remain no-ops under existing generation de-duplication. A "nothing changed this cycle" event is not a patch generation.

### 6.2 Patch generations

Patch generations remain the audit trail of bytes. Recurrence metadata explains why some generations are expected to recur.

Acceptance behavior:

- first recurring update recorded after metadata exists appends or reuses the next normal generation;
- repeated updates do not create new slugs;
- unchanged bytes do not append a fake generation;
- `runs[].generation` points only to existing generation numbers.
- `last_run_generation` equals the highest run generation.

### 6.3 Amend / refresh

`tpatch feature patch refresh <slug>` is the right primitive for most recurring changes because it says "same logical patch, current bytes changed."

`fixup` should not be the default recurrence operation. A recurring timestamp refresh is not an unfolded fixup; it is a refresh of the same logical feature.

`manual-metadata` should not be repurposed as "recurrence run." Current implementation treats `manual-metadata` as a recognized but not writable generation kind; recurrence metadata belongs in its own feature artifact unless the generation-manifest ADR is revised.

### 6.4 Verify

`verify` should distinguish three cases for recurring features:

| Case | Expected result |
|---|---|
| Patch/recipe hashes match current verify snapshot | Existing `verified-fresh` behavior. |
| Patch hash changed only inside expected recurring paths and a new generation exists | Warn or label as `recurring-update-pending-verify`; still require review before fresh. |
| Patch hash changed outside expected paths/claims | Treat as normal unexpected drift; do not hide behind recurrence metadata. |

V1 can implement this as a derived label or verify report warning. It should not mark a feature fresh merely because recurrence metadata exists.

### 6.5 Reconcile

Reconcile should not classify recurring churn as upstream drift by default.

Rules:

1. Reconcile still reads `artifacts/post-apply.patch` as canonical.
2. Reconcile evidence may include a bounded reference to recurrence metadata when touched paths are wholly expected.
3. Unexpected path movement remains normal reconcile drift or conflict.
4. A recurring feature must not auto-retire as upstreamed merely because upstream also changes a generated file or because the patch-id detector finds a match on generated-file churn. Existing upstreamed confirmation rules still apply.
5. Provider-assisted reconciliation should receive recurrence summaries only as structured enums/paths, not raw recurrence prose.

### 6.6 Land

`land` keeps the same trailer block:

```text
Tpatch-Feature: <slug>
Tpatch-Patch-SHA: <sha256>
Tpatch-Recipe-SHA: <sha256 | none>
Tpatch-Base-Commit: <sha>
```

Recurring updates for the same logical patch produce additional Git commits with the same `Tpatch-Feature: <slug>` trailer and different patch SHAs when bytes change. No `Tpatch-Recurrence` trailer is needed in v1.

### 6.7 Dependencies

Recurrence does not add a dependency kind.

If a recurring parent changes patch generation, existing parent-generation-stale behavior remains the right overlay for children. Recurrence metadata can explain why the parent generation changed; it must not suppress stale-child warnings automatically.

### 6.8 `record --auto`

External schedulers will often run committed-range capture after a bot or workflow updates generated files. `record --auto` remains useful, but recurrence metadata should bias documentation toward scoped capture:

```bash
tpatch record build-info --auto --claimed-only
```

or:

```bash
tpatch record build-info --auto --files docs/build-info.md
```

### 6.9 Generated-file / timestamp churn

Generated-file churn is acceptable only when bounded by recurrence metadata and expected paths/claims. Recurrence is not an excuse for:

- broad generated diffs across unclaimed paths;
- source changes unrelated to the recurrence reason;
- changing feature dependencies silently;
- skipping review because a change is "just generated."

## 7. Backwards Compatibility

Existing repositories have no `recurrence.json`; that means "not declared recurring," not corruption.

Existing recurring-like workflows can continue with `record`, `feature patch refresh`, `verify`, `reconcile`, and `land`. Adding recurrence metadata does not migrate existing patch generations, rewrite `status.json`, or change land trailers.

Malformed recurrence metadata should produce explicit diagnostics for commands that consult it. It must not make the feature status unreadable.

## 8. Implementation Notes

- Keep recurrence metadata out of `status.json` in v1 to avoid turning lifecycle state into a metadata bucket.
- Reuse claim path normalization and path-safety helpers.
- Validate `expected_claim_ids` against `claims.json` when present.
- Prefer `expected_claim_ids`; use `expected_paths` as a fallback, not the primary recommendation.
- Validate `runs[].generation` and `last_run_generation` against `patch-generations.json`.
- Stable-sort all arrays.
- Avoid storing cron syntax, workflow paths, URLs, job IDs, commit messages, prompt text, or free-text reasons.
- Start with warn-only checks. A strict enforcement mode should be a follow-up PRD.
- If generation-level recurrence markers are desired, draft an ADR amendment to `ADR-024-patch-generation-manifest-boundary` before changing `patch-generations.json`.
- Refuse or warn before appending a `matched` recurrence run when captured paths exceed expected paths/claims.
- Treat missing or malformed `reason_code`, missing expected scope, malformed metadata, or impossible generation refs as metadata errors, not as permission to proceed silently.

## 9. Acceptance Criteria

1. A recurring update is recorded as a new patch generation under the same feature when patch bytes change.
2. A recurring update does not create a new feature slug.
3. Recurrence metadata is deterministic, stable-sorted, and contains no wall-clock-only scheduling fields.
4. Recurrence metadata requires expected paths and/or expected claim IDs.
5. `expected_claim_ids` are preferred and validated against `claims.json`; `expected_paths` remain a fallback.
6. `runs[]` records every v1 recurrence run, and `last_run_generation` equals the highest run generation.
7. A generation not listed in `runs[]` is not classified as a recurrence run.
8. `last_run_generation` pointing to a non-existent generation reports a clear metadata diagnostic.
9. Recurrence metadata with missing/malformed `reason_code` reports a clear metadata diagnostic.
10. Recurrence metadata with no expected paths and no expected claim IDs reports a clear metadata diagnostic.
11. Captured paths outside expected paths/claims are reported as unexpected drift and must not append a `matched` recurrence run.
12. Malformed recurrence metadata reports a clear diagnostic and preserves existing feature readability.
13. `verify` can distinguish expected recurring-path churn from unexpected drift and does not mark drift fresh solely because recurrence metadata exists.
14. `reconcile` does not incorrectly classify recurring generated-file churn as upstream drift or upstreamed intent without normal evidence.
15. If upstream also changes a generated file and the patch-id detector finds a match, existing upstreamed confirmation rules still apply; recurrence metadata does not auto-retire the feature.
16. `land` keeps `Tpatch-Feature` stable across recurrences and does not add recurrence trailers in v1.
17. Existing `depends_on` and parent-generation-stale overlays remain active for recurring features.
18. `record --auto` and external scheduler workflows are documented as invocation mechanisms, not as scheduler ownership by tpatch.
19. No scheduler, daemon, hook installer, GitHub Actions generator, or bot is introduced by v1.
20. Tests cover deterministic metadata write/read, invalid paths, missing expected scope, malformed reason code, invalid generation refs, recurring refresh generation linkage, same-byte no-op behavior, out-of-scope path behavior, verify warning behavior, reconcile evidence interaction, patch-id generated-file match behavior, and land trailer stability.

## 10. Open Questions

1. Should a future generic `feature metadata` command alias the v1 `feature recurring` surface, or should recurrence remain a dedicated namespace?
2. Should `cadence_hint` ship in v1, or should v1 omit cadence entirely and store only `kind` plus `scheduler=external|none`?
3. Should recurring-path verify output be a new label, a warning in the verify report, or both?
4. Should future ADR-024 work add generation-level recurrence refs, or is the feature-root `runs[]` list enough?

## 11. Disputes

- **Convenience run command**: This PRD locks v1 to `mark/status/unmark` and intentionally does not propose a v1 `run` command. If reviewers consider `run` essential, it must be constrained to an explicit wrapper and must not imply scheduling.
- **Cadence**: `cadence_hint` is descriptive and deterministic, but even an enum can be misread as a scheduler contract. Omitting it is a viable stricter v1.
