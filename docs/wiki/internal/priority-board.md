# 优先级看板（内部开发，2026-04-17）

当前能力状态：

- `CONTRACT_READY`：本页口径与 `docs/development-status-roadmap.md` 一致，用于日常排期同步。
- `PROD_READY`：路线按 PRD v0.2 冻结为“Cloud Access SaaS + Edge Controller（2D/4D）+ Reader 兼容层”。

## 1. 当前高优先事项（无外部依赖优先）

### 1.0 安全修复队列（来自 `CODE-REVIEW.md`）

| 优先级 | 编号 | 事项 | 状态 | 说明 |
|---:|---|---|---|---|
| 1 | Q-P0-1 | 演示凭证隔离（ENABLE_DEMO_USERS + 前端仅开发态展示） | 已完成 | 生产环境默认不启用演示账号 |
| 2 | Q-P0-2 | bcrypt 密码哈希校验 | 已完成 | 登录校验不再明文比较 |
| 3 | Q-P0-3 | JWT Secret 默认值治理 | 已完成（生产环境） | `APP_ENV=production` 且缺失 `JWT_SECRET` 时拒绝启动 |
| 4 | Q-P0-4 | Projection 删除 SQL 表名白名单 | 已完成 | 非白名单表名直接拒绝执行 |
| 5 | Q-P1-2 | token 生命周期治理（AuthProvider + 自动刷新 + 失败登出） | 已完成（首阶段） | 已接入 `AuthProvider/useAuth`，登录态与会话恢复集中治理 |
| 6 | Q-P1-4 | 登录速率限制（IP） | 已完成 | `/api/v1/auth/login` 与 `/api/v1/app/auth/login` 已启用限速 |
| 7 | Q-P1-5 | Error Boundary | 已完成 | 运行时异常具备降级页，避免整页白屏 |
| 8 | Q-P1-1 | React Query 引入与请求层改造 | 已完成 | 已接入 QueryClientProvider，`dashboard-page` + `tenants-page` + `tenant-detail-page` + `events-page` + `alarms-page` + `spaces-page` + `audit-page` + `access-page` + `gateways-page` + `enterprise-page` + `wallet-page(bootstrap)` 已迁移 |
| 9 | Q-P1-3 | `router.go` 按模块拆分 | 已完成 | 已拆出 `routes_auth.go` + `routes_gateway_bootstrap.go` + `routes_gateway_management.go` + `routes_tenant_space.go` + `routes_access_audit.go` + `routes_wallet.go` + `routes_enterprise_auth.go` + `routes_enterprise_exchange.go` + `routes_enterprise_callback.go` + `routes_enterprise_management.go` + `routes_state_change.go` |

### 1.1 前端（Web Admin）

| 优先级 | 编号 | 事项 | 紧急度 | 状态 | 进度 | 当前推进焦点 |
|---:|---|---|---|---|---:|---|
| 1 | F7 | 构建回归、页面验证与性能守护 | `S1` | 进行中 | 100% | 分支级集中验证：`build + smoke + role-boundary + browser e2e + doc-marker` |
| 2 | F1/F4/F5/F6 | 企业态主路径已收口能力守卫 | `S1` | 已完成 | 100% | 新增改动仅做回归守卫，避免角色边界和信息架构回退 |

前端外部依赖项（不纳入本表执行顺序）：

- `F2`：HRIS/SCIM 上游返回语义与投递回执字段映射增强（外部验证后再收口）。

### 1.2 后端（Cloud/API + Edge）

| 优先级 | 编号 | 事项 | 紧急度 | 状态 | 进度 | 当前推进焦点 |
|---:|---|---|---|---|---:|---|
| 1 | R2 | PostgreSQL 增量写入与稳定回放 | `S0` | 进行中 | 99% | 持续积累 nightly 数据并完成 `>=7` 天签字 |
| 2 | R1 | 网关离线可运行闭环 | `S0` | 进行中 | 98% | `queue_hint/retry_subset/checkpoint` 合同回归守护 |
| 3 | R8 | Controller MVP 台架验证（2D/4D 方向） | `S1` | 进行中 | 58% | 单门 I/O 闭环 + `Wiegand/OSDP` 参数基线 + 实体证据留档 |
| 4 | R6 | 协议层强化（legacy->OSDP 升级路径） | `S1` | 进行中 | 68% | `WG-Branch-A` A2/A3 本地放行与门态链路收口 |
| 5 | R9 | Encore 渐进迁移试点 | `S1` | 进行中 | 89% | control-plane 试点评估与边界文档收敛 |
| 6 | R3 | Wallet 队列/重试/DLQ/可观测 | `S1` | 进行中 | 97% | 非 Meta 真通道路径持续守护（mock/resend） |

后端外部依赖项（不纳入本表执行顺序）：

- `R3` 的 Meta 企业号真实通道联调。
- `R7` 的 Google Wallet API（LEI 条件）。

## 2. 本周继续推进顺序（无外部依赖）

1. 后端 S0 主链路：`R2 + R1`（nightly 签字 + 合同回归持续稳定）。
2. 边端主链路：`R8 + R6`（单门闭环、协议兼容、实体留档）。
3. control-plane 持续项：`R9`（PoC 评估与边界决策文档）。
4. 前端守护：`F7`（分支级集中验证，避免已收口能力回退）。

## 3. 建议脚本（按变更域）

### 前端

- `docs/testing/curl-web-admin-smoke.zsh`
- `docs/testing/curl-web-admin-role-boundary-smoke.zsh`
- `docs/testing/curl-web-admin-browser-e2e.zsh`
- `cd web-admin && npm run build`

### 后端

- `docs/testing/curl-gateway-legacy-wiegand-poc.zsh`
- `docs/testing/curl-gateway-door-io-loop.zsh`
- `docs/testing/curl-gateway-event-retry-subset-mixed.zsh`
- `docs/testing/curl-gateway-event-checkpoint-partial.zsh`
- `docs/testing/curl-gateway-event-multi-queue-isolation.zsh`
- `docs/testing/curl-pg-replay-soak-review.zsh`
- `docs/testing/curl-pg-replay-soak-signoff.zsh`
- `docs/testing/curl-wallet-job-metrics-alert.zsh`
- `docs/testing/curl-wallet-job-alert-dispatch.zsh`

## 4. 每日同步最小清单

1. 先看 `docs/development-status-roadmap.md` 与 `docs/web-admin-enterprise-refactor-priority.md` 是否有状态变更。
2. 运行与当前变更域匹配的 `docs/testing/*.zsh` 与构建命令。
3. 若接口契约变化，同步更新：
- `docs/testing/admin-ui-test-and-api-map.md`
- `docs/wiki/internal/*`
- `docs/wiki/external-api/*`（若对外合同受影响）

## 5. 快速命令

```bash
# 文档状态标识守卫
./docs/testing/check-doc-capability-markers.zsh

# 后端单测
cd api && go test ./...

# 前端构建
cd web-admin && npm run build
```
