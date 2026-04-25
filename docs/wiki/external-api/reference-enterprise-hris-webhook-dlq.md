# API Reference：Enterprise HRIS Webhook DLQ

当前能力状态：

- `CONTRACT_READY`：DLQ 列表与 replay 合同已可联调，并覆盖当前 Talenta webhook 失败恢复的主要处置路径。
- `FOUNDATION_READY`：HRIS webhook 处理失败已可落 DLQ，并支持按条目手动 replay；receipt 达到 `max_attempts` 后也已支持显式 handoff 到 DLQ。
- `AUTO_RETRY_IN_PROGRESS`：后台 worker 已可按 `replay_count + retry_cooldown(base) + retry_max_backoff(max)` 自动重试 unresolved DLQ 条目，并已补 `processing_timeout + atomic claim` 以区分 fresh/stale `replaying`；独立异步队列仍待补齐。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| HRIS Webhook DLQ | `GET /api/v1/enterprise/hris-webhook-dlq` | `super_admin/tenant_admin/operator` |
| HRIS Webhook DLQ | `POST /api/v1/enterprise/hris-webhook-dlq/{entryID}/replay` | `super_admin/tenant_admin` |

说明：

- 仅“receipt 已持久化且 `normalize / merge / sync` 失败”的 webhook 会进入 DLQ。
- 签名校验失败的请求不会落 receipt，也不会进入 DLQ。
- 手动 replay 会重新执行当前版本的 `normalizer -> employees/sync -> access upsert` 链路。
- 手动 replay 与后台 worker 共用同一套 claim 语义：会先把条目标记为 `replaying`，再执行实际处理。
- 启用 receipt worker 时，receipt 会先在 retry queue 内重试；达到 `max_attempts` 且 DLQ append 成功后，receipt 状态会转为 `dlq` 并退出 retry queue。
- 若启用后台 worker，未解决的 `dlq` 条目会被自动重试。
- 自动 retry 的 cooldown 现按 `retry_cooldown(base)` 起算，并在多次失败时按 `retry_max_backoff(max)` 做指数退避上限控制。
- fresh `replaying` 会被视为 in-flight 跳过；超时 `replaying` 会基于 `processing_timeout` 被重新接管。
- 若后台 DLQ worker 已接入 Redis lease，多实例部署下会先对 tick 做互斥；未接 Redis 时保持兼容运行，但不保证跨实例互斥。

## 2. `GET /api/v1/enterprise/hris-webhook-dlq`

- Auth：`Authorization: Bearer <access_token>`

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
      "id": "hdlq_abc123def456",
      "tenant_id": "tenant_demo_jakarta",
      "connector_id": "hrc_gadjian_jakarta",
      "vendor": "gadjian",
      "receipt_id": "whr_abc123def456",
      "request_id": "gadjian-webhook-001",
      "event_type": "employee.updated",
      "failure_stage": "normalize",
      "error": "forced webhook normalization failure",
      "raw_payload_ref": "hris_webhook_receipt:whr_abc123def456",
      "status": "dlq",
      "replay_count": 0,
      "replay_state": "ready",
      "created_at": "2026-04-22T10:05:00Z",
      "updated_at": "2026-04-22T10:05:00Z"
    }
  ]
}
```

运行态语义：

- `replay_state=ready`：当前已可被手动 replay 或后台 DLQ worker claim。
- `replay_state=cooldown`：仍处于 backoff 冷却窗口，`next_retry_at` 表示最早重试时间。
- `replay_state=in_flight`：条目仍处于 fresh `replaying` 窗口，`processing_deadline_at` 表示最晚接管时间。
- `replay_state=attempt_limit`：已达到 worker `max_attempts`，需要人工处置。
- `replay_state=terminal`：条目已 `resolved`，不再参与 DLQ retry。

## 3. `POST /api/v1/enterprise/hris-webhook-dlq/{entryID}/replay`

用途：对单条 DLQ 记录重新执行 webhook 处理链路。

### Success（`200`）

```json
{
  "item": {
    "id": "hdlq_abc123def456",
    "tenant_id": "tenant_demo_jakarta",
    "connector_id": "hrc_gadjian_jakarta",
    "vendor": "gadjian",
    "receipt_id": "whr_abc123def456",
    "request_id": "gadjian-webhook-001",
    "event_type": "employee.updated",
    "failure_stage": "normalize",
    "error": "forced webhook normalization failure",
    "raw_payload_ref": "hris_webhook_receipt:whr_abc123def456",
    "status": "resolved",
    "replay_count": 1,
    "last_replay_at": "2026-04-22T10:10:00Z",
    "resolved_at": "2026-04-22T10:10:00Z",
    "created_at": "2026-04-22T10:05:00Z",
    "updated_at": "2026-04-22T10:10:00Z"
  }
}
```

语义说明：

- replay 成功后，条目状态从 `dlq` 变为 `resolved`。
- replay 进行中时，条目会短暂进入 `replaying` 过渡状态。
- replay 失败时，条目保留在 `dlq`，并刷新 `error`、`replay_count`、`last_replay_at`。
- 自动 retry 与手动 replay 共用同一处理链路与同一组计数器。
- receipt 的 `status=dlq` 表示它已从 receipt retry queue handoff 到 DLQ；这与 DLQ 条目的 `status=resolved` 是两套独立状态。

## 4. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `limit must be an integer >= 0` | `limit` 非法 |
| `404` | `hris dlq entry not found` | 条目不存在 |
| `404` | `hris connector not found` | 关联 receipt 无法定位 |
| `409` | `hris dlq entry replay is already in progress` | 条目已有 fresh `replaying` 处理在途 |
| `409` | `hris dlq entry cannot be replayed` | 条目已是终态或不允许 replay |
| `409` | `hris dlq entry cannot be replayed without receipt_id` | 条目不可重放 |
| `500` | `...` | replay 处理链路再次失败 |
