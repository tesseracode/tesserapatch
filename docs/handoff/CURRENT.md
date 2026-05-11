# Current Handoff

## Active Task

- **Task ID**: `m17-wave-a-parallel-dispatch` — M17 Wave A1 + A2 (v0.8.0 boundary-capture cluster, first wave)
- **Milestone**: M17 — boundary-capture cluster, v0.8.0
- **Status**: Ready to dispatch — both Wave A slices are independent and ship in parallel as separate background implementer sub-agents.
- **Assigned**: 2026-05-11 (immediately after v0.7.0 ship)

## Wave A — Parallel Slices

| Slice | Task ID | PRD | ADR placeholder | Owner | Notes |
|-------|---------|-----|-----------------|-------|-------|
| A1 | `impl-record-auto-base` | [`PRD-record-auto-base.md`](../prds/PRD-record-auto-base.md) | [`ADR-016`](../adrs/ADR-016-record-auto-base-resolution.md) | TBD (fresh sub-agent) | Adds `tpatch record <slug> --auto` with `.tpatch/upstream.lock` + merge-base baseline inference. |
| A2 | `impl-reconcile-lock-guard` | [`PRD-reconcile-lock-guard.md`](../prds/PRD-reconcile-lock-guard.md) | [`ADR-017`](../adrs/ADR-017-reconcile-lock-guard-and-writer-normalization.md) | TBD (fresh sub-agent) | Reconcile-time lock validation guard + HIGH writer-normalization fix at `internal/workflow/reconcile.go:599-604` bundled per PRD §5.3. |

**Coordination point**: Both slices reference a shared `internal/store/upstream_lock.go` parser primitive. **Whichever Wave A slice ships first writes the parser; the second consumes by import.** Both PRDs are explicit about this (`PRD-record-auto-base §5`, `PRD-reconcile-lock-guard §5`). Resolve at review-time, not at dispatch — implementers will independently draft the parser; the reviewer for the second-to-land slice will request a rebase to import the first slice's parser.

## Dispatch Plan

1. Dispatch `m17-wave-a1-impl` background agent with `PRD-record-auto-base.md` as authoritative brief.
2. Dispatch `m17-wave-a2-impl` background agent with `PRD-reconcile-lock-guard.md` as authoritative brief + explicit reminder to bundle the HIGH bug at `reconcile.go:599-604`.
3. Both dispatches in parallel; both must include claims-audit-style verification of their PRD's claims against current code before implementing.
4. On each completion: sub-agent reviewer with layered checklist → external supervisor → push.
5. Parser coordination: first-to-land writes `internal/store/upstream_lock.go`; second-to-land reviewer requests rebase.

## Just Shipped

**v0.7.0 — `feat-amend-dependent-warning`** (archived to HISTORY 2026-05-11): amend-detection guard in `record` (exit 1 by default, `--force-amend` escape hatch), `dependent-broken` overlay across all 4 status surfaces with one coalesced line per affected feature. External supervisor APPROVED after one revision (rev-1 `6e78eac`). Ship stack: `8306367` → `6e78eac` → `a5e7de0` → `c9c8de3`, tag at `6e78eac`.

## M17 — Remaining Waves (after Wave A)

| Slice | PRD | Wave deps |
|-------|-----|-----------|
| B — `impl-record-collision-detection` | PRD-record-collision-detection | Wave A1 (recovery hints need `--auto`) |
| C — `impl-tpatch-land` | PRD-tpatch-land | Wave A1 + Wave A2 + Wave B |
| D — `impl-patch-already-upstream-detector` | PRD-patch-already-upstream-detector | Independent (default-OFF, ships in parallel with any wave) |

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

### Candidate follow-up names

These are research outputs only, not queued roadmap work:

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


## Blockers

None. Ready to dispatch Wave A1 + A2 implementers in parallel.
