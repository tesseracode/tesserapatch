# M2 — Provider & Analysis

**Status**: ✅ Complete
**Depends on**: M1

## Tasks

- [x] M2.1 — Implement provider interface (`Check`, `Generate`) in `internal/provider/`
- [x] M2.2 — Implement OpenAI-compatible provider adapter (raw `net/http`, no external deps)
- [x] M2.3 — Implement `tpatch provider check` — probe endpoint, list models
- [x] M2.4 — Implement provider config in `config.yaml` with secret-by-reference (`auth_env` field)
- [x] M2.5 — Implement heuristic offline fallback — template-based analysis when no provider
- [x] M2.6 — Implement `tpatch analyze <slug>` — workspace snapshot, guidance file detection, LLM analysis, save artifacts
- [x] M2.7 — Implement `tpatch define <slug>` — generate acceptance criteria and implementation plan in `spec.md`
- [x] M2.8 — Implement `tpatch explore <slug>` — read relevant files, produce exploration log
- [x] M2.9 — Implement centralized prompt templates (inline builders in workflow.go)
- [x] M2.10 — Implement provider auto-detection (via loadProvider helper)
- [x] M2.11 — Write mock provider test server (httptest in provider_test.go)
- [x] M2.12 — Write tests for provider check, analyze, define, explore

## Acceptance Criteria

- `tpatch provider check` validates a live or mock endpoint
- `tpatch analyze` produces `analysis.md` + `artifacts/analysis.json` with workspace context
- `tpatch define` produces `spec.md` with acceptance criteria
- `tpatch explore` produces `exploration.md` with file summaries
- All three commands work in heuristic mode (no provider) with template output
- Provider errors mark feature as `blocked`, not silent failure

## Reference

- Port provider from `../gpt/internal/provider/openai.go`
- Port heuristic fallback from `../experimental/src/core/workflow.ts`
- Port prompt templates from `../cc/src/utils/prompts.ts` (adapt to Go text/template)
- Port mock server from `../experimental/scripts/mock-copilot-server.ts` (rewrite in Go for tests)
