#!/usr/bin/env zsh
set -euo pipefail

WEB_ADMIN_DIR="${WEB_ADMIN_DIR:-web-admin}"
SMOKE_BUILD="${SMOKE_BUILD:-0}"

if [[ ! -d "${WEB_ADMIN_DIR}" ]]; then
  echo "FAIL role-boundary smoke: missing directory ${WEB_ADMIN_DIR}"
  exit 1
fi

if [[ "${SMOKE_BUILD}" == "1" ]]; then
  echo "role-boundary smoke: build"
  (
    cd "${WEB_ADMIN_DIR}"
    npm run build >/tmp/mp_web_admin_role_boundary_build.log 2>&1
  )
fi

APP_FILE="${WEB_ADMIN_DIR}/src/App.tsx"
VIEWER_FILE="${WEB_ADMIN_DIR}/src/lib/viewer.ts"
if [[ ! -f "${APP_FILE}" || ! -f "${VIEWER_FILE}" ]]; then
  echo "FAIL role-boundary smoke: missing app/viewer file"
  exit 1
fi

typeset -a role_guard_markers=(
  'canAccessEnterprisePage(viewer) ? ('
  'canAccessSpacesPage(viewer) ? ('
  'canAccessAccessPage(viewer) ? ('
  'canAccessIssuancePage(viewer) ? ('
  'canAccessGatewaysPage(viewer) ? ('
  'canAccessEventsPage(viewer) ? ('
  'canAccessAlarmsPage(viewer) ? ('
  '<Navigate to="/access/directory" replace />'
)

echo "role-boundary smoke: route guards"
for marker in "${role_guard_markers[@]}"; do
  if ! rg -F -q "${marker}" "${APP_FILE}"; then
    echo "FAIL role-boundary smoke: missing guard marker ${marker}"
    exit 1
  fi
  echo "PASS guard ${marker}"
done

typeset -a role_identity_markers=(
  "${APP_FILE}::平台工作台总览"
  "${APP_FILE}::管理导航"
  "${VIEWER_FILE}::值守人员"
)

echo "role-boundary smoke: role identity markers"
for item in "${role_identity_markers[@]}"; do
  file="${item%%::*}"
  pattern="${item#*::}"
  if ! rg -F -q "${pattern}" "${file}"; then
    echo "FAIL role-boundary smoke: missing identity marker ${pattern} in ${file}"
    exit 1
  fi
  echo "PASS identity ${pattern}"
done

typeset -a role_boundary_markers=(
  "${WEB_ADMIN_DIR}/src/pages/dashboard-page.tsx::当前楼宇管理员尚未分配 \`building_ids\` 范围。仪表盘只保留空态指标，不展示任何楼宇级运行数据。"
  "${WEB_ADMIN_DIR}/src/pages/spaces-page.tsx::当前楼宇管理员尚未分配 \`building_ids\` 范围。此页不会展示任何楼宇、楼层、区域或门点数据，也不会开放新增操作。"
  "${WEB_ADMIN_DIR}/src/pages/events-page.tsx::当前楼宇管理员尚未分配 \`building_ids\` 范围。此页不会展示任何事件记录，避免误暴露非本楼宇数据。"
  "${WEB_ADMIN_DIR}/src/pages/alarms-page.tsx::当前楼宇管理员尚未分配 \`building_ids\` 范围。此页不会展示任何告警记录，避免误处置非本楼宇事件。"
  "${WEB_ADMIN_DIR}/src/pages/gateways-page.tsx::当前楼宇管理员尚未分配 \`building_ids\` 范围。此页不会展示任何网关、门点或边缘设备数据，避免误操作非本楼宇设备。"
  "${WEB_ADMIN_DIR}/src/pages/gateways-page.tsx::按钮禁用或缺失属于权限边界，不是系统异常。"
  "${WEB_ADMIN_DIR}/src/pages/wallet-page.tsx::按钮禁用或缺失属于权限边界，不是系统异常。"
  "${WEB_ADMIN_DIR}/src/pages/wallet-page.tsx::只读（权限边界）"
)

echo "role-boundary smoke: scope/read-only boundary markers"
for item in "${role_boundary_markers[@]}"; do
  file="${item%%::*}"
  pattern="${item#*::}"
  if [[ ! -f "${file}" ]]; then
    echo "FAIL role-boundary smoke: missing boundary contract file ${file}"
    exit 1
  fi
  if ! rg -F -q "${pattern}" "${file}"; then
    echo "FAIL role-boundary smoke: missing boundary marker ${pattern} in ${file}"
    exit 1
  fi
  echo "PASS boundary ${pattern}"
done

echo "PASS role-boundary smoke: build=${SMOKE_BUILD} guard_markers=${#role_guard_markers[@]} identity_markers=${#role_identity_markers[@]} boundary_markers=${#role_boundary_markers[@]}"
