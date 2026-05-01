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
| 13 | **Cameras / Video Surveillance** | 视频监控集成 | 5+ 天 | 待做 |
| 14 | **Organization Transfers / Certificate Rotation** | 多租户运营 | 3 天 | 待做 |
| 15 | ~~Signed Upload URLs~~ | ~~文件上传~~ | ~~1 天~~ | done |
| 16 | ~~CSV Card Imports 独立资源~~ | ~~对齐 Kisi API~~ | ~~1 天~~ | done |
| 17 | ~~Login Session 管理~~ | ~~活跃会话~~ | ~~2 天~~ | done |
| 18 | ~~Password Reset~~ | ~~自助密码重置~~ | ~~2 天~~ | done |
| 19 | **Apple Pass 真实签名** | 替换 mock PKCS#7 | 3 天 | 待做 |
| 20 | **Google Wallet 真实 API** | 替换 mock JWT | 2 天 | 待做 |
| 21 | **Alert Policy 渠道升级 + DB 持久化** | 通知链路完善 | 3-4 天 | 待做 |
| 22 | **Company / Place Analytics 报告** | 图表分析报告 | 3 天 | 待做 |
| 23 | **Alarm Schedule 周历视图** | 告警排程周历 | 2 天 | 待做 |

### MVP M5 — 硬件对接准备（剩余 2 项）

| 序号 | 事项 | 理由 | 预估 | 状态 |
|---:|---|---|---|---|
| 19 | ~~Gateway 通信协议文档~~ | ~~协议形式化~~ | ~~1 天~~ | done |
| 20 | ~~Bootstrap API 增强（OTA 发现）~~ | ~~Config pull 集成~~ | ~~1 天~~ | done |
| 21 | ~~OTA 状态回报端点~~ | ~~Gateway device token~~ | ~~0.5 天~~ | done |
| 22 | **RS485 协议适配层** | 网关端串口通信 | 2 天 | 待做（依赖硬件） |
| 23 | **OTA Gateway 固件侧** | 下载/验证/安装 | 1 天 | 待做（依赖硬件） |

---

## 4. 文档索引

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
