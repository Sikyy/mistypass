# 企业 OIDC/SAML 回调编排草案（2026-04-14）

当前能力状态：

- `CONTRACT_READY`：`/enterprise/auth/start`、`/enterprise/auth/exchange`、`oidc/saml callback`（含 OIDC `code -> id_token` 交换）与 `/enterprise/auth/logout` 已形成联调链路；`sync_mode=jit` 支持目录缺失自动建档与停用阻断。
- `SKELETON_ONLY`：OIDC `nonce` 强约束、SCIM/HRIS 深属性映射与跨系统撤权联动仍待落地。

## 1. 目标

- 固化企业登录 callback 的统一编排契约，避免 OIDC/SAML 两条链路接口语义分叉。
- 将“验证 -> 归属校验 -> 会话签发 -> 审计留存”固定为单一流程。

## 2. 编排总览

1. 前端发起企业登录（输入邮箱或 tenant）。
2. 服务端生成一次性 `state_token`（短 TTL，绑定 `tenant_id + provider + redirect_uri`）。
3. 用户在 IdP 完成认证后回调到 Cloud callback。
4. Cloud 校验 `state_token` 与 provider 响应。
5. Cloud 将标准化身份载荷转给 `enterprise/auth/exchange`（或内联同等逻辑）。
6. 根据 `sync_mode` 执行会话签发：
   - `manual/scheduled`：仅允许已存在本地用户。
   - `jit`：本地缺失时允许企业员工档案回退签发。
7. 返回 token + 审计字段 + 风险标签。

## 3. 建议接口契约（草案）

### 3.1 启动登录（统一）

- `POST /api/v1/enterprise/auth/start`
- request：
  - `email`（可选，优先用于 tenant resolve）
  - `tenant_id`（可选，若传入需与 resolve 结果一致）
  - `provider`（可选，默认使用租户已激活 idp_config.provider）
  - `redirect_uri`（必填，回调落地地址）
- response：
  - `tenant_id`
  - `provider`
  - `authorize_url`（OIDC）或 `sso_url`（SAML）
  - `state_token`
  - `expires_at`

### 3.2 OIDC callback

- `GET /api/v1/enterprise/auth/oidc/callback?code=...&state=...`
- server actions：
  - 校验 `state`。
  - 用 `code` 换取 `id_token`。
  - 调用现有 OIDC 验签逻辑。
  - 进入统一 exchange 会话签发。

### 3.3 SAML callback

- `POST /api/v1/enterprise/auth/saml/callback`
- request form/json：
  - `saml_response`
  - `state`
- server actions：
  - 校验 `state`。
  - 调用现有 SAML 验签逻辑。
  - 进入统一 exchange 会话签发。

### 3.4 统一退出（联动）

- `POST /api/v1/enterprise/auth/logout`
- request：
  - `refresh_token`（或 access token）
  - `tenant_id`
- server actions：
  - 本地会话失效。
  - 记录是否触发上游 IdP 登出跳转（best-effort）。

## 4. 状态与安全约束

- `state_token`：一次性、默认 TTL 5 分钟、消费后即失效。
- `nonce`：OIDC 必须校验；SAML 通过断言签名与时间窗校验替代。
- `redirect_uri`：必须命中租户白名单（避免 open redirect）。
- 失败返回统一：`{"error":"..."}`，并区分 `400/401/403/409`。

## 5. 审计字段（最低要求）

- `tenant_id`
- `provider`
- `sync_mode`
- `jit_applied`
- `external_id`
- `request_id`
- `failure_code`（失败时）

## 6. 里程碑拆分

1. `M1`：落地 `auth/start` + `state_token` 存储与 TTL 校验。
2. `M2`：落地 OIDC/SAML callback endpoint，复用现有验签能力。
3. `M3`：统一登出接口 + 审计扩展 + 回归脚本。

## 7. 非目标

- 本轮不引入真实第三方企业租户生产凭证。
- 本轮不处理跨区域多活会话一致性。
