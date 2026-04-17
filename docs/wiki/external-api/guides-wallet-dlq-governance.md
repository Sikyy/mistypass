# Guide：Wallet DLQ 治理

当前能力状态：

- `CONTRACT_READY`：DLQ 批量回补、批量清理、清理归档查询接口已稳定。
- `PROD_READY`：治理链路（requeue/cleanup/archives）具备回归脚本与管理台联动验证。

## 1. 目标

面向运营侧，建立 Wallet 任务失败后的可控治理流程：快速回补、定向清理、可追溯留档。

## 2. 关键端点

- `POST /api/v1/wallet/jobs/dlq/requeue`
- `POST /api/v1/wallet/jobs/dlq/cleanup`
- `GET /api/v1/wallet/jobs/dlq/cleanup/archives`

## 3. 推荐治理流程

### 3.1 先看当前积压

结合：

- `GET /api/v1/wallet/jobs/summary`
- `GET /api/v1/wallet/jobs/metrics`

先确认 `dlq` 总量与主要 `error_code`。

### 3.2 批量回补（优先）

调用 `dlq/requeue`：

- 可按 `error_code` 定向回补。
- 可用 `target_id_override` 修正目标主体后重投。

适用场景：模板恢复、短时外部依赖故障恢复后再处理。

### 3.3 历史清理（兜底）

调用 `dlq/cleanup`：

- 使用 `older_than_seconds` 限定时间窗口。
- 可叠加 `error_code` 做定向清理。

清理结果自动归档，可用 `cleanup/archives` 拉取审计证据。

## 4. 参数建议

- `requeue.limit`：单次建议 20~200，避免一次处理过大。
- `cleanup.older_than_seconds`：建议从 `86400`（24h）起步。
- `cleanup.limit`：按租户规模设定，一般 50~500。

## 5. 常见错误

| HTTP | 场景 |
|---|---|
| `400` | `tenant_id` 缺失或参数越界（如超限） |
| `403` | `tenant scope forbidden` |
| `500` | 服务内部错误或状态持久化失败 |

## 6. 回归脚本

- `docs/testing/curl-wallet-job-dlq-governance.zsh`
- `docs/testing/curl-wallet-job-dlq-requeue.zsh`

## 7. 相关 Reference

- `docs/wiki/external-api/reference-wallet-dlq-governance.md`
- `docs/wiki/external-api/reference-wallet-jobs-process-notifications.md`
- `docs/wiki/external-api/reference-wallet-job-metrics.md`
