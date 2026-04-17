# PostgreSQL Replay Nightly 签字与证据归档说明（2026-04-14）

当前能力状态：

- `PROD_READY`：nightly workflow 已自动执行 soak、跨日 review、signoff 快照生成，并统一归档到 `api-replay-soak-nightly` artifact。
- `CONTRACT_READY`：签字模板与决策门口径已脚本化，可用于连续 7 天留档后的结项复核。

## 1. 目标

- 统一 R2 “>=7 天留档复核签字”执行口径。
- 避免每次人工整理字段与证据路径，降低结项准备成本。

## 2. 自动化产物

nightly workflow：`.github/workflows/api-replay-soak-nightly.yml`

- `metrics.csv`：每轮 soak 指标明细。
- `review/report.md`：跨日复核报告（覆盖度、跌幅、失败统计）。
- `review/summary.json`：机器可读复核摘要。
- `review/signoff.md`：签字快照（决策、证据清单、Reviewer 占位）。
- history 收集容错：若本轮 soak 未产出 `metrics.csv`，workflow 会输出 `current_metrics_present=false`，并继续基于历史 artifact 执行 review/signoff。

## 3. 签字脚本

脚本：`docs/testing/curl-pg-replay-soak-signoff.zsh`

默认输入：

- `SOAK_SIGNOFF_SUMMARY_JSON=/tmp/mp_pg_replay_soak/review/summary.json`
- `SOAK_SIGNOFF_DAY_CSV=/tmp/mp_pg_replay_soak/review/day-summary.csv`
- `SOAK_SIGNOFF_METRIC_CSV=/tmp/mp_pg_replay_soak/review/metric-summary.csv`

默认输出：

- `SOAK_SIGNOFF_REPORT_MD=/tmp/mp_pg_replay_soak/review/signoff.md`

示例：

```bash
SOAK_SIGNOFF_SUMMARY_JSON=/tmp/mp_pg_replay_soak/review/summary.json \
SOAK_SIGNOFF_REPORT_MD=/tmp/mp_pg_replay_soak/review/signoff.md \
/bin/zsh ./docs/testing/curl-pg-replay-soak-signoff.zsh
```

## 4. 决策口径

- `ready_for_signoff`
  - `status=passed`
  - `days_gap_to_min_days=0`
  - `failed_rows=0`
  - `metric_failed=0`
  - `metric_near_drop_threshold=0`
- `watch_near_threshold`
  - 通过但存在接近阈值指标（当前口径：`drop_ratio >= 80% * limit`）。
- `hold_collect_more_data`
  - 天数覆盖不足（`insufficient_days` 或 `days_gap_to_min_days>0`）。
  - 或日期连续覆盖不足（`insufficient_coverage` 或 `coverage_days_gap_to_allowed>0`）。
- `hold_investigation_required`
  - 任一失败指标或失败行出现。

workflow 门禁规则（nightly 默认）：

- 当 `days_gap_to_min_days > 0` 时，`SOAK_SIGNOFF_FAIL_ON_HOLD=false`（持续积累数据，不阻断）。
- 当 `days_gap_to_min_days == 0` 时，自动切换 `SOAK_SIGNOFF_FAIL_ON_HOLD=true`（若不是 `ready_for_signoff` 则 job 失败）。

连续覆盖守卫（review 脚本）：

- `SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS`（默认 `1`）用于限制 `earliest_day..latest_day` 区间中的缺失日期数。
- 当 `missing_days_in_span > max_allowed_missing_days` 时，review 状态标记为 `insufficient_coverage`。

## 5. 证据归档最小清单

- `mp_pg_replay_soak.log`
- `mp_pg_replay_soak_review.log`
- `mp_pg_replay_soak_signoff.log`
- `mp_pg_replay_soak/metrics.csv`
- `mp_pg_replay_soak/rounds/`
- `mp_pg_replay_soak/review/`
- `mp_pg_replay_history/`

## 6. 结项建议

- 连续满足 `ready_for_signoff` 后，再由 Reviewer 在 `signoff.md` 填写结果并回写 roadmap 结项记录。
