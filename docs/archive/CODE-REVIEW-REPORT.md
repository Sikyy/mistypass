# MistyPass 全项目 Code Review 与文档整理报告

> 生成日期：2026-04-24

---

## 一、修复进度（2026-04-24）

| 项目 | 当前状态 | 说明 |
|---|---|---|
| C1 | 已修复 | `/api/v1/gateway/register` 已要求 `X-Bootstrap-Token` / Bearer bootstrap token；production 启动时必须配置 `GATEWAY_BOOTSTRAP_TOKEN`。 |
| C2 | 已修复 | production 环境必须显式提供 `HRIS_VAULT_MASTER_KEY`，不再回退复用 `JWT_SECRET`。 |
| C3 | 已修复 | 非生产环境的 ephemeral JWT secret 已改为基于 CSPRNG 生成；随机源失败时直接 fail-closed。 |
| C4 | 已修复 | `docker-compose.yml` 已为 Redis 增加密码，并将宿主机端口绑定收紧到 `127.0.0.1`。 |
| C5 | 已修复 | `docker-compose.yml` 已移除 `admin/public` 默认组合，EMQX Dashboard 凭证改为环境变量可覆盖。 |
| C6 | 已修复 | `docker-compose.yml` 已移除 `postgres/postgres` 弱凭证，并将 PostgreSQL / PgBouncer 宿主机端口绑定收紧到 `127.0.0.1`。 |
| C7 | 已修复 | Wallet QR 预览已改为 Blob URL + `<img>`，移除 `dangerouslySetInnerHTML`。 |
| H1 | 已修复 | OIDC JWKS 拉取已加入响应体大小限制。 |
| H2 | 已修复 | OIDC JWKS 全局缓存已加入容量上限与驱逐策略。 |
| H3 | 已修复 | Demo users 默认关闭，必须显式设置 `ENABLE_DEMO_USERS=true`。 |
| H4 | 已修复 | Admin MFA 状态已持久化到存储层，服务重启后可恢复已注册 MFA 状态。 |
| H5 | 已修复 | 企业公开端点与 HRIS webhook 已拆分独立限速。 |
| H6 | 已修复 | 前端 API 动态路径参数已统一补齐 URL 编码。 |
| H7 | 已修复 | `AuthContext` 已补齐闭包依赖，消除 stale closure 风险。 |
| H8 | 已修复 | `.gitignore` 已补充 `.env*`、证书/密钥文件、compose override、IDE 目录等。 |
| H9 | 已修复 | `api/Dockerfile` builder 基础镜像已固定到具体 patch tag + sha256 digest。 |
| H10 | 已修复 | `security-scan.yml` / `dependency-audit.yml` 已将 `gosec`、`govulncheck` 从 `latest` 固定到明确版本。 |
| M3 | 已修复 | 登录 / 全局 API / enterprise public / enterprise webhook 的内存限速桶均已具备 TTL 清理与最大 key 数收缩。 |
| M4 | 已修复 | audit webhook delivery 记录已加入固定上限，避免无限增长。 |
| M5 | 已修复 | Dashboard 已改为基于真实 `alarms / gateways / checkpoint summary / tenants` 数据汇总，不再展示前端硬编码 mock KPI 与告警列表。 |
| M6 | 已修复 | 顶栏未读告警已改为按当前权限范围实时计算，不再持久化 `3` 这一假初始值到 localStorage。 |
| M1 | 已修复 | Auth Service 高频读路径与 token 校验已移出全局写锁；写锁仅保留在会话 / 用户状态变更阶段，并新增并发回归测试覆盖 `VerifyAccessToken` 读路径不再串行化。 |
| M2 | 已修复 | events / alarms SSE 已改为基于 service 订阅的变更驱动推送，仅保留 heartbeat，不再每 3 秒全量扫描。 |
| M8 | 已修复 | access 页面 grant 生命周期时钟已改为按分钟对齐刷新，移除整页每秒 `setInterval` 重渲染。 |
| M9 | 已修复 | 已移除前端自生成且服务端不校验的伪 CSRF token 机制；当前管理端仅依赖显式 Bearer token，不再保留无效的安全假象。 |
| M11 | 已修复 | 已新增 `web-admin-ci.yml`，在 PR / push 中执行 `check:cjk`、`typecheck`、`test:unit`、`build`，阻止前端 broken 状态合入。 |
| M12 | 已修复 | 已新增 `.github/dependabot.yml`，为 Go、npm 与 GitHub Actions 依赖开启每周自动更新。 |
| M10 | 已修复 | `deploy/caddy/Caddyfile` 已补 `Content-Security-Policy` 与 `Permissions-Policy`。 |
| M13 | 已修复 | `CurrentUser.role` 已从宽泛 `string` 收敛为 `UserRole` 联合类型。 |
| M7 | 已修复 | 已将 `wallet page overview`、`wallet operations overview cards`、`wallet delivery workspace / recent delivery`、`wallet advanced workspace`、`wallet physical tasks / recent physical tasks`、`wallet send delivery`、`wallet recent delivery receipts`、`wallet QR dialog`、`wallet issued passes`、`enterprise page header`、`enterprise page overview`、`enterprise workflow / next action overview` 抽离到独立组件 `wallet-page-overview.tsx`、`wallet-operations-overview-cards.tsx`、`wallet-delivery-workspace-panels.tsx`、`wallet-advanced-workspace.tsx`、`wallet-physical-card-tasks-section.tsx`、`wallet-send-delivery-card.tsx`、`wallet-delivery-receipts-card.tsx`、`wallet-pass-qr-dialog.tsx`、`wallet-issued-passes-card.tsx`、`enterprise-page-header.tsx`、`enterprise-page-overview.tsx`、`enterprise-workflow-overview.tsx`；同时将 wallet 场景/状态/格式化等纯推导逻辑迁出到 `wallet-page-utils.ts`，`wallet-page.tsx` 已从 4944 行降到 3192 行，`enterprise-page.tsx` 保持在 3996 行，两个主页面均已收敛到 4k 阈值内并通过 `npm run typecheck`、`npm run build` 验证。 |
| L1 | 已核实 | 本机 `go version go1.26.2`；`api/go.mod` 中的 `go 1.25.0` 为当前合法的 patch 级 Go 指令格式，原“无效版本”判断已过时，无需额外改码。 |
| L2 | 已修复 | `api/internal/config/config.go` 已拆分为 `loadServerConfig / loadRedisConfig / loadBrokerConfig / loadOTelConfig / loadAuthConfig / loadDatabaseConfig / loadEnterpriseConfig / loadGatewayConfig / loadWalletConfig` 等按域加载函数，`FromEnv()` 不再承载 700+ 行单体逻辑。 |
| L3 | 已修复 | NATS publisher 已在 `PublishJSON` 中检查 canceled context / 初始化状态，并在 publish 后执行 `FlushWithContext(ctx)`。 |
| L4 | 已修复 | PostgreSQL store 已补默认连接池参数：`MaxOpenConns=25`、`MaxIdleConns=10`、`ConnMaxIdleTime=5m`、`ConnMaxLifetime=30m`。 |
| L5 | 已修复 | 页面 `token` prop 已从可选收敛为必填，`App.tsx` 已统一透传，不再保留 `token?: string` 弱类型。 |
| L6 | 已修复 | React Query key 已移除 token；`AuthProvider` 在登录/登出/会话清空时会 `queryClient.clear()`，避免跨会话缓存泄露。 |
| L7 | 已修复 | 密码校验已保留原始输入，不再先 `TrimSpace` 再做 bcrypt 比较。 |
| L8 | 已修复 | events / alarms / audit / tenants / gateways / access 等关键搜索和筛选控件已补 `aria-label`，覆盖高频页面的可访问名称缺口。 |

当前剩余优先级：无（本轮 Code Review 修复项已全部收口）

已完成验证：`cd api && go test ./...`，`cd web-admin && npm install`，`cd web-admin && npm run test:unit`，`cd web-admin && npm run typecheck`，`cd web-admin && npm run build`，`go build -o /tmp/mistypass-api-verify ./cmd/api`，GitHub Actions / Dependabot YAML 本地语法解析通过

补充说明：本机 Docker daemon 当前不可用，因此 `api/Dockerfile` 未执行完整 `docker build` 镜像构建校验；已通过对应的 Go 编译路径完成替代性验证。

---

## 二、Code Review 发现（按严重程度排序）

### CRITICAL（必须立即修复）

| # | 问题 | 位置 | 说明 |
|---|------|------|------|
| C1 | 网关 bootstrap 端点无认证 | `router.go:367-378` | `/api/v1/gateway/` 下所有端点（register/events/batch/config/checkpoint）无任何鉴权中间件，任何人可注册假网关、提交伪造事件 |
| C2 | HRIS 加密密钥回退到 JWT Secret | `router.go:155` | `HRIS_VAULT_MASTER_KEY` 未设置时复用 JWT 签名密钥，泄露 JWT Secret 即可解密所有 HRIS 凭证 |
| C3 | 非生产环境 JWT Secret 可预测 | `auth/service.go:1341` | 非 production 环境的 ephemeral secret 回退为 `mistypass-ephemeral-{timestamp}`，可被猜测 |
| C4 | Redis 无密码暴露 | `docker-compose.yml:69` | Redis 绑定宿主机 6379 端口且无密码，网络可达即可读写 |
| C5 | EMQX 使用默认凭证 | `docker-compose.yml:85-86` | 管理面板 `admin/public` 暴露在 18083 端口 |
| C6 | PostgreSQL 弱凭证暴露 | `docker-compose.yml:33-36` | `postgres/postgres` 绑定宿主机 5432 端口 |
| C7 | 前端 `dangerouslySetInnerHTML` XSS 风险 | `wallet-page.tsx:4875` | QR 码 SVG 直接注入 DOM，未经 sanitize |

### HIGH（高优先级）

| # | 问题 | 位置 | 说明 |
|---|------|------|------|
| H1 | OIDC JWKS 响应无大小限制 | `oidc.go:337` | `io.ReadAll` 无 limit，恶意 JWKS 端点可导致 OOM |
| H2 | OIDC JWKS 缓存无上限 | `oidc.go:88-93` | 全局 map 无容量限制，可被恶意 IdP 配置撑爆内存 |
| H3 | 演示用户弱密码默认开启 | `auth/service.go:1252-1312` | 非 production 环境默认注入 `admin123` 密码的 super_admin |
| H4 | MFA 状态仅内存存储 | `auth/service.go:123` | 重启后所有 MFA 注册丢失，已启用 MFA 的管理员被锁定 |
| H5 | 企业公开端点缺少独立限速 | `router.go:341-348` | HRIS webhook 等公开端点仅有全局限速 |
| H6 | API 路径参数未 URL 编码 | `api.ts` 多处 | ~20 处路径插值中仅 4 处使用 `encodeURIComponent` |
| H7 | Auth Context useMemo 依赖不完整 | `auth-context.tsx:94-103` | `logout`/`setAuthenticatedSession` 未包含在 deps 中，存在 stale closure 风险 |
| H8 | `.gitignore` 严重不足 | 根目录 `.gitignore` | 缺少 `.env*`、`*.pem`、`*.key`、`docker-compose.override.yml` 等 |
| H9 | Dockerfile 基础镜像未固定 digest | `api/Dockerfile` | `golang:1.25-bookworm` 未 pin sha256 |
| H10 | CI 工具版本未固定 | `security-scan.yml` | `gosec@latest`、`govulncheck@latest` 可被供应链攻击 |

### MEDIUM（中优先级）

| # | 问题 | 位置 |
|---|------|------|
| M1 | Auth Service 全局互斥锁（已修复） | `auth/service.go` — 所有操作串行化，高并发瓶颈 |
| M2 | SSE 3 秒轮询无推送机制 | `routes_access_audit.go:624` — O(n) 客户端轮询 |
| M3 | 内存限速桶无上限 | `router.go:75-77` — 分布式攻击下 map 膨胀 |
| M4 | Webhook 投递记录无上限增长 | `audit/webhook.go:303` |
| M5 | Dashboard 硬编码 mock 数据 | `dashboard-page.tsx` — 生产环境展示虚假 KPI |
| M6 | `unreadAlarmCount` 初始化为 3 | `ui-store.ts:7` — 持久化到 localStorage 的假数据 |
| M7 | 页面组件过大（已修复） | `wallet-page.tsx` 3192 行、`enterprise-page.tsx` 3996 行 |
| M8 | 1 秒 setInterval 触发整页重渲染 | `access-page.tsx:473` |
| M9 | CSRF token 纯客户端生成 | `auth.ts:93-113` — 若服务端不校验则无效 |
| M10 | Caddy 缺少 CSP 和 Permissions-Policy | `deploy/caddy/Caddyfile` |
| M11 | 无 web-admin 构建/lint CI | 前端代码可在 broken 状态合入 |
| M12 | 无 Dependabot/Renovate | 依赖更新无自动化 |
| M13 | `CurrentUser.role` 类型为 `string` | `api.ts:289` — 应为联合类型 |

### LOW（低优先级）

| # | 问题 |
|---|------|
| L1 | `go 1.25.0` 在 go.mod 中（已核实：当前为合法 patch 级 Go 指令格式） |
| L2 | Config 结构体 100+ 字段，`FromEnv()` 700+ 行（已拆分为按域加载） |
| L3 | NATS publisher 忽略 context（已修复） |
| L4 | PostgreSQL 连接池未调优（无 MaxOpenConns/MaxIdleConns，已修复） |
| L5 | 所有页面组件声明了未使用的 `token?: string` prop（已修复） |
| L6 | Token 字符串出现在 React Query cache key 中（DevTools 可见，已修复） |
| L7 | 密码 trim 后再 bcrypt 比较（空格被静默去除，已修复） |
| L8 | 无障碍：`aria-label` 覆盖不足（关键高频页面已补齐） |

---

## 三、测试覆盖分析（2026-04-24 更新）

### 后端测试状态

| 模块 | 测试状态 |
|------|----------|
| `space/service.go` | 已补单测：拓扑创建成功、tenant ownership mismatch、door kind/status 校验 |
| `alarm/service.go` | 已有订阅与状态更新测试，不再是零测试 |
| `tenant/service.go` | 已补单测：create success / invalid input、update status success / invalid / not found |
| `enterprise/saml.go` | 已补专项测试：response decode、x509 normalize、identity build |
| `access/service.go` | 已有 sync identity 相关单测；CRUD 维度仍可继续扩展，但不再属于零覆盖 |
| `audit/service.go` | 已有 list/filter service test；更细粒度 service 行为仍可继续扩展 |
| `postgres_store_test.go` | 已在本地 `go test ./...` 通过；`.github/workflows/api-smoke.yml` 已提供 PostgreSQL service，不存在“CI 静默跳过”结论 |

### 前端测试状态

- 已新增 Vitest unit test 流程：`cd web-admin && npm run test:unit`
- 已新增 5 个前端 unit test 文件、16 个用例，覆盖 `auth`、`wallet-page-utils`、`access-page-utils`、`access-grant-ledger-utils`、`access-enterprise-flow-utils`
- `web-admin-ci.yml` 现会执行 `npm ci -> check:cjk -> typecheck -> test:unit -> build`
- 既有 Playwright 覆盖不止 enterprise/HRIS，也包含 access / role boundary 等主线；原“仅覆盖 enterprise/HRIS”判断已过时
- 后续可选增强方向：补 Dashboard / Tenants / Spaces / Gateways / Wallet / Events / Alarms 的独立 page-smoke E2E，以及抽取共享 fixture 降低 mock 重复

---

## 四、文档审查与整理结果

### 已删除

| 文件 | 原因 |
|------|------|
| `.aider.chat.history.md` | aider 工具的废弃会话日志，无任何有用信息 |
| `docs/testing/artifacts/` 下 21 个旧验证日志和报告 | 仅保留最新的 2 个 md + 7 个最新 log |

### 已归档到 `docs/archive/`

| 文件 | 原因 |
|------|------|
| `REFORM-CHECKLIST.md` | 76/76 项全部完成，已是历史快照 |
| `MistyPass-云SaaS平台开发文档.md` | 652 行初始设计文档，已被现有文档体系取代 |
| `encore-migration-playbook.md` | Encore 迁移已决策冻结（保持 Go+Chi），仅保留决策报告 |
| `encore-poc-evaluation-baseline.md` | 同上，评估基线已完成使命 |

### 已整合

| 操作 | 说明 |
|------|------|
| `module-handbook.md` + `module-deep-dive.md` → `module-reference.md` | 合并为一个文件，涵盖模块总览、状态键、前后端映射、测试入口和典型改动场景 |
| `priority-board.md` 精简 | 改为 roadmap 的精简视图，去除与 roadmap 重复的已完成内容 |
| `docs/wiki/README.md` 精简 | 从 41 行冗长链接列表精简为结构化表格 |

### 已更新交叉引用

- `docs/wiki/internal/system-overview.md` — 迁移策略链接更新
- `docs/wiki/internal/README.md` — 阅读顺序更新
- `docs/development-status-roadmap.md` — R9 归档文档路径更新
- `docs/architecture/encore-decision-report-2026-04-19.md` — 评估输入路径更新

---

## 五、正面发现

- 零 SQL 注入风险 — 全部使用 sqlc 参数化查询
- 零 `any` 类型、零 `@ts-ignore` — TypeScript 类型安全做得好
- i18n 三语言 2329 个 key 完全对齐，零缺失
- Token 已从 localStorage 迁移到 sessionStorage
- Token 刷新正确处理了并发去重
- 所有 useEffect 都有正确的 cleanup
- 路由级 code splitting 实现良好
- 表单验证使用 zod + i18n 错误消息
- 外部 API 文档体系完整，模板一致性高
- 能力状态标识（PROD_READY/CONTRACT_READY）统一且有 CI 守卫

---

## 六、整理后文档结构

```
MistyPass/
├── README.md                          # 项目入口
├── NEXT-PHASE-PLAN.md                 # 后续实施计划（76 项）
├── docs/
│   ├── archive/                       # 已归档历史文档
│   │   ├── REFORM-CHECKLIST.md
│   │   ├── MistyPass-云SaaS平台开发文档.md
│   │   ├── encore-migration-playbook.md
│   │   └── encore-poc-evaluation-baseline.md
│   ├── architecture/                  # 架构决策与技术方案
│   │   ├── capability-status-markers.md
│   │   ├── completed-features-tech-stack.md
│   │   ├── encore-decision-report-2026-04-19.md
│   │   ├── gateway-serial-protocol-mobile-plan.md
│   │   └── production-deployment-evaluation.md
│   ├── development-status-roadmap.md  # 开发状态总览（R1-R9）
│   ├── web-admin-enterprise-refactor-priority.md
│   ├── edge/                          # 边缘设备方案
│   ├── enterprise/                    # 企业接入设计
│   ├── sprints/                       # Sprint 计划
│   ├── testing/                       # 测试文档与回归脚本
│   │   └── artifacts/                 # 仅保留最新验证记录
│   ├── wallet/                        # Wallet 方案
│   └── wiki/
│       ├── README.md                  # Wiki 总入口
│       ├── internal/                  # 内部开发 Wiki
│       │   ├── README.md
│       │   ├── system-overview.md
│       │   ├── module-reference.md    # 合并后的模块参考手册
│       │   ├── priority-board.md      # 精简后的优先级看板
│       │   └── dev-workflow.md
│       └── external-api/              # 对外 API 文档（40+ 页）
```
