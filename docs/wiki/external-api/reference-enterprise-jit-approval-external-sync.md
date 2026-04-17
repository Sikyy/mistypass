# API Reference：Enterprise JIT 审批回写

当前能力状态：

- `CONTRACT_READY`：审批回写查询、更新、回调字段与状态机已稳定。
- `PROD_READY`：状态码、token 校验、审计事件与重试计数行为有测试覆盖。

## 1. `GET /api/v1/enterprise/jit-provision-approvals/external-sync-pending`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |
| `limit` | 否 | `>=0`；`0` 或缺省表示不截断 |

### Success (`200`)

```json
{
  "items": [
    {
      "id": "jap_001",
      "tenant_id": "tenant_demo_jakarta",
      "email": "alice@sudirman.co",
      "status": "approved",
      "external_sync_status": "pending",
      "external_sync_attempt_count": 0,
      "created_at": "2026-04-15T08:00:00Z",
      "updated_at": "2026-04-15T08:05:00Z"
    }
  ]
}
```

## 2. `POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/external-sync`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "status": "failed",
  "external_sync_ref": "hris-job-20260415-01",
  "last_error": "upstream timeout"
}
```

`status` 仅接受 `synced` 或 `failed`。

### Success (`200`)

```json
{
  "item": {
    "id": "jap_001",
    "tenant_id": "tenant_demo_jakarta",
    "status": "approved",
    "external_sync_status": "failed",
    "external_sync_ref": "hris-job-20260415-01",
    "external_sync_attempt_count": 1,
    "external_sync_last_error": "upstream timeout",
    "external_sync_updated_at": "2026-04-15T09:00:00Z",
    "updated_at": "2026-04-15T09:00:00Z"
  }
}
```

语义：

- `failed` 会累加 `external_sync_attempt_count`。
- `synced` 会清空 `external_sync_last_error`。

## 3. `POST /api/v1/enterprise/jit-provision-approvals/external-sync/callback`

- Auth：无需 Bearer，会校验 callback token。
- Token 来源优先级：
1. `X-Enterprise-Callback-Token` header
2. `Authorization: Bearer <token>`
3. body `callback_token`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "approval_id": "jap_001",
  "status": "synced",
  "external_sync_ref": "hris-callback-job-001",
  "last_error": "",
  "callback_token": "optional-when-header-present"
}
```

### Success (`200`)

```json
{
  "item": {
    "id": "jap_001",
    "tenant_id": "tenant_demo_jakarta",
    "external_sync_status": "synced",
    "external_sync_ref": "hris-callback-job-001",
    "updated_at": "2026-04-15T09:10:00Z"
  }
}
```

## 4. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `limit must be an integer >= 0` | 查询参数非法 |
| `400` | `invalid jit provision approval external sync status` | `status` 非 `synced/failed` |
| `401` | `invalid callback token` | 回调 token 不匹配 |
| `404` | `enterprise jit provision approval not found` | `approval_id` 不存在 |
| `503` | `...callback is disabled` | 未配置 `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_CALLBACK_TOKEN` |

## 5. 审计事件

- 更新接口：`enterprise_jit_approval_external_sync_updated`
- 回调接口：`enterprise_jit_approval_external_sync_callback`

