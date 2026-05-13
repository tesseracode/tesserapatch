# Current Handoff

## Active Task

- **Task ID**: `feat-skill-doc-references-user-visible`
- **Milestone**: post-v0.8.0 / pre-v0.8.1
- **Description**: Implement PRD-skill-doc-strategy / ADR-020. Remove all `docs/land.md` and `docs/reconcile.md` repo-relative references from the six shipped skill surfaces and replace them with concise inline action snippets per ac.3. Add `TestSkillDocReferencesAreSelfContained` parity guard per ac.4. No `.tpatch/` migration; no new CLI flags.
- **Status**: rev-1 complete — awaiting review
- **Assigned**: 2026-05-14 (rev-1 2026-05-14)

## Session Summary

v0.8.0 shipped: tag `v0.8.0` annotated at `29a6732` (CHANGELOG release-flip on top of tracking-close `e79c7d9`). Pushed to `origin`. M17 cluster archive landed in HISTORY at `e79c7d9`. Skill-doc-references slice landed at `ea5c954`; rev-1 follows up on three external findings:

- **F1 (Medium)** — Reconcile snippet across all six surfaces falsely claimed reconcile is "read-only for the rest of the workflow". Replaced with a mutating-operation wording that grounds in `internal/workflow/reconcile.go` (`ReconcileReapplied`) and `internal/workflow/accept.go` (shadow→tree copy on accept), and tells the user to re-run `tpatch record` afterwards.
- **F2 (Low)** — Parity guard regex `(?:^|[^A-Za-z0-9_/:])(docs/...)` missed `./docs/...md`, `../docs/...md`, `/docs/...md`. Restructured into a two-branch regex `[a-z][a-z0-9+.-]*://\S+|(?:^|[^A-Za-z0-9_])((?:\.{0,2}/)?docs/[A-Za-z0-9_./-]+\.md)\b` extracted behind a `findRepoRelativeDocsRefs` helper. Added 8 synthetic probe sub-tests (4 must-fail + 1 already-failing parens + 3 must-pass URLs).
- **F3 (Low)** — Roadmap M17 header still said "awaiting tag at `34815e8`" and Wave A row said "unreleased — bundled into v0.8.0". Both flipped to released wording referencing the actual tag SHA `29a6732` plus the cluster ship-stack tip `34815e8`.

## Current State

- `main` at `097e1e4` (sub-agent rev-1 verdict log) — pushed; rev-1 implementation at `dd6506a`. Tag `v0.8.0` at `29a6732` pushed to `origin`.
- Worktree clean. Untracked: `.dbg/` (local artifacts).
- All M17 work landed; no in-flight code regions.
- Ready surfaces for this slice (12 references total — see PRD §5.1 table):
  - `assets/skills/claude/tessera-patch/SKILL.md` (lines 68-69)
  - `assets/skills/copilot/tessera-patch/SKILL.md` (lines 43-44)
  - `assets/prompts/copilot/tessera-patch-apply.prompt.md` (lines 50-51)
  - `assets/skills/cursor/tessera-patch.mdc` (lines 40-41)
  - `assets/skills/windsurf/windsurfrules` (lines 34-35)
  - `assets/workflows/tessera-patch-generic.md` (lines 38-39)
- Parity guard target: `assets/assets_test.go` (existing `skillFiles` table at lines 12-30).

## Files Changed (this slice — landed)

### v0 (`ea5c954`)

- `assets/skills/claude/tessera-patch/SKILL.md` — lines 68-69 land/reconcile snippets inlined; line 212 dropped `docs/adrs/ADR-010-...md` pointer.
- `assets/skills/copilot/tessera-patch/SKILL.md` — lines 43-44 land/reconcile snippets inlined; trailing `docs/adrs/ADR-010-...md` pointer dropped.
- `assets/prompts/copilot/tessera-patch-apply.prompt.md` — lines 50-51 land/reconcile snippets inlined; trailing `docs/adrs/ADR-010-...md` pointer dropped.
- `assets/skills/cursor/tessera-patch.mdc` — lines 40-41 land/reconcile snippets inlined; trailing `docs/adrs/ADR-010-...md` pointer dropped.
- `assets/skills/windsurf/windsurfrules` — lines 34-35 land/reconcile snippets inlined; trailing `docs/adrs/ADR-010-...md` pointer dropped.
- `assets/workflows/tessera-patch-generic.md` — lines 38-39 land/reconcile snippets inlined; trailing `docs/adrs/ADR-010-...md` pointer dropped.
- `assets/assets_test.go` — `TestSkillDocReferencesAreSelfContained` added.

### rev-1 (this commit)

- All six surfaces: reconcile bullet `5.` rewritten to drop the false "read-only" claim; new wording calls out reconcile as a mutating operation and instructs `tpatch record` after.
- `assets/assets_test.go` — regex tightened to also catch `./`, `../`, `/` prefixed `docs/...md` paths via a two-branch URL-vs-bare alternation; `reflect` import added; `findRepoRelativeDocsRefs` helper extracted; 8 probe sub-tests added inside `TestSkillDocReferencesAreSelfContained`.
- `docs/ROADMAP.md` line 263 (M17 header) + line 279 (Wave A row) flipped from "awaiting tag" / "unreleased" to released-state wording.

### Unexpected scope expansion (flagged for reviewer)

The PRD §5.1 enumerates only the `docs/land.md` / `docs/reconcile.md` lines, but the PRD §6 / ac.4 regex (`\bdocs/[A-Za-z0-9_./-]+\.md\b`) also matches the pre-existing `Full design: \`docs/adrs/ADR-010-provider-conflict-resolver.md\`.` pointer line that ships in all six surfaces (and the equivalent inline sentence in the Claude SKILL.md "Reconcile Phase 3.5" section). With the new test enabled, those references would have failed the suite, violating ac.5. They are non-command-critical "see further" pointers, so I dropped them rather than inlining the ADR contents. If the supervisor would prefer the ADR design summary inlined instead of removed, that is a follow-up edit to the same six lines.

### Documentation update discipline (ac.7)

The implementation handoff (this section) explicitly records the rule for future contributors: when long-form `docs/*.md` content changes command-critical guidance for `land`, `reconcile`, or any other surface mentioned in the six shipped skills, the corresponding inline snippet in each of the six `skillFiles` MUST be reviewed in the same change. The `TestSkillDocReferencesAreSelfContained` parity guard prevents reintroducing repo-relative `docs/*.md` references; it does not detect drift in the inline content itself, so reviewer discipline remains required.

## Test Results

### v0 (`ea5c954`)

- `gofmt -l .` → empty (clean).
- `go build ./cmd/tpatch` → OK.
- `go test ./assets -count=1` → ok (2.338s); `TestSkillDocReferencesAreSelfContained` PASS for all six surfaces; `TestSkillParityGuard`, `TestAllSkillFilesExist`, `TestSkillRecipeSchemaMatchesCLI` continue to pass.
- `go test ./... -count=1` → all packages PASS (assets 2.338s, internal/cli 56.182s, internal/gitutil 17.978s, internal/provider 14.844s, internal/safety 5.663s, internal/store 5.119s, internal/workflow 47.549s, internal/buildinfo 1.425s).

### rev-1

- `gofmt -l .` → empty.
- `go build ./cmd/tpatch` → OK.
- `go test ./assets -run TestSkillDocReferencesAreSelfContained -count=1 -v` → PASS; 14 sub-tests (8 probes + 6 surfaces) all green; total 0.829s.
- `go test ./assets -count=1` → ok (0.848s).
- `go test ./... -count=1 -timeout 300s` → all packages PASS (assets 1.387s, buildinfo 2.103s, cli 51.590s, gitutil 14.372s, provider 15.008s, safety 4.186s, store 6.950s, workflow 40.429s; wall 55.077s).
- `rg -n 'docs/[A-Za-z0-9_./-]+\.md' assets --glob '!assets/assets_test.go'` → 0 hits.
- `rg -n 'Reconcile is read-only' assets/skills assets/prompts assets/workflows` → 0 hits. Remaining `read-only` matches all describe `tpatch verify` correctly and predate this slice.

## Next Steps

1. Implementer landed → sub-agent reviewer → external supervisor review.
2. On approval, tracking close + push.
3. Pick next backlog item (Wave D deferrals to v0.8.1 / parser dedup / a1-followup).

## Blockers

None.

## Context for Next Agent

- v0.8.0 covers all of M17 (Waves A1+A2, B, C+rev1-4, D+rev1) plus the v0.7.0-superset Wave A landed on top of the v0.7.0 baseline.
- **Wave A1+A2 are CROSS-COMMIT BOUND** (`1d6179c` ↔ `8fc2e4e`); they must be reverted as a unit if needed (`internal/gitutil/gitutil.go:111-115` references `LockState`/`LockDiagnostic` field declarations defined in A2).
- **Frozen-code regions** remain (touch only with an explicit revision brief):
  - `internal/cli/record_auto*.go` (Wave A1)
  - `internal/cli/record_collision*.go` (Wave B)
  - `internal/workflow/reconcile.go` lines ~196-236 (Wave D phase-1.5) and ~560-700 (Wave A2 lock guard)
  - `internal/workflow/patch_id_detector*.go` (Wave D)
  - `Config.PatchIDDetectorEnabled` default — `false`
  - ADR-019 trailer schema, ADR-021 carve-out scope
- The "Side Research — State-of-the-art middle pass" section below is preserved verbatim from before this commit; it is living research notes and stays in `CURRENT.md` across handoff resets.

---

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


