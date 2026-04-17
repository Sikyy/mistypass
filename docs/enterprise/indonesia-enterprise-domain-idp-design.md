# 印尼企业场景：公司域名识别 + 企业登录系统接入设计稿（v1）

## 1. 目标

- 客户接入后，通过公司邮箱域名识别租户。
- 接入客户内部登录系统（OIDC 优先，SAML 次优先），实现统一登录。
- 同步员工目录并按规则分配门禁权限。
- 为后续 Wallet 凭证下发（Email/WhatsApp）和实体卡制作提供数据基础。

## 2. 场景边界

- 本设计聚焦企业接入与身份链路，不覆盖完整计费和 BI 报表。
- 当前阶段先完成 SaaS 后台与 API 能力，读卡器实机属于后续 Sprint。

## 3. 印尼企业场景约束

- 域名形态需要覆盖：`company.co.id`, `company.id`, 子域名（如 `corp.company.co.id`）。
- 集团型客户可能存在多法人、多域名，需要一个租户绑定多个域名。
- 本地运营沟通以 WhatsApp 为主，凭证下发链路需优先支持 WhatsApp。
- 典型组织分布跨城市（Jakarta/Surabaya/Bandung 等），权限规则需支持“地点 -> 楼宇”映射。

## 4. 架构设计

- `enterprise` 模块（新增）：
  - `domain mapping`：域名到租户映射。
  - `idp config`：企业登录配置（OIDC/SAML）。
  - `employee sync`：员工目录同步与权限模板计算。
  - `sync job`：同步任务结果与审计。
- 与现有模块关系：
  - `auth`：登录会话签发（企业身份交换后发平台 token）。
  - `access`：后续承接自动权限分配结果。
  - `wallet`：后续承接凭证下发对象来源。

## 5. 数据库结构（目标模型）

- 详细 DDL 见：
  - `docs/enterprise/enterprise-schema.sql`

核心表：

- `enterprise_domain_mapping`
  - 多域名绑定租户，域名全局唯一，支持启停。
- `enterprise_idp_config`
  - 租户级 IdP 配置，支持 OIDC/SAML。
- `enterprise_employee`
  - 员工主数据与已计算权限模板（role/building/group）。
- `enterprise_sync_job`
  - 同步任务统计（created/updated/deactivated/rejected）。
- `enterprise_identity_link`
  - 企业身份与平台用户映射（外部 subject -> 内部 user_id）。

## 6. 功能设计

### 6.1 域名识别流程

1. 客户管理员在后台录入公司域名（可多条）。
2. 用户输入邮箱时，系统提取 `@domain`。
3. 通过 `domain_mapping` 查找活跃租户映射。
4. 返回租户上下文，用于后续登录路由和安全校验。

### 6.2 企业登录接入流程（OIDC）

1. 租户管理员配置 OIDC 参数（issuer/client/endpoints/scopes）。
2. 用户通过企业 IdP 完成认证，前端拿到 `idp_token`。
3. 后端执行 `auth/exchange`：
   - 校验邮箱域名归属租户。
   - 校验租户 IdP 配置是否可用。
   - 兑换 MistyPass 会话令牌。
4. 写入审计日志（tenant/provider/email/result）。

### 6.3 员工同步与权限分配

1. 触发同步（手动或定时任务）。
2. 接收员工数据：`external_id/email/department/job_title/location/status`。
3. 进行域名归属校验与字段清洗。
4. 执行权限模板计算：
   - `department` -> `access_role`
   - `location` -> `building_id`
   - `tenant` -> `default_group_ids`
5. 写入员工表与同步任务统计。

## 7. API 设计规范（v1）

公共接口：

- `POST /api/v1/enterprise/tenant/resolve`
- `POST /api/v1/enterprise/auth/exchange`

管理接口（Bearer + RBAC）：

- `GET /api/v1/enterprise/domain-mappings`
- `POST /api/v1/enterprise/domain-mappings`
- `PATCH /api/v1/enterprise/domain-mappings/{mappingID}/status`
- `GET /api/v1/enterprise/idp-config`
- `PUT /api/v1/enterprise/idp-config`
- `POST /api/v1/enterprise/idp-config/validate`
- `GET /api/v1/enterprise/employees`
- `POST /api/v1/enterprise/employees/sync`
- `GET /api/v1/enterprise/sync-jobs`

统一规范：

- 读接口返回 `200` + `{items:[]}` 或对象。
- 写接口返回 `201/202/200`，失败使用 `4xx/5xx` + `{error:string}`。
- 所有管理接口强制租户隔离，`tenant_admin` 不可跨租户写入。

## 8. 权限与安全规范

- `super_admin`：可跨租户配置与排障。
- `tenant_admin`：仅可管理本租户域名、IdP、同步任务。
- `operator`：只读企业配置与同步结果，不可变更。
- 安全要求：
  - 不在业务库明文保存企业密钥，使用 `secret_ref`。
  - 登录交换接口必须校验租户归属与 provider 匹配。
  - 审计字段至少包含 `actor/tenant/action/result/request_id/timestamp`。

## 9. 印尼场景的规则模板（MVP）

- 域名规则：
  - 统一小写、去掉前导 `@`、禁止 URL 形式。
  - 支持 `co.id/id` 及企业子域。
- 权限分配模板（可配置化前的默认策略）：
  - `security/satpam/guard` -> `operator`
  - `facility/engineering/building` -> `building_admin`
  - 其他 -> `resident`
- 地点映射模板：
  - `jakarta/jkt/sudirman` -> Jakarta 楼宇
  - `factory/pabrik/bandung/bekasi` -> Factory 楼宇

## 10. 实施现状（本次已落地）

- `CONTRACT_READY` 已新增 `enterprise` 模块 in-memory 契约实现：
  - 域名映射管理
  - 租户识别
  - IdP 配置管理与校验
  - 员工同步与权限模板计算
  - 同步任务记录
- 已新增对应 API 路由并接入 RBAC 与租户隔离。
- 员工同步结果已联动 `access/users`：
  - 幂等键优先级：`tenant_id + external_id`，缺失时回退 `tenant_id + email`。
  - 写入 access 时透传 `sync_source + sync_ref`（`external_id` 优先、`email` 回退）以维持跨模块因果链。
  - 同步响应返回 `access_sync.created/updated/rejected` 统计。
- `PROD_READY` `enterprise/auth/exchange` 已升级为 OIDC + SAML 双通道验签链路：
  - OIDC：验证 JWT 签名（JWKS）、`iss`（issuer）、`aud`（client_id）、`email` 一致性。
  - SAML：验证 Signed Assertion 签名（X509 证书）、issuer/audience/recipient/时效、`email` 一致性。

## 11. 下一步实施

- `enterprise` 状态已落 PostgreSQL `mistypass` + `mistypass_enterprise_*` 投影，下一步聚焦跨模块一致性事务编排与事件驱动增量回放。
- 扩展 `auth/exchange`：补齐 OIDC/SAML 回调编排、JIT 自动建档、会话与登出联动。
- 把员工同步结果写入 `access/users/user-groups` 的正式模型并做幂等更新。
- 对接 Email/WhatsApp 下发通道与模板审批流程。
