# Current Handoff

## Status

**Cluster state**: REV-3 DISPATCHED

v0.15.1 Wave A rev-2 remains blocked by land's post-record discovery and
quoted patch-header parsing in refresh. Rev-3 is dispatched.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-3
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record/reconcile capture and land
  planning.
- **Status**: In Progress
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Rev-2 dispatch HEAD**: `79a6c2b`
- **Target release**: v0.15.1

## Rev-1 finding closures

### F1 (HIGH) — newline fallback intrinsically ambiguous — CLOSED

The fallback is **removed**, not tightened. The reviewer's counter-example
is exact: a worktree path containing a newline whose continuation is
shaped like a valid attribute (`locked x`, `bare`, `HEAD <sha>`) parses
as a well-formed record, so structural validation can never distinguish
it and the worktree escapes the filter.

`listRegisteredWorktreePaths` now runs `git worktree list --porcelain -z`
and nothing else. Deleted from production: `parseWorktreeListLegacy`,
`decodeLegacyWorktreePath`, `unquoteCStyle`, `legacyAmbiguityError`,
`worktreeAttrKeys`, `isUnknownSwitchError`. No quarantined copy was kept
— there was no tested reason to keep one.

### F2 (HIGH) — bare `usage:` accepted as proof `-z` is unsupported — CLOSED

Moot by construction: stderr is no longer classified at all. Every
failure — unknown switch, usage error, broken repository, missing git —
returns the wrapped `ErrNestedWorktreeDiscovery` with guidance naming
the Git 2.36 requirement and stating that tpatch will not fall back.

### F3 (HIGH) — reconcile refresh bypassed the exclusions — CLOSED

`DiffFromCommitForPaths` now:

- discovers nested prefixes/excludes **first**, before the temp index is
  created or any `git add -N` runs, so a discovery failure cannot leave a
  half-mutated temp index (and, at the caller layer, cannot happen after
  an artifact write);
- filters the caller's paths before the intent-to-add pass;
- appends `:(exclude,literal)` pathspecs to the diff so index residue
  cannot re-admit the worktree;
- returns an **empty diff** when every caller path was a nested worktree,
  never a broadened full-tree diff;
- with an empty caller list, still produces the full diff minus nested
  worktrees.

The global `--literal-pathspecs` flag on the diff is replaced by the
per-pathspec `:(literal)` form — identical semantics for caller paths
(pinned by `TestDiffFromCommitForPathsKeepsLiteralPathSemantics` using a
`weird[1]*name.txt` fixture) while leaving pathspec magic enabled so the
excludes are honoured. Linked-worktree effective-index behavior is
unchanged and pinned by two tests.

`RefreshAfterAccept` derives its touched-path set through the new
`gitutil.FilterPathsExcludingNestedWorktrees`, and that one filtered list
feeds the regenerated diff, the numbered `NNN-reconcile.patch` **and**
`Capture.Pathspecs` in the generation metadata — the metadata previously
re-derived from the unfiltered original patch. When every original path
was a nested worktree it regenerates nothing rather than widening scope.

### F4 (HIGH) — writes before a later discovery failure — CLOSED

`runApplyDone`: the canonical patch and the diffstat are both computed
before the first write. The non-reapply branch also defers its
`could not capture patch` warning until after discovery, so the message
order on the success path is unchanged. The reapply branch computes the
diffstat before falling through to the shared write.

`record`: the diffstat is hoisted above `WriteArtifact(post-apply.patch)`
and the numbered snapshot. `record.md` consumes the same hoisted value.

Both route through the new `captureDiffStatFailClosed`, which preserves
the historical tolerance (a transient Git failure yields an empty
diffstat and the artifact is skipped) while surfacing the discovery
class.

**Discovery-count reduction.** Investigating `cycle` showed
`CapturePatchScoped` was itself discovering twice (once for excludes,
once inside `listUntrackedFiles`). Each capture helper now discovers
exactly once and threads both products through
(`listUntrackedFilesWithPrefixes`, `stagedNameOnly`/`unstagedNameOnly`
taking excludes, `untrackedFiltered` taking prefixes). A single command
can no longer observe two different answers, and the failure window
before the first write is minimal.

`cycle` inspected: its only discovery-dependent read is the [6/6]
capture, and every record-phase write follows it. Proven by the seam
test — a discovery failure leaves no `post-apply.patch`,
`post-apply-diff.txt` or numbered patch, and does not advance the feature
to `applied`. The earlier phases' analysis/spec/exploration/recipe
artifacts are not discovery-dependent.

`land` inspected: it delegates capture to the embedded `record` (now
transactional) and performs its own discovery in `computePathSet` /
`dirtyPaths`, both strictly before `stagePathSet` and the commit. A late
discovery failure therefore leaves the index and HEAD untouched; pinned
by `TestNestedWorktree_SecondDiscoveryFailure_LandDoesNotStageOrCommit`.

## Files Changed

Created this rev:

- `internal/cli/nested_worktree_guard.go` — CLI-side GH #7 guards
  (`captureDiffStatFailClosed` plus the two empty-capture-diagnostic
  helpers moved out of `record_capture_modes.go`).

Modified this rev:

- `internal/gitutil/worktrees.go`
- `internal/gitutil/capture_modes.go`
- `internal/gitutil/gitutil.go`
- `internal/gitutil/worktrees_test.go`
- `internal/cli/cobra.go`
- `internal/cli/record_capture_modes.go`
- `internal/cli/nested_worktree_test.go`
- `internal/workflow/refresh.go`
- `internal/workflow/refresh_test.go`
- `CHANGELOG.md`
- `SPEC.md`
- `docs/record.md`
- `docs/handoff/CURRENT.md`

Unchanged this rev: `internal/cli/land.go`, `internal/cli/phase2.go`,
`docs/land.md`.

Deliberately NOT folded: the Makefile nested-repo sentinel LOW is
separately tracked.

## Test Results

- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean.
- `go test -count=1 ./...` — all packages `ok`.
- `go test -race -count=1` on `./internal/gitutil/`, `./internal/workflow/`
  and `./internal/cli/` — ok.
- Assets parity (`go test ./assets/`) — ok; no asset touched.
- 82 passing assertions across the GH #7 test set.

New coverage this rev:

- `TestListRegisteredWorktreePaths_NoLegacyFallback` — the failure text
  names `--porcelain -z`, `2.36` and `will not fall back`, and is in the
  fail-closed class.
- `TestNoLegacyPorcelainParserInProduction` — grep-level guard: the
  production source contains no `parseWorktreeListLegacy`,
  `isUnknownSwitchError`, `unquoteCStyle` or plain `"--porcelain")`
  call, and every `worktree list` argv carries `-z`.
- `TestNestedWorktreePrefixes_NoZRejectionFailsClosedWithoutRetry` — a
  `git` wrapper rejects `-z` like pre-2.36 Git **and logs every
  `worktree list` invocation**; exactly one invocation is recorded, it
  carries `-z`, and `NestedWorktreePrefixes`, `CapturePatchScoped`,
  `CaptureDiffStat`, `DiffFromCommitForPaths` and
  `FilterPathsExcludingNestedWorktrees` all fail closed with no plain
  `--porcelain` retry.
- `TestDiffFromCommitForPathsExcludesNestedWorktree` /
  `...WorktreeOnlyScopeReturnsEmpty` /
  `...EmptyScopeStillExcludesNestedWorktree` /
  `...PreservesLinkedWorktreeIndexAfterFiltering` /
  `...KeepsLiteralPathSemantics`.
- `TestRefreshAfterAcceptExcludesStaleNestedWorktreePaths` — a stale
  mode-160000 entry in the ORIGINAL patch is absent from the refreshed
  canonical patch, the numbered reconcile patch and
  `patch-generations.json`, while `feature.txt` / `README.md` remain.
- `TestRefreshAfterAcceptWorktreeOnlyOriginalPatchRegeneratesNothing` —
  an unrelated real change is NOT swept in by a broadened diff.
- `TestRefreshAfterAcceptFailsClosedBeforeMutation` — feature directory
  byte-identical across the refusal.
- `TestNestedWorktree_SecondDiscoveryFailure_{ApplyDoneNoMutation,
  RecordNoMutation,LandDoesNotStageOrCommit}` — an Nth-call-failing
  `git` wrapper (file-backed counter, so it survives the many
  short-lived `git` processes one tpatch command spawns) proves
  first-success/second-failure leaves the feature directory
  byte-identical and `status.json` unadvanced, across the numbered
  snapshot branch, the same-feature duplicate branch and the scoped
  branch, plus recovery in each case.
- `TestNestedWorktree_CycleSingleDiscovery` — no record-phase artifact,
  no state advance on failure; one successful discovery completes the
  run.

Retained unchanged: the NUL exotic-name coverage
(`TestParseWorktreeListNUL_PreservesPathBytes`,
`TestNestedWorktreePrefixes_ExoticNamesRealRepo`,
`TestNestedWorktreePrefixes_NewlineNameRealRepo`) and the actionable
all-filtered record diagnostic suite.

## Reproduction + control matrix (built binary)

One scratch repo with three nested worktrees (`agent review`,
`agent trail ` with a trailing space, and a newline-named one) plus every
over-filtering control:

| Path | Kind | Result |
|------|------|--------|
| `.claude/worktrees/agent review` | nested worktree | absent from patch, diffstat, land plan, commit |
| `.claude/worktrees/agent trail ` | nested worktree, trailing space | absent from all four |
| `.claude/worktrees/new\nline` | nested worktree, embedded newline | absent from all four |
| `.claude/worktrees/agent-other/f.txt` | ordinary dir, prefix-boundary sibling | captured and landed |
| `vendor/plainrepo` | unregistered nested Git repo | captured and landed as a gitlink (correctly NOT filtered) |
| `../extwt` | worktree outside the root | never referenced |
| `unrelated.txt` | ordinary dirty path | refused, then staged under `--allow-extra-paths` |

Post-land `git status` lists only the three worktrees as untracked and
the carved-out `.tpatch/FEATURES.md`. The actionable all-filtered
`record --files` diagnostic still fires for the space-bearing and the
newline-bearing worktree names.

All scratch repos, worktrees and build artifacts were removed;
`git worktree list` shows only the primary worktree.

## Reviewer focus

1. `DiffFromCommitForPaths` dropped the global `--literal-pathspecs` in
   favour of `:(literal)` per pathspec. Confirm this is semantically
   identical for every caller (`RefreshAfterAccept` file lists,
   `validateReapplyMaterialization`'s `PathsAffectedByPatch`), and note
   the `weird[1]*name.txt` regression that pins it.
2. The record diffstat is now captured before the artifact writes, so it
   no longer observes tpatch's own writes. In a repository where
   `.tpatch/features/<slug>/` is TRACKED, a re-record's
   `post-apply-diff.txt` no longer lists the artifact rewrite it is about
   to perform. This is an intentional consequence of the F4 reorder; no
   test asserted the old bytes.
3. Capture helpers now discover once and thread prefixes/excludes.
   Confirm no path lost its filter in the refactor —
   `stagedNameOnly`/`unstagedNameOnly` take excludes, `untrackedFiltered`
   takes prefixes, `StagedUnstagedOverlap` discovers once for all three.
4. Known limitation (pre-existing, not a GH #7 regression): `--files`
   `TrimSpace`s each comma-separated element, so a worktree whose name
   has significant leading/trailing whitespace cannot be named through
   `--files` and therefore does not trigger the actionable diagnostic.
   Exclusion itself still works — the guard matches the real path bytes
   from `ls-files`, not the flag text.
5. `land`'s transactional boundary is "embedded record artifacts may
   exist; index and HEAD are untouched". That is inherent to land
   delegating to a complete `record` invocation.

## Rev-2 Review Adjudication

- Internal: NEEDS REVISION.
- External/original reproducer: APPROVED.
- Valid residuals:
  1. Land's embedded record mutates feature artifacts/status before later
     `computePathSet`/dirty-path discovery can fail.
  2. `FilesInPatch` skips C-quoted newline paths; refresh can mistake a
     stale worktree-only patch for empty scope and capture the full tree.
- `tpatch_rev2_bin` and review scratch are absent after external cleanup.

## Next Steps

1. Cache land's nested-worktree discovery before embedded record and reuse it
   for all later planning.
2. Add strict Git diff-header parsing with C-quote decoding and fail-closed
   refresh behavior.
3. Add Nth-call land failure and quoted-newline stale-patch regressions.
4. Run final dual review, then close #7 only after approval.

## Blockers

None.

## Context for Next Agent

- `internal/gitutil/worktrees.go` is the single authority, and
  `git worktree list --porcelain -z` is the single Git shape. Do not
  reintroduce a newline-delimited reader: it cannot be made unambiguous.
- Git 2.36+ is now a documented safety floor for this guard
  (`SPEC.md` §9.4, `docs/record.md`, CHANGELOG).
- Byte exactness remains load-bearing: no `TrimSpace`, no hand-rolled
  dequote on any path compared against a worktree prefix.
- Any new discovery-dependent read must complete before its command's
  first artifact write, and should reuse an already-discovered prefix
  set rather than discovering again.
- `PreflightReconcile` is still deliberately unfiltered: it is a hygiene
  gate, not a capture surface, and a nested worktree surfacing there
  produces a refusal, which is the safe direction.
- Side Research md5 `b385fe622db9926f48861105239f113e` preserved.

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
