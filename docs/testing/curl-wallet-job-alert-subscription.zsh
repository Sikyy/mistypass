#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18083}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
WALLET_DLQ_ALERT_THRESHOLD="${WALLET_DLQ_ALERT_THRESHOLD:-7}"
WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS="${WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS:-1200}"
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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_job_alert_subscription_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_job_alert_subscription_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_job_alert_subscription_api.log
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

echo "== check default alert subscription =="
DEFAULT_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/alert-subscription?tenant_id=${TENANT_ID}")"
split_response "${DEFAULT_RAW}"
require_http_code "200" "get default alert subscription"
DEFAULT_THRESHOLD="$(echo "${HTTP_BODY}" | jq -r '.dlq_alert_threshold')"
DEFAULT_WINDOW_SECONDS="$(echo "${HTTP_BODY}" | jq -r '.window_seconds')"
DEFAULT_ENABLED="$(echo "${HTTP_BODY}" | jq -r '.enabled')"
DEFAULT_EMAIL="$(echo "${HTTP_BODY}" | jq -r '.channels.email')"
if [[ "${DEFAULT_THRESHOLD}" -ne "${WALLET_DLQ_ALERT_THRESHOLD}" ]]; then
  echo "FAIL default alert subscription: expected threshold=${WALLET_DLQ_ALERT_THRESHOLD} got ${DEFAULT_THRESHOLD}"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${DEFAULT_WINDOW_SECONDS}" -ne "${WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS}" ]]; then
  echo "FAIL default alert subscription: expected window_seconds=${WALLET_JOB_METRICS_DEFAULT_WINDOW_SECONDS} got ${DEFAULT_WINDOW_SECONDS}"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${DEFAULT_ENABLED}" != "true" || "${DEFAULT_EMAIL}" != "true" ]]; then
  echo "FAIL default alert subscription: expected enabled/email=true"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== upsert alert subscription =="
UPSERT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg actor "qa.alert.subscription.${RUN_TAG}" \
  '{tenant_id:$tenant,enabled:true,dlq_alert_threshold:2,window_seconds:300,cooldown_seconds:120,channels:{email:true,whatsapp:true},receiver_groups:["security","ops"],actor:$actor}')"
UPSERT_RAW="$(api_with_auth PUT "/api/v1/wallet/jobs/alert-subscription" "${UPSERT_PAYLOAD}")"
split_response "${UPSERT_RAW}"
require_http_code "200" "upsert alert subscription"
UPSERT_THRESHOLD="$(echo "${HTTP_BODY}" | jq -r '.dlq_alert_threshold')"
UPSERT_WINDOW="$(echo "${HTTP_BODY}" | jq -r '.window_seconds')"
UPSERT_COOLDOWN="$(echo "${HTTP_BODY}" | jq -r '.cooldown_seconds')"
UPSERT_WHATSAPP="$(echo "${HTTP_BODY}" | jq -r '.channels.whatsapp')"
UPSERT_GROUPS="$(echo "${HTTP_BODY}" | jq -r '.receiver_groups | length')"
if [[ "${UPSERT_THRESHOLD}" -ne 2 || "${UPSERT_WINDOW}" -ne 300 || "${UPSERT_COOLDOWN}" -ne 120 ]]; then
  echo "FAIL upsert alert subscription: unexpected threshold/window/cooldown"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${UPSERT_WHATSAPP}" != "true" || "${UPSERT_GROUPS}" -lt 2 ]]; then
  echo "FAIL upsert alert subscription: expected whatsapp=true and groups>=2"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== verify persisted alert subscription =="
CHECK_RAW="$(api_with_auth GET "/api/v1/wallet/jobs/alert-subscription?tenant_id=${TENANT_ID}")"
split_response "${CHECK_RAW}"
require_http_code "200" "get persisted alert subscription"
CHECK_THRESHOLD="$(echo "${HTTP_BODY}" | jq -r '.dlq_alert_threshold')"
CHECK_WINDOW="$(echo "${HTTP_BODY}" | jq -r '.window_seconds')"
CHECK_GROUP_OPS="$(echo "${HTTP_BODY}" | jq -r '.receiver_groups | map(select(.=="ops")) | length')"
if [[ "${CHECK_THRESHOLD}" -ne 2 || "${CHECK_WINDOW}" -ne 300 || "${CHECK_GROUP_OPS}" -lt 1 ]]; then
  echo "FAIL get persisted alert subscription: expected threshold=2 window=300 and contains ops group"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== validate invalid channel strategy =="
INVALID_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,enabled:true,dlq_alert_threshold:2,window_seconds:300,cooldown_seconds:120,channels:{email:false,whatsapp:false},receiver_groups:["security"],actor:"qa.alert.subscription.invalid"}')"
INVALID_RAW="$(api_with_auth PUT "/api/v1/wallet/jobs/alert-subscription" "${INVALID_PAYLOAD}")"
split_response "${INVALID_RAW}"
require_http_code "400" "invalid alert subscription channels"

echo "PASS: wallet job alert subscription regression complete"
