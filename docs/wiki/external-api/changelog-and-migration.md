# Changelog & Migration（对外 API）

当前能力状态：

- `CONTRACT_READY`：已建立面向集成方的变更记录与迁移指引基线。
- `PROD_READY`：首批变更记录已覆盖当前已发布的 Guide/Reference 页面。

## 1. 版本口径

当前文档版本建议采用“日期批次”标识（例如 `2026-04-15-r10`），用于对外沟通与变更追踪。

变更类型定义：

- `PATCH`：仅补充说明或示例，不影响现有调用。
- `MINOR`：新增可选字段/新端点，保持向后兼容。
- `MAJOR`：删除字段、收紧校验、变更行为默认值（需提前公告）。

## 2. 最近变更

### 2026-04-15-r10（`MINOR`）

- 新增 token/scope 实操与排障指南：
  - `docs/wiki/external-api/guides-api-token-scope-troubleshooting.md`
- 新增限流与 `429` 退避实战指南：
  - `docs/wiki/external-api/guides-rate-limit-and-429.md`
- 同步更新 external README 与信息架构优先级建议。

### 2026-04-15-r9（`MINOR`）

- 新增发布流程模板：
  - `docs/wiki/external-api/release-process-template.md`
- 新增弃用策略页：
  - `docs/wiki/external-api/deprecation-policy.md`
- 完成对外文档目录与信息架构互链更新。

### 2026-04-15-r8（`MINOR`）

- 新增 Wallet 资源参考：
  - `docs/wiki/external-api/reference-wallet-jobs-process-notifications.md`
- 新增 Gateway 资源参考：
  - `docs/wiki/external-api/reference-gateway-serial-inventory-checkpoint-summary.md`
- Guide/Reference 增加互链，降低跨页查找成本。

### 2026-04-15-r7（`MINOR`）

- 新增 Enterprise 资源参考：
  - `docs/wiki/external-api/reference-enterprise-tenant-domain.md`
  - `docs/wiki/external-api/reference-enterprise-idp-config.md`
  - `docs/wiki/external-api/reference-enterprise-employees-sync-jobs.md`

### 2026-04-15-r6（`MINOR`）

- 新增基础资源参考：
  - `docs/wiki/external-api/reference-auth-session-me.md`
  - `docs/wiki/external-api/reference-tenant-space-core.md`
  - `docs/wiki/external-api/reference-access-core.md`

### 2026-04-15-r5（`MINOR`）

- 新增运营资源参考：
  - `docs/wiki/external-api/reference-enterprise-sync-requests-reconcile.md`
  - `docs/wiki/external-api/reference-wallet-job-metrics.md`
  - `docs/wiki/external-api/reference-wallet-alert-subscription.md`

### 2026-04-15-r4（`MINOR`）

- 新增专题 Guide/Reference：
  - Enterprise JIT 审批回写
  - Enterprise Sync Worker 告警
  - Wallet DLQ 治理

### 2026-04-15-r3（`MINOR`）

- 新增首批对外实页：
  - Getting Started / Authentication / Errors & Reliability
  - Gateway Offline Workflow / Audit Webhook
  - Gateway Events Batch Reference

## 3. 迁移策略（给集成方）

1. 保持请求体只发送已文档化字段（服务端默认拒绝未知字段）。
2. 对 `409` 冲突按 `next_action` 实现自动修正重试。
3. 对 `500/502` 使用指数退避重试，不做无限重放。
4. 对关键端点建立最小合同测试：
   - 网关：`events/batch + events/checkpoint`
   - 企业：`auth/start + auth/exchange`
   - Wallet：`jobs/process + alerts/dispatch`
5. 每次升级前先检查本页“最近变更”并回归核心脚本。

## 4. 兼容性声明（截至 2026-04-15）

- 当前记录范围内无已发布的破坏性变更（`MAJOR`）。
- 如出现 `MAJOR` 变更，将在本页提前列出：
  - 受影响端点
  - 迁移窗口
  - 双轨兼容截止日期

## 5. 回归脚本入口

- `docs/testing/curl-gateway-event-checkpoint-partial.zsh`
- `docs/testing/curl-enterprise-sync-access-batch.zsh`
- `docs/testing/curl-wallet-job-queue-process.zsh`
- `docs/testing/curl-wallet-job-alert-dispatch-retry.zsh`

## 6. 相关文档

- `docs/wiki/external-api/guides-api-token-scope-troubleshooting.md`
- `docs/wiki/external-api/guides-rate-limit-and-429.md`
- `docs/wiki/external-api/release-process-template.md`
- `docs/wiki/external-api/deprecation-policy.md`
