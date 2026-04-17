# Guide：审计 Webhook Fan-out

当前能力状态：

- `CONTRACT_READY`：Webhook 配置、手动分发、投递记录接口可联调。
- `PROD_READY`：失败留档、动作过滤、冲突语义均有回归覆盖。

## 1. 目标

将审计事件按租户策略推送到企业侧系统，并可追踪投递结果。

相关 API：

- `GET /api/v1/audit/webhook/config`
- `PUT /api/v1/audit/webhook/config`
- `POST /api/v1/audit/webhook/dispatch`
- `GET /api/v1/audit/webhook/deliveries`

## 2. 配置 Webhook

`PUT /api/v1/audit/webhook/config`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "enabled": true,
  "endpoint": "https://example.com/hooks/mistypass-audit",
  "actions": ["gateway_reboot", "tenant_update"],
  "updated_by": "integration.bot"
}
```

字段说明：

- `enabled=true` 时必须提供合法 `http://` 或 `https://` endpoint。
- `actions` 为空表示不过滤动作；非空时仅推送白名单动作。

## 3. 触发分发

`POST /api/v1/audit/webhook/dispatch`

可以两种方式选择事件：

1. 指定 `audit_log_id`。
2. 传 `action/source`，服务端选符合条件的最新一条。

示例：

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "audit_log_id": "aud_3002"
}
```

成功响应：

- `delivery`：投递记录（状态、HTTP 状态码、错误、响应片段）。
- `event`：对应审计事件。

## 4. 服务端发送头

发送 webhook 时会带：

- `Content-Type: application/json`
- `X-MistyPass-Event-ID`
- `X-MistyPass-Event-Action`

请求体包含：

- `tenant_id`
- `event`（审计事件对象）
- `sent_at`

## 5. 常见失败语义

| HTTP | 错误 | 场景 |
|---|---|---|
| `404` | `audit webhook config not found` | 尚未配置 |
| `409` | `audit webhook is disabled` | 配置禁用 |
| `409` | `audit webhook action is filtered` | 动作被过滤 |
| `502` | `webhook request failed` / `webhook returned status ...` | 下游不可达或返回非 2xx |

即使返回 `502`，仍会写入 `delivery` 记录，便于补偿。

## 6. 查询投递记录

`GET /api/v1/audit/webhook/deliveries?tenant_id=tenant_demo_jakarta&limit=20`

可用字段：

- `status`：`success/failed`
- `http_status`
- `error`
- `response_body`
- `dispatched_at`

## 7. 联调建议

1. 先以 `enabled=false` 保存配置验证格式。
2. 再切 `enabled=true`，用可控 mock endpoint 做 2xx/5xx 场景测试。
3. 所有异常以 `deliveries` 为准做重试与告警。

## 8. 回归脚本

- `docs/testing/curl-audit-webhook-fanout.zsh`

## 9. 相关文档

- `docs/wiki/external-api/errors-and-reliability.md`
- `docs/wiki/external-api/changelog-and-migration.md`
