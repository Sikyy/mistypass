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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_event_batch_replay.log 2>&1
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
  if [[ -f /tmp/mp_gateway_event_batch_replay.log ]]; then
    tail -n 80 /tmp/mp_gateway_event_batch_replay.log
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
GW_SERIAL="MP-GW-BATCH-${RUN_TAG}"
A1="gwea-batch-${RUN_TAG}-1"
A2="gwea-batch-${RUN_TAG}-2"
D1="gwed-batch-${RUN_TAG}-1"
T0="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

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

echo "== import gateway serial and bootstrap register =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg sn "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-event-batch",source:"factory"}]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import gateway serial inventory"

BOOTSTRAP_REGISTER_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
BOOTSTRAP_REGISTER_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/gateway/register" \
  -H "Content-Type: application/json" \
  -d "${BOOTSTRAP_REGISTER_PAYLOAD}" \
  -w $'\n%{http_code}')"
split_response "${BOOTSTRAP_REGISTER_RAW}"
require_http_code "201" "gateway bootstrap register"
GW_ID="$(echo "${HTTP_BODY}" | jq -r '.gateway_id')"
DEVICE_TOKEN="$(echo "${HTTP_BODY}" | jq -r '.device_token')"
require_non_empty "${GW_ID}" "gateway_id"
require_non_empty "${DEVICE_TOKEN}" "device_token"

BATCH_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg b "${BUILDING_ID}" \
  --arg a1 "${A1}" \
  --arg a2 "${A2}" \
  --arg d1 "${D1}" \
  --arg t "${T0}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    access_events:[
      {event_id:$a1,request_id:("rq-"+$a1),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_001",type:"access_granted",actor:"qa.batch.1",result:"success",occurred_at:$t},
      {event_id:$a2,request_id:("rq-"+$a2),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_002",type:"access_denied",actor:"qa.batch.2",result:"denied",occurred_at:$t}
    ],
    device_events:[
      {event_id:$d1,request_id:("rq-"+$d1),building_id:$b,type:"gateway_event",detail:"batch replay test",result:"warning",occurred_at:$t}
    ]
  }')"

echo "== first batch upload =="
BATCH_RAW_1="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_PAYLOAD}")"
split_response "${BATCH_RAW_1}"
require_http_code "202" "batch upload #1"
A_CREATED_1="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
A_DEDUP_1="$(echo "${HTTP_BODY}" | jq -r '.access.deduplicated')"
D_CREATED_1="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
D_DEDUP_1="$(echo "${HTTP_BODY}" | jq -r '.device.deduplicated')"
if [[ "${A_CREATED_1}" != "2" || "${A_DEDUP_1}" != "0" || "${D_CREATED_1}" != "1" || "${D_DEDUP_1}" != "0" ]]; then
  echo "FAIL batch upload #1 unexpected counters: access(created=${A_CREATED_1},dedup=${A_DEDUP_1}) device(created=${D_CREATED_1},dedup=${D_DEDUP_1})"
  exit 1
fi

echo "== replay same batch =="
BATCH_RAW_2="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_PAYLOAD}")"
split_response "${BATCH_RAW_2}"
require_http_code "202" "batch upload #2 replay"
A_CREATED_2="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
A_DEDUP_2="$(echo "${HTTP_BODY}" | jq -r '.access.deduplicated')"
D_CREATED_2="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
D_DEDUP_2="$(echo "${HTTP_BODY}" | jq -r '.device.deduplicated')"
if [[ "${A_CREATED_2}" != "0" || "${A_DEDUP_2}" != "2" || "${D_CREATED_2}" != "0" || "${D_DEDUP_2}" != "1" ]]; then
  echo "FAIL batch upload #2 unexpected counters: access(created=${A_CREATED_2},dedup=${A_DEDUP_2}) device(created=${D_CREATED_2},dedup=${D_DEDUP_2})"
  exit 1
fi

echo "== verify list endpoints no duplicates =="
LIST_ACCESS_RAW="$(api_with_auth GET "/api/v1/events/access?tenant_id=${TENANT_ID}")"
split_response "${LIST_ACCESS_RAW}"
require_http_code "200" "list access events"
A1_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${A1}" '[.items[] | select(.id == $id)] | length')"
A2_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${A2}" '[.items[] | select(.id == $id)] | length')"
if [[ "${A1_COUNT}" != "1" || "${A2_COUNT}" != "1" ]]; then
  echo "FAIL access list duplicate check: a1=${A1_COUNT} a2=${A2_COUNT}"
  exit 1
fi

LIST_DEVICE_RAW="$(api_with_auth GET "/api/v1/events/device?tenant_id=${TENANT_ID}")"
split_response "${LIST_DEVICE_RAW}"
require_http_code "200" "list device events"
D1_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${D1}" '[.items[] | select(.id == $id)] | length')"
if [[ "${D1_COUNT}" != "1" ]]; then
  echo "FAIL device list duplicate check: d1=${D1_COUNT}"
  exit 1
fi

echo "PASS: gateway event batch replay regression complete"
