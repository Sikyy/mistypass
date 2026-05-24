#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18191}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-tenant.admin@sudirman.co}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-$(printf '%s%s' admin 123)}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
DOOR_ID="${DOOR_ID:-door_jkt_006}"
BOOTSTRAP_TOKEN="${GATEWAY_BOOTSTRAP_TOKEN:-mistypass-dev-bootstrap-local-only-20260424}"
API_LOG="${API_LOG:-/tmp/mp_mobile_admin_events_incidents_api.log}"
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

function require_equals() {
  local got="$1"
  local expected="$2"
  local step="$3"
  if [[ "${got}" != "${expected}" ]]; then
    echo "FAIL ${step}: expected ${expected}, got ${got}"
    exit 1
  fi
}

function require_at_least() {
  local got="$1"
  local expected="$2"
  local step="$3"
  if (( got < expected )); then
    echo "FAIL ${step}: expected >= ${expected}, got ${got}"
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

function bootstrap_with_token() {
  local method="$1"
  local endpoint_path="$2"
  local payload="$3"
  curl -sS -X "${method}" "${API_BASE_URL}${endpoint_path}" \
    -H "X-Device-Token: ${DEVICE_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "${payload}" \
    -w $'\n%{http_code}'
}

function ensure_api_running() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    echo "api: already running"
    return
  fi

  echo "api: starting local mobile admin events/incidents server"
  (
    cd api
    PORT="${API_PORT}" \
      ENABLE_DEMO_USERS=true \
      DISABLE_LOGIN_RATE_LIMIT=true \
      GATEWAY_BOOTSTRAP_TOKEN="${BOOTSTRAP_TOKEN}" \
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
    if command -v lsof >/dev/null 2>&1; then
      local pids
      pids="$(lsof -ti tcp:"${API_PORT}" 2>/dev/null || true)"
      for pid in ${(f)pids}; do
        [[ -n "${pid}" ]] && kill "${pid}" >/dev/null 2>&1 || true
      done
    fi
  fi
}

trap cleanup EXIT

RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"
GW_SERIAL="MP-GW-MOB-ADMIN-${RUN_TAG}"
EVENT_ID_1="gwea-mobile-admin-a-${RUN_TAG}"
EVENT_ID_2="gwea-mobile-admin-b-${RUN_TAG}"
ACTOR_ID="qa.mobile.admin.${RUN_TAG}"
OCCURRED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
INCIDENT_ID="inc_deny_${DOOR_ID}"

ensure_api_running

echo "== login as tenant admin =="
LOGIN_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

echo "== import gateway serial and bootstrap register =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg sn "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$sn,product_type:"gateway",batch_code:"qa-mobile-admin-events",source:"factory"}]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import gateway serial inventory"

BOOTSTRAP_REGISTER_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
BOOTSTRAP_REGISTER_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/gateway/register" \
  -H "X-Bootstrap-Token: ${BOOTSTRAP_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${BOOTSTRAP_REGISTER_PAYLOAD}" \
  -w $'\n%{http_code}')"
split_response "${BOOTSTRAP_REGISTER_RAW}"
require_http_code "201" "gateway bootstrap register"
GW_ID="$(echo "${HTTP_BODY}" | jq -r '.gateway_id')"
DEVICE_TOKEN="$(echo "${HTTP_BODY}" | jq -r '.device_token')"
require_non_empty "${GW_ID}" "gateway_id"
require_non_empty "${DEVICE_TOKEN}" "device_token"

echo "== ingest related denied access events =="
ACCESS_PAYLOAD_1="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${EVENT_ID_1}" \
  --arg building "${BUILDING_ID}" \
  --arg door "${DOOR_ID}" \
  --arg actor "${ACTOR_ID}" \
  --arg occurred "${OCCURRED_AT}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:("rq-"+$eid),idempotency_key:("idem-"+$eid),building_id:$building,area_id:"area_demo_001",type:"access_denied",actor:$actor,door_id:$door,result:"denied",occurred_at:$occurred}')"
ACCESS_RAW_1="$(bootstrap_with_token POST "/api/v1/gateway/events/access" "${ACCESS_PAYLOAD_1}")"
split_response "${ACCESS_RAW_1}"
require_http_code "202" "ingest denied access event #1"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.deduplicated')" "false" "ingest denied access event #1 deduplicated"

ACCESS_PAYLOAD_2="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg eid "${EVENT_ID_2}" \
  --arg building "${BUILDING_ID}" \
  --arg door "${DOOR_ID}" \
  --arg actor "${ACTOR_ID}" \
  --arg occurred "${OCCURRED_AT}" \
  '{gateway_id:$gid,tenant_id:$tenant,event_id:$eid,request_id:("rq-"+$eid),idempotency_key:("idem-"+$eid),building_id:$building,area_id:"area_demo_001",type:"access_denied",actor:$actor,door_id:$door,result:"denied",occurred_at:$occurred}')"
ACCESS_RAW_2="$(bootstrap_with_token POST "/api/v1/gateway/events/access" "${ACCESS_PAYLOAD_2}")"
split_response "${ACCESS_RAW_2}"
require_http_code "202" "ingest denied access event #2"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.deduplicated')" "false" "ingest denied access event #2 deduplicated"

echo "== mobile admin event list/detail/related =="
EVENT_LIST_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/events?object_id=${DOOR_ID}&limit=25")"
split_response "${EVENT_LIST_RAW}"
require_http_code "200" "mobile admin event list"
EVENT_LIST_HITS="$(echo "${HTTP_BODY}" | jq -r --arg a "${EVENT_ID_1}" --arg b "${EVENT_ID_2}" '[.items[] | select(.id == $a or .id == $b)] | length')"
require_equals "${EVENT_LIST_HITS}" "2" "mobile admin event list seeded hits"

EVENT_DETAIL_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/events/${EVENT_ID_1}")"
split_response "${EVENT_DETAIL_RAW}"
require_http_code "200" "mobile admin event detail"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.id')" "${EVENT_ID_1}" "event detail id"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.object_type')" "door" "event detail object_type"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.object_id')" "${DOOR_ID}" "event detail object_id"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.result')" "denied" "event detail result"

RELATED_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/events/${EVENT_ID_1}/related")"
split_response "${RELATED_RAW}"
require_http_code "200" "mobile admin related events"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.event_id')" "${EVENT_ID_1}" "related source event"
RELATED_HITS="$(echo "${HTTP_BODY}" | jq -r --arg related "${EVENT_ID_2}" '[.items[] | select(.id == $related and (.relation == "same_actor_same_door" or .relation == "same_actor" or .relation == "same_door"))] | length')"
require_at_least "${RELATED_HITS}" 1 "related events include seeded companion"

echo "== mobile admin incident list/detail/occurrences =="
INCIDENT_LIST_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/incidents?type=access_denied&subject_type=door&limit=25")"
split_response "${INCIDENT_LIST_RAW}"
require_http_code "200" "mobile admin incident list"
INCIDENT_LIST_HITS="$(echo "${HTTP_BODY}" | jq -r --arg id "${INCIDENT_ID}" --arg door "${DOOR_ID}" '[.items[] | select(.id == $id and .type == "access_denied" and .subject_id == $door)] | length')"
require_at_least "${INCIDENT_LIST_HITS}" 1 "incident list includes denied door incident"

INCIDENT_DETAIL_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/incidents/${INCIDENT_ID}")"
split_response "${INCIDENT_DETAIL_RAW}"
require_http_code "200" "mobile admin incident detail"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.id')" "${INCIDENT_ID}" "incident detail id"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.type')" "access_denied" "incident detail type"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.subject_id')" "${DOOR_ID}" "incident detail subject_id"
INCIDENT_EVENT_HITS="$(echo "${HTTP_BODY}" | jq -r --arg a "${EVENT_ID_1}" --arg b "${EVENT_ID_2}" '[.events[] | select(.event_id == $a or .event_id == $b)] | length')"
require_equals "${INCIDENT_EVENT_HITS}" "2" "incident detail seeded event hits"
INCIDENT_COUNT="$(echo "${HTTP_BODY}" | jq -r '.count')"
require_at_least "${INCIDENT_COUNT}" 2 "incident detail count"

OCCURRENCES_RAW="$(api_with_auth GET "/api/v1/app/places/${BUILDING_ID}/incidents/${INCIDENT_ID}/occurrences")"
split_response "${OCCURRENCES_RAW}"
require_http_code "200" "mobile admin incident occurrences"
require_equals "$(echo "${HTTP_BODY}" | jq -r '.incident_id')" "${INCIDENT_ID}" "occurrences incident id"
OCCURRENCE_HITS="$(echo "${HTTP_BODY}" | jq -r --arg a "${EVENT_ID_1}" --arg b "${EVENT_ID_2}" '[.items[] | select(.event_id == $a or .event_id == $b)] | length')"
require_equals "${OCCURRENCE_HITS}" "2" "occurrences seeded event hits"
OCCURRENCE_TOTAL="$(echo "${HTTP_BODY}" | jq -r '.pagination.total')"
require_at_least "${OCCURRENCE_TOTAL}" 2 "occurrences total"

echo "PASS: mobile admin events/incidents regression complete"
