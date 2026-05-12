# ADR-020 - Skill doc references must be self-contained

**Status**: Accepted
**Date**: 2026-05-11
**Owner / byline**: P55
**Source PRD**: [PRD-skill-doc-strategy.md](../prds/PRD-skill-doc-strategy.md)

## Context

Shipped tpatch skill surfaces are embedded in the binary and installed into six harness locations by `tpatch init`, but they currently reference development-repo docs such as `docs/land.md` and `docs/reconcile.md` that are not installed with those surfaces. The PRD contains the full motivation, live grep table, persona analysis, and parity-guard implications.

## Decision drivers

- **Offline ergonomics for Maintainer Mira** - the installed agent handoff must work without this development repo or a network fetch.
- **Skill-as-contract** - SPEC §8 makes the six harness formats the user-facing agent integration surface.
- **Parity-guard simplicity** - the test suite should prevent future repo-relative doc references without validating a bundled-doc version graph.
- **Binary and init simplicity** - avoid adding `.tpatch/docs/` unpacking, overwrite policy, and upgrade sync semantics for a broken-link bug.
- **Maintenance clarity** - concise snippets can be reviewed with skill parity changes; full long-form docs stay contributor references.

## Decision

Adopt the PRD's **inline-minimal** option: shipped skill surfaces must not contain repo-relative `docs/*.md` references. Any command-critical guidance currently delegated to `docs/land.md`, `docs/reconcile.md`, or future long-form docs must be inlined as concise action guidance in all six shipped surfaces. Add a parity-guard test that fails if any shipped surface in `assets/assets_test.go`'s `skillFiles` table contains a repo-relative `docs/*.md` reference.

## Consequences

This enables installed skills to remain self-contained and offline-friendly while preserving the current single-binary and `tpatch init` shape. It forecloses using development-repo `docs/*.md` as end-user skill dependencies unless a later ADR explicitly introduces bundled docs and a resolver guard. Existing repos are not auto-migrated; users receive the corrected guidance when they reinstall or refresh generated skill assets. The accepted maintenance cost is small duplication: concise skill snippets must be reviewed whenever long-form docs change command-critical behavior.

## Alternatives considered

**Embed-and-unpack docs into `.tpatch/docs/`** - rejected because it solves offline access but adds document selection, generated-doc overwrite policy, and tpatch-upgrade sync semantics that are too heavy for the current bug.

**Inline full docs in every skill surface** - rejected because it bloats all six skill formats and turns long-form docs into duplicated sources of truth.

**Public URL references** - rejected because it makes the skill dependent on network access and hosting stability, directly conflicting with Mira's offline ergonomics.

**Hybrid inline-minimal plus optional bundled docs** - rejected for v1 because inline-minimal fixes the user-visible break immediately; bundled docs can be reconsidered only if users need offline long-form design references.

## References

- PRD: `docs/prds/PRD-skill-doc-strategy.md`
- Process precedent: `docs/whitepapers/WP-001-feature-slice-gap.md:177-195`
- Supervisor / review trail: `docs/supervisor/LOG.md` -> "Review - Skill Doc Strategy PRD + ADR-020 - 2026-05-11"
- SPEC: `SPEC.md:155-168`
- Init/install surface: `internal/cli/cobra.go:2055-2077`
- Embed surface: `assets/embed.go:6-9`
- Parity guard: `assets/assets_test.go:114-155`
