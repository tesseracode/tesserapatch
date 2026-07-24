# PRD - Tpatch Doctor / Metadata Migration - `tpatch-doctor`

**Status**: Proposed
**Date**: 2026-07-23
**Owner**: Core
**Cluster**: v0.11.1 stabilization (Slice 4)
**Depends on**: ADR-024, ADR-025, ADR-027; Slice 3 `RELEASING.md` anti-drift guardrails.
**Blocks**: none. Doctor is a leaf feature and does not block v0.11.1 stabilization.

## Related

- [ADR-024 — Patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md)
- [ADR-025 — Reconcile evidence and revision schema](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md)
- [ADR-027 — Capture Context Privacy Boundary](../adrs/ADR-027-capture-context-privacy-boundary.md)
- [RELEASING.md](../../RELEASING.md), especially the Slice 3 CI-check candidate for tag / CHANGELOG / GitHub Release drift.
- [PRD-tpatch-land](./PRD-tpatch-land.md) and [PRD-tpatch-hotfix](./PRD-tpatch-hotfix.md) as PRD-shape precedents.
- [PRD-reconcile-verdict-evidence](./PRD-reconcile-verdict-evidence.md) as the shorter evidence-artifact precedent.
- Slice 1 `TestSkillRecipeSchemaMatchesCLI` parity-guard pattern in `assets/assets_test.go`.

## 0. Meta

### 0.1 Paper-only status

This PRD is **Proposed**. It changes no code, schema, CLI behavior, shipped asset
text, CHANGELOG entry, release artifact, or migration state. Acceptance requires
the same supervisor + external review pair used for the v0.11.1 planning docs.

### 0.2 Claims audit

| Claim | Evidence |
|---|---|
| `status.json` is the authoritative per-feature machine state and current code has no `feature.yaml` file in the feature layout. | `internal/store/types.go:143-190`; `docs/feature-layout.md:16, 86`; `docs/prds/PRD-tpatch-hotfix.md:16, 28`. |
| Patch generations are a separate per-feature artifact with `version: 1`, strict unknown-field/version validation, and `patch-generations.json` path. | `internal/store/patch_generations.go:16-28, 30-55, 90-114`; ADR-024 D1-D9. |
| Reconcile evidence and revision artifacts use per-line `schema_version: 1`, strict malformed handling, and `re_<12hex>` / `rr_<12hex>` IDs. | `internal/store/reconcile_evidence.go:15-18, 89-125`; `internal/store/reconcile_revision.go:15-20, 51-80`; ADR-025 D1-D13. |
| The shipped skill surfaces are six files: Claude, Copilot SKILL, Copilot Prompt, Cursor, Windsurf, and Generic workflow. | `assets/assets_test.go:122-133`. |
| `TestSkillRecipeSchemaMatchesCLI` already decodes skill-side `apply-recipe.json` examples into `workflow.ApplyRecipe` with `DisallowUnknownFields`. | `assets/assets_test.go:253-312`. |
| Slice 3 queued a guard that verifies tags have matching CHANGELOG entries and GitHub Releases within 24 hours. | `RELEASING.md:139-157`; `docs/handoff/CURRENT.md:42-44`. |
| ADR-027 forbids doctor-like readers from dereferencing raw transcripts, IDE buffers, prompt text, or local private buffers by default. | ADR-027 D2-D4, especially lines 135-145 and 188-200. |

### 0.3 Terminology

- **Drift** means persisted tpatch workspace state disagrees with the running
  binary's declared schemas, bundled assets, or release-process invariants.
- **Doctor check** means one independent diagnostic unit with a stable ID.
- **Finding** means one check result for one file, feature, tag, or asset target.
- **Fix** means an explicit `--fix` mutation performed by `tpatch doctor` after a
  default dry-run preview.
- **Migration** is a fix that rewrites tpatch-owned metadata to a newer schema.
  v1 has only narrow migrations; source-code transformations are out of scope.

## 1. Problem statement

v0.11.1 surfaced multiple drift classes that tests and release discipline can
catch in this repository, but users can still encounter at runtime in their own
`tpatch` workspaces:

1. Slice 1 extended `TestSkillRecipeSchemaMatchesCLI` after stale skill examples
   drifted from the production `ApplyRecipe` schema. That protects this repo's
   bundled assets at test time, not in-tree copies already installed into a user
   repository.
2. ADR-024 introduced `patch-generations.json` as a required identity artifact
   for modern record / amend flows, but older features may predate it.
3. ADR-025 introduced `reconcile-evidence.jsonl` and
   `reconcile-revisions.jsonl`; older applied or reconciled features can lack the
   evidence artifact that current reconcile would have written.
4. Slice 3 repaired release metadata after tags v0.8.0 through v0.11.0 existed
   without corresponding GitHub Releases. `RELEASING.md` now names a pre-tag or
   CI check candidate, but no user-facing diagnostic exists.
5. `upstream.lock` is load-bearing for reconcile safety. Older or hand-edited
   lock formats should be reported before reconcile relies on them.
6. `status.json`, legacy `feature.yaml` copies from pre-unification experiments,
   and recipe examples can silently age past the running binary unless a command
   enumerates and explains the mismatch.

`tpatch doctor` is the safety valve: one read-mostly command that reports drift,
offers only safe explicit fixes, and emits a machine-readable report suitable for
CI, release checks, and pre-reconcile hygiene.

## 2. Goals / Non-goals

### 2.1 Goals

1. Detect schema-version drift across tpatch feature metadata and artifact files.
2. Detect features that should have `patch-generations.json` but do not.
3. Detect stale in-tree copies of bundled skill assets across all six shipped
   formats.
4. Detect stale or malformed `upstream.lock` / related lock metadata.
5. Detect missing reconcile evidence for features whose current state implies
   modern reconcile should have produced it.
6. Detect CHANGELOG / tag / GitHub Release drift using the Slice 3 release
   guardrail.
7. Detect recipe-schema drift in installed skill examples and feature recipes.
8. Provide a default dry-run report, explicit `--fix`, backups for every
   mutation, idempotent reruns, and structured `--json` output.
9. Continue past per-check failures so users receive a complete drift inventory
   whenever possible.

### 2.2 Non-goals

1. **No network calls by default.** Doctor must not contact GitHub, providers,
   package registries, telemetry systems, or arbitrary URLs during ordinary
   local checks.
2. **No auth management.** Doctor must not read, create, refresh, or prompt for
   GitHub / provider credentials. GitHub Release publication and auth stay with
   the `gh` CLI and release process.
3. **No GH-Release publishing.** Doctor may report that a tag lacks a release; it
   must not create or edit GitHub Releases.
4. **No source-file transformations.** Doctor may rewrite tpatch-owned metadata
   and installed skill assets only when `--fix` is explicit. It must not edit
   application source files, dependency manifests, tests, or generated code.
5. **No cross-repo migration.** Doctor examines one workspace rooted at `--path`
   or the current directory. It must not scan sibling repos or bulk-migrate a
   fleet.
6. **No raw context inspection.** Doctor must not read provider transcripts,
   prompt logs, IDE buffers, `.git/tpatch/capture/` local private buffers, shell
   history, embeddings, or vector stores. ADR-027's privacy boundary applies.
7. **No historical backfill of untrusted patch generations.** ADR-024 D4 remains
   binding: doctor must not synthesize full `patch-generations.json` history from
   `patches/NNN-*.patch`.
8. **No `tpatch migrate` public alias in v1.** This PRD reserves the concept but
   keeps one user-facing command, `tpatch doctor`, to avoid a duplicate surface.

## 3. Detection checks

Each check has a stable ID. IDs are user-visible in human output, JSON output,
`--check`, and tests.

### D1 — Feature metadata schema drift

**Detects.** Per-feature machine metadata written by an older or unknown schema:
current `status.json`, any legacy `feature.yaml` found under a feature directory,
and known schema-bearing subrecords. Because current `status.json` has no top-level
schema version, v1 treats the absence of a declared version as the current legacy
status contract and reports only concrete field-level differences: unknown fields,
missing required fields, enum values not accepted by `ValidFeatureState`, and
subrecords whose current reader would reject them.

**Read-only or mutating.** Read-only in v1.

**Safe auto-fix.** None in v1. Schema migration requires a future PRD that names
exact field mappings. Doctor may print `run tpatch record --refresh` or a future
migration command when a dedicated mapper exists.

**Failure mode.** Unreadable or malformed feature metadata yields a D1 finding for
that file and does not abort other feature checks unless `.tpatch/` itself cannot
be located.

### D2 — Missing or stale `patch-generations.json`

**Detects.** Features with `artifacts/post-apply.patch` or `status.apply.has_patch`
true but no `artifacts/patch-generations.json`; manifests with unsupported
`version`, unknown fields, feature-slug mismatch, invalid generation kind, missing
`git_patch_id_algorithm: "git-patch-id-stable"`, or invalid cross-links.

**Read-only or mutating.** Read-only in v1.

**Safe auto-fix.** None in v1. The recommended path is `tpatch record --refresh`
(or the then-current record/amend command) because only the record write path can
capture current patch, recipe, base, upper, capture, and dependency snapshots
without violating ADR-024 D4.

**Failure mode.** Malformed manifests mirror ADR-024 D7: report the malformed
file, distrust identity fields for downstream checks, and continue.

### D3 — Stale in-tree skill assets

**Detects.** Installed repository copies of any bundled skill surface that no
longer match the running binary's embedded asset bytes or normalized recipe-schema
examples. Candidate paths include the paths written by `tpatch init` and common
agent locations such as `.copilot/instructions.md`, `.github/copilot-instructions.md`,
`.cursor/rules/`, `.windsurf/`, and copied `SKILL.md` directories when the asset
contains a tpatch marker.

**Read-only or mutating.** Detection is read-only. `--fix` may mutate only files
that doctor can positively identify as installed tpatch asset copies.

**Safe auto-fix.** Replace the stale installed asset with the bundled asset,
creating a sibling backup first, e.g. `<file>.orig` or `<file>.orig.<n>` if the
first backup exists. If the file contains non-tpatch user edits outside the known
asset body, report `manual-merge-required` and do not overwrite.

**Failure mode.** Unreadable candidate paths are D3 findings. A partial fix of one
asset must not prevent reporting other stale assets.

### D4 — Old or malformed lock formats

**Detects.** `.tpatch/upstream.lock` and any future tpatch lock files whose key
set, required fields, or resolved refs do not match the running binary's expected
format. D4 distinguishes: `missing`, `empty`, `malformed`, `old-format`,
`stale-ref`, `unreachable-commit`, and `override-divergence`.

**Read-only or mutating.** Detection is read-only. `--fix` may perform only
format-preserving rewrites when the old lock unambiguously maps to the current
remote / branch / commit fields.

**Safe auto-fix.** Normalize key order, line endings, and equivalent current-field
names after making a backup. Doctor must not fetch, change remotes, advance a lock
commit, or guess a branch.

**Failure mode.** A missing lock is a warning for commands that can operate
without upstream. A malformed lock is a drift finding, not a whole-run abort.

### D5 — Missing reconcile evidence artifacts

**Detects.** Features in `applied`, `active`, `reconciling`, `reconciling-shadow`,
`blocked`, or `upstream_merged` state whose `status.reconcile` indicates a modern
reconcile attempt or outcome but whose `artifacts/reconcile-evidence.jsonl` is
absent or malformed. Doctor also validates `schema_version`, required fields,
closed enums, duplicate `attempt_id`, and optional `refs` shape per ADR-025.

**Read-only or mutating.** Read-only in v1.

**Safe auto-fix.** None in v1. Doctor must not fabricate evidence after the fact.
The recommended path is to rerun reconcile or use the current review command to
append a real revision entry when appropriate.

**Failure mode.** Mirrors ADR-025 D11: report line number and filename when
possible, preserve valid entries for summary counts, and continue loading
`status.json` as current truth.

### D6 — CHANGELOG / tag / GitHub Release drift

**Detects.** Release metadata drift from Slice 3's `RELEASING.md` guardrail:
local git tags lacking a matching `## vX.Y.Z` CHANGELOG entry; CHANGELOG release
entries lacking a local tag; and tags lacking a published GitHub Release in a
caller-provided release snapshot. The snapshot is a local file produced outside
doctor, for example by CI running `gh release list --json tagName,publishedAt`;
doctor itself only reads that file. If no snapshot is provided, the GH Release
subcheck reports `unknown` for every release tag rather than silently passing it.

**Read-only or mutating.** Always read-only.

**Safe auto-fix.** None. CHANGELOG authoring, tag creation, release-snapshot
collection, and GH Release publishing stay with release operators and `gh`.

**Failure mode.** Local tag / CHANGELOG checks run offline. GH Release checks use
only the explicit local snapshot; missing, malformed, or stale snapshots produce
D6 findings and never prompt for auth or contact GitHub.

### D7 — Recipe schema drift

**Detects.** `apply-recipe.json` examples inside installed skill assets and
feature `artifacts/apply-recipe.json` files that no longer decode into the running
binary's `workflow.ApplyRecipe` schema. D7 catches deprecated top-level
`version`, `op` instead of `type`, `contents` instead of `content`, unsupported
operation types, missing top-level `feature`, and unknown fields rejected by the
current decoder.

**Read-only or mutating.** Skill-example drift may be fixed through D3's asset
replacement. Feature recipe drift is read-only in v1.

**Safe auto-fix.** None for feature recipes. Regenerating a recipe is a semantic
operation owned by `tpatch record --regenerate-recipe` or future amend commands,
not doctor.

**Failure mode.** A malformed recipe is reported with feature slug, path, and JSON
parse/schema error. Other checks continue.

### D8 — Doctor hard-invariant and malformed-artifact handling

**Detects.** Workspace-level conditions that make the run unreliable: no `.tpatch/`
workspace, unsafe path traversal, inability to list feature directories, or a
backup target that would overwrite an existing backup.

**Read-only or mutating.** Mostly read-only; backup creation happens only under
`--fix` for D3/D4.

**Safe auto-fix.** None for hard invariants. Doctor should tell the user which
precondition failed.

**Failure mode.** Only hard invariants abort the whole run. Ordinary unreadable
files, malformed JSON/JSONL, unsupported versions, and failed individual fixes are
per-finding errors accumulated into the final report.

## 4. User-facing contract

### 4.1 Command shape

```text
tpatch doctor [--dry-run] [--fix] [--json] [--check <id>] [--path <dir>] [--release-metadata <file>]
```

- Default behavior is equivalent to `--dry-run`.
- `--fix` is the only mode that mutates files.
- `--dry-run --fix` is invalid; users choose preview or mutation.
- `--check <id>` may be repeated. Unknown IDs exit with a usage error before any
  checks run.
- `--json` emits the structured report and suppresses decorative human prose.
- `--release-metadata <file>` is optional, local-only input for D6 GH Release
  verification; doctor never fetches it.

### 4.2 Human output example

```text
$ tpatch doctor
DRIFT  D2 patch-generations-missing  feature=session-search
       .tpatch/features/session-search/artifacts/patch-generations.json missing
       remediation: run tpatch record --refresh session-search

DRIFT  D3 stale-skill-asset  path=.copilot/instructions.md
       bundled sha256=0e7a... installed sha256=9b1d...
       fix: tpatch doctor --fix --check D3 (backup: .copilot/instructions.md.orig)

WARN   D6 release-missing-gh-release  tag=v0.12.0
       CHANGELOG entry and tag exist; GitHub Release status unknown/offline

summary: 3 drift findings, 1 warning, 0 fixed, 0 errors
exit: 1
```

### 4.3 JSON output example

```json
{
  "schema_version": 1,
  "command": "doctor",
  "dry_run": true,
  "fix": false,
  "summary": {
    "checks_run": 7,
    "findings": 3,
    "warnings": 1,
    "fixed": 0,
    "errors": 0
  },
  "findings": [
    {
      "check_id": "D2",
      "code": "patch-generations-missing",
      "severity": "drift",
      "feature": "session-search",
      "path": ".tpatch/features/session-search/artifacts/patch-generations.json",
      "message": "patch-generations.json missing for feature with captured patch",
      "fixable": false,
      "remediation": "run tpatch record --refresh session-search"
    },
    {
      "check_id": "D3",
      "code": "stale-skill-asset",
      "severity": "drift",
      "path": ".copilot/instructions.md",
      "expected_sha256": "0e7a...",
      "actual_sha256": "9b1d...",
      "fixable": true,
      "backup_path": ".copilot/instructions.md.orig"
    }
  ]
}
```

### 4.4 Exit codes

- `0` — all requested checks clean; no drift findings and no errors.
- `1` — drift detected in dry-run / read-only mode.
- `2` — `--fix` attempted at least one mutation and one or more fixes failed or
  only partially completed.
- Other codes are reserved for existing CLI usage/configuration errors, panics,
  interrupted execution, or future expansion.

## 5. Implementation notes

1. CLI registration should live beside other Cobra commands in `internal/cli/`;
   check orchestration should live in a small `internal/workflow/doctor.go` or
   equivalent package so tests can call it without terminal formatting.
2. Reuse store loaders and validators where possible: `LoadFeatureStatus`,
   `LoadPatchGenerations`, `LoadReconcileEvidence`, `LoadReconcileRevisions`,
   and current recipe decoding. Do not duplicate schema rules by hand when a
   production parser exists.
3. For D3, compare embedded asset bytes from `assets.Skills` against installed
   paths. The six source asset paths are the `skillFiles` table in
   `assets/assets_test.go`.
4. For D6, split local and remote-sensitive work. Local tag/CHANGELOG matching is
   always available. GH Release status is verified only from `--release-metadata`
   local input; without it, the status is an explicit `unknown` finding.
5. Backup semantics are mandatory before any overwrite. Backups must be
   repo-relative, must not escape the workspace, and must not overwrite an
   existing backup. If `<file>.orig` exists, use a deterministic next suffix or
   refuse with a clear finding; implementation chooses one and tests it.
6. `--fix` must be idempotent. After doctor replaces a stale asset or normalizes a
   lock, a second `tpatch doctor --fix` on the same workspace reports clean and
   writes no new backup.
7. D11-style malformed handling from ADR-025 applies to doctor output: include
   check ID, path, line number for JSONL when available, and the parser error;
   continue through the rest of the workspace.
8. ADR-027 privacy applies to all checks. Doctor reads committed tpatch metadata,
   installed tpatch skill assets, git tags, and CHANGELOG headings only. It does
   not dereference transcript refs, prompt refs, IDE buffers, private capture
   stores, or environment-secret values.
9. JSON output should be deterministic: checks sorted by ID, findings sorted by
   check ID then feature/path/tag, stable key order, and no wall-clock fields.
10. `tpatch migrate` remains reserved. If a future PRD adds it, it should either
    delegate to doctor check IDs or explicitly supersede this command contract.

## 6. Acceptance criteria

1. **§6.1** `tpatch doctor` defaults to dry-run and performs no writes without
   `--fix`.
2. **§6.2** `tpatch doctor --fix` creates a backup before every file overwrite.
3. **§6.3** `tpatch doctor --fix` run twice on a clean workspace is a no-op on the
   second run and creates no additional backups.
4. **§6.4** D1 reports malformed or unsupported per-feature metadata with check
   ID, feature slug, path, and field/schema error.
5. **§6.5** D1 does not write status migrations in v1.
6. **§6.6** D2 reports a missing `patch-generations.json` for a feature with a
   captured patch and recommends the record/refresh path.
7. **§6.7** D2 reports unsupported `patch-generations.json` versions and unknown
   fields using the production manifest validator.
8. **§6.8** D3 detects stale installed tpatch skill assets across all six shipped
   formats when their bytes differ from bundled assets.
9. **§6.9** D3 `--fix` replaces only positively identified tpatch asset copies and
   refuses candidate files with unrecognized user content.
10. **§6.10** D4 reports malformed, old-format, stale-ref, and unreachable-commit
    lock conditions without fetching from remotes.
11. **§6.11** D4 `--fix` performs only equivalent lock-format normalization and
    never advances the locked commit or guesses a branch.
12. **§6.12** D5 reports missing `reconcile-evidence.jsonl` for applied or
    reconciled features whose status indicates a modern reconcile attempt.
13. **§6.13** D5 reports malformed JSONL with filename and line number while
    continuing to inspect other entries and features.
14. **§6.14** D6 reports local git tags without matching CHANGELOG entries.
15. **§6.15** D6 reports CHANGELOG release headings without matching local git
    tags.
16. **§6.16** D6 verifies GitHub Release presence from `--release-metadata`
    local input and reports tags absent from that snapshot.
17. **§6.17** D6 reports GH Release status as `unknown` when no release snapshot
    is provided, without trying to publish a release, contact GitHub, or prompt
    for auth.
18. **§6.18** D7 rejects stale recipe examples that no longer decode into
    `workflow.ApplyRecipe` with unknown fields disallowed.
19. **§6.19** D7 reports feature recipe schema drift but does not rewrite feature
    recipes in v1.
20. **§6.20** Ordinary per-check errors do not abort the whole run; they appear in
    the final report and other checks still run.
21. **§6.21** Hard invariants such as missing workspace root or unsafe path abort
    before mutation and return a non-zero usage/configuration error.
22. **§6.22** Human output includes a summary count of drift findings, warnings,
    fixed items, and errors.
23. **§6.23** `--json` emits a deterministic schema-versioned report with check
    IDs, stable finding codes, severity, path/feature/tag identifiers, `fixable`,
    remediation, and backup path when relevant.
24. **§6.24** Exit code `0` means clean, `1` means drift detected in dry-run, and
    `2` means `--fix` had a partial failure.
25. **§6.25** `--check <id>` limits execution to the requested check IDs and
    unknown IDs fail before any checks run.
26. **§6.26** Doctor does not read raw transcripts, prompt text, IDE buffers,
    environment-secret values, or local private capture buffers.
27. **§6.27** No doctor check transforms application source files or dependency
    manifests.
28. **§6.28** JSON output is sorted deterministically and contains no wall-clock
    timestamps.
29. **§6.29** Tests cover at least one fixture for each D1-D7 drift class and one
    idempotent `--fix` fixture for every v1 fixable class.

## 7. Open questions

1. **§7.1** Should D6 ever shell out to `gh release view` when `gh` is installed
   and authenticated, or should GH Release verification stay entirely in CI / a
   release script? This PRD requires no network by default either way.
2. **§7.2** Should a later `tpatch migrate` command expose broader schema mappers
   once status-level schema versions are introduced?
3. **§7.3** What exact path markers should `tpatch init` write so D3 can
   distinguish untouched installed assets from user-edited instructions with
   embedded tpatch snippets?
4. **§7.4** Should doctor reports become attachable artifacts for release PRs, or
   remain ephemeral CLI/CI output?

## 8. Out of scope

1. Network calls by default.
2. Authentication prompts, token reads, or auth refresh.
3. GitHub Release publishing or editing; use `gh` and `RELEASING.md`.
4. Source-file transformations.
5. Cross-repo or fleet migration.
6. Historical backfill of patch-generation manifests from old numbered patches.
7. Fabricating reconcile evidence after the reconcile attempt has passed.
8. Reading provider transcripts, prompts, IDE buffers, local private capture
   stores, embeddings, vectors, or secret values.
9. Adding a public `tpatch migrate` command in v1.

## 9. Sources

- ADR-024 D1-D9 for patch-generation separate-file, strict-schema,
  no-backfill, malformed-handling, and schema-version precedents.
- ADR-025 D1-D13 for evidence/revision JSONL schema, D10 privacy, D11 malformed
  handling, and D12 cross-artifact refs.
- ADR-027 D1-D5 for committed-summary vs local-private-buffer privacy lanes and
  least-privilege reads.
- `RELEASING.md:139-157` for tag / CHANGELOG / GH Release anti-drift candidate.
- `assets/assets_test.go:122-133` for the six shipped skill surfaces.
- `assets/assets_test.go:253-312` for the Slice 1 recipe-schema parity guard.
- `internal/store/patch_generations.go:16-114` for current manifest version and
  strict validation.
- `internal/store/reconcile_evidence.go:15-125` and
  `internal/store/reconcile_revision.go:15-80` for current reconcile artifact
  schema versions and IDs.
- `internal/store/types.go:143-190` for current `status.json` status shape.
