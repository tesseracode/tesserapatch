# ADR-016 — `record --auto` baseline-inference algorithm

**Status**: Draft (placeholder)
**Owner**: TBD (assign during implementation)
**Date**: 2026-05-10 (placeholder opened); body to be drafted at implementation time

## Locks in

The `record --auto` baseline-inference algorithm — including the merge-base
strict-refuse-on-N>1 default and the upstream-candidate discovery order — that
underpins remediation for legacy collisions. This is the foundational decision
on which `record collision-detection` recovery hints (ADR-018) and downstream
`tpatch land` semantics (ADR-019) build.

## Source PRD

`docs/prds/PRD-record-auto-base.md` §3.2 (and supporting §3 headers).

## Sections to write at implementation time

- Decision drivers
- Decision
- Consequences
- Alternatives considered

## References

- Supervisor acceptance entry: `docs/supervisor/LOG.md` → "Review — v0.7 Cluster PRDs (land + auto-base + collision + lock-guard) — 2026-05-10"
- ADR precedent: ADR-011 (locked feature-dependencies before M14 implementation)
