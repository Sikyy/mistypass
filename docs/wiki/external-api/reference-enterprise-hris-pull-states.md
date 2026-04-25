# API Reference：Enterprise HRIS Pull States

当前能力状态：

- `CONTRACT_READY`：`GET /api/v1/enterprise/hris-pull-states` 合同已可联调，并覆盖当前 Talenta pull worker 的主要运行态观测字段。
- `FOUNDATION_READY`：HRIS pull worker 已为每个 connector 持久化最新运行状态，可用于后台巡检与人工排障。
- `OPS_READY`：状态快照已覆盖 `running / succeeded / failed`，并暴露最近请求、模式、成功/失败时间与连续失败次数；后台 worker 现也已补 `processing_timeout + atomic claim`，会区分 fresh/stale `running`，失败重试也已升级为 `retry_cooldown(base) + retry_max_backoff(max)` 指数退避，且在接入 Redis 时会先通过 lease 对 tick 做多实例互斥。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| HRIS Pull State | `GET /api/v1/enterprise/hris-pull-states` | `super_admin/tenant_admin/operator` |

说明：

- 每个 connector 只保留一条最新 pull 状态快照，不返回历史执行明细。
- 返回结果按 `updated_at desc` 排序。
- 当前仅支持按租户过滤，不支持按 connector 单独查询或分页。
- worker 会先 claim 再执行 pull：fresh `running` 会被视为 in-flight 跳过，stale `running` 会在超过 `processing_timeout` 后被重新接管。
- `failed` connector 的自动重试 cooldown 现按 `consecutive_failures` 做指数退避，并受 `retry_max_backoff` 上限约束。
- 若部署接入 Redis lease，后台 ticker 在每次 tick 前会先争抢 worker lease；未获得 lease 的实例会直接跳过本轮。

## 2. `GET /api/v1/enterprise/hris-pull-states`

- Auth：`Authorization: Bearer <access_token>`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |

### Success（`200`）

```json
{
  "items": [
    {
      "tenant_id": "tenant_demo_jakarta",
      "connector_id": "connector-talenta",
      "vendor": "talenta",
      "status": "failed",
      "last_request_id": "pull-req-001",
      "last_mode": "incremental",
      "last_started_at": "2026-04-22T09:25:00Z",
      "last_success_at": "2026-04-22T09:05:00Z",
      "last_full_success_at": "2026-04-22T08:00:00Z",
      "last_failure_at": "2026-04-22T09:30:00Z",
      "last_error": "429 throttled",
      "consecutive_failures": 1,
      "created_at": "2026-04-22T08:00:00Z",
      "updated_at": "2026-04-22T09:30:00Z"
    }
  ]
}
```

### 字段语义

- `status`
  - `running`：pull 已开始，尚未完成。
  - `succeeded`：最近一次 pull 成功。
  - `failed`：最近一次 pull 失败。
- `running`
  - 新鲜 `running` 代表当前已有 pull 在途。
  - 若 worker crash 导致状态长期停留在 `running`，后台会在超过 `processing_timeout` 后重新 claim。
- `last_mode`
  - `full`：全量拉取。
  - `incremental`：增量拉取。
- `last_full_success_at`
  - 仅在最近一次全量 pull 成功时刷新。
  - 增量成功不会覆盖该字段。
- `consecutive_failures`
  - 失败时递增。
  - 成功后重置为 `0`。

## 3. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `403` | `tenant scope forbidden` | 租户越权 |
| `500` | `hris pull state service is not configured` | 服务未启用 |

## 4. 数据来源

- 本接口读取 pull worker 的最新状态快照，而不是审计事件聚合。
- `last_request_id` 在 pull 成功时更新；失败时保留最近一次成功写入的 request。
- `last_error` 与 `last_failure_at` 仅在失败后刷新；成功后会清空 `last_error` 并重置失败计数。
- `last_started_at` 在每次 claim 成功时刷新；因此 stale `running` 被重新接管后，该时间会前移到新的接管时刻。
