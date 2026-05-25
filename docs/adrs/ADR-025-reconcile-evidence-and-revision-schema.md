# ADR-025 — Reconcile evidence and revision schema

**Status**: Accepted
**Date**: 2026-05-24

> **Note**: ADR-025 covers the WP-003 cluster's reconcile evidence and
> revision schema (PRDs 1, 2, 3 directly; PRDs 4–9 consume the same
> records). It is distinct from `ADR-024-patch-generation-manifest-boundary`
> (WP-002 cluster). The two ADRs are unrelated but adjacent in number for
> historical reasons (see the broker's 2026-05-16 supervisor LOG renumbering
> decision).

**Source PRDs**: [PRD-reconcile-verdict-evidence](../prds/PRD-reconcile-verdict-evidence.md)
(PRD 1, keystone), [PRD-upstreamed-confirmation-gate](../prds/PRD-upstreamed-confirmation-gate.md)
(PRD 2), [PRD-reconcile-revision-pass-log](../prds/PRD-reconcile-revision-pass-log.md)
(PRD 3). PRDs 4–9 also ship under this ADR.

**Related ADRs**: [ADR-024](./ADR-024-patch-generation-manifest-boundary.md)
(sibling cluster evidence artifact), ADR-011 (feature dependencies), ADR-013
(freshness overlays), ADR-018 (collision/refusal philosophy), ADR-019 (`tpatch
land` trailers), ADR-022 (phase-1.5 detector default deferral).

## Context

WP-003 was accepted as a nine-PRD paper cluster on 2026-05-16. The supervisor
LOG makes this ADR the single cluster gate and enumerates the locked contracts:
`reconcile-evidence.jsonl`, `reconcile-revisions.jsonl`, confirmation-gate state
semantics, patch-id auto-confirm defaults, and the privacy boundary
(`docs/supervisor/LOG.md:635-645`). The same entry records the `.jsonl` drift
fix (`docs/supervisor/LOG.md:653-655`) and the cross-cluster requirement that
WP-003 align with WP-002's `patch-generations.json`
(`docs/supervisor/LOG.md:647-650`).

The cluster dashboard shows WP-002 shipped and its Wave beta prerequisite cleared
(`docs/CLUSTERS.md:43-68`), while WP-003 remains accepted and blocked on this ADR
(`docs/CLUSTERS.md:75-113`). WP-003's whitepaper names this as the one cluster
ADR, covering PRDs 1–3 directly and PRDs 4–9 through the same records
(`docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md:67-78`). Its privacy
anchor forbids source bodies, prompt/transcript text, vectors, and embeddings
(`docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md:80-91`).

Current code persists current reconcile truth in `status.json`.
`ReconcileSummary` stores attempt/upstream fields, outcome, resolver fields,
labels, and optional `PatchIDMatch`, but no evidence bundle or review verdict
(`internal/store/types.go:249-279`). Lifecycle truth is the separate
`FeatureState`, including `upstream_merged` (`internal/store/types.go:5-19`).
`PatchIDMatch` already persists `our_patch_id`, `matched_upstream_sha`, scan
range/count, and additional matches (`internal/store/types.go:363-374`). SPEC and
user docs describe the four classical reconcile phases (`SPEC.md:133-153`,
`docs/reconcile.md:7-14`); PRD 1 adds phase-1.5 and phase-3.5 to the evidence
enum (`docs/prds/PRD-reconcile-verdict-evidence.md:150-160`).

ADR-024 is the structural precedent: same per-feature artifact directory
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:18-24`),
content-addressed `pg_<12hex>` IDs (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:77-105`),
`git patch-id --stable` only (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:189-216`),
writer-refuses / reader-warns / reconcile-distrusts malformed handling
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:218-251`), and reserved
`refs` slots (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:279-315`).
This ADR applies those choices to append-only JSONL.

This is paper-only. It changes no code, CLI behavior, assets, PRDs, supervisor
log, or cluster dashboard.

## Decisions

### D1 — Evidence attempts live in append-only `reconcile-evidence.jsonl`

Evidence attempts live at `.tpatch/features/<slug>/artifacts/reconcile-evidence.jsonl`.
The file is JSONL: one newline-terminated JSON object per line, no wrapping array
or top-level manifest. Each line is one evidence attempt for one reconcile
verdict. The extension is `.jsonl`, not `.json`.

**Rationale**. PRD 1 names the path and one-line-per-attempt shape
(`docs/prds/PRD-reconcile-verdict-evidence.md:100-130`). The supervisor LOG locks
`.jsonl` as the drift fix (`docs/supervisor/LOG.md:653-655`). Keeping evidence
outside `status.json` preserves `ReconcileSummary` as current state, matching PRD
1's note that evidence is audit detail (`docs/prds/PRD-reconcile-verdict-evidence.md:187-200`).

**Alternatives considered**. A single `.json` document was rejected because every
append rewrites history and contradicts the drift fix. One file per attempt was
rejected as harder to inspect and aggregate. Embedding in `status.json` was
rejected because current state and unbounded evidence history have different
lifecycles.

### D2 — Evidence entries use per-line `schema_version: 1` and stable canonical JSON

Every evidence line carries `schema_version: 1`. Writers emit lexicographically
sorted keys and sorted arrays when ordering is not semantic. v1 readers are
strict: unknown fields, missing required fields, unsupported versions, or unknown
enum values make the line malformed.

Required v1 fields: `schema_version`, `feature_slug`, `attempt_id`,
`upstream_ref`, `upstream_commit`, `base_commit`, `raw_reconcile_verdict`,
`phase`, `evidence_kind`, `confidence`, `matched_paths`, `matched_operations`,
`match_origin`, `upstream_commit_refs`, `pre_reconcile_presence`,
`requires_confirmation`, `reason_code`. Known optional fields: `git_patch_id`,
`git_patch_id_algorithm`, `matched_upstream_sha`, `scanned_range`,
`scanned_count`, `additional_matches`, `refs`.

**Rationale**. PRD 1 requires deterministic stable-key JSONL, sorted arrays, and
unknown-version refusal (`docs/prds/PRD-reconcile-verdict-evidence.md:187-222`).
ADR-024 D9 chose strict v1 reads for the sibling schema
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:279-315`). Per-line
versioning keeps copied or aggregated JSONL records self-describing.

**Alternatives considered**. A first-line header was rejected because it creates
two JSONL record shapes. Permissive readers were rejected because ignored fields
become silent schema drift. Insertion-order JSON was rejected because it is not a
portable byte-identity contract.

### D3 — `attempt_id` is content-addressed as `re_<12hex>`

`attempt_id` is `"re_" + first 12 lowercase hex chars of SHA-256(canonical JSON
of identity fields)`, excluding `attempt_id` itself. The hash input is:
`schema_version`, `feature_slug`, `upstream_ref`, `upstream_commit`,
`base_commit`, `raw_reconcile_verdict`, `phase`, `evidence_kind`, `confidence`,
`match_origin`, `pre_reconcile_presence`, `requires_confirmation`, `reason_code`,
sorted `matched_paths`, sorted `matched_operations`, sorted
`upstream_commit_refs`, and, when present, canonical patch-id fields and `refs`.
A duplicate ID with byte-identical payload is a no-op; a duplicate ID with a
different payload is malformed.

**Rationale**. PRD 1 requires a content-derived attempt ID from slug, upstream
commit, base commit, raw verdict, phase, and normalized evidence entries
(`docs/prds/PRD-reconcile-verdict-evidence.md:132-143`). ADR-024's `pg_<12hex>`
pattern is the sibling precedent (`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:77-105`).

**Alternatives considered**. Full `sha256:<64hex>` IDs were rejected as too noisy
for terminal and revision-log links. Monotonic numbers were rejected because they
are ordering primitives, not equivalence primitives. Time-based IDs were rejected
because PRD 1 requires byte-identical reruns (`docs/prds/PRD-reconcile-verdict-evidence.md:213-216`).

### D4 — Evidence enums are closed for v1

v1 `phase` values are `phase-1`, `phase-1.5`, `phase-2`, `phase-3`,
`phase-3.5`, `phase-4`.

v1 `evidence_kind` values are `reverse-apply`, `patch-id-match`,
`recipe-operation-match`, `provider-semantic`, `forward-apply`, `file-novelty`,
`hunk-overlap`, `blocked-classification`, `path-restructure`, `manual-review`,
`unknown`.

v1 `confidence` values are `high`, `medium`, `low`, `unknown`; `match_origin`
values are `upstream`, `fork`, `sibling-feature`, `unknown`, `mixed`;
`pre_reconcile_presence` values are `present`, `absent`, `unknown`,
`not-checked`.

**Rationale**. PRD 1 enumerates phase and evidence kind values and defines the
other evidence enums (`docs/prds/PRD-reconcile-verdict-evidence.md:132-173`). The
classical phases remain the SPEC tree (`SPEC.md:133-153`); the extra values name
already-designed phase-1.5 and phase-3.5 slots without creating lifecycle states.
The supervisor LOG requires this ADR to lock these enums
(`docs/supervisor/LOG.md:637-640`).

**Alternatives considered**. Open text values were rejected because aggregators
and confirmation gates need stable buckets. Splitting phase-1.5 into detector
subphases was rejected because current docs/code use one phase-1.5 detector slot
(`internal/store/types.go:340-348`).

### D5 — Patch-id evidence uses ADR-024's stable algorithm and field names

Patch-id evidence uses `evidence_kind: "patch-id-match"`, `git_patch_id`,
`git_patch_id_algorithm: "git-patch-id-stable"`, `matched_upstream_sha`,
`scanned_range`, `scanned_count`, and optional `additional_matches`. The patch ID
is Git's `git patch-id --stable` output.

**Rationale**. ADR-024 D6 locks `git_patch_id` and the
`git-patch-id-stable` marker for `patch-generations.json`
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:189-216`). Current
`PatchIDMatch` already uses `matched_upstream_sha` and scan fields
(`internal/store/types.go:363-374`). The detector PRD requires `git patch-id
--stable` and shows the same JSON field name (`docs/prds/PRD-patch-already-upstream-detector.md:241-255`,
`docs/prds/PRD-patch-already-upstream-detector.md:293-305`).

**Alternatives considered**. Renaming fields was rejected as cross-artifact drift.
Omitting the algorithm marker was rejected because future structural fingerprints
need a different field or version. Unstable Git patch-id was rejected by the
sibling ADR and detector PRD.

### D6 — Revision corrections live in append-only `reconcile-revisions.jsonl`

Review corrections live at `.tpatch/features/<slug>/artifacts/reconcile-revisions.jsonl`.
The file is JSONL with one newline-terminated JSON object per correction. Each
line carries `schema_version: 1`, sorted keys, `entry_id`, `feature_slug`,
`evidence_attempt_id`, `raw_reconcile_verdict`, `review_verdict`,
`final_feature_state`, `action_taken`, `reason_code`, `validation_refs`, optional
`supersedes_entry_id`, and optional `refs`. `evidence_attempt_id` links to the
`re_<12hex>` attempt being revised; for legacy corrections without evidence it is
empty and `reason_code` must explain why. `entry_id` is `rr_<12hex>` using the
same content-addressing rule as D3, excluding `entry_id` itself.

**Rationale**. PRD 3 names the path and one-correction-per-object shape
(`docs/prds/PRD-reconcile-revision-pass-log.md:82-106`). It requires append-only
entries, evidence links, explicit supersedure, JSONL determinism, and status as
the true state store (`docs/prds/PRD-reconcile-revision-pass-log.md:152-168`).

**Alternatives considered**. Git-history-only revision tracking was rejected
because PRD 3 exists to avoid prose archaeology. A repo-wide log was rejected
because PRD 3 leaves aggregate summaries open (`docs/prds/PRD-reconcile-revision-pass-log.md:182-185`).
An optional evidence link with no explanation was rejected because corrections
must be auditable.

### D7 — Revision verdict and action enums are closed for v1

`review_verdict` in `reconcile-revisions.jsonl` accepts exactly `confirmed`,
`false-positive`, `false-negative`, `inconclusive`, `deferred`.

`action_taken` accepts exactly `none`, `confirmed-retired`, `reapplied`,
`reapplied-and-recorded`, `implemented`, `deferred`, `skipped`,
`cleanup-needed`.

Reason codes are enumerated strings in v1. Free-text notes are deferred.

**Rationale**. These enum sets are binding from the supervisor LOG
(`docs/supervisor/LOG.md:637-641`) and match PRD 3
(`docs/prds/PRD-reconcile-revision-pass-log.md:108-138`). PRD 3 defers free-text
notes to the future capture-context privacy ADR
(`docs/prds/PRD-reconcile-revision-pass-log.md:137-138`).

**Alternatives considered**. Reusing status-level `confirmed-upstreamed` /
`rejected-upstreamed` strings was rejected because revision review has a broader
taxonomy. Arbitrary action text was rejected because it breaks aggregation and
privacy boundaries.

### D8 — Confirmation adds `review_verdict` to `ReconcileSummary`; final state reuses `status.json:state`

PRD 2 adds one status field: `ReviewVerdict` on `ReconcileSummary`, persisted as
`json:"review_verdict,omitempty"`. Its type is a string enum with empty string as
the default/unset value. v1 non-empty status values are `confirmed-upstreamed`
and `rejected-upstreamed`. `final_feature_state` is not a new status field; it
reuses `FeatureStatus.State` / `status.json:state`. No new lifecycle states are
introduced.

**Rationale**. PRD 2 explicitly states this storage split
(`docs/prds/PRD-upstreamed-confirmation-gate.md:119-138`). The supervisor LOG
repeats it and confirms no new lifecycle states (`docs/supervisor/LOG.md:626-642`).
Current `ReconcileSummary` lacks this field (`internal/store/types.go:249-279`),
and `FeatureState` already includes `upstream_merged` (`internal/store/types.go:5-19`).

**Alternatives considered**. Adding `final_feature_state` to `ReconcileSummary`
was rejected as a second lifecycle source. New `ReconcileOutcome` values were
rejected because outcomes and labels are not lifecycle state (`internal/store/types.go:31-62`).
Storing confirmation only in the revision log was rejected because status needs a
cheap latest review bit.

### D9 — Reachable patch-id matches auto-confirm and add no new override flag

When evidence has `evidence_kind: "patch-id-match"`, a populated
`matched_upstream_sha`, and that SHA is reachable from the target upstream range,
the confirmation gate defaults to `review_verdict=confirmed-upstreamed`. If
reachability cannot be proven, manual confirmation is required. v1 adds no new
override flag. Existing `--auto-drop-merged` remains the feature retirement/removal
surface; auto-confirming evidence is not the same as auto-removing the feature.

**Rationale**. PRD 2 requires reachable patch-id matches to auto-confirm and
unreachable matches to require manual confirmation
(`docs/prds/PRD-upstreamed-confirmation-gate.md:139-167`). The detector PRD
established `--auto-drop-merged` for opt-in removal
(`docs/prds/PRD-patch-already-upstream-detector.md:219-240`). ADR-022 explains
that v0.8.1 kept that flag conservative and did not auto-enable the detector
(`docs/adrs/ADR-022-detector-default-deferral.md:24-29`,
`docs/adrs/ADR-022-detector-default-deferral.md:66-70`). Current CLI help keeps
`--auto-drop-merged` off by default and no-op unless phase-1.5 fires
(`internal/cli/cobra.go:1876-1877`).

**Alternatives considered**. Human confirmation for every patch-id match was
rejected because it discards deterministic high-confidence evidence. Treating
`--auto-drop-merged` as auto-confirm for all upstreamed candidates was rejected
because provider/operation evidence lacks the same commit proof. Adding a new
`--no-auto-confirm-patch-id` flag was rejected as unnecessary v1 surface.

### D10 — Privacy is a hard boundary for both JSONL artifacts

Neither JSONL artifact may persist raw source bodies, provider transcripts,
prompt text, vectors, or embeddings. Allowed v1 material is limited to paths,
hashes, operation IDs, enum reason codes, verdicts, upstream refs/commits,
counts, validation references, and cross-artifact IDs.

**Rationale**. WP-003 inherits the deferred capture-context privacy boundary
(`docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md:80-91`). PRD 1 and
PRD 3 both forbid raw source/transcript/embedding persistence
(`docs/prds/PRD-reconcile-verdict-evidence.md:92-99`,
`docs/prds/PRD-reconcile-verdict-evidence.md:201-220`,
`docs/prds/PRD-reconcile-revision-pass-log.md:75-80`). The supervisor LOG checked
cluster privacy alignment (`docs/supervisor/LOG.md:626-627`).

**Alternatives considered**. Source snippets were rejected because Git diffs own
source bodies. Transcript excerpts were rejected because transcript persistence is
deferred. Vector IDs/embeddings were rejected because `PRD-patch-vector-index` is
future work.

### D11 — Malformed JSONL mirrors ADR-024's writer-refuses / reader-warns split

A JSONL artifact is malformed on JSON parse failure, non-newline-terminated final
object, schema violation, unsupported version, unknown field, unknown enum,
duplicate ID with different payload, or locally verifiable cross-link
inconsistency. Writers refuse to append and return an `errors.Is`-compatible
sentinel analogous to `store.ErrMalformedManifest`. Read-only status/list paths
warn and continue to load `status.json`; downstream automation must distrust
identity fields from malformed artifacts. v1 does not truncate or repair partial
writes.

**Rationale**. ADR-024 D7 defines this asymmetry
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:218-251`). The current
manifest reader already wraps JSON/schema failures in `ErrMalformedManifest`
(`internal/store/patch_generations.go:24-28`, `internal/store/patch_generations.go:90-107`).
PRD 1 requires corrupt evidence artifacts not to block status loading
(`docs/prds/PRD-reconcile-verdict-evidence.md:198-212`), and PRD 3 requires line
number reporting while preserving valid entries for review/list output
(`docs/prds/PRD-reconcile-revision-pass-log.md:158-180`).

**Alternatives considered**. Warn-and-append was rejected because it compounds
corruption. Refuse all commands was rejected because `status.json` remains the
current truth. Auto-repair was rejected because append-only audit files must not
be silently rewritten.

### D12 — `refs` is an optional compatible cross-artifact link block

Evidence and revision entries may carry optional `refs`. If present in v1, it has
exactly these string keys: `patch_generation_id`, `patch_generations_path`,
`anchors`, `fingerprints`, `relations`, `vector_manifest`. Empty strings mean no
v1 producer. Writers omit `refs` when all values are empty; if included, the
shape is strict. `patch_generation_id` links to ADR-024's `pg_<12hex>` generation
ID when known. The other keys mirror ADR-024's reserved middle-pass references.

**Rationale**. WP-003 says `patch-generations.json` and `reconcile-evidence.jsonl`
are sibling artifacts that may share `refs` (`docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md:54-65`).
ADR-024 reserves `anchors`, `fingerprints`, `relations`, and `vector_manifest`
(`docs/adrs/ADR-024-patch-generation-manifest-boundary.md:279-315`). PRD 1's
example omits `refs`, so v1 keeps it optional but strictly shaped
(`docs/prds/PRD-reconcile-verdict-evidence.md:108-130`).

**Alternatives considered**. Required empty `refs` was rejected as line bloat and
PRD drift. No `refs` boundary was rejected because it would postpone the
cross-artifact contract. Arbitrary map keys were rejected because strict readers
need a closed shape.

### D13 — PRDs 4–9 consume this ADR and need no separate schema ADRs

PRDs 4–9 ship under ADR-025 because their v1 outputs are evidence/revision
records covered above. PRD 6 adds `file-novelty`, PRD 7 `hunk-overlap`, PRD 8
`blocked-classification`, and PRD 9 `path-restructure`; PRDs 4 and 5 consume
confirmation/revision records for retirement audit and study validation. They add
no new artifact, lifecycle state, transcript store, vector index, or structural
search planner.

**Rationale**. WP-003 says PRDs 4–9 write or consume the same records and ship
under this ADR (`docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md:75-78`).
The cluster dashboard and supervisor LOG record the same dependency tree and
single-ADR approach (`docs/CLUSTERS.md:89-113`, `docs/supervisor/LOG.md:635-645`).

**Alternatives considered**. One ADR per PRD was rejected by the broker's locked
cluster decision. Ad hoc enum extension by later PRDs was rejected because D4
already reserves their values. Deferring PRDs 4–9 to structural/search ADRs was
rejected because their v1 surface is narrow classification/audit over these logs.

## Consequences

Wave α and later WP-003 implementers can rely on:

- two append-only per-feature JSONL artifacts under
  `.tpatch/features/<slug>/artifacts/`;
- per-line `schema_version: 1`, stable keys, strict-on-unknown readers, and
  closed v1 enum sets;
- `re_<12hex>` evidence IDs and `rr_<12hex>` revision IDs;
- patch-id evidence using ADR-024's `git_patch_id` and
  `git-patch-id-stable` algorithm marker;
- `review_verdict` as the only new `ReconcileSummary` field, with
  `status.json:state` remaining lifecycle truth;
- auto-confirm for reachable patch-id matches without a new v1 override flag;
- a hard privacy boundary forbidding source bodies, prompts/transcripts, vectors,
  and embeddings;
- ADR-024-style malformed-file handling via `errors.Is`-compatible sentinels;
- optional strict `refs` links compatible with `patch-generations.json`.

Ruled out for v1 without a follow-up ADR: `.json` evidence files; permissive
unknown-field reads; raw source snippets; transcript/prompt/vector persistence;
new lifecycle states; a parallel `final_feature_state` field; non-stable Git
patch-id algorithms; silent repair of malformed JSONL; ad hoc evidence kinds
beyond D4.

## Open questions deferred

- Structural patch fingerprints.
- Reconcile commutation graph.
- Structural middle-pass boundary.
- Reconcile search planner / planner audit artifacts.
- Patch vector index.
- LLM/transcript persistence.
- `--reason` text and richer agent context persistence (future
  `ADR-capture-context-privacy-boundary`).
- v2 migration/read-compatibility policy.
- Repo-wide aggregate revision summaries.

## References

- [WP-003 Reconcile Safety and Middle-pass Foundation](../whitepapers/WP-003-reconcile-safety-and-middle-pass.md) §§3–6
- [docs/CLUSTERS.md](../CLUSTERS.md) WP-002 shipped block and WP-003 accepted block
- `docs/supervisor/LOG.md:590-699` — 2026-05-16 WP-003 cluster approval
- [PRD-reconcile-verdict-evidence.md](../prds/PRD-reconcile-verdict-evidence.md) §§3, 3.1, 5, 6
- [PRD-upstreamed-confirmation-gate.md](../prds/PRD-upstreamed-confirmation-gate.md) §§3–6
- [PRD-reconcile-revision-pass-log.md](../prds/PRD-reconcile-revision-pass-log.md) §§3–6
- [PRD-patch-already-upstream-detector.md](../prds/PRD-patch-already-upstream-detector.md) §§3.3, 3.4, 5.1, 5.4, 6
- [ADR-024-patch-generation-manifest-boundary.md](./ADR-024-patch-generation-manifest-boundary.md) D2, D6, D7, D9
- [ADR-022-detector-default-deferral.md](./ADR-022-detector-default-deferral.md)
- `SPEC.md:133-153` — four-phase reconcile decision tree
- `docs/reconcile.md:7-14` — user-facing reconcile phase overview
- `internal/store/types.go:5-19` — lifecycle states
- `internal/store/types.go:31-62` — reconcile outcomes and labels
- `internal/store/types.go:249-279` — current `ReconcileSummary`
- `internal/store/types.go:363-374` — `PatchIDMatch`
- `internal/store/patch_generations.go:24-28`, `:90-107` — malformed sentinel precedent
