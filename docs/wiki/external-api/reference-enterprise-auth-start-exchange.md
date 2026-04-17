# API Reference：Enterprise Auth Start / Exchange

当前能力状态：

- `CONTRACT_READY`：`auth/start` 与 `auth/exchange` 合同字段稳定。
- `PROD_READY`：OIDC/SAML 校验、JIT 门禁、错误码细分已回归。

## 1. `POST /api/v1/enterprise/auth/start`

### 1.1 用途

生成一次性 `state_token` 与 IdP 跳转入口（OIDC `authorize_url` 或 SAML `sso_url`）。

### 1.2 Request

```json
{
  "email": "alice@example.com",
  "tenant_id": "tenant_demo_jakarta",
  "provider": "oidc",
  "redirect_uri": "https://admin.example.com/sso/callback"
}
```

规则：

- `email` 与 `tenant_id` 至少提供一个。
- 若两者同时提供，必须属于同一 tenant。

### 1.3 Success Response (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "provider": "oidc",
  "sync_mode": "jit",
  "state_token": "est_xxx",
  "redirect_uri": "https://admin.example.com/sso/callback",
  "expires_at": "2026-04-15T09:00:00Z",
  "authorize_url": "https://idp.example.com/oauth2/authorize?..."
}
```

SAML 场景返回 `sso_url`。

### 1.4 Error

| HTTP | 错误 |
|---|---|
| `400` | `email or tenant_id is required` / `invalid redirect_uri` |
| `401` | `domain is not mapped to any tenant` / `enterprise idp config is inactive` |
| `403` | `tenant scope forbidden` |

## 2. `POST /api/v1/enterprise/auth/exchange`

### 2.1 用途

将上游 IdP token 交换为 MistyPass 会话（不走 callback 托管时使用）。

### 2.2 Request

```json
{
  "email": "alice@example.com",
  "provider": "oidc",
  "tenant_id": "tenant_demo_jakarta",
  "idp_token": "<id_token_or_saml_assertion>",
  "external_id": "optional-subject"
}
```

### 2.3 Success Response (`200`)

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "provider": "oidc",
  "sync_mode": "jit",
  "jit_applied": true,
  "external_id": "sub-123",
  "idp_identity": {
    "email": "alice@example.com"
  },
  "token": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "expires_in": 3600,
    "user": {
      "id": "usr_ent_jit_xxx",
      "email": "alice@example.com",
      "role": "building_admin",
      "tenant_id": "tenant_demo_jakarta"
    }
  }
}
```

### 2.4 Error

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `idp_token is required` | 缺少交换凭据 |
| `401` | `invalid ... token/assertion` | IdP 凭据验证失败 |
| `403` | `enterprise employee is inactive` | 员工状态阻断 |
| `403` | `enterprise jit provisioning requires approval` | JIT 审批门禁 |
| `409` | `enterprise employee external_id conflict` | 外部身份冲突 |
| `500` | internal error | 服务端异常 |

## 3. 相关 Callback 端点

- OIDC：`GET /api/v1/enterprise/auth/oidc/callback`（`state` 必填，支持 `code` 或 `id_token`）
- SAML：`POST /api/v1/enterprise/auth/saml/callback`（`state + saml_response/idp_token`）

两者返回结构与 `exchange` 保持一致。

## 4. 相关 Guide

- `docs/wiki/external-api/guides-enterprise-sso-jit.md`
- `docs/wiki/external-api/guides-enterprise-jit-approval-external-sync.md`
