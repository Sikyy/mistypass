#!/bin/sh
# ExecStartPre guard for gateway-agent self-update auto-rollback.
# Runs before every agent start. If a pending OTA marker is not confirmed after
# MAX_ATTEMPTS boots, restore the backed-up binary. Covers "new binary won't
# start at all" (the agent itself can't roll back if it never runs).
set -eu

MARKER="${MISTYPASS_OTA_MARKER:-/var/lib/mistypass/ota-pending.json}"
BIN="${MISTYPASS_AGENT_BIN:-/usr/local/bin/gateway-agent}"
MAX_ATTEMPTS="${MISTYPASS_OTA_MAX_ATTEMPTS:-3}"

[ -f "$MARKER" ] || exit 0

confirmed=$(grep -o '"confirmed":[^,}]*' "$MARKER" | head -1 | sed 's/.*://; s/[^a-z]//g')
[ "$confirmed" = "true" ] && exit 0

attempts=$(grep -o '"attempts":[0-9][0-9]*' "$MARKER" | head -1 | sed 's/.*://')
[ -n "$attempts" ] || attempts=0
attempts=$((attempts + 1))

bak=$(grep -o '"bak_path":"[^"]*"' "$MARKER" | head -1 | sed 's/.*:"//; s/"$//')

if [ "$attempts" -ge "$MAX_ATTEMPTS" ] && [ -n "$bak" ] && [ -f "$bak" ]; then
  cp "$bak" "${BIN}.new"
  mv "${BIN}.new" "$BIN"
  rm -f "$MARKER"
  echo "mistypass-ota-guard: rolled back to $bak after $attempts failed boots" >&2
  exit 0
fi

tmp="$(mktemp "${MARKER}.XXXXXX")"
trap 'rm -f "$tmp"' EXIT INT TERM
sed "s/\"attempts\":[0-9][0-9]*/\"attempts\":$attempts/" "$MARKER" > "$tmp" && mv "$tmp" "$MARKER"
echo "mistypass-ota-guard: pending OTA boot attempt $attempts" >&2
exit 0
