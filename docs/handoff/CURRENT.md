# Current Handoff

## Status

**Cluster state**: APPROVED

**TW unattended window in progress; v0.22.0 released; wave 2 reviewed
and validated, gate 3 at this close commit.** v0.22.0 (tag on
`d699713`) ships the gated `3239556`. Wave 2's five commits passed
independent review and validation-machine checks. CI on wave 2's first
close commit `c92d371` failed one test: the frozen-source pin PIB-391
for `internal/intent/avp_guards_test.go`, which the GH #17 change edited.
Gate 2 was stopped; `420c485` moves the pin (review APPROVED, validated;
TW-D29, TW-D30). This tracking commit is wave 2's gated close commit, so
the canonical field stays terminal (HK-D9). The operator
authorized a 12-hour unattended window (TW) from 2026-11-12T20:31:32Z to
**2026-11-13T08:31:32Z** (TW-D1). Wave 1 releases **v0.22.0**: GH #26 N2
(approved at `a37e512`) plus a refresh of the 52 displaced
`verify_anchored.go` doc cites (SW-D25). Wave 2: a cite guard, the GH #17
prerequisites, the ROCS8 build-once fixture, the TW-D6 post-tag status
lines, and a measurement of the S6 shard merge (measure only; runs after
gate 2). GH #25 B is not authorized.

## Active Task

- **Task ID**: tw-window (wave 1: `release-v0.22.0`; wave 2: `tw-wave-2`)
- **Milestone**: Post-v0.21 backlog — GH #26 close, SW-D25, GH #17, SW-D14
- **Description**: Release N2 and the doc-cite refresh as v0.22.0, then
  the cite guard, GH #17 prerequisites, ROCS8 fixture template and S6
  shard measurement
- **Status**: In progress — wave 1 shipped v0.22.0 (serial gate PASS at
  `3239556`, CI green); wave 2 reviewed and validated; its PIB-391 pin
  fixed in `420c485`; serial gate 3 at this close commit; then the S6
  measurement and the window close
- **Assigned**: 2026-11-12
- **WAVE_BASE** (TW): `dd4c082a7cbf04b61e3d5514c580e10ac11ee9ec`
  (`origin/main` at dispatch; both TW gates use it)

## Session Summary

- **Dispatch (2026-11-12T20:31:32Z).** Baseline verified: `HEAD` ==
  `origin/main` == `dd4c082`, no tracked changes, no stash, one
  worktree, no Git operation in progress; CI 43130446490 green; the
  validation machine idle.
- The SW handoff is archived byte-for-byte in `docs/handoff/HISTORY.md`
  under **2026-11-12T00:48:26Z - SW unattended window closed; v0.21.0
  shipped; GH #26 N2 approved**, with the SW-D* decisions.
- **SW-Q1 (TW-A1, TW-D2).** Deleted exactly the 72 third-party files
  under the ignored `bin/` snapshots; a private hash manifest records
  them. Two private draft test files in our own snapshots do not parse,
  so `gofmt -l .` still reports them (TW-Q1). Tracked files are clean.
- **Cite refresh (`f66523a`, TW-D3, TW-D4).** 52 displaced
  `verify_anchored.go` cites on 49 lines of six docs now point at the
  code they describe; numbers only. Independent review (gpt-6.1-sol):
  **APPROVED**, no findings. It opened every new range against the code,
  confirmed the 50 pinned, 2 still-correct and 2 uncertain cites are
  unchanged, and confirmed statically that the doc guards'
  fingerprints and bounds are unaffected. Doc guards on the validation
  machine: see Test Results.
- **Release candidate (`1673d03` plus this commit, TW-D5, TW-D6).**
  CHANGELOG `## v0.22.0`, release pin `0.22.0` and the versioned heading
  fixture in `assets/recipe_authority_parity_test.go`, same shape as
  `f7beb47`. Independent review (gpt-6.1-sol): **APPROVED WITH NOTES**.
  This commit fixes its two notes: "Known limits" now says the block
  needs the parent's canonical patch to be absent, and the Context no
  longer says the pin is `0.21.0`. That commit, `3239556`, is the gated
  candidate.
- **Release (TW-D7, TW-D13, TW-D14).** The candidate's doc checks passed
  on the validation machine; CI 43304207390 passed all five jobs after a
  re-run of the macOS test job, which GitHub had never assigned a runner;
  the serial gate passed (42/42, 2026-11-13T00:35:46Z). The
  tracking-only commit `d699713` records the verdict, passed
  `make release-check` 9/9 and is tagged v0.22.0 (pushed 00:40Z).
- **Wave 2 (`6953fef..ed5c232`, rebased onto `d699713`; TW-D8..D12,
  TW-D15..D27).** Independent reviews (gpt-6.1-sol) and
  validation-machine checks:
  - **GH #17 prerequisites** (`6953fef`, `dc76fa2`): all 13 unchecked
    `report["abort"]` casts and the remaining AVP decoded-JSON casts are
    checked; the Windows full-suite step's `-timeout` is 40m. Review
    APPROVED WITH NOTES, then APPROVED; the notes' casts outside AVP are
    inventoried for GH #17 (TW-D25).
  - **Cite guard** (`cd4ca2a`, `d713ef6`): a ledger in
    `internal/workflow/verify_anchored_cites_test.go` classifies every
    `verify_anchored.go` doc cite as live (anchor text must sit inside the
    cited lines) or historical (pinned commit, frozen); 91 rows, 110
    cites. Rev-1 NEEDS REVISION (two wrapped continuation cites escaped
    the ledger); rev-2 APPROVED, no findings.
  - **ROCS8 build-once fixture** (`7ac05c0`): each ROCS8 fixture kind is
    built once per test process and copied per row; Windows keeps fresh
    builds. Review APPROVED WITH NOTES (notes handled, TW-D20).
  - **TW-D6** (`ed5c232`): ADR-013 and PRD-historical-anchor-relanding
    say N2 shipped in v0.22.0.
- **Wave 2 CI failure and fix (TW-D28..D30).** `c92d371` was pushed
  while `d699713`'s `main` CI was still running; that run was cancelled
  and the tag run covers the same commit (TW-D28). CI 43346676120 on
  `c92d371` then failed `TestPreparePIBPreChangeGoldens/PIB-391` on
  Ubuntu and macOS (the only failure; later shard commands did not run):
  `internal/intent/avp_guards_test.go` is frozen accepted check source,
  and `6953fef` changed the `-timeout 20m` literals in five of its
  mutation arms to follow the Windows step to 40m, without moving its
  `CurrentSHA256` pin. Gate 2 was stopped about 12 minutes in. `420c485`
  moves only that pin, as `871d3ab`, `0dd96e1` and `676b9ba` did; the
  accepted-tip hash and history are unchanged. Independent review
  (gpt-6.1-sol): **APPROVED**, no findings; it found no other stale pin
  in the files wave 2 changed.

### Operator answers (2026-11-12, recorded verbatim in substance)

| ID | Answer |
|---|---|
| TW-A1 | SW-Q1: delete the 72 unformatted third-party files under the private `bin/` snapshots. |
| TW-A2 | Release first: v0.22.0 with GH #26 N2 and the doc-cite refresh, then wave 2. |
| TW-A3 | Wave 2 scope: cite guard, GH #17 prerequisites, ROCS8 fixture fix, S6 shard-merge measurement (measure only). |
| TW-A4 | GH #25 B: not now. |
| TW-A5 | Start now, 12 hours, unattended; record decisions for morning review. |

### Decisions for operator review

| ID | Decision |
|---|---|
| TW-D1 | Window starts at the baseline check, 2026-11-12T20:31:32Z (local and GitHub clocks agree); deadline 2026-11-13T08:31:32Z; WAVE_BASE `dd4c082`. |
| TW-D2 | **SW-Q1 executed literally.** I deleted exactly the 72 files the answer covered (36 copies each of two pflag v1.0.10 sources, two distinct hashes) and kept their module directories and the snapshots. A private manifest lists each path and hash. `gofmt -l .` still fails locally on two private draft test files in our own ROC-S6 snapshots, which do not parse. They are not third-party, so the answer did not cover them (TW-Q1). Gates and CI are unaffected: their clones have no `bin/`. |
| TW-D3 | **Cite refresh is scripted from the SW-D25 inventory and checked per cite.** Each of the 52 new ranges was checked against the inventory's quoted code and the enclosing function before writing, and the reviewer reopened every range. Only numbers change; no prose. `verify_anchored.go` and the six docs had not changed since the inventory. |
| TW-D4 | **The two uncertain cites stay as they are.** PRD-historical-anchor-relanding line 322 and the recipe PRD line 3697 describe behaviour that has since changed, so new numbers alone would endorse stale claims. The wave-2 cite guard needs a live-versus-historical marker anyway; these two are resolved there, wording and numbers together. |
| TW-D5 | **CHANGELOG date 2026-11-13**, the expected UTC date of the tag; earlier headings use the UTC release date. If the tag slips to another UTC day, the heading moves with it before tagging. |
| TW-D6 | **ADR-013 and PRD-historical-anchor-relanding status lines** that say N2 "is not yet released" are updated after the tag, as `881b383` did for N1, so gated docs never claim a release that has not happened. |
| TW-D7 | **The candidate was pushed before its validation-machine doc checks finished** (20:50Z; they passed at 20:54:16Z), to start CI early. The push cancelled CI 43301398983 for the tracking-only dispatch commit `64ccb19`; 43304207390 covers the candidate. |
| TW-D8 | **GH #17: all 13 unchecked `report["abort"]` casts are checked**, not only AVP-192's: any one of them panics and aborts the whole Windows `internal/cli` test binary. |
| TW-D9 | **Windows full-suite step `-timeout` 20m → 40m**, with no job `timeout-minutes` (GitHub's default is 360). The "timeouts-swapped" guard fixture was replaced because it means nothing once both are 40m. The Windows ROCS8 command stays at 20m. |
| TW-D10 | **GH #17 review note 1 fixed in `dc76fa2`** (remaining AVP decoded-JSON casts, explicit `float64` compares). Notes 2–3 (casts in the S6 archive-fixture, reject and verify-landed tests) are outside AVP and panic only on malformed input; left for the GH #17 inventory (TW-D25). |
| TW-D11 | **The cite guard is a ledger in test code, not inline doc markers**, which would churn about 100 lines of six accepted docs. It scans `docs/**.md` except the dated `handoff/` and `supervisor/` records, plus top-level `*.md`. It found two stale continuation cites the refresh missed; TW-D4's two uncertain cites are classified historical and pinned (`2e44569`, `e76e0f7`). |
| TW-D12 | **Wave-2 tracking edits waited for the rebase onto the post-tag `main`**, to avoid CURRENT.md conflicts; a private file held them meanwhile. |
| TW-D13 | **A hosted-runner miss is not a code failure.** CI 43304207390's `test (macos-latest)` was cancelled because GitHub never assigned it a runner (macOS arm64 capacity notice) and it ran no step. Only that job was re-run; it passed. The release still required all five jobs green. |
| TW-D14 | **Gate 1 started while CI was still running** (21:11Z), not after CI green: the gate is independent of hosted CI and the validation machine was otherwise idle. The release required both the gate PASS and all five CI jobs green; both held. |
| TW-D15 | **A private probe sized the ROCS8 fixture** before any code: 2.2–3.9 s to build versus about 65 ms to copy and refresh. The fixture's linked worktree stores the template's absolute path in two link files, so a plain copy would point back at the template. |
| TW-D16 | **The ROCS8 template saves about 220 s per gate, not SW-D14's 450–550 s.** A fixture-only row costs about 3.7 s; the remaining ~6 s of an acceptance row is acceptance work. Implemented anyway, per TW-A3. The copier rewrites both link files itself in Git's exact form and fails closed on any other form, rather than calling `git worktree repair`, because Apple Git 2.39's repair can follow a stale link and rewrite the template's own registry. |
| TW-D17 | **Windows keeps fresh fixture builds.** Its native S8 step uses 309 s of 20 m, and the copier is unverified against Git for Windows path spellings; the two template guard tests skip there. |
| TW-D18 | **The remaining ~6 s per ROCS8 acceptance row is not changed.** It is likely macOS `F_FULLFSYNC` durability syncs, which are the product behaviour under test; recorded as a measure-first candidate. |
| TW-D19 | **The template lives in `os.MkdirTemp` and `TestMain` removes it.** A killed or timed-out test binary leaks it in `TMPDIR`, as `t.TempDir()` would. Accepted. |
| TW-D20 | **ROCS8 review notes.** A user's `core.splitIndex` rewrites `sharedindex.*` files, so the copy check skips them and compares staged entries instead; Git is not pinned process-wide, because that would change Git for every workflow test. A user's `core.untrackedCache` would embed the build root, which the leak scan rejects (fails closed, accepted). The stale-index mutant's proof prints the machine's stat settings. |
| TW-D21 | **Cite ledger rev-1 fix: carry the cited path within a block** (a blank line resets it) instead of editing two accepted docs to spell the path. A simulation over the same docs showed the rule changes exactly the two missed cites. |
| TW-D22 | **Measurement never delays a gate.** Gate 1 ran about 20 minutes slower than `a37e512`'s, so the S6 shard measurement moved after gate 2 and the ROCS8 and cite-ledger validations ran first. |
| TW-D23 | **Release and rebase bookkeeping.** v0.22.0's tracking commit `d699713` passed `make release-check` 9/9 and was tagged and pushed at 00:40Z. `tw-wave-2` was rebased onto it cleanly; the AC-L135 totality guard was simulated over TW-D6's changed lines (no forbidden phrase before or after) and no `verify_anchored.go` cite changed. |
| TW-D24 | **GH #15 post-release label drift is recorded, not fixed.** `SPEC.md`'s recipe-authority heading and nine user docs still say "v0.17 planned, unreleased", but v0.17.0 shipped it. The heading is a link anchor and parity tests pin Unreleased-heading shapes, so the relabel needs its own reviewed change; ROADMAP backlog row "GH #15 post-release label drift". |
| TW-D25 | **GH #17 inventory (static).** Ten single-value casts on decoded JSON remain in `internal/cli` tests, all outside AVP: the S6 archive-fixture wire mutation (`prepare_s6_contract_test.go`, 7 casts on 6 lines), three in `reject_test.go` and one in `verify_landed_cli_test.go`. They panic only on a reshaped payload. Four more load values the test itself stored in a `sync.Map` and cannot fail. Not changed this window. |
| TW-D26 | **A validation-harness bug is not a product failure.** w2c's template-off switch left the `runtime` import unused, so its baseline build failed after the template-on package run had passed. A resume script switched the template off with an always-false `runtime.GOOS == ""`, proved the switch took effect (the template guard test skips), and ran the baseline and mutants. No code change. |
| TW-D27 | **The ROCS8 template's measured saving is about 245 s per gate** on the validation machine: the 40 ROCS8 tests shared by both runs take 731 s with fresh builds and 460 s with the template (272 s), less 26.5 s for the two new guard tests. One run each, so the number is approximate. The largest single gains are the refusal and rollback rows (117 → 39 s, 134 → 91 s). |
| TW-D28 | **`c92d371` was pushed while `d699713`'s `main` CI was still in its macOS test step** (60 minutes in, against a usual 38), because waiting would have pushed gate 2 and the close too near the deadline. The push cancelled 43338952877 as expected; tag run 43338954795 tests the same commit `d699713`, which is tracking-only on top of the gated `3239556` (precedent TW-D7). |
| TW-D29 | **CI red on `c92d371`: fix forward, do not restore the frozen file.** The only failure was PIB-391's pin on `internal/intent/avp_guards_test.go`, whose five Windows allowed-failure mutation arms `6953fef` moved from `-timeout 20m` to 40m with the workflow step (TW-D9). Gate 2 was stopped, since it would fail on the same test. `420c485` moves only `CurrentSHA256` (precedents `871d3ab`, `0dd96e1`, `676b9ba`). Rejected: restoring the file byte-identical as on 2026-08-22, which would need the Windows step back at 20m because each arm's literal must match the workflow text to be detected; moving the arms elsewhere also changes the frozen file. Cause: W2A ran only the AVP and intent tests. From now on, a change to any file in `acceptedCheckSourceProvenance` marked `FreezeCurrent` must run `TestPreparePIBPreChangeGoldens` in its targeted validation. |
| TW-D30 | **A validation-harness bug is not a product failure (again).** W2F's mutation step filtered to the PIB-391 subtest, which skips the 50-fixture capture its parent test requires, so the mutant failed for the wrong reason and the script failed closed. W2F2 reran the mutation against the whole parent test. |

### Questions for the operator

| ID | Question |
|---|---|
| TW-Q1 | Two private draft test files under the ignored `bin/roc-s6-20261031/` snapshots do not parse, so local `gofmt -l .` still fails. Delete them, rename them to a non-`.go` suffix, or keep them? Nothing has been changed. |

## Current State

v0.22.0 is released (tag on `d699713`, which records the gate PASS at
the gated `3239556`); it ships GH #26 N2 and the doc-cite refresh. Wave
2 is reviewed and validated; its first close commit `c92d371` failed CI
on one frozen-source pin, fixed in `420c485` (TW-D29). This close
commit is pushed to `main` and under serial gate 3. The S6 shard
measurement runs after gate 3.

## Files Changed

- `f66523a`: `docs/adrs/ADR-013-verify-freshness-overlay.md`,
  `docs/adrs/ADR-036-recipe-coverage-authority.md`,
  `docs/prds/PRD-verify-freshness.md`,
  `docs/prds/PRD-historical-anchor-relanding.md`,
  `docs/prds/PRD-overlap-verification-consolidation.md`,
  `docs/prds/PRD-recipe-generation-authority.md` (cite numbers only).
- Release candidate: `CHANGELOG.md`,
  `assets/recipe_authority_parity_test.go`, `docs/handoff/CURRENT.md`,
  `docs/ROADMAP.md`, `docs/supervisor/LOG.md`.
- Wave 2: `.github/workflows/ci.yml`,
  `internal/cli/prepare_avp_test.go`, `internal/cli/prepare_avp2_test.go`,
  `internal/cli/prepare_s7_ci_timeout_guard_test.go`,
  `internal/intent/avp_guards_test.go` (GH #17);
  `internal/workflow/verify_anchored_cites_test.go`,
  `docs/prds/PRD-historical-anchor-relanding.md`,
  `docs/prds/PRD-recipe-generation-authority.md` (cite guard; the two
  docs gain historical notes); `internal/workflow/main_test.go`,
  `internal/workflow/operation_candidate_template_test.go`,
  `internal/workflow/operation_candidate_acceptance_test.go` (ROCS8);
  `docs/adrs/ADR-013-verify-freshness-overlay.md` and
  `docs/prds/PRD-historical-anchor-relanding.md` (TW-D6);
  `internal/cli/prepare_pib_golden_test.go` (`420c485`, PIB-391 pin);
  tracking: `docs/handoff/CURRENT.md`, `docs/ROADMAP.md`,
  `docs/supervisor/LOG.md`.

## Test Results

- Baseline: CI 43130446490 (`dd4c082`) green.
- `f66523a` on the validation machine (clean clone, 60-second admission):
  `gofmt -l .` clean; `internal/workflow` doc guards (including the S3
  fingerprint parity and `TestAcceptanceLedger_VerifyContractAnchorsResolve`)
  ok; `./assets` ok; `internal/cli` doc guards ok (274 s);
  `go vet ./...` and `go build ./cmd/tpatch` ok. **DOC CHECKS PASS**
  (2026-11-12T20:43:53Z).
- Release candidate `3239556`: validation-machine doc checks **PASS**
  (2026-11-12T20:54:16Z). CI 43304207390: S7 observers (ubuntu, macOS),
  test ubuntu and test windows passed on the first attempt; test macOS
  passed on the re-run (TW-D13). Serial validation-machine gate
  (`WAVE_BASE=dd4c082`): **AUTO GATE: PASS (serial)** at
  2026-11-13T00:35:46Z — 42/42 top-level commands, exact command-multiset
  and pre-flight order parity, fresh isolated caches, source clean
  afterwards, no concurrent test time, `wave-close-check` 8/8,
  12,281 s.
- Wave 2 on the validation machine (clean clone at the reviewed
  pre-rebase commits, whose source the rebase left byte-identical: it
  changed only the three tracking files; 60-second admission,
  `GOMAXPROCS=1`, `-p=1`):
  - **W2A** (GH #17 prerequisites, at the reviewed `dc76fa2` content):
    vet, `internal/intent` and the targeted `internal/cli` AVP tests
    pass; the abort-cast mutant is killed
    (`TestReportAbortFailsInsteadOfPanicking`). PASS.
  - **W2B** (cite ledger rev-1): workflow, assets and cli doc checks,
    vet and build pass; 4/4 mutants killed. PASS.
  - **W2E** (cite ledger rev-2): gofmt, vet and guard tests (13
    sensitivity subtests) PASS; 9/9 mutants killed with their expected
    messages (source shift, doc renumber, continuation parser, historical
    row, unlisted table row, per-line ledger, per-line sensitivity, no
    blank-line reset, anchor-and-pin). **W2E PASS** 01:03:07Z.
  - **W2C/W2C2** (ROCS8 template): gofmt, vet, both template guards
    (creation-false 18.78 s, creation-true 14.86 s) and the full
    `internal/workflow` package with the template on (1,291 s wall, 0
    failures; Git stat settings at defaults). The template-off baseline
    (w2c2 after a harness fix, TW-D26) confirmed the switch took effect
    and ran the 40 shared ROCS8 tests in 731 s with fresh builds versus
    460 s with the template: about 245 s saved per gate after the two
    new guard tests (26.5 s); one run each (TW-D27). 5/5 mutants killed
    with their expected messages (no relink, shared slices, file times,
    no index refresh, no relink with the leak scan off). **W2C2 PASS**
    01:19:12Z.
  - TW-D6 (`ed5c232`, docs only) is covered by the gate at the close
    commit.
- First wave-2 close commit `c92d371`: CI 43346676120 failed
  `test (ubuntu-latest)` and `test (macos-latest)` on one test,
  `TestPreparePIBPreChangeGoldens/PIB-391-routing-provenance-and-immutability`
  (frozen `internal/intent/avp_guards_test.go` hash); every other
  package in that first command passed on both, including
  `internal/workflow` with the ROCS8 template (macOS 454 s). The later
  shard commands did not run (`set -e`). Gate 2 was stopped (TW-D29).
- `420c485` on the validation machine (60-second admission,
  `GOMAXPROCS=1`, `-p=1`): **W2F** gofmt, `go vet ./internal/cli`,
  `TestPreparePIBPreChangeGoldens`, `TestPreparePIBGoldenProvenance`
  and `TestS6GoldenWiringSensitivities` (39.6 s) and the S6
  `PIB-391-frozen-golden-provenance` guard pass; its mutation step had a
  harness bug (TW-D30). **W2F2**: appending one line to the frozen file
  makes `TestPreparePIBPreChangeGoldens` fail on exactly the
  frozen-source hash; source clean afterwards. PASS.

## Next Steps

1. Verify the v0.22.0 GitHub Release (tag CI 43338954795's `release`
   job).
2. Gate 3: serial validation-machine gate at this close commit
   (`WAVE_BASE=dd4c082`) and its CI; record both in the LOG.
3. Wave 2 CI: read the Windows full-suite result for the GH #17
   inventory (TW-D25).
4. After gate 3: the S6 shard-merge measurement (measure only).
5. Close: archive this handoff to HISTORY byte-for-byte; IDLE.

## Blockers

- None.

## Context for Next Agent

- **Old Mac:** reach it as in the archived SW handoff (host key pinned
  with `HostKeyAlias`; any LAN address stays in private notes). Its
  private runner is serial by default until the operator reports the
  cleanup.
- **Validation root:** `R=/Users/jbencardino/tpatch-validation-s5-fBIN5H09`.
- **Gates:** run the gate at the close commit, whose canonical field is
  terminal (HK-D9). Do not push to `main` while the gated commit's CI is
  running.
- **Release guards:** `assets/recipe_authority_parity_test.go:16` pins
  `0.22.0` at the release candidate, which moved the pin with the
  CHANGELOG heading in gated source. After the gate passes, a
  tracking-only commit records the verdict and gated SHA in the LOG
  (release-check `[8/9]`), then `make release-check` runs before
  `git tag -a`.
- **Reviewers:** OpenAI/MAI models only (gpt-6.1-sol); reviewers never run Go.
- **Frozen check sources:** `acceptedCheckSourceProvenance` in
  `internal/cli/prepare_pib_golden_test.go` pins the current bytes of
  every entry marked `FreezeCurrent`. A change to such a file must move
  its `CurrentSHA256` in the same reviewed change, and its targeted
  validation must run `TestPreparePIBPreChangeGoldens` as a whole (a
  subtest filter skips the fixture capture; TW-D29, TW-D30).
- **Private evidence:** `bin/tw-20261112/` (ignored); SW evidence in
  `bin/sw-20261111/`.
