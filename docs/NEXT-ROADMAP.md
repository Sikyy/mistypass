# Mistyislet 后续推进路线图

> 更新日期：2026-05-01 (v2)
> 第一优先级：完成 MVP 项目跑通，包括硬件
> 第一理念：让用户更方便、更安全、更高效地开门
> MVP 聚焦路线图见 `docs/MVP-ROADMAP.md`
> Gateway 通信协议见 `docs/architecture/gateway-cloud-protocol.md`

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

---

## 2. 差距总览（vs Kisi API spec，已更新）

| 分类 | Kisi 资源数 | 已覆盖 | 覆盖率 |
|---|---:|---:|---:|
| 核心 CRUD（Places/Locks/Users/Groups/Teams/Roles/Shares + favorite） | 22 | 22 | 100% |
| 凭证（Cards/CardAssignments/Invites） | 6 | 5 | 83% |
| 硬件（Controllers/Readers/Terminals） | 6 | 6 | 100% |
| 事件与报表（Events/Reports） | 6 | 6 | 100% |
| 集成与策略（Integrations/AlertPolicies） | 4 | 4 | 100% |
| 高级硬件（WirelessLocks/Elevators/ControllerI-O） | 10 | 3 | 30% |
| 组织管理（Organization settings/dashboard/transfer/cert） | 8 | 4 | 50% |
| 用户安全（2FA/Password/Signup/Logins/WebAuthn） | 7 | 7 | 100% |
| 日历与排程（Schedules/Calendar） | 3 | 3 | 100% |
| 访客与在场（Guests/Presences） | 2 | 2 | 100% |
| 摄像头（Cameras） | 2 | 0 | 0% |
| 文件上传（SignedUploadURLs） | 1 | 1 | 100% |

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
| 19 | **Apple Pass 真实签名** | 替换 mock PKCS#7 | 3 天 | 待做（需 Apple Developer 账号） |
| 20 | **Google Wallet 真实 API** | 替换 mock JWT | 2 天 | 待做（需 GCP Service Account） |
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

### 优先级建议

```
1. Apple + Google Wallet（仅需账号，无硬件） → 申请账号后立即可做
2. RS485 + OTA（最小硬件预算 ~$55）       → 买到硬件后 3 天完成
3. Wireless Locks（需特定锁）              → 视产品规划
4. Controller I/O（需控制器板）             → 视产品规划
5. Camera 集成（需摄像头）                  → 视产品规划
```

## 5. 文档索引

| 文档 | 路径 | 说明 |
|---|---|---|
| MVP 路线图 | `docs/MVP-ROADMAP.md` | M1-M4 已完成，M5 部分完成 |
| 后续路线图 | `docs/NEXT-ROADMAP.md` | 本文件 |
| Gateway 通信协议 | `docs/architecture/gateway-cloud-protocol.md` | HTTPS + NATS 协议参考 |
| 凭证安全架构 | `docs/credential-security-architecture.md` | 全链路安全规范 |
| 凭证操作流程 | `docs/CREDENTIAL-FLOWS.md` | Apple/Google/物理/数字凭证流程 |
| 硬件集成指南 | `docs/hardware-integration-guide.md` | 当前 + 下一代硬件链路 |
| Kisi 差距分析 | `docs/kisi-gap-analysis.md` | Kisi API vs MistyPass 逐项对比 |
| OpenAPI Spec | `GET /api/v1/openapi.json` | 自动生成，版本 2026-05-01 |
