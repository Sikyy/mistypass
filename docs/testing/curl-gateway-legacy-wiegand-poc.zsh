#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-8080}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
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

function bootstrap_with_token() {
  local method="$1"
  local endpoint_path="$2"
  local payload="$3"
  curl -sS -X "${method}" "${API_BASE_URL}${endpoint_path}" \
    -H "X-Device-Token: ${DEVICE_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "${payload}" \
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
    PORT="${API_PORT}" GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_legacy_wiegand_poc.log 2>&1
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
  if [[ -f /tmp/mp_gateway_legacy_wiegand_poc.log ]]; then
    tail -n 80 /tmp/mp_gateway_legacy_wiegand_poc.log
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
GW_SERIAL="MP-GW-WGPOC-${RUN_TAG}"
LEGACY_READER_SERIAL="LEG-RD-${RUN_TAG}-001"
MODERN_READER_SERIAL="MOD-RD-${RUN_TAG}-001"
ACCESS_EVENT_ID_1="wg-access-${RUN_TAG}-1"
ACCESS_EVENT_ID_2="wg-access-${RUN_TAG}-2"
ACCESS_IDEMPOTENCY_KEY="wg-idem-${RUN_TAG}"

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

echo "== import serial inventory for gateway/readers =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg gw "${GW_SERIAL}" \
  --arg legacy "${LEGACY_READER_SERIAL}" \
  --arg modern "${MODERN_READER_SERIAL}" \
  '{tenant_id:$tenant,items:[
    {serial_number:$gw,product_type:"gateway",batch_code:"qa-wg-poc",source:"factory"},
    {serial_number:$legacy,product_type:"reader",batch_code:"qa-wg-poc",source:"legacy"},
    {serial_number:$modern,product_type:"reader",batch_code:"qa-wg-poc",source:"factory"}
  ]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import serial inventory"

echo "== bootstrap register gateway to get device token =="
BOOTSTRAP_REGISTER_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
BOOTSTRAP_REGISTER_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/gateway/register" \
  -H "X-Bootstrap-Token: ${GATEWAY_BOOTSTRAP_TOKEN:-mistypass-dev-bootstrap-local-only-20260424}" \
  -H "Content-Type: application/json" \
  -d "${BOOTSTRAP_REGISTER_PAYLOAD}" \
  -w $'\n%{http_code}')"
split_response "${BOOTSTRAP_REGISTER_RAW}"
require_http_code "201" "gateway bootstrap register"
GW_ID="$(echo "${HTTP_BODY}" | jq -r '.gateway_id')"
DEVICE_TOKEN="$(echo "${HTTP_BODY}" | jq -r '.device_token')"
require_non_empty "${GW_ID}" "gateway_id"
require_non_empty "${DEVICE_TOKEN}" "device_token"

echo "== register legacy reader without protocol (expect default wiegand_26) =="
LEGACY_DEVICE_PAYLOAD="$(jq -nc \
  --arg serial "${LEGACY_READER_SERIAL}" \
  '{serial_number:$serial,kind:"legacy_reader",source:"legacy_integration",status:"online"}')"
LEGACY_DEVICE_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/devices?tenant_id=${TENANT_ID}" "${LEGACY_DEVICE_PAYLOAD}")"
split_response "${LEGACY_DEVICE_RAW}"
require_http_code "200" "register legacy reader"
LEGACY_PROTOCOL="$(echo "${HTTP_BODY}" | jq -r --arg serial "${LEGACY_READER_SERIAL}" '.devices[] | select(.serial_number == $serial) | .protocol')"
if [[ "${LEGACY_PROTOCOL}" != "wiegand_26" ]]; then
  echo "FAIL legacy reader protocol mismatch: expected wiegand_26 got ${LEGACY_PROTOCOL}"
  exit 1
fi

echo "== register modern reader without protocol (expect default osdp_v2) =="
MODERN_DEVICE_PAYLOAD="$(jq -nc \
  --arg serial "${MODERN_READER_SERIAL}" \
  '{serial_number:$serial,kind:"reader",source:"mistypass_procured",status:"online"}')"
MODERN_DEVICE_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/devices?tenant_id=${TENANT_ID}" "${MODERN_DEVICE_PAYLOAD}")"
split_response "${MODERN_DEVICE_RAW}"
require_http_code "200" "register modern reader"
MODERN_PROTOCOL="$(echo "${HTTP_BODY}" | jq -r --arg serial "${MODERN_READER_SERIAL}" '.devices[] | select(.serial_number == $serial) | .protocol')"
if [[ "${MODERN_PROTOCOL}" != "osdp_v2" ]]; then
  echo "FAIL modern reader protocol mismatch: expected osdp_v2 got ${MODERN_PROTOCOL}"
  exit 1
fi

echo "== probe legacy serial suggestions =="
PROBE_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/devices/probe-legacy?tenant_id=${TENANT_ID}")"
split_response "${PROBE_RAW}"
require_http_code "200" "probe legacy serials"
PROBE_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
if [[ "${PROBE_COUNT}" -lt 1 ]]; then
  echo "FAIL probe legacy serials: expected at least one suggestion"
  exit 1
fi
PROBE_FIRST="$(echo "${HTTP_BODY}" | jq -r '.items[0]')"
if [[ "${PROBE_FIRST}" != LEG-* ]]; then
  echo "FAIL probe legacy serials: unexpected prefix ${PROBE_FIRST}"
  exit 1
fi
PROBE_LEGACY_PROTOCOL="$(echo "${HTTP_BODY}" | jq -r '.governance.legacy_protocol')"
PROBE_UPGRADE_PROTOCOL="$(echo "${HTTP_BODY}" | jq -r '.governance.upgrade_protocol')"
PROBE_DEGRADED_ACTION="$(echo "${HTTP_BODY}" | jq -r '.governance.degraded_line_action')"
if [[ "${PROBE_LEGACY_PROTOCOL}" != "wiegand_26" || "${PROBE_UPGRADE_PROTOCOL}" != "osdp_v2" || -z "${PROBE_DEGRADED_ACTION}" || "${PROBE_DEGRADED_ACTION}" == "null" ]]; then
  echo "FAIL probe governance mismatch: legacy=${PROBE_LEGACY_PROTOCOL} upgrade=${PROBE_UPGRADE_PROTOCOL} action=${PROBE_DEGRADED_ACTION}"
  exit 1
fi

echo "== ingest access event via bootstrap token =="
ACCESS_PAYLOAD_1="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${ACCESS_EVENT_ID_1}" \
  --arg idem "${ACCESS_IDEMPOTENCY_KEY}" \
  --arg building "${BUILDING_ID}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$eid,idempotency_key:$idem,building_id:$building,area_id:"area_demo_001",type:"access_granted",actor:"legacy-reader",door_id:"door_demo_001",result:"accepted"}')"
ACCESS_RAW_1="$(bootstrap_with_token POST "/api/v1/gateway/events/access" "${ACCESS_PAYLOAD_1}")"
split_response "${ACCESS_RAW_1}"
require_http_code "202" "bootstrap access event #1"
ACCESS_SAVED_ID_1="$(echo "${HTTP_BODY}" | jq -r '.event_id')"
ACCESS_DEDUP_1="$(echo "${HTTP_BODY}" | jq -r '.deduplicated')"
INGESTED_TOTAL_1="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${ACCESS_DEDUP_1}" != "false" ]]; then
  echo "FAIL bootstrap access event #1: expected deduplicated=false got ${ACCESS_DEDUP_1}"
  exit 1
fi
require_non_empty "${ACCESS_SAVED_ID_1}" "bootstrap access event #1 event_id"

echo "== replay same idempotency key (expect deduplicated=true and same event id) =="
ACCESS_PAYLOAD_2="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${ACCESS_EVENT_ID_2}" \
  --arg idem "${ACCESS_IDEMPOTENCY_KEY}" \
  --arg building "${BUILDING_ID}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$eid,idempotency_key:$idem,building_id:$building,area_id:"area_demo_001",type:"access_granted",actor:"legacy-reader",door_id:"door_demo_001",result:"accepted"}')"
ACCESS_RAW_2="$(bootstrap_with_token POST "/api/v1/gateway/events/access" "${ACCESS_PAYLOAD_2}")"
split_response "${ACCESS_RAW_2}"
require_http_code "202" "bootstrap access event #2 replay"
ACCESS_SAVED_ID_2="$(echo "${HTTP_BODY}" | jq -r '.event_id')"
ACCESS_DEDUP_2="$(echo "${HTTP_BODY}" | jq -r '.deduplicated')"
INGESTED_TOTAL_2="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${ACCESS_DEDUP_2}" != "true" ]]; then
  echo "FAIL bootstrap access event #2 replay: expected deduplicated=true got ${ACCESS_DEDUP_2}"
  exit 1
fi
if [[ "${ACCESS_SAVED_ID_2}" != "${ACCESS_SAVED_ID_1}" ]]; then
  echo "FAIL replay event id mismatch: first=${ACCESS_SAVED_ID_1} second=${ACCESS_SAVED_ID_2}"
  exit 1
fi
if [[ "${INGESTED_TOTAL_2}" != "${INGESTED_TOTAL_1}" ]]; then
  echo "FAIL replay ingested_total mismatch: first=${INGESTED_TOTAL_1} second=${INGESTED_TOTAL_2}"
  exit 1
fi

echo "== verify list access events has single deduplicated record =="
LIST_ACCESS_RAW="$(api_with_auth GET "/api/v1/events/access?tenant_id=${TENANT_ID}")"
split_response "${LIST_ACCESS_RAW}"
require_http_code "200" "list access events"
EVENT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg id "${ACCESS_SAVED_ID_1}" '.items | map(select((.id // .event_id) == $id)) | length')"
if [[ "${EVENT_HITS}" != "1" ]]; then
  echo "FAIL list access events dedup check: expected 1 got ${EVENT_HITS}"
  exit 1
fi

echo "== verify access grant audit target normalized =="
ACCESS_AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=gateway_access_grant_recorded&source=gateway_access_event&limit=20")"
split_response "${ACCESS_AUDIT_RAW}"
require_http_code "200" "list gateway access grant audit"
ACCESS_AUDIT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg eid "${ACCESS_SAVED_ID_1}" '.items | map(select(.target | contains("event=\($eid)") and contains("queue=default") and contains("deduplicated=false"))) | length')"
if [[ "${ACCESS_AUDIT_HITS}" -lt 1 ]]; then
  echo "FAIL access grant audit normalized target missing for event ${ACCESS_SAVED_ID_1}"
  exit 1
fi

echo "PASS: gateway legacy wiegand poc regression complete gateway_id=${GW_ID} protocol=${LEGACY_PROTOCOL} probe_count=${PROBE_COUNT} dedup_event=${ACCESS_SAVED_ID_1}"
