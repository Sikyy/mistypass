# API Reference：Auth Session / Me

当前能力状态：

- `CONTRACT_READY`：`login/refresh/logout/me` 会话接口结构已稳定。
- `PROD_READY`：令牌校验、刷新、注销后的拒绝行为有持续测试覆盖。

## 1. Endpoint Matrix

| Method | Path | Auth | 说明 |
|---|---|---|---|
| `POST` | `/api/v1/auth/login` | 否 | 账号密码登录 |
| `POST` | `/api/v1/auth/refresh` | 否 | 刷新 access token |
| `POST` | `/api/v1/auth/logout` | `Bearer` | 注销当前 access token |
| `GET` | `/api/v1/me` | `Bearer` | 获取当前用户信息 |

## 2. `POST /api/v1/auth/login`

### Request

```json
{
  "email": "superadmin@mistypass.local",
  "password": "admin123"
}
```

### Success（`200`）

```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_in": 3600,
  "user": {
    "id": "usr_super_admin_001",
    "email": "superadmin@mistypass.local",
    "role": "super_admin",
    "tenant_id": ""
  }
}
```

## 3. `POST /api/v1/auth/refresh`

### Request

```json
{
  "refresh_token": "eyJ..."
}
```

### Success（`200`）

返回结构与 `login` 相同（新的 `access_token/refresh_token`）。

## 4. `POST /api/v1/auth/logout`

- Header：`Authorization: Bearer <access_token>`
- Success：`204 No Content`

## 5. `GET /api/v1/me`

- Header：`Authorization: Bearer <access_token>`

### Success（`200`）

```json
{
  "id": "usr_tenant_admin_jkt_001",
  "email": "tenant.admin@sudirman.co",
  "role": "tenant_admin",
  "tenant_id": "tenant_demo_jakarta",
  "building_ids": ["building_demo_001"]
}
```

## 6. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | JSON decode error | 请求体格式错误 |
| `401` | `invalid credentials` | 登录凭据错误 |
| `401` | `invalid refresh token` | refresh token 无效或过期 |
| `401` | `missing bearer token` | 缺少 `Authorization` |
| `401` | `invalid access token` | access token 无效/已注销 |

