# ADR-021 — `tpatch land` carve-out for operator-drifted global metadata

**Status**: Accepted
**Date**: 2026-05-13
**Source PRD**: [PRD-tpatch-land.md](../prds/PRD-tpatch-land.md) §3.3 step 3, §3.6, §6 ac.6

## Context

`tpatch land <slug>` composes (record → safe path-set staging → one Git
commit). The original PRD required a strictly clean working tree as a
post-condition (`git status --porcelain` empty).

Wave C rev-1 shipped that strict semantics. The external supervisor's
rev-1 review surfaced **Finding 1**: the path-set builder was sweeping
`.tpatch/upstream.lock` and `.tpatch/FEATURES.md` into the feature
commit whenever they were merely *dirty* in the working tree, regardless
of whether the embedded `record` step had actually modified them. That
silently absorbed unrelated operator drift into the feature commit —
the WP-001 §5.2 row 5 boundary-capture failure moved one step
downstream into Git history.

Wave C rev-2 fixed the over-staging by snapshot/diff: the on-disk
content of both globals is sha256'd before `embedRecord`, re-snapshot'd
after, and only the changed-set is included in the path set. Operator-
drifted globals are filtered out of the dirty/extras set (so they do
not trigger the extras refusal AND do not get staged with
`--allow-extra-paths`), and a one-line stderr `note:` is emitted per
file. The file is left dirty in the working tree.

The rev-2 review surfaced a contract gap: the rev-2 *code* is
note-and-continue, but the PRD still promised a strict clean-tree
post-condition. That promise is incompatible with the rev-2 behavior in
shared / non-ephemeral worktrees, where these two global files
routinely drift between lands (operator-managed upstream pinning,
manual edits to `FEATURES.md`).

This ADR records the decision to align the *contract* with the *code*
rather than the other way around.

## Decision drivers

- **Shared-worktree reality** — In non-ephemeral worktrees the two
  global metadata files routinely accumulate operator drift between
  lands. Refusing on every drift is too rigid for the persona ("Maintainer
  Mira" running `land` on her long-lived integration branch).
- **Boundary discipline preserved** — The drift must NOT leak into the
  feature commit. The fix is "leave it dirty," not "absorb it."
- **Audit visibility** — The carve-out must be visible to operators and
  reviewers. A silent "this file might still be dirty" is the wrong
  shape.
- **Bounded scope** — The carve-out applies to exactly two named files.
  No flag widens it. No additional files qualify.
- **No new flag surface** — A `--allow-dirty-globals` flag would only
  exist to *silence* the note, which is the wrong incentive.

## Decision

Adopt **note-and-continue** semantics for operator-drifted
`.tpatch/upstream.lock` and `.tpatch/FEATURES.md`:

1. `land` snapshots both files' content (sha256, `""` sentinel for
   missing) before the embedded `record` step.
2. `land` re-snapshots after `record` and computes a `metaChanged` set.
3. For each of the two named files:
   - If `record` changed the file → include in the feature commit
     (existing rev-2 behavior).
   - If the file is dirty in the working tree but `record` did not
     change it → emit `note: leaving <path> dirty (operator drift
     outside feature scope; not staged)` to stderr and **leave the
     file dirty in the working tree**. Do not stage it, even with
     `--allow-extra-paths`.
   - If the file is clean → ignore.
4. The PRD's "working tree clean" post-condition is qualified to
   "working tree clean **with respect to feature scope**", with the
   carve-out documented in PRD §3.3 step 3, §3.6, and §6 ac.6.

The note message string is fixed and pinned by
`TestLand_DoesNotStageUnrelatedDirtyMetadata` so future refactors
cannot quietly drop it.

## Considered alternatives

**A. Strict refuse — refuse `land` whenever either global is dirty
without a `record`-driven justification.** Rejected. The only escape
hatch would be `--allow-extra-paths`, which re-introduces the rev-1
F1 failure: the operator habituates to passing the flag, and the next
unrelated drift gets absorbed into the feature commit. This is the
opposite of what the carve-out is meant to protect.

**C. New `--allow-dirty-globals` flag — keep the strict refuse but
add an opt-in flag that suppresses the refusal *and* the note.**
Rejected. The only purpose such a flag could serve is to silence the
visibility note. That is the wrong incentive: any operator who
habitually passes `--allow-dirty-globals` is exactly the operator who
needs to *see* the drift, not hide it. The note is cheap (one line per
file, only when drift exists) and the only sensible behavior.

## Consequences

- The "working tree clean" post-condition is qualified for two named
  files. PRD §3.3 step 3, §3.6, and §6 ac.6 are amended; §1 summary
  and §3.5 dry-run sample call out the carve-out.
- `land` behavioral code is unchanged from rev-2 except for tightening
  the note message string to the canonical wording recorded above.
- The carve-out is bounded and auditable: two named files, mandatory
  stderr note per drifted file, no flag to expand the scope, regression
  test pins the note wording.
- `docs/land.md` documents the carve-out as an exception (a documented
  reality of shared worktrees), not a feature operators should rely on.

## References

- `docs/prds/PRD-tpatch-land.md` §3.3 step 3, §3.6, §6 ac.6 (amended
  in this rev to match this decision).
- `docs/supervisor/LOG.md` — Wave C rev-2 sub-agent verdict (APPROVED,
  commit `c6f4402`) and the external rev-2 NEEDS REVISION verdict
  (commit `f98a789`) that surfaced the contract gap.
- `internal/cli/land.go` `runLand` lines 122 / 144 / 170 / 184-191
  (snapshot/diff + filter + note emission).
- `internal/cli/land_test.go` `TestLand_DoesNotStageUnrelatedDirtyMetadata`
  (pins the note wording and the leave-it-dirty behavior).
- `docs/whitepapers/WP-001-feature-slice-gap.md` §5.2 row 5 — the
  boundary-capture failure mode the carve-out is designed not to
  re-introduce.
