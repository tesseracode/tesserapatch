# PRD - Multi-slug Reconcile Canonical Safety - `multi-slug-reconcile-canonical-safety`

**Status**: Proposed
**Date**: 2026-07-31
**Owner**: Core
**Cluster**: Replay trustworthiness (GH #3)
**Depends on**: [ADR-024 — Patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md), [ADR-011 — Feature dependency DAG](../adrs/ADR-011-feature-dependencies.md), [ADR-028 — Supersession edge model](../adrs/ADR-028-supersession-edge-model.md)
**Blocks**: safe multi-slug reconcile invocations for scoped/claimed feature stacks (GH #3 remediation)
**Companion ADR**: [ADR-030 — Multi-slug reconcile derivation mode](../adrs/ADR-030-multi-slug-reconcile-derivation-mode.md)

## Related

- GH #3 — multi-slug reconcile corrupts independent canonical patches via cumulative delta derivation.
- [PRD-record-capture-modes](./PRD-record-capture-modes.md) — capture-mode provenance vocabulary consumed by this PRD's design.
- [PRD-feature-file-claims](./PRD-feature-file-claims.md) — claim-based ownership metadata (`claims.json`, `--claimed-only`) already flowing into `capture.claim_ids`.
- [PRD-feature-patch-identity-metadata](./PRD-feature-patch-identity-metadata.md) — `patch-generations.json` schema authored by ADR-024.
- [PRD-tpatch-land](./PRD-tpatch-land.md) — one-feature-per-commit trailer contract.
- [PRD-feature-supersession](./PRD-feature-supersession.md) — effective/historical semantics used by the existing multi-slug prelude.
- [PRD-write-file-recipe-safety](./PRD-write-file-recipe-safety.md) — reference PRD for replay-safety patterns (later-touch detection, safeguard classification).
- [ADR-025 — Reconcile evidence and revision schema](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md) — reconcile audit surface where derivation-mode provenance should land.

## 0. Meta

### 0.1 Paper-only status

This PRD is **Proposed**. It changes no code, schema, CLI behavior, shipped assets, CHANGELOG entry, or release artifact. It defines a future implementation contract that ADR-030 will lock semantically. Supervisor acceptance and three-way review are still required before implementation.

### 0.2 Claims audit

Every load-bearing claim below cites a `file:line` anchor at HEAD `5ac458d` or a doc anchor by section heading. Reviewers should spot-check that cites land within ±5 lines of current code.

**HEAD-repro caveat (rev-1)**. The semantic C-cancels-B failure mode was **not** empirically re-reproduced on HEAD `5ac458d` during PRD drafting. The dispatch condition (`len(slugs) > 1` at `internal/workflow/reconcile.go:197`) and the `deriveIncrementalPatches` body (`reconcile.go:1145-1174`) are structurally unchanged from the v0.11.3 layout the reporter observed, so the bug's persistence on `main` is inferred from code identity rather than a fresh reproduction. The `.git/**` leak (§2.3) was reproduced empirically this session (row 7 of the table below). The implementation slice should reproduce the semantic failure empirically as a first step (TS-1) before the fix lands.

| Claim | Evidence |
|---|---|
| `RunReconcile` unconditionally calls `deriveIncrementalPatches` whenever more than one slug is being reconciled. | `internal/workflow/reconcile.go:195-199` (`if len(slugs) > 1 { deriveIncrementalPatches(s, slugs, upstreamCommit) }`). GH #3 reports the dispatch at lines 155-159; v0.12.0 Wave α/β additions have shifted the block to 195-199 without changing its condition. |
| `deriveIncrementalPatches` encodes a hard-coded cumulative assumption in a code comment. | `internal/workflow/reconcile.go:1145-1148` (`// When features A and B are applied sequentially, B's cumulative patch includes A's changes. // This function derives the delta (incremental) patch for each feature and saves it alongside // the cumulative patch so reconciliation uses only the feature's own changes.`). |
| `deriveIncrementalPatches` derives every non-first slug's incremental patch by subtracting the previous slug's canonical patch, with a fallback to the canonical patch when derivation fails. | `internal/workflow/reconcile.go:1149-1174` (`for _, slug := range slugs { … if prevCumulative == "" { s.WriteArtifact(slug, "incremental.patch", currentPatch) } else { incremental, err := gitutil.DeriveIncrementalPatch(...) if err != nil || incremental == "" { s.WriteArtifact(slug, "incremental.patch", currentPatch) } else { s.WriteArtifact(slug, "incremental.patch", incremental) } } prevCumulative = currentPatch }`). |
| `reconcileFeature` prefers `incremental.patch` over `post-apply.patch` when both artifacts exist. | `internal/workflow/reconcile.go:251-258` (`patch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "incremental.patch")) ; if err != nil { patch, err = s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch")); ... }`). |
| Phase-1.5 patch-id detector explicitly loads canonical `post-apply.patch` separately from the derivation-preferred `patch` variable, because the incremental form is unsafe for detector semantics. | `internal/workflow/reconcile.go:283-293` (`Rev-1 (M17 Wave D): the detector MUST run against the canonical post-apply.patch (PRD-patch-already-upstream-detector §5.1). The legacy patch variable above prefers incremental.patch for multi-feature derivation (GAP 4); the incremental form may match a partial absorption that isn't a real merge, producing a false-positive retire path. Load canonical separately and fail-soft skip if it's missing.`). |
| `gitutil.DeriveIncrementalPatch` uses `git clone --no-checkout` twice, then `git checkout <baseCommit>` on each clone, then plain GNU `diff -ruN prevDir currDir` across the two directory trees. | `internal/gitutil/gitutil.go:946-999` (`runGit(".", "clone", "--no-checkout", repoRoot, dir); … runGit(dir, "checkout", baseCommit); … exec.Command("diff", "-ruN", prevDir, currDir); … result = strings.ReplaceAll(result, prevDir+"/", "a/"); result = strings.ReplaceAll(result, currDir+"/", "b/")`). |
| `diff -ruN` on two independent clones deterministically produces diffs of `.git/logs/HEAD`, `.git/logs/refs/heads/*`, `.git/logs/refs/remotes/origin/*`, and a binary-differ line for `.git/index`. | Reproduced during PRD drafting: two `git clone --no-checkout` clones of this repo checked out to the same commit differ in reflog timestamps and index bytes; `diff -ruN` reports `.git/logs/HEAD`, `.git/logs/refs/heads/main`, `.git/logs/refs/remotes/origin/HEAD`, and `Binary files prev/.git/index and curr/.git/index differ`. Root cause: reflog entries are written with wall-clock second precision on each clone/checkout and `.git/index` records the same directory tree with different mtimes. |
| `patch-generations.json` records a `capture.mode`, `capture.pathspecs`, `capture.claim_ids`, `touched_paths`, and `base_commit` per generation. | `internal/store/patch_generations.go:37-79` (schema); `internal/workflow/patch_generations.go:82-100` (population). |
| Current `capture.mode` enum values written by the CLI are `working-tree-all`, `staged-index`, `unstaged-worktree`, `auto-committed-range`, `committed-range`, `explicit-committed-range`. | `internal/cli/record_capture_modes.go:32-38`. Reconcile refresh additionally writes `reconcile` mode. |
| Reconcile refresh appends a `reconcile`-mode generation after a successful post-3-way rewrite. | `internal/workflow/refresh.go:74-93`. |
| Feature dependency ordering (`DependsOn`) is snapshotted into each generation as `Dependencies`. | `internal/workflow/patch_generations.go:120-146`. |
| The active reconcile flow already respects the ADR-011 D9 dependency DAG when `Config.DAGEnabled()` is true, reordering slugs into hard-parent topological order before dispatch. | `internal/workflow/reconcile.go:181-192`. |
| The active reconcile flow already runs the ADR-028 supersession filter before the multi-slug branch. | `internal/workflow/reconcile.go:149-183`. |
| No CLI flag currently controls `deriveIncrementalPatches`; it fires whenever `len(slugs) > 1`. | Grep for `cumulative-legacy`, `CumulativeLegacy`, or `--cumulative` returns no matches under `internal/`. |

No code, schema, command behavior, or asset text is changed by this PRD.

## 1. Summary

`tpatch reconcile` currently treats every multi-slug invocation as if the canonical `post-apply.patch` of each subsequent slug were a **cumulative** diff that includes the previous slug's changes. It then subtracts the previous slug to derive an `incremental.patch` and reconciles against the derived delta rather than the canonical patch.

That assumption predates scoped and claimed capture. Modern features recorded with `--files`, `--auto`, `--from`, `--staged`, `--claimed-only`, or `tpatch land` produce canonical patches that are **independent by design** — each patch is already the smallest useful representation of only that feature's changes. Subtracting a sibling patch from an already-independent canonical patch corrupts the derived artifact:

- Slug B's derived `incremental.patch` gains transient artifacts from the delta-worktree machinery (`.git/logs/**`, `.git/index`).
- Slug C's derived `incremental.patch` cancels out slug B's real files instead of representing slug C's real changes, so reconcile evaluates the wrong hunks and reports diagnostics on files C never touched.

The reporter observed this on `tpatch v0.11.3` during the `t3code v0.0.31` pilot with three independent features (Copilot provider files, session-search files, WSL files). Running each slug in its own `reconcile` invocation avoids the derivation branch and yields correct verdicts, which is the current safe workaround.

This PRD:

1. Defines the invariant that a canonical `post-apply.patch` is authoritative and must be byte-identical in single-slug and multi-slug reconcile.
2. Locks the derivation semantic (via ADR-030): the cumulative-derivation branch is **default-OFF** and gated on an explicit `--cumulative-legacy` opt-in.
3. Requires that no `.git/**` path may enter any tpatch artifact from any code path — a repository-wide invariant, enforced by the delta-worktree helper and by post-write filters.
4. Provides a metadata-driven forward path (Option B) for a future minor version once `patch-generations.json` coverage is universal.

## 2. Problem statement

### 2.1 Reporter's reproduction (verbatim summary)

Reporter: independent feature stack on `t3code v0.0.31`, tpatch `v0.11.3`.

Setup:

- Feature **A** — Copilot provider files.
- Feature **B** — session-search files (`ChatView.tsx`, `SessionSearchBar.tsx`, keybindings JSON).
- Feature **C** — WSL files.

All three are recorded with scoped capture — each `post-apply.patch` only mentions the feature's own paths. No `incremental.patch` exists on disk before the reconcile.

Invocation:

```bash
tpatch reconcile A B C --upstream-ref upstream/main
```

Observed after the run:

- A's `incremental.patch` equals A's canonical patch (expected — first slug, `prevCumulative` empty at `reconcile.go:1160-1161`).
- B's `incremental.patch` equals B's canonical patch plus `.git/logs/**` entries (unexpected — delta-worktree leak).
- C's `incremental.patch` **removes** B's session-search files instead of describing C's WSL changes (incorrect — cumulative subtraction of an independent sibling).

Downstream effect: C's phase-2 reverse-apply and phase-3 3-way analysis run against the corrupted incremental patch, so C's `reconcile.md` diagnostics reference `ChatView.tsx`, `SessionSearchBar.tsx`, and keybindings — files C never touched — instead of WSL paths.

Reporter's workaround: `tpatch reconcile A --upstream-ref upstream/main && tpatch reconcile B --upstream-ref upstream/main && tpatch reconcile C --upstream-ref upstream/main`. This bypasses the `len(slugs) > 1` branch entirely and every slug reconciles against its canonical patch.

### 2.2 Failure mode 1 — cumulative subtraction on independent siblings

`deriveIncrementalPatches` (`reconcile.go:1149-1174`) walks the slug list, seeds `prevCumulative` from slug A, and for each subsequent slug computes `git apply prev` in one temp clone, `git apply curr` in another, then diffs the two directory trees. When A and B are truly cumulative (B's canonical patch is a superset of A's), the diff is correct. When A and B are independent, the diff is not "B's contribution" — it is "B minus A", which is nonsense and can include negative hunks that remove A's real files.

The nesting cascades: C's derivation uses B's canonical patch as `prevCumulative`, but B's canonical patch was never applied to the base — so `curr` starts from the base plus B's changes, `prev` starts from the base plus B's changes, and adding C's changes to `curr` yields "C minus 0" (correct) only when C is truly independent of both A and B. For any overlap (or any accidental base drift), the result is unpredictable.

The reporter's C case shows the pathological shape: because B's patch modifies files that no other slug touches, the "prev" worktree has session-search files but the "curr" worktree does not (C's canonical patch never re-adds them), so the diff describes **removing B's files** as if that were C's contribution.

### 2.3 Failure mode 2 — `.git/**` leak from plain `diff -ruN`

`DeriveIncrementalPatch` (`gitutil.go:946-999`) uses `exec.Command("diff", "-ruN", prevDir, currDir)` to compute the delta. GNU `diff -ruN` recurses **all** files under both directories with no exclusion list. Because both temp dirs are full `git clone --no-checkout` clones with a subsequent `git checkout <baseCommit>`:

- `.git/logs/HEAD` differs — the two clones record their `clone: from …` reflog entry at distinct wall-clock seconds.
- `.git/logs/refs/heads/<default>` and `.git/logs/refs/remotes/origin/*` differ for the same reason.
- `.git/index` differs as a binary — same content, different mtimes and index nanos.
- If the base commit contains untracked-in-parent files, additional `.git/**` byte differences show up.

These diffs land verbatim in `incremental.patch`. Downstream consumers (`reconcileFeature`, phase-2/3 apply, `reconcile.md`, `reconcile-evidence.jsonl`) treat them as legitimate feature hunks. Nothing in the pipeline strips `.git/**` before or after diff.

### 2.4 Failure mode 3 — phase 1.5 already tacitly acknowledged the incremental form is unsafe

The phase-1.5 patch-id detector added in M17 Wave D explicitly bypasses the derivation-preferred `patch` variable and loads canonical `post-apply.patch` separately (`reconcile.go:283-293`), citing exactly this concern: "the incremental form may match a partial absorption that isn't a real merge, producing a false-positive retire path." That local carve-out demonstrates the tension — the codebase already knows the derived artifact is not authoritative — but leaves the corrupted `incremental.patch` in place as the input for phases 1, 2, 3, and 3.5, and for downstream `reconcile.md` / evidence writes.

### 2.5 Scope of impact

- Any user or agent invoking `tpatch reconcile A B C …` with more than one slug.
- Any pilot that composes reconciles (e.g., the reporter's `t3code v0.0.31` upgrade).
- Any automation that assumes `reconcile.md` diagnostics are grounded in the feature's own canonical patch.
- Any external tool that consumes `.tpatch/features/<slug>/artifacts/incremental.patch` (currently: none tracked, but the artifact is on disk and can be picked up by hand-written scripts and by the phase-1/2/3 pipeline).

## 3. Existing primitives audit ("could existing primitives do this?")

Per AGENTS.md's PRD-authoring conventions, an exploratory PRD that proposes new capture semantics should enumerate the primitives already present in the repository and explain why they can or cannot carry the new responsibility. The audit below covers every metadata field that could distinguish "independent canonical patch" from "cumulative canonical patch."

### 3.1 `patch-generations.json` `capture.mode`

Values written today (`internal/cli/record_capture_modes.go:32-38`, `internal/workflow/refresh.go:74-93`):

- `working-tree-all`
- `staged-index`
- `unstaged-worktree`
- `auto-committed-range`
- `committed-range`
- `explicit-committed-range`
- `reconcile`

Verdict: **necessary but not sufficient**. `working-tree-all` in particular is written for the default `tpatch record <slug>` and does **not** promise independence — a user can dirty the working tree with sibling-feature changes and record them all under the default mode. The scoped modes (`staged-index`, `committed-range`, `auto-committed-range`, `explicit-committed-range`) are stronger evidence of independence but the enum was designed for provenance display, not for gating derivation.

### 3.2 `patch-generations.json` `capture.pathspecs`

Populated when `--files` or the resolved `--claimed-only` set is non-empty (`internal/cli/cobra.go:1522-1530`). Present pathspecs are strong evidence of scoped capture; absent pathspecs are ambiguous.

Verdict: **useful as an independence signal, not conclusive alone**.

### 3.3 `patch-generations.json` `capture.claim_ids`

Populated when `--claimed-only` resolved against `claims.json` (`internal/cli/record_capture_modes.go` resolveClaimedOnly path, feeding the same `Capture` struct). Non-empty `claim_ids` means the operator explicitly declared ownership over specific paths before recording, and the recorder intersected the diff with that ownership.

Verdict: **strong independence signal but adoption is not universal** — claims are advisory (PRD-feature-file-claims v1) and most existing feature directories in the wild have none.

### 3.4 `patch-generations.json` `touched_paths`

Sorted list of paths in the patch (`internal/workflow/patch_generations.go:76-77` compute + sort, `:95` assign to `TouchedPaths`). Enables the disjoint-touched-paths check that would prove two canonical patches cannot cumulatively contaminate each other. If A's touched paths and B's touched paths are disjoint, cumulative subtraction is a no-op **and** independent subtraction is a no-op — the two semantics collapse.

Verdict: **decisive when combined with base_commit**, present for every generation. This is the primitive that Option B (metadata-driven auto-detect) leans on.

### 3.5 `patch-generations.json` `base_commit`

`.tpatch/features/<slug>/artifacts/patch-generations.json` records the base commit each generation was captured against (`internal/workflow/patch_generations.go:88`). Two features that share the same `base_commit` and have disjoint `touched_paths` are provably independent; two features with different `base_commit` values were recorded against different upstream tips and cumulative derivation is meaningless anyway.

Verdict: **decisive when combined with `touched_paths`**.

### 3.6 `feature_status.json` `apply.base_commit`

`Status.Apply.BaseCommit` is set during record (`internal/cli/cobra.go:1462-1475`). Same field as §3.5 but on the lifecycle document. Present for every feature that has been recorded.

Verdict: **redundant with §3.5 for post-ADR-024 features; only source for pre-ADR-024 features**.

### 3.7 `feature_status.json` `depends_on` (ADR-011)

Hard-parent dependency edges snapshotted per generation (`internal/workflow/patch_generations.go:120-146`). Two features with no `depends_on` edge between them are declared-independent by the operator; two features with a hard-parent edge are declared-cumulative in the sense that ancestor changes are expected in the descendant's context.

Verdict: **useful for opt-in policy but not sufficient alone** — absence of a declared edge does not mean absence of file overlap, and presence of a declared edge does not mean the canonical patches are actually cumulative on disk. This aligns with the M14.3 DAG topological ordering already applied at `reconcile.go:181-192`.

### 3.8 `claims.json`

Advisory path/dir ownership from PRD-feature-file-claims alpha-1 (`internal/cli/record_capture_modes.go:132+`). Present when the operator ran `tpatch feature claim add …` before recording. When claims exist and are disjoint across slugs, the canonical patches are almost certainly independent.

Verdict: **useful when present, uneven adoption**.

### 3.9 `tpatch land` trailer block (ADR-019)

Each landed commit carries `TpatchPatch: <sha>` and `TpatchRecipe: <sha>` trailers referencing the specific generation. Not directly relevant to reconcile derivation because reconcile reads the on-disk artifact, not commit trailers. Trailers do prove that landed features were expected to be one-per-commit, which is the strongest social contract for independence.

Verdict: **contextual — supports the "modern features are independent" claim but does not gate anything on disk**.

### 3.10 Summary of the audit

Combining §3.4 `touched_paths` and §3.5 `base_commit` from `patch-generations.json` provides a decisive independence signal for every feature recorded after ADR-024 landed (v0.10.0+). Older features may lack `patch-generations.json` entirely (`internal/store/patch_generations.go:88-95` returns an empty manifest when the file is missing). Because of that uneven coverage, this PRD's **v1** design uses an explicit opt-in flag for cumulative derivation and defers the metadata-driven auto-detect (§4.7 Design D7 "Future evolution") to a follow-up minor version once coverage is universal.

The audit conclusion:

- **No new persistent metadata is required.** `patch-generations.json` already carries the fields Option B would consume.
- **No new claim kind is required.** `claims.json` already carries advisory ownership.
- **No user-visible flag is required at record time.** The decision belongs at reconcile time, keyed off already-persisted provenance.

## 4. Design

### 4.1 D1 — Canonical `post-apply.patch` is the authoritative input for reconcile

**Decision**. Every reconcile phase — 1 (reverse-apply), 1.5 (patch-id sweep, already in place), 2 (upstream context extraction), 3 (3-way), 3.5 (provider-assisted resolve) — reads the feature's canonical `post-apply.patch` as its authoritative input, byte-identically in single-slug and multi-slug invocations. This is the invariant.

**Rationale**. The reporter's failure mode traces to a violation of this invariant: `reconcileFeature` at `reconcile.go:251-258` prefers `incremental.patch` over `post-apply.patch` whenever the derivation branch left an `incremental.patch` on disk, and the derivation branch runs unconditionally for `len(slugs) > 1`. The correct semantics is that `post-apply.patch` is authoritative and any incremental artifact is at most an optional companion for a very specific legacy shape.

**Consequence**. A future implementation slice must ensure that in the default multi-slug path, `reconcileFeature` reads `post-apply.patch` and the `incremental.patch` sidecar is not produced.

### 4.2 D2 — Cumulative derivation is default-OFF, opt-in via `--cumulative-legacy`

**Decision**. `deriveIncrementalPatches` runs only when the operator opts in with an explicit `--cumulative-legacy` flag on `tpatch reconcile`. In the default multi-slug path, the function is not called and no `incremental.patch` artifact is written.

**Rationale**. Independent canonical patches are the modern norm across every scoped and claimed capture mode (§3.1-§3.4). The historical cumulative case is rare, is not documented as a supported workflow (§3.9), and produces byte-corrupt artifacts when applied to independent patches (§2.2). Flipping the default matches the reporter's suggested remediation ("Until safely detectable, prefer canonical patches and require an explicit legacy/cumulative flag for GAP 4 derivation") and matches the ADR-030 semantic decision.

**Alternatives considered**. See ADR-030 for the enumeration of Options A (default OFF, opt-in flag), B (metadata-driven auto-detect), and C (deprecate entirely). Option A is the ADR-030 outcome and the design binding for this PRD.

**Sunset**. `--cumulative-legacy` is a candidate for removal once (a) D7 metadata-driven auto-detect ships and (b) telemetry or pilot survey shows the flag is unused for one minor cycle. Until then it stays as a documented opt-in.

**Consequence**. A behavioral change for any user or automation that today implicitly benefits from `incremental.patch` derivation on cumulative recordings. The `--cumulative-legacy` opt-in preserves the historical semantic for those users while making it explicit.

### 4.3 D3 — `--cumulative-legacy` invocations preserve the current derivation code path, plus D4's `.git/**` fix

**Decision**. When `--cumulative-legacy` is set, `deriveIncrementalPatches` and `gitutil.DeriveIncrementalPatch` run substantially as they do today, with two mandatory changes:

1. The `.git/**` leak is fixed at the source (D4 below) so `--cumulative-legacy` produces the intended delta without `.git/logs/**` or `.git/index` noise.
2. Every generated `incremental.patch` carries a header comment (e.g. `# tpatch: derived under --cumulative-legacy from <prev-slug> and <slug>`) so audit consumers can distinguish it from a canonical patch.

**Rationale**. Users who intentionally opt into cumulative derivation still deserve a clean artifact.

**Consequence**. The legacy path stays available; its output is safe to consume; and its provenance is machine-readable.

### 4.4 D4 — `.git/**` must never appear in any tpatch-produced patch

**Decision**. The delta-worktree helper in `gitutil.DeriveIncrementalPatch` must exclude `.git/**` at the diff boundary. Two changes are required:

1. Prefer `git diff --no-index --binary` invoked with `-- ':(exclude,glob).git/**' ':(exclude,glob).git'` between the two temp worktrees, so the diff engine natively excludes `.git/**` and produces canonical `a/`/`b/` prefixes without post-hoc path substitution.
2. If step 1 is not portable across git versions used in the pilot fleet, the fallback is `diff -ruN --exclude=.git prevDir currDir` and a subsequent path-prefix rewrite for any residual entries. In either fallback, a defensive filter rejects any hunk whose new-file or old-file path starts with `.git/`.

**Rationale**. `.git/**` is repository internals, not repository content. It must never enter a feature patch or a reconcile artifact regardless of whether the derivation path was intended. This is a stronger invariant than "fix the leak in the cumulative case" — it applies to any future diff-producing helper. Empirical reproduction (§0.2 row 7) shows that plain `diff -ruN` on two independent clones is guaranteed to produce `.git/logs/**` diffs, so the naive helper is unsafe even after D2 restricts its callers.

**Consequence**. `DeriveIncrementalPatch` gets a portable `.git`-exclusion contract. A parallel invariant (D5 below) prevents downstream code paths from writing `.git/**` entries even if a future helper regresses.

### 4.5 D5 — Post-write `.git/**` guard on every patch artifact

**Decision**. Every code path that writes a `.patch` artifact into `.tpatch/features/<slug>/artifacts/` runs a defensive check that rejects any patch whose file headers reference `.git/`, `.git\`, or the exact path `.git`. On rejection, the writer refuses with a diagnostic identifying the offending path and the calling helper.

**Rationale**. Belt and suspenders. D4 fixes the known leak; D5 catches any future regression at the store boundary before the corrupt artifact reaches the reconcile pipeline.

**Consequence**. The store gains a small validator invoked by `WriteArtifact` for `.patch` extensions (or a wrapper thereof). No new schema — the validator inspects the patch text.

### 4.6 D6 — Reconcile provenance records the derivation-mode decision

**Decision**. `reconcile-session.json` and any per-slug `reconcile.md` written for a multi-slug run include a machine-readable `derivation_mode` field with the possible values:

- `"canonical"` — default multi-slug path, `incremental.patch` was not written, canonical `post-apply.patch` used verbatim.
- `"cumulative-legacy"` — opt-in path, `incremental.patch` was produced by cumulative subtraction.
- `"single-slug"` — the invocation had only one slug and the branch was not entered.

`reconcile-evidence.jsonl` entries (ADR-025) include the same field so the downstream consumer can filter or refuse based on it.

**Rationale**. Auditors and future revisions need to know which semantic was in effect. ADR-025's evidence schema is the natural home; a paper-only PRD does not extend it, but calls out the field so the implementation slice knows to include it.

**Consequence**. One new field in reconcile artifacts; no new files or top-level schema versions.

### 4.7 D7 — Future evolution: metadata-driven auto-detect (Option B)

**Decision** (deferred to a follow-up PRD or minor version, not implemented in v1). Once `patch-generations.json` coverage is universal across the pilot fleet, `RunReconcile` may opt into a metadata-driven auto-detect mode that computes derivation using the pathspec-and-touched-paths independence proof from §3.4-§3.5.

Sketch:

1. For every slug in the multi-slug set, load `patch-generations.json` and read the latest generation's `base_commit` and `touched_paths`.
2. If every slug has a `patch-generations.json` and every pair of slugs is either (a) disjoint on `touched_paths` or (b) linked by a hard-parent `depends_on` edge, treat all patches as independent (equivalent to §4.1 D1).
3. If any slug lacks the manifest, or any pair overlaps without a declared dependency edge, fall back to canonical `post-apply.patch` and emit a warning identifying the ambiguity.

**Rationale**. Auto-detect avoids user-visible flags for the common case once metadata is universal. Deferring it keeps v1 simple and matches the "prefer canonical patches" invariant while future work builds the safe detection surface.

**Consequence**. None in v1. A future PRD may promote this design element from D7 into a D1..D6 of its own.

### 4.8 D8 — `--cumulative-legacy` is mutually exclusive with the DAG-topological reorder

**Decision**. When `--cumulative-legacy` is set, the input slug order is preserved verbatim and the ADR-011 D9 topological reorder at `reconcile.go:181-192` is skipped. The reason: cumulative derivation depends on `prevCumulative` being the exact previous slug in the caller's ordering; if the DAG reorderer inserts a hard-parent between named slugs, the cumulative math no longer matches the operator's intent.

**Rationale**. The two features (topological reorder and cumulative derivation) speak to two different semantics. Cumulative derivation is opt-in for legacy workflows that already control ordering by hand. When the operator opts in, tpatch trusts their ordering.

**Consequence**. A three-line skip in `RunReconcile` when the legacy flag is set. Documented in the flag's `--help` text.

### 4.9 D9 — `--cumulative-legacy` disables phase 1.5 patch-id detection

**Decision**. When `--cumulative-legacy` is set, phase 1.5 is skipped for the run and a note is attached to each `ReconcileResult`.

**Rationale**. Phase 1.5 already explicitly loads canonical `post-apply.patch` because the derived `incremental.patch` produces false-positive retirements (`reconcile.go:283-293`). Under `--cumulative-legacy`, the derivation is the whole point; using the canonical form for the detector while the rest of the pipeline uses the incremental form would mix semantics. Skipping the detector under legacy mode keeps the run self-consistent and preserves the pre-M17-Wave-D behavior the legacy path was written for.

**Consequence**. Users on legacy cumulative workflows see reconcile output that matches their prior expectations. Users on modern independent workflows retain phase 1.5.

### 4.10 D10 — Migration diagnostic when default-canonical trips over a legacy-recorded stack

**Decision**. When the default multi-slug reconcile fails phase 1 (reverse-apply) on slug N, and `patch-generations.json` shows that any earlier slug in the run touched a subset of N's `touched_paths` (i.e. the earlier canonical patch overlaps N's canonical patch on N's own file set), the run must emit a diagnostic hint of the shape:

> `hint: prior features may have been recorded cumulatively; retry with --cumulative-legacy (see ADR-030)`.

The hint is advisory — the phase 1 failure surfaces normally with its own diagnostics, the run does not silently retry, and the exit code is unchanged relative to the phase 1 failure.

**Rationale**. This is the biggest UX risk of flipping the default from cumulative to canonical: an operator with a legacy cumulative-recorded stack will see a new phase 1 failure with no obvious migration path. The overlap-on-touched-paths check is a cheap, high-precision signal (§3.4 `touched_paths` is already populated on every generation post-ADR-024), and it triggers only in the exact shape that legacy cumulative recording produces. False positives cost one line of stderr; false negatives leave the operator with the pre-remediation surprise.

**Consequence**. The multi-slug default path grows one post-phase-1 diagnostic hook that consults `patch-generations.json` for each earlier slug and emits the hint when the overlap check fires. The hint is text-only and does not participate in evidence.

## 5. Safety invariants

The following invariants are load-bearing across the design. They are stated as `INV-N` so the implementation slice can cite them in tests and code comments.

**Ordering note (rev-1)**. INV-1 and INV-2 apply *after* the Wave β later-touch warning-attachment pass at `internal/workflow/reconcile.go:207` (`DetectReconcileLaterTouchWarningsByOwner`) and the ADR-029 write-file-recipe-safety warning attachment run. Wave β and ADR-029 attach warnings to `ReconcileResult` records based on canonical `post-apply.patch` content; the canonical-vs-cumulative invariants below govern which artifact is *read* for phases 1/2/3/3.5 after those warnings have already been composed. Implementations must not re-order the pipeline so that INV-1/INV-2 gate warning composition — the warnings run first, the derivation-mode invariant governs the phase inputs second.

- **INV-1**. Canonical `post-apply.patch` for any feature is byte-identical between single-slug reconcile and default multi-slug reconcile. Any code path that reads a different artifact for the same feature under multi-slug default is a bug.
- **INV-2**. `incremental.patch` is written only under `--cumulative-legacy`. Under any other invocation, the file is not created; if it exists on disk from a prior legacy run, it is not read by the default path (§4.1 D1 rewrite of `reconcileFeature`).
- **INV-3**. No tpatch-produced patch artifact contains a `.git/`, `.git\`, or exact-`.git` path in any `+++`, `---`, `diff --git`, or `Only in` header.
- **INV-4**. The derivation-mode field in reconcile provenance faithfully reports the code path taken. `"canonical"` and `"cumulative-legacy"` are the only legal values in a multi-slug run.
- **INV-5**. Under `--cumulative-legacy`, the input slug order is preserved (§4.8 D8) and phase 1.5 is skipped (§4.9 D9).
- **INV-6**. `.git/**` exclusion is applied at the diff boundary (§4.4 D4) **and** at the store write boundary (§4.5 D5). Either boundary alone is a defense in depth failure.

## 6. Acceptance criteria

- **AC-1**. `tpatch reconcile A B C --upstream-ref upstream/main` on the reporter's three independent-feature stack produces no `incremental.patch` artifact for any of A, B, C.
- **AC-2**. In the same invocation, `reconcile.md` diagnostics for slug C reference C's touched paths (WSL files) only; no reference to slug B's paths appears.
- **AC-3**. Phase 1 (reverse-apply) for each of A, B, C reads canonical `post-apply.patch` byte-identically to the same command run one slug at a time.
- **AC-4**. `tpatch reconcile A B C --upstream-ref upstream/main --cumulative-legacy` produces `incremental.patch` artifacts for A (equal to A's canonical patch), B (containing only B's cumulative contribution), and C (containing only C's cumulative contribution). None of the three contains any `.git/**` path in any header.
- **AC-5**. `tpatch reconcile A --upstream-ref upstream/main` (single-slug) is unchanged: it reads `post-apply.patch`, does not enter the derivation branch, and produces no `incremental.patch`.
- **AC-6**. `tpatch reconcile A B C` on a stack that has been recorded cumulatively (each slug's canonical patch is a superset of the previous slug's canonical patch) without `--cumulative-legacy` still uses canonical patches; if reverse-apply consequently fails for later slugs, the failure surfaces normally without silent corruption.
- **AC-7**. Writing a patch artifact whose header contains `.git/` or `.git` refuses at the store boundary, with a diagnostic naming the writer and the offending path.
- **AC-8**. `gitutil.DeriveIncrementalPatch` invoked with two temp worktrees that both contain a `.git/` directory produces zero `.git/**` entries in the returned patch text.
- **AC-9**. `reconcile-session.json` records `derivation_mode: "canonical"` for the default multi-slug path, `derivation_mode: "cumulative-legacy"` for `--cumulative-legacy`, and `derivation_mode: "single-slug"` for a one-slug run.
- **AC-10**. `--cumulative-legacy` is documented in `tpatch reconcile --help` with a one-line pointer to this PRD and ADR-030.
- **AC-11**. Under `--cumulative-legacy`, the ADR-011 D9 dependency-DAG reorder is skipped (INV-5) and the input slug order is preserved verbatim.
- **AC-12**. Under `--cumulative-legacy`, phase 1.5 patch-id detection is skipped and a note "phase 1.5 skipped: --cumulative-legacy" is attached to each ReconcileResult.
- **AC-13**. Documentation for `tpatch reconcile` (`docs/reconcile.md`) explains that canonical patches are independent by default and that `--cumulative-legacy` is required for the historical cumulative-recording pattern.
- **AC-14**. Existing single-slug reconcile tests, ADR-011 D9 DAG-order tests, ADR-025 evidence tests, ADR-028 supersession-filter tests, ADR-029 / PRD-write-file-recipe-safety later-touch detector tests, and M17 Wave D phase-1.5 detector tests stay green.
- **AC-15**. When default multi-slug reconcile fails phase 1 on slug N and `patch-generations.json` shows any prior slug in the invocation touched a subset of N's `touched_paths`, the run emits the D10 diagnostic `hint: prior features may have been recorded cumulatively; retry with --cumulative-legacy (see ADR-030)` to stderr. The hint is text-only, does not alter exit code, and does not silently retry.

## 7. Test scenarios

The implementation slice must include at least these scenarios. IDs are `TS-N`.

- **TS-1 — Reporter's ABC repro (default)**. Three features A/B/C with independent canonical patches. `tpatch reconcile A B C --upstream-ref <ref>` produces correct phase-2/3 verdicts for each; no `incremental.patch` on disk; no cross-contamination in `reconcile.md`.
- **TS-2 — Reporter's ABC repro under `--cumulative-legacy`**. Same setup as TS-1 with `--cumulative-legacy` added. Verify each incremental patch is the intended per-slug delta (byte-equal to canonical for TS-1's independent inputs; TS-2 documents that legacy mode on truly-independent inputs is a caller error but does not corrupt state — the derived deltas may equal canonical or be empty depending on order).
- **TS-3 — Truly cumulative recording under `--cumulative-legacy`**. Two slugs A' and B' where A' touches `foo.txt` with change X and B' touches `foo.txt` with change X∪Y. Under `--cumulative-legacy`, B's `incremental.patch` reflects only Y. Under default, the same invocation uses canonical B' (X∪Y) and reverse-apply behavior surfaces normally.
- **TS-4 — Delta-worktree `.git/**` isolation**. Direct unit test on `gitutil.DeriveIncrementalPatch`: call it with two temp dirs that each have a `.git/logs/HEAD` with distinct timestamps. Assert the returned patch text contains zero occurrences of `.git/logs`, `.git/index`, or `Only in .git`.
- **TS-5 — Store-boundary `.git/**` refusal**. Attempt to write a patch text containing `+++ b/.git/logs/HEAD` via the store's patch-writer. Assert refusal with a specific diagnostic and no file on disk.
- **TS-6 — DAG reorder skipped under legacy flag**. Config enables the DAG. Two slugs with a hard-parent edge are passed to `tpatch reconcile B A --cumulative-legacy` (i.e., child-first order). Assert the reorder is skipped and slugs are processed in the input order `B, A`.
- **TS-7 — Phase 1.5 skipped under legacy flag**. Config enables phase 1.5 detector. `tpatch reconcile A --cumulative-legacy` skips phase 1.5 and records the skip note.
- **TS-8 — Provenance field emitted**. `reconcile-session.json` and any per-slug `reconcile.md` written by a multi-slug default run contain `derivation_mode: "canonical"`. Legacy run contains `derivation_mode: "cumulative-legacy"`. Single-slug run contains `derivation_mode: "single-slug"`.
- **TS-9 — Single-slug byte identity**. Run `tpatch reconcile A --upstream-ref <ref>` and capture all bytes of `reconcile.md`, `reconcile-session.json`, and any evidence file. Confirm byte-identical output to the pre-implementation baseline (modulo the `derivation_mode: "single-slug"` addition, which is a strict addition and only present if the implementation slice extends the schema).
- **TS-10 — Multi-slug default byte-equals per-slug loop**. Run `tpatch reconcile A B C` under the new default and separately run `tpatch reconcile A`, `tpatch reconcile B`, `tpatch reconcile C`. Compare phase verdicts and touched-path sets. Confirm the multi-slug default produces the same phase verdicts as the per-slug loop (this is the correctness statement that motivates the whole PRD).
- **TS-11 — Empty stack under legacy flag**. `tpatch reconcile --cumulative-legacy` with no slugs and no default effective set: refuse with the same "no features to reconcile" error the default path emits. The legacy flag does not weaken input validation.
- **TS-12 — Superseded slug under both modes**. Superseded slug named explicitly by the caller: historical-feature warning attaches to the `ReconcileResult` under both default and `--cumulative-legacy` (ADR-028 D6 semantics are orthogonal to derivation mode).

## 8. Non-goals

1. Rewriting `deriveIncrementalPatches` to natively handle scoped/claimed features. The design (D2 D3) preserves it as an opt-in legacy path; a native scoped-aware alternative is deferred to D7 auto-detect.
2. Deprecating the cumulative recording pattern outright (Option C in ADR-030). Some workflows may still choose it deliberately.
3. Rewriting existing `incremental.patch` artifacts on disk for pre-remediation reconciles. The remediation is forward-looking; audit of past reconciles remains the reporter's responsibility.
4. Adding a new capture mode to `patch-generations.json` to explicitly label "independent." The audit in §3 shows the existing fields carry the signal.
5. Extending `patch-generations.json` schema in this PRD. D6 provenance lives in reconcile-session/reconcile-evidence.
6. Adding a `tpatch reconcile --canonical` inverse of `--cumulative-legacy`. Canonical is the default; a redundant explicit flag would be a source of confusion.
7. Provider-assisted derivation selection (e.g., "ask the model which mode to use"). The decision is deterministic — either the operator opts in or the invariant applies.
8. Retroactive migration of feature histories that lack `patch-generations.json`. The v1 design is compatible with either the manifest present or absent because the default path does not consult the manifest.

## 9. Open questions

1. Should `--cumulative-legacy` be recorded in reconcile evidence with a `-legacy` suffix on any downstream verdict identifier, or only in the `derivation_mode` field? Deferred to the implementation slice's evidence-schema review.
2. Should the store-boundary `.git/**` guard (D5) also refuse patches whose `diff --git a/.git/…` line matches without a `+++`/`---` header, in case a future patch producer emits a mode-only or rename-only stanza? Recommendation: yes, add a coverage line for `diff --git` too, but the exact regex is implementation detail.
3. Should the future auto-detect (D7) require the `depends_on` edge to also carry a `parent_generation` snapshot, or is any hard-parent edge sufficient? Deferred to the follow-up PRD that promotes D7 into a design element.
4. Should `--cumulative-legacy` also disable ADR-028 supersession filtering, or should filtering still apply? Recommendation: filter still applies (§4.8 rationale is about derivation, not effective-set membership). Confirm at three-way review.
5. ~~Should legacy-mode invocations emit a top-of-run warning that recommends the workaround-then-migrate path? Recommendation: yes, one-line stderr note pointing at this PRD.~~ **Resolved rev-1 by §4.10 D10 / AC-15**: the migration diagnostic is a numbered design decision. The hint fires from the *default* path after a phase 1 failure when overlap on `touched_paths` proves the earlier slug was recorded cumulatively — a stronger signal than the top-of-run advisory this OQ contemplated.

## 10. Sources

- GH #3 body, verbatim reproduction in §2.1 and §2.2. Fetched 2026-07-31.
- `internal/workflow/reconcile.go:195-199` — multi-slug dispatch.
- `internal/workflow/reconcile.go:251-258` — `reconcileFeature` incremental preference.
- `internal/workflow/reconcile.go:283-293` — phase 1.5 canonical carve-out.
- `internal/workflow/reconcile.go:1145-1174` — `deriveIncrementalPatches`.
- `internal/gitutil/gitutil.go:941-999` — `DeriveIncrementalPatch` delta-worktree helper.
- `internal/store/patch_generations.go:37-79` — patch-generation schema.
- `internal/workflow/patch_generations.go:82-146` — patch-generation population.
- `internal/workflow/refresh.go:74-93` — reconcile-mode generation append.
- `internal/cli/record_capture_modes.go:32-38` — capture-mode enum.
- `internal/cli/cobra.go:1522-1530` — capture population site in `record`.
- [ADR-011 — Feature dependency DAG](../adrs/ADR-011-feature-dependencies.md), D9 for topological ordering.
- [ADR-024 — Patch generation manifest boundary](../adrs/ADR-024-patch-generation-manifest-boundary.md), D1-D2 for schema pattern.
- [ADR-025 — Reconcile evidence and revision schema](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md), D2-D3 for evidence pattern.
- [ADR-028 — Supersession edge model](../adrs/ADR-028-supersession-edge-model.md), D6 for supersession filtering.
- [ADR-030 — Multi-slug reconcile derivation mode](../adrs/ADR-030-multi-slug-reconcile-derivation-mode.md), semantic lock for D2.
- [PRD-record-capture-modes](./PRD-record-capture-modes.md), §3.7 for mutex vocabulary.
- [PRD-feature-file-claims](./PRD-feature-file-claims.md), alpha-1 for advisory claim primitive.
- [PRD-feature-patch-identity-metadata](./PRD-feature-patch-identity-metadata.md), for `patch-generations.json` design intent.
- [PRD-tpatch-land](./PRD-tpatch-land.md), for one-feature-per-commit trailer contract.
- [PRD-write-file-recipe-safety](./PRD-write-file-recipe-safety.md), sibling PRD in the replay-trustworthiness cluster.
- Empirical `.git/logs/**` leak reproduction, this session: `git clone --no-checkout` twice + `git checkout <sha>` on each + `diff -ruN` deterministically diffs `.git/logs/HEAD`, `.git/logs/refs/heads/main`, `.git/logs/refs/remotes/origin/HEAD`, and binary `.git/index`.
