#!/usr/bin/env zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/docs/testing/artifacts}"
RUN_TAG="${RUN_TAG:-$(date +%Y%m%d-%H%M%S)-$RANDOM}"
REPORT_FILE="${OUTPUT_DIR}/edge-mvp-validation-${RUN_TAG}.md"
BASE_PORT="${BASE_PORT:-19180}"

mkdir -p "${OUTPUT_DIR}"

SCRIPTS=(
  "docs/testing/curl-gateway-legacy-wiegand-poc.zsh"
  "docs/testing/curl-gateway-door-io-loop.zsh"
  "docs/testing/curl-gateway-serial-protocol.zsh"
  "docs/testing/curl-gateway-event-idempotency.zsh"
  "docs/testing/curl-gateway-event-retry-subset-mixed.zsh"
  "docs/testing/curl-gateway-event-checkpoint-partial.zsh"
  "docs/testing/curl-gateway-edge-queue-executor-sim.zsh"
)

function now_iso() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

function write_report_header() {
  cat >"${REPORT_FILE}" <<MARKDOWN
# Edge MVP Validation Report

- run_tag: ${RUN_TAG}
- generated_at_utc: ${NOW_UTC}
- api_base_url: ${API_BASE_URL:-http://localhost:8080}
- host: $(hostname)

| script | api_port | status | elapsed_sec | log_file |
|---|---:|---|---:|---|
MARKDOWN
}

function append_row() {
  local script="$1"
  local api_port="$2"
  local result="$3"
  local elapsed="$4"
  local logfile="$5"
  printf "| %s | %s | %s | %s | %s |\n" "${script}" "${api_port}" "${result}" "${elapsed}" "${logfile}" >>"${REPORT_FILE}"
}

NOW_UTC="$(now_iso)"
write_report_header

FAIL_COUNT=0
SCRIPT_INDEX=0

for script in "${SCRIPTS[@]}"; do
  SCRIPT_INDEX=$((SCRIPT_INDEX + 1))
  api_port="$((BASE_PORT + SCRIPT_INDEX - 1))"
  api_base_url="http://localhost:${api_port}"
  script_path="${ROOT_DIR}/${script}"
  if [[ ! -f "${script_path}" ]]; then
    append_row "${script}" "${api_port}" "missing" "0" "N/A"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    continue
  fi

  log_file="${OUTPUT_DIR}/$(basename "${script}" .zsh)-${RUN_TAG}.log"

  start_ts="$(date +%s)"
  if API_PORT="${api_port}" API_BASE_URL="${api_base_url}" /bin/zsh "${script_path}" >"${log_file}" 2>&1; then
    result="PASS"
  else
    result="FAIL"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
  end_ts="$(date +%s)"
  elapsed="$((end_ts - start_ts))"

  append_row "${script}" "${api_port}" "${result}" "${elapsed}" "${log_file}"
  echo "${script}: ${result} (${elapsed}s, port=${api_port})"
done

echo "" >>"${REPORT_FILE}"
echo "- fail_count: ${FAIL_COUNT}" >>"${REPORT_FILE}"

echo "report: ${REPORT_FILE}"

if [[ "${FAIL_COUNT}" -gt 0 ]]; then
  exit 1
fi
