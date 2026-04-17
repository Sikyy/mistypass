#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-8080}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
FORCE_RETRYABLE_ERROR="${FORCE_RETRYABLE_ERROR:-true}"
FORCE_RETRYABLE_PREFIX="${FORCE_RETRYABLE_PREFIX:-force-retry-}"
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
    PORT="${API_PORT}" \
    GOCACHE=/tmp/go-build \
      GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_ERROR="${FORCE_RETRYABLE_ERROR}" \
      GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_PREFIX="${FORCE_RETRYABLE_PREFIX}" \
      go run ./cmd/api >/tmp/mp_gateway_event_retry_subset_mixed.log 2>&1
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
  if [[ -f /tmp/mp_gateway_event_retry_subset_mixed.log ]]; then
    tail -n 80 /tmp/mp_gateway_event_retry_subset_mixed.log
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
GW_SERIAL="MP-GW-RETRY-MIX-${RUN_TAG}"
A_RETRY="${FORCE_RETRYABLE_PREFIX}gwea-mix-${RUN_TAG}-retry"
A_OK="gwea-mix-${RUN_TAG}-ok"
D_RETRY="${FORCE_RETRYABLE_PREFIX}gwed-mix-${RUN_TAG}-retry"
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
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-retry-mix",source:"factory"}]}')"
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

echo "== mixed retry_subset source batch =="
BATCH_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg b "${BUILDING_ID}" \
  --arg a_retry "${A_RETRY}" \
  --arg a_ok "${A_OK}" \
  --arg d_retry "${D_RETRY}" \
  --arg t "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    queue:"default",
    access_events:[
      {event_id:$a_retry,request_id:("rq-"+$a_retry),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_001",type:"access_granted",actor:"qa.retry.access",result:"success",occurred_at:$t},
      {event_id:$a_ok,request_id:("rq-"+$a_ok),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_002",type:"access_granted",actor:"qa.ok.access",result:"success",occurred_at:$t}
    ],
    device_events:[
      {event_id:$d_retry,request_id:("rq-"+$d_retry),building_id:$b,type:"gateway_event",detail:"retry subset mixed",result:"warning",occurred_at:$t}
    ]
  }')"
BATCH_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_PAYLOAD}")"
split_response "${BATCH_RAW}"
require_http_code "202" "mixed retry_subset source"
SRC_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
SRC_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
SRC_A_FAILED="$(echo "${HTTP_BODY}" | jq -r '.access.failed')"
SRC_D_CREATED="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
SRC_D_FAILED="$(echo "${HTTP_BODY}" | jq -r '.device.failed')"
SRC_RETRYABLE="$(echo "${HTTP_BODY}" | jq -r '.totals.retryable_failed')"
SRC_NON_RETRYABLE="$(echo "${HTTP_BODY}" | jq -r '.totals.non_retryable_failed')"
SRC_SUBSET_ACCESS="$(echo "${HTTP_BODY}" | jq -r '.retry_subset.access_events | length')"
SRC_SUBSET_DEVICE="$(echo "${HTTP_BODY}" | jq -r '.retry_subset.device_events | length')"
SRC_SUBSET_QUEUE="$(echo "${HTTP_BODY}" | jq -r '.retry_subset.queue')"
SRC_QUEUE_HINT_CODE="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.status_code')"
SRC_QUEUE_HINT_ACTION="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.next_action')"
SRC_QUEUE_HINT_CKPT="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.checkpoint_id')"
SRC_QUEUE_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${SRC_STATUS}" != "partial" || "${SRC_A_CREATED}" != "1" || "${SRC_A_FAILED}" != "1" || "${SRC_D_CREATED}" != "0" || "${SRC_D_FAILED}" != "1" || "${SRC_RETRYABLE}" != "2" || "${SRC_NON_RETRYABLE}" != "0" || "${SRC_SUBSET_ACCESS}" != "1" || "${SRC_SUBSET_DEVICE}" != "1" || "${SRC_SUBSET_QUEUE}" != "default" || "${SRC_QUEUE_HINT_CODE}" != "QUEUE_RETRY_SUBSET_REQUIRED" || "${SRC_QUEUE_HINT_ACTION}" != "replay_retry_subset_then_report_checkpoint" || -z "${SRC_QUEUE_HINT_CKPT}" || "${SRC_QUEUE_HINT_CKPT}" == "null" || "${SRC_QUEUE_HINT_TOTAL}" != "1" ]]; then
  echo "FAIL mixed source counters mismatch: status=${SRC_STATUS} access(created=${SRC_A_CREATED},failed=${SRC_A_FAILED}) device(created=${SRC_D_CREATED},failed=${SRC_D_FAILED}) retryable=${SRC_RETRYABLE} non_retryable=${SRC_NON_RETRYABLE} subset_access=${SRC_SUBSET_ACCESS} subset_device=${SRC_SUBSET_DEVICE} subset_queue=${SRC_SUBSET_QUEUE} hint_code=${SRC_QUEUE_HINT_CODE} hint_action=${SRC_QUEUE_HINT_ACTION} hint_ckpt=${SRC_QUEUE_HINT_CKPT} hint_total=${SRC_QUEUE_HINT_TOTAL}"
  exit 1
fi

RETRY_SUBSET_PAYLOAD="$(echo "${HTTP_BODY}" | jq -c '.retry_subset')"

echo "== replay retry_subset (expected success) =="
REPLAY_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${RETRY_SUBSET_PAYLOAD}")"
split_response "${REPLAY_RAW}"
require_http_code "202" "replay retry_subset"
REPLAY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
REPLAY_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.failed')"
REPLAY_A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
REPLAY_D_CREATED="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
REPLAY_HINT_CODE="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.status_code')"
REPLAY_HINT_ACTION="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.next_action')"
REPLAY_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${REPLAY_STATUS}" != "accepted" || "${REPLAY_FAILED}" != "0" || "${REPLAY_A_CREATED}" != "1" || "${REPLAY_D_CREATED}" != "1" || "${REPLAY_HINT_CODE}" != "QUEUE_READY_TO_CHECKPOINT" || "${REPLAY_HINT_ACTION}" != "report_checkpoint" || "${REPLAY_HINT_TOTAL}" != "3" ]]; then
  echo "FAIL replay retry_subset mismatch: status=${REPLAY_STATUS} failed=${REPLAY_FAILED} access_created=${REPLAY_A_CREATED} device_created=${REPLAY_D_CREATED} hint_code=${REPLAY_HINT_CODE} hint_action=${REPLAY_HINT_ACTION} hint_total=${REPLAY_HINT_TOTAL}"
  exit 1
fi

echo "== replay same retry_subset again (expected deduplicated) =="
REPLAY_2_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${RETRY_SUBSET_PAYLOAD}")"
split_response "${REPLAY_2_RAW}"
require_http_code "202" "replay retry_subset second"
REPLAY_2_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
REPLAY_2_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.failed')"
REPLAY_2_A_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.access.deduplicated')"
REPLAY_2_D_DEDUP="$(echo "${HTTP_BODY}" | jq -r '.device.deduplicated')"
REPLAY_2_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${REPLAY_2_STATUS}" != "accepted" || "${REPLAY_2_FAILED}" != "0" || "${REPLAY_2_A_DEDUP}" != "1" || "${REPLAY_2_D_DEDUP}" != "1" || "${REPLAY_2_HINT_TOTAL}" != "3" ]]; then
  echo "FAIL second replay mismatch: status=${REPLAY_2_STATUS} failed=${REPLAY_2_FAILED} access_dedup=${REPLAY_2_A_DEDUP} device_dedup=${REPLAY_2_D_DEDUP} hint_total=${REPLAY_2_HINT_TOTAL}"
  exit 1
fi

echo "== verify list endpoints =="
ACCESS_LIST_RAW="$(api_with_auth GET "/api/v1/events/access?tenant_id=${TENANT_ID}")"
split_response "${ACCESS_LIST_RAW}"
require_http_code "200" "list access events"
A_OK_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${A_OK}" '[.items[] | select(.id == $id)] | length')"
A_RETRY_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${A_RETRY}" '[.items[] | select(.id == $id)] | length')"
if [[ "${A_OK_COUNT}" != "1" || "${A_RETRY_COUNT}" != "1" ]]; then
  echo "FAIL access list mismatch: ok=${A_OK_COUNT} retry=${A_RETRY_COUNT}"
  exit 1
fi

DEVICE_LIST_RAW="$(api_with_auth GET "/api/v1/events/device?tenant_id=${TENANT_ID}")"
split_response "${DEVICE_LIST_RAW}"
require_http_code "200" "list device events"
D_RETRY_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg id "${D_RETRY}" '[.items[] | select(.id == $id)] | length')"
if [[ "${D_RETRY_COUNT}" != "1" ]]; then
  echo "FAIL device list mismatch: retry=${D_RETRY_COUNT}"
  exit 1
fi

echo "PASS: gateway event retry_subset mixed regression complete"
