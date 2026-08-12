# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave A rev-5 (GitHub issue #7) closes the rev-4 HIGH race with a
land staging transaction. Validated and pushed. Awaiting review.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-5
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record/reconcile capture and land
  planning, staging and commit.
- **Status**: Review
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Rev-5 dispatch HEAD**: `159050f`
- **Target release**: v0.15.1

## Rev-4 finding closure

### HIGH — worktree registers between revalidation and staging — CLOSED

The remaining window is *inside* the staging step, so no amount of
pre-stage planning can close it. `land` now wraps staging in a
transaction.

**New primitives** — `internal/gitutil/index_snapshot.go`, all explicit
Go helpers, no shell:

- `EffectiveIndexPath` — `git rev-parse --git-path index`, absolutised
  against `repoRoot`. Verified to follow all three shapes: the main
  worktree (`.git/index`), a linked worktree
  (`.git/worktrees/<name>/index`), and a redirected `GIT_INDEX_FILE`.
- `SnapshotIndex` — captures existence, exact bytes and file mode.
  Reading does not disturb the index, so the operator's staged state is
  untouched by the snapshot. An absent index is a valid snapshot.
- `(*IndexSnapshot).Restore` — atomic (temp file in the same directory,
  then rename) with the original mode; removes the file when the
  snapshot was "absent". Idempotent.
- `StagedPaths` — `git diff --cached --name-only -z`, byte-exact and
  never quoted, with an `ls-files --cached -z` fallback for a repository
  with no HEAD.
- `AuditStagedPathsForNestedWorktrees` — rediscovers, then returns every
  staged path inside a currently registered nested worktree. Discovery
  failure returns the fail-closed class so the caller rolls back rather
  than committing an unaudited index.

**Land sequence** (`internal/cli/land.go`):

1. Snapshot the effective index.
2. Stage the path set **minus** `status.json`. `status.json` stays in
   the path set for extras classification (so it is never mis-classified
   as an extra) but is held back from this pass.
3. Audit: rediscover + inspect the index. On contamination or discovery
   failure, restore the exact pre-land index and refuse — `HEAD`,
   `status.json` and the landed-at note all untouched.
4. Only then write the landed-at note and stage exactly `status.json`.
   A failure here restores the status preimage as well as the index, so
   no false `landed at` note can survive.
5. Audit once more, immediately before the commit, because the
   status-staging pass is itself a (narrower) window. The commit is
   therefore always taken from an index verified clean after land's
   LAST `git add`.
6. Commit. Past step 5 land performs no staging at all.

**Closure statement.** A worktree registered after the final audit
cannot enter the index by registration alone — registration stages
nothing, and land issues no further `git add`. A concurrent
third-party `git add` racing the commit remains outside supported
semantics, and is documented as such in `docs/land.md` and the
CHANGELOG rather than silently assumed.

**Boundaries preserved.** Embedded record artifacts still persist across
a land refusal (record's own completed transaction, unchanged from
rev-4). The commit-hook contract is unchanged: once the audit has passed
and the commit is attempted, a failing hook intentionally leaves the
audited index staged for a `--no-record` retry.

**Honest note on reachability.** Because land stages explicit file
paths, a bare stage-time registration does not by itself put the
worktree in the index — the audit is a backstop, not a routine catch.
The tests therefore *inject* the contaminated-index state
deterministically (the hook registers the worktree AND stages its
gitlink) so the backstop is proven to work rather than assumed. That
distinction is written into the test's own comment; assuming `git add`
will never widen is what rev-4 did.

## Files Changed

Created this rev:

- `internal/gitutil/index_snapshot.go`
- `internal/gitutil/index_snapshot_test.go`

Modified this rev:

- `internal/cli/land.go`
- `internal/cli/nested_worktree_test.go`
- `CHANGELOG.md`
- `docs/land.md`
- `docs/handoff/CURRENT.md`

Unchanged this rev: `internal/gitutil/worktrees.go`,
`internal/gitutil/patch_paths_strict.go`,
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
- 162 passing assertions across the GH #7 test set.

New coverage this rev:

- `TestEffectiveIndexPathMainWorktree` / `...LinkedWorktree` /
  `...RespectsGitIndexFile` — all three index shapes, including a
  snapshot/restore round-trip against a linked worktree's own index.
- `TestSnapshotIndexAbsentIndexRoundTrip` — a never-staged repository
  has no index; restore removes the one land created, and a second
  restore is a no-op.
- `TestSnapshotIndexPreservesOperatorStagedState` — operator-staged
  content plus an intent-to-add entry survive a rollback with identical
  index bytes, identical `write-tree`, identical `status -z` and the
  original file mode.
- `TestStagedPathsByteExact` — space, tab and newline names survive.
- `TestAuditStagedPathsForNestedWorktrees` — flags a staged gitlink,
  flags nothing when no worktree is nested.
- `TestAuditStagedPathsFailsClosedOnDiscoveryFailure`.
- `TestNestedWorktree_Land_StageTimeRegistrationIsAuditedAndRolledBack`
  — fault injection at FOUR positions in the `git add` sequence (first
  staging add, first staging add under a scoped land, the
  status-staging add, and the very last add). Every one is caught:
  land refuses, `HEAD` is unchanged, `write-tree` matches the pre-land
  index, the cached diff no longer mentions the worktree, no landed-at
  note was written, and a rerun after removing the worktree succeeds.
- `TestNestedWorktree_Land_RollbackPreservesOperatorStagedState` — the
  post-stage audit's discovery is failed at the calibrated call; the
  index is restored byte-for-byte, the operator's staged file is still
  staged, land's staging is gone, `HEAD` is unchanged and no landed-at
  note was written.
- `TestNestedWorktree_Land_CommitHookFailureLeavesAuditedIndexStaged` —
  a rejecting `pre-commit` hook leaves `README.md`,
  `internal/example.go` and `status.json` staged (and no worktree), and
  the `--no-record` retry lands.
- `TestNestedWorktree_Land_CleanSuccessUnchangedByTransaction` — same
  paths land, index empty afterwards, tree clean, landed-at note
  present.

Updated to the rev-5 contract:

- `TestNestedWorktree_Land_DiscoveryBudget` — budget is now
  `4 + recordCalls` (entry gate, pre-stage revalidation, post-stage
  audit, final pre-commit audit), still calibrated against a standalone
  `record` rather than hardcoded.
- `TestNestedWorktree_Land_NoRecordAndDryRunDiscoveryBudget` —
  `--no-record` spends 4; `--dry-run` stages nothing and spends 1.

All prior original-issue, exotic-name, non-goal, refresh, strict-parse
and transaction suites were re-run unchanged and pass.

## Reproduction + control matrix (built binary)

Re-run at this HEAD with nested worktrees (`agent review`,
`agent trail `) plus every over-filtering control:

| Path | Kind | Result |
|------|------|--------|
| `.claude/worktrees/agent review` | nested worktree | absent from patch, diffstat, plan, index, commit |
| `.claude/worktrees/agent trail ` | nested worktree, trailing space | absent from all five |
| `.claude/worktrees/agent-other/f.txt` | ordinary dir, prefix sibling | captured and landed |
| `vendor/plainrepo` | unregistered nested Git repo | captured and landed as a gitlink (correctly NOT filtered) |
| `../extwt` | worktree outside the root | never referenced |

Post-land `git status` lists only the two worktrees as untracked plus
the carved-out `.tpatch/FEATURES.md`. All scratch repos, worktrees and
build artifacts were removed; `git worktree list` shows only the primary
worktree.

## Reviewer focus

1. The rollback restores the **index file**, not the working tree. A
   file land created on disk before staging (none today — land only
   stages existing paths) would remain; the tests assert `write-tree`
   and `status -z` equivalence after removing such a file explicitly, so
   the distinction is visible rather than hidden.
2. Land's discovery budget grew to `4 + recordCalls`. Two of those are
   index audits, which each also run `git diff --cached`. If the extra
   subprocess cost matters for very large repos, the two audits could be
   collapsed by staging `status.json` in the same pass — at the cost of
   reopening the narrow status-pass window that the fourth injection
   point proves is real.
3. `Restore` deliberately does not remove a stale `index.lock`.
   Removing a lock we did not create would be unsafe; `git add` removes
   its own.
4. The fault-injection framing (hook stages the gitlink itself) is
   documented in the test. If a reviewer prefers a naturally-occurring
   reproduction, note that it would require land to stage a directory
   path, which it does not currently do — the audit exists precisely so
   that property does not have to be assumed.
5. `status.json` is held out of the first staging pass but stays in the
   path set for extras classification. Confirm that split reads clearly
   in `land.go`.

## Blockers

None.

## Context for Next Agent

- `internal/gitutil/worktrees.go` is the single discovery authority;
  `git worktree list --porcelain -z` is the single Git shape (Git 2.36+).
- `internal/gitutil/index_snapshot.go` is the single index
  snapshot/restore/audit authority. Any future code that stages on the
  operator's behalf should snapshot first and audit after.
- `FilesInPatchStrict` is mandatory for any NEW code that derives a
  write scope, a diff scope or a staging decision from a patch. Decode
  Git quoting with `unquoteGitCStyle`, never `strconv.Unquote`.
- Land's contract: gate-discover before `record`, revalidate before
  planning, snapshot → stage → audit → status → audit → commit.
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
