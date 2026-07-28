# Current Handoff

## Active Task

- **Task ID**: `doctor-wave-delta-rev1-f2-close`
- **Milestone**: `tpatch doctor` implementation — Wave δ rev-1 (D6 F2 close). FINAL rev on the FINAL wave of the 4-wave cluster.
- **Description**: Rev-1 fix-pass over Wave δ rev-0 (`8b7e969..faf8db1`). Internal + supervisor-external APPROVED (29/29 §6 confirmed); user-external APPROVED WITH NOTES with F2 MEDIUM caught (D6 user-workspace false-positive drift + `RELEASING.md`-in-runtime-output docs-reference defect, ADR-020 class). F2 must be closed before v0.11.2 ships. Three deliverables: F2-1 gate D6 tag-vs-CHANGELOG comparison to tpatch-authored release context; F2-2 self-contain D6 remediation strings per ADR-020 inline-minimal principle; F2-3 test coverage replicating user-workspace scenarios.
- **Status**: In Progress (rev-1).
- **Assigned**: 2026-07-28.

## Rev-1 findings (binding scope)

### F2 (MEDIUM) — D6 user-workspace false-positive drift + docs-reference defect

**Reproduced empirically** by user-external in 3 scenarios:

1. **Upstream repo with own tags + conventional changelog** (`## 1.2.0 (2024-01-01)` format): D6 flags EVERY upstream tag as `release-tag-missing-changelog` drift. `summary: 2 drift findings, 2 warnings` for a 2-tag repo.
2. **Repo with no CHANGELOG.md**: D6 emits `release-changelog-unreadable` at `error` severity.
3. **Docs-reference defect**: remediation strings say "follow RELEASING.md Step 1/2/3". `RELEASING.md` is a tpatch-repo-root doc — NOT installed to user workspaces via `tpatch init`. Same class ADR-020 already locked for shipped SKILL asset docs; parity guard structurally cannot cover runtime CLI output.

**Practical impact**: `tpatch doctor` — whose PRD goal is "suitable for CI, release checks, and pre-reconcile hygiene" — degrades the §6.24 CI-gate exit contract in most real user workspaces.

## Rev-1 deliverables (all three binding)

### F2-1: Gate D6 tag-vs-CHANGELOG comparison to tpatch-authored release context

Implementer chooses among three options + documents choice in closure summary:

**Option A (recommended)**: auto-detect via pattern matching.
- Only flag tags whose format matches `^v\d+\.\d+\.\d+$` (semver-with-v-prefix).
- Additionally require `CHANGELOG.md` to contain at least one `## v\d+\.\d+\.\d+ —` heading (i.e., detect tpatch-style CHANGELOG format).
- When either pattern doesn't match: skip D6 tag-vs-CHANGELOG comparison entirely; emit no findings for those tags. `unknown` GH-Release warnings still allowed but do NOT count as drift.
- Downgrade missing-`CHANGELOG.md` from `error` to `warning` (a missing CHANGELOG is a common state for many workspaces).

**Option B**: opt-in via `--release-metadata` OR a new local sentinel file (e.g., `.tpatch/release-drift-enabled`). Default behavior emits only `unknown` warnings. Safer but adds friction.

**Option C**: signature-gated via presence of `RELEASING.md` OR `.tpatch/tesserapatch-signature` in workspace. Skip D6 entirely when signature absent.

Option A is recommended because it matches tpatch's zero-config philosophy and doesn't require sentinel files or explicit opt-in.

### F2-2: Self-contain D6 remediation strings per ADR-020 inline-minimal principle

Replace ALL `RELEASING.md` references in `internal/workflow/doctor_d6.go` with inline actionable guidance. Recommended replacements (adjust wording as needed):

- **Missing CHANGELOG entry** for tag `vX.Y.Z`: "Add a section `## vX.Y.Z — YYYY-MM-DD — <scope>` to your `CHANGELOG.md` for tag `vX.Y.Z`."
- **Missing tag** for CHANGELOG heading `vX.Y.Z`: "Create annotated tag matching the CHANGELOG heading: `git tag -a vX.Y.Z -m 'vX.Y.Z — <scope>'`."
- **Missing GH Release** for tag `vX.Y.Z`: "Publish via: `gh release create vX.Y.Z --notes-file <extracted-notes> --verify-tag`."

Consistent with `docs/adrs/ADR-020-skill-doc-references.md` inline-minimal principle. Reviewer briefs will grep D6 code for `RELEASING.md` — must be ZERO hits in production code (test files + comments citing ADR-020 rationale acceptable).

### F2-3: Test coverage replicating user-workspace scenarios

Add regression tests to `internal/workflow/doctor_d6_test.go`:

1. **`TestDoctorD6SkipsUpstreamNonSemverTags`** (Option A) OR equivalent for Options B/C: replicate user-external Reproduction 1 — upstream repo with non-tpatch tag format (e.g., `1.2.0` without `v-` prefix, or upstream `v1.0.0` alongside `## 1.2.0 (2024-01-01)` heading). Assert D6 emits NO drift findings on tag-vs-CHANGELOG axis. `unknown` warnings still allowed but do NOT count as drift.
2. **`TestDoctorD6MissingChangelogIsWarning`**: replicate Reproduction 2 — repo with no `CHANGELOG.md`. Assert D6 emits WARNING severity (not error).
3. **`TestDoctorD6RemediationNoRepoRefs`**: assert D6 remediation strings do NOT contain the substring `RELEASING.md` (ADR-020 class regression guard).

## Rev-1 hard constraints (17 binding — Rule 19 promoted; Rule 20 candidate)

Same 16 as Wave δ rev-0 CURRENT.md + Rule 19 promotion:

1-15. Same as rev-0 (see prior handoff snapshot in HISTORY.md — Rule 18 structural trailer; etc.).
16. **Rule 19 (loader-caller-tracing) PROMOTED to binding**: reviewers MUST trace exported loader callers via grep before accepting store/workflow/cli diffs as internal refactor. If any caller is a shipped CLI surface, the diff carries a behavior-change contract that MUST have a §6 criterion, CHANGELOG bullet, and test. Applied successfully in Wave δ rev-0.
17. **Rule 20 CANDIDATE (post-rev-1 promotion)** — Reviewer briefs for user-facing CLI checks (D-clause detection code) MUST include an "empirically reproduce in a user-workspace scenario" step: build the binary, initialize a NON-tpatch repo, run the check, verify output is actionable and not noisy. Rule 9 generalization for user-workspace correctness. Rev-1 reviewer briefs MUST apply this candidate rule; promote to binding after rev-1 confirms broader applicability.

## Rev-1 process for implementer

1. Read the F2 deliverables above in FULL.
2. Read `docs/supervisor/LOG.md` top entry — user-external Wave δ APPROVED WITH NOTES with F2 detail + supervisor decision.
3. Read `docs/adrs/ADR-020-skill-doc-references.md` for the inline-minimal principle (F2-2 rationale).
4. Read `internal/workflow/doctor_d6.go` at HEAD — the code you're modifying.
5. Read `internal/workflow/doctor_d6_test.go` — existing fixtures + test patterns.
6. **REPRODUCE F2 EMPIRICALLY BEFORE FIXING** (Rule 20 candidate first application by implementer):
   - Build: `go build -o /tmp/tpatch_verify ./cmd/tpatch`
   - Scenario 1: `mkdir /tmp/user_workspace && cd /tmp/user_workspace && git init && printf '# Changelog\n\n## 1.2.0 (2024-01-01)\n\n- upstream\n' > CHANGELOG.md && git add . && git commit -m init && git tag v1.0.0 && /tmp/tpatch_verify init && /tmp/tpatch_verify doctor --check D6`
   - Confirm: current behavior emits drift findings (2 drifts, 2 warnings summary).
   - Document reproduction in closure summary.
7. Choose Option A/B/C for F2-1 + document rationale.
8. Implement F2-1 gating logic.
9. Implement F2-2 remediation rewrites.
10. Implement F2-3 tests.
11. **REPRODUCE F2 EMPIRICALLY AFTER FIXING** — same command; expect 0 drift findings, 0 errors (2 unknown warnings still allowed).
12. Update `CHANGELOG.md` `### Wave δ` subsection with a `- **F2 fix**` bullet describing the gating + remediation-self-containment.
13. Update `docs/handoff/CURRENT.md`:
    - Flip Status to Review.
    - Add "Wave δ rev-1 closure summary" subsection: F2-1 option chosen + rationale; F2-2 rewrites cited by function; F2-3 test names + assertions.
    - Add "Rule 20 first application" subsection: implementer's empirical-reproduction commands + expected output for the review to reproduce.
    - Preserve Side Research md5.
14. Verify trailers structurally after each commit: `git log -1 --format='%(trailers)'`.
15. Push to `origin/main`. Return commit hashes + gate output + trailer verification + Rule 20 empirical-repro commands.

If any deliverable is impossible without breaking a hard constraint: STOP and report.

Do not dispatch reviewers — supervisor handles that.

## Doctor implementation cluster wave plan

- **Wave α** ✅ CLOSED 2026-07-27 (three-way APPROVED WITH NOTES). Scaffold + D1 + D2 + D8.
- **Wave β** ✅ CLOSED 2026-07-28 (three-way APPROVED, zero findings). D3 + D7.
- **Wave γ** ✅ CLOSED 2026-07-28 (three-way APPROVED WITH NOTES, F1 folded to Wave δ). D4 + D5.
- **Wave δ rev-0** ✅ CLOSED 2026-07-28 (D6 + F1 fold-in accepted; F2 in rev-1).
- **Wave δ rev-1** ← this handoff (F2 close — final gate to v0.11.2 ship).

## Session Summary

Doctor Wave δ rev-0 APPROVED WITH NOTES (three-way). Full-cluster acceptance sweep 29/29 §6 MET. F1 fold-in exemplary. F2 caught by user-external via empirical user-workspace reproduction. Rev-1 close scope: three deliverables (F2-1 gating + F2-2 remediation self-containment + F2-3 tests). Rule 19 promoted to binding after successful Wave δ rev-0 first application. Rule 20 candidate proposed for post-rev-1 promotion.

## Next Steps

1. Supervisor: dispatch rev-1 implementer with above scope.
2. After rev-1 three-way APPROVED: archive Wave δ (rev-0 + rev-1) to HISTORY.md; close doctor implementation cluster; ship v0.11.2 following `RELEASING.md`.
3. Consider promoting rule 20 to binding based on rev-1 review feedback.

## Blockers

None.

## Context for Next Agent

- HEAD at rev-1 kickoff: `faf8db1` + user-external LOG entry pending commit.
- Doctor cluster ships under `v0.11.2 (unreleased)` — no released-surface break for the F1 or F2 changes.
- 19 binding + 1 candidate carry-forward rules.
- Two-opinion protocol: 14/14 rev cycles at final concurrence; user-external uniquely blocked/caught in 7 of 14 at rev-0 (F2 is the seventh).
- F2 pattern is the same class ADR-020 already locked for shipped SKILL asset docs; ADR-020's parity guard structurally cannot cover runtime CLI output. Consider extending ADR-020 to name runtime output as a covered surface after rev-1.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

## Doctor implementation cluster wave plan

- **Wave α** ✅ CLOSED 2026-07-27 (three-way APPROVED WITH NOTES). Scaffold + D1 + D2 + D8. §6.1-§6.7 + §6.20-§6.29 MET.
- **Wave β** ✅ CLOSED 2026-07-28 (three-way APPROVED, zero findings). D3 + D7. §6.8, §6.9, §6.18, §6.19 MET.
- **Wave γ** ✅ CLOSED 2026-07-28 (three-way APPROVED WITH NOTES; F1 caught by user-external). D4 + D5. §6.10-§6.13 MET. F1 folded into Wave δ.
- **Wave δ** ← this handoff (D6 + F1 fold-in). §6.14-§6.17 + F1 close. FINAL WAVE.

## Wave δ binding scope

### D6 — CHANGELOG / tag / GitHub Release drift (§6.14-§6.17)

**Detect** (§6.14): local git tags without matching CHANGELOG entries.
- Enumerate `git tag -l` output.
- For each tag, check if `CHANGELOG.md` contains a `## <tag> —` header.
- Report tags with no CHANGELOG entry.

**Detect** (§6.15): CHANGELOG release headings without matching local git tags.
- Enumerate `## vX.Y.Z — YYYY-MM-DD — ...` headers in `CHANGELOG.md` (skip `(unreleased)`).
- For each, check `git tag -l 'vX.Y.Z'`.
- Report CHANGELOG entries with no matching tag.

**Detect** (§6.16): GitHub Release presence from `--release-metadata` local input.
- New flag: `--release-metadata <file>` — local JSON file containing published GH Release metadata snapshot.
- Expected file shape (implementer chooses + documents; recommend `{"releases": [{"tag": "vX.Y.Z", "url": "...", "published_at": "..."}]}` OR the `gh release list --json tagName,url,publishedAt` output shape verbatim).
- For each tag in the snapshot, verify a matching local tag exists.
- Report tags absent from the snapshot.

**Detect** (§6.17): GH Release status as `unknown` when no `--release-metadata` is provided.
- Do NOT try to publish a release.
- Do NOT contact GitHub API.
- Do NOT prompt for auth.
- Report status `unknown` in JSON output for each tag; human output shows a compact warning.

Read-only in v1. No `--fix` for D6.

Remediation strings:
- Missing CHANGELOG entry: point at `RELEASING.md` Step 1 (verify path via rule 15 grep).
- Missing tag: point at `RELEASING.md` Step 2.
- Missing GH Release: point at `RELEASING.md` Step 3.

### F1 fold-in (from Wave γ user-external review)

Three deliverables:

**F1-1: Add a test asserting the new `tpatch reconcile review list --json` behavior**

Location: `internal/cli/reconcile_evidence_cli_test.go` (or equivalent CLI test file that already covers `reconcile review list`).

Test contract:
- Create a `reconcile-revisions.jsonl` file whose final line does NOT end with a newline.
- Run `tpatch reconcile review list --json <slug>`.
- Assert `corrupt_entries` array contains exactly one entry with `line=N` (where N is the final line number 1-indexed) and `error="final object is not newline-terminated"`.
- Assert command exits with a NON-ZERO exit code.
- Verify the `valid` / `revisions` array still contains any well-formed prior entries.

**F1-2: Add CHANGELOG bullet documenting the behavior change**

Location: `CHANGELOG.md`, under `## v0.11.2 (unreleased) — tpatch doctor` header. Add either a new `### Wave δ` subsection (recommended) OR amend the existing `### Wave γ` subsection with a `- **Behavior change**` bullet.

Recommended phrasing:
```
- **Behavior change** — `tpatch reconcile review list` now reports a
  non-newline-terminated final line as a `corrupt_entries` row (exits
  non-zero) instead of silently accepting it (exit zero). This aligns
  the lenient loader with ADR-025 D11 malformed-artifact semantics.
  The change was introduced alongside doctor D5 (`internal/store/
  reconcile_revision.go` at `cffeabd`) but was not documented at Wave γ
  ship time; documented here for release-note completeness.
```

**F1-3: Correct the Wave γ HISTORY.md snapshot's F1 misdescription**

The Wave γ HISTORY.md snapshot preserves the original CURRENT.md content verbatim including the phrase "existing `internal/store/reconcile_revision.go:168` lenient loaders" (implying no change). Do NOT rewrite the archived snapshot in place — history integrity matters. Instead:

Option A: Add a supersession footnote at the top of the Wave γ snapshot section noting: "**F1 correction**: The loader was changed. See Wave δ closure summary for accurate description." — This IS a HISTORY.md edit, but it's a supersession footnote at the SNAPSHOT HEADER, not a rewrite of the snapshot body. Acceptable.

Option B (recommended): Document the correction in the Wave δ closure summary in CURRENT.md under `## F1 fold-in closure`. Leave the Wave γ HISTORY.md snapshot untouched. When Wave δ archives, the correction becomes part of the next HISTORY.md snapshot naturally.

Implementer chooses A or B; documents rationale in closure summary.

## Wave δ hard constraints (binding, 16)

Same 15 as Wave γ + 1 new for the F1 fold-in:

1. **PRD as binding contract** — every fix traces to §6.X or STOP.
2. **Safety defaults non-negotiable**.
3. No new `FeatureState` values.
4. No new persisted schemas outside doctor JSON output.
5. **ADR-025 D11 pattern** for any malformed-artifact reporting.
6. **Rule 12 privacy** — D6 MUST NOT contact GitHub API, MUST NOT prompt for auth. `--release-metadata` file is LOCAL input only.
7. **Rule 15 (trigger-name grep)** — D6 remediation strings reference `RELEASING.md` sections that exist. Verify.
8. **Rule 11 (flag-surface accuracy)** — new `--release-metadata <file>` is LOCAL to `doctor` subcommand (not persistent root flag). Document in help text; note `--path` persistent inheritance if help lists flags.
9. **Rule 17 (totality claims)** — D6 detection MUST NOT teach "only `gh release` is checked" without noting GH-API-off scope.
10. **Rule 16 (anti-drift parity guard)** — if `--release-metadata` file shape drift-guards should exist, add a small parity test (e.g., decode a sample `gh release list --json` output into the doctor DTO).
11. **Rule 18 (structural trailer verification)** — every new commit passes `git log --format='%(trailers)' <sha>` non-empty.
12. **CHANGELOG.md** — extend existing `## v0.11.2 (unreleased) — tpatch doctor` header with `### Wave δ` subsection. F1-2 CHANGELOG bullet goes here.
13. **Assets/skills** — Wave δ adds a new user-visible flag (`--release-metadata`); update all 6 skill formats' doctor mention to include the flag OR at minimum note it exists. Parity guard MUST pass.
14. **Side Research md5** == `b385fe622db9926f48861105239f113e`.
15. **Full gates** + Co-authored-by trailer + structural verify.
16. **NEW — Rule 19 candidate** (loader-caller-tracing): if this wave OR any future wave touches an exported loader in `internal/store/`, the implementer MUST trace all callers via `grep -rn "LoaderName" internal/` and document any shipped CLI-surface impact in the closure summary. Reviewer briefs must verify the trace was done. Wave δ is the first wave to apply this rule.

## Wave δ suggested layout

- `internal/workflow/doctor_d6.go` — new: D6 detection using local git tag enumeration + CHANGELOG parse + optional `--release-metadata` local input.
- Extend `internal/workflow/doctor.go` — register D6 in check registry.
- Extend `internal/cli/doctor.go` — add `--release-metadata <file>` flag scoped to `doctor` subcommand.
- Update all 6 skill format doctor mentions to include the new flag.
- F1-1 test: extend `internal/cli/reconcile_evidence_cli_test.go` (or wherever `TestReconcileReviewList*` lives).
- F1-2 CHANGELOG bullet under `### Wave δ`.
- Tests:
  - `internal/workflow/doctor_d6_test.go` — fixtures: clean (tag+CHANGELOG+release match), tag missing CHANGELOG, CHANGELOG missing tag, tag missing GH release (with metadata provided), unknown status (no metadata provided).
  - Extend `internal/cli/doctor_test.go` for D6 `--release-metadata` end-to-end.
  - `internal/cli/reconcile_evidence_cli_test.go` — F1-1 test.

## Wave δ reviewer-brief additions

- Rule 15 for `RELEASING.md` remediation strings.
- Rule 11 for `--release-metadata` flag surface.
- Rule 12 privacy: verify NO GitHub API calls in D6 code (grep for `github.com/google/go-github`, `api.github.com`, HTTP clients).
- **Rule 19 (loader-caller-tracing) — FIRST APPLICATION**: reviewer briefs must verify implementer traced all exported loaders modified in Wave δ (should be none for D6, but if F1 fold-in touches `reconcile_revision.go` beyond the test, verify the trace happened).
- F1 verification: reviewer briefs must reproduce the F1-1 test scenario empirically (create JSONL file without trailing newline, run `review list --json`, verify exit non-zero + `corrupt_entries` present).

## Process for implementer

1. Read `docs/prds/PRD-tpatch-doctor.md` in FULL. Focus on §3 D6, §4 user-facing contract (esp. `--release-metadata`), §5 implementation notes, §6.14-§6.17.
2. Read this handoff Wave δ binding scope + F1 fold-in + hard constraints (16) verbatim.
3. Read `docs/supervisor/LOG.md` top 3 entries: user-external Wave γ APPROVED WITH NOTES (F1 detail) + supervisor Wave γ decision (F1 fold-in) + supervisor-external Wave γ APPROVED.
4. Read production ground truth:
   - `internal/store/reconcile_revision.go` — the lenient loader that F1 references. Trace ALL callers via `grep -rn "LoadReconcileRevisionsLenient" internal/` before writing the F1-1 test (rule 19 application).
   - `RELEASING.md` — the doc D6 remediation strings will reference.
   - `internal/cli/cobra.go` — grep for `tpatch reconcile review list` handling (D6 F1-1 test target).
   - `internal/workflow/doctor.go` — Wave α+β+γ scaffold + check registry.
   - `internal/workflow/doctor_d5.go` — closest pattern for D6 (persisted-artifact drift detection).
5. Verify `RELEASING.md` remediation references (rule 15) BEFORE writing D6 remediation strings.
6. Implement D6 detection (§6.14-§6.17). Include `--release-metadata` flag wiring.
7. Add F1-1 test.
8. Add F1-2 CHANGELOG bullet.
9. F1-3 handoff correction (Option A or B; document choice).
10. Update all 6 skill formats + parity guard.
11. **VERIFY TRAILER STRUCTURALLY** after each commit: `git log -1 --format='%(trailers)'`.
12. Run gates after each commit.
13. Update `docs/handoff/CURRENT.md`:
    - Flip Status to Review.
    - Add "Wave δ closure summary" subsection.
    - Add "F1 fold-in closure" subsection covering F1-1, F1-2, F1-3.
    - Add "Rule 19 application" subsection: list all exported loaders you traced + any callers you found + confirm no undisclosed behavior changes ride along.
    - Preserve Side Research md5.
14. Extend CHANGELOG.md `## v0.11.2 (unreleased)` header with `### Wave δ` subsection.
15. Push to `origin/main`. Return commit hashes + gate output + trailer structural verification + Rule 19 trace log.

If any §6 criterion is impossible without breaking a hard constraint: STOP and report.

Do not dispatch reviewers — supervisor handles that.

## Session Summary

Doctor Wave γ closed 2026-07-28 (three-way APPROVED WITH NOTES; F1 caught by user-external). F1 is a Wave γ user-external finding: undisclosed behavior change to shipped `tpatch reconcile review list` surface via changed lenient loader. Folded into Wave δ scope per user-external recommendation.

Wave δ closes the 4-wave doctor implementation cluster. After Wave δ APPROVED, v0.11.2 (unreleased) is ready to ship.


## Wave δ closure summary

Wave δ implementation is complete and ready for review. D6 now runs as a registered doctor check with local-only release drift detection: local release tags missing CHANGELOG headings, CHANGELOG release headings missing local tags, GitHub Release presence from a caller-provided local `--release-metadata` JSON snapshot, and explicit `unknown` GH Release warnings when no snapshot is provided. D6 remediation strings reference verified `RELEASING.md` Step 1 / Step 2 / Step 3 headings, and the implementation does not call the GitHub API or prompt for auth.

Files changed for Wave δ: `internal/workflow/doctor.go`, `internal/workflow/doctor_d6.go`, `internal/workflow/doctor_d6_test.go`, `internal/cli/doctor.go`, `internal/cli/doctor_test.go`, all six shipped skill/prompt/workflow asset formats, and `CHANGELOG.md`. The doctor-local `--release-metadata <file>` flag is scoped to the `doctor` subcommand; root persistent flags such as `--path` remain inherited.

Validation after implementation commit `a3cfe29`:
- `gofmt -l .` — clean
- `go test ./...` — PASS
- `go build ./cmd/tpatch` — PASS
- Targeted pre-commit coverage also passed: `go test ./internal/workflow ./internal/cli ./assets -run 'TestDoctorD6|TestDoctorCLI|TestReconcileReviewListReportsNonNewline|TestSkillParityGuard' -count=1`

## F1 fold-in closure

F1-1 is closed by `TestReconcileReviewListReportsNonNewlineTerminatedFinalRevision` in `internal/cli/reconcile_evidence_cli_test.go`. The test creates a `reconcile-revisions.jsonl` whose final valid object lacks a trailing newline, runs `tpatch reconcile review list --json <slug>`, asserts non-zero exit, asserts exactly one `corrupt_entries` row at line 2 with `error="final object is not newline-terminated"`, and verifies the preceding valid revision remains in the `revisions` array.

F1-2 is closed by the new `CHANGELOG.md` `### Wave δ` subsection documenting the `tpatch reconcile review list` behavior change and its ADR-025 D11 rationale.

F1-3 uses Option B from the handoff: the Wave γ HISTORY snapshot is left untouched for history integrity, and this Wave δ closure summary records the correction. Rationale: the archived Wave γ snapshot remains a verbatim historical artifact while the next HISTORY archive will naturally preserve this superseding correction.

## Rule 19 application

Exporter/caller trace run before writing F1-1 and re-run at closure:
- `LoadReconcileRevisionsLenient` callers:
  - `internal/cli/cobra.go:2157` — shipped `tpatch reconcile review list` surface; F1 is the only shipped-surface behavior change and is now tested + documented.
  - `internal/workflow/doctor_d5.go:80` — doctor D5 read-only malformed revision reporting.
  - `internal/store/reconcile_revision_test.go:54` — store unit test.
- `LoadReconcileEvidenceLenient` callers:
  - `internal/workflow/doctor_d5.go:37` — doctor D5 read-only malformed evidence reporting.

Wave δ did not modify exported store loaders. The only shipped-surface behavior change carried from Wave γ is the already-implemented `tpatch reconcile review list` non-newline-terminated final-line behavior; Wave δ adds no additional loader behavior changes.

## Files Changed

- `internal/workflow/doctor.go`
- `internal/workflow/doctor_d6.go`
- `internal/workflow/doctor_d6_test.go`
- `internal/cli/doctor.go`
- `internal/cli/doctor_test.go`
- `internal/cli/reconcile_evidence_cli_test.go`
- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/windsurf/windsurfrules`
- `assets/workflows/tessera-patch-generic.md`
- `CHANGELOG.md`
- `docs/handoff/CURRENT.md`

## Test Results

- `gofmt -l .` — clean
- `go test ./...` — PASS
- `go build ./cmd/tpatch` — PASS
- `git log -1 --format='%(trailers)' a3cfe29` — `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`

## Next Steps

1. Supervisor: dispatch Wave δ reviewers.
2. Reviewers: verify D6, F1 closure, Rule 19 trace, skill parity, and full gates.
3. After Wave δ three-way APPROVED: archive to HISTORY, close doctor cluster, decide on v0.11.2 release timing.

## Blockers

None.

## Context for Next Agent

- HEAD at Wave δ kickoff: `a1c1864` + review LOGs. Verify latest via `git log --oneline -n 5`.
- Doctor waves α+β+γ are unreleased; Wave δ still ships under `v0.11.2 (unreleased)`.
- 18 carry-forward rules binding. Rule 19 candidate proposed (loader-caller-tracing) — first application in Wave δ.
- F1 fold-in has three deliverables (test + CHANGELOG bullet + handoff correction) — all binding for Wave δ closure.
- Two-opinion protocol scoreboard: 14/14 cycles at final concurrence; user-external uniquely blocked/caught in 6 of 14 at rev-0. Continues to be the primary defense.
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
