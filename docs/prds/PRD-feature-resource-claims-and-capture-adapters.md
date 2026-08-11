# PRD - Feature Resource Claims and Capture Adapters - `feat-feature-resource-claims-and-capture-adapters`

**Status**: Proposed
**Date**: 2026-08-10
**Owner**: Core
**Byline**: Cluster H rev-0
**Milestone**: v0.15.0 candidate (planning)
**Depends on**: [PRD-feature-file-claims](./PRD-feature-file-claims.md) (v1, shipped
`v0.9.0-alpha-1`), [PRD-record-capture-modes](./PRD-record-capture-modes.md) (v1,
shipped `v0.9.0-alpha-2`). Implementation requires
`ADR-033-resource-capture-boundary`.

## Related

- [SPEC.md](../../SPEC.md) — feature lifecycle state table and CLI surface
- [Feature Layout](../feature-layout.md) — canonical vs. audit-trail artifact map
- [Recording Patches](../record.md) — `record` capture boundary and provenance
- [PRD-feature-file-claims](./PRD-feature-file-claims.md) — advisory path claims,
  shipped v1
- [PRD-record-capture-modes](./PRD-record-capture-modes.md) — `--all` / `--staged`
  / `--unstaged` / `--claimed-only`, shipped v1
- [ADR-027 capture context privacy boundary](../adrs/ADR-027-capture-context-privacy-boundary.md)
  — storage lanes, redaction-as-precondition, default-off new capture surfaces
- [ADR-030 multi-slug reconcile derivation mode](../adrs/ADR-030-multi-slug-reconcile-derivation-mode.md)
  — `.git/**` exclusion at both the diff and store-write boundary (D3/D4
  precedent this PRD reuses)
- [ADR-024 patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md)
  — content-addressed ID precedent (`pg_<12hex>`), no-timestamps determinism
- [WP-002 capture and metadata foundation](../whitepapers/WP-002-capture-and-metadata-foundation.md)
  — claims/capture-modes cluster history and "no schema explosion" posture
- [WP-006 tpatch substrate and non-Git mode](../whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md)
  — Git-first posture; non-Git replay is a separate, unapproved ADR track
- [docs/state-of-the-art/storage-substrate-and-versioned-data.md](../state-of-the-art/storage-substrate-and-versioned-data.md)
  — Dolt research brief §3, "what tpatch should avoid" §9
- [docs/state-of-the-art/patch-capture-context-research-brief.md](../state-of-the-art/patch-capture-context-research-brief.md)
  — layered capture hypothesis this PRD's resource layer extends
- [ADR-032 feature unapply state boundary](../adrs/ADR-032-feature-unapply-state-boundary.md)
  — sibling planning-cluster precedent for structure/format of this PRD
- [ADR-033 resource capture boundary](../adrs/ADR-033-resource-capture-boundary.md)
  — the binding ADR this PRD requires before implementation

## Implementation Gate

Implementation requires a small binding ADR before code lands:

```text
ADR-033-resource-capture-boundary
```

The ADR must lock:

- manifest placement (new `resources.json` vs. extending `claims.json`) and the
  authority boundary between resource sidecars and canonical patch/lifecycle
  truth;
- resource ID derivation and selector normalization;
- the ignored-file/privacy boundary and its ADR-027 D3 redaction dependency;
- the logical Git metadata allowlist (never raw `.git/**`);
- the adapter discovery/execution/sandbox contract;
- Dolt's v1 scope as one closed-set adapter, not an authoritative store;
- `record --resources` transaction and partial-adapter-failure policy;
- the snapshot/diff wire schema;
- resource generation/amend/remove lifecycle and backward compatibility.

No code, schema, CLI behavior, or asset text changes until `ADR-033` reaches
**Accepted** status via the standard three-way review workflow. Cluster H is
planning-only; implementation is a separately dispatched Cluster H'.

## 0. Claims Audit

This PRD is a proposal. It changes nothing. Every claim below is verified
against current source at the time of writing (2026-08-10), not against
possibly-stale line numbers in older drafts.

| Claim | Evidence |
|---|---|
| `tpatch feature claim add\|list\|remove\|clear` persists advisory, path-only, manual-source claims to `.tpatch/features/<slug>/claims.json`; `kind` is a closed switch that accepts only `"path"` and rejects the reserved `glob`/`symbol`/`anchor` values at the input boundary. | `internal/store/claims.go:38-64` (kind/mode/source enums), `internal/store/claims.go:236-246` (`ValidateClaimKindInput`), `internal/cli/feature_claim.go:1-16` |
| Claim IDs are deterministic 12-hex-char SHA-256 prefixes over NUL-separated `(feature, kind, normalized value, mode)`; no wall-clock timestamp is part of identity. | `internal/store/claims.go:98-111` (`ComputeClaimID`) |
| `claims.json` is written atomically via a same-directory `.tmp` file, `fsync`, then `rename`; claims are stable-sorted by `claim_id` on every load/save. | `internal/store/claims.go:294-346` (`SaveClaims`), `internal/store/claims.go:348-355` (`sortClaims`) |
| Claim paths are normalized and validated against repo-escape, absolute-path, `.tpatch/`, and installed-skill-surface rejection using the same `safety.EnsureSafeRepoPath` helper used elsewhere in tpatch. | `internal/store/claims.go:179-234` (`NormalizeClaimPath`), `internal/safety/safety.go:12` (`EnsureSafeRepoPath`) |
| `tpatch record` supports `--all`, `--staged`, `--unstaged`, `--claimed-only`, plus the pre-existing `--auto`/`--from`/`--commit-range`/`--files`; all new-vs-new and new-vs-existing mode combinations are refused by a single mutex validator before capture begins. | `internal/cli/record_capture_modes.go:29-41` (mode enum), `internal/cli/record_capture_modes.go:65-145` (`validateRecordCaptureMode`) |
| `--claimed-only` refuses when the feature has no claims, refuses when the feature has claims but none are path-kind, and intersects with `--files` when both are given, refusing on an empty intersection. | `internal/cli/record_capture_modes.go:147-186` (`resolveClaimedOnly`) |
| Capture-mode provenance (`mode`, `pathspecs`, `claim_ids`) is a documented field group on the `patch-generations.json` `GenerationCapture` struct — a schema slot this PRD's resource layer must not collide with or silently repurpose. | `internal/store/patch_generations.go:63-67` (`GenerationCapture`) |
| Patch generation identity uses a `pg_<12hex>` SHA-256 prefix over NUL-separated identity fields (feature, generation, patch/recipe SHA, base/upper commit, capture mode, sorted pathspecs, sorted claim IDs); this is the ID-derivation precedent this PRD's `res_<12hex>` scheme follows. | `internal/store/patch_generations.go:254-276` (`ComputeGenerationID`) |
| `gitutil` already strips any diff stanza whose header path is `.git` or starts with/contains `.git/` at the diff-derivation boundary, as a defense-in-depth pattern for `.git/**` exclusion. | `internal/gitutil/gitutil.go:1051-1097` (`stripGitInternalFileStanzas`), `internal/gitutil/gitutil.go:1189-1211` (`pathIsGitInternal`) |
| ADR-030 independently locked a two-layer `.git/**` exclusion invariant (diff boundary + store-write boundary) for the multi-slug reconcile derivation path; this PRD's resource-diff writers must honor the same invariant rather than inventing a new one. | `docs/adrs/ADR-030-multi-slug-reconcile-derivation-mode.md` D3/D4 sections |
| tpatch already has a deterministic, git-native "is this path ignored" primitive (`git check-ignore -q --no-index`), returning ignored/not-ignored/`ErrGitUnavailable` as distinct outcomes. | `internal/gitutil/ignore.go:37-75` (`IsPathIgnored`) |
| `feature-layout.md` documents the canonical-vs-audit-trail pattern this PRD's resource sidecars reuse: a single always-current file (`artifacts/post-apply.patch`) plus an append-only numbered history (`patches/NNN-*.patch`), with an explicit "audit trail only, not replay input" framing. | `docs/feature-layout.md` "Canonical vs. audit trail" and "patches/NNN-<label>.patch" sections |
| ADR-027 D1 locks exactly two storage lanes (committed summary, local private buffer) and forbids any other write target, including metadata branches, notes, or external stores, without a future ADR. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md` D1 |
| ADR-027 D3 makes redaction a write precondition — "Redaction failure is a hard failure for committed summaries" — a rule this PRD's resource-snapshot writers must inherit verbatim. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md` D3 |
| ADR-027 D7 makes every *new* capture surface default-off until a downstream PRD proves it safe; existing `record`/claims/capture-mode/generation defaults are explicitly carved out as unaffected. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md` D7 |
| WP-006 recommends tpatch remain Git-first; it explicitly rejects a tpatch-native non-Git change-tracking substrate (Option E) without a dedicated whitepaper+ADR, and is itself still status `Exploring`, not accepted. | `docs/whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md` §4 Option E, header `**Status**: Exploring` |
| The storage-substrate research brief explicitly lists "a repo-local authoritative Dolt or SQLite database" and "a tpatch-native VCS for non-Git mode" under "What tpatch should avoid," and frames Dolt as a design reference, not an embeddable dependency. | `docs/state-of-the-art/storage-substrate-and-versioned-data.md` §9, §3.6, §3.8 |
| `Store` exposes deterministic per-feature directory helpers (`featureDir`, `featureArtifactsDir`, `featureStatusPath`) that this PRD's resource paths must compose with, not duplicate. | `internal/store/store.go:770-782` |
| tpatch's exit-code convention reserves `1` for generic/internal errors, `2` for CLI validation errors (`exitValidation`), and `3` for state/policy refusals (`exitStateRefus`); `verify` is the sole existing user of a bespoke `2` via `*ExitCodeError` for a non-generic reason. | `internal/cli/reject.go:46-47`, `internal/cli/exit_error.go:5-42`, `internal/cli/verify.go:43-45` |
| `tpatch feature` is the noun-scoped per-feature management group (`deps`, `claim`, `patch`, `unapply` all live there); ADR-031 D10 locked this group boundary explicitly so future per-feature nouns land under `feature`, not as new top-level verbs. | `internal/cli/feature_deps.go:38-51`, `docs/adrs/ADR-031-rejected-feature-state-data-model.md` D10 |

No code, schema, command behavior, or asset text is changed by this PRD.

## Summary

Feature file claims (v1) and record capture modes (v1) give tpatch a strong
story for ordinary Git-tracked repository files: declare intended scope, then
capture the Git working tree, index, or a committed range against that scope.
Neither primitive can represent a resource that Git intentionally does not
diff: an explicitly gitignored config/secret-template/generated file a feature
still needs to track intent about, a logical (not raw-byte) view of Git
metadata such as refs or effective attributes, or an external versioned store
such as Dolt whose schema/table diffs matter to the feature but never appear in
`git diff`.

This PRD adds a **feature resource layer**: a new, closed-kind, deterministic
manifest and CLI surface that lets a user or agent explicitly declare
non-file-claim resources per feature, and produce deterministic
add/list/remove/clear/diff metadata for them. Resource diffs are **sidecar
audit artifacts** — visible, reviewable, and useful for context — never
canonical patch content, never lifecycle state, and never a substitute for
`artifacts/post-apply.patch`.

The v1 user-facing surface:

```bash
tpatch feature resource add <slug> --kind <kind> <selector> [adapter flags...]
tpatch feature resource list <slug> [--json]
tpatch feature resource remove <slug> <resource-id-or-selector>
tpatch feature resource clear <slug>
tpatch feature resource diff <slug> [--resource <id>] [--dry-run] [--json]
tpatch record <slug> --resources [--dry-run]
```

Three closed resource kinds ship in v1: `ignored-file` (explicit gitignored or
git-untracked-generated files), `git-metadata` (a small allowlisted set of
logical Git views — never raw `.git/**` bytes), and `adapter-snapshot`
(deterministic command/export snapshots from one of two closed-set adapters:
the built-in `dolt` adapter, and a `generic-command` adapter for other
deterministic external tools). Dolt is realized purely as an optional
`adapter-snapshot` producer; it is discovered at runtime via `exec.LookPath`
and never becomes authoritative storage, never gains a new Go dependency, and
never participates in patch generation, reconcile, or replay decisions.

## 1. Problem Statement

tpatch's shipped capture story (file claims + capture modes) is complete for
one specific shape of resource: files that live in the Git working tree and
that Git is willing to diff. It has no vocabulary for the following four
real-world resource shapes:

1. **Explicit gitignored/generated files.** A feature may depend on a
   generated config, a local secret template, or a build artifact that is
   intentionally excluded from `git diff` by `.gitignore`. Today there is no
   way to say "this feature's story includes this ignored file" without
   force-adding it to Git (defeating the ignore rule) or losing the context
   entirely.
2. **Logical Git metadata.** A feature may care about a ref move, an effective
   `.gitattributes` resolution, or an index-stage fact that is not expressed
   as a working-tree file diff at all. tpatch has no story for this and must
   never treat `.git/**` bytes as capturable content (ADR-030 already locked
   this exclusion at the diff/store boundary for an unrelated reconcile
   defect; this PRD must not reopen that hole from a new direction).
3. **External/versioned resources such as Dolt.** Some repositories embed a
   Dolt database for structured data. A feature that changes a Dolt schema or
   table has no way to attach that fact to its tpatch feature record today.
4. **Deterministic command/export snapshots.** Some teams have their own
   deterministic export tooling (schema dumps, generated API clients,
   dependency lockfile diffs against a known baseline) that they would like to
   attach to a feature's audit trail without inventing a bespoke plugin system
   per tool.

Claims cannot cover any of this: `ClaimKindPath` is the only writable kind in
v1 and it operates purely on repo-relative paths that Git already diffs
(`internal/store/claims.go:236-246`). Capture modes cannot cover this either:
every mode (`--all`/`--staged`/`--unstaged`/`--auto`/`--from`/
`--commit-range`) is defined purely in terms of `git diff`/`git apply`
boundaries (`internal/cli/record_capture_modes.go:29-145`). Extending either
primitive to carry non-file-diff content would either break the closed
`ClaimKind` switch's meaning ("this repo-relative path is Git-diffable scope")
or silently make `record`'s canonical patch responsible for content Git was
never meant to diff — the exact failure class ADR-030 already had to fix once
for `.git/**` leakage into reconcile-derived patches.

### 1.1 Existing-primitives preflight — why claims + capture modes alone cannot represent non-file resources

| Existing primitive | What it represents | Why it cannot carry non-file resources |
|---|---|---|
| `claims.json` (`ClaimKindPath`) | "This feature expects to touch this repo-relative, Git-diffable path." | The value field is validated as a normalized repo-relative path via `safety.EnsureSafeRepoPath` and is matched against `git diff`/`git apply` boundaries downstream (`--claimed-only`). An ignored file, a Dolt table name, or a Git ref name is not a repo-relative path Git will diff; forcing it through `NormalizeClaimPath` would either reject it (path escapes / not on disk) or silently accept a string that has no diff-time meaning. |
| Reserved `ClaimKindGlob`/`ClaimKindSymbol`/`ClaimKindAnchor` | Reserved for finer-grained *path/AST* scoping within the same Git-diffable universe. | These reserved kinds narrow or widen a path match; they do not introduce a new content universe (ignored files, Git metadata views, external database diffs). Writing an adapter-snapshot resource under a repurposed `ClaimKindAnchor` would violate the schema's own documented reservation and silently conflate two different guarantees (`internal/store/claims.go:38-46`). |
| `record` capture modes (`--all`/`--staged`/`--unstaged`/`--auto`/`--from`/`--commit-range`) | "Which Git diff boundary produced the canonical patch." | Every mode's implementation is a `git diff`/`git apply --cached --check` call (`internal/gitutil/capture_modes.go:137-360`). There is no mode that can represent "diff of a Dolt table" or "the effective `.gitattributes` for this path," because those are not Git object diffs at all. |
| `patch-generations.json` (`GenerationCapture`) | Append-only audit of which capture mode/pathspecs/claim IDs produced a given canonical patch generation. | It records *how the canonical Git patch was captured*, not *what non-Git resources exist for the feature*. Repurposing it would conflate patch-generation identity (content-addressed over patch bytes) with resource-declaration identity (content-addressed over a selector), and would violate ADR-024's closed schema boundary. |
| `.tpatch/features/<slug>/artifacts/post-apply.patch` | Canonical, always-current, replay-authoritative feature diff. | This PRD's binding safety boundary (see §1, ROADMAP Cluster H entry) is explicit: resource diffs are v1 sidecars, not canonical patch content. Merging resource bytes into `post-apply.patch` would make `git apply` responsible for content Git cannot represent (ignored files bypass the working tree Git diffed against; Dolt diffs are not Git objects at all). |

Conclusion: none of the shipped primitives can be stretched to cover these
four resource shapes without breaking an existing closed schema boundary or
silently making Git-apply machinery responsible for non-Git content. A new,
narrowly-scoped, sidecar-only resource layer is required.

## 2. Goals / Non-goals

### Goals

1. Add a persistent, per-feature resource manifest (`resources.json`) that is
   structurally and conceptually separate from `claims.json`, while reusing
   claims' deterministic-ID, atomic-write, and stable-sort conventions.
2. Support exactly three closed resource kinds in v1: `ignored-file`,
   `git-metadata`, `adapter-snapshot`.
3. Provide `add`/`list`/`remove`/`clear`/`diff` verbs under
   `tpatch feature resource`, mirroring the `feature claim` verb set.
4. Define a deterministic, versioned wire envelope for resource snapshot/diff
   sidecar artifacts, reusing the feature-layout canonical-vs-audit-trail
   pattern (`current` overwritten file + numbered `history/` audit trail).
5. Define an adapter capability/execution protocol precise enough for a future
   implementation cluster to build without further design decisions:
   discovery, identity, args/env/stdin, cwd, timeout, output limits, exit/error
   taxonomy, and injection-safety.
6. Make Dolt one closed-set `adapter-snapshot` producer among (at most) two in
   v1 (`dolt`, `generic-command`) — never a new core Go dependency, never
   authoritative storage.
7. Define exactly how (and whether) `tpatch record --resources` consumes
   declared resources, including full transaction/partial-failure semantics.
8. Preserve every existing safety invariant: `.git/**` raw content stays
   forbidden at both the diff and store-write boundary; ignored files are
   never swept implicitly; every new capture surface stays default-off per
   ADR-027 D7 until this PRD (and its binding ADR) makes it safe and explicit.
9. Leave ADR-027's privacy/redaction contract (D2/D3/D4/D6/D7/D9/D10) as the
   binding constraint for every resource snapshot, without re-deriving it.

### Non-goals

1. No open/extensible resource-kind plugin system in v1. Kinds are closed;
   a fourth kind requires a new PRD revision, not a runtime plugin discovery
   mechanism.
2. No runtime *binary* plugin discovery (scanning PATH for
   `tpatch-resource-adapter-*` executables or similar). "Optional adapters
   discovered at runtime" means tpatch checks whether a known external tool
   (`dolt`) is present on `PATH` at execution time — it does not mean tpatch
   auto-loads unknown third-party adapter binaries.
3. No implicit sweep of gitignored files. Every `ignored-file` resource is an
   explicit, individually-added selector.
4. No raw `.git/**` byte capture, in any resource kind, under any
   circumstance.
5. No Dolt (or any adapter) becoming the authoritative store for any tpatch
   data. Adapter output is always a read-only, one-way, sidecar-audit
   snapshot.
6. No non-Git replay substrate. Git remains the only change-tracking
   substrate this PRD assumes or requires; WP-006's broader non-Git question
   is out of scope and unresolved by this PRD.
7. No merging of resource diffs into `artifacts/post-apply.patch`,
   `patch-generations.json`, or any lifecycle-state field. Resource diffs
   never gate `apply`, `reconcile`, `land`, `verify`, or dependency
   satisfaction in v1.
8. No interactive hunk-level UI for resource diffs (mirrors
   `PRD-record-capture-modes` §2 non-goal 1).
9. No provider/AI-assisted resource selection or redaction judgment. Redaction
   is deterministic pattern-based scanning per ADR-027 D3, not a model call.
10. No new global repo-level "enable adapters" config toggle. Per-resource
    `add` is itself the explicit per-resource opt-in (§6.6).
11. No strict/enforcing resource claims (no analogue of the deferred `strict`
    claim mode). All v1 resources are advisory/audit-only.
12. No automatic promotion of resource snapshots into the ADR-027 committed
    summary lane beyond what this PRD explicitly defines; no local-private
    buffer lane is introduced by this PRD (resource sidecars are always
    tracked-tree artifacts, never `.git/tpatch/capture/`-style local state).

## 3. Command Surface

### 3.1 Candidate command evaluation

| Candidate | Assessment |
|---|---|
| `tpatch feature resource add\|list\|remove\|clear\|diff <slug> ...` | **Chosen.** Mirrors the shipped `feature claim` and `feature deps` noun-scoped pattern (`internal/cli/feature_deps.go:38-51`); ADR-031 D10 already locked `feature` as the home for per-feature nouns. |
| `tpatch feature claim add --kind resource ...` (extend claims verb) | Rejected. Would require repurposing the closed `ClaimKind` switch (violates §1.1's conclusion) and would conflate two different write-boundary guarantees (Git-diffable path vs. sidecar snapshot). |
| `tpatch resource ...` (top-level, no `feature` prefix) | Rejected. Every resource is feature-scoped by construction (`.tpatch/features/<slug>/resources.json`); a top-level verb would need a mandatory `<slug>` argument identical to the `feature` group's existing shape, adding a redundant namespace. |
| `tpatch feature resources ...` (plural noun) | Rejected for consistency: `feature claim`, `feature deps`, `feature patch` are all singular nouns in the existing surface (`internal/cli/feature_deps.go:38-51`, `internal/cli/feature_claim.go`); a plural resource noun would be the only inconsistency in the group. |

**Locked**: `tpatch feature resource <verb> <slug> ...` (singular noun,
matching `feature claim`/`feature deps`/`feature patch`).

### 3.2 `add`

```bash
tpatch feature resource add <slug> --kind ignored-file <path>
tpatch feature resource add <slug> --kind git-metadata <view>[:<key>]
tpatch feature resource add <slug> --kind adapter-snapshot --adapter dolt --capability schema-diff --arg table=users --arg from=main --arg to=HEAD
tpatch feature resource add <slug> --kind adapter-snapshot --adapter generic-command --arg cmd=<absolute-or-PATH-name> --arg args="<space-separated>" [--arg cwd=<repo-relative-dir>] [--arg env=NAME1,NAME2] [--arg timeout=<seconds>]
```

Rules (apply to every kind unless a kind-specific rule overrides):

- `<slug>` must identify an existing feature (`Store.FeatureExists`, same
  precondition as `feature claim add`).
- `--kind` is required and must be one of the three closed v1 kinds; any other
  value is refused at exit code 2 (validation) before any I/O.
- The selector is normalized and validated per kind (§5). Validation failures
  that are shape/argument problems (missing required `--arg`, malformed
  selector syntax) are exit code 2. Validation failures that are safety/policy
  refusals (path not actually ignored, `.git/**` reference, unsafe adapter
  path, disallowed Git-metadata key) are exit code 3.
- Adding an already-present normalized resource (same `kind` + normalized
  selector + normalized adapter args) is idempotent, mirroring
  `AddClaim`'s idempotent-append behavior (`internal/store/claims.go:356-370`).
- `add` prints the exact command/args/adapter identity it is about to persist
  *before* writing, so the user sees precisely what will execute on a future
  `diff`/`record --resources` — this is the resource-layer analogue of
  ADR-027 D8's hook-transparency requirement. `add` itself does not execute
  the adapter; execution happens only at `diff` or `record --resources` time.
- `--dry-run` prints the normalized resource row and computed `resource_id`
  without writing `resources.json`.

### 3.3 `list`

```bash
tpatch feature resource list <slug> [--json]
```

Mirrors `feature claim list` exactly: human output is one line per resource
(`resource_id`, `kind`, normalized selector, adapter identity if applicable);
`--json` emits the full manifest, stable-sorted by `resource_id`.

### 3.4 `remove` / `clear`

```bash
tpatch feature resource remove <slug> <resource-id-or-selector>...
tpatch feature resource clear <slug>
```

Mirrors `feature claim remove`/`clear` (`internal/cli/feature_claim.go`
`featureClaimRemoveCmd`/`featureClaimClearCmd`): accepts a full or
`>=7`-char unambiguous `resource_id` prefix, or the exact normalized selector
string. `remove` deletes the manifest row **and** the resource's entire
sidecar directory (`resources/<resource_id>/`, current snapshot/diff plus
`history/`). This intentionally does **not** retain history after removal —
matching claims' own no-retained-history-on-remove behavior
(`PRD-feature-file-claims.md` §3.3, `internal/store/claims.go:436-444`
`RemoveClaim`). `clear` removes every resource and its sidecar directory for
the feature, mirroring `featureClaimClearCmd`.

### 3.5 `diff`

```bash
tpatch feature resource diff <slug> [--resource <id>] [--dry-run] [--json]
```

Runs (or, with `--dry-run`, previews) the snapshot/diff pipeline for one
resource (`--resource <id>`) or every declared resource (default). On success,
writes the deterministic sidecar wire envelope (§7) and prints a human-readable
or `--json` summary. This is a read-only-to-the-repo operation: it never
touches `artifacts/post-apply.patch`, `status.json`, or `patch-generations.json`.
`--dry-run` executes normalization/hashing against a scratch buffer without
writing sidecar files, so a user can preview exactly which adapter commands
would run and what they would report, without any on-disk effect.

### 3.6 `record --resources`

```bash
tpatch record <slug> --resources [--dry-run]
tpatch record <slug> --all --resources
tpatch record <slug> --staged --resources
```

`--resources` is a **post-hoc, opt-in, always-explicit** filter flag on
`record`, independent of the Git capture-mode mutex group (§3.7 of
`PRD-record-capture-modes`; it does not participate in that mutex — it can
combine with any Git capture mode, or with none if the feature is
resource-only). Full semantics, including transaction/failure behavior, are
defined in §9.

## 4. Persisted Manifest

Path:

```text
.tpatch/features/<slug>/resources.json
```

This is a **new file**, sibling to `claims.json`, not an extension of it. §5
of `ADR-033-resource-capture-boundary` records the alternatives considered and
why the schema is separate.

Schema:

```json
{
  "version": 1,
  "feature": "model-picker",
  "resources": [
    {
      "resource_id": "res_8f31c0a19b2d",
      "kind": "ignored-file",
      "selector": "config/local-secrets.env.template",
      "adapter": null,
      "capability": null,
      "args": {},
      "source": "manual"
    },
    {
      "resource_id": "res_47a0de331851",
      "kind": "git-metadata",
      "selector": "refs",
      "adapter": null,
      "capability": null,
      "args": {},
      "source": "manual"
    },
    {
      "resource_id": "res_1c2d3e4f5a6b",
      "kind": "adapter-snapshot",
      "selector": "dolt:schema-diff:users",
      "adapter": "dolt",
      "capability": "schema-diff",
      "args": {
        "table": "users",
        "from": "main",
        "to": "HEAD"
      },
      "source": "manual"
    }
  ]
}
```

Field rules:

| Field | Rule |
|---|---|
| `version` | Manifest schema version. Required. `1` in this PRD. |
| `feature` | Feature slug. Must match the directory slug (mirrors `ClaimsManifest.Feature`). |
| `resource_id` | Deterministic `res_<12hex>` — SHA-256 prefix over NUL-separated `(feature, kind, normalized selector, adapter, capability, canonical-JSON-sorted args)`, mirroring `ComputeGenerationID`'s NUL-separated hashing pattern (`internal/store/patch_generations.go:254-276`) rather than claims' shorter tuple, because resources carry adapter identity that claims do not. |
| `kind` | One of the three closed v1 values: `ignored-file`, `git-metadata`, `adapter-snapshot`. Closed switch; unknown values are rejected on write and tolerated (but flagged) on read, mirroring `ValidateClaimKindInput`'s read/write asymmetry (`internal/store/claims.go:236-246`). |
| `selector` | Kind-specific normalized string (§5). Never an absolute path; never raw `.git/**` byte content. |
| `adapter` | `null` for `ignored-file`/`git-metadata`. For `adapter-snapshot`, one of the two closed v1 adapter names: `"dolt"` or `"generic-command"`. |
| `capability` | `null` unless `kind == "adapter-snapshot"`. Names a fixed sub-operation of the adapter (§6). |
| `args` | Map of string→string. Empty object (`{}`), never `null`, for kinds/adapters that take no arguments. Never contains secret values — only names, paths, refs, and adapter-specific identifiers (ADR-027 D2 secret-by-reference rule). |
| `source` | `"manual"` in v1 (mirrors `ClaimSourceManual`); reserved values `agent`/`imported`/`generated` follow the claims precedent for future PRDs, and are rejected on write in v1. |

Determinism rules, mirroring `claims.json` (`internal/store/claims.go:294-346`
`SaveClaims`) and `patch-generations.json` (ADR-024):

- resources are stable-sorted by `resource_id` on every load/save;
- no wall-clock timestamps anywhere in the manifest or in ID derivation;
- atomic write: marshal → `resources.json.tmp` → `fsync` → `rename`;
- a missing file is an empty manifest, not an error (mirrors `LoadClaims`,
  `internal/store/claims.go:263-278`).

## 5. Resource Kinds and Selector Normalization

Kinds are **closed in v1**. A fourth kind is a future PRD revision decision,
not a runtime-discoverable extension point. This section is binding; §16 does
not reopen it as an open question.

### 5.1 `ignored-file`

Selector: a normalized repo-relative path or directory (same Clean/ToSlash/
trailing-slash-for-directories shape as `NormalizeClaimPathShape`,
`internal/store/claims.go:139-177`), reused verbatim rather than
reimplemented.

Add-time validation (all must pass, in order, or `add` refuses at exit code
3):

1. Path passes the same repo-escape / absolute-path / `.tpatch/` /
   installed-skill-surface rejection as `NormalizeClaimPath`
   (`internal/store/claims.go:179-234`).
2. Path is **not** inside `.git/` (defense in depth; already excluded by
   `.tpatch/`-style prefix rejection reuse plus an explicit additional
   `.git/`-prefix check, mirroring `pathIsGitInternal`,
   `internal/gitutil/gitutil.go:1189-1211`).
3. Path **must currently be ignored** per `gitutil.IsPathIgnored`
   (`internal/gitutil/ignore.go:59-75`). If `IsPathIgnored` returns
   `ErrGitUnavailable`, `add` refuses (fail-closed, mirroring the existing
   `ErrGitUnavailable` refusal-class contract). If the path is **not**
   ignored, `add` refuses with a diagnostic pointing at `feature claim add`
   instead — an ordinary Git-tracked or plain-untracked-but-not-ignored file
   belongs to the existing claims/record primitives, not this kind.
4. Directory selectors are permitted; matching descendants at snapshot time
   uses the same "directory claim covers all descendants" semantics as
   claims (`PRD-feature-file-claims.md` §7).

Snapshot content: the literal current bytes of the file(s) at
`diff`/`record --resources` time, subject to the ADR-027 D3 redaction
precondition (§8).

### 5.2 `git-metadata`

Selector: `<view>` or `<view>:<key>`, where `<view>` is one of a fixed
allowlist. **No other view name is accepted in v1.**

| View | Key shape | What it captures | Explicitly excluded |
|---|---|---|---|
| `refs` | none | Sorted `git for-each-ref` output: ref name + target OID, restricted to `refs/heads/**` and `refs/tags/**`. | `refs/remotes/**` (may embed remote-specific tokens in some hosting setups), reflogs, `HEAD` reflog, any raw `.git/refs/**` file bytes. |
| `attributes` | `<key>` = a repo-relative path or pathspec | Effective `.gitattributes` resolution for the given path via `git check-attr --all -- <path>`. | Raw `.gitattributes` file bytes (those are ordinary tracked files and belong to claims/record if needed) — this view is specifically the *resolved* per-path attribute set. |
| `index-summary` | `<key>` = a repo-relative path or pathspec | Normalized `git ls-files -s -- <path>` output: path, mode, stage, object ID. | Raw `.git/index` bytes; blob content (only the object ID is captured, never dereferenced). |
| `config` | `<key>` = one of a fixed allowlist: `core.*`, `remote.*.url`, `branch.*.merge`, `user.name`, `user.email` | The resolved value(s) for the given allowlisted key pattern via `git config --get-regexp`. | Every other config key, and specifically `credential.*`, `http.*.extraheader`, `url.*.insteadof`, and any key whose resolved value looks like a URL with embedded userinfo (`user:pass@host`) — those are refused at exit code 3 even if the key pattern would otherwise match, per the ADR-027 D3 redaction precondition. |

Rules:

- `add` validates `<view>` against the table above; unknown views are exit
  code 2 (unrecognized argument shape). Disallowed `config` keys (e.g.
  `credential.helper`) are exit code 3 (policy refusal), not 2, because the
  view name itself was valid — the specific key was not.
- No `git-metadata` selector may ever resolve to raw bytes under `.git/**`.
  This is the same invariant ADR-030 D3/D4 already locked for reconcile's
  derived diffs; this PRD's `git-metadata` snapshot writer must call the same
  class of guard (logically: reuse `pathIsGitInternal`-style detection over
  any adapter output that happens to echo a path) before persisting.
- Snapshot content for `git-metadata` is always the *logical command output*
  (ref names + OIDs, resolved attribute pairs, ls-files summary rows, resolved
  config values) — never a raw filesystem read of anything under `.git/`.

### 5.3 `adapter-snapshot`

Selector: `<adapter>:<capability>[:<discriminator>]`, e.g.
`dolt:schema-diff:users`, `generic-command:snapshot:api-clients`. The
discriminator is a short user-chosen label (for `dolt`, conventionally the
table name; for `generic-command`, a short identifier the user picks) used
only for human readability and as part of resource-ID uniqueness — it carries
no adapter-execution meaning by itself; the actual adapter invocation is
driven entirely by `args`.

Two adapters are closed-set in v1 (§6):

- `dolt` — capabilities `schema-diff`, `table-diff`.
- `generic-command` — capability `snapshot` (a single fixed capability: run
  one declared deterministic command and capture its normalized output).

Add-time validation: `--adapter` must be one of the two closed values;
`--capability` must be valid for the chosen adapter; required `--arg` keys for
that capability must be present (§6 tables). Any violation is exit code 2.

## 6. Adapter Capability / Execution Protocol

This section is the binding contract for both adapters. A future
implementation cluster (Cluster H') must not need to make any further design
decision to build against it.

### 6.1 Discovery

"Discovered at runtime" means: at `diff`/`record --resources` execution time
(never at `add` time), tpatch resolves the adapter's required external binary
via `exec.LookPath("dolt")` (for the `dolt` adapter) or via the user-declared
`cmd` argument (for `generic-command`, §6.5). tpatch does **not** scan `PATH`
for unknown adapter plugin binaries; the set of adapter *names* is closed and
compiled into tpatch itself (§5.3, Non-goal 2). "Optional" means: if the
required binary is absent, the specific resource's snapshot/diff fails with a
classified `adapter-missing` error (§6.7) — it does not block `add`, `list`,
`remove`, or the rest of `record`.

### 6.2 Executable / version identity

Every successful adapter execution records, alongside the snapshot:

- the resolved absolute path of the executable used (from `exec.LookPath` or
  the user-declared `cmd`);
- a `version_probe` string: the trimmed stdout of a fixed, adapter-defined,
  read-only version command (`dolt version` for `dolt`; for
  `generic-command`, the literal string `"unversioned"` unless the user
  declared a `--arg version_cmd=...` override, in which case that command's
  trimmed stdout is captured with the same timeout/size limits as the main
  capability command).

This identity is informational provenance only — it never gates success or
failure, and it never becomes part of resource ID derivation (adapter versions
change independently of resource *declaration* identity).

### 6.3 Args / env / stdin

- Args are always passed as an explicit `argv` array built by tpatch from the
  fixed per-capability template plus the resource's declared `args` map.
  tpatch **never** invokes a shell (`sh -c`, `cmd /c`, or equivalent); every
  adapter call uses direct `exec.Command(path, argv...)` with no shell
  interpolation, closing the injection vector the task's binding constraints
  call out.
- Environment: the adapter subprocess inherits **no** environment variables by
  default. If a resource's `args` map includes `env` (a comma-separated list
  of variable *names*, never values — e.g. `env=DOLT_ROOT_PATH,DOLT_CONFIG_DIR`),
  tpatch looks up each named variable's *current* value from the invoking
  process's environment at execution time and passes only those
  name/value pairs through. No value is ever persisted to `resources.json`,
  a snapshot file, or a diff artifact — only the variable *names* are
  persisted, per ADR-027 D2's secret-by-reference rule. If a declared name is
  unset in the environment, the adapter runs without it (not an error).
- Stdin: closed (`/dev/null`-equivalent) unless a capability explicitly
  declares stdin usage. No v1 capability in this PRD declares stdin usage;
  this is reserved for symmetry with the general design, not implemented in
  v1.

### 6.4 Working directory

Every adapter invocation's `cwd` defaults to the repository root. A resource
may declare `args.cwd` as a repo-relative directory; it is validated with
`safety.EnsureSafeRepoPath` (identical helper to claims, `internal/safety/
safety.go:12`) before use, and `add` refuses at exit code 3 if it escapes the
repo or resolves to `.tpatch/` or an installed skill surface (reusing the
prefix list at `internal/store/claims.go:120-133`).

### 6.5 The `generic-command` adapter contract

- `args.cmd` (required): either a bare executable name resolved via
  `exec.LookPath`, or a path. An absolute path outside the repository root is
  permitted (the tool itself is expected to live outside the repo, like
  `dolt`), but a path *inside* the repository root must pass
  `safety.EnsureSafeRepoPath` and must not resolve under `.git/`.
- `args.args` (optional): a single string, split on unescaped whitespace into
  an `argv` tail using the same quoting rules as a POSIX shell word-split
  (implemented without invoking a shell — Go's own tokenizer, not
  `sh -c`), so users can express `--flag "quoted value"` without granting
  shell metacharacter interpretation (`;`, `|`, `` ` ``, `$(...)` are treated
  as literal characters in whatever argument they appear in, never as shell
  syntax).
- `args.env` (optional): comma-separated variable name allowlist, per §6.3.
- `args.cwd` (optional): per §6.4.
- `args.timeout` (optional): seconds, integer, default `30`, hard cap `300`
  (`add` refuses values above the cap at exit code 2).

### 6.6 Explicit opt-in and transparency (no global toggle)

There is no repo-level "enable adapters" configuration flag. The `add` command
itself — printing the exact resolved command shape before persisting (§3.2) —
is the explicit, per-resource, user-visible opt-in. This mirrors ADR-027 D7's
"new capture surface is default-off until a PRD proves it safe": the
resource-declaration step *is* the proof-of-intent gate, and no resource
executes anything until a later `diff` or `record --resources` invocation.

### 6.7 Output limits and exit/error taxonomy

Every adapter invocation is bounded by:

- **timeout**: `dolt` capabilities default to 30s (not user-configurable in
  v1 — schema/table diffs are expected to be fast; a future PRD can add a
  flag if this proves wrong); `generic-command` uses `args.timeout` (§6.5).
- **output size cap**: 5 MiB of combined stdout (stderr is captured for
  diagnostics only, capped at 64 KiB, and is never persisted into a
  committed snapshot — only surfaced in the CLI error message on failure).

Classified outcomes, each mapped to one exit code:

| Outcome | Exit code | Meaning |
|---|---|---|
| `ok` | 0 | Adapter ran, produced output within limits, redaction passed. |
| `adapter-missing` | 1 | Required binary not found via `exec.LookPath` at execution time. |
| `timeout` | 1 | Adapter process exceeded its timeout; process is killed. |
| `nonzero-exit` | 1 | Adapter process exited non-zero. |
| `output-too-large` | 1 | Combined stdout exceeded the 5 MiB cap; process output is discarded, not truncated-and-kept (truncating silently would produce a non-deterministic partial snapshot). |
| `unsafe-path` | 3 | `args.cmd`/`args.cwd` failed `safety.EnsureSafeRepoPath` or resolved under `.git/`. |
| `redaction-refused` | 3 | ADR-027 D3 redaction scan flagged the captured output; snapshot is refused, not silently scrubbed-and-kept (matches ADR-027 D3 "Redaction failure is a hard failure for committed summaries"). |
| `closed-kind-rejected` / `closed-adapter-rejected` | 2 | `--kind`/`--adapter`/`--capability` outside the closed v1 sets. |
| `internal-error` | 1 | Any other Go-level failure (I/O, JSON marshal, etc.). |

`1` (generic runtime failure) is used for adapter-execution-class failures,
matching the existing convention that `--claimed-only`'s empty-claims/empty-
intersection refusals are plain `fmt.Errorf` (exit 1), not `*ExitCodeError`
(`internal/cli/record_capture_modes.go:154,163`). `2` is reserved for CLI
input/shape errors (matches `exitValidation`, `internal/cli/reject.go:46`).
`3` is reserved for safety/policy-boundary refusals (matches
`exitStateRefus`, `internal/cli/reject.go:47`), consistent with how
`feature_deps.go:202` uses `exitStateRefus` for DAG-policy refusals.

### 6.8 Deterministic normalization

Before hashing or persisting any adapter output:

1. Normalize line endings to `\n`.
2. Strip trailing whitespace per line.
3. For capabilities whose output is a set of rows with no inherent order
   (`dolt table-diff`, `git-metadata:refs`, `git-metadata:index-summary`),
   sort lines lexicographically before hashing/diffing so re-running the same
   capability against unchanged underlying state reproduces byte-identical
   output. Capabilities whose output is inherently ordered (e.g. a schema
   diff's column ordering) are **not** re-sorted.
4. Record both a `raw_sha256` (post line-ending-normalization, pre-sort) and a
   `normalized_sha256` (post-sort where applicable) in the wire envelope (§7),
   so an auditor can distinguish "adapter output changed" from "adapter output
   was merely reordered by a non-deterministic backend."

## 7. Snapshot / Diff Artifact Layout and Wire Envelope

Reusing the feature-layout canonical-vs-audit-trail pattern
(`docs/feature-layout.md` "Canonical vs. audit trail"):

```text
.tpatch/features/<slug>/resources.json               ← manifest (this PRD §4)
.tpatch/features/<slug>/resources/<resource_id>/
├── snapshot.json          ★ CURRENT normalized snapshot, always overwritten
├── diff.json              ★ CURRENT diff vs. the previous snapshot, always overwritten
└── history/
    ├── 001-diff.json      ← historical full diff-envelope snapshots, append-only
    ├── 002-diff.json      ← each file is a COMPLETE envelope at write-time,
    └── …                     NOT an incremental delta between numbers.
```

`snapshot.json` and `diff.json` (current) mirror `artifacts/post-apply.patch`:
always-current, replaced on every successful `diff`/`record --resources` run.
`history/NNN-diff.json` mirrors `patches/NNN-*.patch`: append-only audit trail,
never replay input, safe to prune down to the newest entry.

Wire envelope for `diff.json` / `history/NNN-diff.json` (identical shape;
`history/` files are just point-in-time copies):

```json
{
  "version": 1,
  "feature": "model-picker",
  "resource_id": "res_1c2d3e4f5a6b",
  "kind": "adapter-snapshot",
  "adapter": "dolt",
  "capability": "schema-diff",
  "executable_path": "/usr/local/bin/dolt",
  "version_probe": "dolt version 1.42.0",
  "raw_sha256": "9f3a...",
  "normalized_sha256": "b71e...",
  "outcome": "ok",
  "changes": {
    "added": [],
    "removed": [],
    "changed": [
      {
        "path": "schema/users.sql",
        "change_kind": "modified"
      }
    ]
  },
  "body_ref": "resources/res_1c2d3e4f5a6b/snapshot.json"
}
```

Rules:

- `version` is the envelope schema version, independent of `resources.json`'s
  `version` field.
- `added`/`removed`/`changed` are **never `null`** — always `[]` when empty,
  matching the existing tpatch convention of stable non-null empty arrays
  (`sortDedupe` returns `nil` only for slices that formatters already treat as
  "none" — this PRD's JSON arrays are the user-facing artifact and must not
  rely on that Go-internal nil/empty distinction leaking into JSON as
  `null`).
- Ordering inside `added`/`removed`/`changed` is stable-sorted by `path`
  (or, for kinds without a path-shaped identity such as `git-metadata:refs`,
  by the row's own normalized string).
- `body_ref` points at the current `snapshot.json` sidecar (the full
  normalized snapshot content), keeping the diff envelope itself small and
  diff-focused; `snapshot.json` is the literal normalized output (post §6.8
  normalization) with no additional wrapping.
- `outcome` uses the taxonomy in §6.7. A non-`ok` outcome still writes an
  envelope (so failure is auditable) but does **not** overwrite a prior
  successful `snapshot.json`/`diff.json` — see §9 for the exact
  no-partial-overwrite rule.
- No wall-clock timestamp field exists anywhere in this envelope, matching
  ADR-024/ADR-027's determinism rule.

## 8. Privacy, Redaction, and Safety

This PRD does not redefine ADR-027; it binds every resource-snapshot writer to
it:

- **Redaction is a write precondition** (ADR-027 D3): every captured
  `ignored-file` byte content, `git-metadata` resolved value, and adapter
  stdout is scanned by the existing redaction contract before
  `snapshot.json`/`diff.json` is written. A redaction hit is a hard failure
  (`redaction-refused`, §6.7) — not a silent scrub-and-continue.
- **Secret-by-reference** (ADR-027 D2): resource `args` may name an
  environment variable (`env=NAME`) but never contain a value; this PRD's
  `config` git-metadata view explicitly excludes credential-shaped keys
  (§5.2).
- **Default-off for the surface as a whole** (ADR-027 D7): no resource
  executes anything until an explicit `diff` or `record --resources`
  invocation; `add` never executes an adapter (§3.2, §6.6).
- **Local-first, no external upload** (ADR-027 D10): resource snapshots never
  leave the local filesystem under this PRD; there is no provider-assisted
  resource capture in v1 (Non-goal 9).
- **Symlink and path safety**: `ignored-file` selectors resolving to a
  symlink are resolved and re-validated with `safety.EnsureSafeRepoPath`
  against the symlink's *target*, not just its literal path text, mirroring
  the symlink-aware validation `NormalizeClaimPath` already performs via the
  same helper (`internal/store/claims.go:205-209`).
- **Size/binary policy**: an `ignored-file` resource whose content exceeds
  5 MiB is refused at `add` time with a diagnostic (not silently truncated);
  binary content (detected via a NUL-byte heuristic in the first 8 KiB, the
  same heuristic class git itself uses) is stored as a `body_ref` pointing at
  a raw byte copy plus a `raw_sha256`, with no attempted textual diff (`changes`
  is reported as a single `changed` entry with `change_kind: "binary"` and no
  line-level detail).

## 9. Auto-record Integration, Transaction, and Failure Semantics

`tpatch record <slug> --resources`:

1. Runs strictly **after** the feature's Git-side canonical patch capture
   (whichever capture mode was selected, or the default) has already
   succeeded and been validated. `--resources` never blocks or alters the
   Git-side capture path; a feature with zero declared resources and
   `--resources` set refuses immediately (exit 1, mirroring
   `--claimed-only`'s empty-claims refusal shape) rather than silently
   no-op-succeeding, so a typo'd invocation is visible.
2. Stages **every** declared resource's snapshot/diff into a temporary
   scratch directory first (parallel structure to `resources/<id>/` but under
   a `.tmp-<random>/` prefix inside the same `resources/` parent so the final
   rename stays same-filesystem-atomic).
3. Runs each declared resource's capability with the limits in §6.7. If
   **any** resource's capability outcome is not `ok`, the **entire**
   `--resources` batch for this invocation is refused: no `resources/<id>/
   snapshot.json`/`diff.json` file is updated, no `history/NNN-diff.json` is
   appended, for **any** resource in the batch — including the ones that
   individually succeeded. This is deliberate all-or-nothing batch semantics,
   not partial-success: a partially-updated resource sidecar tree would leave
   the audit trail internally inconsistent about "what did this record
   invocation actually capture."
4. Only after every declared resource's capability succeeds does the command
   atomically rename the scratch directory's per-resource `snapshot.json`/
   `diff.json` into place and append each resource's `history/NNN-diff.json`.
5. The already-completed Git-side canonical patch capture from step 1 is
   **not** rolled back if step 3 fails — it was independently valid and
   authoritative before `--resources` began. `record` exits non-zero overall
   (surfacing the resource-capture failure) but the canonical patch and
   `record.md` provenance for the Git-side capture remain exactly as they
   would be without `--resources`. The CLI diagnostic explicitly says so:
   "canonical patch recorded; resource capture failed and was not applied
   (see below); rerun `tpatch record <slug> --resources` alone to retry just
   the resource batch."
6. `--dry-run` (combinable with `--resources`) executes steps 2-3 into the
   scratch directory, reports the would-be outcome for every declared
   resource, and then discards the scratch directory without ever reaching
   step 4 — no file under `resources/` changes.
7. `record --resources` never writes to `patch-generations.json`. Resource
   capture is not a patch generation event (§1.1 table row 4); this mirrors
   ADR-024's explicit refusal of "no data-model coupling" for unrelated
   audit-only concerns.

This all-or-nothing batch design is the binding decision; §16 does not treat
it as open.

## 10. Interactions

### 10.1 File claims

Resources and claims are independent, parallel manifests. A path may
simultaneously be an unclaimed ordinary file, a claimed path
(`claims.json`), and/or (if it happens to also be gitignored, which would be
unusual but not forbidden) referenced by an `ignored-file` resource — there is
no cross-validation requiring exclusivity, matching claims' own "two features
may legitimately claim the same file" non-exclusivity stance
(`PRD-feature-file-claims.md` §3.4).

### 10.2 Capture modes / `record`

`--resources` is documented and implemented as an independent post-hoc filter
(§3.6, §9), not a new entry in the `--all`/`--staged`/`--unstaged`/`--auto`/
`--from`/`--commit-range` mutex table (`PRD-record-capture-modes.md` §3.7). It
never changes the bytes of the canonical Git patch.

### 10.3 Patch generations / amend

`patch-generations.json` (ADR-024) is not extended by this PRD. Resource
capture does not create, read, or invalidate any `PatchGeneration` row.
`tpatch feature patch refresh|fixup` (once implemented per
`PRD-feature-patch-amend`) has no interaction with resources; a future PRD
revision may add an optional `resource_ids` reference field to
`PatchGeneration` for cross-audit convenience, but this PRD explicitly defers
that (§13).

### 10.4 `status` / `next` / `land` / `reconcile` / `remove`

- `status`: unaffected. A future optional `--resources` flag on `status` to
  list resource counts is a natural follow-up but out of scope here (mirrors
  `PRD-feature-file-claims.md` §5.3's `status --claims` deferral).
- `next`: unaffected; resources never gate dependency satisfaction or
  lifecycle routing.
- `land`: unaffected; `land`'s trailer block and staged-path guard operate
  purely on Git-tracked paths and are untouched by resource sidecars, which
  live under `.tpatch/features/<slug>/resources/` and are themselves ordinary
  tracked (or gitignored, per repo policy) files, not staged by `land`
  automatically.
- `reconcile`: unaffected. Resource sidecars are not read by any reconcile
  phase; they are not part of the ADR-025 evidence schema and this PRD does
  not reopen that schema.
- `remove` (feature deletion / cascade): deletes `resources.json` and the
  entire `resources/` sidecar tree along with the rest of the feature
  directory, exactly as it already deletes `claims.json` today (no special
  casing required — this is existing whole-directory-removal behavior).

### 10.5 Ignored resources and committed metadata

`resources.json` and the `resources/` sidecar tree are ordinary files under
`.tpatch/features/<slug>/`, which is tracked by default (same as
`claims.json`, `patch-generations.json`, and every other feature artifact).
An `ignored-file` resource's **declaration** (its selector string in
`resources.json`) is committed metadata even though the **referenced file
itself** is gitignored — this is intentional and mirrors how a `.gitignore`
line itself is committed even though the paths it matches are not. The
`snapshot.json` sidecar for an `ignored-file` resource **does** commit the
byte content of an otherwise-ignored file (subject to the §8 redaction/size
gates) — this is the explicit, user-opted-in point of the kind: making an
ignored file's content auditable through the feature record without changing
`.gitignore` or Git's tracking of the original path.

## 11. Backward Compatibility and Migration

- Existing feature directories have no `resources.json`; that is equivalent
  to "no resources declared," exactly as a missing `claims.json` means "no
  claims declared" (`PRD-feature-file-claims.md` §6). All existing commands
  continue to behave identically unless a user explicitly invokes
  `feature resource` verbs or `record --resources`.
  Adding this PRD's schema does not migrate old patches, rewrite
  `status.json`, or re-record any feature.
- No existing manifest (`claims.json`, `patch-generations.json`,
  `status.json`) gains a new field from this PRD.
- `assets/assets_test.go`'s parity guard is unaffected in this planning
  cluster (no code lands); a future Cluster H' implementation must update the
  six shipped skill surfaces to describe the new command group, consistent
  with how Cluster G' updated all six surfaces for `feature unapply`.

## 12. Explicit Deferrals

The following are deliberately out of scope for v1 and are **not** silently
implied by anything above:

1. A fourth (or open/plugin) resource kind beyond `ignored-file`/
   `git-metadata`/`adapter-snapshot`.
2. A third adapter beyond `dolt`/`generic-command` (e.g., a hypothetical
   `sqlite` or `terraform-plan` adapter).
3. Strict/enforcing resource claims (analogous to claims' deferred `strict`
   mode).
4. Any coupling between resource capture and `patch-generations.json`,
   dependency satisfaction, `land` trailers, or reconcile evidence.
5. A repo-level global adapter-enablement config toggle.
6. Provider-assisted (AI) resource selection, redaction judgment, or summary
   generation.
7. Non-Git replay of resource content (WP-006 territory; unresolved and
   unapproved).
8. A `status --resources` summary flag.
9. Committing raw adapter stdin support (reserved in the protocol shape, not
   implemented, §6.3).
10. Cross-feature resource search or indexing (mirrors ADR-027 D9's
    cross-feature isolation stance and the storage-substrate brief's
    "avoid a generic storage abstraction before a second real authoritative
    substrate exists," `docs/state-of-the-art/storage-substrate-and-versioned-data.md`
    §9).

## 13. Implementation Notes (for Cluster H')

- Reuse `safety.EnsureSafeRepoPath` for every path-shaped selector/arg
  (`ignored-file` selector, `git-metadata:attributes`/`index-summary` key,
  `generic-command` `cwd`).
- Reuse `NormalizeClaimPathShape` for `ignored-file` selector normalization
  rather than reimplementing Clean/ToSlash/trailing-slash logic.
- Reuse the exact atomic-write pattern from `SaveClaims`
  (`internal/store/claims.go:294-346`) for `SaveResources`.
- Reuse `gitutil.IsPathIgnored` for the `ignored-file` add-time gate; do not
  reimplement ignore detection.
- The `.git/**`-exclusion guard for `git-metadata` output should be a small,
  reusable predicate sharing the *concept* (not necessarily the exact
  function) of `pathIsGitInternal`
  (`internal/gitutil/gitutil.go:1189-1211`); a future implementer should
  consider extracting a shared helper so both ADR-030's reconcile-derivation
  path and this PRD's resource-diff path call one guard, rather than two
  independently-maintained copies of the same invariant.
- Adapter execution must use `exec.Command` with an explicit `argv` slice —
  never `sh -c` / `cmd /c` string concatenation — for both `dolt` and
  `generic-command`.
- Snapshot files should be written with the same `.tmp` + `fsync` + `rename`
  discipline as every other tpatch atomic writer.
- Six shipped skill surfaces (Claude, Copilot, Copilot Prompt, Cursor,
  Windsurf, Generic) and the parity guard (`assets/assets_test.go`) must be
  updated to describe `feature resource` and `record --resources`, mirroring
  how Cluster G' updated all six surfaces for `feature unapply`.

## 14. Acceptance Criteria

1. `tpatch feature resource add <slug> --kind ignored-file <path>` writes
   `.tpatch/features/<slug>/resources.json` with a `res_<12hex>` ID, and
   refuses (exit 3) when `<path>` is not currently ignored per
   `gitutil.IsPathIgnored`.
2. `tpatch feature resource add <slug> --kind git-metadata <view>` refuses
   (exit 2) for any `<view>` outside `{refs, attributes, index-summary,
   config}`, and refuses (exit 3) for a disallowed `config` key even when
   `view=config` is otherwise valid.
3. `tpatch feature resource add <slug> --kind adapter-snapshot --adapter
   <name>` refuses (exit 2) for any `<name>` outside `{dolt, generic-command}`.
4. Adding the same normalized `(kind, selector, adapter, capability, args)`
   tuple twice is idempotent (no duplicate row, no error).
5. `feature resource list` prints stable human output; `--json` emits a
   manifest stable-sorted by `resource_id` with no wall-clock timestamp
   field anywhere.
6. `feature resource remove` accepts a full or `>=7`-char unambiguous
   `resource_id` prefix or the exact normalized selector, and deletes both
   the manifest row and the resource's entire `resources/<id>/` sidecar
   directory (current + history).
7. `feature resource clear` removes all resources and all sidecar
   directories for the feature.
8. `feature resource diff` with no declared resources for the feature
   succeeds trivially (prints "no resources declared") rather than erroring,
   mirroring `feature claim list`'s empty-state behavior.
9. `feature resource diff --resource <id>` runs only that resource's
   capability; omitting `--resource` runs every declared resource.
10. `feature resource diff --dry-run` never writes any file under
    `resources/`.
11. A `git-metadata` or `adapter-snapshot` capture whose output (after §6.8
    normalization) references a `.git/**` path in any header/row is refused
    (exit 3) at the store-write boundary, never silently persisted — this is
    testable by constructing a synthetic adapter/view output containing a
    `.git/`-prefixed path and asserting refusal.
12. `dolt`/`generic-command` execution never invokes a shell; a resource
    whose `args.args` contains shell metacharacters (`;`, `|`, `` ` ``,
    `$(...)`) executes those characters as literal argv content, never as
    shell syntax — testable by asserting the literal string appears
    unmodified in the captured (redaction-passing) snapshot rather than
    having side effects.
13. An adapter invocation exceeding its timeout is classified `timeout` and
    the batch (§9) is refused without writing any sidecar file for that
    invocation.
14. An adapter invocation producing more than 5 MiB combined stdout is
    classified `output-too-large` and discarded (not truncated-and-kept).
15. An adapter invocation whose captured output trips the ADR-027 D3
    redaction scan is classified `redaction-refused`, and no snapshot/diff
    file is written for that resource.
16. `record <slug> --resources` on a feature with zero declared resources
    refuses (exit 1) rather than silently succeeding as a no-op.
17. `record <slug> --resources` where every declared resource's capability
    succeeds atomically updates every resource's `snapshot.json`/`diff.json`
    and appends `history/NNN-diff.json` for each.
18. `record <slug> --resources` where any one declared resource's capability
    fails updates **zero** resource sidecar files (all-or-nothing), while the
    Git-side canonical patch capture from the same invocation remains intact
    and unaffected.
19. `record <slug> --resources --dry-run` never writes any sidecar file,
    regardless of individual capability outcomes, and reports the would-be
    outcome for every declared resource.
20. `record --resources` never writes to `patch-generations.json`.
21. Diff envelope `added`/`removed`/`changed` arrays are always present and
    never `null`, even when empty.
22. A `dolt`/`generic-command` capability whose output is inherently
    unordered (e.g. `dolt table-diff`, `git-metadata:refs`) produces
    byte-identical `normalized_sha256` across repeated invocations against
    unchanged underlying state, even if the backend's own row ordering is
    non-deterministic.
23. An `ignored-file` resource whose content exceeds 5 MiB is refused at
    `add` time (exit 3), not silently truncated.
24. An `ignored-file` resource whose content is detected as binary produces a
    `changed` entry with `change_kind: "binary"` and no line-level diff
    detail.
25. A resource whose `env` arg names an environment variable never persists
    that variable's *value* to any manifest, snapshot, or diff file — only
    the name is ever written to disk.
26. No `.git/**` byte content appears in any resource sidecar file under any
    of the three kinds, under any test scenario, including adversarial ones
    (e.g., a `generic-command` resource whose declared command happens to
    `cat` a file under `.git/` — the store-write boundary guard (AC 11) must
    still refuse it even though the *selector itself* is not path-shaped).
27. `feature resource add`/`remove`/`clear`/`diff` on a non-existent `<slug>`
    refuses with the same "no such feature: X" diagnostic shape as
    `feature claim` (`internal/cli/feature_claim.go` `FeatureExists` guard).
28. Existing `record`, `reconcile`, `apply`, `land`, `verify`, and
    `feature claim`/`feature deps`/`feature patch`/`feature unapply` behavior
    is byte-for-byte unchanged when no `feature resource` verb or
    `--resources` flag is used.
29. Tests cover: add/list/remove/clear for all three kinds, duplicate add
    idempotency, invalid kind/adapter/capability rejection (exit 2),
    not-ignored-path rejection for `ignored-file` (exit 3), disallowed
    `config` key rejection (exit 3), `.git/**`-reference refusal at the
    store-write boundary (exit 3), shell-injection-safety for
    `generic-command`, timeout/output-cap/redaction classification, and the
    full `record --resources` all-or-nothing transaction (success, one-of-N
    failure, dry-run).
30. Docs (`docs/feature-layout.md`, `docs/record.md`, `SPEC.md`) are updated
    by the implementation cluster to describe the new manifest, sidecar
    layout, and command surface, following this PRD's exact shapes.

## 15. Open Questions

The following are genuinely deferred design points that do **not** block
Cluster H' from implementing this PRD as written — each has a locked v1
default; the question is only whether a *future* PRD revision should change
it:

1. Should a future revision add a fourth resource kind (e.g. a
   `terraform-plan` or `sqlite` adapter capability) once real usage data
   exists? (v1 default: no; kinds/adapters are closed per §5/§6.)
2. Should `status` gain an optional `--resources` count/summary flag? (v1
   default: no; `feature resource list` is sufficient, mirroring
   `PRD-feature-file-claims.md` §5.3's identical deferral for claims.)
3. Should `PatchGeneration` gain an optional `resource_ids` cross-reference
   field once resource capture has shipped and proven stable? (v1 default:
   no coupling, per §10.3.)
4. Should the `dolt` adapter's timeout become user-configurable in a future
   revision if real schema/table diffs prove slower than 30s in practice? (v1
   default: fixed 30s, no flag.)

## 16. Disputes

None logged.
