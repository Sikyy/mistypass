# 对外文档信息架构（Kisi 风格参考版）

当前能力状态：

- `CONTRACT_READY`：信息架构已从模板升级为“目录 + 首批实页”并可持续扩充。
- `PROD_READY`：首批页面与当前 API 合同保持一致。

## 1. 顶层目录建议

1. Getting Started
- 认证方式
- 最小调用闭环
- 环境与限制

2. Core Concepts
- Tenant / Building / Gateway / Policy / User
- Queue / Checkpoint / Replay
- Idempotency / Eventual Consistency

3. Guides（按任务）
- 网关注册与离线上报
- 企业 SSO 与目录同步
- 审计 Webhook 订阅与投递
- Wallet 运营任务

4. API Reference（按资源）
- Auth
- Tenant & Space
- Gateway & Event
- Access
- Enterprise
- Wallet
- Audit / Webhook

5. Errors & Reliability
- 错误码
- 重试策略
- 幂等建议
- 限流与超时

6. Changelog
- 版本变更
- 迁移说明
- 弃用计划

## 2. 已落地页面（截至 2026-04-15）

- Getting Started：
  - `docs/wiki/external-api/getting-started.md`
- Common：
  - `docs/wiki/external-api/authentication.md`
  - `docs/wiki/external-api/errors-and-reliability.md`
  - `docs/wiki/external-api/changelog-and-migration.md`
  - `docs/wiki/external-api/release-process-template.md`
  - `docs/wiki/external-api/deprecation-policy.md`
- Guides：
  - `docs/wiki/external-api/guides-api-token-scope-troubleshooting.md`
  - `docs/wiki/external-api/guides-gateway-offline-workflow.md`
  - `docs/wiki/external-api/guides-audit-webhook.md`
  - `docs/wiki/external-api/guides-enterprise-sso-jit.md`
  - `docs/wiki/external-api/guides-enterprise-jit-approval-external-sync.md`
  - `docs/wiki/external-api/guides-wallet-queue-ops.md`
  - `docs/wiki/external-api/guides-wallet-dlq-governance.md`
  - `docs/wiki/external-api/guides-rate-limit-and-429.md`
- Reference：
  - `docs/wiki/external-api/reference-auth-session-me.md`
  - `docs/wiki/external-api/reference-tenant-space-core.md`
  - `docs/wiki/external-api/reference-access-core.md`
  - `docs/wiki/external-api/reference-gateway-events-batch.md`
  - `docs/wiki/external-api/reference-gateway-serial-inventory-checkpoint-summary.md`
  - `docs/wiki/external-api/reference-enterprise-auth-start-exchange.md`
  - `docs/wiki/external-api/reference-enterprise-tenant-domain.md`
  - `docs/wiki/external-api/reference-enterprise-idp-config.md`
  - `docs/wiki/external-api/reference-enterprise-employees-sync-jobs.md`
  - `docs/wiki/external-api/reference-enterprise-jit-approval-external-sync.md`
  - `docs/wiki/external-api/reference-enterprise-sync-requests-reconcile.md`
  - `docs/wiki/external-api/reference-enterprise-sync-worker-alerts.md`
  - `docs/wiki/external-api/reference-wallet-alert-dispatch.md`
  - `docs/wiki/external-api/reference-wallet-alert-subscription.md`
  - `docs/wiki/external-api/reference-wallet-dlq-governance.md`
  - `docs/wiki/external-api/reference-wallet-job-metrics.md`
  - `docs/wiki/external-api/reference-wallet-jobs-process-notifications.md`

## 3. 页面粒度规范

- Guide 页面回答“怎么完成任务”。
- Reference 页面回答“每个接口怎么调用”。
- 每个接口页固定包含：
  - Endpoint + Method
  - Auth requirement
  - Request schema
  - Response schema
  - Error cases
  - 示例请求/响应

## 4. 与当前 API 的映射

- Gateway/Edge 合同链路：`/api/v1/gateway/*`、`/api/v1/gateways/*`
- 企业接入：`/api/v1/enterprise/*`
- Wallet：`/api/v1/wallet/*`
- 审计与对外投递：`/api/v1/audit/*`

## 5. 下一批建议（优先级）

1. 补齐 `webhook` 安全章节（签名校验、重放保护、时钟漂移容忍）。
2. 增加“按角色/租户/楼宇”的常见越权审计案例库。
