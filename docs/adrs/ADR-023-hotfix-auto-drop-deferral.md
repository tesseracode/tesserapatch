# ADR-023 — Defer hotfix-kind `--auto-drop-merged` default past v0.8.x

**Status**: Accepted
**Date**: 2026-05-14
**Source PRD**: [PRD-patch-already-upstream-detector.md](../prds/PRD-patch-already-upstream-detector.md) §3.3;
[PRD-tpatch-hotfix.md](../prds/PRD-tpatch-hotfix.md) §5.4
**Related ADRs**: ADR-022 (detector default-on deferral — this decision
inherits its trust gate), ADR-011 (cascade rules used by the auto-drop
path)

## Context

PRD-patch-already-upstream-detector §3.3 prescribes that
`--auto-drop-merged` should default **off** for `feature`-kind features
and **on** for `hotfix`-kind features. The product reasoning is that
hotfixes are inherently temporary — they exist to be absorbed upstream —
so auto-dropping them on phase-1.5 match matches operator intent. Under
PRD-tpatch-hotfix the feature carries `Manifest.Kind = "hotfix"` and the
`Tpatch-CVE` trailer, both of which survive the auto-drop removal-commit.

v0.8.1 implements the `--auto-drop-merged` flag itself (item 2 of the
v0.8.1 marquee). The question this ADR settles is whether v0.8.1 should
also ship the kind-conditional default — i.e., should
`Manifest.Kind=="hotfix"` flip the default to ON.

This ADR records the decision to **defer** the kind-conditional default.
v0.8.1 ships `--auto-drop-merged` as universally opt-in regardless of
kind.

## Decision drivers

- **`hotfix` kind value is paper-only today** — `Manifest.Kind` exists as
  a field at `internal/store/types.go:223`, but no code path produces or
  recognizes the literal `"hotfix"` value. `tpatch hotfix` (the verb that
  would create hotfix-kind features) is a paper PRD with no shipped
  surface. Default behavior keyed on a value that nothing in the codebase
  currently sets is undefined behavior dressed up as a feature.
- **Compounded silent-destructive risk** — `--auto-drop-merged` removes
  a feature from the DAG with cascade. Defaulting that ON for an entire
  feature class means any false-positive phase-1.5 match against a
  hotfix-kind feature silently deletes work. The detector's own
  false-positive rate is an open question (see ADR-022). Defaulting two
  trust-sensitive layers ON simultaneously is exactly the worst-case
  combination for evidence we do not yet have.
- **Transitive dependency on ADR-022** — Even when the `hotfix` kind
  value ships, the case for kind-conditional auto-drop default rests on
  detector trustworthiness. ADR-022 explicitly defers that trust gate.
  Shipping the hotfix default before the underlying detector default is
  trusted is structurally inverted.
- **Opt-in path is unobstructed** — Operators who run hotfixes through
  `tpatch reconcile --auto-drop-merged` directly get the intended
  behavior today. The deferral is about the *default*, not the
  capability. PRD §3.3's product intent (hotfixes auto-drop on absorb)
  is reachable by every user; only the keystrokes-saved on the default
  are postponed.
- **PRD-tpatch-hotfix §5.4 already deferred its own auto-drop** — That
  deferral was made before the detector even shipped. This ADR continues
  that deferral now that one half (the detector + the flag) is in
  place but the trust evidence is still missing.

## Decision

For v0.8.x, `--auto-drop-merged` defaults OFF universally. The
`Manifest.Kind` value is **not consulted** by the flag's default-resolution
logic. There is no `Config.AutoDropOnUpstreamMerge` or kind-keyed default
shipped in v0.8.1.

A future ADR may introduce a kind-conditional default (or a
`Config.AutoDropOnUpstreamMerge` knob) only after both:

1. ADR-022's flip-criteria are met (real-world false-positive evidence,
   scan-limit telemetry, warn-mode considered), AND
2. The `tpatch hotfix` verb (or some other surface that actually emits
   `Manifest.Kind = "hotfix"`) has shipped and bedded in for at least one
   release cycle.

Both conditions must hold. Either alone is insufficient.

## Re-evaluation criteria (for a future kind-default ADR)

A future ADR may flip the hotfix-kind default to `--auto-drop-merged=on`
only after:

1. **ADR-022 trust gate satisfied** — The detector default itself has
   either flipped to ON or the evidence supports flipping it. Operators
   should not get auto-drop defaults whose underlying detector is still
   untrusted.
2. **Hotfix kind shipped** — `Manifest.Kind="hotfix"` is produced by a
   real CLI surface (`tpatch hotfix` per PRD-tpatch-hotfix, or any
   substitute), bedded in for ≥1 minor release, and has at least one
   recorded round-trip through reconcile in opt-in usage.
3. **Operator-override path designed** — Even with the kind-conditional
   default ON, hotfix operators must be able to opt OUT of auto-drop
   for a specific reconcile invocation (`--no-auto-drop-merged` or
   equivalent). The future ADR specifies that surface.
4. **CVE-trailer audit-trail validated** — PRD §3.3 promises
   `Tpatch-CVE` and `Tpatch-Slug` trailers survive the removal commit.
   The future ADR confirms that promise has held in practice for opt-in
   `--auto-drop-merged` users (i.e., issue 1 above's evidence base
   includes hotfix-shaped features) before defaulting it on.

## Considered alternatives

1. **Ship the kind-conditional default in v0.8.1 as PRD §3.3 prescribes**.
   Rejected — `hotfix` is not a value any code in the repo produces, so
   the default would have no behavioral effect today. When `tpatch hotfix`
   later ships, the default would activate without separate sign-off,
   which is exactly the kind of surprising hidden-default activation that
   ADR-gating is supposed to prevent.
2. **Ship `Config.AutoDropOnUpstreamMerge` as a global default (not
   kind-keyed) defaulting to `false`**. Rejected — adds configuration
   surface without immediate value. Users opt in per-invocation via
   `--auto-drop-merged` until the future ADR decides whether a global
   default is the right shape.
3. **Implement `--no-auto-drop-merged` now to pre-stage the future
   override path**. Rejected as YAGNI for v0.8.1 — until there is any
   default that needs overriding, the inverse flag is dead code. The
   future ADR introduces it alongside the default flip.
4. **Block `tpatch hotfix` ship on this ADR being resolved**. Rejected
   — this ADR explicitly does NOT block `tpatch hotfix`. When that verb
   ships, hotfix-kind features simply use the same universal opt-in
   semantics as feature-kind. The default-flip is a separate, later
   conversation.

## Consequences

- v0.8.1 ships `--auto-drop-merged` with one consistent semantics across
  all feature kinds: opt-in, no-default. The CHANGELOG entry and
  `tpatch reconcile --help` text describe this without reference to
  kind.
- PRD-patch-already-upstream-detector §3.3's "on by default for
  hotfix-kind" sentence is **not** implemented in v0.8.1. The PRD text
  stays as-written (it describes the intended end state); this ADR
  records the divergence and the gating conditions for closing it.
- PRD-tpatch-hotfix §5.4's pre-existing deferral is now reinforced by
  reference: that deferral was made before the detector existed; this
  ADR continues it now that half the machinery is in place.
- No code change ships under this ADR. It is a "decision to not change
  the default behavior" record.
- A future ADR (when both criteria above are met) will introduce the
  kind-conditional default, reference this ADR by number, and supersede
  it in `docs/adrs/README.md`.

## References

- [PRD-patch-already-upstream-detector.md](../prds/PRD-patch-already-upstream-detector.md) §3.3
- [PRD-tpatch-hotfix.md](../prds/PRD-tpatch-hotfix.md) §5.4 (analogous deferral, pre-detector)
- ADR-022 (detector default-on deferral — trust gate this ADR inherits)
- ADR-011 (`tpatch feature remove` cascade rules — used by `--auto-drop-merged`)
- `internal/store/types.go:223` (`Manifest.Kind` field; `"hotfix"` value not produced anywhere in v0.8.x)
