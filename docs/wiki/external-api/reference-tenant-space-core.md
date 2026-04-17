# API Reference：Tenant / Space Core

当前能力状态：

- `CONTRACT_READY`：租户、楼宇、楼层、区域、门点核心接口已稳定。
- `PROD_READY`：租户作用域与 `building_admin` 楼宇范围约束有回归覆盖。

## 1. Endpoint Matrix

### Tenant

| Method | Path | 角色 |
|---|---|---|
| `GET` | `/api/v1/tenants` | `super_admin` |
| `POST` | `/api/v1/tenants` | `super_admin` |
| `PATCH` | `/api/v1/tenants/{tenantID}/status` | `super_admin` |
| `GET` | `/api/v1/tenants/{tenantID}/topology` | `super_admin` / `tenant_admin` / `operator` / `building_admin` |

### Space

| Method | Path | 角色 |
|---|---|---|
| `GET` | `/api/v1/buildings` | `super_admin` / `tenant_admin` / `operator` / `building_admin` |
| `POST` | `/api/v1/buildings` | `super_admin` / `tenant_admin` |
| `GET` | `/api/v1/floors` | `super_admin` / `tenant_admin` / `operator` / `building_admin` |
| `POST` | `/api/v1/floors` | `super_admin` / `tenant_admin` / `building_admin` |
| `GET` | `/api/v1/areas` | `super_admin` / `tenant_admin` / `operator` / `building_admin` |
| `POST` | `/api/v1/areas` | `super_admin` / `tenant_admin` / `building_admin` |
| `GET` | `/api/v1/doors` | `super_admin` / `tenant_admin` / `operator` / `building_admin` |
| `POST` | `/api/v1/doors` | `super_admin` / `tenant_admin` / `building_admin` |

## 2. Tenant API

### 2.1 `POST /api/v1/tenants`

```json
{
  "name": "MistyPass SG",
  "type": "company",
  "hq_region": "SG"
}
```

`type` 允许值：

- `company`（默认）
- `studio`
- `government`
- `factory`
- `public_facility`

### 2.2 `PATCH /api/v1/tenants/{tenantID}/status`

```json
{
  "status": "suspended"
}
```

`status` 允许值：`active` / `suspended` / `inactive`。

### 2.3 `GET /api/v1/tenants/{tenantID}/topology`

返回 `buildings/floors/areas/doors` 聚合拓扑；`building_admin` 会自动按 `building_ids` 过滤。

## 3. Space API

### 3.1 `POST /api/v1/buildings`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "name": "Sudirman Annex",
  "address": "Jl. Sudirman No. 100",
  "region": "ID-JK"
}
```

### 3.2 `POST /api/v1/floors`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "building_id": "building_demo_001",
  "name": "L12"
}
```

### 3.3 `POST /api/v1/areas`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "building_id": "building_demo_001",
  "floor_id": "floor_demo_001",
  "name": "R&D"
}
```

### 3.4 `POST /api/v1/doors`

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "building_id": "building_demo_001",
  "floor_id": "floor_demo_001",
  "area_id": "area_demo_001",
  "name": "R&D East Gate",
  "gateway_id": "MP-GW-JKT-0001",
  "kind": "office",
  "status": "online"
}
```

`kind` 允许值：

- `office`（默认）
- `turnstile`
- `server-room`
- `elevator`
- `parking-gate`
- `emergency-exit`

`status` 允许值：`online` / `offline`（默认 `offline`）。

## 4. Scope 约束

- 所有空间列表接口支持 `tenant_id` query。
- `building_admin` 在创建 `floor/area/door` 时，若缺少 `building_id` 会返回 `400`。
- `building_admin` 若访问超出授权楼宇范围会返回 `403 building scope forbidden`。

## 5. Error Cases

| HTTP | 错误 | 说明 |
|---|---|---|
| `400` | `tenant_id is required` 等 | 入参缺失 |
| `400` | `invalid tenant type/status` | 枚举值非法 |
| `400` | `invalid door kind/status` | 门点类型或状态非法 |
| `404` | `tenant/building/floor/area not found` | 资源不存在 |
| `409` | `tenant ownership mismatch in topology` | 资源跨租户冲突 |
| `403` | `tenant scope forbidden` / `building scope forbidden` | 作用域越权 |

