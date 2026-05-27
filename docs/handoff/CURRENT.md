# Current Handoff

## Active Task

- **Task ID**: `wp003-wave-alpha-prd1-prd6-impl-rev2`
- **Milestone**: WP-003 — Reconcile safety & middle-pass (T56 cluster), Wave α revision 2
- **Description**: Address rev-1 external NEEDS REVISION finding F3. Add the reader-side CLI surface for reconcile evidence so PRD 1 §4 ("JSON output includes the latest evidence bundle or a reference to the artifact"; "default human output gains a short evidence hint") and PRD 6 §6.3 ("File novelty evidence is available in JSON output") are actually met. Rev-0 and rev-1 only landed the writer + warning paths; the CLI never surfaces evidence.
- **Status**: Not Started (rev-2 implementer pending dispatch).
- **Assigned**: 2026-05-26.
- **Prior rev-1**: Internal APPROVED + supervisor-external APPROVED, but user's parallel external NEEDS REVISION on F3 (commits `4fa1394..7c72323`). F1 (production novelty integration) and F2 (malformed-evidence warning) are fully fixed and independently verified. Root cause of rev-1 miss: supervisor-external brief was scoped to verifying F1+F2, not sweeping ALL PRD §6 acceptance criteria. Carry-forward: external reviewer briefs MUST include a "scan all PRD §6 acceptance criteria, not just the rev's stated findings" sweep.

## Session Summary

Rev-1 shipped at `7c72323` and the supervisor reconciled both Wave α rev-1 verdicts:

- Supervisor-external (mine) — APPROVED zero findings (verified F1+F2 only).
- User's parallel external — NEEDS REVISION on F3 (HIGH, NEW).

F3 verification (independently confirmed by supervisor before dispatching rev-2):

- PRD 1 §4 literal text in `docs/prds/PRD-reconcile-verdict-evidence.md:175-186` requires both human evidence hint + JSON evidence bundle/reference.
- PRD 6 §6.3 in `docs/prds/PRD-reconcile-file-novelty-classifier.md:140-148` requires file novelty evidence in JSON output.
- `grep -rn "LoadReconcileEvidence" --include='*.go' .` shows ZERO non-test, non-writer-internal usages. The only call sites are the function definition (`internal/store/reconcile_evidence.go:212`) and the writer's malformed-precheck (`internal/store/reconcile_evidence.go:141`).
- `grep -rn "evidence\|Evidence" internal/cli/*.go` shows only an unrelated comment in `reconcile_check_applied.go:28`. No CLI command reads or surfaces evidence.

Both reader-side requirements are still unmet at HEAD.

## Current State

- Writer surface: complete and validated (rev-0 + rev-1).
- Reader surface: completely missing (this rev's scope).
- ADR-025 D1–D13 schema unchanged.
- No lifecycle states added.
- No config flags added.
- Verdict semantics unchanged.

## Files Changed

(rev-2 not yet started; expected touch points listed in Next Steps.)

## Test Results

Rev-1 baseline (still green at HEAD `7c72323`):

- `go build ./cmd/tpatch`: clean.
- `go test ./internal/workflow/... ./internal/store/...`: 31/31 targeted tests pass (including rev-1 integration tests for malformed-evidence warning and file-novelty disk writes).
- `gofmt -l .`: clean.

## Next Steps

Rev-2 implementer scope (binding):

1. **JSON surface (PRD 1 §4 + PRD 6 §6.3)**: extend `ReconcileResult` in `internal/workflow/reconcile.go:19` with an evidence-bundle field. Two acceptable shapes:
   - **Option A (preferred)**: inline the just-written attempts as `Evidence []store.ReconcileEvidence \`json:"evidence,omitempty"\`` populated immediately after `saveReconcileArtifacts` runs, restricted to the entries written in THIS reconcile invocation.
   - **Option B (fallback)**: a single `EvidenceArtifact string \`json:"evidence_artifact,omitempty"\`` containing the repo-relative path to `reconcile-evidence.jsonl`.
   - The field MUST be `omitempty` for byte-identity vs pre-rev-2 fixtures when no evidence was written.
2. **Human surface (PRD 1 §4)**: add a short evidence hint to the human reconcile output. Format must match the PRD 1 §4 example pattern (e.g., `evidence: phase-2 recipe-operation-match` for phase evidence; `evidence: file-novelty mixed-additive` for novelty evidence). Hint must appear in `tpatch reconcile` default output, after the verdict line, when evidence was just written.
3. **Reader integration**: add at least one production (non-test, non-writer-internal) call to `store.LoadReconcileEvidence` OR populate the inline field directly from the writer return value if Option A. Either way the production tree must have a real reader-side dependency on the evidence artifact.
4. **CLI command target**: investigate `internal/cli/cobra.go` reconcile command output path (`reconcileCmd()` around `:1701`) and any `--format json` handling. The `tpatch status` command JSON SHOULD also include the latest evidence artifact path (`evidence_artifact` field reference) so operators can locate the audit trail. Confirm whether status JSON should embed entries inline — PRD 1 §5 says `status.json` remains current-truth, so a path reference is the safer minimum.
5. **Privacy (D10)**: any new user-facing output (human or JSON) must NOT contain source bodies, prompts, transcripts, vectors, or embeddings. Evidence entries themselves already enforce this at write time; rev-2 must preserve it at read time.
6. **Tests required**:
   - End-to-end test running a real reconcile, then asserting:
     - The `ReconcileResult` JSON includes phase evidence + file-novelty entries (or artifact-path reference).
     - The human output contains an `evidence:` hint line.
     - A `status` JSON snapshot includes `evidence_artifact` (or equivalent) when evidence exists.
   - Negative test: when no evidence was written (e.g., reconcile early-exits before `saveReconcileArtifacts`), evidence field is `omitempty`'d out — byte-identical to pre-rev-2 fixtures.
   - Privacy assertion: the rendered human + JSON output is grep-clean for source content (mirror the D10 assertion style from existing rev-1 tests).
7. **HARD constraints** (no drift):
   - No ADR-025 schema/enum drift.
   - No new lifecycle state.
   - No new config flag.
   - Verdict semantics unchanged; evidence is diagnostic only.
   - `--format json` byte-identity preserved when evidence is absent.

Validation gates before declaring done:

- `gofmt -l .` (direct, not piped — see standing process note 4).
- `go vet ./...`.
- `go build ./cmd/tpatch`.
- `go test ./...` (full suite).

Commit cadence: 3-4 commits. Mandatory `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer on each.

## Blockers

None.

## Context for Next Agent

- Two-opinion external review protocol has caught regressions in EVERY Wave α revision. User's parallel external is dispatched in addition to supervisor's. Plan accordingly: rev-2 will be reviewed twice externally after internal review.
- The rev-1 integration tests in `internal/workflow/reconcile_evidence_integration_test.go` are excellent templates for end-to-end coverage. Reuse the harness pattern; do not retest the writer side from CLI tests.
- `ReconcileResult` already has multiple `omitempty` fields with byte-identity contracts (`Labels` is documented at `:43` as load-bearing for flag-off byte-identity). Follow that pattern.
- Do NOT modify `status.json` schema or `ReconcileSummary` — adding `evidence_artifact` to the runtime CLI JSON payload for `tpatch status` is OK; modifying the persisted `ReconcileSummary` is not (ADR-025 schema lock).

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
