# Current Handoff

## Active Task

- **Task ID**: M16-SLICE-2 — `bug-record-roundtrip-false-positive-markdown`
- **Milestone**: M16 — Operator polish bundle (v0.6.3)
- **Status**: In Progress (implementer dispatched)
- **Assigned**: 2026-05-10

(Slice 1 — gitignore — closed without commit; the rule was already in place from a prior cycle. See `docs/ROADMAP.md` M16 Slice 1 row.)

## Background

During a v0.4.3 t3code readme-copilot-notice smoke test, `tpatch record <slug>` succeeded — `patches/001-record.patch` was written and the apply itself was correct on disk — but `gitutil.ValidatePatchReverse` (the A3 reverse-apply round-trip check) emitted:

```
warning: patch does not round-trip against working tree: error: corrupt patch at line 28
  The recorded patch may not represent the on-disk changes accurately.
  Common causes: line-ending differences, binary files without --binary, or post-apply edits.
```

The data is fine; only the warning is wrong. But A3's value proposition is "trust the diagnostics" — a misleading warning is a regression of that promise.

Reproduction shape (per the original report):

1. Repo with a `README.md`.
2. `apply-recipe.json` with a `replace-in-file` op that inserts multi-paragraph blockquote content (e.g. `> [!CAUTION]` Markdown alert blocks with multiline body).
3. `tpatch apply --mode execute && tpatch record <slug>`.
4. Observe the false-positive warning despite the on-disk file being correct.

## Acceptance Criteria

1. **Reproducer test**: A regression test under `internal/gitutil/` (preferably `gitutil_test.go` near the existing `TestValidatePatchReverse_*` tests, or a dedicated `gitutil_markdown_roundtrip_test.go`) that constructs a working-tree state mirroring the smoke-test shape — markdown file with a multi-paragraph blockquote insertion via `replace-in-file`-style content — and runs `ValidatePatchReverse` on the captured patch. Test must reproduce the false positive on `main` *before* the fix and pass after.
2. **Root cause identified and named in a code comment** at the fix site. Likely candidates from the todo description:
   - **Final-newline handling** — Markdown files often lack a trailing newline; the diff may carry `\ No newline at end of file`, and the reverse-apply check may stat the working tree differently.
   - **Line-ending normalization** — autocrlf or core.eol set on the host; the patch was generated against LF working tree but read back through git's CRLF lens.
   - **Long single-line content** — multi-paragraph content that the replace-in-file op stored as a single logical operation may be re-flowed by git diff's hunk formatter inconsistently with how `ValidatePatchReverse` re-reads it.
   - **Binary-detection misfire** — markdown with non-ASCII content (em-dashes, smart quotes from the `> [!CAUTION]` block) flips git into binary mode without `--binary`.
3. **Fix is targeted** — minimal change in `internal/gitutil/gitutil.go` `ValidatePatchReverse` (or a sibling helper) that addresses the identified root cause. Prefer fixing the validator (e.g. pass extra `git apply` flags such as `--ignore-whitespace` or `--allow-binary-replacement` if applicable, or normalize input before re-feeding) over silencing the warning.
4. **No regression** — the existing `TestValidatePatchReverse_*` tests still pass, including the negative case `TestValidatePatchReverse_FailsWhenPatchDoesNotMatch` (we must NOT mute genuine round-trip failures).
5. **Validation gate**: `gofmt -l .`, `go build ./cmd/tpatch`, `go test ./...`. All clean.
6. **Handoff updated** with: root cause hypothesis, what reproducer captures, what the fix does, why it doesn't mute the negative case.

## Suggested Investigation Steps (for the implementer)

1. Build a tempdir reproducer harness in a `_test.go` file:
   - `git init`, set `user.email`/`user.name`, write a 50-line `README.md` with no trailing newline plus an existing `> [!NOTE]` block.
   - Replace a section with a multi-paragraph `> [!CAUTION]` block via direct file write (mimic what `replace-in-file` would produce).
   - Capture the diff via `git diff --no-color HEAD -- README.md` (the same path `tpatch record` uses).
   - Pass that captured patch into `ValidatePatchReverse`. Assert it succeeds.
2. If the test passes immediately, vary one knob at a time until it fails: `core.autocrlf=input`, missing trailing newline, smart quotes, `> [!CAUTION]` multi-line body, very long single line.
3. Once you can reproduce, run `git apply --reverse --check -v` manually with the captured patch and read git's verbose error to pinpoint line 28 (or wherever git points).
4. Implement the fix at the validator level. Document the root cause in a comment above the relevant line.
5. Confirm `TestValidatePatchReverse_FailsWhenPatchDoesNotMatch` still fails on a genuinely corrupt patch.

## Files Expected to Change

- `internal/gitutil/gitutil.go` — `ValidatePatchReverse` (or sibling helper)
- `internal/gitutil/gitutil_test.go` (or new `_test.go`) — reproducer + regression test(s)
- `CHANGELOG.md` — add `## v0.6.3` heading if not present, plus a `### Fixed` bullet for this slice. Do not add Slice 3 entries yet.

## Out of Scope (do NOT do)

- Touching `feat-apply-default-execute` (Slice 3).
- Refactoring `ValidatePatchReverse` beyond what's needed to fix the false positive.
- Adding new flags to `tpatch record` (the `--lenient` flag already exists for whitespace-sensitive files; do not add a markdown-specific bypass).
- Modifying the recipe schema, `replace-in-file` semantics, or `tpatch record` outside the validator path.

## Validation Gate

- `gofmt -l .` → empty
- `go build ./cmd/tpatch` → ok
- `go test ./...` → all green, **including the new reproducer**
- `go test ./internal/gitutil/ -run TestValidatePatchReverse -count=1 -v` → shows the new test name(s)

## Hard Rules

- Two-stage review: sub-agent reviewer first, then external supervisor (user) pass. Do NOT request the user pass yourself.
- Mandatory commit trailer:
  `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`
- Use `git -c commit.gpgsign=false commit ...`.
- Stay on branch `main`; verify with `git status` after committing.
- Do not `git add` any of the untracked files listed below.

## Known Untracked Files (DO NOT add)

`docs/whitepapers/`, `docs/market-research/`, `docs/commits.md`, `docs/supervisor/LOG.md.backup`, `docs/adrs/ADR-015-prior-art-identity-mapping.md`, exploratory PRDs in `docs/prds/PRD-*` (PRD-feature-slices-and-nested-changes, PRD-intent-version-control-evaluation, PRD-patch-already-upstream-detector, PRD-reconcile-lock-guard, PRD-record-auto-base, PRD-record-collision-detection, PRD-tpatch-git-primitive-mapping, PRD-tpatch-hotfix, PRD-tpatch-land), and `tpatch` binary at repo root.

`docs/adrs/README.md` shows `M` from a prior session — leave untouched.

## Files Changed

- `internal/gitutil/gitutil.go` — replaced `strings.TrimSpace(...)+ "\n"` tail normalization in `CapturePatchScoped` and `CapturePatchFromCommitsScoped` with a new `normalizePatchTail` helper that preserves trailing whitespace on content lines and only normalizes the count of trailing newlines. Helper carries a long comment naming the root cause.
- `internal/gitutil/gitutil_test.go` — added `TestValidatePatchReverse_MarkdownBlockquoteRoundtrip` regression test. Stages a `> [!CAUTION]` multi-paragraph blockquote whose final added line is `> ` (trailing space), captures via `CapturePatchScoped`, asserts the captured patch still contains `+> \n`, asserts a single trailing newline, and runs `ValidatePatchReverse` against the working tree.
- `CHANGELOG.md` — added `## v0.6.3 — unreleased — M16 (operator polish)` section with a `### Fixed` bullet for this slice.

## Test Results

`go test ./internal/gitutil/ -run TestValidatePatchReverse -count=1 -v`:

```
=== RUN   TestValidatePatchReverse_RoundtripsAgainstWorkingTree
--- PASS: TestValidatePatchReverse_RoundtripsAgainstWorkingTree (0.19s)
=== RUN   TestValidatePatchReverse_FailsWhenPatchDoesNotMatch
--- PASS: TestValidatePatchReverse_FailsWhenPatchDoesNotMatch (0.12s)
=== RUN   TestValidatePatchReverse_EmptyPatch
--- PASS: TestValidatePatchReverse_EmptyPatch (0.00s)
=== RUN   TestValidatePatchReverse_MarkdownBlockquoteRoundtrip
--- PASS: TestValidatePatchReverse_MarkdownBlockquoteRoundtrip (0.22s)
PASS
ok      github.com/tesseracode/tesserapatch/internal/gitutil    0.949s
```

Pre-fix sanity check (stash the `gitutil.go` change, re-run the new test against `main`):

```
--- FAIL: TestValidatePatchReverse_MarkdownBlockquoteRoundtrip (0.20s)
    gitutil_test.go:175: captured patch lost trailing whitespace on blockquote line.
```

`go test ./...` summary (all green):

```
ok      github.com/tesseracode/tesserapatch/assets             (cached)
ok      github.com/tesseracode/tesserapatch/internal/buildinfo (cached)
ok      github.com/tesseracode/tesserapatch/internal/cli       16.826s
ok      github.com/tesseracode/tesserapatch/internal/gitutil   8.389s
ok      github.com/tesseracode/tesserapatch/internal/provider  (cached)
ok      github.com/tesseracode/tesserapatch/internal/safety    (cached)
ok      github.com/tesseracode/tesserapatch/internal/store     2.216s
ok      github.com/tesseracode/tesserapatch/internal/workflow  34.718s
```

`gofmt -l .` → empty. `go build ./cmd/tpatch` → ok.

## Context for Reviewer

**Root cause (named precisely)**: over-eager whole-patch trimming in `gitutil.CapturePatchScoped` (and the same line in `CapturePatchFromCommitsScoped`). The previous code did `result := strings.TrimSpace(patch); if result != "" { result += "\n" }`. `strings.TrimSpace` strips ALL trailing whitespace bytes, not just the trailing newline — so when the very last line of a `git diff` output is a content line whose payload ends in trailing whitespace (canonical example: a markdown blockquote continuation `+> ` whose space after the `>` is semantically meaningful), the trailing space is consumed alongside the trailing `\n`. Re-appending `"\n"` then produces a patch whose last hunk line is `+>` instead of `+> `. That patch no longer matches the on-disk file; `git apply --reverse --check` (correctly) rejects it; `tpatch record` surfaces this as a misleading "patch does not round-trip against working tree" warning; and the corrupted patch is also persisted to `patches/NNN-record.patch`, so even the on-disk artifact is wrong (data bug, not just a warning bug).

**Reproducer shape**: tempdir git repo with `README.md` = `# Project\n` committed. Working tree replaces it with a `> [!CAUTION]` block ending in a `> ` (trailing-space) continuation line. Capture via `CapturePatchScoped`. Assert (a) the captured patch still contains `+> \n`, (b) it has exactly one trailing newline, (c) `ValidatePatchReverse` succeeds. The original "corrupt patch at line 28" wording from the t3code v0.4.3 smoke test is git-version-dependent surface text; the underlying failure mode (patch mutated such that reverse-apply fails) is exactly what the new test pins.

**Why the fix doesn't mute the negative case**: the fix lives in capture, not validation. `ValidatePatchReverse` itself is unchanged — it still runs `git apply --reverse --check` with strict semantics and no `--ignore-whitespace`. `TestValidatePatchReverse_FailsWhenPatchDoesNotMatch` (pre-existing) feeds a hand-crafted patch whose context lines do not match the working tree; that test continues to pass. The new helper `normalizePatchTail` only touches the byte sequence of trailing newlines (and continues to collapse a wholly-whitespace capture to `""` so the upstream "0 bytes — nothing to record" diagnostic still fires); it never touches content bytes.

**Scope of fix**: two functions changed in `internal/gitutil/gitutil.go`; both now route their tail normalization through the same helper. No CLI changes, no recipe-schema changes, no `tpatch record` flag changes. `--lenient` remains the existing escape hatch for whitespace-sensitive captures and is untouched.

## Blockers

None.
