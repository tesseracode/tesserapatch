# PRD - Skill Documentation Strategy - `feat-skill-doc-references-user-visible`

**Status**: Approved (paper design; review approved 2026-05-11)
**Date**: 2026-05-11
**Owner / byline**: P55
**Milestone**: TBD (post-v0.7 cluster)

## Related

- [WP-001 Feature Slice Gap](../whitepapers/WP-001-feature-slice-gap.md) - claims-audit-table convention (§3.5) and graduation conventions (§9).
- [PRD-tpatch-land](./PRD-tpatch-land.md) - v0.7 cluster reference shape and the surfacing context for `docs/land.md` skill references.
- [Personas](../market-research/personas.md) - especially Maintainer Mira's reasoning-preservation and offline-ergonomics constraints.
- [Competitive Landscape](../market-research/competitive-landscape.md) - §6 SMART, §9 prior art, §10 anti-precedents, and §11 skill-emission positioning.
- [Supervisor LOG](../supervisor/LOG.md) - v0.7 cluster cross-review protocol and paper-design acceptance context.
- [SPEC](../../SPEC.md) - §8 skill-system contract.

## 0. Meta

### 0.1 Problem framing

The shipped skill surfaces are the agent-facing UX for end users, but they currently contain repo-relative references to tesserapatch development-repo docs. The live asset grep shows `docs/land.md` and `docs/reconcile.md` references in every one of the six shipped surfaces; those files are not installed into end-user repositories by `tpatch init`. The bug matters now because `PRD-tpatch-land` requires skill files to reference `tpatch land` as the recommended commit path, and the current Wave C-era assets already inherited the same broken-reference pattern for `docs/land.md`.

This is a post-cluster strategy PRD. It does not re-open the v0.7 / M17 paper cluster, and it must not block Wave C closure. The outcome should make future shipped skills self-contained enough that an end user's agent can follow the guidance without access to this development repo.

### 0.2 Claims-audit table

Every current-behavior claim below was re-checked against the live tree on 2026-05-11.

| Claim | Evidence |
|---|---|
| The v0.7 cluster review used claims-audit tables, peer cross-review, and accepted the land / auto-base / collision / lock-guard PRDs as paper designs. | `docs/supervisor/LOG.md:392-416` |
| The same review notes that WP-001's claims-audit-table convention reduced cite drift across the cluster. | `docs/supervisor/LOG.md:470-473` |
| WP-001 §3.5 recommends one-page claims audits with file:line cites for load-bearing current-behavior claims. | `docs/whitepapers/WP-001-feature-slice-gap.md:177-195` |
| WP-001 §9 graduated `PRD-tpatch-land.md` and required `land` implementation to wait for guardrails; it also kept the claims-audit convention as an expectation. | `docs/whitepapers/WP-001-feature-slice-gap.md:654-666`, `docs/whitepapers/WP-001-feature-slice-gap.md:702-718` |
| `PRD-tpatch-land` v2.1 is owned by CO47 and explicitly references the v0.7 cluster and skill-file `tpatch land` updates. | `docs/prds/PRD-tpatch-land.md:1-18`, `docs/prds/PRD-tpatch-land.md:645-651` |
| SPEC defines tpatch as a single binary with embedded assets and six agent integration formats. | `SPEC.md:13-17` |
| SPEC says `tpatch init` creates the `.tpatch/` workspace and installs all skill formats. | `SPEC.md:53-66` |
| SPEC §8 names the six harness formats and their installed locations, and says the parity guard ensures all formats mention current CLI commands. | `SPEC.md:155-168` |
| `assets/embed.go` embeds `prompts`, `skills`, `templates`, and `workflows` into `assets.Skills`; it does not embed the top-level `docs/` tree. | `assets/embed.go:6-9` |
| `tpatch init` calls `installSkills` after `store.Init`. | `internal/cli/cobra.go:87-120` |
| `installSkills` writes exactly six surfaces from embedded assets into user-repo / harness locations: Claude, Copilot, Copilot Prompt, Cursor, Windsurf, and Generic workflow. | `internal/cli/cobra.go:2055-2077` |
| The parity guard's required command list includes `tpatch land` and `tpatch reconcile`. | `assets/assets_test.go:12-30` |
| The parity guard checks required command and anchor strings across the six `skillFiles`. | `assets/assets_test.go:114-155` |
| The recipe-schema guard parses JSON recipe examples across the same six surfaces. | `assets/assets_test.go:168-227` |
| The Claude skill surface currently references both `docs/land.md` and `docs/reconcile.md`. | `assets/skills/claude/tessera-patch/SKILL.md:68-69` |
| The Copilot skill surface currently references both `docs/land.md` and `docs/reconcile.md`. | `assets/skills/copilot/tessera-patch/SKILL.md:43-44` |
| The Cursor surface currently references both `docs/land.md` and `docs/reconcile.md`. | `assets/skills/cursor/tessera-patch.mdc:40-41` |
| The Windsurf surface currently references both `docs/land.md` and `docs/reconcile.md`. | `assets/skills/windsurf/windsurfrules:34-35` |
| The Copilot Prompt surface currently references both `docs/land.md` and `docs/reconcile.md`. | `assets/prompts/copilot/tessera-patch-apply.prompt.md:50-51` |
| The Generic workflow surface currently references both `docs/land.md` and `docs/reconcile.md`. | `assets/workflows/tessera-patch-generic.md:38-39` |
| The live grep found no `docs/record.md` references in `assets/`. | `rg "docs/record\\.md" assets` -> no matches (2026-05-11) |
| Persona Mira values skill emission because the agent file acts as the handoff she does not have; her setup overhead and offline/token-cost sensitivity are explicit counter-evidence. | `docs/market-research/personas.md:165-221` |
| The competitive landscape treats multi-format skill / agent emission as a tpatch edge that nobody else has. | `docs/market-research/competitive-landscape.md:820-825` |
| The same doc says coding-agent integration is a passive distribution channel and that skill files are the UX surface for many users. | `docs/market-research/competitive-landscape.md:647-665` |

No implementation has been changed by this PRD.

### 0.3 Non-goals

- **Not a re-architecture of the skill system.** The six-surface model remains the SPEC §8 contract.
- **Not a change to which CLI commands are mentioned.** The PRD does not remove `record`, `land`, `reconcile`, or any other required command from shipped skills.
- **Not changes to `docs/record.md`, `docs/reconcile.md`, or `docs/land.md` themselves.** The strategy changes references from shipped skill surfaces; it does not rewrite the development-repo docs.
- **Not a Wave C rework request.** `docs/land.md` still ships per `PRD-tpatch-land` §6 ac.16; this PRD handles the broader user-visible reference strategy in a separate wave.
- **Not a generic documentation-site plan.** Public docs may exist later, but this PRD decides what shipped skills can rely on.
- **Not a backlog entry.** The existence of this PRD is sufficient tracking for `feat-skill-doc-references-user-visible`.

## 1. Summary

Shipped tpatch skill surfaces currently point end users at development-repo docs (`docs/land.md`, `docs/reconcile.md`) that are not installed with the six harness files, so agents following those instructions hit broken local paths. Choose **inline-minimal**: remove repo-relative `docs/*.md` references from shipped skill surfaces, replace each with the action-relevant rule-of-thumb already needed at the point of use, and add a parity-guard check that fails future repo-relative docs references. This keeps skills self-contained and offline-friendly for Maintainer Mira without adding `.tpatch/docs/` versioning or public-URL dependencies.

## 2. Motivation

### 2.1 Persona grounding

**Platform Pat** needs audit-friendly guidance, but Pat's strongest concern is that tpatch outputs are trustworthy and repeatable; Pat is not the persona most harmed by a broken skill doc link. Still, a missing `docs/reconcile.md` path weakens the agent-mediated onboarding surface Pat's team relies on (`personas.md:47-88`).

**Security Sam** needs forwardable, deterministic hotfix guidance. A broken local path in the skill does not directly change patch bytes, but it does make a time-sensitive CVE workflow depend on a missing reference at the moment Sam wants the agent to act quickly (`personas.md:103-149`).

**Maintainer Mira** is the primary affected persona. Her value proposition depends on lifecycle artifacts and skill emission: "the agent file is the team handoff Mira doesn't have" (`personas.md:198-203`). She also has low appetite for setup overhead and may prefer offline / heuristic fallback over provider-heavy workflows (`personas.md:206-221`). A skill that says "see docs/reconcile.md" but does not ship that file violates the exact offline ergonomics that make tpatch credible for her.

### 2.2 Evidence base from shipped surfaces

The live state is narrower than the original bug report: `docs/record.md` no longer appears in shipped assets, and no `docs/adrs/ADR-010...` reference appears in the live grep. The current user-visible issue is two doc paths repeated across all six shipped surfaces:

| Referenced path | Shipped surfaces affected | Count across all six surfaces | Count strictly under `assets/skills/**` |
|---|---|---:|---:|
| `docs/land.md` | Claude, Copilot, Copilot Prompt, Cursor, Windsurf, Generic | 6 | 4 |
| `docs/reconcile.md` | Claude, Copilot, Copilot Prompt, Cursor, Windsurf, Generic | 6 | 4 |
| `docs/record.md` | None in live `assets/` grep | 0 | 0 |

This is widespread enough to need a strategy, not a one-off copy edit. It also crosses the exact six-surface set that `assets/assets_test.go` treats as a parity unit (`assets/assets_test.go:114-155`).

### 2.3 Prior-art check

The competitive landscape does not show another tool solving this exact problem because tpatch's six-format skill emission is itself unusual: the doc calls it an edge "nobody else even has one" (`competitive-landscape.md:820-825`). Lane A prior art such as Quilt, StGit, jj, and gbp-pq offers useful patch-management patterns, but not embedded multi-agent skill surfaces (`competitive-landscape.md:779-793`).

The anti-precedents are still instructive. We should not copy tool-specific packaging or distribution coupling from gbp-pq (`competitive-landscape.md:803-806`), and we should not move the product toward a hosted dependency just to make skill links resolve. tpatch's own strategy document says skill files are the user's agent-mediated interface (`competitive-landscape.md:647-665`); therefore the fix belongs in shipped skill content and its parity guard, not in a separate documentation channel that may or may not be present.

## 3. Solution candidates

### (a) Embed-and-unpack docs into `.tpatch/docs/` at `tpatch init` time

Bundle selected docs (`record`, `reconcile`, `land`, and possibly ADR excerpts) with the binary and unpack them into `.tpatch/docs/`. Update shipped skill references to point at `.tpatch/docs/<name>.md`.

| Criterion | Analysis |
|---|---|
| Pros | Local paths resolve; full docs are available offline; Mira gets deep context without the development repo. |
| Cons | Adds init complexity, doc-selection policy, and upgrade sync questions; `.tpatch/docs/` becomes another generated artifact surface. |
| Cost | Medium. Requires embedding docs, writing unpack/update logic, and testing version / overwrite behavior. |
| Persona impact | Pat: neutral-positive for onboarding. Sam: positive for complete local docs. Mira: positive for offline deep reading, but negative if refresh mechanics add setup friction. |
| v0.6 backcompat | Existing repos would need `tpatch init` rerun or a new refresh command; no automatic help for already-unpacked skills unless migration is introduced. |
| Parity-guard implications | Add a resolver check: every repo-relative doc reference in a shipped skill must point at an embedded and installed doc. |

### (b) Inline full content in skill files

Copy the relevant docs wholesale into each skill surface and remove all cross-references.

| Criterion | Analysis |
|---|---|
| Pros | No broken paths; fully offline; no init or upgrade mechanics. |
| Cons | Duplicates long-form content across six surfaces; bloats skill files; makes docs and skills drift unless every content edit fans out. |
| Cost | Low initial edit, high ongoing maintenance. |
| Persona impact | Pat: neutral, maybe noisy. Sam: positive for completeness but slower agent context. Mira: offline-friendly, but larger skill files may bury the rule she needs. |
| v0.6 backcompat | Existing repos need skill reinstall to receive the new text; no data migration. |
| Parity-guard implications | Existing anchors would not prove full-doc parity. A brittle doc-vs-skill diff guard would be tempting and likely overfit. |

### (c) Public URL references

Replace local `docs/*.md` references with public docs URLs or GitHub blob/raw URLs.

| Criterion | Analysis |
|---|---|
| Pros | Cheapest implementation; no binary-size or init impact; content can stay fresh. |
| Cons | Adds hosting / GitHub reachability dependency; breaks offline ergonomics; links can rot; agents may be blocked from network fetches. |
| Cost | Very low. |
| Persona impact | Pat: acceptable in connected enterprise repos. Sam: acceptable for forwardable references, risky during incidents. Mira: poor; this is the least aligned option for the primary affected persona. |
| v0.6 backcompat | Existing repos need skill reinstall; old broken refs remain until then. |
| Parity-guard implications | Could check URL syntax only, not availability or content compatibility. |

### (d) Inline-minimal snippets

Remove `see docs/X.md` references and inline only the action-relevant rule at the point where the agent needs it. The skill remains the end-user contract; development-repo docs remain contributor / maintainer references.

| Criterion | Analysis |
|---|---|
| Pros | Self-contained and offline-friendly; small skill delta; no init-doc versioning; keeps command guidance close to the step where the agent acts. |
| Cons | Still duplicates concise snippets across six surfaces; authors must decide what is essential; full long-form docs are not available to end users unless they have the development repo. |
| Cost | Low. One coordinated skill edit plus guard test. |
| Persona impact | Pat: positive for agent onboarding. Sam: positive for incident-time command clarity. Mira: strongest fit because the shipped agent handoff works offline without extra setup. |
| v0.6 backcompat | Existing repos keep old skills until reinstall. No `.tpatch/` schema or migration required. |
| Parity-guard implications | Add a simple negative guard: shipped skill surfaces must not contain repo-relative `docs/*.md` references. Existing command/anchor/schema guards remain. |

### (e) Hybrid: inline-minimal plus optional bundled docs

Inline action snippets now, and optionally bundle selected long-form docs later under `.tpatch/docs/` for deep dives.

| Criterion | Analysis |
|---|---|
| Pros | Fixes broken refs immediately while preserving a path to offline deep docs. |
| Cons | Two strategies to teach; still needs all `.tpatch/docs/` versioning decisions if enabled. |
| Cost | Low now if the bundled-docs part is deferred; medium if shipped together. |
| Persona impact | Pat: neutral-positive. Sam: positive. Mira: best eventual experience, but only if refresh behavior is simple. |
| v0.6 backcompat | Same as inline-minimal for v1; bundled docs later would need explicit refresh semantics. |
| Parity-guard implications | Start with the no-repo-relative-doc-refs guard; add doc-resolver checks only if bundled docs ship. |

## 4. Recommended choice

Choose **(d) inline-minimal snippets** for the first implementation slice.

This is the smallest solution that fully addresses the user-visible bug: the shipped skill no longer points at files that do not travel with it, and the agent receives the rule it needs without network access. It also matches the product's unusual strength: skill files are the user-facing agent contract, not a thin pointer to contributor docs.

The runner-up is **(a) embed-and-unpack docs**. It is stronger for offline deep reading, but it turns a broken-link bug into an asset-versioning problem: which docs are bundled, when they update, whether user-edited `.tpatch/docs/` files are overwritten, and how old repos discover the refresh. Those questions are worth revisiting only if inline-minimal snippets prove insufficient.

## 5. Affected surfaces

### 5.1 All six shipped surfaces

| Path | Format | Line(s) | Reference(s) |
|---|---|---:|---|
| `assets/skills/claude/tessera-patch/SKILL.md` | Claude | 68 | `docs/land.md` |
| `assets/skills/claude/tessera-patch/SKILL.md` | Claude | 69 | `docs/reconcile.md` |
| `assets/skills/copilot/tessera-patch/SKILL.md` | Copilot | 43 | `docs/land.md` |
| `assets/skills/copilot/tessera-patch/SKILL.md` | Copilot | 44 | `docs/reconcile.md` |
| `assets/prompts/copilot/tessera-patch-apply.prompt.md` | Copilot Prompt | 50 | `docs/land.md` |
| `assets/prompts/copilot/tessera-patch-apply.prompt.md` | Copilot Prompt | 51 | `docs/reconcile.md` |
| `assets/skills/cursor/tessera-patch.mdc` | Cursor | 40 | `docs/land.md` |
| `assets/skills/cursor/tessera-patch.mdc` | Cursor | 41 | `docs/reconcile.md` |
| `assets/skills/windsurf/windsurfrules` | Windsurf | 34 | `docs/land.md` |
| `assets/skills/windsurf/windsurfrules` | Windsurf | 35 | `docs/reconcile.md` |
| `assets/workflows/tessera-patch-generic.md` | Generic | 38 | `docs/land.md` |
| `assets/workflows/tessera-patch-generic.md` | Generic | 39 | `docs/reconcile.md` |

### 5.2 Counts by referenced doc

| Referenced doc | Formats affected | Count |
|---|---|---:|
| `docs/land.md` | Claude, Copilot, Copilot Prompt, Cursor, Windsurf, Generic | 6 |
| `docs/reconcile.md` | Claude, Copilot, Copilot Prompt, Cursor, Windsurf, Generic | 6 |
| `docs/record.md` | None in live `assets/` grep | 0 |

Strictly under `assets/skills/**`, the affected surfaces are Claude, Copilot, Cursor, and Windsurf: 4 references to `docs/land.md` and 4 references to `docs/reconcile.md`.

## 6. Parity-guard interaction

The current parity guard enforces three relevant classes of skill quality:

1. required CLI commands across all six surfaces (`assets/assets_test.go:12-30`, `assets/assets_test.go:127-153`);
2. required anchor strings for invocation, phase ordering, preflight, fallback, conflict, and freshness guidance (`assets/assets_test.go:32-74`);
3. recipe JSON examples that match `workflow.RecipeOperation` (`assets/assets_test.go:168-227`).

Inline-minimal does **not** remove or weaken any of those checks. It needs one new negative check:

```text
TestSkillDocReferencesAreSelfContained:
  for each skillFiles entry:
    fail if content matches repo-relative docs path: `\bdocs/[A-Za-z0-9_./-]+\.md\b`
```

The check should intentionally scan the same `skillFiles` table as `TestSkillParityGuard`, not just `assets/skills/**`, because Copilot Prompt and Generic workflow are shipped surfaces too. If a future design intentionally bundles docs under `.tpatch/docs/`, this guard can be replaced by a resolver guard that verifies every reference points at an installed embedded file. For the recommended v1, no repo-relative docs references should remain.

## 7. Implementation gating

This PRD does not require Wave C to be reworked and does not block Wave C closure. `docs/land.md` is created per `PRD-tpatch-land` §6 ac.16, and the current skill update that mentions `tpatch land` is correct in scope even though it exposed the broader doc-reference bug (`docs/prds/PRD-tpatch-land.md:645-651`).

The implementation slice for this PRD should land after supervisor sign-off and after any active Wave C revision settles, because it touches the same skill surfaces but not the `land` runtime implementation.

## 8. Backwards compatibility

Existing v0.6 / v0.7 repos with already-installed skills are not migrated automatically by this PRD. They will keep the old broken references until a user reinstalls / refreshes skill assets. That is acceptable for v1 because the bug is in generated guidance, not `.tpatch/` state or feature artifacts.

Recommended policy:

1. **No automatic overwrite of user skill files** in this PRD. Users may have edited harness files.
2. **Re-running `tpatch init` refreshes generated skill files** according to current behavior (`internal/cli/cobra.go:2055-2077`), but the implementation should document that this may overwrite local edits.
3. **Follow-up question**: add an explicit `tpatch init --refresh-skills` or `tpatch skills refresh` command if users need a safer migration path.

No `.tpatch/docs/` directory, schema migration, or auto-migration is required for inline-minimal.

## 9. Acceptance criteria

1. All six shipped surfaces remove repo-relative `docs/land.md` and `docs/reconcile.md` references.
2. No shipped surface introduces `docs/record.md` or any other repo-relative `docs/*.md` reference.
3. Each removed reference is replaced by concise action guidance sufficient for an agent to proceed offline:
   - `land`: the command composes record + safe-stage + one Git commit with the `Tpatch-Feature` trailer block.
   - `reconcile`: the command requires a clean working tree at target upstream state and refuses dirty trees / conflict leftovers.
4. `assets/assets_test.go` adds a guard that fails on repo-relative `docs/*.md` references across the same six `skillFiles`.
5. Existing command, anchor, and recipe-schema parity tests continue to pass.
6. Offline use of the installed skill surfaces produces no broken local documentation references for the land / reconcile preflight guidance.
7. Documentation update discipline is explicit in the implementation handoff or contributor docs: when long-form docs change command-critical guidance, the corresponding skill snippets must be reviewed in the same change.
8. Existing repos require no `.tpatch/` migration; users can opt into refreshed skill files by reinstalling / refreshing generated harness assets.
9. Wave C closure remains unblocked; this implementation is tracked as a separate post-cluster slice.

## 10. Open questions

1. Should the project add a first-class `tpatch skills refresh` / `tpatch init --refresh-skills` command so existing repos can update generated skill assets without re-running all init behavior?
2. Should `.tpatch/docs/` be versioned alongside tpatch upgrades if future long-form offline docs become necessary, or should shipped skills stay snippet-only permanently?
3. Does this interact with a future skill-discoverability feature (for example, a command that lists installed harness surfaces and their versions)?
4. Should the no-`docs/*.md` parity guard allow fully qualified public URLs, or should shipped skills be offline-only by default?
5. How should reviewers decide whether a long-form doc change is "command-critical" enough to require a skill snippet update?
6. Should generated skill files carry an embedded tpatch version or content hash so users can see whether their installed skill copy is stale?

## 11. Sources

| Topic | Source |
|---|---|
| v0.7 cluster review protocol, accepted PRDs, and claims-audit convention | `docs/supervisor/LOG.md:392-416`, `docs/supervisor/LOG.md:470-473` |
| WP-001 claims-audit convention and graduation sequencing | `docs/whitepapers/WP-001-feature-slice-gap.md:177-195`, `docs/whitepapers/WP-001-feature-slice-gap.md:654-718` |
| Reference PRD shape and Wave C documentation / skill acceptance criteria | `docs/prds/PRD-tpatch-land.md:1-18`, `docs/prds/PRD-tpatch-land.md:645-651` |
| Skill system, embedded assets, install locations | `SPEC.md:13-17`, `SPEC.md:53-66`, `SPEC.md:155-168`, `assets/embed.go:6-9`, `internal/cli/cobra.go:87-120`, `internal/cli/cobra.go:2055-2077` |
| Parity-guard behavior | `assets/assets_test.go:12-30`, `assets/assets_test.go:32-74`, `assets/assets_test.go:114-155`, `assets/assets_test.go:168-227` |
| Live affected skill references | `assets/skills/claude/tessera-patch/SKILL.md:68-69`, `assets/skills/copilot/tessera-patch/SKILL.md:43-44`, `assets/prompts/copilot/tessera-patch-apply.prompt.md:50-51`, `assets/skills/cursor/tessera-patch.mdc:40-41`, `assets/skills/windsurf/windsurfrules:34-35`, `assets/workflows/tessera-patch-generic.md:38-39` |
| Live absence of `docs/record.md` references | `rg "docs/record\\.md" assets` -> no matches (2026-05-11) |
| Personas | `docs/market-research/personas.md:47-88`, `docs/market-research/personas.md:103-149`, `docs/market-research/personas.md:165-221` |
| Competitive positioning and prior art | `docs/market-research/competitive-landscape.md:647-665`, `docs/market-research/competitive-landscape.md:696-714`, `docs/market-research/competitive-landscape.md:779-807`, `docs/market-research/competitive-landscape.md:820-825` |
