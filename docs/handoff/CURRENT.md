# Current Handoff

## Status

**Cluster state**: REV-2 DISPATCHED

v0.15.1 Wave B rev-1 remains blocked by Anchor-C false results, parent
arbitration and evidence/schema gaps. Rev-2 is dispatched.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V0–V10 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: In Progress (rev-2)
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Dispatch HEAD (rev-1)**: `4fdc18e`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Files Changed (exact, this wave)

Owned and staged by explicit path — nothing else:

| File | Change |
|---|---|
| `docs/adrs/ADR-013-verify-freshness-overlay.md` | **Amendment 1 rev-1** replaces the rev-0 amendment in full: A1.0 context, A1.1 empirical index E1–E22, decisions **D8–D16**, the exhaustively-verified `replace-in-file` predicate, **22 rejected alternatives**, consequences, and a validated reference block. The D7 in-place note is rewritten for the dual-anchor model. |
| `docs/prds/PRD-verify-freshness.md` | **§3.6 rewritten** as §3.6.1–§3.6.9; **§4.3 preamble + §4.3.1/§4.3.4/§4.3.5 golden examples corrected to eleven checks**; **§4.3.6–§4.3.9 rewritten** (LANDED-PASS, LANDED-with-advisories, LANDED-CONTENT-ABSENT, EVIDENCE-INTEGRITY) with closed `failed_at` and advisory vocabularies; **§5 landed rows rebuilt**; **§6 Q12/Q13 resolved, Q14/Q15 added**; **§7.1 rebuilt to 106 rows** + §7.1.1 non-goals; §8 gains five rev-1 risk rows; header/Summary/Related updated. |
| `docs/prds/PRD-tpatch-land.md` | **§3.8 rewritten** (§3.8.1–§3.8.5): 14 normative reader rules incl. cardinality, topology, case-insensitivity, presence-aware digests and reader-error states; §3.8.3 gains a fifth rejected option; §3.8.4 adds the base-commit-vs-parent distinction; §3.8.5 re-dispositioned as non-blocking. **§6.2 rebuilt to 15 rows.** Header/Related updated. |
| `docs/handoff/CURRENT.md` | This file. |

Not touched: `internal/`, `cmd/`, `assets/`, `tests/`, `SPEC.md`,
`CHANGELOG.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`, GitHub issue #8.
ROADMAP and LOG are supervisor-owned (`AGENTS.md` → File Ownership).

## What rev-0 got wrong, and how rev-1 closes it

Both reviewers returned NEEDS REVISION on twelve findings. All twelve are
closed, not just the headline five.

| rev-0 finding | rev-1 closure |
|---|---|
| **1 CRITICAL — V10 absent; 11-check schema undefined** | D8 pins eleven checks V0–V10 and forbids a twelfth. D13 defines landed V10 end to end (anchor, outcomes, ADR-029 D4/D5/D6/D7 consistency, skip reasons, mode, JSON, remediation R11/R12, parent interaction). §4.3.1/4.3.4/4.3.5 goldens corrected to eleven rows. Matrix Group G (AC-L81–L90). |
| **2 HIGH — malformed evidence unclassifiable from parsed-only output; duplicate cardinality unspecified** | D10 mandates a **raw + parsed** reader and exactly-one cardinality for all four keys, with strict lowercase formats. AC-L37/L38/L40/L43. |
| **3 HIGH — duplicate-equivalent undefined for root/merge/incomparable** | D15 gives the exact `git diff` invocation, the strict-parser path set, empty-path-set ⇒ `ambiguous` (no broadening), single-parent requirement and deterministic ordering. AC-L48–L54. |
| **4 HIGH — `landed-content-absent` vocabulary conflict** | §4.3.9 publishes one closed `failed_at` set (10 values) and one closed advisory `code` set (4 values), used identically in PRD and ADR. AC-L96. |
| **5 HIGH — zero-byte patch exactness** | D10 makes digest comparison **presence-aware**: absent ⇒ mismatch, present-zero-byte ⇒ `sha256("")`. Absent ≠ empty at every consumer. AC-L35/L93. |
| **6 HIGH — parent materialization ignored V8 and absent artifacts** | D14 defines **total materialization** as a conjunction over every applicable assertion, with an explicit artifact matrix; both-absent ⇒ `landed-artifacts-absent` FAIL. AC-L65–L68. |
| **7 HIGH — no immutable snapshots** | D14 adds a one-per-run snapshot (status + presence + raw bytes) that every consumer reads, plus end-of-run re-read ⇒ `snapshot-unstable`. AC-L91–L93. |
| **8 HIGH — byte-identical no-evidence output contradicts 1.1** | §4.3.6 states **additive semantic compatibility, not byte identity**; `baseline`/`landing_evidence`/`target_mode` are emitted for every feature. AC-L94. |
| **9 HIGH — write-file byte equality conflicts with ADR-029; reverse-apply is offset-tolerant not byte-exact** | D9 moves recipe judgement to anchor H (replay, not post-state), D11 restates reverse-apply accurately, D13 makes a HEAD-side preimage difference a **warn-class later-touch**, never an automatic block. AC-L82/L85/L86. |
| **10 HIGH — replace-in-file predicate vacuous/false-red** | Replaced with an existential inverse over every occurrence of `R`; **0 false reds, 0 false greens** over 52 416 exhaustively enumerated cases (rev-0: 204 / 15 933). `R==""` undecidable, `S==""` unsupported. AC-L15–L17. |
| **11 HIGH — parent `evidence none` replays already-materialized content** | D12 adds content-materialization arbitration **before** replay for `evidence none`; already-present ⇒ skip with a mandatory `unattributed-materialized` warn and no ownership claim. AC-L64. |
| **12 MEDIUM/HIGH — reader errors, artifact absence, invocation accounting, root/merge, goldens, tiers** | Reader error ⇒ `unavailable` block (never `none`); §5 artifact rows rebuilt with the `land`-refuses-empty-patch context; §3.6.9 publishes an honest six-line invocation budget; root/merge ⇒ `unsupported-topology`; goldens corrected; tiers restated as U / W(`PATH` git wrapper) / C with no production seam. |

## Decisions Made (binding, all with recorded alternatives)

ADR-013 **Amendment 1 rev-1**, decisions **D8–D16**:

1. **D8 — Eleven checks, V0–V10.** No new check ID; the current-materialization
   assertion folds into **V8**, whose subject is the artifact the trailer
   attests. A twelfth check was considered and rejected.
2. **D9 — Baseline: dual-anchor.** *Anchor H* = shadow at `L^`, the selected
   landing commit's **single** parent, running the existing replay machinery;
   *anchor C* = current `HEAD` materialization ladder, read-only, no shadow.
   HEAD-only (rev-0) and `apply.base_commit` replay both rejected with
   reasons. Anchor H unavailable ⇒ explicit skip with a named reason while
   anchor C still runs at block severity.
3. **D10 — Evidence reader.** One `--topo-order` enumeration per run carrying
   `%H`, `%P`, **raw `%B`** and the four parsed trailers, cached across
   `--all`. Exactly-one cardinality per key; exact-value slug match; strict
   lowercase 64/64-or-`none`/40 formats; git's key case-insensitivity stated
   rather than denied; presence-aware patch digest; `none` covers both absent
   and whitespace-only recipes. **Eight** closed evidence states, incl.
   `unsupported-topology` and `unavailable`.
4. **D11 — Anchor-C ladder.** Default-context reverse-check ⇒ clean; on
   failure `-C0` ⇒ pass + `context-drift` **warn**; both fail ⇒
   `landed-content-absent` block. Reverse-apply restated accurately as hunk
   presence modulo context and offset.
5. **D12 — Closure arbitration.** Replay iff content is absent from the
   anchor. Landed+totally-materialized ⇒ skip; `evidence none` + already
   present ⇒ skip + `unattributed-materialized` warn; landed-but-drifted,
   integrity failures, `unapplied`, `rejected` ⇒ named fail-fast.
   **`active` is decided: treated exactly as `applied`**, widening the switch
   so all four call sites agree.
6. **D13 — V10 landed.** Anchor-H preimage evaluation; HEAD-side difference is
   an ADR-029 D5/D6 **warn-class later-touch**, never an automatic block; a
   mismatch at its own baseline still blocks; supersession downgrade
   unchanged; anchor unavailable ⇒ skip, never a live-tree fallback.
7. **D14 — Total materialization + one immutable snapshot per run**, with
   `snapshot-unstable` on concurrent mutation and absent ≠ empty everywhere.
8. **D15 — Topology.** Single-parent required; root/merge ⇒
   `unsupported-topology`, no `^1` guess. Deterministic anchor selection
   (topo-oldest whose parent does not already materialize, then smallest SHA).
   Duplicate-equivalence fully specified; empty path set ⇒ `ambiguous`.
9. **D16 — Honest invocation accounting** (six-line budget), one new
   `internal/gitutil` reader, policy stays in `verify.go`, no store/schema
   change, git floor ≥ 2.25 ⇒ `unavailable` (not `none`).

**Land side (`PRD-tpatch-land` §3.8):** behaviour **unchanged**; 14 normative
reader rules; **no new status metadata** (five options compared, all
rejected, incl. a new `Tpatch-Landed-Parent` trailer);
`status.apply.base_commit` **retained** with the added rationale that
overwriting it would make every landed feature instantly evidence-`stale`,
and with the explicit note that the base trailer and the landing parent are
two independent facts that coincide only in the simple case.

**22 rejected alternatives** are recorded in the ADR.

## Acceptance Matrix

- **`PRD-verify-freshness` §7.1 — 106 rows, `AC-L1` … `AC-L106`**, contiguous
  and unique (machine-checked), across 8 groups: A reported defect + 11-check
  schema (6), B anchor H + op shapes (12), C anchor C ladder (10), D evidence
  reader/grammar (19), E topology/duplicates/rewrites (12), F closure
  arbitration + `active` (21), G V10 (10), H snapshots/schema/diagnostics (16).
- **`PRD-tpatch-land` §6.2 — 15 rows, `AC-LD1` … `AC-LD15`**, all tier C.
- **Total: 121 numbered acceptance criteria** (rev-0: 99).
- **Tiers are executable**: **U** = unit over pure functions or the snapshot
  abstraction; **W** = workflow integration, and where the reader must be
  observed or perturbed, a **`PATH` git wrapper** (test-only shim first on
  `PATH`) — **no production seam, no build tag, no exported hook**; **C** =
  real CLI. AC-L23's rev-0 impossible-seam formulation is gone: concurrent
  mutation is now AC-L92 (U over the snapshot abstraction + W via the
  wrapper).

## Test Results

Planning wave — no code changed, so no build/test delta is claimed.

- Working tree: no tracked source file modified.
- **Citation validation**: **201 `internal/**.go:line` citations, 81 unique**,
  across the four docs machine-checked in range, then every unique anchor
  printed and semantically spot-checked against the source at `4fdc18e`.
  Seven anchors were tightened or corrected during rev-1:
  `writefile_safety.go:108-113`→`:108-112`,
  `recipe_autogen.go:113-118`→`:114-118`, `verify_all.go:88-96`→`:89-97`,
  `verify.go:854-861`→`:853-861`, `verify.go:864-870`→`:862-870`,
  `verify.go:840-846`→`:879-883` (the ADR-029 D4 legacy path is the
  `preimageLegacyWarn` switch arm, not the doc comment), and
  `dependency_gate.go:79-82`→`:79-81`.
- **Cross-reference validation**: every self-`§` reference resolves to an
  existing heading in its own document; ADR `D1`–`D16` all present; all
  `AC-L*` references resolve; all `R1`–`R19` remediation ids defined and
  referenced without dangling.
- **Consistency counts re-derived from the docs, not asserted**: 8 matrix
  groups sized A 6 / B 12 / C 10 / D 19 / E 12 / F 21 / G 10 / H 16 = 106;
  22 ADR rejected alternatives; 22 A1.1 empirical rows (E1–E22); 14 land
  reader rules; 10 `failed_at` values; 4 advisory codes; 8 evidence states.
- **Matrix count validation**: 106 unique contiguous `AC-L`, 15 unique
  contiguous `AC-LD`, every row carrying a tier cell.
- Side Research md5: `b385fe622db9926f48861105239f113e` — preserved
  byte-identical.

## Empirical Validation (read-only probes; scratch removed)

git 2.55.0 / macOS, throwaway repos, all deleted. rev-1 added E10–E22 to the
ADR §A1.1 index.

**V10 reality (new in rev-1).** `RecipeFromPatch` emits `{type,path,content}`
with **no `preimage_hash`**, so every autogenerated recipe takes the ADR-029
D4 legacy path. With a genuine `preimage_hash`, **V10 already FAILS for an
`applied`, un-landed feature** (`expected preimage sha256:5fb14…, observed
sha256:fa6dd8…`), and `preimage_hash: ""` fails with `new-file collision`,
because `checkWriteFilePreimage` reads the **live working tree**. Recorded as
Q15; landed features are fixed by anchoring V10 at `L^`.

**Reverse-apply context ladder (new in rev-1).** 3-hunk patch, 60-line file:

| Scenario | `-C3` | `-C1` | `-C0` |
|---|---|---|---|
| pristine landed tip | OK | OK | OK |
| 10 lines prepended / appended | OK | OK | OK |
| unrelated edit far from a hunk | OK | OK | OK |
| unrelated edit 2 lines from a hunk | **FAIL** | OK | OK |
| unrelated edit 1 line from a hunk | **FAIL** | **FAIL** | OK |
| partial revert / full revert / line re-modified / file deleted | FAIL | FAIL | **FAIL** |

Randomized 400-tree run: `-C0` gave **0 false greens / 216 absent** and
**0 false reds / 184 present**; default `-C3` gave **60 false reds / 184**.
Measured `-C0` hole: revert-in-place plus identical text elsewhere passes;
`--verbose` prints `Context reduced to (0/0)`, which is the Q14 hardening.

**`replace-in-file` predicate (new in rev-1).** Exhaustive enumeration:

| Predicate | decided | undecidable | false reds | false greens |
|---|---|---|---|---|
| rev-0 round trip | 56 784 | 0 | **204** | **15 933** |
| rev-1 existential inverse | 52 416 | 4 368 | **0** | **0** |

**Trailer grammar (new in rev-1).** Duplicate `Tpatch-Patch-SHA` returns
`aaaa,bbbb`; keys are **case-insensitive** (`tpatch-feature:` parses); a root
landing has 0 parents and `rev-parse <root>^` fails; a merge landing has 2 and
its trailer parses normally; a prose paragraph after the block yields an empty
parse while the raw `%B` still holds the line.

Carried from rev-0: reverse-apply succeeds at an unrelated commit with
equivalent content; `--grep` matches a prose commit; `cherry-pick`/`rebase`
copy trailers verbatim; `git revert` leaves the landing reachable with the
content gone; one `git log -z --format` call yields SHA + parents + raw + all
four trailers.

## Open Residuals

1. **Q14 (open, non-blocking, SHOULD)** — whether anchor-C step 2 should parse
   `git apply --verbose` under `LC_ALL=C` to close the measured `-C0` hole.
   Bounded by three independent factors; AC-L28/AC-L47 cover both outcomes.
2. **Q15 (open, out of scope, tracked)** — forward-mode V10 is wrong for
   un-landed applied features with a real `preimage_hash` (measured). No
   anchor exists for an un-landed feature and fixing it would change verdicts
   for features that never landed. Needs its own issue and PRD.
3. **`active` closure widening is a deliberate behaviour change for non-landed
   features.** Decided in D12 rather than left dangling (rev-0's disposition,
   which both reviewers rejected). Pinned by AC-L73/AC-L74 and carried as a
   §8 risk row; reversible by narrowing the switch.
4. **Pre-existing citation drift outside the amended sections** —
   `PRD-verify-freshness` still carries stale anchors in *unamended* prose
   (`gitutil.go:680`, `store.go:232`, `types.go:91`, `types.go:192`,
   `labels.go:89`, `labels.go:143`, `dependency_gate.go:79` in the §3.1 table,
   `validation.go:66`). Deliberately not rewritten — outside this wave's
   scope. Flagged so a reviewer does not mistake them for new drift.
5. **Git floor asserted from docs plus a 2.55.0 run**, not from a matrix
   against 2.22/2.25. Wave C should confirm empirically before relying on the
   `unavailable` advisory path.
6. **CHANGELOG entry** intentionally absent — out of this wave's write scope;
   belongs to the Wave C ship.
7. **`docs/ROADMAP.md` Wave B row and `docs/supervisor/LOG.md` verdict entry**
   are supervisor-owned and remain for review/close.

## Rev-1 Review Adjudication

- Internal: NEEDS REVISION.
- External/original reproducer: NEEDS REVISION.
- Blocking themes:
  1. Anchor C reads the dirty worktree and C0 admits measured false greens.
  2. Historical-anchor unavailable is a success-shaped skip.
  3. Parent arbitration reuses rejected whole-file predicates and is
     circular about replay.
  4. V10 later-touch and parent aggregation do not match ADR-029.
  5. Evidence raw grammar, artifact-absence precedence and invocation
     accounting remain inconsistent.
  6. Re-land remediation can permanently lose historical anchor H.
  7. Golden examples, freshness_label and stale V10 prose still conflict.

## Next Steps

1. Isolate Anchor C at HEAD tree/index and make unsafe C0 reductions block.
2. Separate exact landing attestation from historical replay-anchor selection.
3. Make parent arbitration patch-ladder-based and non-mutating.
4. Align V10 with ADR-029 metadata and total parent materialization.
5. Rebuild evidence grammar, schema, examples and matrix.
6. Run dual rev-2 planning review.

## Reviewer Focus

Ordered by risk:

1. **Is the dual-anchor split correct, or does it re-introduce "validates the
   past"?** Anchor H answers *are the artifacts coherent*, anchor C answers
   *is it still there*. Try to construct a healthy feature that fails, or a
   broken one that passes, because the two questions were separated.
2. **Is the anchor-C ladder's warn/block boundary right?** Step-2 pass is a
   `warn`, and the measured `-C0` hole (revert-in-place + identical text
   elsewhere) lands there. Decide whether that is acceptable given the three
   bounding factors, or whether Q14 must be promoted to MUST.
3. **Is the existential-inverse predicate sound beyond the enumerated
   domain?** 0/0 over 52 416 cases with `{a,b,X}`, |pre| ≤ 7, |c| ≤ 5. Look
   for a shape outside that box — multi-byte runes, overlapping `S`/`R`,
   `len(R) > len(c)`.
4. **Does `active` widening (D12) break anything?** It changes verdicts for
   non-landed features. Confirm the four call sites really do disagree today
   and that widening is the smaller change.
5. **Is total materialization (D14) over-strict?** Both-artifacts-absent now
   FAILs a landed member. Confirm `land` cannot produce that state and that
   the corruption framing in §5 is accurate.
6. **Is the evidence grammar total?** Eight states. Try to construct a commit
   the classifier cannot place, or two commits it places inconsistently.
7. **Is V10's landed contract genuinely ADR-029-consistent?** Compare D13's
   table against ADR-029 D4/D5/D6/D7 line by line, especially "later-touch is
   warning-class" vs "stale preimages on effective features fail".
8. **Does the GH #2 invariant survive?** §3.6.4 restates it mode-independently;
   AC-L101 requires `TestRunVerify_EquivalentRecipeAndPatchBothPass`
   (`internal/workflow/verify_closure_replay_test.go:275`) green **unmodified**
   and AC-L102 pins the reset at anchor H.
9. **Are all 121 rows executable at their stated tier** with only the §3.6.9
   primitives, the `PATH` git wrapper and the snapshot abstraction — no
   production seam?
10. **Citation spot-check** against `4fdc18e`; residual 4 lists the known
    pre-existing drift that is *not* this wave's.

## Blockers

None.

## Context for Next Agent

- Wave B is **planning only**. Wave C owns implementation and dispatches from
  a fresh base.
- The binding contract is `ADR-013` **Amendment 1 rev-1 (D8–D16)** plus
  `PRD-verify-freshness` **§3.6** and **§7.1**. `PRD-tpatch-land` **§3.8** is
  a readers' contract, not a `land` behaviour change — AC-LD15 guards that.
- Two corrections to the original diagnosis, both measured: **the defect is
  not V8-only** (`replace-in-file` fails V7; `append-file` corrupts the
  shadow), and **the check set is eleven, not ten** — rev-0's omission of V10
  was the critical finding.
- Reverse-apply proves **materialization modulo context and offset**, never
  ownership and never byte equality. Trailer evidence and materialization are
  always required together.
- Verify stays read-only; no status/index/worktree mutation on any path.
- Stage explicit paths only; no parallel writers; do not touch the WIP docs in
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
