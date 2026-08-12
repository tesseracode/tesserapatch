# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

v0.15.1 Wave C (GitHub issue #8) is implemented end-to-end and awaiting
review. All 161 accepted rows are implemented and green.

## Active Task

- **Task ID**: v0.15.1 Wave C / GH #8 implementation
- **Description**: Implement the accepted landed-feature verification and
  land producer contract.
- **Status**: Review
- **Assigned**: 2026-08-12
- **WAVE_BASE**: `b768602`
- **Target release**: v0.15.1

## Session Summary

Implemented ADR-013 Amendment 1 rev-7 (D8–D19), PRD-verify-freshness §3.6
and PRD-tpatch-land §3.8.6 in full, with the 135 verify + 26 land
acceptance rows and a mechanically audited ledger.

Production changes:

1. **`internal/gitutil/trailers.go` (new)** — the single git-side reader:
   the ≥ 2.36 floor probe (`ReadGitVersion`), the three-fact repository
   preflight (`ReadRepoFacts`: object format + derived commit-id hex
   length, shallowness, promisor config, `<git-dir>`, `.git/shallow`
   membership), ONE
   `git log --topo-order --reverse -z --format=…` enumeration returning
   raw bodies AND parsed trailers (`EnumerateCommitTrailers`), the
   raw-precedence probe (`RawBodyHasTrailerLine`), the index-isolated
   prober (`TempIndex` + `ApplyCheck` with `(0/0)` counting), the D18
   normalized change identity (`NormalizedChangeIdentity` /
   `NormalizeDiffBytes`), offline `cat-file`/`rev-parse`/`merge-base`
   helpers, and the local-missing-object classifier. Every command
   carries `GIT_NO_LAZY_FETCH=1`.
2. **`internal/workflow/verify_landed.go` (new)** — policy: the immutable
   inventory over `store.ListFeatureEntries()` (status + `RequestedAt`,
   three artifacts' presence states and raw bytes, `patch-generations`
   touched paths, unreadable rows retained), instability re-statement,
   the run context shared across `verify --all`, D10 evidence
   classification with the presence short-circuit, the D12 hardened
   ladder, D14 collect/qualify/compare/select, D15 provenance
   resolution, the ADR-029 later-touch index over the inventory, and the
   R1–R22/R24 remediation templates.
3. **`internal/workflow/verify_anchored.go` (new)** — the V7/V8/V10
   dynamic phase: terminal families, the landed path (anchor H shadow at
   the replay anchor's parent + anchor C ladder folded into V8), the
   forward path, D13 arbitration (patch-ladder presence, `active`
   widening, unapplied/rejected/terminal fail-fast,
   unattributed-materialized advisory), and per-member V10 baselines.
4. **`internal/workflow/verify_diagnostics.go` (new)** — the
   diagnostic-only op-presence predicates, including the existential
   inverse for `replace-in-file`.
5. **`internal/workflow/verify.go` / `verify_all.go`** — schema 1.1
   report (`repository`, `baseline`, `landing_evidence`, `target_mode`,
   `advisories`), one run context per invocation reused by `--all`,
   hashes computed from captured bytes, human-report header lines,
   snapshot-instability marking. The superseded `runClosureReplay`,
   `checkWriteFilePreimageFresh`, `parentReplayFail` and
   `loadParentRecipe` were removed.
6. **`internal/store/types.go`** — `VerifyCheckResult` gains `mode`,
   `anchor_results`, `member_baselines`, `provenance_hash_bound`, all
   `omitempty`. No persisted field changed.
7. **`internal/cli/land_base_commit.go` (new) + `land.go`** — the R23
   producer precondition, inserted after `recoverLand` in Mode A and
   after `record` returns in Mode B.
8. Docs: `SPEC.md`, `CHANGELOG.md`, `docs/land.md`, `internal/cli/verify.go`
   help text.

## Current State

- GH #8 is empirically closed. With the pre-fix binary at `7cf245a` the
  post-land run reports `verdict=failed`, V8 false-red and the wrong
  `run tpatch reconcile` remediation; with the fixed binary the same
  scenario reports `target_mode=landed`, `landing_evidence=exact`,
  `baseline.mode=dual-anchor`, all three anchored checks green, exit 0,
  and the committed-range re-record still passes.
- AC-L68 / AC-L69 (the Wave C acceptance gates) are **closed with a real
  filtered remote**: the fixture starts a real `git daemon` with
  `uploadpack.allowFilter=true`, makes a real `--filter=blob:none`
  clone, backfills through that same remote for AC-L68, and for AC-L69
  deletes a real object and points the promisor at a dead URL. The
  missing-object failure is the LOCAL form under `GIT_NO_LAZY_FETCH=1`,
  never the network form. **No blocker is required.**
- Forward mode (evidence `none`) is byte-unchanged for V7/V8; V10 moves
  to `RecipeProvenance.BaseCommit` for ops carrying `preimage_hash`, as
  §7.1.1 requires.

## Files Changed

New:

- `internal/gitutil/trailers.go`
- `internal/workflow/verify_landed.go`
- `internal/workflow/verify_anchored.go`
- `internal/workflow/verify_diagnostics.go`
- `internal/cli/land_base_commit.go`
- `internal/workflow/verify_landed_fixture_test.go`
- `internal/workflow/verify_landed_groupabc_test.go`
- `internal/workflow/verify_landed_groupd_test.go`
- `internal/workflow/verify_landed_groupe_test.go`
- `internal/workflow/verify_landed_groupf_test.go`
- `internal/workflow/verify_landed_groupg_test.go`
- `internal/workflow/verify_landed_grouph_test.go`
- `internal/workflow/verify_landed_partialclone_test.go`
- `internal/workflow/docs_totality_guard_test.go`
- `internal/workflow/acceptance_ledger_test.go`
- `internal/cli/verify_landed_cli_test.go`
- `internal/cli/land_landing_evidence_test.go`

Modified:

- `internal/workflow/verify.go`, `internal/workflow/verify_all.go`
- `internal/store/types.go`
- `internal/cli/land.go`, `internal/cli/verify.go`
- `internal/workflow/verify_test.go` (schema 1.1 assertion, AC-L113)
- `internal/workflow/writefile_verify_preimage_test.go` (provenance
  anchor fixtures, Q15)
- `internal/cli/land_test.go` (Mode A base-commit precondition fixture)
- `SPEC.md`, `CHANGELOG.md`, `docs/land.md`, `docs/handoff/CURRENT.md`

## Test Results

- `gofmt -l .` — clean.
- `go vet ./...` — clean; `GOOS=linux` and `GOOS=windows` vet clean.
- `go build ./cmd/tpatch` — clean; cross-builds clean for
  linux/amd64, linux/arm64, darwin/arm64, windows/amd64.
- `go test -count=1 ./...` — **all 12 packages pass**.
- `go test -race -count=1 ./internal/workflow ./internal/gitutil ./internal/store`
  — pass.
- Acceptance ledger: **161/161 rows mapped** (135 verify + 26 land),
  audited mechanically against the ids scraped from both PRDs, and every
  named test function proven to exist.
- Docs totality guard (AC-L135 / G1–G10): **zero hits** across the three
  authoritative documents, plus a sensitivity test proving each pattern
  fires on a reintroduced stale claim.
- GH #2 regression `TestRunVerify_EquivalentRecipeAndPatchBothPass` is
  green and **unmodified** (pinned by AC-L121, which diffs the file
  against WAVE_BASE `b768602`).
- Side Research md5: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Dual review against the 161 rows.
2. On acceptance: close #8, tag v0.15.1, run the Wave-Close Checklist.

## Blockers

None.

## Context for Next Agent

Reviewer focus, and the four places where implementation judgement was
exercised inside the contract:

1. **`landing_evidence.state` is OMITTED for the D10 artifact-presence
   short-circuit.** The state vocabulary is a closed set of ten and
   `landed-artifacts-absent` is not one of them — it is a `failed_at`
   value, and the short-circuit happens *before* classification. Rather
   than emit an eleventh state or misreport one of the ten, the field is
   omitted and `landing_evidence.reason` + `failed_at` carry the
   outcome. `patch_presence` / `recipe_presence` are still emitted and
   the digest-match fields are omitted, exactly as AC-L51 requires.
2. **The classification terminals run BEFORE the static short-circuit.**
   `terminal` evidence and `landed-artifacts-absent` are properties of
   the evidence and the captured artifacts, not of the dynamic phase, so
   they are decided even when an earlier block-severity static check
   failed. Neither allocates a shadow, so the "no shadow on static
   failure" guarantee is preserved.
3. **Two rows are proven in two halves because a shipped static check
   pre-empts the arbitration branch.** `AC-L84` (rejected parent) and
   `AC-L111` (unreadable closure member) are blocked by V4
   (`dep_metadata_valid`) before V7 runs — pre-existing ADR-031 /
   validation behaviour this amendment does not change. Each test
   asserts the shipped precedence end-to-end AND drives the specified
   arbitration branch directly, so the R17 / inventory-unreadable text
   is still pinned.
4. **AC-LD18a's refusal half is driven through the production
   validator.** The shipped `record` always writes a valid
   `Apply.BaseCommit` (AC-LD18b proves it), so Mode B's refusal is
   unreachable end-to-end without corrupting `record`. The test asserts
   (a) end-to-end that Mode B validates the *produced* value — an
   invalid pre-record value lands fine — and (b) the exact R23 text plus
   the retained-artifacts note through `validateLandBaseCommit`, with
   `record`'s artifacts and the absent `landed at` note checked on disk.

Other things worth knowing:

- The isolated index lives under `<git-dir>/tpatch-verify`, not
  `.tpatch/local/`. D11 permits either; the git dir is unconditionally
  outside the tracked tree, whereas the `.tpatch/local/` ignore rule is
  installed by `tpatch init` and can be absent in a hand-made workspace.
- `git apply --cached` reports an unreadable blob as
  `error: failed to read <path>`, which is in the missing-object
  classifier; a genuine content mismatch says `patch does not apply`, so
  the two are distinguishable and `history-incomplete` never masks
  `landed-content-absent`.
- Verify's own `WriteVerifyRecord` refreshes that feature's captured
  status in the shared run context, so `verify --all` does not read its
  own freshness write as `snapshot-unstable`.
- The preflight issues ONE combined
  `git rev-parse --path-format=absolute --show-object-format
  --is-shallow-repository --git-dir` plus one `git config --get-regexp`,
  which is inside the documented three-call budget.
- Stray build artifacts in the repo root (`cli.test`, `cli.test.exe`,
  `gitutil.test.exe`, `tpatch`) predate this session and are gitignored;
  they were left untouched.

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
