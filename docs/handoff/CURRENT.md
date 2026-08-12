# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave B **rev-2** (GH #8 contract) is rewritten and ready for dual
independent review. Planning only — no Go code, tests, assets, SPEC,
CHANGELOG or GitHub issue state was touched. Implementation is Wave C.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V0–V10 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: Review (rev-2)
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Dispatch HEAD (rev-2)**: `f9138e6`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Files Changed (exact, this wave)

Owned and staged by explicit path — nothing else:

| File | Change |
|---|---|
| `docs/adrs/ADR-013-verify-freshness-overlay.md` | **Amendment 1 rev-2** replaces the rev-1 amendment in full: revision history, A1.0 context, A1.1 empirical index **E1–E33** (E23–E33 new, E27 a retraction), decisions **D8–D17**, the `replace-in-file` predicate demoted to diagnostic use, **33 rejected alternatives**, consequences and a validated reference block. The D7 in-place note is rewritten for index-isolated anchor C and ladder-based arbitration. |
| `docs/prds/PRD-verify-freshness.md` | **§3.6 rewritten** as §3.6.1–§3.6.9 (anchor-C isolation, mandatory `(0/0)` block, ladder-only parent arbitration, ADR-029 metadata V10, attestation-vs-anchor separation, 20 exact remediations R1–R20); **§4.3.6–§4.3.9 rewritten** with `baseline.current_probe`, `replay_anchor_commit`, `patch_present`/`recipe_present` and an 11-value `failed_at` set; **`freshness_label` removed from every verify sample** incl. the three pre-existing ones; **§5 landed rows rebuilt**; **§6 Q14 resolved binding, Q15 scoped out, Q16 added**; **§7.1 rebuilt to 118 rows**; §8 risk rows refreshed; header/Summary updated. |
| `docs/prds/PRD-tpatch-land.md` | **§3.8.2 gains rules 15–17** (conservative raw precedence, absence-before-digest ordering, attestation-vs-anchor separation); §3.8.3 gains a sixth rejected option; **§6.2 extended to 17 rows**; header/Related updated to rev-2. |
| `docs/handoff/CURRENT.md` | This file. |

Not touched: `internal/`, `cmd/`, `assets/`, `tests/`, `SPEC.md`,
`CHANGELOG.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`, GitHub issue #8.
ROADMAP and LOG are supervisor-owned (`AGENTS.md` → File Ownership).

## How rev-2 closes every rev-1 finding

| rev-1 finding | rev-2 closure |
|---|---|
| **1 CRITICAL — anchor C uses the working tree, producing dirty-tree false greens/reds** | **D11**: anchor C runs against a **temporary index** seeded by `GIT_INDEX_FILE=<tmp> git read-tree <tree>` + `git apply --check --reverse --cached`. Measured (E23): with the feature reverted in the worktree only, the rev-1 check **FAILS** while the isolated form passes. Measured read-only guarantees (E24): real index byte-identical, worktree byte-identical, `git status` unchanged, temp index invisible under the git dir and removed on every exit path. Rows AC-L7–AC-L13. |
| **2 CRITICAL/HIGH — C0 fallback has measured false greens; Q14 cannot stay optional** | **D12**: Q14 is now **binding**. `C3` pass ⇒ clean; `C0` pass **with zero** `Context reduced to (0/0)` ⇒ warn; **any `(0/0)`, or `C0` fail ⇒ BLOCK**, under a mandatory `LC_ALL=C`. Measured (E28/E29): hardened **0 false greens / 69 absent**, unhardened **2 false greens / 69**; the cost is **26 false reds / 151 present**, each carrying remediation R2. Safety over measured false reds, as directed. Rows AC-L14–AC-L28. |
| **3 HIGH — historical-anchor unavailable skips block checks and can pass on Anchor C alone** | **D14**: `historical-anchor-unavailable` is a **terminal `failed_at`**. V7, V8-historical and V10 report `passed: false` with `mode` present — failed-because-unanchored, never skipped. AC-L35/AC-L36 pin that a clean anchor C still **fails** the run. |
| **4 HIGH — parent arbitration reuses byte equality, refers circularly to replay, mishandles evidence-none/absent artifacts** | **D13**: presence is decided **solely** by the non-mutating hardened patch ladder at the anchor. Recipe predicates are **diagnostics only** and never certify. `evidence none` + patch present ⇒ ladder decides: present ⇒ skip + `unattributed-materialized` warn, absent ⇒ replay. Recipe absent/zero-op ⇒ patch is sole authority; both absent ⇒ terminal `landed-artifacts-absent`. Revert timing qualified per anchor. `active` decided total. Rows AC-L69–AC-L88. |
| **5 HIGH — V10 later-touch does not use ADR-029 metadata; parent V10 aggregation incomplete** | **D15**: later-touch is taken from the **shipped detector** — `RequestedAt` ordering plus the union of `patch-generations.json.touched_paths` and recipe op paths (`writefile_safety.go:380-388`, `:409-442`, `:449-481`, `:489-498`) — never byte differences. Historical preimage at anchor H; malformed `preimage_hash` blocks on its own terms. Parent aggregation defined: block-class ⇒ `parent-landing-drift`, warn-class ⇒ advisory. Rows AC-L89–AC-L98. |
| **6 HIGH — raw trailer grammar/absence precedence and invocation ordering inconsistent** | **D10/D17**: conservative precedence — any raw exact-slug trailer-shaped line absent from the parsed terminal block is **`malformed`, never `none`** (E31; prose false-red accepted deliberately). Absence precedes digest mismatch. Exactly-one cardinality per key; `unavailable` distinct. One `git log --topo-order --reverse -z` per run, cached across `--all`; **`rev-list` never used** (E32); honest per-call budget incl. `read-tree`/`apply`/`diff`/ancestor. Rows AC-L41–AC-L68. |
| **7 MEDIUM/HIGH — re-land remediation disqualifies the new candidate and can permanently degrade Anchor H** | **D14**: the **attestation candidate** (authority, must match current artifacts) and the **replay-anchor candidate** (baseline source, hashes may be stale) are separate objects. Measured (E30): after a re-land the newest landing's parent already ladder-passes and is disqualified, while the earlier landing's parent qualifies. AC-L30/AC-L37 pin both the separation and the recovery. |
| **8 MEDIUM — tree probe plumbing, freshness_label, stale V10 prose, matrix tiers** | Tree probing specified as `read-tree <tree>` + `apply --cached` with no worktree (E25, AC-L31); **`freshness_label` removed from every verify sample** and Q16 records why; V10 prose rebuilt from source; tiers restated as U (pure/snapshot/reader abstraction), W (`PATH` git wrapper, proven feasible in probe H4), C (real CLI) — no production seam anywhere. |

## Decisions Made (binding, all with recorded alternatives)

ADR-013 **Amendment 1 rev-2**, decisions **D8–D17**:

1. **D8 — Eleven checks V0–V10**; no new check ID; current materialization
   folds into V8.
2. **D9 — Dual anchor.** Anchor H = shadow at the **replay anchor's** single
   parent, running the existing machinery; anchor C = index-isolated
   assertion at `HEAD`. V7's independent obligation stated explicitly.
3. **D10 — Evidence reader.** One `--topo-order --reverse -z` enumeration with
   raw `%B` + `%P` + four parsed trailers, cached across `--all`;
   **conservative raw precedence**; exactly-one cardinality; strict lowercase
   formats; **absence before digest**; eight closed states.
4. **D11 — Anchor C is index-isolated.** Temp index under the git dir (or
   gitignored `.tpatch/local/`), `read-tree` + `apply --cached`, removed on
   every exit path; measured read-only guarantees; results memoised per
   `(tree, patch)`.
5. **D12 — The hardened ladder.** `(0/0)` blocks, `LC_ALL=C` mandatory, with
   the full measured basis and the explicit rejection of an unproven
   hunk-local corroboration.
6. **D13 — Parent arbitration is non-mutating and patch-ladder-only.** Recipe
   predicates are diagnostics; `active` total; revert timing qualified.
7. **D14 — Attestation ≠ replay anchor.** Four candidate conditions,
   deterministic oldest-first selection, no broadening, ambiguity handled,
   **unavailability terminal**, re-land recovery pinned.
8. **D15 — V10 via the shipped ADR-029 detector**, historical preimage at
   anchor H, warn-class later-touch, block-class malformed-hash exception,
   parent aggregation defined.
9. **D16 — Topology.** Single-parent required; root/merge ⇒
   `unsupported-topology`; no `^1`.
10. **D17 — Snapshots + honest invocation budget.** One immutable snapshot
    consumed as copies; `snapshot-unstable` on mutation; seven-line call
    budget; no `rev-list`; git floor ⇒ `unavailable`.

**Land side (`PRD-tpatch-land` §3.8):** behaviour **unchanged**; **17**
normative reader rules (15–17 new); **no new status metadata** (six options
compared, all rejected); `status.apply.base_commit` retained.

**33 rejected alternatives** are recorded in the ADR.

## Acceptance Matrix

- **`PRD-verify-freshness` §7.1 — 118 rows, `AC-L1` … `AC-L118`**, contiguous
  and unique (machine-checked), across 8 groups: A defect + 11-check schema
  (6), B anchor-C isolation (7), C hardened ladder (15), D historical anchor
  (12), E evidence reader/enumeration (28), F closure arbitration (20),
  G V10 (10), H snapshots/schema/diagnostics (20).
- **`PRD-tpatch-land` §6.2 — 17 rows, `AC-LD1` … `AC-LD17`**, all tier C.
- **Total: 135 numbered acceptance criteria** (rev-1: 121; rev-0: 99).
- **Tiers are feasible**: **U** = pure functions plus the **snapshot** and
  **evidence-reader** abstractions; **W** = workflow integration with a
  **`PATH` git wrapper** for counting, injecting errors and mutating files
  between calls — demonstrated working in probe H4; **C** = real CLI. **No
  production seam, no build tag, no exported hook** anywhere.

## Test Results

Planning wave — no code changed, so no build/test delta is claimed.

- Working tree: no tracked source file modified.
- **Citation validation**: **208 `internal/**.go:line` citations, 87 unique**,
  machine-checked in range, then every unique anchor printed and semantically
  spot-checked against the source at `f9138e6`. Five anchors were corrected
  during rev-2: `writefile_safety.go:388-407`→`:380-388`,
  `:409-448`→`:409-442`, `:449-470`→`:449-481`, `:489-499`→`:489-498`, plus
  two markdown headings that had concatenated onto a preceding `---` rule.
- **Cross-reference validation**: every self-`§` reference resolves in its own
  document; ADR `D1`–`D17` all present; all `AC-L*` references resolve; all
  `R1`–`R20` remediation ids defined with no dangling reference.
- **Matrix count validation**: 118 unique contiguous `AC-L` with a tier cell
  on every row; 17 unique contiguous `AC-LD`.
- **Schema hygiene**: `"freshness_label"` occurrences in verify JSON samples =
  **0** (Q16); every `rev-list` mention is a negation.
- **Counts re-derived from the docs**: 8 matrix groups sized 6/7/15/12/28/20/
   10/20 = 118; 33 ADR rejected alternatives; 33 A1.1 empirical rows (E1–E33,
  contiguous, no dangling reference); 17 land reader rules; 11 `failed_at`
  values; 4 advisory codes present in **both** PRD and ADR; 8 evidence states
  present in both; 20 remediation strings.
- Side Research md5: `b385fe622db9926f48861105239f113e` — preserved
  byte-identical.

## Empirical Validation (read-only probes; scratch removed)

git 2.55.0 / macOS, throwaway repos, all deleted. rev-2 added E23–E33.

**Anchor-C isolation (E23/E24/E25).** With the feature reverted in the
worktree only, `git apply --check --reverse` **FAILS** (false red) while
`GIT_INDEX_FILE=<tmp> git read-tree HEAD` + `--cached` **passes**. A temp index
under the git dir leaves the real index and worktree byte-identical, is
invisible to `git status`, and `read-tree <arbitrary-tree>` + `--cached`
probes any tree with zero worktree mutation.

**Hardened ladder (E26), `C3` / `C0` / `(0/0)`-count:**

| Scenario | C3 | C0 | (0/0) | rule |
|---|---|---|---|---|
| pristine landed tip | OK | OK | 0 | clean |
| landing parent (feature absent) | FAIL | FAIL | 0 | block |
| offset shift; unrelated edit far away | OK | OK | 0 | clean |
| unrelated edit 2 lines from a hunk | FAIL | OK | 0 | warn |
| unrelated edit 1 line from a hunk | FAIL | OK | **1** | **block** |
| partial revert — hunk 1 / hunk 2 / hunk 3 / hunks 1+3 | FAIL | FAIL | 0 | block |
| full revert | FAIL | FAIL | 0 | block |
| degenerate whole-file hunk + header/footer | FAIL | OK | 0 | warn |
| **revert-in-place + identical text pasted at EOF** | FAIL | OK | **1** | **block** ✔ |
| patched file deleted | FAIL | FAIL | 0 | block |

**Randomized corpus (E28/E29)**, 220 trees, 3-hunk patch, 80-line file:

| Rule | false greens (69 absent) | false reds (151 present) |
|---|---|---|
| rev-1, `(0/0)` ignored | **2** | 0 |
| rev-2, any `(0/0)` blocks | **0** | 26 |

**Anchor recovery (E30).** After re-record + re-land, the newest landing's
parent **already materializes** the patch (`C3=OK`) and is disqualified as an
anchor; the earlier landing's parent does not (`C3=FAIL`) and qualifies —
even though its own hashes are stale.

**Enumeration and topology (E32/E33).** One
`git log --topo-order --reverse -z --format=…` yields SHA, parent count, all
four trailers and raw `%B`, oldest-first; `rev-list` cannot emit `%B` and is
not needed. A root landing has 0 parents and `read-tree <root>^` fails
outright; a merge landing has 2 and its trailer parses normally.

**Retraction (E27).** rev-1's "fails at every level" sentence is withdrawn;
only the per-scenario triples above are claimed.

Carried from earlier revisions: the op-kind × landed matrix; V10 already fails
for an applied un-landed feature with a real `preimage_hash`; autogen omits
`preimage_hash`; reverse-apply succeeds at an unrelated commit; `--grep`
matches a prose commit; trailers survive cherry-pick/rebase; `git revert`
leaves the landing reachable with content gone; the existential-inverse
predicate scores 0/0 over 52 416 exhaustive cases.

## Open Residuals

1. **Q15 (scoped out, justified, tracked)** — forward-mode V10 is wrong for
   un-landed applied features with a real `preimage_hash`. An un-landed
   feature has **no anchor**, so this contract structurally cannot fix it;
   doing so needs a new artifact or a forward-mode policy change, each with
   its own issue and PRD. No row of this amendment depends on it.
2. **The mandatory `(0/0)` block costs 26 false reds per 151 present trees.**
   Deliberate and quantified; each carries remediation R2. If Wave C finds a
   *provable* hunk-local corroboration it may narrow the rule, but only with
   the same measured evidence.
3. **`active` closure widening is a deliberate behaviour change for non-landed
   features.** Pinned by AC-L82/AC-L83, carried as a §8 risk row, reversible
   by narrowing the switch.
4. **Pre-existing citation drift outside the amended sections** —
   `PRD-verify-freshness` still carries stale anchors in *unamended* prose
   (`gitutil.go:680`, `store.go:232`, `types.go:91`, `types.go:192`,
   `labels.go:89`, `labels.go:143`, `dependency_gate.go:79` in the §3.1 table,
   `validation.go:66`). Outside this wave's scope; flagged so a reviewer does
   not mistake them for new drift.
5. **Git floor asserted from docs plus a 2.55.0 run**, not a matrix against
   2.22/2.25. Wave C should confirm before relying on the `unavailable` path.
6. **CHANGELOG entry** intentionally absent — belongs to the Wave C ship.
7. **`docs/ROADMAP.md` Wave B row and `docs/supervisor/LOG.md` verdict entry**
   are supervisor-owned and remain for review/close.

## Reviewer Focus

Ordered by risk:

1. **Is anchor C now genuinely isolated?** Every ladder call must carry
   `--cached` against a `read-tree`-seeded temp index. Try to find a path
   where the worktree or the real index can still influence a verdict, or
   where the temp index survives a failure.
2. **Is the `(0/0)` block rule right, and is the cost honestly stated?** 26
   false reds per 151 present trees is real. Confirm the remediation (R2) is
   genuinely actionable, and that no scenario in §3.6.5 is mis-tabulated.
3. **Is the attestation/anchor separation airtight?** Construct a history
   where the anchor selection loops, picks a wrong baseline, or fails to
   recover after a re-land.
4. **Is `historical-anchor-unavailable` too aggressive?** It fails features
   whose content is demonstrably present at HEAD. Decide whether the honesty
   is worth the refusal, and whether R11's fix always works.
5. **Is parent arbitration free of circularity?** The presence test must be
   the patch ladder only. Check that no recipe predicate can cause a skip and
   that a diagnostic can never certify.
6. **Does V10 match ADR-029 exactly?** Compare §3.6.7 against ADR-029
   D1/D4/D5/D6/D7 line by line, especially the malformed-hash exception to
   the warn-class later-touch rule, and the shipped detector's `RequestedAt`
   semantics.
7. **Is the conservative raw-precedence rule acceptable?** It deliberately
   false-reds a docs commit that quotes a trailer line. Confirm the direction
   is right and that AC-L52 asserts it as intended.
8. **Does the GH #2 invariant survive?** AC-L111 requires
   `TestRunVerify_EquivalentRecipeAndPatchBothPass`
   (`internal/workflow/verify_closure_replay_test.go:275`) green **unmodified**
   and AC-L112 pins the reset at anchor H.
9. **Are all 135 rows executable at their stated tier** with only the §3.6.9
   primitives, the `PATH` git wrapper and the snapshot/reader abstractions?
10. **Citation and count spot-check** against `f9138e6`; residual 4 lists the
    known pre-existing drift that is *not* this wave's.

## Blockers

None.

## Context for Next Agent

- Wave B is **planning only**. Wave C owns implementation and dispatches from
  a fresh base.
- The binding contract is `ADR-013` **Amendment 1 rev-2 (D8–D17)** plus
  `PRD-verify-freshness` **§3.6** and **§7.1**. `PRD-tpatch-land` **§3.8** is
  a readers' contract — AC-LD15 guards that `land` is unchanged.
- Three corrections to the original diagnosis, all measured: **the defect is
  not V8-only**; **the check set is eleven, not ten**; and **the current
  assertion must never read the working tree**.
- Reverse-apply proves **materialization modulo context and offset**, never
  ownership and never byte equality — and a match that survives only with all
  context discarded is **not** a match this contract will certify.
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
