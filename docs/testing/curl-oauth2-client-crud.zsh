#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18186}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
API_PID=""
LOG_FILE="${LOG_FILE:-/tmp/mp_oauth2_client_crud_api.log}"

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
  --arg name "CI OAuth2 Client ${RUN_TAG}" \
  '{name:$name,redirect_uris:["https://example.com/oauth/callback","http://localhost:5173/oauth/callback"],scopes:["read","write"]}')"
CREATE_RAW="$(api_with_auth POST "/api/v1/oauth2/clients" "${CREATE_PAYLOAD}")"
split_response "${CREATE_RAW}"
require_http_code "201" "create oauth2 client"
CLIENT_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
CLIENT_SECRET="$(echo "${HTTP_BODY}" | jq -r '.client_secret')"
CREATE_ENABLED="$(echo "${HTTP_BODY}" | jq -r '.enabled')"
CREATE_SCOPES="$(echo "${HTTP_BODY}" | jq -r '.scopes | sort | join(",")')"
require_non_empty "${CLIENT_ID}" "create.id"
require_non_empty "${CLIENT_SECRET}" "create.client_secret"
if [[ "${CREATE_ENABLED}" != "true" || "${CREATE_SCOPES}" != "read,write" ]]; then
  echo "FAIL create oauth2 client: unexpected enabled/scopes"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== list oauth2 clients =="
LIST_RAW="$(api_with_auth GET "/api/v1/oauth2/clients")"
split_response "${LIST_RAW}"
require_http_code "200" "list oauth2 clients"
LIST_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${CLIENT_ID}" '.items | map(select(.id==$id)) | length')"
LIST_SECRET_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | map(select((.client_secret // "") != "")) | length')"
if [[ "${LIST_COUNT}" -lt 1 || "${LIST_SECRET_COUNT}" -ne 0 ]]; then
  echo "FAIL list oauth2 clients: expected created client and no exposed secret"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== update oauth2 client =="
UPDATE_PAYLOAD="$(jq -nc \
  --arg name "CI OAuth2 Client Updated ${RUN_TAG}" \
  '{name:$name,redirect_uris:["https://example.com/oauth/updated"],scopes:["read"],enabled:false}')"
UPDATE_RAW="$(api_with_auth PATCH "/api/v1/oauth2/clients/${CLIENT_ID}" "${UPDATE_PAYLOAD}")"
split_response "${UPDATE_RAW}"
require_http_code "200" "update oauth2 client"
UPDATED_NAME="$(echo "${HTTP_BODY}" | jq -r '.name')"
UPDATED_ENABLED="$(echo "${HTTP_BODY}" | jq -r '.enabled')"
UPDATED_SCOPES="$(echo "${HTTP_BODY}" | jq -r '.scopes | join(",")')"
UPDATED_REDIRECT="$(echo "${HTTP_BODY}" | jq -r '.redirect_uris[0]')"
UPDATED_SECRET="$(echo "${HTTP_BODY}" | jq -r '.client_secret // ""')"
if [[ "${UPDATED_NAME}" != "CI OAuth2 Client Updated ${RUN_TAG}" || "${UPDATED_ENABLED}" != "false" || "${UPDATED_SCOPES}" != "read" || "${UPDATED_REDIRECT}" != "https://example.com/oauth/updated" || -n "${UPDATED_SECRET}" ]]; then
  echo "FAIL update oauth2 client: unexpected response"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== delete oauth2 client =="
DELETE_RAW="$(api_with_auth DELETE "/api/v1/oauth2/clients/${CLIENT_ID}")"
split_response "${DELETE_RAW}"
require_http_code "204" "delete oauth2 client"

echo "== verify oauth2 client deleted =="
GET_DELETED_RAW="$(api_with_auth GET "/api/v1/oauth2/clients/${CLIENT_ID}")"
split_response "${GET_DELETED_RAW}"
require_http_code "404" "get deleted oauth2 client"

LIST_AFTER_RAW="$(api_with_auth GET "/api/v1/oauth2/clients")"
split_response "${LIST_AFTER_RAW}"
require_http_code "200" "list oauth2 clients after delete"
LIST_AFTER_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${CLIENT_ID}" '.items | map(select(.id==$id)) | length')"
if [[ "${LIST_AFTER_COUNT}" -ne 0 ]]; then
  echo "FAIL list oauth2 clients after delete: deleted client still present"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "PASS: oauth2 client CRUD regression complete"
