# API Reference：Enterprise Sync Worker Alerts

当前能力状态：

- `CONTRACT_READY`：结构化告警列表与聚合摘要接口已稳定。
- `PROD_READY`：时间窗过滤、limit 校验与告警字段解析有回归覆盖。

## 1. `GET /api/v1/enterprise/sync-worker-alerts`

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

## 2. `GET /api/v1/enterprise/sync-worker-alerts/summary`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`
- Query：与上一个接口一致（`tenant_id/since/until/limit`）。

### Success (`200`)

```json
{
  "items": [
    {
      "tenant_id": "tenant_demo_jakarta",
      "count": 3,
      "first_seen_at": "2026-04-15T09:00:00Z",
      "last_seen_at": "2026-04-15T10:00:00Z",
      "last_failed": 1,
      "last_threshold": 1,
      "last_processed": 1,
      "last_applied": 0,
      "last_skipped_by_attempt_limit": 0,
      "last_skipped_by_cooldown": 0
    }
  ]
}
```

## 3. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `since must be RFC3339 timestamp` | `since` 格式错误 |
| `400` | `until must be RFC3339 timestamp` | `until` 格式错误 |
| `400` | `since must be <= until` | 时间窗逆序 |
| `400` | `limit must be an integer >= 0` | `limit` 非法 |
| `403` | `tenant scope forbidden` | 租户越权 |

## 4. 数据来源

本接口读取审计事件：

- `action=enterprise_sync_reconcile_worker_alert`
- `source=enterprise_sync_worker`

