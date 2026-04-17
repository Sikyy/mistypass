#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18082}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://siky@localhost:5432/postgres?sslmode=disable}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
SERVER_LOG="${SERVER_LOG:-/tmp/mp_pg_replay_retry_api.log}"
API_PID=""
BAD_ROW_ID=""

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
  if [[ -n "${BAD_ROW_ID}" ]]; then
    psql "${DATABASE_URL}" -Atc "delete from mistypass_change_log where id = ${BAD_ROW_ID}" >/dev/null 2>&1 || true
  fi
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

if ! command -v psql >/dev/null 2>&1; then
  echo "FAIL prereq: psql not found"
  exit 1
fi

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"

echo "== start api =="
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

echo "== cleanup historical forced replay rows =="
psql "${DATABASE_URL}" -Atc "delete from mistypass_change_log where state_key = 'module_tenant' and (payload_hash in ('forced_bad_replay_hash','forced_valid_replay_hash') or payload::text = '\"forced-bad-payload\"');" >/dev/null

echo "== ensure module_tenant has fresh state change =="
TENANT_PAYLOAD="$(jq -nc --arg name "Replay Retry Tenant ${RUN_TAG}" '{name:$name,type:"company",hq_region:"ID-JK"}')"
TENANT_RAW="$(api_with_auth POST "/api/v1/tenants" "${TENANT_PAYLOAD}")"
split_response "${TENANT_RAW}"
require_http_code "201" "create tenant"

echo "== read checkpoint before fault injection =="
CHECKPOINT_BEFORE_RAW="$(api_with_auth GET "/api/v1/state/change-log/checkpoints?state_key=module_tenant&limit=1")"
split_response "${CHECKPOINT_BEFORE_RAW}"
require_http_code "200" "list checkpoint before"
CHECKPOINT_BEFORE="$(echo "${HTTP_BODY}" | jq -r 'if (.items | length) > 0 then .items[0].last_change_id else 0 end')"

if [[ "${CHECKPOINT_BEFORE}" -lt 1 ]]; then
  echo "FAIL checkpoint before: expected >= 1, got ${CHECKPOINT_BEFORE}"
  exit 1
fi

echo "== inject malformed + valid change-log rows =="
BAD_ROW_ID="$(psql "${DATABASE_URL}" -Atc "insert into mistypass_change_log (state_key, change_type, payload_hash, payload, created_at) values ('module_tenant','snapshot_saved','forced_bad_replay_hash','\"forced-bad-payload\"'::jsonb, now()) returning id;" | head -n 1 | tr -d '[:space:]')"
require_non_empty "${BAD_ROW_ID}" "inject bad change row"
VALID_ROW_ID="$(psql "${DATABASE_URL}" -Atc "insert into mistypass_change_log (state_key, change_type, payload_hash, payload, created_at) select 'module_tenant','snapshot_saved','forced_valid_replay_hash',payload,now() from mistypass where state_key='module_tenant' returning id;" | head -n 1 | tr -d '[:space:]')"
require_non_empty "${VALID_ROW_ID}" "inject valid change row"

echo "== replay from checkpoint should fail and keep checkpoint unchanged =="
REPLAY_PAYLOAD="$(jq -nc '{state_key:"module_tenant",limit:500}')"
REPLAY_FAIL_RAW="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${REPLAY_PAYLOAD}")"
split_response "${REPLAY_FAIL_RAW}"
if [[ "${HTTP_CODE}" != "500" ]]; then
  echo "FAIL replay failure status: expected HTTP 500, got ${HTTP_CODE}"
  echo "${HTTP_BODY}"
  exit 1
fi

CHECKPOINT_AFTER_FAIL_RAW="$(api_with_auth GET "/api/v1/state/change-log/checkpoints?state_key=module_tenant&limit=1")"
split_response "${CHECKPOINT_AFTER_FAIL_RAW}"
require_http_code "200" "list checkpoint after failure"
CHECKPOINT_AFTER_FAIL="$(echo "${HTTP_BODY}" | jq -r 'if (.items | length) > 0 then .items[0].last_change_id else 0 end')"
if [[ "${CHECKPOINT_AFTER_FAIL}" -ne "${CHECKPOINT_BEFORE}" ]]; then
  echo "FAIL checkpoint drift on failure: before=${CHECKPOINT_BEFORE} after_fail=${CHECKPOINT_AFTER_FAIL}"
  exit 1
fi

echo "== remove malformed row and retry replay =="
psql "${DATABASE_URL}" -Atc "delete from mistypass_change_log where id = ${BAD_ROW_ID}" >/dev/null
BAD_ROW_ID=""

REPLAY_OK_RAW="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${REPLAY_PAYLOAD}")"
split_response "${REPLAY_OK_RAW}"
require_http_code "200" "replay after cleanup"
REPLAY_OK_APPLIED="$(echo "${HTTP_BODY}" | jq -r '.applied')"
REPLAY_OK_LAST_CHANGE_ID="$(echo "${HTTP_BODY}" | jq -r '.last_change_id')"
if [[ "${REPLAY_OK_APPLIED}" -lt 1 ]]; then
  echo "FAIL replay after cleanup: expected applied >= 1, got ${REPLAY_OK_APPLIED}"
  exit 1
fi
if [[ "${REPLAY_OK_LAST_CHANGE_ID}" -lt "${VALID_ROW_ID}" ]]; then
  echo "FAIL replay after cleanup: last_change_id=${REPLAY_OK_LAST_CHANGE_ID} should cover valid_row_id=${VALID_ROW_ID}"
  exit 1
fi

echo "== idempotent replay should be no-op =="
REPLAY_NOOP_RAW="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${REPLAY_PAYLOAD}")"
split_response "${REPLAY_NOOP_RAW}"
require_http_code "200" "replay no-op"
REPLAY_NOOP_APPLIED="$(echo "${HTTP_BODY}" | jq -r '.applied')"
if [[ "${REPLAY_NOOP_APPLIED}" -ne 0 ]]; then
  echo "FAIL replay no-op: expected applied=0, got ${REPLAY_NOOP_APPLIED}"
  exit 1
fi

echo "PASS: checkpoint replay retry + idempotent replay baseline complete"
