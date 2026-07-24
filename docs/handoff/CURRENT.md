# Current Handoff

## Active Task

- **Task ID**: `doctor-wave-alpha-scaffold-d1-d2-d8`
- **Milestone**: `tpatch doctor` implementation — Wave α (foundation: scaffold + D1 metadata + D2 patch-generations + D8 hard invariants). First wave of a 4-wave implementation cluster for `PRD-tpatch-doctor`.
- **Description**: Ship the `tpatch doctor` command scaffold with the safety-critical semantics locked in from the start (`--dry-run` default, `--fix` opt-in, mandatory backups, idempotence, per-check failure isolation, deterministic `--json` output, exit codes 0/1/2). Implement three of the eight detection clauses: D1 (feature metadata drift), D2 (missing/stale `patch-generations.json`), D8 (hard-invariant + malformed-artifact handling). Waves β/γ/δ ship the remaining D3-D7 checks.
- **Status**: Awaiting implementer dispatch.
- **Assigned**: 2026-07-23.

## Doctor implementation cluster wave plan

**Wave α** ← this handoff (foundation: scaffold + D1 + D2 + D8).
- Rationale: D8 defines the hard-invariant + malformed-artifact + safety-defaults surface; must ship WITH the scaffold or nothing else can be built safely. D1 + D2 are the smallest read-only checks and validate the JSON schema + per-check failure isolation + exit-code contract. §6.1-6.7 + §6.20-§6.29 acceptance criteria (safety + D8 hard invariants + D1 + D2).

**Wave β** — D3 (skill assets) + D7 (recipe schema).
- Asset-drift class. Both compare in-tree files against embedded `assets.Skills` bytes. §6.8, §6.9 (D3) + §6.18, §6.19 (D7).

**Wave γ** — D4 (locks) + D5 (evidence).
- Persisted-artifact class. D4 touches `upstream.lock` + related; D5 touches `reconcile-evidence.jsonl` presence. §6.10, §6.11 (D4) + §6.12, §6.13 (D5).

**Wave δ** — D6 (release drift).
- Needs `--release-metadata <file>` local input plumbing per PRD §4. §6.14-§6.17.

## Wave α binding scope

### D1 — Feature metadata schema drift (`docs/prds/PRD-tpatch-doctor.md §3 D1`)

- Detect malformed or unsupported per-feature metadata (`status.json`, `feature.yaml`) via the production loaders in `internal/store/*.go`. Report check ID, feature slug, path, and field/schema error.
- Read-only in v1 (no `--fix` mutation). Emits `remediation` string in JSON output.

### D2 — Missing or stale `patch-generations.json` (`docs/prds/PRD-tpatch-doctor.md §3 D2`)

- Detect features with `artifacts/post-apply.patch` or `status.apply.has_patch=true` but no `artifacts/patch-generations.json`. Also detect manifests with unsupported `version`, unknown fields, feature-slug mismatch, invalid generation kind, missing `git_patch_id_algorithm: "git-patch-id-stable"`, or invalid cross-links.
- Read-only in v1. Remediation string: `run tpatch feature patch refresh <slug>` (verified at `internal/cli/feature_patch.go:29`).
- Use the production manifest validator (ADR-024 `LoadPatchGenerations`).

### D8 — Doctor hard-invariant + malformed-artifact handling (`docs/prds/PRD-tpatch-doctor.md §3 D8`)

- Enumerate hard invariants that abort BEFORE mutation: missing workspace root, unsafe path, etc.
- Malformed-artifact handling MIRRORS ADR-025 D11 pattern (report with filename + 1-indexed line number; continue other checks).
- Never abort the whole run on ordinary per-check errors — §6.20.
- Exit code 2 for `--fix` partial failure per §6.24.

### Scaffold contract (§6.1, §6.2, §6.3, §6.20-§6.29)

- CLI shape: `tpatch doctor [--dry-run] [--fix] [--json] [--check <id>]`.
- `--dry-run` default (§6.1); `--fix` requires explicit opt-in.
- `--fix` MUST create a backup (`<path>.orig` or similar) before every overwrite (§6.2).
- Idempotence: `--fix` twice on clean workspace is no-op + no new backups on second run (§6.3).
- Per-check errors do not abort whole run (§6.20).
- Hard invariants abort before any mutation with non-zero usage/config error (§6.21).
- Human output: summary count of drift findings + warnings + fixed + errors (§6.22).
- `--json`: deterministic schema-versioned report with check IDs, stable finding codes, severity, identifiers, `fixable`, `remediation`, `backup_path` (§6.23).
- Exit codes: `0` clean, `1` drift in dry-run, `2` `--fix` partial failure (§6.24).
- `--check <id>` limits execution to requested check IDs; unknown IDs fail before any checks run (§6.25).
- Privacy: no reading raw transcripts / prompts / IDE buffers / env secrets / local capture buffers (§6.26, ADR-027 D2+D10 binding).
- No source-file transformations (§6.27).
- Deterministic JSON sort + no wall-clock timestamps (§6.28).
- Test fixtures for each D1-D7 drift class + idempotent `--fix` fixture for every v1 fixable class (§6.29 — Wave α only responsible for D1 + D2 + D8 fixtures; Waves β/γ/δ add the rest).

## Wave α suggested layout

- `internal/workflow/doctor.go` — pure doctor engine: check registry, per-check runner, report builder.
- `internal/workflow/doctor_d1.go` — D1 metadata detection using existing `store.LoadFeatureStatus` etc.
- `internal/workflow/doctor_d2.go` — D2 patch-generations detection using existing `store.LoadPatchGenerations`.
- `internal/workflow/doctor_d8.go` — hard-invariant helpers + malformed-artifact classification.
- `internal/store/doctor_report.go` — persisted report schema (JSON output DTO with schema version).
- `internal/cli/doctor.go` — cobra command wiring: `tpatch doctor [--dry-run] [--fix] [--json] [--check <id>]`. Persistent root flags (`--path`) inherit automatically per rule 11.
- Register subcommand under `root` in `internal/cli/cobra.go`.
- Skill/prompt/workflow assets: add `tpatch doctor` short mention to all 6 formats + parity guard. Reference PRD-tpatch-doctor + doctor command in each.
- Tests:
  - `internal/workflow/doctor_test.go` — check registry + per-check runner + report builder.
  - `internal/workflow/doctor_d1_test.go` — D1 fixtures (§6.4, §6.5).
  - `internal/workflow/doctor_d2_test.go` — D2 fixtures (§6.6, §6.7).
  - `internal/workflow/doctor_d8_test.go` — D8 hard-invariant + malformed-artifact fixtures (§6.20, §6.21).
  - `internal/cli/doctor_test.go` — CLI-level tests: --dry-run default (§6.1), --fix backups (§6.2), idempotence (§6.3), --check filtering (§6.25), exit codes (§6.24), --json determinism (§6.23, §6.28), privacy scan (§6.26), no source transforms (§6.27), summary output (§6.22).

## Wave α hard constraints (binding)

1. **PRD as binding contract** — every fix/behavior claim in the implementation must trace back to a §6.X acceptance criterion. If a design decision isn't covered by the PRD, STOP and either escalate to supervisor for a PRD amendment OR document it in the Wave α closure summary for post-Wave-α PRD extension.
2. **Safety defaults NON-NEGOTIABLE** — `--dry-run` default (§6.1); `--fix` opt-in; backups on every overwrite (§6.2); idempotence (§6.3). Test each explicitly.
3. **No new lifecycle states** (`FeatureState` untouched).
4. **No new persisted schemas outside doctor's own JSON output** — doctor reports go to stdout, not to `.tpatch/`. If any persisted artifact is genuinely needed, draft a small D-clause amendment before writing schema code.
5. **ADR-025 + ADR-027 privacy binding**: D8's malformed-artifact handling mirrors ADR-025 D11. Doctor MUST NOT read raw transcripts / IDE buffers / env secrets / local capture buffers per ADR-027 D2+D10 (PRD §6.26 explicit).
6. **No `--release-metadata` in Wave α** — that's Wave δ (D6).
7. **CHANGELOG.md** — add a `## v0.11.2 (unreleased) — tpatch doctor Wave α` section at the top with Wave α scope bullets. Do NOT touch existing entries.
8. **Assets/skills** — if the new subcommand adds a public CLI surface, update all 6 skill formats + prompt + workflow with a `tpatch doctor` short mention. Parity guard MUST pass.
9. **Side Research md5** in `docs/handoff/CURRENT.md` MUST remain `b385fe622db9926f48861105239f113e`.
10. **Commit trailer mandatory**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`. Use `git -c commit.gpgsign=false commit --no-verify`.
11. **Full gates**: `gofmt -l .` (direct, NEVER piped), `go vet ./...`, `go build ./cmd/tpatch`, `go test ./...`. All green including new tests.
12. **Rule 11 (flag-surface accuracy)**: any doctor help text about "supported flags" MUST account for cobra persistent-flag inheritance (`--path` is inherited from root).
13. **Rule 17 (totality claims)**: avoid "only X is supported" phrasing in doctor help unless truly exhaustive against all layers of the production model.
14. **Rule 16 (anti-drift parity guard)**: if doctor emits any schema shape that could drift from Go structs, add a parity guard test that decodes real fixture bytes into the actual DTO.

## Wave α reviewer-brief additions

Rules 11, 15, 16, 17 all apply. Reviewer briefs (internal + externals) MUST include:
- Grep `internal/cli/cobra.go` + `internal/cli/doctor.go` for `PersistentFlags(` to enumerate the full flag surface.
- Verify D1/D2 remediation strings against actual production commands (rule 15).
- Verify D3/D7 (deferred to Wave β) are NOT accidentally implemented in Wave α.
- Verify safety defaults via explicit test coverage (idempotence, backup creation, --fix opt-in, per-check failure isolation).
- Verify JSON output is deterministic (no wall-clock; sorted fields).

## Process for implementer

1. Read `docs/prds/PRD-tpatch-doctor.md` in FULL (all 8 D-clauses + all 29 §6 criteria + §5 implementation notes + §7 open questions + §8 out-of-scope).
2. Read this handoff Wave α binding scope + hard constraints verbatim.
3. Read `docs/supervisor/LOG.md` top 3-5 entries for cluster context.
4. Read the 17 carry-forward dispatch rules in the archived Slice snapshots (see HISTORY.md).
5. Read production ground truth:
   - `internal/store/` — feature loaders + `LoadPatchGenerations` + `LoadFeatureStatus`.
   - `internal/cli/cobra.go:55` — root persistent `--path` flag (rule 11).
   - `internal/cli/feature_patch.go:29` — `tpatch feature patch refresh` (D2 remediation ground truth).
   - `assets/assets_test.go` — parity guard pattern (rule 16 template).
6. Implement D8 hard invariants + malformed-handling FIRST (before D1/D2 — everything depends on it).
7. Implement scaffold + CLI wiring.
8. Implement D1 + tests.
9. Implement D2 + tests.
10. Add asset/skill mentions + parity-guard update.
11. Run gates after each logical commit.
12. Update `docs/handoff/CURRENT.md`:
    - Flip Status to Review.
    - Add "Wave α closure summary" subsection per prior slice pattern with per-§6-criterion fix + test sites.
    - Preserve Side Research md5.
13. Add CHANGELOG.md `## v0.11.2 (unreleased) — tpatch doctor Wave α` entry.
14. Push to `origin/main`. Return commit hashes + gate output + closure summary.

If any §6 criterion is genuinely impossible to close in Wave α without breaking a hard constraint (e.g., needs D3-D7 infrastructure), STOP and report — do NOT silently defer. Waves β/γ/δ handle those criteria.

Do not dispatch reviewers — supervisor handles that.

## v0.11.1 release summary

- **Tag**: `v0.11.1` on `origin/v0.11.1` at release commit `0b9485f`.
- **GH Release**: https://github.com/tesseracode/tesserapatch/releases/tag/v0.11.1 (marked `Latest`).
- **Scope**: 30 commits since v0.11.0, ~4230 insertions across 21 files.
- **CHANGELOG**: `## v0.11.1 — 2026-07-23 — Stabilization` graduated from `(unreleased)` header. 4 slice subsections + ADR-027 + process-artifacts note.
- **`RELEASING.md` validated**: 3-artifact lock-step (CHANGELOG entry → annotated tag → `gh release create --verify-tag --notes-file --latest`) worked end-to-end. Minor doc improvement: awk end-of-range must reference the PREVIOUS release header explicitly (not a generic `/^## v/`) because em-dash + greedy range matching returns zero lines otherwise. Fix committed in `RELEASING.md` alongside the release.

## Open decision for supervisor

Pick next work block (v0.11.1 shipped; cluster + release both closed):

**Option B — Kick off `tpatch doctor` implementation slice**. Uses the just-shipped PRD-tpatch-doctor draft. Would ship the actual doctor command implementing D1-D8 checks. Pros: closes the loop on Slice 4; delivers a real anti-drift tool for users. Cons: larger scope; needs its own wave slicing (likely D1-D3 first, then D4-D6, then D7-D8).

**Option C — Kick off WP-004 (`auto-feature-dependencies`)**. Existing draft at `docs/whitepapers/WP-004-auto-feature-dependencies.md`. Pros: continues the WP-002 → WP-003 sequence.

**Option D — Kick off WP-005 (`spec-driven-workflows`)**. Existing draft at `docs/whitepapers/WP-005-spec-driven-workflows.md`. Pros: opens spec-workflow surface.

**Option E — Research roadmap continuation**. Return to `docs/state-of-the-art/research-roadmap.md`. Six blocked capture PRDs unlocked by ADR-027 acceptance: PRD-active-feature-session (recommended first — will lock ADR-027 D1 local-buffer path softness / F3), PRD-record-context-summary, PRD-agent-event-log, PRD-ide-capture-hooks, PRD-git-hook-capture-guards, ADR-capture-metadata-branch.

Supervisor default recommendation: **Option B** (doctor implementation) as the natural next step after shipping v0.11.1 — closes the loop between the PRD draft and users.

## Non-blocking follow-ups (deferred from prior clusters)

- **ADR-027 F2** (LOW): PRD-ide-capture-hooks Blocks-header naming coord with research-roadmap.md. Still deferred.
- **ADR-027 F3** (LOW): D1 local-buffer path softness. Still deferred to downstream capture PRD (likely PRD-active-feature-session).

## Carry-forward dispatch rules (17 binding)

All 17 rules from post-v0.11.1 cluster still binding. See prior CURRENT.md snapshots in HISTORY.md for full text.

## Session Summary

v0.11.1 stabilization cluster SHIPPED 2026-07-23 via `RELEASING.md` process. Awaiting next-block decision.

## Next Steps

1. Supervisor: pick Option B, C, D, or E.
2. If Option B (doctor implementation):
   - Read `docs/prds/PRD-tpatch-doctor.md` at `Proposed` status.
   - Ask for PRD sub-slicing (all D1-D8 in one wave, or split by check severity/complexity).
   - Dispatch first implementer.
3. If Option C/D:
   - Read the WP draft.
   - Ask for PRD ordering + wave structure.
   - Dispatch first slice.
4. If Option E:
   - Recommend `PRD-active-feature-session` first (locks ADR-027 D1 F3 follow-up).

## Blockers

None.

## Context for Next Agent

- v0.11.1 is the current `Latest` GH Release; v0.11.0 remains published but demoted.
- `RELEASING.md` fix landed in the release commit (awk end-of-range guidance).
- 17 carry-forward rules live above.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

## v0.11.1 cluster closure summary

**All 4 slices shipped** (2026-07-19..2026-07-23):
- **Slice 1** ✅ Asset/CLI parity fixes (three-way APPROVED). Anti-drift bonus: `TestSkillRecipeSchemaMatchesCLI`.
- **Slice 2** ✅ Reconcile docs refresh (three-way APPROVED rev-1 after user-external caught F1 flag-surface overclaim). New rule 11.
- **Slice 3** ✅ Release ops cleanup (supervisor-direct execution). 5 GH Releases backfilled + `RELEASING.md` added.
- **Slice 4** ✅ `PRD-tpatch-doctor` paper-only draft (three-way APPROVED post-amend). D1-D8 + §6.1-§6.29. Status `Proposed`. New rule 17.

**Two-opinion protocol**: 11 consecutive rev cycles with three-way concurrence at final acceptance. User-external uniquely blocked in 5 of 11.

**Process rules earned**: 17 total binding carry-forward rules (was 15 at cluster start; rules 11 and 17 added). All apply to future implementation waves.

**Deltas since v0.11.0 tag** (need audit before any Slice-1/2 code changes ship in a new tag):
- Slice 1 code + assets touch (`internal/cli/verify.go`, `internal/workflow/verify.go`, 6 skill/prompt/workflow files, `assets/assets_test.go`).
- Slice 2 docs-only (docs/reconcile.md, CHANGELOG.md bullet, docs/handoff/CURRENT.md history).
- Slice 3 ops-only (`RELEASING.md` added, no code).
- Slice 4 PRD-only (docs/prds/PRD-tpatch-doctor.md added, no code).
- No code beyond Slice 1's verify.go text edits + skill assets. `TestSkillRecipeSchemaMatchesCLI` extension is a real code addition.

## Open decision for supervisor

Pick next work block:

**Option A — Ship v0.11.1 release** (Recommended if we want clean release-boundary discipline). Bundles Slices 1+2 code/docs stabilization under `v0.11.1` tag. Uses the fresh `RELEASING.md` process. Slice 3 already published prior releases; Slice 4 is paper-only. Pros: clean release boundary; validates `RELEASING.md` process end-to-end on a real release. Cons: extra release cycle for a stabilization-only version.

**Option B — Kick off `tpatch doctor` implementation slice**. Uses the just-approved PRD-tpatch-doctor draft. Would ship the actual doctor command implementing D1-D8 checks. Pros: closes the loop on Slice 4; delivers a real anti-drift tool for users. Cons: larger scope; will need its own wave slicing.

**Option C — Kick off WP-004 (`auto-feature-dependencies`)**. Existing draft at `docs/whitepapers/WP-004-auto-feature-dependencies.md`. Pros: continues the WP-002 → WP-003 sequence. Cons: bigger cluster; doctor implementation deferred.

**Option D — Kick off WP-005 (`spec-driven-workflows`)**. Existing draft at `docs/whitepapers/WP-005-spec-driven-workflows.md`. Pros: opens spec-workflow surface. Cons: bigger cluster.

**Option E — Research roadmap continuation**. Return to `docs/state-of-the-art/research-roadmap.md`. Six blocked capture PRDs unlocked by ADR-027 acceptance (PRD-active-feature-session, PRD-record-context-summary, PRD-agent-event-log, PRD-ide-capture-hooks, PRD-git-hook-capture-guards, ADR-capture-metadata-branch). Pros: unblocks the capture-context PRD queue. Cons: paper-only progress.

Supervisor default recommendation: **Option A** (ship v0.11.1 release) then **Option B** (doctor implementation) as the natural sequence. Slices 1+2 have accumulated real stabilization value; users benefit from a versioned release, and `RELEASING.md` gets a real-world exercise before the next major cluster.

## Carry-forward dispatch rules (17 binding)

1. (Wave α) Briefs MUST self-audit against binding ADRs.
2. (Wave α) Briefs naming policy ADRs MUST enumerate config-flag opt-out contracts.
3. (Wave α) Internal reviewer checklist MUST include flag-off counter-scenarios.
4. (Wave α) `gofmt -l .` direct — never piped.
5. (Wave α) Drift self-audit during release prep.
6. (Wave α rev-0) Briefs MUST NOT contain escape hatches that override PRD acceptance.
7. (Wave α rev-1) External reviewer briefs MUST sweep ALL PRD §6 acceptance criteria.
8. (Wave β F1) Briefs MUST enumerate per-PRD §6 display-name contracts as binding production + test contracts.
9. (Wave β F2) Reviewer briefs MUST distinguish "behavior implemented" from "behavior tested". Read production code FIRST.
10. (Wave β F6) State-mutation contracts MUST be verified by reloading from disk.
11. (Slice 2 F1) Flag-surface accuracy claims MUST account for cobra persistent-flag inheritance.
12. (Wave β F3) Privacy tests seed secrets into title/slug/path metadata, NOT file content.
13. (Wave β schema-lock) Briefs say "no persisted-schema additions outside what binding ADRs explicitly authorize" — enumerate which fields/clauses.
14. Two-opinion external review protocol (supervisor + user-parallel) MANDATORY. 11/11 rev cycles has caught HIGH BLOCKERs or confirmed fixes.
15. (γ-1 F1) When PRD names a command/event as trigger, verify the command/event actually exists in production code BEFORE wiring implementation.
16. (Slice 1 anti-drift lesson) When a docs-vs-code drift finding is fixed, add or extend a parity guard test that decodes/validates the docs artifact against the code ground-truth when feasible.
17. (Slice 4 / totality generalization) When docs make a totality claim ("only X is supported", "the full list is Y", "no more than Z"), reviewers MUST verify against ALL layers of the production model (root persistent flags, parent-command flags, cobra command groups, embedded asset paths, etc.).

## Non-blocking follow-ups (deferred from prior clusters)

- **ADR-027 F2** (LOW): PRD-ide-capture-hooks Blocks-header naming coord with research-roadmap.md. Still deferred; can bundle with any future small docs slice or leave for downstream capture PRD.
- **ADR-027 F3** (LOW): D1 local-buffer path softness. Still deferred to downstream capture PRD (likely PRD-active-feature-session).

## Session Summary

v0.11.1 stabilization cluster CLOSED 2026-07-23 (all 4 slices, three-way APPROVED). Awaiting next-block decision.

## Next Steps

1. Supervisor: pick Option A, B, C, D, or E.
2. If Option A (v0.11.1 release):
   - Audit `git log v0.11.0..HEAD` for exact release scope.
   - Draft `## v0.11.1 — YYYY-MM-DD — Stabilization` CHANGELOG entry graduating the existing `(unreleased)` bullets.
   - Follow `RELEASING.md` exactly: tag `v0.11.1`, `gh release create v0.11.1 --verify-tag --notes-file --latest`.
   - Archive CURRENT.md, open next kickoff.
3. If Option B (doctor implementation):
   - Read `docs/prds/PRD-tpatch-doctor.md` at Proposed status.
   - Ask (as WP-003 did) for PRD sub-slicing (D1-D8 all in one wave, or D1-D3 first?).
   - Dispatch first implementer.
4. If Option C/D:
   - Read the WP draft.
   - Ask for PRD ordering + wave structure.
   - Dispatch first slice.
5. If Option E:
   - Pick a specific blocked capture PRD to draft (recommendation: `PRD-active-feature-session` since it will lock the ADR-027 D1 path softness).

## Blockers

None.

## Context for Next Agent

- v0.11.1 stabilization cluster took the same 4-slice pattern as WP-003 (α+β+γ-1+γ-2) but scoped to release hygiene rather than a new subsystem. Template works well for small stabilization slots.
- 17 carry-forward rules live above. Every future implementer/reviewer brief must incorporate applicable rules.
- Two-opinion external review protocol continues to be the primary defense against docs-vs-production drift. 11 consecutive concurrence at final; 5 of 11 with user-external uniquely blocking at rev-0.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`. Verify before/after any CURRENT.md edits.

## Side Research — State-of-the-art middle pass (2026-05-10)

Paper-only exploratory pass completed for a non-LLM middle layer between
deterministic reconcile heuristics and full provider/coding-agent workflows.
This does **not** change code, schema, CLI behavior, roadmap status, PRDs, or
ADRs.

### Research packet

Created `docs/state-of-the-art/` with docs modeled after the existing market
research / PRD conventions: header block, related links, refresh triggers,
references, open questions, and disputes.

Files:

- `docs/state-of-the-art/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`
- `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`
- `docs/state-of-the-art/patch-capture-context-research-brief.md`
- `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`
- `docs/state-of-the-art/research-roadmap.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`

### Findings

1. Patch theory is useful as vocabulary for identity, inverse, composition,
   commutation, dependency, and conflict, but tpatch should not claim
   Darcs/Pijul guarantees on top of unified diffs.
2. Patch identity should be treated as a ladder: exact bytes, `git patch-id`,
   token fingerprints, AST/CFG/PDG similarity, behavioral checks, and finally
   provider/human intent judgment.
3. Computer-vision feature matching maps to code relocation: detect salient
   code keypoints, compute local descriptors, match across old/new upstream,
   reject outliers, then attempt relocated apply in a shadow tree.
4. Search-based application should operate only on uncertain patch clusters,
   after deterministic dependency/commutation pre-passes shrink the search
   space.
5. Beam search is the likely first practical non-LLM planner; MCTS and
   evolutionary algorithms remain candidates for larger uncertain clusters.
6. Vector retrieval / RAG fits as a distinct middle layer: dense retrieval can
   rank likely patch/hunk/code-region matches below full provider reasoning,
   while generation over retrieved context still belongs to the provider tier.
7. The experiment guide defines collection formats for feature metadata, hunks,
   keypoints, fingerprints, retrieval results, commutation relations,
   candidate apply attempts, metrics, and ground-truth labels.
8. First-party tpatch metadata should be the happy path for tpatch-aware repos:
   current metadata is good for lifecycle/DAG reasoning, but future patch
   generations, dependency version snapshots, operation IDs/read-write sets,
   structural anchors, relation artifacts, and vector manifests would make
   identity and ordering easier before fuzzy fallback.
9. A new patch-capture research brief preserves this PRD/ADR queue and defines
   the next front: Quilt-style explicit file claims, Git index/hook boundaries,
   IDE hooks, coding-agent event logs, and privacy-safe agent context capture.
10. Entire is verified as a concrete prior-art target. Its model uses Git hooks,
    agent hooks, commit trailers, a separate `entire/checkpoints/v1` metadata
    branch, shadow checkpoints, full transcript/session storage, redaction, and
    optional checkpoint remotes. tpatch should borrow the Git-native linking
    pattern but default toward summaries/references over raw transcripts.
11. `docs/state-of-the-art/research-roadmap.md` is now the durable exploratory
    tracker so research can advance independently if `docs/handoff/CURRENT.md`
    is reassigned to implementation work.
12. Amendment models differ by tool: Quilt/StGit usually refresh the managed
    patch, Git supports both amend and fixup/squash-forward workflows, Aider
    favors small commits plus undo, and Entire preserves context links around
    rewrites. tpatch likely needs canonical-current patch plus append-only
    generations, with explicit amend/fixup/fold/fork semantics.

### PRD drafts promoted from research (2026-05-13)

The first capture/metadata foundation PRDs were drafted as paper-only planning
docs:

- `docs/prds/PRD-feature-file-claims.md`
- `docs/prds/PRD-record-capture-modes.md`
- `docs/prds/PRD-feature-patch-identity-metadata.md`
- `docs/prds/PRD-feature-patch-amend.md`

`docs/state-of-the-art/research-roadmap.md` is updated to point at these drafts.
The remaining gate before implementation is review/acceptance of the queued
capture privacy and amendment-policy ADRs plus PRD review.

### Candidate follow-up names

These are research outputs only, not queued roadmap work. Four items below now
have draft PRDs as noted above.

- `PRD-structural-patch-fingerprints`
- `PRD-feature-patch-identity-metadata`
- `PRD-dependency-version-snapshots`
- `PRD-recipe-operation-identity`
- `PRD-structural-anchor-manifest`
- `PRD-patch-vector-index`
- `PRD-reconcile-commutation-graph`
- `PRD-reconcile-search-planner`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-planner-audit-artifacts`
- `PRD-feature-file-claims`
- `PRD-record-capture-modes`
- `ADR-patch-amendment-policy`
- `PRD-feature-patch-amend`
- `PRD-active-feature-session`
- `PRD-agent-event-log`
- `PRD-ide-capture-hooks`
- `PRD-git-hook-capture-guards`
- `ADR-capture-context-privacy-boundary`
- `ADR-capture-metadata-branch`
- `PRD-record-context-summary`
