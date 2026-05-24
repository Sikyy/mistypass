#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18187}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
API_PID=""
LOG_FILE="${LOG_FILE:-/tmp/mp_oauth2_protocol_api.log}"
REDIRECT_URI="https://example.com/oauth/callback"

function split_response() {
  local raw="$1"
  HTTP_CODE="${raw##*$'\n'}"
  HTTP_BODY="${raw%$'\n'*}"
}

function require_http_code() {
  local expected="$1"
  local step="$2"
  if [[ "${HTTP_CODE}" != "${expected}" ]]; then
    echo "FAIL ${step}: expected HTTP ${expected}, got ${HTTP_CODE}"
    echo "${HTTP_BODY}"
    exit 1
  fi
}

function require_non_empty() {
  local value="$1"
  local step="$2"
  if [[ -z "${value}" || "${value}" == "null" ]]; then
    echo "FAIL ${step}: empty value"
    exit 1
  fi
}

function api_with_auth() {
  local method="$1"
  local endpoint_path="$2"
  local payload="${3:-}"
  if [[ -n "${payload}" ]]; then
    curl -sS -X "${method}" "${API_BASE_URL}${endpoint_path}" \
      -H "Authorization: Bearer ${AT}" \
      -H "Content-Type: application/json" \
      -d "${payload}" \
      -w $'\n%{http_code}'
    return
  fi
  curl -sS -X "${method}" "${API_BASE_URL}${endpoint_path}" \
    -H "Authorization: Bearer ${AT}" \
    -w $'\n%{http_code}'
}

function oauth2_authorize() {
  local scope="$1"
  local state="$2"
  local redirect_uri="${3:-}"
  local args=(
    -sS
    -G
    "${API_BASE_URL}/oauth2/authorize"
    -H
    "Authorization: Bearer ${AT}"
    --data-urlencode
    "response_type=code"
    --data-urlencode
    "client_id=${CLIENT_ID}"
    --data-urlencode
    "scope=${scope}"
    --data-urlencode
    "state=${state}"
    -w
    $'\n%{http_code}'
  )
  if [[ -n "${redirect_uri}" ]]; then
    args+=(--data-urlencode "redirect_uri=${redirect_uri}")
  fi
  curl "${args[@]}"
}

function ensure_api_running() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "api: already running"
    return
  fi

  echo "api: starting local oauth2-enabled server"
  (
    cd api
    PORT="${API_PORT}" \
      OAUTH2_ENABLED=true \
      ENABLE_DEMO_USERS=true \
      DISABLE_LOGIN_RATE_LIMIT=true \
      GOCACHE=/tmp/go-build \
      go run ./cmd/api >"${LOG_FILE}" 2>&1
  ) &
  API_PID="$!"

  local i
  for i in {1..60}; do
    if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "api: started on ${API_BASE_URL}"
      return
    fi
    sleep 0.25
  done

  echo "FAIL api startup: healthz not ready"
  if [[ -f "${LOG_FILE}" ]]; then
    tail -n 120 "${LOG_FILE}"
  fi
  exit 1
}

function cleanup() {
  if [[ -n "${API_PID}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
    if command -v lsof >/dev/null 2>&1; then
      local pids
      pids="$(lsof -ti tcp:"${API_PORT}" 2>/dev/null || true)"
      for pid in ${(f)pids}; do
        [[ -n "${pid}" ]] && kill "${pid}" >/dev/null 2>&1 || true
      done
    fi
  fi
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
ensure_api_running

echo "== login =="
LOGIN_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

echo "== create oauth2 client =="
CREATE_PAYLOAD="$(jq -nc \
  --arg name "CI OAuth2 Protocol ${RUN_TAG}" \
  --arg redirect "${REDIRECT_URI}" \
  '{name:$name,redirect_uris:[$redirect],scopes:["read","write"]}')"
CREATE_RAW="$(api_with_auth POST "/api/v1/oauth2/clients" "${CREATE_PAYLOAD}")"
split_response "${CREATE_RAW}"
require_http_code "201" "create oauth2 client"
CLIENT_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
CLIENT_SECRET="$(echo "${HTTP_BODY}" | jq -r '.client_secret')"
require_non_empty "${CLIENT_ID}" "client.id"
require_non_empty "${CLIENT_SECRET}" "client.client_secret"

echo "== authorize default redirect =="
STATE_JSON="state-${RUN_TAG}-json"
AUTHORIZE_JSON_RAW="$(oauth2_authorize "read" "${STATE_JSON}")"
split_response "${AUTHORIZE_JSON_RAW}"
require_http_code "200" "authorize default redirect"
CODE_JSON="$(echo "${HTTP_BODY}" | jq -r '.code')"
AUTH_REDIRECT="$(echo "${HTTP_BODY}" | jq -r '.redirect_uri')"
AUTH_STATE="$(echo "${HTTP_BODY}" | jq -r '.state')"
require_non_empty "${CODE_JSON}" "authorize.code"
if [[ "${AUTH_REDIRECT}" != "${REDIRECT_URI}" || "${AUTH_STATE}" != "${STATE_JSON}" ]]; then
  echo "FAIL authorize default redirect: unexpected redirect/state"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== exchange code with json token request =="
TOKEN_JSON_PAYLOAD="$(jq -nc \
  --arg client_id "${CLIENT_ID}" \
  --arg client_secret "${CLIENT_SECRET}" \
  --arg code "${CODE_JSON}" \
  --arg redirect "${REDIRECT_URI}" \
  '{grant_type:"authorization_code",client_id:$client_id,client_secret:$client_secret,code:$code,redirect_uri:$redirect}')"
TOKEN_JSON_RAW="$(curl -sS -X POST "${API_BASE_URL}/oauth2/token" \
  -H "Content-Type: application/json" \
  -d "${TOKEN_JSON_PAYLOAD}" \
  -w $'\n%{http_code}')"
split_response "${TOKEN_JSON_RAW}"
require_http_code "200" "exchange json token"
ACCESS_TOKEN_JSON="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
TOKEN_TYPE_JSON="$(echo "${HTTP_BODY}" | jq -r '.token_type')"
TOKEN_SCOPE_JSON="$(echo "${HTTP_BODY}" | jq -r '.scope')"
TOKEN_EXPIRES_JSON="$(echo "${HTTP_BODY}" | jq -r '.expires_in')"
require_non_empty "${ACCESS_TOKEN_JSON}" "token.access_token"
if [[ "${TOKEN_TYPE_JSON}" != "Bearer" || "${TOKEN_SCOPE_JSON}" != "read" || "${TOKEN_EXPIRES_JSON}" -le 0 ]]; then
  echo "FAIL exchange json token: unexpected token metadata"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== reject authorization code replay =="
REPLAY_RAW="$(curl -sS -X POST "${API_BASE_URL}/oauth2/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "client_id=${CLIENT_ID}" \
  --data-urlencode "client_secret=${CLIENT_SECRET}" \
  --data-urlencode "code=${CODE_JSON}" \
  --data-urlencode "redirect_uri=${REDIRECT_URI}" \
  -w $'\n%{http_code}')"
split_response "${REPLAY_RAW}"
require_http_code "400" "reject code replay"
REPLAY_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
if [[ "${REPLAY_ERROR}" != "invalid_grant" ]]; then
  echo "FAIL reject code replay: expected invalid_grant got ${REPLAY_ERROR}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== exchange code with form token request =="
STATE_FORM="state-${RUN_TAG}-form"
AUTHORIZE_FORM_RAW="$(oauth2_authorize "read write" "${STATE_FORM}" "${REDIRECT_URI}")"
split_response "${AUTHORIZE_FORM_RAW}"
require_http_code "200" "authorize explicit redirect"
CODE_FORM="$(echo "${HTTP_BODY}" | jq -r '.code')"
require_non_empty "${CODE_FORM}" "authorize.form.code"
TOKEN_FORM_RAW="$(curl -sS -X POST "${API_BASE_URL}/oauth2/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "client_id=${CLIENT_ID}" \
  --data-urlencode "client_secret=${CLIENT_SECRET}" \
  --data-urlencode "code=${CODE_FORM}" \
  --data-urlencode "redirect_uri=${REDIRECT_URI}" \
  -w $'\n%{http_code}')"
split_response "${TOKEN_FORM_RAW}"
require_http_code "200" "exchange form token"
ACCESS_TOKEN_FORM="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
TOKEN_SCOPE_FORM="$(echo "${HTTP_BODY}" | jq -r '.scope')"
require_non_empty "${ACCESS_TOKEN_FORM}" "form_token.access_token"
if [[ "${TOKEN_SCOPE_FORM}" != "read write" ]]; then
  echo "FAIL exchange form token: unexpected scope ${TOKEN_SCOPE_FORM}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== revoke oauth2 access token =="
REVOKE_RAW="$(curl -sS -X POST "${API_BASE_URL}/oauth2/revoke" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"${ACCESS_TOKEN_FORM}\"}" \
  -w $'\n%{http_code}')"
split_response "${REVOKE_RAW}"
require_http_code "200" "revoke oauth2 token"

echo "== reject excessive scope =="
EXCESSIVE_RAW="$(oauth2_authorize "admin" "state-${RUN_TAG}-admin" "${REDIRECT_URI}")"
split_response "${EXCESSIVE_RAW}"
require_http_code "400" "reject excessive scope"
EXCESSIVE_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
if [[ "${EXCESSIVE_ERROR}" != "requested scopes exceed client's allowed scopes" ]]; then
  echo "FAIL reject excessive scope: unexpected error ${EXCESSIVE_ERROR}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== reject disabled client authorize =="
DISABLE_RAW="$(api_with_auth PATCH "/api/v1/oauth2/clients/${CLIENT_ID}" '{"enabled":false}')"
split_response "${DISABLE_RAW}"
require_http_code "200" "disable oauth2 client"
DISABLED_RAW="$(oauth2_authorize "read" "state-${RUN_TAG}-disabled" "${REDIRECT_URI}")"
split_response "${DISABLED_RAW}"
require_http_code "400" "reject disabled client authorize"
DISABLED_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
if [[ "${DISABLED_ERROR}" != "client is disabled" ]]; then
  echo "FAIL reject disabled client authorize: unexpected error ${DISABLED_ERROR}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== cleanup oauth2 client =="
DELETE_RAW="$(api_with_auth DELETE "/api/v1/oauth2/clients/${CLIENT_ID}")"
split_response "${DELETE_RAW}"
require_http_code "204" "delete oauth2 client"

echo "PASS: oauth2 protocol regression complete"
