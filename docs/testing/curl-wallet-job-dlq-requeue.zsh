#!/usr/bin/env zsh
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_dlq_requeue_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_dlq_requeue_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_dlq_requeue_api.log
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

echo "== create wallet template =="
TEMPLATE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg name "DLQ Template ${RUN_TAG}" \
  '{tenant_id:$tenant,pass_type:"employee",name:$name,status:"active",actor:"qa.dlq"}')"
TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${TEMPLATE_PAYLOAD}")"
split_response "${TEMPLATE_RAW}"
require_http_code "201" "create wallet template"
TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TEMPLATE_ID}" "template.id"

echo "== issue batch in queued mode =="
TARGET_A="usr-dlq-a-${RUN_TAG}"
QUEUE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg target_a "${TARGET_A}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:[$target_a],execution_mode:"queued",actor:"qa.dlq"}')"
QUEUE_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${QUEUE_PAYLOAD}")"
split_response "${QUEUE_RAW}"
require_http_code "202" "issue batch queued"
JOB_ID="$(echo "${HTTP_BODY}" | jq -r '.items[] | select(.status=="pending") | .id' | head -n1)"
require_non_empty "${JOB_ID}" "queued job id"

echo "== set template inactive to force non-retryable process failure =="
INACTIVE_PAYLOAD='{"status":"inactive","actor":"qa.dlq"}'
INACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" "${INACTIVE_PAYLOAD}")"
split_response "${INACTIVE_RAW}"
require_http_code "200" "set template inactive"

echo "== process jobs (expect dlq) =="
PROCESS_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:10,worker_count:1,max_retry:3,base_backoff_ms:10,max_backoff_ms:50,actor:"qa.dlq.worker"}')"
PROCESS_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_RAW}"
require_http_code "200" "process jobs to dlq"
DLQ_COUNT="$(echo "${HTTP_BODY}" | jq -r '.dlq')"
if [[ "${DLQ_COUNT}" -lt 1 ]]; then
  echo "FAIL process jobs to dlq: expected dlq>=1 got ${DLQ_COUNT}"
  echo "${HTTP_BODY}"
  exit 1
fi

JOB_DLQ_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/${JOB_ID}?tenant_id=${TENANT_ID}")"
split_response "${JOB_DLQ_RAW}"
require_http_code "200" "get dlq job"
JOB_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
JOB_ERROR_CODE="$(echo "${HTTP_BODY}" | jq -r '.error_code')"
if [[ "${JOB_STATUS}" != "dlq" ]]; then
  echo "FAIL dlq job status: expected dlq got ${JOB_STATUS}"
  exit 1
fi
if [[ "${JOB_ERROR_CODE}" != "template_inactive" ]]; then
  echo "FAIL dlq job error_code: expected template_inactive got ${JOB_ERROR_CODE}"
  exit 1
fi

echo "== query job summary =="
SUMMARY_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/summary?tenant_id=${TENANT_ID}&max_retry=3")"
split_response "${SUMMARY_RAW}"
require_http_code "200" "wallet jobs summary"
SUMMARY_DLQ="$(echo "${HTTP_BODY}" | jq -r '.dlq')"
if [[ "${SUMMARY_DLQ}" -lt 1 ]]; then
  echo "FAIL wallet jobs summary: expected dlq>=1 got ${SUMMARY_DLQ}"
  exit 1
fi

echo "== requeue dlq job =="
REQUEUE_PAYLOAD="$(jq -nc --arg tenant "${TENANT_ID}" --arg actor "qa.dlq.requeue" '{tenant_id:$tenant,actor:$actor}')"
REQUEUE_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/${JOB_ID}/dlq/requeue" "${REQUEUE_PAYLOAD}")"
split_response "${REQUEUE_RAW}"
require_http_code "200" "dlq requeue"
REQUEUE_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
if [[ "${REQUEUE_STATUS}" != "pending" ]]; then
  echo "FAIL dlq requeue status: expected pending got ${REQUEUE_STATUS}"
  exit 1
fi

echo "== set template back active =="
ACTIVE_PAYLOAD='{"status":"active","actor":"qa.dlq"}'
ACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" "${ACTIVE_PAYLOAD}")"
split_response "${ACTIVE_RAW}"
require_http_code "200" "set template active"

echo "== process jobs again (expect success) =="
PROCESS_OK_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_OK_RAW}"
require_http_code "200" "process jobs after requeue"
SUCCEEDED_COUNT="$(echo "${HTTP_BODY}" | jq -r '.succeeded')"
if [[ "${SUCCEEDED_COUNT}" -lt 1 ]]; then
  echo "FAIL process jobs after requeue: expected succeeded>=1 got ${SUCCEEDED_COUNT}"
  echo "${HTTP_BODY}"
  exit 1
fi

FINAL_JOB_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/${JOB_ID}?tenant_id=${TENANT_ID}")"
split_response "${FINAL_JOB_RAW}"
require_http_code "200" "get final job"
FINAL_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
FINAL_PASS_ID="$(echo "${HTTP_BODY}" | jq -r '.pass_id')"
if [[ "${FINAL_STATUS}" != "success" ]]; then
  echo "FAIL final job status: expected success got ${FINAL_STATUS}"
  exit 1
fi
require_non_empty "${FINAL_PASS_ID}" "final pass_id"

echo "PASS: wallet dlq requeue regression complete"
