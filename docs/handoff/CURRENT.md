# Current Handoff

## Active Task

- **Task ID**: `v0.11.1-slice-2-reconcile-docs`
- **Milestone**: v0.11.1 stabilization — Slice 2
- **Description**: Rewrite `docs/reconcile.md` (last touched 2026-05-11, pre-Wave-α, zero v0.11 evidence-system keyword matches) to cover the entire WP-003 cluster (all 9 PRDs + ADR-025 D1-D13). Docs-only; full review cycle. Optionally roll in ADR-027 F2 (roadmap naming coordination for `PRD-ide-capture-hooks`).
- **Status**: Review.
- **Assigned**: 2026-07-19.

## v0.11.1 cluster progress

- **Slice 1** ✅ CLOSED 2026-07-19 (three-way APPROVED). Snapshot in HISTORY.md.
- **Slice 2** ← this handoff (docs refresh).
- **Slice 3** — release ops cleanup (5 missing GH Releases, add `RELEASING.md`); supervisor-direct execution, no full review cycle.
- **Slice 4** — `PRD-tpatch-doctor` paper-only PRD draft; full review cycle mirroring ADR-027 model.

## Slice 2 binding scope

### Current state of docs/reconcile.md

- File exists at `docs/reconcile.md`, 110 lines, last commit `32ad3a5` (2026-05-11 — pre-Wave-α).
- Zero grep matches for: `evidence`, `revision`, `confirmation gate`, `hunk-overlap`, `blocked_category`, `path-restructure`.
- Represents pre-WP-003 reconcile mental model. Ships in the tree but user-facing docs describe a version of reconcile that no longer exists.

### Target state

Rewrite `docs/reconcile.md` to describe v0.11 reconcile end-to-end:

1. **Evidence + revision schema (ADR-025 D1-D13)** — persisted artifacts (`reconcile-evidence.jsonl`, `reconcile-revisions.jsonl`), content-addressed IDs (`re_<12hex>`, `rr_<12hex>`), lenient reader / strict writer semantics, `corrupt_entries` JSON envelope, malformed-artifact handling.
2. **Wave α surfaces**:
   - **PRD 1 reconcile-verdict-evidence**: every reconcile pass writes an attempt row; evidence artifact reference (`evidence_artifact`) surfaces in `status.json` runtime field + human `evidence:` hint.
   - **PRD 6 file-novelty**: persisted `file-novelty` evidence uses `all-new-files` / `mixed-additive` / `modifies-existing-files` / `deletes-or-renames` / `unknown`; PRD 8 maps additive evidence into the `clean-additive` blocked category.
3. **Wave β surfaces**:
   - **PRD 2 confirmation gate**: `upstreamed` verdicts pass through a gate; unconfirmed candidates downgrade to `blocked` with `review_verdict=rejected-upstreamed` + display `[upstreamed-candidate]`. New `tpatch reconcile confirm-upstreamed <slug> [--json]` triggers audit + revision-pass append.
   - **PRD 3 revision-pass log**: `tpatch reconcile review add` + `tpatch reconcile review list [--json]`; `--json` emits `corrupt_entries` array on malformed lines and exits non-zero.
   - **PRD 7 hunk-overlap detector**: deterministic line-range pass after file-novelty; default `nearby-window=3`.
4. **Wave γ-1 surfaces**:
   - **PRD 4 retirement-state-audit**: `tpatch reconcile audit-retirement <slug> [--json]` read-only audit; auto-runs after `confirm-upstreamed`; appends `cleanup-needed` revision entries.
   - **PRD 5 study-validator**: dev-only `internal/tools/studyvalidator/` (NOT public CLI); enforces per-corrected-verdict linkage.
   - **PRD 8 blocked-verdict-taxonomy**: 8-category classifier with deterministic precedence (`dependency-blocked > validation-blocked > target-deleted > structural-conflict > edit-overlap > shifted-context > clean-additive > unknown-blocked`); evidence-metadata, not a persisted enum.
5. **Wave γ-2 surface**:
   - **PRD 9 path-restructure-detector**: emits `path-restructure` evidence (`prefix-move` / `prefix-split` / `target-deleted` / `mixed` / `none` / `unknown`) via Git name-status; feeds PRD 8 to upgrade `blocked` → `structural-conflict` / `target-deleted`. Thresholds config-driven; candidate prefixes capped at 5 sorted support-desc + path-asc.
6. **Privacy invariants (ADR-025 D10)** — no source bodies / transcripts / prompts / vector artifacts persisted.

Cross-link ADR-025 D-clauses + all 9 PRDs. Consider a small ASCII diagram or table showing the reconcile pass order: file-novelty → hunk-overlap → path-restructure → blocked-taxonomy classifier → confirmation gate → revision-pass writer.

### Optional roll-in — ADR-027 F2

ADR-027's Blocks header references `PRD-ide-capture-hooks` but `research-roadmap.md` §3.1 uses different naming. Since Slice 2 already touches docs surfaces adjacent to the roadmap, implementer may (at their discretion, with brief rationale) reconcile the naming as part of Slice 2 or defer to a separate small edit. Keep scope tight if it grows.

## Slice 2 hard constraints (binding)

1. **Docs-only** — no code changes. No `internal/`, `cmd/`, `assets/` touches. Just `docs/reconcile.md` + optionally `research-roadmap.md` for the F2 roll-in.
2. **PRD verbatim citations** — every claim about a v0.11 behavior must cite either the PRD by name or ADR-025 by D-clause. Reviewers will grep for citation coverage.
3. **CLI accuracy** — every `tpatch <command>` string in the docs must match the actual CLI surface. Grep-verify against `internal/cli/cobra.go` and asset skills before publishing.
4. **JSON schema accuracy** — any embedded JSON example must match the actual persisted artifact shape (mirror Slice 1 Finding 1 lesson: run examples through a decoder if in doubt).
5. **Privacy** — do not paste actual reconcile evidence bytes from any repo into the docs. Use synthetic examples.
6. **Side Research md5** in `docs/handoff/CURRENT.md` MUST remain `b385fe622db9926f48861105239f113e`. Verify: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.
7. **Commit trailer mandatory**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`. Use `git -c commit.gpgsign=false commit --no-verify`.
8. **Validation gates**: `gofmt -l .`, `go vet ./...`, `go build ./cmd/tpatch`, `go test ./...`. Docs-only changes should not change any gate result; verify no regression.
9. **No Slice 3 / Slice 4 file touches**. No CHANGELOG update until Slice 2 lands (Slice 1's v0.11.1 unreleased entry stays as-is; Slice 2 adds a bullet).
10. **No ADR-027 F3 touches** (D1 local-buffer path — downstream PRD will lock).

## Carry-forward dispatch rules (all 15 binding, rule 16 candidate)

1. (Wave α) Briefs MUST self-audit against binding ADRs.
2. (Wave α) Briefs naming policy ADRs MUST enumerate config-flag opt-out contracts.
3. (Wave α) Internal reviewer checklist MUST include flag-off counter-scenarios.
4. (Wave α) `gofmt -l .` direct — never piped.
5. (Wave α) Drift self-audit during release prep.
6. (Wave α rev-0) Briefs MUST NOT contain escape hatches that override PRD acceptance.
7. (Wave α rev-1) External reviewer briefs MUST sweep ALL PRD §6 acceptance criteria.
8. (Wave β F1) Briefs MUST enumerate per-PRD §6 display-name contracts as binding production + test contracts.
9. (Wave β F2) Reviewer briefs MUST distinguish "behavior implemented" from "behavior tested". Read production code FIRST.
10. (Wave β F6) State-mutation contracts MUST be verified by reloading from disk.
11. (Wave β F7) Cross-artifact linkage contracts MUST be verified by loading persisted JSONL.
12. (Wave β F3) Privacy tests seed secrets into title/slug/path metadata, NOT file content.
13. (Wave β schema-lock) Briefs say "no persisted-schema additions outside what binding ADRs explicitly authorize" — enumerate which fields/clauses.
14. Two-opinion external review protocol (supervisor + user-parallel) MANDATORY. 8/8 rev cycles has caught HIGH BLOCKERs or confirmed fixes.
15. (γ-1 F1) When PRD names a command/event as trigger, verify the command/event actually exists in production code BEFORE wiring implementation.
16. **Candidate rule (Slice 1 anti-drift lesson)**: When a docs-vs-code drift finding is fixed, add or extend a parity guard test that decodes/validates the docs artifact against the code ground-truth when feasible. Slice 1's `TestSkillRecipeSchemaMatchesCLI` is the exemplar.

## Session Summary

Slice 2 implementation complete and ready for review.

### Slice 2 closure summary

- `docs/reconcile.md` rewritten around the v0.11 evidence/revision model: purpose paragraph, ordered raw-verdict + evidence pipeline diagram/table, ADR-025 evidence/revision schema section, synthetic JSON examples, strict-writer/lenient-reader handling, `corrupt_entries` envelope, privacy boundary, verdict/label surfaces, all four v0.11 reconcile subcommand groups, dev-only study validator sidebar, and ADR/PRD cross-reference footer.
- CLI strings and flags were grep-verified against `internal/cli/cobra.go`: `audit-retirement <slug> --json`, `confirm-upstreamed <slug> --json/--format`, `review add <slug>` required/optional flags, and `review list <slug> --json/--all`. No unsupported confirmation/rejection surfaces were documented.
- JSON examples were parsed locally and shaped against `internal/store/reconcile_evidence.go` + `internal/store/reconcile_revision.go`; examples are synthetic and use the code/ADR-025 evidence ID prefix `re_<12hex>`.
- File-novelty wording follows the implemented PRD 6 artifact reason codes (`all-new-files`, `mixed-additive`, `modifies-existing-files`, `deletes-or-renames`, `unknown`) and explains that `clean-additive` is the PRD 8 blocked-taxonomy category.
- ADR-027 F2 naming roll-in deferred: `docs/state-of-the-art/research-roadmap.md` already had unrelated unstaged edits at dispatch time, so Slice 2 avoided optional file touches and left F2 for a small separate docs edit.
- CHANGELOG v0.11.1 unreleased entry now includes a Slice 2 bullet.

## Current State

- Main docs rewrite committed as `8a2c632` (`docs(reconcile): rewrite for v0.11 evidence system`).
- Tracking/closure edits are pending in the final Slice 2 handoff/changelog commit.
- Worktree had pre-existing unrelated unstaged/untracked docs changes before Slice 2; this slice touched only `docs/reconcile.md`, `CHANGELOG.md`, and `docs/handoff/CURRENT.md`.

## Files Changed

- `docs/reconcile.md`
- `CHANGELOG.md`
- `docs/handoff/CURRENT.md`

## Test Results

- `gofmt -l .` — PASS (no output)
- `go vet ./...` — PASS (no output)
- `go build ./cmd/tpatch` — PASS
- `go test ./...` — PASS (final run cached; earlier uncached `internal/cli` 65.818s)
- `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)` — preserved after handoff update (expected `b385fe622db9926f48861105239f113e`)

## Next Steps

1. Supervisor: dispatch Slice 2 reviewers; do not dispatch reviewers from this implementer session.
2. Optional follow-up: handle ADR-027 F2 naming coordination in a separate small docs edit after resolving/isolating the pre-existing `research-roadmap.md` worktree changes.
3. After Slice 2 three-way APPROVED: archive to HISTORY, move to Slice 3 (release ops) or Slice 4 (doctor PRD).
4. Consider promoting rule 16 (parity-guard-on-drift-fix) from candidate to binding after Slice 2 reviewer feedback.

## Blockers

None.

## Context for Next Agent

- Slice 1 shipped 6 commits into `origin/main`; HEAD at time of Slice 2 kickoff: `f340cd8`. Slice 2 main docs commit is `8a2c632`.
- Slice 1 anti-drift template lives in `assets/assets_test.go` (`TestSkillRecipeSchemaMatchesCLI` extension). Slice 2 doesn't have a natural parity-guard test target (docs prose vs code) but reviewer briefs can enforce CLI-string + PRD-citation checks.
- All shipped v0.11 reconcile artifacts to cite are enumerated above.
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
