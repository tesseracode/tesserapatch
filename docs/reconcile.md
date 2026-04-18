# Reconcile Workflow

`tpatch reconcile` replays your recorded feature patches onto a new upstream baseline. This doc covers when to run it, the three workflow patterns it supports, the one anti-pattern it refuses, and the preflight contract added in v0.4.2.

## What reconcile does (and does not do)

Reconcile is **not** a git merge driver. It does not run `git merge`, `git rebase`, or `git pull`. It assumes your working tree is already at the target upstream state and then walks each active feature through four verdict phases:

1. **Reverse-apply** — is the recorded patch still *present* in the tree? (Clean fork case.)
2. **Operation-level** — for each recipe operation, does the target still match? (Partial drift.)
3. **Provider-semantic** — ask the LLM whether the feature's *intent* is already satisfied. (Upstream may have merged a semantically equivalent change.)
4. **Forward-apply** — try `git apply --3way`; classify as strict / 3-way-clean / conflicts / blocked via `PreviewForwardApply` in an isolated `git worktree`.

The verdict is written to each feature's `status.json` and surfaced in the terminal output.

## Two supported patterns

### Pattern A — Pristine main, features as patches

- `main` branch is a pure mirror of upstream.
- Features live only as `.tpatch/features/<slug>/artifacts/post-apply.patch` (and optional `apply-recipe.json`).
- `.tpatch/` is committed to the branch so feature state travels with it.

```
git fetch upstream
git merge --ff-only upstream/main
tpatch reconcile
```

Recommended for fast-moving upstreams where you want clean history and reconcile does the heavy lifting.

### Pattern B — Features as commits, `.tpatch/` as audit trail

- Feature edits live as normal git commits on `main`.
- `.tpatch/` is committed alongside them for auditability.

```
git fetch upstream
git rebase upstream/main
tpatch reconcile    # now mostly an audit tool
```

Rebase resolves the merge; reconcile's verdicts tell you whether any feature's *intent* drifted despite the mechanical rebase succeeding.

## The anti-pattern (refused as of v0.4.2)

**Do not** run `tpatch reconcile` on a dirty working tree, a tree containing conflict markers, or a tree with `.orig` / `.rej` merge leftovers. Verdicts become unreliable because `git apply --check` reads file bytes, not git trees — a lingering `<<<<<<<` line looks like any other context line. Observed live: agents called `git stash --include-untracked`, which swept `.tpatch/` out of the tree entirely, leaving reconcile with nothing to replay.

As of v0.4.2, `tpatch reconcile` refuses these trees unconditionally and prints:

```
error: reconcile requires a clean working tree. Detected:
  modified:         M  apps/server/src/foo.ts
  untracked:        bar.txt
  merge markers:    apps/server/src/router.ts
  merge leftover:   apps/server/src/router.ts.orig

To recover:
  - If these changes belong to an active feature, commit them first.
  - If they are a half-applied merge or stash, resolve or abort first:
      git merge --abort         (if mid-merge)
      git reset --hard HEAD     (to discard — destructive!)
      git stash                 (to set aside)
  - If you understand the risks and want to proceed anyway, pass
    `--allow-dirty` (not recommended; verdicts may be wrong).
```

The preflight checks four conditions; any non-empty result blocks the run:

1. `git status --porcelain` is non-empty (unstaged or untracked files).
2. Any tracked file contains a `<<<<<<< `, `=======`, or `>>>>>>> ` line.
3. Any `*.orig` or `*.rej` file exists anywhere in the tree (except `.git/`).

## Flags

| Flag | Effect |
|---|---|
| `--preflight` | Run only the preflight checks and exit. 0 = clean, non-zero = violations printed to stderr. Good for CI gating. |
| `--allow-dirty` | Bypass the preflight. Prints a one-line warning and proceeds. Verdicts may be wrong — **use only when you know why you're overriding**. |
| `--upstream-ref <ref>` | Upstream ref to reconcile against (default `upstream/main`). |
| `--timeout <dur>` | Overall reconcile timeout (default 2m). |

## Troubleshooting

### "I already ran `git stash` and `.tpatch/` is gone"

Recover the state from the stash before reconciling:

```
git checkout stash@{0} -- .tpatch/
tpatch reconcile
git stash pop    # or `git stash drop` if you already have the edits back
```

### "reconcile says blocked but `git apply --3way` works manually"

Check for merge-marker pollution. `PreviewForwardApply` classifies as blocked when the 3-way apply succeeds but leaves conflict markers in the result. The conflict files are listed in the feature's `status.json:reconciliation.conflicts`. Resolve them manually, `record` the resolved feature, then reconcile again.

### "tip: .tpatch/ is not tracked"

Reconcile prints this hint when `.tpatch/` is absent from `git ls-files`. Your feature state won't travel when a collaborator clones the branch. Run `git add .tpatch/ && git commit -m "chore: commit tpatch state"` to fix it.

## Related

- [Recording Patches](./record.md) — covers the sibling `tpatch record` command.
- [Feature Layout](./feature-layout.md) — which files reconcile reads (and which it ignores).
- `SPEC.md` — authoritative CLI surface.
