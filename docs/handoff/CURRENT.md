# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave B **rev-4** (GH #8 contract) is rewritten and ready for dual
independent review. Planning only — no Go code, tests, assets, SPEC,
CHANGELOG or GitHub issue state was touched. Implementation is Wave C.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V0–V10 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: Review (rev-4)
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Dispatch HEAD (rev-4)**: `6d87198`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Files Changed (exact, this wave)

Owned and staged by explicit path — nothing else:

| File | Change |
|---|---|
| `docs/adrs/ADR-013-verify-freshness-overlay.md` | **Amendment 1 relabelled rev-4.** D14 rewritten (`C^` syntax, `-C1` forward qualification with the measured ladder); D11 and D16 gain mandatory `GIT_NO_LAZY_FETCH=1` and preflight-before-parent-count ordering; D17 rebuilt on `ListFeatureEntries` with an explicit read-error policy; D18 gains the hunk-header rewrite and the measured trade; **D19 rewritten as a mode-split producer contract**. Empirical index extended to **E1–E47**; **47 rejected alternatives**; references gain the `store.go` anchors. |
| `docs/prds/PRD-verify-freshness.md` | §3.6.5 exact commands + offline env + invalid-syntax note; §3.6.8 `-C1` ladder table, `C^` syntax, hunk-header normalization and the measured trade; §3.6.8 preflight ordering and offline classification; §3.6.9 inventory rebuilt on `ListFeatureEntries` with the read-error table; budget rows updated; **Q18 RESOLVED**; **§7.1 extended to 133 rows** (AC-L129–AC-L133). |
| `docs/prds/PRD-tpatch-land.md` | **§3.8.6 rewritten as Mode A / Mode B**; **§6.2 extended to 24 rows** with AC-LD18a/AC-LD18b/AC-LD22 and **AC-LD15 reworded** so it no longer contradicts AC-LD18/AC-LD21; header/labels to rev-4. |
| `docs/handoff/CURRENT.md` | This file. |

Not touched: `internal/`, `cmd/`, `assets/`, `tests/`, `SPEC.md`,
`CHANGELOG.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`, GitHub issue #8.

## How rev-4 closes every rev-3 finding

| rev-3 finding | rev-4 closure |
|---|---|
| **1 CRITICAL — `C^{tree}^` is invalid** | **D14 / §3.6.5**: the normative revision is **`C^`** (or `C^^{tree}` for an explicit tree). Measured (E43): `git rev-parse C^{tree}^` → `error: object <tree> is a tree, not a commit`; `git read-tree C^{tree}^` fails; `C^`, `C^^{tree}`, `C~1`, `C~1^{tree}` all resolve and `read-tree` accepts them. Grepped: the string survives only inside the two paragraphs that name it as invalid. AC-L130. |
| **2 HIGH — `ListFeatures` silently drops unreadable status** | **D17 / §3.6.9**: the inventory is built from **`ListFeatureEntries`** (`store.go:274-345`), which returns `FeatureEntry{Slug, Status, Err}` (`:238-245`) sorted by slug (`:343-345`). `ListFeatures` skips at `store.go:226` and is explicitly rejected. Unreadable entries are retained as `Err` rows; target/closure ⇒ **block** `inventory-unreadable`, unrelated ⇒ **`warn` advisory + exclusion from ADR-029 ordering**, never a silent skip. AC-L107, AC-L110, AC-L111. |
| **3 HIGH — land refusal cannot promise "no mutation" after embedded record** | **D19 / §3.8.6 split by invocation mode.** **Mode A (`--no-record`)**: validate at command entry, refuse with R23 **mutating nothing**. **Mode B (embedded `record`)**: `record` owns the invariant — it must write a valid `BaseCommit` before its own first mutation — and `land` re-validates the reloaded value immediately after `record` returns, **before any `land`-owned mutation**; on failure **`record`'s artifacts persist as an independent completed transaction** and the message says so. AC-LD18, AC-LD18a, AC-LD18b; AC-LD15 reworded. |
| **4 HIGH/MEDIUM — default-context qualification dead-ends its own remediation** | **D14 / §3.6.8**: qualification is `git apply --check --cached -C1`. Measured ladder (E44): 2-lines-away noise `C3=FAIL / **C1=OK**`; 1-line-away `FAIL/FAIL`; already-materialized, feature-line-changed and feature-line-deleted **all FAIL at `-C1`**. `-C0` rejected as over-permissive. AC-L131, **AC-L132 remediation-loop fixture**. |
| **5 MEDIUM — duplicate identity retains hunk positions** | **D18 / §3.6.8**: exactly two post-processing rules — drop `^index `, rewrite every `^@@ … @@.*$` to a bare `@@`. Measured (E45): a cherry-pick after 5 prepended lines gives `@@ -10 +10 @@ l9` vs `@@ -15 +15 @@ l9` and is **rejected** without the rewrite, **equal** with it. Trade measured and bounded (E46). AC-L133. |
| **6 HIGH — parent V10 must use its own baseline** | Already binding in D15/§3.6.7 and re-verified this revision: landed member ⇒ its own replay anchor; unlanded applied/active ⇒ `RecipeProvenance.BaseCommit`; **never** the target's anchor. Aggregation and malformed/unavailable outcomes unchanged. AC-L92, AC-L93, AC-L106. |
| **7 HIGH — lazy fetch contradicts offline verify** | **D11/D16/§3.6.5**: **every** object and materialization command carries `GIT_NO_LAZY_FETCH=1`. Measured (E47): with a promisor remote and its object absent, `git cat-file -p` **attempts the network** (`does not appear to be a git repository`); under the variable it fails locally (`Not a valid object name`). Missing object ⇒ `history-incomplete`, no network. Shallow preflight now explicitly **precedes** any parent-count branch. AC-L129, AC-L67, AC-L69. |
| **8 HIGH/MEDIUM — presence precedence, stale prose, AC-LD conflict** | Presence states get an **adversarial exclusivity check** over the 3×4 patch/recipe cross-product (AC-L53). AC-LD15 reworded to "every **pre-existing** refusal is unchanged … not claimed behaviour-neutral in the presence of an invalid base commit", removing the contradiction with AC-LD18/AC-LD21. Q18 resolved; Q15 already resolved and its residual removed. |

## Decisions Made (binding)

ADR-013 **Amendment 1 rev-4**, decisions **D8–D19**. Changed this revision:

1. **D11** — temp-index commands stated exactly, with `GIT_NO_LAZY_FETCH=1`
   mandatory and `C^{tree}^` explicitly forbidden.
2. **D14** — anchor qualification is `read-tree C^` + `apply --check --cached
   -C1`; the ladder is tabulated from measurement; `-C3` and `-C0` are both
   rejected with reasons.
3. **D16** — preflight runs **before** any parent-count branch; missing objects
   are detected locally because of the offline env.
4. **D17** — inventory from `ListFeatureEntries`, slug-sorted, `Err` rows
   retained, read-error policy tabulated, `Err`↔`Status` flips count as
   instability.
5. **D18** — two normalization rules (`index` drop + `@@` rewrite), with the
   collision bound measured.
6. **D19** — producer validation split into Mode A (no mutation) and Mode B
   (record's artifacts persist), with `record` owning the pre-mutation
   invariant.

Unchanged and still binding: D8 eleven checks, D9 dual anchor, D10 grammar and
three-state presence with derived commit-id length, D12 the `(0/0)` block,
D13 patch-ladder-only arbitration with `active` total, D15 per-member V10 and
`RecipeProvenance` (Q15).

**47 rejected alternatives** recorded in the ADR, including the six new
rev-3-specific ones (invalid syntax, `-C3`, `-C0`, `ListFeatures`, lazy fetch,
and the unkeepable "mutating nothing" promise).

## Acceptance Matrix

- **`PRD-verify-freshness` §7.1 — 133 rows, `AC-L1` … `AC-L133`**, contiguous
  and unique (machine-checked). Groups: A 6, B 8, C 15, D 21, E 26, F 20,
  G 15, H 22. AC-L129–AC-L133 are appended to groups B and D.
- **`PRD-tpatch-land` §6.2 — 24 rows, `AC-LD1` … `AC-LD22`** including
  `AC-LD18a` and `AC-LD18b`, all tier C.
- **Total: 157 numbered acceptance criteria** (rev-3: 149; rev-2: 135).
- **Tiers**: **U** = pure functions plus the inventory and evidence-reader
  abstractions; **W** = workflow integration with a **`PATH` git wrapper** for
  argv/env capture, call ordering, error injection and between-call mutation;
  **C** = real CLI. No production seam, no build tag, no exported hook.

## Test Results

Planning wave — no code changed, so no build/test delta is claimed.

- Working tree: no tracked source file modified.
- **Citation validation**: **229 `internal/**.go:line` citations, 96 unique**,
  machine-checked in range, every unique anchor printed and semantically
  spot-checked against the source at `6d87198`.
- **Cross-reference validation**: every self-`§` reference resolves; ADR
  `D1`–`D19` present; all `AC-L*` references resolve; `R1`–`R22` and `R24`
  defined in the verify PRD with no dangling reference; `R23` defined in the
  land PRD (owner-split, asserted).
- **Matrix count validation**: 133 unique contiguous `AC-L` with a tier cell on
  every row; 24 `AC-LD` rows with `AC-LD18a`/`AC-LD18b` inserts.
- **Staleness sweep**: `"freshness_label"` in verify JSON = **0**; no
  `V9-last`, no 10-row claim outside AC-L5's prohibition; `C^{tree}^` survives
  only in the two paragraphs that declare it invalid; every `rev-list` mention
  is a negation; `ListFeatures` appears only where it is rejected.
- **Counts re-derived from the docs**: groups 6/8/15/21/26/20/15/22 = 133;
  47 ADR rejected alternatives; 47 empirical rows E1–E47 contiguous with no
  dangling reference; 19 land reader rules; 13 `failed_at` values; 5 advisory
  codes; 10 evidence states; 23 remediations across the two PRDs.
- Side Research md5: `b385fe622db9926f48861105239f113e` — preserved
  byte-identical.

## Empirical Validation (read-only probes; scratch removed)

git 2.55.0 / macOS, throwaway repos, all deleted. rev-4 added E43–E47.

**E43 — parent-tree syntax.** `git rev-parse C^{tree}^` →
`error: object <tree> is a tree, not a commit`; `git read-tree C^{tree}^` →
`fatal: Not a valid object name`. `C^`, `C^^{tree}`, `C~1`, `C~1^{tree}` all
resolve and `read-tree` accepts them.

**E44 — forward qualification ladder** at candidate parent trees:

| Candidate parent tree | `-C3` | `-C1` | `-C0` |
|---|---|---|---|
| pristine pre-landing parent | OK | OK | OK |
| unrelated edit 4 lines from hunk | OK | OK | OK |
| unrelated edit **2 lines** from hunk | **FAIL** | **OK** | OK |
| unrelated edit **1 line** from hunk | FAIL | **FAIL** | OK |
| 10 lines prepended (pure offset) | OK | OK | OK |
| unrelated edit far away | OK | OK | OK |
| **tree that already has the feature** | FAIL | **FAIL** | **FAIL** |
| feature line changed to something else | FAIL | **FAIL** | **FAIL** |
| feature line deleted | FAIL | **FAIL** | **FAIL** |

**E45 / E46 — normalization.** A cherry-pick applied after five prepended
lines yields `@@ -10 +10 @@ l9` vs `@@ -15 +15 @@ l9`: `-U0` minus `index`
**differs**, and rewriting headers to `@@` makes it **equal**. The trade:
identical `-`/`+` bodies at different positions compare equal — measured with
duplicate `DUP` lines, both normalizing to `@@ / -DUP / +DUP CHANGED`. Distinct
payload, mode-only change and different path all stay distinct; mode-only
normalizes to `diff --git` + `old mode` + `new mode` with no `@@` at all.

**E47 — offline discipline.** With a promisor remote configured and its object
physically removed: default `git cat-file -p <blob>` →
`fatal: '<url>' does not appear to be a git repository` /
`Could not read from remote repository.` (network attempted);
`GIT_NO_LAZY_FETCH=1 git cat-file -p <blob>` →
`fatal: Not a valid object name <sha>` (local, immediate). The variable is
accepted by git 2.55.0 and does not disturb the normal path.

Carried forward: the op-kind × landed matrix; the anchor-C `C3`/`C0`/`(0/0)`
ladder with 0 false greens / 69 absent at 26 false reds / 151 present;
dirty-worktree false red; temp-index read-only guarantees; anchor recovery
after re-land; shallow boundary indistinguishable from a root by `%P`;
sha1/sha256 id lengths; `RecipeProvenance` shipping alongside `preimage_hash`.

## Open Residuals

1. **Q17 (open, non-blocking)** — recipes carrying `preimage_hash` produced
   before the provenance sidecar existed fail closed with
   `recipe-provenance-unavailable`; remediation is `tpatch implement <slug>`.
   A back-fill from `status.apply.base_commit` was rejected as not hash-bound.
2. **The `-C1` false-red boundary** — a parent carrying an unrelated edit one
   line from the hunk does not qualify. Measured and accepted; the only
   alternative is `-C0`, which is unsafe.
3. **The normalization collision** — identical `-`/`+` bodies at different
   positions compare equal, which requires duplicate line content at the
   changed lines. Measured, bounded and documented (Q18 resolved).
4. **`history-incomplete` failure path is specified but not end-to-end
   reproduced.** E47 proves the mechanism (`GIT_NO_LAZY_FETCH=1` converts a
   network attempt into a local error) on a synthetic promisor repo with a
   removed object; a `file://` `--filter=blob:none` clone on git 2.55.0 did
   **not** withhold blobs, so the full partial-clone path could not be
   exercised locally. Wave C must validate AC-L69 against a real filtered
   remote.
5. **`(0/0)` block cost** — 26 false reds per 151 present trees, each with R2.
6. **`active` closure widening** changes verdicts for non-landed features.
7. **Forward-mode V10 changes behaviour** for ops with `preimage_hash` (Q15).
8. **`land` gains a refusal path**; Mode B cannot roll back `record`.
9. **Pre-existing citation drift outside the amended sections** —
   `gitutil.go:680`, `store.go:232`, `types.go:91`, `types.go:192`,
   `labels.go:89`, `labels.go:143`, `dependency_gate.go:79` in §3.1,
   `validation.go:66`. Outside this wave's scope; flagged so a reviewer does
   not mistake them for new drift.
10. **Git floor asserted from docs plus a 2.55.0 run**, not a matrix against
    2.22/2.25. `GIT_NO_LAZY_FETCH` requires git ≥ 2.36, which is above the
    2.25 trailer floor — Wave C should state the effective floor as 2.36.
11. **CHANGELOG entry** intentionally absent; **ROADMAP row and LOG verdict**
    are supervisor-owned.

## Reviewer Focus

Ordered by risk:

1. **Is every git command syntactically valid and offline?** Grep for
   `C^{tree}^` (should appear only where declared invalid) and for any object
   command lacking `GIT_NO_LAZY_FETCH=1`.
2. **Is `-C1` the right level?** Check the E44 table against the claim: it must
   keep the 2-lines-away parent and reject all three materialized/modified
   trees. Try to find a healthy parent it rejects or a materialized tree it
   accepts.
3. **Does the remediation loop actually terminate?** AC-L132 is the fixture;
   construct a history where re-record + re-land still yields R11.
4. **Is the Mode A / Mode B split honest and complete?** Mode B explicitly
   cannot roll back `record`. Confirm no acceptance row still claims "mutating
   nothing" for it, and that AC-LD15 no longer contradicts AC-LD18/AC-LD21.
5. **Is the inventory read-error policy right?** Unrelated-unreadable is a warn
   plus exclusion; target/closure-unreadable is a block. Confirm nothing is
   silently skipped and that `Err`↔`Status` flips are instability.
6. **Is the normalization trade acceptable?** It requires duplicate line
   content at the changed lines. Probe mode-only, binary, rename and
   duplicate-line cases.
7. **Are per-member V10 baselines still independent** after the rev-4 edits?
8. **Is the presence cross-product genuinely exhaustive?** AC-L53 asserts
   3×4 coverage with no unclassified or double-matching combination.
9. **Are all 157 rows executable at their stated tier?**
10. **Citation and count spot-check** against `6d87198`; residual 9 lists the
    known pre-existing drift that is *not* this wave's.

## Blockers

None.

## Context for Next Agent

- Wave B is **planning only**. Wave C owns implementation and dispatches from
  a fresh base.
- The binding contract is `ADR-013` **Amendment 1 rev-4 (D8–D19)** plus
  `PRD-verify-freshness` **§3.6** and **§7.1**. `PRD-tpatch-land`
  §3.8.1–§3.8.5 is a readers' contract; **§3.8.6 is the one producer change**,
  and it is split by invocation mode because Mode B cannot roll back `record`.
- Five corrections to the original diagnosis, all measured: **the defect is not
  V8-only**; **the check set is eleven, not ten**; **the current assertion must
  never read the working tree**; **anchor qualification is a forward question
  at `-C1`**; and **every object command must be offline**.
- `C^` is the parent revision. `C^{tree}^` is invalid.
- **Q15 and Q18 are resolved.** Q17 and the four measured trade-offs are the
  only open items.
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
