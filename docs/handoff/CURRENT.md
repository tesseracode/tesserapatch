# Current Handoff

## Active Task

- **Task ID**: `v0.9.0-alpha-2-capture-modes`
- **Milestone**: v0.9.0 Wave alpha — capture & metadata foundation (slice 2 of 2)
- **Description**: Implement `PRD-record-capture-modes` v1 on `tpatch record` — add explicit `--all`, `--staged`, `--unstaged`, and `--claimed-only` flags with mutex matrix, mode-aware untracked-file policy, refuse-on-overlap diagnostics, and capture-mode provenance written to `record.md`. Default `record` behavior preserved verbatim.
- **Status**: Kickoff — awaiting implementer.
- **Assigned**: 2026-05-14.

## Session Summary

Wave alpha slice 1 (`PRD-feature-file-claims` v1) shipped 2026-05-14. External supervisor APPROVED rev-1 fix at `9d7435b` + verdict at `788438b`. Pure CLI surface add: `tpatch feature claim <add|list|remove|clear>` with deterministic `.tpatch/features/<slug>/claims.json` manifest, atomic writes, claim_id-derived stable sort, advisory mode only. Rev-1 closed the F1 path-normalization gap by factoring `NormalizeClaimPathShape` out of `NormalizeClaimPath` so the matcher can soft-normalize remove operands without re-running the safety/reservation rejects from add-time.

Wave alpha slice 2 (THIS) = `PRD-record-capture-modes` v1. The PRD is fully drafted at `docs/prds/PRD-record-capture-modes.md` and the contract is binding. This slice is the second pure CLI surface add — no ADR required (default behavior preserved, all new behavior gated on explicit flags). After this lands and approves, the two slices ship together under the v0.9.0 tag.

## Settled Design Choices

- **No ADR for this slice**: same rationale as alpha-1 — defaults preserved, new behavior is fully opt-in via flags. ADR-024 (capture-context-privacy-boundary) remains deferred until a future PRD needs structured context capture or `--reason`-style fields.
- **Provenance scope for v1**: write capture-mode provenance into the human-readable `record.md` only (capture mode, pathspecs, claim_ids, base_commit, upper_commit, dirty_state summary). Machine-readable persisted metadata is explicitly the domain of `PRD-feature-patch-identity-metadata` (the third PRD in the cluster, not in scope here).
- **Validation strategy for `--staged`**: prefer temporary-index validation seeded from `HEAD`; fall back to `git apply --cached --check` if the temp-index path proves impractical. Do NOT silently downgrade to live-working-tree validation.
- **`--claimed-only` is a filter, not a mode**: it can combine with any capture mode (worktree-all, staged, unstaged, auto, from, commit-range). It refuses if no claims exist.

## v1 Contract (binding for implementer)

The contract is `docs/prds/PRD-record-capture-modes.md` in its entirety. Highlights the implementer must honor exactly:

### Commands and flag matrix

```
tpatch record <slug>                              # current default (unchanged) → working-tree-all
tpatch record <slug> --all                        # explicit alias for default  → working-tree-all
tpatch record <slug> --staged                     # index-only                  → staged-index
tpatch record <slug> --unstaged                   # unstaged worktree           → unstaged-worktree
tpatch record <slug> --auto                       # existing                    → auto-committed-range
tpatch record <slug> --from <base>                # existing                    → committed-range
tpatch record <slug> --commit-range <a>..<b>      # existing                    → explicit-committed-range
```

Layered filters (combinable with any mode unless noted):

```
--files <pathspecs>     # existing pathspec filter
--claimed-only          # intersect capture with active claims; refuse if no claims
```

### Mutex matrix (PRD §3.7)

`--all`, `--staged`, `--unstaged`, `--auto`, `--from`, `--commit-range` are pairwise mutually exclusive among the new+existing capture modes. `--to` requires `--from` or `--auto`. `--files` and `--claimed-only` combine with any mode. All mutex violations must error out *before* patch capture with a clear message.

### Untracked file policy (PRD §3.2)

| Mode | Untracked files |
|---|---|
| default / `--all` | Included (current working-tree capture behavior; index state must be preserved during the dance) |
| `--staged` | Only when represented in the index (i.e., already `git add`-ed) |
| `--unstaged` | Included when not represented in the index and passing filters |
| `--auto`, `--from`, `--commit-range` | Excluded (committed-range reads commits, not worktree) |

### `--staged` rules (PRD §3.3)

- Generate patch from `HEAD` to the index.
- Include staged additions/modifications/deletions/renames to the extent existing machinery supports them.
- Include new files only when represented in the index.
- **Refuse** if any path included in the staged patch also has unstaged edits (ambiguous reverse-apply).
- **Warn but do not refuse** if unrelated unstaged edits exist on other paths.
- **Refuse** when the staged patch is empty.
- Validate with temp-index-seeded-from-HEAD if feasible; else `git apply --cached --check`. Never downgrade silently to live-tree validation.

### `--unstaged` rules (PRD §3.4)

- **Refuse** if any path has both staged and unstaged changes.
- **Warn (note line)** if unrelated staged paths exist; do not capture them.
- Include plain untracked files when not in index and pass filters.
- **Refuse** when the unstaged patch is empty.

### `--claimed-only` rules (PRD §3.5)

- **Refuse** if feature has no active claims.
- With `--files`: intersect explicit pathspecs with claims.
- If intersection is empty: refuse with diagnostic listing both the claims and the explicit paths.
- Reads from the alpha-1 `.tpatch/features/<slug>/claims.json` manifest. Strict-mode claims are reserved by the alpha-1 PRD; all current v1 claims are advisory and eligible.

### Capture-mode provenance (PRD §4)

Every successful record writes provenance into `record.md` (the per-feature human-readable record file):

- `capture_mode`: one of `working-tree-all`, `staged-index`, `unstaged-worktree`, `auto-committed-range`, `committed-range`, `explicit-committed-range`
- `pathspecs`: normalized explicit `--files` if any
- `claim_ids`: active claim IDs used by `--claimed-only` if any (NOT all claims, just the ones that were active during capture)
- `base_commit`: lower bound for the canonical patch
- `upper_commit`: `HEAD`, resolved `--to`, or the literal string `working-tree`
- `dirty_state`: summary string for staged/unstaged modes (NOT raw diff content; one-line summary like "3 staged paths, 1 unstaged note")

Provenance is human-readable in this slice. Machine-readable metadata is out of scope (next PRD).

## Acceptance Criteria (PRD §7, restated)

The implementer must satisfy every bullet:

1. `record --all` produces byte-identical patch as default `record` in same repo state, records `working-tree-all` provenance explicitly.
2. `record --staged` captures staged changes; ignores unrelated unstaged paths with a one-line note.
3. `record --staged` refuses when captured staged paths also have unstaged edits.
4. `record --staged` includes new files only when represented in the index.
5. `record --unstaged` captures unstaged changes; ignores unrelated staged paths with a note.
6. `record --unstaged` refuses when staged and unstaged edits overlap on a path.
7. `record --unstaged` includes plain untracked files that pass filters.
8. `record --claimed-only` refuses when no claims exist.
9. `record --claimed-only` captures only claimed paths in default worktree mode.
10. `record --auto --claimed-only` intersects auto committed-range capture with active claims.
11. `--files` and `--claimed-only` combine as an intersection; refuse when intersection is empty.
12. All mode mutexes return clear errors *before* patch capture.
13. Successful records include capture-mode provenance in `record.md`.
14. Existing record tests, auto-base tests, collision tests, and dependent-amend tests stay green.
15. Docs and skill assets update only if user-facing recommendation changes (PRD permits not updating these unless necessary).

## Frozen Regions (do NOT touch in this slice)

- Wave D reconcile files: `internal/workflow/reconcile_check_applied.go`, `internal/workflow/reconcile_auto_drop.go`, `internal/cli/reconcile_check_applied.go`, `internal/cli/reconcile_auto_drop.go`
- Provider audit files: `internal/provider/{errors,responses,router}.go`
- Alpha-1 surface: `internal/store/claims.go`, `internal/cli/feature_claim.go` (you may *read* `claims.go` to integrate `--claimed-only`, but do not modify it)
- The `## Side Research` section in this CURRENT.md (preserve byte-identical across handoff resets; md5 `b385fe622db9926f48861105239f113e`)

## Files Expected to Change (implementer guidance — not exhaustive)

- `internal/cli/cobra.go` (or wherever the `record` command is wired) — add flags + mutex validation
- New `internal/gitutil/` helpers — `CaptureStagedPatch(...)`, `CaptureUnstagedPatch(...)`, path-overlap detection (staged ∩ unstaged), index-only untracked-file enumeration
- `internal/cli/record.go` or equivalent — mode dispatch, refusal diagnostics, provenance assembly
- New file `internal/cli/record_modes_test.go` (or extend existing record tests) with end-to-end coverage of every PRD §7 acceptance bullet
- New gitutil unit tests for staged/unstaged capture helpers
- `record.md` template / writer — add capture-mode provenance section

## Test Discipline

- Race-clean: `go test ./... -count=1 -race` must be green.
- The new staged/unstaged path-overlap refusal tests must verify both the exit code (non-zero) AND the diagnostic message shape.
- The `record --all == default record` equivalence test must compare patch bytes verbatim from a fresh repo state.
- `--claimed-only` tests must depend on a pre-populated `claims.json` (use the alpha-1 `feature claim add` CLI to populate, do not write the JSON directly — exercise the integration).

## Blockers

None. Alpha-1 (`9d7435b` + verdict `788438b`) is the only structural prerequisite (`--claimed-only` reads its manifest) and is fully landed.

## Context for Next Agent

- This is the second of two Wave alpha slices. After this lands and external review approves, both slices ship together under the `v0.9.0` annotated tag.
- The PRD is binding and complete. Do not introduce new flags or new modes beyond what §3 specifies.
- `--all` is intentionally a no-op alias today (byte-equivalent to default), but it MUST record `working-tree-all` provenance explicitly so future patch-identity-metadata work has the signal. Don't skip the provenance write.
- The `## Side Research — State-of-the-art middle pass (2026-05-10)` section below is preserved verbatim across handoff resets.

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
