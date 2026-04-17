# API Reference：Access Core（Users / Groups / Policies / Temporary / Visitor）

当前能力状态：

- `CONTRACT_READY`：Access 核心资源接口字段和状态机已稳定。
- `PROD_READY`：作用域过滤、枚举校验、`building_admin` 约束与关键错误码有回归覆盖。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| Users | `GET/POST /api/v1/users` | 列表：`super_admin/tenant_admin/operator/building_admin`；创建：`super_admin/tenant_admin/building_admin` |
| User Groups | `GET/POST /api/v1/user-groups`、`PATCH /api/v1/user-groups/{groupID}` | 同上 |
| Access Policies | `GET/POST /api/v1/access-policies`、`PATCH /api/v1/access-policies/{policyID}` | 同上 |
| Temporary Access | `GET/POST /api/v1/temporary-access` | 同上 |
| Visitor Passes | `GET/POST /api/v1/visitor-passes` | 同上 |

## 2. Users

### `POST /api/v1/users`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "building_id": "building_demo_001",
  "name": "Alice",
  "email": "alice@example.com",
  "role": "employee",
  "status": "active",
  "group_ids": ["ug_common_office_jkt"]
}
```

`status` 允许值：`active`（默认）/ `inactive` / `suspended`。

## 3. User Groups

### `POST /api/v1/user-groups`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "building_id": "building_demo_001",
  "name": "Security Team",
  "description": "night shift",
  "members": ["usr_1001"]
}
```

### `PATCH /api/v1/user-groups/{groupID}?tenant_id=...`

```json
{
  "building_id": "building_demo_001",
  "name": "Security Team v2",
  "description": "night+weekend",
  "members": ["usr_1001", "usr_1002"]
}
```

## 4. Access Policies

### `POST /api/v1/access-policies`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "name": "Finance Workhour Access",
  "scope_type": "area",
  "building_id": "building_demo_001",
  "area_id": "area_demo_001",
  "door_id": "",
  "schedule": "Mon-Fri 08:00-19:00",
  "members": 86,
  "status": "active"
}
```

`scope_type` 允许值：`all`（默认）/ `building` / `area` / `door`。  
`status` 允许值：`active`（默认）/ `inactive` / `draft`。

### `PATCH /api/v1/access-policies/{policyID}?tenant_id=...`

请求体字段与创建接口一致。

## 5. Temporary Access

### `POST /api/v1/temporary-access`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "scope_type": "door",
  "building_id": "building_demo_001",
  "area_id": "area_demo_001",
  "door_id": "door_jkt_001",
  "delivery_method": "wallet",
  "grantee_name": "Bob",
  "grantee_gender": "male",
  "grantee_phone": "+62-811-0000-0000",
  "grantee_email": "bob@example.com",
  "mobile_model": "Pixel 8",
  "pass_type": "employee",
  "valid_until": "2026-04-20 20:00"
}
```

`delivery_method` 允许值：`wallet`（默认）/ `email_qr`。

## 6. Visitor Passes

### `POST /api/v1/visitor-passes`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "building_id": "building_demo_001",
  "host": "Alice",
  "visitor": "PT Integrator Nusantara",
  "delivery_method": "email_qr",
  "expires_at": "2026-04-20 18:30"
}
```

## 7. Scope 与默认行为

- 列表接口都支持 `tenant_id` query。
- `building_admin` 创建资源时若未传 `building_id`，多数接口返回 `400 building_id is required for building_admin`。
- `building_admin` 超出楼宇范围会返回 `403 building scope forbidden`。
- `createTemporaryAccess` 若未传 `scope_type`，默认按 `door` 处理。

## 8. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `tenant_id/user name/user email/... is required` | 必填字段缺失 |
| `400` | `invalid scope type/policy status/user status/delivery method` | 枚举值非法 |
| `404` | `policy not found` / `user group not found` | 资源不存在 |
| `403` | `tenant scope forbidden` / `building scope forbidden` | 作用域越权 |
| `500` | internal error | 服务端异常 |

