#!/usr/bin/env zsh
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
DOOR_ID="${DOOR_ID:-door_jkt_001}"
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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_employee_api.log 2>&1
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
  if [[ -f /tmp/mp_gateway_employee_api.log ]]; then
    tail -n 80 /tmp/mp_gateway_employee_api.log
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
GW_SERIAL="MP-GW-QA-${RUN_TAG}"
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
echo "login: ok"

echo "== gateway register (device_capacity=4) =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg gw "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$gw,product_type:"gateway",batch_code:"qa-regression",source:"factory"}]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import gateway serial inventory"

REGISTER_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
REGISTER_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${REGISTER_PAYLOAD}")"
split_response "${REGISTER_RAW}"
require_http_code "201" "register gateway"
GW_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${GW_ID}" "register.gateway_id"
REGISTER_CAPACITY="$(echo "${HTTP_BODY}" | jq -r '.device_capacity')"
if [[ "${REGISTER_CAPACITY}" != "4" ]]; then
  echo "FAIL register: expected device_capacity=4 got ${REGISTER_CAPACITY}"
  exit 1
fi
echo "register: gateway_id=${GW_ID} device_capacity=${REGISTER_CAPACITY}"

echo "== mount devices (fill capacity and exceed) =="
for idx in 1 2 3 4; do
  DEV_SERIAL="LEG-QA-${RUN_TAG}-${idx}"
  DEVICE_PAYLOAD="$(jq -nc \
    --arg sn "${DEV_SERIAL}" \
    '{serial_number:$sn,kind:"legacy_reader",source:"legacy_integration",status:"online"}')"
  DEVICE_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/devices" "${DEVICE_PAYLOAD}")"
  split_response "${DEVICE_RAW}"
  require_http_code "200" "register device #${idx}"
  DEV_COUNT="$(echo "${HTTP_BODY}" | jq -r '.devices | length')"
  echo "device#${idx}: devices_count=${DEV_COUNT}"
done

EXCEED_PAYLOAD="$(jq -nc \
  --arg sn "LEG-QA-${RUN_TAG}-5" \
  '{serial_number:$sn,kind:"legacy_reader",source:"legacy_integration",status:"online"}')"
EXCEED_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/devices" "${EXCEED_PAYLOAD}")"
split_response "${EXCEED_RAW}"
require_http_code "400" "register device #5 exceed capacity"
EXCEED_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
if [[ "${EXCEED_ERROR}" != "gateway device capacity exceeded" ]]; then
  echo "FAIL exceed capacity: unexpected error ${EXCEED_ERROR}"
  exit 1
fi
echo "device#5: rejected with expected error='${EXCEED_ERROR}'"

echo "== bind + unbind door =="
BIND_PAYLOAD="$(jq -nc --arg door "${DOOR_ID}" '{door_id:$door}')"
BIND_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/bind-door" "${BIND_PAYLOAD}")"
split_response "${BIND_RAW}"
require_http_code "200" "bind door"
BOUND_AFTER_BIND="$(echo "${HTTP_BODY}" | jq -r --arg door "${DOOR_ID}" '(.bound_door_ids // []) | index($door)')"
if [[ "${BOUND_AFTER_BIND}" == "null" ]]; then
  echo "FAIL bind door: ${DOOR_ID} not present in bound_door_ids"
  exit 1
fi
echo "bind: bound_door_ids=$(echo "${HTTP_BODY}" | jq -c '.bound_door_ids')"

UNBIND_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/unbind-door" "${BIND_PAYLOAD}")"
split_response "${UNBIND_RAW}"
require_http_code "200" "unbind door"
BOUND_AFTER_UNBIND="$(echo "${HTTP_BODY}" | jq -r --arg door "${DOOR_ID}" '(.bound_door_ids // []) | index($door)')"
if [[ "${BOUND_AFTER_UNBIND}" != "null" ]]; then
  echo "FAIL unbind door: ${DOOR_ID} still present in bound_door_ids"
  exit 1
fi
echo "unbind: bound_door_ids=$(echo "${HTTP_BODY}" | jq -c '.bound_door_ids // []')"

echo "== reboot + post-state =="
REBOOT_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/reboot")"
split_response "${REBOOT_RAW}"
require_http_code "202" "reboot"
REBOOT_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
REBOOT_COMMAND="$(echo "${HTTP_BODY}" | jq -r '.command')"
if [[ "${REBOOT_STATUS}" != "queued" || "${REBOOT_COMMAND}" != "reboot" ]]; then
  echo "FAIL reboot ack: command=${REBOOT_COMMAND} status=${REBOOT_STATUS}"
  exit 1
fi
echo "reboot: ack_status=${REBOOT_STATUS}"

LIST_RAW="$(api_with_auth GET "/api/v1/gateways?tenant_id=${TENANT_ID}")"
split_response "${LIST_RAW}"
require_http_code "200" "list gateways"
GW_ROW="$(echo "${HTTP_BODY}" | jq -c --arg gid "${GW_ID}" '.items[] | select(.id == $gid)')"
require_non_empty "${GW_ROW}" "list gateway row"
GW_STATUS="$(echo "${GW_ROW}" | jq -r '.status')"
GW_LAST_SEEN_AT="$(echo "${GW_ROW}" | jq -r '.last_seen_at')"
GW_DEVICE_CAPACITY="$(echo "${GW_ROW}" | jq -r '.device_capacity')"
GW_DEVICE_TOTAL="$(echo "${GW_ROW}" | jq -r '.devices | length')"
GW_DEVICE_ONLINE="$(echo "${GW_ROW}" | jq -r '[.devices[] | select(.status == "online")] | length')"
GW_BOUND_TOTAL="$(echo "${GW_ROW}" | jq -r '(.bound_door_ids // []) | length')"
if [[ "${GW_STATUS}" != "online" ]]; then
  echo "FAIL gateway status after reboot: ${GW_STATUS}"
  exit 1
fi
require_non_empty "${GW_LAST_SEEN_AT}" "gateway.last_seen_at"
echo "gateway_state: status=${GW_STATUS} capacity=${GW_DEVICE_CAPACITY} online_devices=${GW_DEVICE_ONLINE}/${GW_DEVICE_TOTAL} bound_doors=${GW_BOUND_TOTAL} last_seen_at=${GW_LAST_SEEN_AT}"

echo "== enterprise employees list =="
EMP_RAW="$(api_with_auth GET "/api/v1/enterprise/employees?tenant_id=${TENANT_ID}")"
split_response "${EMP_RAW}"
require_http_code "200" "list enterprise employees"
EMP_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
if [[ "${EMP_TOTAL}" -lt 1 ]]; then
  echo "FAIL employees list: expected at least 1 item"
  exit 1
fi
EMP_SAMPLE="$(echo "${HTTP_BODY}" | jq -c '.items[0] | {id,full_name,email,department,job_title,access_role,building_id,status}')"
echo "employees: total=${EMP_TOTAL} sample=${EMP_SAMPLE}"

echo "PASS: gateway + enterprise employee curl regression complete"
