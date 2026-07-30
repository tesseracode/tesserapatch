# Current Handoff

## Active Task

- **Task ID**: `v0.12.0-wave-gamma-active-feature-session-rev1`
- **Milestone**: v0.12.0 Wave γ — implement `PRD-active-feature-session` + honor `ADR-027` D1 F3 lock. Rev-1 fold-in of dual-review split findings.
- **Description**: Rev-0 dual review returned a SPLIT: internal APPROVED WITH NOTES (1 HIGH + 3 LOW), supervisor-external BLOCK (1 Critical + 4 HIGH + 1 MEDIUM). Zero overlap; both correct within their scope. Supervisor adjudicated at 2026-07-30 siding with external's contract-authority reading (PRD §4 D6 mandate 4 "Writers" plural, PRD §5 D11 "hard failure" verbatim, PRD §5 D9 `--write` as mutating mode verbatim, PRD §3 D4 no `closed→active` verbatim). Rev-1 folds ALL 10 findings.
- **Status**: Rev-1 dispatched 2026-07-30.
- **Assigned**: 2026-07-30.

## Supervisor adjudication (verbatim contract text)

The two-opinion protocol worked here — external caught contract-authority findings internal missed via a per-command audit that stopped at `session start`. Internal correctly caught the D14 safety-margin issue external missed. Both add net signal.

**Wave β rev-1 lesson applied by external** (correctly): "cross-reference PRD before concluding ADR silence." ADR-027 D1 states only the conditional; PRD-active-feature-session §4 D6 expands into six concrete mandates including the "Writers" plural clause at mandate 4. Reading both together exposed the later-writer D6 gap.

## Rev-1 scope (10 findings, LOCKED)

### F-EXT-γ-1 — CRITICAL — D6 later-writer bypass

- **Contract**: PRD §4 D6 mandate 4 verbatim: "Writers must refuse when Git is unavailable or the path is not ignored." Plural, unqualified.
- **Fix location**: `internal/cli/session.go:181` (session stop `SaveSession` call), `internal/cli/session_summarize.go` writer paths, ANY other caller of `Store.SaveSession` for session state.
- **Preferred implementation**: enforce `EnsureLocalIgnoreContract` inside `Store.SaveSession` (or a dedicated Session-only variant) so it's impossible to bypass by adding a new caller. Sentinel error already exists — reuse the six-mandate refusal message.
- **Regression tests REQUIRED** (detached-worktree fixtures, doctor Wave β D3 template):
  - `TestD6MandateWriter_SessionStopRefusesWithoutGitignore` — start session → `rm .gitignore` → `session stop` MUST exit non-zero with six-mandate message.
  - `TestD6MandateWriter_SessionSummarizeRefusesWithoutGitignore` — analogous for the summarize writer path.
  - Every other Session-state-write surface gets an analogous regression.
- **Rule 19 citation**: Slice R1 commit body MUST cite PRD §4 D6 mandate 4 verbatim.

### F-EXT-γ-2 — HIGH — Redaction hard failure exits 0

- **Contract**: PRD §5 D11 verbatim: "Redaction failure is a hard failure."
- **Fix location**: `internal/cli/session_summarize.go:116`.
- **Fix**: return non-nil error when `opts.Write` requested and redaction refuses. Use existing `promotion_refusal_reason` string as the error message text.
- **Test**: existing test that asserted exit-0 must be updated to assert non-zero exit + specific error type via `errors.Is` (mirror Wave β F-M1 sentinel pattern).

### F-EXT-γ-3 — HIGH — `session start --label` persists raw content

- **Contract**: ADR-027 D3 (redaction before persistence); PRD §7 D16 (forbid raw secret values / prompt-like content in local buffers).
- **Fix location**: `internal/cli/session.go:140` (label persisted verbatim into `session.json`).
- **Fix**: two options, choose one:
  - (a) Apply a redaction-scrub pass to the label before writing (reuse D11 redaction primitives from Slice 3).
  - (b) Reject labels that fail a "provably plaintext-safe" check (e.g., match secret-shaped tokens: `sk-`, `ghp_`, base64-length threshold, etc.).
- **Prefer (a)** if D11 primitives are reusable — else (b) with well-tested rejection patterns.
- **Regression tests**: `TestSessionStartLabelRedactsSecretShapedTokens` + happy-path preservation of safe labels.

### F-EXT-γ-4 — HIGH — `record --from-session` mutates before refusing

- **Contract**: general validate-before-mutate hygiene; refusal must not leave partial artifacts on disk.
- **Fix location**: `internal/cli/cobra.go:1524`.
- **Fix**: hoist the `--from-session` requires `--with-session` mutex validation to run immediately after flag parsing, BEFORE any capture / recipe generation / artifact write.
- **Regression test**: `TestRecordFromSessionRefusalLeavesNoArtifacts` — assert that after refusal, `artifacts/post-apply.patch` + `patches/001-record.patch` are absent (or their prior state is preserved for reruns).

### F-EXT-γ-5 — HIGH — Start-after-close reopens closed session

- **Contract**: PRD §3 D4 verbatim: "reopen is out of scope and valid transitions do not include `closed → active`."
- **Fix location**: `internal/cli/session.go:91` (only active sessions treated as existing).
- **Fix**: check for ANY existing session at the computed content-addressed ID (any state), and:
  - If `closed` / `promoted` / `purged` → refuse start with clear error citing D4.
  - If `active` → keep idempotent-existing behavior (already correct).
- Alternative: add an entropy input to session identity (e.g., a monotonic per-feature sequence) so a new session cannot collide with a historical one. **Prefer the refuse-on-collision path** — content-addressing is intentional per D3.
- **Regression test**: `TestSessionStartAfterCloseRefusesReopen`.

### F-EXT-γ-6 — MEDIUM — `--write` doesn't promote (D9 semantic mismatch)

- **Contract**: PRD §5 D9 rule 3 verbatim: "`session summarize` defaults to dry-run; `--write` is the mutating mode." D9 command listing shows `--write` as the promotion trigger (no `--promote` flag).
- **Fix location**: `internal/cli/session_summarize.go:77`.
- **Fix**: two options:
  - (a) Make `--write` perform the PRD promotion transition (state → `promoted`). Remove the `--promote` flag.
  - (b) Amend PRD §5 D9 to add the two-flag distinction consistently across command shape + acceptance criteria + skill assets.
- **Prefer (a)** — the PRD is Accepted and the split-flag semantic is not intuitive to users. If the implementer had a strong reason for splitting (e.g., "write context summary without committing to state transition"), document it in the rev-1 commit body and choose (b) with PRD amendment.

### F-INT-γ-1 — HIGH — `session purge --yes` with no args deletes ALL sessions

- **Contract**: PRD §6 D14 mutex on `--all` and `<slug>` implies "one of" semantics.
- **Fix location**: `internal/cli/session.go:296-363` (`sessionPurgeCmd`).
- **Fix**: after the existing mutex check, require that at least one of `--all` or `<slug>` is supplied. Refuse with clear error otherwise.
- **Regression tests**:
  - `TestSessionPurgeRefusesNoSlugNoAll` — assert non-zero exit + no filesystem mutation.
  - Preserve existing `--all` happy path.

### LOW findings (3, fold in the same rev-1 handoff commit)

**F-INT-γ-2 — LOW — Misleading `--session` vs `--from-session` in record ambiguity refusal**
- Fix: update the error message to name the correct flag `--from-session` (or verify D9 flag naming and correct whichever surface is stale).

**F-INT-γ-3 — LOW — `RepositoryIdentity == BaseCommit` in session ID identity inputs**
- Fix: distinguish `RepositoryIdentity` (stable per-repo identifier, e.g., first commit or worktree base) from `BaseCommit` (current HEAD at session start), OR remove one field from `SessionIdentityInputs` if the intent was for them to be the same.

**F-INT-γ-4 — LOW — `tpatch init` always prints `appended` even when rule was present**
- Fix: detect existing `.tpatch/local/` line in `.gitignore` and print `already present` (or `preserved`) instead of `appended`.

## Rev-1 slice plan (LOCKED)

1. **Slice R1 (F-EXT-γ-1 Critical, foundation)**: enforce D6 effective-ignore check at ALL session-write surfaces. Preferred: enforce inside `Store.SaveSession` or via a `SaveSessionWithIgnoreContract` wrapper mandatory for all callers. Multiple detached-worktree fixtures (stop, summarize, any other writer).
2. **Slice R2 (F-EXT-γ-2 High)**: D11 hard-failure returns non-nil error + `errors.Is` sentinel pattern.
3. **Slice R3 (F-EXT-γ-3 High)**: `session start --label` redaction/rejection.
4. **Slice R4 (F-EXT-γ-4 High)**: `record --from-session` early-validation hoist.
5. **Slice R5 (F-EXT-γ-5 High)**: session start refuses closed-collision.
6. **Slice R6 (F-EXT-γ-6 Medium + F-INT-γ-1 High + 3 LOW)**: `--write` promotion semantics (prefer option (a) — collapse `--promote` into `--write`). `session purge` requires `--all` or slug. LOW fixes.
7. **Slice R7 (paperwork)**: CHANGELOG `## v0.12.0 — TBD` `#### Wave γ rev-1 amendments` subsection (Wave α + Wave β subsections BYTE-IDENTICAL). Handoff refresh. If R6 chose PRD amendment path (F-EXT-γ-6 option b), also amend `docs/prds/PRD-active-feature-session.md` §5 D9 in this slice — otherwise no PRD amendment.

## Rev-1 validation gates

- Full gate set (`gofmt -l .`, `go vet ./...`, `go build ./cmd/tpatch`, `go test -count=1 ./...`).
- **Baseline 865 top-level PASS at rev-0. Rev-1 total MUST be ≥ 865 + 10-18** (Critical F-EXT-γ-1 needs multi-writer coverage: 3+ regression tests; each HIGH needs at least 1 regression test; some MEDIUM/LOW share test files).
- **Rule 20 REQUIRED for F-EXT-γ-1**: detached-worktree fixtures reproducing each writer surface's refusal path. Doctor Wave β D3 `--fix` refusal-fixture pattern is the template.
- **Rule 20 REQUIRED for F-EXT-γ-2 to F-EXT-γ-5**: empirical CLI reproduction that the refusal / redaction / early-validation / closed-collision path now behaves per PRD.
- **Rule 15**: no new commands in rev-1 (fold-in only). Parity guard test still passes.
- **Rule 18**: every rev-1 commit MUST carry `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.
- **Rule 19**: cite PRD clause verbatim in the commit body for each finding closed. Especially Slice R1 → cite PRD §4 D6 mandate 4 verbatim.
- Wave α + Wave β non-invalidation: `git diff --stat 561e6de..HEAD -- internal/workflow/labels.go internal/store/validation.go internal/cli/status_dag.go internal/workflow/writefile_safety.go internal/workflow/verify.go` MUST show only rev-0 code paths, not new rev-1 modifications (rev-1 is fold-in on session lane only).
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e`. Verify with `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.

## Two-opinion protocol scoreboard

**20/21 at rev-1 dispatch**. Wave γ rev-0 is the first genuine dual-BLOCK-caliber SPLIT in this session (Wave β rev-0 was a false-APPROVE by external corrected via adjudication; Wave γ rev-0 is a true two-scope-catches situation where both reviewers add net signal). Rev-1 will re-run the same dual-review protocol; expect both reviewers to confirm CLOSED on their originally-caught findings AND spot-check each other's fold-ins (mirror pattern of Wave β rev-1 external's Rule 20 compile-fail check).

## Files owned by rev-1 (do not touch Wave α or Wave β files)

Expected touch set:
- `internal/store/*.go` (Session store — F-EXT-γ-1 `SaveSession` wrapper).
- `internal/cli/session.go`, `session_summarize.go` (multiple findings).
- `internal/cli/cobra.go` (F-EXT-γ-4 record flag order).
- `internal/workflow/session_ignore.go` (F-EXT-γ-1 if wrapper landed here).
- New test files: `*_rev1_*_test.go` per slice.
- `CHANGELOG.md`, `docs/handoff/CURRENT.md` (R7).
- ONLY IF F-EXT-γ-6 option (b): `docs/prds/PRD-active-feature-session.md`. Prefer option (a).

## Session Summary — rev-0

Wave γ implementer rev-0 executed all 5 locked slices. SHA range `561e6de..d842697` (5 code commits + 1 LOG commit at `1ce37ff`). Full-suite PASS at 865 (baseline 827 + 38). All 5 commits carry parseable Rule 18 trailers. Wave α + β non-invalidation confirmed. Side Research md5 preserved. Rev-0 dual review dispatched in parallel; returned SPLIT: internal APPROVED WITH NOTES (1 HIGH F-INT-γ-1 + 3 LOW), external BLOCK (1 Critical F-EXT-γ-1 + 4 HIGH F-EXT-γ-2 to F-EXT-γ-5 + 1 MEDIUM F-EXT-γ-6). Supervisor adjudicated 2026-07-30 siding with external's BLOCK on contract authority. Rev-1 dispatched with all 10 findings folded.

## Files Changed — rev-0

Per implementer report at commit `d842697` — see LOG entry `1ce37ff` for detailed enumeration. Highlights: `internal/workflow/session_ignore.go` (new), `internal/cli/session*.go` (new), `internal/store/session*.go` (new), 6 shipped skill assets, CHANGELOG amendment, PRD flip Proposed→Accepted.

## Test Results — rev-0

Full-suite PASS at 865. Wave α non-invalidation confirmed (labels.go, validation.go, status_dag.go BYTE-IDENTICAL). Wave β non-invalidation confirmed. Side Research md5 preserved.

## Next Steps

1. Rev-1 implementer executes Slices R1–R7 per locked plan.
2. Rev-1 dual review dispatched on rev-1 commit range.
3. On three-way APPROVED → user-external pass → Wave γ consolidation → v0.12.0 ship.

## Blockers

None on rev-1 dispatch. The 10 findings are all folded into the locked slice plan.

## Context for Next Agent

- HEAD at rev-1 dispatch: `1ce37ff` (internal LOG entry landed on top of rev-0 code) + supervisor adjudication commit (this handoff commit) at HEAD.
- Rev-0 code range `561e6de..d842697` is on `HEAD` but NOT pushed (per Wave β pattern; consolidation pushes at three-way concurrence).
- 20 binding carry-forward rules unchanged.
- **Non-obvious decisions to know before implementing rev-1**:
  - F-EXT-γ-1 fix architecture: prefer enforcing the D6 check INSIDE `Store.SaveSession` (or a `SaveSessionWithIgnoreContract` wrapper that all Session-state callers must use), NOT scattering `EnsureLocalIgnoreContract` calls at each write site. Scattered enforcement is the exact pattern that let this bug ship at rev-0; a bottleneck enforcement makes future writer additions safe by construction.
  - F-EXT-γ-6 preferred option (a) — collapse `--promote` into `--write` per PRD D9 verbatim. Option (b) requires PRD amendment which is heavier (Rule 19 shipped-surface).
  - F-EXT-γ-5 fix path: refuse-on-collision is preferred over adding entropy. Content-addressing per D3 is intentional and reflected in the parity anchor.
  - PRD §4 D6's "Writers" plural clause is the source of authority — every present + FUTURE session-state writer must honor the ignore contract. Rev-1 test coverage should include a `TestD6_AllWritersRefuse` table-driven test that enumerates all Session-state-writing entry points, so adding a new writer surface later fails until it goes through the enforced path.
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
