# ADR-015 — Prior-Art Mapping for Identity Duality, Operation Log, and Stack Primitives

**Status**: Accepted (research framework — locks the conceptual model that future PRDs will refine; does not by itself authorize implementation)
**Date**: 2026-05-02
**Deciders**: Core (exploratory market-research session)
**Supersedes**: n/a
**Related**: WP-001 §3.3 (op-log claim, restated), WP-001 §5.2 (collision shape on case studies),
PRD-tpatch-land §3.4 (four-trailer block), PRD-record-collision-detection,
PRD-record-auto-base, PRD-tpatch-git-primitive-mapping, PRD-intent-version-control-evaluation,
ADR-010 (provider-assisted resolver), ADR-011 (feature DAG)

## Context

Through v0.6.1, tpatch has reached the edge of what an unstructured
"slug + post-apply.patch + status.json" model can carry. Two real
problems make this concrete:

1. **WP-001 §5.2 was re-verified on the live case-study trees on
   2026-05-01.** The collision shape is not a theoretical risk. It is
   shipping today:

   - `tesseracode/copilot-api` — 9 of 21 features with patches fall
     into 3 collision groups (sizes 5, 2, 2). The 5-feature group
     shares base commit `ac4fefd337d4`, whose subject is literally
     `"feat: implement 5 cosmetic features from tpatch stress test"`.
     A single upstream commit landed five features at once and
     `record --from <pre-commit>` captured the same diff into all
     five feature directories.
   - `tesseracode/t3code` — 11 of 17 features with patches share one
     byte-identical 137285-byte 27-file patch with base commit
     `02bcb16d06e8`. The patch's own embedded `.claude/instructions.md`
     tells the agent to use `record --from main`, which is the failure
     pattern encoded as guidance.

2. **WP-001 §3.3 already restated the op-log gap precisely.** Per-
   feature audit trails ship today (`patches/NNN-<label>.patch`,
   `apply-session.json`, `resolution-session.json`,
   `status.json:notes`). What does not ship is a repo-wide,
   cross-feature, time-ordered log of user-visible operations with
   recovery semantics — the `jj op log` analogue.

The exploratory session that produced this ADR re-read five prior-art
systems that have solved variants of these problems over the last 30
years (Quilt, StGit, Mercurial Queues, Mercurial evolve, jj). The
goal of this ADR is not to copy any of them, but to **lock the
conceptual model** tpatch will use to talk about identity, history,
and stack operations in future PRDs and ADRs. Without that shared
vocabulary, every follow-up PRD has to re-litigate the same
abstractions from scratch.

Operational decisions (exact JSON shapes, CLI flag surfaces, on-disk
layout for new artifacts) remain in the follow-up PRDs this ADR
enables. This ADR governs only the cross-cutting invariants that
would be painful to reverse once code ships.

## Working framework — concept map

The five prior-art systems answer six questions in different ways.
The columns are systems; the rows are concepts; cells are the
primitive each system uses.

| Concept | tpatch today | Quilt (2003) | StGit (2005, modern) | hg MQ (2005, deprecated 2019) | hg evolve (2014) | jj (2020) |
|---|---|---|---|---|---|---|
| Stable identity (the "thing" that survives rewrites) | `slug` | filename in `series` | patch name | filename | **change-id** | **change-id** |
| Moving identity (the "version" of that thing) | `post-apply.patch` bytes (overwritten in place) | none — same filename, different bytes | git commit hash (changes on `refresh`) | filename, different bytes | obsolescence successor hash | **commit-id** |
| Cross-feature operation log | per-feature artifacts only | none beyond `.pc/` backups | git reflog | none | obsolescence DAG (exchanged via push/pull) | `jj op log` (append-only, repo-wide) |
| Recovery / undo of an operation | none beyond Git reflog | manual `pop` + restore | git reflog | manual | `hg evolve` resolves troubles | `jj op restore` |
| Refresh (fold edits back into a tracked patch) | `apply --mode done` + `record` | `quilt refresh` | `stg refresh` | `hg qrefresh` | `hg amend` | `jj squash` into `@-` |
| Peel / uncommit (extract a tracked patch back into the working tree) | none | `quilt pop` (apply-only inverse) | `stg uncommit` (graduate to/from regular commit) | `hg qpop` | `hg uncommit` | `jj abandon` + child preservation |

A few observations the table hides:

- **Quilt and MQ collapse stable + moving identity into one filename.**
  That is exactly tpatch's current model (slug = stable, the patch
  bytes change in place). It works at small scale; both systems
  struggle with rewrites across upstream rebases, which is why MQ was
  formally deprecated in favour of evolve.
- **StGit gives you Git's reflog "for free"** because patches are
  commits. tpatch currently emits patches as plain files outside Git
  history, so reflog is not available even in principle.
- **Evolve's "obsolescence markers"** are the deepest model in the
  set. They survive rebases and are exchanged via push/pull. That is
  the closest prior art to what reconcile-driven patch rewriting
  should leave behind, and tpatch has nothing analogous today.
- **jj is the cleanest synthesis.** Two-layer identity (change-id
  stable, commit-id moves) plus an explicit operation log plus an
  explicit restore primitive. Working-copy-as-commit and "no index"
  are jj-isms tpatch should not adopt; the identity duality and op
  log are.

The framework sentence this ADR locks:

> **Stable identity is what the user names. Moving identity is what
> bytes-on-disk hash to. tpatch needs both, and they should never be
> conflated.**

## Decision

### D1. Adopt the change-id / commit-id duality, mapped to slug / patch-SHA

The stable identity in tpatch is the **feature slug**. The moving
identity is the **`Tpatch-Patch-SHA`** trailer (sha256 of the
post-apply.patch bytes), as already specified in PRD-tpatch-land
§3.4. This ADR locks that they are the **same kind of duality** as
jj's change-id / commit-id, and that future docs and code will name
them consistently:

- `slug` = stable / "what the user names" = jj change-id role.
- `Tpatch-Patch-SHA` = moving / "what the bytes hash to" = jj
  commit-id role.

Implication: any operation that mutates the canonical patch (record,
reconcile, amend) **must** advance the `Tpatch-Patch-SHA` and **must
not** advance the slug. Any operation that renames or splits a
feature **must** mint a new slug. These two rules are the
contract; the field names already exist in PRD-tpatch-land §3.4.

This ADR does not yet require persisting prior `Tpatch-Patch-SHA`
values across reconciliations. That is D2.

### D2. Patch artefacts grow a generation history (`patches/<slug>/v<n>.patch`)

Today `.tpatch/features/<slug>/artifacts/post-apply.patch` is
overwritten in place when `reconcile` rewrites a feature's patch.
That throws away the old hash and makes the change-id / commit-id
duality from D1 unobservable on disk.

Lock-in: future implementation will preserve **every** generation of
a feature's canonical patch as `patches/<slug>/v<n>.patch` (or an
equivalent monotonic naming scheme), where `n` increments on every
record / reconcile-driven rewrite. The current
`artifacts/post-apply.patch` either becomes a symlink to the latest
`v<n>.patch` or remains as a convenience artifact pointing at the
same bytes. Older generations are read-only.

Why now: WP-001 §3.3's "promote per-feature artifacts into a
queryable cross-feature view" requires that the per-feature artifacts
themselves be stable references. Overwriting them in place breaks
the very view the WP wants to enable.

Operational details (exact filename pattern, retention policy,
whether to also write a `patches/<slug>/HISTORY.md` index, whether to
GC old generations) are deferred to a follow-up PRD. This ADR locks
only that **rewrites must not be in-place**.

Existing `existing-patches/N-<label>.patch` snapshots
(`docs/feature-layout.md:51-67`) serve a different purpose
(per-apply-mode capture, not generation tracking) and remain
unchanged.

### D3. An append-only repo-wide operation log is the right primitive for the WP-001 §3.3 gap

Lock-in: `.tpatch/oplog/` will be the canonical home for the
cross-feature, time-ordered, machine-readable record of every
state-changing tpatch command. Each entry is one append-only JSON
file (or one line in an append-only JSONL file — schema deferred to
the follow-up PRD), keyed by timestamp + command + slug.

Why this shape:

- **Append-only** is the only safe model. Any operation that needs
  "the previous state" reads earlier entries; nothing edits or
  deletes them. This matches `jj op log` exactly.
- **Per-file (or per-line) entries** keep merge conflicts on a
  shared op log to a minimum: two collaborators running tpatch on
  the same repo append to disjoint files / lines.
- **Cross-feature** is non-negotiable. The current per-feature
  `status.json:notes` and `apply-session.json` are insufficient
  precisely because they do not relate operations across features
  (the use case "what did I do today across all features" requires a
  flat, time-ordered view).

This ADR does **not** yet authorize a `tpatch op log` /
`tpatch op restore` command surface. Restore semantics in particular
are non-trivial (jj's `op restore` works because every state change
is hash-addressed; tpatch's state is files-on-disk). Authorization
for the command surface is deferred to the follow-up PRD.

This ADR also does **not** require evolve-style obsolescence markers
exchanged via push/pull. Single-repo op log is the floor; the
distributed obsolescence question is out of scope here.

### D4. Path B's "refresh"-equivalent primitive is canonical and gets a name

The current `apply --mode started` → manual edits → `apply --mode
done` → `record` sequence is morally identical to StGit's `stg
refresh`, Quilt's `quilt refresh`, hg MQ's `qrefresh`, and jj's
"squash into `@-`". `docs/commits.md` formalized this as the
**Path B workflow** for the interim 2-commit convention.

Lock-in:

- The Path B workflow as documented in `docs/commits.md` is the
  canonical refresh primitive in tpatch. Future docs and code refer
  to "the refresh primitive" to mean this sequence.
- A future `tpatch refresh <slug>` alias / sugar command is on the
  table as a quality-of-life PRD but **not** required by this ADR.
  The sequence is the contract; the name is convenience.

This ADR forecloses the alternative model where tpatch
operates on a Git-commit-as-patch substrate (StGit-style). That
model requires every patch to be a Git commit, which conflicts with
the "patch lives in `.tpatch/`, separate from main history" design
that ADR-011 D1 and the entire `feature-layout.md` model rest on.
The refresh primitive is files-on-disk in tpatch, not commits.

### D5. A peel / uncommit primitive is on the roadmap, gated on D2 and D3

Today there is no way to say "take this feature off the stack and
reify its diff back into the working tree" — the inverse of `apply
--mode execute`. StGit's `stg uncommit`, Quilt's `quilt pop`, hg's
`hg qpop` and `hg uncommit`, and jj's `jj abandon` + child
preservation all serve this role.

Lock-in: a future `tpatch feature unstack <slug>` (working name) will
be added to the command surface. It will:

- reverse-apply the current `post-apply.patch` against the working
  tree,
- mark the feature back to `requested` (or a new pre-apply state to
  be defined in the PRD),
- leave the diff in the working tree as ordinary uncommitted changes
  the operator can edit, split, or discard.

This is **gated on D2** (so the previous patch generation is still
recoverable) and **D3** (so the unstack itself is an op-log entry
that can be inspected and — eventually — undone). Without those
two, an unstack is a destructive operation that's hard to reverse.

This ADR does not commit a slice plan or a target milestone for
unstack. It locks only that the primitive is the right one to
target.

### D6. tpatch does not adopt working-copy-as-commit (no jj-style index removal)

jj's "the working copy is just another commit, automatically
amended" model is incompatible with tpatch's substrate (Git, with an
index, with `git stage` / `git commit` retained as user verbs in the
Path B workflow). This ADR explicitly **rejects** importing this
jj-ism.

Why this is a decision worth locking: D1–D5 import substantial jj
vocabulary (change-id duality, op log, restore semantics implicitly).
A reasonable next agent might propose "import working-copy-as-commit
too." This ADR forecloses that. tpatch's relationship to the working
tree remains: working tree is the operator's canvas, `tpatch apply`
and `tpatch record` are explicit verbs that move bytes between the
canvas and `.tpatch/`. No magic auto-amend.

### D7. tpatch does not (yet) adopt evolve-style distributed obsolescence

Mercurial-evolve's killer feature is that obsolescence markers are
exchanged via push/pull, so two collaborators rewriting the same
commit can converge. This is the deepest model in the prior-art set
and the most distant from tpatch's current architecture.

Lock-in: this ADR explicitly defers any decision on distributed
obsolescence. The op log in D3 is **single-repo** and **append-only**.
Whether two collaborators' op logs ever need to merge — and what
happens when they do — is out of scope for this ADR and any
follow-up PRD that lands D2 or D3 in isolation.

If distributed obsolescence is ever needed, it will be its own ADR
that explicitly supersedes this clause.

## Consequences

**Positive.**

- Future PRDs (PRD-tpatch-land, PRD-record-collision-detection,
  PRD-record-auto-base, PRD-intent-version-control-evaluation) now
  share one vocabulary for identity duality and op-log shape.
  "Stable identity" and "moving identity" stop being implicit.
- WP-001 §3.3's op-log claim becomes actionable: D3 names the file
  layout class without committing the schema. The follow-up PRD
  starts from a blank schema, not a blank concept.
- The change-id / commit-id duality from D1 is the conceptual lever
  that makes PRD-tpatch-land's four-trailer block defensible. The
  trailers are not arbitrary; each one names exactly one of the two
  identities or fixes one to the other.
- D6 and D7 prevent a future drift toward "tpatch is just jj for
  forks," which would explode scope.

**Negative.**

- D2 (versioned patches) means more bytes-on-disk per feature over
  time. Mitigation: retention policy / GC is a future PRD knob.
- D3 (op log) introduces a new persistent artifact class. Mitigation:
  format is locked here, schema is deferred; and the artifact is
  optional (tpatch works without it).
- Five prior-art systems is a lot to ask future agents to internalize.
  Mitigation: D1's framework sentence and the concept-map table
  collapse the lesson to two lines.

**Neutral.**

- This ADR does not block, accelerate, or alter any in-flight
  milestone. Slice D (M15 Wave 3) is unaffected.

## Out of scope

The following are explicitly **not** decided here:

- Exact on-disk filename pattern for versioned patches
  (`patches/<slug>/v<n>.patch` is illustrative, not binding).
- Exact JSON / JSONL schema for op-log entries.
- Whether `tpatch op log` / `tpatch op restore` ship as commands.
- Retention / GC policy for older patch generations and op-log
  entries.
- Distributed obsolescence (D7 explicitly defers).
- StGit-style "patches are Git commits" architecture (D4 explicitly
  rejects).
- jj-style working-copy-as-commit (D6 explicitly rejects).
- Updating WP-001 §5.2 with the re-verified collision shape numbers.
  Out of scope per current handoff. The 2026-05-01 audit results
  live in `docs/handoff/CURRENT.md` until WP-001 is opened for an
  amendment.
- A `tpatch refresh` command surface (D4 explicitly defers).
- A `tpatch feature unstack` command surface (D5 explicitly defers,
  gated on D2 and D3).

## Follow-ups this ADR enables

These are not commitments — they are the PRDs / ADRs that this ADR's
framework lets future agents draft cleanly:

1. PRD-record-collision-detection (already drafted; WP-001 §5.2
   audit confirms the urgency).
2. PRD-patch-generation-history — implementation of D2.
3. PRD-tpatch-oplog — implementation of D3, including schema and
   `tpatch op log` / `op restore` command-surface decisions.
4. ADR-016 (or PRD) for an explicit `tpatch refresh` alias if D4's
   "Path B sequence is the contract, name is convenience" line
   stops being enough.
5. ADR for `tpatch feature unstack` once D2 + D3 ship.

## Invariants future agents must preserve

Per the cadence rules in `AGENTS.md` and the overall project
philosophy:

- D1 (slug = stable, patch-SHA = moving) is load-bearing for any
  future trailer-emitting code. Never collapse them back into one
  identifier.
- D2 (no in-place patch overwrites) is load-bearing for D3's
  reproducibility and for any future "what changed in this feature
  across reconciliations?" view. Never reintroduce in-place
  rewrites once D2 ships.
- D3 (append-only) is load-bearing for any future undo / restore
  primitive. Never edit historical op-log entries.
- D6 and D7's rejections are deliberate. If a future agent wants to
  reverse them, that requires its own ADR with explicit supersession
  of this clause.

## Amendments

### 2026-05-02 — gbp-pq added to the prior-art set

`gbp pq` (git-buildpackage's patch-queue tool — used by Debian
since ~2007) was researched after this ADR was accepted. It maps
cleanly into the existing framework **without changing D1–D7**:

- gbp-pq's **`Gbp-Pq: Name` per-patch tag** survives renumbering and
  preserves identity across `import` ↔ `export` round-trips. Per-
  patch precedent for **D1** — same role as our `slug` (stable
  identity that survives bytes-level rewrites).
- gbp-pq's **`import` ↔ `export` round-trip** (files in
  `debian/patches/` ↔ commits on a `patch-queue/<branch>` branch ↔
  files again) is a precedent for **D4** (Path B as the canonical
  refresh primitive). Validates that "patches as files / patches as
  edits" round-trip is a recognized first-class workflow.
- gbp-pq's **`--time-machine N`** flag — when re-applying a patch
  queue to a rebased upstream fails, walk back N commits on the
  current branch and try each — is a **new** concrete direction this
  ADR did not anticipate. It is **not** part of D7 (which is about
  *distributed* obsolescence exchange via push/pull). It is a
  *single-repo* upstream-search heuristic. Future PRD candidate:
  `tpatch reconcile --time-machine N`.
- gbp-pq's **DEP-3 header standard** is precedent for the four-
  trailer block in PRD-tpatch-land §3.4. Validates trailer-based
  identity as a forwardable convention; DEP-3 has been forwarded
  upstream by Debian for ~15 years.
- gbp-pq's **"drop-and-recreate the patch-queue branch on every
  round-trip"** worldview is the explicit opposite of **D2**
  (versioned patches must persist). gbp-pq makes this trade-off
  because the integration branch (Debian source) is the truth;
  tpatch makes the opposite trade-off because the patches are the
  truth (we don't own the user's host repo). The opposing decisions
  are both internally consistent — D2 stands.

Net: ADR-015 holds. No D1–D7 are changed. The full positioning
analysis (PESTEL / SWOT / SMART / Strategy Canvas / Business Model
Canvas across all 8 prior-art systems including gbp-pq) lives in
[`docs/market-research/competitive-landscape.md`](../market-research/competitive-landscape.md).
The `--time-machine` direction is logged there as a v0.8 PRD
candidate.

### 2026-05-03 — Patch-theory DVCSes (darcs, Pijul) added to the prior-art set

User flagged Pijul during continued market-research review; darcs
is its conceptual ancestor. Both are *patch-theory* DVCSes:
patches are first-class substrate objects, conflict resolutions
are permanent, and independent patches commute (same result, same
identifier, regardless of order).

These systems sit one level deeper than jj / hg-evolve in our
concept map:

- jj / evolve treat patches as first-class *within* a snapshot
  graph (Git or hg respectively).
- darcs / Pijul make the patch graph the substrate. There is no
  underlying snapshot history; the repo state is a set of
  patches that compose by mathematical rules.

Mapping back to D1–D7:

| Decision | Pijul / darcs evidence | Conclusion |
|---|---|---|
| **D1** (slug stable, patch-SHA moves) | Pijul has patch-id (stable, content-derived) + position-in-channel (moves). Reaffirms the duality. | Holds. |
| **D2** (versioned `patches/<slug>/v<n>.patch`) | Patch theory doesn't need versioning — commutation makes a patch always re-identifiable from its content. We are on Git, no commutation; D2 is a Git-substrate workaround for what patch theory gets natively. | Holds with caveat. |
| **D3** (append-only oplog) | Pijul's patch graph IS the op log. | Direction validated. |
| **D4** (Path B is canonical refresh) | `pijul record` is structurally similar — capture working-copy delta as a new patch object. | Holds. |
| **D5** (peel/uncommit on roadmap) | Pijul's commutation makes "remove a patch" a single math operation. We can't replicate without patch theory; on Git, D5 is gated on D2 + D3 as currently authored. | Holds. |
| **D6** (reject working-copy-as-commit) | Pijul has its own working-copy model, distinct from both Git's index and jj's no-index. Doesn't conflict with D6. | Holds. |
| **D7** (defer distributed obsolescence) | Pijul / darcs effectively *solve* distributed merge via commutation. We can't import the solution onto Git. | Holds — defer is correct for our substrate. |

**No new D-decisions are added.** Pijul / darcs are recorded as a
patch-theory reference frame. When future agents wonder "could we
adopt patch theory?", the answer is: SPEC commits to a Git /
optionally-jj substrate (see SPEC `architecture`). Patch-theory
primitives validate the *direction* of D1 / D2 / D3 / D5 but
cannot be ported onto a snapshot substrate.

Path link to the populated map:
[`docs/market-research/competitive-landscape.md`](../market-research/competitive-landscape.md)
§1 Lane D + §2 patch-theory note.
