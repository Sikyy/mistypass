# 企业 Wallet 发卡方案（Google Wallet 优先）

## 1. 目标与范围

### 1.1 目标

- 为企业客户提供可批量、可审计、可撤销的数字门禁卡发卡能力。
- 第一优先级支持 `Google Wallet`，后续再扩展 Apple Wallet。
- 与现有租户、用户、门禁策略、访客通行证模块打通。
- 支持两种运营模式：
  - 平台管理员按租户范围代发卡（员工/访客）。
  - 授权租户管理员在租户内自助发卡。

### 1.2 MVP 范围（第一阶段）

- 支持租户级 Google 发卡配置（Issuer、服务账号、密钥管理）。
- 支持员工卡、访客卡两类数字卡。
- 支持单发与批量发卡。
- 支持卡片状态流转：`draft -> issued -> active -> suspended -> revoked -> expired`。
- 支持发卡日志、状态回执、失败重试。

### 1.3 非目标（MVP 不做）

- Apple Wallet 发卡链路。
- 跨租户模板市场。
- 复杂计费与对账（先保留埋点）。

## 2. 业务流程（Google Wallet）

### 2.1 管理员发卡流程

1. 租户管理员配置 Google Wallet 发卡参数（Issuer、服务账号）。
2. 在后台选择用户/访客与门禁策略，生成发卡任务。
3. 平台创建 Google Wallet 对应 `Class/Object`（或复用 Class，新增 Object）。
4. 平台生成 `Save to Google Wallet` 链接/二维码。
5. 用户在 Android 设备完成“添加到 Google Wallet”。
6. 平台记录发卡结果并同步到审计日志。

### 2.2 状态回收流程

1. 管理员执行挂失/停用/撤销。
2. 平台更新本地卡片状态并调用 Google 对应对象更新接口。
3. 网关和门禁策略模块收到状态变更（事件驱动）。
4. 审计日志记录完整操作轨迹。

## 3. 系统架构建议

## 3.1 新增模块

- `wallet`（发卡域）
- `wallet_google`（Google 适配层）

### 3.2 分层结构

- `domain`: 卡片、模板、发卡任务、状态机
- `application`: 发卡编排、批量任务、重试策略
- `infrastructure`: Google Wallet API client、密钥管理、任务队列
- `delivery/http`: Admin API

### 3.3 关键依赖

- 任务队列（异步发卡与重试）
- 密钥托管（服务账号私钥不落明文）
- 审计日志（发卡与状态变更全量记录）

## 4. 数据模型（建议）

### 4.1 `wallet_issuer_config`

- `id`
- `tenant_id`
- `provider` (`google`)
- `issuer_id`
- `service_account_email`
- `key_ref`（密钥引用，不存明文）
- `status`
- `created_at`, `updated_at`

### 4.2 `wallet_pass_template`

- `id`
- `tenant_id`
- `provider` (`google`)
- `pass_type` (`employee`, `visitor`)
- `class_id`
- `name`
- `style_config`（logo/brand/color）
- `status`
- `created_at`, `updated_at`

### 4.3 `wallet_pass_instance`

- `id`
- `tenant_id`
- `provider`
- `template_id`
- `user_id` / `visitor_pass_id`
- `object_id`
- `status`
- `issued_at`, `activated_at`, `revoked_at`, `expires_at`
- `created_by`, `updated_by`
- `created_at`, `updated_at`

### 4.4 `wallet_issue_job`

- `id`
- `tenant_id`
- `provider`
- `batch_id`
- `target_type` (`user`, `visitor`)
- `target_id`
- `status` (`pending`, `processing`, `success`, `failed`)
- `retry_count`
- `error_code`, `error_message`
- `created_at`, `updated_at`

## 5. API 设计（/api/v1，草案）

### 5.1 配置与模板

- `GET /wallet/google/config`
- `PUT /wallet/google/config`
- `GET /wallet/templates`
- `POST /wallet/templates`
- `PATCH /wallet/templates/{templateId}/status`

### 5.2 发卡与查询

- `POST /wallet/passes/issue`
- `POST /wallet/passes/issue-batch`
- `GET /wallet/passes`
- `GET /wallet/passes/{passId}`
- `GET /wallet/passes/{passId}/save-link`

### 5.3 生命周期操作

- `PATCH /wallet/passes/{passId}/suspend`
- `PATCH /wallet/passes/{passId}/activate`
- `PATCH /wallet/passes/{passId}/revoke`

### 5.4 任务与审计

- `GET /wallet/jobs`
- `GET /wallet/jobs/{jobId}`
- `POST /wallet/jobs/{jobId}/retry`
- `GET /wallet/audit-logs`

## 6. Google Wallet 对接要点

### 6.1 发卡对象策略

- 建议先使用统一 `Class` + 多 `Object` 模式，降低模板维护成本。
- `Object` 与 `user_id/visitor_pass_id` 做稳定映射，避免重复发卡。

### 6.2 链接下发策略

- 后台生成短时效 `save link`（JWT 或预签名链接封装）。
- 支持二维码下载与移动端一键拉起。

### 6.3 可靠性策略

- 外部 API 调用必须幂等（以 `object_id` 为幂等键）。
- 引入指数退避重试与死信队列（DLQ）。
- 关键失败（鉴权/配置错误）触发告警。

## 7. 安全与合规

- 服务账号密钥只存密钥管理系统引用，不在业务库明文保存。
- 所有发卡接口强制租户隔离与 RBAC 校验。
- 发卡、停卡、撤销均写审计日志（操作者、时间、来源 IP、变更前后状态）。
- 对外返回的链接需短时效、可撤销、防重放。

## 8. 分阶段落地建议

### Phase A（1-2 周）

- `SKELETON_ONLY` 完成数据模型与模块骨架。
- 打通 Google 配置校验接口。
- 实现单张发卡（员工卡）最小链路。

### Phase B（2-3 周）

- 支持批量发卡、任务队列与重试。
- 支持访客卡发卡。
- 接入审计与告警。

### Phase C（1-2 周）

- 增加停卡/恢复/撤销。
- 增加运营视图（成功率、失败原因分布、任务耗时）。
- 准备 Apple Wallet 的抽象接口（不实现业务）。

## 9. 与当前代码库的对齐建议

- 复用现有 `tenant`, `access`, `audit`, `alarm` 模块风格。
- 新增 `api/internal/modules/wallet` 与 `api/internal/modules/wallet_google`。
- 先按 in-memory 完成 API 契约联调，再迁移 PostgreSQL。

## 10. 验收标准（Google 优先）

- 能为指定租户配置并验证 Google 发卡参数。
- 能对单个用户生成可添加到 Google Wallet 的卡片。
- 能批量发卡并可追踪每条任务状态。
- 能执行挂起/恢复/撤销并同步状态。
- 全链路有审计可追溯，失败可重试可告警。

## 11. 当前代码状态（2026-04-12）

`CONTRACT_READY` 已在后端完成 `in-memory` 契约链路，便于前后端先联调 API：

- `GET/PUT /api/v1/wallet/google/config`
- `POST /api/v1/wallet/google/config/validate`
- `GET/POST/PATCH /api/v1/wallet/templates`
- `POST /api/v1/wallet/passes/issue`
- `POST /api/v1/wallet/passes/issue-batch`
- `GET /api/v1/wallet/passes`
- `GET /api/v1/wallet/passes/{passId}`
- `GET /api/v1/wallet/passes/{passId}/save-link`
- `PATCH /api/v1/wallet/passes/{passId}/suspend|activate|revoke`
- `GET /api/v1/wallet/jobs`
- `GET /api/v1/wallet/jobs/{jobId}`
- `POST /api/v1/wallet/jobs/{jobId}/retry`
- `GET /api/v1/wallet/audit-logs`

说明：

- 当前 `save-link` 为模拟链接，用于前端流程联调。
- 当前数据已支持 PostgreSQL `mistypass` 快照持久化，并同步到 `mistypass_wallet_*` 投影表。
- `config/validate` 已支持“本地校验 + 可选远端 Google OAuth/Issuer 探测校验”（默认关闭）。
- `BLOCKED_EXTERNAL` 尚未对接真实 Google Wallet 发卡写接口与消息队列（LEI/外部资质完成后推进）。
- 当前受企业 LEI 认证申请前置限制，真实 Google Wallet 发卡 API 暂不可调用。

下一步优先级（建议）：

1. 在 LEI 完成前，优先完善非外部依赖能力：异步任务队列、重试策略、错误码与可观测指标。
2. 持续推进 wallet 与跨模块写入的一致性治理（当前投影已为 `upsert + stale row 清理`，下一步聚焦事件驱动与事务编排）。
3. 继续完善 `save-link/JWT` 生成抽象与接口契约（先 mock 可回放，后切真实签发）。
4. LEI 完成后恢复真实 Google 发卡写接口联调与验收。

### 11.1 Wallet 多租户参数约定（2026-04-11）

为支持平台按指定租户代发卡，`模板/发卡/任务` 接口统一支持 `tenant_id` 选择与过滤：

- `GET /api/v1/wallet/templates?tenant_id=...`
- `POST /api/v1/wallet/templates`（body: `tenant_id`）
- `PATCH /api/v1/wallet/templates/{templateId}/status?tenant_id=...`
- `POST /api/v1/wallet/passes/issue`（body: `tenant_id`）
- `POST /api/v1/wallet/passes/issue-batch`（body: `tenant_id`）
- `GET /api/v1/wallet/passes?tenant_id=...`
- `GET /api/v1/wallet/passes/{passId}?tenant_id=...`
- `GET /api/v1/wallet/jobs?tenant_id=...`
- `GET /api/v1/wallet/jobs/{jobId}?tenant_id=...`

`tenant_id` 不匹配时，详情接口返回 `not found`，避免跨租户读取。

### 11.2 curl 联调样例（多租户代发卡）

```bash
# 1) 为目标租户创建模板
curl -sS -X POST http://localhost:8080/api/v1/wallet/templates \
  -H 'Authorization: Bearer demo-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "tenant_acme_north",
    "pass_type": "employee",
    "name": "ACME 员工卡模板",
    "status": "active",
    "actor": "platform_admin"
  }'

# 2) 平台代发单张卡
curl -sS -X POST http://localhost:8080/api/v1/wallet/passes/issue \
  -H 'Authorization: Bearer demo-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "tenant_acme_north",
    "template_id": "wpt_xxx",
    "target_type": "user",
    "target_id": "usr_acme_2001",
    "actor": "platform_admin"
  }'

# 3) 按 tenant_id 过滤模板/卡片/任务
curl -sS -H 'Authorization: Bearer demo-token' \
  'http://localhost:8080/api/v1/wallet/templates?tenant_id=tenant_acme_north'
curl -sS -H 'Authorization: Bearer demo-token' \
  'http://localhost:8080/api/v1/wallet/passes?tenant_id=tenant_acme_north'
curl -sS -H 'Authorization: Bearer demo-token' \
  'http://localhost:8080/api/v1/wallet/jobs?tenant_id=tenant_acme_north'

# 4) 错租户查询详情（预期 not found）
curl -sS -H 'Authorization: Bearer demo-token' \
  'http://localhost:8080/api/v1/wallet/passes/wps_xxx?tenant_id=tenant_demo_jakarta'
curl -sS -H 'Authorization: Bearer demo-token' \
  'http://localhost:8080/api/v1/wallet/jobs/wjb_xxx?tenant_id=tenant_demo_jakarta'
```

## 12. 后续未实现需求（真实 API + 实机验卡）

以下需求尚未实现，纳入后续迭代 backlog，目标是完成真实发卡与读卡器闭环验证。

### 12.1 真实 Google Wallet API 对接

- 将 `wallet_google` 从 mock/in-memory 适配升级为真实 Google Wallet API client。
- 当前阻塞：企业 LEI 认证申请未完成，真实发卡 API 调用暂缓。
- `config/validate` 从字段校验升级为真实凭据校验：
  - 使用租户维度 `issuer_id + service_account + key_ref` 生成签名 JWT。
  - 调用 Google API 验证 issuer 可用性与权限范围。
  - 返回结构化错误码（鉴权失败、issuer 不存在、权限不足、配额限制、网络异常）。
- 发卡链路改为真实 Class/Object 写入与更新：
  - 以 `tenant_id + target_id + template_id` 作为幂等键。
  - 支持重复请求幂等返回，避免重复发卡对象。
- `save-link` 改为真实 Google 可消费 payload/JWT 链接，而非本地模拟 URL。

### 12.2 多租户发卡流程闭环

- 场景 A：平台管理员为指定租户代发卡（员工/访客）。
- 场景 B：租户管理员在本租户内自助发卡。
- 两个场景均要求：
  - 全链路 RBAC 与租户隔离。
  - 完整审计日志（创建模板、发卡、停卡、恢复、撤销、重试）。
  - 发卡失败可追踪（失败原因、重试次数、最终状态）。

### 12.3 WalletMate II 实机验卡需求（读卡器适配）

- 将 `ACS WalletMate II` 纳入目标读卡器清单，作为后续长期适配设备之一。
- 验证范围：
  - Android + Google Wallet + Google Smart Tap 开门验证。
  - iOS + Apple Wallet + Apple VAS 开门验证。
- 读卡器适配层要求：
  - 统一抽象读卡事件模型（卡标识、租户、门点、时间戳、验证结果）。
  - 对接门禁决策链路，支持在线校验与离线缓存校验策略。
  - 输出统一事件日志，进入 `events/access` 与 `audit`。
- 验证结果要求可追溯：
  - 每次刷卡可关联到 `wallet_pass_instance.object_id` 或等价唯一标识。
  - 可区分失败原因（卡状态无效、租户不匹配、门点不在权限范围、读卡器协议异常）。

### 12.4 实机联调验收用例（新增）

- 用例 1：平台代发员工卡 -> 用户入 Wallet -> WalletMate II 刷卡成功开门。
- 用例 2：租户自助发卡 -> 用户入 Wallet -> WalletMate II 刷卡成功开门。
- 用例 3：卡片挂起后刷卡，预期拒绝并记录拒绝原因。
- 用例 4：卡片撤销后刷卡，预期拒绝并记录拒绝原因。
- 用例 5：跨租户卡片刷卡，预期拒绝并记录租户隔离命中。
- 用例 6：读卡器离线场景下按离线策略判定，并在恢复在线后补齐事件上报。

### 12.5 依赖与前置条件

- 准备生产/测试隔离的 Google issuer 与 service account 凭据。
- 明确 WalletMate II 在项目中的接入路径（网关直连或中间服务转发）。
- 补齐 Apple VAS 抽象接口定义与配置模型（即使暂不实现完整发卡链路）。
- 增加读卡器能力矩阵文档，后续新增设备复用同一验收模板。

### 12.6 企业接入与员工授权联动（新增）

- 客户接入后，支持基于公司域名识别租户（如 `@acme.com` -> `tenant_acme`）。
- 支持接入客户内部登录/身份系统（如 OIDC/SAML/企业 IdP），实现员工身份数据同步。
- 支持员工目录增量同步（新增、离职、部门变更、职位变更）。
- 支持按组织规则自动分配门禁权限（角色、部门、地点、楼宇范围）。
- 支持多通道下发凭证：
  - Email 发送 Wallet 凭证链接/二维码。
  - WhatsApp 发送 Wallet 凭证链接/二维码。
- 支持实体卡制作流程：
  - 卡号与员工账户绑定。
  - 制卡任务、发卡记录、补卡/挂失流程。
  - 与数字卡状态联动（停用/撤销一致性）。

### 12.7 拆分执行清单（可直接排期）

- 工作包 A：真实 Google API 能力
  - 目标：完成 `config/validate` 真校验 + 真实发卡 + 真实 save-link。
  - 交付：Google client、错误码映射、联调脚本、验收报告。
- 工作包 B：企业接入与租户识别
  - 目标：基于企业域名完成租户识别与员工归属。
  - 交付：域名映射模型、租户识别策略、审计日志。
- 工作包 C：企业身份系统与员工同步
  - 目标：接入客户内部登录系统并同步员工数据。
  - 交付：OIDC/SAML 适配器、同步任务、冲突处理策略。
- 工作包 D：权限自动分配与回收
  - 目标：基于员工属性自动授予/回收门禁权限。
  - 交付：规则引擎（MVP 规则集）、变更回放与审计。
- 工作包 E：凭证分发与实体卡
  - 目标：Email/WhatsApp 凭证下发 + 实体卡制作链路。
  - 交付：消息模板、发送回执、制卡任务与状态追踪。
- 工作包 F：WalletMate II 实机闭环
  - 目标：Google Smart Tap 与 Apple VAS 读卡闭环验证。
  - 交付：适配层、读卡日志、成功率与失败原因统计。
