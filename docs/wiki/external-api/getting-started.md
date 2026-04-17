# Getting Started（Sandbox 10 分钟）

当前能力状态：

- `CONTRACT_READY`：本页步骤基于现有 API 合同，可直接用于联调起步。
- `PROD_READY`：示例路径已被当前回归脚本覆盖（认证 + 网关链路）。

## 1. 目标

本页帮助你在最短路径内完成三件事：

1. 获取 `access_token`。
2. 调通一个管理端受保护接口。
3. 跑通网关侧最小闭环（`register -> config/pull -> events/batch -> events/checkpoint`）。

## 2. 前置条件

- API Base URL：例如 `http://localhost:8080`。
- 测试账号（默认开发环境）：`superadmin@mistypass.local / admin123`。
- 具备 `curl` 与 `jq`。

建议先设置环境变量：

```bash
export API_BASE_URL="http://localhost:8080"
export EMAIL="superadmin@mistypass.local"
export PASSWORD="admin123"
```

## 3. Step 1：登录获取访问令牌

```bash
LOGIN_JSON=$(curl -sS -X POST "$API_BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")

export ACCESS_TOKEN=$(echo "$LOGIN_JSON" | jq -r '.access_token')
```

成功响应示例：

```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_in": 3600,
  "user": {
    "id": "usr_super_admin_001",
    "email": "superadmin@mistypass.local",
    "role": "super_admin",
    "tenant_id": ""
  }
}
```

## 4. Step 2：调用受保护接口

```bash
curl -sS "$API_BASE_URL/api/v1/tenants" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

如果返回 `{"error":"forbidden"}`，说明账号角色不满足该接口要求。

## 5. Step 3：跑通网关最小闭环

### 5.1 导入序列号库存（管理端）

```bash
export TENANT_ID="tenant_demo_jakarta"
export BUILDING_ID="building_demo_001"
export GW_SERIAL="MP-GW-QS-$(date +%s)"

curl -sS -X POST "$API_BASE_URL/api/v1/gateways/serial-inventory/import" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"items\":[{\"serial_number\":\"$GW_SERIAL\",\"product_type\":\"gateway\",\"batch_code\":\"quickstart\",\"source\":\"factory\"}]}"
```

### 5.2 设备 bootstrap 注册（网关侧）

```bash
REGISTER_JSON=$(curl -sS -X POST "$API_BASE_URL/api/v1/gateway/register" \
  -H "Content-Type: application/json" \
  -d "{\"serial_number\":\"$GW_SERIAL\",\"tenant_id\":\"$TENANT_ID\",\"building_id\":\"$BUILDING_ID\",\"device_capacity\":4}")

export GATEWAY_ID=$(echo "$REGISTER_JSON" | jq -r '.gateway_id')
export DEVICE_TOKEN=$(echo "$REGISTER_JSON" | jq -r '.device_token')
```

### 5.3 拉取配置（网关侧）

```bash
curl -sS -X POST "$API_BASE_URL/api/v1/gateway/config/pull" \
  -H "X-Device-Token: $DEVICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"gateway_id\":\"$GATEWAY_ID\",\"tenant_id\":\"$TENANT_ID\",\"current_version\":\"cfg-old\"}"
```

### 5.4 上报批量事件（网关侧）

```bash
NOW_UTC=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EVENT_ID="gwea-qs-$(date +%s)"

BATCH_JSON=$(curl -sS -X POST "$API_BASE_URL/api/v1/gateway/events/batch" \
  -H "X-Device-Token: $DEVICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"gateway_id\":\"$GATEWAY_ID\",\"tenant_id\":\"$TENANT_ID\",\"queue\":\"default\",\"access_events\":[{\"event_id\":\"$EVENT_ID\",\"request_id\":\"rq-$EVENT_ID\",\"building_id\":\"$BUILDING_ID\",\"area_id\":\"area_demo_001\",\"door_id\":\"door_jkt_001\",\"type\":\"access_granted\",\"actor\":\"quickstart\",\"result\":\"success\",\"occurred_at\":\"$NOW_UTC\"}],\"device_events\":[]}")

echo "$BATCH_JSON" | jq '.queue_hint'
```

### 5.5 上报 checkpoint（网关侧）

```bash
ACK_INC=$(echo "$BATCH_JSON" | jq -r '.queue_hint.acked_increment')
CHECKPOINT_ID=$(echo "$BATCH_JSON" | jq -r '.queue_hint.checkpoint_id')

curl -sS -X POST "$API_BASE_URL/api/v1/gateway/events/checkpoint" \
  -H "X-Device-Token: $DEVICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"gateway_id\":\"$GATEWAY_ID\",\"tenant_id\":\"$TENANT_ID\",\"queue\":\"default\",\"checkpoint_id\":\"$CHECKPOINT_ID\",\"last_request_id\":\"rq-$EVENT_ID\",\"acked_count\":$ACK_INC,\"last_occurred_at\":\"$NOW_UTC\"}"
```

## 6. 常见问题

- `401 missing bearer token`：管理端接口未带 `Authorization`。
- `401 missing device token`：网关 bootstrap 接口未带 `X-Device-Token`（注册接口除外）。
- `409 event checkpoint acked_count exceeds server event total`：checkpoint 超出服务端累计值，按返回 `server_event_total` 回调。

## 7. 下一步

- 认证与权限细节：`docs/wiki/external-api/authentication.md`
- 网关离线链路（完整）：`docs/wiki/external-api/guides-gateway-offline-workflow.md`
- 企业 SSO 与 JIT 审批：`docs/wiki/external-api/guides-enterprise-sso-jit.md`
- 企业 JIT 审批回写：`docs/wiki/external-api/guides-enterprise-jit-approval-external-sync.md`
- 企业同步补偿接口参考：`docs/wiki/external-api/reference-enterprise-sync-requests-reconcile.md`
- 企业域名/IdP/员工同步资源参考：`docs/wiki/external-api/reference-enterprise-tenant-domain.md`、`docs/wiki/external-api/reference-enterprise-idp-config.md`、`docs/wiki/external-api/reference-enterprise-employees-sync-jobs.md`
- Wallet 队列运营闭环：`docs/wiki/external-api/guides-wallet-queue-ops.md`
- Wallet DLQ 治理：`docs/wiki/external-api/guides-wallet-dlq-governance.md`
- Wallet 指标与订阅接口参考：`docs/wiki/external-api/reference-wallet-job-metrics.md`、`docs/wiki/external-api/reference-wallet-alert-subscription.md`
- Wallet jobs/process/alert-notifications 资源参考：`docs/wiki/external-api/reference-wallet-jobs-process-notifications.md`
- Auth 会话接口参考：`docs/wiki/external-api/reference-auth-session-me.md`
- Token/Scope 最小权限与排障：`docs/wiki/external-api/guides-api-token-scope-troubleshooting.md`
- Gateway serial-inventory/checkpoint-summary 资源参考：`docs/wiki/external-api/reference-gateway-serial-inventory-checkpoint-summary.md`
- Tenant/Space/Access 资源参考：`docs/wiki/external-api/reference-tenant-space-core.md`、`docs/wiki/external-api/reference-access-core.md`
- 错误与重试策略：`docs/wiki/external-api/errors-and-reliability.md`
- 限流与 429 退避：`docs/wiki/external-api/guides-rate-limit-and-429.md`
- 版本变更与迁移指引：`docs/wiki/external-api/changelog-and-migration.md`
- 发布流程模板：`docs/wiki/external-api/release-process-template.md`
- 弃用标注规范：`docs/wiki/external-api/deprecation-policy.md`
