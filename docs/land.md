# Landing Features as Git Commits

`tpatch land <slug>` is the user-visible bridge between tpatch state and Git history. It composes (record → safe path-set staging → one Git commit) into a single verb that produces an ordinary Git commit carrying the locked four-trailer block (`Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`). The trailer block is the canonical feature↔commit binding — `git log --grep '^Tpatch-Feature: <slug>$'` enumerates every commit that lands a given slug.

The #1 footgun `land` removes is the manual three-step dance — `tpatch record <slug>` → `git add <paths>` → `git commit -m "..."` — where it is easy to forget the trailer, stage unrelated edits, or drop the apply-recipe metadata. `land` does the bookkeeping for you and refuses cleanly when the tree is not in a shape that can produce a well-formed feature commit.

## Command surface

```
tpatch land <slug> [--message <subject>] [--allow-extra-paths]
                   [--no-record] [--from <ref>] [--auto] [--files <paths>]
                   [--dry-run]
```

| Flag | Default | Purpose |
|---|---|---|
| `--message <subject>` | derived (see below) | Subject line of the Git commit. |
| `--allow-extra-paths` | off | Permit staging of files outside the feature's recorded patch scope (with one-line warning per file). |
| `--no-record` | off | Skip the embedded `record` step; trust the existing `post-apply.patch` as-is. Refuses if `status.Apply.HasPatch == false`. |
| `--from <ref>` | none | Forwarded to the embedded `record`; same semantics as `tpatch record --from`. |
| `--auto` | off | Forwarded to the embedded `record`; runs `record --auto` baseline inference. Mutually exclusive with `--from`. |
| `--files <paths>` | none | Forwarded to the embedded `record`; same semantics as `tpatch record --files`. |
| `--dry-run` | off | Print the dry-run contract and exit; do not modify the working tree, the index, or `.tpatch/`. |

`--commit-range <a>..<b>` is intentionally **not** exposed. If you need that form, run `tpatch record --commit-range` then `tpatch land --no-record`. Composability beats flag surface.

Authoritative source: PRD-tpatch-land §3.1.

## Pre-flight (refusals)

`land` refuses (exit non-zero, no mutations) when any of the following hold. The preflight is **deliberately narrower** than `tpatch reconcile`'s — `reconcile` requires a fully clean tree because it replays patches, but `land` is *recording and committing* the working tree, so unstaged and untracked files are expected and welcome.

1. **Feature does not exist** — `status.json` missing for the slug.
2. **Conflict markers present** — any tracked file contains `<<<<<<< `, `======= `, or `>>>>>>> ` lines. Conflict markers in a feature commit are never the right answer.
3. **Merge leftovers present** — any `*.orig` or `*.rej` file anywhere in the tree (excluding `.git/`).
4. **Mid-merge / mid-rebase / mid-cherry-pick** — `.git/MERGE_HEAD`, `.git/REBASE_HEAD`, or `.git/CHERRY_PICK_HEAD` exists.
5. **`record` would refuse** — if the embedded `record` step would itself refuse (empty capture without `--from`/`--auto`), `land` refuses with the same diagnostic surfaced verbatim. No re-wrapping.
6. **Cross-feature collision detected** — if the freshly recorded patch is byte-identical to another feature's `post-apply.patch`, `land` refuses with the same diagnostic `record` emits (see [`docs/record.md`](./record.md) §"Cross-feature collision detection").
7. **Hard-parent dep unsatisfied** — any unsatisfied hard parent in `depends_on` (see [`docs/dependencies.md`](./dependencies.md) §"Apply-time semantics") refuses with the existing apply-time gate diagnostic. `land` reuses the dependency-system gate; it does not reinvent it.

**Not** a refusal:

- Unstaged modifications anywhere in the working tree.
- Untracked files anywhere in the working tree.

These are the substrate `record` is built to capture. The path-set discipline below is what scopes them.

Authoritative source: PRD-tpatch-land §3.2.

## Safe staging algorithm

After the embedded `record` step writes `artifacts/post-apply.patch`:

1. Compute the **feature path set** = files in `post-apply.patch`.
2. Append the feature's metadata directory: `.tpatch/features/<slug>/`.
3. Append `.tpatch/upstream.lock` and `.tpatch/FEATURES.md` if the embedded `record` step modified them (snapshot/diff around step 1). If either file is dirty in the working tree but `record` did not modify it (operator drift unrelated to this feature), `land` emits a one-line stderr note (`note: leaving <path> dirty (operator drift outside feature scope; not staged)`) and **leaves the file dirty in the working tree**. The carve-out is bounded to these two named files; see [ADR-021](./adrs/ADR-021-tpatch-land-global-metadata-carve-out.md) and the "Carve-out for global metadata drift" section below.
4. Diff the working-tree change set against the path set. If any path is dirty in the working tree but **not** in the path set:
   - With `--allow-extra-paths`: stage it and emit a one-line warning per file (`note: staging extra path foo/bar.go (not in feature patch); the feature commit will include this`).
   - Without `--allow-extra-paths`: refuse with the list of extra paths and a hint to either revert them, run `git stash`, or re-run with `--allow-extra-paths`.
5. Stage in an **isolated index**. `land` resolves the effective index (honouring `GIT_INDEX_FILE` byte-for-byte), refuses a symlinked or non-regular index outright, and seeds a private temporary index with identical bytes. Every `git add`, every audit and the commit run against that private index via `GIT_INDEX_FILE`, so your own index is never modified while `land` works. After staging (and again after `status.json` is staged) the temp index is audited: anything inside a registered nested worktree refuses.
6. Publish under Git's index lock. `land` takes `<index>.lock` with `O_EXCL` — the same lock `git add` takes — and re-compares your index against the snapshot from the start of the run. If it changed (a concurrent `git add`, `git reset`, …) `land` refuses without overwriting your work. Otherwise it commits from the temp index while holding the lock, publishes the result, and releases. If the commit fails (for example a rejecting `pre-commit` hook), the audited pre-commit index is published instead so the intended paths are staged for a `tpatch land <slug> --no-record` retry.

Publication is durable: bytes go to a temp in the index's own directory, are fsynced, renamed into place, and the directory is fsynced, so you always see a complete index. The lock is only a mutex sentinel — it is never renamed onto the index as data — and `land` removes the lock it created on every failure path while never touching a lock it did not create.

### Crash recovery

Before committing, `land` writes a journal and a retained copy of its alternate index under the gitignored `.tpatch/local/land-journal/`. Every `tpatch land` checks for one **before** it touches anything, and decides from evidence:

- HEAD still at the pre-land commit → the commit never happened, so the retained audited index is published as the staged-retry state.
- HEAD advanced to a direct child of the pre-land commit carrying this feature's `Tpatch-Feature` trailer, whose tree matches the retained index → the commit completed and only publication was outstanding, so `land` finishes it.
- Anything else → `land` refuses and prints manual-recovery steps, keeping the journal and retained index and changing nothing.

Recovery is idempotent, and a successful land leaves no journal, retained index or lock behind. If publication fails *after* a successful commit, `land` reports "commit succeeded, recovery pending" — it never claims HEAD was rolled back.

This serializes index writes against other Git processes and makes index publication durable. It does **not** protect against concurrent ref or working-tree changes such as `git checkout` or `git reset --hard`, which no index lock can express.

The path set is intentionally **strict**: a `land` that silently absorbs unrelated edits is exactly the WP-001 §5.2 row 5 problem moved one step downstream into Git history.

Nested-worktree discovery runs twice: once at `land` entry — before the embedded `record` — so a broken `git worktree list` refuses before `record` can write anything, and once again immediately before planning and staging, which is the answer the plan is actually built from. The revalidation matters because a linked worktree registered in between (by `record` itself, or by a concurrent agent harness) would otherwise look like an ordinary extra and be staged as a gitlink under `--allow-extra-paths`. If the revalidation fails, `land` refuses with `status.json`, the index and HEAD untouched; the embedded `record`'s artifacts persist, exactly as if you had run `tpatch record` and then a failing `tpatch land`. `land --dry-run` discovers once, at plan time. Step 1 parses `post-apply.patch` strictly: a header tpatch cannot resolve (including Git C-quoted paths) refuses rather than yielding an empty path set, and the refusal happens before `status.json`, the index or HEAD is touched. Registered linked Git worktrees nested beneath the repository root are subtracted from the dirty-path set before steps 1–5 run, so they never appear in the staging plan, in the outside-path refusal list, or in the `--allow-extra-paths` sweep — and a stale pre-fix `post-apply.patch` carrying a `mode 160000` gitlink entry cannot re-introduce one. The dirty-path set is read from `git status --porcelain -z` so paths arrive byte-for-byte and worktree names carrying spaces or trailing whitespace still match. If worktree discovery itself fails, `land` and `land --dry-run` refuse before printing any staging plan rather than emitting a plan that cannot be trusted. Ordinary unrelated dirty paths keep their existing extras semantics. See [GH #7](https://github.com/tesseracode/tesserapatch/issues/7).

Authoritative source: PRD-tpatch-land §3.3.

## Commit message and trailer block

### Subject line precedence

1. `--message <subject>` if provided (verbatim, no munging).
2. `feature.spec.md`: parse the first H1 (`# ...`) line; truncate to 72 chars if needed.
3. `feature.request.md`: first non-empty line; truncate to 72 chars.
4. Fallback: `feat(<slug>): apply tpatch feature`.

Body: empty by default. Operators can `git commit --amend` afterward; `land` does not try to be a commit-message editor.

### The four-trailer block (locked — see ADR-019)

`land` always emits this block, in this exact order, separated from the subject/body by one blank line and immediately above the repo `Co-authored-by:` line:

```
Tpatch-Feature: <slug>
Tpatch-Patch-SHA: <sha256-hex of post-apply.patch>
Tpatch-Recipe-SHA: <sha256-hex of apply-recipe.json | "none">
Tpatch-Base-Commit: <full SHA from status.json:apply.base_commit>
```

Semantics:

- **`Tpatch-Feature: <slug>`** — the **sole** feature↔commit binding in tracked Git state. `git log --grep '^Tpatch-Feature: <slug>$'` enumerates every commit that lands `<slug>`. `status.json` does **not** record the landed commit (chicken-and-egg: a commit cannot embed its own SHA).
- **`Tpatch-Patch-SHA`** — sha256 of the `post-apply.patch` bytes the embedded record step wrote. Audit anchor for "the patch reviewed equals the patch committed."
- **`Tpatch-Recipe-SHA`** — sha256 of `apply-recipe.json` bytes if present; `none` otherwise. Optional audit anchor.
- **`Tpatch-Base-Commit`** — the lower bound of the captured range (per `record --auto`). For working-tree captures this is the HEAD at record time; for `--from` / `--auto` captures it is the resolved lower bound. Always knowable **before** the new commit exists, so it is safe to embed.

`Tpatch-CVE: CVE-YYYY-NNNN` is the only additive trailer permitted; it is emitted by `tpatch hotfix` (when it delegates to `land`) and inserts after `Tpatch-Base-Commit` and before `Co-authored-by:`. No other trailers may be appended by `land`.

The schema is locked in [`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md`](./adrs/ADR-019-tpatch-land-trailer-block-schema.md). Authoritative source: PRD-tpatch-land §3.4.

## Dry-run contract

`tpatch land <slug> --dry-run` prints exactly the following to stdout in this order, then exits 0. No mutations to the working tree, index, or `.tpatch/`.

```
DRY RUN: tpatch land <slug>

Pre-flight:
  feature state         : <state>
  hard-parent gate      : ok | <error>
  working-tree hygiene  : clean | <error>
  collision check       : (deferred to embedded record)

Embedded record:
  mode                  : working-tree | from-ref
  --from                : <ref>           (if any)
  --files               : <paths>         (if any)
  expected patch bytes  : <n>             (current capture)
  expected files in patch: <count>

Staging (path set):
  +A src/extras/button.css                    (new, intent-to-add)
   M src/extras/index.ts
   M .tpatch/features/<slug>/status.json
   M .tpatch/features/<slug>/artifacts/post-apply.patch
   M .tpatch/features/<slug>/artifacts/apply-recipe.json   (if regenerated)
   M .tpatch/features/<slug>/record.md
   ?  patches/NNN-record.patch                              (will be added)

Outside path set (would refuse without --allow-extra-paths):
   M unrelated/file.go

Commit:
  subject               : <derived subject>
  trailers              :
    Tpatch-Feature: <slug>
    Tpatch-Patch-SHA: <sha256>
    Tpatch-Recipe-SHA: <sha256 | none>
    Tpatch-Base-Commit: <sha>

Post-conditions if you re-run without --dry-run:
  HEAD will move from <old-sha> to a new commit.
  Working tree will be clean.
  Feature → commit binding: git log --grep '^Tpatch-Feature: <slug>$'
  status.json:apply.base_commit unchanged (owned by record/auto-base).
```

Authoritative source: PRD-tpatch-land §3.5.

## Post-conditions

After a successful `tpatch land`:

- HEAD has advanced by exactly **one** commit.
- Working tree and index are clean **with respect to feature scope** (`git status --porcelain` shows at most operator-drifted `.tpatch/upstream.lock` and/or `.tpatch/FEATURES.md` entries that were dirty before the embedded `record` step ran; see "Carve-out for global metadata drift" below). All other tracked files, the feature directory, and the index MUST be clean.
- `status.json:apply.base_commit` is **unchanged by `land`** — it remains whatever the embedded `record` step (or `record --auto`) wrote, which is the lower bound of the captured range, not the new HEAD. A commit cannot embed its own SHA in tracked content; the `Tpatch-Feature:` trailer carries the feature↔commit binding instead.
- `status.json:notes` records a one-line `landed at <ts>` entry. The new HEAD's SHA is **not** written here (same chicken-and-egg reason).
- `patches/NNN-record.patch` is the latest numbered audit snapshot (already produced by the embedded `record`).
- The new commit's `Tpatch-Feature: <slug>` trailer is the canonical feature↔commit binding for any consumer (audit, future `tpatch list`, `feat-noncontiguous-feature-commits`).

Authoritative source: PRD-tpatch-land §3.6.

## Carve-out for global metadata drift

Two `.tpatch/`-side global files — `.tpatch/upstream.lock` and `.tpatch/FEATURES.md` — are shared across all features and routinely accumulate **operator drift** between lands in shared / non-ephemeral worktrees (manual upstream pinning, hand edits to `FEATURES.md`, etc.). Sweeping that drift into a feature commit silently is exactly the boundary-capture failure `land` is built to prevent (WP-001 §5.2 row 5). Refusing on every drift, however, is too rigid for the realistic shared-worktree workflow and would push operators toward `--allow-extra-paths`, which is worse.

`land` therefore applies a **bounded, audit-visible carve-out** to those two files and *only* those two files:

- If the embedded `record` step modified the file → it is staged into the feature commit (normal path).
- If the file is dirty in the working tree but `record` did not modify it → `land` prints `note: leaving <path> dirty (operator drift outside feature scope; not staged)` to stderr and leaves the file dirty. It is **not** staged, **not** included in the commit, and **not** treated as an "extras" refusal — even with `--allow-extra-paths`.
- If the file is clean → ignored.

When you see the note, it means: the feature commit is well-formed, but you have unrelated changes to one of the two named globals still sitting in your working tree. Decide what to do with them separately (commit them in their own commit, revert them, or stash). The note is mandatory so that the drift stays visible to reviewers; this is an exception, not a feature to rely on.

Authoritative sources: PRD-tpatch-land §3.3 step 3, §3.6, §6 ac.6; [ADR-021](./adrs/ADR-021-tpatch-land-global-metadata-carve-out.md).

## Error recovery

`land` is **not** atomic across the three steps from the operator's viewpoint, but it is recoverable:

- **`record` failed** — nothing was staged or committed. Re-run after fixing.
- **Staging failed** — nothing was committed. The embedded `record`'s output (`post-apply.patch`, recipe regeneration, numbered audit snapshot) is already on disk. Re-running `land` is safe; the next `record` either no-ops (byte-identical content) or re-captures the same content.
- **`git commit` failed (e.g. pre-commit hook rejects)** — the index is staged but uncommitted. `land` prints the hook output verbatim and a recovery hint: `tpatch land <slug> --no-record` retries the commit step against the existing index without re-running record.

These behaviours fall out of using `git` primitives directly. `land` does not implement its own transaction layer.

Authoritative source: PRD-tpatch-land §3.7.

## Composition with reconcile patterns

Both reconcile patterns documented in [`docs/reconcile.md`](./reconcile.md) §"Two supported patterns" continue to work. `land` is the well-formed-feature-commit producer for both — it's just a no-op for one and a strict improvement for the other.

### Pattern A — Pristine main, features as patches

In Pattern A, `main` mirrors upstream and features live only as `.tpatch/features/<slug>/artifacts/post-apply.patch`. `.tpatch/` is committed so feature state travels with the branch.

`land` in Pattern A is **feature-branch only** in v1: run `land` on a feature branch, not `main`. It produces one feature-branch commit containing both the code edits and the `.tpatch/features/<slug>/` metadata. `main` stays pristine. The branch can then be discarded or kept as a "feature-as-commit" snapshot for tooling like `feat-noncontiguous-feature-commits` to consume.

Metadata-only landing onto `main` is **out of scope** in v1 — `record --files .tpatch/...` cannot work because `CapturePatchScoped` always excludes `.tpatch` before user pathspecs. Pattern A users who today maintain `.tpatch/` metadata as ordinary Git commits on `main` should keep using `git add .tpatch/features/<slug>/ && git commit`.

### Pattern B — Features as commits

In Pattern B, feature edits live as normal Git commits on `main` and `.tpatch/` is committed alongside for auditability.

`land` in Pattern B is the **default** path: one commit per feature, code + metadata together. The `Tpatch-Feature: <slug>` trailer formalizes a convention Pattern B users today maintain by hand (commit message conventions, branch names, etc.). After `land`, `git rebase upstream/main` continues to work exactly as `docs/reconcile.md` documents, and reconcile's audit role is unchanged.

Authoritative source: PRD-tpatch-land §4.

## Boundary with `cycle`

`tpatch cycle <slug>` runs the full `analyze → define → explore → implement → apply → record` sequence in one batch. `land` and `cycle` **compose; they do not overlap**.

| Question | `cycle` answer | `land` answer |
|---|---|---|
| Generates `analysis.md` / `spec.md` / `exploration.md`? | yes | no |
| Generates `apply-recipe.json`? | yes (via `implement`) | no (only via embedded `record`'s autogen) |
| Mutates working-tree code? | yes (via `apply --mode execute`) | no — assumes code is already in place |
| Captures `post-apply.patch`? | yes (via `record`) | yes (via embedded `record`) |
| Stages files? | no | yes |
| Creates a Git commit? | no | yes |

Composition pattern:

```
tpatch cycle extra-button --skip-execute     # phases 1-4
# operator implements / Path-B-edits
tpatch cycle extra-button                     # apply + record
tpatch land  extra-button                     # stage + commit
```

`cycle` is *not* extended to absorb `land`. If a future user wants "do everything including the commit" we expose it as a flag on `cycle` (e.g. `cycle --land`) that delegates to `land`. We do not duplicate `land`'s logic inside `cycle`. **If `land` ever grows phase orchestration, it has merged into `cycle` and should be folded back in.**

Authoritative source: PRD-tpatch-land §5.

## Related

- [Recording Patches](./record.md) — covers the sibling `tpatch record` command that `land` invokes.
- [Reconcile Workflow](./reconcile.md) — what happens to landed feature commits when upstream changes under you.
- [Feature Layout](./feature-layout.md) — which files `land` stages and which the trailer SHAs hash.
- [`docs/dependencies.md`](./dependencies.md) — the hard-parent gate `land` reuses.
- [`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md`](./adrs/ADR-019-tpatch-land-trailer-block-schema.md) — locks the four-trailer schema.
- [`docs/prds/PRD-tpatch-land.md`](./prds/PRD-tpatch-land.md) — authoritative PRD.
- `SPEC.md` — authoritative CLI surface.
