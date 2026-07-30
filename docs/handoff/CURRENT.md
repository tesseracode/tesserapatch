# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-wave-gamma-active-feature-session-implementation`
- **Milestone**: v0.12.0 Wave γ — implement `PRD-active-feature-session` + honor `ADR-027` D1 F3 lock.
- **Description**: Third and final wave of the v0.12.0 3-wave sequential cluster. Introduces a new `tpatch session` command group + `.tpatch/local/capture/` local-buffer storage lane + `tpatch init` `.gitignore` amendment that together satisfy ADR-027 D1's F3 conditional. The six-mandate refusal contract at PRD-active-feature-session §4 D6 is the ENTIRE safety margin — refusal-path test coverage is first-class scope, not an afterthought.
- **Status**: Dispatched 2026-07-30.
- **Assigned**: 2026-07-30.

## Wave γ scope (locked)

**Read these first, in order**:

1. `docs/prds/PRD-active-feature-session.md` (500 lines, 25 acceptance criteria) — the spec. D1 through D19 govern lifecycle, storage, promotion, CLI, and privacy.
2. `docs/adrs/ADR-027-capture-context-privacy-boundary.md` — the conditional lock. D1 permits the worktree path **only** if "the writer MUST verify that path is ignored before the first write and MUST refuse rather than risk accidental commit." Wave γ's `tpatch init` .gitignore amendment + `tpatch session start` verification are the two halves of that contract.
3. `docs/adrs/ADR-028-supersession-edge-model.md` — Wave α edge model. Sessions must not collide with supersession semantics (session belongs to a feature, supersession is between features).
4. Wave α + Wave β diffs (`7081c62..0072fb5`) for pattern reference — especially the deterministic slug ordering + parity guard + severity-first label emission patterns.
5. `docs/prds/PRD-tpatch-doctor.md` §6 (referenced by PRD §0 Related as the acceptance precedent) — the D3 `--fix` refusal fixtures from doctor Wave β are the TEMPLATE for D6 refusal-path testing.

**What ships**:

- New `tpatch session` command group with subcommands `start`, `stop`, `list`, `summarize`, `purge` (D13, D14).
- `.tpatch/local/capture/<feature-slug>/<session-id>/` storage lane (D5). `<session-id>` is content-addressed per D3.
- Local state machine (D4): `idle → active → closed → purged`. Only one active session per feature at a time.
- `tpatch init` amendment (D6 mandate 1): appends `.tpatch/local/` to `.gitignore` at init. Refusal with the exact rule printed if `.gitignore` cannot be edited (D6 mandate 2).
- `tpatch session start` verification (D6 mandate 3): before ANY write, verify the concrete resolved path is effectively ignored (D6 mandate 5 — effective ignore check via `git check-ignore` or equivalent, not textual line matching).
- Refusal-class error surfaces when Git is unavailable OR the path is not effectively ignored (D6 mandate 4).
- `pre-PRD-workspace` fallback path defined for the transitional case (D6 mandate 6).
- Explicit session-to-committed-summary promotion boundary (D9, D10, D11): promotion is opt-in, writes to per-feature artifact under `.tpatch/features/<slug>/session-summary.md`, and passes through the D11 redaction contract.
- Redaction contract: raw session bodies NEVER cross the local→committed boundary. Only redacted summaries (D17).
- New `record` flags for opt-in record-time session close (D15).
- Cross-feature isolation (D18): a session for feature A cannot see feature B's local buffer.
- Provider-assisted carve-out (D19): provider prompts may reference redacted summaries but never raw local buffers.
- CHANGELOG `## v0.12.0 — TBD` amendment: append `### Wave γ` subsection (Wave α + Wave β subsections untouched).
- Status flip: `PRD-active-feature-session` from `Proposed` → `Accepted`.

**What does NOT ship in Wave γ**:

- `PRD-record-context-summary`, `PRD-agent-event-log`, `PRD-ide-capture-hooks`, `PRD-git-hook-capture-guards`, `ADR-capture-metadata-branch` — all listed as `Blocks` in the PRD header and deferred to post-v0.12.0.
- Non-local (committed) session storage — explicitly rejected by D7.
- Session sharing between features or across worktrees — D18 boundary.

## Wave γ slice plan (locked)

1. **Slice 1 — Storage lane + .gitignore contract** (foundation, D5 + D6 six mandates).
   - Add `.tpatch/local/capture/` path constant.
   - Amend `tpatch init` to append `.tpatch/local/` to `.gitignore` (create `.gitignore` if absent).
   - Implement effective-ignore verification helper (git check-ignore semantics, NOT textual match).
   - Refusal error type + actionable message enumerating all six D6 mandates.
   - **Refusal-path test coverage FIRST-CLASS**: no-git case, unwritable-gitignore case, effective-ignore-fails case, path-not-under-worktree case. Doctor Wave β D3 refusal fixtures are the template.
   - Deterministic content-addressed session ID (D3).

2. **Slice 2 — Session command group + lifecycle** (D1, D2, D4, D13, D14).
   - New `tpatch session` cobra group with `start`, `stop`, `list`, `summarize`, `purge` subcommands.
   - Local state machine (`idle → active → closed → purged`).
   - Per-feature single-active-session invariant (validation.go-style write-time rejection).
   - Session identity + storage layout under `.tpatch/local/capture/`.
   - Regression tests for state transitions + single-active invariant.

3. **Slice 3 — Promotion boundary + redaction contract** (D9, D10, D11, D16, D17).
   - `tpatch session summarize --promote` (or equivalent) writes redacted summary to `.tpatch/features/<slug>/session-summary.md`.
   - Redaction contract enforced at the promotion boundary; raw bodies never cross.
   - D11 redaction rules explicit + tested.
   - Regression tests for redaction correctness + boundary invariant.

4. **Slice 4 — Record flags + cross-feature isolation** (D15, D18).
   - `record --close-session` (or equivalent per D15) for opt-in record-time close.
   - Cross-feature isolation: session for feature A cannot observe feature B's buffer.
   - Regression tests for both directions.

5. **Slice 5 — CHANGELOG + PRD flip + ROADMAP** (paperwork).
   - Append `### Wave γ` subsection under `## v0.12.0 — TBD` (Wave α + Wave β subsections untouched).
   - Flip `PRD-active-feature-session` `Proposed → Accepted`.
   - Update ROADMAP v0.12.0 Wave γ status marker.

## Wave γ validation gates

Same as Wave α/β (gofmt, vet, build, full test suite). Baseline: **826 top-level PASS** at Wave β acceptance. Wave γ total MUST be ≥ 826 + 15-25 (six D6 mandates + lifecycle + promotion + isolation + record flags all need regression coverage; refusal-path coverage is first-class).

**Additional gates**:
- Refusal-path regression coverage is REQUIRED — doctor Wave β D3 `--fix` refusal fixtures are the template. Missing refusal fixtures = BLOCKING at rev-0 review.
- Effective-ignore verification MUST use git semantics (`git check-ignore`), not textual `.gitignore` line matching (D6 mandate 5).
- Parity guard test MUST update in Slice 2 if any new command-mentioning skill surface changes (Rule 15 will flag new `tpatch session` in shipped assets).
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.

## Carry-forward dispatch rules (20 binding, all apply)

All 20 rules from prior waves. Highlights for Wave γ:

- **Rule 15**: NEW `tpatch session` command group added — Rule 15 explicitly expects new commands here per D13 ("This PRD proposes a new command group; it does not exist in production today"). Every new command MUST be added to the 6 shipped skill assets in the SAME commit (Slice 2 anti-drift lesson).
- **Rule 17 (totality claims)**: When updating verify help, `tpatch --help`, or any "supported commands" enumeration, include the new `session` group. Wave β F1 (user-external) was a Rule 17 regression on the exact same file (`internal/cli/verify.go`). Do not repeat.
- **Rule 18**: every commit MUST carry parseable `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.
- **Rule 19**: `tpatch init` is a shipped surface. The `.gitignore` amendment behavior change MUST cite ADR-027 D1 + PRD §4 D6 in the commit body.
- **Rule 20**: reproduce empirically — the six D6 mandate refusal paths MUST be reproduced via detached worktree fixtures. This is the safety-margin class where empirical reproduction is the ENTIRE gate.

## Two-opinion protocol scoreboard

**20/20** final-acceptance three-way concurrence going into Wave γ. Wave β rev-0 was the FIRST supervisor-external miss (external APPROVED where PRD §7.2 explicitly answered the question internal caught). Adjudication lesson preserved: **cross-reference PRD before concluding ADR silence**.

## Recurring pattern flags (address at Wave γ close or post-v0.12.0)

- **F1 handoff Status stale** (recurring across Streams A+B + Wave α + Wave β): The Status field of CURRENT.md should flip from "Dispatched" to "Review" when rev-1 lands and reviewers are working, then to "Awaiting user-external" when internal+supervisor-external return. Systematic wave-close gap. Deferred to AGENTS.md closure checklist post-v0.12.0.
- **CHANGELOG graduation**: `## v0.12.0 — TBD` gets a dated header only after Wave γ acceptance ships v0.12.0.

## Session Summary

Wave β accepted 2026-07-30 at three-way concurrence (20th scoreboard entry). Consolidation fold-in of F1 (MEDIUM) + F2 (LOW) + F-INT-β-r1-1 (LOW) landed in the same commit as the LOG + HISTORY + ROADMAP updates and this CURRENT.md reset. Wave γ dispatched 2026-07-30.

## Files Changed (Wave β consolidation)

- `internal/cli/verify.go` — F1 fix: V0-V9 → V0-V10 in help text + 2 doc comments; check-count 10 → 11.
- `internal/workflow/verify.go` — F1 + F2 fix: V0-V9 → V0-V10 in doc header; "remaining nine" → "remaining ten" ×2.
- `docs/ROADMAP.md` — F-INT-β-r1-1 fix: Slice 3 description reflects R1 warn-class revert with ADR-029 D6 + PRD §7.2 citation; Wave β status flipped ⬜/🚧 → ✅ ACCEPTED; Wave γ ⬜ → 🚧 dispatched.
- `docs/supervisor/LOG.md` — Wave β user-external verdict + supervisor decision entries prepended.
- `docs/handoff/HISTORY.md` — Wave β archive prepended.
- `docs/handoff/CURRENT.md` — this reset.

## Test Results (post-consolidation)

- `gofmt -l .` — empty.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean.
- Full test suite — pass (matches rev-1 baseline; consolidation was doc + help-text only).
- Side Research md5 — `b385fe622db9926f48861105239f113e` preserved.

## Next Steps

1. Wave γ implementer executes Slices 1–5 per the locked plan.
2. Rev-0 hand-back triggers dual review (internal + supervisor-external).
3. On any BLOCKING/HIGH finding → rev-1 fold-in cycle.
4. On three-way APPROVED → user-external pass → v0.12.0 ship (CHANGELOG graduation to dated header).

## Blockers

None.

## Context for Next Agent

- HEAD at Wave γ dispatch: Wave β consolidation commit (this handoff + all doc fixes). All prior Wave β commits (rev-0 `639efb2..0d25e75` + rev-0 handoff `7689c39` + rev-0 dual review `9e8d2ba` + `fc261d1` + rev-1 brief `e8d351f` + rev-1 `ec98499..0072fb5` + rev-1 dual review `9eb2fcf` + `63d8650`) pushed to `origin/main` at the same time.
- Wave α + Wave β acceptance on `main` — Wave γ freely uses `store.DependencyKindSupersedes`, `store.ErrMultipleActiveSuperseders`, `IsFeatureSuperseded`, `PreimageHash *string`, `writefile_safety.go` primitives without re-export or refactor.
- 20 binding carry-forward rules; every Wave γ commit MUST carry the Copilot co-author trailer.
- **Non-obvious decisions to know before implementation**:
  - PRD §4 D6 has SIX mandates all of which must be satisfied. Missing any one → refusal-class rejection at Wave γ rev-0 review. The mandates are: (1) `.gitignore` rule at init, (2) refusal-with-rule-printed if `.gitignore` cannot be edited, (3) `session start`-time path verification, (4) refusal when Git unavailable OR path not ignored, (5) EFFECTIVE ignore checking (not textual), (6) defined pre-PRD-workspace fallback path.
  - Doctor Wave β D3 `--fix` refusal fixtures are the PATTERN for D6 refusal-path testing. If you find yourself writing a refusal path without a corresponding detached-worktree regression fixture, stop and add the fixture first.
  - `.tpatch/local/capture/` sits INSIDE the directory users are explicitly told to commit — this is the higher-risk of ADR-027 D1's three options, chosen because it makes session state travel with the branch. The six-mandate refusal contract is the ENTIRE safety margin. Take it seriously.
  - PRD §4.5 supersession interaction: sessions belong to features. Superseded features' sessions are still promoted (they still have history worth preserving); only the reconcile suppression from Wave α applies to the FEATURE, not to its session history.
  - Session promotion is EXPLICIT and OPT-IN per D9. Do not auto-promote on session stop; that is a follow-up PRD (`PRD-record-context-summary`).
  - The D11 redaction contract is where the local→committed boundary lives. Raw bodies MUST NOT cross. If you find yourself copying anything unredacted across that boundary, you are violating the boundary.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e` (preserved through Wave β consolidation and this reset).

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
