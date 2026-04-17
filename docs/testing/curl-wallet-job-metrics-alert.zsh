#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18082}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
WALLET_DLQ_ALERT_THRESHOLD="${WALLET_DLQ_ALERT_THRESHOLD:-1}"
WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS="${WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS:-${WALLET_JOB_METRICS_DEFAULT_WINDOW:-600}}"
WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY="${WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY:-4}"
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

function ensure_api_running() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "api: already running"
    return
  fi

  echo "api: starting local server"
  (
    cd api
    PORT="${API_PORT}" \
    WALLET_DLQ_ALERT_THRESHOLD="${WALLET_DLQ_ALERT_THRESHOLD}" \
    WALLET_JOB_METRICS_DEFAULT_WINDOW="${WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS}" \
    WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY="${WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY}" \
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_job_metrics_alert_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_job_metrics_alert_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_job_metrics_alert_api.log
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

echo "== create template =="
TEMPLATE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg name "Metrics Alert ${RUN_TAG}" \
  '{tenant_id:$tenant,pass_type:"employee",name:$name,status:"active",actor:"qa.metrics.alert"}')"
TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${TEMPLATE_PAYLOAD}")"
split_response "${TEMPLATE_RAW}"
require_http_code "201" "create template"
TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TEMPLATE_ID}" "template.id"

echo "== queue jobs =="
QUEUE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg t1 "usr-metrics-alert-1-${RUN_TAG}" \
  --arg t2 "usr-metrics-alert-2-${RUN_TAG}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:[$t1,$t2],execution_mode:"queued",actor:"qa.metrics.alert"}')"
QUEUE_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${QUEUE_PAYLOAD}")"
split_response "${QUEUE_RAW}"
require_http_code "202" "issue queued batch"

echo "== set template inactive =="
INACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" '{"status":"inactive","actor":"qa.metrics.alert"}')"
split_response "${INACTIVE_RAW}"
require_http_code "200" "set template inactive"

echo "== process queued jobs (expect dlq + configured default max_retry) =="
PROCESS_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:20,worker_count:2,base_backoff_ms:10,max_backoff_ms:50,actor:"qa.metrics.alert.worker"}')"
PROCESS_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_RAW}"
require_http_code "200" "process queued jobs"
DLQ_COUNT="$(echo "${HTTP_BODY}" | jq -r '.dlq')"
MAX_RETRY_USED="$(echo "${HTTP_BODY}" | jq -r '.max_retry')"
if [[ "${DLQ_COUNT}" -lt 2 ]]; then
  echo "FAIL process queued jobs: expected dlq>=2 got ${DLQ_COUNT}"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${MAX_RETRY_USED}" -ne "${WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY}" ]]; then
  echo "FAIL process queued jobs: expected max_retry=${WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY} got ${MAX_RETRY_USED}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== check jobs metrics alert threshold =="
METRICS_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/metrics?tenant_id=${TENANT_ID}")"
split_response "${METRICS_RAW}"
require_http_code "200" "wallet jobs metrics"
METRICS_THRESHOLD="$(echo "${HTTP_BODY}" | jq -r '.dlq_alert_threshold')"
METRICS_WINDOW_SECONDS="$(echo "${HTTP_BODY}" | jq -r '.window.window_seconds')"
ALERT_MATCH_COUNT="$(echo "${HTTP_BODY}" | jq -r '.alerts | map(select(.type=="dlq_error_code_threshold" and .error_code=="template_inactive" and .count >= '"${WALLET_DLQ_ALERT_THRESHOLD}"')) | length')"
if [[ "${METRICS_THRESHOLD}" -ne "${WALLET_DLQ_ALERT_THRESHOLD}" ]]; then
  echo "FAIL wallet jobs metrics: expected threshold=${WALLET_DLQ_ALERT_THRESHOLD} got ${METRICS_THRESHOLD}"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${METRICS_WINDOW_SECONDS}" -ne "${WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS}" ]]; then
  echo "FAIL wallet jobs metrics: expected window_seconds=${WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS} got ${METRICS_WINDOW_SECONDS}"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${ALERT_MATCH_COUNT}" -lt 1 ]]; then
  echo "FAIL wallet jobs metrics: expected template_inactive alert count >= 1"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "PASS: wallet jobs metrics alert threshold regression complete"
