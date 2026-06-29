# Current Handoff

## Active Task

- **Task ID**: `wp003-wave-gamma-1-prd4-prd5-prd8-impl`
- **Milestone**: WP-003 Wave γ slice 1 — PRDs 4 (`reconcile-retirement-state-audit`), 5 (`reconcile-study-validation`), 8 (`reconcile-blocked-verdict-taxonomy`). PRD 9 (`reconcile-path-restructure-detector`) sequenced separately in γ-2 (depends on PRD 8).
- **Description**: Wave γ-1 ships three independent surfaces under ADR-025's existing evidence schema (no new cluster ADR). PRD 4: read-only `tpatch reconcile audit-retirement <slug>` + auto-run after `confirm-upstreamed`. PRD 5: dev-only `internal/tools/` case-study validator (stdlib-only, no public CLI surface). PRD 8: blocked-category enrichment via evidence metadata (no new lifecycle state, no new persisted enum). Wave β acceptance is the gate — all three γ-1 PRDs depend on Wave β surfaces.
- **Status**: Review (γ-1 implementation complete; gates clean 2026-06-29).
- **Assigned**: 2026-06-28.

## γ-1 binding scope per PRD

### PRD 4 — `reconcile-retirement-state-audit` (`docs/prds/PRD-reconcile-retirement-state-audit.md`)

- New command: `tpatch reconcile audit-retirement <slug> [--json]` — read-only, no mutation.
- Audit checks (§4, 1–5):
  1. Feature state + raw/review evidence agree retirement was confirmed.
  2. `Dependency.SatisfiedBy` SHAs reachable from current HEAD.
  3. Child features derive expected labels after parent retirement.
  4. `dependent-broken` labels justified or clearable by current state.
  5. Feature has revision-pass log entry for retirement action.
- Auto-run after `confirm-upstreamed`: prints findings + appends `cleanup-needed` revision-pass entries (via ADR-025 revision schema) but never mutates dependency or status metadata.
- §6 acceptance (6 criteria): stale SHA reporting, child identification, no mutation, stable JSON, auto-run, no v1 fixer.
- §5: reuse existing label composition; do NOT persist new label fields.

### PRD 5 — `reconcile-study-validation` (`docs/prds/PRD-reconcile-study-validation.md`)

- Dev-only `internal/tools/` package (e.g., `internal/tools/studyvalidator/`). NOT in `SPEC.md`. Optional maintainer-only command is acceptable but is not part of the public CLI surface.
- Validates a case-study folder containing `study.json`, `features.jsonl`, `hunks.jsonl`, `patches.jsonl`, `metrics.json`, `summary.md`.
- §4 validation rules (1–6):
  1. Every JSON/JSONL record parses.
  2. `study_id` consistent across all files.
  3. Feature counts in `study.json` match `features.jsonl` rows.
  4. Aggregate ground-truth counts in `metrics.json` match record-level `ground_truth` values.
  5. Every false-positive/false-negative has revision-pass entry OR documented `local-notes.md` reference.
  6. Raw verdict counts not compared directly to final state counts unless phase declared.
- §5: stdlib-only; no target-repo access required; warnings for prose-only discrepancies; parse failures + count contradictions are errors.
- §6 acceptance (6 criteria): filename+line on malformed records; count mismatch detection; raw/post-review/final distinction; runs on t3code study; dev-only path; `local-notes.md` warn-for-old / error-for-new.

### PRD 8 — `reconcile-blocked-verdict-taxonomy` (`docs/prds/PRD-reconcile-blocked-verdict-taxonomy.md`)

- 8 categories with deterministic precedence: `dependency-blocked > validation-blocked > target-deleted > structural-conflict > edit-overlap > shifted-context > clean-additive > unknown-blocked`.
- §5 implementation: store category as **evidence metadata, NOT as a new lifecycle state**. Programmatic decisions read raw outcome + labels separately. Deterministic + sorted when multiple apply; v1 exposes primary category + secondary evidence.
- Human output: `<slug>: blocked (<category>)\n  evidence: ...\n  next: <recommended_action>`.
- JSON: `{"outcome": "blocked", "blocked_category": "...", "recommended_action": "..."}` — raw `outcome` MUST remain `blocked` (backward-compat).
- §6 acceptance (5 criteria): enriched output when evidence exists; `unknown-blocked` for insufficient evidence; JSON exposes raw outcome + category + recommended action; existing status files remain readable without category evidence; multi-category precedence with secondary evidence.

## γ-1 hard constraints (binding)

1. **No new lifecycle states** (no `FeatureState` additions). PRD 8 explicitly says blocked category is evidence metadata, not a state.
2. **No new persisted-schema fields outside ADR-025 authorizations**. PRD 8 category goes into existing evidence record fields (`reason_code`, `matched_operations`, or similar) — NOT a new top-level column on `ReconcileSummary` or `ReconcileEvidence` unless an ADR clause already authorizes it. If a new field is genuinely needed, draft a minor ADR-025 amendment first; do not silently extend the schema.
3. **No new public CLI surface for PRD 5**. Dev-only `internal/tools/` only. Maintainer command allowed but must NOT appear in `assets/skills/` parity guard or `SPEC.md`.
4. **PRD 4 audit is read-only**. No mutation paths. Auto-run after `confirm-upstreamed` appends revision-pass `cleanup-needed` entries via the existing ADR-025 revision writer — do NOT introduce a new persisted artifact.
5. **PRD 8 backward-compat**: existing status.json files with `outcome=blocked` and no `blocked_category` field MUST continue to load and roundtrip. Add a backward-compat test (Wave β F5 lesson template).
6. **D10 privacy**: no source bodies / transcripts / prompts / vectors in persisted artifacts (evidence, revision, audit findings). Privacy tests MUST seed secrets into title/slug/path metadata, NOT just file content (Wave β F3 lesson).
7. **D11 malformed handling**: PRD 5 validator's malformed-record reporting must use line-number + filename pattern (mirror Wave β F2 lenient-loader UX).
8. **ADR-024 / `patch-generations.json` UNTOUCHED**.
9. **Side Research md5 preserved**: `b385fe622db9926f48861105239f113e`.
10. **Commit trailer**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` on every commit.

## γ-1 reviewer-brief preparation (carry-forward 8, 9, 10, 11, 12)

Reviewers (internal + both externals) will be briefed with these binding sweep rules:

- (Carry-forward 8) Per-PRD §6 display contracts are binding test contracts AND production-behavior contracts. PRD 4 example output line "`feature state: upstream_merged`" and PRD 8 example "`blocked (structural-conflict)`" are display-string contracts.
- (Carry-forward 9) Behavior-implemented vs behavior-tested distinction. Reviewers MUST read production code FIRST. Sweep every PRD §6 line for "does the production code path exist?" before "is it tested?"
- (Carry-forward 10) State-mutation tests reload from disk. PRD 4 explicitly says no mutation — reviewer brief must confirm `LoadFeatureStatus` reload after `audit-retirement` shows zero state delta.
- (Carry-forward 11) Cross-artifact linkages (audit findings → revision-pass log) must be verified by loading the persisted JSONL.
- (Carry-forward 12) Privacy tests seed metadata vectors (title/slug/path).

## Process

## Wave β closure summary (for reference)

- **Three-way reviewers APPROVED**: internal (`f25dd83`), supervisor-external (`cc19bff`), user-external (parallel, 2026-06-28).
- **Findings closed**: F1 (display contract) HIGH BLOCKING; F2 (corrupt_entries envelope) HIGH BLOCKING; F3 (privacy seeding); F4 (nearby-window=3 JSON); F5 (backward-compat); F6 (status disk reload); F7 (revision evidence linkage).
- **Per-PRD §6**: PRD 2 8/8 · PRD 3 7/7 · PRD 7 5/5 MET.
- **Ship stack on `origin/main`**: `e45ccdc` `34b2bba` `1e99a9f` `d8774a7` `56791b5` `5280f5d` `bd5bf22`.
- **Process lessons recorded** in `docs/supervisor/LOG.md` under "Decision — WP-003 Wave β rev-1 — supervisor — 2026-06-28" (7 lessons, all binding for Wave γ briefs).

## Wave γ unlocked (next implementation block, per WP-003 §6)

PRDs gated on Wave β acceptance — all four are now unblocked:

1. `docs/prds/PRD-reconcile-retirement-state-audit.md` — PRD 4 (depends on PRD 2).
2. `docs/prds/PRD-reconcile-study-validation.md` — PRD 5 (depends on PRD 3).
3. `docs/prds/PRD-reconcile-blocked-verdict-taxonomy.md` — PRD 8 (depends on PRD 7).
4. `docs/prds/PRD-reconcile-path-restructure-detector.md` — PRD 9 (depends on PRD 8).

Per WP-003 §6 the wave is parallel-safe after Wave β acceptance. Note PRD 9 has an intra-wave dependency on PRD 8 — sequence those two within Wave γ.

ADR-025 already covers the entire reconcile-evidence/revision cluster — no new cluster ADR required for Wave γ unless a structural surface (e.g., new persisted artifact) is introduced.

## Open decision for supervisor

Pick one before dispatching implementer:

**Option A** — Ship interim release v0.10.1 bundling WP-003 Wave α + Wave β. Pros: clean release boundary, CHANGELOG hygiene before Wave γ scope grows. Cons: extra release cycle.

**Option B** — Continue directly to Wave γ implementation; release v0.10.1 (or v0.11.0) when Wave γ acceptance lands. Pros: faster forward progress on the WP-003 cluster. Cons: bigger release surface, more for one CHANGELOG entry to cover.

## Carry-forward dispatch rules (binding for all Wave γ briefs)

1. (Wave α) Briefs MUST self-audit against binding ADRs.
2. (Wave α) Briefs naming policy ADRs MUST enumerate config-flag opt-out contracts.
3. (Wave α) Internal reviewer checklist MUST include flag-off counter-scenarios.
4. (Wave α) `gofmt -l .` MUST be run directly, never piped through grep (returns exit 1 on empty input).
5. (Wave α) Drift self-audit during release prep.
6. (Wave α rev-0) Briefs MUST NOT contain escape hatches that override PRD acceptance.
7. (Wave α rev-1) External reviewer briefs MUST sweep ALL PRD §6 acceptance criteria.
8. (Wave β F1) Briefs MUST enumerate per-PRD §6 display-name contracts as binding production-behavior + test contracts.
9. (Wave β F2) Reviewer briefs MUST distinguish "behavior implemented" from "behavior tested". Read production code FIRST, then verify tests, asking "does this acceptance criterion actually have a code path?" Severity must reflect production-gap vs test-gap.
10. (Wave β F6) State-mutation contracts MUST be verified by reloading from disk (`store.LoadStatus` / `LoadFeatureStatus`), not by inspecting runtime `result` values.
11. (Wave β F7) Cross-artifact linkage contracts MUST be verified by loading the persisted JSONL/JSON, not by inspecting runtime structs.
12. (Wave β F3) Privacy tests MUST seed secrets into plausible exfiltration vectors (title / slug / path metadata) — NOT just file content. Recurring anti-pattern from Wave α F3 and Wave β F3 — internal reviewer checklist must include explicit privacy-vector audit.
13. (Wave β schema-lock) Briefs MUST say "no persisted-schema additions outside what binding ADRs explicitly authorize" + list which fields/clauses are pre-authorized — NOT "schema LOCKED" (over-broad in Wave β rev-0).
14. Two-opinion external review protocol (supervisor + user-parallel) remains MANDATORY. Caught HIGH BLOCKER findings missed by single review in Wave α rev-0/rev-1 and Wave β rev-0. Confirmed concurrence in Wave β rev-1 — protocol earned its keep four times.

## Session Summary

WP-003 Wave γ-1 implementation completed for PRDs 4, 5, and 8.

### γ-1 closure summary — PRD 4 (`reconcile-retirement-state-audit`)

1. Stale `satisfied_by` / base SHA reporting: `internal/workflow/retirement_audit.go` (`AuditRetirement` reachable-SHA checks); tests `internal/workflow/retirement_audit_test.go`.
2. Child features affected by retired parent: `AuditRetirement` scans `DependsOn`; tests `retirement_audit_test.go` and `internal/cli/audit_retirement_test.go`.
3. Read-only audit: `AuditRetirement` only loads status/list/revisions; CLI test reloads status from disk in `audit_retirement_test.go`.
4. Stable JSON: `tpatch reconcile audit-retirement <slug> --json` marshals `RetirementAuditReport`; covered by `audit_retirement_test.go`.
5. Auto-run after `confirm-upstreamed`: `internal/cli/cobra.go` invokes audit and prints findings; cleanup entries appended via `AppendRetirementCleanupRevisions`; covered by `audit_retirement_test.go`.
6. No v1 fixer/mutation path: no dependency/status mutation in audit; auto-run appends only ADR-025 revision-pass entries.

### γ-1 closure summary — PRD 5 (`reconcile-study-validation`)

1. Malformed JSON/JSONL filename + 1-indexed line: `internal/tools/studyvalidator/validator.go`; test `validator_test.go`.
2. Aggregate mismatch detection: metrics/study/count checks in validator; test `validator_test.go`.
3. Raw/post-review/final distinction: phase warnings and post-review handling in validator; test `validator_test.go`.
4. t3code study fixture coverage: `TestValidateRunsOnT3CodeStudyArtifacts`.
5. Dev-only surface: package/binary under `internal/tools/studyvalidator`; no `cmd/tpatch`, `SPEC.md`, or skill asset registration.
6. Missing `local-notes.md`: old-study warning / new-study error in validator; test `validator_test.go`.

### γ-1 closure summary — PRD 8 (`reconcile-blocked-verdict-taxonomy`)

1. Deterministic blocked enrichment: `internal/workflow/blocked_taxonomy.go`; CLI rendering in `internal/cli/cobra.go`; tests `blocked_taxonomy_test.go`, `blocked_taxonomy_cli_test.go`.
2. Unknown fallback: `unknown-blocked` classifier branch; unit test coverage.
3. JSON output: runtime-only `ReconcileResult.BlockedCategory` / `RecommendedAction`; JSON test in `blocked_taxonomy_cli_test.go` keeps raw `outcome=blocked`.
4. Backward compatibility: no `ReconcileSummary` schema field added; `internal/store/reconcile_backward_compat_test.go` roundtrips old blocked status.
5. Precedence + secondary evidence: classifier precedence list and sorted secondary evidence; unit test coverage.

## Files Changed (γ-1 implementation)

- `internal/workflow/blocked_taxonomy.go`, `internal/workflow/blocked_taxonomy_test.go`
- `internal/workflow/retirement_audit.go`, `internal/workflow/retirement_audit_test.go`
- `internal/workflow/reconcile.go`
- `internal/cli/cobra.go`, `internal/cli/audit_retirement_test.go`, `internal/cli/blocked_taxonomy_cli_test.go`
- `internal/store/reconcile_backward_compat_test.go`
- `internal/tools/studyvalidator/validator.go`, `internal/tools/studyvalidator/validator_test.go`, `internal/tools/studyvalidator/cmd/studyvalidate/main.go`
- `assets/assets_test.go` plus six shipped skill/prompt/workflow surfaces for `tpatch reconcile audit-retirement` guidance

## Test Results (γ-1 final)

- `gofmt -l .` — clean
- `go vet ./...` — clean
- `go build ./cmd/tpatch` — clean
- `go test ./...` — green
- Side Research md5 — `b385fe622db9926f48861105239f113e`

## Next Steps

1. Supervisor: dispatch internal + external reviewers for WP-003 Wave γ-1.
2. Reviewers: sweep all 17 PRD §6 criteria using the closure summary above and the binding hard constraints.
3. Supervisor: after approval, archive this handoff and sequence PRD 9 / Wave γ-2.

## Blockers

None.

## Context for Next Agent

- PRD 8 uses ADR-025-authorized evidence metadata (`reason_code`, `matched_operations`) and runtime-only result fields; no persisted `ReconcileSummary` category field was added.
- PRD 4 audit is read-only; only the auto-run path appends `cleanup-needed` revision-pass entries through `AppendReconcileRevision`.
- PRD 5 remains dev-only under `internal/tools/`; the maintainer binary is not registered as a public `tpatch` subcommand.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`. Verify before/after any CURRENT.md edits: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.

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
