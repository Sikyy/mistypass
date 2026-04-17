# API Reference：Gateway Serial Inventory / Checkpoint Summary

当前能力状态：

- `CONTRACT_READY`：serial-inventory 与 checkpoint-summary 资源接口字段/状态码已稳定。
- `PROD_READY`：批量状态迁移、CSV 导入导出、多队列 checkpoint 趋势统计与 scope 边界有回归覆盖。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| Serial Inventory | `GET /api/v1/gateways/serial-inventory` | `super_admin/tenant_admin/operator` |
| Serial Inventory Import | `POST /api/v1/gateways/serial-inventory/import`、`POST /api/v1/gateways/serial-inventory/import-csv` | `super_admin/tenant_admin` |
| Serial Inventory Status | `PATCH /api/v1/gateways/serial-inventory/batch-status`、`PATCH /api/v1/gateways/serial-inventory/{serialNumber}/status` | `super_admin/tenant_admin` |
| Serial Inventory Export | `GET /api/v1/gateways/serial-inventory/export-csv` | `super_admin/tenant_admin/operator` |
| Checkpoint Summary | `GET /api/v1/gateways/events/checkpoint/summary` | `super_admin/tenant_admin/operator/building_admin` |

## 2. Serial Inventory

### 2.1 `GET /api/v1/gateways/serial-inventory`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |
| `product_type` | 否 | `gateway/reader/controller/relay/sensor` |
| `status` | 否 | `available/consumed/frozen/scrapped` |

### Success（`200`）

```json
{
  "items": [
    {
      "id": "sin_001",
      "tenant_id": "tenant_demo_jakarta",
      "serial_number": "MP-GW-JKT-0001",
      "product_type": "gateway",
      "status": "available",
      "batch_code": "batch-202604",
      "source": "factory",
      "consumed_gateway_id": "",
      "consumed_at": null,
      "created_at": "2026-04-15T08:00:00Z",
      "updated_at": "2026-04-15T08:00:00Z"
    }
  ]
}
```

### 2.2 `POST /api/v1/gateways/serial-inventory/import`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "items": [
    {
      "serial_number": "MP-GW-QA-001",
      "product_type": "gateway",
      "batch_code": "factory-q2",
      "source": "factory"
    },
    {
      "serial_number": "RD-QA-001",
      "product_type": "reader",
      "batch_code": "factory-q2",
      "source": "factory"
    }
  ]
}
```

### Success（`201`）

```json
{
  "items": [
    {
      "serial_number": "MP-GW-QA-001",
      "product_type": "gateway",
      "status": "available"
    },
    {
      "serial_number": "RD-QA-001",
      "product_type": "reader",
      "status": "available"
    }
  ]
}
```

### 2.3 `POST /api/v1/gateways/serial-inventory/import-csv`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "csv_content": "serial_number,product_type,batch_code,source\nMP-GW-QA-002,gateway,batch-csv,factory\nRD-QA-002,reader,batch-csv,factory"
}
```

CSV 说明：

- 首行可选 header（识别 `serial_number,product_type`）。
- 逐行解析顺序：`serial_number,product_type,batch_code,source`。
- 空内容或无有效记录返回 `serial inventory records are required`。

### 2.4 `PATCH /api/v1/gateways/serial-inventory/{serialNumber}/status`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "status": "consumed",
  "consumed_gateway_id": "gw_demo_001"
}
```

### 2.5 `PATCH /api/v1/gateways/serial-inventory/batch-status`

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "status": "frozen",
  "serial_numbers": ["MP-GW-QA-001", "RD-QA-001"],
  "consumed_gateway_id": ""
}
```

### 状态流转规则

- 状态枚举：`available` / `consumed` / `frozen` / `scrapped`。
- `scrapped` 为终态：一旦 `scrapped`，不能再转回 `available/consumed/frozen`。
- 转为 `consumed` 时可写入 `consumed_gateway_id`，且首次会补 `consumed_at`。
- 转为 `available` 或 `scrapped` 时会清空 `consumed_gateway_id/consumed_at`。

### 2.6 `GET /api/v1/gateways/serial-inventory/export-csv`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |
| `product_type` | 否 | 同列表 |
| `status` | 否 | 同列表 |

### Success（`200`, `text/csv`）

首行固定：

```csv
serial_number,product_type,status,tenant_id,batch_code,source,consumed_gateway_id,consumed_at,updated_at
```

## 3. `GET /api/v1/gateways/events/checkpoint/summary`

用途：查看租户级（可按 gateway/queue 过滤）checkpoint 与事件滞后统计，并给出时间窗趋势。

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |
| `gateway_id` | 否 | 指定单网关；不存在返回 `404 gateway not found` |
| `queue` | 否 | 队列名；为空表示不过滤 |
| `limit` | 否 | `>=0`，默认 `100`；`0` 表示不截断 |
| `trend_window_minutes` | 否 | `>0`，默认 `60` |

### Success（`200`）

```json
{
  "items": [
    {
      "gateway_id": "gw_demo_001",
      "tenant_id": "tenant_demo_jakarta",
      "building_id": "building_demo_001",
      "queue": "default",
      "checkpoint_id": "gw_demo_001-default-1744704000000",
      "last_request_id": "rq-1744704000",
      "acked_count": 120,
      "event_total": 120,
      "access_event_total": 100,
      "device_event_total": 20,
      "lag_count": 0,
      "last_occurred_at": "2026-04-15T10:40:00Z",
      "updated_at": "2026-04-15T10:40:05Z",
      "time_window_trend": {
        "report_total": 3,
        "acked_delta": 20,
        "direction": "up",
        "first_report_at": "2026-04-15T10:25:00Z",
        "last_report_at": "2026-04-15T10:40:05Z"
      }
    }
  ],
  "totals": {
    "queues": 1,
    "event_total": 120,
    "acked_total": 120,
    "lag_total": 0
  },
  "time_window_trend": {
    "window_minutes": 15,
    "since": "2026-04-15T10:25:05Z",
    "until": "2026-04-15T10:40:05Z",
    "report_total": 3,
    "gateway_total": 1,
    "queue_total": 1,
    "acked_delta_total": 20,
    "direction": "up",
    "last_report_at": "2026-04-15T10:40:05Z"
  }
}
```

`direction` 取值：`up` / `down` / `flat`。

## 4. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `invalid serial inventory product_type` | `product_type` 非法 |
| `400` | `invalid serial inventory status` | `status` 非法 |
| `400` | `serial inventory records are required` | 导入记录为空 |
| `400` | `invalid csv_content format` | CSV 解析失败 |
| `400` | `limit must be an integer >= 0` | summary `limit` 非法 |
| `400` | `trend_window_minutes must be an integer > 0` | summary 时间窗非法 |
| `404` | `serial inventory not found` | 状态更新目标不存在 |
| `404` | `gateway not found` | summary `gateway_id` 不存在 |
| `409` | `serial inventory tenant mismatch` | serial 与租户不匹配 |
| `409` | `serial inventory status transition invalid` | 非法状态流转（典型为 `scrapped` 回退） |
| `403` | `tenant scope forbidden` / `building scope forbidden` | 越权 |

## 5. 回归脚本

- `docs/testing/curl-gateway-serial-inventory-csv.zsh`
- `docs/testing/curl-gateway-serial-inventory-batch.zsh`
- `docs/testing/curl-gateway-event-checkpoint-partial.zsh`

## 6. 相关 Guide

- `docs/wiki/external-api/guides-gateway-offline-workflow.md`
- `docs/wiki/external-api/reference-gateway-events-batch.md`
