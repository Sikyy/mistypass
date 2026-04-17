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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_queue_process_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_queue_process_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_queue_process_api.log
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
  --arg name "Queue Template ${RUN_TAG}" \
  '{tenant_id:$tenant,pass_type:"employee",name:$name,status:"active",actor:"qa.queue"}')"
TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${TEMPLATE_PAYLOAD}")"
split_response "${TEMPLATE_RAW}"
require_http_code "201" "create wallet template"
TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TEMPLATE_ID}" "template.id"

echo "== issue batch (queued mode) with one invalid target =="
TARGET_A="usr-queue-a-${RUN_TAG}"
TARGET_B="usr-queue-b-${RUN_TAG}"
QUEUE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg target_a "${TARGET_A}" \
  --arg target_b "${TARGET_B}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:["",$target_a,$target_b],execution_mode:"queued",actor:"qa.queue"}')"
QUEUE_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${QUEUE_PAYLOAD}")"
split_response "${QUEUE_RAW}"
require_http_code "202" "issue batch queued"
MODE="$(echo "${HTTP_BODY}" | jq -r '.execution_mode')"
if [[ "${MODE}" != "queued" ]]; then
  echo "FAIL issue batch queued: execution_mode expected queued got ${MODE}"
  exit 1
fi

PENDING_IDS=("${(@f)$(echo "${HTTP_BODY}" | jq -r '.items[] | select(.status=="pending") | .id')}")
FAILED_TARGET_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | map(select(.status=="failed" and .error_code=="target_id_required")) | length')"
if [[ "${#PENDING_IDS[@]}" -ne 2 ]]; then
  echo "FAIL issue batch queued: expected 2 pending jobs got ${#PENDING_IDS[@]}"
  exit 1
fi
if [[ "${FAILED_TARGET_COUNT}" -ne 1 ]]; then
  echo "FAIL issue batch queued: expected 1 failed target job got ${FAILED_TARGET_COUNT}"
  exit 1
fi

echo "== process queued jobs =="
PROCESS_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:10,worker_count:2,max_retry:3,base_backoff_ms:10,max_backoff_ms:100,actor:"qa.queue.worker"}')"
PROCESS_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_RAW}"
require_http_code "200" "process wallet jobs"
CLAIMED="$(echo "${HTTP_BODY}" | jq -r '.claimed')"
SUCCEEDED="$(echo "${HTTP_BODY}" | jq -r '.succeeded')"
FAILED="$(echo "${HTTP_BODY}" | jq -r '.failed')"
if [[ "${CLAIMED}" -ne 2 || "${SUCCEEDED}" -ne 2 || "${FAILED}" -ne 0 ]]; then
  echo "FAIL process wallet jobs: claimed=${CLAIMED} succeeded=${SUCCEEDED} failed=${FAILED}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== verify queued jobs become success =="
for job_id in "${PENDING_IDS[@]}"; do
  JOB_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/${job_id}?tenant_id=${TENANT_ID}")"
  split_response "${JOB_RAW}"
  require_http_code "200" "get queued job ${job_id}"
  STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
  PASS_ID="$(echo "${HTTP_BODY}" | jq -r '.pass_id')"
  if [[ "${STATUS}" != "success" ]]; then
    echo "FAIL queued job ${job_id}: expected success got ${STATUS}"
    exit 1
  fi
  require_non_empty "${PASS_ID}" "queued job ${job_id} pass_id"
done

echo "PASS: wallet queued job process regression complete"
