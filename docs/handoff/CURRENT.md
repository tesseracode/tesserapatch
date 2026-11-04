# Current Handoff

## Status

**Cluster state**: APPROVED

**GH #9 implemented and review-APPROVED; validation running.** The
independent review (OpenAI model) approved the change after one MEDIUM
finding was fixed forward, and the guarded old-Mac focused checks (`rev1`)
passed. Source commit `b8a5f37` is pushed; native CI run 41873614423 and the
fresh guarded old-Mac full gate (launched 17:04:35Z) are running against it.
GH #9 stays OPEN until both pass. The operator approved a
12-hour window on 2026-11-04, with an optional read-only v0.18 reproduction
of GH #25/#26 only if GH #9 is complete and time remains. GH #13 is
CLOSED/completed and v0.18.0 is published; neither is reopened by this window.

## Active Task

- **Task ID**: `gh9-nested-repo-sentinel`
- **Milestone**: Post-v0.18 reliability — GH #9
- **Description**: Make wave-close `[2/8]` see source files inside untracked nested Git repositories, preserving allowlist behavior, never descending into `.git/`, with a failure-proving fixture
- **Status**: Review APPROVED; focused checks PASS; source `b8a5f37` pushed — native CI and fresh old-Mac full gate running
- **Assigned**: 2026-11-04
- **Window**: 2026-11-04T16:16:08Z → **2026-11-05T04:16:08Z** (12 hours, operator-approved; no silent extension)
- **WAVE_BASE**: `064bc381dedd07fb2125ef1a75e639c01a841555`

Scope: GH #9 only, then optionally GH #25/#26 read-only reproduction on the
latest release. Out of scope: GH #17, migration/coverage contracts
(GH #18-22/#24), research tracks, and the gate-time reduction experiment
(recorded in ROADMAP as a follow-up after GH #9, not started here).

## Decisions (for operator review)

- **GH9-D1 — Latest-main health at dispatch.** CI 41859120038 at `064bc38`
  had its two macOS jobs cancelled by GitHub ("job was not acquired by Runner
  of type hosted even after multiple attempts", capacity notice, zero steps
  executed). `064bc38` changes only four tracking documents relative to
  `0fc232d`, whose CI 41844293141 passed all five required jobs. Latest-main
  source health is therefore taken from 41844293141; the capacity-cancelled
  run is not re-run, and this dispatch push supersedes it. GH #9's own source
  commit must still receive a complete five-job native CI pass.
- **GH9-D2 — `[2/8]` moves into `scripts/wave-close-untracked-sentinel.sh`;
  the 18 source pathspecs stay in the Makefile as arguments.** The fix needs
  temp files, loops and status checks that do not fit a `\`-continued recipe.
  Keeping the pathspecs in the Makefile preserves the S7 shard-parity guard
  (`'scripts/**'` exactly once, no recursive `$(MAKE)`). Exit contract:
  0 OK, 3 WARN, anything else FAIL (the Makefile maps 3 to `warn=1` and any
  other non-zero status to an explicit FAIL line plus `fail=1`).
- **GH9-D3 — Transparent-boundary supplement, not a per-boundary WARN.**
  Alternatives considered: (a) WARN on every untracked nested repository
  boundary — rejected, it fires on harmless scratch clones and cannot be
  allowlisted per file; (b) run `git ls-files` inside each nested repository —
  rejected, it executes the nested repository's config/hooks/fsmonitor and
  applies the wrong ignore rules. Chosen: list files inside each boundary with
  `find`, pruning every `.git` entry (Git never runs inside the nested
  repository and its internals are never read), load the paths into a
  throwaway index and let the parent's exclude rules (including `.gitignore`
  files on the path) and Git's own pathspec matcher classify them, exactly as
  if the nested `.git` were absent. Top-level output is unchanged for
  checkouts without untracked nested repositories; nested matches are merged
  with `LC_ALL=C sort -u` only when present (differential old-vs-new run:
  byte-identical on every fixture without a nested repository).
- **GH9-D4 — Fail closed.** The old recipe's `2>/dev/null` turned Git errors
  into a false `OK`. Git failures, rejected allowlist patterns, a non-top-level
  working directory, quoted (control-character/quote/backslash) boundary
  names and file names containing newlines now FAIL.
- **GH9-D5 — Allowlist entries are Git pathspecs, never shell globs.**
  `set -f` before the unquoted `$allow_patterns` expansion. Previously a
  pattern such as `*.go` was also shell-globbed against the cwd. No live
  impact: the allowlist has zero entries.
- **GH9-D6 — The new Go tests skip on Windows.** `wave-close-check` is a
  POSIX make/sh gate that never runs on Windows; skipping keeps the GH #17
  known-red Windows failure-identity set unchanged.
- **GH9-D7 — No CHANGELOG entry.** There is no Unreleased convention and
  this is contributor process tooling, not shipped CLI behavior.
- **GH9-D8 — Throwaway-index isolation.** The scratch index is invoked with
  `core.splitIndex=false`, `core.fsmonitor=false` and
  `core.untrackedCache=false`, so nothing is written beside the real index
  and no watcher is consulted.
- **GH9-D9 — Freeze the closed GH #13 stage fence at its closeout head.**
  `TestROCS0NoProductionChanges` diffed the *live* tree against the ROC S0
  baseline `bbb7a43` and rejected every path outside the GH #13 S0-S9
  allowlists (which explicitly reject `Makefile` and `.wave-close-allowlist`),
  so any post-GH #13 work — GH #9 included — could not land. Following the
  PIB frozen-range precedent (`prepare_pib_golden_test.go`), the fence now
  validates `bbb7a43..064bc38` (`rocClosureCommit`, the GH #13 closeout head
  and this window's WAVE_BASE). At `064bc38` the live and frozen diffs are
  identical, so the cluster's scope proof is unchanged. New
  `TestROCS0ClosedStageScopeSensitivity` proves the frozen record still
  rejects unregistered surfaces and that the anchor rejects closures that
  truncate the record (`bbb7a43`, `c18f025`); the anchor also requires the
  S9 terminal commit `5133c95` and ancestry to HEAD. The capture-mode check
  (only under `TPATCH_UPDATE_ROC_S0_GOLDENS=1`, never set) and the
  memo-scope file checks are untouched.
- **GH9-D10 — No ADR.** Gate tooling changes have been documented in the
  AGENTS.md "Mechanical gate" paragraph (allowlist 2026-08-05, sharding
  2026-08-29); GH #9 follows that precedent, with the alternatives recorded
  in GH9-D3.
- **GH9-D11 — Opaque boundary records are replaced, not kept (review
  finding R1, MEDIUM).** A nested repository under a source pathspec (e.g.
  `internal/repo/`) is also reported by the top-level query as the opaque
  record `internal/repo/`. Keeping that record meant allowlisting or ignoring
  the real files still WARNed. Every inspected boundary record (plain
  `dir/` or C-quoted `"dir/"`) is now dropped from the candidates before the
  nested files are merged, matching GH9-D3's "as if `.git` were absent"
  model. Consequences: directory-style allowlist entries written for the old
  record (`internal/repo/`) still cover the files inside; an untracked nested
  repository with no matching files no longer WARNs. The allowlist side is
  deliberately not filtered: `allowed` only filters `candidates`, so a stale
  boundary entry there is inert, and a filter would be unguardable code.
- **GH9-D12 — Tracking pushes are held while source CI runs.**
  `.github/workflows/ci.yml` uses `concurrency: cancel-in-progress: true`
  per ref, so any push to `main` cancels the running CI for `b8a5f37`.
  Tracking updates are committed locally as they happen and pushed only
  after run 41873614423 reaches a terminal state. The gate is unaffected:
  `[3/8]` already verified HEAD == `origin/main` at launch, and `run-gate.py`
  only re-checks the clone's local HEAD.
- **GH9-D13 — The GH #25/#26 reproduction ran during the validation wait.**
  It was local and read-only and could be preempted by any GH #9 fix-forward.
  It used a v0.18.0 binary built from `git archive v0.18.0` under the guarded
  60 s admission, in the ignored directory `bin/gh9-20261104/repro-v018/`.
  The fixtures were fresh public clones of `tesseracode/t3code` checked out
  detached, plus a disposable repository for the minimal #25 case. Verify ran
  with `--json --no-write`, and every fixture worktree was confirmed clean
  afterwards. The operator's installed `tpatch` (v0.16.0) was not touched.
- **GH9-D14 — No external posting.** Results, drafted comment text and the
  new finding (F-V18-1) are recorded here and in
  `bin/gh9-20261104/repro-v018/DRAFTS.md`. No issue comments were posted and
  no issues were filed. The operator decides whether to post or file them.
- **GH9-D15 — F-V18-1 is recorded, not fixed, in this window.** It is outside
  GH #9 scope. The likely fix is to remove `"version": 1` from the docs
  example and add a guard that the authoritative example decodes strictly.
  It is small but needs its own review and CI cycle. A push while source CI
  runs would also cancel it (GH9-D12). It is listed as an unfiled backlog
  candidate in ROADMAP.

## Session Summary

GH #13 was closed as completed at 2026-11-04T15:15:52Z after the operator's
account change. The post-v0.18 backlog was reconciled and archived at
15:32:06Z (ROADMAP "Current Backlog — Post-v0.18.0"). The operator then
approved this 12-hour GH #9 window and asked for a follow-up backlog item to
halve old-Mac full-gate time without starving the machine.

Dispatch baseline verified at 16:22:12Z: HEAD = origin/main = `064bc38`,
clean tree, no stash, one worktree, no merge/rebase/cherry-pick/index lock,
zero untracked files.

Reproduced GH #9 in a scratch repository: the old `[2/8]` query printed
`OK` while `scratch-review/repo/internal/example.go` sat inside an untracked
nested repository (Git lists only `scratch-review/repo/`). Implemented the
script-based sentinel (GH9-D2…D8), shellcheck-clean in `sh` and `dash`
modes; the shell fixture matrix gives identical results under bash-as-sh
and dash. A differential run of the old recipe (extracted from `064bc38`)
against the new script was byte-identical for clean, several-files,
partial/full/no-match allowlist and nested-under-pathspec fixtures; only the
GH #9 reproduction changes (old `OK` → new WARN). Found and fixed the
GH #13 live stage fence that would have rejected any post-GH #13 change
(GH9-D9).

## Current State

- GH #9: OPEN, implemented and review-APPROVED; native CI and full gate pending.
- GH #13: CLOSED/completed; v0.18.0 (release 433986008) published, tag object
  `648afeeee5fcacffdf871177ad022406074b51c3` unchanged.
- GH #17: OPEN; full Windows suite remains known-red; not in scope.
- GH #25/#26: OPEN; both still reproduce on v0.18.0 (read-only, GH9-D13).
- F-V18-1: the docs example `"version": 1` makes a recipe undecodable;
  unfiled (GH9-D14/D15).
- `[2/8]` now runs `sh scripts/wave-close-untracked-sentinel.sh <18 pathspecs>`;
  source inside untracked nested repositories WARNs unless allowlisted.

## Files Changed

- `scripts/wave-close-untracked-sentinel.sh` (new) — the `[2/8]` sentinel.
- `Makefile` — `[2/8]` calls the script and maps its exit status; GH #9 comment.
- `internal/cli/wave_close_sentinel_test.go` (new) — 20 behavior cases,
  14 script mutations, Makefile-recipe wiring with 4 mutations.
- `internal/workflow/operation_candidate_s0_guards_test.go` — GH9-D9 frozen
  GH #13 stage fence plus `TestROCS0ClosedStageScopeSensitivity`.
- `AGENTS.md` — Mechanical gate paragraph: nested-repository supplement.
- `.wave-close-allowlist` — header comment on nested-path matching (still
  zero entries).
- Tracking: `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`, `docs/supervisor/LOG.md`.

## Test Results

Shell-level (no Go): shellcheck `-s sh` and `-s dash` clean; fixture matrix
under `sh` and `dash` as designed (evidence `bin/gh9-20261104/shell-matrix.txt`,
`differential-old-vs-new.txt`). Guarded old-Mac focused run `rev0` (source `aa98763` + 6-file overlay):
gofmt clean (no formatter changes); S7 shard-parity, CI-timeout and
AR-claims guards PASS; `TestGH9WaveCloseUntrackedSentinel` 14/14 PASS;
**`TestGH9WaveCloseUntrackedSentinelMutations/git-dir-prune-removed` FAIL**
— the mutation survived. Root cause: `git update-index --index-info` does not
reject `.git` path components, it skips them with an `Ignoring path` warning
on stderr (Git 2.50.1 and 2.55.0 alike), and the harness compared stdout
only, so the guard was vacuous. Fix (rev1): OK/WARN runs must have empty
stderr, and a new `git-internals-never-traversed` fixture places an
unreadable directory inside the nested `.git`, so any traversal of Git
internals is a hard `find` error; a new mutation also covers the
newline-scan `find` prune, which was previously unguarded. Because `rev0`
ran `-failfast`, the remaining mutations had not executed: all 11 script
mutations and 4 Makefile wiring mutations were then verified at shell level
(no Go) under `sh` and `dash` locally and under `sh` with Git 2.50.1 on the
old Mac. ROC workflow tests, vet and build did not run in `rev0`.

Independent review (OpenAI `gpt-6.1-sol`, code-review agent): NEEDS
REVISION with one MEDIUM finding (R1 → GH9-D11: opaque boundary records
under a source pathspec survived allowlisting/ignoring of the files inside);
fixed forward with 5 new fixtures and 3 new mutations; delta re-review
**APPROVED**, no findings. Pre-Go shell harness after the fix: 17 replicated
cases and 14/14 script mutations caught under `sh`/`dash` with Git 2.55.0
and under `sh` with Apple Git 2.50.1.

Guarded old-Mac focused run **`rev1` PASS** (17:05Z; source `aa98763` +
6-file overlay, overlay sha256 `2601d638…`, files identical to this commit):
gofmt clean (no formatter changes); `internal/cli` (no `-failfast`):
`TestGH9WaveCloseUntrackedSentinel` 20/20, `…Mutations` 14/14,
`…MakefileWiring` 4/4 mutations (the unreadable-directory fixture ran, not
skipped), `TestS7WaveCloseShardParity`, `TestS7CIFullSuiteTimeoutGuard`,
`TestS7ARPermanentBlockClaimsGuard` PASS; `internal/workflow`:
`TestROCS0NoProductionChanges`, `TestROCS0ClosedStageScopeSensitivity`,
`TestROCS0ScopeGuardSensitivity`, `TestROCS[2-9]ScopeTransition` PASS;
`go vet ./...` and `go build ./cmd/tpatch` PASS; no blocked marker.
Evidence archive `bin/gh9-20261104/rev1-evidence.tar.gz` (sha256
`eb537483…`). All Go and gofmt runs use the accepted guarded wrappers (60 s
normal-pressure admission, no new swap-outs, load1 <= 5, serial private Go
1.26.5 with the accepted environment).

Fresh guarded old-Mac full gate at source `b8a5f37` **launched**
2026-11-04T17:04:35Z (runner PID 82853, identity verified; fresh clone of
bundle sha256 `c3853d89…` fetched to HEAD == `origin/main`; 6 reviewed
hashes, `HISTORY.md` prefix, 9 routing controls and exact 42/39 command
parity verified before launch; fresh caches;
`WAVE_BASE=064bc381dedd07fb2125ef1a75e639c01a841555`). Early gate steps on
the committed tree: `[1/8]` OK, `[2/8]` OK (the new sentinel on a real
checkout), `[3/8]` OK, `[4/8]` OK (2 commits), `[5/8]` OK (`APPROVED`).
Native CI run 41873614423 at `b8a5f37` in progress.

### GH #25/#26 reproduction on v0.18.0 (GH9-D13)

The v0.18.0 binary (commit `a14de67`, sha256 `b3bc68ad…3ebd`) was run with
local Git 2.55.0. All evidence is hashed in
`bin/gh9-20261104/repro-v018/SUMMARY.json`.

- **GH #26 still reproduces on v0.18.0.**
  - At t3code `5ce5d79`, `copilot-cli-provider` exits 0. Four other roots
    exit 2 at `recipe_replay_clean` with `failed_at: parent-landing-drift`:
    `copilot-sdk-license-notice`, `copilot-runtime-packaging`,
    `phase1-foundation` and `attention-notifications`.
  - The current anchor is `materialized-clean`; the historical V8/V10 checks
    are skipped.
  - `attention-notifications` still selects the historical base `75bfc26`
    and replay landing `55fd4d7`, not the genuine new base `19785c6`.
  - The diagnostic still says to verify the parent first, but the parent
    already passes, so the advice does not help.
  - The v0.1.4 port case in the issue comment was rerun at source `16d428d`,
    the commit where the archived v0.16.0 summary was produced. All 11 roots
    are identical to v0.16.0 in exit code, `failed_at` and the set of
    blocking checks.
  - New in v0.18: each legacy root gets a warn-only
    `recipe_generation_coverage` (`recipe-coverage-missing`) result. It does
    not change the exit code.
- **GH #25 still reproduces on v0.18.0.**
  - Minimal disposable case (`r25.sh`): before the child lands, the parent
    exits 0. After the child lands, the parent exits 2 with `failed_at:
    landed-content-absent`. Landing evidence is `exact`;
    `recipe_replay_clean` and `write_file_preimage_fresh` pass;
    `post_apply_patch_replay_clean` fails. The child exits 0, and the node
    behavior check passes.
  - Both features now record with `recipe coverage: complete` (D16
    derivation), which does not change the parent result.
  - At the Fusion fixture t3code `184d025`, `copilot-fusion-model-selection`
    exits 2 with `parent-landing-drift` on `copilot-cli-provider`, which
    matches the archived v0.16.0 `nativeVerify` refusal.
- **New finding F-V18-1 (unfiled):** the documented example recipe is
  rejected.
  - The example in `docs/agent-as-provider.md:80` ("apply-recipe.json schema
    (authoritative)") includes a top-level `"version": 1`. It has done so
    since v0.4.3 (`45d0589`).
  - `ApplyRecipe` has never had that field, and no tpatch writer emits it.
  - The strict decoder rejects it. Doctor D7 has flagged it since `daf2e6f`.
    Since v0.17.0, `implement --manual` publishes `recipe-undecodable` and
    `apply --mode execute` refuses with `recipe-generation-incomplete`.
  - Observed on v0.18.0 (`out25/attempt1-version-field.log`). Unchanged on
    `main`.
  - Only hand-written or agent-written recipes that follow the docs are
    affected. The shipped skills do not show a `version` field.
  - A recipe whose `preimage_hash` was bare hex (no `sha256:` prefix) was
    also rejected. The skills document the prefixed form, so that was my
    mistake in the test script, not a finding.

## Next Steps

1. Observe native CI run 41873614423 (five required jobs; audit the
   known-red Windows full suite against the GH #17 baseline).
2. Observe the old-Mac full gate (`[6/8]`-`[8/8]`; ~3h40m from launch).
3. Close GH #9 only after native CI and the gate both pass. Then do the
   closeout tracking: LOG, ROADMAP ✅, HISTORY archive, and Cluster state
   SHIPPED.
4. The GH #25/#26 v0.18 reproduction is done (see Test Results). For the
   operator: decide whether to post the drafted issue comments and whether
   to file F-V18-1 (`bin/gh9-20261104/repro-v018/DRAFTS.md`).

## Blockers

None.

## Context for Next Agent

The old-Mac harness (`run-gate.py`) runs `make wave-close-check` and requires
the exact line `=== Mechanical gate: PASS ===`; `[2/8]` must therefore stay
clean (not WARN) on a normal checkout. The shard-parity guard
(`internal/cli/prepare_s7_ci_timeout_guard_test.go`) requires `'scripts/**'`
exactly once in the Makefile and no recursive `$(MAKE)` in `wave-close-check`.
Private evidence for this window goes under `bin/gh9-20261104/` (ignored).
