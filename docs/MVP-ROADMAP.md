# Mistyislet MVP 路线图

> 更新日期：2026-04-29
> 第一优先级：完成 MVP 项目跑通，包括硬件
> 第一理念：让用户更方便、更安全、更高效地开门

---

## 0. 核心理念

**让用户更方便、更安全、更高效地开门。**

所有产品决策和技术排期都服务于这一目标。不能帮助用户开门的功能一律后置。

---

## 1. MVP 定义

一个完整的 MVP 必须跑通以下端到端流程：

```
管理员创建 Place → 绑定硬件(Controller/Reader) → 创建门点(Lock)
→ 创建用户 → 分配权限(Group/RoleAssignment)
→ 用户通过凭证(NFC/BLE/QR)→ Reader 验证 → 门锁打开
→ 事件记录 → 管理员可在后台查看
```

---

## 2. 当前状态诊断

| 环节 | 状态 | 说明 |
|---|---|---|
| 管理员创建 Place/门点/用户/权限 | **已完成** | API + UI 全链路可用 |
| 硬件注册(Controller/Reader) | **API 已完成** | 注册、绑定门点、配置推送 |
| 门锁解锁 API | **仅模拟** | 返回 `accepted` 但不发送硬件命令 |
| NATS 消息总线 | **已配置** | Publisher 存在，但 unlock 不发消息，无 subscriber |
| 硬件通信协议 | **未实现** | RS485/OSDP/Wiegand 数据模型有，通信层无 |
| 凭证验证(NFC/BLE) | **未实现** | BLE token 生成有，Reader 验证逻辑无 |
| 移动端 App | **未实现** | 仅有 `/app/` 少量 endpoint |
| 事件记录 | **API 已完成** | event_sets 可读写 |
| 管理后台 UI | **大部分完成** | 15+ 页面已接真实数据 |

**核心缺口：从 "API 返回 accepted" 到 "门真的打开" 之间的一切。**

---

## 3. MVP 阶段划分

### Phase M1：解锁命令下发（让 API 真正控制门锁）

**目标**：`POST /locks/{id}/unlock` → NATS → Gateway → Door opens

| 序号 | 任务 | 改动范围 | 预估 |
|---:|---|---|---|
| 1 | **Unlock API 发 NATS 消息** | `routes_reference_api.go` 的 `writeReferenceLockAction` → 查 gateway binding → `bus.PublishJSON` | 0.5天 |
| 2 | **Gateway 命令主题设计** | 定义 NATS 主题：`mistypass.gateway.{gatewayID}.command`，消息体：`{command, lock_id, action, request_id}` | 0.5天 |
| 3 | **Gateway 模拟器（NATS subscriber）** | 新建 `cmd/gateway-simulator/main.go`，订阅命令主题，模拟执行并回复结果到 `mistypass.gateway.{gatewayID}.event` | 1天 |
| 4 | **命令结果回收** | API 端订阅 event 主题，写入 access event + 更新 CommandAck | 0.5天 |
| 5 | **Lockdown/Cancel 同步** | Place lockdown 和 cancel 也走 NATS 命令 | 0.5天 |

**验收标准**：启动 API + NATS + gateway-simulator，调用 unlock API，simulator 打印 "UNLOCK door_xxx"，event log 记录 unlock 事件。

### Phase M2：凭证验证（让 Reader 能识别用户）

**目标**：用户出示凭证 → Reader 验证 → 判断权限 → 自动开门

| 序号 | 任务 | 改动范围 | 预估 |
|---:|---|---|---|
| 6 | **凭证验证 API** | `POST /api/v1/gateway/verify-credential` — 接收 reader_id + credential_payload(UID/token)，查 card → user → group → lock binding → time window → 返回 allow/deny | 1天 |
| 7 | **访问策略实时评估** | 复用 `EvaluateSchedule` — 检查 RoleAssignment/Share 的 valid_from/until + TimeWindow + HolidayCalendar + ExceptionDates | 0.5天 |
| 8 | **Gateway simulator 集成验证** | simulator 收到 NFC UID → 调 verify-credential API → 收到 allow → 发 unlock 命令 | 0.5天 |
| 9 | **Access event 自动生成** | 验证成功/失败都写 access event（actor, door, time, result, credential_type） | 0.5天 |

**验收标准**：simulator 模拟 NFC 刷卡，verify API 返回 allow，自动 unlock，event log 完整记录。

### Phase M3：移动端开门（让用户用手机开门）

**目标**：手机 App 扫码/BLE → 门开

| 序号 | 任务 | 改动范围 | 预估 |
|---:|---|---|---|
| 10 | **BLE unlock flow** | `POST /api/v1/app/access/unlock` — 验证 BLE token + 用户权限 → 发 NATS unlock 命令 | 1天 |
| 11 | **QR Code unlock flow** | `POST /api/v1/app/access/qr-unlock` — 验证 QR payload + group_link token → 发 NATS unlock 命令 | 0.5天 |
| 12 | **可访问门点列表** | 增强 `GET /app/access/doors` — 返回用户有权限的门点 + 当前时间窗状态 + lock status | 0.5天 |
| 13 | **移动端 API 文档** | OpenAPI 补 `/app/` 路由的 schema | 0.5天 |

**验收标准**：用 curl 模拟手机请求 BLE unlock，门开，event 记录。

### Phase M4：管理后台 MVP 收口

**目标**：管理员能完成日常运营闭环

| 序号 | 任务 | 改动范围 | 预估 |
|---:|---|---|---|
| 14 | **实时门状态** | 门点列表/详情展示 online/offline + last_seen + lock/unlock 状态 | 1天 |
| 15 | **硬件健康监控** | Hardware 页展示 controller 在线状态、reader 连接状态、最后通信时间 | 0.5天 |
| 16 | **Access Event 实时刷新** | Event History 页自动轮询或 WebSocket 更新 | 0.5天 |
| 17 | **一键 Lockdown 确认** | Place lockdown 走真实 NATS 命令 + UI 状态反馈 | 0.5天 |
| 18 | **用户权限快速预览** | User Detail 页展示"此用户可以开哪些门 + 当前时间窗状态" | 0.5天 |

**验收标准**：管理员登录后台 → 看到门状态/硬件状态 → 手动解锁一扇门 → 看到事件记录。

### Phase M5：硬件对接准备

**目标**：准备好对接真实硬件

| 序号 | 任务 | 改动范围 | 预估 |
|---:|---|---|---|
| 19 | **Gateway 固件通信协议文档** | 定义 NATS 命令/事件的完整协议：command types, payload schema, ack flow, heartbeat | 1天 |
| 20 | **Gateway Bootstrap API** | 增强 `/gateway/bootstrap` — 网关首次联网时拉取配置、绑定门点列表、access rule 缓存 | 1天 |
| 21 | **离线缓存同步** | Gateway 定期从 API 同步 access rule 快照，断网时本地验证 | 2天 |
| 22 | **RS485 协议适配层** | 网关端 RS485 serial 通信 → 控制电磁锁/电插锁开关 | 2天 |
| 23 | **OTA 固件更新** | Gateway 检查固件版本 → 下载 → 安装 → 回报状态 | 1天 |

---

## 4. 优先级总览

| 优先级 | Phase | 预估 | 核心价值 |
|---|---|---|---|
| **P0** | M1 解锁命令下发 | **完成** | API → NATS → Gateway Simulator → Event |
| **P0** | M2 凭证验证 | **完成** | NFC/BLE/QR → verify → 权限 → 自动 unlock |
| **P1** | M3 移动端开门 | **完成** | App unlock/qr-unlock/my-doors API |
| **P1** | M4 管理后台收口 | **完成** | 15s 自动刷新、gateway 状态、accessible doors |
| **P2** | M5 硬件对接准备 | 待推进 | 协议文档、Bootstrap、离线缓存、RS485 |

**M1 + M2 是最小可验证 MVP**，合计约 5.5 天。完成后可以用 gateway-simulator 演示完整的"刷卡开门"流程。

---

## 5. 已完成事项（不在 MVP 关键路径上，但已有价值）

以下事项已在前序会话中完成，属于管理后台和 API 基础设施：

- Holiday Calendar Regions（5国预设 + preset API）
- Invites 独立资源（组织级邀请列表 + cancel/resend）
- User 2FA Self-Service（所有用户可启用 TOTP）
- Organization Settings（General/Communication/Security/Advanced 接通真实 API）
- Schedules 独立资源（Schedule CRUD + 前端管理页）
- 15+ 资源的 Reference API（Places/Locks/Users/Groups/Teams/Roles/Cards 等）
- 双视角导航（Organization Admin / Place Admin）
- i18n 三语 100% 覆盖

这些在 MVP 跑通后将直接投入运营使用。

---

## 6. 不在 MVP 范围内的事项（后置）

- Apple Pass 真实签名 / Google Wallet 真实 API
- 制卡供应商真实 API
- SSO/SCIM 独立配置页
- Alert Policy 渠道升级 / DB 持久化
- OpenAPI 文档站 / 生成 client
- 旧后台归档 / bundle 拆分
- 电梯/无线锁/摄像头等 P2 硬件
- 多租户 Organization Transfer
