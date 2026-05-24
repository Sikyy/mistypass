#!/usr/bin/env zsh
set -euo pipefail

SCRIPT_DIR="${0:A:h}"
CURVE_SCRIPT="${CURVE_SCRIPT:-${SCRIPT_DIR}/curl-pg-replay-multi-state-curve.zsh}"
SOAK_ROUNDS="${SOAK_ROUNDS:-4}"
SOAK_INTERVAL_SECONDS="${SOAK_INTERVAL_SECONDS:-30}"
SOAK_WORKDIR="${SOAK_WORKDIR:-/tmp/mp_pg_replay_soak}"
SOAK_LOG_CSV="${SOAK_LOG_CSV:-${SOAK_WORKDIR}/metrics.csv}"
SOAK_DROP_RATIO_MAX="${SOAK_DROP_RATIO_MAX:-0.5}"

mkdir -p "${SOAK_WORKDIR}"
mkdir -p "${SOAK_WORKDIR}/rounds"

if [[ ! -x "${CURVE_SCRIPT}" ]]; then
  echo "FAIL prereq: curve script is not executable -> ${CURVE_SCRIPT}"
  exit 1
fi

if [[ "${SOAK_ROUNDS}" -lt 1 ]]; then
  echo "FAIL config: SOAK_ROUNDS must be >= 1"
  exit 1
fi

if [[ "${SOAK_INTERVAL_SECONDS}" -lt 0 ]]; then
  echo "FAIL config: SOAK_INTERVAL_SECONDS must be >= 0"
  exit 1
fi

echo "round,started_at,level,state_key,delta,catchup_applied,catchup_latency_ms,throughput_ops_per_sec,noop_p95_ms,noop_max_ms,min_ops_threshold,p95_threshold_ms,max_threshold_ms,status,error" >"${SOAK_LOG_CSV}"

typeset -A first_throughput
typeset -A latest_throughput

for round in $(seq 1 "${SOAK_ROUNDS}"); do
  started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  round_csv="${SOAK_WORKDIR}/rounds/round_${round}.csv"
  round_log="${SOAK_WORKDIR}/rounds/round_${round}.log"

  echo "== soak round ${round}/${SOAK_ROUNDS} (${started_at}) =="
  set +e
  RESULT_CSV="${round_csv}" /bin/zsh "${CURVE_SCRIPT}" >"${round_log}" 2>&1
  rc="$?"
  set -e

  if [[ "${rc}" -ne 0 ]]; then
    echo "FAIL soak round ${round}: curve script exit code=${rc}"
    tail -n 80 "${round_log}" || true
    echo "${round},${started_at},-,-,0,0,0,0,0,0,0,0,0,failed,curve_exit_${rc}" >>"${SOAK_LOG_CSV}"
    exit 1
  fi

  if [[ ! -s "${round_csv}" ]]; then
    echo "FAIL soak round ${round}: missing round csv -> ${round_csv}"
    exit 1
  fi

  summary_rows=0
  while IFS=, read -r level state_key delta catchup_applied catchup_latency throughput noop_workers noop_failures noop_p95 noop_max min_ops p95_threshold max_threshold; do
    if [[ "${level}" == "level" ]]; then
      continue
    fi
    summary_rows=$((summary_rows + 1))
    echo "${round},${started_at},${level},${state_key},${delta},${catchup_applied},${catchup_latency},${throughput},${noop_p95},${noop_max},${min_ops},${p95_threshold},${max_threshold},passed," >>"${SOAK_LOG_CSV}"

    metric_key="${level}|${state_key}"
    if [[ -z "${first_throughput[$metric_key]-}" ]]; then
      first_throughput[$metric_key]="${throughput}"
    fi
    latest_throughput[$metric_key]="${throughput}"
  done <"${round_csv}"

  if [[ "${summary_rows}" -eq 0 ]]; then
    echo "FAIL soak round ${round}: no summary rows found in ${round_csv}"
    exit 1
  fi

  if [[ "${round}" -lt "${SOAK_ROUNDS}" && "${SOAK_INTERVAL_SECONDS}" -gt 0 ]]; then
    sleep "${SOAK_INTERVAL_SECONDS}"
  fi
done

echo "== soak trend guard (throughput drop ratio max=${SOAK_DROP_RATIO_MAX}) =="
for metric_key in ${(k)latest_throughput}; do
  first="${first_throughput[$metric_key]}"
  last="${latest_throughput[$metric_key]}"
  if awk -v first="${first}" -v last="${last}" -v drop="${SOAK_DROP_RATIO_MAX}" 'BEGIN { threshold = first * (1 - drop); exit !(last + 0 < threshold + 0) }'; then
    echo "FAIL trend ${metric_key}: first=${first} last=${last} drop_ratio_max=${SOAK_DROP_RATIO_MAX}"
    exit 1
  fi
  echo "trend: ${metric_key} first=${first} last=${last}"
done

echo "== soak aggregate summary =="
awk -F, '
NR==1 { next }
$14=="passed" {
  key=$3 "|" $4
  count[key]++
  sum_tp[key]+=$8
  if (!(key in min_tp) || $8 < min_tp[key]) min_tp[key]=$8
  if ($8 > max_tp[key]) max_tp[key]=$8
  if ($9 > max_p95[key]) max_p95[key]=$9
  if ($10 > max_noop[key]) max_noop[key]=$10
}
END {
  printf "metric_key,count,avg_throughput,min_throughput,max_throughput,max_noop_p95_ms,max_noop_max_ms\n"
  for (key in count) {
    avg = sum_tp[key] / count[key]
    printf "%s,%d,%.2f,%.2f,%.2f,%d,%d\n", key, count[key], avg, min_tp[key], max_tp[key], max_p95[key], max_noop[key]
  }
}
' "${SOAK_LOG_CSV}"

echo "summary: soak_log_csv=${SOAK_LOG_CSV}"
echo "summary: soak_workdir=${SOAK_WORKDIR}"
echo "PASS: pg replay multi-state soak baseline complete"
