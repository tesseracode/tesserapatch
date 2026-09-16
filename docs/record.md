# Recording Patches

`tpatch record <slug>` captures the diff for a feature into the canonical
`.tpatch/features/<slug>/artifacts/post-apply.patch`. Numbered
`patches/NNN-record.patch` files are historical snapshots, not the canonical
input. The captured patch is what reconcile and downstream tooling use;
an executable recipe must explain that patch, not silently replace it.

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

> **Nested linked Git worktrees are never captured.** If a linked worktree is registered *beneath* the repository root (the `.claude/worktrees/agent-*` shape agent harnesses create), `record` subtracts that directory and everything under it from every capture mode — including when you name it explicitly in `--files`, and including `post-apply-diff.txt`. Worktree path bytes are compared exactly, so a directory named `agent ` (trailing space) is excluded while a sibling named `agent` is not. Ordinary directories, intentionally tracked submodules/gitlinks, unregistered nested Git repositories, and worktrees registered outside the repository root are unaffected. If tpatch cannot run `git worktree list`, capture **refuses** rather than recording blind; fix the reported Git error (often `git worktree prune`) and retry. **This guard requires Git 2.36 or newer**: `git worktree list --porcelain -z` is the only shape that delimits worktree paths losslessly, and tpatch will not fall back to the ambiguous newline-delimited form. On older Git, capture refuses with guidance naming the requirement. See [GH #7](https://github.com/tesseracode/tesserapatch/issues/7).

> **If `--files` names only nested worktrees**, `record` refuses with a diagnostic that names them and explains they were intentionally excluded, rather than the generic zero-byte message. To capture work done *inside* a linked worktree, point tpatch at that worktree instead: `tpatch record <slug> --path <worktree-root>`.

> **Composed alternative**: `tpatch land <slug>` does (record → safe path-set staging → one Git commit) in a single verb and writes the four-trailer block (`Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`) for you. See [`docs/land.md`](./land.md). Prefer `land` when you would otherwise immediately follow `record` with `git add` + `git commit`.

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

## Capture modes and recipe authority (GH #15; unreleased)

Choose one capture-mode family; `--files <paths>` and `--claimed-only` scope
the selected capture rather than selecting another mode:

| Command selection | Published `capture.mode` | Captured input |
|---|---|---|
| Default or `--all` | `working-tree-all` | Existing worktree diff plus untracked files. |
| `--staged` | `staged-index` | HEAD → index; overlapping staged/unstaged paths refuse. |
| `--unstaged` | `unstaged-worktree` | Index → worktree; overlapping staged/unstaged paths refuse. |
| `--from <base> [--to <tip>]` | `committed-range` | Selected committed range, default tip HEAD. |
| `--auto [--to <tip>]` | `auto-committed-range` | Inferred base and selected/default tip. |
| `--commit-range <base>..<tip>` | `explicit-committed-range` | Explicit committed endpoints. |

The reference is HEAD for accepted working-tree modes and the resolved
lower endpoint for committed ranges. Committed-range postimages come from
the upper commit, not later worktree edits. Accepted `--unstaged` is
commit-kind HEAD, not an index snapshot: overlap refusal ensures the
captured paths' index entries agree with HEAD.

Record is P1 in the [seven-producer contract](../SPEC.md#seven-governed-producers).
The embedded record step of `land` is also P1. A successful capture is
observed once before bound writes. When authorized, a complete derivation
writes the recipe, then justified `recipe-provenance.json`, then any owed
generation. Final publication atomically replaces
`recipe-capture-event.json` (**E**) before atomically replacing
`recipe-coverage.json` (**C**). This is **not a cross-file transaction**;
an error can leave earlier writes behind and returns non-zero. Same-patch
reruns still republish E/C without duplicating a patch generation.

E is unkeyed consistency evidence, not authentication or historical origin.
Readers load it independently, compare its C hash and bound inputs, and
reconstruct the proof. C absence remains legacy even beside E.

### What generation and preservation mean

Record derives only a **complete** recipe for supported regular-text
additions/modifications. It does not emit a partial recipe that drops
deletions, renames, copies, binary/mode/object changes or unavailable/unsafe
effects. If full derivation is unavailable, a missing recipe stays missing
and an existing recipe stays byte-identical; coverage reports the actual
incompleteness.

D16 proves generated origin by comparing **all freshly derived canonical
bytes** with the existing recipe. Matching paths, labels, semantic
equivalence or prior provenance are insufficient. Even formatting-only
differences fail D16. A nonmatching manual/provider recipe and its
provenance are preserved unless explicit `--regenerate-recipe` can replace
it with a complete derivation. Successful D16 reruns can repair provenance
with the current base/time, without inventing an author or past history.

`recipe coverage: complete` or `recipe coverage: incomplete (<reasons>)`
is printed on stderr after publication, with exact sorted/deduplicated
reasons. V1 complete coverage admits only `write-file` with a present,
non-null `preimage_hash` (including `""` for creation), and still requires
all ten predicates in [SPEC](../SPEC.md#complete-coverage-origin-and-reasons).
Append, replacement and ungated writes remain executable under existing
rules but cannot supply complete v1 coverage.

P2 (`feature patch refresh <slug>` and `feature patch fixup <slug> --reason <reason>`)
does not force recipe regeneration. A changed patch whose preserved recipe
no longer explains its effects raises **both** `producer-patch-rewrite` and
`recipe-not-regenerated`. A formatting-only D16 mismatch that remains
semantically exact does not raise those codes; its stale marker still
yields `recipe-stale-marker-present`. A non-empty same-latest-generation
checkpoint writes only E/C, not patch/recipe/provenance/generation/state.
An empty P2 capture is not that checkpoint.

### Diagnose before regenerating

Recipe coverage is necessary, not sufficient, for future replay eligibility; it is not cross-base safety.
A warn/exit0 coverage row is not eligibility and never grants replay permission.

Use `tpatch verify <slug>` for the full report and
`tpatch doctor --check D10` for coverage diagnosis. D10 is warning-only
and read-only even with `--fix`: it does not regenerate, delete a marker,
repair provenance or publish evidence. It offers
`tpatch record <slug> --regenerate-recipe` only after a read-only plan of
that exact default working-tree command establishes complete D16-proven
output and the real capture/state/collision/generation/publication gates.
Otherwise `recipe-generation-no-truthful-regeneration` lists blockers;
review the patch and choose a truthful capture manually. A clean committed
feature may need a consciously selected range, not blind default regeneration.

Verify warning rows do not fail the run absent other failures. Legacy missing
C, including a pre-v0.17 `recipe-stale.json`, stays verify-green; present
malformed or mismatched C/E blocks. For ordinary `apply --mode execute`,
valid incomplete coverage with a decodable recipe warns and retains existing
execution gates; bound absent/unreadable/undecodable recipes refuse with
`recipe-generation-incomplete` (exit 2). See [SPEC's apply table](../SPEC.md#explicit-apply-and-ordered-no-write-success)
for state-aware recovery and the separate state-selected canonical-patch
reapply branch. No coverage row authorizes future GH #13 replay.

## Related

- `docs/reconcile.md` — what happens to patches when upstream changes under you.
- [`docs/land.md`](./land.md) — `tpatch land`, the composed alternative for `record + git add + git commit`.
- `SPEC.md` — authoritative CLI surface.
- The skill files (`assets/skills/**`) carry a one-liner version of this rule for agents.

## Cross-feature collision detection (v0.8.0)

`tpatch record` refuses by default when the new canonical patch is byte-identical to another feature's `artifacts/post-apply.patch`. Almost always this means the record range is too broad — multiple features are being collapsed into one patch.

```
Error: recorded patch for "dynamic-models" is byte-identical to existing feature patch(es):
  - copilot-cli-provider  sha256=ab12cd34abcd... bytes=42118 files=12

This usually means the record range is too broad.
Try one of:
  tpatch record dynamic-models --auto --files <feature-paths>
  tpatch record dynamic-models --from <feature-base> --files <feature-paths>
  tpatch record dynamic-models --from <feature-base> --to <feature-tip>

To accept an intentional duplicate, rerun with:
  tpatch record dynamic-models --allow-collision "<reason>"
```

Re-recording the *same* feature with unchanged patch bytes is treated as a deduplication: the canonical artifact is rewritten in place and the numbered `patches/NNN-record.patch` audit snapshot is skipped (`record: no content change since current artifacts/post-apply.patch; skipping numbered audit snapshot`). A changed re-record appends the next numbered snapshot as before.

Use `--allow-collision "<reason>"` only for legitimate duplicates (test fixtures, demonstrations, staged migrations). The reason is mirrored to stderr and persisted in `record.md` under a "Collision Override" section.

## Typed resource capture (`--resources`, v0.15.0)

`tpatch record <slug> --resources` adds a **second, separate atomic
domain** to a record run: the feature's declared typed resources (see
`SPEC.md` → "Typed feature resources"). The Git-side capture itself is
completely unchanged — `--resources` never alters which files are
diffed, which range is used, or what `post-apply.patch` contains.

Ordering is exact and is not negotiable:

1. **Zero-resource preflight.** A feature with no declared resources
   refuses `no-resources-declared` (exit 1) *before* Git is touched and
   *before* the per-slug lock is acquired.
2. **Stage.** The per-slug `flock` is taken, the lock-gated orphan
   sweeps run, and every declared resource is captured into bounded
   in-process memory. Nothing is written to the tracked tree yet.
3. **Git-side capture.** The existing capture-mode dispatch runs,
   completely unaffected by step 2's outcome.
4. **Publish, gated on Git success.**
   - Git failed → the in-memory candidate batch is discarded and never
     written anywhere, whatever its own outcome was. **No tracked
     resource write is ever attempted before Git succeeds.**
   - Git succeeded and staging succeeded → the batch and pointer are
     published exactly as a standalone `tpatch feature resource
     capture` would publish them.
   - Git succeeded but the resource domain did not complete →
     `resource-domain-incomplete` (exit 1):

     > canonical patch recorded successfully; resource capture did not
     > complete: `<reason>`. Retry with `tpatch feature resource capture
     > <slug>` — this re-stages and republishes and is safe to re-run.

The retry is safe because `batch_id` is content-addressed: a retry over
unchanged state recomputes the identical ID and lands on the idempotent
"already published" branch rather than duplicating anything. A retry
after the underlying state genuinely changed correctly produces a
different `batch_id` — that is expected, not a re-run bug.

`record --resources` always targets **every** declared resource; there
is no subset flag. Use `tpatch feature resource capture <slug>
--resource <id>` when you want a subset.
