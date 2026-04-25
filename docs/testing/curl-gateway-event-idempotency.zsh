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
    PORT="${API_PORT}" GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_event_idempotency.log 2>&1
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
  if [[ -f /tmp/mp_gateway_event_idempotency.log ]]; then
    tail -n 80 /tmp/mp_gateway_event_idempotency.log
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
GW_SERIAL="MP-GW-EVT-${RUN_TAG}"
ACCESS_EVENT_ID="gwea-replay-${RUN_TAG}"
DEVICE_EVENT_ID="gwed-replay-${RUN_TAG}"
ACCESS_REQ_ID="req-access-${RUN_TAG}"
DEVICE_REQ_ID="req-device-${RUN_TAG}"
OCCURRED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

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

echo "== import gateway serial and bootstrap register =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg sn "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-event-idempotency",source:"factory"}]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import gateway serial inventory"

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
echo "bootstrap: gateway_id=${GW_ID}"

echo "== access event replay idempotency =="
ACCESS_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${ACCESS_EVENT_ID}" \
  --arg rid "${ACCESS_REQ_ID}" \
  --arg b "${BUILDING_ID}" \
  --arg d "door_jkt_001" \
  --arg t "${OCCURRED_AT}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$rid,building_id:$b,door_id:$d,type:"access_granted",actor:"qa.replay",result:"success",occurred_at:$t}')"

ACCESS_RAW_1="$(bootstrap_with_token POST "/api/v1/gateway/events/access" "${ACCESS_PAYLOAD}")"
split_response "${ACCESS_RAW_1}"
require_http_code "202" "access event #1"
ACCESS_DEDUP_1="$(echo "${HTTP_BODY}" | jq -r '.deduplicated')"
ACCESS_QUEUE_TOTAL_1="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${ACCESS_DEDUP_1}" != "false" ]]; then
  echo "FAIL access event #1 expected deduplicated=false got ${ACCESS_DEDUP_1}"
  exit 1
fi
if [[ "${ACCESS_QUEUE_TOTAL_1}" != "1" ]]; then
  echo "FAIL access event #1 expected queue_progress.ingested_total=1 got ${ACCESS_QUEUE_TOTAL_1}"
  exit 1
fi

ACCESS_RAW_2="$(bootstrap_with_token POST "/api/v1/gateway/events/access" "${ACCESS_PAYLOAD}")"
split_response "${ACCESS_RAW_2}"
require_http_code "202" "access event #2 replay"
ACCESS_DEDUP_2="$(echo "${HTTP_BODY}" | jq -r '.deduplicated')"
ACCESS_QUEUE_TOTAL_2="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${ACCESS_DEDUP_2}" != "true" ]]; then
  echo "FAIL access event #2 expected deduplicated=true got ${ACCESS_DEDUP_2}"
  exit 1
fi
if [[ "${ACCESS_QUEUE_TOTAL_2}" != "${ACCESS_QUEUE_TOTAL_1}" ]]; then
  echo "FAIL access event #2 expected queue_progress to remain ${ACCESS_QUEUE_TOTAL_1} got ${ACCESS_QUEUE_TOTAL_2}"
  exit 1
fi
echo "access replay: dedup flags ${ACCESS_DEDUP_1} -> ${ACCESS_DEDUP_2}"

echo "== device event replay idempotency =="
DEVICE_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${DEVICE_EVENT_ID}" \
  --arg rid "${DEVICE_REQ_ID}" \
  --arg b "${BUILDING_ID}" \
  --arg t "${OCCURRED_AT}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$rid,building_id:$b,type:"gateway_event",detail:"rs485 timeout spike",result:"warning",occurred_at:$t}')"

DEVICE_RAW_1="$(bootstrap_with_token POST "/api/v1/gateway/events/device" "${DEVICE_PAYLOAD}")"
split_response "${DEVICE_RAW_1}"
require_http_code "202" "device event #1"
DEVICE_DEDUP_1="$(echo "${HTTP_BODY}" | jq -r '.deduplicated')"
DEVICE_QUEUE_TOTAL_1="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${DEVICE_DEDUP_1}" != "false" ]]; then
  echo "FAIL device event #1 expected deduplicated=false got ${DEVICE_DEDUP_1}"
  exit 1
fi
if [[ "${DEVICE_QUEUE_TOTAL_1}" != "2" ]]; then
  echo "FAIL device event #1 expected queue_progress.ingested_total=2 got ${DEVICE_QUEUE_TOTAL_1}"
  exit 1
fi

DEVICE_RAW_2="$(bootstrap_with_token POST "/api/v1/gateway/events/device" "${DEVICE_PAYLOAD}")"
split_response "${DEVICE_RAW_2}"
require_http_code "202" "device event #2 replay"
DEVICE_DEDUP_2="$(echo "${HTTP_BODY}" | jq -r '.deduplicated')"
DEVICE_QUEUE_TOTAL_2="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${DEVICE_DEDUP_2}" != "true" ]]; then
  echo "FAIL device event #2 expected deduplicated=true got ${DEVICE_DEDUP_2}"
  exit 1
fi
if [[ "${DEVICE_QUEUE_TOTAL_2}" != "${DEVICE_QUEUE_TOTAL_1}" ]]; then
  echo "FAIL device event #2 expected queue_progress to remain ${DEVICE_QUEUE_TOTAL_1} got ${DEVICE_QUEUE_TOTAL_2}"
  exit 1
fi
echo "device replay: dedup flags ${DEVICE_DEDUP_1} -> ${DEVICE_DEDUP_2}"

echo "== verify list endpoints no duplicate rows =="
LIST_ACCESS_RAW="$(api_with_auth GET "/api/v1/events/access?tenant_id=${TENANT_ID}")"
split_response "${LIST_ACCESS_RAW}"
require_http_code "200" "list access events"
ACCESS_ROW_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg eid "${ACCESS_EVENT_ID}" '[.items[] | select(.id == $eid)] | length')"
if [[ "${ACCESS_ROW_COUNT}" != "1" ]]; then
  echo "FAIL access events expected 1 row for ${ACCESS_EVENT_ID}, got ${ACCESS_ROW_COUNT}"
  exit 1
fi

LIST_DEVICE_RAW="$(api_with_auth GET "/api/v1/events/device?tenant_id=${TENANT_ID}")"
split_response "${LIST_DEVICE_RAW}"
require_http_code "200" "list device events"
DEVICE_ROW_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg eid "${DEVICE_EVENT_ID}" '[.items[] | select(.id == $eid)] | length')"
if [[ "${DEVICE_ROW_COUNT}" != "1" ]]; then
  echo "FAIL device events expected 1 row for ${DEVICE_EVENT_ID}, got ${DEVICE_ROW_COUNT}"
  exit 1
fi

echo "PASS: gateway bootstrap event idempotency regression complete"
