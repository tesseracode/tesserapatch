# PRD — Agent-as-Provider Skills & `--manual` Phase Flag

**Status**: Draft (Tranche B, v0.4.3 skills + v0.5.0 CLI)
**Date**: 2026-04-20
**Owner**: tpatch core
**Target milestones**: B1 — ships ahead of the conflict resolver (B2, see ADR-010)
**Related**: `feat-skill-handoff-guidance`, `feat-phase-manual-flag`, `feat-init-skill-drift`, ADR-010

## 1. Problem

The v0.4.1 → v0.4.2 live stress test on a real fork surfaced that the biggest gap in tpatch is **not a CLI behaviour** — it is that the shipped skill files do not teach coding agents what to do when the LLM provider underperforms. Direct quote from the provider:

> "The biggest gap in the skills isn't a CLI behavior — it's that the skill doesn't teach agents what to do when the LLM provider fails or is not available."

Concrete failure mode observed repeatedly: `tpatch implement` produces 1-operation stubs / `ensure-directory` only recipes / truncated JSON. An agent reading the skill cold runs `implement` → `apply --mode execute` → failure → stuck with no next-step guidance. In practice every feature in the stress test required the agent to take over — but **the skill does not document that pattern at all**.

Today skills describe *how to invoke tpatch*. What they need to describe is *how to be a tpatch provider*.

A companion observation: when the agent *does* author a phase artifact by hand, there is no clean way to tell tpatch "I wrote `spec.md` / `exploration.md` / `apply-recipe.json` myself, please advance feature state without calling the provider." The agent has to either run the CLI phase and discard its output (wasting a token quota and overwriting the artifact) or manually edit `status.json` (fragile).

## 2. Goals

1. Every shipped skill format teaches a **symmetric Path A / Path B** for every phase (`analyze`, `define`, `explore`, `implement`, `reconcile`).
2. Agent-authored artifacts are treated as first-class — not an escape hatch. Path B is presented as **normal**, not exceptional.
3. The `apply-recipe.json` schema is inlined in every skill so an agent can author recipes without reading Go source.
4. The skills teach the `3WayConflicts` reconcile playbook that emerged during the stress test.
5. A CLI companion flag `--manual` exists on every phase command so agents can advance state without a provider round-trip.
6. All 6 skill formats stay in lockstep (parity guard enforces).

## 3. Non-goals

- Changing any existing phase algorithm. Agents get a documented Path B; the provider-driven Path A is unchanged.
- Implementing the conflict resolver. That is B2 (ADR-010). This PRD only teaches the *manual* 3WayConflicts playbook so agents have a path to unblock themselves *before* B2 lands.
- Implementing apt-style skill refresh (`feat-init-skill-drift`). Deferred to a sibling PRD. Users who want the new skills today use the documented manual-copy recipe.
- Rewriting SPEC.md. Schema-of-record stays in SPEC; skills reference SPEC for detail and inline the minimum needed to unblock the agent.

## 4. User stories

### 4.1 Agent hits a bad `implement` recipe
> *I ran `tpatch implement my-feature` and the recipe only has one `ensure-directory` op. The skill now tells me: "This is normal — use Path B. Run `tpatch apply --mode started`, hand-author the changes following `spec.md` and `exploration.md`, then `tpatch apply --mode done` and `tpatch record`. You are the provider."*

### 4.2 Agent with no provider configured
> *I am a coding agent running in a CI job. tpatch has no provider set. The skill tells me that Path B works without any provider at all — I can author `spec.md` / `exploration.md` / `apply-recipe.json` myself, run `tpatch <phase> --manual` to advance state, and ship the feature normally.*

### 4.3 Agent hits `3WayConflicts`
> *Reconcile returned `3WayConflicts`. The skill now gives me a playbook: use `git checkout stash@{0}^3 -- .tpatch/` to restore .tpatch without popping the stash, read `spec.md` + `post-apply.patch` + the new upstream file, and hand-resolve preserving both intents. I know not to pop the stash.*

### 4.4 Agent needs to author a recipe
> *I know the feature I want to implement but the provider failed. The skill inlines the recipe schema — I know the ops are `write-file {path, contents}`, `replace-in-file {path, search, replace, occurrences?}`, `ensure-directory {path}`, `delete-file {path}` — and that search is literal (not regex), defaults to 1 occurrence, and needs surrounding lines to disambiguate. I author the recipe directly and run `tpatch apply --mode execute`.*

## 5. Requirements

### 5.1 Skills — per-phase structure

Each of the 5 phases (`analyze`, `define`, `explore`, `implement`, `reconcile`) has a section in every skill that contains, in this order:

1. **Purpose** — one sentence.
2. **Artifact(s)** — file path under `.tpatch/features/<slug>/`.
3. **Path A — provider-driven** (fast, for simple features / CI with provider).
4. **Path B — agent-authored** (robust, for complex features / no provider / large-context agent).
5. **Quality checklist** — criteria that apply regardless of path.

Path B must be at least as prominent as Path A visually. No collapsed details, no "advanced users only" framing. Recommended order: present Path A first but immediately follow with "If the output is empty, stubby, or truncated, switch to Path B — that is normal, not an error."

### 5.2 Skills — inline apply-recipe.json schema

A dedicated section in every skill titled **"apply-recipe.json schema"** containing:

- JSON example with all four ops (`write-file`, `replace-in-file`, `ensure-directory`, `delete-file`).
- Per-op field list with types and defaults.
- Semantics callout: *"`replace-in-file.search` is a literal string match, not a regex. `occurrences` defaults to 1. Include surrounding lines in `search` to disambiguate when the same string appears multiple times."*
- Path safety callout: *"All `path` fields must be repo-relative. tpatch enforces this via `EnsureSafeRepoPath` — a path traversal attempt (`../`, absolute path, symlink target outside the repo) will abort `apply --mode execute`."*

### 5.3 Skills — 3WayConflicts playbook

A dedicated section titled **"If reconcile returns 3WayConflicts"** containing:

1. Don't pop the stash. The stash is preserving pre-reconcile state.
2. Restore `.tpatch/` only:
   ```
   git checkout stash@{0}^3 -- .tpatch/
   ```
3. Read `.tpatch/features/<slug>/spec.md` (intent) and `.tpatch/features/<slug>/artifacts/post-apply.patch` (current diff).
4. Read the new upstream version of each conflicted file.
5. Hand-author a resolution that preserves **both** intents: the feature's intent from spec.md AND the upstream change.
6. Forward-apply: `tpatch apply <slug> --mode execute` against the resolved tree.
7. Once clean: `tpatch record <slug>`.
8. Future: this playbook will be automated by the provider-assisted conflict resolver (see ADR-010 / B2). For now, you are the resolver.

### 5.4 Skills — patch-vs-recipe mental model

A paragraph in the "core concepts" section explaining:

> **The `post-apply.patch` captures intent. The `apply-recipe.json` targets a specific upstream snapshot.** When they disagree — e.g., the recipe's `replace-in-file` can't find its anchor because upstream edited the file — trust the patch. The patch is what you want applied; the recipe is one way to apply it. During reconcile, if the recipe fails but the patch applies, the patch wins.

### 5.5 Skills — "You are the provider" framing

A top-level section, one paragraph, near the start of each skill:

> You can act as a tpatch provider. Every CLI phase (`analyze`, `define`, `explore`, `implement`, `reconcile`) has two paths: (A) run the phase command, which calls the configured LLM provider; or (B) write the phase's output artifact yourself and run `tpatch <phase> --manual` to advance feature state. When the provider is unavailable, returns an empty response, or produces an obviously insufficient artifact, use Path B. Do not wait for a better recipe.

### 5.6 CLI — `--manual` / `--skip-llm` flag

Every phase command gains an identical flag:

```
tpatch analyze   <slug> --manual
tpatch define    <slug> --manual
tpatch explore   <slug> --manual
tpatch implement <slug> --manual
tpatch reconcile <slug> --manual     # reserved — refuses for now; see §7.2
```

Behaviour:

1. Validate that the expected output artifact exists at the expected path (e.g. `spec.md` for `analyze`, `apply-recipe.json` for `implement`). If missing → refuse with a clear error pointing at the exact path.
2. Do not call the provider.
3. Advance `status.json` state identically to the provider-driven path (`analyzed | defined | explored | implementing`).
4. Record the transition in `history.json` (or equivalent audit file) with `source: manual`.
5. Print a confirmation that names the artifact and the new state.
6. Exit 0.

Flag name: settled as `--manual`. `--skip-llm` is a documented alias. Harness JSON (`--format json`) includes a `source: "manual"` field so harnesses can distinguish.

### 5.7 CLI — parity guard

`assets/assets_test.go` gains anchors for the new canonical blocks so skills cannot silently drift:

- `provider-fallback/you-are-the-provider`
- `recipe-schema/ops-table`
- `recipe-schema/literal-search`
- `conflict-playbook/checkout-stash`
- `conflict-playbook/never-pop`
- `patch-vs-recipe/intent-vs-snapshot`

Each anchor is a short phrase that must appear in every shipped skill.

### 5.8 CLI — `--manual` tests

Cover in `internal/cli/phase2_test.go`:

- `--manual` with artifact present → advances state, no provider call.
- `--manual` with artifact missing → refuses with non-zero exit + path in error.
- `--manual` with corrupt artifact (e.g. invalid JSON recipe) → refuses (schema validation, not provider call).
- `--manual` + `--format json` → structured output with `source: "manual"`.
- `--skip-llm` alias works identically.

## 6. Deliverables

1. **Skills (6 formats)** rewritten against §5.1–5.5:
   - `assets/skills/claude/tessera-patch/SKILL.md`
   - `assets/skills/copilot/tessera-patch/SKILL.md`
   - `assets/prompts/copilot/tessera-patch-apply.prompt.md`
   - `assets/skills/cursor/tessera-patch.mdc`
   - `assets/skills/windsurf/windsurfrules`
   - `assets/workflows/tessera-patch-generic.md`
2. **Parity guard** — `assets/assets_test.go` with the six new anchors.
3. **CLI** — `--manual` / `--skip-llm` flag on all five phase commands in `internal/cli/cobra.go`. New helper in `internal/store/` for the artifact-exists validation.
4. **Tests** — `internal/cli/phase2_test.go` additions per §5.8.
5. **Docs** — `docs/agent-as-provider.md` (new) — longer-form companion to the skills; skills link to it. Includes the recipe schema with exhaustive examples and the 3WayConflicts playbook with screen-grabs of actual conflict output.
6. **CHANGELOG** entry under v0.4.3 (skills + flag) or v0.5.0 (bundled with B2). Working assumption: ship skills+flag as v0.4.3 so users can adopt without waiting for B2.
7. **Handoff** — `docs/handoff/CURRENT.md` updated when each deliverable lands.

## 7. Out of scope

### 7.1 Conflict resolver implementation
Covered by ADR-010 / B2. The skill documents the *manual* playbook; automated resolution is a separate PRD.

### 7.2 `reconcile --manual`
Reserved but disabled. Reconcile advances through verdicts, not states — "manual reconcile" implies the agent has applied a resolution by hand, and the user surface for that is `tpatch apply <slug>` + `tpatch record <slug>` against the new upstream, not a reconcile invocation. We ship `--manual` accepted by the flag parser but refused with a message pointing at the B2 docs.

### 7.3 Skill refresh UX
`feat-init-skill-drift` (apt-style overwrite / merge / backup) is the right fix and is already scoped. Ships separately. Until then, `docs/agent-as-provider.md` contains the manual copy recipe for existing v0.4.2 users.

### 7.4 Spec-drift detection
When a manual reconcile resolution rewrites files referenced by `spec.md`, the intent document can become stale. Explicitly not fixed here — `feat-spec-drift-detection` tracks it.

## 8. Success metrics

- **Parity**: `go test ./assets/...` green after all 6 skills edited. All 6 new anchors present in all 6 skills.
- **CLI**: `go test ./internal/cli/...` green. `tpatch analyze <slug> --manual` on a fixture repo with hand-authored `spec.md` advances state to `analyzed` and writes a `history.json` entry with `source: "manual"`.
- **Reproduction of the v0.4.2 failure mode**: give a fresh agent the new skills and a feature whose `implement` produces a 1-op stub. The agent identifies Path B as the correct next step without further human prompting, authors the recipe, and lands the feature. (Manual smoke test; not automated.)
- **No regressions**: all v0.4.2 tests still pass.

## 9. Open questions

1. Ship as v0.4.3 (docs + flag, no new core behaviour) or bundle with v0.5.0?
   - Leaning **v0.4.3**. Users are hitting the skill gap *today*. No reason to hold the fix behind B2.
2. Does `--manual` need a companion `--manual-check` that validates the artifact without advancing state? (Useful for pre-commit hooks.)
   - Probably yes; cheap to add alongside. Decide during implementation.
3. Should the skills reference ADR-010 / conflict resolver by name, or just hint at "future automation"?
   - Reference ADR-010 by name in the Generic and Claude skills (agents that will read docs), omit from the shorter harness-targeted formats (Cursor/Windsurf) to keep them tight.
4. `history.json` — does it exist today? If not, creating it is in scope here (cheap, self-contained) or we deferred to a follow-up.
   - Verify in implementation; if missing, add a minimal append-only log file. Either way, `source: manual` is the contract.
5. Recipe schema in skills — inline as JSON example + field list, or inline as a tables? Tables are denser but render worse in `.windsurfrules`/`.cursor/rules/*.mdc`. Leaning JSON + bullet list.
6. Should Path B mention that the agent can use `tpatch cycle --interactive` (stops between phases) as a middle ground? Probably yes — one-line hint.

## 10. Implementation order

1. CLI `--manual` flag (smallest surface, unblocks skill authoring):
   - Flag wiring in `initCmd` for each phase command.
   - Helper `store.AdvanceStateManually(slug, phase)` — validates artifact, appends to history, updates status.
   - Tests.
2. Draft skills — start with the Claude SKILL.md (richest format), use it as the canonical source, propagate the content to the other 5 with format adaptations.
3. Parity guard update.
4. `docs/agent-as-provider.md`.
5. Handoff + CHANGELOG.

Single branch. Single v0.4.3 commit + tag at the end.
