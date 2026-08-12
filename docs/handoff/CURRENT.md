# Current Handoff

## Status

**Cluster state**: REV-7 DISPATCHED

v0.15.1 Wave B rev-6 is substantively complete, but G5 flags correct
anti-hardcode prose. Rev-7 is a one-line guard correction.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V0–V10 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: In Progress (rev-7)
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Dispatch HEAD (rev-6)**: `cbf5fcf`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Files Changed (rev-6) — exact list

| File | What changed in rev-6 |
|---|---|
| `docs/prds/PRD-verify-freshness.md` | **§2 non-goal** replaced the blanket "changes to `land` are out of scope" with the exact D19 exception; **§3.1** now carries the **V10 row** (id, trigger, block-downgraded-to-warn severity, pass criterion, reuses) and an eleven-check header note, and the **V8 row** states GH #2 reset semantics; **§3.1.1** rewritten — V9 then **V10 last**, with the shadow-reset rationale; **§3.1.2** gained the V10 remediation shape; **§3.4.3** steps 4–5 rewritten for the GH #2 reset, fail-fast now names V9 **and V10**; **§5** presence rows made mutually exclusive and the redundant combined row removed; **§6 Q13** no longer says `land` is behaviour-frozen; **§7.1** added **AC-L135** (totality guard) and rewrote AC-L5 / AC-L53; **new §7.1.2** with the G1–G10 regex table, four whitelist rules and the covered-section list; **§8** added the pre-existing `satisfied_by` 40-hex residual; matrix header 134 → **135**; all labels rev-6, empirical index **E1–E47**. |
| `docs/prds/PRD-tpatch-land.md` | **Top status** no longer claims `land` is unchanged — it states the exact two-part scope (readers' contract + the one D19 producer refusal) and enumerates what is *not* changed; **§3.8 header** rewritten to rev-6 (final) with the same scope split and E1–E47; **rule 6** base-commit length object-format-derived; **rule 13** rewritten — three mutually exclusive patch states, absent/empty short-circuit to `landed-artifacts-absent`, never a digest mismatch, with the superseded wording explicitly labelled; **rule 16** aligned and recipe shape scoped to a compatible patch state; **§3.8.5** disposition no longer says behaviour-frozen; **AC-LD5** derived length, **AC-LD12** absent/empty ⇒ terminal, **AC-LD23** added (land-side totality guard); matrix 25 → **26**; labels rev-6. |
| `docs/adrs/ADR-013-verify-freshness-overlay.md` | **Header/status** → Amendment 1 **rev-6 (final)**, D8–D19 declared closed; **D7 step 6** amended for the GH #2 reset with the superseded wording labelled; **§A1.1 preamble** now states the index is **E1–E47**, closed; all amendment section titles (decision, predicate, alternatives, consequences, references) → rev-6; references anchor re-validated at `cbf5fcf`. |
| `docs/handoff/CURRENT.md` | This rewrite. Side Research tail preserved byte-identically (md5 `b385fe622db9926f48861105239f113e`). |

## Decisions closed in rev-6

1. **V10 is in the main check table and V10 is last.** §3.1 gained the eleventh
   row with the shipped severity rule (`block`, downgraded to `warn` when
   `IsFeatureSuperseded` reports an active superseder), the shipped precondition
   (`skipped: V2 (recipe_parses) skipped or failed`) and the shipped
   non-failing outcomes (`preimageLegacyWarn`, `preimageSkip`). §3.1.1 explains
   why it is appended after V9 (`internal/workflow/verify.go:288-289`).
2. **GH #2 reset semantics are now stated where V7/V8 are defined.** §3.1 V8,
   §3.1.1, §3.4.3 steps 4–5 and ADR D7 step 6 all say the shadow is reset to
   `closureBaselineTree` between V7 and V8 (`internal/workflow/verify.go:1092`,
   `:1143`, `:1227-1229`), so recipe and patch are checked independently against
   the same baseline. The old "against the same shadow" text is labelled
   superseded, not silently deleted.
3. **The amendment is not behaviour-neutral, and says so.** The land PRD's top
   status and §3.8 header, the verify PRD's §2 non-goal and Q13, and AC-LD15 /
   AC-LD21 now agree: **one** producer change (the D19 R23 Base-Commit refusal)
   and nothing else.
4. **Absence is never a mismatch, in the producer contract too.** Land rule 13
   is rewritten to the three mutually exclusive patch states with the
   short-circuit; rule 16 and AC-LD12 follow; the base-commit length is
   object-format-derived in rule 6 and AC-LD5.
5. **Totality is now mechanically enforceable.** **AC-L135** (unit tier) plus
   **§7.1.2** define ten regexes G1–G10, four whitelist rules (marked line,
   marked block, negated/quoted, self-reference), explicit pattern scoping for
   G5 and G8, and the list of sections the guard must cover. **AC-LD23** binds
   the land PRD as one of the guard's three inputs. The guard was executed
   during this revision and returns **0 hits**.

## Preserved unchanged from rev-1 … rev-5

Git floor **2.36** · presence short-circuit before classification · Mode A
`recoverLand`-first with the two-case guarantee · **partial clone as a Wave C
acceptance gate (AC-L68 / AC-L69)** · `-C1` forward anchor qualification (E44) ·
the `(0/0)` block rule (E26/E28/E29) · exhaustive anchor collection ·
per-member V10 baselines with `RecipeProvenance.BaseCommit` (E42, Q15) ·
`ListFeatureEntries` inventory · `GIT_NO_LAZY_FETCH=1` on every object command
(E47) · duplicate identity by index-drop + `@@` normalization (E45/E46) · the
existential-inverse `replace-in-file` predicate (0/0 over 52,416 cases) ·
`C^` parent syntax (E43) · Mode A / Mode B producer split.

## Counts

| Metric | Value | Delta vs rev-5 |
|---|---|---|
| Verify acceptance matrix | **135 rows**, AC-L1 … AC-L135, contiguous | +1 (AC-L135 guard) |
| Verify groups | A 6 / B 10 / C 15 / D 21 / E 26 / F 20 / G 15 / H 22 = 135 | B +1 |
| Land acceptance matrix | **26 rows**, AC-LD1 … AC-LD23 incl. AC-LD18a/b/c, all tier C | +1 (AC-LD23) |
| **Total acceptance rows** | **161** | +2 |
| Unique code citations | **106**, every `file:line[-range]` in range | +4 |
| ADR decisions | D1–D19, no gaps; Amendment 1 = D8–D19, **closed** | — |
| Empirical index | E1–E47, contiguous, closed | — |
| Remediation strings | R1–R24, all referenced ids defined | — |
| Totality guard regexes | **G1–G10** over 3 documents | new |

## Test Results

Planning-only revision — no Go source changed. Validation was documentary and
mechanical:

- **Totality guard executed** (`.scratch-gh8/guard.py`, the exact §7.1.2 G1–G10
  patterns and whitelist rules, over all three authoritative documents):
  **0 hits**. It found and I corrected: the land PRD's behaviour-frozen
  disposition (§3.8.5), the ADR's unamended D7 "same shadow" step, an
  unlabelled superseded quote in land rule 13, and three self-referential
  phrasings in AC-L5 / AC-L53 / the §3.1 header note.
- **Doc validator**: **ALL CHECKS PASS** — 106 citation ranges, AC-L contiguity
  (135) + group sum (135) + tier well-formedness, AC-LD uniqueness (26) + all
  tier C, dangling AC/D/R reference check, D1–D19 headings, E1–E47 contiguity,
  R1–R24 definitions, unique Q1–Q18.
- **Semantic spot-checks** printed the source line for every new anchor:
  `verify.go:1092` `closureBaselineTree`, `:1143` `resetShadowToTree`,
  `:1227` its definition, `:288-289` the V10 append, `:852-905` the V10 body,
  `validation.go:22` the 40-hex `satisfied_by` regex, `land.go:1034-1043`
  `readRecipeSHA`, `land.go:394` `baseCommit`.
- Scratch removed; working tree contains only the four owned docs plus the
  guarded WIP files.

## Residuals for Wave C

1. **Q17** remains open (non-blocking, scoped in §6).
2. **`-C1` false reds**: a one-line-away healthy parent is rejected (E44).
   Measured and accepted — safety over reach.
3. **Normalization collision**: `@@`-rewritten identity treats the same payload
   at different positions as equivalent (E45/E46); needs duplicate line content
   to collide.
4. **Partial clone is a Wave C gate** (AC-L68 / AC-L69): mechanism proven, real
   filtered-remote path unproven; a blocker report is required if it cannot be
   constructed.
5. **`(0/0)` block cost**: 26 false reds / 151 present vs 0 false greens / 69
   absent. Deliberate.
6. **`active` widening** and the **forward-mode V10 change** alter existing
   behaviour for non-landed features; both intended and matrixed.
7. **Mode B is non-rollback**: a completed embedded `record` is retained on an
   R23 refusal, and says so.
8. **`satisfied_by` is 40-hex-only in shipped code**
   (`internal/store/validation.go:22`), so V5 cannot accept a SHA-256
   repository even though the landed contract derives base-commit length.
   Pre-existing, out of scope, now recorded in §8 so it is not read as a
   contradiction.
9. **Git floor 2.36** is a user-visible tightening; Wave C should surface it in
   release notes.
10. **The guard is a docs test, not a linter.** It reads three fixed paths; a
    future authoritative document must be added to its input list explicitly.

## Reviewer focus

1. **§3.1 / §3.1.1 / §3.4.3**: confirm the V10 row matches
   `internal/workflow/verify.go:852-905` exactly (severity downgrade,
   precondition, non-failing outcomes) and that the GH #2 reset is stated
   wherever V7/V8 are defined, including ADR D7 step 6.
2. **Scope statements**: land top status, land §3.8 header, verify §2 non-goal
   and Q13, AC-LD15 / AC-LD21 — confirm all five say the same thing (one D19
   producer change, nothing else) with no surviving blanket claim.
3. **Land rules 6 / 13 / 16 and AC-LD5 / AC-LD12**: confirm absence never
   yields a mismatch and no length is hardcoded to 40.
4. **§7.1.2**: confirm the ten regexes and four whitelist rules are
   implementable as a unit test, that G5/G8 scoping is stated in the pattern
   rather than the whitelist, and that the two deliberate out-of-pattern truths
   (`--dry-run`, anchor C) are justified.
5. **Counts**: 135 + 26 = 161 rows; 106 citations; groups sum to 135.

## Context for Next Agent

- Wave B is **planning only**. Implementation is Wave C. Nothing in `internal/`,
  `cmd/`, `assets/`, `tests/`, `SPEC.md` or `CHANGELOG.md` was touched.
- The binding ADR is **ADR-013 Amendment 1 rev-6 (final), D8–D19** — the
  decision set is closed. The operational contract is **PRD-verify-freshness
  §3.6 / §4.3.6–4.3.9 / §5 / §6 / §7.1 / §7.1.2**; the producer contract is
  **PRD-tpatch-land §3.8 / §6.2**.
- Load-bearing measurements are indexed **E1–E47** in ADR-013 §A1.1, closed for
  this amendment. Re-measure before contradicting any of them.
- The check sequence is **eleven** checks, V0–V10
  (`internal/workflow/verify.go:49-71`, V10 appended at `:288-289`).
  `VerifyReport` has **no** `freshness_label` field.
- V7 and V8 share one shadow but are **reset between** them
  (`resetShadowToTree`, `internal/workflow/verify.go:1142-1153`) — the GH #2 /
  v0.11.3 invariant. Do not describe them as sharing a tree.
- Key shipped constraints: `checkWriteFilePreimage`
  (`internal/workflow/writefile_safety.go:108-112`) reads the **live working
  tree**; `ListFeatures` (`internal/store/store.go:210-236`) **silently drops**
  unreadable features, hence `ListFeatureEntries` (`:274-348`);
  `RecipeProvenance` (`internal/workflow/implement.go:30-34`) already ships.
- Verify stays read-only; no status, index or worktree mutation on any path.
- Stage explicit paths only; do not touch the WIP docs in
  `.wave-close-allowlist`.

## Side Research — State-of-the-art middle pass (2026-05-10)

Paper-only exploratory pass completed for a non-LLM middle layer between
deterministic reconcile heuristics and full provider/coding-agent workflows.
This does **not** change code, schema, CLI behavior, roadmap status, PRDs, or
ADRs.

### Research packet

Created `docs/state-of-the-art/` with docs modeled after the existing market
research / PRD conventions: header block, related links, refresh triggers,
references, open questions, and disputes.

Files:

- `docs/state-of-the-art/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`
- `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`
- `docs/state-of-the-art/patch-capture-context-research-brief.md`
- `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`
- `docs/state-of-the-art/research-roadmap.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`

### Findings

1. Patch theory is useful as vocabulary for identity, inverse, composition,
   commutation, dependency, and conflict, but tpatch should not claim
   Darcs/Pijul guarantees on top of unified diffs.
2. Patch identity should be treated as a ladder: exact bytes, `git patch-id`,
   token fingerprints, AST/CFG/PDG similarity, behavioral checks, and finally
   provider/human intent judgment.
3. Computer-vision feature matching maps to code relocation: detect salient
   code keypoints, compute local descriptors, match across old/new upstream,
   reject outliers, then attempt relocated apply in a shadow tree.
4. Search-based application should operate only on uncertain patch clusters,
   after deterministic dependency/commutation pre-passes shrink the search
   space.
5. Beam search is the likely first practical non-LLM planner; MCTS and
   evolutionary algorithms remain candidates for larger uncertain clusters.
6. Vector retrieval / RAG fits as a distinct middle layer: dense retrieval can
   rank likely patch/hunk/code-region matches below full provider reasoning,
   while generation over retrieved context still belongs to the provider tier.
7. The experiment guide defines collection formats for feature metadata, hunks,
   keypoints, fingerprints, retrieval results, commutation relations,
   candidate apply attempts, metrics, and ground-truth labels.
8. First-party tpatch metadata should be the happy path for tpatch-aware repos:
   current metadata is good for lifecycle/DAG reasoning, but future patch
   generations, dependency version snapshots, operation IDs/read-write sets,
   structural anchors, relation artifacts, and vector manifests would make
   identity and ordering easier before fuzzy fallback.
9. A new patch-capture research brief preserves this PRD/ADR queue and defines
   the next front: Quilt-style explicit file claims, Git index/hook boundaries,
   IDE hooks, coding-agent event logs, and privacy-safe agent context capture.
10. Entire is verified as a concrete prior-art target. Its model uses Git hooks,
    agent hooks, commit trailers, a separate `entire/checkpoints/v1` metadata
    branch, shadow checkpoints, full transcript/session storage, redaction, and
    optional checkpoint remotes. tpatch should borrow the Git-native linking
    pattern but default toward summaries/references over raw transcripts.
11. `docs/state-of-the-art/research-roadmap.md` is now the durable exploratory
    tracker so research can advance independently if `docs/handoff/CURRENT.md`
    is reassigned to implementation work.
12. Amendment models differ by tool: Quilt/StGit usually refresh the managed
    patch, Git supports both amend and fixup/squash-forward workflows, Aider
    favors small commits plus undo, and Entire preserves context links around
    rewrites. tpatch likely needs canonical-current patch plus append-only
    generations, with explicit amend/fixup/fold/fork semantics.

### PRD drafts promoted from research (2026-05-13)

The first capture/metadata foundation PRDs were drafted as paper-only planning
docs:

- `docs/prds/PRD-feature-file-claims.md`
- `docs/prds/PRD-record-capture-modes.md`
- `docs/prds/PRD-feature-patch-identity-metadata.md`
- `docs/prds/PRD-feature-patch-amend.md`

`docs/state-of-the-art/research-roadmap.md` is updated to point at these drafts.
The remaining gate before implementation is review/acceptance of the queued
capture privacy and amendment-policy ADRs plus PRD review.

### Candidate follow-up names

These are research outputs only, not queued roadmap work. Four items below now
have draft PRDs as noted above.

- `PRD-structural-patch-fingerprints`
- `PRD-feature-patch-identity-metadata`
- `PRD-dependency-version-snapshots`
- `PRD-recipe-operation-identity`
- `PRD-structural-anchor-manifest`
- `PRD-patch-vector-index`
- `PRD-reconcile-commutation-graph`
- `PRD-reconcile-search-planner`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-planner-audit-artifacts`
- `PRD-feature-file-claims`
- `PRD-record-capture-modes`
- `ADR-patch-amendment-policy`
- `PRD-feature-patch-amend`
- `PRD-active-feature-session`
- `PRD-agent-event-log`
- `PRD-ide-capture-hooks`
- `PRD-git-hook-capture-guards`
- `ADR-capture-context-privacy-boundary`
- `ADR-capture-metadata-branch`
- `PRD-record-context-summary`
