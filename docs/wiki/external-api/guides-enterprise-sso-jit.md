# Guide：Enterprise SSO + JIT Provision

当前能力状态：

- `CONTRACT_READY`：企业登录主链路（start/exchange/oidc-saml callback/logout）与 JIT 审批/回写接口已可联调。
- `PROD_READY`：JIT 停用阻断、审批门禁、external sync callback/worker 已有回归覆盖。

## 1. 目标

让合作企业以 OIDC 或 SAML 接入 MistyPass，并在 `sync_mode=jit` 下完成“登录即建档/或审批后放行”的受控会话签发。

## 2. 关键端点

- 租户识别：`POST /api/v1/enterprise/tenant/resolve`
- 登录起始：`POST /api/v1/enterprise/auth/start`
- 交换模式：`POST /api/v1/enterprise/auth/exchange`
- OIDC callback：`GET /api/v1/enterprise/auth/oidc/callback`
- SAML callback：`POST /api/v1/enterprise/auth/saml/callback`
- 企业会话登出：`POST /api/v1/enterprise/auth/logout`

JIT 审批与回写：

- 列表：`GET /api/v1/enterprise/jit-provision-approvals`
- 审批：`POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/review`
- 待回写：`GET /api/v1/enterprise/jit-provision-approvals/external-sync-pending`
- 回写上报：`POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/external-sync`
- 回写回调：`POST /api/v1/enterprise/jit-provision-approvals/external-sync/callback`

## 3. 推荐流程

### 3.1 识别 tenant

调用 `tenant/resolve`，输入员工邮箱，拿到 `tenant_id` 与域名映射结果。

### 3.2 发起登录

调用 `auth/start`，输入 `email` 或 `tenant_id`，返回：

- `provider`
- `state_token`
- `expires_at`
- OIDC 场景 `authorize_url`，SAML 场景 `sso_url`

`state_token` 为一次性短时票据，默认 5 分钟有效。

### 3.3 回调或交换

两种接入模式都支持：

1. 托管 callback：
- OIDC：`GET /enterprise/auth/oidc/callback?state=...&code=...`（或 `id_token`）
- SAML：`POST /enterprise/auth/saml/callback`（`state + saml_response/idp_token`）

2. 自主交换（SDK/后端代办）：
- `POST /enterprise/auth/exchange`（`email + provider + idp_token`）

成功返回统一结构：

- `tenant_id/provider/sync_mode`
- `jit_applied`
- `external_id`
- `token`（access/refresh/user）

### 3.4 JIT 门禁行为

当 `sync_mode=jit`：

- 若员工活跃且目录匹配，可自动签发会话（必要时自动建档）。
- 若员工状态为 `inactive/terminated/disabled/suspended/deprovisioned`，返回 `403`。
- 若开启 `ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED=true` 且目录未批准，返回 `403 enterprise jit provisioning requires approval`，并落 `pending` 审批记录。

### 3.5 审批与跨系统回写

审批人通过 `review` 执行 `approved/rejected`。

审批后可：

- 由上游系统调用 `external-sync` 上报回写状态（`pending/synced/failed`）。
- 或由上游系统通过 callback token 调用 `external-sync/callback`。
- 后台 worker 会对失败回写进行重试并产生日志告警。

## 4. 错误与状态码（关键）

| HTTP | 典型错误 | 说明 |
|---|---|---|
| `400` | `email or tenant_id is required`、`idp_token is required` | 入参缺失或格式错误 |
| `401` | `domain is not mapped...`、`enterprise idp config is inactive` | 企业身份前置条件不满足 |
| `403` | `tenant scope forbidden`、`enterprise employee is inactive`、`enterprise jit provisioning requires approval` | 作用域或 JIT 门禁拒绝 |
| `409` | `enterprise employee external_id conflict` | 外部身份冲突 |
| `500` | internal error | 服务端异常 |

## 5. 运营侧排障建议

1. 先看 `audit-logs` 中 `enterprise_*` 事件，确认失败点在“验签/门禁/审批/回写”哪一段。
2. 对 JIT 拒绝优先核查员工目录 `status` 与 `external_id` 一致性。
3. 对审批回写失败，结合 `external-sync-pending` 与 worker 告警接口定位。

## 6. 回归脚本

- `docs/testing/curl-enterprise-sync-access-batch.zsh`
- `docs/testing/curl-enterprise-sync-worker-alert.zsh`

## 7. 相关 Reference

- `docs/wiki/external-api/reference-enterprise-auth-start-exchange.md`
- `docs/wiki/external-api/reference-enterprise-tenant-domain.md`
- `docs/wiki/external-api/reference-enterprise-idp-config.md`
- `docs/wiki/external-api/reference-enterprise-employees-sync-jobs.md`
- `docs/wiki/external-api/reference-enterprise-jit-approval-external-sync.md`
