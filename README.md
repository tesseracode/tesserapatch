# Tessera Patch

> Fork. Customize. Reconcile. Repeat.

Tessera Patch is a local CLI that lets you customize open-source projects by describing changes in plain English. It tracks your customizations, records them as reproducible patches, and — when upstream releases a new version — automatically detects whether your changes were adopted or still need to be re-applied.

## Install

```bash
go install github.com/tesseracode/tesserapatch/cmd/tpatch@latest
```

This installs the `tpatch` binary into `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

Or build from source:

```bash
git clone https://github.com/tesseracode/tesserapatch.git
cd tesserapatch
go build -o tpatch ./cmd/tpatch
```

## Quick Start

```bash
# 1. Initialize tracking in a project you want to customize
tpatch init --path /path/to/forked-repo

# 2. Describe what you want changed
tpatch add "Change all buttons to blue" --path /path/to/forked-repo

# 3. Analyze the codebase for compatibility
tpatch analyze change-all-buttons-to-blue --path /path/to/forked-repo

# 4. Generate acceptance criteria and a plan
tpatch define change-all-buttons-to-blue --path /path/to/forked-repo

# 5. Generate a deterministic apply recipe
tpatch implement change-all-buttons-to-blue --path /path/to/forked-repo

# 6. Preview what the recipe would do
tpatch apply change-all-buttons-to-blue --dry-run --path /path/to/forked-repo

# 7. Execute the recipe
tpatch apply change-all-buttons-to-blue --mode execute --path /path/to/forked-repo

# 8. Record the patch
tpatch record change-all-buttons-to-blue --path /path/to/forked-repo

# 9. Later, when upstream updates — reconcile
tpatch reconcile --path /path/to/forked-repo
```

## What It Does

Every customization goes through a tracked lifecycle:

```
analyze → define → explore → implement → apply → record → reconcile
```

All state lives in a `.tpatch/` folder inside the project you're customizing — human-readable Markdown and JSON files that travel with the fork.

When upstream releases a new version, `tpatch reconcile` runs a 4-phase check:

1. **Reverse-apply** — Is your patch already in upstream? (fast, free)
2. **Operation-level** — Are individual recipe operations already present? (deterministic)
3. **Semantic** — Does upstream satisfy your acceptance criteria? (LLM-assisted)
4. **Forward-apply** — Can your patch be cleanly re-applied? (safety net)

Confirmed upstream adoptions are retired. Still-needed patches are reapplied
when the existing checks permit; conflicts remain blocked for review.

## Recipe coverage (v0.17 planned, unreleased)

The current GH #15 implementation keeps `artifacts/post-apply.patch` canonical
and publishes `recipe-capture-event.json` before `recipe-coverage.json`
after governed record, patch-amend, reconcile-accept, cycle, apply-done,
implement and bound-artifact edit events. Each replacement is atomic;
the pair is not a cross-file transaction. See the
[seven-producer contract](./SPEC.md#seven-governed-producers).

Publication reports `recipe coverage: complete` or `recipe coverage: incomplete`
with sorted reasons. Complete v1 coverage requires preimage-bearing
`write-file` operations (including explicit-empty creation gates) **and**
all ten proof predicates. Unsupported captures do not produce partial
recipes; existing manual/provider recipes are preserved unless complete
regeneration is explicitly requested. A label is not generated-origin proof.

Recipe coverage is necessary, not sufficient, for future replay eligibility; it is not cross-base safety.
A warn/exit0 coverage row is not eligibility and never grants replay permission.
Missing legacy coverage, including an old stale marker, remains verify-green
absent other failures; malformed or mismatched present coverage blocks.
`tpatch doctor --check D10` diagnoses without repair, even with `--fix`,
and offers regeneration only when a read-only plan establishes it can
truthfully complete. See [recording and recovery](./docs/record.md).

The evidence pair provides unkeyed consistency, not authentication or
history. Readers reconstruct the proof; landing/attestation remain separate.
GH #13's new reconcile operation-replay consumer is future work for a
separate release, not enabled by this coverage publication.

## LLM Provider

The CLI uses an LLM for analysis and implementation. The default is [copilot-api](https://github.com/ericc-ch/copilot-api) (free for GitHub Copilot subscribers):

```bash
# Start copilot-api in another terminal
npx copilot-api@latest start

# tpatch auto-detects it at localhost:4141 during init
tpatch init --path /path/to/repo
```

Or configure any OpenAI-compatible endpoint:

```bash
tpatch provider set --base-url https://api.openai.com --model gpt-4o --auth-env OPENAI_API_KEY --path /path/to/repo
```

The CLI also works **offline** in heuristic mode — you can set up tracking, add features, and generate template artifacts without a provider.

## Agent Skills

`tpatch init` installs portable skill files for 6 coding agent harnesses:

| Harness | Installed To |
|---------|-------------|
| Claude Code | `.claude/skills/tessera-patch/SKILL.md` |
| GitHub Copilot | `.github/skills/tessera-patch/SKILL.md` |
| Copilot Prompt | `.github/prompts/tessera-patch-apply.prompt.md` |
| Cursor | `.cursor/rules/tessera-patch.mdc` |
| Windsurf | `.windsurfrules` |
| Generic | `.tpatch/workflows/tessera-patch-generic.md` |

All skills teach the same methodology — same lifecycle, same `.tpatch/` structure, same feature tracking. Use whichever agent you prefer.

## CLI Reference

| Command | Purpose |
|---------|---------|
| `tpatch init` | Initialize `.tpatch/` and install skills |
| `tpatch add <description>` | Add a feature request |
| `tpatch status` | Show feature status dashboard |
| `tpatch analyze <slug>` | Analyze codebase for compatibility |
| `tpatch define <slug>` | Generate acceptance criteria and plan |
| `tpatch explore <slug>` | Identify relevant files and minimal changeset |
| `tpatch implement <slug>` | Generate deterministic apply recipe |
| `tpatch apply <slug>` | Execute recipe or manage apply session |
| `tpatch record <slug>` | Capture patches (tracked + untracked files) |
| `tpatch reconcile [slug...]` | Reconcile features against upstream updates |
| `tpatch provider check` | Validate LLM provider endpoint |
| `tpatch provider set` | Configure provider |
| `tpatch config show\|set` | Manage configuration |

## Development

```bash
make build    # Build the binary
make test     # Run all tests
make fmt      # Format code
make lint     # Format + vet
make all      # fmt + lint + test + build
```

## License

MIT
