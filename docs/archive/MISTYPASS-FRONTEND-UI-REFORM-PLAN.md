# MistyPass 前端 UI 改造计划

## 1. 项目类型与定位

MistyPass 是一个云端门禁控制 SaaS 平台。当前仓库主要包含：

- `api/`：Go 后端服务，使用 Chi Router、模块化领域服务、JWT/RBAC、PostgreSQL 状态快照与投影、可选 Redis 易失状态、可选 MQTT/NATS 集成，以及运维指标与链路追踪。
- `web-admin/`：React + TypeScript + Vite 管理后台，使用 Tailwind/shadcn 风格组件、TanStack Query/Table、i18next、多角色可见路由。
- `docs/`：内部架构文档、外部 API Wiki、测试 Runbook、Sprint 计划、Cloud/Edge 边界文档。

该产品不是普通租户 CRUD 后台，而是面向安全运营的云控制台：

- 多租户楼宇、站点、空间与门点管理。
- 企业身份集成：OIDC/SAML、HRIS/Talenta connector、JIT 建档与审批。
- 权限策略、目录、用户组、授权与临时权限管理。
- 网关/控制器运营：legacy Wiegand、OSDP、RS485 兼容，序列号库存，OTA/命令流，checkpoint lag 可视化。
- 通行事件、设备事件、告警、审计、worker alert、replay/DLQ 运维。
- MistyAccess / Wallet 凭证发放与投递运营，面向 Apple Wallet 和 Google Wallet 类移动凭证体验。

## 2. 服务客户画像

核心客户是需要跨多个物理地点运营门禁系统的组织：

- 商业办公楼、多楼宇企业、联合办公运营方。
- 学校、校区、培训园区。
- 医院与高合规设施。
- 物业与园区运营方，需要统一可视化管理安防设备。
- 从传统门禁硬件迁移到云端控制和移动凭证的企业客户。

因此，管理后台应该呈现为“实时安全指挥中心”，而不是普通 SaaS 设置后台。

## 3. 当前架构方向

根据现有文档，项目方向是明确的 Cloud/Edge 分工：

- Cloud 负责租户编排、策略管理、目录同步、审计、Wallet 发放、网关合同链路和运营可见性。
- Edge 负责门侧实时判定、本地缓存、离线执行和事件队列回放。
- 强约束：开门链路不能依赖云端实时往返。

本次 UI 改造不改变后端合同。前端目标是在保留现有路由、权限和 API 数据流的前提下，把现有模块重新组织成更清晰的运营分区。

## 4. 设计资产审查

已审查的设计资产：

- `picture/设计理念.md`
- `picture/generated-1776874282565.jpg`
- `picture/generated-1776873958671.jpg`
- `picture/generated-1776874864848.jpg`
- `picture/generated-1776875190929.jpg`
- `picture/generated-1776875455851.jpg`
- `picture/logo/logo1.png`
- `picture/logo/logo2.png`

其中 `generated-1776874282565` 是当前最适合作为主方向的参考，因为它同时具备：

- 深色指挥中心外壳。
- 黑灰单色的雾气与岛屿氛围。
- 银色高光、柔和发光边框、玻璃质感卡片。
- 清晰 KPI 卡片和小型趋势线。
- 表达“单一客户、多地办公室”的站点概览模块。
- 用心跳/健康度模块表达分布式站点和网关运行状态。
- 把 Wallet 凭证分布作为一等运营模块，而不是附属功能。

## 5. 视觉系统方向

前端应从当前偏青色/蓝绿色的后台主题，转向 MistyIslet / MistyPass 风格的安全指挥中心语言：

- 基础色：近黑、石墨灰、烟雾灰、暖白。
- 点缀色：银色/珍珠高光；绿色和红色只用于状态表达。
- 表面质感：半透明深色卡片、细边框、模糊背景、内发光。
- 品牌母题：雾流、岛屿轮廓、站点轨道连接、心跳线、指挥中心遥测。
- 布局方式：固定左侧 command rail、顶部态势栏、主仪表盘画布、按运营意图分组的功能卡片。
- 字体策略：继续使用项目现有 Geist 依赖，避免引入额外字体风险；通过更强的标题层级、小号大写标签和数字强调来提升识别度。

## 6. 必须保留的功能块

本次改造不改变后端行为和 API 合同。以下功能块必须保留：

- Dashboard：摘要、健康度、最近告警、当前重点。
- Tenants：租户列表与租户拓扑。
- Enterprise：企业目录、同步、IdP、HRIS connector、worker alert 运维。
- Spaces：楼宇、楼层、区域、门点。
- Access：目录、策略、授权。
- Wallet：模板、凭证实例、投递、告警订阅、DLQ、指标和趋势面板。
- Gateways：网关列表、注册、绑定/解绑、下挂设备注册、命令进度、序列号库存、checkpoint monitor。
- Events、Alarms 和基于角色的路由可见性。

## 7. 实施计划

阶段 1：基础层

- 先在根目录建立本计划文档。
- 更新 `web-admin/src/index.css` 中的全局主题 token，切到深色雾岛/银色高光调性。
- 增加可复用 CSS utility，用于雾面面板、指标卡、发光边框、雾气背景、心跳线。
- 更新 `web-admin/src/App.tsx`，把全局 shell 改为指挥中心风格：左侧 command rail、紧凑品牌标识、会话卡、雾面主画布。

阶段 2：Dashboard

- 围绕 `generated-1776874282565` 重建 `/dashboard` 视觉层级。
- 增加氛围 hero、四个 KPI 模块、站点星图/多地办公室心跳模块、访问健康模块、最近告警和当前重点。
- 只使用现有已拉取数据，不新增后端字段。

阶段 3：Gateway Operations

- 把网关 KPI 和 checkpoint monitor 改成“边缘心跳遥测”表达。
- 增加控制器状态、队列 lag、站点/设备在线数的视觉分组。
- 保留注册、绑定、命令、库存和表格逻辑不变。

阶段 4：Wallet 与 Enterprise 入口

- 把 overview card 改成 command module 风格，让 MistyAccess、Wallet 分布、worker alert 运维和企业同步入口在视觉上更统一。
- 保留当前表单、表格和工作流状态。

阶段 5：验证

- 在 `web-admin` 内运行 `npm run typecheck` 和 `npm run build`。
- 如果本地服务可用，使用浏览器重点冒烟 `/login`、`/dashboard`、`/gateways`、`/wallet`、`/enterprise`。

## 8. 编辑边界

- 不编辑 `api/` 后端文件。
- 不改变 API 请求/响应类型，除非现有前端类型错误需要本地展示层修正。
- 不回滚当前 dirty worktree 中的用户已有改动。
- 不做大范围路由或权限变更。
- 优先通过 CSS 和组件展示层完成改造，避免触碰业务逻辑。

## 9. 本次已完成更新项

本次首轮改造已完成以下内容：

- 新增根目录计划文档：`MISTYPASS-FRONTEND-UI-REFORM-PLAN.md`。
- 新增雾岛品牌标识组件：`web-admin/src/components/brand/misty-island-mark.tsx`。
- 更新全局视觉基础：`web-admin/src/index.css`，包含黑灰雾岛配色、雾气背景、玻璃卡片、指标卡、站点轨道图、心跳线等 utility。
- 更新基础卡片质感：`web-admin/src/components/ui/card.tsx`，改为深色玻璃卡片、细边框、内高光和阴影。
- 改造全局应用外壳：`web-admin/src/App.tsx`，加入 command rail、雾岛品牌区、导航说明、会话卡、动态告警按钮和雾面主画布。
- 改造登录页：`web-admin/src/features/auth/pages/login-page.tsx`，统一为黑灰雾岛品牌氛围。
- 改造 Dashboard：`web-admin/src/features/dashboard/pages/dashboard-page.tsx`，加入氛围 hero、KPI 趋势卡、站点星图、多地办公室心跳表达、运营健康度和重点事项。
- 改造 Gateway 页面顶部与 KPI：`web-admin/src/features/gateways/pages/gateways-page.tsx`，加入控制器网络、心跳在线、设备槽位等指挥中心指标。
- 改造 checkpoint monitor：`web-admin/src/components/gateways/checkpoint-monitor.tsx`，改为边缘心跳遥测模块，保留窗口选择、表格、加载/错误/空态。
- 改造 Wallet overview：`web-admin/src/components/wallet/wallet-page-overview.tsx`，把入口卡片统一为 MistyAccess command module 风格。
- 改造 Enterprise 顶部与 overview：`web-admin/src/components/enterprise/enterprise-page-header.tsx`、`web-admin/src/components/enterprise/enterprise-page-overview.tsx`，统一为雾面 hero 和指标卡。
- 补充三语 i18n：`web-admin/src/locales/en-US.json`、`web-admin/src/locales/zh-CN.json`、`web-admin/src/locales/id-ID.json`，避免新 UI 标签硬编码。

## 10. 本次验证结果

已执行并通过：

- `npm run typecheck`
- `npm run build`
- `npm run test:unit`

已执行但存在既有问题：

- `npm run check:cjk` 未通过，失败点是既有文件 `web-admin/src/features/wallet/pages/wallet-page-utils.ts` 中的 `实体` 和 `临时` 关键词判断，不是本次 UI 改造新增内容。

## 11. 后续建议

下一步建议先做浏览器视觉验收：

- `/login`
- `/dashboard`
- `/gateways`
- `/wallet`
- `/enterprise`

如果首轮视觉方向确认，再继续推进更深的页面级改造：

- Spaces 页面：楼宇/楼层/门点改成“受保护岛屿资产图谱”。
- Access 页面：目录、策略、授权改成“身份-空间-时间”三段式分区。
- Events/Alarms 页面：加强实时事件流、告警优先级和处置队列表达。
- Wallet 页面：进一步做凭证分布环图、发行任务队列和 DLQ 风险排行的雾面可视化。
