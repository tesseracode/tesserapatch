# Current Handoff

## Status

**Cluster state**: REV-1 DISPATCHED

**WAVE_BASE**: `c6aaeb2` (Cluster F planning + rev-5 verb-collision amendment, 2026-08-05).

**2026-08-05 Cluster F' rev-0 adjudicated NEEDS REVISION → rev-1 dispatched.** Dual review split: internal BLOCKED (6 findings), external APPROVED WITH NOTES (3 findings, 1 convergent with internal). Convergent finding (F-INT-4 == F-EXT-1) HIGH-confidence: rejected-parent edge refusals exit 1 not 3 across `feature deps add` and `amend --depends-on`. Explored-state pre-flag resolved in implementer's favor by both reviewers — PRD §5 clarifies no distinct `StateExplored`; explore output lives under `defined`.

**Rev-1 fold scope** (7 findings, all folded — internal-strict adjudication precedent):
1. **F-INT-1 BLOCKING** — history schema: rewrite to append ONE completed-cycle entry on reopen only (not per action), using field names `rejected_by`/`reopened_by`/`reject_note`/`reopen_note`/`reject_evidence`/`reopen_evidence`/etc. per PRD §6 spec. Assertions: 0 entries after reject, 1 after reject→reopen (per PRD §8 examples). Reject action alone must NOT append.
2. **F-INT-2 HIGH** — `status --json`: introduce dedicated DTO (not raw `FeatureStatus`) with §8-conformant field names (`rejected_by` not `actor`) + conditional emission (rejection object present only when `state == rejected`, per PRD §8).
3. **F-INT-3 HIGH** — validation ordering: evidence validation (path resolve, safety check, hash) MUST run BEFORE state-machine check per D4 addendum. Exit 2 (validation) precedes exit 3 (state refusal). Applies symmetrically to reject and reopen.
4. **F-INT-4 / F-EXT-1 HIGH** (convergent): wrap `store.ErrRejectedParent` at `runFeatureDepsAdd` and `applyAmendDependsOn` boundaries in `stateRefusalError` / `&ExitCodeError{Code: 3}`. Tighten `TestRejectDependencySymmetry_EdgeOntoRejectedParentRefused` to assert `code == 3` for all 3 edge kinds × both surfaces. Also reconcile golden-string wording to PRD §8 ("cannot add dependency: … first if this dependency is still needed") and emit §8 envelope `parent`/`kind` fields if surface has `--json`.
5. **F-INT-5 MEDIUM** — evidence resolution: only fall through to repo-root candidate on genuine not-found. Return immediately for non-regular / unsafe / unreadable feature-dir candidates. Prevents hashing wrong file.
6. **F-INT-6 LOW** — test 27: use independent literal expected strings (not the production constants). Golden-string assertion must break if either production constant changes.
7. **F-EXT-2 LOW** — wrong-state reject error: either render list with trailing "or" (per PRD §8 golden string) OR update PRD §8 to match implementation's comma-form. Preferred: fix implementation, keep PRD as spec.
8. **F-EXT-3 LOW adjudication** — history_entries: PRD §8's `1` after one reopen is authoritative. Fold subsumed under F-INT-1 (correct schema produces exactly `history_entries: 1` after one reject→reopen cycle).

**Precedent invocation**: internal-strict adjudication — when internal reviewer catches wire-schema violations that external's example-reading passes over, sever severity by internal's classification. Matches Cluster F planning rev-0 where internal BLOCKED (8) and external APPROVED WITH NOTES (2) reflected same reviewer-strictness split with internal reading spec more strictly. Rev-descent expected: internal 6 → target 0 by rev-1 or rev-2.

**2026-08-05 Cluster F rev-5 SHIPPED at `c6aaeb2` — docs-only micro-fold amending planning archive.** Post-Cluster-F external F1 LOW-MEDIUM (`tpatch reject` verb collision with pre-existing `tpatch reconcile --reject <slug>` flag at `cobra.go:2093`) resolved via **Alternative 3**: kept bare `tpatch reject`/`tpatch reopen`, documented intentional non-relationship in PRD §4.1 (4-point rationale + 5 mitigations) and ADR-031 D10 (3-alternatives analysis). Test 27 (`--help` cross-reference golden-string assertion) added. Reviewer's suggested Alt-1 (`tpatch feature reject`) explicitly rejected — `feature` group is noun-scoped per `feature_deps.go:41-49,52-56`; retrofitting a lifecycle verb there contradicts `amend --state`'s reservation at `c1.go:276-284`. External rev-5 confirmation: APPROVED WITH NOTES, 1 LOW residual F2 (§4.1 point 2 imprecise precondition wording for `runReconcileReject`; reviewer explicitly deferred to Cluster F' pickup; non-overlap conclusion unaffected). Range `e493a2d..c6aaeb2` (1 commit).

**Cluster F rev-5 F2 residual (for Cluster F' pickup)**: PRD §4.1 point 2 says `reconcile --reject` "fires only when a feature is in `reconciling-shadow`." Actually `runReconcileReject` (`cobra.go:2794-2826`) fires whenever a shadow is registered (which can occur in `applied`/`active` states as cleanup); only the state rollback to `applied` is gated on `reconciling-shadow`. Non-overlap conclusion holds because `tpatch reject` is refused from all states with shadows. Fix during Cluster F' implementation: reword §4.1 point 2 precondition to "fires only when a shadow worktree is registered (pruning it; rolling state back to `applied` only from `reconciling-shadow`)".

**2026-08-05 Cluster F planning SHIPPED at `377d103`.** PRD + ADR pair for v0.13.0 GH #6 first-class `rejected` feature lifecycle state. 4 review revs (rev-0 through rev-4), three-way APPROVED at rev-4. Range `8574ff3..377d103` (10 commits: 2 rev-0 impl + 2 rev-1 impl + 2 rev-2 impl + 1 rev-3 impl + 1 rev-4 impl + 5 supervisor tracking, plus adjudication + consolidation). Key architectural decisions locked-in: content-hash evidence (`{path, sha256}` lowercase-hex); post-implementation reject OUT OF SCOPE (deferred to future ADR); exit-code envelope 0/1/2/3; CLI shape `--reason` + mandatory `--note` + optional `--evidence`/`--actor`; actor precedence chain; symmetric dependency invariant; reopen unbounded append-only with historical-evidence verification on every reopen. Convergence arc: internal 8→5→3→1→0; external 2→3→2→1→carry.

**2026-08-05 Cluster F DISPATCHED (planning-first — v0.13.0 GH #6).** First-class `rejected` feature lifecycle state per GH #6. Planning phase: PRD + ADR pair. This is a data-model extension (not just a CLI addition), so architectural coverage is required before any code lands. Dual review at planning approval; implementation phase becomes Cluster F' after PRDs+ADRs land three-way APPROVED.

**2026-08-05 Cluster E-prime SHIPPED + E'-N2 amendment (post-Cluster-E hygiene follow-up).** Two LOW observations from external's post-Cluster-E review folded. Scope: Obs 1 (`PinGitAutoGCOff` doc comment clarifying unconditional `GIT_CONFIG_COUNT=1` clobber semantics; mechanism unchanged); Obs 2 (`.wave-close-allowlist` at repo root — Makefile `[2/8]` subtracts allowlisted entries from WARN list; prints `OK (N entries allowlisted)` when residual is empty; residual still WARNs). Post-ship amendment (`189f5d6`): E'-N2 sentinel-self-guard fold — `.wave-close-allowlist` added to `[2/8]` glob list so sentinel catches its own untracked-config case. Post-E-prime external review verdict "NEEDS REVISION" evaluated: F1 HIGH / F2 part-1 / F3 were all against reviewer's stale local pre-fetch tree — only F2 part-2 (self-guard) was state-independent and genuinely valid. E'-N1 (stale-entry bitrot) remains backlog. Range `2281309..8574ff3`.

**2026-08-04 Cluster E SHIPPED (process housekeeping).** Two findings from external's post-Cluster-D review + 1 rev-1 fold. Scope: F1 MEDIUM (`make wave-close-check` never ran `go test` — gate PASSed with red suite empirically demonstrated at Cluster D HEAD; fixed by adding `[8/8] go test -count=1 ./...`), F2 LOW (`t.TempDir()` teardown race on macOS from unpinned `git commit` forking `gc --auto --detach`; fixed by `gc.auto=0` env pin), E-EXT-1 MEDIUM rev-1 fold (F2 pin was `internal/cli`-only; extracted `internal/testutil.PinGitAutoGCOff()` and applied to `internal/gitutil`, `internal/workflow`, `internal/store`). Two-opinion scoreboard: rev-0 dual (internal APPROVED, external APPROVED WITH NOTES 1 MEDIUM), rev-1 external-only confirmation (APPROVED WITH NOTES — 2 non-functional commit-message accuracy notes, no code defects). Range `1bc2a25..b294d8c`.

**2026-08-03 Cluster D SHIPPED.** Correctness housekeeping — 8 items total, single implementer, sequential. Four review revs (rev-0 → rev-3). Two-opinion protocol scoreboard: rev-0 dual (internal NEEDS REVISION 3 MEDIUM + 1 LOW, external APPROVED WITH NOTES 1 MEDIUM overlap), rev-1 dual (internal NEEDS REVISION 1 MEDIUM residual, external APPROVED), rev-2 external-only (NEEDS REVISION 1 MEDIUM new Rule 17 residual), rev-3 external-only (APPROVED via prescriptive verbatim wording). **Notable pattern**: three consecutive iterations on the same fast-path help clause each introduced a new Rule 17 residual; broken by supervisor-prescribed verbatim text at rev-3. All 6 backlog items + 2 review-fold items landed. Deferred: D-INT-2 (--from-revision post-crash out of PRD-#4 F-4 scope), F-EXT-2 (concurrency out of local-CLI scope). Range: `4868f68..42f85d7` (13 commits: 8 rev-0 impl + 3 rev-1 folds + 1 rev-2 fold + 1 rev-3 fold, plus 4 tracking commits).

**2026-08-03 Cluster D DISPATCHED.** Correctness housekeeping — single implementer, sequential, small-scope items. Scope: 6 backlog items (PRD-#3 N2/N3/S1, PRD-#4 F-4, GH #5 docs, Wave γ LOW-γr15-N1) + 2 review-fold items from external's post-Cluster-C review (F1 MEDIUM: gate glob gap; F2 LOW carry-over: LOG SHA pointer).

**v0.12.1 SHIPPED 2026-07-31.** Correctness fix pass (GH #3 + #4 + #5) three-way APPROVED at rev-1 across all three tickets. Cluster A (AGENTS.md wave-close checklist) shipped earlier same day. All work pushed to `origin/main`. v0.12.1 tag pushed.

**2026-08-02 CI back green on `main`.** Inline hygiene fix at `4619b55` — `gitInitTestRepo` pinned to `-b main` — resolved a persistent CI failure class (red since 2026-07-28).

**2026-08-02 Cluster C SHIPPED.** Process housekeeping — parallel-implementer discipline addendum + `make wave-close-check` mechanical gate. Four review revs total (rev-0 → rev-4). Two-opinion protocol scoreboard: **external-only catches** on rev-0 (BLOCKING unpushed), rev-1 (3 HIGH + 2 MEDIUM incl. empirical false-passes), rev-2 (1 HIGH duplicate-field), rev-3 (1 BLOCKING shell-bug + 1 non-blocking). Internal APPROVED at rev-1 and rev-2; rev-3 and rev-4 were external-only cycles for single-issue empirical fixes. Cluster C's own gate now dogfoods on every commit going forward. Range: `bb31872..870182d`.

## Active Task

**Cluster F' — v0.13.0 GH #6 first-class `rejected` feature lifecycle state (implementation phase).** Pending dispatch. WAVE_BASE `c6aaeb2` (Cluster F planning + rev-5 verb-collision amendment). Planning-baseline: `docs/prds/PRD-rejected-feature-state.md` + `docs/adrs/ADR-031-rejected-feature-state-data-model.md`, both three-way APPROVED (rev-4 baseline + rev-5 docs-only amendment).

### Implementation scope (from planning baseline)

Touches (per ADR §7 Implementation Notes):
- `internal/store/types.go` — add `StateRejected` as 11th `FeatureState` value; extend `ValidFeatureState` closed switch.
- `internal/store/status.go` — add `Rejection` field to `FeatureStatus` (`{reason, note, actor, evidence []EvidenceRef, rejected_at, prior_state, history []RejectionHistoryEntry}`); `EvidenceRef{Path, SHA256 string}`.
- `internal/store/validation.go` — add Rule 7 (edge-creation refused if target parent is `rejected`) covering `hard`/`soft`/`supersedes`; extend at existing `ValidateDependencies` function (`113-210`).
- `internal/cli/cobra.go` — new `tpatch reject <slug>` and `tpatch reopen <slug>` commands; `tpatch status` filtering (`--include-rejected` opt-in); `tpatch next` rejection-aware output; guard on `applyConfirmUpstreamedTransition` entry (before reconcile-revision append at `:2535` and `saveConfirmUpstreamedStatus` at `:2554`) refusing on `rejected` source state.
- `assets/` — state enum doc/template updates (parity guard `assets_test.go`).
- `SPEC.md` — new state documentation.
- Tests — PRD §9 27 items (26 + 26b sub-test + test 27 rev-5 `--help` cross-ref): reject from allowed states, refuse from post-implementation states, evidence content-hash + path safety, reopen unbounded append-only, note-only reopen historical verification, divergent_reason taxonomy, dependency-order symmetry (both orders × 3 edge types), CLI shape, JSON envelope, exit-code envelope 0/1/2/3, actor precedence, confirm-upstreamed defense-in-depth guard, `--help` disambiguation for `reject` ↔ `reconcile --reject`.

Do NOT touch (orthogonal per ADR D6): `internal/workflow/reconcile.go` and its `RetirementAudit` on `ReconcileResult`.

### Constraints (per AGENTS.md + CLAUDE.md + planning baseline)

- Implementation phase = code + tests. All planning-baseline decisions binding (see PRD/ADR at `377d103`).
- Content-hash SHA-256 lowercase-hex encoding (`encoding/hex.EncodeToString`).
- Exit-code envelope: 0 success / 1 unexpected error / 2 pre-mutation validation / 3 state-machine refusal.
- CLI shape: `tpatch reject <slug> --reason <enum> --note <string> [--evidence <path>...] [--actor <string>]`; `tpatch reopen <slug> --note <string> [--evidence <path>...] [--actor <string>]`.
- Actor precedence: `--actor` > `TPATCH_ACTOR` > `git config user.email` > `"unknown"`.
- Explicit `git add <path>`; NEVER `-a`/`-A`. `git commit -F <tempfile>`; NEVER inline heredoc. Rule 18 trailer.
- Side Research md5 `b385fe622db9926f48861105239f113e` MUST remain preserved.
- Do NOT touch canonical `**Cluster state**` field.
- Do NOT stage the 15 untracked WIP files (allowlisted).
- Explicit backward-compatibility path per ADR D5 migration.
- Assets parity guard test must pass.

### Non-goals

- Do NOT re-open planning decisions. All 10 decision points D1-D10 are binding.
- Do NOT extend post-implementation reject scope (D6: OUT OF SCOPE, deferred to future ADR).
- Do NOT modify `internal/workflow/reconcile.go` or its `RetirementAudit` field.
- Do NOT block on E'-N1 (allowlist stale-entry bitrot) — orthogonal.

## Session Summary

- **v0.12.0** (three-wave feature cluster: supersession + write-file safety + active-feature-session) — shipped, tagged `v0.12.0`.
- **Cluster A** (AGENTS.md wave-close checklist codifying F1 pattern) — shipped at `5ac458d`.
- **Cluster B planning** (PRDs #3 + #4 with dual-review parallel) — shipped at `4e673a8`.
- **v0.12.1 implementation** (GH #3 + #4 + #5 correctness fix pass) — shipped at `bb31872`, tagged `v0.12.1`.
- **CI hygiene fix** — `4619b55` pinned `gitInitTestRepo` to `-b main`; CI back green 2026-08-02.
- **Cluster C** (parallel-implementer discipline + `make wave-close-check` mechanical gate) — shipped at `4868f68` after 4 review revs.
- **Cluster D** (correctness housekeeping — 6 backlog items + 2 review-fold items) — shipped 2026-08-03 after 4 review revs. Range `4868f68..42f85d7`.
- **Cluster E** (process housekeeping — F1 gate `[8/8] go test` coverage + F2 macOS teardown race via `gc.auto=0` pin extracted to shared `internal/testutil` helper) — shipped 2026-08-04 after 1 rev-1 fold (E-EXT-1 cross-package pin). Range `1bc2a25..b294d8c`.
- **Cluster E-prime** (post-Cluster-E review follow-up — Obs 1 `PinGitAutoGCOff` doc comment + Obs 2 `.wave-close-allowlist` mechanism for `[2/8]` gate step) — shipped 2026-08-05 external-only rev-0 APPROVED WITH NOTES. Range `2281309..aa34f3c`.
- **Cluster F planning** (PRD-rejected-feature-state + ADR-031 pair, v0.13.0 GH #6) — shipped at `c6aaeb2` after 5 review revs (rev-5 = verb-collision amendment).
- **Cluster F' rev-0** (v0.13.0 GH #6 implementation — first-class `rejected` feature lifecycle state) — implemented 2026-08-06, **awaiting review**. 10 commits, range `8cf3c1a..HEAD`. See "Ready for review — Cluster F' rev-0" below.

## Files Changed — Cluster F' rev-0

Source (new):
- `internal/store/status.go` — `EvidenceRef`, `DivergenceDetail`, `RejectionHistoryEntry`, `RejectionStatus`, evidence-integrity + divergence-taxonomy + history-action constants, `RejectableStates`/`IsRejectableState`/`RejectableStateList`.
- `internal/store/rejection_reason.go` — closed 7-value reason enum + `IsValidRejectionReason`, `RejectionReasonList`, `RejectionReasonsJoined`.
- `internal/store/actor.go` — `ResolveActor` / `ResolveActorIn` 4-tier precedence chain + `gitConfigUserEmail` test seam.
- `internal/cli/reject.go` — exit-code helpers, evidence normalization/resolution/hashing, `rejectCmd`/`runReject`, the two disambiguation golden strings, `refuseIfRejected`.
- `internal/cli/reopen.go` — `verifyHistoricalEvidence`, `reopenCmd`/`runReopen`.

Source (modified):
- `internal/store/types.go` — `StateRejected` (11th value) + `ValidFeatureState` arm; `FeatureStatus.Rejection *RejectionStatus` (`omitempty`).
- `internal/store/validation.go` — `ErrRejectedParent` sentinel, `rejectionReasonSuffix`, Rule 7 in `ValidateDependencies`.
- `internal/store/store.go` — `RefreshFeaturesIndex` partitions active/rejected and renders a trailing `## Rejected` table; `singleLineCell` pipe-escaping helper.
- `internal/cli/cobra.go` — `reject`/`reopen` registration; `status --include-rejected` + `rejected_hidden` + always-full detail view + `pluralEntries`; `apply` rejected guard; `reconcile` per-slug rejected filtering; `reconcile` Long description carrying the symmetric cross-reference; `applyConfirmUpstreamedTransition` rejected guard as first statement.
- `internal/cli/phase2.go` — `nextAction` `case store.StateRejected` arm.

Tests (new):
- `internal/store/rejection_test.go` (32 assertions)
- `internal/cli/reject_test.go` (67 assertions)

Docs/assets:
- `SPEC.md` — `rejected` state row + rationale; "Feature rejection" command subsection; `reject` ↔ `reconcile --reject` non-relationship; exit-code envelope table (stated as per-command, not global).
- All 6 shipped skill surfaces + `assets/assets_test.go` — 2 new `requiredCommands`, 3 new parity anchors.
- `docs/prds/PRD-rejected-feature-state.md` — §4.1 point 2 F2 residual wording fix.

## Files Changed at v0.12.1 Consolidation

- `CHANGELOG.md`: v0.12.1 header dated 2026-07-31; GH #4 review-path subsection added; rev-1 fold-in subsection appended.
- `docs/ROADMAP.md`: v0.12.1 ✅ SHIPPED section added above v0.12.0.
- `docs/prds/PRD-confirm-upstreamed-human-review-path.md`: Status `Proposed` → `Accepted`.
- `docs/handoff/HISTORY.md`: v0.12.1 archived under 2026-07-31 header.
- `docs/handoff/CURRENT.md`: reset (this file).

## Test Results — Cluster F' rev-0

- `gofmt -l .` empty; `go vet ./...` clean; `go build ./cmd/tpatch` OK.
- `go test -count=1 ./...` — **960 top-level PASS / 1237 including subtests, 0 FAIL** across all 9 test packages (up from 907 top-level pre-cluster; +99 new rejection assertions counting subtests).
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e` (verified post-implementation).

### `make wave-close-check WAVE_BASE=c6aaeb2`

| Step | Result |
|------|--------|
| `[1/8]` working tree clean | **OK** |
| `[2/8]` untracked-source sentinel | **OK** (16 entries allowlisted; no WIP file staged) |
| `[3/8]` HEAD pushed to `origin/main` | **OK after the rev-0 push** (FAILed pre-push by construction) |
| `[4/8]` Rule 18 trailer on every wave commit | **OK** (12 commits in `c6aaeb2..HEAD`) |
| `[5/8]` canonical `**Cluster state**:` terminal | **FAIL by design** — reads `REV-0 DISPATCHED`. This is a mid-cycle implementation handoff awaiting review, and the dispatch brief explicitly forbids the implementer touching the canonical field. **The supervisor flips it to a terminal token at wave close.** |
| `[6/8]` gofmt clean | **OK** |
| `[7/8]` go vet + go build clean | **OK** |
| `[8/8]` `go test -count=1 ./...` clean | **OK** |

Manual items remain for the supervisor: LOG entry, ROADMAP flip, HISTORY archive, tag.

## Next Steps

**Backlog after Cluster E-prime**:

Feature / release:
- **Cluster F — v0.13.0 GH #6 first-class rejected feature state** — data-model extension, PRD + ADR pair. Larger planning-first cluster. Only remaining open GH issue.

Deferred from Cluster E-prime rev-0 external review (documented, no fold — reviewer's explicit "not required for this rev to ship"):
- **E'-N1 LOW** — `.wave-close-allowlist` stale-entry bitrot is silent. Allowlist entries whose files land (via `git add`) or delete are silently ignored — no active flagging in gate, no protocol coverage in AGENTS.md pruning guidance. Passive signal only via `(N entries allowlisted)` count trend. Two fix options: (a) active stale-entry sub-check reporting patterns matching zero untracked files as "candidate for removal"; (b) extend AGENTS.md checklist bullet to require pruning when a file lands/deletes. Fold into Cluster F pre-flight or next hygiene cluster if allowlist grows beyond initial 16 seed.

Deferred from Cluster D adjudication (documented, no fold):
- **D-INT-2** (`--from-revision <original>` post-crash "superseded" error) — PRD-#4 lines 180/259 document the flag as CI/test override, not the crash-recovery path. Default retry works (external Rule 20 verified). Backlog if operator friction surfaces.
- **F-EXT-2** (concurrency safety of confirm-upstreamed) — pre-existing; concurrent invocation of same slug not a supported local-CLI scenario.

Untracked WIP (surfaced by Cluster D Item 7 gate glob extension; NOT staged by Cluster D):
- `docs/whitepapers/WP-004..WP-007.md` + `.turns.md` siblings (8 files).
- `docs/prds/PRD-feature-unapply.md`, `docs/prds/PRD-recurring-patches.md`.
- `docs/state-of-the-art/*case-study*` (2 files).
- These require operator decision on disposition; not a defect.

Process / hygiene (all shipped this session):
- ✅ AGENTS.md parallel-implementer discipline addendum (Cluster C).
- ✅ Mechanical wave-close-check gate (Cluster C).
- ✅ Gate glob covers whitepapers + state-of-the-art (Cluster D fold F1).

Documentation:
- **ADR-027 F2** (nit).
- **Doctor S3-boundary deferrals** (from Wave β).
- **ADR-029 nit deferrals**.

## Blockers

None.

## Context for Next Agent

- **v0.12.1 SHIPPED** — do NOT re-open Wave α/β/γ or GH #3/#4/#5 scope. All accepted.
- **Two-opinion protocol proven load-bearing again** — v0.12.1 rev-0 external caught 4 findings internal missed (PRD-#4 warning wording, PRD-#4 tie-break correctness bug, PRD-#3 err-branch gap, GH #5 hint mislabel). Internal caught PRD-#3 F-INT-3-1 HIGH (Rule 18 trailer parse failure). Continue dual-review protocol on all clusters ≥ paper-only.
- **Cross-implementer entanglement is now a KNOWN failure mode** — do NOT dispatch parallel implementers to shared source files without briefing them on `git add <path>` discipline. See Cluster A follow-up in backlog.
- **20 binding carry-forward rules** unchanged. Rule 18 empirical demonstration this cluster: heredoc-authored commit bodies leaked `EOF)` after the trailer, breaking `%(trailers)` parse. Rule 20 empirical demonstration: PRD-#4 external caught the tie-break bug via code path enumeration (in-place dedup) that internal's tests-pass verdict didn't surface. Rule 20 continues to require empirical repro even on paper-approved designs.
- **Side Research md5 invariant**: `b385fe622db9926f48861105239f113e`. Verify: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.
- **Commit trailer**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` verbatim + `Copilot-Session: <session-id>` per session. Use `git commit -F <tempfile>` or `git commit -m ""` — NOT `git commit -F -` with heredoc (heredoc close tokens leak into the body).

## Ready for review — Cluster F' rev-0

**Scope delivered**: v0.13.0 GH #6 first-class `rejected` feature lifecycle state, complete per PRD-rejected-feature-state + ADR-031 (D1-D10 treated as binding). Commit range `8cf3c1a..HEAD` (10 commits).

### Commit map

| # | Commit | Scope |
|---|--------|-------|
| 1 | `f688a70` | store: `StateRejected` + `RejectionStatus` data model |
| 2 | `8bb5ce8` | store: Rule 7 dependency guard |
| 3 | `026c179` | store: actor resolution + closed reason enum |
| 4 | `7dfe160` | cli: `tpatch reject` |
| 5 | `8d46631` | cli: `tpatch reopen` + historical evidence verification |
| 6 | `71a73b0` | cli,store: status/next/FEATURES.md + apply/reconcile/confirm-upstreamed guards |
| 7 | `588a957` | spec,assets: `rejected` documentation + parity anchors |
| 8 | `20f1590` | test: PRD §9 matrix (2 files, 99 assertions) |
| 9 | `b06571d` | docs: F2 residual — PRD §4.1 point 2 wording fix |
| 10 | (this) | docs: CURRENT.md session summary |

### F2 residual — CONFIRMED FOLDED

PRD §4.1 point 2 now reads "fires only when a shadow worktree is registered (pruning it; rolling state back to `applied` only from `reconciling-shadow`)", replacing the imprecise "fires only when a feature is in `reconciling-shadow`". The non-overlap conclusion the point supports is unchanged. Commit `b06571d`.

### Adjudication requests for the reviewer

Four points where the **dispatch brief** and the **binding planning baseline** disagreed. In every case the implementer followed the PRD/ADR, on the grounds that `AGENTS.md` and this handoff both state "All 10 decision points D1-D10 binding" / "Do NOT re-open planning decisions", and the brief itself defers to the PRD for the reason enum. Flagging all four explicitly so the reviewer can overrule.

| # | Brief said | Implemented (PRD/ADR) | Anchor |
|---|-----------|----------------------|--------|
| 1 | Reject-eligible from `requested, analyzed, defined, implementing, blocked` | **`requested, analyzed, defined` only.** `implementing` and `blocked` refuse with exit 3. | PRD §5; ADR-031 D4 Consequences ("the reject-eligible state set (requested, analyzed, defined) as a shared symbol in `internal/store`"); D6 puts post-implementation retirement out of scope |
| 2 | 8 reason codes incl. `wont-fix`, `design-rejected`, `deferred`, `replaced-by` | **PRD §6's 7 codes**: `not-a-bug`, `premise-disproved`, `obsolete`, `out-of-scope`, `unsafe`, `duplicate`, `superseded` | PRD §6; brief itself says "Read PRD §6 for the exact authoritative list" |
| 3 | `reject --evidence` optional | **Required, ≥1 entry.** Exit 2 with `evidence required: at least one --evidence path must be supplied` | PRD §3.2, §6, §8 |
| 4 | `reopen` transitions to `Rejection.PriorState` | **Always → `requested`.** `PriorState` is retained purely for audit. | PRD §3.8, §5 ("rejected → requested only"), §8 envelope |

**One deliberate deviation from a PRD illustrative example** (not from a binding decision): history appends on **both** `reject` and `reopen`. ADR D1's narrative suggests history grows only on reopen, and PRD §8's illustrative reopen envelope shows `history_entries: 1`; this implementation emits `2` after one reject+reopen. Rationale: the `action` discriminator field (`"reject"` | `"reopen"`) that the brief and PRD §6 both specify is meaningless unless reject also appends, and D5's binding requirement (append-only, unbounded, nothing ever truncated) is fully satisfied either way. If the reviewer prefers the literal §8 number, the change is a one-line delete in `runReject` plus two test expectations.

**One exit-code judgement call**: "slug not found" maps to exit **2** (unresolvable input), not 3. PRD §9 item 5 says only "error" without specifying a code. This matches `tpatch verify`'s missing-slug precedent.

### Verification highlights the reviewer may want to re-run

- **Evidence path safety is checked on the symlink-RESOLVED path before any byte is read.** `TestReject_EvidenceSymlinkEscapeRefusedWithoutHashing` proves it through the `evidenceHashFn` seam. Both sides of the containment comparison are `EvalSymlinks`-resolved, which is required on macOS where `t.TempDir()` lives under `/var` → `/private/var`.
- **Reopen-time historical verification is unconditional.** `TestReopen_NoteOnly_CleanVerificationRuns` and `TestReopen_NoteOnly_DivergentHistoricalEvidence` (PRD §9 items 26 and 26b) assert it fires with zero `--evidence`, and that divergence never blocks (exit 0).
- **`confirm-upstreamed` guard precedes the audit append.** `TestConfirmUpstreamed_RefusedOnRejectedFeature` asserts both status.json AND the reconcile-revisions log are unchanged after the exit-3 refusal.
- **Dependency-order symmetry both directions × 3 edge kinds**: `TestReject_RefusedWhenDependentsExist` (reject-with-dependents) and `TestRejectDependencySymmetry_EdgeOntoRejectedParentRefused` (edge-onto-rejected-parent), plus `TestValidateDependencies_Rule7_RejectedParentRefused` at the store layer.
- **Test 27 golden strings**: `TestHelp_RejectAndReconcileCrossReference` asserts both `tpatch reject --help` and `tpatch reconcile --help` render their respective cross-reference constants verbatim.

### Non-invalidation invariants

- Side Research md5 `b385fe622db9926f48861105239f113e` — **preserved**, verified post-implementation.
- Canonical `**Cluster state**:` field — **untouched** (still `REV-0 DISPATCHED`; supervisor flips at close).
- `internal/workflow/reconcile.go` and `retirement_audit.go` — **untouched** (ADR D6).
- No file from `.wave-close-allowlist` staged; all 10 commits used explicit `git add <path>` and `git commit -F <file>`.
- Every commit carries the Rule 18 trailer + `Copilot-Session` trailer (`[4/8]` verified 12/12 in range).

## Backlog (post-v0.13.0 candidates, not for Cluster F prime)

- **prd-verify-post-commit-mode** (MEDIUM, external user report 2026-08-05). `tpatch verify` fails V8 `post_apply_patch_replay_clean` on already-committed features with a misleading "run tpatch reconcile" remediation. Root cause verified at `internal/workflow/verify.go:1160-1163`: V8 resets the shadow to the closure-replayed baseline (target recipe NOT applied) then git-apply-checks post-apply.patch — on a local repo whose HEAD already contains the feature, the check conflicts. PRD questions: detect no-upstream state, seed shadow from pre-feature base, conditional remediation string. Full description in todos table row `prd-verify-post-commit-mode`.
- **prd-no-upstream-mode** (MEDIUM, external user report 2026-08-05, sibling of prd-verify-post-commit-mode). Local-only mode for repos without configured upstream: `upstream_merged`/`reconciling`/`reconciling-shadow`/confirm-upstreamed states presume upstream exists. Design questions: detection (config flag vs. absence of `upstream.lock` vs. git remote absence vs. `tpatch init --local`), migration path when local repo later configures upstream, which lifecycle states become unavailable, CLI remediation-string changes across `verify`/`reconcile`/`next`/`land`/`status`. Likely paired with prd-verify-post-commit-mode as single "local-first tpatch" PRD/ADR pair. Full description in todos table row `prd-no-upstream-mode`.

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
