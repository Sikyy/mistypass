# Mistyislet 后续推进路线图

> 更新日期：2026-04-29
> 基于 Kisi API Bundled References 和 docs.kisi.io 产品文档的差距分析

---

## 1. 已完成项（本轮会话 22 个 commit）

### 后端 API
| 事项 | 状态 |
|---|---|
| P0 非硬件 legacy 全域审计（37 个写操作） | done |
| P1 Users 批量治理（batch-status/delete/invite + CSV 导出导入） | done |
| P1 Alert Policy 事件驱动调度器 + cooldown + 通知查询 | done |
| P1 Access Rights 复杂 schedule（TimeWindow + HolidayCalendar + evaluate） | done |
| P1 Apple/Google Wallet mock provider + 凭证流程文档 | done |
| P2 OpenAPI 20+ 资源字段级 schema | done |

### 前端 UI
| 事项 | 状态 |
|---|---|
| kisi → mistyislet 命名统一（40+ 符号） | done |
| i18n 三语 100% 覆盖（en-US/zh-CN/id-ID，~350 keys） | done |
| Bundle 优化（主入口 -13%，legacy 页面归档） | done |
| 语言切换（Shell + 登录页 shadcn DropdownMenu） | done |
| 根路由 / 重定向到 /home，阻断旧 dashboard | done |
| Event History 过滤器改为 DropdownMenu | done |
| Sidebar 底部链接改为外部 <a> | done |
| Organization Setup Coming Soon 提示 | done |
| Topbar 搜索框改为真正的 input | done |
| Place Dashboard 增强（Daily Usage 表格 + Unlock Heatmap） | done |

---

## 2. 差距总览（vs Kisi API spec）

| 分类 | Kisi 资源数 | 已覆盖 | 覆盖率 |
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

## 3. 待推进事项

### P1 — 近期（影响日常运营）

| 序号 | 事项 | 理由 | 预估 |
|---:|---|---|---|
| 1 | **Schedules 独立资源** | Group 时间策略精细管理，当前只有 TimeWindow 嵌入字段 | 2-3 天 |
| 2 | **Organization Settings / Dashboard** | 组织级配置和概览 endpoint 缺失，UI 仅为预留样板 | 2-3 天 |
| 3 | **User 2FA self-service** | 安全合规需求，当前只有 Admin MFA | 1-2 天 |
| 4 | **Invites 独立资源** | 运营需全局查看和管理邀请，当前嵌入 user workflow | 1 天 |
| 5 | **Holiday Calendar Regions** | 按国家预设假日日历，当前需手动创建 | 1 天 |

### P2 — 中期（扩展硬件和场景）

| 序号 | 事项 | 理由 | 预估 |
|---:|---|---|---|
| 6 | **Elevator / Elevator Stops / Group Elevator Stops** | 电梯门禁场景 | 3-4 天 |
| 7 | **Wireless Locks** | 无线锁硬件 | 2 天 |
| 8 | **Controller Inputs / Relays / Wiegands** | 控制器高级 I/O 管理 | 3 天 |
| 9 | **Guests 独立资源** | 统一访客目录 | 2 天 |
| 10 | **Presences / 容量管理** | 在场追踪和容量限制 | 3 天 |
| 11 | **Group Terminals** | Terminal 按组分配 | 1 天 |
| 12 | **Visitor Management UI** | 参考 Kisi 截图中的 Present/Past Visitors 表格 | 2 天 |

### P3 — 长期（完善平台能力）

| 序号 | 事项 | 理由 | 预估 |
|---:|---|---|---|
| 13 | **Cameras / Video Surveillance** | 视频监控集成 | 5+ 天 |
| 14 | **Organization Transfers / Certificate Rotation** | 多租户运营 | 3 天 |
| 15 | **Signed Upload URLs** | 文件上传（头像/证件） | 1 天 |
| 16 | **CSV Card Imports 独立资源** | 对齐 Kisi API | 1 天 |
| 17 | **Login Session 管理** | 查看和管理活跃登录会话 | 2 天 |
| 18 | **Password Reset / Self Signup** | 自助密码重置和注册 | 2 天 |
| 19 | **Apple Pass 真实签名** | 替换 mock PKCS#7 为真实 Pass Type Certificate | 3 天 |
| 20 | **Google Wallet 真实 API** | 替换 mock JWT 为真实 Service Account 签名 | 2 天 |
| 21 | **Alert Policy 渠道升级 + DB 持久化** | 通知链路完善，当前内存态 | 3-4 天 |
| 22 | **Company / Place Analytics 报告** | 参考 Kisi 截图中的图表分析报告 | 3 天 |
| 23 | **Alarm Schedule 周历视图** | 参考 Kisi 截图中的告警排程周历 | 2 天 |

---

## 4. 已修复的 UI 问题

| 问题 | 状态 |
|---|---|
| Dashboard 个人菜单语言切换不起作用 | 已修复 |
| 登录页语言切换按钮样式 | 已改为 shadcn DropdownMenu |
| / 和 /dashboard 旧黑色系仪表盘可访问 | 已重定向到 /home |
| 侧边栏和子页面语言不同步 | 已全部接入 i18n（100%） |
| Organization Setup 子页面不完整 | 已加 Coming Soon 提示 |
| Event History 过滤器死板 | 已改为 DropdownMenu |
| Sidebar 底部三链接指向错误 | 已改为外部链接 |
| Topbar 搜索框不可点击 | 已改为真正的 input |
| Alert Policies 布局不对齐 | 已调整网格和 overflow |
| Event History 行展开后无法收起 | 已修复 useEffect |
| Place 页面崩溃 | 已修复 useTranslation 放置错误 |
