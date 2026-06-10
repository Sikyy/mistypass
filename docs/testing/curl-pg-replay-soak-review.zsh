#!/usr/bin/env zsh
set -euo pipefail

SOAK_REVIEW_ROOT="${SOAK_REVIEW_ROOT:-/tmp/mp_pg_replay_history}"
SOAK_REVIEW_WORKDIR="${SOAK_REVIEW_WORKDIR:-/tmp/mp_pg_replay_soak/review}"
SOAK_REVIEW_MIN_DAYS="${SOAK_REVIEW_MIN_DAYS:-7}"
SOAK_REVIEW_DROP_RATIO_MAX="${SOAK_REVIEW_DROP_RATIO_MAX:-0.5}"
SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS="${SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS:-1}"
SOAK_REVIEW_STRICT="${SOAK_REVIEW_STRICT:-false}"

SOAK_REVIEW_ALL_ROWS_CSV="${SOAK_REVIEW_ALL_ROWS_CSV:-${SOAK_REVIEW_WORKDIR}/all-rows.csv}"
SOAK_REVIEW_DAY_CSV="${SOAK_REVIEW_DAY_CSV:-${SOAK_REVIEW_WORKDIR}/day-summary.csv}"
SOAK_REVIEW_METRIC_CSV="${SOAK_REVIEW_METRIC_CSV:-${SOAK_REVIEW_WORKDIR}/metric-summary.csv}"
SOAK_REVIEW_REPORT_MD="${SOAK_REVIEW_REPORT_MD:-${SOAK_REVIEW_WORKDIR}/report.md}"
SOAK_REVIEW_SUMMARY_JSON="${SOAK_REVIEW_SUMMARY_JSON:-${SOAK_REVIEW_WORKDIR}/summary.json}"

mkdir -p "${SOAK_REVIEW_WORKDIR}"

if [[ ! -d "${SOAK_REVIEW_ROOT}" ]]; then
  echo "FAIL review: SOAK_REVIEW_ROOT does not exist -> ${SOAK_REVIEW_ROOT}"
  exit 1
fi

typeset -a metric_files
while IFS= read -r metrics_file_path; do
  metric_files+=("${metrics_file_path}")
done < <(find "${SOAK_REVIEW_ROOT}" -type f -name 'metrics.csv' | sort)

if [[ "${#metric_files[@]}" -eq 0 ]]; then
  echo "FAIL review: no metrics.csv found under ${SOAK_REVIEW_ROOT}"
  exit 1
fi

echo "source_file,round,started_at,level,state_key,delta,catchup_applied,catchup_latency_ms,throughput_ops_per_sec,noop_p95_ms,noop_max_ms,min_ops_threshold,p95_threshold_ms,max_threshold_ms,status,error" >"${SOAK_REVIEW_ALL_ROWS_CSV}"
for file in "${metric_files[@]}"; do
  awk -F, -v src="${file}" 'NR==1 { next } NF>=15 { print src "," $0 }' "${file}" >>"${SOAK_REVIEW_ALL_ROWS_CSV}"
done

total_rows="$(awk -F, 'NR>1 { c++ } END { print c+0 }' "${SOAK_REVIEW_ALL_ROWS_CSV}")"
if [[ "${total_rows}" -eq 0 ]]; then
  echo "FAIL review: merged csv has no data rows"
  exit 1
fi

failed_rows="$(awk -F, 'NR>1 && $15!="passed" { c++ } END { print c+0 }' "${SOAK_REVIEW_ALL_ROWS_CSV}")"
# Split failed rows by origin: the current run lives under .../current/, history
# under dated dirs. A failed row from a PAST nightly should inform the trend, not
# hard-fail every subsequent night — so only current-run failures gate the build.
current_failed_rows="$(awk -F, 'NR>1 && $1 ~ /\/current\// && $15!="passed" { c++ } END { print c+0 }' "${SOAK_REVIEW_ALL_ROWS_CSV}")"
history_failed_rows="$(awk -F, 'NR>1 && $1 !~ /\/current\// && $15!="passed" { c++ } END { print c+0 }' "${SOAK_REVIEW_ALL_ROWS_CSV}")"
unique_days="$(awk -F, 'NR>1 && $15=="passed" && length($3)>=10 { print substr($3,1,10) }' "${SOAK_REVIEW_ALL_ROWS_CSV}" | sort -u | wc -l | tr -d '[:space:]')"
coverage_stats="$(awk -F, '
function daynum(ymd, y, m, d, era, yoe, doy, doe) {
  split(ymd, part, "-")
  y = part[1] + 0
  m = part[2] + 0
  d = part[3] + 0
  if (m <= 2) {
    y -= 1
  }
  era = int((y >= 0 ? y : y - 399) / 400)
  yoe = y - era * 400
  doy = int((153 * (m + (m > 2 ? -3 : 9)) + 2) / 5) + d - 1
  doe = yoe * 365 + int(yoe / 4) - int(yoe / 100) + doy
  return era * 146097 + doe
}
NR==1 { next }
$15=="passed" && length($3)>=10 {
  day = substr($3, 1, 10)
  days[day] = 1
}
END {
  earliest = ""
  latest = ""
  count = 0
  for (day in days) {
    count++
    if (earliest == "" || day < earliest) {
      earliest = day
    }
    if (latest == "" || day > latest) {
      latest = day
    }
  }

  span = 0
  missing = 0
  ratio = 0
  if (count > 0) {
    span = daynum(latest) - daynum(earliest) + 1
    if (span < 1) {
      span = 1
    }
    missing = span - count
    if (missing < 0) {
      missing = 0
    }
    ratio = count / span
  }
  printf "%d,%s,%s,%d,%d,%.4f\n", count, earliest, latest, span, missing, ratio
}
' "${SOAK_REVIEW_ALL_ROWS_CSV}")"
IFS=, read -r coverage_unique_days coverage_earliest_day coverage_latest_day coverage_span_days coverage_missing_days coverage_ratio <<<"${coverage_stats}"

tmp_day_unsorted_csv="${SOAK_REVIEW_WORKDIR}/day-summary.unsorted.csv"
tmp_day_sorted_csv="${SOAK_REVIEW_WORKDIR}/day-summary.sorted.csv"

awk -F, '
NR==1 { next }
$15=="passed" {
  day = substr($3,1,10)
  metric = $4 "|" $5
  day_key = metric "|" day
  cnt[day_key]++
  tp_sum[day_key] += ($9 + 0)
  if (!(day_key in max_p95) || ($10 + 0) > max_p95[day_key]) {
    max_p95[day_key] = ($10 + 0)
  }
  if (!(day_key in max_noop) || ($11 + 0) > max_noop[day_key]) {
    max_noop[day_key] = ($11 + 0)
  }
}
END {
  for (key in cnt) {
    split(key, parts, "|")
    metric_key = parts[1] "|" parts[2]
    day = parts[3]
    avg = tp_sum[key] / cnt[key]
    printf "%s,%s,%d,%.2f,%d,%d\n", metric_key, day, cnt[key], avg, max_p95[key], max_noop[key]
  }
}
' "${SOAK_REVIEW_ALL_ROWS_CSV}" >"${tmp_day_unsorted_csv}"

sort -t, -k1,1 -k2,2 "${tmp_day_unsorted_csv}" >"${tmp_day_sorted_csv}"
echo "metric_key,date,sample_count,avg_throughput,max_noop_p95_ms,max_noop_max_ms" >"${SOAK_REVIEW_DAY_CSV}"
cat "${tmp_day_sorted_csv}" >>"${SOAK_REVIEW_DAY_CSV}"
rm -f "${tmp_day_unsorted_csv}" "${tmp_day_sorted_csv}"

echo "metric_key,day_count,sample_count,first_day,last_day,first_day_avg_throughput,last_day_avg_throughput,min_day_avg_throughput,max_day_avg_throughput,drop_ratio,max_noop_p95_ms,max_noop_max_ms,status" >"${SOAK_REVIEW_METRIC_CSV}"
awk -F, -v min_days="${SOAK_REVIEW_MIN_DAYS}" -v drop_max="${SOAK_REVIEW_DROP_RATIO_MAX}" '
NR==1 { next }
function reset_metric() {
  day_count = 0
  sample_count = 0
  first_day = ""
  last_day = ""
  first_avg = 0
  last_avg = 0
  min_avg = 0
  max_avg = 0
  max_p95 = 0
  max_noop = 0
}
function emit_metric(metric, drop_ratio, status) {
  if (metric == "") {
    return
  }
  drop_ratio = 0
  if (first_avg > 0) {
    drop_ratio = (first_avg - last_avg) / first_avg
  }
  status = "passed"
  if (day_count < min_days) {
    status = "insufficient_days"
  }
  if (drop_ratio > drop_max + 1e-9) {
    status = "failed_drop_ratio"
  }
  printf "%s,%d,%d,%s,%s,%.2f,%.2f,%.2f,%.2f,%.4f,%d,%d,%s\n", metric, day_count, sample_count, first_day, last_day, first_avg, last_avg, min_avg, max_avg, drop_ratio, max_p95, max_noop, status
}
BEGIN {
  current = ""
  reset_metric()
}
{
  metric = $1
  day = $2
  samples = ($3 + 0)
  avg = ($4 + 0)
  p95 = ($5 + 0)
  noop = ($6 + 0)

  if (current != "" && metric != current) {
    emit_metric(current)
    reset_metric()
  }
  if (current == "") {
    current = metric
  }

  day_count++
  sample_count += samples
  if (first_day == "") {
    first_day = day
    first_avg = avg
    min_avg = avg
    max_avg = avg
  }
  last_day = day
  last_avg = avg

  if (avg < min_avg) {
    min_avg = avg
  }
  if (avg > max_avg) {
    max_avg = avg
  }
  if (p95 > max_p95) {
    max_p95 = p95
  }
  if (noop > max_noop) {
    max_noop = noop
  }

  current = metric
}
END {
  emit_metric(current)
}
' "${SOAK_REVIEW_DAY_CSV}" >>"${SOAK_REVIEW_METRIC_CSV}"

earliest_day="$(awk -F, '
NR==1 { next }
{
  day = $2
  if (first == 0) {
    earliest = day
    first = 1
  }
  if (day < earliest) {
    earliest = day
  }
}
END { print earliest }
' "${SOAK_REVIEW_DAY_CSV}")"
latest_day="$(awk -F, '
NR==1 { next }
{
  day = $2
  if (first == 0) {
    latest = day
    first = 1
  }
  if (day > latest) {
    latest = day
  }
}
END { print latest }
' "${SOAK_REVIEW_DAY_CSV}")"
metric_count="$(awk -F, 'NR>1 { c++ } END { print c+0 }' "${SOAK_REVIEW_METRIC_CSV}")"
metric_fail_count="$(awk -F, 'NR>1 && $13 ~ /^failed_/ { c++ } END { print c+0 }' "${SOAK_REVIEW_METRIC_CSV}")"
metric_insufficient_count="$(awk -F, 'NR>1 && $13 == "insufficient_days" { c++ } END { print c+0 }' "${SOAK_REVIEW_METRIC_CSV}")"
metric_warning_count="$(awk -F, -v drop_max="${SOAK_REVIEW_DROP_RATIO_MAX}" '
NR>1 && $13=="passed" && ($10 + 0) >= (drop_max * 0.8) { c++ }
END { print c+0 }
' "${SOAK_REVIEW_METRIC_CSV}")"
days_gap=0
if [[ "${unique_days}" -lt "${SOAK_REVIEW_MIN_DAYS}" ]]; then
  days_gap=$((SOAK_REVIEW_MIN_DAYS - unique_days))
fi

coverage_days_gap=0
if [[ "${coverage_missing_days}" -gt "${SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS}" ]]; then
  coverage_days_gap=$((coverage_missing_days - SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS))
fi

overall_status="passed"
if [[ "${metric_fail_count}" -gt 0 || "${current_failed_rows}" -gt 0 ]]; then
  overall_status="failed"
elif [[ "${metric_insufficient_count}" -gt 0 || "${unique_days}" -lt "${SOAK_REVIEW_MIN_DAYS}" ]]; then
  overall_status="insufficient_days"
elif [[ "${coverage_missing_days}" -gt "${SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS}" ]]; then
  overall_status="insufficient_coverage"
fi

{
  echo "# PostgreSQL Replay Soak Nightly Review"
  echo
  echo "- status: \`${overall_status}\`"
  echo "- min_days_required: \`${SOAK_REVIEW_MIN_DAYS}\`"
  echo "- unique_days_observed: \`${unique_days}\`"
  echo "- coverage_span_days: \`${coverage_span_days}\`"
  echo "- missing_days_in_span: \`${coverage_missing_days}\`"
  echo "- max_allowed_missing_days: \`${SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS}\`"
  echo "- coverage_ratio: \`${coverage_ratio}\`"
  echo "- earliest_day: \`${earliest_day:-n/a}\`"
  echo "- latest_day: \`${latest_day:-n/a}\`"
  echo "- metrics_files: \`${#metric_files[@]}\`"
  echo "- merged_rows: \`${total_rows}\`"
  echo "- failed_rows: \`${failed_rows}\`"
  echo "- current_failed_rows: \`${current_failed_rows}\`"
  echo "- history_failed_rows: \`${history_failed_rows}\` (informational; does not fail the build)"
  echo "- metric_count: \`${metric_count}\`"
  echo "- metric_failed: \`${metric_fail_count}\`"
  echo "- metric_insufficient_days: \`${metric_insufficient_count}\`"
  echo "- metric_near_drop_threshold: \`${metric_warning_count}\` (>= 80% of drop ratio limit \`${SOAK_REVIEW_DROP_RATIO_MAX}\`)"
  echo "- days_gap_to_min_days: \`${days_gap}\`"
  echo "- coverage_days_gap_to_allowed: \`${coverage_days_gap}\`"
  echo
  echo "## Metric Summary"
  echo
  echo '```csv'
  cat "${SOAK_REVIEW_METRIC_CSV}"
  echo '```'
} >"${SOAK_REVIEW_REPORT_MD}"

cat >"${SOAK_REVIEW_SUMMARY_JSON}" <<EOF
{
  "status": "${overall_status}",
  "min_days_required": ${SOAK_REVIEW_MIN_DAYS},
  "unique_days_observed": ${unique_days},
  "days_gap_to_min_days": ${days_gap},
  "coverage_span_days": ${coverage_span_days},
  "missing_days_in_span": ${coverage_missing_days},
  "max_allowed_missing_days": ${SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS},
  "coverage_days_gap_to_allowed": ${coverage_days_gap},
  "coverage_ratio": ${coverage_ratio},
  "earliest_day": "${earliest_day:-}",
  "latest_day": "${latest_day:-}",
  "metrics_files": ${#metric_files[@]},
  "merged_rows": ${total_rows},
  "failed_rows": ${failed_rows},
  "current_failed_rows": ${current_failed_rows},
  "history_failed_rows": ${history_failed_rows},
  "metric_count": ${metric_count},
  "metric_failed": ${metric_fail_count},
  "metric_insufficient_days": ${metric_insufficient_count},
  "metric_near_drop_threshold": ${metric_warning_count},
  "drop_ratio_limit": ${SOAK_REVIEW_DROP_RATIO_MAX}
}
EOF

echo "summary: review_all_rows_csv=${SOAK_REVIEW_ALL_ROWS_CSV}"
echo "summary: review_day_csv=${SOAK_REVIEW_DAY_CSV}"
echo "summary: review_metric_csv=${SOAK_REVIEW_METRIC_CSV}"
echo "summary: review_report_md=${SOAK_REVIEW_REPORT_MD}"
echo "summary: review_summary_json=${SOAK_REVIEW_SUMMARY_JSON}"
echo "summary: review_status=${overall_status}"

if [[ "${overall_status}" == "failed" ]]; then
  echo "FAIL review: detected current-run failed rows or failed trend metrics"
  exit 1
fi
if [[ ("${overall_status}" == "insufficient_days" || "${overall_status}" == "insufficient_coverage") && "${SOAK_REVIEW_STRICT}" == "true" ]]; then
  echo "FAIL review: ${overall_status} with strict mode enabled"
  exit 1
fi

echo "PASS: pg replay soak nightly review complete"
