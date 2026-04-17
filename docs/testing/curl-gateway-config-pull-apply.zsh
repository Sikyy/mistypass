#!/usr/bin/env zsh
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
TENANT_ID="${TENANT_ID:-tenant_demo_jakarta}"
BUILDING_ID="${BUILDING_ID:-building_demo_001}"
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

  echo "api: starting local server"
  (
    cd api
    GOCACHE=/tmp/go-build go run ./cmd/api >/tmp/mp_gateway_config_pull_apply.log 2>&1
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
  if [[ -f /tmp/mp_gateway_config_pull_apply.log ]]; then
    tail -n 80 /tmp/mp_gateway_config_pull_apply.log
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
GW_SERIAL="MP-GW-CFG-${RUN_TAG}"
TARGET_VERSION="cfg-${RUN_TAG}"
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
echo "login: ok"

echo "== import gateway serial inventory =="
IMPORT_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg gw "${GW_SERIAL}" \
  '{tenant_id:$tenant,items:[{serial_number:$gw,product_type:"gateway",batch_code:"qa-gw-config",source:"factory"}]}')"
IMPORT_RAW="$(api_with_auth POST "/api/v1/gateways/serial-inventory/import" "${IMPORT_PAYLOAD}")"
split_response "${IMPORT_RAW}"
require_http_code "201" "import gateway serial inventory"
echo "inventory: imported ${GW_SERIAL}"

echo "== bootstrap register gateway =="
BOOTSTRAP_REGISTER_PAYLOAD="$(jq -nc \
  --arg sn "${GW_SERIAL}" \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  '{serial_number:$sn,tenant_id:$tenant,building_id:$building,device_capacity:4}')"
BOOTSTRAP_REGISTER_RAW="$(curl -sS -X POST "${API_BASE_URL}/api/v1/gateway/register" \
  -H "Content-Type: application/json" \
  -d "${BOOTSTRAP_REGISTER_PAYLOAD}" \
  -w $'\n%{http_code}')"
split_response "${BOOTSTRAP_REGISTER_RAW}"
require_http_code "201" "gateway bootstrap register"
GW_ID="$(echo "${HTTP_BODY}" | jq -r '.gateway_id')"
DEVICE_TOKEN="$(echo "${HTTP_BODY}" | jq -r '.device_token')"
require_non_empty "${GW_ID}" "gateway_id"
require_non_empty "${DEVICE_TOKEN}" "device_token"
echo "bootstrap: gateway_id=${GW_ID}"

echo "== bind one door for authz cache scope =="
BIND_PAYLOAD="$(jq -nc --arg did "door_jkt_001" '{door_id:$did}')"
BIND_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/bind-door?tenant_id=${TENANT_ID}" "${BIND_PAYLOAD}")"
split_response "${BIND_RAW}"
require_http_code "200" "bind door"
echo "bind-door: door_jkt_001"

echo "== publish config desired version =="
PUBLISH_PAYLOAD="$(jq -nc --arg ver "${TARGET_VERSION}" '{version:$ver}')"
PUBLISH_RAW="$(api_with_auth POST "/api/v1/gateways/${GW_ID}/config/publish?tenant_id=${TENANT_ID}" "${PUBLISH_PAYLOAD}")"
split_response "${PUBLISH_RAW}"
require_http_code "202" "publish config"
PUBLISH_STATUS="$(echo "${HTTP_BODY}" | jq -r '.status')"
if [[ "${PUBLISH_STATUS}" != "queued" ]]; then
  echo "FAIL publish config: unexpected status ${PUBLISH_STATUS}"
  exit 1
fi
echo "publish: status=${PUBLISH_STATUS} version=${TARGET_VERSION}"

echo "== bootstrap pull config (expect should_apply=true) =="
PULL_PAYLOAD_1="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg cur "cfg-old" \
  '{gateway_id:$gid,tenant_id:$tenant,current_version:$cur}')"
PULL_RAW_1="$(bootstrap_with_token POST "/api/v1/gateway/config/pull" "${PULL_PAYLOAD_1}")"
split_response "${PULL_RAW_1}"
require_http_code "200" "gateway config pull #1"
PULL_DESIRED_1="$(echo "${HTTP_BODY}" | jq -r '.desired_version')"
PULL_APPLY_1="$(echo "${HTTP_BODY}" | jq -r '.should_apply')"
PULL_BOUND_COUNT_1="$(echo "${HTTP_BODY}" | jq -r '.bound_door_ids | length')"
AUTHZ_VERSION_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.version')"
AUTHZ_TTL_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.ttl_seconds')"
AUTHZ_EXPIRES_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.expires_at')"
AUTHZ_STALE_UNTIL_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.policy.stale_until')"
AUTHZ_MAX_STALE_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.policy.max_stale_seconds')"
AUTHZ_FALLBACK_MODE_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.policy.fallback_mode')"
AUTHZ_NO_CACHE_MODE_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.policy.no_cache_behavior')"
AUTHZ_CODE_FRESH_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.status_codes.fresh')"
AUTHZ_STATUS_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.status')"
AUTHZ_SCOPE_DOORS_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.scope.door_ids | length')"
AUTHZ_COUNTS_DOORS_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.counts.doors')"
AUTHZ_DOORS_LEN_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.doors | length')"
AUTHZ_COUNTS_POLICIES_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.counts.policies')"
AUTHZ_POLICIES_LEN_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.policies | length')"
AUTHZ_COUNTS_USERS_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.counts.users')"
AUTHZ_USERS_LEN_1="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.users | length')"
if [[ "${PULL_DESIRED_1}" != "${TARGET_VERSION}" || "${PULL_APPLY_1}" != "true" ]]; then
  echo "FAIL config pull #1: desired=${PULL_DESIRED_1} should_apply=${PULL_APPLY_1}"
  exit 1
fi
if [[ "${PULL_BOUND_COUNT_1}" -lt 1 || "${AUTHZ_SCOPE_DOORS_1}" -lt 1 ]]; then
  echo "FAIL config pull #1 authz scope: bound=${PULL_BOUND_COUNT_1} scope_doors=${AUTHZ_SCOPE_DOORS_1}"
  exit 1
fi
if [[ -z "${AUTHZ_VERSION_1}" || "${AUTHZ_VERSION_1}" == "null" || "${AUTHZ_VERSION_1}" == "authz-unavailable" ]]; then
  echo "FAIL config pull #1 authz version: ${AUTHZ_VERSION_1}"
  exit 1
fi
if [[ "${AUTHZ_TTL_1}" -le 0 ]]; then
  echo "FAIL config pull #1 authz ttl: ${AUTHZ_TTL_1}"
  exit 1
fi
if [[ -z "${AUTHZ_EXPIRES_1}" || "${AUTHZ_EXPIRES_1}" == "null" ]]; then
  echo "FAIL config pull #1 authz expires_at: ${AUTHZ_EXPIRES_1}"
  exit 1
fi
if [[ -z "${AUTHZ_STALE_UNTIL_1}" || "${AUTHZ_STALE_UNTIL_1}" == "null" || "${AUTHZ_MAX_STALE_1}" -le "${AUTHZ_TTL_1}" ]]; then
  echo "FAIL config pull #1 authz stale policy: stale_until=${AUTHZ_STALE_UNTIL_1} max_stale=${AUTHZ_MAX_STALE_1} ttl=${AUTHZ_TTL_1}"
  exit 1
fi
if [[ "${AUTHZ_FALLBACK_MODE_1}" != "use_last_acknowledged" || "${AUTHZ_NO_CACHE_MODE_1}" != "deny_all" || "${AUTHZ_CODE_FRESH_1}" != "AUTHZ_CACHE_FRESH" ]]; then
  echo "FAIL config pull #1 authz policy/status-code mismatch: fallback=${AUTHZ_FALLBACK_MODE_1} no_cache=${AUTHZ_NO_CACHE_MODE_1} code_fresh=${AUTHZ_CODE_FRESH_1}"
  exit 1
fi
if [[ "${AUTHZ_STATUS_1}" != "AUTHZ_CACHE_MISSING" ]]; then
  echo "FAIL config pull #1 authz status mismatch: status=${AUTHZ_STATUS_1}"
  exit 1
fi
if [[ "${AUTHZ_COUNTS_DOORS_1}" != "${AUTHZ_DOORS_LEN_1}" || "${AUTHZ_COUNTS_POLICIES_1}" != "${AUTHZ_POLICIES_LEN_1}" || "${AUTHZ_COUNTS_USERS_1}" != "${AUTHZ_USERS_LEN_1}" ]]; then
  echo "FAIL config pull #1 authz counts mismatch: doors=${AUTHZ_COUNTS_DOORS_1}/${AUTHZ_DOORS_LEN_1} policies=${AUTHZ_COUNTS_POLICIES_1}/${AUTHZ_POLICIES_LEN_1} users=${AUTHZ_COUNTS_USERS_1}/${AUTHZ_USERS_LEN_1}"
  exit 1
fi
echo "pull#1: desired=${PULL_DESIRED_1} should_apply=${PULL_APPLY_1}"

echo "== bootstrap config applied ack =="
APPLIED_PAYLOAD="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg ver "${TARGET_VERSION}" \
  --arg authz_ver "${AUTHZ_VERSION_1}" \
  '{gateway_id:$gid,tenant_id:$tenant,version:$ver,authz_cache_version:$authz_ver}')"
APPLIED_RAW="$(bootstrap_with_token POST "/api/v1/gateway/config/applied" "${APPLIED_PAYLOAD}")"
split_response "${APPLIED_RAW}"
require_http_code "200" "gateway config applied"
IN_SYNC="$(echo "${HTTP_BODY}" | jq -r '.in_sync')"
APPLIED_AUTHZ_MATCH="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.version_match')"
APPLIED_AUTHZ_EXPECTED="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.version_expected')"
APPLIED_AUTHZ_EXPIRES="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.expires_at')"
APPLIED_FALLBACK_MODE="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.policy.fallback_mode')"
APPLIED_AUTHZ_STATUS="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.status')"
if [[ "${IN_SYNC}" != "true" || "${APPLIED_AUTHZ_MATCH}" != "true" || "${APPLIED_AUTHZ_EXPECTED}" != "${AUTHZ_VERSION_1}" || -z "${APPLIED_AUTHZ_EXPIRES}" || "${APPLIED_AUTHZ_EXPIRES}" == "null" || "${APPLIED_FALLBACK_MODE}" != "use_last_acknowledged" || "${APPLIED_AUTHZ_STATUS}" != "AUTHZ_CACHE_FRESH" ]]; then
  echo "FAIL config applied: in_sync=${IN_SYNC} authz_match=${APPLIED_AUTHZ_MATCH} authz_expected=${APPLIED_AUTHZ_EXPECTED} authz_reported=${AUTHZ_VERSION_1} authz_expires=${APPLIED_AUTHZ_EXPIRES} fallback=${APPLIED_FALLBACK_MODE} authz_status=${APPLIED_AUTHZ_STATUS}"
  exit 1
fi
echo "applied: in_sync=${IN_SYNC}"

echo "== bootstrap pull config again (expect should_apply=false) =="
PULL_PAYLOAD_2="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg cur "${TARGET_VERSION}" \
  --arg authz_ver "${AUTHZ_VERSION_1}" \
  '{gateway_id:$gid,tenant_id:$tenant,current_version:$cur,authz_cache_version:$authz_ver}')"
PULL_RAW_2="$(bootstrap_with_token POST "/api/v1/gateway/config/pull" "${PULL_PAYLOAD_2}")"
split_response "${PULL_RAW_2}"
require_http_code "200" "gateway config pull #2"
PULL_APPLY_2="$(echo "${HTTP_BODY}" | jq -r '.should_apply')"
PULL_APPLIED_2="$(echo "${HTTP_BODY}" | jq -r '.applied_version')"
AUTHZ_VERSION_2="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.version')"
AUTHZ_ROLLBACK_2="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.policy.rollback_version')"
AUTHZ_STATUS_2="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.status')"
if [[ "${PULL_APPLY_2}" != "false" || "${PULL_APPLIED_2}" != "${TARGET_VERSION}" ]]; then
  echo "FAIL config pull #2: should_apply=${PULL_APPLY_2} applied=${PULL_APPLIED_2}"
  exit 1
fi
if [[ "${AUTHZ_VERSION_2}" != "${AUTHZ_VERSION_1}" ]]; then
  echo "FAIL config pull #2 authz version drift: first=${AUTHZ_VERSION_1} second=${AUTHZ_VERSION_2}"
  exit 1
fi
if [[ "${AUTHZ_ROLLBACK_2}" != "${AUTHZ_VERSION_1}" ]]; then
  echo "FAIL config pull #2 authz rollback_version mismatch: rollback=${AUTHZ_ROLLBACK_2} expected=${AUTHZ_VERSION_1}"
  exit 1
fi
if [[ "${AUTHZ_STATUS_2}" != "AUTHZ_CACHE_FRESH" ]]; then
  echo "FAIL config pull #2 authz status mismatch: status=${AUTHZ_STATUS_2}"
  exit 1
fi
echo "pull#2: should_apply=${PULL_APPLY_2} applied=${PULL_APPLIED_2}"

echo "== bootstrap pull config with drifted local authz version =="
PULL_PAYLOAD_3="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg cur "${TARGET_VERSION}" \
  '{gateway_id:$gid,tenant_id:$tenant,current_version:$cur,authz_cache_version:"authz-local-drift"}')"
PULL_RAW_3="$(bootstrap_with_token POST "/api/v1/gateway/config/pull" "${PULL_PAYLOAD_3}")"
split_response "${PULL_RAW_3}"
require_http_code "200" "gateway config pull #3 drift"
AUTHZ_STATUS_3="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.status')"
if [[ "${AUTHZ_STATUS_3}" != "AUTHZ_CACHE_DRIFT" ]]; then
  echo "FAIL config pull #3 authz status mismatch: status=${AUTHZ_STATUS_3}"
  exit 1
fi
echo "pull#3: authz_status=${AUTHZ_STATUS_3}"

echo "== mutate in-scope authz data to force new authz version =="
POLICY_CREATE_PAYLOAD="$(jq -nc \
  --arg tenant "${TENANT_ID}" \
  --arg building "${BUILDING_ID}" \
  --arg door "door_jkt_001" \
  --arg name "cfg-stale-${RUN_TAG}" \
  '{tenant_id:$tenant,name:$name,scope_type:"door",building_id:$building,door_id:$door,schedule:"always",members:1,status:"active"}')"
POLICY_CREATE_RAW="$(api_with_auth POST "/api/v1/access-policies" "${POLICY_CREATE_PAYLOAD}")"
split_response "${POLICY_CREATE_RAW}"
require_http_code "201" "create in-scope policy for stale authz"
POLICY_CREATED_ID="$(echo "${HTTP_BODY}" | jq -r '.id')"
require_non_empty "${POLICY_CREATED_ID}" "created policy id"
echo "policy: created ${POLICY_CREATED_ID}"

echo "== bootstrap pull config with rollback version (expect stale) =="
PULL_PAYLOAD_4="$(jq -nc \
  --arg gid "${GW_ID}" \
  --arg tenant "${TENANT_ID}" \
  --arg cur "${TARGET_VERSION}" \
  --arg authz_ver "${AUTHZ_VERSION_1}" \
  '{gateway_id:$gid,tenant_id:$tenant,current_version:$cur,authz_cache_version:$authz_ver}')"
PULL_RAW_4="$(bootstrap_with_token POST "/api/v1/gateway/config/pull" "${PULL_PAYLOAD_4}")"
split_response "${PULL_RAW_4}"
require_http_code "200" "gateway config pull #4 stale"
AUTHZ_STATUS_4="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.status')"
AUTHZ_VERSION_4="$(echo "${HTTP_BODY}" | jq -r '.authz_cache.version')"
if [[ "${AUTHZ_STATUS_4}" != "AUTHZ_CACHE_STALE" ]]; then
  echo "FAIL config pull #4 authz status mismatch: status=${AUTHZ_STATUS_4}"
  exit 1
fi
if [[ "${AUTHZ_VERSION_4}" == "${AUTHZ_VERSION_1}" ]]; then
  echo "FAIL config pull #4 expected authz version to change after policy mutation: version_1=${AUTHZ_VERSION_1} version_4=${AUTHZ_VERSION_4}"
  exit 1
fi
echo "pull#4: authz_status=${AUTHZ_STATUS_4}"

echo "PASS: gateway config pull/apply + authz_cache bootstrap regression complete"
