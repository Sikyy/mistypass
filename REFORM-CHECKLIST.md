# MistyPass 整改清单

> 生成日期：2026-04-17
> 依据文档：`CODE-REVIEW.md`、`TECH-STACK-RECOMMENDATIONS.md`
> 状态说明：✅ 已完成 | 🔄 进行中 | ⬜ 未完成

---

## 一、安全整改（P0 — 立即修复）

### 1.1 认证与凭证安全

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 1.1.1 | 演示凭证隔离 | 后端通过 `ENABLE_DEMO_USERS` 环境变量控制演示用户注入，默认关闭；前端登录页仅开发环境展示测试账号 | `auth/service.go`、`login-page.tsx` | ✅ 已完成 |
| 1.1.2 | 密码 bcrypt 哈希存储 | 替换明文密码存储与比较，使用 `golang.org/x/crypto/bcrypt` | `auth/service.go` | ✅ 已完成 |
| 1.1.3 | JWT Secret 默认值治理 | 移除硬编码默认密钥，生产环境未配置 `JWT_SECRET` 时拒绝启动 | `config/config.go` | ✅ 已完成 |
| 1.1.4 | 网关设备 Token 动态化 | 移除 `router.go` 中硬编码的演示网关 token，改为数据库存储 + 注册流程动态生成 | `http/router.go` | ⬜ 未完成 |

### 1.2 数据访问安全

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 1.2.1 | SQL 注入防护 — 表名白名单 | `deleteProjectionRowsNotInIDs` 增加表名白名单校验 | `postgres_store.go` | ✅ 已完成 |

### 1.3 Token 与会话安全

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 1.3.1 | Token 存储迁移 | 将 JWT 从 `localStorage` 迁移到 `httpOnly cookie` 或 `sessionStorage + 内存` 组合方案，防止 XSS 窃取 | `web-admin/src/lib/auth.ts` | ⬜ 未完成 |
| 1.3.2 | Refresh Token 自动续期 | 实现 401 拦截 → refresh token 自动刷新 → 失败自动登出 | `auth.ts`、`api.ts`、`AuthProvider` | ✅ 已完成 |
| 1.3.3 | 401 响应自动登出 | API client 收到 401 时自动清除 token 并跳转登录页 | `api.ts`、`AuthProvider` | ✅ 已完成 |
| 1.3.4 | CSRF 防护预留 | 若未来迁移到 cookie 方案，需同步添加 CSRF token 机制 | — | ⬜ 未完成 |

### 1.4 接口防护

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 1.4.1 | 登录接口速率限制 | 基于 IP 的速率限制，`/api/v1/auth/login` 与 `/api/v1/app/auth/login` 每 IP 每分钟限速 | `http/` 中间件 | ✅ 已完成 |
| 1.4.2 | 全局 API 速率限制 | 对所有 API 端点添加通用速率限制中间件 | `http/` 中间件 | ⬜ 未完成 |

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
| 2.2.2 | 引入 Zustand | 管理 UI 全局状态（sidebar 折叠、通知等），替代 prop drilling | — | ⬜ 未完成 |
| 2.2.3 | API Client 自动附加 Token | API client 从 context 自动获取 token，无需每次调用手动传入 | `api.ts` | ⬜ 未完成 |

### 2.3 错误处理与路由

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 2.3.1 | 全局 Error Boundary | `main.tsx` 添加 `AppErrorBoundary`，运行时异常降级展示 | `main.tsx` | ✅ 已完成 |
| 2.3.2 | ProtectedRoute 组件化 | 提取 `<ProtectedRoute>` 组件，统一权限检查和重定向逻辑，消除重复代码 | `App.tsx` | ⬜ 未完成 |
| 2.3.3 | 404 页面 | 添加 Not Found 页面作为路由兜底，替代静默重定向到 `/dashboard` | `App.tsx` | ⬜ 未完成 |

---

## 三、后端架构整改（P1 — 短期 1-2 周）

### 3.1 代码结构拆分

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 3.1.1 | router.go 按模块拆分 | 拆分为 `routes_auth.go`、`routes_gateway_*.go`、`routes_tenant_space.go`、`routes_access_audit.go`、`routes_wallet.go`、`routes_enterprise_*.go`、`routes_state_change.go` | `internal/http/` | ✅ 已完成 |

### 3.2 关键数据持久化

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 3.2.1 | 用户账号持久化 | 将内存中的用户账号/密码迁移到 PostgreSQL 或外部认证服务 | `auth/service.go` | ⬜ 未完成 |
| 3.2.2 | Refresh Token 持久化 | 将 refresh token session 迁移到 Redis/PostgreSQL | `auth/service.go` | ⬜ 未完成 |
| 3.2.3 | Revoked Token 持久化 | 将已撤销的 access token 迁移到 Redis | `auth/service.go` | ⬜ 未完成 |
| 3.2.4 | 网关设备 Token 持久化 | 将网关 token 迁移到数据库，通过注册流程动态生成 | `http/router.go` | ⬜ 未完成 |

### 3.3 代码健壮性

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 3.3.1 | 事务回滚错误日志 | 将 `_ = tx.Rollback()` 改为记录回滚错误到日志，防止连接池泄漏不可观测 | `postgres_store.go` | ⬜ 未完成 |

---

## 四、中间件与基础设施引入（P2 — 中期 1-2 个月）

### 4.1 Redis 接入

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.1.1 | Redis / Dragonfly 部署 | 引入 Redis 或 Dragonfly 作为缓存与临时状态存储 | 基础设施 | ⬜ 未完成 |
| 4.1.2 | Session 存储迁移 | 将 session 从内存迁移到 Redis | `auth/service.go` | ⬜ 未完成 |
| 4.1.3 | Token 黑名单迁移 | 将 revoked token 列表迁移到 Redis | `auth/service.go` | ⬜ 未完成 |
| 4.1.4 | 速率限制后端存储 | 速率限制计数器从内存迁移到 Redis，支持多实例部署 | 中间件 | ⬜ 未完成 |

### 4.2 可观测性建设

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.2.1 | 结构化日志 — slog | 将 `log.Printf` 替换为 Go 1.21+ 标准库 `slog`，JSON handler 输出 | 全局 | ⬜ 未完成 |
| 4.2.2 | 请求日志中间件 | 添加 HTTP 请求/响应日志中间件，记录 method、path、status、duration、request_id | `internal/http/` | ⬜ 未完成 |
| 4.2.3 | Prometheus Metrics 端点 | 暴露 `/metrics` 端点，接入 Prometheus client | `internal/http/` | ⬜ 未完成 |
| 4.2.4 | OpenTelemetry 接入 | 统一 traces/metrics/logs，支持 Jaeger/Grafana Tempo 后端 | 全局 | ⬜ 未完成 |

### 4.3 前端巨型组件拆分

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.3.1 | access-page 拆分 | 拆分为 DirectorySection、PoliciesSection、GrantsSection（当前 1,727 行 / 58 state） | `access-page.tsx` | ⬜ 未完成 |
| 4.3.2 | gateways-page 拆分 | 拆分为 GatewayList、SerialInventory、DeviceConfig、CheckpointMonitor（当前 1,632 行 / 51 state） | `gateways-page.tsx` | ⬜ 未完成 |
| 4.3.3 | wallet-page 拆分 | 拆分为 TemplateManager、JobQueue、DLQPanel、AlertConfig（当前 ~1,500 行 / ~40 state） | `wallet-page.tsx` | ⬜ 未完成 |
| 4.3.4 | enterprise-page 拆分 | 每个 tab 独立为子组件（当前 ~1,200 行 / ~35 state） | `enterprise-page.tsx` | ⬜ 未完成 |

### 4.4 前端功能补齐

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.4.1 | 列表页分页 | 所有列表页接入分页组件，传递 `page`/`limit` 参数（API 已支持） | 各列表页 | ⬜ 未完成 |
| 4.4.2 | 表单验证 — React Hook Form + Zod | 引入 RHF + Zod，替代当前无验证直接提交的表单，与 shadcn/ui Form 组件集成 | 各表单页 | ⬜ 未完成 |
| 4.4.3 | 表格组件 — TanStack Table | 引入 TanStack Table，统一排序、筛选、分页、列可见性控制 | 各列表页 | ⬜ 未完成 |
| 4.4.4 | 类型安全修复 | 移除 API 类型定义中的 `\| string` 后缀，使用严格联合类型 | `api.ts` | ⬜ 未完成 |
| 4.4.5 | 内存泄漏修复 | `gateways-page.tsx` 中 `setTimeout` 添加 `useEffect` cleanup，防止组件卸载后状态更新 | `gateways-page.tsx` | ⬜ 未完成 |

### 4.5 后端功能补齐

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 4.5.1 | Webhook 签名验证 | 为 webhook 配置 HMAC-SHA256 签名，接收方可验证请求真实性 | `audit/webhook.go` | ⬜ 未完成 |
| 4.5.2 | Webhook 重试机制 | 实现指数退避重试（最多 3 次） | `audit/webhook.go` | ⬜ 未完成 |
| 4.5.3 | Projection 代码去重 | 提取通用 projection 函数，通过泛型或代码生成减少 `postgres_store.go` 中的重复逻辑 | `postgres_store.go` | ⬜ 未完成 |
| 4.5.4 | sqlc 迁移 | 将 2300+ 行手写 SQL 迁移到 sqlc 生成的类型安全代码 | `postgres_store.go` | ⬜ 未完成 |

---

## 五、长期优化（P3 — 持续迭代）

### 5.1 认证体系升级

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.1.1 | 外部认证服务接入 | 评估并接入 Ory Kratos 或 Casdoor，替换自研 auth 模块，一次性解决密码策略、MFA、OIDC/SAML 支持 | `auth/` 模块 | ⬜ 未完成 |
| 5.1.2 | 管理员 MFA | 实现多因素认证（设计文档要求，MVP 非目标） | `auth/` 模块 | ⬜ 未完成 |

### 5.2 设备通信层

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.2.1 | EMQX MQTT Broker 接入 | 部署 EMQX，实现网关设备 MQTT 通信，支持多租户 topic 隔离 | 新增模块 | ⬜ 未完成 |
| 5.2.2 | NATS 内部消息总线 | 引入 NATS 用于服务间异步通信（webhook 重试、告警分发、audit 事件扇出） | 新增模块 | ⬜ 未完成 |
| 5.2.3 | 前端实时推送 — SSE | 事件页面和告警页面接入 Server-Sent Events，替代手动刷新 | 前端 + 后端 | ⬜ 未完成 |

### 5.3 国际化

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.3.1 | react-i18next 接入 | 引入 i18n 框架，提取所有硬编码中文字符串到语言文件 | 前端全局 | ⬜ 未完成 |
| 5.3.2 | 多语言支持 | 至少支持中文 + 英文 + 印尼文（目标市场雅加达） | 语言文件 | ⬜ 未完成 |

### 5.4 后端架构演进

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.4.1 | 模块分层重构 | 按设计文档建议将扁平 `service.go` 拆分为 `domain/application/infrastructure/delivery` 四层 | `internal/modules/` | ⬜ 未完成 |
| 5.4.2 | Encore 框架评估 | 评估从 Chi 迁移到 Encore（已有 `encore-migration-playbook.md` 89% 完成），获得原生 service 调用、API 文档、内置 tracing | 全局 | ⬜ 未完成 |
| 5.4.3 | JWKS 缓存 | `enterprise/oidc.go` 中 JWKS 响应添加缓存（TTL 1 小时），减少对 IdP 的请求压力 | `enterprise/oidc.go` | ⬜ 未完成 |

### 5.5 前端架构演进

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.5.1 | 前端目录按领域重组 | 从页面级扁平组织迁移到 `features/` 目录按领域组织（设计文档建议） | `web-admin/src/` | ⬜ 未完成 |

### 5.6 基础设施与 DevOps

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.6.1 | Docker Compose 开发环境 | 编排 API + PostgreSQL + Redis + EMQX 的本地开发环境 | `docker-compose.yml` | ⬜ 未完成 |
| 5.6.2 | PgBouncer 连接池 | 引入 PgBouncer 减少数据库连接数 | 基础设施 | ⬜ 未完成 |
| 5.6.3 | 事件表分区 | 使用 pg_partman 按时间分区事件表，实现冷热分层 | 数据库 | ⬜ 未完成 |
| 5.6.4 | CI 安全扫描 | GitHub Actions 中添加 Trivy（容器镜像）+ gosec（Go 代码）扫描 | `.github/workflows/` | ⬜ 未完成 |
| 5.6.5 | 依赖审计 | CI 中添加 `go vuln check` + `npm audit` | `.github/workflows/` | ⬜ 未完成 |
| 5.6.6 | 生产部署方案 | 评估 Fly.io（轻运维）或 K8s（已有经验时）作为生产容器编排 | 基础设施 | ⬜ 未完成 |

### 5.7 业务功能补齐（设计文档 vs 实现差异）

| # | 整改项 | 说明 | 涉及文件 | 状态 |
|---|--------|------|----------|------|
| 5.7.1 | App API 实现 | 实现 `/app/*` 端点（Phase 1 目标） | 新增模块 | ⬜ 未完成 |
| 5.7.2 | OTA 升级任务 | 实现网关 OTA 固件升级功能（Phase 2 目标） | 新增模块 | ⬜ 未完成 |
| 5.7.3 | 全量 API 强制 HTTPS/TLS | 部署时配置 TLS 终止 | 基础设施 | ⬜ 未完成 |

---

## 六、整改进度总览

| 分类 | 总项数 | ✅ 已完成 | 🔄 进行中 | ⬜ 未完成 |
|------|--------|----------|----------|----------|
| 一、安全整改（P0） | 10 | 6 | 0 | 4 |
| 二、前端架构整改（P1） | 17 | 14 | 0 | 3 |
| 三、后端架构整改（P1） | 6 | 1 | 0 | 5 |
| 四、中间件与基础设施（P2） | 18 | 0 | 0 | 18 |
| 五、长期优化（P3） | 17 | 0 | 0 | 17 |
| **合计** | **68** | **21** | **0** | **47** |

---

## 七、推荐执行顺序

基于投入产出比和依赖关系，建议按以下顺序推进未完成项：

**第一批（安全收尾）：** 1.1.4 网关 Token 动态化 → 1.3.1 Token 存储迁移 → 1.4.2 全局速率限制

**第二批（前端体验）：** 2.2.2 Zustand → 2.2.3 API Client 自动 Token → 2.3.2 ProtectedRoute → 2.3.3 404 页面 → 4.4.5 内存泄漏修复

**第三批（Redis + 可观测性）：** 4.1.1~4.1.4 Redis 全套接入 → 3.2.1~3.2.4 关键数据持久化 → 4.2.1~4.2.2 结构化日志

**第四批（前端质量）：** 4.3.1~4.3.4 巨型组件拆分 → 4.4.1 分页 → 4.4.2 表单验证 → 4.4.3 TanStack Table → 4.4.4 类型安全

**第五批（后端质量）：** 4.5.1~4.5.2 Webhook 增强 → 4.5.3~4.5.4 Projection 去重 + sqlc → 3.3.1 事务回滚日志 → 4.2.3~4.2.4 Metrics + OTel

**第六批（长期演进）：** 按 P3 各子项的业务优先级逐步推进
