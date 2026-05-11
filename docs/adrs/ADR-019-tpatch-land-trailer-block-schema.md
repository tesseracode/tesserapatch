# ADR-019 — `tpatch land` trailer-block schema

**Status**: Draft (placeholder)
**Owner**: TBD (assign during implementation)
**Date**: 2026-05-10 (placeholder opened); body to be drafted at implementation time

## Locks in

The four mandatory trailers in `land` commits (`Tpatch-Feature`,
`Tpatch-Patch-SHA`, `Tpatch-Recipe-SHA`, `Tpatch-Base-Commit`) plus the
additive fifth `Tpatch-CVE` for hotfix-kind commits; `Tpatch-Feature` as the
sole feature↔commit binding; and the explicit no-overwrite rule for
`apply.base_commit` by `land`. Repo-wide `Co-authored-by:` follows the block.
Wave C is gated on Wave A + Wave B per `PRD-tpatch-land §0.1`.

## Source PRD

`docs/prds/PRD-tpatch-land.md` §3.4, §3.6 (with §0.1 gating context).

## Sections to write at implementation time

- Decision drivers
- Decision
- Consequences
- Alternatives considered

## References

- Supervisor acceptance entry: `docs/supervisor/LOG.md` → "Review — v0.7 Cluster PRDs (land + auto-base + collision + lock-guard) — 2026-05-10"
- ADR precedent: ADR-011 (locked feature-dependencies before M14 implementation)
