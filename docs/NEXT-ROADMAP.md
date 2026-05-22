# Mistyislet 后续推进路线图

> 更新日期：2026-05-19 (v9 — Fingerspot Cloud 集成完成，iOS App 已开发)
> 第一优先级：BLE 移动凭据 MVP（印尼两个工厂客户）— 见第 9 节
> 第一理念：让用户更方便、更安全、更高效地开门
> ⚠️ Apple/Google Wallet 功能因印尼��策原因暂停推进（Apple Pay/Google Pay 在印尼暂不可用）
> 🚀 替代方案：BLE + Android Keystore 自主认证链路（绕过 Apple/Google 生态限制）
> MVP 聚焦路线图见 `docs/MVP-ROADMAP.md`
> Gateway 软件通信安全状态见 `docs/architecture/gateway-security-software-status.md`
> 硬件/BSP 安全后续项见 `docs/architecture/hardware-bsp-followups.md`
> 代码审查报告见 `docs/CODE-REVIEW-2026-05-01.md`

---

## 1. 已完成项

### 后端 API（前序会话）
| 事项 | 状态 |
|---|---|
| P0 非硬件 legacy 全域审计（37 个写操作） | done |
| P1 Users 批量治理（batch-status/delete/invite + CSV 导出导入） | done |
| P1 Alert Policy 事件驱动调度器 + cooldown + 通知查询 | done |
| P1 Access Rights 复杂 schedule（TimeWindow + HolidayCalendar + evaluate） | done |
| P1 Apple/Google Wallet mock provider + 凭证流程文档 | done |
| P2 OpenAPI 20+ 资源字段级 schema | done |

### 前端 UI（前序会话）
| 事项 | 状态 |
|---|---|
| kisi → mistyislet 命名统一（40+ 符号） | done |
| i18n 三语 100% 覆盖（en-US/zh-CN/id-ID，~350 keys） | done |
| Bundle 优化（主入口 -13%，legacy 页面归档） | done |
| 语言切换（Shell + 登录页 shadcn DropdownMenu） | done |
| Place Dashboard 增强（Daily Usage 表格 + Unlock Heatmap） | done |

### 2026-05-01 会话完成项
| 事项 | 类别 | 状态 |
|---|---|---|
| WebAuthn / Passkey 全栈实现（后端 6 端点 + 前端注册/登录/管理） | 安全 | done |
| SSO 前置检查 + MFA 二步联动 | 安全 | done |
| Login Session 管理（后端 3 端点 + 前端 UI） | P3-17 | done |
| 自助密码重置（后端 2 端点 + 前端忘记密码流程） | P3-18 | done |
| Signed Upload URLs（后端 4 端点） | P3-15 | done |
| Guests 独立资源（后端 5 端点 CRUD + check-in/out） | P2-9 | done |
| Visitor Management UI（前端页面 + 导航） | P2-12 | done |
| Gateway 通信协议文档（574 行） | M5-19 | done |
| Config pull 增加 pending OTA tasks | M5-20 | done |
| Gateway OTA 状态回报端点 | M5-21 | done |
| OpenAPI 文档补全（新增 100+ 端点定义） | 文档 | done |
| 数据库自动迁移（WebAuthn 凭证表） | 基础设施 | done |
| Kisi 部分覆盖区域全部补齐（28 个端点 + 前端 UI） | 对齐 Kisi | done |
| favorite/unfavorite 用户偏好（Locks/Places） | 对齐 Kisi | done |
| Readers GET/PATCH/reset_tamper + 前端按钮 | 对齐 Kisi | done |
| Controllers GET/PATCH, Terminals CRUD | 对齐 Kisi | done |
| Members full CRUD, Group Zones CRUD | 对齐 Kisi | done |
| Cards PATCH/DELETE, Card Assignments PATCH/DELETE/activate/deactivate | 对齐 Kisi | done |
| Reports create/delete | 对齐 Kisi | done |
| gateway UpdateDevice/ResetDeviceTamper service 方法 | 后端 | done |
| Kisi 差距分析文档 | 文档 | done |
| Elevators + Elevator Stops + Group Elevator Stops（15 端点） | 对齐 Kisi | done |
| Group Terminals CRUD（3 端点） | 对齐 Kisi | done |
| Presences 在场追踪 | 对齐 Kisi | done |
| CSV Card Imports（3 端点） | 对齐 Kisi | done |
| Locks first_to_arrive / last_to_leave | 对齐 Kisi | done |
| Users self-signup + password change（3 端点） | 对齐 Kisi | done |
| auth CreateUser / ChangePassword service 方法 | 后端 | done |
| 前端：Elevators 页面（CRUD + Stops + lockdown） | 前端 | done |
| 前端：Groups 页增加 Elevator Stops / Terminals tab | 前端 | done |
| 前端：Door 详情加 first_to_arrive / last_to_leave 按钮 | 前端 | done |
| 前端：Credentials 页加 CSV Import 按钮 | 前端 | done |
| 前端：登录页加 Sign Up 表单 | 前端 | done |
| 前端：My Account Security 加 Change Password | 前端 | done |
| 前端：api.ts 补齐 30+ 新增 API 函数 | 前端 | done |

### 2026-05-01 安全审计修复
| 事项 | 类别 | 严重级别 | 状态 |
|---|---|---|---|
| enforceAdminMFA 数据竞争修复（加锁 + TOCTOU 防护） | 安全 | Critical | done |
| WebAuthn 凭证持久化错误处理（注册/登录/删除） | 安全 | Critical | done |
| Upload 下载端点添加签名验证（防匿名下载） | 安全 | Critical | done |
| Upload 签名绑定用户身份（user_id 纳入 HMAC） | 安全 | Critical | done |
| Upload PUT 端点路径穿越防护（filepath.Base 清理） | 安全 | Medium | done |
| WebAuthn 敏感数据从 URL query 移入 POST body | 安全 | High | done |
| 自助注册端点加 SelfRegistrationEnabled 配置开关 | 安全 | High | done |
| 禁用 MFA 要求二次验证（当前 TOTP 码或密码） | 安全 | High | done |
| 空操作组织端点改返回 501 Not Implemented | 安全 | High | done |
| WebAuthn FinishRegistration/Login 预读 body 防锁 DoS | 安全 | High | done |
| CreateUser / ChangePassword TOCTOU 竞争修复 | 安全 | High | done |
| Session 元数据保留（IP/UA/LoginMethod/CreatedAt） | 功能 | Medium | done |
| EnableUserMFA 增加恢复码生成 | 功能 | Medium | done |
| UpdateDevice status/protocol 输入验证 | 安全 | Medium | done |
| Gateway Register() 改用 defer unlock | 质量 | Low | done |
| 生产环境配置校验增加 UploadSigningKey | 安全 | Medium | done |
| auth_users 表 email 列加唯一索引 | 性能 | Low | done |
| 前端 Tab 组件改用语言无关键 | 功能 | Medium | done |
| revokeAllSessions 成功后触发前端登出 | 功能 | Medium | done |
| Demo 账号改为编译时条件排除 | 安全 | Medium | done |
| Visitor/Elevator 页面增加 mutation 错误处理 | 质量 | Low | done |
| WebAuthn/密码重置错误响应屏蔽内部细节 | 安全 | Low | done |
| SQL deleteProjectionRows 加安全注释 | 质量 | Low | done |

### 2026-05-19 会话完成项
| 事项 | 类别 | 状态 |
|---|---|---|
| Fingerspot Cloud REST API 集成（11 端点客户端 + 服务层 + 设备验证） | 南向网关 | done |
| Fingerspot Webhook 接收器（attlog 事件入库 + 审计日志） | 南向网关 | done |
| Fingerspot 管理路由（12 路由：webhook + 11 admin 端点） | 南向网关 | done |
| Fingerspot 配置加载（4 env vars + enable 开关） | 基础设施 | done |
| GatewayDevice Provider 字段（区分 hikvision/zkteco/fingerspot 设备） | 后端 | done |
| 前端 Southbound 页面增加 Fingerspot 选项 | 前端 | done |

---

## 2. 差距总览（vs Kisi API Bundled References，227 operations）

> 基准：`Kisi-API-Bundled References.yaml` (OpenAPI 3.1.0, 227 operations, 17 deprecated) + `https://docs.kisi.io/`
> 详细逐项对比见 `docs/kisi-gap-analysis.md`（含 docs.kisi.io 产品功能差距）
> 代码审查报告见 `docs/CODE-REVIEW-2026-05-01.md`

| 分类 | Kisi Operations | 已覆盖 | 覆盖率 |
|---|---:|---:|---:|
| 核心 CRUD（Places/Locks/Floors/Users/Members/Groups + sub-resources） | 80 | 77 | 96% |
| 凭证（Cards/CardAssignments/CSV Imports） | 22 | 21 | 95% |
| 硬件（Controllers/Readers/Terminals） | 19 | 19 | 100% |
| 事件与报表（Events/Reports/ScheduledReports） | 14 | 14 | 100% |
| 集成（Integrations） | 5 | 5 | 100% |
| 高级硬件（WirelessLocks/ControllerI-O） | 19 | 0 | 0% |
| 电梯（Elevators/Stops/GroupElevatorStops） | 16 | 16 | 100% |
| 组织管理（Organization/Transfers/Certificates） | 14 | 10 | 71% |
| 用户安全（2FA/Password/Signup/Logins） | 16 | 10 | 63% |
| 日历与排程（Schedules/Calendar/Holidays/Regions） | 8 | 8 | 100% |
| 访客与在场（Guests/Presences） | 5 | 5 | 100% |
| 摄像头（Cameras/Video） | 6 | 4 | 67% |
| 文件上传/SCRAM | 2 | 1 | 50% |
| Invites | 1 | 1 | 100% |
| **合计** | **227** | **206** | **91%** |
| 有效（去废弃 17 个） | **210** | **189** | **90%** |

---

## 3. 待推进事项

### P1 — 已全部完成 ✅

| 序号 | 事项 | 状态 |
|---:|---|---|
| 1 | Schedules 独立资源 | done |
| 2 | Organization Settings / Dashboard | done |
| 3 | User 2FA self-service | done |
| 4 | Invites 独立资源 | done |
| 5 | Holiday Calendar Regions | done |

### P2 — 中期（剩余 2 项）

| 序号 | 事项 | 理由 | 预估 | 状态 |
|---:|---|---|---|---|
| 6 | ~~Elevator / Elevator Stops / Group Elevator Stops~~ | ~~电梯门禁场景~~ | ~~3-4 天~~ | done |
| 7 | **Wireless Locks** | 无线锁硬件 | 2 天 | 待做（依赖硬件） |
| 8 | **Controller Inputs / Relays / Wiegands** | 控制器高级 I/O 管理 | 3 天 | 待做（依赖硬件） |
| 9 | ~~Guests 独立资源~~ | ~~统一访客目录~~ | ~~2 天~~ | done |
| 10 | ~~Presences / 容量管理~~ | ~~在场追踪~~ | ~~3 天~~ | done |
| 11 | ~~Group Terminals~~ | ~~Terminal 按组分配~~ | ~~1 天~~ | done |
| 12 | ~~Visitor Management UI~~ | ~~Present/Past Visitors~~ | ~~2 天~~ | done |

### P3 — 长期（剩余 6 项）

| 序号 | 事项 | 理由 | 预估 | 状态 |
|---:|---|---|---|---|
| 13 | ~~Cameras / Video Surveillance~~ | ~~视频监控桩端点~~ | ~~1 天~~ | done（桩，待硬件集成） |
| 14 | ~~Organization Transfers / Certificate Rotation~~ | ~~多租户运营~~ | ~~3 天~~ | done |
| 15 | ~~Signed Upload URLs~~ | ~~文件上传~~ | ~~1 天~~ | done |
| 16 | ~~CSV Card Imports 独立资源~~ | ~~对齐 Kisi API~~ | ~~1 天~~ | done |
| 17 | ~~Login Session 管理~~ | ~~活跃会话~~ | ~~2 天~~ | done |
| 18 | ~~Password Reset~~ | ~~自助密码重置~~ | ~~2 天~~ | done |
| 19 | ~~Apple Pass 真实签名~~ | ~~替换 mock PKCS#7~~ | ~~3 天~~ | ⏸️ 暂停（印尼政策限制） |
| 20 | ~~Google Wallet 真实 API~~ | ~~替换 mock JWT~~ | ~~2 天~~ | ⏸️ 暂停（印尼政策限制） |
| 21 | ~~Alert Policy 渠道升级 + DB 持久化~~ | ~~通知链路完善~~ | ~~3-4 天~~ | done |
| 22 | ~~Company / Place Analytics 报告~~ | ~~图表分析报告~~ | ~~3 天~~ | done |
| 23 | ~~Alarm Schedule 周历视图~~ | ~~告警排程周历~~ | ~~2 天~~ | done |

### MVP M5 — 硬件对接准备（剩余 2 项）

| 序号 | 事项 | 理由 | 预估 | 状态 |
|---:|---|---|---|---|
| 19 | ~~Gateway 通信协议文档~~ | ~~协议形式化~~ | ~~1 天~~ | done |
| 20 | ~~Bootstrap API 增强（OTA 发现）~~ | ~~Config pull 集成~~ | ~~1 天~~ | done |
| 21 | ~~OTA 状态回报端点~~ | ~~Gateway device token~~ | ~~0.5 天~~ | done |
| 22 | **RS485 协议适配层** | 网关端串口通信 | 2 天 | 待做（依赖硬件） |
| 23 | **OTA Gateway 固件侧** | 下载/验证/安装 | 1 天 | 待做（依赖硬件） |

---

## 4. 剩余待做项外部需求明细

以下所有待做项均依赖外部资源（账号、硬件、许可证），纯软件工作已全部完成。

### 4.1 Apple Pass 真实签名

**目标**：替换 mock PKCS#7 签名，使 Apple Wallet 能真正添加和更新通行证。

| 需求 | 说明 | 获取方式 | 费用 |
|------|------|---------|------|
| Apple Developer Program 会员 | 创建 Pass Type ID 和证书的前提 | [developer.apple.com/programs](https://developer.apple.com/programs/) 注册 | $99/年 |
| Pass Type ID | 每个 pass 类型一个标识符 | Developer Portal → Certificates, IDs & Profiles → Identifiers → Pass Type IDs | 免费（含在会员内） |
| Pass Type Certificate (.p12) | 用于 PKCS#7 签名 pass 包 | Developer Portal → Certificates → Create → Pass Type ID Certificate，用 CSR 申请，下载后导出 .p12 | 免费 |
| Apple WWDR 中间证书 | 证书链验证 | [apple.com/certificateauthority](https://www.apple.com/certificateauthority/) 下载 G4 证书 | 免费 |
| APNs 推送证书/Key | pass 更新推送通知 | Developer Portal → Keys → Create → APNs | 免费 |

**配置方式**（完成后设置以下环境变量）：
```
APPLE_PASS_TYPE_ID=pass.com.mistypass.access
APPLE_PASS_CERTIFICATE_PATH=/secrets/pass-cert.p12
APPLE_PASS_CERTIFICATE_PASSWORD=xxx
APPLE_WWDR_CERTIFICATE_PATH=/secrets/AppleWWDRCAG4.cer
APPLE_APNS_KEY_PATH=/secrets/AuthKey_XXXXXXXXXX.p8
APPLE_APNS_KEY_ID=XXXXXXXXXX
APPLE_APNS_TEAM_ID=YYYYYYYYYY
```

**代码改动点**：`api/internal/modules/wallet/` 中替换 mock provider 的 `Sign()` 和 `Push()` 实现。

---

### 4.2 Google Wallet 真实 API

**目标**：替换 mock JWT 签名，使 Google Wallet 能添加和管理通行证。

| 需求 | 说明 | 获取方式 | 费用 |
|------|------|---------|------|
| Google Cloud 项目 | API 调用基础 | [console.cloud.google.com](https://console.cloud.google.com/) 创建项目 | 免费 |
| Google Wallet API 启用 | 在项目中启用 API | Cloud Console → APIs & Services → Enable → Google Wallet API | 免费 |
| Service Account + JSON Key | RS256 签名 JWT | Cloud Console → IAM → Service Accounts → Create → Download JSON key | 免费 |
| Google Wallet Issuer Account | 发卡方注册 | [pay.google.com/business/console](https://pay.google.com/business/console) → 注册为 Issuer | 免费，需审核 |
| Corporate Badge API 访问（可选） | 企业工牌功能 | 需联系 Google 签署 NDA + Terms of Service | 免费，审核周期 2-4 周 |

**配置方式**：
```
GOOGLE_WALLET_SERVICE_ACCOUNT_KEY_PATH=/secrets/gcp-wallet-sa.json
GOOGLE_WALLET_ISSUER_ID=3388000000012345678
GOOGLE_WALLET_CLASS_SUFFIX=mistypass_access_v1
```

**代码改动点**：`api/internal/modules/wallet/` 中替换 mock provider 的 `CreateObject()` 和 `SignSaveLink()` 实现。

---

### 4.3 RS485 协议适配层（网关端）

**目标**：网关通过 RS485 串口与继电器/读卡器通信。

| 需求 | 说明 | 参考型号 | 预算 |
|------|------|---------|------|
| Orange Pi Zero3 或 Raspberry Pi 4B | 网关主控板 | Orange Pi Zero3 (1GB) | ~$20 |
| USB-RS485 转换器 | 串口通信适配 | CH340/FT232 USB-RS485 模块 | ~$3-5 |
| RS485 Modbus 继电器模块 | 控制电锁 | 4 路 Modbus RTU 继电器 | ~$10-15 |
| 12V/24V DC 电磁锁 | 测试门禁 | 单门电磁锁 | ~$15-25 |
| 12V DC 电源适配器 | 供电 | 12V 3A 开关电源 | ~$5 |

**总预算**：~$55-70（一套最小测试环境）

**代码改动点**：`api/cmd/gateway-agent/` 中添加 RS485 串口驱动，实现 Modbus RTU 协议读写。

---

### 4.4 OTA Gateway 固件更新

**目标**：网关自动下载、验证、安装固件更新。

| 需求 | 说明 |
|------|------|
| 同 4.3 的硬件 | 需要实际网关硬件进行测试 |
| 固件存储服务器 | 可复用现有 Upload 签名 URL 系统 |
| 代码签名密钥对 | Ed25519 或 RSA，用于固件包签名验证 |

**代码改动点**：`api/cmd/gateway-agent/` 中添加 OTA 下载/校验/重启逻辑，对接已有的 `/gateway/ota/report` 端点。

---

### 4.5 Wireless Locks API

**目标**：支持蓝牙/WiFi 无线智能锁。

| 需求 | 说明 | 参考型号 | 预算 |
|------|------|---------|------|
| BLE 智能锁 | 支持标准 BLE GATT 协议的门锁 | Tuya WiFi/BLE 智能锁 | ~$50-80 |
| BLE USB 适配器 | 网关蓝牙通信（如主控板无内置 BLE） | USB BLE 5.0 dongle | ~$5 |

**代码改动点**：`api/internal/modules/gateway/service.go` 新增 WirelessLock 资源类型 + CRUD 端点。

---

### 4.6 Controller I/O 管理

**目标**：管理控制器的输入/输出、继电器和 Wiegand 端口。

| 需求 | 说明 | 参考型号 | 预算 |
|------|------|---------|------|
| 门禁控制器板 | 带 Wiegand 输入 + 继电器输出 | ZKTeco C3-100/200 或同类 | ~$60-100 |
| Wiegand 读卡器 | 26/34 bit Wiegand 输出 | EM/Mifare 读头 | ~$15-25 |
| 接线材料 | 杜邦线、端子排等 | — | ~$5-10 |

**代码改动点**：`api/internal/modules/gateway/service.go` 新增 ControllerInput/Relay/Wiegand 资源 + CRUD 端点。

---

### 4.7 Camera 真实集成

**目标**：对接 IP 摄像头实时视频流和录像。

| 需求 | 说明 | 参考型号 | 预算 |
|------|------|---------|------|
| ONVIF 兼容 IP 摄像头 | 标准视频监控协议 | 海康威视/大华 ONVIF 摄像头 | ~$30-60 |
| RTSP 流处理能力 | 需要 FFmpeg 或 GStreamer | 服务端软件 | 免费 |
| 存储空间 | 录像存储 | 本地 NAS 或 S3 | 视容量而定 |

**代码改动点**：`api/internal/http/routes_cameras.go` 替换 501 桩，实现 ONVIF 发现 + RTSP 流代理。

---

### 优先级建议（2026-05-02 v5 更新）

```
🔴 最高优先级：
1. BLE 移动凭据 MVP（第 9 节完整计划）        → 立即启动，目标 7 月中旬交付

🟡 并行推进：
2. RS485 + 电磁锁集成（EM Lock 已采购）     �� 到货后立即推进（BLE MVP 依赖）
3. 海康/ZKTeco 门禁适配（Phase 1D）          → 与 BLE 并行
4. Camera 真实集成（DS-2CD1023G2-LIU 已采购）→ 到货后推进

🟢 后续：
5. OTA Gateway 固件侧                       → BLE MVP 稳定后
6. Wireless Locks / Controller I/O           → 视产品规划
---
⏸️ Apple + Google Wallet                     → 暂停（印尼政策限制，BLE 方案替代）
```

## 5. 执行计划（2026-05-01 代码审查后制定）

> 来源：`docs/CODE-REVIEW-2026-05-01.md` + `docs/kisi-gap-analysis.md`
> 纯软件工作量 ~19 天 | 含账号依赖 +13 天 | 含硬件依赖 +11 天

### 阶段 1 — 安全加固 + 快速补齐（~3 天，无外部依赖）

| 序号 | 事项 | 位置 | 预估 | 状态 |
|---:|------|------|------|------|
| S1 | ~~限制 bootstrap token 仅用于注册/激活端点~~ | `router.go` | 0.5 天 | done |
| S2 | ~~Gateway Agent 默认启用标准 TLS 验证~~ | `agent.go` | 0.5 天 | done |
| S3 | ~~CORS 生产环境拒绝 `*` 通配符~~ | `router.go` | 0.5 天 | done |
| K1 | ~~补 `GET /group_locks/{id}`~~ | `routes_reference_api.go` | 0.5h | done |
| K2 | ~~补 `GET /group_elevator_stops/{id}`~~ | `routes_reference_kisi_full.go` | 0.5h | done |
| K3 | ~~补 `GET /group_terminals/{id}`~~ | `routes_reference_kisi_full.go` | 0.5h | done |
| K4 | ~~补 `GET /team_memberships/{id}`~~ | `routes_reference_api.go` | 0.5h | done |
| K5 | ~~补 `DELETE /user` (self-delete)~~ | `routes_auth.go` | 0.5h | done |
| K6 | ~~补 `POST /card_assignments/{token}/activate_with_token`~~ | `routes_reference_gap.go` | 0.5h | done |
| D3 | ~~Docker Compose 改用 `.env.example`~~ | `docker-compose.yml` | 0.5 天 | done |

### 阶段 2 — 测试补全 + 代码质量（~5 天，无外部依赖）

| 序号 | 事项 | 位置 | 预估 | 状态 |
|---:|------|------|------|------|
| T1 | ~~Redis store 单元测试~~ | `redistore/store_test.go` (60 tests) | 1 天 | done |
| T2 | ~~Wallet provider 测试~~ | `apple_pass_provider_test.go` + `google_wallet_provider_test.go` | 1 天 | done |
| T3 | ~~E2E 测试纳入 CI~~ | `web-admin-ci.yml` 加 Playwright step | 0.5 天 | done |
| T4 | ~~Tenant 模块测试~~ | `service_test.go` 扩展到 7 tests | 1 天 | done |
| Q1 | ~~拆分 `api.ts` 为 20 个领域模块~~ | `web-admin/src/lib/api/` (21 files) | 1 天 | done |
| Q2 | ~~建立 React Query key 工厂~~ | `web-admin/src/lib/query-keys.ts` | 0.5 天 | done |

### 阶段 3 — 功能对齐（~5 天，无外部依赖，可与阶段 2 并行）

| 序号 | 事项 | 来源 | 预估 | 状态 |
|---:|------|------|------|------|
| F1 | ~~报表 PDF 导出~~ | `routes_reports.go` `?format=pdf` | 1 天 | done |
| F3 | ~~Organization Dashboard 聚合端点~~ | `GET /organization/dashboard` | 0.5 天 | done |
| F4 | ~~内置 Incident Policy: Door Held Open~~ | `referenceIncidentAlertPolicies` seed | 1 天 | done |
| F5 | ~~内置 Incident Policy: Hardware Outage~~ | `referenceIncidentAlertPolicies` seed | 1 天 | done |
| F7 | ~~Camera updateCamera + fetchVideoLink~~ | `routes_cameras.go` stubs | 0.5 天 | done |
| K7 | ~~`GET /organizations/{domain}/public`~~ | `getPublicOrganization` | 1h | done |
| K8 | ~~`POST /organizations/find`~~ | `findOrganizations` | 1h | done |

### 阶段 4 — 凭证真实化（暂停 ⏸️）

> **暂停原因（2026-05-02）：** Apple Pay 和 Google Pay 因印尼政府政策原因暂不可用，相关企业功能无法在目标市场部署和测试。Mock provider 保留在代码库中，待政策开放后恢复推进。

| 序号 | 事项 | 预估 | 条件 | 状态 |
|---:|------|------|------|------|
| A1 | Apple Pass 真实签名 | 3 天 | Apple Developer Program $99/年 | ⏸️ 暂停（印尼政策限制） |
| A2 | Google Wallet 真实 API | 2 天 | GCP 项目 + Issuer Account（免费） | ⏸️ 暂停（印尼政策限制） |

### 阶段 5 — 硬件对接（~3 天，硬件采购中 🛒）

> **2026-05-02 更新：** 已采购以下硬件，到货后即可开始集成：
> - **EM Lock 600 LBS** (280KG 电磁锁, 12VDC/400mA, Type B 五线制 NO/NC/COM)
> - **Hikvision DS-2CD1023G2-LIU** (2MP PoE 网络摄像头, ISAPI+ONVIF)

| 序号 | 事项 | 预估 | 条件 | 状态 |
|---:|------|------|------|------|
| H1 | RS485 协议适配层 | 2 天 | Orange Pi + USB-RS485 + 继电器 + 电磁锁 | 待做（电磁锁已采购） |
| H2 | OTA Gateway 固件侧 | 1 天 | 同 H1 硬件 + Ed25519 签名密钥 | 待做 |
| H5 | Camera 真实集成 | 3 天 | DS-2CD1023G2-LIU 已采购 | 待做（硬件已采购） |

### 阶段 6 — 企业能力（~8 天，需账号）

| 序号 | 事项 | 预估 | 条件 | 状态 |
|---:|------|------|------|------|
| A3 | SCIM 2.0 用户供给 | 5 天 | Okta Developer / Entra ID 免费层 | 待做 |
| ~~A4~~ | ~~OAuth2 Authorization Code 流~~ | ~~3 天~~ | ~~纯软件~~ | done (2026-05-02) |

### 后续视产品规划

| 序号 | 事项 | 预估 | 条件 | 状态 |
|---:|------|------|------|------|
| ~~F2~~ | ~~报表邮件推送~~ | ~~1 天~~ | ~~SMTP / Resend API Key~~ | done (2026-05-02) |
| ~~F6~~ | ~~网络拓扑前端可视化~~ | ~~2 天~~ | ~~后端 `/network/topology` 已有~~ | done (2026-05-02) |
| D1 | 生产 HA 部署文档 | 1 天 | 纯文档 | 待做 |
| D2 | 生产 WAF 指南 | 0.5 天 | 纯文档 | 待做 |
| P1 | GPS Geofence 限制 | 2 天 | 需产品决策 | 待做 |
| P3 | 访客管理增强 (NDA/Kiosk) | 5 天 | 需产品决策 | 待做 |
| H3 | Controller I/O (19 operations) | 3 天 | 需控制器硬件 ~$110 | 待做 |
| H4 | Wireless Locks | 2 天 | 需 BLE 智能锁 ~$65 | 待做 |

---

## 7. 印尼本地化推进计划（2026-05-02）

> 来源：Kisi API 对比分析 + 印尼市场调研
> 目标：使 MistyPass 成为印尼市场最适配的云门禁 SaaS

### 7.1 前端 API 绑定审计

> 审计日期：2026-05-02
> 后端 469 端点 / 前端 21 个 API 模块 314 个导出函数 / **52 个函数 (16.6%) 未被前端引用**

#### 未绑定前端的功能领域（13 个差距）

| 差距 | 后端 API | 前端状态 | 优先级 |
|------|---------|---------|--------|
| 假日日历管理 | 6 个端点完整 | **0% — 无 UI** | P1（印尼假日同步依赖） |
| 报表排程与详情 | 5 个端点 | 仅列表，无创建/编辑/详情 | P1 |
| 访客通行证 | 2 个端点 | 仅访客 CRUD，无通行证签发/列表 | P2 |
| 告警策略详情与评估 | 4 个端点 | 仅列表，无详情/测试 | P2 |
| 组-电梯/终端管理 | 6 个端点 | 无 UI | P2 |
| 电梯详情编辑 | 2 个端点 | 仅列表 | P3 |
| 事件元数据与筛选 | 3 个端点 | 基础列表 | P3 |
| 访问权限排程预览 | 1 个端点 | 无预览 UI | P3 |
| 摄像头集成 | listCameras 等 | 完全未使用 | P2（硬件到货后） |
| Access User 完整 CRUD | 6 个端点 | 前端走其他用户创建流程 | 评估后决定 |
| RS485 遥测上报 | 1 个端点 | 无遥测仪表盘 | P3 |
| 团队详情 | getTeam | 无详情页 | P3 |
| HRIS 密钥管理 | upsertEnterpriseHRISSecret | 无 UI | P3（HRIS 后置） |

### 7.2 印尼本地化 — 立即可做

#### ID-1: 运行时区域配置

| 项目 | 说明 |
|------|------|
| `TZ=Asia/Jakarta` | docker-compose + .env.example |
| 租户 timezone 字段 | 影响访问规则时间窗口判断 |
| 前端 id-ID locale 格式化 | 日期 dd/MM/yyyy、数字千分位用点 |

#### ID-2: 印尼国定假日数据源

利用已有的 Holiday Calendar 7 端点 + 前端 UI（待建），接入印尼政府假日数据。印尼每年 15-20 个假日（含伊斯兰历浮动假日），需动态更新。

#### ID-3: UU PDP 合规文档

印尼《个人数据保护法》(UU No.27/2022) 已于 2024-10 全面生效。MistyPass 自托管模式是核心合规优势。需新增：
- 数据主体权利端点 (`GET /me/data-export`)
- 数据泄露通知 workflow
- 合规文档（面向客户的 UU PDP 声明）

#### ID-4: 访客登记增强 (Buku Tamu)

印尼企业强需求：KTP 身份证号采集 + WhatsApp 通知被访人 + 签到/签退记录导出。

### 7.3 祈祷室/会议室预约系统 (Bookings)

> 产品形态：可选插件，不限访问时间，按空间容量管理

#### 功能设计

```
Space Booking 预约系统
├── 空间类型
│   ├── meeting_room（会议室）
│   ├── prayer_room（祈祷室）
│   ├── phone_booth（电话亭）
│   └── custom（自定义）
├── 容量模式
│   ├── single_occupancy — 单人专用，占用时锁定
│   ├── limited_capacity — 多人但有上限（如祈祷室 max=10）
│   └── unlimited — 无上限，仅统计当前人数
├── 状态感知
│   ├── 门禁事件驱动：刷卡进入 → occupied +1，刷卡离开 → occupied -1
│   ├── 实时占用率：当前人数 / 最大容量
│   └── 门口显示屏（未来）：绿灯=可用，红灯=满员
├── 预约流程
│   ├── 用户通过 App/Web 预约时段
│   ├── 预约时段内自动授予门禁权限
│   ├── 超时未到 → 自动释放 + 通知
│   └── 提前离开 → 门禁事件触发自动释放
└── 管理功能
    ├── 启用/禁用预约（祈祷室可设为随到随用，无需预约）
    ├── 容量配置
    ├── 使用率统计报表
    └── 高峰时段热力图
```

#### API 端点设计

```
POST   /api/v1/bookings                    — 创建预约
GET    /api/v1/bookings                    — 查询预约列表
GET    /api/v1/bookings/{id}               — 预约详情
PATCH  /api/v1/bookings/{id}               — 修改预约
DELETE /api/v1/bookings/{id}               — 取消预约
POST   /api/v1/bookings/{id}/check-in      — 签到
POST   /api/v1/bookings/{id}/check-out     — 签退
GET    /api/v1/bookable-spaces             — 可预约空间列表
POST   /api/v1/bookable-spaces             — 创建可预约空间
PATCH  /api/v1/bookable-spaces/{id}        — 更新空间配置
DELETE /api/v1/bookable-spaces/{id}        — 删除可预约空间
GET    /api/v1/bookable-spaces/{id}/status — 实时占用状态
GET    /api/v1/bookable-spaces/{id}/usage  — 使用率统计
```

预估：后端 3-4 天 + 前端 2-3 天 = **~6 天**

### 7.4 支付网关评估

> 调研日期：2026-05-02 | 仅在 SaaS 订阅收费场景下需要

#### 费率对比

| 支付方式 | Midtrans | Xendit | DOKU |
|----------|----------|--------|------|
| 信用卡 | 2.9% + Rp2,000 | 2.9% + Rp2,500 | 定制（可谈） |
| E-wallet (GoPay/OVO/Dana) | 1.5%-2% | 1.5%-2% | 定制 |
| Virtual Account (银行转账) | Rp4,000/笔 | Rp4,000/笔 | Rp3,500-4,500/笔 |
| QRIS (二维码) | 0.7% | 0.7% | 定制 |
| 结算周期 | T+1 ~ T+3 | T+1 ~ T+2 | T+1 ~ T+5 |
| 注册到上线 | 1-2 周 | 1-2 周 | 3-6 周 |
| 月费 | 无 | 无（企业版 $50/月） | Rp500K-2M |
| 退款手续费 | Rp50K-150K | ~Rp230K | Rp100K-200K |

#### 评估结论

| 维度 | 推荐 | 原因 |
|------|------|------|
| **SaaS 订阅** | **Xendit** | API 最友好、Dashboard 最佳、支持多国扩展、结算最快 |
| API 文档质量 | Xendit > Midtrans > DOKU | Xendit 英文文档完整，REST 风格一致 |
| 印尼本土覆盖 | 三家均可 | 均持有 Bank Indonesia 牌照 |
| 高量折扣 | DOKU | 大客户可谈到比竞品低 20-40% |
| 入门门槛 | Midtrans | 中文教程多，适合快速起步 |

**建议：MVP 阶段选 Xendit**，原因：
1. 无月费，按交易计费
2. API 设计现代，与 MistyPass 技术栈契合
3. 支持 VA + QRIS + E-wallet + 信用卡全渠道
4. 结算 T+1~T+2，现金流最优
5. 支持新加坡/菲律宾/马来西亚扩展

### 7.5 WhatsApp Business API 集成商评估

> MistyPass 已实现 Meta WhatsApp Business API 直连（`alert_whatsapp_provider.go`）
> 以下为 BSP（Business Solution Provider）选项，用于获取官方 API 接入和号码验证

#### Meta 官方费率（印尼，2026）

| 消息类型 | 费率/条 | 说明 |
|----------|---------|------|
| Marketing | Rp586 (~$0.036) | 营销推广模板 |
| Utility | Rp357 (~$0.022) | 交易通知（门禁事件等） |
| Authentication | Rp357 (~$0.022) | OTP 验证码 |
| Service | **免费** | 客户发起的对话（24h 内回复） |

> 注：2025-07-01 起 Meta 改为按条计费（非会话制）

#### 印尼 BSP 对比

| 供应商 | 月费 | 消息加价 | 官方 BSP | 适用场景 | 本地支持 |
|--------|------|---------|---------|---------|---------|
| **直连 Meta Cloud API** | 免费 | 无 | — | 技术团队自建 | 无 |
| **Qontak (Mekari)** | ~Rp750K/用户 | 含 Rp175K 消息余额 | 是 | CRM + 全渠道 | 印尼本土 |
| **Barantum** | Rp897K/月 | Meta 原价 | 是 | 客服+销售 | 印尼本土 |
| **WhatsBoost** | ~Rp132K/月 | 无限消息（声称） | 否 | 低成本入门 | 印尼本土 |
| **Wati** | $49/月起 | 有加价 | 是 | 无代码自动化 | 全球 |
| **Respond.io** | 按用量 | **无加价** | 是 | 多渠道+AI | 全球 |
| **Twilio** | 按用量 | 有加价 | 是 | 开发者/企业 | 全球 |
| **360dialog** | 定制 | 无加价 | 是 | 快速接入 | 全球 |

#### 集成建议

MistyPass 已直连 Meta Cloud API，**不需要 BSP 中间商**。但需要：

1. **号码验证**：通过 Meta Business Manager 注册 WhatsApp Business 号码（免费）
2. **模板审核**：提交消息模板供 Meta 审核（门禁通知属 Utility 类，Rp357/条）
3. **费用预估**：
   - 100 个用户 × 平均 5 条通知/天 × Rp357 = **Rp178,500/天 ≈ Rp5.4M/月 (~$337)**
   - Service 类消息免费，鼓励用户主动发起对话

**如果未来需要 CRM + 客服能力**，推荐 **Qontak (Mekari)**：
- 与 Talenta 同属 Mekari 生态，可能有捆绑折扣
- 印尼本土支持，中文/印尼语客服
- 含 CRM + 工单 + 全渠道收件箱

### 7.6 HRIS 生态扩展（降低优先级）

> 2026-05-02 决策：Talenta 已完成，其余供应商后置。待首批客户落地后根据需求再推进。

| 供应商 | 优先级 | 状态 |
|--------|--------|------|
| Talenta (Mekari) | — | **已完成** |
| Gadjian | P3（原 P1） | 后置 |
| GreatDay HR | P3（原 P1） | 后置 |
| LinovHR | P4（原 P2） | 后置 |
| SunFish HR | P4（原 P2） | 后置 |

### 7.7 印尼本地化执行计划

| 阶段 | 序号 | 事项 | 预估 | 状态 |
|------|---:|------|------|------|
| **立即** | ID-1 | TZ=Asia/Jakarta + 前端 locale 格式化 | 0.5 天 | done (2026-05-02) |
| **立即** | ID-2 | 假日日历前端 UI + 印尼假日数据源 | 2 天 | done (2026-05-02) |
| **本周** | ID-3 | 前端未绑定 API 补齐（报表排程、告警详情、组电梯/终端） | 3 天 | done (2026-05-02) |
| **本周** | ID-4 | UU PDP 合规声明文档 | 1 天 | done (2026-05-02) |
| **短期** | ID-5 | 访客登记增强（必填姓名+电话，可选KTP/KITAS/ITAS，WhatsApp通知） | 2 天 | done (2026-05-02) |
| **短期** | ID-6 | Bookings 预约系统（祈祷室/会议室/电话亭，13 端点 + 前端） | 6 天 | done (2026-05-02) |
| **中期** | ID-7 | Xendit 支付集成（如需 SaaS 收费） | 3 天 | 待做 |
| **后置** | ID-8 | HRIS 其余供应商（Gadjian 等） | — | 后置 |

### 7.8 第三方办公 SaaS 集成（2026-05-03）

> 目标：与客户办公平台打通，实现员工入离职自动同步门禁凭据

| 平台 | 优先级 | 集成内容 | 状态 |
|------|--------|---------|------|
| **WhatsApp Business** | P0 | 告警通知（模板消息 + 纯文本） | ✅ 已完成 + 测试通过 |
| **Lark / 飞书** | P1 | Bot 告警通知 + 通讯录同步 + 事件订阅 | ✅ 代码完成，待客户确认 |
| **Google Workspace** | P1 | Directory API 员工同步 | ✅ 代码完成，待客户有 GWS |
| DingTalk / 钉钉 | P2 | 待客户需要时开发 | 待做 |
| Microsoft Teams | P2 | 待大型外企客户 | 待做 |

#### 集成文件索引

| 文件 | 用途 |
|------|------|
| `api/internal/modules/integration/lark_client.go` | Lark API 客户端（自动 token 管理） |
| `api/internal/modules/integration/lark_bot.go` | Lark Bot 消息（文本 + 告警卡片 + 访客通知） |
| `api/internal/modules/integration/lark_contact.go` | Lark 员工目录同步 + 事件订阅 |
| `api/internal/modules/integration/google_workspace.go` | Google Workspace Directory API |
| `api/internal/http/routes_integration_lark.go` | Lark 事件回调 + Bot 测试 + 同步端点 |
| `api/internal/modules/wallet/alert_lark_provider.go` | Lark Bot 告警调度器适配 |
| `docs/integrations/lark-integration.md` | Lark 集成完整方案文档 |
| `docs/integrations/google-workspace-integration.md` | Google Workspace 集成方案文档 |

#### 环境变量

```bash
# WhatsApp 模板消息（模板审核通过后启用）
WALLET_ALERT_WHATSAPP_TEMPLATE_NAME=access_denied_alert
WALLET_ALERT_WHATSAPP_TEMPLATE_LANG=en_US

# Lark Bot 告警（在 Lark 群创建自定义机器人后配置）
LARK_ALERT_WEBHOOK_URL=https://open.larksuite.com/open-apis/bot/v2/hook/xxx

# Lark 应用（通讯录同步需要）
LARK_APP_ID=cli_xxxxxxxxxx
LARK_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxx
```

---

## 9. BLE 移动凭据系统 — 代码级开发计划（2026-05-02）

> 来源：`Indonesia_SaaS_Access_Control_Architecture.md` 方案评估
> 目标：为印尼两个工厂客户交付 BLE 手机开门 MVP
> 核心思路：在现有 gateway-agent + 云端验证架构上新增一条 BLE 认证通道
> 当前 BLE 状态：仅有静态 token 匹配（`ble_token` 类型），无密码学挑战-响应
>
> ⚠️ **2026-05-03 状态更新：**
> - Phase 1A（云端 PKI）：✅ 完成，16 单元测试 + 3 集成测试通过
> - Phase 1B（Gateway BLE）：✅ 完成，15 协议/验证测试通过，TCP 模拟器可用
> - Phase 1C（Android App）：✅ **2026-05-03 小米 15 真机测试通过**（Cloud 开门 + BLE 开门均 GRANTED）
> - Phase 1D（海康/ZKTeco）：✅ 完成，4 API 端点 + Digest Auth + Push Event 解析
> - 前端 Mobile Credentials 页面：✅ 完成
> - 前端 Southbound 设备管理页面：✅ 完成
> - OpenAPI 文档收录（全部 20 个新端点）：✅ 完成
> - Lark 集成（Bot + 通讯录同步 + 事件订阅）：✅ 代码完成，待客户确认使用 Lark 后配置
> - WhatsApp Business API：✅ **2026-05-03 测试通过**（Meta Cloud API，Permanent Token，7 个模板已提交审核）
> - WhatsApp 模板消息集成到告警调度器：✅ 完成（模板审核通过后加一行 env 即启用）
> - Lark Bot 接入告警调度器：✅ 完成（配置 webhook URL 即可同时发 WhatsApp + Lark）
> - Google Workspace Directory 同步：✅ 代码完成（Service Account JWT + Directory API），待客户有 GWS 时配置

### 9.1 现有代码基础（可复用）

| 组件 | 文件 | 可复用内容 |
|------|------|-----------|
| Agent 认证入口 | `api/cmd/gateway-agent/agent.go:399` | `HandleCredentialPresented()` — BLE 读头调用此函数即可接入 |
| 本地规则验证 | `api/cmd/gateway-agent/agent.go:365` | `VerifyCredential()` — 需扩展支持公钥签名验证 |
| 继电器控制 | `api/cmd/gateway-agent/relay.go` | GPIO / RS485 驱动完整可用 |
| 云端验证 | `api/internal/http/routes_gateway_verify.go:208` | `ble_token` case — 需替换为签名验证 |
| App 端点 | `api/internal/http/routes_app_access.go` | `appUnlockDoor` / `appAccessMyDoors` — 直接复用 |
| BLE Token 端点 | `api/internal/http/router.go:1246` | `appAccessBLEToken` — 需重构为密钥注册 |
| Bus 消息 | `api/internal/bus/commands.go:39` | `CredentialType: "ble_token"` — 扩展为 `"ble_signature"` |
| 事件上报 | `api/cmd/gateway-agent/agent.go:431` | `queueEvent` — 直接复用 |

### 9.2 架构变更概览

```
现有流程（静态 token）：
  App 获取 ble_token → Gateway 匹配字符串 → allow/deny

目标流程（密码学签名）：
  App 生成 EC P-256 密钥对（Keystore）→ 公钥注册到云端
  → 云端下发公钥至 Gateway 本地缓存
  → 开门时：Gateway BLE 广播 → App 连接 → 双向认证（Nonce 签名）
  → Gateway 本地用公钥验签 → allow/deny → 开门
```

### 9.3 代码变更清单

---

#### Phase 0 — 硬件准备与验证（1 周）

| 序号 | 事项 | 说明 |
|---:|------|------|
| B0-1 | BLE 硬件选型确认 | ESP32-C3/S3 作为 BLE 读头（独立 MCU），或 Orange Pi + USB BLE dongle |
| B0-2 | 工厂客户需求确认 | 门数量、人数、设备型号（海康/ZKTeco 具体型号）、网络条件 |
| B0-3 | Android BLE 后台保活测试 | 用目标机型（三星 A 系列、小米 Redmi）验证 BLE 扫描可靠性 |
| B0-4 | 硬件采购 | ESP32-S3 开发板 ×2 + USB BLE 5.0 dongle ×2（约 $20） |

**关键决策：BLE 读头方案**

| 方案 | 优点 | 缺点 | 建议 |
|------|------|------|------|
| A: Orange Pi + USB BLE dongle | 复用现有 gateway-agent 代码 | USB BLE 适配器性能一般，Go BLE 库生态弱 | MVP 首选 |
| B: ESP32-S3 独立 BLE 读头 | BLE 性能最优，可量产 | 需 C/MicroPython 固件，增加复杂度 | Phase 4 量产时选 |
| C: nRF52840 专业 BLE SoC | 行业标准，最低功耗 | 开发成本最高 | 长期产品化 |

**MVP 建议选方案 A**：Gateway Agent 直接通过 USB BLE dongle 作为 GATT Server，Go 使用 `tinygo.org/x/bluetooth` 库。

---

#### Phase 1A — 云端 PKI 凭据服务（2-3 周）

**新建模块**：`api/internal/modules/credential/`

```go
// api/internal/modules/credential/service.go

// CredentialService manages mobile credential lifecycle (keypair registration,
// certificate signing, revocation, and public key distribution to gateways).
type CredentialService struct {
    store        Store
    rootCAKey    crypto.PrivateKey  // EC P-256, loaded from KMS/env
    rootCACert   *x509.Certificate
    logger       *slog.Logger
}

// RegisterDevice validates Device Attestation and stores user public key.
func (s *CredentialService) RegisterDevice(input RegisterDeviceInput) (*MobileCredential, error)

// SignCredential issues a short-lived certificate for the user's public key.
func (s *CredentialService) SignCredential(tenantID, userID string) (*SignedCredential, error)

// RevokeCredential immediately marks credential as revoked.
func (s *CredentialService) RevokeCredential(tenantID, credentialID string) error

// ListActiveCredentials returns all active credentials for gateway sync.
func (s *CredentialService) ListActiveCredentials(tenantID string) []MobileCredential

// VerifyBLESignature verifies a BLE challenge-response signature against stored public key.
func (s *CredentialService) VerifyBLESignature(userID string, nonce, signature []byte) (bool, error)
```

```go
// api/internal/modules/credential/model.go

type MobileCredential struct {
    ID              string    `json:"id"`
    TenantID        string    `json:"tenant_id"`
    UserID          string    `json:"user_id"`
    PublicKeyPEM    string    `json:"public_key_pem"`     // EC P-256 公钥
    DeviceID        string    `json:"device_id"`          // 手机设备标识
    Platform        string    `json:"platform"`           // "android" | "ios"
    AttestationData string    `json:"attestation_data"`   // Device Attestation 原始数据
    KeystoreLevel   string    `json:"keystore_level"`     // "strongbox" | "tee" | "software"
    Status          string    `json:"status"`             // "active" | "revoked" | "expired"
    IssuedAt        time.Time `json:"issued_at"`
    ExpiresAt       time.Time `json:"expires_at"`         // TTL（默认 30 天，StrongBox 90 天）
    RevokedAt       *time.Time `json:"revoked_at,omitempty"`
}

type RegisterDeviceInput struct {
    TenantID           string `json:"tenant_id"`
    UserID             string `json:"user_id"`
    PublicKeyPEM       string `json:"public_key_pem"`
    Platform           string `json:"platform"`
    DeviceModel        string `json:"device_model"`
    AttestationCertChain []string `json:"attestation_cert_chain"`
}

type BLEAuthChallenge struct {
    Nonce     []byte `json:"nonce"`      // 32 bytes random
    ReaderID  string `json:"reader_id"`
    Timestamp int64  `json:"timestamp"`
    ExpiresIn int    `json:"expires_in"` // seconds (default 30)
}
```

**数据库迁移**：

```sql
-- 新增表 mistypass_mobile_credentials
CREATE TABLE IF NOT EXISTS mistypass_mobile_credentials (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL,
    user_id        TEXT NOT NULL,
    public_key_pem TEXT NOT NULL,
    device_id      TEXT NOT NULL,
    platform       TEXT NOT NULL DEFAULT 'android',
    keystore_level TEXT NOT NULL DEFAULT 'tee',
    status         TEXT NOT NULL DEFAULT 'active',
    issued_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ NOT NULL,
    revoked_at     TIMESTAMPTZ,
    UNIQUE(tenant_id, user_id, device_id)
);

CREATE INDEX idx_mobile_credentials_tenant_status
    ON mistypass_mobile_credentials(tenant_id, status);
```

**新增 API 端点**：

```
POST   /api/v1/app/credentials/register    — 注册手机密钥对（公钥 + Attestation）
GET    /api/v1/app/credentials              — 获取当前凭据状态
DELETE /api/v1/app/credentials/{id}         — 自助吊销（手机丢失）
POST   /api/v1/app/credentials/refresh      — 刷新即将过期的凭据

POST   /api/v1/credentials/{id}/revoke      — 管理员强制吊销
GET    /api/v1/credentials/active           — Gateway 同步用（返回公钥列表）
```

**文件清单**：

| 新建文件 | 用途 |
|---------|------|
| `api/internal/modules/credential/service.go` | PKI 核心逻辑 |
| `api/internal/modules/credential/model.go` | 数据模型 |
| `api/internal/modules/credential/store.go` | PostgreSQL 持久化 |
| `api/internal/modules/credential/attestation.go` | Android Key Attestation 验证 |
| `api/internal/modules/credential/service_test.go` | 单元测试 |
| `api/internal/http/routes_app_credential.go` | App 端 HTTP handler |
| `api/internal/http/routes_credential_admin.go` | 管理端 HTTP handler |

---

#### Phase 1B — Gateway Agent BLE 子系统（3-4 周）

**新建文件**：`api/cmd/gateway-agent/ble_reader.go`

```go
// ble_reader.go — BLE GATT Server for mobile credential authentication.
// Implements the challenge-response protocol:
//   1. Advertise BLE beacon with Reader_ID
//   2. Phone connects, reads CHALLENGE characteristic (Nonce)
//   3. Phone writes AUTH_RESPONSE with signed(Nonce + UserID)
//   4. Gateway verifies signature against cached public key
//   5. Notify AUTH_RESULT (allow/deny)

package main

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/sha256"
    "crypto/x509"
    "encoding/pem"
    "log/slog"
    "math/big"
    "sync"
    "time"

    "tinygo.org/x/bluetooth"
)

// BLE Service UUID (128-bit, custom)
var (
    mistypassServiceUUID    = bluetooth.NewUUID([16]byte{/* custom UUID */})
    challengeCharUUID       = bluetooth.NewUUID([16]byte{/* ... */})
    authResponseCharUUID    = bluetooth.NewUUID([16]byte{/* ... */})
    readerIdentityCharUUID  = bluetooth.NewUUID([16]byte{/* ... */})
    authResultCharUUID      = bluetooth.NewUUID([16]byte{/* ... */})
)

type BLEReader struct {
    logger       *slog.Logger
    lockID       string
    adapter      *bluetooth.Adapter
    onCredential func(credentialType, credentialData, lockID string)

    mu           sync.Mutex
    currentNonce []byte
    nonceExpiry  time.Time

    // Public key cache (synced from cloud via config pull)
    pubKeys      map[string]*ecdsa.PublicKey // userID → public key
}

func NewBLEReader(logger *slog.Logger, lockID string, onCredential func(string, string, string)) *BLEReader

func (b *BLEReader) Start() error
func (b *BLEReader) Stop()
func (b *BLEReader) UpdatePublicKeys(keys map[string]string) // userID → PEM
func (b *BLEReader) handleAuthResponse(userID string, signature []byte) bool
```

**扩展 `agent.go`**：

```go
// AccessRule 扩展 — 新增 PublicKey 字段
type AccessRule struct {
    CredentialType string   `json:"credential_type"`
    CredentialData string   `json:"credential_data"`
    UserID         string   `json:"user_id"`
    UserEmail      string   `json:"user_email"`
    LockIDs        []string `json:"lock_ids"`
    PublicKeyPEM   string   `json:"public_key_pem,omitempty"` // 新增：BLE 用户公钥
}

// VerifyCredential 扩展 — 新增 ble_signature 类型
func (a *Agent) VerifyCredential(credentialType, credentialData, lockID string) (string, string, string) {
    // ... existing logic ...

    case "ble_signature":
        // credentialData format: "userID:nonceHex:signatureHex"
        // Parse and verify ECDSA signature against cached public key
}
```

**扩展 `main.go`**：

```go
// 新增命令行参数
bleEnabled := flag.Bool("ble", false, "Enable BLE GATT server for mobile credentials")
bleLockID  := flag.String("ble-lock-id", "", "Lock ID for BLE reader (e.g. door_factory_001)")

// 启动 BLE reader（与 NFC reader 并行）
if *bleEnabled && *bleLockID != "" {
    bleReader := NewBLEReader(logger, *bleLockID, agent.HandleCredentialPresented)
    if err := bleReader.Start(); err != nil {
        logger.Warn("BLE reader failed to start", "error", err)
    } else {
        fmt.Printf("BLE:      GATT Server → %s\n", *bleLockID)
        agent.bleReader = bleReader
    }
}
```

**Config Pull 扩展**（`agent.go` `pullConfig`）：

```go
// 云端返回的 AuthzCache 扩展
type AuthzCache struct {
    AccessRules      []AccessRule      `json:"access_rules"`
    MobileCredentials []MobileCredKey  `json:"mobile_credentials"` // 新增
}

type MobileCredKey struct {
    UserID       string `json:"user_id"`
    PublicKeyPEM string `json:"public_key_pem"`
    LockIDs      []string `json:"lock_ids"`
}

// pullConfig 后同步公钥到 BLE reader
if a.bleReader != nil {
    keys := make(map[string]string)
    for _, mc := range result.AuthzCache.MobileCredentials {
        keys[mc.UserID] = mc.PublicKeyPEM
    }
    a.bleReader.UpdatePublicKeys(keys)
}
```

**文件清单**：

| 文件 | 操作 | 用途 |
|------|------|------|
| `api/cmd/gateway-agent/ble_reader.go` | 新建 | BLE GATT Server + 认证协议 |
| `api/cmd/gateway-agent/ble_protocol.go` | 新建 | GATT 特征定义 + 消息编解码 |
| `api/cmd/gateway-agent/agent.go` | 修改 | 扩展 AccessRule + VerifyCredential |
| `api/cmd/gateway-agent/main.go` | 修改 | 新增 `-ble` / `-ble-lock-id` 参数 |
| `api/internal/http/routes_gateway_config.go` | 修改 | Config pull 返回 mobile_credentials |

---

#### Phase 1C — Android App MVP（4-6 周，可与 1A/1B 后期并行）

**技术选型**：Kotlin + Jetpack Compose（原生 BLE 性能最优）

**项目结构**：`mobile/android/` （新建子目录）

```
mobile/android/
├── app/src/main/java/com/mistypass/app/
│   ├── MainActivity.kt
│   ├── ui/
│   │   ├── LoginScreen.kt
│   │   ├── DoorsListScreen.kt
│   │   ├── DoorDetailScreen.kt
│   │   └── SettingsScreen.kt
│   ├── auth/
│   │   ├── AuthRepository.kt          — JWT login/refresh
│   │   └── TokenManager.kt            — Secure token storage
│   ├── credential/
│   │   ├── KeystoreManager.kt         — Android Keystore EC P-256 操作
│   │   ├── AttestationHelper.kt       — Device Attestation 证书链生成
│   │   └── CredentialRepository.kt    — 凭据注册/刷新/吊销
│   ├── ble/
│   │   ├── BLEScanner.kt              — 扫描 MistyPass BLE Beacon
│   │   ├── BLEAuthClient.kt           — GATT Client 认证握手
│   │   └── BackgroundBLEService.kt    — 前台服务保持 BLE 扫描
│   ├── api/
│   │   └── MistyPassApi.kt            — Retrofit HTTP client
│   └── di/
│       └── AppModule.kt               — Hilt DI
├── app/src/main/AndroidManifest.xml
└── build.gradle.kts
```

**核心类设计**：

```kotlin
// credential/KeystoreManager.kt
class KeystoreManager {
    // 生成 EC P-256 密钥对，存入 Android Keystore
    fun generateKeyPair(alias: String): KeyPair {
        val spec = KeyGenParameterSpec.Builder(alias, PURPOSE_SIGN)
            .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
            .setDigests(KeyProperties.DIGEST_SHA256)
            .setUserAuthenticationRequired(false) // 工厂场景不强制生物识别
            .setIsStrongBoxBacked(isStrongBoxAvailable()) // 优先 StrongBox
            .setKeyValidityEnd(Date(System.currentTimeMillis() + 90 * DAY_MS))
            .build()
        val kpg = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, "AndroidKeyStore")
        kpg.initialize(spec)
        return kpg.generateKeyPair()
    }

    // 用私钥签名 Nonce（BLE 认证时调用）
    fun signChallenge(alias: String, nonce: ByteArray): ByteArray {
        val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        val entry = ks.getEntry(alias, null) as KeyStore.PrivateKeyEntry
        return Signature.getInstance("SHA256withECDSA").run {
            initSign(entry.privateKey)
            update(nonce)
            sign()
        }
    }

    // 获取 Key Attestation 证书链
    fun getAttestationChain(alias: String): List<X509Certificate>
}
```

```kotlin
// ble/BLEAuthClient.kt
class BLEAuthClient(private val keystoreManager: KeystoreManager) {
    // 发现 MistyPass GATT Server → 连接 → 读 Challenge → 签名 → 写 Response → 读 Result
    suspend fun authenticate(device: BluetoothDevice): AuthResult {
        val gatt = device.connectGatt(context, false, gattCallback)
        // 1. 读 CHALLENGE characteristic → 获取 Nonce
        val nonce = readCharacteristic(gatt, challengeCharUUID)
        // 2. 用 Keystore 私钥签名
        val signature = keystoreManager.signChallenge("mistypass_key", nonce)
        // 3. 写 AUTH_RESPONSE: userID + signature
        val payload = encodeAuthResponse(userId, signature)
        writeCharacteristic(gatt, authResponseCharUUID, payload)
        // 4. 等待 AUTH_RESULT notification
        return waitForResult(gatt, authResultCharUUID, timeout = 5.seconds)
    }
}
```

**依赖**（`build.gradle.kts`）：

```kotlin
dependencies {
    // UI
    implementation("androidx.compose.material3:material3:1.3.x")
    implementation("androidx.navigation:navigation-compose:2.8.x")
    // Network
    implementation("com.squareup.retrofit2:retrofit:2.11.x")
    implementation("com.squareup.moshi:moshi-kotlin:1.15.x")
    // BLE
    implementation("no.nordicsemi.android:ble:2.8.x") // Nordic BLE library（比原生 API 更可靠）
    // DI
    implementation("com.google.dagger:hilt-android:2.52")
    // Security
    implementation("androidx.security:security-crypto:1.1.0-alpha06")
}
```

---

#### Phase 1D — 海康/ZKTeco 门禁适配（2 周，与 Phase 1A 并行）

> 现有 `api/internal/http/routes_cameras.go` 已有海康/ZKTeco 的**摄像头**集成
> 需要扩展为**门禁控制**集成（开门、权限同步）

**新建文件**：`api/internal/modules/gateway/hikvision_door.go`

```go
// hikvision_door.go — 海康 ISAPI 门禁控制（非摄像头）
// 协议：HTTP DIGEST AUTH + XML/JSON payload
// 参考：ISAPI 文档 "Access Control" 章节

type HikvisionDoorClient struct {
    BaseURL   string // e.g. http://192.168.1.64
    Username  string
    Password  string
    client    *http.Client
}

// RemoteOpenDoor sends ISAPI command to unlock a specific door.
// PUT /ISAPI/AccessControl/RemoteControl/door/{doorNo}
func (h *HikvisionDoorClient) RemoteOpenDoor(doorNo int) error

// SyncUsers pushes user list to Hikvision device for offline card verification.
// POST /ISAPI/AccessControl/UserInfo/Record?format=json
func (h *HikvisionDoorClient) SyncUsers(users []HikUser) error

// SyncCards assigns card numbers to users on device.
// POST /ISAPI/AccessControl/CardInfo/Record?format=json
func (h *HikvisionDoorClient) SyncCards(cards []HikCard) error

// SubscribeEvents opens long-polling for real-time access events.
// GET /ISAPI/Event/notification/alertStream
func (h *HikvisionDoorClient) SubscribeEvents(ctx context.Context, handler func(HikEvent)) error
```

**新建文件**：`api/internal/modules/gateway/zkteco_push.go`

```go
// zkteco_push.go — ZKTeco Push Protocol 门禁集成
// ZKTeco 设备主动推送数据到平台（无需轮询）
// 平台作为 HTTP Server 接收设备推送

type ZKTecoPushHandler struct {
    onEvent func(ZKEvent)
}

// HandlePush is the HTTP handler for ZKTeco device push events.
// POST /api/v1/gateway/zkteco/push
func (z *ZKTecoPushHandler) HandlePush(w http.ResponseWriter, r *http.Request)

// SyncUserToDevice pushes user data to ZKTeco device via its API.
func SyncUserToZKDevice(deviceIP, user ZKUser) error
```

**路由注册**（修改 `router.go`）：

```go
// 南向设备网关端点
r.Route("/api/v1/gateway/southbound", func(r chi.Router) {
    r.Post("/hikvision/{deviceID}/unlock", s.hikvisionUnlock)
    r.Post("/hikvision/{deviceID}/sync-users", s.hikvisionSyncUsers)
    r.Get("/hikvision/{deviceID}/events", s.hikvisionSubscribeEvents)
    r.Post("/zkteco/push", s.zktecoPushReceiver)
    r.Post("/zkteco/{deviceID}/sync-users", s.zktecoSyncUsers)
})
```

---

#### Phase 1E — 集成测试 + 工厂部署（2 周）

| 序号 | 事项 | 说明 |
|---:|------|------|
| B5-1 | 端到端 BLE 认证测试 | App → BLE → Gateway → Relay → 开锁 |
| B5-2 | 延迟测量 | 目标 < 500ms（MVP 可放宽到 800ms） |
| B5-3 | 离线场景测试 | 断网 → 本地公钥验签 → 开门 → 联网后事件同步 |
| B5-4 | 工厂 A 现场部署 | 确认网络、硬件安装位置、电源 |
| B5-5 | 工厂 B 现场部署 | 同上 |
| B5-6 | 用户培训 | App 安装引导、BLE 权限授权、常见问题 |
| B5-7 | 反馈收集 + 修复 | 实际使用 2 周后收集问题 |

---

### 9.4 总体排期

```
Phase 0  [1 周]    硬件选型 + BLE 验证 + 客户需求确认
         │
         ├─── Phase 1A [2-3 周]  云端 PKI 凭据服务
         │         ↓
         ├─── Phase 1B [3-4 周]  Gateway BLE GATT Server（依赖 1A 完成公钥模型）
         │         ↓
         ├─── Phase 1C [4-6 周]  Android App（1A 完成后即可开始）
         │
         ├─── Phase 1D [2 周]    海康/ZKTeco 门禁适配（独立，可与 1A 并行）
         │
         └─── Phase 1E [2 周]    集成测试 + 工厂部署（全部完成后）
```

**关键路径**：Phase 0 → 1A → 1B → 1E（总计约 8-10 周）
**并行路径**：Phase 1C 和 1D 可与关键路径并行

**MVP 交付时间线**：2026-05-02 起算，约 **10 周**（2026 年 7 月中旬）

**2026-05-03 实际进度**：Phase 1A-1D 全部完成（2 天），Phase 1E 待硬件到货。
软件层 MVP 已 100% 就绪，瓶颈转为硬件采购 + 客户现场部署。

---

### 9.5 Phase 2 — 功能扩展（MVP 上线后 2-3 个月）

| 序号 | 事项 | 预估 | 依赖 |
|---:|------|------|------|
| B6-1 | iOS App (BLE + Secure Enclave) | 4-6 周 | Swift + CoreBluetooth |
| B6-2 | NFC HCE 辅助通道 (Android) | 2 周 | Android HCE API |
| B6-3 | TTLock REST 集成 | 2 周 | TTLock OAuth 开发者账号 |
| B6-4 | 访客动态二维码（限时限次） | 1 周 | 现有 QR 基础扩展 |
| B6-5 | 生物识别锁定（高安全场景） | 1 周 | App 端 BiometricPrompt |
| B6-6 | 多门联动/反潜回 | 2 周 | 后端规则引擎 |
| B6-7 | SaaS 计费上线 (Xendit) | 3 天 | 见 7.4 节 |

### 9.6 Phase 3 — 平台化（持续迭代）

| 序号 | 事项 | 预估 | 触发条件 |
|---:|------|------|---------|
| B7-1 | BACnet / BMS 集成 | 3-4 周 | 签约物业大客户 |
| B7-2 | 梯控联动 | 2 周 | 已有电梯模块，需硬件对接 |
| B7-3 | 安全认证申请 | — | 客户量达标后 |
| B7-4 | 多区域部署（新加坡/马来西亚） | 2 周 | 业务扩展需求 |

### 9.7 Phase 4 — 自研硬件（Phase 3 后启动）

| 序号 | 事项 | 说明 |
|---:|------|------|
| B8-1 | ESP32-S3 BLE+NFC 读头量产 | 替代 USB dongle，BOM < $8 |
| B8-2 | 自研 Controller 模组 | ARM Cortex-M + BLE + RS485 + GPIO |
| B8-3 | 工业设计 + 外壳开模 | IP65 防护等级 |
| B8-4 | 供应链建立 | 深圳/东莞工厂 |

---

### 9.8 技术决策记录

| 决策 | 选择 | 原因 |
|------|------|------|
| BLE 库（Go） | `tinygo.org/x/bluetooth` | 跨平台，支持 Linux + macOS，活跃维护 |
| BLE 库（Android） | Nordic `no.nordicsemi.android:ble` | 比原生 API 更可靠，自动重连/重试 |
| 密钥算法 | EC P-256 (secp256r1) | Android Keystore 原生支持，签名 64 bytes |
| 凭据 TTL | StrongBox: 90 天, TEE: 30 天, Software: 7 天 | 安全等级越低 TTL 越短 |
| BLE 方案 | Gateway USB dongle（MVP） | 避免引入第二个固件开发工作流 |
| Android App | Kotlin + Compose | 原生 BLE 性能最优，避免 Flutter BLE 桥接层问题 |
| 离线验证 | Gateway 本地 ECDSA 验签 | 不依赖云端往返，延迟 < 100ms |

---

### 9.9 风险登记

| 风险 | 影响 | 缓解措施 | 状态 |
|------|------|---------|------|
| Android BLE 后台被国产 ROM 杀死 | 用户体验退化为"打开 App 再靠近" | Phase 0 验证 + 前台服务 + 电池优化引导 | Phase 0 验证 |
| `tinygo.org/x/bluetooth` 在 Linux ARM 不稳定 | Gateway BLE 不可用 | 备选：BlueZ D-Bus 直连 | 待验证 |
| 工厂网络条件差（4G 不稳定） | 离线时间超 72h | Controller 已有 72h 缓存，可扩展至 7 天 | 已覆盖 |
| 海康 ISAPI 文档获取需签约 | 集成延迟 | 提前联系海康 TPP 合作伙伴注册 | 待启动 |
| ESP32 量产固件维护成本 | Phase 4 复杂度 | MVP 不涉及，用 USB dongle 绕过 | 后置 |

---

## 11. 2026-05-03 全项目代码审查 + V2 架构对齐

> 基准：`Indonesia_SaaS_Access_Control_Architecture.md` V2（三层凭据体系 + Controller 侧 BLE + 分阶段硬件路线）
> 方法：后端 16 模块逐一审查 + 前端 32 feature 逐一审查 + Gateway Agent 全文件审查
> 结论：**Phase 1 MVP 软件层已 95% 就绪**，剩余 5% 为前端编译错误 + 安全加固 + 少量路由缺失

### 11.1 本次会话完成项

| 事项 | 类别 | 文件 |
|------|------|------|
| V2 架构文档替换 V1 | 文档 | `Indonesia_SaaS_Access_Control_Architecture.md` |
| Tier 3 访客 QR 动态令牌 | 功能 | `access/service.go` (Guest 模型 + token 生成) |
| QR 访客凭据验证 | 功能 | `routes_gateway_verify.go` (qr_code → guest token 解析 + 门级权限) |
| Lark 事件自动化（入职/离职/更新） | 功能 | `routes_integration_lark.go` (3 handlers 完整实现) |
| Lark 联动：创建用户 + 吊销凭据 + 停用账号 | 功能 | `access/service.go` (FindUserByEmail) |
| ZKTeco Push 事件入库审计日志 | 功能 | `routes_southbound.go` (serial→tenant 映射 + IngestAccessEvent) |
| OSDP v2 继电器驱动 | 硬件 | `gateway-agent/osdp_relay.go` (SOM/CRC-16/OUT/LED/BUZ) |
| OSDP v2 单元测试 | 测试 | `gateway-agent/osdp_relay_test.go` (4 tests) |
| DESFire 规格 AES-128→AES-256 | 文档 | 架构文档 4 处修改 |
| OpenAPI 文档同步 | 文档 | `routes_openapi.go` (4 端点 summary 更新 + GuestCreate schema) |
| 全项目 TODO/FIXME 清零确认 | 质量 | 全项目 0 个 TODO |

### 11.2 代码审查发现 — V2 Phase 1 MVP 对齐矩阵

| V2 Phase 1 目标 | 代码状态 | 关键文件 | 缺口 |
|--------|--------|------|------|
| 用户管理 | ✅ | `modules/access/` | — |
| 权限管理（门×用户×时间段） | ✅ | `modules/access/` | — |
| BLE 凭据签发与吊销 | ✅ | `modules/credential/service.go` | — |
| Attestation 验证 | ⚠️ | `modules/credential/attestation.go` | CN 字符串匹配，非 ASN.1 解析 |
| 审计日志 | ✅ | `modules/audit/` + Webhook | — |
| KMS 集成 | ⚠️ | HRIS 有 AES-256-GCM vault | 无独立 Root CA PKI 签发链 |
| Admin Web 控制台 | ⚠️ | `web-admin/` 32 feature | **21 个 TS 编译错误** |
| BLE 双向认证协议 | ✅ | `gateway-agent/ble_protocol.go` | — |
| BLE 读头硬件 | ❌ | `gateway-agent/ble_reader.go` | **仅 TCP 模拟器** |
| 本地白名单缓存 | ✅ | `gateway-agent/agent.go` | — |
| Wiegand 输出 | ✅ | `gateway-agent/relay.go` (GPIO) | — |
| OSDP v2 输出 | ✅ | `gateway-agent/osdp_relay.go` | 待真机 RS-485 测试 |
| 72h 离线能力 | ✅ | `gateway-agent/agent.go` (TTL 可配) | — |
| WebSocket/NATS 同步 | ✅ | Config + NATS + MQTT | — |
| Tier 1: BLE 移动凭据 | ✅ | `modules/credential/` | — |
| Tier 2: DESFire 实体卡 | ✅ | `modules/wallet/physical_card.go` | — |
| Tier 3: 动态 QR 访客 | ✅ | `access/service.go` Guest.AccessToken | — |
| 海康 ISAPI | ✅ | `southbound/hikvision.go` | — |
| 海康 ISUP 5.0 | ❌ | — | **未实现（NAT 穿透场景）** |
| ZKTeco Push SDK | ✅ | `southbound/zkteco.go` | — |
| Fingerspot Cloud API | ✅ | `modules/fingerspot/` (client + service + webhook) | — |
| Webhook 事件推送 | ✅ | `modules/audit/webhook.go` | — |
| REST API | ✅ | 469 端点 | — |

### 11.3 代码审查发现 — 安全问题

| 问题 | 严重级别 | 位置 | 说明 |
|------|---------|------|------|
| OAuth2 "dev-secret" 回退 | **Critical** | `routes_oauth2.go:599` | JWTSecret 未配置时用字面量 "dev-secret" 签名 token，生产环境必须拒绝启动 |
| Attestation 启发式检测 | Medium | `credential/attestation.go:95-144` | `DetectKeystoreLevel()` 用 Issuer CN 字符串匹配判断 strongbox/tee，应改用 ASN.1 扩展解析 |
| MFA 可全局关闭 | Low | `auth/service.go:198` | `adminMFARequired` 为配置开关，生产环境应强制开启 |

### 11.4 代码审查发现 — 前端问题

| 问题 | 影响 | 位置 |
|------|------|------|
| **21 个 TypeScript 编译错误** | 前端无法构建 | analytics (8), visitors (2), routes (2), elevators (1) 等 |
| Analytics 页面 API 类型不匹配 | 图表功能不可用 | `features/analytics/pages/analytics-page.tsx` |
| 访客 QR 码不显示 | Tier 3 凭据无前端入口 | `features/visitors/pages/visitors-page.tsx` |
| Google Workspace 同步无路由 | 客户端代码存在但不可调用 | `integration/google_workspace.go` |
| Integration 模块零测试 | Lark/GWS 客户端未测试 | `modules/integration/` |

### 11.5 代码审查发现 — 基础设施

| 问题 | 影响 | 建议 |
|------|------|------|
| 无版本化 DB 迁移 | 升级部署风险 | 引入 golang-migrate 或 Atlas |
| StateStore 双模式 | 无 DB 时内存存储重启丢失 | 生产环境强制要求 DATABASE_URL |
| 全 in-memory 服务层 | 大数据量时性能降级 | 中期迁移关键模块到 SQL 直查 |

---

## 12. 下一步推进计划（按优先级排序）

### 🔴 P0 — 立即修复（阻塞项） ✅ 已全部完成

| 序号 | 事项 | 预估 | 说明 | 状态 |
|---:|------|------|------|------|
| R1 | **修复前端 TypeScript 编译错误** | 1 天 | Analytics API 类型对齐、Visitors 页面类型修复、Routes props 修复 | ✅ done（验证 0 错误） |
| R2 | **OAuth2 secret 生产加固** | 0.5h | `routes_oauth2.go` — 移除 "dev-secret" 回退，生成 ephemeral key（与 auth 模块一致） | ✅ done（ephemeral secret） |
| R3 | **访客 QR 码前端显示** | 0.5 天 | Visitors 页面：创建访客后显示 access_token QR 码、复制链接、有效期倒计时 | ✅ done（创建后弹窗 + GuestRow QR 按钮） |

### 🟡 P1 — 本周推进（Phase 1 收尾） ✅ 已全部完成

| 序号 | 事项 | 预估 | 说明 | 状态 |
|---:|------|------|------|------|
| R4 | **Google Workspace 同步路由** | 1 天 | 新建 `routes_integration_google.go`：POST /integrations/google-workspace/sync | ✅ done |
| R5 | **Integration 模块测试** | 1 天 | Lark client/contact/bot + Google Workspace client 单元测试 | ✅ done（16 tests） |
| R6 | **Attestation ASN.1 解析升级** | 1 天 | `DetectKeystoreLevel()` 改用 OID 1.3.6.1.4.1.11129.2.1.17 解析 | ✅ done（+ 7 tests） |
| R7 | **生产 HA 部署文档** | 1 天 | PostgreSQL 主从、Redis Sentinel、NATS 集群、网关多实例 | ✅ done（`production-ha-deployment.md` 1961 行） |
| R8 | **DB 版本化迁移基础** | 1 天 | 纯 Go 迁移器，将 EnsureSchema 拆为版本化迁移文件 | ✅ done（`migrator.go` + `001_initial_schema` + 5 tests） |
| R9 | **SCIM 2.0 身份同步** | 5 天 | 8 SCIM Server 端点 + 5 管理 API + 前端 Enterprise SCIM tab + Okta OIN 集成 | ✅ done（Okta 端到端验证通过） |
| F1 | **Lark 前端绑定** | 0.5 天 | 3 API 函数 + Enterprise Sync tab Lark 目录同步 + Bot 测试卡片 | ✅ done |
| F2 | **Google Workspace 前端绑定** | 0.5 天 | 1 API 函数 + Enterprise Sync tab GWS 同步卡片 | ✅ done |
| F3 | **Enterprise 域名映射前端** | 0.5 天 | 3 API 函数 + IdP tab 域名管理面板（CRUD + active/disabled 开关） | ✅ done |
| F4 | **访客详情 + 凭据详情绑定** | 0.5 天 | getGuest API 函数 + 凭据行点击详情弹窗 + 访客 QR 按钮 | ✅ done |
| D2 | **生产 WAF 指南** | 0.5 天 | Cloudflare WAF + 白名单 + DDoS + Terraform | ✅ done（`production-waf-guide.md` 695 行） |

---

## 13. 剩余待做项（全部依赖硬件或外部账号）

> **2026-05-19 v9 状态：Fingerspot Cloud 集成完成，iOS App 已开发。以下所有待做项均卡在外部依赖上。**

### 🔴 等硬件到货（已采购，到货后立即推进）

| 序号 | 事项 | 预估 | 硬件依赖 | 采购状态 | V2 Phase |
|---:|------|------|---------|---------|---------|
| H1 | **BLE 真机硬件集成** | 3 天 | Orange Pi Zero3 + USB BLE 5.0 dongle | ✅ 全部到货 | Phase 1E |
| H2 | **RS485 协议适配层** | 2 天 | USB-RS485 转换器 + Modbus 继电器 + 电磁锁 | USB-RS485 + EM Lock 已有 | Phase 1E |
| H3 | **OTA Gateway 固件侧** | 1 天 | 同 H2 硬件 + Ed25519 签名密钥 | — | Phase 1E |
| H4 | **Camera 真实集成** | 3 天 | Hikvision DS-2CD1023G2-LIU PoE 摄像头 | 已采购 | Phase 2 |
| H5 | **Wireless Locks API** | 2 天 | BLE 智能锁（Tuya/TTLock）~$65 | 待采购 | Phase 2 |
| H6 | **Controller I/O 管理** | 3 天 | ZKTeco C3-100 控制器 + Wiegand 读头 ~$110 | 待采购 | Phase 2 |

**关键路径**：H1 BLE 真机 → Phase 1E 工厂集成测试 → 客户部署

### 🟡 等外部账号/签约

| 序号 | 事项 | 预估 | 外部依赖 | 获取方式 | V2 Phase |
|---:|------|------|---------|---------|---------|
| E1 | **海康 ISUP 5.0 云端透传** | 3 天 | 海康 TPP 合作伙伴签约 | 海康官网申请 TPP 资质 | Phase 1 |
| E2 | **TTLock REST API** | 2 天 | TTLock OAuth 开发者账号 | TTLock 开放平台注册 | Phase 2 |

### 🟢 需要开发周期（按需启动）

| 序号 | 事项 | 预估 | 触发条件 | V2 Phase |
|---:|------|------|---------|---------|
| D1 | ~~iOS App 开发~~ | ~~4-6 周~~ | ~~开发资源就绪~~ | ✅ done（`/ios-MistyisletPass/`） |
| D2 | **Android App 真机 BLE** | 3-5 天 | BLE 硬件到货 + Nordic BLE 库替换 TCP 模拟器 | Phase 1E |
| D3 | **NFC HCE 辅助通道** | 2 周 | Phase 2 BLE 稳定后 | Phase 2 |
| D4 | **多因子认证（BLE + 指纹/PIN）** | 1 周 | 高安全客户需求 | Phase 2 |

### 🔵 客户需求驱动（Phase 3）

| 序号 | 事项 | 预估 | 触发条件 | V2 Phase |
|---:|------|------|---------|---------|
| C1 | BACnet / BMS 楼控联动 | 3-4 周 | 签约物业大客户 | Phase 3 |
| C2 | 大华 DSS API | 2 周 | 客户使用大华设备 | Phase 3 |
| C3 | Suprema BioStar 2 | 2 周 | 高安全生物识别需求 | Phase 3 |
| C4 | 多租户 SaaS 计费 (Xendit) | 3 天 | 商业化启动 | Phase 3 |
| C5 | SI 合作伙伴后台（白标/返佣） | 5 天 | 渠道拓展启动 | Phase 3 |

### ⚪ 长期（V2 Phase 4 自研硬件）

| 序号 | 事项 | 触发条件 |
|---:|------|---------|
| L1 | ESP32-S3 BLE+NFC 自研读头量产 | Phase 3 稳定 + 现金流 |
| L2 | 自研 Controller 模组 (ARM Cortex-M) | 同上 |
| L3 | 工业设计 + IP65 外壳开模 | 同上 |
| L4 | 深圳/东莞供应链建立 | 同上 |

### ⏸️ 暂停

| 序号 | 事项 | 原因 |
|---:|------|------|
| S1 | Apple Pass 真实签名 | Apple Pay 在印尼不可用 |
| S2 | Google Wallet 真实 API | Google Pay 在印尼受限 |

---

## 14. V2 架构文档 Phase 覆盖率总结（2026-05-19 v9 更新）

| V2 Phase | 总项 | 已完成 | 覆盖率 | 瓶颈 |
|----------|---:|---:|---:|------|
| Phase 1 MVP | 23 | 21 | **91%** | ISUP 5.0（需海康签约）、BLE 真机（需硬件） |
| Phase 2 扩展 | 14 | 9 | **64%** | TTLock、NFC HCE、Camera 真实集成 |
| Phase 3 平台化 | 10 | 2 | **20%** | 客户需求驱动 |
| Phase 4 自研硬件 | 4 | 0 | **0%** | Phase 3 稳定后启动 |

> **Phase 1 关键路径**：H1 BLE 硬件到货 → 3 天集成 → Phase 1E 工厂部署
> **Phase 2 并行路径**：E2 TTLock + H4 Camera（不阻塞主线）
> **已验证**：Okta SCIM 集成端到端通过，Fingerspot Cloud API 11 端点全覆盖，iOS App 已开发

---

## 15. 文档索引

> 更新日期：2026-05-19

| 文档 | 路径 | 说明 |
|---|---|---|
| **代码审查报告** | `docs/CODE-REVIEW-2026-05-01.md` | 前端/后端/安全/测试/Kisi 对齐全面审查 |
| MVP 路线图 | `docs/MVP-ROADMAP.md` | M1-M4 已完成，M5 部分完成 |
| 后续路线图 | `docs/NEXT-ROADMAP.md` | 本文件 |
| Kisi 差距分析 | `docs/kisi-gap-analysis.md` | 基于 Bundled References 227 operations 逐项对比 |
| Kisi 架构对照（已归档） | `docs/archive/kisi-comparison.md` | 历史系统/硬件/协议/API 架构对照 |
| API 汇总 | `MISTYISLET-KISI-API-SUMMARY.md` | 资源 API 总索引 + 对齐进度 |
| Gateway 软件安全状态 | `docs/architecture/gateway-security-software-status.md` | 当前 mTLS、WS、nonce、离线补传状态 |
| 硬件/BSP 安全后续项 | `docs/architecture/hardware-bsp-followups.md` | Secure Boot、签名 OTA、物理拓扑等非纯软件事项 |
| 凭证安全架构 | `docs/credential-security-architecture.md` | 全链路安全规范 |
| 凭证操作流程 | `docs/CREDENTIAL-FLOWS.md` | Apple/Google/物理/数字凭证流程 |
| 硬件集成指南 | `docs/hardware-integration-guide.md` | 当前 + 下一代硬件链路 |
| UU PDP 合规指南 | `docs/compliance-uu-pdp-indonesia.md` | 印尼个人数据保护法合规声明 |
| 印尼企业域名设计 | `docs/enterprise/indonesia-enterprise-domain-idp-design.md` | .co.id 域名 + OIDC/SAML 集成 |
| 印尼 HRIS 集成架构 | `docs/enterprise/indonesia-hris-integration-architecture.md` | Talenta + 5 家供应商连接器 |
| **印尼 SaaS 门禁架构方案** | `Indonesia_SaaS_Access_Control_Architecture.md` | BLE + Keystore 完整技术方案 |
| **Lark 集成方案** | `docs/integrations/lark-integration.md` | Bot + 通讯录 + 事件订阅 |
| **Google Workspace 集成方案** | `docs/integrations/google-workspace-integration.md` | Directory API 员工同步 |
| **生产 HA 部署指南** | `docs/production-ha-deployment.md` | PG/Redis/NATS/EMQX HA + K8s + 印尼区域部署 |
| **生产 WAF 指南** | `docs/production-waf-guide.md` | Cloudflare WAF + 白名单 + DDoS + Terraform |
| **iOS App 开发计划** | `ios-MistyisletPass/APP-DEVELOPMENT-PLAN.md` | HIG 合规 + BLE + Secure Enclave + 3 Phase |
| **Android App 开发计划** | `android-MistyisletPass/APP-DEVELOPMENT-PLAN.md` | M3 合规 + Keystore + Nordic BLE + 印尼适配 |
| **Fingerspot 集成设计** | `docs/superpowers/specs/2026-05-19-fingerspot-integration-design.md` | Cloud REST API 11 端点 + Webhook |
| **Fingerspot 集成实施计划** | `docs/superpowers/plans/2026-05-19-fingerspot-integration.md` | 7 任务 TDD 实施计划 |
| OpenAPI Spec | `GET /api/v1/openapi.json` | 自动生成，版本 2026-05-03 |
