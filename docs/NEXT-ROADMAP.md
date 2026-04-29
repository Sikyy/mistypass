# Mistyislet 后续推进路线图

> 创建日期：2026-04-29
> 基于 Kisi API Bundled References 和 docs.kisi.io 产品文档的差距分析

---

## 1. 差距总览

对照 Kisi API spec（81 个资源路径）和 Kisi 产品文档，Mistyislet 当前的覆盖率：

| 分类 | Kisi 资源数 | Mistyislet 已覆盖 | 覆盖率 |
|---|---:|---:|---:|
| 核心 CRUD（Places/Locks/Users/Groups/Teams/Roles/Shares） | 22 | 22 | 100% |
| 凭证（Cards/CardAssignments/Invites） | 6 | 5 | 83% |
| 硬件（Controllers/Readers/Terminals） | 6 | 6 | 100% |
| 事件与报表（Events/Reports） | 6 | 6 | 100% |
| 集成与策略（Integrations/AlertPolicies） | 4 | 4 | 100% |
| 高级硬件（WirelessLocks/Elevators/ControllerI-O） | 10 | 0 | 0% |
| 组织管理（Organization settings/dashboard/transfer/cert） | 8 | 0 | 0% |
| 用户安全（2FA/Password/Signup/Logins） | 7 | 2 | 29% |
| 日历与排程（Schedules/Calendar） | 3 | 1 | 33% |
| 访客与在场（Guests/Presences） | 2 | 0 | 0% |
| 摄像头（Cameras） | 2 | 0 | 0% |
| 文件上传（SignedUploadURLs） | 1 | 0 | 0% |

---

## 2. 推进优先级

### P1 — 近期推进（影响日常运营）

| 序号 | 事项 | 理由 | 预估 |
|---:|---|---|---|
| 1 | **Schedules 独立资源** | Group 时间策略精细管理，当前只有 TimeWindow 嵌入字段 | 2-3 天 |
| 2 | **Organization Settings / Dashboard** | 组织级配置和概览缺失 | 2-3 天 |
| 3 | **User 2FA self-service** | 安全合规需求，当前只有 Admin MFA | 1-2 天 |
| 4 | **Invites 独立资源** | 运营需全局查看和管理邀请 | 1 天 |
| 5 | **Holiday Calendar Regions** | 按国家预设假日日历 | 1 天 |

### P2 — 中期推进（扩展硬件和场景）

| 序号 | 事项 | 理由 | 预估 |
|---:|---|---|---|
| 6 | **Elevator / Elevator Stops / Group Elevator Stops** | 电梯门禁场景 | 3-4 天 |
| 7 | **Wireless Locks** | 无线锁硬件 | 2 天 |
| 8 | **Controller Inputs / Relays / Wiegands** | 控制器高级 I/O 管理 | 3 天 |
| 9 | **Guests 独立资源** | 统一访客目录 | 2 天 |
| 10 | **Presences / 容量管理** | 在场追踪和容量限制 | 3 天 |
| 11 | **Group Terminals** | Terminal 按组分配 | 1 天 |

### P3 — 长期补齐（完善平台能力）

| 序号 | 事项 | 理由 | 预估 |
|---:|---|---|---|
| 12 | **Cameras / Video Surveillance** | 视频监控集成 | 5+ 天 |
| 13 | **Organization Transfers / Certificate Rotation** | 多租户运营 | 3 天 |
| 14 | **Signed Upload URLs** | 文件上传支持 | 1 天 |
| 15 | **CSV Card Imports 独立资源** | 对齐 Kisi API | 1 天 |
| 16 | **Login Session 管理** | 查看和管理登录会话 | 2 天 |
| 17 | **Password Reset / Self Signup** | 自助密码重置和注册 | 2 天 |
| 18 | **Apple Pass 真实签名** | 替换 mock PKCS#7 | 3 天 |
| 19 | **Google Wallet 真实 API** | 替换 mock JWT | 2 天 |
| 20 | **Alert Policy 渠道升级 + DB 持久化** | 通知链路完善 | 3-4 天 |

---

## 3. 已完成项（本轮会话）

| 事项 | Commit |
|---|---|
| P0 非硬件 legacy 全域审计 | `4f26ea7` |
| P1 Users 批量治理（5 API + CSV） | `bf184fc` |
| P1 Alert Policy 事件驱动调度器 | `c3664a2` |
| P1 Access Rights 复杂 schedule | `98a0c4b` |
| P1 Apple/Google Wallet mock provider | `2e54efe` |
| P2 OpenAPI 20+ 资源 schema | `090cadd` |
| P2 i18n 三语（en/zh/id） | `f073d3f` |
| P2 kisi → mistyislet 命名统一 | `c3a2fca` |
| P2 Bundle 优化 + legacy 整理 | `4b7b38a` |

---

## 4. 当前已知 UI 问题

| 问题 | 状态 |
|---|---|
| Dashboard 个人菜单语言切换不起作用 | 待修复 |
| 登录页语言切换按钮样式需改为 shadcn/ui | 待修复 |
| /dashboard 旧黑色系仪表盘仍可访问 | 待重定向到 /home |
| Organization Setup 子页面功能不完整 | 待补全 |
| Event History 选项切换不响应 | 待修复 |
| Sidebar 底部三链接指向不正确 | 待修复 |
