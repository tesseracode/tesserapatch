# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

Cluster H′ rev-1 is complete and pushed. All six adjudicated rev-0
findings are closed; independent internal and external rev-1 reviews have
not yet run.

## Active Task

- **Task ID**: Cluster H′ rev-1
- **Milestone**: v0.15.0 typed feature resources and capture adapters
- **Description**: Implement the Accepted Cluster H PRD and ADR-033
  end-to-end, then close the rev-0 dual-review findings.
- **Status**: Review
- **Assigned**: 2026-08-11
- **WAVE_BASE**: `46c984b`
- **Rev-0 dispatch commit**: `f277d51`
- **Rev-1 dispatch commit**: `2261714`
- **Implementation commits**: `bff5ef5`, `c66845a` (rev-0), `d82a367` (rev-1)
- **Commit range**: `46c984b..<this handoff commit>`; rev-1 fold is
  `2261714..d82a367`
- **Target release**: v0.15.0 (untagged; tagging is a later wave)

## Session Summary

One sequential implementer built the Accepted contract (rev-0) and then
folded the six adjudicated findings (rev-1). No parallel implementers
ran, every stage used explicit-path `git add`, and every commit carries
the Rule 18 trailer. Neither Accepted paper was modified.

## Finding Closures

**F1 — declaration privacy (HIGH).** `runResourceAdd` now calls
`scanDeclarationForRedaction` (shared `internal/redact`) over the
selector and every `--arg` value, for **every** kind, before the store
is opened, before the add-time Dolt TOFU open/hash, and before any lock,
scratch, manifest, pointer or batch write. A match refuses
`redaction-refused` (exit 3) with nothing created. Args *keys* remain a
closed design-owned vocabulary governed by the existing control-byte
rules. Covered by `TestAddRefusesRedactedDeclarationsWithZeroMutation`
(seven shapes across all kinds, each asserting zero artifacts),
`TestAddScanRunsBeforeAnyOtherValidation` (ordering) and
`TestAddAcceptsBenignDeclarations` (control).

**F2 — output bound and timer leak (HIGH/MEDIUM).**
`ProcessRunner.claimRetention` is an atomic CAS claim, so combined
retained stdout+stderr can never exceed cap+1 regardless of interleaving;
reading continues past the cap so the child still exits, but excess bytes
are discarded rather than appended. The invocation timer moved to
`time.AfterFunc`, removing the receiver goroutine that blocked forever on
a stopped timer, with a deterministic `OnTimerStopped` hook. Teardown
stays deferred past the priority re-check, so trigger priority is
unchanged. Covered by `TestRunawayChildIsBoundedToCapPlusOne` (real
3.2 MiB-writing, SIGTERM-ignoring child under a 64 KiB cap),
`TestClaimRetentionNeverOvershootsUnderConcurrency` (32 goroutines take
exactly cap+1), `TestInvocationTimerIsDisarmedOnEverySuccessfulRun` and
`TestInvocationTimerFiresWhenItShould` (control).

**F3 — batch integrity (HIGH).** `LoadBatch` binds the strict
fixed-wire shape (`DisallowUnknownFields`, exactly one JSON value plus
EOF, no null `results`/`args`/`result`), the decoded `batch_id`, and the
recomputed full content address against the requested filename ID.
Tampering is `batch-file-corrupt`, never `tracked-batch-missing` or
`batch-id-collision`. `compareSemanticBody` rejects trailing JSON after
its first object while presentation drift on a single valid object stays
idempotent. Covered by `TestLoadBatchRejectsTampering` (seven mutations,
including a body tamper with `batch_id` kept consistent),
`TestLoadBatchAbsentFileStaysTrackedBatchMissing`,
`TestPublishRejectsTrailingJSONAfterAValidObject`,
`TestPublishStillTreatsPresentationDriftAsIdempotent` and
`TestLoadBatchStrictAcceptsAnAuthenticFile` (control).

**F4 — canonical capability (MEDIUM).** `validateDeclaration` returns a
normalized effective capability used for the identity payload,
`DeriveResourceID` and `Resource.Capability`. A Dolt declaration stores
and hashes `diff-summary` with or without `--capability` (mismatch still
exits 2); `git-metadata head` converges on one canonical identity (empty
capability) for both spellings; other views keep their canonical view
capability. Covered by `TestGoldenResourceIDsThroughTheRealAddCLI`
(all four golden vectors driven through the real `add` CLI, plus
convergence, idempotency, mismatch and non-inferable-view subtests).

**F5 — ledger strength (MEDIUM).** `ac_coverage_test.go` is replaced by
`ac_ledger_test.go`. References are exact `(package, test, optional
literal subtest)` triples resolved with `go/ast` **in the declared
package only** — the cross-package fallback and subtest dropping are
gone. AC 1..120 and matrix 1..189 completeness is retained and is now
described honestly as a ledger, not semantic proof. It caught one
mis-attributed reference on first run
(`TestCurrentPointerIsCommittedByRenameNotDirectWrite` tagged `store`
but declared in `rescap`). Three mutation probes (wrong package, bad
subtest, missing test) were each correctly rejected.

**F6 — mutation-resistant tests.** `Setpgid` is asserted on the real
`exec.Cmd` the runner configured, not by grepping observer source; the
pointer commit point is proven by publishing over a `0444` live
`current.json`, which only temp+rename can do; the `os.SameFile`
descriptor gate is driven by a controlled replacement seam with a
no-swap control. New semantic coverage for rows 173, 174, 175, 176, 179,
180, 183, 186 and 189 (see the ledger for the exact test per clause).

### Defect found while hardening

Writing the row-180 regression exposed a real rev-0 bug:
`adapter-drain-timeout` was **unreachable**. `SetReadDeadline` always
unblocks a blocked read, so rev-0's "did the join take too long?"
detector could never observe an expiry. The drains now record deadline
expiry themselves, and a genuine `setsid`-escaped writer holding the
inherited pipe now produces the specified refusal, publishes nothing and
releases the per-slug `flock`.

## Files Changed

Rev-1 fold `2261714..d82a367`: 12 files, +2462 / -417.

- `internal/cli/feature_resource.go` — F1 scan, F4 normalization
- `internal/cli/feature_resource_test.go` — F1/F4/F6 CLI tests
- `internal/rescap/process.go` — F2 claim/timer, drain-expiry fix, hooks
- `internal/rescap/pathgate.go` — replacement + noexec seams
- `internal/rescap/dolt.go` — noexec preflight routed through the seam
- `internal/rescap/hardening_test.go` — new (linux || darwin)
- `internal/rescap/observer_darwin_test.go` — new (darwin)
- `internal/rescap/ac_ledger_test.go` — new, replaces
  `internal/rescap/ac_coverage_test.go` (deleted)
- `internal/store/resource_publish.go` — F3 strict decode/bind
- `internal/store/resources_test.go` — F3 tests
- `CHANGELOG.md` — rev-1 fold section

Whole wave `46c984b..HEAD`: 57 files, +13276 / -190.

## Test Results

At `d82a367`:

- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go build ./cmd/tpatch` — OK.
- `go test -count=1 ./...` — PASS, all 14 packages.
- `go test -race -count=1` — PASS for `rescap`, `store`, `redact`, `cli`.
- Assets parity guard — PASS.
- Suite totals: 459 passing assertions in `rescap`/`store`/`redact`, plus
  153 in the `cli` resource subset. Rev-1 adds 24 new test functions
  (17 hardening, 3 Darwin-native, 4 ledger) plus 5 store and 4 CLI
  additions.
- Cross-compile `go build ./...`: `linux/{amd64,arm64,386,s390x}`,
  `darwin/{arm64,amd64}`, `windows/{amd64,arm64}` — all OK.
  `go vet` OK on linux/{amd64,arm64}, darwin/{arm64,amd64},
  windows/{amd64,arm64}. Cross `go test -c` OK for linux/{amd64,arm64}
  and darwin/{arm64,amd64}.
- Native Darwin observer test empirically confirms the documented
  `waitid` stopped-child quirk on this kernel.
- No installed Dolt binary is used anywhere.
- Accepted PRD/ADR: zero diff since `46c984b`. Guarded untracked WIP:
  untouched.
- Side Research md5: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Run the independent internal rev-1 review.
2. Run the independent external rev-1 review.
3. Adjudicate, then run the Wave-Close Checklist.
4. Tag `v0.15.0` only after the implementation wave is accepted.

## Blockers

None.

## Residuals and Reviewer Focus

Disclosed residuals (each explicitly accepted by the Accepted papers):

1. Ancestor-directory TOCTOU between the component walk and the open is
   not closable with the Go stdlib (no `openat2`).
2. `cmd.Dir` is pathname-bound, so a `db_path` swap that occurs and
   reverts inside the child's own execution window is undetectable.
3. `cmd.Start()` opens the private copy by pathname after the
   descriptor-scoped `Fchmod`.
4. A reap timeout can leave up to two background goroutines outstanding
   and the leader unreaped.
5. The `ECHILD` finalizer makes no cleanup claim against the process tree.
6. A directory capture is a sequential read, not an atomic snapshot.

Rev-1-specific residuals:

7. The ledger verifies that references *resolve*, not that a referenced
   test semantically exercises its mechanism. That judgement remains a
   reviewer responsibility by design; the ledger only makes it cheap.
8. Table-driven subtest resolution accepts a `name`-keyed or positional
   string literal inside the test's composite literals. This is
   deterministic and breaks on a renamed case, but it is a superset of
   the literal `t.Run("x")` form.
9. Two tests build a helper binary with the local `go` toolchain
   (`TestNativeCrossBuildContract`,
   `TestEngineDrainTimeoutFromEscapedWriterPublishesNothing`) and skip
   under `-short` or without a toolchain.
10. `TestCurrentPointerIsCommittedByRenameNotDirectWrite` skips as root
    or on a filesystem where an owner may write a `0444` file, since the
    discriminator would not hold there.

Reviewer focus, in priority order:

1. `internal/rescap/process.go` — the retention claim, the drain-expiry
   detection rewrite, and that the timer change did not weaken trigger
   priority.
2. `internal/store/resource_publish.go` — whether the strict binding
   over-tightens any legitimate read, and that presentation drift is
   still idempotent.
3. `internal/cli/feature_resource.go` — that the scan genuinely precedes
   every write and that capability normalization cannot mint two
   identities for one semantic resource.
4. Whether the new seams (`SetBeforeGatedOpenForTest`,
   `SetScratchExecCheckForTest`, `OnTimerStopped`,
   `BeforeDrainDeadline`, `lingeringWriter`) are genuinely inert in
   production.

## Context for Next Agent

- Accepted papers remain binding; neither was modified.
- Implementation WAVE_BASE is `46c984b`.
- Pre-existing untracked PRDs, whitepapers and case-study files were not
  touched, staged, or formatted.
- `git-metadata` capability contract: `head` is canonical with an
  **empty** stored capability (both spellings converge); every other
  view stores its view name. Dolt always stores `diff-summary`.
- `record --resources` still wraps record's existing `RunE`, so the
  Git-side path remains byte-identical.
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
