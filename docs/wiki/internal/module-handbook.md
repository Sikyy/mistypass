# 模块手册（Backend + Frontend）

当前能力状态：

- `PROD_READY`：模块职责与当前实现一致，可直接用于开发分工。
- `CONTRACT_READY`：关键接口分组稳定，支持按模块增量迭代。

## 1. 后端模块地图（`api/internal/modules`）

| 模块 | 职责 | 关键能力 | 关键路由（示例） | 常见改动点 |
|---|---|---|---|---|
| `auth` | 用户认证、token 签发与刷新、会话撤销 | login/refresh/logout、trusted user 登录 | `/auth/login` `/auth/refresh` `/auth/logout` | token claim 字段、会话策略、角色校验 |
| `tenant` | 租户主数据 | 租户创建、状态管理 | `/tenants` `/tenants/{tenantID}/status` | 新租户属性、租户状态机 |
| `space` | 空间拓扑 | building/floor/area/door CRUD | `/buildings` `/floors` `/areas` `/doors` | 新空间维度、筛选逻辑 |
| `access` | 权限策略域 | policies/user-groups/temporary-access | `/access-policies` `/user-groups` `/temporary-access` | 新授权约束、模板映射 |
| `gateway` | 网关控制面域 | register/config/checkpoint/queue total | `/gateway/*` `/gateways/*` | checkpoint 规则、设备生命周期 |
| `event` | 事件域 | access/device ingest、幂等去重、查询 | `/gateway/events/access` `/gateway/events/device` `/events/*` | 事件字段扩展、幂等键策略 |
| `alarm` | 告警域 | 告警查询与状态流转 | `/alarms` `/alarms/{alarmID}/status` | 告警状态定义、运营字段 |
| `audit` | 审计域 | 审计日志写入与查询、Webhook fan-out | `/audit-logs` `/audit/webhook/*` | 审计 action 规范、订阅过滤 |
| `enterprise` | 企业接入域 | domain mapping、IdP、OIDC/SAML、目录同步、JIT、审批回写 | `/enterprise/*` | 身份映射、回调编排、补偿重试 |
| `wallet` | 卡券与队列运营 | 模板、发卡任务、重试、DLQ、告警分发 | `/wallet/*` | job 状态机、provider 适配 |
| `wallet/alertdispatch` | 告警分发编排子域 | 订阅评估、冷却、多通道分发 | `/wallet/jobs/alerts/dispatch` | 通道策略、去重键、失败重试 |
| `wallet/googleclient` | Google 客户端适配 | issuer 探测、配置校验 | `/wallet/google/config/*` | 外部 API 交互细节 |

## 2. Router 层分工（`api/internal/http/router.go`）

- 负责事项：
  - HTTP 入参解析与校验。
  - 鉴权（Bearer、Device Token）与角色检查。
  - 统一错误码映射（400/401/403/404/409/500）。
  - 审计事件拼接与写入。
- 不建议在 router 内做的事：
  - 跨模块状态机编排过深。
  - 复杂领域决策（应下沉 module service）。

## 3. 前端页面地图（`web-admin/src/pages`）

| 页面 | 主要功能 | 依赖 API 分组 |
|---|---|---|
| `login-page.tsx` | 登录与 token 落地 | `auth/login` |
| `dashboard-page.tsx` | 全局概览 | `tenants` `gateways` |
| `tenants-page.tsx` / `tenant-detail-page.tsx` | 租户管理与拓扑查看 | `tenants` `topology` |
| `spaces-page.tsx` | 空间资产管理 | `buildings/floors/areas/doors` |
| `access-page.tsx` | 策略、用户组、临时权限 | `access-policies` `user-groups` `temporary-access` |
| `gateways-page.tsx` | 网关注册、绑定、配置、库存 | `gateways/*` `gateway/*` |
| `events-page.tsx` | 访问/设备事件查询 | `events/access` `events/device` |
| `alarms-page.tsx` | 告警查询与处理 | `alarms` |
| `wallet-page.tsx` | Wallet 运营视图 | `wallet/jobs/*` |
| `audit-page.tsx` | 审计查询（已实现，路由待显式挂载） | `audit-logs` |

## 4. 模块改动 checklist（建议）

1. 修改 service 逻辑后补对应单测。
2. 修改 API 合同后补 `docs/testing/*.zsh` 回归脚本。
3. 变更能力状态后更新：
   - `docs/development-status-roadmap.md`
   - `docs/testing/admin-ui-test-and-api-map.md`
   - 必要时更新 `README.md`

## 5. 延伸阅读

- 模块细节与测试入口：`docs/wiki/internal/module-deep-dive.md`
- 当前高优先事项：`docs/wiki/internal/priority-board.md`
