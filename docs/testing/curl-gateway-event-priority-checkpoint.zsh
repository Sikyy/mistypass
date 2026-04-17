#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-8080}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
QUEUE_NAME="${QUEUE_NAME:-priority}"
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
    PORT="${API_PORT}" GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_event_priority_checkpoint.log 2>&1
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
  if [[ -f /tmp/mp_gateway_event_priority_checkpoint.log ]]; then
    tail -n 80 /tmp/mp_gateway_event_priority_checkpoint.log
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
GW_SERIAL="MP-GW-PRI-${RUN_TAG}"
A_EVENT="gwea-priority-${RUN_TAG}"
D_EVENT="gwed-priority-${RUN_TAG}"
VALID_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

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
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-priority-checkpoint",source:"factory"}]}')"
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

echo "== priority queue source batch =="
BATCH_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg b "${BUILDING_ID}" \
  --arg a "${A_EVENT}" \
  --arg d "${D_EVENT}" \
  --arg t "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    queue:$queue,
    access_events:[
      {event_id:$a,request_id:("rq-"+$a),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_001",type:"access_granted",actor:"qa.priority.access",result:"success",occurred_at:$t}
    ],
    device_events:[
      {event_id:$d,request_id:("rq-"+$d),building_id:$b,type:"gateway_event",detail:"priority queue checkpoint",result:"warning",occurred_at:$t}
    ]
  }')"
BATCH_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_PAYLOAD}")"
split_response "${BATCH_RAW}"
require_http_code "202" "priority source batch"
SRC_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
SRC_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
SRC_D_CREATED="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
SRC_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.failed')"
SRC_HINT_QUEUE="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.queue')"
SRC_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
SRC_HINT_CODE="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.status_code')"
SRC_HINT_ACTION="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.next_action')"
if [[ "${SRC_STATUS}" != "accepted" || "${SRC_A_CREATED}" != "1" || "${SRC_D_CREATED}" != "1" || "${SRC_FAILED}" != "0" || "${SRC_HINT_QUEUE}" != "${QUEUE_NAME}" || "${SRC_HINT_TOTAL}" != "2" || "${SRC_HINT_CODE}" != "QUEUE_READY_TO_CHECKPOINT" || "${SRC_HINT_ACTION}" != "report_checkpoint" ]]; then
  echo "FAIL priority source mismatch: status=${SRC_STATUS} access_created=${SRC_A_CREATED} device_created=${SRC_D_CREATED} failed=${SRC_FAILED} hint_queue=${SRC_HINT_QUEUE} hint_total=${SRC_HINT_TOTAL} hint_code=${SRC_HINT_CODE} hint_action=${SRC_HINT_ACTION}"
  exit 1
fi

echo "== replay same priority batch (expected deduplicated) =="
REPLAY_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_PAYLOAD}")"
split_response "${REPLAY_RAW}"
require_http_code "202" "priority replay batch"
REPLAY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
REPLAY_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
REPLAY_D_CREATED="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
REPLAY_A_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.access.deduplicated')"
REPLAY_D_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.device.deduplicated')"
REPLAY_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${REPLAY_STATUS}" != "accepted" || "${REPLAY_A_CREATED}" != "0" || "${REPLAY_D_CREATED}" != "0" || "${REPLAY_A_DEDUP}" != "1" || "${REPLAY_D_DEDUP}" != "1" || "${REPLAY_HINT_TOTAL}" != "${SRC_HINT_TOTAL}" ]]; then
  echo "FAIL priority replay mismatch: status=${REPLAY_STATUS} access_created=${REPLAY_A_CREATED} device_created=${REPLAY_D_CREATED} access_dedup=${REPLAY_A_DEDUP} device_dedup=${REPLAY_D_DEDUP} hint_total=${REPLAY_HINT_TOTAL}"
  exit 1
fi

echo "== checkpoint report on priority queue =="
CHECKPOINT_ID="seq-${RUN_TAG}-2"
CHECKPOINT_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg ck "${CHECKPOINT_ID}" \
  --arg lr "rq-${RUN_TAG}-2" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:$queue,checkpoint_id:$ck,last_request_id:$lr,acked_count:2,last_occurred_at:$t}')"
CHECKPOINT_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CHECKPOINT_PAYLOAD}")"
split_response "${CHECKPOINT_RAW}"
require_http_code "200" "priority checkpoint report"
CKPT_QUEUE="$(echo "${HTTP_BODY}" | jq -r '.queue')"
CKPT_ACKED="$(echo "${HTTP_BODY}" | jq -r '.acked_count')"
if [[ "${CKPT_QUEUE}" != "${QUEUE_NAME}" || "${CKPT_ACKED}" != "2" ]]; then
  echo "FAIL priority checkpoint mismatch: queue=${CKPT_QUEUE} acked=${CKPT_ACKED}"
  exit 1
fi

echo "== checkpoint exceeds server total (expected conflict) =="
CHECKPOINT_EXCEED_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg ck "seq-${RUN_TAG}-999" \
  --arg lr "rq-${RUN_TAG}-999" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:$queue,checkpoint_id:$ck,last_request_id:$lr,acked_count:999,last_occurred_at:$t}')"
CHECKPOINT_EXCEED_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CHECKPOINT_EXCEED_PAYLOAD}")"
split_response "${CHECKPOINT_EXCEED_RAW}"
require_http_code "409" "priority checkpoint exceeds server total"
EXCEED_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
EXCEED_ACTION="$(echo "${HTTP_BODY}" | jq -r '.next_action')"
EXCEED_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.server_event_total')"
EXCEED_SOURCE="$(echo "${HTTP_BODY}" | jq -r '.server_total_source')"
EXCEED_QUEUE="$(echo "${HTTP_BODY}" | jq -r '.queue')"
EXCEED_LATEST_ACKED="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.acked_count')"
EXCEED_LATEST_ID="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.checkpoint_id')"
if [[ "${EXCEED_ERROR}" != "event checkpoint acked_count exceeds server event total" || "${EXCEED_ACTION}" != "retry_with_server_event_total" || "${EXCEED_TOTAL}" != "2" || "${EXCEED_SOURCE}" != "queue_ingest_total" || "${EXCEED_QUEUE}" != "${QUEUE_NAME}" || "${EXCEED_LATEST_ACKED}" != "2" || "${EXCEED_LATEST_ID}" != "${CHECKPOINT_ID}" ]]; then
  echo "FAIL priority checkpoint conflict mismatch: error=${EXCEED_ERROR} action=${EXCEED_ACTION} total=${EXCEED_TOTAL} source=${EXCEED_SOURCE} queue=${EXCEED_QUEUE} latest_acked=${EXCEED_LATEST_ACKED} latest_id=${EXCEED_LATEST_ID}"
  exit 1
fi

echo "PASS: gateway priority queue checkpoint regression complete"
