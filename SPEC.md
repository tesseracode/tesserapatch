# Tessera Patch — Unified Implementation

## SPEC.md

### 1. Purpose

Tessera Patch is a local-first CLI and agent skill framework for customizing upstream open-source projects with natural-language-driven changes while preserving enough structure to reapply, review, and reconcile those changes when upstream evolves.

This is the **unified implementation**, consolidating the best ideas from three independent prototypes (cc/, experimental/, gpt/).

### 2. Architecture

**Language**: Go 1.22+  
**Distribution**: Single binary `tpatch` with embedded assets via `go embed`  
**Dependencies**: Zero external (stdlib only)  
**Agent integration**: Claude skill, VS Code Copilot skill, Copilot prompt companion, Cursor rules, Windsurf rules, generic markdown workflow  
**Provider interface**: OpenAI-compatible HTTP adapter, with heuristic offline fallback

### 3. Core Workflow — The 7-Phase Lifecycle

```
analyse → define → explore → implement → test → record → reconcile
```

Every tracked feature moves through this state machine:

```
requested → analyzed → defined → implementing → applied → active
                                                              ↓
                                                        reconciling → active (new version)
                                                              ↓        or upstream_merged
                                                           blocked      or blocked

applied | active | reconciling | reconciling-shadow
                         ↓ tpatch feature unapply
                     unapplied
                         ↓ tpatch apply
                       applied
```

#### Feature States

| State | Meaning |
|-------|---------|
| `requested` | User submitted feature request |
| `analyzed` | Analysis complete, compatibility assessed |
| `defined` | Acceptance criteria and plan ready |
| `implementing` | Changes being applied |
| `applied` | Changes applied, verification pending or passed |
| `active` | Feature live in fork, being maintained |
| `reconciling` | Upstream updated; re-evaluating patches |
| `blocked` | Failed; needs manual intervention |
| `upstream_merged` | Feature adopted upstream; local patch retired |
| `rejected` | Terminal, pre-implementation decision: this feature should never be implemented. Reachable only from `requested`/`analyzed`/`defined` via `tpatch reject`; left only via `tpatch reopen`. |
| `unapplied` | Tracked post-implementation feature whose canonical patch and audit history remain intact, but whose patch is absent from the current working tree. Reapply with `tpatch apply <slug>`. |

Rejection is deliberately disjoint from the neighbouring terminal
concepts: `upstream_merged` asserts an implementation exists upstream,
`blocked` is temporary (work resumes once the blocker clears), and
`remove` deletes the feature directory outright. `rejected` preserves the
complete feature directory and an append-only audit history.

`tpatch reject` is refused (exit 3) from `implementing`, `applied`,
`active`, `reconciling`, `reconciling-shadow`, `blocked`,
`upstream_merged`, and `unapplied`. Post-implementation retirement is out
of scope; see `docs/adrs/ADR-031-rejected-feature-state-data-model.md` D6.

### 4. CLI Commands

#### Phase 1 (MVP)

| Command | Purpose |
|---------|---------|
| `tpatch init [--path]` | Create `.tpatch/` workspace + install all skill formats |
| `tpatch add <description> [--path]` | Create a feature request with stable slug |
| `tpatch status [--feature] [--json] [--verbose] [--path]` | Feature status dashboard |
| `tpatch analyze <slug> [--path] [--timeout]` | Run analysis phase |
| `tpatch define <slug> [--path]` | Generate acceptance criteria + implementation plan |
| `tpatch explore <slug> [--path]` | Read codebase, find minimal changeset |
| `tpatch implement <slug> [--path]` | Generate deterministic apply recipe |
| `tpatch apply <slug> [--mode auto\|prepare\|started\|execute\|done] [--dry-run] [--path]` | Execute recipe or record session |
| `tpatch prepare <slug> [mode and report flags] [--path]` | Complete, adopt, regenerate, inspect, or abandon an intent-bundle transaction; unrelated to `apply --mode prepare` |
| `tpatch feature intent-archive list <slug> [--json] [--quiet] [--path]` | Inspect retained prior intent bytes and archive repair state |
| `tpatch feature intent-archive purge <slug> <selector> [--yes] [--json] [--quiet] [--path]` | Preview or perform bounded archive retention cleanup |
| `tpatch record <slug> [--path]` | Capture patches (tracked + untracked files) |
| `tpatch reconcile [--upstream-ref] [slug...] [--path]` | Reconcile features against upstream |
| `tpatch reconcile audit-retirement <slug> [--json] [--path]` | Read-only audit of retired feature dependency metadata |
| `tpatch reconcile confirm-upstreamed <slug> [--json\|--format json] [--path]` | Confirm an upstreamed reconcile outcome and auto-run retirement cleanup audit |
| `tpatch provider check [--path]` | Validate provider endpoint |
| `tpatch config show\|set [--path]` | Manage configuration |

#### Prepare intent bundle (GH #16 and GH #23)

`prepare` is optional methodology, not a lifecycle phase or precondition. It has
five modes:

```text
tpatch prepare <slug> --check       [--json] [--quiet] [--path <dir>]
tpatch prepare <slug>               [--json] [--quiet] [--path <dir>] [--timeout <d>] [--timeout-phase <d>] [--no-retry] [--dry-run] [--allow-heuristic]
tpatch prepare <slug> --manual      [--json] [--quiet] [--path <dir>] [--dry-run]
tpatch prepare <slug> --regenerate  [--json] [--quiet] [--path <dir>] [--timeout <d>] [--timeout-phase <d>] [--no-retry] [--dry-run] [--allow-heuristic]
tpatch prepare <slug> --abandon-transaction [--json] [--quiet] [--path <dir>] [--yes]
```

- **generate** (no mode flag) creates only a dependency-coherent missing suffix
  of `analysis.md → spec.md → exploration.md`; existing artifact bytes and a
  present analysis sidecar are preserved. A configured provider is preferred,
  with disclosed heuristic fallback when it is absent or fails.
- **manual** adopts a complete hand-authored three-document bundle. It calls no
  provider and writes no artifact bytes; its structural gate is intentionally
  stricter than the presence-only per-phase `--manual` gates.
- **regenerate** replaces the complete bundle and archives safely readable prior
  bytes. It requires a configured successful provider unless
  `--allow-heuristic` is explicitly passed.
- **abandon** previews or, with `--yes`, moves interrupted local transaction
  evidence aside. It changes no canonical feature file and performs no repair.
- **check** is the unchanged read-only inspector. It reports structural presence
  for the three Markdown artifacts plus optional `artifacts/analysis.json`,
  never advances state, writes nothing, and keeps `provenance: unknown`.

Exactly twelve local flags are registered: `--check`, `--manual`,
`--regenerate`, `--abandon-transaction`, `--allow-heuristic`, `--dry-run`,
`--yes`, `--json`, `--quiet`, `--timeout`, `--timeout-phase`, and `--no-retry`;
`--path` is inherited. The four mode flags are mutually exclusive by flag
presence, so even `--check --abandon-transaction=false` is a parse error.
Explicit `--abandon-transaction=false` alone selects generate. Timeout,
timeout-phase, and no-retry are legal only for generate/regenerate;
allow-heuristic is legal only there (and is a disclosed no-op in generate);
dry-run is legal only for generate/manual/regenerate. `--yes` is valid only
with a true abandon request; every other prepare use exits 1 with
`prepare: --yes is only valid with --abandon-transaction`.

`--dry-run` performs bounded inspection and plan checks only. It makes no
provider call, runs no Git process, takes no workspace authority, and writes
nothing. Its report says `execution_preflight: "not_evaluated"` because the
real mutation can still refuse on platform, filesystem, Git, lock, generation,
redaction, recovery, or publication grounds.

Mutating prepare is supported only on Linux and macOS, on a non-denied local
workspace-root filesystem after a real root-directory flock succeeds. The
authority is per workspace, not per feature, and is host-local; it provides no
cross-machine exclusion. Rooted writes prevent escape from the held workspace,
but do not promise that every in-root alias or final syscall race is detectable.
`--check` retains its separate read-only platform contract. Reports contain
repo-relative identifiers and recovery commands, not absolute paths, provider
transcripts, artifact content, secret values, timestamps, durations, hostnames,
PIDs, or symlink targets. Human mode sends progress to stderr; `--json` emits
one JSON document; `--quiet` emits only the summary form.

The publication set is the planned canonical intent artifacts, archive index,
and `status.json`, with status last. Readers can observe an in-progress prefix;
the command claims final verification and bounded crash recovery, not
instantaneous multi-file visibility. Root `FEATURES.md` is a derived,
best-effort refresh outside the publication set and can temporarily lag
`status.json`. Journal recovery is terminal: the command reports `recovered`
and the sanitized rerun instead of continuing. Divergent journal/publication
evidence exits 6 and names abandon or a repo-relative manual route; archive
purge divergence names its separate archive repair. Confirmed purge also treats
pending-purge recovery as terminal: it finalizes the pending state, reports
`recovered`, and stops without processing the requested selector. The operator
then reruns the reported selector command.

| Code | Meaning |
|------|---------|
| `0` | Success: published, adopted, no-op, admissible dry-run/preview, terminal recovery, purge completion, or check-ready. |
| `1` | Parse/usage error or unexpected internal failure; parse failures emit no report and write nothing. |
| `2` | Bundle not ready or coherent, or staged structural validation failed; no mutation. |
| `3` | Cannot act because of input, state, policy, platform/authority, archive integrity, contention, or pending recovery; no mutation. |
| `4` | Retired and unpopulated. `prepare` never exits 4. |
| `5` | Publication aborted and rolled back, or an archive purge partially advanced with deterministic retry evidence. |
| `6` | Manual intervention required; evidence is preserved and the applicable terminal recovery route is reported. |

The optional check keeps its accepted byte contract: exit 0 is ready, exit 2 is
not ready, and exit 3 is indeterminate. Exit 2 is a report result, not a
workflow or system failure.

Archive retention is managed separately:

```text
tpatch feature intent-archive list  <slug> [--json] [--quiet] [--path <dir>]
tpatch feature intent-archive purge <slug> (--blob <hash>... | --generation <id>... | --orphans | --all) [--yes] [--json] [--quiet] [--path <dir>]
```

`list` is read-only. `purge` requires exactly one selector, previews without
`--yes`, and only the confirmed form takes the workspace authority. Neither
verb runs Git. A missing or repeated scope family, and a `--blob` or
`--generation` value that is not a full lowercase SHA-256, are usage errors:
each exits 1 before any archive is read, prints no report, carries no refusal
code and never echoes the rejected value. A well-formed `--blob` hash the
archive indexes no reference to, or a well-formed `--generation` id it records
no generation for, is refused as `archive-selector-invalid` at exit 3, never as
archive corruption; that refusal names which selector matched nothing and
routes to `intent-archive list` from the workspace root, while
`archive-index-*` codes stay reserved for the archive's own strict decode and
storage observation. Neither path writes anything. Retained blobs provide exact
byte recovery only while present; the archive is not canonical lifecycle truth,
semantic certification, authorship/provenance, or a
general history/undo facility. Purging publishes tombstones and can remove
working-tree bytes, but deleting committed blobs does
not rewrite Git history.

#### Feature rejection (v0.13.0, GH #6)

| Command | Purpose |
|---------|---------|
| `tpatch status [--include-rejected] ...` | Rejected features are excluded from the default listing; `--include-rejected` opts them back in. The `--feature <slug>` detail view always renders the full rejection record. |
| `tpatch reject <slug> --reason <code> --note <string> --evidence <path>... [--actor <string>] [--related <ref>] [--json]` | Mark a feature permanently rejected (terminal, pre-implementation). `--reason` is a closed enum: `not-a-bug`, `premise-disproved`, `obsolete`, `out-of-scope`, `unsafe`, `duplicate`, `superseded`. `--note` is required and non-empty; at least one `--evidence` path is required and is content-hashed (SHA-256, lowercase hex) at write time. |
| `tpatch reopen <slug> --note <string> [--evidence <path>...] [--actor <string>] [--json]` | Reopen a rejected feature (`rejected → requested`). Append-only: the prior rejection record is never deleted. `--note` is required; `--evidence` is optional. Every historical evidence reference is re-verified against its recorded hash on every reopen; divergence is recorded, never blocking. |

#### Feature unapply (v0.14.0)

| Command | Purpose |
|---------|---------|
| `tpatch feature unapply <slug> [--dry-run] [--allow-soft-dependents] [--actor <string>] [--mode patch]` | Strictly reverse-apply a feature's canonical `artifacts/post-apply.patch` from a clean working tree while preserving the feature directory, canonical patch, patch generations, and audit history. |

Successful unapply writes
`artifacts/unapply/<attempt-id>/{unapply-session.json,reverse.patch}`,
clears the Verify freshness record, and records `state: "unapplied"`.
`unapply-session.json` is a fixed version-1 audit envelope; it is not a
`FeatureStatus` sub-record. V1 is patch mode only.

Hard and `supersedes` dependents refuse unapply. Soft dependents refuse by
default and require `--allow-soft-dependents`. Dependency edge creation
onto an unapplied parent remains legal, but `unapplied` does not satisfy a
hard dependency apply gate. Aggregate reconcile skips unapplied features;
explicit reconcile reports forward-apply viability without changing their
lifecycle state.

The command runs strict reverse-check and temporary-worktree preview before
mutation, snapshots every touched path, and restores source files plus removes
partial audit artifacts if reverse apply, artifact writes, or the atomic
status update fail. `--dry-run` reports every blocker and planned artifact
without mutation.

`tpatch reject <slug>` and `tpatch reconcile --reject <slug>` are
**intentionally unrelated surfaces** and are not to be confused. The
former is a top-level command performing a terminal lifecycle transition
on the *feature*; the latter is a flag on `reconcile` that prunes a
*shadow worktree* (a transient, reversible action on a resource). Their
state preconditions never overlap: `tpatch reject` is refused from every
state in which a shadow worktree can exist. See
`docs/prds/PRD-rejected-feature-state.md` §4.1 and
`docs/adrs/ADR-031-rejected-feature-state-data-model.md` D10.

`tpatch reconcile confirm-upstreamed` refuses (exit 3) on `rejected` and
`unapplied` source features, guarded before any reconcile-revision append.

##### Exit-code envelope

Exit codes are **per-command contracts**, not a single global enum
across all `tpatch` subcommands (`tpatch verify` has its own, unrelated
exit-2 meaning). For `reject`, `reopen`, the dependency-editing commands'
rejected-parent guard, and `reconcile confirm-upstreamed`'s rejected-source
guard, the codes are:

| Code | Meaning | Example triggers |
|------|---------|------------------|
| `0` | Success | The command completed and wrote `status.json`. Includes a reopen that recorded non-blocking evidence divergence. |
| `1` | Unexpected internal error | Filesystem I/O failure, store load failure unrelated to the slug itself. |
| `2` | Validation error (pre-mutation input validation) | Invalid `--reason`, missing/empty `--note`, missing `--evidence`, an evidence path that is absolute, `..`-escaping, missing, non-regular, symlink-escaping or unreadable. |
| `3` | State-transition error (post-validation state-machine refusal) | Rejecting from a non-eligible source state; rejecting a feature with live dependents; rejecting an already-rejected feature; reopening a non-rejected feature; creating a `hard`/`soft`/`supersedes` edge onto a rejected parent; `reconcile confirm-upstreamed` / `apply` / `reconcile` on a rejected feature. |

#### Typed feature resources (v0.15.0)

| Command | Purpose |
|---------|---------|
| `tpatch feature resource add <slug> --kind <kind> --selector <sel> [--adapter <a>] [--capability <c>] [--arg k=v ...] [--trust-current-dolt] [--json]` | Declare one typed resource. Kinds are a closed set: `ignored-file`, `git-metadata`, `adapter-snapshot`. |
| `tpatch feature resource list <slug> [--json]` | List declared resources and each one's current capture state. Never acquires the per-slug lock. |
| `tpatch feature resource remove <slug> <resource-id-or-prefix> [--json]` | Remove one declaration. Never touches `current.json` or any batch file. |
| `tpatch feature resource clear <slug> [--json]` | Remove all declarations; the file is kept with `resources: []`. |
| `tpatch feature resource trust-dolt <slug> <resource-id-or-prefix> --binary-sha256 <64hex> [--json]` | Re-pin an already-declared Dolt resource's trusted binary digest without changing its `resource_id`, `current.json` entry or capture history. |
| `tpatch feature resource capture <slug> [--resource <id>] [--dry-run] [--json]` | The only verb that executes an adapter, reads ignored-file content or writes tracked capture state. |
| `tpatch feature resource diff <slug> [--resource <id>] [--json]` | Recompute each resource's structural result and compare it against the last tracked batch. Read-only; never executes an adapter. |
| `tpatch record <slug> [existing flags] --resources [--json]` | Stage resources in memory, run the existing Git-side capture unchanged, and publish the resource domain only if Git succeeded. |

Resources are **audit sidecars**, never canonical patch or lifecycle
truth. Two tracked artifacts exist per feature:

- `artifacts/resources.json` — the declaration manifest, written only by
  `add`/`remove`/`clear`/`trust-dolt`. Each entry carries
  `resource_id`, `kind`, `selector`, `adapter`, `capability`, a sorted
  `args` array, a mutable `trust` pin, and `added_by_tool_version`.
- `artifacts/resource-captures/` — an unordered, content-addressed set of
  immutable `batches/<batch_id>.json` files plus one atomically-rewritten
  `current.json` pointer, written only by
  `capture`/`record --resources`.

`resource_id` is `res_` plus the first 12 hex characters of SHA-256 over
`feature\0kind\0selector\0adapter\0capability\0canonical_args`; the
trust pin is deliberately excluded, so a Dolt upgrade re-pinned via
`trust-dolt` preserves identity and history. `batch_id` is `rb_` plus the
**full** SHA-256 of the canonical `{feature, results}` body. An
invocation that reproduces already-published content writes zero new
batch bytes and only rewrites the pointer.

No tracked resource artifact ever contains raw file bytes, raw adapter
stdout, or a wall-clock timestamp. Ignored-file content and adapter
output are read into bounded in-process buffers, scanned by
`internal/redact` against six closed content classes, hashed, and
discarded; any match hard-refuses the whole invocation. Every path a
resource touches passes an ancestor-symlink walk plus an `O_NOFOLLOW`
open and an `os.SameFile` descriptor-identity check.

The Dolt adapter is the only external adapter in v1. It never runs
`dolt version`, requires an operator-approved `trust.binary_sha256` pin,
executes a hash-verified private copy under ephemeral scratch rather
than the resolved pathname, and runs with a fresh minimal environment
(`HOME`/`DOLT_ROOT_PATH` only) inside an isolated scratch home.

Every mutating verb runs the `.tpatch/local/` ignore + untracked gate,
then takes a nonblocking per-slug `flock`. Resource capture is supported
on **Linux and macOS only**; every other target refuses
`resource-lock-unsupported` without touching the filesystem.

##### Exit-code envelope

| Code | Meaning | Example named refusals |
|------|---------|------------------------|
| `0` | Success, including `diff` reporting "no capture yet" | — |
| `1` | Internal/host fault or data-integrity condition | `tracked-batch-missing`, `adapter-copy-failed`, `adapter-process-observer-failed`, `adapter-group-signal-failed`, `adapter-reap-timeout`, `adapter-output-read-failed`, `no-resources-declared`, `resource-domain-incomplete` |
| `2` | Validation error | `dolt-argument-refused`, `dolt-trust-flag-required`, `adapter-missing-at-add`, `dolt-contract-unsupported`, `resource-not-dolt-adapter` |
| `3` | State/policy refusal | `not-ignored`, `tracked-and-ignored`, `symlink-component-refused`, `path-outside-repo`, `path-replaced-during-open`, `redaction-refused`, `resource-limit-exceeded`, `adapter-missing`, `adapter-binary-untrusted`, `dolt-trust-required`, `adapter-copy-noexec`, `db-path-identity-changed`, `dolt-query-error`, `dolt-json-parse-error`, `capture-in-progress`, `resource-lock-unsupported`, `resource-lock-filesystem-unsupported`, `batch-id-collision`, `batch-file-corrupt`, `resources-file-corrupt`, `resource-id-collision`, `index-entry-missing`, `adapter-drain-timeout` |

See `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md` and
`docs/adrs/ADR-033-resource-capture-boundary.md`.

#### Landed-feature verification (v0.15.1, GH #8)

`tpatch verify` reads the four-trailer block `tpatch land` emits and
switches to a **dual-anchor** model when a feature is landed:

- **Anchor H (historical)** — the closure-replay shadow is rooted at the
  *replay anchor's* single parent tree, not at `HEAD`. Candidates are
  collected from one `git log --topo-order --reverse -z` enumeration,
  qualified by a forward `git apply --check --cached -C1` at `C^`,
  compared by a normalized zero-context change identity when more than
  one qualifies, then selected deterministically. V7 replays the recipe
  there; the shadow is reset to the closure baseline before V8's forward
  check.
- **Anchor C (current)** — an index-isolated assertion at `HEAD` through
  a temporary index seeded by `git read-tree`. It reads neither the
  working tree nor the real index. A match that survives only with all
  context discarded blocks rather than certifying.

Ten closed landing-evidence states are reported (`none`, `exact`,
`duplicate-equivalent`, `stale`, `ambiguous`, `malformed`,
`unsupported-topology`, `shallow-history`, `history-incomplete`,
`unavailable`); only `none` keeps the previous forward-mode behaviour.
`Tpatch-Base-Commit` length is derived from
`git rev-parse --show-object-format`, and `tpatch land` refuses to emit a
base commit that is empty, ill-formed or unresolvable.

Verify requires **git ≥ 2.36** and carries `GIT_NO_LAZY_FETCH=1` on every
object and materialization command, so it never reaches the network.
The `--json` report is `schema_version` **1.1** — an additive superset of
1.0 adding `repository`, `baseline`, `landing_evidence`, `target_mode`,
`advisories` and `checks[].mode`.

See `docs/prds/PRD-verify-freshness.md` §3.6,
`docs/prds/PRD-tpatch-land.md` §3.8 and `docs/adrs/ADR-013-verify-freshness-overlay.md`
Amendment 1 (D8–D19).

#### Phase 2 (Post-MVP)

| Command | Purpose |
|---------|---------|
| `tpatch upstream check [--path]` | Check for new upstream versions |
| `tpatch cycle <slug> [--path]` | Run full lifecycle in one command |
| `tpatch test <slug> [--path]` | Run project tests, verify criteria |
| `tpatch replay <slug> [--path]` | Replay patch on fresh clone for verification |

### 5. `.tpatch/` Data Model

```
.tpatch/
├── config.yaml
├── FEATURES.md
├── upstream.lock
├── steering/
│   ├── local.md
│   └── upstream.md
├── workflows/
│   └── tessera-patch-generic.md
└── features/
    └── <slug>/
        ├── status.json
        ├── request.md
        ├── analysis.md
        ├── spec.md
        ├── exploration.md
        ├── record.md
        ├── reconciliation/
        │   └── <commit-range>.md
        └── artifacts/
            ├── apply-recipe.json
            ├── recipe-provenance.json
            ├── recipe-stale.json
            ├── recipe-capture-event.json
            ├── recipe-coverage.json
            ├── apply-session.json
            ├── analysis.json
            ├── pre-apply.patch
            ├── post-apply.patch
            ├── reconcile.patch
            ├── reconcile-session.json
            ├── reconcile.md
            ├── manual-validation.md
            ├── resources.json
            └── resource-captures/
                ├── batches/
                │   └── <batch_id>.json
                └── current.json
```

Ephemeral, gitignored control state for resource capture lives outside
the tracked tree, under `.tpatch/local/resource-scratch/<slug>/`: a
persistent zero-length `.lock` file plus one `es_<12hex>/` directory per
in-flight invocation. Nothing captured is ever written there.

#### Recipe generation authority (GH #15; v0.17 planned, unreleased)

`artifacts/post-apply.patch` remains the canonical feature diff.
`apply-recipe.json` is an executable plan, not a replacement for that diff.
Recipe coverage is necessary, not sufficient, for future replay eligibility; it is not cross-base safety.
A warn/exit0 coverage row is not eligibility and never grants replay permission.
GH #13's new reconcile operation-replay consumer belongs to a future,
separate release; GH #15 neither implements it nor persists replay anchors.
Landing trailers, verification attestation and dependency gates remain
independent. No recipe-schema or landing-trailer change is introduced.

The contract is ADR-036 rev-7 **as amended by ADR-039, ADR-040, ADR-041
and ADR-042**, not the superseded planning wording in isolation.

##### Seven governed producers

Every successful bound write, successful manual recipe checkpoint, or
explicitly contracted same-patch checkpoint owes fresh publication. An
invocation alone is not an event.

| ID | Command/event | Stored `producer` | `capture.mode` and behavior |
|---|---|---|---|
| P1 | `record`; the embedded record step of `land` | `record` | Selected record mode: `working-tree-all`, `staged-index`, `unstaged-worktree`, `committed-range`, `auto-committed-range`, or `explicit-committed-range`. Derives a recipe when authorized and fully representable. `land` adds no producer or second publication; `land --no-record` skips P1. |
| P2 | `feature patch refresh <slug>`; `feature patch fixup <slug> --reason <reason>` | `feature-patch-amend` | `working-tree-all`. Patch writes use non-regenerating autogen. A non-empty capture matching the latest generation still republishes evidence/coverage, without patch, recipe, provenance, generation or state writes. |
| P3 | `reconcile --accept <slug>`; auto-accept via `reconcile --resolve --apply <slug>` | `reconcile-accept` | `reconcile`. Refresh writes the canonical patch against the accepted upstream commit and preserves the recipe. Publication is owed even for identical or empty refreshed patch bytes, whether or not a generation is appended. |
| P4 | `cycle`'s non-empty patch-capture step | `cycle` | `working-tree-all`. No recipe regeneration here. The earlier implement step is P6; `--skip-execute` or declining subsequent prompts leaves that P6 publication, not an unfulfilled P4 event. |
| P5 | `apply --mode done`'s non-empty patch write | `apply-done` | `working-tree-all`. Preserves the recipe. The state-selected canonical-patch reapply branch writes no bound artifact and owes no P5 publication. |
| P6 | `implement` provider/heuristic recipe writes; `implement --manual` checkpoint | `implement` | `no-capture`, empty selectors, unavailable reference. Publishes incomplete coverage, including when no canonical patch exists. A successful raw undecodable response write also binds its exact bytes and reports `recipe-undecodable`. |
| P7 | `edit <slug> artifacts/apply-recipe.json` or `edit <slug> artifacts/post-apply.patch` when the editor changes bytes | `artifact-edit` | `no-capture`. A durable prior reference is carried only after reconstruction and validation against the frozen prior evidence and unchanged patch; otherwise publication is explicitly incomplete. |

P2 empty capture is **not** its same-patch checkpoint. P4/P5 empty capture,
failed bound writes, failed manual validation, non-bound edits and unchanged
editor results are not events. P7 uses the resolved canonical artifact path:
a same-named feature-root decoy is not bound. A saved change still owes
publication if the editor returns an error. An editor that returns before a
later save cannot cover that save; out-of-band changes are detected at the
next binding read instead.

##### Publication and evidence

All seven producers use one shared publication API. The bound-artifact
publication order is recipe (if authorized), provenance (if justified),
generation (if owed), **E → C**. P6 finalizes after its state-mark attempt;
outer orchestration, such as accept's later state update, is not part of an
atomic publication set:

- **E**, `artifacts/recipe-capture-event.json`, is independently constructed
  from the immutable observation, exact final patch/recipe inputs, event
  facts and effective parent-created-path exclusions. It binds the exact raw
  C bytes through `coverage_sha256`.
- **C**, `artifacts/recipe-coverage.json`, binds the requested feature,
  readable patch/recipe presence and raw-byte SHA-256s, strict recipe
  decodability, reference and capture descriptors, ordered effects,
  operation assignments, status and exact reasons.

E is atomically replaced **before** the final atomic replacement of C.
Atomicity is per file, **not a cross-file transaction**. Earlier writes may
remain after failure; publication errors propagate as non-zero command
failures, never ordinary success. A retry must re-observe the inputs.
Identical events need not append a patch generation.

E is unkeyed consistency evidence, not authentication, authorship or history.
It is neither a journal nor an append-only event sequence. Coordinated
consistent edits or rollback of the whole local set are not authenticated
or detected as historical rollback. Readers independently load E, C and
bound bytes, reconstruct locally available trees/content and recompute the
proof; labels, stored hashes and JSON decoding alone are not proof.
Originally unobserved sides cannot acquire authority from a later worktree
read. Neither artifact stores source bodies, secrets, timestamps or a
persisted cross-base replay anchor.

**Genuine C absence is uniformly legacy, even if E exists** (including an
orphaned E after interruption). E does not promote missing coverage into
authority or a new legacy failure. With C present, absent, unreadable,
malformed or unpaired E is a binding failure.

##### Complete coverage, origin and reasons

C has `schema_version: 1`, `coverage_status: complete|incomplete`, and
`cross_base_status` as follows:

| Coverage | `cross_base_status` |
|---|---|
| Incomplete | `unsupported` |
| Complete, at least one existing-file write | `consumer-derivation-required` |
| Complete, exclusively explicit-empty-gated creations | `reference-tree-only` |

Under ADR-039, the complete-operation domain is **only `write-file` with a
present, non-null `preimage_hash`**, including `""` as an explicit creation
gate. This is necessary, not sufficient. All ten ADR-036 predicates apply:

1. Readable canonical patch, completely parsed, with at least one effect.
2. Durable `reference.kind: commit`.
3. Readable, strictly decodable recipe owned by the coverage feature.
4. Every normalized patch effect represented exactly once, with no extras.
5. Every recipe operation assigned, with no surplus operations.
6. Repository-safe effect/operation paths; no two operations claim one path.
7. Every effect `represented`, with empty effect and record reason arrays.
8. Required sides observed, with exact modes and hashes for present sides.
9. Full simulation reproduces the observed bytes, existence and supported
   modes, with no unmodeled path or mode changes.
10. Reclassification of that result makes every operation already-present
    without writing a byte.

Append, replacement (even exact-postimage replacement), and ungated writes
are not complete v1 operations: assigned effects carry
`operation-not-reclassifiable`. Their existing explicit execution semantics
are preserved. GH #24 owns any future widening, not this implementation.
Generation with unsupported effects withholds the entire new recipe rather
than emitting a partial one; an existing manual/provider recipe is preserved.
Supported derivation covers regular text additions/modifications, not
deletion, rename, copy, binary, executable, symlink, gitlink or mode-only
effects, unsafe paths, missing observations or excluded parent-created targets.

**D16 is total equality with freshly derived canonical recipe bytes.**
File-set equality, semantic equivalence, a producer label or historical
provenance cannot establish it. Even formatting/key-order differences fail
the comparison. Without explicit successful complete regeneration, differing
manual/provider bytes and their provenance are preserved. With D16 proof,
record can create/repair `recipe-provenance.json` using this run's actual
base and time, including a byte-no-op rerun; already-valid provenance is
preserved. No historical origin is reconstructed.

Successful publication prints `recipe coverage: complete` or
`recipe coverage: incomplete (<sorted, deduplicated reasons>)` on stderr.
The stored record-level `reasons` and per-effect `reason_codes` are exact:

| Scope | Reason codes |
|---|---|
| Canonical patch | `canonical-patch-missing`, `canonical-patch-empty`, `canonical-patch-unparseable` |
| Recipe/reference and event | `recipe-undecodable`, `recipe-owner-mismatch`, `reference-not-durable`, `recipe-stale-marker-present`, `manual-bound-artifact-edit`, `producer-patch-rewrite`, `recipe-not-regenerated` |
| Assignment/simulation, record-level | `operation-surplus`, `simulation-mismatch` |
| Effect capability | `effect-delete-unsupported`, `effect-rename-unsupported`, `effect-copy-unsupported`, `effect-binary-unsupported`, `effect-executable-unsupported`, `effect-mode-only-unsupported`, `effect-symlink-unsupported`, `effect-gitlink-unsupported` |
| Effect safety/availability/assignment | `path-unsafe`, `preimage-unavailable`, `postimage-unavailable`, `parent-created-target-unsupported`, `operation-not-reclassifiable`, `operation-missing` |

`operation-missing` is owed only for an otherwise representable effect,
never for an intentionally excluded effect. Effect dispositions are
`represented`, `mismatch` (the singleton `operation-missing`), `ambiguous`
(unavailable observations), or `unsupported`; availability takes precedence
over capability exclusions. `contextual_hint` is advisory, never permission.

ADR-040 keeps explanation separate from origin: the paired
`producer-patch-rewrite` / `recipe-not-regenerated` reasons apply only when
a patch rewrite leaves a preserved, non-regenerated recipe unable to cover,
simulate and reclassify the patch exactly. A P2 formatting-only D16 mismatch
still preserves the recipe and stale marker, yielding incomplete coverage
with `recipe-stale-marker-present`, **not invented rewrite reasons**.
P2's coverage-only checkpoint adds no rewrite reasons.

##### Verify and doctor

Verify adds `recipe_generation_coverage` without changing report schema 1.1.
The first matching D13 rung wins:

| Rung | Condition | Row / report implication |
|---|---|---|
| 1 | C unreadable, malformed or unknown-schema/field | Failed `block`: `recipe-coverage-malformed`; verify exit 2. |
| 2 | Owner, readable-presence, raw hash, E pairing or reconstructed reference/proof mismatch | Failed `block`; verify exit 2. Codes: `recipe-coverage-owner-mismatch`, `recipe-coverage-patch-changed`, `recipe-coverage-recipe-changed`, `recipe-coverage-capture-evidence-invalid`, `recipe-coverage-reference-stale`. |
| 3 | Valid, bound incomplete C | Failed `warn`: `recipe-coverage-incomplete`, all sorted reasons and affected paths. |
| 4 | Valid complete C, current `recipe-stale.json` present | Failed `warn`: `recipe-coverage-stale-marker`. |
| 5 | C genuinely absent, regardless of E or old stale marker | Failed `warn`: `recipe-coverage-missing`. |
| 6 | Valid complete C, no current stale marker | Passed `block` row; no failure contribution. |

Warning rows leave verify passed/exit 0 **only absent other failures**.
Missing-coverage legacy features, including pre-v0.17 stale-marker cohorts,
remain verify-green under that condition. Coverage-envelope owner mismatch
is a binding failure; recipe-owner mismatch is instead a stored incomplete
reason reported through rung 3. Surface diagnostic codes are not schema
reasons and must not be inserted into C.

`tpatch doctor --check D10` is read-only, warning-only and never fixes
artifacts, **even with `--fix`**. Missing C is reported there only when both
patch and recipe are readable; malformed/stale/incomplete C is diagnosed.
A regeneration command is offered only after a read-only plan of the
actual default `tpatch record <slug> --regenerate-recipe` establishes a
complete, D16-proven publication and passes its capture, state, collision,
generation, round-trip and publication-path gates. The plan does not guess
a committed range or offer bypass flags. Otherwise the diagnostic is
`recipe-generation-no-truthful-regeneration`, names blockers and calls for
manual patch/capture/state review. A suggestion is not execution or a
guarantee against subsequent filesystem changes.

##### Explicit apply and ordered no-write success

On the ordinary recipe path, D17 is evaluated before recipe execution:

| Order | Condition | Result |
|---|---|---|
| 1 | Malformed C or any binding failure, including invalid E | Named coverage refusal, exit 2, before mutation. |
| 2 | Valid incomplete C binds no readable recipe | `recipe-generation-incomplete`, exit 2; actual read cause retained. |
| 3 | Valid incomplete C binds readable undecodable bytes | Same named refusal, exit 2, including `recipe-undecodable`. |
| 4 | Valid incomplete C binds a decodable recipe | Warn with reasons, then use existing execution gates; no replay permission is inferred. |
| 5 | C absent and recipe absent/unreadable | Existing recipe-load error, exit 1. |
| 6 | C absent and recipe readable | Existing legacy load/execution behavior, including existing parse errors. |
| 7 | Valid complete C | Existing execution gates and stale-provenance warnings remain. |

For orders 2/3, an already-`applied` feature is directed to `verify` and
`status`, not another apply. Other states are directed to review the
canonical patch, then explicitly run `git apply --check <patch>` before
`git apply <patch>` if appropriate. Authoring a complete recipe and running
`tpatch implement <slug> --manual` is an alternative, but that checkpoint
moves the feature to `implementing`, including from `applied`.

**Canonical-patch reapply is separate.** State `unapplied` or a pending
unapplied baseline selects the existing patch branch of execute/done before
coverage recipe preflight. It uses the canonical patch and its existing
safety/materialization checks, not C as a grant. There is no `--reapply`
flag and no `--mode reapply`. Do not change lifecycle state merely to evade
a recipe refusal. `--dry-run` previews recipe operations; it is not a
coverage or future cross-base eligibility check.

ADR-042 preserves initial preimage authorization and operation order.
Exact current postimage may justify **no write**, not fresh write authority:
the relevant ordered prefix must preserve/restore the witness at that
operation's position, and exact bytes and containment are rechecked just
before skipping. Originally authorized writes may execute if the skip
ceases to apply; a postimage-only candidate cannot gain that permission.
Predictable witness invalidation refuses before any operation runs.
Missing expected targets, malformed gates, unreadability and path safety
remain refusals; legacy omitted gates retain their warning and ordinary
writes. Existing supersession severity is audit policy, not a safety proof.
A proved skip reports `[write-file] <path>: already present (exact postimage), no write`
and increments both `Applied` and `Skipped`. This bounded ordered proof
is **not a whole-worktree transaction** or a rollback promise for concurrent
changes or unexpected I/O failures.

### 6. Provider Interface

```go
type Provider interface {
    Check(ctx context.Context, cfg ProviderConfig) (*Health, error)
    Generate(ctx context.Context, cfg ProviderConfig, req GenerateRequest) (string, error)
}
```

**Config** (in `.tpatch/config.yaml`):
```yaml
provider:
  type: openai-compatible
  base_url: http://localhost:4141
  model: gpt-4o
  auth_env: GITHUB_TOKEN    # env var name, NOT the secret
```

**Heuristic fallback**: When no provider is available, generate template-based artifacts. Mark in `status.json` that heuristic mode was used.

**Auto-detection**: On first run, probe copilot-api at localhost:4141.

### 7. Reconciliation — 4-Phase Decision Tree

```
Phase 1: Reverse-Apply Check (fast, free)
    → If patch already present in upstream → UPSTREAMED

Phase 2: Operation-Level Evaluation (deterministic, no LLM)
    → If apply-recipe.json exists, evaluate each operation individually
    → All already-present → UPSTREAMED
    → Any conflict → BLOCKED
    → Mix → STILL-NEEDED

Phase 3: Provider-Assisted Semantic Check (when Phase 1-2 inconclusive)
    → Send acceptance criteria + upstream diff to LLM
    → upstreamed / still_needed / unclear

Phase 4: Forward-Apply Attempt (safety net)
    → Try git apply on new upstream
    → Clean → REAPPLIED
    → Conflicts → BLOCKED
```

### 8. Skill System

**6 harness formats** embedded in the binary, installed by `tpatch init`:

| Harness | Install Location |
|---------|-----------------|
| Claude Code | `.claude/skills/tessera-patch/SKILL.md` |
| GitHub Copilot | `.github/skills/tessera-patch/SKILL.md` |
| Copilot Prompt | `.github/prompts/tessera-patch-apply.prompt.md` |
| Cursor | `.cursor/rules/tessera-patch.mdc` |
| Windsurf | `.windsurfrules` |
| Generic | `.tpatch/workflows/tessera-patch-generic.md` |

**Parity guard**: `assets_test.go` ensures all formats mention current CLI commands.

### 9. Safety

1. `ensureSafeRepoPath()` before any file write outside `.tpatch/`
2. Deterministic apply recipe — generate, review, then execute
3. Secret-by-reference — store env var name, not secret
4. Patch artifact filtering — exclude `.tpatch/`, `.claude/skills/`, `.github/skills/`, `.github/prompts/`, `.cursor/rules/`, plus any registered linked Git worktree nested beneath the repository root (GH #7). Nested-worktree discovery uses `git worktree list --porcelain -z` exclusively and **requires Git 2.36+**; there is no fallback to the ambiguous newline-delimited shape, and any discovery failure refuses the capture.
5. Untracked file capture in patches

### 10. Sources

This spec consolidates decisions from:
- `review/RECOMMENDATIONS.md` §2 (technical decisions)
- `review/RECONCILIATION-REVIEW.md` §7.1 (4-phase reconciliation)
- `gpt/SPEC.md` (CLI contract, data model, provider interface)
- `cc/SPEC.md` (lifecycle phases, steering files, upstream tracking)
- `experimental/SPEC.md` (apply recipe format, safety constraints)

### 11. Success Criteria

The unified implementation passes the reconciliation bug bash:
1. Clone copilot-api at pinned commit, apply two features, simulate upstream update
2. Feature A (model translation fix) correctly identified as UPSTREAMED
3. Feature B (models CLI subcommand) correctly re-applied as STILL-NEEDED
4. Target repo tests pass after reconciliation
5. All 6 skill formats pass the parity regression test
