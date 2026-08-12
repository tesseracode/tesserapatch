# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave B **rev-5** is written and pushed. All five rev-4 corrections are
closed: the effective Git floor is **2.36**, artifact presence **short-circuits
before** evidence classification, Mode A's no-mutation promise is scoped around
the mandatory `recoverLand` step, the real-filtered-remote partial-clone path is
a **Wave C acceptance gate** rather than a claimed fact, and every authoritative
document has been swept for stale prose. Planning only — no Go, tests, assets,
SPEC, CHANGELOG or GitHub issue state was touched.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V0–V10 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: Review (rev-5)
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Dispatch HEAD (rev-5)**: `4c632b6`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Files Changed (rev-5) — exact list

| File | What changed in rev-5 |
|---|---|
| `docs/adrs/ADR-013-verify-freshness-overlay.md` | **D10** presence states short-circuit before classification + reachable 3×4 patch×recipe outcome table; **D13** the two unreachable `exact + absent/empty` arbitration rows removed, replaced by one short-circuit row; **D16** "Scope of what is proven, without overclaim" — partial-clone mechanism proven, end-to-end path unproven, made a Wave C gate; **D17** Git floor rewritten to **2.36** with a capability table (2.22 / 2.25 / 2.29 demoted to historical component facts); **D19 Mode A** rewritten against the shipped `runLand` ordering with a two-case guarantee table; rev-5 labels, `4c632b6` reference anchor, rev-5 revision-history entry; `land.go` preflight/gate anchors corrected to `:110` / `:116`. |
| `docs/prds/PRD-verify-freshness.md` | **§3.6.2** presence table rewritten with short-circuit semantics + 3×4 reachable outcome table, `stale` now requires `present-nonempty`; **§3.6.6** unreachable arbitration rows collapsed to one; **§3.6.8** partial-clone scope-of-proof paragraph + Wave C gate; **§3.6.9** Git-floor block rewritten to 2.36 with the PATH-shim proof; **R10** names 2.36 and the three sub-capabilities; **§5** two landed rows rewritten; **§7.1** AC-L52 / AC-L53 / AC-L68 / AC-L69 / AC-L79 / AC-L80 rewritten, **AC-L134 added** (Git-floor preflight), matrix header recount and Wave C gate note; **§8** risk row now "below 2.36"; historical Wave 3 10-check row annotated; rev-5 labels. |
| `docs/prds/PRD-tpatch-land.md` | **§3.8.2 rule 6** `Tpatch-Base-Commit` length is object-format-derived (40 `sha1` / 64 `sha256`), not 40-hex-only; **rule 14** floor raised to **2.36** with the component capabilities demoted; **§3.8.6 Mode A** rewritten with the shipped `recoverLand`-first ordering and the two-case guarantee table plus the pending-journal **R23** note; **AC-LD18** narrowed to the no-journal case and **AC-LD18c added** for the pending-journal case; matrix header 24 → **25**; rev-5 labels; `AC-L1–AC-L134`. |
| `docs/handoff/CURRENT.md` | This rewrite. Side Research tail preserved byte-identically (md5 `b385fe622db9926f48861105239f113e`). |

## Decisions closed in rev-5

1. **Git floor is 2.36 for the whole landed contract.** `GIT_NO_LAZY_FETCH`
   (2.36) is mandatory on every object and materialization command, so it — not
   `%(trailers:key=…,valueonly)` (2.22), `separator=` (2.25) or
   `--show-object-format` (2.29) — sets the operative floor. Below it the run
   reports evidence `unavailable` with **R10** and issues **only** `--version`:
   no `log`, `read-tree`, `apply`, `diff`, `cat-file`, `merge-base`, and no
   network. Measured with a PATH shim. The sub-capabilities survive as
   historical component facts only.
2. **Artifact presence short-circuits before classification.** Evidence first
   computes patch state `absent` / `present-empty` / `present-nonempty`. Only
   `present-nonempty` can reach `exact` or `stale` by digest; `absent` and
   `present-empty` are terminal **`landed-artifacts-absent`** *before* any digest
   comparison, regardless of recipe shape. The 3×4 patch×recipe table is now
   reachable and mutually exclusive in all twelve cells, and the rev-4 rows that
   let a recipe be "sole authority" under an absent patch are gone.
3. **Mode A promises no *new* mutation, not absolute command-entry
   immutability.** `recoverLand` (`internal/cli/land.go:127`,
   `land_journal.go:437-445`) is mandatory first by GH #7 and may publish a
   retained index, CAS the branch back and restore `status.json` while finishing
   a *prior* transaction. Base-Commit validation is inserted immediately after
   it returns. Two cases: **no journal** ⇒ refusal mutates nothing at all;
   **pending journal** ⇒ recovery may have mutated, and the refusal makes no NEW
   mutation and names the completed recovery.
4. **Partial-clone behaviour is a Wave C acceptance gate, not a Wave B claim.**
   The offline *mechanism* is measured (E47: default git attempts the network on
   a missing promisor object; `GIT_NO_LAZY_FETCH=1` fails locally). The
   end-to-end path is **not** — a `file://` `--filter=blob:none` clone on git
   2.55.0 does not withhold blobs. **AC-L68/AC-L69** now require a real filtered
   remote (non-local transport with `uploadpack.allowFilter=true`, or a
   deterministic promisor fixture); if Wave C cannot construct one it must
   report a **blocker** and may not mark those rows passed.
5. **Global staleness sweep across whole documents**, not amended fragments:
   no `V9-last` / 10-row shape survives outside the one row that forbids it; the
   ADR header now reads rev-5 D8–D19; the land PRD's 40-hex-only grammar is
   object-format-derived; `C^{tree}^` appears only in negative framing; no
   `"freshness_label"` appears in any verify sample; Q and residual ids are
   unique; `AC-L1–AC-L134` everywhere.

## Preserved unchanged from rev-1 … rev-4

`-C1` forward anchor qualification (E44) · the `(0/0)` block rule (E26/E28/E29,
0 false greens) · exhaustive anchor collection with no stop-at-first ·
per-member V10 baselines with `RecipeProvenance.BaseCommit` (E42, resolves Q15)
· the `ListFeatureEntries` full inventory · `GIT_NO_LAZY_FETCH=1` on every
object command (E47) · duplicate identity by index-drop + `@@` header
normalization (E45/E46) · the exhaustive existential-inverse `replace-in-file`
predicate (0 false reds / 0 false greens over 52,416 cases) · the Mode A /
Mode B producer split · `C^` parent syntax (E43) · eleven checks V0–V10.

## Counts

| Metric | Value |
|---|---|
| Verify acceptance matrix | **134 rows**, AC-L1 … AC-L134, contiguous, groups A 6 / B 9 / C 15 / D 21 / E 26 / F 20 / G 15 / H 22 |
| Land acceptance matrix | **25 rows**, AC-LD1 … AC-LD22 incl. AC-LD18a/b/c, all tier C |
| **Total acceptance rows** | **159** (rev-4: 157) |
| Unique code citations | **102**, every `file:line[-range]` in range — machine-checked |
| ADR decisions | D1–D19, no gaps; Amendment 1 = D8–D19 |
| Empirical index | E1–E47, contiguous |
| Remediation strings | R1–R24, all referenced ids defined |

## Test Results

Planning-only revision — no Go source changed, so no build or test run is
implicated. Validation was documentary and empirical:

- Doc validator (`.scratch-gh8/v.py`, removed after the run): **ALL CHECKS
  PASS** — citation range check over 102 anchors, AC-L contiguity + tier column
  well-formedness, AC-LD uniqueness, dangling AC/D/R/E reference check, E-index
  contiguity, staleness greps, unique Q numbering, `C^{tree}^` negative-framing
  check.
- Semantic spot-checks printed the source line for every newly cited anchor;
  two were off by one (`landPreflight` `:111`→`:110`,
  `CheckDependencyGate` `:115`→`:116`) and were corrected in both documents.
- Below-floor PATH-shim probe re-run for rev-5: only `--version` is invoked
  before the refusal.
- Scratch removed; working tree contains only the four owned docs plus the
  guarded WIP files.

## Residuals for Wave C

1. **Q17** remains open (non-blocking, scoped in §6).
2. **`-C1` false reds**: a one-line-away healthy parent is rejected by the
   forward ladder (E44). Measured and accepted — safety over reach.
3. **Normalization collision**: `@@`-rewritten identity treats the same payload
   at different positions as equivalent (E45/E46). Collides only when the
   `-`/`+` bodies are byte-identical, which needs duplicate line content.
4. **Partial clone is a Wave C gate** (AC-L68/AC-L69). Mechanism proven, path
   unproven; a real filtered remote or a blocker report is required.
5. **`(0/0)` block cost**: 26 false reds over 151 present cases, against 0 false
   greens over 69 absent cases. Deliberate.
6. **`active` widening** and the **forward-mode V10 change** alter existing
   behaviour for non-landed features; both are intended and matrixed.
7. **Mode B is non-rollback**: a completed embedded `record` is retained on an
   R23 refusal, and says so.
8. **Pre-existing citation drift** elsewhere in these PRDs (outside the amended
   sections) was not swept — out of scope for a landed-contract revision.
9. **Git floor 2.36** is a user-visible tightening; Wave C should surface it in
   release notes.

## Reviewer focus

1. **§3.6.2 / D10 short-circuit**: confirm all twelve patch×recipe cells are
   reachable, mutually exclusive, and that no surviving row lets an absent or
   empty patch produce `exact`, `stale`, or a recipe-only authority (AC-L52,
   AC-L53, AC-L79, AC-L80).
2. **§3.8.6 / D19 Mode A**: confirm the ordering matches shipped `runLand` and
   that no row still claims absolute command-entry immutability (AC-LD18,
   AC-LD18c).
3. **Git floor**: confirm 2.36 is used as the *overall* floor everywhere and
   that 2.22 / 2.25 / 2.29 appear only as component history (AC-L134, R10, land
   rule 14).
4. **AC-L68/AC-L69 framing**: confirm the Wave C gate wording forbids marking
   them passed on a synthetic fixture.
5. **Counts**: 134 + 25 = 159 rows; 102 citations; groups sum to 134.

## Context for Next Agent

- Wave B is **planning only**. Implementation is Wave C. Nothing in `internal/`,
  `cmd/`, `assets/`, `tests/`, `SPEC.md` or `CHANGELOG.md` was touched.
- The binding ADR is **ADR-013 Amendment 1 rev-5, D8–D19**. The operational
  contract is **PRD-verify-freshness §3.6 / §4.3.6–4.3.9 / §5 / §6 / §7.1**; the
  producer contract is **PRD-tpatch-land §3.8 / §6.2**.
- Load-bearing measurements are indexed **E1–E47** in ADR-013 §A1.1. Re-measure
  before contradicting any of them.
- The check sequence is **eleven** checks, V0–V10
  (`internal/workflow/verify.go:49-71`, V10 appended at `:288-289`).
  `VerifyReport` has **no** `freshness_label` field.
- Key shipped constraints: `checkWriteFilePreimage`
  (`internal/workflow/writefile_safety.go:108-112`) reads the **live working
  tree**; `ListFeatures` (`internal/store/store.go:210-236`) **silently drops**
  unreadable features, which is why the inventory uses `ListFeatureEntries`
  (`:274-348`); `RecipeProvenance` (`internal/workflow/implement.go:30-34`)
  already ships and anchors forward-mode V10.
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
