#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18189}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
RESEND_PORT="${RESEND_PORT:-19094}"
RESEND_BASE_URL="${RESEND_BASE_URL:-http://127.0.0.1:${RESEND_PORT}}"
RESEND_ENDPOINT="${RESEND_ENDPOINT:-${RESEND_BASE_URL}/emails}"
RESEND_API_KEY="${RESEND_API_KEY:-$(printf '%s-%s-%s' resend dev token)}"
REPORT_EMAIL_FROM="${REPORT_EMAIL_FROM:-reports@mistypass.local}"
GOTENBERG_PORT="${GOTENBERG_PORT:-19095}"
GOTENBERG_BASE_URL="${GOTENBERG_BASE_URL:-http://127.0.0.1:${GOTENBERG_PORT}}"
RESEND_LOG="${RESEND_LOG:-/tmp/mp_report_schedule_resend.jsonl}"
RESEND_SERVER_LOG="${RESEND_SERVER_LOG:-/tmp/mp_report_schedule_resend.log}"
GOTENBERG_LOG="${GOTENBERG_LOG:-/tmp/mp_report_schedule_gotenberg.jsonl}"
GOTENBERG_SERVER_LOG="${GOTENBERG_SERVER_LOG:-/tmp/mp_report_schedule_gotenberg.log}"
API_LOG="${API_LOG:-/tmp/mp_report_schedule_resend_api.log}"
API_PID=""
RESEND_PID=""
GOTENBERG_PID=""

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

function kill_port() {
  local port="$1"
  if ! command -v lsof >/dev/null 2>&1; then
    return
  fi
  local pids
  pids="$(lsof -ti tcp:"${port}" 2>/dev/null || true)"
  for pid in ${(f)pids}; do
    [[ -n "${pid}" ]] && kill "${pid}" >/dev/null 2>&1 || true
  done
}

function ensure_resend_running() {
  if curl -sS "${RESEND_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "resend mock: already running"
    : >"${RESEND_LOG}"
    return
  fi

  : >"${RESEND_LOG}"
  echo "resend mock: starting local server"
  RESEND_PORT="${RESEND_PORT}" RESEND_LOG="${RESEND_LOG}" node -e '
const fs = require("fs")
const http = require("http")
const port = Number(process.env.RESEND_PORT || "19094")
const logFile = process.env.RESEND_LOG
let sequence = 0

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
    sequence += 1
    const record = {
      method: req.method,
      url: req.url,
      sequence,
      headers: req.headers,
      payload,
      at: new Date().toISOString(),
    }
    fs.appendFileSync(logFile, JSON.stringify(record) + "\n")
    res.statusCode = 202
    res.setHeader("content-type", "application/json")
    res.end(JSON.stringify({ id: `email_report_mock_${sequence}`, status: "queued" }))
  })
})

server.listen(port, "127.0.0.1", () => {
  console.log("report resend mock listening on", port)
})

for (const sig of ["SIGTERM", "SIGINT"]) {
  process.on(sig, () => server.close(() => process.exit(0)))
}
' >"${RESEND_SERVER_LOG}" 2>&1 &
  RESEND_PID="$!"

  local i
  for i in {1..40}; do
    if curl -sS "${RESEND_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "resend mock: started"
      return
    fi
    sleep 0.25
  done

  echo "FAIL resend startup: healthz not ready"
  if [[ -f "${RESEND_SERVER_LOG}" ]]; then
    tail -n 80 "${RESEND_SERVER_LOG}"
  fi
  exit 1
}

function ensure_gotenberg_running() {
  if curl -sS "${GOTENBERG_BASE_URL}/health" >/dev/null 2>&1; then
    echo "gotenberg mock: already running"
    : >"${GOTENBERG_LOG}"
    return
  fi

  : >"${GOTENBERG_LOG}"
  echo "gotenberg mock: starting local server"
  GOTENBERG_PORT="${GOTENBERG_PORT}" GOTENBERG_LOG="${GOTENBERG_LOG}" node -e '
const fs = require("fs")
const http = require("http")
const port = Number(process.env.GOTENBERG_PORT || "19095")
const logFile = process.env.GOTENBERG_LOG
let sequence = 0

const server = http.createServer((req, res) => {
  if (req.method === "GET" && req.url === "/health") {
    res.statusCode = 200
    res.end("ok")
    return
  }

  let size = 0
  req.on("data", (chunk) => {
    size += chunk.length
  })
  req.on("end", () => {
    sequence += 1
    fs.appendFileSync(logFile, JSON.stringify({
      method: req.method,
      url: req.url,
      sequence,
      content_type: req.headers["content-type"] || "",
      body_bytes: size,
      at: new Date().toISOString(),
    }) + "\n")
    if (req.method !== "POST" || req.url !== "/forms/chromium/convert/html") {
      res.statusCode = 404
      res.end("not found")
      return
    }
    const pdf = Buffer.from(`%PDF-1.4\n% MistyPass report schedule smoke ${sequence}\n%%EOF\n`, "utf8")
    res.statusCode = 200
    res.setHeader("content-type", "application/pdf")
    res.end(pdf)
  })
})

server.listen(port, "127.0.0.1", () => {
  console.log("gotenberg mock listening on", port)
})

for (const sig of ["SIGTERM", "SIGINT"]) {
  process.on(sig, () => server.close(() => process.exit(0)))
}
' >"${GOTENBERG_SERVER_LOG}" 2>&1 &
  GOTENBERG_PID="$!"

  local i
  for i in {1..40}; do
    if curl -sS "${GOTENBERG_BASE_URL}/health" >/dev/null 2>&1; then
      echo "gotenberg mock: started"
      return
    fi
    sleep 0.25
  done

  echo "FAIL gotenberg startup: health not ready"
  if [[ -f "${GOTENBERG_SERVER_LOG}" ]]; then
    tail -n 80 "${GOTENBERG_SERVER_LOG}"
  fi
  exit 1
}

function ensure_api_running() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "api: already running"
    return
  fi

  echo "api: starting local report email server"
  (
    cd api
    PORT="${API_PORT}" \
      ENABLE_DEMO_USERS=true \
      DISABLE_LOGIN_RATE_LIMIT=true \
      REPORT_EMAIL_ENABLED=true \
      USER_INVITATION_RESEND_ENDPOINT="${RESEND_ENDPOINT}" \
      USER_INVITATION_RESEND_API_KEY="${RESEND_API_KEY}" \
      USER_INVITATION_EMAIL_FROM="${REPORT_EMAIL_FROM}" \
      USER_INVITATION_RESEND_TIMEOUT="5s" \
      GOTENBERG_URL="${GOTENBERG_BASE_URL}" \
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
  fi
  if [[ -n "${RESEND_PID}" ]]; then
    kill "${RESEND_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${GOTENBERG_PID}" ]]; then
    kill "${GOTENBERG_PID}" >/dev/null 2>&1 || true
  fi
  kill_port "${API_PORT}"
  kill_port "${RESEND_PORT}"
  kill_port "${GOTENBERG_PORT}"
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
SCHEDULE_NAME="CI Report Resend ${RUN_TAG}"
RECIPIENT_ONE="reports-${RUN_TAG}@example.com"
RECIPIENT_TWO="ops-${RUN_TAG}@example.com"

ensure_resend_running
ensure_gotenberg_running
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

echo "== provider status =="
STATUS_RAW="$(api_with_auth GET "/api/v1/report-schedules/provider-status?tenant_id=${TENANT_ID}")"
split_response "${STATUS_RAW}"
require_http_code "200" "provider status"
STATUS_READY="$(echo "${HTTP_BODY}" | jq -r '.ready')"
STATUS_PROVIDER="$(echo "${HTTP_BODY}" | jq -r '.provider')"
STATUS_ENDPOINT="$(echo "${HTTP_BODY}" | jq -r '.endpoint')"
STATUS_FROM="$(echo "${HTTP_BODY}" | jq -r '.from')"
STATUS_MISSING="$(echo "${HTTP_BODY}" | jq -r '(.missing // []) | length')"
require_equals "${STATUS_READY}" "true" "provider status.ready"
require_equals "${STATUS_PROVIDER}" "resend" "provider status.provider"
require_equals "${STATUS_ENDPOINT}" "${RESEND_ENDPOINT}" "provider status.endpoint"
require_equals "${STATUS_FROM}" "${REPORT_EMAIL_FROM}" "provider status.from"
require_equals "${STATUS_MISSING}" "0" "provider status.missing"

echo "== create report schedule =="
CREATE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg name "${SCHEDULE_NAME}" \
  --arg r1 "${RECIPIENT_ONE}" \
  --arg r2 "${RECIPIENT_TWO}" \
  '{report_schedule:{tenant_id:$tenant,name:$name,report_type:"events",frequency:"daily",recipients:[$r1,$r2],format:"pdf",enabled:true}}')"
CREATE_RAW="$(api_with_auth POST "/api/v1/report-schedules" "${CREATE_PAYLOAD}")"
split_response "${CREATE_RAW}"
require_http_code "201" "create report schedule"
SCHEDULE_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
SCHEDULE_FORMAT="$(echo "${HTTP_BODY}" | jq -r '.format')"
require_non_empty "${SCHEDULE_ID}" "report schedule.id"
require_equals "${SCHEDULE_FORMAT}" "pdf" "report schedule.format"

echo "== send report schedule =="
SEND_RAW="$(api_with_auth POST "/api/v1/report-schedules/${SCHEDULE_ID}/send?tenant_id=${TENANT_ID}")"
split_response "${SEND_RAW}"
require_http_code "200" "send report schedule"
LAST_SENT_AT="$(echo "${HTTP_BODY}" | jq -r '.last_sent_at')"
require_non_empty "${LAST_SENT_AT}" "report schedule.last_sent_at"

echo "== verify gotenberg mock received render =="
GOTENBERG_COUNT="$(jq -sr '[.[] | select(.method == "POST" and .url == "/forms/chromium/convert/html")] | length' "${GOTENBERG_LOG}")"
require_equals "${GOTENBERG_COUNT}" "1" "gotenberg render count"

echo "== verify resend mock received PDF attachment =="
RESEND_COUNT="$(jq -sr '[.[] | select(.method == "POST" and .url == "/emails")] | length' "${RESEND_LOG}")"
require_equals "${RESEND_COUNT}" "1" "resend send count"

jq -es \
  --arg from "${REPORT_EMAIL_FROM}" \
  --arg r1 "${RECIPIENT_ONE}" \
  --arg r2 "${RECIPIENT_TWO}" \
  --arg schedule_id "${SCHEDULE_ID}" \
  --arg schedule_name "${SCHEDULE_NAME}" \
  --arg api_key "${RESEND_API_KEY}" \
  '
  .[0] as $record
  | ($record.headers.authorization == ("Bearer " + $api_key))
  and (($record.headers["idempotency-key"] // "") | startswith("report_schedule:" + $schedule_id + ":"))
  and ($record.payload.from == $from)
  and ($record.payload.to | index($r1) != null)
  and ($record.payload.to | index($r2) != null)
  and ($record.payload.subject | contains($schedule_name))
  and ($record.payload.metadata.report_schedule_id == $schedule_id)
  and ($record.payload.metadata.report_type == "events")
  and (($record.payload.attachments // []) | length == 1)
  and ($record.payload.attachments[0].filename | test("^events_.*\\.pdf$"))
  and ($record.payload.attachments[0].content | startswith("JVBERi0x"))
  ' "${RESEND_LOG}" >/dev/null

echo "== verify schedule persisted with last_sent_at =="
GET_RAW="$(api_with_auth GET "/api/v1/report-schedules/${SCHEDULE_ID}?tenant_id=${TENANT_ID}")"
split_response "${GET_RAW}"
require_http_code "200" "get report schedule"
GET_LAST_SENT_AT="$(echo "${HTTP_BODY}" | jq -r '.last_sent_at')"
require_equals "${GET_LAST_SENT_AT}" "${LAST_SENT_AT}" "persisted last_sent_at"

echo "== verify audit log =="
AUDIT_RAW="$(api_with_auth GET "/api/v1/audit-logs?tenant_id=${TENANT_ID}&action=report_schedule_sent&source=report_schedule&limit=10")"
split_response "${AUDIT_RAW}"
require_http_code "200" "report schedule audit log"
jq -e \
  --arg schedule_id "${SCHEDULE_ID}" \
  '
  (.items // [])
  | any(
      .action == "report_schedule_sent"
      and .source == "report_schedule"
      and (.target // "" | contains("schedule_id=" + $schedule_id))
      and (.target // "" | contains("provider=resend"))
      and (.target // "" | contains("provider_delivery_id=email_report_mock_1"))
    )
  ' <<<"${HTTP_BODY}" >/dev/null

echo "== delete report schedule =="
DELETE_RAW="$(api_with_auth DELETE "/api/v1/report-schedules/${SCHEDULE_ID}?tenant_id=${TENANT_ID}")"
split_response "${DELETE_RAW}"
require_http_code "204" "delete report schedule"

echo "PASS: report schedule resend regression complete"
