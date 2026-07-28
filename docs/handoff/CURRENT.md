# Current Handoff

## Active Task

- **Task ID**: `doctor-wave-beta-d3-d7`
- **Milestone**: `tpatch doctor` implementation — Wave β (D3 skill assets + D7 recipe schema). Second wave of the 4-wave cluster.
- **Description**: Extend the doctor scaffold (shipped in Wave α at `a3b9fe3`) with two asset-drift detection classes: D3 (stale in-tree skill assets — compare installed bytes to embedded `assets.Skills` bytes across the 6 shipped install paths) and D7 (recipe schema drift — decode `apply-recipe.json` files against `workflow.ApplyRecipe` with `DisallowUnknownFields`). Both classes are candidates for `--fix`, so Wave β is the FIRST slice to exercise the mutating half of the scaffold: backup semantics, idempotence, and exit-code 2 for partial-failure-on-fix.
- **Status**: Awaiting implementer dispatch.
- **Assigned**: 2026-07-27.

## Doctor implementation cluster wave plan

- **Wave α** ✅ CLOSED 2026-07-27 (three-way APPROVED WITH NOTES). Scaffold + D1 + D2 + D8. §6.1-§6.7 + §6.20-§6.29 MET. Snapshot in HISTORY.md.
- **Wave β** ← this handoff (D3 skill assets + D7 recipe schema; §6.8-§6.9 + §6.18-§6.19). FIRST wave with fixers.
- **Wave γ** — D4 (locks) + D5 (evidence). §6.10-§6.13.
- **Wave δ** — D6 (release drift). Needs `--release-metadata <file>` plumbing. §6.14-§6.17.

## Wave β binding scope

### D3 — Stale in-tree skill assets (§6.8, §6.9)

- **Detect** (§6.8): stale installed tpatch skill assets across all six shipped formats when installed bytes differ from bundled `assets.Skills` bytes. The six install paths (verified against `internal/cli/cobra.go:2780-2801` at Slice 4 F2 amend):
  - `.claude/skills/tessera-patch/SKILL.md`
  - `.github/skills/tessera-patch/SKILL.md`
  - `.github/prompts/tessera-patch-apply.prompt.md`
  - `.cursor/rules/tessera-patch.mdc`
  - `.windsurfrules` (single file, NOT a directory)
  - `.tpatch/workflows/tessera-patch-generic.md`
- **`--fix` (§6.9)**: replaces only positively-identified tpatch asset copies. REFUSES candidate files with unrecognized user content (i.e., a file at a doctor-managed install path that doesn't look like a tpatch asset must NOT be overwritten — refuse with a specific finding + remediation asking user to move or delete the file manually).
- Positive-identification contract: the file at the install path must decode/parse plausibly as the same asset class the bundled version writes. Suggested identification rules (implementer chooses + documents):
  - Byte-level sha256 comparison against bundled bytes → exact match = clean; mismatch = drift.
  - If mismatch: read the installed file's first ~256 bytes and check for a tpatch marker (e.g., first line contains `tessera-patch` OR `tpatch` OR a bundled asset's opening heading). If no marker: refuse `--fix` with `unrecognized user content` finding.
  - Do NOT parse the installed bytes as tpatch DSL — a bad DSL parse should not cause `--fix` to overwrite; refuse instead.
- Backup semantics: before overwrite, write `<path>.orig`. If `<path>.orig` already exists AND matches installed bytes: skip backup (idempotence). If `<path>.orig` exists AND differs from installed bytes: **refuse** the `--fix` for that file with a specific finding — do NOT clobber the prior backup.

### D7 — Recipe schema drift (§6.18, §6.19)

- **Detect** (§6.18): decode each per-feature `.tpatch/features/<slug>/artifacts/apply-recipe.json` (and any bundled skill-asset apply-recipe examples if in-tree copies exist per D3) against `workflow.ApplyRecipe` with `DisallowUnknownFields`. Rejection = drift finding.
- **Read-only in v1 (§6.19)**: doctor reports recipe schema drift but does NOT rewrite feature recipes. Remediation string: point at Slice 1's `TestSkillRecipeSchemaMatchesCLI` pattern and the canonical schema — implementer must fix by hand OR regenerate via `tpatch implement <slug>` (verify command exists first per rule 15).
- Anti-drift reuse: this check is the runtime analog of `assets/assets_test.go` `TestSkillRecipeSchemaMatchesCLI`. Implementation SHOULD share a decoder helper if practical.

### Non-blocking observations to fold in (from Wave α reviews)

- **Exit-code semantics for `--fix` + `Findings>0` + `Errors==0`**: Wave α internal noted `DoctorExitCode` returns 1 (not 2) in this case. Wave α had no fixers so the path was unreachable. Wave β has D3 fixers — verify §6.24 (exit 2 = `--fix` partial failure) fires correctly when D3 finds drift AND `--fix` refuses due to unrecognized user content OR pre-existing backup collision.
- **`--check` case-insensitivity**: Wave α internal noted `--check` is case-insensitive but untested. Wave β adds new check IDs (D3, D7); pick one convention (case-sensitive per rule 8 display-string contract, OR document case-insensitivity + add coverage) and enforce.

## Wave β hard constraints (binding, 15)

1. **PRD as binding contract** — every fix traces to §6.X or STOP.
2. **Safety defaults NON-NEGOTIABLE** — Wave α's scaffold semantics extend to D3 `--fix`. Verify: `--dry-run` default; `--fix` opt-in; backup on every overwrite (§6.2); idempotence (§6.3).
3. **No new lifecycle states** (`FeatureState` untouched).
4. **No new persisted schemas outside doctor's JSON output**.
5. **Rule 5 (ADR-025 D11 pattern)** — malformed recipe JSON reports filename + 1-indexed line number where practical; continues other checks.
6. **Rule 12 privacy** — D3 must NOT read user files at doctor-managed install paths beyond the byte-comparison + first-256-byte marker check. Specifically: doctor MUST NOT parse installed files as tpatch DSL and MUST NOT hash/log the content of unrecognized user files beyond a truncated hash for the finding evidence. ADR-027 D2+D10 binding.
7. **Rule 15 (trigger-name grep)** — verify `tpatch implement <slug>` (D7 remediation candidate) exists via `internal/cli/cobra.go` grep. If it doesn't, pick a real command or drop the remediation.
8. **Rule 11 (flag-surface accuracy)** — no new persistent flags. `--fix` remains local to `doctor` subcommand.
9. **Rule 17 (totality claims)** — D3 detection MUST NOT teach "only these six paths are managed" without noting the intentional non-scope of hand-copied assets (per PRD §7.3 open question). Match Slice 4 F2 amend framing.
10. **Rule 16 (anti-drift parity guard)** — if D7 introduces a shared decoder helper with `TestSkillRecipeSchemaMatchesCLI`, extend the parity test to also cover per-feature apply-recipe files (or add a doctor-side unit test that decodes fixtures and asserts against `workflow.ApplyRecipe`).
11. **Rule 18 (structural trailer verification, NEW)** — every commit's trailer must pass `git interpret-trailers --parse` structural check, not just text-grep. Implementer should sanity-check trailer parse before pushing.
12. **CHANGELOG.md** — extend the existing `## v0.11.2 (unreleased) — tpatch doctor Wave α` header to cover Wave β. Add Wave β bullets under a `### Wave β` subsection OR promote the header to a broader scope; do NOT create a separate `(unreleased)` entry.
13. **Assets/skills** — Wave α added the `tpatch doctor` scaffold mention to 6 formats. Wave β should not need new asset mentions unless a new user-facing flag lands. Verify parity guard still passes.
14. **Side Research md5** == `b385fe622db9926f48861105239f113e`.
15. **Full gates** + Co-authored-by trailer.

## Wave β suggested layout

- `internal/workflow/doctor_d3.go` — new: D3 detection + optional fix (byte comparison + marker check + refuse-on-unrecognized).
- `internal/workflow/doctor_d7.go` — new: D7 detection (recipe schema decode with DisallowUnknownFields).
- Extend `internal/workflow/doctor.go` — register D3 + D7 in the check registry from Wave α.
- Extend `internal/cli/doctor.go` — no new CLI shape; --fix path exercises D3 for the first time.
- Tests:
  - `internal/workflow/doctor_d3_test.go` — fixtures: clean, drift-then-fix, unrecognized-user-content-refused, pre-existing-backup-collision-refused, idempotence (§6.8, §6.9).
  - `internal/workflow/doctor_d7_test.go` — fixtures: clean recipe, missing feature field, unknown field, disallowed field type (§6.18, §6.19).
  - `internal/cli/doctor_test.go` — extend for D3 `--fix` end-to-end: dry-run reports drift; `--fix` writes backup + replaces; second `--fix` is no-op; exit code 2 when `--fix` refuses.
- No new asset mentions expected. Parity guard should stay clean.

## Reviewer-brief additions (Wave β specific)

- Rule 18 (structural trailer verification) MUST be in every reviewer brief now. Include: "Run `git log --format='%(trailers)' <sha>` on every commit in the review range; empty output for any commit is a MEDIUM finding unless it's a merge or fixup that intentionally omits authorship."
- Rule 15: verify D7 remediation string names a real command.
- Rule 17: verify D3 doesn't teach a totality claim about install paths.
- Backup semantics (§6.2 + §6.3 + collision case): reviewer briefs must verify test coverage for:
  1. Backup created on first `--fix` when target has drift.
  2. NO second backup on idempotent re-run.
  3. `--fix` REFUSES when `.orig` already exists AND differs from installed bytes.
  4. `--fix` REFUSES when installed file lacks a tpatch marker (unrecognized user content).

## Process for implementer

1. Read `docs/prds/PRD-tpatch-doctor.md` in FULL. Focus on §3 D3 + §3 D7 + §4 user-facing contract + §5 implementation notes + §6.8, §6.9, §6.18, §6.19 + §7 open questions.
2. Read this handoff Wave β binding scope + hard constraints verbatim.
3. Read `docs/supervisor/LOG.md` top 5 entries: user-external Wave α APPROVED WITH NOTES + supervisor decision + supervisor-external F-EXT-1 + internal Wave α + Wave α ship commit.
4. Read production ground truth:
   - `internal/cli/cobra.go:2780-2801` — `installSkills` six install paths (D3 ground truth).
   - `assets/assets_test.go` — `TestSkillRecipeSchemaMatchesCLI` (D7 pattern reference).
   - `assets/skills/` — bundled bytes (D3 comparison target).
   - `internal/workflow/implement.go:42` — `ApplyRecipe` struct (D7 decode target).
   - `internal/workflow/doctor.go` (added at `6319c0b`) — Wave α scaffold + check registry.
5. Verify `tpatch implement <slug>` command exists in `internal/cli/cobra.go` before using it in D7 remediation string.
6. Implement D3 first (mutating; exercises scaffold `--fix` semantics for the first time). Then D7 (read-only decode, simpler).
7. Verify exit-code semantics for `--fix` + `Findings>0` + `Errors==0` case now that D3 fixers exist (§6.24). Fix the Wave α observation if it manifests.
8. Verify `--check` case-sensitivity intent (pick one convention + document + test).
9. Update `docs/handoff/CURRENT.md`:
   - Flip Status to Review.
   - Add "Wave β closure summary" subsection: per-§6-criterion + file:line.
   - Preserve Side Research md5.
10. Extend CHANGELOG.md `## v0.11.2 (unreleased) — tpatch doctor Wave α` header to cover Wave β (add a `### Wave β` subsection with new bullets).
11. Push to `origin/main`. Sanity-check `git log --format='%(trailers)' <sha>` for every new commit before pushing (rule 18).
12. Return commit hashes + gate output + closure summary.

If any §6 criterion is impossible without breaking a hard constraint: STOP and report.

Do not dispatch reviewers — supervisor handles that.

## Session Summary

Doctor Wave α closed 2026-07-27 (three-way APPROVED WITH NOTES). F-EXT-1 malformed-trailer finding caught by supervisor-external + fixed via forward attribution at `a3b9fe3`. Rule 18 (structural trailer verification) promoted to binding (18 total rules). Wave β kickoff ready.

## Next Steps

1. Supervisor: dispatch Wave β implementer.
2. After Wave β three-way APPROVED: archive to HISTORY, move to Wave γ.

## Blockers

None.

## Context for Next Agent

- HEAD at Wave β kickoff: `a3b9fe3` + review LOGs. Verify latest via `git log --oneline -n 5`.
- Wave α ship at `6319c0b` (malformed trailer, not restorable in-place — see F-EXT-1 note in `a3b9fe3` body).
- 18 carry-forward rules now binding. Rule 18 is the newest; sanity-check commit trailers structurally, not just via text-grep.
- Slice 1's `TestSkillRecipeSchemaMatchesCLI` is D7's build-time analog; Wave β's D7 is the runtime version.
- Slice 4 F2 amend documented the six `installSkills` paths for D3.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

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
