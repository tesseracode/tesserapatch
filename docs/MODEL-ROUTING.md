# Model Routing Matrix & Provider Selection

This document records the live contract observed against the local
`copilot-api` proxy at `http://localhost:4141` and how `tpatch` routes provider
requests for that proxy.

## Current finding

The proxy catalog advertises `/responses` for several GPT-5.x models, but the
live server tested here returns `404 Not Found` for both `/responses` and
`/v1/responses`. Those same models work through `/v1/chat/completions`.

That means the safe default for tpatch is:

1. Use `/v1/messages` for models that advertise `/v1/messages` (Claude family).
2. Use `/v1/chat/completions` for all other chat models, including GPT-5.x.
3. Keep `ResponsesProvider` experimental/gated until the local proxy actually
   serves a responses route.

## Live curl evidence

Representative probes against `http://localhost:4141`:

| Model | Route | Status | Result |
|---|---|---:|---|
| `claude-sonnet-4.6` | `/v1/messages` | 200 | Anthropic-format text response |
| `gpt-5.4` | `/v1/chat/completions` | 200 | OpenAI chat-format text response |
| `gpt-5.5` | `/v1/chat/completions` | 200 | OpenAI chat-format text response |
| `gpt-5.5` | `/responses` | 404 | `404 Not Found` |
| `gpt-5.5` | `/v1/responses` | 404 | `404 Not Found` |
| `gpt-4.1` | `/v1/chat/completions` | 200 | OpenAI chat-format text response |
| `gpt-4o` | `/v1/chat/completions` | 200 | OpenAI chat-format text response |
| `gemini-2.5-pro` | `/v1/chat/completions` | 200 | OpenAI chat-format text response when token budget is large enough |

Streaming probes:

| Probe | Route | Status | Result |
|---|---|---:|---|
| Claude messages streaming | `/v1/messages` | 200 | Anthropic SSE events (`event:` + `data:`) |
| GPT chat streaming | `/v1/chat/completions` | 200 | OpenAI chat SSE events (`data:`) |
| GPT responses streaming | `/responses` | 404 | Route not served by local proxy |

## Request shapes tpatch uses

### Claude via `/v1/messages`

```bash
curl -sS http://localhost:4141/v1/messages \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  --data '{
    "model": "claude-sonnet-4.6",
    "system": "You are terse.",
    "messages": [{"role": "user", "content": "Reply exactly: TPATCH_OK"}],
    "max_tokens": 512,
    "temperature": 0.1,
    "stream": false
  }'
```

### GPT/Gemini via `/v1/chat/completions`

```bash
curl -sS http://localhost:4141/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  --data '{
    "model": "gpt-5.5",
    "messages": [
      {"role": "system", "content": "You are terse."},
      {"role": "user", "content": "Reply exactly: TPATCH_OK"}
    ],
    "max_tokens": 512,
    "temperature": 0.1,
    "stream": false
  }'
```

### Experimental `/responses` probe

This is the shape implemented by `ResponsesProvider`, but the local proxy tested
here does not serve the route:

```bash
curl -sS http://localhost:4141/responses \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  --data '{
    "model": "gpt-5.5",
    "input": [
      {"type": "message", "role": "developer", "content": "You are terse."},
      {"type": "message", "role": "user", "content": "Reply exactly: TPATCH_OK"}
    ],
    "max_output_tokens": 512,
    "stream": false
  }'
```

Expected result on the current proxy: `404 Not Found`.

## Full live matrix command

Run the curl suite:

```bash
tests/scripts/model-routing-matrix.sh --stream --combos
```

Useful variants:

```bash
# Include every chat model, not just user-pickable models.
tests/scripts/model-routing-matrix.sh --all

# Also probe advertised endpoint variants, including /responses and /v1/responses.
tests/scripts/model-routing-matrix.sh --full

# Change target proxy.
tests/scripts/model-routing-matrix.sh --base-url http://localhost:4141
```

The suite prints:

1. Catalog table with advertised endpoints, reasoning-effort metadata,
   streaming support, and structured-output support.
2. Tpatch default request matrix for user-pickable chat models.
3. Optional advertised-endpoint probes.
4. Optional payload-combination probes.
5. Optional SSE probes.

## Payload-combination notes

Reasoning-heavy models can spend small token budgets on internal reasoning and
return empty or truncated visible content. In live probes:

| Model family | Small `max_tokens` | Larger `max_tokens` / tpatch default | `max_completion_tokens` |
|---|---|---|---|
| GPT-5 mini | Often 200 with empty content at `max_tokens: 32` | Works at `max_tokens: 512` and tpatch's normal budgets | Works |
| Gemini 3 preview | Often truncates at tiny budgets | Works at `max_tokens: 512` and tpatch's normal budgets | Works |
| Gemini 2.5 Pro | Often truncates at tiny budgets | Works at `max_tokens: 512` and tpatch's normal budgets | Works |

Current workflow calls use materially larger budgets (`4096`, `1024`, or
`16384` depending on phase), so the tiny-budget failure mode is mostly a curl
diagnostic trap rather than a default tpatch problem.

## Implementation notes

- `OpenAICompatible.Check` parses `supported_endpoints` from `/v1/models` into
  `Health.ModelInfo`.
- `loadAndProbeProvider` probes local endpoints once and passes `Health` into
  `provider.PickProvider`.
- `PickProvider` routes `/v1/messages` models to `AnthropicProvider`.
- `ResponsesProvider` remains gated by `TPATCH_ENABLE_RESPONSES_PROVIDER`
  because this live proxy advertises `/responses` but does not serve it.
- With the gate off, GPT-5.x models fall through to `OpenAICompatible`, which
  posts to `/v1/chat/completions`; live curl confirms that path works for
  `gpt-5.2`, `gpt-5.2-codex`, `gpt-5.3-codex`, `gpt-5.4`,
  `gpt-5.4-mini`, and `gpt-5.5`.

## Troubleshooting

### Model appears in `/models` but generation fails

Check which route is actually served:

```bash
curl -sS http://localhost:4141/models \
  | jq '.data[] | select(.id == "gpt-5.5") | {id, supported_endpoints}'

tests/scripts/model-routing-matrix.sh --full --verbose
```

If `/responses` is advertised but returns 404, keep
`TPATCH_ENABLE_RESPONSES_PROVIDER` unset and use the default chat-completions
fallback.

### Empty content with status 200

Increase the token budget before judging the route as broken:

```bash
MAX_TOKENS=1024 tests/scripts/model-routing-matrix.sh --combos --verbose
```

This matters most for GPT mini and Gemini reasoning models.
