#!/usr/bin/env zsh
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
AUDIT_LOG_ID="${AUDIT_LOG_ID:-aud_3002}"

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

echo "== login =="
LOGIN_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

echo "== upsert webhook config (disabled) =="
DISABLED_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,enabled:false,endpoint:"http://127.0.0.1:9/hooks/audit",actions:["gateway_reboot"],updated_by:"qa.audit.webhook"}')"
UPSERT_DISABLED_RAW="$(api_with_auth PUT "/api/v1/audit/webhook/config" "${DISABLED_PAYLOAD}")"
split_response "${UPSERT_DISABLED_RAW}"
require_http_code "200" "upsert webhook config disabled"
ENABLED_FLAG="$(echo "${HTTP_BODY}" | jq -r '.enabled')"
if [[ "${ENABLED_FLAG}" != "false" ]]; then
  echo "FAIL upsert disabled: expected enabled=false got ${ENABLED_FLAG}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== get webhook config =="
GET_CONFIG_RAW="$(api_with_auth GET "/api/v1/audit/webhook/config?tenant_id=${TENANT_ID}")"
split_response "${GET_CONFIG_RAW}"
require_http_code "200" "get webhook config"
GET_ENDPOINT="$(echo "${HTTP_BODY}" | jq -r '.endpoint')"
if [[ "${GET_ENDPOINT}" != "http://127.0.0.1:9/hooks/audit" ]]; then
  echo "FAIL get webhook config: unexpected endpoint ${GET_ENDPOINT}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== dispatch while disabled (expect conflict) =="
DISPATCH_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg logid "${AUDIT_LOG_ID}" \
  '{tenant_id:$tenant,audit_log_id:$logid}')"
DISPATCH_DISABLED_RAW="$(api_with_auth POST "/api/v1/audit/webhook/dispatch" "${DISPATCH_PAYLOAD}")"
split_response "${DISPATCH_DISABLED_RAW}"
require_http_code "409" "dispatch disabled"
DISPATCH_DISABLED_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
if [[ "${DISPATCH_DISABLED_ERROR}" != "audit webhook is disabled" ]]; then
  echo "FAIL dispatch disabled: unexpected error ${DISPATCH_DISABLED_ERROR}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== upsert webhook config (enabled) =="
ENABLED_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,enabled:true,endpoint:"http://127.0.0.1:9/hooks/audit",actions:["gateway_reboot"],updated_by:"qa.audit.webhook"}')"
UPSERT_ENABLED_RAW="$(api_with_auth PUT "/api/v1/audit/webhook/config" "${ENABLED_PAYLOAD}")"
split_response "${UPSERT_ENABLED_RAW}"
require_http_code "200" "upsert webhook config enabled"

echo "== dispatch with unreachable endpoint (expect bad gateway + failed delivery) =="
DISPATCH_FAIL_RAW="$(api_with_auth POST "/api/v1/audit/webhook/dispatch" "${DISPATCH_PAYLOAD}")"
split_response "${DISPATCH_FAIL_RAW}"
require_http_code "502" "dispatch unreachable endpoint"
DELIVERY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.delivery.status')"
if [[ "${DELIVERY_STATUS}" != "failed" ]]; then
  echo "FAIL dispatch unreachable: expected failed delivery status got ${DELIVERY_STATUS}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== list webhook deliveries =="
DELIVERIES_RAW="$(api_with_auth GET "/api/v1/audit/webhook/deliveries?tenant_id=${TENANT_ID}&limit=5")"
split_response "${DELIVERIES_RAW}"
require_http_code "200" "list webhook deliveries"
DELIVERY_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | length')"
FAILED_COUNT="$(echo "${HTTP_BODY}" | jq -r '.items | map(select(.status=="failed")) | length')"
if [[ "${DELIVERY_COUNT}" -lt 1 || "${FAILED_COUNT}" -lt 1 ]]; then
  echo "FAIL list webhook deliveries: expected failed delivery records"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== action filter guard (expect conflict) =="
FILTER_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,enabled:true,endpoint:"http://127.0.0.1:9/hooks/audit",actions:["tenant_update"],updated_by:"qa.audit.webhook"}')"
UPSERT_FILTER_RAW="$(api_with_auth PUT "/api/v1/audit/webhook/config" "${FILTER_PAYLOAD}")"
split_response "${UPSERT_FILTER_RAW}"
require_http_code "200" "upsert webhook config filtered action"

DISPATCH_FILTER_RAW="$(api_with_auth POST "/api/v1/audit/webhook/dispatch" "${DISPATCH_PAYLOAD}")"
split_response "${DISPATCH_FILTER_RAW}"
require_http_code "409" "dispatch action filtered"
FILTER_ERROR="$(echo "${HTTP_BODY}" | jq -r '.error')"
if [[ "${FILTER_ERROR}" != "audit webhook action is filtered" ]]; then
  echo "FAIL dispatch action filtered: unexpected error ${FILTER_ERROR}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "PASS: audit webhook fan-out regression complete"
