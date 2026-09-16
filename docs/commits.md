# Commit Strategy

> **Status**: Legacy/manual two-commit fallback. For current uncommitted
> work prefer [`tpatch land`](./land.md): it already composes record,
> scoped staging and one commit with the locked four-trailer block.
> The two-commit convention below remains useful for retroactive recording
> or older pinned tools; its single trailer is not modern landing
> attestation.

## TL;DR

For the manual fallback, one feature → **two ordinary Git commits**, both carrying a
`Tpatch-Feature: <slug>` trailer:

1. **Production-code commit** — the actual change. Prefix `feat:` /
   `fix:` / etc. as your repo's convention dictates. Must NOT include
   anything under `.tpatch/features/<slug>/`.
2. **`chore(tpatch):` metadata commit** — `.tpatch/features/<slug>/`
   only, written by `tpatch record --from HEAD~1`. Must NOT include
   any production-code edits.

The trailer records feature identity; by itself it does not provide the
four-trailer landing evidence that modern verify evaluates.

### Current recipe authority (GH #15; v0.17 planned, unreleased)

Record/embedded land is P1 of the
[seven producers](../SPEC.md#seven-governed-producers). Canonical patch,
authorized recipe/provenance and generation writes precede atomic
`recipe-capture-event.json` (**E**), then atomic `recipe-coverage.json`
(**C**) last. Commit the relevant feature artifacts together after reviewing
the reported complete/incomplete status and exact reasons. Publication is
not a cross-file transaction; a later commit does not retroactively make
an interrupted publication complete.

D16 requires **full freshly derived canonical-byte equality** to justify
record-generated provenance, not matching paths, labels or historical
origin. Differing manual/provider recipes are preserved unless complete
regeneration is explicitly authorized. Unsupported effects withhold a new
partial recipe; complete v1 admits only preimage-bearing `write-file`
operations (explicit-empty creation included) and all ten predicates.
E is unkeyed consistency evidence, not authentication/history; readers
reconstruct the proof, and C absence stays legacy even beside E.

Recipe coverage is necessary, not sufficient, for future replay eligibility; it is not cross-base safety.
A warn/exit0 coverage row is not eligibility and never grants replay permission.
Missing legacy coverage/old stale markers remain verify-green absent other
failures. Doctor D10 is read-only and warning-only even `--fix`; it offers
regeneration only when a truthful current record plan passes. Landing
trailers and attestation are independent of coverage. GH #13's new replay
consumer is future separate-release work.

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
slug via a trailer — addresses this seam for the manual fallback:

- The production-code commit is auditable on its own
  (`git log -- <files>` shows feature work without `.tpatch/` noise).
- The metadata commit is reviewable as a contract change rather than
  as part of the feature diff.
- `git rebase upstream/main` (Pattern B) handles the two commits
  cleanly because they touch disjoint paths.
- The trailer survives clones, force-pushes, and rebases, so feature
  identity persists even after history rewriting.

For current uncommitted work, `tpatch land` instead produces **one** commit
per feature, combining both halves with the four-trailer block. The split
pattern does not create that attestation automatically.

---

## 2. Path B workflow (manual fallback)

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
│     # writes canonical patch; conditionally derives recipe     │
│ 12. review coverage; use tpatch edit for bound recipe edits     │
│ 13. (optional) tpatch apply <slug> --dry-run    # preview        │
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
applied    → tpatch record --from HEAD~1   → applied
applied    → git commit (chore tpatch)     → applied (HEAD advances)
applied    → (optional) tpatch implement   → implementing (recipe checkpoint/write)
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
  the agent-authored edit window. Done writes the canonical patch for a
  non-empty ordinary capture and publishes P5 coverage; its current audit
  label is `apply`. The canonical reapply branch writes no bound artifact.
  Step 11 then records the consciously selected committed range as P1.
- **Step 11 — `record --from HEAD~1`.** Diffs the just-committed
  feature against the parent commit. Equivalent to "all changes the
  production-code commit introduced". `--auto` is also available, but
  review its inferred scope rather than substituting a broad base for a
  feature-specific range.
- **Step 12 — recipe edits.** Use
  `tpatch edit <slug> artifacts/apply-recipe.json` so P7 observes changed
  bytes and republishes coverage. Out-of-band annotations invalidate a
  present coverage binding, even if they appear semantically harmless.
- **Step 13 — `apply --dry-run`.** Previews operations without writing.
  It is not a patch-coverage check, cross-base proof or replay grant.
  Inspect the report's errors/warnings and coverage separately; unsupported
  effects are not repaired by blindly requesting regeneration.
- **Steps 9–15 — staging discipline.** Resist the temptation to
  `git add -A`. Use explicit paths. Mixing code and `.tpatch/` in one
  commit by accident is what `land`'s scoped staging avoids; an intentional
  combined feature commit through `land` is the supported default.

### Why `record` lives between the two commits

Running `record` *after* the code commit but *before* the metadata
commit is the load-bearing ordering. It means:

- `record` reads the committed code, not the working tree, so
  `post-apply.patch` is exactly the diff the production-code commit
  introduced.
- Recipe derivation, when complete and authorized, uses the same captured
  reference. Preserved recipes need not agree; coverage reports the actual
  result, and a commit boundary does not prove correspondence.
- The metadata commit is a pure follow-up that captures `record`'s
  output. No interleaving with edits.

The anti-pattern (refused since v0.4.2 — see
[`docs/record.md`](./record.md)) of running `record` on a clean tree
without `--from` is structurally avoided because step 11 always
passes `--from HEAD~1`.

---

## 3. The `Tpatch-Feature` trailer (manual identity convention)

For this manual fallback, **both commits carry**:

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
3. **Distinct from `tpatch land` attestation.** `land`
   adds three additional trailers (`Tpatch-Patch-SHA`,
   `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`) on a single combined
   commit. Historical single-trailer identity does not manufacture these
   missing hashes/base or pass modern landing-evidence checks by itself.
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

There is no `tpatch status --check-trailers` command. `land` writes the
four-trailer block; `verify` independently evaluates available landing
evidence rather than trusting a manually written identity trailer.

---

## 4. Anti-patterns

| Anti-pattern | Failure shape |
|---|---|
| `git add -A && git commit -m "feat+tpatch"` | Unscoped staging can sweep unrelated work into the feature. An intentionally scoped combined commit via `land` is supported. |
| Skipping the `Tpatch-Feature` trailer | Identity by commit-message convention only — fragile under rebase, invisible to `git log --grep`. |
| Different slugs on the two commits | Splits the feature in `git log --grep`. Almost always a typo; CI should reject. |
| Assuming a working-tree record captured an already committed feature | Choose an explicit committed range instead. Recording before commit is the supported default, including embedded `land`. |
| Running `git stash --include-untracked` mid-flow | Stashes `.tpatch/` away. On `git stash pop` the metadata can be lost or merged at the wrong base. See `docs/reconcile.md:46-72`. |
| Squashing without reviewing feature metadata and trailer evidence | Can erase the chosen boundary; coverage and landing evidence need independent review after history changes. |

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

For current uncommitted work:

| Manual fallback | `tpatch land` |
|---|---|
| Two manual commits per feature | One `tpatch land` invocation per feature |
| `Tpatch-Feature` trailer added by hand | Trailer block written by `land` (incl. SHA trailers) |
| `record --from HEAD~1` between commits | `record` embedded in `land` |
| Manual `git add` of feature paths | Safe-staging algorithm in `land` (PRD §3.3) |
| Operator chooses commit message | Subject derived from `spec.md` / `request.md` (PRD §3.4) |

Historical identity trailers remain in Git. They are not automatically
upgraded into full landing evidence or recipe authority. See
[`docs/land.md`](./land.md) for the current command contract.

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
  design contract behind the implemented `land` command.
- [`docs/whitepapers/WP-001-feature-slice-gap.md`](./whitepapers/WP-001-feature-slice-gap.md)
  §5.2 — the boundary-capture failures this convention defends
  against.
