# API Reference：`POST /api/v1/wallet/jobs/alerts/dispatch`

当前能力状态：

- `CONTRACT_READY`：请求参数与响应结构已稳定。
- `PROD_READY`：冷却跳过、失败重试、通道回执均有回归覆盖。

## 1. Endpoint

- Method：`POST`
- Path：`/api/v1/wallet/jobs/alerts/dispatch`
- Auth：`Authorization: Bearer <access_token>`（`super_admin` / `tenant_admin`）

## 2. Request

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `tenant_id` | string | 是 | 租户 ID |
| `window_seconds` | int | 否 | 指标窗口，`>0` 时覆盖订阅默认值 |
| `max_retry` | int | 否 | `>0` 时覆盖默认最大重试次数 |
| `dlq_alert_threshold` | int | 否 | `>0` 时覆盖告警阈值 |
| `actor` | string | 否 | 审计操作者 |

示例：

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "window_seconds": 600,
  "max_retry": 3,
  "dlq_alert_threshold": 1,
  "actor": "ops.bot"
}
```

## 3. Success Response (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "window_seconds": 600,
  "max_retry": 3,
  "dlq_alert_threshold": 1,
  "total_alerts": 1,
  "dispatched": 1,
  "skipped": 0,
  "failed": 0,
  "items": [
    {
      "id": "wan_xxx",
      "type": "dlq_error_code_threshold",
      "error_code": "template_inactive",
      "status": "sent",
      "channel_results": [
        {
          "channel": "email",
          "status": "sent",
          "provider": "mock",
          "retryable": false,
          "receivers": ["security@mistypass.local"]
        }
      ]
    }
  ],
  "updated_at": "2026-04-15T09:00:00Z"
}
```

## 4. 关键字段语义

### 4.1 聚合统计

- `total_alerts`：本轮评估出的告警总数。
- `dispatched`：成功发送数量。
- `skipped`：被策略跳过数量（如 `cooldown`、`subscription_disabled`）。
- `failed`：发送失败数量。

### 4.2 `items[].status`

- `sent`
- `skipped`
- `failed`

### 4.3 `items[].channel_results[]`

按通道返回细粒度结果：

- `channel`：`email` / `whatsapp`
- `status`
- `provider`
- `reason` / `provider_error`
- `retryable`
- `receivers`

## 5. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `window_seconds must be an integer >= 0` 等 | 参数非法 |
| `403` | `tenant scope forbidden` | 租户作用域不匹配 |
| `500` | internal error | provider 或服务异常 |

## 6. 相关接口

- 订阅配置：`GET/PUT /api/v1/wallet/jobs/alert-subscription`
- 发送记录：`GET /api/v1/wallet/jobs/alert-notifications`
- 失败重试：`POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry`
