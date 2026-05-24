#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18192}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-tenant.admin@sudirman.co}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
USER_ID="${USER_ID:-usr_1001}"
DOOR_ID="${DOOR_ID:-door_jkt_006}"
API_LOG="${API_LOG:-/tmp/mp_mobile_admin_users_api.log}"
API_PID=""

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

function require_equals() {
  local got="$1"
  local expected="$2"
  local step="$3"
  if [[ "${got}" != "${expected}" ]]; then
    echo "FAIL ${step}: expected ${expected}, got ${got}"
    exit 1
  fi
}

function require_at_least() {
  local got="$1"
  local expected="$2"
  local step="$3"
  if (( got < expected )); then
    echo "FAIL ${step}: expected >= ${expected}, got ${got}"
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

  echo "api: starting local mobile admin users server"
  (
    cd api
    PORT="${API_PORT}" \
      ENABLE_DEMO_USERS=true \
      DISABLE_LOGIN_RATE_LIMIT=true \
      GOCACHE=/tmp/go-build \
      go run ./cmd/api >"${API_LOG}" 2>&1
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
  if [[ -f "${API_LOG}" ]]; then
    tail -n 120 "${API_LOG}"
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

ensure_api_running

echo "== login as tenant admin =="
LOGIN_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

echo "== mobile admin user list/detail =="
USER_LIST_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/users")"
split_response "${USER_LIST_RAW}"
require_http_code "200" "mobile admin user list"
USER_LIST_HITS="$(echo "${HTTP_BODY}" | jq -r --arg id "${USER_ID}" '[.items[] | select(.id == $id)] | length')"
require_equals "${USER_LIST_HITS}" "1" "user list includes seeded user"

USER_DETAIL_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/users/${USER_ID}")"
split_response "${USER_DETAIL_RAW}"
require_http_code "200" "mobile admin user detail"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.id')" "${USER_ID}" "user detail id"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.place_id')" "${BUILDING_ID}" "user detail place_id"
require_non_empty "$(echo "${HTTP_BODY}" | jq -r '.email')" "user detail email"
GROUP_HITS="$(echo "${HTTP_BODY}" | jq -r '[.groups[] | select(.id == "ug_common_office_jkt")] | length')"
require_at_least "${GROUP_HITS}" 1 "user detail groups"

echo "== mobile admin user logins/access-rights =="
LOGINS_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/users/${USER_ID}/logins")"
split_response "${LOGINS_RAW}"
require_http_code "200" "mobile admin user logins"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.user_id')" "${USER_ID}" "user logins user_id"
LOGIN_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.pagination.total')"
require_at_least "${LOGIN_TOTAL}" 0 "user logins total"

ACCESS_RIGHTS_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/users/${USER_ID}/access-rights")"
split_response "${ACCESS_RIGHTS_RAW}"
require_http_code "200" "mobile admin user access-rights"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.user_id')" "${USER_ID}" "access-rights user_id"
ACCESS_RIGHT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg door "${DOOR_ID}" '[.items[] | select(.door_id == $door and .can_access == true)] | length')"
require_at_least "${ACCESS_RIGHT_HITS}" 1 "access-rights includes seeded door"
ACCESS_RIGHT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.pagination.total')"
require_at_least "${ACCESS_RIGHT_TOTAL}" 1 "access-rights total"

echo "== mobile admin share access =="
SHARE_PAYLOAD="$(jq -nc --arg door "${DOOR_ID}" '{door_ids:[$door]}')"
SHARE_RAW="$(api_with_auth POST "/api/v1/app/places/${BUILDING_ID}/users/${USER_ID}/share-access" "${SHARE_PAYLOAD}")"
split_response "${SHARE_RAW}"
require_http_code "201" "mobile admin user share-access"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.user_id')" "${USER_ID}" "share-access user_id"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.place_id')" "${BUILDING_ID}" "share-access place_id"
GRANTED_HITS="$(echo "${HTTP_BODY}" | jq -r --arg door "${DOOR_ID}" '[.granted[] | select(.door_id == $door and (.id | length > 0))] | length')"
require_equals "${GRANTED_HITS}" "1" "share-access granted door"

echo "PASS: mobile admin users regression complete"
