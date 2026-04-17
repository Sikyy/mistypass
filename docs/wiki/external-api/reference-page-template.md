# API Reference 页面模板

当前能力状态：

- `CONTRACT_READY`：模板可用于统一所有 endpoint 文档风格。
- `PROD_READY`：字段项已覆盖现有接口常见需求（鉴权、幂等、错误、示例）。

## 1. Endpoint

- Method: `POST`
- Path: `/api/v1/example/resource`
- Scope: `tenant_admin` / `super_admin`

## 2. Authentication

- Header: `Authorization: Bearer <access_token>`
- 可选：`X-Device-Token`（仅 gateway bootstrap 端点）

## 3. Request

### 3.1 Body schema

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `tenant_id` | string | 是 | 租户标识 |
| `request_id` | string | 否 | 幂等键（推荐） |

### 3.2 Request 示例

```json
{
  "tenant_id": "tenant_demo_jakarta",
  "request_id": "req-20260415-001"
}
```

## 4. Response

### 4.1 Success (`200` / `201` / `202`)

```json
{
  "status": "accepted",
  "request_id": "req-20260415-001"
}
```

### 4.2 Error

| HTTP | code / error | 含义 | 建议处理 |
|---|---|---|---|
| `400` | bad request | 入参不合法 | 修复参数后重试 |
| `401` | unauthorized | 凭证失效 | 刷新或重新登录 |
| `403` | forbidden | 权限不足 | 使用正确角色或租户 |
| `409` | conflict | 状态冲突 | 根据返回 `next_action` 调整 |
| `500` | internal error | 服务端异常 | 指数退避重试 + 告警 |

## 5. Idempotency / Retry

- 若支持 `request_id` 或 `idempotency_key`，应明确重复请求语义。
- 明确哪些错误可重试，哪些不可重试。

## 6. Audit / Trace（可选）

- 该接口是否写审计日志。
- 审计 `action/source` 示例。

## 7. Changelog

- `2026-04-15`: 初版发布。
