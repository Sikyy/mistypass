#!/usr/bin/env zsh
set -euo pipefail

WEB_ADMIN_DIR="${WEB_ADMIN_DIR:-web-admin}"
WEB_HOST="${WEB_HOST:-127.0.0.1}"
WEB_PORT="${WEB_PORT:-18743}"
WEB_BASE_URL="${WEB_BASE_URL:-http://${WEB_HOST}:${WEB_PORT}}"
SMOKE_BUILD="${SMOKE_BUILD:-1}"
SMOKE_HTTP_CHECK="${SMOKE_HTTP_CHECK:-0}"
SMOKE_AUTO_PREVIEW="${SMOKE_AUTO_PREVIEW:-0}"

PREVIEW_PID=""
STARTED_PREVIEW=0
HTTP_CODE=""
HTTP_BODY=""

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

function require_contains() {
  local content="$1"
  local pattern="$2"
  local step="$3"
  if ! echo "${content}" | rg -q "${pattern}"; then
    echo "FAIL ${step}: missing pattern ${pattern}"
    exit 1
  fi
}

function cleanup() {
  if [[ "${STARTED_PREVIEW}" == "1" && -n "${PREVIEW_PID}" ]]; then
    kill "${PREVIEW_PID}" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

if [[ ! -d "${WEB_ADMIN_DIR}" ]]; then
  echo "FAIL web-admin smoke: missing directory ${WEB_ADMIN_DIR}"
  exit 1
fi

if [[ "${SMOKE_BUILD}" == "1" ]]; then
  echo "web-admin smoke: build"
  (
    cd "${WEB_ADMIN_DIR}"
    npm run build >/tmp/mp_web_admin_smoke_build.log 2>&1
  )

  enterprise_chunks=("${WEB_ADMIN_DIR}"/dist/assets/enterprise-page-*.js(N))
  access_chunks=("${WEB_ADMIN_DIR}"/dist/assets/access-page-*.js(N))
  if [[ "${#enterprise_chunks[@]}" -eq 0 || "${#access_chunks[@]}" -eq 0 ]]; then
    echo "FAIL web-admin smoke: missing enterprise/access chunks after build"
    exit 1
  fi
fi

APP_FILE="${WEB_ADMIN_DIR}/src/App.tsx"
if [[ ! -f "${APP_FILE}" ]]; then
  echo "FAIL web-admin smoke: missing app file ${APP_FILE}"
  exit 1
fi

typeset -a route_markers=(
  'path="/login"'
  'path="/dashboard"'
  'path="/enterprise"'
  'path="/spaces"'
  'path="/access"'
  'path="/access/directory"'
  'path="/access/policies"'
  'path="/access/grants"'
  'path="/access/:section"'
  'path="/wallet"'
  'path="/gateways"'
  'path="/events"'
  'path="/alarms"'
)

echo "web-admin smoke: route contract markers"
for marker in "${route_markers[@]}"; do
  if ! rg -F -q "${marker}" "${APP_FILE}"; then
    echo "FAIL web-admin smoke: missing route marker ${marker}"
    exit 1
  fi
  echo "PASS marker ${marker}"
done

typeset -a guard_markers=(
  'canAccessEnterprisePage(viewer) ? ('
  'canAccessSpacesPage(viewer) ? ('
  'canAccessAccessPage(viewer) ? ('
  'canAccessIssuancePage(viewer) ? ('
  'canAccessGatewaysPage(viewer) ? ('
  'canAccessEventsPage(viewer) ? ('
  'canAccessAlarmsPage(viewer) ? ('
  '<Navigate to="/access/directory" replace />'
)

echo "web-admin smoke: auth guard markers"
for marker in "${guard_markers[@]}"; do
  if ! rg -F -q "${marker}" "${APP_FILE}"; then
    echo "FAIL web-admin smoke: missing guard marker ${marker}"
    exit 1
  fi
  echo "PASS guard ${marker}"
done

typeset -a flow_contracts=(
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::同步来源状态总览"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::目录到策略主流程连通检查"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::title: \"凭证发放\""
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::/wallet?scenario=employee_mobile"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::/access/directory"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::/access/policies"
  "${WEB_ADMIN_DIR}/src/components/access/access-page-recommendation-utils.ts::label: \"去员工与用户组\""
  "${WEB_ADMIN_DIR}/src/components/access/access-page-recommendation-utils.ts::label: \"去权限策略\""
  "${WEB_ADMIN_DIR}/src/components/access/access-page-recommendation-utils.ts::label: \"去凭证发放\""
)

echo "web-admin smoke: mainflow contract markers"
for item in "${flow_contracts[@]}"; do
  file="${item%%::*}"
  pattern="${item#*::}"
  if [[ ! -f "${file}" ]]; then
    echo "FAIL web-admin smoke: missing flow contract file ${file}"
    exit 1
  fi
  if ! rg -F -q "${pattern}" "${file}"; then
    echo "FAIL web-admin smoke: missing flow contract marker ${pattern} in ${file}"
    exit 1
  fi
  echo "PASS flow ${pattern}"
done

typeset -a interaction_contracts=(
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-alerts-workspace.tsx::批量批准 pending（"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-alerts-workspace.tsx::批量拒绝 pending（"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-alerts-workspace.tsx::批量标记已回写（"
  "${WEB_ADMIN_DIR}/src/pages/wallet-page.tsx::批量重发失败通道（"
  "${WEB_ADMIN_DIR}/src/pages/wallet-page.tsx::批量状态修复（"
  "${WEB_ADMIN_DIR}/src/pages/alarms-page.tsx::告警通知策略"
  "${WEB_ADMIN_DIR}/src/pages/gateways-page.tsx::导出当前筛选结果"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::处理后去员工与用户组"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::处理后去权限策略"
  "${WEB_ADMIN_DIR}/src/components/enterprise/enterprise-sync-workspace.tsx::处理后去凭证发放"
)

echo "web-admin smoke: interaction contract markers"
for item in "${interaction_contracts[@]}"; do
  file="${item%%::*}"
  pattern="${item#*::}"
  if [[ ! -f "${file}" ]]; then
    echo "FAIL web-admin smoke: missing interaction contract file ${file}"
    exit 1
  fi
  if ! rg -F -q "${pattern}" "${file}"; then
    echo "FAIL web-admin smoke: missing interaction contract marker ${pattern} in ${file}"
    exit 1
  fi
  echo "PASS interaction ${pattern}"
done

if [[ "${SMOKE_HTTP_CHECK}" != "1" ]]; then
  echo "PASS web-admin smoke: build=${SMOKE_BUILD} route_markers=${#route_markers[@]} guard_markers=${#guard_markers[@]} flow_markers=${#flow_contracts[@]} interaction_markers=${#interaction_contracts[@]} http_check=skipped"
  exit 0
fi

if curl -sS "${WEB_BASE_URL}/" >/dev/null 2>&1; then
  echo "web-admin smoke: reuse running server ${WEB_BASE_URL}"
elif [[ "${SMOKE_AUTO_PREVIEW}" == "1" ]]; then
  echo "web-admin smoke: start preview ${WEB_BASE_URL}"
  (
    cd "${WEB_ADMIN_DIR}"
    setopt nobgnice
    npm run preview -- --host "${WEB_HOST}" --port "${WEB_PORT}" >/tmp/mp_web_admin_smoke_preview.log 2>&1
  ) &
  PREVIEW_PID="$!"
  STARTED_PREVIEW=1

  ready="0"
  for _ in {1..60}; do
    if curl -sS "${WEB_BASE_URL}/" >/dev/null 2>&1; then
      ready="1"
      break
    fi
    sleep 0.25
  done
  if [[ "${ready}" != "1" ]]; then
    echo "FAIL web-admin smoke: preview not ready at ${WEB_BASE_URL}"
    if [[ -f /tmp/mp_web_admin_smoke_preview.log ]]; then
      tail -n 80 /tmp/mp_web_admin_smoke_preview.log
    fi
    exit 1
  fi
else
  echo "FAIL web-admin smoke: HTTP check enabled but server not reachable at ${WEB_BASE_URL}"
  echo "hint: run SMOKE_AUTO_PREVIEW=1 ./docs/testing/curl-web-admin-smoke.zsh"
  exit 1
fi

echo "web-admin smoke: index + assets"
INDEX_RAW="$(curl -sS "${WEB_BASE_URL}/" -w $'\n%{http_code}')"
split_response "${INDEX_RAW}"
require_http_code "200" "GET /"
require_contains "${HTTP_BODY}" "id=\"root\"" "GET / root container"
require_contains "${HTTP_BODY}" "/assets/index-" "GET / bundle entry"

INDEX_JS_PATH="$(echo "${HTTP_BODY}" | rg -o '/assets/index-[^"]+\.js' | head -n 1)"
if [[ -z "${INDEX_JS_PATH}" ]]; then
  echo "FAIL web-admin smoke: missing index js path"
  exit 1
fi

INDEX_JS_RAW="$(curl -sS "${WEB_BASE_URL}${INDEX_JS_PATH}" -w $'\n%{http_code}')"
split_response "${INDEX_JS_RAW}"
require_http_code "200" "GET ${INDEX_JS_PATH}"
require_contains "${HTTP_BODY}" "react" "GET ${INDEX_JS_PATH} content"

typeset -a routes=(
  "/login"
  "/dashboard"
  "/enterprise"
  "/access"
  "/access/directory"
  "/access/policies"
  "/access/grants"
  "/wallet"
  "/gateways"
  "/events"
  "/alarms"
  "/spaces"
)

echo "web-admin smoke: routes"
for route in "${routes[@]}"; do
  ROUTE_RAW="$(curl -sS "${WEB_BASE_URL}${route}" -w $'\n%{http_code}')"
  split_response "${ROUTE_RAW}"
  require_http_code "200" "GET ${route}"
  require_contains "${HTTP_BODY}" "id=\"root\"" "GET ${route} root container"
  echo "PASS route ${route}"
done

echo "PASS web-admin smoke: build=${SMOKE_BUILD} route_markers=${#route_markers[@]} guard_markers=${#guard_markers[@]} flow_markers=${#flow_contracts[@]} interaction_markers=${#interaction_contracts[@]} base=${WEB_BASE_URL} routes=${#routes[@]}"
