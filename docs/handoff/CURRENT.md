# Current Handoff

## Status

**Cluster state**: IN PROGRESS

`implement-prepare-intent-bundle` is dispatched from WAVE_BASE `3b579fc` under
[GH #23](https://github.com/tesseracode/tesserapatch/issues/23). The accepted
rev-15 PRD + ADR-035 are authoritative, with strict implementation order
`S1b → S1 → S3 → S4 → S4b`, pre-change goldens before producer refactors,
then S5/S6 and sequential S7 hardening. No release tag is authorized before
joint acceptance. Golden implementation review discovered that rev-14
PIB-391 required standalone `prepare --check` output files to have been
committed by GH #16, but GH #16 committed no such paths. Rev-15 corrects only
that impossible evidence predicate. Rev-15 review round 0 returned internal
NEEDS REVISION and external APPROVED WITH NOTES; round 1 aligned ADR-035
D14, the rev-15 amendment ledger, shipped-prerequisite prose, producer
auditability and fixture-before-production sensitivity. Joint re-review
returned internal APPROVED and external APPROVED WITH NOTES; the sole
`cacaaf8` terminology note is folded. Rev-15 is accepted and the
pre-production golden baseline landed at `f9208c7`, with its Git-maintenance
race pin at `977b9d5`. CI run
[32178723042](https://github.com/tesseracode/tesserapatch/actions/runs/32178723042)
is green on Ubuntu, macOS and Windows; the dedicated GH #23 native Windows
resource golden is blocking and passed.

S1b landed at `1f35605`; CI
[32185709105](https://github.com/tesseracode/tesserapatch/actions/runs/32185709105)
is green on Ubuntu, macOS and Windows. S1 landed at `f0ae54b`; CI
[32202082897](https://github.com/tesseracode/tesserapatch/actions/runs/32202082897)
is green on all three platforms. S3 landed at `4c3dbfe`; CI
[32220278819](https://github.com/tesseracode/tesserapatch/actions/runs/32220278819)
passed every Ubuntu, macOS and Windows test job (the release job was correctly
skipped). S2 landed at `16d614a`; CI
[32229096085](https://github.com/tesseracode/tesserapatch/actions/runs/32229096085)
is green on all three platforms.

S4 mutating prepare is complete at `49301eb`. Its primary implementation
landed at `5853ba7`; its first
blocking CI run
[32280073787](https://github.com/tesseracode/tesserapatch/actions/runs/32280073787)
failed deterministically on two pre-S4 source guards that only saw
`prepare_publish.go` after it became tracked. AVP-134 still admitted one
`internal/intent` importer instead of the accepted S4 pair, and AVP-141 still
expected one module-wide `os.OpenRoot` site instead of the frozen check site
plus S4's two mutating sites. The correction retains exact file/site
allowlists and sensitivity arms; no product behavior changes. Follow-up CI
[32281269945](https://github.com/tesseracode/tesserapatch/actions/runs/32281269945)
is green on Ubuntu, macOS and Windows.

S4b retention rev-4 is implemented in the worktree and internally approved
pending full local validation and explicit-path commit.
Rev-0's pending-preview, predicted-remediation, terminal-recovery,
partial-class, divergence-shape and retry-heading defects are closed. Rev-1
findings are also closed. Rev-2's shell quoting and selector-precedence
findings are closed. Rev-3's final control-character path finding is closed by
one managed-path predicate at snapshot/plan report boundaries, with
non-echoing zero-write refusal and real newline/tab/ESC/DEL/C1 fixtures.
Focused rev-4 re-review returned APPROVED.

Pre-commit tracked-state validation found one stale S4 guard: AVP-134 admitted
the two prepare command files but not S4b's accepted
`feature_intent_archive.go` importer. The bounded correction pins the exact
three-file set and retains both extra-forbidden-importer and
missing-authorized-set sensitivities; no product behavior changes.
The corrected staged-state full and race gates are green.

Before that CI blocker, S4 was internally approved. Its first independent code review
returned NEEDS REVISION on exit-3 writes after stale cleanup/staging, use of a
pre-authority artifact snapshot, unsafe abandon rollback, lost deadline
classification and repeated human rollback sections. The revision re-inspects
under the held authority, moves every exit-2/3 gate ahead of cleanup/staging,
prevalidates V1–V5 without writing, preserves concurrent abandon evidence,
carries bounded deadline metadata and renders archive residue once. Serialized
normal and race suites pass across all five changed packages. Focused re-review
found one remaining exit-class demotion in staging failures; the final fold
preserves typed exit 6 for both base and archive-index post-rename durability
failures, and re-review returned APPROVED.

The `tesseracode/copilot-api` v0.15.1 feedback was independently triaged on
2026-08-18 and accepted at evidence commit `e6901a2` (range
`7206dab..e6901a2`). GH #18–#22 now track the confirmed migration gaps; the
diagnosis and evidence are archived in the cumulative-verification case study.
This backlog intake does not preempt the active prepare queue.

## Active Task

- **Task ID**: `implement-prepare-intent-bundle`
- **Issue**: [GH #23](https://github.com/tesseracode/tesserapatch/issues/23)
- **Description**: Implement the mutating `tpatch prepare <slug>` intent-bundle
  contract from the accepted `PRD-prepare-intent-bundle` rev-15 +
  `ADR-035-intent-bundle-publication-and-history` rev-15 (ADR-035 normative
  where they overlap).
- **Status**: **In Progress — S4b approved, ready to commit**
- **Assigned**: 2026-08-18
- **WAVE_BASE**: `3b579fc7243bf0d1b21605d3c87562226f1fd936`
- **Release tag**: TBD; the accepted `prepare --check` prerequisite will ship
  with this release

## Prerequisite Status

PRD §19's three acceptance conditions are now all satisfied:

1. `PRD-prepare-intent-bundle` Accepted at rev-15 (2026-08-18).
2. `ADR-035` Accepted at rev-15 (2026-08-18), reviewed jointly with the PRD.
3. §19(3) — the accepted `prepare --check` contract
   (`PRD-artifact-validation-and-provenance` rev-5 / rev-6 errata + `ADR-034`
   rev-2 / rev-3 errata) has frozen implementation content at `cacaaf8` and
   was formally accepted/closed at `7206dab`.

Implementation is therefore unblocked. **The PRD's implementation slices and
their required sequence must be re-read in full before dispatch** — this
handoff deliberately does not restate them, and the slice partition, not this
file, is the dispatch authority.

## Backlog

- [GH #17](https://github.com/tesseracode/tesserapatch/issues/17) — open,
  **non-blocking**: the pre-existing `windows-latest` full-suite failures
  (200 top-level / 283 including subtests, six packages) stay visible behind
  one `continue-on-error` step that names the issue. AVP-175 pins the demotion
  to exactly one step, the exact literal `true`, and the Windows full-suite
  step; removing it when GH #17 lands is a deliberate edit, not a silent one.
- Two LOW AVP-175 parser follow-ups from the rev-4 external verdict: no
  flow-mapping step decoding, and a first-match (not uniqueness-proving)
  decoy-leaf floor. Address them whenever `.github/workflows/ci.yml` or
  AVP-175 is next edited.
- `GOOS=js GOARCH=wasm go build ./cmd/tpatch` fails in `internal/rescap` at
  `WAVE_BASE` unchanged; unticketed, out of scope of any prepare wave.
- [GH #12](https://github.com/tesseracode/tesserapatch/issues/12),
  [GH #13](https://github.com/tesseracode/tesserapatch/issues/13),
  [GH #14](https://github.com/tesseracode/tesserapatch/issues/14),
  [GH #15](https://github.com/tesseracode/tesserapatch/issues/15) — parked
  research backlog; no implementation or architecture decision authorized.
- [GH #18](https://github.com/tesseracode/tesserapatch/issues/18) — cumulative
  verify semantics and a truthful migration assessment.
- [GH #19](https://github.com/tesseracode/tesserapatch/issues/19) — Path B
  manual recipe provenance publication and safe adoption.
- [GH #20](https://github.com/tesseracode/tesserapatch/issues/20) — honest
  legacy patch-generation adoption without fabricated provenance.
- [GH #21](https://github.com/tesseracode/tesserapatch/issues/21) — guarded
  mechanical recipe-schema migrations through doctor.
- [GH #22](https://github.com/tesseracode/tesserapatch/issues/22) — durable
  later-touch acknowledgement that does not weaken preimage/replay safety.

## Downstream Feedback Assessment

- Exact installed `tpatch v0.15.1` result at `copilot-api@e2d7ce4`: 0 passed,
  53 failed, 3 skipped, 0 error; failing checks 38 V8 / 16 V7 / 6 V10 /
  1 intent.
- Downstream health remains green: typecheck, lint, 352 tests and build.
- 29 of 38 V8-failing patches apply at their own recorded base; nine do not.
  Own-base verification is therefore useful evidence, not sufficient proof.
- Four recent V10 failures are missing `recipe-provenance.json`, not measured
  stale hashes. All 11 non-empty preimages match their recorded base.
- Doctor confirms 24 D2 manifests cannot be produced by the recommended
  same-byte refresh. All 24 have reachable candidate bases in `status.json`,
  but only six patches apply to them; eight historical D7 recipes required a
  manual mechanical migration.
- Full evidence and limits:
  `docs/state-of-the-art/case-studies/copilot-api-cumulative-verify-2026-08/summary.md`.

## Files Changed

- `internal/cli/prepare.go`
- `internal/cli/prepare_publish.go`
- `internal/cli/prepare_publish_s4_test.go`
- `internal/cli/prepare_test.go`
- `internal/cli/prepare_avp_test.go`
- `internal/cli/prepare_avp2_test.go`
- `internal/cli/prepare_pib_golden_test.go`
- `internal/gitutil/ignore.go`
- `internal/gitutil/ignore_prepare_test.go`
- `internal/intentpub/stage.go`
- `internal/intentpub/plan_stage_hardening_test.go`
- `internal/intent/avp_source_scans_test.go`
- `internal/rescap/gitgate.go`
- `internal/rescap/scratch.go`
- `internal/workflow/session_ignore.go`
- `docs/handoff/CURRENT.md`
- `docs/ROADMAP.md`
- `docs/supervisor/LOG.md`
- S4b worktree (uncommitted):
  `internal/cli/feature_intent_archive.go`,
  `internal/cli/feature_intent_archive_test.go`, and the registration line in
  `internal/cli/feature_deps.go`.
- S4b tracked-source correction:
  `internal/intent/avp_source_scans_test.go`.

## Test Results

- `go test -p=1 -count=1 ./...` — PASS.
- Serialized race coverage for all five S4 packages — PASS; the final revision
  additionally reran the five transaction-order/fault regressions under
  `-race`.
- Exact 51-fixture `TestPreparePIBPreChangeGoldens` — PASS; no fixture was
  re-recorded.
- `gofmt -l .`, `go vet ./...`, host build and Linux amd64, Darwin amd64 and
  Windows amd64 cross-builds — PASS.
- S2 prerequisite CI
  [32229096085](https://github.com/tesseracode/tesserapatch/actions/runs/32229096085)
  — green on Ubuntu, macOS and Windows.
- Side Research EOF tail remains
  `b385fe622db9926f48861105239f113e`.
- S4 blocking CI
  [32280073787](https://github.com/tesseracode/tesserapatch/actions/runs/32280073787)
  — FAILED only AVP-134 and AVP-141 on stale pre-S4 tracked-source allowlists.
- Corrected AVP-134/AVP-141 targeted tests, the exact 51-fixture/provenance
  guards and `go test -p=1 -count=1 ./...` from the tracked S4 state — PASS.
- Corrected blocking CI
  [32281269945](https://github.com/tesseracode/tesserapatch/actions/runs/32281269945)
  — PASS on Ubuntu, macOS and Windows; release job correctly skipped.
- S4b rev-0 targeted `TestFeatureIntentArchive*`, affected feature-deps/AVP
  guards, gofmt and diff check — PASS before review; verdict NEEDS REVISION on
  seven substantive state/report/sensitivity findings.
- S4b rev-1 closes all seven rev-0 findings and its targeted tests pass;
  re-review remains NEEDS REVISION on six narrower retry/list/schema/spy
  findings.
- S4b rev-2 closes those six findings; re-review remains NEEDS REVISION on
  shell-safe corrupt-object repair paths and selector gate precedence.
- S4b rev-3 closes both rev-2 findings; final re-review remains NEEDS REVISION
  on non-echoing refusal of control-containing managed paths.
- S4b rev-4 closes the final path-safety finding; focused re-review APPROVED.
- First tracked-state `go test -p=1 -count=1 ./...` failed only AVP-134's stale
  exact importer set; correction active.
- Corrected staged-state `go test -p=1 -count=1 ./...`, full
  `go test -race -p=1 ./internal/cli`, exact 51-fixture/provenance guards,
  gofmt, vet, host build and Linux/Darwin/Windows amd64 cross-builds — PASS.

## Next Steps

1. Commit the seven explicit S4b/guard/tracking paths, push and require
   blocking three-platform CI.
2. Add S5 doctor D9 and S6 public docs/assets, then complete S7's 567-row
   acceptance ledger and sensitivity hardening.
3. Run joint internal/external review to acceptance; only then select the
   release tag carrying `prepare --check` plus mutating prepare.

## Blockers

- None. Rev-15 is accepted and the mandatory golden commit is landed and
  green.

## Context for Next Agent

- `internal/intent` must not import `internal/store`; the status schema is
  mirrored locally on purpose and kept honest by the AST parity guard.
- The guard registry is the single source of truth for the 43 `G` rows. Adding
  a matrix row whose Kind contains `G` without registering a guard fails
  AVP-139 and the ledger automatically.
- Routing goldens must never be re-recorded from the current binary.
  Reconstruct the `WAVE_BASE` binary in a temporary detached worktree
  **outside** the repository, exactly as
  `internal/cli/testdata/routing-goldens/README.md` documents.
- `prepare --check` is read only by contract. The mutating wave adds new modes
  alongside it; it must not reopen the accepted read-only contract, and
  ADR-034's rooted boundary is a **read** boundary that ADR-035 explicitly does
  not extend to writes.
- The untracked research WIP in `git status` predates these waves and is
  covered by `.wave-close-allowlist`. Do not touch it.
- GH #23's explicit file partition is authoritative. Stage exact files only;
  never use `git add .`, `git add -A`, directory-scope adds or `git commit -a`.

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
