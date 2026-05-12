# ADR-019 — `tpatch land` trailer-block schema

**Status**: Accepted
**Owner**: M17 Wave C implementation
**Date**: 2026-05-10 (opened) / accepted at M17 Wave C landing

## Locks in

The four mandatory trailers emitted by `tpatch land` on every commit, in
this exact order, immediately above the repo `Co-authored-by:` line:

1. `Tpatch-Feature: <slug>`
2. `Tpatch-Patch-SHA: <sha256-hex of post-apply.patch>`
3. `Tpatch-Recipe-SHA: <sha256-hex of apply-recipe.json | "none">`
4. `Tpatch-Base-Commit: <full SHA from status.json:apply.base_commit>`

`Tpatch-CVE: <id>` is the only additive trailer permitted (hotfix-kind
commits per `PRD-tpatch-hotfix.md §3.4`); it inserts after `Tpatch-Base-Commit`
and before `Co-authored-by:`. No other trailers may be appended by `land`.

`Tpatch-Feature` is the **sole** feature↔commit binding. The commit's own SHA
is intentionally NOT written back into `status.json:apply.base_commit` —
that field continues to be owned by `record` / auto-base resolution
(PRD `tpatch-land §3.6` F2; PRD `auto-base-resolution`).

## Source PRD

`docs/prds/PRD-tpatch-land.md` §3.4, §3.6 (with §0.1 gating context on Wave A
auto-base + Wave B collision detection).

## Decision drivers

- **Auditability** — A reviewer of any landed commit must be able to,
  in isolation, (a) identify the originating feature, (b) verify the
  patch payload byte-identity, (c) verify the apply-recipe byte-identity,
  and (d) reconstruct the base ref the patch was authored against.
  All four facts must travel inside the commit message because the
  `.tpatch/` artifacts may be rewritten or rebased independently.
- **No chicken-and-egg** — A commit cannot embed its own SHA in tracked
  content. The `Tpatch-Feature` slug is therefore the binding key;
  external tools rebuild the slug→commit map by walking history.
- **Forward-compatibility for hotfix kind** — Reserving the additive
  `Tpatch-CVE` slot now (even though hotfix lands later) prevents a
  schema break when `tpatch hotfix` ships.
- **Tooling parity** — Trailer parsers in DEP-3 (`git interpret-trailers`),
  `git-gud`, and `stk` all key on `Key: Value` lines anchored above the
  trailer block; matching that convention keeps `git log --grep` and
  third-party tools working without bespoke parsing.

## Decision

Adopt the four-trailer block above, in that order, with `Co-authored-by:`
appended last. `Tpatch-CVE` is the only additive trailer reserved.
`apply.base_commit` is never overwritten by `land`.

## Consequences

- `tpatch status` and `tpatch reconcile` can recover the
  `(slug, patch-sha, recipe-sha, base-sha)` quad from any landed commit
  via `git log --format=%(trailers)`.
- A patch payload change between `record` and `land` is detectable
  post-hoc by re-hashing the on-disk `post-apply.patch` and comparing
  to `Tpatch-Patch-SHA`. Same for the recipe.
- Re-landing a feature after `record` re-runs is intentionally
  permitted; the resulting two commits will share the same
  `Tpatch-Feature` slug but diverge on the SHA fields. Tooling that
  consumes the trailers must treat the slug as a *not-unique* key and
  pick the most-recent commit (or the one matching the on-disk
  patch-sha).
- Adding a new mandatory trailer in the future is a breaking change
  to the schema and requires a new ADR.

## Alternatives considered

- **Embed the new commit SHA in `apply.base_commit`** — rejected: causes
  a circular write (commit-then-rewrite-then-commit) and breaks the
  invariant that `record` owns that field (auto-base resolution depends
  on it).
- **Single `Tpatch-Manifest:` JSON-encoded trailer** — rejected: not
  human-readable in `git log`, defeats the audit driver.
- **Compute trailers from artifacts at read-time only (no commit
  trailers)** — rejected: artifacts are mutable on a rebase / re-record;
  the commit message is the durable record.
- **Optional trailers (`Tpatch-Recipe-SHA` only when present)** —
  rejected: variable trailer count complicates parsers; explicit
  `Tpatch-Recipe-SHA: none` is unambiguous.

## References

- PRD: `docs/prds/PRD-tpatch-land.md` §3.4, §3.6, §6 ac.3
- PRD: `docs/prds/PRD-tpatch-hotfix.md` §3.4 (additive `Tpatch-CVE`)
- ADR: `docs/adrs/ADR-016-record-auto-base-resolution.md` (owner of `apply.base_commit`)
- ADR: `docs/adrs/ADR-018-record-collision-detection-signature.md` (Wave B gate)
- Supervisor entry: `docs/supervisor/LOG.md` → "Review — v0.7 Cluster PRDs (land + auto-base + collision + lock-guard) — 2026-05-10"
- ADR precedent: ADR-011 (locked feature-dependencies before M14 implementation)
