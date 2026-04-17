# Guide：API Token & Scope（最小权限与排障）

当前能力状态：

- `CONTRACT_READY`：Token 传递方式与 role/tenant/building 作用域语义已稳定。
- `PROD_READY`：`missing bearer token / tenant scope forbidden / building scope forbidden` 等关键错误有持续回归覆盖。

## 1. 适用场景

当你需要：

- 为合作方分配最小权限账号；
- 排查 `401/403` 认证与越权问题；
- 管理 `building_admin` 的楼宇范围；

建议按本页步骤执行。

## 2. Token 类型矩阵（当前实现）

| 类型 | 获取方式 | 传递方式 | 主要用途 |
|---|---|---|---|
| 管理端访问令牌（`access_token`） | `POST /api/v1/auth/login`（或 `refresh`） | `Authorization: Bearer <access_token>` | `/api/v1/*` 管理端受保护接口 |
| 网关设备令牌（`device_token`） | `POST /api/v1/gateway/register` | `X-Device-Token: <device_token>`（兼容 Bearer） | `/api/v1/gateway/*` bootstrap/事件链路 |
| 企业回调令牌（callback token） | 平台配置 `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_CALLBACK_TOKEN` | `X-Enterprise-Callback-Token`（兼容 Bearer / body） | `POST /api/v1/enterprise/jit-provision-approvals/external-sync/callback` |

说明（截至 2026-04-15）：

- 管理端 API 当前不支持 `X-API-Key` 作为通用鉴权方式，默认使用 Bearer token。
- token 过期或失效后，统一走 `refresh` 或重新登录获取新 token 对。

## 3. 作用域模型

### 3.1 Role Scope（角色作用域）

- 路由层按角色白名单校验。
- 角色不满足返回 `403 forbidden`。

### 3.2 Tenant Scope（租户作用域）

- 非 `super_admin` 用户会被限制在 token 内的 `tenant_id`。
- 跨租户访问返回 `403 tenant scope forbidden`。

### 3.3 Building Scope（楼宇作用域）

- `building_admin` 额外受 `building_ids` 限制。
- 无楼宇范围或访问越界返回 `403 building scope forbidden`。

## 4. 最小权限分配建议

1. 仅单租户运维：使用 `tenant_admin`，避免 `super_admin` 常驻。
2. 单楼宇运维：使用 `building_admin` 并显式维护 `building_ids`。
3. 平台级跨租户排障：仅在必要窗口使用 `super_admin`。

## 5. Building Scope 管理（可执行）

接口：

- `GET /api/v1/auth/users/{userID}/building-scope`
- `PUT /api/v1/auth/users/{userID}/building-scope`

权限：

- `super_admin` 可管理任意租户用户。
- `tenant_admin` 仅可管理同租户用户。

示例（更新楼宇范围）：

```bash
curl -sS -X PUT "$API_BASE_URL/api/v1/auth/users/usr_building_admin_001/building-scope" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "building_ids": ["building_demo_001", "building_demo_002"]
  }'
```

成功返回（`200`）：

```json
{
  "user_id": "usr_building_admin_001",
  "email": "ops.building@sudirman.co",
  "role": "building_admin",
  "tenant_id": "tenant_demo_jakarta",
  "building_ids": ["building_demo_001", "building_demo_002"]
}
```

## 6. 常见错误排查

| HTTP | 错误 | 典型原因 | 动作 |
|---|---|---|---|
| `401` | `missing bearer token` | 未带 `Authorization` | 补齐 Bearer 头 |
| `401` | `invalid access token` | token 过期/已注销/签名无效 | 先 refresh，失败则重新登录 |
| `403` | `forbidden` | 角色不满足接口要求 | 更换角色或降级目标接口 |
| `403` | `tenant scope forbidden` | 跨租户访问 | 使用同租户账号或修正 `tenant_id` |
| `403` | `building scope forbidden` | `building_admin` 越界或无 scope | 补齐 `building_ids` |
| `404` | `user not found` | `userID` 错误 | 校验目标用户 ID |
| `409` | `user role does not support building scope` | 对非 `building_admin` 更新 scope | 调整目标账号角色或改用租户级权限模型 |

## 7. 推荐回归入口

- `docs/testing/curl-regression-gateway-employee.zsh`
- `docs/testing/curl-enterprise-sync-access-batch.zsh`
- `docs/testing/admin-ui-test-and-api-map.md`

## 8. 相关文档

- `docs/wiki/external-api/authentication.md`
- `docs/wiki/external-api/reference-auth-session-me.md`
- `docs/wiki/external-api/reference-tenant-space-core.md`
- `docs/wiki/external-api/reference-access-core.md`
