# Current Handoff

## Status

**Cluster state**: REV-0 DISPATCHED

**WAVE_BASE**: `1bc2a25` (Cluster E process housekeeping dispatch, 2026-08-04).

**2026-08-04 Cluster E DISPATCHED (process housekeeping).** Two findings from external's post-Cluster-D review folded into a small process cluster before v0.13.0 GH #6 work. Scope: F1 MEDIUM (`make wave-close-check` never runs `go test` — gate PASSes with red suite empirically demonstrated at Cluster D HEAD) + F2 LOW (`TestLand_Success_OneCommit_FourTrailers` teardown race on macOS: `unlinkat .git/info: directory not empty`; passes in isolation, fails under full-suite load). Single implementer, sequential. Mirrors Cluster C process-first discipline before feature cluster.

**2026-08-03 Cluster D SHIPPED.** Correctness housekeeping — 8 items total, single implementer, sequential. Four review revs (rev-0 → rev-3). Two-opinion protocol scoreboard: rev-0 dual (internal NEEDS REVISION 3 MEDIUM + 1 LOW, external APPROVED WITH NOTES 1 MEDIUM overlap), rev-1 dual (internal NEEDS REVISION 1 MEDIUM residual, external APPROVED), rev-2 external-only (NEEDS REVISION 1 MEDIUM new Rule 17 residual), rev-3 external-only (APPROVED via prescriptive verbatim wording). **Notable pattern**: three consecutive iterations on the same fast-path help clause each introduced a new Rule 17 residual; broken by supervisor-prescribed verbatim text at rev-3. All 6 backlog items + 2 review-fold items landed. Deferred: D-INT-2 (--from-revision post-crash out of PRD-#4 F-4 scope), F-EXT-2 (concurrency out of local-CLI scope). Range: `4868f68..42f85d7` (13 commits: 8 rev-0 impl + 3 rev-1 folds + 1 rev-2 fold + 1 rev-3 fold, plus 4 tracking commits).

**2026-08-03 Cluster D DISPATCHED.** Correctness housekeeping — single implementer, sequential, small-scope items. Scope: 6 backlog items (PRD-#3 N2/N3/S1, PRD-#4 F-4, GH #5 docs, Wave γ LOW-γr15-N1) + 2 review-fold items from external's post-Cluster-C review (F1 MEDIUM: gate glob gap; F2 LOW carry-over: LOG SHA pointer).

**v0.12.1 SHIPPED 2026-07-31.** Correctness fix pass (GH #3 + #4 + #5) three-way APPROVED at rev-1 across all three tickets. Cluster A (AGENTS.md wave-close checklist) shipped earlier same day. All work pushed to `origin/main`. v0.12.1 tag pushed.

**2026-08-02 CI back green on `main`.** Inline hygiene fix at `4619b55` — `gitInitTestRepo` pinned to `-b main` — resolved a persistent CI failure class (red since 2026-07-28).

**2026-08-02 Cluster C SHIPPED.** Process housekeeping — parallel-implementer discipline addendum + `make wave-close-check` mechanical gate. Four review revs total (rev-0 → rev-4). Two-opinion protocol scoreboard: **external-only catches** on rev-0 (BLOCKING unpushed), rev-1 (3 HIGH + 2 MEDIUM incl. empirical false-passes), rev-2 (1 HIGH duplicate-field), rev-3 (1 BLOCKING shell-bug + 1 non-blocking). Internal APPROVED at rev-1 and rev-2; rev-3 and rev-4 were external-only cycles for single-issue empirical fixes. Cluster C's own gate now dogfoods on every commit going forward. Range: `bb31872..870182d`.

## Active Task

**Cluster E — Process housekeeping (F1 gate coverage + F2 flake fix).** Single implementer, sequential. WAVE_BASE `1bc2a25`. Dispatched 2026-08-04.

### Scope (2 items from external post-Cluster-D review)

1. **F1 MEDIUM — `make wave-close-check` never runs the test suite.**
   - Empirical evidence: at Cluster D HEAD `1bc2a25`, `make wave-close-check` reports PASS while `go test -count=1 ./...` exits 1 (F2 flake). Gate correctness blind spot.
   - Fix: add an 8th mechanical check to `Makefile` `wave-close-check` target that runs `go test ./...` (fail-fast). Update AGENTS.md Wave-Close Checklist to reflect the new mechanical check and remove any implicit reliance on manual test runs.
   - Do NOT count subtests differently or use `-short` unless it's demonstrably needed for gate practicality — the whole point is to catch what full-suite runs catch.

2. **F2 LOW — `TestLand_Success_OneCommit_FourTrailers` cleanup race on macOS.**
   - Symptom: `unlinkat <TempDir>/001/.git/info: directory not empty` at teardown, under full-suite load only.
   - Location: `internal/cli/land_test.go:123` (`TestLand_Success_OneCommit_FourTrailers`).
   - Investigation required before fix: reproduce the flake (`go test -count=20 ./internal/cli/`, or `-parallel` variants), identify what leaves `.git/info` in a non-empty state at teardown (likely `git gc`/index-lock cleanup or a subprocess holding a file open). Fix at the test level (e.g., explicit `t.Cleanup` that drains git subprocesses; `git gc --prune=now` before teardown; sync-wait for git subprocess exit).
   - Do NOT paper over with `t.Skip` on macOS — the point is to make the gate signal reliable.

### Constraints (per AGENTS.md)

- Explicit `git add <path>` per commit; NEVER `-a`/`-A`/`<dir>/`.
- `git commit -F /tmp/msg.txt` with Copilot + Copilot-Session trailers; never inline heredoc.
- Side Research md5 `b385fe622db9926f48861105239f113e` MUST remain preserved on any CURRENT.md edit.
- Do NOT touch canonical `**Cluster state**` field — supervisor flips at wave close.
- Do NOT stage the 16 untracked WIP files under `docs/whitepapers/`, `docs/prds/`, `docs/state-of-the-art/case-studies/`.

### Non-goals

- Do NOT extend `wave-close-check` beyond adding the test step (e.g., no coverage checks, no lint additions).
- Do NOT touch v0.13.0 GH #6 scope — that's Cluster F.
- Do NOT refactor `TestLand_*` beyond what's needed to close the race.

## Session Summary

- **v0.12.0** (three-wave feature cluster: supersession + write-file safety + active-feature-session) — shipped, tagged `v0.12.0`.
- **Cluster A** (AGENTS.md wave-close checklist codifying F1 pattern) — shipped at `5ac458d`.
- **Cluster B planning** (PRDs #3 + #4 with dual-review parallel) — shipped at `4e673a8`.
- **v0.12.1 implementation** (GH #3 + #4 + #5 correctness fix pass) — shipped at `bb31872`, tagged `v0.12.1`.
- **CI hygiene fix** — `4619b55` pinned `gitInitTestRepo` to `-b main`; CI back green 2026-08-02.
- **Cluster C** (parallel-implementer discipline + `make wave-close-check` mechanical gate) — shipped at `4868f68` after 4 review revs.
- **Cluster D** (correctness housekeeping — 6 backlog items + 2 review-fold items) — shipped 2026-08-03 after 4 review revs. Range `4868f68..42f85d7`.

## Files Changed at v0.12.1 Consolidation

- `CHANGELOG.md`: v0.12.1 header dated 2026-07-31; GH #4 review-path subsection added; rev-1 fold-in subsection appended.
- `docs/ROADMAP.md`: v0.12.1 ✅ SHIPPED section added above v0.12.0.
- `docs/prds/PRD-confirm-upstreamed-human-review-path.md`: Status `Proposed` → `Accepted`.
- `docs/handoff/HISTORY.md`: v0.12.1 archived under 2026-07-31 header.
- `docs/handoff/CURRENT.md`: reset (this file).

## Test Results

- `gofmt -l .` empty; `go vet ./...` clean; `go build ./cmd/tpatch` OK.
- `go test ./...` 907 top-level PASS + subtests (0 FAIL).
- Wave α + β + γ non-invalidation: empty diff on 5 guarded files at v0.12.1 consolidation.
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.

## Next Steps

**Backlog after Cluster D**:

Feature / release:
- **Cluster E — v0.13.0 GH #6 first-class rejected feature state** — data-model extension, PRD + ADR pair. Larger planning-first cluster. Only remaining open GH issue.

Deferred from Cluster D adjudication (documented, no fold):
- **D-INT-2** (`--from-revision <original>` post-crash "superseded" error) — PRD-#4 lines 180/259 document the flag as CI/test override, not the crash-recovery path. Default retry works (external Rule 20 verified). Backlog if operator friction surfaces.
- **F-EXT-2** (concurrency safety of confirm-upstreamed) — pre-existing; concurrent invocation of same slug not a supported local-CLI scenario.

Untracked WIP (surfaced by Cluster D Item 7 gate glob extension; NOT staged by Cluster D):
- `docs/whitepapers/WP-004..WP-007.md` + `.turns.md` siblings (8 files).
- `docs/prds/PRD-feature-unapply.md`, `docs/prds/PRD-recurring-patches.md`.
- `docs/state-of-the-art/*case-study*` (2 files).
- These require operator decision on disposition; not a defect.

Process / hygiene (all shipped this session):
- ✅ AGENTS.md parallel-implementer discipline addendum (Cluster C).
- ✅ Mechanical wave-close-check gate (Cluster C).
- ✅ Gate glob covers whitepapers + state-of-the-art (Cluster D fold F1).

Documentation:
- **ADR-027 F2** (nit).
- **Doctor S3-boundary deferrals** (from Wave β).
- **ADR-029 nit deferrals**.

## Blockers

None.

## Context for Next Agent

- **v0.12.1 SHIPPED** — do NOT re-open Wave α/β/γ or GH #3/#4/#5 scope. All accepted.
- **Two-opinion protocol proven load-bearing again** — v0.12.1 rev-0 external caught 4 findings internal missed (PRD-#4 warning wording, PRD-#4 tie-break correctness bug, PRD-#3 err-branch gap, GH #5 hint mislabel). Internal caught PRD-#3 F-INT-3-1 HIGH (Rule 18 trailer parse failure). Continue dual-review protocol on all clusters ≥ paper-only.
- **Cross-implementer entanglement is now a KNOWN failure mode** — do NOT dispatch parallel implementers to shared source files without briefing them on `git add <path>` discipline. See Cluster A follow-up in backlog.
- **20 binding carry-forward rules** unchanged. Rule 18 empirical demonstration this cluster: heredoc-authored commit bodies leaked `EOF)` after the trailer, breaking `%(trailers)` parse. Rule 20 empirical demonstration: PRD-#4 external caught the tie-break bug via code path enumeration (in-place dedup) that internal's tests-pass verdict didn't surface. Rule 20 continues to require empirical repro even on paper-approved designs.
- **Side Research md5 invariant**: `b385fe622db9926f48861105239f113e`. Verify: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.
- **Commit trailer**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` verbatim + `Copilot-Session: <session-id>` per session. Use `git commit -F <tempfile>` or `git commit -m ""` — NOT `git commit -F -` with heredoc (heredoc close tokens leak into the body).

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
