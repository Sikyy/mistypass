# MistyPass 角色化前端整改计划

## 1. 目标与边界

本次整改的核心目标不是继续堆功能，而是把 MistyPass 管理后台改成按客户类型、角色权限和真实运营任务组织的产品界面。

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

当前状态：告警列表 + 状态操作混在一起。
目标：处置队列 + 状态分组。

改造内容：

| 子项 | 说明 | 涉及文件 |
|------|------|----------|
| 3.2.1 | 默认按状态分组展示：Open → Investigating → Mitigated → Closed | `alarms-page.tsx` |
| 3.2.2 | 每条告警卡片显示下一步动作按钮（确认/调查/缓解/关闭） | `alarms-page.tsx` |
| 3.2.3 | 通知策略和通知日志收进二级 tab，不和主告警列表同屏 | `alarms-page.tsx` |
| 3.2.4 | `building_admin` 只显示本楼宇告警 | `alarms-page.tsx` |

#### 3.3 Enterprise Alerts 拆分

当前状态：`enterprise-alerts-workspace.tsx` 5336 行，堆叠 5 个产品域。
目标：拆为独立子工作区组件。

| 子工作区 | 新组件文件 | 内容 |
|----------|-----------|------|
| JIT 审批收件箱 | `enterprise-jit-approval-inbox.tsx` | 待审批列表 + 审批/拒绝动作 |
| 同步异常 | `enterprise-sync-exceptions.tsx` | 失败同步任务 + 重试/跳过 |
| Worker 告警 | `enterprise-worker-alerts.tsx` | 阈值告警 + 订阅配置 |
| HRIS 回执 | `enterprise-hris-receipts.tsx` | Webhook 投递回执 + 状态 |
| HRIS DLQ | `enterprise-hris-dlq.tsx` | DLQ 条目 + 回放/清理 |

每个子工作区使用统一模式：”摘要卡 + 处理队列 + 详情抽屉”。

#### 3.4 Wallet 高级运营折叠

当前状态：`wallet-page.tsx` 3192 行，模板/发放/投递/物理卡/队列/趋势/告警/DLQ 全部平铺。
目标：主路径（模板 + 发放 + 投递）保持首屏，高级运营（队列/趋势/DLQ/告警订阅）折叠到 “Advanced Operations” 可展开区。

涉及文件：
- `web-admin/src/features/wallet/pages/wallet-page.tsx` — 将高级运营区包裹在 collapsible 容器中。
- `web-admin/src/components/wallet/wallet-advanced-workspace.tsx` — 已存在，调整为默认折叠。

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

当前状态：`index.css` 已有深色雾岛 token（oklch），但 `tailwind.config.ts` 仍引用 `hsl(var(--*))` 旧格式。

| 子项 | 说明 |
|------|------|
| 4.2.1 | 统一 `tailwind.config.ts` 颜色引用为 CSS variable 直接引用，移除 `hsl()` 包裹 |
| 4.2.2 | 新增 `--radius-card: 22px` 对应 DESIGN.md 签名圆角 |
| 4.2.3 | 新增 `--color-interaction: #1863dc` 用于 hover/focus 统一 |
| 4.2.4 | 新增任务卡片变体 token：`--card-task: #ffffff`（亮色任务卡背景）、`--card-task-border: #e5e7eb` |

涉及文件：
- `web-admin/src/index.css`
- `web-admin/tailwind.config.ts`

#### 4.3 shadcn/ui 组件定制

基于当前已安装的 22 个 shadcn/ui 组件，按 DESIGN.md 规范定制：

| 组件 | 当前状态 | 定制方向 |
|------|----------|----------|
| `Card` | 深色玻璃卡片（`bg-card` oklch 深色） | 新增 `variant=”task”` 亮色任务卡变体：白色背景、`border-cool` 边框、22px 圆角、无阴影 |
| `Button` | 6 个变体，深色主题适配 | 新增 `variant=”interaction”` 变体：透明底 → hover 时 Interaction Blue 文字 |
| `Table` | 基础表格 | 新增 `variant=”task”` 亮色表格：白色行、冷灰分隔线、hover 行高亮 `#fafafa` |
| `Dialog` | 标准弹窗 | 圆角改为 `8px`（DESIGN.md dialog 规范），亮色背景 |
| `Sheet` | 侧滑面板 | 作为 DetailDrawer 的基础，亮色背景 + 22px 顶部圆角 |
| `Badge` | 6 个变体 | 新增语义状态变体：`success`（绿）、`warning`（黄）、`danger`（红） |
| `Tabs` | 2 个变体（default/line） | line 变体下划线改为 Interaction Blue |
| `Skeleton` | 基础骨架屏 | 保持当前深色适配，任务卡内使用亮色骨架 |

涉及文件：
- `web-admin/src/components/ui/card.tsx` — 新增 `variant` prop
- `web-admin/src/components/ui/button.tsx` — 新增 interaction 变体
- `web-admin/src/components/ui/table.tsx` — 新增 task 变体
- `web-admin/src/components/ui/badge.tsx` — 新增语义变体

#### 4.4 通用业务组件库

新建以下通用组件，供所有页面复用：

| 组件 | 用途 | 文件 |
|------|------|------|
| `RoleScopeBanner` | 顶栏角色范围标识 | `components/role-scope-banner.tsx` |
| `ScopeLockedField` | 角色固定的筛选字段（静态标签替代 select） | `components/scope-locked-field.tsx` |
| `OperationalKPI` | 统一 KPI 指标卡（数值 + 趋势 + 标签） | `components/operational-kpi.tsx` |
| `DetailDrawer` | 统一详情侧滑面板（基于 Sheet） | `components/detail-drawer.tsx` |
| `ActionInbox` | 统一处置队列（摘要卡 + 列表 + 动作按钮） | `components/action-inbox.tsx` |
| `EmptyState` | 统一空状态（品牌化插图 + 下一步引导） | `components/empty-state.tsx` |
| `PermissionBoundary` | 权限不足说明（替代静默隐藏） | `components/permission-boundary.tsx` |

#### 4.5 溢出与响应式修复

逐页扫描并修复：

| 问题类型 | 修复规则 | 优先页面 |
|----------|----------|----------|
| 固定宽度承载翻译文本 | 改为 `minmax(0, 1fr)` 或 flex wrap | Spaces, Gateways, Enterprise |
| 表格截断关键状态 | 改为 tooltip 或 DetailDrawer | Events, Alarms, Wallet |
| 单选项 Select | 改为静态标签 | Spaces（楼宇联动）, Access |
| 禁用无解释 | 添加 tooltip 说明原因 | Gateways, Wallet, Enterprise |
| 筛选区窄屏溢出 | 改为单列 stack | Events, Gateways |

#### 4.6 页面密度规则

- 首屏只放”当前角色今天最该看的 5 件事”。
- 配置表单不和运营队列同屏竞争。
- 高危操作进入二级抽屉或确认弹窗。
- 表格默认列不超过 6 列，其他信息进入 DetailDrawer。
- 移动端优先卡片化，避免宽表横向溢出。

---

### 阶段 5：文案和文档收口

| 子项 | 说明 | 状态 |
|------|------|------|
| 5.1 | 替换 `en-US.json` 剩余 115 个占位值（`description`、`hint`、`card description`） | ⬜ |
| 5.2 | 替换 `zh-CN.json` 剩余占位值 | ✅ 首批已完成 |
| 5.3 | 把 TSX 中仍承担实际展示的 `defaultValue` 收口到 locale 文件 | ⬜ |
| 5.4 | 为权限不可见和禁用态补充明确提示文案 | ⬜ |
| 5.5 | 按角色说话：删除平台内部术语，用客户能理解的语言 | ⬜ |
| 5.6 | 危险操作写结果（如”将重启网关并产生短暂离线”），不只写”执行” | ⬜ |

---

### 阶段 6：验证

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
阶段 3.2 Alarms 处置队列重构 ← 下一步
    ↓（可并行）
阶段 3.3 Enterprise Alerts 拆分
阶段 3.4 Wallet 高级运营折叠
    ↓
阶段 4.2 CSS Token 统一
阶段 4.3 shadcn/ui 组件定制（Card/Button/Table/Badge 变体）
    ↓
阶段 4.4 通用业务组件库
阶段 4.5 溢出与响应式修复
    ↓
阶段 5 文案收口
    ↓
阶段 6 验证
```

## 11. 首批建议更新项

> 已被阶段 10 的详细实施计划取代。阶段 2 与阶段 3.1 已完成，下一步从阶段 3.2（Alarms 处置队列重构）开始执行。

## 12. 需要业务确认的点

- `tenant_admin` 是否就是“办公室/企业管理员”，还是还存在“物业公司管理多个办公室”的客户类型。
- `building_admin` 是否需要管理员工权限，还是只做楼宇资产和设备运维。
- `operator` 是否允许重试 Wallet 投递和队列任务，还是只读。
- 普通 `resident` 登录 web-admin 时，是直接拒绝、跳转自助端，还是显示无权限说明页。
- 是否需要新增“客户类型/账号类型”字段来区分办公楼物业、入驻企业、园区、学校、医院；如果需要，应作为后续后端最小增量，不影响第一阶段前端重组。

## 13. 本轮推进更新项

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
