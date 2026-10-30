# Current Handoff

## Status

**Cluster state**: IDLE

GH #13 S1 and the memory-bounded validation maintenance are ACCEPTED.
No validation processes remain active. S2 and the deferred upstream
Git-strategy research have not been dispatched. No release/tag was created.

## Active Task

- **Task ID**: `implement-operation-replay-candidate-s1`
- **Milestone**: GH #13 / ADR-037, ADR-043 and ADR-044
- **Description**: S1 typed roots, caller-held locks and offline Git primitives;
  associated test-only memory-bounded sharding maintenance
- **Status**: Complete — idle pending explicit next assignment
- **Assigned**: 2026-10-29
- **Accepted validation checkpoint**: `24be224e46cb0569de3488bf466467e9c252fd8b`
- **WAVE_BASE**: `5120dc9395769fe8148b61836e546729b66cb4dc`
- **Maintenance base**: `56311c4fa13fa45b28ebb00a86ef3021d70c7b04`
- **Contract**: ADR-037/PRD rev-7, narrowly amended by accepted ADR-044

## Session Summary

S1 implements held reference/candidate roots, isolated candidate worktrees,
offline Git/tree primitives and portable nonblocking advisory locks. ADR-044
makes create/prune borrow caller-owned feature/global lock capabilities.
Borrowed descriptors cannot be released through the original or a copied
Handle. Git subprocesses share a hardened environment, including removal of
inherited `GIT_CONFIG_PARAMETERS`. No S2 eligibility, execution or CLI wiring.

The accepted test-only maintenance replaces 22 test processes with 39:
smaller S6/S7/AVP families retain non-CLI package coverage; AR16/17/18 run
separately; purge fixture loops and AR17/18 use bounded analyzer memo lifetimes.
All assertions, S0 golden bytes and native blocking rows are preserved.
The original twelve-insertion S0 capture exception remains strict; normal S1
validation permits only the nineteen explicitly guarded AR insertions.

The complete handoff and review/measurement chronology are archived in
`docs/handoff/HISTORY.md` under the 2026-10-29 S1 acceptance entry.

## Current State

- S1 implementation and validation maintenance accepted; no open code findings.
- Independent S1 findings closed at `3054b84`; maintenance review approved
  `31fd46e`. All six pilots and full local/native validation pass.
- `OpenReferenceRoot` and `CreateCandidateRoot` remain the two bare-root
  constructors. Create/prune require live feature/global handles, validate and
  borrow them, and never reacquire or release caller-owned locks.
- Caller acquires feature then global, owns the transaction-wide hold, and
  releases global then feature. Root Close owns only filesystem handles.
- v0.17.0 remains the last release; S1 has not introduced public commands.

## Files Changed

S1: `internal/candidate/`, `internal/candidatelock/`, Git/tree environment
helpers in `internal/gitutil/`, focused S0/CLI guards, native CI registration,
accepted ADR-044 and the narrow ADR-037/PRD amendment.

Maintenance: CI, `scripts/wave-close-test-shards.sh`,
`scripts/profile-validation{,-test}.py`, S6 memo helpers, seven additional AR
test lines, S7 partition guards and finite S0/S1 scope guards.
Closure changes only CURRENT/HISTORY/ROADMAP/supervisor LOG.

## Test Results

- **Exact checkpoint**: `24be224e46cb0569de3488bf466467e9c252fd8b`.
- **Local gate**: PASS, 8/8 checks; 42/42 top-level commands including 39/39
  uncached test commands, exact command parity. Formatting, vet and build pass.
- **Native CI**: [40975085201](https://github.com/tesseracode/tesserapatch/actions/runs/40975085201)
  SUCCESS, all five required jobs with no failed steps; release skipped.
  Actual Windows CLW-001 through CLW-005 PASS lines verified.
- **Full-run measurements**: 6,305.16s (about 105 minutes), 6.99 GiB peak
  sampled combined owned RSS, normal pressure throughout, zero new swap-outs.
  Historical occupied swap remains; up to 13 external Go processes observed.
- **Cleanup**: all 584 sampled owned identities gone; tested source clean.
- **Comparison**: previous successful full gate peaked at 10.37 GiB.
  The six pilots peaked at 0.47-1.74 GiB, but are not a full-suite ceiling.
  The largest remaining shard is the AR core registration observer (6.99 GiB).
- **Evidence**: `bin/roc-sharding-20261029/full-gate/evidence/summary.json`,
  adjacent gate/stage logs, pilot directories and native Windows logs.
  Preserve earlier failed runs and all prior experiment directories.

## Next Steps

1. Remain idle until the operator assigns the next task; do not start S2
   or the queued research automatically.
2. A future slice must read the accepted contracts, scope CURRENT explicitly,
   fetch/record its own wave base, and transition the finite S0/S1 scope guard
   deliberately before changing new surfaces.
3. Use the adopted 39-command partition, not historical 22-command recipes.
   `scripts/wave-close-test-shards.sh`, CI and their parity guards are canonical.

## Validation Policy — GH #13

Require 60 continuous seconds of normal OS pressure, no new swap-outs and
load1 <=5 before every Go validation command. External Go is observed, not
vetoed; own work is serial (`GOMAXPROCS=1`, `-p=1`). Percentage is diagnostic;
historical occupied swap alone is not a veto. Stop on warning/critical
pressure, new swap-outs, unreliable telemetry or gaps over 5s. Retain 600s
admission budgets, `-count=1`, 40-minute test-process timeouts, clean isolated
full-gate source/caches and owned-process reconciliation.

## Blockers

None for the completed S1/maintenance work. No subsequent task is assigned.

## Context for Next Agent

S0 goldens, assets, public SPEC, CHANGELOG, dependencies and
`.wave-close-allowlist` are unchanged from S1 wave base. Maintenance changes
no production Go or accepted ADR/PRD bytes. The paired contract parity regions
remain preserved. Do not add an `Unreleased` heading or casually edit the
frozen ADR index.

The v0.17.0 annotated tag object remains
`e7c748ada40988fe72f47b638cb37d07e24f0ac3`, targeting
`d1d6c3fff6cdbab4b5bb9f6f7054b2ce9b8667e8`.

Backlog item `research-upstream-git-strategies` remains deferred. No branching
policy, full research deliverable, Git replacement or separate central patch
store was adopted. The completed archives retain the original research scope.
