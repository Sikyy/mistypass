# Mistyislet 管理后台 UI 重构执行计划

> 创建日期：2026-04-25
> 更新日期：2026-04-27
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
4. 下一批继续接 Cards create/assign/deassign、group_locks 写入、alert-policies resource endpoint 和正式 route module。

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

后续 API 不再以 `user.role` 作为主要权限模型。Bundled Reference 的权限层级是：

| 层级 | 目标 API | 作用 |
|---|---|---|
| Role | `/api/v1/roles` | 权限能力包，字段包括 `applies_to` 和 `permissions` |
| Role Assignment | `/api/v1/role_assignments` | 把 Role 分配给 User、Team 或 Guest，并绑定 Organization、Place 或 Group scope |
| Share | `/api/v1/shares` | 通过 email 向 Group 分享访问权限，支持 `valid_from`、`valid_until` |
| Team Membership | `/api/v1/team_memberships` | 把 User 或 Guest 加入 Team，Team 可作为权限受让者 |
| Group binding | `/api/v1/group_locks`、`/api/v1/group_zones` 等 | 把访问组绑定到门点、区域、电梯或终端 |

映射规则：

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
| My Account | `features/account/pages/my-account-page.tsx` | 已拆 Profile/Logins/Credentials/Security/API |
| Users | `features/users/pages/users-page.tsx` | 已接 `/api/v1/users` 只读 adapter |
| User Detail | `features/users/pages/user-detail-page.tsx` | 样板页 |
| Teams | `features/teams/pages/teams-page.tsx` | 已接 `/api/v1/teams` + `/api/v1/team_memberships` 只读 adapter |
| Groups | `features/groups/pages/groups-page.tsx` | 样板页 |
| Access Rights | `features/access-rights/pages/access-rights-page.tsx` | 已切 `role_assignments` + `shares` 只读数据，并复用搜索/筛选组件 |
| Credentials | `features/credentials/pages/credentials-page.tsx` | 已接 `/api/v1/cards` + `/api/v1/card_assignments` 只读 adapter，wallet passes 仅 fallback |
| Places | `features/places/pages/places-list-page.tsx` | 已接只读 adapter |
| Event History | `features/event-history/pages/event-history-page.tsx` | 已接只读 adapter |
| Reports | `features/reports/pages/reports-page.tsx` | 样板页 |
| Organization Setup | `features/organization/pages/organization-setup-page.tsx` | 样板页 |

### 5.2 Place 页面

| 页面 | 当前模块 | 状态 |
|---|---|---|
| Place Dashboard | `features/places/pages/place-dashboard-page.tsx` | 已接只读 adapter |
| Place Users | `features/users/pages/users-page.tsx` | 已按当前 Place 过滤 users |
| Place Groups | `features/groups/pages/groups-page.tsx` | 样板页 |
| Doors | `features/places/pages/door-detail-page.tsx` | 已接只读 adapter |
| Floors | `features/places/pages/floors-page.tsx` | 已接只读 adapter |
| Hardware | `features/places/pages/hardware-page.tsx` | 已接只读 adapter |
| Unlock History | `features/event-history/pages/event-history-page.tsx` | 已接只读 adapter |
| Analytics | `features/reports/pages/reports-page.tsx` | 样板页 |
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

### 6.1 参考规则

所有新增资源 API 都必须按 `Kisi-API-Bundled References.yaml` 与 `https://api.getkisi.com/docs#` 的 resource-oriented 风格设计，同时产品行为和仪表盘概念参考 `https://docs.kisi.io/`。

具体要求：

- 使用资源型 REST endpoint，不使用前端模块名作为 endpoint。
- 集合路径使用复数，例如 `/integrations`、`/places`、`/locks`、`/role_assignments`。
- OpenAPI operation 命名使用参考文档风格，例如 `fetchIntegrations`、`fetchIntegration`、`createIntegration`、`updateIntegration`、`deleteIntegration`。
- 查询参数和 JSON 字段使用 snake_case，例如 `place_id`、`group_id`、`organization_id`、`integration_id`。
- 集合查询使用 `ids`、`query`、`limit`、`offset` 和资源 scope query；正式响应应支持 `X-Collection-Range`。
- 写入 payload 应按对应资源的请求体建模，例如 `{ "place": {} }`、`{ "group": {} }`、`{ "role_assignment": {} }`，不沿用页面表单结构。
- UI 文案 Doors 对应目标 API `locks`；UI 文案 Access Rights 对应目标 API `role_assignments` + `shares` + `groups`。
- 前端 adapter 可以兼容旧接口，但新后端 endpoint 不应继续暴露旧的前端模块语义。

### 6.2 Mistyislet 目标 endpoint

Mistyislet 保留 `/api/v1` 前缀，但资源结构对齐上述参考格式。

| 资源 | 目标 endpoint | Operation 示例 | 当前状态 |
|---|---|---|---|
| Places | `GET /api/v1/places` | `fetchPlaces`、`fetchPlace` | 已落地 wrapper，前端 summary 已切 |
| Floors | `GET /api/v1/floors?place_id=` | `fetchFloors`、`fetchFloor` | 已支持 `place_id` 查询 |
| Doors UI / Locks API | `GET /api/v1/locks?place_id=` | `fetchLocks`、`fetchLock`、`unlockLock` | 已落地 wrapper，前端 summary 已切 |
| Hardware | `GET /api/v1/controllers`、`GET /api/v1/readers`、`GET /api/v1/terminals?place_id=` | `fetchControllers`、`fetchReaders`、`fetchTerminals` | 已落地只读 wrapper，前端 summary 已切 |
| Users | `GET /api/v1/users?place_id=`、`GET /api/v1/members?place_id=` | `fetchUsers`、`fetchMembers` | 已支持 `place_id` 与 members wrapper |
| Teams | `GET /api/v1/teams`、`GET /api/v1/team_memberships` | `fetchTeams`、`fetchTeamMemberships` | 已落地只读 wrapper，前端 summary 已切 |
| Groups | `GET /api/v1/groups`、`GET /api/v1/group_locks`、`GET /api/v1/group_zones` | `fetchGroups`、`fetchGroupLocks` | groups/group_locks 已落地 wrapper，group_zones 待补 |
| Roles | `GET /api/v1/roles` | `fetchRoles`、`fetchRole` | 已落地内置角色 |
| Role Assignments | `GET /api/v1/role_assignments` | `fetchRoleAssignments`、`createRoleAssignment` | 已落地 state + wrapper |
| Shares | `GET /api/v1/shares` | `fetchShares`、`createShare` | 已落地 temporary-access wrapper |
| Credentials | `GET /api/v1/cards`、`GET /api/v1/card_assignments` | `fetchCards`、`fetchCardAssignments` | 已落地 wallet passes wrapper，前端 summary 已切 |
| Events | `POST /api/v1/event_sets`、`GET /api/v1/event_sets/:id`、`GET /api/v1/events/meta`、`GET /api/v1/events/types` | `createEventSet`、`fetchEventSet`、`fetchEventMetadata`、`fetchEventTypes` | 已落地 reference wrapper，前端 summary 已切 |
| Integrations | `GET /api/v1/integrations` | `fetchIntegrations` | 已落地只读 wrapper，Organization Setup 已切 |

### 6.3 迁移期 adapter 规则

当前后端已有这些旧/现有接口：

- `GET /api/v1/buildings`
- `GET /api/v1/floors`
- `GET /api/v1/areas`
- `GET /api/v1/doors`
- `GET /api/v1/gateways`
- `GET /api/v1/events/access`
- `GET /api/v1/users`
- `GET /api/v1/wallet/passes`
- `GET /api/v1/user-groups`
- `GET /api/v1/door-groups`
- `GET /api/v1/access-policies`
- `GET /api/v1/temporary-access`

在正式资源 endpoint 完成前，前端通过 `features/kisi-shell/resource-data.ts` 映射到新 UI 资源模型。当前 summary 已优先使用 `/api/v1/places`、`/api/v1/locks`、`/api/v1/controllers`、`/api/v1/readers`、`/api/v1/groups`、`/api/v1/teams`、`/api/v1/team_memberships`、`/api/v1/cards`、`/api/v1/card_assignments`、`/api/v1/roles`、`/api/v1/role_assignments`、`/api/v1/shares`、`/api/v1/event_sets`；旧 `gateways`、`events/access`、`wallet/passes`、`access-policies`、`temporary-access` 仅作为 fallback。

约束：

- 保留 fallback，单个接口失败不导致整页崩溃。
- adapter 类型留在 shell/resource 层，不泄露到用户可见文案。
- 页面组件只消费资源视图模型，后续替换正式 endpoint 时尽量不改页面。
- Place scope 不能只依赖前端过滤，正式接口必须由后端做权限约束。

### 6.4 权限和范围

- Organization Admin 可以请求组织级资源，也可以请求 Place 级资源。
- Place Admin 只能请求被分配 Place 的资源。
- 迁移期只读 Place 账号也归入 Place Admin 视角，但后端必须限制其写入权限，不能只依赖前端隐藏入口。
- 所有 Place 级 endpoint 都必须在后端校验 scope。
- 新权限写入必须创建或更新 `role_assignments`，不能再扩展历史 `user.role` 枚举。
- 临时访问和邮件分享必须使用 `shares` 或 `group_links` 模型，不新增 `/access_rights/share`。

---

## 7. 当前实现状态

### 7.1 已完成

| 范围 | 结果 |
|---|---|
| Preview shell | `/home`、`/my-account`、`/users`、`/teams`、`/groups`、`/access-rights`、`/credentials`、`/places`、`/event-history`、`/reports`、`/organization/*`、`/places/:placeId/*` 已进入新 shell |
| Sidebar A/B | Organization Admin 与 Place Admin 两套导航已实现 |
| Navigation context | `currentView`、`selectedPlaceID`、`selectedPlaceName`、enter/back 已实现 |
| Canonical Place route | Place 卡片使用真实 `building.id`，Place 侧 Sidebar 跟随当前 `placeId` |
| 共享组件 | `components/kisi/primitives.tsx`、`components/kisi/data-display.tsx` 已新增 |
| 页面拆分 | 大型 preview 内容已拆到 feature pages |
| Home 瘦身 | `home-page.tsx` 从约 1931 行降到约 402 行 |
| Reference API 首批落地 | `/api/v1/places`、`/api/v1/locks`、`/api/v1/controllers`、`/api/v1/readers`、`/api/v1/terminals`、`/api/v1/groups`、`/api/v1/group_locks`、`/api/v1/members`、`/api/v1/teams`、`/api/v1/team_memberships`、`/api/v1/cards`、`/api/v1/card_assignments`、`/api/v1/roles`、`/api/v1/role_assignments`、`/api/v1/shares`、`/api/v1/event_sets`、`/api/v1/events/meta`、`/api/v1/events/types`、`/api/v1/integrations` 已可用 |
| 权限层级收敛 | 后端新增内置 `Organization Admin` / `Place Admin` Role 和可持久化 Role Assignment；非管理账号不作为管理权限层级展示 |
| 资源 adapter | Places、Dashboard、Doors/Locks、Floors、Hardware、Events、Users、Credentials、Groups、Teams、Organization Integrations 已接只读数据；Hardware 已切到 `controllers` / `readers` / `terminals`，Events 已切到 `event_sets`，Credentials 已切到 `cards` / `card_assignments`，Access Rights 已切到 `roles` / `role_assignments` / `shares`，Organization Setup 已切到 `integrations`，旧 `gateways` / `events/access` / `wallet passes` / `access-policies` / `temporary-access` 仅作 fallback |
| Sidebar 底部资源区 | 已固定在桌面左下角，Documentation 单行显示 |
| 品牌清理 | 用户可见品牌统一为 Mistyislet |
| Demo 登录 | 开启 demo users 后可使用组织管理员和 Place 管理员账号 |

### 7.2 继续工作的关键文件

| 文件 | 作用 |
|---|---|
| `web-admin/src/features/kisi-shell/resource-data.ts` | 旧后端资源到新 UI 资源视图的 adapter |
| `web-admin/src/features/kisi-shell/use-resource-summary.ts` | 共享 React Query hook 与 fallback |
| `web-admin/src/features/kisi-shell/preview-routes.tsx` | 临时 preview 路由分发 |
| `web-admin/src/features/kisi-shell/kisi-admin-shell.tsx` | Shell 与 Sidebar |
| `web-admin/src/context/navigation-context.tsx` | 导航上下文 |
| `web-admin/src/features/users/pages/users-page.tsx` | Organization Users / Place Users |
| `web-admin/src/features/credentials/pages/credentials-page.tsx` | Credentials |
| `web-admin/src/features/places/pages/*` | Place 资源页面 |
| `web-admin/src/features/event-history/pages/event-history-page.tsx` | Event History / Unlock History |
| `web-admin/src/features/kisi-shell/resource-data.test.ts` | Access Rights reference mapper 单元测试 |

### 7.3 最近验证

最近通过的验证项：

- `npm run typecheck`
- `npm run build`，仅有既有 chunk size warning
- `npm run test:unit`
- `go test ./...`
- `curl -sS http://127.0.0.1:5173/`
- `curl -sS http://127.0.0.1:8080/healthz`
- demo 登录 API
- 浏览器 smoke：登录、打开 `/places`、进入真实 Place、验证 Dashboard/Doors/Floors/Hardware/Unlock History
- Sidebar smoke：滚动长页面后，左下角资源入口位置不变，Documentation 单行显示
- 品牌 smoke：无旧品牌或基准方资源入口文案残留

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
| preview route renderer 替换成正式 route module | 待做 |
| route code splitting | 待做 |

### Phase 2：共享 UI 原子

状态：大部分完成。

| 子项 | 状态 |
|---|---|
| Page frame / breadcrumbs | 完成 |
| Settings panel / form field | 完成 |
| Search / filter / empty state | 完成 |
| Row actions menu | 待做 |
| Confirm dialog 规范 | 待做 |
| CSS token 收口 | 待做 |

### Phase 3：只读资源 API

状态：推进中。

| 子项 | 状态 | 下一步 |
|---|---|---|
| Places | reference wrapper 完成 | 补 `fetchPlace`、`updatePlace`、`deletePlace` 和页面写入流 |
| Doors/Floors/Hardware | locks/floors scope 完成，controllers/readers/terminals 只读 wrapper 完成 | 补 lock detail/action、controller/reader/terminal 写入和命令流 |
| Events | `event_sets`、`events/meta`、`events/types` wrapper 和前端 adapter 完成 | 补更细的筛选、详情抽屉字段和导出/report 衔接 |
| Users/Members | `place_id` 与 `/members` wrapper 完成 | 补 User detail/update/delete |
| Credentials | cards/card_assignments 只读 wrapper 和前端 adapter 完成 | 补 create/assign/deassign 与详情写入流 |
| Teams | 只读 endpoint 和前端 adapter 完成 | 补 Team / Team Membership 创建、删除和 role assignment flow |
| Groups/Access Rights | Access Rights 已以 `role_assignments` + `shares` 为主数据源，旧 adapter fallback 保留 | 补 group_locks 写入与后续创建/编辑 flow |
| Organization Setup | Integrations 只读 wrapper 和页面 adapter 完成 | 补 alert-policies resource endpoint、Integration detail/write flow |

### Phase 4：写入流

在 Phase 3 只读 hook 稳定后开始。

| 页面 | 写入流 |
|---|---|
| Users | Add User Sheet、Invite、Suspend/Enable、详情保存 |
| Teams | Create Team、Create Team Membership、Assign Role Assignment |
| Groups | Create Group、Add Locks/Zones、限制条件编辑 |
| Access Rights | Create Role Assignment / Share：assignee -> group/place -> role -> schedule -> review |
| Credentials | Create Card、Card Assignment、Deactivate、Group Link expiry |
| Places | Create Place、Add Door、Add Floor、Add Hardware |

### Phase 5：Place Admin 收口

| 子项 | 要求 |
|---|---|
| Place Dashboard | KPI 全部来自 Place scope |
| Place Users | 只展示该 Place 有访问权限的用户 |
| Place Groups | 只展示该 Place 的门/楼层/限制条件 |
| Doors/Floors/Hardware | 真实状态和拓扑 |
| Unlock History | 时间、动作、用户、门点筛选 |
| Place Settings | General/access/schedules/notifications/advanced 保存 |
| Place Admin scope guard | 隐藏入口不能通过 URL 绕过；只读账号不能调用写入接口 |

### Phase 6：Organization Setup

| 子项 | 要求 |
|---|---|
| Alert Policies | 规则、渠道、升级策略 CRUD |
| Integrations | SSO、SCIM、HRIS、Webhook、MQTT |
| SSO & SCIM | 独立身份配置页面 |
| Billing | Plan、invoice、usage 模型预留 |
| Audit events | Setup 变更写入 Event History |
| API docs | 新资源 endpoint 写入 OpenAPI，operation 命名对齐参考 API |

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
| `preview-routes.tsx` 仍是临时路由层 | 还不是最终路由架构 | Phase 1/7 |
| 资源 API 仍部分依赖 adapter | Places/Locks/Controllers/Readers/Terminals/Groups/Teams/Cards/Roles/Shares/Events/Integrations 已有 wrapper，alert-policies 与部分写入接口仍待补 | Phase 3 |
| Teams 写入流未实现 | Teams 只读已接 live endpoint，但 Add Member / Delete Team / Assign Access Right 仍待 Phase 4 | Phase 4 |
| 权限 API 写入流尚未实现 | Access Rights 只读页已切到 role_assignments/shares，但创建/编辑/删除仍待 Phase 4 | Phase 4 |
| 创建/编辑/删除未实现 | 页面可读但不是完整工作台 | Phase 4 |
| assigned Place 仍偏 demo 兼容 | building_admin 默认 Place 应由后端解析 | Phase 5 |
| 新 UI 文案未完全 i18n | 多语言环境仍有硬编码英文 | Phase 7 |
| 旧后台仍并存 | 体验割裂且 bundle 偏大 | Phase 7 |

---

## 10. 下一步

1. 为 Places/Floors/Locks/Hardware/Events/Users/Groups/Credentials adapter 增加 mapper 单元测试。
2. 将 `preview-routes.tsx` 替换为正式 route module。
3. 后端继续补 Cards create/assign/deassign、group_locks 写入和 alert-policies resource endpoint。
4. 进入 Phase 4 时补 Team / Team Membership 创建、删除和 Assign Access Right flow。
