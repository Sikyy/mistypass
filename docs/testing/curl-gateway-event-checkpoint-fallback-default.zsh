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
    PORT="${API_PORT}" GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_event_checkpoint_fallback_default.log 2>&1
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
  if [[ -f /tmp/mp_gateway_event_checkpoint_fallback_default.log ]]; then
    tail -n 80 /tmp/mp_gateway_event_checkpoint_fallback_default.log
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
GW_SERIAL="MP-GW-CKPT-FB-${RUN_TAG}"
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
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-ckpt-fallback-default",source:"factory"}]}')"
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

echo "== checkpoint exceeds default fallback total (expect 409 + event_rows_fallback) =="
CHECKPOINT_EXCEED_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg t "${VALID_TIME}" \
  '{gateway_id:$gid,tenant_id:$tenant,queue:"default",checkpoint_id:"seq-fallback-1",last_request_id:"rq-fallback-1",acked_count:1,last_occurred_at:$t}')"
CHECKPOINT_EXCEED_RAW="$(bootstrap_with_token POST "/api/v1/gateway/events/checkpoint" "${CHECKPOINT_EXCEED_PAYLOAD}")"
split_response "${CHECKPOINT_EXCEED_RAW}"
require_http_code "409" "default fallback checkpoint conflict"

EXCEED_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
EXCEED_ACTION="$(echo "${HTTP_BODY}" | jq -r '.next_action')"
EXCEED_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.server_event_total')"
EXCEED_SOURCE="$(echo "${HTTP_BODY}" | jq -r '.server_total_source')"
EXCEED_QUEUE="$(echo "${HTTP_BODY}" | jq -r '.queue')"
if [[ "${EXCEED_ERROR}" != "event checkpoint acked_count exceeds server event total" || "${EXCEED_ACTION}" != "retry_with_server_event_total" || "${EXCEED_TOTAL}" != "0" || "${EXCEED_SOURCE}" != "event_rows_fallback" || "${EXCEED_QUEUE}" != "default" ]]; then
  echo "FAIL fallback checkpoint conflict mismatch: error=${EXCEED_ERROR} action=${EXCEED_ACTION} total=${EXCEED_TOTAL} source=${EXCEED_SOURCE} queue=${EXCEED_QUEUE}"
  exit 1
fi

echo "PASS: gateway checkpoint default fallback regression complete"
