#!/usr/bin/env zsh
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-https://staging-api.mistyislet.com}"
LOGIN_EMAIL="${LOGIN_EMAIL:-tenant.admin@sudirman.co}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
PUSH_TITLE="${PUSH_TITLE:-MistyPass staging push}"
PUSH_BODY="${PUSH_BODY:-FCM smoke from Mac mini}"

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
    if [[ "${HTTP_CODE}" == "404" ]]; then
      echo "Hint: deploy the API version that includes /api/v1/mobile-push routes before running FCM smoke."
    fi
    exit 1
  fi
}

function require_http_code_any() {
  local step="$1"
  shift
  local expected
  for expected in "$@"; do
    if [[ "${HTTP_CODE}" == "${expected}" ]]; then
      return
    fi
  done
  echo "FAIL ${step}: expected HTTP ${(j:/:)@}, got ${HTTP_CODE}"
  echo "${HTTP_BODY}"
  exit 1
}

function require_non_empty() {
  local value="$1"
  local step="$2"
  if [[ -z "${value}" || "${value}" == "null" ]]; then
    echo "FAIL ${step}: empty value"
    exit 1
  fi
}

echo "== login =="
LOGIN_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token // empty')"
require_non_empty "${AT}" "login.access_token"

echo "== mobile push provider status =="
STATUS_RAW="$(curl -sS "${API_BASE_URL}/api/v1/mobile-push/provider-status?tenant_id=${TENANT_ID}" \
  -H "Authorization: Bearer ${AT}" \
  -w $'\n%{http_code}')"
split_response "${STATUS_RAW}"
require_http_code "200" "provider status"

CONFIGURED="$(echo "${HTTP_BODY}" | jq -r '.configured // false')"
if [[ "${CONFIGURED}" != "true" ]]; then
  echo "FAIL provider status: FCM provider is not configured"
  echo "${HTTP_BODY}" | jq '{enabled, provider, configured, missing}'
  exit 1
fi
echo "${HTTP_BODY}" | jq '{enabled, provider, configured, missing, target_device_id, target_device_model}'

echo "== send FCM smoke =="
SMOKE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg title "${PUSH_TITLE}" \
  --arg body "${PUSH_BODY}" \
  '{tenant_id:$tenant,title:$title,body:$body}')"
SMOKE_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/mobile-push/smoke" \
  -H "Authorization: Bearer ${AT}" \
  -H "Content-Type: application/json" \
  -d "${SMOKE_PAYLOAD}" \
  -w $'\n%{http_code}')"
split_response "${SMOKE_RAW}"
require_http_code_any "send smoke" "200" "202"
echo "${HTTP_BODY}" | jq '{provider, provider_message_id, target_device_id, target_device_model, status}'

echo "PASS: mobile push smoke accepted by provider"
