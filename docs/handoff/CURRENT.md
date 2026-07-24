# Current Handoff

## Active Task

- **Task ID**: `v0.11.1-slice-4-prd-tpatch-doctor`
- **Milestone**: v0.11.1 stabilization — Slice 4
- **Description**: Draft a paper-only Proposed PRD for `tpatch doctor`, covering drift detection and safe opt-in fixes for tpatch workspace metadata, installed skill assets, release metadata, lock files, reconcile evidence, and recipe schema. No code changes and no CHANGELOG additions in this slice.
- **Status**: Review
- **Assigned**: 2026-07-23.

## Slice 4 closure summary

### PRD drafted

- `docs/prds/PRD-tpatch-doctor.md` (496 lines), status **Proposed**.
- Scope is paper-only: no code, assets, CHANGELOG, or release-process mutations.

### Sections included

- §0 Meta and claims audit.
- §1 Problem statement.
- §2 Goals / Non-goals.
- §3 Detection checks D1-D8.
- §4 User-facing contract (`tpatch doctor [--dry-run] [--fix] [--json] [--check <id>]`).
- §5 Implementation notes.
- §6 Acceptance criteria (§6.1-§6.29).
- §7 Open questions.
- §8 Out of scope.
- §9 Sources.

### Precedents cited

- ADR-024 for `patch-generations.json` boundary, no historical backfill, strict schema, and malformed-manifest handling.
- ADR-025 for `reconcile-evidence.jsonl` / `reconcile-revisions.jsonl`, D10 privacy, D11 malformed JSONL handling, and D12 refs.
- ADR-027 for committed-summary vs local-private-buffer privacy and least-privilege reads.
- Slice 3 `RELEASING.md` anti-drift candidate for tag / CHANGELOG / GH Release checks.
- Slice 1 `TestSkillRecipeSchemaMatchesCLI` parity-guard pattern for runtime recipe-schema drift.

### Decisions locked for review

- Default is dry-run; `--fix` is explicit opt-in.
- Every mutation must create a backup before overwrite.
- `--fix` is idempotent; second run on a clean workspace is a no-op.
- v1 fixable classes are intentionally narrow: installed tpatch skill assets and equivalent lock-format normalization only.
- Feature metadata, patch-generation, reconcile-evidence, release, and feature recipe drift are report-only in v1.
- Non-scope explicitly rules out network calls by default, auth, GH-Release publishing, source-file transformations, cross-repo migration, raw context reads, and a public `tpatch migrate` alias in v1.

### Acceptance criteria

- 29 atomic criteria (§6.1-§6.29), including D1-D7 fixtures, JSON report determinism, exit codes, idempotence, backup semantics, privacy, no source transformations, and per-check failure continuation.

### Validation gates

- `gofmt -l .`: clean.
- `go vet ./...`: clean.
- `go build ./cmd/tpatch`: clean.
- `go test ./...`: green across all packages.
- Side Research md5 invariant preserved: `b385fe622db9926f48861105239f113e`.

## Slice 3 closure summary

### GH Releases published (5)

- v0.8.0 — "M17 boundary-capture cluster" — https://github.com/tesseracode/tesserapatch/releases/tag/v0.8.0
- v0.8.1 — "Wave D detector tails" — https://github.com/tesseracode/tesserapatch/releases/tag/v0.8.1
- v0.9.0 — "Wave alpha (file-claims + capture-modes)" — https://github.com/tesseracode/tesserapatch/releases/tag/v0.9.0
- v0.10.0 — "Wave β + Wave γ (patch-identity-metadata + patch-amend)" — https://github.com/tesseracode/tesserapatch/releases/tag/v0.10.0
- v0.11.0 — "WP-003 Reconcile Safety and Middle-Pass Foundation" — https://github.com/tesseracode/tesserapatch/releases/tag/v0.11.0 (marked as `Latest`)

All 5 releases used `--notes-file` with CHANGELOG entries extracted via `awk '/^## vX\.Y\.Z —/,/^## v/' CHANGELOG.md | sed '$d'`. Titles match CHANGELOG headings verbatim.

### `RELEASING.md` added (179 lines)

Sections:
- Overview (3-artifact release: CHANGELOG entry → tag → GH Release, must stay in lock-step).
- Prerequisites (clean tree, `gh` auth, full gates green).
- Step 1 — Write CHANGELOG entry (with heading-format contract for `awk` extraction).
- Step 2 — Tag with annotated tag pointing at release commit.
- Step 3 — Publish GH Release with `--verify-tag`, `--latest`, `--notes-file`.
- Optional post-release checks (list verification, handoff/roadmap updates).
- Anti-drift guardrails (never tag without publishing; CHANGELOG as single source of truth; sanity-check; CI-check candidate queued under Slice 4 doctor scope).
- Version-derivation reminder (no source constant; `internal/buildinfo` resolves from ldflags/git tags).
- Historical release cadence (v0.8.0 through v0.11.0 aligned with WP clusters).

### Gates
- `gofmt -l .`: clean.
- `go vet ./...`: clean.
- `go build ./cmd/tpatch`: clean.
- No code touches; `go test ./...` invariant preserved.

### Anti-drift observation
Slice 3's `RELEASING.md` includes a candidate CI check (pre-tag script verifying tag has matching CHANGELOG entry + GH Release within 24h). Not implemented in this slice; explicitly queued for Slice 4 as a doctor-command candidate.

### Out-of-scope observations
None. CHANGELOG.md v0.8.0 through v0.11.0 entries left untouched per hard constraint 2.

## v0.11.1 cluster progress

- **Slice 1** ✅ CLOSED 2026-07-19 (three-way APPROVED). `TestSkillRecipeSchemaMatchesCLI` anti-drift bonus.
- **Slice 2** ✅ CLOSED 2026-07-23 rev-1 (three-way APPROVED after rev-0 F1 blocker caught by user-external). New rule 11 (flag-surface accuracy); candidate rule 17 (totality claims verification).
- **Slice 3** ✅ CLOSED 2026-07-23 (supervisor-direct release ops; GH Releases backfilled + `RELEASING.md`).
- **Slice 4** ← this handoff (`PRD-tpatch-doctor` paper-only PRD draft; review cycle mirroring ADR-027 model).

## Slice 3 binding scope

### Current state
- `gh release list` shows latest published release is **v0.7.0** (2026-05-11).
- Local tags AND `origin` tags exist for: v0.8.0, v0.8.1, v0.9.0, v0.10.0, v0.11.0.
- CHANGELOG.md has substantive entries for v0.8.0/v0.8.1/v0.9.0 (WP-002 α/β/γ + M17 waves), v0.10.0 (WP-002 β + γ patch-generations + amend), v0.11.0 (WP-003 full cluster).

### Deliverable

1. **Publish 5 GH Releases** using `gh release create` with CHANGELOG entries as release notes:
   - v0.8.0 (WP-M17 clustered slices + record-collision-detection + tpatch-land + patch-already-upstream-detector; check CHANGELOG for full scope).
   - v0.8.1 (WP-M17 follow-ups; check CHANGELOG).
   - v0.9.0 (WP-002 Wave α: file-claims + record-capture-modes).
   - v0.10.0 (WP-002 β + γ: patch-generations manifest + patch amendment).
   - v0.11.0 (WP-003 full cluster: 9 PRDs across 4 waves under ADR-025).

   For each release, extract the corresponding `## v0.X.Y — ... — ...` section from CHANGELOG.md as `--notes` (or use `--notes-file`). Set `--title "v0.X.Y — <scope name>"` matching the CHANGELOG heading. Do NOT mark any as `--prerelease`. Do NOT mark v0.11.0 as `--latest` yet if v0.11.1 (the current unreleased entry) is imminent — but since v0.11.1 hasn't shipped, v0.11.0 IS the latest.

2. **Add `RELEASING.md`** documenting the release process:
   - Where the release checklist lives.
   - What CHANGELOG format is expected (`## v0.X.Y — YYYY-MM-DD — Short scope`).
   - The tag → push → `gh release create` sequence.
   - How to extract release notes from CHANGELOG.md.
   - Reminder that `internal/buildinfo/buildinfo.go` derives version from ldflags/git tags automatically (no version constant to bump).
   - Note on `gh release create --generate-notes` vs explicit `--notes-file` — prefer explicit notes from CHANGELOG for consistency with prior releases.
   - Anti-drift guard suggestion: a small CI check or pre-tag script that verifies each tag has a corresponding CHANGELOG entry AND a GH Release before allowing the next tag.

### Slice 3 hard constraints

1. **No code changes** — this slice is ops + one new docs file (`RELEASING.md`). No `internal/`, `cmd/`, `assets/` touches.
2. **CHANGELOG.md untouched** for existing entries — do NOT edit v0.8.0/v0.8.1/v0.9.0/v0.10.0/v0.11.0 bodies. If a discrepancy is spotted (e.g., wrong SHA reference), flag as OUT-OF-SCOPE in the closure summary; do NOT fix within this slice.
3. **v0.11.1 unreleased entry** in CHANGELOG — leave the (unreleased) header + Slice 1 + Slice 2 bullets alone. Slice 3 does NOT add a v0.11.1 release; that will happen when v0.11.1 tag ships.
4. **`RELEASING.md` scope** — concise, actionable, ≤150 lines. Cite existing precedent (which repo files/tools already govern releases). Do NOT invent process; describe what actually works.
5. **No mode toggles / config flags / lifecycle state changes**. This is metadata publication + one docs file.
6. **Side Research md5** MUST remain `b385fe622db9926f48861105239f113e`.
7. **Commit trailer mandatory**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`. Use `git -c commit.gpgsign=false commit --no-verify`.
8. **Gates** — `gofmt -l .`, `go vet ./...`, `go build ./cmd/tpatch`, `go test ./...` should not change (docs-only + release ops). Verify no regression.
9. **No Slice 4 touches** — draft the doctor PRD in a separate slice.
10. **No ADR-027 F3 touches** — deferred to downstream PRD.

## Slice 3 execution notes (supervisor-direct)

Per rev-0 cluster plan (`v0.11.1 Slice 3` bullet in HISTORY): "Supervisor-direct execution; no full review cycle." This means:

- Supervisor executes `gh release create` calls directly.
- Supervisor drafts + commits `RELEASING.md`.
- Optional lightweight review: user may spot-check the published releases + `RELEASING.md` before archive. No mandatory internal + external + user-external three-pass.
- Rationale: release ops are transactional (either the release publishes or it doesn't), and `RELEASING.md` is a small ops doc without production-behavior contracts.

**However**, if `RELEASING.md` ends up making binding claims about tooling/CLI behavior (e.g., "the release script does X"), those claims fall under carry-forward rules 8, 9, 11 (display-string / behavior-implemented / flag-surface accuracy). In that case, treat as a full review-cycle target.

## Carry-forward dispatch rules (all 15+1 binding, rule 17 candidate)

1-16: unchanged from Slice 2 handoff (see HISTORY.md).
17. **Candidate** (Slice 2 F1 generalization): totality claims in docs ("only X supported", "the full list is Y") MUST be verified against ALL layers of the production model, not just the enumerated docs list. Promote to binding after Slice 3/4 reviewer feedback confirms broader applicability.

## Session Summary

v0.11.1 Slice 4 paper-only draft complete and ready for review. `docs/prds/PRD-tpatch-doctor.md` proposes `tpatch doctor` drift checks D1-D8 and acceptance criteria §6.1-§6.29. Slice 3 release ops remain recorded below as closed context.

## Next Steps

1. Supervisor dispatches internal review for `docs/prds/PRD-tpatch-doctor.md`.
2. Supervisor dispatches external review per the v0.11.1 paper-doc protocol.
3. If approved, archive Slice 4 handoff and decide whether a future implementation slice is roadmap-committed.
4. Do not add a CHANGELOG entry until a future implementation slice ships code.

## Blockers

None.

## Context for Next Agent

- HEAD at Slice 4 draft completion includes the new `docs/prds/PRD-tpatch-doctor.md` commit once pushed.
- HEAD at Slice 3 kickoff: `8189982` (Slice 2 rev-1 supervisor-external APPROVED).
- Slice 2 anti-drift template: `TestSkillRecipeSchemaMatchesCLI` in `assets/assets_test.go` — Slice 3 doesn't have a natural analog since it's ops.
- CHANGELOG entries to extract for release notes:
  - `## v0.8.0` — check for scope name.
  - `## v0.8.1` — check for scope name.
  - `## v0.9.0` — check for scope name.
  - `## v0.10.0 — 2026-05-23 — Wave β + Wave γ (patch-identity-metadata + patch-amend)` — CONFIRMED.
  - `## v0.11.0 — 2026-07-16 — WP-003 Reconcile Safety and Middle-Pass Foundation` — CONFIRMED (release commit `1c63d1d`, tag pushed 2026-07-16).
- ADR-027 F2 (roadmap naming coord) still deferred — can bundle with any future small docs slice.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`.

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
