# Current Handoff

## Status

**Cluster state**: REV-5 DISPATCHED

v0.15.1 Wave A rev-4 closes parser defects but remains blocked by the
pre-stage registration race. Rev-5 is dispatched.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-5
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record/reconcile capture and land
  planning.
- **Status**: In Progress
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Rev-4 dispatch HEAD**: `d516f5e`
- **Target release**: v0.15.1

## Rev-3 finding closures

### F1 (HIGH) — strict Git patch grammar — CLOSED

Three separate holes, all closed in
`internal/gitutil/patch_paths_strict.go`.

**Headerless input.** Non-blank input containing zero `diff --git`
headers returned an empty, error-free scope — the same "empty scope
means everything" failure the strict API exists to remove, reached
through a truncated artifact or a plain `diff -u` patch. It is now an
error. Whitespace-only input remains legitimately empty and is the
explicit control.

**Both operands validated.** `parseDiffGitOperands` parses and validates
the a-side AND the b-side before either is used: each must decode, must
carry its `a/` / `b/` prefix, and must have a non-empty payload. A
malformed a-side with a healthy b-side used to pass unnoticed. A
repository configured with `diff.noprefix` or `diff.mnemonicPrefix`
produces a different shape and is refused with an error naming that
cause, rather than guessed at. The `rename to` / `copy to` / `+++` /
`---` fallback is consulted only after the operands themselves are
structurally valid (the ambiguous-but-well-formed unquoted rename/copy
case).

**Git-specific C decoder.** `unquoteGitCStyle` replaces
`strconv.Unquote`. It accepts exactly what git's `quote_c_style()`
emits — `\a \b \f \n \r \t \v \" \\` plus three-digit octal — and
refuses everything else. Go's decoder accepts `\x41`, `\u0041`,
`\U0001F600`, `\'` and one- or two-digit octal; Git emits none of them,
so accepting them meant accepting corrupted bytes as if they were a
path. Short octal, overflowing octal (`\400`), trailing backslash,
unterminated quote, bytes after the closing quote and extra operands are
all refused. Every refusal returns `error` **and a nil slice** — never a
partial or empty scope.

Downstream behavior is unchanged in shape: `RefreshAfterAccept` and
`land`'s `computePathSet` both still fail before their own writes, and a
stale quoted-newline worktree-only patch still filters to empty and
regenerates nothing rather than widening to a full-tree diff.

### F2 (HIGH) — land pre-stage revalidation — CLOSED

`land` now spends exactly two discoveries of its own:

1. **Entry gate**, before the metadata snapshot and before the embedded
   `record`. Its result is deliberately discarded; its only job is to
   refuse before `record` can write anything if `git worktree list` is
   broken. (This is rev-3's F1 fix, retained.)
2. **Pre-stage revalidation**, immediately before planning and staging.
   The entry answer can go stale: `record` itself, or a concurrent agent
   harness, may register a linked worktree in between. A stale set turns
   that worktree into an ordinary "extra", which `--allow-extra-paths`
   would then stage as a gitlink — the GH #7 bug re-entering through a
   race.

The **entire** plan — path set, dirty paths, carve-out notes, extras —
is computed once against the revalidated set, so no diagnostic is
emitted twice and the success-path bytes are unchanged. The final path
set is then defensively re-filtered with
`gitutil.FilterNestedWorktreePaths` immediately before `git add`, so
nothing under a nested worktree can reach the index no matter which
branch put it in the set (including the `--allow-extra-paths` fold).

`land --dry-run` discovers once, at plan time rather than at entry, so
the printed plan reflects the latest registered-worktree set. It runs no
embedded record and performs no writes, so one call is both the first
and the last word.

**Documented boundary when the revalidation fails**: land refuses with
`status.json`, the index and HEAD all untouched. The embedded `record`'s
artifacts persist — that is `record`'s own completed transaction,
identical to running `tpatch record` followed by a failing
`tpatch land`. This is asserted explicitly, not left implicit.

### F3 (dispatch item 3) — status notes ordering — CONFIRMED

`status.json:notes` is written only after the revalidated planning, the
dirty-path classification and the extras gate have all passed, and it is
staged through the ordinary path set (which names it explicitly via
`includeStatus`). Pinned by
`TestNestedWorktree_Land_ExtrasSemanticsUnchangedAfterReorder` and the
revalidation-failure boundary test.

## Files Changed

Modified this rev:

- `internal/gitutil/patch_paths_strict.go`
- `internal/gitutil/patch_paths_strict_test.go`
- `internal/cli/land.go`
- `internal/cli/nested_worktree_test.go`
- `CHANGELOG.md`
- `docs/land.md`
- `docs/handoff/CURRENT.md`

Unchanged this rev: `internal/gitutil/worktrees.go`,
`internal/gitutil/capture_modes.go`, `internal/gitutil/gitutil.go`,
`internal/cli/cobra.go`, `internal/cli/nested_worktree_guard.go`,
`internal/cli/phase2.go`, `internal/workflow/refresh.go`, `SPEC.md`,
`docs/record.md`.

Deliberately NOT folded: the Makefile nested-repo sentinel LOW is
separately tracked.

## Test Results

- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean.
- `go test -count=1 ./...` — all packages `ok`.
- `go test -race -count=1` on `./internal/gitutil/`, `./internal/workflow/`
  and `./internal/cli/` — ok.
- Assets parity (`go test ./assets/`) — ok; no asset touched.
- 146 passing assertions across the GH #7 test set.

New coverage this rev:

- `TestFilesInPatchStrictRefusesHeaderlessNonBlankPatch` — plain
  `diff -u` output, a truncated artifact, prose, a lone hunk; each
  refuses with a nil scope and names the missing header.
- `TestFilesInPatchStrictWhitespaceOnlyIsNotAnError` — the control.
- `TestFilesInPatchStrictValidatesASide` — bad a-side escape,
  unterminated a-side, wrong prefix on either side, empty payload on
  either side, unquoted a-side without `a/`, missing b-side operand, a
  trailing third operand.
- `TestFilesInPatchStrictRefusesGoOnlyEscapes` — `\x41`, `\u0041`,
  `\U0001F600`, `\'`, `\e`, `\3`, `\30`, `\400`, each exercised on BOTH
  sides (16 cases).
- `TestUnquoteGitCStyleAcceptsExactlyGitsEscapes` — every escape Git
  emits decodes byte-correctly (including `\000` and `\377`), and
  eleven malformed forms are refused.
- `TestFilesInPatchStrictPreservesUnquotedWhitespace` — double-space
  unquoted path survives.
- `TestFilesInPatchStrictHandlesRealGitCopyEntry` — real
  `git diff -C --find-copies-harder` copy entry.
- `TestNestedWorktree_Land_RevalidatesBeforeStaging` — a `git` wrapper
  registers a NEW linked worktree on the 2nd `worktree list` call (i.e.
  after land's entry gate, during the embedded record, before the
  pre-stage revalidation). Broad land and scoped land, both with
  `--allow-extra-paths`, must keep it out of the index, the commit and
  the committed tree, while the intended files land.
- `TestNestedWorktree_Land_RevalidationFailureBoundary` — a wrapper
  fails exactly the pre-stage revalidation call (calibrated as
  `2 + recordCalls`); HEAD, the index and the `landed at` note are all
  unchanged, the record artifacts are asserted to persist, and a rerun
  recovers.
- `TestNestedWorktree_Land_RevalidationNoOpPreservesOutput` — with no
  registration in between, land's stdout and stderr are byte-stable
  across runs (landing SHA normalised).
- `TestNestedWorktree_LandDryRun_ReflectsLatestSetWithoutMutation` — a
  worktree registered after `apply` is absent from the printed plan, and
  the dry-run mutates neither HEAD nor the working tree.

Updated to the rev-4 contract:

- `TestNestedWorktree_Land_DiscoveryBudget` (was
  `..._DiscoversOnceBeforeEmbeddedRecord`) — the budget is now exactly
  `2 + recordCalls`, still calibrated against a standalone `record`
  rather than hardcoded, so a stray rediscovery in the planning path
  still fails the test.
- `TestNestedWorktree_Land_NoRecordAndDryRunDiscoveryBudget` —
  `--no-record` spends 2 (entry gate + revalidation), `--dry-run`
  spends 1 (plan time).

All prior original-issue, exotic-name, non-goal, refresh and
transaction tests were re-run unchanged and pass.

## Reproduction + control matrix (built binary)

Re-run at this HEAD with three nested worktrees (`agent review`,
`agent trail `, newline-named) plus every over-filtering control:

| Path | Kind | Result |
|------|------|--------|
| `.claude/worktrees/agent review` | nested worktree | absent from patch, diffstat, land plan, commit |
| `.claude/worktrees/agent trail ` | nested worktree, trailing space | absent from all four |
| `.claude/worktrees/new\nline` | nested worktree, embedded newline | absent from all four |
| `.claude/worktrees/agent-other/f.txt` | ordinary dir, prefix sibling | captured and landed |
| `vendor/plainrepo` | unregistered nested Git repo | captured and landed as a gitlink (correctly NOT filtered) |
| `../extwt` | worktree outside the root | never referenced |

Post-land `git status` lists only the three worktrees as untracked plus
the carved-out `.tpatch/FEATURES.md`. All scratch repos, worktrees and
build artifacts were removed; `git worktree list` shows only the primary
worktree.

## Reviewer focus

1. The `a/`+`b/` prefix requirement means a repository configured with
   `diff.noprefix` or `diff.mnemonicPrefix` now refuses rather than
   mis-parses. That is deliberate fail-closed behavior and the error
   names the cause; confirm the trade-off is the one you want, since
   tpatch's own artifacts are always generated without those options.
2. Land's discovery budget grew from `1 + recordCalls` to
   `2 + recordCalls`. The extra call is the revalidation; the test is
   calibrated, so any *third* land-side call still fails it.
3. The revalidation-failure boundary lets the embedded record's
   artifacts persist. Alternative designs (rolling record back) would
   require undoing a completed sub-command; the chosen boundary is
   asserted and documented instead.
4. A post-stage defensive audit/rollback was considered and rejected:
   un-staging after the fact risks disturbing unrelated index state that
   the operator staged themselves. The equivalent guarantee is provided
   pre-stage by the revalidated plan plus the final
   `FilterNestedWorktreePaths` pass immediately before `git add`, which
   is proven by the concurrent-registration test.
5. `land --dry-run` moved its discovery from entry to plan time. It
   performs no writes, so this is purely about the printed plan being
   current.

## Rev-4 Review Adjudication

- Internal: NEEDS REVISION.
- External/original reproducer: APPROVED.
- Parser findings are closed.
- Remaining HIGH: a worktree can register after pre-stage revalidation but
  before `dirtyPaths`/staging; `--allow-extra-paths` can stage its gitlink.
- `tpatch_rev4_bin` and review scratch are absent after external cleanup.

## Next Steps

1. Snapshot the effective index before land staging.
2. Stage non-status paths, rediscover, and audit the staged index.
3. Roll back exact index bytes on discovery failure or nested contamination.
4. Write/stage status only after the staged-index audit passes.
5. Run final dual review, then close #7 only after approval.

## Blockers

None.

## Context for Next Agent

- `internal/gitutil/worktrees.go` is the single discovery authority;
  `git worktree list --porcelain -z` is the single Git shape (Git 2.36+).
- `FilesInPatchStrict` is mandatory for any NEW code that derives a
  write scope, a diff scope or a staging decision from a patch. Decode
  Git quoting with `unquoteGitCStyle`, never `strconv.Unquote`.
- Land's contract: gate-discover before `record`, revalidate before
  planning/staging, filter the path set once more before `git add`,
  write `status.json` only after the last refusal.
- Byte exactness remains load-bearing: no `TrimSpace`, no hand-rolled
  dequote on any path compared against a worktree prefix.
- `PreflightReconcile` is still deliberately unfiltered: it is a hygiene
  gate, not a capture surface.
- Side Research md5 `b385fe622db9926f48861105239f113e` preserved.

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
