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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_dlq_governance_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_dlq_governance_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_dlq_governance_api.log
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
  --arg name "DLQ Governance ${RUN_TAG}" \
  '{tenant_id:$tenant,pass_type:"employee",name:$name,status:"active",actor:"qa.dlq.governance"}')"
TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${TEMPLATE_PAYLOAD}")"
split_response "${TEMPLATE_RAW}"
require_http_code "201" "create template"
TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TEMPLATE_ID}" "template.id"

echo "== build old queued set =="
OLD_BATCH_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg t1 "usr-dlq-old-1-${RUN_TAG}" \
  --arg t2 "usr-dlq-old-2-${RUN_TAG}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:[$t1,$t2],execution_mode:"queued",actor:"qa.dlq.governance"}')"
OLD_BATCH_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${OLD_BATCH_PAYLOAD}")"
split_response "${OLD_BATCH_RAW}"
require_http_code "202" "issue old dlq batch"

echo "== set template inactive then process old queued jobs (expect dlq) =="
INACTIVE_PAYLOAD='{"status":"inactive","actor":"qa.dlq.governance"}'
INACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" "${INACTIVE_PAYLOAD}")"
split_response "${INACTIVE_RAW}"
require_http_code "200" "set template inactive for old batch"

PROCESS_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:20,worker_count:2,max_retry:3,base_backoff_ms:10,max_backoff_ms:50,actor:"qa.dlq.governance.worker"}')"
PROCESS_OLD_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_OLD_RAW}"
require_http_code "200" "process old dlq batch"
OLD_DLQ="$(echo "${HTTP_BODY}" | jq -r '.dlq')"
if [[ "${OLD_DLQ}" -lt 2 ]]; then
  echo "FAIL process old dlq batch: expected dlq>=2 got ${OLD_DLQ}"
  echo "${HTTP_BODY}"
  exit 1
fi

sleep 3

echo "== build recent dlq set =="
ACTIVE_PAYLOAD='{"status":"active","actor":"qa.dlq.governance"}'
ACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" "${ACTIVE_PAYLOAD}")"
split_response "${ACTIVE_RAW}"
require_http_code "200" "set template active for new batch queue"

NEW_BATCH_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg t1 "usr-dlq-new-1-${RUN_TAG}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:[$t1],execution_mode:"queued",actor:"qa.dlq.governance"}')"
NEW_BATCH_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${NEW_BATCH_PAYLOAD}")"
split_response "${NEW_BATCH_RAW}"
require_http_code "202" "issue new dlq batch"

INACTIVE_NEW_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" "${INACTIVE_PAYLOAD}")"
split_response "${INACTIVE_NEW_RAW}"
require_http_code "200" "set template inactive for new batch"

PROCESS_NEW_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_NEW_RAW}"
require_http_code "200" "process new dlq batch"

echo "== batch requeue one dlq job =="
REQUEUE_BATCH_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:1,error_code:"template_inactive",actor:"qa.dlq.governance.requeue"}')"
REQUEUE_BATCH_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/dlq/requeue" "${REQUEUE_BATCH_PAYLOAD}")"
split_response "${REQUEUE_BATCH_RAW}"
require_http_code "200" "batch dlq requeue"
REQUEUED_COUNT="$(echo "${HTTP_BODY}" | jq -r '.requeued')"
if [[ "${REQUEUED_COUNT}" -lt 1 ]]; then
  echo "FAIL batch dlq requeue: expected requeued>=1 got ${REQUEUED_COUNT}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== set template active and process pending requeue =="
ACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" "${ACTIVE_PAYLOAD}")"
split_response "${ACTIVE_RAW}"
require_http_code "200" "set template active"
PROCESS_ACTIVE_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_ACTIVE_RAW}"
require_http_code "200" "process requeued pending"
SUCCEEDED_COUNT="$(echo "${HTTP_BODY}" | jq -r '.succeeded')"
if [[ "${SUCCEEDED_COUNT}" -lt 1 ]]; then
  echo "FAIL process requeued pending: expected succeeded>=1 got ${SUCCEEDED_COUNT}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== cleanup old dlq jobs =="
CLEANUP_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:20,error_code:"template_inactive",older_than_seconds:2,actor:"qa.dlq.governance.cleanup"}')"
CLEANUP_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/dlq/cleanup" "${CLEANUP_PAYLOAD}")"
split_response "${CLEANUP_RAW}"
require_http_code "200" "cleanup dlq jobs"
REMOVED_COUNT="$(echo "${HTTP_BODY}" | jq -r '.removed')"
if [[ "${REMOVED_COUNT}" -lt 1 ]]; then
  echo "FAIL cleanup dlq jobs: expected removed>=1 got ${REMOVED_COUNT}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== verify cleanup archive records =="
ARCHIVE_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/dlq/cleanup/archives?tenant_id=${TENANT_ID}&limit=5")"
split_response "${ARCHIVE_RAW}"
require_http_code "200" "list dlq cleanup archives"
ARCHIVE_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
ARCHIVE_LATEST_REMOVED="$(echo "${HTTP_BODY}" | jq -r '.items[0].removed // 0')"
ARCHIVE_LATEST_ACTOR="$(echo "${HTTP_BODY}" | jq -r '.items[0].actor // ""')"
if [[ "${ARCHIVE_COUNT}" -lt 1 ]]; then
  echo "FAIL list dlq cleanup archives: expected items>=1 got ${ARCHIVE_COUNT}"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${ARCHIVE_LATEST_REMOVED}" -lt 1 ]]; then
  echo "FAIL list dlq cleanup archives: expected latest removed>=1 got ${ARCHIVE_LATEST_REMOVED}"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${ARCHIVE_LATEST_ACTOR}" != "qa.dlq.governance.cleanup" ]]; then
  echo "FAIL list dlq cleanup archives: expected latest actor=qa.dlq.governance.cleanup got ${ARCHIVE_LATEST_ACTOR}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== verify summary endpoint still available =="
SUMMARY_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/summary?tenant_id=${TENANT_ID}&max_retry=3")"
split_response "${SUMMARY_RAW}"
require_http_code "200" "wallet jobs summary after cleanup"

echo "PASS: wallet dlq governance regression complete"
