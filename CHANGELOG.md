# Changelog

All notable changes to tpatch are recorded here.

## v0.10.0 — 2026-05-23 — Wave β + Wave γ (patch-identity-metadata + patch-amend)

Bundles two slices of the WP-002 capture-and-metadata foundation cluster.
Wave α (file-claims + record-capture-modes) shipped earlier as v0.9.0.

### Patch-generations manifest (Wave β — ADR-024)

- New append-only `.tpatch/features/<slug>/artifacts/patch-generations.json`
  schema (`version: 1`) recording every patch capture per feature as a
  content-addressed `generation_id` of the form `pg_<12hex>` derived from
  patch bytes, base commit, capture mode, and pathspecs. `current_generation`
  is a monotonic integer; `generation_id` is stable across reorders.
- Zero wall-clock timestamps in the manifest — IDs are reproducible across
  machines and times.
- `git_patch_id` recorded via `gitutil.PatchID` with the `stable` algorithm
  for cross-rewrite stability.
- `refs` block is mandatory in v1 (strict-on-unknown plus refs-presence
  enforcement at load time). `PatchGeneration.Refs` is `*GenerationRefs`
  so omitted `refs` keys produce a load-time refusal rather than silent
  defaults.
- Dependency snapshots persisted per generation: each entry records the
  parent slug, kind, and the parent's `generation_id` + `git_patch_id`
  at the moment of capture.
- `record` and reconcile-driven refresh paths append a new generation
  entry. `RefreshAfterAccept` non-fatally warns to stderr (rather than
  failing the operation) when append fails for non-malformed reasons.
- `store.ErrMalformedManifest` sentinel narrows malformed-vs-I/O error
  classification: `LoadPatchGenerations` wraps JSON-decode and schema
  validation failures with `%w` and leaves I/O errors unwrapped. Workflow
  `AllowMalformedManifest` swallow is narrowed to
  `errors.Is(..., ErrMalformedManifest)` so I/O errors escape to the
  warning path.

### Patch amendment (Wave γ — ADR-026)

- New `tpatch feature patch refresh <slug>` — re-runs capture for the
  current feature and appends a generation with
  `kind: amend-refresh`. Exit 0 with no append on no-byte-change.
- New `tpatch feature patch fixup <slug> --reason "..."` — appends a
  generation with `kind: amend-fixup` referencing the previously-current
  generation via `fixup_of_generation` (auto-derived from the manifest;
  no `--target` flag). `--reason` is mandatory and persists on the entry.
- `tpatch record <slug>` (plain) classifies the resulting generation
  hybrid: `record` if no prior generation exists; otherwise
  `amend-refresh` (byte-changed re-record) or no-op (byte-identical).
- `kind ∈ {record, amend-refresh, amend-fixup}` enum landed; Wave β D8
  reservations transition to writable for `amend-refresh` and `amend-fixup`.
- Dependent-staleness gate: when a dependency's parent generation
  drifts from the snapshot captured at the child's last generation,
  `tpatch status` surfaces a `parent-generation-stale` overlay; `tpatch
  apply <slug>` and `tpatch reconcile <slug>` refuse for hard dependents
  and warn for soft dependents.
- The new gate honors the existing `features_dependencies` config
  opt-out — when the flag is `false`, dependency enforcement (both the
  existing ADR-011 gate and the new ADR-026 stale-parent gate) is a
  true no-op. Matches `internal/workflow/recipe.go:34` contract.
- `record --force-amend` remains Git-rewrite orphan-only per D8 —
  it is NOT a refresh shortcut, NOT a fixup shortcut, and continues
  to require unbroken downstream dependents.
- Metadata-only amend (D9) does not append patch generations in v1 —
  manifest identity is patch-byte boundary.
- Verify-freshness invalidation (D6): patch-content amendments
  invalidate cached verify state by hash inputs.

### Notes

- No schema version bump from v1 introduced in Wave β.
- Six skill surfaces updated by the parity guard for both Waves.
- Test count grew from 590 (pre-cluster) to 632 (post-Wave γ rev-2).
- `gofmt`, `go vet`, `go build ./cmd/tpatch`, `go test ./... -race`,
  and `go test ./assets/...` all clean at release.

### Process notes (carry-forward)

- Supervisor kickoff briefs must self-audit against binding ADRs
  before dispatch. Wave γ rev-0 briefly drifted from ADR-026 D4/D7
  on the `fixup --target` surface (fixed in rev-1).
- Briefs that reference policy ADRs (ADR-011, ADR-026) must
  enumerate config-flag opt-out contracts, not just enforcement
  semantics. Wave γ rev-1 missed the `features_dependencies`
  opt-out path for the new stale-parent gate (fixed in rev-2).
- Internal-reviewer checklist must cover explicit flag-off
  counter-scenarios for any new dependency-related enforcement.

## v0.9.0 — 2026-05-14 — Wave alpha (file-claims + capture-modes)

### Record capture modes

- `tpatch record --all` — explicit alias for the historical default
  (working-tree-all). Default `record` behavior is preserved
  byte-identical and pinned by regression test.
- `tpatch record --staged` — captures the diff from `HEAD` to the index
  using a temp index seeded with `GIT_INDEX_FILE` from `HEAD`. Refuses
  on empty index diff. Validation falls back to live-index
  `git apply --cached --check` only when temp-index setup fails (never
  silently downgrades to worktree validation).
- `tpatch record --unstaged` — captures index→worktree changes;
  untracked files are included via `git add --intent-to-add`. Refuses
  on empty unstaged diff.
- `tpatch record --claimed-only` — intersects the capture pathspecs
  with `claims.json` from the alpha-1 manifest. Refuses when the
  feature has no claims; composes with `--all`/`--staged`/`--unstaged`
  and with explicit `--files`. Empty intersection refuses.
- PRD §3.7 mutex matrix enforced pre-capture: explicit modes are
  mutually exclusive with each other, with `--from`/`--to`/`--range`,
  and with `--auto`. Pre-existing flag-pair diagnostics are preserved
  for backward compatibility; new pairs use a uniform "X is mutually
  exclusive with Y" message.
- `--staged` and `--unstaged` refuse on path overlap with the other
  layer (with refusal diagnostics) and emit a single note line for
  unrelated edits in the other layer (does not mutate the patch).
- New `## Capture Provenance` section in `record.md` with six PRD §4
  fields: `capture_mode`, `pathspecs`, `claim_ids` (the actually
  contributing subset when `--claimed-only` narrows with `--files`),
  `base_commit`, `upper_commit`, and `dirty_state` (one-line summary,
  never raw diff). Untracked-file policy varies by mode and is
  reflected in provenance. PRD: `docs/prds/PRD-record-capture-modes.md`.

### Feature claims

- `tpatch feature claim add <slug> <path...>` — declare advisory path
  claims for a feature; writes `.tpatch/features/<slug>/claims.json`
  (atomic `.tmp` + rename). Duplicate adds are idempotent and print
  `already claimed`. Reserved kinds (`glob`, `symbol`, `anchor`) and
  reserved mode `strict` are rejected with clear errors; v1 only writes
  `kind=path`, `mode=advisory`, `source=manual`. Paths under `.tpatch/`,
  installed skill surfaces (`.claude/skills/`, `.github/skills/`,
  `.github/prompts/`, `.cursor/rules/`, `.windsurfrules`), absolute
  paths, and `..`-escapes are all refused at the input boundary.
- `tpatch feature claim list <slug> [--json]` — list claims in a
  deterministic, stable-sorted order (by `claim_id`). `--json` emits
  the full manifest pretty-printed with two-space indent. Empty case
  prints `Claims for <slug>: (none)`.
- `tpatch feature claim remove <slug> <claim-id-or-path...>` — remove
  by full claim_id, by claim_id prefix of ≥ 7 hex chars (ambiguous
  prefixes error), or by exact normalized path value. Missing claims
  are a no-op with a `no such claim:` note (exit 0).
- `tpatch feature claim clear <slug>` — empties the manifest while
  keeping the file (`claims: []`, `version: 1`, `feature: <slug>`).
- Manifest schema: `{ version: 1, feature, claims: [ { claim_id, kind,
  value, mode, source } ] }`. `claim_id` is the first 12 hex chars of
  `SHA-256(feature ⧺ \x00 ⧺ kind ⧺ \x00 ⧺ value ⧺ \x00 ⧺ mode)`; no
  wall-clock timestamps anywhere on disk. PRD:
  `docs/prds/PRD-feature-file-claims.md`.

## v0.8.1 — 2026-05-13 — Wave D detector tails

### Reconcile

- `tpatch reconcile --check-applied-only <slug>` — read-only patch-id sweep; exit 0 on upstream match, 2 on no match. Forces phase 1.5 even when `patch_id_detector_enabled` is false.
- `tpatch reconcile --auto-drop-merged <slug>` — opt-in; on phase-1.5 match, removes the feature from the DAG (ADR-011 cascade) and preserves `Tpatch-CVE` / `Tpatch-Slug` trailers in the removal-commit message.

### Skill assets

- **Skill-doc references are self-contained** — the six shipped skill
  surfaces (Claude, Copilot, Copilot Prompt, Cursor, Windsurf, Generic
  workflow) no longer reference repo-relative `docs/*.md` paths. The
  `docs/land.md` and `docs/reconcile.md` "see further" pointers (and
  the equivalent `docs/adrs/ADR-010-...md` pointer) have been replaced
  with concise inline action snippets covering the land flow's record +
  safe-stage + four-trailer commit composition and the reconcile flow's
  clean-tree preflight + mutating-operation note (re-run `tpatch record`
  after). A new `TestSkillDocReferencesAreSelfContained` parity guard
  with 8 probe sub-tests prevents reintroduction of repo-relative
  `docs/*.md` references (matching bare, `./`, `../`, and `/` prefixed
  forms; URL-prefixed references like `https://`, `http://`, `file://`
  are allowed). PRD: `docs/prds/PRD-skill-doc-strategy.md`. ADR:
  `docs/adrs/ADR-020-skill-doc-references.md`.

## v0.8.0 — 2026-05-14 — M17 boundary-capture cluster

### Wave C

- **`tpatch land <slug>`** — new flagship verb that composes
  `record` (unless `--no-record`) with safe path-set staging and a
  single Git commit carrying the locked four-trailer block:
  `Tpatch-Feature`, `Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`,
  `Tpatch-Base-Commit`, followed by the repo `Co-authored-by:` trailer.
  The path set is computed deterministically from
  `git apply --numstat post-apply.patch` plus any dirty paths under
  `.tpatch/features/<slug>/`, plus the two named global metadata files
  (`.tpatch/upstream.lock`, `.tpatch/FEATURES.md`) **only when the
  embedded `record` step modified them**; operator-drifted globals are
  carved out (left dirty + stderr note — see Wave C rev-3 entry below).
  Any other dirty path is an "extra" and the
  command refuses unless `--allow-extra-paths` is passed (warn + stage).
  Preflight (PRD §3.2) refuses on conflict markers, `*.orig`/`*.rej`
  leftovers, mid-merge state, or hard-parent mismatch. The new HEAD's
  SHA is intentionally NOT written back to `apply.base_commit`
  (PRD F2 / ADR-019); `Tpatch-Feature` is the sole feature↔commit
  binding. Subject derivation precedence: `--message` >
  `spec.md` H1 > first non-empty `request.md` line > fallback
  `tpatch land: <slug>`. New flags: `--message`, `--no-record`,
  `--auto`, `--from`, `--allow-extra-paths`, `--dry-run` (and
  `--files` / `--allow-collision` forwarded to the embedded `record`
  step via `embedRecord`). The `--dry-run` contract (PRD §3.5) is non-mutating
  and prints the would-be subject, trailers, path set, and extras
  classification computed from existing artifacts. Pre-commit hook
  failures surface `git`'s diagnostics verbatim and the recovery hint
  `re-run with --no-record after fixing the hook`. PRD:
  `docs/prds/PRD-tpatch-land.md`. ADR: `docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md`.
- **Wave C rev-3 — global metadata carve-out (contract revision)** —
  `land`'s "working tree clean" post-condition is qualified to "clean
  with respect to feature scope". The two named global metadata files
  `.tpatch/upstream.lock` and `.tpatch/FEATURES.md` MAY retain
  unrelated operator drift after a successful `land`; in that case
  `land` emits a one-line stderr note per file
  (`note: leaving <path> dirty (operator drift outside feature scope;
  not staged)`) and leaves the file dirty in the working tree. The
  carve-out is bounded to those two files, no flag widens it. PRD
  §3.3 step 3 / §3.6 / §6 ac.6 amended; rationale and rejected
  alternatives in `docs/adrs/ADR-021-tpatch-land-global-metadata-carve-out.md`.
  No behavioral code change vs. rev-2 except the canonical note
  string (now pinned by `TestLand_DoesNotStageUnrelatedDirtyMetadata`).

### Wave D

- **Phase-1.5 deterministic patch-already-upstream detector
  (default-OFF)** — `tpatch reconcile` gained a new fast-path slotted
  between phase 1 (reverse-apply) and phase 2 (operation-level)
  evaluation. When enabled, it computes `git patch-id --stable` for the
  feature's `post-apply.patch` and walks the `(upstream.lock.commit,
  upstream-tip]` range looking for a matching upstream commit. On match,
  the verdict short-circuits to `ReconcileUpstreamed` with
  `Phase="phase-1.5-patch-id-match"`, the matching SHA replaces
  `UpstreamCommit`, and phases 2/3/4 are skipped — including any
  provider semantic call. Conservative by design: phase 1.5 never
  *flips* a non-merged verdict to merged and never overrides phases
  2-4 on no-match; tooling failures are surfaced as a one-line note
  and the reconcile continues. **Gated behind
  `Config.PatchIDDetectorEnabled` (default `false`).** When the flag
  is off (the v0.8.0 default), reconcile behaviour is byte-identical
  to pre-Wave-D: phase 1.5 is silently skipped, `PatchIDMatch` never
  populates, and `reconcile-session.json` round-trips unchanged. New
  flat YAML config keys: `patch_id_detector_enabled: true|false`
  (default false) and `patch_id_scan_limit: <int>` (default 5000,
  PRD §5.2). New types: `store.PatchIDMatch`,
  `store.ReconcileSummary.PatchIDMatch` (`omitempty`). New primitives:
  `gitutil.PatchID`, `gitutil.CommitPatchID`, `gitutil.RevListInRange`.
  Workflow: `internal/workflow/patch_id_detector.go`. PRD acceptance
  criteria not implemented in this wave: the optional CLI surfaces
  `--check-applied-only` (PRD §3.2) and `--auto-drop-merged` (PRD §3.3)
  remain on the backlog — the deterministic primitive ships first so
  the user-facing flag work can build on stable foundations. PRD:
  `docs/prds/PRD-patch-already-upstream-detector.md`.

- **Rev-1 fix: phase-1.5 now reads the canonical `post-apply.patch`
  directly (PRD §5.1)**. The initial Wave D landing reused the legacy
  `patch` variable in `internal/workflow/reconcile.go`, which prefers
  `incremental.patch` for multi-feature derivation (GAP 4). On a
  feature with both artifacts where upstream absorbs only the
  incremental subset, this caused phase-1.5 to false-positive a
  `[upstreamed]` retire path. The detector now loads
  `artifacts/post-apply.patch` separately for the patch-id sweep; if
  the canonical artifact is missing or empty, phase-1.5 fail-soft
  skips with a one-line note and reconcile falls through to phase 2.
  The legacy `patch` variable used by phases 2/3/4 is unchanged.
  Regression coverage: `TestPatchIDDetector_PrefersCanonicalOverIncremental`
  (negative — incremental subset matches upstream but canonical does
  not) and `TestPatchIDDetector_CanonicalMatchesEvenWhenIncrementalDiffers`
  (positive — canonical matches even when an unrelated incremental is
  present). Default-OFF behaviour is unchanged.

### Wave B

- **Record-time canonical patch collision detection** — `tpatch record`
  now scans every existing feature's `artifacts/post-apply.patch`
  before persisting the new canonical patch. Byte-identical
  cross-feature matches are refused by default with a diagnostic that
  lists each colliding slug, hash prefix, byte count, and file count;
  recovery hints are tailored to the capture mode (working-tree, `--from`,
  `--commit-range`, `--auto`) and point at `--files` and either
  `--from` or `--auto` as appropriate. The refusal happens before any
  artifact is written, so the colliding feature directory stays clean.
  Override the refusal with `--allow-collision "<reason>"` — the
  reason is mirrored to stderr and persisted in `record.md` under a
  "Collision Override" section. Same-feature re-records with identical
  bytes are treated as deduplication: the canonical artifact is
  rewritten in place and the numbered `patches/NNN-record.patch`
  audit snapshot is skipped with a one-line note. Same-feature
  re-records with changed bytes continue to append numbered snapshots
  as today. Empty captures skip the scan entirely (PRD §4 step 0).
  Numbered audit snapshots under `patches/` are intentionally not
  scanned — only the canonical `artifacts/post-apply.patch`
  participates (PRD §7). New CLI flag: `--allow-collision <reason>`.
  New primitive: `gitutil.PatchSignature(patch) (sha256Hex, bytes)`,
  reusable by future `tpatch patches --collisions`. PRD:
  `docs/prds/PRD-record-collision-detection.md`.

### Fixed (Wave A1 revision-1)

- **`record --auto` empty-capture false-green** — when a `--files`
  pathspec filtered the auto-inferred range down to zero textual diff,
  the command previously printed "No changes to record in the specified
  range" and exited 0, silently advancing the feature to applied state
  with an empty patch. Now refuses with a diagnostic naming the
  inferred range, ahead-count, the pathspec used, and a recovery hint
  (drop `--files`, widen the pathspec, or supply explicit
  `--from`/`--to`). Explicit `--from`/`--to` empty ranges keep the
  legacy success semantic so harness scripts probing ranges are not
  broken.
- **`record --auto` lock-fallback policy** — a populated-but-bogus
  `.tpatch/upstream.lock` (e.g. `remote: bogus`, `branch: missing`,
  empty commit) previously hard-refused with "populated but no field
  resolves to a commit reachable from HEAD". Per PRD-record-auto-base
  §3.2 step 5, "empty or unusable" lock content must trigger discovery
  of `refs/remotes/upstream/HEAD` / `refs/remotes/origin/HEAD` /
  conventional refs. The fallback predicate now treats unresolvable
  lock fields as unusable, emits a one-line warning
  (`record --auto: upstream.lock unusable (<reason>); falling back to
  discovery`), and only re-raises the populated-but-no-field-resolves
  refusal if discovery itself also fails.

### Added

- **`tpatch record <slug> --auto`** — opt-in baseline inference for the
  committed-range capture mode. Resolves the lower bound from
  `.tpatch/upstream.lock` (direct ancestor wins), falls back to
  `git merge-base` when the lock commit is on a divergent branch, and
  discovers a default upstream candidate (`refs/remotes/upstream/HEAD`,
  then `refs/remotes/origin/HEAD`, then conventional
  `upstream/main` / `origin/master` only when exactly one resolves) when
  the lock is empty. Prints the chosen base, source label, ahead count,
  and the equivalent `--from` command, then captures
  `<base>..HEAD` (capped by `--to` if given). Refuses ambiguous merge-base
  fallbacks (>1 commit), ambiguous multi-candidate discovery, dirty
  trees, and `--auto` + `--from` / `--auto` + `--commit-range`
  combinations. Persists the resolved lower bound in
  `status.apply.base_commit` for all committed-range modes (PRD-record-auto-base).
- **Shared `internal/store/upstream_lock.go` parser** (`LoadUpstreamLock` /
  `ParseUpstreamLock`) — zero-dep YAML-extraction primitive consumed by
  `record --auto` (Wave A1) and, when Wave A2 lands, the reconcile lock
  guard. Mirrors the flat-scalar pattern used by `parseYAMLConfig` for
  `.tpatch/config.yaml`.
- **`gitutil.SymbolicRef`** and **`gitutil.CommitCountInRange`** —
  thin git wrappers used by the auto-base inference.
- **6 skill surfaces** updated: the "before you run anything" preflight
  now recommends `tpatch record <slug> --auto` as the first-choice
  recovery path when feature edits have already been committed; explicit
  `--from <base>` remains documented as the fallback when `--auto`
  refuses (ambiguous merge-base or empty lock).

### Changed

- `record` now persists `status.apply.base_commit` as the resolved
  lower bound for **all** committed-range modes (`--auto`, `--from`,
  `--commit-range`), aligning persisted metadata with the canonical
  `artifacts/post-apply.patch` per `docs/feature-layout.md`. Working-tree
  capture continues to store `HEAD` (PRD §3.3).
- The empty-clean-tree refusal diagnostic now points at `tpatch record
  <slug> --auto` as the preferred recovery path, with `--from <base>`
  as the explicit fallback.
- `record.md` provenance grew **Capture mode**, **Base commit**, **Upper
  bound**, and **Pathspecs** lines (PRD §3.3 example).

### Wave A2 — reconcile upstream-lock validation guard

#### Added

- **`tpatch reconcile` upstream-lock validation guard** (PRD-reconcile-lock-guard).
  Every reconcile invocation now classifies `.tpatch/upstream.lock` into
  one of five states (Valid, Empty, Missing, Stale, Skipped) and applies
  an independent policy from the existing dirty-tree gate:
  - **Valid** → silent.
  - **Empty / Missing** → one-line warning, then proceed (the v0.6 init
    scaffold default is Empty; refusing first-reconcile on every fresh
    repo would be hostile).
  - **Stale** (recorded `commit:` is not an ancestor of
    `<remote>/<branch>` HEAD, or does not resolve at all, or the lock is
    partial) → block-style refusal with `STALE-COMMIT` / `STALE-RESOLVE`
    / `STALE-REF` sub-cause and a three-option recovery hint
    (`git fetch <remote>` / `--allow-stale-lock` override / different
    `--upstream-ref`).
  - **Skipped** (`--upstream-ref` resolves to a different symbolic
    ref-name than the lock keys on, compared via
    `git rev-parse --symbolic-full-name` rather than SHA equality so two
    branches at the same commit still classify as different keys) →
    informational note, proceed.
- **`--allow-stale-lock` flag** on `tpatch reconcile`. Bypasses the new
  lock-guard for one invocation only — no persistent suppression.
  Mirrors `--allow-dirty` in shape but composes independently; neither
  flag implies the other (clean tree + stale lock requires only
  `--allow-stale-lock`; dirty tree + valid lock requires only
  `--allow-dirty`).
- **Legacy lock read-side tolerance**: locks written by the pre-v0.7
  `updateUpstreamLock` (with `branch: upstream/main`) are read as if
  they had been written correctly (`branch: main`) so v0.6 repos do not
  refuse on day one. The next successful reconcile overwrites the file
  in the v0.7 normalized shape, eliminating the legacy case after one
  reconcile cycle per repo.
- **`gitutil.SplitUpstreamRef`** helper plus extension fields
  (`LockState`, `LockDiagnostic`) on `gitutil.ReconcilePreflight`. The
  existing `gitutil.PreflightReconcile(repoRoot)` single-arg signature
  is preserved verbatim; new callers use the sibling
  `gitutil.PreflightReconcileWithOverride(repoRoot, upstreamRefOverride)`
  to thread `--upstream-ref` into the classifier.

#### Fixed

- **`updateUpstreamLock` writer normalization** (PRD-reconcile-lock-guard
  §5.3; bundled with the guard as a precondition for it to function).
  The pre-v0.7 writer at `internal/workflow/reconcile.go:599-604`
  hard-coded `remote: upstream` regardless of the operator's real
  remote and interpolated `branch: %s` with the full upstream ref
  (e.g. `branch: upstream/main`). The lock-guard's ancestry check would
  have reassembled `<lock.remote>/<lock.branch>` as
  `upstream/upstream/main` — a ref that does not resolve — and refused
  every populated lock on day one. The writer now splits the upstream
  ref into the correct `(remote, branch)` pair, populates the
  decorative `url:` from `git remote get-url`, and refuses to clobber
  the lock with malformed input (zero or more-than-one slash) so a
  bad ref leaves the previous lock untouched.

## v0.7.0 — 2026-05-10 — feat-amend-dependent-warning

Continuation of M15 W3 freshness overlay. Adds detection + warning for the
classic failure mode where `git commit --amend` (or `git rebase`) on a commit
referenced by a downstream feature's `satisfied_by` or `base_commit` silently
orphans the dependent feature.

### Added

- **`tpatch status` — `dependent-broken` derived label.** Walks every active
  feature.yaml, collects every `apply.base_commit` and
  `dependencies[].satisfied_by` SHA, and checks reachability via
  `git merge-base --is-ancestor`. Composable with M15 W3 freshness labels per
  ADR-013. JSON output adds `"dependent_broken": true` plus
  `"broken_refs": [{"kind","sha","feature"}]` per affected feature.
- **`tpatch record --force-amend` flag.** When `tpatch record` detects an
  amend shape (current HEAD's parent equals reflog `HEAD@{1}`'s parent), it
  checks whether the amended-away SHA is referenced by any downstream feature
  and aborts by default with a clear error listing the orphaned features.
  `--force-amend` bypasses the gate (with a warning still printed to stderr).
- **6 skill surfaces** updated with a troubleshooting note for the
  `dependent-broken` label. New parity-guard anchor
  `dependent-broken/troubleshoot-line` locks the surfaces to the message.

### Implementation notes

- New `internal/store/dependents.go` houses `CollectDependentSHAs`,
  `IsAmendBreaking`, and `CollectBrokenRefs`; reused by both `status` and
  `record`.
- New `internal/gitutil/RevParse` wraps `git rev-parse --verify --quiet` so
  the record amend gate can probe `HEAD@{1}` without erroring on a missing
  reflog (fresh clone, single-commit history, …).
- 5 new tests cover record-refusal, `--force-amend` bypass, status label
  emission (text + JSON), and the broken-base-commit edge case.

### Fixed (revision-1)

- **`status --dag` and `status --dag --json` now emit `dependent-broken`.** The
  initial v0.7.0 commit wired the new derived label only into the non-DAG
  status renderers; the DAG renderers (`renderNodeLineWithFreshness` and
  `writeDAGJSON`) bypassed it, so users running `tpatch status --dag` or
  consuming `--dag --json` did not see the new label. The DAG renderers now
  receive the broken-refs map collected by `store.CollectBrokenRefs` and
  overlay `LabelDependentBroken` via the existing `appendLabel` helper. The
  DAG JSON node shape gains `dependent_broken` and `broken_refs` fields
  matching the non-DAG `--json` contract.
- **Plain-text `dependent-broken` status line now coalesces per feature.** The
  initial commit emitted one line per broken ref, producing duplicate output
  when a feature referenced the same rewritten SHA via both `apply.base_commit`
  and `satisfied_by`. Now exactly one line per affected feature, listing all
  broken abbrev SHAs (deduped + sorted).

## v0.6.4 — 2026-05-10 — M16 (operator polish, completion)

Final M16 release. Closes Slice 3 of the operator-polish bundle by
unifying `feat-apply-default-execute` and
`feat-skills-apply-auto-default`: the CLI default has been
`--mode auto` (which runs `prepare → execute → done` as a single
verb) since v0.6.0, but the 6 shipped skill surfaces still
documented `apply --mode execute` as the canonical invocation in
their lifecycle diagrams. This release closes that docs/skills gap so
agents reading the skills now see the simpler `tpatch apply <slug>`
form first, with the four-mode ladder preserved as an advanced /
state-machine fallback.

### Changed

- **All 6 skill surfaces now recommend `tpatch apply <slug>` (no
  explicit `--mode`) as the canonical user-facing invocation.** The
  Phase Ordering tables in `assets/skills/claude/tessera-patch/SKILL.md`,
  `assets/skills/copilot/tessera-patch/SKILL.md`,
  `assets/prompts/copilot/tessera-patch-apply.prompt.md`,
  `assets/skills/cursor/tessera-patch.mdc`,
  `assets/skills/windsurf/windsurfrules`, and
  `assets/workflows/tessera-patch-generic.md` were updated so the
  `implementing → applied` row reads `tpatch apply` instead of
  `tpatch apply --mode execute`. The advanced-fallback row
  (`tpatch apply --mode started / edit / --mode done`) is preserved
  and now explicitly tagged `(advanced)`. Path-safety prose
  (`EnsureSafeRepoPath` aborts) and `created_by`-gate prose
  (v0.6.0 hard-parent enforcement) intentionally retain the explicit
  `apply --mode execute` reference because they describe what the
  execute *phase* enforces, not how to invoke apply — `auto` runs the
  same execute phase, so the semantics are unchanged.

### Added

- **Parity-guard anchor `apply-default-auto/simple-invocation`** in
  `assets/assets_test.go`. The literal byte sequence
  `tpatch apply <slug>` is now required to appear in every shipped
  skill surface. This locks the simplified one-verb invocation
  in across all 6 surfaces and prevents future drift back to
  `apply --mode execute` in invocation-recommendation prose. The
  anchor only asserts that the simple form is present; it does not
  forbid the four-mode ladder, which remains documented as the
  advanced state-machine path.

### Fixed

- **Parity-guard anchor for `tpatch apply <slug>` strengthened**
  (revision-1 of v0.6.4, external supervisor finding on `eab2c3c`).
  The original `apply-default-auto/simple-invocation` anchor used
  `strings.Contains` and false-passed on the copilot prompt
  (`assets/prompts/copilot/tessera-patch-apply.prompt.md`) and the
  generic workflow (`assets/workflows/tessera-patch-generic.md`):
  both surfaces lacked a true standalone `tpatch apply <slug>`
  recommendation but did contain `tpatch apply <slug> --mode done`
  inside an advanced-fallback example, which satisfied the substring
  check. The anchor is now a regex
  (`(?m)tpatch apply <slug>(?:\s*$|\s+[^-\s]|`+"`"+`)`) that rejects
  substring matches inside advanced-mode continuations such as
  `tpatch apply <slug> --mode done`. The two affected surfaces had
  their Phase Ordering rows updated to carry the genuine standalone
  form (`implementing → tpatch apply <slug> → applied`), so the
  v0.6.4 prose claim that all 6 surfaces recommend the simple form
  is now true at the docs level *and* enforced at the test level.

### Notes

- No CLI behavior change. `internal/cli/cobra.go` still defaults
  `--mode` to `auto`, the four-mode ladder
  (`prepare`, `started`, `execute`, `done`) remains fully usable,
  and `auto` mode still runs `prepare → execute → done` end-to-end.
  This release is documentation/skills-only.

## v0.6.3 — 2026-05-10 — M16 (operator polish, partial)

Polish release. Slice 2 of the M16 bundle ships now;
`feat-apply-default-execute` (Slice 3) deferred to v0.6.4.

### Fixed

- **`tpatch record` no longer corrupts captured patches whose final
  hunk line ends in trailing whitespace** (Slice 2 —
  `bug-record-roundtrip-false-positive-markdown`). Markdown
  blockquote inserts of the form `> [!CAUTION]` whose body terminated
  in a `> ` continuation line (trailing space) had that space eaten
  by an over-eager `strings.TrimSpace` in `gitutil.CapturePatchScoped`
  / `CapturePatchFromCommitsScoped`. The corrupted patch then
  (correctly) failed `git apply --reverse --check`, surfacing as a
  misleading "patch does not round-trip against working tree" warning
  in `tpatch record`. Worse, the corrupted patch was also persisted
  to `patches/NNN-record.patch`. Capture now only normalizes the
  trailing-newline count and preserves all content bytes, including
  trailing whitespace on the final hunk line.

## v0.6.2 — 2026-05-10 — M15 Wave 3 (verify-freshness rollout complete)

Final M15 release. Adds `tpatch verify` — a freshness-overlay verb
that runs ten ordered structural checks (V0–V9) against a feature's
recipe, intent files, dependency graph, and shadow-replay
reconstruction, then writes a small `Verify` sub-record on
`status.json`. Composes with the existing reconcile-label vocabulary;
the lifecycle state is never mutated by verify.

PRD: [`docs/prds/PRD-verify-freshness.md`](./docs/prds/PRD-verify-freshness.md)
· ADR: [`docs/adrs/ADR-013-verify-freshness-overlay.md`](./docs/adrs/ADR-013-verify-freshness-overlay.md)
· Slice landing plan: [§9 of the PRD](./docs/prds/PRD-verify-freshness.md#9-implementation-slices).

### Added

- **`tpatch verify <slug>`** (Slices A + C) — runs the V0–V9 freshness
  overlay against a single post-apply feature. Writes a minimal
  `Verify` record (`verified_at`, `passed`, `recipe_hash_at_verify`,
  `patch_hash_at_verify`, `parent_snapshot`) to `status.json`. Emits
  a 10-check report on `--json` stdout. `--no-write` runs every check
  but skips persistence. `--quiet` suppresses per-check output. Exit
  codes: `0` passed, `2` for every failure mode (verdict failed,
  refused pre-apply state, V0 abort, missing slug, non-tpatch
  workspace), `1` reserved for generic CLI usage errors.
- **Hard-parent topological closure replay (V7/V8)** — verify
  reconstructs the closure of hard parents in a `gitutil.CreateShadow`
  worktree before replaying the target recipe and `git apply
  --check`-ing the captured patch. Soft parents do NOT contribute to
  the closure (same rule as the apply gate). Single shadow allocation
  per verify run; pruned on exit regardless of verdict (ADR-013 D7).
- **Four derived freshness labels** (Slice B) — `never-verified`,
  `verified-fresh`, `verified-stale`, `verify-failed` are derived at
  read time inside `composeLabelsFromStatus` and surfaced by
  `tpatch status` / `--dag` / `--json`. Pure read-time computation;
  no persisted transitions.
- **`amend` invalidates freshness on recipe touch** (Slice B) — when
  the bytes of `apply-recipe.json` change OR when `recipe_sha256` no
  longer matches `Verify.RecipeHashAtVerify`, `amend` clears the
  `Verify` record so the next read derives `never-verified`. Intent-
  only amends preserve. `amend --state tested` is rejected: there is
  no `tested` lifecycle state under the freshness-overlay model.
- **`tpatch verify --all`** (Slice D) — aggregate runner that walks
  every tracked feature in topological order (`store.TopologicalOrder`
  Kahn primitive; lex tie-break) and dispatches each post-apply
  feature through the unchanged single-feature `RunVerify` entry
  point. Pre-apply features (`requested`, `analyzed`, `defined`,
  `implementing`, `reconciling`, `reconciling-shadow`) are reported
  with a `skipped: pre-apply` row at their topo position; V0 is
  **not** invoked for skipped features and no `Verify` record is
  written. Aggregate footer counts {passed, failed, skipped, error};
  exit `2` if any feature failed or errored (skips alone do not flip
  the gate). `--json` emits
  `{schema_version, features: [...], summary: {...}}`.
- **§4.4 freshness bullet rolled to all 6 skill surfaces** (Slice D)
  — Claude, Copilot, Copilot Prompt, Cursor, Windsurf, and the
  generic workflow now ship the verbatim "Verify before composing."
  paragraph plus a one-line `verify --all` pointer. Anchors enforced
  by the parity guard.
- **`assets/assets_test.go` parity-guard extension** — two new anchor
  substrings (`Verify before composing.` and `tpatch verify --all`)
  must appear in every shipped skill format. Existing anchors
  preserved; the guard fails the moment any of the six surfaces
  drops or paraphrases the bullet.
- **`docs/dependencies.md` cross-link to verify** (Slice D) — short
  paragraph in the "Apply-time semantics" section explaining that
  hard dependencies are also the input to verify's V7/V8 closure
  replay; soft parents are not.

### Changed

- Skill assets gain a new `## Verify (freshness overlay)` section
  immediately after the lifecycle/phase-ordering block. No existing
  CLI-command anchors were dropped.
- `CHANGELOG.md` now documents the v0.6.2 release per Slice D's
  scope; v0.6.1 entry is unchanged.

### Notes

- **Out of scope for v0.6.2** (deliberately): provider calls (verify
  is offline by construction); state transitions (verify never
  mutates `FeatureState`); apply-gate consultation of freshness (the
  gate stays pure-lifecycle, ADR-013 D2); `--shadow` flag (rejected
  by design); `tpatch test` integration as a freshness producer
  (deferred to `feat-tested-state-test-producer`); recipe-op JSON
  schema additions (frozen); `verify --all` interaction with the
  `--shadow` lifecycle (no shared shadow primitive between aggregate
  runs and the per-feature V7/V8 closure replay — each feature
  allocates its own shadow inside `RunVerify` and prunes on exit).
- **Backward compatibility.** A v0.6.1 repo that never runs
  `tpatch verify` round-trips `status.json` byte-for-byte identical;
  the `Verify` field is `omitempty`-marshalled (ADR-013 D4).
- **No new external Go dependencies.** stdlib + `cobra/pflag` only.
- **Source-truth guard preserved.** Verify reads
  `status.Reconcile.Outcome` for V9; never any artifact under
  `artifacts/reconcile-session.json` (ADR-010 D5 / ADR-011 D6).

## v0.6.1 — 2026-04-27 — M15 Stabilization (Wave 1 + Wave 2 + fix-pass)

User-facing stabilization release. Seven backlog items + a four-finding fix-pass against the merged surface. No schema, ADR, or default-behavior changes — `v0.6.0` repos round-trip byte-identical and behave the same except where explicitly noted.

### Added

- **`tpatch define <slug>` accepts `spec` as an alias** (`internal/cli/cobra.go`). Lifecycle commands enumerated in skill assets gain the alias too. Backwards compatible: `define` is unchanged.
- **`tpatch record <slug> --files <pathspec>...`** narrows the captured patch to the supplied pathspecs (`internal/cli/cobra.go`, `internal/gitutil/gitutil.go::CapturePatchScoped`). `--files` is mutually exclusive with `--from`; the empty-pathspec path is byte-identical to the historical full-tree capture. Companion `gitutil.CaptureDiffStatScoped` keeps `post-apply-diff.txt` and `record.md` scoped to the same pathspecs (no metadata leak).
- **`tpatch record <slug>` autogenerates `apply-recipe.json`** from the captured patch (`internal/workflow/recipe.go`). Default produces a fresh recipe alongside the patch; existing recipes are preserved unless the file/op-set has drifted, in which case a `recipe-stale.json` sidecar is emitted with a stderr warning. `--regenerate-recipe` opts into overwriting. `--no-recipe-autogen` disables generation entirely. The recipe-op JSON schema is unchanged: file deletions are not in the schema today, so they are skipped during autogen with a stderr note (a future ADR is required to add a `delete-file` op).
- **Validation: `satisfied_by` reachability + 40-hex contract.** `internal/store/validation.go` now refuses to persist a `satisfied_by` value that is not a 40-character hex SHA *and* not reachable from `HEAD`. Sentinel errors `ErrSatisfiedByMalformed` and `ErrSatisfiedBySHANotReachable` make the failure mode explicit. Closes the M14.1 deliberate limitation (any well-formed hex was accepted) and the M15-W2 review F1 contract drift (validation accepted reachable short SHAs that the apply-time gate would reject later).
- **Cross-platform user-shell selection.** New `internal/workflow.UserShell()` helper picks `sh -c` on Unix and `cmd /C` on Windows. Used by the `test_command` runner, the syntax-check gate, and `tpatch test`. Companion `shellQuoteFor(goos, p)` quotes `{file}` substitutions with the right convention for the chosen shell (single-quote on Unix, double-quote with `""` doubling on Windows). Unix behavior is byte-identical.
- **Skill assets ship valid YAML frontmatter.** `assets/skills/copilot/tessera-patch/SKILL.md` and `assets/skills/claude/tessera-patch/SKILL.md` now begin with a `---`-delimited frontmatter block (`name`, `description`). The Copilot CLI / Claude Code skill loaders that previously rejected the files with "missing or malformed YAML frontmatter" now accept them. Skill body is unchanged; parity guard (`assets/assets_test.go`) holds.

### Changed

- **`tpatch record` now detects post-record drift** between the captured patch and any pre-existing recipe (file-set comparison). Drift triggers a `recipe-stale.json` sidecar plus a stderr warning rather than overwriting silently. Same-files-different-content is intentionally below the floor (file-set drift only); deeper drift detection is deferred to a follow-up.
- **`gitutil.CapturePatchScoped` surfaces git errors when called with explicit pathspecs.** Previously any `git diff` failure was collapsed into an empty patch, then `recordCmd` reported the generic "captured 0 bytes" diagnostic. Empty pathspecs preserves the historical tolerant behavior the unscoped capture path has always relied on.

### Notes

- **Apply-time dependency gate is unchanged.** It still does the cheap 40-hex regex check; reachability lives in validation. The two layers now enforce the same value space (defense-in-depth) and validation refuses to persist anything reachability would later reject.
- **Hookable seam pattern.** Two new package-level vars (`store.isAncestor = gitutil.IsAncestor`, `workflow.userShellFor`) keep unit tests free of real git/shell calls. Convention for future external-command call sites.
- **Recipe-op JSON schema unchanged.** The schema does not include a `delete-file` op; autogen skips deletions and warns. Adding the op type requires an ADR and a parity-guard update. Flagged for a future wave.
- **Source-truth guard (ADR-011 D6) preserved.** No new readers of `artifacts/reconcile-session.json`. `status.Reconcile.Outcome` remains the authoritative machine-readable reconcile result; `artifacts/post-apply.patch` remains the reconcile authority for replay.
- **No new external Go dependencies.** stdlib + `cobra/pflag` only.
- **Backward compatibility.** A v0.6.0 repo with no `--files`, no `spec` alias, no autogen interaction, and a Unix shell behaves byte-identical. Repos that already declared a `satisfied_by` short SHA will fail validation on next status — fix by replacing with the full 40-hex SHA (this was a save-now/fail-later path before; now it fails at edit time, which is the intended contract).
- **Out of scope for v0.6.1** (deferred to Wave 3 / later): `tpatch verify <slug>`, `tested` lifecycle state, code-presence reconcile verdicts, fresh-branch reconcile mode, recipe-op schema extension for deletions.



First user-facing release of the feature-dependency DAG. Features can now declare hard / soft parents; apply, reconcile, status, and remove all observe the graph. Default-on (toggle via `features_dependencies: false`). PRD: `docs/prds/PRD-feature-dependencies.md` · ADR: `docs/adrs/ADR-011-feature-dependencies.md` · User reference: `docs/dependencies.md`.

### Added

- **M14.1 — Schema + validator (`internal/store`)** — `status.json` gains `depends_on: [{slug, kind, satisfied_by?}]` (omitempty round-trip). `Config.FeaturesDependencies` (`features_dependencies: true|false` in `.tpatch/config.yaml`). Pure DAG primitives: `DetectCycles`, `TopologicalOrder`, `Children`. Validator (`ValidateDependencies` / `ValidateAllFeatures`) with five sentinel errors: self-dep, dangling, kind-conflict, cycles, satisfied_by-requires-upstream-merged. Atomic edit semantics (rejected change leaves store untouched).
- **M14.2 — Apply gate + ordering (`internal/workflow`)** — `tpatch apply --mode execute` checks each hard parent before any file mutation; lists unsatisfied parents with their states. Soft parents never gate. Recipe-level `created_by` field on every operation: declares "this op was authored by parent feature X", validated against the recipe's `depends_on`. Cycle-aware traversal everywhere; `RunImplement` and `RunApply` topo-sort dependents.
- **M14.3 — Reconcile labels + compound presentation (`internal/store`, `internal/workflow`)** — Three composable labels overlayed on `Reconcile.Outcome`: `waiting-on-parent`, `blocked-by-parent`, `stale-parent-applied`. `EffectiveOutcome()` reports the compound string `blocked-by-parent-and-needs-resolution` when the child's intrinsic outcome is `blocked-requires-human` AND `blocked-by-parent` is set (display-only — programmatic decisions still read `Outcome` and `Labels` separately). Source-truth guard: all DAG/label code reads `status.Reconcile.Outcome` via `store.LoadFeatureStatus`, NEVER `artifacts/reconcile-session.json` (ADR-010 D5). Adversarial test pins this.
- **M14.4 chunk A — `tpatch status --dag`** — ASCII renderer (hard `─►`, soft `┄►`) with full-graph and scoped-to-slug modes; `--json` flag emits a structured shape for harnesses. Cycle-safe (corrupted graph degrades to flat list with `⚠ cycle detected` warning rather than recursing).
- **M14.4 chunk B — Default flip** — `features_dependencies` now defaults to **true** when the key is absent. `Init()` template writes the explicit `true`. v0.5.3 byte-identity preserved by setting `features_dependencies: false`.
- **M14.4 chunk C — Dependency-management CLI** — New verb tree `tpatch feature deps [<slug> [add|remove] <parent>[:hard|:soft]] | --validate-all`. `tpatch amend <slug> --depends-on <parent>[:kind] / --remove-depends-on <parent>` (deps-only mode skips request.md rewrite). `tpatch remove <slug> --cascade` performs reverse-topo deletion with TTY confirm; `--cascade --force` skips the prompt for non-TTY use. **`--force` alone never bypasses the dep-integrity gate** (PRD §3.7, ADR-011 D7).
- **M14.4 chunk D — Status-time validation** — `tpatch status` (with or without `--dag`) re-runs `ValidateAllFeatures` and surfaces every cycle / dangling / kind-conflict inline.
- **M14.4 chunk E — 6-skill rollout** — All six shipped skill formats (Claude, Copilot, Copilot Prompt, Cursor, Windsurf, Generic workflow) updated atomically with the dependency surface. `created_by` description updated from "inert" (v0.5.x) to live apply-time gate (v0.6.0+) with the dry-run downgrade noted. Parity guard (`assets_test.go`) holds.
- **M14.4 chunk F — User reference** — `docs/dependencies.md` covers edge model, declaration paths, validation rules, apply-time gate, `created_by` op-level gate, reconcile labels, compound verdict, `status --dag` examples, `--cascade` contract, migration, and the explicit out-of-scope list.

### Changed

- `tpatch apply --mode execute` now **fails fast** when an op's `created_by` parent is missing from `depends_on` or when a hard parent is unsatisfied. `tpatch apply --dry-run` downgrades the missing-hard-parent miss to a **warning** so operators can inspect the planned changes; recipe-shape errors (parent absent from `depends_on`, unknown kind) remain hard in both modes (PRD §4.3).
- `Reconcile.Labels` is now persisted on `status.json` (omitempty — empty slices round-trip to `nil`).
- `tpatch status` topology and presentation are unchanged on dependency-free repos; the new label / DAG output only appears when relevant.

### Notes

- **Version**: bumped to `0.6.0` in `internal/cli/cobra.go`.
- **No new external Go dependencies**; stdlib + `cobra/pflag` only.
- **No tag is created in this commit** — supervisor performs the `v0.6.0` tag at closeout.
- **Backward compatibility**: a v0.5.3 repo that does not declare any `depends_on` continues to round-trip byte-identical and behaves exactly as before. Setting `features_dependencies: false` in `.tpatch/config.yaml` restores full v0.5.3 semantics for projects that pin tpatch behaviour by SHA.
- **Out of scope for v0.6.0** (deferred): provider-assisted parent-patch injection in the M12 resolver (ADR-011 D8); auto-inference of `created_by` from file paths (PRD §4.3.1); soft-only cascade modes.

## v0.6.0 — 2026-04-26 — Feature Dependencies (Tranche D)

First user-facing release of the feature-dependency DAG. Features can now declare hard / soft parents; apply, reconcile, status, and remove all observe the graph. Default-on (toggle via `features_dependencies: false`). PRD: `docs/prds/PRD-feature-dependencies.md` · ADR: `docs/adrs/ADR-011-feature-dependencies.md` · User reference: `docs/dependencies.md`.

### Added

- **M14.1 — Schema + validator (`internal/store`)** — `status.json` gains `depends_on: [{slug, kind, satisfied_by?}]` (omitempty round-trip). `Config.FeaturesDependencies` (`features_dependencies: true|false` in `.tpatch/config.yaml`). Pure DAG primitives: `DetectCycles`, `TopologicalOrder`, `Children`. Validator (`ValidateDependencies` / `ValidateAllFeatures`) with five sentinel errors: self-dep, dangling, kind-conflict, cycles, satisfied_by-requires-upstream-merged. Atomic edit semantics (rejected change leaves store untouched).
- **M14.2 — Apply gate + ordering (`internal/workflow`)** — `tpatch apply --mode execute` checks each hard parent before any file mutation; lists unsatisfied parents with their states. Soft parents never gate. Recipe-level `created_by` field on every operation: declares "this op was authored by parent feature X", validated against the recipe's `depends_on`. Cycle-aware traversal everywhere; `RunImplement` and `RunApply` topo-sort dependents.
- **M14.3 — Reconcile labels + compound presentation (`internal/store`, `internal/workflow`)** — Three composable labels overlayed on `Reconcile.Outcome`: `waiting-on-parent`, `blocked-by-parent`, `stale-parent-applied`. `EffectiveOutcome()` reports the compound string `blocked-by-parent-and-needs-resolution` when the child's intrinsic outcome is `blocked-requires-human` AND `blocked-by-parent` is set (display-only — programmatic decisions still read `Outcome` and `Labels` separately). Source-truth guard: all DAG/label code reads `status.Reconcile.Outcome` via `store.LoadFeatureStatus`, NEVER `artifacts/reconcile-session.json` (ADR-010 D5). Adversarial test pins this.
- **M14.4 chunk A — `tpatch status --dag`** — ASCII renderer (hard `─►`, soft `┄►`) with full-graph and scoped-to-slug modes; `--json` flag emits a structured shape for harnesses. Cycle-safe (corrupted graph degrades to flat list with `⚠ cycle detected` warning rather than recursing).
- **M14.4 chunk B — Default flip** — `features_dependencies` now defaults to **true** when the key is absent. `Init()` template writes the explicit `true`. v0.5.3 byte-identity preserved by setting `features_dependencies: false`.
- **M14.4 chunk C — Dependency-management CLI** — New verb tree `tpatch feature deps [<slug> [add|remove] <parent>[:hard|:soft]] | --validate-all`. `tpatch amend <slug> --depends-on <parent>[:kind] / --remove-depends-on <parent>` (deps-only mode skips request.md rewrite). `tpatch remove <slug> --cascade` performs reverse-topo deletion with TTY confirm; `--cascade --force` skips the prompt for non-TTY use. **`--force` alone never bypasses the dep-integrity gate** (PRD §3.7, ADR-011 D7).
- **M14.4 chunk D — Status-time validation** — `tpatch status` (with or without `--dag`) re-runs `ValidateAllFeatures` and surfaces every cycle / dangling / kind-conflict inline.
- **M14.4 chunk E — 6-skill rollout** — All six shipped skill formats (Claude, Copilot, Copilot Prompt, Cursor, Windsurf, Generic workflow) updated atomically with the dependency surface. `created_by` description updated from "inert" (v0.5.x) to live apply-time gate (v0.6.0+) with the dry-run downgrade noted. Parity guard (`assets_test.go`) holds.
- **M14.4 chunk F — User reference** — `docs/dependencies.md` covers edge model, declaration paths, validation rules, apply-time gate, `created_by` op-level gate, reconcile labels, compound verdict, `status --dag` examples, `--cascade` contract, migration, and the explicit out-of-scope list.

### Changed

- `tpatch apply --mode execute` now **fails fast** when an op's `created_by` parent is missing from `depends_on` or when a hard parent is unsatisfied. `tpatch apply --dry-run` downgrades the missing-hard-parent miss to a **warning** so operators can inspect the planned changes; recipe-shape errors (parent absent from `depends_on`, unknown kind) remain hard in both modes (PRD §4.3).
- `Reconcile.Labels` is now persisted on `status.json` (omitempty — empty slices round-trip to `nil`).
- `tpatch status` topology and presentation are unchanged on dependency-free repos; the new label / DAG output only appears when relevant.

### Notes

- **Version**: bumped to `0.6.0` in `internal/cli/cobra.go`.
- **No new external Go dependencies**; stdlib + `cobra/pflag` only.
- **No tag is created in this commit** — supervisor performs the `v0.6.0` tag at closeout.
- **Backward compatibility**: a v0.5.3 repo that does not declare any `depends_on` continues to round-trip byte-identical and behaves exactly as before. Setting `features_dependencies: false` in `.tpatch/config.yaml` restores full v0.5.3 semantics for projects that pin tpatch behaviour by SHA.
- **Out of scope for v0.6.0** (deferred): provider-assisted parent-patch injection in the M12 resolver (ADR-011 D8); auto-inference of `created_by` from file paths (PRD §4.3.1); soft-only cascade modes.


## v0.5.3 — Shadow Accept Accounting Fixes (Tranche C3)

Three follow-up findings from an external reviewer on the v0.5.2 shadow-accept flow. One silent correctness regression (manual `reconcile --accept` broken after shadow-awaiting), one missing end-to-end regression test, one status-metadata inconsistency that would mis-feed M14.3 DAG label composition.

### Fixed

- **c3-separate-resolution-artifact** (silent correctness bug) — Two writers had collided on `artifacts/reconcile-session.json`: the resolver wrote `ResolveResult` with per-file `outcomes[]`, then `saveReconcileArtifacts` overwrote the same path with the high-level `ReconcileResult` (no outcomes). `loadResolvedFiles` — called by manual `reconcile --accept <slug>` and `--shadow-diff` — then failed with "no resolved files recorded". Fix (Option A): split the artifacts. Resolver now writes `artifacts/resolution-session.json` (per-file outcomes, resolver-owned); `reconcile-session.json` remains the reconcile-owned high-level summary (external contract unchanged). `loadResolvedFiles` + `shadow-diff` + the `tryPhase35` notes string all read the new path. Drift audit synchronized 5 skill/prompt/workflow assets + `docs/agent-as-provider.md` + `docs/prds/PRD-provider-conflict-resolver.md`.
- **c3-accept-stamps-reconcile-outcome** (internal consistency / M14.3 blocker) — `workflow.AcceptShadow` marked `State=applied` and cleared the shadow pointer but left `Reconcile.Outcome=shadow-awaiting` stale in `status.json`. M14.3 label composition (ADR-011 D6) reads `Reconcile.Outcome` as the child's intrinsic verdict before overlaying DAG labels — stale outcome would yield wrong labels. Fix: `clearShadowPointerAndStamp` signature extended to `(s, slug, sessionID, phase)`; now sets `Reconcile.Outcome = ReconcileReapplied` and refreshes `Reconcile.AttemptedAt`. Auto-apply path already wrote the same value at the outer `updateFeatureState` — double-write is harmless (idempotent). Manual `--accept` now leaves a truthful terminal outcome.

### Added

- **c3-manual-accept-regression-test** — `TestGoldenReconcile_ManualAcceptFlow` in `internal/workflow/golden_reconcile_test.go`. Drives `RunReconcile(Resolve:true)` to a `shadow-awaiting` verdict, parses `resolution-session.json` inline (mirrors the CLI `loadResolvedFiles` path), calls `workflow.AcceptShadow`, then asserts: merged content on disk, `State=applied`, `Reconcile.Outcome=reapplied`, `ShadowPath` cleared, shadow directory pruned. Counterpart to `TestGoldenReconcile_ResolveApplyTruthful` (v0.5.2) — together they cover both shadow-accept paths end-to-end. Would have caught both artifact-collision and stale-outcome bugs in v0.5.2.

### Notes

- Version bumped to `0.5.3` in `internal/cli/cobra.go`.
- `gofmt -l .` clean · `go build ./cmd/tpatch` ok · `go test ./...` all green · assets parity guard passes.
- All 3 findings shipped as single-purpose commits on `main` (`4636878`, `3ac7465`, `8a4af4b`).
- Backward compatibility: an old `reconcile-session.json` written by v0.5.2's resolver with the pre-split schema is ignored on v0.5.3; re-running `reconcile --resolve` creates the correct `resolution-session.json`. Shadow worktrees are ephemeral — no migration needed.
- Code-review sub-agent verdict: **APPROVED**. Both manual and auto-apply paths converge on `Reconcile.Outcome=reapplied` with no divergence.

## v0.5.2 — Correctness Fix Pass (Tranche C2)

Six confirmed findings from the v0.4.3..v0.5.1 delta review. One silent correctness bug on the v0.5.0 headline feature (`reconcile --resolve --apply`), one index-dirt bug, one stale-guard gap, one contract-drift bug, one feature addition, one doc drift. 8 regression tests added. No new Go dependencies.

### Fixed

- **c2-resolve-apply-truthful** (silent correctness bug) — `reconcile --resolve --apply` could set `ReconcileReapplied` without ever copying the shadow worktree into the real tree. Root cause: `ResolveVerdictAutoAccepted` was mapped directly to `Reapplied` by the caller, while the actual copy logic lived only in the manual `--accept` CLI path. Fix: new shared helper `workflow.AcceptShadow` owns the full accept sequence (forward-apply non-conflicting hunks → copy shadow → real via `ensureSafeRepoPath` → refresh artifacts → mark state → prune shadow). Both the manual `--accept` path and the auto-apply path route through it. On mid-flight failure the shadow is preserved and outcome maps to `ReconcileBlockedRequiresHuman` with instructions (per ADR-010 D4). Regression guards: `TestAcceptShadowCopiesResolvedContentToRealTree`, `TestAcceptShadowErrorsWithoutShadow`, `TestGoldenReconcile_ResolveApplyTruthful`.
- **c2-refresh-index-clean** — `DiffFromCommitForPaths` used `git add -N` (intent-to-add) to surface untracked files in diffs but never cleaned up, leaving intent-to-add entries in the user's real git index after reconcile/refresh. Fix: run the diff against a throwaway index via `GIT_INDEX_FILE` (temp file, deferred unlink, seeded from the real index). Regression guard: `TestRefreshAfterAcceptLeavesIndexClean` (byte-compares `git status --porcelain` before/after + checks `git ls-files --stage` for intent-to-add marker).
- **c2-recipe-hash-provenance** — Recipe stale guard only detected HEAD drift, not content drift. Modifying `apply-recipe.json` bytes without a new commit went unnoticed. Fix: provenance sidecar now records `recipe_sha256` at generation; `apply --mode execute` warns if either HEAD or hash differs from stored. Backward compatible with legacy sidecars (missing hash field) — emits "predates recipe-hash guard" note, does not error. Regression guards: `content-drift-warning` and `legacy-sidecar-skips-hash-check` subtests of `TestApplyExecuteRecipeStaleGuard`.
- **c2-remove-piped-stdin** — `printf 'y\n' \| tpatch remove <slug>` refused with "non-TTY" even though the v0.5.1 contract said piped stdin auto-confirms. Fix: TTY check inverted — non-TTY now auto-yes (matches shipped contract); interactive TTY still prompts `[y/N]`; `--force` always skips. Regression guard: `TestRemovePipedStdinSkipsConfirmation` (uses `os.Pipe()`, not a fake reader).

### Added

- **c2-amend-append-flag** — New `tpatch amend --append <slug>` flag for append semantics; replace stays the default (per supervisor decision). `--append` and `--reset` are mutually exclusive (rejected with clear error). Tests: `TestAmendAppendConcatenates`, `TestAmendAppendAndResetRejected`. Structured section-aware append left for a future enhancement.

### Docs

- **c2-max-conflicts-drift** — 8 doc/skill sites claimed `--max-conflicts` default was 3; runtime (`DefaultMaxConflicts = 10`) was correct. Fixed all 8 (CHANGELOG, `docs/agent-as-provider.md`, and 6 shipped skill/prompt/workflow formats). Parity guard passes.

### Notes

- Version bumped to `0.5.2` in `internal/cli/cobra.go`.
- `gofmt -l .` clean · `go build ./cmd/tpatch` ok · `go test ./...` all green · assets parity guard passes.
- All 6 findings shipped as single-purpose commits on `main` (`36e058d..73cd648`).
- Code-review sub-agent verdict: **APPROVED**. No drift remains between manual and auto accept paths; `ReconcileReapplied` now unreachable without `AcceptShadow` success for shadow-based paths.

## v0.5.1 — UX Polish & Quick Wins (Tranche C1 / M13)

Low-risk, high-daily-use-impact improvements. 8 items; no new Go dependencies; all prior tests remain green.

### New

- **c1-recipe-stale-guard** — `tpatch implement` now writes `.tpatch/features/<slug>/artifacts/recipe-provenance.json` (sidecar, not a field on `apply-recipe.json` — avoids updating all 6 skill formats). `tpatch apply --mode execute` compares the current recipe hash + HEAD against the sidecar and prints a stderr warning if either drifted since implementation.
- **c1-apply-default-execute** — `tpatch apply` default mode flipped from `prepare` to `auto` (chains `prepare → execute → done`). **Breaking UX**: pass `--mode prepare` explicitly to retain v0.5.0 behavior. `applyCmd` refactored into `runApplyPrepare / Started / Execute / Done / Auto` helpers.
- **c1-add-stdin** — `tpatch add` now accepts the feature description from stdin when piped, e.g. `echo "Fix model ID translation" | tpatch add`. Empty stdin is rejected; positional args still work.
- **c1-progress-indicator** — Braille spinner (150ms cadence) shown during every LLM call. Wired at the single `GenerateWithRetry` choke point so it covers `analyze / define / explore / implement` uniformly. TTY-only by default; can be forced on for tests.
- **c1-edit-flag** — New `tpatch edit <slug> [--artifact <name>]` opens feature artifacts (`spec.md`, `exploration.md`, `apply-recipe.json`, etc.) in `$EDITOR`. Default artifact is state-aware.
- **c1-feature-amend** — New `tpatch amend <slug> [<additional notes...>|<stdin>] [--reset]` appends or replaces the feature description. Refuses missing features.
- **c1-feature-removal** — New `tpatch remove <slug> [--force]` deletes `.tpatch/features/<slug>/` and refreshes `FEATURES.md`. Interactive `[y/N]` prompt on TTY; `--force` or piped stdin skips it.
- **c1-record-lenient** — New `--lenient` flag on `tpatch record` skips reverse-apply round-trip validation (for whitespace-sensitive files where the check would false-positive). The default failure message now points users at `--lenient`. See commit for investigation notes — synthetic repros of the reported markdown false-positive all passed cleanly, so we ship the documented escape hatch rather than a speculative root-cause fix.

### Breaking UX

- `tpatch apply` without `--mode` now runs the full prepare→execute→done chain. Users or agents that relied on the previous `prepare`-only default must pass `--mode prepare` explicitly.

### Notes

- Version bumped to `0.5.1` in `internal/cli/cobra.go`.
- No changes to skill assets; parity guard green.
- All 9 tranche items landed as 9 single-purpose commits on `main`.

## v0.5.0 — Provider-Assisted Conflict Resolution (Tranche B2 / M12)

Headline ship of ADR-010: `tpatch reconcile --resolve` now routes 3-way conflicts through the configured provider, one file at a time, inside a **shadow worktree** (`.tpatch/shadow/<slug>-<ts>/`) so the real working tree is untouched until you `--accept`.

### New

- **Phase 3.5 in `reconcile`** — after `PreviewForwardApply` returns `ForwardApply3WayConflicts` and `--resolve` is set, the resolver hands each conflicted file to the provider with `spec.md` + `exploration.md` + base/ours/theirs + the `<<<<<<<`-marked conflict as context. Proposed resolutions land in a shadow git worktree and go through a validation gate: rejected if any `<<<<<<<` / `>>>>>>>` markers remain, or if resolver output fails the JSON schema.
- **New flags on `reconcile`**: `--resolve`, `--apply` (auto-accept when every file is `resolved`; requires `--resolve`), `--max-conflicts N` (abort before provider call if count > N, default 10), `--model <name>` (per-run override), `--accept <slug>` / `--reject <slug>` / `--shadow-diff <slug>` (terminal operations on a pending shadow session — mutually exclusive; slug is the flag value, not a positional arg). `validateReconcileFlags` rejects nonsensical combos before `openStoreFromCmd`.
- **Three new verdicts**: `shadow-awaiting` (all files resolved; feature state `reconciling-shadow`), `blocked-requires-human` (validation failed or no provider configured — ADR-010 D9: no heuristic fallback), `blocked-too-many-conflicts` (count > `--max-conflicts`; provider never called).
- **New feature state `reconciling-shadow`** — surfaced by `tpatch status` with the shadow path so agents acting as provider (Path B) can `ls` the shadow, edit files directly, and `tpatch reconcile --accept <slug>`.
- **`reconcile-session.json`** — `.tpatch/features/<slug>/reconciliation/reconcile-session.json` records per-file status, validation reasons, model used, shadow path, and overall verdict. Source of truth for Path B shadow editing.
- **`--accept` is surgical, not a blind copy** — applies non-conflicting hunks of `post-apply.patch` via `git apply --3way --exclude=<resolved>`, overlays resolved files from the shadow, regenerates `artifacts/post-apply.patch` scoped to the feature's touched files (intent-to-add ensures new files appear), and snapshots the resolution delta as `patches/NNN-reconcile.patch`. `apply-recipe.json` is deliberately NOT auto-regenerated (lossy from a raw diff) — re-run `tpatch implement` or `tpatch record` if the recipe matters to you. Without this fix the accept flow would leave the tree half-reconciled (non-conflicted hunks never applied).
- **Skill updates (all 6 formats)** — Claude, Copilot, Copilot Prompt, Cursor, Windsurf, Generic workflow, plus `docs/agent-as-provider.md`. New "Reconcile Phase 3.5" section in every format documents the flags, verdicts, `reconcile-session.json` schema, shadow worktree concept, and the authoritative accept-flow algorithm. Claude SKILL extends the feature-state diagram with the `reconciling-shadow` branch. Parity guard green.
- **Golden scenarios** — `internal/workflow/golden_reconcile_test.go` exercises `RunReconcile` end-to-end across the five ADR-010 acceptance scenarios: clean-reapply, shadow-awaiting, validation-failed, too-many-conflicts, and no-provider. Fixtures capture real `git diff --cached HEAD` output so `git apply --3way` can find its base blobs. A single `go test -run GoldenReconcile` now serves as the ADR-010 acceptance suite.

### Design notes

- **No heuristic fallback (ADR-010 D9)** — if `--resolve` is set and no provider is configured, the verdict is `blocked-requires-human` immediately. The CLI never silently degrades to a rule-based merger.
- **Back-compat** — `ReconcileOptions` zero-value preserves v0.4.x behavior (no phase 3.5, no shadow). All new `ReconcileSummary` fields are `omitempty`. Nothing changes for callers that don't opt in via `--resolve`.
- **`promoteIfMarkers` v0.4.4 guard preserved** on strict and 3-way-clean paths; phase 3.5 only runs in the 3-way-conflicts branch, so the defensive conflict-marker scan is never bypassed.

### Also in v0.5.0

- **bug-features-md-stale-state** — `FEATURES.md` was only regenerated inside `AddFeature`, so every subsequent state transition (`analyze`, `define`, `explore`, `implement`, `apply`, `record`, `reconcile`) updated `status.json` but left the human-readable index stuck on the original state. Most visible on Path B flows (`add → apply --mode started → --mode done → record`): the table kept showing `requested` even after the feature was fully `applied`. Fix: `SaveFeatureStatus` now calls `RefreshFeaturesIndex` after every write, so the index is always a projection of the live feature statuses. Errors refreshing the index are swallowed (status.json remains the source of truth; next write retries). New `TestSaveFeatureStatusRefreshesIndex` locks this in.

### Follow-ups registered

- `feat-resolver-heuristic-fallback` — opt-in `--heuristic` (basic `git merge -Xours/theirs` + `git rebase` attempts) for future consideration; blocked until the provider path has a track record.
- `feat-parallel-feature-workflows` — fan out multiple features across shadow worktrees for parallel agent workers.
- `feat-feature-standalonify` — rebase a feature that used to depend on another into a standalone feature.

## v0.4.4 — Honest Recipes (pre-B2 ground truth)

Two HIGH-severity bugs surfaced by the v0.4.3 live stress test on tesseracode/t3code (~20h, 9 features, 1 upstream sync). Tight patch release: no new features, fix the ground truth so Tranche B2 (provider-assisted conflict resolution) can land on a reconcile that doesn't lie.

### Fixes

- **bug-skill-recipe-schema-mismatch** — the v0.4.3 skills documented `apply-recipe.json` with wrong field names (`op` instead of `type`, `contents` instead of `content`), an invented `occurrences` field, and an unimplemented `delete-file` op. Every Path B user hit `ERROR: unknown operation type ""` on the first `apply --mode execute`. Corrected in all 6 formats (Claude, Copilot skill/prompt, Cursor, Windsurf, Generic) and in `docs/agent-as-provider.md`. Documented the supported `append-file` op (previously omitted). Added `TestSkillRecipeSchemaMatchesCLI` — a new parity-guard pass that extracts every ```json block from every skill, unmarshals the `operations` array into the authoritative `workflow.RecipeOperation` struct with `DisallowUnknownFields`, and verifies the op type against the CLI's switch. Prevents future drift: any field the skill documents that the CLI rejects (or vice versa) fails the build.
- **bug-reconcile-reapplied-with-conflict-markers** — the reconcile phase-4 preview had a degraded fallback: if `git worktree add` failed (bare repo, permissions, full disk), it silently dropped to `git apply --3way --check` and returned verdict `3WayClean` — the exact behaviour v0.4.2's A4 was supposed to eliminate. Fixed: the degraded path now returns `Blocked` with a clear "cannot verify 3-way merge cleanliness — refusing to guess" stderr. Added a belt-and-braces defensive pass: `ScanConflictMarkers` walks the live working tree after every Reapplied verdict and promotes to `Blocked` if any `<<<<<<< / >>>>>>>` markers are found, naming the offending files. New regression test `TestReconcilePromotesOnLiveMarkers` plants markers in an unrelated file and asserts promotion.

### Context

Both bugs were blockers for Tranche B2 (provider-assisted conflict resolution, ADR-010):
- B2 hinges on agents writing correct `apply-recipe.json` — Bug 1 made every agent-authored recipe fail.
- B2's entry point is the `3WayConflicts` verdict — Bug 2 meant reconcile could silently return `Reapplied` instead, never triggering the resolver.

No behavioural changes beyond the fixes. `--manual`, Path A/B, the v0.4.3 skills' structural additions all carry forward unchanged.

## v0.4.3 — Stand-In Agent, Part 1 (Tranche B1)

First slice of Tranche B. Surfaces the "agent-as-provider" pattern that emerged from v0.4.2 stress testing as a first-class workflow, and lets the agent advance feature state without calling the configured provider.

### New

- **`--manual` / `--skip-llm` flag on `analyze`, `define`, `explore`, `implement`** — when the agent has authored the phase's artifact by hand (Path B), pass `--manual` to advance state without invoking the provider. The flag validates that the expected artifact exists at the canonical path (and, for `implement`, is valid JSON) and refuses otherwise, pointing at the exact file. `--skip-llm` is an alias.
- **Skill rewrite (all 6 formats)** — Claude, Copilot skill, Copilot prompt, Cursor, Windsurf, Generic now teach Path A (CLI) and Path B (agent-authored) as equal peers. New sections in every format: "You Are the Provider" (when and why to take over), `apply-recipe.json` operation schema (literal search semantics, `EnsureSafeRepoPath`), "Patch vs recipe — mental model", and the 3WayConflicts playbook (`git checkout stash@{0}^3 -- .tpatch/`, never pop the stash). Parity guard extended from 10 → 16 anchor phrases.
- **`docs/agent-as-provider.md`** — long-form companion covering Path B end-to-end with worked recipe examples and a sample 3WayConflicts resolution.

### Design

- **ADR-010 `provider-conflict-resolver`** — locks the shape of the headline v0.5.0 feature (B8): phase 3.5 in reconcile, shadow worktree, per-file provider call with spec + exploration as intent, validation gate, report + `--apply`/`--accept`/`--reject` flags.
- **PRD `agent-as-provider-skills`** — full scope for this tranche (Path A/B contract, artifact map, flag spec, skill requirements, deferred items).

## v0.4.2 — Truthful Errors (Tranche A)

Ten fixes + three new docs surfaced by the v0.4.1 live stress test. Theme: when something goes wrong, say so loudly instead of silently advancing state.

### Fixes

- **A1 bug-implement-silent-fallback** — the implement phase no longer swallows LLM failures. Fallback to heuristic mode now emits a stderr warning naming the retry count, the underlying error, the raw-response artefact path, and the `max_tokens_implement` knob to try next. New `max_tokens_implement` config (default 16384, up from 8192 hard-coded).
- **A2 bug-cycle-state-mismatch** — `RunImplement` writes `state=implementing` instead of `state=defined`. Each `tpatch cycle` phase now asserts the state advanced post-Run* via `featureStateRank`.
- **A3 bug-record-validation-false-positive** — record now validates via `git apply --reverse --check` (proves round-trip against the tree the patch was captured from). The old forward `--check` produced guaranteed false positives because the patch is, by definition, already applied.
- **A4 bug-reconcile-phase4-false-positive** — phase 4 now runs `--3way` inside an isolated `git worktree` and classifies via a 4-state verdict: `Strict | 3WayClean | 3WayConflicts | Blocked`. Conflict markers promote to `ReconcileBlocked` instead of silently succeeding.
- **A5 bug-skill-invocation-clarity** — all 6 skill formats (Claude, Copilot skill/prompt, Cursor, Windsurf, Generic) carry three canonical top-of-file blocks: Invocation (no npx), Phase Ordering (state machine), Before You Run Anything (preflight). Parity guard enforces anchor phrases so the wording cannot drift.
- **A6 bug-provider-set-global** — `tpatch provider set` defaults to the **global** config (`$XDG_CONFIG_HOME/tpatch/config.yaml`); `--repo` overrides per-repo. Matches the user-level nature of provider config and stops failing outside a `.tpatch/` tree.
- **A7 bug-extract-json-robustness** — one `ExtractJSONObject` helper replaces four ad-hoc extractors. Brace-balanced, string-aware, handles trailing prose, nested objects, arrays, escaped quotes, bare fences. Subsumes `stripJSONFences`.
- **A8 doc-record-timing** — `tpatch record` on a clean tree without `--from` now refuses with exit 1, a "captured 0 bytes" diagnostic, and up to 10 candidate base commits from `git log`. Dirty-but-empty-diff case gets a distinct hint.

### New documentation

- **A8 docs/record.md** — two supported orderings (working tree / `--from`), the anti-pattern, decision table, refusal example.
- **A9 docs/feature-layout.md** — file-by-file reference with the big "canonical vs audit trail" callout: `artifacts/post-apply.patch` is always the replay target; `patches/NNN-*.patch` is append-only audit history with full-diff snapshots, not incremental deltas. `tpatch record` now prints a cleanup hint when `patches/` exceeds six files.
- **A10 docs/reconcile.md** — two supported workflow patterns, the anti-pattern, troubleshooting block, full preflight contract.

### A10 reconcile preflight

- `tpatch reconcile` refuses dirty trees, lingering conflict markers, and `*.orig`/`*.rej` leftovers. Error message names every violating file and prescribes the remediation (abort merge, reset, stash, or `--allow-dirty` override).
- `tpatch reconcile --preflight` — CI-friendly gate: runs the checks and exits, no reconcile phases.
- `tpatch reconcile --allow-dirty` — escape hatch with a warning banner; verdicts may be wrong.
- On successful reconcile, tips you off if `.tpatch/` is untracked in git.

### Deferred to v0.5.x / v0.6.0 (logged in session tracker)

Ideas captured during Tranche A for future milestones: `feat-init-skill-drift`, `feat-soft-recipe-mode`, `feat-noncontiguous-feature-commits`, `feat-max-tokens-uncapped`, `feat-record-auto-base`, `feat-patches-subcommand`, `feat-record-dedup-patches`, `feat-ci-cd-integration`, `feat-autoresearch-iterate-until-green`, `feat-delivery-modes`.

---

## v0.4.1 and earlier

See commit history — changelog adopted at v0.4.2.
