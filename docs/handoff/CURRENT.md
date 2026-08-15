# Current Handoff

## Status

**Cluster state**: IDLE

The prepare intent-bundle **planning** wave is closed and accepted.
`PRD-prepare-intent-bundle` **rev-14** and `ADR-035` **rev-14** are both
**Accepted — 2026-08-14** (internal and external review both APPROVED, no
findings). [GH #11](https://github.com/tesseracode/tesserapatch/issues/11) is
closed. Accepted contract: **567** rows `PIB-001`…`PIB-567`, **176** claims
`C1`…`C176`, ADR decisions **D1–D21**. No release tag — the wave shipped
documents only; v0.15.1 remains fixed at `15560af`. The full wave record is
archived in [`HISTORY.md`](./HISTORY.md) under
*2026-08-14 — Prepare intent-bundle PRD + ADR-035 — ACCEPTED*.

**No implementation is authorized for mutating `prepare`.** PRD §19(1) and
§19(2) are satisfied; §19(3) is not.

## Active Task

- **Task ID**: `implement-prepare-check`
- **Description**: Implement the **accepted read-only** artifact-validation and
  provenance contract — `PRD-artifact-validation-and-provenance` rev-5 +
  `ADR-034` rev-2 — i.e. `tpatch prepare <slug> --check`: rooted `os.Root`
  inspection, full three-Markdown bundle readiness, the nine structural states,
  the constant `provenance: unknown`, the `unix || windows` fail-closed
  platform policy, and its 208-row acceptance matrix. No mutation, no provider.
- **Status**: **Not Started — awaiting dispatch**
- **Issue**: none yet — register one at dispatch
- **WAVE_BASE**: none yet — record a fresh `origin/main` SHA immediately before
  the first implementer is dispatched
- **Blocks**: every mutating `prepare` slice (PRD-prepare-intent-bundle §17.2
  S1–S6)

## Current State

- Planning for mutating `prepare` is **complete and accepted**; nothing about
  that contract is open. Do not reopen it — errata aside, rev-14 is frozen.
- The read-only `--check` contract has been accepted since 2026-08-13
  (GH #10) and **has no implementation**. That is the whole of the remaining
  gap.
- No production code, test, asset, `SPEC.md` or `CHANGELOG.md` change has been
  made by either planning wave.
- Shipped surface is unchanged at v0.15.1.

## Files Changed (this close)

- `docs/prds/PRD-prepare-intent-bundle.md` — Accepted rev-14; header, revision
  history, ADR references, §8.4 and the §19 gate
- `docs/adrs/ADR-035-intent-bundle-publication-and-history.md` — Accepted
  rev-14; header, revision history, companion/blocks wording. **No decision
  text changed.**
- `docs/adrs/README.md` — ADR-035 index row → Accepted (2026-08-14)
- `docs/ROADMAP.md` — prepare planning row → ✅ ACCEPTED with final tips/range
- `docs/supervisor/LOG.md` — rev-14 internal + external verdicts and the
  supervisor ACCEPTED decision
- `docs/handoff/HISTORY.md` — full wave archive
- `docs/handoff/CURRENT.md` — this file, replaced with an idle handoff

## Verification

- `make wave-close-check WAVE_BASE=d060ff4` run after commit + push.
- Docs-only change set; no Go source touched, so build/test status is
  inherited unchanged (the gate re-runs `go build` and
  `go test -count=1 ./...` regardless).
- Ledger checks re-run at close: `PIB-001`…`PIB-567` contiguous (567 unique
  IDs), claims `C1`…`C176` contiguous, ADR `D1`…`D21` present, exactly one
  `**Cluster state**:` field.

## Next Steps

1. Register a GitHub issue for `implement-prepare-check` (the read-only
   `prepare --check` implementation).
2. `git fetch origin && git rev-parse origin/main` — record that SHA as the new
   `WAVE_BASE` in the dispatch brief **and in this file** before any implementer
   starts.
3. Declare the file partition. If two implementers would touch the same file,
   run them **sequentially** (AGENTS.md → Parallel-Implementer Discipline);
   stage by explicit path only.
4. Implement `prepare --check` against its 208-row matrix, then run the full
   internal + external review cycle to acceptance.
5. **Only after** that implementation lands on `origin/main` and passes its own
   matrix may the mutating prepare slices S1–S6 be dispatched, with goldens
   taken from that implementation's commit range (PIB-391).

## Blockers

- **None** for `implement-prepare-check` — it is dispatchable now.
- **Hard sequencing block** for every mutating `prepare` slice: it may not
  dispatch until the `prepare --check` implementation is accepted and landed
  (PRD-prepare-intent-bundle §17.1 / §19(3); ADR-035 D14).

## Context for Next Agent

- The mutating-prepare planning documents are long (7.8k-line PRD, 2.0k-line
  ADR). Read the PRD's §17 and §19 and ADR-035's D14 first — they define what
  you may and may not touch.
- `prepare --check` goldens must be produced by the check implementation
  itself. Do not synthesize them from the intent-bundle cluster.
- ADR-034 governs rooted **reads** only; its D14 forbids citing it as a
  persistence precedent. All write-side decisions live in ADR-035.
- Known untracked WIP (whitepapers, `PRD-recurring-patches.md`, case studies)
  is intentionally untracked and covered by `.wave-close-allowlist`; leave it
  alone and do not extend the allowlist without a tracked disposition.
