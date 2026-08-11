# Current Handoff

## Status

**Cluster state**: REV-2 DISPATCHED

**WAVE_BASE**: `f04dec7` (`origin/main` immediately before Cluster H
planning dispatch, 2026-08-10).

**2026-08-10 Cluster H rev-1 adjudicated NEEDS REVISION → rev-2
DISPATCHED.** Internal review: 5 HIGH + 2 MEDIUM. External review: 1 CRITICAL
+ 3 HIGH + 7 MEDIUM + 2 LOW. Golden IDs, four shared JSON blocks, 41 AC
clauses, 66 contiguous rows, and most corrected citations verified clean.
Remaining blockers: every Dolt argv combines mutually exclusive flags;
local persistent raw bodies and wall-clock/overwrite semantics still conflict
with ADR-027; symlink gate misses ancestor components and executable policy
rejects every valid Dolt location; tracked publication still spans N files;
wire variants and exit codes remain incomplete; locks/failure directories are
not crash-recoverable; local-ignore and literal-pathspec mandates lack exact
coverage; CURRENT counts drifted after addendum. Rev-2 replaces Dolt diff
flags with the source-verified `dolt_diff_summary` SQL interface, removes
persistent raw bodies, uses one immutable tracked batch + atomic pointer,
and closes path/lock/wire/tracking details.

**2026-08-10 Cluster H rev-1 WRITTEN — full fold of both rev-0 dual-review
verdicts, awaiting dual review.** Single sequential writer rewrote both
papers end-to-end from `dd08157` (rev-0) against the adjudication at
`89c8d79`. `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
is now 1396 lines (was 1086); `docs/adrs/ADR-033-resource-capture-boundary.md`
is now 664 lines (was 992, corrected from CURRENT's stale "963" rev-0
claim). Raw snapshot/diff/file bytes moved off every tracked artifact into a
new gitignored `.tpatch/local/resource-capture/<slug>/` lane (immutable
batches + one atomic `current`-pointer rename); `generic-command` removed
from v1 entirely (Dolt is the sole closed adapter); a mandatory
Lstat+EvalSymlinks+containment+`.git`-target-refusal gate now runs at both
`add` and every `capture`, for every path a feature touches; `ignored-file`
now requires both `gitutil.IsPathIgnored` **and** confirmed-untracked (the
`--no-index` gap is closed); `git-metadata` narrowed to 4 closed views with
exactly 4 non-PII `config` keys; Dolt's protocol uses only verified DoltHub
flags (no fabricated `--json`); `resource_id` uses a new sorted
canonical-JSON `args` encoding with 4 independently-recomputed golden
vectors (reordered-key equivalence proven); a new six-class hard-refusal
redaction scanner is required of Cluster H' (PEM/OpenSSH keys, DB/connection
URLs, emails/PII, credential assignments, bearer/token patterns, home
paths); `record --resources` now stages privately before Git-side capture
and publishes the resource pointer only after Git succeeds, with an honest
partial-domain error + recovery command when the two domains diverge; six
false/fabricated rev-0 citations were corrected (fabricated `ADR-031 D10`,
false "`EnsureSafeRepoPath` is symlink-aware," false "`verify` is
`ExitCodeError`'s sole user," normative reliance on untracked WP-006, a
stale `RemoveClaim` line range, and the rev-0 stale ADR line count itself).
See "Ready for review — Cluster H rev-1" below.

**2026-08-10 Cluster H rev-0 WRITTEN — v0.15.0 candidate typed feature
resources + capture adapters (planning only), awaiting dual review.** Single
sequential writer produced `docs/prds/
PRD-feature-resource-claims-and-capture-adapters.md` (1086 lines) and
`docs/adrs/ADR-033-resource-capture-boundary.md` (963 lines). The PRD extends
shipped v0.9 file claims/capture modes with a new, separate
`resources.json` manifest covering three closed v1 resource kinds
(`ignored-file`, `git-metadata`, `adapter-snapshot` with exactly two closed
adapters `dolt`/`generic-command`), a full adapter execution/sandbox
protocol, a snapshot/diff sidecar wire envelope mirroring the existing
`post-apply.patch`/`patches/NNN-*.patch` canonical-vs-audit pattern, and
`record --resources` all-or-nothing batch transaction semantics with no
`patch-generations.json` coupling. The ADR locks 10 binding decisions
(D1–D10, exceeding the required 8) with byte-identical wire examples shared
with the PRD, Implementation Notes for Cluster H', a Negative Consequences
Summary, and a 45-row test matrix providing exact 1:1 coverage of all 30 PRD
acceptance criteria plus 15 supporting safety rows. Binding safety honored:
raw `.git/**` remains forbidden at both the diff and store-write boundary
(reusing ADR-030's two-layer pattern), ignored files are explicit opt-in
gated by `gitutil.IsPathIgnored` (never swept implicitly), resource diffs are
v1 sidecar audit artifacts only (never canonical-patch/lifecycle authority —
no lifecycle command reads `resources.json`), Git remains the sole
replay/change-tracking substrate (WP-006 non-Git replay explicitly not
relied upon), and Dolt/`generic-command` are optional runtime-discovered
adapters with zero new core dependency and no adapter granted tpatch
authority. No code, assets, ROADMAP, SPEC, CHANGELOG, or supervisor/LOG.md
changes in Cluster H rev-0.

**2026-08-10 v0.14.0 post-release close-claim review APPROVED WITH NOTES.**
Headline release claims, 1022/0 count, tag/push, tracking, WIP preservation,
8/8 gate and invariants independently confirmed. Notes folded on `main`
without moving `v0.14.0`: disclose the runtime alignment that lets `active`
hard parents satisfy apply/land, rename its regression test without false
continuity, and repair SPEC conjunction/wrapping.

**2026-08-10 Cluster G' SHIPPED — v0.14.0 `tpatch feature unapply`.**
Six-revision implementation/review arc closed three-way APPROVED at rev-5
(internal APPROVED, external APPROVED, zero residuals). Code tip `6941d41`;
pre-consolidation handoff `633a95d`; 21 wave commits before release
consolidation. Delivered atomic status writes, `StateUnapplied`, D3 fixed
audit artifacts, D6 transaction/rollback, canonical direct reapply, complete
lifecycle/dependency/status integration, SPEC + six-skill parity, and all 61
matrix rows. Final suite: 1022 top-level PASS / 0 FAIL. Side Research md5 and
Rule 18 preserved. Tag `v0.14.0` is created on the release consolidation
commit and pushed with `main`.

**2026-08-10 Cluster G' rev-5 IMPLEMENTED — AWAITING DUAL REVIEW at
`6941d41`.** Linked-worktree effective-index resolution folded and verified:
temporary projections now seed from Git's resolved index path, fail closed,
and preserve staged status/index identity in a real linked worktree. Full
gates pass.

**2026-08-10 Cluster G' rev-4 adjudicated NEEDS REVISION → rev-5
DISPATCHED.** Internal review confirmed staged-owned-path closure, then found
one MEDIUM linked-worktree gap: temporary-index seeding assumed
`<repo>/.git/index`, but `.git` is a file in linked worktrees. Rev-5 resolves
the effective index via Git and adds a linked-worktree regression. External
review is deferred to the fold.

**2026-08-10 Cluster G' rev-4 IMPLEMENTED — AWAITING DUAL REVIEW at
`a5a814e`.** Internal rev-3 HIGH residual folded: canonical reapply
verification now compares complete staged+unstaged+untracked HEAD→worktree
projection via temporary index and literal paths. Staged owned-path drift is
refused with source/status/canonical/index preserved. Full gates pass.

**2026-08-10 Cluster G' rev-3 adjudicated NEEDS REVISION → rev-4
DISPATCHED.** Internal review found one HIGH residual: reapply comparison used
index→worktree `git diff`, so staged extra changes on canonical-owned paths
were invisible and could false-finalize. Rev-4 replaces it with a literal,
temporary-index HEAD→worktree projection covering staged, unstaged and
untracked changes. External rev-3 is deferred to the fold.

**2026-08-10 Cluster G' rev-3 IMPLEMENTED — AWAITING DUAL REVIEW at
`b11f1ee`.** Both external rev-2 HIGH findings folded: mode-only
materialization is warning-aware and cannot false-finalize; reapply comparison
uses only canonical literal touched paths, so unrelated dirty work no longer
blocks either immediate or committed-baseline reapply. Full gates pass.

**2026-08-10 Cluster G' rev-2 adjudicated NEEDS REVISION → rev-3
DISPATCHED.** Internal reviewer APPROVED clean. External full review found
two reproduced HIGH reapply defects: mode-only canonical patches could
false-pass reverse-check warnings and finalize without restoring executable
mode; and whole-tree materialization comparison rejected valid reapply when
unrelated disjoint work was dirty. Rev-3 is restricted to strict warning-aware
materialization checks, canonical-touched-path-scoped comparison, and direct
regressions for both HEAD-baseline shapes.

**2026-08-10 Cluster G' rev-2 IMPLEMENTED — AWAITING DUAL REVIEW at
`a2a11b2`.** Four rev-1 MEDIUM findings folded: canonical patch is now direct
strict reapply authority independent of recipe drift; dependency/generation
gates precede materialized shortcut; path snapshots restore both
file↔directory transitions; amend refuses before request mutation. Targeted
and full gates pass.

**2026-08-10 Cluster G' rev-1 adjudicated NEEDS REVISION → rev-2
DISPATCHED.** Internal confirmation verified both rev-0 findings closed, then
found four new MEDIUM gaps: reapply still executed potentially stale recipe
instead of canonical patch; already-materialized reapply shortcut ran before
dependency/generation gates; touched-path snapshot rejected file↔directory
transitions; and `amend --reset` wrote request.md before the unapplied state
transition refusal. External rev-1 full review is deferred to rev-2 after this
bounded semantic fold.

**2026-08-10 Cluster G' rev-1 IMPLEMENTED — AWAITING DUAL REVIEW at
`7b65d7f`.** Both accepted rev-0 MEDIUM findings are folded: all temporary
Git add/reset paths use literal-pathspec mode with `:(literal)`/wildcard/
bracket deletion regressions, and all six shipped assets plus parity guard
carry the complete applied/active/reconciling/reconciling-shadow source set.
The rejected stale `Copilot-Session` handoff claim is removed. Full gates pass.

**2026-08-10 Cluster G' rev-0 adjudicated NEEDS REVISION → rev-1
DISPATCHED.** Internal formal review found two valid MEDIUM defects: Git
pathspec-magic filenames were not literalized during temporary
intent-to-add/reset cleanup (ADR rows 7/9), and all six shipped lifecycle
diagrams omitted valid `reconciling`/`reconciling-shadow` source states
(rows 34/41/42). A third process finding claimed `Copilot-Session` trailers
were required; adjudication rejects it because current AGENTS.md and Rule 18
require the parseable `Co-authored-by` trailer only. The stale inherited
CURRENT.md context sentence is corrected in rev-1. External rev-0 returned no
usable verdict and will be rerun after the fold.

**2026-08-10 Cluster G' rev-0 IMPLEMENTED — AWAITING DUAL REVIEW at
`1746ebb`.** Five-commit range `9e77617..1746ebb`: dispatch, atomic store
foundation, transactional command/lifecycle integration, SPEC + six-skill
parity, and adversarial safety fold. All 61 ADR-032 rows are covered (60
mechanically, row 3 by source-switch audit + successful build). Three
independent pre-review passes found and closed canonical-patch inversion,
rename/space/Unicode path omission, sibling capture-command corruption,
base-commit drift, and partial-reapply rollback failures. Full
gofmt/vet/test-count=1/build gates pass. No tag until review and wave close.

**2026-08-10 Cluster G' rev-0 DISPATCHED — v0.14.0 `tpatch feature
unapply` implementation.** Single implementer, sequential: the store,
command, lifecycle integrations, assets, and tests overlap on shared files.
Binding baseline: Accepted `docs/prds/PRD-feature-unapply.md` + Accepted
`docs/adrs/ADR-032-feature-unapply-state-boundary.md`. The post-ship external
MEDIUM summary-vs-source note is closed by `1eba8ee`, `75d80f0`, and
`9e77617`. Corrected scope: add real `StateUnapplied`; write D3's fixed
`unapply-session.json` as a separate audit artifact; do NOT add
`UnappliedStatus` or `ErrUnappliedParent`; dependency edge creation onto an
unapplied parent remains allowed while `StateUnapplied` does not satisfy hard
dependency apply gates. All 61 ADR matrix rows are in scope. v0.14.0 is tagged
only after implementation review and wave close.

**2026-08-05 Cluster G planning SHIPPED at `e1a5898` — v0.14.0 candidate paper package APPROVED.** Four review revs (rev-0 → rev-3), convergent close arc terminated three-way clean at rev-3. Deliverables: `PRD-feature-unapply.md` (refreshed 587 → ~950 lines, moved from allowlisted untracked to tracked, Accepted) + `ADR-032-feature-unapply-state-boundary.md` (~1100 lines, new, D1-D8 with 61-row test matrix, Accepted). Rev-3 dual verdict: internal APPROVED clean; external APPROVED clean (no residuals, no notes — clean APPROVED, not APPROVED WITH NOTES). Rev arc: rev-0 BLOCKED (internal 8 HIGH + 2 MEDIUM; external NEEDS REVISION 10 findings dominated by 7/13 fabricated citations) → rev-1 BLOCKED (internal 3 HIGH; external NEEDS REVISION 2 MEDIUM; **9/10 rev-0 external findings closed byte-for-byte, 16/16 anchors verified — citation-fabrication vector fully neutralized**) → rev-2 BLOCKED (internal 1 HIGH + 1 MEDIUM; external APPROVED WITH NOTES 1 LOW + 1 INFO; convergent AC-10c gap + supervisor-verified AC-35 row 43 semantic contradiction with PRD §3.5:271) → rev-3 APPROVED clean (both reviewers). Range `99a1e06..e1a5898`. Key design decisions locked: **composition Alt A** (parallel independent states, mutually exclusive) — resolves ADR-031 D6 data-model composition sub-question; retirement-command sub-question explicitly deferred to future `tpatch retire`. **D6 8-step atomicity** with `os.CreateTemp`+`os.Rename` POSIX-atomic status.json write (Cluster G' pre-req to upgrade `SaveFeatureStatus`). **Impl Note 4** guard placement: first statement of `applyConfirmUpstreamedTransition` (caller), NOT in `saveConfirmUpstreamedStatus` (callee). Wire schema `unapply-session.json` byte-for-byte identical PRD §7.1 vs ADR D3. 39 §15 ACs + 3 §10 atomicity ACs all mapped 1:1. Side Research md5 preserved: `b385fe622db9926f48861105239f113e`. Cluster G' implementation cluster next (v0.14.0 code + tag).

**2026-08-05 Cluster G planning DISPATCHED (docs-only) — v0.14.0 candidate PRD-feature-unapply + ADR-032.** Direct successor to Cluster F ADR-031 D6 deferral. Scope: refresh existing 587-line PRD-feature-unapply.md draft (dated 2026-07-10, predates Cluster F entirely, no cross-refs to shipped `StateRejected`/`RejectionStatus`/Rule 7) + author new `ADR-032-feature-unapply-state-boundary.md` from scratch. Two-deliverable planning cluster mirrors Cluster F planning shape. Composition question the refresh must resolve: how do `unapplied` (proposed) and `rejected` (shipped v0.13.0) compose — coexist as parallel states, mutually exclusive, or hierarchical (`rejected` implies `unapplied` first)? ADR-031 D6 explicitly said "post-implementation reject deferred to future ADR, potentially PRD-feature-unapply." Non-goals: no code, no v0.14.0 tag, no re-opening Cluster F' scope. Same-implementer sonnet-4.6 dispatched; dual review (sol + opus, both high) at every rev.

**2026-08-05 Cluster F' SHIPPED at `70764a3` — v0.13.0 GH #6 first-class `rejected` feature lifecycle state.** Four review revs (rev-0 → rev-3), convergent close pattern. Cluster F' rev-3 dual verdict: internal APPROVED clean, external APPROVED WITH NOTES with 1 INFORMATIONAL only (`F-EXT-Rev3-1`: rev-3's `Unreadable` → `Missing` swap also improves persisted reopen `divergent_reason` for dangling-symlink historical evidence — non-defect, non-blocking, external explicitly deferred). Rev arc: rev-0 BLOCKED (internal 6 findings incl. 1 BLOCKING wire-schema; external APPROVED WITH NOTES 3 findings) → rev-1 APPROVED WITH NOTES (internal 1 MEDIUM residual F-INT-Rev1-1 dangling-symlink; external APPROVED clean) → rev-2 APPROVED WITH NOTES (external 1 LOW F-EXT-Rev2-1 audit-label; internal APPROVED clean) → rev-3 APPROVED (internal clean; external APPROVED WITH NOTES 1 INFORMATIONAL only, non-defect). Range `c6aaeb2..70764a3` (27 commits: 10 rev-0 impl + 8 rev-1 fold + 2 rev-2 fold + 1 rev-3 fold, plus 6 supervisor tracking commits). Test count: 971 top-level PASS / 0 FAIL. Rule 18 trailer verified on all 27 commits. Side Research md5 preserved: `b385fe622db9926f48861105239f113e`.

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

**Cluster H rev-1 — v0.15.0 candidate resource capture planning.**

- **Task ID**: Cluster H rev-1
- **Milestone**: v0.15.0 candidate (planning)
- **Description**: Fold both rev-0 dual-review verdicts into the
  feature-resource PRD and ADR-033 boundary.
- **Status**: In Progress (rev-2 fold)
- **Assigned**: 2026-08-10
- **WAVE_BASE**: `f04dec7`

### Deliverables

- `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`.
- `docs/adrs/ADR-033-resource-capture-boundary.md`.
- Claims-audit anchors against shipped implementation and source papers.
- Exact v1 CLI/schema/adapter/error/determinism/privacy contracts.
- 1:1 acceptance/test matrix sufficient for a future Cluster H'
  implementation dispatch.

### Required decisions

- Extend `claims.json` vs. introduce a separate resource manifest.
- Resource kind/selector/ID schema and deterministic ordering.
- Snapshot/diff artifact envelope and canonical-authority boundary.
- Explicit ignored-file opt-in, privacy, redaction and secret refusal.
- Logical Git metadata allowlist; raw `.git/**` remains impossible.
- Optional adapter capability/protocol, command execution and version capture.
- Dolt scope (schema/table diff exports only; no authoritative Dolt store).
- Auto-record integration, failure semantics and partial-adapter policy.
- Resource generation/amend/remove lifecycle and feature metadata diff UX.

### Constraints

- Reuse shipped file claims, capture modes, capture provenance and generation
  identity rather than creating a parallel ownership model.
- Honor ADR-027 privacy and ADR-030 `.git/**` store-boundary refusal.
- Keep Git-first assumptions from WP-006; non-Git replay remains out of scope.
- Planning only; no CLI/schema implementation.
- Preserve Side Research md5 `b385fe622db9926f48861105239f113e`.
- Do not stage allowlisted untracked research files.
- Stage explicit file paths only; Rule 18 trailer required on every commit.

## Session Summary

- **Cluster H rev-0** — dispatched 2026-08-10 from `WAVE_BASE=f04dec7`;
  written 2026-08-10. `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md`
  (1086 lines) and `docs/adrs/ADR-033-resource-capture-boundary.md`
  (963 lines) authored from scratch, both Claims Audits verified against
  current source (not stale line references). New `resources.json` manifest
  (separate from `claims.json`, D1); `res_<12hex>` deterministic ID scheme
  (D3); three closed v1 resource kinds and two closed adapter names (D6/D7,
  Dolt scoped strictly to `schema-diff`/`table-diff`, no core dependency, no
  authority); logical Git metadata restricted to a 4-view allowlist with a
  `config` key allowlist, raw `.git/**` refused at both diff and store-write
  boundaries (D5); `ignored-file` resources gated on `gitutil.IsPathIgnored`
  plus ADR-027 D3 redaction-as-precondition (D4); argv-only, no-shell,
  env-name-only adapter execution protocol with timeout/output caps (D6);
  resource diffs are pure sidecar audit artifacts with zero lifecycle
  coupling — no command in `internal/workflow`/`internal/cli` gates on them
  (D2); `record --resources` is an all-or-nothing batch with independent
  Git-side capture and no `patch-generations.json` write (D8); snapshot/diff
  wire envelope mirrors the existing `post-apply.patch`/`patches/NNN-*.patch`
  canonical-vs-audit pattern with non-null empty arrays (D9); no coupling to
  patch generations and clean, non-recoverable local delete on
  `remove`/`clear`, mirroring `RemoveClaim` (D10). PRD has 30 numbered
  acceptance criteria; ADR has a 45-row test matrix mapping every AC 1:1
  plus 15 supporting safety rows. Both new-document JSON wire examples
  verified byte-identical between PRD and ADR. Ready for dual review; see
  "Ready for review — Cluster H rev-0" below.
- **Cluster H rev-1** — dispatched 2026-08-10 (adjudication `89c8d79`) from
  `WAVE_BASE=f04dec7`; written 2026-08-10. Both rev-0 dual-review verdicts
  (internal 7 HIGH + 1 MEDIUM, external 3 HIGH + 8 MEDIUM + 3 LOW) folded in
  full across both papers. `PRD-feature-resource-claims-and-capture-adapters.md`
  rewritten to 1396 lines; `ADR-033-resource-capture-boundary.md` rewritten
  to 664 lines (corrected from the rev-0 handoff's stale "963" claim — actual
  rev-0 file was 992 lines). Preserved: separate `resources.json`, no
  canonical-patch/lifecycle authority, Dolt never authoritative/a core
  dependency, Git-only replay. Rewritten: raw snapshot/diff/file bytes now
  live only under a new gitignored `.tpatch/local/resource-capture/<slug>/`
  lane (immutable `lb_<12hex>` batches + one atomic `current`-pointer
  rename, D8) — tracked sidecars are hash/count/classification-only, never
  verbatim (D1); `generic-command` removed entirely, Dolt is the sole v1
  adapter (D2); a mandatory Lstat+`EvalSymlinks`+containment+`.git`-target-refusal
  gate runs at both `add` and every `capture` for every path touched (D6);
  `ignored-file` requires both `gitutil.IsPathIgnored` **and**
  confirmed-untracked, closing the `--no-index` gap, plus exact 5 MiB/20
  MiB/200-file directory limits re-checked at capture time (D4); `git-metadata`
  narrowed from an unrestricted allowlist to 4 closed views with exactly 4
  non-PII `config` keys, no more `user.name`/`user.email`/wildcards (D5);
  the Dolt protocol uses only DoltHub-verified flags (`--name-only
  --filter=`), never a fabricated `--json`/per-row schema (D7); `resource_id`
  uses a new sorted, minimally-escaped canonical-JSON `args` encoding with 4
  independently-recomputed golden vectors, including a reordered-key
  equivalence proof (D3); a new exported, content-agnostic six-class
  hard-refusal redaction scanner is required of Cluster H' — PEM/OpenSSH
  keys, DB/connection URLs, emails/PII, credential assignments, bearer/token
  patterns, home paths (D9); `record --resources` now stages privately
  before Git-side capture and publishes the resource pointer only after Git
  succeeds, with an honest partial-domain error, exact recovery command, and
  a new per-command exit-code table (D10). Corrected six false/fabricated
  rev-0 citations (fabricated `ADR-031 D10`; false "`EnsureSafeRepoPath`/
  `NormalizeClaimPath` are symlink-aware"; false "`verify` is the sole
  `ExitCodeError` user" — six commands actually use it, matching `SPEC.md`'s
  own "per-command contracts, not a global enum" language; normative
  reliance on untracked `WP-006`, replaced with the tracked
  `storage-substrate-and-versioned-data.md` research; a stale `RemoveClaim`
  line range; and the rev-0 handoff's own stale ADR line count). Rebuilt the
  acceptance-criteria list at clause granularity: 41 `AC`-tagged clauses,
  mechanically verified present in the PRD; the ADR's Test Matrix has 66
  mechanically-counted rows covering all 41 clauses (not claimed "exactly
  once" anywhere — several clauses map to 2+ rows for human/JSON pairs or
  multi-outcome tables). All 4 shared JSON wire examples (`resources.json`
  + 3 per-kind `summary.json` examples) verified programmatically
  byte-identical between the two documents. `git diff --check` clean on
  both files. Ready for dual review; see "Ready for review — Cluster H
  rev-1" below.
- **Cluster G' rev-0** — dispatched 2026-08-10 from `WAVE_BASE=9e77617`.
  Store foundation complete: `StateUnapplied` is the twelfth valid state and
  `SaveFeatureStatus` now uses a same-directory temp file, fsync, and atomic
  rename that preserves the prior status bytes on rename failure. Core command
  and lifecycle integration complete: transactional reverse-unapply, D3 audit
  envelope, rollback seams, apply/reconcile/status/next/land/dependency and
  reject/reopen/confirm-upstreamed behavior are wired. SPEC, dependency docs,
  all six shipped skill surfaces, and parity anchors are updated. Three
  adversarial pre-review passes found and closed canonical-patch data-loss,
  rename/space/Unicode path, sibling capture-command, and partial-reapply
  rollback failures. Matrix audit: 60 rows mechanically covered + row 3
  source-switch audit. Repository-wide gates pass. Implementation tip
  `1746ebb`; awaiting dual review.
- **Cluster G' rev-1** — internal rev-0 findings folded: every temporary
  add/reset path now runs under Git literal-pathspec mode; adversarial
  deletion tests cover `:(literal)`, `*`, and `[]`; all six asset diagrams
  and parity anchors carry the complete four-state source set; stale
  `Copilot-Session` guidance removed. Full gates pass.
- **Cluster G' rev-2** — internal rev-1 semantic findings folded: reapply
  strictly uses canonical post-apply.patch (recipe-independent), shortcut
  finalization follows dependency/generation gates, touched-path rollback
  supports file↔directory transitions, and amend refuses before request
  mutation. Full gates pass.
- **Cluster G' rev-3** — external rev-2 HIGH findings folded: reverse
  materialization checks now reject success-with-warning mode mismatches;
  reapply comparison is scoped to canonical literal touched paths so
  unrelated disjoint work is preserved. Immediate and committed mode-only/
  unrelated-dirt regressions pass with full gates.
- **Cluster G' rev-4** — internal rev-3 HIGH finding folded: reapply
  comparison now uses literal temporary-index HEAD→worktree projection,
  including staged, unstaged and untracked canonical-owned changes while
  preserving user index. Staged owned-path drift regression and full gates
  pass.
- **Cluster G' rev-5** — internal rev-4 MEDIUM finding folded: complete
  projection resolves Git's effective index with `rev-parse --git-path
  index`, so linked worktrees and GIT_INDEX_FILE layouts are supported.
  Linked-worktree staged-diff/index-identity regression and full gates pass.
- **Cluster G' SHIPPED** — rev-5 internal + external APPROVED clean; v0.14.0
  release consolidation authorized with 1022 top-level tests passing.
- **Post-release external review** — APPROVED WITH NOTES; three disclosure/
  naming/prose notes folded after the tag without changing release code or
  moving v0.14.0.
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
- **Cluster F' rev-0** (v0.13.0 GH #6 implementation — first-class `rejected` feature lifecycle state) — implemented 2026-08-06, reviewed, adjudicated NEEDS REVISION. 10 commits, range `8cf3c1a..d3e5a11`.
- **Cluster F' rev-1** (7-finding fold from the rev-0 dual review) — implemented 2026-08-06, reviewed, external APPROVED clean, internal APPROVED WITH NOTES (1 MEDIUM residual). 8 commits, range `d3e5a11..fbdf815`.
- **Cluster F' rev-2** (F-INT-Rev1-1 MEDIUM: dangling-symlink guard in `resolveEvidence` fallback) — implemented 2026-08-05. 1 commit, range `fbdf815..1492fb0`. See "Ready for review — Cluster F' rev-2" below.
- **Cluster G planning** (docs-only; PRD-feature-unapply.md refresh + ADR-032-feature-unapply-state-boundary.md from scratch; v0.14.0 candidate) — implemented 2026-08-05, dispatched for dual review. See "Ready for review — Cluster G rev-0" below.

## Files Changed — Cluster H rev-0

- `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md` — new,
  1086 lines. Full v1 design: Claims Audit (20 rows), Summary, Problem
  Statement with existing-primitives preflight (§1.1), Goals/Non-goals,
  Command Surface with candidate-command evaluation table (§3), Persisted
  Manifest schema (§4), Resource Kinds and Selector Normalization (§5),
  Adapter Capability/Execution Protocol (§6), Snapshot/Diff Artifact Layout
  and Wire Envelope (§7), Privacy/Redaction/Safety (§8), Auto-record
  Integration/Transaction/Failure Semantics (§9), Interactions (§10),
  Backward Compatibility (§11), Explicit Deferrals (§12), Implementation
  Notes for Cluster H' (§13), 30 numbered Acceptance Criteria (§14), 4 Open
  Questions each with a locked v1 default (§15), Disputes (§16, none).
- `docs/adrs/ADR-033-resource-capture-boundary.md` — new, 963 lines. 10
  binding decisions (D1–D10): manifest placement (new `resources.json`, not
  an extension of `claims.json`), authority boundary (sidecar-only, zero
  lifecycle coupling), resource ID/selector normalization (`res_<12hex>`),
  ignored-file/privacy boundary (`gitutil.IsPathIgnored` gate + ADR-027 D3),
  logical Git metadata allowlist (4 closed views), adapter discovery/
  execution/sandbox contract (argv-only, no shell, env-name-only), Dolt's v1
  scope (one closed adapter, never authoritative), `record --resources`
  transaction/failure policy (all-or-nothing batch), snapshot/diff wire
  schema (fixed envelope, non-null empty arrays), and generation/amend/
  remove lifecycle/backward compatibility (no `patch-generations.json`
  coupling, clean delete on remove). Includes byte-identical JSON wire
  examples shared with the PRD, an Implementation Notes section for
  Cluster H' (8 items), a Negative Consequences Summary (one row per
  decision), and a 45-row Test Matrix mapping every PRD §14 AC (1–30) 1:1
  plus 15 supporting safety rows.
- `docs/handoff/CURRENT.md` — this update: Status narrative, Active Task
  Status → Review, Session Summary, this Files Changed section, Test
  Results, Next Steps, and the "Ready for review — Cluster H rev-0" section
  below. Cluster state flipped to `AWAITING REVIEW`.

No other files touched. `docs/ROADMAP.md`, `docs/supervisor/LOG.md`, `SPEC.md`,
`CHANGELOG.md`, any existing PRD/ADR, and all code/assets remain untouched by
this cluster.

## Test Results — Cluster H rev-0

Docs-only cluster; no Go build/test/fmt required or run since no `internal/`,
`cmd/`, or `assets/` files were touched. Validation performed instead:

- `git diff --check` on both new files — clean (no whitespace errors, no
  conflict markers).
- Citation verification: every `file:line` anchor cited in both the PRD
  Claims Audit and the ADR Claims Audit/decision sections was independently
  re-read against current source (not trusted from stale line references).
  Two incorrect citations found during self-review were corrected before
  finalizing: `internal/store/claims.go:436-444` → `:434-444` (`RemoveClaim`
  exact line range), and a fabricated `ADR-031 D10` citation (that D10 is
  actually about the `reject` verb naming disposition, not feature-noun
  scoping) replaced with a direct quote from `featureCmd`'s own doc comment
  at `internal/cli/feature_deps.go:39-51`.
- Wire-example parity check: extracted every fenced ` ```json ` block from
  both new documents programmatically and confirmed the ADR's `resources.json`
  example (D1) and diff-envelope example (D9) are byte-identical to the
  PRD's §4 and §7 examples respectively.
- AC/test-matrix mapping check: confirmed all 30 PRD §14 acceptance-criterion
  numbers (AC-1..AC-30) appear exactly once each in the ADR's Test Matrix
  `PRD AC` column, with 15 additional untagged supporting-safety rows, for a
  total of 45 mechanically-countable rows.
- Decision count check: confirmed exactly 10 `## D<N>:` decision headers
  (D1–D10) in the ADR, exceeding the required minimum of 8.
- `git status --short` confirmed only the two new files are added by this
  writer; all pre-existing untracked WIP (`PRD-recurring-patches.md`,
  WP-004/005/006/007 drafts and turns files, case-studies directory) remains
  untouched and unstaged.
- Side Research md5 invariant re-verified unchanged after this edit (see
  "Context for Next Agent" below).

## Files Changed — Cluster H rev-1

- `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md` —
  rewritten in full, 1396 lines (was 1086). New structure: §0 Rev-1 Fold
  Summary (Claims Audit — 10 rows, `C1`–`C10`, each independently
  re-verified against current source this session; What rev-1 removes;
  golden ID vectors; requirement-letter → section map), §1 Problem
  Statement, §2 Goals/Non-Goals, §3 Command Surface (`capture`/`diff` split
  as two distinct verbs), §4 Data Model (`resources.json` + per-resource
  `summary.json`, missing-local semantics), §5 Resource Kinds
  (`ignored-file` dual-gate + limits, `git-metadata` 4-view allowlist,
  `adapter-snapshot` Dolt-only), §6 Adapter Protocol (verified Dolt argv,
  probe outcomes, timeouts/caps/env), §7 Local Storage & Transaction Design
  (immutable batches, atomic pointer, crash-window table, history model),
  §8 Privacy & Redaction (six closed classes, hard refusal), §9 Symlink &
  Path Safety (four-step gate + outcome table), §10 `record --resources`
  Semantics (stage-then-publish, partial-domain error, exit codes), §11
  Wire Schemas (one JSON example per kind), §12 Resource ID
  Canonicalization (algorithm + 4 golden vectors), §13 Acceptance Criteria
  (41 `AC`-tagged clauses + exact-count reasoning), §14 Open
  Questions/Negative Consequences, §15 Rev-1 Changelog.
- `docs/adrs/ADR-033-resource-capture-boundary.md` — rewritten in full,
  664 lines (was 992 — the rev-0 handoff's "963" claim was itself stale;
  the actual rev-0 file was 992 lines). 10 binding decisions (D1–D10):
  scope/authority reaffirmation (D1), closed 3-kind set with
  `generic-command` removed (D2), resource-ID canonicalization + golden
  vectors (D3), `ignored-file` dual-gate + limits (D4), `git-metadata`
  4-view allowlist (D5), symlink/path safety gate (D6), verified Dolt
  protocol (D7), local storage/transaction model (D8), six-class
  hard-refusal redaction (D9), `record --resources` stage-then-publish
  semantics + exit codes (D10). Wire Schema Appendix with byte-identical
  JSON examples shared with the PRD, Implementation Notes for Cluster H'
  (5 items), Negative Consequences Summary, and a 66-row Test Matrix
  covering all 41 PRD acceptance-criteria clauses.
- `docs/handoff/CURRENT.md` — this update: Status narrative, Active Task
  Status → Review, Session Summary, this Files Changed section, Test
  Results, and the "Ready for review — Cluster H rev-1" section below.
  Cluster state flipped to `AWAITING REVIEW`.

No other files touched. `docs/ROADMAP.md`, `docs/supervisor/LOG.md`,
`SPEC.md`, `CHANGELOG.md`, any existing PRD/ADR, and all code/assets remain
untouched by this cluster — confirmed via `git status --short` showing only
the three owned paths as modified/deleted-then-recreated.

## Test Results — Cluster H rev-1

Docs-only cluster; no Go build/test/fmt required or run since no `internal/`,
`cmd/`, or `assets/` files were touched. Validation performed instead:

- `git diff --check` on both rewritten files — clean (no whitespace errors,
  no conflict markers).
- Citation re-verification: every `file:line` anchor cited in the PRD's §0.1
  Claims Audit (`C1`–`C10`) was independently re-read against current
  source this session, including `internal/cli/feature_deps.go:38-51`
  (`featureCmd` doc comment, confirmed the correct citation replacing rev-0's
  fabricated `ADR-031 D10`), `internal/safety/safety.go` (confirmed
  `EnsureSafeRepoPath` is lexical-only, no `Lstat`/`EvalSymlinks`),
  `internal/gitutil/ignore.go` (confirmed `IsPathIgnored` uses `git
  check-ignore -q --no-index`), `internal/cli/session_redaction.go`
  (confirmed unexported/observation-shaped/drop-line), `internal/cli/exit_error.go`
  plus a repo-wide grep confirming six (not one) `&ExitCodeError{` call
  sites, `internal/store/claims.go:436-444` (`RemoveClaim`, confirmed exact
  range), and `SPEC.md`'s exact "Exit codes are per-command contracts, not a
  single global enum ... `tpatch verify` has its own, unrelated exit-2
  meaning" quote (verified verbatim at `SPEC.md:137-138`).
- Dolt CLI syntax re-verified against the DoltHub CLI reference
  (`https://www.dolthub.com/docs/cli-reference/cli/`): confirmed
  `--name-only`, `--filter={added,modified,renamed,dropped}`,
  `--schema`/`--data`, and `-r`/`--result-format {tabular,sql,json}` all
  exist as documented; confirmed no bare `--json` flag exists; confirmed
  `dolt version`'s single-line output shape.
- Golden-vector recomputation: all four `resource_id` vectors (§12.3 in the
  PRD, D3 in the ADR) independently recomputed via Python `hashlib.sha256`
  against the exact canonical-JSON/payload construction described in both
  documents; all four match; Vector 3 (reordered `args`) confirmed
  byte-identical to Vector 2, proving order-independence.
- Wire-example parity check: programmatically extracted every fenced
  ` ```json ` block from both rewritten documents and confirmed all four
  shared examples (`resources.json`, and the `adapter-snapshot`/
  `git-metadata`/`ignored-file` `summary.json` examples) are byte-identical
  between the PRD and the ADR (Python `str ==` comparison on the raw fenced
  block text, not just semantic/sorted-key equality).
- AC/test-matrix mapping check: mechanically extracted every `AC-<n>` tag
  from the PRD (41 distinct, `AC-1`..`AC-41`, including two multi-item
  ranges `AC-16`–`AC-21` and `AC-25`–`AC-27` that expand to individually
  countable clauses) and every numbered row of the ADR's Test Matrix
  section specifically (66 rows, `1`..`66`, isolated from the document's
  other numbered tables — golden-vector and view tables — which also use a
  leading-integer column but are not part of the Test Matrix); confirmed
  all 41 clauses appear in at least one matrix row and no row count is
  claimed "exactly once" anywhere in either document.
- `git status --short` confirmed only the three owned paths are
  touched by this writer; all pre-existing untracked WIP
  (`PRD-recurring-patches.md`, WP-004/005/006/007 drafts and turns files,
  case-studies directory) remains untouched and unstaged.
- Side Research md5 invariant re-verified unchanged after this edit (see
  "Context for Next Agent" below).
- **Addendum (supervisor source check)**: supervisor independently verified
  the primary `dolthub/dolt` source at commit
  `59fb843bf6a4b653d7c8b6d997a603b10cf279d9` — `go/cmd/dolt/commands/diff.go`
  (synopsis `dolt diff [options] <commit> <commit> [tables...]`,
  `--schema`/`--data` selection, `-r`/`--result-format`
  `tabular`/`sql`/`json`) and `go/cmd/dolt/commands/version.go`
  (`dolt version [--verbose] [--feature]`, prints `dolt version
  <version>`) — and flagged that `dolt status --porcelain` was not found
  in source. Both owned papers already avoided that flag (the PRD's
  §5.1 "porcelain check" wording referred generically to `git ls-files
  --error-unmatch`, not any Dolt flag; tightened anyway to remove the
  ambiguity). Updated PRD C9/§6/§6.3 and ADR Decision Drivers/D7 to cite
  the primary source commit directly (previously cited the DoltHub docs
  page only), and to state precisely which facts (`diff` synopsis,
  `--schema`/`--data`, `-r`/`--result-format` values, `version` output)
  are source-verified versus docs-cross-checked (`--filter`,
  `--name-only`) rather than overclaiming full-source verification.
  No design/decision changed; `git diff --check` re-run clean; AC/matrix
  coverage (41 clauses, 66 rows) and all 4 wire-example byte-identity
  checks re-run and unaffected.

## Files Changed — Cluster G' rev-0

Foundation:
- `internal/store/types.go` — `StateUnapplied` + closed-switch validation.
- `internal/store/store.go` — atomic JSON/file writer used by
  `SaveFeatureStatus`; distinct Unapplied FEATURES.md section.
- `internal/store/unapply_test.go` — state wire-value guard and failed-rename
  preservation/cleanup regression.

Core command and Git transaction:
- `internal/gitutil/unapply.go` — strict reverse check/apply preview,
  mid-Git-operation detection, safe touched-path snapshot/restore.
- `internal/gitutil/unapply_test.go` — reverse preview/apply/restore, mode and
  traversal coverage.
- `internal/cli/feature_unapply.go` — noun-scoped command, dry-run report,
  exit envelope, D3 fixed schema, D6 transaction/rollback.
- `internal/cli/feature_unapply_test.go` — core AC matrix, source states,
  dependents, failure rollback, wire order, help, apply/reconcile/status/next/
  land and rejected-state interactions.

Lifecycle integrations:
- `internal/cli/{feature_deps.go,cobra.go,phase2.go,land.go,reject.go,reopen.go}`
  — registration, help cross-reference, next, land, confirm-upstreamed and
  reject/reopen guards.
- `internal/workflow/{dependency_gate.go,labels.go,reconcile.go,doctor_d5.go}`
  — active/unapplied dependency semantics, waiting label, explicit viability
  reconcile without state mutation, doctor coverage.
- `internal/store/rejection_test.go` — unapplied remains reject-ineligible.

Adversarial pre-review folds:
- `internal/gitutil/{gitutil.go,capture_modes.go,unapply.go}` — NUL-delimited
  untracked capture with `core.quotePath=false`, both-side rename/copy parser,
  unquoted spaces/Unicode and literal pathspec handling, fail-closed HEAD check.
- `internal/cli/{cobra.go,phase2.go,feature_patch.go,c1.go}` — conditional
  record/patch capture guards, unapplied-only lifecycle boundary, canonical
  reapply preservation, original base-commit retention, and transactional
  partial-reapply rollback.
- `internal/workflow/verify.go` — explicit unapplied verify omission.
- `internal/cli/feature_unapply_test.go`,
  `internal/gitutil/unapply_test.go`, `internal/store/unapply_test.go` —
  regressions for every pre-review finding (rename, spaces, Unicode,
  capture-command inversion, incomplete recipe, state drift, base commit).

Contract documentation and assets:
- `SPEC.md` — unapplied state, command, dependency, transaction and interaction
  contract.
- `docs/dependencies.md` — legal edge creation vs hard-gate dissatisfaction.
- `assets/assets_test.go` — command and three unapply parity anchors.
- All six shipped skill/prompt/workflow assets — phase arc, command guidance,
  dependents and corrected edge semantics.
- `docs/handoff/CURRENT.md` — implementation progress and targeted results.

## Test Results — Cluster G' rev-0

- `go test ./internal/store` — PASS.
- `go test ./internal/gitutil` — PASS.
- `go test ./internal/cli -run 'TestFeatureUnapply|TestFeatureApplyReapplies|TestUnappliedParent|TestActiveParent|TestDependencyEdgeOntoUnappliedParent|TestExplicitReconcileOnUnapplied|TestAggregateReconcileSkips' -count=1` — PASS.
- `go test ./internal/gitutil ./internal/store ./internal/workflow ./internal/cli` — PASS.
- `go test ./assets` — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.
- ADR-032 matrix audit — 60 rows mechanically covered; row 3 manually audited
  across state-aware source switches and covered by successful build.

## Files Changed — Cluster G' rev-1

- `internal/gitutil/{gitutil.go,capture_modes.go}` — literal-pathspec mode for
  all temporary intent-to-add/reset operations.
- `internal/cli/feature_unapply_test.go` — pathspec-magic deletion coverage.
- `assets/assets_test.go` — four-state source-set parity anchor.
- All six shipped skill/prompt/workflow assets — complete
  applied/active/reconciling/reconciling-shadow unapply arc.
- `docs/handoff/CURRENT.md` — rev-1 progress and corrected Rule 18 guidance.

## Test Results — Cluster G' rev-1

- Pathspec-magic + Unicode targeted tests — PASS.
- `go test ./assets` — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.

## Files Changed — Cluster G' rev-2

- `internal/cli/cobra.go` — canonical strict reapply path, dependency-first
  shortcut ordering and retained base_commit.
- `internal/gitutil/unapply.go` — strict forward apply plus directory-aware
  two-phase snapshot restore.
- `internal/cli/c1.go` — unapplied amend refusal before request writes.
- `internal/cli/feature_unapply_test.go` — stale-recipe authority,
  materialized-gate, amend identity and file→directory command regressions.
- `internal/gitutil/unapply_test.go` — both file↔directory restore directions.
- `docs/handoff/CURRENT.md` — rev-2 progress and gates.

## Test Results — Cluster G' rev-2

- Rev-2 targeted semantic tests — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.

## Files Changed — Cluster G' rev-3

- `internal/gitutil/gitutil.go` — warning-aware strict reverse validation.
- `internal/gitutil/unapply.go` — warning-aware HEAD materialization check.
- `internal/cli/{feature_unapply.go,cobra.go}` — shared literal pathspecs and
  canonical-touched-path-scoped reapply comparison with mode-only fallback.
- `internal/cli/feature_unapply_test.go` — mode-only immediate/committed and
  unrelated-dirt reapply regressions.
- `docs/handoff/CURRENT.md` — rev-3 progress and gates.

## Test Results — Cluster G' rev-3

- Rev-3 targeted mode-only/unrelated-dirt tests — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.

## Files Changed — Cluster G' rev-4

- `internal/gitutil/gitutil.go` — literal temporary-index
  HEAD→worktree projection.
- `internal/cli/cobra.go` — complete canonical-path reapply comparison.
- `internal/cli/feature_unapply_test.go` — staged owned-path drift/index
  preservation regression.
- `docs/handoff/CURRENT.md` — rev-4 progress and gates.

## Test Results — Cluster G' rev-4

- Rev-4 staged-owned-path targeted test — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.

## Files Changed — Cluster G' rev-5

- `internal/gitutil/gitutil.go` — effective-index resolution for temporary
  projection.
- `internal/gitutil/unapply_test.go` — linked-worktree staged diff and user
  index identity regression.
- `docs/handoff/CURRENT.md` — rev-5 progress and gates.

## Test Results — Cluster G' rev-5

- Linked-worktree targeted test — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.

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

## Files Changed — Cluster F' rev-1

Source (modified):
- `internal/store/status.go` — `RejectionHistoryEntry` rewritten as a completed-cycle record (reject half + reopen half); `History` removed from `RejectionStatus`; `RejectionActionReject`/`RejectionActionReopen` constants deleted. **[F-INT-1]**
- `internal/store/types.go` — `FeatureStatus.RejectionHistory []RejectionHistoryEntry` (`rejection_history`, `omitempty`) added at top level; `Rejection` doc comment corrected to "live record, cleared on reopen". **[F-INT-1]**
- `internal/store/validation.go` — `ErrRejectedParent` sentinel text becomes `"cannot add dependency"` so `%w` renders PRD §8's golden string verbatim; Rule 7 message reworded to match §8. **[F-INT-4/F-EXT-1]**
- `internal/cli/reject.go` — reject no longer appends history; `rejectionStatusView`/`evidenceRefView` §8 DTO + `newRejectionStatusView`; evidence validation moved ahead of the state-machine checks; `resolveEvidence` falls through only on `os.IsNotExist`; `joinStatesOr` added and used in the wrong-state refusal. **[F-INT-1, F-INT-2, F-INT-3, F-INT-5, F-EXT-2]**
- `internal/cli/reopen.go` — reopen folds the live `Rejection` into one completed-cycle history entry, appends it to `FeatureStatus.RejectionHistory`, then sets `Rejection = nil`; new-evidence validation moved ahead of the state check; `--help` reworded to describe the fold. **[F-INT-1, F-INT-3]**
- `internal/cli/cobra.go` — `featureWithFreshness` gains a depth-0 `Rejection *rejectionStatusView` field that shadows the embedded `FeatureStatus.Rejection` in `encoding/json`; status detail renders "Rejection history: N completed cycle(s)"; `pluralEntries` → `pluralCycles`. **[F-INT-1, F-INT-2]**
- `internal/cli/feature_deps.go` — `mapDependencyValidationError` maps `ErrRejectedParent` to exit 3, applied at both `ValidateDependencies` call sites and re-applied at the `amend --depends-on` boundary. **[F-INT-4/F-EXT-1]**

Tests (modified):
- `internal/store/rejection_test.go` — round-trip split; new `TestRejectionHistoryEntry_CompletedCycleRoundTrip` (asserts all 13 wire keys) and `TestFeatureStatus_RejectionHistoryOmittedWhenEmpty`.
- `internal/cli/reject_test.go` — history assertions rewritten to the per-cycle invariant; 8 new tests: `TestRejectReopen_TwoCyclesYieldTwoHistoryEntries`, `TestStatusJSON_RejectionDTOMatchesSpecFieldNames`, `TestStatusJSON_NoRejectionObjectAfterReopen`, `TestReject_EvidenceValidationPrecedesStateCheck`, `TestReopen_EvidenceValidationPrecedesStateCheck`, `TestReject_EvidenceFallbackOnlyOnGenuineNotFound`, `TestReject_WrongStateErrorMatchesGoldenString`, `TestJoinStatesOr`. Test 27 rewritten against independent literals.

No docs/assets changed in rev-1: no CLI flag, command name, reason code or state name changed, so the shipped skill surfaces and `SPEC.md` remain accurate and the parity guard is untouched.

## Test Results — Cluster F' rev-1

- `gofmt -l .` empty; `go vet ./...` clean; `go build ./cmd/tpatch` OK.
- `go test -count=1 ./...` — **970 top-level PASS / 0 FAIL** across all 9 test packages (up from 960 at rev-0; +10 top-level from the rev-1 regression tests).
- Side Research md5 preserved: `b385fe622db9926f48861105239f113e` (verified post-fold).

### `make wave-close-check WAVE_BASE=c6aaeb2`

| Step | Result |
|------|--------|
| `[1/8]` working tree clean | **OK** |
| `[2/8]` untracked-source sentinel | **OK** (16 entries allowlisted; no WIP file staged) |
| `[3/8]` HEAD pushed to `origin/main` | **OK after the rev-1 push** (FAILs pre-push by construction) |
| `[4/8]` Rule 18 trailer on every wave commit | **OK** (21 commits in `c6aaeb2..HEAD` at the pre-docs-commit run) |
| `[5/8]` canonical `**Cluster state**:` terminal | **FAIL by design** — reads `REV-1 DISPATCHED`. Mid-cycle implementation handoff awaiting review; the dispatch brief forbids the implementer touching the canonical field. **The supervisor flips it at wave close.** |
| `[6/8]` gofmt clean | **OK** |
| `[7/8]` go vet + go build clean | **OK** |
| `[8/8]` `go test -count=1 ./...` clean | **OK** |

Manual items remain for the supervisor: LOG entry, ROADMAP flip, HISTORY archive, tag.

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

1. Replace invalid Dolt argv with verified deterministic SQL summary capture.
2. Remove persistent raw bodies and align fully with ADR-027 D1-D6.
3. Lock single-pointer publication, path/executable, stale-lock, local-ignore,
   literal-pathspec and failure-directory semantics.
4. Complete tagged wire variants, exit codes and new matrix rows.
5. Re-run internal and external rev-2 review with exact count audits.
6. Reuse the same writer context for further bounded folds if needed.
7. On dual APPROVED (or APPROVED WITH NOTES): accept papers, archive
   Cluster H to `docs/handoff/HISTORY.md`, flip ROADMAP's Cluster H status,
   and dispatch implementation as a separate Cluster H' — no code changes
   belong in this cluster's history.

## Registered Candidate — Typed Feature Resources and Capture Adapters

**Status**: Cluster H planning active; rev-2 dispatched.

Existing shipped primitives already cover normal repository files:
`feature claim add|list|remove|clear`, record
`--all|--staged|--unstaged|--claimed-only|--files`, and non-ignored untracked
files. The gap is typed resources outside ordinary Git diff authority:
explicit gitignored files, logical Git metadata, Dolt schema/table diffs, and
optional deterministic adapter exports.

The candidate must provide add/remove/list/diff/capture metadata and define
how `record --resources` consumes explicit resource claims without ever
becoming canonical-patch/lifecycle authority. It must not capture raw
`.git/**`, sweep ignored files implicitly, make Dolt authoritative or a core
dependency, or replace Git as the supported change-tracking substrate.
Resource capture results remain sidecar audit artifacts in v1, and (as of
rev-1) raw content of any kind is local-only and opt-in — tracked artifacts
are hash/count/classification-only. See ROADMAP registered-candidate section
for the CLI sketch, dependencies and safety boundaries.

## Blockers

None.

## Context for Next Agent

- **Cluster H rev-1 papers are rewritten and awaiting dual review at
  `WAVE_BASE=f04dec7`.** No code or release tag belongs in this cluster;
  implementation is a separately dispatched Cluster H' only after both
  papers reach Accepted status. See "Ready for review — Cluster H rev-1"
  below for reviewer focus areas.
- **Both rev-0 verdicts were NEEDS REVISION** — rev-1 is a substantial
  rewrite, not a patch (see the Session Summary entry above for the full
  fold list). If rev-1 also comes back NEEDS REVISION, re-read this
  handoff's Session Summary/Files Changed/Test Results entries for rev-1
  before re-diving into the papers — the design rationale for every
  changed decision is recorded there, not just in the papers themselves.
- **v0.12.1 SHIPPED** — do NOT re-open Wave α/β/γ or GH #3/#4/#5 scope. All accepted.
- **Two-opinion protocol proven load-bearing again** — v0.12.1 rev-0 external caught 4 findings internal missed (PRD-#4 warning wording, PRD-#4 tie-break correctness bug, PRD-#3 err-branch gap, GH #5 hint mislabel). Internal caught PRD-#3 F-INT-3-1 HIGH (Rule 18 trailer parse failure). Continue dual-review protocol on all clusters ≥ paper-only.
- **Cross-implementer entanglement is now a KNOWN failure mode** — do NOT dispatch parallel implementers to shared source files without briefing them on `git add <path>` discipline. See Cluster A follow-up in backlog.
- **20 binding carry-forward rules** unchanged. Rule 18 empirical demonstration this cluster: heredoc-authored commit bodies leaked `EOF)` after the trailer, breaking `%(trailers)` parse. Rule 20 empirical demonstration: PRD-#4 external caught the tie-break bug via code path enumeration (in-place dedup) that internal's tests-pass verdict didn't surface. Rule 20 continues to require empirical repro even on paper-approved designs.
- **Side Research md5 invariant**: `b385fe622db9926f48861105239f113e`. Verify: `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)`.
- **Rule 18 commit trailer**: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` verbatim. Use `git commit -F <tempfile>` or `git commit -m ""` — NOT `git commit -F -` with heredoc (heredoc close tokens leak into the body). `Copilot-Session` is historical metadata, not a current Rule 18 requirement.

## Ready for review — Cluster H rev-1

**Scope**: docs-only planning cluster, rev-1 fold. Same two deliverables as
rev-0, both fully rewritten (not patched), plus this handoff update — no
code:

1. `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md` — 1396
   lines (was 1086). See "Files Changed — Cluster H rev-1" above for the
   full section-by-section breakdown; see the Session Summary entry above
   for the complete list of changed decisions mapped to the dispatch's
   requirement letters (A–N).
2. `docs/adrs/ADR-033-resource-capture-boundary.md` — 664 lines (was 992).
   Same 10-decision (D1–D10) shape as rev-0 but every decision's content is
   rewritten; see "Files Changed — Cluster H rev-1" above.

**Files changed (Cluster H rev-1)**:
- `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md` — rewritten
- `docs/adrs/ADR-033-resource-capture-boundary.md` — rewritten
- `docs/handoff/CURRENT.md` — this update

**No code, no assets, no ROADMAP.md, no SPEC.md, no CHANGELOG.md, no
supervisor/LOG.md, no existing PRD/ADR changes** (docs-only cluster, exactly
the three owned paths staged).

**Internal-consistency check** (Summary-vs-source cross-check):
- D1 (ADR) ↔ §4/§8.1 (PRD): both state tracked artifacts are
  hash/count/classification-only, never raw bytes; both cite the same
  gitignored local path shape.
- D3 (ADR) ↔ §12 (PRD): identical canonical-JSON `args` encoding and
  identical four golden vectors (`res_acc91dc23a8b`, `res_19b4675405e2`
  ×2, `res_79f5ac5dca13`), independently recomputed and matching in both
  documents.
- D8 (ADR) ↔ §7 (PRD): identical local batch layout, commit protocol, and
  crash-window table.
- D9 (ADR) ↔ §8 (PRD): identical six closed redaction classes and
  hard-refusal (not scrub-and-continue) policy.
- D10 (ADR) ↔ §10 (PRD): identical stage-then-publish ordering, identical
  partial-domain error framing, identical exit-code table.
- WP-006 is not cited anywhere in either document (replaced by the tracked
  `storage-substrate-and-versioned-data.md`).
- No raw `.git/**` capture is proposed anywhere; `.git`-target refusal (D6)
  and the `.git/**` boundary precedent (`ADR-030` D3/D4) are both cited
  consistently in both documents.
- All 41 PRD §13 acceptance-criteria clauses appear in the ADR's 66-row Test
  Matrix; the matrix explicitly disclaims any "exactly once" mapping claim.

**Validation performed**:
- `git diff --check` on both rewritten files — clean.
- Every `file:line` citation in the PRD's §0.1 Claims Audit re-verified
  directly against current source this session (see "Test Results —
  Cluster H rev-1" above for the full list).
- Dolt CLI syntax re-verified against the live DoltHub CLI reference this
  session (no fabricated flags or output schema).
- All four golden `resource_id` vectors independently recomputed via Python
  `hashlib.sha256` and confirmed matching in both documents, including the
  reordered-key equivalence proof.
- All four shared JSON wire examples programmatically confirmed
  byte-identical (raw string equality, not just semantic equality) between
  the PRD and the ADR.
- AC/test-matrix coverage mechanically verified: 41 distinct `AC-<n>` tags
  in the PRD, all 41 present in the ADR's 66-row Test Matrix.
- `git status --short` confirmed only the three owned paths are touched by
  this writer; pre-existing untracked WIP is untouched.
- Side Research md5 invariant re-verified unchanged:
  `b385fe622db9926f48861105239f113e`.

**Reviewer focus areas**:
- Confirm the new local-storage lane (`.tpatch/local/resource-capture/`)
  genuinely satisfies ADR-027 D1's ignored-before-first-write mandate as
  designed (D8/§7), and that no tracked artifact anywhere in the rewritten
  wire schema (§11/Wire Schema Appendix) contains raw bytes.
- Confirm the symlink-safety gate (D6/§9) is re-run at *every* capture, not
  just `add`, and that it covers the resolved Dolt executable path as well
  as ordinary resource selectors.
- Confirm the `ignored-file` dual gate (D4/§5.1) correctly closes the
  `--no-index` gap and that the removed "untracked but not ignored"
  acceptance case is genuinely gone, not just de-emphasized.
- Confirm the Dolt protocol (D7/§6) never asserts a fabricated JSON schema
  and that every flag cited is independently verifiable against the DoltHub
  reference.
- Confirm the `record --resources` stage-then-publish ordering (D10/§10)
  and partial-domain error framing are internally consistent and that no
  code-facing claim (exit codes, message text) contradicts `record`'s
  existing, unmodified behavior.
- Confirm the 41-clause/66-row AC/matrix accounting is genuinely mechanical
  (spot-check a handful of `AC-<n>` tags against their matrix rows) and not
  merely asserted.

## Ready for review — Cluster H rev-0

**Scope**: docs-only planning cluster. Two new deliverables, no code:

1. `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md` — new,
   1086 lines. Extends shipped v0.9 file claims (`PRD-feature-file-claims.md`)
   and record capture modes (`PRD-record-capture-modes.md`) with a typed
   feature-resource layer for content Git does not (or cannot) diff:
   explicit gitignored/generated files, logical Git metadata, Dolt schema/
   table diffs, and deterministic generic-command adapter snapshots.
   - §0 Claims Audit: 20 rows, every `file:line` anchor independently
     re-verified against current source during this session (not carried
     over from any stale draft).
   - §1.1 Existing-primitives preflight: explains why `claims.json` (a
     Git-diffable-path scoping primitive) and capture modes (Git-diff-only
     dispatch) cannot represent non-file, adapter-executed, or logical-view
     resources without conflating two different write-boundary guarantees.
   - §3 Command Surface: locks `tpatch feature resource add|list|remove|
     clear|diff <slug> ...` plus `tpatch record <slug> --resources`, with a
     candidate-command evaluation table showing alternatives considered.
   - §4 Persisted Manifest: new `resources.json` (NOT an extension of
     `claims.json`), full schema example, `res_<12hex>` deterministic ID.
   - §5 Resource Kinds: exactly three closed v1 kinds (`ignored-file`,
     `git-metadata` with a 4-view allowlist, `adapter-snapshot` with exactly
     two closed adapters `dolt`/`generic-command`) — explicitly closed, not
     runtime-plugin-extensible.
   - §6 Adapter Protocol: discovery (`exec.LookPath`), executable/version
     identity, args/env/stdin/cwd/timeout/output-limit contract, exit/error
     taxonomy mapped to tpatch's existing exit-code convention (1/2/3, no
     new exit code 4), deterministic normalization rules.
   - §7 Wire Envelope: snapshot/diff sidecar layout mirroring the existing
     `artifacts/post-apply.patch`/`patches/NNN-*.patch` canonical-vs-audit
     pattern from `docs/feature-layout.md`; full JSON example with non-null
     empty `added`/`removed`/`changed` arrays.
   - §8 Privacy/Safety: binds to ADR-027 D3 (redaction is a write
     precondition, hard failure on refusal), size/binary handling, symlink/
     path safety via `safety.EnsureSafeRepoPath`.
   - §9 Auto-record Integration: `record --resources` all-or-nothing batch
     transaction; a single adapter failure blocks the whole resource batch
     but never rolls back the already-completed Git-side canonical capture
     from the same invocation; `--dry-run` shares the staging code path.
   - §10 Interactions with claims, capture modes, patch generations,
     status/next/land/reconcile/remove — resources never gate any lifecycle
     command (sidecar-only authority).
   - §11 Backward Compatibility, §12 Explicit Deferrals (10 items), §13
     Implementation Notes for Cluster H', §14 30 numbered Acceptance
     Criteria, §15 4 Open Questions (each with a locked v1 default so
     Cluster H' is not blocked), §16 Disputes (none).

2. `docs/adrs/ADR-033-resource-capture-boundary.md` — new, 963 lines. Locks
   10 binding decisions (D1–D10, exceeding the required minimum of 8):
   - D1: new `resources.json` manifest, not an extension of `claims.json`
     (Alternative 2 chosen) — includes the byte-identical wire example
     shared with PRD §4.
   - D2: resource diffs are pure sidecar audit artifacts with zero lifecycle
     coupling; no command in `internal/workflow`/`internal/cli` reads
     `resources.json` or `resources/` for correctness (Alternative 2 chosen).
   - D3: `res_<12hex>` deterministic ID scheme, NUL-separated hash over
     `(feature, kind, selector, adapter, capability, canonical_args)`, plus
     locked per-kind selector normalization rules (Alternative 2 chosen).
   - D4: `ignored-file` `add` requires `gitutil.IsPathIgnored == true`
     (fail-closed on `ErrGitUnavailable`), plus ADR-027 D3 redaction as a
     write precondition (Alternative 2 chosen).
   - D5: `git-metadata` restricted to a closed 4-view allowlist (`refs`,
     `attributes`, `index-summary`, `config` with a locked-safe key
     pattern), with a `.git/**`-reference store-write-boundary refusal
     applied defense-in-depth regardless of which view produced the output,
     mirroring ADR-030 D3/D4's two-layer pattern (Alternative 2 chosen).
   - D6: adapters invoked via direct `exec.Command` argv (never a shell);
     env resolved by name-allowlist at execution time only; hard timeout and
     output-size caps on every invocation (Alternative 2 chosen).
   - D7: Dolt realized exclusively as one closed-set `adapter-snapshot`
     adapter name (`dolt`) with exactly two capabilities (`schema-diff`,
     `table-diff`), discovered at runtime, never embedded as a Go
     dependency, never granted write access or tpatch authority
     (Alternative 2 chosen).
   - D8: `record --resources` is an all-or-nothing batch (scratch-then-
     atomic-rename); independent of the already-completed Git-side capture;
     never writes `patch-generations.json`; `--dry-run` shares the staging
     path (Alternative 2 chosen).
   - D9: fixed-field JSON envelope (`version`, `feature`, `resource_id`,
     `kind`, `adapter`, `capability`, `executable_path`, `version_probe`,
     `raw_sha256`, `normalized_sha256`, `outcome`, `changes.{added,removed,
     changed}`, `body_ref`) via `json.Marshal` on a fixed struct
     (declaration-order serialization, ADR-032 D3 precedent) — includes the
     byte-identical wire example shared with PRD §7.
   - D10: no `patch-generations.json` coupling; `remove`/`clear` fully
     delete manifest rows and sidecar directories (mirrors `RemoveClaim`
     clean-delete, no tombstone); missing `resources.json` is the
     empty-manifest state (mirrors `LoadClaims`) (Alternative 2 chosen).
   - Implementation Notes (8 items) for Cluster H'.
   - Negative Consequences Summary (one row per decision, 10 rows).
   - Test Matrix: 45 rows, exactly 1:1-mapping all 30 PRD §14 acceptance
     criteria (AC-1..AC-30, each appearing exactly once in the `PRD AC`
     column) plus 15 untagged supporting safety/determinism rows. Rows 15,
     16, 19, 26, 30, and 45 are flagged safety-critical
     (`.git/**` exclusion, shell-injection safety, redaction-as-precondition,
     secret-by-reference, defense-in-depth store-write boundary, and the
     mechanical zero-lifecycle-coupling grep check).

**Files changed (Cluster H rev-0)**:
- `docs/prds/PRD-feature-resource-claims-and-capture-adapters.md` — new
- `docs/adrs/ADR-033-resource-capture-boundary.md` — new
- `docs/handoff/CURRENT.md` — this update

**No code, no assets, no ROADMAP.md, no SPEC.md, no CHANGELOG.md, no
supervisor/LOG.md, no existing PRD/ADR changes** (docs-only cluster,
exactly the three owned paths staged).

**Internal-consistency check** (Summary-vs-source cross-check, per this
cluster's review-risk controls):
- D1 (ADR) ↔ §4 (PRD): both choose a new, separate `resources.json`; the
  `resources.json` JSON example is byte-identical between the two documents
  (programmatically verified).
- D9 (ADR) ↔ §7 (PRD): both lock the same fixed-field envelope; the
  diff-envelope JSON example is byte-identical between the two documents
  (programmatically verified).
- D3 (ADR) `res_<12hex>` hash-input tuple order `(feature, kind, selector,
  adapter, capability, canonical_args)` matches PRD §4's field-rule prose
  for `resource_id` exactly.
- D6 (ADR) exit-code classification (adapter-missing/timeout/nonzero-exit/
  output-too-large → 1; unsafe-path/redaction-refused → 3; closed-set
  rejections → 2) matches PRD §6.7's exit/error taxonomy table exactly.
- WP-006 is cited throughout as `Status: Exploring` (not accepted); neither
  document claims non-Git replay is approved or in scope.
- No raw `.git/**` capture is proposed anywhere; both documents distinguish
  logical Git metadata (resolved refs/attributes/index-summary/config
  values) from filesystem-internal `.git/**` byte content throughout.
- All 4 PRD §15 Open Questions carry an explicit locked v1 default, so
  Cluster H' is not blocked by any deferred choice.

**Validation performed**:
- `git diff --check` on both new files — clean.
- Every `file:line` citation in both Claims Audits re-verified directly
  against current source this session; two incorrect citations found during
  self-review were corrected (see "Test Results — Cluster H rev-0" above).
- `git status --short` confirmed only the two new files are untracked by
  this writer; pre-existing untracked WIP is untouched.
- Side Research md5 invariant re-verified unchanged: `b385fe622db9926f48861105239f113e`.

**Reviewer focus areas**:
- Confirm D1's separate-manifest choice does not leave any realistic path
  for `ClaimKind` reuse to have been simpler without the stated costs.
- Confirm D2's "zero lifecycle coupling" claim by spot-checking that no
  reference to `resources.json`/`ResourcesManifest`/`resources/` exists in
  `internal/workflow/reconcile.go`, `internal/cli/land.go`, the dependency
  gate, or `internal/cli/verify.go` (there should be none — no code changed
  in this cluster, so this is trivially true today, but the ADR's Test
  Matrix row 45 should mechanically re-verify it at Cluster H' close).
- Confirm the D5 Git-metadata allowlist and its `config`-key exclusions are
  sufficient to prevent secret/credential leakage without over- or
  under-restricting legitimate use (e.g., is `user.email` genuinely safe to
  expose in a sidecar artifact for every repo's threat model?).
- Confirm the D8 all-or-nothing batch design is the right tradeoff versus
  partial-commit, given that a single flaky adapter can block an entire
  `record --resources` invocation's sidecar update.
- Confirm the AC/test-matrix 1:1 mapping is genuinely mechanically countable
  (30 unique `AC-N` tags, 45 total rows) and not merely asserted.

## Ready for review — Cluster G' rev-0

**Range**: `9e77617..1746ebb` (5 commits; WAVE_BASE excluded).

**Delivered**:
- `StateUnapplied` as the twelfth real FeatureState.
- Atomic `SaveFeatureStatus` temp-write/fsync/rename with prior-byte
  preservation on failure.
- `tpatch feature unapply <slug>` with read-only dry-run, strict reverse
  validation, detached-worktree preview, safe both-side touched-path
  snapshot/restore, D3 fixed audit envelope, and D6 rollback.
- Apply/reconcile/status/JSON/FEATURES/next/land/dependency/verify/reject/
  reopen/confirm-upstreamed integration.
- Corrected dependency semantics: edge creation allowed; unapplied hard parent
  does not satisfy apply.
- SPEC, dependency docs, six shipped assets, and parity anchors.
- 61-row matrix: 60 mechanically covered; row 3 manually audited.

**Commit map**:
- `34f0757` — dispatch tracking.
- `55817c3` — atomic store + state foundation.
- `15388b8` — core command and lifecycle integration.
- `f4f151d` — SPEC/docs/six-skill parity.
- `1746ebb` — adversarial safety fold.

**Review focus**:
1. D3 exact field order, stable empty arrays, hashes, paths, and audit truth.
2. D6 rollback under reverse/artifact/status failures, including rename,
   spaces, Unicode, symlinks, absent files, and modes.
3. Canonical-patch preservation across record/cycle/feature-patch/apply and
   incomplete/drifted reapply.
4. Immediate vs committed unapplied baseline semantics and base_commit.
5. Corrected no-Rule-7 edge-creation behavior plus hard-gate dissatisfaction.
6. All state/exit-code and reject/reopen/confirm-upstreamed interactions.

**Verification**:
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.
- Rule 18 trailer — 5/5 commits.
- Side Research md5 — `b385fe622db9926f48861105239f113e`.

**Known residuals**: none.

## Ready for review — Cluster G' rev-1

**Fold commit**: `7b65d7f` (dispatch/adjudication baseline `653b775`).
**Full implementation**: `9e77617..7b65d7f`.

**Rev-0 findings closed**:
1. Temporary per-file `git add --intent-to-add` and `git reset` calls now
   run under `--literal-pathspecs`. Regression coverage includes deleted
   `:(literal)gone.txt`, `*.txt`, and `[x].txt`.
2. Six asset diagrams now list all four legal source states. The exact
   `applied/active/reconciling/reconciling-shadow` source set is parity-locked.
3. Stale handoff-only `Copilot-Session` requirement removed; current Rule 18
   `Co-authored-by` requirement remains and parses on every commit.

**Verification**:
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.
- Side Research md5 — `b385fe622db9926f48861105239f113e`.

## Ready for review — Cluster G' rev-2

**Fold commit**: `a2a11b2` (adjudication baseline `46d4bdd`).
**Full implementation**: `9e77617..a2a11b2`.

**Rev-1 findings closed**:
1. Reapply executes strict canonical `post-apply.patch`; stale/missing recipe
   cannot block or redefine replay.
2. Hard-dependency and parent-generation gates run before already-materialized
   shortcut finalization.
3. Snapshot/restore supports directory entries and both file↔directory
   transitions in depth-safe remove/recreate phases.
4. Non-dependency amend operations refuse while unapplied before request.md
   writes; deps-only edits remain available.

**Verification**:
- Rev-2 targeted tests — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.
- Side Research md5 — `b385fe622db9926f48861105239f113e`.

## Ready for review — Cluster G' rev-3

**Fold commit**: `b11f1ee` (adjudication baseline `864744c`).
**Full implementation**: `9e77617..b11f1ee`.

**External rev-2 findings closed**:
1. `ValidatePatchReverse` and HEAD checks reject warning-only mode mismatch;
   mode-only immediate and committed-baseline reapply restore executable mode
   before marking applied.
2. Reapply diff/patch-id comparison uses canonical literal touched paths,
   ignoring unrelated disjoint dirty work while still rejecting extra changes
   on feature-owned paths.

**Verification**:
- Rev-3 targeted tests — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.
- Side Research md5 — `b385fe622db9926f48861105239f113e`.

## Ready for review — Cluster G' rev-4

**Fold commit**: `a5a814e` (adjudication baseline `3d19245`).
**Full implementation**: `9e77617..a5a814e`.

**Internal rev-3 finding closed**:
- Reapply comparison uses `DiffFromCommitForPaths(HEAD, canonicalPaths)` with
  a temporary index and literal pathspecs, so staged/unstaged/untracked
  canonical-owned changes are all visible while unrelated paths are excluded.
- Regression proves staged owned-path drift refuses and restores worktree,
  status, canonical patch and user index.

**Verification**:
- Rev-4 targeted test — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.
- Side Research md5 — `b385fe622db9926f48861105239f113e`.

## Ready for review — Cluster G' rev-5

**Fold commit**: `6941d41` (adjudication baseline `dcbd570`).
**Full implementation**: `9e77617..6941d41`.

**Internal rev-4 finding closed**:
- `DiffFromCommitForPaths` resolves the effective Git index via
  `git rev-parse --git-path index` and seeds its temporary index from that
  path; linked worktrees no longer produce zero-byte-index failures.
- Regression uses a real linked worktree with a staged change and proves
  complete diff visibility plus unchanged status/index tree.

**Verification**:
- Linked-worktree targeted test — PASS.
- `gofmt -l .` — clean.
- `go vet ./...` — PASS.
- `go test -count=1 ./...` — PASS.
- `go build ./cmd/tpatch` — PASS.
- Side Research md5 — `b385fe622db9926f48861105239f113e`.

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

## Ready for review — Cluster F' rev-1

**Scope delivered**: all 7 findings from the Cluster F' rev-0 dual review, folded. None deferred, none partially folded. Commit range `d3e5a11..HEAD` (8 commits).

### Commit map

| # | Commit | Finding | Scope |
|---|--------|---------|-------|
| 1 | `aa9b17a` | F-INT-1 BLOCKING | history schema: one entry per completed cycle, appended on reopen only |
| 2 | `9cf7a29` | F-INT-2 HIGH | dedicated `status --json` DTO with §8-conformant field names |
| 3 | `7eca395` | F-INT-3 HIGH | evidence validation precedes the state-machine check in reject/reopen |
| 4 | `2903ffc` | F-INT-4 / F-EXT-1 HIGH | `ErrRejectedParent` → exit 3 at both edge-creation surfaces + §8 golden string |
| 5 | `1b1f2c7` | F-INT-5 MEDIUM | evidence fallback only on genuine not-found |
| 6 | `39194c9` | F-INT-6 LOW | independent golden-string literals for test 27 |
| 7 | `a60c4c4` | F-EXT-2 LOW | Oxford "or" in the reject wrong-state error |
| 8 | (this) | — | docs: CURRENT.md session summary |

### Finding-by-finding disposition

**F-INT-1 BLOCKING — history schema.** The `action` discriminator is gone. `RejectionHistoryEntry` is now a completed-cycle record; `reject` appends nothing; `reopen` snapshots the live `Rejection` into the entry's reject half, records the reopen half, appends, and then sets `Rejection = nil`. `History` moved off `RejectionStatus` onto `FeatureStatus.RejectionHistory` — that relocation is what makes clearing `Rejection` non-destructive. Invariants now asserted:

| after | `Rejection` | `len(RejectionHistory)` |
|---|---|---|
| reject only | non-nil | 0 |
| one reject→reopen cycle | nil | 1 |
| N cycles | nil | N |

This supersedes rev-0's "One deliberate deviation from a PRD illustrative example" — the reviewer's reading is adopted in full, and PRD §8's `history_entries: 1` after one reopen is now what the CLI emits (which also closes F-EXT-3 as the adjudication anticipated).

**F-INT-2 HIGH — status --json DTO.** `rejectionStatusView` renders the §8 key set exactly — `reason`, `evidence[{path,sha256}]`, `note`, `rejected_at`, `rejected_by`, `prior_state`, `related` — and is attached at depth 0 on `featureWithFreshness` so it shadows the embedded `FeatureStatus.Rejection`. The internal `actor` name can no longer reach the wire. The object is emitted only when `state == "rejected"`; post-reopen the envelope carries `rejection_history` and no `rejection`. `TestStatusJSON_RejectionDTOMatchesSpecFieldNames` asserts the key set is exact in both directions (no extra keys, no missing keys).

**F-INT-3 HIGH — validation ordering.** Both commands now run reason → note → evidence (resolve, path-safety, hash) → load status → state check → dependents check → mutate. The store is opened before evidence resolution because resolution needs the repository root; only the status LOAD and the store-state checks moved after it. Three combined-invalidity cases assert exit 2 wins over exit 3.

**F-INT-4 / F-EXT-1 HIGH — exit 3 at the edge boundary.** `mapDependencyValidationError` wraps `ErrRejectedParent` in `&ExitCodeError{Code: 3}` at both `ValidateDependencies` call sites in `feature_deps.go` and again at the `amend --depends-on` / `--remove-depends-on` boundary. The golden string now matches PRD §8 verbatim, achieved by making the sentinel's own text the leading clause. The symmetry test asserts `code == 3` and the verbatim string for 3 edge kinds × 2 surfaces.

**F-INT-5 MEDIUM — evidence fallback.** Fallback to the repo-root candidate is taken only on `os.IsNotExist`. Non-regular, path-safety-failed and unreadable candidates terminate resolution with their taxonomy reason. The regression test stands up a directory at the feature-dir path shadowing a regular file at the root path and asserts the root decoy is never passed to the `evidenceHashFn` seam.

**F-INT-6 LOW — test 27 independence.** `wantRejectSnippet` / `wantReconcileSnippet` are literals holding the PRD §4.1 wording. Editing either production constant now fails the test.

**F-EXT-2 LOW — Oxford "or".** `joinStatesOr` renders `"requested, analyzed, or defined"`; the wrong-state refusal uses it and is asserted verbatim against PRD §8. `joinStates` is retained for the `--help` "Allowed source states:" line, where a plain list reads correctly. The implementation was fixed rather than the PRD, per the adjudication's stated preference.

### Judgment calls the reviewer may want to check

1. **`RejectionHistoryEntry` JSON tags follow the dispatch brief, not PRD §6's prose.** §6's table says the history entry "snapshots the fields above (`reason`, `evidence`, `note`, …)", which would imply bare `note`/`evidence` for the reject half alongside `reopen_note`/`reopen_evidence` for the reopen half — an asymmetric shape. The rev-1 dispatch brief specifies the symmetric `reject_note`/`reject_evidence` ↔ `reopen_note`/`reopen_evidence` pair, and that is what shipped. §6's prose is loose here rather than contradictory (it enumerates *which* fields are snapshotted, not their tags), but flagging it: if the reviewer prefers §6-literal tags, it is a 4-tag rename plus the key list in `TestRejectionHistoryEntry_CompletedCycleRoundTrip`.
2. **`prior_state` and `related` are carried on the history entry** even though the brief's struct sketch omitted them. Without them a completed cycle would lose the two audit fields PRD §6 explicitly names as snapshotted, and reopen needs to read `PriorState` before nilling `Rejection` anyway.
3. **`reopen` still transitions to `requested`, not to `prior_state`.** The brief's sketch had a `// transition to prior state` comment, but PRD §3.8 / §5 / §8 all fix `rejected → requested`, and rev-0's adjudication row #4 already resolved this in the PRD's favour. Unchanged from rev-0.
4. **The completed-cycle entry retains the ORIGINAL reject-time evidence hashes.** Divergence detection records its verdict in `evidence_integrity` / `divergence_detail` and never rewrites `reject_evidence`, so the audit record still says what was reviewed at rejection time.
5. **`status --json` renders `rejection_history` via the embedded struct**, not a second view type — the entry's tags are already the spec names from finding 1, so a second DTO would be a pure identity mapping.

### Non-invalidation invariants

- Side Research md5 `b385fe622db9926f48861105239f113e` — **preserved**, verified post-fold.
- Canonical `**Cluster state**:` field — **untouched** (still `REV-1 DISPATCHED`; supervisor flips at close).
- `internal/workflow/reconcile.go` and `retirement_audit.go` — **untouched** (ADR D6).
- `assets/`, `SPEC.md`, `docs/prds/`, `docs/adrs/` — **untouched** in rev-1; parity guard unchanged and passing.
- No file from `.wave-close-allowlist` staged; all 8 commits used explicit `git add <path>` and `git commit -F <file>`.
- Every commit carries the Rule 18 trailer + the `Copilot-Session` trailer (`[4/8]` verified 21/21 in `c6aaeb2..HEAD`).

## Ready for review — Cluster F' rev-2

**Scope**: one MEDIUM finding F-INT-Rev1-1 (dangling-symlink guard). Single commit `1492fb0`.

**Problem**: `resolveEvidence` used `os.IsNotExist(EvalSymlinks(cand))` to decide whether to fall through to the root candidate. `EvalSymlinks` returns ENOENT for a dangling symlink (the symlink entry exists; its target does not), so `os.IsNotExist` was true and the code fell through — potentially hashing a root-level decoy file as evidence.

**Fix** (`internal/cli/reject.go`): after `os.IsNotExist` on `EvalSymlinks`, call `os.Lstat(cand)`:
- Lstat also returns not-found → true absence → `continue` to root candidate (unchanged behavior).
- Lstat succeeds → dangling symlink entry present → `return "", store.DivergentReasonUnreadable` without falling through.

**Test** (`internal/cli/reject_test.go`): `TestReject_EvidenceDanglingSymlinkNotFallenThrough`:
1. Feature-dir entry is a dangling symlink (target does not exist).
2. Root-level decoy of same name is a regular file.
3. Asserts exit 2; asserts `evidenceHashFn` never called with the decoy path; asserts no `Rejection` written.
4. `t.Skipf` on `os.Symlink` failure (Windows guard, matching existing test pattern at line 576/911).

**Validation**:
- `gofmt -l .` clean · `go vet ./...` clean · `go build ./cmd/tpatch` OK.
- `go test -count=1 ./...` **971 top-level PASS / 0 FAIL** (+1 test over rev-1).
- `md5 -q <(sed -n '/^## Side Research/,$p' docs/handoff/CURRENT.md)` = `b385fe622db9926f48861105239f113e` ✅.
- `make wave-close-check WAVE_BASE=c6aaeb2`: [1/8]–[4/8] OK · [5/8] FAIL by design (state = REV-2 DISPATCHED) · [6/8]–[8/8] OK.

**Non-goals confirmed**: no other files touched; DTO, history schema, exit-code mapping, Oxford joiner, test 27, and all rev-1 changes untouched.

 (post-v0.13.0 candidates, not for Cluster F prime)

- **prd-verify-post-commit-mode** (MEDIUM, external user report 2026-08-05). `tpatch verify` fails V8 `post_apply_patch_replay_clean` on already-committed features with a misleading "run tpatch reconcile" remediation. Root cause verified at `internal/workflow/verify.go:1160-1163`: V8 resets the shadow to the closure-replayed baseline (target recipe NOT applied) then git-apply-checks post-apply.patch — on a local repo whose HEAD already contains the feature, the check conflicts. PRD questions: detect no-upstream state, seed shadow from pre-feature base, conditional remediation string. Full description in todos table row `prd-verify-post-commit-mode`.
- **prd-no-upstream-mode** (MEDIUM, external user report 2026-08-05, sibling of prd-verify-post-commit-mode). Local-only mode for repos without configured upstream: `upstream_merged`/`reconciling`/`reconciling-shadow`/confirm-upstreamed states presume upstream exists. Design questions: detection (config flag vs. absence of `upstream.lock` vs. git remote absence vs. `tpatch init --local`), migration path when local repo later configures upstream, which lifecycle states become unavailable, CLI remediation-string changes across `verify`/`reconcile`/`next`/`land`/`status`. Likely paired with prd-verify-post-commit-mode as single "local-first tpatch" PRD/ADR pair. **Depends on `wp-006-tpatch-substrate-and-non-git-mode`** (merge candidate — WP-006 is the broader substrate-abstraction whitepaper foundation). Full description in todos table row `prd-no-upstream-mode`.

**Untracked-WIP paper inventory (2026-08-05, registered from allowlist review)** — 16 files in `.wave-close-allowlist`, all substantive drafts (not scratch). Todos rows created for each:

- **`prd-feature-unapply`** (HIGH priority — Draft 2026-07-10, 587 lines → refreshed to ~950 lines 2026-08-05): **✅ CLUSTER G PLANNING DISPATCHED FOR REVIEW.** PRD refreshed with v0.13.0 compat sections + ADR-032 authored from scratch. See "Ready for review — Cluster G rev-0" below.
- **`prd-recurring-patches`** (Approved paper 2026-06-26, 433 lines): implementation gated on ADR-recurring-patch-metadata-boundary. Draft the gating ADR to unblock.
- **`wp-004-auto-feature-dependencies`** (Approved paper research 2026-06-25, 488 lines): elevate to PRD-auto-dependencies + ADR pair. Feeds off case study `t3code-upstream-v0.0.23-2026-05` (also untracked/allowlisted).
- **`wp-005-spec-driven-workflows`** (Exploring 2026-06-25, 530 lines): OpenSpec/GitHub-Spec/tpatch comparison. Related to WP-001. Elevate-or-archive decision pending.
- **`wp-006-tpatch-substrate-and-non-git-mode`** (Exploring 2026-07-10, 379 lines): **merge candidate with `prd-no-upstream-mode`**. WP-006 explores broader substrate abstraction below git + non-git mode; prd-no-upstream-mode is narrower "local repo without upstream" scenario. Decide: (a) merge WP-006 into prd-no-upstream-mode as paper foundation, or (b) elevate WP-006 to its own broader PRD/ADR with prd-no-upstream-mode as a milestone within it.
- **`wp-007-decision-tickets-and-ticket-tracking`** (Exploring 2026-07-16, 414 lines): decision-ticket tracking. May overlap with ADR conventions. Elevate-or-archive decision pending.
- **Case study fixtures** (7 files under `docs/state-of-the-art/case-studies/`): `t3code-upstream-v0.0.23-2026-05/dependency-analysis.md` feeds WP-003/WP-004; `tws-dev-2026-05/{summary,local-notes,dependency-analysis,metrics,study}.md,json` is real-world v0.6.1 usage evidence. Retain as-is; disposition follows parent paper elevation.
- **`.turns.md` siblings for WP-004..007** (4 files): session traces. Retain paired with parent papers; archive when parents elevate or archive.

**`investigate-test-suite-wedge`** (LOW priority, tooling): external reviewer reports 3 consecutive sessions where `go test -count=1 ./...` wedges terminal partway through. Since `[8/8]` gate runs this suite, intermittent wedge could block wave-close. Investigate output-buffering / long-running child issue (possibly macOS-tty specific). Deliverable: repro recipe + fix or documented workaround.

## Ready for review — Cluster G rev-3

**Fold scope**: 3 items, ADR-032 test matrix + Claims Audit only.

| # | Finding | Action |
|---|---|---|
| Item 1 (HIGH) | Matrix row 43 said `reconciling-shadow` → refused (exit 3), contradicting PRD §3.5:271 and AC-35:920 which list it as a PERMITTED source state. Also missing `defined`, `implementing`, `blocked`, `upstream_merged` refusal rows | Replaced rows 39-43 with 6 rows: rows 39-42 = 4 permitted source states (applied, active, reconciling, reconciling-shadow) each asserting exit 0; rows 43-44 = 8 refused states grouped in 2 consolidated rows asserting exit 3. Verified against `sed -n '265,280p' PRD` before writing |
| Item 2 (MEDIUM) | AC-10c had zero matrix rows despite header claiming "3 §10 atomicity ACs" | Added row 51 (AC-10c): after any rollback, `LoadFeatureStatus` returns nil error and previous `state` value |
| Item 3 (INFO) | Claims Audit line 47 said "immediately after status load" — loose framing pointing at the caller | Fixed to "first statement of `applyConfirmUpstreamedTransition` at `cobra.go:2626`" |

**Files changed (Cluster G rev-3)**:
- `docs/adrs/ADR-032-feature-unapply-state-boundary.md` — test matrix rows 39-61 (AC-35 fix + AC-10c + renumber); Claims Audit line 47 phrasing

## Ready for review — Cluster G rev-2

**Fold scope**: 4 items, ADR-032 only + CURRENT.md fix.

| # | Finding | Action |
|---|---|---|
| Item 1 (HIGH) | Impl Note 4 caller/callee direction INVERTED — said "do NOT place guard in `applyConfirmUpstreamedTransition`" but that IS where the guard belongs; real callee to avoid is `saveConfirmUpstreamedStatus` (`cobra.go:2699`) | Rewrote Impl Note 4: guard is FIRST STATEMENT of `applyConfirmUpstreamedTransition` (`cobra.go:2626`), mirroring `StateRejected` guard at `cobra.go:2635-2648`; do NOT place in `saveConfirmUpstreamedStatus`; verbatim quote from `cobra.go:2627-2634` source comment added |
| Item 2 (HIGH) | Test matrix claimed "1:1 mirror" but 10+ ACs missing; 5 tags mis-attached (AC-1, AC-28, AC-29, AC-36, AC-21); AC-27 safety-critical absent | Matrix expanded from 40 → 59 rows; all 39 PRD §15 ACs + 3 §10 atomicity ACs (10a/10b) covered; AC-35 split into 5 per-source-state rows; all tags corrected; AC-27 safety-critical explicitly called out with bold |
| Item 3 (HIGH) | D6 status.json write atomicity gap: `SaveFeatureStatus` uses `os.WriteFile` (non-atomic) but D6 only promised invariant without specifying how | Added to D6 step 8: write MUST use `os.CreateTemp` + write + `os.Rename` (POSIX-atomic); cited `store.go:368` and `store.go:829`; Cluster G' pre-req noted; D6 Consequences updated; PRD §10 expanded with 8-step numbered protocol + AC-10a/AC-10b/AC-10c |
| Item 4 (LOW) | CURRENT.md rev-1 summary (line 421) said "state = REV-0 DISPATCHED" instead of "REV-1 DISPATCHED" | Fixed to "REV-1 DISPATCHED" |

**Files changed (Cluster G rev-2)**:
- `docs/adrs/ADR-032-feature-unapply-state-boundary.md` — Impl Note 4 rewrite (Item 1); test matrix 40→59 rows with correct AC tags (Item 2); D6 step 8 atomicity + Consequences update (Item 3)
- `docs/prds/PRD-feature-unapply.md` — §10 expanded with 8-step numbered protocol + AC-10a/10b/10c + atomic-write requirement (Item 3)
- `docs/handoff/CURRENT.md` — line 421 state token fix (Item 4)

**Anchor verification**: `cobra.go:2626` (`applyConfirmUpstreamedTransition` func decl), `cobra.go:2627-2634` (source comment), `cobra.go:2635-2648` (`StateRejected` guard body), `cobra.go:2699` (`saveConfirmUpstreamedStatus` decl), `store.go:368` (`SaveFeatureStatus`), `store.go:821-829` (`writeJSON`/`writeFile`) — all verified with `sed -n` against HEAD.

## Ready for review — Cluster G rev-1

**Fold scope**: 13 items across two documents (PRD + ADR). All anchor cites re-verified
with `grep -n`/`sed -n` before writing.

| # | Finding | Action |
|---|---|---|
| G1 | `RejectableStates` cited `status.go:108-118` (6 occurrences across both docs) | Fixed to `status.go:135-145` in all occurrences |
| G2 | `RejectionStatus` cited `status.go:98-106` | Fixed to `status.go:119-133`; added missing `related` field to PRD §0 |
| G3 | `confirm-upstreamed` guard cited `cobra.go:2525-2540` | Fixed to `cobra.go:2635-2648` in PRD §11.7 and ADR Claims Audit; ADR Impl Note 4 rewritten with correct guard placement (immediately after status load, before fast-path, using `stateRefusalError`) |
| G4 | `reconcile.go:205-214` cited for reverse-apply | Fixed to `reconcile.go:353-357` in PRD §0 |
| G5 | `store.go:509-521` cited for RemoveFeature | Fixed to `store.go:658-666` in PRD §0 |
| G6 | Fabricated `feature_deps.go` quote in PRD §3.4 | Replaced with actual doc comment at line 38 and `Short` at line 45 (no quotation marks on fabricated text) |
| G7 | ADR-031 D6 blockquote cited at `:604-630`; wrong text | Fixed cite to `ADR-031:1111-1114`; fixed verbatim text to match actual ADR-031 content |
| G-2 | Wire-schema divergence: `attempted_at` and `actor` absent from PRD §7.1 | Added to PRD §7.1 example; both examples now byte-for-byte identical; `--actor` added to command syntax; removed stale "stable-sorted" claim (replaced with "struct-field order"); `omitempty` language removed |
| G-3 | Absolute invariant in PRD §5.1 contradicts ADR D2 | Softened to "best-effort gate + race-detection via DAG warning"; added supersedes dependent policy (refused, no bypass); removed false absolute invariant |
| G-4 | Exit-code envelope missing | Added §3.5 binding exit-code table to PRD; ADR D6 8-step protocol already covers the mechanics |
| G-5 | confirm-upstreamed guard placement (wrong callee vs caller) | ADR Impl Note 4 rewritten: guard must be in caller immediately after loading status, not in `applyConfirmUpstreamedTransition` |
| G-6 | v1 dirty-tree restriction undocumented (applied-and-dirty workflow unsupported) | Added §12.1 to PRD; added AC-39 to §15; added row 39 to ADR test matrix |
| G-7 | Artifact-write failure not covered by D6 atomicity | Extended D6 protocol to 8 steps; added artifact-write and status-write rollback triggers with exit 1 |
| G-8/G10 | Test matrix 30 rows; missing coverage (AC-20 verify, AC-32 supersedes, AC-33 actor, AC-34 unknown-slug, AC-35 wrong-state, AC-36 dry-run, AC-37 reject-from-unapplied, AC-38 confirm-upstreamed, AC-39 committed-patch, rollback) | Matrix grown to 40 rows; header changed to "1:1 mirror of PRD §15" |
| G9 | Old-binary/new-state backward compat undocumented | Added to PRD §13 |
| related | PRD §0 composition framing oversold D6 closure | §8.2 reframed: D7 resolves data-model composition sub-question only; retirement-command gap explicitly stated as still deferred; `tpatch remove` named as destructive workaround |

**Files changed (Cluster G rev-1)**:
- `docs/prds/PRD-feature-unapply.md` — anchor sweep + new sections (§3.5, §5.1 supersedes, §8.2 reframe, §12.1, §13 old-binary, §15 AC-32–39)
- `docs/adrs/ADR-032-feature-unapply-state-boundary.md` — anchor sweep + D3 schema fix + D6 atomicity 8-step + D7 reframe + D8 feature_deps fix + Impl Note 4 rewrite + test matrix 40 rows + Related PRD-rejected-feature-state

**wave-close-check**: [5/8] intentional FAIL (state = REV-1 DISPATCHED); 7/8 expected PASS.

## Ready for review — Cluster G rev-0

**Scope**: docs-only planning cluster. Two deliverables:

1. `docs/prds/PRD-feature-unapply.md` — refreshed from 587-line draft (2026-07-10, untracked)
   to ~950 lines (tracked at rev-0). Additions:
   - Header: status → `Rev-0 (Cluster G planning)`, milestone → `v0.14.0 candidate`.
   - Related section: added ADR-031 (D6 deferral source), ADR-032, PRD-rejected-feature-state.
   - Implementation Gate: updated ADR slug to `ADR-032-feature-unapply-state-boundary`; added D7/D8 bullet points.
   - §0 Claims Audit: 8 new v0.13.0 rows (`StateRejected`, `RejectableStates`, `RejectionStatus`/`RejectionHistoryEntry`, Rule 7 / `ErrRejectedParent`, `reject`/`reopen` top-level verbs, `status --include-rejected` pattern, ADR-031 D6 deferral text).
   - §3.4 (NEW): Intentional naming asymmetry with `reject`/`reopen` — 4-point rationale + golden `--help` cross-reference strings (Cluster F' test-27 precedent).
   - §5.1 (NEW): Rule 7 interaction — dependency edges onto `unapplied` parents allowed; symmetric-invariant statement; unapply with hard dependents refused; `--allow-soft-dependents` bypass for soft edges.
   - §8.2 (NEW): Composition with `rejected` state — three options (A/B/C) enumerated; Alternative A (parallel independent states) chosen with 4-point rationale; consequence: `reject` refuses `unapplied` at exit 3.
   - §8.3 (NEW): Status filtering — `unapplied` shown by default (no `--include-unapplied` flag); `[unapplied]` badge; `next` recommends `tpatch apply`; `FEATURES.md ## Unapplied` section after active/applied, before `## Rejected`.
   - §9: Added v0.13.0 rows for `tpatch reject`, `tpatch reopen`, dependency gate on unapplied parent, `--include-rejected` non-interaction, docs/skills, and expanded test coverage row.
   - §11.7 (NEW): Interactions with `reject`/`reopen` — reject refused from `unapplied` (exit 3), reopen non-interaction, `confirm-upstreamed` guard proposal.
   - §16: OQ 5 added (composition — resolved via §8.2). OQ 3 expanded with §5.1 ADR follow-up.
   - §17: Composition dispute entry (resolved), `confirm-upstreamed` guard dispute (new, open).

2. `docs/adrs/ADR-032-feature-unapply-state-boundary.md` — authored from scratch. 8 decision
   points (D1–D8), each with ≥3 alternatives + rationale + consequences + cited precedents.
   - D1: `unapplied` as real `FeatureState` — Alternative 3 chosen.
   - D2: Does not satisfy hard deps — Alternative 2 chosen.
   - D3: `unapply-session.json` wire schema locked byte-for-byte (including `[]` not `null` for arrays, F-INT-1 lesson).
   - D4: No patch-generation writes — Alternative 2 chosen.
   - D5: Patch-mode-only v1 — Alternative 2 chosen.
   - D6: Snapshot/restore failure atomicity — Alternative 2 chosen with 7-step protocol.
   - D7: Parallel independent states with `rejected` — Alternative A chosen; ADR-031 D6 cited as deferral source.
   - D8: `feature unapply` under `feature` group — Alternative 2 chosen; contrast with ADR-031 D10's opposite decision documented.
   - Implementation Notes (12 items for Cluster G').
   - Negative Consequences table (8 rows, one per D).
   - 30-item test matrix baseline.

**`.wave-close-allowlist`** updated: `docs/prds/PRD-feature-unapply.md` removed (now tracked).

**Files changed (Cluster G rev-0)**:
- `docs/prds/PRD-feature-unapply.md` — refreshed (was untracked, now tracked)
- `docs/adrs/ADR-032-feature-unapply-state-boundary.md` — new
- `.wave-close-allowlist` — PRD-feature-unapply.md entry removed
- `docs/handoff/CURRENT.md` — session summary

**No code, no tests, no SPEC.md changes** (docs-only cluster per non-goals).

**wave-close-check**: `make wave-close-check WAVE_BASE=2c8a207`
- `[5/8]` FAIL by design: state = `REV-0 DISPATCHED` (supervisor flips at wave close).
- All other checks expected PASS (docs-only cluster: go build/test/fmt unaffected).

**Internal-consistency check**:
- D7 (ADR-032) ↔ §8.2 (PRD): both choose Alternative A / parallel independent states. ✅
- D3 schema (ADR-032) ↔ §7.1 example (PRD): keys match (`version`, `feature`, `attempt_id`, `mode`, `previous_state`, `result`, `canonical_patch_sha256`, `reverse_patch`, `touched_paths`, `dependency_blockers`, `preflight`). ✅ ADR-032 adds `attempted_at` and `actor` not in PRD §7.1 example — these are additions to the PRD example, not conflicts.
- D8 (ADR-032) ↔ §3.4 (PRD): both say `feature unapply` under `feature` group. ✅

**Open reviewer questions from this revision** (not blocking, informational):
- PRD §7.1 example omits `attempted_at` and `actor`; ADR-032 D3 schema adds them. If reviewers feel the PRD example should be updated to match D3 exactly, that is a fold item for the rev-0 response.
- ADR-032 D2 defers the exact label name (`parent-unapplied` vs reuse of `blocked-by-parent`) to Cluster G'. Reviewers may prefer the label name to be locked here.

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
