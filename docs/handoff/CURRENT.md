# Current Handoff

## Status

**Cluster state**: REV-4 DISPATCHED

v0.15.1 Wave A rev-3 remains blocked by strict-parser validation gaps and
land cache staleness before staging. Rev-4 is dispatched.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-4
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record/reconcile capture and land
  planning.
- **Status**: In Progress
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Rev-3 dispatch HEAD**: `3f5363c`
- **Target release**: v0.15.1

## Rev-2 finding closures

### F1 (HIGH) — land discovery transaction — CLOSED

`runLand` now discovers **once**, at entry, immediately after the
dependency gate and **before** `snapshotMetadataFiles` and the embedded
`record`. The resulting prefix set is immutable for the rest of the
command and is threaded through `computePathSet` and `dirtyPaths`, both
of which lost their own discovery calls. No `git worktree list` runs
after the embedded record begins.

`runLandDryRun` follows the same shape: one discovery at entry, threaded
through the path-set and dirty-path computations.

The embedded `record` keeps its own fail-closed discovery — that is its
own transaction — but once it succeeds, land's planning never asks Git
again, so a discovery failure can no longer leave record's artifacts
behind while land refuses.

**Status-notes reorder.** `status.json:notes` used to be written before
`computePathSet` specifically so the freshly dirty `status.json` would be
swept up by the slug-prefix branch. That made every later refusal —
malformed patch, extras, dirty-path classification — leave a
`landed at ...` note behind. The path set now names
`.tpatch/features/<slug>/status.json` explicitly (`includeStatus`), so
the write moved below the last refusal. Both the real-land and dry-run
paths pass `includeStatus=true`, keeping the dry-run plan faithful to
what land will actually stage.

Verification is calibrated, not hardcoded:
`measureRecordDiscoveryCalls` runs a standalone `record` on an identical
fixture under a counting `git` wrapper, then the land test configures
the wrapper to fail after `1 + recordCalls` invocations. Land must
**succeed** — only possible if it makes zero post-record discovery calls
— and the observed total must equal `1 + recordCalls` exactly.

### F2 (HIGH) — strict `diff --git` header parsing — CLOSED

New `gitutil.FilesInPatchStrict` (`internal/gitutil/patch_paths_strict.go`).
It resolves each entry's b-side path from the `diff --git` operand when
that is unambiguous (both operands quoted, or both unquoted with
byte-identical payloads), and otherwise from the unambiguous
corroborating headers `rename to` / `copy to` / `+++` / `---`. Quoted
fields are decoded with `strconv.Unquote`, which accepts exactly the
escape set Git emits (`\a \b \f \n \r \t \v \" \\` and three-digit
octal); `splitGitDiffPaths` is reused for field splitting, as directed.
A header that cannot be resolved returns an error and a **nil** slice —
it can never degrade to an empty scope, which is what `git diff` reads
as "everything".

Wired into the two write-scope surfaces:

- `workflow.RefreshAfterAccept` — strict-parses `originalPatch` **before**
  discovery and before any write; then filters nested worktrees; if a
  non-empty touched set filters down to empty it regenerates nothing
  rather than widening to a full-tree diff.
- `cli.computePathSet` — strict-parses the canonical patch; a malformed
  patch refuses before `status.json`, the index or HEAD is touched.
  `runLandDryRun`'s "expected files in patch" count uses it too.

**Fail-soft callers audited and documented** in `FilesInPatch`'s doc
comment. The only two remaining are advisory:
`workflow.touchedPathsFromPostApplyPatch` (D10 migration-hint
suppression, already documented fail-soft) and
`AppendPatchGenerationForFeature` (the `touched` audit field). Neither
drives a write, a diff scope or a staging decision, so a dropped quoted
path understates a hint or an audit list rather than widening what
tpatch touches. `PathsAffectedByPatch` is unchanged.

## Files Changed

Created this rev:

- `internal/gitutil/patch_paths_strict.go`
- `internal/gitutil/patch_paths_strict_test.go`

Modified this rev:

- `internal/gitutil/gitutil.go` (FilesInPatch doc/audit only)
- `internal/cli/land.go`
- `internal/cli/nested_worktree_test.go`
- `internal/workflow/refresh.go`
- `internal/workflow/refresh_test.go`
- `CHANGELOG.md`
- `docs/land.md`
- `docs/handoff/CURRENT.md`

Unchanged this rev: `internal/gitutil/worktrees.go`,
`internal/gitutil/capture_modes.go`, `internal/cli/cobra.go`,
`internal/cli/nested_worktree_guard.go`, `internal/cli/phase2.go`,
`SPEC.md`, `docs/record.md`.

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
- 104 passing assertions across the GH #7 test set.

New coverage this rev — **all quoted-header fixtures are produced by
real Git**, not hand-written strings:

- `TestFilesInPatchStrictRealGitQuotedHeaders` — one real
  `git diff --cached -M --binary` covering `sp ace.txt`,
  `tab\tin.txt`, `new\nline.txt`, `qu"ote.txt`, `back\slash.md`,
  `café.txt` (octal), a binary entry with no `---`/`+++`, a rename, a
  deletion and a mode-only change. It also asserts that `FilesInPatch`
  demonstrably MISSES the quoted entries, so the strict/fail-soft split
  stays justified.
- `TestFilesInPatchStrictQuotedGitlinkOnlyPatch` — a real
  `mode 160000` entry for a newline-named worktree resolves to exactly
  that one path.
- `TestFilesInPatchStrictRefusesMalformedHeaders` — unterminated quote,
  single operand, three operands, bad escape, empty operand,
  unresolvable rename; each returns an error and a nil scope.
- `TestFilesInPatchStrictEmptyPatch` (empty is not malformed) and
  `TestFilesInPatchStrictAgreesWithFailSoftOnPlainPatches`.
- `TestRefreshAfterAcceptQuotedWorktreeOnlyPatchDoesNotCaptureUnrelatedDirt`
  — the F2 regression end to end: a real quoted worktree-only patch plus
  unrelated dirt (`README.md` edit + `stray.txt`) regenerates NOTHING;
  the numbered reconcile patch and generation metadata are equally clean.
- `TestRefreshAfterAcceptQuotedWorktreePlusIntendedPathKeepsOnlyIntended`.
- `TestRefreshAfterAcceptMalformedHeaderFailsBeforeWrites` — feature
  directory byte-identical.
- `TestNestedWorktree_Land_DiscoversOnceBeforeEmbeddedRecord` — the
  calibrated invocation-budget test described above.
- `TestNestedWorktree_Land_EntryDiscoveryFailureLeavesEverythingUntouched`
  — feature dir, HEAD and `git status -z` all unchanged.
- `TestNestedWorktree_Land_NoRecordAndDryRunUseCachedPrefixes` — exactly
  one discovery each.
- `TestNestedWorktree_Land_StaleQuotedGitlinkPatchFilteredFromPlan` —
  a real quoted gitlink entry in the canonical artifact is parsed,
  filtered, and absent from the plan while the intended path remains.
- `TestNestedWorktree_Land_MalformedPatchFailsBeforeMutation` — refuses
  before `status.json`, index or HEAD change.
- `TestNestedWorktree_Land_ExtrasSemanticsUnchangedAfterReorder` —
  ordinary extras still refuse naming only themselves, the refusal does
  not write land's `landed at` note, `--allow-extra-paths` still stages
  the extra, `status.json` is still staged and the tree is clean after.

Preserved unchanged: `PathsAffectedByPatch` behavior and its tests, the
linked-worktree effective-index tests
(`TestDiffFromCommitForPathsUsesLinkedWorktreeIndex`,
`...PreservesLinkedWorktreeIndexAfterFiltering`), the NUL exotic-name
suite and the actionable all-filtered record diagnostic suite.

## Reproduction + control matrix (built binary)

One scratch repo with three nested worktrees (`agent review`,
`agent trail `, and a newline-named one) plus every over-filtering
control:

| Path | Kind | Result |
|------|------|--------|
| `.claude/worktrees/agent review` | nested worktree | absent from patch, diffstat, land plan, commit |
| `.claude/worktrees/agent trail ` | nested worktree, trailing space | absent from all four |
| `.claude/worktrees/new\nline` | nested worktree, embedded newline | absent from all four |
| `.claude/worktrees/agent-other/f.txt` | ordinary dir, prefix sibling | captured and landed |
| `vendor/plainrepo` | unregistered nested Git repo | captured and landed as a gitlink (correctly NOT filtered) |
| `../extwt` | worktree outside the root | never referenced |
| `unrelated.txt` | ordinary dirty path | refused naming only itself, then staged under `--allow-extra-paths` |

Post-land `git status` lists only the three worktrees as untracked plus
the carved-out `.tpatch/FEATURES.md`; `status.json` carries
`"notes": "landed at ..."` and is committed, so the tree is clean w.r.t.
feature scope.

All scratch repos, worktrees and build artifacts were removed;
`git worktree list` shows only the primary worktree.

## Reviewer focus

1. `computePathSet` gained `nested []string` and `includeStatus bool`.
   Confirm the dry-run passing `includeStatus=true` is the right call —
   it makes the printed plan match what real land stages, at the cost of
   listing `status.json` even in the rare case where it is not yet
   dirty (it normally is, since `apply`/`record` write it).
2. The status-notes write moved below `stagePathSet`'s precondition
   checks but stays above `stagePathSet` itself. Confirm PRD §3.6
   post-land cleanliness still holds — the end-to-end run above shows
   `status.json` staged, committed and clean.
3. `FilesInPatchStrict` precedence is: unambiguous `diff --git` operand,
   then `rename to`/`copy to`, then `+++`, then `---`. Renames of
   space-bearing unquoted paths rely on `rename to`; confirm that is
   sound for `diff.renames=false` repos, where Git emits separate
   delete/create entries that each resolve from the operand.
4. `land --no-record` with a malformed pre-existing canonical patch is
   the honest test of land's own strict parse; with the embedded record
   the parse failure would be attributable to `record` instead. Both
   paths are covered.
5. Advisory `FilesInPatch` callers are documented in its doc comment
   rather than converted. Confirm neither is a write-scope surface.

## Rev-3 Review Adjudication

- Internal: NEEDS REVISION.
- External/original reproducer: APPROVED.
- Valid residuals:
  1. Strict parsing accepts nonblank headerless input, Go-only escapes and an
     invalid a-side; refresh can still broaden malformed input.
  2. Land's entry-time worktree cache can stale if a linked worktree is
     registered before staging; `--allow-extra-paths` can then stage it.
- `tpatch_rev3_bin` and scratch artifacts are absent after external cleanup.

## Next Steps

1. Reject nonblank headerless patches and validate both operands with
   Git-only C escapes.
2. Revalidate/filter land immediately before staging and guard index/HEAD.
3. Add malformed-escape/a-side/headerless and concurrent-registration tests.
4. Run final dual review, then close #7 only after approval.

## Blockers

None.

## Context for Next Agent

- `internal/gitutil/worktrees.go` is the single discovery authority;
  `git worktree list --porcelain -z` is the single Git shape (Git 2.36+).
- `FilesInPatchStrict` is mandatory for any NEW code that derives a
  write scope, a diff scope or a staging decision from a patch.
  `FilesInPatch` is advisory-only and its two remaining callers are
  named in its doc comment.
- Land's contract is now: discover once at entry, thread the set, write
  nothing until every refusal has passed.
- Byte exactness remains load-bearing: no `TrimSpace`, no hand-rolled
  dequote on any path compared against a worktree prefix.
- `PreflightReconcile` is still deliberately unfiltered: it is a hygiene
  gate, not a capture surface.
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
