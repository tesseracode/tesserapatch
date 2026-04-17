# M4 — Reconciliation

**Status**: ⬜ Not started  
**Depends on**: M3

## Tasks

- [ ] M4.1 — Implement `upstream.lock` read/write with commit hash, timestamp, branch tracking
- [ ] M4.2 — Implement Phase 1: reverse-apply check (`git apply -R --check`)
- [ ] M4.3 — Implement Phase 2: operation-level evaluation from `apply-recipe.json` (port from experimental)
- [ ] M4.4 — Implement Phase 3: provider-assisted semantic detection (port from GPT)
- [ ] M4.5 — Implement Phase 4: forward-apply attempt (`git apply --check`)
- [ ] M4.6 — Implement incremental patch derivation for multi-feature scenarios (port from GPT)
- [ ] M4.7 — Implement `tpatch reconcile [--upstream-ref] [slug...] --path`
- [ ] M4.8 — Implement reconciliation artifact generation: `reconcile.md`, `reconcile.patch`, `reconcile-session.json`
- [ ] M4.9 — Implement per-version reconciliation logs in `reconciliation/<commit-range>.md` (from CC)
- [ ] M4.10 — Implement state transitions: `applied` → `upstream_merged` / `applied` (reapplied) / `blocked`
- [ ] M4.11 — Write reconciliation tests: upstreamed scenario, still-needed scenario, blocked scenario, provider-assisted scenario

## Acceptance Criteria

- `tpatch reconcile` correctly classifies features as `upstream_merged`, reapplied, or `blocked`
- Phase 1 (reverse-apply) works as fast path for exact upstream matches
- Phase 2 (operation-level) detects partial upstream adoption
- Phase 3 (provider-assisted) catches structural differences (the case GPT found in bug bash)
- Phase 4 (forward-apply) re-applies surviving patches cleanly
- Multi-feature reconciliation handles feature isolation via incremental patches
- `upstream.lock` is updated with new commit hash after reconciliation
- Reconciliation artifacts are generated and stored properly

## Reference

- Port 3-phase reconciliation from `../gpt/internal/workflow/reconcile.go`
- Port operation-level evaluation from `../experimental/src/core/workflow.ts` (`evaluateRecipeReconciliation`)
- Port per-version logs from `../cc/src/core/reconciler.ts`
- Port incremental patch derivation from `../gpt/internal/workflow/reconcile.go` (`deriveIncrementalPatch`)
