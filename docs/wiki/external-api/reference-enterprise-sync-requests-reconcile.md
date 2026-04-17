# API Reference：Enterprise Sync Requests / Reconcile

当前能力状态：

- `CONTRACT_READY`：`sync-requests` 查询与两类 reconcile 接口字段和状态码已稳定。
- `PROD_READY`：单次回放、批量补偿、失败记录与参数边界有回归覆盖。

## 1. `GET /api/v1/enterprise/sync-requests`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 限制 |
| `request_id` | 否 | 传入时返回单条记录（`item`） |
| `limit` | 否 | 仅列表模式生效；默认 `50`；需为整数 |

### Success（列表，`200`）

```json
{
  "items": [
    {
      "request_id": "req-20260415-001",
      "tenant_id": "tenant_demo_jakarta",
      "access_applied": true,
      "access_created": 1,
      "access_updated": 0,
      "access_rejected": 0,
      "access_attempt_count": 1,
      "last_access_error": "",
      "last_access_attempt_at": "2026-04-15T10:00:00Z",
      "created_at": "2026-04-15T09:59:00Z",
      "result": {
        "job": {
          "id": "syn_001",
          "tenant_id": "tenant_demo_jakarta",
          "status": "completed"
        },
        "items": []
      }
    }
  ]
}
```

### Success（单条，`200`）

```json
{
  "item": {
    "request_id": "req-20260415-001",
    "tenant_id": "tenant_demo_jakarta",
    "access_applied": true,
    "access_attempt_count": 1
  }
}
```

## 2. `POST /api/v1/enterprise/employees/sync/reconcile`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

用途：按 `request_id` 对单次企业员工同步结果执行 access 回放补偿。

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "request_id": "req-20260415-001"
}
```

### Success（`200`）

```json
{
  "request_id": "req-20260415-001",
  "job": {
    "id": "syn_001",
    "tenant_id": "tenant_demo_jakarta",
    "status": "completed"
  },
  "items": [],
  "access_sync": {
    "created": 1,
    "updated": 0,
    "rejected": 0
  }
}
```

## 3. `POST /api/v1/enterprise/sync-requests/reconcile-pending`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

用途：批量回放当前租户 `access_applied=false` 的 pending 同步记录。

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "limit": 20
}
```

`limit` 规则：

- `limit < 0`：`400`
- `limit = 0`：使用默认值 `20`
- `limit > 200`：按 `200` 执行（服务端封顶）

### Success（`200`）

```json
{
  "processed": 2,
  "applied": 1,
  "failed": 1,
  "skipped_by_attempt_limit": 0,
  "skipped_by_cooldown": 0,
  "items": [
    {
      "request_id": "req-1",
      "job_id": "syn_1",
      "access_applied": true,
      "access_created": 1,
      "access_updated": 0,
      "access_rejected": 0,
      "access_attempt_count": 1,
      "last_access_error": ""
    }
  ]
}
```

## 4. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `tenant_id is required` | 租户缺失 |
| `400` | `request_id is required` | 单次 reconcile 缺少请求号 |
| `400` | `reconcile limit must be >= 1` | `reconcile-pending` 传负值 |
| `400` | `limit must be an integer` | `sync-requests` 列表 `limit` 非法 |
| `404` | `sync request not found` | 请求号不存在 |
| `403` | `tenant scope forbidden` | 租户越权 |

## 5. 回归脚本

- `docs/testing/curl-enterprise-sync-access-batch.zsh`
- `docs/testing/curl-enterprise-sync-worker-alert.zsh`

