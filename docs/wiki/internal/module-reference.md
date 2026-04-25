# 模块参考手册（Backend + Frontend）

当前能力状态：

- `PROD_READY`：模块职责与当前实现一致，可直接用于开发分工。
- `CONTRACT_READY`：关键接口分组稳定，支持按模块增量迭代。

## 1. 后端模块总览（`api/internal/modules`）

| 模块 | 职责 | 关键能力 | 状态键（state key） | 关键数据 | 关键路由（示例） | 常见改动点 |
|---|---|---|---|---|---|---|
| `auth` | 用户认证、token 签发与刷新、会话撤销 | login/refresh/logout、trusted user 登录 | 无独立 state key（内存会话） | token、refresh session、trusted user | `/auth/login` `/auth/refresh` `/auth/logout` `/me` | token claim 字段、会话策略、角色校验 |
| `tenant` | 租户主数据 | 租户创建、状态管理 | `module_tenant` | tenants | `/tenants` `/tenants/{tenantID}/status` | 新租户属性、租户状态机 |
| `space` | 空间拓扑 | building/floor/area/door CRUD | `module_space` | buildings/floors/areas/doors | `/buildings` `/floors` `/areas` `/doors` | 新空间维度、筛选逻辑 |
| `access` | 权限策略域 | policies/user-groups/temporary-access | `module_access` | policies/users/groups/temporary-access/visitor-passes | `/access-policies` `/user-groups` `/temporary-access` | 新授权约束、模板映射 |
| `gateway` | 网关控制面域 | register/config/checkpoint/queue total | `module_gateway` | gateways、serial inventory、config states、event checkpoints、queue ingest totals | `/gateway/*` `/gateways/*` | checkpoint 规则、设备生命周期 |
| `event` | 事件域 | access/device ingest、幂等去重、查询 | `module_event` | access events、device events | `/gateway/events/access` `/gateway/events/device` `/events/*` | 事件字段扩展、幂等键策略 |
| `alarm` | 告警域 | 告警查询与状态流转 | `module_alarm` | alarms | `/alarms` `/alarms/{alarmID}/status` | 告警状态定义、运营字段 |
| `audit` | 审计域 | 审计日志写入与查询、Webhook fan-out | `module_audit` | audit logs、webhook config、deliveries | `/audit-logs` `/audit/webhook/*` | 审计 action 规范、订阅过滤 |
| `enterprise` | 企业接入域 | domain mapping、IdP、OIDC/SAML、目录同步、JIT、审批回写 | `module_enterprise` | domain mapping、idp config、employees、sync jobs、jit approvals | `/enterprise/*` | 身份映射、回调编排、补偿重试 |
| `wallet` | 卡券与队列运营 | 模板、发卡任务、重试、DLQ、告警分发 | `module_wallet` | config/templates/passes/jobs/dlq/metrics/alerts | `/wallet/*` | job 状态机、provider 适配 |
| `wallet/alertdispatch` | 告警分发编排子域 | 订阅评估、冷却、多通道分发 | 复用 wallet 状态 | 订阅评估、冷却、通道分发编排 | `/wallet/jobs/alerts/dispatch` | 通道策略、去重键、失败重试 |
| `wallet/googleclient` | Google 客户端适配 | issuer 探测、配置校验 | — | — | `/wallet/google/config/*` | 外部 API 交互细节 |

## 2. Router 层约束（`api/internal/http/router.go`）

Router 只做：

- 入参解析、鉴权、角色校验。
- 错误码映射与统一错误响应。
- 审计埋点与跨模块轻量编排。

复杂领域决策必须下沉到 `api/internal/modules/*`，避免"胖路由"。

不建议在 router 内做的事：

- 跨模块状态机编排过深。
- 复杂领域决策（应下沉 module service）。

## 3. 前端页面与后端域映射（`web-admin/src/pages`）

| 页面 | 文件 | 主要后端域 | 依赖 API 分组 |
|---|---|---|---|
| 登录 | `login-page.tsx` | auth | `auth/login` |
| 概览 | `dashboard-page.tsx` | tenant、gateway | `tenants` `gateways` |
| 租户列表/详情 | `tenants-page.tsx` `tenant-detail-page.tsx` | tenant、space | `tenants` `topology` |
| 空间管理 | `spaces-page.tsx` | space | `buildings/floors/areas/doors` |
| 权限管理 | `access-page.tsx` | access、enterprise | `access-policies` `user-groups` `temporary-access` |
| 网关管理 | `gateways-page.tsx` | gateway、event | `gateways/*` `gateway/*` |
| 事件查询 | `events-page.tsx` | event | `events/access` `events/device` |
| 告警管理 | `alarms-page.tsx` | alarm | `alarms` |
| Wallet 运营 | `wallet-page.tsx` | wallet | `wallet/jobs/*` |
| 企业管理 | `enterprise-page.tsx` | enterprise | `enterprise/*` |
| 审计页（已实现未挂路由） | `audit-page.tsx` | audit | `audit-logs` |

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

## 6. 模块改动 checklist（建议）

1. 修改 service 逻辑后补对应单测。
2. 修改 API 合同后补 `docs/testing/*.zsh` 回归脚本。
3. 变更能力状态后更新：
   - `docs/development-status-roadmap.md`
   - `docs/testing/admin-ui-test-and-api-map.md`
   - 必要时更新 `README.md`

## 7. 延伸阅读

- 当前高优先事项：`docs/wiki/internal/priority-board.md`
- 开发工作流：`docs/wiki/internal/dev-workflow.md`
- 系统总览：`docs/wiki/internal/system-overview.md`
