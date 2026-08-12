# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave C rev-1 folds all six adjudicated rev-0 findings and is
awaiting review. All 161 accepted rows remain green, with the rev-1
regressions added on top.

## Active Task

- **Task ID**: v0.15.1 Wave C / GH #8 implementation
- **Description**: Implement the accepted landed-feature verification and
  land producer contract.
- **Status**: Review
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `b768602`
- **Target release**: v0.15.1

## Session Summary — rev-1 fold

Every adjudicated finding is closed, each with a regression that fails
against the rev-0 code.

**F1 (P1) — one floor-validated offline Git context.**
New `internal/workflow/verify_gitgate.go` is the single gateway. Every
Git operation the verify path performs is a method on
`verifyRunContext`; each calls `gitGate()` BEFORE spawning anything and
applies `GIT_NO_LAZY_FETCH=1`. Routed through it: ancestry
(`isAncestorChecked`, used by V5 **and** by the V4 dependency
validator), the landing enumeration, `TempIndex`, shadow
create/prune/read-tree/apply/write-tree/clean, `cat-file`, the
normalized-identity `diff`, parent replay and `verify --all`.
`internal/gitutil/shadow.go` gained `CreateShadowEnv`,
`PruneShadowEnv` and `PruneAllShadowsEnv` so the offline environment is
explicit and unrelated callers are byte-unchanged. Below the floor the
run now spawns **only** `git --version` even when the feature carries
`satisfied_by` deps; V5 refuses with a floor-naming remediation instead
of silently answering.

**F2 (P1) — a single immutable inventory.**
`newVerifyRunContext` builds it once and it is now the authority for
V0 (target status), V3 (parent states), V4
(`store.ValidateDependenciesWith` over a new `store.ValidationEnv`),
V5, V6 (`CheckDependencyGateSnapshot`), the closure walk, supersession
(`isFeatureSupersededIn(ctx.inv.Statuses(), …)`), the parent snapshot,
`verify --all`'s own feature enumeration and persistence. New
`store.FeatureSnapshot` is the read-once view; `ListFeatures`,
`IsFeatureSuperseded` and per-parent `LoadFeatureStatus` are gone from
the verify path. `store.WriteVerifyRecordFrom` persists from the
captured status and returns the exact persisted value, so the capture is
folded forward without a reload.

**F3 (P1) — probe failures classify honestly.**
`gitutil.ApplyCheckResult` gained `ExitCode` and `ApplyAnswered()`:
exit 1 is a patch-level answer, anything else is an execution failure.
`forwardQualifies` returns `(qualified, state, err)`; `anchorResolution`
carries `FailState`/`FailDetail`; `identitiesFor` failures route through
`classifyGitFailure`. A locally missing object is `history-incomplete`
with R22, any other unrunnable probe is `unavailable` — never "no
qualifier" or "ambiguous". The AC-L69 fallback allowance is deleted: the
row now requires `history-incomplete` + R22 verbatim.

**F4 (P1) — read errors are not absence.**
`snapshotArtifact` treats only `os.ErrNotExist` as absent and retains
every other read failure with its path; `patch-generations.json` read
and parse failures are retained too. `inventoryEntry.ReadErr()` drives
the D17 policy: the target or a closure member blocks with
`inventory-unreadable` naming the artifact, an unrelated feature warns
and is excluded from ADR-029 ordering.

**F5 (P2) — the reachability advisory.**
An `exact`/`duplicate-equivalent` landing whose attested base commit is
well-formed but not reachable from `HEAD` now emits the required
`base-commit-unreachable` warn advisory alongside the report field, and
never fails on its own.

**F6 (P2) — land base-commit exactness.**
`validateLandBaseCommit` validates the stored bytes verbatim (any
leading/trailing whitespace, whitespace-only, uppercase or wrong-length
value is malformed), returns the validated canonical string, and `land`
emits **that** string in the trailer. Mode A/B ordering is unchanged.

## Current State

- GH #8 stays empirically closed with the rev-1 binary: before land
  passes in forward mode; after land `target_mode=landed`,
  `landing_evidence=exact`, `baseline.mode=dual-anchor`, all three
  anchored rows green, exit 0; `verify --all` exits 0.
- AC-L68 / AC-L69 remain closed against a **real filtered remote**
  (`git daemon` + `uploadpack.allowFilter`, real object deletion, dead
  promisor URL). AC-L69 no longer accepts a fallback classification.
- No blocker.

## Files Changed (rev-1)

New:

- `internal/workflow/verify_gitgate.go`
- `internal/store/feature_snapshot.go`
- `internal/workflow/verify_rev1_fold_test.go`
- `internal/cli/land_rev1_fold_test.go`

Modified:

- `internal/gitutil/shadow.go` (env-aware create/prune helpers)
- `internal/gitutil/trailers.go` (`ExitCode` / `ApplyAnswered`)
- `internal/store/store.go` (`WriteVerifyRecordFrom`)
- `internal/store/validation.go` (`ValidationEnv`, `ValidateDependenciesWith`)
- `internal/workflow/dependency_gate.go` (`CheckDependencyGateSnapshot`)
- `internal/workflow/verify.go`, `verify_landed.go`, `verify_anchored.go`,
  `verify_all.go`, `verify_diagnostics.go`
- `internal/cli/land.go`, `internal/cli/land_base_commit.go`
- `internal/workflow/acceptance_ledger_test.go`,
  `verify_landed_groupd_test.go`, `verify_landed_partialclone_test.go`,
  `verify_test.go`, `internal/cli/land_landing_evidence_test.go`
- `CHANGELOG.md`, `docs/handoff/CURRENT.md`

The rev-0 files (`internal/gitutil/trailers.go`,
`internal/workflow/verify_landed.go`, `verify_anchored.go`,
`verify_diagnostics.go`, `internal/cli/land_base_commit.go` and the
eight acceptance-test files) are unchanged except where a finding
required it.

## Test Results

- `gofmt -l .` — clean.
- `go vet ./...` — clean; `GOOS=linux` and `GOOS=windows` vet clean.
- `go build ./cmd/tpatch` — clean; cross-builds clean for
  linux/amd64, linux/arm64, darwin/arm64, windows/amd64.
- `go test -count=1 ./...` — **all 12 packages pass**.
- `go test -race -count=1 ./internal/workflow ./internal/gitutil
  ./internal/store ./internal/cli` — pass.
- Acceptance ledger: **161/161 rows mapped**, audited against the ids
  scraped from both PRDs; 17 rows gained rev-1 mappings and every named
  test is proven to exist.
- Docs totality guard (AC-L135 / G1–G10): zero hits, sensitivity test
  green. The accepted documents were **not** edited.
- GH #2 regression `TestRunVerify_EquivalentRecipeAndPatchBothPass` green
  and **unmodified** (AC-L121 diffs it against WAVE_BASE `b768602`).
- Side Research md5: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Dual review of the rev-1 fold against all 161 rows.
2. On acceptance: close #8, tag v0.15.1, run the Wave-Close Checklist.

## Blockers

None.

## Context for Next Agent

Reviewer focus for rev-1, and the judgement calls made inside the
contract:

1. **`git apply --check` exit 1 vs 128 is the answer/failure
   discriminator.** Treating every non-zero exit as a failure would
   break the ladder (a patch that does not apply is the answer the
   contract wants); treating none as a failure is the rev-0 defect.
   `ApplyAnswered()` encodes exactly that split, and
   `TestRev1_GenericProbeFailureIsUnavailable` injects a 128 to pin it.
2. **V5 below the floor is a block, not a skip.** The run already fails
   via `unavailable`, but V5 must not claim reachability it could not
   check; the remediation names the 2.36 requirement. A feature with no
   `satisfied_by` dep is unaffected, so AC-L134's fixture is unchanged.
3. **`store.ValidationEnv` / `CheckDependencyGateSnapshot` /
   `WriteVerifyRecordFrom` are additive.** A zero env or a nil snapshot
   reproduces the shipped behaviour byte-for-byte, so `amend`, `apply`,
   `status` and every other caller is untouched — only verify passes the
   capture and the gated ancestry resolver.
4. **`pruneShadow` is a no-op below the floor.** No shadow can exist in
   that case (creation is gated), so the deferred cleanup returns nil
   rather than reporting an error the operator cannot act on.
5. **The unreachable-base advisory has no R-number.** The accepted R
   table stops at R24 and the advisory vocabulary is closed at five
   codes; the message is therefore a new string carrying the code,
   pinned verbatim by `TestRev1_BaseCommitUnreachableAdvisoryIsEmitted`.

Carried forward from rev-0 (still true):

- `landing_evidence.state` is OMITTED for the D10 artifact-presence
  short-circuit; `failed_at` and `reason` carry the outcome.
- The classification terminals run before the static short-circuit, and
  neither allocates a shadow.
- AC-L84 and AC-L111 are proven in two halves because V4 pre-empts the
  arbitration branch; AC-LD18a's refusal half is driven through the
  production validator because the shipped `record` always writes a
  valid base commit.
- The isolated index lives under `<git-dir>/tpatch-verify`.
- Stray build artifacts in the repo root predate the session and are
  gitignored; they were left untouched.

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
