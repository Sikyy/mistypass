#!/usr/bin/env zsh
set -euo pipefail

API_PORT="${API_PORT:-18085}"
API_BASE_URL="${API_BASE_URL:-http://localhost:${API_PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://siky@localhost:5432/postgres?sslmode=disable}"
LOGIN_EMAIL="${LOGIN_EMAIL:-superadmin@mistypass.local}"
LOGIN_PASSWORD="${LOGIN_PASSWORD:-admin123}"
SERVER_LOG="${SERVER_LOG:-/tmp/mp_pg_replay_curve_api.log}"
RESULT_CSV="${RESULT_CSV:-}"
API_STARTUP_ATTEMPTS="${API_STARTUP_ATTEMPTS:-240}"
API_STARTUP_SLEEP_SECONDS="${API_STARTUP_SLEEP_SECONDS:-0.5}"
CATCHUP_RETRY_ATTEMPTS="${CATCHUP_RETRY_ATTEMPTS:-5}"
CATCHUP_RETRY_SLEEP_SECONDS="${CATCHUP_RETRY_SLEEP_SECONDS:-0.75}"

LEVEL_TENANT_WRITES="${LEVEL_TENANT_WRITES:-10,20,40}"
LEVEL_BUILDINGS_PER_TENANT="${LEVEL_BUILDINGS_PER_TENANT:-1,1,1}"
LEVEL_CONCURRENT_NOOP="${LEVEL_CONCURRENT_NOOP:-4,6,8}"
LEVEL_REPLAY_LIMIT="${LEVEL_REPLAY_LIMIT:-600,1200,2400}"
LEVEL_MIN_OPS="${LEVEL_MIN_OPS:-15,12,8}"
LEVEL_P95_MS="${LEVEL_P95_MS:-4000,7000,10000}"
LEVEL_MAX_MS="${LEVEL_MAX_MS:-7000,11000,15000}"

STATE_KEYS="${STATE_KEYS:-module_tenant,module_space}"
API_PID=""
TMP_RESULTS_FILE=""
RUN_TAG="$(date +%Y%m%d%H%M%S)-$RANDOM"

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

function now_ms() {
  perl -MTime::HiRes=time -e 'printf("%.0f\n", time()*1000)'
}

function elapsed_ms() {
  local start_ms="$1"
  local end_ms
  end_ms="$(now_ms)"
  echo $((end_ms - start_ms))
}

function stop_port_processes() {
  local pids
  pids="$(lsof -ti "tcp:${API_PORT}" 2>/dev/null || true)"
  if [[ -z "${pids}" ]]; then
    return
  fi
  echo "${pids}" | xargs kill >/dev/null 2>&1 || true
  sleep 0.3
  pids="$(lsof -ti "tcp:${API_PORT}" 2>/dev/null || true)"
  if [[ -n "${pids}" ]]; then
    echo "${pids}" | xargs kill -9 >/dev/null 2>&1 || true
  fi
}

function start_api() {
  if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
    stop_port_processes
    if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "FAIL start_api: ${API_BASE_URL} still has a running service"
      exit 1
    fi
  fi

  (
    cd api
    PORT="${API_PORT}" \
      DATABASE_URL="${DATABASE_URL}" \
      ENABLE_DEMO_USERS="${ENABLE_DEMO_USERS:-true}" \
      DISABLE_LOGIN_RATE_LIMIT="${DISABLE_LOGIN_RATE_LIMIT:-true}" \
      GOCACHE=/tmp/go-build \
      go run ./cmd/api >"${SERVER_LOG}" 2>&1
  ) &
  API_PID="$!"

  local i
  for (( i = 1; i <= API_STARTUP_ATTEMPTS; i++ )); do
    if curl -sS "${API_BASE_URL}/healthz" >/dev/null 2>&1; then
      echo "api: started on ${API_BASE_URL}"
      return
    fi
    sleep "${API_STARTUP_SLEEP_SECONDS}"
  done

  echo "FAIL start_api: healthz not ready after ${API_STARTUP_ATTEMPTS} attempts"
  if [[ -f "${SERVER_LOG}" ]]; then
    tail -n 120 "${SERVER_LOG}"
  fi
  exit 1
}

function stop_api() {
  if [[ -n "${API_PID}" ]]; then
    kill "${API_PID}" >/dev/null 2>&1 || true
    wait "${API_PID}" >/dev/null 2>&1 || true
    API_PID=""
  fi
  stop_port_processes
}

function cleanup() {
  if [[ -n "${TMP_RESULTS_FILE}" ]]; then
    rm -f "${TMP_RESULTS_FILE}" >/dev/null 2>&1 || true
  fi
  stop_api
}

function api_with_auth() {
  local method="$1"
  local endpoint_path="$2"
  local payload="${3:-}"
  if [[ -n "${payload}" ]]; then
    curl -sS --connect-timeout 5 --max-time 40 -X "${method}" "${API_BASE_URL}${endpoint_path}" \
      -H "Authorization: Bearer ${AT}" \
      -H "Content-Type: application/json" \
      -d "${payload}" \
      -w $'\n%{http_code}'
    return
  fi
  curl -sS --connect-timeout 5 --max-time 40 -X "${method}" "${API_BASE_URL}${endpoint_path}" \
    -H "Authorization: Bearer ${AT}" \
    -w $'\n%{http_code}'
}

function csv_compact() {
  local csv="$1"
  echo "${csv//[[:space:]]/}"
}

function fetch_latest_change_id() {
  local state_key="$1"
  local raw
  raw="$(api_with_auth GET "/api/v1/state/change-log?state_key=${state_key}&limit=1")"
  split_response "${raw}"
  require_http_code "200" "list latest change for ${state_key}"
  echo "${HTTP_BODY}" | jq -r 'if (.items | length) > 0 then .items[0].id else 0 end'
}

function set_checkpoint() {
  local state_key="$1"
  local last_change_id="$2"
  psql "${DATABASE_URL}" -Atc "insert into mistypass_change_replay_checkpoints (state_key, last_change_id, updated_at) values ('${state_key}', ${last_change_id}, now()) on conflict (state_key) do update set last_change_id=${last_change_id}, updated_at=now();" >/dev/null
}

function benchmark_state_key() {
  local level_name="$1"
  local state_key="$2"
  local from_change_id="$3"
  local latest_change_id="$4"
  local replay_limit="$5"
  local concurrent_noop="$6"
  local min_ops="$7"
  local p95_threshold_ms="$8"
  local max_threshold_ms="$9"

  local expected_delta=$((latest_change_id - from_change_id))
  if [[ "${expected_delta}" -le 0 ]]; then
    echo "FAIL ${level_name}/${state_key}: expected positive delta, got ${expected_delta}"
    exit 1
  fi

  local effective_limit="${replay_limit}"
  if [[ "${effective_limit}" -lt "${expected_delta}" ]]; then
    effective_limit=$((expected_delta + 20))
  fi

  set_checkpoint "${state_key}" "${from_change_id}"

  local replay_payload
  replay_payload="$(jq -nc --arg sk "${state_key}" --argjson limit "${effective_limit}" '{state_key:$sk,limit:$limit}')"

  local catchup_start_ms catchup_raw catchup_latency_ms catchup_applied catchup_last_change_id catchup_throughput
  local catchup_attempt
  for (( catchup_attempt = 1; catchup_attempt <= CATCHUP_RETRY_ATTEMPTS; catchup_attempt++ )); do
    catchup_start_ms="$(now_ms)"
    catchup_raw="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${replay_payload}")"
    catchup_latency_ms="$(elapsed_ms "${catchup_start_ms}")"
    split_response "${catchup_raw}"
    if [[ "${HTTP_CODE}" != "500" || ("${HTTP_BODY}" != *"bad connection"* && "${HTTP_BODY}" != *"context deadline exceeded"*) || "${catchup_attempt}" -eq "${CATCHUP_RETRY_ATTEMPTS}" ]]; then
      break
    fi
    echo "warn ${level_name}/${state_key}: transient replay error on attempt ${catchup_attempt}/${CATCHUP_RETRY_ATTEMPTS}; retrying"
    sleep "${CATCHUP_RETRY_SLEEP_SECONDS}"
  done
  require_http_code "200" "${level_name}/${state_key} catch-up replay"
  catchup_applied="$(echo "${HTTP_BODY}" | jq -r '.applied')"
  catchup_last_change_id="$(echo "${HTTP_BODY}" | jq -r '.last_change_id')"

  if [[ "${catchup_applied}" -lt 1 ]]; then
    echo "FAIL ${level_name}/${state_key}: catch-up applied should be >=1, got ${catchup_applied}"
    exit 1
  fi
  if [[ "${catchup_last_change_id}" -lt "${latest_change_id}" ]]; then
    echo "FAIL ${level_name}/${state_key}: last_change_id=${catchup_last_change_id} < latest_change_id=${latest_change_id}"
    exit 1
  fi

  catchup_throughput="$(awk -v applied="${catchup_applied}" -v ms="${catchup_latency_ms}" 'BEGIN { if (ms <= 0) { printf "%.2f", applied * 1000 } else { printf "%.2f", (applied * 1000.0) / ms } }')"

  local tmp_result_file sorted_file
  tmp_result_file="$(mktemp /tmp/mp_pg_replay_curve_workers.XXXXXX)"
  sorted_file="$(mktemp /tmp/mp_pg_replay_curve_workers_sorted.XXXXXX)"
  typeset -a worker_pids

  local i
  for i in $(seq 1 "${concurrent_noop}"); do
    (
      local worker_start_ms worker_raw worker_latency_ms worker_code worker_body worker_applied
      worker_start_ms="$(now_ms)"
      worker_raw="$(api_with_auth POST "/api/v1/state/change-log/replay/checkpoint" "${replay_payload}")"
      worker_latency_ms="$(elapsed_ms "${worker_start_ms}")"
      worker_code="${worker_raw##*$'\n'}"
      worker_body="${worker_raw%$'\n'*}"
      worker_applied="$(echo "${worker_body}" | jq -r '.applied // -1' 2>/dev/null || echo "-1")"
      echo "${i},${worker_code},${worker_latency_ms},${worker_applied}" >>"${tmp_result_file}"
    ) &
    worker_pids+=($!)
  done
  for pid in "${worker_pids[@]}"; do
    wait "${pid}"
  done

  local total_workers fail_count p95_index p95_ms max_ms
  total_workers="$(wc -l <"${tmp_result_file}" | tr -d '[:space:]')"
  fail_count="$(awk -F, '$2 != "200" || $4 != "0" {c++} END {print c+0}' "${tmp_result_file}")"
  sort -t, -k3n "${tmp_result_file}" >"${sorted_file}"
  p95_index="$(awk -v n="${total_workers}" 'BEGIN { idx = int((n*95 + 99) / 100); if (idx < 1) idx = 1; print idx }')"
  p95_ms="$(awk -F, -v idx="${p95_index}" 'NR==idx {print $3}' "${sorted_file}")"
  max_ms="$(awk -F, 'BEGIN{m=0} {if ($3>m) m=$3} END{print m+0}' "${tmp_result_file}")"
  rm -f "${tmp_result_file}" "${sorted_file}"

  local checkpoint_raw checkpoint_after_id
  checkpoint_raw="$(api_with_auth GET "/api/v1/state/change-log/checkpoints?state_key=${state_key}&limit=1")"
  split_response "${checkpoint_raw}"
  require_http_code "200" "${level_name}/${state_key} list checkpoint"
  checkpoint_after_id="$(echo "${HTTP_BODY}" | jq -r 'if (.items | length) > 0 then .items[0].last_change_id else 0 end')"

  if [[ "${checkpoint_after_id}" -lt "${latest_change_id}" ]]; then
    echo "FAIL ${level_name}/${state_key}: checkpoint=${checkpoint_after_id} < latest=${latest_change_id}"
    exit 1
  fi
  if [[ "${fail_count}" -ne 0 ]]; then
    echo "FAIL ${level_name}/${state_key}: replay no-op failures=${fail_count}"
    exit 1
  fi
  if [[ "${p95_ms}" -gt "${p95_threshold_ms}" ]]; then
    echo "FAIL ${level_name}/${state_key}: noop_p95_ms=${p95_ms} threshold=${p95_threshold_ms}"
    exit 1
  fi
  if [[ "${max_ms}" -gt "${max_threshold_ms}" ]]; then
    echo "FAIL ${level_name}/${state_key}: noop_max_ms=${max_ms} threshold=${max_threshold_ms}"
    exit 1
  fi
  if awk -v throughput="${catchup_throughput}" -v threshold="${min_ops}" 'BEGIN {exit !(throughput+0 < threshold+0)}'; then
    echo "FAIL ${level_name}/${state_key}: throughput=${catchup_throughput} threshold=${min_ops}"
    exit 1
  fi

  echo "${level_name},${state_key},${expected_delta},${catchup_applied},${catchup_latency_ms},${catchup_throughput},${total_workers},${fail_count},${p95_ms},${max_ms},${min_ops},${p95_threshold_ms},${max_threshold_ms}" >>"${TMP_RESULTS_FILE}"
  echo "summary: level=${level_name} state_key=${state_key} delta=${expected_delta} catchup_applied=${catchup_applied} catchup_latency_ms=${catchup_latency_ms} throughput=${catchup_throughput} noop_workers=${total_workers} noop_p95_ms=${p95_ms} noop_max_ms=${max_ms}"
}

trap cleanup EXIT

if ! command -v psql >/dev/null 2>&1; then
  echo "FAIL prereq: psql not found"
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "FAIL prereq: jq not found"
  exit 1
fi
if ! command -v perl >/dev/null 2>&1; then
  echo "FAIL prereq: perl not found"
  exit 1
fi

typeset -a tenant_levels
typeset -a building_levels
typeset -a noop_levels
typeset -a replay_levels
typeset -a min_ops_levels
typeset -a p95_levels
typeset -a max_levels
typeset -a state_keys

tenant_levels=("${(@s:,:)$(csv_compact "${LEVEL_TENANT_WRITES}")}")
building_levels=("${(@s:,:)$(csv_compact "${LEVEL_BUILDINGS_PER_TENANT}")}")
noop_levels=("${(@s:,:)$(csv_compact "${LEVEL_CONCURRENT_NOOP}")}")
replay_levels=("${(@s:,:)$(csv_compact "${LEVEL_REPLAY_LIMIT}")}")
min_ops_levels=("${(@s:,:)$(csv_compact "${LEVEL_MIN_OPS}")}")
p95_levels=("${(@s:,:)$(csv_compact "${LEVEL_P95_MS}")}")
max_levels=("${(@s:,:)$(csv_compact "${LEVEL_MAX_MS}")}")
state_keys=("${(@s:,:)$(csv_compact "${STATE_KEYS}")}")

level_count="${#tenant_levels[@]}"
if [[ "${level_count}" -lt 1 ]]; then
  echo "FAIL levels: at least one level is required"
  exit 1
fi
if [[ "${#building_levels[@]}" -ne "${level_count}" || "${#noop_levels[@]}" -ne "${level_count}" || "${#replay_levels[@]}" -ne "${level_count}" || "${#min_ops_levels[@]}" -ne "${level_count}" || "${#p95_levels[@]}" -ne "${level_count}" || "${#max_levels[@]}" -ne "${level_count}" ]]; then
  echo "FAIL levels: LEVEL_* arrays must have same length"
  exit 1
fi
if [[ "${#state_keys[@]}" -lt 1 ]]; then
  echo "FAIL state keys: at least one state key is required"
  exit 1
fi

TMP_RESULTS_FILE="$(mktemp /tmp/mp_pg_replay_curve_results.XXXXXX)"
echo "level,state_key,delta,catchup_applied,catchup_latency_ms,catchup_throughput_ops_per_sec,noop_workers,noop_failures,noop_p95_ms,noop_max_ms,min_ops_threshold,p95_threshold_ms,max_threshold_ms" >"${TMP_RESULTS_FILE}"

echo "== start api =="
start_api

echo "== login =="
LOGIN_RAW="$(curl -sS --connect-timeout 5 --max-time 30 -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${LOGIN_EMAIL}\",\"password\":\"${LOGIN_PASSWORD}\"}" \
  -w $'\n%{http_code}')"
split_response "${LOGIN_RAW}"
require_http_code "200" "login"
AT="$(echo "${HTTP_BODY}" | jq -r '.access_token')"
require_non_empty "${AT}" "login.access_token"

for idx in $(seq 1 "${level_count}"); do
  level_name="L${idx}"
  tenant_writes="${tenant_levels[$idx]}"
  buildings_per_tenant="${building_levels[$idx]}"
  concurrent_noop="${noop_levels[$idx]}"
  replay_limit="${replay_levels[$idx]}"
  min_ops="${min_ops_levels[$idx]}"
  p95_threshold="${p95_levels[$idx]}"
  max_threshold="${max_levels[$idx]}"

  echo "== ${level_name}: seed writes (tenants=${tenant_writes}, buildings_per_tenant=${buildings_per_tenant}) =="

  typeset -a baseline_change_ids
  typeset -a latest_change_ids
  for key_idx in $(seq 1 "${#state_keys[@]}"); do
    local_state_key="${state_keys[$key_idx]}"
    baseline_change_ids[$key_idx]="$(fetch_latest_change_id "${local_state_key}")"
    # Keep module replay checkpoint caught up before seeding this level to avoid stale backlog affecting write path.
    set_checkpoint "${local_state_key}" "${baseline_change_ids[$key_idx]}"
  done

  typeset -a level_tenant_ids
  for i in $(seq 1 "${tenant_writes}"); do
    tenant_payload="$(jq -nc --arg name "Replay Curve ${RUN_TAG}-${level_name}-tenant-${i}" '{name:$name,type:"company",hq_region:"ID-JK"}')"
    tenant_raw="$(api_with_auth POST "/api/v1/tenants" "${tenant_payload}")"
    split_response "${tenant_raw}"
    require_http_code "201" "${level_name} create tenant #${i}"
    tenant_id="$(echo "${HTTP_BODY}" | jq -r '.id')"
    require_non_empty "${tenant_id}" "${level_name} create tenant #${i}.id"
    level_tenant_ids+=("${tenant_id}")
  done

  if [[ "${buildings_per_tenant}" -gt 0 ]]; then
    for tenant_id in "${level_tenant_ids[@]}"; do
      for j in $(seq 1 "${buildings_per_tenant}"); do
        building_payload="$(jq -nc --arg tid "${tenant_id}" --arg name "Replay Curve ${RUN_TAG}-${level_name}-b-${j}" --arg addr "Replay Bench Address" '{tenant_id:$tid,name:$name,address:$addr,region:"ID-JK"}')"
        building_raw="$(api_with_auth POST "/api/v1/buildings" "${building_payload}")"
        split_response "${building_raw}"
        require_http_code "201" "${level_name} create building tenant=${tenant_id} #${j}"
      done
    done
  fi

  for key_idx in $(seq 1 "${#state_keys[@]}"); do
    local_state_key="${state_keys[$key_idx]}"
    latest_change_ids[$key_idx]="$(fetch_latest_change_id "${local_state_key}")"
    if [[ "${latest_change_ids[$key_idx]}" -le "${baseline_change_ids[$key_idx]}" ]]; then
      echo "FAIL ${level_name}/${local_state_key}: seed write did not increase change log"
      exit 1
    fi
  done

  for key_idx in $(seq 1 "${#state_keys[@]}"); do
    local_state_key="${state_keys[$key_idx]}"
    benchmark_state_key \
      "${level_name}" \
      "${local_state_key}" \
      "${baseline_change_ids[$key_idx]}" \
      "${latest_change_ids[$key_idx]}" \
      "${replay_limit}" \
      "${concurrent_noop}" \
      "${min_ops}" \
      "${p95_threshold}" \
      "${max_threshold}"
  done
done

echo "== curve summary =="
if command -v column >/dev/null 2>&1; then
  column -s, -t "${TMP_RESULTS_FILE}"
else
  cat "${TMP_RESULTS_FILE}"
fi
if [[ -n "${RESULT_CSV}" ]]; then
  cp "${TMP_RESULTS_FILE}" "${RESULT_CSV}"
  echo "summary: result_csv=${RESULT_CSV}"
fi
echo "PASS: pg replay multi-state curve baseline complete"
