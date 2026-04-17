# API Reference：`POST /api/v1/gateway/events/batch`

当前能力状态：

- `CONTRACT_READY`：请求/响应字段已稳定。
- `PROD_READY`：路由级与脚本级回归已覆盖主分支行为。

## 1. Endpoint

- Method：`POST`
- Path：`/api/v1/gateway/events/batch`
- Auth：`X-Device-Token`（推荐）或 `Authorization: Bearer <device_token>`

## 2. Request Schema

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `gateway_id` | string | 是 | 网关 ID |
| `tenant_id` | string | 是 | 租户 ID |
| `queue` | string | 否 | 队列名，缺省 `default` |
| `access_events` | array | 否 | 访问事件列表 |
| `device_events` | array | 否 | 设备事件列表 |

约束：

- `access_events` 与 `device_events` 不能同时为空。
- 两者总条数不得超过 `200`。

### 2.1 `access_events[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `event_id` | string | 否 | 事件 ID（建议传） |
| `request_id` | string | 否 | 请求 ID（建议传） |
| `idempotency_key` | string | 否 | 幂等键（优先） |
| `building_id` | string | 否 | 楼宇 ID（缺省回退网关楼宇） |
| `area_id` | string | 否 | 区域 ID |
| `type` | string | 否 | 缺省 `access_event` |
| `actor` | string | 否 | 触发主体 |
| `door_id` | string | 否 | 门 ID |
| `result` | string | 否 | 缺省 `accepted` |
| `occurred_at` | string | 是 | RFC3339 时间 |

### 2.2 `device_events[]`

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `event_id` | string | 否 | 事件 ID |
| `request_id` | string | 否 | 请求 ID |
| `idempotency_key` | string | 否 | 幂等键（优先） |
| `building_id` | string | 否 | 楼宇 ID（缺省回退网关楼宇） |
| `type` | string | 否 | 缺省 `gateway_event` |
| `detail` | string | 否 | 事件详情 |
| `result` | string | 否 | 缺省 `accepted` |
| `occurred_at` | string | 是 | RFC3339 时间 |

## 3. Response Schema（`202 Accepted`）

顶层字段：

- `status`：`accepted` 或 `partial`
- `gateway_id` / `tenant_id` / `received_at`
- `access` / `device`：分项统计与逐条结果
- `totals`：全量统计
- `retry_subset`：可重试失败子集
- `queue_hint`：checkpoint 指引

### 3.1 `queue_hint`

| 字段 | 类型 | 说明 |
|---|---|---|
| `queue` | string | 队列名 |
| `checkpoint_id` | string | 建议 checkpoint ID |
| `acked_increment` | int | 本批建议 ack 增量 |
| `server_ingested_total` | int | 服务端已入队总量（仅新创建记录增长） |
| `status_code` | string | 队列状态码 |
| `next_action` | string | 下一动作建议 |
| `retry_subset_present` | bool | 是否有可重试子集 |

### 3.2 `status_code` 与 `next_action`

| `status_code` | `next_action` | 语义 |
|---|---|---|
| `QUEUE_READY_TO_CHECKPOINT` | `report_checkpoint` | 可直接报 checkpoint |
| `QUEUE_RETRY_SUBSET_REQUIRED` | `replay_retry_subset_then_report_checkpoint` | 先重放 `retry_subset` |
| `QUEUE_PARTIAL_NON_RETRYABLE` | `report_checkpoint_with_non_retryable_failures` | 含不可重试失败，按业务策略记账后报 checkpoint |

## 4. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `batch events are required` | 两类事件都为空 |
| `400` | `batch size exceeded: max 200` | 超过最大批量 |
| `400` | `occurred_at must be RFC3339` | 单条时间格式非法 |
| `401` | `missing device token` / `invalid device token` | 设备凭证问题 |
| `404` | `gateway not found` | 网关或租户不匹配 |
| `500` | `internal error` | 服务端异常 |

## 5. 最小请求示例

```json
{
  "gateway_id": "gw_demo_001",
  "tenant_id": "tenant_demo_jakarta",
  "queue": "default",
  "access_events": [
    {
      "event_id": "gwea-001",
      "request_id": "rq-gwea-001",
      "building_id": "building_demo_001",
      "area_id": "area_demo_001",
      "door_id": "door_jkt_001",
      "type": "access_granted",
      "actor": "device",
      "result": "success",
      "occurred_at": "2026-04-15T08:00:00Z"
    }
  ],
  "device_events": []
}
```

## 6. 相关 Guide

- `docs/wiki/external-api/guides-gateway-offline-workflow.md`
- `docs/wiki/external-api/reference-gateway-serial-inventory-checkpoint-summary.md`
