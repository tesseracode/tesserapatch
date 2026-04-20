# Changelog

All notable changes to tpatch are recorded here.

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
