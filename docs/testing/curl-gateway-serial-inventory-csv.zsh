#!/usr/bin/env zsh
set -euo pipefail

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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_inventory_csv_api.log 2>&1
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
  if [[ -f /tmp/mp_gateway_inventory_csv_api.log ]]; then
    tail -n 80 /tmp/mp_gateway_inventory_csv_api.log
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
GW_SERIAL="MP-GW-CSV-${RUN_TAG}"
RD_SERIAL="RD-CSV-${RUN_TAG}"

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

echo "== import serial inventory by csv_content =="
CSV_CONTENT=$'serial_number,product_type,batch_code,source\n'"${GW_SERIAL}"$',gateway,csv-batch,factory\n'"${RD_SERIAL}"$',reader,csv-batch,factory'
CSV_IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg csv "${CSV_CONTENT}" \
  '{tenant_id:$tenant,csv_content:$csv}')"
CSV_IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import-csv" "${CSV_IMPORT_PAYLOAD}")"
split_response "${CSV_IMPORT_RAW}"
require_http_code "201" "import csv serial inventory"
IMPORTED_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
if [[ "${IMPORTED_COUNT}" -lt 2 ]]; then
  echo "FAIL import csv serial inventory: expected >=2 items got ${IMPORTED_COUNT}"
  exit 1
fi

echo "== export csv and verify rows =="
EXPORT_RAW="$(api_with_auth GET "/api/v1/gateways/serial-inventory/export-csv?tenant_id=${TENANT_ID}")"
split_response "${EXPORT_RAW}"
require_http_code "200" "export csv serial inventory"
if ! echo "${HTTP_BODY}" | grep -q "^serial_number,product_type,status,tenant_id"; then
  echo "FAIL export csv header mismatch"
  exit 1
fi
if ! echo "${HTTP_BODY}" | grep -q "${GW_SERIAL}"; then
  echo "FAIL export csv missing gateway serial ${GW_SERIAL}"
  exit 1
fi
if ! echo "${HTTP_BODY}" | grep -q "${RD_SERIAL}"; then
  echo "FAIL export csv missing reader serial ${RD_SERIAL}"
  exit 1
fi

echo "== register gateway with csv imported serial =="
REGISTER_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
REGISTER_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${REGISTER_PAYLOAD}")"
split_response "${REGISTER_RAW}"
require_http_code "201" "register gateway from csv serial"
GW_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${GW_ID}" "gateway.id"

echo "== export consumed csv and verify consumed gateway id =="
CONSUMED_RAW="$(api_with_auth GET "/api/v1/gateways/serial-inventory/export-csv?tenant_id=${TENANT_ID}&status=consumed&product_type=gateway")"
split_response "${CONSUMED_RAW}"
require_http_code "200" "export consumed gateway csv"
if ! echo "${HTTP_BODY}" | grep -q "${GW_SERIAL}"; then
  echo "FAIL consumed export missing gateway serial ${GW_SERIAL}"
  exit 1
fi
if ! echo "${HTTP_BODY}" | grep -q "${GW_ID}"; then
  echo "FAIL consumed export missing consumed_gateway_id ${GW_ID}"
  exit 1
fi

echo "PASS: gateway serial inventory csv import/export regression complete"
