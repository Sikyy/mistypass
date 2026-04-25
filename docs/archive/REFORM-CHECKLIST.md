# MistyPass 整改清单

> 生成日期：2026-04-17
> 依据文档：`CODE-REVIEW.md`、`TECH-STACK-RECOMMENDATIONS.md`
> 状态说明：✅ 已完成 | 🔄 进行中 | ⬜ 未完成

当前能力状态：

- `SKELETON_ONLY`：历史整改追踪清单，保留为归档审计材料；当前实际完成度以主干代码、测试与最新 roadmap/doc 为准。

---

## 一、安全整改（P0 — 立即修复）

### 1.1 认证与凭证安全

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 1.1.1 | 演示凭证隔离 | 后端通过 `ENABLE_DEMO_USERS` 环境变量控制演示用户注入，默认关闭；前端登录页仅开发环境展示测试账号 | `auth/service.go`、`login-page.tsx` | ✅ 已完成 |
| 1.1.2 | 密码 bcrypt 哈希存储 | 替换明文密码存储与比较，使用 `golang.org/x/crypto/bcrypt` | `auth/service.go` | ✅ 已完成 |
| 1.1.3 | JWT Secret 默认值治理 | 移除硬编码默认密钥，生产环境未配置 `JWT_SECRET` 时拒绝启动 | `config/config.go` | ✅ 已完成 |
| 1.1.4 | 网关设备 Token 动态化 | 移除 `router.go` 中硬编码的演示网关 token，改为数据库存储 + 注册流程动态生成 | `internal/http/router.go`、`internal/http/routes_gateway_bootstrap.go` | ✅ 已完成 |

### 1.2 数据访问安全

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 1.2.1 | SQL 注入防护 — 表名白名单 | `deleteProjectionRowsNotInIDs` 增加表名白名单校验 | `postgres_store.go` | ✅ 已完成 |

### 1.3 Token 与会话安全

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 1.3.1 | Token 存储迁移 | 将 JWT 从 `localStorage` 迁移到 `httpOnly cookie` 或 `sessionStorage + 内存` 组合方案，防止 XSS 窃取 | `web-admin/src/lib/auth.ts` | ✅ 已完成 |
| 1.3.2 | Refresh Token 自动续期 | 实现 401 拦截 → refresh token 自动刷新 → 失败自动登出 | `auth.ts`、`api.ts`、`AuthProvider` | ✅ 已完成 |
| 1.3.3 | 401 响应自动登出 | API client 收到 401 时自动清除 token 并跳转登录页 | `api.ts`、`AuthProvider` | ✅ 已完成 |
| 1.3.4 | CSRF 防护预留 | 若未来迁移到 cookie 方案，需同步添加 CSRF token 机制（前端非 GET 请求已自动携带 `X-CSRF-Token`，后端 CORS 已放行该请求头） | `web-admin/src/lib/auth.ts`、`web-admin/src/lib/api.ts`、`api/internal/http/router.go` | ✅ 已完成 |

### 1.4 接口防护

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 1.4.1 | 登录接口速率限制 | 基于 IP 的速率限制，`/api/v1/auth/login` 与 `/api/v1/app/auth/login` 每 IP 每分钟限速 | `http/` 中间件 | ✅ 已完成 |
| 1.4.2 | 全局 API 速率限制 | 对所有 API 端点添加通用速率限制中间件 | `internal/http/router.go` | ✅ 已完成 |

---

## 二、前端架构整改（P1 — 短期 1-2 周）

### 2.1 数据请求层改造

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 2.1.1 | 引入 TanStack Query | 接入 `@tanstack/react-query`，配置 `QueryClientProvider` | `main.tsx` | ✅ 已完成 |
| 2.1.2 | dashboard-page 迁移 | 替换手动 `useEffect` + `useState` 为 `useQuery` | `dashboard-page.tsx` | ✅ 已完成 |
| 2.1.3 | tenants-page 迁移 | 同上 | `tenants-page.tsx` | ✅ 已完成 |
| 2.1.4 | tenant-detail-page 迁移 | 同上 | `tenant-detail-page.tsx` | ✅ 已完成 |
| 2.1.5 | events-page 迁移 | 同上 | `events-page.tsx` | ✅ 已完成 |
| 2.1.6 | alarms-page 迁移 | 同上 | `alarms-page.tsx` | ✅ 已完成 |
| 2.1.7 | spaces-page 迁移 | 同上 | `spaces-page.tsx` | ✅ 已完成 |
| 2.1.8 | audit-page 迁移 | 同上 | `audit-page.tsx` | ✅ 已完成 |
| 2.1.9 | access-page 迁移 | 同上 | `access-page.tsx` | ✅ 已完成 |
| 2.1.10 | gateways-page 迁移 | 同上 | `gateways-page.tsx` | ✅ 已完成 |
| 2.1.11 | enterprise-page 迁移 | 同上 | `enterprise-page.tsx` | ✅ 已完成 |
| 2.1.12 | wallet-page 迁移 | bootstrap 阶段已完成 | `wallet-page.tsx` | ✅ 已完成 |

### 2.2 全局状态管理

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 2.2.1 | AuthProvider + useAuth | 封装认证 Context，消除 token/viewer prop drilling | `context/AuthProvider.tsx` | ✅ 已完成 |
| 2.2.2 | 引入 Zustand | 管理 UI 全局状态（sidebar 折叠、通知等），替代 prop drilling | `web-admin/src/stores/ui-store.ts`、`web-admin/src/App.tsx` | ✅ 已完成 |
| 2.2.3 | API Client 自动附加 Token | API client 从 context 自动获取 token，无需每次调用手动传入（AuthProvider 注入 token provider，request/requestText 自动附加） | `web-admin/src/lib/api.ts`、`web-admin/src/context/auth-context.tsx` | ✅ 已完成 |

### 2.3 错误处理与路由

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 2.3.1 | 全局 Error Boundary | `main.tsx` 添加 `AppErrorBoundary`，运行时异常降级展示 | `main.tsx` | ✅ 已完成 |
| 2.3.2 | ProtectedRoute 组件化 | 提取 `<ProtectedRoute>` 组件，统一权限检查和重定向逻辑，消除重复代码 | `web-admin/src/App.tsx`、`web-admin/src/components/protected-route.tsx` | ✅ 已完成 |
| 2.3.3 | 404 页面 | 添加 Not Found 页面作为路由兜底，替代静默重定向到 `/dashboard` | `web-admin/src/App.tsx`、`web-admin/src/pages/not-found-page.tsx` | ✅ 已完成 |

---

## 三、后端架构整改（P1 — 短期 1-2 周）

### 3.1 代码结构拆分

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 3.1.1 | router.go 按模块拆分 | 拆分为 `routes_auth.go`、`routes_gateway_*.go`、`routes_tenant_space.go`、`routes_access_audit.go`、`routes_wallet.go`、`routes_enterprise_*.go`、`routes_state_change.go` | `internal/http/` | ✅ 已完成 |

### 3.2 关键数据持久化

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 3.2.1 | 用户账号持久化 | Auth 服务支持持久化接口并接入 PostgreSQL（`mistypass_auth_users`），登录/可信登录按持久层读写用户与密码哈希 | `api/internal/modules/auth/service.go`、`api/internal/state/postgres_store.go`、`api/internal/http/router.go` | ✅ 已完成 |
| 3.2.2 | Refresh Token 持久化 | Refresh session 持久化到 PostgreSQL（`mistypass_auth_refresh_sessions`），刷新/撤销/批量撤销统一走持久层 | `api/internal/modules/auth/service.go`、`api/internal/state/postgres_store.go` | ✅ 已完成 |
| 3.2.3 | Revoked Token 持久化 | Access token 撤销记录持久化到 PostgreSQL（`mistypass_auth_revoked_access_tokens`），鉴权时支持持久层校验 | `api/internal/modules/auth/service.go`、`api/internal/state/postgres_store.go` | ✅ 已完成 |
| 3.2.4 | 网关设备 Token 持久化 | 将网关 token 迁移到数据库，通过注册流程动态生成（新增 `mistypass_gateway_device_tokens` 持久化表，按哈希校验） | `api/internal/http/router.go`、`api/internal/http/routes_gateway_bootstrap.go`、`api/internal/state/postgres_store.go` | ✅ 已完成 |

### 3.3 代码健壮性

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 3.3.1 | 事务回滚错误日志 | 将 `_ = tx.Rollback()` 改为记录回滚错误到日志，防止连接池泄漏不可观测 | `api/internal/state/postgres_store.go` | ✅ 已完成 |

---

## 四、中间件与基础设施引入（P2 — 中期 1-2 个月）

### 4.1 Redis 接入

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.1.1 | Redis / Dragonfly 部署 | 已完成 Redis 连接配置与客户端接入，支持通过 `REDIS_*` 环境变量启用 Redis/Dragonfly 作为临时态后端 | `api/internal/config/config.go`、`api/internal/redistore/store.go`、`api/internal/http/router.go`、`README.md` | ✅ 已完成 |
| 4.1.2 | Session 存储迁移 | 已完成 refresh session 迁移：`auth` 服务支持可插拔 volatile store，启用 Redis 时 session 写入/读取/删除走 Redis（保留内存兜底） | `api/internal/modules/auth/service.go`、`api/internal/redistore/store.go` | ✅ 已完成 |
| 4.1.3 | Token 黑名单迁移 | 已完成 revoked access token 迁移：启用 Redis 时黑名单记录与校验走 Redis TTL key（保留内存兜底） | `api/internal/modules/auth/service.go`、`api/internal/redistore/store.go` | ✅ 已完成 |
| 4.1.4 | 速率限制后端存储 | 已完成登录/API 限流计数迁移：优先使用 Redis 计数器（按时间窗口），异常自动回退内存桶 | `api/internal/http/router.go`、`api/internal/redistore/store.go` | ✅ 已完成 |

### 4.2 可观测性建设

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.2.1 | 结构化日志 — slog | 后端入口统一初始化 `slog.JSONHandler`，并将 `cmd/api`、`router`、`postgres_store` 中遗留 `log.*` 调用迁移为结构化日志字段输出 | `api/cmd/api/main.go`、`api/internal/http/router.go`、`api/internal/state/postgres_store.go` | ✅ 已完成 |
| 4.2.2 | 请求日志中间件 | 添加 HTTP 请求/响应日志中间件，记录 method、path、status、duration、request_id、client_ip、user_agent | `api/internal/http/middleware_request_log.go`、`api/internal/http/router.go`、`api/internal/http/middleware_request_log_test.go` | ✅ 已完成 |
| 4.2.3 | Prometheus Metrics 端点 | 暴露 `/metrics` 端点并接入 Prometheus client，请求总量与时延指标通过请求中间件统一采集（按 method/route/status 维度） | `api/internal/http/router.go`、`api/internal/http/middleware_metrics.go`、`api/internal/http/middleware_request_log.go`、`api/internal/http/router_metrics_test.go` | ✅ 已完成 |
| 4.2.4 | OpenTelemetry 接入 | 新增 OTLP Trace 导出配置与初始化，接入 HTTP Trace 中间件（W3C TraceContext/Baggage 传播），支持对接 Jaeger/Grafana Tempo（OTLP）后端 | `api/internal/config/config.go`、`api/cmd/api/otel.go`、`api/internal/http/middleware_trace.go`、`api/internal/http/router.go` | ✅ 已完成 |

### 4.3 前端巨型组件拆分

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.3.1 | access-page 拆分 | 已完成：访问域已拆分为独立 section 组件（Directory / Policies / Grants），由 `AccessSectionsTabs` 统一编排 | `web-admin/src/pages/access-page.tsx`、`web-admin/src/components/access/access-sections-tabs.tsx`、`web-admin/src/components/access/access-directory-section.tsx`、`web-admin/src/components/access/access-policies-section.tsx`、`web-admin/src/components/access/access-grants-section.tsx` | ✅ 已完成 |
| 4.3.2 | gateways-page 拆分 | 已完成：页面视图块已拆分为独立组件（`CheckpointMonitor`、`GatewaySearchCard`、`GatewayListCard`、`GatewayCommandCenterCard`、`GatewaySerialInventoryIngestCard`、`GatewaySerialInventoryLedgerCard`） | `web-admin/src/pages/gateways-page.tsx`、`web-admin/src/components/gateways/checkpoint-monitor.tsx`、`web-admin/src/components/gateways/gateway-search-card.tsx`、`web-admin/src/components/gateways/gateway-list-card.tsx`、`web-admin/src/components/gateways/gateway-command-center-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ingest-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ledger-card.tsx` | ✅ 已完成 |
| 4.3.3 | wallet-page 拆分 | 已完成：发放模板、发放队列、告警配置、通知记录、DLQ 归档、风险总览与趋势面板已拆分为独立组件（`WalletTemplateManagerCard`、`WalletIssueJobQueueCard`、`WalletAlertConfigCard`、`WalletAlertSubscriptionCard`、`WalletAlertNotificationRecordsCard`、`WalletDlqCleanupArchivesCard`、`WalletRiskOverviewPanels`、`WalletAlertTrendPanels`） | `web-admin/src/pages/wallet-page.tsx`、`web-admin/src/components/wallet/wallet-template-manager-card.tsx`、`web-admin/src/components/wallet/wallet-issue-job-queue-card.tsx`、`web-admin/src/components/wallet/wallet-alert-config-card.tsx`、`web-admin/src/components/wallet/wallet-alert-subscription-card.tsx`、`web-admin/src/components/wallet/wallet-alert-notification-records-card.tsx`、`web-admin/src/components/wallet/wallet-dlq-cleanup-archives-card.tsx`、`web-admin/src/components/wallet/wallet-risk-overview-panels.tsx`、`web-admin/src/components/wallet/wallet-alert-trend-panels.tsx` | ✅ 已完成 |
| 4.3.4 | enterprise-page 拆分 | 已完成：各 tab 已拆分为独立 workspace 组件（Employees / Sync / IDP / Alerts） | `web-admin/src/pages/enterprise-page.tsx`、`web-admin/src/components/enterprise/enterprise-employees-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-sync-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-idp-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-alerts-workspace.tsx` | ✅ 已完成 |

### 4.4 前端功能补齐

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.4.1 | 列表页分页 | 已完成：`events` / `alarms` 已接入 API 分页参数（`page`/`limit`）；其余主要列表页（`wallet` Passes、`tenants`、`audit`、`spaces` 门点台账、`tenant-detail` 楼宇台账、`gateways` 网关与序列号库存台账、`access` 三类台账、`alarms` 通知日志）均已统一接入分页组件（每页条数、页码切换、筛选后重置页码） | `web-admin/src/pages/events-page.tsx`、`web-admin/src/pages/alarms-page.tsx`、`web-admin/src/pages/wallet-page.tsx`、`web-admin/src/pages/tenants-page.tsx`、`web-admin/src/pages/audit-page.tsx`、`web-admin/src/pages/spaces-page.tsx`、`web-admin/src/pages/tenant-detail-page.tsx`、`web-admin/src/components/gateways/gateway-list-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ledger-card.tsx`、`web-admin/src/pages/gateways-page.tsx`、`web-admin/src/components/access/access-group-ledger-table.tsx`、`web-admin/src/components/access/access-policy-ledger-table.tsx`、`web-admin/src/components/access/access-grant-ledger-table.tsx`、`web-admin/src/components/ui/list-pagination.tsx`、`web-admin/src/lib/api.ts` | ✅ 已完成 |
| 4.4.2 | 表单验证 — React Hook Form + Zod | 已完成：已引入 `react-hook-form` + `zod` + `@hookform/resolvers`，并完成 `tenants-page` 创建租户、`gateways-page` 注册网关/序列号入库（单条/CSV）/库存批量状态流转、`wallet-page` 发放模板新建 + 单个发放 + 批量发放 + 外部投递 + 实体卡任务、`spaces-page` 创建楼宇/楼层/区域/门点、`access-page` 用户组/权限策略/临时授权表单、`enterprise-page` 同步提交表单、`login-page` 登录表单校验改造（必填项、长度、枚举、范围依赖、到期时间格式、通道必选、批量对象最小数量与 JSON 数组合法性校验）；前端现存 `form` 提交流程已统一迁移到 RHF+Zod | `web-admin/package.json`、`web-admin/package-lock.json`、`web-admin/src/pages/tenants-page.tsx`、`web-admin/src/pages/gateways-page.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ingest-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ledger-card.tsx`、`web-admin/src/pages/wallet-page.tsx`、`web-admin/src/components/wallet/wallet-template-manager-card.tsx`、`web-admin/src/components/wallet/wallet-issue-job-queue-card.tsx`、`web-admin/src/pages/spaces-page.tsx`、`web-admin/src/pages/access-page.tsx`、`web-admin/src/components/access/access-group-form.tsx`、`web-admin/src/components/access/access-policy-form.tsx`、`web-admin/src/components/access/access-grant-form.tsx`、`web-admin/src/components/enterprise/enterprise-sync-workspace.tsx`、`web-admin/src/pages/enterprise-page.tsx`、`web-admin/src/pages/login-page.tsx` | ✅ 已完成 |
| 4.4.3 | 表格组件 — TanStack Table | 已完成：已引入 `@tanstack/react-table`，并完成主要列表页统一改造（`access` 三类台账、`gateways` 网关台账与序列号库存台账、`tenants` 租户列表、`tenant-detail` 楼宇台账、`spaces` 门点台账、`wallet` 凭证台账、`events` 事件表、`alarms` 告警队列与通知日志、`audit` 审计日志），统一接入列排序、筛选、分页与列显隐控制 | `web-admin/package.json`、`web-admin/package-lock.json`、`web-admin/src/components/access/access-group-ledger-table.tsx`、`web-admin/src/components/access/access-policy-ledger-table.tsx`、`web-admin/src/components/access/access-grant-ledger-table.tsx`、`web-admin/src/components/gateways/gateway-list-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ledger-card.tsx`、`web-admin/src/pages/tenants-page.tsx`、`web-admin/src/pages/tenant-detail-page.tsx`、`web-admin/src/pages/spaces-page.tsx`、`web-admin/src/pages/wallet-page.tsx`、`web-admin/src/pages/events-page.tsx`、`web-admin/src/pages/alarms-page.tsx`、`web-admin/src/pages/audit-page.tsx` | ✅ 已完成 |
| 4.4.4 | 类型安全修复 | 已完成：`api.ts` 中宽泛联合类型已收敛（移除 `\| string`），并修复 `api.ts` / `enterprise-page` 的 TS1016 参数签名问题，前端构建已恢复通过 | `web-admin/src/lib/api.ts`、`web-admin/src/pages/enterprise-page.tsx` | ✅ 已完成 |
| 4.4.5 | 内存泄漏修复 | `gateways-page.tsx` 中 `setTimeout` 添加 `useEffect` cleanup，防止组件卸载后状态更新 | `web-admin/src/pages/gateways-page.tsx` | ✅ 已完成 |

### 4.5 后端功能补齐

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.5.1 | Webhook 签名验证 | 支持在 webhook 配置中设置 `signing_secret`，派发时附加 `X-MistyPass-Signature-Timestamp` 与 `X-MistyPass-Signature`（HMAC-SHA256）请求头供接收方验签 | `api/internal/modules/audit/webhook.go`、`api/internal/http/routes_access_audit.go`、`api/internal/modules/audit/webhook_test.go` | ✅ 已完成 |
| 4.5.2 | Webhook 重试机制 | 实现指数退避重试（最多 3 次），对网络异常/超时/429/5xx 自动重试，4xx（除 408/429）快速失败，并记录最终 attempt_count | `api/internal/modules/audit/webhook.go`、`api/internal/modules/audit/webhook_test.go` | ✅ 已完成 |
| 4.5.3 | Projection 代码去重 | 已提取通用 projection helper（泛型 upsert + JSON 序列化 + 批量清理），减少 `postgres_store.go` 中重复逻辑并保持 SQL 行为不变 | `postgres_store.go` | ✅ 已完成 |
| 4.5.4 | sqlc 迁移 | 已完成：分阶段完成 sqlc 迁移（Auth/Gateway Token、state/change_log/checkpoint 主链路、以及 tenant/space/access/gateway/enterprise/event/alarm/audit/wallet projection 查询），`postgres_store.go` 读写主路径已切换为 sqlc 生成代码 | `postgres_store.go`、`api/internal/state/sqlc.yaml`、`api/internal/state/sqlc/`、`api/internal/state/sqlcgen/` | ✅ 已完成 |

---

## 五、长期优化（P3 — 持续迭代）

### 5.1 认证体系升级

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.1.1 | 外部认证服务接入 | ✅ 已完成：新增外部认证接入链路（`POST /api/v1/auth/external/login`），支持通过 `EXTERNAL_AUTH_*` 配置对接 `generic_oidc/casdoor/ory_kratos` userinfo/whoami 端点校验 access token，解析外部身份并桥接到本地 trusted session（签发 MistyPass JWT） | `api/internal/http/routes_auth_mfa_external.go`、`api/internal/http/router.go`、`api/internal/config/config.go`、`api/internal/config/config_test.go`、`api/internal/http/routes_auth_external_test.go`、`README.md` | ✅ 已完成 |
| 5.1.2 | 管理员 MFA | ✅ 已完成：新增管理员 TOTP MFA 能力（状态查询/密钥下发/启用/禁用），并在 `AUTH_ADMIN_MFA_REQUIRED=true` 时强制 `super_admin/tenant_admin` 登录提交 `mfa_code`；补齐 auth service 与 HTTP 路由单测 | `api/internal/modules/auth/service.go`、`api/internal/modules/auth/service_mfa_test.go`、`api/internal/http/routes_auth.go`、`api/internal/http/routes_auth_mfa_external.go`、`api/internal/http/router.go`、`README.md` | ✅ 已完成 |

### 5.2 设备通信层

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.2.1 | EMQX MQTT Broker 接入 | ✅ 已完成：新增 MQTT 配置开关（`MQTT_ENABLED`/`MQTT_BROKER_URL`/`MQTT_TOPIC_PREFIX`）与网关 MQTT bootstrap 接口（`GET /api/v1/gateways/{gatewayID}/mqtt/bootstrap`），按租户与网关生成隔离 topic 命名空间（commands/events/checkpoint/status）；并补齐配置校验、接口单测与 README 操作说明 | `api/internal/config/config.go`、`api/internal/config/config_test.go`、`api/internal/http/router.go`、`api/internal/http/routes_gateway_management.go`、`api/internal/http/routes_gateway_mqtt_test.go`、`README.md` | ✅ 已完成 |
| 5.2.2 | NATS 内部消息总线 | ✅ 已完成：新增 NATS 总线模块与配置开关（`NATS_ENABLED`/`NATS_SERVER_URL`/`NATS_SUBJECT_PREFIX`），接入内部事件发布器并在审计链路落地事件扇出（`audit.log.appended`、`audit.webhook.dispatched`、`audit.webhook.dispatch.failed`）；补齐配置校验、总线单测与 README / Compose 说明 | `api/internal/bus/publisher.go`、`api/internal/bus/publisher_test.go`、`api/internal/config/config.go`、`api/internal/config/config_test.go`、`api/internal/http/router.go`、`api/internal/http/routes_access_audit.go`、`docker-compose.yml`、`README.md` | ✅ 已完成 |
| 5.2.3 | 前端实时推送 — SSE | ✅ 已完成：新增事件与告警实时流端点（`/api/v1/events/stream`、`/api/v1/alarms/stream`），按租户/楼宇权限过滤并在数据变更时推送 `update` 事件（含心跳）；前端 `events-page` / `alarms-page` 接入 SSE 消费器并在流更新时触发 React Query 失效刷新，替代手动刷新 | `api/internal/http/router.go`、`api/internal/http/routes_access_audit.go`、`web-admin/src/lib/api.ts`、`web-admin/src/pages/events-page.tsx`、`web-admin/src/pages/alarms-page.tsx` | ✅ 已完成 |

### 5.3 国际化

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.3.1 | react-i18next 接入 | ✅ 已完成：已完成 `react-i18next` + `i18next` + 浏览器语言检测接入，完成 `web-admin` 页面与组件全量迁移（登录/404/App 壳层/dashboard/events/alarms/audit/tenants/tenant-detail/spaces/gateways/access/wallet/enterprise 全域），并完成 `accessPage/walletPage` 新增 key 三语同步与 CI 中文硬编码扫描接入（`web-admin-cjk-guard.yml` + `scripts/check-hardcoded-cjk.mjs` + `web-admin/cjk-whitelist.txt`）；本轮完成剩余英文占位批量翻译与长尾修正，当前全量 key 2,136 中：`zh-CN` 已本地化 2,126、`id-ID` 已本地化 2,126，剩余 10 个与 `en-US` 相同项为符号占位/品牌词（`-`、`********`、`WhatsApp`、`中文`）按设计保留 | `web-admin/package.json`、`web-admin/src/lib/i18n.ts`、`web-admin/src/locales/`、`web-admin/src/main.tsx`、`web-admin/src/pages/login-page.tsx`、`web-admin/src/pages/not-found-page.tsx`、`web-admin/src/App.tsx`、`web-admin/src/pages/dashboard-page.tsx`、`web-admin/src/pages/events-page.tsx`、`web-admin/src/pages/alarms-page.tsx`、`web-admin/src/pages/audit-page.tsx`、`web-admin/src/pages/tenants-page.tsx`、`web-admin/src/pages/tenant-detail-page.tsx`、`web-admin/src/pages/spaces-page.tsx`、`web-admin/src/pages/gateways-page.tsx`、`web-admin/src/components/gateways/gateway-search-card.tsx`、`web-admin/src/components/gateways/gateway-list-card.tsx`、`web-admin/src/components/gateways/checkpoint-monitor.tsx`、`web-admin/src/components/gateways/gateway-command-center-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ingest-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ledger-card.tsx`、`web-admin/src/pages/enterprise-page.tsx`、`web-admin/src/components/enterprise/enterprise-employees-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-idp-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-sync-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-alerts-workspace.tsx`、`web-admin/src/pages/access-page.tsx`、`web-admin/src/components/access/`、`web-admin/src/pages/wallet-page.tsx`、`web-admin/src/components/wallet/` | ✅ 已完成 |
| 5.3.2 | 多语言支持 | ✅ 已完成：已完成 `zh-CN/en-US/id-ID` 三语词条落地与全站多语言渲染收口，覆盖登录页、404、全局导航壳层及业务域页面（dashboard/events/alarms/audit/tenants/tenant-detail/spaces/gateways/access/wallet/enterprise）与对应拆分组件；完成 `walletPage.components.*`、`accessPage.components.*`、`appErrorBoundary/listPagination/viewer.role` 正式翻译与术语统一，enterprise 域文案精修完成；本轮完成剩余英文等值文案清理，最终仅保留符号占位与品牌词等 10 个设计性等值项 | `web-admin/src/locales/`、`web-admin/src/pages/login-page.tsx`、`web-admin/src/pages/not-found-page.tsx`、`web-admin/src/App.tsx`、`web-admin/src/pages/dashboard-page.tsx`、`web-admin/src/pages/events-page.tsx`、`web-admin/src/pages/alarms-page.tsx`、`web-admin/src/pages/audit-page.tsx`、`web-admin/src/pages/tenants-page.tsx`、`web-admin/src/pages/tenant-detail-page.tsx`、`web-admin/src/pages/spaces-page.tsx`、`web-admin/src/pages/gateways-page.tsx`、`web-admin/src/components/gateways/gateway-search-card.tsx`、`web-admin/src/components/gateways/gateway-list-card.tsx`、`web-admin/src/components/gateways/checkpoint-monitor.tsx`、`web-admin/src/components/gateways/gateway-command-center-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ingest-card.tsx`、`web-admin/src/components/gateways/gateway-serial-inventory-ledger-card.tsx`、`web-admin/src/pages/enterprise-page.tsx`、`web-admin/src/components/enterprise/enterprise-employees-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-idp-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-sync-workspace.tsx`、`web-admin/src/components/enterprise/enterprise-alerts-workspace.tsx`、`web-admin/src/pages/access-page.tsx`、`web-admin/src/components/access/`、`web-admin/src/pages/wallet-page.tsx`、`web-admin/src/components/wallet/` | ✅ 已完成 |

### 5.4 后端架构演进

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.4.1 | 模块分层重构 | ✅ 已完成：完成 `tenant` 模块分层重构，按 `domain/application/infrastructure/delivery` 拆分原扁平 `service.go`；对外 `tenant.Service` API 保持兼容（`List/Create/UpdateStatus`），并通过 facade 适配承接应用层，state store 持久化下沉到基础设施层 | `api/internal/modules/tenant/service.go`、`api/internal/modules/tenant/domain/tenant.go`、`api/internal/modules/tenant/application/service.go`、`api/internal/modules/tenant/infrastructure/state_repository.go`、`api/internal/modules/tenant/delivery/facade.go` | ✅ 已完成 |
| 5.4.2 | Encore 框架评估 | ✅ 已完成：补齐最终二选一决策报告，结论冻结为“保持 `Go + Chi` 作为唯一生产主干，暂停 `service-by-service` 的 Encore 迁移计划，仅保留 `Encore.go` 技术选项用于隔离 PoC”；并给出重启迁移评审触发条件 | `docs/architecture/encore-migration-playbook.md`、`docs/architecture/encore-poc-evaluation-baseline.md`、`docs/architecture/encore-decision-report-2026-04-19.md` | ✅ 已完成 |
| 5.4.3 | JWKS 缓存 | 已完成：`VerifyOIDCIDToken` 的 JWKS 拉取新增 1 小时内存缓存（按 `jwks_url` 维度命中、过期自动回源）；补充缓存命中与过期回源单测 | `api/internal/modules/enterprise/oidc.go`、`api/internal/modules/enterprise/oidc_test.go` | ✅ 已完成 |

### 5.5 前端架构演进

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.5.1 | 前端目录按领域重组 | ✅ 已完成：`web-admin/src/pages` 页面源文件已按领域迁移到 `web-admin/src/features/*/pages`（`access/alarms/audit/auth/dashboard/enterprise/events/gateways/spaces/tenants/wallet/app`），并保留原 `pages/*` 兼容 re-export 入口，确保路由与现有导入零回归 | `web-admin/src/features/`、`web-admin/src/pages/` | ✅ 已完成 |

### 5.6 基础设施与 DevOps

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.6.1 | Docker Compose 开发环境 | 已完成：新增根目录 `docker-compose.yml`，编排 `api` + `postgres` + `redis` + `emqx` 四服务，并补充 README 本地启动/停止说明 | `docker-compose.yml`、`README.md`、`api/Dockerfile` | ✅ 已完成 |
| 5.6.2 | PgBouncer 连接池 | 已完成：在 `docker-compose.yml` 引入 PgBouncer 服务，API 默认通过 `pgbouncer:5432` 访问 PostgreSQL，并开放本地 `6432` 端口便于连接池验证 | `docker-compose.yml`、`README.md` | ✅ 已完成 |
| 5.6.3 | 事件表分区 | ✅ 已完成：事件投影表写入冲突键升级为 `ON CONFLICT (id, at)`，并补齐事件表组合唯一索引与时间索引（分区迁移兼容）；新增 `pg_partman` 迁移脚本，可将 `mistypass_access_events` / `mistypass_device_events` 转换为按月分区并配置 6 个月保留策略，同时补齐 README 执行说明 | `api/internal/state/postgres_store.go`、`api/internal/state/sqlc/queries/projection_core.sql`、`api/internal/state/sqlc/schema.sql`、`api/internal/state/sqlcgen/projection_core.sql.go`、`deploy/postgres/event-partitioning-partman.sql`、`README.md` | ✅ 已完成 |
| 5.6.4 | CI 安全扫描 | 已完成：新增 `security-scan.yml`，执行 `gosec ./...` 与 Trivy 容器镜像扫描（`api/Dockerfile` 构建 `mistypass-api:ci` 后按 `CRITICAL,HIGH` 阈值检查） | `.github/workflows/security-scan.yml`、`api/Dockerfile` | ✅ 已完成 |
| 5.6.5 | 依赖审计 | 已完成：新增 `dependency-audit.yml`，在 GitHub Actions 中执行 `go vuln check`（`govulncheck ./...`）与 `npm audit --audit-level=high` | `.github/workflows/dependency-audit.yml` | ✅ 已完成 |
| 5.6.6 | 生产部署方案 | 已完成：补充生产部署评估文档（Fly.io vs Kubernetes），给出分阶段推荐“先 Fly.io、后 Kubernetes”与迁移触发阈值 | `docs/architecture/production-deployment-evaluation.md` | ✅ 已完成 |

### 5.7 业务功能补齐（设计文档 vs 实现差异）

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.7.1 | App API 实现 | 已完成：`/api/v1/app/*` 端点已实现并接入鉴权与 resident 角色约束，覆盖 `auth/login`、`auth/refresh`、`me`、`credentials`、`access/doors`、`access/ble-token`、`access/logs`、`visitor-passes` | `api/internal/http/router.go`、`api/internal/http/routes_auth.go`、`README.md` | ✅ 已完成 |
| 5.7.2 | OTA 升级任务 | ✅ 已完成：新增网关 OTA 任务能力（创建/查询/状态流转），落地管理端接口 `POST/GET /api/v1/gateways/{gatewayID}/ota/tasks` 与 `PATCH /api/v1/gateways/{gatewayID}/ota/tasks/{taskID}/status`，支持固件版本/下载地址/SHA256 校验、任务状态（`queued/dispatching/succeeded/failed/canceled`）和审计留痕；补齐 gateway service 与 HTTP 路由单测 | `api/internal/modules/gateway/service.go`、`api/internal/modules/gateway/service_test.go`、`api/internal/http/routes_gateway_management.go`、`api/internal/http/router.go`、`api/internal/http/routes_gateway_ota_test.go`、`README.md` | ✅ 已完成 |
| 5.7.3 | 全量 API 强制 HTTPS/TLS | 已完成：新增 Caddy TLS 终止配置与 compose 覆盖文件，支持 `:443` 入口反向代理到 API，并在 README 提供生产域名启动方式 | `deploy/caddy/Caddyfile`、`docker-compose.tls.yml`、`README.md` | ✅ 已完成 |

---

## 六、整改进度总览

| 分类 | 总项数 | ✅ 已完成 | 🔄 进行中 | ⬜ 未完成 |
|------|--------|----------|----------|----------|
| 一、安全整改（P0） | 11 | 11 | 0 | 0 |
| 二、前端架构整改（P1） | 18 | 18 | 0 | 0 |
| 三、后端架构整改（P1） | 6 | 6 | 0 | 0 |
| 四、中间件与基础设施（P2） | 21 | 21 | 0 | 0 |
| 五、长期优化（P3） | 20 | 20 | 0 | 0 |
| **合计** | **76** | **76** | **0** | **0** |

---

## 七、推荐执行顺序

基于投入产出比和依赖关系，建议按以下顺序推进未完成项（当前已全部完成）：

**第一批（安全收尾）：** 已完成（P0 安全项全部完成）

**第二批（前端体验）：** 已完成（2.2.2 Zustand、2.2.3 API Client 自动 Token）

**第三批（Redis + 可观测性）：** 已完成（4.1.1~4.1.4 Redis 全套接入）

**第四批（前端质量）：** 已完成（4.3.2~4.3.4 组件拆分、4.4.1~4.4.5 分页/表单验证/TanStack Table/类型安全/内存泄漏修复）

**第五批（后端质量）：** 已完成（4.5.3 Projection 去重、4.5.4 sqlc 迁移）

**第六批（长期演进）：** 5.3.1/5.3.2（i18n 与多语言）已完成收口（全站页面 + 组件迁移完成，`access/wallet/enterprise` 文案精修完成，CI 中文硬编码扫描已接入，`check:cjk` 与 `build` 持续通过）；当前全量 key 2,136 中：`zh-CN` / `id-ID` 均已本地化 2,126，剩余 10 个等值项为符号占位/品牌词按设计保留；本轮完成 5.6.3（事件表分区）落地：事件写入键升级 + `pg_partman` 迁移维护脚本 + README 操作指引；完成 5.4.2（Encore 框架评估）最终决策报告收口；完成 5.2.3（事件/告警 SSE 实时推送）端到端接入；完成 5.5.1（前端 `features/` 目录重组）迁移收口；完成 5.2.1（EMQX MQTT 接入）配置与租户隔离 topic bootstrap 收口；完成 5.7.2（OTA 升级任务）网关任务 API 与状态流转落地；完成 5.2.2（NATS 总线）内部事件扇出接入；完成 5.1.1/5.1.2（外部认证 + 管理员 MFA）认证体系升级落地；并完成 5.4.1（模块分层重构）`tenant` 模块四层拆分收口 → 清单全部完成（76/76）
