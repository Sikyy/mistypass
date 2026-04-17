#!/usr/bin/env zsh
set -euo pipefail

SCRIPT_DIR="${0:A:h}"
REVIEW_SCRIPT="${REVIEW_SCRIPT:-${SCRIPT_DIR}/curl-pg-replay-soak-review.zsh}"
SIGNOFF_SCRIPT="${SIGNOFF_SCRIPT:-${SCRIPT_DIR}/curl-pg-replay-soak-signoff.zsh}"
WORKDIR="${WORKDIR:-/tmp/mp_pg_replay_soak_review_smoke}"
HISTORY_ROOT="${HISTORY_ROOT:-${WORKDIR}/history}"
REVIEW_WORKDIR="${REVIEW_WORKDIR:-${WORKDIR}/review}"

if ! command -v jq >/dev/null 2>&1; then
  echo "FAIL prereq: jq is required"
  exit 1
fi

if [[ ! -f "${REVIEW_SCRIPT}" ]]; then
  echo "FAIL prereq: review script not found -> ${REVIEW_SCRIPT}"
  exit 1
fi
if [[ ! -f "${SIGNOFF_SCRIPT}" ]]; then
  echo "FAIL prereq: signoff script not found -> ${SIGNOFF_SCRIPT}"
  exit 1
fi

rm -rf "${WORKDIR}"
mkdir -p "${HISTORY_ROOT}/sample-day-1" "${HISTORY_ROOT}/sample-day-2" "${REVIEW_WORKDIR}"

cat >"${HISTORY_ROOT}/sample-day-1/metrics.csv" <<'EOF'
round,started_at,level,state_key,delta,catchup_applied,catchup_latency_ms,throughput_ops_per_sec,noop_p95_ms,noop_max_ms,min_ops_threshold,p95_threshold_ms,max_threshold_ms,status,error
1,2026-04-10T01:00:00Z,L1,key_a,100,100,1200,80.0,15,30,10,200,400,passed,
1,2026-04-10T01:00:00Z,L2,key_b,120,120,1300,70.0,18,35,10,200,400,passed,
EOF

cat >"${HISTORY_ROOT}/sample-day-2/metrics.csv" <<'EOF'
round,started_at,level,state_key,delta,catchup_applied,catchup_latency_ms,throughput_ops_per_sec,noop_p95_ms,noop_max_ms,min_ops_threshold,p95_threshold_ms,max_threshold_ms,status,error
1,2026-04-12T01:00:00Z,L1,key_a,110,110,1180,78.0,16,29,10,200,400,passed,
1,2026-04-12T01:00:00Z,L2,key_b,130,130,1350,69.0,17,34,10,200,400,passed,
EOF

SOAK_REVIEW_ROOT="${HISTORY_ROOT}" \
SOAK_REVIEW_WORKDIR="${REVIEW_WORKDIR}" \
SOAK_REVIEW_MIN_DAYS=2 \
SOAK_REVIEW_DROP_RATIO_MAX=0.5 \
SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS=0 \
SOAK_REVIEW_STRICT=false \
/bin/zsh "${REVIEW_SCRIPT}" >/tmp/mp_pg_replay_soak_review_smoke_review_gap.log

review_status="$(jq -r '.status // "unknown"' "${REVIEW_WORKDIR}/summary.json")"
coverage_gap="$(jq -r '.coverage_days_gap_to_allowed // -1' "${REVIEW_WORKDIR}/summary.json")"
if [[ "${review_status}" != "insufficient_coverage" || "${coverage_gap}" != "1" ]]; then
  echo "FAIL smoke(gap): expected status=insufficient_coverage coverage_gap=1, got status=${review_status} coverage_gap=${coverage_gap}"
  exit 1
fi

SOAK_SIGNOFF_SUMMARY_JSON="${REVIEW_WORKDIR}/summary.json" \
SOAK_SIGNOFF_DAY_CSV="${REVIEW_WORKDIR}/day-summary.csv" \
SOAK_SIGNOFF_METRIC_CSV="${REVIEW_WORKDIR}/metric-summary.csv" \
SOAK_SIGNOFF_REPORT_MD="${REVIEW_WORKDIR}/signoff.md" \
SOAK_SIGNOFF_FAIL_ON_HOLD=false \
/bin/zsh "${SIGNOFF_SCRIPT}" >/tmp/mp_pg_replay_soak_review_smoke_signoff_gap.log

if ! grep -q 'decision: `hold_collect_more_data`' "${REVIEW_WORKDIR}/signoff.md"; then
  echo "FAIL smoke(gap): expected hold_collect_more_data decision"
  exit 1
fi

SOAK_REVIEW_ROOT="${HISTORY_ROOT}" \
SOAK_REVIEW_WORKDIR="${REVIEW_WORKDIR}" \
SOAK_REVIEW_MIN_DAYS=2 \
SOAK_REVIEW_DROP_RATIO_MAX=0.5 \
SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS=1 \
SOAK_REVIEW_STRICT=false \
/bin/zsh "${REVIEW_SCRIPT}" >/tmp/mp_pg_replay_soak_review_smoke_review_pass.log

review_status="$(jq -r '.status // "unknown"' "${REVIEW_WORKDIR}/summary.json")"
coverage_gap="$(jq -r '.coverage_days_gap_to_allowed // -1' "${REVIEW_WORKDIR}/summary.json")"
if [[ "${review_status}" != "passed" || "${coverage_gap}" != "0" ]]; then
  echo "FAIL smoke(pass): expected status=passed coverage_gap=0, got status=${review_status} coverage_gap=${coverage_gap}"
  exit 1
fi

SOAK_SIGNOFF_SUMMARY_JSON="${REVIEW_WORKDIR}/summary.json" \
SOAK_SIGNOFF_DAY_CSV="${REVIEW_WORKDIR}/day-summary.csv" \
SOAK_SIGNOFF_METRIC_CSV="${REVIEW_WORKDIR}/metric-summary.csv" \
SOAK_SIGNOFF_REPORT_MD="${REVIEW_WORKDIR}/signoff.md" \
SOAK_SIGNOFF_FAIL_ON_HOLD=false \
/bin/zsh "${SIGNOFF_SCRIPT}" >/tmp/mp_pg_replay_soak_review_smoke_signoff_pass.log

if ! grep -q 'decision: `ready_for_signoff`' "${REVIEW_WORKDIR}/signoff.md"; then
  echo "FAIL smoke(pass): expected ready_for_signoff decision"
  exit 1
fi

echo "summary: smoke_workdir=${WORKDIR}"
echo "summary: history_root=${HISTORY_ROOT}"
echo "summary: review_workdir=${REVIEW_WORKDIR}"
echo "PASS: pg replay soak review/signoff smoke complete"
