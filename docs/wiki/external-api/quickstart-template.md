# Quick Start 模板（面向 API 调用方）

当前能力状态：

- `CONTRACT_READY`：模板可直接复用为对外 Quick Start 页面。
- `PROD_READY`：示例路径基于现有 API，能覆盖从认证到核心调用的最小闭环。

## 1. 前置条件

- 已获取测试账号（email/password）。
- 已拿到 API Base URL（Sandbox 或 Production）。
- 调用工具：curl / Postman / 任意 HTTP client。

## 2. Step 1: 获取访问令牌

```bash
curl -X POST "$API_BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"YOUR_EMAIL","password":"YOUR_PASSWORD"}'
```

期望结果：返回 `access_token`。

## 3. Step 2: 调用一个受保护接口

```bash
curl -X GET "$API_BASE_URL/api/v1/tenants" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

期望结果：返回租户列表。

## 4. Step 3: 跑通一个业务流程（示例：网关注册）

1. 导入序列号库存
2. 准备平台发放的 `GATEWAY_BOOTSTRAP_TOKEN`
3. 带 `X-Bootstrap-Token` 调用 `POST /api/v1/gateway/register`
4. 记录返回的 `gateway_id` 与 `device_token`

## 5. 常见错误

- `401 unauthorized`：token 缺失或过期。
- `403 forbidden`：角色权限不足。
- `409 conflict`：资源状态冲突（如 checkpoint 回退、序列号重复）。

## 6. 下一步链接（发布版建议）

- Authentication
- Gateway Guide
- Enterprise SSO Guide
- Wallet Guide
- Error Code Reference
