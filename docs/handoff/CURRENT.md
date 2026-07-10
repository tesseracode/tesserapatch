# Current Handoff

## Active Task

- **Task ID**: `wp003-wave-gamma-2-prd9-impl`
- **Milestone**: WP-003 Wave γ-2 — PRD 9 `reconcile-path-restructure-detector`. Final PRD in WP-003; γ-2 closes out the entire reconcile safety cluster.
- **Description**: Ship path-restructure detector as a new evidence pass under ADR-025 (`evidence_kind=path-restructure`, likely under D13's blocked-classification family or a new evidence kind if D13 doesn't cover — verify FIRST). Detector reads Git name-status between base and target upstream, applies prefix-split / prefix-move thresholds, emits candidate prefixes + affected paths. Integrates with PRD 8 blocked-taxonomy: prefix restructure evidence upgrades generic `blocked` to `structural-conflict` or `target-deleted` category. No provider integration required.
- **Status**: In Progress (γ-2 implementer dispatch pending).
- **Assigned**: 2026-07-10.

## Wave γ-1 closure summary (for reference)

- **Three-way APPROVED**: internal (`dc476c8`), supervisor-external (`56c0320`), user-external (2026-07-10).
- **Ship stack on `origin/main`**: `f50e09b` (rev-0) → `cb61032` `98b3256` `c409bcd` (rev-1) → LOG updates `971b251` `eb066ac` `a408e58` `dc476c8` `56c0320`.
- **Per-PRD §6**: PRD 4 6/6 · PRD 5 6/6 · PRD 8 5/5.
- **13 hard constraints preserved**. New process rule 15 added.
- **Full snapshot** in `docs/handoff/HISTORY.md` under "Archived 2026-07-10".

## γ-2 binding scope — close all §6 (6 criteria)

Read `docs/prds/PRD-reconcile-path-restructure-detector.md` verbatim. Key contracts:

### §3 Detector Contract
- Input: feature patch paths, upstream diff name-status between base and target, optional Git rename/copy detection.
- Output evidence (JSON schema at PRD line 92-104):
  ```json
  {
    "evidence_kind": "path-restructure",
    "classification": "prefix-split | prefix-move | target-deleted | mixed | none | unknown",
    "old_prefix": "apps/desktop/src/",
    "candidate_prefixes": [...],
    "affected_feature_paths": [...]
  }
  ```
- Classifications enumerated: `none`, `prefix-move`, `prefix-split`, `target-deleted`, `mixed`, `unknown`.
- Threshold defaults:
  - `prefix-split`: ≥3 files moved to ≥2 distinct new prefixes.
  - `prefix-move`: ≥5 files moved to one new prefix.
- Thresholds tunable; v1 exposes them in evidence output.

### §4 Reconcile Behavior
- Prefix restructure evidence upgrades generic `blocked` to `structural-conflict` or `target-deleted` category (PRD 8 taxonomy integration).
- Candidate prefixes are hints only, not authoritative moves.
- Provider integration NOT required.

### §5 Implementation Notes
- Start with Git name-status + path-prefix counts.
- Thresholds prevent over-reporting tiny path churn.
- **No source snippets persisted** (D10 privacy).
- Candidate prefix output capped at 5 entries, sorted by support count desc then path asc.

### §6 Acceptance Criteria (6)
1. Detector reports when feature paths fall under upstream-renamed or split prefix.
2. Blocked taxonomy (PRD 8) can consume restructure evidence.
3. Output includes old prefix, candidate prefixes, affected paths, confidence.
4. Detector runs without language parsers or a provider.
5. Candidate prefix output capped at 5 entries + deterministically sorted (support desc, path asc).
6. Thresholds use documented defaults unless explicit config override.

## γ-2 hard constraints (binding)

1. **No new `FeatureState` values**.
2. **No new persisted-schema fields outside ADR-025 D1-D13**. Read D13 FIRST — does it authorize `path-restructure` evidence kind? D13 pre-authorized PRDs 4-9 evidence kinds including `path-restructure` (verify by grepping the ADR). If NOT covered, draft a minor D14 amendment BEFORE extending schema (do not silently extend).
3. **D10 privacy**: NO source snippets in persisted evidence. Only path strings + counts + classification. Any new tests seed secrets into TITLE/SLUG/PATH metadata (Wave β F3 lesson).
4. **Blocked taxonomy integration (PRD 8)**: When path-restructure evidence is present + outcome is `blocked`, PRD 8 classifier MUST upgrade category to `structural-conflict` or `target-deleted`. Verify via integration test that PRD 8 precedence still holds.
5. **PRD 8 backward-compat**: existing blocked cases without path-restructure evidence still classify correctly.
6. **ADR-025 D11 malformed handling**: any new evidence writer follows lenient-loader pattern (γ-1 F2 lesson doesn't apply here since we're writing, but reader-side integration into PRD 8 classifier must handle malformed path-restructure evidence gracefully).
7. **Thresholds MUST be config-driven** per PRD §6.6 — expose in `config.yaml` (mirroring existing config patterns) with documented defaults.
8. **No provider integration** (PRD §4).
9. **Deterministic output**: candidate prefixes sorted by support count desc, path asc. Test the sort explicitly.
10. **ADR-024 / `patch-generations.json` UNTOUCHED**.
11. **Side Research md5**: `b385fe622db9926f48861105239f113e`. Verify: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.
12. **Commit trailer**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`. Use `git -c commit.gpgsign=false commit --no-verify`.
13. **Validation gates**: `gofmt -l .` (direct, NEVER piped), `go vet ./...`, `go build ./cmd/tpatch`, `go test ./...`.

## Carry-forward dispatch rules (all 15 binding for γ-2)

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
14. Two-opinion external review protocol (supervisor + user-parallel) MANDATORY. 5 consecutive rev cycles has caught HIGH BLOCKERs.
15. (γ-1 F1) When PRD names a command/event as trigger, verify the command/event actually exists in production code BEFORE wiring implementation.

## Session Summary

γ-1 rev-1 archived 2026-07-10. γ-2 (PRD 9, final WP-003 slice) unblocks.

## Next Steps

1. γ-2 implementer dispatch — single implementer, no wave slicing (single PRD).
2. Suggested layout:
   - `internal/workflow/path_restructure.go` — detector pass (Git name-status → classification + candidate prefixes).
   - `internal/workflow/blocked_taxonomy.go` — extend classifier to consume path-restructure evidence and upgrade `blocked` → `structural-conflict` / `target-deleted`.
   - `internal/store/config.go` / `config.yaml` — expose thresholds.
   - `internal/workflow/reconcile.go` — wire detector into reconcile flow (likely alongside file-novelty / hunk-overlap passes).
3. Tests:
   - Unit tests per classification (`prefix-move`, `prefix-split`, `target-deleted`, `mixed`, `none`, `unknown`).
   - Threshold boundary tests (2 files → not split; 3 files 2 prefixes → split).
   - Candidate cap + sort determinism test.
   - PRD 8 integration test: path-restructure evidence upgrades blocked category correctly.
   - PRD 8 precedence test: multi-category scenario including path-restructure still respects PRD 8's precedence order.
   - D10 privacy test: seed secret into path component; assert evidence artifact does not contain source content.
4. After γ-2 lands + three-way APPROVED: WP-003 cluster complete. Plan v0.11.0 release bundling Wave α + Wave β + Wave γ.

## Blockers

None.

## Context for Next Agent

- WP-003 §6 lists 9 PRDs. 8 shipped. PRD 9 is the final slice.
- ADR-025 D13 should pre-authorize `path-restructure` evidence kind — verify FIRST before scaffolding schema.
- PRD 8 classifier (`internal/workflow/blocked_taxonomy.go`) already accepts secondary evidence — integration point is well-defined.
- Reviewer briefs for γ-2 MUST include the "does the PRD trigger/name/event exist in production?" grep (rule 15) — γ-1 F1 root cause.
- Side Research md5 invariant: `b385fe622db9926f48861105239f113e`. Verify before/after any CURRENT.md edits.

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
