# Guide：Rate Limit & 429（限流与退避实战）

当前能力状态：

- `CONTRACT_READY`：对外错误语义与重试建议已稳定，支持统一客户端退避策略。
- `PROD_READY`：Wallet 告警外发链路可识别 provider `429/5xx` 为可重试并在通知记录中留档。

## 1. 先明确当前行为（截至 2026-04-15）

1. MistyPass 主 API 当前没有统一入口级 `429` 限流中间件。
2. 管理端常见重试状态仍以 `409/500/502` 为主。
3. `429` 目前最常见于 Wallet 告警外发 provider（如 email/whatsapp 通道）返回，系统会记录为通知失败并标记 `retryable=true`。

## 2. 客户端统一退避策略

建议把错误分为三类：

- 不可重试：`400/401/403/404`
- 可纠正后重试：`409`（按 `next_action` 修正）
- 可直接退避重试：`429/500/502`

推荐退避（带抖动）：

1. 第 1 次：`1s + jitter(0~300ms)`
2. 第 2 次：`2s + jitter(0~600ms)`
3. 第 3 次：`4s + jitter(0~1200ms)`
4. 上限：`60s`
5. 达到最大重试次数后进入人工补偿或异步队列

## 3. Wallet 告警链路里的 429 处理

### 3.1 现象

调用 `POST /api/v1/wallet/jobs/alerts/dispatch` 时，HTTP 可能仍是 `200`，但 `items[].channel_results[]` 中出现：

- `status=failed`
- `reason=provider_transient_error`
- `retryable=true`
- `provider_error` 包含 provider 的 `429/5xx` 信息

### 3.2 建议动作

1. 先查询 `GET /api/v1/wallet/jobs/alert-notifications` 定位失败通知。
2. 对 `retryable=true` 的记录执行：
   - `POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry`
3. 若重复失败，延长重试间隔并检查 provider 配额与凭证。

## 4. 监控与告警建议

最小指标集：

- 每分钟 `5xx + 502` 比例
- `409` 分类计数（checkpoint regression / exceeds total）
- Wallet 告警通知 `failed + retryable` 数量
- 同一 `idempotency_key` 的重复发送尝试次数

建议阈值（起步）：

- `5xx+502` 比例连续 5 分钟 > 2%
- `retryable failed notifications` 连续 10 分钟 > 20
- 同一租户连续 3 次 retry 仍失败

## 5. 排障清单

1. 先区分是“平台 API 错误”还是“下游 provider 限流”。
2. 平台 API：看 HTTP 状态码与 `error` 字段。
3. Wallet provider：看 `channel_results[].provider_error/retryable`。
4. 对 `409` 按响应诊断字段修正后再发，不要盲重试。
5. 对 `429/500/502` 使用指数退避，不做无限重放。

## 6. 相关文档

- `docs/wiki/external-api/errors-and-reliability.md`
- `docs/wiki/external-api/reference-wallet-alert-dispatch.md`
- `docs/wiki/external-api/reference-wallet-jobs-process-notifications.md`
- `docs/wiki/external-api/changelog-and-migration.md`
