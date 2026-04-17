# API Reference：Wallet Job Metrics / Trend

当前能力状态：

- `CONTRACT_READY`：`metrics` 与 `metrics/trend` 查询参数和响应结构已稳定。
- `PROD_READY`：阈值告警、时间桶聚合、参数边界（`bucket_count<=120`）有回归覆盖。

## 1. `GET /api/v1/wallet/jobs/metrics`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 限制 |
| `window_seconds` | 否 | `>0`；缺省使用系统默认窗口 |
| `max_retry` | 否 | `>=0`；`0` 表示使用默认重试次数 |
| `dlq_alert_threshold` | 否 | `>0`；缺省使用系统默认阈值 |

### Success（`200`）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "max_retry": 3,
  "dlq_alert_threshold": 1,
  "summary": {
    "total": 10,
    "pending": 0,
    "processing": 0,
    "success": 7,
    "failed": 1,
    "dlq": 2,
    "retryable_failed": 1,
    "non_retryable_failed": 2
  },
  "window": {
    "window_seconds": 600,
    "since": "2026-04-15T10:00:00Z",
    "until": "2026-04-15T10:10:00Z",
    "created": 3,
    "updated": 3,
    "pending": 0,
    "processing": 0,
    "success": 0,
    "failed": 1,
    "dlq": 2,
    "error_code_breakdown": {
      "template_inactive": 2
    }
  },
  "alerts": [
    {
      "type": "dlq_error_code_threshold",
      "error_code": "template_inactive",
      "count": 2,
      "threshold": 1
    }
  ],
  "updated_at": "2026-04-15T10:10:00Z"
}
```

## 2. `GET /api/v1/wallet/jobs/metrics/trend`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | 同上 |
| `window_seconds` | 否 | `>0` |
| `bucket_count` | 否 | `1..120`，默认 `12` |
| `max_retry` | 否 | `>=0` |
| `dlq_alert_threshold` | 否 | `>0` |

### Success（`200`）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "window_seconds": 600,
  "bucket_seconds": 120,
  "bucket_count": 5,
  "summary": {
    "total": 10,
    "dlq": 2
  },
  "alerts": [
    {
      "type": "dlq_error_code_threshold",
      "error_code": "template_inactive",
      "count": 2,
      "threshold": 1
    }
  ],
  "buckets": [
    {
      "index": 0,
      "start": "2026-04-15T10:00:00Z",
      "end": "2026-04-15T10:02:00Z",
      "created": 1,
      "updated": 1,
      "dlq": 1,
      "error_code_breakdown": {
        "template_inactive": 1
      }
    }
  ],
  "updated_at": "2026-04-15T10:10:00Z"
}
```

## 3. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `window_seconds must be an integer > 0` | 时间窗非法 |
| `400` | `max_retry must be a non-negative integer` | 重试参数非法 |
| `400` | `dlq_alert_threshold must be an integer > 0` | 阈值非法 |
| `400` | `bucket_count must be an integer > 0` | 桶数非法 |
| `400` | `bucket_count must be <= 120` | 桶数上限超出 |
| `403` | `tenant scope forbidden` | 租户越权 |

## 4. 回归脚本

- `docs/testing/curl-wallet-job-metrics-alert.zsh`
- `docs/testing/curl-wallet-job-metrics-trend.zsh`

