#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18084}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://siky@localhost:5432/postgres?sslmode=disable}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
SERVER_LOG="${SERVER_LOG:-/tmp/mp_pg_replay_concurrency_api.log}"
WRITE_COUNT="${WRITE_COUNT:-40}"
CONCURRENT_NOOP="${CONCURRENT_NOOP:-10}"
REPLAY_LIMIT="${REPLAY_LIMIT:-500}"
P95_MS_THRESHOLD="${P95_MS_THRESHOLD:-3000}"
NOOP_MAX_MS_THRESHOLD="${NOOP_MAX_MS_THRESHOLD:-8000}"
CATCHUP_MIN_OPS_PER_SEC="${CATCHUP_MIN_OPS_PER_SEC:-20}"
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

function now_ms() {
  perl -MTime::HiRes=time -e 'printf("%.0f\n", time()*1000)'
}

function elapsed_ms() {
  local start_ms="$1"
  local end_ms
  end_ms="$(now_ms)"
  echo $((end_ms - start_ms))
}

function start_api() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
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
  for i in {1..100}; do
    if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "api: started on ${API_BASE_URL}"
      return
    fi
    sleep 0.2
  done

  echo "FAIL start_api: healthz not ready"
  if [[ -f "${SERVER_LOG}" ]]; then
    tail -n 120 "${SERVER_LOG}"
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
    curl -sS --connect-timeout 5 --max-time 30 -X "${method}" "${API_BASE_URL}${endpoint_path}" \
      -H "Authorization: Bearer ${AT}" \
      -H "Content-Type: application/json" \
      -d "${payload}" \
      -w $'\n%{http_code}'
    return
  fi
  curl -sS --connect-timeout 5 --max-time 30 -X "${method}" "${API_BASE_URL}${endpoint_path}" \
    -H "Authorization: Bearer ${AT}" \
    -w $'\n%{http_code}'
}

trap cleanup EXIT

if ! command -v psql >/dev/null 2>&1; then
  echo "FAIL prereq: psql not found"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "FAIL prereq: jq not found"
  exit 1
fi

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"

echo "== start api =="
start_api

echo "== login =="
LOGIN_RAW="$(curl -sS --connect-timeout 5 --max-time 30 -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

echo "== generate module_tenant change-log samples (${WRITE_COUNT}) =="
for i in $(seq 1 "${WRITE_COUNT}"); do
  TENANT_PAYLOAD="$(jq -nc --arg name "Replay Bench Tenant ${RUN_TAG}-${i}" '{name:$name,type:"company",hq_region:"ID-JK"}')"
  TENANT_RAW="$(api_with_auth POST "/api/v1/tenants" "${TENANT_PAYLOAD}")"
  split_response "${TENANT_RAW}"
  require_http_code "201" "create tenant #${i}"
done

echo "== read module_tenant latest change_id =="
LATEST_CHANGE_RAW="$(api_with_auth GET "/api/v1/state/change-log?state_key=module_tenant&limit=1")"
split_response "${LATEST_CHANGE_RAW}"
require_http_code "200" "list latest module_tenant change"
LATEST_CHANGE_ID="$(echo "${HTTP_BODY}" | jq -r 'if (.items | length) > 0 then .items[0].id else 0 end')"
if [[ "${LATEST_CHANGE_ID}" -lt 1 ]]; then
  echo "FAIL latest change_id: expected >=1, got ${LATEST_CHANGE_ID}"
  exit 1
fi

echo "== reset replay checkpoint for module_tenant to 0 =="
psql "${DATABASE_URL}" -Atc "insert into mistypass_change_replay_checkpoints (state_key, last_change_id, updated_at) values ('module_tenant', 0, now()) on conflict (state_key) do update set last_change_id=0, updated_at=now();" >/dev/null

echo "== catch-up replay from checkpoint =="
CATCHUP_PAYLOAD="$(jq -nc --arg sk "module_tenant" --argjson limit "${REPLAY_LIMIT}" '{state_key:$sk,limit:$limit}')"
CATCHUP_START_MS="$(now_ms)"
CATCHUP_RAW="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${CATCHUP_PAYLOAD}")"
CATCHUP_LATENCY_MS="$(elapsed_ms "${CATCHUP_START_MS}")"
split_response "${CATCHUP_RAW}"
require_http_code "200" "catch-up replay"
CATCHUP_APPLIED="$(echo "${HTTP_BODY}" | jq -r '.applied')"
CATCHUP_LAST_CHANGE_ID="$(echo "${HTTP_BODY}" | jq -r '.last_change_id')"
if [[ "${CATCHUP_APPLIED}" -lt 1 ]]; then
  echo "FAIL catch-up replay: expected applied >=1, got ${CATCHUP_APPLIED}"
  exit 1
fi
if [[ "${CATCHUP_LAST_CHANGE_ID}" -lt "${LATEST_CHANGE_ID}" ]]; then
  echo "FAIL catch-up replay: last_change_id=${CATCHUP_LAST_CHANGE_ID} should cover latest_change_id=${LATEST_CHANGE_ID}"
  exit 1
fi
CATCHUP_THROUGHPUT="$(awk -v applied="${CATCHUP_APPLIED}" -v ms="${CATCHUP_LATENCY_MS}" 'BEGIN { if (ms <= 0) { printf "%.2f", applied * 1000 } else { printf "%.2f", (applied * 1000.0) / ms } }')"

echo "== concurrent replay no-op stability (${CONCURRENT_NOOP} workers) =="
TMP_RESULT_FILE="$(mktemp /tmp/mp_pg_replay_bench_results.XXXXXX)"
typeset -a worker_pids
for i in $(seq 1 "${CONCURRENT_NOOP}"); do
  (
    worker_start_ms="$(now_ms)"
    worker_raw="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${CATCHUP_PAYLOAD}")"
    worker_latency_ms="$(elapsed_ms "${worker_start_ms}")"
    worker_code="${worker_raw##*$'\n'}"
    worker_body="${worker_raw%$'\n'*}"
    worker_applied="$(echo "${worker_body}" | jq -r '.applied // -1' 2>/dev/null || echo "-1")"
    echo "${i},${worker_code},${worker_latency_ms},${worker_applied}" >>"${TMP_RESULT_FILE}"
  ) &
  worker_pids+=($!)
done
for pid in "${worker_pids[@]}"; do
  wait "${pid}"
done

TOTAL_WORKERS="$(wc -l <"${TMP_RESULT_FILE}" | tr -d '[:space:]')"
FAIL_COUNT="$(awk -F, '$2 != "200" || $4 != "0" {c++} END {print c+0}' "${TMP_RESULT_FILE}")"
SORTED_FILE="$(mktemp /tmp/mp_pg_replay_bench_sorted.XXXXXX)"
sort -t, -k3n "${TMP_RESULT_FILE}" >"${SORTED_FILE}"
P95_INDEX="$(awk -v n="${TOTAL_WORKERS}" 'BEGIN { idx = int((n*95 + 99) / 100); if (idx < 1) idx = 1; print idx }')"
P95_MS="$(awk -F, -v idx="${P95_INDEX}" 'NR==idx {print $3}' "${SORTED_FILE}")"
MAX_MS="$(awk -F, 'BEGIN{m=0} {if ($3>m) m=$3} END{print m+0}' "${TMP_RESULT_FILE}")"
rm -f "${TMP_RESULT_FILE}" "${SORTED_FILE}"

CHECKPOINT_AFTER_RAW="$(api_with_auth GET "/api/v1/state/change-log/checkpoints?state_key=module_tenant&limit=1")"
split_response "${CHECKPOINT_AFTER_RAW}"
require_http_code "200" "list checkpoint after bench"
CHECKPOINT_AFTER_ID="$(echo "${HTTP_BODY}" | jq -r 'if (.items | length) > 0 then .items[0].last_change_id else 0 end')"
if [[ "${CHECKPOINT_AFTER_ID}" -lt "${LATEST_CHANGE_ID}" ]]; then
  echo "FAIL checkpoint after bench: last_change_id=${CHECKPOINT_AFTER_ID} should cover latest_change_id=${LATEST_CHANGE_ID}"
  exit 1
fi

if [[ "${FAIL_COUNT}" -ne 0 ]]; then
  echo "FAIL concurrent no-op replay: workers=${TOTAL_WORKERS} fail_count=${FAIL_COUNT}"
  exit 1
fi
if [[ "${P95_MS}" -gt "${P95_MS_THRESHOLD}" ]]; then
  echo "FAIL concurrent no-op replay latency: p95_ms=${P95_MS} threshold=${P95_MS_THRESHOLD}"
  exit 1
fi
if [[ "${MAX_MS}" -gt "${NOOP_MAX_MS_THRESHOLD}" ]]; then
  echo "FAIL concurrent no-op replay max latency: max_ms=${MAX_MS} threshold=${NOOP_MAX_MS_THRESHOLD}"
  exit 1
fi
if awk -v throughput="${CATCHUP_THROUGHPUT}" -v threshold="${CATCHUP_MIN_OPS_PER_SEC}" 'BEGIN {exit !(throughput+0 < threshold+0)}'; then
  echo "FAIL catch-up replay throughput: ops_per_sec=${CATCHUP_THROUGHPUT} threshold=${CATCHUP_MIN_OPS_PER_SEC}"
  exit 1
fi

echo "summary: catchup_applied=${CATCHUP_APPLIED} catchup_latency_ms=${CATCHUP_LATENCY_MS} catchup_throughput_ops_per_sec=${CATCHUP_THROUGHPUT} catchup_min_ops_per_sec=${CATCHUP_MIN_OPS_PER_SEC}"
echo "summary: noop_workers=${TOTAL_WORKERS} noop_failures=${FAIL_COUNT} noop_p95_ms=${P95_MS} noop_p95_threshold_ms=${P95_MS_THRESHOLD} noop_max_ms=${MAX_MS} noop_max_threshold_ms=${NOOP_MAX_MS_THRESHOLD}"
echo "PASS: pg replay concurrency baseline complete"
