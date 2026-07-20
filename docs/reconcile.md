# Reconcile Workflow

`tpatch reconcile` maintains a feature's applied state as the upstream tree changes: it re-evaluates the recorded patch, records the machine verdict, appends evidence to explain that verdict, and writes revision entries when a confirmation or review pass changes how humans should interpret the result (ADR-025 D1-D13; PRD-reconcile-verdict-evidence §1, §3, §6; PRD-reconcile-revision-pass-log §1, §3, §6).

## Pipeline overview

Reconcile has two layers: the classical verdict engine decides a raw outcome, then the v0.11 evidence/revision pipeline records why that outcome is trustworthy, suspicious, or blocked (ADR-025 D4, D13; PRD-reconcile-verdict-evidence §3.1, §6).

```text
raw verdict engine
  reverse-apply / patch-id / operation-level / provider-semantic / forward-apply
        |
        v
evidence + review pipeline
  verdict evidence
  -> file-novelty
  -> hunk-overlap
  -> path-restructure
  -> blocked taxonomy
  -> upstreamed confirmation gate
  -> revision-pass writer
```

| Order | Pass | What it records |
|---:|---|---|
| 1 | Reverse-apply detection | Phase-1 evidence that the recorded patch is already present; phase-1.5 patch-id evidence is the high-confidence upstream match path (ADR-025 D4-D5, D9; PRD-reconcile-verdict-evidence §3.1, §6). |
| 2 | File-novelty classifier | `file-novelty` evidence for whether patch paths are all new, mixed additive, existing-file modifications, delete/rename cases, or unknown (ADR-025 D4, D13; PRD-reconcile-file-novelty-classifier §3, §6). |
| 3 | Hunk-overlap detector | `hunk-overlap` evidence for line-range overlap on modified paths, including `nearby-window=3` (ADR-025 D4, D13; PRD-reconcile-hunk-overlap-detector §3-§4, §6). |
| 4 | Path-restructure detector | `path-restructure` evidence for prefix moves, splits, deletions, mixed cases, none, or unknown (ADR-025 D4, D13; PRD-reconcile-path-restructure-detector §3, §6). |
| 5 | Blocked-verdict taxonomy | A deterministic blocked category and recommended action derived from dependency labels plus file, hunk, and path evidence (ADR-025 D13; PRD-reconcile-blocked-verdict-taxonomy §3-§4, §6). |
| 6 | Confirmation gate | Converts raw `upstreamed` candidates into `confirmed-upstreamed` or `rejected-upstreamed` review verdicts before retirement (ADR-025 D8-D9; PRD-upstreamed-confirmation-gate §3-§6). |
| 7 | Revision-pass writer | Appends review/correction entries that link back to evidence attempts and explain confirmation, rejection, cleanup, or later human action (ADR-025 D6-D7; PRD-reconcile-revision-pass-log §3-§6). |

## Evidence and revision artifacts

### `reconcile-evidence.jsonl`

Evidence attempts live at `.tpatch/features/<slug>/artifacts/reconcile-evidence.jsonl`; the file is append-only JSONL, one newline-terminated object per attempt, with per-line `schema_version: 1` and strict v1 fields/enums (ADR-025 D1-D2, D4). Each evidence entry uses content-addressed `attempt_id` values shaped as `re_<12hex>`; the hash excludes `attempt_id` itself and includes the normalized identity fields and sorted arrays (ADR-025 D3). Evidence entries are audit detail; `status.json` remains current state, and status JSON can expose an `evidence_artifact` reference when valid evidence exists (PRD-reconcile-verdict-evidence §4-§6; ADR-025 D1).

Synthetic evidence entry (not copied from any repository):

```json
{
  "schema_version": 1,
  "feature_slug": "demo-feature",
  "attempt_id": "re_e6a802d2a675",
  "upstream_ref": "upstream/main",
  "upstream_commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "base_commit": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "raw_reconcile_verdict": "blocked",
  "phase": "phase-3.5",
  "evidence_kind": "hunk-overlap",
  "confidence": "medium",
  "matched_paths": ["src/example.go"],
  "matched_operations": [
    "nearby-window=3",
    "src/example.go:hunk_4d2a9c:context-only:nearby=1"
  ],
  "match_origin": "unknown",
  "upstream_commit_refs": [],
  "pre_reconcile_presence": "present",
  "requires_confirmation": false,
  "reason_code": "context-only"
}
```

### `reconcile-revisions.jsonl`

Revision corrections live at `.tpatch/features/<slug>/artifacts/reconcile-revisions.jsonl`; the file is append-only JSONL, one newline-terminated object per correction, with `entry_id`, `feature_slug`, `evidence_attempt_id`, raw verdict, review verdict, final feature state, action, reason code, validation refs, optional supersedure, and optional `refs` (ADR-025 D6, D12; PRD-reconcile-revision-pass-log §3, §6). Revision entries use content-addressed `rr_<12hex>` IDs and closed v1 enums for `review_verdict` and `action_taken` (ADR-025 D6-D7).

Synthetic revision entry (not copied from any repository):

```json
{
  "schema_version": 1,
  "entry_id": "rr_fe613be10f5b",
  "feature_slug": "demo-feature",
  "evidence_attempt_id": "re_e6a802d2a675",
  "raw_reconcile_verdict": "blocked",
  "review_verdict": "false-negative",
  "final_feature_state": "applied",
  "action_taken": "reapplied-and-recorded",
  "reason_code": "false-negative-blocked",
  "validation_refs": [
    {
      "kind": "test-command",
      "value": "go test ./...",
      "result": "pass"
    }
  ]
}
```

### Strict writers, lenient readers, and privacy

Writers refuse to append when pre-existing JSONL is malformed; read-only status/list surfaces keep loading `status.json` current truth and report artifact problems rather than silently repairing or truncating audit files (ADR-025 D11; PRD-reconcile-verdict-evidence §5-§6; PRD-reconcile-revision-pass-log §5-§6). `tpatch reconcile review list <slug> --json` emits a JSON envelope with `revisions` plus `corrupt_entries`; if corrupt lines exist, valid entries remain in the payload and the command exits non-zero (PRD-reconcile-revision-pass-log §5-§6). Neither JSONL artifact may persist source bodies, provider transcripts, prompt text, vectors, or embeddings; v1 is limited to paths, hashes, operation IDs, enum codes, verdicts, refs/commits, counts, validation references, and cross-artifact IDs (ADR-025 D10; PRD-reconcile-verdict-evidence §2, §6; PRD-reconcile-revision-pass-log §2, §6).

## Verdict, label, and category surfaces

- Human reconcile output renders an unconfirmed raw `upstreamed` as `[upstreamed-candidate]`; JSON keeps the programmatic truth as `outcome: "blocked"` with `review_verdict: "rejected-upstreamed"` so automation can reconstruct the gate decision without inventing a new outcome (ADR-025 D8; PRD-upstreamed-confirmation-gate §3-§6).
- Reachable patch-id evidence can auto-confirm an upstreamed verdict; low-confidence operation/provider candidates require confirmation and can be rejected by the gate (ADR-025 D9; PRD-upstreamed-confirmation-gate §5-§6).
- Blocked taxonomy has deterministic precedence: `dependency-blocked > validation-blocked > target-deleted > structural-conflict > edit-overlap > shifted-context > clean-additive > unknown-blocked` (PRD-reconcile-blocked-verdict-taxonomy §3, §6). The category is evidence/runtime metadata and display JSON (`blocked_category`, `recommended_action`), not a new persisted lifecycle state or `ReconcileOutcome` enum (ADR-025 D8, D13; PRD-reconcile-blocked-verdict-taxonomy §4-§6).
- File novelty persists PRD 6 evidence classifications as `reason_code` values `all-new-files`, `mixed-additive`, `modifies-existing-files`, `deletes-or-renames`, or `unknown`; the blocked taxonomy may map all-new or mixed-additive evidence into the user-facing `clean-additive` blocked category (ADR-025 D4, D13; PRD-reconcile-file-novelty-classifier §3-§6; PRD-reconcile-blocked-verdict-taxonomy §3).
- Hunk overlap persists `reason_code` values `none`, `context-only`, `edit-overlap`, `target-deleted`, `path-moved`, or `unknown`; the default nearby window is three lines and is included in evidence as `nearby-window=3` (ADR-025 D4, D13; PRD-reconcile-hunk-overlap-detector §3-§6).
- Path restructure persists `reason_code` values `prefix-move`, `prefix-split`, `target-deleted`, `mixed`, `none`, or `unknown`; default thresholds are config-driven with `prefix-split` at at least 3 moved files to at least 2 prefixes, and `prefix-move` at at least 5 moved files to one prefix (ADR-025 D4, D13; PRD-reconcile-path-restructure-detector §3-§6).

## v0.11 reconcile subcommands

The command strings below are the production CLI surface; only the flags shown here are supported for these v0.11 subcommands (PRD-upstreamed-confirmation-gate §3-§6; PRD-reconcile-revision-pass-log §4-§6; PRD-reconcile-retirement-state-audit §3, §6).

| Command | Flags | Behavior |
|---|---|---|
| `tpatch reconcile audit-retirement <slug>` | `--json` | Read-only audit of retired-feature dependency/status metadata; reports stale `satisfied_by` or base SHAs, affected child features, and cleanup-needed findings without mutating dependency or status metadata (PRD-reconcile-retirement-state-audit §3-§6). |
| `tpatch reconcile confirm-upstreamed <slug>` | `--json`, `--format human\|json` | PRD-named trigger for the confirmation/retirement path: requires the feature's latest reconcile state to be `upstreamed` or `review_verdict=confirmed-upstreamed`, runs the retirement audit, and may append `cleanup-needed` revision entries (ADR-025 D8-D9; PRD-upstreamed-confirmation-gate §3-§6; PRD-reconcile-retirement-state-audit §3, §6). |
| `tpatch reconcile review add <slug>` | Required: `--verdict`, `--action`, `--reason-code`. Optional: `--raw-verdict`, `--final-state`, `--evidence`. | Appends one revision-pass entry; it records review metadata only and does not perform a repair (ADR-025 D6-D7; PRD-reconcile-revision-pass-log §3-§6). |
| `tpatch reconcile review list <slug>` | `--json`, `--all` | Lists latest revision entries by default; `--all` includes superseded entries. JSON output is `{ "revisions": [...], "corrupt_entries": [...] }` and exits non-zero when corrupt entries are present (ADR-025 D6-D7, D11; PRD-reconcile-revision-pass-log §4-§6). |

`review add` accepts review verdicts `confirmed`, `false-positive`, `false-negative`, `inconclusive`, and `deferred`; it accepts actions `none`, `confirmed-retired`, `reapplied`, `reapplied-and-recorded`, `implemented`, `deferred`, `skipped`, and `cleanup-needed` (ADR-025 D7; PRD-reconcile-revision-pass-log §3, §6).

## Dev-only study validator

`internal/tools/studyvalidator/` is a stdlib-only maintainer helper for validating reconcile case-study folders; it is not part of the public `tpatch` binary or `SPEC.md` CLI surface (PRD-reconcile-study-validation §3, §6). The validator checks JSON/JSONL parse errors with filename and line number, cross-file study/metric consistency, raw-vs-reviewed-vs-final-state counts, and per-corrected-verdict linkage to revision entries or `local-notes.md` references (PRD-reconcile-study-validation §4-§6).

## Operational reminders

- Run reconcile from a clean target upstream tree; dirty files, merge markers, and `.orig`/`.rej` leftovers can make patch application verdicts unreliable. Use `tpatch reconcile --preflight` for the read-only clean-tree check, and reserve `--allow-dirty` for deliberate overrides.
- `.tpatch/` should be tracked if feature state needs to travel with a branch; evidence and revision JSONL files live under each feature's `artifacts/` directory (ADR-025 D1, D6; PRD-reconcile-verdict-evidence §3; PRD-reconcile-revision-pass-log §3).

## Cross-references

- [ADR-025 — Reconcile evidence and revision schema](./adrs/ADR-025-reconcile-evidence-and-revision-schema.md)
- [PRD-reconcile-verdict-evidence](./prds/PRD-reconcile-verdict-evidence.md)
- [PRD-upstreamed-confirmation-gate](./prds/PRD-upstreamed-confirmation-gate.md)
- [PRD-reconcile-revision-pass-log](./prds/PRD-reconcile-revision-pass-log.md)
- [PRD-reconcile-retirement-state-audit](./prds/PRD-reconcile-retirement-state-audit.md)
- [PRD-reconcile-study-validation](./prds/PRD-reconcile-study-validation.md)
- [PRD-reconcile-file-novelty-classifier](./prds/PRD-reconcile-file-novelty-classifier.md)
- [PRD-reconcile-hunk-overlap-detector](./prds/PRD-reconcile-hunk-overlap-detector.md)
- [PRD-reconcile-blocked-verdict-taxonomy](./prds/PRD-reconcile-blocked-verdict-taxonomy.md)
- [PRD-reconcile-path-restructure-detector](./prds/PRD-reconcile-path-restructure-detector.md)
- [Feature Layout](./feature-layout.md)
- [SPEC](../SPEC.md)
