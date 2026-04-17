#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18082}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://siky@localhost:5432/postgres?sslmode=disable}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
QUEUE_NAME="${QUEUE_NAME:-priority}"
SERVER_LOG="${SERVER_LOG:-/tmp/mp_gateway_event_queue_ingest_restart.log}"
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

function stop_port_processes() {
  local pids
  pids="$(lsof -ti "tcp:${API_PORT}" 2>/dev/null || true)"
  if [[ -z "${pids}" ]]; then
    return
  fi
  echo "${pids}" | xargs kill >/dev/null 2>&1 || true
  sleep 0.3
  pids="$(lsof -ti "tcp:${API_PORT}" 2>/dev/null || true)"
  if [[ -n "${pids}" ]]; then
    echo "${pids}" | xargs kill -9 >/dev/null 2>&1 || true
  fi
}

function start_api() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "api: ${API_BASE_URL} already running, trying to free port"
    stop_port_processes
    if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "FAIL start_api: ${API_BASE_URL} still has a running service"
      exit 1
    fi
  fi

  (
    cd api
    PORT="${API_PORT}" DATABASE_URL="${DATABASE_URL}" GOCACHE=/tmp/go-build go run ./cmd/api >"${SERVER_LOG}" 2>&1
  ) &
  API_PID="$!"

  local i
  for i in {1..80}; do
    if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "api: started on ${API_BASE_URL}"
      return
    fi
    sleep 0.25
  done

  echo "FAIL start_api: healthz not ready"
  if [[ -f "${SERVER_LOG}" ]]; then
    tail -n 120 "${SERVER_LOG}"
  fi
  exit 1
}

function stop_api() {
  if [[ -n "${API_PID}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
    wait "${API_PID}" >/dev/null 2>&1 || true
    API_PID=""
  fi
  stop_port_processes
}

function cleanup() {
  stop_api
}

function login() {
  local login_raw
  login_raw="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
    -w $'\n%{http_code}')"
  split_response "${login_raw}"
  require_http_code "200" "login"
  AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
  require_non_empty "${AT}" "login.access_token"
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

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
GW_SERIAL="MP-GW-QTOT-${RUN_TAG}"
ACCESS_1="gwea-qtot-${RUN_TAG}-1"
DEVICE_1="gwed-qtot-${RUN_TAG}-1"
ACCESS_2="gwea-qtot-${RUN_TAG}-2"
VALID_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "== start #1 =="
start_api

echo "== login #1 =="
login

echo "== import gateway serial and bootstrap register =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg sn "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-queue-ingest-restart",source:"factory"}]}')"
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

echo "== priority queue source batch #1 =="
BATCH_1_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg b "${BUILDING_ID}" \
  --arg a "${ACCESS_1}" \
  --arg d "${DEVICE_1}" \
  --arg t "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    queue:$queue,
    access_events:[
      {event_id:$a,request_id:("rq-"+$a),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_001",type:"access_granted",actor:"qa.restart.access",result:"success",occurred_at:$t}
    ],
    device_events:[
      {event_id:$d,request_id:("rq-"+$d),building_id:$b,type:"gateway_event",detail:"queue_ingest restart batch1",result:"warning",occurred_at:$t}
    ]
  }')"
BATCH_1_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_1_PAYLOAD}")"
split_response "${BATCH_1_RAW}"
require_http_code "202" "priority batch #1"
BATCH_1_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${BATCH_1_TOTAL}" != "2" ]]; then
  echo "FAIL priority batch #1 expected hint_total=2 got ${BATCH_1_TOTAL}"
  exit 1
fi

echo "== priority queue source batch #2 =="
BATCH_2_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg b "${BUILDING_ID}" \
  --arg a "${ACCESS_2}" \
  --arg t "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    queue:$queue,
    access_events:[
      {event_id:$a,request_id:("rq-"+$a),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_002",type:"access_granted",actor:"qa.restart.access",result:"success",occurred_at:$t}
    ],
    device_events:[]
  }')"
BATCH_2_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_2_PAYLOAD}")"
split_response "${BATCH_2_RAW}"
require_http_code "202" "priority batch #2"
BATCH_2_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
BATCH_2_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
BATCH_2_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${BATCH_2_STATUS}" != "accepted" || "${BATCH_2_A_CREATED}" != "1" || "${BATCH_2_TOTAL}" != "3" ]]; then
  echo "FAIL priority batch #2 mismatch: status=${BATCH_2_STATUS} access_created=${BATCH_2_A_CREATED} hint_total=${BATCH_2_TOTAL}"
  exit 1
fi

echo "== checkpoint report before restart =="
CHECKPOINT_ID="seq-${RUN_TAG}-3"
CHECKPOINT_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg ck "${CHECKPOINT_ID}" \
  --arg lr "rq-${RUN_TAG}-3" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:$queue,checkpoint_id:$ck,last_request_id:$lr,acked_count:3,last_occurred_at:$t}')"
CHECKPOINT_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CHECKPOINT_PAYLOAD}")"
split_response "${CHECKPOINT_RAW}"
require_http_code "200" "checkpoint report before restart"

echo "== restart API =="
stop_api
start_api

echo "== login #2 =="
login

echo "== replay batch #2 after restart (expected deduplicated + total unchanged) =="
REPLAY_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_2_PAYLOAD}")"
split_response "${REPLAY_RAW}"
require_http_code "202" "replay batch #2 after restart"
REPLAY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
REPLAY_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
REPLAY_A_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.access.deduplicated')"
REPLAY_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${REPLAY_STATUS}" != "accepted" || "${REPLAY_A_CREATED}" != "0" || "${REPLAY_A_DEDUP}" != "1" || "${REPLAY_TOTAL}" != "3" ]]; then
  echo "FAIL replay after restart mismatch: status=${REPLAY_STATUS} access_created=${REPLAY_A_CREATED} access_dedup=${REPLAY_A_DEDUP} hint_total=${REPLAY_TOTAL}"
  exit 1
fi

echo "== checkpoint regression after restart (expected conflict) =="
CHECKPOINT_REGRESS_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg ck "seq-${RUN_TAG}-2" \
  --arg lr "rq-${RUN_TAG}-2" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:$queue,checkpoint_id:$ck,last_request_id:$lr,acked_count:2,last_occurred_at:$t}')"
CHECKPOINT_REGRESS_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CHECKPOINT_REGRESS_PAYLOAD}")"
split_response "${CHECKPOINT_REGRESS_RAW}"
require_http_code "409" "checkpoint regression after restart"
REGRESS_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
REGRESS_ACTION="$(echo "${HTTP_BODY}" | jq -r '.next_action')"
REGRESS_LATEST_ID="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.checkpoint_id')"
REGRESS_LATEST_ACKED="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.acked_count')"
if [[ "${REGRESS_ERROR}" != "event checkpoint acked_count regression" || "${REGRESS_ACTION}" != "retry_with_non_regressing_acked_count" || "${REGRESS_LATEST_ID}" != "${CHECKPOINT_ID}" || "${REGRESS_LATEST_ACKED}" != "3" ]]; then
  echo "FAIL checkpoint regression after restart mismatch: error=${REGRESS_ERROR} action=${REGRESS_ACTION} latest_id=${REGRESS_LATEST_ID} latest_acked=${REGRESS_LATEST_ACKED}"
  exit 1
fi

echo "== checkpoint exceeds server total after restart (expected queue_ingest_total conflict) =="
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
require_http_code "409" "checkpoint exceeds server total after restart"
EXCEED_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
EXCEED_ACTION="$(echo "${HTTP_BODY}" | jq -r '.next_action')"
EXCEED_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.server_event_total')"
EXCEED_SOURCE="$(echo "${HTTP_BODY}" | jq -r '.server_total_source')"
EXCEED_QUEUE="$(echo "${HTTP_BODY}" | jq -r '.queue')"
if [[ "${EXCEED_ERROR}" != "event checkpoint acked_count exceeds server event total" || "${EXCEED_ACTION}" != "retry_with_server_event_total" || "${EXCEED_TOTAL}" != "3" || "${EXCEED_SOURCE}" != "queue_ingest_total" || "${EXCEED_QUEUE}" != "${QUEUE_NAME}" ]]; then
  echo "FAIL checkpoint exceed after restart mismatch: error=${EXCEED_ERROR} action=${EXCEED_ACTION} total=${EXCEED_TOTAL} source=${EXCEED_SOURCE} queue=${EXCEED_QUEUE}"
  exit 1
fi

echo "PASS: gateway queue_ingest_totals restart regression complete"
