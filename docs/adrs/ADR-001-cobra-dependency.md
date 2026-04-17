# ADR-001: Add spf13/cobra as CLI Framework Dependency

**Status**: Accepted  
**Date**: 2026-04-16  
**Context**: Unified tpatch implementation (consolidation project)

## Decision

Add `spf13/cobra` (and its transitive dependency `spf13/pflag`) as the CLI framework, replacing Go's stdlib `flag` package.

## Context

The original review recommendation (§2.2) specified:

> Use Go's standard `flag` package for v0, consider `cobra` for v1 if command count exceeds 10.

The locked-in decision #1 stated "Go with zero external dependencies."

During the M6 bug bash, **BUG-1** was discovered: Go's stdlib `flag` package **cannot parse flags that appear after positional arguments**. For example:

```bash
# This fails with stdlib flag — --mode is ignored
tpatch apply my-feature --mode done

# User must write this instead — unintuitive
tpatch apply --mode done my-feature
```

This is a fundamental limitation of `flag.Parse()` — it stops processing at the first non-flag argument. The `cobra`/`pflag` library supports interspersed flags natively.

## Rationale

1. The CLI has 12+ commands (exceeds the 10-command threshold from the recommendation)
2. Interspersed flag parsing is a **user-facing correctness issue**, not a convenience feature
3. `cobra` is the de facto standard for Go CLI tools (`kubectl`, `docker`, `gh`, `hugo` all use it)
4. The dependency is minimal: `cobra` + `pflag` only, no deep dependency tree
5. The recommendation itself anticipated this: "consider cobra if command count exceeds 10"

## Consequences

- `go.mod` adds 3 modules: `spf13/cobra`, `spf13/pflag`, `inconshreveable/mousetrap` (Windows-only, indirect)
- Binary size increases by ~1MB (acceptable for a CLI tool)
- The "zero external dependencies" locked-in decision is amended to "minimal external dependencies — cobra/pflag only"
- Auto-generated `--help` and shell completion are available for free

## Alternatives Considered

1. **Keep stdlib `flag`**: Rejected — users would hit BUG-1 constantly
2. **Hand-roll interspersed parsing**: Rejected — reimplementing cobra poorly
3. **Use `urfave/cli`**: Rejected — cobra is more widely adopted and has better subcommand support
