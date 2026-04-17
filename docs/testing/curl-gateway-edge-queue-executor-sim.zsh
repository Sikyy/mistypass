#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18085}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://siky@localhost:5432/postgres?sslmode=disable}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
QUEUE_NAME="${QUEUE_NAME:-default}"
FORCE_RETRYABLE_ERROR="${FORCE_RETRYABLE_ERROR:-true}"
FORCE_RETRYABLE_PREFIX="${FORCE_RETRYABLE_PREFIX:-force-retry-}"
SERVER_LOG="${SERVER_LOG:-/tmp/mp_gateway_edge_queue_executor_sim.log}"
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
    PORT="${API_PORT}" \
    DATABASE_URL="${DATABASE_URL}" \
    GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_ERROR="${FORCE_RETRYABLE_ERROR}" \
    GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_PREFIX="${FORCE_RETRYABLE_PREFIX}" \
    GOCACHE=/tmp/go-build go run ./cmd/api >"${SERVER_LOG}" 2>&1
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

function edge_report_checkpoint() {
  local checkpoint_id="$1"
  local acked_count="$2"
  local step_name="$3"
  local occurred_at="$4"

  local checkpoint_payload
  checkpoint_payload="$(jq -nc \
    --arg gid "${GW_ID}" \
    --arg tenant "${TENANT_ID}" \
    --arg queue "${QUEUE_NAME}" \
    --arg ck "${checkpoint_id}" \
    --arg lr "rq-${checkpoint_id}" \
    --arg t "${occurred_at}" \
    --argjson acked "${acked_count}" \
    '{gateway_id:$gid,tenant_id:$tenant,queue:$queue,checkpoint_id:$ck,last_request_id:$lr,acked_count:$acked,last_occurred_at:$t}')"

  local checkpoint_raw
  checkpoint_raw="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${checkpoint_payload}")"
  split_response "${checkpoint_raw}"
  require_http_code "200" "${step_name}"

  local returned_acked
  returned_acked="$(echo "${HTTP_BODY}" | jq -r '.acked_count')"
  if [[ "${returned_acked}" != "${acked_count}" ]]; then
    echo "FAIL ${step_name}: expected acked=${acked_count}, got ${returned_acked}"
    exit 1
  fi
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
GW_SERIAL="MP-GW-EDGE-SIM-${RUN_TAG}"
ACCESS_OK_1="gwea-edge-sim-${RUN_TAG}-ok1"
ACCESS_RETRY_1="${FORCE_RETRYABLE_PREFIX}gwea-edge-sim-${RUN_TAG}-retry1"
DEVICE_OK_1="gwed-edge-sim-${RUN_TAG}-ok1"
ACCESS_OK_2="gwea-edge-sim-${RUN_TAG}-ok2"
DEVICE_OK_2="gwed-edge-sim-${RUN_TAG}-ok2"
VALID_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
EDGE_ACKED_COUNT="0"


echo "== start api =="
start_api

echo "== login =="
login

echo "== import gateway serial and bootstrap register =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg sn "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-edge-executor-sim",source:"factory"}]}')"
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

echo "== edge executor: source batch #1 with retryable subset =="
SOURCE_BATCH_1_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg b "${BUILDING_ID}" \
  --arg a1 "${ACCESS_OK_1}" \
  --arg ar "${ACCESS_RETRY_1}" \
  --arg d1 "${DEVICE_OK_1}" \
  --arg t "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    queue:$queue,
    access_events:[
      {event_id:$a1,request_id:("rq-"+$a1),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_001",type:"access_granted",actor:"qa.edge.sim.access",result:"success",occurred_at:$t},
      {event_id:$ar,request_id:("rq-"+$ar),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_002",type:"access_granted",actor:"qa.edge.sim.retry",result:"success",occurred_at:$t}
    ],
    device_events:[
      {event_id:$d1,request_id:("rq-"+$d1),building_id:$b,type:"gateway_event",detail:"edge executor source batch #1",result:"warning",occurred_at:$t}
    ]
  }')"
SOURCE_BATCH_1_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${SOURCE_BATCH_1_PAYLOAD}")"
split_response "${SOURCE_BATCH_1_RAW}"
require_http_code "202" "source batch #1"
SRC1_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
SRC1_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.failed')"
SRC1_RETRYABLE_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.retryable_failed')"
SRC1_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
SRC1_A_FAILED="$(echo "${HTTP_BODY}" | jq -r '.access.failed')"
SRC1_D_CREATED="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
SRC1_SUBSET_A="$(echo "${HTTP_BODY}" | jq -r '.retry_subset.access_events | length')"
SRC1_SUBSET_D="$(echo "${HTTP_BODY}" | jq -r '.retry_subset.device_events | length')"
SRC1_HINT_ACTION="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.next_action')"
SRC1_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${SRC1_STATUS}" != "partial" || "${SRC1_FAILED}" != "1" || "${SRC1_RETRYABLE_FAILED}" != "1" || "${SRC1_A_CREATED}" != "1" || "${SRC1_A_FAILED}" != "1" || "${SRC1_D_CREATED}" != "1" || "${SRC1_SUBSET_A}" != "1" || "${SRC1_SUBSET_D}" != "0" || "${SRC1_HINT_ACTION}" != "replay_retry_subset_then_report_checkpoint" || "${SRC1_HINT_TOTAL}" != "2" ]]; then
  echo "FAIL source batch #1 mismatch: status=${SRC1_STATUS} failed=${SRC1_FAILED} retryable_failed=${SRC1_RETRYABLE_FAILED} access(created=${SRC1_A_CREATED},failed=${SRC1_A_FAILED}) device_created=${SRC1_D_CREATED} subset(access=${SRC1_SUBSET_A},device=${SRC1_SUBSET_D}) hint_action=${SRC1_HINT_ACTION} hint_total=${SRC1_HINT_TOTAL}"
  exit 1
fi

RETRY_SUBSET_PAYLOAD="$(echo "${HTTP_BODY}" | jq -c '.retry_subset')"

echo "== edge executor: replay retry_subset from batch #1 =="
REPLAY_SUBSET_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${RETRY_SUBSET_PAYLOAD}")"
split_response "${REPLAY_SUBSET_RAW}"
require_http_code "202" "replay retry subset"
REPLAY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
REPLAY_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.failed')"
REPLAY_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
REPLAY_HINT_ACTION="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.next_action')"
REPLAY_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
REPLAY_HINT_CKPT="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.checkpoint_id')"
if [[ "${REPLAY_STATUS}" != "accepted" || "${REPLAY_FAILED}" != "0" || "${REPLAY_A_CREATED}" != "1" || "${REPLAY_HINT_ACTION}" != "report_checkpoint" || "${REPLAY_HINT_TOTAL}" != "3" || -z "${REPLAY_HINT_CKPT}" || "${REPLAY_HINT_CKPT}" == "null" ]]; then
  echo "FAIL replay retry subset mismatch: status=${REPLAY_STATUS} failed=${REPLAY_FAILED} access_created=${REPLAY_A_CREATED} hint_action=${REPLAY_HINT_ACTION} hint_total=${REPLAY_HINT_TOTAL} hint_ckpt=${REPLAY_HINT_CKPT}"
  exit 1
fi

EDGE_ACKED_COUNT="${REPLAY_HINT_TOTAL}"

echo "== edge executor: report checkpoint after replay subset =="
edge_report_checkpoint "${REPLAY_HINT_CKPT}" "${EDGE_ACKED_COUNT}" "checkpoint after replay subset" "${VALID_TIME}"

echo "== edge executor: source batch #2 =="
SOURCE_BATCH_2_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg b "${BUILDING_ID}" \
  --arg a2 "${ACCESS_OK_2}" \
  --arg d2 "${DEVICE_OK_2}" \
  --arg t "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    queue:$queue,
    access_events:[
      {event_id:$a2,request_id:("rq-"+$a2),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_003",type:"access_granted",actor:"qa.edge.sim.access",result:"success",occurred_at:$t}
    ],
    device_events:[
      {event_id:$d2,request_id:("rq-"+$d2),building_id:$b,type:"gateway_event",detail:"edge executor source batch #2",result:"ok",occurred_at:$t}
    ]
  }')"
SOURCE_BATCH_2_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${SOURCE_BATCH_2_PAYLOAD}")"
split_response "${SOURCE_BATCH_2_RAW}"
require_http_code "202" "source batch #2"
SRC2_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
SRC2_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
SRC2_D_CREATED="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
SRC2_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.failed')"
SRC2_HINT_ACTION="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.next_action')"
SRC2_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
SRC2_HINT_CKPT="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.checkpoint_id')"
if [[ "${SRC2_STATUS}" != "accepted" || "${SRC2_A_CREATED}" != "1" || "${SRC2_D_CREATED}" != "1" || "${SRC2_FAILED}" != "0" || "${SRC2_HINT_ACTION}" != "report_checkpoint" || "${SRC2_HINT_TOTAL}" != "5" || -z "${SRC2_HINT_CKPT}" || "${SRC2_HINT_CKPT}" == "null" ]]; then
  echo "FAIL source batch #2 mismatch: status=${SRC2_STATUS} access_created=${SRC2_A_CREATED} device_created=${SRC2_D_CREATED} failed=${SRC2_FAILED} hint_action=${SRC2_HINT_ACTION} hint_total=${SRC2_HINT_TOTAL} hint_ckpt=${SRC2_HINT_CKPT}"
  exit 1
fi

EDGE_ACKED_COUNT="${SRC2_HINT_TOTAL}"

echo "== edge executor: report checkpoint after source batch #2 =="
edge_report_checkpoint "${SRC2_HINT_CKPT}" "${EDGE_ACKED_COUNT}" "checkpoint after source batch #2" "${VALID_TIME}"

echo "== edge executor: resend source batch #2 (deduplicated replay) =="
SOURCE_BATCH_2_REPLAY_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${SOURCE_BATCH_2_PAYLOAD}")"
split_response "${SOURCE_BATCH_2_REPLAY_RAW}"
require_http_code "202" "source batch #2 replay"
SRC2_REPLAY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
SRC2_REPLAY_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
SRC2_REPLAY_A_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.access.deduplicated')"
SRC2_REPLAY_D_CREATED="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
SRC2_REPLAY_D_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.device.deduplicated')"
SRC2_REPLAY_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${SRC2_REPLAY_STATUS}" != "accepted" || "${SRC2_REPLAY_A_CREATED}" != "0" || "${SRC2_REPLAY_A_DEDUP}" != "1" || "${SRC2_REPLAY_D_CREATED}" != "0" || "${SRC2_REPLAY_D_DEDUP}" != "1" || "${SRC2_REPLAY_HINT_TOTAL}" != "5" ]]; then
  echo "FAIL source batch #2 replay mismatch: status=${SRC2_REPLAY_STATUS} access(created=${SRC2_REPLAY_A_CREATED},dedup=${SRC2_REPLAY_A_DEDUP}) device(created=${SRC2_REPLAY_D_CREATED},dedup=${SRC2_REPLAY_D_DEDUP}) hint_total=${SRC2_REPLAY_HINT_TOTAL}"
  exit 1
fi

echo "== checkpoint regression guard (expect conflict) =="
CKPT_REGRESS_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${QUEUE_NAME}" \
  --arg ck "seq-${RUN_TAG}-regress" \
  --arg lr "rq-${RUN_TAG}-regress" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:$queue,checkpoint_id:$ck,last_request_id:$lr,acked_count:4,last_occurred_at:$t}')"
CKPT_REGRESS_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CKPT_REGRESS_PAYLOAD}")"
split_response "${CKPT_REGRESS_RAW}"
require_http_code "409" "checkpoint acked regression guard"
CKPT_REGRESS_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
CKPT_REGRESS_ACTION="$(echo "${HTTP_BODY}" | jq -r '.next_action')"
CKPT_REGRESS_ACKED="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.acked_count')"
if [[ "${CKPT_REGRESS_ERROR}" != "event checkpoint acked_count regression" || "${CKPT_REGRESS_ACTION}" != "retry_with_non_regressing_acked_count" || "${CKPT_REGRESS_ACKED}" != "5" ]]; then
  echo "FAIL checkpoint regression guard mismatch: error=${CKPT_REGRESS_ERROR} action=${CKPT_REGRESS_ACTION} latest_acked=${CKPT_REGRESS_ACKED}"
  exit 1
fi

echo "== verify checkpoint read model =="
CHECKPOINT_LIST_RAW="$(api_with_auth GET "/api/v1/gateways/${GW_ID}/events/checkpoint?tenant_id=${TENANT_ID}")"
split_response "${CHECKPOINT_LIST_RAW}"
require_http_code "200" "list gateway checkpoints"
LIST_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
LIST_QUEUE="$(echo "${HTTP_BODY}" | jq -r '.items[0].queue')"
LIST_ACKED="$(echo "${HTTP_BODY}" | jq -r '.items[0].acked_count')"
if [[ "${LIST_COUNT}" -lt 1 || "${LIST_QUEUE}" != "${QUEUE_NAME}" || "${LIST_ACKED}" != "5" ]]; then
  echo "FAIL checkpoint list mismatch: count=${LIST_COUNT} queue=${LIST_QUEUE} acked=${LIST_ACKED}"
  exit 1
fi

echo "== verify no duplicate rows in event lists =="
ACCESS_LIST_RAW="$(api_with_auth GET "/api/v1/events/access?tenant_id=${TENANT_ID}")"
split_response "${ACCESS_LIST_RAW}"
require_http_code "200" "list access events"
ACCESS_OK1_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${ACCESS_OK_1}" '[.items[] | select(.id == $id)] | length')"
ACCESS_RETRY1_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${ACCESS_RETRY_1}" '[.items[] | select(.id == $id)] | length')"
ACCESS_OK2_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${ACCESS_OK_2}" '[.items[] | select(.id == $id)] | length')"
if [[ "${ACCESS_OK1_COUNT}" != "1" || "${ACCESS_RETRY1_COUNT}" != "1" || "${ACCESS_OK2_COUNT}" != "1" ]]; then
  echo "FAIL access dedup mismatch: ok1=${ACCESS_OK1_COUNT} retry1=${ACCESS_RETRY1_COUNT} ok2=${ACCESS_OK2_COUNT}"
  exit 1
fi

DEVICE_LIST_RAW="$(api_with_auth GET "/api/v1/events/device?tenant_id=${TENANT_ID}")"
split_response "${DEVICE_LIST_RAW}"
require_http_code "200" "list device events"
DEVICE_OK1_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${DEVICE_OK_1}" '[.items[] | select(.id == $id)] | length')"
DEVICE_OK2_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${DEVICE_OK_2}" '[.items[] | select(.id == $id)] | length')"
if [[ "${DEVICE_OK1_COUNT}" != "1" || "${DEVICE_OK2_COUNT}" != "1" ]]; then
  echo "FAIL device dedup mismatch: ok1=${DEVICE_OK1_COUNT} ok2=${DEVICE_OK2_COUNT}"
  exit 1
fi

echo "PASS: gateway edge queue executor simulation regression complete"
