# ADR-017 — Reconcile lock-guard semantics and writer-normalization

**Status**: Draft (placeholder)
**Owner**: TBD (assign during implementation)
**Date**: 2026-05-10 (placeholder opened); body to be drafted at implementation time

## Locks in

The lock-state taxonomy (Valid / Empty / Missing / Stale / Skipped), the
`--allow-stale-lock` override semantics, and the mandate that
`updateUpstreamLock()` writer-normalization (separating `remote:` from
`branch:` rather than concatenating a full `<remote>/<branch>` ref) ship with
or before the guard. Read-side legacy tolerance for pre-fix locks is included
so the guard does not regress existing repos. Per `PRD-reconcile-lock-guard
§5.3`, the writer fix bundles into Wave A and is not a standalone task.

## Source PRD

`docs/prds/PRD-reconcile-lock-guard.md` §3.1, §3.2, §5.3.

## Sections to write at implementation time

- Decision drivers
- Decision
- Consequences
- Alternatives considered

## References

- Supervisor acceptance entry: `docs/supervisor/LOG.md` → "Review — v0.7 Cluster PRDs (land + auto-base + collision + lock-guard) — 2026-05-10"
- ADR precedent: ADR-011 (locked feature-dependencies before M14 implementation)
- HIGH bug locus: `internal/workflow/reconcile.go` `updateUpstreamLock()` (lines 595–605 at routing time)
