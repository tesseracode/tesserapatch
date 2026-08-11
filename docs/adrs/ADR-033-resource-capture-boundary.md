# ADR-033 — Resource Capture Boundary

**Status**: Proposed
**Date**: 2026-08-10 (Cluster H rev-0 planning)
**Author**: Cluster H rev-0 planning agent
**Related**:
- [SPEC.md](../../SPEC.md) — feature lifecycle state table and CLI surface
- [docs/feature-layout.md](../feature-layout.md) — canonical vs. audit-trail artifact map (D2/D9 precedent)
- [docs/record.md](../record.md) — `record` capture boundary
- [docs/prds/PRD-feature-file-claims.md](../prds/PRD-feature-file-claims.md) — shipped v1 advisory path claims (D1/D3 precedent)
- [docs/prds/PRD-record-capture-modes.md](../prds/PRD-record-capture-modes.md) — shipped v1 capture-mode mutex vocabulary
- [docs/adrs/ADR-024-patch-generation-manifest-boundary.md](ADR-024-patch-generation-manifest-boundary.md) — content-addressed ID precedent (`pg_<12hex>`), no-data-model-coupling precedent
- [docs/adrs/ADR-027-capture-context-privacy-boundary.md](ADR-027-capture-context-privacy-boundary.md) — storage lanes, redaction-as-precondition, default-off new-surface rule; binding on every decision below
- [docs/adrs/ADR-030-multi-slug-reconcile-derivation-mode.md](ADR-030-multi-slug-reconcile-derivation-mode.md) — `.git/**` two-layer exclusion precedent (D3/D4 in that ADR)
- [docs/adrs/ADR-032-feature-unapply-state-boundary.md](ADR-032-feature-unapply-state-boundary.md) — sibling planning-cluster format precedent this ADR follows
- [docs/whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md](../whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md) — Git-first posture (Status: Exploring, not accepted)
- [docs/state-of-the-art/storage-substrate-and-versioned-data.md](../state-of-the-art/storage-substrate-and-versioned-data.md) — Dolt research brief §3, "what tpatch should avoid" §9
- [docs/prds/PRD-feature-resource-claims-and-capture-adapters.md](../prds/PRD-feature-resource-claims-and-capture-adapters.md) — the paper design this ADR gates

---

## Purpose

This ADR locks the implementation-blocking design decisions for the feature
resource layer (`tpatch feature resource add|list|remove|clear|diff` and
`tpatch record --resources`) before any code lands. The PRD
(`PRD-feature-resource-claims-and-capture-adapters.md`) may be reviewed as
paper design before this ADR is accepted; implementation must not start until
this ADR reaches **Accepted** status via the standard three-way review
workflow. No code, schema, CLI behavior, or asset text changes in Cluster H;
implementation is a separately dispatched Cluster H'.

**Scope**: this ADR covers exactly the ten decision points below (D1–D10):
manifest placement/reuse, authority boundary, resource ID/selector
normalization, the ignored-file/privacy boundary, the logical Git metadata
allowlist, the adapter discovery/execution/sandbox contract, Dolt's v1 scope,
`record --resources`'s transaction/failure policy, the snapshot/diff wire
schema, and generation/amend/remove lifecycle plus backward compatibility. It
does **not** reopen WP-006's Git-first-vs-non-Git question, does not extend
`patch-generations.json`, and does not change `claims.json`'s existing v1
contract.

---

## 0. Claims Audit

| Claim | Evidence |
|---|---|
| `claims.json` `ClaimKind` is a closed switch; only `"path"` is writable in v1, and `"glob"`/`"symbol"`/`"anchor"` are reserved-but-rejected at the input boundary. | `internal/store/claims.go:38-46`, `internal/store/claims.go:236-246` (`ValidateClaimKindInput`) |
| Claim IDs (`ComputeClaimID`) are a bare 12-hex SHA-256 prefix over NUL-separated `(feature, kind, value, mode)` — no `cl_`-style prefix letter. | `internal/store/claims.go:98-111` |
| `claims.json` is written atomically (`.tmp` + `fsync` + `rename`) and stable-sorted by `claim_id` on load and save. | `internal/store/claims.go:294-346` (`SaveClaims`), `internal/store/claims.go:348-355` (`sortClaims`) |
| `NormalizeClaimPath`/`NormalizeClaimPathShape` implement Clean/ToSlash/trailing-slash-for-directories normalization plus `safety.EnsureSafeRepoPath` symlink-aware validation and installed-skill-surface rejection. | `internal/store/claims.go:139-234`, `internal/safety/safety.go:12` |
| `record` capture-mode dispatch (`--all`/`--staged`/`--unstaged`/`--auto`/`--from`/`--commit-range`/`--claimed-only`) is entirely `git diff`/`git apply` based; there is no committed-range or worktree mode that operates on non-Git-object content. | `internal/cli/record_capture_modes.go:29-186`, `internal/gitutil/capture_modes.go:137-360` |
| `patch-generations.json` (`GenerationCapture`) records only `mode`/`pathspecs`/`claim_ids` as capture provenance for a Git patch; it has no field for non-Git resource identity today. | `internal/store/patch_generations.go:63-67` |
| Patch generation identity (`ComputeGenerationID`) is a `pg_<12hex>` SHA-256 prefix over NUL-separated identity fields, following ADR-024's content-addressed, no-wall-clock-timestamp precedent. | `internal/store/patch_generations.go:254-276` |
| `gitutil` already implements a `.git/**`-reference detector (`pathIsGitInternal`) used to strip diff stanzas that reference repo-internal paths, as a defense-in-depth pattern for reconcile's derived diffs. | `internal/gitutil/gitutil.go:1051-1097`, `internal/gitutil/gitutil.go:1189-1211` |
| ADR-030 independently locked a two-layer `.git/**` exclusion invariant — diff-derivation boundary (D3) plus store-write boundary (D4) — for an unrelated reconcile defect; this ADR must not reopen that hole from the new resource-capture direction. | `docs/adrs/ADR-030-multi-slug-reconcile-derivation-mode.md` D3/D4 |
| `gitutil.IsPathIgnored` is a deterministic `git check-ignore -q --no-index` wrapper returning ignored/not-ignored/`ErrGitUnavailable` as distinct outcomes. | `internal/gitutil/ignore.go:37-75` |
| `docs/feature-layout.md` documents an established canonical-vs-audit-trail pattern: one always-current overwritten file (`artifacts/post-apply.patch`) plus an append-only numbered `patches/NNN-*.patch` history, explicitly framed as "audit trail only, not replay input." | `docs/feature-layout.md` "Canonical vs. audit trail" section |
| ADR-027 D1 locks exactly two storage lanes (committed summary, local private buffer); D3 makes redaction a write precondition and a hard failure on refusal; D7 makes every new capture surface default-off until a downstream PRD proves it safe; D10 forbids external upload except the narrow provider-assisted carve-out. | `docs/adrs/ADR-027-capture-context-privacy-boundary.md` D1, D3, D7, D10 |
| The storage-substrate research brief explicitly lists "a repo-local authoritative Dolt or SQLite database" and "a tpatch-native VCS for non-Git mode" under "What tpatch should avoid," and frames Dolt purely as a design-reference/possible-future-server-aggregation technology, never an embeddable dependency. | `docs/state-of-the-art/storage-substrate-and-versioned-data.md` §9, §3.6, §3.8 |
| WP-006 is `Status: Exploring` (not accepted) and explicitly recommends Git-first with no tpatch-native non-Git substrate absent a dedicated future whitepaper+ADR. | `docs/whitepapers/WP-006-tpatch-substrate-and-non-git-mode.md` header, §4 Option E, §6 |
| tpatch's exit-code convention: `1` generic/internal error (default `Execute()` fallback), `2` = `exitValidation` (CLI input/shape errors), `3` = `exitStateRefus` (state/policy refusals); `*ExitCodeError` is the carrier type. | `internal/cli/reject.go:46-47`, `internal/cli/exit_error.go:5-42`, `internal/cli/cobra.go:32-41` |
| `feature` is the established noun-scoped home for per-feature management verbs (`deps`, `claim`, `patch`, `unapply`); its own doc comment states the namespace "is reserved for future per-feature management surfaces... so we don't keep flat-listing new top-level commands." | `internal/cli/feature_deps.go:39-51` (`featureCmd`) |
| `Store` exposes deterministic per-feature path helpers (`featureDir`, `featureArtifactsDir`, `featureStatusPath`) that every new per-feature artifact path composes with. | `internal/store/store.go:770-782` |

---

## D1: Manifest placement — new `resources.json` vs. extending `claims.json`

### Question

Should feature resources be persisted as new rows inside the existing
`claims.json` (e.g. by adding new `ClaimKind` values), or as a wholly separate
manifest file?

### Alternatives

**Alternative 1 (extend `claims.json`)**: Add new `ClaimKind` values
(`ignored-file`, `git-metadata`, `adapter-snapshot`) to the existing
`Claim` struct and reuse `claims.json`.

Pros:
- One manifest file per feature instead of two.
- Reuses `LoadClaims`/`SaveClaims` verbatim.

Cons:
- `ClaimKindGlob`/`ClaimKindSymbol`/`ClaimKindAnchor` are marked `// reserved`
  at their declaration (`internal/store/claims.go:41-43`) and
  `ValidateClaimKindInput` explicitly rejects them with `"claim kind %q is
  reserved for a future PRD and not accepted in v1"`
  (`internal/store/claims.go:239-240`) — documented, reserved extensions of
  the *same* Git-diffable-path universe, in the context of path/AST scoping.
  Repurposing the kind enum
  for a fundamentally different universe (adapter-executed snapshots, logical
  Git metadata, ignored-file byte content) would silently break the
  documented meaning of `ClaimKind` and the guarantee `NormalizeClaimPath`
  provides (a Git-diffable, repo-relative path) for every existing consumer
  of `claims.json`, including `--claimed-only`'s path-intersection logic
  (`internal/cli/record_capture_modes.go:147-186`), which assumes every claim
  value is a pathspec `git diff`/`git apply` can consume.
- `Claim.Value` has no field for adapter identity, capability, or argument
  maps; adding them to the shared struct would make every `path`-kind claim
  carry unused, always-empty `adapter`/`capability`/`args` fields, bloating
  the schema every existing consumer has to skip past.
- A single shared file conflates two different write-boundary guarantees:
  claims are advisory scope over content Git already diffs; resources are
  sidecar snapshots over content Git explicitly does not diff (or does not
  represent as a file at all). Mixing them raises the risk that a future
  maintainer treats a resource row as claim-like (i.e., assumes
  `--claimed-only` should silently pick it up), which would be wrong per D2.

**Alternative 2 (new `resources.json`) — chosen**: A new manifest file,
sibling to `claims.json`, at `.tpatch/features/<slug>/resources.json`.

Pros:
- Preserves `claims.json`'s existing, reviewed, shipped v1 contract exactly
  as documented — zero risk of an accidental behavior change to a shipped
  primitive.
- Lets the resource schema carry fields (`adapter`, `capability`, `args`)
  that have no meaning for a path claim, without polluting the claim schema.
- Makes the "resources are sidecar audit, not scope-of-record" distinction a
  structural fact (two files, two purposes) rather than a convention buried
  inside one file's row-level `kind` discriminator.
- Reuses the *conventions* from `claims.json` (deterministic ID derivation,
  atomic write, stable sort, missing-file-is-empty-manifest) without reusing
  the *file* — this satisfies the binding "extend, don't duplicate" intent
  from the ROADMAP registration by reusing patterns, not by cramming
  unrelated content into one artifact.

Cons:
- One more file to read per feature; negligible I/O cost given existing
  per-feature files already number half a dozen (`status.json`,
  `claims.json`, `patch-generations.json`, `request.md`, etc., per
  `docs/feature-layout.md`).
- Requires a second `Load*`/`Save*` pair in `internal/store`, mirroring
  `LoadClaims`/`SaveClaims` almost line-for-line.

### Decision: **Alternative 2**

`resources.json` is a new manifest, structurally independent of
`claims.json`. `internal/store` gains `LoadResources`/`SaveResources`
mirroring `LoadClaims`/`SaveClaims`'s atomic-write and stable-sort
conventions, but as separate functions over a separate `ResourcesManifest`
type. `ClaimKind`'s reserved values (`glob`/`symbol`/`anchor`) remain
reserved for path/AST-scoping PRDs only, and are not touched by this ADR.

Exact wire shape (byte-identical to PRD §4 — the definitive schema
reference; this ADR does not maintain a second copy of field-rule prose):

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

### Consequences

- Two manifest files per feature that has both claims and resources
  declared; `feature-layout.md` gains a new row for `resources.json` and the
  `resources/` sidecar tree.
- No risk to `claims.json`'s shipped v1 behavior; `assets/assets_test.go`'s
  claims-related parity anchors are unaffected.
- A future PRD that wants symbol/anchor path-claims still has the reserved
  `ClaimKind` slots available, uncontaminated by this ADR.

---

## D2: Authority boundary — resource diffs are sidecar audit artifacts, never canonical patch/lifecycle truth

### Question

Do resource snapshots/diffs participate in `artifacts/post-apply.patch`,
`patch-generations.json`, dependency satisfaction, `apply`/`reconcile`/`land`
gating, or `status.json` lifecycle state — or are they strictly sidecar audit
artifacts that no lifecycle command reads for correctness?

### Alternatives

**Alternative 1**: Merge resource diff bytes into `artifacts/post-apply.patch`
so `git apply` replays the feature's full context including declared
resources.

Pros:
- Single replay artifact continues to be "the" feature diff.

Cons:
- Git-apply machinery cannot consume Dolt table diffs, resolved Git-metadata
  views, or arbitrary adapter output — none of these are Git unified-diff
  hunks. Merging them would require inventing a non-standard patch dialect
  `git apply` cannot parse, defeating the entire purpose of `post-apply.patch`
  remaining a byte-identical, `git apply`-replayable artifact
  (`docs/feature-layout.md` "use this one" framing).
- An `ignored-file` resource's content did not come from the working tree
  Git diffed against `HEAD` (it is explicitly excluded from that diff by
  `.gitignore`); folding it into the canonical patch would silently change
  what "replay this patch" means for every consumer that currently assumes
  `post-apply.patch` is a plain Git diff.
- Violates the binding task constraint: "Resource diffs are sidecar audit
  artifacts in v1, not post-apply.patch, lifecycle truth, or patch-generation
  content authority."

**Alternative 2 (chosen) — resources are pure sidecar audit, zero lifecycle coupling**:
Resource snapshots/diffs live entirely under
`.tpatch/features/<slug>/resources/`, are never read by `apply`, `reconcile`,
`land`, `verify`, dependency-gate evaluation, or any `status.json` field, and
never modify `artifacts/post-apply.patch` or `patch-generations.json`.

Pros:
- Zero risk to any existing lifecycle command's correctness. This ADR touches
  no shipped state machine, no shipped gate, no shipped audit schema.
- Matches the storage-substrate brief's explicit stance that a new
  authoritative substrate is not justified without proven need
  (`docs/state-of-the-art/storage-substrate-and-versioned-data.md` §9).
- Keeps the review surface small and mechanically bounded: a reviewer can
  verify "no lifecycle command reads `resources.json`/`resources/`" as a
  single grep-able invariant.

Cons:
- Resource content cannot (in v1) block `apply`/`land`/`reconcile` even if a
  declared resource is stale or missing — this is accepted as a deliberate v1
  scope limit (PRD §12 explicit deferral), not an oversight.

### Decision: **Alternative 2**

Resources are strictly sidecar audit artifacts. No lifecycle command in
tpatch reads, writes, or is gated by `resources.json` or the `resources/`
sidecar tree. `feature resource diff`/`record --resources` are the only two
verbs that ever touch these files.

### Consequences

- A future PRD wanting resource-aware gating (e.g., "refuse `land` if a
  declared resource's snapshot is stale") must reopen this decision
  explicitly; it is not implied by anything in this cluster.
- Reviewers of Cluster H' can verify this invariant mechanically: no
  reference to `resources.json`/`ResourcesManifest`/`resources/` should
  appear in `internal/workflow/reconcile.go`, `internal/cli/land.go`, the
  dependency gate, or `verify.go`.

---

## D3: Resource ID derivation and selector normalization

### Question

How is `resource_id` derived, and what normalization applies to each kind's
selector before hashing?

### Alternatives

**Alternative 1**: Reuse `ComputeClaimID`'s exact tuple shape
`(feature, kind, value, mode)` bare-12-hex scheme for resources.

Pros: identical code path to claims.

Cons: resources have no `mode` field (claims' advisory/strict axis has no
resource analogue) and *do* have adapter/capability/args fields claims never
had. Forcing the claim tuple shape would either drop adapter identity from
resource-ID derivation (two different adapter-snapshot resources with the
same selector but different `args` would collide on the same ID) or require
a shape claims never needed.

**Alternative 2 (chosen)**: A new `res_<12hex>` scheme, following
`ComputeGenerationID`'s established pattern of hashing an ordered,
NUL-separated tuple with pre-sorted collection fields.

`resource_id = "res_" + hex(sha256(feature ‖ 0 ‖ kind ‖ 0 ‖ selector ‖ 0 ‖ adapter ‖ 0 ‖ capability ‖ 0 ‖ canonical_args))[:12]`

where `canonical_args` is the `args` map rendered as `key=value` pairs joined
by `\n`, sorted lexicographically by key before joining (mirroring
`ComputeGenerationID`'s `sort.Strings(pathspecs)` /
`sort.Strings(claimIDs)` pre-sort discipline,
`internal/store/patch_generations.go:257-259`).

Pros:
- Correctly distinguishes two `adapter-snapshot` resources with the same
  `(adapter, capability)` but different `args` (e.g. two `dolt schema-diff`
  resources for two different tables).
- Uses the `pg_`-style prefixed-ID convention already established for
  generations, reconcile evidence (`re_`/`rr_`), and reserved by ADR-027 for
  context artifacts (`ctx_`/`cs_`/`ce_`) — `res_` joins that family rather
  than reviving claims' unprefixed scheme, keeping ID-prefix-based type
  disambiguation possible across `.tpatch/` artifacts.
- No wall-clock timestamp anywhere in the hash input, matching every existing
  ID scheme.

Cons: one more bespoke ID-derivation function to maintain, distinct from both
`ComputeClaimID` and `ComputeGenerationID`. Accepted — the two existing
schemes each encode assumptions (claim's `mode` axis; generation's patch/
recipe/commit tuple) that do not fit resources cleanly.

**Selector normalization per kind** (locked, not left to Cluster H'
discretion):

- `ignored-file`: reuse `NormalizeClaimPathShape` verbatim
  (`internal/store/claims.go:139-177`) for Clean/ToSlash/trailing-slash
  normalization, then apply the additional `ignored-file`-specific gates in
  PRD §5.1 (not-ignored refusal, `.git/`-prefix refusal) that
  `NormalizeClaimPath`'s installed-skill-surface list does not itself cover
  as a *reason* (though the prefix list already blocks `.tpatch/` literally).
- `git-metadata`: selector is `<view>[:<key>]`; `<view>` validated against
  the closed table in D5; `<key>` validated per-view (path-shaped keys reuse
  `safety.EnsureSafeRepoPath`; `config` keys validated against the fixed
  allowlist regex set).
- `adapter-snapshot`: selector is `<adapter>:<capability>[:<discriminator>]`;
  `<adapter>`/`<capability>` validated against the closed tables in D6/D7;
  `<discriminator>` is a free-form label with no execution meaning (pure
  human-readability + ID-uniqueness aid) and is normalized by trimming
  whitespace and rejecting embedded NUL/newline bytes only.

### Decision: **Alternative 2**, with the per-kind normalization rules above
locked as part of this decision (not deferred).

### Consequences

- `internal/store` gains `ComputeResourceID` as a new, standalone function;
  it does not call or extend `ComputeClaimID`/`ComputeGenerationID`.
- Resource IDs are stable across repeated `add` invocations with identical
  normalized inputs (idempotency, PRD AC-4), and distinct across any change
  to selector, adapter, capability, or args (correctness for AC-9/AC-16
  per-resource addressing).

---

## D4: Ignored-file / privacy boundary

### Question

What gate must an `ignored-file` selector pass before `add` accepts it, and
what privacy contract governs its captured content?

### Alternatives

**Alternative 1**: Accept any repo-relative path as an `ignored-file`
resource, ignored or not, and let `record --resources`/`diff` simply read
whatever bytes are on disk.

Pros: simplest implementation; no extra gate.

Cons: this would let `ignored-file` become a second, unintended way to claim
an ordinary Git-tracked file — duplicating `claims.json`'s job with no
`--claimed-only`-style intersection logic, no advisory-vs-resource
distinction visible to the user, and no signal that "this resource kind
exists specifically because Git will not diff it." It also weakens the
task's explicit binding constraint that "Ignored files are NEVER swept
implicitly; selectors are explicit opt-in" — accepting non-ignored paths
under this kind would blur exactly the line that constraint is drawing.

**Alternative 2 (chosen)**: `add` refuses (exit 3) unless
`gitutil.IsPathIgnored` reports the path is currently ignored at add-time;
`ErrGitUnavailable` is treated as fail-closed refusal, not silent
acceptance.

Pros:
- Makes the resource kind's purpose self-enforcing: a user cannot
  accidentally declare an ordinary tracked file as an `ignored-file`
  resource; the CLI actively points them at `feature claim add` instead.
- Fail-closed on `ErrGitUnavailable` matches the existing precedent
  (`internal/gitutil/ignore.go:59-75` doc comment's own stated contract) and
  ADR-027 D7's "new capture surface defaults to refusing when unsafe" spirit.
- Every captured byte from this kind additionally passes the ADR-027 D3
  redaction contract before persisting (hard failure on a hit, no
  scrub-and-continue) — inherited verbatim, not re-derived.

Cons: a resource that starts ignored and later becomes tracked (e.g. a
`.gitignore` rule is removed) is not automatically detected as stale in v1;
this is an accepted, documented v1 gap (see Negative Consequences Summary),
not a silent one.

### Decision: **Alternative 2**

`ignored-file` `add` requires `gitutil.IsPathIgnored(repoRoot, path) == true`
at add-time; `git-unavailable` is a refusal, not an acceptance. Every
captured snapshot passes ADR-027 D3 redaction as a write precondition, with
size (5 MiB) and binary-detection handling per PRD §8.

### Consequences

- `add` has a genuine dependency on `git check-ignore` being answerable,
  meaning `ignored-file` resources cannot be declared in a non-Git or
  metadata-only-substrate workspace (WP-006 territory) — consistent with
  every other Git-dependent tpatch command, not a new limitation.
- A `.gitignore` rule change that un-ignores a previously-declared resource's
  path is not detected automatically; `diff`/`record --resources` will still
  read and snapshot whatever bytes are on disk (the resource itself does not
  re-validate ignored-ness on every read, only at `add` time) — flagged as a
  known limitation, not silently hidden.

---

## D5: Logical Git metadata allowlist

### Question

Which Git-derived facts, if any, may a `git-metadata` resource expose, and
how is raw `.git/**` byte content categorically excluded?

### Alternatives

**Alternative 1**: Allow arbitrary `git <subcommand>` invocation as the
selector, trusting the user to avoid dangerous subcommands.

Pros: maximally flexible; no allowlist to maintain.

Cons: directly contradicts the binding constraint "Raw `.git/**` content
remains forbidden... Git metadata must be logical, allowlisted, normalized
views." An arbitrary `git cat-file`/`git show` invocation could dereference
raw object content indistinguishable from arbitrary file content, defeating
the entire purpose of a bounded "logical view" contract. It also reopens
exactly the failure class ADR-030 had to fix (`.git/**` leaking into an
artifact through an underspecified extraction path).

**Alternative 2 (chosen)**: A closed, four-entry view allowlist — `refs`,
`attributes`, `index-summary`, `config` — each backed by one fixed, safe Git
plumbing/porcelain command shape, with `config` additionally gated by a
key-pattern allowlist that explicitly excludes credential- and
URL-embedded-token-shaped keys.

Pros:
- Every view is a *resolved, logical* fact (ref name + OID; resolved
  attribute pairs; index summary rows; resolved config values) — never a
  dereferenced blob, never a raw `.git/` filesystem read.
- The `config` view's key-pattern allowlist directly prevents the one
  realistic secret-leak vector in Git metadata (`credential.helper`,
  `http.extraheader`, embedded-userinfo remote URLs) without needing a
  general-purpose secret scanner to catch it after the fact — though the
  general ADR-027 D3 redaction scan still runs as defense in depth.
- Closed enumeration means the store-write boundary guard (D2/PRD AC-11) has
  a small, auditable set of expected output shapes to validate against,
  rather than an unbounded surface.

Cons: a user who wants a fifth logical view (e.g. `git log --oneline` for a
path) cannot get it in v1 without a PRD revision. Accepted — matches the
"closed in v1" decision already locked for resource kinds generally (PRD §5,
Non-goal 1).

### Decision: **Alternative 2**

The `git-metadata` view allowlist is exactly `{refs, attributes,
index-summary, config}` as specified in PRD §5.2, with `config`'s key
pattern further restricted to `{core.*, remote.*.url, branch.*.merge,
user.name, user.email}` and an explicit hard exclusion for
`credential.*`/`http.*.extraheader`/`url.*.insteadof`/any resolved value
matching an embedded-userinfo URL shape (`scheme://user:pass@host`). Any
`git-metadata` output whose extracted path/header data matches the
`pathIsGitInternal`-class `.git/**` predicate is refused at the store-write
boundary regardless of which view produced it (defense in depth mirroring
ADR-030 D3/D4's two-layer pattern).

### Consequences

- Cluster H' should extract or reuse a `.git/**`-reference guard shared in
  spirit with `pathIsGitInternal` (`internal/gitutil/gitutil.go:1189-1211`)
  so both ADR-030's reconcile-derivation path and this resource-diff path
  enforce the same invariant, ideally via one shared predicate rather than
  two independently-maintained copies (PRD §13 implementation note).
- A fifth view or a wider `config` allowlist requires a future PRD revision,
  not a flag or config toggle.

---

## D6: Adapter discovery / execution / sandbox contract

### Question

How are external adapter binaries located, invoked, sandboxed, and bounded,
such that no shell injection, secret leakage, or unbounded resource
consumption is possible?

### Alternatives

**Alternative 1**: Invoke adapters via a shell (`sh -c "<user-supplied
string>"`), letting users freely compose pipelines.

Pros: maximal expressiveness (pipes, redirects, globs).

Cons: a direct shell-injection vector — any resource `args` value containing
shell metacharacters becomes executable syntax, not literal data, violating
the binding constraint "no shell injection." Rejected outright.

**Alternative 2 (chosen)**: Direct `exec.Command(path, argv...)` invocation
with a fixed, capability-specific argv template plus the resource's declared
`args`; a Go-native whitespace/quote tokenizer (not a shell) splits any
free-form `args.args` string into literal argv elements; environment is an
explicit name-only allowlist resolved fresh at execution time; stdin is
closed by default; cwd defaults to repo root and is validated with
`safety.EnsureSafeRepoPath`; hard timeout and output-size caps apply to every
invocation; adapter identity (`exec.LookPath` result, version-probe output)
is recorded as informational provenance only.

Pros:
- Closes the shell-injection vector categorically: shell metacharacters in
  any `args` value are inert data to `exec.Command`, never interpreted.
- Environment allowlist-by-name plus fresh-lookup-at-execution-time satisfies
  ADR-027 D2's secret-by-reference rule: only variable *names* are ever
  persisted, never values.
- Timeout/output caps make every adapter invocation bounded and prevent a
  misbehaving or malicious external tool from hanging `record` indefinitely
  or exhausting disk with an unbounded capture.
- Adapter names are a closed, compiled-in set (D7); "discovered at runtime"
  is scoped to binary-presence checking (`exec.LookPath`), not to loading
  unknown third-party plugin code — this closes the "arbitrary discovered
  binary" attack surface the task's binding constraints implicitly warn
  against by requiring "no shell injection" and "no secret values."

Cons: less expressive than a shell (no pipes/redirects within a single
adapter invocation) — accepted; `generic-command`'s `args.args` can still
name a wrapper script the user controls if genuine pipeline composition is
needed, and that script is the user's own responsibility, not tpatch's
injection surface.

### Decision: **Alternative 2**, exactly as specified in PRD §6 (discovery,
identity, args/env/stdin, cwd, timeout, output limits, exit/error taxonomy,
deterministic normalization).

### Consequences

- Every adapter invocation is auditable: recorded executable path, version
  probe, outcome classification, raw/normalized hashes.
- `generic-command`'s tokenizer must be implemented as a small, dedicated,
  test-covered Go function (not `strings.Fields`, which does not handle
  quoted segments) — flagged as an implementation note, not a design
  decision left open.
- The exit/error taxonomy (PRD §6.7 table) is binding: `adapter-missing`,
  `timeout`, `nonzero-exit`, `output-too-large` classify to exit 1;
  `unsafe-path`, `redaction-refused` classify to exit 3; closed-set
  rejections classify to exit 2.

---

## D7: Dolt's v1 scope — one closed-set adapter, never authoritative storage

### Question

How does Dolt participate in tpatch, given the storage-substrate brief's
explicit rejection of Dolt as a repo-local authoritative store?

### Alternatives

**Alternative 1**: Embed Dolt as a Go library dependency and let tpatch
manage a Dolt database directly (e.g., for schema/table history).

Pros: tightest integration; no external binary dependency.

Cons: directly contradicts the binding constraint "no new core dependency"
and the storage-substrate brief's explicit rejection reasoning — review
mismatch, substrate duplication, commit-coordination risk, dependency
weight, migration burden, scale mismatch, and human-ownership loss
(`docs/state-of-the-art/storage-substrate-and-versioned-data.md` §3.6).
Rejected outright; not a close call.

**Alternative 2 (chosen)**: Dolt participates purely as one of exactly two
closed-set `adapter-snapshot` adapter names (`dolt`, `generic-command`),
discovered at runtime via `exec.LookPath("dolt")`, invoked as a subprocess
for two fixed capabilities (`schema-diff`, `table-diff`), producing a
read-only sidecar snapshot. tpatch never manages a Dolt database, never
writes to one, and never treats Dolt state as authoritative for anything.

Pros:
- Matches the binding constraint exactly: "no new core dependency and no
  database/adapter becomes tpatch authority."
- Directly satisfies the storage-substrate brief's own framing of Dolt as "a
  design reference and possible future server-side aggregation technology"
  rather than an embedded or repo-local authoritative store (§3.8).
- If `dolt` is absent from `PATH`, the feature degrades gracefully (a single
  resource's `diff`/`record --resources` classifies `adapter-missing`
  exit 1) rather than tpatch itself failing to build or run.

Cons: users without `dolt` installed cannot exercise `dolt`-kind resources —
accepted, this is the expected behavior for any optional, runtime-discovered
external tool.

### Decision: **Alternative 2**

Dolt is realized exclusively as the `dolt` adapter name under the
closed-set `adapter-snapshot` resource kind (D6/PRD §6), with exactly two
capabilities (`schema-diff`, `table-diff`), discovered via `exec.LookPath`
at execution time, never embedded as a Go dependency, never granted write
access to any Dolt database by tpatch, and never made authoritative for any
tpatch data.

### Consequences

- `go.mod` gains zero new dependencies from this ADR.
- A repository without Dolt installed sees `dolt`-kind resources as
  declarable (the `resources.json` row persists) but non-executable until
  the binary is present — matching the "optional adapter" framing precisely.
- Any future desire for a third adapter or a deeper Dolt integration
  requires a new PRD revision reopening D6/D7, not a runtime plugin
  mechanism.

---

## D8: `record --resources` transaction and partial-adapter-failure policy

### Question

When `record --resources` runs against multiple declared resources and one
adapter invocation fails, does the batch commit the resources that
succeeded, roll back everything, or something else? Does `record --resources`
ever write to `patch-generations.json`?

### Alternatives

**Alternative 1 (partial commit)**: Persist each resource's
snapshot/diff independently as its capability succeeds; only the failed
resource's sidecar files are left stale/unchanged.

Pros: maximizes forward progress; a single flaky adapter does not block
every other resource's capture.

Cons: produces an internally inconsistent audit trail for a single `record
--resources` invocation — some resources reflect "as of this run," others
reflect an older run, with no single artifact recording which resources
belong to which logical capture event. This directly conflicts with the
audit-trail framing this PRD borrows from `feature-layout.md` (a numbered
snapshot is supposed to represent "what did the feature's resources look
like at this point," not a mix of two different points).

**Alternative 2 (chosen) — all-or-nothing batch, independent of the
already-completed Git-side capture**: Stage every declared resource's
snapshot/diff into a scratch directory; only rename the entire batch into
place if every resource's capability outcome is `ok`. Any single failure
refuses the whole `--resources` batch (no sidecar file changes for any
resource in that invocation), while the Git-side canonical patch capture
that already completed earlier in the same `record` invocation is
**not** rolled back — it was independently valid before `--resources` began.
`record --resources` never writes to `patch-generations.json` under any
outcome.

Pros:
- Produces one coherent, all-succeeded-together audit snapshot per
  successful invocation — matches the "resource diffs are sidecar audit
  artifacts" framing (D2) by keeping the audit trail internally consistent.
- Does not compound the Git-side capture's success with the resource
  batch's failure: a user who only cares about the Git-side canonical patch
  is not blocked by resource capture, and re-running just
  `--resources` alone is a cheap, safe retry (no Git-side re-capture is
  needed or performed).
- `--dry-run` is a strict subset of the same code path (stage into scratch,
  report, discard) — no separate simulation logic to maintain and drift out
  of sync with the real path.
- Never touching `patch-generations.json` avoids reopening ADR-024's closed
  schema (D2/PRD §10.3) and avoids coupling resource-capture reliability to
  patch-generation correctness.

Cons: a single misconfigured resource (e.g. one `dolt` table renamed
upstream) blocks the entire batch's sidecar update until fixed or removed —
accepted; the CLI diagnostic names exactly which resource failed and how to
retry (either fix it, `feature resource remove` it, or run `diff --resource
<other-id>` individually to update just that one outside the `record` batch).

### Decision: **Alternative 2**

All-or-nothing batch commit for `record --resources`; Git-side capture
completion is independent and unaffected by resource-batch outcome; no
`patch-generations.json` write under any outcome; `--dry-run` shares the
staging code path and discards before the final rename.

### Consequences

- Cluster H' implements a scratch-directory-then-atomic-rename pattern for
  the whole resource batch, not per-resource independent atomic writes.
- `feature resource diff --resource <id>` (outside `record`) remains
  per-resource atomic (PRD §3.5) — the all-or-nothing constraint is specific
  to the `record --resources` *batch* entry point, not to `diff`'s
  single-resource invocation, which has no "other resources in the batch" to
  stay consistent with.

---

## D9: Snapshot/diff wire schema

### Question

What is the exact, versioned wire shape for a resource's current snapshot,
current diff, and historical diff-envelope audit trail?

### Alternatives

**Alternative 1**: Store only a raw byte snapshot per resource, with no
structured diff envelope; let `feature resource diff` recompute a diff
on-the-fly by comparing two raw snapshots at read time.

Pros: smaller on-disk footprint (one file instead of two-plus-history per
resource).

Cons: no audit trail of what changed *at diff time* — a user inspecting
`resources/<id>/` days later cannot see "what did the last `diff` run
report" without re-running the adapter, which may no longer be
reproducible (external state may have moved). Loses the append-only
audit-history property every other tpatch artifact class has
(`patches/NNN-*.patch`, `patch-generations.json` rows, ADR-025 evidence
JSONL).

**Alternative 2 (chosen)**: A fixed-envelope JSON diff artifact
(`diff.json`, current + `history/NNN-diff.json`, append-only) referencing a
separate raw `snapshot.json` body, following the exact canonical-vs-audit
pattern already documented for `artifacts/post-apply.patch` vs.
`patches/NNN-*.patch`.

Pros:
- Directly reuses an established, documented, already-understood tpatch
  pattern rather than inventing a new one — reviewers and future
  implementers already know how to reason about "current file always
  overwritten, `history/` append-only, prune down to newest plus current is
  safe" from `feature-layout.md`.
- `added`/`removed`/`changed` structured arrays (never `null`) give
  `--json` consumers (agents, CI) a stable, machine-parseable diff shape
  without needing to parse adapter-specific raw text.
- `raw_sha256`/`normalized_sha256` let an auditor distinguish "real change"
  from "adapter reordered its own output" (PRD §6.8), which a raw-bytes-only
  approach could not express.

Cons: two files per resource (`snapshot.json` + `diff.json`) plus a
`history/` directory — modestly more files than Alternative 1, accepted for
the audit-trail and machine-readability benefits.

### Decision: **Alternative 2**, exactly as specified in PRD §7 (fixed
fields: `version`, `feature`, `resource_id`, `kind`, `adapter`, `capability`,
`executable_path`, `version_probe`, `raw_sha256`, `normalized_sha256`,
`outcome`, `changes.{added,removed,changed}`, `body_ref`). Implemented via
`json.Marshal` on a fixed Go struct (declaration-order field serialization),
mirroring ADR-032 D3's precedent (`docs/adrs/ADR-032-feature-unapply-state-boundary.md`
Implementation Notes item 8) rather than `map[string]interface{}`.

Exact wire shape (byte-identical to PRD §7's `diff.json`/
`history/NNN-diff.json` example):

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

### Consequences

- `changes.added`/`removed`/`changed` must be initialized as `[]T{}`, never
  left as a nil slice that Go's `encoding/json` would render as `null` — a
  mechanically testable invariant (PRD AC-21).
- A non-`ok` outcome still writes an envelope (for auditability of the
  failure itself) but must not overwrite the prior successful
  `snapshot.json`/`diff.json` — this is the same no-partial-overwrite
  invariant as D8, applied at the single-resource-envelope level.

---

## D10: Generation/amend/remove lifecycle and backward compatibility

### Question

Do resources participate in patch-generation versioning or amendment? What
happens to a resource's sidecar history on `remove`? What is the backward
compatibility contract for repositories with no `resources.json`?

### Alternatives

**Alternative 1**: Version resources per patch generation (each
`patch-generations.json` row references the resource state at that
generation), and retain resource sidecar history even after `feature
resource remove`.

Pros: strongest audit trail; a user could reconstruct "what resources existed
at generation N."

Cons: directly reopens ADR-024's closed `patch-generations.json` schema (D2
already forbids this coupling) and requires resource removal to leave orphan
history around indefinitely with no cleanup story — inconsistent with
claims' own no-retained-history-on-remove precedent
(`internal/store/claims.go:434-444`, `RemoveClaim` — a claim removal is a
clean delete, not a tombstone).

**Alternative 2 (chosen)**: No coupling to `patch-generations.json` (per D2/
D8); `feature resource remove`/`clear` fully deletes the manifest row and
the entire `resources/<id>/` sidecar directory (current + history), exactly
mirroring `RemoveClaim`'s clean-delete behavior. A missing `resources.json`
is the empty-manifest state, exactly mirroring `LoadClaims`'s missing-file
handling (`internal/store/claims.go:263-278`). No existing manifest gains a
new field.

Pros:
- Perfectly consistent with the claims precedent already shipped and
  reviewed — no new removal-semantics pattern to design or explain.
- Zero backward-compatibility risk: every existing feature directory
  (having no `resources.json`) behaves exactly as before until a user
  explicitly invokes `feature resource` or `record --resources`.
- Keeps `patch-generations.json`'s schema closed, avoiding a second
  ADR-024 amendment cycle inside this cluster.

Cons: once a resource is removed, its full snapshot/diff history is gone —
accepted, matching claims' own behavior and the PRD's explicit non-goal
that resources are audit-only, not permanent lifecycle record.

### Decision: **Alternative 2**

No `patch-generations.json` coupling. `remove`/`clear` fully delete manifest
rows and their entire sidecar directories, mirroring `RemoveClaim`. Missing
`resources.json` is the empty-manifest state. No existing manifest
(`claims.json`, `status.json`, `patch-generations.json`) gains a new field
from this ADR.

### Consequences

- A future PRD wanting generation-level resource versioning or
  tombstone-on-remove history must reopen this decision explicitly — it is
  not implied by anything shipped here.
- `feature remove`/cascade deletion of a whole feature directory already
  deletes `resources.json` and `resources/` for free (whole-directory
  deletion), requiring no special-case code.

---

## Implementation Notes (for Cluster H')

The following notes are for the implementation cluster that executes after
this ADR is accepted. They are not decisions; they are forward-guidance
collected during planning.

1. **New files**: `internal/store/resources.go` (manifest load/save/
   `ComputeResourceID`, mirroring `internal/store/claims.go`'s shape),
   `internal/cli/feature_resource.go` (CLI verbs, mirroring
   `internal/cli/feature_claim.go`), `internal/gitutil/resource_adapters.go`
   (adapter execution: `dolt`, `generic-command`), `internal/gitutil/
   resource_gitmetadata.go` (the four `git-metadata` views).
2. **Shared `.git/**`-internal guard**: extract or share the predicate
   concept behind `pathIsGitInternal` (`internal/gitutil/gitutil.go:1189-1211`)
   so both ADR-030's reconcile-derivation path and this ADR's
   `git-metadata`/adapter-output store-write boundary call one guard.
   Independently re-deriving the same invariant twice is a latent
   drift risk this note exists to prevent.
3. **`generic-command` tokenizer**: implement a small, dedicated,
   test-covered whitespace/quote tokenizer for `args.args` — do not reuse
   `strings.Fields` (it does not honor quoted segments) and do not shell out
   to split it (that would reintroduce the injection vector D6 closes).
4. **Env allowlist resolution timing**: resolve `env` names to values at the
   moment of `exec.Command` construction, immediately before `cmd.Run()` —
   never earlier, and never persist the resolved value anywhere, including
   in-memory structures that later get logged or serialized.
5. **Redaction integration point**: call the existing ADR-027 D3 redaction
   contract on captured `snapshot.json` bytes *before* the atomic rename in
   D8's batch-commit step, not after — a `redaction-refused` outcome must
   prevent the rename from ever happening for that resource.
6. **Six shipped skill surfaces + parity guard**: Claude, Copilot, Copilot
   Prompt, Cursor, Windsurf, Generic assets and `assets/assets_test.go` need
   a `feature resource` / `record --resources` command-reference update,
   mirroring how Cluster G' updated all six surfaces for `feature unapply`.
7. **`docs/feature-layout.md` update**: add `resources.json` and
   `resources/<resource_id>/{snapshot.json,diff.json,history/}` to the "at a
   glance" tree and the canonical-vs-audit-trail section, using this ADR's
   exact framing (current = always-overwritten, `history/` = append-only,
   never replay input).
8. **Test isolation for adapters**: `dolt`-adapter tests should skip cleanly
   (not fail) when `dolt` is not present on the test-runner `PATH`, mirroring
   the existing `IsGitAvailable`-style skip pattern already used for
   git-dependent test helpers (`internal/gitutil/ignore.go:25-33`).

---

## Negative Consequences Summary

| Decision | What breaks / what's deferred |
|---|---|
| D1 (separate manifest) | Two manifest files per feature with both claims and resources; one more `Load*`/`Save*` pair to maintain. |
| D2 (sidecar-only authority) | Resource staleness never blocks `apply`/`land`/`reconcile`/`verify` in v1; a future PRD must reopen this to add gating. |
| D3 (new `res_<12hex>` scheme) | A third bespoke ID-derivation function alongside `ComputeClaimID`/`ComputeGenerationID`, rather than reuse of either. |
| D4 (must-be-ignored gate) | `ignored-file` resources cannot be declared in a non-Git/metadata-only substrate; a path that later becomes tracked is not auto-detected as stale. |
| D5 (closed 4-view allowlist) | A fifth logical Git view, or a wider `config` key pattern, requires a future PRD revision — not available in v1. |
| D6 (argv-only, no shell) | No pipe/redirect composition inside a single adapter invocation; users needing that must write their own wrapper script. |
| D7 (Dolt as one closed adapter) | No deep Dolt integration (e.g. querying live Dolt SQL); only `schema-diff`/`table-diff` snapshot capabilities exist in v1. |
| D8 (all-or-nothing batch) | One misconfigured resource blocks the whole `--resources` batch's sidecar update until fixed, removed, or run individually via `diff --resource`. |
| D9 (fixed envelope + history) | Schema extension in a future revision requires an envelope `version: 2` migration path, mirroring ADR-032 D3's own accepted cost. |
| D10 (no generation coupling, clean delete) | No "what resources existed at generation N" audit query; `remove` is a full, non-recoverable local delete of sidecar history. |

---

## Test Matrix (1:1 mirror of PRD §14 acceptance criteria + supporting safety rows)

Every PRD §14 acceptance criterion (1–30) has at least one dedicated row
below. Rows without a PRD AC number are supporting safety/determinism rows
that harden the specific decision named, mirroring ADR-032's own pattern of
including safety rows beyond the strict AC count.

| # | PRD AC | Decision | Test scenario | Expected |
|---|---|---|---|---|
| 1 | AC-1 | D4 | `feature resource add <slug> --kind ignored-file <not-ignored-path>` | exit 3; message points at `feature claim add` |
| 2 | AC-1 | D3/D4 | `feature resource add <slug> --kind ignored-file <ignored-path>` | `resources.json` written with `res_<12hex>` ID |
| 3 | AC-2 | D5 | `feature resource add <slug> --kind git-metadata unknownview` | exit 2 |
| 4 | AC-2 | D5 | `feature resource add <slug> --kind git-metadata config:credential.helper` | exit 3 (disallowed key, valid view) |
| 5 | AC-2 | D5 | `feature resource add <slug> --kind git-metadata config:user.name` | success (allowlisted key) |
| 6 | AC-3 | D6/D7 | `feature resource add <slug> --kind adapter-snapshot --adapter unknown-tool` | exit 2 |
| 7 | AC-4 | D3 | `add` the same normalized `(kind, selector, adapter, capability, args)` twice | idempotent; single row, no error |
| 8 | AC-5 | D1/D3 | `feature resource list --json` on a feature with 3 resources of mixed kinds | stable-sorted by `resource_id`; no timestamp field present |
| 9 | AC-6 | D1/D10 | `feature resource remove <slug> <7-char-id-prefix>` (unambiguous) | manifest row + `resources/<id>/` sidecar tree both deleted |
| 10 | AC-7 | D10 | `feature resource clear <slug>` with 3 resources declared | all 3 manifest rows and all 3 sidecar trees deleted |
| 11 | AC-8 | D2 | `feature resource diff <slug>` with zero declared resources | prints "no resources declared"; exit 0 |
| 12 | AC-9 | D2/D9 | `feature resource diff <slug> --resource <id>` | only that resource's capability runs |
| 13 | AC-9 | D2/D9 | `feature resource diff <slug>` (no `--resource`) | every declared resource's capability runs |
| 14 | AC-10 | D8/D9 | `feature resource diff <slug> --dry-run` | no file under `resources/` changes |
| 15 | AC-11 | D5 | Synthetic `git-metadata`/adapter-snapshot output containing a `.git/`-prefixed path/header | refused at store-write boundary; exit 3; no sidecar file written |
| 16 | AC-12 | D6 | `generic-command` resource with `args.args` containing `; rm -rf /tmp/x` | literal argv content executed; no shell side effect; content appears verbatim in the (redaction-passing) snapshot |
| 17 | AC-13 | D6 | Adapter invocation exceeding its timeout | classified `timeout`; batch (D8) refused; no sidecar file written for that invocation |
| 18 | AC-14 | D6 | Adapter invocation producing >5 MiB stdout | classified `output-too-large`; output discarded, not truncated-and-kept |
| 19 | AC-15 | D4/D6 | Adapter/ignored-file output that trips the ADR-027 D3 redaction scan | classified `redaction-refused`; no snapshot/diff file written |
| 20 | AC-16 | D8 | `record <slug> --resources` on a feature with zero declared resources | exit 1; explicit "no resources declared" diagnostic |
| 21 | AC-17 | D8 | `record <slug> --resources` where every resource's capability succeeds | all resources' `snapshot.json`/`diff.json` updated; `history/NNN-diff.json` appended for each |
| 22 | AC-18 | D8 | `record <slug> --resources` where one of N resources' capability fails | zero resource sidecar files updated (all-or-nothing); Git-side canonical patch from the same invocation remains intact |
| 23 | AC-19 | D8 | `record <slug> --resources --dry-run` (mixed success/failure resources) | no sidecar file written under any outcome; would-be outcome reported per resource |
| 24 | AC-20 | D2/D8 | `record <slug> --resources` (success and failure cases) | `patch-generations.json` byte-identical before/after in both cases |
| 25 | AC-21 | D9 | Diff envelope for a resource with zero changes | `added`/`removed`/`changed` present as `[]`, never `null` |
| 26 | AC-22 | D6 | Repeated `dolt table-diff`/`git-metadata:refs` capture against unchanged underlying state, with a backend that reorders rows nondeterministically | `normalized_sha256` identical across repeated runs |
| 27 | AC-23 | D4 | `ignored-file` resource content >5 MiB at `add` time | exit 3; not silently truncated |
| 28 | AC-24 | D4/D9 | `ignored-file` resource with binary content (NUL byte in first 8 KiB) | `changed` entry has `change_kind: "binary"`; no line-level diff detail |
| 29 | AC-25 | D6 | Resource with `args.env=SOME_TOKEN` where `SOME_TOKEN` is set in the environment | only the name `SOME_TOKEN` appears in `resources.json`/snapshot/diff; the value never appears in any persisted artifact |
| 30 | AC-26 | D5/D6 | `generic-command` resource whose declared command `cat`s a file under `.git/` | store-write boundary guard still refuses (exit 3) even though the *selector* itself is not path-shaped |
| 31 | AC-27 | D1 | `feature resource add\|remove\|clear\|diff <bad-slug> ...` | "no such feature: X" diagnostic, matching `feature claim`'s shape |
| 32 | AC-28 | D1/D2 | Full existing regression suite (`record`, `reconcile`, `apply`, `land`, `verify`, `feature claim`, `feature deps`, `feature patch`, `feature unapply`) with no `feature resource`/`--resources` invocation | byte-for-byte unchanged behavior |
| 33 | AC-29 | all | Full test matrix (this table) | green on Cluster H' |
| 34 | AC-30 | — | `docs/feature-layout.md`, `docs/record.md`, `SPEC.md` updated to describe the new manifest/sidecar/command surface | docs match this ADR's exact shapes |
| 35 | — | D1 | `resources.json` schema-version mismatch on load | refusal, mirroring `LoadClaims`'s unsupported-version refusal shape |
| 36 | — | D3 | Two `adapter-snapshot` resources, same `(adapter, capability)`, different `args` | distinct `resource_id`s |
| 37 | — | D3 | `ignored-file` selector normalization: `add src/gen` (directory) vs. `add src/gen/` | same normalized selector, same `resource_id` (mirrors `NormalizeClaimPathShape` directory-trailing-slash behavior) |
| 38 | — | D4 | `gitutil.IsPathIgnored` returns `ErrGitUnavailable` (non-Git directory) at `add` time | refusal (fail-closed), not silent acceptance |
| 39 | — | D6 | Adapter binary absent from `PATH` (`dolt` not installed) | classified `adapter-missing`; exit 1; `add`/`list`/`remove` unaffected |
| 40 | — | D6 | Adapter version-probe command | `version_probe` field populated with trimmed stdout; never gates success/failure |
| 41 | — | D7 | `go.mod` diff after this ADR's implementation | zero new dependencies |
| 42 | — | D8 | `record <slug> --resources` invoked twice in a row with identical underlying state | second run's `history/NNN-diff.json` still appended (append-only; not deduplicated like patch-generation collision skip) |
| 43 | — | D9 | `json.Marshal` output field order for `diff.json` | matches struct declaration order (not alphabetical or map-iteration order) |
| 44 | — | D10 | `feature remove <slug>` (whole-feature cascade delete) with resources declared | `resources.json` and `resources/` deleted as part of whole-directory removal, no special-case code required |
| 45 | — | D2 | Static grep across `internal/workflow/reconcile.go`, `internal/cli/land.go`, dependency-gate source, `internal/cli/verify.go` | no reference to `resources.json`/`ResourcesManifest`/`resources/` |

Cluster H' implementation must achieve green on all 45 rows. Rows 15, 16, 19,
26, and 30 are **safety-critical**: they directly test the `.git/**`
exclusion, shell-injection safety, redaction-as-precondition, secret-by-
reference, and defense-in-depth store-write boundary invariants that this
ADR's binding safety constraints exist to enforce. Row 45 is the mechanical
verification of D2's "resources never gate lifecycle correctness" boundary.
