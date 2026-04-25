# ADR-010 — Provider-Assisted Conflict Resolution

**Status**: Accepted (pending PRD-resolver — implementation is the v0.5.0 headline)
**Date**: 2026-04-20
**Deciders**: Core
**Supersedes**: n/a
**Related**: `feat-provider-conflict-resolver`, ADR-004 (merge strategy), SPEC §Reconcile

## Context

The v0.4.1 → v0.4.2 stress test made one thing clear: phase-4 of `tpatch reconcile` is the feature that distinguishes tpatch from plain git. Today phase-3 (`providerSemanticCheck`) only answers "is this feature upstreamed?" — a boolean obsolescence check. When the 3-way merge fails, tpatch reports `3WayConflicts` (new in v0.4.2) and hands control back to the human. That is correct but incomplete: the promise of the product is that **the user should not worry about how to apply the changes**.

A4 shipped the honest verdict (`3WayConflicts` instead of a false "reapplied"). ADR-010 covers what happens next: a new phase **3.5** between the obsolescence check and the forward-apply bailout in which the provider proposes resolutions for every conflicted file, we validate them, and the user accepts or rejects atomically.

## Decision

### D1. Introduce phase 3.5 `providerConflictResolve`

Ordering in reconcile becomes:

```
phase-1  reverse-apply   (does our patch still come off the old upstream cleanly?)
phase-2  operation-level replay (dry-run the recipe against new upstream)
phase-3  provider semantic check (obsolescence)
phase-3.5 provider conflict resolve  ← NEW
phase-4  forward-apply + verdict
```

Phase 3.5 runs **only** when phase-4 would otherwise produce `3WayConflicts`. It is opt-in via `--resolve` so the default reconcile behaviour stays honest-and-safe; CI/CD pipelines that want autonomy flip `--resolve --apply`.

### D2. Shadow worktree as the unit of atomicity

The resolver writes into a throwaway `git worktree` under `.tpatch/shadow/<slug>-<timestamp>/`, not into the real working tree. This gives us:

- Atomicity — resolver output is either wholly accepted (`tpatch reconcile --accept <slug>`) or wholly discarded (`--reject`).
- Auditability — the shadow survives until explicit accept/reject so a human can diff it.
- Safety — user-authored code in the real tree is never touched until accept.

Shadow worktrees are pruned automatically on accept/reject and on any subsequent `tpatch reconcile` run for the same slug (previous shadow from the same slug is reaped before a new one is created).

### D3. Per-file provider call, not whole-tree

Each conflicted file is resolved independently with a prompt that receives:

- `base` — the original upstream version of the file.
- `ours` — our patched version.
- `theirs` — the new upstream version.
- `spec_excerpt` — feature intent from `spec.md`.
- `exploration_excerpt` — relevant grounding from `exploration.md`.

System prompt: *"You are resolving a 3-way merge conflict. Your goal is to preserve BOTH intents: the feature intent (from spec.md) AND the upstream change. Output the resolved file content only, no commentary."*

Per-file isolation keeps token cost bounded, lets us parallelise, and lets us mark individual files as `resolved | validation-failed | skipped` in the resolution report.

### D4. Mandatory validation gate before accept is allowed

Each resolved file must:
1. Parse in its native language (Go via `go/parser`, TS via `tsc --noEmit`, others via configured `syntax_check` command).
2. Contain **no** conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`).
3. Preserve identifiers referenced from both `ours` and `theirs`.
4. If the repo has a `test_command` configured, it must pass in the shadow worktree.

Files that fail validation stay in the shadow with their proposed content for human review, but the verdict becomes `blocked-requires-human`. Accept is refused until the shadow is clean OR the user passes `--accept --partial` (documented escape hatch).

### D5. DERIVED artifact refresh is atomic and only triggered on accept

Accept is the single transition point. On `tpatch reconcile --accept <slug>`:

- Copy the shadow worktree over the real working tree (feature files only; never `.git/`).
- Regenerate DERIVED artifacts atomically via `store.RefreshDerivedArtifacts(slug, newPatch, newUpstreamCommit)`:
  - `artifacts/post-apply.patch`, `artifacts/incremental.patch`, `artifacts/apply-recipe.json`
  - `record.md` metadata block (Files Changed, Diff Stat, Upstream Commit, Reconciled At)
  - `patches/NNN-reconcile.patch` — numbered snapshot with label `reconcile`
  - `status.json` — `last_command=reconcile`, new `upstream_commit` field, bump `updated_at`. **Source of current truth post-accept** (e.g., `Reconcile.Outcome=reapplied` after a successful manual accept).
- **Artifact ownership** (clarified post-v0.5.3 — split during Tranche C3):
  - `artifacts/resolution-session.json` — **resolver-owned**. Per-file `outcomes[]` from one provider-resolver invocation. Read by `loadResolvedFiles` (manual `reconcile --accept` and `--shadow-diff`).
  - `artifacts/reconcile-session.json` — **reconcile-owned**. High-level `ReconcileResult` (verdict, phase, upstream ref, notes, cost) for one `RunReconcile` invocation. **Audit record of the invocation, not the post-accept state** — manual accept does not rewrite this artifact; `status.json` is the source of current truth. Re-running `reconcile` overwrites it.
  - The pre-v0.5.3 design (single `reconcile-session.json` containing both summary and per-file outcomes) caused a dual-writer schema collision; the split removes that risk.
- INTENT artifacts (`request.md`, `analysis.md`, `spec.md`, `exploration.md`, record.md narrative sections) are **never** touched by reconcile.
- If any step of the atomic refresh fails, nothing is written and the shadow is preserved.

### D6. Spec drift is out of scope — warn and log

If `spec.md` references line numbers, function names, or file paths that the provider resolution invalidates, we **warn** on accept (`spec.md references N files that were modified; consider tpatch analyze --refresh <slug>`) and log `feat-spec-drift-detection` for later. Phase 3.5 does not silently rewrite intent documents.

### D7. Cost controls

- Default cap: **10** conflicted files per reconcile. Beyond that, verdict is `blocked-too-many-conflicts` with a suggestion to split the feature (`feat-feature-decomposition`). Override via `--max-conflicts N`.
- `--model <name>` override on reconcile — use a bigger model just for conflict resolution without bloating the default provider config.
- Streaming (`feat-streaming-responses`) is desirable but NOT a prerequisite; the resolver works with blocking calls at v0.5.0.

### D8. UX surface

```
tpatch reconcile <slug>                                    # no resolution — stops at 3WayConflicts (today's behaviour)
tpatch reconcile <slug> --resolve                          # enable phase 3.5; stops before copying shadow → real tree
tpatch reconcile <slug> --resolve --apply                  # resolve + auto-accept if validation + tests pass
tpatch reconcile --accept <slug>                           # commit the shadow to the real tree (triggers D5 refresh)
tpatch reconcile --reject <slug>                           # discard the shadow worktree
tpatch reconcile --shadow-diff <slug>                      # show the shadow vs. current tree diff for review
```

State machine addition: `reconciling-shadow` (shadow exists, awaiting accept/reject). `tpatch status <slug>` surfaces this with a pointer to the shadow path and the resolution report.

### D9. No heuristic fallback

Phase 3.5 is explicitly provider-only. If the provider is unavailable or returns validation-failing output, the verdict is `blocked-requires-human` — it does NOT silently degrade to a heuristic resolver. The product value proposition is provider-assisted resolution; a heuristic that "does its best" would violate the v0.4.2 Truthful Errors theme that just shipped.

## Consequences

**Positive**
- Ships the core value promise of tpatch: "you don't worry about how to apply the changes."
- Atomic accept/reject makes the experience safe enough for CI/CD (`feat-ci-cd-integration`) and the bot (`feat-dependabot-bot`).
- Resolution report becomes a review artifact — reviewers grade the LLM merge just like any other PR.
- Per-file isolation caps blast radius and token cost.
- Shadow worktree leaves the user's real tree unchanged until they commit — identical safety guarantee to a `git merge --no-commit`.

**Negative / accepted trade-offs**
- Non-trivial implementation: shadow worktree lifecycle, atomic artifact refresh, validation per language.
- New transient state (`reconciling-shadow`) widens the state machine. Skills, status command, and cycle must all handle it.
- Token cost is real. The 10-conflict default cap may frustrate large-upstream-drift cases; mitigated by `--max-conflicts` and `feat-feature-decomposition`.
- Validation per language adds a maintenance surface (Go parser in-tree; others via a configurable command). Acceptable — if `syntax_check` is unset, parse-level validation is skipped and only the "no conflict markers + test_command" gate remains.
- Spec-drift-detection (D6) is deliberately deferred. Advertise the warning loudly so users know intent docs can become stale.

## Open questions (resolve in PRD-resolver)

1. Parallel per-file calls — do we fan out, and if so, with what concurrency limit per provider?
2. How do we attribute the resolver's work in `patches/NNN-reconcile.patch` — single combined patch vs. one-per-file?
3. Should `--resolve --apply` gate on a manual CI signal (e.g., "tests green on a prior commit") before auto-accepting? Or is the test_command gate enough?
4. Cross-feature interaction: if two features both conflict in the same reconcile run, can their shadows share a worktree or must each feature have its own? (Leaning one-per-feature for isolation, but increases disk usage.)
5. Resolution prompt — do we pass the full files or chunked by hunk? Large files blow context. Chunking needs careful overlap so the LLM sees enough context.
6. Roll-forward test suite — should we ship a golden-set of conflict scenarios (tests/reconcile/golden/) the resolver must pass before a release? (Leaning yes, Tranche B deliverable.)

These questions block the PRD, not the ADR. The architectural shape above is decided.
