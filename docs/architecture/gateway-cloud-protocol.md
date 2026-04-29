# Gateway ↔ Cloud 通信协议规范

> 版本：1.0
> 更新日期：2026-04-30
> 状态：MVP 初版，随硬件联调持续修订

---

## 0. Kisi 方案 vs NATS：选型对比

### Kisi 的方案

Kisi Controller Pro 使用 **HTTPS 长轮询 + 推送回调** 模式：

- Controller 通过 **HTTPS** 连接 Kisi Cloud（不是 MQTT/NATS）
- 使用 **TLS mutual auth**（设备证书 + 服务端证书）认证设备身份
- **策略同步**：Controller 定期 pull 配置（`config/pull`），Cloud 返回版本化的策略包
- **事件上报**：Controller 将 access event 批量 POST 到 Cloud（`events/batch`）
- **远程开门**：Cloud 通过 HTTP push notification 或设备下次 poll 时下发命令
- **本地判定优先**：门禁判定在 Controller 本地完成，不依赖 Cloud 实时响应

### 对比表

| 维度 | Kisi (HTTPS pull/push) | NATS | MQTT |
|---|---|---|---|
| **延迟** | 高（取决于 poll 间隔，秒级~分钟级） | 极低（毫秒级） | 低（毫秒级） |
| **远程开门实时性** | 差（需等 Controller 下次 poll） | 好（即时推送） | 好（即时推送） |
| **防火墙穿透** | 好（HTTPS 443 出站） | 中（需开 4222 或走 WebSocket） | 好（1883/8883，也可走 WebSocket） |
| **离线容忍** | 好（本地判定不依赖云） | 好（断开重连自动恢复） | 好（QoS 1/2 + retained message） |
| **运维复杂度** | 低（只需 HTTPS） | 中（需部署 NATS server） | 中（需部署 MQTT broker） |
| **设备端实现** | 简单（HTTP client） | 中（需 NATS client 库） | 简单（大量嵌入式 MQTT 库） |
| **消息可靠性** | 高（HTTP 重试 + 幂等） | 高（JetStream 持久化） | 高（QoS 2 + 持久会话） |
| **适合场景** | 大规模部署、网络受限环境 | 内部系统、低延迟需求 | IoT 设备、嵌入式硬件 |

### 结论和建议

**不需要换。两套都保留，分层使用：**

```
┌─────────────────────────────────────────────┐
│  Cloud API                                   │
│  ┌─────────────┐  ┌──────────────────────┐  │
│  │ NATS (内部)  │  │ HTTPS Gateway API     │  │
│  │ 低延迟推送   │  │ 设备注册/配置/事件     │  │
│  └──────┬──────┘  └──────────┬───────────┘  │
│         │                     │              │
└─────────┼─────────────────────┼──────────────┘
          │                     │
    ┌─────┴─────┐         ┌────┴────┐
    │ Gateway    │         │ Gateway  │
    │ (内网/低延迟)│        │ (公网/NAT)│
    │ NATS client│         │ HTTPS pull│
    └───────────┘         └──────────┘
```

- **NATS**：用于**内网部署、低延迟场景**（办公室/园区内 Gateway 直连 NATS，远程开门实时响应）
- **HTTPS pull/push**：用于**公网部署、防火墙受限场景**（Gateway 只需 HTTPS 出站，策略 pull + 事件 batch push）
- **Gateway 端**：同时支持两种模式，启动时按配置选择
- **Cloud 端**：同时接受两种来源的事件，统一写入 event store

**当前 MVP 阶段用 NATS 是对的**——快速验证、实时反馈、开发调试方便。生产部署时加上 HTTPS pull 模式作为 fallback。

---

## 1. 设计原则

1. **本地判定优先**：门禁放行判定在 Gateway 本地完成，不依赖 Cloud 实时 round-trip
2. **异步 + 幂等**：策略下发和事件回传允许短暂延迟，通过幂等键防重复
3. **版本化策略**：配置和 access rule 通过版本号管理，Gateway 只接受更新的版本
4. **双通道**：NATS（低延迟推送）+ HTTPS（防火墙穿透 fallback）

---

## 2. NATS 主题设计

### 2.1 命名规则

```
{prefix}.gateway.{gateway_id}.{message_type}
```

- `prefix`：默认 `mistypass`，可通过 `NATS_SUBJECT_PREFIX` 配置
- `gateway_id`：Gateway 设备 ID（如 `gw_demo_001`）
- `message_type`：消息类型

### 2.2 主题清单

| 主题 | 方向 | 说明 |
|---|---|---|
| `mistypass.gateway.{id}.command` | Cloud → Gateway | 远程命令（unlock/lockdown/cancel） |
| `mistypass.gateway.{id}.event` | Gateway → Cloud | 事件回报（command_ack/access/heartbeat） |
| `mistypass.gateway.{id}.config` | Cloud → Gateway | 配置推送（access rules/策略包） |
| `mistypass.gateway.{id}.verify` | Gateway → Cloud | 凭证验证请求（在线验证模式） |
| `mistypass.gateway.{id}.verify_result` | Cloud → Gateway | 凭证验证结果 |

---

## 3. 消息格式

### 3.1 GatewayCommand（Cloud → Gateway）

```json
{
  "request_id": "door_jkt_001:unlock:1777480207056373000",
  "gateway_id": "gw_demo_001",
  "command": "unlock",
  "lock_id": "door_jkt_001",
  "place_id": "building_demo_001",
  "tenant_id": "tenant_demo_jakarta",
  "issued_by": "admin@mistypass.local",
  "issued_at": "2026-04-29T16:30:07Z"
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| request_id | string | 是 | 唯一请求 ID，用于 ack 关联 |
| gateway_id | string | 是 | 目标 Gateway |
| command | string | 是 | `unlock` / `lock_down` / `cancel_lockdown` / `reboot` / `config_publish` |
| lock_id | string | 否 | 目标门点（unlock/lockdown 必填） |
| place_id | string | 否 | 所属 Place |
| tenant_id | string | 是 | 租户 ID |
| issued_by | string | 否 | 操作人邮箱 |
| issued_at | string | 是 | ISO 8601 时间 |

### 3.2 GatewayEvent（Gateway → Cloud）

```json
{
  "request_id": "door_jkt_001:unlock:1777480207056373000",
  "gateway_id": "gw_demo_001",
  "event_type": "command_ack",
  "command": "unlock",
  "lock_id": "door_jkt_001",
  "place_id": "building_demo_001",
  "tenant_id": "tenant_demo_jakarta",
  "status": "success",
  "error": "",
  "occurred_at": "2026-04-29T16:30:07Z"
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| request_id | string | 否 | 关联的请求 ID（command_ack 必填） |
| gateway_id | string | 是 | 来源 Gateway |
| event_type | string | 是 | `command_ack` / `access_granted` / `access_denied` / `heartbeat` / `door_held` / `door_forced` / `tamper` |
| command | string | 否 | 被执行的命令（command_ack 填） |
| lock_id | string | 否 | 相关门点 |
| place_id | string | 否 | 所属 Place |
| tenant_id | string | 是 | 租户 ID |
| status | string | 是 | `success` / `failed` / `timeout` / `online` / `offline` |
| error | string | 否 | 失败原因 |
| occurred_at | string | 是 | ISO 8601 时间 |

### 3.3 CredentialVerifyRequest（Gateway → Cloud，在线验证模式）

```json
{
  "request_id": "verify_001",
  "gateway_id": "gw_demo_001",
  "reader_id": "gdv_demo_001",
  "lock_id": "door_jkt_001",
  "tenant_id": "tenant_demo_jakarta",
  "credential_type": "nfc_uid",
  "credential_data": "UID-1001",
  "occurred_at": "2026-04-29T16:30:07Z"
}
```

| credential_type | credential_data | 说明 |
|---|---|---|
| `nfc_uid` | NFC 卡 UID hex | 物理卡 |
| `card_number` | 卡号 | 物理卡 |
| `wiegand_26` | 26-bit bitstream | Wiegand 读卡器 |
| `wiegand_34` | 34-bit bitstream | Wiegand 读卡器 |
| `osdp_card` | OSDP card data | OSDP 读卡器 |
| `ble_token` | BLE 令牌 | 手机 BLE |
| `qr_code` | QR payload | 二维码 |
| `pin` | PIN 码 | 键盘 |

### 3.4 CredentialVerifyResponse（Cloud → Gateway）

```json
{
  "request_id": "verify_001",
  "gateway_id": "gw_demo_001",
  "lock_id": "door_jkt_001",
  "decision": "allow",
  "reason": "access_granted",
  "user_id": "usr_1001",
  "user_email": "andri@mistypass.local",
  "occurred_at": "2026-04-29T16:30:07Z"
}
```

### 3.5 Heartbeat

```json
{
  "gateway_id": "gw_demo_001",
  "event_type": "heartbeat",
  "status": "online",
  "tenant_id": "tenant_demo_jakarta",
  "occurred_at": "2026-04-29T16:30:07Z"
}
```

周期：每 30 秒一次。Cloud 超过 90 秒未收到 heartbeat 则标记 Gateway 为 offline。

---

## 4. HTTPS Pull/Push 模式（生产 fallback）

适用于公网部署、无法直连 NATS 的场景。

### 4.1 Gateway → Cloud（已有 API）

| 端点 | 方法 | 说明 |
|---|---|---|
| `/gateway/register` | POST | 首次注册 |
| `/gateway/activate` | POST | 激活设备 |
| `/gateway/heartbeat` | POST | 心跳 |
| `/gateway/status` | POST | 状态上报 |
| `/gateway/config/pull` | POST | 拉取配置（含 access rules） |
| `/gateway/config/applied` | POST | 确认配置已应用 |
| `/gateway/events/access` | POST | 单条 access event |
| `/gateway/events/device` | POST | 单条 device event |
| `/gateway/events/batch` | POST | 批量 event |
| `/gateway/events/checkpoint` | POST | 事件 checkpoint |
| `/gateway/verify-credential` | POST | 在线凭证验证 |

### 4.2 Cloud → Gateway（待实现）

需要在 `config/pull` 响应中附带 pending commands：

```json
{
  "config_version": "v42",
  "access_rules": [...],
  "pending_commands": [
    {
      "request_id": "...",
      "command": "unlock",
      "lock_id": "door_jkt_001",
      "issued_at": "..."
    }
  ]
}
```

Gateway 执行完毕后在下次 `events/batch` 中上报 command_ack。

---

## 5. Access Rule 缓存包格式

Gateway 本地缓存的策略包，用于离线判定。

```json
{
  "version": "v42",
  "tenant_id": "tenant_demo_jakarta",
  "generated_at": "2026-04-29T16:00:00Z",
  "expires_at": "2026-04-30T16:00:00Z",
  "rules": [
    {
      "credential_type": "nfc_uid",
      "credential_data": "UID-1001",
      "user_id": "usr_1001",
      "lock_ids": ["door_jkt_001", "door_jkt_014"],
      "time_windows": [
        {"day_of_week_set": "weekday", "start_time": "07:00", "end_time": "19:00"}
      ],
      "exception_dates": ["2026-08-17"],
      "valid_from": "2026-01-01T00:00:00Z",
      "valid_until": "2027-01-01T00:00:00Z"
    }
  ],
  "lockdown_locks": [],
  "blocked_credentials": []
}
```

Gateway 收到新版本后：
1. 校验 `version > local_version`
2. 替换本地规则缓存
3. 回报 `config/applied` 确认

---

## 6. 典型流程

### 6.1 NFC 刷卡开门（本地判定）

```
Reader → [Wiegand/OSDP] → Gateway
Gateway: 查本地 access_rules → 匹配 UID + lock_id + time_window
  → 命中 → 继电器开门 → 上报 access_granted event
  → 未命中 → 拒绝 → 上报 access_denied event
```

### 6.2 NFC 刷卡开门（在线验证 fallback）

```
Reader → Gateway: 本地无缓存规则
Gateway → Cloud (NATS verify / HTTPS verify-credential)
Cloud: 查 card → user → group → lock → time_window
Cloud → Gateway: allow / deny
Gateway: allow → 继电器开门 → 上报 access_granted
```

### 6.3 远程解锁（管理员）

```
Admin UI → POST /locks/{id}/unlock → Cloud
Cloud → NATS mistypass.gateway.{id}.command (或 pending_commands in config/pull)
Gateway: 收到 unlock → 继电器开门 → 上报 command_ack
```

### 6.4 Place Lockdown

```
Admin UI → POST /places/{id}/lock_down → Cloud
Cloud → 对 Place 下每扇门发 lock_down 命令
Gateway: 收到 lock_down → 标记门为 lockdown → 拒绝所有本地放行
→ 直到收到 cancel_lockdown
```

---

## 7. 安全要求

| 要求 | NATS 模式 | HTTPS 模式 |
|---|---|---|
| 传输加密 | NATS TLS | HTTPS TLS |
| 设备认证 | NATS credential (token/nkey) | Bootstrap token + device cert |
| 消息签名 | 可选（NATS nkey 已含） | HMAC-SHA256 |
| 重放防护 | request_id + timestamp | idempotency_key + checkpoint |

---

## 8. 下一步实现

| 序号 | 任务 | 依赖 |
|---|---|---|
| 1 | Gateway Bootstrap 增强（config/pull 返回 access rules + pending commands） | 无 |
| 2 | Access Rule 生成器（从 groups/role_assignments/cards 生成缓存包） | 无 |
| 3 | Gateway 端 NATS client（香橙派上的 Go/Python 程序） | 硬件到位 |
| 4 | RS485 继电器驱动（串口 → 继电器板 → 电锁） | 继电器板 + 电锁 + 电源 |
| 5 | Wiegand 输入解析（GPIO → 26/34 bit 解析） | Reader + 香橙派 GPIO |
| 6 | OSDP 输入解析（RS485 → OSDP v2 协议） | OSDP Reader + RS485 |
| 7 | 离线判定引擎（本地 access rule 评估） | #2 完成 |
| 8 | 断网事件队列 + 重连补传 | #7 完成 |
