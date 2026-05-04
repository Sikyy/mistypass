#!/usr/bin/env zsh
set -euo pipefail

# Verify docs/openapi.json and docs/openapi-mobile.json match the current code.
# Fails if the checked-in specs are stale (someone changed routes without
# regenerating the spec).

SCRIPT_DIR="${0:a:h}"
REPO_ROOT="${SCRIPT_DIR}/../.."
API_DIR="${REPO_ROOT}/api"
DOCS_DIR="${REPO_ROOT}/docs"

SPEC_FULL="${DOCS_DIR}/openapi.json"
SPEC_MOBILE="${DOCS_DIR}/openapi-mobile.json"

TMPDIR_DRIFT="$(mktemp -d)"
trap "rm -rf ${TMPDIR_DRIFT}" EXIT

echo "== generate fresh openapi.json =="
(cd "${API_DIR}" && go run ./cmd/openapi-extract/) > "${TMPDIR_DRIFT}/openapi-fresh.json"

echo "== normalize + compare full spec =="
python3 -m json.tool --sort-keys < "${TMPDIR_DRIFT}/openapi-fresh.json" > "${TMPDIR_DRIFT}/fresh-sorted.json" 2>/dev/null || cp "${TMPDIR_DRIFT}/openapi-fresh.json" "${TMPDIR_DRIFT}/fresh-sorted.json"
python3 -m json.tool --sort-keys < "${SPEC_FULL}" > "${TMPDIR_DRIFT}/checked-sorted.json" 2>/dev/null || cp "${SPEC_FULL}" "${TMPDIR_DRIFT}/checked-sorted.json"

if ! diff -q "${TMPDIR_DRIFT}/fresh-sorted.json" "${TMPDIR_DRIFT}/checked-sorted.json" >/dev/null 2>&1; then
  echo "FAIL: docs/openapi.json is stale. Regenerate with: cd api && make openapi"
  echo "Diff (first 40 lines):"
  diff "${TMPDIR_DRIFT}/fresh-sorted.json" "${TMPDIR_DRIFT}/checked-sorted.json" | head -40 || true
  exit 1
fi
echo "OK: docs/openapi.json is up to date"

echo "== generate fresh openapi-mobile.json =="
python3 "${API_DIR}/cmd/openapi-extract/filter_mobile.py" < "${TMPDIR_DRIFT}/openapi-fresh.json" > "${TMPDIR_DRIFT}/mobile-fresh.json"

echo "== normalize + compare mobile spec =="
python3 -m json.tool --sort-keys < "${TMPDIR_DRIFT}/mobile-fresh.json" > "${TMPDIR_DRIFT}/mobile-fresh-sorted.json" 2>/dev/null || cp "${TMPDIR_DRIFT}/mobile-fresh.json" "${TMPDIR_DRIFT}/mobile-fresh-sorted.json"
python3 -m json.tool --sort-keys < "${SPEC_MOBILE}" > "${TMPDIR_DRIFT}/mobile-checked-sorted.json" 2>/dev/null || cp "${SPEC_MOBILE}" "${TMPDIR_DRIFT}/mobile-checked-sorted.json"

if ! diff -q "${TMPDIR_DRIFT}/mobile-fresh-sorted.json" "${TMPDIR_DRIFT}/mobile-checked-sorted.json" >/dev/null 2>&1; then
  echo "FAIL: docs/openapi-mobile.json is stale. Regenerate with: cd api && make openapi-mobile"
  echo "Diff (first 40 lines):"
  diff "${TMPDIR_DRIFT}/mobile-fresh-sorted.json" "${TMPDIR_DRIFT}/mobile-checked-sorted.json" | head -40 || true
  exit 1
fi
echo "OK: docs/openapi-mobile.json is up to date"

echo "PASS: OpenAPI specs match generated output"
