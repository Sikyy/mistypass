# MistyPass 项目 Wiki（内部 + 外部）

当前能力状态：

- `CONTRACT_READY`：本目录已提供内部开发 Wiki 与对外 API 文档（骨架 + 首批实页），可直接作为团队协作与对接基线。
- `PROD_READY`：已对齐当前主干代码与回归口径（`go test ./...` + `docs/testing/*.zsh`），并可纳入持续维护。

## 1. 使用方式

- 内部研发同学优先看：`docs/wiki/internal/README.md`
- 面向 API 调用开发者/合作企业文档优先看：`docs/wiki/external-api/README.md`

快速入口（新增）：

- 当前优先级事项看板：`docs/wiki/internal/priority-board.md`
- 模块深描（职责/状态/测试入口）：`docs/wiki/internal/module-deep-dive.md`
- 对外 Quick Start：`docs/wiki/external-api/getting-started.md`
- 对外 Enterprise SSO/JIT Guide：`docs/wiki/external-api/guides-enterprise-sso-jit.md`
- 对外 Enterprise JIT 审批回写 Guide：`docs/wiki/external-api/guides-enterprise-jit-approval-external-sync.md`
- 对外 Wallet Queue Ops Guide：`docs/wiki/external-api/guides-wallet-queue-ops.md`
- 对外 Wallet DLQ 治理 Guide：`docs/wiki/external-api/guides-wallet-dlq-governance.md`
- 对外 Enterprise Sync Reconcile Reference：`docs/wiki/external-api/reference-enterprise-sync-requests-reconcile.md`
- 对外 Enterprise Domain/IdP/Employees Reference：`docs/wiki/external-api/reference-enterprise-tenant-domain.md`、`docs/wiki/external-api/reference-enterprise-idp-config.md`、`docs/wiki/external-api/reference-enterprise-employees-sync-jobs.md`
- 对外 Wallet Metrics / Subscription Reference：`docs/wiki/external-api/reference-wallet-job-metrics.md`、`docs/wiki/external-api/reference-wallet-alert-subscription.md`
- 对外 Wallet Jobs/Notifications Reference：`docs/wiki/external-api/reference-wallet-jobs-process-notifications.md`
- 对外 Gateway Serial/Checkpoint Reference：`docs/wiki/external-api/reference-gateway-serial-inventory-checkpoint-summary.md`
- 对外 Changelog/Migration：`docs/wiki/external-api/changelog-and-migration.md`
- 对外 Release/Deprecation：`docs/wiki/external-api/release-process-template.md`、`docs/wiki/external-api/deprecation-policy.md`
- 对外 Token/Scope 与 429 指南：`docs/wiki/external-api/guides-api-token-scope-troubleshooting.md`、`docs/wiki/external-api/guides-rate-limit-and-429.md`
- 对外 Auth / Tenant-Space / Access Reference：`docs/wiki/external-api/reference-auth-session-me.md`、`docs/wiki/external-api/reference-tenant-space-core.md`、`docs/wiki/external-api/reference-access-core.md`

## 2. 文档边界

- 本目录不替代现有专题文档（`docs/enterprise/*`、`docs/testing/*`、`docs/architecture/*`）。
- 本目录负责“索引 + 可执行说明 + 模板化规范”，让新同学能快速定位到正确文档与代码。

## 3. 维护约定

- 代码结构或接口分组有新增时，同步更新本目录对应页。
- 对外 API 文档发布前，先走内部评审，再从 `external-api` 骨架生成正式发布版本。
