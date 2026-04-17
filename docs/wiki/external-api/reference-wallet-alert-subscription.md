# API Reference：Wallet Alert Subscription

当前能力状态：

- `CONTRACT_READY`：告警订阅读写接口字段已稳定。
- `PROD_READY`：默认值回退、通道组合校验、持久化回读有回归覆盖。

## 1. `GET /api/v1/wallet/jobs/alert-subscription`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 限制 |

### Success（`200`）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "enabled": true,
  "dlq_alert_threshold": 7,
  "window_seconds": 1200,
  "cooldown_seconds": 900,
  "channels": {
    "email": true,
    "whatsapp": false
  },
  "receiver_groups": ["security"],
  "updated_at": "2026-04-15T10:20:00Z"
}
```

说明：若租户尚未配置，服务端会返回默认策略（不是 `404`）。

## 2. `PUT /api/v1/wallet/jobs/alert-subscription`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "enabled": true,
  "dlq_alert_threshold": 2,
  "window_seconds": 300,
  "cooldown_seconds": 120,
  "channels": {
    "email": true,
    "whatsapp": true
  },
  "receiver_groups": ["security", "ops"],
  "actor": "ops.bot"
}
```

### 参数规则

- `dlq_alert_threshold`：`1..100000`
- `window_seconds`：`1..604800`（7 天）
- `cooldown_seconds`：`0..604800`（7 天）
- `receiver_groups`：最多 20 个；空数组会回退为 `["security"]`
- 当 `enabled=true` 时，`channels.email` 与 `channels.whatsapp` 不能同时为 `false`

未显式传入的字段，会沿用当前订阅值（若首次配置则沿用默认值）。

### Success（`200`）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "enabled": true,
  "dlq_alert_threshold": 2,
  "window_seconds": 300,
  "cooldown_seconds": 120,
  "channels": {
    "email": true,
    "whatsapp": true
  },
  "receiver_groups": ["security", "ops"],
  "updated_at": "2026-04-15T10:25:00Z"
}
```

## 3. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `invalid wallet job alert subscription options` | 阈值/窗口/通道策略非法 |
| `403` | `tenant scope forbidden` | 租户越权 |
| `500` | internal error | 服务端异常 |

## 4. 相关接口

- 告警评估与发送：`POST /api/v1/wallet/jobs/alerts/dispatch`
- 告警发送记录：`GET /api/v1/wallet/jobs/alert-notifications`
- 告警失败重试：`POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry`

## 5. 回归脚本

- `docs/testing/curl-wallet-job-alert-subscription.zsh`

