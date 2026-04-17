#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18081}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
SERVER_LOG="${SERVER_LOG:-/tmp/mp_enterprise_sync_worker_alert_api.log}"
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
    stop_port_processes
  fi

  (
    cd api
    PORT="${API_PORT}" \
      DATABASE_URL="${DATABASE_URL}" \
      GOCACHE=/tmp/go-build \
      ENTERPRISE_SYNC_RECONCILE_WORKER_ENABLED=true \
      ENTERPRISE_SYNC_RECONCILE_WORKER_INTERVAL=1s \
      ENTERPRISE_SYNC_RECONCILE_WORKER_BATCH_SIZE=10 \
      ENTERPRISE_SYNC_RECONCILE_WORKER_MAX_ATTEMPTS=1 \
      ENTERPRISE_SYNC_RECONCILE_WORKER_RETRY_COOLDOWN=0s \
      ENTERPRISE_SYNC_RECONCILE_WORKER_ALERT_FAILURE_THRESHOLD=1 \
      ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR=true \
      ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR_TENANT_ID="${TENANT_ID}" \
      go run ./cmd/api >"${SERVER_LOG}" 2>&1
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

function seed_pending_worker_record() {
  local payload
  payload="$(jq -nc \
    --arg tenant_id "${TENANT_ID}" \
    --arg request_id "${REQUEST_ID}" \
    --arg record_key "${RECORD_KEY}" \
    --arg now "${NOW_UTC}" \
    --arg job_id "${JOB_ID}" \
    --arg employee_id "${EMPLOYEE_ID}" \
    --arg employee_email "${EMPLOYEE_EMAIL}" \
    '{
      domain_mappings: [],
      idp_configs: {},
      employees: [],
      sync_jobs: [],
      sync_request_records: {
        ($record_key): {
          request_id: $request_id,
          tenant_id: $tenant_id,
          result: {
            job: {
              id: $job_id,
              tenant_id: $tenant_id,
              source: "manual_sync",
              status: "completed",
              total: 1,
              created: 1,
              updated: 0,
              deactivated: 0,
              rejected: 0,
              actor: "qa",
              started_at: $now,
              ended_at: $now
            },
            items: [
              {
                id: $employee_id,
                tenant_id: $tenant_id,
                external_id: "worker-alert-external-1",
                email: $employee_email,
                full_name: "Worker Alert QA",
                department: "IT",
                job_title: "Engineer",
                location: "Lab",
                access_role: "resident",
                building_id: "building_demo_001",
                group_ids: [],
                status: "active",
                source: "manual_sync",
                last_synced_at: $now
              }
            ]
          },
          access_applied: false,
          access_created: 0,
          access_updated: 0,
          access_rejected: 0,
          access_attempt_count: 0,
          created_at: $now
        }
      }
    }')"

  psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -q <<'SQL'
create table if not exists mistypass (
  state_key text primary key,
  payload jsonb not null,
  updated_at timestamptz not null default now()
);
SQL

  psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -q -v payload="${payload}" <<'SQL'
insert into mistypass (state_key, payload, updated_at)
values ('module_enterprise', :'payload'::jsonb, now())
on conflict (state_key) do update
set payload = excluded.payload,
    updated_at = now();
SQL
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
TENANT_ID="tenant_worker_alert_${RUN_TAG}"
REQUEST_ID="req_worker_alert_${RUN_TAG}"
RECORD_KEY="${TENANT_ID}:${REQUEST_ID}"
JOB_ID="syn_worker_alert_${RUN_TAG}"
EMPLOYEE_ID="emp_worker_alert_${RUN_TAG}"
EMPLOYEE_EMAIL="worker.alert.${RUN_TAG}@example.com"
NOW_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "== seed enterprise pending record for worker =="
seed_pending_worker_record
echo "seed: tenant_id=${TENANT_ID} request_id=${REQUEST_ID}"

echo "== start api with worker enabled + forced error =="
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

echo "== wait for worker alert audit log =="
ALERT_FOUND="false"
for i in {1..80}; do
  AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=enterprise_sync_reconcile_worker_alert&source=enterprise_sync_worker&limit=5")"
  split_response "${AUDIT_RAW}"
  require_http_code "200" "audit-log query"
  ALERT_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
  if [[ "${ALERT_COUNT}" -ge 1 ]]; then
    ALERT_ACTION="$(echo "${HTTP_BODY}" | jq -r '.items[0].action')"
    ALERT_SOURCE="$(echo "${HTTP_BODY}" | jq -r '.items[0].source')"
    ALERT_TARGET="$(echo "${HTTP_BODY}" | jq -r '.items[0].target')"
    if [[ "${ALERT_ACTION}" != "enterprise_sync_reconcile_worker_alert" ]]; then
      echo "FAIL audit action mismatch: ${ALERT_ACTION}"
      exit 1
    fi
    if [[ "${ALERT_SOURCE}" != "enterprise_sync_worker" ]]; then
      echo "FAIL audit source mismatch: ${ALERT_SOURCE}"
      exit 1
    fi
    if [[ "${ALERT_TARGET}" != *"failed=1"* || "${ALERT_TARGET}" != *"threshold=1"* ]]; then
      echo "FAIL audit target mismatch: ${ALERT_TARGET}"
      exit 1
    fi
    ALERT_FOUND="true"
    echo "worker alert: action=${ALERT_ACTION} source=${ALERT_SOURCE} target=${ALERT_TARGET}"
    break
  fi
  sleep 0.25
done

if [[ "${ALERT_FOUND}" != "true" ]]; then
  echo "FAIL worker alert: no enterprise_sync_reconcile_worker_alert audit log found"
  if [[ -f "${SERVER_LOG}" ]]; then
    tail -n 120 "${SERVER_LOG}"
  fi
  exit 1
fi

echo "== verify structured worker alert API =="
STRUCTURED_RAW="$(api_with_auth GET "/api/v1/enterprise/sync-worker-alerts?tenant_id=${TENANT_ID}&since=2000-01-01T00:00:00Z&until=2100-01-01T00:00:00Z&limit=5")"
split_response "${STRUCTURED_RAW}"
require_http_code "200" "sync-worker-alerts query"
STRUCTURED_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
if [[ "${STRUCTURED_COUNT}" -lt 1 ]]; then
  echo "FAIL sync-worker-alerts: expected >=1 item, got ${STRUCTURED_COUNT}"
  exit 1
fi
STRUCTURED_FAILED="$(echo "${HTTP_BODY}" | jq -r '.items[0].failed')"
STRUCTURED_THRESHOLD="$(echo "${HTTP_BODY}" | jq -r '.items[0].threshold')"
if [[ "${STRUCTURED_FAILED}" != "1" || "${STRUCTURED_THRESHOLD}" != "1" ]]; then
  echo "FAIL sync-worker-alerts metrics mismatch: failed=${STRUCTURED_FAILED} threshold=${STRUCTURED_THRESHOLD}"
  exit 1
fi
echo "sync-worker-alerts: failed=${STRUCTURED_FAILED} threshold=${STRUCTURED_THRESHOLD}"

echo "== verify worker alert summary API =="
SUMMARY_RAW="$(api_with_auth GET "/api/v1/enterprise/sync-worker-alerts/summary?tenant_id=${TENANT_ID}&since=2000-01-01T00:00:00Z&until=2100-01-01T00:00:00Z&limit=5")"
split_response "${SUMMARY_RAW}"
require_http_code "200" "sync-worker-alerts summary query"
SUMMARY_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
if [[ "${SUMMARY_COUNT}" -lt 1 ]]; then
  echo "FAIL sync-worker-alerts summary: expected >=1 item, got ${SUMMARY_COUNT}"
  exit 1
fi
SUMMARY_TENANT_ID="$(echo "${HTTP_BODY}" | jq -r '.items[0].tenant_id')"
SUMMARY_ALERT_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items[0].count')"
SUMMARY_LAST_FAILED="$(echo "${HTTP_BODY}" | jq -r '.items[0].last_failed')"
SUMMARY_LAST_THRESHOLD="$(echo "${HTTP_BODY}" | jq -r '.items[0].last_threshold')"
if [[ "${SUMMARY_TENANT_ID}" != "${TENANT_ID}" ]]; then
  echo "FAIL sync-worker-alerts summary tenant mismatch: ${SUMMARY_TENANT_ID}"
  exit 1
fi
if [[ "${SUMMARY_ALERT_COUNT}" -lt 1 ]]; then
  echo "FAIL sync-worker-alerts summary count mismatch: ${SUMMARY_ALERT_COUNT}"
  exit 1
fi
if [[ "${SUMMARY_LAST_FAILED}" != "1" || "${SUMMARY_LAST_THRESHOLD}" != "1" ]]; then
  echo "FAIL sync-worker-alerts summary metric mismatch: last_failed=${SUMMARY_LAST_FAILED} last_threshold=${SUMMARY_LAST_THRESHOLD}"
  exit 1
fi
echo "sync-worker-alerts summary: tenant=${SUMMARY_TENANT_ID} count=${SUMMARY_ALERT_COUNT} last_failed=${SUMMARY_LAST_FAILED}"

echo "== verify request audit fields updated by worker =="
REQUEST_RAW="$(api_with_auth GET "/api/v1/enterprise/sync-requests?tenant_id=${TENANT_ID}&request_id=${REQUEST_ID}")"
split_response "${REQUEST_RAW}"
require_http_code "200" "sync-request query"
ATTEMPT_COUNT="$(echo "${HTTP_BODY}" | jq -r '.item.access_attempt_count')"
LAST_ERROR="$(echo "${HTTP_BODY}" | jq -r '.item.last_access_error')"
if [[ "${ATTEMPT_COUNT}" != "1" ]]; then
  echo "FAIL worker retry audit: expected access_attempt_count=1 got ${ATTEMPT_COUNT}"
  exit 1
fi
if [[ "${LAST_ERROR}" != *"forced enterprise sync reconcile worker apply failure"* ]]; then
  echo "FAIL worker retry audit last_access_error mismatch: ${LAST_ERROR}"
  exit 1
fi
echo "sync request: attempt_count=${ATTEMPT_COUNT} last_error=${LAST_ERROR}"

echo "== verify max_attempts stop further retries =="
sleep 2
REQUEST_RAW_2="$(api_with_auth GET "/api/v1/enterprise/sync-requests?tenant_id=${TENANT_ID}&request_id=${REQUEST_ID}")"
split_response "${REQUEST_RAW_2}"
require_http_code "200" "sync-request query second check"
ATTEMPT_COUNT_2="$(echo "${HTTP_BODY}" | jq -r '.item.access_attempt_count')"
if [[ "${ATTEMPT_COUNT_2}" != "1" ]]; then
  echo "FAIL max_attempts guard: expected access_attempt_count to stay 1, got ${ATTEMPT_COUNT_2}"
  exit 1
fi
echo "max_attempts guard: attempt_count_stable=${ATTEMPT_COUNT_2}"

echo "PASS: enterprise sync worker alert regression complete"
