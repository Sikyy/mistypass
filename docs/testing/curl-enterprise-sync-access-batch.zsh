#!/usr/bin/env zsh
set -euo pipefail
setopt no_bg_nice

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
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_enterprise_sync_access_api.log 2>&1
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
  if [[ -f /tmp/mp_enterprise_sync_access_api.log ]]; then
    tail -n 80 /tmp/mp_enterprise_sync_access_api.log
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
VALID_EMAIL="batch.sync.${RUN_TAG}@sudirman.co"
REJECT_EMAIL="batch.reject.${RUN_TAG}@example.com"
SYNC_REQUEST_ID="req-${RUN_TAG}"

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

echo "== enterprise sync -> access batch upsert =="
SYNC_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg valid_email "${VALID_EMAIL}" \
  --arg reject_email "${REJECT_EMAIL}" \
  --arg request_id "${SYNC_REQUEST_ID}" \
  --arg run "${RUN_TAG}" \
  '{
    tenant_id: $tenant,
    source: "manual_sync",
    actor: "qa",
    request_id: $request_id,
    employees: [
      {
        external_id: ("hr-" + $run + "-ok"),
        email: $valid_email,
        full_name: ("Batch Sync " + $run),
        department: "IT",
        job_title: "Engineer",
        location: "Jakarta",
        status: "active"
      },
      {
        external_id: ("hr-" + $run + "-reject"),
        email: $reject_email,
        full_name: ("Reject Domain " + $run),
        department: "IT",
        job_title: "Engineer",
        location: "Jakarta",
        status: "active"
      }
    ]
  }')"
SYNC_RAW="$(api_with_auth POST "/api/v1/enterprise/employees/sync" "${SYNC_PAYLOAD}")"
split_response "${SYNC_RAW}"
require_http_code "202" "enterprise sync"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.job.status')" != "completed" ]]; then
  echo "FAIL enterprise sync: unexpected job status"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "$(echo "${HTTP_BODY}" | jq -r '.access_sync.created')" != "1" ]]; then
  echo "FAIL enterprise sync: access_sync.created should be 1"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "$(echo "${HTTP_BODY}" | jq -r '.access_sync.rejected')" != "0" ]]; then
  echo "FAIL enterprise sync: access_sync.rejected should be 0"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "$(echo "${HTTP_BODY}" | jq -r '.items | length')" != "1" ]]; then
  echo "FAIL enterprise sync: items length should be 1 (domain mismatch rejected in enterprise layer)"
  echo "${HTTP_BODY}"
  exit 1
fi
FIRST_JOB_ID="$(echo "${HTTP_BODY}" | jq -r '.job.id')"
FIRST_ACCESS_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.created')"
FIRST_ACCESS_UPDATED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.updated')"
FIRST_ACCESS_REJECTED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.rejected')"
require_non_empty "${FIRST_JOB_ID}" "enterprise sync.job.id"

echo "== enterprise sync retry with same request_id (idempotent) =="
SYNC_RETRY_RAW="$(api_with_auth POST "/api/v1/enterprise/employees/sync" "${SYNC_PAYLOAD}")"
split_response "${SYNC_RETRY_RAW}"
require_http_code "202" "enterprise sync retry"
SECOND_JOB_ID="$(echo "${HTTP_BODY}" | jq -r '.job.id')"
SECOND_ACCESS_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.created')"
SECOND_ACCESS_UPDATED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.updated')"
SECOND_ACCESS_REJECTED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.rejected')"
if [[ "${SECOND_JOB_ID}" != "${FIRST_JOB_ID}" ]]; then
  echo "FAIL enterprise sync retry: job.id should be idempotent"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${SECOND_ACCESS_CREATED}" != "${FIRST_ACCESS_CREATED}" || "${SECOND_ACCESS_UPDATED}" != "${FIRST_ACCESS_UPDATED}" || "${SECOND_ACCESS_REJECTED}" != "${FIRST_ACCESS_REJECTED}" ]]; then
  echo "FAIL enterprise sync retry: access_sync counters should match first request"
  echo "first=${FIRST_ACCESS_CREATED}/${FIRST_ACCESS_UPDATED}/${FIRST_ACCESS_REJECTED}"
  echo "second=${SECOND_ACCESS_CREATED}/${SECOND_ACCESS_UPDATED}/${SECOND_ACCESS_REJECTED}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== enterprise sync reconcile by request_id =="
RECONCILE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg request_id "${SYNC_REQUEST_ID}" \
  '{tenant_id:$tenant,request_id:$request_id}')"
RECONCILE_RAW="$(api_with_auth POST "/api/v1/enterprise/employees/sync/reconcile" "${RECONCILE_PAYLOAD}")"
split_response "${RECONCILE_RAW}"
require_http_code "200" "enterprise sync reconcile"
RECONCILE_JOB_ID="$(echo "${HTTP_BODY}" | jq -r '.job.id')"
RECONCILE_ACCESS_CREATED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.created')"
RECONCILE_ACCESS_UPDATED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.updated')"
RECONCILE_ACCESS_REJECTED="$(echo "${HTTP_BODY}" | jq -r '.access_sync.rejected')"
if [[ "${RECONCILE_JOB_ID}" != "${FIRST_JOB_ID}" ]]; then
  echo "FAIL enterprise sync reconcile: job.id should match original request"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${RECONCILE_ACCESS_CREATED}" != "${FIRST_ACCESS_CREATED}" || "${RECONCILE_ACCESS_UPDATED}" != "${FIRST_ACCESS_UPDATED}" || "${RECONCILE_ACCESS_REJECTED}" != "${FIRST_ACCESS_REJECTED}" ]]; then
  echo "FAIL enterprise sync reconcile: access_sync counters should match original request"
  echo "first=${FIRST_ACCESS_CREATED}/${FIRST_ACCESS_UPDATED}/${FIRST_ACCESS_REJECTED}"
  echo "reconcile=${RECONCILE_ACCESS_CREATED}/${RECONCILE_ACCESS_UPDATED}/${RECONCILE_ACCESS_REJECTED}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== enterprise sync request status audit =="
SYNC_REQUEST_STATUS_RAW="$(api_with_auth GET "/api/v1/enterprise/sync-requests?tenant_id=${TENANT_ID}&request_id=${SYNC_REQUEST_ID}")"
split_response "${SYNC_REQUEST_STATUS_RAW}"
require_http_code "200" "enterprise sync request status"
STATUS_JOB_ID="$(echo "${HTTP_BODY}" | jq -r '.item.result.job.id')"
STATUS_ACCESS_APPLIED="$(echo "${HTTP_BODY}" | jq -r '.item.access_applied')"
STATUS_ATTEMPT_COUNT="$(echo "${HTTP_BODY}" | jq -r '.item.access_attempt_count')"
if [[ "${STATUS_JOB_ID}" != "${FIRST_JOB_ID}" ]]; then
  echo "FAIL enterprise sync request status: job.id mismatch"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${STATUS_ACCESS_APPLIED}" != "true" ]]; then
  echo "FAIL enterprise sync request status: access_applied should be true"
  echo "${HTTP_BODY}"
  exit 1
fi
if [[ "${STATUS_ATTEMPT_COUNT}" -lt 1 ]]; then
  echo "FAIL enterprise sync request status: access_attempt_count should be >= 1"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== enterprise reconcile-pending should be no-op after applied =="
RECONCILE_PENDING_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:10}')"
RECONCILE_PENDING_RAW="$(api_with_auth POST "/api/v1/enterprise/sync-requests/reconcile-pending" "${RECONCILE_PENDING_PAYLOAD}")"
split_response "${RECONCILE_PENDING_RAW}"
require_http_code "200" "enterprise reconcile pending"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.processed')" != "0" ]]; then
  echo "FAIL enterprise reconcile pending: processed should be 0 after sync applied"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== enterprise reconcile-pending invalid limit should return 400 =="
INVALID_RECONCILE_PENDING_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:-1}')"
INVALID_RECONCILE_PENDING_RAW="$(api_with_auth POST "/api/v1/enterprise/sync-requests/reconcile-pending" "${INVALID_RECONCILE_PENDING_PAYLOAD}")"
split_response "${INVALID_RECONCILE_PENDING_RAW}"
require_http_code "400" "enterprise reconcile pending invalid limit"

echo "== enterprise sync reconcile missing request_id should return 404 =="
MISSING_RECONCILE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg request_id "missing-${RUN_TAG}" \
  '{tenant_id:$tenant,request_id:$request_id}')"
MISSING_RECONCILE_RAW="$(api_with_auth POST "/api/v1/enterprise/employees/sync/reconcile" "${MISSING_RECONCILE_PAYLOAD}")"
split_response "${MISSING_RECONCILE_RAW}"
require_http_code "404" "enterprise sync reconcile missing request"
if [[ "$(echo "${HTTP_BODY}" | jq -r '.error')" != "sync request not found" ]]; then
  echo "FAIL enterprise sync reconcile missing request: unexpected error message"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== verify access users =="
USERS_RAW="$(api_with_auth GET "/api/v1/users?tenant_id=${TENANT_ID}")"
split_response "${USERS_RAW}"
require_http_code "200" "list users"
VALID_EXISTS="$(echo "${HTTP_BODY}" | jq --arg email "${VALID_EMAIL}" '[.items[] | select((.email | ascii_downcase) == ($email | ascii_downcase))] | length')"
REJECT_EXISTS="$(echo "${HTTP_BODY}" | jq --arg email "${REJECT_EMAIL}" '[.items[] | select((.email | ascii_downcase) == ($email | ascii_downcase))] | length')"
if [[ "${VALID_EXISTS}" != "1" ]]; then
  echo "FAIL users verify: valid email not found"
  exit 1
fi
if [[ "${REJECT_EXISTS}" != "0" ]]; then
  echo "FAIL users verify: rejected email should not be written to access users"
  exit 1
fi

echo "PASS: enterprise sync access batch upsert regression complete"
