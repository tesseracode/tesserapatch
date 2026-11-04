# Current Handoff

## Status

**Cluster state**: SHIPPED

**v0.18.0 is published; GH #13's external issue closure is BLOCKED.**
All S0-S9 implementation, independent review and required release acceptance
are complete. GitHub denied both `addComment` and `closeIssue` under its
managed-account policy. GH #13 remains OPEN; no release comment was posted.
Do not confuse the shipped implementation with completion of that external
action, and do not bypass the restriction.

**GH #9 is deferred, not started.** GH #17 remains separate known Windows
full-suite debt. No new source work is active.

## Active Task

- **Task ID**: `release-operation-replay-candidate-v0-18-0`
- **Milestone**: GH #13 / v0.18.0
- **Description**: External issue closure for the already published and accepted S0-S9 release
- **Status**: BLOCKED - an appropriately authorized GitHub operator must close GH #13
- **Assigned**: 2026-11-04
- **Validated source**: `5133c950161efd6d4ba2ace1f51fb78f8d54832e`
- **Release commit**: `a14de672141dd243729805d6442a0577bea1383b`
- **WAVE_BASE**: `faeedd38a7b1056945864b1964bc55632eae7416`
- **Original authorization window**: `2026-11-04T06:20:23Z` through `2026-11-04T16:20:23Z`

WAVE_BASE = faeedd38a7b1056945864b1964bc55632eae7416

Do not restart or silently extend the overnight window. A later GH #9 cycle
needs new authorization, a fresh wave base and its own complete acceptance.

## Session Summary

S9 completed exact-byte public-contract parity, all six skill formats,
real adjacent-argument downstream soaks and executable candidate guidance.
Native CI exposed a coupled AP prose-hash dependency missed by the first
focused selector. The bounded correction added a separately pinned S9
projection over unchanged historical oracles, preserved the provenance closed
set and added rejecting controls. Expanded RGA/AP/observer/provenance checks,
independent source review and fresh full acceptance passed.

Annotated v0.18.0 was created before the atomic main/tag push. Its
tracking-only consolidation preserves all 19 reviewed source hashes.
Both main/tag CI passed; actual tagged Linux/macOS installs print exactly
`tpatch v0.18.0`. The existing workflow automatically published the non-draft,
non-prerelease source-only release at **2026-11-04T13:50:13Z**.
No packaging pipeline, binary assets or manual early publication was added.

Publication was independently accepted at **13:54:47Z**. The optional issue
comment was then denied, followed by the separately authorized state-only
close through the same unchanged client/account. Read-only verification at
**13:59:59Z** confirmed no issue state/comment change. No account, credential
or permission change, alternate write API or identity was used.

### Decisions and exact archives

The full decisions **S9-D1 through S9-D15**, failure/revision evidence,
review scoreboard and closeout decisions are preserved in
`docs/handoff/HISTORY.md` and `docs/supervisor/LOG.md`.

| Archive | Terminal body |
|---|---|
| `2026-11-04T12:32:43Z - GH #13 S9 implementation accepted` | 35,869 bytes; SHA256 `2506e683d6c96c17282d74e98679ab65c2c2a6440e5ff037dbb00988e3130b16` |
| `2026-11-04T14:02:20Z - v0.18.0 published; GH #13 external closure blocked` | 14,343 bytes; SHA256 `2cf81decbf9804fd12061f34ba6b3d19fe1466d2f4ed8606fa4ab72fbdf03afd` |

Both copies are byte-for-byte, with each prior HISTORY prefix unchanged.
HISTORY is now 1,155,403 bytes, SHA256
`7a450ad3a3e6a19916b57abf2ac039bc4129ec58748225f9c0c3edd01dfdef20`.
Do not archive S8, accepted S9 or the release handoff again.

Key closeout decisions: respect both GitHub write denials and hand off the
external action; defer GH #9 rather than weaken checks. At the budget
assessment, only **2h20m24s** remained, shorter than the **3h38m1s** fresh
gate alone, and GH #13's closure prerequisite was unmet.

## Current State

- [v0.18.0 release](https://github.com/tesseracode/tesserapatch/releases/tag/v0.18.0):
  published, release ID **433986008**.
- Release tag object: `648afeeee5fcacffdf871177ad022406074b51c3`, peeled to
  `a14de672141dd243729805d6442a0577bea1383b`. Do not rewrite it.
- GH #13: OPEN, external closure blocked by GitHub account policy.
- GH #9: OPEN, deferred without implementation.
- GH #17: OPEN; blocking Windows lanes pass, but its full suite is not green.

The final tracking-only commits change no reviewed source. Their normal main
CI outcome is bound to the corresponding GitHub Actions run and private
closeout audit, rather than another self-referential tracking commit.
Authoritative source/tag acceptance is recorded below.

## Files Changed

Final tracking touches only `docs/handoff/CURRENT.md`,
`docs/handoff/HISTORY.md`, `docs/ROADMAP.md` and `docs/supervisor/LOG.md`.
The 19 reviewed source files remain frozen under
`bin/roc-s9-20261104-0620/rev1-formatted-sha256.json`.
No dependency, CI, resource-guard, allowlist, frozen fixture, historical
oracle, research or protected tag change was made during release closeout.

## Test Results

- Independent rev-1 source review: APPROVED; 19 final hashes and historical
  boundaries verified.
- Expanded guarded checks: 6/6 including formatting; 45 top-level passes,
  18 required exact markers and 29 new projection sensitivity leaves.
- Fresh complete gate at 5133c95: **42/42 commands, 39/39 uncached test
  commands**, exact parity and mechanical PASS in **3h38m1s**.
- Resource evidence: 6,336 normal-pressure samples, zero new swap-outs,
  minimum admission 61.121s, maximum within-stage gap 2.280s. All 1,346 owned
  identities reconciled; one fixture git-daemon was identity-safely terminated.
  No owned process/runner or blocked marker remained.
- Source CI **41801886005**: all five required jobs/steps passed.
- [Release main CI 41830644336](https://github.com/tesseracode/tesserapatch/actions/runs/41830644336):
  all five required jobs and exact step outcomes independently accepted.
- [Tag/release CI 41830644313](https://github.com/tesseracode/tesserapatch/actions/runs/41830644313):
  all five required jobs plus release passed; both actual tagged Unix installs
  print exactly `tpatch v0.18.0`. Native Go1.26.1 verified in raw logs;
  private old-Mac Go1.26.5 is separate evidence.
- Both release Windows lanes: 26 required exact markers, 52 native top-level
  passes, zero skips; the separate legacy full suite retains 224 known
  failures and unchanged panic. No new/missing fixed failure identities.
  **The full Windows suite remains known-red under GH #17.**

## Next Steps

1. An appropriately authorized operator must close GH #13. Do not retry
   denied writes through another identity/API or change authentication under
   this overnight authorization.
2. Leave GH #9 for a newly authorized complete implementation/review/full-gate
   cycle. It concerns source hidden inside untracked nested Git repositories,
   not upstream-history/rebase research.
3. Keep GH #17 and unrelated migration, coverage expansion and upstream-Git
   research separate. No further source work is active in this handoff.

## Blockers

GitHub returned `Unauthorized: As an Enterprise Managed User, you cannot
access this content` for both `addComment` and `closeIssue`. This is an
external authorization blocker, not a source or release failure. The original
request is not fully complete until the issue's external state is closed.

## Context for Next Agent

Evidence is preserved under `bin/roc-s9-20261104-0620/`, including:
`full-gate-5133c95-final-audit.json`, `v018-release-final-audit.json`,
`v018-issue13-blocked.json`, `issue9-final-budget-decision.json`,
both archive audits, native job metadata and raw logs.
The complete gate archive SHA256 is
`e889e129e53d1ba0879b989ed63a6fd0147b05e4ed211d02f1d6dd486fb87403`.
Do not reuse interrupted rev-0 evidence or restart completed acceptance runs.

Protected v0.17.0 tag object:
`e7c748ada40988fe72f47b638cb37d07e24f0ac3`; peeled commit:
`d1d6c3fff6cdbab4b5bb9f6f7054b2ce9b8667e8`. Both are unchanged.
No new local/old-Mac Go is needed for this external closure. Any future source
change must reopen review and retain the accepted guarded resource protocol.
