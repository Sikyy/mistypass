# Errors & Reliability（错误处理与重试）

当前能力状态：

- `CONTRACT_READY`：核心错误语义与重试动作已可预测。
- `PROD_READY`：关键冲突路径（checkpoint、retry_subset）已持续回归。

## 1. 统一错误格式

大多数错误响应遵循：

```json
{
  "error": "<message>"
}
```

部分场景会带附加诊断字段，例如 checkpoint 冲突会追加 `next_action`、`server_event_total`、`checkpoint`。

## 2. JSON 校验规则（全局）

所有 JSON 请求默认遵循：

1. 请求体最大约 `1 MiB`。
2. 必须是单个 JSON 对象。
3. 不允许未知字段（发送未定义字段会返回 `400`）。

## 3. 常见 HTTP 状态语义

| HTTP | 语义 | 客户端动作 |
|---|---|---|
| `400` | 入参格式/语义错误 | 修复请求，不直接重试 |
| `401` | 凭证缺失或失效 | 刷新/重新认证 |
| `403` | 角色或作用域不允许 | 切换账号或调整作用域 |
| `404` | 资源不存在 | 校验 ID 与租户范围 |
| `409` | 状态冲突 | 根据 `next_action` 修正后重试 |
| `429` | 限流/下游通道节流 | 指数退避重试，必要时走异步补偿 |
| `502` | 下游依赖失败（如 webhook 目标不可达） | 延迟重试并告警 |
| `500` | 服务端异常 | 指数退避重试 + 告警 |

## 4. 网关链路关键冲突

### 4.1 Checkpoint 回退冲突

响应关键字段：

- `error=event checkpoint acked_count regression`
- `next_action=retry_with_non_regressing_acked_count`
- `checkpoint.acked_count`（服务端最新值）

建议：基于返回的最新 `acked_count` 继续单调推进。

### 4.2 Checkpoint 上界冲突

响应关键字段：

- `error=event checkpoint acked_count exceeds server event total`
- `next_action=retry_with_server_event_total`
- `server_event_total`
- `server_total_source`（`queue_ingest_total` 或 `event_rows_fallback`）

建议：将本地 ack 调整到 `<= server_event_total` 后重发。

## 5. 幂等建议

### 5.1 事件上报

对于 `gateway/events/batch`：

- 建议每条事件显式提供 `idempotency_key`。
- 如不提供，服务端回退使用 `request_id`，再回退 `event_id`。

### 5.2 重试策略

- 仅对可恢复错误（`409/502/500`）做重试。
- 推荐指数退避：`1s -> 2s -> 4s -> ...`，上限 `60s`。
- 对设备链路建议设置最大重试次数与死信策略，避免阻塞主循环。

## 6. 运营可观测建议

1. 记录每次失败请求的 `request_id/event_id/checkpoint_id`。
2. 对 `409` 分类统计（回退冲突、上界冲突）。
3. 对 `502` 下游失败建立告警（Webhook/外部通道）。

## 7. 回归脚本参考

- `docs/testing/curl-gateway-event-checkpoint-partial.zsh`
- `docs/testing/curl-gateway-event-retry-subset-mixed.zsh`
- `docs/testing/curl-gateway-event-checkpoint-fallback-default.zsh`
- `docs/testing/curl-audit-webhook-fanout.zsh`

## 8. 相关文档

- `docs/wiki/external-api/changelog-and-migration.md`
- `docs/wiki/external-api/release-process-template.md`
- `docs/wiki/external-api/deprecation-policy.md`
- `docs/wiki/external-api/guides-rate-limit-and-429.md`
