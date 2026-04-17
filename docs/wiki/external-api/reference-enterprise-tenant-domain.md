# API Reference：Enterprise Tenant Resolve / Domain Mappings

当前能力状态：

- `CONTRACT_READY`：租户域名解析与 domain mapping 管理接口字段/状态码已稳定。
- `PROD_READY`：域名归一化、最长后缀匹配、冲突与越权边界有回归覆盖。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| Tenant Resolve | `POST /api/v1/enterprise/tenant/resolve` | 无需 Bearer（公开入口） |
| Domain Mappings | `GET /api/v1/enterprise/domain-mappings` | `super_admin/tenant_admin/operator` |
| Domain Mappings | `POST /api/v1/enterprise/domain-mappings`、`PATCH /api/v1/enterprise/domain-mappings/{mappingID}/status` | `super_admin/tenant_admin` |

## 2. `POST /api/v1/enterprise/tenant/resolve`

用途：根据邮箱域名解析所属租户（仅匹配 `active` 域名映射）。

### Request

```json
{
  "email": "alice@sso.sudirman.co"
}
```

### Success（`200`）

```json
{
  "email": "alice@sso.sudirman.co",
  "domain": "sso.sudirman.co",
  "tenant_id": "tenant_demo_jakarta",
  "matched": true
}
```

解析规则：

- 邮箱会归一化为小写。
- 采用“最长域名后缀”匹配（`a.b.example.com` 可匹配 `example.com` 与 `b.example.com`，优先更长项）。
- 仅 `status=active` 的 mapping 参与解析。

## 3. `GET /api/v1/enterprise/domain-mappings`

- Auth：`Authorization: Bearer <access_token>`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |

### Success（`200`）

```json
{
  "items": [
    {
      "id": "dm_001",
      "tenant_id": "tenant_demo_jakarta",
      "domain": "sudirman.co",
      "status": "active",
      "created_at": "2026-04-15T08:00:00Z",
      "updated_at": "2026-04-15T08:00:00Z"
    }
  ]
}
```

## 4. `POST /api/v1/enterprise/domain-mappings`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "domain": "@sudirman.co",
  "status": "active"
}
```

### Success（`201`）

```json
{
  "id": "dm_003",
  "tenant_id": "tenant_demo_jakarta",
  "domain": "sudirman.co",
  "status": "active",
  "created_at": "2026-04-15T10:00:00Z",
  "updated_at": "2026-04-15T10:00:00Z"
}
```

`domain` 归一化规则：

- 自动去掉前导 `@` 并转小写。
- 不允许 `://`、`/`、空格。
- 必须至少包含一个 `.`。

`status` 允许值：`active`（默认）/ `inactive`。

## 5. `PATCH /api/v1/enterprise/domain-mappings/{mappingID}/status`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |

### Request

```json
{
  "status": "inactive"
}
```

### Success（`200`）

```json
{
  "id": "dm_003",
  "tenant_id": "tenant_demo_jakarta",
  "domain": "sudirman.co",
  "status": "inactive",
  "created_at": "2026-04-15T10:00:00Z",
  "updated_at": "2026-04-15T10:05:00Z"
}
```

## 6. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `email is required` | resolve 缺少邮箱 |
| `400` | `invalid domain` | 域名格式非法 |
| `400` | `tenant_id is required` | 创建 mapping 缺少租户 |
| `400` | `invalid domain mapping status` | `status` 非 `active/inactive` |
| `404` | `domain is not mapped` | resolve 未命中有效域名 |
| `404` | `domain mapping not found` | `mappingID` 不存在或越权 |
| `409` | `domain already mapped` | 域名已被占用 |
| `403` | `tenant scope forbidden` | 租户越权 |
