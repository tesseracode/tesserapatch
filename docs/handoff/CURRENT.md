# Current Handoff

## Active Task

- **Task ID**: M16-SLICE-3 — `feat-apply-default-execute` + `feat-skills-apply-auto-default` (unified)
- **Milestone**: M16 — Operator polish bundle (v0.6.4)
- **Status**: In Progress (implementer dispatched)
- **Assigned**: 2026-05-10

## Background and Re-scope

Original `feat-apply-default-execute` brief said "make `tpatch apply <slug>` default to `--mode execute`". On inspection of `internal/cli/cobra.go:586`, the **code default is already `"auto"`** (which "runs prepare→execute→done"). That's even better than the original ask — the common case is already a one-verb command: `tpatch apply <slug>` runs the full pipeline.

The remaining work is **doc/skill alignment**: 58 mentions of `apply --mode execute` across all 6 skill surfaces still document the old explicit-mode invocation in lifecycle diagrams, recipe-authoring guidance, and prose. Users reading the skills are still being told to type `--mode execute` explicitly.

This slice unifies two backlog todos:
- `feat-apply-default-execute` — confirm code default is `auto`; ensure help text / error messages reflect that the mode flag is optional.
- `feat-skills-apply-auto-default` — replace `apply --mode execute` with `apply` (or `apply --mode auto` where the explicit form makes more pedagogical sense) across all 6 skill surfaces.

## Acceptance Criteria

1. **Code default verified and documented**: `internal/cli/cobra.go` line ~586 already has `cmd.Flags().String("mode", "auto", ...)`. No code change needed there. Confirm:
   - The flag help text clearly says `auto` is the default.
   - The error message at `cobra.go:582` (`unknown apply mode %q (valid: auto, prepare, started, execute, done)`) lists `auto` first as the default.
   - If either is unclear, tighten with a one-line edit.

2. **Skill surface alignment** — all 6 skill files updated so the recommended user-facing invocation is `tpatch apply <slug>` (no explicit mode flag). The four-mode ladder (`prepare`, `started`, `execute`, `done`) remains documented as an advanced/state-machine fallback. Specifically:
   - `assets/skills/claude/tessera-patch/SKILL.md`
   - `assets/skills/copilot/tessera-patch/SKILL.md`
   - `assets/prompts/copilot/tessera-patch-apply.prompt.md`
   - `assets/skills/cursor/tessera-patch.mdc`
   - `assets/skills/windsurf/windsurfrules`
   - `assets/workflows/tessera-patch-generic.md`

   Replacement guidance:
   - **Lifecycle diagrams** (e.g. `implementing → tpatch apply --mode execute → applied`): replace with `implementing → tpatch apply → applied`. The `auto` default makes the explicit mode redundant in the simple case.
   - **Path-safety / created_by gate prose** (e.g. "abort `apply --mode execute`"): KEEP the `--mode execute` reference; this prose discusses what specifically gates the execute phase, and `auto` runs the same execute phase. Same with the apply-time gate language at v0.6.0. Touching these would obscure the actual semantics of which phase enforces what.
   - **General recommended-invocation prose** (anywhere that says "to apply your feature, run `tpatch apply <slug> --mode execute`"): replace with `tpatch apply <slug>`.
   - When in doubt: the `--mode execute` reference is preserved if it's *describing what the execute phase does*, and replaced if it's *telling the user how to invoke apply*.

3. **Parity-guard anchor** — extend `assets/assets_test.go` parity guard to require a new anchor that appears in every skill format. Suggested: the literal string `tpatch apply <slug>` (without any `--mode`) must appear at least once in each of the 6 skill files. This locks in the simplified user-facing invocation across all surfaces and prevents future drift.

4. **No behavioral change** to `cobra.go` flag default, mode dispatch, or the four-mode ladder semantics. The `--mode {prepare|started|execute|done}` flag remains fully usable for state-machine-style workflows.

5. **CHANGELOG v0.6.4 entry**: new `## v0.6.4 — 2026-05-10 — M16 (operator polish, completion)` section above v0.6.3 with a `### Changed` entry documenting the skill alignment, plus a `### Added` entry for the new parity-guard anchor. Note in prose that the code default has been `auto` since the v0.6.0 introduction of the auto mode — this slice closes the docs/skills gap.

## Validation Gate

- `gofmt -l .` → empty
- `go build ./cmd/tpatch` → ok
- `go test ./...` → all green
- `go test ./assets/ -run TestSkillAssetParity -count=1 -v` (or whatever the parity-guard test name is) → green and the new anchor is asserted
- `grep -rn "apply --mode execute" assets/skills/ assets/prompts/ assets/workflows/` — should return ONLY occurrences in the path-safety / created_by-gate prose contexts (i.e. talking about what the execute phase enforces, not how to invoke it). Lifecycle diagrams and recommended-invocation prose should be clean.

## Out of Scope

- Removing the four-mode ladder.
- Changing `auto` mode's behavior (still runs prepare→execute→done).
- Touching `internal/cli/cobra.go` beyond the help-text/error-message tightening if needed.
- Editing the 14 newly-committed v0.7-cluster paper docs (PRDs, WP-001, market-research, ADR-015, commits.md). They're approved as-is.
- Anything related to the v0.7 cluster routing pass (queued separately).

## Hard Rules

- Two-stage review: sub-agent reviewer first, then external supervisor (user) pass.
- Mandatory commit trailer:
  `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`
- Use `git -c commit.gpgsign=false commit ...`.
- Stay on branch `main`. Verify with `git status` after committing.
- Tree must be clean before you start (verified by supervisor — only the `tpatch` binary at root may be present, which is `.gitignore`-d).

## Files Expected to Change

- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/windsurf/windsurfrules`
- `assets/workflows/tessera-patch-generic.md`
- `assets/assets_test.go` — parity-guard anchor extension
- `CHANGELOG.md` — v0.6.4 section
- (Possibly `internal/cli/cobra.go` — only if help-text or error-message tightening is needed; keep diff minimal)

## Files Changed

- `assets/skills/claude/tessera-patch/SKILL.md` — Phase Ordering row + line ~276 worked example simplified to `tpatch apply <slug>`
- `assets/skills/copilot/tessera-patch/SKILL.md` — Phase Ordering row simplified
- `assets/prompts/copilot/tessera-patch-apply.prompt.md` — Phase Ordering row simplified
- `assets/skills/cursor/tessera-patch.mdc` — Phase Ordering row simplified
- `assets/skills/windsurf/windsurfrules` — Phase Ordering row simplified
- `assets/workflows/tessera-patch-generic.md` — Phase Ordering row simplified
- `assets/assets_test.go` — added parity-guard anchor `apply-default-auto/simple-invocation` requiring literal `tpatch apply <slug>` in all 6 surfaces
- `CHANGELOG.md` — new `## v0.6.4 — 2026-05-10 — M16 (operator polish, completion)` section above v0.6.3 with Changed/Added/Notes subsections

`internal/cli/cobra.go` was NOT modified — the help text at line 586 already says `auto (default, runs prepare→execute→done)` and the error message at line 582 already lists `auto` first. No tightening needed.

## Test Results

```
$ gofmt -l .
(empty)

$ go build ./cmd/tpatch
(ok)

$ go test ./...
ok  	github.com/tesseracode/tesserapatch/assets	0.642s
?   	github.com/tesseracode/tesserapatch/cmd/tpatch	[no test files]
ok  	github.com/tesseracode/tesserapatch/internal/buildinfo	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/cli	7.755s
ok  	github.com/tesseracode/tesserapatch/internal/gitutil	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/provider	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/safety	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/store	(cached)
ok  	github.com/tesseracode/tesserapatch/internal/workflow	(cached)

$ go test ./assets/ -count=1 -v
=== RUN   TestSkillParityGuard          ... PASS (all 6 surfaces)
=== RUN   TestAllSkillFilesExist        ... PASS (all 6)
=== RUN   TestSkillRecipeSchemaMatchesCLI ... PASS (all 6)
PASS
ok  	github.com/tesseracode/tesserapatch/assets	0.434s
```

The new `apply-default-auto/simple-invocation` anchor is asserted on every one of the 6 skill surfaces and all pre-existing anchors (invocation/go-binary, no-npx, no-cd, phase-ordering/table, never-skip, preflight/*, recipe-schema/*, conflict-playbook/*, patch-vs-recipe, verify-freshness/*) still pass.

## Context for Reviewer

### Scope of `apply --mode execute` audit

The handoff cited "58 mentions"; the actual count in `assets/skills/`, `assets/prompts/`, `assets/workflows/` was 25 before edit and 18 after. All 18 remaining occurrences are in **phase-semantics prose** that describes what the execute phase enforces, which is preserved by design.

### Replaced (invocation-recommendation prose)

- 6 × Phase Ordering table rows (one per surface): `implementing → tpatch apply --mode execute → applied` → `implementing → tpatch apply → applied`. The `OR` line below was retained and tagged `(advanced)` so the four-mode ladder remains documented as a fallback.
- 1 × `assets/skills/claude/tessera-patch/SKILL.md` line ~276 — manual-conflict-resolution worked example: `tpatch apply <slug> --mode execute  # or --mode started / --mode done if you authored ad-hoc` → `tpatch apply <slug>  # auto runs prepare→execute→done; or use --mode started / --mode done if you authored ad-hoc`. The ladder is still mentioned in the trailing comment.

### Preserved (phase-semantics prose)

The following 18 mentions ALL describe execute-phase enforcement, not invocation, so they were intentionally left as `apply --mode execute`:

```
$ grep -rn "apply --mode execute" assets/skills/ assets/prompts/ assets/workflows/
assets/skills/copilot/tessera-patch/SKILL.md:123 — created_by gate (live apply-time gate)
assets/skills/copilot/tessera-patch/SKILL.md:126 — path-safety abort (EnsureSafeRepoPath)
assets/skills/copilot/tessera-patch/SKILL.md:169 — created_by gate detail
assets/skills/cursor/tessera-patch.mdc:120 — created_by gate
assets/skills/cursor/tessera-patch.mdc:123 — path-safety abort
assets/skills/cursor/tessera-patch.mdc:166 — created_by gate detail
assets/skills/claude/tessera-patch/SKILL.md:165 — path-safety abort
assets/skills/claude/tessera-patch/SKILL.md:171 — created_by gate
assets/skills/claude/tessera-patch/SKILL.md:197 — created_by gate detail
assets/skills/windsurf/windsurfrules:114 — created_by gate
assets/skills/windsurf/windsurfrules:117 — path-safety abort
assets/skills/windsurf/windsurfrules:160 — created_by gate detail
assets/prompts/copilot/tessera-patch-apply.prompt.md:111 — created_by gate
assets/prompts/copilot/tessera-patch-apply.prompt.md:114 — path-safety abort
assets/prompts/copilot/tessera-patch-apply.prompt.md:157 — created_by gate detail
assets/workflows/tessera-patch-generic.md:137 — created_by gate
assets/workflows/tessera-patch-generic.md:140 — path-safety abort
assets/workflows/tessera-patch-generic.md:183 — created_by gate detail
```

Each is in prose like "from v0.6.0 a **live apply-time gate**: `apply --mode execute` rejects ops whose `created_by` parent is missing from `depends_on` (hard-parent miss fails in execute, warns in `--dry-run`)" or "`../`, absolute paths, or symlinks outside the repo abort `apply --mode execute`". Per the handoff §"Acceptance Criteria 2", these are kept verbatim because they describe phase semantics — `auto` runs the same execute phase, so the semantics carry through unchanged. Replacing with `tpatch apply` here would obscure which phase enforces what.

### Other `--mode` references (not in scope of this slice)

`--mode started` and `--mode done` references (Quick Start blocks, copilot apply-prompt Steps, "If reconcile returns 3WayConflicts" recipes, etc.) were left untouched because:
1. They document the four-mode ladder, which the handoff explicitly preserves as the advanced/state-machine fallback.
2. The handoff acceptance criteria scope only `--mode execute`.
3. Those flows describe the manual ad-hoc workflow where there's no recipe to auto-execute.

### Parity-guard anchor

Anchor label: `apply-default-auto/simple-invocation`
Anchor string: literal `tpatch apply <slug>` (with literal angle brackets — verified to appear in all 6 surfaces before adding the anchor).
Verification: `grep -c 'tpatch apply <slug>'` returns ≥1 on every surface (claude:2, copilot:2, copilot-prompt:1, cursor:2, windsurf:2, generic:2).

### Layered-discovery check

Confirmed all 6 surfaces still mention the four-mode ladder somewhere — every Phase Ordering table now has the explicit `OR tpatch apply --mode started / edit / --mode done    → applied (advanced)` row, and most surfaces additionally mention `--mode done` and/or `--mode started` in Steps / Quick Start / 3WayConflicts recipes. The advanced ladder is not removed from documentation; it is only de-emphasized for the common case.

### Code unchanged

`internal/cli/cobra.go` was reviewed:
- Line 586: `cmd.Flags().String("mode", "auto", "Apply mode: auto (default, runs prepare→execute→done), prepare, started, execute, done")` — already names `auto` as default with the pipeline expansion in parens. Clear; no edit.
- Line 582: `unknown apply mode %q (valid: auto, prepare, started, execute, done)` — already lists `auto` first. Clear; no edit.

### Two-stage review reminder

This sub-agent did not push, did not tag, and did not self-approve. The commit lands on `main` locally with the mandatory co-author trailer. Awaiting reviewer sub-agent then external supervisor pass.

## Blockers

None.
