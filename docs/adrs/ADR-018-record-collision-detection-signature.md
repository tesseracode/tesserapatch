# ADR-018 — `record` collision-detection signature scheme

**Status**: Draft (placeholder)
**Owner**: TBD (assign during implementation)
**Date**: 2026-05-10 (placeholder opened); body to be drafted at implementation time

## Locks in

The `record` collision-detection refuse-by-default policy, the
`--allow-collision <reason>` override (with mandatory reason string), and the
choice of byte-identity (not `git patch-id`) as the v1 signature check.
Recovery hints surfaced by detection lean on `record --auto` (ADR-016) being
available, hence the Wave B placement after Wave A.

## Source PRD

`docs/prds/PRD-record-collision-detection.md` §3.1 (and supporting §3 headers).

## Sections to write at implementation time

- Decision drivers
- Decision
- Consequences
- Alternatives considered

## References

- Supervisor acceptance entry: `docs/supervisor/LOG.md` → "Review — v0.7 Cluster PRDs (land + auto-base + collision + lock-guard) — 2026-05-10"
- ADR precedent: ADR-011 (locked feature-dependencies before M14 implementation)
