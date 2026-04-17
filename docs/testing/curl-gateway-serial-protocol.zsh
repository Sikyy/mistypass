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

function ensure_api_running() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "api: already running"
    return
  fi

  echo "api: starting local server"
  (
    cd api
    PORT="${API_PORT}" GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_serial_protocol_api.log 2>&1
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
  if [[ -f /tmp/mp_gateway_serial_protocol_api.log ]]; then
    tail -n 80 /tmp/mp_gateway_serial_protocol_api.log
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
GW_SERIAL_1="MP-GW-QA-${RUN_TAG}-A"
GW_SERIAL_2="MP-GW-QA-${RUN_TAG}-B"
GW_SERIAL_BAD="GW-${RUN_TAG}"
DEVICE_SERIAL_OSDP="RD-QA-${RUN_TAG}-001"
DEVICE_SERIAL_BLE="RD-QA-${RUN_TAG}-BLE"
DEVICE_SERIAL_RS485="RD-QA-${RUN_TAG}-RS485"

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

echo "== import serial inventory for gateways/readers =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg gw1 "${GW_SERIAL_1}" \
  --arg gw2 "${GW_SERIAL_2}" \
  --arg rd1 "${DEVICE_SERIAL_OSDP}" \
  --arg rd2 "${DEVICE_SERIAL_BLE}" \
  --arg rd3 "${DEVICE_SERIAL_RS485}" \
  '{tenant_id:$tenant,items:[
    {serial_number:$gw1,product_type:"gateway",batch_code:"qa-serial",source:"factory"},
    {serial_number:$gw2,product_type:"gateway",batch_code:"qa-serial",source:"factory"},
    {serial_number:$rd1,product_type:"reader",batch_code:"qa-serial",source:"factory"},
    {serial_number:$rd2,product_type:"reader",batch_code:"qa-serial",source:"factory"},
    {serial_number:$rd3,product_type:"reader",batch_code:"qa-serial",source:"factory"}
  ]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import serial inventory"

echo "== register gateway #1 with valid serial =="
REGISTER_1_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL_1}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:8}')"
REGISTER_1_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${REGISTER_1_PAYLOAD}")"
split_response "${REGISTER_1_RAW}"
require_http_code "201" "register gateway #1"
GW_ID_1="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${GW_ID_1}" "register gateway #1 id"
INVENTORY_CHECK_1_RAW="$(api_with_auth GET "/api/v1/gateways/serial-inventory?tenant_id=${TENANT_ID}&product_type=gateway&status=consumed")"
split_response "${INVENTORY_CHECK_1_RAW}"
require_http_code "200" "inventory consumed check #1"
GW1_CONSUMED_GATEWAY="$(echo "${HTTP_BODY}" | jq -r --arg sn "${GW_SERIAL_1}" '.items[] | select(.serial_number == $sn) | .consumed_gateway_id')"
if [[ "${GW1_CONSUMED_GATEWAY}" != "${GW_ID_1}" ]]; then
  echo "FAIL gateway serial consume mismatch: expected ${GW_ID_1} got ${GW1_CONSUMED_GATEWAY}"
  exit 1
fi

echo "== register duplicate serial (case-insensitive) should conflict =="
DUP_PAYLOAD="$(jq -nc \
  --arg sn "$(echo "${GW_SERIAL_1}" | tr 'A-Z' 'a-z')" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
DUP_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${DUP_PAYLOAD}")"
split_response "${DUP_RAW}"
require_http_code "409" "register duplicate gateway serial"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "gateway serial_number already registered" ]]; then
  echo "FAIL duplicate serial error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "== register invalid serial format should reject =="
BAD_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL_BAD}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
BAD_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${BAD_PAYLOAD}")"
split_response "${BAD_RAW}"
require_http_code "400" "register invalid gateway serial"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "serial_number format is invalid" ]]; then
  echo "FAIL invalid serial error mismatch: ${HTTP_BODY}"
  exit 1
fi

REGISTER_2_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL_2}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:8}')"

echo "== freeze gateway #2 serial then verify register blocked =="
FREEZE_GW2_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,status:"frozen"}')"
FREEZE_GW2_RAW="$(api_with_auth PATCH "/api/v1/gateways/serial-inventory/${GW_SERIAL_2}/status" "${FREEZE_GW2_PAYLOAD}")"
split_response "${FREEZE_GW2_RAW}"
require_http_code "200" "freeze gateway serial #2"
REGISTER_2_FROZEN_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${REGISTER_2_PAYLOAD}")"
split_response "${REGISTER_2_FROZEN_RAW}"
require_http_code "409" "register frozen gateway serial"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "serial_number is not available" ]]; then
  echo "FAIL frozen gateway serial error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "== set gateway #2 serial back to available =="
THAW_GW2_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,status:"available"}')"
THAW_GW2_RAW="$(api_with_auth PATCH "/api/v1/gateways/serial-inventory/${GW_SERIAL_2}/status" "${THAW_GW2_PAYLOAD}")"
split_response "${THAW_GW2_RAW}"
require_http_code "200" "thaw gateway serial #2"

echo "== register gateway #2 for cross-gateway serial conflict =="
REGISTER_2_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${REGISTER_2_PAYLOAD}")"
split_response "${REGISTER_2_RAW}"
require_http_code "201" "register gateway #2"
GW_ID_2="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${GW_ID_2}" "register gateway #2 id"

echo "== register legacy device without protocol (expect default wiegand_26) =="
DEVICE_SERIAL_LEG="LEG-QA-${RUN_TAG}-001"
LEGACY_PAYLOAD="$(jq -nc \
  --arg sn "${DEVICE_SERIAL_LEG}" \
  '{serial_number:$sn,kind:"legacy_reader",source:"legacy_integration",status:"online"}')"
LEGACY_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices" "${LEGACY_PAYLOAD}")"
split_response "${LEGACY_RAW}"
require_http_code "200" "register legacy device"
LEGACY_PROTOCOL="$(echo "${HTTP_BODY}" | jq -r --arg sn "${DEVICE_SERIAL_LEG}" '.devices[] | select(.serial_number == $sn) | .protocol')"
if [[ "${LEGACY_PROTOCOL}" != "wiegand_26" ]]; then
  echo "FAIL legacy default protocol: expected wiegand_26 got ${LEGACY_PROTOCOL}"
  exit 1
fi

echo "== register modern reader without protocol (expect default osdp_v2) =="
OSDP_PAYLOAD="$(jq -nc \
  --arg sn "${DEVICE_SERIAL_OSDP}" \
  '{serial_number:$sn,kind:"reader",source:"mistypass_procured",status:"online"}')"
OSDP_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices" "${OSDP_PAYLOAD}")"
split_response "${OSDP_RAW}"
require_http_code "200" "register modern reader"
OSDP_PROTOCOL="$(echo "${HTTP_BODY}" | jq -r --arg sn "${DEVICE_SERIAL_OSDP}" '.devices[] | select(.serial_number == $sn) | .protocol')"
if [[ "${OSDP_PROTOCOL}" != "osdp_v2" ]]; then
  echo "FAIL modern default protocol: expected osdp_v2 got ${OSDP_PROTOCOL}"
  exit 1
fi
INVENTORY_CHECK_RD_RAW="$(api_with_auth GET "/api/v1/gateways/serial-inventory?tenant_id=${TENANT_ID}&product_type=reader&status=consumed")"
split_response "${INVENTORY_CHECK_RD_RAW}"
require_http_code "200" "inventory consumed check reader"
RD1_CONSUMED_GATEWAY="$(echo "${HTTP_BODY}" | jq -r --arg sn "${DEVICE_SERIAL_OSDP}" '.items[] | select(.serial_number == $sn) | .consumed_gateway_id')"
if [[ "${RD1_CONSUMED_GATEWAY}" != "${GW_ID_1}" ]]; then
  echo "FAIL reader serial consume mismatch: expected ${GW_ID_1} got ${RD1_CONSUMED_GATEWAY}"
  exit 1
fi

echo "== freeze BLE reader serial then verify attach blocked =="
FREEZE_BLE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,status:"frozen"}')"
FREEZE_BLE_RAW="$(api_with_auth PATCH "/api/v1/gateways/serial-inventory/${DEVICE_SERIAL_BLE}/status" "${FREEZE_BLE_PAYLOAD}")"
split_response "${FREEZE_BLE_RAW}"
require_http_code "200" "freeze BLE reader serial"
BLE_FROZEN_PAYLOAD="$(jq -nc \
  --arg sn "${DEVICE_SERIAL_BLE}" \
  '{serial_number:$sn,kind:"reader",source:"mistypass_procured",protocol:"ble",status:"offline"}')"
BLE_FROZEN_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices" "${BLE_FROZEN_PAYLOAD}")"
split_response "${BLE_FROZEN_RAW}"
require_http_code "409" "attach frozen BLE reader serial"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "device serial_number is not available" ]]; then
  echo "FAIL frozen BLE reader error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "== set BLE reader serial back to available =="
THAW_BLE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,status:"available"}')"
THAW_BLE_RAW="$(api_with_auth PATCH "/api/v1/gateways/serial-inventory/${DEVICE_SERIAL_BLE}/status" "${THAW_BLE_PAYLOAD}")"
split_response "${THAW_BLE_RAW}"
require_http_code "200" "thaw BLE reader serial"

echo "== register reader with explicit BLE protocol =="
BLE_PAYLOAD="$(jq -nc \
  --arg sn "${DEVICE_SERIAL_BLE}" \
  '{serial_number:$sn,kind:"reader",source:"mistypass_procured",protocol:"ble",status:"offline"}')"
BLE_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices" "${BLE_PAYLOAD}")"
split_response "${BLE_RAW}"
require_http_code "200" "register reader ble"
BLE_PROTOCOL="$(echo "${HTTP_BODY}" | jq -r --arg sn "${DEVICE_SERIAL_BLE}" '.devices[] | select(.serial_number == $sn) | .protocol')"
if [[ "${BLE_PROTOCOL}" != "ble" ]]; then
  echo "FAIL explicit ble protocol mismatch: got ${BLE_PROTOCOL}"
  exit 1
fi

echo "== register reader with explicit RS485 protocol =="
RS485_PAYLOAD="$(jq -nc \
  --arg sn "${DEVICE_SERIAL_RS485}" \
  '{serial_number:$sn,kind:"reader",source:"mistypass_procured",protocol:"rs485",status:"online",rs485_config:{baud_rate:19200,parity:"even",stop_bits:1,device_address:10,timeout_ms:1200}}')"
RS485_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices" "${RS485_PAYLOAD}")"
split_response "${RS485_RAW}"
require_http_code "200" "register reader rs485"
RS485_DEVICE_ID="$(echo "${HTTP_BODY}" | jq -r --arg sn "${DEVICE_SERIAL_RS485}" '.devices[] | select(.serial_number == $sn) | .id')"
require_non_empty "${RS485_DEVICE_ID}" "rs485 device id"
RS485_PROTOCOL="$(echo "${HTTP_BODY}" | jq -r --arg sn "${DEVICE_SERIAL_RS485}" '.devices[] | select(.serial_number == $sn) | .protocol')"
if [[ "${RS485_PROTOCOL}" != "rs485" ]]; then
  echo "FAIL explicit rs485 protocol mismatch: got ${RS485_PROTOCOL}"
  exit 1
fi
RS485_BAUD="$(echo "${HTTP_BODY}" | jq -r --arg sn "${DEVICE_SERIAL_RS485}" '.devices[] | select(.serial_number == $sn) | .rs485_config.baud_rate')"
RS485_PARITY="$(echo "${HTTP_BODY}" | jq -r --arg sn "${DEVICE_SERIAL_RS485}" '.devices[] | select(.serial_number == $sn) | .rs485_config.parity')"
if [[ "${RS485_BAUD}" != "19200" || "${RS485_PARITY}" != "even" ]]; then
  echo "FAIL explicit rs485 config mismatch: baud=${RS485_BAUD} parity=${RS485_PARITY}"
  exit 1
fi

echo "== report rs485 telemetry and verify health counters =="
RS485_TELEMETRY_1_PAYLOAD="$(jq -nc '{retries:2,timeouts:2,collisions:1,last_error:"crc_error"}')"
RS485_TELEMETRY_1_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices/${RS485_DEVICE_ID}/rs485/telemetry" "${RS485_TELEMETRY_1_PAYLOAD}")"
split_response "${RS485_TELEMETRY_1_RAW}"
require_http_code "200" "report rs485 telemetry #1"
RS485_ALERTED_1="$(echo "${HTTP_BODY}" | jq -r '.alerted')"
RS485_TIMEOUT_1="$(echo "${HTTP_BODY}" | jq -r '.device.rs485_health.timeout_count')"
RS485_CONSEC_1="$(echo "${HTTP_BODY}" | jq -r '.device.rs485_health.consecutive_timeouts')"
if [[ "${RS485_ALERTED_1}" != "false" || "${RS485_TIMEOUT_1}" != "2" || "${RS485_CONSEC_1}" != "2" ]]; then
  echo "FAIL rs485 telemetry #1 mismatch: alerted=${RS485_ALERTED_1} timeout=${RS485_TIMEOUT_1} consecutive=${RS485_CONSEC_1}"
  exit 1
fi

RS485_TELEMETRY_2_PAYLOAD="$(jq -nc '{timeouts:1,last_error:"line_timeout"}')"
RS485_TELEMETRY_2_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices/${RS485_DEVICE_ID}/rs485/telemetry" "${RS485_TELEMETRY_2_PAYLOAD}")"
split_response "${RS485_TELEMETRY_2_RAW}"
require_http_code "200" "report rs485 telemetry #2"
RS485_ALERTED_2="$(echo "${HTTP_BODY}" | jq -r '.alerted')"
RS485_CONSEC_2="$(echo "${HTTP_BODY}" | jq -r '.device.rs485_health.consecutive_timeouts')"
RS485_ALERT_LEVEL_2="$(echo "${HTTP_BODY}" | jq -r '.telemetry.alert_level')"
RS485_LINE_QUALITY_2="$(echo "${HTTP_BODY}" | jq -r '.telemetry.line_quality')"
RS485_GOV_ACTION_2="$(echo "${HTTP_BODY}" | jq -r '.telemetry.governance_action')"
if [[ "${RS485_ALERTED_2}" != "true" || "${RS485_CONSEC_2}" != "3" ]]; then
  echo "FAIL rs485 telemetry #2 mismatch: alerted=${RS485_ALERTED_2} consecutive=${RS485_CONSEC_2}"
  exit 1
fi
if [[ "${RS485_ALERT_LEVEL_2}" != "warning" || "${RS485_LINE_QUALITY_2}" != "degraded" || -z "${RS485_GOV_ACTION_2}" || "${RS485_GOV_ACTION_2}" == "null" ]]; then
  echo "FAIL rs485 telemetry summary mismatch: alert_level=${RS485_ALERT_LEVEL_2} line_quality=${RS485_LINE_QUALITY_2} action=${RS485_GOV_ACTION_2}"
  exit 1
fi

RS485_AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=gateway_rs485_health_alert&source=gateway_rs485&limit=20")"
split_response "${RS485_AUDIT_RAW}"
require_http_code "200" "list rs485 alert audit logs"
RS485_AUDIT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg did "${RS485_DEVICE_ID}" '.items | map(select(.target | contains($did))) | length')"
if [[ "${RS485_AUDIT_HITS}" -lt 1 ]]; then
  echo "FAIL rs485 alert audit: expected at least one alert for device ${RS485_DEVICE_ID}"
  exit 1
fi
PROTOCOL_AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=gateway_protocol_health_alert&source=gateway_protocol_health&limit=20")"
split_response "${PROTOCOL_AUDIT_RAW}"
require_http_code "200" "list protocol health audit logs"
PROTOCOL_AUDIT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg did "${RS485_DEVICE_ID}" '.items | map(select(.target | contains($did) and contains("alert_level=warning"))) | length')"
if [[ "${PROTOCOL_AUDIT_HITS}" -lt 1 ]]; then
  echo "FAIL protocol health audit: expected warning alert target for device ${RS485_DEVICE_ID}"
  exit 1
fi

echo "== scrap BLE reader serial then verify attach blocked =="
SCRAP_BLE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,status:"scrapped"}')"
SCRAP_BLE_RAW="$(api_with_auth PATCH "/api/v1/gateways/serial-inventory/${DEVICE_SERIAL_BLE}/status" "${SCRAP_BLE_PAYLOAD}")"
split_response "${SCRAP_BLE_RAW}"
require_http_code "200" "scrap BLE reader serial"
BLE_SCRAP_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices" "${BLE_PAYLOAD}")"
split_response "${BLE_SCRAP_RAW}"
require_http_code "409" "attach scrapped BLE reader serial"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "device serial_number is not available" ]]; then
  echo "FAIL scrapped BLE reader error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "== register invalid protocol should reject =="
DEVICE_SERIAL_BAD_PROTOCOL="RD-QA-${RUN_TAG}-BADP"
BAD_PROTOCOL_PAYLOAD="$(jq -nc \
  --arg sn "${DEVICE_SERIAL_BAD_PROTOCOL}" \
  '{serial_number:$sn,kind:"reader",source:"mistypass_procured",protocol:"uart",status:"online"}')"
BAD_PROTOCOL_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices" "${BAD_PROTOCOL_PAYLOAD}")"
split_response "${BAD_PROTOCOL_RAW}"
require_http_code "400" "register invalid protocol"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "invalid gateway device protocol" ]]; then
  echo "FAIL invalid protocol error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "== register BLE with rs485_config should reject =="
DEVICE_SERIAL_BAD_RS485_CFG="RD-QA-${RUN_TAG}-BADRS485"
BAD_RS485_CFG_PAYLOAD="$(jq -nc \
  --arg sn "${DEVICE_SERIAL_BAD_RS485_CFG}" \
  '{serial_number:$sn,kind:"reader",source:"mistypass_procured",protocol:"ble",status:"online",rs485_config:{baud_rate:9600}}')"
BAD_RS485_CFG_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_1}/devices" "${BAD_RS485_CFG_PAYLOAD}")"
split_response "${BAD_RS485_CFG_RAW}"
require_http_code "400" "register ble with rs485_config"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "rs485_config requires protocol=rs485" ]]; then
  echo "FAIL rs485 config protocol mismatch error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "== register duplicated device serial on another gateway should conflict =="
CONFLICT_PAYLOAD="$(jq -nc \
  --arg sn "${DEVICE_SERIAL_OSDP}" \
  '{serial_number:$sn,kind:"reader",source:"mistypass_procured",status:"online"}')"
CONFLICT_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID_2}/devices" "${CONFLICT_PAYLOAD}")"
split_response "${CONFLICT_RAW}"
require_http_code "409" "cross-gateway device serial conflict"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "device serial_number already registered on another gateway" ]]; then
  echo "FAIL device serial conflict error mismatch: ${HTTP_BODY}"
  exit 1
fi

echo "PASS: gateway serial/protocol regression complete"
