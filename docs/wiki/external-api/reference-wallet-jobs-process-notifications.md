# API Reference：Wallet Jobs Process / Alert Notifications

当前能力状态：

- `CONTRACT_READY`：`jobs/process` 与 `alert-notifications` 查询/重试接口字段和状态码已稳定。
- `PROD_READY`：队列处理参数边界、失败重试幂等、通道回执与告警留档有回归覆盖。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| Jobs List | `GET /api/v1/wallet/jobs`、`GET /api/v1/wallet/jobs/{jobID}`、`GET /api/v1/wallet/jobs/summary` | `super_admin/tenant_admin/operator` |
| Jobs Process | `POST /api/v1/wallet/jobs/process` | `super_admin/tenant_admin` |
| Alert Notifications | `GET /api/v1/wallet/jobs/alert-notifications` | `super_admin/tenant_admin/operator` |
| Alert Notification Retry | `POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry` | `super_admin/tenant_admin` |

## 2. Jobs 查询

### 2.1 `GET /api/v1/wallet/jobs`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |

### Success（`200`）

```json
{
  "items": [
    {
      "id": "wjb_001",
      "tenant_id": "tenant_demo_jakarta",
      "provider": "google_wallet",
      "batch_id": "wb_20260415_001",
      "template_id": "wpt_001",
      "target_type": "user",
      "target_id": "usr_1001",
      "status": "pending",
      "retry_count": 0,
      "error_code": "",
      "error_message": "",
      "created_at": "2026-04-15T10:00:00Z",
      "updated_at": "2026-04-15T10:00:00Z"
    }
  ]
}
```

`status` 常见值：`pending` / `processing` / `success` / `failed` / `dlq`。

### 2.2 `GET /api/v1/wallet/jobs/{jobID}`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |

### Success（`200`）

返回单条 `IssueJob` 记录（字段同上）。

### 2.3 `GET /api/v1/wallet/jobs/summary`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |
| `max_retry` | 否 | `>=0`；`0` 使用系统默认值 |

### Success（`200`）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "max_retry": 3,
  "total": 12,
  "pending": 2,
  "processing": 0,
  "success": 7,
  "failed": 1,
  "dlq": 2,
  "retryable_failed": 1,
  "non_retryable_failed": 2,
  "error_code_breakdown": {
    "template_inactive": 2
  },
  "updated_at": "2026-04-15T10:30:00Z"
}
```

## 3. `POST /api/v1/wallet/jobs/process`

用途：批量拉取并处理当前租户 `pending/failed` 可处理任务。

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "limit": 20,
  "worker_count": 2,
  "max_retry": 3,
  "base_backoff_ms": 200,
  "max_backoff_ms": 5000,
  "actor": "wallet-worker"
}
```

### 参数规则

- `limit`：`1..500`，缺省按服务默认值（通常 `20`）。
- `worker_count`：`1..32`，缺省按服务默认值（通常 `2`）。
- `max_retry`：`1..20`，`<=0` 时回退到系统默认值。
- `base_backoff_ms` / `max_backoff_ms`：若传入需满足 `max_backoff >= base_backoff`。

### Success（`200`）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "limit": 20,
  "worker_count": 2,
  "max_retry": 3,
  "claimed": 2,
  "succeeded": 2,
  "failed": 0,
  "dlq": 0,
  "skipped": 0,
  "retried": 0,
  "pending_after": 0,
  "processed_job_ids": ["wjb_001", "wjb_002"],
  "started_at": "2026-04-15T10:31:00Z",
  "completed_at": "2026-04-15T10:31:01Z"
}
```

## 4. Alert Notifications

### 4.1 `GET /api/v1/wallet/jobs/alert-notifications`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |
| `limit` | 否 | `1..500`，默认 `50` |

### Success（`200`）

```json
{
  "items": [
    {
      "id": "wan_001",
      "tenant_id": "tenant_demo_jakarta",
      "type": "dlq_error_code_threshold",
      "error_code": "template_inactive",
      "count": 3,
      "threshold": 1,
      "channels": ["email"],
      "receiver_groups": ["security"],
      "status": "failed",
      "reason": "provider_transient_error",
      "idempotency_key": "walert_tenant_demo_jakarta_dlq_error_code_threshold_template_inactive_1",
      "attempt": 1,
      "retryable": true,
      "provider": "mock",
      "provider_error": "provider_unavailable",
      "channel_results": [
        {
          "channel": "email",
          "status": "failed",
          "reason": "provider_transient_error",
          "provider": "mock",
          "provider_error": "provider_unavailable",
          "retryable": true,
          "receivers": ["security@mistypass.local"]
        }
      ],
      "triggered_at": "2026-04-15T10:32:00Z"
    }
  ]
}
```

`status` 常见值：`sent` / `failed` / `skipped`。

### 4.2 `POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "actor": "ops.retry"
}
```

### Success（`200`）

```json
{
  "id": "wan_002",
  "tenant_id": "tenant_demo_jakarta",
  "status": "sent",
  "attempt": 2,
  "source_notification_id": "wan_001",
  "retryable": false,
  "channel_results": [
    {
      "channel": "email",
      "status": "sent",
      "provider": "mock",
      "retryable": false,
      "receivers": ["security@mistypass.local"]
    }
  ],
  "triggered_at": "2026-04-15T10:33:00Z"
}
```

重试语义：

- 仅 `status=failed && retryable=true` 的通知允许重试。
- 已有同 `idempotency_key` 的成功发送时，会返回 `status=skipped`、`reason=idempotent_already_sent`。

## 5. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `invalid wallet job process options` | `jobs/process` 参数越界或退避参数非法 |
| `400` | `limit must be an integer > 0` / `limit must be <= 500` | 通知列表 `limit` 非法 |
| `400` | `notificationID is required` | retry 路径参数缺失 |
| `400` | `max_retry must be a non-negative integer` | summary 参数非法 |
| `404` | `job not found` | 查询 job 不存在 |
| `404` | `wallet job alert notification not found` | retry 目标不存在 |
| `409` | `wallet job alert retry not allowed` | 通知不可重试 |
| `403` | `tenant scope forbidden` | 租户越权 |

## 6. 回归脚本

- `docs/testing/curl-wallet-job-queue-process.zsh`
- `docs/testing/curl-wallet-job-alert-dispatch-retry.zsh`

## 7. 相关 Guide

- `docs/wiki/external-api/guides-wallet-queue-ops.md`
- `docs/wiki/external-api/guides-wallet-dlq-governance.md`
