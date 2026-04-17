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
    cd /Users/siky/code/MistyPass/api
    PORT="${API_PORT}" GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_door_io_loop.log 2>&1
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
  if [[ -f /tmp/mp_gateway_door_io_loop.log ]]; then
    tail -n 80 /tmp/mp_gateway_door_io_loop.log
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
GW_SERIAL="MP-GW-DOORIO-${RUN_TAG}"
ACCESS_EVENT_ID="door-access-${RUN_TAG}"
REX_EVENT_ID="door-rex-${RUN_TAG}"
TAMPER_EVENT_ID="door-tamper-${RUN_TAG}"
TIMEOUT_EVENT_ID="door-timeout-${RUN_TAG}"
TAMPER_REPLAY_EVENT_ID="door-tamper-replay-${RUN_TAG}"

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

echo "== import gateway serial inventory =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg sn "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-door-io",source:"factory"}]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import gateway serial"

echo "== bootstrap register gateway =="
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

echo "== ingest relay unlock access event =="
ACCESS_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${ACCESS_EVENT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$eid,idempotency_key:$eid,building_id:$building,area_id:"area_demo_001",type:"access_granted",actor:"door-controller",door_id:"door_jkt_001",result:"accepted"}')"
ACCESS_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/access" "${ACCESS_PAYLOAD}")"
split_response "${ACCESS_RAW}"
require_http_code "202" "access relay unlock"
ACCESS_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.deduplicated')"
ACCESS_INGEST_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${ACCESS_DEDUP}" != "false" || "${ACCESS_INGEST_TOTAL}" != "1" ]]; then
  echo "FAIL access relay unlock queue progress mismatch dedup=${ACCESS_DEDUP} total=${ACCESS_INGEST_TOTAL}"
  exit 1
fi

echo "== ingest device events: rex/tamper/timeout =="
REX_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${REX_EVENT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$eid,idempotency_key:$eid,building_id:$building,type:"rex_triggered",detail:"rex pressed",result:"accepted"}')"
REX_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/device" "${REX_PAYLOAD}")"
split_response "${REX_RAW}"
require_http_code "202" "device rex"
REX_INGEST_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${REX_INGEST_TOTAL}" != "2" ]]; then
  echo "FAIL device rex queue progress mismatch total=${REX_INGEST_TOTAL}"
  exit 1
fi

TAMPER_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${TAMPER_EVENT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$eid,idempotency_key:$eid,building_id:$building,type:"tamper_triggered",detail:"panel opened",result:"warning"}')"
TAMPER_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/device" "${TAMPER_PAYLOAD}")"
split_response "${TAMPER_RAW}"
require_http_code "202" "device tamper"
TAMPER_INGEST_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${TAMPER_INGEST_TOTAL}" != "3" ]]; then
  echo "FAIL device tamper queue progress mismatch total=${TAMPER_INGEST_TOTAL}"
  exit 1
fi

TIMEOUT_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${TIMEOUT_EVENT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$eid,idempotency_key:$eid,building_id:$building,type:"door_timeout",detail:"door held open",result:"warning"}')"
TIMEOUT_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/device" "${TIMEOUT_PAYLOAD}")"
split_response "${TIMEOUT_RAW}"
require_http_code "202" "device timeout"
TIMEOUT_INGEST_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${TIMEOUT_INGEST_TOTAL}" != "4" ]]; then
  echo "FAIL device timeout queue progress mismatch total=${TIMEOUT_INGEST_TOTAL}"
  exit 1
fi

echo "== replay tamper idempotency key (expect deduplicated=true, queue total unchanged) =="
TAMPER_REPLAY_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${TAMPER_REPLAY_EVENT_ID}" \
  --arg idem "${TAMPER_EVENT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:$eid,idempotency_key:$idem,building_id:$building,type:"tamper_triggered",detail:"panel opened replay",result:"warning"}')"
TAMPER_REPLAY_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/device" "${TAMPER_REPLAY_PAYLOAD}")"
split_response "${TAMPER_REPLAY_RAW}"
require_http_code "202" "device tamper replay"
TAMPER_REPLAY_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.deduplicated')"
TAMPER_REPLAY_INGEST_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_progress.ingested_total')"
if [[ "${TAMPER_REPLAY_DEDUP}" != "true" || "${TAMPER_REPLAY_INGEST_TOTAL}" != "4" ]]; then
  echo "FAIL device tamper replay mismatch dedup=${TAMPER_REPLAY_DEDUP} total=${TAMPER_REPLAY_INGEST_TOTAL}"
  exit 1
fi

echo "== verify device event list contains rex/tamper/timeout once =="
LIST_DEVICE_RAW="$(api_with_auth GET "/api/v1/events/device?tenant_id=${TENANT_ID}")"
split_response "${LIST_DEVICE_RAW}"
require_http_code "200" "list device events"
for eid in "${REX_EVENT_ID}" "${TAMPER_EVENT_ID}" "${TIMEOUT_EVENT_ID}"; do
  hits="$(echo "${HTTP_BODY}" | jq -r --arg id "${eid}" '.items | map(select((.id // .event_id) == $id)) | length')"
  if [[ "${hits}" != "1" ]]; then
    echo "FAIL list device events missing or duplicated event=${eid} hits=${hits}"
    exit 1
  fi
done

echo "== verify audit action mapping for rex/tamper/timeout =="
REX_AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=gateway_rex_event_recorded&source=gateway_device_event&limit=20")"
split_response "${REX_AUDIT_RAW}"
require_http_code "200" "audit rex"
REX_AUDIT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg eid "${REX_EVENT_ID}" '.items | map(select(.target | contains("event=\($eid)") and contains("queue=default") and contains("deduplicated=false"))) | length')"
if [[ "${REX_AUDIT_HITS}" -lt 1 ]]; then
  echo "FAIL audit rex target mismatch"
  exit 1
fi

TAMPER_AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=gateway_tamper_event_recorded&source=gateway_device_event&limit=20")"
split_response "${TAMPER_AUDIT_RAW}"
require_http_code "200" "audit tamper"
TAMPER_AUDIT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg eid "${TAMPER_EVENT_ID}" '.items | map(select(.target | contains("event=\($eid)") and contains("queue=default") and contains("deduplicated=false"))) | length')"
if [[ "${TAMPER_AUDIT_HITS}" -lt 1 ]]; then
  echo "FAIL audit tamper target mismatch"
  exit 1
fi

TIMEOUT_AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=gateway_door_timeout_recorded&source=gateway_device_event&limit=20")"
split_response "${TIMEOUT_AUDIT_RAW}"
require_http_code "200" "audit timeout"
TIMEOUT_AUDIT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg eid "${TIMEOUT_EVENT_ID}" '.items | map(select(.target | contains("event=\($eid)") and contains("queue=default") and contains("deduplicated=false"))) | length')"
if [[ "${TIMEOUT_AUDIT_HITS}" -lt 1 ]]; then
  echo "FAIL audit timeout target mismatch"
  exit 1
fi

echo "PASS: gateway door io loop regression complete gateway_id=${GW_ID} queue_total=${TIMEOUT_INGEST_TOTAL} rex=${REX_EVENT_ID} tamper=${TAMPER_EVENT_ID} timeout=${TIMEOUT_EVENT_ID}"
