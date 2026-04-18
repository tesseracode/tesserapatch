# Recording Patches

`tpatch record <slug>` captures the on-disk diff for a feature and stores it under `.tpatch/features/<slug>/patches/NNNN-record.patch` (plus `artifacts/post-apply.patch` for backwards compatibility). The captured patch is what `tpatch reconcile` replays and what downstream tooling diffs against the upstream baseline.

The #1 footgun with `record` is **running it at the wrong time relative to `git commit`**. This doc explains the two supported orderings, how to recover if you got it wrong, and what the CLI does now to stop you from silently producing an empty patch.

## When to run `tpatch record`

### A. Record from the working tree (default, recommended)

Run `tpatch record <slug>` **before** `git commit`:

```
# edit files, run apply, test…
tpatch record fix-model-translation
git add -A && git commit -m "feat: fix model translation"
```

`record` captures unstaged modifications plus untracked files (via `git add --intent-to-add`, so untracked files appear in the diff). This is the common path and matches the default cycle.

### B. Record from commits after the fact

If you already ran `git commit` before realising you needed `record`, pass `--from <base>`:

```
tpatch record fix-model-translation --from HEAD~3
tpatch record fix-model-translation --from upstream/main
tpatch record fix-model-translation --from $(git merge-base HEAD upstream/main)
```

This captures the diff between `<base>` and `HEAD`. Typical picks for `<base>`:

- `upstream/main` (or whatever upstream ref you track) — full fork-vs-upstream diff. Good if every commit on this branch is part of the feature.
- The SHA just *before* your feature work began — precise, avoids picking up unrelated commits.
- `HEAD~N` — fine for a quick one-feature branch.

### Anti-pattern (refused as of v0.4.2)

Do **not** run `tpatch record` on a clean working tree without `--from`. The command now refuses this case:

```
$ tpatch record fix-model-translation
Error: tpatch record captured 0 bytes — nothing unstaged or untracked in the working tree.
  If you already committed your feature edits, rerun with --from <base>:
    tpatch record fix-model-translation --from <base-commit-or-ref>
  Recent commits on this branch (candidates for --from base):
    a1b2c3d  2 hours ago  feat: fix model translation
    e4f5g6h  yesterday    docs: update readme
```

Previously this produced a 0-byte patch, advanced the feature state to `applied`, and made reconcile look like the feature had no content. That silent failure is now impossible.

## Quick decision table

| Situation | Command |
|---|---|
| Edited files, not yet committed | `tpatch record <slug>` |
| Committed one feature to this branch, nothing else | `tpatch record <slug> --from upstream/main` |
| Committed feature A, then committed feature B on top, realise A needs a fix | see `feat-noncontiguous-feature-commits` (planned) |
| Working tree clean, no commits either | Nothing to record — do your edits first |

## Related

- `docs/reconcile.md` — what happens to patches when upstream changes under you.
- `SPEC.md` — authoritative CLI surface.
- The skill files (`assets/skills/**`) carry a one-liner version of this rule for agents.
