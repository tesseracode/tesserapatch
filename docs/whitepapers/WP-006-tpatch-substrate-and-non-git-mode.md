# WP-006 — tpatch Substrate and Non-Git Mode

**Status**: Exploring
**Authors**: V64
**Started**: 2026-07-10
**Turn log**: [WP-006-tpatch-substrate-and-non-git-mode.turns.md](./WP-006-tpatch-substrate-and-non-git-mode.turns.md)
**Related**:
- [SPEC](../../SPEC.md)
- [Feature layout](../feature-layout.md)
- [Recording patches](../record.md)
- [Landing features as Git commits](../land.md)
- [Reconcile workflow](../reconcile.md)
- [WP-001 Feature-slice gap and intent-VCS direction](./WP-001-feature-slice-gap.md)
- [WP-002 Capture and metadata foundation](./WP-002-capture-and-metadata-foundation.md)
- [WP-003 Reconcile safety and middle-pass foundation](./WP-003-reconcile-safety-and-middle-pass.md)
- [Competitive landscape](../market-research/competitive-landscape.md)
- [ADR-015 Prior-art identity mapping](../adrs/ADR-015-prior-art-identity-mapping.md)

## 0. Intake rebaseline *(O54, 2026-09-22)*

This paper remains **Exploring**. It does not authorize substrate
implementation, a pluggable substrate interface, or tpatch-native non-Git
change tracking.

Current shipped guidance has moved beyond the July code snapshot in one
important respect: `docs/path-b-operator-guide.md` now documents mutating
`prepare` behavior for non-Git and unusable-Git workspaces. Any future PRD
must reconcile with that shipped behavior instead of silently overriding it.

The line-level citations below were not fully revalidated during this intake.
Treat them as an exploratory baseline. If this topic is reopened, the scope
should stay narrow: explicit Git preflight / degraded-mode planning only, with
the paper's Git-first recommendation preserved unless a later ADR explicitly
changes it.

## 1. Context *(V64)*

The immediate design question is small enough to become a PRD: `tpatch init`
currently creates `.tpatch/` and installs skill files without first proving the
directory is a Git worktree. The larger design question is not PRD-sized:
supporting non-Git repositories with tpatch-native file snapshots or a pluggable
change-tracking substrate would touch tpatch's patch authority model,
reconcile, land trailers, patch generations, operation-log plans, and the
"stay out of Git's way" posture from WP-001 / ADR-015.

This paper therefore recommends **both**:

1. a near-term PRD for explicit Git preflight / init UX; and
2. no tpatch-native non-Git change-tracking substrate without a later ADR that
   explicitly supersedes the Git-first assumptions documented here.

The strong default position survives the first pass: tpatch should remain
Git-first. If non-Git mode exists in v1, it should be metadata-only, visibly
degraded, and should not pretend `record`, `land`, `reconcile`, patch-id
detection, or freshness checks work without Git.

## 2. Current Git assumptions in tpatch *(V64)*

### 2.1 Product and data-model assumptions

The product purpose is explicitly upstream/fork oriented: tpatch customizes
upstream open-source projects while preserving enough structure to reapply,
review, and reconcile when upstream evolves (`SPEC.md:5-8`). The core lifecycle
ends in `record` and `reconcile`, and the command table names `record`,
`reconcile`, `upstream check`, `replay`, and `land`-adjacent workflows as core
surfaces (`SPEC.md:19-78`).

The `.tpatch/` model includes `upstream.lock`, feature patch artifacts, and
reconciliation artifacts (`SPEC.md:79-111`). Feature layout says
`artifacts/post-apply.patch` is the canonical replay input and is a full diff
against `status.json:apply.base_commit` (`docs/feature-layout.md:34-44`,
`:84-88`). That base commit is a Git commit by current design.

`land` makes the Git dependency explicit: it creates one ordinary Git commit,
adds the locked `Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`, and
`Tpatch-Base-Commit` trailer block, and uses `git log --grep` as the canonical
feature-to-commit lookup (`docs/land.md:1-6`, `:77-97`, `:148-159`).

`reconcile` assumes a Git working tree at an upstream target state, uses
`git apply --reverse --check`, `git apply --3way`, and isolated `git worktree`
preview, and documents Git workflows for both pristine-main and
feature-as-commit patterns (`docs/reconcile.md:5-18`, `:20-45`, `:70-83`).

### 2.2 Current implementation behavior

Current `tpatch init` does **not** check Git. The command resolves the target
path, calls `store.Init(root)`, installs skill files, auto-detects provider
settings, and prints initialization output (`internal/cli/cobra.go:87-124`).
`store.Init` creates `.tpatch/`, writes `config.yaml`, `FEATURES.md`,
`upstream.lock`, and steering files; it also has no Git check
(`internal/store/store.go:42-130`).

Commands that only need the store can plausibly work outside Git today:

| Command family | Git dependency in current code | Evidence |
|---|---|---|
| `init` | none explicit | `internal/cli/cobra.go:87-124`, `internal/store/store.go:42-130` |
| `add` | none explicit; writes feature request/status/index | `internal/cli/cobra.go:128-171`, `internal/store/store.go:146-207` |
| `status` | mostly store/config reads; may derive labels from stored refs | `internal/cli/cobra.go:193-250` |
| `config show/set` | store/config reads/writes | `internal/cli/cobra.go:2627-2729` |
| `provider check/set` | provider/config only, but `check` currently opens repo store | `internal/cli/cobra.go:2450-2602` |

Git-required commands fail indirectly today, often through low-level Git errors
rather than a substrate-aware diagnostic:

| Command / feature | Git dependency | Evidence |
|---|---|---|
| `record` | `git diff`, `git ls-files`, `git add --intent-to-add`, `git reset`, `git apply --reverse --check`, `git rev-parse HEAD` | `internal/gitutil/gitutil.go:214-338`, `:405-436`; `internal/cli/cobra.go:939-960`, `:1355-1388` |
| `reconcile` | `git status --porcelain`, `git grep`, `git apply`, `git worktree`, upstream refs / lock guard | `internal/gitutil/gitutil.go:126-187`, `:489-555`; `internal/cli/cobra.go:1798-1855` |
| `land` | Git commit/staging/trailer contract | `docs/land.md:1-6`, `:50-64`, `:66-97`, `:148-159` |
| `verify` freshness overlays | hashes recipe/patch and records parent snapshots; freshness labels reference base commits and parent refs | `internal/store/types.go:174-211`, `docs/feature-layout.md:84-92` |
| patch-id detector | `git patch-id --stable` and upstream commit scan | `internal/store/types.go:282-288`, `:349-358` |

The most important current-behavior gap is not that Git-required commands fail;
they should fail outside Git. The gap is that `init` can create a workspace whose
most important future commands are impossible, and later failures may surface as
generic Git plumbing errors rather than a clear tpatch substrate contract.

### 2.3 Existing config shape

Repo config is currently flat YAML with keys such as `merge_strategy`,
`features_dependencies`, and detector thresholds written by `store.Init`
(`internal/store/store.go:66-96`). `Config` has no `substrate` field
(`internal/store/types.go:315-374`). The current `config set` parser supports a
fixed set of flat keys and rejects unknown keys (`internal/cli/cobra.go:2688-2721`).

Implication: a nested shape like:

```yaml
substrate:
  type: git
```

is desirable long-term, but a near-term PRD must either extend the config parser
for nested maps or choose a flat key such as `substrate_type: git` for v1.

## 3. Prior art / competitive landscape *(V64)*

ADR-015's accepted framework says stable identity is what the user names and
moving identity is what bytes-on-disk hash to; for tpatch, that maps to
`slug` and `Tpatch-Patch-SHA` (`docs/adrs/ADR-015-prior-art-identity-mapping.md:91-119`).
This is already Git-adjacent because land trailers project those identities into
Git commits.

ADR-015 explicitly rejects a StGit-style architecture where every patch is a Git
commit, because tpatch's design keeps the patch in `.tpatch/` separate from main
history (`docs/adrs/ADR-015-prior-art-identity-mapping.md:183-205`). It also
rejects jj's working-copy-as-commit model and defers distributed obsolescence
(`docs/adrs/ADR-015-prior-art-identity-mapping.md:234-264`). Those rejections do
not ban future substrate work, but they set a high bar: substrate changes need a
new ADR, not an incidental PRD.

The competitive landscape adds the broader map: Quilt, StGit, hg MQ/evolve, jj,
and gbp-pq all solve patch identity/history differently, but tpatch's lane is
"patch-as-artifact in `.tpatch/`, deterministic apply-recipe, AI-assisted
4-phase reconcile, multi-format skill emission" (`docs/market-research/competitive-landscape.md:52-63`,
`:113-121`). Patch-theory systems such as darcs and Pijul are inspirational only;
the doc explicitly says tpatch is not adopting patch theory and that SPEC commits
to a Git / optionally-jj substrate (`docs/market-research/competitive-landscape.md:129-151`).

Prior-art takeaway: requiring or explicitly initializing Git is consistent with
tpatch's current architecture. Building a hidden tpatch-native VCS is the option
most likely to recreate prior-art complexity while losing Git's interoperability.

## 4. Options A-E *(V64)*

### Option A — Require Git

`tpatch init` refuses unless the target path is inside a Git worktree.

| Pros | Cons |
|---|---|
| Honest: most valuable commands need Git. | Less friendly for brand-new directories. |
| Simplest command matrix and least implementation risk. | Cannot use tpatch as a metadata tracker for non-Git projects. |
| Avoids silently creating unusable workspaces. | Users must run `git init` themselves first. |

Assessment: good strict mode, but too rigid as the only UX. The near-term PRD
should include `--git=require`.

### Option B — Offer to initialize Git

`tpatch init` detects a non-Git directory and can run `git init` only when the
user explicitly requests it, e.g. `tpatch init --git=init`.

| Pros | Cons |
|---|---|
| Friendly setup while preserving explicit consent. | Mutates version-control state, so it must never be implicit in scripts. |
| Produces a fully capable tpatch workspace. | Needs clear docs and tests for non-interactive behavior. |

Assessment: recommended near-term path. Default should **not** auto-run
`git init`; explicit `--git=init` is acceptable.

### Option C — Metadata-only mode

`tpatch init --git=skip` creates `.tpatch/` and records a degraded substrate
mode. Only commands that do not require Git are valid.

| Pros | Cons |
|---|---|
| Lets users capture feature intent before a repo is ready. | Many core commands become disabled. |
| Preserves local-first feature tracking for notes/specs/config. | Could confuse users unless every disabled command errors clearly. |
| Avoids fake patch/snapshot behavior. | Needs command availability matrix and docs. |

Assessment: acceptable only if it is explicit and visibly degraded. It should not
be the silent default for non-Git directories unless the CLI prints a warning and
stores `substrate_type: metadata-only`.

### Option D — Pluggable substrate interface

Abstract Git behind an interface with Git as the only v1 implementation.

| Pros | Cons |
|---|---|
| Names the boundary for future work. | Adds architecture before there is a second real implementation. |
| Could make Git-required diagnostics cleaner. | Risks broad refactors across record/reconcile/land/verify. |

Assessment: possible ADR topic, not a near-term PRD requirement. The near-term
PRD can add substrate preflight helpers without introducing a full abstraction.

### Option E — tpatch-native change tracking

tpatch stores snapshots/diffs/patch generations itself without Git.

| Pros | Cons |
|---|---|
| Would support non-Git projects in theory. | Recreates version control: snapshots, diffs, ignores, moves, binary files, merge, history, recovery. |
| Could preserve intent for non-Git directories. | Conflicts with `land`, `reconcile`, patch-id detection, upstream refs, and Git trailer identity. |
| Might help future non-code artifact tracking. | Requires whitepaper + ADR; not safe as an incremental PRD. |

Assessment: reject for now. It should be reopened only with a dedicated
whitepaper/ADR that explains why Git, jj-on-Git, or metadata-only mode cannot
serve the use case.

## 5. Recommended near-term path *(V64)*

Graduate a small PRD:

```text
docs/prds/PRD-git-init-and-substrate-preflight.md
```

Recommended contract:

1. `tpatch init` detects whether the target path is inside a Git worktree.
2. Existing Git repos preserve current behavior and write substrate metadata
   (`git`) if the PRD chooses to persist it.
3. Non-Git dirs do not silently become fake repos.
4. `--git=require` refuses outside Git with a clear diagnostic.
5. `--git=init` explicitly runs `git init`, then proceeds.
6. `--git=skip` creates a metadata-only workspace and records that degraded
   mode.
7. Default `--git=auto` should be decided by PRD review. V64's recommendation:
   in interactive mode, prompt or print a strong warning; in non-interactive
   mode, refuse unless `--git=init` or `--git=skip` is provided. If prompting is
   out of scope, use `require` as the default for v1 and document `--git=skip`.
8. Git-required commands fail early with a tpatch-level error:

```text
error: tpatch record requires substrate git; this workspace is metadata-only.
Run `git init` and `tpatch substrate adopt-git` (future) or reinitialize with
`tpatch init --git=init`.
```

The PRD should prefer clear, command-specific errors over broad fallbacks. Do
not hide Git failures behind empty patches, empty upstream refs, or no-op
verification.

### 5.1 Command availability matrix

Proposed v1 matrix:

| Command | Git substrate | Metadata-only substrate |
|---|---:|---:|
| `init` | yes | yes, explicit/degraded |
| `add` | yes | yes |
| `status` | yes | yes, but no Git freshness labels |
| `config show/set` | yes | yes |
| `provider check/set` | yes | yes if repo config exists |
| `analyze` | yes | yes; provider/heuristic over request/spec only |
| `define` | yes | yes; writes spec docs only |
| `explore` | yes | degraded; may inspect files but must not rely on Git history |
| `implement` | yes | degraded; can generate recipe, but apply/record gates must explain limits |
| `apply` | yes | maybe prepare-only; execute/done should be reviewed in PRD |
| `record` | yes | no |
| `land` | yes | no |
| `verify` | yes | no for freshness; possible metadata-only lint later |
| `reconcile` | yes | no |
| `upstream check` | yes | no |
| patch-id detector | yes | no |

The PRD must decide whether `apply` remains valid in metadata-only mode. V64's
recommendation is conservative: allow only planning/spec phases (`add`,
`analyze`, `define`, maybe `explore`) until Git exists, because applying code
without any supported capture mechanism creates untracked state tpatch cannot
reconcile.

## 6. Recommended long-term stance *(V64)*

tpatch should remain Git-first and should not build a hidden non-Git VCS.

Long-term, "substrate" should mean a documented capability contract, not a
promise that every substrate can support every command. The only supported v1
change-tracking substrate should be Git. A future `jj` story, if desired, should
likely ride jj's Git compatibility rather than replacing tpatch's Git semantics.

Metadata-only mode can exist as an intent/spec tracker, but it should be honest:
it tracks feature requests and planning artifacts, not patch bytes against an
upstream baseline. Once a user wants record/reconcile/land, they need Git.

Any tpatch-native snapshot substrate would need to answer at least:

1. what is the canonical replay artifact if not `post-apply.patch` from Git diff?
2. how are ignores/pathspecs/untracked files/binary files represented?
3. how are base and upper identities modeled without commits?
4. how does reconcile compare against upstream without upstream refs?
5. how does `land` project into ordinary review workflows without Git commits?
6. how does this avoid duplicating Git, Quilt, StGit, or jj badly?

Those questions are ADR-level.

## 7. Candidate PRDs / ADRs *(candidate only; planning unresolved)* *(V64)*

### 7.1 Recommended PRD

`PRD-git-init-and-substrate-preflight.md`

Scope:

- `tpatch init --git=<auto|require|init|skip>` or equivalent;
- substrate metadata in repo config or a small `.tpatch/substrate` artifact;
- explicit command availability matrix;
- early preflight errors for Git-required commands outside Git;
- docs updates for substrate requirements;
- tests for non-Git init, explicit Git init, metadata-only mode, and Git-required
  command errors.

Non-goals:

- no tpatch-native snapshots;
- no pluggable substrate interface;
- no ADR;
- no behavior change for existing initialized Git repos.

### 7.2 Possible ADR, not now

`ADR-0XX-substrate-boundary.md`

Only needed if the implementation PRD wants a durable `Substrate` interface or
if future work proposes a second real change-tracking substrate. The ADR should
explicitly preserve or supersede ADR-015's D4/D6/D7 constraints.

### 7.3 Future whitepaper, only if reopened

`WP-0XX-tpatch-native-change-tracking.md`

Only justified if a real user segment needs non-Git patch maintenance strongly
enough to justify re-evaluating tpatch's lane. It should compare Git, jj-on-Git,
Quilt-like patch files, and true tpatch-native snapshots with case studies.

## 8. Open questions *(V64)*

1. Should default `tpatch init` outside Git refuse, prompt, or create
   metadata-only with a warning? The safe automation default is refuse.
2. Is a nested config field (`substrate.type`) worth a parser upgrade, or should
   v1 use a flat key (`substrate_type`) matching current config style?
3. Should metadata-only mode allow `implement` / `apply --mode prepare`, or only
   planning phases?
4. What command, if any, converts an existing metadata-only workspace after the
   user later runs `git init`? Candidate: `tpatch substrate adopt-git`.
5. Should `tpatch init --git=init` also create an initial commit containing
   `.tpatch/`, or only run `git init` and leave commits to the user? V64
   recommends only `git init`.
6. Should `upstream.lock` be omitted, marked inactive, or written empty in
   metadata-only mode? Current init always writes it (`internal/store/store.go:107-117`).

## 9. References *(V64)*

| Claim | Citation |
|---|---|
| tpatch purpose is local-first customization of upstream OSS projects with reapply/review/reconcile structure. | `SPEC.md:5-8` |
| Core lifecycle and commands include Git-shaped record/reconcile/upstream/replay concepts. | `SPEC.md:19-78` |
| `.tpatch/` includes `upstream.lock`, feature patch artifacts, and reconciliation artifacts. | `SPEC.md:79-111` |
| `post-apply.patch` is canonical replay input and replays with `git apply`. | `docs/feature-layout.md:34-44` |
| `status.json:apply.base_commit` and land trailers bind feature state to Git commits. | `docs/feature-layout.md:84-94`, `docs/land.md:77-97` |
| `record` captures Git diffs and warns against clean-working-tree after commit ordering mistakes. | `docs/record.md:1-21`, `docs/record.md:39-53` |
| `land` creates ordinary Git commits and trailers. | `docs/land.md:1-6`, `docs/land.md:66-97`, `docs/land.md:148-159` |
| `reconcile` uses Git apply/worktree semantics and assumes target upstream state. | `docs/reconcile.md:5-18`, `docs/reconcile.md:70-83` |
| WP-001 narrowed the gap to record UX, land projection, and modest audit/recovery rather than a new data model. | `docs/whitepapers/WP-001-feature-slice-gap.md:32-69` |
| WP-002 says patch generations are append-only audit metadata, not lifecycle state, and `post-apply.patch` remains replay authority. | `docs/whitepapers/WP-002-capture-and-metadata-foundation.md:70-75`, `:139-163` |
| WP-003 reconcile evidence stores upstream commit refs and blocks broad structural/search work pending more studies. | `docs/whitepapers/WP-003-reconcile-safety-and-middle-pass.md:54-65`, `:80-91`, `:104-112` |
| Competitive landscape says tpatch's lane is patch-as-artifact in `.tpatch/` with Git / optionally-jj substrate, not patch theory. | `docs/market-research/competitive-landscape.md:52-63`, `:129-151` |
| ADR-015 maps stable/moving identity to slug/patch-SHA and rejects StGit-style patch-as-commit, jj working-copy-as-commit, and distributed obsolescence for now. | `docs/adrs/ADR-015-prior-art-identity-mapping.md:91-119`, `:183-205`, `:234-264` |
| Current `init` calls store init and installs skills with no Git check. | `internal/cli/cobra.go:87-124`, `internal/store/store.go:42-130` |
| Store root detection is `.tpatch/`-based, not Git-root-based. | `internal/store/store.go:23-40`, `internal/cli/cobra.go:2768-2779` |
| Current config has no substrate field and parser accepts a fixed set of flat keys. | `internal/store/types.go:315-374`, `internal/cli/cobra.go:2688-2721` |
| Git utilities use `git rev-parse`, `git status`, `git diff`, `git apply`, `git worktree`, and related Git plumbing. | `internal/gitutil/gitutil.go:13-20`, `:126-187`, `:214-338`, `:371-436`, `:489-555` |
