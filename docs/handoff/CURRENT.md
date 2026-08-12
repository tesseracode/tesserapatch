# Current Handoff

## Status

**Cluster state**: REV-7 DISPATCHED

v0.15.1 Wave A rev-6 isolates staging but leaves publish-failure lock
leakage and a crash-recovery gap. Rev-7 is dispatched.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-7
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record/reconcile capture and from
  land planning, staging and commit.
- **Status**: In Progress
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Rev-6 dispatch HEAD**: `3dfbad0`
- **Target release**: v0.15.1

## Rev-5 finding closures

The architecture changed as directed: land no longer touches the
operator's index while it works. `internal/gitutil/index_snapshot.go`
was rewritten around an `IndexTransaction`.

### F1 (HIGH) — effective index path + symlink — CLOSED

- `GIT_INDEX_FILE` is taken **verbatim** via `os.LookupEnv`; leading and
  trailing spaces and tabs are preserved. A relative value resolves
  against `repoRoot`, which is `cmd.Dir` for every git subprocess tpatch
  spawns — the same rule Git applies.
- Otherwise `git rev-parse --git-path index` is parsed by stripping only
  the protocol line terminator (`\n`, then a trailing `\r`). No
  `TrimSpace`. Output that still contains a newline is **ambiguous** and
  refused rather than guessed at.
- `requireRegularIndexFile` **Lstat**s the effective index before
  anything is staged and refuses a symlink or any non-regular file
  (directory, FIFO, …) with actionable guidance. The previous code
  followed the link and would have replaced it with a regular file,
  writing Git state wherever it pointed.

### F2 (HIGH) — blind rollback / concurrent operator staging — CLOSED

Replaced, not patched:

1. `BeginIndexTransaction` snapshots the live index (existence, exact
   bytes, mode) and seeds a private temp index byte-identically. An
   absent live index seeds an absent temp index, so the first `git add`
   creates it exactly as Git would.
2. Every `git add`, both staged-path audits and the commit run with
   `GIT_INDEX_FILE=<temp>` through new env-aware helpers
   (`stagePathSetEnv`, `runGitEnvOut`, `runGitCaptureEnv`,
   `StagedPathsEnv`, `AuditStagedPathsForNestedWorktreesEnv`). No global
   env mutation anywhere.
3. The live index is therefore untouched during staging, audits, status
   staging and hooks.
4. Because the temp index is seeded from the live one, all pre-existing
   operator-staged entries are present, so extras classification and
   commit contents keep exactly their previous meaning.

### F3 (MEDIUM) — status preimage restoration — CLOSED

`captureFileState`/`(*fileState).restore` capture existence, bytes and
mode and restore atomically. `restore` **returns** its error; land's
`abort` helper combines the primary cause with every restore/cleanup
failure into one diagnostic. Nothing is swallowed.

### Guarded commit and publication

- `LockLive` takes `<effective-index>.lock` with `O_CREAT|O_EXCL` — the
  same lock `git add` and `git commit` contend for. A pre-existing lock
  is somebody else's: land refuses and never removes it.
- `VerifyLiveUnchanged` re-compares the live index against the Begin
  snapshot while the lock is held. Divergence ⇒ refuse, restore the
  status preimage, delete the temp index, leave HEAD and the live index
  untouched.
- `git commit` runs with `GIT_INDEX_FILE=<temp>` **while the lock is
  held**, so hooks inherit the temp-index environment and normal hook
  semantics are preserved.
- On success the post-commit temp index is published through the held
  lock (bytes into the lock file, then rename onto the index — Git's own
  publish shape), so the live index matches the new HEAD.
- On commit failure the audited pre-commit temp index is published
  instead, preserving the existing contract that the intended paths stay
  staged for `land --no-record`. Hook output is surfaced verbatim, and a
  publish failure is reported explicitly alongside the commit failure.
- Temp dir and lock are cleaned on every path; `Close` is idempotent and
  reports its own failures.

### Honest scope

Documented in the source header, `docs/land.md` and the CHANGELOG: this
serializes **index** writes against other Git processes. It does **not**
protect against concurrent ref or working-tree mutation (`git checkout`,
`git reset --hard`, direct ref updates), which no index lock can
express. No broader claim is made anywhere.

## Files Changed

Modified this rev:

- `internal/gitutil/index_snapshot.go` (rewritten around
  `IndexTransaction`)
- `internal/gitutil/index_snapshot_test.go` (rewritten)
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
- 178 passing assertions across the GH #7 test set.

New coverage this rev:

- `TestEffectiveIndexPathRespectsWhitespaceBearingGitIndexFile` —
  absolute `" odd\tindex name "` and relative `"sub dir/idx "`; exact
  path, full stage → lock → publish round-trip, and no lock residue.
- `TestEffectiveIndexPathLinkedWorktree` — a linked worktree's own index
  is seeded from and published back to, and the live index does not move
  during temp staging.
- `TestBeginIndexTransactionRefusesSymlinkedIndex` — refusal before any
  mutation; the symlink topology and its target bytes both survive.
- `TestBeginIndexTransactionRefusesNonRegularIndex` — directory and
  FIFO.
- `TestIndexTransactionDetectsConcurrentLiveMutation` — a concurrent
  `git add` is detected at publish time; the operator's index is
  byte-identical, land's staging never reached it, no lock or temp
  residue.
- `TestIndexTransactionSeedsFromOperatorState` — operator-staged and
  intent-to-add entries seed the temp index (verified through
  `ls-files --cached`, since `diff --cached` deliberately ignores
  intent-to-add), and the temp bytes equal the live bytes.
- `TestIndexTransactionAbsentIndexLifecycle` — absent live index stays
  absent through staging and is created by publish.
- `TestIndexTransactionRefusesAndPreservesForeignLock`,
  `TestIndexTransactionCloseIsIdempotent`,
  `TestPublishLockedRequiresTheLock`.
- `TestStagedPathsEnvByteExact` — space/tab/newline names, and the live
  index sees none of it.
- `TestNestedWorktree_Land_ConcurrentOperatorAddIsDetectedNotOverwritten`
  — a `git` wrapper runs a real `env -u GIT_INDEX_FILE git add` against
  the LIVE index during land's first staging call; land refuses,
  HEAD is unchanged, both operator paths survive, land's paths are
  absent, no landed-at note, no lock residue.
- `TestNestedWorktree_Land_RefusesSymlinkedIndexBeforeMutation`,
  `TestNestedWorktree_Land_RefusesOnLiveIndexLockContention`,
  `TestNestedWorktree_Land_WhitespaceBearingGitIndexFile`,
  `TestNestedWorktree_Land_LeavesNoIndexResidue` (success output and
  trailers unchanged, live index clean vs new HEAD).
- `TestFileStateRestoreReportsFailures` — bytes and mode restored
  exactly, an unwritable directory produces an error rather than a
  silent failure, and an absent preimage restores to absent.

Retained and re-run unchanged: the four-position stage-time
fault-injection suite (now injecting into the temp index, since the
hook inherits `GIT_INDEX_FILE`), the commit-hook retry contract,
the discovery-budget tests, and every prior original-issue,
exotic-name, strict-parser, refresh and non-goal regression.
`TestNestedWorktree_Land_RollbackPreservesOperatorStagedState` was
renamed `..._AbortLeavesOperatorStagedStateUntouched` because there is
no longer anything to roll back.

## Reproduction + control matrix (built binary)

Re-run at this HEAD with nested worktrees (`agent review`,
`agent trail `) plus every over-filtering control: the worktrees are
absent from patch, diffstat, plan, index and commit; the prefix sibling
`agent-other/f.txt`, the unregistered nested repo `vendor/plainrepo` and
the external worktree behave exactly as before. `.git/index.lock` does
not exist after the run. All scratch repos, worktrees and build
artifacts were removed; `git worktree list` shows only the primary
worktree.

## Reviewer focus

1. Hook compatibility was the named blocker risk. It holds on this
   platform: `git commit` propagates `GIT_INDEX_FILE` to hooks, so a
   `pre-commit` hook sees the audited temp index, and the existing
   hook-failure retry test passes unchanged. No fallback to live-index
   restore was needed, so none was added.
2. `PublishLocked` writes into the lock file and renames it onto the
   index — Git's own publish shape — which means the lock is consumed by
   a successful publish. `Close` afterwards only cleans the temp dir.
3. The commit runs *while* the lock is held. Git locks `<temp>.lock`
   for its own index writes, so there is no self-deadlock; verified by
   the whole land suite.
4. `VerifyLiveUnchanged` compares bytes and mode. It does not compare
   mtime, so a touch-without-change is correctly not treated as
   divergence.
5. Scope wording: please check `docs/land.md`, the CHANGELOG and the
   `index_snapshot.go` header all say the same thing — index
   serialization only, no claim about refs or the working tree.

## Rev-6 Review Adjudication

- Internal: NEEDS REVISION.
- External/original reproducer: APPROVED.
- Isolated staging and operator-divergence protection are effective.
- Valid residuals:
  1. Publish clears lock ownership too early; later chmod/rename failure can
     leave a stale owned index lock.
  2. Commit advances HEAD before durable live-index publication; a crash can
     leave HEAD/index/lock inconsistent.
- `tpatch_rev6_bin` and review scratch are absent after external cleanup.

## Next Steps

1. Retain owned-lock cleanup through all publish outcomes.
2. Make live-index publication atomic and fsync-durable.
3. Add a durable land-index recovery journal for commit/publish crash windows.
4. Recover or refuse safely at the next land invocation.
5. Run final dual review, then close #7 only after approval.

## Blockers

None. The lock/alternate-index strategy proved compatible with Git
hooks on the supported platform, so the dispatch's "stop and report"
branch was not taken.

## Context for Next Agent

- `internal/gitutil/worktrees.go` is the single discovery authority;
  `git worktree list --porcelain -z` is the single Git shape (Git 2.36+).
- `internal/gitutil/index_snapshot.go` is the single index-transaction
  authority. Any future code that stages on the operator's behalf should
  use `BeginIndexTransaction` rather than writing to the live index.
- `FilesInPatchStrict` is mandatory for any NEW code that derives a
  write scope, a diff scope or a staging decision from a patch. Decode
  Git quoting with `unquoteGitCStyle`, never `strconv.Unquote`.
- Land's contract: gate-discover → revalidate → begin transaction →
  stage temp → audit → status → audit → lock + verify → commit temp →
  publish → release.
- Byte exactness remains load-bearing: no `TrimSpace`, no hand-rolled
  dequote on any path compared against a worktree prefix, and no
  trimming of `GIT_INDEX_FILE`.
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
