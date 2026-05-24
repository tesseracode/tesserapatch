# Current Handoff

## Active Task

- **Task ID**: `v0.10.0-release-decision`
- **Milestone**: v0.10.0 capture-and-metadata foundation cluster — **4-of-4 slices complete, ready for release tag**.
- **Description**: Cluster shipped. Awaiting user decision on release shape (tag, CHANGELOG entry, version bump).
- **Status**: Awaiting user decision.
- **Assigned**: 2026-05-23.

### Cluster summary

The v0.10.0 cluster (capture-and-metadata foundation) is complete. Four PRD slices landed cleanly through the implementation → internal review → external review cycle:

| Wave | PRD | ADR | Final commit | Closed |
|------|-----|-----|--------------|--------|
| α | `PRD-feature-capture-modes` | ADR-022 | (in `main`) | earlier in cluster |
| β | `PRD-feature-patch-identity-metadata` | ADR-024 | (in `main`) | 2026-05-19 |
| δ | `PRD-stable-patch-id` | ADR-025 | (in `main`) | earlier in cluster |
| γ | `PRD-feature-patch-amend` | ADR-026 | `3c71383` | 2026-05-23 |

Full Wave γ rev-0 → rev-2 stack and review history archived in `docs/handoff/HISTORY.md`. Wave β archive remains the canonical reference for the patch-generations schema invariants Wave γ built on.

### Next user-facing decision

1. **Release tag**: tag `v0.10.0` at `main` HEAD (currently `13872c9`)? Or wait for additional waves?
2. **CHANGELOG entry**: draft a v0.10.0 entry covering all four slices? (no CHANGELOG entries are added per-slice during a cluster cycle; aggregated at cluster close.)
3. **Next cluster**: pick from `docs/ROADMAP.md` pending work, or start a new exploratory PRD.

### Closed work pointer

- Wave γ archive: `docs/handoff/HISTORY.md` top entry (`2026-05-23 — v0.10.0 Wave γ patch-amend — COMPLETE`).
- Wave γ external APPROVED verdict: `docs/supervisor/LOG.md` top entry (`Wave γ patch amend rev-2 (external) — 2026-05-23`).
- Wave γ commit stack on `main`: `df35ab7..3c71383` (rev-0 + rev-1 + rev-2 + handoff/log commits).

### Process lessons from Wave γ (carry-forward)

1. Supervisor kickoff briefs MUST self-audit against binding ADRs before dispatch. F1 (rev-0) shipped because the brief said `fixup --target` against ADR-026 D4.
2. Briefs that reference policy ADRs (ADR-011 for dependency enforcement, etc.) MUST enumerate config-flag opt-out contracts explicitly, not just enforcement semantics. F3 (rev-1) shipped because the brief named ADR-011 but did not flag the `features_dependencies` opt-out.
3. Internal reviewer checklist must include explicit flag-off counter-scenarios for any new dependency-related enforcement. rev-1 internal reviewer verified flag-on but missed flag-off.
4. `gofmt` gotcha: `gofmt -l . 2>&1 | grep -v '^$'` returns exit 1 on empty input. Always run `gofmt -l .` directly and read literal output. Brief this in every dispatch.

## Session Summary

Wave γ rev-2 external APPROVED. v0.10.0 cluster is complete. Awaiting user decision on release shape (tag, CHANGELOG, next work).

## Current State

- v0.10.0 cluster 4-of-4 complete.
- No active implementation work.
- No blockers.
- `docs/state-of-the-art/` working-tree modifications remain untouched (pre-existing, not from this cluster).

## Files Changed

None this turn beyond LOG.md (external verdict) and HISTORY.md/CURRENT.md (archive + reset).

## Test Results

Full suite green at `3c71383` (rev-2 final): `gofmt -l .` clean, `go vet ./...` clean, `go build ./cmd/tpatch` succeeds, `go test ./... -count=1 -race` green, `go test ./assets/...` green.

## Next Steps

1. Await user decision on v0.10.0 release tag + CHANGELOG.
2. After release decision, pick next milestone from `docs/ROADMAP.md` or new exploratory work.

## Blockers

None.

## Context for Next Agent

- v0.10.0 cluster ships the capture-and-metadata foundation: capture modes (α), patch-generations manifest with content-addressed `pg_<12hex>` IDs (β), stable git patch-id (δ), and amendment semantics with dependency-aware staleness (γ). Together these are the substrate for future identity, replay, and amendment-tracking features.
- ADR-026 D1–D10 + IC1–IC6 + IC4 frozen regions remain the binding contract for any future amendment work.
- `features_dependencies` config flag is the user-controllable opt-out for ALL dependency-related enforcement (ADR-011 base + ADR-026 D5 stale-parent). Any new dependency gate MUST honor it — see `internal/cli/cobra.go:856-862` and `internal/workflow/dependency_gate.go` for the pattern.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`. Always verify after editing CURRENT.md.

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
