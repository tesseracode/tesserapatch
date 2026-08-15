# Adjacent CLI Argument Conflict — Semantic Replay Case Study

**Status**: Empirical research — awaiting review; no implementation authorized
**Date**: 2026-08-15
**Owner**: Core
**Issues**:
[GH #12 operation replay](https://github.com/tesseracode/tesserapatch/issues/12),
[GH #13 feature absorption](https://github.com/tesseracode/tesserapatch/issues/13),
[GH #14 verified reorder](https://github.com/tesseracode/tesserapatch/issues/14)

## 1. User scenario

A feature branch added two arguments to a Go CLI argument slice. A later change
on `master` intentionally removed neighboring arguments. GitHub reported the
feature PR as conflicting even though the desired semantic result was simple:

- keep the upstream deletions;
- keep the two feature arguments;
- do not restore the deleted arguments.

The question was whether merge was the right repair, whether rebase would avoid
the conflict, and whether tpatch could replay the intent rather than the
textual hunk.

## 2. Reproduction

[`reproduce.sh`](./reproduce.sh) creates disposable repositories and compares
merge with rebase over four variants.

| Feature edit | Upstream edit | Merge | Rebase |
|---|---|---|---|
| add after old arguments | delete both old arguments | conflict | conflict |
| add before old arguments | delete both old arguments | conflict | conflict |
| add between old arguments | delete the first old argument | conflict | conflict |
| add a separate `append(args, ...)` statement | delete both old arguments | clean | clean |

In each adjacent case, Git places the feature additions and upstream deletion
in one conflicting hunk. Rebase does not avoid the conflict: it replays the
feature commit onto the new base and hits the same overlapping edit. With one
feature commit, merge and rebase therefore require the same semantic decision.
With multiple commits, rebase may expose that decision earlier or more than
once because each commit is replayed separately.

The correct resolved slice is:

```go
args := []string{
	"--feature-x",
	"--feature-y",
}
```

The separate-append variant is textual evidence for the semantic distinction:
when the feature operation has an anchor independent of the removed entries,
Git can preserve both intents without help.

## 3. Current tpatch experiment

The current CLI was built from the repository and run against the same fixture.

### 3.1 Recorded patch

`tpatch record add-feature-args --files command.go` succeeds, but recipe
autogeneration emits a whole-file `write-file` operation. On the upstream tree
with the old arguments deleted:

- operation evaluation reports the whole-file operation as conflicting;
- forward-apply preview leaves conflict markers;
- reconcile returns `blocked`;
- hunk evidence classifies the case as high-confidence `edit-overlap`;
- the recommended action is `human-or-provider-resolution`.

That result is safe and correct, but not automatic.

### 3.2 Structural operation

A hand-authored `replace-in-file` operation anchored on the slice closing brace
and the following `return run(args)` is applicable on the updated tree.

- `tpatch apply --mode execute --dry-run` succeeds.
- `tpatch apply --mode execute` writes only the two feature arguments.
- The intentionally deleted upstream arguments remain deleted.

However, `tpatch reconcile` still falls through to the conflicting canonical
patch. Current phase 2 recognizes only the `allPresent` case; an applicable
operation is not replayed. Source: `internal/workflow/reconcile.go` phase 2 and
`evaluateRecipeOperations`.

## 4. Conclusions

### 4.1 Git workflow

Merging `master` into the PR branch and resolving the conflict was a correct
approach. Rebasing is also valid when branch-history policy permits a
force-push, but it is a history-shaping choice, not a conflict-avoidance tool
for this case.

### 4.2 Existing tpatch value

Existing surfaces already help:

1. hunk-overlap evidence identifies a true textual overlap rather than an
   unrelated-file conflict;
2. `reconcile --resolve` can ask a provider to resolve the conflicted file in a
   shadow worktree;
3. an intent-preserving structural recipe can reapply cleanly with
   `tpatch apply`.

The deterministic bridge is missing: reconcile does not execute an applicable
recipe operation before escalating to textual/provider resolution.

### 4.3 Recommended product direction

Do not introduce a branch-history command named `tpatch rebase` first. Research
GH #12 as a reconcile middle pass:

1. replay applicable operations in an isolated worktree;
2. validate preimages, path scope, resulting diff and tests;
3. emit a reviewable patch/evidence record;
4. require acceptance by default;
5. write a new patch generation.

Commit rebasing can remain the operator's Git-policy choice.

## 5. Absorption and ordering follow-ons

The same investigation found adjacent but distinct lifecycle gaps.

### 5.1 Absorption

`upstream_merged` already means “absorbed into baseline”; a new lifecycle state
is not yet justified. Existing retention endpoints are:

- keep the complete `upstream_merged` feature directory;
- remove it completely with `--auto-drop-merged` / `remove` when dependency
  gates allow.

Missing middle tiers are tracked in GH #13: retain intent, retain a stub, or
drop bulky patches/audit artifacts. The design must reconcile `satisfied_by`
provenance with the current no-dangling-parent validator and must define hard
ancestor closure before a child can be absorbed.

### 5.2 Reparenting and reorder

`tpatch feature deps add/remove` edits graph metadata atomically and rejects
cycles, but it does not transform patches or prove that two orders are
equivalent. GH #14 tracks verified reorder/reparent:

- replay `A;B` and `B;A` from one immutable base;
- compare normalized trees and tests;
- update edges, patch generations, base metadata and verification snapshots
  transactionally;
- refuse unknown/non-commuting cases.

This is the concrete use case anticipated by
[`patch-theory-and-commutation.md`](../../patch-theory-and-commutation.md) and
the existing `feat-feature-reorder` / `feat-feature-standalonify` backlog.

## 6. Limits

- The fixture is synthetic, not a production-repository transition study.
- Structural replay proves one base and one operation representation, not
  general semantic equivalence.
- No new command, state, schema or architecture is authorized by this study.
