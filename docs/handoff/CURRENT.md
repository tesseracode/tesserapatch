# Current Handoff

## Active Task

- **Task ID**: v0.4.2 released — Tranche A "Truthful Errors" complete
- **Milestone**: All 10 Tranche A items (A1–A10) landed + `docs/{record,feature-layout,reconcile}.md` shipped.
- **Status**: Ready to tag `v0.4.2`. No open Tranche A work.
- **Next**: Tranche B kickoff — headline is `feat-provider-conflict-resolver`. Full backlog in session SQL `todos` table (32 pending feature/improvement todos).

## Session Summary

One full v0.4.2 release cycle landed in this session:

- **A1 bug-implement-silent-fallback** — `Config.MaxTokensImplement` knob (default 16384, was hard-coded 8192). New `WarnWriter io.Writer = os.Stderr` in `internal/workflow/implement.go`; fallback emits a stderr warning naming retry count, error, raw-response path, and the config knob.
- **A2 bug-cycle-state-mismatch** — `RunImplement` writes `StateImplementing`. `assertCycleState` + `featureStateRank` check every phase transition in `internal/cli/phase2.go`.
- **A3 bug-record-validation-false-positive** — new `gitutil.ValidatePatchReverse`. Record now validates round-trip against the tree it captured from; forward validation stays for reconcile.
- **A4 bug-reconcile-phase4-false-positive** — new `gitutil.PreviewForwardApply` runs `--3way` in an isolated `git worktree` and classifies `Strict | 3WayClean | 3WayConflicts | Blocked`. Conflicts promote to `ReconcileBlocked`.
- **A5 bug-skill-invocation-clarity** — three canonical top-of-file blocks (Invocation / Phase Ordering / Preflight) in all 6 skill formats. Parity guard (`assets/assets_test.go`) enforces anchor phrases — wording can't drift.
- **A6 bug-provider-set-global** — `tpatch provider set` defaults to the global config; `--repo` for per-repo override. New `TestMain` in `internal/cli/phase2_test.go` redirects `XDG_CONFIG_HOME` so tests cannot clobber the developer's machine config.
- **A7 bug-extract-json-robustness** — one `ExtractJSONObject` helper replaces four extractors. Brace-balanced, string-aware, handles trailing prose / nested / arrays / escaped quotes / fences. 11-case table test.
- **A8 doc-record-timing** — `tpatch record` refuses clean-tree-no-`--from` with a "captured 0 bytes" diagnostic + up to 10 `git log` candidates. New helpers: `gitutil.RecentCommits`, `gitutil.IsWorkingTreeDirty`. Plus `docs/record.md` + skill one-liner.
- **A9 doc-patches-vs-artifacts** — `docs/feature-layout.md` with the "canonical vs audit trail" callout. `tpatch record` prints a cleanup hint past 6 patches. CLI subcommand (`tpatch patches`) + dedup deferred to v0.5.x (`feat-patches-subcommand`, `feat-record-dedup-patches`).
- **A10 doc-reconcile-workflow** — new `gitutil.PreflightReconcile` + `ReconcilePreflight` struct. `tpatch reconcile` refuses dirty trees / conflict markers / `*.orig|*.rej`. New flags: `--preflight`, `--allow-dirty`. Untracked-`.tpatch/` tip. `docs/reconcile.md` + skill one-liner.

### Version / release

- `internal/cli/cobra.go`: `const version = "0.4.2"`.
- `CHANGELOG.md`: new file, v0.4.2 section written.
- Commit + tag `v0.4.2` pending at time of handoff write.

## Files Changed (net vs v0.4.1)

New files:
- `CHANGELOG.md`
- `docs/record.md`
- `docs/feature-layout.md`
- `docs/reconcile.md`
- `internal/workflow/jsonextract.go` + `jsonextract_test.go`
- `internal/workflow/implement_test.go` (A1/A2)
- `internal/gitutil/gitutil_test.go` (A3/A4/A10)

Substantial edits:
- `internal/cli/cobra.go` — record empty-capture refusal, reconcile preflight + flags, `providerSetCmd` global default, version bump.
- `internal/cli/phase2.go` — `assertCycleState`, `featureStateRank`.
- `internal/cli/phase2_test.go` — `TestMain` XDG isolation, 3 new regression tests.
- `internal/gitutil/gitutil.go` — `ValidatePatchReverse`, `PreviewForwardApply`, `RecentCommits`, `IsWorkingTreeDirty`, `IsPathTracked`, `PreflightReconcile`.
- `internal/workflow/implement.go` — `WarnWriter`, state transition fix, MaxTokens knob, `ExtractJSONObject` migration.
- `internal/workflow/workflow.go`, `retry.go`, `reconcile.go` — migrated to `ExtractJSONObject`.
- `internal/store/{types,store,global}.go` — `MaxTokensImplement` knob.
- All 6 skill files (Claude / Copilot / Cursor / Windsurf / Generic / prompt) — 3 canonical blocks + 2 one-liners (record timing, reconcile clean tree).
- `assets/assets_test.go` — `requiredAnchors` list (10 anchors total).

## Test Results

```
$ gofmt -l .
(clean)

$ go build ./cmd/tpatch
(clean)

$ go test ./...
ok  	.../assets              0.469s
ok  	.../internal/cli        0.945s
ok  	.../internal/gitutil    1.486s
ok  	.../internal/provider   (cached)
ok  	.../internal/safety     (cached)
ok  	.../internal/store      (cached)
ok  	.../internal/workflow   2.124s
```

## Next Steps

1. Single commit with all v0.4.2 changes + co-author trailer; tag `v0.4.2`; push.
2. Begin Tranche B. Top of the backlog: **`feat-provider-conflict-resolver`** — a dedicated LLM-assisted resolver that can process phase-4 3-way conflicts instead of bubbling them up as `blocked`. Natural fit with `feat-soft-recipe-mode` (guidance recipes reconcile more easily).
3. Secondary Tranche B candidates (from session SQL):
   - `feat-feature-amend` — amend an already-recorded feature from an in-tree edit.
   - `feat-noncontiguous-feature-commits` — per-feature commit ledger for features that span discontiguous commits.
   - `feat-init-skill-drift` — apt/dpkg-style skill reconciliation on re-init.
   - `feat-max-tokens-uncapped` — research OpenRouter / LiteLLM / OpenCode conventions before deciding.
4. Stretch (v0.6.0): `feat-ci-cd-integration`, `feat-autoresearch-iterate-until-green`, `feat-delivery-modes`.

## Blockers

None.

## Context for Next Agent

- Session SQL is the authoritative task tracker. 29 pending todos, 49 done at this point.
- All three new docs in `docs/` (`record.md`, `feature-layout.md`, `reconcile.md`) cross-link to each other and `SPEC.md`. When adding another lifecycle doc, follow the same Related section pattern.
- The parity guard (`assets/assets_test.go` `requiredAnchors`) is now the enforcement surface for "what must all skill files say verbatim". When adding a skill block, add an anchor here or it will silently drift.
- `TestMain` in `internal/cli/phase2_test.go` redirects `XDG_CONFIG_HOME`. Any new CLI test that writes provider / global config MUST run in the `internal/cli` package (not elsewhere) to inherit that isolation.
- Reconcile preflight is now a hard gate. When writing tests that exercise reconcile phases, stage a fully clean tree first OR pass `--allow-dirty`.
- The `WarnWriter` pattern (see implement.go) is the convention for non-fatal workflow warnings. Swappable in tests via `prev := WarnWriter; WarnWriter = &buf; defer func() { WarnWriter = prev }()`.
