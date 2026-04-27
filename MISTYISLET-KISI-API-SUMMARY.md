# Mistyislet API 汇总

> 更新日期：2026-04-27
> 目标：作为 Mistyislet 管理后台资源 API 的总索引
> 本地基准：`Kisi-API-Bundled References.yaml`
> 线上基准：`https://api.getkisi.com/docs#`
> 产品概念参考：`https://docs.kisi.io/`
> UI 计划：`MISTYPASS-KISI-UI-REFORM-PLAN.md`

本文档按 Bundled Reference 的资源模型组织 API，而不是按旧后端模块组织。Mistyislet UI 可以继续使用更容易理解的产品文案，例如 Doors、Access Rights、Credentials；但新增后端 API 必须优先对齐参考 API 的资源名，例如 `locks`、`role_assignments`、`shares`、`cards`、`card_assignments`。

当前已完成第一批 reference-style endpoint：`/api/v1/places`、`/api/v1/locks`、`/api/v1/controllers`、`/api/v1/readers`、`/api/v1/terminals`、`/api/v1/groups`、`/api/v1/group_locks`、`/api/v1/members`、`/api/v1/teams`、`/api/v1/team_memberships`、`/api/v1/cards`、`/api/v1/card_assignments`、`/api/v1/roles`、`/api/v1/role_assignments`、`/api/v1/shares`、`/api/v1/event_sets`、`/api/v1/events/meta`、`/api/v1/events/types`、`/api/v1/integrations`。其中 `roles` 与 `role_assignments` 已成为新的权限层级落点；历史 `user.role` 只保留为登录和迁移期 guard 的兼容字段。

---

## 1. 基准结论

### 1.1 OpenAPI 规则

| 项 | 规则 |
|---|---|
| Format | JSON only；请求应发送 `Accept: application/json` 与 `Content-Type: application/json` |
| Endpoint | 保留 `/api/v1` 前缀，资源路径使用复数，并尽量与 Bundled Reference 资源名一致 |
| Operation | 使用 `fetch*` / `create*` / `update*` / `delete*`，动作接口使用动词，例如 `unlockLock`、`lockDownPlace` |
| 字段 | JSON 与 query 使用 snake_case |
| 集合查询 | 使用 `ids`、`query`、`limit`、`offset`、资源 scope query，例如 `place_id`、`group_id`、`user_id` |
| 集合响应 | 目标形态为数组响应，并通过 `X-Collection-Range` 表达分页；迁移期旧接口可继续返回 `{ items: [] }` |
| 写入 payload | 使用资源包裹，例如 `{ "place": {} }`、`{ "group": {} }`、`{ "role_assignment": {} }` |
| ID 参数 | 参考 API 使用 `/resources/{id}`；Mistyislet 文档使用 `/resources/:id` 表达同一语义 |
| Error | 401/403/404/422 使用统一 error schema；不要让页面模块返回自定义错误结构 |

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

当前阶段保持 `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/me` 可用；后续若提供 OpenAPI，需要同时描述 Bearer JWT 与参考 scheme 的兼容关系。

### 1.3 状态标记

| 状态 | 含义 |
|---|---|
| 已接 adapter | 前端新 UI 已使用现有旧 endpoint 转成资源视图 |
| 已落地 reference wrapper | 后端已提供 Bundled Reference 风格路径，内部可暂时复用旧数据模型 |
| 已落地 | 后端已有新资源状态或内置资源模型 |
| 旧接口可用 | 后端已有旧模块 endpoint，但名称或结构未对齐参考 API |
| 目标接口 | 按 Bundled Reference 需要新增或替换的 endpoint |
| 待定义 | 资源模型还需要补字段、权限或状态语义 |

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
| `fetchPlace` | GET | `/api/v1/places/:id` | `/api/v1/tenants/:tenant_id/topology` | Organization Admin / Place Admin | 目标接口 | Place Dashboard |
| `createPlace` | POST | `/api/v1/places` | `/api/v1/buildings` | Organization Admin | 已落地 reference wrapper | Create Place |
| `updatePlace` | PATCH | `/api/v1/places/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Place Settings |
| `deletePlace` | DELETE | `/api/v1/places/:id` | 待补 | Organization Admin | 目标接口 | Places |
| `lockDownPlace` | POST | `/api/v1/places/:id/lock_down` | 待补 | Organization Admin / Place Admin | 目标接口 | Place Settings |
| `cancelPlaceLockdown` | POST | `/api/v1/places/:id/cancel_lockdown` | 待补 | Organization Admin / Place Admin | 目标接口 | Place Settings |

### 3.2 Floors

参考 API 使用 `/floors?place_id=`，不是嵌套 `/places/:place_id/floors`。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchFloors` | GET | `/api/v1/floors?place_id=:place_id` | `/api/v1/floors` + `/api/v1/areas` | Organization Admin / Place Admin | 已支持 `place_id` 查询 | Floors |
| `fetchFloor` | GET | `/api/v1/floors/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Floors |
| `createFloor` | POST | `/api/v1/floors` | `/api/v1/floors` | Organization Admin / Place Admin | 旧接口可用 | Floors |
| `updateFloor` | PATCH | `/api/v1/floors/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Floors |

### 3.3 Locks（UI Doors）

Kisi API 的门点资源名是 `locks`。Mistyislet UI 可以继续显示 Doors，但目标 API 应使用 `/locks`。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchLocks` | GET | `/api/v1/locks?place_id=:place_id` | `/api/v1/doors` | Organization Admin / Place Admin | 已落地 reference wrapper | Doors |
| `fetchLock` | GET | `/api/v1/locks/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Door Detail |
| `createLock` | POST | `/api/v1/locks` | `/api/v1/doors` | Organization Admin / Place Admin | 已落地 reference wrapper | Doors |
| `updateLock` | PATCH | `/api/v1/locks/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Door Detail |
| `deleteLock` | DELETE | `/api/v1/locks/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Doors |
| `unlockLock` | POST | `/api/v1/locks/:id/unlock` | 待补 | Organization Admin / Place Admin | 目标接口 | Door Detail |
| `lockDownLock` | POST | `/api/v1/locks/:id/lock_down` | 待补 | Organization Admin / Place Admin | 目标接口 | Door Detail |
| `cancelLockLockdown` | POST | `/api/v1/locks/:id/cancel_lockdown` | 待补 | Organization Admin / Place Admin | 目标接口 | Door Detail |

### 3.4 Hardware

参考 API 将硬件拆成 controllers、readers、terminals、wireless_locks 等资源。Mistyislet 当前 `gateways` 仍是实现层聚合。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchControllers` | GET | `/api/v1/controllers` | `/api/v1/gateways` wrapper | Organization Admin / Place Admin | 已落地只读 reference wrapper，前端已切 | Hardware |
| `fetchReaders` | GET | `/api/v1/readers` | `/api/v1/gateways` devices wrapper | Organization Admin / Place Admin | 已落地只读 reference wrapper，前端已切 | Hardware |
| `fetchTerminals` | GET | `/api/v1/terminals?place_id=` | `/api/v1/gateways` devices wrapper | Organization Admin / Place Admin | 已落地只读 reference wrapper，前端已切 | Hardware |
| `assignController` | POST | `/api/v1/controllers/:token/assign` | `/api/v1/gateways/register` | Organization Admin / Place Admin | 旧接口可用 | Hardware |
| `deassignController` | POST | `/api/v1/controllers/:id/deassign` | 待补 | Organization Admin / Place Admin | 目标接口 | Hardware |
| `rebootController` | POST | `/api/v1/controllers/:id/reboot` | `/api/v1/gateways/:gatewayID/reboot` | Organization Admin / Place Admin | 旧接口可用 | Hardware |
| `rebootReader` | POST | `/api/v1/readers/:id/reboot` | 待补 | Organization Admin / Place Admin | 目标接口 | Hardware |

### 3.5 Users And Members

`users` 是组织用户资源；`members` 更接近“某个 Place/Group 下的可访问成员视图”。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchUsers` | GET | `/api/v1/users` | `/api/v1/users` | Organization Admin / Place Admin | 已接 adapter | Users |
| `fetchPlaceUsers` | GET | `/api/v1/users?place_id=:place_id` | `/api/v1/users` + 前端过滤 `building_id` | Place Admin | 已支持 `place_id` 查询 | Place Users |
| `fetchMembers` | GET | `/api/v1/members?place_id=:place_id` | `/api/v1/users` | Organization Admin / Place Admin | 已落地 reference wrapper | Place Users / Groups |
| `fetchUser` | GET | `/api/v1/users/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | User Detail |
| `createUser` | POST | `/api/v1/users` | `/api/v1/users` | Organization Admin / Place Admin | 旧接口可用 | Users |
| `updateUser` | PATCH | `/api/v1/users/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | User Detail |
| `deleteUser` | DELETE | `/api/v1/users/:id` | 待补 | Organization Admin | 目标接口 | Users |
| `fetchCurrentUser` | GET | `/api/v1/user` | `/api/v1/me` | Authenticated | 旧接口可用 | My Account |
| `updateCurrentUser` | PATCH | `/api/v1/user` | 待补 | Authenticated | 目标接口 | My Account |

### 3.6 Teams

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchTeams` | GET | `/api/v1/teams?scope=organization|place` | 新 `teams` state | Organization Admin / Place Admin | 已落地只读 | Teams |
| `fetchTeam` | GET | `/api/v1/teams/:id` | 待补 | Organization Admin / Place Admin | 待定义 | Team Detail |
| `createTeam` | POST | `/api/v1/teams` | 待补 | Organization Admin | 待定义 | Teams |
| `updateTeam` | PATCH | `/api/v1/teams/:id` | 待补 | Organization Admin | 待定义 | Team Detail |
| `deleteTeam` | DELETE | `/api/v1/teams/:id` | 待补 | Organization Admin | 待定义 | Teams |
| `fetchTeamMemberships` | GET | `/api/v1/team_memberships?team_id=` | 新 `team_memberships` state | Organization Admin / Place Admin | 已落地只读 | Team Detail |
| `createTeamMembership` | POST | `/api/v1/team_memberships` | 待补 | Organization Admin | 目标接口 | Team Detail |
| `deleteTeamMembership` | DELETE | `/api/v1/team_memberships/:id` | 待补 | Organization Admin | 目标接口 | Team Detail |

### 3.7 Groups And Access Resources

Groups 是访问控制核心：一个 Group 通过 relation resources 绑定 locks/zones/elevator stops/terminals，并带限制条件。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchGroups` | GET | `/api/v1/groups?scope=&place_id=` | `/api/v1/user-groups` + `/api/v1/door-groups` | Organization Admin / Place Admin | 已落地 reference wrapper | Groups / Place Groups |
| `fetchGroup` | GET | `/api/v1/groups/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Group Detail |
| `createGroup` | POST | `/api/v1/groups` | `/api/v1/user-groups` / `/api/v1/door-groups` | Organization Admin / Place Admin | 已落地 reference wrapper | Groups |
| `updateGroup` | PATCH | `/api/v1/groups/:id` | `/api/v1/user-groups/:groupID` | Organization Admin / Place Admin | 旧接口可用 | Group Detail |
| `deleteGroup` | DELETE | `/api/v1/groups/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Groups |
| `fetchGroupLocks` | GET | `/api/v1/group_locks?group_id=` | `/api/v1/door-groups` | Organization Admin / Place Admin | 已落地 reference wrapper | Group Detail |
| `createGroupLock` | POST | `/api/v1/group_locks` | `/api/v1/door-groups` | Organization Admin / Place Admin | 目标接口 | Group Detail |
| `deleteGroupLock` | DELETE | `/api/v1/group_locks/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Group Detail |
| `fetchGroupZones` | GET | `/api/v1/group_zones?group_id=` | 待补 | Organization Admin / Place Admin | 目标接口 | Groups |
| `fetchGroupLinks` | GET | `/api/v1/group_links?group_id=` | 待补 | Organization Admin / Place Admin | 目标接口 | Access Links |
| `createGroupLink` | POST | `/api/v1/group_links` | 待补 | Organization Admin / Place Admin | 目标接口 | Access Links |

### 3.8 Roles And Role Assignments

这一组替代之前自造的 `/access_rights` 目标模型。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchRoles` | GET | `/api/v1/roles` | 内置角色种子 | Organization Admin / Place Admin | 已落地 | Access Rights / Settings |
| `fetchRole` | GET | `/api/v1/roles/:id` | 内置角色种子 | Organization Admin / Place Admin | 已落地 | Role Detail |
| `fetchRoleAssignments` | GET | `/api/v1/role_assignments?applies_to_type=&applies_to_id=` | 新 `role_assignments` state | Organization Admin / Place Admin | 已落地 | Access Rights |
| `createRoleAssignment` | POST | `/api/v1/role_assignments` | 新 `role_assignments` state | Organization Admin / Place Admin | 已落地 | Share Access Flow |
| `fetchRoleAssignment` | GET | `/api/v1/role_assignments/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Access Rights |
| `updateRoleAssignment` | PATCH | `/api/v1/role_assignments/:id` | 新 `role_assignments` state | Organization Admin / Place Admin | 已落地 | Access Rights |
| `deleteRoleAssignment` | DELETE | `/api/v1/role_assignments/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Access Rights |

### 3.9 Shares

Shares 覆盖“给某个邮箱分享访问权限”和临时访问流，替代 `/access_rights/share`。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchShares` | GET | `/api/v1/shares?group_id=&place_id=&user_id=&role_id=` | `/api/v1/temporary-access` | Organization Admin / Place Admin | 已落地 reference wrapper | Access Rights / Access Links |
| `createShare` | POST | `/api/v1/shares` | `/api/v1/temporary-access` | Organization Admin / Place Admin | 已落地 reference wrapper | Share Access Flow |
| `fetchShare` | GET | `/api/v1/shares/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Access Link Detail |
| `updateShare` | PATCH | `/api/v1/shares/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Access Link Detail |
| `deleteShare` | DELETE | `/api/v1/shares/:id` | 待补 | Organization Admin / Place Admin | 目标接口 | Access Rights |

### 3.10 Credentials

参考 API 中长期实体凭证更接近 `cards` 与 `card_assignments`；临时链接更接近 `group_links` 或 `shares`。

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchCards` | GET | `/api/v1/cards?user_id=` | `/api/v1/wallet/passes` wrapper | Organization Admin | 已落地只读 reference wrapper，前端已切 | Credentials |
| `createCard` | POST | `/api/v1/cards` | `/api/v1/wallet/passes/issue` | Organization Admin | 旧接口可用 | Credentials |
| `assignCard` | POST | `/api/v1/cards/:id/assign` | 待补 | Organization Admin | 目标接口 | Credential Detail |
| `deassignCard` | POST | `/api/v1/cards/:id/deassign` | 待补 | Organization Admin | 目标接口 | Credential Detail |
| `activateCard` | POST | `/api/v1/cards/:id/activate` | `/api/v1/wallet/passes/:passID/activate` wrapper | Organization Admin | 已落地 reference wrapper | Credentials |
| `deactivateCard` | POST | `/api/v1/cards/:id/deactivate` | `/api/v1/wallet/passes/:passID/suspend` wrapper | Organization Admin | 已落地 reference wrapper | Credentials |
| `fetchCardAssignments` | GET | `/api/v1/card_assignments?user_id=` | `/api/v1/wallet/passes` wrapper | Organization Admin | 已落地只读 reference wrapper，前端已切 | Credential Detail |
| `createCardAssignment` | POST | `/api/v1/card_assignments` | 待补 | Organization Admin | 目标接口 | Credentials |

### 3.11 Events And Reports

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `createEventSet` | POST | `/api/v1/event_sets` | `/api/v1/events/access` + `/api/v1/events/device` wrapper | Organization Admin / Place Admin | 已落地 reference wrapper，前端已切 | Event History |
| `fetchEventSet` | GET | `/api/v1/event_sets/:id` | `/api/v1/events/access` + `/api/v1/events/device` wrapper | Organization Admin / Place Admin | 已落地 reference wrapper | Event Detail |
| `fetchEventMetadata` | GET | `/api/v1/events/meta` | 内置 metadata | Organization Admin / Place Admin | 已落地 reference wrapper | Event History |
| `fetchEventTypes` | GET | `/api/v1/events/types` | 内置 event types | Organization Admin / Place Admin | 已落地 reference wrapper | Event History |
| `fetchReports` | GET | `/api/v1/reports` | `/api/v1/alarms` + `/api/v1/audit-logs` | Organization Admin / Place Admin | 旧接口可用 | Reports |
| `createReport` | POST | `/api/v1/reports` | 待补 | Organization Admin | 目标接口 | Reports |
| `downloadReport` | POST | `/api/v1/reports/:id/download` | 待补 | Organization Admin | 目标接口 | Reports |
| `fetchScheduledReports` | GET | `/api/v1/scheduled_reports` | 待补 | Organization Admin | 目标接口 | Reports |

### 3.12 Integrations

| Operation | Method | 目标路径 | 当前路径 | Scope | 状态 | 页面 |
|---|---:|---|---|---|---|---|
| `fetchIntegrations` | GET | `/api/v1/integrations` | `/api/v1/enterprise/hris-connectors` + `/api/v1/enterprise/idp-config` wrapper | Organization Admin | 已落地只读 reference wrapper，前端已切 | Organization Setup |
| `fetchIntegration` | GET | `/api/v1/integrations/:id` | 待补 | Organization Admin | 目标接口 | Integrations |
| `createIntegration` | POST | `/api/v1/integrations` | `/api/v1/enterprise/hris-connectors` | Organization Admin | 旧接口可用 | Integrations |
| `updateIntegration` | PATCH | `/api/v1/integrations/:id` | `/api/v1/enterprise/hris-connectors/:connectorID` | Organization Admin | 旧接口可用 | Integrations |
| `deleteIntegration` | DELETE | `/api/v1/integrations/:id` | 待补 | Organization Admin | 目标接口 | Integrations |

---

## 4. 当前迁移优先级

| 优先级 | 工作 | 原因 |
|---:|---|---|
| 1 | `/api/v1/places`、`/api/v1/floors?place_id=`、`/api/v1/locks?place_id=` | 已落地首批 reference wrapper；继续补详情、更新、删除和 lock action |
| 2 | `/api/v1/roles` 与 `/api/v1/role_assignments` | 已落地内置 Role 与可持久化 Role Assignment；后续把鉴权 guard 从历史 role 迁到该模型 |
| 3 | `/api/v1/groups`、`/api/v1/group_locks`、`/api/v1/members` | 已落地只读/基础 wrapper；后续补 group_locks 写入与 group_zones |
| 4 | `/api/v1/shares` 与 `/api/v1/group_links` | `/api/v1/shares` 已落地 wrapper；后续补 group_links token 流程 |
| 5 | 新增 `/api/v1/teams` 与 `/api/v1/team_memberships` | 只读 endpoint 已落地；后续补创建/删除 membership |
| 6 | 新增 `/api/v1/cards` 与 `/api/v1/card_assignments` | 只读 wrapper 已落地并替换前端 wallet pass 主数据源；后续补 create/assign/deassign |
| 7 | 补 OpenAPI operationId、collection pagination、统一 error schema | 保持文档和前端 client 生成稳定 |

---

## 5. 前端 Adapter 对照

| Adapter | 当前来源 | 目标资源 | 页面 |
|---|---|---|---|
| `loadKisiResourceSummary` | places/floors/areas/locks/controllers/readers/terminals/event_sets/users/cards/card_assignments/groups/door-groups/teams/team_memberships/roles/role_assignments/shares；`gateways`、`events/access`、`wallet passes`、`access-policies`、`temporary-access` 仅 fallback | Places、Floors、Locks、Hardware、Events、Users、Cards、Groups、Teams、Team Memberships、Role Assignments、Shares | Home 以外的 preview resources |
| `selectKisiPlaceContext` | `KisiResourceSummary` | 当前 Place 的 Floors、Locks、Hardware、Events、Users、Groups、Role Assignments、Shares | Place Dashboard、Doors、Floors、Hardware、Unlock History、Place Users、Place Groups |
| `useKisiResourceSummary` | React Query | Organization 资源 summary | Places、Users、Credentials、Groups、Access Rights、Event History |
| `useKisiPlaceContext` | React Query + route `placeId` | Place-scoped summary | Place resource pages |

迁移期间，前端资源类型名可以暂时保留 `Kisi*` / `Door*` 以减少改动；正式后端 endpoint 和 OpenAPI 文档必须使用本文件的目标资源名。
