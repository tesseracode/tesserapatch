# Commit Strategy

> **Status**: Interim convention. This document captures the
> per-feature commit pattern that operators should follow today, while
> [`docs/prds/PRD-tpatch-land.md`](./prds/PRD-tpatch-land.md) is gated
> behind the two `record` guardrail PRDs. Once `tpatch land` ships,
> that command supersedes the manual two-commit dance described here,
> and the four-trailer block in PRD-tpatch-land §3.4 supersedes the
> single-trailer convention in §3 of this doc.

## TL;DR

One feature → **two ordinary Git commits**, both carrying a
`Tpatch-Feature: <slug>` trailer:

1. **Production-code commit** — the actual change. Prefix `feat:` /
   `fix:` / etc. as your repo's convention dictates. Must NOT include
   anything under `.tpatch/features/<slug>/`.
2. **`chore(tpatch):` metadata commit** — `.tpatch/features/<slug>/`
   only, written by `tpatch record --from HEAD~1`. Must NOT include
   any production-code edits.

The trailer makes the feature ↔ commit mapping machine-readable today
and forward-compatible with `tpatch land`.

---

## 1. Why two commits?

Every other workflow in the codebase is silent about the commit
boundary between feature edits and `.tpatch/` metadata. `docs/record.md`
and `docs/reconcile.md` document the *patterns* (Pattern A / Pattern B)
but leave the staging boundary to the operator. Live stress testing
(WP-001 §5.2, Cases A1 + A2 — `tesseracode/copilot-api` and
`tesseracode/t3code`) found that ad-hoc operator decisions in this seam
are the dominant source of **boundary-capture failures**: features
ending up with byte-identical `post-apply.patch` files because record
captured the wrong scope.

Splitting code and metadata into two commits — and binding both to a
slug via a trailer — addresses this seam without waiting for `tpatch
land` to land:

- The production-code commit is auditable on its own
  (`git log -- <files>` shows feature work without `.tpatch/` noise).
- The metadata commit is reviewable as a contract change rather than
  as part of the feature diff.
- `git rebase upstream/main` (Pattern B) handles the two commits
  cleanly because they touch disjoint paths.
- The trailer survives clones, force-pushes, and rebases, so feature
  identity persists even after history rewriting.

This pattern is **not** a substitute for `tpatch land`. Once
`tpatch land` ships:

- It produces **one** commit per feature (combining both halves with
  full SHA trailers).
- This doc transitions from "interim contract" to "deprecated
  fallback for repos pinned to pre-`land` tpatch versions".

---

## 2. Path B workflow (recommended)

This is the agent-collaboration flow stress-tested in our
`tesseraspaces` work and documented as authoritative for repos using
agent-as-provider authoring.

```
┌─────────────────────────────────────────────────────────────────┐
│  1. tpatch analyze  <slug>                                      │
│  2. tpatch define   <slug>     (alias: tpatch spec)             │
│  3. tpatch explore  <slug>                                      │
│  4. tpatch feature deps --validate-all     # DAG sanity check   │
│  5. tpatch apply    <slug> --mode started                       │
│  6. <agent edits production code>                               │
│  7. (optional) run tests                                        │
│  8. tpatch apply    <slug> --mode done                          │
│  9. git add <production paths only>                             │
│ 10. git commit -m "feat: <subject>                              │
│                                                                 │
│         Tpatch-Feature: <slug>"                                 │
│ 11. tpatch record   <slug> --from HEAD~1                        │
│     # writes post-apply.patch + auto-generates apply-recipe.json│
│ 12. (optional) agent annotates recipe op descriptions           │
│ 13. (optional) tpatch apply <slug> --dry-run    # round-trip    │
│ 14. git add .tpatch/features/<slug>/                            │
│ 15. git commit -m "chore(tpatch): record <slug>                 │
│                                                                 │
│         Tpatch-Feature: <slug>"                                 │
└─────────────────────────────────────────────────────────────────┘
```

Phase ordering (matches the steering convention):

```
requested  → tpatch analyze                → analyzed
analyzed   → tpatch define                 → defined
defined    → tpatch explore                → defined (exploration.md enriched)
defined    → tpatch apply --mode started   → implementing
implementing → <agent edits + tests>       → implementing
implementing → tpatch apply --mode done    → applied
applied    → git commit (production)       → applied (HEAD advances)
applied    → tpatch record --from HEAD~1   → active
active     → git commit (chore tpatch)     → active (HEAD advances)
active     → (optional) tpatch implement   → active (apply-recipe.json refreshed)
active     → tpatch reconcile              → active | upstream_merged | blocked
```

Notes on the steps:

- **Step 4 — DAG validation.** New parent links and ordering
  constraints often surface during `explore`. Register them with
  `tpatch feature deps <slug> add <parent>:hard` (or `:soft`) before
  moving on; running `tpatch feature deps --validate-all` confirms the
  DAG is acyclic and free of dangling refs. See
  [`docs/dependencies.md`](./dependencies.md).
- **Steps 5 + 8 — `--mode started` / `--mode done`.** These bracket
  the agent-authored edit window. They produce the
  `patches/NNN-started.patch` / `patches/NNN-done.patch` audit
  snapshots, but do not capture the canonical `post-apply.patch` —
  step 11 does that.
- **Step 11 — `record --from HEAD~1`.** Diffs the just-committed
  feature against the parent commit. Equivalent to "all changes the
  production-code commit introduced". Once `PRD-record-auto-base`
  ships, prefer `tpatch record <slug> --auto`, which infers the base
  from `.tpatch/upstream.lock` or `merge-base(HEAD, upstream/main)`.
- **Step 13 — `apply --dry-run`.** Replays the auto-generated recipe
  in-memory against the working tree without modifying anything.
  Confirms the recipe round-trips cleanly. Failure here means the
  recipe has drifted from the patch (often because the recipe
  auto-generation skipped a deletion or a binary file); regenerate
  with `tpatch record <slug> --regenerate-recipe`.
- **Steps 9–15 — staging discipline.** Resist the temptation to
  `git add -A`. Use explicit paths. Mixing code and `.tpatch/` in one
  commit is the failure shape `tpatch land`'s safe-staging algorithm
  is being designed to prevent (PRD-tpatch-land §3.3).

### Why `record` lives between the two commits

Running `record` *after* the code commit but *before* the metadata
commit is the load-bearing ordering. It means:

- `record` reads the committed code, not the working tree, so
  `post-apply.patch` is exactly the diff the production-code commit
  introduced.
- `apply-recipe.json` is auto-generated against the same commit, so
  the patch and the recipe agree by construction.
- The metadata commit is a pure follow-up that captures `record`'s
  output. No interleaving with edits.

The anti-pattern (refused since v0.4.2 — see
[`docs/record.md`](./record.md)) of running `record` on a clean tree
without `--from` is structurally avoided because step 11 always
passes `--from HEAD~1`.

---

## 3. The `Tpatch-Feature` trailer (interim convention)

Until `tpatch land` ships, **every commit produced by the Path B
workflow above MUST carry**:

```
Tpatch-Feature: <slug>
```

as a Git trailer (last block of the commit message, after a blank
line, RFC-822-style key:value).

### Format

```
<conventional commit subject>

<optional body>

Tpatch-Feature: <slug>
```

The slug is the feature identifier from `.tpatch/features/<slug>/`
(kebab-case, matches `status.json`'s implicit ID).

### Where it goes

Both commits in the pair carry the same trailer. They share a slug
because they describe one feature.

```
$ git log --pretty=full -2
commit 9a8b7c…  (HEAD)
    chore(tpatch): record extra-button

    Tpatch-Feature: extra-button

commit 1d2e3f…  (HEAD~1)
    feat: blue extra button on the dashboard

    Tpatch-Feature: extra-button
```

### Why a trailer at all?

1. **Machine-readable feature identity.**
   `git log --grep "Tpatch-Feature: <slug>"` returns exactly the
   commits associated with a feature. Without a trailer, identity is
   inferred from commit-message conventions, which are fragile under
   rebase / squash / force-push.
2. **Survives rebases.** Trailers are part of the commit message and
   travel with the commit through cherry-pick, rebase, and amend.
   Branch-name conventions do not.
3. **Forward-compatible with `tpatch land`.** PRD-tpatch-land §3.4
   adds three additional trailers (`Tpatch-Patch-SHA`,
   `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`) on a single combined
   commit. Repos that adopted `Tpatch-Feature` early will see the
   trailer block grow, not change. No migration needed.
4. **Detection of WP-001-shaped failures.** Once trailers are
   established, a CI check or `tpatch status` extension can flag
   features whose `post-apply.patch` byte-hash is shared with another
   feature's, even retroactively, by walking history with
   `git interpret-trailers --parse`.

### How to add it

Manually:

```
$ git commit -m "feat: blue extra button on the dashboard

Tpatch-Feature: extra-button"
```

Or use Git's `interpret-trailers` plumbing:

```
$ git commit -m "feat: blue extra button on the dashboard"
$ git -c trailer.tpatchfeature.key="Tpatch-Feature" \
      commit --amend -m "$(git log -1 --pretty=%B)
Tpatch-Feature: extra-button"
```

Or — simplest — set up a one-line `prepare-commit-msg` hook in your
fork that prompts for the slug. (Not provided as a shipped hook;
operator preference.)

### Validation (optional)

CI can enforce the trailer with:

```bash
# fail if any commit on this branch lacks the trailer
git log --format=%B upstream/main..HEAD | \
  awk '/^commit /{slug=""} /^Tpatch-Feature:/{slug=$2} \
       /^$/&&!slug{print "missing trailer"; exit 1}'
```

A first-class `tpatch status --check-trailers` is **not** planned for
this interim phase; once `tpatch land` ships, the four-trailer block
becomes a structural property of every feature commit and a CI check
becomes redundant.

---

## 4. Anti-patterns

| Anti-pattern | Failure shape |
|---|---|
| `git add -A && git commit -m "feat+tpatch"` | One commit mixing code and metadata. Defeats the audit and breaks `git log -- <code paths>`. Reproduces WP-001 §5.2 row 5. |
| Skipping the `Tpatch-Feature` trailer | Identity by commit-message convention only — fragile under rebase, invisible to `git log --grep`. |
| Different slugs on the two commits | Splits the feature in `git log --grep`. Almost always a typo; CI should reject. |
| Running `tpatch record` *before* the code commit | Captures the working tree (or refuses on a clean tree per `docs/record.md:38-50`). Inconsistent with the recipe auto-gen baseline. |
| Running `git stash --include-untracked` mid-flow | Stashes `.tpatch/` away. On `git stash pop` the metadata can be lost or merged at the wrong base. See `docs/reconcile.md:46-72`. |
| Squashing the two commits before `tpatch land` ships | Defeats the separation of audit history; once squashed, you cannot recover the boundary. |

---

## 5. Pattern A vs Pattern B compatibility

This convention is described above for **Pattern B** (features as
commits with `.tpatch/` as audit trail —
[`docs/reconcile.md:32-44`](./reconcile.md)) because that's the
dominant agent workflow.

**Pattern A** (pristine main, features as patches) is also
compatible:

- Run the same Path B workflow on a feature branch, not on `main`.
- The two commits live on the feature branch; `main` stays pristine.
- The branch can be discarded after `record` if you only want the
  patch in `.tpatch/`, or kept as a feature-as-commit snapshot for
  `feat-noncontiguous-feature-commits` to consume later.

In both patterns, the `Tpatch-Feature` trailer is the load-bearing
identity bridge.

---

## 6. Migration to `tpatch land`

When `tpatch land` ships:

| Today (interim) | After `tpatch land` |
|---|---|
| Two manual commits per feature | One `tpatch land` invocation per feature |
| `Tpatch-Feature` trailer added by hand | Trailer block written by `land` (incl. SHA trailers) |
| `record --from HEAD~1` between commits | `record` embedded in `land` |
| Manual `git add` of feature paths | Safe-staging algorithm in `land` (PRD §3.3) |
| Operator chooses commit message | Subject derived from `spec.md` / `request.md` (PRD §3.4) |

The migration is purely additive: existing repos using this
convention will see their commit history continue to work with
`tpatch land`-aware tooling because `Tpatch-Feature` is preserved.

`tpatch land` will be gated behind the two `record` guardrail PRDs
(`PRD-record-auto-base`, `PRD-record-collision-detection`) per
WP-001. Until then, this doc is the authoritative commit contract.

---

## 7. Related

- [`docs/record.md`](./record.md) — when to run `tpatch record`,
  `--from` flag semantics, the v0.4.2 anti-pattern refusal.
- [`docs/reconcile.md`](./reconcile.md) — Pattern A vs Pattern B,
  reconcile preflight, the dirty-tree refusal.
- [`docs/feature-layout.md`](./feature-layout.md) — what's canonical
  vs audit trail under `.tpatch/features/<slug>/`.
- [`docs/dependencies.md`](./dependencies.md) — `tpatch feature deps`
  and DAG validation.
- [`docs/agent-as-provider.md`](./agent-as-provider.md) — the broader
  Path B contract; the workflow above is the commit-discipline layer
  that sits on top of it.
- [`docs/prds/PRD-tpatch-land.md`](./prds/PRD-tpatch-land.md) — the
  drafted `land` command that supersedes this convention.
- [`docs/whitepapers/WP-001-feature-slice-gap.md`](./whitepapers/WP-001-feature-slice-gap.md)
  §5.2 — the boundary-capture failures this convention defends
  against.
