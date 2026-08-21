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
| `tpatch apply <slug> [--mode prepare\|started\|done] [--path]` | Execute recipe or record session |
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
verb runs Git. Retained blobs provide exact byte recovery only while present;
the archive is not canonical lifecycle truth, semantic certification,
authorship/provenance, or a general history/undo facility. Purging publishes
tombstones and can remove working-tree bytes, but deleting committed blobs does
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
