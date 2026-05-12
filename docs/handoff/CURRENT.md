# Current Handoff

## Active Task

- **Task ID**: `m17-wave-bd-parallel-dispatch` — M17 Wave B + Wave D (v0.8.0 boundary-capture cluster, second + third waves; can ship in parallel)
- **Milestone**: M17 — boundary-capture cluster, v0.8.0
- **Status**: Wave A shipped. Wave B implemented (awaiting review). **Wave D implemented (awaiting review)**. Wave C still blocked on B.
- **Assigned**: 2026-05-11 (immediately after Wave A external approval)

## Wave D summary (2026-05-11)

Phase-1.5 deterministic patch-already-upstream detector landed,
default-OFF behind `Config.PatchIDDetectorEnabled` (PRD §6). Files
touched:

- `internal/store/types.go` — added `Config.PatchIDDetectorEnabled`,
  `Config.PatchIDScanLimit`, `DefaultPatchIDScanLimit = 5000`,
  `PatchIDMatch` struct, `ReconcileSummary.PatchIDMatch *PatchIDMatch`
  (omitempty).
- `internal/store/store.go` — flat YAML parser learns
  `patch_id_detector_enabled` and `patch_id_scan_limit`; `SaveConfig`
  emits the keys only when non-default (preserves pre-Wave-D byte
  identity for fixtures).
- `internal/gitutil/patch_id.go` — new file. Primitives `PatchID`,
  `CommitPatchID`, `RevListInRange` wrap `git patch-id --stable` and
  `git rev-list --no-merges`.
- `internal/workflow/patch_id_detector.go` — new file. `runPatchIDDetector`
  reads `upstream.lock`, computes our patch-id, walks the range, applies
  PRD §5.3 multi-match policy (earliest match wins), enforces the
  scan-limit cap (PRD §5.2), fails soft on tooling errors (PRD §5.1).
- `internal/workflow/reconcile.go` — fast-path slotted between phase 1
  (reverse-apply) and phase 2 (operation-level). Gated on
  `cfg.PatchIDDetectorEnabled`; on match sets `Outcome=ReconcileUpstreamed`,
  `Phase="phase-1.5-patch-id-match"`, `UpstreamCommit = matched SHA`,
  `PatchIDMatch = audit payload`, skips phases 2/3/4.
- `internal/workflow/patch_id_detector_test.go` — new file. Coverage:
  default-OFF no-op, primitive match, primitive no-match, missing-lock
  Skip, empty-baseline Skip, empty-range no-match, scan-limit Skip,
  flag-on integration match (provider=nil) with `status.json`
  persistence assertion, flag-on no-match falls through, config
  parser round-trip for the new keys.
- `CHANGELOG.md` — Wave D entry under v0.8.0 in-development; explicitly
  flags default-OFF gating and lists deferred CLI surfaces.

### PRD acceptance criteria — what shipped and what deferred

Shipped: PRD §3.1 (phase-1.5 default behaviour), §3.4 JSON
`patch_id_match` block, §4 schema additions, §5.1 fail-soft semantics,
§5.2 scan-limit cap, §5.3 multi-match policy, §5.6 heuristic-fallback
friendliness, §6 default-OFF rollout, §9 validation plan items 1-4 and
7-9.

Deferred to follow-up backlog (not in this commit, called out in
CHANGELOG): PRD §3.2 `--check-applied-only` CLI verb/flag, PRD §3.3
`--auto-drop-merged` CLI flag, PRD §3.3 hotfix-kind auto-drop default
gating. Rationale: the deterministic primitive + reconcile fast-path
is the load-bearing M17 Wave D contract; the user-facing flags layer
on top and can ship in v0.8.1+ without invalidating the core. Brief
permits this scoping ("if you add a CLI flag, ..."). No skill files
were touched (no new CLI flag → parity guard unaffected).

### Verification

- `gofmt -l .` clean.
- `go build ./cmd/tpatch` clean.
- `go test ./...` green across all packages.
- New `TestPatchIDDetector_*` suite (9 tests) passes; new
  `TestConfigParserRoundTripsPatchIDKeys` passes.
- Skill parity guard test (`go test ./assets`) still cached/green —
  no CLI flag changes, no skill asset changes.

### Default-OFF preservation — explicit confirmation

`TestPatchIDDetector_DefaultOffNoOp` asserts that a freshly-init
config keeps `PatchIDDetectorEnabled` false and that reconcile does
*not* enter phase 1.5 nor populate `PatchIDMatch`. All pre-existing
reconcile tests (`TestReconcilePhase1_*`, `TestReconcilePhase4_*`,
labels tests, etc.) continue to pass without modification — none of
them enable the flag, so the new fast-path is silently skipped and
the original phase ladder runs unchanged.

## Wave Status

| Slice | Task ID | PRD | Status | Depends |
|-------|---------|-----|--------|---------|
| A1 | `impl-record-auto-base` | PRD-record-auto-base | ✅ shipped (`1d6179c` + rev-1 `4484e04`) | — |
| A2 | `impl-reconcile-lock-guard` | PRD-reconcile-lock-guard | ✅ shipped (`8fc2e4e`) | — |
| **B** | `impl-record-collision-detection` | PRD-record-collision-detection | 🟡 **implemented, awaiting review** | A1 (recovery hints reference `--auto`) |
| C | `impl-tpatch-land` | PRD-tpatch-land | ⬜ blocked on A1+A2+B | A1, A2, B |
| **D** | `impl-patch-already-upstream-detector` | PRD-patch-already-upstream-detector | 🟡 **implemented, awaiting review** (default-OFF) | independent (default-OFF) |

**Coordination notes for parallel B + D dispatch**:
- Same checkout, same working tree — same risk pattern as Wave A's parallel dispatch.
- Recommend: dispatch B and D in parallel but with explicit "do not touch the other's territory" lists in their briefs.
- B touches `internal/cli/cobra.go` record path + `internal/gitutil/` (collision-signature primitive); D touches reconcile workflow (`internal/workflow/reconcile.go`) + `internal/store/types.go` (config flag `PatchIDDetectorEnabled`).
- Minimal overlap — main shared file is `cobra.go` if D adds a `--no-patch-id-detector` flag. Risk lower than Wave A's A1↔A2 because they share fewer surfaces.

## Just Shipped — M17 Wave A

**M17 Wave A (A1 + A2)** — APPROVED WITH NOTES, archived to HISTORY 2026-05-11. Ship stack: `1d6179c` (A1 v0) + `8fc2e4e` (A2) + `6d67b41` (verdicts) + `4484e04` (A1 rev-1) + `63a0373` (rev-1 verdict). External one revision on A1 (zero-diff false-green + lock-fallback policy); both addressed in `4484e04`. A2 clean. One non-blocking external follow-up captured as backlog `m17-wave-a1-followup-ambig-discovery-diag`. Cross-commit binding (A1 ↔ A2) accepted; HISTORY notes the revert must move both as a unit.

## Wave B — Just Implemented (2026-05-11)

**Status**: Implementation complete on `main`, awaiting sub-agent reviewer + external supervisor.

**Summary**: Cross-feature byte-identical canonical patch collision detection at `tpatch record` time, per `docs/prds/PRD-record-collision-detection.md`. Refuses by default, overridable with `--allow-collision "<reason>"`. Same-feature re-record with unchanged bytes is treated as deduplication (numbered audit snapshot skipped, canonical artifact rewritten in place).

**Files changed**:
- `internal/gitutil/collision.go` — new `PatchSignature(patch) (sha256Hex, bytes)` primitive. Tiny stdlib-only helper. Reusable by future `tpatch patches --collisions` (PRD §7).
- `internal/gitutil/collision_test.go` — deterministic / distinct-bytes / empty-string coverage for the primitive.
- `internal/cli/record_collision.go` — `scanCanonicalPatchCollisions(store, slug, patch)` enumerates `.tpatch/features/*/artifacts/post-apply.patch`, applies the length → SHA-256 → byte-for-byte ladder (PRD §4), splits matches into same-feature vs cross-feature buckets. `printCollisionRefusal` writes the PRD §3.1 diagnostic and tailors recovery hints per capture mode (PRD §5).
- `internal/cli/cobra.go` — wired the scan into `recordCmd` AFTER empty-patch handling and BEFORE `WriteArtifact`; added `--allow-collision` flag; same-feature dedup skips `WritePatch`; `generateRecordMD` extended to persist the override reason under a "Collision Override" section.
- `internal/cli/record_collision_test.go` — 9 acceptance tests covering each PRD §8 row (cross-feature refusal, allow-collision override, same-feature dedup, changed-bytes appends, ≥3 colliders recommendation, missing artifact ignored, empty capture skips scan, working-tree + from-mode recovery hints).
- `internal/cli/record_auto_test.go` — existing `TestRecordAuto_AutoEqualsFromExplicit` updated to use `--allow-collision` for its deliberate byte-equivalence assertion (the test records the same bytes under two slugs to verify `--auto` == `--from`; collision detection would otherwise refuse it).
- `CHANGELOG.md` — `### Wave B` subheading under unreleased v0.8.0.
- `docs/record.md` — "Cross-feature collision detection (v0.8.0)" section with the refusal example and dedup semantics.

**Test results**:
- `gofmt -l .` clean
- `go build ./cmd/tpatch` clean
- `go test ./...` all green (full suite)
- `go test ./assets -run TestSkillParityGuard` green (no CLI command surface changes; flag-only addition does not require skill updates — the parity guard inspects `requiredCommands` and `requiredAnchors`, none of which mention `--allow-collision`)

**Notable**:
- The collision diagnostic is printed directly to `cmd.ErrOrStderr()`, then a short error is returned. Same pattern as the rest of `recordCmd` (`Execute()` in `cobra.go` will additionally print `error: <err>` from `os.Stderr`). The user sees the long diagnostic once and the short error once.
- `--allow-collision <reason>` requires a non-empty trimmed reason. Empty / whitespace-only reasons fail the override and the refusal stands. This matches the PRD §3.1 contract ("the reason is required to proceed").
- The scan helper lives in `internal/cli/` (not `internal/gitutil/`) because it depends on `store.Store` for feature enumeration. The brief allowed either layer; only the byte signature primitive landed in gitutil to satisfy the "collision-signature primitive" line of the brief.
- One pre-existing test (`TestRecordAuto_AutoEqualsFromExplicit`) deliberately records the same patch bytes under two slugs. It was updated to pass `--allow-collision` with a self-describing reason rather than restructure the assertion — the test still validates byte equality, which is the PRD §6 row 6 acceptance for Wave A1.
- No changes to `internal/workflow/reconcile.go`, no changes to `Config.PatchIDDetectorEnabled` — Wave D territory left untouched.
- `internal/cli/record_auto.go` not modified.

**For the reviewer (start here)**:
1. `internal/cli/cobra.go` recordCmd block from `// Determine capture mode once` through the WritePatch/dedup branch — that is the wiring. Confirm refusal happens before `WriteArtifact` (it does: scan runs immediately after `empty capture` handling and before any artifact write).
2. `internal/cli/record_collision.go` — verify PRD §4 algorithm (length → SHA-256 → bytes), missing-file skip, and the recovery-hint switch per capture mode (PRD §5).
3. `internal/cli/record_collision_test.go` — each PRD §8 acceptance row has a named test.
4. The two diagnostic paths: refusal (no artifact write) vs override (stderr warn + record.md persist).

## Tagging Decision (Open)

Wave A alone is partial v0.8.0. Two paths:
1. **Tag `v0.8.0-alpha.1` after Wave A push** — early-adopter signal, enables progress visibility, follows precedent of mid-milestone alphas. Cost: pre-release tag count grows; minor versioning bookkeeping.
2. **Defer tagging until Wave A+B+C+D complete** — single clean `v0.8.0` release. Cost: longer dark period; if Waves B/C/D are large, no incremental ship signal.

Recommendation: defer (option 2). Wave A is internal-facing infrastructure; user-facing value lands with Wave C (`tpatch land`). Tag once at v0.8.0 unless we hit a long pause between waves.

## Dispatch Plan (proposed)

1. Push current 5-commit stack (`1d6179c` + `8fc2e4e` + `6d67b41` + `4484e04` + `63a0373`) to `origin/main`. No tag.
2. Dispatch `m17-wave-b-impl` background agent with `PRD-record-collision-detection.md` as authoritative brief.
3. Dispatch `m17-wave-d-impl` background agent with `PRD-patch-already-upstream-detector.md` as authoritative brief + reminder of default-OFF gating.
4. Each implementer completes → sub-agent reviewer → external supervisor → push. Same pattern as Wave A.
5. After both B and D ship: dispatch `m17-wave-c-impl` (depends on A1+A2+B; D independent, can have shipped or be in flight).
6. After all 4 waves ship + follow-ups assessed: tag `v0.8.0`, archive cluster to HISTORY.

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

None. Awaiting user direction on:
- Wave A push (5 commits, no tag) — user can drive or supervisor can execute on confirmation.
- Wave B + D parallel dispatch — user OK with same pattern as Wave A?

