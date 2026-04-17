#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

server_pid=""

cleanup() {
  if [[ -n "${server_pid}" ]] && kill -0 "${server_pid}" 2>/dev/null; then
    kill "${server_pid}" 2>/dev/null || true
    wait "${server_pid}" 2>/dev/null || true
  fi
}

start_server() {
  (
    cd "${API_DIR}"
    go run ./cmd/api
  ) &
  server_pid=$!
  echo "[dev-hot] backend started (pid=${server_pid})"
}

file_fingerprint() {
  find "${API_DIR}" -type f -name "*.go" -not -path "*/vendor/*" -print0 \
    | xargs -0 stat -f "%m %N" \
    | shasum \
    | awk '{print $1}'
}

trap cleanup EXIT INT TERM

start_server
last_fingerprint="$(file_fingerprint)"

while true; do
  sleep 1
  next_fingerprint="$(file_fingerprint)"

  if [[ "${next_fingerprint}" == "${last_fingerprint}" ]]; then
    continue
  fi

  echo "[dev-hot] change detected, restarting backend..."
  cleanup
  start_server
  last_fingerprint="${next_fingerprint}"
done
