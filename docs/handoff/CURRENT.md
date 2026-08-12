# Current Handoff

## Status

**Cluster state**: ACCEPTED

v0.15.1 Wave B **rev-7** is accepted. It is a one-line guard fix
on top of rev-6: §7.1.2 **G5** gains the narrow exemption the rev-6 external
review asked for, so the pattern stops flagging the two correct affirmative
anti-hardcode sentences while still catching an affirmative fixed-40 claim.
**No substantive decision, matrix row or citation changed** — 161 rows and 106
citations are byte-for-byte preserved. Planning only: no Go, tests, assets,
SPEC, CHANGELOG or GitHub issue state was touched.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V0–V10 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: Complete
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Dispatch HEAD (rev-7)**: `785d261`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Files Changed (rev-7) — exact list

| File | What changed in rev-7 |
|---|---|
| `docs/prds/PRD-verify-freshness.md` | **§7.1.2 G5** — pattern is now `40[- ](lowercase[- ])?hex\|hardcode[sd]? 40\|fixed 40`, still scoped to base-commit lines, and **exempt when the same line also matches `derived\|--show-object-format\|would reject\|rejects\|fails this row`**; the "Why" column states exactly which two sentence shapes the exemption clears and why the optional `lowercase` alternation exists. **§7.1.2 G3** class widened to `rev-[0-6]` so a stale current-amendment label cannot survive a revision bump. **Whitelist rule 3** gained one sentence pointing affirmative-voice cases at the per-pattern exemptions instead of at the negation rule. Amendment labels rev-6 → **rev-7** (11 occurrences). |
| `docs/prds/PRD-tpatch-land.md` | Amendment labels rev-6 → **rev-7** (7 occurrences). No rule, row or count changed. |
| `docs/adrs/ADR-013-verify-freshness-overlay.md` | Amendment labels rev-6 → **rev-7** (9 occurrences); references anchor re-validated at `785d261`; revision-history entries added for **rev-7** and **rev-6 (superseded)**. No decision changed; D8–D19 remain closed. |
| `docs/handoff/CURRENT.md` | This rewrite. Side Research tail preserved byte-identically (md5 `b385fe622db9926f48861105239f113e`). |

## The fix, precisely

**Problem (rev-6 external review).** G5's pattern flagged two *correct*
sentences that forbid the hardcode affirmatively rather than by negation, so
whitelist rule 3 could not clear them:

- `PRD-tpatch-land` §3.8.2 rule 18 — "A reader that hardcodes 40 **rejects**
  every valid landing in a SHA-256 repository."
- `PRD-tpatch-land` §6.2 AC-LD5 / AC-LD19 — "a fixture that hardcodes 40
  **fails this row**" / "a hardcoded 40 fails this row."

**Fix.** A per-pattern exemption on G5 only:
`derived|--show-object-format|would reject|rejects|fails this row`. This is the
narrowest rule that clears both: each of the two shapes names either the
derivation or the consequence of hardcoding, whereas an affirmative claim
(""`Tpatch-Base-Commit` is 40 lowercase hex"") names neither and still trips.

**Second-order correction found while proving sensitivity.** The rev-6 pattern
`40[- ]hex` did **not** match the exact pre-rev-5 phrasing it exists to catch —
"is 40 **lowercase** hex". The optional `(lowercase[- ])?` alternation closes
that gap, so the guard is now strictly more sensitive than in rev-6 despite the
new exemption.

## Exact guard output

Run against the three authoritative documents, G1–G10 with the four §7.1.2
whitelist rules and the per-pattern scoping/exemptions implemented as written:

```
GUARD HITS: 0
exit=0
```

**Synthetic sensitivity harness** — each stale phrase appended to a copy of
`PRD-tpatch-land.md`, each control sentence likewise:

```
── stale text MUST be caught ──
  CAUGHT  G5 affirmative fixed-40  ['G5']
  CAUGHT  G5 hardcode-40 affirm    ['G5']
  CAUGHT  G5 fixed 40 affirm       ['G5']
  CAUGHT  G1 V9-last               ['G1']
  CAUGHT  G2 ten-check             ['G2']
  CAUGHT  G3 stale label           ['G3']
  CAUGHT  G4 blanket unchanged     ['G4']
  CAUGHT  G6 absence mismatch      ['G6']
  CAUGHT  G7 exact+absent          ['G7']
  CAUGHT  G8 mutating nothing      ['G8']
  CAUGHT  G9 freshness_label       ['G9']
  CAUGHT  G10 stale E-range        ['G10']
── correct anti-hardcode text MUST pass ──
  PASS    G5 exempt: rejects       []
  PASS    G5 exempt: fails row     []
  PASS    G5 exempt: derived       []
  PASS    G5 exempt: flag          []
BASELINE HITS ON UNMODIFIED COPY: 0
SENSITIVITY: OK
exit=0
```

All twelve stale shapes are caught; all four correct anti-hardcode shapes pass;
the unmodified copy is clean, so the harness itself introduces no hits.

## Counts — unchanged from rev-6

| Metric | Value | Delta vs rev-6 |
|---|---|---|
| Verify acceptance matrix | **135 rows**, AC-L1 … AC-L135, contiguous | 0 |
| Verify groups | A 6 / B 10 / C 15 / D 21 / E 26 / F 20 / G 15 / H 22 = 135 | 0 |
| Land acceptance matrix | **26 rows**, AC-LD1 … AC-LD23, all tier C | 0 |
| **Total acceptance rows** | **161** | 0 |
| Unique code citations | **106**, all in range | 0 |
| ADR decisions | D1–D19; Amendment 1 = D8–D19, closed | 0 |
| Empirical index | E1–E47, contiguous, closed | 0 |
| Remediation strings | R1–R24 | 0 |
| Totality guard regexes | G1–G10 | 0 (G5 and G3 patterns refined) |

## Test Results

Planning-only revision — no Go source changed.

- **Totality guard**: `GUARD HITS: 0`, exit 0.
- **Synthetic sensitivity**: 12/12 stale shapes caught, 4/4 controls pass,
  0 baseline hits, exit 0.
- **Doc validator**: **ALL CHECKS PASS** — 106 citation ranges, AC-L
  contiguity (135) and group sum (135), AC-LD count (26) all tier C, dangling
  AC/D/R reference check, D1–D19 headings, E1–E47 contiguity, R1–R24
  definitions.
- Scratch removed; working tree contains only the four owned docs plus the
  guarded WIP files.

## Residuals for Wave C

Unchanged from rev-6:

1. **Q17** remains open (non-blocking, scoped in §6).
2. **`-C1` false reds** on a one-line-away healthy parent (E44) — accepted.
3. **Normalization collision** for identical payloads at different positions
   (E45/E46).
4. **Partial clone is a Wave C gate** (AC-L68 / AC-L69) — mechanism proven,
   real filtered-remote path unproven; blocker report required if it cannot be
   constructed.
5. **`(0/0)` block cost**: 26 false reds / 151 present vs 0 false greens / 69
   absent.
6. **`active` widening** and the **forward-mode V10 change** alter existing
   behaviour for non-landed features.
7. **Mode B is non-rollback** on an R23 refusal.
8. **`satisfied_by` is 40-hex-only in shipped code**
   (`internal/store/validation.go:22`) — pre-existing, recorded in §8, and
   deliberately outside G5's scope.
9. **Git floor 2.36** is a user-visible tightening.
10. **The guard is a docs test, not a linter** — it reads three fixed paths; a
    future authoritative document must be added to its input list explicitly.
11. **New, rev-7**: G3's character class must be widened by one on every
    revision bump. This is stated in the G3 row so it is not forgotten, but it
    is a manual step.

## Reviewer focus

1. **§7.1.2 G5**: confirm the exemption is the narrowest that clears land rule
   18 and AC-LD5 / AC-LD19, and that an affirmative "is 40 lowercase hex" claim
   still trips — the sensitivity harness proves both directions.
2. **Confirm nothing else moved**: 161 rows, 106 citations, D8–D19, E1–E47,
   R1–R24 all identical to rev-6; the only other edits are label bumps and two
   ADR revision-history entries.
3. **G3 widening**: confirm `rev-[0-6]` is correct now that the amendment is
   rev-7, and that only revision-history and rejected-alternatives narrative
   names earlier revisions.

## Context for Next Agent

- Wave B is **planning only**. Implementation is Wave C. Nothing in `internal/`,
  `cmd/`, `assets/`, `tests/`, `SPEC.md` or `CHANGELOG.md` was touched.
- The binding ADR is **ADR-013 Amendment 1 rev-7 (final), D8–D19** — closed.
  The operational contract is **PRD-verify-freshness §3.6 / §4.3.6–4.3.9 / §5 /
  §6 / §7.1 / §7.1.2**; the producer contract is **PRD-tpatch-land §3.8 / §6.2**.
- Load-bearing measurements are indexed **E1–E47** in ADR-013 §A1.1, closed.
- The check sequence is **eleven** checks, V0–V10
  (`internal/workflow/verify.go:49-71`, V10 appended at `:288-289`).
- V7 and V8 share one shadow but are **reset between** them
  (`resetShadowToTree`, `internal/workflow/verify.go:1142-1153`) — the GH #2 /
  v0.11.3 invariant.
- **AC-L135 / §7.1.2 is executable**: G1–G10, four whitelist rules, per-pattern
  scoping for G5 and G8, fence-scoped G9. Wave C should port the harness used
  here (guard + synthetic sensitivity) into a docs unit test.
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
