# Guide：Wallet Queue Ops（队列运营与告警）

当前能力状态：

- `CONTRACT_READY`：队列处理、DLQ 治理、指标趋势、告警订阅与发送接口已稳定。
- `PROD_READY`：关键运营链路（dispatch/retry/resend/whatsapp/mock）有持续回归。

## 1. 目标

面向运营和集成侧，建立 Wallet 发卡任务从“排队 -> 处理 -> 监控 -> 告警 -> 补偿”的闭环。

## 2. 核心端点

任务处理：

- `POST /api/v1/wallet/passes/issue-batch`（`execution_mode=queued`）
- `POST /api/v1/wallet/jobs/process`
- `GET /api/v1/wallet/jobs`
- `GET /api/v1/wallet/jobs/summary`

指标与趋势：

- `GET /api/v1/wallet/jobs/metrics`
- `GET /api/v1/wallet/jobs/metrics/trend`

告警订阅与发送：

- `GET/PUT /api/v1/wallet/jobs/alert-subscription`
- `POST /api/v1/wallet/jobs/alerts/dispatch`
- `GET /api/v1/wallet/jobs/alert-notifications`
- `POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry`

DLQ 治理：

- `POST /api/v1/wallet/jobs/dlq/requeue`
- `POST /api/v1/wallet/jobs/dlq/cleanup`
- `GET /api/v1/wallet/jobs/dlq/cleanup/archives`

## 3. 推荐运营流程

### 3.1 任务入队

发卡时使用 `issue-batch` 且 `execution_mode=queued`，将任务落入后台队列。

### 3.2 Worker 处理

调用 `jobs/process`，可调参数：

- `limit`
- `worker_count`
- `max_retry`
- `base_backoff_ms`
- `max_backoff_ms`

### 3.3 指标巡检

定期读取：

- `jobs/summary`（状态分布）
- `jobs/metrics`（窗口统计 + 告警触发）
- `jobs/metrics/trend`（时间桶趋势）

### 3.4 告警策略

通过 `alert-subscription` 设置：

- `enabled`
- `dlq_alert_threshold`
- `window_seconds`
- `cooldown_seconds`
- `channels.email/whatsapp`
- `receiver_groups`

### 3.5 告警发送与重试

- 调 `alerts/dispatch` 执行阈值告警评估与发送。
- 若有失败通知，调 `alert-notifications/{id}/retry` 手动重试。

### 3.6 DLQ 补偿与清理

- `dlq/requeue`：按条件回补（支持 `error_code`、目标覆盖）。
- `dlq/cleanup`：按 `older_than_seconds` 清理并落归档。
- 归档记录可由 `dlq/cleanup/archives` 查询。

## 4. 关键语义

### 4.1 告警发送结果

`alerts/dispatch` 返回：

- `total_alerts/dispatched/skipped/failed`
- `items[].channel_results`（每个通道的 provider 结果）

### 4.2 告警冷却与幂等

- 冷却窗口内相同告警会被 `skipped`（`reason=cooldown`）。
- retry 路径带 `idempotency_key` 语义，避免重复成功发送。

### 4.3 默认阈值来源

若请求未显式覆盖，服务端按配置项回退（如 `WALLET_DLQ_ALERT_THRESHOLD`、`WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY`）。

## 5. 常见错误

| HTTP | 场景 |
|---|---|
| `400` | 参数非法（如 `window_seconds<=0`、`bucket_count` 越界） |
| `404` | 通知或任务不存在 |
| `409` | 不允许重试（如 notification 非 failed/retryable） |
| `500` | provider 或服务端内部异常 |

## 6. 回归脚本

- `docs/testing/curl-wallet-job-queue-process.zsh`
- `docs/testing/curl-wallet-job-metrics-alert.zsh`
- `docs/testing/curl-wallet-job-metrics-trend.zsh`
- `docs/testing/curl-wallet-job-alert-subscription.zsh`
- `docs/testing/curl-wallet-job-alert-dispatch.zsh`
- `docs/testing/curl-wallet-job-alert-dispatch-retry.zsh`
- `docs/testing/curl-wallet-job-dlq-governance.zsh`

## 7. 相关 Reference

- `docs/wiki/external-api/reference-wallet-jobs-process-notifications.md`
- `docs/wiki/external-api/reference-wallet-job-metrics.md`
- `docs/wiki/external-api/reference-wallet-alert-subscription.md`
- `docs/wiki/external-api/reference-wallet-alert-dispatch.md`
- `docs/wiki/external-api/reference-wallet-dlq-governance.md`
