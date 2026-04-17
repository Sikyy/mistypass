# 模块深描（职责 / 状态 / 测试入口）

当前能力状态：

- `CONTRACT_READY`：模块边界与主路由分组已稳定。
- `PROD_READY`：下述代码路径、状态键与回归入口均对齐当前主干。

## 1. 后端模块总览

| 领域 | 代码目录 | 状态键（state key） | 关键数据 | 关键接口组 |
|---|---|---|---|---|
| Auth | `api/internal/modules/auth` | 无独立 state key（内存会话） | token、refresh session、trusted user | `/auth/*`、`/me` |
| Tenant | `api/internal/modules/tenant` | `module_tenant` | tenants | `/tenants*` |
| Space | `api/internal/modules/space` | `module_space` | buildings/floors/areas/doors/door-groups | `/buildings` `/floors` `/areas` `/doors` |
| Access | `api/internal/modules/access` | `module_access` | policies/users/groups/temporary-access/visitor-passes | `/access-policies` `/users` `/user-groups` `/temporary-access` |
| Gateway | `api/internal/modules/gateway` | `module_gateway` | gateways、serial inventory、config states、event checkpoints、queue ingest totals | `/gateway/*`、`/gateways/*` |
| Event | `api/internal/modules/event` | `module_event` | access events、device events | `/events/*`、`/gateway/events/*` |
| Alarm | `api/internal/modules/alarm` | `module_alarm` | alarms | `/alarms*` |
| Audit | `api/internal/modules/audit` | `module_audit` | audit logs、webhook config、deliveries | `/audit-logs`、`/audit/webhook/*` |
| Enterprise | `api/internal/modules/enterprise` | `module_enterprise` | domain mapping、idp config、employees、sync jobs、jit approvals | `/enterprise/*` |
| Wallet | `api/internal/modules/wallet` | `module_wallet` | config/templates/passes/jobs/dlq/metrics/alerts | `/wallet/*` |
| Wallet Alert Dispatch | `api/internal/modules/wallet/alertdispatch` | 复用 wallet 状态 | 订阅评估、冷却、通道分发编排 | `/wallet/jobs/alerts/dispatch` |

## 2. Router 层约束

主入口：`api/internal/http/router.go`

Router 只做：

- 入参解析、鉴权、角色校验。
- 错误码映射与统一错误响应。
- 审计埋点与跨模块轻量编排。

复杂领域决策必须下沉到 `api/internal/modules/*`，避免“胖路由”。

## 3. 前端页面与后端域映射

| 页面 | 文件 | 主要后端域 |
|---|---|---|
| 登录 | `web-admin/src/pages/login-page.tsx` | auth |
| 概览 | `web-admin/src/pages/dashboard-page.tsx` | tenant、gateway |
| 租户列表/详情 | `web-admin/src/pages/tenants-page.tsx` `tenant-detail-page.tsx` | tenant、space |
| 空间管理 | `web-admin/src/pages/spaces-page.tsx` | space |
| 权限管理 | `web-admin/src/pages/access-page.tsx` | access、enterprise |
| 网关管理 | `web-admin/src/pages/gateways-page.tsx` | gateway、event |
| 事件查询 | `web-admin/src/pages/events-page.tsx` | event |
| 告警管理 | `web-admin/src/pages/alarms-page.tsx` | alarm |
| Wallet 运营 | `web-admin/src/pages/wallet-page.tsx` | wallet |
| 审计页（已实现未挂路由） | `web-admin/src/pages/audit-page.tsx` | audit |

## 4. 测试与回归入口（按域）

| 领域 | 单测入口 | 脚本入口 |
|---|---|---|
| Gateway/Event | `api/internal/http/router_gateway_*_test.go`、`api/internal/modules/gateway/service_test.go`、`api/internal/modules/event/service_test.go` | `docs/testing/curl-gateway-*.zsh` |
| Enterprise | `api/internal/http/router_enterprise_*_test.go`、`api/internal/modules/enterprise/*_test.go` | `docs/testing/curl-enterprise-*.zsh` |
| Wallet | `api/internal/modules/wallet/*_test.go`、`api/internal/modules/wallet/alertdispatch/planner_test.go` | `docs/testing/curl-wallet-job-*.zsh` |
| Audit/Webhook | `api/internal/modules/audit/*_test.go` | `docs/testing/curl-audit-webhook-fanout.zsh` |
| Replay/State | `api/internal/state/*_test.go` | `docs/testing/curl-pg-replay-*.zsh` |

## 5. 典型改动落点

### 场景 A：新增网关 batch 字段

1. `router.go`：补 request decode 与响应映射。
2. `modules/event`：补领域校验与存储。
3. 回归：新增 `curl-gateway-event-*.zsh` 断言。
4. 文档：更新 external reference 页与 admin API map。

### 场景 B：新增 Wallet 告警通道

1. `modules/wallet/alertdispatch`：补编排策略。
2. `modules/wallet/alert_*_provider.go`：补 provider 实现。
3. 回归：补 `alert-dispatch` 与 `retry` 脚本。
4. 文档：更新 wallet 运营页说明与外部 reliability 章节。

### 场景 C：企业回调错误码调整

1. `router_enterprise_*`：细化 HTTP 映射。
2. `modules/enterprise`：统一业务错误类型。
3. 回归：补 callback/exchange 路由级状态码测试。
4. 文档：更新 external guide（企业接入）与 roadmap。
