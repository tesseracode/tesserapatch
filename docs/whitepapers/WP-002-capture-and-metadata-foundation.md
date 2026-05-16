# WP-002 — Capture and Metadata Foundation

**Status**: Graduated (N47, 2026-05-16)
**Authors**: N47
**Started**: 2026-05-13
**Supersedes claims in** (does **not** edit them): None

**Related**:
- [WP-001 Feature-slice gap](./WP-001-feature-slice-gap.md)
- [WP-003 Reconcile safety](./WP-003-reconcile-safety-and-middle-pass.md)
- [Active clusters](../CLUSTERS.md)
- [Recording Patches](../record.md)
- [Feature Layout](../feature-layout.md)
- [State of the art research](../state-of-the-art/README.md)

## 1. Context

WP-001's boundary-capture findings exposed three operational gaps in tpatch's
`record` workflow:

1. **Capture-mode visibility**: `record` produces a patch but does not explicitly
   record *which paths* were captured, under what capture semantics (`--all` /
   `--staged` / `--unstaged`), or how untracked files entered the boundary.
   This gap is cited in `PRD-record-capture-modes §1` as the observability debt.

2. **Patch scope metadata**: patches recorded against the same feature across
   multiple `record` invocations share no identity, generation history, or
   provenance chain. The feature `post-apply.patch` is the authority, but
   observers cannot trace *when* it was last recorded, *which base commit*
   anchored it, or *what amendments* were applied after recording.
   This gap is cited in `PRD-feature-patch-identity-metadata §1` as the
   audit-trail debt.

3. **Patch amendment semantics**: once a feature is recorded, `record` always
   captures a fresh patch. There is no first-class way to amend a recorded
   patch (e.g., to fix a pre-recorded typo, reword a comment, or refresh
   metadata after upstream drift) without re-capturing the entire boundary.
   This gap is cited in `PRD-feature-patch-amend §1` as the amendment UX debt.

A fourth gap — **explicit patch scope binding** — emerged: `record` currently
accepts patches with any file set, but it does not offer tooling to constrain
recording to files explicitly *claimed* to be part of the feature. The dependency
model (`PRD-feature-dependencies.md`) ships hard/soft edges, but there is no
symmetric scoping mechanism for patch content. This gap is cited in
`PRD-feature-file-claims §1` as the patch-scope-binding debt.

These four gaps are **not** data-model gaps (WP-001 §5.2 row 11 concluded:
"no data-model gap"). They are observability, audit, UX, and constraint-binding
gaps that all fit within the existing `status.json` store model. The cluster
solves them by introducing three lightweight append-only manifests and one
advisory constraint layer, without displacing the canonical `post-apply.patch`
as replay authority.

## 2. Agreed re-statement of the gap

As of 2026-05-13, the cluster PRDs and supervisor acceptance agree on:

- **File claims are scope metadata, not authorization.** `feature claim add
  path` documents intent; it does not reject recording of other paths.
  Version 1 is advisory-only. Strict mode (rejecting unclaimed files) is
  deferred to v2 when privacy/compliance constraints demand it. This avoids
  a cross-cutting privacy ADR blocking shipping.

- **Capture-mode provenance is deterministic and append-only.** Each `record`
  invocation logs which capture mode (`--all` / `--staged` / `--unstaged` /
  `--claimed-only`) was used and lists the contributing paths. The log lives
  in `record.md` as a `## Capture Provenance` block. Capture-mode strings are
  immutable; audit trail never rewrites.

- **Patch generations are append-only, not lifecycle state.** `patch-generations.json`
  records metadata about each `record` or `patch-amend` operation: base commit,
  timestamp-free determinism marker, amendment reason, and invocation options.
  This is an **audit trail**, not the `status.json` feature state. `post-apply.patch`
  remains the replay authority. The generation manifest is consulted but never
  overwrites it.

- **Patch amendment is metadata-first, not content-rewriting.** `tpatch feature
  patch refresh|fixup <slug> [--reason "..."]` updates metadata and optionally
  re-records a patch; it does not modify existing operations. Fork and fold
  workflows are explicitly deferred to a follow-up PRD (not part of this cluster).

- **File claims do not persist rich context in v1.** Free-text `--reason` for
  claims, claim context retention, or user-provided per-file metadata are all
  gated on a privacy/compliance ADR (ADR-capture-context-privacy-boundary).
  Shipping without this avoids blockers and lets advisory-only claims land
  in Wave α.

- **No schema explosion.** The cluster adds zero new data-model objects.
  `patch-generations.json` is a manifest in `.tpatch/patches/`, not a new
  field in `status.json`. File claims append to an existing `.tpatch/claims.json`
  manifest. Capture provenance is a section in the existing `record.md` side
  research file.

## 3. The four PRDs

### 3.1 PRD-feature-file-claims

**What it does**: Introduces an advisory-only file-claims manifest at
`.tpatch/claims.json`. Users run `tpatch feature claim add path` to document
intent; the manifest stores claim ID (SHA-256 digest), path, claim kind
(`path` only in v1), mode (`advisory`), and source (`manual` or `imported`).

**Why it is part of the cluster**: File claims establish the scope-binding
intent layer that enables downstream `record --claimed-only` (PRD #2) and
future strict-mode enforcement. The advisory mode in v1 lets the claim
infrastructure ship without a privacy ADR. This unblocks Wave α.

**What it does not do**: Claims do not reject unclaimed files at record time.
Claim-based access control, inheritance, or symbol-level/anchor-level claim
kinds are deferred to future PRDs. Free-text reasons are deferred to v2
pending `ADR-capture-context-privacy-boundary`. See `PRD-feature-file-claims §8` for the
full out-of-scope list.

### 3.2 PRD-record-capture-modes

**What it does**: Adds four explicit capture modes to `tpatch record`:
`--all` (default, captures all dirty state), `--staged` (index only,
refusing overlap with unstaged), `--unstaged` (worktree only, refusing overlap
with staged), `--claimed-only` (intersects dirty state with file claims,
refusing if no claims exist).

Each mode writes capture-mode provenance to `record.md`: the mode string,
dirty-state summary (e.g., "2 staged paths, 1 unrelated unstaged"), and path
list for audit. Provenance is append-only; rerecording the same feature under
a different mode produces a new provenance entry.

**Why it is part of the cluster**: Capture modes transform `record` from
observability-opaque to observable-and-composable. They ground the file-claims
advisory layer (mode 4 uses claims). They ship in Wave α in parallel with
file claims. See `PRD-record-capture-modes §3.7` for the mutex matrix
(e.g., `--staged` and `--unstaged` cannot both be set).

**What it does not do**: Capture modes do not change replay semantics;
`post-apply.patch` remains the authority regardless of mode. Modes do not
add new file-selection flags beyond the four listed. Capture-mode persistence
into `status.json` feature state is explicitly out-of-scope; the manifest
is side research, not the feature's applied truth.

### 3.3 PRD-feature-patch-identity-metadata

**What it does**: Introduces `patch-generations.json`, an append-only manifest
recording metadata about each `record` or `patch-amend` invocation: operation
kind (`record` / `refresh` / `fixup`), base commit, patch SHA-256, generation
ID (content-addressed or monotonic per `ADR-patch-generation-manifest-boundary`), timestamp-free determinism
marker, amendment reason (if applicable), and contributing capture-mode
string.

Each row is immutable; no backfill or rewrite. Collision detection (same
patch bytes for the same feature) skips a generation entry, matching the
existing collision-detection semantics (see WP-002 §6, cross-cluster
dependencies, for the `PRD-record-collision-detection` alignment).

**Why it is part of the cluster**: The generation manifest is the bridge from
operation-level provenance (`created_by` on recipe ops) to patch-level audit.
It grounds amendment semantics (PRD #4) and enables downstream reconciliation
features to consult cached `git_patch_id` values when deciding upstream match.
It ships in Wave β, gated on `ADR-patch-generation-manifest-boundary`.

**What it does not do**: The manifest is not the feature's lifecycle state;
`status.json` remains authoritative. Generations do not alter existing replay
paths. They do not enable automatic rebasing, automatic metadata inference,
or agent-session state persistence (all explicitly deferred in
`PRD-feature-patch-identity-metadata §8`).

### 3.4 PRD-feature-patch-amend

**What it does**: Introduces `tpatch feature patch refresh|fixup <slug>
[--reason "..."]` commands for first-class patch amendment. `refresh` re-records
the patch against the current worktree (e.g., after upstream drift). `fixup`
records a metadata-only amendment with an optional reason (e.g., a comment
correction). Both operations produce a generation entry in `patch-generations.json`
with `kind` set to `refresh` or `fixup`.

Staleness guards prevent amendment of a patch whose base commit no longer
applies cleanly or whose dependencies are stale (controlled by
`ADR-patch-amendment-policy` boolean flags.

**Why it is part of the cluster**: Amendment closes the "already-recorded patch
needs a correction" UX gap exposed by boundary-capture case studies. It grounds
the generation manifest's amendment reason field (PRD #3) and completes the
audit-trail story. It ships in Wave γ, gated on `ADR-patch-amendment-policy`.

**What it does not do**: Amendment does not fork/fold (parent amendment with
child rebase); fork/fold are deferred to a follow-up PRD. Amendment does not
alter `post-apply.patch` in-place; instead, it appends a generation entry and
may re-record the patch, leaving the old version traceable via git history
(the `.tpatch/patches/` snapshots and generation metadata). See
`PRD-feature-patch-amend §8` for the full deferred list.

## 4. Cluster dependency graph

The four PRDs form a linear pipeline with the following wave structure:

```
┌─ Wave α (parallel, v0.9.0-alpha-1 and alpha-2 shipped)
│  ├─ PRD-feature-file-claims (shipped v0.9.0-alpha-1, rev-1 fixed)
│  └─ PRD-record-capture-modes (shipped v0.9.0-alpha-2, rev-1 fixed)
│
├─ PRD-feature-patch-identity-metadata ──→ Wave β (depends on `ADR-patch-generation-manifest-boundary`)
│
└─ PRD-feature-patch-amend ──────────────→ Wave γ (depends on `ADR-patch-amendment-policy`)
```

**Runtime dependency chain**: claims → capture-modes → identity-metadata →
patch-amend. Later PRDs reference artifacts (claims manifest, capture
provenance, generation manifest) introduced by earlier PRDs.

**Implementation sequencing**: Wave α (claims + modes) must ship before Wave β
(identity). Wave β must ship before Wave γ (amendment). Fork/fold is a
separate follow-up PRD, not part of this sequencing.

## 5. ADR plan

Three architecture decisions are required before code lands. The cluster uses
named ADR slugs; implementers assign numbers when drafting:

| ADR slug | Locks in | Blocks | Status as of 2026-05-16 |
|---|---|---|---|
| `ADR-capture-context-privacy-boundary` | Whether `--reason` text and richer context can be persisted to tracked metadata; privacy/compliance constraints on metadata retention | v2 claims work (not v1); **not required for Wave α** | Deferred (v1 ships advisory-only) |
| `ADR-patch-generation-manifest-boundary` | `patch-generations.json` schema, append-only semantics, content-addressed vs monotonic identity, `git patch-id --stable` algorithm choice, no-timestamps determinism, no-backfill default | Wave β (PRD #3 implementation) | Pending; required before Wave β implementation |
| `ADR-patch-amendment-policy` | `refresh` vs `fixup` policy defaults, dependent-staleness behavior, verify-freshness invalidation rules, command-namespace surface (`tpatch feature patch refresh|fixup <slug>` is locked by broker; `ADR-patch-amendment-policy` locks in staleness policy) | Wave γ (PRD #4 implementation) | Pending; required before Wave γ implementation |

`ADR-capture-context-privacy-boundary` may remain unwritten as long as v1 ships
advisory-only; if v2 introduces rich context, that ADR must precede the work.
`ADR-patch-generation-manifest-boundary` and `ADR-patch-amendment-policy` are
blocking and must be drafted before their respective implementation waves.

## 6. Cross-cluster relationships

### Upstream: WP-001 (feature-slice gap)

WP-002 inherits WP-001's **"no data-model gap"** finding (WP-001 §5.2, row 11).
The capture-and-metadata cluster preserves that posture: no slice object, no
containment edge, no schema explosion. Instead, the cluster adds lightweight
audit/constraint layers (`claims.json`, capture provenance in `record.md`,
`patch-generations.json`) that extend observability without displacing the
existing `status.json` store model.

WP-001 also established that `post-apply.patch` is the canonical replay
authority. WP-002's generation manifest is explicitly advisory; it informs
but does not overwrite the patch.

### Downstream: WP-003 (reconcile safety)

WP-003 (accepted 2026-05-16; see `WP-003-reconcile-safety-and-middle-pass.md`) defines a reconcile cluster that will
coordinate artifact schema with WP-002. Specifically:

- PRD-reconcile-evidence (WP-003 Wave α) will consult `patch-generations.json`
  (PRD-feature-patch-identity-metadata) to read cached `git_patch_id` values
  as a performance optimization when deciding upstream-match.

- PRD-reconcile-evidence will validate that its own schema (`reconcile-session.json`
  amendment metadata) does not conflict with WP-002's generation manifest.

- WP-003 Wave α gates on WP-002 Wave β acceptance (identity manifest in place).

### Sibling: v0.7 cluster (shipped)

The four PRDs from WP-002 align with the shipped v0.7 cluster
(ADR-016 through ADR-019) on:

- **`PRD-tpatch-land` four-trailer block**: Patch-generation manifest does
  not alter trailer schema. `Tpatch-Feature: <slug>` remains the solo
  feature-identity trailer; generation metadata lives in `.tpatch/`, not
  in git metadata.

- **`PRD-record-collision-detection` same-feature dedup**: WP-002's PRD #3
  (identity) explicitly aligns (§5.1 of that PRD) by skipping a generation
  entry when patch bytes are unchanged, matching collision-PRD's
  skip-numbered-snapshot behavior.

- **`PRD-record-auto-base` `apply.base_commit` ownership**: The identity
  manifest reads `apply.base_commit` from `status.json` but does not
  overwrite it. The feature state remains authoritative.

- **`PRD-patch-already-upstream-detector` patch-id usage**: Reconciliation
  may use stored `git_patch_id` as a cache when `patch_sha256` matches
  live bytes (identity PRD §5.4 alignment). No data-model regressions.

## 7. Implementation status as of writing

### Wave α shipped (v0.9.0-alpha)

- **v0.9.0-alpha-1 (file-claims)**: Shipped 2026-05-13. PRD-feature-file-claims
  v1 fully implemented. Supervisor log: 2026-05-13 entry "Review —
  v0.9.0-alpha-1-file-claims". Rev-1 fix (path-normalization) shipped 2026-05-13
  per LOG entry "Review — v0.9.0-alpha-1-file-claims (rev-1 F1 fix) — 2026-05-13".

- **v0.9.0-alpha-2 (capture-modes)**: Shipped 2026-05-14. PRD-record-capture-modes
  v1 fully implemented with all four modes, provenance logging, and mutex
  matrix validation. Supervisor log: 2026-05-14 entry "Review —
  v0.9.0-alpha-2-capture-modes". Rev-1 fix (claim_ids provenance narrowing)
  shipped 2026-05-14 per LOG entry "Review — v0.9.0-alpha-2-capture-modes
  (rev-1 F1 fix) — 2026-05-14".

Both alphas are in production (internal testing). No blockers on Wave α.

### Wave β and γ blocked on ADRs

- **Wave β** (`PRD-feature-patch-identity-metadata`): Gated on
  `ADR-patch-generation-manifest-boundary`
  (pending; draft required before routing to implementation).

- **Wave γ** (`PRD-feature-patch-amend`): Gated on
  `ADR-patch-amendment-policy` (pending; draft required before routing to
  implementation). Note: PRD-feature-patch-amend
  §4 requires update by T55 to reflect broker's CLI surface decision
  (`tpatch feature patch refresh|fixup <slug>` vs `tpatch record <slug>
  --amend-kind`); this update is a prerequisite to Wave γ routing but is
  not blocking WP-002 whitepaper acceptance.

## 8. Out of scope (pinned)

The following are deliberately deferred and not part of WP-002:

- **Strict claim enforcement**: v1 is advisory-only. Strict mode (rejecting
  unclaimed files at record time) is deferred to v2 pending claim-enforcement
  privacy impact review. See PRD-feature-file-claims §8.

- **Free-text claim metadata**: `--reason` for claims, claim context
  persistence, or user-provided per-file structured metadata are all gated
  on `ADR-capture-context-privacy-boundary`. v1 omits these fields.

- **Fork and fold patch-amendment workflows**: Forking (parent amendment with
  child rebase) and folding (child amendment with parent retroactive merge)
  are deferred to a follow-up PRD. See PRD-feature-patch-amend §8.

- **Automatic file-ownership inference**: The system does not auto-detect or
  infer which files "belong" to a feature based on history, dependency-DAG
  structure, or symbol dependencies. Claims are manual. Inference is deferred
  to future PRDs if desired.

- **IDE hooks and agent-session metadata**: IDE integration (Cursor, VS Code
  tpatch extension triggers) and session-persistent agent metadata are
  orthogonal to the capture/amendment pipeline and deferred to separate
  integration PRDs.

- **Symbol/anchor claim kinds**: v1 only supports `path` claims (files and
  directories). Symbol-level (function, class, struct field) and anchor-level
  (line-number, AST-node) claim kinds are deferred to v2 pending symbol-table
  infrastructure discussions.

## 9. Sources

Every cite in this whitepaper is anchored:

| Reference | Source |
|---|---|
| WP-001 boundary-capture findings | `docs/whitepapers/WP-001-feature-slice-gap.md:64-69` (case-study finding: no true data-model gap) |
| WP-001 §5.2 "no data-model gap" | `docs/whitepapers/WP-001-feature-slice-gap.md` (end of §5.1, carried through to §5.2 conclusion) |
| PRD-feature-file-claims §1 scope-binding debt | `docs/prds/PRD-feature-file-claims.md` §1 Problem statement |
| PRD-record-capture-modes §1 observability debt | `docs/prds/PRD-record-capture-modes.md` §1 Problem statement |
| PRD-feature-patch-identity-metadata §1 audit-trail debt | `docs/prds/PRD-feature-patch-identity-metadata.md` §1 Problem statement |
| PRD-feature-patch-amend §1 amendment UX debt | `docs/prds/PRD-feature-patch-amend.md` §1 Problem statement |
| PRD-record-collision-detection alignment | `docs/prds/PRD-record-collision-detection.md` §3.2; identity PRD §5.1 cross-cite |
| Supervisor acceptance (2026-05-13) | `docs/supervisor/LOG.md` entry "Review — Capture-and-Metadata Foundation Cluster (4 PRDs) — 2026-05-13" |
| `ADR-capture-context-privacy-boundary` deferred to v2 | `docs/CLUSTERS.md` Pending ADRs table and supervisor LOG privacy-boundary finding |
| Wave α shipped dates | Supervisor LOG entries 2026-05-13 and 2026-05-14 for alpha-1 and alpha-2 verdicts respectively |
| `status.json` as authoritative feature state | `docs/feature-layout.md:5`, `docs/feature-layout.md:34-44` (canonical `post-apply.patch` reference) |
| `post-apply.patch` replay authority | Same as above; also `docs/agent-as-provider.md:137-142` (trust patch over recipe on conflict) |
| Existing `created_by` op-level provenance | `docs/dependencies.md:114-138` (documented as live gate from v0.6.0 onward) |
| `PRD-feature-dependencies.md` hard/soft edges | `docs/prds/PRD-feature-dependencies.md` and `docs/dependencies.md:35-42` |
| Four-trailer block (`PRD-tpatch-land`) | `docs/prds/PRD-tpatch-land.md` §3.4 (or equivalent section if path differs) |
| Broker CLI namespace decision (2026-05-13) | Supervisor LOG entry above, Findings section, bullets 1-2 (claims namespace, patch-amendment CLI surface) |
| Research prior art (state-of-the-art) | `docs/state-of-the-art/README.md` and `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md` cited in supervisor LOG checklist |

---

**Whitepaper authored by**: N47 (2026-05-16)
**Status**: Ready for broker review before citation from WP-003
