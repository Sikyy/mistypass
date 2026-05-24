#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18193}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-tenant.admin@sudirman.co}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
ZONE_ID="${ZONE_ID:-area_demo_001}"
DOOR_ID="${DOOR_ID:-door_jkt_001}"
HOLIDAY_REGION_ID="${HOLIDAY_REGION_ID:-ID}"
HOLIDAY_YEAR="${HOLIDAY_YEAR:-2026}"
API_LOG="${API_LOG:-/tmp/mp_mobile_admin_resources_api.log}"
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
  curl -sS -X "${method}" "${API_BASE_URL}${endpoint_path}" \
    -H "Authorization: Bearer ${AT}" \
    -w $'\n%{http_code}'
}

function ensure_api_running() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "api: already running"
    return
  fi

  echo "api: starting local mobile admin resources server"
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

echo "== mobile admin zones =="
ZONE_LIST_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/zones")"
split_response "${ZONE_LIST_RAW}"
require_http_code "200" "mobile admin zone list"
ZONE_HITS="$(echo "${HTTP_BODY}" | jq -r --arg zone "${ZONE_ID}" '[.items[] | select(.id == $zone and .door_count >= 1)] | length')"
require_at_least "${ZONE_HITS}" 1 "zone list includes seeded zone"

ZONE_DETAIL_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/zones/${ZONE_ID}")"
split_response "${ZONE_DETAIL_RAW}"
require_http_code "200" "mobile admin zone detail"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.id')" "${ZONE_ID}" "zone detail id"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.place_id')" "${BUILDING_ID}" "zone detail place_id"
DOOR_HITS="$(echo "${HTTP_BODY}" | jq -r --arg door "${DOOR_ID}" '[.doors[] | select(.id == $door)] | length')"
require_at_least "${DOOR_HITS}" 1 "zone detail includes seeded door"

echo "== mobile admin holiday regions =="
REGION_LIST_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/holiday-regions")"
split_response "${REGION_LIST_RAW}"
require_http_code "200" "mobile admin holiday region list"
REGION_HITS="$(echo "${HTTP_BODY}" | jq -r --arg region "${HOLIDAY_REGION_ID}" '[.items[] | select(.code == $region)] | length')"
require_at_least "${REGION_HITS}" 1 "holiday region list includes configured region"

HOLIDAYS_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/holiday-regions/${HOLIDAY_REGION_ID}/holidays?year=${HOLIDAY_YEAR}")"
split_response "${HOLIDAYS_RAW}"
require_http_code "200" "mobile admin holidays"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.region_id')" "${HOLIDAY_REGION_ID}" "holidays region_id"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.year')" "${HOLIDAY_YEAR}" "holidays year"
HOLIDAY_HITS="$(echo "${HTTP_BODY}" | jq -r --arg date "${HOLIDAY_YEAR}-08-17" '[.items[] | select(.date == $date)] | length')"
require_at_least "${HOLIDAY_HITS}" 1 "holidays include independence day"

echo "PASS: mobile admin resources regression complete"
