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

(implementer fills)

## Test Results

(implementer fills)

## Context for Reviewer

(implementer fills with: which files were touched, which `--mode execute` mentions were preserved and why, parity-guard anchor used, evidence that the simple invocation works end-to-end)

## Blockers

None.
