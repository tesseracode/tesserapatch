# Architecture Decision Records

*ADRs are created when a non-obvious technical decision needs to be documented. They are numbered sequentially.*

## Index

- [ADR-001: Add spf13/cobra as CLI Framework Dependency](ADR-001-cobra-dependency.md) — Accepted

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
