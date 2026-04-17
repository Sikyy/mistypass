#!/usr/bin/env zsh
set -euo pipefail

echo "DEPRECATED: use docs/testing/curl-wallet-job-alert-dispatch-resend.zsh"
exec /bin/zsh "$(dirname "$0")/curl-wallet-job-alert-dispatch-resend.zsh"
