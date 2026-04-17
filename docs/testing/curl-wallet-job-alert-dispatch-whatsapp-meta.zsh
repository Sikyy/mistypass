#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18089}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
WHATSAPP_MOCK_PORT="${WHATSAPP_MOCK_PORT:-19092}"
WHATSAPP_ENDPOINT_BASE="${WHATSAPP_ENDPOINT_BASE:-http://127.0.0.1:${WHATSAPP_MOCK_PORT}/v22.0}"
WHATSAPP_API_KEY="${WHATSAPP_API_KEY:-wa-dev-token}"
WHATSAPP_PHONE_NUMBER_ID="${WHATSAPP_PHONE_NUMBER_ID:-112233445566}"
WHATSAPP_RECEIVER_MAP="${WHATSAPP_RECEIVER_MAP:-security=+62811111111}"

API_PID=""
WA_PID=""
WA_LOG=""

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

function ensure_whatsapp_mock_running() {
  WA_LOG="/tmp/mp_whatsapp_meta_dispatch_${RANDOM}.jsonl"
  echo "whatsapp meta mock: starting local server"
  WHATSAPP_MOCK_PORT="${WHATSAPP_MOCK_PORT}" WHATSAPP_LOG="${WA_LOG}" node -e '
const fs = require("fs")
const http = require("http")
const port = Number(process.env.WHATSAPP_MOCK_PORT || "19092")
const logFile = process.env.WHATSAPP_LOG
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
    fs.appendFileSync(logFile, JSON.stringify({
      method: req.method,
      url: req.url,
      headers: req.headers,
      payload,
      at: new Date().toISOString(),
    }) + "\n")
    res.statusCode = 200
    res.setHeader("content-type", "application/json")
    res.end(JSON.stringify({ ok: true }))
  })
})
server.listen(port, "127.0.0.1", () => {
  console.log("whatsapp meta mock listening on", port)
})
for (const sig of ["SIGTERM", "SIGINT"]) {
  process.on(sig, () => server.close(() => process.exit(0)))
}
' >/tmp/mp_whatsapp_meta_dispatch.log 2>&1 &
  WA_PID="$!"

  local i
  for i in {1..40}; do
    if curl -sS -X POST "${WHATSAPP_ENDPOINT_BASE}/${WHATSAPP_PHONE_NUMBER_ID}/messages" \
      -H "Content-Type: application/json" \
      -d '{"messaging_product":"whatsapp","to":"+62811111111","type":"text","text":{"body":"ping"}}' >/dev/null 2>&1; then
      echo "whatsapp meta mock: started"
      return
    fi
    sleep 0.25
  done

  echo "FAIL whatsapp meta mock startup"
  if [[ -f /tmp/mp_whatsapp_meta_dispatch.log ]]; then
    tail -n 80 /tmp/mp_whatsapp_meta_dispatch.log
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
    WALLET_ALERT_EMAIL_PROVIDER="mock" \
    WALLET_ALERT_WHATSAPP_PROVIDER="meta" \
    WALLET_ALERT_WHATSAPP_RECEIVER_MAP="${WHATSAPP_RECEIVER_MAP}" \
    WALLET_ALERT_WHATSAPP_ENDPOINT="${WHATSAPP_ENDPOINT_BASE}" \
    WALLET_ALERT_WHATSAPP_API_KEY="${WHATSAPP_API_KEY}" \
    WALLET_ALERT_WHATSAPP_PHONE_NUMBER_ID="${WHATSAPP_PHONE_NUMBER_ID}" \
    WALLET_ALERT_WHATSAPP_TIMEOUT="5s" \
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_wallet_job_alert_whatsapp_meta_api.log 2>&1
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
  if [[ -f /tmp/mp_wallet_job_alert_whatsapp_meta_api.log ]]; then
    tail -n 80 /tmp/mp_wallet_job_alert_whatsapp_meta_api.log
  fi
  exit 1
}

function cleanup() {
  if [[ -n "${API_PID}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${WA_PID}" ]]; then
    kill "${WA_PID}" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
ensure_whatsapp_mock_running
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
  --arg name "Alert Dispatch WhatsApp Meta ${RUN_TAG}" \
  '{tenant_id:$tenant,pass_type:"employee",name:$name,status:"active",actor:"qa.alert.dispatch.whatsapp.meta"}')"
TEMPLATE_RAW="$(api_with_auth POST "/api/v1/wallet/templates" "${TEMPLATE_PAYLOAD}")"
split_response "${TEMPLATE_RAW}"
require_http_code "201" "create template"
TEMPLATE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${TEMPLATE_ID}" "template.id"

echo "== queue jobs =="
QUEUE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg template "${TEMPLATE_ID}" \
  --arg t1 "usr-alert-whatsapp-meta-1-${RUN_TAG}" \
  --arg t2 "usr-alert-whatsapp-meta-2-${RUN_TAG}" \
  '{tenant_id:$tenant,template_id:$template,target_type:"user",target_ids:[$t1,$t2],execution_mode:"queued",actor:"qa.alert.dispatch.whatsapp.meta"}')"
QUEUE_RAW="$(api_with_auth POST "/api/v1/wallet/passes/issue-batch" "${QUEUE_PAYLOAD}")"
split_response "${QUEUE_RAW}"
require_http_code "202" "issue queued batch"

echo "== set template inactive + process to dlq =="
INACTIVE_RAW="$(api_with_auth PATCH "/api/v1/wallet/templates/${TEMPLATE_ID}/status?tenant_id=${TENANT_ID}" '{"status":"inactive","actor":"qa.alert.dispatch.whatsapp.meta"}')"
split_response "${INACTIVE_RAW}"
require_http_code "200" "set template inactive"

PROCESS_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,limit:20,worker_count:2,max_retry:3,base_backoff_ms:10,max_backoff_ms:30,actor:"qa.alert.dispatch.whatsapp.meta.worker"}')"
PROCESS_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/process" "${PROCESS_PAYLOAD}")"
split_response "${PROCESS_RAW}"
require_http_code "200" "process queued jobs"

echo "== upsert alert subscription (whatsapp only) =="
SUB_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  '{tenant_id:$tenant,enabled:true,dlq_alert_threshold:1,window_seconds:600,cooldown_seconds:120,channels:{email:false,whatsapp:true},receiver_groups:["security"],actor:"qa.alert.dispatch.whatsapp.meta"}')"
SUB_RAW="$(api_with_auth PUT "/api/v1/wallet/jobs/alert-subscription" "${SUB_PAYLOAD}")"
split_response "${SUB_RAW}"
require_http_code "200" "upsert alert subscription"

echo "== dispatch alerts =="
DISPATCH_RAW="$(api_with_auth POST "/api/v1/wallet/jobs/alerts/dispatch" "{\"tenant_id\":\"${TENANT_ID}\",\"actor\":\"qa.alert.dispatch.whatsapp.meta\"}")"
split_response "${DISPATCH_RAW}"
require_http_code "200" "dispatch alerts"
DISPATCHED="$(echo "${HTTP_BODY}" | jq -r '.dispatched')"
FAILED="$(echo "${HTTP_BODY}" | jq -r '.failed')"
WHATSAPP_META_SENT="$(echo "${HTTP_BODY}" | jq -r '.items[0].channel_results | map(select(.channel=="whatsapp" and .status=="sent" and .provider=="meta")) | length')"
if [[ "${DISPATCHED}" -lt 1 || "${FAILED}" -ne 0 || "${WHATSAPP_META_SENT}" -lt 1 ]]; then
  echo "FAIL dispatch alerts: expected dispatched>=1 failed=0 and whatsapp meta sent"
  echo "${HTTP_BODY}"
  exit 1
fi

echo "== verify provider request =="
if [[ -z "${WA_LOG}" || ! -f "${WA_LOG}" ]]; then
  echo "FAIL verify provider request: missing log file"
  exit 1
fi
REQUEST_COUNT="$(wc -l < "${WA_LOG}" | tr -d ' ')"
if [[ "${REQUEST_COUNT}" -lt 1 ]]; then
  echo "FAIL verify provider request: expected at least one request"
  cat "${WA_LOG}"
  exit 1
fi
LAST_LINE="$(tail -n 1 "${WA_LOG}")"
AUTH_HEADER="$(printf '%s\n' "${LAST_LINE}" | jq -r '.headers.authorization // ""')"
REQ_PATH="$(printf '%s\n' "${LAST_LINE}" | jq -r '.url // ""')"
REQ_PRODUCT="$(printf '%s\n' "${LAST_LINE}" | jq -r '.payload.messaging_product // ""')"
REQ_TO="$(printf '%s\n' "${LAST_LINE}" | jq -r '.payload.to // ""')"
if [[ "${AUTH_HEADER}" != "Bearer ${WHATSAPP_API_KEY}" ]]; then
  echo "FAIL verify provider auth: expected Bearer token"
  echo "${LAST_LINE}"
  exit 1
fi
if [[ "${REQ_PATH}" != "/v22.0/${WHATSAPP_PHONE_NUMBER_ID}/messages" ]]; then
  echo "FAIL verify provider path: expected /v22.0/${WHATSAPP_PHONE_NUMBER_ID}/messages got ${REQ_PATH}"
  echo "${LAST_LINE}"
  exit 1
fi
if [[ "${REQ_PRODUCT}" != "whatsapp" || "${REQ_TO}" != "+62811111111" ]]; then
  echo "FAIL verify provider payload: expected whatsapp product and receiver"
  echo "${LAST_LINE}"
  exit 1
fi

echo "PASS: wallet job alert dispatch whatsapp meta regression complete"
