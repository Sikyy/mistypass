#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18190}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
WEBHOOK_SECRET="${WEBHOOK_SECRET:-$(printf '%s-%s-%s' email inbound secret)}"
API_LOG="${API_LOG:-/tmp/mp_email_inbound_webhook_api.log}"
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

function require_equals() {
  local got="$1"
  local expected="$2"
  local step="$3"
  if [[ "${got}" != "${expected}" ]]; then
    echo "FAIL ${step}: expected ${expected}, got ${got}"
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

function sign_payload() {
  local timestamp="$1"
  local payload="$2"
  WEBHOOK_SECRET="${WEBHOOK_SECRET}" WEBHOOK_TIMESTAMP="${timestamp}" WEBHOOK_PAYLOAD="${payload}" node -e '
const crypto = require("crypto")
const secret = String(process.env.WEBHOOK_SECRET || "")
const timestamp = String(process.env.WEBHOOK_TIMESTAMP || "")
const payload = String(process.env.WEBHOOK_PAYLOAD || "")
const digest = crypto.createHmac("sha256", secret).update(`${timestamp}.${payload}`).digest("hex")
process.stdout.write(`sha256=${digest}`)
'
}

function ensure_api_running() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "api: already running"
    return
  fi

  echo "api: starting local email inbound webhook server"
  (
    cd api
    PORT="${API_PORT}" \
      ENABLE_DEMO_USERS=true \
      DISABLE_LOGIN_RATE_LIMIT=true \
      EMAIL_INBOUND_WEBHOOK_SECRET="${WEBHOOK_SECRET}" \
      GOCACHE=/tmp/go-build \
      go run ./cmd/api >"${API_LOG}" 2>&1
  ) &
  API_PID="$!"

  local i
  for i in {1..60}; do
    if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "api: started on ${API_BASE_URL}"
      return
    fi
    sleep 0.25
  done

  echo "FAIL api startup: healthz not ready"
  if [[ -f "${API_LOG}" ]]; then
    tail -n 120 "${API_LOG}"
  fi
  exit 1
}

function cleanup() {
  if [[ -n "${API_PID}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
    if command -v lsof >/dev/null 2>&1; then
      local pids
      pids="$(lsof -ti tcp:"${API_PORT}" 2>/dev/null || true)"
      for pid in ${(f)pids}; do
        [[ -n "${pid}" ]] && kill "${pid}" >/dev/null 2>&1 || true
      done
    fi
  fi
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
MESSAGE_ID="msg-report-reply-${RUN_TAG}"
PROVIDER_DELIVERY_ID="email_report_mock_${RUN_TAG}"
REPORT_SCHEDULE_ID="rs_${RUN_TAG}"
ensure_api_running

echo "== post unsigned webhook should fail =="
UNSIGNED_PAYLOAD="$(jq -nc --arg tenant "${TENANT_ID}" '{tenant_id:$tenant}')"
UNSIGNED_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/webhooks/email/inbound" \
  -H "Content-Type: application/json" \
  -d "${UNSIGNED_PAYLOAD}" \
  -w $'\n%{http_code}')"
split_response "${UNSIGNED_RAW}"
require_http_code "401" "unsigned webhook"

echo "== post signed email inbound webhook =="
PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg message_id "${MESSAGE_ID}" \
  --arg provider_delivery_id "${PROVIDER_DELIVERY_ID}" \
  --arg schedule_id "${REPORT_SCHEDULE_ID}" \
  '{
    tenant_id:$tenant,
    provider:"cloudflare_email_worker",
    event_type:"reply",
    message_id:$message_id,
    provider_delivery_id:$provider_delivery_id,
    from:"manager@example.com",
    to:["reports@mistyislet.com"],
    subject:"Re: Daily operations report",
    metadata:{report_schedule_id:$schedule_id},
    attachments:[{filename:"reply.eml",content_type:"message/rfc822",size_bytes:512}]
  }')"
TIMESTAMP="$(date +%s)"
SIGNATURE="$(sign_payload "${TIMESTAMP}" "${PAYLOAD}")"
WEBHOOK_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/webhooks/email/inbound" \
  -H "Content-Type: application/json" \
  -H "X-MistyPass-Email-Timestamp: ${TIMESTAMP}" \
  -H "X-MistyPass-Email-Signature: ${SIGNATURE}" \
  -d "${PAYLOAD}" \
  -w $'\n%{http_code}')"
split_response "${WEBHOOK_RAW}"
require_http_code "202" "signed webhook"
EVENT_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
EVENT_MESSAGE_ID="$(echo "${HTTP_BODY}" | jq -r '.message_id')"
EVENT_PROVIDER="$(echo "${HTTP_BODY}" | jq -r '.provider')"
EVENT_RELATED="$(echo "${HTTP_BODY}" | jq -r '.metadata.report_schedule_id')"
EVENT_ATTACHMENT_COUNT="$(echo "${HTTP_BODY}" | jq -r '.attachments | length')"
require_non_empty "${EVENT_ID}" "webhook event.id"
require_equals "${EVENT_MESSAGE_ID}" "${MESSAGE_ID}" "webhook message_id"
require_equals "${EVENT_PROVIDER}" "cloudflare_email_worker" "webhook provider"
require_equals "${EVENT_RELATED}" "${REPORT_SCHEDULE_ID}" "webhook metadata.report_schedule_id"
require_equals "${EVENT_ATTACHMENT_COUNT}" "1" "webhook attachment count"

echo "== login =="
LOGIN_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

echo "== list email inbound events =="
LIST_RAW="$(api_with_auth GET "/api/v1/webhooks/email/inbound/events?tenant_id=${TENANT_ID}&event_type=reply&provider=cloudflare_email_worker&limit=5")"
split_response "${LIST_RAW}"
require_http_code "200" "list email inbound events"
jq -e \
  --arg event_id "${EVENT_ID}" \
  --arg message_id "${MESSAGE_ID}" \
  --arg provider_delivery_id "${PROVIDER_DELIVERY_ID}" \
  --arg schedule_id "${REPORT_SCHEDULE_ID}" \
  '
  (.items // [])
  | any(
      .id == $event_id
      and .message_id == $message_id
      and .provider_delivery_id == $provider_delivery_id
      and .metadata.report_schedule_id == $schedule_id
      and ((.attachments // []) | length == 1)
    )
  ' <<<"${HTTP_BODY}" >/dev/null

echo "== verify email inbound audit log =="
AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=email_inbound_event_received&source=email_inbound_webhook&limit=10")"
split_response "${AUDIT_RAW}"
require_http_code "200" "email inbound audit log"
jq -e \
  --arg event_id "${EVENT_ID}" \
  --arg schedule_id "${REPORT_SCHEDULE_ID}" \
  '
  (.items // [])
  | any(
      .action == "email_inbound_event_received"
      and .source == "email_inbound_webhook"
      and (.target // "" | contains("event_id=" + $event_id))
      and (.target // "" | contains("related=report_schedule_id:" + $schedule_id))
    )
  ' <<<"${HTTP_BODY}" >/dev/null

echo "PASS: email inbound webhook regression complete"
