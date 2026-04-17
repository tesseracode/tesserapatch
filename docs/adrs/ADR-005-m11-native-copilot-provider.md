# ADR-005 — M11 Native Copilot Provider

**Status**: Accepted (implementation gated on open questions 9 & 10 of PRD)
**Date**: 2026-04-17
**Deciders**: Core
**Supersedes**: n/a
**Related**: `docs/prds/PRD-native-copilot-auth.md`, ADR-004

## Context

M11 adds a first-party Go provider (`type: copilot-native`) that talks to GitHub Copilot endpoints directly, with no external proxy process. Three reference implementations exist:

| | opencode | ericc-ch/copilot-api | litellm |
|---|---|---|---|
| Client ID | `Ov23li8tweQw6odWQebz` | `Iv1.b507a08c87ecfe98` | `Iv1.b507a08c87ecfe98` (env-overridable) |
| Session-token exchange | No — OAuth token used as Bearer | Yes — `GET api.github.com/copilot_internal/v2/token`, TTL ~25 min | Yes, with `api-key.json` on-disk cache |
| Editor headers | `user-agent: opencode/<v>`, `Openai-Intent: conversation-edits`, `x-initiator` | VS Code Copilot Chat impersonation: `copilot-integration-id: vscode-chat`, `editor-version: vscode/<v>`, `editor-plugin-version: copilot-chat/0.26.7`, `user-agent: GitHubCopilotChat/0.26.7`, `openai-intent: conversation-panel`, `x-github-api-version: 2025-04-01`, `x-request-id: <uuid>` | VS Code impersonation, older version strings |
| Persisted token | – | `~/.local/share/copilot-api/github-token` | `~/.config/litellm/github_copilot/{access-token,api-key.json}` |
| Base URL | `api.githubcopilot.com` (or `copilot-api.<enterprise>`) | `api.githubcopilot.com` for individual; `api.<account-type>.githubcopilot.com` for enterprise | Same as copilot-api |

Two of three choose the session-token exchange. That path is the one that GitHub Copilot's own editor plugins use and is therefore the least likely to be tightened without collateral damage. The opencode path is newer and narrower.

## Decisions

### D1. Follow the copilot-api/litellm pattern, not opencode's

**Client ID**: `Iv1.b507a08c87ecfe98` (the well-known VS Code Copilot Chat extension client ID; not a secret, shared across copilot-api, litellm, aider, and many others).

**Flow**:

1. **One-time login** (`tpatch provider copilot-login`):
   - Device-code flow against `https://github.com/login/device/code` with `scope=read:user`.
   - Poll `https://github.com/login/oauth/access_token` until `access_token` is returned.
   - Persist the OAuth access token to `$XDG_DATA_HOME/tpatch/copilot-auth.json` (chmod 0600).

2. **Per-session**:
   - Read OAuth token from disk.
   - Call `GET https://api.github.com/copilot_internal/v2/token` with `Authorization: token <oauth>` and the VS Code editor headers → receive a Copilot session token with a ~25-minute expiry.
   - Cache the session token + `expires_at` + `endpoints` object in `copilot-auth.json`.
   - Use the session token as `Authorization: Bearer <session>` for chat completions against `api.githubcopilot.com/chat/completions`.
   - Refresh the session token when it's within 60 seconds of expiry.

**Rationale**: this is identical to what the user's Phase 1 copilot-api is doing today — switching presets changes the transport but not the wire protocol. Any quirks that existed in Phase 1 we already know about.

### D2. Token storage: file on disk, chmod 0600, under `$XDG_DATA_HOME/tpatch/`

File: `$XDG_DATA_HOME/tpatch/copilot-auth.json` (default `~/.local/share/tpatch/copilot-auth.json`).

Schema:

```json
{
  "version": 1,
  "oauth": {
    "access_token": "<ghu_...>",
    "obtained_at": "2026-04-17T07:30:00Z",
    "enterprise_url": null
  },
  "session": {
    "token": "<tid=...>",
    "expires_at": "2026-04-17T07:55:00Z",
    "endpoints": { "api": "https://api.githubcopilot.com" },
    "refreshed_at": "2026-04-17T07:30:04Z"
  }
}
```

Permissions: 0600 on the file, 0700 on its parent directory. Verified on open; if perms are wider we warn and chmod in place.

**OS keychain deferred**: adds macOS `security`/Linux `secret-tool`/Windows DPAPI platform-specific code and a cross-platform dep. The file-based approach matches copilot-api and litellm; both have shipped this pattern without reported compromises. We add a `TODO(adr-005)` to revisit if keychain integration becomes a requested feature.

**Security risk accepted**: a local attacker who can read this file gets a GitHub OAuth token scoped `read:user` plus a 25-min Copilot session token. The OAuth token cannot be used to read private code (scope is `read:user` only, matching copilot-api/litellm) but can be used to make Copilot requests against the user's quota until the user revokes it via GitHub settings. This risk is equivalent to what any developer accepts when running copilot-api today.

### D3. Treat OAuth token as long-lived; re-login on failure

GitHub OAuth access tokens from this flow do not have a published expiry; in practice they last until the user revokes them in GitHub settings or rotates their session. We do **not** proactively refresh the OAuth token. On a 401 from either `copilot_internal/v2/token` or `api.githubcopilot.com/*`, we:

1. Invalidate the cached session token.
2. Retry once with a fresh session-token exchange.
3. If that also 401s, print "OAuth token rejected — run `tpatch provider copilot-login` again" and exit non-zero.

Session token (Copilot) is refreshed proactively 60s before expiry.

### D4. Enterprise: ask at login, default to GitHub.com

`tpatch provider copilot-login` prompts:

```
GitHub deployment?
  1) GitHub.com (default)
  2) GitHub Enterprise Cloud (data residency or self-hosted)
> 1
```

If Enterprise is selected, ask for the domain (e.g. `company.ghe.com`). Store in `copilot-auth.json`. Enterprise login targets `https://<domain>/login/device/code` and the Copilot API is reached at `https://api.<account-type>.githubcopilot.com` (the copilot-api convention — we fetch `account-type` from the Copilot token response, which includes it).

### D5. Model discovery every session, no hardcoded list

On first use per session, call `GET https://api.githubcopilot.com/models` with the session token and cache the response in memory for the lifetime of the tpatch invocation. Do not persist the model list across invocations — Copilot's model catalog changes rapidly (gpt-5, claude-sonnet-4, etc. rotate in/out) and a stale cache causes confusing errors.

If `/models` fails, we still try the configured model; the user's explicit choice wins.

### D6. Editor headers configurable, default = copilot-api's set

Default header set (matching `ericc-ch/copilot-api@0.26.7`):

```
Authorization:             Bearer <session-token>
content-type:              application/json
copilot-integration-id:    vscode-chat
editor-version:            vscode/1.95.0
editor-plugin-version:     copilot-chat/0.26.7
user-agent:                GitHubCopilotChat/0.26.7
openai-intent:             conversation-panel
x-github-api-version:      2025-04-01
x-request-id:              <uuid v4 per request>
x-vscode-user-agent-library-version: electron-fetch
```

`x-initiator` is **not** set by default (matching copilot-api/litellm). It is opt-in via config:

```yaml
provider:
  type: copilot-native
  initiator: agent   # or "user" or unset (default)
```

Users may override any header via `provider.headers_override: { key: value }` in config. This is the escape hatch for when GitHub bumps the expected editor version or integration-id.

Versions baked into the binary are bumped when we observe copilot-api bumping theirs; tracked via a `TODO(adr-005)` in the provider file referencing the copilot-api `api-config.ts` source of truth.

### D7. Distinct provider types, not a shared `copilot` type

To avoid ambiguity when a user has both the proxy running and native configured:

- `type: openai-compatible` with `base_url: http://localhost:4141` — the Phase 1 path via copilot-api (via `--preset copilot`).
- `type: copilot-native` — the Phase 2 path (via `--preset copilot-native`).

Auto-detection probes both (`localhost:4141` first because it's faster), but setting one explicitly disables the other. Preset `copilot` stays pointed at the proxy for backwards compatibility.

### D8. Opt-in gate before first use

`copilot-native` must be explicitly opted into:

```
tpatch config set provider.copilot_native_optin true
```

Attempting to set `type: copilot-native` without the opt-in prints the full AUP quote and exits. The opt-in acknowledgement is recorded in the **global** config (`~/.config/tpatch/config.yaml`) with a timestamp, so it persists across repos.

### D9. No streaming in M11

tpatch workflows are strictly request/response (JSON in, JSON out). We pass `stream: false` in all chat completion requests. If a user configures `stream: true` manually, we reject at config validation.

### D10. Rate limiting: none by default, configurable

Unlike copilot-api which defaults to a 30s rate limit, the native provider has **no artificial throttle** by default — tpatch makes 4–5 calls per feature phase which is well within any reasonable quota. Users can opt in via `provider.rate_limit_seconds: N`.

## Consequences

**Positive**

- Removes the Node/Bun runtime dependency for Copilot users who want single-binary distribution.
- Uses the proven, field-tested transport (copilot-api/litellm pattern) rather than a newer variant.
- Header set and auth flow match what copilot-api already does — regressions are unlikely since the wire format is identical.
- Distinct provider type avoids misconfiguration surprises.

**Negative**

- Material new code to maintain (~350–400 LOC across `copilot_native.go`, `copilot_auth.go`, and tests).
- We now own the "editor impersonation" liability directly rather than delegating it to the copilot-api proxy.
- First-run UX is heavier (device-flow prompt, enterprise question, AUP warning) — mitigated by doing it once then persisting.

## Alternatives considered and rejected

- **Follow opencode's simplified Bearer flow** — rejected because copilot-api/litellm's session-token path has more field exposure. If opencode's simpler path is valid long-term we can migrate to it later with no user-visible change.
- **Use a PAT with "Copilot Requests" permission instead of OAuth device flow** — rejected. PATs with that permission are newer, poorly documented, and require the user to go create one in GitHub settings; OAuth device flow is a single browser tap.
- **Keychain-backed token storage** — deferred, not rejected. Revisit when requested or when a security review calls for it.

## Open questions that must be answered before merge

Carried from `PRD-native-copilot-auth.md`:

1. **Legal/ToS**: can tpatch legitimately send VS Code editor-identification headers to `api.githubcopilot.com`? copilot-api, litellm, opencode, aider, and Neovim Copilot all do this openly today with no observed action from GitHub. Not a legal opinion; a precedent.
2. **GitHub roadmap**: is an official OpenAI-compatible Copilot endpoint planned? If yes within one quarter, we pause M11 and wait.

## Follow-ups / TODOs

- `internal/provider/copilot_native.go` — add `TODO(adr-005): keep editor-version/editor-plugin-version in sync with upstream copilot-api api-config.ts`.
- Add a `GET /_health` style fixture test that replays a captured 200 from `copilot_internal/v2/token` to verify session-token parsing without hitting the network.
- `docs/harnesses/copilot.md` gets a second section: "Proxy path (stable)" and "Native path (experimental, opt-in)".
