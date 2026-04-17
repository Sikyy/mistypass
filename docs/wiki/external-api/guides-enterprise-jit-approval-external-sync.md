# Guide：Enterprise JIT 审批回写

当前能力状态：

- `CONTRACT_READY`：审批待回写查询、状态上报、公开回调接口已稳定。
- `PROD_READY`：`pending -> synced/failed` 状态流转、失败重试与审计记录有回归覆盖。

## 1. 目标

让企业上游系统（HRIS/SCIM/审批中台）在 JIT 审批后，能可靠回写 MistyPass 审批状态，并支持失败补偿。

## 2. 关键端点

- `GET /api/v1/enterprise/jit-provision-approvals/external-sync-pending`
- `POST /api/v1/enterprise/jit-provision-approvals/{approvalID}/external-sync`
- `POST /api/v1/enterprise/jit-provision-approvals/external-sync/callback`

## 3. 推荐流程

### 3.1 拉取待回写审批

使用 `external-sync-pending` 查询 `status in (approved,rejected)` 且 `external_sync_status=pending` 的审批单。

### 3.2 上报回写结果

两种模式二选一：

1. 受保护接口模式：调用 `/{approvalID}/external-sync`（需要管理端访问令牌）。
2. 回调模式：调用 `/external-sync/callback`（使用 callback token，不依赖用户会话）。

状态只接受：

- `synced`：回写成功。
- `failed`：回写失败（会累计 `external_sync_attempt_count` 并保留 `last_error`）。

### 3.3 失败重试与观测

- 后台 worker 会重试 `failed` 记录（受环境变量策略控制）。
- 可通过审计日志跟踪：
  - `enterprise_jit_approval_external_sync_updated`
  - `enterprise_jit_approval_external_sync_callback`
  - `enterprise_jit_approval_external_sync_worker_alert`

## 4. 回调令牌与 Worker 配置

关键配置项：

- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_CALLBACK_TOKEN`
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ENABLED`
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_INTERVAL`
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_BATCH_SIZE`
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_MAX_ATTEMPTS`
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_RETRY_COOLDOWN`
- `ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ALERT_FAILURE_THRESHOLD`

若 `CALLBACK_TOKEN` 未配置，`/external-sync/callback` 返回 `503`。

## 5. 常见错误

| HTTP | 场景 |
|---|---|
| `400` | `status` 非 `synced/failed`，或 `tenant_id` 缺失 |
| `401` | callback token 无效 |
| `404` | `approval_id` 不存在或不在当前 tenant |
| `503` | callback 功能未启用（token 未配置） |

## 6. 回归脚本

- `docs/testing/curl-enterprise-sync-access-batch.zsh`

## 7. 相关 Reference

- `docs/wiki/external-api/reference-enterprise-jit-approval-external-sync.md`
- `docs/wiki/external-api/reference-enterprise-sync-worker-alerts.md`
- `docs/wiki/external-api/reference-enterprise-sync-requests-reconcile.md`
