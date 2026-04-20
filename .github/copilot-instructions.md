# Copilot CLI — Repository Pointer

This file is auto-loaded by GitHub Copilot CLI. It's intentionally short: the real
instructions live in the repository root.

## Read these, in order

1. [`CLAUDE.md`](../CLAUDE.md) — agent orientation, repo shape, working rules
2. [`AGENTS.md`](../AGENTS.md) — agent roles, cyclic workflow, handoff contract
3. [`SPEC.md`](../SPEC.md) — product & technical contract for the `tpatch` CLI
4. [`docs/ROADMAP.md`](../docs/ROADMAP.md) — milestone plan and current phase
5. [`docs/handoff/CURRENT.md`](../docs/handoff/CURRENT.md) — active task and session state
6. [`CHANGELOG.md`](../CHANGELOG.md) — released versions and their scope

## Resuming work

- Session state for the Copilot CLI itself lives in `~/.copilot/` (independent
  of this repo). Use `/resume` to pick up a prior session.
- Project state lives in `docs/handoff/CURRENT.md`. If you are a fresh agent,
  read it first — it tells you what the previous agent was doing, what landed,
  and what is next.
- Architecture decisions are in `docs/adrs/`. Check there before proposing
  alternatives to locked-in choices (Go, zero-dep, deterministic recipe, etc.).

## Working rules (summary — see `CLAUDE.md` for full list)

1. Only create/edit files inside this `tpatch/` folder.
2. Minimal external Go dependencies: `cobra/pflag` + stdlib.
3. No secrets in tracked files. Use the secret-by-reference pattern.
4. `.tpatch/` artifacts must be deterministic.
5. After code changes, run `gofmt`, `go test ./...`, `go build ./cmd/tpatch`.
6. Update `docs/handoff/CURRENT.md` at every phase transition — not only at
   session end. See `AGENTS.md` → "Context Preservation Rules".
7. Shipped skill assets in `assets/` must stay aligned with the CLI contract.
   The parity guard (`assets/assets_test.go`) enforces this.
8. Every git commit must include the co-author trailer:

   ```
   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

## Current release

See [`CHANGELOG.md`](../CHANGELOG.md) for the latest version. `docs/handoff/CURRENT.md`
tracks work-in-progress after the most recent tag.
