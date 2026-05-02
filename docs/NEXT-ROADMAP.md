# Mistyislet 后续推进路线图

> 更新日期：2026-05-02 (v4)
> 第一优先级：完成 MVP 项目跑通，包括硬件
> 第一理念：让用户更方便、更安全、更高效地开门
> ⚠️ Apple/Google Wallet 功能因印尼政策原因暂停推进（Apple Pay/Google Pay 在印尼暂不可用）
> MVP 聚焦路线图见 `docs/MVP-ROADMAP.md`
> Gateway 通信协议见 `docs/architecture/gateway-cloud-protocol.md`
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

### 优先级建议（2026-05-02 更新）

```
1. RS485 + 电磁锁集成（EM Lock 已采购）     → 到货后立即推进
2. Camera 真实集成（DS-2CD1023G2-LIU 已采购）→ 到货后立即推进
3. OTA Gateway 固件侧                       → 与 #1 同步
4. API 场景完善 + 印尼企业功能适配            → 持续推进
5. Wireless Locks（需特定锁）                → 视产品规划
6. Controller I/O（需控制器板）               → 视产品规划
---
⏸️ Apple + Google Wallet                     → 暂停（印尼政策限制）
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

---

## 8. 文档索引

| 文档 | 路径 | 说明 |
|---|---|---|
| **代码审查报告** | `docs/CODE-REVIEW-2026-05-01.md` | 前端/后端/安全/测试/Kisi 对齐全面审查 |
| MVP 路线图 | `docs/MVP-ROADMAP.md` | M1-M4 已完成，M5 部分完成 |
| 后续路线图 | `docs/NEXT-ROADMAP.md` | 本文件 |
| Kisi 差距分析 | `docs/kisi-gap-analysis.md` | 基于 Bundled References 227 operations 逐项对比 |
| Kisi 架构对照 | `docs/architecture/kisi-comparison.md` | 系统/硬件/协议/API 架构对照 |
| API 汇总 | `MISTYISLET-KISI-API-SUMMARY.md` | 资源 API 总索引 + 对齐进度 |
| Gateway 通信协议 | `docs/architecture/gateway-cloud-protocol.md` | HTTPS + NATS 协议参考 |
| 凭证安全架构 | `docs/credential-security-architecture.md` | 全链路安全规范 |
| 凭证操作流程 | `docs/CREDENTIAL-FLOWS.md` | Apple/Google/物理/数字凭证流程 |
| 硬件集成指南 | `docs/hardware-integration-guide.md` | 当前 + 下一代硬件链路 |
| UU PDP 合规指南 | `docs/compliance-uu-pdp-indonesia.md` | 印尼个人数据保护法合规声明 |
| 印尼企业域名设计 | `docs/enterprise/indonesia-enterprise-domain-idp-design.md` | .co.id 域名 + OIDC/SAML 集成 |
| 印尼 HRIS 集成架构 | `docs/enterprise/indonesia-hris-integration-architecture.md` | Talenta + 5 家供应商连接器 |
| OpenAPI Spec | `GET /api/v1/openapi.json` | 自动生成，版本 2026-05-01 |
