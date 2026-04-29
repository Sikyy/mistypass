# Mistyislet 管理后台 UI 重构执行计划

> 创建日期：2026-04-25
> 更新日期：2026-04-29
> 品牌名：Mistyislet
> 基准产品：Kisi Dashboard 的信息架构、视觉克制度与资源型 API 风格
> 产品文档参考：`https://docs.kisi.io/`
> API 文档参考：`https://api.getkisi.com/docs#`
> 本地 API 基准：`Kisi-API-Bundled References.yaml`
> API 汇总文档：`MISTYISLET-KISI-API-SUMMARY.md`
> 截图参考：`kisi-image/`
> 前置条件：`MISTYPASS-ROLE-BASED-UI-REFORM-PLAN.md` 阶段 0-6 已完成

本文档是 Mistyislet 管理后台重构的执行手册。Kisi 只作为重构基准，不是品牌名；用户可见文案必须统一使用 Mistyislet。

---

## 1. 执行原则

### 1.1 产品方向

Mistyislet 后台要从“功能堆叠型后台”转为“任务驱动型门禁运营控制台”：

- 用户通过左侧导航和路由理解当前上下文，而不是依赖散落在页面中的租户/楼宇筛选器。
- Organization 工作与 Place 工作使用不同导航。
- 每个页面只服务一个主任务：管理人员、配置访问组、进入 Place、查看事件、处理凭证。
- 长列表和运营页面优先使用表格、设置面板、状态圆点，不使用营销式 hero 或装饰卡片。
- 先完成只读资源模型，再做创建、编辑、删除和确认流。

### 1.2 命名规则

| 场景 | 规则 |
|---|---|
| UI 产品名 | 一律使用 `Mistyislet` |
| 基准产品引用 | 只允许出现在本计划、截图说明、内部 benchmark 文件名或代码命名中 |
| 用户可见禁用项 | 不显示旧品牌名，也不显示基准方资源入口文案 |
| 内部路径 | 现有 `kisi-shell` / `components/kisi` 可暂时保留，后续再做 rename pass |

### 1.3 当前优先级

当前处于 Phase 3：只读资源 API 接入。产品权限模型收敛为两类管理视角：Organization Admin 和 Place Admin；API 权限模型必须按 Bundled Reference 的 `roles` + `role_assignments` + `shares` 设计，后端历史角色只作为迁移期兼容存在。第一批 reference-style endpoint 已落地，后续重点是把页面 adapter 和后端 guard 继续迁到这些 endpoint 上。

1. 保持 preview shell 稳定。
2. 将 Places、Doors、Floors、Hardware、Events、Users 从静态数据切到真实后端数据。
3. 在正式资源 API 补齐前保留 fallback，避免旧接口局部失败导致整页不可用。
4. OpenAPI baseline 已补 `GET /api/v1/openapi.json`，覆盖 reference resources、Mistyislet extension、legacy compatibility、operationId、pagination components 与统一 error schema；实体卡库存 status 后端治理和 Credentials 前端控件 baseline 已接，下一批继续补制卡供应商真实 API 对接、实体卡完整运营视图、Users 批量治理和 Alert Policies 调度器/渠道升级。

---

## 2. 与基准产品的差距

| 领域 | 基准产品做法 | Mistyislet 原问题 | 重构方向 |
|---|---|---|---|
| 导航 | Organization 与 Place 使用不同 Sidebar | 所有模块混在一套 Sidebar 中 | Sidebar A/B + 路由上下文 |
| 资源模型 | People -> Teams/Groups -> Access Rights -> Places/Doors -> Events | UI 跟着后端模块命名走 | UI 跟着用户任务和门禁资源走 |
| 页面密度 | 每页聚焦一个任务 | Enterprise/Wallet/Access 页面过大 | 拆到 feature pages 和 settings panels |
| 操作模式 | 大资源用页面，小编辑用 Sheet | 弹窗、折叠、内联表单混用 | 只读页先稳定，写入流后置 |
| 视觉风格 | 白色画布、弱装饰、强表格可读性 | 深色雾岛与亮色任务卡混杂 | 白色企业控制台，登录页保留品牌氛围 |

---

## 3. 信息架构

### 3.1 Sidebar A：Organization Admin

适用角色：`super_admin`、`tenant_admin`。

| 分区 | 入口 | 路由 | 任务 |
|---|---|---|---|
| Home | Home | `/home` | 组织级运营概览 |
| People & Access | Users | `/users` | 全组织人员目录 |
| People & Access | Teams | `/teams` | 用于批量分配访问权限的用户集合 |
| People & Access | Groups | `/groups` | 门、楼层、限制条件组成的访问组 |
| People & Access | Access Rights | `/access-rights` | 谁在什么时间访问什么资源 |
| People & Access | Credentials | `/credentials` | 移动凭证、Pass、卡等日常操作 |
| Places | Places | `/places` | 进入某个 Place 并切换到 Place Admin 上下文 |
| Events & Reports | Event History | `/event-history` | 组织级事件时间线 |
| Events & Reports | Reports | `/reports` | 告警、审计、统计 |
| Organization Setup | Alert Policies | `/organization/alert-policies` | 告警策略与通知 |
| Organization Setup | Integrations | `/organization/integrations` | SSO、HRIS、SCIM、Webhook、MQTT |
| Organization Setup | Billing | `/organization/billing` | 计费预留 |
| Organization Setup | Create Place | `/organization/create-place` | 创建 Place 入口 |
| Organization Setup | Settings | `/organization/settings` | 组织设置 |
| Organization Setup | SSO & SCIM | `/organization/sso-scim` | 身份集成快捷入口 |
| Resources | Mistyislet Shop | `/organization/create-place` | 硬件/服务入口 |
| Resources | Mistyislet Documentation | `/organization/settings` | 文档入口 |
| Resources | Help & Feedback | `/organization/settings` | 帮助反馈入口 |

### 3.2 Sidebar B：Place Admin

适用产品角色：Place Admin，以及 Organization Admin 进入某个 Place 后。

| 分区 | 入口 | 路由 | 任务 |
|---|---|---|---|
| Home | Home | `/home` | 当前用户的概览入口 |
| Dashboard | Place Dashboard | `/places/:placeId/dashboard` | 单个 Place 的 KPI 和最近活动 |
| People & Access | Place Users | `/places/:placeId/users` | 本 Place 的有效访问人员 |
| People & Access | Place Groups | `/places/:placeId/groups` | 本 Place 的访问组 |
| Site Structure | Doors | `/places/:placeId/doors` | 门点列表和详情设置 |
| Site Structure | Floors | `/places/:placeId/floors` | 楼层/区域拓扑 |
| Site Structure | Elevators | `/places/:placeId/elevators` | 预留 |
| Activity | Unlock History | `/places/:placeId/unlock-history` | 本 Place 开锁事件 |
| Activity | Analytics | `/places/:placeId/analytics` | 本 Place 统计 |
| Operations | Capacity Management | `/places/:placeId/capacity-management` | 预留 |
| Operations | Intrusion Detection | `/places/:placeId/intrusion-detection` | 预留 |
| Operations | Integrations | `/places/:placeId/integrations` | Place 级集成 |
| Operations | Hardware | `/places/:placeId/hardware` | 网关、控制器、读卡器 |
| Operations | Place Settings | `/places/:placeId/settings` | Place 设置 |
| Resources | Mistyislet Shop | `/organization/create-place` | 硬件/服务入口 |
| Resources | Mistyislet Documentation | `/organization/settings` | 文档入口 |
| Resources | Help & Feedback | `/organization/settings` | 帮助反馈入口 |

### 3.3 管理角色映射

新 UI 只暴露两类管理角色：

| 产品角色 | 实现层映射 | 默认视角 | 可返回组织视角 | Place 路由 | 说明 |
|---|---|---|---:|---:|---|
| Organization Admin | `super_admin`、`tenant_admin` | Organization | 是 | 是 | 管理组织级人员、权限、Places、集成与报表 |
| Place Admin | `building_admin`，以及迁移期只读 Place 账号 | Place | 否 | 是 | 管理或查看被分配 Place 的门点、人员、硬件、事件 |

非管理账号不进入本计划；若后端历史数据中仍存在非管理角色，应在登录/入口层阻止进入管理后台，不在新 UI 信息架构中展示。

### 3.4 API 权限层级

API 权限模型的完整定义见 `MISTYISLET-KISI-API-SUMMARY.md` §2。本节只列出 UI 信息架构需要遵守的映射规则：

- Organization Admin 是 Organization scope 的 Role Assignment，不是单独的后端角色枚举。
- Place Admin 是 Place scope 的 Role Assignment，不是单独的后端角色枚举。
- Access Rights 页面应聚合 role assignments、shares、groups、teams 和 group resource bindings。
- 后端历史 `super_admin`、`tenant_admin`、`building_admin`、迁移期只读 Place 账号只作为 bootstrap/login 兼容层，不能成为新 OpenAPI 的产品权限模型。
- Group scope Role Assignment 可带有效期；这类能力用于临时访问和访问链接，不要新增自造 `/access_rights` 资源。

### 3.5 上下文规则

- 上下文由路由和 Sidebar 模式决定。
- `/places/:placeId/*` 一律是 Place 上下文。
- 从 `/places` 点击卡片进入 `/places/:placeId/dashboard`。
- Organization Admin 可以从 Place 上下文返回 `/places`。
- Place Admin 后续要从后端解析默认 assigned Place。
- `/places/assigned/*` 只作为兼容入口，最终必须替换成 canonical Place id。

---

## 4. 视觉系统

### 4.1 颜色和布局

| Token | 值 | 用途 |
|---|---|---|
| Canvas | `#f7f7f8` | App 背景 |
| Surface | `#ffffff` | Sidebar、表格、面板 |
| Surface muted | `#fbfbfc` | 表头、hover、底栏 |
| Border | `#e1e3e8` | 卡片和面板边框 |
| Divider | `#eceef2` | 表格行和分区线 |
| Text strong | `#17171c` | 标题和数字 |
| Text body | `#2f3037` | 正文 |
| Text muted | `#6f717c` | 辅助文字 |
| Primary | `#4f55ff` | 选中导航、链接、主按钮 |
| Success | `#35a853` | 在线/成功 |
| Warning | `#d98b06` | 需关注 |
| Danger | `#d93025` | 失败/高危 |
| Info | `#1863dc` | 信息状态 |

### 4.2 App Shell 规则

- Topbar 高 64px，深蓝 `#202443`，固定在顶部。
- 桌面 Sidebar 宽 248px，固定在 Topbar 下方，填满 `calc(100vh - 64px)`。
- 主内容最大宽度 1180px。
- Sidebar 底部资源入口必须固定在左下角，不随主内容滚动上移。
- Mistyislet Documentation 必须单行显示，空间不足时截断，不换行。
- 退出登录只出现在账号菜单中。

### 4.3 页面规则

- 管理后台内不做 landing page hero。
- 页面顶部统一为面包屑、标题、可选数量、可选操作按钮。
- 表格紧凑、可扫描、行高稳定。
- 状态使用 8px 圆点 + 文案。
- 卡片 6px 圆角、1px 边框、无阴影。
- 创建/编辑后续统一用 Sheet；确认类操作用 Dialog。

### 4.4 共享组件

| 文件 | 职责 |
|---|---|
| `web-admin/src/components/kisi/primitives.tsx` | `PageFrame`、`KpiCard`、`SettingsPanel`、`FormField`、`StatusDot`、Toggle |
| `web-admin/src/components/kisi/data-display.tsx` | Search、Filter、空表格行、分页 |
| `web-admin/src/features/kisi-shell/kisi-admin-shell.tsx` | Topbar、Sidebar、移动端 Topbar、Shell 布局 |
| `web-admin/src/features/kisi-shell/navigation.ts` | 导航配置、角色可见性、路由解析 |
| `web-admin/src/context/navigation-context.tsx` | 当前视角、当前 Place、进入/返回操作 |

---

## 5. 页面地图

### 5.1 Organization 页面

| 页面 | 当前模块 | 状态 |
|---|---|---|
| Home | `features/home/pages/home-page.tsx` | 已读取 gateways/alarms/events |
| My Account | `features/account/pages/my-account-page.tsx` | 已拆 Profile/Logins/Credentials/Security/API；Profile 已接 `/api/v1/user` 保存 name/language |
| Users | `features/users/pages/users-page.tsx` | 已接 `/api/v1/users` adapter、Add User、Invite record creation + invitation queue、批量 Suspend/Enable；后端已补 user detail/update/delete/invite/invitations/receipt 与 client helper |
| User Detail | `features/users/pages/user-detail-page.tsx` | 已接真实 user detail、General 保存、Suspend/Enable、Resend Invite、Invitation Deliveries 历史和 Delete User 确认 |
| Teams | `features/teams/pages/teams-page.tsx` | 已接 `/api/v1/teams` + `/api/v1/team_memberships` adapter，并接 New Team、General Save、Delete Team、Add/Remove Member、Assign Access Right |
| Groups | `features/groups/pages/groups-page.tsx` | 已接 `/api/v1/groups` 创建/保存/删除、`group_locks` 写入、`group_zones` 只读、限制条件编辑、`group_links` Add/Edit/Delete 和 token 验证 wrapper |
| Access Rights | `features/access-rights/pages/access-rights-page.tsx` | 已切 `role_assignments` + `shares` 数据，并接 Share Access 创建 Role Assignment / Access Link、Delete、detail/edit sheet、schedule template、批量 schedule edit、target/schedule 过滤、review 计数、valid_from/valid_until schedule baseline、权限影响预览与批量 review |
| Access Link Claim | `features/access-links/pages/access-link-claim-page.tsx` | 已接 public `/access-link`、`/access-link/:token`、`/access-links/claim`，使用 `group_links/verify` 验证 secret / QR token，并展示 claim timestamp |
| Credentials | `features/credentials/pages/credentials-page.tsx` | 已接 `/api/v1/cards` + `/api/v1/card_assignments` adapter、单人/批量 Issue、batch audit 搜索/状态筛选/CSV 导出、Assign、Activate、Deactivate、Deassign、Revoke、delivery、physical card vendor/inventory、CSV import 与 physical card task detail sheet；已按 `credential_kind` 展示 Google Wallet / Apple Wallet / Physical Card；旧 `listWalletPasses` 已转接 `/cards` |
| Places | `features/places/pages/places-list-page.tsx` | 已接真实 adapter 与 Create Place Sheet；Place Settings 已接 General 保存、lockdown/cancel 和显式确认删除 |
| Event History | `features/event-history/pages/event-history-page.tsx` | 已接只读 adapter |
| Reports | `features/reports/pages/reports-page.tsx` | 已接 `/api/v1/reports`、`/api/v1/reports/:id/download` 与 `/api/v1/scheduled_reports` baseline，支持真实指标、排程列表与 CSV 导出 |
| Organization Setup | `features/organization/pages/organization-setup-page.tsx` | Alert Policies 已接 `/api/v1/alert_policies` 列表、Add、Save、Delete/Disable、Custom condition 与 Preview；Integrations/SSO 已接 `/api/v1/integrations` list/detail/Add/Edit/Disable，写流程已有浏览器 E2E；其余仍为样板 |

### 5.2 Place 页面

| 页面 | 当前模块 | 状态 |
|---|---|---|
| Place Dashboard | `features/places/pages/place-dashboard-page.tsx` | 已接只读 adapter |
| Place Users | `features/users/pages/users-page.tsx` | 已按当前 Place 过滤 users |
| Place Groups | `features/groups/pages/groups-page.tsx` | 复用 Groups 页并按 Place scope 展示访问组、门点、楼层、限制条件和链接 |
| Doors | `features/places/pages/door-detail-page.tsx` | 已接 Add Door、detail、保存、action、删除 |
| Floors | `features/places/pages/floors-page.tsx` | 已接真实 adapter、Add Floor Sheet 和 General Save |
| Hardware | `features/places/pages/hardware-page.tsx` | 已接 `controllers` / `readers` / `terminals` adapter、Add Hardware、门点绑定、config/reboot 命令和 Terminal Trigger |
| Unlock History | `features/event-history/pages/event-history-page.tsx` | 已接只读 adapter |
| Analytics | `features/reports/pages/reports-page.tsx` | 复用 Reports API，Place 视角按当前 Place scope 拉取 report/scheduled report |
| Operations | `features/places/pages/place-operations-page.tsx` | 预留样板页 |
| Place Settings | `features/places/pages/place-settings-page.tsx` | 样板页 |

### 5.3 旧页面替换关系

| 旧区域 | 新目标 |
|---|---|
| Dashboard | Home + Place Dashboard |
| Enterprise | Users + Teams + Organization Setup |
| Access | Groups + Access Rights + Place Groups |
| Wallet | Credentials |
| Spaces | Places + Floors + Doors |
| Gateways | Hardware |
| Events | Event History + Unlock History |
| Alarms/Audit | Reports |

---

## 6. API 契约

API 规则、目标 endpoint 表格、迁移期 adapter 对照和前后端进度追踪的唯一来源是 `MISTYISLET-KISI-API-SUMMARY.md`。本节只保留 UI 层面必须遵守的约束。

### 6.1 前端 adapter 约束

- 保留 fallback，单个接口失败不导致整页崩溃。
- adapter 类型留在 shell/resource 层，不泄露到用户可见文案。
- 页面组件只消费资源视图模型，后续替换正式 endpoint 时尽量不改页面。
- Place scope 不能只依赖前端过滤，正式接口必须由后端做权限约束。
- UI 文案 Doors 对应目标 API `locks`；UI 文案 Access Rights 对应目标 API `role_assignments` + `shares` + `groups`。

### 6.2 权限和范围

- Organization Admin 可以请求组织级资源，也可以请求 Place 级资源。
- Place Admin 只能请求被分配 Place 的资源。
- 迁移期只读 Place 账号也归入 Place Admin 视角，但后端必须限制其写入权限，不能只依赖前端隐藏入口。
- 所有 Place 级 endpoint 都必须在后端校验 scope。
- 新权限写入必须创建或更新 `role_assignments`，不能再扩展历史 `user.role` 枚举。
- 临时访问和邮件分享必须使用 `shares` 或 `group_links` 模型，不新增 `/access_rights/share`。

### 6.3 前后端 / API 文档对照进度

前后端对照进度的唯一来源是 `MISTYISLET-KISI-API-SUMMARY.md` §1.5。本文件不再重复维护该表格。

---

## 7. 当前实现状态

### 7.1 已完成

| 范围 | 结果 |
|---|---|
| Preview shell | `/home`、`/my-account`、`/users`、`/teams`、`/groups`、`/access-rights`、`/credentials`、`/places`、`/event-history`、`/reports`、`/organization/*`、`/places/:placeId/*` 已进入新 shell |
| Sidebar A/B | Organization Admin 与 Place Admin 两套导航已实现 |
| Navigation context | `currentView`、`selectedPlaceID`、`selectedPlaceName`、enter/back 已实现 |
| Canonical Place route | Place 卡片使用真实 `building.id`，Place 侧 Sidebar 跟随当前 `placeId` |
| Route module | `features/kisi-shell/routes.tsx` 已替代临时 `preview-routes.tsx` if/else renderer |
| 共享组件 | `components/kisi/primitives.tsx`、`components/kisi/data-display.tsx` 已新增 |
| 页面拆分 | 大型 preview 内容已拆到 feature pages |
| Home 瘦身 | `home-page.tsx` 从约 1931 行降到约 402 行 |
| Reference API 首批落地 | `/api/v1/places`、`/api/v1/locks`、`/api/v1/controllers`、`/api/v1/readers`、`/api/v1/terminals`、`/api/v1/groups`、`/api/v1/group_locks`、`/api/v1/group_zones`、`/api/v1/group_links`、`/api/v1/members`、`/api/v1/teams`、`/api/v1/team_memberships`、`/api/v1/cards`、`/api/v1/card_assignments`、`/api/v1/roles`、`/api/v1/role_assignments`、`/api/v1/shares`、`/api/v1/event_sets`、`/api/v1/events/meta`、`/api/v1/events/types`、`/api/v1/integrations`、`/api/v1/alert_policies` 已可用，`places/:id`、`locks/:id`、`role_assignments/:id`、`shares/:id`、`teams/:id`、`cards/:id`、`card_assignments/:id`、`integrations/:id`、`alert_policies/:id` 已支持 GET detail，`places/:id`、`locks/:id`、`teams/:id`、`role_assignments/:id`、`shares/:id`、`integrations/:id`、`alert_policies/:id` 已支持 PATCH/DELETE，`alert_policies` 已支持内置订阅、custom policy baseline、condition preview 与 event evaluate，`places/:id/lock_down`、`places/:id/cancel_lockdown`、`locks/:id/unlock`、`locks/:id/lock_down`、`locks/:id/cancel_lockdown` 已支持 action response，`controllers/:token/assign`、`readers/:token/assign`、controller-lock bind/unbind、controller config/reboot、reader/terminal reboot 和 terminal trigger 已支持 action route，`team_memberships` 已支持 create/delete，`cards/:id/revoke` 已支持撤销，`group_links/verify` 已支持 token 验证 |
| 权限层级收敛 | 后端新增内置 `Organization Admin` / `Place Admin` Role 和可持久化 Role Assignment；非管理账号不作为管理权限层级展示 |
| 资源 adapter | Places、Dashboard、Doors/Locks、Floors/Zones、Hardware、Events、Users、Credentials、Groups、Teams、Organization Setup 已接数据；Places 已接 Create 与 Settings 保存/action/delete confirmation，Doors 已接 Add Door、lock detail/save/action/delete，Floors 已接 Add Floor 与 General Save，Hardware 已切到 `controllers` / `readers` / `terminals` 并接 reference-style Add Hardware、门点绑定、config/reboot 命令和 Terminal Trigger，旧 `listGateways` 读 helper 已 reference-first 组合硬件资源并仅失败时 fallback，旧 gateway bind/unbind/config/reboot helper 已 reference-first 转接 controller action，Events 已切到 `event_sets`，Teams 已接 team/membership 写入与 team access assignment，Credentials 已切到 `cards` / `card_assignments` 并接单人/批量发放、batch audit 搜索/状态筛选/CSV 导出、delivery、physical card vendor/inventory、CSV import、physical card task 与 detail sheet，旧 wallet pass list helper 已转接 `cards`，Users 已接 Add/Invite、invitation queue/receipt/audit、Invitation Deliveries 历史与批量 Suspend/Enable，Groups 已接 `groups` 创建/保存/删除、限制条件保存、`group_locks` 添加/移除门点、`group_zones` 只读与 `group_links` Add/Edit/Delete，Access Rights 已切到 `roles` / `role_assignments` / `shares` 并接 Add/Edit/Delete、review 计数、target/schedule 过滤、schedule template baseline、批量 schedule edit、权限影响预览与批量 review，Organization Setup 已切到 `integrations` / `alert_policies` 列表与 Add/Save/Delete/Custom condition/Preview，Integrations 已接 detail/Edit/Disable，旧 `access-policies` 仅作 fallback |
| Sidebar 底部资源区 | 已固定在桌面左下角，Documentation 单行显示 |
| 品牌清理 | 用户可见品牌统一为 Mistyislet |
| Demo 登录 | 开启 demo users 后可使用组织管理员和 Place 管理员账号 |

### 7.2 继续工作的关键文件

| 文件 | 作用 |
|---|---|
| `web-admin/src/features/kisi-shell/resource-data.ts` | 旧后端资源到新 UI 资源视图的 adapter |
| `web-admin/src/features/kisi-shell/use-resource-summary.ts` | 共享 React Query hook 与 fallback |
| `web-admin/src/features/kisi-shell/routes.tsx` | 新 shell 正式 route module |
| `web-admin/src/features/kisi-shell/kisi-admin-shell.tsx` | Shell 与 Sidebar |
| `web-admin/src/context/navigation-context.tsx` | 导航上下文 |
| `web-admin/src/features/users/pages/users-page.tsx` | Organization Users / Place Users |
| `web-admin/src/features/credentials/pages/credentials-page.tsx` | Credentials |
| `web-admin/src/features/places/pages/*` | Place 资源页面 |
| `web-admin/src/features/event-history/pages/event-history-page.tsx` | Event History / Unlock History |
| `web-admin/src/features/kisi-shell/resource-data.test.ts` | Resource summary、Hardware、Access Rights mapper 单元测试 |

### 7.3 最近验证

最近通过的验证项：

- `go test ./internal/modules/wallet -run TestPhysicalCardInventory -count=1`
- `go test ./internal/http -run 'TestWalletPhysicalCardInventoryRoutes|TestOpenAPISpecDocumentsReferenceExtensionsAndErrors' -count=1`
- `go test ./internal/http -run TestOpenAPISpecDocumentsReferenceExtensionsAndErrors -count=1`
- `go test ./internal/http -count=1`
- `go test ./...`
- `go build -o /tmp/mistypass-api ./cmd/api`
- `git diff --check`
- `curl -sS http://127.0.0.1:8080/api/v1/openapi.json`
- `npm run typecheck`
- `npm run build`，仅有既有 chunk size warning
- `npm run test:unit`
- `curl -sS http://127.0.0.1:5173/`
- `curl -sS http://127.0.0.1:8080/healthz`
- demo 登录 API
- 浏览器 smoke：登录、打开 `/places`、进入真实 Place、验证 Dashboard/Doors/Floors/Hardware/Unlock History
- 浏览器 smoke：Teams 删除/成员移除确认、Floor 删除确认、Door Detail 删除确认、Hardware Deassign/Reboot/Terminal Trigger 确认、Credentials Deactivate/Deassign 确认、Alert Policy 删除确认
- 按钮 smoke：Kisi outline action button 普通态/hover 态 computed colors 对比正常
- Sidebar smoke：滚动长页面后，左下角资源入口位置不变，Documentation 单行显示
- 品牌 smoke：无旧品牌或基准方资源入口文案残留

### 7.4 当前状态与优先级

#### 已完成

| 分类 | 状态 |
|---|---|
| Shell / Navigation | Organization / Place 双视角、canonical Place route、正式 route module、资源页 lazy chunk 已落地 |
| Reference API baseline | Places、Locks、Controllers、Readers、Terminals、Groups、Group Links、Teams、Cards、Roles、Shares、Events、Reports、Integrations、Alert Policies 等主要 wrapper 已接入，Integrations 已补 detail/write |
| Place 拓扑写入 | Create Place、Place Settings 保存/action/delete confirmation、Add Floor、Floor Save、Add Area、Area Save、Add Door、Door detail/save/action/delete confirmation 已接 |
| Hardware 写入/命令 | Add Hardware、Controller/Reader assign、Controller Deassign、Reader Deassign、controller-lock bind/unbind、publish config、controller/reader/terminal reboot、terminal trigger 已接 |
| People & Access 写入 | Teams 创建/保存/删除、成员增删、Assign Access Right；Groups 创建/保存/删除、限制条件保存、门点绑定、Group Links Add/Edit/Delete；Access Rights Add/Edit/Delete 已接 |
| Credentials 写入 | Cards / Card Assignments、单人/批量发放、Batch Audit、Assign/Activate/Deactivate/Deassign/Revoke、Delivery、Apple Pass self-service baseline、实体卡 vendor/inventory/scan/CSV import/task baseline 和库存 status 前后端治理已接；Google Wallet、Apple Wallet 与 Physical Card 已用 `credential_kind` 区分 |
| 共享 UI | PageFrame、SettingsPanel、Search/Empty、RowActionsMenu、ConfirmActionDialog、Button readable states 已建立并在核心页面逐步迁移 |
| Destructive UX | Groups、Access Rights、Teams、Floor Detail、Door Detail、Place Settings、Alert Policies、Hardware Remove/Deassign/Publish Config/Reboot/Terminal Trigger、Credentials Deactivate/Deassign/Revoke、Physical Card Task 终态推进已接确认 |
| OpenAPI baseline | `GET /api/v1/openapi.json` 已输出 OpenAPI 3.0 baseline，覆盖 reference resources、Mistyislet extensions、legacy compatibility、operationId、pagination/error components 与认证 schemes |

#### 进行中

| 分类 | 当前推进 |
|---|---|
| Destructive UX 收口 | 主要资源删除/解绑/撤销、硬件 Publish Config/重启/触发和实体卡终态推进已接确认；legacy 全域写操作审计已完成：building/door/door-group/temporary-access/access-policy/user/user-group/visitor-pass/alarm/tenant/floor/area/MFA/state-replay/wallet config/template/pass issue/status/delivery/physical card/job/DLQ 全链路已补审计与回归；Organization Advanced 预留动作已禁用 |
| Row actions 收口 | Groups Links、Access Rights、Teams Members、Credentials 表格、Hardware door bindings、Alert Policies 已迁入；剩余 Settings/Advanced 等非表格操作继续按风险逐步统一 |
| API 语义收口 | Reference wrapper 覆盖面已大，reference destructive/write audit baseline 已覆盖 Place/Lock/Group/Group Link/Group Lock/Controller/Reader/Team/Team Membership/Role Assignment/Share/Card/Alert Policy，并已扩展 Team create/update、Team Membership create、Role Assignment create/update、Share create/update、Card create/assign/status/deassign/revoke 等关键写操作回归；Access Link claim 已补 `last_used_at` / `claimed_at` 写回与 `reference_group_link_claimed` 审计；collection 响应已补 pagination metadata 与 `limit` / `offset` 裁剪，CORS 已 expose `X-Collection-Range`，错误响应已补 `{error,message,code,status}` baseline；OpenAPI baseline 已补 `GET /api/v1/openapi.json`，覆盖 operationId、pagination/error components、extension 分组和 legacy archive；Place delete 已落归档语义，Terminal 已补 detail 且 lifecycle 保持 Reader assign/deassign；legacy gateway register/bind/unbind/device register/config publish/reboot 已补审计与回归；legacy building/door/door-group/temporary-access create 与 access policy create/update 已补审计与回归；Users detail/update/delete/invite/invitations/receipt/provider webhook API、client helper、User Detail UI 写入、Users Add/Invite/provider dispatch/批量启停、Access Rights schedule template/bulk schedule/review/impact preview/bulk review baseline、Reports baseline、Alert Policy custom baseline 与实体卡库存 status 前后端治理 baseline 已补 |
| Scope guard | Place Admin 视角可用；后端 `building_admin` scope 已从 token `building_ids` 扩展到 Role Assignment / Team Membership 推导，API-level URL 绕过回归已覆盖 Places/Locks/Shares/Role Assignments/Access Rights impact-review-schedule，Operator 只读写保护 smoke 已覆盖 Team/Card/Role Assignment/Share/Alert Policy 写入拒绝，浏览器 URL 绕过 E2E 已覆盖 unassigned Place direct route |
| 质量与体积 | 单测与构建稳定；主入口 bundle、旧后台并存、i18n 和视觉回归仍在推进 |

#### 未完成

| 分类 | 缺口 |
|---|---|
| Users | 批量治理已补齐：`batch-status`/`batch-delete`/`batch-invite`/`export-csv`/`import-csv` 后端 endpoint 和前端批量操作/CSV 导出导入已落地；剩余目录差异审阅和批量 role/group 细化 |
| Teams 高级治理 | SCIM source diff、成员批量导入、team access review 尚未实现 |
| Access Rights 高级语义 | target/schedule 过滤、review 计数、valid_from/valid_until baseline、schedule template baseline、批量 schedule edit、权限影响预览和批量 review 已接；剩余复杂时间窗、节假日/例外规则 |
| Reports 高级语义 | Reports 已接聚合指标、CSV 下载与 scheduled report baseline；剩余排程投递持久化、更多导出格式与模板参数化 |
| Organization Setup | SSO/SCIM 独立配置、Billing 仍待补齐；Alert Policy 事件驱动调度器已落地（自动评估 + cooldown + 通知查询），剩余渠道升级策略和 DB 持久化 |
| Credentials 高级治理 | Apple Pass `.pkpass` 签名与设备回调、制卡供应商真实 API 和实体卡任务全流程运营视图仍待补齐 |
| 质量收口 | 新 UI 文案 i18n、Organization/Place E2E、移动端视觉 smoke、legacy archive、OpenAPI 资源 schema 细化仍待完成 |

#### 优先级

| 优先级 | 事项 | 原因 |
|---|---|---|
| P0 | destructive mutation 审计已完成：reference 关键写操作 audit baseline 已覆盖全资源域；legacy 全域写操作审计（user/user-group/visitor-pass/alarm/tenant/floor/area/MFA/state-replay/wallet 全链路）已补齐并有回归测试；未来启用 Organization Advanced 写入前必须先接确认和审计 | 直接影响数据安全与用户误操作风险 |
| P0 | Place Admin scope guard 与 demo 登录稳定性回归；Role Assignment / Access Rights 跨 Place 写入 smoke 和 Operator 只读写保护 smoke 已补，继续补更多 Place route smoke | 这是两类管理账号可用性的底线 |
| P0 | 每轮保持 `typecheck`、unit、build、关键 smoke 通过 | 当前改动跨 API、adapter、UI，回归面大 |
| P1 | Users 批量治理与 Access Rights 复杂 schedule 规则/节假日例外 | 补齐 People & Access 的日常运营闭环 |
| P1 | Credentials Apple Pass `.pkpass`/设备回调、实体卡完整运营视图与供应商真实 API | Google Wallet、Apple Pass self-service、Access Link、实体卡 scan/inventory/task/status baseline 已可用，下一步是 Apple Wallet 真实协议和运营深度 |
| P2 | i18n、移动端视觉、旧后台 archive、OpenAPI 资源 schema 细化、bundle 拆分 | 影响体验和维护成本，但可在核心闭环稳定后集中处理 |

---

## 8. 路线图

### Phase 1：Shell 与路由边界

状态：大部分完成。

| 子项 | 状态 |
|---|---|
| 拆出 admin shell | 完成 |
| 拆出导航配置 | 完成 |
| 新增 navigation context | 完成 |
| canonical Place route | 完成 |
| 底部资源入口可点击且固定 | 完成 |
| preview route gate 从 App 移出 | 完成 |
| preview route renderer 替换成正式 route module | 完成 |
| route code splitting | 新 shell 资源页已拆包，主入口 chunk 已下降；旧后台与 App shell 仍需继续拆 |

### Phase 2：共享 UI 原子

状态：大部分完成。

| 子项 | 状态 |
|---|---|
| Page frame / breadcrumbs | 完成 |
| Settings panel / form field | 完成 |
| Search / filter / empty state | 完成 |
| Row actions menu | 推进中：新增共享 RowActionsMenu，Groups Links、Access Rights 表格、Teams Members、Credentials 表格、Hardware door bindings 与 Alert Policies 已迁入 |
| Confirm dialog 规范 | 推进中：新增共享 ConfirmActionDialog，Groups 删除、Group Link 删除、Access Right 删除、Team 删除、Team Member 移除、Floor 删除、Door Detail 删除、Hardware Remove/Deassign/Publish Config/Reboot/Terminal Trigger、Credentials Deactivate/Deassign/Revoke、Physical Card Task 终态推进和 Alert Policy 删除已迁入 |
| CSS token 收口 | 推进中：Button 普通态/悬停态对比已收口，Kisi action 按钮与 SettingsPanel tab hover 已增强，剩余表单、表格与 badge token 继续统一 |

### Phase 3：只读资源 API

状态：推进中。

| 子项 | 状态 | 下一步 |
|---|---|---|
| Places | `fetchPlace`、`createPlace`、`updatePlace`、`deletePlace`、place lockdown/cancel wrapper 完成；Places 页已接 Create，Place Settings 已接保存、action 和显式确认删除；Floors 页已接 Area 创建/编辑，Place -> Floor -> Area -> Door 写入闭环已补；delete 已写 reference audit log，Place 归档语义已落地并补回归 | 继续观察是否需要 Archived Places 管理入口 |
| Doors/Floors/Hardware | locks/floors scope 完成，lock detail/update/delete/action 完成，Door Detail 已接 Add Door，floor detail/update/delete 与 Add/Save 完成，controllers/readers/terminals reference-style wrapper 完成，Terminal detail 已补，独立 update/delete 决策为不落地；Hardware 已接 Add Hardware、门点 Bind/Remove、Publish Config/Reboot、Terminal Trigger、Controller/Reader Deassign；Publish Config/Reboot/Terminal Trigger 已改为确认后执行并固定命令目标；Hardware mapper 已补单元测试；legacy gateway 高危写/命令审计已补 | 转入高级运营治理 |
| Events | `event_sets`、`events/meta`、`events/types` wrapper 和前端 adapter 完成 | 补更细的筛选、详情抽屉字段和导出/report 衔接 |
| Users/Members | `place_id` 与 `/members` wrapper 完成；User detail/update/delete/invite/invitations/receipt/provider webhook endpoint、client helper、status/delete/invitation sent/receipt audit 与关联清理回归已补；User Detail UI 已接真实数据、保存、Suspend/Enable、Resend Invite、Invitation Deliveries provider metadata 和 Delete；Users 列表已接 Add User、Invite record creation + queue、provider dispatch 与批量 Suspend/Enable | 补批量治理深化 |
| Credentials | cards/card_assignments wrapper、前端 adapter、detail endpoint/detail sheet、批量发放、batch audit 搜索/状态筛选/CSV 导出、delivery、physical card vendor/inventory/scan baseline、CSV import、库存 status 前后端治理、physical card task 和撤销流完成；已补 `credential_kind` / `save_link`，区分 Google Wallet、Apple Wallet、Physical Card 与 Access Link；Apple Pass resident 自助 enrollment 与 Admin 管理 baseline 已接 | 补 Apple Pass `.pkpass` 签名/设备回调、制卡供应商真实 API 对接和完整运营视图 |
| Teams | Team / Team Membership endpoint、前端 adapter、创建、保存、删除、成员增删和 role assignment flow 完成 | 补更细的 SCIM source diff、成员批量导入和 team access review |
| Groups/Access Rights | Access Rights 已以 `role_assignments` + `shares` 为主数据源，并接 Role Assignment / Access Link 创建、详情编辑、删除、review 计数、target/schedule 过滤、valid_from/valid_until schedule baseline、schedule template baseline、批量 schedule edit、权限影响预览和批量 review；Groups 已接 `groups` 创建/保存/删除、限制条件保存、`group_locks` 添加/移除门点、`group_zones` 只读、`group_links` Add/Edit/Delete 与 token 验证；Access Link Claim public UI 已接 `/access-link`；legacy temporary-access create 与 access policy create/update 已补审计与回归 | 补复杂时间窗和节假日/例外规则；继续设计旧 access-policies 写模型到 reference 资源的安全迁移 |
| Organization Setup | Integrations 与 Alert Policies wrapper 和页面 adapter 完成，Integrations 已接 detail/Add/Edit/Disable 写入并补浏览器 E2E，Alert Policies 已接 Add/Save/Delete、Custom condition 写入、condition preview 和 event evaluate helper | 补 SSO/SCIM 独立配置、Alert Policy 调度器、渠道升级和持久化投递 |

### Phase 4：写入流

在 Phase 3 只读 hook 稳定后开始。

| 页面 | 写入流 |
|---|---|
| Users | User Detail 保存、Suspend/Enable、Resend Invite、Invitation Deliveries、删除已接；Add User Sheet、Invite record creation + queue/receipt baseline 和批量 Suspend/Enable 已接 |
| Teams | Create Team、Update Team、Delete Team、Create/Delete Team Membership、Assign Role Assignment |
| Groups | Create Group、Add Locks/Zones、限制条件编辑、Group Links Add/Edit/Delete |
| Access Rights | Create Role Assignment / Share：assignee -> group/place -> role -> schedule -> review |
| Credentials | Create Card、Batch Issue、Batch Audit、Card Assignment、Deactivate、Revoke、Delivery、Physical Card Task |
| Places | Create Place、Place Settings 保存、Place/Lock lockdown action、Delete Place 确认、Add Floor、Floor Save、Add Area、Area Save、Add Door、Add Hardware、Hardware Deassign 已接 |

### Phase 5：Place Admin 收口

| 子项 | 要求 |
|---|---|
| Place Dashboard | KPI 全部来自 Place scope |
| Place Users | 只展示该 Place 有访问权限的用户 |
| Place Groups | 只展示该 Place 的门/楼层/限制条件 |
| Doors/Floors/Hardware | 真实状态和拓扑 |
| Unlock History | 时间、动作、用户、门点筛选 |
| Place Settings | General/access/schedules/notifications/advanced 保存 |
| Place Admin scope guard | 后端 scope 已可从 Role Assignment / Team Membership 推导；API-level URL 绕过回归已覆盖未授权 Place detail、Lock detail、Lock create 与 Share create，浏览器 E2E 已覆盖 unassigned Place direct route；继续补只读账号写入保护 |

### Phase 6：Organization Setup

| 子项 | 要求 |
|---|---|
| Alert Policies | 规则、渠道、升级策略 CRUD；内置策略 Add/Save/Delete、custom policy baseline、condition preview 和 event evaluate 已接，后续补调度器、渠道升级与持久化投递 |
| Integrations | SSO、SCIM、HRIS、Webhook、MQTT |
| SSO & SCIM | 独立身份配置页面 |
| Billing | Plan、invoice、usage 模型预留 |
| Audit events | Setup 变更写入 Event History |
| API docs | `GET /api/v1/openapi.json` baseline 已接，operation 命名已对齐参考 API；后续补字段级 resource schemas 与文档站/生成 client 集成 |

### Phase 7：质量与旧页面迁移

| 子项 | 要求 |
|---|---|
| i18n | 新 UI 文案进入 `en-US`、`zh-CN`、`id-ID` |
| Unit tests | Navigation、scope guard、resource mapper |
| E2E | Organization Admin、Place Admin 两类路径 |
| Visual smoke | 桌面、平板、移动端无重叠 |
| Legacy archive | 被替换旧页面移入 `features/legacy` 或在功能等价后删除 |
| Bundle | 路由拆包，降低主 bundle |

---

## 9. 已知缺口

| 缺口 | 影响 | 所属阶段 |
|---|---|---|
| 主入口 bundle 仍偏大 | 新 shell 资源页已拆包，但 App shell 与旧后台入口仍触发 Vite chunk warning | Phase 1/7 |
| 资源 API 仍部分依赖 adapter | Places/Locks/Floors/Areas/Controllers/Readers/Terminals/Users/Groups/Group Links/Teams/Cards/Roles/Shares/Events/Integrations/Alert Policies 已有 wrapper 或目标 endpoint；主要硬件写入、Terminal Trigger 与 User detail/update/delete 已接 | Phase 3 |
| Teams 高级治理未实现 | Teams 已接 Add/Remove Member、Delete Team、Assign Access Right；SCIM source diff、批量导入和 review 仍待 Phase 4 | Phase 4 |
| 权限 API 高级编辑语义尚未完善 | Access Rights 已切到 role_assignments/shares，并接创建、详情编辑、删除、review/schedule baseline、schedule template baseline、批量 schedule edit、影响预览和批量 review；复杂 schedule 规则仍待 Phase 4 | Phase 4 |
| 创建/编辑/删除未实现 | 部分页面可读但不是完整工作台；Users 核心写入已接，剩余集中在高级治理和 Setup 预留页面 | Phase 4 |
| assigned Place 仍偏 demo 兼容 | building_admin 默认 Place 应由后端解析 | Phase 5 |
| 新 UI 文案未完全 i18n | 多语言环境仍有硬编码英文 | Phase 7 |
| 旧后台仍并存 | 体验割裂且 bundle 偏大 | Phase 7 |

---

## 10. 下一步

1. 继续补制卡供应商真实 API 对接、实体卡完整运营视图和 Alert Policies 调度器/渠道升级。
2. 继续补 Apple Pass `.pkpass` 签名、device registration 与 pass update callback。
3. 补 Users 批量治理深化，或补 Access Rights 复杂 schedule 规则/节假日例外。
4. 细化 OpenAPI resource schemas，并继续拆 App shell / legacy admin route，进一步降低主 bundle。
