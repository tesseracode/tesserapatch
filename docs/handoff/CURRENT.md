# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave A rev-8 (GitHub issue #7) closes all three rev-7 findings:
recovery now runs under the live lock with byte-level state comparison,
the journal is only cleared after a successful durable publish, and the
retained index IS the alternate index land stages and commits against.
Validated and pushed. Awaiting review.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-8
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record/reconcile capture and from
  land planning, staging and commit — with a durable, crash-recoverable
  land transaction.
- **Status**: Review
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Rev-8 dispatch HEAD**: `7905cc9`
- **Target release**: v0.15.1

## Rev-7 finding closures

### F1 (HIGH) — recovery under the live lock + state compare — CLOSED

Recovery previously published without locking or comparing the live
index, so an operator `git add` after the crash could be silently
overwritten.

- Recovery now takes the live `<index>.lock` (`O_EXCL`) before it
  reads, compares or publishes anything. A stale lock is removed only
  after **both** the nonce and, where the platform exposes one, the
  recorded **inode** match the journal; a foreign lock refuses outright
  and is left byte-identical.
- The journal carries the canonical **preimage** identity —
  `LiveIndexPre{Exists, SHA256, Mode}` — plus the index path identity,
  and the retained index's identity and `write-tree`.
- Under the lock the live index is classified by CONTENT, never by path
  match: `livePreimage` (safe to publish), `livePostimage` (a previous
  recovery already published it → idempotent cleanup, **no rewrite**),
  or `liveDivergent` (→ refuse, preserving the live index, journal and
  retained index; only our own lock is released).
- Both HEAD branches go through the same comparison, and recovery
  remains evidence-based and idempotent.

### F2 (HIGH) — journal clearing — CLOSED

The journal and retained index are the **only** copy of the staged-retry
evidence, so they are cleared strictly after a durable publish and lock
cleanup succeed:

- commit failure + publish failure → journal retained, diagnostic
  `land: commit failed, staged retry recovery pending — …`;
- commit success + publish failure → journal retained, diagnostic
  `land: commit succeeded, recovery pending — …`, which explicitly never
  claims a rollback of HEAD;
- a later `tpatch land` recovers both cases (HEAD==pre publishes the
  staged retry; HEAD advanced finishes the publication) and only then
  clears.

An abort *before* the journal exists still removes the scratch alternate
index; after it exists, nothing on a failure path deletes it.

### F3 (MEDIUM) — retained index is the commit index — CLOSED

`gitutil.BeginIndexTransactionAt` lets land point `GIT_INDEX_FILE` at
`.tpatch/local/land-journal/<slug>.index` directly. Staging, both
pre-commit audits, the commit and every hook mutate that one durable
file, so a hook's edits are inside the evidence.

- Before the commit: `write-tree` (which rewrites the index in place),
  then fsync, then hash — in that order, because hashing first records a
  checksum the file no longer has. This ordering bug was found and fixed
  during implementation and is now stated in `indexTree`'s doc comment.
- After `git commit` returns (success **or** failure): the alternate
  index is re-audited for nested worktrees, fsynced, re-hashed, its tree
  recomputed, and the journal durably updated — all **before** any live
  publish or clear.
- A hook that stages a nested worktree is caught by that re-audit on
  both branches and refuses with the evidence preserved.

### F4 — evidence matrix (dispatch item 4)

Stated in the `landJournal` doc comment, in `docs/land.md`, in the
CHANGELOG and exercised by tests. A retained checksum that differs from
the pre-commit value is accepted **only** in the HEAD-advanced branch
and **only** when path containment holds and the retained `write-tree`
equals the direct-child HEAD tree; the observed identity is then
persisted under the lock. `Phase` is written and refreshed but no
decision reads it.

## Files Changed

Created this rev:

- `internal/gitutil/file_ino_unix.go`
- `internal/gitutil/file_ino_other.go`

Modified this rev:

- `internal/gitutil/index_snapshot.go`
- `internal/cli/land_journal.go`
- `internal/cli/land_journal_test.go`
- `internal/cli/land.go`
- `internal/cli/nested_worktree_test.go`
- `CHANGELOG.md`
- `docs/land.md`
- `docs/handoff/CURRENT.md`

Deliberately NOT folded: the Makefile nested-repo sentinel LOW is
separately tracked.

## Test Results

- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean; `GOOS=windows go build ./...` — clean
  (the inode lookup is behind a build tag; Windows degrades to the nonce
  check, which is the decisive one).
- `go test -count=1 ./...` — all packages `ok`, including the
  cross-build contract test that caught the original `syscall.Stat_t`
  portability break.
- `go test -race -count=1` on `./internal/gitutil/`, `./internal/workflow/`
  and `./internal/cli/` — ok.
- 238 passing assertions across the GH #7 test set.

New coverage this rev:

- `TestRecoverLandRefusesDivergentLiveIndex` — an operator `git add`
  after the crash makes the live index divergent; recovery refuses, the
  operator's bytes AND mode are preserved, their staged file survives,
  the journal/retained index are kept and no lock is left. Restoring the
  exact preimage then lets recovery succeed.
- `TestRecoverLandPostimageCleansWithoutRewrite` — an already-published
  index is recognised by content; recovery cleans up without rewriting
  (asserted via unchanged mtime) and is idempotent.
- `TestRecoverLandLockValidation` — an inode mismatch on an otherwise
  ours-looking lock refuses and preserves it; a foreign lock blocks
  recovery and is untouched.
- `TestLandHookStagedFileIsCommittedAndJournalled` — a `pre-commit` hook
  that stages an allowed extra file: the commit contains it, the
  published index agrees with HEAD, no journal residue.
- `TestLandHookNestedWorktreeContaminationRefuses` — a hook that stages
  the nested worktree is caught by the post-commit re-audit; HEAD does
  not advance and the contamination never reaches the live index.
- `TestLandCommitFailurePublishFailureRetainsEvidenceThenRecovers` —
  injected live-publish failure on the commit-failure path: the exact
  `commit failed, staged retry recovery pending` diagnostic, evidence
  retained, no lock residue, and a later recovery publishes the staged
  retry and clears.
- `TestLandPostCommitPublishFailureIsRecoveryPending` — injected
  live-publish failure after a successful commit: the exact
  `commit succeeded, recovery pending` diagnostic, no "rolled back"
  wording, HEAD advanced, evidence retained, and a later recovery
  finishes the publication and clears (idempotent on a second pass).

Updated: the publish failure seams now receive the **target** path, so a
test can fail only the live publication and leave alternate-index
seeding alone. Discovery budgets moved to `5 + recordCalls` (entry gate,
pre-stage revalidation, post-stage audit, pre-commit audit, post-commit
re-audit); `--no-record` spends 5, `--dry-run` 1.

All prior GH #7, strict-parser, refresh, non-goal and land-hook
regressions were re-run unchanged and pass.

## Reproduction + control matrix (built binary)

Re-run at this HEAD: the nested worktree is absent from patch, diffstat,
plan, index and commit; the prefix sibling is captured and landed. After
land there is no `.git/index.lock`, the journal directory is empty, and
`git diff --cached` is empty. Scratch repos, worktrees and binaries
removed.

## Reviewer focus

1. **`write-tree` rewrites the index.** This is the subtlest part of the
   rev: any hash taken before `write-tree` is stale. Production computes
   tree → fsync → hash in `writeLandJournal` and
   `refreshLandJournalAfterCommit`, and in recovery the tree is computed
   only inside the HEAD-advanced branch with the identity re-taken
   afterwards. Worth a careful read.
2. **Divergence is strict.** `git reset` produces a semantically
   equivalent but not byte-identical index, so it is classified
   divergent and refuses. That is deliberate — it is exactly the "the
   operator touched it" signal — but it means a well-meaning `git reset`
   forces manual recovery. The refusal explains how.
3. **Recovery does not open an `IndexTransaction`** (that would re-seed
   the alternate index from the live one and destroy the evidence); it
   uses a minimal `recoveryLock`. Confirm the duplication is warranted.
4. **Windows inode degradation.** `FileIno` returns `ok=false` there, so
   stale-lock identification rests on the nonce alone. The nonce is the
   decisive check; the inode is a strengthening.
5. **Hook environment.** Hooks inherit `GIT_INDEX_FILE` pointing at the
   retained index, so `git add` inside a hook mutates the evidence — by
   design, and covered both for an allowed file and for a nested
   worktree.

## Blockers

None. A safe durable protocol was provable in the current architecture.

## Context for Next Agent

- `internal/gitutil/worktrees.go` — discovery authority.
- `internal/gitutil/index_snapshot.go` — index transaction, owned-lock
  lifetime, durable publish, target-aware failure seams.
- `internal/cli/land_journal.go` — journal schema (v2), the evidence
  matrix, locked recovery. Any schema change must bump
  `landJournalVersion`; recovery refuses unknown versions.
- Land's contract: recover (under lock) → gate-discover → revalidate →
  begin transaction at the retained index → stage → audit → status →
  audit → lock + verify → journal → commit → re-audit + refresh journal
  → durable publish → clear journal → release.
- Never hash an index before `write-tree`.
- Never clear the journal before a publish has succeeded.
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
