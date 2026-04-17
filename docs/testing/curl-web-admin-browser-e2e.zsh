#!/usr/bin/env zsh
set -euo pipefail

WEB_ADMIN_DIR="${WEB_ADMIN_DIR:-web-admin}"

if [[ ! -d "${WEB_ADMIN_DIR}" ]]; then
  echo "FAIL web-admin browser e2e: missing directory ${WEB_ADMIN_DIR}"
  exit 1
fi

echo "web-admin browser e2e: playwright access + enterprise flow baseline"
(
  cd "${WEB_ADMIN_DIR}"
  PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 npx playwright test -c playwright.config.ts --reporter=line
)

echo "PASS web-admin browser e2e: playwright access + enterprise flow baseline"
