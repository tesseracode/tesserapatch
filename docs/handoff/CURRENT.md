# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave A / GH #7 rev-9 is implemented, validated and pushed. Both
rev-8 HIGH findings (successful-hook contamination, journal-controlled
lock paths) are closed, and the Windows test-portability finding is
closed by build-tag separation rather than a runtime skip.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-9
- **Milestone**: v0.15.1 — nested linked worktrees are never captured or staged
- **Description**: Close the rev-8 adjudication findings: audit the exact
  hook-mutated index that produced a *successful* commit and roll the
  branch back on contamination; remove every caller-controlled filesystem
  path from the crash-recovery journal; make the FIFO test compile on
  Windows.
- **Status**: Review
- **WAVE_BASE**: `5d15fcf`

## Session Summary

### F1 (HIGH) — a successful commit hook could still land a nested worktree

`git commit` passes `GIT_INDEX_FILE` to hooks, so a `pre-commit` hook
stages into the retained index. Rev-8 audited that index after the commit
with `git diff --cached`, which is the wrong instrument: once the commit
exists, `--cached` compares the index against the **new** HEAD, so a
gitlink the hook staged and the commit captured is invisible. A hook that
ran `git add .claude/worktrees/agent` and exited 0 therefore produced a
landed commit containing a `mode 160000` entry.

- Added `gitutil.IndexEntriesEnv` (`git ls-files --cached -z`) and
  `gitutil.AuditIndexEntriesForNestedWorktreesEnv`, which list the index
  outright rather than diffing it against a moving reference. Both
  post-commit audits (success and failure) now use them; the pre-commit
  audits keep `--cached` semantics, which are correct before the commit.
- On contamination `land` publishes nothing. It compare-and-swaps the
  branch back to the pre-land commit
  (`git update-ref -m <reflog> HEAD <preHead> <newHead>`), restores the
  `status.json` preimage and leaves the live index untouched, then
  returns the nested-worktree refusal naming the paths. The commit object
  is orphaned and never presented as landed.
- The swap is gated on HEAD still being **exactly** the landing commit
  (direct child of pre-HEAD carrying this feature's trailer). If a
  `post-commit` hook or another process advanced the branch, `land`
  refuses with manual guidance rather than discarding that work — a
  blind CAS would have thrown away a concurrent commit.
- Rollback / status-restore / cleanup failures route to
  `contaminatedRollbackPending`, which keeps the journal and retained
  index and returns a precise recovery-pending diagnostic.
- Recovery refuses to publish a retained index that contains a nested
  worktree, so the contaminated evidence cannot be published by a later
  invocation either.

### F2 (HIGH) — the journal directed filesystem operations

Schema v2 persisted `lock_abs` / `lock_rel`, and stale-lock removal used
them. A tampered journal could therefore have aimed an unlink at an
arbitrary file. Schema is now **v3**:

- `LockAbs` / `LockRel` removed from the struct and from every decision.
  The lock path is always derived at use time as
  `EffectiveIndexPath(repoRoot) + ".lock"`, after index-identity
  validation. The journal keeps `LockNonce` and `LockIno` purely as
  ownership evidence.
- Journals decode with `DisallowUnknownFields`, so a reintroduced
  `lock_abs` (or any other injected field) is refused, not ignored.
- `validateContainedRelPath` checks every remaining relative path for
  repository containment, regular-file type and the absence of symlinked
  components before it is read or written.
- A v2-or-older journal refuses with manual-recovery guidance instead of
  being interpreted.

### F3 (LOW) — Windows test portability

The FIFO helper referenced `syscall.Mkfifo`, which does not exist on
Windows, so `internal/gitutil` did not test-compile there — a runtime
skip could not have fixed an unresolved symbol. Split into
`fifo_unix_test.go` (`//go:build !windows`) and `fifo_other_test.go`
(`//go:build windows`, returns "unsupported"). Native FIFO coverage is
unchanged on Unix.

## Files Changed

**rev-9 (this cycle)**

- `internal/gitutil/index_snapshot.go` — `IndexEntriesEnv`,
  `AuditIndexEntriesForNestedWorktreesEnv`
- `internal/gitutil/index_snapshot_test.go` — FIFO helper call site
- `internal/gitutil/fifo_unix_test.go` (new) — `//go:build !windows`
- `internal/gitutil/fifo_other_test.go` (new) — `//go:build windows`
- `internal/cli/land.go` — post-commit index-entry audit, CAS rollback,
  `contaminatedRollbackPending`, landing-commit gate
- `internal/cli/land_journal.go` — schema v3, derived lock path, strict
  decode, `validateContainedRelPath`, contaminated-retained refusal
- `internal/cli/land_journal_test.go` — rev-9 tests
- `CHANGELOG.md`, `docs/land.md`, `docs/handoff/CURRENT.md`

## Test Results

```
gofmt -l .                                    (clean)
go vet ./...                                  ok
GOOS=windows GOARCH=amd64 go vet ./...        ok
GOOS=linux  GOARCH=amd64 go build ./...       ok
GOOS=windows GOARCH=amd64 go test -c          ok  (gitutil, cli)
GOOS=linux   GOARCH=amd64 go test -c          ok  (cli)
go build ./cmd/tpatch                         ok
go test -count=1 ./...                        ok  (all 13 packages)
go test -race -count=1 ./internal/gitutil/ ./internal/workflow/ ./internal/cli/
                                              ok
```

New rev-9 tests, all passing:

- `TestLandSuccessfulHookContaminationRollsBackHead` — a real
  `pre-commit` hook stages a registered nested worktree and exits 0;
  asserts nonzero exit, HEAD exactly pre-HEAD, live index unchanged, no
  binding commit on the branch, journal cleared, no landed-at note.
- `TestLandContaminatedRollbackFailureRetainsEvidence` — a `post-commit`
  hook advances the branch via `commit-tree`, defeating the CAS gate;
  asserts the manual-recovery diagnostic, retained journal + index, and
  a refusing, non-mutating recovery.
- `TestRecoverLandRefusesContaminatedRetainedIndex` — recovery refuses to
  publish a contaminated retained index even when HEAD is exactly
  pre-HEAD and every other check passes.
- `TestLandBenignMessageHooksStillLand` — `prepare-commit-msg` /
  `commit-msg` controls, and a hook staging an *allowed* file, land
  normally.
- `TestRecoverLandRejectsJournalSuppliedLockPaths` — a journal carrying
  `lock_abs` / `lock_rel` aimed at a victim file is refused; the victim
  is untouched.
- `TestRecoverLandRefusesPreviousSchemaVersion` — v2 journal refuses.
- `TestRecoverLandRefusesSymlinkedRetainedPath` — symlinked retained
  path refused before use.

## Reproduction

Built binary against a scratch repo with two registered nested worktrees
(`.claude/worktrees/agent review` — note the space — and
`.claude/worktrees/agent-other`) plus an ordinary unrelated dirty
directory:

- `record` → patch contains `README.md`, `internal/example.go`,
  `.gitignore`, `unrelated-dir/sub/file.txt`; zero `160000` entries; no
  `.claude/worktrees` reference anywhere under the feature directory.
- `record --files '.claude/worktrees/agent review'` → the targeted
  "every requested path is a registered nested Git worktree" diagnostic,
  nonzero exit.
- `land --dry-run` → zero `.claude/worktrees` occurrences in the staging
  plan, the outside-path list or the carve-out section; the ordinary
  unrelated dirty paths still appear and still refuse without
  `--allow-extra-paths`.
- `land --no-record --allow-extra-paths` → landed; `git ls-tree -r HEAD`
  has zero `160000` entries, the index has zero `.claude/worktrees`
  entries, both worktrees remain registered, `README.md` and
  `internal/example.go` are in the commit, and no journal or
  `index.lock` residue remains.

Scratch repo removed with `git worktree remove --force` on both
worktrees followed by `git worktree prune`; the repository's own
`git worktree list` is back to a single entry.

## Next Steps

1. Reviewer pass on rev-9.
2. On APPROVED: run the Wave-Close Checklist, flip `ROADMAP.md`, archive
   this handoff into `HISTORY.md`, and tag v0.15.1 (tagging is explicitly
   out of scope for the implementer).

## Blockers

None.

## Context for Next Agent

- **`git diff --cached` is the wrong audit after a commit.** It compares
  against the new HEAD, so anything the commit captured looks clean. Any
  post-commit index assertion must use `git ls-files --cached`.
- **`git commit` really does export `GIT_INDEX_FILE` to hooks** (verified
  empirically), which is why the retained index *is* the commit index —
  hook mutations are inside the evidence, not lost with an ephemeral
  copy.
- **`git write-tree` rewrites the index in place** (cache-tree
  extension), so any hash of the index must be taken *after* it, never
  before. This bit both production code and fixtures earlier in the wave.
- **The CAS rollback is deliberately conservative.** It fires only when
  HEAD is still the exact landing commit. Rolling back a branch that
  moved on would destroy someone else's commit, which is worse than the
  refusal.
- The status preimage `land` restores is the one taken *after* the
  embedded `record` has already rewritten `status.json`. Tests must
  assert the absence of the landed-at note rather than byte equality
  with a pre-`land` snapshot.
- Split-index (`core.splitIndex`) still has no dedicated fixture; it is
  covered only indirectly by the `write-tree` tree comparison. Flagged
  for the reviewer, unchanged from rev-8.
- Symlinked *parent* components of the index are resolved (the durable
  temp is created in the resolved directory) rather than refused, so
  symlinked `.git` setups keep working. Only the index file itself is
  refused when it is a symlink or non-regular.

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
