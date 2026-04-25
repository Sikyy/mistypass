#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-8080}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
PRIORITY_QUEUE="${PRIORITY_QUEUE:-priority}"
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
    PORT="${API_PORT}" GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_event_multi_queue_isolation.log 2>&1
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
  if [[ -f /tmp/mp_gateway_event_multi_queue_isolation.log ]]; then
    tail -n 80 /tmp/mp_gateway_event_multi_queue_isolation.log
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
GW_SERIAL="MP-GW-MQI-${RUN_TAG}"
A_DEFAULT="gwea-mqi-default-${RUN_TAG}"
D_DEFAULT="gwed-mqi-default-${RUN_TAG}"
A_PRIORITY="gwea-mqi-priority-${RUN_TAG}"
D_PRIORITY="gwed-mqi-priority-${RUN_TAG}"
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
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-multi-queue-isolation",source:"factory"}]}')"
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

echo "== source batch on default queue =="
DEFAULT_BATCH_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg b "${BUILDING_ID}" \
  --arg a "${A_DEFAULT}" \
  --arg d "${D_DEFAULT}" \
  --arg t "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    queue:"default",
    access_events:[
      {event_id:$a,request_id:("rq-"+$a),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_001",type:"access_granted",actor:"qa.mqi.default",result:"success",occurred_at:$t}
    ],
    device_events:[
      {event_id:$d,request_id:("rq-"+$d),building_id:$b,type:"gateway_event",detail:"multi queue isolation default",result:"warning",occurred_at:$t}
    ]
  }')"
DEFAULT_BATCH_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${DEFAULT_BATCH_PAYLOAD}")"
split_response "${DEFAULT_BATCH_RAW}"
require_http_code "202" "default queue source batch"
DEFAULT_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${DEFAULT_HINT_TOTAL}" != "2" ]]; then
  echo "FAIL default queue_hint.server_ingested_total mismatch: ${DEFAULT_HINT_TOTAL}"
  exit 1
fi

echo "== source batch on priority queue =="
PRIORITY_BATCH_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${PRIORITY_QUEUE}" \
  --arg b "${BUILDING_ID}" \
  --arg a "${A_PRIORITY}" \
  --arg d "${D_PRIORITY}" \
  --arg t "${VALID_TIME}" \
  '{
    gateway_id:$gid,
    tenant_id:$tenant,
    queue:$queue,
    access_events:[
      {event_id:$a,request_id:("rq-"+$a),building_id:$b,area_id:"area_demo_001",door_id:"door_jkt_001",type:"access_granted",actor:"qa.mqi.priority",result:"success",occurred_at:$t}
    ],
    device_events:[
      {event_id:$d,request_id:("rq-"+$d),building_id:$b,type:"gateway_event",detail:"multi queue isolation priority",result:"warning",occurred_at:$t}
    ]
  }')"
PRIORITY_BATCH_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/batch" "${PRIORITY_BATCH_PAYLOAD}")"
split_response "${PRIORITY_BATCH_RAW}"
require_http_code "202" "priority queue source batch"
PRIORITY_HINT_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.queue_hint.server_ingested_total')"
if [[ "${PRIORITY_HINT_TOTAL}" != "2" ]]; then
  echo "FAIL priority queue_hint.server_ingested_total mismatch: ${PRIORITY_HINT_TOTAL}"
  exit 1
fi

echo "== report default queue checkpoint acked=2 =="
DEFAULT_CKPT_OK_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:"default",checkpoint_id:"mqi-default-seq-2",last_request_id:"rq-mqi-default-2",acked_count:2,last_occurred_at:$t}')"
DEFAULT_CKPT_OK_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${DEFAULT_CKPT_OK_PAYLOAD}")"
split_response "${DEFAULT_CKPT_OK_RAW}"
require_http_code "200" "default checkpoint acked=2"

echo "== report priority queue checkpoint acked=1 =="
PRIORITY_CKPT_1_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${PRIORITY_QUEUE}" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:$queue,checkpoint_id:"mqi-priority-seq-1",last_request_id:"rq-mqi-priority-1",acked_count:1,last_occurred_at:$t}')"
PRIORITY_CKPT_1_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${PRIORITY_CKPT_1_PAYLOAD}")"
split_response "${PRIORITY_CKPT_1_RAW}"
require_http_code "200" "priority checkpoint acked=1"

echo "== regress default queue checkpoint to acked=1 (expect 409) =="
DEFAULT_CKPT_REGRESS_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:"default",checkpoint_id:"mqi-default-seq-1-regress",last_request_id:"rq-mqi-default-1-regress",acked_count:1,last_occurred_at:$t}')"
DEFAULT_CKPT_REGRESS_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${DEFAULT_CKPT_REGRESS_PAYLOAD}")"
split_response "${DEFAULT_CKPT_REGRESS_RAW}"
require_http_code "409" "default checkpoint regression conflict"
REGRESS_ACTION="$(echo "${HTTP_BODY}" | jq -r '.next_action')"
REGRESS_QUEUE="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.queue')"
REGRESS_ACKED="$(echo "${HTTP_BODY}" | jq -r '.checkpoint.acked_count')"
if [[ "${REGRESS_ACTION}" != "retry_with_non_regressing_acked_count" || "${REGRESS_QUEUE}" != "default" || "${REGRESS_ACKED}" != "2" ]]; then
  echo "FAIL default regression payload mismatch: action=${REGRESS_ACTION} queue=${REGRESS_QUEUE} acked=${REGRESS_ACKED}"
  exit 1
fi

echo "== priority queue continues to advance to acked=2 =="
PRIORITY_CKPT_2_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg queue "${PRIORITY_QUEUE}" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:$queue,checkpoint_id:"mqi-priority-seq-2",last_request_id:"rq-mqi-priority-2",acked_count:2,last_occurred_at:$t}')"
PRIORITY_CKPT_2_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${PRIORITY_CKPT_2_PAYLOAD}")"
split_response "${PRIORITY_CKPT_2_RAW}"
require_http_code "200" "priority checkpoint keeps advancing"
PRIORITY_ACKED_2="$(echo "${HTTP_BODY}" | jq -r '.acked_count')"
PRIORITY_QUEUE_2="$(echo "${HTTP_BODY}" | jq -r '.queue')"
if [[ "${PRIORITY_ACKED_2}" != "2" || "${PRIORITY_QUEUE_2}" != "${PRIORITY_QUEUE}" ]]; then
  echo "FAIL priority progress mismatch after default conflict: acked=${PRIORITY_ACKED_2} queue=${PRIORITY_QUEUE_2}"
  exit 1
fi

echo "PASS: gateway multi-queue checkpoint isolation regression complete"
