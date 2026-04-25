# 系统总览与组件关系

当前能力状态：

- `PROD_READY`：Cloud 主链路（网关事件、回放、企业认证、Wallet 队列）具备稳定回归闭环。
- `CONTRACT_READY`：跨模块接口契约已稳定，可用于新功能增量开发。

## 1. 项目结构（代码视角）

- `api/`：Go 后端，Chi Router + 模块化服务。
- `web-admin/`：React 管理台（租户、空间、权限、网关、事件、告警、Wallet）。
- `docs/testing/`：回归脚本与验证口径。
- `.github/workflows/`：CI smoke + nightly。

## 2. 运行时结构（服务视角）

1. API 入口：`api/cmd/api/main.go`
- 读取 `config.FromEnv()`。
- 按需接入 PostgreSQL `state.Store`。
- 初始化 Router 并暴露 `:PORT`。

2. Router 层：`api/internal/http/router.go`
- 承载所有 `api/v1` 路由。
- 做鉴权、参数校验、错误码映射、审计写入。
- 调用下层模块服务，不直接承载复杂领域状态。

3. 模块服务层：`api/internal/modules/*`
- 每个模块维护自己的领域模型与状态读写。
- 通过 `state.Store` 做快照持久化（启用 DB 时）。
- 保持模块边界清晰，跨模块通过 router 组装。

4. 状态层：`api/internal/state`
- 统一快照表 `mistypass`。
- 投影表 `mistypass_*`。
- 增量变更日志 + checkpoint replay。

## 3. 主要业务子系统

- 认证授权：`auth` + `access`。
- 企业接入：`enterprise`（OIDC/SAML、目录同步、JIT、审批回写）。
- 网关合同链路：`gateway` + `event`（batch/retry_subset/checkpoint）。
- Wallet 运营链路：`wallet` + `wallet/alertdispatch`。
- 运维可见性：`audit` + `alarm` + 回归脚本 + CI。

## 4. Cloud / Edge 边界（开发必读）

- Cloud 负责策略编排、多租户、审计、控制面 API。
- Edge 负责门侧实时判定、本地缓存、离线执行。
- 强约束：开门链路不能依赖云端实时往返。

## 5. 入口索引

- 总体排期：`docs/development-status-roadmap.md`
- API/UI 映射：`docs/testing/admin-ui-test-and-api-map.md`
- 迁移决策：`docs/architecture/encore-decision-report-2026-04-19.md`
