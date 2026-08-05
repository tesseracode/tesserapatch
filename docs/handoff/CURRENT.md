# Current Handoff

## Status

**Cluster state**: REV-2 DISPATCHED (planning phase)

**WAVE_BASE**: `8574ff3` (Cluster F v0.13.0 GH #6 planning dispatch, 2026-08-05).

**2026-08-05 Cluster F DISPATCHED (planning-first — v0.13.0 GH #6).** First-class `rejected` feature lifecycle state per GH #6. Planning phase: PRD + ADR pair. This is a data-model extension (not just a CLI addition), so architectural coverage is required before any code lands. Dual review at planning approval; implementation phase becomes Cluster F' after PRDs+ADRs land three-way APPROVED.

**2026-08-05 Cluster E-prime SHIPPED + E'-N2 amendment (post-Cluster-E hygiene follow-up).** Two LOW observations from external's post-Cluster-E review folded. Scope: Obs 1 (`PinGitAutoGCOff` doc comment clarifying unconditional `GIT_CONFIG_COUNT=1` clobber semantics; mechanism unchanged); Obs 2 (`.wave-close-allowlist` at repo root — Makefile `[2/8]` subtracts allowlisted entries from WARN list; prints `OK (N entries allowlisted)` when residual is empty; residual still WARNs). Post-ship amendment (`189f5d6`): E'-N2 sentinel-self-guard fold — `.wave-close-allowlist` added to `[2/8]` glob list so sentinel catches its own untracked-config case. Post-E-prime external review verdict "NEEDS REVISION" evaluated: F1 HIGH / F2 part-1 / F3 were all against reviewer's stale local pre-fetch tree — only F2 part-2 (self-guard) was state-independent and genuinely valid. E'-N1 (stale-entry bitrot) remains backlog. Range `2281309..8574ff3`.

**2026-08-04 Cluster E SHIPPED (process housekeeping).** Two findings from external's post-Cluster-D review + 1 rev-1 fold. Scope: F1 MEDIUM (`make wave-close-check` never ran `go test` — gate PASSed with red suite empirically demonstrated at Cluster D HEAD; fixed by adding `[8/8] go test -count=1 ./...`), F2 LOW (`t.TempDir()` teardown race on macOS from unpinned `git commit` forking `gc --auto --detach`; fixed by `gc.auto=0` env pin), E-EXT-1 MEDIUM rev-1 fold (F2 pin was `internal/cli`-only; extracted `internal/testutil.PinGitAutoGCOff()` and applied to `internal/gitutil`, `internal/workflow`, `internal/store`). Two-opinion scoreboard: rev-0 dual (internal APPROVED, external APPROVED WITH NOTES 1 MEDIUM), rev-1 external-only confirmation (APPROVED WITH NOTES — 2 non-functional commit-message accuracy notes, no code defects). Range `1bc2a25..b294d8c`.

**2026-08-03 Cluster D SHIPPED.** Correctness housekeeping — 8 items total, single implementer, sequential. Four review revs (rev-0 → rev-3). Two-opinion protocol scoreboard: rev-0 dual (internal NEEDS REVISION 3 MEDIUM + 1 LOW, external APPROVED WITH NOTES 1 MEDIUM overlap), rev-1 dual (internal NEEDS REVISION 1 MEDIUM residual, external APPROVED), rev-2 external-only (NEEDS REVISION 1 MEDIUM new Rule 17 residual), rev-3 external-only (APPROVED via prescriptive verbatim wording). **Notable pattern**: three consecutive iterations on the same fast-path help clause each introduced a new Rule 17 residual; broken by supervisor-prescribed verbatim text at rev-3. All 6 backlog items + 2 review-fold items landed. Deferred: D-INT-2 (--from-revision post-crash out of PRD-#4 F-4 scope), F-EXT-2 (concurrency out of local-CLI scope). Range: `4868f68..42f85d7` (13 commits: 8 rev-0 impl + 3 rev-1 folds + 1 rev-2 fold + 1 rev-3 fold, plus 4 tracking commits).

**2026-08-03 Cluster D DISPATCHED.** Correctness housekeeping — single implementer, sequential, small-scope items. Scope: 6 backlog items (PRD-#3 N2/N3/S1, PRD-#4 F-4, GH #5 docs, Wave γ LOW-γr15-N1) + 2 review-fold items from external's post-Cluster-C review (F1 MEDIUM: gate glob gap; F2 LOW carry-over: LOG SHA pointer).

**v0.12.1 SHIPPED 2026-07-31.** Correctness fix pass (GH #3 + #4 + #5) three-way APPROVED at rev-1 across all three tickets. Cluster A (AGENTS.md wave-close checklist) shipped earlier same day. All work pushed to `origin/main`. v0.12.1 tag pushed.

**2026-08-02 CI back green on `main`.** Inline hygiene fix at `4619b55` — `gitInitTestRepo` pinned to `-b main` — resolved a persistent CI failure class (red since 2026-07-28).

**2026-08-02 Cluster C SHIPPED.** Process housekeeping — parallel-implementer discipline addendum + `make wave-close-check` mechanical gate. Four review revs total (rev-0 → rev-4). Two-opinion protocol scoreboard: **external-only catches** on rev-0 (BLOCKING unpushed), rev-1 (3 HIGH + 2 MEDIUM incl. empirical false-passes), rev-2 (1 HIGH duplicate-field), rev-3 (1 BLOCKING shell-bug + 1 non-blocking). Internal APPROVED at rev-1 and rev-2; rev-3 and rev-4 were external-only cycles for single-issue empirical fixes. Cluster C's own gate now dogfoods on every commit going forward. Range: `bb31872..870182d`.

## Active Task

**Cluster F — v0.13.0 GH #6 first-class `rejected` feature lifecycle state (planning phase).** Single implementer for planning. WAVE_BASE `8574ff3`. Dispatched 2026-08-05.

### Deliverables (planning phase — no code)

1. **`docs/prds/PRD-rejected-feature-state.md`** — user-facing behavior spec.
2. **`docs/adrs/ADR-028-rejected-feature-state-data-model.md`** — data-model choice + rationale.
3. Both cite GH #6 verbatim (evidence anchor) and existing patterns from PRD-#4 (confirm-upstreamed retirement audit) as related-work anchors.

### PRD scope (user-facing)

Extract from GH #6 verbatim wherever possible; PRD should paraphrase only for precision, never for scope. Cover:

- **New terminal state**: `rejected` — must be first-class in `feature.yaml` state enum, not encoded in prose.
- **Required fields**:
  - `reason` — enum from: `not-a-bug`, `premise-disproved`, `obsolete`, `out-of-scope`, `unsafe`, `duplicate`, `superseded`. (Open question: closed enum vs open-string with recommended enum?)
  - `evidence` — at least one path reference; PRD must specify path validation semantics (must-exist? must-be-in-feature-dir? committed vs untracked?).
  - `note` — free-form rationale.
  - `rejected_at`, `rejected_by`, `prior_state` (append-only audit).
  - `related` — optional feature slug or GH issue reference.
- **CLI**: `tpatch reject <slug> --reason <code> --evidence <path> [--note <text>] [--related <ref>]`. PRD should evaluate alternative `tpatch feature state <slug> rejected ...` shape and pick one with rationale.
- **State machine transitions**:
  - Allowed from: `requested`, `analyzed`, `defined`, `explored`. (Question: from `implementing`/`applied`/beyond? GH #6 §9 says "refuse from states where source changes are already applied unless the command also performs a safe retirement audit". PRD must define this.)
  - Reopen transition: `rejected → requested` via `tpatch reopen <slug> --evidence <path> --note <text>`. Append-only audit; does not erase rejection record.
- **Integration**:
  - `tpatch status`: rejected features excluded from actionable backlog by default; `--all` or `--include-rejected` opts in.
  - `tpatch next`: prints rejection reason + evidence + reopen command instead of proposing `analyze`.
  - `FEATURES.md` renderer: distinct section or row style; shows reason + evidence link.
  - `apply`/`reconcile`: refuse by default on rejected features; explicit override behavior TBD.
- **JSON envelope**: `tpatch reject --json` shape; `tpatch status --json` addition of rejection fields.
- **Tests-to-write list** (matches GH #6 acceptance criteria):
  1. Reject before analysis (from `requested`).
  2. Reject after analysis/definition (from `analyzed`/`defined`/`explored`).
  3. Reject with missing evidence file → error.
  4. Reject with invalid reason code → error.
  5. Dependency effects (if slug B depends on A, and A rejected, what happens to B? Fail loudly? Warn? PRD must define.).
  6. Reopen from rejected → requested.
  7. Reject on already-applied state → error unless explicit escape hatch.
  8. Status/next/FEATURES.md integration.
- **Distinctions from related concepts** (verbatim from GH #6 §"Distinction from related concepts"): remove, supersedes (#1), upstream_merged, blocked, rejected-upstreamed (#4). PRD must cite each and confirm no overlap.

### ADR scope (data model)

- **Storage location**: `feature.yaml` state field extension vs separate `.tpatch/features/<slug>/rejection.json` sidecar vs append to `history.json`? Evaluate the three options with:
  - Human editability tradeoffs.
  - Consistency with PRD-#4 confirm-upstreamed retirement audit (which uses `RetirementAudit` on `ReconcileResult`, not on `store.FeatureStatus` — verify current state).
  - Migration path (existing features without the field).
  - Determinism / stability guarantees (`.tpatch/` artifacts must be deterministic per CLAUDE.md rule 4).
- **Reason enum shape**: closed enum in code + validation vs open string + recommended-values doc?
- **Evidence field format**: single string vs list; relative path vs URL; path resolution semantics.
- **State machine formalization**: draw the transition graph, list allowed transitions, list refused transitions with exit codes.
- **Reopen mechanism**: append-only history vs full re-state replay; how many reopen cycles are supported (bounded vs unbounded)?
- **Interaction with confirm-upstreamed**: PRD-#4's confirm-upstreamed is a RECONCILIATION verdict on already-implemented features; Cluster F's rejected is a PRE-implementation lifecycle terminal. These are ORTHOGONAL. ADR must explicitly note this and cite PRD-#4 lines to prevent future conflation.
- **Backward compatibility**: how do existing feature.yaml files that lack the field render in `tpatch status` after upgrade? Silent OK? Warning? Migration hint like the D10 `patch-generations.json` fallback pattern (Cluster D Item 1)?

### Constraints (per AGENTS.md + CLAUDE.md)

- Planning-phase = docs ONLY. No `internal/`, `cmd/`, `assets/`, `Makefile`, or test changes.
- PRD claims-audit appendix (per AGENTS.md WP-001 §3.5 graduated conventions): every load-bearing claim about current behavior must cite `file:line` in `SPEC.md`, `docs/dependencies.md`, `docs/feature-layout.md`, or `internal/`/`assets/` source.
- ADR must evaluate at least 3 alternatives per major decision, not just present a chosen path.
- Explicit `git add <path>` per commit; NEVER `-a`/`-A`.
- `git commit -F <tempfile>` with Copilot + Copilot-Session trailers; never inline heredoc.
- Side Research md5 `b385fe622db9926f48861105239f113e` MUST remain preserved.
- Do NOT touch canonical `**Cluster state**` field.
- Do NOT stage the 15 remaining untracked WIP files (allowlisted at rev-0).

### Non-goals (planning phase)

- Do NOT write implementation code. That's the F' implementation cluster.
- Do NOT add tests. Tests-to-write list belongs in the PRD.
- Do NOT modify feature.yaml schema in `assets/` or `internal/`. That's implementation.
- Do NOT extend to related-issues #1 (supersedes) or #4 (confirm-upstreamed). The PRD must cite them for orthogonality but NOT re-scope.
- Do NOT block on the E'-N1 backlog item (allowlist stale-entry bitrot) — orthogonal.

## Session Summary

- **v0.12.0** (three-wave feature cluster: supersession + write-file safety + active-feature-session) — shipped, tagged `v0.12.0`.
- **Cluster A** (AGENTS.md wave-close checklist codifying F1 pattern) — shipped at `5ac458d`.
- **Cluster B planning** (PRDs #3 + #4 with dual-review parallel) — shipped at `4e673a8`.
- **v0.12.1 implementation** (GH #3 + #4 + #5 correctness fix pass) — shipped at `bb31872`, tagged `v0.12.1`.
- **CI hygiene fix** — `4619b55` pinned `gitInitTestRepo` to `-b main`; CI back green 2026-08-02.
- **Cluster C** (parallel-implementer discipline + `make wave-close-check` mechanical gate) — shipped at `4868f68` after 4 review revs.
- **Cluster D** (correctness housekeeping — 6 backlog items + 2 review-fold items) — shipped 2026-08-03 after 4 review revs. Range `4868f68..42f85d7`.
- **Cluster E** (process housekeeping — F1 gate `[8/8] go test` coverage + F2 macOS teardown race via `gc.auto=0` pin extracted to shared `internal/testutil` helper) — shipped 2026-08-04 after 1 rev-1 fold (E-EXT-1 cross-package pin). Range `1bc2a25..b294d8c`.
- **Cluster E-prime** (post-Cluster-E review follow-up — Obs 1 `PinGitAutoGCOff` doc comment + Obs 2 `.wave-close-allowlist` mechanism for `[2/8]` gate step) — shipped 2026-08-05 external-only rev-0 APPROVED WITH NOTES. Range `2281309..aa34f3c`.

## Files Changed at v0.12.1 Consolidation

- `CHANGELOG.md`: v0.12.1 header dated 2026-07-31; GH #4 review-path subsection added; rev-1 fold-in subsection appended.
- `docs/ROADMAP.md`: v0.12.1 ✅ SHIPPED section added above v0.12.0.
- `docs/prds/PRD-confirm-upstreamed-human-review-path.md`: Status `Proposed` → `Accepted`.
- `docs/handoff/HISTORY.md`: v0.12.1 archived under 2026-07-31 header.
- `docs/handoff/CURRENT.md`: reset (this file).

## Test Results

- `gofmt -l .` empty; `go vet ./...` clean; `go build ./cmd/tpatch` OK.
- `go test ./...` 907 top-level PASS + subtests (0 FAIL).
- Wave α + β + γ non-invalidation: empty diff on 5 guarded files at v0.12.1 consolidation.
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.

## Next Steps

**Backlog after Cluster E-prime**:

Feature / release:
- **Cluster F — v0.13.0 GH #6 first-class rejected feature state** — data-model extension, PRD + ADR pair. Larger planning-first cluster. Only remaining open GH issue.

Deferred from Cluster E-prime rev-0 external review (documented, no fold — reviewer's explicit "not required for this rev to ship"):
- **E'-N1 LOW** — `.wave-close-allowlist` stale-entry bitrot is silent. Allowlist entries whose files land (via `git add`) or delete are silently ignored — no active flagging in gate, no protocol coverage in AGENTS.md pruning guidance. Passive signal only via `(N entries allowlisted)` count trend. Two fix options: (a) active stale-entry sub-check reporting patterns matching zero untracked files as "candidate for removal"; (b) extend AGENTS.md checklist bullet to require pruning when a file lands/deletes. Fold into Cluster F pre-flight or next hygiene cluster if allowlist grows beyond initial 16 seed.

Deferred from Cluster D adjudication (documented, no fold):
- **D-INT-2** (`--from-revision <original>` post-crash "superseded" error) — PRD-#4 lines 180/259 document the flag as CI/test override, not the crash-recovery path. Default retry works (external Rule 20 verified). Backlog if operator friction surfaces.
- **F-EXT-2** (concurrency safety of confirm-upstreamed) — pre-existing; concurrent invocation of same slug not a supported local-CLI scenario.

Untracked WIP (surfaced by Cluster D Item 7 gate glob extension; NOT staged by Cluster D):
- `docs/whitepapers/WP-004..WP-007.md` + `.turns.md` siblings (8 files).
- `docs/prds/PRD-feature-unapply.md`, `docs/prds/PRD-recurring-patches.md`.
- `docs/state-of-the-art/*case-study*` (2 files).
- These require operator decision on disposition; not a defect.

Process / hygiene (all shipped this session):
- ✅ AGENTS.md parallel-implementer discipline addendum (Cluster C).
- ✅ Mechanical wave-close-check gate (Cluster C).
- ✅ Gate glob covers whitepapers + state-of-the-art (Cluster D fold F1).

Documentation:
- **ADR-027 F2** (nit).
- **Doctor S3-boundary deferrals** (from Wave β).
- **ADR-029 nit deferrals**.

## Blockers

None.

## Context for Next Agent

- **v0.12.1 SHIPPED** — do NOT re-open Wave α/β/γ or GH #3/#4/#5 scope. All accepted.
- **Two-opinion protocol proven load-bearing again** — v0.12.1 rev-0 external caught 4 findings internal missed (PRD-#4 warning wording, PRD-#4 tie-break correctness bug, PRD-#3 err-branch gap, GH #5 hint mislabel). Internal caught PRD-#3 F-INT-3-1 HIGH (Rule 18 trailer parse failure). Continue dual-review protocol on all clusters ≥ paper-only.
- **Cross-implementer entanglement is now a KNOWN failure mode** — do NOT dispatch parallel implementers to shared source files without briefing them on `git add <path>` discipline. See Cluster A follow-up in backlog.
- **20 binding carry-forward rules** unchanged. Rule 18 empirical demonstration this cluster: heredoc-authored commit bodies leaked `EOF)` after the trailer, breaking `%(trailers)` parse. Rule 20 empirical demonstration: PRD-#4 external caught the tie-break bug via code path enumeration (in-place dedup) that internal's tests-pass verdict didn't surface. Rule 20 continues to require empirical repro even on paper-approved designs.
- **Side Research md5 invariant**: `b385fe622db9926f48861105239f113e`. Verify: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.
- **Commit trailer**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` verbatim + `Copilot-Session: <session-id>` per session. Use `git commit -F <tempfile>` or `git commit -m ""` — NOT `git commit -F -` with heredoc (heredoc close tokens leak into the body).

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
