# ADR-030 — Multi-slug Reconcile Derivation Mode

**Status**: Accepted
**Date**: 2026-07-31
**Owner**: Core
**Cluster**: Replay trustworthiness (GH #3)
**Supersedes**: none
**Superseded by**: none
**Depends on**: [ADR-011](./ADR-011-feature-dependencies.md), [ADR-024](./ADR-024-patch-generation-manifest-boundary.md), [ADR-025](./ADR-025-reconcile-evidence-and-revision-schema.md), [ADR-028](./ADR-028-supersession-edge-model.md)
**Blocks**: [PRD-multi-slug-reconcile-canonical-safety](../prds/PRD-multi-slug-reconcile-canonical-safety.md) implementation

## Context

GH #3 reports that `tpatch reconcile <slug-a> <slug-b> …` unconditionally treats every non-first slug's canonical `post-apply.patch` as a cumulative diff (`internal/workflow/reconcile.go:1145-1148`) and derives a per-slug `incremental.patch` by subtracting the previous slug's canonical patch (`reconcile.go:1149-1174`). The derivation uses `git clone --no-checkout` twice, then `git checkout <baseCommit>` on each clone, then plain GNU `diff -ruN` over the two directory trees (`internal/gitutil/gitutil.go:946-999`).

The assumption behind the derivation — "features A and B applied sequentially means B's cumulative patch includes A's changes" — was written when tpatch had only a single default capture mode (`working-tree-all`) and expected all recording to happen on a branch where earlier features had already been applied. It does not hold under the scoped and claimed capture pattern that ships today (`internal/cli/record_capture_modes.go:32-38`, `PRD-feature-file-claims`, `PRD-tpatch-land`), where each feature's canonical patch is independently authored against a known base.

Two failure modes fall out of the current default when canonical patches are independent:

1. **Cumulative subtraction on independent siblings** — `deriveIncrementalPatches` writes an `incremental.patch` whose content is `curr − prev` rather than `slug's own contribution`. For independent inputs, `curr − prev` can include negative hunks that remove earlier slugs' files (the reporter's C-cancels-B pathology).
2. **`.git/**` leak from plain `diff -ruN`** — the delta-worktree helper's `diff -ruN prevDir currDir` diffs every path under both temp clones, including `.git/logs/**` and `.git/index`. Reflog timestamps are wall-clock and index bytes differ between clones, so the leak is deterministic. Reproduced empirically during PRD drafting.

The reconcile pipeline downstream (`reconcileFeature` at `reconcile.go:251-258`) prefers `incremental.patch` over `post-apply.patch` whenever both exist. So the corrupted delta is what phases 1, 2, 3, and 3.5 evaluate. Phase 1.5 already carves itself out and re-reads canonical `post-apply.patch` for exactly this reason (`reconcile.go:283-293`, cited: "the incremental form may match a partial absorption that isn't a real merge, producing a false-positive retire path").

This ADR is paper-only. It changes no code, schema, CLI behavior, assets, PRD body, handoff, release artifact, or CHANGELOG entry.

## Decision

### D1 — Derivation is default-OFF; canonical `post-apply.patch` is authoritative in multi-slug reconcile

`tpatch reconcile <slug-a> <slug-b> …` uses each feature's canonical `post-apply.patch` as its authoritative input, byte-identical to a single-slug invocation. `deriveIncrementalPatches` does not run in the default multi-slug path and `incremental.patch` is not written.

**Rationale**. Independent canonical patches are the modern norm across every scoped and claimed capture mode. Cumulative subtraction on independent inputs produces byte-corrupt artifacts (`reconcile.go:1149-1174` semantics on independent siblings). Making canonical the default matches the reporter's suggested remediation and is the smallest behavior change that fixes the reported class of failures without disturbing single-slug reconcile, DAG topological order, supersession filtering, or phase 1.5.

### D2 — `--cumulative-legacy` opt-in preserves the historical cumulative semantic

Operators who explicitly opted into the cumulative-recording pattern (record everything on a branch where prior features are already applied) may re-enable derivation with a `tpatch reconcile … --cumulative-legacy` flag. Under this flag, `deriveIncrementalPatches` runs and `incremental.patch` is produced.

**Rationale**. Some pilots may already rely on the cumulative behavior. Deprecating it outright (Option C in the enumeration below) would break those workflows without a migration story. A named opt-in preserves the semantic while making its presence explicit and auditable.

**Sunset**. `--cumulative-legacy` is a candidate for removal once (a) D8 metadata-driven auto-detect ships and (b) telemetry or pilot survey shows the flag is unused for one minor cycle. The flag stays a documented opt-in until both conditions hold; removal is a separate ADR revision.

### D3 — `.git/**` exclusion is enforced at the diff boundary

`gitutil.DeriveIncrementalPatch` must exclude `.git/**` at the diff boundary. Preferred implementation: `git diff --no-index --binary -- ':(exclude,glob).git/**' ':(exclude,glob).git'` between the two temp worktrees. Fallback: `diff -ruN --exclude=.git prevDir currDir` plus a defensive filter that rejects any hunk header referencing `.git/`.

**Rationale**. `.git/**` is repository internals, not repository content. The leak is deterministic under plain `diff -ruN` because `git clone --no-checkout` and `git checkout` both write to `.git/logs/*` with wall-clock-second timestamps that differ between clones. Fixing the leak at the diff boundary prevents the primary regression path. D4 provides defense in depth.

### D4 — `.git/**` refusal is enforced at the store write boundary

Every code path that writes a `.patch` artifact into `.tpatch/features/<slug>/artifacts/` runs a defensive check that rejects any patch whose file headers reference `.git/`, `.git\`, or the exact path `.git`. On rejection, the writer refuses with a diagnostic identifying the writer and the offending path.

**Rationale**. Belt and suspenders. D3 fixes the known leak; D4 catches any future regression at the store boundary before the corrupt artifact reaches the reconcile pipeline. The invariant is stronger stated at both boundaries.

### D5 — Reconcile provenance carries a `derivation_mode` field

`reconcile-session.json`, `reconcile.md`, and `reconcile-evidence.jsonl` entries include a `derivation_mode` field with values:

- `"canonical"` — default multi-slug path (D1).
- `"cumulative-legacy"` — opt-in path (D2).
- `"single-slug"` — the invocation had one slug and the derivation branch was not entered.

**Rationale**. Auditors need to know which semantic was in effect. ADR-025's evidence schema is the natural home. This ADR does not extend the ADR-025 schema; it declares the field so the ADR-025 next-revision cycle knows to include it.

### D6 — `--cumulative-legacy` disables the DAG-topological reorder for the run

Under `--cumulative-legacy`, the ADR-011 D9 topological reorder at `reconcile.go:181-192` is skipped and the input slug order is preserved verbatim.

**Rationale**. Cumulative derivation depends on `prevCumulative` being the exact previous slug in the caller's ordering. If the DAG reorderer inserts or reorders siblings between named slugs, the cumulative math no longer matches the operator's intent. Skipping the reorder under legacy mode trusts the operator's ordering.

### D7 — `--cumulative-legacy` disables phase 1.5 patch-id detection

Under `--cumulative-legacy`, phase 1.5 is skipped for the run and a note is attached to each `ReconcileResult`.

**Rationale**. Phase 1.5 already loads canonical `post-apply.patch` because the derived incremental form produces false-positive retirements (`reconcile.go:283-293`). Under `--cumulative-legacy`, the derivation is the whole point; using the canonical form for the detector while the rest of the pipeline uses the incremental form would mix semantics. Skipping the detector under legacy mode keeps the run self-consistent.

### D8 — Future evolution (Option B, deferred)

Once `patch-generations.json` coverage is universal, a follow-up PRD may introduce a metadata-driven auto-detect mode that infers independence from the manifest's `capture.mode`, `capture.pathspecs`, `capture.claim_ids`, `touched_paths`, and `base_commit` fields (see the audit in PRD §3). Auto-detect avoids user-visible flags for the common case but is deferred to keep v1 simple and to match the "prefer canonical patches" invariant until the safe detection surface is provable.

## Consequences

- Multi-slug reconcile behavior changes for any user or automation that today implicitly benefited from `deriveIncrementalPatches` on cumulative-recorded stacks. The `--cumulative-legacy` opt-in preserves the historical semantic.
- `gitutil.DeriveIncrementalPatch` gains a portable `.git`-exclusion contract at the diff boundary (D3) and downstream writers gain a defensive refusal at the store boundary (D4).
- Reconcile artifacts gain a `derivation_mode` field (D5) that participates in the ADR-025 evidence schema next revision.
- `--cumulative-legacy` invocations are self-contained: DAG reorder off (D6), phase 1.5 off (D7), ordering preserved verbatim.
- `deriveIncrementalPatches` and `DeriveIncrementalPatch` remain in the tree as the opt-in path; they are not removed.

## Alternatives considered

1. **Option A (chosen) — Default OFF, opt-in `--cumulative-legacy`**. Canonical patches are authoritative by default; legacy semantic preserved behind a named flag. Matches the reporter's remediation suggestion. Minimal behavior change beyond the reported failure class.
2. **Option B — Metadata-driven auto-detect**. Derive when `patch-generations.json` proves patches are cumulative (shared base, overlapping touched paths, declared dependency edge). Rejected for v1 because `patch-generations.json` coverage is not yet universal — legacy features may have no manifest, and falling back to canonical for missing manifests reduces to Option A in practice. Deferred to a follow-up PRD once coverage is universal (D8).
3. **Option C — Deprecate cumulative derivation entirely**. Remove `deriveIncrementalPatches` and `DeriveIncrementalPatch`; document one-feature-per-invocation as the only supported multi-slug semantics. Rejected: some pilots may depend on cumulative derivation and a hard removal has no migration story. The named opt-in gives us the option to deprecate later after usage data.
4. **Fix the leak but keep the default cumulative** (`.git/**` exclusion + comment about assumption). Rejected: leaves the C-cancels-B independent-sibling pathology in place. The leak is the tractable half of the bug; the cumulative-subtraction-on-independent-siblings semantic is the underlying failure and must be addressed.
5. **New capture-mode label `independent`** and gate derivation off that. Rejected: the audit in PRD §3.1 shows that `capture.mode` is a provenance label, not an independence proof. `working-tree-all` in particular is written for the default `record` and cannot promise independence. A new label duplicates fields already carried by `touched_paths` and `base_commit`.
6. **Provider-assisted derivation selection** (ask the model which mode to use). Rejected: the decision is deterministic — either the operator opts in or the invariant applies. Provider assistance adds cost and non-determinism for a well-defined semantic choice.

## References

- GH #3 — multi-slug reconcile corrupts independent canonical patches via cumulative delta derivation.
- [PRD-multi-slug-reconcile-canonical-safety](../prds/PRD-multi-slug-reconcile-canonical-safety.md).
- [ADR-011 — Feature dependency DAG](./ADR-011-feature-dependencies.md), D9 topological ordering.
- [ADR-024 — Patch generation manifest boundary](./ADR-024-patch-generation-manifest-boundary.md), D1-D2 schema precedent.
- [ADR-025 — Reconcile evidence and revision schema](./ADR-025-reconcile-evidence-and-revision-schema.md), D2-D3 evidence pattern.
- [ADR-028 — Supersession edge model](./ADR-028-supersession-edge-model.md), D6 filter interaction.
- [PRD-record-capture-modes](../prds/PRD-record-capture-modes.md), §3.7 mutex vocabulary.
- [PRD-feature-file-claims](../prds/PRD-feature-file-claims.md), advisory claim primitive.
- [PRD-feature-patch-identity-metadata](../prds/PRD-feature-patch-identity-metadata.md), `patch-generations.json` intent.
- `internal/workflow/reconcile.go:195-199` — multi-slug dispatch.
- `internal/workflow/reconcile.go:251-258` — `reconcileFeature` incremental preference.
- `internal/workflow/reconcile.go:283-293` — phase 1.5 canonical carve-out.
- `internal/workflow/reconcile.go:1145-1174` — `deriveIncrementalPatches`.
- `internal/gitutil/gitutil.go:946-999` — `DeriveIncrementalPatch`.
- `internal/store/patch_generations.go:37-79` — patch-generation schema.
- `internal/cli/record_capture_modes.go:32-38` — capture-mode enum.
- Empirical `.git/**` leak reproduction, this session: two `git clone --no-checkout` + `git checkout <sha>` clones diffed with `diff -ruN` deterministically produce `.git/logs/HEAD`, `.git/logs/refs/heads/main`, `.git/logs/refs/remotes/origin/HEAD`, and binary `.git/index` differences.
