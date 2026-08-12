# PRD — `tpatch land`: Per-Feature Git Projection — `feat-tpatch-land`

**Status**: Draft v2.1 (2026-05-09 v2 after G55 cross-review F1–F5; 2026-05-10 v2.1 cross-cite tidy-up after OX47's lock-guard PRD landed) · **§3.8 / §4.3 / §6.2 amended 2026-08-12 for v0.15.1 Wave B (GH #8) — AWAITING REVIEW. `land`'s behavior is unchanged; §3.8 is a readers' contract.**
**Date**: 2026-04-29 (v1), 2026-05-09 (v2), 2026-05-10 (v2.1), 2026-08-12 (landing-evidence readers' contract)
**Owner**: CO47
**Milestone**: v0.7 ship target per `docs/market-research/competitive-landscape.md` §6 SMART.
**Graduates from**: [`docs/whitepapers/WP-001-feature-slice-gap.md`](../whitepapers/WP-001-feature-slice-gap.md)
**Related**:
- [`docs/whitepapers/WP-001-feature-slice-gap.md`](../whitepapers/WP-001-feature-slice-gap.md) §5.2 row 7, §7 Step B, §9
- [`docs/prds/PRD-feature-dependencies.md`](./PRD-feature-dependencies.md) — DAG primitives `land` interacts with
- [`docs/prds/PRD-record-auto-base.md`](./PRD-record-auto-base.md) (G55, parallel) — implementation prerequisite; owns `status.apply.base_commit` semantics (lower bound of the captured range); shares the `LoadUpstreamLock` parser primitive with `PRD-reconcile-lock-guard §5`
- [`docs/prds/PRD-record-collision-detection.md`](./PRD-record-collision-detection.md) (G55, parallel) — implementation prerequisite
- [`docs/prds/PRD-reconcile-lock-guard.md`](./PRD-reconcile-lock-guard.md) (OX47, parallel) — third strict v0.7 deliverable; independent of `land` implementation but part of the same v0.7 cluster
- [`docs/prds/PRD-tpatch-hotfix.md`](./PRD-tpatch-hotfix.md) §3.4 — `Tpatch-CVE` trailer is additive to this PRD's four-trailer block
- [`docs/prds/PRD-patch-already-upstream-detector.md`](./PRD-patch-already-upstream-detector.md) (OX47, post-M14) — phase-1.5 fast path; downstream of the v0.7 cluster, depends on a valid `upstream.lock` (per `PRD-reconcile-lock-guard`)
- [`docs/market-research/personas.md`](../market-research/personas.md) — Platform Pat (audit JTBD), Security Sam (forwardability JTBD), Maintainer Mira (reasoning preservation)
- [`docs/market-research/competitive-landscape.md`](../market-research/competitive-landscape.md) §9 (DEP-3 / git-gud trailer precedents), §10 (what we don't adopt), §11 (deterministic apply-recipe edge)
- [`docs/record.md`](../record.md), [`docs/reconcile.md`](../reconcile.md), [`docs/feature-layout.md`](../feature-layout.md)
- [`docs/prds/PRD-verify-freshness.md`](./PRD-verify-freshness.md) §3.6, §7.1 — the landing-evidence consumer this PRD's §3.8 serves (v0.15.1 Wave B / GH #8)
- [`docs/adrs/ADR-013-verify-freshness-overlay.md`](../adrs/ADR-013-verify-freshness-overlay.md) Amendment 1 rev-5 D8–D19 — binding ADR for the readers' contract and the §3.8.6 producer precondition
- [`docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md`](../adrs/ADR-019-tpatch-land-trailer-block-schema.md) — the locked schema §3.8 makes readable
- [GH #8](https://github.com/tesseracode/tesserapatch/issues/8) — `verify` V8 fails after land

## 0. Meta

### 0.1 Implementation gating (non-negotiable)

`tpatch land` **drafting** is independent of the two guardrail PRDs.
`tpatch land` **implementation must not start** until **both**
`PRD-record-auto-base` and `PRD-record-collision-detection` ship. The
guardrails are what makes a per-feature Git projection trustworthy: a
`land` that commits a colliding or wrongly-baselined patch into Git
history would amplify the failure WP-001 documented in Cases A1/A2,
not fix it.

This is a sequencing constraint, not a scope constraint. The PRD may
mention guardrail behaviors as preconditions; it must not redefine
them.

### 0.2 Claims-audit note

Per WP-001 §3.5 expectation: every load-bearing claim about current
behavior in this PRD cites a `file:line` in current code or docs. Cites
re-verified against the post-Wave-3 / Slice-C-in-progress tree on
2026-05-09 (v2). Stale or placeholder cites from v1 (G55 review F5) have
been replaced or removed.

### 0.3 v2 changelog (2026-05-09)

Addresses G55's cross-review findings F1–F5 plus market-research
integration:

- **F1 (preflight)** — Replaced reuse of `gitutil.PreflightReconcile`
  (which refuses dirty trees) with a `land`-specific preflight that only
  checks for conflict markers / `.orig` / `.rej`. A dirty working tree
  is expected; that's what `record` is meant to capture. See §3.2.
- **F2 (base_commit)** — Removed the `apply.base_commit = new HEAD`
  postcondition. `apply.base_commit` is owned by
  `PRD-record-auto-base` and remains the lower bound of the captured
  range. The feature↔commit binding lives **only** in the
  `Tpatch-Feature: <slug>` trailer; `git log --grep '^Tpatch-Feature:
  <slug>$'` is the lookup. No SHA-of-this-commit needs to be written
  into the commit's own tracked content. See §3.4, §3.6, §6 ac.5.
- **F3 (Pattern A metadata-only)** — Dropped the sub-mode entirely;
  `record --files .tpatch/...` cannot work because
  `CapturePatchScoped` always excludes `.tpatch` before user pathspecs
  (`internal/gitutil/gitutil.go:217-227`). Pattern A v1 is "feature
  branch only"; metadata-only landing is out of scope (§9). See §4.1.
- **F4 (`--auto`)** — Added `--auto` to §3.1 flag table; clarified
  forwarding semantics to the embedded `record`.
- **F5 (cites)** — Replaced placeholder text and re-verified line
  numbers across §2.1, §3.2, §3.3, §5.

### 0.4 Non-goals (pinned)

- **Not** a replacement for `cycle <slug>` (`SPEC.md:73`,
  `internal/cli/phase2.go:24-34`). `cycle` orchestrates phases;
  `land` projects already-materialized state into Git history.
- **Not** a new operation log. WP-001 §2 ratified that repo-wide op
  history with recovery semantics is a real gap, but is not unblocked
  by `land`.
- **Not** a Git wrapper. `land` calls `git` underneath; users still
  reach for `git log`, `git show`, `git rebase` for everything Git
  already does well.
- **Not** a hidden-ref or metadata-only-commit scheme. Every commit
  `land` makes is an ordinary Git commit visible in `git log`.

---

## 1. Summary

`tpatch land <slug>` is one user-visible operation that bridges the
seam between tpatch state and Git history. It composes three steps the
operator currently performs by hand:

1. **`record`** — capture (or re-capture) the feature's canonical
   `post-apply.patch` and update `.tpatch/features/<slug>/` metadata.
2. **Safe staging** — stage *only* the files the feature touches plus
   the relevant `.tpatch/features/<slug>/` metadata. Refuse to stage
   anything outside that scope without an explicit override.
3. **One Git commit** — produce one ordinary Git commit with a
   machine-readable `Tpatch-Feature: <slug>` trailer, leaving the
   working tree clean **with respect to feature scope** (see §3.6
   for the precise post-condition; the two named global metadata
   files MAY retain unrelated operator drift with a stderr note).

That is the entire scope. `land` does not run `analyze`, `define`,
`explore`, `implement`, `apply`, or `test`. It does not push, rebase,
or merge. It produces audit-friendly Git history for one feature whose
content already exists in the working tree.

```
tpatch land extra-button
  → record (or re-record) the feature's post-apply.patch
  → stage feature-touched files + .tpatch/features/extra-button/
  → git commit -m "<subject>" with Tpatch-Feature: extra-button trailer
  → working tree clean w.r.t. feature scope; status.json:notes records the landed-at timestamp (apply.base_commit unchanged — see §3.6 / §6 ac.5)
  (the two global metadata files .tpatch/upstream.lock and .tpatch/FEATURES.md
   MAY retain unrelated operator drift — see §3.3 step 3 / §3.6)
```

---

## 2. Motivation

### 2.1 Persona grounding

Three primary personas in `docs/market-research/personas.md` all hit
this seam:

- **Platform Pat** (audit JTBD) — security/compliance asks "what did
  you change vs upstream?" `git format-patch` answers shape but not
  *why*. The `Tpatch-Feature:` trailer + `git log --grep` makes the
  fork's intent enumerable from Git history alone
  (`personas.md:64-77, 90-99`).
- **Security Sam** (forwardability JTBD) — Sam often wants the patch
  in upstream eventually; the commit must be reviewable as a PR
  without restating context for upstream maintainers. The
  trailer-block is exactly that forwarding mechanism
  (`personas.md:117-149`).
- **Maintainer Mira** (reasoning-preservation JTBD) — Mira's coding
  agent needs feature↔commit traceability the `git log` doesn't
  carry. The trailer makes feature lookup machine-readable from
  ordinary Git tools (`personas.md:179-208`).

The Common JTBD synthesis (`personas.md:239-253`) names "stays out of
the way of `git log` / `git review` muscle memory" as a primary
constraint. `land` produces an ordinary Git commit — not a hidden
ref, not a metadata-only commit — exactly to honor that.

### 2.2 The seam is sharp (current behavior)

Today the operator runs `tpatch record <slug>`, then separately
`git add`, then `git commit`. The seam between those steps produces
two known footguns:

1. **Record-after-commit anti-pattern** — `tpatch record` on a clean
   working tree without `--from` was silently producing 0-byte
   patches and advancing the feature to `applied` until v0.4.2 added
   the explicit refusal at `internal/cli/cobra.go:864-895`. The
   user-facing doc warning lives at `docs/record.md:38-50`.
2. **Stage-too-much / stage-too-little** — manual `git add -A`
   captures unrelated edits in the same commit; a manual
   `git add <paths>` may miss new files (untracked) that the feature
   created and that `record` already correctly captures via
   `git add --intent-to-add` (`internal/gitutil/gitutil.go:228-251`).

### 2.3 The boundary-capture failure documented in WP-001

WP-001 §5.2 documented two real repos (Cases A1
`tesserabox/copilot-api` and A2 `tesserabox/t3code`) where 10/11 and
11/21 features respectively share byte-identical `post-apply.patch`
files — i.e. the operator recorded the wrong scope and then committed
it into the repo's own `.tpatch/` state. WP-001 §5.2 row 11 (G55
ratified, T13) found **no data-model gap**; the fix is in `record` UX
(the two G55 guardrail PRDs) plus this Git-projection PRD.

`land`'s contribution to that fix:

- A single user-visible operation removes the "did I run record
  before commit?" decision the operator currently has to make
  manually.
- The `Tpatch-Feature: <slug>` trailer makes the feature → commit
  mapping machine-readable, which `feat-noncontiguous-feature-commits`
  (pending) can later consume to derive a per-feature commit ledger
  without convention-only inference.
- Safe staging refuses to mix unrelated edits into the feature
  commit, which addresses the post-record half of the WP-001 §5.2
  row 5 finding.

### 2.4 Prior-art validation

`docs/market-research/competitive-landscape.md` §9 anchors the
trailer-block design in two pieces of prior art:

- **DEP-3** (Debian's `Description: / Origin: / Forwarded:` header
  block, ~15 years in production) validates trailer-based identity
  as a forwardable convention. We adopt the **trailer concept**, not
  the Debian-bound fields (`competitive-landscape.md` §10).
- **git-gud** (`GG-ID`, `GG-Parent`) and **stk**
  (`.git/stacks/<name>.yaml`) use commit trailers for stable identity
  in a different lane (stacked PRs). The trailer pattern is proven
  to survive `git rebase` / `git cherry-pick` without external state.

`competitive-landscape.md` §11 entry 3 names the deterministic
apply-recipe (richer than `git format-patch` because it carries
replay semantics plus the four-trailer block) as one of three things
nobody else has. `land` is how that richer model becomes visible in
ordinary Git history.

### 2.5 What `land` does not solve

If the operator records the **wrong** patch (the WP-001 §5.2 row 1
collision shape), `land` will faithfully commit the wrong patch into
Git history — making the failure worse, not better. That is why
implementation is gated on the two guardrail PRDs. `land` is the
last mile; the guardrails are the road.

---

## 3. Proposed behavior

### 3.1 Command surface

```
tpatch land <slug> [--message <subject>] [--allow-extra-paths]
                   [--no-record] [--from <ref>] [--auto] [--files <paths>]
                   [--dry-run]
```

| Flag | Default | Purpose |
|---|---|---|
| `--message <subject>` | derived (see §3.4) | Subject line of the Git commit. |
| `--allow-extra-paths` | off | Permit staging of files outside the feature's recorded patch scope (with one-line warning per file). |
| `--no-record` | off | Skip the embedded `record` step; trust the existing `post-apply.patch` as-is. Refuses if `status.Apply.HasPatch == false`. |
| `--from <ref>` | none | Forwarded to the embedded `record`; same semantics as `tpatch record --from` (`internal/cli/cobra.go:802`, `internal/cli/cobra.go:819`). |
| `--auto` | off | Forwarded to the embedded `record`; runs `record --auto` baseline inference per `PRD-record-auto-base`. Mutually exclusive with `--from`. If both `--from` and `--auto` are supplied, refuse with the same diagnostic `record` would emit. |
| `--files <paths>` | none | Forwarded to the embedded `record`; same semantics as `tpatch record --files` (`internal/cli/cobra.go:804`, `internal/cli/cobra.go:822-830`). |
| `--dry-run` | off | Print the §3.5 dry-run contract and exit; do not modify the working tree, the index, or `.tpatch/`. |

`--commit-range <a>..<b>` is **not** exposed. If a user needs that
form they should run `tpatch record --commit-range` then
`tpatch land --no-record`. Composability beats flag surface.

### 3.2 Pre-flight (refusals)

`land` refuses (exit non-zero, no mutations) when any of the
following hold. The preflight is **deliberately narrower** than
`gitutil.PreflightReconcile` — `reconcile` requires a fully clean
tree because it replays patches, but `land` is *recording and
committing* the working tree, so unstaged and untracked files are
expected and welcome.

1. **Feature does not exist** — `status.json` missing. (Same
   precondition as `record`.)
2. **Conflict markers present** — any tracked file contains
   `<<<<<<< `, `======= `, or `>>>>>>> ` lines. Reuses
   `gitutil.HasConflictMarkers` / `gitutil.ScanConflictMarkers`
   (`internal/gitutil/gitutil.go:560-605`). Conflict markers in a
   feature commit are never the right answer.
3. **Merge leftovers present** — any `*.orig` or `*.rej` file
   anywhere in the tree (excluding `.git/`). Reuses the
   `PreflightReconcile` scan logic but **not** its dirty-tree
   refusal (`internal/gitutil/gitutil.go:120-180`).
4. **Mid-merge / mid-rebase / mid-cherry-pick** — refuse if
   `.git/MERGE_HEAD`, `.git/REBASE_HEAD`, or
   `.git/CHERRY_PICK_HEAD` exist. (Detect by file presence; no new
   gitutil helper required for v1.)
5. **`record` would refuse** — if the embedded `record` step would
   itself refuse (empty capture without `--from`/`--auto`,
   `internal/cli/cobra.go:864-895`), `land` refuses with the same
   diagnostic surfaced verbatim. No re-wrapping.
6. **Cross-feature collision detected** (gated on
   `PRD-record-collision-detection` shipping) — if the freshly
   recorded patch is byte-identical to another feature's
   `post-apply.patch`, `land` refuses with that PRD's recommended
   diagnostic. `land` does not invent a collision check.
7. **Hard-parent dep unsatisfied** — any unsatisfied hard parent in
   `depends_on` (`docs/dependencies.md:96-112`) refuses with the
   existing apply-time error. `land` reuses the dependency-system
   gate; it does not reinvent it.

**Not** a refusal:

- Unstaged modifications anywhere in the working tree.
- Untracked files anywhere in the working tree.

These are the substrate `record` is built to capture. The path-set
discipline in §3.3 is what scopes them, not the preflight.

### 3.3 Safe staging algorithm

After the embedded `record` step writes `post-apply.patch`:

1. Compute **feature path set** =
   `gitutil.FilesInPatch(post-apply.patch)`
   (`internal/gitutil/gitutil.go:741`).
2. Append the feature's metadata directory:
   `.tpatch/features/<slug>/`.
3. Compare `.tpatch/upstream.lock` and `.tpatch/FEATURES.md` content
   before vs. after the embedded `record` step (sha256 snapshot
   around step 1). For each file:
   - **Changed by `record`**: include in the path set (gets staged
     and committed as part of the feature commit).
   - **Unchanged by `record` but dirty in the working tree** (operator
     drift unrelated to this feature): emit a one-line stderr note
     (`note: leaving <path> dirty (operator drift outside feature
     scope; not staged)`) and **leave the file dirty in the working
     tree**. Do NOT stage it, even with `--allow-extra-paths`.
   - **Unchanged and clean**: ignore.

   Rationale: in shared / non-ephemeral worktrees these two global
   files routinely drift between lands; sweeping them into the
   feature commit reproduces the WP-001 §5.2 row 5 problem, but
   refusing on every drift is too rigid. Carving them out
   explicitly is bounded (two named files) and visible (the
   stderr note). See ADR-021 for the full decision record.
4. Diff the working-tree change set against the path set. If any
   path is dirty in the working tree but **not** in the path set:
   - With `--allow-extra-paths`: stage it and emit a one-line warning
     per file (`note: staging extra path foo/bar.go (not in feature
     patch); the feature commit will include this`).
   - Without `--allow-extra-paths`: refuse with the list of extra
     paths and a hint to either revert them, run `git stash`, or
     re-run with `--allow-extra-paths`.
5. `git add` the path set (using `--intent-to-add` first for
   untracked files, mirroring the `record` working-tree mode behavior
   at `internal/gitutil/gitutil.go:228-251`).

The path set is intentionally **strict**: a `land` that silently
absorbs unrelated edits is exactly the WP-001 §5.2 row 5 problem
moved one step downstream.

### 3.4 Commit message and trailer block

Subject line precedence:

1. `--message <subject>` if provided (verbatim, no munging).
2. `feature.spec.md`: parse the first H1 (`# ...`) line; truncate to
   72 chars if needed.
3. `feature.request.md`: first non-empty line; truncate to 72 chars.
4. Fallback: `feat(<slug>): apply tpatch feature`.

Body: empty by default. (Operators can amend with `git commit --amend`
afterward; `land` does not try to be a commit-message editor.)

#### The four-trailer block (locked)

`land` always emits this block, in this order, separated from the
subject/body by one blank line:

```
Tpatch-Feature: <slug>
Tpatch-Patch-SHA: <sha256 of post-apply.patch bytes>
Tpatch-Recipe-SHA: <sha256 of apply-recipe.json bytes, or "none">
Tpatch-Base-Commit: <status.json apply.base_commit>
```

Semantics:

- **`Tpatch-Feature: <slug>`** — the feature↔commit binding. This
  is the **only** place that binding lives in tracked Git state.
  `git log --grep '^Tpatch-Feature: <slug>$'` enumerates every
  commit that lands `<slug>`. Status.json does **not** record the
  landed commit (F2 fix; see §3.6).
- **`Tpatch-Patch-SHA`** — sha256 of the `post-apply.patch` bytes
  the embedded record step wrote. Audit anchor for "the patch
  reviewed equals the patch committed."
- **`Tpatch-Recipe-SHA`** — sha256 of `apply-recipe.json` bytes if
  present (the autogen output of `record`'s recipe pass,
  `internal/cli/cobra.go:961-991`); `none` otherwise. Optional
  audit anchor.
- **`Tpatch-Base-Commit`** — the lower bound of the captured range
  per `PRD-record-auto-base` §3.3. For working-tree captures this
  is the HEAD at record time; for `--from` / `--auto` captures it
  is the resolved lower bound. Always knowable **before** the new
  commit exists, so it is safe to embed.

Rationale (`docs/market-research/competitive-landscape.md` §9, §11):

- Trailer-based identity is validated by **DEP-3** (Debian, ~15 years
  upstream-forwarded) and by **git-gud** / **stk** in the stacked-PR
  lane.
- We adopt the trailer **concept**, not DEP-3's Debian-bound fields
  (`Origin: upstream`, `Forwarded:` URLs) — those are out of scope
  for our Lane A (`competitive-landscape.md §10`).
- The four trailers carry **replay semantics** (Patch-SHA,
  Recipe-SHA, Base-Commit), not just diffs — this is the
  "deterministic apply-recipe richer than `git format-patch`" edge
  named in `competitive-landscape.md §11` entry 3.

#### Coordination with `PRD-tpatch-hotfix`

`PRD-tpatch-hotfix` §3.4 adds **`Tpatch-CVE: CVE-YYYY-NNNN`** as an
**additive** fifth trailer that appears only when
`FeatureStatus.Kind == "hotfix"` and `FeatureStatus.CVE` is set.
Non-hotfix commits round-trip unchanged. `land` v1 does **not**
emit `Tpatch-CVE` itself — that is the hotfix verb's responsibility
when it delegates to `land`.

> **Cross-PRD coordination (resolved 2026-05-09)**: an earlier draft of
> `PRD-tpatch-hotfix.md` §3.4 displayed a non-canonical trailer
> example. That has since been corrected — hotfix's §3.4 now displays
> the locked four-trailer block plus the additive `Tpatch-CVE`. No
> remaining action.

#### Repo-level trailers

The repo-wide `Co-authored-by:` trailer per `CLAUDE.md` working rule
8 is appended after the four (or five) Tpatch trailers. That is a
project-policy concern, not a `land` design concern.

### 3.5 Dry-run contract

`tpatch land <slug> --dry-run` prints exactly the following to stdout
in this order, then exits 0. No mutations.

```
DRY RUN: tpatch land <slug>

Pre-flight:
  feature state         : <state>
  hard-parent gate      : ok | <error>
  working-tree hygiene  : clean | <error>
  collision check       : ok | colliding-with: <slug>

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

Carved-out global metadata (left dirty in working tree, NOT staged):
   M .tpatch/upstream.lock         (operator drift; record did not modify)
     → stderr: note: leaving .tpatch/upstream.lock dirty (operator drift outside feature scope; not staged)

Commit:
  subject               : <derived subject>
  trailers              :
    Tpatch-Feature: <slug>
    Tpatch-Patch-SHA: <sha256>
    Tpatch-Recipe-SHA: <sha256 | none>
    Tpatch-Base-Commit: <sha>

Post-conditions if you re-run without --dry-run:
  HEAD will move from <old-sha> to a new commit.
  Working tree will be clean w.r.t. feature scope (the two global
    metadata files .tpatch/upstream.lock and .tpatch/FEATURES.md
    MAY retain unrelated operator drift; see §3.6).
  Feature → commit binding: git log --grep '^Tpatch-Feature: <slug>$'
  status.json:apply.base_commit unchanged (owned by record/auto-base).
```

This contract is the §7-Step-B paper-spike contract WP-001 specified,
verbatim where possible.

### 3.6 Post-conditions

After a successful `tpatch land`:

- HEAD has advanced by exactly **one** commit.
- Working tree clean **with respect to feature scope** — the new HEAD
  commit covers the feature path set, the feature directory under
  `.tpatch/features/<slug>/` is clean, tracked source files outside
  the feature scope are unchanged, and the index is empty. The two
  global metadata files (`.tpatch/upstream.lock`, `.tpatch/FEATURES.md`)
  MAY retain unrelated operator drift if they were dirty before the
  embedded `record` step ran; in that case `land` emits a one-line
  note per file (see §3.3 step 3 and ADR-021). All other files MUST
  be clean.
- `status.json:apply.base_commit` is **unchanged** by `land` — it
  remains whatever the embedded `record` step (or `record --auto` per
  `PRD-record-auto-base` §3.3) wrote. `land` does **not** overwrite
  this field with the new HEAD. (F2 fix: a commit cannot contain its
  own SHA in tracked content; the `Tpatch-Feature:` trailer is the
  feature↔commit binding.)
- `status.json:notes` records a one-line `landed at <ts>` entry
  (mirroring the Path B `--manual` notes pattern from
  `docs/agent-as-provider.md:43-50`). The new HEAD's SHA is **not**
  written here (same F2 reason).
- `patches/NNN-record.patch` is the latest numbered audit snapshot
  (already produced by the embedded `record`).
- The new commit's `Tpatch-Feature: <slug>` trailer is the canonical
  feature↔commit binding. `git log --grep '^Tpatch-Feature: <slug>$'`
  enumerates every commit that lands `<slug>`. No tracked
  `.tpatch/`-side state needs to know the landed SHA.

### 3.7 Error recovery

`land` is **not** atomic across the three steps from the operator's
viewpoint, but it is recoverable:

- If `record` fails: nothing was staged or committed. Re-run after
  fixing.
- If staging fails: nothing was committed. The embedded `record`'s
  output (`post-apply.patch`, recipe regeneration, numbered audit
  snapshot) is already on disk. Re-running `land` is safe; the next
  `record` either no-ops or re-captures the same content.
- If `git commit` fails (e.g. pre-commit hook rejects): the index is
  staged but uncommitted. `land` prints a recovery hint:
  `tpatch land <slug> --no-record` will retry the commit step against
  the existing index.

These behaviors fall out of using `git` primitives directly. `land`
does not implement its own transaction layer.

---
### 3.8 Landing-evidence contract — v0.15.1 Wave B / GH #8 (rev-5)

> **Amendment status**: proposed rev-1, 2026-08-12, AWAITING REVIEW.
> **`land`'s behaviour is unchanged by this amendment.** This section is
> normative for *readers* of the trailer block. It exists because
> `tpatch verify` now derives a load-bearing decision from it
> (`docs/prds/PRD-verify-freshness.md` §3.6; ADR-013 Amendment 1 rev-1 D10 and
> D15), and the previously-implicit reader rules must become explicit before a
> consumer depends on them. Binding schema: ADR-019.
>
> **rev-2 → rev-3** adds rules 18–19 (derived commit-id length; shallow and
> partial history are not root topology) and a **new §3.8.6 producer
> precondition**: `land` must refuse before committing when
> `status.apply.base_commit` is not valid for the reader grammar. Everything
> else about `land` is unchanged.
>
> **rev-1 → rev-2** added rules 15–17: the conservative raw-precedence rule
> (a slug-bearing line Git does not parse as a trailer is *malformed*, never
> *absent*), the ordering rule (artifact absence is evaluated before any digest
> comparison), and the **attestation-vs-anchor separation** — a reader that
> needs a pre-landing tree must select an anchor candidate independently of the
> commit that carries the current attestation. Every claim below was measured;
> the probe index is ADR-013 §A1.1 E1–E33.

#### 3.8.1 What the trailer block attests

The four-trailer block emitted at `internal/cli/land.go:397-400` is an
**attestation**, not an index. It asserts: *at the moment this commit was
created, feature `<slug>`'s canonical `post-apply.patch` hashed to
`<Tpatch-Patch-SHA>`, its `apply-recipe.json` hashed to
`<Tpatch-Recipe-SHA>` (or was absent/whitespace-only, `none`), and `record`
had resolved its base to `<Tpatch-Base-Commit>`.*

It does **not** assert that the content is still in the tree. A `git revert`
of a landing leaves the commit reachable with its attestation intact while
removing the content — measured. Any consumer that treats reachability as
presence is wrong; verify pairs the attestation with an independent
materialization ladder (`PRD-verify-freshness` §3.6.5).

It also does **not** assert *where* the content is. Reverse-applying the
attested patch succeeds against any tree containing equivalent content,
including one produced by an unrelated actor — measured. Reverse-apply is a
materialization signal, never ownership.

#### 3.8.2 Reader rules (normative)

1. **`git log --grep '^Tpatch-Feature: <slug>$'` is a prefilter, not
   authority.** It matches any commit whose *message text* contains that line,
   including a docs commit that quotes it in prose. The authoritative value is
   `git log --format='%(trailers:key=Tpatch-Feature,valueonly)'`, which yields
   nothing for such a commit. `docs/land.md` and §3.4 advertise the `--grep`
   form as a *human* convenience; tooling must not stop there.
2. **Git parses trailers only from the last paragraph.** A `git commit
   --amend` (or a `commit-msg` hook) that appends a paragraph after the block
   makes the trailers unreadable even though the bytes remain visible in
   `git log`. A reader must therefore **retain the raw message** and classify
   this as *malformed attestation*, distinct from *no attestation*. A
   parsed-only reader cannot tell the two apart.
3. **The slug is matched by exact string equality**, after trimming leading
   and trailing ASCII space/tab. `my-slug` must never match
   `my-slug-extended`. The anchored `--grep` form already behaves this way;
   the unanchored form does not.
4. **`Tpatch-Feature` cardinality is exactly one.** `land` emits exactly one
   and ADR-019 admits no other producer. A commit carrying two or more values
   — a hand-rolled squash of two landings — is **malformed** for reading
   purposes: its sibling `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` /
   `Tpatch-Base-Commit` cannot be attributed to a specific slug.
5. **`Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA` and `Tpatch-Base-Commit`
   cardinality is exactly one each.** Duplicates are observable — two
   `Tpatch-Patch-SHA` lines are returned as `aaaa,bbbb` — and a "take the
   first" parser would silently pick a convenient one. Duplicates are
   **malformed**; no reader may select among them.
6. **Value formats are strict and lowercase.** `Tpatch-Patch-SHA` is 64
   lowercase hex and `Tpatch-Recipe-SHA` is 64 lowercase hex or the literal
   `none` — both are SHA-256 digests of artifact bytes and are independent of
   the repository object format. `Tpatch-Base-Commit` is **`N` lowercase hex
   where `N` is derived from `git rev-parse --show-object-format`** — 40 for
   `sha1`, **64 for `sha256`** (rule 18). Anything else is malformed. Lowercase
   strictness follows the ADR-029 D1 precedent already enforced by
   `isLowercaseHex` (`internal/workflow/writefile_safety.go:176`).
7. **Trailer keys are matched case-insensitively by Git.**
   `tpatch-feature: <slug>` is returned by
   `%(trailers:key=Tpatch-Feature,valueonly)` — measured. A reader inherits
   this and the contract states it rather than claiming a case-sensitive match
   it cannot implement.
8. **The slug key is not unique across commits** (already in ADR-019 →
   Consequences). Re-landing after a re-`record` produces a second commit with
   the same slug and different SHA fields. Readers must define a deterministic
   selection rule; verify's is `PRD-verify-freshness` §3.6.8.
9. **Reachability is full-graph.** A landing merged from a side branch is
   reachable only through a non-first parent — measured. Readers must not
   scope with `--first-parent`.
10. **Topology matters: `land` emits single-parent commits.** A reader that
    needs the pre-landing tree must take the commit's **single** parent and
    must **refuse** rather than approximate when the commit has 0 parents (a
    root landing — `git rev-parse <root>^` fails outright) or ≥2 (a merge
    carrying the trailer, whose trailer parses normally). Both shapes were
    measured. `^1` is not a defensible approximation.
11. **Trailers survive `rebase` and `cherry-pick` verbatim**, while the commit
    SHA and the parent change and `Tpatch-Base-Commit` may become unreachable.
    Readers must not key on the commit SHA or on the commit's parent identity,
    and must treat base unreachability as advisory.
12. **`Tpatch-Recipe-SHA: none` is a value, not an absence**, and it covers
    **two** producer cases: no `apply-recipe.json`, *and* one that is
    whitespace-only — `readRecipeSHA` returns `none` on a read error and on
    `strings.TrimSpace(...) == ""` (`internal/cli/land.go:1034-1043`). A
    reader comparing against a repo in either state must treat `none == none`
    as a match.
13. **Patch-digest comparison is presence-aware.** An **absent**
    `post-apply.patch` has no digest, so any attested value mismatches. A
    **present, zero-byte** patch hashes to `sha256("")` and must compare
    equal. Absent is not empty.
14. **A reader error is not an absence.** If the enumeration cannot be run or
    parsed — a git failure, or a git below the reader's floor — the reader must
    surface an **error state**, never "no evidence". Degrading to absence
    converts an unknown into a positive claim. The **effective floor for a
    reader that verifies offline is git ≥ 2.36**, set by `GIT_NO_LAZY_FETCH`;
    the component capabilities (`%(trailers:key=…,valueonly)` 2.22,
    `separator=` 2.25, `--show-object-format` 2.29) are historical facts about
    individual features, not the operative floor.
15. **A slug-bearing line that Git does not parse as a trailer is
    *malformed*, never *absent*.** A prose paragraph that quotes
    `Tpatch-Feature: <slug>` and a trailer block destroyed by a later
    `--amend` are measurably **indistinguishable** from the outside: both yield
    an empty parsed value while the raw `%B` still contains the line. A
    conservative reader classifies **both** as malformed and accepts the
    resulting false red on the prose case, because reading a destroyed
    attestation as "never attested" is the unsafe direction.
16. **Artifact absence is evaluated before any digest comparison.** If the
    feature's `post-apply.patch` is absent there is no digest to compare, and a
    reader must report absence rather than a mismatch. Only a **present**
    artifact is hashed; a present zero-byte artifact hashes to `sha256("")`.
17. **The commit that carries the current attestation is not necessarily the
    commit that can supply a pre-landing tree.** After a re-record and re-land
    the newest landing's parent already contains the feature — measured — so a
    reader that needs a baseline must select that candidate **independently**,
    from among exact-slug single-parent landings whose parent does *not*
    already contain the feature, and must not assume the two coincide. Their
    own recorded hashes may be stale; they are not an authority, only a
    baseline source.

18. **The accepted commit-id length is derived, not assumed.**
    `Tpatch-Base-Commit` is `N` lowercase hex where `N` comes from
    `git rev-parse --show-object-format` — measured as **40** for `sha1` and
    **64** for a repository created with `--object-format=sha256`. A reader
    that hardcodes 40 rejects every valid landing in a SHA-256 repository. If
    the object format cannot be read, the reader must report an error state
    rather than guess.
19. **Zero parents does not mean "root" in a shallow clone.** Measured: in a
    `--depth 2` clone the graft-boundary commit reports **0 parents** in `%P`
    exactly like a real root, is marked `(grafted)`, and
    `git read-tree <boundary>^` fails with the *same*
    `fatal: Not a valid object name` text. A reader must run
    `git rev-parse --is-shallow-repository` (and/or check `.git/shallow`
    membership) **before** classifying topology, and must distinguish a
    partial clone (`remote.<name>.promisor`) whose objects are fetched lazily.
    Telling an operator to re-land when the actual fix is
    `git fetch --unshallow` is a wrong remediation, and a CI shallow checkout
    is the common case.

#### 3.8.3 No new status metadata — decision

`land` persists **no** landing identifier, and this amendment does not add
one. Considered and rejected:

| Option | Verdict |
|---|---|
| `status.landed_commit` (or a `landings[]` array) on `FeatureStatus` | **Rejected.** ADR-019 already refused writing the commit SHA back (a commit cannot contain its own SHA without a rewrite cycle). Beyond that: `rebase`/`cherry-pick` would stale the field at exactly the moment the trailers stay correct — measured; it would need a migration for every repo landed since v0.8.0; and it would create a second source of truth for a derived decision, against ADR-011 D6 / ADR-010 D5. |
| A `status.landed_at` timestamp | **Rejected.** `status.json:notes` already records `landed at <ts>` (§3.6, `internal/cli/land.go:357`) and no consumer needs it as structured data. |
| A new `Tpatch-Landed-At` trailer | **Rejected.** ADR-019 locks the block; a new mandatory trailer is a breaking schema change requiring its own ADR, and the commit's author date already carries it. |
| A `Tpatch-Landed-Parent` trailer recording `L^` explicitly | **Rejected.** It would make the §3.8.2 rule 10 topology check unnecessary, but it is redundant with `%P`, which the reader already obtains in the same enumeration, and it would stale under rebase exactly like a stored SHA. |
| A `Tpatch-Landing-Generation` counter to order re-landings | **Rejected.** The enumeration's `--topo-order --reverse` already provides a deterministic order (rule 17), and a counter would need a migration for every repo landed since v0.8.0 while staling under history rewrites. |
| Keeping the existing four trailers as the sole attestation | **Chosen.** Zero migration; retroactively valid for every feature landed since v0.8.0; degrades honestly — a hand-rolled `git commit` with no trailers yields "no evidence" and today's verify behaviour. |

**Compatibility.** Because nothing is added, there is no migration, no
schema-version bump on `status.json`, and no backfill. Repos that landed
features before this amendment are fully supported: their commits already
carry the four trailers.

#### 3.8.4 `status.apply.base_commit` stays unchanged — reaffirmed with new rationale

§3.6 already locks that `land` does not overwrite `status.apply.base_commit`.
This amendment **retains** that decision and adds a second, independent reason
to the original F2 chicken-and-egg one:

`verify` validates `Tpatch-Base-Commit` against `status.apply.base_commit`
(`PRD-verify-freshness` §3.6.2). If `land` overwrote that field with the new
HEAD, then immediately after every `land` the on-disk value would differ from
the value just written into the trailer, and **every landed feature would be
classified as evidence-`stale` the instant it landed** — the amendment would
fail on its own first test. It would also break `record` auto-base
resolution, which owns the field (ADR-016).

Note that `Tpatch-Base-Commit` is deliberately **not** the same thing as the
landing commit's parent, even though the two coincide in the simple case
(measured). Rebase and cherry-pick preserve the trailer while rewriting the
parent, so a reader must validate the trailer against `status.apply.base_commit`
and derive the pre-landing tree from `%P` — two independent facts, neither
substitutable for the other.

The field remains owned by `record` / auto-base resolution. `land` reads it
(`internal/cli/land.go:394`) and never writes it.

#### 3.8.5 Residual — hook-appended prose (non-blocking follow-up)

A `commit-msg` / `prepare-commit-msg` hook that appends a paragraph after the
trailer block silently destroys trailer readability (rule 2). `land` could
detect this after the commit returns — it already re-reads the commit for its
audit (§3.3) — and warn.

**Disposition (rev-1): non-blocking follow-up, not a Wave C obligation.**
Verify already classifies the result as `malformed` and emits a remediation
naming `git commit --amend` (`PRD-verify-freshness` §3.6.9 R7), so the
operator is told precisely, at the next verify. A `land`-side warning would
move the signal earlier but cannot change any verdict, and `land` is
behaviour-frozen by §6.2 AC-LD9. Tracked as `PRD-verify-freshness` §6 Q13.

#### 3.8.6 Producer validation — `Tpatch-Base-Commit`, by invocation mode (rev-5)

**This is the one place §3.8 changes `land`'s behaviour, and it only adds a
refusal.** `land` reads `status.Apply.BaseCommit` and interpolates it into the
trailer block with **no validation** (`internal/cli/land.go:394`, `:397-400`).
On a legacy or corrupt status the emitted trailer can be
`Tpatch-Base-Commit: ` with an empty value, which the §3.8.2 grammar
classifies as **malformed** at every future read.

> **rev-3 said "refuses before the commit, mutating nothing". That is
> unkeepable for the embedded-`record` mode and is withdrawn.** `land` runs
> `record` first; `record` writes `post-apply.patch`, may regenerate
> `apply-recipe.json`, and may write a numbered audit snapshot — all before
> `land` has any post-record value to inspect. The contract below states the
> boundary honestly instead of over-promising.

**Validity predicate** (both modes). `status.Apply.BaseCommit` is valid iff:

1. **non-empty**;
2. **well-formed** — `N` lowercase hex where `N` derives from
   `git rev-parse --show-object-format` (rule 18: 40 for `sha1`, 64 for
   `sha256`); derived, never hardcoded;
3. **resolvable** — `GIT_NO_LAZY_FETCH=1 git rev-parse --verify <base>^{commit}`
   succeeds;
4. **reachable from `HEAD`** — `gitutil.IsAncestor`
   (`internal/gitutil/gitutil.go:828`), **unless** the repository is shallow or
   a partial clone (rule 19), in which case unreachability is a one-line `warn`
   and the landing proceeds.

##### Mode A — `--no-record` (`internal/cli/land.go:66`, `:98`)

`land` does not run `record`, so the field is whatever is already on disk.

**Ordering against the shipped sequence.** `runLand`
(`internal/cli/land.go:76`) already performs, in order: store open and
`LoadFeatureStatus` (`:85`), the `unapplied` refusal (`:89`), the dry-run
branch (`:93`), `landPreflight` (`:110`), `CheckDependencyGate` (`:116`), then
**`recoverLand`** (`:127`, defined at `internal/cli/land_journal.go:437-445`).
Base-Commit validation is inserted **immediately after `recoverLand` returns**
and **before** the pre-record gate, the metadata snapshot, and every
`land`-owned mutation.

**`recoverLand` remains first — the one documented exception.** GH #7 made
crash recovery mandatory before anything else mutates record, status or the
index, and recovery is not read-only: with a pending journal it may publish a
retained index, compare-and-swap the branch back to the pre-land commit, and
restore the `status.json` preimage. That work completes a **prior, interrupted
transaction**; it is not this invocation's.

The guarantee is therefore stated in two cases:

| Case at command entry | Guarantee on an invalid `BaseCommit` |
|---|---|
| **no pending journal** — `recoverLand` is a no-op | `land` refuses with **R23** having made **no mutation whatsoever**: no commit, no index change, no `status.json` write, no artifact. |
| **pending journal** — `recoverLand` completes or refuses the prior transaction | `recoverLand` **may have mutated** the index, the branch ref and/or `status.json` **while finishing that earlier land**. `land` then refuses with **R23** having made **no NEW mutation for this invocation**. The refusal names the completed recovery. |

**Absolute command-entry immutability is not promised.** The shipped
recovery-first ordering forbids it, and that ordering is correct: leaving an
interrupted land unfinished to preserve a weaker promise would be worse.

##### Mode B — embedded `record` (the default)

The field is `record`'s output and cannot be validated before `record` runs.

- **`record` owns the producer invariant.** `record` must guarantee it writes a
  valid `Apply.BaseCommit` — non-empty, object-format-correct, resolvable and
  reachable — **before its own first mutation**. `record` already resolves the
  base up front, including the `--auto` path (`PRD-record-auto-base` §3.3), so
  this is a precondition it is positioned to enforce.
- **`land` re-validates immediately after `record` returns**, on the reloaded
  status, **before any `land`-owned mutation** — before the `landed at` note
  (`internal/cli/land.go:357`), before staging, before the commit.
- **On failure `land` refuses with R23, and `record`'s artifacts persist**,
  because `record` completed as an **independent, already finished
  transaction**. The refusal message says so. `land` itself has still mutated
  nothing: no commit, no index change, no `landed at` note.

**Why not pre-validate the old value in Mode B.** `record` is expected to
replace it: a stale-but-invalid value would refuse a landing that would have
succeeded, and a stale-but-valid value would not protect against `record`
producing an invalid one. Only the actually-produced value is worth checking.

##### The refusal

> **R23** — `land refuses: status.apply.base_commit is <empty|malformed|unresolvable> (<value>); the Tpatch-Base-Commit trailer would be unreadable. Run tpatch record <slug> --auto (or --from <base>) to repopulate it, then re-run tpatch land <slug>`
>
> In **Mode B** the message additionally states: `note: the embedded record step already completed; its artifacts under .tpatch/features/<slug>/artifacts/ are retained and are not rolled back`.
>
> In **Mode A with a pending journal** it additionally states: `note: an interrupted previous land was recovered before this refusal; that recovery is complete and is not rolled back`.

**Migration.** The affected population is features recorded before `record`
populated `apply.base_commit`, and `--no-record` landings against a status that
never had it. The remediation is a single `tpatch record` invocation; no data
is lost and no artifact is rewritten.

**`status.apply.base_commit` is still never written by `land`.** §3.6 and
ADR-019 stand: this is a precondition, not a writeback, and the field remains
owned by `record` / auto-base resolution (ADR-016). Every pre-existing `land`
refusal, its ordering and message, and the entire **successful** path are
byte-unchanged.

**Object-format assumption, stated honestly.** Both the producer here and the
reader in rule 18 derive the accepted length from
`git rev-parse --show-object-format` and fail closed if it cannot be read.

---

## 4. Compatibility with reconcile patterns

Both patterns documented in `docs/reconcile.md:18-44` must continue
to work. `land` is designed to be a no-op for one and a strict
improvement for the other.

### 4.1 Pattern A — Pristine main, features as patches

In Pattern A, `main` mirrors upstream and features live only as
`.tpatch/features/<slug>/artifacts/post-apply.patch` (plus optional
`apply-recipe.json`). `.tpatch/` is committed so feature state
travels with the branch.

`land` in Pattern A is **feature-branch only** in v1:

- The operator runs `land` on a feature branch, not `main`. `land`
  produces one feature-branch commit containing both the code edits
  and the `.tpatch/features/<slug>/` metadata. `main` stays
  pristine. The branch can then be discarded or kept as a
  "feature-as-commit" snapshot for tooling like
  `feat-noncontiguous-feature-commits` to consume.
- **Metadata-only landing onto `main`** (a previously-considered
  sub-mode) is **out of scope** in v1 (G55 review F3): the
  `record --files .tpatch/...` approach cannot work because
  `CapturePatchScoped` always excludes `.tpatch` before user
  pathspecs (`internal/gitutil/gitutil.go:217-227`). A separate
  metadata-only landing path would require its own staging
  primitive and its own PRD; do not add it to v1.

For Pattern A users who today maintain `.tpatch/` metadata as
ordinary Git commits on `main`, the existing manual flow
(`git add .tpatch/features/<slug>/ && git commit`) continues to
work unchanged. `land` does not replace it; it just doesn't help.

### 4.2 Pattern B — Features as commits

In Pattern B, feature edits live as normal Git commits on `main` and
`.tpatch/` is committed alongside for auditability.

`land` in Pattern B is the **default** path:

- One commit per feature, code + metadata together.
- The `Tpatch-Feature: <slug>` trailer formalizes a convention
  Pattern B users today maintain by hand (commit message
  conventions, branch names, etc.).
- After `land`, `git rebase upstream/main` continues to work
  exactly as `docs/reconcile.md:38-44` documents, and reconcile's
  audit role is unchanged.

### 4.3 Verification post-`land`

After `land`, the following must remain true and explainable:

- `tpatch status <slug>` reports `state=applied` and an
  `apply.base_commit` value that reflects the captured range's
  lower bound (per `PRD-record-auto-base` §3.3) — **not** the new
  HEAD.
- `git log --grep '^Tpatch-Feature: <slug>$'` returns at least one
  commit; the most recent is the just-created landing commit.
- `tpatch record --from <prev-base>` re-captures a patch identical
  to the one the embedded record produced (round-trip).
- `tpatch reconcile` against an unchanged upstream produces the same
  verdict it would have produced if the operator had run
  `record + git add + git commit` by hand.

These are acceptance criteria, not implementation details (see §6).

**Amended 2026-08-12 (v0.15.1 Wave B / GH #8).** A fifth post-`land`
invariant is added, and it did **not** hold before the amendment:

- `tpatch verify <slug>` produces the **same verdict immediately after
  `land` as it did immediately before**, for an otherwise unchanged feature.

`land` is a projection of already-applied state into Git history; it changes
where the content lives, not whether the feature is healthy. Before the
amendment this invariant failed for every recipe shape — `write-file`
recipes passed V7 vacuously and failed V8, `replace-in-file` recipes failed
V7 outright, and `append-file` recipes passed V7 only by double-appending
into the shadow. The fix is entirely on the verify side
(`docs/prds/PRD-verify-freshness.md` §3.6); `land` itself is unchanged. The
executable rows are `PRD-verify-freshness` §7.1 AC-L1–AC-L6.

---

## 5. Boundary with `cycle`

`cycle <slug>` (`internal/cli/phase2.go:24-34`,
`SPEC.md:73`) runs the full
`analyze → define → explore → implement → apply → record` sequence in
one batch with optional `--interactive` pauses and `--skip-execute`
gating.

`land` and `cycle` compose; they do not overlap.

| Question | `cycle` answer | `land` answer |
|---|---|---|
| Generates `analysis.md`? | yes | no |
| Generates `spec.md`? | yes | no |
| Generates `exploration.md`? | yes | no |
| Generates `apply-recipe.json`? | yes (via `implement`) | no (only via embedded `record`'s autogen, `internal/cli/cobra.go:961-991`) |
| Mutates working-tree code? | yes (via `apply --mode execute`) | no — assumes code is already in place |
| Captures `post-apply.patch`? | yes (via `record`) | yes (via embedded `record`) |
| Stages files? | no | yes |
| Creates a Git commit? | no | yes |

Composition:

```
tpatch cycle extra-button --skip-execute     # phases 1-4
# operator implements / Path-B-edits
tpatch cycle extra-button                     # apply + record
tpatch land  extra-button                     # stage + commit
```

`cycle` is *not* extended to absorb `land`. If a future user wants
"do everything including the commit" we expose it as a flag on
`cycle` (e.g. `cycle --land`) that delegates to `land`. We do not
duplicate `land`'s logic inside `cycle`. **If `land` ever grows
phase orchestration, it has merged into `cycle` and should be folded
back in.**

---

## 6. Acceptance criteria

A `land` implementation is acceptable when **all** of the following
hold. Numbered for the supervisor's checklist.

1. `tpatch land <slug>` produces exactly one Git commit on success.
2. The commit contains a `Tpatch-Feature: <slug>` trailer.
3. The commit contains the four trailers in §3.4 (Patch-SHA,
   Recipe-SHA, Base-Commit, Feature) in the documented order.
4. The commit's diff equals exactly the union of: (a) the files in
   the feature's `post-apply.patch`, (b) the relevant
   `.tpatch/features/<slug>/` metadata files. No extras (without
   `--allow-extra-paths`).
5. After success, `tpatch status <slug>` reports `state=applied`. The
   `apply.base_commit` field is **unchanged by `land`** — it remains
   the value the embedded `record` step (or `record --auto` per
   `PRD-record-auto-base` §3.3) wrote, which is the lower bound of
   the captured range, not the new HEAD. (F2: a commit cannot embed
   its own SHA; the `Tpatch-Feature:` trailer carries the
   feature↔commit binding instead.)
6. After success, the working tree and index are clean **with
   respect to feature scope** (`git status --porcelain` shows at
   most operator-drifted `.tpatch/upstream.lock` and/or
   `.tpatch/FEATURES.md` entries that were dirty before the embedded
   `record` step ran; see §3.3 step 3, §3.6, and ADR-021). All
   other tracked files, the feature directory, and the index MUST
   be clean. Each carved-out file produces a one-line stderr note.
7. `git log --grep '^Tpatch-Feature: <slug>$'` returns the
   landing commit; this is the canonical feature↔commit binding
   for any consumer (audit, `feat-noncontiguous-feature-commits`,
   future `tpatch list`).
8. `tpatch record <slug> --from <previous-HEAD>` re-captures a patch
   byte-identical to the patch produced by the embedded record step.
9. `tpatch reconcile` against an unchanged upstream produces the
   same verdict as the manual `record + git add + git commit`
   workflow would have produced.
10. `land` works in **Pattern A feature-branch mode** and **Pattern B
    default mode** per `docs/reconcile.md:18-44`. Pattern A
    metadata-only landing is explicitly out of scope for v1 (§4.1,
    §9).
11. `land --dry-run` produces the §3.5 contract output and exits 0
    with zero mutations to the working tree, index, or `.tpatch/`.
12. All seven §3.2 refusals are tested and surface the correct
    diagnostic. Notably: dirty working trees with **unstaged or
    untracked files** are **not** refused (§3.2 explicit
    non-refusal); only conflict markers, merge leftovers, and
    mid-merge state refuse.
13. Cross-feature collision detection from
    `PRD-record-collision-detection` is honored; `land` refuses
    when that PRD's check fires.
14. `--auto` baseline inference from `PRD-record-auto-base` is
    honored when forwarded; `--from` and `--auto` are mutually
    exclusive on the `land` command line, mirroring `record`.
15. The hard-parent dep gate from
    `docs/dependencies.md:96-112` blocks `land` with the same
    diagnostic the existing apply-time gate uses.
16. Documentation: `docs/land.md` exists with the full command
    contract; `docs/record.md`, `docs/reconcile.md`, and
    `docs/feature-layout.md` cross-link to it where relevant.
17. Skill files (`assets/skills/**`) reference `tpatch land` as the
    recommended way to commit feature work, replacing the manual
    `record + git add + git commit` instructions; the parity guard
    test in `assets/assets_test.go` continues to pass.

### 6.1 v0.7 SMART target

Per `docs/market-research/competitive-landscape.md` §6 SMART, v0.7
ships **four** deliverables:

1. `tpatch land` (this PRD).
2. `tpatch record` collision-detection (`PRD-record-collision-detection`).
3. `tpatch reconcile` upstream-lock validation guard
   (`PRD-reconcile-lock-guard`).
4. `tpatch record --auto` (`PRD-record-auto-base`) — positioned as
   the **remediation** mechanism, not a fourth shipping primitive in
   the strict SMART reading.

Success criteria from `competitive-landscape.md §6`: zero new
collisions in either WP-001 case-study repo for any feature recorded
post-v0.7 (audited at v0.7+30 days), and ≥50% of v0.6 collision-group
features re-recorded with `--auto`.

`land`'s implementation is gated on (1)+(2)+(4) shipping per §0.1.
The `reconcile` guard (3) is implementation-independent of `land` —
they cover different verbs and can ship in either order.

### 6.2 Landing-evidence acceptance rows — v0.15.1 Wave B / GH #8 (rev-5)

These rows are **`land`-side** obligations of the readers' contract in §3.8.
They assert that the evidence substrate `tpatch verify` now depends on is
actually produced, and — critically — that `land`'s **behaviour is
unchanged**. The verify-side matrix lives in
`docs/prds/PRD-verify-freshness.md` §7.1 (AC-L1–AC-L134) and is not
duplicated here.

| # | Criterion | Tier |
|---|---|---|
| AC-LD1 | `land` emits all four trailers in the ADR-019 order on every successful landing, and **Git parses them** — `%(trailers:key=Tpatch-Feature,valueonly)` is non-empty — not merely that the text appears in `git log`. Existing coverage: `internal/cli/land_test.go:145-148`, `:203-206`. | C |
| AC-LD2 | Each of the four keys appears **exactly once**. Adversarial against §3.8.2 rule 5: a landing must never produce a duplicate that a reader would have to disambiguate. | C |
| AC-LD3 | `Tpatch-Patch-SHA` equals `sha256` of the canonical `artifacts/post-apply.patch` bytes, and `Tpatch-Recipe-SHA` equals `sha256` of `artifacts/apply-recipe.json` or the literal `none`. Existing coverage: `internal/cli/land_test.go:465-496`. | C |
| AC-LD4 | `Tpatch-Recipe-SHA` is `none` in **both** producer cases — recipe absent, and recipe present but whitespace-only — matching `readRecipeSHA` (`internal/cli/land.go:1034-1043`). Pins §3.8.2 rule 12. | C |
| AC-LD5 | All emitted hex values are **lowercase** and of the documented length (64 for the two digests, 40 for the base commit), so §3.8.2 rule 6's strict reader never rejects a commit `land` produced. | C |
| AC-LD6 | `Tpatch-Base-Commit` equals `status.apply.base_commit` at commit time (`internal/cli/land.go:394`). | C |
| AC-LD7 | `land` does **not** write `status.apply.base_commit` (§3.6, §3.8.4). Byte-comparison of the field before and after a successful `land`. | C |
| AC-LD8 | `land` adds **no** new field to `status.json`. The `.tpatch/` diff produced by a successful `land` is exactly the existing §3.6 set — no landing identifier, no timestamp field (§3.8.3). | C |
| AC-LD9 | The landing commit has **exactly one parent** in every supported invocation, so §3.8.2 rule 10's single-parent requirement is satisfiable by construction. If any reachable invocation can produce a root or merge landing, this row documents it explicitly rather than leaving readers to discover it. | C |
| AC-LD10 | The trailer block is the **last paragraph** of the commit message, so `%(trailers:…)` parses it. Adversarial: a fixture repo with a `commit-msg` hook that appends prose demonstrates the failure mode §3.8.2 rule 2 describes, and confirms `land` itself does not introduce it. | C |
| AC-LD11 | Landing a feature twice (re-`record` then re-`land`) produces two commits with the same `Tpatch-Feature` value and different SHA fields, and `land` does not refuse on that basis. Pins the not-unique-key property §3.8.2 rule 8 depends on. | C |
| AC-LD12 | `land` refuses when the embedded `record` would capture nothing, so a landed feature with an absent or zero-byte `post-apply.patch` is a corruption/hand-edit case rather than a normal outcome. This is the fact `PRD-verify-freshness` §5 relies on when it classifies those rows. | C |
| AC-LD13 | A feature landed **before** this amendment (fixture from a v0.8.0-era repo) is readable under the §3.8.2 rules with no migration. Retroactive-validity guard. | C |
| AC-LD14 | `land --dry-run` still prints the trailer block and mutates nothing (existing §3.5 contract, unchanged). | C |
| AC-LD15 | **Every pre-existing** `land` refusal, diagnostic string, exit code and staging-plan line is unchanged, and their relative ordering is unchanged. Golden-output comparison against the pre-amendment binary **over inputs whose `status.apply.base_commit` is valid**. The amendment adds exactly one new refusal (§3.8.6) and alters nothing else; it is **not** claimed to be behaviour-neutral in the presence of an invalid base commit. | C |
| AC-LD16 | Two successive landings of the same slug (re-`record` then re-`land`) leave the **earlier** landing reachable and single-parent, so a reader can still derive a pre-landing tree from it after the newer landing's parent has absorbed the feature. Pins the substrate §3.8.2 rule 17 and `PRD-verify-freshness` §3.6.8 depend on. | C |
| AC-LD17 | `land` never emits a trailer value outside the §3.8.2 rule 6 formats — no uppercase hex, no wrong-length digest, no `Recipe-SHA` that is neither 64 lowercase hex nor `none` — so a strict reader never rejects a commit `land` produced. | C |
| AC-LD18 | **Mode A, no pending journal**: with an empty, malformed or unresolvable `status.apply.base_commit` and **no land journal present**, `land` refuses immediately after the no-op `recoverLand`, emitting **R23** verbatim and **mutating nothing** — no commit, no index change, no `status.json` write, no artifact. Asserted by hashing `.tpatch/`, the index and the worktree before and after. | C |
| AC-LD18a | **Mode B (embedded `record`)**: `land` re-validates the reloaded value **immediately after `record` returns and before any `land`-owned mutation**, refuses with **R23** plus the retained-artifacts note, and leaves **no** commit, **no** index change and **no** `landed at` note — while `record`'s artifacts **do** persist. The row asserts both halves; it must **not** claim "mutating nothing". | C |
| AC-LD18b | `record` writes a valid `Apply.BaseCommit` **before its own first mutation**, so Mode B's post-record validation passes for every well-formed workspace. Adversarial: a fixture that forces `record` to produce an invalid value must fail this row rather than silently landing. | C |
| AC-LD18c | **Mode A, pending journal**: with a land journal present **and** an invalid `BaseCommit`, `recoverLand` still runs **first** and completes or refuses the prior transaction; `land` then refuses with **R23** plus the recovery note, having made **no NEW mutation of its own**. The row asserts (a) recovery ran, (b) no commit and no `landed at` note were written by this invocation, and (c) the row does **not** claim absolute immutability. | C |
| AC-LD19 | The accepted base-commit length is **derived** from `git rev-parse --show-object-format`: a **SHA-256** repository accepts a 64-hex base and refuses a 40-hex one; a SHA-1 repository does the inverse. Two fixtures; a hardcoded 40 fails this row. | C |
| AC-LD20 | In a **shallow** or **partial** clone, a base commit that is well-formed and resolvable but not reachable from `HEAD` produces a one-line `warn` and the landing **proceeds** — unreachability alone never refuses. | C |
| AC-LD21 | `land`'s **successful** path is byte-unchanged: for a feature with a valid base commit, every trailer, message, exit code and staging-plan line is identical to the pre-amendment binary. Together with AC-LD15 this bounds the amendment to exactly one added refusal. | C |
| AC-LD22 | Every `land`-side git invocation added by §3.8.6 carries `GIT_NO_LAZY_FETCH=1`, so validation never reaches the network. Asserted by a `PATH` git wrapper recording the call environment. | C |

**Matrix size: 25 numbered criteria (AC-LD1 … AC-LD22, including AC-LD18a, AC-LD18b and AC-LD18c), all tier C.** Most are
already satisfied by shipped behaviour and existing tests; they are listed so
Wave C proves the substrate rather than assuming it. AC-LD21 is the guard that `land`'s successful path is untouched, and AC-LD18
is the guard that it can no longer produce evidence the reader must reject.


## 7. Open questions

These are deliberately unresolved. Each will be answered during
implementation, in a follow-up PRD revision, or in the ADR that
locks `land`'s behavior.

### 7.1 `land` vs `commit` aliasing  *(deferred per WP-001 T15)*

Three options:

1. **`land` only.** Distinct verb. Pro: doesn't pretend to be Git;
   no collision with `git commit` muscle memory. Con: another
   command to remember.
2. **`commit` as an alias for `land`.** Help text explains. Pro:
   discoverable for Git-native users. Con: implies tpatch is a Git
   wrapper, which it isn't.
3. **`commit` as canonical, `land` as alias.** Inverts (2). Pro:
   maximum discoverability. Con: `tpatch commit` doing *more* than
   `git commit` is exactly the kind of confusion WP-001 §3.4 warned
   against.

**Default position (per T15)**: option 1. Investigation during PRD
work: do real users (after `T-doc-1` lands and skill files mention
`land`) reach for `tpatch commit` first? If yes in two or more
documented sessions, revisit and probably ship option 2. If no, lock
option 1 in an ADR before implementation.

### 7.2 Behavior when `cycle` is mid-flight

If the operator runs `cycle <slug>` with `--interactive` and pauses
mid-lifecycle, then opens another shell and runs `land <slug>`,
should `land` refuse, warn, or proceed?

Default position: refuse with a hint pointing at the in-flight
`cycle` (detectable via `status.json:last_command` and a recency
check). Resolve in implementation; not a paper-design blocker.

### 7.3 Multi-feature `land`

`land <slug1> <slug2>...` is **out of scope** for v1. One feature,
one commit, one `Tpatch-Feature:` trailer. If a future use case
emerges (e.g. landing a parent + hard-children atomic group), it
gets its own PRD and either extends `land` or adds `land-stack`.

### 7.4 Push behavior

`land` does **not** push. `git push` remains the operator's
explicit step. This is locked, not deferred — `land` that pushes
silently is exactly the surprise hidden state WP-001 §2 warned
against.

### 7.5 Composition rule for parent + hard children

WP-001 §6 Q2 was deferred (canonical patch authority composition
rule when a feature has hard children). `land` v1 lands one feature
per commit; the composition rule does not bind. If §6 Q2 is
answered later and produces a "commit the parent and all
hard-children together" semantic, that lands as a separate PRD
(`feat-tpatch-land-stack`).

### 7.6 Pre-commit hook interaction

If a repo has a pre-commit hook that rewrites files (e.g.
`gofmt`-on-commit), the hook's rewrites would mutate the working
tree *after* `land`'s safe-staging check. Behavior: let the hook
run; if it produces a non-empty post-commit diff, `land` exits
with a warning pointing the operator at `git commit --amend` to
re-include the hook's edits.

This is the right default for v1 because pre-commit-hook ergonomics
are a Git feature, not a tpatch feature. Revisit if real-world use
shows it's noisy.

---

## 8. Risks

| Risk | Why it matters | Mitigation |
|---|---|---|
| `land` ships before the guardrail PRDs and silently commits the WP-001 collision shape | Amplifies the very failure mode `land` is meant to help fix. | §0.1 implementation gating (non-negotiable). Supervisor enforces. |
| Operators expect `land` to push | Surprise hidden state; trust erosion. | §7.4 locked; help text explicit; `docs/land.md` says it in the first paragraph. |
| `--allow-extra-paths` becomes the default by habit | Defeats the safe-staging point; reproduces WP-001 row 5. | Help text and `docs/land.md` flag it as an escape hatch, not a normal mode. Skill files do not mention it. |
| `Tpatch-Feature:` trailer collides with another tool's trailer | Ecosystem friction. | `Tpatch-` prefix is namespaced; if a real collision is reported, switch to `X-Tpatch-Feature` in a v2 ADR. |
| Cross-PRD drift between guardrail behavior and `land` assumptions | Boundary-capture failure leaks past the gate. | Guardrail PRDs and this PRD share a "Related" header link both ways; any change to either re-opens the others. |
| Carve-out misuse: operator habitually relies on `land` to leave drift on the two named globals (`.tpatch/upstream.lock`, `.tpatch/FEATURES.md`), normalizing dirty trees post-land. | Visibility erodes; reviewers stop noticing genuine drift. | The stderr note is mandatory and one line per file; `docs/land.md` documents the carve-out as an exception not a feature; assertion in `TestLand_DoesNotStageUnrelatedDirtyMetadata` pins the note wording so future refactors can't quietly drop it. ADR-021 records the bounded scope (two named files, no flag to expand). |

---

## 9. Out of scope

- Replacing `git push`, `git rebase`, `git merge`, `git log`.
- Producing more than one commit per `land` invocation.
- Editing prior commits (use `git commit --amend` or
  `git rebase -i`).
- Writing to hidden refs, parallel commit graphs, or any
  non-standard Git object.
- Generating commit messages via the LLM provider. (Subject
  derivation from `spec.md` / `request.md` per §3.4 is the upper
  bound.)
- **Pattern A metadata-only landing onto `main`** (G55 review F3).
  The `record --files .tpatch/...` approach cannot work because
  `CapturePatchScoped` excludes `.tpatch` before user pathspecs
  (`internal/gitutil/gitutil.go:217-227`). A separate metadata-only
  staging path would need its own PRD.
- Writing the new HEAD's SHA into `status.json` or any other tracked
  file inside the same commit (G55 review F2 — chicken-and-egg
  impossibility). The `Tpatch-Feature:` trailer is the only
  feature↔commit binding.
- An operation log (WP-001 §2 deferred).
- A `land-stack` for parent + hard-children atomic groups
  (§7.5 deferred).

---

## 10. Sources

- `docs/whitepapers/WP-001-feature-slice-gap.md` — §2 (agreed
  re-statement), §5.2 row 7 (missing-Git-projection), §7 Step B
  (the `land` paper-spike contract this PRD implements verbatim
  where possible), §9 (graduation plan and three-PRD split).
- `docs/whitepapers/WP-001-feature-slice-gap.turns.md` — T3.4,
  T4.4, T5, T6 (agreed `land` scope); T13 (gap-table ratification);
  T15 (human-broker decisions); T16 (closure).
- `docs/market-research/personas.md` — Platform Pat audit JTBD
  (`:64-77, 90-99`), Security Sam forwardability JTBD
  (`:117-149`), Maintainer Mira reasoning-preservation JTBD
  (`:179-208`), Common JTBD synthesis (`:239-253`).
- `docs/market-research/competitive-landscape.md` — §6 SMART (v0.7
  ship target), §9 (DEP-3 / git-gud / stk trailer-block precedents),
  §10 (anti-precedents we don't adopt), §11 entry 3 (deterministic
  apply-recipe edge over `git format-patch`).
- `docs/prds/PRD-tpatch-hotfix.md` §3.4 — `Tpatch-CVE` additive
  trailer; cross-PRD coordination point flagged in §3.4 of this PRD.
- `docs/record.md` — `record` semantics this PRD composes.
- `docs/reconcile.md` — Pattern A and Pattern B compatibility
  contract.
- `docs/feature-layout.md` — canonical `post-apply.patch` and
  `patches/NNN-*.patch` audit-trail rules.
- `docs/dependencies.md` — hard-parent gate that `land` honors.
- `internal/cli/cobra.go:797-1015` — current `record` command
  (refusals, scoped capture, recipe autogen, status writeback).
- `internal/cli/phase2.go:22-100` — current `cycle` command
  (boundary delineation).
- `internal/gitutil/gitutil.go:120-180` (`PreflightReconcile` —
  *not* reused; see §3.2), `:217-251` (`CapturePatchScoped`,
  intent-to-add for untracked files), `:560-605`
  (`HasConflictMarkers`, `ScanConflictMarkers` — reused),
  `:666` (`ResolveRef`), `:705` (`IsAncestor`), `:728`
  (`MergeBase`), `:741` (`FilesInPatch`).
