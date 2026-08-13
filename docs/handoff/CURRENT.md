# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

The artifact-validation/provenance PRD writer has delivered rev-0 from base
`0aa0d95` (HEAD at dispatch `12980f2`). This is planning only; no code, no
assets, no SPEC, no ADR. `prepare` mutation remains blocked.

## Active Task

- **Task ID**: PRD-artifact-validation-and-provenance
- **Description**: Define truthful read-only intent-artifact inspection,
  provenance/migration boundaries and `tpatch prepare --check` as the
  prerequisite to mutating preparation.
- **Status**: Writer rev-0 delivered — Awaiting Review
- **Assigned**: 2026-08-13
- **WAVE_BASE**: `0aa0d956b090288780b51d8270eb3a250fabeee3`
- **Issue**: [GH #10](https://github.com/tesseracode/tesserapatch/issues/10)
- **Scope**: one PRD + WP-005 graduation/index updates + handoff
- **Release tag**: v0.15.1 remains fixed at `15560af`

## PRD Writer Contract

- Claims-audit current `manual.go`, Path A workflow, `FeatureStatus.Notes`,
  `next`, `cycle`, feature layout and shipped skill surfaces.
- Define deterministic structural artifact states without claiming semantic
  quality.
- Define read-only `prepare --check` human/JSON output, precedence and exits.
- Pin `provenance: unknown` until accepted metadata proves otherwise.
- Decide explicitly whether individual mutating `--manual` gates change.
- Preserve existing `defined`, `next` and `cycle` behavior unless a behavior
  delta is enumerated and accepted.
- Cover symlinks, non-regular files, unreadable paths, privacy, determinism,
  migration and Path A/B parity.
- Evaluate provenance representation alternatives. No ADR is pre-created; if
  the PRD selects persistent representation, flag the ADR gate before
  acceptance for implementation.
- Exclude mutating prepare, regenerate, provider execution, atomic bundle
  publication, new lifecycle state and downstream SDD mandates.
- Mark WP-005 Graduated when the PRD file is created.

## PRD Writer Result — rev-0 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | **new**, 1,472 lines, `Status: Draft — Awaiting Review (rev-0)` |
| `docs/whitepapers/WP-005-spec-driven-workflows.md` | header only: `Exploring` → `Graduated (2026-08-13)` + one `Related` bullet to the PRD. Body and turn log untouched. |
| `docs/whitepapers/README.md` | index row: WP-005 `Exploring` → `Graduated (2026-08-13)` |
| `docs/handoff/CURRENT.md` | this section + `Cluster state` flip |

Nothing else was touched. No edit to `docs/ROADMAP.md`, `docs/supervisor/LOG.md`,
`SPEC.md`, `docs/adrs/**`, `internal/**`, `cmd/**`, `assets/**`, `tests/**`,
`.wave-close-allowlist`, or any guarded untracked WIP (`WP-004`, `WP-006`,
`WP-007`, `PRD-recurring-patches.md`, state-of-the-art case studies).

### Counts

- **64 claims** in the §23 claims-audit appendix, each with a `file:line`
  anchor.
- **95 acceptance rows** (`AVP-001`…`AVP-095`) across 12 categories: CLI
  grammar 10, classification 20, readiness/exit 8, output shape 14, zero
  mutation 6, provenance 5, compatibility 9, Path A/B parity 4,
  security/privacy 6, concurrency 4, source scans/parity 6, totality guards 3.
- **133 distinct `file:line` anchors** across the whole document; all
  mechanically verified in-range at HEAD `12980f2`, and the load-bearing ones
  content-verified by reading the cited lines.

### Contract decisions made (not deferred)

1. **Scope**: `tpatch prepare <slug> --check [--json] [--quiet] [--path]` only.
   Plain `prepare` refuses with exit 4 *before* the store is opened;
   `--manual` / `--regenerate` are deliberately **unregistered** (cobra
   unknown-flag → exit 1).
2. **Turn-4 question answered**: mutating `analyze|define|explore|implement
   --manual` gates are **unchanged** in this slice. The inspector is pure and
   wired to `prepare --check` only. Six acceptance rows (`AVP-064`…`AVP-069`)
   pin the *loose* current behavior — including "zero-byte `spec.md` still
   advances to `defined`" — as deliberately unchanged. §12.3 lists what a
   future tightening PRD must enumerate.
3. **Closed 9-value state enum** extending the shipped
   `absent`/`present-empty`/`present-nonempty` vocabulary with
   `symlink-refused`, `not-regular`, `unreadable`, `oversize`,
   `invalid-structured`, `unstable`; 17-row total precedence table.
   `unstable` deliberately outranks every content-derived state so a
   mid-truncation read is never reported as `present-empty`.
4. **Required = `analysis.md` + `spec.md`.** `exploration.md` and
   `artifacts/analysis.json` are optional and can never change readiness; the
   sidecar is reported separately, and no output field may assert Path A vs
   Path B for a feature (that would be provenance inference from sidecar
   presence).
5. **Provenance = constant `unknown`.** Seven forbidden inference sources
   enumerated with anchors. Four persistence alternatives evaluated (status
   sub-record / dedicated manifest / derivation-only / write-time attestation);
   **none selected in rev-0** — the PRD is coherent without one — and the ADR
   trigger is stated as a gate on any future PRD that selects one.
6. **Exit envelope**: 0 ready / 1 generic CLI / 2 not_ready / 3 indeterminate
   (abort or required-artifact instability) / 4 reserved-surface refusal, with
   a six-step first-match-wins precedence and five closed abort codes.
7. **Output**: `schema_version: 1`, fixed key order, fixed-length 4-element
   `artifacts` array (even on abort), closed reason/advisory/abort catalogs,
   frozen disclaimer string, verify-style `--json`/`--quiet` stream routing.
   No wall-clock, no size, no hash, no content, no absolute path, no symlink
   target — 17 forbidden field names asserted absent.
8. **Snapshot semantics**: one descriptor-scoped capture per artifact, three
   named instability probes, **explicitly stated limits** (a same-length
   in-place rewrite is not detectable), no retry, no lock, and no
   cross-artifact atomicity claim.
9. **Skill parity**: v1 ships the command in all six surfaces and extends
   `requiredCommands` + `requiredAnchors` (`AVP-090`…`AVP-092`).
10. **Blocked-PRD statement**: `PRD-prepare-intent-bundle.md` remains blocked
    until this PRD is accepted — stated in the header, the summary and §20.

### Open decisions left to review (§21, four)

Q1 a later `--all` sweep mode; Q2 whether exit `4` becomes a cross-command
convention or stays local to `prepare`; Q3 the `MaxArtifactBytes = 4 MiB`
value; Q4 whether `request.md` becomes a fifth optional reported row. All four
have a stated default and none blocks review.

### Implementation status

**None.** No Go file, test, asset or CLI surface was created or modified. The
PRD defines five ordered implementation slices (S1–S5) for a future wave and
explicitly requires the AGENTS.md parallel-implementer file partition to be
declared at dispatch.

### Validation performed (docs-only change)

- Mechanical anchor audit: 133 distinct `file:line` citations parsed and
  checked in-range against the working tree; **zero** out-of-range, zero
  missing files. Re-run after the WP-005 header edit shifted that file by +2
  lines, and the ten affected anchors were re-resolved and content-verified.
- Markdown hygiene: fence balance (16 markers, balanced), table column
  consistency (0 mismatched rows), zero trailing-whitespace lines, all 13
  relative links resolve on disk.
- The `json` example block parses as valid JSON.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Notes for the reviewer

- The PRD deliberately does **not** claim a test proves semantics merely by
  existing: §18.1 states five disqualifying assertion shapes, and §18.14
  requires sensitivity regressions for the three mechanical guards
  (`AVP-093`/`094`/`095`) because a byte-scanning guard can false-pass — the
  lesson from the F2 finding recorded above in this file.
- §4 is the required WP-005 existing-primitives pre-flight and covers
  `--manual`, `cycle`, `next`, `verify`, `status`, `doctor` and the generic
  path/fs helpers, each with an anchor.
- The `apply --mode prepare` name collision is treated as a real risk with two
  mandated help-text mitigations and two acceptance rows, not waved away.

## WP-005 Turn 2 Scope

- Append one `agreement` turn introducing the new stable agent ID.
- Record the council method and the advisor/reviewer split honestly.
- Add `## Agreed` findings: validation before orchestration, `prepare --check`
  first, no new lifecycle state, optional internal SDD, downstream SDD
  encouraged but never enforced.
- Resolve WP-005's prior "separate whitepaper" uncertainty: this paper is
  sufficient prior art for two gated PRDs.
- Keep the paper `Exploring` until the first graduating PRD is drafted.
- Do not edit code, create an ADR, or add prepare to the implementation
  roadmap in this task.

## WP-005 Turn 2 Writer Result

- Added CO76 as the second stable whitepaper author and appended one
  `agreement` turn without editing Turn 1.
- Recorded the full council split (2 ship now / 2 reject / 1 specify first),
  five-reviewer convergence and chairman disposition.
- Added an unbylined `## Agreed` section covering validation before
  orchestration, check-first delivery, no new state, optional internal SDD and
  downstream encourage-never-enforce.
- Replaced the open-ended "separate whitepaper" product note with two ordered
  PRDs: artifact validation/provenance first; prepare bundle blocked second.
- Kept `Status: Exploring` until the first graduating PRD is drafted.
- Added current-code anchors for manual validation, Path A's structured
  analysis sidecar and `next`'s file-presence inference.

## Turn 2 Review Adjudication

- **Internal**: NEEDS REVISION — WP-004 is guarded/untracked, so WP-005's
  Related link breaks in a clean checkout.
- **External**: APPROVED WITH NOTES; five valid findings are folded:
  1. provenance must be `unknown` until a representation is chosen, and the
     prerequisite PRD may trigger an ADR at decision time;
  2. all-or-nothing publication includes intent artifacts, sidecars and
     `status.json`, not just staged Markdown;
  3. compare `prepare` explicitly with `cycle`, `next` and the individual
     manual phases;
  4. richer validation is advisory for existing routing unless a later PRD
     enumerates a deliberate behavior change;
  5. map each council lens to its position in the durable record.
- **Coupled observation accepted**: update the whitepaper index for all tracked
  WP-002/WP-003/WP-005 documents.
- **Protocol**: Turn 2 remains immutable; corrections append as Turn 3.

## Turn 3 Revision Result

- Removed the broken guarded-WP-004 link without staging unrelated WIP.
- Mapped every advisor lens to its position and explained the 3/2 peer-review
  selection that produced the validation-first verdict.
- Defined legacy/current provenance as `unknown` until accepted persistent
  metadata proves otherwise; PRD-1 may trigger an ADR when it makes that
  choice.
- Expanded atomic publication to Markdown, structured sidecars and
  `status.json`; distinguished shared validation from incremental phase
  writers.
- Added the required existing-primitives pre-flight for individual
  `--manual`, `cycle` and `next`.
- Made slice-1 validation advisory and routing-compatible by default.
- Refreshed the tracked whitepaper index for WP-002, WP-003 and WP-005.

## Turn 3 Review Verdict

- **Internal**: APPROVED.
- **External**: APPROVED WITH NOTES.
- Every rev-0 finding is closed.
- LOW N1 closed by attributing council versus review-derived Agreed items.
- LOW N2 is pinned as a required first-PRD acceptance decision: report-only
  versus stronger mutating `--manual` gates and their compatibility.
- Turn 4 records the disposition append-only.

## Post-Release Review Adjudication

- **External verdict**: APPROVED WITH NOTES.
- **F1 LOW — valid**: Wave C extraction left 16 accepted-contract
  `verify.go:<line>` anchors stale; re-resolve them against current symbols
  and refresh the validation marker.
- **F2 LOW — valid**: `TestAcceptanceLedger_TestsExist` can false-pass on a
  comment because it searches raw bytes. Replace it with package-aware AST
  resolution and a sensitivity regression.
- **Observation — deferred explicitly**: the wave-close untracked-source
  sentinel cannot enumerate source files inside an untracked nested Git
  repository. Registered as GH #9; no release invariant is invalidated.
- **Tag policy**: fold only on `main`; never move v0.15.1.
- **Next planning work, deferred one turn by the operator**: append WP-005
  council Turn 2, then draft artifact-validation/provenance and prepare-bundle
  PRDs through the normal writer/review loop.

## Post-Release Fold Result

- Every verify-family citation in ADR-013 and PRD-verify-freshness was
  re-resolved against the extracted source at tag `15560af`; a non-vacuous
  bounds guard now checks the full reference set.
- Stale present-tense `active`-parent prose was corrected to describe the
  shipped Wave C behavior and its current source sites.
- The 161-row ledger now resolves an exact package, runnable
  `func TestX(*testing.T)` declaration, and optional literal subtest through
  Go AST. Comments, wrong/external packages, invalid signatures, unused table
  names and unrelated literals all fail sensitivity fixtures.
- Three internal review passes closed package/signature/subtest and citation
  scope/semantics findings. The final WIP-file observation was dismissed:
  those files predate this task, are guarded, and have an empty diff.
- Full uncached suite, vet and build pass.
- GH #9 records the deferred nested-repository source-sentinel blind spot.
- v0.15.1 remains byte-fixed at `15560af`; the fold is main-only at `64010bf`.

## Rev-3 Review Verdict

- **Internal**: APPROVED.
- **External/original reproducer**: APPROVED.
- No high-confidence defect remains.
- The original pre-land → land → post-land sequence and `verify --all` pass.
- The C-locale, offline apply-classifier grammar resisted fatal, mixed,
  wrapper, signalled, translated and broad-phrase adversarial outcomes.
- All rev-0 through rev-2 findings remain closed.

## Session Summary — rev-3 fold

**P1 — deterministic locale.** `internal/gitutil/trailers.go` appends
`GIT_NO_LAZY_FETCH=1` and the new `CLocaleEnv` (`LC_ALL=C`) LAST to
every evidence invocation, so both win over the inherited environment
AND any caller-supplied extra. That covers `log`, `rev-parse`,
`read-tree`, `apply`, `diff`, `cat-file` and `merge-base` — rev-2
forced the locale only on the `-C0` ladder step, leaving every other
classified diagnostic at the mercy of the ambient locale. The
`ForceCLocale` option is removed rather than left as a second, weaker
path. The workflow gateway's `offlineEnv()` carries both entries too, so
the shadow worktree commands are equally deterministic. Unrelated
callers are untouched: `shadowEnv(nil)` still returns nil.

**P1 — narrow, safe `ApplyProbeAnswered`.** The broad substring list
(`already exists`, `new file`, `deleted file`, `patch does not apply`,
…) is gone. The rule is now:

- success → answer;
- exit 1 → answer, by exit code alone (measured: every ordinary
  conflict exits 1, so no text is consulted);
- any exit other than 0/1/128 → FAILURE regardless of stderr, which
  covers signalled and unstartable processes (-1), 2, 126, 127, 129+;
- exit 128 with a missing-object or network diagnostic → FAILURE;
- exit 128 → answer ONLY when every non-empty stderr line matches the
  anchored C-locale malformed-patch grammar, with at least one
  recognised diagnostic line.

The grammar is `IsMalformedPatchDiagnostic`: anchored regexps for
`No valid patches in input (allow with "--allow-empty")`, `corrupt patch
at <path>:<line>` and `at line N`, `patch fragment without header`,
`patch with only garbage at line N`, and `corrupt binary patch`
(including its paired no-valid-patches line), plus `Checking patch …`
as an informational line that can never satisfy the requirement alone.
A `fatal:` spoof, a wrapper line, an unknown line, an empty diagnostic
or any mixture is rejected.

Measured on git 2.55.0 under `LC_ALL=C` (probe repo removed):

| input | exit | stderr |
|---|---|---|
| empty / garbage / prose | 128 | `error: No valid patches in input (allow with "--allow-empty")` |
| truncated or garbage hunk | 128 | `error: corrupt patch at ../p.patch:5` |
| fragment without header | 128 | `error: patch fragment without header at ../p.patch:1: @@ …` |
| corrupt binary payload | 128 | `error: corrupt binary patch at …` + the no-valid-patches line |
| new file that already exists | **1** | `error: f.txt: already exists in index` |
| delete of a missing file | **1** | `error: gone.txt: does not exist in index` |
| context mismatch | **1** | `error: f.txt: patch does not apply` |

`unrecognized input` was NOT reproducible on this git and is therefore
excluded — the classifier fails closed to `unavailable` for anything it
cannot demonstrate.

## Current State

- GH #8 stays empirically closed with the rev-3 binary, including under
  a foreign ambient locale (`LC_ALL=fr_FR.UTF-8`): before land passes,
  after land `landing evidence: exact` with a dual-anchor baseline, and
  `verify --all` exits 0.
- AC-L68 / AC-L69 remain green against a real filtered remote.
- No blocker.

## Files Changed (rev-3)

New:

- `internal/gitutil/apply_classifier_test.go`
- `internal/workflow/verify_rev3_fold_test.go`

Modified:

- `internal/gitutil/trailers.go` (`CLocaleEnv`, unconditional locale,
  anchored grammar, exit-code-led `ApplyProbeAnswered`; `ForceCLocale`
  removed)
- `internal/workflow/verify_gitgate.go` (`offlineEnv` carries the
  locale)
- `internal/workflow/verify_landed.go` (ladder no longer passes a
  per-call locale flag)
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
- **Regression proof.** Against the rev-2 production files, 6 rev-3
  workflow sub-tests fail (four broad-phrase historical-V8 exits, the
  ladder broad-phrase case, and the C-locale assertion). The classifier
  itself is proven in-tree by
  `TestRev3ClassifierFixesRev2Misclassifications`, which reproduces the
  rev-2 predicate verbatim and enumerates ten outcomes it promoted to a
  patch verdict and rev-3 rejects;
  `TestRev3ClassifierKeepsEveryRealAnswer` proves no real git answer was
  lost.
- Real-git goldens: `TestRealGit_MalformedPatchesExit128AndAreAdmitted`
  and `TestRealGit_OrdinaryConflictsExitOne` measure the actual
  diagnostics rather than mocking them.
- Acceptance ledger: 161/161 rows mapped; AC-L26/L58/L69/L117/L123/
  L129/L134 remapped to the rev-3 black-box and golden proofs.
- Docs totality guard (AC-L135 / G1–G10): zero hits; accepted documents
  untouched.
- GH #2 regression green and unmodified (AC-L121 diffs it against
  WAVE_BASE `b768602`).
- Side Research md5: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Complete writer rev-0 and transition to review.
2. Run independent internal/external PRD reviews.
3. Fold revisions until dual acceptance.
4. Keep `PRD-prepare-intent-bundle.md` blocked throughout.

## Blockers

None.

## Context for Next Agent

Reviewer focus for rev-3:

1. **Exit 1 is decided WITHOUT reading stderr.** That is what makes the
   grammar safe to narrow: every ordinary conflict — including the
   `already exists` / `does not exist` / `does not apply` forms rev-2
   listed — exits 1, so removing them from the grammar cannot cost a
   real answer. `TestRealGit_OrdinaryConflictsExitOne` measures it.
2. **Exit 128 is the only text-sensitive case**, and it is required:
   git reports a malformed or empty patch with exit 128, and two shipped
   GH #2-era rows depend on that staying a patch-level answer.
3. **The grammar fails closed.** A form that could not be demonstrated
   on this git (`unrecognized input`) is excluded; an unrecognised
   diagnostic yields `unavailable` (R10), which is the honest "the
   reader could not answer" outcome rather than a fabricated patch
   verdict.
4. **Locale is applied at the ONE env builder**, so there is no second
   path that can drift. `ForceCLocale` was deleted rather than kept as a
   no-op, to remove the ambiguity of two mechanisms for one guarantee.

Carried forward from rev-0/rev-1/rev-2 (still true):

- `landing_evidence.state` is OMITTED for the D10 artifact-presence
  short-circuit; `failed_at` and `reason` carry the outcome.
- Classification terminals run before the static short-circuit and
  allocate no shadow; unanswerable probes route through
  `terminalEvidencePhase` so remediations stay inside the closed
  R10/R22 set and `failed_at` inside the thirteen-value vocabulary.
- V2 parses the captured recipe bytes; the verify path has no live
  artifact read (AST-guarded).
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
