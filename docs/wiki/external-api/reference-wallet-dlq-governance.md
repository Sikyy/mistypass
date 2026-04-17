# API Reference：Wallet DLQ 治理

当前能力状态：

- `CONTRACT_READY`：DLQ 治理接口请求与响应字段稳定。
- `PROD_READY`：默认值回退、参数边界与归档留痕行为已验证。

## 1. `POST /api/v1/wallet/jobs/dlq/requeue`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "limit": 50,
  "error_code": "template_inactive",
  "target_id_override": "usr_1001",
  "actor": "ops.bot"
}
```

说明：

- `limit <= 0` 时服务端默认按 `20`。
- `limit > 500` 返回 `400`。

### Success (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "limit": 50,
  "requeued": 12,
  "skipped": 3,
  "remaining_dlq": 8,
  "processed_jobs": ["wj_001", "wj_002"],
  "updated_at": "2026-04-15T10:30:00Z"
}
```

## 2. `POST /api/v1/wallet/jobs/dlq/cleanup`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "limit": 100,
  "error_code": "template_inactive",
  "older_than_seconds": 86400,
  "actor": "ops.bot"
}
```

说明：

- `limit <= 0`：回退到 `WALLET_DLQ_CLEANUP_DEFAULT_LIMIT`，无配置时默认 `50`。
- `older_than_seconds <= 0`：回退到 `WALLET_DLQ_CLEANUP_DEFAULT_OLDER_THAN`，无配置时默认 `24h`。
- `limit > 1000` 或 `older_than_seconds > 365d`：返回 `400`。

### Success (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "limit": 100,
  "removed": 9,
  "remaining_dlq": 4,
  "processed_jobs": ["wj_010", "wj_011"],
  "updated_at": "2026-04-15T10:35:00Z"
}
```

## 3. `GET /api/v1/wallet/jobs/dlq/cleanup/archives`

- Auth：`Authorization: Bearer <access_token>`
- 角色：`super_admin` / `tenant_admin` / `operator`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 限制 |
| `limit` | 否 | `>=0`，默认 `20` |

### Success (`200`)

```json
{
  "items": [
    {
      "id": "wdlqca_001",
      "tenant_id": "tenant_demo_jakarta",
      "limit": 100,
      "error_code": "template_inactive",
      "older_than_seconds": 86400,
      "actor": "ops.bot",
      "removed": 9,
      "remaining_dlq": 4,
      "processed_jobs": ["wj_010", "wj_011"],
      "at": "2026-04-15T10:35:00Z"
    }
  ]
}
```

## 4. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `limit must be an integer >= 0` | 查询参数非法 |
| `400` | `invalid wallet job dlq options` | 请求参数越界 |
| `403` | `tenant scope forbidden` | 租户越权 |
| `500` | internal error | 服务端异常 |

## 5. 相关 Guide

- `docs/wiki/external-api/guides-wallet-dlq-governance.md`
- `docs/wiki/external-api/guides-wallet-queue-ops.md`
