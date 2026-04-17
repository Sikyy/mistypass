#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18087}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
RESEND_PORT="${RESEND_PORT:-19091}"
RESEND_API_KEY="${RESEND_API_KEY:-resend-dev-token}"
RESEND_ENDPOINT="${RESEND_ENDPOINT:-http://127.0.0.1:${RESEND_PORT}/emails}"
RESEND_EMAIL_FROM="${RESEND_EMAIL_FROM:-alerts@mistypass.local}"
RESEND_RECEIVER_MAP="${RESEND_RECEIVER_MAP:-security=security@example.com}"

API_PID=""
RESEND_PID=""
RESEND_LOG=""

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

function ensure_resend_running() {
  if curl -sS -X POST "${RESEND_ENDPOINT}" -H "Content-Type: application/json" -d '{}' >/dev/null 2>&1; then
    echo "resend mock: already running"
    return
  fi

  RESEND_LOG="/tmp/mp_resend_alert_dispatch_${RANDOM}.jsonl"
  echo "resend mock: starting local server"
  RESEND_PORT="${RESEND_PORT}" RESEND_LOG="${RESEND_LOG}" node -e '
const fs = require("fs")
const http = require("http")
const port = Number(process.env.RESEND_PORT || "19091")
const logFile = process.env.RESEND_LOG

const server = http.createServer((req, res) => {
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
    const record = {
      method: req.method,
      url: req.url,
      headers: req.headers,
      payload,
      at: new Date().toISOString(),
    }
    fs.appendFileSync(logFile, JSON.stringify(record) + "\n")
    res.statusCode = 202
    res.setHeader("content-type", "application/json")
    res.end(JSON.stringify({ ok: true }))
  })
})

server.listen(port, "127.0.0.1", () => {
  console.log("resend mock listening on", port)
})

for (const sig of ["SIGTERM", "SIGINT"]) {
  process.on(sig, () => {
    server.close(() => process.exit(0))
  })
}
' >/tmp/mp_resend_alert_dispatch.log 2>&1 &
  RESEND_PID="$!"

  local i
  for i in {1..40}; do
    if curl -sS -X POST "${RESEND_ENDPOINT}" -H "Content-Type: application/json" -d '{}' >/dev/null 2>&1; then
      echo "resend mock: started"
      return
    fi
    sleep 0.25
  done

  echo "FAIL resend startup: endpoint not ready"
  if [[ -f /tmp/mp_resend_alert_dispatch.log ]]; then
    tail -n 80 /tmp/mp_resend_alert_dispatch.log
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
    WALLET_ALERT_EMAIL_PROVIDER="resend" \
    WALLET_ALERT_EMAIL_FROM="${RESEND_EMAIL_FROM}" \
    WALLET_ALERT_EMAIL_RECEIVER_MAP="${RESEND_RECEIVER_MAP}" \
    WALLET_ALERT_RESEND_ENDPOINT="${RESEND_ENDPOINT}" \
    WALLET_ALERT_RESEND_API_KEY="${RESEND_API_KEY}" \
    WALLET_ALERT_RESEND_TIMEOUT="5s" \
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_job_alert_resend_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_job_alert_resend_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_job_alert_resend_api.log
  fi
  exit 1
}

function cleanup() {
  if [[ -n "${API_PID}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${RESEND_PID}" ]]; then
    kill "${RESEND_PID}" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
ensure_resend_running
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
  --arg name "Alert Dispatch Resend ${RUN_TAG}" \
  '{tenant_id:$tenant,pass_type:"employee",name:$name,status:"active",actor:"qa.alert.dispatch.resend"}')"
TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${TEMPLATE_PAYLOAD}")"
split_response "${TEMPLATE_RAW}"
require_http_code "201" "create template"
TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TEMPLATE_ID}" "template.id"

echo "== queue jobs =="
QUEUE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg t1 "usr-alert-resend-1-${RUN_TAG}" \
  --arg t2 "usr-alert-resend-2-${RUN_TAG}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:[$t1,$t2],execution_mode:"queued",actor:"qa.alert.dispatch.resend"}')"
QUEUE_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${QUEUE_PAYLOAD}")"
split_response "${QUEUE_RAW}"
require_http_code "202" "issue queued batch"

echo "== set template inactive + process to dlq =="
INACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" '{"status":"inactive","actor":"qa.alert.dispatch.resend"}')"
split_response "${INACTIVE_RAW}"
require_http_code "200" "set template inactive"

PROCESS_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:20,worker_count:2,max_retry:3,base_backoff_ms:10,max_backoff_ms:30,actor:"qa.alert.dispatch.resend.worker"}')"
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
  '{tenant_id:$tenant,enabled:true,dlq_alert_threshold:1,window_seconds:600,cooldown_seconds:120,channels:{email:true,whatsapp:false},receiver_groups:["security"],actor:"qa.alert.dispatch.resend"}')"
SUB_RAW="$(api_with_auth PUT "/api/v1/wallet/jobs/alert-subscription" "${SUB_PAYLOAD}")"
split_response "${SUB_RAW}"
require_http_code "200" "upsert alert subscription"

echo "== dispatch alerts (expect provider=resend) =="
DISPATCH_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/alerts/dispatch" "{\"tenant_id\":\"${TENANT_ID}\",\"actor\":\"qa.alert.dispatch.resend\"}")"
split_response "${DISPATCH_RAW}"
require_http_code "200" "dispatch alerts"
DISPATCHED="$(echo "${HTTP_BODY}" | jq -r '.dispatched')"
FAILED="$(echo "${HTTP_BODY}" | jq -r '.failed')"
PROVIDER="$(echo "${HTTP_BODY}" | jq -r '.items[0].provider')"
EMAIL_CHANNEL_SENT="$(echo "${HTTP_BODY}" | jq -r '.items[0].channel_results | map(select(.channel=="email" and .status=="sent")) | length')"
if [[ "${DISPATCHED}" -lt 1 || "${FAILED}" -ne 0 || "${PROVIDER}" != "resend" || "${EMAIL_CHANNEL_SENT}" -lt 1 ]]; then
  echo "FAIL dispatch alerts: expected dispatched>=1 failed=0 provider=resend email channel sent"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== verify provider request =="
if [[ -z "${RESEND_LOG}" || ! -f "${RESEND_LOG}" ]]; then
  echo "FAIL verify provider request: missing log file"
  exit 1
fi
REQUEST_COUNT="$(wc -l < "${RESEND_LOG}" | tr -d ' ')"
if [[ "${REQUEST_COUNT}" -lt 1 ]]; then
  echo "FAIL verify provider request: expected at least one request, got ${REQUEST_COUNT}"
  cat "${RESEND_LOG}"
  exit 1
fi
LAST_LINE="$(tail -n 1 "${RESEND_LOG}")"
AUTH_HEADER="$(printf '%s\n' "${LAST_LINE}" | jq -r '.headers.authorization // ""')"
REQ_FROM="$(printf '%s\n' "${LAST_LINE}" | jq -r '.payload.from // ""')"
REQ_TO_COUNT="$(printf '%s\n' "${LAST_LINE}" | jq -r '.payload.to | length')"
REQ_TO_SECURITY="$(printf '%s\n' "${LAST_LINE}" | jq -r '.payload.to | map(select(.=="security@example.com")) | length')"
if [[ "${AUTH_HEADER}" != "Bearer ${RESEND_API_KEY}" ]]; then
  echo "FAIL verify provider auth: expected Bearer token"
  echo "${LAST_LINE}"
  exit 1
fi
if [[ "${REQ_FROM}" != "${RESEND_EMAIL_FROM}" ]]; then
  echo "FAIL verify provider from: expected ${RESEND_EMAIL_FROM}, got ${REQ_FROM}"
  echo "${LAST_LINE}"
  exit 1
fi
if [[ "${REQ_TO_COUNT}" -lt 1 || "${REQ_TO_SECURITY}" -lt 1 ]]; then
  echo "FAIL verify provider to: expected security@example.com"
  echo "${LAST_LINE}"
  exit 1
fi

echo "PASS: wallet job alert dispatch resend regression complete"
