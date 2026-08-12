# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave A rev-7 (GitHub issue #7) closes the owned-lock-lifetime
and crash-durability findings. Validated and pushed. Awaiting review.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-7
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record/reconcile capture and from
  land planning, staging and commit — with a durable, crash-recoverable
  land transaction.
- **Status**: Review
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Rev-7 dispatch HEAD**: `150da09`
- **Target release**: v0.15.1

## Rev-6 finding closures

### F1 (HIGH) — owned lock lifetime — CLOSED

`IndexTransaction` now tracks `lockOwned bool` **independently** of the
open `*os.File`. Closing the descriptor no longer relinquishes cleanup
responsibility, which was the exact defect: rev-6 nil'd `lockFile` on
close, so a later chmod/rename failure left an owned `<index>.lock`
behind and wedged the repository.

- `releaseOwnedLock` closes the descriptor if open, removes the lock
  **only when `lockOwned`**, fsyncs the parent directory, then clears
  ownership.
- `PublishLocked` routes every failure through `joinRelease`, which
  releases the owned lock and folds any release failure into the primary
  error. Cleanup failures are never silent.
- A foreign lock is still never removed — `LockLive` refuses to acquire
  it, and recovery only removes a lock carrying this run's nonce.

### F2 (MEDIUM) — durable publish + crash recovery — CLOSED

**Durable publish.** The lock is now purely a **mutex sentinel**; it is
never renamed onto the index as the data file. `durableWriteFile`
creates an `O_EXCL` temp in the live index's own symlink-resolved
directory, writes, `Sync`s the file, `Close`s, `Chmod`s, `Rename`s onto
the index and `Sync`s the parent directory. A reader therefore always
observes the complete old index or the complete new one. Six failure
seams (`write`, `fsync`, `close`, `chmod`, `rename`, `dir fsync`) are
injectable and each is tested.

**Crash-recovery journal.** Before `git commit` can advance HEAD, land
persists, with fsyncs:

- `.tpatch/local/land-journal/<slug>.json` — versioned, owner-only
  (0600), recording slug, `created_at`, advisory phase, pre-HEAD,
  live-index identity (existence / SHA-256 / mode), retained-index
  reference + identity, and the owned lock's path + nonce.
- `.tpatch/local/land-journal/<slug>.index` — the retained alternate
  index bytes.

`.tpatch/local/` is the existing gitignored local root, so nothing leaks
into `git status`. The journal stores **repo-relative** references; an
absolute path appears only when `GIT_INDEX_FILE` genuinely points
outside the repository. No source content, no secrets, no home paths.

**Recovery** runs at the very top of `runLand`, before the pre-record
gate and therefore before any record/status/staging mutation. It
validates schema version, slug, required fields, retained-path
containment inside the journal directory, retained-index checksum and
effective-index identity — then decides from **evidence, not phase**:

| Evidence | Action |
|----------|--------|
| `HEAD == pre_head` | The commit never happened. Publish the retained audited index — exactly the staged-retry contract — and keep the landed-at note consistent with it. |
| HEAD advanced, is a direct child of `pre_head`, carries `Tpatch-Feature: <slug>`, and the retained index's `write-tree` equals HEAD's tree | The commit completed; only publication was outstanding. Publish and clean. |
| anything else | Refuse with manual-recovery guidance. Journal and retained index are preserved; nothing is guessed at or overwritten. |

The "crash between HEAD advance and phase update" case is handled by
this evidence, which is why the phase field is explicitly advisory.
Recovery is idempotent: a successful pass clears the journal, so the
next land finds nothing.

**Commit flow.** Journal written before commit → commit against the
alternate index under the live lock → phase updated (best effort) →
durable publish → journal cleared → lock released. On ordinary commit
failure the audited pre-commit index is published for the `--no-record`
retry and the journal is cleared. If publish fails **after** a
successful commit, land returns a precise "commit succeeded, recovery
pending" diagnostic, **keeps** the journal for the next invocation, and
never claims HEAD was rolled back.

### Status errors (dispatch item 5)

`captureFileState`/`restore` snapshot existence, bytes and mode and
restore atomically; `restore` returns its error and land's `abort`
aggregates it with the primary cause. On the commit-failure path the
landed-at note is deliberately retained, consistent with the published
staged-retry index — asserted by
`TestLandCommitFailureClearsJournalAndKeepsStagedRetry`.

### Index path / topology (dispatch item 6)

`GIT_INDEX_FILE` is still taken verbatim (whitespace preserved); the
index file itself is `Lstat`-refused when it is a symlink or otherwise
non-regular. Rather than *also* rejecting a symlinked parent — which
would break legitimate symlinked `.git` setups — the publish temp is
created in the **symlink-resolved** parent directory, so the final
rename is same-directory and same-filesystem and cannot be redirected.
That trade-off is called out for review below.

## Files Changed

Created this rev:

- `internal/cli/land_journal.go`
- `internal/cli/land_journal_test.go`
- `internal/gitutil/index_publish_test.go`

Modified this rev:

- `internal/gitutil/index_snapshot.go`
- `internal/cli/land.go`
- `CHANGELOG.md`
- `docs/land.md`
- `docs/handoff/CURRENT.md`

Unchanged this rev: `internal/gitutil/worktrees.go`,
`internal/gitutil/patch_paths_strict.go`,
`internal/gitutil/capture_modes.go`, `internal/gitutil/gitutil.go`,
`internal/cli/cobra.go`, `internal/cli/nested_worktree_guard.go`,
`internal/cli/nested_worktree_test.go`, `internal/cli/phase2.go`,
`internal/workflow/refresh.go`, `SPEC.md`, `docs/record.md`.

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
- 204 passing assertions across the GH #7 test set.

New coverage this rev:

- `TestPublishFailureSeamsCleanOwnedLockAndNeverTruncate` — all six
  seams. Each asserts: an error is surfaced, **no owned lock survives**,
  the live index is the complete old OR complete new file (never a
  truncation), Git can still read it, and no publish-temp residue
  remains.
- `TestPublishDoesNotConsumeTheLockAsData` — the lock holds our nonce
  sentinel, the published index equals the temp index, and the lock is
  gone afterwards.
- `TestOwnedLockCleanupNeverTouchesAForeignLock` — `Close` leaves a
  foreign lock byte-identical and `LockNonceAt` reports it as not ours.
- `TestPublishAbsentIndexReleasesLock`.
- `TestRecoverLandNoJournalIsNoOp`.
- `TestRecoverLandCrashBeforeCommitPublishesStagedRetry` — retained
  index published, staged retry present, HEAD unchanged, journal +
  retained index + our stale lock all cleaned, and a second pass is a
  no-op.
- `TestRecoverLandCrashAfterHeadAdvanceBeforePhaseUpdate` — journal
  still says `pre-commit` (asserted), HEAD advanced via a constructed
  landing commit; recovery publishes the **committed** index, the
  published index agrees with the new HEAD, everything is cleaned, and
  a second pass is a no-op.
- `TestRecoverLandRefusesUnrelatedHeadAdvance` — refusal names the
  journal directory and the `git log --grep` recovery step; HEAD, the
  live index, the journal and the retained index are all untouched.
- `TestRecoverLandValidatesJournal` — unsupported version, retained-path
  containment escape (`../../etc/passwd`), checksum mismatch, changed
  effective-index identity, malformed JSON.
- `TestRecoverLandLockOwnership` — our stale lock removed; a foreign
  lock preserved AND reported.
- `TestLandRunsRecoveryBeforeAnyMutation` — an unrecoverable journal
  makes `land` refuse with the feature directory byte-identical and HEAD
  unmoved.
- `TestLandLeavesNoJournalResidue` — clean land leaves no journal,
  retained index or lock, and nothing appears in `git status`.
- `TestLandCommitFailureClearsJournalAndKeepsStagedRetry` — journal
  cleared, audited staged retry present, landed-at note retained,
  `--no-record` retry lands.

Crash phases are exercised by **constructing** the durable state a crash
would leave and invoking recovery, rather than killing processes: that
is deterministic, leaves no orphans to clean, and tests exactly the
evidence recovery reasons about.

All prior GH #7, strict-parser, refresh, non-goal, land-hook and
discovery-budget regressions were re-run unchanged and pass.

## Reproduction + control matrix (built binary)

Re-run at this HEAD: nested worktrees (`agent review`, `agent trail `)
absent from patch, diffstat, plan, index and commit; prefix sibling,
unregistered nested repo and external worktree unchanged. After land:
no `.git/index.lock`, an empty `.tpatch/local/land-journal/`, and
`git diff --cached` empty (index agrees with the new HEAD). All scratch
repos, worktrees and binaries removed.

## Reviewer focus

1. **Symlinked parent components.** The dispatch asked to reject them.
   Rejecting would break legitimate symlinked `.git` setups, so instead
   the publish temp is created in the resolved parent, making the rename
   same-directory and non-redirectable. Please confirm that reading of
   the requirement; switching to outright rejection is a one-line change
   if preferred.
2. **Split index.** `core.splitIndex` writes a shared index in
   `$GIT_DIR` that the retained index may reference. We retain the index
   file's bytes, and recovery validates by SHA-256 plus a `write-tree`
   tree comparison, so a dangling shared reference surfaces as a
   refusal rather than a wrong publish. No dedicated fixture was added
   because the tree comparison is the real guard.
3. **Journal phase is advisory.** It is written and updated, but no
   decision reads it. That is deliberate per the dispatch; the field is
   kept for human forensics.
4. Recovery publishes with `durableWriteOwnerFile` using the journal's
   recorded live-index mode. If the journal has no mode (an absent live
   index at Begin), it falls back to 0644 — matching Git.
5. The commit-failure path clears the journal because the audited index
   *was* published; the post-commit publish-failure path keeps it. Those
   are the only two asymmetric branches — worth a careful read.

## Blockers

None. A safe durable protocol was provable within the current
architecture, so the dispatch's "stop and report" branch was not taken.

## Context for Next Agent

- `internal/gitutil/worktrees.go` — discovery authority
  (`git worktree list --porcelain -z`, Git 2.36+).
- `internal/gitutil/index_snapshot.go` — index transaction, owned-lock
  lifetime and durable publish.
- `internal/cli/land_journal.go` — journal schema, durable writes and
  evidence-based recovery. Any change to the schema must bump
  `landJournalVersion`; recovery refuses unknown versions.
- `FilesInPatchStrict` is mandatory for any NEW code deriving a write
  scope from a patch; decode Git quoting with `unquoteGitCStyle`.
- Land's contract: recover → gate-discover → revalidate → begin
  transaction → stage temp → audit → status → audit → lock + verify →
  journal → commit temp → durable publish → clear journal → release.
- Byte exactness remains load-bearing: no `TrimSpace` on paths or on
  `GIT_INDEX_FILE`.
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
