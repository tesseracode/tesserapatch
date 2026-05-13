# ADR-022 — Defer `Config.PatchIDDetectorEnabled` default-on flip past v0.8.x

**Status**: Accepted
**Date**: 2026-05-14
**Source PRD**: [PRD-patch-already-upstream-detector.md](../prds/PRD-patch-already-upstream-detector.md) §3.1, §4
**Related ADRs**: ADR-011 (cascade rules referenced by `--auto-drop-merged`),
ADR-023 (hotfix-kind auto-drop deferral — depends on this decision)

## Context

The Wave D phase 1.5 patch-id detector shipped in v0.8.0 (M17 cluster). It
inserts a deterministic `git patch-id` sweep between phase 1 (reverse-apply)
and phase 2 (operation-level) of `tpatch reconcile`. When the sweep finds
that our `post-apply.patch` matches a commit in
`<upstream.lock.commit>..<upstream-ref>`, it short-circuits the rest of
reconcile with `Outcome=ReconcileUpstreamed`.

The detector ships **off by default** behind `Config.PatchIDDetectorEnabled`
(defaults to `false`, persisted as `patch_id_detector_enabled` in
`config.yaml`). Users who want the behavior must opt in explicitly. This is a
conservative ship: with the flag off, `reconcile-session.json` and all
existing artifacts round-trip byte-identical to v0.7.x.

v0.8.1 adds two CLI surfaces that consume the detector (`--check-applied-only`
and `--auto-drop-merged`, per PRD §3.2 and §3.3). The natural follow-up
question is whether the detector itself should now flip default-on so the new
flags work without each user editing `config.yaml` first. This ADR records
the decision to **defer** that flip.

## Decision drivers

- **Failure mode is silent and destructive** — A false-positive patch-id
  match short-circuits reconcile with `ReconcileUpstreamed`. If the user is
  also opted into `--auto-drop-merged`, the feature is removed from the DAG
  with cascade. A bad detector verdict therefore deletes work. We need
  evidence the false-positive rate is low enough to default-on, not just an
  intuition.
- **No telemetry yet** — v0.8.0 ships the detector as opt-in with
  `Config.PatchIDScanLimit` defaulting to 5000. We have no production data
  on (a) how often opt-in users see a phase-1.5 match, (b) how often the
  match is actually correct, (c) what scan-limit values get hit on
  long-lived feature branches against fast-moving upstreams, or (d) the
  cost of the sweep on real repos.
- **`git patch-id` is exact-bytes-after-normalization, not semantic** — Two
  unrelated commits that happen to produce the same normalized hunk text
  collide. This is rare in practice but rises with very small patches
  (single-line CVE fixes especially) and with auto-generated code. The
  v0.7.x risk-modeling work flagged this as the dominant tail risk.
- **Conservative-default precedent** — `PRD-tpatch-hotfix` §5.4 already
  deferred its own `--auto-drop-on-merged` for the same trust-gating
  reason. Flipping the underlying detector default-on while leaving its
  consumers' defaults conservative would invert that precedent without
  added evidence.
- **Reversibility cost** — Flipping a default OFF after users come to rely
  on it being ON is a breaking change. Flipping a default ON later is
  additive. Asymmetric reversibility argues for staying off until evidence
  pulls us off the conservative default, not the reverse.

## Decision

Keep `Config.PatchIDDetectorEnabled` default at `false` for the entire
v0.8.x line. The flip to `true` is eligible no earlier than v0.9.0, and
only after the criteria below are met. No code change ships under this
ADR — this is a "decision to not change the default yet" record.

The v0.8.1 CLI flags work around the conservative default explicitly:
`--check-applied-only` forces phase 1.5 to run for that single invocation
regardless of the config flag (see implementation in v0.8.1 items 1+2);
`--auto-drop-merged` is a no-op when the detector is off (it does not
auto-enable). This preserves user agency without burying a default change.

## Re-evaluation criteria (for a future flip-default ADR)

A future ADR may flip the default to `true` only after:

1. **Real-world false-positive evidence**: at least one v0.8.x release cycle
   of opt-in users reporting their phase-1.5 outcomes. Evidence form: issue
   reports / explicit user testimony / a `tpatch reconcile --json` log
   analysis pass. We need to see how often `phase-1.5-patch-id-match` was
   the *wrong* call (e.g., the upstream commit was unrelated and reconcile
   should have continued to phase 2/3).
2. **Scan-limit telemetry**: data on what `PatchIDScanLimit=5000` actually
   costs on real long-lived branches. If most users routinely sweep
   thousands of upstream commits per reconcile, the wall-cost of a
   default-on detector may itself be a regression. Either the data shows
   the cost is fine, or the default flip ships paired with a lower default
   scan limit.
3. **Warn-mode option considered**: the future ADR explicitly considers and
   rejects (or adopts) a "warn-mode" intermediate where the detector runs
   on every reconcile but only logs the would-be verdict to stderr without
   short-circuiting. This is the lowest-risk default-on shape and should be
   on the table.
4. **Hotfix interaction settled**: ADR-023 (hotfix-kind auto-drop deferral)
   either ships its own flip or stays deferred; the detector default and
   the hotfix-default decisions should not be made in isolation, since
   default-on detector + default-on hotfix-auto-drop is the highest-stakes
   combination.

## Considered alternatives

1. **Flip default-on now (v0.8.1)**. Rejected — no evidence base. Users who
   want the behavior already have a clean opt-in path via
   `patch_id_detector_enabled: true` in `config.yaml` (and the v0.8.1
   `--check-applied-only` flag for one-off invocations). The cost of
   leaving the default off is one config-file edit per user; the cost of
   flipping prematurely is potential silent destructive behavior under
   `--auto-drop-merged`.
2. **Default-on with warn-mode** (run detector, log verdict, do NOT
   short-circuit unless `--auto-drop-merged` or another consumer opts
   in). Rejected for v0.8.x as scope creep — this would be a meaningful
   product change requiring its own PRD. The future flip ADR may revisit
   this as the preferred shape.
3. **Never flip; require explicit opt-in forever**. Rejected as too rigid.
   The detector is a real product feature and a perpetual config-flag
   gate is a UX tax once trust is established. The right shape is "off
   until evidence, then on" — this ADR captures the "until evidence"
   half.
4. **Flip default-on but keep `--auto-drop-merged` opt-in**. Plausible
   alternative for the future ADR; explicitly on the table as a
   compromise. Not adopted now because there is still no data on the
   detector's own false-positive rate, and `--check-applied-only`
   benefits from the same evidence base.

## Consequences

- v0.8.1 ships items 1+2 (`--check-applied-only`, `--auto-drop-merged`)
  with the detector still off by default. Users wanting the new flags to
  work without one-shot overrides set `patch_id_detector_enabled: true`
  in `config.yaml`, exactly as they would have under v0.8.0.
- The `--check-applied-only` flag includes a per-invocation override of
  the config flag (forces phase 1.5 for that one call). This is documented
  in the flag help text and in the v0.8.1 CHANGELOG entry. The override is
  scoped intentionally: it applies only to the read-only inspection path,
  never to `--auto-drop-merged`.
- The `Config.PatchIDDetectorEnabled` line in CURRENT.md "frozen-code
  regions" stays. Any change to the default is ADR-gated.
- A future ADR (when the criteria above are met) will flip the default,
  reference this ADR by number, and supersede this decision in
  `docs/adrs/README.md`.

## References

- [PRD-patch-already-upstream-detector.md](../prds/PRD-patch-already-upstream-detector.md) §3.1, §4
- [PRD-tpatch-hotfix.md](../prds/PRD-tpatch-hotfix.md) §5.4 (analogous deferral)
- ADR-011 (`tpatch feature remove` cascade rules — referenced by `--auto-drop-merged`)
- ADR-023 (hotfix-kind auto-drop deferral — sibling decision, depends on this)
- `internal/store/types.go` L342 (`PatchIDDetectorEnabled` field, default `false`)
- `internal/workflow/patch_id_detector.go` (frozen detector implementation)
- `internal/workflow/reconcile.go` ~196-236 (frozen phase 1.5 slot)
