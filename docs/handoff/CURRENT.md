# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-wave-alpha-supersession-implementation`
- **Milestone**: v0.12.0 Wave α — implement `PRD-feature-supersession` + `ADR-028-supersession-edge-model`.
- **Description**: First wave of the v0.12.0 3-wave implementation cluster (supervisor picked Option A 2026-07-29). Implements the third-kind `depends_on[].kind: "supersedes"` edge on the ADR-011 dependency graph, 4 new composable labels, and reconcile suppression of superseded features. Extends (does NOT fork) the ADR-011 D1 storage lane.
- **Status**: Dispatched — implementer in flight.
- **Assigned**: 2026-07-29.

## Wave α scope (locked)

**Read these first, in order**:

1. `docs/prds/PRD-feature-supersession.md` (259 lines, 12 acceptance criteria) — the spec.
2. `docs/adrs/ADR-028-supersession-edge-model.md` (109 lines, D1–D8) — the locked model.
3. `docs/adrs/ADR-011-feature-dependencies.md` (D1–D4) — what you are extending. D1 storage lane (`status.json.depends_on[]`) is authoritative. D2 DFS cycle detection extends cleanly to the third edge kind. D3 composable-derived-labels pattern is what the 4 new labels attach to. D4 hard/soft semantics unchanged.

**What ships**:

- New third `kind: "supersedes"` value on `depends_on[]` entries, alongside existing `hard` and `soft`. Directed edge: `X supersedes Y` means X replaces Y.
- Reconcile suppression: superseded features are excluded from replay by default (both hard-parent replay in verify V7 and the general reconcile pass).
- 4 new composable labels on the ADR-011 D3 pattern: `superseded-by`, `active-superseder`, `stale-superseder`, `orphan-superseder`.
- Cycle detection extends via ADR-011 D2's existing DFS (X supersedes Y + Y supersedes X = cycle).
- `depends_on[].kind` schema addition ripples through skill assets — parity guards must update in the same commit.

**What does NOT ship in Wave α**:

- `write-file` recipe safety (`preimage_hash`, later-touch detection) → Wave β.
- Active feature session lane (`tpatch session` command group, `.tpatch/local/capture/`) → Wave γ.

## Cross-wave coordination

- **Wave β depends on Wave α**: `PRD-write-file-recipe-safety §PRD-1-interaction` says superseded features suppress/downgrade `write-file` drift on the parent verify path. Wave β cannot dispatch until Wave α acceptance.
- **Wave γ is independent** of Waves α/β. It touches `internal/cli/init.go`, new `tpatch session` command group, and a new `.tpatch/local/capture/` storage lane.

## Implementer directives (Wave α)

**Approach**: sequential slices per PRD acceptance criteria groupings. Recommended shape:

1. **Slice 1 — Schema + parity guards** (foundation, must land first):
   - Add `kind: "supersedes"` as a third valid value on `depends_on[]` in `internal/store/status.go` (or wherever the type lives).
   - Update `TestSkillRecipeSchemaMatchesCLI` / any dependency-schema parity guards in the SAME commit (Slice 1 anti-drift lesson from doctor Wave β).
   - Update the 6 shipped skill assets (`assets/*/SKILL.md` + prompt/recipe templates) so any documentation of `depends_on[].kind` mentions the new value.
2. **Slice 2 — Graph + cycle detection extension**:
   - Extend ADR-011 D2 DFS cycle detection to include supersedes edges (X supersedes Y + Y supersedes X = cycle).
   - Add unit tests for the mixed hard/soft/supersedes cycle cases.
3. **Slice 3 — 4 composable labels**:
   - Add `superseded-by`, `active-superseder`, `stale-superseder`, `orphan-superseder` to the ADR-011 D3 label composition pattern.
   - `tpatch status` / label rendering must surface these.
4. **Slice 4 — Reconcile suppression**:
   - `internal/workflow/reconcile.go` and verify V7 (`runClosureReplay` in `internal/workflow/verify.go`) exclude superseded features from the hard-parent closure by default.
   - Add regression tests for both paths.
5. **Slice 5 — CHANGELOG + docs**:
   - Add `## v0.12.0 — TBD` header + Wave α entry.
   - Flip `PRD-feature-supersession.md` and `ADR-028` status from `Proposed` → `Accepted`.
   - Update `docs/ROADMAP.md` v0.12.0 entry.

**Binding carry-forward rules** (all 20):

- **Rule 15**: any `tpatch` command referenced in docs must exist in `internal/cli/cobra.go`.
- **Rule 18**: every commit MUST carry a parseable `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer (verify via `git log -1 --format='%(trailers)'`).
- **Rule 19**: any diff touching exported `store`/`workflow`/`cli` surface MUST be justified by a PRD/ADR clause. Wave α touches store schema (kind value) + workflow (reconcile + verify) — both are PRD-scoped, but call out the exact clause in each commit message.
- **Rule 20**: for any user-facing CLI behavior change (label rendering, reconcile output, verify remediation strings), reproduce empirically in a scratch repo before shipping. Rigor extension optional but recommended for the reconcile-suppression regression tests.
- Full 20-rule list carry-forward from v0.11.3 close in HISTORY.md snapshots.

**Validation gates** (must pass before handoff back for review):

- `gofmt -l .` (empty output).
- `go vet ./...` (clean).
- `go build ./cmd/tpatch` (clean).
- `go test ./...` (full suite pass; currently 99 tests as of v0.11.3).
- Parity guard test (`assets_test.go` or `TestSkillRecipeSchemaMatchesCLI`) must pass with schema addition.
- Side Research md5 preserved: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)` = `b385fe622db9926f48861105239f113e`.

## Wave sequencing plan (v0.12.0)

1. **Wave α — Supersession** (this task): PRD-feature-supersession + ADR-028. Foundation for graph model.
2. **Wave β — Write-file safety** (after α acceptance): PRD-write-file-recipe-safety + ADR-029. Depends on α for reconcile-suppression interaction.
3. **Wave γ — Active-feature-session** (after β acceptance, OR parallel with β if capacity allows): PRD-active-feature-session. Independent code surface.

Ship v0.12.0 when Waves α+β+γ all acceptance-approved. Doctor cluster's 4-wave pattern (v0.11.2) is the proven template.

## Non-blocking follow-ups deferred

- ADR-027 F2 (LOW): PRD-ide-capture-hooks naming coord.
- Doctor S3-boundary (LOW): mixed-CHANGELOG scope documentation.
- Stream B ADR-029 informational nit: literalize `TestSkillRecipeSchemaMatchesCLI` symbol in parity contract prose (post-v0.12.0 docs polish).

## Carry-forward dispatch rules (20 binding)

Same 20 rules as v0.11.3 close. See prior CURRENT.md snapshots in HISTORY.md for full text. Rule 20 rigor extension pattern (detached-worktree-at-pre-fix + test-copy) remains optional.

## Session Summary

v0.11.3 shipped 2026-07-29 (Stream C closed GH #2). Streams A + B closed at three-way APPROVED 2026-07-29 (paper-only PRD/ADR drafts). Supervisor picked Option A 2026-07-29 — v0.12.0 3-wave sequential cluster kicked off with Wave α (supersession) implementer dispatched.

## Next Steps

1. Wave α implementer executes Slices 1–5 (schema/parity → cycle detection → labels → reconcile suppression → CHANGELOG/docs).
2. Wave α implementer updates this handoff at each phase transition (Rule 8 cadence, see AGENTS.md).
3. Supervisor dispatches internal reviewer + supervisor-external reviewer in parallel.
4. User's parallel external pass on rev-0.
5. On three-way APPROVED: archive Wave α to HISTORY.md, dispatch Wave β (write-file safety).

## Blockers

None.

## Context for Next Agent

- HEAD at v0.12.0 kickoff: `442fd4f` + supervisor decision LOG entry pending commit.
- Doctor cluster + v0.11.2 shipped; v0.11.3 shipped 2026-07-29 (Stream C).
- Streams A + B closed at three-way APPROVED 2026-07-29 (paper-only).
- 20 binding carry-forward rules. Rule 20 rigor extension pattern optional.
- Two-opinion protocol scoreboard: 17/17 rev cycles at final concurrence; user-external uniquely blocked/caught in 7 of 17 at rev-0 + one process-level F1 (handoff-stale) this consolidation.
- All 5 PRDs/ADRs from Streams A+B are `Proposed` and ready for implementation cluster.
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
