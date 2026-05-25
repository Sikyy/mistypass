#!/usr/bin/env zsh
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-https://staging-api.mistyislet.com}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
REPORT_SMOKE_RECIPIENTS="${REPORT_SMOKE_RECIPIENTS:-}"
REPORT_SMOKE_CLEANUP="${REPORT_SMOKE_CLEANUP:-1}"
ALLOW_NON_DELIVERABLE="${ALLOW_NON_DELIVERABLE:-0}"

HTTP_CODE=""
HTTP_BODY=""
AT=""
SCHEDULE_ID=""

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

function cleanup() {
  if [[ "${REPORT_SMOKE_CLEANUP}" != "1" || -z "${SCHEDULE_ID}" || -z "${AT}" ]]; then
    return
  fi
  api_with_auth DELETE "/api/v1/report-schedules/${SCHEDULE_ID}?tenant_id=${TENANT_ID}" >/dev/null 2>&1 || true
}

trap cleanup EXIT

if [[ -z "${REPORT_SMOKE_RECIPIENTS}" ]]; then
  echo "FAIL report cloudflare smoke: set REPORT_SMOKE_RECIPIENTS to one or more real inboxes, comma-separated"
  echo "example: REPORT_SMOKE_RECIPIENTS=ops@mistyislet.com ./docs/testing/curl-report-schedule-cloudflare.zsh"
  exit 2
fi

RECIPIENTS_JSON="$(jq -nc --arg recipients "${REPORT_SMOKE_RECIPIENTS}" '$recipients | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))')"
RECIPIENT_COUNT="$(echo "${RECIPIENTS_JSON}" | jq -r 'length')"
if [[ "${RECIPIENT_COUNT}" -eq 0 ]]; then
  echo "FAIL report cloudflare smoke: no usable recipients"
  exit 2
fi
if [[ "${ALLOW_NON_DELIVERABLE}" != "1" ]] && echo "${RECIPIENTS_JSON}" | jq -e 'any(.[]; test("@example\\.|@mistypass\\.local$"))' >/dev/null; then
  echo "FAIL report cloudflare smoke: recipients must be real deliverable inboxes"
  echo "${RECIPIENTS_JSON}"
  exit 2
fi

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
SCHEDULE_NAME="Staging Cloudflare Report Smoke ${RUN_TAG}"

echo "== healthz =="
HEALTH_RAW="$(curl -sS "${API_BASE_URL}/healthz" -w $'\n%{http_code}')"
split_response "${HEALTH_RAW}"
require_http_code "200" "healthz"

echo "== login =="
LOGIN_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

echo "== provider status =="
STATUS_RAW="$(api_with_auth GET "/api/v1/report-schedules/provider-status?tenant_id=${TENANT_ID}")"
split_response "${STATUS_RAW}"
require_http_code "200" "provider status"
STATUS_READY="$(echo "${HTTP_BODY}" | jq -r '.ready')"
STATUS_PROVIDER="$(echo "${HTTP_BODY}" | jq -r '.provider')"
STATUS_FROM="$(echo "${HTTP_BODY}" | jq -r '.from')"
STATUS_MISSING="$(echo "${HTTP_BODY}" | jq -r '(.missing // []) | length')"
require_equals "${STATUS_READY}" "true" "provider status.ready"
require_equals "${STATUS_PROVIDER}" "cloudflare" "provider status.provider"
require_non_empty "${STATUS_FROM}" "provider status.from"
require_equals "${STATUS_MISSING}" "0" "provider status.missing"

echo "== create report schedule =="
CREATE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg name "${SCHEDULE_NAME}" \
  --argjson recipients "${RECIPIENTS_JSON}" \
  '{report_schedule:{tenant_id:$tenant,name:$name,report_type:"events",frequency:"daily",recipients:$recipients,format:"pdf",enabled:true}}')"
CREATE_RAW="$(api_with_auth POST "/api/v1/report-schedules" "${CREATE_PAYLOAD}")"
split_response "${CREATE_RAW}"
require_http_code "201" "create report schedule"
SCHEDULE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
SCHEDULE_FORMAT="$(echo "${HTTP_BODY}" | jq -r '.format')"
require_non_empty "${SCHEDULE_ID}" "report schedule.id"
require_equals "${SCHEDULE_FORMAT}" "pdf" "report schedule.format"

echo "== send report schedule via cloudflare =="
SEND_RAW="$(api_with_auth POST "/api/v1/report-schedules/${SCHEDULE_ID}/send?tenant_id=${TENANT_ID}")"
split_response "${SEND_RAW}"
if [[ "${HTTP_CODE}" != "200" && "${HTTP_CODE}" != "202" ]]; then
  echo "FAIL send report schedule: expected HTTP 200/202, got ${HTTP_CODE}"
  echo "${HTTP_BODY}"
  exit 1
fi
LAST_SENT_AT="$(echo "${HTTP_BODY}" | jq -r '.last_sent_at')"

echo "== wait for report schedule delivery =="
AUDIT_MATCH="false"
for attempt in {1..18}; do
  GET_RAW="$(api_with_auth GET "/api/v1/report-schedules/${SCHEDULE_ID}?tenant_id=${TENANT_ID}")"
  split_response "${GET_RAW}"
  if [[ "${HTTP_CODE}" == "200" ]]; then
    LAST_SENT_AT="$(echo "${HTTP_BODY}" | jq -r '.last_sent_at')"
  fi

  AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=report_schedule_sent&source=report_schedule&limit=10")"
  split_response "${AUDIT_RAW}"
  require_http_code "200" "report schedule audit log"
  if jq -e \
    --arg schedule_id "${SCHEDULE_ID}" \
    '
    (.items // [])
    | any(
        .action == "report_schedule_sent"
        and .source == "report_schedule"
        and (.target // "" | contains("schedule_id=" + $schedule_id))
        and (.target // "" | contains("provider=cloudflare"))
      )
    ' <<<"${HTTP_BODY}" >/dev/null; then
    AUDIT_MATCH="true"
  fi

  if [[ -n "${LAST_SENT_AT}" && "${LAST_SENT_AT}" != "null" && "${AUDIT_MATCH}" == "true" ]]; then
    break
  fi
  sleep 5
done
require_non_empty "${LAST_SENT_AT}" "report schedule.last_sent_at"
require_equals "${AUDIT_MATCH}" "true" "report schedule audit log"

echo "PASS: report schedule cloudflare smoke accepted; recipients=${RECIPIENT_COUNT}; schedule_id=${SCHEDULE_ID}; cleanup=${REPORT_SMOKE_CLEANUP}"
