# Architecture Decision Records

*ADRs are created when a non-obvious technical decision needs to be documented. They are numbered sequentially.*

## Index

- [ADR-001: Add spf13/cobra as CLI Framework Dependency](ADR-001-cobra-dependency.md) — Accepted
- [ADR-002: Provider Strategy](ADR-002-provider-strategy.md) — Accepted
- [ADR-003: SDK Evaluation](ADR-003-sdk-evaluation.md) — Accepted
- [ADR-004: M10 Copilot Proxy UX](ADR-004-m10-copilot-proxy-ux.md) — Accepted
- [ADR-005: M11 Native Copilot Provider](ADR-005-m11-native-copilot-provider.md) — Accepted
- [ADR-010: Provider-Assisted Conflict Resolver](ADR-010-provider-conflict-resolver.md) — Accepted
- [ADR-011: Feature Dependency DAG](ADR-011-feature-dependencies.md) — Accepted
- [ADR-012: Feature Tested State](ADR-012-feature-tested-state.md) — Superseded by ADR-013
- [ADR-013: Verify Freshness Overlay](ADR-013-verify-freshness-overlay.md) — Accepted
- [ADR-014: Smart Endpoint Routing for the copilot-api Proxy](ADR-014-smart-endpoint-routing.md) — Accepted
- [ADR-015: Prior-Art Mapping for Identity Duality, Operation Log, and Stack Primitives](ADR-015-prior-art-identity-mapping.md) — Accepted (research framework)
- [ADR-027: Capture Context Privacy Boundary](ADR-027-capture-context-privacy-boundary.md) — Accepted
- [ADR-028: Supersession Edge Model](ADR-028-supersession-edge-model.md) — Accepted
- [ADR-029: Write-file Recipe Safety](ADR-029-write-file-recipe-safety.md) — Accepted
- [ADR-034: Rooted Filesystem Inspection Boundary](ADR-034-rooted-filesystem-inspection-boundary.md) — Accepted (2026-08-13), errata rev-3 (2026-08-17, three `openFlags()` build halves, no decision change); companion to accepted [PRD-artifact-validation-and-provenance](../prds/PRD-artifact-validation-and-provenance.md) rev-5 (errata rev-6)
- [ADR-035: Intent Bundle Publication and History](ADR-035-intent-bundle-publication-and-history.md) — Accepted (2026-08-14), no-decision errata through rev-19 accepted jointly on 2026-08-29, and rev-20's bounded selector diagnostic amendment accepted on 2026-08-30; companion to accepted [PRD-prepare-intent-bundle](../prds/PRD-prepare-intent-bundle.md) rev-20. Decisions D1–D21. The mutating implementation (PRD §17.2 slices S1–S7, GH #23) and its acceptance are complete; release authorization is pending.
- [ADR-036: Recipe Generation and Coverage Authority](ADR-036-recipe-coverage-authority.md) — Accepted rev-7 (2026-09-01, after rev-0 through rev-6 NEEDS REVISION and rev-7 APPROVED); companion to accepted [PRD-recipe-generation-authority](../prds/PRD-recipe-generation-authority.md) rev-7, GH #15, and prerequisite for GH #13. Decisions D1–D17; amends [ADR-029](ADR-029-write-file-recipe-safety.md) D3 for exact-postimage recognition while preserving D7 supersession severity.
- [ADR-037: Reconcile Operation-Replay Candidate Authority](ADR-037-reconcile-operation-replay-candidate-authority.md) — Accepted rev-6 (2026-09-02, after rev-0 through rev-4 NEEDS REVISION, rev-5 APPROVED with notes, and rev-6 APPROVED); companion to accepted [PRD-reconcile-operation-replay-candidate](../prds/PRD-reconcile-operation-replay-candidate.md) rev-6, GH #13, target v0.18.0. Decisions D1–D36 plus D7b, D9.1, D12b, D20b and D29b; implementation remains blocked until GH #15 ships in v0.17.0.

## Locked-In Decisions (from review process)

These decisions were made during the three-team review and are pre-approved. They do not need individual ADRs unless they are revisited:

1. ~~Go with zero external dependencies~~ → Amended by ADR-001: minimal deps (cobra/pflag only)
2. 4-phase reconciliation architecture
3. Deterministic apply recipe format
4. Path traversal protection
5. Secret-by-reference pattern
6. 6 skill formats with parity guard
7. Heuristic offline fallback
8. Untracked file capture in patches
9. `upstream.lock` + `steering/` directory
10. YAML for config, JSON for structured artifacts, Markdown for human docs
