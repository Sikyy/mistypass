# Enterprise HRIS Webhooks

当前能力状态：

- `CONTRACT_READY`：receipt 查询与手动处理合同已可联调，并覆盖当前 Talenta webhook receipt 队列的主要运行态字段。

## `GET /api/v1/enterprise/hris-webhook-receipts`

按租户查看 webhook receipt 队列现状。主要用于观察 receipt 当前是 `ready`、`cooldown`、`in_flight` 还是已进入终态，而不必只依赖后台 worker 日志。

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |
| `connector_id` | 否 | 过滤指定 connector |
| `limit` | 否 | `>= 0`；默认 `50` |

### Success（`200`）

```json
{
  "items": [
    {
      "id": "whr_abc123def456",
      "tenant_id": "tenant_demo_jakarta",
      "connector_id": "hrc_talenta_jakarta",
      "vendor": "talenta",
      "event_type": "talenta.employee.detail.updated",
      "request_id": "mekari-evt-001",
      "status": "failed",
      "attempt_count": 1,
      "last_error": "forced merge failure",
      "received_at": "2026-04-23T09:15:00Z",
      "last_attempt_at": "2026-04-23T09:16:00Z",
      "processed_at": "2026-04-23T09:16:00Z",
      "queue_state": "cooldown",
      "next_retry_at": "2026-04-23T09:21:00Z"
    }
  ]
}
```

运行态语义：

- `queue_state=ready`：当前已可被 receipt worker 重新 claim。
- `queue_state=cooldown`：仍处于 backoff 冷却窗口，`next_retry_at` 表示最早重试时间。
- `queue_state=in_flight`：当前仍处于 fresh `processing` 窗口，`processing_deadline_at` 表示 visibility timeout。
- `queue_state=attempt_limit`：已达到 worker `max_attempts`，通常下一步应看是否已 handoff 到 DLQ。
- `queue_state=terminal`：receipt 已进入 `processed/skipped/dlq` 等终态，不再属于 retry queue。

## `POST /api/v1/enterprise/hris-webhook/{connectorID}`

通用 HRIS webhook 接收入口。当前版本负责：

- 根据 `connectorID` 定位租户级 HRIS connector
- 校验 connector 是否存在且处于 `active`
- 对 `Talenta` webhook 先执行 Mekari HMAC 验签（`Authorization` / `Date` / `Digest`）
- 持久化原始 webhook receipt（headers / payload / request_id / event_type / source_ip）
- 在启用 receipt worker 时，按 `received -> processing -> processed/failed/skipped/dlq` 状态机推进 receipt
- 写入审计日志 `enterprise_hris_webhook_received`
- 对已支持的 `Talenta` 员工与排班事件尝试 inline `normalize -> merge(if sparse) -> employees/sync -> access upsert`
- 快速返回 `202 Accepted`

当前版本暂未实现：

- 独立异步队列
- 多厂商 normalizer 完整覆盖
- deferred 事件的生产级实时消费（当前仅显式落 `skipped + deferred audit`）

## 路径参数

- `connectorID`: Connector Registry 中的 connector ID，例如 `hrc_abc123def456`

## 请求头

`Talenta` 当前额外要求：

- `Authorization`: Mekari HMAC header
- `Date`: RFC7231 / HTTP-date
- `Digest`: `SHA-256=<base64 digest>`

当前会优先提取以下 header 作为 receipt 元数据：

- `X-Request-ID`
- `X-Webhook-ID`
- `X-Event-ID`
- `X-Event-Type`
- `X-Webhook-Event`
- `X-Mekari-Request-ID`
- `X-Mekari-Event`
- `X-Mekari-Webhook-Event`

同时会保留：

- `Content-Type`
- `User-Agent`
- 所有 `X-*` header

## Payload 限制

- 最大请求体：`1 MiB`

## 示例请求

```http
POST /api/v1/enterprise/hris-webhook/hrc_talenta_jakarta HTTP/1.1
Content-Type: application/json
Authorization: hmac username="mekari-client-id", algorithm="hmac-sha256", headers="date request-line", signature="..."
Date: Wed, 22 Apr 2026 09:15:00 GMT
Digest: SHA-256=...
X-Request-ID: mekari-evt-001
X-Event-Type: talenta.employee.detail.created

{
  "event_type": "talenta.employee.detail.created",
  "employee": {
    "id": "EMP-001",
    "email": "arief.putra@sudirman.co"
  }
}
```

## `202 Accepted`

```json
{
  "receipt_id": "whr_abc123def456",
  "connector_id": "hrc_talenta_jakarta",
  "vendor": "talenta",
  "event_type": "talenta.employee.detail.created",
  "request_id": "mekari-evt-001",
  "status": "received",
  "received_at": "2026-04-22T09:15:00Z"
}
```

处理语义补充：

- `Talenta` webhook 会先验签；签名失败、header 缺失或时间窗超出 `300` 秒不会落 receipt。
- 当前 `webhook_secret_ref` 用于解析验签 secret；`credential_ref` 如已配置，会额外用于比对 HMAC `client_id`。
- 当前内联消费覆盖 `talenta.employee.detail.created/updated/deleted`、`talenta.employee.transfer.approved/cancelled`、`talenta.employee.resignation.created/cancelled`、`talenta.attendance.scheduler.changeshift`、`talenta.attendance.scheduler.changeschedule`。
- `scheduler.changeshift` / `scheduler.changeschedule` 属于稀疏变更 payload，当前会先按 `external_id` / email merge 既有员工快照，再进入 enterprise sync。
- 不支持的事件会保留 receipt，并记录处理跳过审计日志，不会阻塞 `202 Accepted`。
- `Talenta attendance.liveattendance` 现已显式作为 deferred 事件处理：receipt 会落成 `skipped`，并记录 `enterprise_hris_webhook_processing_deferred` 审计，但不会进入 canonical employee sync 或 DLQ。
- 截止 2026-04-23，Mekari 公开 webhook 目录仍未见 Talenta `leave/time-off` 事件文档；当前不会硬编码猜测 event name。
- 若 inline 或 receipt worker 路径上的 `normalize / merge / sync` 失败，当前版本仍优先返回 `202 Accepted`。
- 启用 receipt worker 时，receipt 会先在 receipt retry queue 内按 `attempt_count + retry_cooldown(base) + retry_max_backoff(max)` 做指数退避重试；达到 `max_attempts` 且 DLQ append 成功后，receipt 状态会转为 `dlq` 并退出 retry queue，后续仅由 DLQ replay / 后台 DLQ worker 接管。
- `processing` receipt 会按 `ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_PROCESSING_TIMEOUT` 作为最小 visibility timeout 处理；未超时视为 in-flight，超时后可被 worker 重新接管，默认 `5m`。
- receipt worker 处理前会先执行 service 层原子 claim，在同一锁内校验 `attempt_limit / exponential retry cooldown / in_flight`，claim 成功后才递增 `attempt_count` 并继续处理。
- 若后台 receipt worker 已接入 Redis lease，多实例部署下会先对 tick 做互斥；未接 Redis 时保持兼容运行，但不保证跨实例互斥。
- 若 exhausted 时 DLQ append 失败，receipt 会保留为 `failed`，避免出现“标成 dlq 但实际没进 DLQ”的假终态。

## 错误响应

- `404 Not Found`: connector 不存在
- `409 Conflict`: connector 已禁用（`inactive`）
- `401 Unauthorized`: `Talenta` webhook 签名无效、header 缺失或 `Date` 超出允许时间窗
- `413 Payload Too Large`: webhook body 超过 `1 MiB`
- `500 Internal Server Error`: `Talenta` connector 缺少可解析的验签 secret
