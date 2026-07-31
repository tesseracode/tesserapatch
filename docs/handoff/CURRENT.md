# Current Handoff

## Status

Cluster A — **AGENTS.md wave-close checklist amendment** — dual APPROVED and SHIPPED. Ready to dispatch Cluster B (v0.12.1 correctness fix pass — issues #3, #4, #5).

## Active Task

**Cluster A: DONE.** Amendment committed and pushed. Cluster B dispatch imminent — v0.12.1 correctness fix pass bundling GH #3 (multi-slug reconcile cumulative delta corruption), #4 (confirmed human review can't transition rejected-upstreamed → upstream_merged), #5 (record round-trip failure exits 0 + mutates metadata).

## Session Summary

- Wave α (supersession): three-way APPROVED rev-1 at `e5e0091`, consolidated `a05a918`.
- Wave β (write-file safety): three-way APPROVED rev-1 at `63d8650`, user-external fold-in (F1 MEDIUM V0-V9 stale, F2 LOW, F-INT-β-r1-1 LOW) at consolidation `561e6de`.
- Wave γ (active-feature-session): rev-0 dual SPLIT (external BLOCK vs internal APPROVED-WITH-NOTES, zero overlap), supervisor sided external; rev-1 folded 10 findings, dual SPLIT AGAIN (external NEW Critical F-EXT-γ-1 residual on SaveContextSummary ordering); rev-1.5 targeted preflight amendment at `274fbb6`, three-way concurrent APPROVED at `87648a6`, user-external APPROVED with F1 LOW (unpushed backlog).
- v0.12.0 CHANGELOG dated, ROADMAP flipped ✅, Wave γ archived to HISTORY, tagged and pushed.

## Files Changed at Consolidation

- `CHANGELOG.md`: v0.12.0 header dated + Wave γ rev-1.5 amendment subsection.
- `docs/ROADMAP.md`: v0.12.0 status ✅ SHIPPED; Wave γ status ✅ ACCEPTED with rev-1.5 close narrative + commit ranges.
- `docs/handoff/HISTORY.md`: Wave γ archived (18 commits, ~5,600 lines, three-round arc).
- `docs/handoff/CURRENT.md`: reset (this file).

## Test Results

- `gofmt -l .` empty; `go vet ./...` clean; `go build ./cmd/tpatch` OK.
- `go test ./...` 877 top-level PASS + 217 subtests (0 FAIL). Rev-1.5 baseline established.
- Wave α + β non-invalidation: empty diff on 5 guarded files across the wave.
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. **Deferred to next cluster / post-v0.12.0**:
   - AGENTS.md wave-close checklist amendment (Status flip + push discipline). F1 LOW recurring across Streams A+B + Wave α + β + γ.
   - LOW-γr15-N1: `--json --write` D6 refusal plaintext → JSON envelope (Wave δ candidate).
   - ADR-027 F2 (nit): capture-context privacy boundary language refinement.
   - Doctor S3-boundary deferrals (from Wave β).
   - ADR-029 nit deferrals.

2. **Next cluster selection**: Await user direction. Candidate roadmap items — reconcile safety WP-003 middle-pass, new feature per GH issues, or the AGENTS.md hygiene amendment cluster.

## Blockers

None.

## Context for Next Agent

- **v0.12.0 SHIPPED** at HEAD after this consolidation. Do NOT re-open Wave α/β/γ scope.
- **Two-opinion protocol proven load-bearing** — Wave γ produced two real BLOCK-caliber external catches (rev-0 D6 writer-scope, rev-1 SaveContextSummary ordering) where internal reviewers APPROVED. Continue the dual-review protocol for future clusters.
- **Recurring F1 LOW pattern**: handoff Status flip + push discipline. Every wave user-external raised this. Amend AGENTS.md wave-close checklist as first post-v0.12.0 task.
- **20 binding carry-forward rules** unchanged; extension pattern from Wave β rev-1 (detached-worktree pre-fix compile-fail check on new symbols) has been documented as Rule 20 extension in Wave γ rev-1.5 empirical confirmation record.
- **Side Research md5 invariant**: `b385fe622db9926f48861105239f113e`. Verify: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.
- **Commit trailer**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` verbatim + `Copilot-Session: <session-id>` per session.

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
