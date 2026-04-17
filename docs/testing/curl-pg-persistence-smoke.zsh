#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18080}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://siky@localhost:5432/postgres?sslmode=disable}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
SERVER_LOG="${SERVER_LOG:-/tmp/mp_pg_persistence_api.log}"
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
    tail -n 80 "${SERVER_LOG}"
  fi
  exit 1
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

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
echo "== start #1 =="
start_api

echo "== login =="
LOGIN_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

echo "== create tenant/space/access/gateway/enterprise sample data =="
TENANT_PAYLOAD="$(jq -nc \
  --arg name "PG Persist Tenant ${RUN_TAG}" \
  '{name:$name,type:"company",hq_region:"ID-JK"}')"
TENANT_RAW="$(api_with_auth POST "/api/v1/tenants" "${TENANT_PAYLOAD}")"
split_response "${TENANT_RAW}"
require_http_code "201" "create tenant"
TENANT_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TENANT_ID}" "tenant.id"

BUILDING_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg name "PG Persist Building ${RUN_TAG}" \
  '{tenant_id:$tenant,name:$name,address:"Jakarta",region:"ID-JK"}')"
BUILDING_RAW="$(api_with_auth POST "/api/v1/buildings" "${BUILDING_PAYLOAD}")"
split_response "${BUILDING_RAW}"
require_http_code "201" "create building"
BUILDING_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${BUILDING_ID}" "building.id"

USER_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  --arg name "Persist User ${RUN_TAG}" \
  --arg email "persist.${RUN_TAG}@example.com" \
  '{tenant_id:$tenant,building_id:$building,name:$name,email:$email,role:"employee",status:"active",group_ids:[] }')"
USER_RAW="$(api_with_auth POST "/api/v1/users" "${USER_PAYLOAD}")"
split_response "${USER_RAW}"
require_http_code "201" "create user"
USER_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${USER_ID}" "user.id"

GW_SERIAL="MP-GW-PG-${RUN_TAG}"
GW_IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg sn "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"pg-smoke",source:"factory"}]}')"
GW_IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${GW_IMPORT_PAYLOAD}")"
split_response "${GW_IMPORT_RAW}"
require_http_code "201" "import gateway serial inventory"

GATEWAY_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
GATEWAY_RAW="$(api_with_auth POST "/api/v1/gateways/register" "${GATEWAY_PAYLOAD}")"
split_response "${GATEWAY_RAW}"
require_http_code "201" "create gateway"
GATEWAY_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${GATEWAY_ID}" "gateway.id"

EMP_DOMAIN="persist-${RUN_TAG}.sudirman.co"
EMP_EMAIL="persist.${RUN_TAG}@${EMP_DOMAIN}"
EMP_DOMAIN_PAYLOAD="$(jq -nc \
  --arg domain "${EMP_DOMAIN}" \
  '{tenant_id:"tenant_demo_jakarta",domain:$domain,status:"active"}')"
EMP_DOMAIN_RAW="$(api_with_auth POST "/api/v1/enterprise/domain-mappings" "${EMP_DOMAIN_PAYLOAD}")"
split_response "${EMP_DOMAIN_RAW}"
require_http_code "201" "create enterprise domain mapping for persistence employee"

EMP_SYNC_PAYLOAD="$(jq -nc \
  --arg email "${EMP_EMAIL}" \
  --arg ext "hris-${RUN_TAG}" \
  --arg full "Persist Emp ${RUN_TAG}" \
  '{tenant_id:"tenant_demo_jakarta",source:"manual_sync",actor:"qa.persist",employees:[{external_id:$ext,email:$email,full_name:$full,department:"IT",job_title:"Engineer",location:"Jakarta",status:"active"}]}')"
EMP_SYNC_RAW="$(api_with_auth POST "/api/v1/enterprise/employees/sync" "${EMP_SYNC_PAYLOAD}")"
split_response "${EMP_SYNC_RAW}"
require_http_code "202" "sync enterprise employees"
EMP_ID="$(echo "${HTTP_BODY}" | jq -r 'if (.items | type) == "array" and (.items | length) > 0 then .items[0].id else (.job.id // "") end')"
if [[ -z "${EMP_ID}" || "${EMP_ID}" == "null" ]]; then
  EMP_ID="n/a"
fi

ALARM_LIST_RAW="$(api_with_auth GET "/api/v1/alarms?tenant_id=tenant_demo_jakarta")"
split_response "${ALARM_LIST_RAW}"
require_http_code "200" "list alarms before update"
ALARM_ID="$(echo "${HTTP_BODY}" | jq -r '.items[0].id')"
require_non_empty "${ALARM_ID}" "alarm.id"
ALARM_UPDATE_RAW="$(api_with_auth PATCH "/api/v1/alarms/${ALARM_ID}/status?tenant_id=tenant_demo_jakarta" '{"status":"resolved"}')"
split_response "${ALARM_UPDATE_RAW}"
require_http_code "200" "update alarm status"

WALLET_TEMPLATE_PAYLOAD="$(jq -nc \
  --arg name "PG Persist Wallet ${RUN_TAG}" \
  '{tenant_id:"tenant_demo_jakarta",pass_type:"visitor",name:$name,class_id:"",style_config:{themeColor:"#24577A"},status:"active",actor:"qa.persist"}')"
WALLET_TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${WALLET_TEMPLATE_PAYLOAD}")"
split_response "${WALLET_TEMPLATE_RAW}"
require_http_code "201" "create wallet template"
WALLET_TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${WALLET_TEMPLATE_ID}" "wallet_template.id"

echo "created: tenant=${TENANT_ID} building=${BUILDING_ID} user=${USER_ID} gateway=${GATEWAY_ID} employee=${EMP_ID} alarm=${ALARM_ID} wallet_template=${WALLET_TEMPLATE_ID}"

echo "== restart API =="
stop_api
start_api

echo "== verify persisted data after restart =="
TENANTS_RAW="$(api_with_auth GET "/api/v1/tenants")"
split_response "${TENANTS_RAW}"
require_http_code "200" "list tenants after restart"
TENANT_EXISTS="$(echo "${HTTP_BODY}" | jq -r --arg id "${TENANT_ID}" '.items | map(select(.id == $id)) | length')"
if [[ "${TENANT_EXISTS}" -lt 1 ]]; then
  echo "FAIL tenant persistence: ${TENANT_ID} not found"
  exit 1
fi

BUILDINGS_RAW="$(api_with_auth GET "/api/v1/buildings?tenant_id=${TENANT_ID}")"
split_response "${BUILDINGS_RAW}"
require_http_code "200" "list buildings after restart"
BUILDING_EXISTS="$(echo "${HTTP_BODY}" | jq -r --arg id "${BUILDING_ID}" '.items | map(select(.id == $id)) | length')"
if [[ "${BUILDING_EXISTS}" -lt 1 ]]; then
  echo "FAIL building persistence: ${BUILDING_ID} not found"
  exit 1
fi

USERS_RAW="$(api_with_auth GET "/api/v1/users?tenant_id=${TENANT_ID}")"
split_response "${USERS_RAW}"
require_http_code "200" "list users after restart"
USER_EXISTS="$(echo "${HTTP_BODY}" | jq -r --arg id "${USER_ID}" '.items | map(select(.id == $id)) | length')"
if [[ "${USER_EXISTS}" -lt 1 ]]; then
  echo "FAIL user persistence: ${USER_ID} not found"
  exit 1
fi

GATEWAYS_RAW="$(api_with_auth GET "/api/v1/gateways?tenant_id=${TENANT_ID}")"
split_response "${GATEWAYS_RAW}"
require_http_code "200" "list gateways after restart"
GATEWAY_EXISTS="$(echo "${HTTP_BODY}" | jq -r --arg id "${GATEWAY_ID}" '.items | map(select(.id == $id)) | length')"
if [[ "${GATEWAY_EXISTS}" -lt 1 ]]; then
  echo "FAIL gateway persistence: ${GATEWAY_ID} not found"
  exit 1
fi

EMP_RAW="$(api_with_auth GET "/api/v1/enterprise/employees?tenant_id=tenant_demo_jakarta")"
split_response "${EMP_RAW}"
require_http_code "200" "list enterprise employees after restart"
EMP_EXISTS="$(echo "${HTTP_BODY}" | jq -r --arg email "${EMP_EMAIL}" '.items | map(select(.email == $email)) | length')"
if [[ "${EMP_EXISTS}" -lt 1 ]]; then
  echo "FAIL enterprise persistence: ${EMP_EMAIL} not found"
  exit 1
fi

ALARMS_RAW="$(api_with_auth GET "/api/v1/alarms?tenant_id=tenant_demo_jakarta")"
split_response "${ALARMS_RAW}"
require_http_code "200" "list alarms after restart"
ALARM_STATUS="$(echo "${HTTP_BODY}" | jq -r --arg id "${ALARM_ID}" '.items[] | select(.id == $id) | .status')"
if [[ "${ALARM_STATUS}" != "resolved" ]]; then
  echo "FAIL alarm persistence: ${ALARM_ID} status expected resolved got ${ALARM_STATUS}"
  exit 1
fi

WALLET_TEMPLATES_RAW="$(api_with_auth GET "/api/v1/wallet/templates?tenant_id=tenant_demo_jakarta")"
split_response "${WALLET_TEMPLATES_RAW}"
require_http_code "200" "list wallet templates after restart"
WALLET_TEMPLATE_EXISTS="$(echo "${HTTP_BODY}" | jq -r --arg id "${WALLET_TEMPLATE_ID}" '.items | map(select(.id == $id)) | length')"
if [[ "${WALLET_TEMPLATE_EXISTS}" -lt 1 ]]; then
  echo "FAIL wallet persistence: ${WALLET_TEMPLATE_ID} not found"
  exit 1
fi

EVENTS_RAW="$(api_with_auth GET "/api/v1/events/access?tenant_id=tenant_demo_jakarta")"
split_response "${EVENTS_RAW}"
require_http_code "200" "list access events after restart"

AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=tenant_demo_jakarta")"
split_response "${AUDIT_RAW}"
require_http_code "200" "list audit logs after restart"

CHANGE_LOG_RAW="$(api_with_auth GET "/api/v1/state/change-log?state_key=module_enterprise&limit=20")"
split_response "${CHANGE_LOG_RAW}"
require_http_code "200" "list state change log"
CHANGE_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
if [[ "${CHANGE_COUNT}" -lt 1 ]]; then
  echo "FAIL state change log: expected at least one enterprise change event"
  exit 1
fi

REPLAY_PAYLOAD="$(jq -nc '{state_key:"module_enterprise",from_id:0,limit:1}')"
REPLAY_RAW="$(api_with_auth POST "/api/v1/state/change-log/replay" "${REPLAY_PAYLOAD}")"
split_response "${REPLAY_RAW}"
require_http_code "200" "replay state change log"
REPLAY_APPLIED="$(echo "${HTTP_BODY}" | jq -r '.applied')"
if [[ "${REPLAY_APPLIED}" -lt 1 ]]; then
  echo "FAIL state change replay: applied should be >= 1"
  exit 1
fi

CHECKPOINT_REPLAY_PAYLOAD="$(jq -nc '{state_key:"module_enterprise",limit:500}')"
CHECKPOINT_REPLAY_RAW="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${CHECKPOINT_REPLAY_PAYLOAD}")"
split_response "${CHECKPOINT_REPLAY_RAW}"
require_http_code "200" "replay state change log from checkpoint"
CHECKPOINT_REPLAY_APPLIED="$(echo "${HTTP_BODY}" | jq -r '.applied')"
if [[ "${CHECKPOINT_REPLAY_APPLIED}" -lt 0 ]]; then
  echo "FAIL state checkpoint replay: applied should be >= 0"
  exit 1
fi
CHECKPOINT_REPLAY_FROM_ID="$(echo "${HTTP_BODY}" | jq -r '.from_id')"
if [[ "${CHECKPOINT_REPLAY_APPLIED}" -eq 0 && "${CHECKPOINT_REPLAY_FROM_ID}" -lt 1 ]]; then
  echo "FAIL state checkpoint replay: applied=0 requires from_id >= 1 (checkpoint should already be advanced)"
  exit 1
fi
CHECKPOINT_REPLAY_LAST_CHANGE_ID="$(echo "${HTTP_BODY}" | jq -r '.last_change_id')"
if [[ "${CHECKPOINT_REPLAY_LAST_CHANGE_ID}" -lt 1 ]]; then
  echo "FAIL state checkpoint replay: last_change_id should be >= 1"
  exit 1
fi

CHECKPOINT_REPLAY_2_RAW="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${CHECKPOINT_REPLAY_PAYLOAD}")"
split_response "${CHECKPOINT_REPLAY_2_RAW}"
require_http_code "200" "replay state change log from checkpoint second run"
CHECKPOINT_REPLAY_2_APPLIED="$(echo "${HTTP_BODY}" | jq -r '.applied')"
if [[ "${CHECKPOINT_REPLAY_2_APPLIED}" -ne 0 ]]; then
  echo "FAIL state checkpoint replay: second run expected applied=0 got ${CHECKPOINT_REPLAY_2_APPLIED}"
  exit 1
fi

CHECKPOINT_LIST_RAW="$(api_with_auth GET "/api/v1/state/change-log/checkpoints?state_key=module_enterprise&limit=20")"
split_response "${CHECKPOINT_LIST_RAW}"
require_http_code "200" "list state change replay checkpoints"
CHECKPOINT_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
if [[ "${CHECKPOINT_COUNT}" -lt 1 ]]; then
  echo "FAIL state checkpoint list: expected checkpoint row for module_enterprise"
  exit 1
fi
CHECKPOINT_STORED_LAST="$(echo "${HTTP_BODY}" | jq -r '.items[0].last_change_id')"
if [[ "${CHECKPOINT_STORED_LAST}" -lt "${CHECKPOINT_REPLAY_LAST_CHANGE_ID}" ]]; then
  echo "FAIL state checkpoint list: stored last_change_id should be >= replay last_change_id"
  exit 1
fi

echo "PASS: PostgreSQL persistence survived restart for tenant/space/access/gateway/enterprise/alarm/wallet (event/audit readable) + state change log/replay + checkpoint replay"
