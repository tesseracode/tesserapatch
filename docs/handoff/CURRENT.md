# Current Handoff

## Active Task
- **Task ID**: M10 — Managed Copilot proxy UX (ADR-004)
- **Milestone**: M10 delivered
- **Description**: Honest UX for the reverse-engineered `copilot-api` proxy — global config, reachability probe, first-run AUP warning, install pointers, CI release automation.
- **Status**: Implemented; awaiting supervisor review.
- **Assigned**: 2026-04-17

## Session Summary

1. **CI release automation** — added a `release` job to `.github/workflows/ci.yml` that triggers on `v*` tag pushes, creates a GitHub Release via `softprops/action-gh-release@v2`, auto-generates release notes, and marks tags containing `-` as prereleases. Uses the default `GITHUB_TOKEN` with `contents: write`. Cost: free.
2. **Global config** — new `internal/store/global.go` adds `GlobalConfigPath()`, `LoadGlobalConfig`, `SaveGlobalConfig`, `(s *Store).LoadMergedConfig`, `AcknowledgeCopilotAUP`, `CopilotAUPAcknowledged`, `mergeConfig`, `renderGlobalYAML`. Honors `XDG_CONFIG_HOME`, falls back to `os.UserConfigDir()` (macOS caveat documented in the harness doc). Chmod 0600 on write.
3. **Config precedence** — repo `.tpatch/config.yaml` overrides the global config field-by-field; zero values do **not** clear globals (must set the field explicitly). AUP ack is global-only.
4. **Types** — `Config.CopilotAUPAckAt string` added to `internal/store/types.go`.
5. **Reachability probe** — new `internal/provider/probe.go` with `Reachable(ctx, cfg)` (2s timeout), `IsLocalEndpoint(cfg)`, `IsCopilotProxyEndpoint(cfg)` helpers. Probes via existing `Check()`.
6. **CLI wiring** — new `internal/cli/copilot.go` with `copilotInstallHint`, `copilotAUPWarning`, `maybeShowAUPWarning`, `ensureProviderReachable`, `warnIfUnreachable`, `providerConfigFromStore`. Wired into `init` (warn-continue + AUP) and `providerSetCmd` + `autoDetectProvider` (AUP on first Copilot selection).
7. **Workflow hard-fail** — `loadAndProbeProvider(ctx, s)` replaces `loadProviderFromStore` in analyze/define/explore/implement/cycle. Probes once per process (cached per base URL). Local-endpoint-only; opt-out via `TPATCH_NO_PROBE=1`. Non-local endpoints skip the probe to avoid penalising custom remote configs.
8. **Execute now surfaces errors** — `Execute()` prints `error: %v` to stderr before returning exit code 1 so probe failures are visible. Preserves existing `SilenceErrors: true` cobra behaviour for graceful formatting.
9. **Harness doc refresh** — `docs/harnesses/copilot.md` now documents the install path, OS-dependent global config path (macOS caveat), warn-vs-fail behaviour, and links to ADR-004/005.
10. **Tests** — 6 new tests in `internal/store/global_test.go` (roundtrip, missing file, ack idempotency, precedence, merge-no-clear, save creates dir) and 5 in `internal/provider/probe_test.go` (httptest OK, TEST-NET-1 timeout, not-configured, URL matcher, cancelled ctx). All 7 packages pass.

## Files Created
- `.github/workflows/ci.yml` — amended (release job)
- `internal/store/global.go`
- `internal/store/global_test.go`
- `internal/provider/probe.go`
- `internal/provider/probe_test.go`
- `internal/cli/copilot.go`

## Files Changed
- `internal/cli/cobra.go` — `loadAndProbeProvider`, `Execute` prints errors, AUP wiring in `init` / `providerSetCmd` / `autoDetectProvider`, `sync` import.
- `internal/store/types.go` — `CopilotAUPAckAt` field.
- `docs/harnesses/copilot.md` — M10 section.

## Test Results
- `gofmt -w .` clean
- `go vet ./...` clean
- `go test ./... -count=1` — 7/7 packages pass
- `go build ./cmd/tpatch` OK
- Smoke: `init` + `provider set --preset copilot` prints AUP warning exactly once; second run is quiet; `analyze` against a dead localhost port hard-fails with an install hint; against a live copilot-api proxy falls through to the workflow.

## Key Behaviours

- **Warn vs fail**: `init` and `provider set` are warn-continue (a user may be bootstrapping before starting the proxy). Workflow commands that actually call the LLM (`analyze|define|explore|implement|cycle`) hard-fail when the local endpoint is unreachable.
- **Probe scope**: only runs for local endpoints (`localhost`, `127.0.0.1`, `[::1]`). Remote endpoints are trusted.
- **AUP once**: the AUP warning fires only when the new config actually points at the copilot-api proxy (`openai-compatible` + port 4141) and the user has not acknowledged before.
- **TODO**: `copilotInstallHint` carries an inline `TODO(adr-004)` comment to revisit the tesserabox fork recommendation if its divergent fixes become blocking.

## Blockers
- None for M10.
- M11 still soft-blocked on the two open questions in ADR-005 (editor-headers legal/ToS, official endpoint roadmap). User direction: proceed with editor headers, monitor; so these are effectively closed as "accept risk".

## Next Steps
1. Supervisor review of M10 implementation.
2. Commit as `feat(m10): managed copilot-api proxy UX (ADR-004)` and push.
3. Consider tagging `v0.3.1` once review lands — CI will produce the GitHub Release automatically.
4. Start M11 implementation per ADR-005 (native Copilot provider with session-token exchange) once M10 is merged.

## Context for Next Agent
- Global config on macOS defaults to `~/Library/Application Support/tpatch/config.yaml` unless `XDG_CONFIG_HOME` is set. Every test that touches global state sets `XDG_CONFIG_HOME` to a tempdir; follow this pattern.
- `TPATCH_NO_PROBE=1` disables the workflow hard-fail probe (useful for offline demos or CI steps that only read store state). Add it to future tests that should not hit the network.
- The probe cache is a process-level `map[string]error` guarded by a mutex — fine for the CLI's one-shot lifecycle but intentionally not time-bound, so long-running processes would need to invalidate it. Not a concern today.
- `Execute()` now prints errors. Tests that exercise `rootCmd.Execute()` directly still use the cobra `SetErr` buffer; only the top-level wrapper prints to stderr.
- The AUP warning text lives in `internal/cli/copilot.go::copilotAUPWarning`. Tweak there, not in harness docs.
