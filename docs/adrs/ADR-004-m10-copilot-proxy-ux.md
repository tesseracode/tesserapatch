# ADR-004 — M10 Copilot Proxy UX

**Status**: Accepted
**Date**: 2026-04-17
**Deciders**: Core
**Supersedes**: n/a
**Related**: `docs/prds/PRD-native-copilot-auth.md`

## Context

M10 gives users a one-command path to use GitHub Copilot through tpatch. The only working transport today is [`copilot-api`](https://github.com/ericc-ch/copilot-api), a reverse-engineered proxy. The proxy itself is a separate binary that the user installs out-of-band; tpatch must decide how much of its lifecycle to own, where its config lives, and how to fail gracefully when the proxy is absent or unresponsive.

## Decisions

### D1. We do **not** supervise the proxy process

tpatch detects whether the proxy is reachable and surfaces actionable errors, but it does **not** start, stop, or background-manage `copilot-api`. Rationale:

- Supervising a Node/Bun process in Go across macOS and Linux is not free (signal handling, stdout piping, zombie reaping, `npx` temp-dir gotchas).
- The proxy outlives a single `tpatch` invocation by design; the user already runs it once and reuses it across tools (Claude Code, aider, tpatch).
- Owning the lifecycle implies responsibility for crashes, log rotation, port conflicts — all scope we do not need to take on to deliver the stated UX win.

Consequence: there is **no** `tpatch provider copilot-start` / `copilot-stop` process-manager in M10. Those commands, if they exist at all, only print instructions and/or check reachability.

### D2. Upstream `ericc-ch/copilot-api`, not the `tesserabox` fork

The `tesserabox/copilot-api` fork adds 1M-context model translation fixes; upstream has a simpler one-liner install. For M10 we document and point users at upstream. An internal `TODO: review switching to tesserabox fork if context-window fixes become blocking` is added in `internal/cli/cobra.go` where the install instructions are emitted.

Consequence: `tpatch init` and first-run messages recommend `npm install -g copilot-api` or `npx copilot-api@latest start` (the install options the upstream README documents).

### D3. Global config at `~/.config/tpatch/config.yaml`, repo overrides in `.tpatch/config.yaml`

Today, `.tpatch/config.yaml` is per-repo and there is no global config. M10 introduces a **global config file** at `$XDG_CONFIG_HOME/tpatch/config.yaml` (defaulting to `~/.config/tpatch/config.yaml`). Load order:

1. Global config (if present) — base values.
2. Repo `.tpatch/config.yaml` — overrides and feature-specific fields.
3. Environment variables (e.g. `OPENAI_API_KEY`) — credentials only, never stored.

Fields most users will set globally: `provider.type`, `provider.base_url`, `provider.model`, `provider.auth_env`, `max_retries`.

Fields that stay per-repo: `test_command`, feature state, artifacts.

Secrets are never written to either file; only the env var *name* is stored (`auth_env`).

### D4. Warn-on-unreachable, do not block `tpatch init`

`tpatch init` with a Copilot-flavoured preset runs a best-effort reachability probe:

```
GET <base_url>/v1/models  (timeout 2s)
```

- **200 with a `data` array**: success, print "connected".
- **Connection refused / timeout**: print a prominent warning pointing at the copilot-api install instructions, then **continue** with init (do not fail).
- **Any other HTTP error**: print the status and the response body (truncated), then continue.

Subsequent workflow commands (`analyze`, `define`, …) re-run the same probe on first call and error out hard if the proxy is still unreachable, with the same install pointer.

### D5. No log piping, no log files in M10

Because tpatch does not own the proxy, we do not capture its stdout/stderr. Users troubleshoot via the proxy's own terminal. We add a single line in `docs/harnesses/copilot.md` that says "run `copilot-api start` in a dedicated terminal so you can see its logs".

### D6. Windows: not supported in M10

Copilot-api on Windows works through `npx` with some caveats. Rather than claim support, we document:

> **Windows is not tested in M10.** The underlying `copilot-api` proxy works on Windows, but tpatch's Copilot preset and the auto-detection heuristics are only validated on macOS and Linux in v0.3.0.

No CI matrix entry for Windows.

### D7. First-run AUP warning, stored in global config

On the first time a user sets any Copilot-flavoured preset, tpatch prints:

```
Heads up — the GitHub Copilot integration uses copilot-api
(https://github.com/ericc-ch/copilot-api), a reverse-engineered
proxy that is not supported by GitHub. Excessive automated use
may trigger GitHub's abuse-detection systems.
See https://docs.github.com/en/site-policy/acceptable-use-policies
```

And records `copilot_aup_acknowledged_at: <ISO-8601>` in the global config. Subsequent preset switches skip the warning.

### D8. Enterprise support deferred

Copilot-api supports enterprise via `--account-type <slug>`. M10 exposes this only as a passthrough in `tpatch provider set --preset copilot --base-url <custom>`. We do not add a dedicated `--enterprise` flag until an enterprise user requests it. (Tracked in the M11 ADR instead, since M11 has native OAuth and needs enterprise first-class anyway.)

## Consequences

**Positive**

- Minimal new CLI surface: no `copilot-start/stop` supervision logic.
- Global+repo config pattern unlocks future reuse across repos without touching M10.
- First-run warning ensures users consent to abuse-detection exposure without nagging on every call.
- Upstream proxy pointer keeps tpatch aligned with the widely-reviewed version.

**Negative**

- Users still run a separate process in a separate terminal — not "fully native".
- The two-file config loader is new code and must be tested for precedence edge cases.
- If the upstream proxy diverges from the tesserabox fork in ways that hurt users, we carry the TODO debt.

## Alternatives considered and rejected

- **Auto-start a supervised child process** — rejected per D1.
- **Ship the proxy as a vendored Node script inside the Go binary** — rejected; Node dependency runtime is outside our stdlib-only ethos.
- **Silent install via `npx copilot-api@latest`** — rejected; pulling unreviewed code without prompting violates the consent posture we want for an unsupported surface.

## Follow-ups / TODOs

- `internal/cli/cobra.go`: add `TODO(adr-004)` comment at the install-instruction emission site noting the fork-vs-upstream decision point.
- Update `docs/harnesses/copilot.md` to reflect "run in a dedicated terminal" and the reachability probe.
- Add a smoke test that verifies `tpatch init` warns-but-continues when `localhost:4141` is unreachable.
