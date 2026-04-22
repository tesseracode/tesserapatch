# Changelog

All notable changes to tpatch are recorded here.

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
