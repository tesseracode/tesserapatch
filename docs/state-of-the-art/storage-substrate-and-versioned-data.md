# Storage Substrate and Versioned Data

**Status**: Snapshot research (paper-only; no implementation authorized)  
**Date**: 2026-07-16  
**Owner / byline**: S66  
**Related**: [Feature layout](../feature-layout.md),
[Recording patches](../record.md),
[Reconcile workflow](../reconcile.md),
[Feature dependencies](../dependencies.md),
[WP-001 Feature-slice gap](../whitepapers/WP-001-feature-slice-gap.md),
[WP-002 Capture and metadata foundation](../whitepapers/WP-002-capture-and-metadata-foundation.md),
[WP-003 Reconcile safety and middle-pass foundation](../whitepapers/WP-003-reconcile-safety-and-middle-pass.md),
[WP-006 Substrate and non-Git mode](../whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md),
[WP-007 Decision tickets](../whitepapers/WP-007-decision-tickets-and-ticket-tracking.md),
[ADR-015 Prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md),
[ADR-024 Patch generation manifest](../adrs/ADR-024-patch-generation-manifest-boundary.md),
[ADR-025 Reconcile evidence and revision schema](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md),
[Competitive landscape](../market-research/competitive-landscape.md)

## Executive summary

The default stance survives the research:

> **tpatch should keep authoritative project state in reviewable `.tpatch/`
> Markdown, JSON, JSONL, and patch files. Databases, hidden Git refs, Git notes,
> remote agent memory, and search indexes should be caches or optional
> projections, not the source of truth.**

The current model is not an accidental pile of files. It already has a sound
layering:

1. `status.json` holds current lifecycle truth.
2. `artifacts/post-apply.patch` holds current replay truth.
3. Markdown preserves intent and human reasoning.
4. `patch-generations.json` preserves deterministic moving patch identity.
5. JSONL evidence and revision artifacts preserve append-only audit history.
6. Git commits, trailers, commit SHAs, and patch IDs project or correlate that
   state into Git; they do not replace the feature store.
7. `FEATURES.md` is already a derived index rebuilt from per-feature state.

The strongest alternatives improve one dimension by sacrificing several others:

- **Dolt** provides excellent versioned-table semantics, structural sharing, SQL
  history, and branch/merge behavior, but its opaque database store and large
  operational/dependency footprint are a poor fit for Git PR review, a small Go
  CLI, and tpatch's file-oriented contract.
- **SQLite** is the only database worth keeping as a practical option. If
  adopted, it should be a gitignored, rebuildable cache for cross-feature
  queries. It should never own lifecycle state, patch bytes, generations, or
  evidence.
- **Generated JSON indexes** should be tried before SQLite. They preserve normal
  Git review, deterministic serialization, offline use, and the current
  dependency posture.
- **Git notes and custom refs** are useful metadata mechanisms but are easy to
  omit from fetch/push and invisible in ordinary pull-request review. They are
  weaker than tracked `.tpatch/` files for project truth.
- **CRDT/local-first databases** solve concurrent replicated editing, not
  patch-authority review. Their binary formats and language/runtime requirements
  are unjustified unless tpatch later needs real-time concurrent editing of the
  same logical record.
- **Agent memory systems** optimize semantic recall from conversations. They are
  not appropriate for deterministic lifecycle or patch truth. MCP is an access
  protocol, not a storage model.

No database should be the source of truth in any currently described tpatch
scenario. A future shared server may aggregate many repositories, but each
repository should remain independently reconstructable from its own files.

This paper does **not** recommend creating WP-008 now. There is no live product
dispute: the evidence supports the existing architecture. WP-008 becomes
necessary only if a future proposal would make a database, hidden ref, remote
service, or shared workspace store authoritative.

## Refresh triggers

Refresh this paper when any of the following becomes true:

- repositories with more than 500 features become common;
- measured `status`, DAG, evidence, or patch-history queries become too slow;
- tpatch introduces a repo-wide operation log with concurrent writers;
- tpatch proposes a shared multi-repository service or durable remote registry;
- a second change-tracking substrate is proposed after Git;
- non-Git mode grows beyond metadata-only planning;
- a database, Git notes, or custom refs are proposed as authoritative storage;
- decision tickets graduate into a separate product with a concrete persistence
  contract;
- raw agent session or embedding persistence is reopened.

## 1. Current tpatch storage model

### 1.1 The current store is layered, not flat

| Layer | Current artifacts | Authority |
|---|---|---|
| Human intent and reasoning | `request.md`, `analysis.md`, `spec.md`, `exploration.md`, `record.md` | Authoritative narrative for their phase; not lifecycle state |
| Current lifecycle | `status.json` | Source of truth for feature state, apply/reconcile summaries, dependencies, and verify overlay |
| Current replay | `artifacts/post-apply.patch` | Canonical feature diff |
| Deterministic execution | `artifacts/apply-recipe.json` | Operation plan; subordinate to canonical patch if they disagree |
| Patch history | `patches/NNN-*.patch` | Historical full-diff snapshots, not replay input |
| Moving patch identity | `artifacts/patch-generations.json` | Append-only generation history and dependency snapshots |
| Reconcile evidence | `artifacts/reconcile-evidence.jsonl` | Append-only evidence audit; not current verdict truth |
| Review corrections | `artifacts/reconcile-revisions.jsonl` | Append-only review/audit history |
| Upstream anchor | `.tpatch/upstream.lock` | Current recorded upstream reference |
| Git projection | land trailers, commit SHAs, `git patch-id --stable` | Commit-scoped link and evidence, not sole feature identity |
| Generated view | `.tpatch/FEATURES.md` | Rebuildable human index; per-feature `status.json` remains authoritative |

The canonical boundary is explicit in `docs/feature-layout.md`: numbered patches
are history, while `artifacts/post-apply.patch` is the file reconcile and replay
should trust. `status.json` is similarly explicit in code and docs as current
state truth. The DAG is derived from `status.json:depends_on`, not stored in a
second graph database.

This distinction already avoids the most common database-design error: mixing
current state, immutable history, human explanation, and derived indexes in one
ever-growing object.

### 1.2 Current state and audit history have different lifecycles

The shipped schemas deliberately separate bounded current state from unbounded
history:

- `status.json` stays small and hot.
- `patch-generations.json` is a deterministic manifest with monotonic generation
  numbers and content-addressed `pg_<12hex>` IDs.
- reconcile evidence and revisions use newline-terminated JSONL with
  content-addressed IDs, strict schemas, sorted arrays, and writer-refuses /
  reader-warns corruption behavior.
- malformed audit metadata does not make current lifecycle state unreadable.

ADR-024 and ADR-025 therefore already implement the most useful part of a
versioned database model: immutable version identifiers, schema versions,
deduplication, audit history, and explicit current-vs-history boundaries.

### 1.3 Determinism is a storage feature

tpatch's deterministic serialization rules are load-bearing:

- no wall-clock fields in patch generations;
- stable field sets and closed enums;
- sorted arrays where order is not semantic;
- content-derived IDs;
- strict unknown-field handling;
- atomic temp-file, sync, and rename writes for generation and evidence
  artifacts;
- relative paths rather than machine-specific absolute paths.

A database can provide transactions, but it does not automatically provide
reviewable or reproducible logical serialization. tpatch's current schemas make
that contract visible in Git.

### 1.4 A generated projection already exists

`Store.SaveFeatureStatus` writes `status.json`, then attempts to rebuild
`FEATURES.md`. Its own comment states that index refresh errors do not invalidate
the authoritative status write. That is exactly the source/projection boundary
this paper recommends for future indexes:

```text
per-feature files -> deterministic derived view
```

The next scaling step should extend this pattern rather than introduce a new
source of truth.

### 1.5 The known queryability limit

Current aggregate reads generally scan `.tpatch/features/`, load each
`status.json`, and sort by slug. The DAG algorithms operate over the resulting
in-memory graph. This is simple and correct, with an expected cost proportional
to feature count plus edge count.

That cost is appropriate for dozens or hundreds of features. It may become
noticeable for thousands of features, large evidence histories, cross-feature
"touches path X" searches, or workspace-wide aggregation across many repos.
Those are projection/index problems, not evidence that lifecycle truth belongs
in a database.

### 1.6 Earlier tpatch research points the same way

WP-001 found no feature-slice data-model gap: the observed failures came from
capture boundaries and missing Git projection, not from a lack of a database or
new storage object. WP-002 then added lightweight manifests without displacing
canonical patch authority. WP-003 added evidence and revision logs without
turning audit records into lifecycle truth.

WP-006 keeps non-Git mode metadata-only and rejects a hidden tpatch-native VCS.
WP-007 keeps decision tickets outside `.tpatch/features/` until they become
patch-bearing implementation work. Together these decisions establish a
consistent rule:

> Add a new durable object only when its lifecycle and authority cannot be
> represented by an existing file or a derived view.

## 2. Git storage model

### 2.1 Content-addressed object database

Git stores four core object types:

| Object | Meaning for tpatch |
|---|---|
| Blob | File contents, including tracked `.tpatch/` files |
| Tree | Path/name mapping for a repository snapshot |
| Commit | Tree plus parents, author/committer metadata, and message |
| Tag | Named annotated pointer to another object |

The object database gives tpatch:

- content-addressed transport for tracked artifacts;
- structural sharing across repository history;
- integrity checking;
- a standard clone/fetch/push distribution path;
- ordinary PR diffs for text artifacts.

It does **not** give a first-class feature object. Git knows snapshots, paths,
commits, and refs; it does not know that several changing patch byte sequences
are generations of one stable tpatch feature.

### 2.2 Packfiles

Git repacks loose objects into packfiles and uses delta compression. This is
already the storage compaction layer for tracked `.tpatch/` history. tpatch does
not need its own content-addressed blob pack merely to avoid storing repeated
JSON or patch bytes in every Git commit.

Large raw vectors, model caches, and agent transcripts are different: they may
be too sensitive or voluminous for normal Git history. The correct pattern there
is a small tracked manifest plus an optional external or gitignored payload, not
a replacement of the core feature store.

### 2.3 Refs and reflogs

A ref is a mutable name pointing to an object. A reflog records local ref
movement and provides valuable recovery after rewrite.

For tpatch:

- refs are useful for branches, tags, and optional projections;
- reflogs are useful local recovery evidence;
- neither is a portable project audit log, because reflogs are local and expire;
- mutable refs cannot substitute for stable feature identity.

Hidden refs can protect otherwise unreachable objects from garbage collection,
as StGit demonstrates, but that benefit matters only if tpatch moves
authoritative data into Git's object database. It currently does not need to.

### 2.4 Index and working tree

Git's split among HEAD, index, and working tree is directly relevant to record
capture:

- HEAD supplies a committed baseline;
- the index supplies an explicit staged boundary;
- the working tree supplies unstaged and untracked edits.

tpatch capture modes build on this existing three-way model. A database would
not improve the correctness of deciding which bytes belong to a feature; that
remains a Git/path/claim boundary problem.

### 2.5 Trailers

Commit trailers are visible, grep-able metadata inside the commit message.
tpatch's land trailer block is therefore a strong Git projection:

- `Tpatch-Feature`
- `Tpatch-Patch-SHA`
- `Tpatch-Recipe-SHA`
- `Tpatch-Base-Commit`

Trailers travel with ordinary commits and are visible in `git log`, but they are
commit-scoped. Rebase or amend creates a new commit object. The stable binding is
the feature slug and current patch identity, not the old commit SHA.

The lesson from jj and git-gud is the same: stable logical identity must be
separate from moving commit identity. tpatch already maps that to slug plus patch
generation/hash.

### 2.6 Git notes

Git notes attach blobs to objects without changing those objects and can carry
text or JSON under `refs/notes/*`. They are attractive because they support
retroactive metadata.

They are still a poor source of truth for tpatch:

- notes refs require explicit distribution policy;
- ordinary PR views do not show them;
- rewrite propagation needs configuration;
- note merges have their own conflict semantics;
- many tools and agents do not fetch or display them by default.

Possible use: a fully regenerable commit annotation or experiment. Current
recommendation: do not add such a projection until a concrete consumer needs it.
The land trailers and tracked files already cover the visible use cases.

### 2.7 `git patch-id`

`git patch-id` computes a reasonably stable diff identity while ignoring line
numbers. `--stable` also removes file-order sensitivity. It is valuable for
likely-equivalence and upstream duplicate detection.

It is not a complete feature identity:

- one feature may span several commits;
- one commit may contain several features;
- semantic rewrites may not preserve patch ID;
- collision resistance and canonical patch SHA serve different purposes;
- a patch ID says nothing about request, recipe, dependency generation, or
  reconcile evidence.

tpatch is correct to persist both exact SHA-256 and Git patch ID with an
algorithm marker.

### 2.8 Reachability and rewrite

Commit IDs change under amend, rebase, parent change, metadata change, or message
change. Old commits survive only while reachable from refs or reflogs and may
later be garbage-collected.

This creates two identity classes:

| Identity class | tpatch example | Behavior |
|---|---|---|
| Stable logical identity | feature slug | Survives patch rewrite |
| Moving content/version identity | generation ID, patch SHA, commit SHA | Advances when content or context changes |

Git alone supplies only the moving commit identity. tpatch must retain its own
stable logical identity and generation chain.

### 2.9 Git conclusion

Git is the right transport, history, review, and patch-comparison substrate. It
is not a complete feature-state database. Tracked `.tpatch/` files are the layer
that gives Git history stable feature meaning.

## 3. Dolt research brief

### 3.1 What problem Dolt solves

Dolt applies Git-like version control to SQL tables:

- branch, commit, diff, merge, clone, push, and pull;
- SQL queries over current and historical data;
- versioned schema and data;
- system tables for commits, branches, diffs, history, conflicts, and ancestry.

That is a real and valuable problem class: mutable relational data normally has
excellent current-state queries but weak branchable history.

### 3.2 Storage model

Dolt combines:

1. content-addressed Prolly Trees for table data and schema;
2. structural sharing among unchanged subtrees;
3. a Merkle commit graph for database versions;
4. working and staged states analogous to Git.

The central algorithmic benefit is that diff cost scales with changed data
rather than total table size. That benefit matters for large tables and many
versions.

### 3.3 Branch, merge, and schema semantics

Data and schema changes are versioned together. Branches can diverge and merge;
row and schema conflicts are represented explicitly. SQL system tables expose
history and conflict state.

This is stronger than plain SQLite history and much more queryable than scanning
JSON files. It is also a different product boundary: the database becomes the
version-control system for its records.

### 3.4 Mapping Dolt concepts to tpatch

| tpatch artifact | Dolt-like table model |
|---|---|
| `status.json` | `features` current-state table |
| `status.json:depends_on` | `feature_dependencies` edge table |
| `patch-generations.json` | `patch_generations` history table |
| `reconcile-evidence.jsonl` | `reconcile_evidence` append table |
| `reconcile-revisions.jsonl` | `reconcile_revisions` append table |
| `upstream.lock` | singleton upstream record |
| cross-feature path search | indexed relation over touched paths |

The mapping is conceptually clean. It proves that some tpatch data is naturally
relational. It does not prove that the authoritative representation should be a
relational database.

### 3.5 What Dolt would improve

- ad hoc SQL over history;
- joins across features, generations, dependencies, paths, and evidence;
- indexed queries at very large scale;
- database-level multi-table transactions;
- branch/merge semantics for structured records;
- compact structural sharing across many table versions.

### 3.6 Why Dolt is inappropriate as tpatch's repo-local store

1. **Review mismatch**: the logical data is queryable through Dolt tools, but the
   underlying store is not a normal Markdown/JSON/patch PR diff.
2. **Substrate duplication**: the repository would contain Git history plus a
   second version-control graph for metadata.
3. **Commit coordination**: tpatch would need to define when a Dolt commit and a
   Git commit are atomically related. Failure between them creates split-brain
   history.
4. **Dependency weight**: embedding Dolt is far beyond tpatch's current minimal
   dependency posture; invoking a separate binary creates a runtime prerequisite.
5. **Migration burden**: every existing repository and tool reading `.tpatch/`
   would need a compatibility or export layer.
6. **Scale mismatch**: tpatch currently stores small per-feature records, not
   millions of rows.
7. **Human ownership**: users can inspect and repair JSON/Markdown with ordinary
   tools; a Dolt store requires Dolt-aware tooling.

Dolt works locally without a server, so network dependence is not the objection.
The objection is that its whole database/VCS substrate is heavier and less
reviewable than the problem requires.

### 3.7 What Dolt teaches tpatch

Dolt contributes four useful design lessons:

1. **Version identity should be content-derived where possible.**
2. **Current state and history should be independently queryable.**
3. **Diff and merge should avoid scanning unchanged history.**
4. **Schema changes are part of versioned data and need explicit compatibility
   rules.**

tpatch already applies lessons 1, 2, and 4 in generation/evidence schemas.
Lesson 3 motivates generated indexes or a cache if histories become large.

### 3.8 Dolt conclusion

Dolt is relevant as a design reference and possible future server-side
aggregation technology. It is overkill as tpatch's embedded or repo-local
authoritative store.

## 4. Prior-art competitor storage comparison

### 4.1 Comparison

| System | Durable state | Identity | Rewrite/rebase survival | Human review | tpatch lesson |
|---|---|---|---|---|---|
| Quilt | Patch files, `series`, `.pc/` backups/state | Patch filename | Manual refresh/pop/push; file persists | Excellent | Plain files can be durable product state; explicit scope matters |
| StGit | Git commits plus hidden patch/stack refs and stack metadata | Patch name plus moving commit | Strong; refs update and protect objects | Good through StGit, weak in ordinary PR UI | Hidden refs can provide GC-safe projections but add tooling opacity |
| Mercurial MQ | `.hg/patches/` patch files, `series`, status | Patch filename | Moderate; destructive operations historically risky | Excellent | Keep files visible, but every mutation needs recovery/audit |
| Mercurial evolve | Obsolescence markers and successor graph in Mercurial store | Change across successor changesets | Strong and distributed | Moderate through `hg obslog` | Successor links are the principled rewrite model; complexity is substantial |
| jj | Git commit backend plus `.jj/` operation/view store | Stable change ID plus moving commit ID | Strong; operation DAG and automatic descendant rewrite | Good through jj tooling | Stable/moving duality and operation log are useful; custom store is a tooling commitment |
| gbp-pq | `debian/patches/` files as truth plus transient patch-queue branch | Patch filename plus `Gbp-Pq` mapping | Regenerate editable branch from files | Excellent | Best precedent for file truth plus optional Git projection |
| git-gud | Commit trailers plus clone-local `.git` config and remote PR mapping | Stable trailer ID on commits | Good when messages preserve trailers | Moderate | Trailers are good stable overlays; clone-local config must not be truth |
| git-stk | Ordinary branches as truth; `.git/config` annotations and shared metadata ref are repairable projections | Branch/task | Rebase/restack updates branches; metadata can be rebuilt | Good in normal Git | Treat annotations as rebuildable; visible branches remain truth |
| Graphite | Git branches/PRs plus proprietary local/remote metadata | Branch/PR stack | Restack rewrites descendants and keeps PR relations | Strong in Graphite UI, limited public storage visibility | Remote visualization can be valuable, but public evidence is insufficient to copy its storage |
| darcs | `_darcs/` patch store | Patch identity in patch theory | Independent patches commute; amend creates new identity | Good through darcs tooling | Dependency and commutation concepts help reconcile, not storage replacement |
| Pijul | `.pijul/` content-addressed database and channels | First-class change hash | Strong by patch-theory design | Moderate through Pijul tooling | Conflict resolutions as reusable changes are aspirational; opaque custom DB is not a fit |

### 4.2 Cross-system patterns

#### Pattern A: visible files remain the most interoperable truth

Quilt, MQ, and gbp-pq show that plain patch files and small text indexes can
remain useful for decades. They are easy to archive, review, edit, and migrate.
Their weaknesses are workflow safety and rewrite semantics, not lack of SQL.

#### Pattern B: stable identity must not be a commit SHA

jj change IDs, evolve successor markers, patch names, and commit trailers all
separate logical identity from one physical version. tpatch's slug plus patch
generation model is aligned with this prior art.

#### Pattern C: hidden metadata needs a recovery story

StGit protects hidden objects through refs and stack-state commits. git-stk can
repair annotations from branches/review bases and shares metadata through an
explicit ref. Clone-local config without reconstruction is fragile.

tpatch gets a simpler recovery story because tracked `.tpatch/` files clone with
the repository and remain visible in normal diffs.

#### Pattern D: operation logs solve a different problem than databases

jj and evolve provide rewrite/operation history because their core data model
needs to explain mutation of history. A future tpatch operation log may be
valuable, but JSONL or one-file-per-operation records remain the first substrate
to try. A SQL database is not a prerequisite for an operation DAG.

#### Pattern E: patch-theory systems are conceptual references

darcs and Pijul make patches or changes the VCS substrate. They demonstrate
stable change identity, dependency, commutation, and reusable conflict
resolution. tpatch can borrow those concepts for reconcile evidence while
remaining on Git and retaining reviewable artifacts.

### 4.3 Public-source caveats

- `stk` is name-ambiguous. The public `lararosekelley/git-stk` repository
  available on 2026-07-16 stores stack-parent annotations in `.git/config`,
  publishes a repairable `refs/stk/metadata`, and treats branches as real state.
  This differs from the older `.git/stacks/<name>.yaml` summary in the local
  landscape and should be rechecked when that market document is refreshed.
- Graphite's exact local metadata schema is not fully documented in first-party
  public sources. Storage claims beyond ordinary branches, PRs, and restack
  behavior should be treated as secondary-source observations.
- git-gud's trailer persistence depends on preserving the commit message during
  rewrite; no trailer survives an operator intentionally dropping it.

## 5. Emerging and agent-aware storage scan

### 5.1 SQLite

SQLite is serverless, transactional, mature, and queryable. A CGo-free Go driver
exists through `modernc.org/sqlite`, although it adds a material dependency and
version-pinning surface.

#### Source-of-truth role

Not recommended:

- the database file is not reviewable in a normal PR;
- page layout is not a deterministic logical serialization contract;
- WAL/journal files add hidden operational state;
- manual repair requires SQLite tools;
- schema migrations become a product contract;
- agents and scripts that currently read files directly would need SQL access.

#### Cache/projection role

Potentially useful:

```text
tracked Markdown/JSON/JSONL/patch files
                 |
                 v
      gitignored SQLite read model
```

Good candidate queries include:

- all features touching path X;
- all stale children of generation Y;
- evidence attempts by phase, confidence, or upstream commit;
- cross-repo dashboards;
- full-text search over human summaries;
- large decision-ticket frontier queries.

Required cache contract:

1. deleting the database loses no project truth;
2. a full rebuild from tracked files is always possible;
3. stale or schema-incompatible caches are discarded, not migrated in place;
4. mutating commands write canonical files first;
5. correctness paths fall back to canonical files;
6. the cache path is gitignored and clearly named as derived state;
7. cache version and input digest are explicit.

Current conclusion: defer until measurement proves a need.

### 5.2 Deterministic generated indexes

Generated indexes preserve the most important tpatch properties while improving
aggregate reads. Candidate shape:

```text
.tpatch/index/
├── features.json
├── dependencies.json
├── touched-paths.json
└── evidence-summary.json
```

Not all of these should be created. Start with one measured query problem.

Index rules:

- canonical inputs are named explicitly;
- entries and arrays are sorted deterministically;
- serialization is schema-versioned;
- rebuild is a command or automatic operation;
- stale index never blocks reading source files;
- generated files are either committed for PR visibility or gitignored for
  acceleration, but the choice is explicit per index.

Because `FEATURES.md` already follows this pattern, generated JSON is the lowest
risk scaling path.

### 5.3 CRDT and local-first stores

Automerge and Loro provide local copies, offline mutation, history, and automatic
merge of concurrent edits. These are valuable for collaborative applications.

They do not currently fit tpatch:

- their native persisted forms are binary rather than PR-readable;
- Go embedding is weak or unavailable for the strongest candidates;
- automatic field merge does not resolve semantic conflicts in lifecycle state;
- concurrent mutation of one `status.json` should normally be prevented or
  isolated, not merged field-by-field;
- Git worktrees and branches already provide coarse-grained multi-agent
  isolation.

Revisit CRDTs only if tpatch acquires a requirement for multiple offline replicas
to concurrently edit the same logical operation log and later merge without a
Git-level review step.

### 5.4 Content-addressed stores

Git already supplies the content-addressed store for tracked artifacts. IPFS,
OrbitDB, Noms, and similar systems add distribution or data-model machinery
without improving ordinary PR review.

Content-addressing should continue at the logical record level:

- patch SHA-256;
- generation ID;
- evidence attempt ID;
- revision entry ID;
- recipe SHA;
- optional future manifest hashes.

There is no need to add a second blob-addressing substrate merely to obtain
hashes.

### 5.5 Agent memory systems

Mem0 and Graphiti/Zep are designed for semantic and temporal retrieval from
agent interactions. They may retain entities, facts, episodes, embeddings, and
conversation-derived memory.

They are not deterministic project state:

- extraction depends on models and prompts;
- retrieval is probabilistic;
- stores may require cloud services, graph databases, or embedding models;
- normal Git review cannot show the complete state transition;
- privacy and retention risks are much higher than for hashes and enums.

Use case boundary:

- acceptable: optional retrieval aid that points an agent to canonical files;
- unacceptable: deciding feature state, dependency satisfaction, patch
  generation, or reconcile outcome from agent memory alone.

### 5.6 Agent artifact branches

Entire provides a more relevant pattern than semantic memory: agent sessions are
linked to commits while metadata lives on a separate Git branch.

The lesson is mixed:

- a separate branch keeps active code history clean;
- commit linkage and checkpoints can aid recovery;
- branch storage still needs explicit fetch/push and review policy;
- raw prompts, transcripts, files, and tool inputs create serious privacy and
  repository-size concerns.

tpatch's current privacy boundary is safer: persist deterministic summaries,
paths, hashes, operation IDs, and verdicts; do not persist raw transcripts,
source bodies, or embeddings by default. A future optional raw-session store
should remain non-authoritative and separately consented.

### 5.7 Agent-native protocols

MCP standardizes how agents access data sources and tools. It is not a durable
storage format.

A future tpatch MCP server could expose:

- feature status;
- canonical artifact paths;
- DAG queries;
- evidence search;
- generation lookup;
- safe command execution.

That would improve agent ergonomics without changing the file store. Protocol
adapters should read canonical files or their rebuildable indexes.

### 5.8 Git notes and custom refs as projections

They remain technically valid but low priority:

- good for object-attached metadata;
- content-addressed through Git;
- invisible in normal PR workflows;
- distribution and rewrite policies are easy to misconfigure;
- no clear advantage over tracked indexes and trailers for current use cases.

Recommendation: no authoritative use; no projection until a specific
commit-attached query cannot be served by trailers plus `.tpatch/`.

## 6. Decision matrix

Scores: 5 = strongest fit, 1 = weakest fit. Hidden-state risk is reversed:
5 = low risk, 1 = high risk.

| Option | PR review | Offline | Determinism | Minimal Go fit | Human readable | Queryability | 1k+ scale | Agent ergonomics | Low hidden risk | Low migration cost |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Current tracked files | 5 | 5 | 5 | 5 | 5 | 2 | 3 | 5 | 5 | 5 |
| Deterministic JSON index | 5 | 5 | 5 | 5 | 5 | 3 | 4 | 5 | 5 | 5 |
| Gitignored SQLite projection | 1 | 5 | 3 | 3 | 1 | 5 | 5 | 4 | 4 | 3 |
| Git notes/custom-ref projection | 1 | 5 | 5 | 5 | 3 | 2 | 3 | 2 | 2 | 3 |
| Dolt repo-local authority | 1 | 5 | 5 | 1 | 1 | 5 | 5 | 4 | 2 | 1 |
| CRDT binary authority | 1 | 5 | 4 | 1 | 1 | 3 | 5 | 3 | 3 | 1 |
| Remote agent memory | 1 | 1 | 1 | 1 | 1 | 5 | 5 | 5 | 1 | 1 |

### 6.1 Lifecycle interaction

| Option | `record` | `land` | `reconcile` | `verify` | Patch generations |
|---|---|---|---|---|---|
| Tracked files | Native producer and authority | Trailers project file hashes | Reads/writes status, patch, evidence | Hashes canonical files | Native manifest |
| Generated JSON index | Refresh after canonical write; never block record | Read-only lookup | Accelerate selection/reporting | Accelerate aggregate status | Summary only |
| SQLite projection | Rebuild/update after files commit successfully | Optional query aid | Evidence/path query acceleration | Aggregate cache | Mirror rows only |
| Git notes/refs | No capture benefit | Optional object annotation | Rewrite/fetch risk | No core benefit | Must never replace manifest |
| Dolt authority | Requires dual Git/Dolt transaction model | Needs commit-to-DB commit binding | Moves evidence/state into second VCS | Requires SQL path | Replaces shipped schema and breaks compatibility |
| CRDT authority | Merges metadata, not patch-byte boundaries | No natural Git projection | Semantic verdict conflicts remain | Binary state requires adapter | Weak fit for immutable generation records |
| Agent memory | May suggest context only | No authoritative role | Retrieval hint only | No deterministic role | Must not create identity |

## 7. What tpatch should keep

### 7.1 Keep as plain Markdown

- request, analysis, specification, exploration, and record narratives;
- whitepapers, PRDs, ADRs, and decision explanations;
- human-readable generated dashboards;
- redacted/summarized agent context when explicitly approved.

Markdown is appropriate where review, explanation, and ordinary editing matter
more than strict field-level queries.

### 7.2 Keep as deterministic JSON

- current feature status;
- apply recipes;
- claims;
- patch generation manifests;
- bounded structured session summaries;
- upstream/config records where the existing simple format remains sufficient;
- generated indexes when a real query need appears.

JSON is appropriate when schema validation and machine interoperability matter,
while PR review remains essential.

### 7.3 Keep as JSONL or one-record-per-file

- append-only reconcile evidence;
- review corrections;
- a possible future operation log;
- high-contention audit events.

JSONL is compact and streamable. One-record-per-file may be preferable when many
agents append concurrently because Git merges disjoint files more naturally than
one shared log. That is a future concurrency decision, not a reason to adopt a
database.

### 7.4 Keep as patch files

- canonical replay patch;
- historical full-diff snapshots;
- operation-specific reverse or repair patches where their semantics are clearly
  separated from canonical history.

Patch bytes remain the most direct, tool-compatible representation of the code
change.

### 7.5 Keep Git identifiers as links and evidence

- base/upstream commit SHAs;
- land trailers;
- patch SHA-256;
- `git patch-id --stable`;
- reachability evidence.

These identifiers should never be mistaken for stable feature identity.

## 8. What tpatch could add

### 8.1 First addition: measured generated indexes

Before opening a PRD, benchmark current scans at representative sizes such as
100, 1,000, and 10,000 features with realistic generation/evidence counts.

If a bottleneck is proven, the first candidate is a deterministic
`.tpatch/index/features.json` or query-specific index. It must remain derived and
rebuildable.

### 8.2 Second addition: optional SQLite read model

Only after a generated index is insufficient, consider a gitignored SQLite
cache. Its purpose would be ad hoc joins, full-text search, and large aggregate
queries. It would not be committed and would not participate in record, land, or
reconcile correctness.

### 8.3 Optional access adapters

An MCP server, language-server-like indexer, or local query command could expose
the canonical store to agents. These are interfaces over the files, not new
authorities.

### 8.4 Future server-side aggregation

A shared service could index multiple repositories for dashboards or fleet
analysis. Dolt, PostgreSQL, or another database may be reasonable there. The
server must be rebuildable from repository exports and must not make a local repo
unusable when offline.

## 9. What tpatch should avoid

- a repo-local authoritative Dolt or SQLite database;
- lifecycle state stored only in Git notes, custom refs, reflogs, or `.git/`
  config;
- remote SaaS as the only copy of project metadata;
- model-derived agent memory as deterministic truth;
- CRDT adoption without a concrete same-record concurrent editing requirement;
- raw prompt/transcript/vector persistence in normal tracked feature artifacts;
- dual writes where Git and a database can disagree without a recovery protocol;
- a generic storage abstraction before a second real authoritative substrate
  exists;
- a tpatch-native VCS for non-Git mode.

## 10. Scenario-specific guidance

### 10.1 Normal feature repository

Use tracked files only. The current layout is the best fit. No database, hidden
ref, or cache is justified.

### 10.2 Large repository with hundreds of features

Keep the same truth model. Add measurement and, if needed, one deterministic
generated index. Hundreds of small JSON files are not by themselves a database
problem.

At thousands of features or large audit histories, a gitignored SQLite
projection becomes reasonable if it materially improves named queries.

### 10.3 Multi-agent tesseraworkspaces-style environment

Prefer workspace and branch isolation:

- each agent works in its own worktree/branch;
- tracked `.tpatch/` changes merge through Git;
- agents claim features or tasks to reduce same-file writes;
- append-only records use disjoint files where merge contention is real;
- same-feature current-state updates use explicit ownership, locking, or
  compare-before-write semantics.

A shared database can serialize writes, but it also creates a hidden coordination
service and bypasses normal PR review. It is not a substitute for work ownership
and merge semantics.

### 10.4 Future decision-ticket project

WP-007's separation remains correct. Decision tickets should use their own
Markdown/JSON graph outside `.tpatch/features/` and link to tpatch slugs,
generation IDs, PRDs, ADRs, commits, or workspaces.

If that project reaches thousands of tickets, SQLite may be a good local
projection for frontier/blocker queries. Ticket files should remain authoritative
unless the future product explicitly chooses a different architecture.

### 10.5 Non-Git metadata-only mode

Use the same Markdown/JSON files for request, analysis, specification, and
configuration. Without Git:

- no canonical Git-derived patch capture;
- no land trailers;
- no commit reachability;
- no `git patch-id`;
- no normal reconcile/verify freshness semantics.

The store remains local-first but loses automatic clone/push review and
distribution. Metadata-only mode should be visibly degraded and should not grow
a hidden database or tpatch-native snapshot engine to simulate Git.

### 10.6 Shared multi-repository service

This is the one scenario where a database can be authoritative for the
**service's aggregate view**, but not for each repository's feature truth.
Repositories remain the durable roots; the service is a rebuildable index.

## 11. Candidate follow-up PRDs, ADRs, and experiments

No follow-up is required to preserve the current architecture.

### 11.1 Candidate experiment: storage/query scale benchmark

Measure:

- `status` and `list` over 100/1,000/10,000 features;
- DAG validation and topological ordering;
- generation manifest loading;
- evidence filtering;
- touched-path reverse lookup;
- cold vs warm agent read cost.

The experiment should name a latency or memory threshold before proposing an
index.

### 11.2 Candidate PRD: deterministic feature index

Open only if the benchmark proves a problem.

Candidate contract:

- `.tpatch/index/features.json`;
- schema-versioned and deterministic;
- generated from per-feature canonical files;
- atomic replacement;
- stale/missing index falls back to source scan;
- no command treats it as authoritative.

### 11.3 Candidate ADR: derived index and cache authority boundary

Required only if tpatch adds SQLite or multiple generated indexes. It should lock:

- canonical input files;
- rebuild and invalidation rules;
- failure behavior;
- whether each index is tracked or gitignored;
- privacy exclusions;
- no cache-only fields;
- no correctness dependency on cache availability.

### 11.4 Candidate PRD: SQLite query cache

Open only after generated JSON is proven insufficient. The PRD should name the
specific query workload and benchmark improvement. "SQL might be useful" is not
enough.

### 11.5 Candidate concurrency study

If same-feature concurrent agent writes become common, study:

- optimistic compare-before-write;
- repo-local advisory locks;
- one-record-per-file audit logs;
- workspace ownership;
- conflict presentation.

Do not assume a database is the answer before characterizing the races.

### 11.6 WP-008 decision

Do not create WP-008 now.

Create `WP-008-storage-substrate-and-tpatch.md` only when one of these proposals
has a real sponsor and evidence:

- authoritative database state;
- authoritative hidden refs or notes;
- durable remote service state;
- shared multi-agent operation database;
- non-Git change tracking;
- replacement of canonical patch/file authority.

## 12. Open questions

1. At what measured feature/evidence count does directory scanning become a user
   problem?
2. Which first query cannot be answered cleanly by a deterministic JSON index?
3. Should a generated machine index be committed for PR review or gitignored to
   avoid merge noise?
4. Would one-file-per-event audit storage reduce multi-agent merge conflicts
   enough to justify more files than JSONL?
5. Should raw agent-session storage remain entirely outside tpatch, or should
   tpatch define an optional manifest/link protocol?
6. In metadata-only mode, what export/backup guidance replaces Git distribution?
7. For a future shared service, what repository export proves that the service
   can be rebuilt without hidden state?
8. Should decision-ticket links live only on the ticket side, as WP-007
   recommends, or is a small optional external-link field eventually useful in
   feature metadata?

## 13. References

All external URLs were accessed on **2026-07-16**.

### 13.1 tpatch sources

- [`docs/feature-layout.md`](../feature-layout.md) - canonical patch vs audit
  snapshots; feature-to-commit trailers.
- [`docs/record.md`](../record.md) - capture semantics and collision behavior.
- [`docs/reconcile.md`](../reconcile.md) - Git-first reconcile assumptions.
- [`docs/dependencies.md`](../dependencies.md) - `status.json` DAG authority.
- [`docs/whitepapers/WP-001-feature-slice-gap.md`](../whitepapers/WP-001-feature-slice-gap.md)
  - no data-model gap; canonical patch authority.
- [`docs/whitepapers/WP-002-capture-and-metadata-foundation.md`](../whitepapers/WP-002-capture-and-metadata-foundation.md)
  - lightweight manifests and current/history split.
- [`docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md`](../whitepapers/WP-003-reconcile-safety-and-middle-pass.md)
  - evidence/audit boundary and privacy rules.
- [`docs/whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md`](../whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md)
  - Git-first and metadata-only non-Git mode.
- [`docs/whitepapers/WP-007-decision-tickets-and-ticket-tracking.md`](../whitepapers/WP-007-decision-tickets-and-ticket-tracking.md)
  - decision-ticket separation and hybrid links.
- [`docs/adrs/ADR-015-prior-art-identity-mapping.md`](../adrs/ADR-015-prior-art-identity-mapping.md)
  - stable/moving identity and prior-art mapping.
- [`docs/adrs/ADR-024-patch-generation-manifest-boundary.md`](../adrs/ADR-024-patch-generation-manifest-boundary.md)
  - generation identity, strict schema, deterministic writes.
- [`docs/adrs/ADR-025-reconcile-evidence-and-revision-schema.md`](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md)
  - JSONL evidence/revision schema and corruption behavior.
- [`docs/market-research/competitive-landscape.md`](../market-research/competitive-landscape.md)
  - prior-art lanes and tpatch positioning.
- `internal/store/store.go` - file store, derived `FEATURES.md`, aggregate scans.
- `internal/store/types.go` - current lifecycle, DAG, verify, and reconcile
  records.
- `internal/store/patch_generations.go` - content-addressed deterministic
  generation manifest.
- `internal/store/reconcile_evidence.go` - append-only canonical JSONL evidence.
- `internal/store/dag.go` - deterministic in-memory DAG algorithms.
- `internal/gitutil/patch_id.go` - `git patch-id --stable` integration.

### 13.2 Official Git documentation

- [Git objects](https://git-scm.com/book/en/v2/Git-Internals-Git-Objects)
- [Git packfiles](https://git-scm.com/book/en/v2/Git-Internals-Packfiles)
- [Git references](https://git-scm.com/book/en/v2/Git-Internals-Git-References)
- [Git maintenance and data recovery](https://git-scm.com/book/en/v2/Git-Internals-Maintenance-and-Data-Recovery)
- [Rewriting history](https://git-scm.com/book/en/v2/Git-Tools-Rewriting-History)
- [`git notes`](https://git-scm.com/docs/git-notes)
- [`git interpret-trailers`](https://git-scm.com/docs/git-interpret-trailers)
- [`git patch-id`](https://git-scm.com/docs/git-patch-id)

### 13.3 Official Dolt sources

- [What is Dolt?](https://docs.dolthub.com/introduction/what-is-dolt)
- [Dolt storage engine](https://docs.dolthub.com/architecture/storage-engine)
- [Prolly Trees](https://docs.dolthub.com/architecture/storage-engine/prolly-tree)
- [Dolt commit graph](https://docs.dolthub.com/architecture/storage-engine/commit-graph)
- [Dolt system tables](https://docs.dolthub.com/reference/sql/version-control/dolt-system-tables)
- [Dolt installation](https://docs.dolthub.com/introduction/installation)
- [Dolt source repository](https://github.com/dolthub/dolt)

### 13.4 Prior-art official sources

- [Quilt - GNU Savannah](https://savannah.nongnu.org/projects/quilt)
- [Using Quilt - Debian Wiki](https://wiki.debian.org/UsingQuilt)
- [StGit repository](https://github.com/stacked-git/stgit)
- [StGit tutorial](https://stacked-git.github.io/guides/tutorial/)
- [Mercurial MQ](https://www.mercurial-scm.org/wiki/MqExtension)
- [Mercurial evolve](https://www.mercurial-scm.org/wiki/EvolveExtension)
- [Mercurial changeset evolution](https://www.mercurial-scm.org/wiki/ChangesetEvolution)
- [Jujutsu repository](https://github.com/jj-vcs/jj)
- [Jujutsu glossary](https://docs.jj-vcs.dev/latest/glossary/)
- [Jujutsu concurrency](https://docs.jj-vcs.dev/latest/technical/concurrency/)
- [gbp-pq manual](https://honk.sigxcpu.org/projects/git-buildpackage/manual-html/man.gbp.pq.html)
- [git-gud repository](https://github.com/mrmans0n/git-gud)
- [git-stk repository](https://github.com/lararosekelley/git-stk)
- [Graphite introduction](https://graphite.dev/blog/introducing-graphite)
- [darcs](https://darcs.net/)
- [The darcs book](https://darcsbook.acmelabs.space/)
- [Pijul](https://pijul.org/)
- [Pijul project repository](https://nest.pijul.com/pijul/pijul)

### 13.5 Emerging and agent-aware official sources

- [Local-first software](https://www.inkandswitch.com/essay/local-first/)
- [Automerge](https://automerge.org/docs/hello/)
- [Loro](https://github.com/loro-dev/loro)
- [CR-SQLite](https://github.com/vlcn-io/cr-sqlite)
- [CGo-free SQLite Go driver](https://pkg.go.dev/modernc.org/sqlite)
- [DuckDB Go client](https://duckdb.org/docs/current/clients/go.html)
- [libSQL](https://github.com/tursodatabase/libsql)
- [Entire CLI](https://github.com/entireio/cli)
- [Mem0](https://github.com/mem0ai/mem0)
- [Graphiti](https://github.com/getzep/graphiti)
- [Model Context Protocol](https://modelcontextprotocol.io/docs/getting-started/intro)

### 13.6 Secondary/commentary sources

- Graphite's exact local metadata is not fully specified in first-party public
  documentation. Secondary competitive analyses can be used to generate
  hypotheses, but this paper does not rely on an undocumented Graphite schema for
  its recommendation.

## Disputes

None. The research found no evidence that requires replacing tpatch's current
authoritative file model.

---

**Research authored by**: S66 (2026-07-16)
