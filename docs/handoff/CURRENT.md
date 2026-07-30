# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-wave-gamma-active-feature-session-implementation`
- **Milestone**: v0.12.0 Wave γ — implement `PRD-active-feature-session` + honor `ADR-027` D1 F3 lock.
- **Description**: Third and final wave of the v0.12.0 3-wave sequential cluster. Introduces the `tpatch session` command group + `.tpatch/local/capture/` local-buffer storage lane + `tpatch init` `.gitignore` amendment that together satisfy ADR-027 D1's F3 conditional. Six-mandate refusal contract at PRD-active-feature-session §4 D6 is the ENTIRE safety margin.
- **Status**: Rev-0 landed 2026-07-30 — ready for dual review.
- **Assigned**: 2026-07-30.

## Session Summary

Wave γ implementer rev-0 executed all 5 locked slices in order. SHA
range covering rev-0: `561e6de..HEAD` (four implementer commits atop
Wave β consolidation `561e6de` on `origin/main`).

Per-slice landing:

- **Slice 1 — Storage lane + `.gitignore` contract** (`7c77723`):
  - New `internal/store/session.go` — Session/ContextSummary wire
    types, `cs_<12hex>` + `ctx_<12hex>` content-addressed IDs (PRD §3
    D3.2 identity inputs = `{schema_version, repository_identity,
    feature, base_commit, capture_mode, workspace_discriminator}`,
    wall-clock EXCLUDED per D3.3), Save/Load/List/Purge with path
    safety + D18 cross-feature isolation refusal at LoadSession +
    symlink-escape refusal at PurgeSession (macOS-safe: resolves
    both sides of the compare so `/var/folders` symlink tree does not
    spuriously trip the guard).
  - New `internal/gitutil/ignore.go` — `IsGitAvailable`,
    `IsPathIgnored` via `git check-ignore -q --no-index` exit-code
    semantics (PRD §4 D6 mandate 5). NOT textual matching.
  - New `internal/workflow/session_ignore.go` — single sentinel
    `ErrLocalIgnoreRefusal` wrapping typed
    `*LocalIgnoreRefusal{Reason, Path, Detail}`, `LocalIgnoreRule =
    ".tpatch/local/"`, `PrePRDWorkspaceFallbackPath =
    ".git/tpatch/capture/"` (mandate 6), `EnsureLocalIgnoreContract`
    (mandates 3+4+5 at session-start-time), `EnsureLocalGitignoreRule`
    (mandates 1+2 at init-time). Rendered `Error()` enumerates all
    six mandates verbatim.
  - `internal/cli/cobra.go` initCmd amended: after `store.Init`,
    calls `EnsureLocalGitignoreRule`; emits `gitignore:` line in the
    init output (Rule 19 + ADR-027 D1 + PRD §4 D6 mandate 1).
  - New `internal/workflow/session_ignore_test.go` — 9 detached-
    worktree D6 fixtures: git-unavailable, effective-check
    no-gitignore, negation-rule defeats textual match,
    path-outside-worktree, happy path, init creates/appends/no-dup/
    refuses-unwritable. `mustBeRefusal` helper asserts `errors.Is`
    on sentinel + reason + all six mandates in message.
  - New `internal/store/session_test.go` — 8 fixtures:
    ID determinism, shape validation, round-trip, D18 refusal,
    malformed rejection, idempotent purge, bad-id refusal, symlink
    refusal, list sort, malformed isolation.

- **Slice 2 — Session command group + lifecycle** (`f52fcfa`):
  - New `internal/cli/session.go` — `tpatch session` cobra group +
    `start`, `stop`, `list`, `summarize`, `purge` subcommands; per-
    subcommand help text lists only subcommand-specific flags per
    Rule 11; `pickSessionForOp` shared ambiguity helper (reused by
    Slice 4 `record --with-session`); `SessionListJSON` deterministic
    shape; single-active-session invariant at write time (PRD §3 D1.5
    idempotence).
  - New `internal/cli/session_summarize.go` — `runSessionSummarize`
    shared entry point.
  - New `internal/cli/session_redaction.go` — Slice 2 stub
    (fleshed out in Slice 3).
  - `internal/cli/cobra.go` — `sessionCmd()` registered on root
    command list.
  - Parity guard (Rule 15): `assets/assets_test.go` +5 required
    commands (`tpatch session start|stop|list|summarize|purge`) and
    +2 required anchors (`session-local-capture/root`,
    `session-local-capture/gitignore-rule`). All 6 shipped skill
    assets updated in the same commit:
    - `assets/skills/claude/tessera-patch/SKILL.md`
    - `assets/skills/copilot/tessera-patch/SKILL.md`
    - `assets/prompts/copilot/tessera-patch-apply.prompt.md`
    - `assets/skills/cursor/tessera-patch.mdc`
    - `assets/skills/windsurf/windsurfrules`
    - `assets/workflows/tessera-patch-generic.md`
  - Rule 17 fold-in: corrected stale `V0-V9` verify enumeration to
    `V0-V10` on the Claude, Copilot, and Copilot Prompt assets
    (Cursor, Windsurf, Generic already at V0-V10). Tightly coupled
    to the same tables I was touching for session anchors.
  - New `internal/cli/session_lifecycle_test.go` — 9 CLI
    regression tests (init gitignore amendment, refusal without
    gitignore, start idempotence, stop transitions + idempotence,
    list JSON determinism, purge dry-run default, refusal on
    unknown feature, cross-feature isolation, summarize invalid
    flag pairs).

- **Slice 3 — D11 redaction contract + promotion boundary** (`1863733`):
  - `internal/cli/session_redaction.go` rewritten with 10 forbidden
    content classes: `secret-like-string` (OpenAI/GitHub PAT/AWS/
    Slack tokens, Bearer, `secret=`/`token=`/`api_key=` assigns),
    `absolute-home-path` (Unix + Windows), `prompt-text-marker`,
    `tool-call-argument`, `command-output-marker`,
    `stack-trace-marker` (Go/Python/JS), `ide-buffer-marker`,
    `clipboard-marker`, `vector-embedding-payload` (16+ float JSON
    arrays), `source-snippet-marker` (fenced ```<lang>``` blocks).
    Matched observations DROPPED from committed body; class code
    recorded in `ContextSummaryRedaction.FindingCodes`.
  - Labels scrubbed unconditionally (`label` in `ScrubbedFields`).
  - If EVERY observation is dropped, `runSessionSummarize` refuses
    with `promotion_refusal_reason` set; existing committed
    summaries left BYTE-IDENTICAL (PRD §8.12).
  - New `internal/cli/session_redaction_test.go` — 15 forbidden-
    class sub-tests + refusal-on-empty test + boundary invariant
    proof (`TestSessionSummarizePromoteWritesRedactedCopy`) + prior-
    summary-preserved-on-refusal test.

- **Slice 4 — record flags + cross-feature isolation** (`84d18ff`):
  - `internal/cli/cobra.go` recordCmd extended with two flags:
    `--with-session` (PRD §6 D15.1, opts in to same-feature session
    promotion after patch capture) and `--from-session <cs_id>`
    (PRD §6 D15.2 disambiguator, REQUIRES `--with-session` per
    §8.8).
  - Cross-feature isolation (PRD §7 D18): enforced by
    `store.LoadSession` slug/manifest agreement check inherited
    from Slice 1.
  - New `internal/cli/session_record_test.go` — 4 regression
    tests: --from-session-requires-with-session, cross-feature
    isolation refusal, promotion happy path (raw secret NEVER
    crosses, safe body committed, state -> promoted), ambiguous
    refusal without --from-session.

- **Slice 5 — CHANGELOG + PRD flip + ROADMAP + handoff refresh**
  (this commit):
  - `CHANGELOG.md` — new `### Wave γ` subsection under `## v0.12.0
    — TBD`; the earlier `### Wave β` and `### Wave α` subsections
    are byte-identical to their prior state.
  - `docs/prds/PRD-active-feature-session.md` line 2 flipped
    `Proposed` → `Accepted`.
  - `docs/ROADMAP.md` — Wave γ block expanded with per-slice
    landing summary and the `Rev-0 landed 2026-07-30, awaiting
    three-way review` status marker.
  - `docs/handoff/CURRENT.md` — this refresh. Side Research block
    preserved verbatim; md5 `b385fe622db9926f48861105239f113e`
    invariant maintained.
  - Test-fixture correction:
    `TestFeaturePatchRefreshNoByteChangeSkips` in
    `internal/cli/feature_patch_test.go` updated to commit the
    init-installed `.gitignore` before recording. Previously this
    test made a hidden assumption that `tpatch init` left the
    working tree clean; Wave γ's D6 mandate 1 rule installation
    surfaces the assumption and the fix is a two-line git-add /
    commit sequence at the top of the fixture. Cited in the Slice 5
    commit body.

## Current State

- All 5 slices landed on `main` locally. Rev-0 SHA range:
  `561e6de..HEAD`. No push (per dispatch: supervisor pushes after
  dual review dispatch).
- Full-suite `go test -count=1 ./...` passes across every package.
- Top-level test count: **865** (827 baseline at Wave β acceptance
  + 38 Wave γ additions across store/workflow/cli/assets). Target
  was ≥ 826 + 15-25 = 841+; delivered 838+ additions counted at
  the top-level `=== RUN <TestName>$` grain.
- `gofmt -l .` empty. `go vet ./...` clean.
- Side Research md5 `b385fe622db9926f48861105239f113e` preserved
  verbatim (verified post-refresh).

## Files Changed

Rev-0 SHA range: `561e6de..HEAD` (4 implementer commits + Slice 5).

Slice 1 (`7c77723`):
- `internal/store/session.go` (new, 478 lines)
- `internal/store/session_test.go` (new)
- `internal/gitutil/ignore.go` (new)
- `internal/workflow/session_ignore.go` (new, 258 lines)
- `internal/workflow/session_ignore_test.go` (new)
- `internal/cli/cobra.go` (init amendment)

Slice 2 (`f52fcfa`):
- `internal/cli/session.go` (new, 503 lines)
- `internal/cli/session_summarize.go` (new, 129 lines)
- `internal/cli/session_redaction.go` (new, Slice 2 stub)
- `internal/cli/session_lifecycle_test.go` (new)
- `internal/cli/cobra.go` (sessionCmd registration)
- `assets/assets_test.go` (+5 required commands, +2 required anchors)
- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/windsurf/windsurfrules`
- `assets/workflows/tessera-patch-generic.md`
- `internal/store/session.go` (macOS symlink-resolve fold-in)

Slice 3 (`1863733`):
- `internal/cli/session_redaction.go` (fleshed out with 10 D11
  matchers)
- `internal/cli/session_redaction_test.go` (new)

Slice 4 (`84d18ff`):
- `internal/cli/cobra.go` (recordCmd --with-session/--from-session
  wire-up)
- `internal/cli/session_record_test.go` (new)

Slice 5 (this commit):
- `CHANGELOG.md`
- `docs/prds/PRD-active-feature-session.md` (line 2 status flip)
- `docs/ROADMAP.md` (Wave γ block expanded)
- `docs/handoff/CURRENT.md` (this refresh)
- `internal/cli/feature_patch_test.go` (test-fixture correction:
  commit `.gitignore` after init before recording so the byte-
  identical refresh assertion still holds).

## Test Results

- `gofmt -l .` — empty.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean.
- `go test -count=1 ./...` — full-suite PASS.
- Top-level test count (`go test -v ./... | grep -E '^=== RUN\s+[^/]+$' | wc -l`):
  **865** (baseline 827; delta +38 Wave γ additions).

## Next Steps

1. Supervisor dispatches three-way review (internal + external +
   user-external) per Wave α + Wave β cadence.
2. Reviewers verify:
   - All six D6 mandates covered by refusal-path fixtures.
   - `git check-ignore` semantics used, not textual matching.
   - Parity guard passes with 5 new commands + 2 new anchors.
   - Boundary invariant (raw bodies never cross local→committed).
   - Cross-feature isolation regression coverage.
   - All 5 Wave γ commits carry the Rule 18 co-author trailer.
3. Any findings routed via `docs/supervisor/LOG.md` per AGENTS.md.

## Blockers

None. Rev-0 landed cleanly.

## Context for Next Agent

- Rev-0 commits are LOCAL only. Supervisor pushes after dispatching
  review. Do NOT push from the implementer chair.
- `.tpatch/local/capture/` sits INSIDE the committed worktree —
  higher-risk of ADR-027 D1's three options. The six-mandate
  refusal contract is the ENTIRE safety margin. If a future change
  touches ANY of the five files below without updating a
  corresponding detached-worktree refusal fixture, that change is
  a regression:
    - `internal/workflow/session_ignore.go` (mandate wiring)
    - `internal/gitutil/ignore.go` (effective check, mandate 5)
    - `internal/cli/session.go` (session start pre-write verify)
    - `internal/cli/cobra.go` initCmd (mandate 1 + 2 amendment)
    - `internal/store/session.go` (LoadSession D18 refusal +
      PurgeSession symlink refusal)
- Slice 3 introduced a macOS-safe symlink-resolve in
  `PurgeSession` (`.tpatch/local/capture/` sits under
  `/var/folders/...` -> `/private/var/folders/...` on macOS temp
  trees). Both sides of the safety compare are now resolved. Real
  symlink escapes still refuse — the guard now requires the
  RESOLVED path to be outside the RESOLVED base.
- `TestFeaturePatchRefreshNoByteChangeSkips` was touched in Slice 5
  because `tpatch init` now dirties the working tree with the new
  `.gitignore` and the test previously assumed a clean baseline.
  Other tests that call `runCmd("init")` and then `record` in a
  git worktree pass because they don't do a byte-identical refresh
  round-trip; if you add such a test, mirror the two-line
  git-add / commit in `feature_patch_test.go` L163+.
- Wave α + Wave β acceptance is on `main`. You may freely use
  `store.DependencyKindSupersedes`, `store.ErrMultipleActiveSuperseders`,
  `IsFeatureSuperseded`, `PreimageHash *string`, and the
  `writefile_safety.go` primitives without re-export.

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
