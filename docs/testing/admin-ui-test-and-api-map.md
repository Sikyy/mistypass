# 管理后台联调测试说明与页面 API 对照（2026-04-15）

## 1. 测试入口

- 前端地址：`http://localhost:5173/`
- 后端地址：`http://localhost:8080/`
- 健康检查：`GET /healthz`

> 当前会话已为你启动前后端开发服务，可直接打开前端地址开始测试。

## 2. 登录注意事项

### 2.1 登录页调用 API

- 页面：`/login`
- API：`POST /api/v1/auth/login`
- 用途：校验账号密码并返回 `access_token/refresh_token`。

### 2.2 Token 行为

- 前端把 `access_token` 存在 `sessionStorage`：`mistypass_admin_access_token`。
- 前端把 `refresh_token` 存在 `sessionStorage`：`mistypass_admin_refresh_token`。
- 后续所有管理页 API 都通过 `Authorization: Bearer <token>` 访问。
- 当前管理端 UI 已接入 refresh token 流程；刷新失败或 refresh token 失效后需要重新登录。

### 2.3 测试账号（建议）

- `superadmin@mistypass.local / admin123`：跨租户最高权限（推荐先用它冒烟）。
- `tenant.admin@sudirman.co / admin123`：`tenant_demo_jakarta` 租户管理员。
- `building.admin.sudirman@mistypass.local / admin123`：楼宇管理员（有楼宇范围限制）。
- `ops.jkt.01@mistypass.local / admin123`：运营角色。

### 2.4 常见登录失败原因

- 使用了旧示例账号 `admin@mistypass.local`（历史文档残留）。
- 前端未连到正确后端（确认 `VITE_API_BASE_URL`，默认 `http://localhost:8080`）。
- token 过期、refresh 失败或浏览器 `sessionStorage` 残留旧会话（可清空后重登）。

## 3. 页面与 API 对照

### 3.1 已挂载路由

1. `/login`
- 用途：登录并写入 token。
- API：`POST /api/v1/auth/login`

2. `/dashboard`
- 用途：概览租户与网关运行状态。
- API：`GET /api/v1/tenants`、`GET /api/v1/gateways`

3. `/tenants`
- 用途：租户列表、创建租户、变更租户状态。
- API：`GET /api/v1/tenants`、`POST /api/v1/tenants`、`PATCH /api/v1/tenants/{tenantID}/status`

4. `/tenants/:tenantID`
- 用途：查看单租户拓扑（楼宇/楼层/区域/门点）。
- API：`GET /api/v1/tenants`、`GET /api/v1/tenants/{tenantID}/topology`

5. `/spaces`
- 用途：空间资产管理（楼宇、楼层、区域、门点）。
- 本轮增强：租户选择支持“先搜索再下拉”。
- API：
  - 查询：`GET /api/v1/buildings|floors|areas|doors?tenant_id=...`
  - 创建：`POST /api/v1/buildings|floors|areas|doors`

6. `/access`
- 用途：权限策略、用户组、临时权限管理。
- 本轮增强：
  - 授权台账支持实时有效期倒计时、授权人/授权时间展示、日期筛选。
  - 点击“查看”可弹出被授权人详情。
  - 范围选择强制互斥（`all/building/area/door`），避免“全部+局部”混选。
  - 用户组页增加岗位自动分组与权限模板说明（与企业同步模板一致）。
- API：
  - 策略：`GET/POST/PATCH /api/v1/access-policies`
  - 用户组：`GET/POST/PATCH /api/v1/user-groups`
  - 临时权限：`GET/POST /api/v1/temporary-access`（返回 `authorized_by_*`、`authorized_at`）
  - 企业员工：`GET /api/v1/enterprise/employees?tenant_id=...`
  - 企业同步补偿：`POST /api/v1/enterprise/employees/sync/reconcile`（按 `request_id` 重放 access 写入）
  - 企业同步补偿审计：`GET /api/v1/enterprise/sync-requests`（按 `request_id` 查询补偿状态/尝试次数/错误）
  - 企业同步 worker 告警：`GET /api/v1/enterprise/sync-worker-alerts`（统一覆盖 reconcile / HRIS DLQ / HRIS pull 三类 worker 的结构化阈值告警，支持 `since/until`）
  - 企业同步 worker 告警汇总：`GET /api/v1/enterprise/sync-worker-alerts/summary`（按 `tenant + worker_action` 聚合 `count/first_seen_at/last_seen_at`，支持 `since/until`）
  - 企业同步 worker 告警订阅：`GET /api/v1/enterprise/sync-worker-alert-subscription`、`PUT /api/v1/enterprise/sync-worker-alert-subscription`（wallet 风格订阅策略，支持默认值回退、partial update、tenant scope）
  - 企业同步补偿批处理：`POST /api/v1/enterprise/sync-requests/reconcile-pending`（按租户批量处理 pending）
  - 辅助数据：`GET /api/v1/tenants`、`GET /api/v1/buildings|areas|doors?tenant_id=...`

7. `/wallet`
- 用途：Wallet 队列运营看板（状态分布、窗口指标、趋势图、阈值告警、DLQ 清理归档、跨租户风险排行、告警订阅策略、告警发送记录）。
- API：
  - `GET /api/v1/wallet/jobs/metrics?tenant_id=...&window_seconds=...&max_retry=...&dlq_alert_threshold=...`
  - `GET /api/v1/wallet/jobs/metrics/trend?tenant_id=...&window_seconds=...&bucket_count=...&max_retry=...&dlq_alert_threshold=...`
  - `GET /api/v1/wallet/jobs/alert-subscription?tenant_id=...`
  - `PUT /api/v1/wallet/jobs/alert-subscription`
  - `GET /api/v1/wallet/jobs/alert-notifications?tenant_id=...&limit=...`
  - `POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry`
  - `POST /api/v1/wallet/jobs/alerts/dispatch`（返回 `items[].channel_results` 统一通道回执）
  - `GET /api/v1/wallet/jobs/dlq/cleanup/archives?tenant_id=...&limit=...`
  - 辅助：`GET /api/v1/tenants`

8. `/gateways`
- 用途：网关列表、注册、绑定门、发布配置、重启。
- 本轮增强：
  - 门点绑定改为可选门点列表，不再要求手输 ID。
  - 发布配置/重启提供命令进度面板（已入队→下发中→设备已收→执行回执）。
  - 新增序列号库存管理面板：支持单条入库、CSV 入库、CSV 导出、筛选查询、批量状态流转（回库/冻结/报废）与核销状态查看。
  - 新增 checkpoint 趋势窗口面板：按当前网关展示 `time_window_trend`（窗口上报次数、acked 增量、方向、队列明细）。
- API：
  - `GET /api/v1/gateways`
  - `GET /api/v1/gateways/serial-inventory`
  - `POST /api/v1/gateways/serial-inventory/import`
  - `POST /api/v1/gateways/serial-inventory/import-csv`
  - `PATCH /api/v1/gateways/serial-inventory/batch-status`
  - `PATCH /api/v1/gateways/serial-inventory/{serialNumber}/status`
  - `GET /api/v1/gateways/serial-inventory/export-csv`
  - `POST /api/v1/gateways/register`
  - `POST /api/v1/gateways/{gatewayID}/bind-door`
  - `POST /api/v1/gateways/{gatewayID}/unbind-door`
  - `POST /api/v1/gateways/{gatewayID}/devices`
  - `POST /api/v1/gateways/{gatewayID}/devices/{deviceID}/rs485/telemetry`
  - `POST /api/v1/gateways/{gatewayID}/devices/probe-legacy`
  - `POST /api/v1/gateways/{gatewayID}/config/publish`
  - `POST /api/v1/gateways/{gatewayID}/reboot`
  - `GET /api/v1/gateways/events/checkpoint/summary`（支持 `gateway_id/tenant_id/trend_window_minutes`）
  - 辅助：`GET /api/v1/tenants`
- 序列号规则：
  - 我方产品需先导入序列号库存（`serial-inventory/import`）再注册；注册成功后序列号会核销为 `consumed`。
  - 已支持库存状态流转：`available`（可用/回库）、`frozen`（冻结）、`scrapped`（报废），冻结/报废状态不可用于注册核销。
  - 批量状态流转可通过 `serial-inventory/batch-status` 一次处理多条序列号。

9. `/events`
- 用途：访问事件与设备事件查询。
- API：`GET /api/v1/events/access`、`GET /api/v1/events/device`、`GET /api/v1/tenants`

10. `/alarms`
- 用途：告警列表与告警状态流转。
- 本轮增强：增加通知策略（默认安全组，邮件/WhatsApp 通道）与通知日志队列视图。
- API：`GET /api/v1/alarms`、`PATCH /api/v1/alarms/{alarmID}/status`、`GET /api/v1/tenants`

11. `/enterprise`
- 用途：企业目录、同步、IDP、审批异常与 HRIS connector 运维总览。
- 本轮增强：
  - `sync` 工作区已新增 Talenta connector 配置面板，支持 `webhook / pull / hybrid` 策略切换。
  - 支持在 UI 中选择已有 HRIS vault secret ref，或直接写入新的 connector credential / webhook secret。
  - 已展示稳定 webhook URL、当前 connector refs、最近 secret refs 与其他 HRIS connectors 摘要。
  - `alerts` 工作区已支持基于 `worker_action / worker_alert_label / worker_kind / worker_filter_hint / worker_query_hint` 的精确 worker scope，并可按当前筛选批量跳转到目录、策略、同步三条处置入口；Talenta receipt / DLQ 也已补 `queue_state / replay_state` 运行态快捷筛选按钮，且与 `worker_status / worker_queue_state / worker_replay_state` URL hint/runtime scope 双向同步；receipt 已支持单条 `process` 与按当前可见且 `queue_state=ready` 的 `process-batch`，DLQ 也已支持单条 `replay` 与按当前可见且 `replay_state=ready` 的 `replay-batch`。上述 4 个手动入口当前在 UI 默认都传 `execution_mode=queued`，先 claim、持久化 execution record（`dispatch_mode=worker_tick`）再返回 queued summary；worker enabled 时会主动 wake 对应 worker，由 worker tick 扫描 `queued` execution 并继续执行，worker disabled 时才回退本进程后台异步处理。
- API：
  - 目录与同步：`GET /api/v1/enterprise/employees`、`GET /api/v1/enterprise/sync-jobs`、`POST /api/v1/enterprise/employees/sync`
  - HRIS connector：`GET /api/v1/enterprise/hris-connectors`、`POST /api/v1/enterprise/hris-connectors`、`PATCH /api/v1/enterprise/hris-connectors/{connectorID}`
  - HRIS vault metadata：`GET /api/v1/enterprise/hris-secrets`
  - HRIS 运行态：`GET /api/v1/enterprise/sync-worker-alert-subscription`、`PUT /api/v1/enterprise/sync-worker-alert-subscription`、`GET /api/v1/enterprise/sync-worker-alerts`、`GET /api/v1/enterprise/sync-worker-alerts/summary`、`GET /api/v1/enterprise/hris-webhook-receipts`、`POST /api/v1/enterprise/hris-webhook-receipts/{receiptID}/process`、`POST /api/v1/enterprise/hris-webhook-receipts/process-batch`、`GET /api/v1/enterprise/hris-webhook-dlq`、`POST /api/v1/enterprise/hris-webhook-dlq/{entryID}/replay`、`POST /api/v1/enterprise/hris-webhook-dlq/replay-batch`、`GET /api/v1/enterprise/hris-pull-states`
  - `GET /api/v1/enterprise/hris-webhook-receipts` 与 `GET /api/v1/enterprise/hris-webhook-dlq` 现已支持 `connector_id / status / q / offset / limit`，并分别支持 `queue_state` / `replay_state` runtime filter；返回体除 `items` 外，也会补 `total / has_more / next_offset` 与 `queue_counts / replay_counts`，便于前端按真实 backlog 做增量加载与 runtime 快捷筛选。
  - `POST /api/v1/enterprise/hris-webhook-receipts/{receiptID}/process`、`POST /api/v1/enterprise/hris-webhook-receipts/process-batch`、`POST /api/v1/enterprise/hris-webhook-dlq/{entryID}/replay`、`POST /api/v1/enterprise/hris-webhook-dlq/replay-batch` 现都支持可选 `execution_mode=queued`；当前实现语义是“claim + 持久化 execution record（`dispatch_mode=worker_tick`）+ 立即返回 queued + wake worker，由 worker tick 扫描 `queued` execution 继续执行”；仅在 worker disabled 时才 fallback 到进程内后台异步执行，并非真正外部队列。
  - IDP 与审批：`GET /api/v1/enterprise/idp-config`、`GET /api/v1/enterprise/jit-provision-approvals`、`POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/review`

### 3.2 运维接口挂载状态

- 页面文件：`web-admin/src/features/audit/pages/audit-page.tsx`
- API 已有：`GET /api/v1/audit-logs`
- `/audit` 路由已挂载，并已接入 Audit Webhook config / deliveries / dispatch。
- 设备 bootstrap 运维接口（仅网关设备侧调用，当前未在 UI 暴露）：
  - `POST /api/v1/gateway/register`
  - `POST /api/v1/gateway/activate`
  - `POST /api/v1/gateway/heartbeat`
  - `POST /api/v1/gateway/status`
  - `POST /api/v1/gateway/config/pull`
  - `POST /api/v1/gateway/config/applied`
  - `POST /api/v1/gateway/events/access`
  - `POST /api/v1/gateway/events/device`
  - `POST /api/v1/gateway/events/batch`
  - `POST /api/v1/gateway/events/checkpoint`
  - `GET /api/v1/gateways/{gatewayID}/events/checkpoint`（管理端运维查询）
  - `GET /api/v1/gateways/events/checkpoint/summary`（管理端队列 lag 摘要）
- 运维 API（仅 super_admin，当前未在 UI 暴露）：
  - `GET /api/v1/state/change-log?state_key=...&limit=...`
  - `POST /api/v1/state/change-log/replay`（`state_key/from_id/limit`）
  - `GET /api/v1/state/change-log/checkpoints?state_key=...&limit=...`
  - `POST /api/v1/state/change-log/replay/checkpoint`（`state_key/limit`）
  - `GET /api/v1/wallet/jobs/summary?tenant_id=...&max_retry=...`
  - `POST /api/v1/wallet/jobs/process`
  - `POST /api/v1/wallet/jobs/{jobID}/dlq/requeue`
  - `POST /api/v1/wallet/jobs/dlq/requeue`
  - `POST /api/v1/wallet/jobs/dlq/cleanup`

## 4. 当前项目建设情况（汇总）

### 4.0 能力状态标识

- 统一口径见：`docs/architecture/capability-status-markers.md`
- 本文使用的状态标识：
  - `PROD_READY`：生产可用闭环已具备。
  - `CONTRACT_READY`：API 契约可联调（可包含 mock/in-memory 路径）。
  - `BLOCKED_EXTERNAL`：受外部依赖阻塞，暂不具备真实生产验收条件。

### 4.1 已完成（可联调）

- 后端 Go + Chi 可运行，JWT + RBAC + 多租户隔离生效。
- 管理端核心域模型接口（租户、空间、权限、网关、事件、告警）已打通。
- 关键易用性修复已落地（租户详情白屏、空间页租户搜索、授权台账可追踪、网关命令进度、告警通知配置）。
- `building_scope` 动态更新链路已回归通过。
- 企业接入第一批能力已接通：
  - 域名识别、IdP 配置、员工同步。
  - `enterprise/auth/exchange` 已支持 `OIDC + SAML` 验签。
  - `enterprise/auth/start` + `oidc/saml callback` 合同层接口已落地，可联调 `state_token` 一次性消费流程；OIDC callback 已支持 `code -> id_token` 交换。
  - `enterprise/auth/logout` 已支持企业会话 `access_token/refresh_token` 联动撤销。
  - `enterprise/auth/exchange` 在 `sync_mode=jit` 下支持“本地账号缺失 -> 企业员工档案回退发会话”。
  - 企业 callback/exchange 会话签发错误码已细分：`inactive=403`、`external_id 冲突=409`、输入错误 `400`、其余内部错误 `500`。
  - 路由级状态码回归已覆盖 `exchange + OIDC callback + SAML callback` 三条链路（`sync_mode=jit` 下 `inactive=403`、`external_id 冲突=409`）。
  - JIT 新增身份声明停用阻断：当 OIDC/SAML 声明携带 `employment_status/status` 为 `inactive/terminated`（或 `active=false`）时，直接拒绝会话签发并返回 `403`。
  - JIT 目录优先级（第一阶段）：对 `source` 含 `scim/hris` 的员工，callback claims 不再覆盖已有 `full_name/department/job_title/location`，仅填充空字段，避免目录快照被登录态声明回写污染。
  - JIT 深属性映射（第二阶段）：回调身份属性已纳入 `phone/manager_external_id/employment_status` 归一；`OIDC(active/status/employment_status)` 与 SAML 对应属性统一口径后写入 employee 档案。
  - 停用撤权联动（第一阶段）：命中 `inactive` 拦截时批量撤销同邮箱 refresh session，并写审计 `enterprise_jit_deprovision_applied`（可由 `GET /api/v1/audit-logs?action=enterprise_jit_deprovision_applied&source=enterprise_auth` 查询）。
  - 组织审批流门禁（第一阶段）：启用 `ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED=true` 后，仅已存在目录员工允许 `sync_mode=jit` 会话签发；目录缺失返回 `403` 并写审计 `enterprise_jit_approval_required`。
  - JIT 审批流最小闭环：目录缺失拦截会落库 `pending` 审批项，可通过 `GET /api/v1/enterprise/jit-provision-approvals` 查询，并由 `POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/review` 审批放行后再次登录。
  - JIT 跨系统回写编排（第一阶段）：提供待回写队列 `GET /api/v1/enterprise/jit-provision-approvals/external-sync-pending` 与回写结果上报 `POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/external-sync`（`synced/failed` + `external_sync_ref` + `last_error`）。
  - JIT 跨系统回写编排（第二阶段）：新增外部主动回调入口 `POST /api/v1/enterprise/jit-provision-approvals/external-sync/callback`（callback token 校验），并上线失败自动重试 worker（阈值告警审计 `enterprise_jit_approval_external_sync_worker_alert`）。
  - 停用撤权联动（第二阶段）：命中 `inactive` 拦截时，本地 trusted user 自动降级为最小权限（`role=resident` + 清空楼栋范围），并在 `enterprise_jit_deprovision_applied` 审计附带 `old_role/new_role/downgraded_local` 字段。
- `CONTRACT_READY` Wallet 第一阶段 API 契约已完成（当前 in-memory/mock）。

### 4.2 进行中/待完成

- 企业与 wallet 已接入 PostgreSQL `mistypass` 快照持久化并同步 `mistypass_*` 投影表；当前 `tenant/space/access/gateway/enterprise/event/alarm/audit/wallet` 全模块已升级为 `upsert + stale row 清理`。
- PostgreSQL replay 已补“多租户 + 多 `state_key` 分层曲线”回归（短时基线），长时 soak 与 nightly 留档仍在推进。

### 4.3 Enterprise HRIS 当前优先级（2026-04-25）

- 自动化回归已补 Talenta `changeschedule` webhook merge、`merge miss -> DLQ -> replay`、`hybrid webhook + pull worker`，以及 DLQ / pull worker 的 `failure alert + retry cooldown + attempt limit` 主链路；同时已补 `GET /api/v1/enterprise/sync-worker-alerts` 与 `/summary` 的路由级过滤 / 聚合回归。
- 后端已把 HRIS webhook `normalize/merge/sync` 失败补入 `sync-worker-alert` 聚合（`enterprise_hris_webhook_processing_alert`），Talenta merge miss / 字段缺失不再只停留在普通 audit。
- 前端已补 Talenta connector Playwright smoke：`/enterprise#sync` 下已覆盖 `existing ref` 创建、`inline secret` 创建、`hybrid + incremental` 更新，以及保存失败态的首批错误分类与 retry suggestion；当前已校验 `credential_ref` 缺失与 upstream `429/503` 临时失败两类分支，并继续校验 `POST/PATCH /api/v1/enterprise/hris-connectors` 请求体与 reload 后 webhook URL / refs 回显。
- 前端已补 `worker alerts` Playwright smoke：`/enterprise#sync` 下消费 `GET /api/v1/enterprise/sync-worker-alerts/summary`，校验 hot/stable 过滤、URL hint 回流、二次复核入口跳转参数，以及首批运行期恢复指引（冷却、字段映射/merge 分类）。
- 前端已补 `sync -> alerts -> sync` 的 worker alert 跨 workspace Playwright smoke，覆盖 `#alerts` 目录异常视图、查询 hint 回填、回跳同步页、平台租户切换、按 `worker_action / worker_label / worker_kind` 的精确定位，以及告警页的精简态 worker guidance；最新也已覆盖 Talenta receipt / DLQ 运行态快捷筛选按钮、receipt 单条 `process`、receipt `process-batch`、DLQ 单条 `replay`、DLQ `replay-batch`、最近批处理明细面板、URL hint/runtime scope 回填、以及回跳同步页后的 scope 保留。2026-04-24 新增 queued 回归后，这 4 个手动动作都已验证默认走 `execution_mode=queued`，并覆盖单条/批量 summary 与 batch result 明细；同日后端也补上 execution record + wake worker 分发回归，确认 worker enabled 时会先落 `dispatch_mode=worker_tick` 的 execution record，再由 worker tick 扫描 `queued` execution 执行，而不是由请求线程直接承接主路径。随后又补了 running worker 的共享 state refresh 回归：已启动的第二实例在 tick 前会主动刷新 enterprise/DLQ state，因此无需重启也能接住另一实例刚写入的 queued receipt / queued replay。
- `#alerts` 现已接通 read-only 运行期数据面：在 summary 之外继续展示 raw worker alerts、HRIS webhook receipt queue、HRIS pull states、DLQ 四段明细；其中 receipt queue 会直接暴露 `queue_state / next_retry_at / processing_deadline_at / remaining_attempts / cooldown_remaining_seconds / stale_in_flight`，DLQ 会直接暴露 `replay_state / next_retry_at / processing_deadline_at / remaining_attempts / cooldown_remaining_seconds / stale_in_flight`，并在 Talenta 告警页提供对应的 receipt / DLQ 快捷筛选按钮，以及单条 `process/replay` 与按当前可见结果执行的 `process-batch/replay-batch` 入口；最新列表接口与前端处置面也已一起补 `offset / limit / has_more / next_offset / queue_counts / replay_counts` 的真实 backlog 增量加载，`#alerts` 现支持 receipt / DLQ 的 `Load more`、pagination summary 与服务端 runtime counts 消费。最近一轮 batch 的汇总与条目级结果也已直接展示在卡片内，便于按 `vendor / connector / request / failure_stage` 做一线排障。
- 从 `#alerts` 的 receipt / DLQ 卡片跳回 `#sync` 时，当前也会保留 `worker_status / worker_queue_state / worker_replay_state` scope，便于在同步工作区继续沿着同一 Talenta 故障上下文处置。
- `#alerts` 当前也可直接消费 `worker_status / worker_queue_state / worker_replay_state` URL hints，并把对应 scope 应用到 receipt / DLQ 列表筛选，便于按 `received+ready`、`dlq+cooldown` 这类组合直接落到目标条目。
- `#alerts` 当前已支持把当前筛选批量跳转到目录、策略、同步三条处置路径，并已接入 `sync-requests/reconcile-pending`、receipt `process-batch` 与 DLQ `replay-batch` 的真实执行入口；Talenta connector 保存失败态与运行期 worker alert 都已补首批错误分类与 retry suggestion。`GET /enterprise/hris-webhook-executions` 统一 execution history 列表/详情已接通，queued 单条/批量 receipt 与 DLQ 响应都会回传 `execution_id`；`#alerts` 前端也已补 execution history 面板，支持按 `kind / status / q / queue_state / replay_scope` 查看、增量加载、详情 drill-down、`execution_id` 深链，以及从 failed execution 直接触发新的 queued replay/process。通知策略、告警去重与 execution history 第二阶段细化均已落地，不再属于 Talenta 待办。
- `attendance.liveattendance` 继续 deferred；Talenta 单厂商公开 webhook 范围已收口。最新后端已把 receipt / DLQ worker 候选收窄到 claimable 的 due-now / stale 项，cooldown / attempt-limit / fresh processing/replaying 继续通过 `queue_state / replay_state` 暴露，但不再进入后台 worker loop 产生额外 skip-only alert。`execution_mode=queued` 的第二阶段已进一步收口为 Redis external queue + candidate index + due-index 主路径，execution history 也会显式暴露 `external_queue_state` 与聚合 `external_queues` telemetry；execution claim 后的 target-state refresh 现也已补齐，因此跨实例 worker-only 窗口不再会按陈旧 receipt/DLQ target 重跑 Talenta 任务。Talenta 当前仅剩公开文档未提供的 `leave/time-off` 事件继续挂起，其他厂商仍按当前排期暂停推进。
- PostgreSQL replay 已新增 soak + nightly 自动化链路，当前进入多日数据稳定性观察阶段。
- Wallet 队列链路已补 DLQ + 摘要观测 + 批量治理 + 指标阈值 + 告警订阅策略 + 趋势指标 + 失败重试策略 + 双通道发送（`email: resend/mock` + `whatsapp: mock/meta`），并统一输出 `channel_results` 回执。
- `BLOCKED_EXTERNAL` WhatsApp Meta 企业号真实通道待外部资质完成；当前以 mock/provider contract 回归为主。
- 管理端已接入 Wallet 运营页面（metrics + metrics/trend + cleanup archives + 跨租户风险排行 + 告警订阅策略 + 告警发送记录）；enterprise 完整运营页面仍待接入。
- UI 未做 token 自动 refresh。
- 自动化测试覆盖仍偏低。

### 4.4 后续规划重点

- 真实 Google Wallet API 落地（替换 mock）。
- 企业完整运营页面接入（当前后端接口已具备，管理端 enterprise 运营视图待补齐）。
- Email/WhatsApp 凭证分发、实体卡流程。
- WalletMate II（Apple VAS + Google Smart Tap）实机闭环。

## 5. 建议测试顺序（10~15 分钟）

1. 用 `superadmin` 登录，确认能进 `/dashboard`。
2. 到 `/tenants` 新建租户并切换状态。
3. 到 `/spaces` 为指定租户创建 building/floor/area/door。
4. 到 `/access` 新建 policy/group/temporary-access。
5. 到 `/gateways` 完成 register + bind-door + publish + reboot。
6. 到 `/events` 和 `/alarms` 验证查询和状态更新。

## 6. 2026-04-11 回归记录（网关 + 员工列表）

- 脚本：`docs/testing/curl-regression-gateway-employee.zsh`
- 变量修复：
  - 将冲突变量 `GID` 替换为普通变量 `GW_ID`（避免 zsh 保留变量冲突）。
  - `api_with_auth` 内部参数从 `path` 改为 `endpoint_path`（避免覆盖 zsh 的 `PATH`）。
- 覆盖场景：
  - 网关注册容量：`device_capacity=4` 注册成功并在列表可见。
  - 挂设备容量校验：前 4 个设备成功，第 5 个返回 `400` + `gateway device capacity exceeded`。
  - 门点解绑：`bind-door` 后 `bound_door_ids` 包含门点，`unbind-door` 后为空。
  - 重启后状态：`reboot` 返回 `202 queued`，列表状态保持 `online` 且 `last_seen_at` 更新。
  - 员工列表：`GET /api/v1/enterprise/employees?tenant_id=tenant_demo_jakarta` 返回有效员工项，字段包含 `full_name/email/department/job_title/access_role/building_id/status`，与 `/access` 页面搜索和展示字段一致。

## 7. 2026-04-12 回归记录（Wallet 任务重试）

- 脚本：`docs/testing/curl-wallet-job-retry.zsh`
- 覆盖场景：
  - 创建租户模板后，批量发卡混入空 `target_id`，生成失败任务（`target_id_required`）。
  - 调用 `POST /api/v1/wallet/jobs/{jobID}/retry` 并补充 `target_id` 后，任务状态从 `failed` 变为 `success`。
  - 重试后校验 `retry_count` 增长、`pass_id` 回填，并能在 `GET /api/v1/wallet/passes` 查询到对应卡片。
  - 兼容性约束：`POST /api/v1/wallet/passes/issue-batch` 默认 `execution_mode=inline`，确保历史调用方行为不变。

## 8. 2026-04-12 回归记录（网关序列号 + 协议兼容）

- 脚本：`docs/testing/curl-gateway-serial-protocol.zsh`
- 覆盖场景：
  - 序列号生命周期：
    - 序列号库存导入后可注册核销（`available -> consumed`）。
    - 支持 CSV 导入与导出（批量入库与台账导出）。
    - 冻结状态（`frozen`）注册被阻断，回库为 `available` 后可继续注册。
    - 报废状态（`scrapped`）注册被阻断。
  - 网关序列号格式校验：非法前缀/格式返回 `400 serial_number format is invalid`。
  - 网关序列号去重：大小写不敏感重复注册返回 `409 gateway serial_number already registered`。
  - 下挂设备协议默认值：
    - `legacy_reader + legacy_integration` 未传协议，落库默认 `wiegand_26`。
    - `reader + mistypass_procured` 未传协议，落库默认 `osdp_v2`。
  - 显式协议校验：`ble`、`rs485` 可注册；`rs485` 支持 `rs485_config` 参数回传校验；非法协议（如 `uart`）返回 `400 invalid gateway device protocol`。
  - 约束校验：非 `rs485` 协议携带 `rs485_config` 返回 `400 rs485_config requires protocol=rs485`。
  - RS485 运行态可靠性：`/devices/{deviceID}/rs485/telemetry` 支持上报 `retries/timeouts/collisions`，达到阈值后写入审计 `gateway_rs485_health_alert`。
  - 跨网关设备序列号冲突：返回 `409 device serial_number already registered on another gateway`。

## 9. 2026-04-12 回归记录（序列号库存 CSV + 批量状态）

- 脚本：
  - `docs/testing/curl-gateway-serial-inventory-csv.zsh`
  - `docs/testing/curl-gateway-serial-inventory-batch.zsh`
- 覆盖场景：
  - CSV 入库：`POST /api/v1/gateways/serial-inventory/import-csv` 导入库存台账。
  - CSV 导出：`GET /api/v1/gateways/serial-inventory/export-csv` 导出并校验状态筛选结果。
  - 批量状态流转：`PATCH /api/v1/gateways/serial-inventory/batch-status` 支持冻结/回库/报废批量处理。
  - 行为校验：冻结后注册阻断、回库后恢复可注册、报废后阻断挂载。

## 10. 2026-04-12 回归记录（企业同步 + access 批量写入）

- 脚本：`docs/testing/curl-enterprise-sync-access-batch.zsh`
- 覆盖场景：
  - 企业同步接口：`POST /api/v1/enterprise/employees/sync` 返回 `job` 与 `access_sync` 统计。
  - access 批量写入：仅 enterprise 通过校验的员工会进入 access 批量 upsert（`created/updated/rejected`）。
  - 域名不匹配员工会在 enterprise 层被拒绝，不写入 access。
  - 请求幂等：同租户 + 同 `request_id` 重试返回同一 `job.id`，`access_sync` 统计保持一致，不重复写入。
  - 显式补偿：`POST /api/v1/enterprise/employees/sync/reconcile` 可按 `request_id` 返回同一 `job.id` 与一致计数，支持不携带员工明细重放。
  - 审计查询：`GET /api/v1/enterprise/sync-requests?request_id=...` 可读取 `access_attempt_count`、`last_access_error`、`last_access_attempt_at`。
  - 批量补偿：`POST /api/v1/enterprise/sync-requests/reconcile-pending` 支持 `limit`，仅处理 `access_applied=false` 请求。
  - 失败注入：补偿未知 `request_id` 返回 `404 sync request not found`。
  - 一致性保障：`access` 写入失败时会回滚本次 `enterprise` 同步结果，避免跨模块部分成功。
  - 结果校验：`GET /api/v1/users?tenant_id=...` 中仅存在有效员工邮箱，不存在被拒绝员工邮箱。

## 11. 2026-04-12 回归记录（PostgreSQL change-log + replay + checkpoint）

- 脚本：`docs/testing/curl-pg-persistence-smoke.zsh`
- 覆盖场景：
  - `GET /api/v1/state/change-log` 可读取 `module_enterprise` 的变更事件。
  - `POST /api/v1/state/change-log/replay` 可按 `from_id` 回放并返回 `applied/last_change_id`。
  - `POST /api/v1/state/change-log/replay/checkpoint` 首次可从断点回放并推进 checkpoint，再次调用验证 `applied=0`（幂等续跑）。
  - `GET /api/v1/state/change-log/checkpoints` 可读取 `state_key/last_change_id/updated_at` 断点状态。

## 12. 2026-04-12 回归记录（企业同步 worker 告警）

- 脚本：`docs/testing/curl-enterprise-sync-worker-alert.zsh`
- 覆盖场景：
  - 向 PostgreSQL `module_enterprise` 快照注入 pending `sync_request_record`，模拟“待补偿”请求。
  - 启用后台 worker，并开启测试故障注入（`ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR=true`）。
  - 验证 worker 失败达到阈值后写入审计日志：
    - `action=enterprise_sync_reconcile_worker_alert`
    - `source=enterprise_sync_worker`
    - `target` 包含 `failed`/`threshold` 统计。
  - 验证结构化告警接口：`GET /api/v1/enterprise/sync-worker-alerts?tenant_id=...` 返回 `worker_action/worker_kind/worker_label` 与 `failed/threshold/processed/applied/skipped_*` 字段。
  - 验证告警汇总接口：`GET /api/v1/enterprise/sync-worker-alerts/summary?tenant_id=...` 返回 `tenant_id + worker_action` 维度的 `count/first_seen_at/last_seen_at` 与最近一次告警指标。
  - 验证审计筛选参数可用：`GET /api/v1/audit-logs?tenant_id=...&action=...&source=...&limit=...`。
  - 验证请求补偿审计字段被 worker 更新：`access_attempt_count`、`last_access_error`。
  - 验证 `max_attempts` 生效：达到上限后等待多个轮询周期，`access_attempt_count` 保持不再增长。

## 13. 2026-04-13 回归记录（网关配置拉取/回执）

- 脚本：`docs/testing/curl-gateway-config-pull-apply.zsh`
- 覆盖场景：
  - 云端发布配置：`POST /api/v1/gateways/{gatewayID}/config/publish` 返回 `queued`。
- 网关拉取配置：`POST /api/v1/gateway/config/pull` 返回 `desired_version`、`should_apply`、`bound_door_ids`、`devices`，以及可离线直接使用的 `authz_cache`（含 `version/generated_at/expires_at/ttl_seconds/scope/counts`、过滤后的授权数据集，以及 `policy + status_codes + status`；请求可选上报 `authz_cache_version`）。
- 网关回执配置：`POST /api/v1/gateway/config/applied` 上报已应用版本并回执 `authz_cache_version`，返回 `in_sync=true`、`authz_cache.version_match=true`、`authz_cache.status=AUTHZ_CACHE_FRESH`。
- 二次拉取校验：当 `current_version == desired_version` 时 `should_apply=false`，`applied_version` 与目标版本一致，`authz_cache.version` 保持稳定（数据未变不漂移），且 `policy.rollback_version` 与上次对账成功版本一致。
- authz 状态机校验：覆盖 `AUTHZ_CACHE_MISSING`（首次未上报）、`AUTHZ_CACHE_FRESH`（上报一致）、`AUTHZ_CACHE_DRIFT`（上报漂移版本）、`AUTHZ_CACHE_STALE`（上报 rollback 版本但云端已生成新缓存版本）。

## 14. 2026-04-13 回归记录（网关事件补传幂等）

- 脚本：`docs/testing/curl-gateway-event-idempotency.zsh`
- 覆盖场景：
  - bootstrap 事件上报：`POST /api/v1/gateway/events/access`、`POST /api/v1/gateway/events/device`。
  - 重放去重：重复提交同一 `event_id/request_id` 返回 `deduplicated=true`。
  - `queue_progress.ingested_total` 进度校验：首次上报递增，重复回放（纯 dedup）保持不增长。
  - 列表校验：`GET /api/v1/events/access|device?tenant_id=...` 中对应事件仅保留 1 条记录（无重复落库）。

## 15. 2026-04-13 回归记录（网关事件批量补传）

- 脚本：`docs/testing/curl-gateway-event-batch-replay.zsh`
- 覆盖场景：
  - 批量补传入口：`POST /api/v1/gateway/events/batch`。
  - 首次补传：`access.created=2/device.created=1`，`deduplicated=0`。
  - 重放同批次：`access.created=0/device.created=0`，`deduplicated` 计数按原批次条数返回。
  - 列表校验：`/api/v1/events/access|device` 不出现重复记录。

## 16. 2026-04-13 回归记录（网关 checkpoint + partial）

- 脚本：`docs/testing/curl-gateway-event-checkpoint-partial.zsh`
- 覆盖场景：
  - `POST /api/v1/gateway/events/batch` 部分失败：坏条目仅记 `failed`，并返回 `retryable`；响应含 `retry_subset`（可直接重试子集）。
  - `POST /api/v1/gateway/events/batch` 返回 `queue_hint`（`checkpoint_id/acked_increment/server_ingested_total/status_code/next_action`），并覆盖非可重试分支 `QUEUE_PARTIAL_NON_RETRYABLE`。
  - `retry_subset` 正样本：注入一次性可重试失败后，直接回放 `retry_subset` 请求体验证重试成功。
  - `POST /api/v1/gateway/events/checkpoint` 上报 `queue/checkpoint_id/acked_count`，并校验两类 `409`：`acked_count` 回退（返回最新 checkpoint 快照 + `next_action`）与 `acked_count` 超过服务端队列总量（返回 `server_event_total + server_total_source + next_action`）。
  - `GET /api/v1/gateways/{gatewayID}/events/checkpoint` 可查询最新队列水位记录。
  - `GET /api/v1/gateways/events/checkpoint/summary` 可查询队列摘要（`event_total/acked_total/lag_total`）以及 `time_window_trend`（按最近窗口 checkpoint 审计计算）。

## 17. 2026-04-13 回归记录（网关 retry_subset mixed）

- 脚本：`docs/testing/curl-gateway-event-retry-subset-mixed.zsh`
- 覆盖场景：
  - `POST /api/v1/gateway/events/batch` 混合 access/device 注入可重试失败，返回 `retry_subset`（同时包含 `queue/access_events/device_events`）和 `queue_hint.status_code=QUEUE_RETRY_SUBSET_REQUIRED`。
  - `queue_hint.server_ingested_total` 进度校验：源批次为 `1`，回放成功后推进到 `3`，重复回放（纯 dedup）保持 `3` 不再增长。
  - 直接回放 `retry_subset` 到同一接口后成功落库（`status=accepted`，`failed=0`）。
  - 回放成功后 `queue_hint.status_code=QUEUE_READY_TO_CHECKPOINT`，可直接上报 checkpoint。
  - 再次回放同一 `retry_subset` 返回去重统计（`deduplicated`）。
  - `/api/v1/events/access|device` 列表校验：目标事件仅一条记录。

## 18. 2026-04-13 回归记录（网关 priority 队列 checkpoint 上界）

- 脚本：`docs/testing/curl-gateway-event-priority-checkpoint.zsh`
- 覆盖场景：
  - `POST /api/v1/gateway/events/batch` 使用 `queue=priority` 上报 access/device 事件，校验 `queue_hint.queue=priority`。
  - `queue_hint.server_ingested_total` 进度校验：首次写入为 `2`，重复回放（纯 dedup）保持 `2` 不增长。
  - `POST /api/v1/gateway/events/checkpoint` 在 `priority` 队列上报 `acked_count=2` 成功。
  - `POST /api/v1/gateway/events/checkpoint` 上报超量 `acked_count` 返回 `409`，并校验 `server_total_source=queue_ingest_total`、`server_event_total=2`。

## 19. 2026-04-13 回归记录（queue_ingest_totals 跨重启恢复）

- 脚本：`docs/testing/curl-gateway-event-queue-ingest-restart.zsh`
- 覆盖场景：
  - 首次运行 API 写入 `priority` 队列累计进度：两次 batch 后 `queue_hint.server_ingested_total=3`。
  - 重启 API 后回放同一批次，校验纯 dedup 返回且 `server_ingested_total` 仍为 `3`（累计值连续）。
  - 重启后 checkpoint 回退（`acked_count` 回归）返回 `409`，并携带重启前最新 checkpoint 快照。
  - 重启后 checkpoint 超量上报返回 `409`，校验 `server_total_source=queue_ingest_total` 与 `server_event_total=3`。

## 20. 2026-04-13 回归记录（PostgreSQL replay retry + idempotent）

- 脚本：`docs/testing/curl-pg-replay-retry-idempotent.zsh`
- 覆盖场景：
  - 注入异常 change-log 行后，`POST /api/v1/state/change-log/replay/checkpoint` 返回 `500`。
  - 失败回放不推进 checkpoint：`GET /api/v1/state/change-log/checkpoints` 的 `last_change_id` 与失败前一致。
  - 清理异常行后重试回放成功，并推进 `last_change_id` 到最新变更。
  - 再次回放验证幂等 no-op：`applied=0`。

## 21. 2026-04-13 回归记录（PostgreSQL replay 并发基线）

- 脚本：`docs/testing/curl-pg-replay-concurrency-baseline.zsh`
- 覆盖场景：
  - 批量写入 `module_tenant` 样本后，重置 checkpoint 并执行 catch-up 回放。
  - 校验 catch-up 回放覆盖最新 change-log（`last_change_id >= latest_change_id`）。
  - 并发 no-op 回放稳定性：并发请求全部 `200` 且 `applied=0`。
  - KPI 阈值校验：
    - `catchup_throughput_ops_per_sec >= CATCHUP_MIN_OPS_PER_SEC`（默认 `20`）。
    - `noop_p95_ms <= P95_MS_THRESHOLD`（默认 `3000`，CI 使用 `8000`）。
    - `noop_max_ms <= NOOP_MAX_MS_THRESHOLD`（默认 `8000`，CI 使用 `12000`）。
  - 统计输出：`catchup_applied/catchup_latency/catchup_throughput` 与 no-op `p95/max` 延迟（含阈值字段）。

## 22. 2026-04-13 回归记录（PostgreSQL replay 多 state_key 分层曲线）

- 脚本：`docs/testing/curl-pg-replay-multi-state-curve.zsh`
- 覆盖场景：
  - 默认分层参数：
    - 写入规模：`LEVEL_TENANT_WRITES=10,20,40`、`LEVEL_BUILDINGS_PER_TENANT=1,1,1`
    - 并发与回放：`LEVEL_CONCURRENT_NOOP=4,6,8`、`LEVEL_REPLAY_LIMIT=600,1200,2400`
    - KPI 阈值：`LEVEL_MIN_OPS=15,12,8`、`LEVEL_P95_MS=4000,7000,10000`、`LEVEL_MAX_MS=7000,11000,15000`
  - 同轮压测同时写入多租户 `tenant + building` 数据，驱动 `module_tenant` 与 `module_space` 两个 `state_key` 产生增量变化。
  - 每个 level 先记录基线 `latest_change_id`，写入后回放从基线 checkpoint 开始，仅验证本轮 delta（避免历史累计干扰）。
  - 按 level 对每个 `state_key` 执行：
    - catch-up 回放（校验 `last_change_id >= latest_change_id`）。
    - 并发 no-op 回放（校验全部 `200` 且 `applied=0`）。
  - 阈值校验：
    - `catchup_throughput_ops_per_sec >= LEVEL_MIN_OPS[level]`
    - `noop_p95_ms <= LEVEL_P95_MS[level]`
    - `noop_max_ms <= LEVEL_MAX_MS[level]`
  - 统计输出：
    - 每 level/`state_key` 输出 `delta/catchup_applied/catchup_latency/throughput/noop_p95/noop_max`。
    - 末尾输出曲线汇总表（CSV -> 列表化展示）。

## 23. 2026-04-13 回归记录（PostgreSQL replay soak + nightly）

- 脚本：`docs/testing/curl-pg-replay-multi-state-soak.zsh`
- 配套 workflow：`.github/workflows/api-replay-soak-nightly.yml`
- 覆盖场景：
  - 多轮循环执行 `curl-pg-replay-multi-state-curve.zsh`，每轮导出独立 `round_N.csv + round_N.log`。
  - 汇总留档：统一写入 `metrics.csv`（含 `round/started_at/level/state_key/delta/throughput/p95/max/status`）。
  - 趋势守卫：按 `level|state_key` 对比首轮与末轮吞吐，若跌幅超过 `SOAK_DROP_RATIO_MAX`（默认 `0.5`）即失败。
  - 聚合统计：输出 `avg/min/max throughput` 与 `max noop p95/max`。
- 默认参数：
  - `SOAK_ROUNDS=4`
  - `SOAK_INTERVAL_SECONDS=30`
  - `SOAK_DROP_RATIO_MAX=0.5`
  - `SOAK_WORKDIR=/tmp/mp_pg_replay_soak`

## 23.1 2026-04-14 回归记录（PostgreSQL replay nightly 跨日复核）

- 脚本：`docs/testing/curl-pg-replay-soak-review.zsh`
- 配套 workflow：`.github/workflows/api-replay-soak-nightly.yml`（历史 artifact 聚合 + review 报告）
- 覆盖场景：
  - 自动聚合历史 nightly `metrics.csv` 与当日 `metrics.csv`。
  - 输出 `day-summary.csv`（按 `metric_key + date` 聚合）与 `metric-summary.csv`（按 `level|state_key` 汇总）。
  - 校验 `>= SOAK_REVIEW_MIN_DAYS`（默认 `7`）覆盖度与首日/末日吞吐跌幅（`SOAK_REVIEW_DROP_RATIO_MAX`）。
  - 产出 `report.md` 并写入 GitHub Actions Job Summary。
- 默认参数：
  - `SOAK_REVIEW_MIN_DAYS=7`
  - `SOAK_REVIEW_DROP_RATIO_MAX=0.5`
  - `SOAK_REVIEW_STRICT=false`（数据积累阶段仅告警不阻断；完成积累后可切 `true`）

## 23.2 2026-04-14 回归记录（PostgreSQL replay nightly 签字快照）

- 脚本：`docs/testing/curl-pg-replay-soak-signoff.zsh`
- 配套 workflow：`.github/workflows/api-replay-soak-nightly.yml`（review 后自动生成 signoff 快照）
- 覆盖场景：
  - 读取 `review/summary.json` 与 `day-summary.csv/metric-summary.csv`，自动输出 `review/signoff.md`。
  - 统一决策门：`ready_for_signoff / watch_near_threshold / hold_collect_more_data / hold_investigation_required`。
  - 统一证据清单：`soak/review/signoff` 三阶段日志、rounds 数据、history 快照路径写入签字快照。
  - 自动附加到 GitHub Actions Job Summary，并随 nightly artifact 归档。
- 默认参数：
  - `SOAK_SIGNOFF_FAIL_ON_HOLD=false`（数据积累阶段不阻断 job）
  - `SOAK_SIGNOFF_REPORT_MD=/tmp/mp_pg_replay_soak/review/signoff.md`
  - workflow 内部按 `days_gap_to_min_days` 自动切换门禁：达到最小天数后开启 `fail_on_hold=true`，非 `ready_for_signoff` 将阻断 nightly。

## 24. 2026-04-13 回归记录（Wallet queued job process）

- 脚本：`docs/testing/curl-wallet-job-queue-process.zsh`
- 覆盖场景：
  - `POST /api/v1/wallet/passes/issue-batch` 传 `execution_mode=queued`，返回 `pending` job 列表（空 `target_id` 仍即时标记 `failed/target_id_required`）。
  - `POST /api/v1/wallet/jobs/process` 执行队列 worker，支持 `limit/worker_count/max_retry/base_backoff_ms/max_backoff_ms`。
  - worker 处理后校验：
    - `claimed/succeeded/failed` 统计符合预期。
    - `pending` job 状态收敛为 `success`，并回填 `pass_id`。
    - `GET /api/v1/wallet/jobs/{jobID}` 能读取最终状态与结果。

## 25. 2026-04-13 回归记录（Wallet DLQ requeue + summary）

- 脚本：`docs/testing/curl-wallet-job-dlq-requeue.zsh`
- 覆盖场景：
  - `execution_mode=queued` 入队后，通过将模板切到 `inactive`，触发 `POST /api/v1/wallet/jobs/process` 的不可重试失败分流（`status=dlq`，`error_code=template_inactive`）。
  - `GET /api/v1/wallet/jobs/summary` 返回 `dlq` 数量与错误码分布（`error_code_breakdown`）。
  - `POST /api/v1/wallet/jobs/{jobID}/dlq/requeue` 将 DLQ 任务回补到 `pending`。
  - 恢复模板 `active` 后再次 `jobs/process`，任务最终收敛为 `success` 并回填 `pass_id`。

## 26. 2026-04-14 回归记录（Wallet DLQ 批量治理）

- 脚本：`docs/testing/curl-wallet-job-dlq-governance.zsh`
- 覆盖场景：
  - 构造 old/new 两组 DLQ 任务（`template_inactive`）用于治理验证。
  - `POST /api/v1/wallet/jobs/dlq/requeue` 按 `limit/error_code` 批量回补 DLQ 到 `pending`。
  - 恢复模板 `active` 后，`POST /api/v1/wallet/jobs/process` 将回补任务处理为 `success`。
  - `POST /api/v1/wallet/jobs/dlq/cleanup` 按 `older_than_seconds/error_code/limit` 清理旧 DLQ。
  - `GET /api/v1/wallet/jobs/dlq/cleanup/archives` 返回最近清理归档（`removed/remaining_dlq/actor/processed_jobs`）。
  - 清理后 `GET /api/v1/wallet/jobs/summary` 仍可返回状态分布与错误码汇总。

## 27. 2026-04-14 回归记录（Wallet jobs metrics 告警阈值）

- 脚本：`docs/testing/curl-wallet-job-metrics-alert.zsh`
- 覆盖场景：
  - 启动 API 时注入 `WALLET_DLQ_ALERT_THRESHOLD/WALLET_JOB_METRICS_DEFAULT_WINDOW/WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY`。
  - 通过 `execution_mode=queued` 入队后将模板置为 `inactive`，再执行 `POST /api/v1/wallet/jobs/process` 触发 DLQ。
  - 校验 `jobs/process` 在请求未传 `max_retry` 时，返回值使用配置默认值。
  - 调用 `GET /api/v1/wallet/jobs/metrics`，校验 `dlq_alert_threshold/window.window_seconds` 为配置默认值，且 `alerts` 中命中 `template_inactive` 阈值告警。

## 28. 2026-04-14 回归记录（Wallet 告警订阅策略）

- 脚本：`docs/testing/curl-wallet-job-alert-subscription.zsh`
- 覆盖场景：
  - 启动 API 时注入 `WALLET_DLQ_ALERT_THRESHOLD/WALLET_JOB_METRICS_DEFAULT_WINDOW`，校验 `GET /api/v1/wallet/jobs/alert-subscription` 默认策略回读。
  - 调用 `PUT /api/v1/wallet/jobs/alert-subscription` 更新 `enabled/channels/dlq_alert_threshold/window_seconds/cooldown_seconds/receiver_groups` 并回读确认持久化。
  - 提交非法渠道组合（`enabled=true` 且 `email=false, whatsapp=false`）校验返回 `400`。

## 29. 2026-04-14 回归记录（Wallet jobs metrics 趋势图）

- 脚本：`docs/testing/curl-wallet-job-metrics-trend.zsh`
- 覆盖场景：
  - 通过 `execution_mode=queued` 入队后将模板置为 `inactive`，执行 `POST /api/v1/wallet/jobs/process` 触发 DLQ 样本。
  - 调用 `GET /api/v1/wallet/jobs/metrics/trend`，校验 `bucket_count` 与 `buckets.length` 一致。
  - 校验趋势聚合计数（`updated`/`dlq`）在桶内收敛，且 `alerts` 与阈值口径联动。

## 30. 2026-04-14 回归记录（Wallet 告警发送链路 mock）

- 脚本：`docs/testing/curl-wallet-job-alert-dispatch.zsh`
- 覆盖场景：
  - 构造 DLQ 告警样本后，通过 `PUT /api/v1/wallet/jobs/alert-subscription` 配置订阅阈值、渠道与冷却时间。
  - 首次 `POST /api/v1/wallet/jobs/alerts/dispatch` 命中并发送（`dispatched >= 1`）。
  - 冷却窗口内再次 dispatch 被跳过（`reason=cooldown`）。
  - dispatch 响应 `items[].channel_results` 返回按通道拆分的统一回执。
  - `GET /api/v1/wallet/jobs/alert-notifications` 返回 sent + cooldown skipped 记录。

## 31. 2026-04-14 回归记录（Wallet 告警发送失败重试）

- 脚本：`docs/testing/curl-wallet-job-alert-dispatch-retry.zsh`
- 覆盖场景：
  - 启动 API 时注入 `WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT=1`，构造可观测的瞬时失败窗口。
  - 首次 `POST /api/v1/wallet/jobs/alerts/dispatch` 命中 provider 瞬时失败（`status=failed`，`reason=provider_transient_error`，`retryable=true`）。
  - 失败记录包含 `channel_results`，用于区分通道级失败原因。
  - 调用 `POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry` 对失败记录手动重试，校验发送成功（`status=sent`）。
  - 对同一失败记录再次重试返回幂等跳过（`status=skipped`，`reason=idempotent_already_sent`）。

## 32. 2026-04-14 回归记录（Wallet Resend Provider）

- 脚本：`docs/testing/curl-wallet-job-alert-dispatch-resend.zsh`
- 覆盖场景：
  - 启动本地 Resend mock 服务，并注入 `WALLET_ALERT_EMAIL_PROVIDER=resend` 与 provider 配置。
  - `POST /api/v1/wallet/jobs/alerts/dispatch` 命中真实 provider 路径（`items[].provider=resend`，`failed=0`）。
  - 校验 `items[].channel_results` 包含 `channel=email,status=sent`。
  - 校验 provider 请求包含 `Authorization: Bearer <api_key>` 与 `from/to/subject` 字段。
  - 校验 `receiver_groups=["security"]` 能映射到配置的邮件接收地址。

## 33. 2026-04-14 回归记录（Wallet WhatsApp + Unified Receipt）

- 脚本：`docs/testing/curl-wallet-job-alert-dispatch-whatsapp.zsh`
- 覆盖场景：
  - 启动 API 时注入 `WALLET_ALERT_WHATSAPP_PROVIDER=mock` 与 `WALLET_ALERT_WHATSAPP_RECEIVER_MAP`。
  - 配置 whatsapp-only 订阅后执行 `POST /api/v1/wallet/jobs/alerts/dispatch`，校验 `dispatched >= 1` 且 `failed=0`。
  - 校验响应 `items[].channel_results[]` 包含 `channel=whatsapp` 且 `status=sent`，验证统一回执口径生效。

## 34. 2026-04-14 回归记录（Wallet WhatsApp Meta Provider，Mock 验证）

- 脚本：`docs/testing/curl-wallet-job-alert-dispatch-whatsapp-meta.zsh`
- 覆盖场景：
  - 启动本地 WhatsApp meta mock 服务，并注入 `WALLET_ALERT_WHATSAPP_PROVIDER=meta` 与 provider 配置。
  - `POST /api/v1/wallet/jobs/alerts/dispatch` 命中 meta provider 路径（`items[].channel_results[].provider=meta`）。
  - 校验 provider 请求包含 `Authorization: Bearer <api_key>`，并命中 `/v22.0/{phone_number_id}/messages` 路径。
- 说明：该脚本用于本地 contract 回归，不等价于 Meta 企业号真实联调验收；真实通道待外部资质申请完成后执行。

## 35. 2026-04-14 回归记录（Gateway Edge 本地队列执行器仿真）

- 脚本：`docs/testing/curl-gateway-edge-queue-executor-sim.zsh`
- 覆盖场景：
  - 启动 API 时注入 `GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_*`，构造 `retry_subset` 事件。
  - 按“本地执行器”决策链执行：`events/batch` -> 读取 `queue_hint.next_action` -> 重放 `retry_subset` -> 上报 `events/checkpoint`。
  - 覆盖二次补传去重（deduplicated 不增长）与 `server_ingested_total` 进度一致性。
  - 覆盖 checkpoint 回退冲突（`acked_count` regression -> HTTP `409` + `next_action=retry_with_non_regressing_acked_count`）。
  - 校验最终 checkpoint 读模型与事件列表无重复。

## 36. 2026-04-14 回归记录（Edge MVP 一键执行包装）

- 脚本：`docs/testing/run-edge-mvp-validation.zsh`
- 覆盖场景：
  - 串行执行关键链路脚本：`serial-protocol`、`idempotency`、`retry-subset-mixed`、`checkpoint-partial`、`edge-queue-executor-sim`。
  - 每条脚本使用独立 `API_PORT/API_BASE_URL` 运行，避免进程与注入参数互相污染。
  - 自动生成报告：`docs/testing/artifacts/edge-mvp-validation-*.md`，输出 `status/elapsed_sec/log_file` 索引。
