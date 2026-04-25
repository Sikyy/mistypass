# Guide：网关离线链路（Register -> Batch -> Checkpoint）

当前能力状态：

- `CONTRACT_READY`：网关 bootstrap 合同链路字段已稳定。
- `PROD_READY`：`retry_subset + queue_hint + checkpoint` 路径有持续回归覆盖。

## 1. 适用场景

适用于设备侧离线队列上报：

1. 管理端先导入序列号库存。
2. 设备调用 bootstrap 注册拿到 `device_token`。
3. 设备周期拉配置。
4. 设备批量上报事件并处理 `retry_subset`。
5. 设备上报 checkpoint 保持单调推进。

## 2. 认证方式

管理端接口：

- `Authorization: Bearer <access_token>`。

设备 bootstrap 接口：

- 推荐 `X-Device-Token: <device_token>`。
- 兼容 `Authorization: Bearer <device_token>`。

## 3. 时序步骤

### 3.1 序列号入库（管理端）

`POST /api/v1/gateways/serial-inventory/import`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "items": [
    {
      "serial_number": "MP-GW-EDGE-001",
      "product_type": "gateway",
      "batch_code": "factory-202604",
      "source": "factory"
    }
  ]
}
```

### 3.2 bootstrap 注册（设备）

`POST /api/v1/gateway/register`

```json
{
  "serial_number": "MP-GW-EDGE-001",
  "tenant_id": "tenant_demo_jakarta",
  "building_id": "building_demo_001",
  "device_capacity": 4
}
```

请求头要求：

- `X-Bootstrap-Token: <bootstrap_token>`
- `bootstrap_token` 来自服务端配置 `GATEWAY_BOOTSTRAP_TOKEN`

关键响应字段：

- `gateway_id`
- `device_token`

### 3.3 配置拉取与回执（设备）

拉取：`POST /api/v1/gateway/config/pull`

- 输入：`gateway_id/tenant_id/current_version/authz_cache_version(optional)`。
- 关注返回：
  - `desired_version`
  - `should_apply`
  - `authz_cache.version`
  - `authz_cache.status`（`AUTHZ_CACHE_MISSING/FRESH/STALE/DRIFT`）

回执：`POST /api/v1/gateway/config/applied`

- 输入：`gateway_id/tenant_id/version/authz_cache_version`。
- 关注返回：`in_sync` 与 `authz_cache.version_match`。

### 3.4 批量事件上报（设备）

`POST /api/v1/gateway/events/batch`

关键点：

- 每批最多 `200` 条（`access_events + device_events` 总和）。
- `queue` 可选，缺省为 `default`。
- 单条事件幂等键回退顺序：`idempotency_key -> request_id -> event_id`。

响应关键字段：

- `status`：`accepted` 或 `partial`。
- `retry_subset`：仅包含可重试失败项。
- `queue_hint`：
  - `checkpoint_id`
  - `acked_increment`
  - `server_ingested_total`
  - `status_code`
  - `next_action`

`queue_hint.status_code` 语义：

- `QUEUE_READY_TO_CHECKPOINT`
- `QUEUE_RETRY_SUBSET_REQUIRED`
- `QUEUE_PARTIAL_NON_RETRYABLE`

### 3.5 重放 retry_subset（设备）

当 `retry_subset` 非空时，建议直接将该对象作为下一次 `events/batch` 请求体重放，直到失败项归零。

### 3.6 checkpoint 上报（设备）

`POST /api/v1/gateway/events/checkpoint`

```json
{
  "gateway_id": "gw_demo_001",
  "tenant_id": "tenant_demo_jakarta",
  "queue": "default",
  "checkpoint_id": "gw_demo_001-default-1744700000000",
  "last_request_id": "rq-1744700000",
  "acked_count": 128,
  "last_occurred_at": "2026-04-15T08:00:00Z"
}
```

返回 `200` 时，`acked_count` 已被服务端接受。

## 4. 409 冲突处理（必须实现）

### 4.1 ack 回退冲突

响应特征：

- `error = event checkpoint acked_count regression`
- `next_action = retry_with_non_regressing_acked_count`
- `checkpoint.acked_count` 返回服务端最新值

处理建议：

1. 使用返回的最新 checkpoint 作为基线。
2. 仅向前推进，不回退计数。

### 4.2 ack 超上界冲突

响应特征：

- `error = event checkpoint acked_count exceeds server event total`
- `next_action = retry_with_server_event_total`
- `server_event_total`
- `server_total_source`：`queue_ingest_total` 或 `event_rows_fallback`

处理建议：

1. 将本地 ack 修正到 `<= server_event_total`。
2. 重新发送 checkpoint。

## 5. 建议执行顺序（设备侧）

1. 先发 `events/batch`。
2. 如有 `retry_subset`，先重试至收敛。
3. 再报 `events/checkpoint`。

## 6. 验证脚本

- `docs/testing/curl-gateway-config-pull-apply.zsh`
- `docs/testing/curl-gateway-event-retry-subset-mixed.zsh`
- `docs/testing/curl-gateway-event-checkpoint-partial.zsh`
- `docs/testing/curl-gateway-event-multi-queue-isolation.zsh`
- `docs/testing/curl-gateway-event-checkpoint-fallback-default.zsh`

## 7. 相关 Reference

- `docs/wiki/external-api/reference-gateway-events-batch.md`
- `docs/wiki/external-api/reference-gateway-serial-inventory-checkpoint-summary.md`
- `docs/wiki/external-api/errors-and-reliability.md`
