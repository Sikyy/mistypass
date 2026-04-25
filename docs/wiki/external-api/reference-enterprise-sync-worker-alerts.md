# API Reference：Enterprise Sync Worker Alerts

当前能力状态：

- `CONTRACT_READY`：已补 `sync-worker-alert-subscription` 订阅策略合同，支持 tenant-scoped `GET/PUT`、默认值回退与持久化。
- `CONTRACT_READY`：结构化告警列表与聚合摘要接口已稳定，当前统一覆盖 `enterprise sync reconcile`、`HRIS webhook receipt queue`、`HRIS webhook DLQ replay`、`HRIS webhook processing` 与 `HRIS pull reconcile` 五类后台 worker。
- `CONTRACT_READY`：已补 `POST /api/v1/enterprise/sync-worker-alerts/dispatch` 最小通知执行闭环，按订阅窗口、threshold、cooldown 与 channel 配置执行一次手动派发。
- `CONTRACT_READY`：已补 `GET /api/v1/enterprise/sync-worker-alerts/notifications`，提供 tenant-scoped 通知历史查询，并支持服务端 `q/status/reason/retryable/due_now/offset/limit` 过滤、分页与 filter/status counts；`manual_suppressed` 历史还会显式返回 `restore_status`。
- `CONTRACT_READY`：已补 `GET /api/v1/enterprise/sync-worker-alerts/notifications/export-csv`，支持按当前过滤条件导出完整通知历史 CSV。
- `CONTRACT_READY`：已补 `POST /api/v1/enterprise/sync-worker-alerts/notifications/{notificationID}/retry`，提供针对失败且可重试通知的手动 retry。
- `CONTRACT_READY`：已补 `POST /api/v1/enterprise/sync-worker-alerts/notifications/retry-batch`，支持在当前执行历史视图里做批量 retry，并按 fingerprint 抑制同类重复项。
- `CONTRACT_READY`：已补 `POST /api/v1/enterprise/sync-worker-alerts/notifications/suppress-batch`，支持把当前失败通知显式标记为 `manual_suppressed`，用于人工收口批量处置。
- `CONTRACT_READY`：已补 `POST /api/v1/enterprise/sync-worker-alerts/notifications/restore-batch`，支持把 `manual_suppressed` 通知恢复为新的 `failed + retryable` history，供后续手动 retry 或 auto-retry 继续接管；当前仅 `restore_status=ready` 的最新 suppression 会被真正放行。
- `CONTRACT_READY`：已补 `POST /api/v1/enterprise/sync-worker-alerts/notifications/auto-retry`，显式 HTTP due-now 扫描仍保留；后台也可由配置化 ticker worker 周期复用同一 service 语义，并对失败可重试记录落最小 backoff 语义。多实例部署下，后台 ticker worker 会优先通过 Redis lease 对单次 tick 做互斥；未接 Redis 时仍保持兼容运行，但不保证跨实例互斥。
- `CONTRACT_READY`：dispatch 候选与 cooldown 现已按 `worker_action + connector_id + vendor + failure_stage + mode + event_type` 的 granular bucket 对齐；`/api/v1/enterprise/sync-worker-alerts/summary` 仍保持 `worker_action` 级聚合，并对 `HRIS pull` 告警补充可选的 `last_consecutive_failures / last_failure_age_seconds` stateful 指标。
- `CONTRACT_READY`：wallet/provider 发送结果现会保留 `provider_delivery_id / provider_delivery_status`；`notifications list/export-csv/dispatch/retry-batch/auto-retry/single retry` 入口会先做最小 provider reconciliation。若 latest `dispatch_commit_unknown` 能被 provider receipt 确认，服务端会追加新的 `sent + reason=dispatch_commit_confirmed` history，并刷新该 lineage 的 cooldown；若 receipt 明确返回负向终态，则会追加新的 `failed + reason=dispatch_commit_rejected` history；若长时间仍停留在 `accepted/queued/not_found` 等非终态，则会追加新的 `failed + reason=dispatch_commit_confirmation_timeout` history。
- `PROD_READY`：时间窗过滤、limit 校验、多 action 聚合与告警字段解析有回归覆盖。

## 1. `GET /api/v1/enterprise/sync-worker-alert-subscription`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 条件必填 | `super_admin` 必填；其他角色默认使用 token tenant |

### Success (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "enabled": true,
  "worker_alert_threshold": 3,
  "window_seconds": 900,
  "cooldown_seconds": 900,
  "channels": {
    "email": true,
    "whatsapp": false
  },
  "receiver_groups": ["security"],
  "updated_at": "2026-04-22T10:00:00Z"
}
```

合同边界：

- `/summary` 继续返回 action 级聚合视图，不展开 granular dispatch bucket。
- dispatch / notifications 内部使用更细粒度 fingerprint 做候选去重与 cooldown 抑制，但不改变 `/summary` 的返回字段和聚合键。

默认回退：

- 当租户尚未保存订阅策略时，接口返回默认值，而不是 `404`。
- 默认值为 `enabled=true`、`worker_alert_threshold=3`、`window_seconds=900`、`cooldown_seconds=900`、`channels.email=true`、`channels.whatsapp=false`、`receiver_groups=["security"]`。

## 2. `PUT /api/v1/enterprise/sync-worker-alert-subscription`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request Body

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "enabled": true,
  "worker_alert_threshold": 5,
  "window_seconds": 600,
  "cooldown_seconds": 1200,
  "channels": {
    "email": true,
    "whatsapp": true
  },
  "receiver_groups": ["security", "ops"]
}
```

更新语义：

- `enabled` 与 `channels.*` 支持部分更新；未传字段沿用当前值或默认值。
- `worker_alert_threshold`、`window_seconds`、`cooldown_seconds` 传 `0` 时沿用当前值或默认值。
- `receiver_groups` 未传时沿用当前值；传空数组时会回退为默认 `["security"]`。

### Validation

- `worker_alert_threshold`：`1..100000`
- `window_seconds`：`1..604800`
- `cooldown_seconds`：`0..604800`
- `receiver_groups`：最多 `20` 个，去重后持久化
- 当 `enabled=true` 时，`email/whatsapp` 不能同时为 `false`

### Success (`200`)

返回结构与 `GET` 一致。

## 3. `GET /api/v1/enterprise/sync-worker-alerts`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 限制 |
| `since` | 否 | RFC3339，起始时间 |
| `until` | 否 | RFC3339，结束时间，且 `since <= until` |
| `limit` | 否 | `>=0`，默认 `50` |

### Success (`200`)

```json
{
  "items": [
    {
      "id": "audit_001",
      "tenant_id": "tenant_demo_jakarta",
      "actor": "enterprise.sync.worker",
      "role": "system",
      "action": "enterprise_sync_reconcile_worker_alert",
      "worker_action": "enterprise_sync_reconcile_worker_alert",
      "worker_kind": "sync_reconcile",
      "worker_label": "Enterprise Sync Reconcile",
      "source": "enterprise_sync_worker",
      "at": "2026-04-15T10:00:00Z",
      "failed": 1,
      "threshold": 1,
      "processed": 1,
      "applied": 0,
      "skipped_by_attempt_limit": 0,
      "skipped_by_cooldown": 0,
      "raw_target": "failed=1 threshold=1 processed=1 applied=0 skipped_attempt_limit=0 skipped_cooldown=0"
    }
  ]
}
```

## 4. `GET /api/v1/enterprise/sync-worker-alerts/summary`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`
- Query：与上一个接口一致（`tenant_id/since/until/limit`）。

### Success (`200`)

```json
{
  "items": [
    {
      "tenant_id": "tenant_demo_jakarta",
      "worker_action": "enterprise_hris_pull_worker_alert",
      "worker_kind": "hris_pull",
      "worker_label": "HRIS Pull Reconcile",
      "count": 3,
      "first_seen_at": "2026-04-15T09:00:00Z",
      "last_seen_at": "2026-04-15T10:00:00Z",
      "last_failed": 1,
      "last_threshold": 1,
      "last_processed": 1,
      "last_applied": 0,
      "last_consecutive_failures": 3,
      "last_failure_age_seconds": 3600,
      "last_skipped_by_attempt_limit": 0,
      "last_skipped_by_cooldown": 0
    }
  ]
}
```

说明：

- `/summary` 继续按 `tenant_id + worker_action` 聚合，不展开 granular dispatch bucket。
- 对 `enterprise_hris_pull_worker_alert`，响应可能额外带 `last_consecutive_failures` 与 `last_failure_age_seconds`，用于区分“单轮失败”与“已连续失败多轮”的 stateful 风险。

## 5. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `tenant_id is required` | `super_admin` 未提供租户 |
| `400` | `invalid enterprise sync worker alert subscription options` | 订阅策略校验失败 |
| `400` | `since must be RFC3339 timestamp` | `since` 格式错误 |
| `400` | `until must be RFC3339 timestamp` | `until` 格式错误 |
| `400` | `since must be <= until` | 时间窗逆序 |
| `400` | `limit must be an integer >= 0` | worker alert list / summary 或 auto-retry body 的 `limit` 非法 |
| `400` | `limit must be an integer > 0` | notifications list 的 `limit` 非法 |
| `400` | `limit must be <= 500` | notifications list 的 `limit` 超出上限 |
| `400` | `offset must be an integer >= 0` | notifications list 的 `offset` 非法 |
| `400` | `status must be one of sent, failed, skipped` | notifications list 的 `status` 非法 |
| `400` | `retryable must be a boolean` | notifications list / export-csv 的 `retryable` 非法 |
| `400` | `due_now must be a boolean` | notifications list / export-csv 的 `due_now` 非法 |
| `400` | `notification_ids is required` | batch retry / suppress / restore 未提供 notification id 集合 |
| `400` | `notification_ids must contain at most 100 items` | batch retry / suppress / restore 的 id 数量超限 |
| `400` | `notificationID is required` | retry 路径参数缺失 |
| `400` | `max_attempts must be an integer >= 0` | auto-retry body 的 `max_attempts` 非法 |
| `400` | `base_backoff_ms must be an integer >= 0` | auto-retry body 的 `base_backoff_ms` 非法 |
| `400` | `max_backoff_ms must be an integer >= 0` | auto-retry body 的 `max_backoff_ms` 非法 |
| `403` | `tenant scope forbidden` | 租户越权 |
| `404` | `enterprise sync worker alert notification not found` | 单条 retry / batch retry / suppress / restore 引用的 notification 不存在 |
| `409` | `enterprise sync worker alert retry not allowed` | notification 不是 `failed + retryable` |

## 6. `POST /api/v1/enterprise/sync-worker-alerts/dispatch`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request Body

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "actor": "tenant.admin@sudirman.co",
  "worker_actions": ["enterprise_hris_pull_worker_alert"]
}
```

字段说明：

- `tenant_id`：`super_admin` 必填；其他角色默认使用 token tenant。
- `actor`：可选；未传时默认使用当前登录用户 email，仍为空时回退 `system`。
- `worker_actions`：可选；为空时按当前窗口内所有达到订阅阈值的 worker action 派发，传值时仅允许现有 worker alert action 集合。

### Dispatch Semantics

- 服务端会先读取当前租户 `sync-worker-alert-subscription`；未保存时使用默认订阅策略。
- 仅统计“当前时间 - window_seconds”到“当前时间”窗口内的 worker alert 审计事件。
- 只有达到订阅阈值的 worker alert bucket 才会进入派发候选集；外部 API 仍以 `worker_action` 作为选择键，不新增 bucket 级请求参数。
- 每条候选集会生成持久化 notification record，并按 `worker_action + connector_id + vendor + failure_stage + mode + event_type` 组成的 fingerprint 执行 cooldown。
- 对同一 lineage 已有进行中的 provider 派发时，服务端会把当前候选计入 `skipped`，`reason=dispatch_inflight`，且不会额外落新的 notification history。
- 已过期的 in-flight lease 不再只是被动清理；若 provider 派发后的最终写回缺失，服务端会先把它恢复成 `failed + retryable + reason=dispatch_commit_unknown` 的 notification history，再交由手动 retry / auto-retry 复用同一 `idempotency_key` 收口。
- 当同一 lineage 已存在最新的 `dispatch_commit_unknown` 恢复记录时，新的 dispatch 候选会先被计入 `skipped`，`reason=dispatch_recovery_pending`，避免在恢复完成前再次直接派发。
- 在 `dispatch`、`notifications list/export-csv`、`retry-batch`、`auto-retry` 与单条 `retry` 入口之前，服务端会先对 latest `dispatch_commit_unknown` history 做最小 provider reconciliation；若确认成功，最新 history 会变成 `sent + dispatch_commit_confirmed`，后续处置不再被 recovery pending 阻断。
- provider receipt 只有在进入正向终态时才会被视为确认成功；`bounced/failed/rejected/cancelled/undelivered` 一类负向终态会落成新的 `dispatch_commit_rejected` history，避免把负向 delivery result 误标成已发送。
- 若 provider receipt 长时间仍停留在 `accepted/queued/processing/not_found` 等非终态，服务端会把这条 pending recovery 落成 `dispatch_commit_confirmation_timeout` 的显式失败 history；该 history 会保持 `retryable=true`，并把 `next_retry_at` 置为 due-now，供后续手动 retry / auto-retry 接管。
- 同一条 notification history 复用的 `idempotency_key` 会继续透传到 wallet/provider 执行层，供下游按相同 key 做内部幂等收口。
- 派发执行层复用 wallet 的 email / WhatsApp provider 配置，但 notification state 保存在 enterprise 模块。
- `items` 返回的就是 notification record；`notifications list` 复用相同模型，后续 `retry` 也以它为目标资源。
- `/summary` 不读取 notification record，也不会因为 granular cooldown bucket 而改变聚合键；但 `HRIS pull` 告警可额外返回 stateful metrics。

### Success (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "total_alerts": 1,
  "dispatched": 1,
  "skipped": 0,
  "failed": 0,
  "items": [
    {
      "id": "swa_001",
      "tenant_id": "tenant_demo_jakarta",
      "worker_action": "enterprise_hris_pull_worker_alert",
      "worker_kind": "hris_pull",
      "worker_label": "HRIS Pull Reconcile",
      "fingerprint": "enterprise_hris_pull_worker_alert|connector-talenta|talenta|incremental",
      "count": 3,
      "threshold": 3,
      "failed": 2,
      "processed": 3,
      "applied": 1,
      "channels": ["email"],
      "receiver_groups": ["security"],
      "status": "sent",
      "attempt": 1,
      "provider": "mock",
      "channel_results": [
        {
          "channel": "email",
          "status": "sent",
          "provider": "mock",
          "retryable": false,
          "receivers": ["security@mistypass.local"]
        }
      ],
      "triggered_at": "2026-04-22T14:00:00Z"
    }
  ],
  "updated_at": "2026-04-22T14:00:00Z"
}
```

常见 `status/reason`：

- `sent`
- `skipped: subscription_disabled`
- `skipped: channel_disabled`
- `skipped: cooldown`
- `skipped: dispatch_inflight`
- `skipped: dispatch_recovery_pending`
- `failed: provider_error`
- `failed: provider_transient_error`
- `failed: dispatch_commit_unknown`
- `failed: dispatch_commit_rejected`
- `failed: dispatch_commit_confirmation_timeout`

## 7. `GET /api/v1/enterprise/sync-worker-alerts/notifications`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 条件必填 | `super_admin` 必填；其他角色默认使用 token tenant |
| `status` | 否 | 可选值：`sent` / `failed` / `skipped` |
| `reason` | 否 | 可选；常见值包括 `manual_suppressed` / `manual_suppressed_restored` / `provider_transient_error` / `dispatch_commit_unknown` / `dispatch_commit_confirmed` / `dispatch_commit_rejected` / `dispatch_commit_confirmation_timeout` |
| `retryable` | 否 | `true/false`；按 retryable 状态过滤 |
| `due_now` | 否 | `true/false`；按 `next_retry_at <= now` 过滤 |
| `q` | 否 | 关键字搜索；匹配 worker、connector、reason、request id、provider error、`pending_age_seconds`、`confirm_attempts`、`last_confirm_attempt_at`、`last_confirm_result`、channel results 等字段 |
| `offset` | 否 | `>=0`，默认 `0` |
| `limit` | 否 | `1..500`，默认 `50` |

### Success (`200`)

```json
{
  "items": [
    {
      "id": "swa_002",
      "tenant_id": "tenant_demo_jakarta",
      "worker_action": "enterprise_hris_webhook_processing_alert",
      "worker_kind": "hris_webhook",
      "worker_label": "HRIS Webhook Merge",
      "fingerprint": "enterprise_hris_webhook_processing_alert|connector-talenta|talenta|merge",
      "count": 3,
      "threshold": 2,
      "failed": 2,
      "processed": 3,
      "applied": 1,
      "channels": ["email"],
      "receiver_groups": ["security"],
      "status": "failed",
      "reason": "provider_transient_error",
      "idempotency_key": "f0e1d2c3b4a5",
      "attempt": 1,
      "retryable": true,
      "provider": "mock",
      "provider_error": "temporary outage",
      "confirm_attempts": 2,
      "last_confirm_attempt_at": "2026-04-22T14:06:30Z",
      "last_confirm_result": "provider_error",
      "channel_results": [
        {
          "channel": "email",
          "status": "failed",
          "reason": "provider_transient_error",
          "provider": "mock",
          "provider_delivery_id": "email_123",
          "provider_delivery_status": "accepted",
          "provider_error": "temporary outage",
          "retryable": true,
          "receivers": ["security@mistypass.local"]
        }
      ],
      "triggered_at": "2026-04-22T14:05:00Z"
    }
  ],
  "total": 12,
  "offset": 0,
  "limit": 50,
  "next_offset": 50,
  "has_more": true,
  "filter_counts": {
    "all": 12,
    "failed": 5,
    "retryable": 4,
    "suppressed": 2,
    "due_now": 3
  },
  "status_counts": {
    "sent": 4,
    "failed": 5,
    "skipped": 3
  }
}
```

说明：

- 本接口读取 enterprise notification records，而不是 worker alert audit log。
- 默认按 `triggered_at desc` 返回最新记录；`dispatch`、单条 `retry`、auto-retry、batch retry、suppress-batch 与 restore-batch 产生的记录会共同出现在历史列表中。
- `filter_counts` 基于当前 `tenant_id + q` 查询范围统计，可直接用于前端筛选按钮计数；`status_counts` 基于当前完整过滤条件统计，但不受 `offset/limit` 截断影响。
- `total/offset/limit/next_offset/has_more` 描述的是“完整过滤结果集”的分页状态，而不是仅当前页计数。
- 当记录满足 `status=failed` 且 `retryable=true` 时，响应可能包含 `next_retry_at`，表示最早允许进入下一轮 auto retry 的时间点。
- 当记录满足 `status=failed` + `retryable=true` + `reason=dispatch_commit_unknown` 时，响应还会额外返回 `pending_age_seconds`，表示当前 reconciliation pending 已持续的秒数。
- 当记录进入 confirmation/reconciliation 轮询后，响应还可能包含 `confirm_attempts`、`last_confirm_attempt_at` 与 `last_confirm_result`；其中 `last_confirm_result` 当前可能为 `pending` / `provider_error` / `confirmed` / `rejected` / `timeout`。
- 当记录满足 `status=skipped` 且 `reason=manual_suppressed` 时，响应可能包含 `restore_status`；当前可选值为 `ready` / `already_sent` / `newer_history_exists`，仅 `ready` 允许 `restore-batch` 真正恢复。
- `channel_results` 里的 `provider_delivery_id / provider_delivery_status` 会直接回传 provider receipt；若服务端在入口 reconciliation 中确认了此前的 `dispatch_commit_unknown`，历史顶部会追加一条新的 `sent + dispatch_commit_confirmed` 记录，而不是原地覆写旧失败记录。
- 若 provider receipt 给出负向终态，历史顶部会追加 `failed + dispatch_commit_rejected`；若 receipt 长时间不进入终态，历史顶部会追加 `failed + dispatch_commit_confirmation_timeout`。两者都会保留最新的 provider receipt 信息，便于后续人工处置。

## 7.1. `GET /api/v1/enterprise/sync-worker-alerts/notifications/export-csv`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`
- Query：与上一节列表接口一致，但会忽略 `offset/limit`，始终导出当前过滤条件下的完整结果集。

导出语义：

- CSV 列包含 `id/tenant_id/triggered_at/worker_action/worker_kind/worker_label/fingerprint/connector_id/vendor/event_type/request_id/failure_stage/mode/count/threshold/failed/processed/applied/status/reason/attempt/retryable/next_retry_at/pending_age_seconds/confirm_attempts/last_confirm_attempt_at/last_confirm_result/channels/receiver_groups/provider/provider_error/source_notification_id/restore_status/idempotency_key/channel_results`。
- `channel_results` 会压平成单列文本，保留 `channel:status` 与可用的 `reason/provider/delivery_id/delivery_status/error/receivers` 片段，便于直接导入表格做人工排查。
- 本接口导出的是 enterprise notification records，不会重新计算 dispatch，也不会写回 worker alert action 集合。
- `restore_status` 列仅对 `manual_suppressed` 历史有意义；`pending_age_seconds` 列仅对 `dispatch_commit_unknown` 一类确认中记录有意义；`confirm_attempts / last_confirm_attempt_at / last_confirm_result` 则用于观察 reconciliation 已尝试了几次、最后一次尝试时间，以及最后一次确认结果；其语义与列表接口一致。

## 8. `POST /api/v1/enterprise/sync-worker-alerts/notifications/retry-batch`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request Body

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "actor": "tenant.admin@sudirman.co",
  "notification_ids": ["swa_002", "swa_001"]
}
```

### Batch Retry Semantics

- 仅处理当前请求里的 notification records，不重新计算 `worker_alert_threshold/window`，也不会重新写回 worker alert action 集合。
- 服务端会先按请求顺序去重 `notification_ids`；空值与重复 id 会被忽略。
- batch retry 会按 notification fingerprint 抑制同类重复项；同一 fingerprint 只会放行首条，其余计入 `suppressed`，不再重复派发。
- 对于已不满足 `failed + retryable` 的过期记录，或同一 lineage 下已不是 latest history 的 stale failed 记录，服务端会计入 `skipped`，不报 `409`，以便批处理可以部分完成。lineage 内部按 `worker_action + fingerprint + request_id`（有 `request_id` 时）判定，不受 threshold 变化影响。
- 若目标 notification 所属 lineage 已有进行中的 provider 派发，当前条目也会计入 `skipped`，不再并发重试。
- 成功进入执行的 notification 会沿用原有 `idempotency_key`，并继续按单条 retry 规则生成新的 retry record。

### Success (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "total_notifications": 2,
  "retried": 1,
  "skipped": 0,
  "failed": 0,
  "suppressed": 1,
  "items": [
    {
      "id": "swa_003",
      "tenant_id": "tenant_demo_jakarta",
      "worker_action": "enterprise_hris_webhook_processing_alert",
      "worker_kind": "hris_webhook",
      "worker_label": "HRIS Webhook Merge",
      "fingerprint": "enterprise_hris_webhook_processing_alert|connector-talenta|talenta|merge",
      "count": 3,
      "threshold": 2,
      "failed": 2,
      "processed": 3,
      "applied": 1,
      "channels": ["email"],
      "receiver_groups": ["security"],
      "status": "sent",
      "attempt": 2,
      "retryable": false,
      "provider": "mock",
      "source_notification_id": "swa_002",
      "triggered_at": "2026-04-22T14:10:00Z"
    }
  ],
  "updated_at": "2026-04-22T14:10:00Z"
}
```

## 9. `POST /api/v1/enterprise/sync-worker-alerts/notifications/suppress-batch`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request Body

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "actor": "tenant.admin@sudirman.co",
  "notification_ids": ["swa_002", "swa_001"]
}
```

### Suppress Semantics

- 仅处理当前请求里的 notification records，不重新计算 `worker_alert_threshold/window`，也不会重新写回 worker alert action 集合。
- 服务端会先按请求顺序去重 `notification_ids`；空值与重复 id 会被忽略。
- 仅 `status=failed` 且仍是同一 lineage 下 latest history 的 notification 会被实际 suppress；其他状态或 stale failed history 都计入 `skipped`。
- suppress 会生成新的 notification history record，`status=skipped`、`reason=manual_suppressed`，以便后续审计与人工复盘。
- suppress 不会刷新 dispatch cooldown，也不会触发 provider 派发。

### Success (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "total_notifications": 2,
  "suppressed": 1,
  "skipped": 1,
  "items": [
    {
      "id": "swa_004",
      "tenant_id": "tenant_demo_jakarta",
      "worker_action": "enterprise_hris_webhook_processing_alert",
      "worker_kind": "hris_webhook",
      "worker_label": "HRIS Webhook Merge",
      "fingerprint": "enterprise_hris_webhook_processing_alert|connector-talenta|talenta|merge",
      "count": 3,
      "threshold": 2,
      "failed": 2,
      "processed": 3,
      "applied": 1,
      "channels": ["email"],
      "receiver_groups": ["security"],
      "status": "skipped",
      "reason": "manual_suppressed",
      "attempt": 2,
      "retryable": false,
      "source_notification_id": "swa_002",
      "triggered_at": "2026-04-22T14:11:00Z"
    }
  ],
  "updated_at": "2026-04-22T14:11:00Z"
}
```

## 10. `POST /api/v1/enterprise/sync-worker-alerts/notifications/restore-batch`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request Body

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "actor": "tenant.admin@sudirman.co",
  "notification_ids": ["swa_004", "swa_002"]
}
```

### Restore Semantics

- 仅处理当前请求里的 notification records，不重新计算 `worker_alert_threshold/window`，也不会重新写回 worker alert action 集合。
- 服务端会先按请求顺序去重 `notification_ids`；空值与重复 id 会被忽略。
- 仅 `status=skipped` 且 `reason=manual_suppressed` 的 notification 会进入 restore eligibility 判断；其他状态或 reason 计入 `skipped`。
- 若被 suppress 的 history 带有 `source_notification_id`，restore 会回溯原始失败记录作为模板，保持 fingerprint、指标字段与 channel/provider 元数据对齐。
- 列表 / 导出会对 suppression 记录额外给出 `restore_status`：`ready` 表示可恢复，`already_sent` 表示同一 lineage 下已有发送成功 history，`newer_history_exists` 表示 suppression 之后已出现更新的 history。
- `restore-batch` 仅会实际恢复 `restore_status=ready` 的记录；`already_sent` 与 `newer_history_exists` 都会计入 `skipped`，避免把 stale suppression 或已成功恢复的通知再次重新打开。
- restore 会生成新的 notification history record，`status=failed`、`reason=manual_suppressed_restored`、`retryable=true`、`source_notification_id=<suppressed notification id>`、`next_retry_at=restored_at`；本接口本身不触发 provider 派发，后续可继续走单条 retry、batch retry、显式 HTTP auto-retry 或后台 ticker worker。

### Success (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "total_notifications": 2,
  "restored": 1,
  "skipped": 1,
  "items": [
    {
      "id": "swa_005",
      "tenant_id": "tenant_demo_jakarta",
      "worker_action": "enterprise_hris_webhook_processing_alert",
      "worker_kind": "hris_webhook",
      "worker_label": "HRIS Webhook Merge",
      "fingerprint": "enterprise_hris_webhook_processing_alert|connector-talenta|talenta|merge",
      "count": 3,
      "threshold": 2,
      "failed": 2,
      "processed": 3,
      "applied": 1,
      "channels": ["email"],
      "receiver_groups": ["security"],
      "status": "failed",
      "reason": "manual_suppressed_restored",
      "idempotency_key": "f0e1d2c3b4a5",
      "attempt": 3,
      "retryable": true,
      "provider": "mock",
      "provider_error": "temporary outage",
      "channel_results": [
        {
          "channel": "email",
          "status": "failed",
          "reason": "manual_suppressed_restored"
        }
      ],
      "source_notification_id": "swa_004",
      "next_retry_at": "2026-04-22T14:12:00Z",
      "triggered_at": "2026-04-22T14:12:00Z"
    }
  ],
  "updated_at": "2026-04-22T14:12:00Z"
}
```

## 11. `POST /api/v1/enterprise/sync-worker-alerts/notifications/auto-retry`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request Body

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "actor": "tenant.admin@sudirman.co",
  "limit": 20,
  "max_attempts": 3,
  "base_backoff_ms": 300000,
  "max_backoff_ms": 3600000
}
```

### Auto Retry Semantics

- 仅扫描当前 tenant 下 `status=failed`、`retryable=true` 且 `next_retry_at <= now` 的 notification records。
- 服务端按 notification history 的最新记录优先处理；若同一 fingerprint 存在更旧的 due 失败项，旧项计入 `suppressed`，不重复派发。
- 当最新 due 记录的 `attempt >= max_attempts` 时，服务端不会继续派发，而是生成新的 history record，`status=skipped`、`reason=auto_retry_attempt_limit`。
- `limit <= 0`、`max_attempts <= 0`、未显式传入的 backoff 参数都会回退到默认值 `20 / 3 / 5m / 1h`；`limit > 100` 会被服务端截断到 `100`，`max_backoff_ms < base_backoff_ms` 时会自动抬到同一上限。
- auto retry 继续沿用原有 `idempotency_key`，成功后刷新 cooldown；若再次失败且仍可重试，会根据 `attempt` 和 backoff 参数重新计算 `next_retry_at`。
- 若目标 notification 所属 lineage 已有进行中的 provider 派发，当前 due item 会计入 `skipped`，避免多实例并发 auto retry。
- 显式 HTTP due-now 扫描会继续保留；同时服务端也可通过配置化 ticker worker 周期调用同一 `AutoRetrySyncWorkerAlertNotifications` service 语义，处理逻辑与状态迁移保持一致。

### Success (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "total_notifications": 2,
  "retried": 1,
  "skipped": 1,
  "failed": 0,
  "suppressed": 0,
  "items": [
    {
      "id": "swa_005",
      "tenant_id": "tenant_demo_jakarta",
      "worker_action": "enterprise_hris_webhook_processing_alert",
      "worker_kind": "hris_webhook",
      "worker_label": "HRIS Webhook Merge",
      "fingerprint": "enterprise_hris_webhook_processing_alert|connector-talenta|talenta|merge",
      "count": 3,
      "threshold": 2,
      "failed": 2,
      "processed": 3,
      "applied": 1,
      "channels": ["email"],
      "receiver_groups": ["security"],
      "status": "sent",
      "idempotency_key": "f0e1d2c3b4a5",
      "attempt": 2,
      "retryable": false,
      "provider": "mock",
      "source_notification_id": "swa_002",
      "triggered_at": "2026-04-22T14:12:00Z"
    },
    {
      "id": "swa_006",
      "tenant_id": "tenant_demo_jakarta",
      "worker_action": "enterprise_hris_pull_worker_alert",
      "worker_kind": "hris_pull",
      "worker_label": "HRIS Pull Reconcile",
      "fingerprint": "enterprise_hris_pull_worker_alert|connector-talenta|talenta|incremental",
      "count": 4,
      "threshold": 3,
      "failed": 4,
      "processed": 8,
      "applied": 4,
      "channels": ["email"],
      "receiver_groups": ["security"],
      "status": "skipped",
      "reason": "auto_retry_attempt_limit",
      "idempotency_key": "a1b2c3d4e5f6",
      "attempt": 4,
      "retryable": false,
      "source_notification_id": "swa_004",
      "triggered_at": "2026-04-22T14:12:00Z"
    }
  ],
  "updated_at": "2026-04-22T14:12:00Z"
}
```

## 12. `POST /api/v1/enterprise/sync-worker-alerts/notifications/{notificationID}/retry`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request Body

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "actor": "tenant.admin@sudirman.co"
}
```

### Retry Semantics

- 只允许重试 `status=failed` 且 `retryable=true` 的 notification record。
- 若目标记录在同一 lineage 下已不是 latest history，服务端会直接返回 `retry not allowed`，避免对 stale failed history 重复处置。
- 若目标记录所属 lineage 已有进行中的 provider 派发，服务端会返回 `409 dispatch is already in flight`，避免单条 retry 与其他实例并发重复发信。
- retry 直接针对既有 notification record 重新派发，不重新计算 `worker_alert_threshold/window`，也不会重新写回 worker alert action 集合。
- retry 不参与 dispatch cooldown 选举，但若 retry 成功，仍会刷新该 fingerprint 的 cooldown 时间，避免紧接着的重复派发。
- 新记录会保留相同 `idempotency_key`，`attempt` 递增，并通过 `source_notification_id` 指向原始失败记录。

### Success (`200`)

```json
{
  "id": "swa_003",
  "tenant_id": "tenant_demo_jakarta",
  "worker_action": "enterprise_hris_webhook_processing_alert",
  "worker_kind": "hris_webhook",
  "worker_label": "HRIS Webhook Merge",
  "fingerprint": "enterprise_hris_webhook_processing_alert|connector-talenta|talenta|merge",
  "count": 3,
  "threshold": 2,
  "failed": 2,
  "processed": 3,
  "applied": 1,
  "channels": ["email"],
  "receiver_groups": ["security"],
  "status": "sent",
  "idempotency_key": "f0e1d2c3b4a5",
  "attempt": 2,
  "retryable": false,
  "provider": "mock",
  "channel_results": [
    {
      "channel": "email",
      "status": "sent",
      "provider": "mock",
      "retryable": false,
      "receivers": ["security@mistypass.local"]
    }
  ],
  "source_notification_id": "swa_002",
  "triggered_at": "2026-04-22T14:10:00Z"
}
```

## 13. 数据来源

本接口读取审计事件：

- `action=enterprise_sync_reconcile_worker_alert`
- `action=enterprise_hris_webhook_receipt_worker_alert`
- `action=enterprise_hris_webhook_dlq_worker_alert`
- `action=enterprise_hris_pull_worker_alert`
- `action=enterprise_hris_webhook_processing_alert`
- `source=enterprise_sync_worker`

聚合规则：

- 列表接口按 `at desc` 返回所有匹配 worker alert 审计事件。
- 汇总接口按 `tenant_id + worker_action` 聚合，而不是仅按租户聚合；即使 dispatch 内部已按更细粒度 bucket 执行 cooldown，这个合同也不变。
- `last_applied` 为通用“最近一次成功处理量”语义：对 reconcile / webhook processing worker 映射 `applied`，对 HRIS DLQ worker 映射 `replayed`，对 webhook receipt queue / HRIS pull worker 映射 `synced`。
- `last_consecutive_failures / last_failure_age_seconds` 当前仅在 `enterprise_hris_pull_worker_alert` 上有意义，用于表达 connector 连续失败次数与最近失败持续时长。
- 手动派发接口并不会把 dispatch 结果重新写回 worker alert action 集合，避免告警聚合自循环。
- `notifications list/export-csv/retry/auto-retry/retry-batch/suppress-batch/restore-batch` 读取与写回的是 enterprise notification records，不会重新写入 worker alert action 集合，避免告警聚合自循环。
- `restore-batch` 会把 `restore_status=ready` 的 `manual_suppressed` history 恢复成新的 `failed + retryable` 记录，`reason=manual_suppressed_restored`，并把 `next_retry_at` 置为恢复时间，供后续单条 retry、batch retry、显式 HTTP auto-retry 或后台 ticker worker 继续处理。
- 后台 ticker worker 的 Redis lease 仅包住单次 tick 外层，不改变 `auto-retry` service 语义；它是多实例止血项，不负责 notification state 的全局一致性治理。
