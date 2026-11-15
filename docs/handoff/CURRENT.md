# Current Handoff

## Status

**Cluster state**: IN PROGRESS

**MW: approved documentation correction and legacy-migration planning.**
The operator accepted the proposed 10-12 hour unattended window. The
supervisor uses 12 hours as the hard ceiling, with no automatic extension.
FW remains accepted at `f61e2b6`; v0.22.0 remains the latest release.

## Active Task

- **Task ID**: MW-1, then MW-2
- **Milestone**: Post-v0.22 documentation and legacy-migration planning
- **Description**: Correct GH #15 shipped-status labels, links and coupled
  parity tests; independently review and serial-gate the correction.
  Prepare and independently review a design proposal for GH #18-20,
  ending with explicit operator questions, not migration implementation.
- **Status**: In Progress - baseline and source inspection
- **Assigned / window start**: 2026-11-15T09:41:12Z
- **Hard deadline**: 2026-11-15T21:41:12Z
- **WAVE_BASE**: `f61e2b68b8b23e43cb1fdfdf24cda95f87588c04`

## Session Summary

- Recorded clean `main`, no local changes, and fetched `origin/main`;
  both point to WAVE_BASE. Final FW CI 43571328967 is completed/success.
- Archived the superseded final FW IDLE handoff byte-for-byte under
  `2026-11-15T09:41:12Z - FW final IDLE handoff superseded by MW`.
  Archived body: 18,244 bytes, SHA256
  `885d507c0196d765e0c4b607768f01b8a964fb75ad3408ab4bbe5fc064ab0266`.
  Prior HISTORY prefix is unchanged.
- **MW-D1:** 12-hour ceiling; stop sooner when scope is complete. No
  release/tag, migration implementation, GH #17 fixes, GH #25 B, machine
  cleanup, Spotlight change, lane retry, or CI-job/service expansion.
- **MW-D2:** the live SPEC recipe-authority heading already says v0.17.0.
  Nine user guides still say unreleased/planned; the FAQ links to the old,
  nonexistent SPEC anchor. Correct current guidance, not frozen planning
  history or deliberately retained pre-release parser fixtures.
- **MW-D3:** validation stays serial under the inherited guarded resource
  protocol. T3 reports no linked environments; a validation-machine link
  card was requested. Do not bypass it with an ad hoc SSH connection or
  represent hosted CI as the required validation-machine gate.

## Current State

No product behavior or schema change is authorized. MW-1 owns the nine
current user guides and `assets/recipe_authority_parity_test.go`; SPEC may
change only if an actual coupled correction is needed. CHANGELOG and its
v0.22.0 release pin remain unchanged.

MW-2 is planning only: re-ground GH #18 cumulative stacks, GH #19 manual
recipe provenance, and GH #20 legacy generation adoption in current code,
accepted ADRs and the SW-D13 evidence. Historical reproduction is not a
fresh run. Do not fabricate timestamps/provenance, assume current-stack
integrity from old-base replay, or bypass ADR-024's backfill prohibition.
Any proposed architecture remains explicitly PROPOSED pending operator
decisions. No existing accepted ADR or frozen parity block is rewritten.

## Files Changed

- Tracking: `docs/handoff/CURRENT.md`, `docs/handoff/HISTORY.md`,
  `docs/supervisor/LOG.md`, `docs/ROADMAP.md`.
- Implementation and design files: none yet.

## Test Results

- No Go commands run in MW yet.
- Baseline hosted CI: 43571328967, success at WAVE_BASE.
- Required after the correction: targeted parity/link checks, formatting,
  vet/build, independent static review, full guarded serial wave-close
  gate (39 commands / 36 test commands), and exact-candidate hosted CI.

## Next Steps

1. Correct current release labels and the FAQ anchor; add sensitivity
   coverage without weakening current-contract or historical-parser checks.
2. Obtain independent static review and address findings. Reviewers do
   not run Go; only one admitted validation process may use the host.
3. Push the reviewed candidate only after prior main CI is complete, then
   run the serial gate and hosted CI. Never claim acceptance without both.
4. Prepare and review the legacy-migration proposal; independent planning
   may proceed during validation, with no shared-file implementation.
5. Archive completed work, update tracking, push and verify final CI before
   the deadline. Leave unresolved operator questions explicit.

## Blockers

- Required validation machine is not linked to this T3 environment.
  Link request is pending. Documentation and planning can proceed.

## Context for Next Agent

- Use a fresh WAVE_BASE for any later wave, not the historical FW base.
- Reviewers: OpenAI/MAI only; inherited selection is gpt-6.1-sol, static
  review only. Every new review/re-check is a fresh delegated review.
- Stage explicit file paths only. Every commit includes the Copilot trailer.
- Keep old releases, `.wave-close-allowlist`, CLI flags, report schemas,
  source behavior, serial/CI partitions and existing safety guards unchanged.
- FreezeCurrent changes require their hash pin and the whole
  `TestPreparePIBPreChangeGoldens` targeted run; avoid those sources here.
- Private prior validation evidence is under `bin/fw-20261114/`; connection
  details and resource telemetry must not enter tracked documents.
