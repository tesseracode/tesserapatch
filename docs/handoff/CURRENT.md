# Current Handoff

## Status

**Cluster state**: REV-4 DISPATCHED

v0.15.1 Wave B rev-3 remains blocked by anchor syntax/qualification,
inventory, land producer timing, offline history and duplicate normalization.
Rev-4 is dispatched.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V0–V10 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: In Progress (rev-4)
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Dispatch HEAD (rev-3)**: `c2e7b85`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Files Changed (exact, this wave)

Owned and staged by explicit path — nothing else:

| File | Change |
|---|---|
| `docs/adrs/ADR-013-verify-freshness-overlay.md` | **Amendment 1 rev-3** replaces the rev-2 amendment in full: revision history, A1.1 empirical index **E1–E42** (E34–E42 new, E35 a self-correction, E27 the rev-1 retraction), decisions **D8–D19** (D14 anchor collection rewritten, D15 per-member V10 + Q15, D16 shallow/partial, D17 full inventory + budget, **D18 normalized identity**, **D19 land producer contract**), **41 rejected alternatives**, consequences and a validated reference block. The D7 note is updated for per-member anchors. |
| `docs/prds/PRD-verify-freshness.md` | **§3.6 rewritten** as §3.6.1–§3.6.9 (three-state presence precedence, derived commit-id length, collect-qualify-compare-select anchors, normalized identity, per-member V10 + provenance, full inventory, shallow/partial states, **24 remediations R1–R22 + R24**); **§4.3.6–§4.3.9 rewritten** with `repository`, `candidates_collected/qualified`, `patch_presence`/`recipe_presence`, `member_baselines`, `provenance_hash_bound`, and a **13-value `failed_at`** + **5-value advisory** vocabulary; **§5 rebuilt**; **§6 Q15 RESOLVED, Q17/Q18 added**; **§7.1 rebuilt to 128 rows**; §8 gains five rev-3 risk rows; two stale 10-row prose remnants corrected. |
| `docs/prds/PRD-tpatch-land.md` | **§3.8.2 gains rules 18–19** (derived commit-id length; shallow/partial ≠ root topology); **new §3.8.6 producer precondition** with **R23**; **§6.2 extended to 21 rows**; header/Related updated to rev-3. |
| `docs/handoff/CURRENT.md` | This file. |

Not touched: `internal/`, `cmd/`, `assets/`, `tests/`, `SPEC.md`,
`CHANGELOG.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`, GitHub issue #8.

## How rev-3 closes every rev-2 residual

| rev-2 residual | rev-3 closure |
|---|---|
| **1 HIGH — anchor selection says stop-at-first but must compare all qualifiers** | **D14**: **collect → qualify → compare → select**. Every reachable single-parent slug-bearing candidate is collected from the one enumeration; each is qualified; only then are normalized identities compared; differing identities ⇒ `historical-anchor-unavailable`. `candidates_collected` / `candidates_qualified` are reported. AC-L29, AC-L34, AC-L35. |
| **2 HIGH — parent V10 evaluated at a baseline that may already contain the parent's postimage** | **D15**: **every landed member resolves its OWN replay anchor** and V10 runs there; `checks[].member_baselines` proves it in the report. AC-L92, AC-L93. |
| **3 HIGH — snapshot omits later-feature metadata needed by ADR-029** | **D17**: the inventory covers **every** feature from `store.ListFeatures()` — status + `RequestedAt`, three artifacts' presence and bytes, and `patch-generations.touched_paths` — because `loadLaterFeatureTouches` reads the whole repo (`writefile_safety.go:409-442`). Add/remove/change detection, read-error policy, deterministic order and cost are all defined. AC-L107–AC-L112. |
| **4 HIGH — empty/absent patch states overlap; exact precedence ambiguous** | **D10**: three closed mutually exclusive states — **absent / present-empty / present-nonempty** — evaluated **before** any digest comparison; `exact` authority requires **present-nonempty** because `land` refuses empty; the recipe's four shapes are named separately. AC-L51–AC-L53. |
| **5 HIGH — `land` can emit an invalid Base-Commit on legacy/corrupt `--no-record` state** | **D19** / `PRD-tpatch-land` §3.8.6: `land` validates non-empty, well-formed (derived length), resolvable and reachable **before any commit**, refusing with **R23** and mutating nothing. Migration stated; `base_commit` still never written by `land`. AC-LD18–AC-LD21. |
| **6 MEDIUM — shallow/partial misclassified as unsupported topology** | **D16**: a preflight (`--is-shallow-repository`, `--show-object-format`, promisor config) runs before topology classification; new closed states **`shallow-history`** (R21 `git fetch --unshallow`) and **`history-incomplete`** (R22). Measured: a `--depth 2` boundary reports 0 parents exactly like a root and fails `read-tree ^` with the same text. AC-L66, AC-L67, AC-L69. |
| **7 MEDIUM — anchor qualification inherits reverse-ladder false reds** | **D14 step 2**: qualification is a **forward** `git apply --check --cached` at the candidate's parent tree. Measured: forward passes and reverse fails at a clean pre-landing tree; the inverse holds at the landing. The honest bound (forward apply is context-sensitive) is stated rather than over-claimed. AC-L30–AC-L32. |
| **8 MEDIUM — raw duplicate comparison rejects healthy cherry-pick/merge-back** | **D18**: normalized identity = `git diff … --unified=0 <C>^ <C> -- <P…>` with **only** `^index ` lines removed, SHA-256 over the rest. Measured: raw `-U3` differ, `-U0` still differ, `-U0` minus `index` are **identical**; mode / binary / rename semantics are preserved. AC-L42–AC-L45. |
| **9 MEDIUM — Q15 ignores recipe provenance; stale V10 prose** | **D15 / Q15 RESOLVED**: `RecipeProvenance.BaseCommit` (`implement.go:30-34`, written `:220-238`) anchors forward-mode V10 under four gates — present, well-formed, reachable, inventory-consistent — else **FAIL `recipe-provenance-unavailable`** (R24), never the live tree. Stale V10 and 10-row prose corrected. AC-L96–AC-L101. |

## Decisions Made (binding, all with recorded alternatives)

ADR-013 **Amendment 1 rev-3**, decisions **D8–D19**:

1. **D8** — eleven checks V0–V10; no new check ID.
2. **D9** — dual anchor; V7's independent obligation stated.
3. **D10** — one raw+parsed `--topo-order --reverse` enumeration; strict
   cardinality; **object-format-derived** commit-id length (40 `sha1` / 64
   `sha256`); **three-state presence precedence**; ten closed evidence states.
4. **D11** — anchor C index-isolated (`read-tree` + `--cached`), removed on
   every exit path.
5. **D12** — hardened ladder; `(0/0)` blocks; `LC_ALL=C` mandatory.
6. **D13** — parent arbitration non-mutating and patch-ladder-only; recipe
   predicates diagnostic; `active` total.
7. **D14** — **collect all candidates, qualify by FORWARD apply, compare
   normalized identities, then select**; stale candidates supply a tree only;
   unavailability terminal; re-land regains H.
8. **D15** — **per-member V10 baselines**; `RecipeProvenance.BaseCommit`
   anchors forward mode; `recipe-provenance-unavailable`; parent aggregation.
9. **D16** — preflight-driven topology; `shallow-history` and
   `history-incomplete` as first-class states.
10. **D17** — **full repository metadata inventory**, instability detection,
    read-error policy, deterministic order, cost, and an honest nine-line
    invocation budget including tree materialization.
11. **D18** — **normalized change identity**: exact args, `^index `-only
    strip, mode/binary/rename preserved, empty path set ⇒ ambiguous.
12. **D19** — **`land` refuses an invalid `Tpatch-Base-Commit`** before
    committing; `base_commit` still never written by `land`.

**41 rejected alternatives** recorded in the ADR.

## Acceptance Matrix

- **`PRD-verify-freshness` §7.1 — 128 rows, `AC-L1` … `AC-L128`**, contiguous
  and unique (machine-checked), across 8 groups: A defect + schema (6),
  B anchor-C isolation (7), C hardened ladder (15), D anchor
  collection/qualification/normalization (17), E reader/grammar/presence/
  topology/shallow (26), F closure arbitration (20), G V10 + provenance (15),
  H inventory/schema/run-level (22).
- **`PRD-tpatch-land` §6.2 — 21 rows, `AC-LD1` … `AC-LD21`**, all tier C.
- **Total: 149 numbered acceptance criteria** (rev-2: 135; rev-1: 121).
- **Tiers**: **U** = pure functions plus the **inventory** and
  **evidence-reader** abstractions; **W** = workflow integration with a
  **`PATH` git wrapper** for argv/env capture, error injection and
  between-call mutation (demonstrated working); **C** = real CLI. **No
  production seam, no build tag, no exported hook.**

## Test Results

Planning wave — no code changed, so no build/test delta is claimed.

- Working tree: no tracked source file modified.
- **Citation validation**: **212 `internal/**.go:line` citations, 90 unique**,
  machine-checked in range, then every unique anchor printed and semantically
  spot-checked against the source at `c2e7b85`.
- **Cross-reference validation**: every self-`§` reference resolves in its own
  document; ADR `D1`–`D19` all present; all `AC-L*` references resolve;
  `R1`–`R22` and `R24` defined in the verify PRD with no dangling reference,
  and `R23` defined in the land PRD (deliberately split by owner).
- **Matrix count validation**: 128 unique contiguous `AC-L` with a tier cell on
  every row; 21 unique contiguous `AC-LD`.
- **Staleness sweep**: `"freshness_label"` in verify JSON samples = **0**;
  the two remaining 10-row prose remnants corrected (one rewritten to
  "eleven-row", one annotated as a historical Slice A note); the only surviving
  `10-row` string is AC-L5, which exists to forbid it.
- **Counts re-derived from the docs**: matrix groups 6/7/15/17/26/20/15/22 =
  128; 41 ADR rejected alternatives; 42 A1.1 empirical rows (E1–E42,
  contiguous, no dangling reference); 19 land reader rules; 13 `failed_at`
  values; 5 advisory codes; 10 evidence states; 23 remediation strings across
  the two PRDs.
- Side Research md5: `b385fe622db9926f48861105239f113e` — preserved
  byte-identical.

## Empirical Validation (read-only probes; scratch removed)

git 2.55.0 / macOS, throwaway repos, all deleted. rev-3 added E34–E42.

**Forward vs reverse qualification (E34/E35).** At a clean pre-landing parent
tree: forward `git apply --check --cached` **OK**, reverse **FAIL**. At the
landing itself: forward **FAIL**, reverse **OK**. Qualification is the forward
question. **Self-correction (E35)**: a candidate parent carrying unrelated
drift inside the patch's context window does **not** forward-qualify — forward
apply is context-sensitive like any `git apply`, and the contract says so
rather than claiming immunity.

**Normalization (E36/E37).** Same logical change on two branches with
different preimage blobs: raw `-U3` hashes **differ**; `-U0` alone still
**differs** (`index 58b8997..c3badef` vs `index ed24a75..1b142d1`); `-U0` with
`^index ` stripped is **EQUAL**. The normalized form still carries `old
mode`/`new mode` (4 lines), `GIT binary patch` (1) and the `new file mode` /
`deleted file mode` rename split (2).

**Shallow and partial (E38/E39/E40).** In a `--depth 2` clone the boundary
commit reports `parents_in_%P = 0` **exactly like a real root**, is marked
`(grafted)`, appears in `.git/shallow`, and `read-tree <boundary>^` fails with
the *same* `fatal: Not a valid object name` text as a true root; a full repo
has no `.git/shallow`. A blobless partial clone reports `is-shallow=false`,
`promisor=true`, has commits and trees locally and fetches blobs lazily.

**Object format (E41).** `git rev-parse --show-object-format` → `sha1` with
40-hex ids; a repository created with `--object-format=sha256` → `sha256` with
**64**-hex ids.

**Provenance (E42).** `RecipeProvenance` (`implement.go:30-34`) records
`base_commit` + `recipe_sha256`; `implement` writes it (`:220-238`) on the
same path that emits `preimage_hash`, while the autogen path that omits
`preimage_hash` writes no provenance — the two travel together, and
`warnRecipeStale` (`cobra.go:1285-1315`) already consumes both.

Carried forward: the op-kind × landed matrix; the `C3`/`C0`/`(0/0)` ladder
table; **0 false greens / 69 absent** hardened vs **2 / 69** unhardened, at
**26 false reds / 151 present**; dirty-worktree false red; temp-index
read-only guarantees; anchor recovery after re-land; one-call enumeration with
no `rev-list`; the existential-inverse predicate at 0/0 over 52 416 cases.

## Open Residuals

1. **Q17 (open, non-blocking)** — recipes carrying `preimage_hash` produced
   before the provenance sidecar existed now **fail closed** with
   `recipe-provenance-unavailable` instead of silently mis-verifying.
   Remediation is one command (`tpatch implement <slug>`). A back-fill from
   `status.apply.base_commit` was considered and rejected: that field is
   `record`-owned, operator-mutable and not hash-bound to the recipe bytes.
2. **Q18 (open, non-blocking)** — the `-U0` hunk header retains git's
   function-context suffix. Deliberately not stripped; the failure mode of
   keeping it is a conservative `ambiguous`, never a false equivalence.
3. **The mandatory `(0/0)` block costs 26 false reds per 151 present trees.**
   Quantified and deliberate; each carries R2.
4. **`active` closure widening is a behaviour change for non-landed
   features**, pinned by AC-L86/AC-L87.
5. **Forward-mode V10 changes behaviour** for ops with `preimage_hash` (Q15
   fix). Ops without it are untouched.
6. **`land` gains a refusal path** (R23) on legacy/corrupt status; the
   successful path is byte-unchanged (AC-LD21).
7. **Pre-existing citation drift outside the amended sections** —
   `PRD-verify-freshness` still carries stale anchors in *unamended* prose
   (`gitutil.go:680`, `store.go:232`, `types.go:91`, `types.go:192`,
   `labels.go:89`, `labels.go:143`, `dependency_gate.go:79` in §3.1,
   `validation.go:66`). Outside this wave's scope; flagged so a reviewer does
   not mistake them for new drift.
8. **Git floor asserted from docs plus a 2.55.0 run**, not a matrix against
   2.22/2.25.
9. **CHANGELOG entry** intentionally absent — belongs to the Wave C ship.
   **ROADMAP row and LOG verdict** are supervisor-owned.

## Reviewer Focus

Ordered by risk:

1. **Is collect-qualify-compare-select internally consistent?** Try to
   construct a history where the loop selects before observing a second
   qualifier, or where a stale-hash candidate leaks into
   `landing_evidence.state`.
2. **Is forward qualification the right instrument, and is its bound honest?**
   E35 is a self-correction; confirm no row claims immunity to context.
3. **Does the normalized identity ever equate genuinely different changes?**
   Only `^index ` is stripped. Probe mode-only, binary-only and rename cases,
   and the Q18 funcname suffix.
4. **Are per-member V10 baselines actually independent?** `member_baselines`
   should show a landed parent's baseline differing from the target's; look
   for a path where the target's anchor is reused.
5. **Is the provenance gate (Q15) sound?** Four conditions, `recipe_sha256`
   tolerance for pre-v0.5.2 sidecars, and a hard fail rather than a live-tree
   read. Confirm the fail-closed direction is right and Q17's population is
   bounded.
6. **Is the inventory both sufficient and bounded?** It must cover everything
   `loadLaterFeatureTouches` reads, detect add/remove/change, and stay O(F).
7. **Is the presence precedence genuinely closed?** Three patch states, four
   recipe shapes, no overlap, no unreachable row, no double-vacuous pass.
8. **Is the shallow/partial discrimination correct?** A shallow boundary and a
   true root are indistinguishable by `%P`; confirm the preflight is the only
   discriminator used and that CI shallow gets R21.
9. **Is D19 the minimal producer change?** It adds a refusal and nothing else;
   confirm AC-LD21 really pins the successful path.
10. **Does the GH #2 invariant survive?** AC-L121/AC-L122.
11. **Are all 149 rows executable at their stated tier** with only the §3.6.9
    primitives, the `PATH` wrapper and the inventory/reader abstractions?
12. **Citation and count spot-check** against `c2e7b85`; residual 7 lists the
    known pre-existing drift that is *not* this wave's.

## Rev-3 Review Adjudication

- Internal: NEEDS REVISION.
- External/original reproducer: APPROVED WITH NOTES, requiring folds before
  Wave C.
- Required corrections:
  1. Fix invalid parent-tree revision syntax.
  2. Use `ListFeatureEntries`, not error-dropping `ListFeatures`.
  3. Define producer-side Base-Commit validation timing honestly.
  4. Anchor qualification uses measured `-C1` forward apply.
  5. Normalize positional hunk metadata for duplicate equivalence.
  6. Parent V10 uses each parent's own baseline.
  7. Add offline `GIT_NO_LAZY_FETCH` partial-clone behavior.
  8. Reconcile empty-patch states, stale V10 prose and AC-LD contradictions.

## Next Steps

1. Fold all rev-3 contract corrections and rebuild matrix/parity.
2. Run final dual planning review.
3. Accept Wave B and dispatch Wave C from a fresh base.

## Blockers

None.

## Context for Next Agent

- Wave B is **planning only**. Wave C owns implementation and dispatches from
  a fresh base.
- The binding contract is `ADR-013` **Amendment 1 rev-3 (D8–D19)** plus
  `PRD-verify-freshness` **§3.6** and **§7.1**. `PRD-tpatch-land` §3.8.1–§3.8.5
  is a readers' contract; **§3.8.6 is the one producer change** and AC-LD21
  guards that nothing else about `land` moves.
- Four corrections to the original diagnosis, all measured: **the defect is not
  V8-only**; **the check set is eleven, not ten**; **the current assertion must
  never read the working tree**; and **anchor qualification is a forward
  question, not a reverse one**.
- Reverse-apply proves **materialization modulo context and offset**, never
  ownership and never byte equality; a match surviving only with all context
  discarded is **not** certified.
- **Q15 is resolved**: `RecipeProvenance.BaseCommit` is the forward-mode V10
  anchor. rev-2's "no anchor exists" claim was wrong.
- Verify stays read-only; no status/index/worktree mutation on any path.
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
