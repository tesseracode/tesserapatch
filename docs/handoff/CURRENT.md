# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

Cluster H′ rev-2 is complete and pushed. All six adjudicated rev-1
findings are closed and the rev-0 closures still hold; independent
internal and external rev-2 reviews have not yet run.

## Active Task

- **Task ID**: Cluster H′ rev-2
- **Milestone**: v0.15.0 typed feature resources and capture adapters
- **Description**: Implement the Accepted Cluster H PRD and ADR-033
  end-to-end, then close the rev-0 and rev-1 review findings.
- **Status**: Review
- **Assigned**: 2026-08-11
- **WAVE_BASE**: `46c984b`
- **Rev-2 dispatch commit**: `407d68b`
- **Implementation commits**: `bff5ef5`, `c66845a` (rev-0), `d82a367`
  (rev-1), `86f93b7` (rev-2)
- **Rev-2 fold range**: `407d68b..86f93b7`
- **Target release**: v0.15.0 (untagged; tagging is a later wave)

## Session Summary

One sequential implementer owns the whole wave. Every stage used
explicit-path `git add`, every commit carries the Rule 18 trailer, and
neither Accepted paper nor any guarded WIP file was touched.

## Rev-2 Finding Closures

**R2-F1 — CLI batch taxonomy (HIGH).** `list`/`diff` route every
batch-load error through `classifyBatchLoadError`, which preserves the
store's own `*store.PublicationError` reason, and
`aggregateBatchFailures`, which folds per-resource failures into the
returned error. An absent file stays `tracked-batch-missing` exit 1; a
present-but-corrupt or identity-invalid batch surfaces
`batch-file-corrupt` exit 3 at the process boundary **and** in the
per-resource JSON `state`/`status` and text line. Mixed failures name
every distinct reason and take the most severe code. Covered by
`TestListPreservesBatchLoadTaxonomy` and
`TestDiffPreservesBatchLoadTaxonomy` (six shapes each: deleted,
batch_id mismatch, body tamper with consistent batch_id, trailing
object, unknown field, nulled results),
`TestListTextOutputCarriesTheBatchReason`,
`TestMixedBatchFailuresStayCoherent` and
`TestListAndDiffStayCleanForAuthenticBatches` (control).

**R2-F2 — publication authenticity (MEDIUM).** `compareSemanticBody`
calls `bindBatchIdentity` before any drift/collision comparison, so a
file that does not authentically hash to its own name is
`batch-file-corrupt`. `batch-id-collision` is reserved for two
*authentic* bodies sharing one full digest, which tampering can no
longer reach — so a substitutable `SetBatchIDDeriverForTest` seam makes
that branch reachable honestly. Covered by
`TestPublishBatchCollisionAndCorruption` subtests
`semantic-collision-between-two-authentic-bodies` and
`tampered-existing-body-is-corruption-not-collision`,
`TestCaptureOverTamperedBatchIsCorruptionNotCollision` (real `capture`),
`TestRecordResourcesWrapsBatchCorruption` (wrapped as
`resource-domain-incomplete` once Git succeeded) and
`TestCaptureRepublishesUnchangedContentIdempotently` (control).

**R2-F3 — AC-104/105 and rows 173/174 (MEDIUM).** AC-104 now references
`TestNoexecPreflightRunsBeforeTheCopyIsCreated`. AC-105 references
`TestPrivateCopyExactHostErrnosCleanUpAndStartNothing`, which injects
exact `syscall.ENOSPC` and `syscall.EIO` at the streamed-write and
`Sync` steps through a narrow `privateCopyTarget` seam, asserting
`adapter-copy-failed` exit 1, the injected errno present in the
refusal, the partial copy removed, and — via
`TestPrivateCopyHostFailureStartsNoProcess` — zero process starts and
zero publication. The rev-1 generic directory-read and
unwritable-directory substitutes are removed.
`TestPrivateCopySucceedsWithoutInjection` is the inertness control.

**R2-F4 — row 180 real CLI (MEDIUM).**
`TestCaptureCLIDrainTimeoutFromEscapedWriter` drives the actual
`feature resource capture` command against a real `setsid`-escaped
writer through lock acquisition and publication orchestration:
`adapter-drain-timeout` exit 3, no batch and no pointer, ephemeral
scratch cleaned, lock released (verified by re-acquiring it), and a
subsequent normal capture succeeding. The fixture switches behaviour by
marker rather than by rewriting itself, so recovery runs against the
same pinned digest. Row 180 maps to this test; the `Engine.Stage` test
remains as unit coverage only.

**R2-F5 — ledger subtest parser (LOW).** Discovery recognizes only a
literal `t.Run("name", ...)` or an explicitly keyed `name: "..."` table
field; arbitrary positional string literals are rejected. Four
positional tables (`TestValidateDoltArgs`, `TestParseDiffSummaryJSON`,
`TestWaitIsLaunchedStrictlyAfterTheSignalPhase`,
`TestLocalPathTrackedRefusal`) were converted to keyed `name` fields,
and two package-scope tables are referenced at top level honestly.
`TestLedgerSubtestDiscoveryRejectsUnrelatedLiterals` adds five negative
cases parsed from fixtures, proving SQL fragments, script bodies and
non-`name` keyed fields cannot resolve.

**R2-F6 — SameFile mechanism (LOW).** A seam sits strictly after the
descriptor `os.SameFile` decision and strictly before the redundant
pathname re-Lstat.
`TestSameFileDescriptorGateIsTheLoadBearingCheck` swaps the path for the
open and restores it only at that seam, and asserts the restore seam
never fires — i.e. the descriptor comparison already refused.
`TestGatedOpenAcceptsAnUnreplacedEntry` is the control. A scratch
mutation probe deleting the `os.SameFile` guard was run: the test failed
with `got <nil>`, accepting the swapped descriptor. The probe was
reverted and the file restored.

## Files Changed

Rev-2 fold `407d68b..86f93b7`: 12 files, +1359 / -218.

- `internal/cli/feature_resource.go` — R2-F1 classify/aggregate
- `internal/cli/feature_resource_test.go` — R2-F1/F2 CLI tests; keyed table
- `internal/cli/feature_resource_drain_test.go` — new, R2-F4
- `internal/rescap/dolt.go` — R2-F3 privateCopyTarget seam
- `internal/rescap/pathgate.go` — R2-F6 post-SameFile seam
- `internal/rescap/dolt_test.go` — keyed tables
- `internal/rescap/hardening_test.go` — superseded tests removed; keyed table
- `internal/rescap/mechanism_test.go` — new, R2-F3/F6
- `internal/rescap/ac_ledger_test.go` — R2-F3/F4/F5
- `internal/store/resource_publish.go` — R2-F2 bind-before-compare + seam
- `internal/store/resources_test.go` — R2-F2 tests
- `CHANGELOG.md` — rev-2 fold section

Whole wave `46c984b..HEAD`: 59 files, +14509 / -194.

## Test Results

At `86f93b7`:

- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — OK.
- `go test -count=1 ./...` — PASS, all 14 packages.
- `go test -race -count=1` — PASS for `rescap`, `store`, `redact`, `cli`.
- Assets parity guard — PASS.
- Suite totals: 471 passing assertions in `rescap`/`store`/`redact`,
  182 in the `cli` resource subset. Rev-2 adds 6 new test functions
  (5 mechanism, 1 real-CLI drain) plus new subtests in existing tests.
- Cross-compile `go build ./...`: `linux/{amd64,arm64,386,s390x}`,
  `darwin/{arm64,amd64}`, `windows/{amd64,arm64}` — all OK.
  `go vet` OK on all six of those targets. Cross `go test -c` OK for
  `linux/{amd64,arm64}` and `darwin/{arm64,amd64}`.
- Rev-0/rev-1 closure regression re-run explicitly: redaction,
  golden-ID-via-CLI, retention bound, timer lifecycle, forced-close
  joins, Wait ordering, late-ECHILD, Start-failure and strict batch
  loading all still PASS.
- Accepted PRD/ADR: zero diff since `46c984b`. Guarded untracked WIP:
  untouched.
- Side Research md5: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Run the independent internal rev-2 review.
2. Run the independent external rev-2 review.
3. Adjudicate, then run the Wave-Close Checklist.
4. Tag `v0.15.0` only after the implementation wave is accepted.

## Blockers

None.

## Residuals and Reviewer Focus

Disclosed design residuals (accepted by the Accepted papers):

1. Ancestor-directory TOCTOU is not closable with the Go stdlib.
2. `cmd.Dir` is pathname-bound; a swap-and-revert inside the child's own
   execution window is undetectable.
3. `cmd.Start()` opens the private copy by pathname after the
   descriptor-scoped `Fchmod`.
4. A reap timeout can leave up to two background goroutines outstanding.
5. The `ECHILD` finalizer makes no cleanup claim against the process tree.
6. A directory capture is a sequential read, not an atomic snapshot.

Test-contract residuals:

7. The ledger verifies that references *resolve*, not that a referenced
   test semantically exercises its mechanism.
8. Subtest discovery binds only literal `t.Run` names and keyed `name`
   fields declared **inside** the test function; a table declared at
   package scope is referenced at top level instead.
9. Three tests build a helper binary with the local `go` toolchain and
   skip under `-short` or without one.
10. `TestCurrentPointerIsCommittedByRenameNotDirectWrite` skips as root
    or where an owner may write a `0444` file.
11. `TestNoexecPreflightRunsBeforeTheCopyIsCreated` and the ENOSPC/EIO
    cases inject their conditions through seams rather than mounting a
    noexec filesystem or filling a real disk.

Reviewer focus, in priority order:

1. `aggregateBatchFailures`' severity/primary-reason rule — whether the
   reported reason and exit code can ever disagree.
2. `compareSemanticBody`'s new ordering — that binding first cannot
   reclassify a legitimate presentation-drift republish.
3. Whether the four new seams (`privateCopyTarget`,
   `afterDescriptorIdentityCheck`, `batchIDDeriver`, and the existing
   `beforeGatedOpen`) are genuinely inert in production.
4. Whether the ledger's keyed-`name` restriction now under-reports any
   subtest a reviewer would expect to see referenced.

## Context for Next Agent

- Accepted papers remain binding; neither was modified.
- Implementation WAVE_BASE is `46c984b`.
- Pre-existing untracked PRDs, whitepapers and case-study files were not
  touched, staged, or formatted.
- Ledger rule: reference a subtest only when it is declared by a literal
  `t.Run` or a keyed `name:` field inside that test function; otherwise
  reference the top-level test.
- `git-metadata` capability contract: `head` is canonical with an empty
  stored capability; every other view stores its view name; Dolt always
  stores `diff-summary`.
- Resources are audit sidecars, never canonical patch or lifecycle truth.

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
