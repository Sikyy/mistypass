#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18086}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT="${WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT:-1}"
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
    WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT="${WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT}" \
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_job_alert_dispatch_retry_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_job_alert_dispatch_retry_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_job_alert_dispatch_retry_api.log
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
  --arg name "Alert Dispatch Retry ${RUN_TAG}" \
  '{tenant_id:$tenant,pass_type:"employee",name:$name,status:"active",actor:"qa.alert.dispatch.retry"}')"
TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${TEMPLATE_PAYLOAD}")"
split_response "${TEMPLATE_RAW}"
require_http_code "201" "create template"
TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TEMPLATE_ID}" "template.id"

echo "== queue jobs =="
QUEUE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg t1 "usr-alert-dispatch-retry-1-${RUN_TAG}" \
  --arg t2 "usr-alert-dispatch-retry-2-${RUN_TAG}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:[$t1,$t2],execution_mode:"queued",actor:"qa.alert.dispatch.retry"}')"
QUEUE_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${QUEUE_PAYLOAD}")"
split_response "${QUEUE_RAW}"
require_http_code "202" "issue queued batch"

echo "== set template inactive + process to dlq =="
INACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" '{"status":"inactive","actor":"qa.alert.dispatch.retry"}')"
split_response "${INACTIVE_RAW}"
require_http_code "200" "set template inactive"

PROCESS_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:20,worker_count:2,max_retry:3,base_backoff_ms:10,max_backoff_ms:30,actor:"qa.alert.dispatch.retry.worker"}')"
PROCESS_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_RAW}"
require_http_code "200" "process queued jobs"
DLQ_COUNT="$(echo "${HTTP_BODY}" | jq -r '.dlq')"
if [[ "${DLQ_COUNT}" -lt 2 ]]; then
  echo "FAIL process queued jobs: expected dlq>=2 got ${DLQ_COUNT}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== upsert alert subscription =="
SUB_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,enabled:true,dlq_alert_threshold:1,window_seconds:600,cooldown_seconds:120,channels:{email:true,whatsapp:false},receiver_groups:["security"],actor:"qa.alert.dispatch.retry"}')"
SUB_RAW="$(api_with_auth PUT "/api/v1/wallet/jobs/alert-subscription" "${SUB_PAYLOAD}")"
split_response "${SUB_RAW}"
require_http_code "200" "upsert alert subscription"

echo "== dispatch alerts first pass (expect provider transient failure) =="
DISPATCH_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/alerts/dispatch" "{\"tenant_id\":\"${TENANT_ID}\",\"actor\":\"qa.alert.dispatch.retry\"}")"
split_response "${DISPATCH_RAW}"
require_http_code "200" "dispatch alerts first"
FAILED_1="$(echo "${HTTP_BODY}" | jq -r '.failed')"
FAILED_ITEM_ID="$(echo "${HTTP_BODY}" | jq -r '.items | map(select(.status=="failed" and .retryable==true and .reason=="provider_transient_error")) | .[0].id')"
FAILED_CHANNEL_RESULT="$(echo "${HTTP_BODY}" | jq -r '.items | map(select(.status=="failed")) | .[0].channel_results | length')"
if [[ "${FAILED_1}" -lt 1 ]]; then
  echo "FAIL dispatch first: expected failed>=1"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${FAILED_CHANNEL_RESULT}" -lt 1 ]]; then
  echo "FAIL dispatch first: expected channel_results for failed item"
  echo "${HTTP_BODY}"
  exit 1
fi
require_non_empty "${FAILED_ITEM_ID}" "dispatch.failed_item_id"

echo "== retry failed notification (expect sent) =="
RETRY_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/alert-notifications/${FAILED_ITEM_ID}/retry" "{\"tenant_id\":\"${TENANT_ID}\",\"actor\":\"qa.alert.dispatch.retry\"}")"
split_response "${RETRY_RAW}"
require_http_code "200" "retry failed notification"
RETRY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
RETRY_ATTEMPT="$(echo "${HTTP_BODY}" | jq -r '.attempt')"
RETRY_SOURCE="$(echo "${HTTP_BODY}" | jq -r '.source_notification_id')"
RETRY_CHANNEL_RESULT="$(echo "${HTTP_BODY}" | jq -r '.channel_results | length')"
if [[ "${RETRY_STATUS}" != "sent" || "${RETRY_ATTEMPT}" -lt 2 || "${RETRY_SOURCE}" != "${FAILED_ITEM_ID}" || "${RETRY_CHANNEL_RESULT}" -lt 1 ]]; then
  echo "FAIL retry failed notification: expected status=sent attempt>=2 source=${FAILED_ITEM_ID} channel_results>=1"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== retry same failed notification again (expect idempotent skip) =="
RETRY_AGAIN_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/alert-notifications/${FAILED_ITEM_ID}/retry" "{\"tenant_id\":\"${TENANT_ID}\",\"actor\":\"qa.alert.dispatch.retry\"}")"
split_response "${RETRY_AGAIN_RAW}"
require_http_code "200" "retry same failed notification again"
RETRY_AGAIN_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
RETRY_AGAIN_REASON="$(echo "${HTTP_BODY}" | jq -r '.reason')"
if [[ "${RETRY_AGAIN_STATUS}" != "skipped" || "${RETRY_AGAIN_REASON}" != "idempotent_already_sent" ]]; then
  echo "FAIL retry same failed notification again: expected skipped/idempotent_already_sent"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== list alert notifications =="
LOGS_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/alert-notifications?tenant_id=${TENANT_ID}&limit=10")"
split_response "${LOGS_RAW}"
require_http_code "200" "list alert notifications"
HAS_FAILED="$(echo "${HTTP_BODY}" | jq -r '.items | map(select(.status=="failed" and .reason=="provider_transient_error")) | length')"
HAS_SENT_RETRY="$(echo "${HTTP_BODY}" | jq -r --arg source "${FAILED_ITEM_ID}" '.items | map(select(.status=="sent" and .source_notification_id==$source)) | length')"
HAS_IDEMPOTENT_SKIP="$(echo "${HTTP_BODY}" | jq -r '.items | map(select(.status=="skipped" and .reason=="idempotent_already_sent")) | length')"
if [[ "${HAS_FAILED}" -lt 1 || "${HAS_SENT_RETRY}" -lt 1 || "${HAS_IDEMPOTENT_SKIP}" -lt 1 ]]; then
  echo "FAIL list alert notifications: expected failed/sent-retry/idempotent-skip entries"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "PASS: wallet job alert dispatch retry regression complete"
