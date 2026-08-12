# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave A (GitHub issue #7) is implemented, validated and pushed.
Awaiting review.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record capture and land planning.
- **Status**: Review
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Dispatch HEAD**: `3c55e43`
- **Target release**: v0.15.1

## Session Summary

Implemented the bounded fix direction recorded on the issue: one shared
nested-linked-worktree discovery/filter helper, reused by every untracked
capture surface and by `land`'s dirty-path / staging-plan classification.

New file `internal/gitutil/worktrees.go` is the single authority:

- `NestedWorktreePrefixes(repoRoot)` — discovery. Prefers the
  NUL-delimited `git worktree list --porcelain -z` shape (unambiguous for
  paths containing any byte except NUL) and transparently falls back to
  the newline-delimited `--porcelain` form when `-z` is rejected, so the
  repository's Git floor is not raised past the pre-existing one. The
  path is everything after the `worktree ` key and is never split on
  whitespace, so spaces and other special characters parse correctly.
- `PathUnderNestedWorktree(path, prefixes)` — the one path-membership
  primitive. Segment-boundary exact: `agent` never matches `agent-other`;
  a nested worktree root and every descendant match. Trailing `/` (which
  both `git ls-files --others` and `git status --porcelain` emit for
  un-recursed directories) and a leading `./` normalize away.
- `FilterNestedWorktreePaths`, `nestedWorktreeCaptureFilters` (renders
  `:(exclude,literal)` diff pathspecs — `literal` is required because a
  worktree directory name may contain glob metacharacters), and
  `NestedWorktreeDiscoveryError` / `ErrNestedWorktreeDiscovery` for the
  fail-closed class.

Classification rules: the target repository root itself is never a
prefix; worktrees resolving outside the root are dropped; paths are
compared after `filepath.EvalSymlinks` because Git prints physical paths
(this is exactly the `/var` → `/private/var` shape `t.TempDir()`
produces on macOS).

Wiring:

- `internal/gitutil/capture_modes.go` — `listUntrackedFiles` subtracts
  nested worktrees before any caller can hand a path to
  `git add --intent-to-add`. This single choke point covers default and
  manual Path B capture (`CapturePatchScoped`), `--all`, `--unstaged`
  (`CaptureUnstagedPatch`), and the overlap diagnostics
  (`untrackedFiltered` → `StagedUnstagedOverlap`). Subtraction is applied
  regardless of pathspecs, so `record --files <worktree>` cannot re-admit
  it. `CaptureStagedPatch`, `CaptureUnstagedPatch`, `stagedNameOnly` and
  `unstagedNameOnly` additionally append `:(exclude,literal)` pathspecs
  to their diffs as defense in depth against index residue.
- `internal/gitutil/gitutil.go` — `CapturePatchScoped` appends the same
  exclude pathspecs and fails closed on discovery error.
- `internal/cli/land.go` — `dirtyPaths` subtracts nested worktrees (so
  they never reach staging, the outside-path/refusal list, or the
  `--allow-extra-paths` sweep) and now fails closed; `computePathSet`
  additionally filters `FilesInPatch`, so a stale pre-fix
  `post-apply.patch` carrying a `mode 160000` gitlink cannot cause a
  broad/default `land` to stage the worktree. `computePathSet` returns an
  error; both the real-land and `--dry-run` call sites propagate it.
- `internal/cli/cobra.go` (`runApplyDone`) and `internal/cli/phase2.go`
  (`cycle` record step) hard-fail on `ErrNestedWorktreeDiscovery` instead
  of degrading to their historical warning-and-continue path.

Recipe autogen needs no separate filter: it derives write-file ops from
the captured patch, which no longer contains the worktree, so the
directory read never happens. This is asserted, not assumed.

No ADR was required — the fix reuses the locked capture/land
architecture and introduces no new decision.

## Current State

Working. Reproduced the pre-fix behavior with a binary built from
`3c55e43` and confirmed each symptom is gone with the fixed binary
(see Test Results).

Deliberately unchanged: ordinary directories (including prefix-boundary
siblings), intentionally tracked gitlinks/submodules, unregistered nested
Git repositories, linked worktrees registered outside the target root,
running tpatch *from* a linked worktree (the existing effective-index
behavior in `DiffFromCommitForPaths` is untouched and still covered by
`TestDiffFromCommitForPathsUsesLinkedWorktreeIndex`), and
`--allow-extra-paths` semantics for ordinary unrelated dirty paths.

## Files Changed

Created:

- `internal/gitutil/worktrees.go`
- `internal/gitutil/worktrees_test.go`
- `internal/cli/nested_worktree_test.go`

Modified:

- `internal/gitutil/capture_modes.go`
- `internal/gitutil/gitutil.go`
- `internal/cli/land.go`
- `internal/cli/cobra.go`
- `internal/cli/phase2.go`
- `CHANGELOG.md`
- `SPEC.md`
- `docs/land.md`
- `docs/record.md`
- `docs/handoff/CURRENT.md`

## Test Results

- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean.
- `go test -count=1 ./...` — all packages `ok` (assets, buildinfo, cli,
  gitutil, provider, redact, rescap, safety, store, studyvalidator,
  workflow, tests/integration).
- `go test -race -count=1 ./internal/gitutil/ -run 'NestedWorktree|...'`
  — ok. The changed code spawns Git subprocesses and holds no shared
  mutable state, so no other package needed a race pass.
- Assets parity (`go test ./assets/`) — ok. No asset was touched.

New coverage:

- `internal/gitutil/worktrees_test.go` — porcelain parsing in both the
  NUL and newline shapes; paths with spaces; malformed/failing output
  (empty output, no worktree records, empty path) errors rather than
  reporting "no nested worktrees"; classification of primary root,
  nested path, nested path with spaces and outside-root worktree;
  prefix-boundary (`agent` vs `agent-other`); real-repo discovery with a
  nested worktree, an external worktree, an unregistered nested repo and
  a sibling directory; discovery from inside a linked worktree; fail-
  closed outside a Git repository; untracked discovery (default and
  explicitly-scoped-to-the-worktree); `CapturePatchScoped` in three
  pathspec shapes plus a no-residue check; `CaptureUnstagedPatch` and
  overlap diagnostics; tracked-gitlink non-regression.
- `internal/cli/nested_worktree_test.go` — real `git worktree add`
  fixtures for manual Path B `apply --mode done` (canonical + every
  numbered patch); default `record` (patch, recipe, diffstat, and no
  autogen directory-read warning); `record --all/--staged/--unstaged`;
  scoped `record --files` in three shapes including one that names the
  worktree explicitly and one that names its parent; `land --dry-run
  --files` (absent from staging AND from outside-path, while an ordinary
  unrelated dirty path still refuses); real `land` (not staged, not
  committed, intended paths land, worktree survives); scoped refusal +
  `--allow-extra-paths` + broad/default land; external registered
  worktree and intentionally tracked gitlink non-regression.

Every fixture worktree is removed with `git worktree remove --force`
plus a prune via `t.Cleanup`, which runs before `t.TempDir()` teardown.
`git worktree list` in this repository shows only the primary worktree
after the run.

## Issue #7 reproduction — before and after

Built a pre-fix binary from `3c55e43` (`git archive HEAD`) and a fixed
binary from the working tree, then ran the issue's reproduction against
a scratch repo with `git worktree add ".claude/worktrees/agent review"`:

| Step | pre-fix `3c55e43` | fixed |
|------|-------------------|-------|
| `apply --mode done` canonical patch | `diff --git a/.claude/worktrees/agent review …` + `new file mode 160000` | only `README.md` + `internal/example.go` |
| numbered `patches/001-apply.patch` | same gitlink entry | clean |
| default `record` + recipe autogen | captures the gitlink; autogen reads the directory | clean, 2 ops, no warning |
| `record --files …,'.claude/worktrees/agent review'` | n/a | clean |
| `land --dry-run --files …` | `Outside path set: M .claude/worktrees/agent review/` | worktree absent; `unrelated.txt` still listed |
| real `land` | n/a | worktree neither staged nor committed; `README.md` + `internal/example.go` land |
| broad `land --allow-extra-paths` | n/a | worktree still absent from the commit |

The scratch repos and their worktrees were removed after validation.

## Next Steps

1. Review this wave (internal + external).
2. On acceptance, run the Wave-Close Checklist and ship v0.15.1.
3. Proceed to GH #8 (contract amendment, then implementation).

## Blockers

None.

## Context for Next Agent

- `internal/gitutil/worktrees.go` is the single authority. Any new
  untracked-discovery or staging surface must route through
  `NestedWorktreePrefixes` + `PathUnderNestedWorktree` rather than
  re-deriving the classification.
- The filter is scoped to *registered linked worktrees nested under the
  target root*. It deliberately does not touch gitlinks in general —
  `TestCaptureKeepsTrackedGitlink` and
  `TestNestedWorktree_ExternalWorktreeAndTrackedGitlinkUnaffected` pin
  that boundary.
- `PreflightReconcile` was left alone on purpose: it is a hygiene gate,
  not a capture surface, and a nested worktree surfacing there as an
  untracked path produces a refusal (fail-closed), which is the safe
  direction.
- `computePathSet` now returns `([]string, error)`; both call sites in
  `land.go` propagate.
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
