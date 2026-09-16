# Templates
# File templates for .tpatch/ artifacts — used by tpatch init and feature operations.

## Recipe generation and coverage authority

Recipe coverage is necessary, not sufficient, for future replay eligibility; it is not cross-base safety.
A warn/exit0 coverage row is not eligibility and never grants replay permission.
The canonical `post-apply.patch` remains feature intent; neither coverage nor a producer label authorizes replay.

Under `.tpatch/features/<slug>/artifacts/`, `recipe-coverage.json` (C) binds exact
readable patch/recipe bytes, reference and effects. `recipe-capture-event.json`
(E) is independently built from immutable observation/final bound inputs and
contains C's raw-byte hash. E is published atomically before C atomically last.
Single-file atomicity is not a transaction: interruption can preserve an old
coherent pair on a same-byte event; differing mixed pairs are detectable.
Independent unkeyed consistency is not authenticated authorship or historical
proof; coordinated consistent edits or whole-set rollback are not detected.
Only a real producer repairs the pair; never hand-author these derived files
or fabricate evidence in a reader.
Readers independently reconstruct content and trees; neither persisted file
is proof by itself.

| Producer | Identity | Commands / governed event |
|---|---|---|
| P1 | `record` | `tpatch record <slug>` including `--auto`, `--from`/`--to`, `--staged`, `--unstaged`; `tpatch land <slug>` embeds P1, not a new producer. |
| P2 | `feature-patch-amend` | `tpatch feature patch refresh <slug>` / `tpatch feature patch fixup <slug> --reason "..."`; non-empty same-patch checkpoint writes only E + C, not patch/recipe/provenance/generation/marker/state. Empty capture is no event. |
| P3 | `reconcile-accept` | `tpatch reconcile --accept <slug>` (also `--resolve --apply` auto-accept) refreshes patch, preserves recipe and republishes the pair. |
| P4 | `cycle` | `tpatch cycle <slug>` patch-capture step; earlier implement is P6. Skipped/declined/empty patch steps owe no P4 event. |
| P5 | `apply-done` | `tpatch apply <slug> --mode done` / auto apply's done step on patch write; state-selected canonical-patch reapply is separate, not a P5 write event. |
| P6 | `implement` | `tpatch implement <slug>` provider/heuristic writes and successful `tpatch implement <slug> --manual` checkpoints; `no-capture`, including raw undecodable provider output as incomplete. |
| P7 | `artifact-edit` | `tpatch edit <slug> artifacts/apply-recipe.json` / `tpatch edit <slug> artifacts/post-apply.patch`; changed bytes at the resolved canonical path, even before editor error. Root decoys, other artifacts and unchanged edits are no event. |

Every governed event finalizes the pair, including incomplete/same-byte events.
Publication failure is nonzero and suppresses success-shaped completion.
External edits publish nothing; bound-byte drift is detected on read.

**D16:** derive the entire canonical recipe freshly; compare all raw bytes,
not file sets, parsed operations, formatting-equivalent JSON, labels or prior
provenance. Equality permits convergent provenance repair without rewriting
matching recipe bytes. Otherwise preserve manual/provider recipe and provenance
bytes; record/P2 may mark `recipe-stale.json`. Explicit
`tpatch record <slug> --regenerate-recipe` replaces only with a complete
derivation; unsupported effects withhold a partial recipe or preserve an
existing one, even when regeneration is requested.
P1/P2 create a missing recipe only from a complete derivation.
`record --no-recipe-autogen` suppresses automatic creation, not explicit regeneration.
P2's paired `producer-patch-rewrite` / `recipe-not-regenerated` reasons require
a rewritten patch no longer exactly covered, simulated and reclassified by
the preserved recipe. Formatting-only D16 mismatch does not raise the pair;
the stale marker still makes coverage incomplete (ADR-040).

**ADR-039 completeness:** only `write-file` with present, non-null
`preimage_hash` is admissible; explicit `""` gates creation. `append-file`,
`replace-in-file` (even exact replacement) and ungated writes cannot satisfy
v1 completeness (`operation-not-reclassifiable` for assigned effects).
Existing mixed-operation/manual/provider recipes retain their execution
behavior. All ten predicates must hold:

1. Present canonical patch, fully strict-parsed, with at least one effect.
2. Durable `reference.kind: commit`.
3. Readable, decodable recipe owned by the coverage feature.
4. Every normalized patch effect exactly once; no extra effects.
5. Every operation assigned to an effect; no surplus operations.
6. Repository-safe effect/operation paths; no two operations claim one path.
7. Every effect `represented`; all effect/record reason arrays empty.
8. Required sides observed with exact modes/hashes on present sides, including observed absence where required.
9. Full immutable-preimage simulation reproduces exact postimage bytes, existence and supported modes without unmodeled changes.
10. Simulated-result reclassification is already-present for every operation and writes no byte.

`cross_base_status`: incomplete → `unsupported`; complete existing-file writes
→ `consumer-derivation-required`; complete exclusively empty-gated creations
→ `reference-tree-only`. No GH #13 replay consumer or persisted anchors;
broader operation coverage stays GH #24 work.

**Readers:** verify's `recipe_generation_coverage` orders malformed C, then
owner/raw-byte/reference/E binding failures (block/exit 2), then valid incomplete
coverage, current stale marker and genuinely absent C (warn/exit0), otherwise
passes. Missing/invalid E beside C uses `recipe-coverage-capture-evidence-invalid`.
Legacy no-C, including old `recipe-stale.json` or orphan E, remains
warn/verify-green absent other failures. Recipe execution and auto apply's
pre-prepare preflight refuse binding failures first. Valid incomplete coverage
with no readable/decodable recipe refuses before mutation:
`recipe-generation-incomplete`, exit 2; no partial synthesis or silent patch
fallback. Decodable incomplete recipes retain execution with a non-authority
warning and all other gates. Legacy missing/invalid-recipe errors retain
existing behavior.

**ADR-042 ordered no-write:** initial preimage authorization and exact-postimage
equality are separate. Equality grants only a no-write exemption, proved at each
operation's sequential position and rechecked before skipping; it cannot grant
fallback write permission. Invalidated postimage-only witnesses refuse before
any operation; originally authorized writes retain ordinary execution.
Skips increment `Applied` and `Skipped` with
`[write-file] <path>: already present (exact postimage), no write`. Dry-run uses ordered
previews without writes. Missing/unreadable targets, malformed gates, containment,
`created_by`, later-touch warnings and supersession severity remain independent;
explicit superseded drift may warn/write under existing audit-not-safety policy.
The 8 MiB projection bound does not limit streaming equality or large-file no-ops.

**Recovery:** `tpatch doctor --check D10` is read-only, warning-only and
`Fixable:false` even with `--fix`. Regeneration guidance requires dry feasibility
of the actual named record command, complete derivation and pair publication;
otherwise `recipe-generation-no-truthful-regeneration` calls for manual review.
Review a withheld recipe's patch and run `git apply --check` before deliberate
`git apply`, or author a full recipe and `tpatch implement <slug> --manual`
(moves to `implementing`, including from `applied`). Already-applied effects
need no reapplication. The state-selected canonical-patch `tpatch apply <slug>`
path stays separate; coverage does not select that path.
