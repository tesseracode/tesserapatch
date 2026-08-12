# Current Handoff

## Status

**Cluster state**: REV-2 DISPATCHED

v0.15.1 Wave A rev-1 remains blocked by an ambiguous legacy fallback,
reconcile-refresh bypass, and mutation ordering. Rev-2 is dispatched.

## Active Task

- **Task ID**: v0.15.1 Wave A / GH #7 rev-2
- **Description**: Exclude registered linked Git worktrees nested beneath
  the target repository from apply/record capture and land planning.
- **Status**: In Progress
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `5d15fcf`
- **Rev-1 dispatch HEAD**: `556f9fa`
- **Target release**: v0.15.1

## Rev-0 residual closures

### R1 (HIGH) — NUL porcelain path bytes — CLOSED

`normalizeRepoRelPath` applied `strings.TrimSpace`. For a worktree
directory named `wt/trailing space `, discovery derived the prefix
`wt/trailing space` while `git ls-files --others` reported
`wt/trailing space /`, which `path.Clean`s to `wt/trailing space ` — the
two no longer matched, so the worktree flowed straight back into
capture as a `mode 160000` gitlink.

All whitespace normalization is removed from the path pipeline:

- `parseWorktreeListNUL` takes the value after the exact `worktree `
  key verbatim; the emptiness guard is exact equality, not `TrimSpace`;
- `nestedWorktreePrefixes` skips only exactly-empty records;
- `normalizeRepoRelPath` performs `ToSlash` + `path.Clean` only.

Absolute canonicalization (`filepath.Abs`, `EvalSymlinks`, the
missing-ancestor fallback) and `:(exclude,literal)` pathspec rendering
already preserved bytes and are unchanged.

**Proof**: temporarily restoring the single `TrimSpace` call makes
`TestNestedWorktreePrefixes_ExoticNamesRealRepo` fail with
`new file mode 160000` entries for `wt/trailing space ` and
`wt/trailing tab\t`; removing it again makes the test pass. The
regression is therefore pinned, not merely asserted.

### R2 (HIGH) — legacy newline fallback — CLOSED

Empirically, Git does **not** C-quote in `worktree list --porcelain`
without `-z`: a worktree at `wt/new\nline` is emitted as a raw embedded
newline, which is exactly why `-z` was added in Git 2.36. The rev-0
fallback split on `\n` and took whatever followed `worktree `, so such a
path was silently truncated.

`parseWorktreeListLegacy` is now strict:

- a record starts at a `worktree ` line and ends at a blank line;
- every other line must be a known attribute key (`HEAD`, `branch`,
  `bare`, `detached`, `locked`, `prunable`), optionally with a value;
  an unrecognised line is treated as the continuation of a
  newline-bearing path and refused;
- a value starting with `"` must be a well-formed, fully terminated Git
  C-quoted string, decoded by `unquoteCStyle` (`\a \b \f \n \r \t \v
  \" \\` and 1–3 digit octal). Malformed quoting is refused, never
  taken literally;
- an unquoted value is preserved byte-for-byte including trailing
  spaces and tabs; `\r` is not stripped, since Git does not emit CRLF
  here and a trailing `\r` is a legitimate path byte;
- every refusal names the fix: upgrade to Git 2.36+ for
  `git worktree list --porcelain -z`.

Routing is also tightened. `-z` failure is classified by stderr: only
an unknown-switch / usage error routes to the legacy shape. Any other
`-z` failure (broken repository, not a work tree) fails closed instead
of silently re-running through the weaker parser.

### R3 (MEDIUM) — diffstat bypass — CLOSED

`CaptureDiffStatScoped` (and therefore `CaptureDiffStat`) now appends
the same `:(exclude,literal)` nested-worktree pathspecs as the patch
capture and fails closed on discovery error. Both CLI diffstat call
sites propagate `ErrNestedWorktreeDiscovery` rather than swallowing it,
which matters on the reapply branch of `apply --mode done` where the
diffstat is the first Git read.

Additionally, `land`'s `dirtyPaths` now reads
`git status --porcelain -z`. The newline shape C-quotes any path
containing a space, and the previous `TrimSpace` + `Trim(path, "\"")`
dequote corrupted exactly the names the filter must match; the `-z`
shape is unquoted and byte-exact. Rename/copy entries consume their
extra origin field; the new path is still the staged one.

### R4 (test note) — CLI fail-closed regression — CLOSED

`installFailingWorktreeListGit` puts a `git` wrapper on `PATH` that
fails only for `git worktree list` (with a `fatal:` error, so it
exercises the genuine-failure branch rather than the legacy fallback)
and execs the real git otherwise. Production code is untouched — the
seam is the `git` executable. The same helper's returned `restore`
proves recovery in the same test.

### R5 (user-external LOW, rev-0 fold) — misleading empty-capture diagnostic — CLOSED

`record --files '<nested worktree>'` filtered every requested path away
and then fell through to the generic zero-byte diagnostic, which
speculates "possibly mode-only or binary changes". Actively misleading:
the paths were deliberately excluded by the GH #7 guard.

`nestedWorktreeEmptyCaptureRefusal` (in
`internal/cli/record_capture_modes.go`) runs first on the empty-capture
branch whenever the capture was scoped, and returns nil — falling
through to the untouched legacy diagnostics — unless at least one
requested pathspec is a nested worktree. Two shapes:

- **all requested paths are worktrees** → "every requested path is a
  registered nested Git worktree and is intentionally excluded from
  capture", listing them;
- **mixed** → names the count excluded, lists them, and separately
  lists the requested paths that simply produced no diff, so it never
  claims the whole request was worktrees.

Both explain why (a nested worktree is another checkout's working
directory; capturing it records a mode-160000 gitlink) and offer three
targeted recoveries, including `--path <worktree-root>` for operators
who genuinely meant to capture work done inside the worktree. The
`--staged` / `--unstaged` targeted refusals ("nothing staged for
capture") route through the new diagnostic too, since they are equally
misleading for a worktree-only scope. Committed-range mode is
deliberately excluded: it never consults the working tree, so its
`--from`/`--to` and `--auto` semantics are untouched.

Pathspecs carrying Git magic (a leading `:`) are conservatively
classified as "not a worktree", so the refusal can never be invented
from a spec tpatch cannot interpret as a plain path.

Same defect class, unscoped: `IsWorkingTreeDirty` counted a nested
worktree as dirt, so an unscoped empty capture printed the same
"possibly mode-only or binary changes" line and suppressed the correct
`--from` guidance. It now subtracts nested worktrees, reading
`git status --porcelain -z --untracked-files=all` so paths are
byte-exact and untracked directories are not collapsed to a parent that
the filter cannot classify. It has exactly one caller (this
diagnostic), and falls back to the raw answer if discovery fails.

## Files Changed

Modified this rev:

- `internal/gitutil/worktrees.go`
- `internal/gitutil/worktrees_test.go`
- `internal/gitutil/capture_modes.go`
- `internal/gitutil/gitutil.go`
- `internal/cli/land.go`
- `internal/cli/cobra.go`
- `internal/cli/record_capture_modes.go`
- `internal/cli/nested_worktree_test.go`
- `CHANGELOG.md`
- `docs/land.md`
- `docs/record.md`
- `docs/handoff/CURRENT.md`

Unchanged since rev-0: `internal/cli/phase2.go`, `SPEC.md`.

Deliberately NOT folded: the Makefile nested-repo sentinel LOW is
separately tracked and is not part of this code wave.

## Test Results

- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — clean.
- `go test -count=1 ./...` — all packages `ok`.
- `go test -race -count=1 ./internal/gitutil/` — ok.
- `go test -race -count=1 ./internal/cli/` — ok.
- Assets parity (`go test ./assets/`) — ok; no asset touched.
- 82 passing assertions across the GH #7 test set.

New or reworked coverage this rev:

- `TestParseWorktreeListNUL_PreservesPathBytes` — trailing space,
  trailing tab, leading space, internal tab + double spaces, embedded
  newline, quote/backslash; every path asserted byte-for-byte.
- `TestParseWorktreeListNUL_Malformed` — empty output, no worktree
  records, empty path.
- `TestParseWorktreeListLegacy_RawPathsPreserved` — raw trailing space,
  trailing tab, internal tab, double spaces, with `locked`/`prunable`
  attributes present.
- `TestParseWorktreeListLegacy_DecodesCQuoting` — newline, tab,
  trailing space, embedded quote, backslash, octal (`caf\303\251`),
  bell/vtab.
- `TestParseWorktreeListLegacy_RefusesAmbiguity` — raw newline
  continuation, unterminated quote, unknown escape, escape at end of
  input, bytes after the closing quote, octal out of range, attribute
  before any record, second `worktree` line inside a record, empty
  path, no records, empty output; each refusal asserted to carry
  `ErrNestedWorktreeDiscovery`.
- `TestParseWorktreeListLegacy_RefusalIsActionable` — refusal names
  Git 2.36 and `--porcelain -z`.
- `TestIsUnknownSwitchError` and
  `TestListRegisteredWorktreePaths_NonUsageFailureFailsClosed` — `-z`
  probe classification and the no-silent-downgrade rule.
- `TestNestedWorktreePrefixes_ExoticNamesRealRepo` — real
  `git worktree add` for four exotic names, four prefix-boundary
  control directories, byte-exact prefixes, untracked discovery,
  capture and diffstat.
- `TestNestedWorktreePrefixes_NewlineNameRealRepo` — real newline-named
  worktree (skips if the platform refuses the name; it does not on
  macOS/APFS).
- `TestNestedWorktreePrefixes_LegacyFallbackRealRepo` — a `git` wrapper
  rejecting `-z` exactly like pre-2.36 Git; the legacy parser still
  discovers and excludes a space-bearing nested worktree and
  over-filters nothing.
- `TestCaptureDiffStatExcludesStagedNestedWorktreeResidue` — staged
  intent-to-add gitlink residue absent from patch, diffstat and scoped
  diffstat.
- `TestNestedWorktree_StagedGitlinkResidue_AbsentFromPatchAndDiffstat`
  — the same at CLI level, for default and scoped record.
- `TestNestedWorktree_DiscoveryFailure_{ApplyDone,Record,Land}RefusesThenRecovers`
  — refusal + byte-identical feature dir + no HEAD advance + no
  staging plan, then recovery on the same fixture.
- `TestNestedWorktree_TrailingWhitespaceName_ExcludedEndToEnd` — a
  `wt/agent ` worktree excluded from patch, diffstat and land plan
  while the sibling `wt/agent` control stays in.
- `TestNestedWorktree_ScopedRecord_WorktreeOnlyFiles_ActionableDiagnostic`
  and `..._WorktreeDescendant_...` — the new refusal, asserted to name
  the offending pathspec and to NOT emit the mode-only/binary
  speculation or the `--from` hint; refusal writes no artifacts.
- `TestNestedWorktree_ScopedRecord_MixedWithRealChanges_Succeeds` and
  `..._MixedWithoutChanges_PartitionedDiagnostic` — the two mixed
  controls.
- `TestNestedWorktree_ScopedRecord_WorktreeOnlyFiles_CaptureModes` —
  `--all` / `--staged` / `--unstaged` route through the new diagnostic
  instead of "nothing staged for capture".
- `TestNestedWorktree_GenericEmptyCaptureDiagnosticPreserved` — both
  legacy branches (dirty-tree speculation, clean-tree `--from`
  candidates) preserved verbatim for genuinely empty non-worktree
  captures, in a repository that DOES contain a nested worktree.
- `TestIsWorkingTreeDirtyIgnoresNestedWorktree` — a nested worktree is
  not dirt; untracked, tracked-modified and staged changes still are.

Every fixture worktree is removed with `git worktree remove --force`
plus a prune via `t.Cleanup`, which runs before `t.TempDir()` teardown.

## Issue #7 regression + over-filtering control matrix (built binary)

Single scratch repo carrying all controls at once:

| Path | Kind | Result |
|------|------|--------|
| `.claude/worktrees/agent review` | nested linked worktree | excluded from patch, diffstat, land plan, commit |
| `.claude/worktrees/agent trail ` | nested linked worktree, trailing space | excluded from patch, diffstat, land plan, commit |
| `.claude/worktrees/agent-other/f.txt` | ordinary dir, prefix-boundary sibling | captured and landed |
| `vendor/plainrepo` | unregistered nested Git repo | captured and landed as a gitlink (correctly NOT filtered) |
| `../ext-wt` | linked worktree outside the root | never referenced |
| `unrelated.txt` | ordinary dirty path | still refused, then staged under `--allow-extra-paths` |
| tracked submodule gitlink | intentional | still captured (`TestCaptureKeepsTrackedGitlink`) |

Post-land `git status` shows only the two nested worktrees as untracked
and the carved-out `.tpatch/FEATURES.md`. Running tpatch **from** a
linked worktree was re-verified end to end with the built binary:
`apply --mode done` captures normally and the main worktree is not
treated as nested.

Empty-capture diagnostic matrix, re-verified with the built binary:

| Invocation | Result |
|------------|--------|
| `record --files '<worktree>'` | actionable refusal, names the worktree, exit 1 |
| `record --files 'README.md,<worktree>'` with README changed | succeeds; patch has only README |
| `record --files 'internal/example.go,<worktree>'` with no changes | partitioned refusal (excluded vs. no-diff), exit 1 |
| `record --files 'internal/example.go'` with unrelated dirt | generic diagnostic preserved verbatim, exit 1 |
| `record` unscoped, only dirt is the worktree | correct `--auto` / `--from` guidance with commit candidates, exit 1 |

All scratch repos, worktrees and build artifacts were removed. The
review-named `scratch-review/` and `tpatch_review_bin` were already
absent; this session's `.scratch7/`, `.scratch8/`, `.scratch9/` and the
gitignored root `tpatch` binary are removed. `git worktree list` in
this repository shows only the primary worktree, and
`git status --porcelain` shows only the guarded WIP docs.

## Reviewer focus

1. `parseWorktreeListLegacy` structural validation — is the known-key
   set the right forward-compatibility trade-off? A future Git
   attribute key would be rejected as ambiguous. Deliberate: the legacy
   shape only runs on Git < 2.36, which will not gain new keys.
2. `dirtyPaths` rename/copy handling under `-z` — the extra origin
   field is consumed by index advance; confirm against `R`/`C` in
   either status column.
3. `CaptureDiffStatScoped` now emits `--` whenever nested excludes
   exist even with no caller pathspecs; confirm no artifact byte drift
   for repositories without nested worktrees (the argument list is
   identical in that case).
4. The two `git` wrappers in tests install via `t.Setenv("PATH", …)`;
   confirm no test in these packages calls `t.Parallel()`.
5. `nestedWorktreeEmptyCaptureRefusal` runs before the `--staged` /
   `--unstaged` targeted refusals but after the capture itself, and is
   skipped entirely for committed-range mode; confirm that ordering
   matches the intended precedence and that no legacy diagnostic byte
   changed for non-worktree cases.
6. `IsWorkingTreeDirty` moved to `--porcelain -z --untracked-files=all`.
   It has one caller (the empty-capture diagnostic), so the blast
   radius is that message only — confirm the `all` upgrade is not
   observable anywhere else.

## Rev-1 Review Adjudication

- Internal: NEEDS REVISION.
- External/original reproducer: APPROVED.
- Valid residuals:
  1. Legacy newline porcelain cannot distinguish a path continuation that
     looks like a valid attribute; fallback must fail closed.
  2. Bare `usage:` text is too weak to authorize fallback.
  3. Reconcile accepted-result refresh uses `DiffFromCommitForPaths` without
     nested-worktree filtering.
  4. Apply/record can write canonical artifacts before a later diffstat
     discovery failure.
- Scratch artifacts reported by the user-external rev-0 review are absent;
  only guarded WIP remains.

## Next Steps

1. Remove the ambiguous legacy fallback; require NUL porcelain or fail closed.
2. Filter `DiffFromCommitForPaths`, including reconcile refresh.
3. Compute all discovery-dependent patch/diffstat results before writes.
4. Add late-failure transactional regressions.
5. Run final dual review and close #7 only after approval.

## Blockers

None.

## Context for Next Agent

- `internal/gitutil/worktrees.go` is the single authority. Any new
  untracked-discovery, diff or staging surface must route through
  `NestedWorktreePrefixes` + `PathUnderNestedWorktree`.
- `NestedWorktreePrefixes` now returns an already-wrapped
  `ErrNestedWorktreeDiscovery`; `NestedWorktreeDiscoveryError` is
  idempotent, so callers just propagate.
- Byte exactness is load-bearing. Do not reintroduce `TrimSpace`,
  `TrimSuffix(" ")` or a hand-rolled dequote anywhere on a path that
  will be compared against a worktree prefix.
- `PreflightReconcile` is still deliberately unfiltered: it is a
  hygiene gate, not a capture surface, and a nested worktree surfacing
  there produces a refusal, which is the safe direction.
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
