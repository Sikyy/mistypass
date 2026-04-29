# Mistyislet API 汇总

> 更新日期：2026-04-29
> 目标：作为 Mistyislet 管理后台资源 API 的总索引
> 本地基准：`Kisi-API-Bundled References.yaml`
> 线上基准：`https://api.getkisi.com/docs#`
> 产品概念参考：`https://docs.kisi.io/`
> UI 计划：`MISTYPASS-KISI-UI-REFORM-PLAN.md`

本文档按 Bundled Reference 的资源模型组织 API，而不是按旧后端模块组织。Mistyislet UI 可以继续使用更容易理解的产品文案，例如 Doors、Access Rights、Credentials；但新增后端 API 必须优先对齐参考 API 的资源名，例如 `locks`、`role_assignments`、`shares`、`cards`、`card_assignments`。

当前已完成第一批 reference-style endpoint：`/api/v1/places`、`/api/v1/locks`、`/api/v1/controllers`、`/api/v1/readers`、`/api/v1/terminals`、`/api/v1/groups`、`/api/v1/group_locks`、`/api/v1/group_zones`、`/api/v1/group_links`、`/api/v1/members`、`/api/v1/users`、`/api/v1/teams`、`/api/v1/team_memberships`、`/api/v1/cards`、`/api/v1/card_assignments`、`/api/v1/roles`、`/api/v1/role_assignments`、`/api/v1/shares`、`/api/v1/access_rights/schedule_templates`、`/api/v1/access_rights/schedule`、`/api/v1/access_rights/impact_preview`、`/api/v1/access_rights/review`、`/api/v1/event_sets`、`/api/v1/events/meta`、`/api/v1/events/types`、`/api/v1/reports`、`/api/v1/scheduled_reports`、`/api/v1/integrations`、`/api/v1/alert_policies`。`places/:id` 与 `locks/:id` 已支持 detail/update/delete，Place delete 已改为归档保存并默认从 active 列表/detail 隐藏，可通过 `include_archived=true` 或 `status=archived` 回看；place 与 lock action 已支持 lockdown/unlock/cancel response。`floors/:id` 已支持 detail/update/delete，`areas/:id` 已支持 detail/update；Floors 页已接 Area 创建/编辑，Place -> Floor -> Area -> Lock 写入闭环已补回归测试。`controllers/:token/assign`、`controllers/:id/deassign`、`readers/:token/assign`、`readers/:id/deassign`、controller-lock bind/unbind、controller config/reboot、reader reboot、terminal detail/reboot/trigger 已支持 reference-style route；Terminal 是由 Reader 派生的 command target，独立 update/delete 暂不落地，生命周期继续走 Reader assign/deassign。`users/:id` 已支持 detail/update/delete，并补 status change/delete 审计与 client helper。其中 `roles` 与 `role_assignments` 已成为新的权限层级落点，`role_assignments/:id` 已支持 detail/update/delete；历史 `user.role` 只保留为登录和迁移期 guard 的兼容字段。`shares/:id` 已支持 detail/update/delete，Access Rights 已支持 schedule template、批量 schedule edit、影响预览和批量 review。`teams/:id` 已支持 detail/update/delete，`team_memberships` 已支持 create/delete，Team 可作为 Role Assignment assignee。`reports` 已支持基于 alarms/audit/events 的聚合 list/detail/download，`scheduled_reports` 已支持 baseline CRUD。`cards/:id` 与 `card_assignments/:id` 已支持 detail 读取，`cards/:id/revoke` 已支持撤销；实体卡库存已补 `available` / `frozen` / `scrapped` 单张和批量状态治理，并阻止 `reserved` / `issued` 库存绕过 task lifecycle 直接改态。`group_links/verify` 已支持 secret / QR token 验证、写回 `last_used_at` / `claimed_at` 并追加 `reference_group_link_claimed` 审计。`integrations/:id` 已支持 detail/update/delete，`POST /integrations` 已支持 Identity Provider 与 HRIS wrapper，delete 语义为保存 inactive 状态。`alert_policies` 已补 create/detail/update/delete wrapper，支持内置 Enterprise Sync worker / Wallet job/DLQ 订阅和 `custom` category 的 trigger、severity、condition expression、channels、receiver groups baseline；`POST /alert_policies/condition_preview` 已支持 custom condition 语法校验与样例事件匹配预览，`POST /alert_policies/evaluate` 已支持对真实事件 payload 批量评估启用的 custom policies 并返回命中策略和通知路由，delete 语义为保存 inactive 状态。Reference collection 响应已在保留 `X-Collection-Range + {items}` 的同时补充 `{pagination:{offset,limit,total,has_more}}`，并实际应用 `limit` / `offset` 裁剪返回项；CORS 已暴露 `X-Collection-Range` 供浏览器 client 读取；`writeError` 已统一输出兼容旧字段的 `{error,message,code,status}` baseline。`GET /api/v1/openapi.json` 已提供 OpenAPI 3.0 baseline，覆盖 reference 资源、Mistyislet extension 分组、legacy compatibility 归档标记、operationId、collection pagination components 和统一 error schema。

---

## 1. 基准结论

### 1.1 OpenAPI 规则

| 项 | 规则 |
|---|---|
| Format | JSON only；请求应发送 `Accept: application/json` 与 `Content-Type: application/json` |
| Endpoint | 保留 `/api/v1` 前缀，资源路径使用复数，并尽量与 Bundled Reference 资源名一致 |
| Spec | `GET /api/v1/openapi.json` 输出 OpenAPI 3.0 baseline；reference 资源、Mistyislet extension 与 legacy compatibility 使用独立 tags / extension 标记 |
| Operation | 使用 `fetch*` / `create*` / `update*` / `delete*`，动作接口使用动词，例如 `unlockLock`、`lockDownPlace` |
| 字段 | JSON 与 query 使用 snake_case |
| 集合查询 | 使用 `ids`、`query`、`limit`、`offset`、资源 scope query，例如 `place_id`、`group_id`、`user_id` |
| 集合响应 | 目标形态为数组响应，并通过 `X-Collection-Range` 表达分页；迁移期 reference wrapper 返回 `{ items: [], pagination: { offset, limit, total, has_more } }`，并已支持 `limit` / `offset`；CORS 已 expose `X-Collection-Range` |
| 写入 payload | 使用资源包裹，例如 `{ "place": {} }`、`{ "group": {} }`、`{ "role_assignment": {} }` |
| ID 参数 | 参考 API 使用 `/resources/{id}`；Mistyislet 文档使用 `/resources/:id` 表达同一语义 |
| Error | 401/403/404/409/422 使用统一 `{ error, message, code, status }` schema；不要让页面模块返回自定义错误结构 |

### 1.2 Authentication

Bundled Reference 提供这些认证方式：

| Scheme | Header | Mistyislet 状态 |
|---|---|---|
| `Kisi-Login` | `Authorization: KISI-LOGIN <token>` | 参考模型 |
| `OAuth2` | `Authorization: Bearer <token>` | Mistyislet 当前 JWT 可继续兼容 Bearer |
| `Kisi-Access-Key` | `Authorization: KISI-ACCESS-KEY <key>` | 后续用于服务端/设备访问 |
| `Kisi-Group-Link` | `Authorization: KISI-GROUP-LINK <token>` | 后续用于 Access Link / Group Link |
| `Kisi-Service` | `Authorization: KISI-SERVICE <token>` | 后续用于内部服务 |
| `Webhook-Signature` | `X-Signature` | Webhook 验签参考 |

当前阶段保持 `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/me` 可用，并新增 `/api/v1/user` self profile alias/update；OpenAPI baseline 已描述 Bearer JWT 与参考 scheme 的兼容关系。

### 1.3 状态标记

| 状态 | 含义 |
|---|---|
| 已接 adapter | 前端新 UI 已使用现有旧 endpoint 转成资源视图 |
| 已落地 reference wrapper | 后端已提供 Bundled Reference 风格路径，内部可暂时复用旧数据模型 |
| 已落地 | 后端已有新资源状态或内置资源模型 |
| 旧接口可用 | 后端已有旧模块 endpoint，但名称或结构未对齐参考 API |
| 目标接口 | 按 Bundled Reference 需要新增或替换的 endpoint |
| 待定义 | 资源模型还需要补字段、权限或状态语义 |

### 1.4 当前状态快照

| 状态 | 范围 |
|---|---|
| 已完成 | Places / Floors / Areas / Locks、Controllers / Readers / Terminals 主要 reference wrapper，Terminal detail 与 command-only lifecycle 决策，Users detail/update/delete API、User Detail UI 保存/启停/删除/邀请历史、Users Add/Invite record creation、邀请投递 queue/receipt/audit/provider webhook baseline、Resend/mock provider dispatch 与批量启停，Groups / Group Locks / Group Zones / Group Links，Teams / Team Memberships，Roles / Role Assignments / Shares，Access Rights review/schedule template/bulk schedule baseline 与影响预览/批量 review，Cards / Card Assignments，实体卡库存状态治理 baseline，Events，Reports / Scheduled Reports baseline，Integrations detail/write，Alert Policies 内置订阅、custom policy、condition preview 与 event evaluate baseline，Place 归档语义，legacy gateway 高危写/命令审计 baseline，OpenAPI 3.0 baseline |
| 进行中 | 前端 adapter 收口；reference destructive/write audit baseline 已覆盖 Place/Lock/Group/Group Lock/Group Link/Controller/Reader/Team/Team Membership/Role Assignment/Share/Card/Alert Policy 等高风险操作，已扩展 Team create/update、Team Membership create、Role Assignment create/update、Share create/update、Card create/assign/status/deassign/revoke 回归；legacy gateway register/bind/unbind/device register/config publish/reboot 已补审计与回归；legacy building/door/door-group/temporary-access create 与 access policy create/update 已补审计与回归；非硬件 legacy 写操作全域审计已补齐：legacy user/user-group create/update、visitor pass create、alarm status update、tenant create/status、floor create/update/delete、area create/update、admin MFA setup/enable/disable、state change replay、wallet config/template/pass issue/status/delivery/physical card inventory/task/job/DLQ 全链路已补审计与回归 |
| 未完成 | Users 批量治理深化、Teams 高级治理、Access Rights schedule 复杂时间窗/节假日/例外规则、Reports 排程投递持久化/更多导出格式、Alert Policy 事件调度器/渠道升级策略/持久化投递、Apple Pass `.pkpass`/设备回调、制卡供应商真实 API、OpenAPI 资源 schema 细化 |

| 优先级 | API 事项 | 原因 |
|---|---|---|
| P0 | destructive action 确认流与审计：reference delete/deassign/revoke/disable/status change 已补 audit baseline，并已扩展 Team/Team Membership/Role Assignment/Share/Card 的关键 create/update/assign 写操作审计；Place delete 已落归档语义，legacy gateway 高危写/命令、legacy building/door/door-group/temporary-access create 和 legacy access policy create/update 已补 audit baseline；非硬件 legacy 写操作全域审计已完成：user/user-group/visitor-pass/alarm/tenant/floor/area/MFA/state-replay/wallet 全链路已补 | 降低误操作和追踪缺口 |
| P0 | Place Admin scope guard；后端 `building_admin` scope 已可从 Role Assignment / Team Membership 推导，API-level URL 回归已覆盖未授权 Place/Lock/Share/Role Assignment/Access Rights impact-review-schedule，Operator 只读写保护 smoke 已覆盖 Team/Card/Role Assignment/Share/Alert Policy 写入拒绝，浏览器 URL 绕过 E2E 已覆盖 unassigned Place direct route | 防止只靠前端隐藏入口造成越权 |
| P1 | Users 批量治理深化 | User Detail、目录创建、邀请记录创建、邀请 queue/receipt/audit、Resend/mock provider dispatch、签名 provider webhook、邀请历史视图和批量启停已接，下一步补更细的批量治理 |
| P1 | Alert Policy 调度器与渠道升级、Access Rights schedule 高级语义 | 补齐运营策略表达能力 |
| P2 | OpenAPI 资源 schema 细化与生成链路 | operationId、collection pagination、统一 error schema、extension 分组和 legacy archive baseline 已接，下一步补资源字段级 schema |

### 1.5 前后端 / API 文档对照进度

本节按 2026-04-28 的实际后端 route、`web-admin/src/lib/api.ts` client helper 和本文件目标 API 对照。结论是：主资源已基本对齐 reference-style 路径，剩余主要集中在迁移期格式规范和高级业务语义。

#### 已完成

| 范围 | 后端状态 | 前端状态 | 文档状态 |
|---|---|---|---|
| Places / Floors / Areas / Locks | `/places`、`/floors`、`/areas`、`/locks` 已覆盖 list/detail/create/update/delete 和 lock/place action；Place delete 已改归档 | Places、Place Settings、Floors、Door Detail 已接写入与确认流 | 3.1-3.3 已列为已落地 |
| Hardware reference wrapper | `/controllers`、`/readers`、`/terminals` list/detail/action、assign/deassign、controller-lock bind/unbind、config/reboot/trigger 已落地 | Hardware 已接 Add Hardware、绑定、移除、命令确认；Terminal 独立 update/delete 明确不落地 | 3.4 已列为已落地，Terminal lifecycle 决策已记录 |
| Users / Members | `/users?place_id=`、`/members`、user detail/update/delete/invite/invitations/receipt/provider-receipts 已落地，含 Place Admin scope 与 audit | Users Add/Invite、批量 Suspend/Enable、User Detail 保存/启停/删除/邀请历史已接 | 3.5 已列为已落地 |
| My Account self profile | `/user` 已支持 GET/PATCH，`/me` 保持兼容；PATCH 仅允许 `name` / `language`，并写入 auth profile audit | My Account Profile 已接 name/language 保存并同步当前 session viewer | 3.5 Current user 已列为已落地 |
| Teams / Groups / Links | `/teams`、`/team_memberships`、`/groups`、`/group_locks`、`/group_zones`、`/group_links` 与 token verify 已落地 | Teams 与 Groups 已接创建、保存、删除、成员/门点/链接写入 | 3.6-3.7 已列为已落地 |
| Roles / Access Rights / Shares | `/roles`、`/role_assignments`、`/shares`、`/access_rights/schedule_templates`、`/access_rights/schedule`、`/access_rights/impact_preview`、`/access_rights/review` 已落地 | Access Rights 已接 Add/Edit/Delete、schedule template、批量 schedule edit、target/schedule 过滤、影响预览、批量 review | 3.8-3.9 已更新为已覆盖 |
| Credentials baseline | `/cards`、`/card_assignments`、card actions、wallet issue/delivery/jobs、Apple Pass self-service enrollment、physical card vendor/inventory/task baseline 与库存状态治理已落地；`cards` 已补 `credential_kind` / `save_link` | Credentials 已接 issue、batch audit、assign/deactivate/revoke、delivery、Apple Pass resident enrollment、实体卡入库/任务、库存 status 单张/批量治理控件，并区分 Google Wallet、Apple Wallet、Physical Card 与 Access Link | 3.10 已列为已落地，Apple `.pkpass`/设备回调仍进行中 |
| Reports baseline | `/reports`、`/reports/:id`、`/reports/:id/download`、`/scheduled_reports` CRUD 已落地，Place Admin scope 已限制 | Reports 页已切真实 report/scheduled report API，并接 CSV 下载 | 3.11 已更新为已覆盖 |
| Integrations / Alert Policies baseline | `/integrations` 与 `/alert_policies` list/detail/create/update/delete 已落地，Alert Policy 已支持 `custom` category trigger/severity/condition/channels/receivers baseline 和 condition preview，delete 为 soft-disable | Organization Setup 已接 integration detail/write/disable 与 alert policy Add/Save/Delete/custom condition/Preview | 3.12-3.13 已列为已落地 |
| Scope guard / URL bypass | Place Admin scope 可从 JWT、Role Assignment、Team Membership 推导，API-level 和浏览器 URL 绕过回归已补 | Place scoped routes 已走后端 guard，不只靠隐藏入口 | 1.4 P0 已记录 |
| OpenAPI baseline | `GET /api/v1/openapi.json` 已输出 reference resources、Mistyislet extensions、legacy compatibility 分组，operationId、pagination components、统一 error schema 与认证 schemes 已接 | 暂未接文档站或生成 TS client | 1.1 已列为 baseline |

#### 进行中 / 格式未完全匹配

| 差异 | 当前实际 | 目标格式 | 影响 / 处理 |
|---|---|---|---|
| Collection response | reference wrapper 已加 `X-Collection-Range`，响应体为 `{ "items": [...], "pagination": { "offset": n, "limit": n, "total": n, "has_more": bool } }`，并按 `limit` / `offset` 裁剪 `items`；CORS 已暴露 `X-Collection-Range`；OpenAPI 已补 `CollectionResponse`、`Pagination`、`LimitParam`、`OffsetParam` 和 response header schema | 最终可继续收敛为数组响应 + `X-Collection-Range`，或明确统一 `{items,pagination}` schema | 前端 `requestItems` 已兼容数组和 `{items}`，`normalizeOffsetListResponse` 已兼容嵌套 `pagination` 与旧顶层 offset 响应；字段级资源 schema 后续细化 |
| Operation 命名 | OpenAPI baseline 已使用 `fetch*` / `create*` / `update*` / `delete*` 和动作动词 operationId；TS client helper 大量使用 `list*`，例如 `listPlaces`、`listRoles`、`listAlertPolicies` | 生成 client 时以 OpenAPI operationId 为准 | 运行不受影响；后续如生成 client 再统一 helper 命名 |
| Error schema | `writeError` 已统一返回 `{ "error": "...", "message": "...", "code": "...", "status": "422" }`，保留全字符串字段兼容旧 consumer；OpenAPI 已补 `Error` schema 与 401/403/404/409/422 response components；前端 `APIError` 已保留 `code` / `responseStatus`；部分批处理 route 仍带额外业务字段 | 统一 401/403/404/409/422 error schema 并保留业务扩展字段 | API consumer 可用；生成 client 前需继续补业务扩展字段 schema |
| Legacy endpoint coexistence | 后端仍保留 `/buildings`、`/doors` 兼容入口，并对 `/buildings`、`/doors`、旧 `/door-groups`、`/gateways`、`/events/access`、`/wallet/passes`、`/access-policies`、`/temporary-access` 返回 `Deprecation`、`Link`、`X-MistyPass-Replacement` header；前端旧命名 helper `listBuildings` / `createBuilding` 已转接 reference `/places`，`listDoors` / `createDoor` 已转接 `/locks`，`listTemporaryAccess` / `createTemporaryAccess` 已转接 `/shares`，`listAccessEvents` 已转接 `/event_sets`，`listWalletPasses` 已转接 `/cards`，并映射回旧 UI view model；`listUserGroups` / `createUserGroup` / `updateUserGroup` 旧函数名已改走 reference `/groups`；`listDoorGroups` 已改走 snake_case extension `/door_groups`，旧 `/door-groups` 保持兼容；`listGateways` 旧函数名已改为先组合 `/controllers`、`/readers`、`/terminals` 成旧 `Gateway[]` view model，仅在 reference 硬件读取失败时才回退 `/gateways`；`bindGatewayDoor`、`unbindGatewayDoor`、`publishGatewayConfig`、`rebootGateway` 旧函数名已 reference-first 转接 controller lock/config/reboot action，失败时才回退 legacy gateway action；`/access-policies` 已从新 shell summary 的无条件并发调用降级为 reference endpoint 失败后的按需兜底 | 新 UI 主路径使用 `/places`、`/locks`、`/controllers`、`/readers`、`/terminals`、`/groups`、`/shares`、`/cards`、`/event_sets` 等 reference-style endpoint | 新 shell 主资源已优先 reference；后端 legacy 兼容入口进入可观测归档阶段，fallback archive 仍在进行 |
| Product-specific extension | `/door_groups`、`/access_rights/schedule_templates`、`/access_rights/schedule`、`/access_rights/impact_preview`、`/access_rights/review`、Google Wallet config/templates/passes/jobs/deliveries、physical-card endpoints 属于 Mistyislet 扩展或运营后台能力 | OpenAPI 已通过 `x-mistyislet-extension` 与 `x-mistyislet-extension-group` 标注，避免和 Bundled Reference 原生资源混淆 | 功能已可用；后续文档站可直接按 extension group 分栏 |

#### 未完成 / 待补接口

| 范围 | 缺口 | 当前替代 | 优先级 |
|---|---|---|---:|
| Reports advanced | 排程投递持久化、更多导出格式、报表模板参数化 | 当前已有 `/reports` 聚合、CSV download 与 `/scheduled_reports` baseline CRUD | P2 |
| Access Rights schedule | 复杂时间窗、节假日/例外规则 | 当前已有 `valid_from` / `valid_until` baseline、schedule template baseline、批量 schedule edit 与 review/impact preview | P1 |
| Teams governance | SCIM source diff、成员批量导入、team access review | 当前支持 team/membership CRUD 与 role assignment flow | P1 |
| Users governance | 目录差异审阅、批量 role/group 变更细化 | `batch-status`、`batch-delete`、`batch-invite`、`export-csv`、`import-csv` 已落地；前端已接真正的 batch endpoint 替换 Promise.all 并发，CSV 导出/导入按钮已接 | P2 |
| Alert Policy advanced execution | 渠道升级策略、通知持久化到 DB | 事件驱动调度器已落地：event subscriber 自动评估 enabled custom policies 并 dispatch 通知，cooldown 机制防止重复通知，`GET /alert_policies/notifications` 已支持 tenant/policy/severity/status 过滤；剩余渠道升级策略和 DB 级持久化 | P2 |
| Apple Pass production protocol | `.pkpass` 签名、pass type identifier / certificate 管理、device registration、push update callback | 当前已有 resident 自助 enrollment 记录与 Admin 管理 baseline，尚未生成真实 Apple Wallet pass bundle | P1 |
| Physical card supplier | 真实制卡供应商 API、完整库存运营视图和前端治理控件 | 当前有 vendor/inventory/task baseline、CSV import，以及 `available` / `frozen` / `scrapped` 后端生命周期治理；`reserved` / `issued` 仍必须通过实体卡 task lifecycle 推进 | P1 |
| Digital access link governance | Claim 后写回与基础审计已落地；复杂过期策略、访问链接审计运营视图与 QR code 运营视图待深化 | 当前有 `/group_links`、`/group_links/verify`、`/shares` 和 public claim UI baseline；verify 成功会写 `last_used_at` / `claimed_at` 并追加 `reference_group_link_claimed` audit | P2 |
| OpenAPI | 资源字段级 schema、request body 精细类型、生成 client / 文档站集成 | 当前已有 `/api/v1/openapi.json` baseline，覆盖 operationId、分页、统一 error schema、extension 分组、legacy endpoint archive | P2 |

---

## 2. 权限层级

### 2.1 产品角色

新 UI 只暴露两类管理人员：

| 产品角色 | 默认视角 | 说明 |
|---|---|---|
| Organization Admin | Organization | 管理组织级用户、Places、权限、集成、报表 |
| Place Admin | Place | 管理或查看被分配 Place 的门点、人员、硬件、事件 |

后端历史枚举只作为迁移期实现映射存在，不再作为产品权限层级对外说明。

### 2.2 API 权限模型

Bundled Reference 的权限模型不是单一 `user.role`，而是以下层级：

| 层级 | API 资源 | 关键字段 | 说明 |
|---|---|---|---|
| Role | `/roles` | `id`、`applies_to`、`permissions` | 权限能力包，`applies_to` 只能是 `Organization`、`Place`、`Group` |
| Role Assignment | `/role_assignments` | `role_id`、`applies_to_type/id`、`assignee_type/id` | 把一个 Role 分配到 Organization、Place 或 Group 上 |
| Assignee | `User` / `Team` / `Guest` | `assignee_type`、`assignee_id` | 权限受让者可以是用户、团队或访客 |
| Share | `/shares` | `email`、`group_id`、`role_id`、`valid_from/until` | 通过邮件向 Group 分享访问权限，可用于 Access Link 流程 |
| Team | `/teams` + `/team_memberships` | `team_id`、`member_type/id` | 管理成员集合，便于批量分配 role assignment |
| Group | `/groups` + `group_*` | `place_id`、restrictions | 访问组，绑定 locks/zones/elevator stops/terminals 与限制条件 |

### 2.3 Mistyislet 映射规则

| 产品概念 | 目标 API 表达 |
|---|---|
| Organization Admin | `role_assignment.applies_to_type = Organization`，`role_id` 指向具备组织级管理权限的 Role |
| Place Admin | `role_assignment.applies_to_type = Place`，`role_id` 指向具备 Place 级管理权限的 Role |
| Access Rights 页面 | 聚合展示 `/role_assignments`、`/shares`、`/groups`、`/group_locks`、`/team_memberships` |
| Place Users 页面 | 优先使用 `/users?place_id=` 或 `/members?place_id=`，不能只靠前端过滤 |
| 临时访问 / 分享访问 | 使用 `/shares` 或 `/group_links`，不再新增自造 `/access_rights/share` |
| 门点权限 | 使用 `/groups` + `/group_locks`，UI 可显示为 Doors |

---

## 3. 核心资源 API

### 3.1 Places

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchPlaces` | GET | `/api/v1/places` | `/api/v1/buildings` | Organization Admin | 已落地 reference wrapper，前端已切 | Places |
| `fetchPlace` | GET | `/api/v1/places/:id` | Space service reference wrapper | Organization Admin / Place Admin | 已落地 reference wrapper，client 已补 | Place Dashboard |
| `createPlace` | POST | `/api/v1/places` | `/api/v1/buildings` | Organization Admin | 已落地 reference wrapper，Places 页已接 Create Place Sheet | Create Place |
| `updatePlace` | PATCH | `/api/v1/places/:id` | Space service reference wrapper | Organization Admin / Place Admin | 已落地 reference wrapper，Place Settings 已接 General 保存 | Place Settings |
| `deletePlace` | DELETE | `/api/v1/places/:id` | Space service reference wrapper | Organization Admin | 已落地 reference wrapper、归档语义与回归测试；默认隐藏 archived Place，`include_archived=true` / `status=archived` 可回看 | Places |
| `lockDownPlace` | POST | `/api/v1/places/:id/lock_down` | reference action wrapper | Organization Admin / Place Admin | 已落地 action response，Place Settings 已接 | Place Settings |
| `cancelPlaceLockdown` | POST | `/api/v1/places/:id/cancel_lockdown` | reference action wrapper | Organization Admin / Place Admin | 已落地 action response，Place Settings 已接 | Place Settings |

### 3.2 Floors

参考 API 使用 `/floors?place_id=`，不是嵌套 `/places/:place_id/floors`。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchFloors` | GET | `/api/v1/floors?place_id=:place_id` | `/api/v1/floors` + `/api/v1/areas` | Organization Admin / Place Admin | 已支持 `place_id` 查询 | Floors |
| `fetchFloor` | GET | `/api/v1/floors/:id` | Space service endpoint | Organization Admin / Place Admin | 已落地 detail endpoint 与 client | Floors |
| `createFloor` | POST | `/api/v1/floors` | `/api/v1/floors` | Organization Admin / Place Admin | 已落地，Floors 页已接 Add Floor Sheet | Floors |
| `updateFloor` | PATCH | `/api/v1/floors/:id` | Space service endpoint | Organization Admin / Place Admin | 已落地，Floors 页 General 已接 Save | Floors |
| `deleteFloor` | DELETE | `/api/v1/floors/:id` | Space service endpoint | Organization Admin / Place Admin | 已落地 endpoint 与回归测试，Floors UI 已接确认删除 | Floors |
| `fetchArea` | GET | `/api/v1/areas/:id` | Space service endpoint | Organization Admin / Place Admin | 已落地 detail endpoint | Floors |
| `createArea` | POST | `/api/v1/areas` | Space service endpoint | Organization Admin / Place Admin | 已落地，Floors 页 Areas 已接 Add Area Sheet | Floors |
| `updateArea` | PATCH | `/api/v1/areas/:id` | Space service endpoint | Organization Admin / Place Admin | 已落地，Floors 页 Areas 已接 Save Area；移动 Area 会同步门点拓扑 | Floors |

### 3.3 Locks（UI Doors）

Kisi API 的门点资源名是 `locks`。Mistyislet UI 可以继续显示 Doors，但目标 API 应使用 `/locks`。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchLocks` | GET | `/api/v1/locks?place_id=:place_id` | `/api/v1/doors` | Organization Admin / Place Admin | 已落地 reference wrapper | Doors |
| `fetchLock` | GET | `/api/v1/locks/:id` | Space service reference wrapper | Organization Admin / Place Admin | 已落地 reference wrapper，Door Detail 已接 detail query | Door Detail |
| `createLock` | POST | `/api/v1/locks` | `/api/v1/doors` | Organization Admin / Place Admin | 已落地 reference wrapper 与 client，Door Detail 已接 Add Door Sheet | Doors |
| `updateLock` | PATCH | `/api/v1/locks/:id` | Space service reference wrapper | Organization Admin / Place Admin | 已落地 reference wrapper，Door Detail General 已接 Save | Door Detail |
| `deleteLock` | DELETE | `/api/v1/locks/:id` | Space service reference wrapper | Organization Admin / Place Admin | 已落地 reference wrapper，Door Detail 已接 Delete Door | Doors |
| `unlockLock` | POST | `/api/v1/locks/:id/unlock` | reference action wrapper | Organization Admin / Place Admin | 已落地 action response，Door Detail 已接 | Door Detail |
| `lockDownLock` | POST | `/api/v1/locks/:id/lock_down` | reference action wrapper | Organization Admin / Place Admin | 已落地 action response，Door Detail 已接 | Door Detail |
| `cancelLockLockdown` | POST | `/api/v1/locks/:id/cancel_lockdown` | reference action wrapper | Organization Admin / Place Admin | 已落地 action response，Door Detail 已接 | Door Detail |

### 3.4 Hardware

参考 API 将硬件拆成 controllers、readers、terminals、wireless_locks 等资源。Mistyislet 当前 `gateways` 仍是实现层聚合。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchControllers` | GET | `/api/v1/controllers` | `/api/v1/gateways` wrapper | Organization Admin / Place Admin | 已落地只读 reference wrapper，前端已切 | Hardware |
| `fetchReaders` | GET | `/api/v1/readers` | `/api/v1/gateways` devices wrapper | Organization Admin / Place Admin | 已落地只读 reference wrapper，前端已切 | Hardware |
| `fetchTerminals` | GET | `/api/v1/terminals?place_id=` | `/api/v1/gateways` devices wrapper | Organization Admin / Place Admin | 已落地只读 reference wrapper，前端已切 | Hardware |
| `fetchTerminal` | GET | `/api/v1/terminals/:id` | `/api/v1/gateways` device wrapper | Organization Admin / Place Admin | 已落地 detail wrapper 与 client；Terminal 独立 update/delete 暂不落地 | Hardware |
| `assignController` | POST | `/api/v1/controllers/:token/assign` | `/api/v1/gateways/register` wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Add Hardware 已接 Controller 注册并按需导入 serial inventory | Hardware |
| `assignReader` | POST | `/api/v1/readers/:token/assign` | `/api/v1/gateways/:gatewayID/devices` wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Add Hardware 已接 Reader 挂接并按需导入 serial inventory | Hardware |
| `bindControllerLock` | POST/DELETE | `/api/v1/controllers/:id/locks`、`/api/v1/controllers/:id/locks/:lock_id` | `/api/v1/gateways/:gatewayID/bind-door`、`/unbind-door` wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Doors 面板已接 Bind/Remove | Hardware |
| `deassignController` | POST | `/api/v1/controllers/:id/deassign` | gateway deassign wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Settings 已接 Deassign | Hardware |
| `deassignReader` | POST | `/api/v1/readers/:id/deassign` | gateway device deassign wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Settings 已接 Reader Deassign | Hardware |
| `publishControllerConfig` | POST | `/api/v1/controllers/:id/config/publish` | `/api/v1/gateways/:gatewayID/config/publish` wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Settings 已接确认流并固定命令目标 | Hardware |
| `rebootController` | POST | `/api/v1/controllers/:id/reboot` | `/api/v1/gateways/:gatewayID/reboot` wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Settings 已接确认流并固定命令目标 | Hardware |
| `rebootReader` | POST | `/api/v1/readers/:id/reboot` | 所属 controller reboot wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Settings 对 Reader 已接确认流并固定命令目标 | Hardware |
| `rebootTerminal` | POST | `/api/v1/terminals/:id/reboot` | 所属 controller reboot wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Settings 对 Terminal 已接确认流并固定命令目标 | Hardware |
| `triggerTerminal` | POST | `/api/v1/terminals/:id/trigger` | terminal trigger wrapper | Organization Admin / Place Admin | 已落地 reference-style route 与 client，Hardware Settings 对 Terminal 已接 Trigger 确认流并固定命令目标 | Hardware |

### 3.5 Users And Members

`users` 是组织用户资源；`members` 更接近“某个 Place/Group 下的可访问成员视图”。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchUsers` | GET | `/api/v1/users` | `/api/v1/users` | Organization Admin / Place Admin | 已接 adapter | Users |
| `fetchPlaceUsers` | GET | `/api/v1/users?place_id=:place_id` | `/api/v1/users` + 前端过滤 `building_id` | Place Admin | 已支持 `place_id` 查询 | Place Users |
| `fetchMembers` | GET | `/api/v1/members?place_id=:place_id` | `/api/v1/users` | Organization Admin / Place Admin | 已落地 reference wrapper | Place Users / Groups |
| `fetchUser` | GET | `/api/v1/users/:id` | Access service endpoint | Organization Admin / Place Admin | 已落地 endpoint 与 client helper | User Detail |
| `createUser` | POST | `/api/v1/users` | `/api/v1/users` | Organization Admin / Place Admin | 目标路径一致；client helper、Users Add User/Invite sheet 已接 | Users |
| `sendUserInvitation` | POST | `/api/v1/users/:id/invite` | Access service delivery wrapper | Organization Admin / Place Admin | 已落地 queued delivery response、Resend/mock provider dispatch、Place Admin scope guard、`reference_user_invitation_sent` audit 与 client helper | Users / User Detail |
| `fetchUserInvitations` | GET | `/api/v1/users/:id/invitations` | Access service delivery wrapper | Organization Admin / Place Admin | 已落地 invitation delivery list、Place Admin scope guard 与 client helper | User Detail |
| `recordUserInvitationReceipt` | POST | `/api/v1/users/:id/invitations/:delivery_id/receipt` | Access service delivery wrapper | Organization Admin / Place Admin | 已落地 sent/failed receipt、provider metadata、`reference_user_invitation_receipt` audit 与 client helper | Invitation Provider |
| `receiveUserInvitationProviderReceipt` | POST | `/api/v1/users/invitations/provider-receipts` | Access service signed webhook wrapper | Invitation Provider | 已落地 HMAC/secret protected provider webhook，支持 sent/failed/bounced receipt、provider error/retryable metadata 与 system audit | Invitation Provider |
| `updateUser` | PATCH | `/api/v1/users/:id` | Access service endpoint | Organization Admin / Place Admin | 已落地 endpoint、client helper 与 status change audit | User Detail |
| `deleteUser` | DELETE | `/api/v1/users/:id` | Access service endpoint | Organization Admin | 已落地 endpoint、client helper、关联 Role Assignment/Team Membership/Group member 清理与 delete audit | Users |
| `fetchCurrentUser` | GET | `/api/v1/user` | `/api/v1/user` + `/api/v1/me` 兼容 | Authenticated | 已落地 self profile alias，旧 `/me` 保持可用 | My Account |
| `updateCurrentUser` | PATCH | `/api/v1/user` | `/api/v1/user` | Authenticated | 已落地，仅允许 `name` / `language` 自助更新并写 audit | My Account |
| `batchUpdateUserStatus` | POST | `/api/v1/users/batch-status` | Access service batch | Organization Admin / Place Admin | 已落地，前端已接真正的 batch endpoint 替换 Promise.all 并发 | Users |
| `batchDeleteUsers` | POST | `/api/v1/users/batch-delete` | Access service batch | Organization Admin | 已落地，前端已接批量删除确认 | Users |
| `batchInviteUsers` | POST | `/api/v1/users/batch-invite` | Access service batch | Organization Admin / Place Admin | 已落地，前端已接批量邀请 | Users |
| `exportUsersCSV` | GET | `/api/v1/users/export-csv` | Access service CSV export | Organization Admin / Place Admin | 已落地，前端已接 CSV 下载按钮 | Users |
| `importUsersCSV` | POST | `/api/v1/users/import-csv` | Access service CSV import | Organization Admin | 已落地，前端已接文件选择导入 | Users |

### 3.6 Teams

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchTeams` | GET | `/api/v1/teams?scope=organization|place` | 新 `teams` state | Organization Admin / Place Admin | 已落地只读 | Teams |
| `fetchTeam` | GET | `/api/v1/teams/:id` | 新 `teams` state | Organization Admin / Place Admin | 已落地 reference wrapper | Team Detail |
| `createTeam` | POST | `/api/v1/teams` | 新 `teams` state | Organization Admin | 已落地 reference wrapper，Teams 页已接 New Team | Teams |
| `updateTeam` | PATCH | `/api/v1/teams/:id` | 新 `teams` state | Organization Admin | 已落地 reference wrapper，Teams 页 General Save 已接 | Team Detail |
| `deleteTeam` | DELETE | `/api/v1/teams/:id` | 新 `teams` state cleanup wrapper | Organization Admin | 已落地 reference wrapper，Teams 页已接 Delete Team | Teams |
| `fetchTeamMemberships` | GET | `/api/v1/team_memberships?team_id=` | 新 `team_memberships` state | Organization Admin / Place Admin | 已落地只读 | Team Detail |
| `createTeamMembership` | POST | `/api/v1/team_memberships` | 新 `team_memberships` state | Organization Admin | 已落地 reference wrapper，Teams 页已接 Add Member | Team Detail |
| `deleteTeamMembership` | DELETE | `/api/v1/team_memberships/:id` | 新 `team_memberships` state | Organization Admin | 已落地 reference wrapper，Teams 页已接 Remove | Team Detail |

### 3.7 Groups And Access Resources

Groups 是访问控制核心：一个 Group 通过 relation resources 绑定 locks/zones/elevator stops/terminals，并带限制条件。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchGroups` | GET | `/api/v1/groups?scope=&place_id=` | 旧 `/api/v1/user-groups` helper 已迁到 `/groups`；Door group target 走 MistyPass extension `/api/v1/door_groups` + `/api/v1/group_locks` | Organization Admin / Place Admin | 已落地 reference wrapper | Groups / Place Groups |
| `fetchGroup` | GET | `/api/v1/groups/:id` | 旧 `/api/v1/user-groups` lookup wrapper 已迁到 `/groups/:id` | Organization Admin / Place Admin | 已落地 reference wrapper | Group Detail |
| `createGroup` | POST | `/api/v1/groups` | 旧 `createUserGroup` helper 已改走 `/groups`；Door group target 创建仍待 reference 化 | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页已接 Add Group | Groups |
| `updateGroup` | PATCH | `/api/v1/groups/:id` | 旧 `updateUserGroup` helper 已改走 `/groups/:id` | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页 General 与 Restrictions 已接 Save | Group Detail |
| `deleteGroup` | DELETE | `/api/v1/groups/:id` | user group state cleanup wrapper | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页已接 Delete Group | Groups |
| `fetchGroupLocks` | GET | `/api/v1/group_locks?group_id=` | Door group target metadata 走 `/api/v1/door_groups` extension | Organization Admin / Place Admin | 已落地 reference wrapper | Group Detail |
| `createGroupLock` | POST | `/api/v1/group_locks` | Door group membership 写入继续复用 DoorGroup state | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页已接 Add Doors | Group Detail |
| `deleteGroupLock` | DELETE | `/api/v1/group_locks/:id` | Door group membership 写入继续复用 DoorGroup state | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页已接 Remove | Group Detail |
| `fetchGroupZones` | GET | `/api/v1/group_zones?group_id=` | Door group target metadata 走 `/api/v1/door_groups` extension + `/api/v1/areas` wrapper | Organization Admin / Place Admin | 已落地只读 reference wrapper，Groups 页已接 Zones 面板 | Groups |
| `fetchGroupLinks` | GET | `/api/v1/group_links?group_id=` | 新 `group_links` state | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页已接 Links 面板 | Access Links |
| `fetchGroupLink` | GET | `/api/v1/group_links/:id` | 新 `group_links` state | Organization Admin / Place Admin | 已落地 reference wrapper | Access Link Detail |
| `createGroupLink` | POST | `/api/v1/group_links` | 新 `group_links` state | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页已接 Add Link | Access Links |
| `updateGroupLink` | PATCH | `/api/v1/group_links/:id` | 新 `group_links` state | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页已接 Edit Link | Access Link Detail |
| `deleteGroupLink` | DELETE | `/api/v1/group_links/:id` | 新 `group_links` state | Organization Admin / Place Admin | 已落地 reference wrapper，Groups 页已接 Delete Link | Access Links |
| `verifyGroupLinkToken` | GET/POST | `/api/v1/group_links/verify` | 新 `group_links` state | Public token flow | 已落地 secret / QR token 验证、有效期检查、`last_used_at` / `claimed_at` 写回和 `reference_group_link_claimed` 审计；前端 `/access-link` claim UI 已接 claimed time 展示 | Access Link Claim |

### 3.8 Roles And Role Assignments

这一组替代之前自造的 `/access_rights` 目标模型。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchRoles` | GET | `/api/v1/roles` | 内置角色种子 | Organization Admin / Place Admin | 已落地 | Access Rights / Settings |
| `fetchRole` | GET | `/api/v1/roles/:id` | 内置角色种子 | Organization Admin / Place Admin | 已落地 | Role Detail |
| `fetchRoleAssignments` | GET | `/api/v1/role_assignments?applies_to_type=&applies_to_id=` | 新 `role_assignments` state | Organization Admin / Place Admin | 已落地 | Access Rights |
| `createRoleAssignment` | POST | `/api/v1/role_assignments` | 新 `role_assignments` state | Organization Admin / Place Admin | 已落地 | Share Access Flow |
| `fetchRoleAssignment` | GET | `/api/v1/role_assignments/:id` | 新 `role_assignments` state | Organization Admin / Place Admin | 已落地，Access Rights edit sheet 已接 detail | Access Rights |
| `updateRoleAssignment` | PATCH | `/api/v1/role_assignments/:id` | 新 `role_assignments` state | Organization Admin / Place Admin | 已落地，Access Rights edit sheet 已接保存 | Access Rights |
| `deleteRoleAssignment` | DELETE | `/api/v1/role_assignments/:id` | 新 `role_assignments` state | Organization Admin / Place Admin | 已落地 | Access Rights |
| `fetchAccessRightsScheduleTemplates` | GET | `/api/v1/access_rights/schedule_templates` | 内置 schedule template 计算 | Organization Admin / Place Admin / Operator | 已落地，Access Rights create/edit sheet 已接模板套用 | Access Rights |
| `updateAccessRightsSchedule` | PATCH | `/api/v1/access_rights/schedule` | `role_assignments` + `shares` bulk schedule updater | Organization Admin / Place Admin | 已落地，Access Rights selected rows 已接批量 Edit schedule | Access Rights |
| `previewAccessRightsImpact` | POST | `/api/v1/access_rights/impact_preview` | `role_assignments` + `shares` impact aggregator | Organization Admin / Place Admin / Operator | 已落地，Access Rights bulk preview 已接 | Access Rights |
| `reviewAccessRights` | POST | `/api/v1/access_rights/review` | `role_assignments.reviewed_*` + `shares.reviewed_*` | Organization Admin / Place Admin | 已落地，批量 Mark reviewed 已接 audit | Access Rights |

### 3.9 Shares

Shares 覆盖“给某个邮箱分享访问权限”和临时访问流，替代 `/access_rights/share`。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchShares` | GET | `/api/v1/shares?group_id=&place_id=&area_id=&user_id=&role_id=` | `/api/v1/temporary-access` | Organization Admin / Place Admin | 已落地 reference wrapper；`listTemporaryAccess` 旧 helper 已转接并映射回旧 UI view model | Access Rights / Access Links |
| `createShare` | POST | `/api/v1/shares` | `/api/v1/temporary-access` | Organization Admin / Place Admin | 已落地 reference wrapper，`valid_from` / `valid_until` 已贯通，并保留 `area_id`、`grantee_phone`、`mobile_model`、`pass_type`、`authorized_at`；`createTemporaryAccess` 旧 helper 已转接 | Share Access Flow |
| `fetchShare` | GET | `/api/v1/shares/:id` | `/api/v1/temporary-access` | Organization Admin / Place Admin | 已落地 reference wrapper，Access Rights edit sheet 已接 detail | Access Link Detail |
| `updateShare` | PATCH | `/api/v1/shares/:id` | `/api/v1/temporary-access` | Organization Admin / Place Admin | 已落地 reference wrapper，Access Rights edit sheet 已接保存 | Access Link Detail |
| `deleteShare` | DELETE | `/api/v1/shares/:id` | `/api/v1/temporary-access` | Organization Admin / Place Admin | 已落地 reference wrapper | Access Rights |

### 3.10 Credentials

参考 API 中长期实体凭证更接近 `cards` 与 `card_assignments`；临时链接更接近 `group_links` 或 `shares`。对照 Kisi Credentials 文档后，发放语义需要按凭证类型拆开，不能把 Apple Wallet、Google Wallet、实体卡和访问链接统一理解成同一个“发卡”动作。

核对来源：Kisi Dashboard Credentials 入口 `https://docs.kisi.io/dashboard/credentials/` 及其 Apple Passes、Physical Credentials、Access Links / QR Code 子页面。结论是 Apple Passes 偏用户自助 enrollment + Admin 管理，实体卡偏库存/读卡器/导入/分配工作流，Access Links / QR Code 偏临时访问链接；Google Wallet 未出现在该 Credentials 分类中，因此继续作为 MistyPass wallet extension 标注。

| 凭证类型 | Kisi 文档语义 | MistyPass API 对齐 | 当前判断 |
|---|---|---|---|
| Apple Wallet / Apple Passes | 用户在 Kisi app 内自助添加 Apple Pass；Admin 主要查看、暂停、删除/移除已有 pass | `/api/v1/app/credentials/apple-pass` 已支持 resident 自助 enrollment；Admin 通过 `/cards?credential_kind=apple_wallet` 查看，并复用 `deactivate/revoke` 管理既有 pass；`POST /cards` 已禁止 Admin 直接 issue Apple Wallet | 已完成 baseline：后续补真实 `.pkpass` 签名/设备回调 |
| Google Wallet | Kisi dashboard credentials 文档未把 Google Wallet 列为原生 Credentials 子类；当前属于 MistyPass product extension | `/wallet/google/config`、`/wallet/templates`、`/wallet/passes/issue`、`/issue-batch`、`/save-link`、delivery、job/DLQ 已覆盖发放、保存链接、通知和批量审计；`/cards` wrapper 已输出 `credential_kind=google_wallet` 与 `save_link` | 已完成 baseline：OpenAPI 已标注为 Mistyislet extension |
| Physical card / third-party HF / DESFire | Admin 管理实体卡库存、分配、启用、挂失/丢失处理；发放依赖 UID/card number/vendor/库存任务 | `/cards` 可注册实体卡 UID/card_number/token，`card_assignments` 可绑定用户；`/wallet/physical-card-vendors`、`/physical-card-inventory`、`/physical-card-inventory/scan`、`/physical-card-inventory/batch-status`、`/physical-card-inventory/:inventoryID/status`、`/physical-card-tasks` 已覆盖供应商、库存、读卡器扫描入库、CSV import、`available/frozen/scrapped` 生命周期治理和 issue/reissue/loss lifecycle；Credentials detail 已接库存单张/批量治理控件；`/cards` wrapper 已输出 `credential_kind=physical_card` | 已完成 baseline：真实供应商 API 和完整运营视图仍待补 |
| Digital / virtual access credential | 更接近 link/QR/share，不是长期实体卡；可面向访客或临时访问 | `/group_links`、`/group_links/verify`、`/shares` 与 public `/access-link` claim UI 已覆盖 token/QR 验证、有效期、claim timestamp 和基础审计 | 已完成 baseline：复杂过期策略和更完整审计运营视图可继续深化 |
| Mobile in-app credential | Kisi app 内移动开门依赖用户、group、reader/lock 权限，不一定是单独 card issuance | 通过 `users`、`groups`、`role_assignments`、`shares` 决定权限；不新增单独 Mobile Card API | 已对齐：UI 文案需避免把移动 App 权限误写成“发卡” |

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchCards` | GET | `/api/v1/cards?user_id=&provider=&credential_kind=` | `/api/v1/wallet/passes` wrapper | Organization Admin | 已落地 reference wrapper，前端已切；已补 `credential_kind` / `save_link`，区分 Google Wallet 与实体卡 | Credentials |
| `fetchCard` | GET | `/api/v1/cards/:id` | `/api/v1/wallet/passes/:passID` wrapper | Organization Admin | 已落地 reference wrapper，Credentials detail sheet 已接；detail 展示 credential type 与 wallet save link | Credential Detail |
| `createCard` | POST | `/api/v1/cards` | `/api/v1/wallet/passes` wrapper | Organization Admin | 已落地 reference wrapper，前端已接 Issue sheet；有 UID/card_number/token 时按 `physical_card` 语义注册，空实体标识时走 Google Wallet baseline | Credentials |
| `assignCard` | POST | `/api/v1/cards/:id/assign` | `/api/v1/wallet/passes` wrapper | Organization Admin | 已落地 reference wrapper，Credentials detail sheet 已接保存 | Credential Detail |
| `deassignCard` | POST | `/api/v1/cards/:id/deassign` | `/api/v1/wallet/passes` wrapper | Organization Admin | 已落地 reference wrapper，前端已接行操作与 detail action | Credential Detail |
| `activateCard` | POST | `/api/v1/cards/:id/activate` | `/api/v1/wallet/passes/:passID/activate` wrapper | Organization Admin | 已落地 reference wrapper，前端已接行操作与 detail action | Credentials |
| `deactivateCard` | POST | `/api/v1/cards/:id/deactivate` | `/api/v1/wallet/passes/:passID/suspend` wrapper | Organization Admin | 已落地 reference wrapper，前端已接行操作与 detail action | Credentials |
| `revokeCard` | POST | `/api/v1/cards/:id/revoke` | `/api/v1/wallet/passes/:passID/revoke` wrapper | Organization Admin | 已落地 reference wrapper，前端已接行操作与 detail action | Credential Detail |
| `fetchCardAssignments` | GET | `/api/v1/card_assignments?user_id=` | `/api/v1/wallet/passes` wrapper | Organization Admin | 已落地 reference wrapper，前端已切 | Credential Detail |
| `fetchCardAssignment` | GET | `/api/v1/card_assignments/:id` | `/api/v1/wallet/passes/:passID` wrapper | Organization Admin | 已落地 reference wrapper，Credentials detail sheet 已接 | Credential Detail |
| `createCardAssignment` | POST | `/api/v1/card_assignments` | `/api/v1/wallet/passes` wrapper | Organization Admin | 已落地 reference wrapper | Credentials |
| `enrollApplePass` | POST | `/api/v1/app/credentials/apple-pass` | Wallet Apple self-service enrollment | Resident app user | 已落地 resident 自助 enrollment，创建 `credential_kind=apple_wallet` pass；Admin 直接 `POST /cards` 创建 Apple Wallet 已被 422 拦截 | Mobile App Credentials |
| `issueCardsBatch` | POST | `/api/v1/wallet/passes/issue-batch` | Wallet batch issue | Organization Admin | 复用既有 wallet batch endpoint，Credentials Issue sheet 已接批量发放，成功/失败/queued 提示已按 job 结果汇总 | Credentials |
| `fetchCardIssueJobs` | GET | `/api/v1/wallet/jobs?tenant_id=` | Wallet issue jobs | Organization Admin / Operator | 复用既有 wallet jobs endpoint，Credentials Batch Audit 已展示最近批量 job、pass id、错误原因和更新时间，并支持前端搜索、状态筛选与 CSV 导出 | Credentials |
| `fetchCardDeliveries` | GET | `/api/v1/wallet/deliveries?pass_id=` | Wallet delivery notifications | Organization Admin / Operator | 复用既有 wallet delivery endpoint，Credentials detail sheet 已接记录 | Credential Detail |
| `dispatchCardDelivery` | POST | `/api/v1/wallet/deliveries/dispatch` | Wallet delivery dispatch | Organization Admin | 复用既有 wallet delivery endpoint，Credentials detail sheet 已接发送与 retry | Credential Detail |
| `fetchPhysicalCardVendors` | GET | `/api/v1/wallet/physical-card-vendors` | Wallet physical-card vendors | Organization Admin / Operator | 已落地供应商只读资源，Credentials Physical Card 区域已接供应商选择 | Credential Detail |
| `fetchPhysicalCardInventory` | GET | `/api/v1/wallet/physical-card-inventory?status=` | Wallet physical-card inventory | Organization Admin / Operator | 已落地库存列表与状态筛选，Credentials Physical Card 区域已接可用卡选择 | Credential Detail |
| `createPhysicalCardInventory` | POST | `/api/v1/wallet/physical-card-inventory`、`/scan`、`/import`、`/import-csv` | Wallet physical-card inventory | Organization Admin | 已落地单张入库、读卡器 UID scan 入库、JSON batch import 与 CSV import；创建 task 时可按 inventory_id 占用库存 | Credential Detail |
| `updatePhysicalCardInventoryStatus` | PATCH | `/api/v1/wallet/physical-card-inventory/:inventoryID/status`、`/batch-status` | Wallet physical-card inventory governance | Organization Admin | 已落地单张/批量状态治理，支持 `available` / `frozen` / `scrapped` 转换；`reserved` / `issued` 不允许绕过实体卡 task lifecycle 直接变更；写入 audit log；Credentials detail 已接前端控件 | Credential Detail |
| `fetchPhysicalCardTasks` | GET | `/api/v1/wallet/physical-card-tasks` | Wallet physical-card tasks | Organization Admin / Operator | 复用既有 wallet physical card endpoint，Credentials detail sheet 已按 card 过滤 | Credential Detail |
| `createPhysicalCardTask` | POST/PATCH | `/api/v1/wallet/physical-card-tasks`、`/status` | Wallet physical-card tasks | Organization Admin | 复用既有 wallet physical card endpoint，Credentials detail sheet 已接创建与状态推进；task issued/loss/cancel 会同步 inventory issued/lost/available，终态推进已接确认 | Credential Detail |

### 3.11 Events And Reports

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `createEventSet` | POST | `/api/v1/event_sets` | `/api/v1/events/access` + `/api/v1/events/device` wrapper | Organization Admin / Place Admin | 已落地 reference wrapper，前端已切；`listAccessEvents` 旧 helper 已转接 `/event_sets` 并保留 `tenant_id`、`area_id`、`gateway_id` 映射 | Event History |
| `fetchEventSet` | GET | `/api/v1/event_sets/:id` | `/api/v1/events/access` + `/api/v1/events/device` wrapper | Organization Admin / Place Admin | 已落地 reference wrapper | Event Detail |
| `fetchEventMetadata` | GET | `/api/v1/events/meta` | 内置 metadata | Organization Admin / Place Admin | 已落地 reference wrapper | Event History |
| `fetchEventTypes` | GET | `/api/v1/events/types` | 内置 event types | Organization Admin / Place Admin | 已落地 reference wrapper | Event History |
| `fetchReports` | GET | `/api/v1/reports` | `/api/v1/alarms` + `/api/v1/audit-logs` + `/api/v1/events/*` 聚合 | Organization Admin / Place Admin | 已落地 reference wrapper，Reports 页已接 | Reports |
| `fetchReport` | GET | `/api/v1/reports/:id` | 聚合 report detail | Organization Admin / Place Admin | 已落地 reference wrapper | Reports |
| `downloadReport` | GET | `/api/v1/reports/:id/download` | 聚合 CSV export | Organization Admin / Place Admin | 已落地，Reports 页已接 CSV 下载 | Reports |
| `fetchScheduledReports` | GET | `/api/v1/scheduled_reports` | 内置 scheduled report state | Organization Admin / Place Admin | 已落地 baseline | Reports |
| `createScheduledReport` | POST | `/api/v1/scheduled_reports` | 内置 scheduled report state | Organization Admin / Place Admin | 已落地 baseline，含 audit | Reports |
| `updateScheduledReport` | PATCH | `/api/v1/scheduled_reports/:id` | 内置 scheduled report state | Organization Admin / Place Admin | 已落地 baseline，含 audit | Reports |
| `deleteScheduledReport` | DELETE | `/api/v1/scheduled_reports/:id` | 内置 scheduled report state | Organization Admin / Place Admin | 已落地 baseline，含 audit | Reports |

### 3.12 Integrations

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchIntegrations` | GET | `/api/v1/integrations` | `/api/v1/enterprise/hris-connectors` + `/api/v1/enterprise/idp-config` wrapper | Organization Admin | 已落地只读 reference wrapper，前端已切 | Organization Setup |
| `fetchIntegration` | GET | `/api/v1/integrations/:id` | Enterprise service wrapper | Organization Admin | 已落地 detail wrapper，Organization Setup edit sheet 已接 | Integrations |
| `createIntegration` | POST | `/api/v1/integrations` | Enterprise service wrapper | Organization Admin | 已落地 Identity Provider / HRIS wrapper，Organization Setup Add 已接 | Integrations |
| `updateIntegration` | PATCH | `/api/v1/integrations/:id` | Enterprise service wrapper | Organization Admin | 已落地 Identity Provider / HRIS wrapper，Organization Setup Save 已接 | Integrations |
| `deleteIntegration` | DELETE | `/api/v1/integrations/:id` | Enterprise service wrapper | Organization Admin | 已落地 soft-disable wrapper，Organization Setup Disable 已接 | Integrations |

### 3.13 Alert Policies

Alert Policies 先聚合现有告警订阅策略：Enterprise Sync worker 告警与 Wallet job/DLQ 告警。当前 create/update/delete 写入这些内置订阅，并支持 `custom` category 的 name、trigger、severity、condition expression、channels 与 receiver groups baseline；condition preview 可校验表达式并对样例事件返回匹配结果；event evaluate 可对 tenant 内启用的 custom policies 批量评估真实事件 payload 并返回命中策略和通知路由；delete 语义为禁用策略并保留可审计配置。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchAlertPolicies` | GET | `/api/v1/alert_policies` | built-in subscription wrapper + custom policy memory store | Organization Admin / Operator | 已落地 reference wrapper，支持 category/trigger/status/query filter，Alert Policies 页已切 | Organization Setup |
| `fetchAlertPolicy` | GET | `/api/v1/alert_policies/:id` | subscription lookup wrapper + custom policy lookup | Organization Admin / Operator | 已落地 reference wrapper，custom policy detail 已覆盖 | Alert Policies |
| `createAlertPolicy` | POST | `/api/v1/alert_policies` | subscription upsert wrapper + custom policy create | Organization Admin | 已落地 reference wrapper，Alert Policies 页已接 Add Policy 与 Custom condition | Alert Policies |
| `updateAlertPolicy` | PATCH | `/api/v1/alert_policies/:id` | subscription PUT wrapper + custom policy update | Organization Admin | 已落地 reference wrapper，Alert Policies 页已接 Save Policies 与 custom fields | Alert Policies |
| `deleteAlertPolicy` | DELETE | `/api/v1/alert_policies/:id` | subscription/custom policy soft-disable | Organization Admin | 已落地 reference wrapper，Alert Policies 页已接 Delete 确认 | Alert Policies |
| `previewAlertPolicyCondition` | POST | `/api/v1/alert_policies/condition_preview` | custom condition evaluator | Organization Admin / Operator | 已落地，支持 `condition_expression` + sample `event` 或 `policy_id` preview | Alert Policies |
| `evaluateAlertPoliciesForEvent` | POST | `/api/v1/alert_policies/evaluate` | custom policy event evaluator | Organization Admin / Operator | 已落地，支持启用 custom policies 批量事件评估并返回 matches / notification route | Alert Policies |

---

## 4. 当前迁移优先级

| 优先级 | 工作 | 原因 |
|---:|---|---|
| 1 | `/api/v1/places`、`/api/v1/floors?place_id=`、`/api/v1/locks?place_id=` | 已落地首批 reference wrapper；继续补详情、更新、删除和 lock action |
| 2 | `/api/v1/roles` 与 `/api/v1/role_assignments` | 已落地内置 Role、Role Assignment list/create/detail/update/delete；后续把鉴权 guard 从历史 role 迁到该模型 |
| 3 | `/api/v1/groups`、`/api/v1/group_locks`、`/api/v1/group_zones`、`/api/v1/members` | 已落地 groups detail/update/delete、group restrictions、group_locks 写入与 group_zones 只读；后续补更完整 schedule/restriction semantics |
| 4 | `/api/v1/shares` 与 `/api/v1/group_links` | `/api/v1/shares` 已落地 list/create/detail/update/delete wrapper，`/api/v1/group_links` 已落地管理写入、token 验证流与 Access Link claim UI |
| 5 | 新增 `/api/v1/teams` 与 `/api/v1/team_memberships` | Team detail/create/update/delete 与 Team Membership create/delete 已落地；Teams 页已接 New Team、General Save、Add/Remove Member 和 Assign Access Right |
| 6 | 新增 `/api/v1/cards` 与 `/api/v1/card_assignments` | wrapper 已落地并替换前端 wallet pass 主数据源；`cards/:id`、`card_assignments/:id` detail 与 `cards/:id/revoke` 已落地；`cards` 已补 `credential_kind`、`provider` 过滤和 `save_link`；Credentials 已接 create/batch issue/batch audit 搜索筛选导出/assign/activate/deactivate/deassign/revoke、delivery、Apple Pass self-service enrollment、physical card inventory/vendor/scan、CSV import、库存 status 治理与 task 流 |
| 7 | 新增 `/api/v1/alert_policies` | create/detail/update/delete wrapper 已落地并替换 Alert Policies 样板数据；custom policy baseline、审计、condition preview 和 event evaluate 已补，后续补调度器、渠道升级与持久化 |
| 8 | 细化 OpenAPI resource schemas 与生成链路 | operationId、collection pagination、统一 error schema、extension 分组和 legacy archive baseline 已接；后续补字段级 schema 与 client/docs 生成 |

---

## 5. 前端 Adapter 对照

| Adapter | 当前来源 | 目标资源 | 页面 |
|---|---|---|---|
| `loadKisiResourceSummary` | places/floors/areas/locks/controllers/readers/terminals/event_sets/users/cards/card_assignments/groups/door_groups/group_locks/group_zones/teams/team_memberships/roles/role_assignments/shares；`listGateways` 兼容 helper 已 reference-first 组合 controllers/readers/terminals，`access-policies` 仅 fallback，且旧 endpoint 已返回 replacement/deprecation headers | Places、Floors、Zones、Locks、Hardware、Events、Users、Cards、Groups、Teams、Team Memberships、Role Assignments、Shares | Home 以外的 preview resources |
| 旧命名 space helpers | `listBuildings` / `createBuilding` -> `/api/v1/places`；`listDoors` / `createDoor` -> `/api/v1/locks` | 旧后台或兼容调用方继续使用旧函数名，网络层走 reference endpoint；后端 `/buildings`、`/doors`、`/door-groups` 兼容入口已返回 replacement/deprecation headers | 已补单元测试，避免重新退回 `/buildings` / `/doors`；后端已补 deprecation header 回归 |
| 旧命名 temporary access helpers | `listTemporaryAccess` / `createTemporaryAccess` -> `/api/v1/shares`，并把 Share 映射回旧 `TemporaryAccess` view model | Access 旧后台继续使用旧函数名，网络层走 reference shares endpoint；后端 `/shares` wrapper 已保留 area、访客联系方式、设备型号和 pass type 字段 | 已补单元测试，避免重新退回 `/temporary-access` |
| 旧命名 access event helper | `listAccessEvents` -> `/api/v1/event_sets`，使用 `event_object_type=Lock` 过滤门禁事件，并映射回旧 `AccessEvent` view model | Home、Events 旧调用方继续使用旧函数名，网络层走 reference event set endpoint；后端 reference event 已保留 `tenant_id`、`area_id`、`gateway_id` | 已补单元测试，避免重新退回 `/events/access` |
| 旧命名 wallet pass helper | `listWalletPasses` -> `/api/v1/cards`，并把 Card 映射回旧 `WalletPassInstance` view model | Wallet / Enterprise 旧调用方继续使用旧函数名，网络层走 reference cards endpoint；issue、batch、save-link、status action 继续作为 Wallet extension | 已补单元测试，避免重新退回 `/wallet/passes` list |
| 旧命名 gateway helper | `listGateways` -> `/api/v1/controllers` + `/api/v1/readers` + `/api/v1/terminals`，旧 bind/unbind/config/reboot helper -> controller action | Gateways 旧页面继续使用旧函数名，网络层优先 reference hardware；legacy gateway action 仅保留失败兜底 | 已补单元测试，避免主路径重新退回 `/gateways` list/bind/config/reboot |
| `listAlertPolicies` / `createAlertPolicy` / `updateAlertPolicy` / `deleteAlertPolicy` / `previewAlertPolicyCondition` / `evaluateAlertPoliciesForEvent` | `/api/v1/alert_policies`、`/api/v1/alert_policies/:id`、`/api/v1/alert_policies/condition_preview`、`/api/v1/alert_policies/evaluate` | Enterprise Sync worker、Wallet job/DLQ、custom alert policy baseline、condition preview 与 event evaluate | Organization Setup / Alert Policies |
| `selectKisiPlaceContext` | `KisiResourceSummary` | 当前 Place 的 Floors、Locks、Hardware、Events、Users、Groups、Role Assignments、Shares | Place Dashboard、Doors、Floors、Hardware、Unlock History、Place Users、Place Groups |
| `useKisiResourceSummary` | React Query | Organization 资源 summary | Places、Users、Credentials、Groups、Access Rights、Event History |
| `useKisiPlaceContext` | React Query + route `placeId` | Place-scoped summary | Place resource pages |

迁移期间，前端资源类型名可以暂时保留 `Kisi*` / `Door*` 以减少改动；正式后端 endpoint 和 OpenAPI 文档必须使用本文件的目标资源名。
