# ADR-036 - Recipe Generation and Coverage Authority

**Status**: Accepted rev-7
**Date**: 2026-09-01
**Owner**: Core
**Issue**: [GH #15](https://github.com/tesseracode/tesserapatch/issues/15)
**Companion**:
[PRD-recipe-generation-authority](../prds/PRD-recipe-generation-authority.md)
rev-7
**Depends on**:
[ADR-010](./ADR-010-provider-conflict-resolver.md),
[ADR-024](./ADR-024-patch-generation-manifest-boundary.md),
[ADR-025](./ADR-025-reconcile-evidence-and-revision-schema.md),
[ADR-028](./ADR-028-supersession-edge-model.md), and
[ADR-029](./ADR-029-write-file-recipe-safety.md)
**Amends**: [ADR-029](./ADR-029-write-file-recipe-safety.md) D3 only, and only
for exact-postimage recognition, which narrows **two** of D3's four refusal
cases: the empty-preimage collision ("empty preimage, file exists") and the
non-empty expected-hash mismatch ("expected hash present, file hash differs"),
in each case only when the observed bytes are byte-exactly the operation's
postimage. ADR-029 D3's missing-target refusal ("expected hash present, file
missing"), its unreadable-target refusal, its all-or-nothing precheck
atomicity, path safety and ADR-029 D7's supersession severity are preserved
unchanged.
**Blocks**: GH #13 operation-replay candidate planning and implementation

## Revision history

| Rev | Date | Change |
|---|---|---|
| rev-0 | 2026-09-01 | Initial proposal. Reviewed NEEDS REVISION. |
| rev-1 | 2026-09-01 | Preserve ADR-029 D7 supersession severity (D7); convergent provenance rerun including noop (D6); executable/non-reclassifiable-operation/parent-created incompleteness (D5); this document's D3 is the single canonical strict coverage schema, extended with `recipe_present`, effect modes/presence flags, `effect_sha256`, `contextual_hint` and `cross_base_status`; empty-preimage exact-postimage amendment to ADR-029 D3 (D7); `created_by` parent-postimage reuse dropped from v1 (D7); current `--unstaged` classified as commit-kind HEAD (D2); verify check `recipe_generation_coverage` severity ladder (D13); already-present result accounting (D14); GH #15 planning acceptance amended to record that v0.17 persists no anchor (D8). Reviewed NEEDS REVISION. |
| rev-2 | 2026-09-01 | Complete five-producer inventory and one shared coverage publication API (**D15**, new); record-generated origin proved by exact derived-recipe byte equality rather than any trust-by-label marker, retiring file-set equality for that decision (**D16**, new); missing coverage is uniformly warning-class and never authorizes replay, so `missing-produced` versus `missing-legacy` is retired (D13); `recipe-stale.json` presence is warning-class and consumer-ineligible rather than a new hard verify failure, with a pinned precedence ladder (D13); the parser rule is restated as one authoritative strict effect grammar plus registered thin adapters and registered non-authoritative scanners over the complete derived PI-1..PI-11 inventory (D1); `effects[].kind` is replaced by orthogonal `change_kind` / `content_kind` / `object_kind` axes and `reason_code` by a sorted `reason_codes[]` array (D3); coverage gains a required `producer` field and `capture.mode` gains `reconcile` (D3, D2); new reason codes `producer-patch-rewrite` and `recipe-not-regenerated` (D3, D15). Reviewed NEEDS REVISION. |
| rev-3 | 2026-09-01 | Producer registry expanded from five to **seven** (P1-P7): `implement` (P6, provider/heuristic `RunImplement` recipe writes and the `implement --manual` checkpoint) and `artifact-edit` (P7, `tpatch edit` on a canonical bound artifact) join the registry, `cycle` is restated as a composite whose implement step is P6, and direct filesystem edits outside `tpatch` are named as ungovernable external tamper (D15); required `patch_present` with `canonical-patch-missing` / `canonical-patch-empty` so an absent or empty canonical patch is explicit rather than vacuously complete (D3, D5); `reasons[]` given an exact feature-versus-effect allocation table and an iff completeness predicate that no contradictory record can decode (D3, D4); `producer-patch-rewrite` redefined conditionally and always paired with `recipe-not-regenerated` (D3, D15); the undefined normalized hunk digest replaced by an explicitly specified required `patch_fragment_sha256`, modes and object kind sourced from the immutable observation rather than headers alone, and `content_kind` extended to `text` / `binary` / `none` (D3, D5); execute-mode apply refuses a withheld partial recipe with a named `recipe-generation-incomplete` contract instead of falling back silently (**D17**, new); PI-7 corrected — the unapply splitter already decodes quoting and deliberately returns both sides, so its replacement is a new `PathsAffectedByPatchStrict` all-paths API, not `FilesInPatchStrict`'s b-side projection, migrated fail-closed at all three call sites (D1); the existing `FilesInPatchStrict` b-side consumers registered as PI-12 with an unchanged contract (D1); surface code `recipe-coverage-stale-marker` mapped explicitly to schema reason `recipe-stale-marker-present` (D13); immutable observation capture generalized to every producer (D2); regeneration remediation printed only after a dry derivation proves it can succeed (D11). Reviewed NEEDS REVISION. |
| rev-4 | 2026-09-01 | A governed producer event is defined over **three** categories — a successful bound write, a successful manual checkpoint of a bound artifact, and an explicitly contracted re-observation checkpoint even when the bytes are identical — so P2's non-empty same-patch `classification.Append == false` branch is a category-(c) event that publishes coverage before returning while keeping its shipped user meaning, and P2's `patch == ""` early return is explicitly **not** a checkpoint (D15); D17 rewritten — its guidance is now reachable and state-aware, because D17 is reached only when `reapplying` is false, so the unreachable "choose the canonical-patch reapply path" instruction is deleted, the false "the operator chooses that internal branch" claim is removed, the `implement --manual` state transition to `implementing` is disclosed, coverage-binding refusal precedes the withheld-recipe branch, and exit `2` is justified as pre-mutation executable-plan validation (D17); new record-level reason `canonical-patch-unparseable` so a present non-empty patch the strict grammar refuses binds its raw hash and publishes explicit incompleteness rather than nothing (D3, D5); required `recipe_decodable`, with `recipe_sha256` now always hashing the exact raw recipe bytes whenever the file is present, and new record-level reason `recipe-undecodable` (D3, D5, D9); required per-effect `preimage_observed` / `postimage_observed`, new effect-local `postimage-unavailable`, and both flags added to the `effect_sha256` inputs, so an unobserved side is no longer encoded as a proven absence (D3); exact reason-code → `disposition` mapping with a deterministic multi-reason precedence (`mismatch` → `ambiguous` → `unsupported`), strictly validated (D3); one canonical **ten-predicate** completeness list in D3, reproduced byte-identically in the companion PRD under a parity guard (D3, D4); the impossible surface/schema bijection replaced by a one-way mapping table plus vocabulary disjointness (D13); the raw seven-writer threshold replaced by a registry count of seven plus a separate AST-derived site-to-producer mapping over the **eight** direct bound `WriteArtifact` call sites (D15); P7 retriggered on the **resolved** artifact path rather than the user token, with feature-root decoy precedence pinned (D15); P6 source truth corrected — `internal/workflow/implement.go:194` is the JSON-unmarshal-failure raw-response arm and `:209` the valid-JSON reserialized arm, both P6 events, the failure arm owing bound incomplete coverage for the undecodable bytes it just wrote (D15); `openInEditor`'s missing error propagation named as a refactor precondition, with changed bytes snapshotted and coverage published or invalidated even on editor error (D2, D15); `tpatch land`'s embedded record step named as P1 orchestration rather than a new producer (D15); `patch_fragment_sha256` boundaries pinned to strict-grammar line-start `diff --git` records rather than a naive substring scan (D3); `CapturePatchScoped` citation corrected to `internal/cli/feature_patch.go:88` (D2). Reviewed NEEDS REVISION. |
| rev-5 | 2026-09-01 | `unknown` added to **both** `effects[].object_kind` and `effects[].content_kind`, with a total selection rule that never infers a kind across an unobserved transition, mandatory `preimage-unavailable` / `postimage-unavailable` company, an `ambiguous`-class disposition, a decode refusal on any complete record carrying it, and pinned no-capture pure-rename / mode-only / type-transition cohorts (D3, D5); **semantic emptiness** defined as zero bytes, whitespace-only bytes or any strict parse yielding zero normalized effects, all three carrying the existing `canonical-patch-empty` code with the raw patch hash bound whenever the file is present, and completeness predicate 1 tightened to require **at least one** normalized effect so `complete` beside `effects: []` no longer decodes (D3, D5); **P6 source truth corrected again** — `internal/workflow/implement.go:192-195` writes the raw response on the unmarshal-failure arm and, when that write succeeds, does **not** return the parse error but falls through to the provenance attempt and the ordinary `nil` return, so the successful raw write is a governed event that publishes `recipe_present: true`, `recipe_decodable: false`, the raw-byte hash and `recipe-undecodable` at the common coverage-last point before that `nil` return, while a failed `WriteArtifact` is no event and owes nothing (D10, D15); D17 generalized from "a withheld partial recipe" to **"coverage binds no executable recipe"**, with a **total** seven-case ordered execute/apply classifier — coverage-binding refusal, withheld recipe, present-but-undecodable recipe, present-and-decodable recipe under incomplete coverage (which still executes, backward-compatibly, with a non-authority warning), the two legacy shapes, and complete coverage — and every other shape routed to the malformed refusal (D17); D9 presence binding made **bidirectional**, so `patch_present` / `recipe_present` are recomputed from actual on-disk readable existence and both `true`-beside-absent and `false`-beside-present are binding-stale (D9, D17); every stale `predicate 8` reclassification cross-reference retargeted to **predicate 10** and the parity guard widened to validate all `predicate N` references (D3); the surface-mapping totality sentence scoped to the explicitly mapped subset, with the remaining surface codes named as verify/binding/aggregate conditions outside the schema-reason vocabulary and disjointness preserved (D13); `openInEditor`'s call sites corrected from three to **two**, `$EDITOR`-unset modeled as no process, no byte change and no P7 event (D2, D15); site mapping restated so each **reachable call chain** maps to exactly one producer and the `recipe_autogen` helper maps to P1 or P2 by caller rather than to one static producer (D15); P2's same-byte classification corrected to compare the **latest** generation (D15); `Recipe generated`'s operation count sourced from the derived recipe inside its actual `AutogenGenerated` output branch, with the incomplete-coverage status emitted from the common post-autogen coverage output path (D15); and the `patch_fragment_sha256` boundary question closed — a line-start `diff --git` inside a valid unified-diff hunk is prefixed by `+`, `-` or a space and therefore cannot be mistaken for a boundary, while a malformed bare token is parsed or refused as a new record by the strict grammar, so grammar recognition wins and PI-12's semantics stay fixed (D1, D3). |
| rev-6 | 2026-09-01 | **Classification reads the effect's extant sides only** — an `add` whose postimage is observed and present takes `object_kind` and `content_kind` from that postimage even though its preimage was never independently observed-as-absent, a `delete` takes both axes from its observed preimage on the same terms, and `unknown` is reached only when an **extant** side the effect needs was not observed and no decisive binary marker stands on its own, so `modify` / `rename` / `copy` (which need both sides) are the shapes an unobserved side pushes to `unknown` (D3, D5); the unobserved **non-extant** side still carries its mandatory `preimage-unavailable` / `postimage-unavailable` reason, still lands the effect in `ambiguous`, and still fails predicate 8 — availability reasons record observation, `unknown` records classification, and the two are no longer conflated (D3); D17 restated over **readable existence** — order 2 fires whenever valid `incomplete` coverage carries `recipe_present: false` and no **readable** recipe is on disk, covering an absent file and a present-but-unreadable one alike under the phrase "coverage binds no readable executable recipe", while order 1 fires only when recomputed readable existence **differs** from the stored flag, so a truthfully recorded present-but-unreadable recipe takes order 2 and a later readability change takes order 1 from either direction (D9, D17); readable existence is stated as a deliberate record-layer collapse of physical absence and unreadability, with the read-error diagnostic retaining the actual cause and every readability transition treated as bidirectional binding drift (D9); **owner mismatch is two conditions with two severities** — a coverage envelope whose `feature` differs from the requested target slug is a D9 binding failure at rung 2, surfaced as `recipe-coverage-owner-mismatch` with `block` / exit `2`, mapped to **no** schema reason and moved into the unmapped binding-level set; a decoded recipe whose `feature` differs from the coverage owner is the record-level schema reason `recipe-owner-mismatch`, fails predicate 3, makes valid coverage `incomplete`, and surfaces only through the aggregate `recipe-coverage-incomplete` row at rung 3 with `warn` / exit `0`, so the mapped subset is **seven** codes and the unmapped set **eleven** (D9, D13); `reasons` and `reason_codes` required to equal **exactly** the applicable closed codes, sorted and deduplicated, with no applicable code optional (D3, D15); `effects[].ordinal` defined as the one-based strict-grammar record order of the canonical patch, which is also the order `patch_fragment_sha256` boundaries follow (D3); P6's finalizer corrected — `RunImplement`'s final statement is `return s.MarkFeatureState(...)` (`internal/workflow/implement.go:243`), not a literal `nil`, so coverage publishes after the recipe write, the provenance attempt and the state-mark attempt and before that return is propagated, still binding the recipe write when the state mark fails, and a coverage-publication failure is surfaced rather than dropped (D10, D15); and P3's generation append corrected to the conditional it is — the patch write at `internal/workflow/refresh.go:82` is unconditional while `AppendPatchGenerationForFeature` runs only when `newPatch != originalPatch` (`internal/workflow/refresh.go:93,102`) (D15); and the coverage record's **own** readability decided the other way round from its presence fields — a coverage file that exists but cannot be read is present-and-unusable (rung 1, order 1, `recipe-coverage-malformed` with the read error named) rather than absent, so rung 5 and orders 5-6 stay reserved for genuinely absent coverage and the classifier stays total (D13, D17). |
| rev-7 | 2026-09-01 | **`operation-missing` is scoped to effects for which an operation was actually owed** — it is raised for a normalized effect **if and only if** that effect is otherwise representable in v1 (its path is repository-safe, its `change_kind` / `content_kind` / `object_kind` / mode domain is the supported one of D5, every side its `change_kind` names was observed, no parent-created-target exclusion applies, and no assigned operation is non-reclassifiable) **and** the decoded recipe supplies it no assigned operation. It is never raised merely because an operation was intentionally not emitted for an effect that already carries a capability, safety or availability exclusion, so an absent, unreadable or undecodable recipe raises it on the otherwise-representable effects only. `operation-missing` therefore occurs exactly as the singleton `["operation-missing"]`, `disposition: mismatch` is exactly that singleton, its co-occurrence with any other effect-local code is refused at decode, and the precedence ladder's live cases are `ambiguous` over `unsupported` (D3, D5); **contradictory observations are named and refused** — for a side its `change_kind` requires to exist, `*_observed: true` with `*_present: false` is an impossible publication and schema input, a publisher that cannot establish an extant side marks it **unobserved** with its mandatory availability reason instead, and the strict validator rejects the impossible observed-absence shape, which keeps `object_kind` selection total without inventing a reason code (D3); the ADR-029 amendment restated accurately — exact-postimage recognition narrows **both** the empty-preimage collision **and** the non-empty expected-hash mismatch whenever the observed bytes equal the operation postimage, while the missing-target expected-hash refusal, the unreadable-target refusal, path safety, all-or-nothing atomicity and ADR-029 D7 supersession severity are unchanged, so the "all other three cases unchanged" claim is deleted and the rows aligned (D7); **P2's specific policy re-paired** — whenever the conditional `producer-patch-rewrite` applies, `recipe-not-regenerated` is raised with it, and P2's incomplete record carries the full exhaustive applicable set rather than `recipe-not-regenerated` alone (D3, D15); **coverage-publication failure is surfaced on every P1-P7 event** — publication is last, so a failure may leave coverage absent or stale, but it is returned to the caller and the command exits non-zero, never success-shaped, and one generic table-driven per-producer failure contract replaces any per-producer prose (D10, D15); D17's legacy rows restated over **readable existence** — coverage genuinely absent with no readable recipe keeps the existing `LoadRecipe` read / no-recipe error and exit `1`, coverage genuinely absent with a readable recipe keeps the existing execution, and a physically unreadable recipe follows the no-readable row rather than the readable one (D17); PI-10's production display consumers listed in full — `internal/cli/cobra.go:1863`, `internal/cli/feature_patch.go:163` and `internal/cli/record_collision.go:96` are the three legitimate human file counts that stay, and `internal/cli/cobra.go:1908`'s operation-count misuse is the one consumer removed, so the two-consumer reading is corrected (D1); P6's finalizer ordering restated — coverage finalization happens after the state-mark attempt and before `RunImplement`'s return is propagated, still binds a successful recipe write when the state mark fails, and when publication also fails the coverage failure is surfaced with the state-mark failure preserved and chained under the repository's tight error handling, with no success-shaped fallback (D10, D15); P3's generation-append row corrected to the conditional it is (D15); and `effects[].ordinal` stated everywhere as the canonical patch's strict-grammar record order (D3). |

## Context

`RecipeFromPatch` currently parses touched paths with `strings.Fields`, emits
one postimage `write-file` operation for each non-deleted path, skips deletions,
treats a rename as a write to the destination, and emits no
`preimage_hash` (`internal/workflow/recipe_autogen.go:24-126`). The canonical
patch remains authoritative and the generated recipe is explicitly described
as replay/inspection only.

ADR-029 already defines exact-byte `write-file` preimages and all-or-nothing
precheck semantics. The runtime supports those semantics, but no production
generator populates the field. Activating it naively would break three shipped
surfaces:

1. verify requires usable `recipe-provenance.json` as soon as any write-file
   carries a preimage (`internal/workflow/verify_anchored.go:816-840`);
2. apply currently checks the preimage before recognizing an exact postimage,
   so a repeated apply would refuse an already-applied feature
   (`internal/workflow/recipe.go:54-112`,
   `internal/workflow/writefile_safety.go:108-170`);
3. ADR-029 D7 downgrades preimage drift on a superseded feature to
   warning-class and lets execution proceed
   (`docs/adrs/ADR-029-write-file-recipe-safety.md:74-76`,
   `internal/workflow/writefile_safety.go:264-305,326-364`). A new unconditional
   refusal rule would silently revoke that accepted decision.

Two production callers still derive path sets from the deliberately fail-soft
`gitutil.FilesInPatch`: `patch-generations.json` `touched_paths`
(`internal/workflow/patch_generations.go:76`) and the reconcile derivation
fallback (`internal/workflow/reconcile_derivation.go:104-124`). Both silently
drop C-quoted paths (`internal/gitutil/gitutil.go:885-910`). Coverage cannot be
authoritative while a sibling manifest disagrees about which paths a patch
touched. They are not the only naive header readers: `parsePatchNoveltyPaths`
assigns a per-path create/modify/delete/rename action through
`parseDiffGitPaths`'s `strings.Fields` split and `cleanPatchPath`'s
`strings.Trim(path, "\"")` (`internal/workflow/file_novelty.go:130-231`) and
`parsePatchHunks` attributes hunk ranges through the same helper
(`internal/workflow/hunk_overlap.go:150-175`). A rule phrased as "one parser"
without naming these is not implementable.

`PathsAffectedByPatch` (`internal/gitutil/unapply.go:36-125`) is a fourth
reader, and rev-2 mischaracterized it. It is **not** a naive splitter: it
decodes C-quoting through `strconv.Unquote`
(`internal/gitutil/unapply.go:47-49`), it disambiguates unquoted paths
containing spaces by requiring byte-identical `a/` and `b/` payloads
(`internal/gitutil/unapply.go:105-121`), and it deliberately returns the
**union of both diff sides** plus `rename from`/`rename to` and
`copy from`/`copy to` operands (`internal/gitutil/unapply.go:33-35,80-91`)
because a reverse rename recreates the a-side path and removes the b-side one.
Replacing it with `FilesInPatchStrict`'s b-side projection would silently
shrink reverse-apply and unapply scope and leave a rename source unrestored and
unsnapshotted. Its migration therefore needs a new strict **all-paths** API,
not the existing b-side one (D1).

`record` is also not the only writer of the inputs coverage binds. Six other
production paths write `artifacts/post-apply.patch` or
`artifacts/apply-recipe.json`:

- `feature patch refresh|fixup` writes the patch
  (`internal/cli/feature_patch.go:114`) and then calls
  `AutogenRecipeForRecord(..., autogen=true, regenerate=false)`
  (`internal/cli/feature_patch.go:135`);
- `reconcile --accept` refreshes the patch through `RefreshAfterAccept`
  (`internal/workflow/refresh.go:82`) and deliberately leaves
  `apply-recipe.json` stale, which that function documents
  (`internal/workflow/refresh.go:20-24`);
- `cycle` step `[6/6]` rewrites the patch with no autogen and no generation
  append (`internal/cli/phase2.go:166`);
- `apply --mode done` rewrites the patch inside `runApplyDone`
  (`internal/cli/cobra.go:982,1044`);
- **`implement` writes the recipe itself.** `RunImplement` persists
  `apply-recipe.json` on **two mutually exclusive arms of one parse**: when
  `json.Unmarshal` of the extracted response fails it writes the **raw**
  response verbatim (`internal/workflow/implement.go:192-195`), and when the
  response is valid JSON it writes the **reserialized** recipe
  (`internal/workflow/implement.go:209`). Both the provider response and the
  heuristic fallback flow through that same parse
  (`internal/workflow/implement.go:176-190`), so the arms are not
  "provider versus heuristic". **Neither arm returns the parse error**: the
  unmarshal error is consumed by the `if` that selects the arm, the only
  `return` inside either arm is its own `WriteArtifact` error, and a successful
  write on either arm falls through to the provenance step and to
  `RunImplement`'s final statement, which is
  `return s.MarkFeatureState(slug, store.StateImplementing, ...)`
  (`internal/workflow/implement.go:243`) — a returned call, not a literal
  `nil`. It then writes `recipe-provenance.json`
  (`internal/workflow/implement.go:220-237`); `implement --manual` instead
  checkpoints an already-authored recipe after validating that it exists and is
  syntactically valid JSON, advancing state to `implementing`
  (`internal/cli/cobra.go:744`, `internal/store/manual.go:51-80`). rev-2 omitted
  this writer entirely, which left the single most common origin of a
  non-record recipe outside the registry;
- **`tpatch edit` mutates a bound artifact in place.** It resolves an artifact
  by name or by state default — `apply-recipe.json` in `implementing` and
  `post-apply.patch` in `applied`/`unapplied`
  (`internal/cli/c1.go:45-60,79-91`) — resolving the **feature root before**
  `artifacts/` (`internal/cli/c1.go:33-44`), and hands the resolved path to
  `$EDITOR`. `openInEditor` blocks on the configured process but **discards its
  error** (`_ = c.Run()`, `internal/cli/phase2.go:251-262`), so today the CLI
  cannot tell a clean editor exit from a failed one.

If coverage were published only by `record`, any of these six could silently
leave a `complete` coverage record describing a patch or a recipe that no
longer exists, which is a worse failure than having no coverage at all.

`amend` is **not** among them: it rewrites `request.md`
(`internal/cli/c1.go:207`) and feature metadata, and writes neither bound
artifact. A registry entry claiming otherwise would assert a write that does
not exist.

`tpatch land` is **not** a new producer either. It composes an **embedded
`record` step** (`internal/cli/land.go:180`, `internal/cli/land.go:694-698`)
and then stages and commits; every bound write on that path is `record`'s own,
at `record`'s own call site. It is P1 orchestration, and a registry entry for
`land` would double-count one write.

Patch generations already bind patch bytes, recipe bytes, base, capture mode,
touched paths and dependencies (`internal/store/patch_generations.go:28-76`).
They are append-only and may deliberately remain unchanged when patch bytes
are unchanged. They therefore cannot be the sole authority for a regenerated
recipe over identical patch bytes.

GH #13 needs to distinguish complete, bound generation evidence from partial
or legacy recipes. It must still decide replay eligibility independently.

## Decision

### D1 - One authoritative strict effect grammar over a derived parser inventory

Recipe generation extends and reuses `gitutil.FilesInPatchStrict` and its
underlying strict grammar. `parsePatchTouchedFiles`
(`internal/workflow/recipe_autogen.go:45-84`) is retired. No new patch parser
is introduced.

The normalized effect domain includes:

- add, modify, delete, rename and copy;
- regular, executable, symlink and gitlink objects, plus `unknown` where no
  observed **extant** side establishes which (D3);
- text and binary content, plus `unknown` on the same terms;
- mode changes;
- C-quoted paths, spaces, CRLF and no-newline-at-EOF markers.

Unknown, contradictory or partially parsed effects fail closed. A fail-soft
path list is not coverage authority.

**The rule is scoped, not absolute.** rev-1 claimed "exactly one grammar
answers the path question in production" while naming only two callers, which
was false: production contains several other `diff --git` readers. The
implementable rule is:

> **One strict normalized effect grammar is authoritative for every production
> consumer that claims a file path or an effect kind. A thin adapter may
> project paths, hunk ranges or per-path actions out of that grammar's
> normalized effect set. A specialized parser may remain only where it asserts
> neither path nor effect authority, and every such parser is registered and
> guarded.**

An adapter is acceptable only if it derives its output from the strict
normalized effect set and propagates the strict error. It may not re-implement
header splitting, dequoting or a/b-prefix stripping, and it may not swallow the
error to preserve a shorter legacy result.

The complete derived inventory of production `diff --git` path/effect readers
is:

| ID | Site | Today | Authority claimed today | v0.17.0 disposition |
|---|---|---|---|---|
| PI-1 | `parsePatchTouchedFiles` (`internal/workflow/recipe_autogen.go:45-84`) | `strings.Fields` b-side split | path + effect | **removed** |
| PI-2 | `gitutil.FilesInPatch` (`internal/gitutil/gitutil.go:885-911`) | fail-soft split on the first ` b/` | path | **deleted or demoted to a test-only helper**; unreachable from production |
| PI-3 | `AppendPatchGenerationForFeature` → `touched_paths` (`internal/workflow/patch_generations.go:76`) | PI-2 consumer | path | migrate to the strict authority or an exact adapter |
| PI-4 | `touchedPathsFromPostApplyPatch` (`internal/workflow/reconcile_derivation.go:118-124`) | PI-2 consumer | path | migrate to the strict authority or an exact adapter |
| PI-5 | `parsePatchNoveltyPaths` + `parseDiffGitPaths` + `cleanPatchPath` (`internal/workflow/file_novelty.go:130-231`) | `strings.Fields` split and `strings.Trim(path, "\"")` dequote | path **and** change kind (`create`/`modify`/`delete`/`rename`) | migrate to an adapter projecting path and `change_kind` from the strict effect set |
| PI-6 | `parsePatchHunks` (`internal/workflow/hunk_overlap.go:150-175`) | PI-5 helper for path attribution; own hunk-range scan | path (hunk ranges are its own) | path attribution from the strict adapter; the hunk-range projection may remain |
| PI-7 | `PathsAffectedByPatch` + `pathsFromDiffGitHeader` (`internal/gitutil/unapply.go:36-125`) | **already quoting-aware** (`strconv.Unquote`, `internal/gitutil/unapply.go:47-49`); deliberately returns the union of both diff sides plus rename/copy from/to operands (`internal/gitutil/unapply.go:33-35,80-91`) | path, **both-side rollback scope** | migrate to a new strict **all-paths** API (`PathsAffectedByPatchStrict`) that preserves both-side scope; **not** `FilesInPatchStrict`'s b-side projection |
| PI-8 | `headerReferencedGitPath` (`internal/store/store.go:504-540`) | `strings.Fields` over several diff dialects | **none** — `.git`-containment refusal only | retained, registered, guarded |
| PI-9 | `stripGitInternalFileStanzas` + `headerPathIsGitInternal` (`internal/gitutil/gitutil.go:1170-1270`) | multi-dialect sanitizer (`diff --git`, `diff -ruN`, `Only in`, `Binary files`) | **none** — sanitization only | retained, registered, guarded |
| PI-10 | `countPatchFiles` (`internal/cli/cobra.go:2094-2101`) | counts `diff --git` prefixes; consumed by **four** production sites — three as a human file count (`internal/cli/cobra.go:1863`, `internal/cli/feature_patch.go:163`, `internal/cli/record_collision.go:96`) **and one, wrongly, as a recipe operation count** (`internal/cli/cobra.go:1908`) | **none** — display counter | retained, registered, guarded as a **human file count only**; the three file-count consumers stay unchanged and the `cobra.go:1908` operation-count use is removed/migrated (see below) |
| PI-11 | `FilesInPatchStrict` and its grammar (`internal/gitutil/patch_paths_strict.go:235-253`) | strict header grammar, b-side projection | path (b-side) | **the authority**, extended to the full normalized effect model; the b-side projection is retained unchanged for PI-12 |
| PI-12 | Existing `FilesInPatchStrict` b-side consumers: `internal/cli/land.go:767,1212`, `internal/workflow/refresh.go:59`, `internal/workflow/verify_landed.go:1009,1163` | strict b-side path list | path (b-side) | **unchanged contract**; they keep consuming the b-side projection with identical semantics, and extending the shared grammar may not change what they receive |

PI-8, PI-9 and PI-10 are retained deliberately. PI-8 and PI-9 must recognize
non-Git diff dialects that the strict Git grammar does not model, and their
output is a refusal or a sanitization, never a path set a caller acts on. PI-10
is a human-facing count. Each is registered in the inventory and covered by a
guard that fails if its output starts feeding a path or effect decision.

**PI-10 has four production consumers, and exactly one of them is
illegitimate.** rev-6 named only two, which read as though the counter fed one
correct site and one wrong one; it feeds **four**. `countPatchFiles` counts
`diff --git` prefixes, which is a **file** count, and three consumers use it as
exactly that and are unchanged by this decision:

| Consumer | Use | Disposition |
|---|---|---|
| `internal/cli/cobra.go:1863` | `filesChanged` for `record.md` | correct file count; unchanged |
| `internal/cli/feature_patch.go:163` | the `%d files` field of `Amended patch for %s (%s, %d bytes, %d files)` | correct file count; unchanged |
| `internal/cli/record_collision.go:96` | the `Files:` field of a collision report entry | correct file count; unchanged |
| `internal/cli/cobra.go:1908` | `(%d ops)` in `Recipe generated: artifacts/apply-recipe.json` | **operation** count derived from a display scanner; removed and migrated to the derived recipe's own count |

At `internal/cli/cobra.go:1908` it is used as
`countPatchFiles(patch)-len(skippedPaths)` to print
`Recipe generated: artifacts/apply-recipe.json (%d ops)` — an **operation**
count derived from a display scanner rather than from the recipe. Under this
decision that line prints the actual operation count of **the derived recipe**,
and it stays where it is: inside the `case workflow.AutogenGenerated` arm of
the shipped autogen switch (`internal/cli/cobra.go:1906-1908`), which is by
construction the branch in which a recipe *was* generated and therefore the
only branch that can hold a real operation count. The incomplete-coverage
status is **not** printed from that line — a withheld derivation never enters
the `AutogenGenerated` arm — but from the **common post-autogen coverage
output path** that every autogen outcome reaches. rev-4's "when no recipe was
published, `cobra.go:1908` prints the incomplete coverage status instead"
attributed an output to a branch that cannot execute in that case. PI-10
remains registered as a human file counter, and the guard
fails any reuse of it as an operation, effect or eligibility count.

**The `diff --git` boundary question is closed for PI-11/PI-12 too.** D3's
`patch_fragment_sha256` boundaries are the strict grammar's own record starts,
and the two shapes that could look like a boundary are decided by the grammar
rather than by a scanner: inside a valid hunk, every body line is prefixed by
`+`, `-` or a space, so an embedded `diff --git` is not at line start; a bare
line-start token outside a valid hunk is parsed as a new record, or the patch is
refused as `canonical-patch-unparseable`. Nothing about PI-12's b-side contract
changes as a result — fragment boundaries reuse the record starts PI-11 already
recognizes, so no projection is widened and no leniency is introduced.

**PI-7 needs a new strict API, not the existing one.** rev-2 recorded PI-7 as
"own splitter over both sides; same quoting gap" and prescribed migration to
"the strict authority". Both halves were wrong:

- the quoting gap does not exist there. `PathsAffectedByPatch` runs every
  operand through `strconv.Unquote` before use
  (`internal/gitutil/unapply.go:47-49`), and its unquoted-path fallback
  disambiguates embedded spaces by requiring byte-identical `a/` and `b/`
  payloads (`internal/gitutil/unapply.go:105-121`);
- the both-side union is the point, not an accident. Unapply reverse-applies a
  patch, so a rename's **source** must be snapshotted and restored, and its
  destination removed. The function documents exactly that
  (`internal/gitutil/unapply.go:33-35`) and a shipped regression pins it
  (`internal/gitutil/unapply_test.go:83-102`).

`FilesInPatchStrict` returns only the b-side path of each entry
(`internal/gitutil/patch_paths_strict.go:235-236`). Projecting PI-7 onto it
would drop every rename and copy **source** from unapply scope: the reverse
apply would recreate a file at a path nobody snapshotted, and the rollback
guard would not cover it. That is a safety regression, not a hardening.

The v0.17.0 migration therefore adds a second strict entry point over the same
grammar:

```go
// PathsAffectedByPatchStrict returns the sorted, unique union of every
// path a patch touches on either side: each effect's canonical path plus
// its old_path for rename and copy effects. It preserves the rollback
// scope PathsAffectedByPatch provides today and refuses, with an error,
// every input FilesInPatchStrict refuses.
func PathsAffectedByPatchStrict(patch string) ([]string, error)
```

The name may vary; the signature and the semantics may not. It is derived from
the same normalized effect set as `FilesInPatchStrict` — one grammar, two
documented projections — and it returns an error rather than a short list on
any header the grammar refuses.

All three production call sites migrate to it and **fail closed**:

| Call site | Today | After S1 |
|---|---|---|
| `internal/cli/cobra.go:919` (`apply --mode execute` reapply snapshot) | `uniqueSortedPaths(gitutil.PathsAffectedByPatch(canonical))` feeds `SnapshotWorktreePaths` with no error channel | strict call; a parse error returns before the snapshot is taken and before any mutation |
| `internal/cli/cobra.go:1131` (`validateReapplyMaterialization`) | `gitutil.PathsAffectedByPatch(canonical)` feeds `DiffFromCommitForPaths` with no error channel | strict call; a parse error returns before the diff is computed |
| `internal/cli/feature_unapply.go:156` | `gitutil.PathsAffectedByPatch(patch)` feeds `ValidateWorktreePaths` and the unapply scope with no error channel | strict call; a parse error returns before any snapshot, reverse patch or artifact write |

**None of these three has an existing fail-soft handler**, so the migration
adds a new refusal path at each and may not claim to reuse one. This is the
opposite of PI-3 and PI-4, where the caller already tolerates an error and the
strict error joins that existing channel. A migration note that says "propagate
into the caller's existing continue-on-error handling" is true for PI-3/PI-4
and false for PI-7.

PI-12 is registered so the shared grammar cannot be extended out from under it.
`land.go`, `refresh.go` and `verify_landed.go` consume the **b-side** list and
must keep receiving exactly that: adding rename/copy sources to
`FilesInPatchStrict`'s output would silently widen a landed-patch file set, a
refresh comparison and two verify path scopes. The all-paths union is a
separate function with a separate name, and a regression row pins the b-side
contract.

A **source-inventory guard** derives the inventory from production sources
rather than from this table: it enumerates every production `diff --git`
reader and requires each to be the authority (PI-11), a registered adapter, a
registered b-side consumer (PI-12), or a registered non-authoritative scanner.
A new unregistered reader fails the guard. The PRD's S1 slice and its
parser-ownership guard rows are scoped to this complete PI-1..PI-12 inventory,
not to a two-caller subset.

A migrated caller that **already tolerates** an unparseable header — PI-3 and
PI-4 — now surfaces the strict error to that existing fail-soft handler rather
than returning a silently short path list. A migrated caller that has **no**
such handler — all three PI-7 call sites — gains a new fail-closed return
instead. The distinction is stated rather than blurred, because claiming a
handler that does not exist is how a strict parser turns into a nil-scope
write.

### D2 - Capture an immutable observation before the first bound write, on every producer

**Every governed producer (D15) captures its immutable observation before its
first bound write, not just `record`.** rev-2 stated this for `record` only,
which left P2-P7 free to publish coverage derived from bytes they had already
overwritten. The generalized rule is:

> Before a governed producer writes or checkpoints a bound artifact, it
> captures the canonical patch bytes it is about to bind, the reference
> descriptor, the capture descriptor, and the ordered pre/postimage
> observations for every effect it will describe. Coverage is derived from that
> immutable observation and from nothing re-read afterwards.

Each observation records paths, kinds, modes and exact byte hashes; source
bodies remain in memory only. It also records, per effect and per side,
**whether that side was observed at all** — the `preimage_observed` /
`postimage_observed` flags of D3. An unobserved side is never encoded as a
proven absence. **No producer may claim an origin proof (D16) or a `complete`
status without a complete immutable observation**: a partial observation yields
`preimage-unavailable` or `postimage-unavailable` on the affected effects and,
where the reference itself is missing, `reference-not-durable` at the top
level.

Per-producer application:

| Producer | Observation captured before its first bound write |
|---|---|
| P1 `record` | patch, reference, capture descriptor, pre/postimage set — before recipe, provenance, generation and coverage writes |
| P2 `feature patch refresh\|fixup` | the same set, captured before `internal/cli/feature_patch.go:114` writes the patch, so the *pre-refresh* recipe and patch are both observable; the same observation also serves P2's non-writing category-(c) checkpoint (D15) |
| P3 `reconcile --accept` | the same set, captured before `internal/workflow/refresh.go:82` rewrites the patch, against the accepted upstream commit it diffs from (`internal/workflow/refresh.go:78-95`) |
| P4 `cycle` patch-capture step | the same set, captured before `internal/cli/phase2.go:166` writes the patch |
| P5 `apply --mode done` | the same set, captured before `internal/cli/cobra.go:1044` writes the patch, in the same discovery-before-writes window the shipped code already establishes (`internal/cli/cobra.go:999-1039`) |
| P6 `implement` | the recipe bytes it is about to write (or, for `--manual`, the externally authored bytes it is about to checkpoint) plus whatever canonical patch already exists; `implement` performs no patch capture, so its capture mode is `no-capture` |
| P7 `tpatch edit` | a **before** snapshot of the **resolved** bound artifact path taken prior to invoking `$EDITOR`, and an **after** snapshot taken once the editor process returns (`internal/cli/phase2.go:251-262`) |

**`openInEditor` must start propagating its error, and P7 must snapshot
regardless.** `openInEditor` is synchronous only as far as the configured
process: it blocks on `c.Run()` and then discards the result
(`internal/cli/phase2.go:261`). Two consequences are pinned here rather than
discovered in implementation:

- the refactor contract is that `openInEditor` **returns**, and its caller
  propagates, the `c.Run` error. A producer that cannot tell whether the editor
  ran cannot decide truthfully what it observed;
- an editor error does **not** excuse publication. P7 takes its **after**
  snapshot even when `c.Run` returns an error, and if the bytes changed it
  publishes — or invalidates — coverage **before** returning that error. The
  alternative leaves a mutated bound artifact beside coverage that still claims
  the old bytes.

A GUI editor that forks and exits before the operator's later save is a
different case and is stated as such: at the moment `c.Run` returns, no
mutation is observable, so P7 correctly publishes nothing. The operator's later
save happens with no `tpatch` process running, which makes it **external
tamper** under D15 — ungovernable at write time and caught at read time as
binding-stale coverage (D9). P7 does not claim to cover it.

P7 is the weakest case and is treated as such. `tpatch edit` observes a
mutation; it does not author one, and it holds no capture of the tree the
edited bytes describe. Its published coverage therefore carries
`reference.kind: unavailable` and the `manual-bound-artifact-edit` reason
**unless** the producer can independently reconstruct and validate a prior
durable reference from the feature's own bound artifacts — the pre-edit
coverage's `reference` plus a recomputed `preimage_set_sha256` that still
matches. Reconstructed-and-validated is the only path by which P7 may carry a
`commit` reference forward; a remembered or copied reference is not.

The reference descriptor is typed:

- `commit`: a resolved commit reconstructs the preimage;
- `index-snapshot`: the preimage came from the captured index and is not
  durable unless another committed authority binds it;
- `unavailable`: no trustworthy preimage can be reconstructed.

**Every shipped record capture mode maps to `commit` or `unavailable`. No
shipped record mode emits `index-snapshot` in v1.** In particular,
`unstaged-worktree` is commit-kind HEAD: the CLI refuses a capture whose
staged and unstaged edits touch the same path before any capture runs
(`internal/cli/cobra.go:1549-1556`), so every path that reaches an accepted
`--unstaged` patch has an index entry identical to HEAD and therefore a HEAD
preimage. Unrelated staged paths elsewhere in the index are reported as an
advisory note (`internal/cli/cobra.go:1560-1565`) and are outside the captured
effect set, so they cannot contaminate the reference.

| Record mode | Reference kind | Preimage source |
|---|---|---|
| `working-tree-all` | `commit` | resolved HEAD |
| `staged-index` | `commit` | resolved HEAD |
| `unstaged-worktree` | `commit` | resolved HEAD (overlap refused pre-capture) |
| `committed-range`, `auto-committed-range`, `explicit-committed-range` | `commit` | resolved lower commit |
| `reconcile` | `commit` | the accepted upstream commit `RefreshAfterAccept` diffs against (`internal/workflow/refresh.go:78-95`) |
| `no-capture` | `unavailable`, or a reconstructed-and-validated `commit` | none of its own — the producer ran no capture (P6, P7) |
| any mode whose commit cannot be resolved | `unavailable` | none |

`reconcile` is a capture mode of the `reconcile-accept` producer (D15), not of
`tpatch record`; it is the mode that path already writes into
`patch-generations.json` (`internal/workflow/refresh.go:113`). The
patch-writing governed producers capture a whole working tree and reuse
`working-tree-all`: `feature patch refresh|fixup` through
`CapturePatchScoped(s.Root, nil)` (`internal/cli/feature_patch.go:88`), and
`cycle` and `apply --mode done` through `gitutil.CapturePatch`
(`internal/cli/phase2.go:160-166`, `internal/cli/cobra.go:1025-1044`).

`no-capture` is the mode of the two producers that write or checkpoint a bound
artifact without running a patch capture at all: `implement` (P6), whose whole
job is to author a recipe, and `tpatch edit` (P7), which observes an operator's
in-place mutation. `no-capture` carries empty `pathspecs` and `claim_ids`. It
does **not** license an unbound record: a `no-capture` producer still binds the
canonical patch and recipe hashes it observed, and it still publishes
`reference.kind: unavailable` unless D2's reconstruct-and-validate path
succeeds.

`index-snapshot` remains a decodable schema value reserved for future or
internal callers whose preimage genuinely is an uncommitted index state. A v1
producer that emits it is a bug; a v1 consumer that receives it treats the
reference as non-durable.

A deterministic `preimage_set_sha256` binds the ordered path/kind/mode/hash
set. It does not turn an ephemeral index snapshot into a durable tree.

### D3 - Add deterministic `artifacts/recipe-coverage.json` (canonical schema)

Coverage is a strict sidecar rather than a field on `apply-recipe.json` or an
entry in `patch-generations.json`.

**This decision is the single canonical definition of the schema.** The
companion PRD reproduces the block below byte-identically and adds no field,
vocabulary value or reason code of its own. Where the two documents could be
read as disagreeing, this decision governs.

Version 1 is exactly:

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

Every field is required and every field is present on every record. Arrays are
present and non-null. Unknown fields are rejected at decode; the decoder is
strict in both directions and never tolerates a trailing, null or duplicate
member. Entries are ordered by normalized effect ordinal; `operation_indexes`
are one-based and sorted ascending. The artifact carries no timestamp, source
body, prompt, provider response or secret.

`effects[].ordinal` is the **one-based position of the effect's record in the
strict grammar's parse of the canonical patch**, counted in the order the
grammar recognizes record starts, from the first byte of the file. It is a
property of the patch, not of the recipe, of the effect list's JSON order or
of any sort the producer chose: the first `diff --git` record the grammar
admits is ordinal `1`, the next is `2`, and so on with no gaps. Because
`patch_fragment_sha256`'s boundaries are those same recognized record starts,
the fragment ranges follow the ordinal order exactly — fragment *n* begins at
record *n*'s first byte and ends immediately before record *n+1*'s, or at end
of file for the last ordinal. A consumer therefore recomputes both from one
parse.

#### Field rules

- `producer` names the governed producer that published this record (D15). It
  is advisory context for diagnostics, never authority: a consumer still
  recomputes every binding under D9. Because the field lives inside the file,
  it carries nothing when the file is absent — which is exactly why missing
  coverage cannot be graded by origin (D13).
- `patch_present` is `true` exactly when `artifacts/post-apply.patch` exists
  and was readable at publication time.
  - **Readable existence deliberately collapses two physical states.** A file
    that is not there and a file that is there but cannot be read produce the
    same `false`, and that collapse is intentional: the record's question is
    "does a usable canonical patch exist for this feature", and both answers
    are "no". The record layer therefore carries no third value, and no
    implementation may invent one. What is **not** collapsed is the
    diagnostic: a read failure is reported with its actual cause (the
    underlying I/O or permission error and the path), never as "file not
    found", so an operator is never told a present file is missing. D9 applies
    the same rule at read time and treats every readability transition —
    readable becoming unreadable, unreadable becoming readable — as binding
    drift in the direction it occurred.
  - When it is `false`, `patch_sha256` is `""`, `effects` is `[]`,
    `coverage_status` is `incomplete`, `cross_base_status` is `unsupported`,
    and `reasons` contains `canonical-patch-missing`. A producer may not
    publish an effect list, a patch hash or a `complete` status for a patch it
    does not have.
  - When it is `true`, `patch_sha256` is the SHA-256 of the exact canonical
    patch bytes **even when those bytes are empty** — the empty-input digest is
    a real value and is published as one. `patch_sha256` is never `""` while
    `patch_present` is `true`.
  - **A semantically empty canonical patch is explicitly incomplete, not
    vacuously complete.** rev-4 defined emptiness as "zero bytes", which is
    narrower than the condition that actually matters: a patch of whitespace,
    or one the strict grammar parses cleanly into **zero** normalized effects,
    reaches the same dead end — there is nothing to cover — while satisfying
    "every effect is represented" vacuously. **Semantic emptiness** is
    therefore any of:
    - zero bytes;
    - bytes that are entirely whitespace;
    - any strict parse that succeeds and yields **zero** normalized effects.

    All three publish the same shape, and the v1 code is unchanged:
    `patch_present: true`, `patch_sha256` bound to the exact raw patch bytes
    (**always**, whenever the file is present — including the empty-byte-string
    digest), `effects: []`, `coverage_status: incomplete`,
    `cross_base_status: unsupported`, and `canonical-patch-empty` in `reasons`.
    `canonical-patch-empty` is defined as **zero normalized effects**, and the
    zero-byte and whitespace-only cases are instances of it rather than
    separate codes. rev-2 left this state undefined, where an empty effect list
    would have satisfied "every effect is represented" and published
    `complete` — and, worse, would have derived
    `cross_base_status: reference-tree-only` from the vacuous truth that every
    represented effect is a creation. None of that is permitted.
  - **A present, non-empty patch the strict effect grammar refuses is
    `canonical-patch-unparseable`, not an absent patch and not silence.**
    rev-3 assumed every present non-empty patch normalizes, so a strict-parse
    refusal had no encoding: a producer holding such a patch could only lie
    (`patch_present: false`), publish an effect list it does not have, or
    publish nothing. The record is instead `patch_present: true`, `patch_sha256`
    = the digest of the **exact raw patch bytes** (the binding still holds, and
    D9 still recomputes it), `effects: []`, `coverage_status: incomplete`,
    `cross_base_status: unsupported`, and `canonical-patch-unparseable` in
    `reasons`. **A strict parse refusal never leaves a feature with no coverage
    after a producer event.**
- `recipe_present` and `recipe_decodable` are two questions, and rev-3 conflated
  them into one flag whose `false` value could mean either "no file" or "a file
  that does not decode". The consequence was that a present-but-corrupt recipe
  published `recipe_sha256: ""`, so D9's deletion/tamper recomputation had
  nothing to compare and an out-of-band corruption was indistinguishable from a
  deletion. v1 separates them:
  - `recipe_present` is `true` exactly when `artifacts/apply-recipe.json` exists
    and was readable at publication time. `recipe_decodable` is `true` exactly
    when those bytes strict-decode as a valid recipe.
  - **Readable existence, on the same terms as `patch_present`.** An absent
    recipe and a present-but-unreadable recipe both publish
    `recipe_present: false`; the collapse is at the record layer only, the
    read-error diagnostic keeps the real cause and path, and D9/D17 read the
    flag as "no readable recipe is bound", never as "no file is on disk".
  - **File absent, or present but unreadable**: `recipe_present: false`,
    `recipe_decodable: false`, `recipe_sha256: ""`, and every
    `operation_indexes` array empty. `recipe-undecodable` is **not** raised
    here: nothing was read, so nothing failed to decode.
  - **File present**: `recipe_present: true` and `recipe_sha256` is **always**
    the SHA-256 of the exact raw bytes on disk, decodable or not.
    `recipe_sha256` is never `""` while `recipe_present` is `true`, and it is
    never omitted and never null.
  - **Present and undecodable**: `recipe_decodable: false`, the raw bytes are
    still bound by `recipe_sha256`, `coverage_status` is `incomplete`, and
    `reasons` contains the record-level `recipe-undecodable`. Operation
    assignment is impossible, so every `operation_indexes` array is empty.
  - **Present and valid**: both flags `true`.
  - `recipe_decodable: true` with `recipe_present: false` is an invalid record
    and is refused at decode: nothing can decode from a file that is not there.
- `reference.commit` is a 40-character lowercase hex commit for
  `reference.kind: commit` and `""` for every other kind.
- **The effect kind axes are orthogonal, and all three are recomputable.**
  rev-2's single `effects[].kind` conflated three independent questions and
  could not describe a binary rename, an executable rename or a symlink delete
  without losing one of them. v1 uses three fields instead:
  - `change_kind` — what happened to the path: `add`, `modify`, `delete`,
    `rename`, `copy`. It is read from the strict grammar's own record header,
    which exists whether or not any tree was observed, so it is never
    `unknown`;
  - `content_kind` — how the content is expressed: `text`, `binary`, `none`,
    `unknown`;
  - `object_kind` — what the object is: `regular`, `executable`, `symlink`,
    `gitlink`, `unknown`.

  Every combination the strict grammar can produce is representable, and each
  axis is decided independently of the others.
- **`object_kind` is selected from one named side, and is `unknown` when the
  extant side it needs was not observed.** rev-4 said it is "the postimage
  side's kind when the postimage is present, and the preimage side's kind
  otherwise", which is only decidable by a producer that looked. A no-capture
  producer (D15 P6) observes no tree at all, so rev-4 obliged it to name a kind
  it had never seen. rev-5 then over-corrected in the other direction, making a
  side the effect does not even require load-bearing. The rule is now total and
  scoped to the **extant** sides — the sides the effect's `change_kind`
  requires to exist: an `add` needs its postimage, a `delete` its preimage, and
  a `modify`, `rename` or `copy` both:
  - `postimage_observed: true` with `postimage_present: true` — the value is
    the **postimage** side's kind;
  - the postimage side is **not extant for this `change_kind`** (a `delete`),
    **and** `preimage_observed: true` with `preimage_present: true` — the value
    is the **preimage** side's kind. rev-6 also admitted a "proven absent"
    extant postimage here; rev-7 removes that clause, because an extant side
    can no longer be published as observed-and-absent (see the contradictory
    observation rule below), so the only postimage this branch can meet is a
    non-extant one;
  - in every other case — an extant side the selection needs unobserved — the
    value is `unknown`.
  - **No kind is inferred across an unobserved transition.** A patch header may
    carry a mode transition, but a producer that read neither side has verified
    nothing about the object it holds, and rev-4's rule would have let it
    publish `regular` for a symlink it never opened. `unknown` is the truthful
    answer, not a fallback for a hard case.
  - **An unobserved non-extant side does not force `unknown`.** A `delete`
    whose preimage was observed and present is classified from that preimage
    even when nobody looked at the (already gone) postimage; an `add` whose
    postimage was observed and present is classified from that postimage even
    when nobody independently observed the preimage's absence. The unobserved
    side still carries its mandatory unavailable reason, still makes the
    disposition `ambiguous` and still fails predicate 8 — it just does not
    erase a kind the producer actually saw.
- **Modes and object kind come from the immutable observation, not from the
  headers alone.** rev-2 said modes were "the exact Git file modes as written
  in the patch headers", which is not recomputable: a patch omits an
  unchanged-mode header entirely, so a consumer reading headers alone cannot
  tell an unchanged `100644` from a mode it failed to parse. `old_mode` and
  `new_mode` are instead read from the immutable pre/post capture observation
  (D2) — the reconstructed named tree for the preimage side and the observed
  postimage for the postimage side — and the patch headers are used only to
  corroborate them. A consumer recomputes them the same way, from the reference
  tree it reconstructs plus the patch it already has.
  - The value is `""` **exactly** when that side is either **proven absent** or
    **unobserved**, and in both cases the corresponding presence flag is
    `false`. `""` never means "the header did not say"; which of the two it
    does mean is read from the side's `*_observed` flag.
  - **The transition examples below are stated for observed sides.** With both
    sides observed, a mode transition is fully captured by the pair:
    `100644` → `100755` is
    `old_mode: "100644"`, `new_mode: "100755"`, and `object_kind: executable`
    from the postimage side. `100755` → `100644` is the reverse pair with
    `object_kind: regular`. A regular-to-symlink transition is
    `old_mode: "100644"`, `new_mode: "120000"`, `object_kind: symlink`; a
    symlink-to-regular transition is its reverse with `object_kind: regular`.
    An unobserved side's mode is always `""`, and when that side is **extant**
    for the effect `object_kind` is `unknown` as well: the header's declared
    transition is not a substitute for having looked. A half-observed `add` or
    `delete`, whose unobserved side is not extant, still records `""` for that
    side while keeping the kind its observed side established.
  - Permitted values are `100644`, `100755`, `120000`, `160000` and `""`. They
    are the authoritative mode record; `object_kind` is their derived
    classification, taken from whichever side D3 names.
- **`content_kind` is decided by an explicit rule, `none` exists, and so does
  `unknown`.** The rule is evaluated in this order, and the first branch that
  holds wins:
  - `content_kind: none` **if and only if** the effect's **known**
    `object_kind` is `gitlink`. A submodule pointer has no file content on
    either side, so calling it `text` was a fiction rev-2 could not defend and
    calling it `binary` would have invited a body-bearing operation. An effect
    whose `object_kind` is `unknown` is never `none`: nothing has established
    that it is a gitlink;
  - `content_kind: binary` when **either** a strict-grammar stanza marker
    proves it (a `GIT binary patch` stanza or a `Binary files ... differ`
    line) **or** the observed bytes of an extant side contain a NUL byte.
    Either proof is positive evidence and stands on its own, so this branch is
    reachable even when the other side was never observed;
  - `content_kind: text` **only** when every extant side the effect needs was
    observed and neither binary rule fires. "Extant" is the side the effect's
    `change_kind` requires to exist: an `add` needs its postimage, a `delete`
    its preimage, and a `modify`, `rename` or `copy` both. An extant side that
    was observed and present is a side whose bytes the producer read, and
    `text` is exactly the claim that those bytes carry no NUL;
  - `content_kind: unknown` otherwise — that is, whenever an extant side the
    effect needs was not observed and no binary proof is available. `text` is
    a positive claim about bytes, and a producer that did not read them may
    not make it.
  - **Only an unobserved extant side reaches `unknown`.** An unobserved
    **non-extant** side — the preimage of an `add`, the postimage of a
    `delete` — is not a side the content claim rests on, so its absence from
    the observation set does not degrade the axis. A `delete` whose preimage
    was read is `text` or `binary` from those bytes; an `add` whose postimage
    was read is `text` or `binary` from those bytes. Both still carry the
    unavailable reason for the side nobody looked at, are still `ambiguous`,
    and are still `incomplete` under predicate 8, because completeness demands
    observation of every side the effect names — including the proven-absent
    one. Classification and observation are two questions, and rev-5 answered
    the first with the second's evidence.
  - Symlink targets are ordinary bytes and are therefore normally `text`; a
    symlink whose target somehow trips the binary rule is `binary`, and either
    way the effect is unsupported for a different reason
    (`effect-symlink-unsupported`).
  - The rule reads the observed sides, so it is stable for effects the patch
    expresses without content: an **observed** pure rename with no content
    change and an **observed** mode-only change both have two observed sides
    with equal bytes, and both take `text` or `binary` from those bytes exactly
    as a content-changing effect would. A rename whose stanza carries no hunks
    is not thereby `none`. The same rename with **no** capture is `unknown` —
    the absence of hunks is not evidence about bytes nobody read.
- **`unknown` on either axis is a bounded, reasoned, non-complete state.**
  It is never free-standing decoration:
  - an effect carrying `object_kind: unknown` or `content_kind: unknown`
    **must** carry the unobserved-side reason(s) in `reason_codes` —
    `preimage-unavailable`, `postimage-unavailable`, or both — for **at least**
    the extant sides whose absence from the observation set forced the
    `unknown`. A record with an `unknown` axis and no such reason is refused at
    decode;
  - the converse does **not** hold. An availability reason is also mandatory
    for an unobserved **non-extant** side (see the observation rules below),
    and that occurrence proves nothing about classification: a half-observed
    `add` publishes `preimage-unavailable` beside a definite `content_kind`
    and a definite `object_kind` taken from its observed postimage. `unknown`
    tracks the extant side(s) that prevented classification; the availability
    reasons track observation. A decoder that infers `unknown` from the
    presence of an availability reason is wrong, and so is a producer that
    publishes `unknown` because one is present;
  - its `disposition` is therefore in the `ambiguous` class by the mapping
    below. Since rev-7 scopes `operation-missing` to effects that are
    *otherwise representable* — which an effect carrying a mandatory
    availability reason is not — no `mismatch` can co-occur with `unknown`, so
    `ambiguous` is the only disposition an `unknown` axis ever takes. `unknown`
    can never be `represented`;
  - `unknown` is **forbidden on a complete record**. Predicate 7 already
    excludes it through the mandatory reason, and the decoder refuses
    `coverage_status: complete` beside any `unknown` axis directly, so the two
    checks cannot drift apart;
  - the modes of an unobserved side stay `""`, exactly as the observation rules
    below require. `unknown` describes the classification; `""` describes the
    mode record. Neither is repurposed to carry the other's meaning.
- `old_path` is non-empty exactly when `change_kind` is `rename` or `copy`,
  and `""` for every other `change_kind`. Its presence follows the change axis
  alone and is independent of `content_kind` and `object_kind`.
- **`preimage_observed` / `postimage_observed` say whether the producer looked;
  `preimage_present` / `postimage_present` say what it found.** rev-3 carried
  only the presence pair, so "the producer could not read this side" and "this
  side provably does not exist" collapsed onto the same `false`. That made the
  P6 and P7 no-capture shapes untruthful: `implement` observes no tree at all,
  and its effects would have claimed proven absence on both sides. The rules
  are exact:
  - `*_present` is meaningful **only** when the corresponding `*_observed` is
    `true`. A consumer that reads a presence flag without checking the
    observation flag is wrong.
  - `*_observed: false` requires `*_present: false`, the corresponding hash
    `""`, the corresponding mode `""`, **and** the matching effect-local reason
    in `reason_codes`: `preimage-unavailable` for an unobserved preimage,
    `postimage-unavailable` for an unobserved postimage. The reason is
    mandatory, not optional colour.
  - `*_observed: true` with `*_present: false` is **proven absence**: the
    producer looked and the path was not there. Its hash and mode are `""`, and
    that `""` carries the positive meaning "absent", exactly as rev-3 intended.
    It is a valid shape **only for a side the effect's `change_kind` does not
    require to exist** — the preimage of an `add`, the postimage of a
    `delete` (see the contradictory-observation rule below);
  - `*_observed: true` with `*_present: true` requires a real 64-hex content
    hash and a real mode from the permitted mode set.
  - **Contradictory observations are impossible input, not a new state.** For a
    side the effect's `change_kind` requires to exist — the postimage of an
    `add`, `modify`, `rename` or `copy`, and the preimage of a `modify`,
    `rename`, `copy` or `delete` — the pair `*_observed: true` with
    `*_present: false` asserts that the producer looked and found the extant
    side missing, which contradicts the `change_kind` the same record publishes
    from the strict grammar. A producer that cannot establish an extant side's
    bytes does **not** publish it as observed-and-absent: it marks that side
    **unobserved** (`*_observed: false`, `*_present: false`, `""` hash, `""`
    mode) and raises the mandatory `preimage-unavailable` /
    `postimage-unavailable` reason, which is the honest record of "nobody
    established this". The strict validator therefore **refuses** every
    impossible observed-absence shape at decode, in the publisher and in the
    consumer alike. This is what keeps the `object_kind` and `content_kind`
    selection rules total without minting a reason code for a state that cannot
    truthfully occur: every extant side is observed-and-present, or unobserved
    with its availability reason, and there is no third alternative to
    classify.
  - Completeness requires every side the effect names to be **observed**, not
    merely absent (see the canonical predicate list below). An `add` needs its
    postimage observed **and** its preimage observed-as-absent; a `delete`
    needs its preimage observed **and** its postimage observed-as-absent; a
    `modify`, `rename` or `copy` needs both sides observed. The proven-absent
    side is included deliberately and symmetrically: predicate 8 is about
    having looked, so **both** half-observed cohorts fail it. This is
    a stricter set than classification uses: classification reads the
    **extant** sides only, so a half-observed `add` still names its kinds,
    while completeness also demands the proof of absence that the same record
    lacks. The two rules are deliberately different, and predicate 8 is the one
    that gates `complete`.
- **`patch_fragment_sha256` replaces rev-2's undefined "normalized hunk
  digest".** rev-2 hashed a digest it never specified, which made
  `effect_sha256` unrecomputable by any consumer. The field is now explicit,
  required on every effect, and defined exactly:

  > `patch_fragment_sha256` is the SHA-256 of the **exact raw bytes** of this
  > effect's record in the canonical `artifacts/post-apply.patch`, beginning at
  > the first byte of the effect's `diff --git` line and ending at the byte
  > immediately before the next effect's `diff --git` line, or at end of file
  > for the last effect.

  **The boundaries are the strict grammar's own record starts, not a substring
  scan.** A fragment begins only at a `diff --git` sequence the strict grammar
  has already recognized as the start of a file record — that is, at the start
  of a line, in the header position the grammar accepts. A `diff --git`
  sequence appearing inside a hunk body, inside a quoted path, inside a binary
  stanza or mid-line does **not** open a fragment, because the grammar did not
  admit it as a record start. rev-3's "the next `diff --git` line" wording
  invited a naive `strings.Index` implementation that a patch containing a
  literal `diff --git` in added content would silently split in the wrong
  place, giving two consumers different digests for the same effect.

  The fragment is hashed verbatim: original line endings are retained (a CRLF
  body hashes differently from an LF body), `\ No newline at end of file`
  markers are retained, binary stanzas are included whole, and no
  normalization, trimming, re-wrapping or re-encoding is applied. A consumer
  holding the canonical patch recomputes the value by running the same strict
  grammar to obtain the record offsets, then hashing that byte range.

  **Grammar recognition wins, and the embedded-token case is closed rather
  than merely warned about.** Two shapes exhaust it:
  - inside a **valid** unified-diff hunk, every body line is prefixed by `+`,
    `-` or a space, so a line whose content is `diff --git a/x b/y` appears on
    the wire as `+diff --git a/x b/y` (or `-`/` `). Its `diff --git` sequence
    is therefore **not** at line start, and cannot be mistaken for a record
    boundary by a grammar that requires the line-start header position. The
    hazard rev-4 described is real for a substring scan and structurally
    impossible for the grammar;
  - a **bare** line-start `diff --git` token appearing where the hunk-line
    prefix discipline has been violated is, by definition, not inside a valid
    hunk. The strict grammar parses it as the start of a new file record — or
    refuses the patch outright under `canonical-patch-unparseable` when the
    surrounding record is then ill-formed. Either way the outcome is decided by
    the grammar, not by a heuristic, and both outcomes are already encodable.

  This closes the question without widening any parser contract: PI-12's b-side
  semantics (D1) are unchanged, because fragment boundaries are computed from
  the same record starts PI-12 already recognizes, and no new projection or
  leniency is introduced.
- `effect_sha256` is a deterministic SHA-256 over the canonical JSON encoding
  of the complete normalized effect descriptor: ordinal, `change_kind`,
  `content_kind`, `object_kind`, path, old path, old mode, new mode, **both
  observation flags**, both presence flags, both content hashes, and
  `patch_fragment_sha256`. The three kind axes are hashed exactly as
  published — `unknown` included — so a record that later asserts a definite
  kind for a side nobody observed does not recompute. It is the per-effect
  binding a consumer recomputes under D9; it is not a hash of the effect's
  file bytes and not a hash of this
  JSON object. **No hidden or unspecified input contributes to it** — the
  descriptor list above is exhaustive, and the phrase "normalized hunk digest"
  appears nowhere in this schema. The observation flags are inputs because they
  are load-bearing: without them, an unobserved side and a proven-absent side
  would produce the same digest.
- `reason_codes` is a sorted-ascending, duplicate-free, non-null array. It is
  `[]` exactly when `disposition` is `represented`, and non-empty for every
  other disposition. An array is required because the axes are orthogonal: a
  binary rename raises `effect-binary-unsupported` **and**
  `effect-rename-unsupported`, and a rev-1-style scalar would have had to drop
  one of them. `disposition` remains a single closed value. It carries
  **effect-local** conditions only (see the allocation table below).
- `reasons` is the required, non-null, sorted-ascending, duplicate-free array
  of **feature/record-level** codes. rev-2 declared the field and then never
  said what belonged in it, which left every incompleteness cause silently
  assignable to either array. It is `[]` exactly when no record-level condition
  holds. It carries no effect-local condition.
- **Both arrays are exhaustive, not illustrative.** `reasons` equals **exactly**
  the set of record-level closed codes whose raising condition holds for this
  record, and each effect's `reason_codes` equals **exactly** the set of
  effect-local closed codes whose raising condition holds for that effect —
  each set sorted ascending and deduplicated. No applicable code is optional,
  no applicable code may be dropped because another already explains the same
  incompleteness, and no code may be present whose condition does not hold. A
  binary rename that publishes only `effect-rename-unsupported` is as invalid
  as one that publishes `effect-copy-unsupported`. Producers therefore compute
  the arrays as the full evaluation of the closed condition list, and both
  validation and the acceptance matrix check set equality rather than
  membership.
  **Exactness ranges over each code's own raising condition as written in the
  closed list below, and over nothing else.** A code is applicable when its
  stated condition holds, not when it plausibly describes the situation. That
  distinction is load-bearing for `operation-missing`, whose condition rev-7
  scopes to effects for which an operation was actually **owed**: an effect
  already excluded by a capability, safety or availability condition does not
  satisfy that condition, so set equality neither permits nor requires
  `operation-missing` on it. rev-6's exactness rule, read against rev-6's
  unconditional trigger ("a normalized effect has no assigned operation"),
  attached `operation-missing` to every deliberately unrepresented effect and
  — through `mismatch` precedence — contradicted every pinned `unsupported` and
  `ambiguous` cohort. The trigger, not the exactness rule, was the defect.
- **Allocation is exact, and no occurrence appears in both arrays.** A
  condition is record-level when it is a statement about the feature's record
  as a whole — a missing patch, a non-durable reference, a producer event, a
  whole-recipe simulation outcome. It is effect-local when it is a statement
  about one normalized effect. Each occurrence is written once, in the array
  its class names:

  | Reason code | Array | Rationale |
  |---|---|---|
  | `canonical-patch-missing` | `reasons` | No patch exists, so there are no effects to attribute it to |
  | `canonical-patch-empty` | `reasons` | The patch as a whole is empty; `effects` is `[]` |
  | `canonical-patch-unparseable` | `reasons` | The strict grammar refused the patch as a whole, so no effect was normalized to carry it; `effects` is `[]` |
  | `reference-not-durable` | `reasons` | One reference descriptor governs the whole record |
  | `recipe-undecodable` | `reasons` | Decodability is a property of the recipe file, not of any one effect |
  | `recipe-owner-mismatch` | `reasons` | The **decoded recipe's** owning feature is a record-level binding. This is not the coverage envelope's own owner check, which is a D9 binding failure with no schema reason at all (D13) |
  | `recipe-stale-marker-present` | `reasons` | The marker sits beside the recipe, not on an effect |
  | `producer-patch-rewrite` | `reasons` | A statement about the producer event |
  | `recipe-not-regenerated` | `reasons` | A statement about the recipe as a whole |
  | `manual-bound-artifact-edit` | `reasons` | A statement about an out-of-band mutation of the artifact |
  | `operation-surplus` | `reasons` | A surplus operation belongs to **no** effect, so no effect can carry it |
  | `simulation-mismatch` | `reasons` | Simulation runs over the complete operation set against the complete preimage set; the affected paths are named in the diagnostic, never by copying the code onto effects |
  | `effect-delete-unsupported` | `reason_codes` | Effect-local |
  | `effect-rename-unsupported` | `reason_codes` | Effect-local |
  | `effect-copy-unsupported` | `reason_codes` | Effect-local |
  | `effect-binary-unsupported` | `reason_codes` | Effect-local |
  | `effect-mode-only-unsupported` | `reason_codes` | Effect-local |
  | `effect-symlink-unsupported` | `reason_codes` | Effect-local |
  | `effect-gitlink-unsupported` | `reason_codes` | Effect-local |
  | `effect-executable-unsupported` | `reason_codes` | Effect-local |
  | `operation-missing` | `reason_codes` | Names the one **otherwise-representable** effect for which an operation was owed and none was assigned. It is a statement about that effect's own coverage gap, never about the recipe as a whole |
  | `operation-not-reclassifiable` | `reason_codes` | Names each effect the unreclassifiable operation covers |
  | `parent-created-target-unsupported` | `reason_codes` | Names the one effect whose preimage depends on a parent |
  | `path-unsafe` | `reason_codes` | Names the one effect whose path is unsafe |
  | `preimage-unavailable` | `reason_codes` | Names the one effect whose preimage could not be observed |
  | `postimage-unavailable` | `reason_codes` | Names the one effect whose postimage could not be observed |

  A decoder that finds a `reasons`-class code inside an effect's
  `reason_codes`, or an effect-class code inside `reasons`, refuses the record.
- **`disposition` is a function of `reason_codes`, not an independent
  editorial choice.** rev-3 pinned four closed values and never said which
  reason produces which, so three of the four were unreachable by any stated
  rule and a record could carry `unsupported` beside `operation-missing`
  without contradiction. The mapping is exact and strictly validated:

  | `disposition` | Holds exactly when |
  |---|---|
  | `represented` | `reason_codes` is `[]` — and, conversely, an empty `reason_codes` forces `represented` |
  | `mismatch` | `reason_codes` is **exactly** `["operation-missing"]`. rev-7's trigger makes that the only array `operation-missing` can appear in: the code is raised only for an effect that is otherwise representable, which is precisely an effect no other effect-local condition holds for |
  | `ambiguous` | `operation-missing` is absent **and** `reason_codes` contains `preimage-unavailable` or `postimage-unavailable` — which is also the class every `object_kind: unknown` / `content_kind: unknown` effect lands in, since those axes require one of those two reasons |
  | `unsupported` | `reason_codes` is non-empty and none of the above applies — that is, every remaining effect-local code: any `effect-*-unsupported`, `operation-not-reclassifiable`, `parent-created-target-unsupported` or `path-unsafe` |

  **`operation-missing` never co-occurs with another effect-local code, and a
  record that pairs them is refused at decode.** The pairing is not a severity
  question to be settled by precedence; it is a contradiction, because
  `operation-missing`'s raising condition requires the absence of every other
  effect-local condition. A validator that accepts
  `["effect-binary-unsupported", "operation-missing"]`, or
  `["operation-missing", "preimage-unavailable"]`, is accepting a record no
  conforming producer can emit.

  **Multiple effect-local reasons are still resolved by a deterministic
  precedence**, highest first: `mismatch`, then `ambiguous`, then
  `unsupported`. The ladder is retained because it makes the mapping total for
  every array shape a decoder may be handed, but with the rev-7 trigger its
  live cases are the lower two: an effect carrying `postimage-unavailable`
  **and** `effect-rename-unsupported` is `ambiguous`, and an effect carrying
  `effect-binary-unsupported` and `effect-rename-unsupported` is `unsupported`.
  rev-6's worked example — `operation-missing` **and** `preimage-unavailable`
  resolving to `mismatch` — describes a shape rev-7 refuses outright, and is
  replaced by that refusal. The full sorted `reason_codes` array is published
  either way: precedence chooses the single `disposition` value, it never drops
  a reason.

  A record whose `disposition` contradicts its `reason_codes` under this table
  is refused at decode, in both directions: `represented` with a non-empty
  array, a non-`represented` value with an empty array, and any value the
  precedence rule does not select for that array.
- **`coverage_status` is defined by an iff, not by a convention.** The
  predicate list below is **canonical**: the companion PRD reproduces it
  byte-identically under a parity guard, and neither document may state a
  narrower or wider one. Every cross-reference to a predicate anywhere in
  either document uses these numbers.

  `coverage_status` is `complete` **if and only if** every one of the following
  holds:

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

  `coverage_status` is `incomplete` **if and only if** any of those fails —
  equivalently, whenever `reasons` is non-empty or any effect is
  non-`represented` or carries a non-empty `reason_codes`.

  **A contradictory record does not decode.** The strict decoder refuses
  `coverage_status: complete` together with a non-empty `reasons`, a
  non-`represented` effect, a non-empty `reason_codes`, an **empty `effects`
  array**, an effect carrying `object_kind: unknown` or
  `content_kind: unknown`, `patch_present: false`,
  `recipe_present: false`, `recipe_decodable: false`, an effect whose required
  side is unobserved, or `reference.kind` other than `commit`. It equally
  refuses `coverage_status: incomplete` with all of `reasons` empty, every
  effect `represented`, at least one effect present, `patch_present: true`,
  `recipe_present: true`, `recipe_decodable: true`, every required side
  observed and a `commit` reference. There is no encoding of "complete, but…",
  and — by predicate 1 — no encoding of "complete over nothing".
- `contextual_hint` is a closed, explicitly non-authoritative advisory:
  `none` or `additive-text`. It records that GH #13 *may* be able to derive an
  ephemeral anchor for this effect. It never affects `coverage_status`,
  `cross_base_status`, `disposition`, apply behavior or eligibility, and a
  consumer that reads it as permission is wrong. There is no other placement
  for contextual advice in v1: it is this field, on the effect, and nothing
  else.
- `cross_base_status` is producer scope, not consumer eligibility. It states
  how far the producer's own authority reaches, and is derived
  deterministically:
  - `unsupported` when `coverage_status` is `incomplete` — generation is not
    complete, so no cross-base statement exists. This covers the absent-patch
    and empty-patch records, which are always incomplete and therefore never
    reach the two complete branches;
  - `consumer-derivation-required` when coverage is complete and at least one
    represented effect is a whole-file write over a pre-existing file. A
    whole-file postimage write is never cross-base safe: replayed against a
    different base it would clobber unrelated upstream edits to that file;
  - `reference-tree-only` when coverage is complete and every represented
    effect is a creation carrying the explicit-empty-preimage gate, so the
    recipe's own preconditions already constrain it and no external
    derivation is required. Because a complete record has at least one effect
    (predicate 1), this branch is never reached vacuously by
    an empty effect list.

  GH #13 never treats any value as authorization. `reference-tree-only` means
  "the producer needs nothing further from a consumer", not "eligible".

#### Closed vocabularies

| Field | Closed values |
|---|---|
| `schema_version` | `1` |
| `producer` | `record`, `feature-patch-amend`, `reconcile-accept`, `cycle`, `apply-done`, `implement`, `artifact-edit` |
| `reference.kind` | `commit`, `index-snapshot`, `unavailable` |
| `capture.mode` | `working-tree-all`, `staged-index`, `unstaged-worktree`, `committed-range`, `auto-committed-range`, `explicit-committed-range`, `reconcile`, `no-capture` |
| `coverage_status` | `complete`, `incomplete` |
| `cross_base_status` | `reference-tree-only`, `consumer-derivation-required`, `unsupported` |
| `effects[].change_kind` | `add`, `modify`, `delete`, `rename`, `copy` |
| `effects[].content_kind` | `text`, `binary`, `none`, `unknown` |
| `effects[].object_kind` | `regular`, `executable`, `symlink`, `gitlink`, `unknown` |
| `effects[].old_mode`, `effects[].new_mode` | `100644`, `100755`, `120000`, `160000`, `""` |
| `effects[].disposition` | `represented`, `unsupported`, `ambiguous`, `mismatch` |
| `effects[].contextual_hint` | `none`, `additive-text` |
| `effects[].reason_codes[]` | the effect-local subset of the closed reason-code list below; `[]` exactly when the disposition is `represented` |
| `reasons[]` | the record-level subset of the closed reason-code list below |

#### Closed v1 reason codes

| Reason code | Array | Raised when |
|---|---|---|
| `canonical-patch-missing` | `reasons` | `patch_present` is `false`: no canonical patch exists to cover |
| `canonical-patch-empty` | `reasons` | `patch_present` is `true` and the canonical patch is **semantically empty** — zero bytes, whitespace-only bytes, or a strict parse yielding zero normalized effects; the raw patch hash is still bound and `effects` is `[]` |
| `canonical-patch-unparseable` | `reasons` | `patch_present` is `true`, the canonical patch is non-empty, and the strict effect grammar refuses it; the raw patch hash is still bound and `effects` is `[]` |
| `reference-not-durable` | `reasons` | The capture's reference kind is not `commit` |
| `recipe-undecodable` | `reasons` | `recipe_present` is `true` and the raw recipe bytes do not strict-decode; the raw recipe hash is still bound |
| `effect-delete-unsupported` | `reason_codes` | `change_kind` is `delete` |
| `effect-rename-unsupported` | `reason_codes` | `change_kind` is `rename` |
| `effect-copy-unsupported` | `reason_codes` | `change_kind` is `copy` |
| `effect-binary-unsupported` | `reason_codes` | `content_kind` is `binary` |
| `effect-mode-only-unsupported` | `reason_codes` | `change_kind` is `modify`, **both sides were observed and present**, both content hashes are equal, and `old_mode` differs from `new_mode`. Two unobserved sides carry `""` hashes and `""` modes and do not satisfy it — an unobserved effect is `unknown`/unavailable, never a proven mode-only change |
| `effect-symlink-unsupported` | `reason_codes` | Either mode is `120000` |
| `effect-gitlink-unsupported` | `reason_codes` | Either mode is `160000` |
| `effect-executable-unsupported` | `reason_codes` | Either mode is `100755`, including an executable add |
| `operation-not-reclassifiable` | `reason_codes` | A preserved recipe operation has no exact present-state reclassification in v1 (`append-file`, and `replace-in-file` outside a proven exact-postimage case) |
| `parent-created-target-unsupported` | `reason_codes` | The operation's correct preimage depends on a path a hard parent creates, and v1 has no field that can carry that authority |
| `producer-patch-rewrite` | `reasons` | A governed producer rewrote the canonical patch on this run **and** left an on-disk recipe that neither exactly covers nor simulates the patch just written (D15) |
| `recipe-not-regenerated` | `reasons` | The present recipe was not regenerated against the rewritten patch and no longer exactly covers it (D15) |
| `manual-bound-artifact-edit` | `reasons` | A bound artifact was mutated out of band through `tpatch edit` and the producer cannot truthfully recompute coverage for the result (D15 P7) |
| `path-unsafe` | `reason_codes` | The effect path fails repository path safety |
| `preimage-unavailable` | `reason_codes` | The preimage side was not observed (`preimage_observed: false`), so its bytes could not be read. It is raised for **every** unobserved preimage, extant or not, and is additionally the mandatory company of an `object_kind` / `content_kind` of `unknown` that an unobserved **extant** preimage forced. Its presence alone never implies an `unknown` axis |
| `postimage-unavailable` | `reason_codes` | The postimage side was not observed (`postimage_observed: false`), so its bytes could not be read. It is raised for **every** unobserved postimage, extant or not, and is additionally the mandatory company of an `object_kind` / `content_kind` of `unknown` that an unobserved **extant** postimage forced. Its presence alone never implies an `unknown` axis |
| `operation-missing` | `reason_codes` | The effect is **otherwise representable in v1** — its path is repository-safe, its `change_kind` / `content_kind` / `object_kind` / mode combination is inside the supported domain of D5, every side its `change_kind` names was observed, no `parent-created-target-unsupported` condition holds, and no operation assigned to it is `operation-not-reclassifiable` — **and** the decoded recipe assigns it no operation. Equivalently: no other effect-local condition holds for this effect and its `operation_indexes` is empty. It is **never** raised because an operation was deliberately not emitted for an effect that already carries a capability, safety or availability exclusion — nothing was owed there. An absent, unreadable or undecodable recipe leaves every effect with empty `operation_indexes`, so it raises this code on the otherwise-representable effects **only** |
| `operation-surplus` | `reasons` | An operation is assigned to no effect |
| `simulation-mismatch` | `reasons` | Simulating the operation set does not reproduce the exact postimage set |
| `recipe-owner-mismatch` | `reasons` | The **decoded recipe's** `feature` differs from the coverage record's `feature`. It fails predicate 3 and makes valid coverage `incomplete`. A coverage envelope whose own `feature` differs from the requested target slug is a different condition entirely — a D9 binding failure surfaced as `recipe-coverage-owner-mismatch` at rung 2 — and raises no schema reason (D13) |
| `recipe-stale-marker-present` | `reasons` | `recipe-stale.json` exists beside the recipe |

The list is closed: a decoder rejects any other value, rejects a code written
into the wrong array, and the implementation PRD may not add one. Splitting an
existing reason into narrower codes is a schema change requiring an ADR
revision. Unsupported effects may never be collapsed into `represented`.

**`producer-patch-rewrite` is conditional, not automatic.** rev-2 defined it as
"a governed producer other than a regenerating one rewrote the canonical patch
on this run", which made it unconditional on P3/P4/P5 — every such run would
have been permanently incomplete even when the recipe on disk still covered the
new patch exactly. The v1 rule is:

- the code is raised **only** when the patch rewrite leaves the on-disk recipe
  **not regenerated** and **unable to exactly cover and simulate** the patch
  the producer just wrote;
- when it is raised, `recipe-not-regenerated` is raised with it. The two codes
  describe one event from two angles — the producer rewrote the patch, and the
  recipe was not brought along — and neither is emitted alone for that event;
- a rewrite whose existing recipe still fully covers, simulates and
  reclassifies against the new patch raises **neither** code, and that record
  may be `complete` if the rest of the predicate holds.

That last case is real: a `cycle` or `apply --mode done` capture that produces
byte-identical patch content, or content whose derived recipe is unchanged,
leaves a recipe that still covers it exactly.

Because the axes are orthogonal, one effect commonly carries several codes.
The canonical examples are exact:

| Effect | Axes | `reason_codes` (sorted) |
|---|---|---|
| Binary rename | `change_kind: rename`, `content_kind: binary`, `object_kind: regular` | `["effect-binary-unsupported", "effect-rename-unsupported"]` |
| Symlink delete | `change_kind: delete`, `content_kind: text`, `object_kind: symlink`, `old_mode: 120000`, `new_mode: ""` | `["effect-delete-unsupported", "effect-symlink-unsupported"]` |
| Executable rename | `change_kind: rename`, `object_kind: executable`, both modes `100755` | `["effect-executable-unsupported", "effect-rename-unsupported"]` |
| Executable add | `change_kind: add`, `object_kind: executable`, `new_mode: 100755` | `["effect-executable-unsupported"]` |
| Pure mode change `100644`→`100755` | `change_kind: modify`, equal content hashes | `["effect-executable-unsupported", "effect-mode-only-unsupported"]` |
| Gitlink modify | `object_kind: gitlink`, `content_kind: none`, modes `160000` | `["effect-gitlink-unsupported"]` |
| Pure rename, text content unchanged | `change_kind: rename`, `content_kind: text` from the observed sides, `object_kind: regular`, both modes `100644` | `["effect-rename-unsupported"]` |
| Regular → symlink transition | `change_kind: modify`, `old_mode: 100644`, `new_mode: 120000`, `object_kind: symlink` from the postimage side | `["effect-symlink-unsupported"]` |
| Regular → gitlink transition | `change_kind: modify`, `old_mode: 100644`, `new_mode: 160000`, `object_kind: gitlink`, `content_kind: none` | `["effect-gitlink-unsupported"]` |
| Gitlink → regular transition | `change_kind: modify`, `old_mode: 160000`, `new_mode: 100644`, `object_kind: regular` from the postimage side, `content_kind` by the observed bytes | `["effect-gitlink-unsupported"]` |
| **No-capture pure rename** (D15 P6: neither side observed) | `change_kind: rename` from the record header, `object_kind: unknown`, `content_kind: unknown`, both modes `""`, both `*_observed: false` | `["effect-rename-unsupported", "postimage-unavailable", "preimage-unavailable"]` |
| **No-capture mode-only change** (neither side observed) | `change_kind: modify`, `object_kind: unknown`, `content_kind: unknown`, both modes `""` — the producer cannot even confirm the modes differ, so `effect-mode-only-unsupported` is **not** raised | `["postimage-unavailable", "preimage-unavailable"]` |
| **No-capture type transition** (a header-declared `100644` → `120000`, neither side observed) | `change_kind: modify`, `object_kind: unknown` — the symlink side was never read, so no kind is inferred across the transition — `content_kind: unknown`, both modes `""` | `["postimage-unavailable", "preimage-unavailable"]` |
| **Half-observed add** (postimage observed and present, preimage never looked at) | `change_kind: add`, `object_kind` **and** `content_kind` from the observed postimage — `text`, or `binary` under a marker or a NUL — because the postimage is the only **extant** side an `add` has; the unobserved preimage is not a side the classification rests on | `["preimage-unavailable"]` |
| **Half-observed delete** (preimage observed and present, postimage never looked at) | `change_kind: delete`, `object_kind` **and** `content_kind` from the observed preimage on the same terms; the postimage of a `delete` is not extant, so leaving it unobserved degrades no axis | `["effect-delete-unsupported", "postimage-unavailable"]` |
| **Otherwise-representable modify the recipe does not cover** (regular `100644` both sides, both sides observed and present, safe path, no assigned operation) | `change_kind: modify`, `content_kind: text`, `object_kind: regular`, both modes `100644`, both sides observed and present, `operation_indexes: []` | `["operation-missing"]` — the single positive case, `disposition: mismatch` |

**The positive and negative `operation-missing` cases are pinned together.**
The last row above is the only shape that raises it: an operation was owed for
a fully representable effect and the decoded recipe supplies none. The rows
above it are the negatives and they do **not** gain the code, in any record
state:

- an **unsupported** effect — the binary rename, symlink delete, executable
  rename or add, pure mode change, gitlink, type-transition and delete rows —
  keeps exactly its own `effect-*-unsupported` codes and stays `unsupported`.
  v1 deliberately emits no operation for it, so none was owed;
- an **ambiguous** effect — the no-capture rename, mode-only and
  type-transition rows and the two half-observed rows — keeps exactly its
  availability codes (plus any capability code it also earns) and stays
  `ambiguous`. A side nobody observed cannot make an operation owed;
- the same holds when the recipe is **absent, unreadable or undecodable**:
  every effect's `operation_indexes` is empty, and only the
  otherwise-representable effects raise `operation-missing`. The record-level
  cause is carried by `canonical-patch-*` / `recipe-undecodable` in `reasons`,
  never by promoting every effect to `mismatch`.

Contextual classification has no reason code. Advisory contextual information
is carried only by `effects[].contextual_hint`; the *incompleteness* caused by
a contextual operation that cannot be reclassified is reported by
`operation-not-reclassifiable`.

### D4 - Coverage completeness is not replay eligibility

`coverage_status: complete` means exactly D3's canonical ten-predicate iff and
nothing more. D3 owns the list; this decision does not restate it in different
words, because two prose renderings of one predicate set is exactly how rev-3
ended up with a wider list in the ADR and a narrower one in the PRD. The
summary a reader needs is that completeness spans four dimensions and each is
numbered in D3:

- **artifact reality** — a present, non-empty, strictly parsed patch and a
  present, decodable, correctly owned recipe (predicates 1 and 3);
- **reference durability** — a `commit` reference (predicate 2);
- **explanation totality** — every effect present exactly once, every
  operation assigned, all paths safe and unique, and no reason of either class
  outstanding (predicates 4-7);
- **observed, reproducible behavior** — every required side actually observed,
  exact simulation within the D5 mode domain, and a stable already-present
  reclassification (predicates 8-10).

rev-2 asserted only the explanation-totality dimension, which allowed a record
with a non-durable reference, an absent recipe or an absent patch to satisfy
"complete" on a reading of the prose. rev-3 added artifact presence and
durability but still had two divergent lists. D3's canonical list closes both
gaps in one place.

It does not mean the recipe can replay against a different upstream tree, and
it says nothing about apply-time severity: a superseded feature can carry
complete coverage and still be excluded from default effective replay under
D7. GH #13 recomputes patch, recipe, owner, reference and effect bindings and
then applies its own eligibility rules. Generated-by-tpatch is never a trust
shortcut, and neither is `cross_base_status` nor the `producer` field.

**The converse also holds, and is load-bearing given the severity ladder:
a warning never authorizes replay.** GH #13 independently hard-refuses
missing coverage, malformed coverage and `coverage_status: incomplete`, and
refuses any feature carrying `recipe-stale.json`, regardless of the fact that
verify grades all four as warning-class with exit `0` (D13). Producer
completeness is necessary and never sufficient; producer *absence* is
unconditionally insufficient.

Patch-generation identifiers may appear in diagnostics as advisory context.
Readers trust recomputed patch and recipe hashes, not a generation pointer.

### D5 - Generate safe whole-file preimages only, over the supported mode domain

The supported effect domain for v1 generation and simulation is **regular
`100644` files only**, on both sides of the effect.

For a new regular `100644` file, a generated `write-file` carries an explicit
empty preimage, meaning the path must be absent. For an existing regular
`100644` file with a durable preimage, it carries `sha256:<64 lowercase hex>`
over exact bytes.

A capture without a durable reference may retain legacy recipe behavior for
compatibility, but its coverage is incomplete and GH #13 cannot authorize it.
The generator never fabricates a commit for an index-relative snapshot.

**An absent or empty canonical patch is an explicit incomplete state, never a
silent absence and never a vacuous completeness.** Coverage describes how a
recipe explains a patch, so with no patch there is nothing to explain:

| Canonical patch | Record shape |
|---|---|
| absent | `patch_present: false`, `patch_sha256: ""`, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `reasons` containing `canonical-patch-missing` |
| present, **semantically empty** — zero bytes, whitespace-only bytes, or a strict parse yielding zero normalized effects | `patch_present: true`, `patch_sha256` = the digest of the exact raw patch bytes (the empty-byte-string digest in the zero-byte case), `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `reasons` containing `canonical-patch-empty` |
| present, non-empty, strict grammar refuses it | `patch_present: true`, `patch_sha256` = the digest of the exact raw patch bytes, `effects: []`, `coverage_status: incomplete`, `cross_base_status: unsupported`, `reasons` containing `canonical-patch-unparseable` |
| present, strictly parsed into **at least one** normalized effect | the ordinary derivation |

The same shape rule governs the recipe side, and for the same reason:

| Recipe file | Record shape |
|---|---|
| absent, **or present but unreadable** | `recipe_present: false`, `recipe_decodable: false`, `recipe_sha256: ""`, all `operation_indexes` empty, `coverage_status: incomplete`. Readable existence collapses the two physical states at the record layer; the read-error diagnostic keeps the real cause |
| present, does not strict-decode | `recipe_present: true`, `recipe_decodable: false`, `recipe_sha256` = the digest of the exact raw recipe bytes, all `operation_indexes` empty, `coverage_status: incomplete`, `reasons` containing `recipe-undecodable` |
| present, strict-decodes | `recipe_present: true`, `recipe_decodable: true`, ordinary operation assignment |

The empty-patch row exists because "every effect is represented" is vacuously
true over an empty effect list, and "every represented effect is a creation"
is vacuously true too. Left unstated, a semantically empty patch would have
published `complete` + `reference-tree-only` — the single most permissive
record the schema can express — for a feature that changes nothing. D3's
predicate 1 now requires **at least one** normalized effect, so the vacuous
reading is unreachable in the predicate as well as in this table. **Emptiness
is semantic, not byte-length**: zero bytes, whitespace-only bytes and a clean
strict parse yielding zero effects are the same dead end and carry the same
`canonical-patch-empty` code.

The unparseable rows exist for the mirror-image reason. A producer holding a
patch or a recipe it cannot interpret has exactly three dishonest options and
one honest one: it can claim the artifact is absent, it can publish an effect
or operation list it did not derive, it can publish nothing at all, or it can
bind the raw bytes it does hold and say plainly that it could not read them.
Only the last is truthful, and it is the only one this table permits. **A
strict parse or decode refusal never leaves a feature with no coverage after a
producer event.**

**A producer that has no canonical patch publishes this state rather than
publishing nothing.** `implement` (D15 P6) is the ordinary case: it writes a
recipe before any patch exists, so its published coverage is
`patch_present: false` with `canonical-patch-missing`. That record is truthful,
consumer-ineligible and repairable — a later `record` run for the same feature
recomputes coverage against the patch it captures and replaces it. The
alternative, publishing nothing, is the state D15 exists to eliminate: an
observer cannot distinguish "no producer has run" from "a producer ran and
declined to say".

The following are incomplete in v1. The generator does not emit a
destination-only write, a mode-dropping write or a partial recipe and call it
complete. Cohorts are stated on the axis that disqualifies them, and an effect
that is disqualified on more than one axis carries every applicable code:

| Cohort | Axis | Reason code(s) | Why v1 cannot represent it |
|---|---|---|---|
| delete | `change_kind: delete` | `effect-delete-unsupported` | No delete operation exists |
| rename | `change_kind: rename` | `effect-rename-unsupported` | A destination-only write loses the source removal |
| copy | `change_kind: copy` | `effect-copy-unsupported` | No copy operation exists |
| binary | `content_kind: binary` | `effect-binary-unsupported` | `write-file` carries a JSON string body |
| pure mode change | `change_kind: modify`, equal content hashes, differing modes | `effect-mode-only-unsupported` plus the out-of-domain mode's own code | No operation changes a mode |
| symlink | `object_kind: symlink` (`120000`) | `effect-symlink-unsupported` | `write-file` writes a regular file |
| gitlink | `object_kind: gitlink` (`160000`), `content_kind: none` | `effect-gitlink-unsupported` | Submodule pointers are not file bytes |
| **executable add or executable side (`100755`)** | `object_kind: executable` | `effect-executable-unsupported` | `executeOperation` writes every `write-file` target with a fixed `0o644` mode (`internal/workflow/recipe.go:211`), so simulation cannot reproduce `100755`. This applies to an executable *add* exactly as it does to an executable modify: the byte content would be right and the mode would be silently wrong |
| unsafe/unreadable/ambiguous path | path | `path-unsafe` / `preimage-unavailable` / `postimage-unavailable` | Path safety and observation of every required side must both hold |
| **unobserved side** | `object_kind: unknown` and/or `content_kind: unknown` **only when the unobserved side is extant for the `change_kind`**; otherwise the axes keep the observed extant side's values | `preimage-unavailable` and/or `postimage-unavailable` for **every** unobserved side, extant or not | The producer took no capture of that side (D15 P6's `no-capture` mode is the ordinary case), so it can state what the record header says happened but not what an unread object or its content is. An unobserved **non-extant** side (an `add`'s preimage, a `delete`'s postimage) leaves the axes definite and still raises its unavailable reason. Either way the disposition is `ambiguous` and predicate 8 fails; `unknown` never decodes on a complete record |
| absent, empty or unparseable canonical patch | record | `canonical-patch-missing` / `canonical-patch-empty` / `canonical-patch-unparseable` | There is no coverable patch — absent, semantically empty (zero effects), or refused by the strict grammar — so no completeness claim can be made |
| present but undecodable recipe | record | `recipe-undecodable` | The raw bytes are bound, but no operation set can be assigned to any effect |
| patch rewritten, recipe left uncovering | producer | `producer-patch-rewrite`, `recipe-not-regenerated` | The recipe on disk neither covers nor simulates the patch the producer just wrote (D15) |
| bound artifact edited out of band | producer | `manual-bound-artifact-edit` | The producer observed a mutation it cannot truthfully attribute or recompute (D15 P7) |

**No effect-level cohort in this table raises `operation-missing`.** Each of
them is a case in which v1 deliberately emits **no** operation, so no operation
was ever owed and the effect's `reason_codes` are exactly the capability,
safety or availability codes its own row names (D3). `operation-missing` is the
complement of this table: it is raised only for an effect that this table does
**not** exclude — a regular `100644` effect on both sides, inside the supported
domain, with a safe path, every side its `change_kind` names observed, no
parent-created dependency and no non-reclassifiable assigned operation — for
which the decoded recipe nonetheless supplies no operation. A withheld or
absent recipe therefore produces `operation-missing` on the representable
effects and leaves every excluded cohort above carrying exactly its own codes.

A **preserved** recipe — one the producer did not generate, kept because it is
richer than a derivable whole-file recipe — is judged as it stands. If it
contains any operation that v1 cannot exactly reclassify against the present
state, its coverage is incomplete with `operation-not-reclassifiable`:

- `append-file` appends unconditionally
  (`internal/workflow/recipe.go:230-242`), so no present-state check can
  distinguish "already appended" from "must append again";
- `replace-in-file` writes the first substring match
  (`internal/workflow/recipe.go:213-228`), so it has no exact present-state
  reclassification outside a case where the postimage is provably already
  exact.

This changes neither operation's schema nor its execution semantics. Existing
`replace-in-file` and `append-file` behavior is byte-compatible after GH #15;
the only new statement is that a recipe containing them cannot be
coverage-complete in v1.

**The no-partial-recipe policy is retained, and its apply-side consequence is
now specified.** When a *fresh* derivation is incomplete and no recipe exists,
the producer publishes no `apply-recipe.json`: a partial recipe that silently
omits a delete, a rename or an executable mode is exactly the quiet loss GH #15
removes. When regeneration of an *existing* recipe would be incomplete, the
existing recipe is preserved byte-identical and is never overwritten by the
partial derivation. Both halves are unchanged from rev-2.

What rev-2 left unsaid is what `apply --mode execute` then does, since the
feature now has coverage but no recipe. D17 answers it — and, from rev-5,
answers the whole state space around it: a **total** classifier over every
coverage/recipe shape, of which "coverage binds no readable executable recipe"
is one case and a named refusal rather than a silent fallback.

### D6 - Record autogeneration writes truthful recipe provenance, convergently

New record autogeneration is a real generation event. For a preimage-bearing
recipe with a durable commit reference, record writes truthful
`recipe-provenance.json` carrying that actual base commit, generation time and
recipe hash.

**One convergence rule governs every rerun.** Whenever all of the following
hold, record writes or repairs provenance:

1. the feature's recipe is **record-generated under D16** — this run's freshly
   derived canonical recipe bytes equal the on-disk recipe bytes exactly — and
   the recipe carries at least one non-nil `preimage_hash`;
2. the capture has a durable truthful reference (`reference.kind: commit`);
3. matching provenance is missing, undecodable, or stale — its `base_commit`
   or `recipe_sha256` does not match what the producer would truthfully write
   now.

Condition 1 is **proved on this run, not remembered from a previous one**. No
stored label, marker or action name is consulted; see D16.

This applies on **generated, regenerated, and no-op/preserved-generated**
reruns alike. The recipe-noop path is not exempt: a run that changes no recipe
byte still repairs missing or stale provenance for a record-generated recipe.
This is what makes a rerun a real recovery operation after a crash between the
recipe write and the provenance write.

Existing provenance bytes and timestamp are preserved **only** when matching
provenance already exists. Record does not manufacture a new timestamp for an
unchanged, already-truthful sidecar.

A **preserved manual or provider** recipe is different and is not covered by
this rule. Record does not fabricate provenance for a recipe whose bytes its
own derivation does not exactly reproduce (D16): it cannot truthfully state
which base or time that recipe was authored against. Such a recipe remains
governed by GH #19 (historical/manual adoption) and by verify's existing V10
requirement. GH #19 remains the owner of that surface; GH #15 does not consume
it.

If the capture has no durable commit reference, record does not write
commit-shaped provenance for any recipe class. Coverage remains incomplete
with `reference-not-durable`.

### D7 - Newly generated write-file operations gain three-state apply classification

**Classification happens first, for every preimage-bearing `write-file`, on
every feature.** The all-or-nothing precheck classifies each operation into
exactly one of three states:

1. exact postimage already present — whichever preimage authority the
   operation carries, a non-empty expected hash or a non-nil empty preimage:
   `already-present`, no write;
2. exact expected preimage present, or explicit-empty preimage with an absent
   target: `applicable`;
3. any other state: `drift`.

**Severity of the third state follows supersession and is unchanged from
ADR-029 D7.** Classification is new; severity is not.

| Feature status | Third-state (`drift`) result | Effective replay |
|---|---|---|
| effective / non-superseded | refusal-class: no operation from the recipe is written, the whole recipe refuses before any mutation (ADR-029 D3) | recipe does not execute |
| superseded by an active superseder (healthy or stale) | warning-class with the existing "superseded by `<slug>`" downgrade note; execution proceeds exactly as it does today (`internal/workflow/writefile_safety.go:264-305,326-364`) | **default effective replay already excludes the superseded feature** (`docs/adrs/ADR-028-supersession-edge-model.md:77-88`); the warning is an audit signal about historical drift |

The superseded warning is **not** a statement that applying that superseded
recipe explicitly is safe, and it is **not** a statement that its coverage or
replay is safe. It records that a historical feature has expected historical
drift and that the active superseder — not this recipe — carries the current
semantics. An operator who explicitly applies a superseded, drifted recipe is
overriding an audit signal, and the diagnostic says so. Coverage severity and
replay eligibility are answered by D4, D9 and GH #13, never by the presence of
a downgraded warning.

Path-safety refusals are never downgraded, on any feature, in any state
(`internal/workflow/writefile_safety.go:270-278`).

This makes repeated apply idempotent without weakening preimage protection.
Shared-path stacks succeed only in the captured dependency order.

**Amendment to ADR-029 D3, exact-postimage recognition only.** ADR-029 D3
lists four refusal cases. Exact-postimage recognition narrows **two** of them,
and only where the bytes on disk are byte-exactly the operation's generated
postimage:

| ADR-029 D3 refusal case | ADR-036 effect |
|---|---|
| empty preimage, file exists | **narrowed**: a **non-nil empty** preimage whose existing file is byte-exactly the generated postimage is `already-present` and no write occurs; any other existing value remains a collision and refuses under the severity ladder above |
| expected hash present, file hash differs | **narrowed**: an operation whose expected preimage hash does not match, but whose target's observed bytes are byte-exactly that operation's postimage, is `already-present` and no write occurs; any other differing value remains drift and refuses under the severity ladder above |
| expected hash present, file missing | **unchanged**: still a refusal. A missing target is not an applied postimage, and nothing about exact-postimage recognition speaks to it |
| unreadable target needed for a precondition | **unchanged**: still a refusal, with the read error named |

rev-6 claimed "the other three ADR-029 D3 refusal cases … are unchanged",
which contradicted this decision's own classifier: state 1 above recognizes an
exact postimage for **every** preimage-bearing `write-file`, non-empty expected
hashes included, so the expected-hash-mismatch case was already narrowed. The
claim is deleted and the rows are aligned with the classifier.

**What is not amended, and is not narrowed by anything above**: ADR-029 D3's
all-or-nothing precheck atomicity (every precondition is still evaluated before
the first write, and one refusal still means no operation is written),
path-safety refusals, and ADR-029 D7's supersession severity. Apply and verify
mirror the two narrowed cases: verify's `preimageAtTree` branches
(`internal/workflow/verify_anchored.go:964-981`) gain the same exact-postimage
recognition and keep their failure for every other observed value.

**ADR-029 D7 is preserved in full and is not amended.** ADR-036 amends ADR-029
D3 only, and only for exact-postimage recognition.

`created_by` grants no exemption. An absent target declared `created_by` a
hard parent is gated exactly like any other empty-preimage target: if the path
is present with different bytes, the operation is drift. v1 has no schema
carrier for a parent's postimage authority (Rejected alternative 9), so a
generated or preserved operation whose correct preimage depends on a
parent-created path is **coverage-incomplete** with
`parent-created-target-unsupported`. The producer does not reuse a parent's
postimage as the child's preimage in v1.

Legacy omitted preimages keep ADR-029 D4's warning compatibility.

### D8 - Do not persist contextual anchors in v1, and say so in the acceptance

GH #15 does not change the schema or global semantics of `replace-in-file` or
`append-file`.

Three alternatives were evaluated:

1. globally require unique-match and present-first semantics for existing
   `replace-in-file`;
2. add a new operation type or authority field;
3. persist no new anchors and let GH #13 derive ephemeral anchors from the
   canonical patch against a named reference tree.

Option 3 is selected. Options 1 and 2 change legacy/provider/manual apply
semantics and all six skill schemas before the candidate consumer exists.
GH #13 may derive an anchor in memory, prove uniqueness and idempotent
reclassification, and emit it as candidate evidence without writing it back to
the recipe. GH #13 owns proving those anchors independently; GH #15 hands it
no anchor and no anchor-shaped claim.

**This rev-1 decision amends GH #15's planning acceptance explicitly: v0.17.0
does not persist an anchor of any kind.** That is a deliberate scope
reduction, not an omission, and it must be recorded as such rather than
discovered later by a consumer looking for an anchor field.

The consequence for the motivating fixture is stated exactly. For the adjacent
CLI argument case
(`docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md:1-243`),
GH #15 produces:

- `coverage_status: complete` — the whole-file write does exactly reproduce
  the captured postimage from the captured preimage, so generation is
  complete and correctly bound to that base;
- `cross_base_status: consumer-derivation-required` — the producer explicitly
  withholds cross-base authority. Replaying that whole-file write against an
  upstream tree where the neighboring arguments were deleted would restore
  them, which is precisely the failure the case study documents;
- `contextual_hint: additive-text` on the additive effect — advisory only.

No surface may present that whole-file write as cross-base safe. "Complete
coverage" and "safe to replay elsewhere" are different statements, and the
adjacent fixture is the canonical proof that they differ.

`contextual_hint` is the only carrier for contextual advice (D3). It never
changes `coverage_status` or `cross_base_status`.

### D9 - Recompute every binding at read time

A consumer accepts coverage only when it recomputes and matches:

- **the coverage envelope's own owner**: `coverage.feature` must equal the
  feature slug the consumer asked about. A mismatch is a **binding failure**,
  not an incompleteness reason — the record in hand describes some other
  feature, so nothing in it is authority for this one. It is surfaced as
  `recipe-coverage-owner-mismatch` at D13 rung 2 with `block` and exit `2`,
  refuses `apply --mode execute` at D17 order 1, and is hard-refused by
  GH #13. It maps to **no** schema reason, and a producer never writes one for
  it: the mismatch is a property of the pairing between a record and a
  request, and a record cannot describe its own misdelivery. The **recipe's**
  owner is a separate question, answered by predicate 3 and the record-level
  `recipe-owner-mismatch` reason (D3), which is warning-class incompleteness
  at rung 3 — the two conditions never share a severity or a code;
- **`patch_present` and `recipe_present`, recomputed from actual on-disk
  readable existence rather than believed.** rev-4 recomputed only the hashes,
  which caught one direction and missed the other. Both directions are
  binding-stale:
  - a record claiming `patch_present: true` or `recipe_present: true` beside a
    file that is absent or unreadable is **stale** — the deletion or tamper
    case;
  - a record claiming `patch_present: false` or `recipe_present: false` beside
    a file that **is** present and readable is **equally stale** — the record
    describes a feature state that no longer exists, and treating it as
    authoritative would let an out-of-band restore (or a later producer's
    write that failed to publish) masquerade as a producer's honest "I had
    nothing". Whenever the file is present, the raw-byte hash is recomputed and
    compared as well, so a `false`-beside-present record fails on presence and
    a wrong-hash record fails on content;
  - **the recomputed predicate is readable existence, and a readability
    transition is drift in whichever direction it happened.** A file that is
    present but unreadable recomputes to `false`, exactly as an absent one
    does (D3), so a record that truthfully published `false` for an unreadable
    file still binds while it stays unreadable, and becomes stale the moment it
    becomes readable. The mirror holds: a record that published `true` for a
    readable file becomes stale when the file stops being readable, whether it
    was deleted or had its permissions changed. The record layer's collapse of
    absence and unreadability never reaches the diagnostic, which names the
    actual cause — "no such file" or the real read error — so an operator is
    never told a present artifact is missing;
- exact canonical patch hash (or `patch_present: false` **proved** by an absent
  or unreadable patch, with `patch_sha256: ""`);
- exact recipe hash over the **raw bytes on disk**, whenever
  `recipe_present: true` — decodable or not — and `recipe_sha256: ""` exactly
  when `recipe_present: false`, itself proved by an absent or unreadable file;
- `recipe_decodable`, recomputed by attempting the strict decode itself rather
  than believing the flag;
- reference descriptor;
- capture descriptor;
- preimage-set digest;
- normalized effect count and per-effect `effect_sha256` digests.

Coverage is bound to **recomputed content hashes, never to generation
identity**. A patch-generation ID, the `producer` field, a
`generated`/`regenerated` action label or any other provenance pointer is
advisory context only and can never substitute for a recomputed hash. Content
hashes are recomputed at read time from the artifacts on disk, not read back
out of the coverage record.

`recipe-stale.json`, a coverage-envelope owner mismatch, missing coverage,
unknown fields,
malformed arrays or any mismatch make the artifact unusable **as consumer
authority**. No success-shaped fallback is permitted. Unusable-as-authority is
a separate question from verify severity, which D13 pins: a missing coverage
file and a present stale marker are both warning-class in verify and both
consumer-ineligible, and neither ever authorizes replay.

### D10 - Publish coverage last and make rerun the recovery

The producer computes every output in memory first, then publishes:

1. recipe, when generation/regeneration is authorized;
2. truthful recipe provenance, when D6 requires it;
3. patch generation metadata;
4. coverage, last.

Each file uses the repository's atomic single-file writer. Cross-file atomicity
is not claimed. Coverage-last means any interruption leaves missing or
hash-stale authority, which consumers reject.

**Coverage publication is last and unconditional on every governed producer
event** (D15), not only on `record`'s. For `record` that means every autogen
outcome — generated, regenerated, preserved manual/provider, preserved
generated, and recipe-noop — even when unchanged patch bytes append no patch
generation and no recipe byte changes. For P2-P5 it means every event that
writes or checkpoints the canonical patch, whether or not that producer touches
the recipe at all — including P2's non-writing category-(c) checkpoint. For P6
it means after the recipe and provenance writes, on **both** arms of its recipe
parse, at the **common coverage-last point both arms fall through to** — which
sits after the state-mark attempt and before `RunImplement` propagates its
final return. That final statement is
`return s.MarkFeatureState(slug, store.StateImplementing, ...)`
(`internal/workflow/implement.go:243`), not a literal `nil`, so the finalizer is
defined against the return itself rather than against a value: coverage is
published whether the state mark succeeded or failed, because the event being
recorded is the recipe write that already landed, and a feature left in the
previous state with a new recipe on disk is exactly the shape that most needs a
bound record. Error precedence is fixed and loses nothing — a state-mark error
is still returned to the caller, a coverage-publication failure is surfaced
rather than swallowed, and when both fail the returned error names both,
state-mark first, chained under the repository's ordinary wrapping so neither
cause is discarded. For P7 it means after the editor
returns having changed the bound artifact, whether or not the editor itself
succeeded. A rerun is the recovery
operation, and it repairs provenance under D6 in the same run.

**A coverage-publication failure is surfaced on every P1-P7 event, and never
degrades to success.** This is one contract over the whole registry, not seven
per-producer rules:

> When a governed producer event owes coverage and the shared publication API
> fails, the producing command **returns that failure to its caller and exits
> non-zero**. It may not log-and-continue, may not fall through to its ordinary
> success message, and may not treat "the real work already landed" as a reason
> to report success.

Two consequences are deliberate. First, because publication is **last**, a
failed publication can leave coverage **absent or stale** on disk — that is the
same recoverable state a crash leaves, and D9's recomputation plus D13's rungs
already reject it, so the artifact layer needs nothing new. Second, the
*command* must still fail: a run that wrote a recipe or a patch and then could
not record what it wrote has produced exactly the silently-stale state D15
exists to remove, and a zero exit would hide it. When another failure is
already in flight — a failed state mark, a returned editor error — both are
reported, in the fixed precedence above; the coverage failure never displaces
the primary error and is never dropped in its favour.

The acceptance matrix covers this with a **single table-driven row over
P1-P7**, one case per producer event, rather than seven prose rows.

### D11 - Doctor reports missing, stale and incomplete coverage as check `D10`

The GH #15 implementation adds doctor check **`D10`**, the next free ID after
the shipped `D1`-`D9` registry (`internal/workflow/doctor.go:233-245`). It
reports:

- missing coverage where a canonical patch and recipe exist;
- coverage that is present but could not be read, reported with its read error
  rather than as absence;
- malformed or hash-stale coverage;
- incomplete coverage and its sorted reason codes.

Doctor `D10` is **read-only**: every finding is warning-class, `Fixable` is
false, and the check acquires no lock and performs no write, backup or
normalization. `doctor --fix` does not act on it. Mechanical repair requires a
future accepted doctor contract; it is not in GH #15.

**A regeneration command is printed only after a dry derivation proves it can
succeed.** rev-2 said remediation names a command "only where that command can
truthfully reconstruct authority", which is a policy without a test — nothing
said how the check knows. The rule is now mechanical:

> Before printing `tpatch record <slug> --regenerate-recipe` (or any other
> regeneration command) as remediation, the producing surface performs a
> **dry derivation**: it re-derives effects and the candidate recipe from the
> canonical patch and the reconstructable reference, in memory, writing
> nothing. The command is printed **only if** that dry derivation would produce
> `coverage_status: complete`.

When the dry derivation would not produce a complete record — an unsupported
effect cohort, a non-durable or unreconstructable reference, an absent or empty
canonical patch, a preserved non-reclassifiable operation — the finding says
plainly that **no automatic regeneration can truthfully repair this feature**,
names the blocking reasons, and directs the operator to manual review and
remediation instead. It never offers a command that will fail or, worse, one
that will "succeed" into another incomplete record.

This applies identically to every surface that prints remediation: doctor
`D10`, verify's rung-3 remediation (D13) and the governed producers' own
incomplete-coverage output (D15). A surface that prints a regeneration command
without the dry-derivation proof fails the acceptance matrix.

### D12 - Ship GH #15 separately from GH #13

GH #15 changes record, apply and verify behavior and needs downstream soak.
It targets v0.17.0. GH #13 remains blocked until the producer contract is
accepted, implemented and soaked, and targets a later v0.18.0 release. The two
releases stay separate; no v0.17.0 build contains a GH #13 consumer.

### D13 - Verify gains check `recipe_generation_coverage` with a pinned severity ladder

The verify check ID is exactly **`recipe_generation_coverage`**, matching the
shipped snake_case frozen vocabulary (`internal/workflow/verify.go:47-70`).
Severity uses the shipped vocabulary `block` / `warn`
(`internal/workflow/verify.go:73-77`).

The row evaluates exactly one state, chosen by this **precedence ladder**,
highest first. The first matching rung wins; no lower rung can raise or lower
it:

| Rung | Coverage state | Severity | Check row | Verdict/exit implication |
|---|---|---|---|---|
| 1 | present but **unreadable**, malformed, or carrying an unknown field | `block` | fails with `recipe-coverage-malformed`, whose diagnostic names the read error when that is the cause | verdict `failed`, exit `2` |
| 2 | present but patch-hash, recipe-hash, reference-, capture- or **coverage-envelope-owner**-stale — including a presence flag whose recomputed readable existence differs from the record in either direction | `block` | fails with the matching binding code | verdict `failed`, exit `2` |
| 3 | present, valid, `coverage_status: incomplete` — including a record whose `reasons` carry `recipe-owner-mismatch` | `warn` | fails, remediation lists every reason code sorted ascending with affected paths | verdict stays `passed`, exit `0` (`internal/workflow/verify.go:624-637`) |
| 4 | present, valid, `complete`, with `recipe-stale.json` beside the recipe | `warn` | fails with `recipe-coverage-stale-marker` | verdict stays `passed`, exit `0` |
| 5 | absent — genuinely absent, not merely unreadable | `warn` | fails with `recipe-coverage-missing` | verdict stays `passed`, exit `0` |
| 6 | present, valid, `complete`, no stale marker | `block` | passes | contributes nothing to the verdict; exit unchanged |

**Two vocabularies, one one-way mapping, no dual names.** rev-2 used
`recipe-stale-marker-present` on rung 4 and `recipe-coverage-stale-marker` in
the PRD's surface table, which put two names for one condition inside what read
as one vocabulary. rev-3 fixed the naming but overcorrected into a claimed
**bijection** between the two vocabularies, which cannot exist: effect-local
schema reasons have no individual surface code and never will, because verify
and doctor report an *aggregate* incomplete row rather than one row per effect.
The correct statement is a one-way mapping over the mapped subset, plus
disjointness over the whole:

| Surface failure code | Schema reason it names | Layer note |
|---|---|---|
| `recipe-coverage-stale-marker` | `recipe-stale-marker-present` | rung 4 |
| `recipe-coverage-patch-missing` | `canonical-patch-missing` | rung 3, via the record's `reasons` |
| `recipe-coverage-patch-empty` | `canonical-patch-empty` | rung 3 |
| `recipe-coverage-patch-unparseable` | `canonical-patch-unparseable` | rung 3 |
| `recipe-coverage-recipe-undecodable` | `recipe-undecodable` | rung 3 |
| `recipe-coverage-manual-edit` | `manual-bound-artifact-edit` | rung 3 |
| `recipe-generation-not-regenerated` | `producer-patch-rewrite` + `recipe-not-regenerated` | producer output; one surface code names the pair, which is itself why a bijection is impossible |

**`recipe-coverage-owner-mismatch` is not in that table, and rev-5's mapping
of it onto `recipe-owner-mismatch` was wrong.** The two names denote two
different conditions with two different severities, and pairing them made one
condition simultaneously rung-2 blocking and rung-3 warning:

- **coverage envelope owner mismatch** — the coverage record's own `feature`
  differs from the slug the caller asked about. This is a **binding failure**
  (D9): the record describes another feature, so no field in it is authority
  here. It is rung 2, `block`, exit `2`, surface code
  `recipe-coverage-owner-mismatch`, and it refuses `apply --mode execute` at
  D17 order 1. It has **no** schema reason and never will: a record cannot
  encode the fact that it was handed to the wrong question, so
  `recipe-coverage-owner-mismatch` joins the unmapped binding-level codes
  below;
- **recipe owner mismatch** — the decoded recipe's `feature` differs from the
  coverage record's `feature`. This is a statement the record *can* make about
  artifacts it owns, so it is the record-level schema reason
  `recipe-owner-mismatch` (D3), it fails predicate 3, it makes otherwise-valid
  coverage `incomplete`, and it therefore surfaces at rung 3 through the
  **aggregate** `recipe-coverage-incomplete` row, `warn`, exit `0`, with
  `recipe-owner-mismatch` printed in that row's sorted reason list. It has no
  surface code of its own, exactly like every other aggregated reason.

The mapping is **one-way and partial**, and its subject is the **seven surface
codes listed in the table above** — not the whole surface vocabulary. Three
properties are required, and no fourth is claimed:

1. **Totality on the mapped side.** Every surface code in the table above
   names exactly one schema condition, and the same schema condition is never
   named by two surface codes. Surface codes **outside** that table —
   `recipe-coverage-malformed`, `recipe-coverage-patch-changed`,
   `recipe-coverage-recipe-changed`, `recipe-coverage-reference-stale`,
   `recipe-coverage-owner-mismatch`,
   `recipe-coverage-missing`, `recipe-coverage-incomplete`,
   `recipe-generation-incomplete`,
   `recipe-generation-provenance-unavailable`,
   `recipe-generation-origin-unproved` and
   `recipe-generation-no-truthful-regeneration` — name **verify-, binding- or
   aggregate-level conditions that have no schema-reason counterpart at all**:
   a decode failure, a recomputed-hash mismatch, a misdelivered record, an
   absent file, the aggregate incomplete row, a producer-side outcome. They are
   unmapped by construction, not by omission, and inventing schema reasons for
   them would put verify-layer conditions inside the artifact's vocabulary.
   The mapped subset therefore has **seven** members and the unmapped set
   **eleven**.
2. **Aggregation for the rest.** Every remaining schema reason — every
   effect-local `reason_codes` value, plus record-level
   `reference-not-durable`, `operation-surplus`, `simulation-mismatch` and
   `recipe-owner-mismatch` — has **no** individual surface code. It is reported inside the aggregate
   `recipe-coverage-incomplete` row, whose remediation prints every reason code
   sorted ascending. Inventing a surface code per effect-local reason is
   forbidden.
3. **Disjointness.** No token appears in both vocabularies — and this holds
   over the **whole** of both, mapped subset and unmapped remainder alike. A
   verify row may never fail with a schema reason token, and a coverage record
   may never carry a surface code in `reasons` or `reason_codes`.

Every mapped surface code follows the same shape, `recipe-coverage-<condition>`
or `recipe-generation-<condition>`; the schema reason is the D3 code.

**Rung 3 wins over rung 4, and does not hide the marker.** When a feature has
valid `incomplete` coverage *and* a `recipe-stale.json` beside its recipe, rung
3 is the row's outcome — the higher rung wins, as the ladder says. But the
marker is not thereby invisible: the coverage record's own `reasons[]` contains
`recipe-stale-marker-present` (D3), and rung 3's remediation prints **every**
reason code sorted ascending, so the marker appears in the rung-3 output. The
ladder chooses which row fires; it never suppresses a reason. A rung-3
remediation that omits a present stale-marker reason fails the acceptance
matrix.

**Missing coverage is uniformly warning-class.** rev-1 graded absence by
origin — `missing-legacy` warned, `missing-produced` blocked — which is not
implementable: the only carrier that could have said "this build produced
coverage here" is the coverage file itself, and it is gone. This decision
retires the `missing-produced` / `missing-legacy` split and asserts no marker
that does not exist. A single `recipe-coverage-missing` code covers both a
pre-v0.17.0 feature that never had coverage and a feature whose coverage was
deleted after this build wrote it.

The acknowledged consequence is stated plainly: **deleting coverage can reduce
verify's diagnostic severity, and it can never gain replay authority.** A
deleted coverage file drops the feature from rung 6 to rung 5, so verify says
`warn` instead of pass — but GH #13 independently hard-refuses missing
coverage (D4, D9), so the deletion converts a possibly-eligible feature into a
definitely-ineligible one. Absence is always ineligible; it can never be
laundered into permission. The PRD pins this with an explicit deletion test.

**`recipe-stale.json` presence is warning-class, not a new hard failure.**
rev-1 put the marker on a `block` rung, which would have turned every existing
repository carrying a pre-v0.17.0 stale marker red on upgrade — the marker has
been written by shipped builds since `AutogenRecipeForRecord` gained the
sidecar (`internal/workflow/recipe_autogen.go:196`). Its meaning in v0.17.0 is
that consumer authority is unusable: a feature at rung 4 is refused by GH #13
even though its coverage decodes and binds. Verify warns; it does not fail.

Precedence between the marker and coverage state is pinned, in both
directions:

- a stale marker beside **malformed or binding-stale** coverage does not lower
  that coverage's own `block` rung — rungs 1 and 2 outrank rung 4;
- a stale marker beside **valid complete** coverage does not raise the row to
  `block` — rung 4 is warning-class regardless of cohort, legacy or current;
- a stale marker beside **absent** coverage lands on rung 5, which is also
  warning-class, so a pre-v0.17.0 feature carrying a stale marker and no
  coverage verifies green.

Coverage success does not make the existing V10
`write_file_preimage_fresh` provenance requirement optional. The two checks
are independent rows. `recipe_generation_coverage` receives no supersession
downgrade of its own: it reports generation completeness, which supersession
does not change. Apply/verify preimage severity continues to follow ADR-029 D7
through the existing V10 row.

**No warning-class rung authorizes replay.** Rungs 3, 4 and 5 all leave the
verdict `passed` and the exit `0`, and all three are independently ineligible
for GH #13. Verify severity answers "does this repository still verify"; it
never answers "may a consumer replay this".

### D14 - Already-present operations count as succeeded and as no-write

An `already-present` classification is a **successful operation that performed
no write**. In `RecipeExecResult` (`internal/workflow/recipe.go:15-25`) it
increments **both**:

- `Applied`, because the operation succeeded and its postcondition holds;
- `Skipped`, because no byte was written.

`Applied` therefore continues to equal the number of operations that
succeeded, so the shipped summary line
`Recipe executed: %d/%d operations succeeded` (`internal/cli/cobra.go:972`)
still reports `N/N operations succeeded` for a fully already-present recipe.
An operator re-applying an applied feature sees success, not `0/N`.

`Skipped` is the no-write accounting channel and is what distinguishes a
re-apply from a first apply. Each already-present operation emits the exact
message:

```text
[write-file] <path>: already present (exact postimage), no write
```

Messages are per-operation and appear in `Messages` alongside the existing
`[<type>] <path>: OK` form for operations that did write.

### D15 - Seven governed producers publish coverage through one shared API

Coverage binds two inputs: the canonical patch and the recipe. **Every
CLI-owned path that writes, or knowingly checkpoints, a bound canonical patch
or recipe is a governed producer** and must leave coverage in a truthful state.
That universal rule is unchanged from rev-2; what changes is the registry it
generates. rev-2 enumerated five producers and missed two, and misdescribed a
third:

- `implement` writes `apply-recipe.json` on **two arms of one parse**: the
  JSON-unmarshal-failure arm writes the raw response verbatim
  (`internal/workflow/implement.go:192-195`) and the valid-JSON arm writes the
  reserialized recipe (`internal/workflow/implement.go:209`); the provider
  response and the heuristic fallback both flow through that same parse
  (`internal/workflow/implement.go:176-190`). `implement --manual`
  checkpoints an externally authored one
  (`internal/store/manual.go:51-80`). It is the ordinary origin of a
  non-`record` recipe, and rev-2 left it ungoverned;
- `tpatch edit` hands `apply-recipe.json` or `post-apply.patch` to `$EDITOR`
  (`internal/cli/c1.go:33-60,79-91`, `internal/cli/phase2.go:251-262`) and
  returns after the editor exits, so the CLI both selects the artifact and
  observes the mutation — although it currently discards the editor's exit
  error (D2);
- `cycle` is not a single patch writer. It is a **composite**: its implement
  step (`internal/cli/phase2.go:112`) is a P6 event, and its patch-capture step
  (`internal/cli/phase2.go:166`) is a separate P4 event.

The complete v0.17.0 producer inventory is:

| ID | Producer | `producer` value | Writes patch | Writes recipe | Appends generation | Coverage obligation |
|---|---|---|---|---|---|---|
| P1 | `tpatch record` (`internal/cli/cobra.go:1795,1900,1933`) | `record` | yes | yes, via `AutogenRecipeForRecord` | yes | recompute and publish full coverage |
| P2 | `tpatch feature patch refresh\|fixup` (`internal/cli/feature_patch.go:114,135,150`) | `feature-patch-amend` | yes | yes, but **non-regenerating** (`autogen=true, regenerate=false`) | yes | supply capture/reference truth; publish complete coverage only when D16 proves freshly-derived byte equality, otherwise incomplete. Also owes a category-(c) checkpoint publication on its non-writing same-patch branch (`internal/cli/feature_patch.go:104-112`) |
| P3 | `reconcile --accept` → `RefreshAfterAccept` (`internal/workflow/refresh.go:82,102`) | `reconcile-accept` | yes, **unconditionally** at `internal/workflow/refresh.go:82` | no, deliberately (`internal/workflow/refresh.go:20-24`) | only when `newPatch != originalPatch` (`internal/workflow/refresh.go:93,102`), `capture.mode: reconcile` | recompute; publish incomplete with `producer-patch-rewrite` + `recipe-not-regenerated` when the existing recipe no longer covers the new patch. The event is the **patch write**, so coverage is owed on every accept, including one whose bytes are unchanged and whose generation append therefore does not run |
| P4 | `tpatch cycle`'s patch-capture step (`internal/cli/phase2.go:166`) | `cycle` | yes | no | no | recompute, or publish incomplete — **composite, see below** |
| P5 | `tpatch apply --mode done` → `runApplyDone` (`internal/cli/cobra.go:982,1044`) | `apply-done` | yes, **when it actually writes the canonical patch** | no | no | recompute, or publish incomplete |
| P6 | `tpatch implement` — both arms of the `RunImplement` recipe parse (`internal/workflow/implement.go:192-195,209`) **and** a successful `implement --manual` checkpoint of an externally authored recipe (`internal/cli/cobra.go:744`, `internal/store/manual.go:51-80`) | `implement` | no | yes | no | publish coverage after the recipe and its provenance are published, at the common coverage-last point both parse arms reach; when no canonical patch exists, publish explicit incomplete coverage — never silent absence |
| P7 | `tpatch edit` when the **resolved** artifact path is the canonical `artifacts/post-apply.patch` or `artifacts/apply-recipe.json` (`internal/cli/c1.go:33-44,64-95`) | `artifact-edit` | via the operator's editor | via the operator's editor | no | after the editor process returns — successfully or not — detect the bound mutation and either recompute coverage where that is truthful, or publish explicit incomplete coverage with `manual-bound-artifact-edit` |

**P4 is a composite and owes publication per event, not per command.**
`tpatch cycle` runs implement at step `[4/6]` (`internal/cli/phase2.go:112`)
and captures the patch at step `[6/6]` (`internal/cli/phase2.go:166`). The
implement step **is** a P6 event and publishes P6 coverage at that point. That
matters because `cycle` can exit between the two: `--skip-execute` returns
right after implement (`internal/cli/phase2.go:122-126`), and each interactive
confirmation prompt can decline and return
(`internal/cli/phase2.go:127-129,152-154`). Because P6 already published, those
early exits are safe and P4 owes nothing. P4 owes a publication **only if the
patch write at step `[6/6]` actually happens**, and then it publishes again,
superseding the P6 record with one bound to the patch.

**P7 is observation, not authorship, and it triggers on the resolved path.**
`tpatch edit` resolves the artifact by explicit argument or by state default
(`apply-recipe.json` in `implementing`, `post-apply.patch` in
`applied`/`unapplied`, `internal/cli/c1.go:45-60`) and then resolves that token
to a filesystem path through `resolveArtifactPath`, which probes the **feature
root first** and the `artifacts/` subdirectory second
(`internal/cli/c1.go:33-44`). The governed trigger is the **resolved absolute
path**, never the token the operator typed:

- only `<feature>/artifacts/post-apply.patch` and
  `<feature>/artifacts/apply-recipe.json` are bound artifacts;
- a same-named file at the **feature root** — `<feature>/apply-recipe.json` —
  resolves first and shadows the canonical one. Editing it is therefore **not**
  a P7 event, because it is not a bound artifact, even though the operator
  typed `apply-recipe.json`. A token-matching trigger would have published
  coverage for a decoy;
- the explicit spelling `tpatch edit <slug> artifacts/apply-recipe.json`
  resolves to `<feature>/artifacts/apply-recipe.json` on the first probe and
  **is** a P7 event. Both spellings are pinned in the acceptance matrix,
  together with the root-decoy precedence, so an implementation cannot satisfy
  one and miss the other.

`openInEditor` runs the editor and returns when the process exits
(`internal/cli/phase2.go:257-261`). When `$EDITOR` is unset it starts **no**
process at all: it prints `  (set $EDITOR to review <path> in your editor)` and
returns (`internal/cli/phase2.go:252-255`). That path is modeled explicitly —
no process runs, so no byte can change, so **no P7 event occurs and nothing is
owed** — rather than left to fall out of the byte comparison by accident. On
return from an editor that did run — including a return carrying an
editor error, which D2 requires the refactored helper to propagate — the
producer compares its before-snapshot (D2) with the file on disk:

- **no byte changed** — the operator inspected and quit. No mutation occurred,
  so no coverage change is owed; any existing coverage remains valid because
  its bindings still hold;
- **bytes changed and the result is truthfully recomputable** — the producer
  re-derives and republishes coverage exactly as any other producer would. This
  is possible only when D2's reconstruct-and-validate path succeeds;
- **bytes changed and the result is not truthfully recomputable** — the
  producer publishes explicit incomplete coverage carrying
  `manual-bound-artifact-edit`, plus `reference-not-durable` when no durable
  reference could be reconstructed.

The comparison and the publication happen **before** any editor error is
returned to the caller. A failed editor that nevertheless changed the bytes is
still a mutation of a bound artifact, and leaving stale coverage beside it
because the exit code was non-zero is exactly the silently-stale state D15
exists to remove.

**Editing an unrelated artifact is not a P7 event.** `tpatch edit <slug>
spec.md`, `request.md`, `analysis.md`, `exploration.md` or any other
artifact whose resolved path is not one of the two canonical bound paths
touches neither the canonical patch nor the recipe, so it publishes nothing and
is outside the registry. The trigger is the identity of the resolved file, not
the invocation of the command and not the spelling of the argument.

**Direct filesystem edits are outside the boundary, and this is stated rather
than papered over.** An operator who opens
`.tpatch/features/<slug>/artifacts/apply-recipe.json` in an editor without
going through `tpatch`, or writes it from a script, is not observable by any
producer: no `tpatch` process runs, so no code can publish anything. That is
**external tamper**, and GH #15 does not claim to govern it. What it does claim
is that such tamper is **detected at read time, not trusted**: every consumer
and every verify/doctor rung recomputes the patch and recipe hashes from the
bytes on disk (D9), so an out-of-band edit lands on rung 2 as binding-stale
coverage and is refused by GH #13. The boundary is therefore: `tpatch`-mediated
mutations are governed and published; filesystem-level mutations are
ungovernable but never silently accepted.

**One shared publication API.** All seven call a single coverage publication
entry point with a typed input describing producer, slug, canonical patch
presence and bytes, capture/reference observation, recipe presence and
derivation outcome. No producer encodes coverage bytes itself, and no producer
has a private policy: policy differences between P1 and P3 are expressed as
different inputs to the same API, not as different writers. This is what makes
the obligation auditable — the **publication guard** has exactly one call-site
shape to look for. (It is a different guard from D1's source-inventory guard,
which enumerates `diff --git` readers.)

**What counts as a governed producer event.** rev-2 said publication was
"unconditional on every governed producer path", which read as "once per
command invocation" and was wrong in both directions: it demanded publication
from runs that touched no bound input, and it under-specified runs that touch
one twice. rev-3 replaced that with "a successful bound write or a successful
manual checkpoint", which was still too narrow — it left P2's non-writing
same-patch branch, the branch an operator reaches for precisely when coverage
needs repairing, outside the definition while D15 simultaneously demanded
publication from it. The definition has **three** categories:

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
> this decision names, in which the producer re-observes a bound artifact and
> re-asserts that the recipe covers the patch as of now, **even though the
> bytes are identical and nothing is written**.
>
> Coverage publication is last and unconditional **per event**. A command
> invocation that produces no event of any category owes nothing.

Category (c) is closed: it is not "any run that looked at a bound artifact". It
contains exactly the branches this decision enumerates, and today that is one
branch — **P2's non-empty same-patch branch**.

**P2's category-(c) branch, exactly.** `feature patch refresh|fixup` captures a
patch (`internal/cli/feature_patch.go:88`), and when that patch is non-empty
but classifies as `Append == false` — the captured bytes hash equal to the
**latest** generation on record, which is the only generation
`ClassifyPatchGenerationKind` compares
(`internal/store/patch_generation_kinds.go:46-49`) — it prints
`no patch byte change; refresh skipped` (or
`fixup skipped`) and returns **without writing anything**
(`internal/cli/feature_patch.go:100-112`). rev-4's "a generation already on
record" was wider than the shipped comparison: matching an *older* generation
does not suppress the append, so that phrasing described a branch the code does
not take. rev-3 additionally described this branch as "the write is a
checkpoint of the bound patch", which is false: there is no write on that
branch at all. It is a category-(c) event, and the obligation is stated in
those terms:

- the branch **computes and publishes coverage before returning**;
- it **keeps its existing user meaning and its existing message** — the patch
  bytes did not change and the refresh/fixup was skipped — and **adds** the
  coverage status line (D11's dry-derivation proof gates any remediation
  command it prints);
- it changes disk **only** by repairing or publishing coverage. No patch, no
  numbered patch, no recipe, no provenance and no patch generation is written,
  and no feature state is advanced. That is the whole point: it is the
  zero-side-effect repair path.

**P2's empty-capture branch is not an event.** When the capture returns zero
bytes, `feature patch refresh` prints `no patch byte change; refresh skipped`
and returns at `internal/cli/feature_patch.go:91-95` (and `fixup` errors out).
That branch **is not a checkpoint of the old canonical patch**: the producer
observed the *working tree*, not the stored artifact, and it re-asserts nothing
about it. It owes no publication unless some other bound event occurred in the
same invocation.

Applied to the registry:

| Situation | Event? | Obligation |
|---|---|---|
| P2 run whose captured patch is non-empty and classifies `Append == false` (`internal/cli/feature_patch.go:104-112`) | **yes — category (c)** | compute and publish coverage before returning, alongside the unchanged `no patch byte change; refresh\|fixup skipped` message; an early return that skips publication is a violation |
| P2 run whose capture is empty (`internal/cli/feature_patch.go:91-95`) | no | nothing owed; this is not a checkpoint of the stored patch |
| P2 run that writes the patch (`internal/cli/feature_patch.go:114`) | yes — category (a) | publish after the write |
| P4 exit after the implement step (`--skip-execute` or a declined prompt) | no P4 event | nothing owed by P4; P6 already published at the implement step |
| P4 step `[6/6]` that writes the patch | yes — category (a) | publish, superseding the P6 record |
| P4/P5 capture that produces an empty patch, so no patch write occurs (`internal/cli/phase2.go:165`, `internal/cli/cobra.go:1043`) | no patch event | no patch-bound publication owed |
| P5 reapply branch, which reads the canonical patch and writes none (`internal/cli/cobra.go:1006-1022`) | no | nothing owed |
| `apply --mode execute` reapply branch, which reads the canonical patch (`internal/cli/cobra.go:914-919`) | no | nothing owed |
| P6 run on either arm of the recipe parse whose **write succeeded** (`internal/workflow/implement.go:192-195,209`) | yes — category (a) | publish at the common coverage-last point both arms fall through to: after the recipe write, the provenance attempt and the state-mark attempt (`internal/workflow/implement.go:243`), and before that final return is propagated. A failed state mark does not cancel the obligation — the recipe write it describes already landed |
| P6 run whose `WriteArtifact` **failed** on either arm (`internal/workflow/implement.go:195,210`) | no | no successful bound write occurred; nothing owed, and the write error is returned |
| P6 `--manual` run that checkpoints an existing recipe | yes — category (b) | publish |
| P7 edit whose resolved path is bound and whose bytes changed | yes — category (a) | publish, even when the editor returned an error |
| P7 edit that changed no byte | no | nothing owed |
| P7 invocation with `$EDITOR` unset (`internal/cli/phase2.go:252-255`) | no | `openInEditor` prints its pointer line and returns without starting a process, so no byte can change and no event occurs |
| `tpatch edit` whose resolved path is not a bound artifact — including a feature-root decoy | no | nothing owed |

**An ignored bound-write error is a bug the implementation must fix.**
`cycle`'s patch write discards its error
(`s.WriteArtifact(slug, "post-apply.patch", patch)` at
`internal/cli/phase2.go:166`), so today a failed write is indistinguishable
from a successful one, and no correct event decision can be made from it. The
implementation propagates that error. Publication follows a **successful**
write; a producer that cannot tell whether its write succeeded cannot publish
truthfully about it.

**The three permitted terminal states.** Every governed producer event leaves
the feature in exactly one of:

1. **newly recomputed valid coverage** — the producer re-derived effects from
   the artifact it just wrote and published a bound record, `complete` or not;
2. **explicit incomplete coverage with reasons** — the producer could not
   recompute a complete record and said so in sorted `reasons` (and, where
   applicable, per-effect `reason_codes`), which under D3 contain **exactly**
   the applicable closed codes rather than a chosen subset:
   `producer-patch-rewrite` with `recipe-not-regenerated` when the cause is a
   patch rewrite that left the recipe uncovering, `canonical-patch-missing` /
   `canonical-patch-empty` / `canonical-patch-unparseable` for the patch it
   could not use, `recipe-undecodable` for a recipe it read and could not
   decode, `recipe-owner-mismatch` for a decoded recipe owned by another
   feature, or `manual-bound-artifact-edit` for a P7 mutation — together with
   every other code whose raising condition also holds;
3. **legacy untouched state** — coverage stays absent, and this is permitted
   **only** when the run genuinely predates v0.17.0 semantics for that feature,
   meaning the run produced no governed producer event at all.

**No governed producer may leave formerly complete coverage stale.** A run
that produces a governed event and exits with the previous `complete` coverage
still on disk fails the acceptance matrix, regardless of whether the bytes
happen to still validate. Because publication is last (D10), a crash can leave
coverage absent or hash-stale — both of which consumers reject — but never
silently-current. **A publication that fails is the same shape as a crash on
disk and is never the same shape in the exit code**: D10's publication-failure
contract applies to every P1-P7 event, so the producing command surfaces the
failure and exits non-zero rather than printing its ordinary success line over
absent or stale coverage.

**P2's specific policy.** `feature patch refresh|fixup` runs autogen with
`regenerate=false`, so a drifted recipe is preserved and `recipe-stale.json`
is written rather than the recipe being rewritten. P2 therefore publishes
`complete` coverage only when D16's exact derived-byte equality holds against
the patch it just wrote. Otherwise it publishes `incomplete` carrying
**exactly** the applicable closed codes, which on a patch-writing run always
include the **pair**: P2 rewrote the canonical patch
(`internal/cli/feature_patch.go:114`) and left a recipe that no longer covers
and simulates it, so `producer-patch-rewrite`'s condition holds, and
`recipe-not-regenerated` is raised **with** it — the two codes are never
emitted alone for that event (D3). `recipe-stale-marker-present` joins them
when the marker was written, and any other applicable record- or effect-level
code joins them too, because the arrays are exact sets rather than a chosen
subset. rev-6 described this policy as publishing "`recipe-not-regenerated`
and, when the marker was written, `recipe-stale-marker-present`", which read
as though the rewrite code were optional here; it is not. It never repairs
provenance for a recipe D16 does not prove.

**P2's category-(c) checkpoint raises neither rewrite code.** That branch
writes no patch (`internal/cli/feature_patch.go:104-112`), so
`producer-patch-rewrite`'s condition — "a governed producer rewrote the
canonical patch on this run" — does not hold, and `recipe-not-regenerated`,
which is defined against that rewrite, does not hold either. If the preserved
recipe does not cover the unchanged patch, the incompleteness is carried by the
codes whose conditions genuinely hold: `operation-missing` on the
otherwise-representable effects it fails to cover, `operation-surplus` or
`simulation-mismatch` at the record level. Exact-set semantics forbid
borrowing a rewrite code to describe a run that rewrote nothing.

**P2 repairs coverage even when nothing changed.** A refresh whose captured
patch is non-empty and hashes equal to the **latest** generation on record
writes nothing at all (`internal/cli/feature_patch.go:104-112`), yet it is
still a
category-(c) checkpoint: it asserts that this recipe covers this patch as of
now. It therefore recomputes and republishes coverage before returning, which
is what repairs a missing or hash-stale record without requiring a synthetic
change, and it does so while leaving the shipped `no patch byte change;
refresh|fixup skipped` message and every other on-disk artifact untouched. The
rerun-repairs-everything property (D10) would otherwise have a hole exactly
where an operator is most likely to reach for it.

The corollary is stated so it is not mistaken for a loophole: when the patch
**did** change, the preserved recipe can still be judged `complete` only if the
full derived bytes genuinely remain the same — D16's total comparison, over the
new patch. That is possible (a patch change that does not alter any whole-file
postimage the recipe writes), and when it happens the record is honestly
complete. Any other outcome is `incomplete`; "the recipe looks close enough"
has no encoding.

**P3/P4/P5's specific policy.** These three rewrite the patch and do not touch
the recipe at all. Each recomputes coverage from the patch it just wrote; if
the existing recipe no longer exactly covers and simulates that patch, the
published record is `incomplete` with `producer-patch-rewrite` and
`recipe-not-regenerated`, never stale `complete`. If the existing recipe does
still cover it exactly, neither code is raised (D3) and the record may be
complete. GH #13 later regenerates recipe and coverage from accepted operation
candidates; until it ships, an explicit incomplete record is the honest state,
and the remediation — subject to D11's dry-derivation proof — is
`tpatch record <slug> --regenerate-recipe`.

**P6's specific policy.** `implement` publishes coverage **after** the recipe
and `recipe-provenance.json` are published
(`internal/workflow/implement.go:192-237`), preserving D10's coverage-last
order. Both arms of its recipe parse are P6 events, and rev-3 described them
wrongly as "provider and heuristic": the provider response and the heuristic
fallback are chosen *earlier* (`internal/workflow/implement.go:176-190`) and
both then flow into one `json.Unmarshal`. The two arms are that parse's
outcomes:

- **unmarshal fails** — the raw response bytes are written verbatim as
  `apply-recipe.json` (`internal/workflow/implement.go:192-195`). **rev-4
  described this arm's control flow incorrectly, and rev-5 corrects it**: the
  arm does *not* return the parse error. The `json.Unmarshal` error is consumed
  by the `if` that selects the arm; the only `return` inside the arm is the
  **write** error at `implement.go:195`. When `WriteArtifact` succeeds, control
  falls out of the `if`/`else` and continues to the provenance attempt
  (`internal/workflow/implement.go:214-237`) and the state mark, and the
  function returns whatever
  `s.MarkFeatureState(slug, store.StateImplementing, ...)` returns
  (`internal/workflow/implement.go:243`) — `nil` on the ordinary path, the
  state-mark error otherwise. rev-5 called that a literal `nil` return; it is a
  returned call, which is why the coverage finalizer is anchored to the return
  point rather than to a value. Two consequences follow, and neither is
  optional:
  - **a successful raw write is a governed category-(a) event.** The recipe on
    disk has just been replaced with bytes that do not decode, so coverage is
    published — at the same common coverage-last point the valid-JSON arm
    reaches, after the state-mark attempt and before `RunImplement` propagates
    its final `s.MarkFeatureState` return (`internal/workflow/implement.go:243`)
    — carrying
    `recipe_present: true`, `recipe_decodable: false`, the raw-byte
    `recipe_sha256`, and `recipe-undecodable` in `reasons` (D3, D5). Publishing
    nothing here is exactly the silently-stale state D15 forbids: the previous
    coverage would still claim the old recipe hash while the new,
    non-decoding bytes sit beside it, and the command would report success;
  - **a failed `WriteArtifact` is no event at all.** `implement.go:194-196`
    returns the write error before any byte reached the artifact, so no bound
    write succeeded, no category applies, and **no coverage is owed**.
    Publishing on that path would assert an observation of a write that never
    landed;
- **unmarshal succeeds** — the reserialized recipe is written
  (`internal/workflow/implement.go:209`), and if that write fails the function
  returns its error and likewise owes nothing. On success the arm falls through
  to the same provenance and coverage-last point, and coverage is published on
  the ordinary terms.

Stating it as one rule: **coverage publication for P6 is attached to the common
post-write path both arms fall through to, not to either arm's own `return`.**
There is no parse-error return to publish "before", and rev-4's instruction to
do so described control flow the source does not have.

**P6's finalizer ordering, stated as a sequence.** For a P6 event the order is
fixed: the recipe write lands, the provenance attempt runs, the state-mark
attempt runs, **then** coverage is finalized, and only then is
`RunImplement`'s return propagated. Three properties follow and each is
contract:

1. **A failed state mark does not cancel publication.** Coverage is still
   finalized and still binds the recipe write that landed, because the event
   being recorded is that write, not the state transition;
2. **A failed publication is surfaced, not absorbed.** It is returned and
   `implement` exits non-zero (D10's publication-failure contract). There is no
   success-shaped fallback in which the recipe write is reported as a clean
   run over absent or stale coverage;
3. **When both fail, both are reported.** The returned error names the
   state-mark failure first and the coverage-publication failure with it,
   chained under the repository's ordinary error wrapping, so neither cause is
   dropped in favour of the other and the tight error handling elsewhere in
   `RunImplement` is not loosened to accommodate the finalizer.

In the ordinary case there is no canonical patch yet, so the published record
is `patch_present: false`, `coverage_status: incomplete`, with
`canonical-patch-missing` (D5). That is not a defect to be hidden by publishing
nothing: it is the truthful statement that a recipe exists and nothing yet
proves what patch it explains. A later `record` for the same feature repairs it
by publishing a bound record over the patch it captures. When a canonical patch
*does* already exist — `implement` rerun on a recorded feature — P6 recomputes
against it and publishes whatever that derivation supports.

A **producer-inventory guard** and a per-producer runtime row in the acceptance
matrix cover each of P1-P7. rev-3 backed that guard with a raw arithmetic
threshold — "the guard's expected-writer count is **seven**", failing when an
**eighth** writer appears — which is not implementable, because production does
not contain seven bound writer sites. It contains **eight** direct
`WriteArtifact` call sites for the two bound artifacts, distributed unevenly
across the registry, plus checkpoints and delegations that write nothing at
all. A threshold of seven would have failed on the shipped tree before a single
line was added.

The guard is therefore two guards over two different objects:

1. **Registry count.** The registry has **seven** entries and the `producer`
   enum has **seven** values. These two numbers must agree with each other and
   with the per-producer matrix rows. This is a count, and seven is its value.
2. **Site-to-producer mapping.** A separate, AST-derived mapping enumerates
   every production site that writes or checkpoints a bound artifact and
   requires each **reachable call chain** through it to map to **exactly one**
   of P1-P7. The unit of the mapping is the call chain, not the source line:
   one shared line reached from two producers is two chains, each mapping to a
   different single producer, and that is conformant. This is a mapping, not a
   count; adding a site to an existing producer is legal, and the guard's job
   is to prove no reachable chain is unmapped or ambiguous.

The current mapping is:

| Bound site or checkpoint | Maps to |
|---|---|
| `internal/cli/cobra.go:1795` — `post-apply.patch` | P1 |
| `internal/workflow/recipe_autogen.go:204` — `apply-recipe.json`, via `AutogenRecipeForRecord` | **two chains, no static producer**: reached from `internal/cli/cobra.go:1900` it is P1; reached from `internal/cli/feature_patch.go:135` it is P2. The helper itself has no `producer` value |
| `internal/cli/feature_patch.go:114` — `post-apply.patch` | P2 |
| `internal/cli/feature_patch.go:104-112` — non-writing same-patch checkpoint | P2, category (c) |
| `internal/workflow/refresh.go:82` — `post-apply.patch` | P3 |
| `internal/cli/phase2.go:166` — `post-apply.patch` | P4 |
| `internal/cli/cobra.go:1044` — `post-apply.patch` | P5 |
| `internal/workflow/implement.go:194` — `apply-recipe.json`, unmarshal-failure arm (raw response) | P6 |
| `internal/workflow/implement.go:209` — `apply-recipe.json`, valid-JSON arm | P6 |
| `internal/store/manual.go:51-80` — `implement --manual` checkpoint | P6, category (b) |
| `tpatch edit`'s `$EDITOR` delegation (`internal/cli/phase2.go:251-262`) | P7 |

Two properties of that table are load-bearing and are stated so the guard
cannot be written against the wrong object:

- **Shared helpers map by caller, not by site.** `recipe_autogen.go:204` is one
  source line reached from two producers, so it contributes **two reachable
  call chains**, each mapping to exactly one producer. It is not an eighth
  producer and it has no `producer` value of its own; the publication carries
  the calling producer's value. A guard that mapped *sites* one-to-one onto
  producers would either invent a producer for the helper or drop one of its
  two callers — which is why the guard's unit is the chain.
- **P7 writes nothing itself.** It delegates to an external editor process, and
  P6's `--manual` arm checkpoints bytes it did not write. Neither appears in a
  `WriteArtifact` grep, which is why the mapping enumerates checkpoints and
  delegations alongside writes rather than counting writer call sites.

A new production site that writes or checkpoints a bound artifact and maps to
no registry entry **fails the guard**. The phrase "an eighth writer" does not
appear in this decision, and an implementation that hard-codes a writer-count
threshold fails the acceptance matrix.

### D16 - Record-generated origin is proved by exact derived-byte equality

D6 needs to know whether the recipe on disk is one this producer generated. It
must answer that **without a trust-by-label marker**: a stored `generated by
tpatch` flag, an action name persisted from a prior run, or any other
self-asserted origin claim is forgeable and unverifiable, and adopting one
would be exactly the historical-adoption move GH #19 governs.

**The rule.** On every rerun, the producer recomputes the canonical derived
recipe bytes from the current canonical patch plus the captured reference
observation. If those bytes equal the existing `apply-recipe.json` bytes
**exactly**, then this run has just generated and validated those bytes, and
it may truthfully publish provenance carrying its own actual base commit and
generation time — even when prior provenance is missing entirely. The proof is
the derivation, performed now; the provenance statement describes this run,
not a reconstructed history, so it is true regardless of what happened before.

Three properties make the equality meaningful, and all three are required:

1. **The derivation is pure.** It is a function of the canonical patch bytes,
   the immutable capture observation and the slug only. It does not read live
   worktree bytes and does not embed a timestamp. (Today
   `RecipeFromPatch(s.Root, slug, patch)` reads postimages from the live
   worktree, `internal/workflow/recipe_autogen.go:86-122`; under this decision
   it reads them from the captured observation instead.)
2. **The encoding is canonical.** One deterministic encoder produces the
   recipe bytes, so equal semantics implies equal bytes.
3. **The comparison is total.** Full canonical bytes, not a projection.

**File-set equality is retired for this decision.** `compareRecipeFileSets`
(`internal/workflow/recipe_autogen.go:211-251`) proves only that two recipes
name the same paths, which is satisfied by a manual recipe with entirely
different operations. It may not be used to decide origin, provenance or
coverage completeness.

**A richer manual or provider recipe is never record-generated by
coincidence.** Origin holds only when the full canonical bytes match a
*complete* derivation. A near-match — differing by key order, indentation,
one operation field, an extra `replace-in-file` operation, or an added
`created_by` edge — is **not** record-generated. In that case the producer:

- preserves the recipe bytes exactly;
- fabricates no provenance, and leaves any existing provenance untouched;
- publishes coverage that makes no origin claim, and names GH #19 as the owner
  of historical/manual adoption in its diagnostic.

Refusing a near-match is deliberate. "Almost the derivation" is the shape a
hand-edited recipe has, and treating it as generated would silently overwrite
a human's authorship record with a machine's.

**Recovery after a provenance failure is exactly this rule applied twice.** A
run that writes the recipe and then crashes before writing provenance leaves
bytes on disk that the *next* run's derivation reproduces exactly — same
patch, same capture, same pure derivation. The rerun therefore proves origin,
repairs provenance with its own truthful base and time, and republishes
coverage. No marker survives the crash, and none is needed.

If the derivation itself is incomplete (any unsupported effect), there are no
complete canonical bytes to compare against, so origin is not proved and no
provenance is written; coverage carries the incompleteness reasons instead.

### D17 - `apply --mode execute` classifies every coverage/recipe shape, and refuses by name when coverage binds no readable executable recipe

D5 withholds a partial recipe rather than publishing one that silently omits an
effect. That is the right producer policy, and it is retained. Its consumer-side
consequence was never stated: the feature now has valid coverage and **no**
`apply-recipe.json`, and `apply --mode execute` reaches `LoadRecipe`, which
returns `no recipe found — run 'tpatch implement <slug>' first`
(`internal/workflow/recipe.go:116-120`). That message is wrong for this state —
`implement` would author an unrelated provider recipe over a feature whose
patch is already recorded — and worse, it is indistinguishable from the honest
legacy case.

**rev-5 widens this decision from that one case to the whole state space.**
rev-4 specified four shapes and left the rest to fall through to whatever the
shipped code happened to do, which put a present-but-undecodable recipe on a
path that surfaces a raw JSON decoding error, and left "incomplete coverage
beside a perfectly good recipe" undefined — a shape that must keep working,
because it is what every legacy and every `implement`-authored feature looks
like on the day GH #15 ships. D17 is therefore a **total ordered classifier**
over the cross product of coverage state and recipe state (D17.2), and the
named refusal is the subset of it in which **coverage binds no readable
executable recipe**. The refusal is stated in those terms rather than as
"generation withheld a recipe", because the producer that withheld one is only
*one* of the ways to reach it — an operator who never generated a recipe
reaches the same state, and the diagnostic should not accuse a producer of a
decision it did not make.

**rev-6 makes the recipe half of that phrase read "readable", because D3 and
D9 define presence as readable existence.** rev-5's order 2 required the recipe
file to be "genuinely absent on disk", which left a truthfully published
`recipe_present: false` beside a present-but-**unreadable** recipe matching no
row at all: order 1 does not fire, because the record and the recomputed
readable existence agree, and order 2's absence test fails, because the file is
there. That shape now takes order 2, which is where it always belonged — the
coverage record binds no recipe the run can read, which is the same operator
situation as an absent one and carries the same refusal.

#### D17.1 - Where D17 sits, and what is therefore unreachable from it

`apply --mode execute` computes `reapplying` from the feature's state — true
when the state is `unapplied` or an unapplied baseline is pending — and, when
it is true, reapplies the canonical patch and **returns before `LoadRecipe` is
ever called** (`internal/cli/cobra.go:911-946`). `LoadRecipe` runs only on the
`reapplying == false` fall-through (`internal/cli/cobra.go:948-950`).

**Therefore D17 is reached only when `reapplying` is false.** rev-3's
diagnostic told the operator to "reapply it verbatim through the canonical-patch
reapply path" and claimed "the operator chooses it". Both are wrong at the
moment the message is printed: the canonical-patch reapply branch is
state-selected, not operator-selected, and it is by construction not selected
in the only state where D17 can fire. The instruction named a door that had
already closed.

Two consequences are decided here:

- the unreachable instruction is **deleted**. No D17 output tells an operator
  to take the canonical-patch reapply branch;
- the false claim that the operator chooses that internal branch is **removed**.
  `--mode` still accepts only `auto`, `prepare`, `started`, `execute` and
  `done` (`internal/cli/cobra.go:848`), GH #15 still adds no `reapply` value,
  and the reapply behavior remains the state-selected branch of `execute` — but
  that is now stated as a fact about the code, not offered as an action.

**Nor does D17 tell the operator to run `tpatch feature unapply`.** Moving a
feature into `unapplied` would make the reapply branch selectable, which is
precisely why it is tempting to suggest — and precisely why it must not be
suggested blindly. `feature unapply` has its own preconditions, reverse-applies
the patch and rewrites feature state. A diagnostic that recommends it without
having proved those preconditions hold is recommending a destructive operation
on speculation. GH #15 does not print it.

#### D17.2 - The ordered classifier, with coverage-binding refusal first

Before D17 classifies anything, apply validates the coverage record's own
bindings (D9). The classifier below is **total**: every reachable
coverage/recipe shape matches exactly one row, the rows are evaluated in order,
and the first match wins. Nothing falls through to an unspecified path.

| Order | State on disk | `apply --mode execute` behavior |
|---|---|---|
| 1 | coverage present but malformed, **or** any binding mismatch (D9) — in particular a presence flag whose **recomputed readable existence differs from the stored value in either direction**: a claimed-present artifact that is absent or unreadable, or a claimed-absent artifact that is present and readable — or any hash, reference, capture or **coverage-envelope-owner** mismatch | **named coverage refusal**, exit `2`, by the D13 rung-1/rung-2 code for the condition, raised **before the recipe is loaded at all**. The claimed-present-but-not-readable recipe case is `recipe-coverage-recipe-changed`: the record binds a recipe hash that no longer recomputes. The claimed-absent-but-now-readable case is the same code from the mirror direction, and it is how a later readability change is caught. A coverage envelope owned by another feature is `recipe-coverage-owner-mismatch`. None of these is the legacy no-recipe state and none is order 2 |
| 2 | coverage present, valid, `incomplete`, `recipe_present: false`, **and no readable `artifacts/apply-recipe.json` on disk** — the file is absent, **or** it is present and cannot be read | refuse with the named `recipe-generation-incomplete` diagnostic below; exit `2`; nothing is written and no operation runs. The diagnostic says **coverage binds no readable executable recipe** — it does not assert that a producer withheld one, because an operator who never generated a recipe reaches the identical state. Because the record and the recomputed readable existence agree, this is **not** an order-1 binding failure; the read error itself is reported with its actual cause rather than as a missing file |
| 3 | coverage present, valid, `incomplete`, `recipe_present: true`, `recipe_decodable: false` — the bytes were read and do not decode | refuse with the **same** named `recipe-generation-incomplete` contract and exit `2`, listing `recipe-undecodable` among the sorted reason codes and naming the recipe path. It does **not** fall through to the shipped raw JSON decode error, which reports a syntax position for bytes the operator did not necessarily write and says nothing about coverage |
| 4 | coverage present, valid, `incomplete`, `recipe_present: true`, `recipe_decodable: true` | **executes.** Explicit recipe execution stays backward-compatible: the recipe is loaded and proceeds through the existing preimage, path-safety and all-or-nothing gates exactly as it does today, and a **warning** is emitted stating that coverage for this feature is `incomplete` with its sorted reason codes and that incomplete coverage **is not replay authority** (D4). Apply is not blocked. Blocking here would break every legacy and `implement`-authored feature on upgrade, for a recipe the operator explicitly asked to run |
| 5 | coverage genuinely absent **and** no **readable** `artifacts/apply-recipe.json` — the file is absent, or present and unreadable | **unchanged**: the shipped `LoadRecipe` behavior for either state — the existing `no recipe found — run 'tpatch implement <slug>' first` error — and its existing exit `1` (`internal/workflow/recipe.go:116-120`, `internal/cli/cobra.go:52-59`). Physical unreadability follows the no-readable row, exactly as readable existence requires; GH #15 adds no coverage-derived message here |
| 6 | coverage genuinely absent **and** a **readable** recipe present | **unchanged**: the shipped legacy recipe execution, with no coverage-derived warning, refusal or gate added. A feature that predates v0.17.0 behaves exactly as it did |
| 7 | coverage present, valid, `complete` | the recipe is present and decodable **by schema** — predicate 3 makes any other combination undecodable — and it executes unchanged |

**The coverage record's own readability is decided the other way round from
its fields.** An `artifacts/recipe-coverage.json` that exists but cannot be
read is **present and unusable**, not absent: it takes order 1 with
`recipe-coverage-malformed` (D13 rung 1), and its diagnostic names the read
error. Only a genuinely absent coverage file reaches orders 5 and 6, because
those rows exist for features that predate v0.17.0 — and a feature whose
coverage file is on disk is not one of them. The asymmetry is deliberate: the
presence *fields* collapse absence and unreadability because both mean "no
usable artifact to bind", while the coverage *file* distinguishes them because
its absence is what selects the legacy path.

**Any other shape is not a state, it is a malformed record**, and it is caught
by order 1. `complete` beside `recipe_present: false`, `recipe_decodable: true`
beside `recipe_present: false`, `complete` beside an empty `effects` array, and
`complete` beside an `unknown` axis all fail the strict decode (D3), so they
arrive at the classifier as "present but malformed" and take the order-1
refusal. The classifier therefore needs no default arm, and an implementation
that adds one is asserting a state the schema cannot produce.

Order 1 outranking order 2 matters. A record that says `recipe_present: true`
beside a file that is no longer readable is describing a **deletion, a
permission change or a tamper**, not a withheld generation, and reporting it as
"generation was incomplete" would attribute an operator's or a script's action
to the producer. The binding refusal names the real condition. The mirror
direction matters for the same reason: a record saying `recipe_present: false`
beside a recipe that is now present and readable is stale authority, and
executing that recipe on the strength of a record that denies it exists would
be acting on a binding nobody validated.

**The division between orders 1 and 2 is exactly "does recomputed readable
existence match the record".** When it does not — in either direction — order 1
fires. When it does, and the agreed value is "no readable recipe", order 2
fires. A publisher that truthfully recorded `recipe_present: false` for a
present-but-unreadable recipe is therefore honest and lands on order 2; if that
same file later becomes readable, the record stops matching and order 1 takes
over. Neither state has a fall-through.

Order 4 is the row rev-4 omitted, and it is the common one. Incomplete coverage
means "the producer cannot prove this recipe covers this patch"; it does not
mean "this recipe is unsafe to run when you explicitly ask". The two are kept
apart: the recipe's own ADR-029 preimage gates still decide safety, coverage
decides *replay authority*, and the warning tells the operator which of those
two they just relied on.

Legacy coverage that is genuinely **absent** stays on orders 5 and 6 with its
existing behavior; the migration contract of D13 is unchanged. **Those two rows
split on readable existence, exactly as orders 2-4 do**: order 6 needs a recipe
the run can actually read, and an absent or unreadable one takes order 5, where
the shipped `LoadRecipe` error for that condition — no-recipe or read failure —
and its exit `1` are what the operator sees. rev-6 split them on "recipe
absent" versus "recipe present", which left a present-but-unreadable recipe
beside absent coverage nominally matching order 6 while the shipped code would
have failed to read it, so the two rows now use the same predicate the rest of
the classifier uses and no legacy shape is unrouted.

#### D17.3 - The refusal output is state-aware and every command it names is reachable

The refusal is reached only with `reapplying == false`, which in practice means
the feature is in `applied` — the dominant state immediately after `record` —
or in another non-reapplying state whose effects are not materialized. The
output branches on that, and every command it prints is one the operator can
actually run right now.

The header states the **binding** fact, not an attribution. Order 2 prints:

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

The `recipe status` line is what keeps the record-layer collapse from reaching
the operator: the coverage record says only `recipe_present: false`, but the
refusal distinguishes an absent file from one whose read failed and prints the
underlying cause. An implementation that reports an unreadable recipe as
missing fails the acceptance matrix.

Order 3 prints the same contract with its own first two lines, so an operator
learns that the bytes exist and cannot be read rather than being told a file is
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

Neither header claims that generation withheld a recipe. rev-4's wording
("recipe generation for `<slug>` was incomplete, so no apply-recipe.json was
published") asserted a producer decision the refusal cannot observe: the same
state is reached by a feature nobody ever generated a recipe for, and by one
whose recipe was written by `implement` and later removed. The coverage record
is the thing apply actually read, so the coverage binding is the thing the
diagnostic states.

Both are followed by exactly one state-selected block.

**When the feature is `applied`** — its effects are already materialized in the
working tree — no reapply is required at all, and saying so is the whole
correction:

```text
  This feature is already applied: its effects are materialized in the working
  tree, so nothing needs to be reapplied. To confirm that:
    tpatch verify <slug>
    tpatch status <slug>
  To make the recipe executable later, author a complete apply-recipe.json and
  checkpoint it with `tpatch implement <slug> --manual` — note that the
  checkpoint moves this feature from applied to implementing.
```

**When the feature is not `applied` and its effects are not materialized**, the
canonical patch is the authority, and the operator materializes it with
**external Git commands**, having reviewed it first:

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

Four properties of that output are decided, not incidental:

1. **`git apply --check` precedes `git apply`.** The dry-run is named first so
   the operator learns whether the patch applies before mutating anything.
2. **The commands are external.** They are `git`, not `tpatch`, precisely
   because no reachable `tpatch` command performs this operation from a
   non-reapplying state. Naming an external tool is honest; naming an
   unreachable internal branch is not.
3. **Review is explicit.** The patch path is printed on its own line and the
   operator is told to review it. `tpatch` does not apply it silently, and the
   text says so.
4. **The `implement --manual` state transition is disclosed.** Checkpointing a
   recipe advances the feature to `implementing`
   (`internal/store/manual.go:26-31,80`). An operator moving a feature out of
   `applied` deserves to be told, in the same breath as the suggestion.

`<sorted reason codes>` is the coverage record's `reasons` array followed by the
union of its effects' `reason_codes`, each sorted ascending and deduplicated —
the same list verify's rung 3 prints (D13).

#### D17.4 - Why exit `2`

`internal/cli/reject.go:38-47` documents exit `2` as "pre-mutation input
validation" and exit `3` as "post-validation state-machine refusal", and
enumerates rejection-flow examples for exit `2` — a bad reason, an empty note,
unresolvable evidence, a path-safety violation. That enumeration predates this
decision and is narrower than the constant's actual role.

Exit `2` is correct here, and the justification is the exit-`3` boundary rather
than the older example list. Orders 2 and 3 refuse because the **executable
plan is not valid**: apply was asked to execute a recipe, and the input
required to build an executable plan — a readable, decodable recipe the
coverage record binds — is not available. That is
readiness validation performed **before any mutation**, which is exactly what
the `exitValidation` constant means. Critically, **no state-machine transition
occurred and none was refused**: the feature's state is untouched, no
precondition on a source state was violated, and nothing about the feature's
position in the lifecycle caused the refusal. Exit `3` would assert a state
refusal that did not happen.

Order 1's coverage-binding refusal exits `2` for the same reason and by the
same constant: it is validation of an input artifact, performed before
anything is written. Orders 5 and 6 keep their **existing** exits — `1` for the
legacy `LoadRecipe` error, and whatever the shipped legacy execution path
already returns — because this decision does not renumber exits it did not
introduce.

The implementation updates the comment block at `internal/cli/reject.go:38-47`
to include pre-mutation executable-plan validation in the exit-`2` description,
so the constant's documentation matches its use.

#### D17.5 - What this decision deliberately does not do

- **It does not silently fall back to reapplying the canonical patch.** A
  fallback would turn a recipe execution into a patch application without the
  operator asking, which is a different operation with different failure modes.
  D17 refuses and explains; it never substitutes.
- **It does not weaken D5.** The answer to an incomplete derivation is still no
  recipe. This decision only makes the resulting state legible, and reachable
  to act on, at the point an operator meets it.
- **It does not block explicit apply on incomplete coverage.** Order 4 executes
  a present, decodable recipe and warns. Turning incomplete coverage into an
  apply refusal would be a breaking change for every pre-v0.17.0 feature and
  for every `implement`-authored recipe, in exchange for a guarantee GH #13
  already enforces on the only surface where it matters — replay eligibility.
- **It does not add a coverage gate to the legacy no-coverage path.** Order 6
  executes exactly as the shipped build does.

`recipe-generation-incomplete` is the surface code (§7 of the companion PRD);
the schema reasons it prints are the D3 codes. The mapping is the same
two-layer convention D13 pins.

## Consequences

### Positive

- Generated whole-file writes finally carry the preimage ADR-029 requires.
- Coverage cannot be made authoritative by editing a boolean, a stale
  generation pointer or a self-asserted origin label.
- Partial delete/rename recipes stop masquerading as replay candidates.
- Record-generated recipes remain verifiable without fabricating historical
  provenance, and a crashed run converges on rerun because origin is re-proved
  by derivation rather than remembered.
- Every production path that writes or checkpoints a bound input leaves
  truthful coverage, so no producer can silently invalidate another's
  `complete` record — including `implement`, which is the ordinary origin of a
  non-`record` recipe.
- One strict grammar answers "what does this patch touch" for every production
  consumer that claims a path or an effect kind, in two documented projections
  (b-side and all-paths), and every remaining specialized parser is registered
  as non-authoritative.
- Orthogonal effect axes describe binary renames, symlink deletes and
  executable renames without losing a dimension, and `reason_codes[]` reports
  every applicable cause.
- Every field a consumer must check is recomputable from artifacts it already
  holds: modes and object kind from the reconstructed reference tree, and
  `patch_fragment_sha256` from a byte range of the canonical patch.
- An absent, empty, unparseable or hand-edited bound artifact has an explicit
  encoding, so no state is reachable in which coverage says nothing and a
  consumer must guess.
- An unobserved image side is distinguishable from a proven-absent one, so a
  no-capture producer's record is truthful rather than merely well-formed — and
  from rev-5 the same is true of its *classification*: `unknown` says the axis
  was never established, instead of naming a kind nobody looked at. rev-6 keeps
  that honest without making it lossy: an axis goes `unknown` only when an
  **extant** side went unobserved, so a half-observed `add` or `delete` still
  publishes the kind and content it genuinely read while its unavailable reason
  and its incompleteness stand.
- `apply --mode execute` classifies every coverage/recipe shape it can meet,
  so no state falls through to an accidental path, and an operator running an
  explicit recipe under incomplete coverage is warned rather than blocked.
- Every incomplete effect's `disposition` follows deterministically from its
  reason codes, so two implementations cannot disagree about the same effect.
  rev-7 makes the reverse direction as tight: a reason is raised only where its
  own condition holds, so a deliberately unrepresented effect is not also
  reported as a coverage gap the producer failed to fill.
- GH #13 can consume one strict, deterministic producer artifact while still
  rechecking all trust inputs.
- ADR-029 D7's accepted supersession severity survives the migration intact.

### Costs

- Record must capture preimages before writing artifacts, and the derivation
  must become pure so byte equality means something.
- Six additional producers gain a coverage publication obligation they did not
  have, including three (`cycle`, `apply --mode done`, `implement`) that write
  no coverage-adjacent manifest today and one (`tpatch edit`) that must take
  before/after snapshots around an interactive editor.
- Several of those producers will routinely publish `incomplete` coverage
  until GH #13 regenerates recipes from accepted candidates; `implement` will
  publish `canonical-patch-missing` on essentially every pre-record run. That
  is the honest state, not a regression.
- `apply --mode execute` gains a new refusal path for a state that previously
  produced a misleading message, and that refusal must name external `git`
  commands because no reachable `tpatch` command materializes a canonical
  patch from a non-reapplying state. It also gains a warning on the common
  incomplete-coverage-plus-valid-recipe path, which will be noisy until GH #13
  regenerates coverage.
- `openInEditor` gains an error return that its **two** call sites
  (`internal/cli/c1.go:91` and `internal/cli/phase2.go:88`) must handle, and
  P7 must publish before propagating an editor failure. Only the `c1.go` site
  can reach a bound artifact; the `phase2.go` site opens `spec.md` and is never
  a P7 event.
- Apply gains explicit already-present classification for preimage-bearing
  writes, and a second accounting counter in its summary.
- A new strict artifact, decoder, verify row, doctor check and public
  documentation are required.
- Some capture modes and patch effects remain explicitly incomplete in v1,
  including every executable-mode effect and every recipe containing
  `append-file`.
- Migrating the PI-3 through PI-7 callers changes `touched_paths`, novelty
  classification, hunk attribution and reverse-apply scope for C-quoted paths
  and malformed headers from silent omission to a surfaced error, and the three
  PI-7 call sites gain refusal paths they did not have.
- Missing coverage is warning-class, so an operator who deletes the file sees
  a softer verify diagnostic; the compensating guarantee is that absence is
  always consumer-ineligible.
- Direct filesystem edits to bound artifacts remain ungovernable at write time;
  they are caught at read time as binding-stale coverage, which is detection
  rather than prevention.
- Presence is readable existence, so every surface that reports an absent bound
  artifact must carry the real read error alongside the collapsed flag, and the
  test suite must be able to inject a read failure through the repository's own
  file-access seam rather than assuming an OS permission model — a chmod-based
  fixture is not portable to Windows or to a privileged test user.

## Rejected alternatives

1. **Trust the latest patch generation.** Rejected because unchanged patch
   bytes can leave its recipe hash stale after recipe regeneration.
2. **Put coverage in patch-generations.json.** Rejected because that manifest
   is append-only and coverage must be repairable without a new patch
   generation.
3. **Put coverage in apply-recipe.json.** Rejected because it changes the
   recipe schema and permits a producer claim to be mistaken for consumer
   eligibility.
4. **Reuse recipe-provenance.json for coverage.** Rejected because provenance
   answers when/against what a recipe was generated; it does not prove every
   patch effect is represented.
5. **Emit delete and rename operations now.** Deferred until their apply,
   idempotency, path-type and backward-compatibility semantics have a dedicated
   decision.
6. **Generate persisted anchors with current replace-in-file.** Rejected
   because first-match execution is neither unique nor second-application
   safe.
7. **Make publication cross-file atomic.** Rejected as unnecessary for trust:
   recomputed hashes make every partial state fail closed, and rerun repairs it.
8. **Refuse a superseded feature's drifted recipe unconditionally.** Rejected
   in rev-1: it silently revokes accepted ADR-029 D7 and would turn every
   historical superseded feature into a hard apply failure at upgrade.
9. **Carry a hard parent's postimage as a child's preimage.** Rejected for v1:
   no field can bind the parent slug, generation and hash, so the authority
   would be unverifiable at read time. The affected operations are marked
   `parent-created-target-unsupported` instead. Revisit only with a schema
   change and an ADR revision.
10. **Emit a partial `write-file` for an executable add.** Rejected: the bytes
    would be right and the mode silently wrong, which is exactly the class of
    quiet loss GH #15 exists to remove.
11. **Govern only `record` and let the other patch writers alone.** Rejected in
    rev-2: `feature patch refresh|fixup`, `reconcile --accept`, `cycle` and
    `apply --mode done` all rewrite the canonical patch, so any of them could
    leave a `complete` coverage record describing a patch that no longer
    exists. Silently-stale complete coverage is worse than no coverage. See
    D15.
12. **Persist a `generated-by: tpatch` origin marker on the recipe.** Rejected
    in rev-2: a self-asserted, unverifiable label is precisely the trust
    shortcut D9 forbids everywhere else, it is forgeable by hand-editing, and
    it does not survive the crash window it was meant to cover. Exact derived-
    byte equality proves the same thing by recomputation (D16).
13. **Keep file-set equality as the origin or completeness test.** Rejected:
    `compareRecipeFileSets` is satisfied by a manual recipe with entirely
    different operations over the same paths, so it can neither prove origin
    nor prove coverage. Retired for both decisions (D16).
14. **Grade missing coverage by whether this build produced it.** Rejected in
    rev-2 as not implementable: the only carrier of that fact would be the
    coverage file, which is by definition absent. rev-1's
    `missing-produced` / `missing-legacy` split asserted a nonexistent marker
    and let deletion downgrade a hard failure. Uniform warning-class plus
    unconditional consumer ineligibility is the honest encoding (D13).
15. **Make `recipe-stale.json` presence a hard verify failure.** Rejected in
    rev-2: shipped builds have written that marker for a long time
    (`internal/workflow/recipe_autogen.go:196`), so blocking on it turns
    existing repositories red at upgrade for a condition that predates the
    decision. It is warning-class in verify and unconditionally ineligible for
    GH #13 (D13).
16. **Keep one `effects[].kind` enum.** Rejected in rev-2: a single field
    cannot say "binary" and "rename" at once, so `kind: binary` erased the
    rename and `kind: rename` erased the binariness. Orthogonal
    `change_kind` / `content_kind` / `object_kind` axes with a
    `reason_codes[]` array describe every combination the strict grammar can
    produce (D3).
17. **Claim one parser for all patch reading in production.** Rejected in
    rev-2 as false: `headerReferencedGitPath` and
    `stripGitInternalFileStanzas` must read non-Git diff dialects, and
    `countPatchFiles` is a display counter. The implementable rule scopes
    authority to path/effect claims and registers the rest (D1).
18. **Migrate `PathsAffectedByPatch` onto `FilesInPatchStrict`.** Rejected in
    rev-3 as a safety regression. `FilesInPatchStrict` returns the b-side path
    only (`internal/gitutil/patch_paths_strict.go:235-236`), while unapply
    needs the union of both sides so a reverse rename can restore the a-side
    path it recreates (`internal/gitutil/unapply.go:33-35`, pinned by
    `internal/gitutil/unapply_test.go:83-102`). Projecting onto the b-side
    would drop every rename and copy source from snapshot and rollback scope.
    The fix is a second strict projection, `PathsAffectedByPatchStrict`, over
    the same grammar (D1).
19. **Change `FilesInPatchStrict` to return both sides.** Rejected in rev-3:
    five shipped call sites (`internal/cli/land.go:767,1212`,
    `internal/workflow/refresh.go:59`,
    `internal/workflow/verify_landed.go:1009,1163`) consume its b-side list
    today. Widening it would silently change a landed-patch file set, a refresh
    comparison and two verify path scopes. Those callers are registered as
    PI-12 with an explicitly unchanged contract (D1).
20. **Leave `producer-patch-rewrite` unconditional.** Rejected in rev-3: as
    written in rev-2, any P3/P4/P5 run raised it, so a patch rewrite whose
    existing recipe still covered the result exactly would have been reported
    incomplete forever. The code is now conditional on the recipe failing to
    cover and simulate the new patch, and is always paired with
    `recipe-not-regenerated` (D3, D15).
21. **Hash an unspecified "normalized hunk digest" inside `effect_sha256`.**
    Rejected in rev-3: a consumer that cannot reproduce an input cannot
    recompute the binding, which makes D9's recomputation requirement
    unsatisfiable. `patch_fragment_sha256` replaces it with an exact byte range
    of the canonical patch (D3).
22. **Let an absent or empty canonical patch publish `complete`.** Rejected in
    rev-3: "every effect is represented" and "every represented effect is a
    creation" are both vacuously true over an empty effect list, so the most
    permissive record in the schema — `complete` plus `reference-tree-only` —
    would have been the default for a feature with nothing recorded. Explicit
    `canonical-patch-missing` / `canonical-patch-empty` incompleteness replaces
    it (D3, D5).
23. **Fall back to canonical-patch reapply when the recipe was withheld.**
    Rejected in rev-3: it silently substitutes a different operation for the
    one the operator asked for. `apply --mode execute` refuses by name (D17).
    rev-3 then pointed the diagnostic at that reapply path, which rev-4 also
    rejects — see alternative 26.
24. **Publish nothing when a producer has no canonical patch.** Rejected in
    rev-3: silence is exactly what D15 exists to remove, since an observer
    cannot distinguish "no producer ran" from "a producer ran and declined to
    speak". `implement` publishes an explicit `canonical-patch-missing` record
    instead (D5, D15 P6).
25. **Treat `tpatch edit` as out of scope because a human made the change.**
    Rejected in rev-3: the CLI selects the artifact, invokes the editor and
    observes its exit, so it can compare before and after. What it cannot do is
    attribute or recompute the result in general — which is why P7 publishes
    `manual-bound-artifact-edit` rather than a fabricated recomputation (D15
    P7). Direct filesystem edits, where no `tpatch` process runs at all, remain
    outside the boundary and are caught at read time instead.
26. **Point the D17 refusal at the canonical-patch reapply path.** Rejected in
    rev-4: that path is state-selected and runs only when `reapplying` is true,
    which returns before `LoadRecipe` — the very call that produces the D17
    state. The instruction is unreachable from the state it is printed in, and
    the accompanying claim that "the operator chooses it" describes a choice
    the CLI does not offer. D17 names reachable external `git apply --check` /
    `git apply` commands and an explicitly disclosed
    `implement --manual` checkpoint instead (D17.1, D17.3).
27. **Recommend `tpatch feature unapply` so the reapply branch becomes
    selectable.** Rejected in rev-4: it is a destructive reverse-apply with its
    own preconditions, recommended on speculation by a diagnostic that has
    proved none of them. A refusal may not propose a mutation it has not
    validated.
28. **Report an absent recipe file beside `recipe_present: true` coverage as
    the D17 withheld-generation state.** Rejected in rev-4: that record
    describes a deletion or tamper, and attributing it to the producer's
    generation policy misnames the condition. It is a coverage-binding refusal
    (`recipe-coverage-recipe-changed`) and it outranks D17 (D17.2).
29. **Count bound writer call sites and fail on the eighth.** Rejected in
    rev-4: production already contains eight direct bound `WriteArtifact`
    sites across seven producers, one shared helper reached from two producers,
    and two producers that write nothing themselves. The threshold would have
    failed on the shipped tree. The registry count (seven) and the
    site-to-producer mapping are separate guards over separate objects (D15).
30. **Keep one `recipe_present` flag for "exists and decodes".** Rejected in
    rev-4: it forced a present-but-corrupt recipe to publish
    `recipe_sha256: ""`, which made an out-of-band corruption indistinguishable
    from a deletion and left D9 with nothing to recompute.
    `recipe_present` + `recipe_decodable`, with the raw-byte hash always bound
    when the file is present, restores the recomputation (D3).
31. **Encode an unobserved image side as an absent one.** Rejected in rev-4: it
    publishes a proven-absence claim the producer never made, which is exactly
    the fabricated authority this ADR refuses everywhere else. `*_observed`
    flags plus mandatory `preimage-unavailable` / `postimage-unavailable`
    reasons make the no-capture shapes truthful (D3).
32. **Leave `disposition` unmapped to `reason_codes`.** Rejected in rev-4:
    three of the four closed values were unreachable by any stated rule, and a
    record could carry `unsupported` beside `operation-missing` without
    contradiction. An exact mapping with `mismatch` → `ambiguous` →
    `unsupported` precedence is strictly validated instead (D3). rev-7 closes
    the same pairing from the other side: `operation-missing` is raised only
    for an otherwise-representable effect, so `unsupported` beside it is not a
    precedence question but a refused record (D3).
33. **Claim a bijection between surface codes and schema reasons.** Rejected in
    rev-4 as impossible: effect-local reasons are reported in aggregate, and one
    surface code (`recipe-generation-not-regenerated`) names a pair of schema
    reasons. A one-way partial mapping plus vocabulary disjointness is the
    implementable rule (D13).

## Implementation order

1. strict effect parser, the `PathsAffectedByPatchStrict` all-paths projection,
   PI-3..PI-7 caller migration, PI-12 b-side contract regression, the
   registered non-authoritative scanner set, the source-inventory guard, the
   `openInEditor` error-return refactor, and the capture snapshot;
2. pure derivation, preimage synthesis, D16 origin proof and convergent record
   provenance;
3. strict coverage schema with orthogonal axes classified over the effect's
   **extant** sides, the refusal of contradictory observed-absence shapes on an
   extant side, `patch_present`,
   `recipe_present` + `recipe_decodable` as readable existence, the observation
   flags, the one-based grammar-order `ordinal` over the canonical patch's
   strict-grammar records,
   `patch_fragment_sha256`, reason allocation with set-equality
   exhaustiveness over each code's own raising condition — including
   `operation-missing`'s otherwise-representable scoping — the `disposition`
   mapping,
   simulation and binding;
4. the shared publication API, the seven-producer obligation, the
   site-to-producer mapping guard, the three event categories and rerun
   recovery;
5. apply/verify/doctor integration, the D13 precedence ladder with the two
   owner conditions kept at their own severities, D17's total
   seven-case execute classifier including the bidirectional readable-existence
   binding refusal and the state-aware named refusal, and result accounting;
6. docs, skills, parity and downstream soak.

## Deferred decisions and review triggers

Reopen this ADR if implementation or a later wave requires any of the
following. Each row is a real deferral, not an unanswered question.

| Deferred | Current v1 answer | Reopen when |
|---|---|---|
| A durable tree object minted for a non-durable capture | Not minted; coverage is incomplete with `reference-not-durable` | A capture mode genuinely needs durable authority without a commit |
| Delete/rename/copy/binary/mode-only/symlink/gitlink/executable operations | Not emitted; explicit incomplete reason codes | A dedicated ADR defines their apply, idempotency and path-type semantics |
| Exact present-state reclassification for `append-file` / `replace-in-file` | None; `operation-not-reclassifiable` | GH #13's ephemeral derivation proves a reclassification rule worth persisting |
| Parent-created target authority | `parent-created-target-unsupported`; no carrier field | A schema change can bind parent slug, generation and hash verifiably |
| Persisted contextual anchors | None; `contextual_hint` advisory only | GH #13 proves uniqueness and idempotence and a consumer needs persistence |
| Missing-coverage severity | Uniform `warn`, never graded by origin, never eligible | A durable out-of-file carrier exists that can prove a build produced coverage for a feature |
| `recipe-stale.json` severity | `warn` plus unconditional consumer ineligibility | The legacy marker cohort has aged out and a migration policy graduates it |
| Recipe regeneration for `reconcile-accept`, `cycle` and `apply --mode done` | Not attempted; conditional `producer-patch-rewrite` / `recipe-not-regenerated` incompleteness | GH #13 regenerates recipe and coverage from accepted operation candidates |
| Truthful recomputation of a P7 (`tpatch edit`) mutation in the general case | `manual-bound-artifact-edit` incompleteness unless a prior durable reference reconstructs and validates | A durable pre-edit reference is bound by an artifact a later run can independently verify |
| Governing direct filesystem edits to bound artifacts | Out of scope; detected at read time as binding-stale coverage | A file-watch or content-addressed store makes write-time observation possible |
| A GUI editor that forks and exits before the operator's later save | Not observable by P7; the later save is external tamper caught at read time | The same file-watch or content-addressed store trigger |
| Recovering effect coverage from a patch the strict grammar refuses | `canonical-patch-unparseable`; raw hash bound, `effects: []` | A lenient recovery projection is proved safe enough to bind an effect list |
| Recovering operation coverage from a recipe that does not decode | `recipe-undecodable`; raw hash bound, no operation assignment | A tolerant recipe decoder exists whose output can be bound as authority |
| Classifying `object_kind` / `content_kind` when an **extant** side went unobserved | `unknown` on the axis, with the mandatory unavailable reason and an `ambiguous` disposition; an unobserved **non-extant** side leaves the axes definite and still raises its reason | A producer can take a capture on that path — for `implement` (P6), that means a decision to reconstruct a reference tree before writing a recipe |
| Distinguishing an absent bound artifact from a present-but-unreadable one **in the record** | Not distinguished: `patch_present` / `recipe_present` are readable existence, and only the diagnostic names the cause | A consumer needs to act differently on the two, which would require a third presence value and a migration |
| Gating explicit `apply --mode execute` on incomplete coverage | Not gated; order 4 executes and warns | GH #13 has shipped and the incomplete cohort has aged out, so a gate would not break legacy features |
| A per-effect surface code for effect-local reasons | Aggregated under `recipe-coverage-incomplete`; no individual code | A surface genuinely needs to gate on one effect-local reason |
| Automatic coverage repair in doctor | Read-only `D10` warnings | A future accepted doctor contract authorizes a guarded mechanical fix |
| Cross-file transactional publication | Coverage-last plus rerun recovery | Recomputed-hash rejection proves insufficient in soak |
| Bundling GH #15 and GH #13 in one release | Separate v0.17.0 / v0.18.0 | Never without a new decision |

## References

- GH #15 and GH #13.
- `internal/workflow/recipe_autogen.go:24-260`.
- `internal/workflow/implement.go:18-100,131-243,176-195,209`.
- `internal/cli/cobra.go:52-59,744,848,905-950,972,982,999-1044,1549-1565,1795,1863,1894-1949,2007-2008,2094-2101`.
- `internal/cli/c1.go:26-60,64-96,207`.
- `internal/cli/feature_patch.go:88,91-95,100-112,114,135,150-160,163`.
- `internal/cli/record_collision.go:96`.
- `internal/cli/feature_unapply.go:156`.
- `internal/cli/land.go:180,694-698,767,1212`.
- `internal/cli/phase2.go:112-129,152-170,251-262`.
- `internal/cli/reject.go:38-47`.
- `internal/store/manual.go:26-81`.
- `internal/store/patch_generation_kinds.go:46-64`.
- `internal/workflow/refresh.go:20-24,59,78-120`.
- `internal/workflow/recipe.go:15-25,54-126,199-247`.
- `internal/workflow/writefile_safety.go:108-170,222-330`.
- `internal/workflow/patch_generations.go:26-101`.
- `internal/workflow/reconcile_derivation.go:95-124`.
- `internal/workflow/file_novelty.go:130-231`.
- `internal/workflow/hunk_overlap.go:150-175`.
- `internal/workflow/verify_landed.go:1009,1163`.
- `internal/store/patch_generations.go:18-180`.
- `internal/store/store.go:504-540`.
- `internal/workflow/verify.go:47-77,624-637`.
- `internal/workflow/verify_anchored.go:788-1000`.
- `internal/workflow/doctor.go:233-245`.
- `internal/gitutil/gitutil.go:885-911,1170-1270`.
- `internal/gitutil/patch_paths_strict.go:235-253`.
- `internal/gitutil/unapply.go:33-125`.
- `internal/gitutil/unapply_test.go:83-102`.
- `internal/gitutil/capture_modes.go:275-328`.
- `internal/workflow/reconcile.go:445-464`.
- `docs/adrs/ADR-028-supersession-edge-model.md:77-88`.
- `docs/adrs/ADR-029-write-file-recipe-safety.md:43-56,74-76`.
- `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/summary.md:1-243`.
