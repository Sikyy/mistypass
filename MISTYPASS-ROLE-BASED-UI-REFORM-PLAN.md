# Mistyislet 历史角色化前端整改计划

> 2026-04-26 更新：本文件记录早期角色化整改过程，后续开发以 `MISTYPASS-KISI-UI-REFORM-PLAN.md` 与 `MISTYISLET-KISI-API-SUMMARY.md` 为准。新的产品权限只对外暴露 `Organization Admin` 与 `Place Admin` 两类管理人员；API 权限层级按 Kisi Bundled Reference 的 `roles`、`role_assignments`、`shares` 设计。本文中的 `super_admin`、`tenant_admin`、`building_admin`、`operator`、`resident` 只作为历史实现和迁移期兼容说明，不再作为新产品信息架构。

## 1. 目标与边界

本次整改的核心目标不是继续堆功能，而是把 Mistyislet 管理后台改成按客户类型、角色权限和真实运营任务组织的产品界面。

必须解决的问题：

- 不同登录角色看到不同后台，尤其区分 SaaS 平台管理方、写字楼/楼宇管理员、写字楼内办公室/企业管理员。
- 事件、告警、Enterprise worker、Wallet queue 等页面过度复杂，需要按“今天要处理什么”重组，而不是把所有底层对象平铺。
- 文案、UI 样式、固定宽度、截断、无意义选框、占位文案、深浅主题冲突需要统一审查和修复。
- 设计方向需要同时吸收 `DESIGN.md` 的企业级克制白色卡片系统，以及 `picture/generated-1776874282565.jpg` 的黑灰雾岛指挥中心表达。

边界：

- 第一阶段不改变后端功能和 API 合同。
- 第一阶段以前端角色能力矩阵、导航可见性、组件分区、文案和样式整改为主。
- 如后续发现现有角色不足以表达真实客户模型，再单独提出后端最小增量方案，不在本轮直接改。

## 2. 项目与客户判断

MistyPass 当前是一个云端门禁控制 SaaS 平台，不是普通办公系统后台。

当前仓库结构：

- `api/`：Go 后端，负责认证、租户、楼宇/空间、门禁策略、Wallet 凭证、网关、事件、告警、审计、Enterprise HRIS/SSO/JIT 等能力。
- `web-admin/`：React + TypeScript + Vite 管理后台，使用 Tailwind/shadcn、TanStack Query/Table、i18next、多角色路由守卫。
- `docs/`：架构、API、测试、部署、Sprint 和外部 Wiki 文档。
- `picture/`：品牌 logo、设计理念和概念图。

服务客户：

- SaaS 平台运营方：管理所有租户、跨租户健康度、全局审计、平台级网关/队列/状态。
- 楼宇/物业管理方：管理指定楼宇、楼层、区域、门点、网关、事件和告警。
- 办公室/企业租户管理员：管理自己公司或办公室内的人员、组、权限、凭证、访客和可用空间。
- 一线运营人员：查看事件、告警、凭证状态、网关状态，执行有限处置，不做结构性配置。
- 普通员工/住户：不应进入 web-admin，应该走移动端或自助凭证体验。

## 3. 当前审查结论

### 3.1 当前角色模型

前端当前角色定义在 `web-admin/src/lib/api.ts`：

- `super_admin`
- `tenant_admin`
- `operator`
- `building_admin`
- `resident`

前端权限判断集中在 `web-admin/src/lib/viewer.ts`，目前是函数式散落判断，例如：

- `super_admin` 可以看 `/tenants`。
- `super_admin / tenant_admin / operator` 可以看 `/enterprise`。
- `super_admin / tenant_admin` 可以看 `/access`。
- `super_admin / tenant_admin / operator` 可以看 `/wallet`。
- `super_admin / tenant_admin / operator / building_admin` 可以看 `/events` 和 `/alarms`。
- `building_admin` 可以看 `/spaces` 和 `/gateways`。

问题：

- 当前只是“页面级 allow/deny”，没有定义客户类型和工作台语义。
- `operator` 能进入 Enterprise，容易把一线运营人员带进 HRIS/SSO/worker/DLQ 这类平台配置型界面。
- `building_admin` 前端隐藏 Access，但后端部分 Access API 允许 `building_admin`，需要产品上明确到底是“楼宇物理资产运营”还是“楼宇人员权限管理员”。
- `resident` 不应有管理后台入口，但当前类型存在，需要明确只作为移动端/自助端角色。

### 3.2 页面复杂度热点

当前前端复杂度主要集中在以下文件：

- `web-admin/src/components/enterprise/enterprise-alerts-workspace.tsx`：5336 行。
- `web-admin/src/features/enterprise/pages/enterprise-page.tsx`：3996 行。
- `web-admin/src/features/wallet/pages/wallet-page.tsx`：3192 行。
- `web-admin/src/components/enterprise/enterprise-sync-workspace.tsx`：1779 行。
- `web-admin/src/features/gateways/pages/gateways-page.tsx`：1447 行。
- `web-admin/src/features/spaces/pages/spaces-page.tsx`：1379 行。
- `web-admin/src/features/events/pages/events-page.tsx`：741 行。

判断：

- Enterprise Alerts 不是一个普通组件，而是多个产品域堆叠：审批、同步异常、worker alert、HRIS receipt、DLQ、执行历史。
- Wallet 页面同时承载模板、发放、投递、物理卡、队列、趋势、告警订阅、DLQ，信息密度过高。
- Events 页面本身行数不算最高，但概念上与 Alarms、Audit、Gateway checkpoint、Enterprise worker alert 混在一起，用户会把“事件”理解成所有日志入口。

### 3.3 文案和 i18n 问题

已扫描 `web-admin/src/locales`，发现：

- `en-US.json` 仍有 115 个明显占位值，例如 `description`、`hint`、`card description`、`description attention`。
- `zh-CN.json` 仍有 20 个明显占位值，例如 `说明`、`卡片标题`、`卡片说明`、`指标`。
- `id-ID.json` 当前没有命中同类占位值。
- `npm run check:cjk` 当前失败，位置是 `web-admin/src/features/wallet/pages/wallet-page-utils.ts` 中的硬编码中文关键词 `实体` 和 `临时`。
- `docs/testing/admin-ui-test-and-api-map.md` 中仍写着 token 存在 `localStorage` 的旧行为，但当前 `web-admin/src/lib/auth.ts` 实际使用 `sessionStorage` 的 `mistypass_admin_access_token` 和 `mistypass_admin_refresh_token`，并有 refresh token 流程。

判断：

- 中文 UI 当前不是单纯翻译不完整，而是有不少占位文案已经进入可见界面。
- 部分 `defaultValue` 还在 TSX 中承担实际文案，后续需要收口到 i18n 文件。
- 测试文档与真实认证实现不一致，会继续误导登录和联调排障。

### 3.4 样式、溢出和交互问题

已扫描以下风险模式：

- 固定宽度：`w-[...]`、`min-w-[...]`、`max-w-[...]`、`grid-cols-[...]`。
- 强截断：`truncate`、`whitespace-nowrap`。
- 表格密集列：多处 `TableCellText` 配合 `max-w`，中文、印尼语、长租户名、长门点名容易被截断。
- 选框与禁用态：大量 `SelectTrigger`、`SelectValue` 和 `disabled`，需要逐个判断是否存在“只有一个选项还让用户选”“禁用但没有解释”“选框影响范围不清”的情况。

重点风险文件：

- `web-admin/src/App.tsx`
- `web-admin/src/features/events/pages/events-page.tsx`
- `web-admin/src/features/alarms/pages/alarms-page.tsx`
- `web-admin/src/features/spaces/pages/spaces-page.tsx`
- `web-admin/src/features/gateways/pages/gateways-page.tsx`
- `web-admin/src/components/gateways/*`
- `web-admin/src/components/enterprise/*`
- `web-admin/src/components/wallet/*`
- `web-admin/src/components/access/*`

### 3.5 设计方向冲突

`DESIGN.md` 要求的是 Cohere 风格：

- 明亮白色画布。
- 22px 圆角卡片。
- 黑白灰为主。
- 极克制的蓝色交互。
- 企业级、干净、可信。

`picture/设计理念.md` 和 `generated-1776874282565.jpg` 要求的是 MistyIslet/MistyPass 风格：

- 黑灰雾气。
- 抽象岛屿。
- 银色高光。
- 左侧 command rail。
- 多地办公室站点图。
- 心跳/健康度模块。
- Wallet 凭证分布作为核心运营模块。

判断：

- 不应全站继续做重玻璃深色，否则表格、表单和长文案可读性会变差。
- 不应完全改成白色企业官网风，否则会丢失 MistyPass 的雾岛品牌和安全指挥中心识别度。
- 目标应是“深色指挥中心外壳 + 高可读任务卡片”。Dashboard、登录页、平台总览保留黑灰雾岛氛围；表格、表单、审批、配置区使用更亮、更干净的企业任务卡。

## 4. 目标角色与功能分区

### 4.1 SaaS 平台管理方：`super_admin`

定位：

平台控制平面，管理所有租户和跨租户安全运营。

应看到：

- Platform Dashboard：跨租户总览、租户健康度、网关在线率、告警量、Wallet 队列风险。
- Tenants：租户生命周期、租户详情、跨租户拓扑。
- Enterprise：只作为租户目录/SSO/HRIS 的跨租户观察和排障入口，不应混同为普通租户工作台。
- Gateways：跨租户网关、序列号库存、checkpoint、OTA、命令。
- Events：跨租户事件检索和高级筛选。
- Alarms：跨租户告警处置。
- Audit/State：当前已有 API 和页面文件但路由未完全挂载，后续作为平台管理方专属入口。

不应默认看到：

- 某一个租户内部的日常操作表单，除非明确进入租户上下文。

### 4.2 办公室/企业租户管理员：`tenant_admin`

定位：

单租户组织管理员。这里的“租户”是 SaaS 数据隔离单位，不代表它还能继续管理下级租户。

应看到：

- Tenant Dashboard：本组织人员、门点、凭证、告警、事件。
- Directory & SSO：企业目录、HRIS/SCIM/CSV、SSO、JIT 审批。当前 `/enterprise` 应在此角色下改名或重组为“目录与登录”，避免让用户以为这是平台级租户管理。
- People & Access：用户、用户组、策略、临时权限、访客授权。
- Credentials：Wallet 凭证模板、发放、投递、状态维护。
- Spaces：本租户楼宇/区域/门点。
- Gateways：本租户网关和边缘状态。
- Events/Alarms：本租户事件检索和告警处置。

不应看到：

- `/tenants` 租户列表。
- 跨租户选择器。
- 平台级 state replay、全局审计、全局序列号策略。

### 4.3 写字楼/楼宇管理员：`building_admin`

定位：

只管理被分配 `building_ids` 的物理楼宇和现场运营。

应看到：

- Building Dashboard：本楼宇门点、网关、告警、最近事件、在线率。
- Spaces：仅本楼宇的楼层、区域、门点。
- Gateways：仅本楼宇网关、绑定、配置发布、重启、checkpoint。
- Events：仅本楼宇访问事件和设备事件。
- Alarms：仅本楼宇告警处置。

不应看到：

- `/tenants`。
- Enterprise/HRIS/SSO/JIT。
- Wallet 高级凭证模板、跨组织发放队列、DLQ。
- 跨租户或跨楼宇选择器。

需要确认的产品决策：

- 如果楼宇管理员也要管理员工权限，才开放 Access 的楼宇范围版。
- 如果只是物业/设备运维，Access 继续隐藏，避免越权和概念混乱。

### 4.4 一线运营人员：`operator`

定位：

只读或有限处置角色，负责看状态和处理队列，不做结构性配置。

应看到：

- Operations Dashboard：待处理告警、事件、队列、网关异常。
- Events：事件检索。
- Alarms：告警状态流转。
- Gateways：状态、checkpoint、库存只读或导出。
- Credentials：凭证状态、发放结果、投递回执只读或有限重试。

不应看到：

- Enterprise 配置型入口。
- 租户生命周期。
- HRIS secret、IdP secret、Webhook secret、全局策略写入。

整改建议：

- 前端先移除 `operator` 对 `/enterprise` 的导航入口。
- 如必须保留排障能力，提供只读“同步健康”轻量入口，不暴露完整 Enterprise workspace。

### 4.5 普通员工/住户：`resident`

定位：

移动端或自助端用户，不进入 web-admin。

应看到：

- web-admin 登录后应进入无权限页或引导页。
- 后续如果有自助端，应展示自己的凭证、访客申请、设备绑定或通行记录。

## 5. 新信息架构

### 5.1 全局导航分组

建议将当前单层导航重组为四类：

- Command：Dashboard、Alarms、Events。
- Sites：Spaces、Gateways。
- People：Directory & SSO、Access、Credentials。
- Platform：Tenants、Audit、State、Integrations。

不同角色只显示自己有意义的分组：

- `super_admin`：Command、Sites、People、Platform。
- `tenant_admin`：Command、Sites、People。
- `operator`：Command、Sites、Credentials 只读。
- `building_admin`：Command、Sites。
- `resident`：不显示管理后台导航。

### 5.2 租户选择器规则

- 只有 `super_admin` 才显示跨租户选择器。
- `tenant_admin` 使用 token 中的 `tenant_id`，界面不显示“租户切换”。
- `building_admin` 使用 `building_ids`，界面显示“当前楼宇范围”，不显示跨楼宇全局选择器。
- 如果某个筛选只有一个有效选项，默认展示为静态范围标签，不使用 select。
- 如果 select 被禁用，必须给出原因，例如“当前角色固定在本租户范围，不支持切换”。

### 5.3 Enterprise 重命名和拆分

当前 `/enterprise` 在不同角色下语义不一致，建议拆为前端层面的工作区：

- `Directory & SSO`：面向 `tenant_admin`，管理员工目录、SSO、HRIS、JIT。
- `Enterprise Health`：面向 `super_admin`，跨租户看同步健康和配置风险。
- `Sync Inbox`：面向需要排障的只读角色，只显示 worker alert、失败原因和跳转建议。

第一阶段可以不改 URL，先改导航名称、页面头部、可见区块和默认 tab。

## 6. 事件与告警降复杂方案

### 6.1 Events 页面

目标：

Events 应该是“事件检索与时间线”，不是所有系统日志的垃圾桶。

保留：

- 访问事件。
- 设备事件。
- 关键字段筛选：时间、类型、租户/楼宇范围、门点、人员、关键字。

改造：

- 默认显示“最近 24 小时需要关注的事件”。
- 顶部只保留 3 到 4 个关键指标：访问量、拒绝量、设备异常、离线相关事件。
- 主体改为 timeline + detail drawer，减少宽表。
- 高级筛选折叠，不默认露出所有字段。
- `building_admin` 默认固定楼宇范围。
- `operator` 默认看处置相关事件。
- `super_admin` 才显示租户列和跨租户筛选。

### 6.2 Alarms 页面

目标：

Alarms 应该是“事件升级后的处置队列”。

改造：

- 默认按状态分组：Open、Investigating、Mitigated、Closed。
- 每条告警必须有下一步动作：确认、调查、缓解、升级、误报、关闭。
- 通知策略和通知日志收进二级区块，不和主告警列表抢视线。
- 楼宇管理员只能看到本楼宇。

### 6.3 Enterprise Alerts 拆分

当前 `enterprise-alerts-workspace.tsx` 过大，建议拆为：

- Approval Inbox：JIT 审批。
- Sync Exceptions：目录同步异常。
- Worker Alerts：worker 阈值告警。
- HRIS Receipts：webhook receipt 处理。
- HRIS DLQ：DLQ 回放和失败原因。

每个区块都使用“摘要卡 + 处理队列 + 详情抽屉”，避免在一个页面里横向堆满所有操作。

## 7. 视觉与 UI 体系

### 7.1 视觉原则

采用混合体系：

- 外壳：黑灰雾岛指挥中心，来自 `generated-1776874282565.jpg`。
- 任务卡：高可读的白/雪灰企业卡片，来自 `DESIGN.md`。
- 关键图形：多地办公室岛屿地图、站点心跳、网关健康线、凭证分布环图。
- 圆角：主卡 22px，符合 `DESIGN.md`。
- 颜色：黑、白、冷灰、银色高光为主；蓝色只做 focus/hover；红/黄/绿只做状态语义。

### 7.2 页面密度规则

- 首屏只放“当前角色今天最该看的 5 件事”。
- 配置表单不和运营队列同屏竞争。
- 高危操作进入二级抽屉或确认弹窗。
- 表格默认列不超过 6 列，其他信息进入详情抽屉或列配置。
- 移动端优先卡片化，避免宽表横向溢出。

### 7.3 溢出修复规则

- 不用固定 `220px/180px/160px` 承载可翻译文本，改为 `minmax(0, 1fr)` 或响应式 stack。
- 只在 ID、邮箱、序列号等低语义字段使用截断；业务文案默认换行。
- 表格单元格支持 tooltip 或详情抽屉，不直接截断关键状态原因。
- `SelectTrigger` 需要 `min-w-0` 和可换行容器配合，长语言不撑破布局。
- 筛选区在窄屏改为单列，不保留桌面 grid template。

## 8. 文案整改计划

第一批必须修：

- 替换 `zh-CN.json` 中的 `说明`、`卡片标题`、`卡片说明`、`指标`。
- 替换 `en-US.json` 中的 `description`、`hint`、`card description` 等占位文案。
- 把 TSX 中仍承担实际展示的 `defaultValue` 收口到 locale 文件。
- 修复 `wallet-page-utils.ts` 的硬编码中文关键词，保证 `npm run check:cjk` 通过。
- 更新 `docs/testing/admin-ui-test-and-api-map.md` 的 token 存储描述：当前是 `sessionStorage`，不是旧的 `localStorage` 单 token。

文案原则：

- 按角色说话，不用平台内部术语吓用户。
- 把“为什么看不到/为什么不能点”写清楚，减少权限误解。
- 删除“模块标签”“提示”“说明”这类占位词。
- 对危险操作写结果，例如“将重启网关并产生短暂离线”，不要只写“执行”。

## 9. 无意义选框和禁用态整改计划

逐页审查所有 Select、Switch、禁用按钮：

- 只有一个选项：改为静态标签。
- 由角色固定的选项：改为范围说明，不让用户误以为可切换。
- 无数据导致不能选：显示空态和下一步，例如“先创建楼宇后才能选择门点”。
- 权限不足导致不能选：显示权限边界，例如“当前角色只能查看，不能修改策略”。
- 联动筛选：明确上游选择依赖，避免用户看到空下拉。

高优先级页面：

- Spaces：租户、楼宇、楼层、区域、门点联动。
- Gateways：租户、门点、产品类型、库存状态、checkpoint 窗口。
- Access：授权范围、发放方式、用户组、策略状态。
- Wallet：模板、目标、执行模式、投递通道、物理卡任务。
- Enterprise：租户、同步来源、worker 状态、receipt/DLQ 状态。

## 10. 技术实施阶段（详细）

> 以下为结合 `DESIGN.md`、`MISTYPASS-FRONTEND-UI-REFORM-PLAN.md`（已归档）、当前 shadcn/ui 组件库和代码审查结论制定的细化实施计划。
> 每个阶段内的任务按依赖顺序排列，同阶段内无依赖的任务可并行。

---

### 阶段 0：计划和基线 ✅ 已完成

- [x] 新增本计划文档。
- [x] 记录当前角色矩阵、复杂度热点、文案问题、样式风险。
- [x] 不改后端。

---

### 阶段 1：角色能力矩阵与路由守卫 ✅ 已完成

- [x] 在 `viewer.ts` 建立声明式 `ROLE_CAPABILITIES` 矩阵（14 个 capability × 5 个角色）。
- [x] 收紧 `operator` 权限：不再进入完整 Enterprise。
- [x] `App.tsx` 导航接入角色化命名（平台管理员 → “企业健康”，租户管理员 → “目录与登录”）。
- [x] 隐藏运营角色无权限的跳转入口。
- [x] 修复 `wallet-page-utils.ts` 硬编码中文，`check:cjk` 通过。
- [x] 清理 `zh-CN.json` 首批占位文案。
- [x] 更新 `role-boundary-e2e.spec.ts` 覆盖 `operator` 拒绝 `/enterprise`。

---

### 阶段 2：导航分组与工作台重组 ✅ 已完成

目标：将当前单层导航重组为语义化分组，不同角色只看到有意义的区块。

#### 2.1 Sidebar 导航分组 ✅ 已完成

将 `buildNavItems` 从平铺列表改为分组结构：

```
Command    — Dashboard, Alarms, Events
Sites      — Spaces, Gateways
People     — Directory & SSO, Access, Credentials (Wallet)
Platform   — Tenants, Audit (仅 super_admin)
```

涉及文件：
- `web-admin/src/App.tsx` — `NavItem` 类型增加 `group` 字段，`buildNavItems` 按组生成，Sidebar 渲染 `SidebarGroup` 分组。
- `web-admin/src/locales/*.json` — 新增分组标签 i18n key。

角色可见性：

| 分组 | super_admin | tenant_admin | operator | building_admin |
|------|:-----------:|:------------:|:--------:|:--------------:|
| Command | ✓ | ✓ | ✓ | ✓ |
| Sites | ✓ | ✓ | Gateways 只读 | ✓ |
| People | ✓ | ✓ | Credentials 只读 | — |
| Platform | ✓ | — | — | — |

落地状态：

- `web-admin/src/App.tsx` 已将 `NavItem` 增加 `group` 字段，并按 `Command / Sites / People / Platform` 渲染多个 `SidebarGroup`。
- `Platform` 分组已挂载已有 `AuditPage`，并通过 `platform.audit.view` / `super_admin` 守卫访问。
- `web-admin/src/locales/*.json` 已新增导航分组、Audit 导航和范围标识文案。
- `role-boundary-e2e.spec.ts` 已覆盖 `super_admin` 分组导航、平台范围标识和 `/audit` 入口。

#### 2.2 租户/楼宇范围标识 ✅ 已完成（范围标识）

- `super_admin`：顶栏显示跨租户选择器（当前已有 tenant selector 逻辑）。
- `tenant_admin`：顶栏显示当前租户名称（静态标签，不可切换）。
- `building_admin`：顶栏显示”当前楼宇范围”标签，列出 `building_ids` 对应的楼宇名。
- `operator`：顶栏显示角色标签”运营视图”。

涉及文件：
- `web-admin/src/App.tsx` — header 区域新增 `RoleScopeBanner` 组件。
- 新建 `web-admin/src/components/role-scope-banner.tsx`。

落地状态：

- 已新增 `web-admin/src/components/role-scope-banner.tsx`。
- `super_admin` 顶栏显示平台跨租户范围标识；全局可切换 tenant selector 暂未接入，因为当前各业务页仍使用页面内局部 tenant 状态，后续应随跨页面 tenant context 一起处理。
- `tenant_admin` 顶栏显示当前 `tenant_id` 静态标签。
- `building_admin` 顶栏根据 `building_ids` 拉取楼宇列表并显示楼宇名，查不到名称时回退显示 ID；空范围显示“未分配楼宇”。
- `operator` 顶栏显示“运营视图”范围标签。

#### 2.3 Enterprise 页面角色化入口 ✅ 已完成

- `tenant_admin` 进入 `/enterprise` 时，页面标题显示”目录与登录”，默认展示 Directory + SSO tab。
- `super_admin` 进入 `/enterprise` 时，页面标题显示”企业健康”，默认展示跨租户同步健康总览。
- 隐藏当前角色无权操作的 tab（如 `operator` 不显示 HRIS Secret、IdP Config）。

涉及文件：
- `web-admin/src/features/enterprise/pages/enterprise-page.tsx` — 根据 `viewer.role` 过滤可见 tab。
- `web-admin/src/components/enterprise/enterprise-page-header.tsx` — 角色化标题。

落地状态：

- `EnterprisePageHeader` 已按角色切换标题：`super_admin` 显示“企业健康”，`tenant_admin` 显示“目录与登录”。
- `/enterprise` 无 hash 进入时，`super_admin` 默认落到 `sync` 区块，`tenant_admin` 默认落到 `employees` 区块；已有 `#idp/#sync/#alerts/#employees` 深链继续生效。
- `enterprise-page.tsx` 已将可见 section 改为集中数组渲染，`super_admin` 顺序为 `sync / alerts / employees / idp`，`tenant_admin` 顺序为 `employees / idp / sync / alerts`，为后续继续隐藏无权 tab 保留单点入口。
- `role-boundary-e2e.spec.ts` 已覆盖平台管理员和租户管理员的 `/enterprise` 默认入口与标题。
- `enterprise-idp-outcome-e2e.spec.ts` 已稳定登录语言和 IDP outcome 动作定位，保留 `#idp -> #alerts` 深链回归。

#### 2.4 resident 拦截页 ✅ 已完成

- `resident` 角色登录后，不进入 AppShell，显示无权限说明页 + 引导到移动端。

涉及文件：
- `web-admin/src/App.tsx` — 在 `AppShell` 渲染前检查 `viewer.role === “resident”`。
- 新建 `web-admin/src/pages/no-permission-page.tsx`。

落地状态：

- 已新增 `web-admin/src/pages/no-permission-page.tsx`。
- `App.tsx` 在 `viewer.role === "resident"` 时直接渲染无权限页，不进入 `AppShell`。
- `role-boundary-e2e.spec.ts` 已覆盖 resident 登录后停留在无权限页，且不出现管理后台导航入口。

---

### 阶段 3：事件、告警和 Enterprise 降复杂

#### 3.1 Events 页面重构

当前状态：✅ 已完成。Events 已调整为默认最近 24 小时窗口、精简 KPI、主筛选 + 折叠高级筛选、行点击详情抽屉。
目标：时间线检索 + 详情抽屉。

改造内容：

| 子项 | 说明 | 涉及文件 |
|------|------|----------|
| 3.1.1 ✅ | 顶部 KPI 精简为 4 个：访问量、拒绝量、设备异常、离线事件 | `events-page.tsx` |
| 3.1.2 ✅ | 默认时间范围改为”最近 24 小时” | `events-page.tsx` |
| 3.1.3 ✅ | 高级筛选折叠，默认只露出时间 + 类型 + 关键字 | `events-page.tsx` |
| 3.1.4 ✅ | 表格行点击打开 `DetailDrawer`（Sheet 侧滑），展示完整事件 JSON | 新建 `web-admin/src/components/events/event-detail-drawer.tsx` |
| 3.1.5 ✅ | `building_admin` 默认固定楼宇范围，隐藏楼宇筛选 | `events-page.tsx` |
| 3.1.6 ✅ | `super_admin` 显示租户列，其他角色隐藏 | `events-page.tsx` |

落地状态：

- 新增 `web-admin/src/components/events/event-detail-drawer.tsx`，在事件行点击后展示事件元数据和完整原始 JSON。
- `events-page.tsx` 已增加默认最近 24 小时时间筛选、主筛选区（时间 / 类型 / 关键字）和平台管理员专属高级租户筛选。
- 楼宇管理员仍由账号 `building_ids` 固定数据范围，不提供楼宇选择器；平台管理员保留租户列，其他角色隐藏租户列。
- `zh-CN / en-US / id-ID` 已补齐 Events 新 KPI、筛选、详情抽屉文案。
- `f4-role-surface-e2e.spec.ts` 已覆盖默认时间范围、事件类型筛选、重置和详情抽屉原始 JSON。

#### 3.2 Alarms 页面重构

当前状态：✅ 已完成。Alarms 已调整为默认状态分组队列、卡片下一步动作和通知配置二级 Tab。
目标：处置队列 + 状态分组。

改造内容：

| 子项 | 说明 | 涉及文件 |
|------|------|----------|
| 3.2.1 ✅ | 默认按状态分组展示：Open → Investigating → Mitigated → Closed | `alarms-page.tsx` |
| 3.2.2 ✅ | 每条告警卡片显示下一步动作按钮（确认/调查/缓解/关闭） | `alarms-page.tsx` |
| 3.2.3 ✅ | 通知策略和通知日志收进二级 tab，不和主告警列表同屏 | `alarms-page.tsx` |
| 3.2.4 ✅ | `building_admin` 只显示本楼宇告警 | `alarms-page.tsx` |

落地状态：

- `web-admin/src/features/alarms/pages/alarms-page.tsx` 默认展示 `Open / Investigating / Mitigated / Closed` 四段队列，告警以卡片呈现。
- 告警卡片按当前状态提供下一步主动作：确认、调查、缓解、关闭；完成态卡片显示已完成。
- 通知策略与通知日志已移动到 `Notifications` 二级 Tab，默认队列视图不再与通知配置同屏。
- `building_admin` 继续通过账号 `building_ids` 固定过滤本楼宇告警，未分配楼宇范围时不展示记录。
- `zh-CN / en-US / id-ID` 已补齐 Alarms 新增 Tab、队列分组、卡片字段和下一步动作文案。
- `f4-role-surface-e2e.spec.ts` 已覆盖分组标题、下一步确认动作、通知日志 Tab、失败时通知日志保持为空，以及既有筛选链路。

#### 3.3 Enterprise Alerts 拆分

当前状态：✅ 首轮已完成。已拆出 5 个子工作区组件边界，`enterprise-alerts-workspace.tsx` 仍保留共享筛选、状态和动作编排，后续再逐步下沉业务逻辑与详情抽屉。
目标：拆为独立子工作区组件。

| 子工作区 | 新组件文件 | 内容 |
|----------|-----------|------|
| JIT 审批收件箱 ✅ | `enterprise-jit-approval-inbox.tsx` | 待审批列表 + 审批/拒绝动作 |
| 同步异常 ✅ | `enterprise-sync-exceptions.tsx` | 失败同步任务 + 重试/跳过 |
| Worker 告警 ✅ | `enterprise-worker-alerts.tsx` | 阈值告警 + 订阅配置 |
| HRIS 回执 ✅ | `enterprise-hris-receipts.tsx` | Webhook 投递回执 + 状态 |
| HRIS DLQ ✅ | `enterprise-hris-dlq.tsx` | DLQ 条目 + 回放/清理 |

每个子工作区使用统一模式：”摘要卡 + 处理队列 + 详情抽屉”。

落地状态：

- 已新增 5 个组件文件，先承担工作区标题、说明、操作区、边界样式和稳定 `data-testid`。
- `enterprise-alerts-workspace.tsx` 已将 JIT 审批、同步异常、Worker 告警、HRIS Receipts、HRIS DLQ 包入对应子工作区组件。
- JIT 审批区恢复带计数的批量按钮、状态筛选和批量结果文案，便于运营判断处理范围。
- `enterprise-sync-worker-alerts-e2e.spec.ts` 已覆盖 Sync Worker 到 Alerts 的子工作区边界可见性。
- `enterprise-idp-outcome-e2e.spec.ts` 已覆盖 JIT 审批边界和批量审批/外部回写 11 条回归。

#### 3.4 Wallet 高级运营折叠

当前状态：✅ 已完成。Wallet 日常操作流已保持首屏直达，高级运营区默认折叠。
目标：主路径（模板 + 发放 + 投递）保持首屏，高级运营（队列/趋势/DLQ/告警订阅）折叠到 “Advanced Operations” 可展开区。

涉及文件：
- `web-admin/src/features/wallet/pages/wallet-page.tsx` — 移除 Wallet 二级 Tab，保留日常操作流，实体卡任务迁入高级运营折叠区。
- `web-admin/src/components/wallet/wallet-advanced-workspace.tsx` — 改为默认关闭的折叠工作区，展示实体卡任务、告警订阅、风险概览、趋势、通知记录和 DLQ 归档。
- `web-admin/src/locales/*.json` — 补齐 Advanced Operations 折叠、摘要和 Wallet 回执恢复动作文案。
- `web-admin/e2e/role-boundary-e2e.spec.ts` — 覆盖 Wallet 高级运营默认折叠和展开行为。

落地状态：

- `/wallet` 不再显示 `Operations / Advanced` Tab；模板、单笔/批量发放、投递、回执恢复和已发放凭证台账在一个日常操作流中连续展示。
- 实体卡任务、告警订阅、队列风险概览、趋势、通知记录、DLQ 清理归档已收进 `Advanced Operations` 折叠区，默认关闭。
- 折叠区顶部提供摘要徽标：实体卡任务、告警、通知、DLQ 归档和订阅状态，用户不展开也能判断是否需要处理。
- 恢复 Wallet 投递回执文案：批量重发失败通道、批量状态修复、单条重发失败通道，以及成功/失败结果摘要。

---

### 阶段 4：视觉系统整改（结合 shadcn/ui + DESIGN.md）

#### 4.1 设计方向定调

采用混合体系，融合两个设计源：

| 区域 | 风格来源 | 表现 |
|------|----------|------|
| App Shell（Sidebar + Header） | MistyIslet 雾岛指挥中心 | 深色背景、银色高光、雾气纹理、command rail |
| Dashboard Hero | MistyIslet | 站点星图、心跳线、KPI 发光卡片 |
| 登录页 | MistyIslet | 全屏深色雾气 + 居中玻璃卡片 |
| 任务卡片（表格、表单、配置） | DESIGN.md Cohere 风格 | 白/雪灰背景、22px 圆角、冷灰边框、极克制色彩 |
| 状态标签 | 语义色 | 绿=正常、黄=警告、红=异常、蓝=信息 |
| 交互态 | DESIGN.md | Interaction Blue `#1863dc` 仅用于 hover/focus |

#### 4.2 CSS Token 体系统一

当前状态：✅ 已完成。`tailwind.config.ts` 已移除 `hsl(var(--*))` 旧格式，统一改为 CSS variable 直接引用；`index.css` 已补齐 DESIGN.md 任务卡与交互态 token，并同步暴露给 Tailwind v4 `@theme inline`。

| 子项 | 说明 |
|------|------|
| 4.2.1 ✅ | 统一 `tailwind.config.ts` 颜色引用为 CSS variable 直接引用，移除 `hsl()` 包裹 |
| 4.2.2 ✅ | 新增 `--radius-card: 22px` 对应 DESIGN.md 签名圆角 |
| 4.2.3 ✅ | 新增 `--color-interaction: #1863dc` 用于 hover/focus 统一 |
| 4.2.4 ✅ | 新增任务卡片变体 token：`--card-task: #ffffff`（亮色任务卡背景）、`--card-task-border: #e5e7eb` |

涉及文件：
- `web-admin/src/index.css`
- `web-admin/tailwind.config.ts`

落地状态：

- `tailwind.config.ts` 的 `border/input/ring/background/foreground`、语义色和 `card` 色值已全部改为 `var(--*)` 直接引用。
- `tailwind.config.ts` 已新增 `interaction`、`card-task`、`card-task-border` 颜色映射，以及 `rounded-card` 对应的 `var(--radius-card)`。
- `index.css` 已新增 `--radius-card: 22px`、`--color-interaction: #1863dc`、`--card-task: #ffffff`、`--card-task-border: #e5e7eb`，并补齐 `--destructive-foreground`。
- Tailwind v4 `@theme inline` 已暴露 `--color-interaction`、`--color-card-task`、`--color-card-task-border` 和 `--radius-card`，供后续阶段 4.3 组件变体直接使用。

#### 4.3 shadcn/ui 组件定制

当前状态：✅ 已完成。Card/Button/Table/Badge 已具备 DESIGN.md 变体 API；Dialog/Sheet/Tabs/Skeleton 已完成亮色任务面、Interaction Blue line tab 和任务骨架屏收口。

基于当前已安装的 22 个 shadcn/ui 组件，按 DESIGN.md 规范定制：

| 组件 | 当前状态 | 定制方向 |
|------|----------|----------|
| `Card` | ✅ 已新增 `variant="task"` | 亮色任务卡变体：白色背景、`border-cool` 边框、22px 圆角、无阴影 |
| `Button` | ✅ 已新增 `variant="interaction"` | 透明底 → hover 时 Interaction Blue 文字 |
| `Table` | ✅ 已新增 `variant="task"` | 亮色表格：白色行、冷灰分隔线、hover 行高亮 `#fafafa` |
| `Dialog` | ✅ 已改为亮色弹窗 | 圆角改为 `8px`（DESIGN.md dialog 规范），亮色背景 |
| `Sheet` | ✅ 已改为亮色侧滑面板 | 作为 DetailDrawer 的基础，亮色背景 + 22px 顶部圆角 |
| `Badge` | ✅ 已新增语义状态变体 | `success`（绿）、`warning`（黄）、`danger`（红） |
| `Tabs` | ✅ line 变体已使用 Interaction Blue | line 变体下划线改为 Interaction Blue |
| `Skeleton` | ✅ 已新增 `variant="task"` | 保持当前深色适配，任务卡内使用亮色骨架 |

涉及文件：
- `web-admin/src/components/ui/card.tsx` — 新增 `variant` prop
- `web-admin/src/components/ui/button.tsx` — 新增 interaction 变体
- `web-admin/src/components/ui/table.tsx` — 新增 task 变体
- `web-admin/src/components/ui/badge.tsx` — 新增语义变体
- `web-admin/src/components/ui/dialog.tsx` — 改为亮色 `8px` 弹窗
- `web-admin/src/components/ui/sheet.tsx` — 改为亮色侧滑面板与 22px 边缘圆角
- `web-admin/src/components/ui/tabs.tsx` — line 变体 active underline 使用 Interaction Blue
- `web-admin/src/components/ui/skeleton.tsx` — 新增 task 亮色骨架屏变体
- `web-admin/src/components/ui/ui-variants.test.ts` — 覆盖新增变体 token 类名

落地状态：

- `Card` 改为 `cva` 管理样式，默认深色玻璃外观保持不变；`variant="task"` 使用 `bg-card-task`、`border-card-task-border`、`rounded-card`、无阴影和亮色文字。
- `CardHeader / CardTitle / CardDescription / CardFooter` 已通过 `group-data-[variant=task]` 继承任务卡语义，避免调用方重复覆盖子组件样式。
- `Button` 新增 `interaction` 变体，透明底，hover/expanded/focus 使用 `text-interaction`、`border-interaction` 和 `ring-interaction`。
- `Table` 新增 `variant="task"`，容器使用任务卡边框和 22px 圆角，行、表头、单元格通过 table group 继承亮色表格样式。
- `Badge` 新增 `success / warning / danger` 语义变体，为后续状态标签从 outline/secondary/destructive 迁移到明确语义色做准备。
- `DialogContent` 默认使用亮色任务面、冷灰边框、`rounded-lg` 8px 圆角和无阴影；标题、描述、Footer 与关闭按钮已适配亮色面。
- `SheetContent` 默认使用亮色任务面，按侧边方向提供 22px 边缘圆角和冷灰边框，作为后续 `DetailDrawer` 基础。
- `TabsTrigger` 的 line 变体 active 文本和下划线改为 `interaction` token。
- `Skeleton` 新增 `variant="task"`，任务卡内可使用 `#f2f2f2` 亮色骨架屏。

#### 4.4 通用业务组件库

当前状态：✅ 已完成首轮组件库。`RoleScopeBanner` 已沿用现有实现，其余通用组件已补齐，并将 Events 详情抽屉接入 `DetailDrawer` 做低风险验证。

新建以下通用组件，供所有页面复用：

| 组件 | 用途 | 文件 |
|------|------|------|
| `RoleScopeBanner` ✅ | 顶栏角色范围标识 | `components/role-scope-banner.tsx` |
| `ScopeLockedField` ✅ | 角色固定的筛选字段（静态标签替代 select） | `components/scope-locked-field.tsx` |
| `OperationalKPI` ✅ | 统一 KPI 指标卡（数值 + 趋势 + 标签） | `components/operational-kpi.tsx` |
| `DetailDrawer` ✅ | 统一详情侧滑面板（基于 Sheet） | `components/detail-drawer.tsx` |
| `ActionInbox` ✅ | 统一处置队列（摘要卡 + 列表 + 动作按钮） | `components/action-inbox.tsx` |
| `EmptyState` ✅ | 统一空状态（品牌化插图 + 下一步引导） | `components/empty-state.tsx` |
| `PermissionBoundary` ✅ | 权限不足说明（替代静默隐藏） | `components/permission-boundary.tsx` |

落地状态：

- `ScopeLockedField` 提供亮色任务面静态范围字段，用于后续替换角色固定的 select。
- `OperationalKPI` 基于 `Card variant="task"`，支持数值、说明、图标和趋势 Badge。
- `DetailDrawer` 封装 `Sheet`、标题、说明、正文和 Footer；`EventDetailDrawer` 已接入该通用抽屉并保留原事件详情视觉类名。
- `ActionInbox` 提供统一处置队列骨架：标题、说明、摘要、列表项、状态 Badge、动作按钮和空状态。
- `EmptyState` 提供统一空态面板，内置 MistyPass 品牌标识并支持自定义说明和动作。
- `PermissionBoundary` 用 `EmptyState` 替代静默隐藏，用于后续权限不足说明。
- 新增 `business-components.test.ts`，覆盖通用组件的任务面、抽屉接线和权限 fallback。

#### 4.5 溢出与响应式修复

当前状态：✅ 高优先级首轮已完成。已优先收口 Events、Alarms、Gateways、Spaces 中最容易在平板/窄屏挤压的筛选区、命令区、角色固定范围字段和只读禁用态；Access 已完成租户范围锁定、授权/策略表单和授权筛选响应式扫尾；Wallet 已完成发放模板、告警订阅、投递表单和已发凭证筛选的低频响应式与禁用态说明扫尾；Enterprise 的低频区域后续随页面密度与文案阶段继续扫尾。

逐页扫描并修复：

| 问题类型 | 修复规则 | 优先页面 | 当前状态 |
|----------|----------|----------|----------|
| 固定宽度承载翻译文本 | 改为 `minmax(0, 1fr)` 或 flex wrap | Spaces, Gateways, Enterprise | ✅ Spaces/Gateways 首轮完成；Access 授权筛选与表单完成；Wallet 发放工作台首轮完成；Enterprise 待扫尾 |
| 表格截断关键状态 | 改为 tooltip 或 DetailDrawer | Events, Alarms, Wallet | ✅ Events 接入详情抽屉，Alarms 主表格已卡片化；Wallet 已发凭证筛选与动作区首轮完成，台账详情仍可后续增强 |
| 单选项 Select | 改为静态标签 | Spaces（楼宇联动）, Access | ✅ Spaces/Access 完成 |
| 禁用无解释 | 添加 tooltip 说明原因 | Gateways, Wallet, Enterprise | ✅ Gateways 完成；Access 表单范围 Select 完成；Wallet 模板/订阅/投递首轮完成；Enterprise 待扫尾 |
| 筛选区窄屏溢出 | 改为单列 stack | Events, Gateways | ✅ Events/Gateways/Alarms 首轮完成；Wallet 已发凭证筛选完成 |

#### 4.6 页面密度规则

当前状态：✅ Gateways 首轮已完成。已将网关注册这类低频配置任务从首屏运营面折叠，网关重启进入确认弹窗，网关台账默认列收敛到 6 列以内；Wallet 高级运营已在阶段 3.4 折叠，Events 详情和 Alarms 队列也已在前序阶段完成主路径减密。

- 首屏只放”当前角色今天最该看的 5 件事”。
- 配置表单不和运营队列同屏竞争。
- 高危操作进入二级抽屉或确认弹窗。
- 表格默认列不超过 6 列，其他信息进入 DetailDrawer。
- 移动端优先卡片化，避免宽表横向溢出。

---

### 阶段 5：文案和文档收口

当前状态：✅ Access 核心工作台、Wallet 工作台、Enterprise 前端文案收口、`en-US` 精确占位替换与 Enterprise Sync/Alerts + enterprisePage + walletPage 弱占位第一批已完成，阶段 5.4 权限不可见和禁用态提示首轮已完成，阶段 5.5/5.6 角色化表达与操作结果提示首轮已完成。已将 Access、Wallet 与 Enterprise 相关展示 `defaultValue` 收口到三套 locale，并清理 `en-US.json` 中精确 `title / description / empty / source value` 占位；`enterpriseSyncWorkspace / enterpriseAlertsWorkspace / enterprisePage / walletPage / accessPage` 的 `action / helper / row / fallback / description empty / label / metrics / stats / loading / summary` 等弱占位已完成首轮收口；Access 临时授权表单错误、占位符和范围禁用原因已收口到三套 locale；Wallet 告警订阅阈值、窗口、冷却时间与模板 style_config 占位符也已收口到三套 locale。下一批建议继续 Enterprise 低频响应式扫尾；若继续文案扫尾，优先处理低频表单错误和非主路径状态词。

| 子项 | 说明 | 状态 |
|------|------|------|
| 5.1 | 替换 `en-US.json` 剩余占位值（`description`、`title`、`empty` 等） | ✅ 精确占位扫描归零；Enterprise Sync/Alerts + enterprisePage + walletPage 弱占位清零 |
| 5.2 | 替换 `zh-CN.json` 剩余占位值 | ✅ 首批已完成；第二十九批补齐 Enterprise/Wallet/Access 主路径弱占位 |
| 5.3 | 把 TSX 中仍承担实际展示的 `defaultValue` 收口到 locale 文件 | ✅ Access + Wallet + Enterprise 完成 |
| 5.4 | 为权限不可见和禁用态补充明确提示文案 | ✅ 首轮完成（Wallet + Gateways + Enterprise HRIS + Enterprise Alerts） |
| 5.5 | 按角色说话：删除平台内部术语，用客户能理解的语言 | ✅ 首轮完成（Enterprise Sync/Alerts、enterprisePage、Wallet、Access summary） |
| 5.6 | 危险操作写结果（如”将重启网关并产生短暂离线”），不只写”执行” | ✅ 首轮完成（Access/Wallet 批量动作、投递恢复、策略状态与发放结果提示） |

---

### 阶段 6：验证

当前状态：✅ 首轮完成。已完成阶段 5.5/5.6 后的主路径验证收口，修复 Enterprise Alerts / Sync Worker / Wallet Enterprise 回流 E2E 中的订阅卡片兼容、批量草稿同步、JIT 审批状态筛选和深链定位回归；核心检查、E2E 分片回归与完整 E2E 均通过。

必须跑：

- `npm run typecheck`
- `npm run build`
- `npm run check:cjk`
- `npm run test:unit`

建议跑：

- `npm run test:e2e -- role-boundary`
- 浏览器冒烟 `/login`、`/dashboard`、`/events`、`/alarms`、`/spaces`、`/gateways`、`/wallet`、`/enterprise`。
- 每个角色登录一次，验证导航分组、范围标识、可见 tab 符合预期。

---

### 实施优先级排序

```
阶段 2.1 Sidebar 导航分组 ✅
    ↓
阶段 2.2 租户/楼宇范围标识 + 2.4 resident 拦截 ✅
    ↓（可并行）
阶段 2.3 Enterprise 角色化入口 ✅
阶段 3.1 Events 时间线重构 ✅
阶段 3.2 Alarms 处置队列重构 ✅
    ↓（可并行）
阶段 3.3 Enterprise Alerts 拆分 ✅（首轮组件边界）
阶段 3.4 Wallet 高级运营折叠 ✅
    ↓
阶段 4.2 CSS Token 统一 ✅
阶段 4.3 shadcn/ui 组件定制 ✅（Card/Button/Table/Badge 变体首轮）
阶段 4.3 shadcn/ui 组件收口 ✅（Dialog/Sheet/Tabs/Skeleton）
    ↓
阶段 4.4 通用业务组件库 ✅
阶段 4.5 溢出与响应式修复 ✅（高优先级首轮）
阶段 4.5 Access 低频响应式与范围锁定扫尾 ✅
阶段 4.5 Wallet 低频响应式与禁用态扫尾 ✅
    ↓
阶段 4.6 页面密度规则 ✅（Gateways 首轮）
    ↓
阶段 5 文案收口 ✅（Access + Wallet + Enterprise 完成；en-US 精确占位归零；Enterprise Sync/Alerts + enterprisePage + walletPage + accessPage 弱占位首轮清零）
    ↓
阶段 5.4 权限和禁用态提示 ✅（Wallet + Gateways + Enterprise HRIS + Enterprise Alerts 首轮完成）
    ↓
阶段 5.5 / 5.6 角色化表达与危险操作结果提示 ✅（主路径首轮完成）
    ↓
阶段 6 验证 ✅（首轮完成）
```

## 11. 首批建议更新项

> 已被阶段 10 的详细实施计划取代。阶段 2、阶段 3.1、阶段 3.2、阶段 3.3 首轮组件边界、阶段 3.4、阶段 4.2、阶段 4.3、阶段 4.4、阶段 4.5 高优先级首轮、阶段 4.6 Gateways 首轮、阶段 5 Access/Wallet/Enterprise 前端文案收口、`en-US` 精确占位替换与 Enterprise Sync/Alerts + enterprisePage + walletPage + accessPage 弱占位第一批已完成，阶段 5.4 权限不可见和禁用态提示首轮已完成，阶段 5.5/5.6 角色化表达与危险操作结果提示首轮已完成，阶段 6 主路径验证首轮已完成。

## 12. 需要业务确认的点

- `tenant_admin` 是否就是“办公室/企业管理员”，还是还存在“物业公司管理多个办公室”的客户类型。
- `building_admin` 是否需要管理员工权限，还是只做楼宇资产和设备运维。
- `operator` 是否允许重试 Wallet 投递和队列任务，还是只读。
- 普通 `resident` 登录 web-admin 时，是直接拒绝、跳转自助端，还是显示无权限说明页。
- 是否需要新增“客户类型/账号类型”字段来区分办公楼物业、入驻企业、园区、学校、医院；如果需要，应作为后续后端最小增量，不影响第一阶段前端重组。

## 13. 本轮推进更新项

2026-04-25 第三十二批阶段 4.5 / 5 Wallet 低频响应式、禁用态与本地联调已完成：

- 已启动本地后端与前端：API 运行在 `http://127.0.0.1:8080`，Web Admin 运行在 `http://127.0.0.1:5173/`，`/healthz` 返回 `ok`。
- `WalletIssuedPassesCard` 的已发凭证筛选区改为移动端双列/单列优先，Select 触发器统一 `w-full min-w-0`，固定列轨道改为 `minmax(0, ...)`。
- `WalletTemplateManagerCard` 补齐模板表单只读/忙碌/加载禁用原因，模板列表动作在窄屏全宽换行；`style_config` 占位符收口到三套 locale。
- `WalletAlertSubscriptionCard` 补齐订阅开关、阈值/窗口/冷却时间输入和保存/立即通知按钮的禁用原因；告警阈值、窗口、冷却时间占位符收口到三套 locale。
- `WalletSendDeliveryCard` 的凭证 Select 补齐响应式宽度，所选凭证目标 ID 增加换行约束，避免长目标 ID 撑开投递卡片。
- Wallet 页面发放模板/队列、投递/回执两组双栏布局改为 `minmax(0, ...)`，降低窄屏和长翻译文本横向溢出风险。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `git diff --check`
- locale JSON 解析检查
- `npm run test:unit`（25 passed）
- `npm run build`（通过，保留既有 500k chunk warning）
- `npm run test:e2e -- role-boundary enterprise-idp-outcome`（92 passed）

2026-04-25 第三十一批阶段 4.5 / 5 Access 低频响应式与文案扫尾已完成：

- `AccessPageHeader` 的非平台租户范围从简单 Badge 改为 `ScopeLockedField`，明确当前账号已按角色锁定到单一租户；平台视角租户选择器改为 `w-full min-w-0`，避免长租户名挤压。
- `AccessGrantFilterBar`、`AccessGrantForm`、`AccessPolicyForm` 的 Select 触发器统一补齐 `w-full min-w-0`，授权筛选 grid 改为 `minmax(0, ...)` 轨道，清除按钮在窄屏全宽显示。
- Access 目录/策略/授权三块双栏布局改为 `minmax(0, 0.9fr) / minmax(0, 1.1fr)`，授权/策略表单内部两列字段改为 `sm:grid-cols-2`，移动端优先单列。
- 授权/策略范围 Select 的禁用态增加 title 说明，解释“全部区域 / 楼宇级 / 未选楼宇或区域”导致下级选择器不可用的原因。
- `AccessGrantForm` 将低频校验错误、性别/电话/邮箱/手机型号/对象类型占位符、禁用态说明收口到 `en-US / zh-CN / id-ID`，避免临时授权表单继续显示硬编码英文。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `git diff --check`
- `npm run test:unit`（25 passed）
- `npm run test:e2e -- access-route f4-role-surface`（15 passed）
- `npm run build`（通过，保留既有 500k chunk warning）

2026-04-25 第三十批阶段 6 E2E 验证与回流链路收口已完成：

- 修复 `EnterpriseSyncWorkerAlertSubscriptionCard` 对旧 mock / 旧接口形态的兼容：`channels` 缺失或数组形态时会归一化为默认通道配置，避免 Enterprise Alerts 页面进入 ErrorBoundary。
- 修复 Wallet 批量发放草稿受控同步：Enterprise `target_ids` 回流预填、仅保留缺失目标、恢复全部预填目标会同步到批量发放 textarea 和表单状态。
- 固定 Enterprise JIT Approval Inbox 的 pending / approved / rejected 状态筛选展示，计数归零后仍可看到对应 0 状态，避免操作后筛选入口消失。
- 为 Enterprise Sync 主流程分段和 Worker 告警钱包跳转补充稳定 `data-testid`，E2E 不再依赖同名“前往钱包 / 复核目录”链接顺序。
- 更新 Enterprise / Wallet 回流 E2E 断言到当前 locale 文案：回执恢复、Worker 复核、批量补发草稿、仅保留缺失目标、恢复全部预填目标、JIT 批量审批/拒绝等链路已收口。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 500k chunk warning）
- `git diff --check`
- `npm run test:e2e -- role-boundary`（10 passed）
- `npm run test:e2e -- enterprise-sync-worker-alerts enterprise-idp-outcome`（105 passed）
- `npm run test:e2e -- access-route f4-role-surface enterprise-idp-outcome enterprise-sync-worker-alerts`（120 passed）
- `npm run test:e2e -- access-enterprise-flow enterprise-hris-connector enterprise-hris-connector-hybrid enterprise-hris-connector-inline-secret-smoke`（14 passed）
- `npm run test:e2e`（144 passed）

2026-04-25 第二十九批阶段 5.5/5.6 角色化表达与操作结果提示首轮补强已完成：

- `enterpriseSyncWorkspace` 三套语言补齐同步结果、流程就绪度、主流程分段、Worker 告警和 HRIS 主路径说明，去掉“说明 / Deskripsi / step title”式占位表达。
- `enterpriseAlertsWorkspace` 三套语言补齐审批闭环、目录闭环、落地动作、回执恢复和恢复动作说明，明确哪些事项会继续阻塞目录、策略、钱包闭环。
- `enterprisePage` 三套语言补齐告警恢复、关注项、同步结果、IDP 结果、推荐下一步、同步来源状态与 source feedback 文案，用客户任务语言替代内部占位。
- `walletPage` 三套语言补齐投递回执恢复三步说明、批量重试/状态修复/补发草稿结果提示、只读提示和 QR 对话框文案。
- `accessPage` 三套语言补齐 Enterprise 回流 summary、策略批量启用结果、用户组/策略预填充、授权创建和环节尾注文案，操作反馈从“动作已执行”改为说明结果和下一步。
- 已重新扫描本批目标占位，剩余命中为表单字段名、aria label 或“允许为空”类真实文案。
- 下一批建议进入阶段 6 验证与浏览器冒烟；如继续阶段 5 扫尾，优先处理低频表单错误和非主路径状态词。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（仍存在既有 500k chunk warning）
- `git diff --check`

2026-04-25 第二十八批阶段 5.4 Enterprise Alerts 批量/通知/队列禁用态原因首轮补强已完成：

- Approval closure 与 JIT Approval Inbox 已为批量审批、批量拒绝、批量标记外部回写完成增加禁用原因，覆盖只读、操作执行中、当前视图无 pending 审批、无失败/待确认外部回写记录和动作不可用。
- JIT approval 单条审批/拒绝/标记回写按钮已补充只读与执行中 `title` 原因。
- Worker alert notification 历史已为导出、自动重试到期项、批量静默、批量恢复、批量重试和单条重试/静默/恢复增加禁用原因，覆盖加载中、导出中、只读、已有通知动作执行中、无可处理通知。
- HRIS webhook receipt 批量处理和单条处理已补充只读、加载中、执行中、无 ready receipt 的禁用原因。
- HRIS DLQ 批量重放和单条重放已补充只读、加载中、执行中、无 ready DLQ 条目的禁用原因。
- `enterpriseAlertsWorkspace.disabledReasons` 已同步补齐 `en-US / zh-CN / id-ID`，阶段 5.4 权限不可见和禁用态提示首轮至此覆盖 Wallet、Gateways、Enterprise HRIS Connector 与 Enterprise Alerts 主路径。
- 下一批建议转入阶段 5.5/5.6，优先复核剩余全局短标签、角色化表达和危险操作结果提示。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（仍存在既有 500k chunk warning）

2026-04-25 第二十七批阶段 5.4 Enterprise HRIS Connector 禁用态原因首轮补强已完成：

- Talenta connector 表单已为 connector status、sync strategy、credential mode、webhook secret mode 等选择器补充只读、加载中、保存中禁用原因。
- Credential ref / Webhook secret ref 下拉已补充无可复用 secret ref 的禁用原因，避免空库存状态被误判为组件异常。
- Client credential、pull 参数、webhook secret 输入已补充只读/加载/保存中 `title` 提示。
- Incremental override 开关与参数输入已补充 existing credential ref 锁定原因，明确这类字段需在 vault secret 内维护。
- 保存按钮已补充只读、加载中、保存中原因，并在表单底部显示可见提示。
- `enterpriseSyncWorkspace.hrisConnector.form` 已同步补齐 `en-US / zh-CN / id-ID` 的 HRIS connector 禁用态文案。
- 下一批建议继续阶段 5.4，处理 Enterprise Alerts 的审批批量动作、Worker notification 批量重试/静默/恢复、HRIS receipt/DLQ 队列按钮禁用原因。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（仍存在既有 500k chunk warning）

2026-04-25 第二十六批阶段 5.4 Gateways 禁用态原因首轮补强已完成：

- Gateway Command Center 已为绑定/解绑门点、发布配置、重启确认、挂载设备、探测旧设备序列号增加禁用原因，覆盖只读、命令执行中、未选择网关/门点、未输入配置版本、设备槽位不足和未输入设备序列号。
- 下挂设备表单已为设备类型、来源、在线状态、协议和 RS485 参数补充只读禁用提示，避免只读角色误判为系统异常。
- Gateway Setup 注册表单已为注册按钮补充执行中和未选择租户原因提示。
- Serial Inventory 入库、CSV 入库、导出和库存台账批量状态流转已增加禁用原因，覆盖执行中、未选择租户、未选择/粘贴序列号。
- 库存台账单条 Return / Freeze / Scrap 操作已增加状态原因提示，覆盖当前状态无需重复操作和已报废序列号不能直接冻结。
- `gateways.disabledReasons` 已同步补齐 `en-US / zh-CN / id-ID`，用于命令、注册、库存和危险状态流转的统一禁用原因。
- 下一批建议继续阶段 5.4，优先处理 Enterprise Alerts/HRIS 工作台的批量操作、审批/同步动作和只读边界提示。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（仍存在既有 500k chunk warning）

2026-04-25 第二十五批阶段 5.4 Wallet 禁用态原因首轮补强已完成：

- Issued passes 批量维护已为 Suspend / Activate / Revoke / Clear Selection 增加禁用原因 `title` 与按钮组可见提示，覆盖只读、忙碌和未选择凭证。
- Send delivery 已为 Email / WhatsApp 开关、收件人输入和发送按钮增加禁用原因，覆盖只读、通道未启用、投递中、加载中和未选择凭证。
- Delivery receipts recovery 已为批量重发、状态修复、补发草稿和单条重试增加禁用原因，覆盖只读、忙碌、加载中、无可重试投递、无可修复凭证和无补发目标。
- Issue queue 已为 Enterprise 预填目标筛选、恢复预填、单笔发放和批量发放增加禁用原因，覆盖只读、加载中、发放中、未选择模板、未输入目标和无 Enterprise 可操作目标。
- `walletPage.disabledReasons` 已同步补齐 `en-US / zh-CN / id-ID`，新增发放队列相关的模板、目标、Enterprise 预填和发放中原因文案。
- 下一批建议继续阶段 5.4，优先处理 Enterprise Alerts/HRIS 工作台与 Gateways 低频危险操作的只读边界、禁用原因和结果提示。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（仍存在既有 500k chunk warning）

2026-04-25 第二十四批阶段 5 en-US walletPage 弱占位文案替换已完成：

- `en-US.json` 中 `walletPage.actions` 的常用按钮、批量动作、企业回流入口和实体卡任务动作已从小写占位替换为可读英文。
- `walletPage.enterprise / hints / labels / status / validation / table / errors / scenarios / summaries` 已补齐企业回流摘要、只读边界提示、状态标签、校验错误和操作反馈文案，并保留 `{{count}} / {{targetID}} / {{status}} / {{flowLabel}} / {{stageLabel}}` 等插值。
- `walletPage.cards.statusMaintenance / sendDelivery / physicalTasks / issuedPasses` 的 stats、空态、加载态和操作提示已替换为可读英文。
- 严格弱占位复扫结果：`enterpriseSyncWorkspace / enterpriseAlertsWorkspace / enterprisePage / walletPage = 0`。全局启发式弱扫描从第二十三批后的 106 降到 86；剩余多为有效的 `Go to... / View...` 按钮短文案或后续权限提示阶段再复核的短标签。
- 下一批建议转入阶段 5.4，集中补强权限不可见、只读边界、禁用态原因与危险操作结果提示。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（仍存在既有 500k chunk warning）

2026-04-25 第二十三批阶段 5 en-US enterprisePage 弱占位文案替换已完成：

- `en-US.json` 中 `enterprisePage.actions` 的按钮动作、`alertRecovery` 的恢复说明、`cards.approvalBacklog / directoryException / syncOutcome` 的状态说明与返回提示已替换为可读英文。
- `enterprisePage` 的 errors、flowHints、kpi、labels、status、syncCheckpoints、syncResult、syncSourceStatus、syncSources、syncSummary、workflow helper/metric 等弱占位已替换，并保留 `{{action}} / {{blockerCount}} / {{sourceLabel}} / {{count}} / {{created}}` 等插值。
- 严格弱占位复扫结果：`enterprisePage = 0`。全局启发式弱扫描从第二十二批后的 127 降到 106；剩余主要集中在 `walletPage`，另有部分 `Go to... / View... / Worker alert...` 属于有效按钮或领域文案。
- 下一批建议继续处理 `walletPage` 的 `empty / stats / worker alert label / empty select pass / loading` 类弱文案，并同步推进阶段 5.4 权限不可见和禁用态提示。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（仍存在既有 500k chunk warning）

2026-04-25 第二十二批阶段 5 en-US Enterprise Sync/Alerts 弱占位文案第一批替换已完成：

- `en-US.json` 中 `enterpriseSyncWorkspace` 的 mainflow、outcome、pipeline、pipelineCheck、recentResults、remediation、sourceOverview、workerAlerts 与路由 hint 文案已从 `action / helper / row / fallback / metric / status` 类占位替换为可读英文，并保留 `{{count}} / {{source}} / {{rank}} / {{stage}}` 等插值。
- `enterpriseAlertsWorkspace` 的 landing actions、approval closure、directory closure、blockers、receiptRecovery、recoveryActions、JIT approval、Sync & Worker actions/filter 以及跨页面 hint 文案已替换为可读英文。
- 严格弱占位复扫结果：`enterpriseSyncWorkspace = 0`，`enterpriseAlertsWorkspace = 0`。全局启发式弱扫描从 189 降到 127；剩余主要集中在 `enterprisePage / walletPage`，另有部分 `Go to... / View...` 属于有效按钮文案。
- 下一批建议继续处理 `enterprisePage` 的 `description pending / pending count / helper pending / label / metrics / no result` 与 `walletPage` 的 `empty / stats / worker alert label` 类弱文案，并同步推进阶段 5.4 权限不可见和禁用态提示。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（仍存在既有 500k chunk warning）

2026-04-25 第二十一批阶段 5 en-US enterprisePage / Wallet 精确占位文案替换已完成：

- `en-US.json` 中 `enterprisePage` 的 attention、cards、idpOutcome、nextAction、quickActions、remediation、syncOutcome、syncSourceFeedback、syncSourceStatus、syncSources 等剩余 `title / description` 精确占位已替换为可读英文。
- Wallet 剩余 4 个精确 `empty` 占位已替换：最近投递、投递回执恢复、最近实体卡任务、保存链接二维码空态。
- 精确占位扫描已从第二十批后的 97 项降到 0 项，覆盖规则为值完全等于 `title / description / empty / source value` 以及 `card description / hint` 的占位字符串。
- 下一批建议继续处理更宽泛的弱文案，例如 `action go`, `helper empty`, `label`, `description pending`, `row summary` 等非精确占位，以及阶段 5.4 的权限不可见和禁用态提示。

本批验证已通过：

- 精确占位扫描：0
- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）

2026-04-25 第二十批阶段 5 en-US Enterprise Sync/Alerts 占位文案替换已完成：

- `en-US.json` 中 `enterpriseSyncWorkspace` 的主流程优先级、pipeline 标题、pipeline readiness、最近同步结果空态、source overview 和 worker alert 空态不再显示 `title / description / empty` 这类占位词。
- `en-US.json` 中 `enterpriseAlertsWorkspace` 的审批闭环、目录闭环、JIT 审批、落地动作、回执恢复、恢复动作，以及 Sync & Worker 子区块的标题、说明、空态和来源值文案已替换为可读英文。
- 精确占位扫描从 146 项降到 97 项；剩余主要集中在 `enterprisePage.*` 和 Wallet 少量空态，下一批继续处理。

本批验证已通过：

- locale JSON 解析检查（`en-US / zh-CN / id-ID`）
- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）

2026-04-25 第十九批阶段 5 Enterprise Alerts Receipt / DLQ 文案扫尾已完成：

- `EnterpriseAlertsWorkspace` 的 Webhook Receipt 与 DLQ 子块已完成展示文案收口：批量处理/重放按钮、批量提示、分页摘要、最新批次标题与摘要、dispatch 文案、批次条目 meta、Open execution、Process、Load more 不再在 TSX 中维护展示 `defaultValue`。
- `zh-CN / en-US / id-ID` 已补齐 `enterpriseAlertsWorkspace.syncAndWorker.webhookReceipts.*` 与 `enterpriseAlertsWorkspace.syncAndWorker.dlq.*` 缺失 key，并保留 `count / loaded / total / queued / skipped / failed / processed / dlq / replayed / dispatchMode / receiptID / connector / entryID / stage / detail` 等插值参数。
- `web-admin/src/components/enterprise` 与 `web-admin/src/features/enterprise` 已无 `defaultValue` 命中，Enterprise 前端文案收口进入完成状态。
- 下一批优先转入 `en-US.json` 剩余占位值替换，尤其是 `description`、`title`、`empty`、`source value` 等仍可见占位文案。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- enterprise-sync-worker-alerts-e2e.spec.ts`

2026-04-25 第十八批阶段 5 Enterprise Alerts 执行历史文案推进已完成：

- `EnterpriseAlertsWorkspace` 的 Webhook execution history 区块已完成展示文案收口：标题、说明、类型/执行模式/派发模式筛选、搜索占位、执行详情、运行态摘要、表格列、空态、加载更多与详情动作不再在 TSX 中维护展示 `defaultValue`。
- `zh-CN / en-US / id-ID` 已补齐 `enterpriseAlertsWorkspace.syncAndWorker.executionHistory.*` 主体文案树，并保留 `loaded / total / value / requestedBy / auditSource / sourceExecutionID / workerRequirement / queueState / nextRetryAt / processingDeadlineAt / seconds / attempts / requeues / status` 等插值参数。
- 当前 `enterprise-alerts-workspace.tsx` 的剩余 `defaultValue` 已从 Webhook Receipt 与 DLQ 子块开始，下一批优先处理 Receipt 批量处理、最新批次摘要、Open execution、Process、Load more 以及 DLQ replay 对应文案。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- enterprise-sync-worker-alerts-e2e.spec.ts`

2026-04-25 第十七批阶段 5 Enterprise Alerts 通知历史文案推进已完成：

- `EnterpriseAlertsWorkspace` 的 Sync & Worker notification history 区块已完成展示文案收口：标题、说明、筛选器、搜索占位、导出、状态摘要、批量重试/静默/恢复、表格列、空态、行内状态、详情字段与加载更多按钮不再在 TSX 中维护展示 `defaultValue`。
- `zh-CN / en-US / id-ID` 已补齐 `enterpriseAlertsWorkspace.syncAndWorker.notificationHistory.*` 完整文案树，并保留 `count / id / age / at / status / result` 等插值参数。
- `enterprise-sync-worker-alerts-e2e.spec.ts` 已同步通知历史按钮和筛选器的中文 locale 断言，覆盖导出、自动重试、批量重试、批量静默和恢复流程。
- 当前 `enterprise-alerts-workspace.tsx` 的剩余 `defaultValue` 已从 Webhook execution history 区块开始，下一批优先处理执行历史、Webhook Receipt 与 DLQ 文案。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- enterprise-sync-worker-alerts-e2e.spec.ts`

2026-04-25 第十六批阶段 5 Enterprise 文案首轮推进已完成：

- Enterprise 同步工作台执行上下文提示收口：`EnterpriseSyncWorkspace` 的执行记录来源提示和“返回告警”按钮不再在 TSX 中维护展示 `defaultValue`。
- Worker alert guidance 时间文案收口：`enterprise-sync-worker-alert-guidance.ts` 的秒/分钟/小时/天格式全部改为 locale key。
- Enterprise 页面 worker alert 通知加载、导出、重试、静默、恢复相关错误和成功摘要迁入 `enterprisePage.errors.*` 与 `enterprisePage.syncSummary.*`。
- `EnterpriseAlertsWorkspace` 首批收口执行历史基础文案：执行类型、worker 要求、通知恢复状态、pending age、执行状态筛选、运行态筛选、重放链路筛选均改为 locale key。
- `enterprise-sync-worker-alerts-e2e.spec.ts` 补齐 `hris-webhook-executions` 列表/详情 mock，并同步 worker alert 成功摘要中文断言，避免新执行历史加载触发 unmocked route。
- `zh-CN / en-US / id-ID` 已补齐上述 Enterprise 文案。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- enterprise-sync-worker-alerts-e2e.spec.ts`

2026-04-25 第十五批阶段 5 Wallet 文案首轮推进已完成：

- Wallet 高价值展示文案收口：`WalletAlertConfigCard`、`WalletRiskOverviewPanels`、`WalletAlertTrendPanels`、`WalletAlertSubscriptionCard`、`WalletAlertNotificationRecordsCard`、`WalletDlqCleanupArchivesCard`、`WalletTemplateManagerCard`、`WalletIssueJobQueueCard` 不再在 TSX 中维护展示 `defaultValue`。
- Wallet 模板表单校验错误迁入 locale：模板名称必填/长度、`class_id` 长度、`style_config` 长度均改为 `walletPage.components.templateManager.validation.*`。
- Wallet 单笔/批量发放表单校验错误迁入 locale：模板必选、员工/访客 ID 必填与长度、过期时间格式、批量目标必填与长度均改为 `walletPage.components.issueQueue.validation.*`。
- `zh-CN / en-US / id-ID` 已补齐上述 Wallet 校验文案，并确认 `components/wallet` 与 `features/wallet` 已无展示 `defaultValue` 命中。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- role-boundary-e2e.spec.ts`

2026-04-25 第十四批阶段 5 Access 文案首轮推进已完成：

- Access 核心工作台展示文案收口：`AccessPageHeader`、`AccessSectionsTabs`、`AccessGrantFilterBar`、`AccessGroupForm`、`AccessPolicyForm` 不再在 TSX 中维护展示 `defaultValue`。
- 用户组表单校验错误迁入 locale：用户组名称必填/长度、描述长度、成员搜索长度均改为 `accessPage.components.groupForm.validation.*`。
- 策略表单校验错误迁入 locale：策略名称、楼宇/区域/门点 ID、时段、成员数格式和范围必选错误均改为 `accessPage.components.policyForm.validation.*`。
- `zh-CN / en-US / id-ID` 已补齐上述校验文案，避免表单错误只能显示英文硬编码。
- 修复 `access-route-e2e.spec.ts` 的验证前置：登录流程显式切换中文，并将旧的“权限策略”断言更新为当前 locale 的“访问策略”。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- access-route-e2e.spec.ts`
- `npm run test:e2e -- access-enterprise-flow-e2e.spec.ts`

2026-04-25 第十三批阶段 4.6 Gateways 页面密度首轮推进已完成：

- 网关注册表单改为默认折叠的 `Gateway Setup` 面板，避免低频注册配置和命令中心、检索、台账同屏竞争。
- 网关重启从直接执行改为确认弹窗，弹窗说明短暂离线影响并展示目标网关，符合高危操作先确认的密度规则。
- 网关台账默认隐藏 `Device Capacity / Bound Doors`，platform 视角额外默认隐藏 `Building`，默认可见列控制在 6 列以内，其余字段仍可通过 Columns 菜单打开。
- 网关注册表单同步复用 4.5 的响应式规则，宽屏再展开多列，窄屏保持单列。
- 补齐 `zh-CN / en-US / id-ID` 的网关 setup 和重启确认文案。
- 更新 E2E：`f4-role-surface-e2e.spec.ts` 覆盖重启确认弹窗；`role-boundary-e2e.spec.ts` 覆盖 tenant_admin 网关注册默认折叠。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- f4-role-surface-e2e.spec.ts -g "building_admin reboot failure should show api error"`
- `npm run test:e2e -- role-boundary-e2e.spec.ts`
- `npm run test:e2e -- f4-role-surface-e2e.spec.ts`

2026-04-25 第十二批阶段 4.5 高优先级首轮推进已完成：

- 完成 Events 筛选区响应式收口：主筛选和平台高级筛选从 `md` 多列改为 `lg` 多列，窄屏按钮改为双列/单列稳定布局，避免 Reset 和 Advanced Filters 与长翻译文本互相挤压。
- 完成 Alarms 筛选区和通知策略响应式收口：搜索、状态、严重程度和通知渠道配置在中屏先堆叠或双列展示，宽屏再展开为多列。
- 完成 Gateways 搜索、列表工具条和命令中心响应式收口：搜索栏、绑定/解绑、发布配置、重启、挂载设备、RS485 参数从过早多列展开改为 `lg/xl` 展开，中屏保持可读。
- 完成 Spaces 角色固定范围字段替换：非 platform 视角下不再用窄 Badge 承载租户/楼宇范围，改用 `ScopeLockedField` 静态范围字段。
- 完成 Gateways 只读禁用态说明补齐：绑定、解绑、发布配置、重启、探测 legacy 设备和挂载设备按钮增加 `title`，挂载设备同时提示只读原因或容量已满。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- f4-role-surface-e2e.spec.ts`
- `npm run test:e2e -- role-boundary-e2e.spec.ts`

2026-04-25 第十一批阶段 4.4 推进已完成：

- 完成通用业务组件库首轮补齐：`ScopeLockedField`、`OperationalKPI`、`DetailDrawer`、`ActionInbox`、`EmptyState`、`PermissionBoundary` 已新增；`RoleScopeBanner` 沿用现有实现。
- `ScopeLockedField` 提供角色固定范围字段的静态展示，后续可替换单选项 select。
- `OperationalKPI` 提供统一 KPI 卡片，支持趋势 Badge、说明和图标。
- `DetailDrawer` 封装基于 Sheet 的详情抽屉；`EventDetailDrawer` 已迁入该通用抽屉，作为低风险接入验证。
- `ActionInbox` 提供处置队列基础骨架：摘要、列表、状态 Badge、动作按钮和空状态。
- `EmptyState` 与 `PermissionBoundary` 提供统一空态和权限不足 fallback，减少后续页面静默隐藏。
- 新增 `business-components.test.ts`，覆盖通用组件任务面、抽屉接线和权限 fallback。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- role-boundary-e2e.spec.ts`
- `npm run test:e2e -- f4-role-surface-e2e.spec.ts -g "tenant_admin events filter should support type switch"`

2026-04-25 第十批阶段 4.3 收口已完成：

- 完成 `Dialog` 收口：默认弹窗改为亮色任务面、冷灰边框、`rounded-lg` 8px 圆角和无阴影；标题、描述、Footer 与关闭按钮适配亮色背景。
- 完成 `Sheet` 收口：默认侧滑面板改为亮色任务面，按 `top/right/bottom/left` 方向提供 22px 边缘圆角和冷灰边框，为后续 `DetailDrawer` 复用打基础。
- 完成 `Tabs` 收口：line 变体 active 文本和下划线使用 `interaction` token。
- 完成 `Skeleton` 收口：新增 `variant="task"`，任务卡内可使用 `#f2f2f2` 亮色骨架屏。
- 扩展 `ui-variants.test.ts`，覆盖 line tab 的 Interaction Blue 类名和 task skeleton 类名。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- role-boundary-e2e.spec.ts`

2026-04-25 第九批阶段 4.3 首轮推进已完成：

- 完成 shadcn/ui 首轮组件变体：`Card`、`Button`、`Table`、`Badge` 已接入阶段 4.2 新增的设计 token。
- `Card` 新增 `variant="task"`，使用 `bg-card-task`、`border-card-task-border`、`rounded-card`、无阴影和亮色文本；Header/Title/Description/Footer 已通过 group data 继承任务卡样式。
- `Button` 新增 `variant="interaction"`，保持透明底，并在 hover/focus/expanded 态使用 Interaction Blue。
- `Table` 新增 `variant="task"`，容器和行级样式支持亮色任务表格、冷灰边框和 `#fafafa` hover。
- `Badge` 新增 `success / warning / danger` 语义状态变体，为后续页面状态标签迁移打基础。
- 新增 `ui-variants.test.ts`，直接覆盖任务卡、交互按钮和语义徽标的 token 类名输出。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- role-boundary-e2e.spec.ts`

2026-04-25 第八批阶段 4.2 推进已完成：

- 完成 CSS Token 体系统一：`tailwind.config.ts` 已移除 `hsl(var(--*))` 旧格式，统一改为 CSS variable 直接引用。
- 新增 DESIGN.md 对齐 token：`--radius-card: 22px`、`--color-interaction: #1863dc`、`--card-task: #ffffff`、`--card-task-border: #e5e7eb`。
- Tailwind 配置新增 `interaction`、`card-task`、`card-task-border` 色值映射，以及 `rounded-card` 半径映射，为阶段 4.3 的 Card/Button/Table/Badge 变体做准备。
- Tailwind v4 `@theme inline` 已同步暴露交互色、任务卡背景、任务卡边框和卡片半径 token，避免后续组件同时维护两套样式入口。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- role-boundary-e2e.spec.ts`

2026-04-25 第七批阶段 3.4 推进已完成：

- 完成 Wallet 高级运营折叠：`/wallet` 不再使用 `Operations / Advanced` 二级 Tab，日常操作流直接承载模板、发放、投递、回执恢复和已发放凭证台账。
- `WalletAdvancedWorkspace` 改为默认关闭的 `Advanced Operations` 折叠区，顶部展示实体卡任务、告警、通知、DLQ 归档和订阅状态摘要。
- 实体卡任务迁入高级运营区，避免低频任务队列和日常发放主路径抢首屏空间。
- 高级运营区展开后继续承载实体卡任务、告警订阅、风险概览、趋势面板、告警通知记录和 DLQ 清理归档。
- 恢复 Wallet 投递回执恢复文案：批量重发失败通道、批量状态修复、单条重发失败通道，以及成功/失败结果摘要。
- 更新 `role-boundary-e2e.spec.ts`，为 Wallet mock 补齐 metrics/trend/subscription 响应，并覆盖高级运营默认折叠、无高级 Tab、点击展开的行为。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:e2e -- role-boundary-e2e.spec.ts`
- `npm run test:e2e -- enterprise-idp-outcome-e2e.spec.ts -g "wallet batch retry should update retryable count"`
- `npm run test:e2e -- enterprise-idp-outcome-e2e.spec.ts -g "wallet batch repair should update repairable count"`

2026-04-25 第六批阶段 3.3 推进已完成：

- 完成 Enterprise Alerts 首轮拆分：新增 `enterprise-jit-approval-inbox.tsx`、`enterprise-sync-exceptions.tsx`、`enterprise-worker-alerts.tsx`、`enterprise-hris-receipts.tsx`、`enterprise-hris-dlq.tsx`。
- `enterprise-alerts-workspace.tsx` 已将 JIT 审批、同步异常、Worker 告警、HRIS Receipts、HRIS DLQ 包入对应子工作区组件，先建立清晰边界与稳定测试定位。
- 保持共享筛选、查询、状态和动作编排仍在父组件，避免一次性迁移导致审批、回执、DLQ、Worker 告警联动回归；后续可按子工作区逐步下沉状态和详情抽屉。
- 恢复 JIT 审批区带计数的批量审批/拒绝/回写按钮、状态筛选和批量结果文案，补齐部分失败提示，便于运营确认处理范围。
- 更新 `enterprise-sync-worker-alerts-e2e.spec.ts`，覆盖 Sync Worker 跳转 Alerts 后新增子工作区边界可见。
- 更新 `enterprise-idp-outcome-e2e.spec.ts`，覆盖 JIT 审批边界，并回归批量审批、拒绝和外部回写的成功、部分失败、全失败、筛选子集场景。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:unit`
- `npm run test:e2e -- enterprise-sync-worker-alerts-e2e.spec.ts -g "enterprise worker alert link bridges sync and alerts workspaces"`
- `npm run test:e2e -- enterprise-idp-outcome-e2e.spec.ts -g "enterprise alerts batch"`

2026-04-25 第五批阶段 3.2 推进已完成：

- 完成 Alarms 页面处置队列改造：默认按 `Open / Investigating / Mitigated / Closed` 分组展示，不再以全量表格作为主视图。
- 告警条目改为卡片式处置单元，按当前状态提供下一步主动作：确认、调查、缓解、关闭；已完成状态显示完成态按钮。
- 通知策略与通知日志收进“通知配置”二级 Tab，默认“处置队列”视图只保留筛选、分页和分组告警。
- 保持 `building_admin` 楼宇边界：仍由账号 `building_ids` 固定过滤，缺少楼宇范围时不展示告警。
- 补齐 `zh-CN / en-US / id-ID` Alarms 新增 Tab、分组、字段和下一步动作文案。
- 更新 `f4-role-surface-e2e.spec.ts`，覆盖状态分组标题、卡片下一步确认动作、通知日志 Tab、失败时通知日志为空，以及既有告警筛选链路。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:unit`
- `npm run test:e2e -- f4-role-surface-e2e.spec.ts -g "building_admin should update alarm workflow|building_admin alarm status update failure|tenant_admin alarm filters"`

2026-04-25 第四批阶段 3.1 推进已完成：

- 完成 Events 页面时间线化收口：默认最近 24 小时窗口，顶部 KPI 精简为通行量、拒绝事件、设备异常、离线事件。
- 事件筛选改为主筛选区 + 折叠高级筛选：默认只露出时间、事件类型、关键字；平台管理员的租户筛选收进高级筛选。
- 新增事件详情抽屉，点击表格行后展示事件结果、租户/楼宇/区域/门点/网关/执行体信息，以及完整原始 JSON。
- 保持角色边界：`building_admin` 继续按账号楼宇范围过滤且无楼宇选择器；`super_admin` 显示租户列，非平台角色隐藏租户列。
- 补齐 `zh-CN / en-US / id-ID` Events 新增文案。
- 扩展 `f4-role-surface-e2e.spec.ts`，覆盖默认 24 小时窗口、事件筛选重置和详情抽屉原始 JSON；同时让该文件登录 helper 显式切换中文，避免浏览器默认语言影响中文断言。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:unit`
- `npm run test:e2e -- f4-role-surface-e2e.spec.ts -g "tenant_admin events filter should support type switch"`

2026-04-25 第三批阶段 2.3 推进已完成：

- 完成 Enterprise 页面角色化入口：`super_admin` 的 `/enterprise` 标题为“企业健康”，默认落到同步健康区；`tenant_admin` 标题为“目录与登录”，默认落到员工目录区。
- 保留 Enterprise 深链能力：`#employees / #sync / #idp / #alerts` 仍可直接进入对应工作区。
- `enterprise-page.tsx` 的 tab 入口已改为按角色可见 section 数组渲染，后续继续收紧 tab 权限时只需调整单点配置。
- `enterprise-page-header.tsx` 已改为按角色读取 header 文案，`zh-CN / en-US / id-ID` 已补齐新文案。
- `role-boundary-e2e.spec.ts` 新增平台管理员、租户管理员的 Enterprise 默认入口断言。
- 稳定 `enterprise-idp-outcome-e2e.spec.ts` 的登录语言选择和 outcome 动作定位，验证 `#idp` 到 `#alerts` 的既有深链流程未回归。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:unit`
- `npm run test:e2e -- role-boundary-e2e.spec.ts`
- `npm run test:e2e -- enterprise-idp-outcome-e2e.spec.ts -g "enterprise idp outcome should go to alerts when approvals are pending"`

2026-04-25 第二批阶段 2 推进已完成：

- 完成阶段 2.1 Sidebar 导航分组：`App.tsx` 的导航从平铺列表调整为 `Command / Sites / People / Platform` 分组结构，不同角色只渲染有意义的分组。
- 将已有 `AuditPage` 挂到 `/audit`，并通过 `platform.audit.view` 能力和 `ProtectedRoute` 限定为平台管理员入口。
- 完成阶段 2.2 的范围标识：新增 `RoleScopeBanner`，分别展示平台跨租户范围、当前租户、楼宇范围和运营视图；楼宇管理员会按 `building_ids` 解析楼宇名，失败时回退 ID。
- 完成阶段 2.4 resident 拦截：新增 `NoPermissionPage`，`resident` 登录后不进入 `AppShell`，只显示无权限说明和切换账号操作。
- 补齐 `zh-CN / en-US / id-ID` 对应导航分组、Audit、范围标识和无权限页文案。
- 更新 `role-boundary-e2e.spec.ts`，新增平台管理员导航分组与 `/audit` 入口、resident 无权限页、楼宇范围标识的回归断言。

本批验证已通过：

- `npm run typecheck`
- `npm run check:cjk`
- `npm run build`（通过，保留既有 chunk size warning）
- `npm run test:unit`
- `npm run test:e2e -- role-boundary-e2e.spec.ts`

已完成第一批低风险整改：

- 在 `web-admin/src/lib/viewer.ts` 建立声明式角色能力矩阵，保留原有 `canAccess*` 函数作为兼容层。
- 收紧 `operator` 权限：运营角色不再进入完整 Enterprise/HRIS/SSO 工作区。
- 在 `web-admin/src/App.tsx` 接入角色化 Enterprise 导航命名：平台管理员看到“企业健康”，租户管理员看到“目录与登录”。
- 在 Wallet 工作台隐藏运营角色无权限访问的 Enterprise/Access 跳转入口，避免只读角色点击后被路由守卫打回。
- 修复 `wallet-page-utils.ts` 中导致 `check:cjk` 失败的硬编码中文关键词。
- 清理 `zh-CN.json` 首批明显占位文案，当前占位扫描为 `zh-CN 0`、`id-ID 0`；`en-US` 还剩 Enterprise 深层工作区占位文案，列入下一批。
- 修正 `docs/testing/admin-ui-test-and-api-map.md` 的认证说明：当前 token 存储在 `sessionStorage`，并已接入 refresh token。
- 更新 `role-boundary-e2e.spec.ts`，新增 `operator` 不能访问 `/enterprise` 的回归断言，并让测试显式切到中文，避免浏览器语言导致误判。

本轮验证已通过：

- `npm run typecheck`
- `npm run build`
- `npm run check:cjk`
- `npm run test:unit`
- `npm run test:e2e -- role-boundary-e2e.spec.ts`

---

## 14. 阶段 7：以 Kisi 为基准的前端全面重构

> 本阶段已独立为 [`MISTYPASS-KISI-UI-REFORM-PLAN.md`](./MISTYPASS-KISI-UI-REFORM-PLAN.md)。
>
> 核心内容：
> - 双 Sidebar 导航架构（Organization Admin vs Place Admin）
> - 视觉系统全面转向白色企业风
> - 页面重构方案（基于 5 张 Kisi 截图分析）
> - 操作模式按信息量分层（独立页面 vs Sheet 侧滑）
> - 实施路径：Phase 0 先出 Home 页验证 UI 风格，确认后再推进
