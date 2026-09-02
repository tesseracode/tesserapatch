# PRD - Recipe Generation Authority

**Status**: Accepted rev-7
**Date**: 2026-09-01
**Owner**: Core
**Issue**: [GH #15](https://github.com/tesseracode/tesserapatch/issues/15)
**Architecture**:
[ADR-036 - Recipe Generation and Coverage Authority](../adrs/ADR-036-recipe-coverage-authority.md)
rev-7 — **normative where the two documents overlap**
**Depends on**:
[ADR-010](../adrs/ADR-010-provider-conflict-resolver.md),
[ADR-024](../adrs/ADR-024-patch-generation-manifest-boundary.md),
[ADR-025](../adrs/ADR-025-reconcile-evidence-and-revision-schema.md),
[ADR-028](../adrs/ADR-028-supersession-edge-model.md),
[ADR-029](../adrs/ADR-029-write-file-recipe-safety.md), and
[PRD-write-file-recipe-safety](./PRD-write-file-recipe-safety.md)
**Blocks**: GH #13 safe phase-2 operation replay
**Target release**: v0.17.0, separate from GH #13's v0.18.0

## 0. Revision history

| Rev | Date | Change |
|---|---|---|
| rev-0 | 2026-09-01 | Initial proposal. Reviewed NEEDS REVISION. |
| rev-1 | 2026-09-01 | ADR-029 D7 supersession severity preserved and mirrored (§6.7); convergent provenance rerun including the no-op path (§6.6); executable, non-reclassifiable-operation and parent-created cohorts marked incomplete (§6.3); the strict coverage schema now lives only in ADR-036 D3 and is reproduced here byte-identically (§6.4); ADR-029 D3 amended for empty-preimage exact postimage only (§6.7); `created_by` parent-postimage reuse dropped (§6.8); `--unstaged` reclassified as commit-kind HEAD (§6.2); verify check `recipe_generation_coverage` with pinned severity and exits (§6.12); doctor `D10` read-only (§6.13); already-present result accounting (§6.11); GH #15 anchor deferral recorded as a planning-acceptance amendment (§6.9); the two fail-soft `FilesInPatch` callers migrated in S1 (§6.1, §8); matrix rebuilt to 114 rows (I 37, C 28, G 21, U 26, S 2). Reviewed NEEDS REVISION. |
| rev-2 | 2026-09-01 | Complete five-producer inventory P1-P5 with one shared coverage publication API and three permitted terminal states (§6.15, ADR-036 D15); record-generated origin proved by exact derived-recipe byte equality instead of any trust-by-label marker, with file-set equality retired, near-match refusal and crash recovery pinned (§6.16, ADR-036 D16); missing coverage made uniformly warning-class with the `missing-produced`/`missing-legacy` split retired and a deletion test added (§6.12, §7); `recipe-stale.json` presence made warning-class and consumer-ineligible with a pinned precedence ladder and a pre-v0.17.0 legacy fixture (§6.12); the parser rule rescoped to one authoritative grammar plus registered adapters and registered non-authoritative scanners over the complete derived PI-1..PI-11 inventory (§6.1, §8); `effects[].kind` replaced by orthogonal `change_kind`/`content_kind`/`object_kind` axes and `reason_code` by sorted `reason_codes[]` (§6.4); coverage gains `producer` and `capture.mode` gains `reconcile` (§6.4, §6.2); new reason codes `producer-patch-rewrite` and `recipe-not-regenerated` (§6.4, §7); matrix rebuilt to 156 rows (I 46, C 37, G 37, U 34, S 2). Reviewed NEEDS REVISION. |
| rev-3 | 2026-09-01 | Producer registry expanded from five to **seven** — `implement` (P6) and `artifact-edit` (P7) added, `cycle` restated as a composite whose implement step is P6, `apply --mode done` scoped to runs that actually write the patch, and direct filesystem edits named as ungovernable external tamper detected at read time (§6.15, ADR-036 D15); required `patch_present` with `canonical-patch-missing` / `canonical-patch-empty`, so an absent or empty canonical patch is explicitly incomplete rather than vacuously complete or silently absent (§6.4, §6.3); exact feature-versus-effect allocation for `reasons[]` and `reason_codes[]`, plus an iff completeness predicate no contradictory record can decode (§6.4, §6.5); `producer-patch-rewrite` redefined conditionally and always paired with `recipe-not-regenerated` (§6.4, §6.15); the undefined normalized hunk digest replaced by an exactly specified required `patch_fragment_sha256`, modes and object kind read from the immutable observation, and `content_kind` extended to `text`/`binary`/`none` (§6.4); `apply --mode execute` given a named `recipe-generation-incomplete` refusal instead of a silent fallback or the misleading legacy no-recipe error (§6.3, §6.11, ADR-036 D17); PI-7 corrected — the unapply splitter already decodes quoting and deliberately returns both sides, so the migration target is a new `PathsAffectedByPatchStrict` all-paths API, fail-closed at all three call sites (§6.1); existing `FilesInPatchStrict` b-side callers registered as PI-12 with an unchanged contract (§6.1); PI-10's illegitimate recipe-operation-count use at `cobra.go:1908` removed (§6.1); surface code `recipe-coverage-stale-marker` mapped explicitly to schema reason `recipe-stale-marker-present` (§6.12, §7); immutable capture generalized to every producer (§6.2); regeneration remediation printed only after a dry derivation proves it can succeed (§6.11, §6.13); matrix rebuilt to 261 rows (I 69, C 56, G 60, U 74, S 2). |
| rev-4 | 2026-09-01 | Governed producer events split into **three** categories — successful bound write, successful manual checkpoint, and an explicitly contracted re-observation checkpoint — so P2's non-empty same-patch `classification.Append == false` branch (`internal/cli/feature_patch.go:104-112`) becomes a category-(c) event that publishes coverage before returning while keeping its shipped `no patch byte change; refresh\|fixup skipped` meaning and writing nothing else, and P2's empty-capture early return is explicitly not a checkpoint (§6.15); §6.11's execute-mode refusal rewritten to be reachable and state-aware — D17 fires only when `reapplying` is false, so the canonical-patch reapply instruction and the false "the operator chooses it" claim are deleted, `applied` features are told no reapply is needed, other states get reviewed external `git apply --check` / `git apply` commands, `implement --manual`'s transition to `implementing` is disclosed, `feature unapply` is never recommended unproven, coverage-binding refusal precedes the withheld-recipe branch, and exit `2` is justified as pre-mutation executable-plan validation (§6.11, ADR-036 D17); new record-level reasons `canonical-patch-unparseable` and `recipe-undecodable`, with required `recipe_decodable` and `recipe_sha256` always binding the exact raw recipe bytes whenever the file is present (§6.3, §6.4); required per-effect `preimage_observed` / `postimage_observed`, new effect-local `postimage-unavailable`, and both flags folded into `effect_sha256`, so an unobserved side is no longer encoded as a proven absence (§6.4); an exact reason-code → `disposition` mapping with `mismatch` → `ambiguous` → `unsupported` precedence (§6.4); one canonical ten-predicate completeness list owned by ADR-036 D3 and reproduced byte-identically in §6.5 under a parity guard (§6.5); the impossible surface/schema bijection replaced by a one-way partial mapping plus disjointness (§6.12, §7); the raw seven-writer threshold replaced by a registry count of seven plus an AST-derived site-to-producer mapping over the eight direct bound `WriteArtifact` sites (§6.15); P7 retriggered on the resolved artifact path with feature-root decoy precedence pinned (§6.15); P6 source truth corrected — `internal/workflow/implement.go:194` is the unmarshal-failure raw-response arm and `:209` the valid-JSON reserialized arm, both P6 events, the failure arm owing bound incomplete coverage for the undecodable bytes it just wrote (§2.7, §6.15); `openInEditor` given an error-returning contract with P7 publishing before propagating an editor failure (§6.2, §6.15); `tpatch land`'s embedded record named as P1 orchestration (§2.7); `patch_fragment_sha256` boundaries pinned to strict-grammar line-start `diff --git` records (§6.4); `CapturePatchScoped` cited at `internal/cli/feature_patch.go:88` (§6.2); matrix rebuilt to 311 rows (I 75, C 67, G 72, U 95, S 2). Reviewed NEEDS REVISION. |
| rev-5 | 2026-09-01 | `unknown` added to **both** `effects[].object_kind` and `effects[].content_kind`, with a total selection rule that never infers a kind across an unobserved transition, mandatory `preimage-unavailable` / `postimage-unavailable` company, an `ambiguous` disposition, a decode refusal on any complete record carrying it, and pinned no-capture pure-rename / mode-only / type-transition cohorts (§6.3, §6.4, ADR-036 D3/D5); **semantic emptiness** defined as zero bytes, whitespace-only bytes or any strict parse yielding zero normalized effects, all three keeping the existing `canonical-patch-empty` code with the raw patch hash bound whenever the file is present, and completeness predicate 1 tightened to require **at least one** normalized effect so `complete` beside `effects: []` no longer decodes (§6.3, §6.5); **P6 source truth corrected again** — `internal/workflow/implement.go:192-195` writes the raw response on the unmarshal-failure arm and, when that write succeeds, does **not** return the parse error but falls through to the provenance attempt and the ordinary `nil` return, so the successful raw write is a governed event publishing `recipe_present: true`, `recipe_decodable: false`, the raw-byte hash and `recipe-undecodable` at the common coverage-last point before that `nil` return, while a failed `WriteArtifact` is no event and owes nothing (§2.7, §6.15); `apply --mode execute` given a **total** seven-case ordered classifier — bidirectional coverage-binding refusal, the withheld-recipe refusal (rephrased as "coverage binds no executable recipe" rather than an accusation that generation withheld one), a present-but-undecodable recipe refused by the same named contract instead of a raw JSON error, a present-and-decodable recipe under incomplete coverage that **still executes** with a non-authority warning, the two legacy shapes unchanged, and complete coverage — with every other shape routed to the malformed refusal (§6.11, ADR-036 D17); presence binding made **bidirectional**, so `patch_present` / `recipe_present` are recomputed from on-disk readable existence and both `true`-beside-absent and `false`-beside-present are binding-stale (§6.11, §6.14, ADR-036 D9); every stale `§6.5 predicate 8` reclassification cross-reference retargeted to **predicate 10** and the parity guard widened to validate all `predicate N` references (§6.3, §6.5); the surface-mapping totality sentence scoped to the explicitly mapped subset, with the remaining surface codes named as verify/binding/aggregate conditions outside the schema-reason vocabulary and disjointness preserved (§6.12, §7); `openInEditor`'s call sites corrected from three to **two** and `$EDITOR`-unset modeled as no process, no byte change and no P7 event (§6.2, §6.15); the site mapping restated so each **reachable call chain** maps to exactly one producer and the `recipe_autogen` helper maps to P1 or P2 by caller (§6.15); P2's same-byte classification corrected to compare the **latest** generation (§6.15); `Recipe generated`'s operation count sourced from the derived recipe inside its actual `AutogenGenerated` branch, with the incomplete-coverage status emitted from the common post-autogen coverage output path (§6.1.3, §6.11); the `diff --git` fragment-boundary question closed — a line-start token inside a valid hunk is prefixed by `+`, `-` or a space and cannot be a boundary, a malformed bare token is parsed or refused as a new record, so grammar recognition wins and PI-12's semantics stay fixed (§6.1.2, §6.4); matrix rebuilt to 332 rows (I 77, C 74, G 74, U 105, S 2). |
| rev-6 | 2026-09-01 | **Effect classification reads the extant sides only** — an `add` whose postimage was observed and present takes `object_kind` **and** `content_kind` from that postimage even though nobody independently observed its preimage's absence, a `delete` takes both from its observed preimage on the same terms, and `unknown` is reached only when an **extant** side went unobserved with no decisive binary marker, so `modify` / `rename` / `copy` are the shapes that degrade; the unobserved non-extant side keeps its mandatory `preimage-unavailable` / `postimage-unavailable` reason, the `ambiguous` disposition and predicate-8 incompleteness, so availability reasons record observation while `unknown` records classification, and neither implies the other (§6.3, §6.4, §9.4, ADR-036 D3/D5); **`apply --mode execute` order 2 restated over readable existence** — it fires whenever valid `incomplete` coverage carries `recipe_present: false` and no **readable** recipe is on disk, covering an absent file and a present-but-unreadable one alike under the phrase "coverage binds no **readable** executable recipe", while order 1 fires only where recomputed readable existence **differs** from the stored flag, so a truthfully recorded unreadable recipe takes order 2 and a later readability change takes order 1 from either direction; the refusal gains a `recipe status` line naming the real read cause, and the fixtures inject the failure through the repository's file-access seam rather than an OS `chmod` (§6.11, §9.6, ADR-036 D9/D17); **owner mismatch split into its two real conditions** — a coverage envelope whose `feature` differs from the requested target slug is a D9 binding failure at rung 2, `recipe-coverage-owner-mismatch`, `block` / exit `2`, apply order 1, mapped to **no** schema reason; a decoded recipe whose `feature` differs from the coverage owner is the record-level schema reason `recipe-owner-mismatch`, fails predicate 3 and surfaces only through the aggregate `recipe-coverage-incomplete` row at rung 3 with `warn` / exit `0`, so rev-5's direct mapping between them is removed and the mapped subset becomes **seven** codes against **eleven** unmapped (§6.12, §6.14, §7, §9.4, §9.9, ADR-036 D9/D13); readable existence stated as a deliberate record-layer collapse of absence and unreadability whose diagnostics keep the true cause, with every readability transition treated as bidirectional binding drift for both bound artifacts (§6.11, §6.14, ADR-036 D3/D9); `reasons` and `reason_codes` required to equal **exactly** the applicable closed codes, sorted and deduplicated, with no applicable code optional (§6.4, §6.15, §9.4); `effects[].ordinal` defined as the one-based strict-grammar record order that `patch_fragment_sha256` boundaries follow (§6.4, §9.4); **P6's finalizer corrected** — `RunImplement`'s final statement is `return s.MarkFeatureState(...)` (`internal/workflow/implement.go:243`), not a literal `nil`, so coverage publishes after the recipe write, the provenance attempt and the state-mark attempt and before that return is propagated, still binding the recipe write when the state mark fails, with a coverage-publication failure surfaced rather than dropped (§2.7, §6.15, §9.1, §12); **P3's generation append corrected to the conditional it is** — the patch write at `internal/workflow/refresh.go:82` is unconditional while `AppendPatchGenerationForFeature` runs only when `newPatch != originalPatch` (`internal/workflow/refresh.go:93,102`), and the P3 event is the write (§2.7, §6.15, §9.1, §12); the stale §11 row claiming predicates 1-4 added artifact presence and reference durability corrected to predicates 1-3, and the superseded rev-3/rev-4 answer rows annotated where later revisions overrode them (§11); the coverage record's **own** readability decided the other way round from its presence fields — a coverage file that exists but cannot be read is present-and-unusable (rung 1, apply order 1) rather than absent, so rung 5 and orders 5-6 stay reserved for genuinely absent coverage (§6.11, §6.12, §6.13, ADR-036 D13/D17); matrix rebuilt to 351 rows (I 77, C 80, G 79, U 113, S 2). |
| rev-7 | 2026-09-01 | **`operation-missing` scoped to effects for which an operation was actually owed** — it is raised for a normalized effect **if and only if** that effect is otherwise representable in v1 (repository-safe path, supported change/content/object/mode domain, every side its `change_kind` names observed, no parent-created-target exclusion, no non-reclassifiable assigned operation) **and** the decoded recipe assigns it no operation; it is never raised because an operation was intentionally not emitted for an effect already carrying a capability, safety or availability exclusion, and an absent, unreadable or undecodable recipe raises it on the otherwise-representable effects only. `mismatch` is therefore exactly the singleton `["operation-missing"]`, co-occurrence with any other effect-local code is refused at decode, the precedence ladder's live cases become `ambiguous` over `unsupported`, and the positive missing case plus the negative unsupported and ambiguous cases are pinned in the cohort tables and the matrix (§6.3, §6.4, §9.4, §9.5, ADR-036 D3/D5); **contradictory observations defined and refused** — for a side the `change_kind` requires to exist, `*_observed: true` with `*_present: false` is invalid publication and schema input, the publisher marks such a side **unobserved** with its mandatory availability reason when it cannot establish it, and the strict validator rejects the impossible observed-absence shape, keeping axis selection total without inventing a reason code (§6.4, §9.4, ADR-036 D3); **the ADR-029 restatement corrected** — exact-postimage recognition narrows **both** the empty-preimage collision and the non-empty expected-hash mismatch when the observed bytes equal the operation postimage, while the missing-target expected-hash refusal, the unreadable-target refusal, path safety, all-or-nothing atomicity and ADR-029 D7 supersession severity are unchanged, so the "other three cases unchanged" claim is removed and the rows aligned (§6.7, §9.6, ADR-036 D7); **P2 pairing restated** — whenever the conditional `producer-patch-rewrite` applies, `recipe-not-regenerated` is paired with it, P2's specific policy no longer implies `recipe-not-regenerated` alone, and exact exhaustive-set semantics are enforced on the published arrays (§6.11, §6.15, §9.1, ADR-036 D3/D15); **coverage-publication failure surfaced non-zero on every P1-P7 event** — it may leave coverage absent or stale but never yields command success, expressed as one generic table-driven per-producer contract and a single matrix row rather than seven prose rows (§6.10, §6.15, §9.1, ADR-036 D10); **legacy `apply --mode execute` rows restated over readable existence** — coverage absent with no readable recipe keeps the existing `LoadRecipe` read / no-recipe error and exit `1`, coverage absent with a readable recipe keeps the existing execution, and physical unreadability follows the no-readable row (§6.11, §9.6, ADR-036 D17); **PI-10's four production display consumers listed** — `internal/cli/cobra.go:1863`, `internal/cli/feature_patch.go:163` and `internal/cli/record_collision.go:96` stay as human file counts, and only `internal/cli/cobra.go:1908`'s operation-count misuse is removed/migrated, so the two-consumer reading is corrected (§2.5.1, §6.1, §6.1.3, §9.2, §12); **P6 finalizer ordering pinned** — coverage finalization runs after the state attempt and before the return is propagated, still binds a successful recipe write when the state mark fails, and when publication also fails the coverage failure is surfaced with the state failure preserved and chained under the repository's tight error handling, with no success-shaped fallback (§6.15, §9.1, ADR-036 D10/D15); P3's generation row confirmed as the conditional append it is against the unconditional patch write (§6.15, §12); and `ordinal` stated as the canonical patch's strict-grammar record order wherever it is referenced (§6.4, §9.4); matrix rebuilt to 360 rows (I 77, C 83, G 81, U 117, S 2). |

## 1. Summary

`tpatch record` currently derives `apply-recipe.json` from the canonical patch
by listing touched paths and writing each non-deleted postimage as a whole-file
operation. The generator does not populate ADR-029 preimages, cannot represent
deletions, handles renames as destination writes, and does not prove that its
operations cover the canonical patch.

This PRD makes generated recipes truthful inputs to later safety analysis. It:

1. makes one strict grammar authoritative for every production consumer that
   claims a file path or an effect kind, over a complete derived parser
   inventory (PI-1..PI-12), in two documented projections — b-side and
   all-paths — and registers every remaining specialized parser as
   non-authoritative;
2. captures the exact reference/preimage observation before artifact writes,
   on every producer, and records **whether each image side was observed at
   all** rather than collapsing "not looked at" onto "not there";
3. emits preimage-bearing whole-file operations only where authority exists
   and only over the supported `100644` mode domain;
4. writes deterministic `artifacts/recipe-coverage.json` exactly as ADR-036 D3
   defines it, with orthogonal effect axes — including an explicit `unknown`
   on both classification axes when an **extant** side no producer observed
   leaves the axis unestablished — required
   patch presence, separate
   recipe presence and decodability, per-side observation flags, an exactly
   specified `patch_fragment_sha256`, a `disposition` that follows
   deterministically from its reason codes, and record-level `reasons[]`
   allocated separately from effect-local `reason_codes[]`;
5. obliges **all seven** governed producers — `record`,
   `feature patch refresh|fixup`, `reconcile --accept`, `cycle`,
   `apply --mode done`, `implement` and `tpatch edit` on a bound artifact — to
   publish coverage through one shared API on every governed **event**, where
   an event is a bound write, a manual checkpoint, or an explicitly contracted
   re-observation checkpoint, so no producer can silently invalidate another's
   `complete` record;
6. proves a recipe is record-generated by recomputing the canonical derivation
   and comparing full bytes, never by a stored origin label, and converges on
   every rerun including a no-op;
7. makes repeated preimage-bearing writes recognize exact postimages as no-op
   without changing ADR-029 D7's supersession severity;
8. records unsupported, unparseable, undecodable and **unobserved** inputs
   explicitly instead of publishing a partial recipe as complete or publishing
   nothing at all, and classifies **every** `apply --mode execute`
   coverage/recipe shape — refusing by name, with state-aware reachable
   guidance, when coverage binds no **readable** executable recipe, and
   executing an explicitly requested recipe with a non-authority warning when
   coverage is merely incomplete;
9. gives verify and doctor exact missing/stale/incomplete diagnostics with a
   pinned precedence ladder, in which no warning-class state ever authorizes
   replay.

Coverage proves generation completeness only. GH #13 must recompute every
binding and independently decide whether a recipe can produce a candidate
against another tree.

**ADR-036 is normative.** Where this document and ADR-036 could be read as
disagreeing about the schema, severity or publication order, ADR-036 governs.

## 2. Problem

### 2.1 Current generator loses effect truth

`parsePatchTouchedFiles` extracts the b-side path using `strings.Fields`.
`RecipeFromPatch` then:

- emits `write-file` from the live postimage;
- skips deleted paths;
- records a rename only as its destination;
- emits no `preimage_hash`;
- compares recipe drift by file set only.

These behaviors are in
`internal/workflow/recipe_autogen.go:45-122,151-251`.
A deleted source, rename source, mode change, binary hunk or quoted path can
therefore be missing or misrepresented while an apparently usable recipe is
written. An executable add is written back as a `0644` regular file
(`internal/workflow/recipe.go:207-211`) with no signal that the mode was
dropped.

### 2.2 Activating ADR-029 has migration consequences

ADR-029 D1-D4 is implemented in the apply path, but no generator populates the
field. Once record starts doing so:

- verify enters the V10 provenance path and blocks if
  `recipe-provenance.json` is absent
  (`internal/workflow/verify_anchored.go:816-840`);
- a repeated apply sees the postimage rather than the preimage
  (`internal/workflow/writefile_safety.go:108-170`);
- an empty-preimage child operation can collide with a path a hard parent
  legitimately creates;
- shared-file stacks become order-sensitive at the preimage gate;
- ADR-029 D7's supersession downgrade
  (`docs/adrs/ADR-029-write-file-recipe-safety.md:74-76`,
  `internal/workflow/writefile_safety.go:264-305,326-364`) starts firing on
  generated recipes for the first time. A new apply rule that refuses on drift
  unconditionally would silently revoke an accepted decision and turn every
  superseded historical feature into a hard failure at upgrade.

The feature is incomplete unless these are specified and tested in the same
wave.

### 2.3 Patch generation is context, not sufficient authority

`PatchGeneration` records patch hash, recipe hash, base, capture metadata and
touched paths (`internal/store/patch_generations.go:28-76`). A same-patch
recipe regeneration may append no generation, while the recipe hash changes.
Binding coverage only to the latest generation would make the supported
upgrade path permanently stale.

### 2.4 Persisted contextual operations are not safe today

Current `replace-in-file` execution uses the first substring match and writes
once. `append-file` always appends. Neither operation carries unique-anchor
authority or a general second-application proof
(`internal/workflow/recipe.go:142-198,213-242`).

The adjacent CLI argument case study demonstrates that contextual replay is
valuable, but also reproduces duplicate application and duplicate-anchor
hazards. This producer wave must not label those operations safe.

### 2.5 Production contains several naive patch-header readers, not two

`gitutil.FilesInPatch` is deliberately fail-soft: it splits each `diff --git`
header on the first ` b/` and silently skips every header it cannot split,
which includes every Git C-quoted path
(`internal/gitutil/gitutil.go:885-911`). Two production callers remain on it:

- `AppendPatchGenerationForFeature` fills `patch-generations.json`
  `touched_paths` (`internal/workflow/patch_generations.go:76`);
- `touchedPathsFromPostApplyPatch` feeds the reconcile derivation fallback
  (`internal/workflow/reconcile_derivation.go:118-124`).

Both are documented as advisory, which was defensible while nothing depended
on their totality. It stops being defensible the moment coverage claims every
effect is explained: a repository could hold coverage asserting a quoted path
is represented next to a patch generation whose `touched_paths` never
mentions it. One patch, two disagreeing path sets, one of them authoritative —
that is exactly the ambiguity GH #15 exists to remove.

**rev-1 stopped at those two, and that was wrong.** Two more production
readers derive paths or effect kinds from `diff --git` headers with their own
naive scanners:

- `parsePatchNoveltyPaths` assigns each path a
  `create`/`modify`/`delete`/`rename` action, taking paths from
  `parseDiffGitPaths`'s `strings.Fields` split and dequoting with
  `cleanPatchPath`'s `strings.Trim(path, "\"")`, which strips surrounding
  quotes without decoding a single C escape
  (`internal/workflow/file_novelty.go:130-231`). Its output feeds novelty
  classification and reconcile (`internal/workflow/reconcile.go:987`);
- `parsePatchHunks` attributes hunk ranges to paths through the same helper
  (`internal/workflow/hunk_overlap.go:150-175`).

A fifth reader exists, and **rev-2 described it incorrectly**.
`PathsAffectedByPatch` (`internal/gitutil/unapply.go:36-125`) is not a naive
splitter with "the same quoting gap":

- it decodes C-quoting through `strconv.Unquote` on every operand
  (`internal/gitutil/unapply.go:47-49`);
- its unquoted fallback disambiguates paths containing spaces by requiring the
  `a/` and `b/` payloads to be byte-identical
  (`internal/gitutil/unapply.go:105-121`);
- it deliberately returns the **union of both diff sides**, plus
  `rename from`/`rename to` and `copy from`/`copy to` operands, because a
  reverse rename recreates the a-side path and removes the b-side one. The
  function documents exactly that (`internal/gitutil/unapply.go:33-35`) and a
  shipped test pins it (`internal/gitutil/unapply_test.go:83-102`).

It is consumed at `internal/cli/cobra.go:919,1131` and
`internal/cli/feature_unapply.go:156`. rev-2's prescription — migrate it to
"the strict authority" — would have replaced a both-side union with
`FilesInPatchStrict`'s b-side projection
(`internal/gitutil/patch_paths_strict.go:235-236`) and dropped every rename and
copy **source** from unapply's snapshot and rollback scope. §6.1 replaces that
prescription with a new strict all-paths API.

Three further readers exist that are **not** path or effect authorities and
must not be migrated: `headerReferencedGitPath`
(`internal/store/store.go:504-540`) and `stripGitInternalFileStanzas` /
`headerPathIsGitInternal` (`internal/gitutil/gitutil.go:1170-1270`) recognize
non-Git diff dialects (`diff -ruN`, `Only in `, `Binary files `) for `.git`
containment and sanitization, and `countPatchFiles`
(`internal/cli/cobra.go:2094-2101`) is a display counter. A rule phrased as
"exactly one parser in production" is therefore false as stated. §6.1 states
the rule that is actually implementable.

### 2.5.1 One display counter is already feeding a decision-shaped number

`countPatchFiles` counts `diff --git` prefixes, which is a **file** count. It
has **four** production consumers, not two. Three of them are correct and stay:
`filesChanged` for `record.md` (`internal/cli/cobra.go:1863`), the `%d files`
field of `Amended patch for %s (%s, %d bytes, %d files)`
(`internal/cli/feature_patch.go:163`), and the `Files:` field of a collision
report entry (`internal/cli/record_collision.go:96`). The fourth is not: at
`internal/cli/cobra.go:1908` it is used as
`countPatchFiles(patch)-len(skippedPaths)` to print
`Recipe generated: artifacts/apply-recipe.json (%d ops)` — an **operation**
count for an artifact the scanner never read. The number is wrong whenever a
patch effect does not map one-to-one onto an operation, which after this PRD is
routine. §6.1 migrates that one consumer to the derived recipe's actual
operation count, in that same `case workflow.AutogenGenerated` arm — the only
branch where a recipe was in fact generated — and leaves the three file counts
untouched.

### 2.6 Whole-file writes are not cross-base safe, and nothing says so

The adjacent CLI argument case study
(`docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md:1-243`)
records a feature that adds two arguments while upstream deletes neighboring
ones. A generated whole-file `write-file` reproduces the feature's postimage
exactly against its own base, and would restore the deleted upstream arguments
if replayed against the new base. Nothing in the current artifacts
distinguishes "generation is complete" from "safe against another tree", so a
consumer reading `complete` could reasonably reach the wrong conclusion.

### 2.7 `record` is not the only writer of the inputs coverage binds

Coverage binds the canonical patch and the recipe. Six production paths other
than `record` write one of them:

| Path | Patch write | Recipe write | Generation append |
|---|---|---|---|
| `feature patch refresh\|fixup` | `internal/cli/feature_patch.go:114`, only when the captured patch is non-empty **and** classifies `Append == true` (`internal/cli/feature_patch.go:91-112`) | `AutogenRecipeForRecord(..., autogen=true, regenerate=false)` at `internal/cli/feature_patch.go:135`, reaching the shared writer at `internal/workflow/recipe_autogen.go:204` | `internal/cli/feature_patch.go:150` |
| `reconcile --accept` → `RefreshAfterAccept` | `internal/workflow/refresh.go:82`, **unconditionally** | none — deliberately left stale, documented at `internal/workflow/refresh.go:20-24` | `internal/workflow/refresh.go:102`, `capture.mode: reconcile`, but **only when `newPatch != originalPatch`** (`internal/workflow/refresh.go:93`) |
| `cycle` patch-capture step | `internal/cli/phase2.go:166`, only when the captured patch is non-empty (`internal/cli/phase2.go:165`) | none at this step | none |
| `apply --mode done` → `runApplyDone` | `internal/cli/cobra.go:982,1044`, only when the captured patch is non-empty (`internal/cli/cobra.go:1043`) | none | none |
| `implement` (`RunImplement`) | none | `internal/workflow/implement.go:194` on the JSON-unmarshal-failure arm and `internal/workflow/implement.go:209` on the valid-JSON arm — **neither arm returns the parse error**; each returns only its own `WriteArtifact` error, and a successful write falls through to the provenance attempt and to the final `return s.MarkFeatureState(...)` at `internal/workflow/implement.go:243` — plus `recipe-provenance.json` at `internal/workflow/implement.go:220-237` | none |
| `implement --manual` | none | checkpoints an externally authored recipe after existence and JSON validation, then advances state to `implementing` (`internal/cli/cobra.go:744`, `internal/store/manual.go:31,51-80`) | none |
| `tpatch edit <slug> [artifact]` | via `$EDITOR`, when the **resolved** path is `artifacts/post-apply.patch` | via `$EDITOR`, when the **resolved** path is `artifacts/apply-recipe.json` | none |

If only `record` published coverage, any of these could leave a valid,
bound, `complete` coverage record describing a canonical patch or recipe that
no longer exists on disk — a record whose hashes would still verify against the
*stored* values while describing the wrong artifact, or which would silently
flip to hash-stale with no producer accountable for it. Silently stale complete
coverage is strictly worse than no coverage: it is the one state a consumer
could act on and be wrong. §6.15 makes all seven producers accountable.

**rev-2 missed `implement` entirely**, which is the sharpest of the three
omissions: `RunImplement` is the ordinary way a recipe comes into existence,
and it sat outside the registry while `record`'s autogen — a *secondary* recipe
writer — sat inside it.

**rev-3 then described `implement`'s two writes wrongly.** They are not
"provider and heuristic" arms. The provider response and the heuristic fallback
are selected earlier (`internal/workflow/implement.go:176-190`) and both flow
into a single `json.Unmarshal` of the extracted JSON
(`internal/workflow/implement.go:192`). The two writes are that parse's
outcomes: `:194` writes the **raw** response verbatim when the unmarshal
**fails**, and `:209` writes the **reserialized** recipe when it succeeds.

**rev-4 then described the failure arm's control flow wrongly, and rev-5
corrects it.** rev-4 said the arm "returns its parse error" and that coverage
must be published "before" that return. The source does neither:

- the `json.Unmarshal` error is consumed by the `if` that selects the arm
  (`internal/workflow/implement.go:192`). It is never returned;
- the only `return` inside the arm is the **write** error
  (`internal/workflow/implement.go:194-196`). When `WriteArtifact` succeeds,
  control leaves the `if`/`else` and continues to the provenance attempt
  (`:214-237`) and the state mark, and `RunImplement` ends by **returning that
  state mark**:
  `return s.MarkFeatureState(slug, store.StateImplementing, "implement", "Apply recipe generated")`
  (`internal/workflow/implement.go:243`). rev-5 described this as a literal
  `nil` return; it is a returned call, which yields `nil` on the ordinary path
  and the state-mark error otherwise;
- so the load-bearing fact is not "publish before an error return" but **"a
  successful raw write is a governed event"**: an `apply-recipe.json` that does
  not decode now sits on disk and the command is about to report success.
  §6.15 requires that event to publish bound incomplete coverage —
  `recipe_present: true`, `recipe_decodable: false`, the raw-byte
  `recipe_sha256`, `recipe-undecodable` — at the common coverage-last point
  both arms fall through to: after the provenance and state-mark attempts and
  before that final return is propagated, so a failing state mark cannot
  swallow the record for a recipe write that already landed;
- and a **failed** `WriteArtifact` on either arm is **not** an event: nothing
  was written, the error propagates, and no coverage is owed. Publishing there
  would assert an observation of a write that never landed.

**`cycle` is a composite, not a single writer.** Its implement step
(`internal/cli/phase2.go:112`) writes the recipe and its patch-capture step
(`internal/cli/phase2.go:166`) writes the patch. Between them the command can
return: `--skip-execute` exits right after implement
(`internal/cli/phase2.go:122-126`), and two interactive prompts can decline
(`internal/cli/phase2.go:127-129,152-154`). rev-2's single "step `[6/6]`" row
could not express an obligation that is owed at the earlier step.

**`amend` writes neither bound artifact.** It replaces `request.md`
(`internal/cli/c1.go:207`) and feature metadata. No registry entry claims
otherwise; a claim that generic `amend` writes a recipe today would be false.
`implement --manual` does not author a recipe either — it *observes and
checkpoints* one an agent or human already wrote, and in doing so advances the
feature to `implementing` (`internal/store/manual.go:31,80`).

**`tpatch land` is not an eighth producer.** It composes an embedded `record`
step (`internal/cli/land.go:180`, `internal/cli/land.go:694-698`) and then
stages and commits. Every bound write on that path is `record`'s own, at
`record`'s own call site, so `land` is **P1 orchestration**. A registry entry
for `land` would double-count one write and would give two `producer` values to
a single publication.

**Two of those writes cannot currently report failure.** `cycle` discards the
error from `s.WriteArtifact(slug, "post-apply.patch", patch)`
(`internal/cli/phase2.go:166`), so the command cannot distinguish a successful
write from a failed one. `openInEditor` likewise discards the editor's exit
status (`_ = c.Run()`, `internal/cli/phase2.go:261`), so `tpatch edit` cannot
tell a clean editor exit from a crashed one. Publication follows a *successful*
event, so the implementation propagates both errors (§6.2, §6.15).
`openInEditor` has exactly **two** call sites — `internal/cli/c1.go:91` and
`internal/cli/phase2.go:88` — and only the first can resolve to a bound
artifact; the second opens `spec.md` and is never a P7 event. It also starts no
process at all when `$EDITOR` is unset (`internal/cli/phase2.go:252-255`),
which is likewise not an event (§6.15).

**Direct filesystem edits are outside the CLI boundary.** An operator who edits
`.tpatch/features/<slug>/artifacts/apply-recipe.json` without going through
`tpatch` runs no `tpatch` process, so no producer can observe or publish
anything. That is external tamper. It is not governed at write time and it is
not silently accepted either: read-time hash recomputation (§6.14) turns it
into binding-stale coverage, which verify blocks on and GH #13 refuses.

### 2.8 Nothing in the artifacts distinguishes a generated recipe from a manual one

`recipe-provenance.json` is required by verify as soon as any `write-file`
carries a preimage (`internal/workflow/verify_anchored.go:816-840`), but
record must only write it for recipes it actually generated — fabricating a
base and time for a hand-authored or provider-authored recipe is the
historical-adoption move GH #19 owns.

Today the only available discriminators are both inadequate:

- **an action label** (`AutogenGenerated`, `AutogenNoop`, …,
  `internal/workflow/recipe_autogen.go:126-132`) describes what *this* run
  did. On a rerun it says `noop` for a generated recipe and a manual recipe
  alike, and any persisted form of it would be a self-asserted, hand-editable
  claim — precisely the trust shortcut ADR-036 D9 forbids everywhere else;
- **file-set equality** (`compareRecipeFileSets`,
  `internal/workflow/recipe_autogen.go:211-251`) is satisfied by a manual
  recipe with entirely different operations over the same paths.

rev-1 leaned on the informal notion "this producer generated it, now or on a
prior run" without saying how a later run establishes that. §6.16 replaces it
with a recomputation.

## 3. Goals

1. Every generated recipe is bound to the exact patch and reference
   observation used to derive it.
2. Every canonical patch effect is represented or has a closed incomplete
   reason, and an absent, empty or unparseable canonical patch — and a present
   but undecodable recipe — each has an explicit incomplete encoding of its
   own, with its raw bytes still bound.
3. Applying all represented operations to the captured preimage reproduces the
   exact postimage effect set within the supported `100644` mode domain.
4. A generated existing-file write carries an exact ADR-029 preimage.
5. A generated creation carries explicit absent-target authority.
6. Fresh record-generated preimage recipes pass verify with truthful
   provenance, and a rerun repairs missing or stale provenance on every path
   including the recipe no-op.
7. Repeated apply of an exact postimage is a no-op that still reports success.
8. Missing or stale coverage is repairable by rerunning record even when patch
   bytes and patch generation are unchanged.
9. Coverage bytes are deterministic and contain no source bodies or
   timestamps, and every field a consumer must check is recomputable from
   artifacts the consumer already holds.
10. GH #13 can reject incomplete, stale or non-reconstructable inputs without
    trusting producer claims, and no warning-class verify state authorizes
    replay.
11. ADR-029 D7's supersession severity survives the migration unchanged, and
    upgrading an existing repository to v0.17.0 introduces no new verify
    failure — including a repository carrying pre-v0.17.0 `recipe-stale.json`
    markers.
12. Exactly one patch grammar answers "which paths and effects does this patch
    contain" for every production consumer that claims a path or an effect
    kind, in two documented projections, and every remaining specialized parser
    is registered as claiming neither.
13. Every production writer or checkpointer of a bound patch or recipe input
    leaves truthful coverage; none may leave formerly complete coverage
    silently stale, and none may leave silence where a bound artifact changed.
14. Whether a recipe is record-generated is decided by recomputation on the
    current run, never by a stored label.
15. Every state a producer can leave behind is legible at the point an operator
    meets it: no command reports a misleading cause, no command silently
    substitutes a different operation for the one requested, and every command
    a diagnostic names is one the operator can run from the state printed
    beside it.

## 4. Non-goals

- Automatically replaying a recipe during reconcile.
- Persisting contextual anchors.
- Adding delete, rename, copy or mode-change operations.
- Representing executable (`100755`), symlink or gitlink effects.
- Changing `replace-in-file` or `append-file` schema or execution semantics.
- Changing legacy missing-preimage compatibility.
- Changing ADR-029 D7 supersession severity.
- Reusing a hard parent's postimage as a child's preimage.
- Adopting historical/manual recipe provenance (GH #19).
- Regenerating recipes inside `reconcile --accept`, `cycle` or
  `apply --mode done`; those producers publish honest incompleteness instead,
  and GH #13 owns regeneration from accepted operation candidates.
- Truthfully recomputing a `tpatch edit` mutation in the general case; P7
  publishes `manual-bound-artifact-edit` unless a prior durable reference
  reconstructs and validates.
- Governing direct filesystem edits to bound artifacts at write time; they are
  external tamper, detected at read time.
- Changing `FilesInPatchStrict`'s b-side result for its existing callers.
- Adding an `apply --mode reapply` flag value, or changing which state selects
  the existing canonical-patch reapply branch.
- Adding any `tpatch` command that materializes a canonical patch from a
  non-reapplying state; §6.11 names reviewed external `git apply` instead.
- Governing a GUI editor's post-exit save; it is external tamper caught at
  read time.
- Migrating the non-authoritative `.git`-containment, sanitization and
  display-count patch scanners (PI-8, PI-9, PI-10) — beyond removing PI-10's
  one illegitimate operation-count consumer.
- Treating coverage completeness as replay eligibility or cross-base safety.
- Cross-file atomic publication.
- Automatically repairing coverage through `doctor --fix`.
- Rewriting Git history.

## 5. Existing primitives pre-flight

| Primitive | What it already proves | Why it is insufficient |
|---|---|---|
| `apply-recipe.json` | Ordered operation payload and feature owner | Does not prove canonical patch coverage |
| `preimage_hash` | Exact-byte/absent-target authority for write-file | No production generator populates it |
| `recipe-provenance.json` | Generation base/time and recipe hash | Does not enumerate patch effects; absent on record autogen; has no rerun repair rule |
| `patch-generations.json` | Append-only patch/recipe/base/capture history | Same-patch recipe regeneration may append nothing |
| `recipe-stale.json` | Existing recipe file-set drift warning | File-set comparison cannot prove byte/effect coverage |
| canonical `post-apply.patch` | Authoritative feature diff | Does not say how recipe operations explain it |
| `FilesInPatchStrict` | Hardened path grammar, b-side projection | Does not yet return complete normalized effects/preimages, and its b-side result is not the both-side rollback scope unapply needs |
| `FilesInPatch` | Advisory path list for two manifests | Fail-soft by design; silently drops C-quoted paths, so it cannot back a totality claim |
| `parseDiffGitPaths` / `cleanPatchPath` | Path plus change kind for novelty and hunk attribution | `strings.Fields` split and quote-trimming dequote; no C-escape decoding, no effect totality |
| `PathsAffectedByPatch` | Quoting-aware both-side reverse-apply/unapply scope, including rename/copy sources | Returns no error, so a caller cannot fail closed on a header the grammar would refuse |
| `countPatchFiles` | `diff --git` prefix count for display | A file count; it is not an operation count, and using it as one (`internal/cli/cobra.go:1908`) misreports the recipe |
| `AutogenAction` labels | What this run did to the recipe | Describes the run, not the artifact's origin; says `noop` for generated and manual recipes alike |
| `compareRecipeFileSets` | Same-path-set drift signal | Satisfied by a manual recipe with entirely different operations over those paths |
| ADR-029 apply precheck | All-or-nothing write-file preconditions | Does not recognize exact postimage as already applied |
| ADR-029 D7 supersession downgrade | Severity policy for historical drift | Answers severity, not whether generation was complete |
| `RecipeExecResult.Applied` / `.Skipped` | Per-run operation accounting | No state today means "succeeded without writing" |
| `write-file` execution | Deterministic whole-file write | Fixed `0o644` mode, so it cannot reproduce `100755` |
| later-touch warnings | Shared path was touched later | Warning is not coverage or replay authority |
| verify `write_file_preimage_fresh` (V10) | Preimage freshness at a named tree | Says nothing about whether the recipe explains the patch |
| doctor `D1`-`D9` registry | Read-only and guarded-fix evidence checks | No check inspects recipe coverage |

No existing object can carry the new responsibility without conflating
history, provenance or consumer eligibility. A dedicated coverage sidecar is
therefore required. In particular, no existing field can carry a parent's
postimage authority for a `created_by` child (§6.8) and no existing field can
carry cross-base scope (§6.9); the former is refused in v1 and the latter is a
new coverage field, not a recipe field.

## 6. Product contract

### 6.1 One authoritative effect grammar over a derived parser inventory

The generator consumes the same strict patch grammar used by hardened patch
path validation. `parsePatchTouchedFiles`
(`internal/workflow/recipe_autogen.go:45-84`) is removed rather than extended.

For each file-level effect, the parser produces:

- one-based ordinal;
- `change_kind`: `add`, `modify`, `delete`, `rename` or `copy` — read from the
  record header, so it is never `unknown`;
- `content_kind`: `text`, `binary`, `none` or `unknown`;
- `object_kind`: `regular`, `executable`, `symlink`, `gitlink` or `unknown`;
- canonical path and optional old path;
- old/new mode, read from the immutable observation and corroborated by the
  headers;
- exact preimage/postimage **observation** flags, existence flags and hashes,
  where an unobserved side forces `unknown` on the axes it would have decided
  (§6.3);
- `patch_fragment_sha256`, the digest of the effect's exact raw byte range in
  the canonical patch (§6.4);
- parse/refusal reason where complete interpretation is impossible.

The three kind axes are orthogonal and are decided independently (ADR-036 D3):
a binary rename is `rename` + `binary` + `regular`, a symlink delete is
`delete` + `text` + `symlink`, and an executable rename is `rename` + `text` +
`executable`.

Paths remain repo-relative and are passed through existing path safety.
C-quoted paths and spaces are decoded by the strict grammar. Contradictory
headers, unsupported quoting, duplicate destinations and path escape refuse
generation authority rather than falling back to a partial scan.

**The ownership rule, stated so it is implementable** (ADR-036 D1). rev-1
claimed production would contain "exactly one" patch grammar while naming two
callers; §2.5 lists the readers that claim disproves. The rule v0.17.0
actually enforces is:

> One strict normalized effect grammar is authoritative for every production
> consumer that claims a file path or an effect kind. A thin adapter may
> project paths, hunk ranges or per-path actions out of that grammar's
> normalized effect set. A specialized parser may remain only where it asserts
> neither path nor effect authority, and every such parser is registered and
> guarded.

S1 migrates the complete derived inventory, not a subset:

| ID | Caller | Today | After S1 |
|---|---|---|---|
| PI-1 | `parsePatchTouchedFiles` (`internal/workflow/recipe_autogen.go:45-84`) | `strings.Fields` b-side split; path + effect | **removed** |
| PI-2 | `gitutil.FilesInPatch` (`internal/gitutil/gitutil.go:885-911`) | fail-soft ` b/` split | **deleted or demoted to a test-only helper**; unreachable from production |
| PI-3 | `AppendPatchGenerationForFeature` → `touched_paths` (`internal/workflow/patch_generations.go:76`) | PI-2 consumer | strict normalized path authority, or an exact adapter over it; the strict error propagates to this caller's **existing** continue-on-error handling |
| PI-4 | `touchedPathsFromPostApplyPatch` (`internal/workflow/reconcile_derivation.go:118-124`) | PI-2 consumer | same authority; the strict error propagates to this caller's **existing** continue-on-error handling |
| PI-5 | `parsePatchNoveltyPaths` + `parseDiffGitPaths` + `cleanPatchPath` (`internal/workflow/file_novelty.go:130-231`) | naive split and quote-trim; claims path **and** change kind | adapter projecting path and `change_kind` from the strict effect set |
| PI-6 | `parsePatchHunks` (`internal/workflow/hunk_overlap.go:150-175`) | PI-5 helper for path attribution | path attribution from the strict adapter; the hunk-range projection is unchanged |
| PI-7 | `PathsAffectedByPatch` + `pathsFromDiffGitHeader` (`internal/gitutil/unapply.go:36-125`) | **quoting-aware** both-side union including rename/copy sources; returns no error | new strict **all-paths** projection `PathsAffectedByPatchStrict`; all three call sites fail closed on its error |
| PI-8 | `headerReferencedGitPath` (`internal/store/store.go:504-540`) | multi-dialect `.git` containment refusal; **no** path/effect authority | retained, registered, guarded |
| PI-9 | `stripGitInternalFileStanzas` + `headerPathIsGitInternal` (`internal/gitutil/gitutil.go:1170-1270`) | multi-dialect sanitizer; **no** path/effect authority | retained, registered, guarded |
| PI-10 | `countPatchFiles` (`internal/cli/cobra.go:2094-2101`) | display counter with **four** production consumers: correct file counts at `internal/cli/cobra.go:1863`, `internal/cli/feature_patch.go:163` and `internal/cli/record_collision.go:96`, and **incorrectly an operation count** at `internal/cli/cobra.go:1908` | retained, registered, guarded as a **human file count only**; the three file-count consumers are unchanged and the operation-count consumer is removed/migrated |
| PI-11 | `FilesInPatchStrict` and its grammar (`internal/gitutil/patch_paths_strict.go:235-253`) | strict header grammar, b-side projection | **the authority**, extended to the full normalized effect model; the b-side projection keeps its exact current result |
| PI-12 | Existing `FilesInPatchStrict` b-side callers: `internal/cli/land.go:767,1212`, `internal/workflow/refresh.go:59`, `internal/workflow/verify_landed.go:1009,1163` | strict b-side path list | **unchanged contract**; a regression guard pins what they receive |

An adapter is acceptable only if it derives its output from the strict
normalized effect set and propagates the strict error. It may not re-implement
header splitting, dequoting or `a/`/`b/` stripping, and it may not swallow the
error to preserve today's short list.

A **source-inventory guard** derives this list from production sources rather
than trusting the table: it enumerates every production `diff --git` reader and
requires each to be PI-11, a registered adapter, a registered b-side consumer,
or a registered non-authoritative scanner. A new unregistered reader fails the
guard, and a registered non-authoritative scanner whose output starts feeding a
path, effect or operation decision fails the authority-boundary guard.

#### 6.1.1 PI-7 gets a new strict projection, not the b-side one

rev-2 recorded PI-7 as "own splitter over both sides; same quoting gap" and
prescribed migration to "the strict authority". Both halves were wrong, and
acting on them would have been a safety regression:

- `PathsAffectedByPatch` already decodes C-quoting
  (`internal/gitutil/unapply.go:47-49`) and already disambiguates unquoted
  paths containing spaces (`internal/gitutil/unapply.go:105-121`);
- it deliberately returns **both** diff sides plus `rename from`/`rename to`
  and `copy from`/`copy to` operands
  (`internal/gitutil/unapply.go:33-35,80-91`), because unapply reverse-applies
  a patch: a reverse rename recreates the a-side path and removes the b-side
  one, so both must be in the snapshot and rollback scope;
- `FilesInPatchStrict` returns the **b-side path only**
  (`internal/gitutil/patch_paths_strict.go:235-236`). Projecting PI-7 onto it
  would drop every rename and copy source from unapply scope — the reverse
  apply would recreate a file at a path nobody snapshotted.

S1 therefore adds a second strict entry point over the same grammar:

```go
// PathsAffectedByPatchStrict returns the sorted, unique union of every
// path a patch touches on either side: each effect's canonical path plus
// its old_path for rename and copy effects. It preserves the rollback
// scope PathsAffectedByPatch provides today and refuses, with an error,
// every input FilesInPatchStrict refuses.
func PathsAffectedByPatchStrict(patch string) ([]string, error)
```

The name may vary; the signature and semantics may not. Both projections come
from the same normalized effect set — one grammar, two documented views.

All three production call sites migrate and **fail closed**:

| Call site | Today | After S1 |
|---|---|---|
| `internal/cli/cobra.go:919` — `apply --mode execute` reapply snapshot | `uniqueSortedPaths(gitutil.PathsAffectedByPatch(canonical))` feeds `SnapshotWorktreePaths`; no error channel | strict call; a parse error returns before the snapshot and before any mutation |
| `internal/cli/cobra.go:1131` — `validateReapplyMaterialization` | `gitutil.PathsAffectedByPatch(canonical)` feeds `DiffFromCommitForPaths`; no error channel | strict call; a parse error returns before the diff is computed |
| `internal/cli/feature_unapply.go:156` | `gitutil.PathsAffectedByPatch(patch)` feeds `ValidateWorktreePaths` and the unapply scope; no error channel | strict call; a parse error returns before any snapshot, reverse patch or artifact write |

**None of these three has an existing fail-soft handler.** The migration adds a
new refusal path at each, and this PRD does not claim to reuse one. That claim
would be true only of PI-3 and PI-4, whose callers already tolerate an error.

#### 6.1.2 PI-12's b-side contract is frozen

Five shipped call sites consume `FilesInPatchStrict`'s b-side list today:
`internal/cli/land.go:767`, `internal/cli/land.go:1212`,
`internal/workflow/refresh.go:59`, `internal/workflow/verify_landed.go:1009`
and `internal/workflow/verify_landed.go:1163`. Their semantics are **unchanged**
by this PRD: they receive exactly the b-side paths they receive today, in the
same order, with the same refusals.

This is stated as a contract because extending the shared grammar is the
obvious way to break it. Adding rename and copy sources to
`FilesInPatchStrict`'s output would silently widen a landed-patch file set, a
refresh comparison and two verify path scopes — none of which asked for
rollback scope. The both-side union lives in `PathsAffectedByPatchStrict`, a
separate function with a separate name, and a regression row pins the b-side
result.

**§6.4's fragment boundaries do not widen it either.** A `patch_fragment_sha256`
range begins at a record start the strict grammar already recognizes, so the
two shapes that could look like a boundary are decided by the grammar rather
than by a scanner: inside a **valid** unified-diff hunk every body line carries
a `+`, `-` or space prefix, so an embedded `diff --git` is not at line start and
cannot open a fragment; a **bare** line-start token outside a valid hunk is
parsed as the start of a new file record, or the patch is refused as
`canonical-patch-unparseable`. Grammar recognition wins in both cases, and
PI-12's b-side semantics are untouched — fragments reuse PI-11's record
offsets and introduce no new projection and no leniency.

#### 6.1.3 PI-10 stops being used as an operation count

`countPatchFiles` counts `diff --git` prefixes. **Four** production sites
consume it, and rev-6's two-site description understated that:

| Consumer | Use | Disposition |
|---|---|---|
| `internal/cli/cobra.go:1863` | `filesChanged` for `record.md` | correct file count; unchanged |
| `internal/cli/feature_patch.go:163` | `%d files` in `Amended patch for %s (%s, %d bytes, %d files)` | correct file count; unchanged |
| `internal/cli/record_collision.go:96` | `Files:` in a collision report entry | correct file count; unchanged |
| `internal/cli/cobra.go:1908` | `(%d ops)` in `Recipe generated: artifacts/apply-recipe.json` | **operation** count the scanner cannot compute; removed and migrated |

At `internal/cli/cobra.go:1908` it is used as
`countPatchFiles(patch)-len(skippedPaths)` to print
`Recipe generated: artifacts/apply-recipe.json (%d ops)`, which is an operation
count the scanner cannot compute.

After this PRD that one line prints the **actual operation count of the recipe
the producer just derived**, and it stays where it is: inside the
`case workflow.AutogenGenerated` arm of the shipped autogen switch
(`internal/cli/cobra.go:1906-1908`). That arm is by construction the branch in
which a recipe *was* generated, so it is the only branch that can hold a real
operation count. A withheld partial derivation (§6.3) never enters it, so the
incomplete coverage status is **not** printed from `cobra.go:1908` — rev-4 said
it was, which attributed an output to a branch that cannot execute in that
case. The status line is emitted from the **common post-autogen coverage output
path** that every autogen outcome reaches, and it never prints a computed file
count. PI-10 stays registered as a human file counter, its three legitimate
file-count consumers keep their current output byte-for-byte, and the
authority-boundary guard fails any reuse of it as an operation, effect or
eligibility count.

The behavior change is deliberate and stated in the release notes: a C-quoted
path that used to vanish from `touched_paths`, from novelty classification or
from hunk attribution now either appears or produces an error, and a malformed
header that reverse-apply scope used to ignore now refuses the command.

### 6.2 Capture reference authority, on every producer

**Every governed producer captures its immutable observation before its first
bound write** (ADR-036 D2), not just `record`. rev-2 stated the rule for
`record` only, which left P2-P7 free to derive coverage from bytes they had
already overwritten. The captured input is:

```text
canonical patch bytes (or explicit absence)
capture mode/pathspecs/claim ids
typed reference descriptor
ordered preimage observations
ordered postimage observations
normalized effects
```

Per-producer:

| Producer | Captured before |
|---|---|
| P1 `record` | its recipe, provenance, generation and coverage writes |
| P2 `feature patch refresh\|fixup` | the patch write at `internal/cli/feature_patch.go:114`, so the pre-refresh patch and recipe are both observable; the same observation serves P2's non-writing category-(c) checkpoint (§6.15) |
| P3 `reconcile --accept` | the patch rewrite at `internal/workflow/refresh.go:82` |
| P4 `cycle` patch-capture step | the patch write at `internal/cli/phase2.go:166` |
| P5 `apply --mode done` | the patch write at `internal/cli/cobra.go:1044`, inside the existing discovery-before-writes window (`internal/cli/cobra.go:999-1039`) |
| P6 `implement` | its recipe write on either parse arm (`internal/workflow/implement.go:194,209`) or, for `--manual`, its checkpoint of the externally authored bytes |
| P7 `tpatch edit` | invoking `$EDITOR`, with a matching **after** snapshot of the same **resolved** path once the editor process returns (`internal/cli/phase2.go:251-262`) |

**Each observation records whether each side was looked at, not only what was
found.** Every effect carries `preimage_observed` and `postimage_observed`
alongside `preimage_present` and `postimage_present` (§6.4). rev-3 had only the
presence pair, so a producer that could not read a side published the same
`false` as a producer that proved the path absent — which made the P6 and P7
`no-capture` shapes untruthful rather than merely thin.

**No origin proof and no `complete` status without a complete immutable
observation.** A partial observation yields `preimage-unavailable` or
`postimage-unavailable` on the affected effects, and a missing reference yields
`reference-not-durable` at the record level. §6.16's byte-equality proof
consumes the observation, so it is unavailable to a producer that did not take
one.

**`openInEditor` must return its error, and P7 must snapshot regardless**
(ADR-036 D2). `openInEditor` is synchronous only as far as the configured
process: it blocks on `c.Run()` and discards the result
(`internal/cli/phase2.go:261`). It has exactly **two** call sites —
`internal/cli/c1.go:91`, which is the `tpatch edit` path and the only one that
can resolve to a bound artifact, and `internal/cli/phase2.go:88`, which opens
`spec.md` and is therefore never a P7 event. rev-4 said three; the shipped tree
has two, and a refactor plan sized for three would leave one caller
unaccounted. Three things follow, and all are part of this
contract rather than implementation detail:

- the refactor makes `openInEditor` **return** the `c.Run` error and **both**
  call sites propagate it. A producer that cannot tell whether the editor ran
  cannot say truthfully what it observed;
- **an unset `$EDITOR` is not an event.** `openInEditor` prints
  `  (set $EDITOR to review <path> in your editor)` and returns without
  starting a process (`internal/cli/phase2.go:252-255`). No process runs, so no
  byte can change, so P7 observes no mutation and publishes nothing. This is
  modeled explicitly rather than left to fall out of the byte comparison, so
  that an implementation cannot "helpfully" publish a no-op record on a path
  where nothing happened;
- an editor error does not excuse publication. P7 takes its **after** snapshot
  even when `c.Run` fails, and when the bytes changed it publishes — or
  invalidates — coverage **before** returning that error. Otherwise a failed
  editor that still saved leaves a mutated bound artifact beside coverage
  claiming the old bytes.

A GUI editor that forks and exits before the operator's later save is a
different case and is stated as such: at the moment `c.Run` returns, no
mutation is observable, so P7 correctly publishes nothing. The later save runs
with no `tpatch` process alive, which makes it **external tamper** — caught at
read time as binding-stale coverage (§6.14), not covered by P7.

**P7 is the weak case and is treated as such.** `tpatch edit` observes a
mutation without holding any capture of the tree those bytes describe. Its
coverage carries `reference.kind: unavailable` and the
`manual-bound-artifact-edit` reason **unless** it can independently reconstruct
*and validate* a prior durable reference from the feature's own artifacts — the
pre-edit coverage's reference plus a recomputed `preimage_set_sha256` that
still matches. Reconstructed-and-validated is the only way P7 carries a
`commit` reference forward; remembering or copying one is not.

Reference kinds:

| Kind | Meaning | Emitted by a shipped v1 record mode | Reconstructable by GH #13 |
|---|---|---|---|
| `commit` | Exact resolved commit supplied the preimage | Yes | Yes |
| `index-snapshot` | Exact record-time index supplied the preimage | **No** | No, unless separately committed and bound |
| `unavailable` | No trustworthy preimage was captured | Yes, when no commit resolves | No |

Every shipped record capture mode maps to `commit` or `unavailable`
(ADR-036 D2). `index-snapshot` stays in the schema for future or internal
callers whose preimage genuinely is an uncommitted index state; a v1 record
run that emits it is a bug, and a consumer that reads it treats the reference
as non-durable.

| Record mode | Reference kind | Preimage source |
|---|---|---|
| `working-tree-all` | `commit` | resolved HEAD |
| `staged-index` | `commit` | resolved HEAD |
| `unstaged-worktree` | `commit` | resolved HEAD |
| `committed-range`, `auto-committed-range`, `explicit-committed-range` | `commit` | resolved lower commit |
| `reconcile` | `commit` | the accepted upstream commit `RefreshAfterAccept` diffs against (`internal/workflow/refresh.go:78-95`) |
| `no-capture` | `unavailable`, or a reconstructed-and-validated `commit` | none — the producer ran no capture (P6, P7) |
| any mode whose commit cannot be resolved | `unavailable` | none |

`reconcile` belongs to the `reconcile-accept` producer (§6.15), not to
`tpatch record`; it is the mode that path already writes into
`patch-generations.json` (`internal/workflow/refresh.go:113`). The other
patch-writing producers capture a whole working tree and reuse
`working-tree-all`: `feature patch refresh|fixup` via
`CapturePatchScoped(s.Root, nil)` (`internal/cli/feature_patch.go:88`),
`cycle` and `apply --mode done` via `gitutil.CapturePatch`
(`internal/cli/phase2.go:160-166`, `internal/cli/cobra.go:1025-1044`).

`no-capture` is the mode of the two producers that write or checkpoint a bound
artifact without running a patch capture: `implement` (P6) and `tpatch edit`
(P7). It carries empty `pathspecs` and `claim_ids`, and it licenses nothing: a
`no-capture` producer still binds the hashes it observed and still publishes
`reference.kind: unavailable` unless the reconstruct-and-validate path
succeeds.

**`--unstaged` is commit-kind HEAD, not an index snapshot.** Record refuses
the capture outright when any path carries both staged and unstaged edits, and
it does so before capture runs:

```text
record --unstaged refuses: staged and unstaged edits both touch <paths>.
Commit, stash, or split the staged edits, then rerun
```

(`internal/cli/cobra.go:1549-1556`, backed by
`gitutil.StagedUnstagedOverlap` at `internal/gitutil/capture_modes.go:275-328`.)
Therefore every path that survives into an accepted `--unstaged` patch has an
index entry identical to HEAD, and HEAD is its exact preimage. Unrelated
staged paths elsewhere in the index are reported as an advisory note
(`internal/cli/cobra.go:1561-1565`) and are not in the captured effect set, so
they cannot contaminate the reference. Classifying this mode as
`index-snapshot` would have made the single most common dirty-tree capture
permanently ineligible for no reason.

`preimage_set_sha256` is computed over canonical JSON containing ordered
path/kind/mode/existence/content-hash observations. It proves observation
identity, not durable reconstructability.

### 6.3 Recipe generation policy

The supported generation and simulation domain is regular `100644` files on
both sides of the effect (ADR-036 D5).

**Patch presence is decided first.** Coverage explains how a recipe covers a
patch, so with no readable patch there is nothing to explain and no
completeness claim to make:

| Canonical patch | Published record |
|---|---|
| absent | `patch_present: false`, `patch_sha256: ""`, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `reasons` containing `canonical-patch-missing` |
| present, **semantically empty** — zero bytes, whitespace-only bytes, or a strict parse that succeeds and yields **zero** normalized effects | `patch_present: true`, `patch_sha256` = the digest of the exact raw patch bytes (the empty-byte-string digest in the zero-byte case), `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `reasons` containing `canonical-patch-empty` |
| present, non-empty, strict grammar refuses it | `patch_present: true`, `patch_sha256` = the digest of the exact raw patch bytes, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `reasons` containing `canonical-patch-unparseable` |
| present, strictly parsed into **at least one** normalized effect | the ordinary derivation below |

**Emptiness is semantic, not byte-length.** rev-4 defined the empty row as
"zero bytes", which is narrower than the condition that matters: a
whitespace-only patch, and a patch the grammar parses cleanly into zero
effects, reach the identical dead end — nothing to cover — while satisfying
"every effect is represented" vacuously. All three are the same state, they
keep the existing `canonical-patch-empty` code rather than gaining new ones,
and **the raw patch hash is bound whenever the file is present**, in every one
of them. Completeness predicate 1 is tightened to match (§6.5): a complete
record requires at least one normalized effect, so `complete` beside
`effects: []` no longer decodes at all.

**Recipe presence and decodability are two separate questions**, and the same
shape rule governs them:

| Recipe file | Published record |
|---|---|
| absent, **or present but unreadable** | `recipe_present: false`, `recipe_decodable: false`, `recipe_sha256: ""`, every `operation_indexes` empty, `coverage_status: incomplete`. Presence is **readable existence** (ADR-036 D3): the two physical states deliberately collapse to one record value, while the read-error diagnostic keeps the actual cause. `recipe-undecodable` is not raised — nothing was read, so nothing failed to decode |
| present, does not strict-decode | `recipe_present: true`, `recipe_decodable: false`, `recipe_sha256` = the digest of the exact raw recipe bytes, every `operation_indexes` empty, `coverage_status: incomplete`, `reasons` containing `recipe-undecodable` |
| present, strict-decodes | `recipe_present: true`, `recipe_decodable: true`, ordinary operation assignment |

The empty-patch row is not pedantry. "Every effect is represented" is vacuously
true over an empty effect list, and so is "every represented effect is a
creation" — so an unstated semantically-empty case would have published the
schema's most permissive record, `complete` plus `reference-tree-only`, for a
feature that changes nothing. All of it is forbidden explicitly, in the table
above and in predicate 1.

The unparseable and undecodable rows exist for the mirror-image reason. A
producer holding an artifact it cannot interpret has three dishonest options —
claim the artifact is absent, publish a list it did not derive, or publish
nothing — and one honest one: bind the raw bytes it does hold and say plainly
that it could not read them. **A strict parse or decode refusal never leaves a
feature with no coverage after a producer event.** Binding the raw hash is what
keeps §6.14's recomputation working: an out-of-band corruption of a recipe is
then distinguishable from its deletion, which rev-3's single `recipe_present`
flag made impossible.

For each effect:

- **new regular `100644` file**: emit `write-file` with non-nil
  `preimage_hash: ""`;
- **modified existing regular `100644` file with durable preimage**: emit
  `write-file` with `preimage_hash: "sha256:<exact preimage bytes>"`;
- **existing safe richer recipe**: preserve it unless explicit regeneration
  was requested, then simulate it against the captured preimage;
- **`change_kind` of `delete`, `rename` or `copy`**: emit no lossy substitute
  and record the matching incomplete reason;
- **`content_kind: binary`**: emit no operation and record
  `effect-binary-unsupported`;
- **`object_kind` of `executable`, `symlink` or `gitlink`, including an
  executable add**: emit no operation and record the matching mode reason;
- **pure mode change** (`modify` with equal content hashes and differing
  modes): emit no operation and record `effect-mode-only-unsupported`
  alongside the out-of-domain mode's own code;
- **unsafe path**: emit no operation and record `path-unsafe`;
- **unreadable or unobservable preimage**: emit no operation, set
  `preimage_observed: false`, and record `preimage-unavailable`;
- **unreadable or unobservable postimage**: emit no operation, set
  `postimage_observed: false`, and record `postimage-unavailable`.

**None of those exclusions raises `operation-missing`.** Each line above is a
case in which v1 deliberately emits **no** operation, so no operation was owed
and the effect carries exactly the capability, safety or availability codes its
own line names (ADR-036 D3). `operation-missing` is the complement: it is
raised **if and only if** the effect is otherwise representable — a regular
`100644` effect on both sides, inside the supported domain, with a
repository-safe path, every side its `change_kind` names observed, no
parent-created-target dependency and no non-reclassifiable assigned operation —
**and** the decoded recipe supplies it no assigned operation. Equivalently: no
other effect-local condition holds for the effect and its `operation_indexes`
is empty.

Three consequences are pinned, because rev-6's unconditional trigger ("a
normalized effect has no assigned operation"), read against exact-set
validation, attached the code to every unsupported and ambiguous effect and —
through `mismatch` precedence — contradicted every cohort below:

1. **the positive case** is a representable effect the recipe does not cover:
   `reason_codes: ["operation-missing"]`, `disposition: mismatch`, coverage
   `incomplete`;
2. **the negative cases** are every excluded effect above. An unsupported
   effect keeps its `effect-*-unsupported` codes and stays `unsupported`; an
   ambiguous effect keeps its availability codes and stays `ambiguous`. Neither
   gains `operation-missing`;
3. **an absent, unreadable or undecodable recipe** leaves every effect with
   empty `operation_indexes`, and raises `operation-missing` on the
   otherwise-representable effects **only**. The record-level cause is carried
   by `recipe-undecodable` or by the presence flags, never by promoting every
   effect to `mismatch`.

**An unobserved side also blocks classification, and the axes say so**
(ADR-036 D3). rev-4 required the producer to name an `object_kind` and a
`content_kind` for every effect, which a no-capture producer (§6.15 P6, whose
`capture.mode` is `no-capture`) cannot truthfully do: it has read neither side.
Both axes therefore admit `unknown`, and the selection rules are total:

Both rules are scoped to the effect's **extant** sides — the sides its
`change_kind` requires to exist. An `add` needs its postimage, a `delete` its
preimage, and a `modify`, `rename` or `copy` both. A side that is not extant
was never going to carry a kind, so failing to observe it cannot remove one:

- **`object_kind`** is the **postimage** side's kind when the postimage is
  observed and present; the **preimage** side's kind when the postimage is
  **not extant** for this `change_kind` (a `delete`) and the preimage is
  observed and present; and `unknown` in every other case — that is, whenever
  an **extant** side the selection needs was not observed. rev-6 also admitted
  a proven-absent extant postimage here; rev-7 removes that branch, because an
  extant side observed-as-absent is a contradictory publication the validator
  refuses (§6.4). **No kind is inferred across an unobserved
  transition**: a header may declare `100644` → `120000`, but a producer that
  read neither side has verified nothing about the object, and publishing
  `symlink` (or `regular`) on the strength of a header alone is the
  looked-like-it guess the observation flags exist to prevent;
- **`content_kind`** is `none` **if and only if** the effect's *known*
  `object_kind` is `gitlink` — an `unknown` object kind is never `none`;
  `binary` when a strict-grammar stanza marker proves it (`GIT binary patch`,
  `Binary files ... differ`) **or** the observed bytes of an extant side
  contain a NUL, either of which is positive evidence that stands alone;
  `text` **only** when every extant side the effect needs was observed and
  neither binary rule fires; and `unknown` otherwise. `text` is a positive
  claim about bytes, and a producer that did not read them may not make it;
- **an unobserved non-extant side degrades no axis.** A half-observed `add`
  whose postimage was read publishes that postimage's `object_kind` and its
  `text`/`binary` `content_kind`; a half-observed `delete` whose preimage was
  read does the same from the preimage. rev-5 marked both `unknown`, which
  asserted that a kind the producer had actually seen was unestablished. What
  the unobserved side still costs is real and unchanged: the mandatory
  unavailable reason, an `ambiguous` disposition, and failure of predicate 8,
  which demands observation of **every** side the effect names — including the
  proven-absent one;
- an effect carrying `unknown` on either axis **must** carry the
  `preimage-unavailable` / `postimage-unavailable` reason(s) for **at least**
  the extant sides whose absence from the observation set forced it, which
  places it in the `ambiguous` disposition class — and, since rev-7 raises
  `operation-missing` only for otherwise-representable effects, nothing can
  outrank that class here, so an `unknown` axis is always `ambiguous`. The
  converse does not
  hold: an availability reason is mandatory for any unobserved side and can sit
  beside definite axes, so it never implies `unknown`, and a producer may not
  publish `unknown` merely because one is present.
  `unknown` is **forbidden on a complete record** and is refused at decode
  beside `coverage_status: complete`;
- modes stay `""` for an unobserved side, exactly as before. `unknown`
  classifies; `""` records the mode. Neither carries the other's meaning.

The pinned cohorts follow directly. The first three are no-capture effects,
where **both** sides went unobserved; the next two are half-observed effects,
where the one extant side was read; the last is the single positive
`operation-missing` case:

| Effect | Axes | `reason_codes` (sorted) |
|---|---|---|
| Pure rename | `change_kind: rename` from the record header, `object_kind: unknown`, `content_kind: unknown`, both modes `""` | `["effect-rename-unsupported", "postimage-unavailable", "preimage-unavailable"]` |
| Mode-only change | `change_kind: modify`, `object_kind: unknown`, `content_kind: unknown`, both modes `""`; `effect-mode-only-unsupported` is **not** raised, because nothing observed proves the modes differ | `["postimage-unavailable", "preimage-unavailable"]` |
| Type transition (header-declared `100644` → `120000`) | `change_kind: modify`, `object_kind: unknown` — the symlink side was never read — `content_kind: unknown`, both modes `""` | `["postimage-unavailable", "preimage-unavailable"]` |
| Half-observed add (postimage read, preimage never looked at) | `change_kind: add`, `object_kind` **and** `content_kind` from the observed postimage — `text`, or `binary` under a marker or a NUL; the preimage is not extant for an `add` | `["preimage-unavailable"]` |
| Half-observed delete (preimage read, postimage never looked at) | `change_kind: delete`, `object_kind` **and** `content_kind` from the observed preimage on the same terms; the postimage is not extant for a `delete` | `["effect-delete-unsupported", "postimage-unavailable"]` |
| **Representable modify the recipe does not cover** (both sides observed and present, regular `100644`, safe path, `operation_indexes: []`) | `change_kind: modify`, `content_kind: text`, `object_kind: regular`, both modes `100644` | `["operation-missing"]` — the **only** shape that raises it, `disposition: mismatch` |

The last row is the positive `operation-missing` case; the five above it are
the negatives and keep exactly the codes listed, with **no** `operation-missing`
added — including when the recipe is absent, unreadable or undecodable, which
empties every effect's `operation_indexes` without making an operation owed for
an effect v1 never intended to represent.

Because the axes are orthogonal, an effect disqualified on more than one axis
carries **every** applicable code in its sorted `reason_codes` array. A binary
rename carries `effect-binary-unsupported` and `effect-rename-unsupported`; an
executable rename carries `effect-executable-unsupported` and
`effect-rename-unsupported`; a symlink delete carries
`effect-delete-unsupported` and `effect-symlink-unsupported`. rev-1's single
`reason_code` string forced a choice between them, which is how a rename could
have been reported as merely "binary".

**Executable files are incomplete, adds included.** `executeOperation` writes
every `write-file` target with a fixed `0o644` mode
(`internal/workflow/recipe.go:207-211`), so a generated write can never
reproduce `100755`. An executable add would produce byte-correct content with
a silently wrong mode — the exact class of quiet loss this wave removes — so
it joins the incomplete cohort rather than the represented one.

**A preserved recipe containing a non-reclassifiable operation cannot be
coverage-complete in v1.** Completeness requires reclassifying every operation
against the present state (§6.5 predicate 10), and two shipped operations have
no exact present-state reclassification:

| Operation | Shipped behavior | Why it cannot be reclassified |
|---|---|---|
| `append-file` | appends unconditionally (`internal/workflow/recipe.go:230-242`) | nothing distinguishes "already appended" from "must append again" |
| `replace-in-file` | replaces the first substring match (`internal/workflow/recipe.go:213-228`) | first-match has no unique-anchor proof; only a provably exact postimage would qualify, which v1 does not attempt |

Such a recipe is preserved and left byte-identical, and its coverage is
`incomplete` with `operation-not-reclassifiable`. **This changes neither
operation's schema nor its execution semantics**: existing `replace-in-file`
and `append-file` recipes apply exactly as they do today after GH #15.

If a new derivation is incomplete and no recipe exists, the producer does not
publish a partial `apply-recipe.json`. Coverage is still written with
`recipe_present: false`, `recipe_decodable: false`, `recipe_sha256: ""` and all
reasons. If regeneration of an existing recipe would be incomplete, the
existing recipe is preserved byte-identical and is never overwritten by the
partial derivation. Both halves of this policy are unchanged.

**What rev-2 left unsaid is what `apply --mode execute` does next.** The
feature now has valid coverage and no recipe, so apply reaches `LoadRecipe`,
which returns `no recipe found — run 'tpatch implement <slug>' first`
(`internal/workflow/recipe.go:116-120`). That message is wrong here —
`implement` would author an unrelated provider recipe over a feature whose
patch is already recorded — and it is indistinguishable from the honest legacy
case. §6.11 pins a named, state-aware refusal for this state, inside a **total**
classifier over every coverage/recipe shape (ADR-036 D17). Apply does not
silently fall back to reapplying the canonical patch, it does not point the
operator at a branch that is unreachable from the state the refusal fires in,
and it does not accuse a producer of withholding a recipe when the operator may
simply never have generated one: the refusal states that **coverage binds no
readable executable recipe**, which covers an absent recipe and a
present-but-unreadable one alike.

If a richer existing recipe is preserved, coverage judges that exact recipe.
It does not replace it merely because a simpler whole-file recipe could be
generated.

### 6.4 Coverage wire format

`artifacts/recipe-coverage.json` is strict schema version 1, defined
canonically by **ADR-036 D3**. That decision is the single source of truth for
every field, vocabulary and reason code. This section reproduces the canonical
block byte-identically and adds nothing:

```json
{
  "schema_version": 1,
  "feature": "fix-model-id-translation",
  "producer": "record",
  "patch_present": true,
  "recipe_present": true,
  "recipe_decodable": true,
  "patch_sha256": "<64 lowercase hex>",
  "recipe_sha256": "<64 lowercase hex>",
  "reference": {
    "kind": "commit",
    "commit": "<40 lowercase hex>",
    "preimage_set_sha256": "<64 lowercase hex>"
  },
  "capture": {
    "mode": "working-tree-all",
    "pathspecs": [],
    "claim_ids": []
  },
  "coverage_status": "complete",
  "cross_base_status": "consumer-derivation-required",
  "effects": [
    {
      "ordinal": 1,
      "change_kind": "modify",
      "content_kind": "text",
      "object_kind": "regular",
      "path": "command.go",
      "old_path": "",
      "old_mode": "100644",
      "new_mode": "100644",
      "preimage_observed": true,
      "preimage_present": true,
      "preimage_sha256": "<64 lowercase hex>",
      "postimage_observed": true,
      "postimage_present": true,
      "postimage_sha256": "<64 lowercase hex>",
      "patch_fragment_sha256": "<64 lowercase hex>",
      "effect_sha256": "<64 lowercase hex>",
      "operation_indexes": [1],
      "disposition": "represented",
      "reason_codes": [],
      "contextual_hint": "none"
    }
  ],
  "reasons": []
}
```

Implementation notes that follow from the canonical definition, restated here
without extending it:

- every field is required on every record, arrays are non-null, and unknown,
  trailing, null or duplicate members are refused at decode;
- `producer` names the governed producer that published the record (§6.15). It
  is advisory context only; it never substitutes for a recomputed binding, and
  because it lives inside the file it carries nothing when the file is absent;
- `patch_present: false` implies `patch_sha256: ""`, `effects: []`,
  `coverage_status: incomplete`, `cross_base_status: unsupported` and
  `canonical-patch-missing` in `reasons`. `patch_present: true` implies a real
  SHA-256 of the exact raw bytes even when those bytes are empty or the strict
  grammar refuses them; a **semantically empty** patch — zero bytes,
  whitespace-only bytes, or a clean parse yielding zero normalized effects — is
  `incomplete` with `canonical-patch-empty`, and an unparseable one with
  `canonical-patch-unparseable`, both with `effects: []` (§6.3);
- `recipe_present` says the file exists **and was readable** — an absent file
  and a present-but-unreadable one both publish `false`, deliberately and
  identically, while the read-error diagnostic keeps the real cause (§6.11,
  §6.14); `recipe_decodable`
  says those bytes strict-decode. `recipe_present: false` implies
  `recipe_decodable: false`, `recipe_sha256: ""` and empty `operation_indexes`
  on every effect. `recipe_present: true` implies `recipe_sha256` over the
  exact raw bytes, decodable or not; when `recipe_decodable` is `false` the
  record is `incomplete` with `recipe-undecodable` and every
  `operation_indexes` is empty. `recipe_decodable: true` beside
  `recipe_present: false` is refused at decode;
- the three kind axes are orthogonal: `change_kind` says what happened to the
  path, `content_kind` says how the content is expressed, `object_kind` says
  what the object is. `change_kind` comes from the strict grammar's record
  header and is never `unknown`; the other two admit `unknown`, and
  `object_kind` is the postimage side's kind when the postimage is observed and
  present, the preimage side's kind when the postimage is **not extant for the
  `change_kind`** (a `delete`) and the preimage is observed present, and
  `unknown` otherwise — that is, only when an **extant** side
  went unobserved, and never inferred across an unobserved transition;
- `content_kind` is `text`, `binary`, `none` or `unknown`. It is `none` **if
  and only if** the effect's *known* `object_kind` is `gitlink`. It is `binary`
  when the effect's patch stanza carries a Git binary marker
  (`GIT binary patch`, or a `Binary files ... differ` line) **or** an observed
  extant side's bytes contain a NUL. It is `text` **only** when every extant
  side the effect needs was observed and neither binary rule fires, and
  `unknown` in every remaining case — where "extant" is the side the
  `change_kind` requires to exist, so an unobserved **non-extant** side leaves
  the axis definite. Symlink targets are ordinary bytes and are
  therefore normally `text`;
- **`unknown` is bounded, reasoned and never complete**: an effect carrying it
  on either axis must carry the matching `preimage-unavailable` /
  `postimage-unavailable` reason(s) for **at least** the extant sides whose
  absence forced it, lands in the `ambiguous` disposition class — which, with
  rev-7's `operation-missing` scoping, nothing can outrank — and is refused at
  decode
  beside `coverage_status: complete`. The reverse inference is invalid: an
  availability reason is mandatory for **every** unobserved side and coexists
  with definite axes, so its presence never establishes `unknown`. Modes of an
  unobserved side stay `""`;
- `old_path` is non-empty exactly when `change_kind` is `rename` or `copy`,
  independently of `content_kind` and `object_kind`;
- `old_mode`/`new_mode` come from the immutable pre/post observation — the
  reconstructed named tree and the observed postimage — corroborated by, not
  taken from, the patch headers. `""` means that side of the effect is proven
  absent **or** unobserved (the side's `*_observed` flag distinguishes them),
  never "the header did not say". A mode transition is fully
  captured by the pair **when both sides were observed**, and `object_kind` is
  derived from whichever side the rule above names; with an unobserved side
  both modes are `""` and `object_kind` is `unknown`;
- **`preimage_observed`/`postimage_observed` say whether the producer looked;
  `preimage_present`/`postimage_present` say what it found.** A presence flag
  is meaningful only when its observation flag is `true`. `*_observed: false`
  forces `*_present: false`, the corresponding hash `""`, the corresponding
  mode `""`, **and** the matching effect-local reason —
  `preimage-unavailable` or `postimage-unavailable` — in `reason_codes`.
  `*_observed: true` with `*_present: false` is proven absence, and its `""`
  hash and mode carry that positive meaning. `*_observed: true` with
  `*_present: true` requires a real 64-hex hash and a real mode;
- **contradictory observations are invalid input, not a state.** For a side the
  effect's `change_kind` requires to exist — the postimage of an `add`,
  `modify`, `rename` or `copy`, and the preimage of a `modify`, `rename`,
  `copy` or `delete` — the pair `*_observed: true` with `*_present: false`
  claims the producer looked and found the extant side missing, contradicting
  the `change_kind` the same record publishes. A publisher that cannot
  establish such a side marks it **unobserved** (`*_observed: false`,
  `*_present: false`, `""` hash, `""` mode) with its mandatory availability
  reason, which is the truthful record of "nobody established this". The strict
  validator **refuses** every impossible observed-absence shape at decode, in
  the publisher and in the consumer alike. This is what keeps the axis
  selection rules total without inventing a reason code: an extant side is
  either observed-and-present or unobserved-with-its-reason, and there is no
  third case to classify. Proven absence remains valid — and required by
  predicate 8 — for a **non-extant** side: an `add`'s preimage and a `delete`'s
  postimage;
- `patch_fragment_sha256` is the SHA-256 of this effect's **exact raw byte
  range** in the canonical `post-apply.patch`: from the first byte of its
  `diff --git` line through the byte immediately before the **next effect's**
  `diff --git` line, or to end of file for the last effect. The boundaries are
  the **strict grammar's own recognized line-start record starts**, not a
  substring scan: a `diff --git` sequence inside a hunk body, a quoted path, a
  binary stanza or mid-line does not open a fragment. That case is closed
  rather than merely warned about — inside a **valid** hunk every body line
  carries a `+`, `-` or space prefix, so an embedded token is never at line
  start; a **bare** line-start token outside a valid hunk is parsed as a new
  record by the grammar, or the patch is refused as
  `canonical-patch-unparseable`. Grammar recognition wins in both shapes, and
  PI-12's b-side semantics are unaffected (§6.1.2). Original line endings and
  `\ No newline at end of file` markers are retained; nothing is normalized,
  trimmed or re-encoded. A consumer recomputes it by running the same grammar
  for the record offsets and hashing that range;
- `effect_sha256` is a deterministic digest over the complete normalized
  effect descriptor: ordinal, all three kind axes, path, old path, both modes,
  **both observation flags**, both presence flags, both content hashes, and
  `patch_fragment_sha256`. The axes are hashed exactly as published, `unknown`
  included, so a record that later asserts a definite kind for a side nobody
  observed does not recompute. That list is exhaustive — no unspecified or
  hidden input contributes, and the rev-2 phrase "normalized hunk digest" is
  gone from the schema. The observation flags are inputs because without them
  an unobserved side and a proven-absent side would hash identically;
- `reason_codes` is sorted ascending, duplicate-free and non-null, is `[]`
  exactly when `disposition` is `represented`, and carries **effect-local**
  codes only. `disposition` remains a single closed value and is a **function**
  of `reason_codes`: `represented` iff the array is empty, `mismatch` iff the
  array is **exactly** `["operation-missing"]`, `ambiguous` when
  `operation-missing` is absent
  and it contains `preimage-unavailable` or `postimage-unavailable`, and
  `unsupported` for any other non-empty array. `operation-missing` never
  co-occurs with another effect-local code — its raising condition requires the
  absence of every other one — so a record pairing them is refused at decode
  rather than resolved by precedence. Multiple codes still resolve by the
  precedence `mismatch` → `ambiguous` → `unsupported`, whose live cases are now
  the lower two; precedence chooses the single `disposition`, it never drops a
  reason. A contradictory pairing is refused at decode in both directions;
- `reasons` is sorted ascending, duplicate-free, non-null and required, and
  carries **record-level** codes only. ADR-036 D3's allocation table assigns
  each code to exactly one array, and no occurrence is written to both;
- **both arrays are exhaustive, not illustrative**: `reasons` equals exactly
  the set of record-level closed codes whose raising condition holds, and each
  effect's `reason_codes` equals exactly the set of effect-local closed codes
  whose raising condition holds for that effect. No applicable code is
  optional, none may be dropped because another already explains the same
  incompleteness, and none may appear whose condition does not hold. Validation
  and the acceptance matrix check **set equality**, not membership.
  **Exactness ranges over each code's own raising condition as ADR-036 D3
  writes it**, not over an intuition of what the code might describe — which is
  what makes `operation-missing`'s scoping (§6.3) binding rather than
  cosmetic: an effect already excluded by a capability, safety or availability
  condition does not satisfy that code's condition, so set equality neither
  permits nor requires it there;
- `ordinal` is the **one-based position of the effect's record in the strict
  grammar's parse of the canonical patch**, counted from the first recognized
  record start with no gaps. It is a property of the patch, not of the recipe
  or of the JSON array's order, and `patch_fragment_sha256`'s boundaries follow
  that same order — fragment *n* spans record *n*, ending immediately before
  record *n+1* or at end of file for the last ordinal — so one parse recomputes
  both;
- `coverage_status` is defined by ADR-036 D3's canonical ten-predicate iff,
  reproduced in §6.5, and a contradictory record — `complete` with a non-empty
  `reasons`, a non-`represented` effect, an **empty `effects` array**, an
  `unknown` axis, an absent patch, an absent or undecodable recipe, an
  unobserved required side or a non-`commit` reference —
  does not decode in either direction;
- `contextual_hint` is the only carrier of contextual advice, is closed to
  `none` and `additive-text`, and is explicitly non-authoritative: it never
  affects `coverage_status`, `cross_base_status`, `disposition`, apply
  behavior or eligibility. There is no open question about its placement;
- `cross_base_status` is producer scope, not eligibility (§6.9);
- the reason-code list is closed. The implementation may not add, rename or
  split a code, or move one between arrays, without an ADR-036 revision.

A parity guard asserts this block stays byte-identical to ADR-036 D3's block,
so the two documents cannot drift.

### 6.5 Completeness and simulation

**ADR-036 D3 owns the completeness predicate list, and this section reproduces
it byte-identically.** rev-3 carried two prose renderings of one predicate set —
an eight-item list in each document, worded differently and scoped differently —
which is exactly how a reviewer found the two documents disagreeing about what
`complete` means. There is now one canonical numbered list of **ten**
predicates, it lives in ADR-036 D3, and the block below is copied from it
verbatim, indentation included, so a parity guard can compare the two byte for
byte. Every predicate cross-reference in either document uses these numbers.

Coverage is `complete` **if and only if** all of the following hold:

  1. `patch_present` is `true`, the strict effect grammar parses the canonical
     patch completely, and that parse yields **at least one** normalized
     effect;
  2. `reference.kind` is `commit`, so the reference is durable;
  3. `recipe_present` and `recipe_decodable` are both `true` and the decoded
     recipe's owning feature equals the coverage owner;
  4. every normalized effect of the canonical patch appears in `effects`
     exactly once, and `effects` contains no entry the patch does not produce;
  5. every recipe operation is assigned to at least one effect and no
     operation is surplus;
  6. every effect path and every operation path is repository-safe, and no two
     operations claim the same path;
  7. every effect carries `disposition: represented` with `reason_codes: []`,
     and the record-level `reasons` array is `[]`;
  8. every side each effect requires is observed — `preimage_observed` and
     `postimage_observed` are `true` wherever the effect's `change_kind` needs
     that side — and every observed present side carries an exact mode and an
     exact content hash;
  9. simulating the complete operation set against the immutable preimage
     observation reproduces the exact postimage byte, existence and mode set
     within the supported mode domain of D5, with no unmodeled path change and
     no unmodeled mode change;
  10. reclassifying that simulated result reports every operation
      already-present and writes no byte.

Coverage is `incomplete` **if and only if** any predicate fails — equivalently,
whenever `reasons` is non-empty, or any effect is non-`represented` or carries
a non-empty `reason_codes`.

The ten predicates span four dimensions, and rev-2 asserted only the third:

- **artifact reality** — predicates 1 and 3: a present patch that strictly
  parses into at least one effect, and a present, decodable, correctly owned
  recipe;
- **reference durability** — predicate 2;
- **explanation totality** — predicates 4-7: every effect present exactly once,
  every operation assigned, all paths safe and unique, and no reason of either
  class outstanding;
- **observed, reproducible behavior** — predicates 8-10: every required side
  actually observed, exact simulation within the `100644` domain, and a stable
  already-present reclassification.

**A contradictory record does not decode.** The strict decoder refuses
`complete` alongside a non-empty `reasons`, a non-`represented` effect, a
non-empty `reason_codes`, an **empty `effects` array**, an effect carrying
`object_kind: unknown` or `content_kind: unknown`, `patch_present: false`,
`recipe_present: false`,
`recipe_decodable: false`, an effect whose required side is unobserved, or a
non-`commit` reference; and it refuses `incomplete` when all of those are
absent. There is no encoding of "complete, but…", and — by predicate 1 — none
of "complete over nothing".

**Predicate numbers are load-bearing cross-references.** Both documents cite
predicates by number, so a renumbering that is not propagated silently
retargets a claim: rev-4 cited "§6.5 predicate 8" for operation
reclassification, which had been predicate 10 since the list grew to ten, so
the citation pointed at the observation predicate instead. The parity guard
(RGA row in §9.5) therefore checks two things, not one: that the block above is
byte-identical to ADR-036 D3's, **and** that every `predicate N` reference
anywhere in either document resolves to the predicate whose text the
surrounding sentence describes.

Predicate 8 is why the observation flags exist. rev-3 could satisfy
"every effect is represented" over an effect whose preimage the producer never
read, because `preimage_present: false` was indistinguishable from proven
absence. Completeness now requires the producer to have **looked** at every
side the effect needs, not merely to have recorded a `false`.

Predicate 9 is scoped deliberately. Exact-mode simulation is only claimed for
`100644` effects because that is the only mode `write-file` can produce
(`internal/workflow/recipe.go:207-211`). Any effect whose `object_kind` is
`executable`, `symlink` or `gitlink` — equivalently, any effect with a
`100755`, `120000` or `160000` side — never reaches simulation: it is already
`unsupported` under §6.3 with its own reason codes, so predicate 7 has failed
and coverage is `incomplete`. The simulator therefore never has to decide what
"exact mode" means outside the domain it can reproduce.

Predicate 10 is reclassification, not a second execution. A preimage-bearing
write is expected to see its postimage after the first simulation. An
operation with no exact present-state reclassification — `append-file`, or
`replace-in-file` outside a provably exact postimage — fails this predicate
and yields `operation-not-reclassifiable` (§6.3).

### 6.6 Record-generated provenance

**One convergence rule governs every rerun** (ADR-036 D6). Record writes or
repairs `recipe-provenance.json` whenever all of the following hold:

1. the recipe is **record-generated under §6.16** — this run's freshly derived
   canonical recipe bytes equal the on-disk recipe bytes exactly — and it
   carries at least one non-nil `preimage_hash`;
2. the capture has a durable truthful reference (`reference.kind: commit`);
3. matching provenance is missing, undecodable, or stale, meaning its
   `base_commit` or `recipe_sha256` differs from what the producer would
   truthfully write now.

Condition 1 is **proved on this run by recomputation**, never remembered from
a previous run and never read out of a stored label (§6.16).

The written sidecar carries the actual reference commit, the actual generation
time, and the exact recipe hash.

This rule applies identically on all three autogen outcomes for a
record-generated recipe:

| Rerun outcome | Matching provenance exists | Action |
|---|---|---|
| generated | n/a (new recipe) | write truthful provenance |
| regenerated | no or stale | write truthful provenance |
| regenerated | yes | preserve bytes and timestamp |
| **noop / preserved, byte equality proved** | **no or stale** | **write or repair truthful provenance** |
| noop / preserved, byte equality proved | yes | preserve bytes and timestamp |
| noop / preserved, byte equality **not** proved | any | preserve the recipe; fabricate nothing |

The no-op row is the one that makes a rerun a real recovery operation. If a
run crashes or fails after writing the recipe but before writing provenance,
the next `tpatch record` finds an unchanged recipe, re-derives the identical
bytes, proves origin, and repairs provenance — otherwise the feature would be
permanently stuck with a preimage-bearing recipe that V10 blocks on and no
command that fixes it.

Existing bytes and timestamp are preserved **only** when matching provenance
already exists. Record never manufactures a new timestamp for an unchanged,
already-truthful sidecar.

**Preserved manual and provider recipes are excluded.** Record does not
fabricate provenance for a recipe whose bytes its own derivation does not
exactly reproduce: it cannot truthfully state the base or time that recipe was
authored against, and inventing one would be exactly the historical-adoption
move GH #19 exists to govern. Such a recipe keeps whatever provenance it has
(or none) and remains subject to V10. **GH #19 remains the owner of
historical/manual adoption**; GH #15 does not consume, pre-empt or partially
implement it.

If the capture has no durable commit reference, record writes no commit-shaped
provenance for any recipe class, and coverage is incomplete with
`reference-not-durable`.

This makes fresh `record -> verify` green for generated recipes without
authorizing historical adoption.

### 6.7 Preimage-bearing apply behavior

Before any operation writes, every `write-file` with a non-nil preimage is
classified. **Classification runs first, on every feature, regardless of
supersession.**

| Current target | Expected preimage | Classification |
|---|---|---|
| byte-exact postimage | any valid authority | `already-present`, no write |
| byte-exact expected preimage | matching `sha256:` hash | `applicable` |
| absent | explicit empty | `applicable` creation |
| present, byte-exact postimage | explicit empty | `already-present`, no write |
| any other state | any | `drift` |

**Severity of `drift` follows supersession and is unchanged from ADR-029 D7.**

| Feature status | `drift` severity | Effect on this run | Default effective replay |
|---|---|---|---|
| effective / non-superseded | refusal | no operation from the recipe is written; the whole recipe refuses before any mutation (ADR-029 D3) | recipe does not execute |
| superseded by an active superseder (healthy or stale) | warning with the existing `superseded by "<slug>"` note | execution proceeds, exactly as today (`internal/workflow/writefile_safety.go:264-305,326-364`) | already excludes the superseded feature (`docs/adrs/ADR-028-supersession-edge-model.md:77-88`) |

The superseded warning is an audit signal about expected historical drift. It
does **not** state that explicitly applying that superseded recipe is safe, and
it does **not** make that recipe's coverage or replay safe. The active
superseder carries current semantics; an operator who explicitly applies a
drifted superseded recipe is overriding an audit signal, and the diagnostic
says so. Coverage severity and replay eligibility are answered by §6.5, §6.12
and GH #13, never by the presence of a downgraded warning.

Path-safety refusals are never downgraded, in any state
(`internal/workflow/writefile_safety.go:270-278`).

**Empty preimage versus exact postimage — an amendment to ADR-029 D3 only.**
ADR-029 D3 lists four refusal cases. Exact-postimage recognition narrows
**two** of them, and only where the observed bytes are byte-exactly the
operation's generated postimage:

| ADR-029 D3 refusal case | ADR-036 effect |
|---|---|
| empty preimage, file exists | **narrowed**: non-nil empty preimage with the target present and byte-exactly the postimage → `already-present`, no write; target absent → `applicable` creation (unchanged); target present with **any other** value → collision, `drift`, refused under the severity ladder above |
| expected hash present, file hash differs | **narrowed**: a non-empty expected-hash operation whose target's observed bytes are byte-exactly that operation's postimage → `already-present`, no write; any other differing value → `drift`, refused under the same ladder |
| expected hash present, file missing | **unchanged**: still a refusal — a missing target is not an applied postimage |
| unreadable target needed for a precondition | **unchanged**: still a refusal, with the read error named |

rev-6 said "the other three ADR-029 D3 refusal cases … are unchanged", which
contradicted this section's own classification table: the first row of that
table recognizes an exact postimage for **any** valid preimage authority, so
the expected-hash-mismatch case was already narrowed. The claim is removed and
the rows are aligned.

**What is not amended**: ADR-029 D3's all-or-nothing precheck atomicity — every
precondition is still evaluated before the first write and one refusal still
means no operation is written — path-safety refusals, and **ADR-029 D7's
supersession severity**. Apply and verify mirror the two narrowed cases:
verify's empty-preimage and expected-hash branches
(`internal/workflow/verify_anchored.go:964-981`) gain the same exact-postimage
recognition at the reference tree and keep their failure for every other
observed value.

A mixed recipe may contain `already-present` and `applicable` operations, but
every precondition is checked before the first write. Legacy omitted preimages
retain the existing ADR-029 D4 warning path.

Result accounting for `already-present` operations is specified in §6.11.

### 6.8 Dependencies and `created_by`

An explicit empty preimage always means the target must be absent or already
byte-exactly the postimage (§6.7). The apply path does not exempt a collision
merely because `created_by` names a hard parent.

**v1 drops parent-postimage reuse entirely.** There is no field in the recipe
or coverage schema that can bind a parent's slug, generation and postimage
hash, so an operation carrying a parent-derived preimage would be unverifiable
at read time — the consumer could not recompute what it was trusting. Adding
such a carrier is a schema change, and GH #15 does not make one.

Therefore a generated or preserved operation whose **correct preimage depends
on a path a hard parent creates** is coverage-incomplete with
`parent-created-target-unsupported`. There is no empty-preimage exemption and
no partial credit: the operation is not `represented`, so the feature's
coverage is `incomplete`, and GH #13 cannot authorize it.

Independent shared-path stacks are unaffected. When each feature's own
captured preimage is durable and correct, applying the stack in dependency
order satisfies each preimage in turn; wrong order or later drift is `drift`
under §6.7.

### 6.9 No persisted contextual anchors in v1, and an explicit acceptance amendment

The producer does not emit a new replace/append operation, operation type or
authority field. It may set `contextual_hint: additive-text` on an additive
textual effect, which is advisory and does not change `coverage_status` or
`cross_base_status`.

GH #13 derives candidate anchors ephemerally from the canonical patch and a
named reference tree. **GH #13 must prove uniqueness, postcondition identity
and idempotent reclassification for every anchor it derives**, independently of
anything GH #15 wrote. GH #15 hands it no anchor and no anchor-shaped claim.

**This rev-1 amends GH #15's planning acceptance: v0.17.0 persists no anchor
of any kind.** That is a deliberate scope reduction recorded here so a later
consumer does not go looking for an anchor field and conclude one was
forgotten.

`cross_base_status` carries the resulting producer-scope statement. It is
derived deterministically and is **not eligibility**:

| Value | When the producer emits it | Meaning |
|---|---|---|
| `unsupported` | `coverage_status: incomplete` | generation is not complete, so there is no cross-base statement to make. This covers every absent-patch and empty-patch record |
| `consumer-derivation-required` | complete, and at least one represented effect is a whole-file write over a pre-existing file | the persisted operation is **not** cross-base safe; a consumer wanting another base must derive and prove its own evidence |
| `reference-tree-only` | complete, and every represented effect is a creation carrying the explicit-empty-preimage gate | the recipe's own preconditions already constrain it; the producer needs nothing further from a consumer. A complete record has at least one effect, so this value is never reached vacuously by an empty effect list |

For the motivating adjacent CLI argument fixture
(`docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md:1-243`)
the required outcome is exact:

- `coverage_status: complete` — the whole-file write does reproduce the
  captured postimage from the captured preimage, so **base-bound generation is
  complete**;
- `cross_base_status: consumer-derivation-required` — cross-base authority is
  explicitly withheld/refused, because replaying that whole-file write against
  an upstream tree that deleted the neighboring arguments would restore them;
- `contextual_hint: additive-text` on the additive effect — advisory only.

No output, document, skill or diagnostic may present that whole-file write as
cross-base safe. "Complete coverage" and "safe to replay elsewhere" are
different statements, and this fixture is the canonical proof that they
differ.

### 6.10 Publication and recovery

All outputs are computed before the first write. Publication order is:

1. generated/regenerated recipe;
2. truthful record-generated provenance where §6.6 requires it;
3. patch generation metadata;
4. recipe coverage, last.

Each write is atomic as one file; no cross-file atomicity is claimed.
Consumers recompute all bindings, so every partial state is unusable rather
than falsely current.

**Coverage publication is last and unconditional on every governed producer
event** (§6.15), not only on `record`'s. Record writes/rebuilds coverage on
every autogen action:

- generated;
- regenerated;
- preserved existing manual/provider recipe;
- preserved generated recipe;
- recipe noop;
- unchanged patch generation;
- no recipe because effects are incomplete.

`feature patch refresh|fixup`, `reconcile --accept`, `cycle` and
`apply --mode done` publish on every event that writes or checkpoints the
canonical patch, whether or not they touch the recipe at all — including P2's
non-writing category-(c) same-patch checkpoint (§6.15). `implement` publishes
after its recipe and provenance writes, at the **common coverage-last point
both arms of its recipe parse fall through to** — after the state-mark attempt
and before `RunImplement` propagates its final
`return s.MarkFeatureState(...)` (`internal/workflow/implement.go:243`); a
failed recipe write on either arm is not an event and
publishes nothing.
`tpatch edit` publishes after the editor returns having changed a bound
artifact, whether or not the editor itself succeeded — and publishes nothing
when `$EDITOR` was unset, since no process ran.

Rerunning the same command repairs missing/stale coverage even when no new
patch generation is appended and no recipe byte changes, and repairs
provenance in the same run under §6.6.

**A coverage-publication failure is surfaced on every P1-P7 event, and never
degrades to success** (ADR-036 D10). One contract covers the whole registry:

> When a governed producer event owes coverage and the shared publication API
> fails, the producing command **returns that failure to its caller and exits
> non-zero**. It does not log-and-continue, does not fall through to its
> ordinary success line, and does not treat "the real work already landed" as
> a reason to report success.

Because publication is **last**, such a failure can leave coverage **absent or
stale** on disk. That is the same recoverable state a crash leaves: §6.14's
recomputation and §6.12's rungs already reject it, and a rerun repairs it. What
it may never leave is a **zero exit** over an unrecorded write — a run that
wrote a patch or a recipe and could not record what it wrote has produced
exactly the silently-stale state §6.15 exists to remove. When another failure
is already in flight — a failed state mark, a returned editor error — both are
reported under the fixed precedence in §6.15, and the coverage failure is
neither dropped in favour of the primary error nor allowed to displace it. The
acceptance matrix pins this with **one table-driven row over P1-P7**, one case
per producer event, rather than seven per-producer rows (§9.1).

### 6.11 Producer and apply output

Every governed producer (§6.15) reports one closed coverage status:

- `recipe coverage: complete`;
- `recipe coverage: incomplete (<sorted reason codes>)`.

Incomplete coverage is warning-class for the producer and preserves the
canonical patch. An explicit `--regenerate-recipe` request
(`internal/cli/cobra.go:1898,2008`) never overwrites an existing recipe with an
incomplete derivation; it returns a named warning and preserves the old recipe.

A producer that rewrote the canonical patch and left an on-disk recipe that
neither covers nor simulates it (P3, P4, P5, and P2 when §6.16 does not prove
origin) publishes the **paired** `producer-patch-rewrite` +
`recipe-not-regenerated` — neither code is emitted alone for that event — and
additionally prints the remediation:

```text
recipe coverage: incomplete (<sorted reason codes, always including both
  producer-patch-rewrite and recipe-not-regenerated>)
  the recipe on disk no longer covers the patch just written. To regenerate:
    tpatch record <slug> --regenerate-recipe
```

The printed list is the record's **exact** sorted set, not an illustrative
pair: any other applicable code — `recipe-stale-marker-present` when the marker
was written, and every effect-local code the record carries — appears in it
too (§6.4).

**The regeneration line is printed only after a dry derivation proves it can
succeed** (ADR-036 D11). Before naming any regeneration command, the surface
re-derives effects and the candidate recipe in memory, writing nothing, and
prints the command only if that dry derivation would produce
`coverage_status: complete`. Otherwise it says plainly that no automatic
regeneration can truthfully repair the feature, names the blocking reasons, and
directs the operator to manual review. A command that would fail — or, worse,
"succeed" into another incomplete record — is never offered.

**`Recipe generated` prints the recipe's real operation count.** The shipped
line at `internal/cli/cobra.go:1908` derives its `(%d ops)` from
`countPatchFiles`, a `diff --git` prefix counter (§6.1.3). It now prints the
operation count of the recipe just derived, from **inside its own
`case workflow.AutogenGenerated` arm** (`internal/cli/cobra.go:1906-1908`) —
the only branch in which a recipe was generated, and therefore the only branch
that can hold a real operation count. When no recipe was published the run does
not enter that arm at all, so the incomplete coverage status is printed from
the **common post-autogen coverage output path** every outcome reaches, not
from `cobra.go:1908`.

**`apply --mode execute` classifies every coverage/recipe shape, and refuses by
name when coverage binds no readable executable recipe** (ADR-036 D17).

**Where the refusal sits.** `apply --mode execute` computes `reapplying` from
the feature's state — true when the state is `unapplied` or an unapplied
baseline is pending — and when it is true it reapplies the canonical patch and
**returns before `LoadRecipe` is called** (`internal/cli/cobra.go:911-946`).
`LoadRecipe` runs only on the `reapplying == false` fall-through
(`internal/cli/cobra.go:948-950`). **The refusal is therefore reached only when
`reapplying` is false.** rev-3's diagnostic told the operator to "reapply it
verbatim through the canonical-patch reapply path" and asserted "the operator
chooses it"; both are deleted, because that branch is state-selected rather
than operator-selected and is by construction not selected in the only state
where this refusal can fire. `--mode` still accepts only `auto`, `prepare`,
`started`, `execute` and `done` (`internal/cli/cobra.go:848`) and GH #15 adds
no `reapply` value — but that is now stated as a fact, not offered as an
action. The refusal also never recommends `tpatch feature unapply`: it is a
destructive reverse-apply with preconditions the diagnostic has not proved.

**Coverage-binding refusal comes first, and the classifier is total.** Seven
states are separated, in this order; every reachable coverage/recipe shape
matches exactly one row, and the first match wins:

| Order | State on disk | Behavior |
|---|---|---|
| 1 | coverage present but malformed, **or** any binding mismatch (§6.14) — in particular a presence flag whose **recomputed readable existence differs from the stored value** in either direction: a claimed-present artifact that is absent or unreadable, or a claimed-absent artifact that is present and readable — or any hash, reference, capture or **coverage-envelope-owner** mismatch | named coverage refusal with the §6.12 rung-1/rung-2 code, exit `2`, raised **before the recipe is loaded**. The claimed-present-but-not-readable recipe case is `recipe-coverage-recipe-changed`: the record binds a recipe hash that no longer recomputes; the mirror direction uses the same code and is how a later readability change is caught. A record owned by another feature is `recipe-coverage-owner-mismatch`. None is the legacy no-recipe state nor order 2 |
| 2 | coverage present, valid, `incomplete`, `recipe_present: false`, and **no readable `artifacts/apply-recipe.json`** — the file is absent, **or** present and unreadable | the named `recipe-generation-incomplete` refusal below, exit `2`. Its wording is **"coverage binds no readable executable recipe"**, not "generation withheld one" — the same state is reached by a feature nobody ever generated a recipe for. The record and the recomputed readable existence agree here, which is exactly why this is not order 1 |
| 3 | coverage present, valid, `incomplete`, `recipe_present: true`, `recipe_decodable: false` — the bytes were read and do not decode | the **same** named `recipe-generation-incomplete` refusal and exit `2`, listing `recipe-undecodable` among the sorted reason codes and naming the recipe path. It does **not** fall through to the shipped raw JSON decode error, which reports a syntax offset for bytes the operator may not have written and says nothing about coverage |
| 4 | coverage present, valid, `incomplete`, `recipe_present: true`, `recipe_decodable: true` | **executes.** Explicit recipe execution stays backward-compatible: the recipe loads and proceeds through the existing preimage, path-safety and all-or-nothing gates exactly as today, and a **warning** states that coverage is `incomplete`, lists its sorted reason codes, and says that incomplete coverage **is not replay authority** (§6.14). Apply is **not** blocked |
| 5 | coverage **genuinely absent** **and** no **readable** recipe — absent, or present and unreadable (legacy / never implemented) | unchanged: the shipped `LoadRecipe` behavior for either state — `no recipe found — run 'tpatch implement <slug>' first` (`internal/workflow/recipe.go:116-120`) — and exit `1` (`internal/cli/cobra.go:52-59`). Physical unreadability follows this row, not row 6 |
| 6 | coverage **genuinely absent** **and** a **readable** recipe present | unchanged: the shipped legacy recipe execution, with no coverage-derived warning, refusal or gate added |
| 7 | coverage present, valid, `complete` | the recipe is present and decodable by schema (predicate 3 forbids any other combination) and executes unchanged |

**A coverage file that exists but cannot be read is present-and-unusable, not
absent.** It takes order 1 with `recipe-coverage-malformed` and a diagnostic
naming the read error; only a genuinely absent coverage file reaches orders 5
and 6, which exist for pre-v0.17.0 features. The asymmetry with the presence
*fields* — which do collapse absence and unreadability — is deliberate: those
fields answer "is there a usable artifact to bind", while the coverage file's
absence is what selects the legacy path. **Within** orders 5 and 6 the recipe
half uses readable existence exactly as orders 2-4 do: order 6 requires a
recipe the run can actually read, and an absent or unreadable one takes order 5
with the shipped `LoadRecipe` error for that condition and exit `1`. rev-6
split those two rows on physical presence, which left a present-but-unreadable
recipe beside absent coverage nominally on order 6 while the shipped code could
not read it.

**Any other shape is a malformed record, not a state**, and order 1 catches it.
`complete` beside `recipe_present: false`, `recipe_decodable: true` beside
`recipe_present: false`, `complete` beside `effects: []`, and `complete` beside
an `unknown` axis all fail the strict decode (§6.5), so they reach apply as
"present but malformed". The classifier needs no default arm, and adding one
would assert a state the schema cannot produce.

Order 1 outranking order 2 matters: a record saying `recipe_present: true`
beside a file that is no longer readable describes a **deletion, a permission
change or a tamper**, and reporting it as "generation was incomplete" would
attribute an operator's action to the producer. The mirror direction matters
equally: a record saying `recipe_present: false` beside a recipe that is now
present and readable is stale authority, and executing that recipe on the
strength of a record denying its existence would be acting on a binding nobody
validated.

**The line between orders 1 and 2 is whether recomputed readable existence
matches the record.** When it does not, in either direction, order 1 fires.
When it does, and the agreed value is "no readable recipe", order 2 fires — so
a publisher that truthfully recorded `recipe_present: false` for a
present-but-unreadable recipe lands on order 2, and if that file later becomes
readable the record stops matching and order 1 takes over. No shape falls
between them.

Order 4 is the case rev-4 left undefined, and it is the common one. Incomplete
coverage means "the producer cannot prove this recipe covers this patch"; it
does not mean "this recipe is unsafe to run when you explicitly ask for it".
Blocking it would break every legacy feature and every `implement`-authored
recipe on upgrade, in exchange for a guarantee §6.14 already enforces where it
matters. The recipe's own ADR-029 gates still decide safety; coverage decides
replay authority; the warning tells the operator which one they just relied on.

**The refusal output.** Its header states the **binding** fact rather than
attributing a decision, and its remediation is state-selected, so that every
command printed is one the operator can run right now. Order 2 prints:

```text
apply --mode execute refuses: the recipe coverage for "<slug>" binds no
readable executable recipe — no readable apply-recipe.json is present.
  recipe coverage: incomplete (<sorted reason codes>)
  recipe path: .tpatch/features/<slug>/artifacts/apply-recipe.json
  recipe status: <absent | unreadable: <the actual read error>>
  affected paths: <sorted repo-relative paths>
  feature state: <state>
  There is no readable recipe to execute, and tpatch will not synthesize a
  partial one.
```

The `recipe status` line is where the record layer's collapse stops. Coverage
records only `recipe_present: false`, but the refusal separates an absent file
from an unreadable one and prints the underlying cause; an implementation that
reports an unreadable recipe as missing fails the acceptance matrix.

Order 3 prints the same contract with its own first lines, so an operator
learns that the bytes exist and cannot be read rather than that a file is
missing:

```text
apply --mode execute refuses: the recipe coverage for "<slug>" binds no
readable executable recipe — apply-recipe.json was read and does not decode.
  recipe coverage: incomplete (<sorted reason codes, including
    recipe-undecodable>)
  recipe path: .tpatch/features/<slug>/artifacts/apply-recipe.json
  feature state: <state>
  There is no decodable recipe to execute, and tpatch will not guess at
  partially readable operations.
```

Neither header says generation withheld a recipe. rev-4's wording asserted a
producer decision the refusal cannot observe: the identical state is reached by
a feature nobody generated a recipe for, and by one whose recipe was removed
after `implement` wrote it. Coverage is what apply actually read, so the
coverage binding is what the diagnostic states.

Both are followed by exactly one state-selected block.

When the feature is `applied` — the dominant state immediately after `record`,
where the effects are already materialized in the working tree — no reapply is
required at all:

```text
  This feature is already applied: its effects are materialized in the working
  tree, so nothing needs to be reapplied. To confirm that:
    tpatch verify <slug>
    tpatch status <slug>
  To make the recipe executable later, author a complete apply-recipe.json and
  checkpoint it with `tpatch implement <slug> --manual` — note that the
  checkpoint moves this feature from applied to implementing.
```

When the feature is not `applied` and its effects are not materialized, the
canonical patch is the authority and the operator materializes it with
**external Git commands**, after reviewing it:

```text
  The canonical patch is the authority for this feature. Review it, then apply
  it yourself — tpatch will not do this silently on your behalf:
    .tpatch/features/<slug>/artifacts/post-apply.patch
    git apply --check .tpatch/features/<slug>/artifacts/post-apply.patch
    git apply .tpatch/features/<slug>/artifacts/post-apply.patch
  Or author a complete apply-recipe.json and checkpoint it:
    tpatch implement <slug> --manual
  The checkpoint moves this feature to state implementing.
```

Four properties of that output are contract, not phrasing:

1. `git apply --check` is printed before `git apply`, so the operator learns
   whether the patch applies before mutating anything;
2. the commands are **external** `git`, precisely because no reachable `tpatch`
   command materializes a canonical patch from a non-reapplying state. Naming
   an external tool is honest; naming an unreachable internal branch is not;
3. the patch path is printed on its own line and review is explicit. `tpatch`
   does not apply it silently, and the text says so;
4. `implement --manual`'s state transition is disclosed. Checkpointing a recipe
   advances the feature to `implementing`
   (`internal/store/manual.go:26-31,80`), and an operator moving a feature out
   of `applied` is told so in the same breath as the suggestion.

`<sorted reason codes>` is the record's `reasons` array plus the deduplicated
union of its effects' `reason_codes`, sorted ascending — the same list verify's
rung 3 prints.

**Exit `2`, and why.** `internal/cli/reject.go:38-47` documents exit `2` as
pre-mutation input validation and exit `3` as a post-validation state-machine
refusal, and its example list — bad reason, empty note, unresolvable evidence,
path-safety violation — predates this PRD and is narrower than the constant's
role. Exit `2` is correct for orders 1, 2 and 3 because each refusal is
**pre-mutation
executable-plan validation**: apply was asked to execute a recipe and the input
required to build an executable plan — a readable, decodable recipe the
coverage record binds — is not available, and readiness is checked
before anything is written. **No state-machine transition occurred and none was
refused** — the feature's state is untouched and no source-state precondition
was violated — so exit `3` would assert a refusal that did not happen. Orders 5
and 6 keep their **existing** exits; this PRD renumbers no exit it did not
introduce. The
implementation extends the comment block at `internal/cli/reject.go:38-47` to
name pre-mutation executable-plan validation, so the constant's documentation
matches its use.

**Nothing is written on a refusal.** Orders 1, 2 and 3 refuse before any
progress marker, snapshot or operation. Order 4's warning is printed before the
recipe runs and changes nothing about what it writes.

Diagnostics include feature, producer, repo-relative path, effect ordinal,
hashes and reason codes. They contain no source bytes.

**Apply result accounting for `already-present` operations.** An
`already-present` classification is a *successful operation that performed no
write*. In `RecipeExecResult` (`internal/workflow/recipe.go:15-25`) it
increments **both** counters:

| Counter | Incremented | Why |
|---|---|---|
| `Applied` | yes | the operation succeeded and its postcondition holds |
| `Skipped` | yes | no byte was written |

`Applied` therefore continues to equal the number of operations that
succeeded, so the shipped summary line
`Recipe executed: %d/%d operations succeeded` (`internal/cli/cobra.go:972`)
still prints `N/N operations succeeded` for a fully already-present recipe. An
operator re-applying an applied feature sees success, not `0/N`. `Skipped` is
the no-write accounting channel that distinguishes a re-apply from a first
apply.

Each already-present operation emits exactly:

```text
[write-file] <path>: already present (exact postimage), no write
```

alongside the existing `[<type>] <path>: OK` messages for operations that did
write.

### 6.12 Verify behavior

Verify adds one check with the exact ID **`recipe_generation_coverage`**,
matching the frozen snake_case vocabulary
(`internal/workflow/verify.go:47-70`), using the shipped severity vocabulary
`block` / `warn` (`internal/workflow/verify.go:73-77`).

The row evaluates exactly one state, chosen by the ADR-036 D13 **precedence
ladder**, highest first. The first matching rung wins; no lower rung raises or
lowers it:

| Rung | Coverage state | Severity | Row outcome | Verdict and exit |
|---|---|---|---|---|
| 1 | present but **unreadable**, malformed or unknown-field | `block` | fails with `recipe-coverage-malformed`, whose diagnostic names the read error when that is the cause | verdict `failed`, exit `2` |
| 2 | present but patch-hash, recipe-hash, reference-, capture- or **coverage-envelope-owner**-stale — including a presence flag whose recomputed readable existence differs from the record in either direction | `block` | fails with the matching binding code | verdict `failed`, exit `2` |
| 3 | present, valid, `incomplete` — including a record whose `reasons` carry `recipe-owner-mismatch` | `warn` | fails, remediation lists every reason code sorted ascending with affected paths | verdict `passed`, exit `0` |
| 4 | present, valid, `complete`, with `recipe-stale.json` beside the recipe | `warn` | fails with `recipe-coverage-stale-marker` | verdict `passed`, exit `0` |
| 5 | absent — genuinely absent, not merely unreadable | `warn` | fails with `recipe-coverage-missing` | verdict `passed`, exit `0` |
| 6 | present, valid, `complete`, no stale marker | `block` | passes | unchanged; contributes nothing |

Exits follow the shipped verdict rule: any non-skipped failing `block` or
`block-abort` check produces `failed` and exit `2`; a failing `warn` check
leaves the verdict `passed` and exit `0`
(`internal/workflow/verify.go:624-637`).

**Two vocabularies, one one-way mapping, no dual names.** rev-2 used
`recipe-stale-marker-present` on the verify rung and
`recipe-coverage-stale-marker` in §7's surface table, reading as two names for
one condition inside one vocabulary. rev-3 fixed the naming but overcorrected
into a claimed **bijection**, which cannot exist: effect-local schema reasons
have no individual surface code, because verify and doctor report one aggregate
incomplete row rather than one row per effect, and one surface code
(`recipe-generation-not-regenerated`) names a *pair* of schema reasons. The
implementable rule is a one-way partial mapping plus disjointness
(ADR-036 D13):

| Surface failure code | Schema reason it names | Layer note |
|---|---|---|
| `recipe-coverage-stale-marker` | `recipe-stale-marker-present` | rung 4 |
| `recipe-coverage-patch-missing` | `canonical-patch-missing` | rung 3 |
| `recipe-coverage-patch-empty` | `canonical-patch-empty` | rung 3 |
| `recipe-coverage-patch-unparseable` | `canonical-patch-unparseable` | rung 3 |
| `recipe-coverage-recipe-undecodable` | `recipe-undecodable` | rung 3 |
| `recipe-coverage-manual-edit` | `manual-bound-artifact-edit` | rung 3 |
| `recipe-generation-not-regenerated` | `producer-patch-rewrite` + `recipe-not-regenerated` | producer output |

**`recipe-coverage-owner-mismatch` is deliberately absent from that table.**
rev-5 mapped it onto the schema reason `recipe-owner-mismatch`, which made one
name span two conditions at two severities — a rung-2 `block` and a rung-3
`warn` — and the two are now stated separately (ADR-036 D9, D13):

| Condition | What differs | Layer | Severity |
|---|---|---|---|
| **coverage envelope owner mismatch** | `coverage.feature` ≠ the requested target slug | binding failure (§6.14), surface code `recipe-coverage-owner-mismatch`; **no** schema reason exists or ever will, because a record cannot encode having been handed to the wrong question | rung 2, `block`, exit `2`; `apply --mode execute` order 1; GH #13 hard refusal |
| **recipe owner mismatch** | the decoded recipe's `feature` ≠ `coverage.feature` | schema reason `recipe-owner-mismatch` in record-level `reasons`; fails predicate 3, so valid coverage is `incomplete` | rung 3, `warn`, exit `0`, reported inside the aggregate `recipe-coverage-incomplete` row's sorted reason list; GH #13 hard refusal |

Three properties are required, and no fourth is claimed. Their subject is the
**seven surface codes in the mapping table above**, not the whole surface
vocabulary:

1. **Totality on the mapped side** — every surface code in that table names
   exactly one schema condition, and no schema condition is named by two
   surface codes. Surface codes **outside** it —
   `recipe-coverage-malformed`, `recipe-coverage-patch-changed`,
   `recipe-coverage-recipe-changed`, `recipe-coverage-reference-stale`,
   `recipe-coverage-owner-mismatch`,
   `recipe-coverage-missing`, `recipe-coverage-incomplete`,
   `recipe-generation-incomplete`, `recipe-generation-provenance-unavailable`,
   `recipe-generation-origin-unproved` and
   `recipe-generation-no-truthful-regeneration` — name verify-, binding- or
   aggregate-level conditions with **no schema-reason counterpart at all**: a
   decode failure, a recomputed-hash mismatch, a misdelivered record, an absent
   file, the aggregate row, a producer-side outcome. They are unmapped by
   construction, not by omission, and minting schema reasons for them would put
   verify-layer conditions inside the artifact's vocabulary. The mapped subset
   therefore has **seven** members and the unmapped set **eleven**;
2. **Aggregation for the rest** — every effect-local `reason_codes` value, plus
   record-level `reference-not-durable`, `operation-surplus`,
   `simulation-mismatch` and `recipe-owner-mismatch`, has **no** individual
   surface code. Each is reported
   inside the aggregate `recipe-coverage-incomplete` row, whose remediation
   prints every reason code sorted ascending. Inventing a per-effect surface
   code is forbidden;
3. **Disjointness** — no token appears in both vocabularies, over the **whole**
   of both, mapped subset and unmapped remainder alike. A verify row may
   never fail with a schema reason token, and a coverage record may never carry
   a surface code in `reasons` or `reason_codes`.

**Rung 3 wins over rung 4, and does not hide the marker.** A feature with valid
`incomplete` coverage *and* a `recipe-stale.json` lands on rung 3 — the higher
rung wins. The marker is not thereby suppressed: the coverage record's
`reasons[]` contains `recipe-stale-marker-present` (§6.4), and rung 3's
remediation prints **every** reason code sorted ascending, so the marker appears
in the rung-3 output. The ladder selects which row fires; it never removes a
reason. A rung-3 remediation that omits a present stale-marker reason fails the
acceptance matrix.

**Missing coverage is uniformly warning-class, whatever its history.** rev-1
split absence into `missing-legacy` (warn) and `missing-produced` (block).
That split is retired in rev-2 because it has no carrier: the only artifact
that could record "this build produced coverage for this feature" is the
coverage file, and in this state it is gone. GH #15 asserts no marker that does
not exist, and uses one code, `recipe-coverage-missing`, for both a
pre-v0.17.0 feature that never had coverage and a feature whose coverage was
deleted after this build wrote it.

The consequence is acknowledged rather than hidden: **deleting coverage does
reduce verify's diagnostic severity, and it can never buy replay authority.**
Deletion moves a feature from rung 6 (pass) to rung 5 (`warn`), so verify says
less than it did — but GH #13 independently hard-refuses missing coverage, so
the same deletion converts a possibly-eligible feature into a definitely
ineligible one. Absence is always ineligible. The acceptance matrix deletes
coverage this build just produced and proves both halves: the `warn`/exit `0`
verify row and the consumer refusal.

**Missing coverage on pre-v0.17.0 / legacy features does not turn existing
repositories red.** This is the migration contract: an operator who upgrades
and runs verify over historical features sees warnings and a `passed` verdict,
never a new failure.

**`recipe-stale.json` presence is warning-class, not a new hard failure.**
rev-1 put the marker on a blocking rung, which would have turned every
repository carrying a pre-v0.17.0 marker red at upgrade — shipped builds have
written that sidecar since `AutogenRecipeForRecord` gained it
(`internal/workflow/recipe_autogen.go:196`). In v0.17.0 the marker means
consumer authority is unusable: a rung-4 feature is refused by GH #13 even
though its coverage decodes and binds. Verify warns; it does not fail. The
precedence is pinned in both directions:

- a marker beside malformed or binding-stale coverage does **not** lower that
  coverage's `block` rung — rungs 1 and 2 outrank rung 4;
- a marker beside valid complete coverage does **not** raise the row to
  `block` — rung 4 is warning-class for the legacy and the current cohort
  alike;
- a marker beside absent coverage lands on rung 5, so a pre-v0.17.0 feature
  carrying a stale marker and no coverage verifies green;
- a marker beside valid **incomplete** coverage lands on rung 3, whose
  remediation still prints the record's `recipe-stale-marker-present` reason,
  so the higher rung reports the marker rather than concealing it.

`recipe_generation_coverage` receives no supersession downgrade of its own: it
reports generation completeness, which supersession does not change.
Apply/verify **preimage** severity continues to follow ADR-029 D7 through the
existing `write_file_preimage_fresh` (V10) row, unchanged.

Coverage success does not make V10 provenance optional; the two are
independent rows. Fresh record-generated preimage recipes pass both because
record writes truthful provenance under §6.6.

**No warning-class rung authorizes replay.** Rungs 3, 4 and 5 all leave the
verdict `passed` and the exit `0`, and all three are independently ineligible
for GH #13 (§6.14). Verify severity answers "does this repository still
verify"; it never answers "may a consumer replay this".

### 6.13 Doctor behavior

The new check is doctor **`D10`**, the next free ID after the shipped `D1`-`D9`
registry (`internal/workflow/doctor.go:233-245`). It is evidence-only,
read-only and warning-class. It reports:

- coverage absent beside patch/recipe;
- coverage present but unreadable, reported with its read error rather than as
  absence;
- strict decode failure;
- patch or recipe hash mismatch;
- reference/capture mismatch;
- incomplete effects with sorted reason codes and affected paths;
- coverage present with no recipe/patch where the schema forbids it.

`D10` acquires no lock and performs no write, backup or normalization. Every
finding sets `Fixable: false`, and `doctor --fix` does not act on it.

**Remediation naming a regeneration command requires a dry-derivation proof**
(ADR-036 D11). rev-2's rule — name a command "only for capture modes that can
truthfully reconstruct the authority" — was a policy with no test. The
mechanical rule is: before printing `tpatch record <slug> --regenerate-recipe`
or any other regeneration command, the surface re-derives effects and the
candidate recipe in memory, writing nothing, and prints the command **only if**
that dry derivation would produce `coverage_status: complete`.

When it would not — an unsupported effect cohort, a non-durable or
unreconstructable reference, an absent or empty canonical patch, a preserved
non-reclassifiable operation — the finding states plainly that **no automatic
regeneration can truthfully repair this feature**, names the blocking reasons,
and directs the operator to manual review. It never claims a non-durable
reference is recoverable from HEAD, and it never offers a command that would
produce another incomplete record.

The same proof gates verify's rung-3 remediation (§6.12) and the producers'
incomplete-coverage output (§6.11).

### 6.14 GH #13 consumer boundary

GH #13 must:

1. strict-decode coverage;
2. recompute patch and recipe hashes — the recipe hash over the **raw bytes on
   disk**, decodable or not — and every `effect_sha256`, and recompute
   `recipe_decodable` by attempting the decode rather than believing the flag;
3. **recompute `patch_present` and `recipe_present` from actual on-disk
   readable existence, in both directions.** A record claiming an artifact is
   present beside an absent or unreadable file is stale; a record claiming an
   artifact is absent beside a file that is present and readable is **equally**
   stale, because it describes a feature state that no longer exists. Whenever
   the file is readable the raw-byte hash is recomputed and compared as well, so
   a `false`-beside-readable record fails on presence and a wrong-hash record
   fails on content. **Readable existence intentionally collapses physical
   absence and unreadability** at the record layer — both are "no usable
   artifact" — while the consumer's own diagnostic keeps the actual cause; a
   readability transition in either direction is therefore binding drift, and a
   consumer may not treat "the file exists but I could not read it" as a
   third, tolerated state;
4. **check the coverage envelope's own owner separately from the recipe's.**
   `coverage.feature` ≠ the requested slug is a binding failure — the record
   describes another feature and nothing in it is authority here. The decoded
   recipe's `feature` ≠ `coverage.feature` is instead the record-level schema
   reason `recipe-owner-mismatch`, which fails predicate 3 and makes the
   coverage `incomplete`. Both are hard refusals for GH #13; they differ in
   verify severity (rung 2 `block` versus rung 3 `warn`) and in whether a
   schema reason exists at all;
5. verify capture binding;
6. reconstruct and verify the named reference where required;
7. reject missing/stale/incomplete/surplus effects;
8. independently classify operation safety;
9. treat `cross_base_status` and `producer` as producer scope, never as
   authorization, and derive and prove its own anchors.

The producer's `complete` token is necessary, never sufficient.

**GH #13 hard-refuses each of these independently of verify's severity**, and
in particular independently of the fact that verify grades three of them
warning-class with exit `0`:

| State | Verify (§6.12) | GH #13 |
|---|---|---|
| coverage missing | `warn`, exit `0` (rung 5) | hard refusal |
| coverage malformed | `block`, exit `2` (rung 1) | hard refusal |
| coverage `incomplete` | `warn`, exit `0` (rung 3) | hard refusal |
| `recipe-stale.json` present | `warn`, exit `0` (rung 4) | hard refusal |
| `canonical-patch-unparseable` | `warn`, exit `0` (rung 3) | hard refusal; a patch the producer could not parse is never "nothing to check" |
| `recipe-undecodable` | `warn`, exit `0` (rung 3) | hard refusal; the bound raw hash proves *which* undecodable bytes are on disk, never that they are usable |
| presence flag contradicted in **either** direction, including by a readability change | `block`, exit `2` (rung 2) | hard refusal; `true`-beside-unreadable and `false`-beside-readable are the same class of stale binding |
| coverage envelope owner mismatch (`coverage.feature` ≠ requested slug) | `block`, exit `2` (rung 2) | hard refusal; the record describes another feature, and no schema reason is or can be emitted for it |
| `recipe-owner-mismatch` (recipe's `feature` ≠ `coverage.feature`) | `warn`, exit `0` (rung 3, inside the aggregate row) | hard refusal; predicate 3 has failed, so the coverage is `incomplete` |
| any `object_kind: unknown` / `content_kind: unknown` effect | `warn`, exit `0` (rung 3) | hard refusal; an unclassified axis means an **extant** side went unobserved, and coverage carrying one is `incomplete` by construction |

A verify warning is a statement about repository health, never a grant. No
warning-class state, and no absence of state, ever authorizes replay.

### 6.15 Governed producers and one shared coverage publication API

Coverage binds the canonical patch and the recipe, so **every CLI-owned path
that writes, or knowingly checkpoints, either one is a governed producer**
(ADR-036 D15). That universal rule is unchanged. rev-2's registry of five was
incomplete: it missed `implement` and `tpatch edit`, and it described `cycle`
as a single patch writer.

| ID | Producer | `producer` value | Writes patch | Writes recipe | Appends generation | Coverage obligation |
|---|---|---|---|---|---|---|
| P1 | `tpatch record` (`internal/cli/cobra.go:1795,1900,1933`) | `record` | yes | yes, via `AutogenRecipeForRecord` | yes | recompute and publish full coverage |
| P2 | `tpatch feature patch refresh\|fixup` (`internal/cli/feature_patch.go:114,135,150`) | `feature-patch-amend` | yes | yes, **non-regenerating** (`autogen=true, regenerate=false`) | yes | supply capture/reference truth; publish `complete` only when §6.16 proves freshly-derived byte equality, otherwise `incomplete`. Also owes a category-(c) checkpoint publication on its non-writing same-patch branch (`internal/cli/feature_patch.go:104-112`) |
| P3 | `reconcile --accept` → `RefreshAfterAccept` (`internal/workflow/refresh.go:82,102`) | `reconcile-accept` | yes, **unconditionally** at `:82` | no, deliberately (`internal/workflow/refresh.go:20-24`) | only when `newPatch != originalPatch` (`internal/workflow/refresh.go:93,102`), `capture.mode: reconcile` | recompute; publish `incomplete` with `producer-patch-rewrite` + `recipe-not-regenerated` when the existing recipe no longer covers the new patch. The event is the **write**, so coverage is owed on every accept — including one whose bytes are unchanged and whose generation append therefore does not run |
| P4 | `tpatch cycle`'s patch-capture step (`internal/cli/phase2.go:166`) | `cycle` | yes | no | no | recompute, or publish `incomplete` — **composite; see below** |
| P5 | `tpatch apply --mode done` → `runApplyDone` (`internal/cli/cobra.go:982,1044`) | `apply-done` | yes, **when it actually writes the canonical patch** | no | no | recompute, or publish `incomplete` |
| P6 | `tpatch implement` — both arms of the `RunImplement` recipe parse (`internal/workflow/implement.go:192-195,209`) **and** a successful `implement --manual` checkpoint of an externally authored recipe (`internal/cli/cobra.go:744`, `internal/store/manual.go:51-80`) | `implement` | no | yes | no | publish coverage after the recipe and its provenance are published, at the common coverage-last point both parse arms reach; when no canonical patch exists, publish explicit incomplete coverage — never silent absence |
| P7 | `tpatch edit` when the **resolved** artifact path is the canonical `artifacts/post-apply.patch` or `artifacts/apply-recipe.json` (`internal/cli/c1.go:33-44,64-95`) | `artifact-edit` | via the operator's editor | via the operator's editor | no | after the editor process returns — successfully or not — detect the bound mutation and either recompute coverage where that is truthful, or publish explicit incomplete coverage with `manual-bound-artifact-edit` |

**P4 is a composite.** `cycle` runs implement at step `[4/6]`
(`internal/cli/phase2.go:112`) — that **is** a P6 event and publishes P6
coverage there — and captures the patch at step `[6/6]`
(`internal/cli/phase2.go:166`). Because `cycle` can return between the two
(`--skip-execute` at `internal/cli/phase2.go:122-126`, declined prompts at
`internal/cli/phase2.go:127-129,152-154`), the earlier publication is what
makes those early exits safe. P4 owes a publication **only if the patch write
at step `[6/6]` actually happens**, and then publishes again, superseding the
P6 record with one bound to the patch.

**P6's policy in detail.** `implement` publishes coverage **after** its recipe
and `recipe-provenance.json` writes
(`internal/workflow/implement.go:192-237`) and after the state-mark attempt,
immediately before `RunImplement` propagates its final return, preserving the
coverage-last order (§6.10). Both arms of its recipe parse are P6 events, and §2.7 corrects two
generations of description: rev-3's "provider and heuristic" (both are selected
earlier and flow into one `json.Unmarshal`), and rev-4's claim that the failure
arm returns a parse error.

- **unmarshal fails** — the raw response is written verbatim
  (`internal/workflow/implement.go:194`). The unmarshal error is consumed by
  the `if` that selected the arm and is **never returned**; the only `return`
  inside the arm is its own `WriteArtifact` error at `:195`. On a **successful**
  write control falls through to the provenance attempt (`:214-237`) and the
  final `return s.MarkFeatureState(...)` at `internal/workflow/implement.go:243`
  — a returned call, not a literal `nil` — so the event to govern is the
  successful write of
  bytes that do not decode. Coverage is published for it at the **common
  coverage-last point both arms fall through to**, after the provenance and
  state-mark attempts and before that return is propagated,
  carrying `recipe_present: true`, `recipe_decodable: false`, the raw-byte
  `recipe_sha256` and `recipe-undecodable` in `reasons` (§6.3, §6.4).
  Publishing nothing would leave a just-replaced, non-decoding recipe beside
  coverage still claiming the previous recipe hash, while the command reports
  success — the silently-stale state this section exists to remove;
- **the `WriteArtifact` error path is not an event.** When the write fails on
  either arm (`:195`, `:210`) nothing reached the artifact, the error
  propagates, and **no coverage is owed**. Publishing there would assert an
  observation of a write that never landed;
- **unmarshal succeeds** — the reserialized recipe is written
  (`internal/workflow/implement.go:209`) and, on success, the same fall-through
  publishes coverage on ordinary terms.

Stated as one rule: **P6's publication hangs off the common post-write path
both arms reach, not off either arm's `return`.** That path ends at
`internal/workflow/implement.go:243`, whose value is the state mark's own
result, so the finalizer is anchored to the return **point** rather than to a
value. The ordering and its error handling are contract:

1. **coverage finalization runs after the state attempt and before the return
   is propagated.** Recipe write → provenance attempt → state-mark attempt →
   coverage finalization → return;
2. **a failed state mark does not cancel it.** Coverage still publishes and
   still binds the recipe write that landed — a recipe on disk beside a feature
   left in its previous state is precisely the shape that most needs a bound
   record — and the state-mark error is still returned;
3. **a failed publication is surfaced, not absorbed.** It is returned and
   `implement` exits non-zero (§6.10). There is **no success-shaped fallback**
   in which the recipe write is reported as a clean run over absent or stale
   coverage;
4. **when both fail, both are reported.** The returned error names the
   state-mark failure first and the coverage-publication failure with it,
   chained under the repository's ordinary error wrapping, so neither cause is
   discarded and the tight error handling elsewhere in `RunImplement` is not
   loosened to accommodate the finalizer.

In the ordinary case no canonical patch exists yet, so the record is
`patch_present: false`, `coverage_status: incomplete`, carrying
`canonical-patch-missing` (§6.3). That is the truthful statement that a recipe
exists and nothing yet proves what patch it explains — and it is repairable: a
later `record` publishes a bound record over the patch it captures. When a
canonical patch *does* already exist (an `implement` rerun on a recorded
feature), P6 recomputes against it. `implement --manual` publishes on the same
terms after a successful checkpoint; it does not author the recipe, it observes
one an agent or human already wrote, and the checkpoint advances the feature to
`implementing` (`internal/store/manual.go:31,80`).

**P7's policy in detail, and its trigger.** `tpatch edit` resolves the artifact
by argument or by state default — `apply-recipe.json` in `implementing`,
`post-apply.patch` in `applied`/`unapplied` (`internal/cli/c1.go:45-60`) — and
then resolves that token to a path through `resolveArtifactPath`, which probes
the **feature root first** and `artifacts/` second (`internal/cli/c1.go:33-44`).
The governed trigger is the **resolved absolute path**, never the token typed:

- only `<feature>/artifacts/post-apply.patch` and
  `<feature>/artifacts/apply-recipe.json` are bound artifacts;
- a same-named file at the feature root — `<feature>/apply-recipe.json` —
  resolves first and shadows the canonical one. Editing that decoy is **not** a
  P7 event, even though the operator typed `apply-recipe.json`. A
  token-matching trigger would publish coverage for a decoy;
- the explicit spelling `tpatch edit <slug> artifacts/apply-recipe.json`
  resolves to the canonical path on the first probe and **is** a P7 event. Both
  spellings and the root-decoy precedence are pinned in the acceptance matrix.

`openInEditor` runs the editor and returns when the process exits
(`internal/cli/phase2.go:257-261`). When `$EDITOR` is unset it starts **no**
process: it prints its pointer line and returns
(`internal/cli/phase2.go:252-255`), so no byte can change and **no P7 event
occurs** — modeled explicitly rather than left to fall out of the byte
comparison. On return from an editor that did run — including a return carrying
the editor error §6.2 requires the refactored helper to propagate — P7 compares
its before-snapshot with the file on disk:

- **no byte changed** — the operator inspected and quit; no mutation, so no
  publication is owed and existing coverage stays valid;
- **changed and truthfully recomputable** — republish coverage as any producer
  would. This requires §6.2's reconstruct-and-validate path to succeed;
- **changed and not truthfully recomputable** — publish explicit incomplete
  coverage with `manual-bound-artifact-edit`, plus `reference-not-durable` when
  no durable reference could be reconstructed.

The comparison and publication happen **before** any editor error is returned
to the caller. A failed editor that nevertheless saved is still a mutation of a
bound artifact, and leaving stale coverage beside it because the exit code was
non-zero is the silently-stale state again.

**Editing an unrelated artifact is not a P7 event.** `tpatch edit <slug>
spec.md`, `request.md`, `analysis.md` or `exploration.md` — or any token whose
resolved path is not one of the two canonical bound paths — touches neither
bound artifact and publishes nothing. The trigger is the identity of the
resolved file, not the invocation of the command and not the spelling of the
argument.

**Direct filesystem edits are ungovernable and are said to be.** An operator
who edits `.tpatch/features/<slug>/artifacts/apply-recipe.json` outside
`tpatch` runs no `tpatch` process, so nothing can publish. That is external
tamper, and GH #15 does not claim to govern it. It does claim that such tamper
is **detected at read time**: every consumer and every verify/doctor rung
recomputes patch and recipe hashes from the bytes on disk (§6.14), so the edit
surfaces as rung-2 binding-stale coverage and GH #13 refuses it. Governed at
write time; detected at read time; never silently accepted.

**One shared publication API.** All seven call a single coverage publication
entry point with a typed input carrying producer, slug, canonical patch
presence and bytes, capture/reference observation, recipe presence and
derivation outcome. No producer encodes coverage bytes itself. Policy
differences between producers are expressed as different *inputs* to that one
API, never as producer-specific writers — which is what makes the obligation
auditable, since the **publication guard** has exactly one call-site shape to
look for. It is a different guard from §6.1's source-inventory guard, which
enumerates `diff --git` readers.

**What counts as a governed producer event.** rev-2's "unconditional on every
governed producer path" read as "once per command invocation", which was wrong
in both directions — it demanded publication from runs that touched no bound
input, and under-specified runs that touch one twice. rev-3 narrowed it to "a
successful bound write or a successful manual checkpoint", which was too narrow
in turn: it left P2's non-writing same-patch branch — the branch an operator
reaches for precisely when coverage needs repairing — outside the definition
while this section simultaneously demanded publication from it. The definition
has **three** categories:

> A **governed producer event** is any of:
>
> **(a)** a successful bound write — the producer wrote
> `artifacts/post-apply.patch` or `artifacts/apply-recipe.json` and the write
> succeeded;
>
> **(b)** a successful manual checkpoint of a bound artifact — the producer
> validated and accepted bytes an operator or agent authored, without writing
> them itself;
>
> **(c)** an **explicitly contracted re-observation checkpoint** — a branch
> named in this section, in which the producer re-observes a bound artifact and
> re-asserts that the recipe covers the patch as of now, **even though the
> bytes are identical and nothing is written**.
>
> Coverage publication is last and unconditional **per event**. A command
> invocation that produces no event of any category owes nothing.

Category (c) is closed. It is not "any run that read a bound artifact"; it
contains exactly the branches enumerated here, and today that is one branch.

**P2's category-(c) branch, exactly.** `feature patch refresh|fixup` captures a
patch (`internal/cli/feature_patch.go:88`), and when that patch is non-empty
but classifies `Append == false` — its bytes hash equal to the **latest**
generation on record, the only generation `ClassifyPatchGenerationKind`
compares (`internal/store/patch_generation_kinds.go:46-49`) — it prints
`no patch byte change; refresh skipped` (or `fixup skipped`)
and returns **without writing anything**
(`internal/cli/feature_patch.go:100-112`). rev-4's "a generation already on
record" was wider than the shipped comparison — matching an *older* generation
does not suppress the append — and rev-3 called this branch "the write is a
checkpoint of the bound patch", which is false: there is no write on that
branch. The obligation is restated in category-(c) terms:

- the branch **computes and publishes coverage before returning**;
- it **keeps its existing user meaning and its existing message** — the patch
  bytes did not change, the refresh/fixup was skipped — and **adds** the
  coverage status line (§6.11's dry-derivation proof gates any remediation
  command it prints);
- it changes disk **only** by repairing or publishing coverage. No patch, no
  numbered patch, no recipe, no provenance, no patch generation and no state
  advance. That is the point: it is the zero-side-effect repair path.

**P2's empty-capture branch is not an event.** When the capture returns zero
bytes, refresh prints the same skipped message and returns at
`internal/cli/feature_patch.go:91-95` (and fixup errors out). That branch is
**not** a checkpoint of the stored canonical patch: the producer observed the
*working tree*, not the artifact, and re-asserts nothing about it. It owes no
publication unless another bound event occurred in the same invocation.

| Situation | Event? | Obligation |
|---|---|---|
| P2 run whose captured patch is non-empty and classifies `Append == false` (`internal/cli/feature_patch.go:104-112`) | **yes — category (c)** | compute and publish coverage before returning, alongside the unchanged `no patch byte change; refresh\|fixup skipped` message; an early return that skips publication is a violation |
| P2 run whose capture is empty (`internal/cli/feature_patch.go:91-95`) | no | nothing owed; this is not a checkpoint of the stored patch |
| P2 run that writes the patch (`internal/cli/feature_patch.go:114`) | yes — category (a) | publish after the write |
| P4 exit after the implement step (`--skip-execute` or a declined prompt) | no P4 event | nothing owed by P4; P6 already published |
| P4 step `[6/6]` that writes the patch | yes — category (a) | publish, superseding the P6 record |
| P4/P5 capture that produces an empty patch, so no patch write occurs (`internal/cli/phase2.go:165`, `internal/cli/cobra.go:1043`) | no patch event | no patch-bound publication owed |
| P5 reapply branch, which reads the canonical patch and writes none (`internal/cli/cobra.go:1006-1022`) | no | nothing owed |
| `apply --mode execute` reapply branch, which reads the canonical patch (`internal/cli/cobra.go:914-919`) | no | nothing owed |
| P6 run on either arm of the recipe parse whose **write succeeded** (`internal/workflow/implement.go:192-195,209`) | yes — category (a) | publish at the common coverage-last point both arms fall through to: after the recipe write, the provenance attempt and the state-mark attempt, and before `RunImplement` propagates its final `return s.MarkFeatureState(...)` (`internal/workflow/implement.go:243`). A failing state mark does not cancel the obligation |
| P6 run whose `WriteArtifact` **failed** on either arm (`internal/workflow/implement.go:195,210`) | no | no successful bound write occurred; the write error propagates and nothing is owed |
| P6 `--manual` run that checkpoints an existing recipe | yes — category (b) | publish |
| P7 edit whose resolved path is bound and whose bytes changed | yes — category (a) | publish, even when the editor returned an error |
| P7 edit that changed no byte | no | nothing owed |
| P7 invocation with `$EDITOR` unset (`internal/cli/phase2.go:252-255`) | no | no process starts, so no byte can change and no event occurs |
| `tpatch edit` whose resolved path is not a bound artifact — including a feature-root decoy | no | nothing owed |

**An ignored bound-write error becomes a propagated one, in two places.**
`cycle` currently discards the error from
`s.WriteArtifact(slug, "post-apply.patch", patch)`
(`internal/cli/phase2.go:166`), so it cannot tell a successful write from a
failed one and cannot make a correct event decision. `openInEditor` likewise
discards `c.Run`'s error (`internal/cli/phase2.go:261`), so P7 cannot tell a
clean editor exit from a crashed one. The implementation propagates both.
Publication follows a **successful** category-(a) or category-(b) event — with
the single, deliberate exception that P7 publishes a changed bound artifact
even when the editor itself failed (§6.2).

**Three permitted terminal states.** Every governed producer event leaves the
feature in exactly one of:

1. **newly recomputed valid coverage** — the producer re-derived effects from
   the artifact it just wrote and published a bound record, `complete` or not;
2. **explicit incomplete coverage with reasons** — it could not recompute a
   complete record and said so in sorted `reasons` (and, where applicable,
   per-effect `reason_codes`), which under §6.4 hold **exactly** the applicable
   closed codes rather than a chosen subset: `producer-patch-rewrite` with
   `recipe-not-regenerated` when a patch rewrite left the recipe uncovering,
   `canonical-patch-missing` / `canonical-patch-empty` /
   `canonical-patch-unparseable` for the patch it could not use,
   `recipe-undecodable` for a recipe it read and could not decode,
   `recipe-owner-mismatch` for a decoded recipe owned by another feature, or
   `manual-bound-artifact-edit` for a P7 mutation — together with every other
   code whose condition also holds;
3. **legacy untouched state** — coverage stays absent, permitted **only** when
   the run genuinely predates v0.17.0 semantics for that feature, meaning it
   produced no governed producer event at all.

**No writer may silently leave formerly complete coverage stale.** A run that
produces a governed event and exits with the previous `complete` coverage still
on disk fails the acceptance matrix, whether or not those bytes happen to still
validate. Because publication is last (§6.10), a crash can leave coverage
absent or hash-stale — both of which consumers reject — but never silently
current. A **failed publication** is that same recoverable disk state and never
the same exit state: §6.10's publication-failure contract applies to every
P1-P7 event, so the command surfaces the failure and exits non-zero instead of
printing its ordinary success line over absent or stale coverage.

**P2's policy in detail.** `feature patch refresh|fixup` runs autogen with
`regenerate=false`, so a drifted recipe is preserved and `recipe-stale.json` is
written instead of the recipe being rewritten. P2 therefore publishes
`complete` coverage only when §6.16's exact byte equality holds against the
patch it just wrote. Otherwise it publishes `incomplete` carrying **exactly**
the applicable closed codes, which on a patch-writing run always include the
**pair**: P2 rewrote the canonical patch (`internal/cli/feature_patch.go:114`)
and left a recipe that no longer covers and simulates it, so
`producer-patch-rewrite`'s condition holds and `recipe-not-regenerated` is
raised **with** it — neither is emitted alone for that event.
`recipe-stale-marker-present` joins them when the marker was written, and so
does every other applicable record- or effect-level code, because the arrays
are exact sets (§6.4). rev-6 wrote this policy as "`recipe-not-regenerated`,
plus `recipe-stale-marker-present` when the marker was written", which read as
though the rewrite code were optional here; it is not. P2 repairs provenance
only where §6.16 proves origin, and never for a preserved manual recipe.

**P2's category-(c) checkpoint raises neither rewrite code.** That branch
writes no patch, so `producer-patch-rewrite`'s condition — a rewrite on this
run — does not hold, and `recipe-not-regenerated`, defined against that
rewrite, does not hold either. If the preserved recipe does not cover the
unchanged patch, the incompleteness is carried by the codes whose conditions do
hold: `operation-missing` on the otherwise-representable effects it fails to
cover (§6.3), and `operation-surplus` or `simulation-mismatch` at the record
level. Exact-set semantics forbid borrowing a rewrite code for a run that
rewrote nothing.

**P2 repairs coverage even when nothing changed.** A refresh whose captured
patch is non-empty and hashes equal to the **latest** generation on record
writes nothing at all (`internal/cli/feature_patch.go:104-112`), yet it is
still a
category-(c) checkpoint: it asserts that this recipe covers this patch as of
now. It recomputes and republishes coverage before returning — leaving the
shipped `no patch byte change; refresh|fixup skipped` message and every other
on-disk artifact untouched — which is what repairs a missing or hash-stale
record without an operator having to invent a change. Without this, the
rerun-repairs-everything property (§6.10) would have a hole exactly where an
operator reaches for it first.

The corollary is stated so it cannot be read as a loophole: when the patch
**did** change, a preserved recipe can still be judged `complete` only if the
full derived bytes genuinely remain the same — §6.16's total comparison, run
against the new patch. That is possible (a patch change that alters no
whole-file postimage the recipe writes), and when it happens the record is
honestly complete. Every other outcome is `incomplete`; "the recipe looks close
enough" has no encoding in this schema.

**P3/P4/P5's policy in detail.** These three write the patch and never touch
the recipe. Each recomputes coverage against the patch it just wrote; if the
existing recipe no longer exactly covers and simulates that patch, the
published record is `incomplete` with `producer-patch-rewrite` and
`recipe-not-regenerated`, never stale `complete`. If the existing recipe does
still cover it exactly, **neither** code is raised (§6.4) and the record may be
complete — `producer-patch-rewrite` is conditional, not automatic. The
remediation these producers print is `tpatch record <slug> --regenerate-recipe`,
subject to §6.11's dry-derivation proof. GH #13 later regenerates recipe and
coverage from accepted operation candidates; until then, an explicit incomplete
record is the honest state, and it is exactly the state that keeps those
features ineligible for replay.

**Guards, counts and mappings.** A producer-inventory guard pins the registry; a
publication guard fails a producer that writes coverage bytes outside the
shared API or returns before publication on an event. rev-3 backed the third
guard with a raw arithmetic threshold — the guard's "expected-writer count is
seven", failing when an **eighth** writer appeared — which is not implementable,
because production does not contain seven bound writer sites. It contains
**eight** direct `WriteArtifact` call sites for the two bound artifacts, spread
unevenly across the registry, plus checkpoints and delegations that write
nothing at all. A threshold of seven would have failed on the shipped tree.

The guard is therefore two guards over two different objects:

1. **Registry count** — the registry has **seven** entries and the `producer`
   enum has **seven** values; these two numbers must agree with each other and
   with the per-producer matrix rows. This is a count, and seven is its value;
2. **Site-to-producer mapping** — a separate, AST-derived mapping enumerates
   every production site that writes or checkpoints a bound artifact and
   requires each **reachable call chain** through it to map to **exactly one**
   of P1-P7. The unit is the call chain, not the source line: one shared line
   reached from two producers is two chains, each mapping to a single distinct
   producer, and that is conformant. It is a mapping, not a count — adding a
   site to an existing producer is legal, and the guard proves no reachable
   chain is unmapped or ambiguous.

The current mapping is:

| Bound site or checkpoint | Maps to |
|---|---|
| `internal/cli/cobra.go:1795` — `post-apply.patch` | P1 |
| `internal/workflow/recipe_autogen.go:204` — `apply-recipe.json`, via `AutogenRecipeForRecord` | **two chains, no static producer**: from `internal/cli/cobra.go:1900` it is P1; from `internal/cli/feature_patch.go:135` it is P2. The helper itself has no `producer` value |
| `internal/cli/feature_patch.go:114` — `post-apply.patch` | P2 |
| `internal/cli/feature_patch.go:104-112` — non-writing same-patch checkpoint | P2, category (c) |
| `internal/workflow/refresh.go:82` — `post-apply.patch` | P3 |
| `internal/cli/phase2.go:166` — `post-apply.patch` | P4 |
| `internal/cli/cobra.go:1044` — `post-apply.patch` | P5 |
| `internal/workflow/implement.go:194` — `apply-recipe.json`, unmarshal-failure arm (raw response) | P6 |
| `internal/workflow/implement.go:209` — `apply-recipe.json`, valid-JSON arm | P6 |
| `internal/store/manual.go:51-80` — `implement --manual` checkpoint | P6, category (b) |
| `tpatch edit`'s `$EDITOR` delegation (`internal/cli/phase2.go:251-262`) | P7 |

Two properties are load-bearing:

- **shared helpers map by caller.** `recipe_autogen.go:204` is one source line
  reached from two producers, so it contributes **two reachable call chains**,
  each mapping to exactly one producer. It is not an eighth producer and has no
  `producer` value of its own; the publication carries the calling producer's
  value. A guard mapping *sites* one-to-one onto producers would either invent
  a producer for the helper or drop one of its callers — which is why the
  guard's unit is the chain;
- **P7 writes nothing itself**, delegating to an external editor, and P6's
  `--manual` arm checkpoints bytes it did not write. Neither appears in a
  `WriteArtifact` grep, which is why the mapping enumerates checkpoints and
  delegations alongside writes.

A new production site that writes or checkpoints a bound artifact and maps to
no registry entry **fails the guard**. The phrase "an eighth writer" appears
nowhere in this contract, and an implementation that hard-codes a writer-count
threshold fails the acceptance matrix. Per-producer runtime rows cover P1-P7.

### 6.16 Proving a recipe is record-generated

§6.6 must decide whether the recipe on disk is one this producer generated,
**without a trust-by-label marker** (ADR-036 D16). A stored `generated by
tpatch` flag, a persisted `AutogenAction`, or any other self-asserted origin
claim is forgeable by hand-editing and unverifiable at read time — the exact
trust shortcut every other binding in this design refuses.

**The rule.** On every rerun the producer recomputes the canonical derived
recipe bytes from the current canonical patch plus the captured reference
observation. If those bytes equal the on-disk `apply-recipe.json` bytes
**exactly**, then this run generated and validated those bytes, and it may
truthfully publish provenance carrying its own actual base commit and
generation time — **even when prior provenance is missing entirely**. The
provenance statement describes this run, not a reconstructed history, so it is
true regardless of what happened before.

Three properties make the equality meaningful, and S2/S3 must deliver all
three:

1. **Pure derivation.** The derivation is a function of canonical patch bytes,
   the immutable capture observation and the slug only. It reads no live
   worktree bytes and embeds no timestamp. Today `RecipeFromPatch` reads
   postimages from the live worktree
   (`internal/workflow/recipe_autogen.go:86-122`); under this PRD it reads them
   from the captured observation.
2. **Canonical encoding.** One deterministic encoder produces recipe bytes, so
   equal semantics implies equal bytes.
3. **Total comparison.** Full canonical bytes, never a projection.

**File-set equality is retired for this decision.** `compareRecipeFileSets`
(`internal/workflow/recipe_autogen.go:211-251`) proves only that two recipes
name the same paths, which a manual recipe with entirely different operations
satisfies. It may not decide origin, provenance or coverage completeness
(`RGA-293`, `RGA-240`).

**A richer manual or provider recipe is never record-generated by
coincidence.** Origin holds only against a *complete* derivation's full bytes.
A near-match — differing by key order, indentation, one operation field, an
extra `replace-in-file` operation, or an added `created_by` edge — is **not**
record-generated. The producer then:

- preserves the recipe bytes exactly;
- fabricates no provenance and leaves existing provenance untouched;
- publishes coverage making no origin claim, naming GH #19 as the owner of
  historical/manual adoption in its diagnostic.

Refusing a near-match is deliberate: "almost the derivation" is the shape a
hand-edited recipe has, and treating it as generated would overwrite a human's
authorship record with a machine's (`RGA-291`, `RGA-292`).

**Recovery after a provenance failure is this same rule applied twice.** A run
that writes the recipe and then crashes before writing provenance leaves bytes
that the next run's derivation reproduces exactly — same patch, same capture,
same pure derivation. The rerun proves origin, repairs provenance with its own
truthful base and time, and republishes coverage (`RGA-295`). No marker
survives a crash, and none is needed.

If the derivation is itself incomplete, there are no complete canonical bytes
to compare against: origin is not proved, no provenance is written, and
coverage carries the incompleteness reasons instead.

## 7. Failure and warning vocabulary

Every code below is closed and machine-readable. Each row states exactly one
class; there are no dual-severity cells.

| Code | Surface | Class | Verdict/exit implication |
|---|---|---|---|
| `recipe-coverage-incomplete` | producers, verify, doctor `D10` | warning | producer proceeds; verify `warn`, exit `0` (rung 3) |
| `recipe-coverage-missing` | verify, doctor `D10` | warning | verify `warn`, exit `0` (rung 5); one code for both the legacy and the deleted case; always consumer-ineligible |
| `recipe-coverage-malformed` | verify, doctor `D10` | refusal | verify `block`, exit `2` (rung 1) |
| `recipe-coverage-patch-changed` | verify, doctor `D10` | refusal | verify `block`, exit `2` (rung 2) |
| `recipe-coverage-recipe-changed` | verify, doctor `D10` | refusal | verify `block`, exit `2` (rung 2) |
| `recipe-coverage-reference-stale` | verify, doctor `D10` | refusal | verify `block`, exit `2` (rung 2) |
| `recipe-coverage-owner-mismatch` | verify, doctor `D10`, **`apply --mode execute`** | refusal | verify `block`, exit `2` (rung 2); apply order 1. It names the **coverage envelope** owner mismatch (`coverage.feature` ≠ the requested slug), which is a binding failure with **no** schema reason. The recipe-versus-coverage owner mismatch is the schema reason `recipe-owner-mismatch` and is warning-class at rung 3 inside `recipe-coverage-incomplete` (§6.12) |
| `recipe-coverage-stale-marker` | verify, doctor `D10` | warning | verify `warn`, exit `0` (rung 4); consumer authority unusable. Surface name for the schema reason `recipe-stale-marker-present` (§6.12) |
| `recipe-coverage-patch-missing` | producers, verify, doctor `D10` | warning | producer proceeds; the record carries `canonical-patch-missing`; verify rung 3 |
| `recipe-coverage-patch-empty` | producers, verify, doctor `D10` | warning | producer proceeds and binds the exact raw patch hash with `effects: []`; the record carries `canonical-patch-empty`, which covers every semantically empty patch — zero bytes, whitespace-only bytes, or a clean parse yielding zero normalized effects; verify rung 3 |
| `recipe-coverage-patch-unparseable` | producers, verify, doctor `D10` | warning | producer proceeds and binds the exact raw patch hash with `effects: []`; the record carries `canonical-patch-unparseable`; verify rung 3. A strict parse refusal never leaves a feature without coverage |
| `recipe-coverage-recipe-undecodable` | producers, verify, doctor `D10` | warning | producer proceeds and binds the exact raw recipe hash with no operation assignment; the record carries `recipe-undecodable`; verify rung 3 |
| `recipe-generation-incomplete` | producers, **`apply --mode execute`** | warning at the producer; **refusal at apply** | producer proceeds and publishes no partial recipe, preserving any existing one. At `apply --mode execute` it is a named refusal with exit `2` and state-aware guidance in **two** shapes (§6.11): order 2, valid `incomplete` coverage with `recipe_present: false` and **no readable recipe on disk** — absent, or present and unreadable, with the actual cause printed on the `recipe status` line — and order 3, valid `incomplete` coverage with a recipe that was read and is **undecodable**, whose reason list includes `recipe-undecodable` and which never falls through to a raw JSON decode error. Both are phrased as "coverage binds no **readable** executable recipe", never as an accusation that generation withheld one. A coverage-binding refusal (order 1, either direction of a readable-existence, hash, reference, capture or envelope-owner mismatch) outranks both; order 4 — valid `incomplete` coverage with a present, decodable recipe — is **not** a refusal and executes with a non-authority warning; and the legacy no-coverage shapes and their exits are unchanged |
| `recipe-generation-not-regenerated` | `feature patch`, `reconcile --accept`, `cycle`, `apply --mode done` | warning | producer proceeds; coverage carries `producer-patch-rewrite` **and** `recipe-not-regenerated`, and only when the recipe no longer covers the rewritten patch; remediation names `tpatch record <slug> --regenerate-recipe` subject to the dry-derivation proof |
| `recipe-generation-provenance-unavailable` | record | warning | record proceeds; coverage carries `reference-not-durable` and V10 continues to govern the recipe |
| `recipe-generation-origin-unproved` | record, `feature patch` | warning | the recipe is preserved byte-identical, no provenance is fabricated, and GH #19 is named as the adoption owner |
| `recipe-coverage-manual-edit` | `tpatch edit`, verify, doctor `D10` | warning | P7 proceeds; the record carries `manual-bound-artifact-edit`; consumer-ineligible |
| `recipe-generation-no-truthful-regeneration` | producers, verify, doctor `D10` | warning | printed in place of a regeneration command when the dry derivation shows no command can produce complete coverage (§6.11, §6.13) |

rev-1's `recipe-coverage-missing-legacy` / `recipe-coverage-missing-produced`
pair is **retired**: absence has no carrier that could distinguish the two, so
GH #15 emits the single `recipe-coverage-missing` code and compensates with
unconditional consumer ineligibility (§6.12, §6.14).

**A coverage-publication failure has no code in this table, deliberately.** It
is not a coverage state a surface reports; it is the producing command's own
error, returned to the caller with a non-zero exit (§6.10). Minting a warning
code for it would invite exactly the behavior §6.10 forbids — printing a
status line and exiting `0` over an unrecorded write — so the eighteen rows
above, their seven mapped and eleven unmapped codes, are unchanged by that
contract.

Doctor `D10` surfaces every row above as a read-only warning finding
regardless of verify's class, because doctor does not gate.

**The two vocabularies are distinct and mapped one way, never merged.** This
table is the *surface* vocabulary — what verify, doctor and the producers
print. The coverage artifact's own `reason_codes` / `reasons` vocabulary is the
closed list in ADR-036 D3. The mapping between them is **one-way and partial**,
and its subject is the **explicitly mapped subset** §6.12 tabulates — exactly
these seven pairings: `recipe-coverage-stale-marker` ↔
`recipe-stale-marker-present`, `recipe-coverage-patch-missing` ↔
`canonical-patch-missing`, `recipe-coverage-patch-empty` ↔
`canonical-patch-empty`, `recipe-coverage-patch-unparseable` ↔
`canonical-patch-unparseable`, `recipe-coverage-recipe-undecodable` ↔
`recipe-undecodable`, `recipe-coverage-manual-edit` ↔
`manual-bound-artifact-edit`, and `recipe-generation-not-regenerated` ↔ the
*pair* `producer-patch-rewrite` + `recipe-not-regenerated`.
`recipe-coverage-owner-mismatch` is **not** among them: rev-5 paired it with
`recipe-owner-mismatch`, but the surface code names the coverage envelope's
misdelivery (a rung-2 binding failure) while the schema reason names the
recipe's own owner (rung-3 incompleteness), so the pairing gave one name two
severities (§6.12).

**Every other surface code in this table — eleven of them — is unmapped by
construction.**
`recipe-coverage-malformed`, `recipe-coverage-patch-changed`,
`recipe-coverage-recipe-changed`, `recipe-coverage-reference-stale`,
`recipe-coverage-owner-mismatch`,
`recipe-coverage-missing`, `recipe-coverage-incomplete`,
`recipe-generation-incomplete`, `recipe-generation-provenance-unavailable`,
`recipe-generation-origin-unproved` and
`recipe-generation-no-truthful-regeneration` name **verify-, binding- or
aggregate-level conditions that have no schema-reason counterpart at all** — a
decode failure, a recomputed-hash mismatch in either direction, a misdelivered
record, an absent file,
the aggregate incomplete row, a producer-side outcome. They are not omissions
from the mapping, and minting schema reasons for them would drag verify-layer
conditions into the artifact's vocabulary. Meanwhile every effect-local reason
and the record-level `reference-not-durable`, `operation-surplus`,
`simulation-mismatch` and `recipe-owner-mismatch` have **no** surface code of
their own and are aggregated
under `recipe-coverage-incomplete`. A bijection is therefore impossible and is
not claimed. What **is** guaranteed, over the whole of both vocabularies rather
than the mapped subset alone, is disjointness: no token appears in both layers.

## 8. Implementation slices

### S0 - Frozen evidence

- Preserve current record/apply/verify output goldens, including the exact
  `Recipe executed: %d/%d operations succeeded` line, the current
  `Recipe generated: artifacts/apply-recipe.json (%d ops)` line, the legacy
  `no recipe found — run 'tpatch implement <slug>' first` error with its exit
  `1`, and the ADR-029 D7 superseded-downgrade note text.
- Promote the adjacent-conflict scripts and downstream V10 case into fixtures.
- Freeze a pre-v0.17.0 legacy repository fixture that must stay verify-green
  through the whole wave, **including a feature carrying `recipe-stale.json`
  and no coverage**.
- Freeze the current `feature patch`, `reconcile --accept`, `cycle`,
  `apply --mode done`, `implement` and `tpatch edit` artifact-write goldens,
  since S4 adds a publication step to each.
- Freeze the current `FilesInPatchStrict` b-side results for its five shipped
  callers (PI-12), so the S1 grammar extension cannot change them.
- Add failing contract tests before producer refactors.

### S1 - Strict effects, the parser inventory and immutable capture

- Extend the existing strict parser to the full normalized effect model with
  orthogonal `change_kind` / `content_kind` / `object_kind` axes, modes read
  from the immutable observation, per-side observation flags, and
  `patch_fragment_sha256` whose record boundaries come from the grammar's own
  recognized line-start `diff --git` records.
- Remove `parsePatchTouchedFiles` (PI-1).
- Delete `gitutil.FilesInPatch` or demote it to a test-only helper (PI-2) so
  no production path reaches it.
- **Migrate the complete derived inventory** onto the strict authority or exact
  adapters over it: `touched_paths` (PI-3,
  `internal/workflow/patch_generations.go:76`) and the reconcile derivation
  fallback (PI-4, `internal/workflow/reconcile_derivation.go:118-124`)
  propagate the strict error into their **existing** continue-on-error
  handlers; file novelty path **and change-kind** classification (PI-5,
  `internal/workflow/file_novelty.go:130-231`) and hunk-overlap path
  attribution (PI-6, `internal/workflow/hunk_overlap.go:150-175`) move to
  adapters.
- Add `PathsAffectedByPatchStrict` — the sorted unique union of canonical path
  plus `old_path` for rename/copy — and migrate PI-7's three call sites
  (`internal/cli/cobra.go:919`, `internal/cli/cobra.go:1131`,
  `internal/cli/feature_unapply.go:156`) to **fail closed** on its error before
  any snapshot, diff or write. None of the three has an existing fail-soft
  handler; each gains a new refusal path.
- Add the PI-7 corpus: quoted paths with spaces, and rename/copy effects whose
  **old side** must survive into the returned scope.
- Register PI-12 (`internal/cli/land.go:767,1212`,
  `internal/workflow/refresh.go:59`,
  `internal/workflow/verify_landed.go:1009,1163`) and pin the b-side contract
  with a regression guard.
- Register PI-8, PI-9 and PI-10 as non-authoritative and add the guard that
  fails if any of them starts feeding a path, effect or operation decision.
- Replace the `internal/cli/cobra.go:1908` operation count with the derived
  recipe's actual operation count (PI-10, §6.1.3).
- Add the source-inventory guard that derives the registry from production
  sources and fails on an unregistered `diff --git` reader.
- Capture pre/postimage observations for every record mode **and for every
  other governed producer**, recording per-side observation flags, including
  P7's before/after snapshots of the **resolved** artifact path.
- Refactor `openInEditor` (`internal/cli/phase2.go:251-262`) to return the
  `c.Run` error, and propagate it at its call sites.
- Add the exotic path/type corpus, including the C-quoted path that the
  fail-soft parsers used to drop, and a patch whose added content contains a
  literal `diff --git` line that must not open a new fragment.

### S2 - Preimage synthesis, pure derivation and convergent provenance

- Make the derivation pure: postimages come from the captured observation,
  not the live worktree, and no timestamp enters the bytes.
- Add the canonical deterministic recipe encoder.
- Generate explicit creation/existing-file preimages over the `100644` domain.
- Classify executable, symlink, gitlink, binary, mode-change, delete, rename
  and copy effects as unsupported with every applicable reason code.
- Implement the §6.16 exact-derived-byte origin proof and retire
  `compareRecipeFileSets` from origin and completeness decisions.
- Implement the §6.6 convergence rule across generated, regenerated and
  no-op/preserved reruns.
- Preserve matching provenance bytes and timestamp on noop.
- Never fabricate provenance for a recipe the derivation does not reproduce
  exactly, including a near-match.
- Mark `parent-created-target-unsupported` where a correct preimage would
  depend on a parent-created path.

### S3 - Coverage schema and simulation

- Add strict types, decoder, canonical encoder and hash helpers for the exact
  ADR-036 D3 schema, including `producer`, `patch_present`, `recipe_present`,
  `recipe_decodable`, the three kind axes, both modes, both observation flags,
  both presence flags, `patch_fragment_sha256`, `effect_sha256`, sorted
  `reason_codes[]`, sorted record-level `reasons[]`, `contextual_hint` and
  `cross_base_status`.
- Enforce the reason-allocation table: a record-level code inside an effect,
  or an effect-local code inside `reasons`, is refused at decode.
- Enforce the `disposition` mapping and its `mismatch` → `ambiguous` →
  `unsupported` precedence, refusing any contradictory pairing in both
  directions, including `operation-missing` beside any other effect-local code.
- Enforce the observation-flag rules: `*_observed: false` forces
  `*_present: false`, `""` hash, `""` mode and the matching
  `preimage-unavailable` / `postimage-unavailable` reason; and refuse the
  contradictory observed-absence shape on a side the `change_kind` requires to
  exist, so an unestablished extant side must be published as unobserved.
- Implement the total `object_kind` / `content_kind` selection rules over the
  effect's **extant** sides, including `unknown`, and enforce its constraints: a
  mandatory matching unavailable reason, an `ambiguous`-class disposition, and a
  decode refusal beside `coverage_status: complete`. An unobserved **non-extant**
  side raises its unavailable reason without degrading either axis.
- Compute `reasons` and `reason_codes` as the **full** evaluation of their
  closed condition lists and validate them by **set equality**, so an
  applicable code can be neither omitted nor invented — evaluating each code's
  own raising condition, including `operation-missing`'s scoping to effects
  that are otherwise representable and carry no assigned operation.
- Derive `ordinal` as the one-based strict-grammar record order of the
  canonical patch, and derive `patch_fragment_sha256` boundaries from those
  same record starts, in that order.
- Enforce the `coverage_status` iff predicate in both directions over the
  canonical ten predicates, so no contradictory `complete`/`incomplete` record
  decodes — including `complete` beside an empty `effects` array.
- Implement the absent-, semantically-empty- and unparseable-patch records with
  `canonical-patch-missing` / `canonical-patch-empty` /
  `canonical-patch-unparseable`, `effects: []` and
  `cross_base_status: unsupported`, where semantic emptiness covers zero bytes,
  whitespace-only bytes and a clean parse yielding zero normalized effects, and
  the present-but-undecodable recipe
  record with `recipe-undecodable` and a bound raw-byte hash.
- Assign operations to effects.
- Simulate exact preimage -> postimage within the `100644` domain.
- Reclassify the result as present without second execution, and emit
  `operation-not-reclassifiable` where no exact reclassification exists.
- Add the PRD/ADR schema-block **and completeness-predicate** parity guards,
  the latter also validating every `predicate N` cross-reference in both
  documents.

### S4 - The shared publication API and the seven-producer obligation

- Add the single coverage publication entry point and its typed input.
- Wire P1-P7 to it: `record`, `feature patch refresh|fixup`,
  `RefreshAfterAccept`, `cycle`'s patch-capture step, `runApplyDone`,
  `RunImplement` plus the `implement --manual` checkpoint, and `tpatch edit`
  on a bound artifact.
- Add the `producer` values `implement` and `artifact-edit`, and the
  `reconcile` and `no-capture` capture modes.
- Implement the event-boundary rules over the three event categories:
  publication per governed event, P4's composite implement/patch split, P2's
  category-(c) same-patch checkpoint repair and its non-event empty capture,
  and the remaining no-event cases (P4/P5 empty capture, reapply branches,
  `tpatch edit` whose resolved path is not bound).
- Trigger P7 on the **resolved** artifact path, so a feature-root decoy is not
  an event and `artifacts/<name>` is.
- Propagate the currently ignored `post-apply.patch` write error in
  `internal/cli/phase2.go:166`, and the `openInEditor` error at **both** of its
  call sites (`internal/cli/c1.go:91`, `internal/cli/phase2.go:88`), so event
  decisions rest on a known outcome.
- Publish P7 coverage for a changed bound artifact **before** returning an
  editor error, and treat an unset `$EDITOR` as a non-event.
- Attach P6 publication to the common post-write path both parse arms fall
  through to, so a **successful** raw write on the unmarshal-failure arm
  (`internal/workflow/implement.go:194`) publishes bound incomplete coverage
  with `recipe-undecodable` after the provenance and state-mark attempts and
  before `RunImplement` propagates its final
  `return s.MarkFeatureState(...)` (`internal/workflow/implement.go:243`) —
  publishing even when that state mark fails, returning the state-mark error to
  the caller, and surfacing rather than dropping a coverage-publication failure
  — while a **failed** `WriteArtifact` on either arm publishes nothing.
- Owe P3 coverage on the **unconditional** patch write at
  `internal/workflow/refresh.go:82`, independently of the generation append at
  `:102`, which runs only when `newPatch != originalPatch` (`:93`).
- Emit `producer-patch-rewrite` + `recipe-not-regenerated` **only** where the
  rewritten patch is no longer covered and simulated by the recipe on disk, and
  always as a **pair** — including on P2's patch-writing run — never one alone.
- Surface a coverage-publication failure on **every** P1-P7 event: return it
  and exit non-zero, with no success-shaped fallback, and chain it with any
  concurrent state-mark or editor failure rather than dropping either.
- Coordinate recipe, provenance, generation and coverage ordering, including
  P6's publish-after-provenance order.
- Write coverage unconditionally on every governed producer event, including
  record's noop.
- Repair provenance on the no-op path per §6.6.
- Preserve existing recipe on incomplete regeneration.
- Add the producer-inventory guard, the **site-to-producer mapping** guard and
  the publication guard, with the registry count of seven pinned separately
  from the mapping.
- Add crash/failure injection at every boundary, including failure between the
  recipe write and the provenance write, for each producer.

### S5 - Apply, verify, doctor and accounting

- Add exact-postimage no-op classification for **both** narrowed ADR-029 D3
  cases: the empty-preimage collision and the non-empty expected-hash mismatch,
  each only when the observed bytes equal the operation postimage.
- Keep the two unchanged ADR-029 D3 refusals (expected hash with a missing
  target, unreadable target), all-or-nothing precheck atomicity, path safety
  and ADR-029 D7 supersession severity byte-for-byte.
- Add `Applied` + `Skipped` accounting and the already-present message.
- Add the `recipe_generation_coverage` verify row with the §6.12 precedence
  ladder, uniform warning-class missing coverage, warning-class stale-marker
  handling, and the surface/schema vocabulary mapping.
- Add the total seven-case `apply --mode execute` classifier: the bidirectional
  coverage-binding refusal at order 1, the named `recipe-generation-incomplete`
  refusal at orders 2 and 3 with exit `2` and its state-aware guidance blocks,
  the executing-with-warning path at order 4, and the two legacy shapes and the
  complete-coverage shape unchanged — with orders 5 and 6 split on **readable
  existence** too, so an absent or unreadable recipe beside absent coverage
  keeps the shipped `LoadRecipe` behavior and exit `1`. Order 2 matches on
  **readable existence**,
  so a present-but-unreadable recipe takes it and the refusal prints the actual
  read cause on its `recipe status` line. Extend the
  `internal/cli/reject.go:38-47` comment block
  to name pre-mutation executable-plan validation.
- Recompute `patch_present` / `recipe_present` from on-disk readable existence
  in both directions wherever coverage is read, so `true`-beside-unreadable and
  `false`-beside-readable are both binding-stale, and expose a test seam that
  injects a read failure without depending on OS permission semantics.
- Separate the two owner checks: a coverage envelope owner mismatch is a rung-2
  binding failure surfaced as `recipe-coverage-owner-mismatch` with no schema
  reason, while a recipe owner mismatch is the schema reason
  `recipe-owner-mismatch` reported at rung 3 inside the aggregate row.
- Add read-only doctor `D10` and the dry-derivation gate on every remediation
  that names a regeneration command.
- Pin legacy behavior and remediation, including the pre-v0.17.0 stale-marker
  fixture staying green.

### S6 - Public parity and soak

- Update SPEC, feature layout, operator docs and six skill surfaces, including
  the seven-producer table and the statement that a coverage warning is never
  a replay grant.
- Extend asset parity and overclaim guards, including "coverage is not
  cross-base safety" and "a `warn`/exit `0` coverage row is not eligibility".
- Run downstream fixture against a realistic cumulative repository, including
  a pre-v0.17.0 legacy cohort that must stay green — one feature with no
  coverage, one carrying a pre-v0.17.0 `recipe-stale.json`.
- Exercise all seven producers over that repository and confirm none leaves
  stale complete coverage.
- Ship v0.17.0 only after review and soak.

## 9. Acceptance matrix

Kinds: `I` integration/runtime, `C` failure/concurrency, `G` semantic guard,
`U` unit/schema, `S` security/privacy.

Every `G` row runs the **same validator** used by the production path and
feeds it a **semantically wrong input**, named in the Observable column.
Token-presence-only fixtures do not satisfy a semantic guard.

### 9.1 Producer inventory, publication and event boundaries

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-001 | G | Producer inventory totality | Wrong-input fixture `producer-inventory-five-only` (a registry naming only `record` / `feature-patch-amend` / `reconcile-accept` / `cycle` / `apply-done`) fails the same producer-registry validator that the complete seven-entry registry passes |
| RGA-002 | G | Site-to-producer mapping sensitivity | Wrong-input fixture `unmapped-bound-input-site` (a new production writer or checkpointer of `post-apply.patch` or `apply-recipe.json` that maps to no P1-P7 entry) fails the same site-mapping guard that the shipped eleven-row mapping passes |
| RGA-003 | G | Registry count | Wrong-input fixture `producer-enum-count-drift` (a six-value `producer` enum beside a seven-entry registry) fails the same registry-count validator that the seven/seven configuration passes |
| RGA-004 | G | Writer-count threshold rejected | Wrong-input fixture `guard-counts-bound-writer-sites` (a guard asserting an expected bound-writer count and failing on "the eighth") fails against the **shipped** tree, which already has eight direct bound `WriteArtifact` sites across seven producers; the mapping guard passes on the same tree |
| RGA-005 | U | Site-to-producer mapping totality | Every **reachable call chain** through the §6.15 mapping table — `cobra.go:1795`, `recipe_autogen.go:204` **from each of its two callers**, `feature_patch.go:114`, `feature_patch.go:104-112`, `refresh.go:82`, `phase2.go:166`, `cobra.go:1044`, `implement.go:194`, `implement.go:209`, `manual.go:51-80` and the `$EDITOR` delegation — maps to exactly one of P1-P7; the shared helper's two chains map to P1 and P2 respectively, and a mapping keyed on the source line rather than the chain fails the row |
| RGA-006 | G | Shared helper maps by caller | Wrong-input fixture `autogen-helper-mapped-to-its-own-producer` (`recipe_autogen.go:204` given a `producer` value of its own instead of inheriting P1 from `cobra.go:1900` or P2 from `feature_patch.go:135`) fails the same mapping validator |
| RGA-007 | I | `tpatch land` is P1 orchestration | `tpatch land` publishes exactly one coverage record, with `producer: record`, written by the embedded record step (`internal/cli/land.go:180`); no `land` producer value exists and no second record is published |
| RGA-008 | G | Single publication API | Wrong-input fixture `producer-inline-coverage-writer` (a producer encoding coverage bytes itself instead of calling the shared publication entry point) fails the same publication validator |
| RGA-009 | I | P1 `tpatch record` | Coverage published last with `producer: record`; recipe, provenance, generation, coverage in that order |
| RGA-010 | I | P2 `feature patch refresh` | Patch rewritten (`internal/cli/feature_patch.go:114`); autogen runs non-regenerating; coverage republished with `producer: feature-patch-amend` |
| RGA-011 | I | P2 `feature patch fixup` | Same publication as refresh; the fixup generation append does not substitute for coverage |
| RGA-012 | C | P2 preserved drifted recipe raises the paired rewrite codes | Recipe preserved byte-identical and `recipe-stale.json` written; the patch-writing run publishes `incomplete` whose sorted `reasons` are **set-equal** to the applicable codes and therefore contain **both** `producer-patch-rewrite` **and** `recipe-not-regenerated` (the pair is never split) plus `recipe-stale-marker-present`; a record carrying `recipe-not-regenerated` alone fails the row, and the prior complete coverage is never left in place |
| RGA-013 | C | P2 category-(c) checkpoint repairs coverage | A refresh whose captured patch is non-empty and hashes equal to the **latest** generation, so `ClassifyPatchGenerationKind` returns `Append == false` (`internal/cli/feature_patch.go:104-112`, `internal/store/patch_generation_kinds.go:46-49`), still publishes coverage before returning, prints the unchanged `no patch byte change; refresh skipped` line plus the coverage status line, and writes **no** patch, numbered patch, recipe, provenance, generation or state advance; an early return that skips publication fails |
| RGA-014 | C | P2 `fixup` category-(c) checkpoint | The `fixup` arm of the same branch publishes identically and keeps its `no patch byte change; fixup skipped` line |
| RGA-015 | C | P2 empty capture is not an event | A refresh whose capture returns zero bytes (`internal/cli/feature_patch.go:91-95`) publishes nothing; it is not a checkpoint of the stored canonical patch and pre-existing coverage is untouched |
| RGA-016 | I | P2 changed patch, preserved recipe, derived bytes still equal | When the new patch's full derived recipe bytes equal the preserved recipe exactly, coverage is `complete` and neither `producer-patch-rewrite` nor `recipe-not-regenerated` is raised |
| RGA-017 | I | P3 `reconcile --accept` → `RefreshAfterAccept` | `internal/workflow/refresh.go:82` rewrites the patch and leaves the recipe untouched by design; coverage is republished `incomplete` with `producer-patch-rewrite`, `recipe-not-regenerated`, `producer: reconcile-accept`, `capture.mode: reconcile` |
| RGA-018 | I | P4 `cycle` patch-capture step | `internal/cli/phase2.go:166` writes the patch with no autogen and no generation append; coverage is republished (recomputed or explicitly incomplete) with `producer: cycle` |
| RGA-019 | I | P4 composite, `--skip-execute` early exit | `cycle --skip-execute` (`internal/cli/phase2.go:122-126`) returns after the implement step; a P6 record with `producer: implement` is already on disk, and no P4 obligation is outstanding |
| RGA-020 | I | P4 composite, declined prompt after implement | Declining `Execute recipe now?` (`internal/cli/phase2.go:127-129`) or `Record captured patch?` (`internal/cli/phase2.go:152-154`) leaves the P6 record in place with no P4 publication owed |
| RGA-021 | C | P4 empty capture | A capture producing an empty patch skips the write (`internal/cli/phase2.go:165`), so no patch event occurs and no patch-bound coverage is owed |
| RGA-022 | C | P4 patch-write error propagates | A forced failure of `s.WriteArtifact(slug, "post-apply.patch", patch)` (`internal/cli/phase2.go:166`) returns an error instead of being discarded, and no coverage is published for the failed write |
| RGA-023 | I | P5 `tpatch apply --mode done` | `internal/cli/cobra.go:1044` writes the patch inside `runApplyDone`; coverage is republished with `producer: apply-done` |
| RGA-024 | C | P5 reapply branch is not an event | The reapply branch (`internal/cli/cobra.go:1006-1022`) reads the canonical patch and writes none; no P5 publication is owed and existing coverage is untouched |
| RGA-025 | C | P5 empty capture | An empty captured patch skips the write (`internal/cli/cobra.go:1043`), so no patch event occurs |
| RGA-026 | I | P6 `implement`, valid-JSON parse arm | `internal/workflow/implement.go:209` writes the reserialized recipe; coverage is published with `producer: implement`, `capture.mode: no-capture`, `recipe_present: true` and `recipe_decodable: true` |
| RGA-027 | I | P6 heuristic fallback flows through the same parse | A heuristic-fallback run publishes coverage identically; the fallback warning does not suppress publication, and the arm taken is decided by `json.Unmarshal` (`internal/workflow/implement.go:192`), not by which generator produced the bytes |
| RGA-028 | C | P6 unmarshal-failure arm publishes on the common path | A response the extractor cannot unmarshal is written raw at `internal/workflow/implement.go:194`; on a **successful** write coverage is published with `recipe_present: true`, `recipe_decodable: false`, `recipe_sha256` over the exact raw bytes and `recipe-undecodable` in `reasons`, at the common coverage-last point both arms fall through to and before `RunImplement` propagates its final `return s.MarkFeatureState(...)` (`internal/workflow/implement.go:243`). A run that leaves the prior coverage in place fails; so does an implementation that hooks publication to a parse-error return the source does not have, or to a literal `nil` |
| RGA-029 | I | P6 `implement --manual` checkpoint | A successful checkpoint of an externally authored recipe (`internal/store/manual.go:51-80`) publishes coverage with `producer: implement`; a failed checkpoint (missing artifact, invalid JSON) publishes nothing |
| RGA-030 | C | P6 with no canonical patch | Coverage is published with `patch_present: false`, `patch_sha256: ""`, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported` and `canonical-patch-missing`; silent absence fails |
| RGA-031 | I | P6 publication order | Recipe (`internal/workflow/implement.go:194,209`) → provenance (`internal/workflow/implement.go:220-237`) → coverage; coverage is last |
| RGA-032 | I | `record` → `implement` → `verify` | A P6 record carrying `canonical-patch-missing` is replaced by a bound record when `record` runs, and the following verify is green |
| RGA-033 | I | P6 rerun on a recorded feature | With a canonical patch already present, `implement` recomputes coverage against it rather than publishing `canonical-patch-missing` |
| RGA-034 | I | P7 `tpatch edit` on `apply-recipe.json` | The resolved path is `<feature>/artifacts/apply-recipe.json`, the editor changes bytes, and on return coverage is republished with `producer: artifact-edit` |
| RGA-035 | C | P7 feature-root decoy is not bound | With `<feature>/apply-recipe.json` present at the feature root, `tpatch edit <slug> apply-recipe.json` resolves to the decoy first (`internal/cli/c1.go:33-44`) and publishes nothing; the canonical `artifacts/apply-recipe.json` is untouched and its coverage still binds |
| RGA-036 | I | P7 explicit `artifacts/` spelling is bound | With the same decoy present, `tpatch edit <slug> artifacts/apply-recipe.json` resolves to the canonical path on the first probe and **is** a P7 event |
| RGA-037 | C | P7 publishes before propagating an editor error | An editor exiting non-zero after saving changed bytes still yields a republished coverage record, and the editor error is returned afterwards |
| RGA-038 | U | `openInEditor` returns its error at both call sites | `openInEditor` propagates the `c.Run` error instead of discarding it (`internal/cli/phase2.go:261`), and **both** of its call sites — `internal/cli/c1.go:91` and `internal/cli/phase2.go:88` — handle it; a refactor sized for three call sites, or one that leaves either caller unhandled, fails |
| RGA-039 | C | Editor exits before a later save | An editor that returns with the bytes unchanged publishes nothing; a subsequent out-of-band save is external tamper and surfaces as binding-stale coverage at the next read, not as a P7 publication |
| RGA-040 | I | P7 `tpatch edit` on `post-apply.patch` | Same publication for the canonical patch artifact, selected by argument or by the `applied`/`unapplied` state default (`internal/cli/c1.go:45-60`) and resolved under `artifacts/` |
| RGA-041 | C | P7 edit that changed no byte | The before/after snapshots match; nothing is published and the pre-existing coverage still binds |
| RGA-042 | C | P7 on a non-bound artifact | `tpatch edit <slug> spec.md` (and `request.md`, `analysis.md`, `exploration.md`) publishes nothing and is not a governed event |
| RGA-043 | C | P7 reconstruct-and-validate succeeds | A pre-edit durable reference that reconstructs and revalidates against a recomputed `preimage_set_sha256` is carried forward as `reference.kind: commit` |
| RGA-044 | C | P7 reconstruct-and-validate fails | Coverage is `incomplete` with `manual-bound-artifact-edit`, plus `reference-not-durable` when no durable reference could be reconstructed |
| RGA-045 | G | P7 artifact boundary | Wrong-input fixtures `edit-spec-md-publishes-coverage` (a non-bound artifact edit treated as a governed event) and `p7-triggers-on-user-token` (the trigger matched against the typed token rather than the resolved path, so the feature-root decoy publishes) each fail the same producer-event validator |
| RGA-046 | C | External filesystem tamper | An out-of-band edit to `apply-recipe.json` made without running `tpatch` publishes nothing, and the next read lands on the binding-stale rung; it is never accepted as current |
| RGA-047 | C | No silently stale complete coverage | For each of P2-P7 starting from valid `complete` coverage, the post-event coverage is either freshly recomputed and valid or explicitly `incomplete`; a byte-identical carry-over of the prior complete record fails |
| RGA-048 | C | Producer crash before publication | Coverage is absent or hash-stale after the failure; rerunning the same producer republishes it without a duplicate patch generation |
| RGA-049 | U | `producer` closed enum | Exactly `record`, `feature-patch-amend`, `reconcile-accept`, `cycle`, `apply-done`, `implement`, `artifact-edit` decode; an unlisted producer is refused and a six-value enum fails the arithmetic check |
| RGA-050 | I | GH #13 refuses non-record producer output | A `reconcile-accept`, `cycle`, `implement` or `artifact-edit` feature whose coverage is `incomplete` is ineligible with the exact producer reason codes |
| RGA-051 | G | Publication is unconditional per event | Wrong-input fixture `producer-skips-coverage-on-noop` (a governed producer returning before publication when it changed no recipe byte) fails the same publication validator |
| RGA-052 | G | Event boundary, not invocation boundary | Wrong-input fixture `publication-per-invocation-not-per-event` fails the same event-boundary validator on genuinely **no-event** commands: `tpatch edit <slug> spec.md`, a P4/P5 capture that produces an empty patch, and the `apply --mode execute` / `apply --mode done` read-only reapply branches. The P2 category-(c) checkpoint is **not** part of this fixture — it is a real event and publishing there is required |
| RGA-053 | G | Three event categories | Wrong-input fixture `event-definition-write-only` (the rev-3 two-category definition, which excludes the contracted re-observation checkpoint) fails the same event-boundary validator on P2's non-writing same-patch branch |
| RGA-054 | C | P6 unmarshal-failure arm does not return the parse error, and still publishes | A response that fails `json.Unmarshal` is written raw and the `WriteArtifact` **succeeds**: `RunImplement` does **not** return the unmarshal error, it falls through to the provenance attempt and to its final `return s.MarkFeatureState(...)` (`internal/workflow/implement.go:243`), and coverage is published at the common coverage-last point before that return is propagated, with `recipe_present: true`, `recipe_decodable: false`, the raw-byte `recipe_sha256` and `recipe-undecodable` |
| RGA-055 | C | P6 publishes when the state mark fails | With the recipe write successful and `s.MarkFeatureState` (`internal/workflow/implement.go:243`) forced to fail, coverage is still published and still binds the recipe that landed; `RunImplement` returns the state-mark error, and a run that skips publication because the state mark failed fails the row |
| RGA-056 | C | P6 coverage-publication failure is surfaced | With publication forced to fail, the error reaches the caller rather than being dropped; when the state mark **also** fails, the returned error names both, state-mark first, and neither failure is silently lost |
| RGA-057 | C | Coverage-publication failure is surfaced on every producer | One **table-driven** case per governed event — P1 `record`, P2 write and P2 category-(c) checkpoint, P3 `reconcile --accept`, P4 `cycle` patch step, P5 `apply --mode done`, P6 both parse arms and `--manual`, P7 bound edit — with the shared publication API forced to fail: each command **returns the failure and exits non-zero**, none prints its ordinary success line, and none reports success because the patch or recipe write already landed. Coverage may be left absent or stale on disk (publication is last) and the next read rejects it; a producer that logs-and-continues, or that maps the failure to exit `0`, fails the row. Seven per-producer prose rows are explicitly **not** the shape of this guarantee |
| RGA-058 | G | P6 finalizer anchored to a literal `nil` | Wrong-input fixture `p6-publication-hooked-to-literal-nil-return` (a finalizer that publishes only when `RunImplement` is about to return a literal `nil`, so a failing state mark skips coverage) fails the same event-obligation validator that the return-point finalizer passes |
| RGA-059 | U | P3 owes coverage on the unconditional write | `RefreshAfterAccept` writes `post-apply.patch` unconditionally (`internal/workflow/refresh.go:82`) while `AppendPatchGenerationForFeature` runs only when `newPatch != originalPatch` (`internal/workflow/refresh.go:93,102`); an accept whose regenerated bytes equal the original appends no generation and still owes a P3 coverage publication for the write |
| RGA-060 | C | P6 failed bound write is not an event | A forced `WriteArtifact` failure on either arm (`internal/workflow/implement.go:195,210`) returns the write error, publishes **no** coverage, and leaves pre-existing coverage untouched; nothing was written, so nothing is owed |
| RGA-061 | C | `$EDITOR` unset is not a P7 event | With `$EDITOR` unset, `tpatch edit <slug> artifacts/apply-recipe.json` prints the pointer line (`internal/cli/phase2.go:252-255`), starts no process, changes no byte and publishes nothing; pre-existing coverage still binds |
| RGA-062 | C | P2 older-generation match is not the checkpoint branch | A capture whose bytes equal a **non-latest** generation classifies `Append == true` (`internal/store/patch_generation_kinds.go:46-49`) and takes the writing category-(a) path; only equality with the **latest** generation reaches the category-(c) checkpoint |

### 9.2 Strict effect parsing and parser ownership

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-063 | G | Effect-grammar ownership | Wrong-input fixture `parser-fields-split-quoted-path` (a reintroduced `strings.Fields` b-side splitter fed a C-quoted path containing a space) fails the same normalized-effect validator that the strict grammar passes |
| RGA-064 | G | Parser inventory totality | Wrong-input fixture `unregistered-diff-git-parser` (a new production `diff --git` scanner absent from the PI-1..PI-12 registry) fails the same derived parser-inventory guard |
| RGA-065 | G | Adapter exactness | Wrong-input fixture `adapter-drops-quoted-path` (a thin adapter that re-implements header splitting instead of projecting the strict effect set) fails the same path-totality validator |
| RGA-066 | G | Non-authoritative scanner boundary | Wrong-input fixture `countpatchfiles-feeds-effect-count` (`countPatchFiles` output used as an effect count) fails the same authority-boundary validator; `headerReferencedGitPath` and `stripGitInternalFileStanzas` stay registered as refusal/sanitization-only |
| RGA-067 | G | PI-10 operation-count boundary | Wrong-input fixture `recipe-op-count-from-file-counter` (the `internal/cli/cobra.go:1908` use of `countPatchFiles(patch)-len(skippedPaths)` as an operation count) fails the same authority-boundary validator |
| RGA-068 | U | PI-10's four production consumers | The registry records exactly four consumers of `countPatchFiles`: `internal/cli/cobra.go:1863` (`filesChanged`), `internal/cli/feature_patch.go:163` (`%d files` in `Amended patch for ...`) and `internal/cli/record_collision.go:96` (`Files:` in a collision entry) keep their shipped output byte-for-byte, while `internal/cli/cobra.go:1908`'s operation count is the only one removed/migrated. An inventory naming two consumers, or one that migrates a file-count consumer, fails the row |
| RGA-069 | I | Recipe operation count is derived | `Recipe generated: artifacts/apply-recipe.json (%d ops)` prints the operation count of the recipe just derived, and differs from the `diff --git` file count on a patch whose effects do not map one-to-one onto operations |
| RGA-070 | I | Withheld recipe prints status, not a count | When no recipe was published the line prints the incomplete coverage status; no computed file count is printed |
| RGA-071 | G | `touched_paths` migration | Wrong-input fixture `touched-paths-failsoft-quoted-drop` (the fail-soft splitter restored under `AppendPatchGenerationForFeature`) fails the same path-totality validator the strict caller passes |
| RGA-072 | G | Reconcile derivation migration | Wrong-input fixture `derivation-failsoft-quoted-drop` (the fail-soft splitter restored under `touchedPathsFromPostApplyPatch`) fails the same path-totality validator |
| RGA-073 | G | File-novelty migration | Wrong-input fixture `novelty-fields-split-and-naive-dequote` (`parseDiffGitPaths` plus `cleanPatchPath`'s `strings.Trim(path, "\"")` restored under `parsePatchNoveltyPaths`) fails the same path-and-change-kind validator |
| RGA-074 | G | Hunk-overlap path attribution | Wrong-input fixture `hunk-overlap-fields-path-attribution` (hunk ranges attributed by the naive header splitter) fails the same path-attribution validator; the hunk-range projection itself is unchanged |
| RGA-075 | U | PI-3/PI-4 error routing | The strict error surfaces through each caller's **existing** continue-on-error handler (`internal/workflow/patch_generations.go:76`, `internal/workflow/reconcile_derivation.go:118-124`); no silently short path list is returned |
| RGA-076 | U | `PathsAffectedByPatchStrict` union | Returns the sorted, unique union of each effect's canonical path plus `old_path` for rename and copy, and is not the b-side projection |
| RGA-077 | U | Rename old side retained | A rename effect yields both source and destination paths, matching `internal/gitutil/unapply_test.go:83-102` |
| RGA-078 | U | Copy old side retained | A copy effect yields both source and destination paths |
| RGA-079 | U | Quoted path with a space | A C-quoted path containing a space decodes byte-correctly through the strict all-paths projection, as it already does through `internal/gitutil/unapply.go:47-49` |
| RGA-080 | U | Strict refusal parity | `PathsAffectedByPatchStrict` refuses exactly the inputs `FilesInPatchStrict` refuses, returning an error and a nil slice rather than a partial scope |
| RGA-081 | G | PI-7 both-side scope | Wrong-input fixture `unapply-scope-from-b-side-projection` (PI-7 migrated onto `FilesInPatchStrict`, dropping the rename source) fails the same unapply-scope validator |
| RGA-082 | C | `cobra.go:919` fails closed | A patch whose header the strict grammar refuses returns an error before `SnapshotWorktreePaths` runs; no snapshot, no reapply, no mutation |
| RGA-083 | C | `cobra.go:1131` fails closed | The same patch returns an error from `validateReapplyMaterialization` before `DiffFromCommitForPaths` is called |
| RGA-084 | C | `feature_unapply.go:156` fails closed | The same patch returns an error before any snapshot, reverse patch or unapply artifact write |
| RGA-085 | G | PI-7 fail-closed guard | Wrong-input fixture `unapply-strict-error-swallowed` (a call site discarding the strict error and continuing with a partial path list) fails the same fail-closed validator |
| RGA-086 | G | No false fail-soft claim | Wrong-input fixture `pi7-error-routed-to-nonexistent-handler` (a migration asserting a pre-existing fail-soft handler at any of the three PI-7 call sites) fails the same call-site-shape validator |
| RGA-087 | U | PI-12 contract, `land.go:767` | The b-side path list is byte-identical to the pre-GH #15 result for the frozen corpus |
| RGA-088 | U | PI-12 contract, `land.go:1212` | Same |
| RGA-089 | U | PI-12 contract, `refresh.go:59` | Same |
| RGA-090 | U | PI-12 contract, `verify_landed.go:1009,1163` | Same for both call sites |
| RGA-091 | G | PI-12 regression guard | Wrong-input fixture `filesinpatchstrict-widened-to-both-sides` (the shared grammar extended so `FilesInPatchStrict` returns rename/copy sources) fails the same b-side contract validator |
| RGA-092 | U | Ordinary `100644` add | `change_kind: add`, `content_kind: text`, `object_kind: regular`, `old_mode: ""`, `new_mode: 100644`, `preimage_present: false`, `preimage_sha256: ""` |
| RGA-093 | U | Ordinary `100644` modify | Both presence flags true, exact pre/postimage hashes, stable `effect_sha256` |
| RGA-094 | U | Quoted path with a space in effects | The strict decoded repo-relative path appears in effects, in `touched_paths`, in novelty and in reverse-apply scope |
| RGA-095 | U | CRLF and no-newline-at-EOF | Exact-byte hashes and a stable `effect_sha256` |
| RGA-096 | U | Strict error propagation | An unparseable header surfaces the strict error to every migrated caller in the shape §6.1 assigns it — existing handler for PI-3/PI-4, new refusal for PI-7 |
| RGA-097 | G | Parser totality | Wrong-input fixture `effects-rename-arm-deleted` (one effect arm removed from the normalizer) fails the same effect validator; contradictory, duplicate-destination and escaping paths refuse generation authority |
| RGA-098 | U | Unparseable patch record | A present, non-empty patch the strict grammar refuses yields `patch_present: true`, `patch_sha256` over the exact raw bytes, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported` and `canonical-patch-unparseable`; no effect list is invented and no publication is skipped |
| RGA-099 | G | Unparseable patch silence | Wrong-input fixtures `unparseable-patch-publishes-nothing` and `unparseable-patch-reported-absent` (`patch_present: false` for a patch that exists) each fail the same publication validator |

### 9.3 Capture and reference authority

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-100 | I | Working-tree-all capture | Reference `commit` = resolved HEAD; observations match the patch base |
| RGA-101 | I | Staged-index capture | Reference `commit` = HEAD; HEAD supplies the staged preimage |
| RGA-102 | I | Unstaged with an unrelated staged path | Stage an edit to `other.go`, leave an unstaged edit on `target.go`, record `--unstaged`: reference is `commit` = HEAD, coverage covers `target.go` only, `other.go` appears in no effect |
| RGA-103 | C | Staged∩unstaged overlap on one path | Record refuses pre-capture with the exact `record --unstaged refuses: staged and unstaged edits both touch <paths>` message; no patch, recipe, provenance, generation or coverage artifact is written |
| RGA-104 | I | Committed range | The resolved lower commit supplies the preimage, not the live worktree |
| RGA-105 | C | Worktree differs from the range postimage | Derivation reads the committed postimage, not live bytes |
| RGA-106 | I | Unresolvable commit | Reference `unavailable`; coverage `incomplete` with `reference-not-durable` |
| RGA-107 | I | `reconcile` capture mode | `RefreshAfterAccept` coverage decodes with `capture.mode: reconcile` and `capture.pathspecs` equal to the refreshed path set |
| RGA-108 | U | `index-snapshot` is decode-only | A hand-written coverage file with `reference.kind: index-snapshot` decodes and is non-durable; no shipped producer emits it |
| RGA-109 | U | `no-capture` mode shape | `capture.mode: no-capture` decodes with empty `pathspecs` and `claim_ids`; a `no-capture` record with a non-empty pathspec list is refused |
| RGA-110 | I | P6 capture mode | `implement` coverage decodes with `capture.mode: no-capture` and `reference.kind: unavailable` unless the reconstruct-and-validate path succeeded; its effects, if any, carry `preimage_observed: false` / `postimage_observed: false` with the matching unavailable reasons rather than claiming proven absence |
| RGA-111 | I | P7 capture mode | `tpatch edit` coverage decodes with `capture.mode: no-capture` and the reference kind §6.2 assigns it |
| RGA-112 | G | Capture-mode map | Wrong-input fixture `capture-mode-unstaged-mislabeled-index-snapshot` fails the same reference validator, as does an unmapped capture mode |
| RGA-113 | I | P2 captures before its first bound write | The pre-refresh patch and recipe are both observable in the published record's inputs, taken before `internal/cli/feature_patch.go:114` |
| RGA-114 | I | P3 captures before its first bound write | Observation taken before `internal/workflow/refresh.go:82` |
| RGA-115 | I | P4 captures before its first bound write | Observation taken before `internal/cli/phase2.go:166` |
| RGA-116 | I | P5 captures before its first bound write | Observation taken inside the existing discovery-before-writes window (`internal/cli/cobra.go:999-1039`), before `internal/cli/cobra.go:1044` |
| RGA-117 | C | Partial preimage observation | An unreadable preimage yields `preimage_observed: false`, `preimage_present: false`, `preimage_sha256: ""`, `old_mode: ""` and `preimage-unavailable` on the affected effect; no origin proof is claimed and coverage is `incomplete` |
| RGA-118 | C | Partial postimage observation | An unreadable postimage yields `postimage_observed: false`, `postimage_present: false`, `postimage_sha256: ""`, `new_mode: ""` and `postimage-unavailable` on the affected effect; coverage is `incomplete` |
| RGA-119 | G | Capture generalization | Wrong-input fixture `producer-derives-coverage-from-post-write-bytes` (a producer observing the artifact it already overwrote) fails the same capture-ordering validator |

### 9.4 Coverage schema, patch presence and effect axes

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-120 | U | Strict decode | Unknown, trailing, null and duplicate members refused in both directions |
| RGA-121 | U | Deterministic encoding | The same publication input produces byte-identical coverage bytes |
| RGA-122 | S | Privacy | No timestamp, source body, prompt, provider response or secret in coverage |
| RGA-123 | U | Hash bindings | Exact `patch_sha256`, `recipe_sha256`, `preimage_set_sha256` |
| RGA-124 | U | `patch_present:false` shape | `patch_sha256: ""`, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `canonical-patch-missing` in `reasons`; a non-empty patch hash or a non-empty effect list with `patch_present:false` is refused |
| RGA-125 | U | `patch_present:true` with zero bytes | `patch_sha256` is the digest of the empty byte string, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `canonical-patch-empty` in `reasons`; `patch_sha256: ""` with `patch_present:true` is refused |
| RGA-126 | U | Whitespace-only canonical patch | A patch whose bytes are entirely whitespace is **semantically empty**: `patch_present: true`, `patch_sha256` over the exact raw bytes (not the empty-string digest), `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `canonical-patch-empty` in `reasons`. It is not `canonical-patch-unparseable` and not a new code |
| RGA-127 | U | Clean parse yielding zero normalized effects | A patch the strict grammar accepts but from which it derives **no** effect publishes the identical `canonical-patch-empty` record; emptiness is decided on normalized effects, not on byte length |
| RGA-128 | U | `canonical-patch-unparseable` shape | `patch_present: true` with a real raw-byte `patch_sha256`, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported` and `canonical-patch-unparseable` in `reasons`; a non-empty effect list beside that reason is refused |
| RGA-129 | G | Vacuous completeness | Wrong-input fixture `empty-patch-marked-complete-reference-tree-only` (an empty patch published as `complete` + `reference-tree-only` on the vacuous truth of an empty effect list) fails the same completeness validator |
| RGA-130 | G | Absent-patch silence | Wrong-input fixture `absent-patch-publishes-nothing` (a producer that skips publication because it has no canonical patch) fails the same publication validator |
| RGA-131 | U | `recipe_present:false` shape | `recipe_decodable: false`, `recipe_sha256: ""` and every `operation_indexes` empty; a non-empty hash with `recipe_present:false` is refused |
| RGA-132 | U | `recipe_present:true`, `recipe_decodable:false` shape | `recipe_sha256` is the digest of the exact raw recipe bytes, every `operation_indexes` is empty, `coverage_status` is `incomplete` and `reasons` contains `recipe-undecodable`; `recipe_sha256: ""` in this state is refused |
| RGA-133 | U | `recipe_decodable:true` requires presence | A record with `recipe_decodable: true` and `recipe_present: false` is refused at decode |
| RGA-134 | C | Recipe corruption is distinguishable from deletion | Corrupting `apply-recipe.json` out of band yields a recomputable raw-byte mismatch against the bound `recipe_sha256`, while deleting it yields `recipe_present: false`; the two states are distinguishable at read time, which rev-3's single flag made impossible |
| RGA-135 | G | Recipe decodability conflation | Wrong-input fixture `recipe-present-means-decodes` (the rev-3 single flag, publishing `recipe_sha256: ""` for a present-but-corrupt recipe) fails the same schema validator on the corrupt-recipe fixture |
| RGA-136 | U | Effect descriptor completeness | `change_kind`, `content_kind`, `object_kind`, `old_mode`, `new_mode`, **both observation flags**, both presence flags, both hashes, `patch_fragment_sha256` and `effect_sha256` present on every effect; an absent side carries `""`, never null and never a hash of empty bytes |
| RGA-137 | U | Unobserved side shape | `preimage_observed: false` forces `preimage_present: false`, `preimage_sha256: ""`, `old_mode: ""` and `preimage-unavailable` in `reason_codes`; any other combination is refused. The same holds for `postimage_observed` with `postimage-unavailable` |
| RGA-138 | U | Observed absence versus unobserved | `preimage_observed: true` with `preimage_present: false` decodes as proven absence with no unavailable reason; `preimage_observed: false` with no `preimage-unavailable` reason is refused |
| RGA-139 | U | Observed present side needs hash and mode | `*_observed: true` with `*_present: true` requires a 64-hex hash and a permitted non-empty mode; `""` in either is refused |
| RGA-140 | U | Contradictory observation on an extant side is refused | For a side the `change_kind` requires to exist — the postimage of an `add`, `modify`, `rename` or `copy`, the preimage of a `modify`, `rename`, `copy` or `delete` — a record publishing `*_observed: true` with `*_present: false` is **refused at decode**. A publisher that cannot establish such a side publishes it as **unobserved** with its mandatory `preimage-unavailable` / `postimage-unavailable` reason instead. Proven absence stays valid, and predicate 8 still requires it, for the **non-extant** side of an `add` or a `delete` |
| RGA-141 | G | Impossible observed-absence shape accepted | Wrong-input fixture `extant-side-observed-absent` (an `add` publishing `postimage_observed: true` with `postimage_present: false`, and a `modify` doing the same for its preimage) fails the same observation validator that the unobserved-with-reason record passes; the mirror fixture `unestablished-side-published-as-proven-absent` (a producer encoding "I could not read it" as proven absence on an extant side) fails it too |
| RGA-142 | G | Observation-flag collapse | Wrong-input fixture `presence-flags-only` (the rev-3 pair, encoding an unobserved side as a proven-absent one) fails the same schema validator on the `no-capture` P6 fixture |
| RGA-143 | U | `change_kind` closed enum | Only `add`, `modify`, `delete`, `rename`, `copy` decode |
| RGA-144 | U | `content_kind` closed enum | Only `text`, `binary`, `none`, `unknown` decode |
| RGA-145 | U | `object_kind` closed enum | Only `regular`, `executable`, `symlink`, `gitlink`, `unknown` decode |
| RGA-146 | U | `content_kind: none` iff **known** gitlink | `none` decodes only with `object_kind: gitlink`; a gitlink effect with `text` or `binary` is refused, and `none` beside `object_kind: unknown` is refused — nothing has established it is a gitlink |
| RGA-147 | U | `object_kind: unknown` under no-capture | A P6 effect with neither side observed carries `object_kind: unknown`; the postimage side is used when observed and present, the preimage side when the postimage is **not extant** for the `change_kind` (a `delete`) and the preimage is observed and present, and `unknown` only when an **extant** side went unobserved. There is no observed-absent extant branch: that shape is refused as a contradictory observation |
| RGA-148 | U | `content_kind: unknown` under no-capture | The same effect carries `content_kind: unknown`: `text` requires every **extant** side the effect needs to have been observed, so an unobserved extant side cannot yield `text`, while a stanza binary marker still yields `binary` on its own |
| RGA-149 | U | `unknown` is reasoned, ambiguous and never complete | An `unknown` on either axis requires the matching `preimage-unavailable` / `postimage-unavailable` reason for **at least** the extant side(s) whose absence from the observation set forced it — every unobserved side carries its reason regardless — decodes only with `disposition: ambiguous` — `operation-missing` cannot co-occur, because an effect carrying a mandatory availability reason is not otherwise representable — and is refused at decode beside `coverage_status: complete`; an `unknown` with no unavailable reason is refused, while an unavailable reason beside definite axes is valid |
| RGA-150 | U | No-capture effect cohorts | Pure rename → `object_kind`/`content_kind` `unknown`, both modes `""`, `["effect-rename-unsupported", "postimage-unavailable", "preimage-unavailable"]`; mode-only change → both axes `unknown`, **no** `effect-mode-only-unsupported`; header-declared `100644`→`120000` → both axes `unknown` with no symlink code, since no kind is inferred across an unobserved transition. All three degrade because **both** sides — each of them extant for a `modify`/`rename` — went unobserved |
| RGA-151 | U | Half-observed add classifies from its one extant side | With `postimage_observed: true`, `postimage_present: true` and `preimage_observed: false`, an `add` publishes `object_kind` from the postimage and `content_kind: text` (or `binary` under a stanza marker or an observed NUL) — **never** `unknown` — with `reason_codes: ["preimage-unavailable"]`, `disposition: ambiguous` and `coverage_status: incomplete` because predicate 8 still requires the preimage to be observed-as-absent |
| RGA-152 | U | Half-observed delete classifies from its one extant side | With `preimage_observed: true`, `preimage_present: true` and `postimage_observed: false`, a `delete` publishes both axes from the observed preimage, `reason_codes: ["effect-delete-unsupported", "postimage-unavailable"]`, `disposition: ambiguous` and `coverage_status: incomplete` — failing **predicate 8** for the unobserved postimage independently of predicate 7's `effect-delete-unsupported`, so an implementation that satisfies the row only through the unsupported code fails it |
| RGA-153 | G | An availability reason does not force `unknown` | Wrong-input fixture `unavailable-reason-forces-unknown-axis` (a classifier that sets `content_kind`/`object_kind` to `unknown` whenever any `preimage-unavailable` / `postimage-unavailable` is present, including for a **non-extant** side) fails the same axis validator that the half-observed add and delete records pass; the inverse fixture `unknown-axis-without-unavailable-reason` still fails as before |
| RGA-154 | U | Half-observed digest binding | The half-observed add's `effect_sha256` hashes `content_kind: text`, `object_kind` and `preimage_observed: false` together, so republishing the same effect with `content_kind: unknown`, or with `preimage_observed: true`, does not reproduce the digest; the definite kind cannot be forged onto a record whose extant side was unobserved, and the unobserved flag cannot be forged onto a definite one |
| RGA-155 | G | Inference across an unobserved transition | Wrong-input fixture `object-kind-inferred-from-header` (a classifier taking `object_kind` from the patch's mode header when neither side was observed, e.g. publishing `symlink` for an unread `120000` side) fails the same axis validator that the `unknown` record passes |
| RGA-156 | U | Binary by marker | A stanza carrying `GIT binary patch` or `Binary files ... differ` yields `content_kind: binary` |
| RGA-157 | U | Binary by NUL byte | An observed side containing a NUL yields `content_kind: binary` even with no binary marker in the stanza |
| RGA-158 | U | Symlink target is text | A symlink effect whose target bytes carry no NUL and no binary marker is `content_kind: text` with `object_kind: symlink` |
| RGA-159 | U | Pure rename content kind | An **observed** rename with unchanged content takes `content_kind` from the observed sides, not from the absence of hunks; the same rename with no capture is `unknown` |
| RGA-160 | U | Mode-only change content kind | An **observed** mode-only change takes `content_kind` from the observed sides, which are byte-equal |
| RGA-161 | U | Regular → symlink transition | `old_mode: 100644`, `new_mode: 120000`, `object_kind: symlink` from the postimage side, `reason_codes: [effect-symlink-unsupported]` |
| RGA-162 | U | Regular → gitlink transition | `old_mode: 100644`, `new_mode: 160000`, `object_kind: gitlink`, `content_kind: none`, `reason_codes: [effect-gitlink-unsupported]` |
| RGA-163 | U | Gitlink → regular transition | `old_mode: 160000`, `new_mode: 100644`, `object_kind: regular` from the postimage side, `content_kind` from the observed bytes, `reason_codes: [effect-gitlink-unsupported]` |
| RGA-164 | U | `old_path` follows rename/copy only | `old_path` is non-empty exactly when `change_kind` is `rename` or `copy`, independently of `content_kind` and `object_kind`; any other combination is refused |
| RGA-165 | U | Modes come from the observation | An unchanged-mode modify whose patch carries no mode header still records `old_mode`/`new_mode` `100644` from the reconstructed tree and observed postimage; `""` appears exactly when a side is proven absent **or** unobserved — the side's `*_observed` flag distinguishes the two — and never for a mode header the reader failed to parse |
| RGA-166 | G | Header-only mode derivation | Wrong-input fixture `modes-read-from-headers-only` (modes taken from patch headers alone, so an omitted mode header becomes `""`) fails the same mode validator on the unchanged-mode fixture |
| RGA-167 | U | `patch_fragment_sha256`, first effect | The digest covers exactly the bytes from the effect's `diff --git` line to the byte before the next `diff --git` line, and is recomputable from the canonical patch by offset alone |
| RGA-168 | U | `patch_fragment_sha256`, last effect | The final effect's fragment runs to EOF |
| RGA-169 | U | `patch_fragment_sha256` is verbatim | CRLF line endings and `\ No newline at end of file` markers change the digest; no trimming, re-wrapping or re-encoding is applied |
| RGA-170 | G | Fragment normalization | Wrong-input fixture `patch-fragment-normalized-before-hash` (line endings normalized before hashing) fails the same fragment validator on the CRLF corpus |
| RGA-171 | U | Fragment boundaries are grammar records | A patch whose added content contains a literal line-start-looking `diff --git` string inside a hunk body yields fragments split only at the grammar's recognized record starts, so the effect count and every `patch_fragment_sha256` are unchanged by that content |
| RGA-172 | G | Naive fragment scan | Wrong-input fixture `fragment-boundaries-by-substring-scan` (`strings.Index` over `diff --git`) fails the same fragment validator on that embedded-content corpus |
| RGA-173 | U | Hunk prefixes make the embedded token unreachable as a boundary | In a **valid** unified-diff hunk the embedded line is on the wire as `+diff --git a/x b/y` (or `-`/` `), so its token is not at line start and the grammar never treats it as a record start; a **bare** line-start `diff --git` outside a valid hunk is parsed as a new record, or the patch is refused as `canonical-patch-unparseable`. Both shapes are decided by the grammar, and PI-12's b-side result is byte-identical before and after |
| RGA-174 | U | `effect_sha256` inputs | The digest is computed over exactly ordinal, three kind axes, path, old path, both modes, both observation flags, both presence flags, both content hashes and `patch_fragment_sha256`; a consumer reproduces it from the record plus the canonical patch |
| RGA-175 | G | Hidden digest input | Wrong-input fixture `effect-sha256-hashes-unspecified-hunk-digest` (an implementation feeding an unpublished normalized hunk digest into the effect hash) fails the same recomputation validator |
| RGA-176 | U | `reason_codes` shape | Sorted ascending, non-null, duplicate-free; `[]` exactly when `disposition: represented`, non-empty for every other disposition; and **set-equal** to the full set of effect-local codes whose raising condition holds for that effect |
| RGA-177 | U | `reasons` shape | Required, non-null, sorted ascending, duplicate-free; `[]` exactly when no record-level condition holds; and **set-equal** to the full set of record-level codes whose raising conditions hold |
| RGA-178 | G | Applicable reason omitted | Wrong-input fixture `applicable-reason-omitted` (a binary rename publishing only `effect-rename-unsupported`, and a record publishing `producer-patch-rewrite` without `recipe-not-regenerated`) fails the same exhaustiveness validator that the complete sorted arrays pass; the mirror fixture `inapplicable-reason-present` fails it too |
| RGA-179 | U | `ordinal` is the grammar's record order | `ordinal` is one-based over the strict grammar's recognized record starts in the canonical patch, contiguous with no gaps, independent of the effect array's JSON order and of the recipe; effect *n*'s `patch_fragment_sha256` covers the range from record *n*'s first byte to the byte before record *n+1*'s, or to EOF for the last ordinal, so both are recomputed from one parse |
| RGA-180 | U | Record-level code in an effect | An effect whose `reason_codes` contains `canonical-patch-missing`, `canonical-patch-unparseable`, `recipe-undecodable`, `producer-patch-rewrite`, `simulation-mismatch`, `operation-surplus` or any other record-level code is refused at decode |
| RGA-181 | U | Effect-local code in `reasons` | A record whose `reasons` contains `effect-rename-unsupported`, `operation-missing`, `path-unsafe`, `preimage-unavailable`, `postimage-unavailable` or any other effect-local code is refused at decode |
| RGA-182 | U | No occurrence in both arrays | For every incompleteness cause, the code appears in exactly one array; a record duplicating one occurrence across both is refused |
| RGA-183 | G | Allocation guard | Wrong-input fixture `simulation-mismatch-copied-onto-effects` (a record-level simulation failure also written onto each affected effect) fails the same allocation validator |
| RGA-184 | U | `disposition: represented` iff empty codes | `represented` decodes only with `reason_codes: []`, and `reason_codes: []` decodes only with `represented`; either half alone is refused |
| RGA-185 | U | `disposition: mismatch` mapping | An effect whose `reason_codes` is **exactly** `["operation-missing"]` decodes only as `mismatch`, and `mismatch` decodes only with that exact array; any other array carrying `operation-missing` is refused rather than resolved by precedence |
| RGA-186 | U | `disposition: ambiguous` mapping | An effect with `preimage-unavailable` or `postimage-unavailable` and no `operation-missing` decodes only as `ambiguous` |
| RGA-187 | U | `disposition: unsupported` mapping | An effect whose codes are drawn only from `effect-*-unsupported`, `operation-not-reclassifiable`, `parent-created-target-unsupported` and `path-unsafe` decodes only as `unsupported` |
| RGA-188 | U | Multi-reason precedence | The ladder `mismatch` → `ambiguous` → `unsupported` selects the disposition and drops no code: `postimage-unavailable` + `effect-rename-unsupported` is `ambiguous`; `effect-binary-unsupported` + `effect-rename-unsupported` is `unsupported`. rev-6's `operation-missing` + `preimage-unavailable` example is **not** resolved to `mismatch` — it is refused at decode — and an implementation that resolves it instead of refusing fails the row |
| RGA-189 | G | Disposition contradiction | Wrong-input fixtures `unsupported-beside-operation-missing`, `operation-missing-beside-unavailable-resolved-by-precedence` (a validator that accepts the pair and grades it `mismatch` rather than refusing it) and `disposition-unmapped-to-reasons` (a validator treating `disposition` as an independent editorial value) each fail the same disposition validator |
| RGA-190 | U | `operation-missing` is raised only where an operation was owed | It decodes **only** on an effect that is otherwise representable — repository-safe path, supported change/content/object/mode domain, every side its `change_kind` names observed, no `parent-created-target-unsupported`, no `operation-not-reclassifiable` — and whose `operation_indexes` is empty. On such an effect the array is exactly `["operation-missing"]` with `disposition: mismatch` and coverage `incomplete`; the same effect with an assigned operation decodes with `reason_codes: []` |
| RGA-191 | U | Excluded effects never gain `operation-missing` | With **no** operation assigned to any effect — an absent recipe, a present-but-unreadable one and an undecodable one, each exercised — the binary rename, symlink delete, executable add and gitlink cohorts publish exactly their `effect-*-unsupported` codes and stay `unsupported`, and the no-capture rename, no-capture mode-only, half-observed add and half-observed delete cohorts publish exactly their availability codes and stay `ambiguous`. Only the otherwise-representable effects in the same record carry `operation-missing`, and the record-level cause is `recipe-undecodable` or the presence flags |
| RGA-192 | G | Exact-set validation must not over-attach `operation-missing` | Wrong-input fixture `operation-missing-on-every-unassigned-effect` (the rev-6 unconditional trigger, which set-equality then forces onto every unsupported and ambiguous effect and promotes each to `mismatch`) fails the same reason-exhaustiveness validator that the scoped trigger passes; the mirror fixture `operation-missing-omitted-when-owed` (a representable uncovered effect published as `represented`) fails it too |
| RGA-193 | U | Completeness iff, `reasons` | `coverage_status: complete` with a non-empty `reasons` is refused at decode |
| RGA-194 | U | Completeness iff, at least one effect | `coverage_status: complete` with `effects: []` is refused at decode, in every combination of the other fields; predicate 1 requires at least one normalized effect, so "complete over nothing" has no encoding |
| RGA-195 | U | Completeness iff, no `unknown` axis | `coverage_status: complete` beside an effect carrying `object_kind: unknown` or `content_kind: unknown` is refused at decode, independently of predicate 7's reason check |
| RGA-196 | U | Completeness iff, no failing predicate | `coverage_status: incomplete` with empty `reasons`, every effect `represented`, `patch_present:true`, `recipe_present:true` and a `commit` reference is refused at decode |
| RGA-197 | U | Completeness iff, patch presence | `complete` with `patch_present:false` is refused |
| RGA-198 | U | Completeness iff, recipe presence | `complete` with `recipe_present:false` or `recipe_decodable:false` is refused |
| RGA-199 | U | Completeness iff, observation | `complete` with an effect whose required side is unobserved is refused, and `incomplete` with every required side observed and no other failing predicate is refused |
| RGA-200 | U | Completeness iff, reference durability | `complete` with `reference.kind` other than `commit` is refused |
| RGA-201 | G | Contradictory record | Wrong-input fixture `complete-with-reasons-decodes` (a decoder tolerating `complete` beside a populated `reasons` array) fails the same schema validator |
| RGA-202 | U | Binary rename | `change_kind: rename`, `content_kind: binary`, `object_kind: regular`, `old_path` set, `reason_codes: [effect-binary-unsupported, effect-rename-unsupported]`; no body persisted |
| RGA-203 | U | Symlink delete | `change_kind: delete`, `content_kind: text`, `object_kind: symlink`, `old_mode: 120000`, `new_mode: ""`, `reason_codes: [effect-delete-unsupported, effect-symlink-unsupported]` |
| RGA-204 | U | Executable rename | `change_kind: rename`, `object_kind: executable`, both modes `100755`, `reason_codes: [effect-executable-unsupported, effect-rename-unsupported]` |
| RGA-205 | U | Executable add | `change_kind: add`, `object_kind: executable`, `new_mode: 100755`, `reason_codes: [effect-executable-unsupported]`; no `write-file` emitted; coverage `incomplete` |
| RGA-206 | U | Mode flip `100644`→`100755` with content change | `change_kind: modify`, differing modes, `reason_codes: [effect-executable-unsupported]`; never represented as a plain content write |
| RGA-207 | U | Pure mode change | `change_kind: modify` with equal content hashes and differing modes yields `effect-mode-only-unsupported` plus the out-of-domain mode's own code |
| RGA-208 | U | Gitlink effect | `object_kind: gitlink`, `content_kind: none`, mode `160000`, `reason_codes: [effect-gitlink-unsupported]` |
| RGA-209 | U | Delete of a regular text file | `change_kind: delete`, `reason_codes: [effect-delete-unsupported]`; no lossy substitute operation |
| RGA-210 | U | Copy | `change_kind: copy`, `old_path` set, `reason_codes: [effect-copy-unsupported]` |
| RGA-211 | U | `effect_sha256` sensitivity | The digest changes when any one of ordinal, path, old path, `change_kind`, `content_kind`, `object_kind`, either mode, **either observation flag**, either presence flag, either content hash or `patch_fragment_sha256` changes, and reproduces byte-identically otherwise; in particular an unobserved side and a proven-absent side do not hash alike |
| RGA-212 | U | `contextual_hint` closed enum | Only `none` and `additive-text` decode; `additive-text` changes neither `coverage_status` nor `cross_base_status` nor `disposition` |
| RGA-213 | U | `cross_base_status` closed enum | Only `reference-tree-only`, `consumer-derivation-required`, `unsupported` decode, and each derivation branch is exercised |
| RGA-214 | U | Reason-code closure | Every ADR-036 D3 code — including `canonical-patch-unparseable`, `recipe-undecodable` and `postimage-unavailable` — decodes in its assigned array; an unlisted code is refused |
| RGA-215 | C | Patch tamper | A recomputed patch mismatch makes coverage unusable |
| RGA-216 | C | Recipe tamper | A recomputed recipe mismatch makes coverage unusable |
| RGA-217 | C | Coverage tamper | A changed status, effect axis or reason cannot retain a valid binding |
| RGA-218 | C | Coverage envelope owner mismatch | A coverage record whose `feature` differs from the requested target slug is a **binding failure**: verify rung 2 `block` with `recipe-coverage-owner-mismatch`, exit `2`; `apply --mode execute` takes the order-1 refusal with exit `2`; GH #13 hard-refuses. The record emits **no** schema reason for it, and a fixture that writes one is refused |
| RGA-219 | C | Recipe owner mismatch | A decoded recipe whose `feature` differs from `coverage.feature` fails predicate 3: the record is `coverage_status: incomplete` with record-level `recipe-owner-mismatch` in `reasons`, verify lands on rung **3** with `warn`, exit `0`, reporting it inside the aggregate `recipe-coverage-incomplete` row's sorted reason list, and GH #13 hard-refuses. It never produces a rung-2 block |
| RGA-220 | C | Capture or reference mismatch | Coverage unusable |
| RGA-221 | G | Binding recomputation | Wrong-input fixture `stored-hash-trusting-reader` (a reader trusting stored hashes and the `complete` token) fails the same binding validator on a tampered artifact |
| RGA-222 | G | Orthogonal axis collapse | Wrong-input fixture `single-kind-enum-restored` (the rev-1 `effects[].kind` conflating change, content and object axes) fails the same schema validator on the binary-rename fixture |
| RGA-223 | G | Single reason regression | Wrong-input fixture `scalar-reason-code-restored` (a single `reason_code` string) fails the same schema validator on the executable-rename fixture, which needs two codes |
| RGA-224 | G | PRD/ADR schema parity | Wrong-input fixture `schema-block-field-dropped` (the PRD block with one field removed) fails the same doc-parity validator the byte-identical pair passes |

### 9.5 Simulation and completeness

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-225 | I | Exact simulation | The full operation set reproduces the exact `100644` postimage set from the captured preimage |
| RGA-226 | C | Operation omission on an otherwise-representable effect | A decodable recipe that omits an operation for a regular `100644` effect whose sides are all observed and whose path is safe publishes `reason_codes: ["operation-missing"]` and `disposition: mismatch` on exactly that effect; coverage is `incomplete` |
| RGA-227 | C | Surplus operation | `operation-surplus` at the record level, since a surplus operation belongs to no effect; coverage `incomplete` |
| RGA-228 | C | Duplicate operation assignment | Strict refusal at decode/validate |
| RGA-229 | C | Unmodeled path change | `simulation-mismatch` at the record level with the affected paths named in the diagnostic; coverage `incomplete` |
| RGA-230 | U | Preserved recipe containing `append-file` | Coverage `incomplete` with `operation-not-reclassifiable` on each covered effect; recipe bytes preserved unchanged |
| RGA-231 | U | Preserved recipe containing `replace-in-file` | Same; recipe bytes preserved unchanged |
| RGA-232 | I | `replace-in-file`/`append-file` execution unchanged | Both operations apply byte-identically to pre-GH #15 behavior and round-trip unchanged |
| RGA-233 | I | Preserved richer write-file-only recipe | The exact recipe is simulated and bound rather than replaced by a simpler derivation |
| RGA-234 | U | Completeness needs a durable reference | A record whose reference kind is `unavailable` is `incomplete` with `reference-not-durable`, even when every effect is otherwise representable |
| RGA-235 | U | Completeness needs a decodable recipe | A record with `recipe_present:false`, and a record with `recipe_present:true` + `recipe_decodable:false`, are both `incomplete` even when every effect could have been represented |
| RGA-236 | U | Completeness needs a patch that parses to at least one effect | A semantically empty canonical patch — zero bytes, whitespace-only, or a clean parse yielding zero effects — is `incomplete` with `canonical-patch-empty`, and an unparseable one is `incomplete` with `canonical-patch-unparseable` |
| RGA-237 | U | Canonical predicate parity | The ten-predicate block in §6.5 is byte-identical to ADR-036 D3's, and every `predicate N` cross-reference in both documents resolves to the predicate whose text the surrounding sentence describes |
| RGA-238 | G | Predicate-list divergence | Wrong-input fixture `prd-predicate-list-reworded` (the PRD block restated in its own words, or with one predicate dropped) fails the same doc-parity validator the byte-identical pair passes |
| RGA-239 | G | Stale predicate cross-reference | Wrong-input fixture `predicate-number-points-at-wrong-predicate` (rev-4's "§6.5 predicate 8" cited for operation reclassification, which is predicate 10) fails the same cross-reference validator; the guard checks **every** `predicate N` reference in both documents, not only the block's byte-identity |
| RGA-240 | G | Simulation authority | Wrong-input fixture `fileset-only-comparator` (drift judged by file set, as `compareRecipeFileSets` does today) fails the same completeness validator on a same-file-set byte-different recipe |
| RGA-241 | G | Completeness predicate laxity | Wrong-input fixture `complete-without-durable-reference` (a validator applying only the effect/operation/simulation predicates) fails the same completeness validator on the non-durable, absent-recipe, undecodable-recipe, empty-patch, unparseable-patch and unobserved-side fixtures |

### 9.6 Preimage synthesis, apply classification and the execute refusal

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-242 | I | Existing-file preimage | Exact `sha256:` generated from reference bytes |
| RGA-243 | I | New-file authority | A non-nil empty preimage is generated only for an absent path |
| RGA-244 | I | Exact postimage repeated apply, either preimage authority | A target whose bytes are byte-exactly the operation postimage classifies `already-present` with no write, both for a non-nil **empty** preimage and for a **non-empty expected hash** that no longer matches; bytes unchanged |
| RGA-245 | I | Repeated apply of a creation-only recipe | Empty preimage with the target present and byte-exactly the postimage classifies `already-present`; no write; the second run is byte-stable |
| RGA-246 | C | Empty preimage, target present with other bytes | Collision `drift`, refused on an effective feature. The narrowing is scoped to exact-postimage equality only: any other present value still refuses |
| RGA-247 | C | ADR-029 D3's unchanged refusal cases and unchanged guarantees | An expected-hash operation whose target is **missing** still refuses, and an **unreadable** target still refuses with its read error named — neither is narrowed by exact-postimage recognition. All-or-nothing precheck atomicity, path-safety refusals and ADR-029 D7's supersession severity are byte-compatible with the shipped behavior. A build that reports "the other three cases are unchanged" fails the row, because the expected-hash **mismatch** case is narrowed whenever the observed bytes equal the operation postimage |
| RGA-248 | I | Exact preimage apply | Write succeeds |
| RGA-249 | C | Third state on an effective feature | The entire recipe refuses before any mutation; no operation is written |
| RGA-250 | I | Third state on a superseded feature | Warning-class with the existing `superseded by "<slug>"` note; execution proceeds; default effective replay still excludes the superseded feature |
| RGA-251 | I | Explicit apply of a drifted superseded recipe | The diagnostic states that the warning is an audit signal and does not certify the explicit apply, the coverage or the replay as safe |
| RGA-252 | C | Multi-operation atomic precheck | An earlier applicable operation is not written when a later operation refuses |
| RGA-253 | I | Shared path, dependency order | The child preimage matches and applies |
| RGA-254 | C | Shared path, wrong order or later touch | Refuses with exact expected and observed hashes |
| RGA-255 | C | Path-safety refusal under supersession | Never downgraded; remains an error on a superseded feature |
| RGA-256 | I | Result accounting | A fully already-present recipe increments `Applied` and `Skipped` per operation and prints `Recipe executed: N/N operations succeeded` plus `[write-file] <path>: already present (exact postimage), no write` |
| RGA-257 | G | Three-state classifier | Wrong-input fixtures `classifier-postimage-branch-removed` (preimage checked first) and `classifier-collision-branch-removed` each fail the same apply-classification validator |
| RGA-258 | G | Supersession severity boundary | Wrong-input fixtures `superseded-third-state-hard-refusal` and `superseded-coverage-marked-safe` each fail the same severity validator |
| RGA-259 | C | `created_by` parent-created target | An operation whose correct preimage depends on a parent-created path yields `parent-created-target-unsupported` on that effect; coverage `incomplete` |
| RGA-260 | G | `created_by` boundary | Wrong-input fixture `created-by-empty-preimage-exemption` (a collision exempted because a hard parent is named) fails the same gate validator |
| RGA-261 | G | No parent-postimage carrier | Wrong-input fixture `parent-postimage-reused-as-preimage` fails the same coverage validator, since no field binds parent slug, generation and hash |
| RGA-262 | I | Legacy omitted preimage | ADR-029 D4 warning path and apply behavior byte-compatible |
| RGA-263 | C | Order 2 — coverage binds no readable executable recipe | With valid coverage, `incomplete`, `recipe_present: false`, the file absent and the feature **not** reapplying, apply prints the exact §6.11 order-2 header — stating that coverage **binds no readable executable recipe**, never that generation withheld one — with the recipe path, `recipe status: absent`, sorted reasons, affected paths and feature state, and exits `2` |
| RGA-264 | C | Order 2 — present but unreadable recipe | With valid coverage, `incomplete`, a truthfully recorded `recipe_present: false` and an `apply-recipe.json` that is **present but whose read fails**, apply takes the **same** order-2 refusal and exit `2`, prints `recipe status: unreadable: <the actual read error>`, and never reports the file as missing. It is **not** order 1 (the record and the recomputed readable existence agree), **not** order 3 (nothing was read, so `recipe-undecodable` is absent), and does not reach a default arm or surface a raw read error |
| RGA-265 | U | Readable existence is the recomputed predicate | `patch_present` / `recipe_present` recompute as "exists **and** is readable": the absent and present-but-unreadable cases produce the identical record value and the identical binding result, while each surface's diagnostic names the actual cause. A consumer that distinguishes them at the record layer, or that tolerates unreadable-as-a-third-state, fails the row |
| RGA-266 | C | Readability transition is bidirectional drift | A recipe recorded `recipe_present: true` that becomes unreadable is binding-stale (verify rung 2, apply order 1); a recipe recorded `recipe_present: false` for an unreadable file that later becomes readable is binding-stale in the mirror direction, by the same code, and is not executed on the strength of the record that denied it |
| RGA-267 | C | Unreadable coverage file is present-and-unusable | A `recipe-coverage.json` that exists but whose read fails takes verify rung 1 `block` with `recipe-coverage-malformed`, exit `2`, and `apply --mode execute` order 1 with exit `2`, its diagnostic naming the read error. It does **not** fall through to the legacy orders 5/6 or to rung 5, which are reserved for a genuinely absent coverage file |
| RGA-268 | U | Read failure is injected through the repository seam | Every unreadable-artifact fixture induces the failure through the repository's own file-access seam rather than an OS permission change, so the rows run identically on Linux, macOS and Windows and under a privileged test user; a fixture that depends on `chmod` semantics fails the portability guard |
| RGA-269 | G | Unreadable recipe falls through | Wrong-input fixture `order2-matches-physical-absence-only` (an implementation matching order 2 on "the file does not exist" rather than on readable existence, so a present-but-unreadable recipe reaches a default arm, order 1, or the raw read error) fails the same classifier-totality validator that the readable-existence implementation passes |
| RGA-270 | I | Refusal guidance in `applied` state | An `applied` feature's refusal prints the already-materialized block naming `tpatch verify <slug>` and `tpatch status <slug>`, states that no reapply is required, and discloses that `tpatch implement <slug> --manual` moves the feature to `implementing` |
| RGA-271 | I | Refusal guidance in a non-applied, unmaterialized state | The refusal prints the canonical patch path, `git apply --check ...` before `git apply ...`, and the `implement --manual` alternative with its state transition; the `--check` line precedes the mutating line |
| RGA-272 | G | Unreachable reapply guidance | Wrong-input fixtures `refusal-names-canonical-reapply-path` (the rev-3 text directing the operator to a branch reachable only when `reapplying` is true) and `refusal-claims-operator-chooses-branch` each fail the same guidance-reachability validator, which asserts every named `tpatch` command is runnable from the printed feature state |
| RGA-273 | G | No speculative `feature unapply` | Wrong-input fixture `refusal-recommends-feature-unapply` (the refusal proposing a reverse-apply whose preconditions it has not proved) fails the same guidance validator |
| RGA-274 | U | D17 is reached only when not reapplying | With state `unapplied` or a pending unapplied baseline, `apply --mode execute` takes the canonical reapply branch and returns before `LoadRecipe` (`internal/cli/cobra.go:911-946`), so the refusal never fires there |
| RGA-275 | C | Coverage-binding refusal outranks the withheld branch | Coverage claiming `recipe_present: true` beside an absent `apply-recipe.json` refuses with `recipe-coverage-recipe-changed`, not `recipe-generation-incomplete` and not the legacy no-recipe error |
| RGA-276 | C | Presence-flag mirror, recipe | Coverage claiming `recipe_present: false` (with `recipe_sha256: ""`) beside an `apply-recipe.json` that **is** present and readable is binding-stale: verify lands on rung 2, `apply --mode execute` takes the order-1 refusal with exit `2`, and GH #13 hard-refuses. The recipe is **not** executed on the strength of a record that denies it exists |
| RGA-277 | C | Presence-flag mirror, patch | Coverage claiming `patch_present: false` beside a `post-apply.patch` that **is** present and readable is binding-stale in the same way; when the file is present the raw-byte hash is recomputed too, so a wrong-hash record fails on content and a wrong-presence record fails on presence |
| RGA-278 | U | Exit `2` is validation, not state refusal | The refusal exits `2` (`exitValidation`, `internal/cli/reject.go:46`) with the feature's state unchanged and no state-machine transition attempted; exit `3` is never produced by this path |
| RGA-279 | I | Legacy no-readable-recipe path unchanged | With no coverage and no readable recipe, apply keeps the shipped `LoadRecipe` behavior: the existing `no recipe found — run 'tpatch implement <slug>' first` message for an absent file, exit `1`; the order-2 refusal state remains distinguishable by message and exit code |
| RGA-280 | C | Order 5 — coverage absent, recipe present but unreadable | With **no** coverage on disk and an `apply-recipe.json` that is present but whose read fails, apply follows the **no-readable-recipe** row: the shipped `no recipe found — run 'tpatch implement <slug>' first` error and exit `1`, with no coverage-derived message. It does **not** take order 6, and physical unreadability is never treated as a readable legacy recipe |
| RGA-281 | I | Order 6 — coverage absent, **readable** recipe present | A pre-v0.17.0 feature with a readable recipe and no coverage executes exactly as the shipped build does: no coverage-derived warning, refusal or gate, and byte-identical output |
| RGA-282 | I | Order 4 — incomplete coverage, decodable recipe, executes | With valid `incomplete` coverage and a present decodable recipe, `apply --mode execute` **runs** the recipe through the existing preimage, path-safety and all-or-nothing gates, exits as it would today, and prints a warning naming the sorted reason codes and stating that incomplete coverage is not replay authority. An implementation that refuses here fails the row |
| RGA-283 | C | Order 3 — incomplete coverage, undecodable recipe, refused by name | With valid `incomplete` coverage, `recipe_present: true` and `recipe_decodable: false`, apply prints the §6.11 order-3 header, lists `recipe-undecodable` among the sorted reasons, names the recipe path and exits `2`; the shipped raw JSON decode error must **not** be what the operator sees |
| RGA-284 | U | Execute classifier totality | The seven ordered §6.11 rows are total over the coverage/recipe state space: every constructible shape matches exactly one row, no shape reaches a default arm, and the shapes the schema cannot produce (`complete` with `recipe_present: false`, `recipe_decodable: true` with `recipe_present: false`, `complete` with `effects: []`, `complete` with an `unknown` axis) arrive as strict-decode failures and take order 1 |
| RGA-285 | C | The refusal writes nothing | No progress marker, snapshot, artifact or operation is written on the refused run |
| RGA-286 | G | No silent fallback | Wrong-input fixture `execute-falls-back-to-canonical-reapply` (apply reapplying `post-apply.patch` when the recipe is absent) fails the same apply-mode validator |
| RGA-287 | C | Parent-created and unobserved effects reach the refusal | A withheld derivation caused by `parent-created-target-unsupported` or `postimage-unavailable` prints those codes in the refusal's sorted reason list |
| RGA-288 | G | No new apply mode | Wrong-input fixture `apply-mode-reapply-flag-added` (a `--mode reapply` value added to the shipped `auto`/`prepare`/`started`/`execute`/`done` set) fails the same CLI-surface validator |

### 9.7 Provenance convergence and the origin discriminator

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-289 | U | Derivation purity | Re-deriving the canonical recipe twice from the same immutable capture produces byte-identical bytes; live-worktree bytes and wall-clock never enter the derivation |
| RGA-290 | I | Exact-derived-byte discriminator | A rerun whose freshly derived complete recipe equals the on-disk recipe byte for byte publishes provenance with this run's actual base commit and time, even when prior provenance is absent |
| RGA-291 | C | Near-match manual recipe | A manual recipe differing from the derivation only by key order, indentation or one operation field is not record-generated: bytes are preserved, no provenance is fabricated, coverage makes no origin claim |
| RGA-292 | C | Richer provider recipe | A provider recipe with `replace-in-file` operations is never treated as record-generated; it is preserved and GH #19 is named as the adoption owner |
| RGA-293 | C | Coincidental file-set match | A recipe with the same file set but different bytes is not record-generated; the retired file-set comparison cannot make it so |
| RGA-294 | G | Origin discriminator | Wrong-input fixtures `provenance-trust-by-label-marker` (a stored `generated-by: tpatch` marker used as origin proof) and `origin-by-file-set-equality` each fail the same origin validator that exact byte equality passes |
| RGA-295 | C | Crash after recipe write, provenance never written | The rerun re-derives identical bytes, proves origin, repairs provenance, republishes coverage, and the following verify is green |
| RGA-296 | C | Provenance write failure mid-run | No authoritative coverage is published on the failing run; the next rerun repairs provenance and coverage |
| RGA-297 | C | Stale provenance | A `base_commit` or `recipe_sha256` mismatch is repaired on the next rerun, including a rerun that changes no recipe byte |
| RGA-298 | I | Matching provenance no-op | Bytes and timestamp preserved; no new timestamp manufactured |
| RGA-299 | C | Non-durable reference | No commit-shaped provenance is fabricated for any recipe class; coverage `incomplete` with `reference-not-durable` |
| RGA-300 | I | Fresh record → verify | `recipe_generation_coverage` and V10 both pass |
| RGA-301 | G | Provenance truthfulness boundary | Wrong-input fixtures `historical-adoption-backdated-base` and `manual-recipe-provenance-fabricated` each fail the same provenance validator |
| RGA-302 | G | Origin needs a complete observation | Wrong-input fixture `origin-proved-without-immutable-observation` (byte equality claimed against bytes re-read after the write) fails the same origin validator |

### 9.8 Publication and rerun recovery

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-303 | I | Generated recipe publication order | recipe → provenance → generation → coverage |
| RGA-304 | I | Same patch, regenerated recipe | Coverage refreshes although the patch-generation append is a no-op |
| RGA-305 | I | Recipe no-op | Missing coverage is rebuilt unconditionally |
| RGA-306 | I | No recipe because effects are incomplete | Coverage publishes `recipe_present:false`, `recipe_sha256:""` and all sorted reasons |
| RGA-307 | C | Failure after recipe write | Coverage absent or stale; a rerun repairs it |
| RGA-308 | C | Failure after generation write | Coverage absent; a rerun repairs it without appending a duplicate generation |
| RGA-309 | C | Failure during coverage rename | Prior coverage remains or the file is absent; never partial JSON |
| RGA-310 | G | Publication ordering | Wrong-input fixture `coverage-published-before-recipe` fails the same ordering guard |
| RGA-311 | U | Generation id is advisory | A coverage record whose diagnostics name a patch generation is still accepted or refused purely on recomputed hashes; a matching generation id cannot rescue a stale hash |
| RGA-312 | I | P6 publication order | `implement` publishes recipe → provenance → coverage, with coverage last |
| RGA-313 | G | P6 ordering guard | Wrong-input fixture `implement-publishes-coverage-before-provenance` fails the same ordering guard |

### 9.9 Verify and doctor

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-314 | I | Verify complete coverage | Check ID is exactly `recipe_generation_coverage`, severity `block`, row passes, exit `0` |
| RGA-315 | I | Verify incomplete coverage | Severity `warn`, row fails with every reason code sorted ascending and affected paths, verdict `passed`, exit `0` |
| RGA-316 | I | Verify unparseable-patch coverage | A `canonical-patch-unparseable` record lands on rung 3 with surface code `recipe-coverage-patch-unparseable`, `warn`, verdict `passed`, exit `0`, and GH #13 refuses it |
| RGA-317 | I | Verify undecodable-recipe coverage | A `recipe-undecodable` record lands on rung 3 with surface code `recipe-coverage-recipe-undecodable`, `warn`, verdict `passed`, exit `0`, and GH #13 refuses it |
| RGA-318 | C | Verify malformed coverage | Severity `block`, verdict `failed`, exit `2` |
| RGA-319 | C | Verify hash-, reference-, capture- and coverage-envelope-owner-stale coverage | Severity `block`, verdict `failed`, exit `2` for each of the four states; the fourth is the envelope owner, not the recipe owner |
| RGA-320 | I | Verify missing coverage on a legacy feature | Severity `warn`, verdict `passed`, exit `0`; a repository of historical features does not turn red on upgrade |
| RGA-321 | C | Verify missing coverage after deletion | Deleting coverage this build just produced yields the same `warn` / exit `0` row as the legacy case, and GH #13 refuses the feature as ineligible; deletion buys no replay authority |
| RGA-322 | G | Missing-coverage uniformity | Wrong-input fixture `missing-coverage-split-by-origin` (the retired `missing-produced` / `missing-legacy` split, asserting a nonexistent producer marker in an absent file) fails the same missing-coverage validator |
| RGA-323 | I | Verify pre-v0.17.0 stale-marker fixture | A legacy feature carrying `recipe-stale.json` and no coverage verifies `warn` only, verdict `passed`, exit `0` |
| RGA-324 | I | Verify stale marker beside valid complete coverage | Severity `warn` with surface code `recipe-coverage-stale-marker`, verdict `passed`, exit `0`; consumer authority is unusable and GH #13 refuses |
| RGA-325 | C | Verify stale marker beside malformed coverage | The malformed-coverage `block` row governs: verdict `failed`, exit `2`; the marker does not lower it |
| RGA-326 | I | Verify stale marker beside incomplete coverage | Rung 3 fires, and its sorted remediation still lists the record's `recipe-stale-marker-present` reason; the marker is reported, not hidden by the higher rung |
| RGA-327 | G | Marker suppression | Wrong-input fixture `rung3-remediation-omits-stale-marker-reason` (rung 3 printing every reason except the stale-marker one) fails the same remediation validator |
| RGA-328 | G | Stale-marker precedence | Wrong-input fixtures `stale-marker-hard-fails-verify` (marker presence raised to `block`) and `stale-marker-downgrades-malformed-coverage` each fail the same precedence validator |
| RGA-329 | G | Verify severity boundary | Wrong-input fixtures `legacy-missing-coverage-blocks` and `stale-coverage-downgraded-to-warn` each fail the same severity validator |
| RGA-330 | U | Surface→schema one-way mapping | The mapped subset has exactly **seven** members, each naming exactly one schema condition with no schema condition named twice: `recipe-coverage-stale-marker` → `recipe-stale-marker-present`, `recipe-coverage-patch-missing` → `canonical-patch-missing`, `recipe-coverage-patch-empty` → `canonical-patch-empty`, `recipe-coverage-patch-unparseable` → `canonical-patch-unparseable`, `recipe-coverage-recipe-undecodable` → `recipe-undecodable`, `recipe-coverage-manual-edit` → `manual-bound-artifact-edit`, and `recipe-generation-not-regenerated` → the `producer-patch-rewrite` + `recipe-not-regenerated` pair |
| RGA-331 | U | Unmapped surface set | The unmapped set has exactly **eleven** members — `recipe-coverage-malformed`, `recipe-coverage-patch-changed`, `recipe-coverage-recipe-changed`, `recipe-coverage-reference-stale`, `recipe-coverage-owner-mismatch`, `recipe-coverage-missing`, `recipe-coverage-incomplete`, `recipe-generation-incomplete`, `recipe-generation-provenance-unavailable`, `recipe-generation-origin-unproved`, `recipe-generation-no-truthful-regeneration` — and seven plus eleven exhausts the §7 surface vocabulary with no code in both sets |
| RGA-332 | G | Owner-mismatch dual severity | Wrong-input fixture `owner-mismatch-dual-severity` (a mapping that pairs surface `recipe-coverage-owner-mismatch` with schema `recipe-owner-mismatch`, so one condition is both a rung-2 `block` and a rung-3 `warn`) fails the same mapping-and-severity validator that the separated pair passes; a fixture emitting `recipe-owner-mismatch` for an envelope mismatch fails it too |
| RGA-333 | U | Unmapped schema reasons aggregate | Every effect-local reason, plus `reference-not-durable`, `operation-surplus`, `simulation-mismatch` and `recipe-owner-mismatch`, has **no** surface code and is reported inside the aggregate `recipe-coverage-incomplete` row's sorted remediation |
| RGA-334 | U | Vocabulary disjointness | No token appears in both vocabularies: no verify or doctor row fails with a schema reason token, and no coverage record carries a surface code in `reasons` or `reason_codes` |
| RGA-335 | G | Bijection claim rejected | Wrong-input fixture `surface-schema-bijection-asserted` (a guard requiring a surface code for every schema reason) fails against the **shipped** vocabulary, whose effect-local reasons have none by design, while the one-way mapping guard passes on the same input |
| RGA-336 | G | Dual-name vocabulary | Wrong-input fixture `schema-reason-used-as-surface-code` (a verify row failing with `recipe-stale-marker-present`, or a coverage record carrying `recipe-coverage-stale-marker`) fails the same vocabulary validator |
| RGA-337 | I | V10 independence | `recipe_generation_coverage` receives no supersession downgrade; `write_file_preimage_fresh` keeps its ADR-029 D7 downgrade unchanged |
| RGA-338 | I | Doctor `D10` missing coverage | Read-only warning finding with exact remediation |
| RGA-339 | I | Doctor `D10` stale coverage | The exact mismatch class is reported |
| RGA-340 | I | Doctor `D10` incomplete coverage | Sorted reason codes and affected paths |
| RGA-341 | C | Doctor `D10` unreconstructable capture | No dishonest regeneration command; the finding says plainly that no rerun can recover the authority |
| RGA-342 | I | Remediation after a passing dry derivation | When the dry derivation would yield `coverage_status: complete`, the finding names `tpatch record <slug> --regenerate-recipe`, and running it produces complete coverage |
| RGA-343 | C | Remediation after a failing dry derivation | When the dry derivation would still be incomplete, no regeneration command is printed; the finding names the blocking reasons and directs the operator to manual review |
| RGA-344 | G | Remediation proof | Wrong-input fixture `regeneration-command-printed-without-dry-derivation` fails the same remediation validator on the unsupported-effect and non-durable-reference fixtures, across doctor, verify rung 3 and producer output |
| RGA-345 | G | Doctor `D10` read-only | Wrong-input fixture `d10-fix-writes-coverage` (a `--fix` arm that writes, backs up or locks) fails the same read-only doctor validator; every `D10` finding has `Fixable:false` |

### 9.10 Legacy, consumer boundary and parity

| ID | Kind | Case | Observable |
|---|---|---|---|
| RGA-346 | I | Legacy recipe end to end | Existing warning and apply behavior byte-compatible |
| RGA-347 | G | Legacy boundary | Wrong-input fixture `missing-coverage-treated-complete` fails the same coverage validator |
| RGA-348 | I | GH #13 strict consumer stub | Complete coverage is rechecked, not trusted |
| RGA-349 | C | GH #13 sees missing, malformed or incomplete coverage | Each of the three is an independent hard refusal with the exact reason, regardless of verify's warn-class severity |
| RGA-350 | C | GH #13 sees an unsupported effect | Ineligible, with the exact producer reason codes |
| RGA-351 | C | GH #13 sees an absent-, empty- or unparseable-patch record | All three are hard refusals; `canonical-patch-missing`, `canonical-patch-empty` and `canonical-patch-unparseable` are never read as "nothing to check, therefore safe" |
| RGA-352 | C | GH #13 sees an undecodable recipe | `recipe-undecodable` is a hard refusal; the bound raw-byte hash identifies which bytes are on disk and never implies they are usable |
| RGA-353 | G | Consumer independence | Wrong-input fixture `coverage-status-flipped-to-complete` cannot authorize replay through the same consumer validator |
| RGA-354 | G | Warnings never authorize replay | Wrong-input fixture `warn-class-read-as-eligible` (a consumer treating verify's `warn`/exit `0` on missing or stale-marker state as eligibility) fails the same consumer validator |
| RGA-355 | I | Adjacent-args fixture | `coverage_status: complete`, `cross_base_status: consumer-derivation-required`, `contextual_hint: additive-text` on the additive effect; the whole-file write is never reported as cross-base safe |
| RGA-356 | G | Cross-base overclaim | Wrong-input fixture `adjacent-fixture-marked-reference-tree-only` fails the same `cross_base_status` derivation validator |
| RGA-357 | G | No persisted anchors | Wrong-input fixture `persisted-anchor-field-added` (a new operation field or type) fails the same schema/parity guard; legacy replace semantics remain unchanged |
| RGA-358 | S | Diagnostics | Paths, hashes, ordinals and reason codes only; no source bytes |
| RGA-359 | G | Docs and assets parity | Wrong-input fixture `skill-line-claims-replay-safe` fails the same overclaim guard; every public surface says coverage is necessary, not sufficient, and not cross-base safety |
| RGA-360 | I | Downstream soak | A realistic cumulative repository records and verifies successfully across all seven producers; the legacy cohort stays green; the unsupported cohort is explicit |

**Matrix size**: 360 rows: I 77, C 83, G 81, U 117, S 2.

Every `G` row above names the semantically wrong input its guard must reject,
and that input is fed to the same validator the correct input passes. Counts
are recomputed from the row list, and IDs are contiguous from `RGA-001` to
`RGA-360` with no gaps. rev-7, like every revision before it, renumbers the
whole matrix rather than inserting sub-numbered rows, so no earlier identifier
carries a different meaning at the same number. rev-7 keeps the rev-6 row set
intact — rewriting eleven rows in place (`RGA-012`, `RGA-147`, `RGA-149`,
`RGA-185`, `RGA-188`, `RGA-189`, `RGA-226`, `RGA-244`, `RGA-246`, `RGA-279`
and `RGA-281`) — and adds **9**: the generic table-driven
per-producer publication-failure row in §9.1; PI-10's four-consumer inventory
in §9.2; the contradictory-observation row and its guard, and the
`operation-missing` positive, negative and exhaustiveness-guard rows in §9.4;
the ADR-029 unchanged-refusal-cases row and the coverage-absent
unreadable-recipe row in §9.6.

## 10. Rollout and release

1. Land S0-S2 behind no consumer authority.
2. Land S3-S5 and run the full producer/apply/verify migration suite across
   all seven producers.
3. Run a downstream soak over existing legacy, new generated and unsupported
   effect cohorts, including a pre-v0.17.0 legacy repository that must remain
   verify-green with and without a `recipe-stale.json` marker.
4. Review findings before enabling any GH #13 consumer.
5. Ship GH #15 as v0.17.0.
6. Plan/implement GH #13 against the released schema and behavior, then ship it
   as v0.18.0, including regeneration of recipe and coverage for the features
   P3/P4/P5 left explicitly incomplete and the features P6/P7 published with
   `canonical-patch-missing` or `manual-bound-artifact-edit`.

The releases stay separate. No v0.17.0 release includes automatic reconcile
replay or any GH #13 consumer.

## 11. Deferred decisions and review triggers

Rev-0's four open questions were answered in rev-1 and remain closed:

| Rev-0 question | Answer |
|---|---|
| Should `index-snapshot` coverage mint a durable tree object? | No. It stays incomplete, and no shipped record mode emits `index-snapshot` at all (§6.2) |
| Is contextual classification a field or a reason? | A field: `effects[].contextual_hint`, closed to `none`/`additive-text`, explicitly non-authoritative (§6.4). There is no remaining placement question |
| Which doctor check ID? | `D10`, the next free ID after the shipped `D1`-`D9` registry (§6.13) |
| Suppress a partial recipe or preserve a legacy one? | Suppress publication of a new partial recipe; preserve an existing recipe unchanged (§6.3) |

Rev-1's six findings are answered in rev-2 and are likewise closed:

| Rev-1 finding | Rev-2 answer |
|---|---|
| Coverage omitted `feature patch refresh\|fixup`, reconcile accept and cycle patch writers | Five governed producers P1-P5, one shared publication API, three permitted terminal states, and the rule that no writer may leave formerly complete coverage stale (§6.15) |
| The noop path could not distinguish record-derived bytes from a manual/provider recipe | Exact derived-recipe byte equality recomputed on every rerun, with a pure derivation, a canonical encoder and a total comparison; no trust-by-label marker; near-match refused (§6.16) |
| Missing-produced versus legacy coverage had no durable carrier, so deletion downgraded failures | Uniform warning-class `recipe-coverage-missing`; the split is retired, deletion's severity reduction is acknowledged, and absence is unconditionally consumer-ineligible (§6.12) |
| Legacy `recipe-stale.json` became an unintended hard failure | Warning-class plus consumer ineligibility, with the precedence ladder pinned in both directions and a pre-v0.17.0 fixture that stays green (§6.12) |
| The one-parser claim omitted novelty/hunk/unapply consumers | The rule is rescoped to path/effect authority over the complete derived parser inventory, with registered adapters and registered non-authoritative scanners (§6.1, §8 S1) |
| One effect `kind` conflated change, binary and object modes | Orthogonal `change_kind` / `content_kind` / `object_kind` axes and sorted `reason_codes[]` (§6.4) |

Rev-2's six blocking findings and eleven minor findings are answered in rev-3
and are likewise closed:

| Rev-2 finding | Rev-3 answer |
|---|---|
| C1 — the P1-P5 registry omitted the production `implement` recipe writer and misstated composite `cycle` | Seven governed producers P1-P7, with `implement` (provider, heuristic and `--manual` checkpoint) and `artifact-edit` added, `cycle` restated as a composite whose implement step is P6, `apply --mode done` scoped to runs that actually write the patch, and direct filesystem edits named as ungovernable external tamper detected at read time (§2.7, §6.15) |
| C2 — top-level `reasons[]` had no allocation or completeness rules | An exact record-level/effect-local allocation table, no occurrence in both arrays, and an iff `coverage_status` predicate that no contradictory record can decode (§6.4, §6.5) |
| C3 — `producer-patch-rewrite` was both unconditional and conditional | Redefined conditionally: raised only when the rewrite leaves the recipe unable to cover and simulate the new patch, always paired with `recipe-not-regenerated`, and absent from a rewrite whose recipe still validates (§6.4, §6.15) |
| C4 — modes, content kind and the hidden hunk digest were not recomputable | Modes and object kind read from the immutable observation; `content_kind` extended to `text`/`binary`/`none` with an explicit decision rule and pinned transition cases; the undefined hunk digest replaced by `patch_fragment_sha256`, a byte range of the canonical patch (§6.4) |
| C5 — suppressing a partial recipe left execute-mode apply unspecified | *(rev-3 answer, since superseded)* `apply --mode execute` refuses with the named `recipe-generation-incomplete` diagnostic and exit `2`, listing sorted reasons and directing to the canonical reapply path or `implement --manual`; the legacy no-recipe error and exit `1` are unchanged; no silent fallback. **rev-4 deleted the canonical-reapply direction as unreachable (see the rev-3 B2 row below) and rev-5 replaced the single refusal with the total classifier; the current contract is §6.11 / ADR-036 D17** |
| C6 — the PI-7 migration would have lost both-side rename/copy scope | PI-7's source re-characterized correctly, and migration retargeted to a new `PathsAffectedByPatchStrict` all-paths API with the same rollback scope, fail-closed at all three call sites, with no falsely claimed fail-soft handler (§2.5, §6.1.1) |
| M1 — dual names for the stale-marker condition | Two vocabularies with one explicit mapping: surface `recipe-coverage-stale-marker` ↔ schema `recipe-stale-marker-present` (§6.12, §7) |
| M2 — completeness omitted reference durability and artifact presence | Artifact presence and reference durability became predicates **1-3** of the canonical list in §6.5 / ADR-036 D3, summarized in D4 as the artifact-reality and reference-durability dimensions |
| M3 — PI-10 fed a recipe operation count | `internal/cli/cobra.go:1908` prints the derived recipe's actual operation count; PI-10 stays a human file count with a guard (§6.1.3). *(rev-5 moved the incomplete-coverage status to the common post-autogen output path, since a withheld derivation never enters the `AutogenGenerated` arm; rev-7 records that PI-10 has **four** consumers and that `cobra.go:1908` is the only one migrated.)* |
| M4 — event boundaries and early returns were unspecified | A governed producer event is a successful bound write or manual checkpoint; publication is per event, with the P4 composite, P2 checkpoint, empty-capture, reapply-branch and non-bound-artifact cases pinned, and the ignored `phase2.go:166` write error propagated (§6.15) |
| M5 — immutable capture was specified only for `record` | Generalized to every producer, with P7's before/after snapshots and the reconstruct-and-validate rule, and no origin proof without a complete observation (§6.2) |
| M6 — `tpatch edit` was ungoverned | Covered by P7 (§6.15) |
| M7 — P2's byte-identical path could skip repair | A same-bytes refresh is a checkpoint event that still republishes coverage; the changed-patch exact-equality corollary is stated (§6.15) |
| M8 — an empty canonical patch had no encoding | `canonical-patch-empty`, `effects: []`, `cross_base_status: unsupported` (§6.3, §6.4) |
| M9 — rung 3 could hide a present stale marker | Rung 3 wins but its sorted remediation still prints the record's `recipe-stale-marker-present` reason (§6.12) |
| M10 — regeneration remediation could name a command that cannot succeed | A dry derivation must prove `complete` before any regeneration command is printed; otherwise the surface says no truthful automatic regeneration exists (§6.11, §6.13) |
| M11 — existing `FilesInPatchStrict` callers were unpinned | PI-12 registers `land.go:767,1212`, `refresh.go:59` and `verify_landed.go:1009,1163` as b-side consumers with an unchanged contract and a regression guard (§6.1.2) |

Rev-3's six blockers, five must-fix findings and its non-blocking
contradictions are answered in rev-4 and are likewise closed:

| Rev-3 finding | Rev-4 answer |
|---|---|
| B1 — P2's same-byte checkpoint contradicted the event definition, since that branch performs no write | Governed events have **three** categories: bound write, manual checkpoint, and an explicitly contracted re-observation checkpoint. P2's non-empty `Append == false` branch is category (c): it publishes coverage before returning, keeps its shipped `no patch byte change; refresh\|fixup skipped` meaning plus a coverage status line, and changes disk only by repairing coverage. P2's empty-capture early return is explicitly **not** a checkpoint. The event-boundary wrong-input fixture is rebuilt from genuinely no-event commands (§6.15, `RGA-052`, `RGA-053`) |
| B2 — D17 recommended a reapply branch unreachable when D17 fires | The refusal is reached only when `reapplying` is false, and says so. The unreachable instruction and the false "the operator chooses it" claim are deleted; `feature unapply` is never recommended unproven. `applied` features are told no reapply is required and pointed at `tpatch verify` / `tpatch status`; other states get the reviewed canonical patch path with `git apply --check` before `git apply`; `implement --manual` is offered with its `implementing` transition disclosed. A coverage-binding refusal (`recipe-coverage-recipe-changed`) precedes the withheld-recipe branch, and exit `2` is justified as pre-mutation executable-plan validation with no state transition (§6.11) |
| B3 — the schema could not encode a present unparseable patch | New record-level `canonical-patch-unparseable`: `patch_present: true`, raw-byte `patch_sha256` bound, `effects: []`, `cross_base_status: unsupported`, `incomplete`. Surface code `recipe-coverage-patch-unparseable`. A strict parse refusal never leaves a feature without coverage after an event (§6.3, §6.4, §7) |
| B4 — the schema could not encode a present undecodable recipe | Required `recipe_decodable`, with `recipe_sha256` always binding the exact raw bytes when the file is present, and new record-level `recipe-undecodable`. `recipe_decodable: true` beside `recipe_present: false` is refused. *(rev-4's added claim that P6's `internal/workflow/implement.go:194` arm publishes "before returning its parse error" was wrong and was corrected in rev-5's B3 row; rev-6 further corrects the return itself to `s.MarkFeatureState` at `:243`.)* (§6.3, §6.4, §6.15) |
| B5 — an existing but unobserved image side was indistinguishable from a proven-absent one | Required `preimage_observed` / `postimage_observed` beside the presence flags, new effect-local `postimage-unavailable`, both flags folded into `effect_sha256`, and completeness predicate 8 requiring every needed side to be observed (§6.4, §6.5) |
| B6 — effect `disposition` had no reason-code mapping | An exact mapping — `represented` iff empty, `mismatch` on `operation-missing`, `ambiguous` on an unavailable side, `unsupported` otherwise — with `mismatch` → `ambiguous` → `unsupported` precedence for multi-reason effects, strictly validated in both directions (§6.4). *(rev-7 tightens the `mismatch` row to the exact singleton `["operation-missing"]` and refuses any co-occurrence rather than resolving it by precedence — see the rev-6 B1 row below.)* |
| M-a — the PRD and ADR carried divergent completeness predicates | One canonical **ten-predicate** list, owned by ADR-036 D3 and reproduced byte-identically in §6.5 under a parity guard; every cross-reference uses those numbers (§6.5) |
| M-b — the surface/schema bijection is impossible | Replaced by a one-way partial mapping over the mapped subset plus vocabulary disjointness; effect-local reasons have no individual surface code and aggregate under `recipe-coverage-incomplete` (§6.12, §7) |
| M-c — the seven-writer count is unimplementable | Registry count (seven) and site-to-producer mapping are separate guards. The mapping covers the **eight** direct bound `WriteArtifact` sites plus checkpoints and the editor delegation; the shared autogen helper maps by caller; the phrase "eighth writer" is gone (§6.15) |
| M-d — see B4 | Answered with B4 |
| M-e — P7 triggered on the user's artifact token | P7 triggers on the **resolved** path. A feature-root decoy resolves first and is non-bound; `artifacts/<name>` resolves canonical. Both spellings and the precedence are pinned (§6.15) |
| `openInEditor` described as synchronous without qualification | It is synchronous only as far as the configured process, and it discards `c.Run`'s error today. The refactor returns and propagates that error; P7 snapshots and publishes changed bytes even on editor error; a GUI editor that exits before a later save produces no observable mutation, and that later save is external tamper caught at read time (§6.2, §6.15) |
| `CapturePatchScoped` cited at `internal/cli/feature_patch.go:158` | Corrected to `internal/cli/feature_patch.go:88` (§6.2, §12) |
| `tpatch land`'s embedded record was unnamed | Named as P1 orchestration, not a new producer (§2.7, §6.15) |
| `patch_fragment_sha256` boundaries invited a substring scan | Boundaries are the strict grammar's recognized line-start `diff --git` record starts; an embedded literal in a hunk body does not open a fragment (§6.4) |
| `implement --manual`'s state transition was undisclosed | Disclosed wherever it is suggested, including the §6.11 refusal (§6.11, §6.15) |
| D17 did not pre-validate coverage binding | It does: malformed, stale or missing-bound-recipe coverage takes its own refusal first, and legacy absent coverage keeps existing behavior (§6.11). *(rev-7 states the legacy rows over readable existence: absent coverage with no **readable** recipe keeps the shipped `LoadRecipe` error and exit `1`, and absent coverage with a readable recipe keeps the shipped execution.)* |

Rev-4's seven blockers and seven optional cleanups are answered in rev-5 and
are likewise closed:

| Rev-4 finding | Rev-5 answer |
|---|---|
| B1 — effect axes had no encoding for a side no producer observed, so a no-capture record had to name a kind it never saw | `unknown` added to **both** `object_kind` and `content_kind`, requiring the matching unavailable reason, yielding `ambiguous`, refused beside `complete`, keeping unobserved modes `""`, and hashed into `effect_sha256` as published; `content_kind` is `none` only for a **known** gitlink and `binary` on a stanza marker or an observed NUL. *(rev-5's selection rules also degraded an effect whose one **extant** side had been observed; rev-6 scopes both axes to the extant sides and pins the half-observed add and delete cohorts — see the rev-5 B1 row below.)* (§6.3, §6.4, §9.4) |
| B2 — "empty" was defined as zero bytes, leaving whitespace-only and zero-effect parses to satisfy completeness vacuously | **Semantic emptiness** = zero bytes, whitespace-only bytes, or any strict parse yielding zero normalized effects. All three keep `canonical-patch-empty`, bind the raw patch hash whenever the file is present, and publish `effects: []`, `incomplete`, `cross_base_status: unsupported`. Predicate 1 now requires **at least one** normalized effect, and the decoder refuses `complete` beside `effects: []` (§6.3, §6.5, §9.4) |
| B3 — the P6 unmarshal-failure arm was described as returning a parse error, which the source does not do | Corrected throughout. `internal/workflow/implement.go:192-195` writes the raw response; the unmarshal error is consumed by the `if`, the only `return` is the write error, and a **successful** write falls through to the provenance attempt and to the function's final return. That successful raw write is the governed event, and it publishes `recipe_present: true`, `recipe_decodable: false`, the raw hash and `recipe-undecodable` at the common coverage-last point. *(rev-5 called that final return a literal `nil`; rev-6 corrects it to `return s.MarkFeatureState(...)` at `:243` and anchors the finalizer to the return point.)* A failed `WriteArtifact` is **no** event and owes nothing (§2.7, §6.15, §9.1) |
| B4 — `apply --mode execute` was specified for four shapes and undefined for the rest | A **total** seven-case ordered classifier (§6.11): bidirectional binding refusal; the withheld-recipe refusal, rephrased as *coverage binds no executable recipe*; a present-but-undecodable recipe refused by the same named contract listing `recipe-undecodable` instead of a raw JSON error; a present-and-decodable recipe under incomplete coverage that **executes** backward-compatibly with a non-authority warning; the two legacy shapes unchanged; and complete coverage. Every other shape is a strict-decode failure and takes order 1. *(rev-5 gated order 2 on the file being "genuinely absent", which left a present-but-unreadable recipe unmatched; rev-6 restates it over readable existence and adds "readable" to the refusal phrase — see the rev-5 B2 row below.)* |
| B5 — presence binding was recomputed in one direction only | `patch_present` and `recipe_present` are recomputed from actual on-disk readable existence in **both** directions: `true`-beside-absent and `false`-beside-present are equally binding-stale, with the raw hash also recomputed whenever the file is present. Order 1 covers both, and mirror rows are pinned (§6.11, §6.14, §9.6) |
| B6 — `§6.5 predicate 8` was cited for operation reclassification, which is predicate 10 | Retargeted, and the parity guard widened from block byte-identity to validating **every** `predicate N` reference in both documents (§6.3, §6.5, §9.5) |
| B7 — the surface-mapping totality sentence read as covering the whole surface vocabulary | Scoped to the explicitly mapped subset. The remaining surface codes are named as verify-, binding- or aggregate-level conditions with no schema-reason counterpart, unmapped by construction, while disjointness continues to hold over the whole of both vocabularies. *(rev-5 left `recipe-coverage-owner-mismatch` inside the mapped subset, which gave one condition two severities; rev-6 removes that pairing, leaving seven mapped and eleven unmapped — see the rev-5 B3 row below.)* (§6.12, §7) |
| O1 — `openInEditor` was said to have three call sites | It has **two**: `internal/cli/c1.go:91` (the only one that can resolve to a bound artifact) and `internal/cli/phase2.go:88` (`spec.md`, never a P7 event) (§2.7, §6.2, §9.1) |
| O2 — the mapping demanded that each *site* map to one producer, which the shared autogen helper cannot satisfy | The unit is the **reachable call chain**: `recipe_autogen.go:204` contributes two chains mapping to P1 and P2 by caller, and has no `producer` value of its own (§6.15, §9.1) |
| O3 — P2's same-byte branch was described as matching "a generation already on record" | It compares the **latest** generation only (`internal/store/patch_generation_kinds.go:46-49`); an older-generation match still appends and takes the writing path (§6.15, §9.1) |
| O4 — the D17 trigger language implied a producer decision | Already addressed by the neutral trigger phrasing, now carried through the header text itself: coverage binds no executable recipe. *(rev-6 widens that header to "no **readable** executable recipe" so the unreadable case is covered by the same words.)* (§6.11) |
| O5 — an unset `$EDITOR` was unmodeled | Modeled: no process starts, so no byte can change and **no P7 event occurs** (§6.2, §6.15, §9.1) |
| O6 — the incomplete status was said to print from `cobra.go:1908` | The operation count comes from the derived recipe inside its actual `case workflow.AutogenGenerated` arm, which a withheld derivation never enters; the incomplete status is emitted from the common post-autogen coverage output path (§6.1.3, §6.11) |
| O7 — the embedded `diff --git` hazard was warned about but not closed | Closed: inside a valid hunk every body line carries a `+`, `-` or space prefix, so the token is never at line start; a bare token outside a valid hunk is parsed as a new record or refused as `canonical-patch-unparseable`. Grammar recognition wins, and PI-12's semantics are fixed (§6.1.2, §6.4, §9.4) |

Rev-5's three blockers and its optional accuracy findings are answered in
rev-6 and are likewise closed:

| Rev-5 finding | Rev-6 answer |
|---|---|
| B1 — the half-observed add published `content_kind: unknown` although its only **extant** side, the postimage, had been observed | Both classification rules are scoped to the effect's **extant** sides. An `add` whose postimage is observed and present takes `object_kind` and `content_kind` from it — `text`, or `binary` on a marker or NUL — and a `delete` does the same from its observed preimage; `unknown` is reached only when an extant side went unobserved with no decisive binary marker, which for `modify` / `rename` / `copy` still means either side. The unobserved non-extant side keeps its mandatory unavailable reason, the `ambiguous` disposition and predicate-8 incompleteness, and the ADR cohort table, the §6.3 cohorts, the rev-5 `RGA-140` / `RGA-142` equivalents (`RGA-148` and `RGA-150` in rev-7's numbering), the new cohort rows `RGA-151` / `RGA-152` and the digest expectation `RGA-154` are aligned. Unknown-forcing reasons now explicitly track the extant side(s) that prevented classification, while availability reasons may exist for a non-extant side without forcing `unknown` (§6.3, §6.4, §9.4, ADR-036 D3/D5) |
| B2 — a present-but-unreadable recipe matched no D17 row | Order 2 is stated over **readable existence**: it fires whenever valid `incomplete` coverage carries `recipe_present: false` and no readable recipe is on disk — absent **or** present-and-unreadable — under the phrase *coverage binds no readable executable recipe*, with a `recipe status` line carrying the actual read cause. Order 1 fires only where recomputed readable existence **differs** from the stored flag, so a truthful `false`-beside-unreadable record takes order 2 and a later readability change takes order 1 from either direction. The legacy no-coverage orders 5 and 6 are untouched, and the injected read-failure fixtures use the repository's file-access seam rather than an OS `chmod` (§6.11, §7, §9.6, ADR-036 D9/D17) |
| B3 — `recipe-owner-mismatch` was simultaneously a warning-class schema reason and a blocking rung-2 surface mapping | The two conditions are separated. A **coverage envelope** owner mismatch (`coverage.feature` ≠ the requested target slug) is a D9 binding failure: surface `recipe-coverage-owner-mismatch`, rung 2, `block`, exit `2`, apply order 1, and **no** schema reason — it joins the unmapped binding-level codes. A **recipe** owner mismatch (`recipe.feature` ≠ `coverage.feature`) is the record-level schema reason `recipe-owner-mismatch`, fails predicate 3, makes valid coverage `incomplete`, and surfaces only through the aggregate `recipe-coverage-incomplete` row at rung 3 with `warn` / exit `0`. rev-5's direct surface→schema pairing is removed, leaving **seven** mapped codes and **eleven** unmapped, and both conditions gain rows with no dual severity (§6.12, §6.14, §7, §9.4, §9.9, ADR-036 D9/D13) |
| O1 — readable existence collapsed two physical states without saying so | Stated explicitly: the record layer intentionally collapses physical absence and unreadability into one presence value, because both mean "no usable artifact", while every read-error diagnostic keeps the actual cause. A later readability transition is binding drift in whichever direction it occurred, for the patch exactly as for the recipe (§6.4, §6.11, §6.14, ADR-036 D3/D9) |
| O2 — a §11 row claimed predicates 1-4 added artifact presence and reference durability | Corrected to predicates **1-3**, so every `predicate N` semantic reference in either document passes the widened parity guard (§11, §6.5) |
| O3 — the effect ordinal was used but never defined | Defined as the **one-based strict-grammar record order** of the canonical patch, contiguous and independent of the JSON array order, with `patch_fragment_sha256` boundaries following that same order (§6.4, §9.4, ADR-036 D3) |
| O4 — P6's common finalization point was described against a literal `nil` return | `RunImplement`'s final statement is `return s.MarkFeatureState(slug, store.StateImplementing, ...)` (`internal/workflow/implement.go:243`). The coverage finalizer runs after the recipe write, the provenance attempt and the state-mark attempt, and before that return is propagated; a failing state mark does not cancel the obligation, because the recipe write it binds already landed. Error precedence is explicit: the state-mark error is still returned, a coverage-publication failure is surfaced rather than dropped, and when both fail the returned error names both, state-mark first (§2.7, §6.15, §9.1, §12) |
| O5 — the P3 generation append was described as unconditional | Corrected: the patch write at `internal/workflow/refresh.go:82` is unconditional, while `AppendPatchGenerationForFeature` runs only when `newPatch != originalPatch` (`:93,102`). The P3 event is the write, so an accept whose regenerated bytes are unchanged still owes coverage (§2.7, §6.15, §9.1, §12) |
| O6 — `reasons` / `reason_codes` were specified with an "at minimum" floor | Both arrays must equal **exactly** the applicable closed codes, sorted and deduplicated; no applicable code is optional and none may be invented. Validation and the matrix check set equality, with a guard on both the omission and the surplus direction (§6.4, §6.15, §9.4) |
| O7 — the fix to `docs/handoff/CURRENT.md` | Deliberately not made here. rev-6 edits only this PRD, ADR-036 and the ADR index; tracking is updated separately |

Rev-6's single blocker and the accuracy findings raised with it are answered in
rev-7 and are likewise closed:

| Rev-6 finding | Rev-7 answer |
|---|---|
| B1 — exact-set validation made `operation-missing` applicable to every intentionally unsupported effect, because its trigger was "a normalized effect has no assigned operation" | The trigger is scoped to effects for which an operation was **owed**. `operation-missing` is raised **iff** the effect is otherwise representable in v1 — repository-safe path, supported change/content/object/mode domain, every side its `change_kind` names observed, no `parent-created-target-unsupported`, no `operation-not-reclassifiable` — **and** the decoded recipe assigns it no operation; equivalently, no other effect-local condition holds and `operation_indexes` is empty. It is never raised because v1 deliberately emitted no operation for an excluded effect, and a missing, unreadable or undecodable recipe raises it on the otherwise-representable effects only. The closed-list trigger, the allocation rationale, the exact-set semantics, the disposition table (`mismatch` iff the exact singleton), the precedence examples (rev-6's `operation-missing` + `preimage-unavailable` case is now a refusal, not a resolution), the §6.3 generation policy, the §6.4 wire-format notes, the ADR and PRD cohort tables and the matrix rows `RGA-185`, `RGA-188`, `RGA-189`, `RGA-190`, `RGA-191`, `RGA-192`, `RGA-226` are all aligned, with the positive missing case and the negative unsupported / ambiguous cases pinned together (§6.3, §6.4, §9.4, §9.5, ADR-036 D3/D5) |
| A1 — the ADR-029 restatement claimed "all other three cases unchanged" | Corrected. Exact-postimage recognition narrows **two** of ADR-029 D3's four refusal cases — the empty-preimage collision and the non-empty expected-hash mismatch — in each case only when the observed bytes equal the operation's postimage, which is what this PRD's own classification table already did. The missing-target expected-hash refusal, the unreadable-target refusal, all-or-nothing precheck atomicity, path safety and ADR-029 D7 supersession severity are unchanged, and the rows and matrix entries (`RGA-244`, `RGA-246`, `RGA-247`) say so (§6.7, §9.6, ADR-036 D7) |
| A2 — P2's specific policy implied `recipe-not-regenerated` alone | Whenever the conditional `producer-patch-rewrite` applies, `recipe-not-regenerated` is **paired** with it; P2's patch-writing run therefore publishes both, plus every other applicable code, under exact exhaustive-set semantics. P2's non-writing category-(c) checkpoint raises neither rewrite code and carries the codes whose conditions genuinely hold (§6.11, §6.15, `RGA-012`, ADR-036 D3/D15) |
| A3 — coverage-publication failure had no general contract | One contract over P1-P7: a failed publication is returned and the command exits **non-zero**; it may leave coverage absent or stale on disk (publication is last, and the next read rejects both) but never yields command success. Pinned by a single **table-driven** matrix row over every producer event (`RGA-057`) rather than seven prose rows (§6.10, §6.15, ADR-036 D10) |
| A4 — the legacy `apply --mode execute` rows did not use readable existence | Orders 5 and 6 split on readable existence like the rest of the classifier: coverage absent with **no readable** recipe keeps the existing `LoadRecipe` read / no-recipe error and exit `1`; coverage absent with a **readable** recipe keeps the existing execution; a physically unreadable recipe follows the no-readable row (§6.11, `RGA-279`, `RGA-280`, `RGA-281`, ADR-036 D17) |
| A5 — PI-10 was described as having two consumers | It has **four**: `internal/cli/cobra.go:1863`, `internal/cli/feature_patch.go:163` and `internal/cli/record_collision.go:96` are correct human file counts and stay byte-for-byte; `internal/cli/cobra.go:1908`'s operation count is the one consumer removed and migrated to the derived recipe's own count (§2.5.1, §6.1, §6.1.3, `RGA-068`, §12) |
| A6 — "contradictory observations" were undefined | Defined and refused. For a side the `change_kind` requires to exist, `*_observed: true` with `*_present: false` is invalid publication and schema input; a publisher that cannot establish such a side marks it **unobserved** with its mandatory availability reason, and the strict validator rejects every impossible observed-absence shape. Proven absence remains valid, and predicate 8 still requires it, for a **non-extant** side. This keeps `object_kind` / `content_kind` selection total without inventing a reason code (§6.4, `RGA-140`, `RGA-141`, ADR-036 D3) |
| A7 — P6's finalizer ordering and its two-failure case were under-specified | Ordering is pinned: recipe write → provenance attempt → state-mark attempt → **coverage finalization** → return propagation. A failed state mark does not cancel publication, which still binds the recipe write that landed; a failed publication is surfaced and exits non-zero, with **no success-shaped fallback**; when both fail the returned error names the state-mark failure first and the publication failure with it, chained under the repository's tight error handling (§6.15, `RGA-055`, `RGA-056`, `RGA-057`, ADR-036 D10/D15) |
| A8 — the P3 generation row | Confirmed as the conditional it is: the patch write at `internal/workflow/refresh.go:82` is unconditional and `AppendPatchGenerationForFeature` runs only when `newPatch != originalPatch` (`internal/workflow/refresh.go:93,102`). The P3 event remains the write (§2.7, §6.15, `RGA-059`, §12) |
| A9 — residual stale `ordinal` prose | `ordinal` is stated everywhere as the **one-based strict-grammar record order of the canonical patch**, contiguous, independent of the JSON array order and of the recipe, and the order `patch_fragment_sha256` boundaries follow (§6.4, `RGA-179`, ADR-036 D3) |
| A10 — tracking documents | Deliberately untouched here. rev-7 edits only this PRD, ADR-036 and the ADR index; the handoff, roadmap and supervisor log are updated separately |

What remains is a set of real deferrals with explicit review triggers. Each
mirrors ADR-036's deferred-decision table.

| Deferred | v1 answer | Reopen when |
|---|---|---|
| Durable tree object for a non-durable capture | Not minted; `reference-not-durable` | A capture mode needs durable authority without a commit |
| Delete/rename/copy/binary/mode-change/symlink/gitlink/executable operations | Not emitted; explicit reason codes | A dedicated ADR defines their apply, idempotency and path-type semantics |
| Exact present-state reclassification for `append-file` / `replace-in-file` | None; `operation-not-reclassifiable` | GH #13's ephemeral derivation proves a rule worth persisting |
| Parent-created target authority | `parent-created-target-unsupported`; no carrier field | A schema change can bind parent slug, generation and hash verifiably |
| Persisted contextual anchors | None; `contextual_hint` advisory only | GH #13 proves uniqueness and idempotence and a consumer needs persistence |
| Missing-coverage severity | Uniform `warn`, never graded by origin, never eligible | A durable out-of-file carrier exists that can prove a build produced coverage for a feature |
| `recipe-stale.json` severity | `warn` plus unconditional consumer ineligibility | The legacy marker cohort has aged out and a migration policy graduates it |
| Recipe regeneration inside `reconcile --accept`, `cycle` and `apply --mode done` | Not attempted; conditional `producer-patch-rewrite` / `recipe-not-regenerated` | GH #13 regenerates recipe and coverage from accepted operation candidates |
| Truthful recomputation of a P7 (`tpatch edit`) mutation in the general case | `manual-bound-artifact-edit` incompleteness unless a prior durable reference reconstructs and validates | A durable pre-edit reference is bound by an artifact a later run can independently verify |
| Governing direct filesystem edits to bound artifacts | Out of scope; detected at read time as binding-stale coverage | A file-watch or content-addressed store makes write-time observation possible |
| A GUI editor that forks and exits before the operator's later save | Not observable by P7; the later save is external tamper caught at read time | The same file-watch or content-addressed store trigger |
| Recovering effect coverage from a patch the strict grammar refuses | `canonical-patch-unparseable`; raw hash bound, `effects: []` | A lenient recovery projection is proved safe enough to bind an effect list |
| Recovering operation coverage from a recipe that does not decode | `recipe-undecodable`; raw hash bound, no operation assignment | A tolerant recipe decoder exists whose output can be bound as authority |
| A per-effect surface code for effect-local reasons | Aggregated under `recipe-coverage-incomplete` | A surface genuinely needs to gate on one effect-local reason |
| A `tpatch`-native way to materialize a canonical patch from a non-reapplying state | None; §6.11 names reviewed external `git apply --check` / `git apply` | A decision authorizes an explicit, non-silent materialization command |
| Classifying `object_kind` / `content_kind` when an **extant** side went unobserved | `unknown` on the axis, with its mandatory unavailable reason and an `ambiguous` disposition; an unobserved **non-extant** side leaves both axes definite and still raises its reason | A producer can take a capture on that path — for `implement` (P6) that means a decision to reconstruct a reference tree before writing a recipe |
| Distinguishing an absent bound artifact from a present-but-unreadable one **in the record** | Not distinguished: `patch_present` / `recipe_present` are readable existence, and only the diagnostic names the cause | A consumer needs to act differently on the two, which would require a third presence value and a migration |
| Gating explicit `apply --mode execute` on incomplete coverage | Not gated; order 4 executes and warns | GH #13 has shipped and the incomplete cohort has aged out, so a gate would not break legacy features |
| Automatic coverage repair in doctor | Read-only `D10` warnings | A future accepted doctor contract authorizes a guarded fix |
| Cross-file transactional publication | Coverage-last plus rerun recovery | Recomputed-hash rejection proves insufficient in soak |
| Bundling GH #15 and GH #13 in one release | Separate v0.17.0 / v0.18.0 | Never without a new decision |

## 12. Claims-audit appendix

| Claim | Source |
|---|---|
| Current generator uses touched paths and whole-file postimages | `internal/workflow/recipe_autogen.go:45-122` |
| Deletes are skipped and recipe remains secondary to canonical patch | `internal/workflow/recipe_autogen.go:86-122`, `internal/cli/cobra.go:1894-1896` |
| Drift comparison is file-set based | `internal/workflow/recipe_autogen.go:211-251` |
| `AutogenAction` labels describe the run, not the artifact | `internal/workflow/recipe_autogen.go:123-132` |
| Autogen writes `recipe-stale.json` when a drifted recipe is preserved | `internal/workflow/recipe_autogen.go:184-198` |
| `RecipeFromPatch` reads postimages from the live worktree today | `internal/workflow/recipe_autogen.go:86-122` |
| Record runs autogen before patch-generation append | `internal/cli/cobra.go:1894-1949` |
| Record writes `post-apply.patch` before autogen | `internal/cli/cobra.go:1795` |
| Record stores base/capture metadata | `internal/cli/cobra.go:1830-1892` |
| `feature patch refresh\|fixup` writes the patch, then runs non-regenerating autogen, then appends a generation | `internal/cli/feature_patch.go:114,135,150-160` |
| `feature patch` captures the whole working tree via `CapturePatchScoped(s.Root, nil)` | `internal/cli/feature_patch.go:88` |
| An empty capture returns early with `no patch byte change; refresh skipped` and writes nothing | `internal/cli/feature_patch.go:91-95` |
| A non-empty capture classifying `Append == false` prints the same skipped line and returns before any write | `internal/cli/feature_patch.go:100-112` |
| `RefreshAfterAccept` rewrites `post-apply.patch` **unconditionally** | `internal/workflow/refresh.go:82` |
| Its `reconcile` capture generation is appended **only when `newPatch != originalPatch`** | `internal/workflow/refresh.go:93,102-115` |
| `RefreshAfterAccept` deliberately leaves `apply-recipe.json` stale | `internal/workflow/refresh.go:20-24` |
| `cycle` runs implement at step `[4/6]` and writes the patch at step `[6/6]` | `internal/cli/phase2.go:112`, `internal/cli/phase2.go:160-166` |
| `cycle` can return between those two steps | `internal/cli/phase2.go:122-126,127-129,152-154` |
| `cycle` skips the patch write on an empty capture and discards the write error | `internal/cli/phase2.go:165-166` |
| `apply --mode done` rewrites the patch inside `runApplyDone` | `internal/cli/cobra.go:982,1025-1044` |
| `apply --mode done`'s reapply branch reads the canonical patch and writes none | `internal/cli/cobra.go:1006-1022` |
| `apply --mode done` skips the patch write on an empty capture | `internal/cli/cobra.go:1043` |
| `apply --mode done` completes discovery before its first artifact write | `internal/cli/cobra.go:999-1039` |
| The provider response and the heuristic fallback are selected before one shared `json.Unmarshal` | `internal/workflow/implement.go:176-192` |
| `implement.go:194` writes the **raw** response when that unmarshal fails | `internal/workflow/implement.go:192-194` |
| Neither parse arm returns the unmarshal error; the only `return` in each arm is its own `WriteArtifact` error, and a successful write falls through to the provenance step and to the function's final return | `internal/workflow/implement.go:192-212,238-243` |
| `RunImplement`'s final statement is `return s.MarkFeatureState(slug, store.StateImplementing, "implement", "Apply recipe generated")` — a returned call, not a literal `nil` | `internal/workflow/implement.go:243` |
| `implement.go:209` writes the **reserialized** recipe when the unmarshal succeeds | `internal/workflow/implement.go:195-209` |
| `RunImplement` writes `recipe-provenance.json` after the recipe | `internal/workflow/implement.go:220-237` |
| `implement --manual` checkpoints an externally authored recipe after existence and JSON validation | `internal/cli/cobra.go:744`, `internal/store/manual.go:51-80` |
| That checkpoint advances the feature to `implementing` | `internal/store/manual.go:31,80` |
| The `--manual` implement artifact is `artifacts/apply-recipe.json` | `internal/store/manual.go:27-32` |
| `tpatch edit` selects an artifact by argument or state default | `internal/cli/c1.go:45-60,79-91` |
| `resolveArtifactPath` probes the feature root **before** `artifacts/` | `internal/cli/c1.go:33-44` |
| `tpatch edit` defaults to `apply-recipe.json` in `implementing` and `post-apply.patch` in `applied`/`unapplied` | `internal/cli/c1.go:47-58` |
| `openInEditor` blocks on the configured process and returns when it exits | `internal/cli/phase2.go:251-262` |
| `openInEditor` discards the `c.Run` error | `internal/cli/phase2.go:261` |
| `openInEditor` has exactly two call sites | `internal/cli/c1.go:91`, `internal/cli/phase2.go:88` |
| `openInEditor` starts no process when `$EDITOR` is unset | `internal/cli/phase2.go:252-255` |
| `amend` rewrites `request.md` and writes no bound artifact | `internal/cli/c1.go:207` |
| `tpatch land` composes an embedded `record` step rather than writing bound artifacts itself | `internal/cli/land.go:180`, `internal/cli/land.go:694-698` |
| `AutogenRecipeForRecord`'s recipe write is one shared site reached from `record` and `feature patch` | `internal/workflow/recipe_autogen.go:204`, `internal/cli/cobra.go:1900`, `internal/cli/feature_patch.go:135` |
| Recipe provenance stores base, time and recipe hash | `internal/workflow/implement.go:18-34,225-238` |
| RecipeOperation supports four types and optional preimage | `internal/workflow/implement.go:48-100` |
| Apply prechecks write-file before executing operations | `internal/workflow/recipe.go:54-112` |
| A missing recipe yields `no recipe found — run 'tpatch implement <slug>' first` | `internal/workflow/recipe.go:116-120` |
| A plain RunE error exits `1`; `ExitCodeError` carries its own code | `internal/cli/cobra.go:41-59` |
| Exit `2` is documented as pre-mutation input validation and exit `3` as a post-validation state-machine refusal | `internal/cli/reject.go:38-47` |
| `apply --mode` accepts `auto`, `prepare`, `started`, `execute`, `done` — there is no `reapply` value | `internal/cli/cobra.go:844,848` |
| The canonical-patch reapply path is the state-selected branch of `--mode execute`, taken when the state is `unapplied` or an unapplied baseline is pending | `internal/cli/cobra.go:906-946` |
| That branch returns before `LoadRecipe`, which runs only on the non-reapplying fall-through | `internal/cli/cobra.go:945-950` |
| Replace is first-match and append is unconditional | `internal/workflow/recipe.go:213-242` |
| `write-file` execution uses a fixed `0o644` mode | `internal/workflow/recipe.go:207-211` |
| `RecipeExecResult` already carries `Applied` and `Skipped` | `internal/workflow/recipe.go:15-25` |
| Apply summary prints `%d/%d operations succeeded` from `Applied`/`Operations` | `internal/cli/cobra.go:972` |
| ADR-029 D7 downgrades superseded preimage drift to warning-class | `docs/adrs/ADR-029-write-file-recipe-safety.md:74-76` |
| Supersession excludes the historical feature from default effective replay | `docs/adrs/ADR-028-supersession-edge-model.md:77-88` |
| The runtime implements that downgrade and proceeds with execution | `internal/workflow/writefile_safety.go:240-264,326-364` |
| Path-safety refusals are never downgraded by supersession | `internal/workflow/writefile_safety.go:270-278` |
| Verify V10 downgrades to `warn` for superseded features | `internal/workflow/verify_anchored.go:797-801,888-892` |
| Apply precheck compares the preimage before any postimage recognition | `internal/workflow/writefile_safety.go:108-170` |
| Verify's empty-preimage branch treats any existing file as a collision | `internal/workflow/verify_anchored.go:964-981` |
| Patch generation binds patch/recipe/base/capture | `internal/store/patch_generations.go:28-76` |
| Same-patch append can be a no-op | `internal/workflow/patch_generations.go:26-64` |
| `Append == false` requires equality with the **latest** generation, not with any prior generation | `internal/store/patch_generation_kinds.go:46-49` |
| `touched_paths` is filled by the fail-soft `FilesInPatch` | `internal/workflow/patch_generations.go:76` |
| Reconcile derivation fallback also uses the fail-soft `FilesInPatch` | `internal/workflow/reconcile_derivation.go:118-124` |
| `FilesInPatch` is documented fail-soft and silently drops C-quoted headers | `internal/gitutil/gitutil.go:885-911` |
| File novelty derives path and change kind from a `strings.Fields` split | `internal/workflow/file_novelty.go:130-219` |
| `cleanPatchPath` dequotes by trimming quotes without decoding C escapes | `internal/workflow/file_novelty.go:221-231` |
| Novelty output feeds reconcile classification | `internal/workflow/reconcile.go:987`, `internal/workflow/file_novelty.go:51` |
| Hunk-overlap attribution reuses the novelty path helper | `internal/workflow/hunk_overlap.go:150-160` |
| `PathsAffectedByPatch` returns the union of both diff sides plus rename/copy operands, by design | `internal/gitutil/unapply.go:33-35,80-91` |
| `PathsAffectedByPatch` decodes C-quoting through `strconv.Unquote` | `internal/gitutil/unapply.go:47-49` |
| Its unquoted fallback requires byte-identical `a/` and `b/` payloads | `internal/gitutil/unapply.go:105-121` |
| A shipped test pins the rename/copy both-side result | `internal/gitutil/unapply_test.go:83-102` |
| Reverse-apply path scope is consumed by apply, unapply and worktree validation, with no error channel at any of the three | `internal/cli/cobra.go:919,1131`, `internal/cli/feature_unapply.go:156` |
| `FilesInPatchStrict` returns the b-side path of every entry | `internal/gitutil/patch_paths_strict.go:235-253` |
| Five shipped call sites consume that b-side list | `internal/cli/land.go:767,1212`, `internal/workflow/refresh.go:59`, `internal/workflow/verify_landed.go:1009,1163` |
| `.git`-containment header detection spans several diff dialects and asserts no path authority | `internal/store/store.go:504-540` |
| Git-internal stanza sanitization spans `diff -ruN`, `Only in ` and `Binary files ` | `internal/gitutil/gitutil.go:1170-1270` |
| `countPatchFiles` counts `diff --git` prefixes for display only | `internal/cli/cobra.go:2094-2101` |
| `countPatchFiles` is used correctly as a file count | `internal/cli/cobra.go:1863` |
| `countPatchFiles` is used correctly as a file count in the amend summary | `internal/cli/feature_patch.go:163` |
| `countPatchFiles` is used correctly as a file count in a collision entry | `internal/cli/record_collision.go:96` |
| `countPatchFiles` is used incorrectly as a recipe operation count | `internal/cli/cobra.go:1908` |
| That use sits inside the `case workflow.AutogenGenerated` arm, so it cannot execute when no recipe was generated | `internal/cli/cobra.go:1906-1908` |
| Verify requires provenance for preimage-bearing recipes | `internal/workflow/verify_anchored.go:816-840` |
| Verify check IDs are a frozen snake_case vocabulary | `internal/workflow/verify.go:47-70` |
| Verify severity vocabulary is `block`/`block-abort`/`warn` | `internal/workflow/verify.go:73-77` |
| A failing `block` check yields verdict `failed` and exit `2`; `warn` does not | `internal/workflow/verify.go:624-637` |
| Doctor's shipped registry is `D1`-`D9`, so `D10` is the next free ID | `internal/workflow/doctor.go:233-245` |
| Record refuses a same-path staged/unstaged overlap before capture | `internal/cli/cobra.go:1538,1549-1556` |
| Record notes unrelated staged paths without refusing | `internal/cli/cobra.go:1561-1565` |
| Overlap classification is computed by `StagedUnstagedOverlap` | `internal/gitutil/capture_modes.go:275-328` |
| `CaptureUnstagedPatch` diffs index → worktree | `internal/gitutil/capture_modes.go:175-272` |
| Record capture-mode vocabulary | `internal/cli/record_capture_modes.go:32-37` |
| `--regenerate-recipe` overwrites an existing recipe | `internal/cli/cobra.go:1898,2008` |
| Autogen writes `recipe-stale.json` when a drifted recipe is preserved | `internal/workflow/recipe_autogen.go:184-198` |
| Phase 2 currently terminally handles only all-present | `internal/workflow/reconcile.go:445-464` |
| Current operation evaluator treats missing write target as applicable | `internal/workflow/reconcile.go:611-655` |
| Adjacent argument fixture reproduces merge/rebase and replay hazards | `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md:1-243` |
| ADR-029 requires exact-byte preimages and all-or-nothing precheck | `docs/adrs/ADR-029-write-file-recipe-safety.md:25-56` |

## 13. Context for GH #13

GH #13 must not assume:

- complete coverage means the new upstream satisfies the preimage;
- `cross_base_status` is an eligibility verdict rather than producer scope;
- the `producer` field is an authority claim;
- `reference-tree-only` means "safe to replay elsewhere";
- a producer-generated recipe is trustworthy without recomputation;
- patch-generation identity proves current recipe bytes;
- a persisted replace/append operation has unique anchor authority;
- an anchor exists anywhere in v0.17.0 artifacts;
- a downgraded supersession warning certifies a superseded recipe;
- a verify `warn` with exit `0` — on missing coverage, incomplete coverage or
  a present `recipe-stale.json` — implies eligibility;
- the absence of coverage is a neutral or recoverable state rather than an
  unconditional refusal;
- `patch_present: false` or `canonical-patch-empty` means "nothing to check,
  therefore safe" rather than an unconditional refusal;
- `canonical-patch-unparseable` or `recipe-undecodable` means the artifact is
  absent, or that its bound raw hash makes the bytes usable;
- `preimage_present: false` or `postimage_present: false` proves the path was
  absent, without checking the matching `*_observed` flag;
- `manual-bound-artifact-edit` is an advisory note rather than a refusal;
- `PathsAffectedByPatchStrict`'s union and `FilesInPatchStrict`'s b-side list
  are interchangeable;
- a conflict should terminate before provider/phase-4 evidence;
- textual patch application belongs in operation-candidate acceptance.

GH #13 derives its own ephemeral anchors and independently proves uniqueness,
postcondition identity and idempotent reclassification for each one. It also
owns regenerating recipe and coverage for features that P3, P4 or P5 left
explicitly incomplete, and for features whose only record came from P6
(`canonical-patch-missing`) or P7 (`manual-bound-artifact-edit`) (§6.15).

Its planning starts only after this PRD and ADR-036 are accepted.
