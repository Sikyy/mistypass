# OTA Rollout 管理 UI — 设计文档(#5b)

> 日期：2026-06-08
> 状态：设计已确认,待出实施计划
> 上层目标:**全面对标 Kisi OTA**。本文是子项 **#5** 管理 UI 的第二块 **#5b Rollout UI**(#5a 固件 UI 已完成)。**做完即整个 OTA 对标收官。**
> 纯前端(web-admin React/TS);对标既有 web-admin 模式 + #5a。

---

## 1. 背景与范围

后端 #3(灰度 rollout)+ #4(调度)已完成;#5a 给了固件 UI。**#5b** = rollout 管理 UI:创建(完整,含调度)、列表、详情监控(每网关进度)、审批/暂停/恢复/中止控制。

### 已确认的关键决策
| 决策 | 选定 |
|---|---|
| 结构 | 独立 `/ota/rollouts` 页(创建卡 + 列表)+ 详情页 `/ota/rollouts/:rolloutID`(监控 + 操作);导航加「Rollouts」入口(复用已有 `:param` 详情路由先例如 `/users/:userID`) |
| 创建表单 | **完整(含调度)**:固件选择 + target(all/building/gateways)+ phases 数组 + 失败阈值 + 可选调度 |
| 详情监控 | `useRolloutDetail` 轮询 `refetchInterval` 5s(active/scheduled 时;终态停) |
| 目标多选 | 无 combobox 组件 → 指定网关用 **checkbox 列表**(填自 `listGateways`) |

### 复用(已核实)
- #5a 全部模式:`request`/`requestFormData`/`requestItems`(`lib/api/core.ts`)、shadcn ui、react-query v5、react-hook-form + zod、react-i18next 三语、vitest(逻辑/API 单测,非 .test.tsx)。
- `listGateways(token)` → `Gateway[]`(带 `building_id`、`current_firmware_version`);`listFirmware`(#5a)。
- 详情路由先例:`/users/:userID`、`/tenants/:tenantID`(`routes.tsx`)。
- 无 multi-select/combobox 组件(用 checkbox 列表);`Intl.supportedValuesOf('timeZone')` 可填时区 Select。

---

## 2. 真实后端契约(我建的 #3/#4)
| 用途 | 调用 |
|---|---|
| 建 | `POST /api/v1/gateways/rollouts?tenant_id=` body `{firmware_id, target, phases, failure_threshold_pct?, schedule?}` → `GatewayRollout` |
| 列表 | `GET /api/v1/gateways/rollouts?tenant_id=` → `{items: GatewayRollout[]}` |
| 详情 | `GET /api/v1/gateways/rollouts/{id}?tenant_id=` → `{rollout: GatewayRollout, gateways: RolloutGatewayStatus[]}` |
| 操作 | `POST /api/v1/gateways/rollouts/{id}/{approve\|pause\|resume\|abort}?tenant_id=` → `GatewayRollout` |

```
GatewayRollout = {
  id, tenant_id, firmware_id, firmware_version,
  target: { kind: "all"|"building"|"gateways", building_id?, gateway_ids?: string[] },
  phases: [{ percentage: number, requires_approval: boolean }],
  failure_threshold_pct: number,
  state: "pending"|"active"|"awaiting_approval"|"paused"|"completed"|"failed"|"scheduled",
  current_phase: number,
  schedule?: { start_at?, window_start?, window_end?, timezone? },
  created_by?, updated_by?, created_at, updated_at
}
RolloutGatewayStatus = { gateway_id, phase: number, ota_status: string, current_firmware_version? }
```
错误:firmware 不存在/target 空/phases 非法/阈值越界/schedule 非法 → 400;状态冲突(如 approve 非 awaiting_approval、pause 非 active)→ 409;not-found → 404。角色:写(建/操作)= super_admin/tenant_admin/building_admin;读 = + operator。tenant 走 query(tenant-scoped 用户省略)。

---

## 3. 架构 — 新增结构(`web-admin/src/features/ota/`)
- `pages/rollouts-page.tsx`(`/ota/rollouts`:创建卡 + 列表卡)
- `pages/rollout-detail-page.tsx`(`/ota/rollouts/:rolloutID`:详情 + 监控 + 操作)
- `components/rollout-create-card.tsx`(完整创建表单)
- `components/rollout-list-card.tsx`(列表表格,行链接详情)
- `components/rollout-detail.tsx`(详情主体:状态/phases/调度 + 每网关进度表 + 操作按钮)
- `hooks/use-rollouts.ts`(`useRolloutList`、`useRolloutDetail`[轮询]、`useRolloutAction` mutations)
- `lib/rollout-utils.ts`(可测纯逻辑)+ `.test.ts`
- `lib/api/ota.ts` 扩展 rollout 函数 + 类型(+ ota.test.ts 加 rollout 用例)
- 改 `lib/query-keys.ts`(rolloutList/rolloutDetail keys,`ns` 形)、`routes.tsx`(两路由)、`navigation.ts`(Rollouts 入口)、`locales/{en,id,zh}.json`(`ota.rollout` 命名空间)

---

## 4. 可测逻辑(`rollout-utils.ts`)
- `validatePhases(phases): boolean` — 非空、各 1–100、严格递增、末=100。
- `rolloutStateBadgeVariant(state): string` — active→"default"、completed→"success"、failed→"destructive"、paused→"warning"、awaiting_approval→"warning"、scheduled→"outline"、pending→"secondary"。
- `availableRolloutActions(state): Array<"approve"|"pause"|"resume"|"abort">` — 镜像后端守卫:awaiting_approval→[approve,abort];active→[pause,abort];paused→[resume,abort];scheduled→[abort];pending→[abort];completed/failed→[]。
- `buildingOptions(gateways): string[]` — 去重非空 building_id,排序。
- `isHHMM(s): boolean` — `^([01][0-9]|2[0-3]):[0-5][0-9]$`。
- `buildSchedulePayload(form): Schedule | undefined` — 任一调度字段有值则组对象(start_at 由 datetime-local 转 ISO),否则 undefined。
- `targetSummary(target): string` — "All gateways" / "Building {id}" / "{n} gateways"(列表展示用,可接 i18n)。

---

## 5. 组件设计

### 5.1 创建卡(`rollout-create-card.tsx`,write 角色)
react-hook-form + zod。字段:
- `firmware_id`:Select,选项来自 `listFirmware(token, tenantID)`(显示 version + channel)。
- `target.kind`:Select(all/building/gateways)。
  - `building`:Select,选项 = `buildingOptions(gateways)`。
  - `gateways`:checkbox 列表(来自 `listGateways`,显示 id + current_firmware_version);收集勾选 id 数组。
- `phases`:`useFieldArray` 动态行 {percentage(number), requires_approval(checkbox)};增/删按钮;客户端 `validatePhases` 报错;默认一行 `{percentage:100, requires_approval:false}`。
- `failure_threshold_pct`:number 输入,默认 20,范围 0–100。
- 调度(可选,折叠/开关):`start_at`(datetime-local)、`window_start`/`window_end`(HH:MM,`isHHMM` 校验)、`timezone`(Select 填 `Intl.supportedValuesOf('timeZone')`,回退 text)。
- 提交:组 payload(`buildSchedulePayload`、target 按 kind)→ `createRollout` → 成功 invalidate rolloutList + `navigate('/ota/rollouts/'+id)`;失败显后端错误。

### 5.2 列表卡(`rollout-list-card.tsx`)
`useRolloutList` → 表格:firmware_version / target(`targetSummary`)/ state(badge,`rolloutStateBadgeVariant`)/ current_phase / created_at(本地化)。行点击 → `navigate('/ota/rollouts/'+id)`。加载/空/错误态。

### 5.3 详情(`rollout-detail.tsx` + `rollout-detail-page.tsx`)
- `rollout-detail-page.tsx`:`useParams` 取 rolloutID,接 `{token, viewer}`,渲染 `rollout-detail`。
- `useRolloutDetail(token, tenantID, id)`:`refetchInterval` = state ∈ {active,scheduled,awaiting_approval,paused} ? 5000 : false。
- 展示:state badge、firmware_version、`targetSummary`、phases(列表)、current_phase、schedule(若有)、created/updated_by/at。
- **每网关进度表**:gateway_id / phase / ota_status(badge)/ current_firmware_version。
- **操作按钮**:`availableRolloutActions(state)` × write 角色 → 每个按钮 `useRolloutAction` mutation(approve/pause/resume/abort)→ 成功 invalidate 该 rollout detail;abort 二次确认(Dialog)。失败显消息(409 冲突文案)。

---

## 6. 数据流 / 轮询 / 错误
```
/ota/rollouts → useRolloutList;创建成功 → invalidate list + 跳详情
/ota/rollouts/:id → useRolloutDetail(非终态 5s 轮询)→ 操作 → mutation → invalidate detail
```
错误:卡内/按钮旁错误条(后端消息);409 状态冲突显友好文案;轮询失败保留上次数据 + 错误提示。

---

## 7. i18n(`ota.rollout.*` 三语)
页/卡标题、创建(字段标签/占位/校验/调度段/提交)、target kind 标签、phases(列/增删)、列表(表头/空)、详情(状态/进度表头/操作按钮/确认)、状态名(各 state 展示文案)、错误。三 locale 键结构一致。

## 8. 测试(vitest)
- **`rollout-utils.test.ts`**:`validatePhases`(合法/各非法)、`availableRolloutActions`(每 state)、`rolloutStateBadgeVariant`、`buildingOptions`(去重)、`isHHMM`、`buildSchedulePayload`(有/无调度)、`targetSummary`。
- **`ota.test.ts` 加**:createRollout(POST + body)、listRollouts、getRolloutDetail(解出 {rollout, gateways})、rolloutAction(POST 路径)。mock fetch。
- 组件不做渲染测(惯例;逻辑已在 utils + API)。

## 9. 不做(YAGNI)
rollout 编辑(后端无)、跨租户、复杂 target、图形化时间线(用表格 + badge)、组件渲染测、per-gateway 实时 SSE(用 5s 轮询)、调度的"下次窗口"倒计时。

## 10. 安全 / 边界
- 操作按钮按 `availableRolloutActions` + write 角色双门控;后端仍是真正守卫(409)。
- 轮询仅非终态;终态停轮询省请求。
- target=gateways 的 checkbox 列表只列**本租户**网关(listGateways 已租户范围)。

## 11. 工作量
约 2–2.5 天(纯前端;完整创建表单[phases 数组 + target + 调度] + 列表 + 详情监控轮询 + 操作 + 三语 + vitest)。这是 #5 中最大的一块。
