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
      go run ./cmd/api >/tmp/mp_gateway_event_checkpoint_partial.log 2>&1
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
  if [[ -f /tmp/mp_gateway_event_checkpoint_partial.log ]]; then
    tail -n 80 /tmp/mp_gateway_event_checkpoint_partial.log
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
GW_SERIAL="MP-GW-CKPT-${RUN_TAG}"
A_OK="gwea-ckpt-${RUN_TAG}-ok"
A_BAD="gwea-ckpt-${RUN_TAG}-bad"
D_OK="gwed-ckpt-${RUN_TAG}-ok"
A_RETRY="${FORCE_RETRYABLE_PREFIX}gwea-ckpt-${RUN_TAG}-retry"
VALID_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
INVALID_TIME="2026-13-99T99:99:99Z"

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
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-ckpt",source:"factory"}]}')"
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

echo "== batch upload with partial failure =="
BATCH_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg b "${BUILDING_ID}" \
  --arg aok "${A_OK}" \
  --arg abad "${A_BAD}" \
  --arg dok "${D_OK}" \
  --arg ok_time "${VALID_TIME}" \
  --arg bad_time "${INVALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    access_events:[
      {event_id:$aok,request_id:("rq-"+$aok),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_001",type:"access_granted",actor:"qa.ckpt.ok",result:"success",occurred_at:$ok_time},
      {event_id:$abad,request_id:("rq-"+$abad),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_002",type:"access_denied",actor:"qa.ckpt.bad",result:"denied",occurred_at:$bad_time}
    ],
    device_events:[
      {event_id:$dok,request_id:("rq-"+$dok),building_id:$b,type:"gateway_event",detail:"checkpoint partial test",result:"warning",occurred_at:$ok_time}
    ]
  }')"
BATCH_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${BATCH_PAYLOAD}")"
split_response "${BATCH_RAW}"
require_http_code "202" "batch partial"
BATCH_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
A_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
A_FAILED="$(echo "${HTTP_BODY}" | jq -r '.access.failed')"
D_CREATED="$(echo "${HTTP_BODY}" | jq -r '.device.created')"
TOTAL_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.failed')"
RETRYABLE_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.retryable_failed')"
NON_RETRYABLE_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.non_retryable_failed')"
RETRY_TOTAL="$(echo "${HTTP_BODY}" | jq -r '(.retry_subset.access_events | length) + (.retry_subset.device_events | length)')"
RETRY_ACCESS_COUNT="$(echo "${HTTP_BODY}" | jq -r '.retry_subset.access_events | length')"
FAILED_RETRYABLE="$(echo "${HTTP_BODY}" | jq -r '.access.results[] | select(.status == "failed") | .retryable')"
QUEUE_HINT_CODE="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.status_code')"
QUEUE_HINT_ACTION="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.next_action')"
QUEUE_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${BATCH_STATUS}" != "partial" || "${A_CREATED}" != "1" || "${A_FAILED}" != "1" || "${D_CREATED}" != "1" || "${TOTAL_FAILED}" != "1" || "${RETRYABLE_FAILED}" != "0" || "${NON_RETRYABLE_FAILED}" != "1" || "${RETRY_TOTAL}" != "0" || "${RETRY_ACCESS_COUNT}" != "0" || "${FAILED_RETRYABLE}" != "false" || "${QUEUE_HINT_CODE}" != "QUEUE_PARTIAL_NON_RETRYABLE" || "${QUEUE_HINT_ACTION}" != "report_checkpoint_with_non_retryable_failures" || "${QUEUE_HINT_TOTAL}" != "2" ]]; then
  echo "FAIL batch partial counters: status=${BATCH_STATUS} access(created=${A_CREATED},failed=${A_FAILED}) device(created=${D_CREATED}) total_failed=${TOTAL_FAILED} hint_code=${QUEUE_HINT_CODE} hint_action=${QUEUE_HINT_ACTION} hint_total=${QUEUE_HINT_TOTAL}"
  exit 1
fi

echo "== checkpoint report =="
CHECKPOINT_ID="seq-${RUN_TAG}-2"
CKPT_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg ck "${CHECKPOINT_ID}" \
  --arg lr "rq-${RUN_TAG}-2" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:"default",checkpoint_id:$ck,last_request_id:$lr,acked_count:2,last_occurred_at:$t}')"
CKPT_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CKPT_PAYLOAD}")"
split_response "${CKPT_RAW}"
require_http_code "200" "checkpoint report"
CKPT_RETURN="$(echo "${HTTP_BODY}" | jq -r '.checkpoint_id')"
CKPT_ACKED="$(echo "${HTTP_BODY}" | jq -r '.acked_count')"
if [[ "${CKPT_RETURN}" != "${CHECKPOINT_ID}" || "${CKPT_ACKED}" != "2" ]]; then
  echo "FAIL checkpoint report mismatch: checkpoint_id=${CKPT_RETURN} acked=${CKPT_ACKED}"
  exit 1
fi

echo "== checkpoint report with regressed acked_count (expect conflict) =="
CKPT_REGRESS_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg ck "seq-${RUN_TAG}-1" \
  --arg lr "rq-${RUN_TAG}-1" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:"default",checkpoint_id:$ck,last_request_id:$lr,acked_count:1,last_occurred_at:$t}')"
CKPT_REGRESS_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CKPT_REGRESS_PAYLOAD}")"
split_response "${CKPT_REGRESS_RAW}"
require_http_code "409" "checkpoint acked_count regression"
CKPT_REGRESS_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
CKPT_REGRESS_ACTION="$(echo "${HTTP_BODY}" | jq -r '.next_action')"
CKPT_REGRESS_LATEST_ACKED="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.acked_count')"
CKPT_REGRESS_LATEST_ID="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.checkpoint_id')"
if [[ "${CKPT_REGRESS_ERROR}" != "event checkpoint acked_count regression" || "${CKPT_REGRESS_ACTION}" != "retry_with_non_regressing_acked_count" || "${CKPT_REGRESS_LATEST_ACKED}" != "2" || "${CKPT_REGRESS_LATEST_ID}" != "${CHECKPOINT_ID}" ]]; then
  echo "FAIL checkpoint regression payload mismatch: error=${CKPT_REGRESS_ERROR} action=${CKPT_REGRESS_ACTION} latest_acked=${CKPT_REGRESS_LATEST_ACKED} latest_id=${CKPT_REGRESS_LATEST_ID}"
  exit 1
fi

echo "== checkpoint report with acked_count above server total (expect conflict) =="
CKPT_EXCEED_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg ck "seq-${RUN_TAG}-999" \
  --arg lr "rq-${RUN_TAG}-999" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:"default",checkpoint_id:$ck,last_request_id:$lr,acked_count:999,last_occurred_at:$t}')"
CKPT_EXCEED_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CKPT_EXCEED_PAYLOAD}")"
split_response "${CKPT_EXCEED_RAW}"
require_http_code "409" "checkpoint acked_count exceeds server total"
CKPT_EXCEED_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
CKPT_EXCEED_ACTION="$(echo "${HTTP_BODY}" | jq -r '.next_action')"
CKPT_EXCEED_SERVER_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.server_event_total')"
CKPT_EXCEED_SOURCE="$(echo "${HTTP_BODY}" | jq -r '.server_total_source')"
CKPT_EXCEED_LATEST_ACKED="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.acked_count')"
if [[ "${CKPT_EXCEED_ERROR}" != "event checkpoint acked_count exceeds server event total" || "${CKPT_EXCEED_ACTION}" != "retry_with_server_event_total" || "${CKPT_EXCEED_SERVER_TOTAL}" != "2" || "${CKPT_EXCEED_SOURCE}" != "queue_ingest_total" || "${CKPT_EXCEED_LATEST_ACKED}" != "2" ]]; then
  echo "FAIL checkpoint exceed payload mismatch: error=${CKPT_EXCEED_ERROR} action=${CKPT_EXCEED_ACTION} server_total=${CKPT_EXCEED_SERVER_TOTAL} source=${CKPT_EXCEED_SOURCE} latest_acked=${CKPT_EXCEED_LATEST_ACKED}"
  exit 1
fi

echo "== admin list checkpoint =="
LIST_RAW="$(api_with_auth GET "/api/v1/gateways/${GW_ID}/events/checkpoint?tenant_id=${TENANT_ID}")"
split_response "${LIST_RAW}"
require_http_code "200" "list checkpoint"
LIST_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
LIST_CKPT="$(echo "${HTTP_BODY}" | jq -r '.items[0].checkpoint_id')"
if [[ "${LIST_COUNT}" -lt 1 || "${LIST_CKPT}" != "${CHECKPOINT_ID}" ]]; then
  echo "FAIL list checkpoint mismatch: count=${LIST_COUNT} checkpoint=${LIST_CKPT}"
  exit 1
fi

echo "== admin checkpoint summary =="
SUMMARY_RAW="$(api_with_auth GET "/api/v1/gateways/events/checkpoint/summary?tenant_id=${TENANT_ID}&gateway_id=${GW_ID}&trend_window_minutes=15")"
split_response "${SUMMARY_RAW}"
require_http_code "200" "checkpoint summary"
SUMMARY_QUEUES="$(echo "${HTTP_BODY}" | jq -r '.totals.queues')"
SUMMARY_LAG="$(echo "${HTTP_BODY}" | jq -r '.totals.lag_total')"
SUMMARY_ACKED="$(echo "${HTTP_BODY}" | jq -r '.totals.acked_total')"
TREND_WINDOW_MINUTES="$(echo "${HTTP_BODY}" | jq -r '.time_window_trend.window_minutes')"
TREND_REPORT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.time_window_trend.report_total')"
TREND_DIRECTION="$(echo "${HTTP_BODY}" | jq -r '.time_window_trend.direction')"
ITEM_TREND_REPORT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.items[0].time_window_trend.report_total')"
if [[ "${SUMMARY_QUEUES}" -lt 1 || "${SUMMARY_LAG}" != "0" || "${SUMMARY_ACKED}" != "2" || "${TREND_WINDOW_MINUTES}" != "15" || "${TREND_REPORT_TOTAL}" -lt 1 || "${TREND_DIRECTION}" != "flat" || "${ITEM_TREND_REPORT_TOTAL}" -lt 1 ]]; then
  echo "FAIL checkpoint summary mismatch: queues=${SUMMARY_QUEUES} lag=${SUMMARY_LAG} acked=${SUMMARY_ACKED}"
  exit 1
fi

echo "== batch retry_subset positive sample =="
RETRY_FAIL_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg b "${BUILDING_ID}" \
  --arg eid "${A_RETRY}" \
  --arg ok_time "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    access_events:[
      {event_id:$eid,request_id:("rq-"+$eid),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_003",type:"access_granted",actor:"qa.ckpt.retry",result:"success",occurred_at:$ok_time}
    ],
    device_events:[]
  }')"
RETRY_FAIL_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${RETRY_FAIL_PAYLOAD}")"
split_response "${RETRY_FAIL_RAW}"
require_http_code "202" "batch retry_subset source"
RETRY_FAIL_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
RETRY_FAIL_RETRYABLE="$(echo "${HTTP_BODY}" | jq -r '.totals.retryable_failed')"
RETRY_FAIL_NON_RETRYABLE="$(echo "${HTTP_BODY}" | jq -r '.totals.non_retryable_failed')"
RETRY_FAIL_SUBSET_TOTAL="$(echo "${HTTP_BODY}" | jq -r '(.retry_subset.access_events | length) + (.retry_subset.device_events | length)')"
RETRY_FAIL_SUBSET_EVENT_ID="$(echo "${HTTP_BODY}" | jq -r '.retry_subset.access_events[0].event_id')"
if [[ "${RETRY_FAIL_STATUS}" != "partial" || "${RETRY_FAIL_RETRYABLE}" != "1" || "${RETRY_FAIL_NON_RETRYABLE}" != "0" || "${RETRY_FAIL_SUBSET_TOTAL}" != "1" || "${RETRY_FAIL_SUBSET_EVENT_ID}" != "${A_RETRY}" ]]; then
  echo "FAIL retry_subset source mismatch: status=${RETRY_FAIL_STATUS} retryable=${RETRY_FAIL_RETRYABLE} non_retryable=${RETRY_FAIL_NON_RETRYABLE} subset_total=${RETRY_FAIL_SUBSET_TOTAL} subset_event=${RETRY_FAIL_SUBSET_EVENT_ID}"
  exit 1
fi
RETRY_SUBSET_PAYLOAD="$(echo "${HTTP_BODY}" | jq -c '.retry_subset')"
RETRY_REPLAY_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${RETRY_SUBSET_PAYLOAD}")"
split_response "${RETRY_REPLAY_RAW}"
require_http_code "202" "batch retry_subset replay"
RETRY_REPLAY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
RETRY_REPLAY_FAILED="$(echo "${HTTP_BODY}" | jq -r '.totals.failed')"
RETRY_REPLAY_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access.created')"
if [[ "${RETRY_REPLAY_STATUS}" != "accepted" || "${RETRY_REPLAY_FAILED}" != "0" || "${RETRY_REPLAY_CREATED}" != "1" ]]; then
  echo "FAIL retry_subset replay mismatch: status=${RETRY_REPLAY_STATUS} failed=${RETRY_REPLAY_FAILED} created=${RETRY_REPLAY_CREATED}"
  exit 1
fi

echo "PASS: gateway event checkpoint + partial batch regression complete"
