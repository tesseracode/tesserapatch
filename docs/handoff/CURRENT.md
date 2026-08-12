# Current Handoff

## Status

**Cluster state**: REV-1 DISPATCHED

v0.15.1 Wave B rev-0 is blocked by V10 omission, evidence grammar,
materialization false-green/false-red rules and schema contradictions.
Rev-1 is dispatched.

## Active Task

- **Task ID**: v0.15.1 Wave B / GH #8 contract
- **Description**: Define verify V7/V8 semantics after a feature or hard
  parent has been landed into reachable Git history.
- **Status**: In Progress
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `ad39e4a`
- **Dispatch HEAD**: `13a885c`
- **Target release**: v0.15.1
- **Implementation**: deferred to Wave C

## Files Changed (exact, this wave)

Owned and staged by explicit path — nothing else:

| File | Change |
|---|---|
| `docs/adrs/ADR-013-verify-freshness-overlay.md` | **Amendment 1** appended (D8–D14), plus 16 rejected alternatives, consequences, and a validated reference block. D7 gains an in-place amendment note. One pre-existing drifted anchor (`gitutil.go:680` → `:828`) corrected in the reference list. |
| `docs/prds/PRD-verify-freshness.md` | New **§3.6** (§3.6.1–§3.6.8) landed-feature contract; new **§4.3.6–§4.3.8** JSON shapes; landed rows appended to **§5**; new **Q9–Q13** in §6; new **§7.1** 90-row acceptance matrix + §7.1.1 non-goals; header/Summary/§0.3 updated. |
| `docs/prds/PRD-tpatch-land.md` | New **§3.8** (§3.8.1–§3.8.5) landing-evidence readers' contract; **§4.3** gains the same-verdict-across-`land` invariant; new **§6.2** 9-row land-side matrix; header + Related updated. §6.1/§6.2 ordered numerically. |
| `docs/handoff/CURRENT.md` | This file. |

Not touched: `internal/`, `cmd/`, `assets/`, `tests/`, `SPEC.md`,
`CHANGELOG.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`, GitHub issue #8.
`docs/ROADMAP.md` and `docs/supervisor/LOG.md` are supervisor-owned
(`AGENTS.md` → File Ownership) and are left for the review/close step.

## Decisions Made (all binding, all with recorded alternatives)

ADR-013 **Amendment 1**, decisions **D8–D14**:

1. **D8 — Evidence authority.** A feature is landed iff a commit reachable
   from current HEAD carries a well-formed four-trailer block naming it by
   **exact** slug value, whose `Tpatch-Patch-SHA` / `Tpatch-Recipe-SHA` /
   `Tpatch-Base-Commit` still match the current canonical bytes,
   `apply-recipe.json` digest (or `none` on both sides), and
   `status.apply.base_commit`. `--grep` is a prefilter only;
   `%(trailers:key=…,valueonly)` is authority. Selection is total and
   deterministic across six states: `none` / `exact` /
   `duplicate-equivalent` / `stale` / `ambiguous` / `malformed`.
   **Reverse-apply is never ownership proof.**
2. **D9 — Baseline model.** *HEAD-anchored closure with landed-materialization
   arbitration.* Compared against replay-at-`apply.base_commit` and
   replay-at-landing-parent (`L^`); both rejected with reasons (operator-owned
   / possibly unreachable / undefined for squash landings / per-member so no
   shared baseline / "validates the past" and therefore blind to later
   overlapping drift). GH #2's invariant is preserved and generalised:
   *any check that may mutate the shadow MUST reset to `closureBaselineTree`
   before the next check; V7's result is never an input to V8's tree.*
3. **D10 — Landed target.** V7 switches to **post-state predicates** (no
   execution) covering `write-file` (byte equality), `append-file` (strict
   suffix), `replace-in-file` (op-inverse round trip), `ensure-directory`;
   V8 becomes `git apply --check --reverse` at the same baseline, sound only
   because it is gated on the matching `Tpatch-Patch-SHA`. **The
   idempotent-overwrite shortcut is explicitly forbidden.** Landed-mode
   remediation never routes to `tpatch reconcile`.
4. **D11 — Hard-parent closure.** Ten-row classification table. Landed +
   materialized parents are **skipped** (locally landed ≈ `upstream_merged`);
   landed-but-reverted, stale/ambiguous/malformed, `unapplied` and `rejected`
   parents all **fail-fast** with named reasons; unlanded applied parents are
   still replayed. Governing invariant: **a member is replayed iff its content
   is not already on the baseline.** Landing order is never consulted.
5. **D12 — Drift/evolution.** 19-row matrix. **Exactly one fallback edge:**
   verification falls back to forward semantics **iff** evidence is `none`.
   `stale`, `ambiguous`, `malformed` are terminal failures. **No
   success-shaped fallback** anywhere.
6. **D13 — Diagnostics.** Verify stays read-only; `schema_version` 1.0 → 1.1
   with additive `omitempty` fields (`baseline`, `landing_evidence`,
   `target_mode`, `checks[].mode`); closed `failed_at` vocabulary; 12 exact
   remediation strings. **No new freshness label** — the derivation stays
   mode-agnostic, so sticky `verify-failed` clears on the first passing run.
7. **D14 — Implementability.** One `git log -z --format='…%(trailers:…)'`
   per run; one new generic reader in `internal/gitutil/`; policy stays in
   `internal/workflow/verify.go` per D7. No new store field, artifact, or
   migration. Git floor ≥ 2.25; below it, degrade to evidence `none`
   (a false red, never a false green).

**Land side (PRD-tpatch-land §3.8):** `land`'s behavior is **unchanged**.
No new status metadata (four options compared, all rejected;
`status.landed_commit` rejected on ADR-019 chicken-and-egg + rebase-staleness
+ migration + second-source-of-truth grounds). `status.apply.base_commit`
**retained unchanged**, with a new independent reason: overwriting it would
make every landed feature evidence-`stale` the instant it landed.

**16 rejected alternatives** are recorded in ADR-013 Amendment 1, including
the closest call (#15, ownership-only evidence) and the escape-flag option
(#16).

## Acceptance Matrix

- **`PRD-verify-freshness.md` §7.1 — 90 rows, `AC-L1` … `AC-L90`**, contiguous
  and unique (machine-checked), across 7 groups: A reported defect (5),
  B recipe op kinds (13), C baseline + GH #2 (9), D evidence authority (21),
  E hard-parent closure (15), F drift/evolution (13), G output/persistence/
  read-only (14). Every row carries a tier: **U** unit / **W** workflow
  integration / **C** real CLI (`W+C` means both).
- **`PRD-tpatch-land.md` §6.2 — 9 rows, `AC-LD1` … `AC-LD9`**, all tier C.
  AC-LD9 is the behavior-neutrality guard for `land`.
- **Total: 99 numbered acceptance criteria.**

## Test Results

Planning wave — no code changed, so no build/test delta is claimed.

- Working tree: no tracked source file modified; `go` toolchain not invoked
  for a code change.
- **Citation validation**: 163 `internal/**.go:line` citations across the
  three amended docs machine-checked to resolve to an existing line range,
  then spot-checked by printing each cited line. Five anchors were corrected
  during the pass (`verify.go:296`→`:293`, `:305-309`→`:310-314`,
  `:1093`→`:998`, `:127-137`→`:127-134`, `types.go:343`→`:347`), plus the
  pre-existing `gitutil.go:680`→`:828` drift in ADR-013's reference list.
- **Cross-reference validation**: every `§` reference used by the amendments
  resolves to a heading that exists (verify §3.6.x, §4.3.6–8, §7.1, §7.1.1;
  land §3.8.x, §6.2; ADR D1–D14 all present).
- **Matrix count validation**: 90 unique contiguous `AC-L` ids, 9 `AC-LD` ids.
- Side Research md5: `b385fe622db9926f48861105239f113e` — preserved
  byte-identical.

## Empirical Validation (read-only probes; scratch removed)

Every load-bearing behavioral claim was executed before being written down,
on git 2.55.0 / macOS, in throwaway repos under a scratch directory that has
been deleted. Guarded WIP files were not touched.

**Real-CLI op-kind × landed matrix** (built binary, full
`apply → record → land → verify` cycle):

| Recipe | pre-land | post-land V7 | post-land V8 |
|---|---|---|---|
| `write-file` | ✓/✓ | **✓ false green** | **✗ false red** |
| `replace-in-file` | ✓/✓ | **✗ false red** (`search text not found`) | skipped |
| `append-file` | ✓/✓ | **✓ false green + double-append** | **✗ false red** |

**This widened the diagnosis: GH #8 is not V8-only.** The reporter saw
`V7 ✓ / V8 ✗` because their recipe was `write-file` shaped. A V8-only fix
would have shipped with `replace-in-file` still false-failing.

Also measured: landed hard parent + unlanded applied child passes today only
by accident (the parent's `write-file` replay is a no-op); reverse-apply
succeeds at an unrelated commit with equivalent content; `--grep` matches a
prose commit that merely quotes the trailer line; a prose paragraph after the
trailer block makes Git parse **no** trailers; `cherry-pick`/`rebase` copy
trailers verbatim while rewriting SHA and parent; `git revert` leaves the
landing reachable with the content gone; one `git log -z --format` call
enumerates SHA + all four trailer values.

## Open Residuals

1. **Q12 (open)** — whether `verify --all` should order landed features
   differently. Wave C must keep ordering byte-identical (AC-L88).
2. **Q13 (open)** — `land`-side warning when a `commit-msg` hook appends prose
   after the trailer block. Deferred; verify already classifies the result
   honestly as `malformed`. Tracked in PRD-tpatch-land §3.8.5.
3. **Pre-existing citation drift outside the amended sections.** Spot-check
   found stale anchors in *unamended* parts of `PRD-verify-freshness.md`
   (`gitutil.go:680`, `store.go:232`, `types.go:91`, `types.go:192`,
   `labels.go:89`, `labels.go:143`, `dependency_gate.go:79`,
   `validation.go:66`). Deliberately **not** rewritten — they are outside
   this wave's scope. Flagged so a reviewer does not mistake them for new
   drift; a separate housekeeping pass should re-anchor them.
4. **Git version floor is asserted from docs + a 2.55.0 run**, not from a
   matrix run against 2.22/2.25. Wave C should confirm the exact floor
   empirically before relying on the advisory path (D14).
4b. **Pre-existing `active`-parent inconsistency, found but NOT fixed.**
   `runClosureReplay`'s switch (`internal/workflow/verify.go:1061-1089`)
   handles only `upstream_merged` and `applied`, so an `active` hard parent
   fail-fasts — while `CheckDependencyGate` accepts `applied` **and**
   `active` (`internal/workflow/dependency_gate.go:80`) and
   `postApplyVerifyStates` allows verifying `active` features
   (`internal/workflow/verify.go:127-134`). Discovered during this wave and
   documented in both D11 tables with an explicit scope note. Deliberately
   **not** widened: it changes verdicts for features unrelated to landing and
   needs its own PRD row.
5. **`duplicate-equivalent` byte-equivalence rule** is specified as
   "byte-identical change over its own parent restricted to the canonical
   patch's path set". Wave C must pin the exact diff invocation; the PRD
   deliberately does not over-specify the plumbing.
6. **CHANGELOG entry** for the contract is intentionally absent — CHANGELOG is
   out of this wave's write scope and belongs to the Wave C ship.
7. **`docs/ROADMAP.md` Wave B row and `docs/supervisor/LOG.md` verdict entry**
   are supervisor-owned and remain to be written at review/close.

## Reviewer Focus

Ordered by risk:

1. **Is the fallback edge airtight?** D12 claims exactly one downgrade path
   (`none` → forward mode) and no success-shaped fallback. Try to construct a
   sequence that reaches PASS from `stale`, `ambiguous` or `malformed`
   evidence, or from a materialization failure.
2. **Is the `replace-in-file` op-inverse round trip sound?**
   `Replace(Replace(c,R,S,1),S,R,1) == c` (§3.6.3). Look for a postimage it
   accepts that the op could not have produced, and for a legitimate
   postimage it rejects. This predicate is the core new logic.
3. **Is the D10 accepted bound acceptable?** Non-final ops on a shared path
   are not independently asserted (AC-L16 pins it). Decide whether V8's
   byte-exact reverse check really covers the gap.
4. **Is rejected alternative #15 (ownership-only evidence) correctly
   rejected?** It is the friendlier design and the closest call. The rejection
   rests on the swapped-`post-apply.patch` false-green scenario.
5. **Does D11's fail-fast set over-refuse?** Specifically
   `parent-evidence-integrity` and `parent-landing-drift`: both make a target
   unverifiable because of a parent's state. Confirm neither can be reached in
   an ordinary healthy workflow.
6. **Does the GH #2 invariant survive verbatim?** §3.6.4 restates it as a
   mode-independent rule; AC-L19 requires
   `TestRunVerify_EquivalentRecipeAndPatchBothPass`
   (`internal/workflow/verify_closure_replay_test.go:275`) to stay green
   **unmodified**.
7. **Are the remediation strings actually actionable, and free of
   `reconcile`?** §3.6.6 has 12 exact strings; AC-L82 is the adversarial guard.
8. **Is the matrix executable?** Each of the 90 rows should be placeable at
   its stated tier with the primitives named in §3.6.8, with no new
   dependency and no store schema change.
9. **Citation spot-check.** Sample §3.6 and ADR D8–D14 anchors against the
   source at `13a885c`; residual #3 lists the known pre-existing drift that is
   *not* this wave's.

## Rev-0 Review Adjudication

- Internal: NEEDS REVISION.
- External/original reproducer: NEEDS REVISION.
- Critical/high themes:
  1. V10 is omitted; landed write-file preimage freshness is undefined.
  2. Trailer reader/duplicate cardinality/root/merge/duplicate-equivalent
     authority is incomplete.
  3. Landed write/replace predicates produce false reds and false greens;
     reverse apply is hunk-local, not byte-exact.
  4. Parent `none`, absent recipe/patch and reader errors create replay or
     vacuous-skip holes.
  5. Artifact/status bytes are not snapshotted consistently.
  6. Schema/check count/failed_at/golden compatibility contradict current V10
     and landed diagnostics.
  7. D14 invocation accounting, merge/root semantics, citations and matrix
     rows need total definitions.

## Next Steps

1. Rewrite landed V7/V8/V10 and parent materialization semantics.
2. Make evidence parsing/cardinality/topology/error states total.
3. Add immutable artifact/status snapshots and honest schema compatibility.
4. Rebuild the acceptance matrix and parity audit.
5. Run dual rev-1 planning review.

## Blockers

None.

## Context for Next Agent

- Wave B is **planning only**. Do not implement verify changes here; Wave C
  owns implementation and must dispatch from a fresh base.
- The binding contract is `ADR-013` **Amendment 1 (D8–D14)** plus
  `PRD-verify-freshness` **§3.6** and **§7.1**. `PRD-tpatch-land` **§3.8** is
  a readers' contract, not a `land` behavior change — AC-LD9 exists to keep it
  that way.
- The single most important correction to the original diagnosis: **the defect
  is not V8-only.** `replace-in-file` fails V7 post-land and `append-file`
  passes V7 while corrupting the shadow.
- Reverse-apply success is evidence of *materialization*, never of ownership.
  Both signals are required, always together.
- Verify remains read-only; no status/index/worktree mutation on any path.
- Stage explicit paths only; no parallel writers; do not touch WIP docs listed
  in `.wave-close-allowlist`.

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
