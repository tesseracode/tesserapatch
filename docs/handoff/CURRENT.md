# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave C rev-2 closes the five adjudicated rev-1 findings and is
awaiting review. All 161 accepted rows remain green; every rev-2 fix has
a black-box regression proven to fail against the rev-1 code.

## Active Task

- **Task ID**: v0.15.1 Wave C / GH #8 implementation
- **Description**: Implement the accepted landed-feature verification and
  land producer contract.
- **Status**: Review
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `b768602`
- **Target release**: v0.15.1

## Session Summary — rev-2 fold

**F1 (P1) — V2 parses the CAPTURED recipe.**
`checkRecipeParses(ctx, slug)` decodes `inventoryEntry.Recipe.Bytes`
with the same strict `DisallowUnknownFields` and the same presence
semantics; there is no live read and no fallback. A NON-absence read
failure is reported as a block ("cannot read apply-recipe.json"), never
as "no recipe", so it composes with the D17 `inventory-unreadable`
terminal instead of contradicting it. The parsed value and the bytes are
the ones V3, V7, V10 and `recipe_hash_at_verify` all use.
`readArtifactBytes` and the `inventoryRecipeBytes` fallback are deleted.

**F2 (P1) — duplicate-attestation identity failure.**
In `classifyEvidence`'s `len(allMatch) >= 2` branch the `identitiesFor`
error is classified with `classifyGitFailure`: a missing object is
`history-incomplete` (R22), any other command failure is `unavailable`
(R10), and only a SUCCESSFUL comparison that disagrees stays
`ambiguous` (R7).

**F3 (P1) — historical V8 apply failure.**
New `gitutil.OfflineGitResult` + `ctx.shadowApplyCheck` retain the exit
code and stderr through the offline gateway. Exit 1 — and a
patch-level diagnostic such as "no valid patches in input", which git
reports with exit 128 — remain ANSWERS: a genuine non-apply is still
exactly R5, and a corrupt artifact still gets the shipped forward-mode
V8 string. A missing object or a repository-level fatal routes through
`terminalEvidencePhase`, so the emitted remediation is the closed
R22/R10 string and no patch/attestation disagreement is claimed.

**F4 (P1) — unlanded parent presence probe.**
In the evidence-none branch a probe that cannot be answered is terminal:
missing object → R22/`history-incomplete`, generic failure →
R10/`unavailable`. It never falls through to recipe replay.
`ladder.Blocked` is an ANSWERED absence and still means replay; a clean
or context-drift probe still means skip + R18.

**F5 (P2) — instability readability transitions.**
The re-statement compares `Err` nil/non-nil for the recipe, patch and
provenance snapshots plus `GenerationsErr`, alongside presence and
bytes. `absent ↔ unreadable` — both of which carry `Presence == absent`
and no bytes — is now `snapshot-unstable` naming the exact slug and
path. A stably-unreadable unrelated feature keeps its existing warn.

Shared mechanism: `gitutil.ApplyProbeAnswered` is the ONE
answer-vs-failure classifier, used by `ApplyCheckResult.ApplyAnswered`
(ladder + qualifier) and `OfflineGitResult.Answered` (shadow V8). It is
exit-code and stderr based, never status-text matching.

## Current State

- GH #8 stays empirically closed with the rev-2 binary: before land
  passes, after land `landing evidence: exact`, dual-anchor baseline,
  exit 0, `verify --all` exit 0.
- AC-L68 / AC-L69 remain green against a real filtered remote.
- No blocker.

## Files Changed (rev-2)

New:

- `internal/workflow/verify_rev2_fold_test.go`

Modified:

- `internal/gitutil/trailers.go` (`OfflineGitResult`,
  `RunOfflineGitInResult`, `IsPatchInputError`, `ApplyProbeAnswered`)
- `internal/workflow/verify_gitgate.go` (`shadowApplyCheck`)
- `internal/workflow/verify.go` (V2 over captured bytes; dead reader
  removed)
- `internal/workflow/verify_landed.go` (identity-failure classification,
  readability transitions, fallback removal)
- `internal/workflow/verify_anchored.go` (historical V8 split, unlanded
  parent probe terminal)
- `internal/workflow/acceptance_ledger_test.go`
- `CHANGELOG.md`, `docs/handoff/CURRENT.md`

## Test Results

- `gofmt -l .` clean; `go vet ./...` clean; `GOOS=linux` and
  `GOOS=windows` vet clean.
- Cross-builds clean: linux/amd64, linux/arm64, darwin/arm64,
  windows/amd64.
- `go test -count=1 ./...` — all 12 packages pass.
- `go test -race -count=1 ./internal/workflow ./internal/gitutil
  ./internal/store ./internal/cli` — pass.
- **Regression proof**: with the rev-1 production files restored and the
  rev-2 tests in place, 8 rev-2 tests fail (V2 capture purity, the
  source guard, both duplicate-attestation branches, both historical-V8
  branches, the unlanded-parent probe, and the workspace mutation);
  restoring rev-2 turns them green. The readability table additionally
  fails 8 of its 16 sub-cases — exactly the `absent ↔ unreadable` pairs
  rev-1 could not see.
- Acceptance ledger: 161/161 rows mapped; AC-L35/L53/L58/L69/L77/L78/
  L108/L109/L110/L117/L123 remapped to the black-box proofs, and the two
  helper-only rev-1 entries were removed from the ledger so no row
  claims assurance a unit call cannot give.
- Docs totality guard (AC-L135 / G1–G10): zero hits; accepted documents
  untouched.
- GH #2 regression green and unmodified (AC-L121 diffs it against
  WAVE_BASE `b768602`).
- Real filtered-remote AC-L68 / AC-L69: green.
- Side Research md5: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Dual review of the rev-2 fold against all 161 rows.
2. On acceptance: close #8, tag v0.15.1, run the Wave-Close Checklist.

## Blockers

None.

## Context for Next Agent

Reviewer focus for rev-2:

1. **`git apply --check` exits 128 for a MALFORMED patch**, not 1.
   Measured: a zero-byte or non-diff artifact yields
   `error: No valid patches in input` with exit 128. Treating every
   non-1 exit as an execution failure would report a corrupt artifact as
   an unavailable reader and would have broken two shipped GH #2-era
   rows. `ApplyProbeAnswered` therefore accepts exit 0, exit 1, and any
   exit whose diagnostic is about the PATCH, while a missing-object or
   network diagnostic is always a failure.
2. **Unanswerable probes route through `terminalEvidencePhase`.** That
   is what makes the emitted remediation the closed R10/R22 string
   rather than a new sentence, and it keeps `failed_at` inside the
   thirteen-value vocabulary (`landing-evidence`). No schema vocabulary
   was invented for any rev-2 branch.
3. **The purity guard is AST-based**, not a substring scan: it allows
   `os.ReadFile` only inside `snapshotArtifact`, `buildInventory`,
   `inventoryInstability` (the documented re-statement) and
   `replayOpInShadow` (which reads inside the shadow worktree, not the
   store). V1's intent-file stat and V3's op-target stat are working-tree
   probes, not captured artifacts, and are deliberately out of scope.
4. **Readability comparison is nil/non-nil only.** Comparing error TEXT
   would make two independent `*PathError` values look like a change on
   every run; the contract-useful signal is the transition, and the
   stable-unreadable case is pinned by its own test.

Carried forward from rev-0/rev-1 (still true):

- `landing_evidence.state` is OMITTED for the D10 artifact-presence
  short-circuit; `failed_at` and `reason` carry the outcome.
- The classification terminals run before the static short-circuit, and
  neither allocates a shadow.
- AC-L84 and AC-L111 are proven in two halves because V4 pre-empts the
  arbitration branch; AC-LD18a's refusal half is driven through the
  production validator.
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
