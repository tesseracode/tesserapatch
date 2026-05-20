# Current Handoff

## Active Task

- **Task ID**: `adr-026-patch-amendment-policy`
- **Milestone**: v0.10.0 Wave γ prep (WP-002 cluster slice-4 gate). Per user standing direction, draft this ADR before any `PRD-feature-patch-amend` implementation work.
- **Description**: Draft `ADR-026-patch-amendment-policy.md`. Locks the policy gaps left open by `PRD-feature-patch-amend` and called out by WP-002 §4 row 3: refresh/fixup defaults, dependent-staleness surface shape, verify-freshness invalidation rules, command-namespace constraints, `--reason` flag policy, fork/fold v2 deferral. Same flow as ADR-024: implementation sub-agent drafts → reviewer sub-agent verifies → external review.
- **Status**: In Progress (implementer being dispatched).
- **Assigned**: 2026-05-19.

### Scope

Docs-only. Sole deliverable is `docs/adrs/ADR-026-patch-amendment-policy.md`. No code, no test, no PRD body changes. Implementer must enumerate the open decisions, evaluate alternatives where alternatives exist, and lock one option per decision with a binding `## Decision` block per ADR convention.

### Open decisions the ADR must lock

1. **Plain `record <slug>` byte change classification (D1)**: PRD §3.2 says plain changed-bytes `record` defaults to `kind: amend-refresh` for backward-compat. ADR locks: is this `amend-refresh` or stays at `record` (Wave β D8 enum)? Conflict — Wave β D8 currently writes only `record` or `reconcile`. ADR must reconcile.
2. **`--reason` persistence (D2)**: PRD §4.2 requires `--reason` for fixup. Where does the reason text live — `record.md` body, a new `amendments[].reason` array, or generation `metadata` field? Privacy: PRD §3 cross-refs `ADR-capture-context-privacy-boundary` (deferred). ADR must say "advisory-only, no privacy gate in v1" or defer to that ADR.
3. **No-byte-change refresh semantics (D3)**: PRD §4.1 step 3 says no-byte-change refresh prints a note and does NOT append a generation. ADR locks: silent skip, exit 0 success, or exit code signaling no-op? Mirror Wave β append-skip semantics for consistency.
4. **`fixup_of_generation` field (D4)**: New per-generation field. Confirm placement (`generations[].fixup_of_generation`), type (`generation_id` string), and whether v1 schema bumps to v2 or stays v1 (Wave β D9 strict-on-unknown). Likely add as optional field present only for `kind: amend-fixup`; schema stays v1 because adding optional fields is backward-compatible under D9 ONLY IF the field is registered up front. ADR must lock the field shape and whether schema version increments.
5. **Dependent-staleness surface shape (D5)**: PRD §5.1 mandates "one visible dependent-staleness surface" but defers the name (`parent-generation-stale` is suggested). ADR locks the canonical surface — overlay/label on `status`, separate `status --stale` output, or a new manifest field? PRD §5.1 also says "label/overlay, not lifecycle state."
6. **Verify-freshness invalidation rules (D6)**: PRD §5.2 says patch-content amendments invalidate freshness "because either patch hash or recipe hash changed." ADR cross-references ADR-013 verify freshness overlay and confirms which exact hash inputs trigger invalidation. Metadata-only amend should not invalidate unless touching ADR-013-named inputs.
7. **Command-namespace finality (D7)**: Broker locked `tpatch feature patch refresh|fixup <slug>`. ADR records the locked surface, confirms no aliases, and confirms `fork`/`fold` are v2-deferred (PRD §4.4–4.5). ADR also locks that plain `record <slug>` does NOT require the new namespace for backward-compat (PRD §3.2).
8. **`record --force-amend` boundary (D8)**: PRD §6 promises this stays unchanged. ADR locks: `--force-amend` is for Git-rewrite orphan detection, NOT a fixup/refresh shortcut. Cross-reference ADR-014 or wherever `--force-amend` is currently locked.
9. **Metadata-only amend manifest revisions (D9)**: PRD §9 Open Question — "Should metadata-only amendments have their own manifest revision number?" ADR must answer yes or no. Cost of yes: schema growth, new write path, ambiguity with patch-bytes generations. Cost of no: metadata-only audit story lives outside the manifest (claims.json, status diffs). Recommendation in ADR will be NO for v1; defer to v2 if needed.
10. **Wave β D8 enum forward-compat (D10)**: Wave β D8 reserves `amend-refresh` and `amend-fixup` as forward-compat read kinds. ADR confirms Wave γ now writes them — no Wave β schema change needed, but ADR explicitly transitions D8 from "reserved" to "writable for `tpatch feature patch refresh|fixup`."

### Out of scope for THIS ADR

- Fork/fold v2 commands (deferred — call out).
- `ADR-capture-context-privacy-boundary` decisions (deferred — call out).
- WP-003 reconcile cluster work.
- Any code or test.
- CHANGELOG (Wave γ is mid-cluster).

### ADR header constraints

- File: `docs/adrs/ADR-026-patch-amendment-policy.md`.
- Status: Accepted.
- Date: 2026-05-19.
- Use the same structural shape as ADR-024: header (Status/Date/Context), `## Decision` per D1–D10, `## Alternatives Considered` where alternatives exist, `## Consequences`, `## References`.
- References section MUST cite: `PRD-feature-patch-amend`, `WP-002-capture-and-metadata-foundation` §4, ADR-024 (Wave β contract), ADR-013 (verify freshness), ADR-011 (feature dependencies), and any code anchors for `--force-amend`.
- Disambiguation note at top noting ADR-025 is the WP-003 reconcile cluster's slot (unwritten, reserved) — this ADR-026 is the WP-002 cluster slice-4 gate.

### Quality gates

Docs-only — no code build/test required. Implementer must ensure markdown lints clean (no orphaned references, internal links resolve).

### History of prior section (superseded)



- **Task ID**: TBD (awaiting next-slice decision)
- **Milestone**: v0.10.0 capture-and-metadata foundation cluster (Wave alpha + beta complete; Wave gamma pending)
- **Description**: Wave beta (`PRD-feature-patch-identity-metadata`) shipped 2026-05-19 (commits `916ee39`, `e7be5e8`, `7e5dea6`; external APPROVED on rev-2). Next slice options awaiting user direction:
  - **Option A — Wave gamma ADR prep**: Draft `ADR-patch-amendment-policy` (gates `PRD-feature-patch-amend`). Next ADR slot after the WP-003-reserved `ADR-025-reconcile-evidence-and-revision-schema`. Per user standing direction, hold gamma implementation until this ADR exists.
  - **Option B — v0.10.0 cluster closeout**: Ship Wave alpha + beta as a release (defer Wave gamma to v0.11.0). Would need CHANGELOG entry, version bump, tag.
  - **Option C — WP-003 reconcile cluster work** or other pending todos.
- **Status**: Idle — awaiting supervisor dispatch.
- **Assigned**: 2026-05-19.

## Session Summary

Wave beta closed. See `docs/handoff/HISTORY.md` top entry for the full stack and ADR-024 D1–D9 conformance summary.

## Current State

- v0.10.0 cluster: Wave alpha shipped (v0.9.0 tag at `9267026`), Wave beta complete (on `main`, not yet released).
- WP-002 (capture-and-metadata foundation) at 3/4 slices. WP-003 (reconcile safety & middle-pass) unstarted.
- ADRs: ADR-024 shipped; ADR-025 reserved-but-unwritten (WP-003 Wave α implementer drafts when that work begins); `ADR-patch-amendment-policy` (next slot = ADR-026) pending for Wave gamma.

## Files Changed

None in this transition (archival only).

## Test Results

Baseline: 612 `func Test...` declarations, all ten packages green under `go test ./... -count=1 -race`.

## Next Steps

1. Supervisor presents next-slice options to user.
2. User picks A / B / C.
3. Dispatch implementer with full brief.

## Blockers

None.

## Context for Next Agent

- ADR-024 is binding for any future patch-generation code: D1–D9 are not reopenable without a follow-up ADR.
- `store.ErrMalformedManifest` is the canonical sentinel for malformed-manifest classification (rev-2). I/O errors are unwrapped; JSON-decode + schema-validation failures are `%w`-wrapped with it.
- Wave gamma scope (`PRD-feature-patch-amend`) explicitly depends on Wave beta's patch-generations.json — generation_id provides the stable amendment-target anchor.

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
