# API Reference：Enterprise HRIS Secrets

当前能力状态：

- `CONTRACT_READY`：`GET/PUT /api/v1/enterprise/hris-secrets` 已可用于租户级 HRIS secret metadata 管理。
- `SECURE_STORAGE_READY`：secret value 已加密存入 `module_hris` vault snapshot；管理 API 不返回 plaintext，`Talenta webhook` 与 `pull/hybrid` connector 均已通过 vault ref 消费 secret。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| HRIS Secrets | `GET /api/v1/enterprise/hris-secrets` | `super_admin/tenant_admin/operator` |
| HRIS Secrets | `PUT /api/v1/enterprise/hris-secrets` | `super_admin/tenant_admin` |

说明：

- secret ref 格式固定为 `vault://{tenant_id}/{name}`。
- 当前 API 只返回 metadata，不暴露 secret plaintext。
- `POST/PATCH /api/v1/enterprise/hris-connectors` 也支持直接提交 `credential_value` / `webhook_secret_value`，服务端会自动 materialize 成 vault ref。
- `Talenta` 当前约定：`webhook_secret_ref` 保存 webhook HMAC secret；`credential_ref` 可保存 raw `client_id`，也可保存用于 pull/hybrid connector 的 JSON credential（至少包含 `client_id`、`client_secret`，可选 `base_url`、`employee_path`、`page_limit`）。

## 2. `GET /api/v1/enterprise/hris-secrets`

- Auth：`Authorization: Bearer <access_token>`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |
| `ref` | 否 | 传入时返回单条 secret metadata；`ref` 自带 tenant scope，`super_admin` 可仅传 `ref` |

### Success（列表 `200`）

```json
{
  "items": [
    {
      "ref": "vault://tenant_demo_jakarta/hris/talenta/webhook_secret",
      "tenant_id": "tenant_demo_jakarta",
      "name": "hris/talenta/webhook_secret",
      "kind": "webhook_secret",
      "updated_by": "tenant.admin@sudirman.co",
      "created_at": "2026-04-22T09:30:00Z",
      "updated_at": "2026-04-22T09:30:00Z"
    }
  ]
}
```

### Success（单条 `200`）

```json
{
  "item": {
    "ref": "vault://tenant_demo_jakarta/hris/talenta/credential",
    "tenant_id": "tenant_demo_jakarta",
    "name": "hris/talenta/credential",
    "kind": "connector_credential",
    "updated_by": "tenant.admin@sudirman.co",
    "created_at": "2026-04-22T09:30:00Z",
    "updated_at": "2026-04-22T10:10:00Z"
  }
}
```

## 3. `PUT /api/v1/enterprise/hris-secrets`

用途：创建或覆盖租户级 HRIS secret。相同 `tenant_id + name` 会做 upsert。

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "name": "hris/talenta/webhook_secret",
  "kind": "webhook_secret",
  "value": "talenta-webhook-secret-001",
  "updated_by": "tenant.admin@sudirman.co"
}
```

Talenta pull / hybrid connector 推荐 credential value：

```json
{
  "client_id": "talenta-client-id-001",
  "client_secret": "talenta-client-secret-001",
  "base_url": "https://api.mekari.com",
  "employee_path": "/v2/talenta/v2/employee",
  "page_limit": 20,
  "updated_after_param": "updated_after",
  "updated_before_param": "updated_before",
  "timestamp_format": "rfc3339"
}
```

说明：

- `client_id`：webhook 验签时也会作为可选 `username` 比对值。
- `client_secret`：当前用于 Talenta API pull 的 HMAC 请求签名。
- `base_url`、`employee_path`、`page_limit` 可选；缺省时服务端会回退到 Talenta 默认公开员工列表配置。
- `updated_after_param` 可选；存在时 worker 会在非 full reconcile 周期内按上次成功 pull 时间发起增量请求。
- `updated_before_param` 可选；存在时服务端会在增量请求中附带“本次 pull 截止时间”。
- `timestamp_format` 可选；支持 `rfc3339`、`rfc3339nano`、`datetime`/`date_time`/`talenta`、`date`，或直接传 Go time layout。
- 若未提供 `updated_after_param`，当前 connector 会退回“周期性全量分页 pull + 每日 full reconcile 停用缺失员工”的保守模式。

字段说明：

- `name` 仅允许规范化的路径式命名，例如 `hris/talenta/credential`、`hris/talenta/webhook_secret`。
- `kind` 可空；缺省时按 `generic` 处理。
- `value` 必填，但不会在响应中回显。

### Success（`200`）

```json
{
  "item": {
    "ref": "vault://tenant_demo_jakarta/hris/talenta/webhook_secret",
    "tenant_id": "tenant_demo_jakarta",
    "name": "hris/talenta/webhook_secret",
    "kind": "webhook_secret",
    "updated_by": "tenant.admin@sudirman.co",
    "created_at": "2026-04-22T09:30:00Z",
    "updated_at": "2026-04-22T10:15:00Z"
  }
}
```

## 4. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `tenant_id is required` | 缺少租户 |
| `400` | `secret name is required` | `name` 为空 |
| `400` | `secret value is required` | `value` 为空 |
| `400` | `invalid hris vault secret name` | `name` 不符合规范 |
| `400` | `invalid hris vault secret kind` | `kind` 非法 |
| `400` | `invalid hris vault secret ref` | `ref` 非法 |
| `404` | `hris vault secret not found` | 单条查询目标不存在 |
