# API Reference：Enterprise IdP Config / Validate

当前能力状态：

- `CONTRACT_READY`：IdP 配置读取、更新、校验接口字段与错误语义已稳定。
- `PROD_READY`：OIDC/SAML 必填项、URL 规则、状态枚举与租户作用域约束有回归覆盖。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| IdP Config | `GET /api/v1/enterprise/idp-config` | `super_admin/tenant_admin/operator` |
| IdP Config | `PUT /api/v1/enterprise/idp-config` | `super_admin/tenant_admin` |
| IdP Validate | `POST /api/v1/enterprise/idp-config/validate` | `super_admin/tenant_admin` |

## 2. `GET /api/v1/enterprise/idp-config`

- Auth：`Authorization: Bearer <access_token>`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |

### Success（`200`）

```json
{
  "id": "idp_001",
  "tenant_id": "tenant_demo_jakarta",
  "provider": "oidc",
  "issuer_url": "https://id.sudirman.co",
  "client_id": "mistypass-web-admin",
  "auth_url": "https://id.sudirman.co/oauth2/auth",
  "token_url": "https://id.sudirman.co/oauth2/token",
  "jwks_url": "https://id.sudirman.co/.well-known/jwks.json",
  "user_info_url": "https://id.sudirman.co/oauth2/userinfo",
  "scopes": ["openid", "profile", "email"],
  "status": "active",
  "sync_mode": "jit",
  "updated_by": "system",
  "created_at": "2026-04-15T08:00:00Z",
  "updated_at": "2026-04-15T08:00:00Z"
}
```

## 3. `PUT /api/v1/enterprise/idp-config`

### Request（OIDC 示例）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "provider": "oidc",
  "issuer_url": "https://id.sudirman.co",
  "client_id": "mistypass-web-admin",
  "auth_url": "https://id.sudirman.co/oauth2/auth",
  "token_url": "https://id.sudirman.co/oauth2/token",
  "jwks_url": "https://id.sudirman.co/.well-known/jwks.json",
  "user_info_url": "https://id.sudirman.co/oauth2/userinfo",
  "scopes": ["openid", "profile", "email"],
  "status": "active",
  "sync_mode": "jit",
  "actor": "tenant-admin@sudirman.co"
}
```

### Request（SAML 最小示例）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "provider": "saml",
  "issuer_url": "https://sso.sudirman.co",
  "client_id": "mistypass-saml",
  "saml_acs_url": "https://api.mistypass.local/api/v1/enterprise/auth/saml/callback",
  "saml_x509_cert": "-----BEGIN CERTIFICATE-----...-----END CERTIFICATE-----",
  "status": "active",
  "sync_mode": "scheduled"
}
```

### Success（`200`）

返回完整 `IDPConfig` 对象（同 `GET`）。

行为说明：

- `provider`：仅支持 `oidc` / `saml`。
- `status`：`active`（默认）/ `inactive`。
- `sync_mode`：`jit`（默认）/ `manual` / `scheduled`。
- `scopes`：会去重并转小写。
- `saml` 模式下必须提供 `saml_acs_url`（若缺失且有 `auth_url`，会回退使用 `auth_url`）与 `saml_x509_cert`。

## 4. `POST /api/v1/enterprise/idp-config/validate`

用途：只校验，不落库。

### Request

请求体字段与 `PUT /idp-config` 一致。

### Success（`200`）

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "provider": "saml",
  "valid": false,
  "checked_at": "2026-04-15T10:10:00Z",
  "items": [
    {
      "field": "tenant_id",
      "status": "ok",
      "message": "tenant_id looks good"
    },
    {
      "field": "saml_x509_cert",
      "status": "error",
      "message": "saml_x509_cert is required for saml provider"
    },
    {
      "field": "domain_mapping",
      "status": "warn",
      "message": "no active domain mapping for tenant"
    }
  ]
}
```

`items.status` 取值：`ok` / `warn` / `error`。

## 5. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `tenant_id is required` | 缺少租户 |
| `400` | `invalid idp provider` | `provider` 非 `oidc/saml` |
| `400` | `issuer_url is required` | issuer 缺失 |
| `400` | `client_id is required` | client id 缺失 |
| `400` | `saml_acs_url is required for saml provider` | SAML ACS 缺失 |
| `400` | `saml_acs_url must use https://` | SAML ACS 非 HTTPS |
| `400` | `saml_x509_cert is required for saml provider` | SAML 证书缺失 |
| `400` | `invalid idp status` | `status` 非 `active/inactive` |
| `404` | `enterprise idp config not found` | 读取不存在租户配置 |
| `403` | `tenant scope forbidden` | 租户越权 |
