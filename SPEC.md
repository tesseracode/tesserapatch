# Tessera Patch — Unified Implementation

## SPEC.md

### 1. Purpose

Tessera Patch is a local-first CLI and agent skill framework for customizing upstream open-source projects with natural-language-driven changes while preserving enough structure to reapply, review, and reconcile those changes when upstream evolves.

This is the **unified implementation**, consolidating the best ideas from three independent prototypes (cc/, experimental/, gpt/).

### 2. Architecture

**Language**: Go 1.22+  
**Distribution**: Single binary `tpatch` with embedded assets via `go embed`  
**Dependencies**: Zero external (stdlib only)  
**Agent integration**: Claude skill, VS Code Copilot skill, Copilot prompt companion, Cursor rules, Windsurf rules, generic markdown workflow  
**Provider interface**: OpenAI-compatible HTTP adapter, with heuristic offline fallback

### 3. Core Workflow — The 7-Phase Lifecycle

```
analyse → define → explore → implement → test → record → reconcile
```

Every tracked feature moves through this state machine:

```
requested → analyzed → defined → implementing → applied → active
                                                              ↓
                                                        reconciling → active (new version)
                                                              ↓        or upstream_merged
                                                           blocked      or blocked
```

#### Feature States

| State | Meaning |
|-------|---------|
| `requested` | User submitted feature request |
| `analyzed` | Analysis complete, compatibility assessed |
| `defined` | Acceptance criteria and plan ready |
| `implementing` | Changes being applied |
| `applied` | Changes applied, verification pending or passed |
| `active` | Feature live in fork, being maintained |
| `reconciling` | Upstream updated; re-evaluating patches |
| `blocked` | Failed; needs manual intervention |
| `upstream_merged` | Feature adopted upstream; local patch retired |

### 4. CLI Commands

#### Phase 1 (MVP)

| Command | Purpose |
|---------|---------|
| `tpatch init [--path]` | Create `.tpatch/` workspace + install all skill formats |
| `tpatch add <description> [--path]` | Create a feature request with stable slug |
| `tpatch status [--feature] [--json] [--verbose] [--path]` | Feature status dashboard |
| `tpatch analyze <slug> [--path] [--timeout]` | Run analysis phase |
| `tpatch define <slug> [--path]` | Generate acceptance criteria + implementation plan |
| `tpatch explore <slug> [--path]` | Read codebase, find minimal changeset |
| `tpatch implement <slug> [--path]` | Generate deterministic apply recipe |
| `tpatch apply <slug> [--mode prepare\|started\|done] [--path]` | Execute recipe or record session |
| `tpatch record <slug> [--path]` | Capture patches (tracked + untracked files) |
| `tpatch reconcile [--upstream-ref] [slug...] [--path]` | Reconcile features against upstream |
| `tpatch provider check [--path]` | Validate provider endpoint |
| `tpatch config show\|set [--path]` | Manage configuration |

#### Phase 2 (Post-MVP)

| Command | Purpose |
|---------|---------|
| `tpatch upstream check [--path]` | Check for new upstream versions |
| `tpatch cycle <slug> [--path]` | Run full lifecycle in one command |
| `tpatch test <slug> [--path]` | Run project tests, verify criteria |
| `tpatch replay <slug> [--path]` | Replay patch on fresh clone for verification |

### 5. `.tpatch/` Data Model

```
.tpatch/
├── config.yaml
├── FEATURES.md
├── upstream.lock
├── steering/
│   ├── local.md
│   └── upstream.md
├── workflows/
│   └── tessera-patch-generic.md
└── features/
    └── <slug>/
        ├── status.json
        ├── request.md
        ├── analysis.md
        ├── spec.md
        ├── exploration.md
        ├── record.md
        ├── reconciliation/
        │   └── <commit-range>.md
        └── artifacts/
            ├── apply-recipe.json
            ├── apply-session.json
            ├── analysis.json
            ├── pre-apply.patch
            ├── post-apply.patch
            ├── reconcile.patch
            ├── reconcile-session.json
            ├── reconcile.md
            └── manual-validation.md
```

### 6. Provider Interface

```go
type Provider interface {
    Check(ctx context.Context, cfg ProviderConfig) (*Health, error)
    Generate(ctx context.Context, cfg ProviderConfig, req GenerateRequest) (string, error)
}
```

**Config** (in `.tpatch/config.yaml`):
```yaml
provider:
  type: openai-compatible
  base_url: http://localhost:4141
  model: gpt-4o
  auth_env: GITHUB_TOKEN    # env var name, NOT the secret
```

**Heuristic fallback**: When no provider is available, generate template-based artifacts. Mark in `status.json` that heuristic mode was used.

**Auto-detection**: On first run, probe copilot-api at localhost:4141.

### 7. Reconciliation — 4-Phase Decision Tree

```
Phase 1: Reverse-Apply Check (fast, free)
    → If patch already present in upstream → UPSTREAMED

Phase 2: Operation-Level Evaluation (deterministic, no LLM)
    → If apply-recipe.json exists, evaluate each operation individually
    → All already-present → UPSTREAMED
    → Any conflict → BLOCKED
    → Mix → STILL-NEEDED

Phase 3: Provider-Assisted Semantic Check (when Phase 1-2 inconclusive)
    → Send acceptance criteria + upstream diff to LLM
    → upstreamed / still_needed / unclear

Phase 4: Forward-Apply Attempt (safety net)
    → Try git apply on new upstream
    → Clean → REAPPLIED
    → Conflicts → BLOCKED
```

### 8. Skill System

**6 harness formats** embedded in the binary, installed by `tpatch init`:

| Harness | Install Location |
|---------|-----------------|
| Claude Code | `.claude/skills/tessera-patch/SKILL.md` |
| GitHub Copilot | `.github/skills/tessera-patch/SKILL.md` |
| Copilot Prompt | `.github/prompts/tessera-patch-apply.prompt.md` |
| Cursor | `.cursor/rules/tessera-patch.mdc` |
| Windsurf | `.windsurfrules` |
| Generic | `.tpatch/workflows/tessera-patch-generic.md` |

**Parity guard**: `assets_test.go` ensures all formats mention current CLI commands.

### 9. Safety

1. `ensureSafeRepoPath()` before any file write outside `.tpatch/`
2. Deterministic apply recipe — generate, review, then execute
3. Secret-by-reference — store env var name, not secret
4. Patch artifact filtering — exclude `.tpatch/`, `.claude/skills/`, `.github/skills/`, `.github/prompts/`, `.cursor/rules/`
5. Untracked file capture in patches

### 10. Sources

This spec consolidates decisions from:
- `review/RECOMMENDATIONS.md` §2 (technical decisions)
- `review/RECONCILIATION-REVIEW.md` §7.1 (4-phase reconciliation)
- `gpt/SPEC.md` (CLI contract, data model, provider interface)
- `cc/SPEC.md` (lifecycle phases, steering files, upstream tracking)
- `experimental/SPEC.md` (apply recipe format, safety constraints)

### 11. Success Criteria

The unified implementation passes the reconciliation bug bash:
1. Clone copilot-api at pinned commit, apply two features, simulate upstream update
2. Feature A (model translation fix) correctly identified as UPSTREAMED
3. Feature B (models CLI subcommand) correctly re-applied as STILL-NEEDED
4. Target repo tests pass after reconciliation
5. All 6 skill formats pass the parity regression test
