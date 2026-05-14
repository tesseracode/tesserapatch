#!/usr/bin/env bash
# Curl-based live provider matrix for the local copilot-api proxy.
#
# The script discovers models from localhost:4141, then tests the same
# request shapes tpatch uses today:
#   - Claude models advertising /v1/messages use Anthropic Messages payloads.
#   - Other chat models use OpenAI Chat Completions payloads at /v1/chat/completions.
#   - The experimental /responses shape can be probed, but it is not the
#     default because current local proxies may advertise /responses while not
#     serving it.
#
# Usage:
#   tests/scripts/model-routing-matrix.sh
#   tests/scripts/model-routing-matrix.sh --base-url http://localhost:4141 --stream --combos
#   tests/scripts/model-routing-matrix.sh --all --full

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:4141}"
INCLUDE_ALL=0
FULL_ENDPOINTS=0
STREAM_PROBES=0
COMBO_PROBES=0
VERBOSE=0
TIMEOUT="${TIMEOUT:-90}"
MAX_TOKENS="${MAX_TOKENS:-512}"

usage() {
  cat <<'USAGE'
Usage: tests/scripts/model-routing-matrix.sh [flags]

Flags:
  --base-url URL   Provider proxy URL (default: $BASE_URL or http://localhost:4141)
  --all            Include non-picker chat models in request probes
  --full           Probe advertised endpoint variants in addition to tpatch route
  --stream         Run representative SSE probes
  --combos         Run payload/body-combination probes for representative models
  --verbose        Print response snippets for failures and empty-content responses
  -h, --help       Show this help

Requires: curl, jq
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url)
      BASE_URL="${2:?missing value for --base-url}"
      shift 2
      ;;
    --all)
      INCLUDE_ALL=1
      shift
      ;;
    --full)
      FULL_ENDPOINTS=1
      shift
      ;;
    --stream)
      STREAM_PROBES=1
      shift
      ;;
    --combos)
      COMBO_PROBES=1
      shift
      ;;
    --verbose)
      VERBOSE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown flag: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

for bin in curl jq; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "missing required command: $bin" >&2
    exit 127
  fi
done

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/tpatch-provider-matrix.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT
MODELS_JSON="$tmpdir/models.json"

curl_json() {
  local path="$1"
  curl -sS --max-time 10 -H 'Accept: application/json' "$BASE_URL$path"
}

fetch_models() {
  if curl_json /models >"$MODELS_JSON"; then
    if jq -e '.data | type == "array"' "$MODELS_JSON" >/dev/null 2>&1; then
      echo "/models"
      return 0
    fi
  fi
  if curl_json /v1/models >"$MODELS_JSON"; then
    if jq -e '.data | type == "array"' "$MODELS_JSON" >/dev/null 2>&1; then
      echo "/v1/models"
      return 0
    fi
  fi
  echo "could not fetch model catalog from $BASE_URL/models or $BASE_URL/v1/models" >&2
  return 1
}

has_endpoint() {
  local csv=",$1,"
  local endpoint="$2"
  [[ "$csv" == *",$endpoint,"* ]]
}

tpatch_route_for() {
  local endpoints="$1"
  if has_endpoint "$endpoints" "/v1/messages"; then
    printf '/v1/messages'
  else
    printf '/v1/chat/completions'
  fi
}

payload_for() {
  local route="$1"
  local model="$2"
  local stream="${3:-false}"
  case "$route" in
    /v1/messages)
      jq -nc \
        --arg model "$model" \
        --argjson max_tokens "$MAX_TOKENS" \
        --argjson stream "$stream" \
        '{
          model: $model,
          system: "You are terse.",
          messages: [{role: "user", content: "Reply exactly: TPATCH_OK"}],
          max_tokens: $max_tokens,
          temperature: 0.1,
          stream: $stream
        }'
      ;;
    /v1/chat/completions|/chat/completions)
      jq -nc \
        --arg model "$model" \
        --argjson max_tokens "$MAX_TOKENS" \
        --argjson stream "$stream" \
        '{
          model: $model,
          messages: [
            {role: "system", content: "You are terse."},
            {role: "user", content: "Reply exactly: TPATCH_OK"}
          ],
          max_tokens: $max_tokens,
          temperature: 0.1,
          stream: $stream
        }'
      ;;
    /responses|/v1/responses)
      jq -nc \
        --arg model "$model" \
        --argjson max_tokens "$MAX_TOKENS" \
        --argjson stream "$stream" \
        '{
          model: $model,
          input: [
            {type: "message", role: "developer", content: "You are terse."},
            {type: "message", role: "user", content: "Reply exactly: TPATCH_OK"}
          ],
          max_output_tokens: $max_tokens,
          stream: $stream
        }'
      ;;
    *)
      return 1
      ;;
  esac
}

extract_text() {
  local file="$1"
  jq -r '
    (
      .choices[0].message.content //
      .content[0].text //
      .output[0].content[0].text //
      .output_text //
      ""
    )
    | tostring
    | gsub("[\r\n|]"; " ")
    | .[0:80]
  ' "$file" 2>/dev/null || true
}

finish_reason() {
  local file="$1"
  jq -r '
    .choices[0].finish_reason //
    .stop_reason //
    .error.message //
    .error //
    ""
    | tostring
    | gsub("[\r\n|]"; " ")
    | .[0:90]
  ' "$file" 2>/dev/null || true
}

request_json() {
  local route="$1"
  local payload="$2"
  local body_file="$3"
  curl -sS --max-time "$TIMEOUT" \
    -o "$body_file" \
    -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json' \
    "$BASE_URL$route" \
    --data "$payload" || true
}

result_icon() {
  local status="$1"
  local text="$2"
  if [[ "$status" == "200" && -n "$text" ]]; then
    printf '✅'
  elif [[ "$status" == "200" ]]; then
    printf '⚠'
  else
    printf '❌'
  fi
}

note_for() {
  local status="$1"
  local text="$2"
  local finish="$3"
  local body_file="$4"
  if [[ "$status" == "200" && -n "$text" ]]; then
    printf 'text extracted'
  elif [[ "$status" == "200" ]]; then
    printf '200 but no text extracted'
  elif [[ -n "$finish" ]]; then
    printf '%s' "$finish"
  else
    head -c 100 "$body_file" | tr '\r\n|' '   '
  fi
}

print_catalog() {
  echo
  echo "## Catalog"
  echo
  echo "| Model | Type | Picker | Advertised endpoints | Reasoning effort | Streaming | Structured |"
  echo "|---|---|---:|---|---|---:|---:|"
  jq -r '
    .data[]
    | (((.supported_endpoints // []) | join(",")) // "") as $endpoints
    | (((.capabilities.supports.reasoning_effort // []) | join(",")) // "") as $effort
    | [
        .id,
        (.capabilities.type // "unknown"),
        (.model_picker_enabled // false | tostring),
        (if $endpoints == "" then "—" else $endpoints end),
        (if $effort == "" then "—" else $effort end),
        (.capabilities.supports.streaming // false | tostring),
        (.capabilities.supports.structured_outputs // false | tostring)
      ]
    | @tsv
  ' "$MODELS_JSON" |
    while IFS=$'\t' read -r model type picker endpoints effort streaming structured; do
      echo "| \`$model\` | $type | $picker | \`$endpoints\` | \`$effort\` | $streaming | $structured |"
    done
}

print_tpatch_matrix() {
  echo
  echo "## Tpatch default request matrix"
  echo
  echo "| Model | Tpatch endpoint | Body format | Status | Works? | Extracted text | Notes |"
  echo "|---|---|---|---:|---:|---|---|"

  local jq_filter='.data[] | select((.capabilities.type // "") == "chat")'
  if [[ "$INCLUDE_ALL" != "1" ]]; then
    jq_filter="$jq_filter | select(.model_picker_enabled == true)"
  fi

  jq -r "$jq_filter | [.id, ((.supported_endpoints // []) | join(\",\"))] | @tsv" "$MODELS_JSON" |
    while IFS=$'\t' read -r model endpoints; do
      local route body_format payload body status text finish icon note
      route="$(tpatch_route_for "$endpoints")"
      if [[ "$route" == "/v1/messages" ]]; then
        body_format="Anthropic Messages"
      else
        body_format="OpenAI Chat Completions"
      fi
      payload="$(payload_for "$route" "$model" false)"
      body="$tmpdir/body-${model//[^A-Za-z0-9_.-]/_}.json"
      status="$(request_json "$route" "$payload" "$body")"
      text="$(extract_text "$body")"
      finish="$(finish_reason "$body")"
      icon="$(result_icon "$status" "$text")"
      note="$(note_for "$status" "$text" "$finish" "$body")"
      echo "| \`$model\` | \`$route\` | $body_format | $status | $icon | \`${text:-—}\` | $note |"
      if [[ "$VERBOSE" == "1" && ( "$status" != "200" || -z "$text" ) ]]; then
        echo "<details><summary>$model raw response</summary>"
        echo
        echo '```json'
        jq '.' "$body" 2>/dev/null || cat "$body"
        echo '```'
        echo
        echo "</details>"
      fi
    done
}

print_full_endpoint_matrix() {
  echo
  echo "## Advertised endpoint probes"
  echo
  echo "| Model | Advertised endpoint | Curl route tested | Status | Works? | Extracted text | Notes |"
  echo "|---|---|---|---:|---:|---|---|"

  jq -r '
    .data[]
    | select((.capabilities.type // "") == "chat")
    | select(.model_picker_enabled == true)
    | [.id, ((.supported_endpoints // []) | join(","))]
    | @tsv
  ' "$MODELS_JSON" |
    while IFS=$'\t' read -r model endpoints; do
      [[ -n "$endpoints" ]] || endpoints="/chat/completions"
      IFS=',' read -ra endpoint_list <<<"$endpoints"
      for endpoint in "${endpoint_list[@]}"; do
        [[ "$endpoint" == ws:* ]] && continue
        local routes=()
        case "$endpoint" in
          /v1/messages) routes=(/v1/messages) ;;
          /chat/completions) routes=(/v1/chat/completions) ;;
          /responses) routes=(/responses /v1/responses) ;;
          *) routes=("$endpoint") ;;
        esac
        for route in "${routes[@]}"; do
          local payload body status text finish icon note
          payload="$(payload_for "$route" "$model" false || true)"
          if [[ -z "$payload" ]]; then
            continue
          fi
          body="$tmpdir/body-${model//[^A-Za-z0-9_.-]/_}-${route//\//_}.json"
          status="$(request_json "$route" "$payload" "$body")"
          text="$(extract_text "$body")"
          finish="$(finish_reason "$body")"
          icon="$(result_icon "$status" "$text")"
          note="$(note_for "$status" "$text" "$finish" "$body")"
          echo "| \`$model\` | \`$endpoint\` | \`$route\` | $status | $icon | \`${text:-—}\` | $note |"
        done
      done
    done
}

first_model_matching() {
  local pattern="$1"
  jq -r --arg pattern "$pattern" '
    .data[]
    | select((.capabilities.type // "") == "chat")
    | select(.model_picker_enabled == true)
    | select(.id | test($pattern))
    | .id
  ' "$MODELS_JSON" | head -n 1
}

first_existing_model() {
  local model
  for model in "$@"; do
    if jq -e --arg model "$model" '
      any(.data[]; .id == $model and (.capabilities.type // "") == "chat" and (.model_picker_enabled == true))
    ' "$MODELS_JSON" >/dev/null; then
      printf '%s\n' "$model"
      return 0
    fi
  done
  return 1
}

combo_payload() {
  local model="$1"
  local combo="$2"
  case "$combo" in
    tpatch-chat-512)
      jq -nc --arg model "$model" '{
        model: $model,
        messages: [{role:"system",content:"You are terse."},{role:"user",content:"Reply exactly: TPATCH_OK"}],
        max_tokens: 512,
        temperature: 0.1,
        stream: false
      }'
      ;;
    tiny-max-tokens-32)
      jq -nc --arg model "$model" '{
        model: $model,
        messages: [{role:"system",content:"You are terse."},{role:"user",content:"Reply exactly: TPATCH_OK"}],
        max_tokens: 32,
        temperature: 0.1,
        stream: false
      }'
      ;;
    max-completion-tokens-32)
      jq -nc --arg model "$model" '{
        model: $model,
        messages: [{role:"system",content:"You are terse."},{role:"user",content:"Reply exactly: TPATCH_OK"}],
        max_completion_tokens: 32,
        temperature: 0.1,
        stream: false
      }'
      ;;
    reasoning-effort-low)
      jq -nc --arg model "$model" '{
        model: $model,
        messages: [{role:"system",content:"You are terse."},{role:"user",content:"Reply exactly: TPATCH_OK"}],
        max_tokens: 128,
        temperature: 0.1,
        reasoning_effort: "low",
        stream: false
      }'
      ;;
    reasoning-object-low)
      jq -nc --arg model "$model" '{
        model: $model,
        messages: [{role:"system",content:"You are terse."},{role:"user",content:"Reply exactly: TPATCH_OK"}],
        max_tokens: 128,
        temperature: 0.1,
        reasoning: {effort: "low"},
        stream: false
      }'
      ;;
  esac
}

print_combo_matrix() {
  local models=()
  local gpt gemini
  gpt="$(first_existing_model gpt-5.5 gpt-5.4 gpt-5-mini || true)"
  gemini="$(first_model_matching '^gemini-')"
  [[ -n "$gpt" ]] && models+=("$gpt")
  [[ -n "$gemini" ]] && models+=("$gemini")
  [[ ${#models[@]} -gt 0 ]] || return 0

  echo
  echo "## Payload combination probes"
  echo
  echo "| Model | Combo | Route | Status | Works? | Extracted text | Notes |"
  echo "|---|---|---|---:|---:|---|---|"
  for model in "${models[@]}"; do
    for combo in tpatch-chat-512 tiny-max-tokens-32 max-completion-tokens-32 reasoning-effort-low reasoning-object-low; do
      local payload body status text finish icon note
      payload="$(combo_payload "$model" "$combo")"
      body="$tmpdir/combo-${model//[^A-Za-z0-9_.-]/_}-$combo.json"
      status="$(request_json /v1/chat/completions "$payload" "$body")"
      text="$(extract_text "$body")"
      finish="$(finish_reason "$body")"
      icon="$(result_icon "$status" "$text")"
      note="$(note_for "$status" "$text" "$finish" "$body")"
      echo "| \`$model\` | \`$combo\` | \`/v1/chat/completions\` | $status | $icon | \`${text:-—}\` | $note |"
    done
  done
}

stream_probe() {
  local label="$1"
  local route="$2"
  local model="$3"
  local body="$tmpdir/stream-${label//[^A-Za-z0-9_.-]/_}.txt"
  local headers="$tmpdir/stream-${label//[^A-Za-z0-9_.-]/_}.headers"
  local payload status content_type bytes events icon note
  payload="$(payload_for "$route" "$model" true)"
  status="$(curl -sS -N --max-time "$TIMEOUT" \
    -D "$headers" \
    -o "$body" \
    -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -H 'Accept: text/event-stream' \
    "$BASE_URL$route" \
    --data "$payload" || true)"
  content_type="$(awk 'BEGIN{IGNORECASE=1} /^content-type:/{sub(/\r$/,""); print $0; exit}' "$headers")"
  bytes="$(wc -c <"$body" | tr -d ' ')"
  events="$(grep -E '^(event|data):' "$body" | head -n 3 | tr '\r\n|' '   ' | cut -c1-100 || true)"
  if [[ "$status" == "200" && -n "$events" ]]; then
    icon="✅"
    note="SSE events present"
  elif [[ "$status" == "200" ]]; then
    icon="⚠"
    note="200 but no SSE event lines"
  else
    icon="❌"
    note="$(head -c 100 "$body" | tr '\r\n|' '   ')"
  fi
  echo "| $label | \`$route\` | \`$model\` | $status | $icon | \`$content_type\` | $bytes | $note |"
}

print_stream_matrix() {
  local claude gpt
  claude="$(first_existing_model claude-sonnet-4.6 claude-haiku-4.5 claude-opus-4.6 || true)"
  gpt="$(first_existing_model gpt-5.5 gpt-5.4 gpt-4.1 gpt-4o || true)"

  echo
  echo "## Streaming probes"
  echo
  echo "| Probe | Route | Model | Status | Works? | Content-Type | Bytes | Notes |"
  echo "|---|---|---|---:|---:|---|---:|---|"
  [[ -n "$claude" ]] && stream_probe "Claude messages SSE" /v1/messages "$claude"
  [[ -n "$gpt" ]] && stream_probe "GPT chat SSE" /v1/chat/completions "$gpt"
  [[ -n "$gpt" ]] && stream_probe "Responses SSE" /responses "$gpt"
}

catalog_path="$(fetch_models)"
model_count="$(jq '.data | length' "$MODELS_JSON")"
picker_chat_count="$(jq '[.data[] | select((.capabilities.type // "") == "chat") | select(.model_picker_enabled == true)] | length' "$MODELS_JSON")"

echo "# tpatch provider model routing matrix"
echo
echo "- Base URL: \`$BASE_URL\`"
echo "- Catalog endpoint: \`$catalog_path\`"
echo "- Models discovered: $model_count"
echo "- User-pickable chat models: $picker_chat_count"
echo "- Tpatch default token budget used by this script: $MAX_TOKENS"

print_catalog
print_tpatch_matrix
[[ "$FULL_ENDPOINTS" == "1" ]] && print_full_endpoint_matrix
[[ "$COMBO_PROBES" == "1" ]] && print_combo_matrix
[[ "$STREAM_PROBES" == "1" ]] && print_stream_matrix
exit 0
