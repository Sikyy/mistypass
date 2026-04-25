# API Reference：Enterprise Employees / Sync Jobs

当前能力状态：

- `CONTRACT_READY`：员工目录查询、同步入库、同步任务查询接口字段已稳定。
- `PROD_READY`：`request_id` 幂等、access 回写统计、域名约束与状态归一化有回归覆盖。

## 1. Endpoint Matrix

| 资源 | 主要接口 | 角色 |
|---|---|---|
| Employees | `GET /api/v1/enterprise/employees` | `super_admin/tenant_admin/operator` |
| Employee Sync | `POST /api/v1/enterprise/employees/sync` | `super_admin/tenant_admin` |
| Sync Jobs | `GET /api/v1/enterprise/sync-jobs` | `super_admin/tenant_admin/operator` |

说明：`sync-requests/reconcile` 已独立文档：`docs/wiki/external-api/reference-enterprise-sync-requests-reconcile.md`。

## 2. `GET /api/v1/enterprise/employees`

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
      "id": "emp_001",
      "tenant_id": "tenant_demo_jakarta",
      "external_id": "hris-jkt-1001",
      "email": "arief.putra@sudirman.co",
      "full_name": "Arief Putra",
      "department": "Finance",
      "job_title": "Finance Manager",
      "location": "Jakarta",
      "employment_status": "active",
      "access_role": "resident",
      "building_id": "building_demo_001",
      "group_ids": ["ug_1001"],
      "status": "active",
      "source": "scim_sync",
      "last_synced_at": "2026-04-15T09:30:00Z"
    }
  ]
}
```

## 3. `POST /api/v1/enterprise/employees/sync`

用途：写入企业员工快照并触发 access upsert 汇总。

### Request

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "source": "hris_talenta",
  "actor": "hris-sync-worker",
  "request_id": "req-20260415-1001",
  "connector_id": "connector_talenta_jakarta",
  "raw_payload_ref": "s3://mistypass-sync-raw/talenta/2026-04-22/event-001.json",
  "employees": [
    {
      "external_id": "hris-jkt-1001",
      "employee_number": "EMP-1001",
      "email": "arief.putra@sudirman.co",
      "full_name": "Arief Putra",
      "department": "Finance",
      "job_title": "Finance Manager",
      "location": "Jakarta",
      "phone": "+62-811-0000-1111",
      "manager_external_id": "hris-jkt-0001",
      "employment_status": "active",
      "join_date": "2024-01-15",
      "resign_date": "",
      "shift_code": "SHIFT-A",
      "schedule_window": "mon-fri:09:00-18:00",
      "leave_status": "none",
      "cost_center": "CC-FIN-01",
      "photo_url": "https://cdn.vendor.local/photos/EMP-1001.jpg"
    }
  ]
}
```

### Success（`202`）

```json
{
  "job": {
    "id": "syn_001",
    "tenant_id": "tenant_demo_jakarta",
    "source": "scim_sync",
    "status": "completed",
    "total": 1,
    "created": 1,
    "updated": 0,
    "deactivated": 0,
    "rejected": 0,
    "actor": "hris-sync-worker",
    "started_at": "2026-04-15T10:20:00Z",
    "ended_at": "2026-04-15T10:20:01Z"
  },
  "items": [
    {
      "id": "emp_1001",
      "tenant_id": "tenant_demo_jakarta",
      "external_id": "hris-jkt-1001",
      "email": "arief.putra@sudirman.co",
      "status": "active"
    }
  ],
  "access_sync": {
    "created": 1,
    "updated": 0,
    "rejected": 0
  }
}
```

同步语义：

- `source` 缺省时默认 `manual_sync`，并做白名单校验（推荐值：`hris_talenta/hris_gadjian/hris_greatday/hris_linovhr/hris_sunfish/manual/csv_import`；兼容 `manual_sync/scim_sync/hris_import/hris/scim`）。
- `actor` 缺省时默认 `system`。
- `request_id` 参与幂等：同租户同 `request_id` 重复提交会复用已记录结果。
- `connector_id`、`raw_payload_ref` 可选；当携带 `request_id` 时会写入 sync request 记录，供排障与重放引用。
- 仅邮箱域名匹配 `active domain mapping` 的员工会被接受，其他记录计入 `rejected`。
- `employment_status` 会归一化，`inactive/terminated/disabled/suspended/deprovisioned` 会映射到员工 `status=inactive`。

## 4. `GET /api/v1/enterprise/sync-jobs`

### Query

| 字段 | 必填 | 说明 |
|---|---|---|
| `tenant_id` | 否 | `super_admin` 可空；其他角色按 token tenant 约束 |

### Success（`200`）

```json
{
  "items": [
    {
      "id": "syn_001",
      "tenant_id": "tenant_demo_jakarta",
      "source": "scim_sync",
      "status": "completed",
      "total": 120,
      "created": 35,
      "updated": 70,
      "deactivated": 5,
      "rejected": 10,
      "actor": "hris-sync-worker",
      "started_at": "2026-04-15T10:20:00Z",
      "ended_at": "2026-04-15T10:20:06Z"
    }
  ]
}
```

## 5. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `tenant_id is required` | 缺少租户 |
| `400` | `employees is required` | 同步输入为空 |
| `400` | `access sync applier is required` | 服务依赖缺失 |
| `403` | `tenant scope forbidden` | 租户越权 |
| `500` | `access sync failed, enterprise sync rolled back: ...` | access upsert 失败并触发回滚 |
