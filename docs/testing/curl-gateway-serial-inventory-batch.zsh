#!/usr/bin/env zsh
set -euo pipefail
setopt no_bg_nice

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
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

  echo "api: starting local server"
  (
    cd api
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_inventory_batch_api.log 2>&1
  ) &
  API_PID="$!"

  local i
  for i in {1..40}; do
    if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "api: started"
      return
    fi
    sleep 0.25
  done

  echo "FAIL api startup: healthz not ready"
  if [[ -f /tmp/mp_gateway_inventory_batch_api.log ]]; then
    tail -n 80 /tmp/mp_gateway_inventory_batch_api.log
  fi
  exit 1
}

function cleanup() {
  if [[ -n "${API_PID}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
GW_SERIAL_1="MP-GW-BATCH-${RUN_TAG}-A"
GW_SERIAL_2="MP-GW-BATCH-${RUN_TAG}-B"
RD_SERIAL_1="RD-BATCH-${RUN_TAG}-01"

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

echo "== import serial inventory =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg gw1 "${GW_SERIAL_1}" \
  --arg gw2 "${GW_SERIAL_2}" \
  --arg rd1 "${RD_SERIAL_1}" \
  '{tenant_id:$tenant,items:[
    {serial_number:$gw1,product_type:"gateway",batch_code:"batch-reg",source:"factory"},
    {serial_number:$gw2,product_type:"gateway",batch_code:"batch-reg",source:"factory"},
    {serial_number:$rd1,product_type:"reader",batch_code:"batch-reg",source:"factory"}
  ]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import inventory"

echo "== batch freeze gateway #2 + reader #1 =="
BATCH_FREEZE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg gw2 "${GW_SERIAL_2}" \
  --arg rd1 "${RD_SERIAL_1}" \
  '{tenant_id:$tenant,status:"frozen",serial_numbers:[$gw2,$rd1]}')"
BATCH_FREEZE_RAW="$(api_with_auth PATCH "/api/v1/gateways/serial-inventory/batch-status" "${BATCH_FREEZE_PAYLOAD}")"
split_response "${BATCH_FREEZE_RAW}"
require_http_code "200" "batch freeze"
BATCH_FREEZE_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
if [[ "${BATCH_FREEZE_COUNT}" != "2" ]]; then
  echo "FAIL batch freeze count expected=2 got=${BATCH_FREEZE_COUNT}"
  exit 1
fi
NON_FROZEN="$(echo "${HTTP_BODY}" | jq -r '[.items[] | select(.status != "frozen")] | length')"
if [[ "${NON_FROZEN}" != "0" ]]; then
  echo "FAIL batch freeze status mismatch"
  exit 1
fi

echo "== frozen gateway serial should be blocked =="
REGISTER_2_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL_2}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
REGISTER_2_FROZEN_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${REGISTER_2_PAYLOAD}")"
split_response "${REGISTER_2_FROZEN_RAW}"
require_http_code "409" "register frozen gateway"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "serial_number is not available" ]]; then
  echo "FAIL frozen gateway error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "== batch set back to available =="
BATCH_AVAILABLE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg gw2 "${GW_SERIAL_2}" \
  --arg rd1 "${RD_SERIAL_1}" \
  '{tenant_id:$tenant,status:"available",serial_numbers:[$gw2,$rd1]}')"
BATCH_AVAILABLE_RAW="$(api_with_auth PATCH "/api/v1/gateways/serial-inventory/batch-status" "${BATCH_AVAILABLE_PAYLOAD}")"
split_response "${BATCH_AVAILABLE_RAW}"
require_http_code "200" "batch available"

echo "== register gateway #2 after thaw =="
REGISTER_2_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${REGISTER_2_PAYLOAD}")"
split_response "${REGISTER_2_RAW}"
require_http_code "201" "register gateway #2"
GW_ID_2="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${GW_ID_2}" "gateway #2 id"

echo "== attach reader #1 after thaw =="
ATTACH_READER_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_2}/devices" "$(jq -nc --arg sn "${RD_SERIAL_1}" '{serial_number:$sn,kind:"reader",source:"mistypass_procured",status:"online"}')")"
split_response "${ATTACH_READER_RAW}"
require_http_code "200" "attach reader after thaw"

echo "== batch scrap reader #1 and verify blocked =="
BATCH_SCRAP_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg rd1 "${RD_SERIAL_1}" \
  '{tenant_id:$tenant,status:"scrapped",serial_numbers:[$rd1]}')"
BATCH_SCRAP_RAW="$(api_with_auth PATCH "/api/v1/gateways/serial-inventory/batch-status" "${BATCH_SCRAP_PAYLOAD}")"
split_response "${BATCH_SCRAP_RAW}"
require_http_code "200" "batch scrap reader"
ATTACH_READER_SCRAP_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_2}/devices" "$(jq -nc --arg sn "${RD_SERIAL_1}" '{serial_number:$sn,kind:"reader",source:"mistypass_procured",status:"offline"}')")"
split_response "${ATTACH_READER_SCRAP_RAW}"
require_http_code "409" "attach scrapped reader"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "device serial_number is not available" ]]; then
  echo "FAIL scrapped reader error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "PASS: gateway serial inventory batch regression complete"
