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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_retry_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_retry_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_retry_api.log
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
echo "login: ok"

echo "== create wallet template =="
TEMPLATE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg name "Retry Template ${RUN_TAG}" \
  '{tenant_id:$tenant,pass_type:"employee",name:$name,status:"active",actor:"qa.retry"}')"
TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${TEMPLATE_PAYLOAD}")"
split_response "${TEMPLATE_RAW}"
require_http_code "201" "create wallet template"
TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TEMPLATE_ID}" "template.id"
echo "template: id=${TEMPLATE_ID}"

echo "== issue batch with one invalid target (to produce failed job) =="
VALID_TARGET="usr-retry-${RUN_TAG}"
BATCH_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg valid_target "${VALID_TARGET}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:["",$valid_target],actor:"qa.retry"}')"
BATCH_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${BATCH_PAYLOAD}")"
split_response "${BATCH_RAW}"
require_http_code "202" "issue batch"
FAILED_JOB_ID="$(echo "${HTTP_BODY}" | jq -r '.items[] | select(.status=="failed" and .error_code=="target_id_required") | .id' | head -n1)"
require_non_empty "${FAILED_JOB_ID}" "failed_job.id"
echo "batch: failed_job_id=${FAILED_JOB_ID}"

echo "== retry failed job with override target_id =="
RETRY_TARGET="usr-retry-fixed-${RUN_TAG}"
RETRY_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg target "${RETRY_TARGET}" \
  '{tenant_id:$tenant,target_id:$target,actor:"qa.retry"}')"
RETRY_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/${FAILED_JOB_ID}/retry" "${RETRY_PAYLOAD}")"
split_response "${RETRY_RAW}"
require_http_code "200" "retry wallet job"
RETRY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
RETRY_COUNT="$(echo "${HTTP_BODY}" | jq -r '.retry_count')"
PASS_ID="$(echo "${HTTP_BODY}" | jq -r '.pass_id')"
UPDATED_TARGET="$(echo "${HTTP_BODY}" | jq -r '.target_id')"
if [[ "${RETRY_STATUS}" != "success" ]]; then
  echo "FAIL retry wallet job: expected success got ${RETRY_STATUS}"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${RETRY_COUNT}" -lt 1 ]]; then
  echo "FAIL retry wallet job: expected retry_count>=1 got ${RETRY_COUNT}"
  exit 1
fi
require_non_empty "${PASS_ID}" "retry.pass_id"
if [[ "${UPDATED_TARGET}" != "${RETRY_TARGET}" ]]; then
  echo "FAIL retry wallet job: expected target_id=${RETRY_TARGET} got ${UPDATED_TARGET}"
  exit 1
fi
echo "retry: status=${RETRY_STATUS} retry_count=${RETRY_COUNT} pass_id=${PASS_ID}"

echo "== verify job detail + pass list =="
JOB_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/${FAILED_JOB_ID}?tenant_id=${TENANT_ID}")"
split_response "${JOB_RAW}"
require_http_code "200" "get retried job"
JOB_PASS_ID="$(echo "${HTTP_BODY}" | jq -r '.pass_id')"
if [[ "${JOB_PASS_ID}" != "${PASS_ID}" ]]; then
  echo "FAIL get retried job: pass_id mismatch expected ${PASS_ID} got ${JOB_PASS_ID}"
  exit 1
fi

PASSES_RAW="$(api_with_auth GET "/api/v1/wallet/passes?tenant_id=${TENANT_ID}")"
split_response "${PASSES_RAW}"
require_http_code "200" "list passes after retry"
PASS_EXISTS="$(echo "${HTTP_BODY}" | jq -r --arg id "${PASS_ID}" '.items | map(select(.id == $id)) | length')"
if [[ "${PASS_EXISTS}" -lt 1 ]]; then
  echo "FAIL pass list: retried pass ${PASS_ID} not found"
  exit 1
fi

echo "PASS: wallet retry curl regression complete"
