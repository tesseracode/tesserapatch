# Current Handoff

## Active Task

- **Task ID**: _idle — M15 Wave 3 design APPROVED, awaiting Slice A code dispatch_
- **Milestone**: M15 → Wave 3 (lifecycle / reconcile semantics tranche)
- **Status**: Idle
- **Assigned**: 2026-04-27

## Session Summary

M15-W3-DESIGN approved after one revision cycle. PRD + ADR-012 locked; archived to `docs/handoff/HISTORY.md` (top entry, 2026-04-27).

The design covers `feat-verify-command` + `feat-feature-tested-state` in a single combined PRD because the two share contract surface — most notably D2 (does `tested` satisfy hard dependencies?), which is now locked: **yes, `tested` is a strict superset of `applied`**.

The PRD slices the work into four independently-dispatchable code waves (Slice A: verify command shell; Slice B: tested state plumbing; Slice C: verify produces tested; Slice D: --all / JSON / docs). Slice A is the next dispatch.

## Locked design contract (binding for all Wave 3 code dispatches)

- **D1**: `tested` is a linear forward state from `applied`. Single-direction extension to `FeatureState` enum.
- **D2**: `tested` satisfies the hard-dep gate. Implementation is one switch arm: extend `case StateApplied:` in `internal/workflow/dependency_gate.go:79–101` to also match `StateTested`.
- **D3**: `verify` is the sole producer of `tested` in v0.6.2. `test` is unchanged; `amend` does not produce `tested`.
- **D4**: New `Tested *TestedRecord` field on the feature status block carries `omitempty` so v0.6.1 repos round-trip byte-identical until verify is run.
- **D5**: Transitions: `applied + verify PASS → tested`; `tested + verify PASS → tested` (idempotent); `tested + verify FAIL (block-severity) → applied`; `tested + amend (recipe-touching) → applied`; `tested + amend (intent-only) → tested` (preserved). Demotion does NOT cascade to children.
- **D6**: `tested` lives in `status.json`. Never inferred from `artifacts/reconcile-session.json`. Reuses ADR-011 D6 source-truth guard verbatim.
- **D7**: `verify` is read-only on the working tree. Apply-simulation uses the existing shadow workspace plumbing.

## Reviewer adjudications (binding inputs to Slice A's contract)

- **Q1 (V9 severity)**: warn (default).
- **Q2 (`verify --all` skip)**: pre-apply slugs are skipped with a `"skipped: pre-apply state"` reason line in the JSON output, not a failure.
- **Q3 (`passed` field name)**: retained. `severity` carries gating; `passed` carries pass/fail intent.
- **Q4 (D2 wording)**: resolved by `e6473ea` revision pass.
- **Q5 (parent-state hook)**: inserted into the existing M14.3 label-recomputation loop. No new hot path.

## Files Changed

_No active task; nothing pending._

Last work: see `docs/handoff/HISTORY.md` 2026-04-27 entry for the full design dispatch + revision archive (commits `fdc6e70`, `90375c9`, `e6473ea`).

## Test Results

N/A — design-only phase. The next code dispatch (Slice A) will run the standard `go test ./... && go build ./cmd/tpatch && gofmt -l .` gate.

## Next Steps

1. **Refresh backlog mirror** to reflect Slice A as the next active code item:
   ```
   chmod 644 .tpatch-backlog/backlog.db
   sqlite3 $SESSION_DB ".backup '.tpatch-backlog/backlog.db'"
   chmod 444 .tpatch-backlog/backlog.db
   ```
2. **Dispatch `m15-w3-slice-a-implementer`** (general-purpose, background) with a tight per-slice contract:
   - **Scope**: verify command shell — register `tpatch verify <slug>` cobra command + `--json`, `--all`, `--shadow` flags + skeleton check runner that returns the new `VerifyReport` struct shape from PRD §4.2. Implement V0–V2 (cheap structural checks: spec.md present, exploration.md targets exist, recipe parses). Stub V3–V9 with TODO + clean-up sentinel.
   - **Out of scope for Slice A**: the actual `tested` state plumbing (Slice B), recipe re-apply against shadow (Slice C), `--all` orchestration (Slice D).
   - **Constraints**: PRD §4.2 JSON shape is binding; cobra wiring follows the existing `applyCmd` / `recordCmd` pattern; skill anchors must be regenerated to mention `verify` (parity guard will fail otherwise).
3. **Wait for completion**, dispatch `m15-w3-slice-a-reviewer` (`code-review` agent), then user gate before Slice B.

## Blockers

None.

## Context for Next Agent

- v0.6.1 is shipped on `origin/main` (tag `v0.6.1`, commit `572a038`). Wave 3 design commits (`fdc6e70`, `90375c9`, `e6473ea`) are committed locally and pushed. The current `main` HEAD is the supervisor approval of the revision pass.
- Authoritative design surface: `docs/prds/PRD-verify-and-tested-state.md` and `docs/adrs/ADR-012-feature-tested-state.md`. Read both before dispatching Slice A. Supplement with `docs/handoff/HISTORY.md` 2026-04-27 entry for the why-this-was-locked-this-way context and reviewer adjudications.
- Hard rules that still hold: ADR-010 D5 (source-truth guard), ADR-011 D6 (status-as-truth), recipe-op JSON schema frozen (no `delete-file` op), `omitempty` round-trip invariant, secret-by-reference, no nested map keys in YAML config (per stored memory).
- The `tpatch` root binary is not gitignored; `rm -f tpatch` after any local `go build`.
- Sub-agent self-reviews remain status-only signals. Always run an external review before approving anything non-trivial.
