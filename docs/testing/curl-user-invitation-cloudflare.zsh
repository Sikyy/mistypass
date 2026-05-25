#!/usr/bin/env zsh
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-https://staging-api.mistyislet.com}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
INVITATION_SMOKE_RECIPIENT="${INVITATION_SMOKE_RECIPIENT:-}"
INVITATION_SMOKE_CLEANUP="${INVITATION_SMOKE_CLEANUP:-1}"
INVITATION_SMOKE_USE_PLUS_ALIAS="${INVITATION_SMOKE_USE_PLUS_ALIAS:-1}"

AT=""
USER_ID=""
HTTP_CODE=""
HTTP_BODY_FILE=""
RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"

function make_body_file() {
  mktemp "${TMPDIR:-/tmp}/mistypass-invite-cloudflare.XXXXXX"
}

function split_recipient_alias() {
  local recipient="$1"
  if [[ "${INVITATION_SMOKE_USE_PLUS_ALIAS}" != "1" ]]; then
    printf "%s" "${recipient}"
    return
  fi
  local local_part="${recipient%@*}"
  local domain="${recipient#*@}"
  if [[ "${domain}" == "${recipient}" || -z "${local_part}" || -z "${domain}" ]]; then
    printf "%s" "${recipient}"
    return
  fi
  printf "%s+mistypass-smoke-%s@%s" "${local_part}" "${RUN_TAG}" "${domain}"
}

function api_with_auth() {
  local method="$1"
  local endpoint_path="$2"
  local payload="${3:-}"
  HTTP_BODY_FILE="$(make_body_file)"
  if [[ -n "${payload}" ]]; then
    HTTP_CODE="$(curl -sS -o "${HTTP_BODY_FILE}" -w "%{http_code}" \
      -X "${method}" "${API_BASE_URL}${endpoint_path}" \
      -H "Authorization: Bearer ${AT}" \
      -H "Content-Type: application/json" \
      -d "${payload}")"
    return
  fi
  HTTP_CODE="$(curl -sS -o "${HTTP_BODY_FILE}" -w "%{http_code}" \
    -X "${method}" "${API_BASE_URL}${endpoint_path}" \
    -H "Authorization: Bearer ${AT}")"
}

function require_http_code() {
  local expected="$1"
  local step="$2"
  if [[ "${HTTP_CODE}" != "${expected}" ]]; then
    echo "FAIL ${step}: expected HTTP ${expected}, got ${HTTP_CODE}"
    cat "${HTTP_BODY_FILE}"
    echo
    exit 1
  fi
}

function cleanup() {
  if [[ "${INVITATION_SMOKE_CLEANUP}" != "1" || -z "${USER_ID}" || -z "${AT}" ]]; then
    return
  fi
  api_with_auth DELETE "/api/v1/users/${USER_ID}?tenant_id=${TENANT_ID}" >/dev/null 2>&1 || true
}

trap cleanup EXIT

if [[ -z "${INVITATION_SMOKE_RECIPIENT}" ]]; then
  echo "FAIL user invitation cloudflare smoke: set INVITATION_SMOKE_RECIPIENT to a real inbox"
  echo "example: INVITATION_SMOKE_RECIPIENT=ops@example.com ./docs/testing/curl-user-invitation-cloudflare.zsh"
  exit 2
fi

SMOKE_EMAIL="$(split_recipient_alias "${INVITATION_SMOKE_RECIPIENT}")"
if [[ "${SMOKE_EMAIL}" == *"@example."* || "${SMOKE_EMAIL}" == *"@mistypass.local" ]]; then
  echo "FAIL user invitation cloudflare smoke: recipient must be a real deliverable inbox"
  echo "${SMOKE_EMAIL}"
  exit 2
fi

echo "== healthz =="
HTTP_BODY_FILE="$(make_body_file)"
HTTP_CODE="$(curl -sS -o "${HTTP_BODY_FILE}" -w "%{http_code}" "${API_BASE_URL}/healthz")"
require_http_code "200" "healthz"

echo "== login =="
HTTP_BODY_FILE="$(make_body_file)"
HTTP_CODE="$(curl -sS -o "${HTTP_BODY_FILE}" -w "%{http_code}" \
  -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}")"
require_http_code "200" "login"
AT="$(jq -r '.access_token' "${HTTP_BODY_FILE}")"
if [[ -z "${AT}" || "${AT}" == "null" ]]; then
  echo "FAIL login.access_token: empty value"
  exit 1
fi

echo "== create smoke user =="
CREATE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  --arg email "${SMOKE_EMAIL}" \
  '{tenant_id:$tenant,building_id:$building,name:"Cloudflare Plain Email Smoke",email:$email,role:"employee",status:"inactive"}')"
api_with_auth POST "/api/v1/users" "${CREATE_PAYLOAD}"
require_http_code "201" "create smoke user"
USER_ID="$(jq -r '.id' "${HTTP_BODY_FILE}")"
if [[ -z "${USER_ID}" || "${USER_ID}" == "null" ]]; then
  echo "FAIL user.id: empty value"
  exit 1
fi

echo "== send invitation email via cloudflare =="
api_with_auth POST "/api/v1/users/${USER_ID}/invite" "{\"tenant_id\":\"${TENANT_ID}\",\"delivery_method\":\"email\"}"
require_http_code "202" "send invitation email"
STATUS="$(jq -r '.status' "${HTTP_BODY_FILE}")"
PROVIDER="$(jq -r '.provider' "${HTTP_BODY_FILE}")"
PROVIDER_ERROR="$(jq -r '.provider_error // ""' "${HTTP_BODY_FILE}")"
if [[ "${STATUS}" != "sent" || "${PROVIDER}" != "cloudflare" ]]; then
  echo "FAIL invitation provider result: status=${STATUS} provider=${PROVIDER} provider_error=${PROVIDER_ERROR}"
  cat "${HTTP_BODY_FILE}"
  echo
  exit 1
fi

DELIVERY_ID="$(jq -r '.id' "${HTTP_BODY_FILE}")"
PROVIDER_DELIVERY_ID="$(jq -r '.provider_delivery_id // ""' "${HTTP_BODY_FILE}")"

echo "PASS: user invitation cloudflare smoke accepted; email=${SMOKE_EMAIL}; delivery_id=${DELIVERY_ID}; provider_delivery_id=${PROVIDER_DELIVERY_ID}; cleanup=${INVITATION_SMOKE_CLEANUP}"
