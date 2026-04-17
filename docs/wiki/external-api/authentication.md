# Authentication（Token 与权限）

当前能力状态：

- `CONTRACT_READY`：认证端点与字段稳定，可对接客户端登录/续期逻辑。
- `PROD_READY`：`login/refresh/logout/me` 已在管理端与回归路径长期使用。

## 1. 认证模型

MistyPass 使用 Bearer Token：

- 访问令牌：`access_token`，用于访问受保护 API。
- 刷新令牌：`refresh_token`，用于换取新的 token 对。

默认 TTL（可配置）：

- `access_token`：`1h`（`JWT_ACCESS_TTL`）
- `refresh_token`：`7d`（`JWT_REFRESH_TTL`）

说明（截至 2026-04-15）：

- 管理端 API 默认使用 `Authorization: Bearer <access_token>`。
- 不提供通用 `X-API-Key` 鉴权入口。

## 2. 端点总览

| Endpoint | Method | 用途 |
|---|---|---|
| `/api/v1/auth/login` | `POST` | 账号密码登录，返回 token 对 |
| `/api/v1/auth/refresh` | `POST` | 刷新 token 对 |
| `/api/v1/auth/logout` | `POST` | 注销当前 `access_token` |
| `/api/v1/me` | `GET` | 获取当前用户信息 |

## 3. Login

请求：

```json
{
  "email": "superadmin@mistypass.local",
  "password": "admin123"
}
```

成功响应：

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

失败语义：

- `400`：JSON 结构错误。
- `401`：`invalid credentials`。

## 4. Refresh

请求：

```json
{
  "refresh_token": "<refresh_token>"
}
```

行为说明：

- 刷新令牌采用一次性会话语义，成功刷新后旧 refresh session 会失效。
- 建议客户端收到新 token 对后立即覆盖本地旧 token。

失败语义：

- `400`：JSON 错误。
- `401`：`invalid refresh token`。

## 5. Logout

请求头：

```http
Authorization: Bearer <access_token>
```

响应：

- `204 No Content`：成功注销当前 access token。
- `401`：token 缺失或无效。

## 6. Me

请求头：

```http
Authorization: Bearer <access_token>
```

响应示例：

```json
{
  "id": "usr_tenant_admin_jkt_001",
  "email": "tenant.admin@sudirman.co",
  "role": "tenant_admin",
  "tenant_id": "tenant_demo_jakarta"
}
```

## 7. 角色与租户作用域

- 角色在路由层校验，不满足返回 `403 forbidden`。
- 非 `super_admin` 用户会被强制限制在 token 内 `tenant_id` 作用域。
- `building_admin` 还会追加楼宇范围限制，不在范围内返回 `403 building scope forbidden`。

## 8. 客户端建议

1. 所有受保护请求都附加 `Authorization: Bearer <access_token>`。
2. 接口返回 `401` 时，先尝试 `refresh`，仍失败则回登录。
3. 服务器严格校验 JSON：未知字段会触发 `400`，不要发送未定义参数。

## 9. 相关文档

- Token/Scope 最小权限与排障：`docs/wiki/external-api/guides-api-token-scope-troubleshooting.md`
- 限流与 429 退避：`docs/wiki/external-api/guides-rate-limit-and-429.md`
