#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18188}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
AUDIT_LOG_ID="${AUDIT_LOG_ID:-aud_3002}"
RECEIVER_PORT="${RECEIVER_PORT:-19093}"
RECEIVER_BASE_URL="${RECEIVER_BASE_URL:-http://127.0.0.1:${RECEIVER_PORT}}"
RECEIVER_LOG="${RECEIVER_LOG:-/tmp/mp_audit_webhook_receiver.jsonl}"
SIGNING_SECRET="${SIGNING_SECRET:-$(printf '%s-%s-%s' audit webhook key)}"
API_PID=""
RECEIVER_PID=""
API_LOG="${API_LOG:-/tmp/mp_audit_webhook_receiver_api.log}"

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

function ensure_receiver_running() {
  if curl -sS "${RECEIVER_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "receiver: already running"
    : >"${RECEIVER_LOG}"
    return
  fi

  : >"${RECEIVER_LOG}"
  echo "receiver: starting local audit webhook server"
  RECEIVER_PORT="${RECEIVER_PORT}" RECEIVER_LOG="${RECEIVER_LOG}" SIGNING_SECRET="${SIGNING_SECRET}" node -e '
const crypto = require("crypto")
const fs = require("fs")
const http = require("http")

const port = Number(process.env.RECEIVER_PORT || "19093")
const logFile = process.env.RECEIVER_LOG
const signingSecret = process.env.SIGNING_SECRET || ""
const counts = new Map()

function signatureValid(timestamp, body, signature) {
  if (!signingSecret || !timestamp || !signature) return false
  const expected = "sha256=" + crypto
    .createHmac("sha256", signingSecret)
    .update(`${timestamp}.${body}`)
    .digest("hex")
  return crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(signature))
}

const server = http.createServer((req, res) => {
  if (req.method === "GET" && req.url === "/healthz") {
    res.statusCode = 200
    res.end("ok")
    return
  }

  let body = ""
  req.on("data", (chunk) => {
    body += chunk.toString()
  })
  req.on("end", () => {
    let payload = {}
    try {
      payload = JSON.parse(body || "{}")
    } catch {
      payload = { _raw: body }
    }

    const nextCount = (counts.get(req.url) || 0) + 1
    counts.set(req.url, nextCount)
    const signature = String(req.headers["x-mistypass-signature"] || "")
    const timestamp = String(req.headers["x-mistypass-signature-timestamp"] || "")
    const record = {
      method: req.method,
      url: req.url,
      count: nextCount,
      event_id: req.headers["x-mistypass-event-id"] || "",
      event_action: req.headers["x-mistypass-event-action"] || "",
      signature,
      signature_timestamp: timestamp,
      signature_valid: signatureValid(timestamp, body, signature),
      payload,
      at: new Date().toISOString(),
    }
    fs.appendFileSync(logFile, JSON.stringify(record) + "\n")

    if (req.url === "/hooks/audit-retry" && nextCount < 3) {
      res.statusCode = 500
      res.setHeader("content-type", "application/json")
      res.end(JSON.stringify({ ok: false, attempt: nextCount }))
      return
    }

    res.statusCode = 202
    res.setHeader("content-type", "application/json")
    res.end(JSON.stringify({ ok: true, attempt: nextCount }))
  })
})

server.listen(port, "127.0.0.1", () => {
  console.log("audit receiver mock listening on", port)
})

for (const sig of ["SIGTERM", "SIGINT"]) {
  process.on(sig, () => server.close(() => process.exit(0)))
}
' >/tmp/mp_audit_webhook_receiver.log 2>&1 &
  RECEIVER_PID="$!"

  local i
  for i in {1..40}; do
    if curl -sS "${RECEIVER_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "receiver: started"
      return
    fi
    sleep 0.25
  done

  echo "FAIL receiver startup: healthz not ready"
  if [[ -f /tmp/mp_audit_webhook_receiver.log ]]; then
    tail -n 80 /tmp/mp_audit_webhook_receiver.log
  fi
  exit 1
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
      ENABLE_DEMO_USERS=true \
      DISABLE_LOGIN_RATE_LIMIT=true \
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

function kill_port() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    local pids
    pids="$(lsof -ti tcp:"${port}" 2>/dev/null || true)"
    for pid in ${(f)pids}; do
      [[ -n "${pid}" ]] && kill "${pid}" >/dev/null 2>&1 || true
    done
  fi
}

function cleanup() {
  if [[ -n "${API_PID}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
    kill_port "${API_PORT}"
  fi
  if [[ -n "${RECEIVER_PID}" ]]; then
    kill "${RECEIVER_PID}" >/dev/null 2>&1 || true
    kill_port "${RECEIVER_PORT}"
  fi
}

trap cleanup EXIT

ensure_receiver_running
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

echo "== configure success receiver =="
SUCCESS_ENDPOINT="${RECEIVER_BASE_URL}/hooks/audit"
SUCCESS_CONFIG="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg endpoint "${SUCCESS_ENDPOINT}" \
  --arg signing "${SIGNING_SECRET}" \
  '{tenant_id:$tenant,enabled:true,endpoint:$endpoint,actions:["gateway_reboot"],signing_secret:$signing,updated_by:"qa.audit.receiver"}')"
SUCCESS_CONFIG_RAW="$(api_with_auth PUT "/api/v1/audit/webhook/config" "${SUCCESS_CONFIG}")"
split_response "${SUCCESS_CONFIG_RAW}"
require_http_code "200" "configure success receiver"
CONFIG_ENDPOINT="$(echo "${HTTP_BODY}" | jq -r '.endpoint')"
CONFIG_SECRET_PRESENT="$(echo "${HTTP_BODY}" | jq -r 'has("signing_secret")')"
require_equals "${CONFIG_ENDPOINT}" "${SUCCESS_ENDPOINT}" "success config endpoint"
require_equals "${CONFIG_SECRET_PRESENT}" "false" "success config should not echo signing_secret"

echo "== dispatch to success receiver =="
DISPATCH_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg logid "${AUDIT_LOG_ID}" \
  '{tenant_id:$tenant,audit_log_id:$logid}')"
DISPATCH_SUCCESS_RAW="$(api_with_auth POST "/api/v1/audit/webhook/dispatch" "${DISPATCH_PAYLOAD}")"
split_response "${DISPATCH_SUCCESS_RAW}"
require_http_code "200" "dispatch success receiver"
SUCCESS_STATUS="$(echo "${HTTP_BODY}" | jq -r '.delivery.status')"
SUCCESS_HTTP_STATUS="$(echo "${HTTP_BODY}" | jq -r '.delivery.http_status')"
SUCCESS_ATTEMPTS="$(echo "${HTTP_BODY}" | jq -r '.delivery.attempt_count')"
SUCCESS_ACTION="$(echo "${HTTP_BODY}" | jq -r '.event.action')"
require_equals "${SUCCESS_STATUS}" "success" "success delivery status"
require_equals "${SUCCESS_HTTP_STATUS}" "202" "success delivery http_status"
require_equals "${SUCCESS_ATTEMPTS}" "1" "success delivery attempt_count"
require_equals "${SUCCESS_ACTION}" "gateway_reboot" "success event action"

echo "== verify receiver request and signature =="
SUCCESS_RECEIVER_COUNT="$(jq -sr --arg url "/hooks/audit" '[.[] | select(.url==$url)] | length' "${RECEIVER_LOG}")"
SUCCESS_SIGNATURE_VALID="$(jq -sr --arg url "/hooks/audit" '[.[] | select(.url==$url)][0].signature_valid' "${RECEIVER_LOG}")"
SUCCESS_EVENT_ID="$(jq -sr --arg url "/hooks/audit" '[.[] | select(.url==$url)][0].event_id' "${RECEIVER_LOG}")"
SUCCESS_EVENT_ACTION="$(jq -sr --arg url "/hooks/audit" '[.[] | select(.url==$url)][0].event_action' "${RECEIVER_LOG}")"
SUCCESS_PAYLOAD_TENANT="$(jq -sr --arg url "/hooks/audit" '[.[] | select(.url==$url)][0].payload.tenant_id' "${RECEIVER_LOG}")"
SUCCESS_PAYLOAD_EVENT_ID="$(jq -sr --arg url "/hooks/audit" '[.[] | select(.url==$url)][0].payload.event.id' "${RECEIVER_LOG}")"
SUCCESS_SIGNATURE_PREFIX="$(jq -sr --arg url "/hooks/audit" '[.[] | select(.url==$url)][0].signature | startswith("sha256=")' "${RECEIVER_LOG}")"
require_equals "${SUCCESS_RECEIVER_COUNT}" "1" "success receiver request count"
require_equals "${SUCCESS_SIGNATURE_VALID}" "true" "success receiver signature"
require_equals "${SUCCESS_EVENT_ID}" "${AUDIT_LOG_ID}" "success receiver event id header"
require_equals "${SUCCESS_EVENT_ACTION}" "gateway_reboot" "success receiver action header"
require_equals "${SUCCESS_PAYLOAD_TENANT}" "${TENANT_ID}" "success receiver tenant payload"
require_equals "${SUCCESS_PAYLOAD_EVENT_ID}" "${AUDIT_LOG_ID}" "success receiver event payload"
require_equals "${SUCCESS_SIGNATURE_PREFIX}" "true" "success receiver signature prefix"

echo "== list success delivery =="
DELIVERIES_SUCCESS_RAW="$(api_with_auth GET "/api/v1/audit/webhook/deliveries?tenant_id=${TENANT_ID}&limit=5")"
split_response "${DELIVERIES_SUCCESS_RAW}"
require_http_code "200" "list success deliveries"
DELIVERY_SUCCESS_COUNT="$(echo "${HTTP_BODY}" | jq -r --arg logid "${AUDIT_LOG_ID}" '[.items[] | select(.audit_log_id==$logid and .status=="success" and .http_status==202)] | length')"
if [[ "${DELIVERY_SUCCESS_COUNT}" -lt 1 ]]; then
  echo "FAIL list success deliveries: expected successful delivery for ${AUDIT_LOG_ID}"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== configure retry receiver =="
RETRY_ENDPOINT="${RECEIVER_BASE_URL}/hooks/audit-retry"
RETRY_CONFIG="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg endpoint "${RETRY_ENDPOINT}" \
  --arg signing "${SIGNING_SECRET}" \
  '{tenant_id:$tenant,enabled:true,endpoint:$endpoint,actions:["gateway_reboot"],signing_secret:$signing,updated_by:"qa.audit.receiver.retry"}')"
RETRY_CONFIG_RAW="$(api_with_auth PUT "/api/v1/audit/webhook/config" "${RETRY_CONFIG}")"
split_response "${RETRY_CONFIG_RAW}"
require_http_code "200" "configure retry receiver"

echo "== dispatch to retry receiver =="
DISPATCH_RETRY_RAW="$(api_with_auth POST "/api/v1/audit/webhook/dispatch" "${DISPATCH_PAYLOAD}")"
split_response "${DISPATCH_RETRY_RAW}"
require_http_code "200" "dispatch retry receiver"
RETRY_STATUS="$(echo "${HTTP_BODY}" | jq -r '.delivery.status')"
RETRY_HTTP_STATUS="$(echo "${HTTP_BODY}" | jq -r '.delivery.http_status')"
RETRY_ATTEMPTS="$(echo "${HTTP_BODY}" | jq -r '.delivery.attempt_count')"
require_equals "${RETRY_STATUS}" "success" "retry delivery status"
require_equals "${RETRY_HTTP_STATUS}" "202" "retry delivery http_status"
require_equals "${RETRY_ATTEMPTS}" "3" "retry delivery attempt_count"

echo "== verify receiver retry attempts =="
RETRY_RECEIVER_COUNT="$(jq -sr --arg url "/hooks/audit-retry" '[.[] | select(.url==$url)] | length' "${RECEIVER_LOG}")"
RETRY_SIGNATURES_VALID="$(jq -sr --arg url "/hooks/audit-retry" '[.[] | select(.url==$url and .signature_valid==true)] | length' "${RECEIVER_LOG}")"
RETRY_COUNTS="$(jq -sr --arg url "/hooks/audit-retry" '[.[] | select(.url==$url) | .count] | join(",")' "${RECEIVER_LOG}")"
require_equals "${RETRY_RECEIVER_COUNT}" "3" "retry receiver request count"
require_equals "${RETRY_SIGNATURES_VALID}" "3" "retry receiver signatures"
require_equals "${RETRY_COUNTS}" "1,2,3" "retry receiver attempt order"

echo "PASS: audit webhook receiver regression complete"
