#!/usr/bin/env zsh
set -euo pipefail

SOAK_SIGNOFF_SUMMARY_JSON="${SOAK_SIGNOFF_SUMMARY_JSON:-/tmp/mp_pg_replay_soak/review/summary.json}"
SOAK_SIGNOFF_DAY_CSV="${SOAK_SIGNOFF_DAY_CSV:-/tmp/mp_pg_replay_soak/review/day-summary.csv}"
SOAK_SIGNOFF_METRIC_CSV="${SOAK_SIGNOFF_METRIC_CSV:-/tmp/mp_pg_replay_soak/review/metric-summary.csv}"
SOAK_SIGNOFF_REPORT_MD="${SOAK_SIGNOFF_REPORT_MD:-/tmp/mp_pg_replay_soak/review/signoff.md}"
SOAK_SIGNOFF_FAIL_ON_HOLD="${SOAK_SIGNOFF_FAIL_ON_HOLD:-false}"

if ! command -v jq >/dev/null 2>&1; then
  echo "FAIL signoff: jq is required"
  exit 1
fi

if [[ ! -f "${SOAK_SIGNOFF_SUMMARY_JSON}" ]]; then
  echo "FAIL signoff: summary json not found -> ${SOAK_SIGNOFF_SUMMARY_JSON}"
  exit 1
fi

review_status="$(jq -r '.status // "unknown"' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
min_days_required="$(jq -r '.min_days_required // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
unique_days_observed="$(jq -r '.unique_days_observed // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
days_gap_to_min_days="$(jq -r '.days_gap_to_min_days // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
coverage_span_days="$(jq -r '.coverage_span_days // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
missing_days_in_span="$(jq -r '.missing_days_in_span // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
max_allowed_missing_days="$(jq -r '.max_allowed_missing_days // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
coverage_days_gap_to_allowed="$(jq -r '.coverage_days_gap_to_allowed // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
coverage_ratio="$(jq -r '.coverage_ratio // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
earliest_day="$(jq -r '.earliest_day // ""' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
latest_day="$(jq -r '.latest_day // ""' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
metrics_files="$(jq -r '.metrics_files // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
merged_rows="$(jq -r '.merged_rows // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
failed_rows="$(jq -r '.failed_rows // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
metric_count="$(jq -r '.metric_count // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
metric_failed="$(jq -r '.metric_failed // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
metric_insufficient_days="$(jq -r '.metric_insufficient_days // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
metric_near_drop_threshold="$(jq -r '.metric_near_drop_threshold // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"
drop_ratio_limit="$(jq -r '.drop_ratio_limit // 0' "${SOAK_SIGNOFF_SUMMARY_JSON}")"

generated_at_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

decision="hold_investigation_required"
decision_reason="failed metrics or failed rows detected"

if [[ "${review_status}" == "insufficient_days" || "${days_gap_to_min_days}" -gt 0 ]]; then
  decision="hold_collect_more_data"
  decision_reason="nightly coverage has not reached min days requirement"
elif [[ "${review_status}" == "insufficient_coverage" || "${coverage_days_gap_to_allowed}" -gt 0 ]]; then
  decision="hold_collect_more_data"
  decision_reason="nightly data coverage has date gaps beyond allowed threshold"
elif [[ "${review_status}" == "failed" || "${metric_failed}" -gt 0 || "${failed_rows}" -gt 0 ]]; then
  decision="hold_investigation_required"
  decision_reason="failed metrics or failed rows detected"
elif [[ "${review_status}" == "passed" ]]; then
  if [[ "${metric_near_drop_threshold}" -gt 0 ]]; then
    decision="watch_near_threshold"
    decision_reason="all checks passed, but one or more metrics are close to drop-ratio threshold"
  else
    decision="ready_for_signoff"
    decision_reason="coverage and stability checks passed with no near-threshold warnings"
  fi
else
  decision="hold_investigation_required"
  decision_reason="review status is unknown"
fi

mkdir -p "$(dirname "${SOAK_SIGNOFF_REPORT_MD}")"

{
  echo "# PostgreSQL Replay Soak Nightly Sign-off Snapshot"
  echo
  echo "- generated_at_utc: \`${generated_at_utc}\`"
  echo "- decision: \`${decision}\`"
  echo "- decision_reason: ${decision_reason}"
  echo
  echo "## Review Summary"
  echo
  echo "| key | value |"
  echo "|---|---|"
  echo "| status | \`${review_status}\` |"
  echo "| min_days_required | \`${min_days_required}\` |"
  echo "| unique_days_observed | \`${unique_days_observed}\` |"
  echo "| days_gap_to_min_days | \`${days_gap_to_min_days}\` |"
  echo "| coverage_span_days | \`${coverage_span_days}\` |"
  echo "| missing_days_in_span | \`${missing_days_in_span}\` |"
  echo "| max_allowed_missing_days | \`${max_allowed_missing_days}\` |"
  echo "| coverage_days_gap_to_allowed | \`${coverage_days_gap_to_allowed}\` |"
  echo "| coverage_ratio | \`${coverage_ratio}\` |"
  echo "| earliest_day | \`${earliest_day:-n/a}\` |"
  echo "| latest_day | \`${latest_day:-n/a}\` |"
  echo "| metrics_files | \`${metrics_files}\` |"
  echo "| merged_rows | \`${merged_rows}\` |"
  echo "| failed_rows | \`${failed_rows}\` |"
  echo "| metric_count | \`${metric_count}\` |"
  echo "| metric_failed | \`${metric_failed}\` |"
  echo "| metric_insufficient_days | \`${metric_insufficient_days}\` |"
  echo "| metric_near_drop_threshold | \`${metric_near_drop_threshold}\` |"
  echo "| drop_ratio_limit | \`${drop_ratio_limit}\` |"
  echo
  echo "## Evidence Checklist"
  echo
  echo "- [ ] summary json: \`${SOAK_SIGNOFF_SUMMARY_JSON}\`"
  echo "- [ ] day summary csv: \`${SOAK_SIGNOFF_DAY_CSV}\`"
  echo "- [ ] metric summary csv: \`${SOAK_SIGNOFF_METRIC_CSV}\`"
  echo "- [ ] signoff snapshot: \`${SOAK_SIGNOFF_REPORT_MD}\`"
  echo "- [ ] workflow logs: \`/tmp/mp_pg_replay_soak.log\` + \`/tmp/mp_pg_replay_soak_review.log\` + \`/tmp/mp_pg_replay_soak_signoff.log\`"
  echo "- [ ] rounds raw data: \`/tmp/mp_pg_replay_soak/rounds\`"
  echo "- [ ] history snapshot: \`/tmp/mp_pg_replay_history\`"
  echo
  echo "## Reviewer Sign-off"
  echo
  echo "- reviewer:"
  echo "- review_date_utc:"
  echo "- result: \`approved\` / \`hold_collect_more_data\` / \`hold_investigation_required\`"
  echo "- notes:"
} >"${SOAK_SIGNOFF_REPORT_MD}"

echo "summary: signoff_report_md=${SOAK_SIGNOFF_REPORT_MD}"
echo "summary: signoff_decision=${decision}"
echo "summary: signoff_reason=${decision_reason}"

if [[ "${SOAK_SIGNOFF_FAIL_ON_HOLD}" == "true" && "${decision}" != "ready_for_signoff" ]]; then
  echo "FAIL signoff: decision=${decision} fail_on_hold=true"
  exit 1
fi

echo "PASS: pg replay soak nightly signoff snapshot complete"
