# 对外 API 文档（开发者 / 合作企业）

当前能力状态：

- `CONTRACT_READY`：已形成“快速开始 + 任务指南 + 参考页 + 错误可靠性”的首批可交付文档。
- `PROD_READY`：关键页面已对齐当前主干 API 与回归脚本，能够支持 Sandbox 联调。

## 1. 目标受众

- 集成开发者（ISV/合作方研发）
- 企业 IT / 实施工程师
- 内部解决方案与技术支持

## 2. 推荐阅读顺序

1. `docs/wiki/external-api/getting-started.md`
2. `docs/wiki/external-api/authentication.md`
3. `docs/wiki/external-api/guides-api-token-scope-troubleshooting.md`
4. `docs/wiki/external-api/guides-gateway-offline-workflow.md`
5. `docs/wiki/external-api/errors-and-reliability.md`
6. `docs/wiki/external-api/guides-rate-limit-and-429.md`
7. `docs/wiki/external-api/changelog-and-migration.md`
8. `docs/wiki/external-api/release-process-template.md`
9. `docs/wiki/external-api/deprecation-policy.md`

## 3. 已发布页面

- 起步与公共能力：
  - `docs/wiki/external-api/getting-started.md`
  - `docs/wiki/external-api/authentication.md`
  - `docs/wiki/external-api/errors-and-reliability.md`
  - `docs/wiki/external-api/changelog-and-migration.md`
  - `docs/wiki/external-api/release-process-template.md`
  - `docs/wiki/external-api/deprecation-policy.md`
- 任务指南：
  - `docs/wiki/external-api/guides-api-token-scope-troubleshooting.md`
  - `docs/wiki/external-api/guides-gateway-offline-workflow.md`
  - `docs/wiki/external-api/guides-audit-webhook.md`
  - `docs/wiki/external-api/guides-enterprise-sso-jit.md`
  - `docs/wiki/external-api/guides-enterprise-jit-approval-external-sync.md`
  - `docs/wiki/external-api/guides-wallet-queue-ops.md`
  - `docs/wiki/external-api/guides-wallet-dlq-governance.md`
  - `docs/wiki/external-api/guides-rate-limit-and-429.md`
- 参考页示例：
  - `docs/wiki/external-api/reference-auth-session-me.md`
  - `docs/wiki/external-api/reference-tenant-space-core.md`
  - `docs/wiki/external-api/reference-access-core.md`
  - `docs/wiki/external-api/reference-gateway-events-batch.md`
  - `docs/wiki/external-api/reference-gateway-serial-inventory-checkpoint-summary.md`
  - `docs/wiki/external-api/reference-enterprise-auth-start-exchange.md`
  - `docs/wiki/external-api/reference-enterprise-tenant-domain.md`
  - `docs/wiki/external-api/reference-enterprise-idp-config.md`
  - `docs/wiki/external-api/reference-enterprise-employees-sync-jobs.md`
  - `docs/wiki/external-api/reference-enterprise-hris-secrets.md`
  - `docs/wiki/external-api/reference-enterprise-hris-webhooks.md`
  - `docs/wiki/external-api/reference-enterprise-hris-webhook-dlq.md`
  - `docs/wiki/external-api/reference-enterprise-hris-pull-states.md`
  - `docs/wiki/external-api/reference-enterprise-jit-approval-external-sync.md`
  - `docs/wiki/external-api/reference-enterprise-sync-requests-reconcile.md`
  - `docs/wiki/external-api/reference-enterprise-sync-worker-alerts.md`
  - `docs/wiki/external-api/reference-wallet-alert-dispatch.md`
  - `docs/wiki/external-api/reference-wallet-alert-subscription.md`
  - `docs/wiki/external-api/reference-wallet-dlq-governance.md`
  - `docs/wiki/external-api/reference-wallet-job-metrics.md`
  - `docs/wiki/external-api/reference-wallet-jobs-process-notifications.md`

## 4. 模板页（继续扩展用）

- 信息架构：`docs/wiki/external-api/information-architecture.md`
- Quick Start 模板：`docs/wiki/external-api/quickstart-template.md`
- Reference 模板：`docs/wiki/external-api/reference-page-template.md`
- Release 流程模板：`docs/wiki/external-api/release-process-template.md`

## 5. 与 Kisi 风格对齐点（参考）

- 先任务路径，再资源型 Reference。
- 公共章节统一收敛认证、错误、重试、幂等。
- 每条主流程都给最小可跑通步骤与常见失败处理。
- 参考文档站：https://docs.kisi.io/
