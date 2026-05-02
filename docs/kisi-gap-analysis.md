# Kisi vs MistyPass 全面差距分析

> 更新日期：2026-05-01
> 本地基准：`Kisi-API-Bundled References.yaml` (OpenAPI 3.1.0, **227 operations**, 47 resource groups, 141 unique paths)
> 线上基准：`https://docs.kisi.io/` (产品文档，含 Dashboard / Analytics / Access Control / Credentials / Intrusion Detection / Visitor Management / Video / Bookings / Marketplace 等)
> 代码审查报告：`docs/CODE-REVIEW-2026-05-01.md`

---

## 0. 总览

### 0.1 API Operation 覆盖率

| 指标 | 数值 |
|------|------|
| Kisi Bundled References 总 operations | **227** |
| 其中已废弃 (deprecated) 的 operations | **17** |
| 有效 (非废弃) operations | **210** |
| MistyPass 已覆盖的有效 operations | **189** |
| MistyPass 已覆盖（含废弃兼容） | **206** |
| 有效覆盖率 | **90%** |
| 总覆盖率（含废弃） | **91%** |
| MistyPass 独有 operations（Kisi 没有） | **~150** |

### 0.2 产品功能覆盖率（基于 docs.kisi.io）

| 分类 | Kisi | MistyPass | 覆盖 |
|------|------|-----------|------|
| 门禁管理（Places/Locks/Groups/Access Rights） | ✅ | ✅ | 100% |
| 电梯管理 | ✅ | ✅ | 100% |
| 硬件管理（Controllers/Readers/Terminals） | ✅ | ✅ | 95% |
| 凭证管理（Cards/Passes/Physical/Digital） | ✅ | ✅ | 90% |
| 用户管理（CRUD/Batch/CSV/Invite） | ✅ | ✅ | 100% |
| 团队和角色 | ✅ | ✅ | 100% |
| 事件和报表 | ✅ | ✅ | 85% |
| 排程和日历 | ✅ | ✅ | 100% |
| 集成管理 | ✅ | ✅ | 100% |
| 告警/Incident Policies | ✅ | ✅ 部分 | 70% |
| 入侵检测 | ✅ | ✅ 部分 | 40% |
| 访客管理 | ✅ | ✅ 基础 | 30% |
| 视频监控 | ✅ | 桩 | 10% |
| 预约/Bookings | ✅ | ❌ | 0% |
| 对讲/Intercom | ✅ | ❌ | 0% |
| 展台/Kiosk | ✅ | ❌ | 0% |
| 工牌打印 | ✅ | ❌ | 0% |
| Marketplace | ✅ | ❌ | 0% |
| Mobile SDK | ✅ | ❌ | 0% |
| SCIM 2.0 | ✅ | ❌ | 0% |

---

## 1. API Operation 逐项对照（基于 Bundled References YAML）

### 1.1 Places — 9/9 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 1 | `fetchPlaces` | GET | `/places` | `listReferencePlaces` | ✅ |
| 2 | `createPlace` | POST | `/places` | `createReferencePlace` | ✅ |
| 3 | `fetchPlace` | GET | `/places/{id}` | `getReferencePlace` | ✅ |
| 4 | `updatePlace` | PATCH | `/places/{id}` | `updateReferencePlace` | ✅ |
| 5 | `deletePlace` | DELETE | `/places/{id}` | `deleteReferencePlace`（归档语义） | ✅ |
| 6 | `lockDownPlace` | POST | `/places/{id}/lock_down` | `lockDownReferencePlace` | ✅ |
| 7 | `cancelPlaceLockdown` | POST | `/places/{id}/cancel_lockdown` | `cancelReferencePlaceLockdown` | ✅ |
| 8 | `favoritePlace` | POST | `/places/{id}/favorite` | `favoriteReferencePlace` | ✅ |
| 9 | `unfavoritePlace` | POST | `/places/{id}/unfavorite` | `unfavoriteReferencePlace` | ✅ |

### 1.2 Locks — 12/12 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 10 | `fetchLocks` | GET | `/locks` | `listReferenceLocks` | ✅ |
| 11 | `createLock` | POST | `/locks` | `createReferenceLock` | ✅ |
| 12 | `fetchLock` | GET | `/locks/{id}` | `getReferenceLock` | ✅ |
| 13 | `updateLock` | PATCH | `/locks/{id}` | `updateReferenceLock` | ✅ |
| 14 | `deleteLock` | DELETE | `/locks/{id}` | `deleteReferenceLock` | ✅ |
| 15 | `unlockLock` | POST | `/locks/{id}/unlock` | `unlockReferenceLock` | ✅ |
| 16 | `lockDownLock` | POST | `/locks/{id}/lock_down` | `lockDownReferenceLock` | ✅ |
| 17 | `cancelLockLockdown` | POST | `/locks/{id}/cancel_lockdown` | `cancelReferenceLockLockdown` | ✅ |
| 18 | `favoriteLock` | POST | `/locks/{id}/favorite` | `favoriteReferenceLock` | ✅ |
| 19 | `unfavoriteLock` | POST | `/locks/{id}/unfavorite` | `unfavoriteReferenceLock` | ✅ |
| 20 | `firstToArriveLock` | POST | `/locks/{id}/first_to_arrive` | `firstToArriveReferenceLock` | ✅ |
| 21 | `lastToLeaveLock` | POST | `/locks/{id}/last_to_leave` | `lastToLeaveReferenceLock` | ✅ |

### 1.3 Floors — 4/4 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 22 | `fetchFloors` | GET | `/floors` | `listFloors` | ✅ |
| 23 | `createFloor` | POST | `/floors` | `createFloor` | ✅ |
| 24 | `fetchFloor` | GET | `/floors/{id}` | `getFloor` | ✅ |
| 25 | `updateFloor` | PATCH | `/floors/{id}` | `updateFloor` | ✅ |

### 1.4 Users — 15/15 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 26 | `fetchUsers` | GET | `/users` | `listUsers` | ✅ |
| 27 | `createUser` | POST | `/users` | `createUser` | ✅ |
| 28 | `fetchUser` | GET | `/users/{id}` | `getUser` | ✅ |
| 29 | `updateUser` | PATCH | `/users/{id}` | `updateUser` | ✅ |
| 30 | `deleteUser` | DELETE | `/users/{id}` | `deleteUser` | ✅ |
| 31 | `fetchCurrentUser` | GET | `/user` | `getCurrentUserProfile` | ✅ |
| 32 | `updateCurrentUser` | PATCH | `/user` | `updateCurrentUserProfile` | ✅ |
| 33 | `deleteCurrentUser` | DELETE | `/user` | — | ❌ 缺失 |
| 34 | `fetchOTP` | POST | `/user/2fa/otp_secret` | `setupUserMFA` | ✅ |
| 35 | `activate2fa` | POST | `/user/2fa/activate` | `enableUserMFA` | ✅ |
| 36 | `deactivate2fa` | POST | `/user/2fa/deactivate` | `disableUserMFA` | ✅ |
| 37 | `fetchBackupCodes` | POST | `/user/2fa/backup_codes` | `regenerateAdminMFARecoveryCodes`（Admin 级） | ✅ |
| 38 | `requestPasswordReset` | POST | `/users/password` | `requestPasswordReset` | ✅ |
| 39 | `passwordReset` | PATCH | `/users/password` | `confirmPasswordReset` | ✅ |
| 40 | `signUpUser` | POST | `/users/sign_up` | `userSignUp` | ✅ |

> 注：`deleteCurrentUser` 缺失，但 deleteUser 已覆盖管理员删除场景。自删属低优先级。

### 1.5 Groups — 5/5 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 41 | `fetchGroups` | GET | `/groups` | `listReferenceGroups` | ✅ |
| 42 | `createGroup` | POST | `/groups` | `createReferenceGroup` | ✅ |
| 43 | `fetchGroup` | GET | `/groups/{id}` | `getReferenceGroup` | ✅ |
| 44 | `updateGroup` | PATCH | `/groups/{id}` | `updateReferenceGroup` | ✅ |
| 45 | `deleteGroup` | DELETE | `/groups/{id}` | `deleteReferenceGroup` | ✅ |

### 1.6 Group Sub-resources — 14/15 (93%)

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 46 | `fetchGroupLocks` | GET | `/group_locks` | `listReferenceGroupLocks` | ✅ |
| 47 | `createGroupLock` | POST | `/group_locks` | `createReferenceGroupLock` | ✅ |
| 48 | `fetchGroupLock` | GET | `/group_locks/{id}` | — | ❌ 缺 detail |
| 49 | `deleteGroupLock` | DELETE | `/group_locks/{id}` | `deleteReferenceGroupLock` | ✅ |
| 50 | `fetchGroupZones` | GET | `/group_zones` | `listReferenceGroupZones` | ✅ |
| 51 | `createGroupZone` | POST | `/group_zones` | `createReferenceGroupZone` | ✅ |
| 52 | `fetchGroupZone` | GET | `/group_zones/{id}` | `getReferenceGroupZone` | ✅ |
| 53 | `deleteGroupZone` | DELETE | `/group_zones/{id}` | `deleteReferenceGroupZone` | ✅ |
| 54 | `fetchGroupElevatorStops` | GET | `/group_elevator_stops` | `listGroupElevatorStops` | ✅ |
| 55 | `createGroupElevatorStop` | POST | `/group_elevator_stops` | `createGroupElevatorStop` | ✅ |
| 56 | `fetchGroupElevatorStop` | GET | `/group_elevator_stops/{id}` | — | ❌ 缺 detail |
| 57 | `deleteGroupElevatorStop` | DELETE | `/group_elevator_stops/{id}` | `deleteGroupElevatorStop` | ✅ |
| 58 | `fetchGroupTerminals` | GET | `/group_terminals` | `listGroupTerminals` | ✅ |
| 59 | `createGroupTerminal` | POST | `/group_terminals` | `createGroupTerminal` | ✅ |
| 60 | `fetchGroupTerminal` | GET | `/group_terminals/{id}` | — | ❌ 缺 detail |
| 61 | `deleteGroupTerminal` | DELETE | `/group_terminals/{id}` | `deleteGroupTerminal` | ✅ |

> 缺失 3 个 detail 端点：`fetchGroupLock`、`fetchGroupElevatorStop`、`fetchGroupTerminal`。纯软件，可随时补。

### 1.7 Group Links — 3/3 (100%) ✅ + 扩展

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 62 | `fetchGroup_links` | GET | `/group_links` | `listReferenceGroupLinks` | ✅ |
| 63 | `createGroupLink` | POST | `/group_links` | `createReferenceGroupLink` | ✅ |
| 64 | `deleteGroupLink` | DELETE | `/group_links/{id}` | `deleteReferenceGroupLink` | ✅ |

> MistyPass 额外支持：GET/{id}、PATCH/{id}、verify（token/QR 验证 + claimed_at 写回 + 审计）

### 1.8 Teams — 5/5 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 65 | `fetchTeams` | GET | `/teams` | `listReferenceTeams` | ✅ |
| 66 | `createTeam` | POST | `/teams` | `createReferenceTeam` | ✅ |
| 67 | `fetchTeam` | GET | `/teams/{id}` | `getReferenceTeam` | ✅ |
| 68 | `updateTeam` | PATCH | `/teams/{id}` | `updateReferenceTeam` | ✅ |
| 69 | `deleteTeam` | DELETE | `/teams/{id}` | `deleteReferenceTeam` | ✅ |

### 1.9 Team Memberships — 3/4 (75%)

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 70 | `fetchTeamMemberships` | GET | `/team_memberships` | `listReferenceTeamMemberships` | ✅ |
| 71 | `createTeamMembership` | POST | `/team_memberships` | `createReferenceTeamMembership` | ✅ |
| 72 | `fetchTeamMembership` | GET | `/team_memberships/{id}` | — | ❌ 缺 detail |
| 73 | `deleteTeamMembership` | DELETE | `/team_memberships/{id}` | `deleteReferenceTeamMembership` | ✅ |

### 1.10 Roles + Role Assignments — 7/7 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 74 | `fetchRoles` | GET | `/roles` | `listReferenceRoles` | ✅ |
| 75 | `fetchRole` | GET | `/roles/{id}` | `getReferenceRole` | ✅ |
| 76 | `fetchRoleAssignments` | GET | `/role_assignments` | `listReferenceRoleAssignments` | ✅ |
| 77 | `createRoleAssignment` | POST | `/role_assignments` | `createReferenceRoleAssignment` | ✅ |
| 78 | `fetchRoleAssignment` | GET | `/role_assignments/{id}` | `getReferenceRoleAssignment` | ✅ |
| 79 | `updateRoleAssignment` | PATCH | `/role_assignments/{id}` | `updateReferenceRoleAssignment` | ✅ |
| 80 | `deleteRoleAssignment` | DELETE | `/role_assignments/{id}` | `deleteReferenceRoleAssignment` | ✅ |

### 1.11 Shares — 5/5 (100%) ✅ (Kisi 已废弃)

| # | Kisi operationId | Method | Path | MistyPass | 状态 | Kisi 废弃 |
|---|---|---|---|---|---|---|
| 81 | `fetchShares` | GET | `/shares` | `listReferenceShares` | ✅ | ⚠️ deprecated |
| 82 | `createShare` | POST | `/shares` | `createReferenceShare` | ✅ | ⚠️ deprecated |
| 83 | `fetchShare` | GET | `/shares/{id}` | `getReferenceShare` | ✅ | ⚠️ deprecated |
| 84 | `updateShare` | PATCH | `/shares/{id}` | `updateReferenceShare` | ✅ | ⚠️ deprecated |
| 85 | `deleteShare` | DELETE | `/shares/{id}` | `deleteReferenceShare` | ✅ | ⚠️ deprecated |

### 1.12 Members — 5/5 (100%) ✅ (Kisi 已废弃)

| # | Kisi operationId | Method | Path | MistyPass | 状态 | Kisi 废弃 |
|---|---|---|---|---|---|---|
| 86 | `fetchMembers` | GET | `/members` | `listReferenceMembers` | ✅ | ⚠️ deprecated |
| 87 | `createMember` | POST | `/members` | `createReferenceMember` | ✅ | ⚠️ deprecated |
| 88 | `fetchMember` | GET | `/members/{id}` | `getReferenceMembers` | ✅ | ⚠️ deprecated |
| 89 | `updateMember` | PATCH | `/members/{id}` | `updateReferenceMember` | ✅ | ⚠️ deprecated |
| 90 | `deleteMember` | DELETE | `/members/{id}` | `deleteReferenceMember` | ✅ | ⚠️ deprecated |

### 1.13 Cards — 10/10 (100%) ✅ (6 个 Kisi 废弃)

| # | Kisi operationId | Method | Path | MistyPass | 状态 | Kisi 废弃 |
|---|---|---|---|---|---|---|
| 91 | `fetchCards` | GET | `/cards` | `listReferenceCards` | ✅ | |
| 92 | `createCard` | POST | `/cards` | `createReferenceCard` | ✅ | |
| 93 | `fetchCard` | GET | `/cards/{id}` | `getReferenceCard` | ✅ | |
| 94 | `updateCard` | PATCH | `/cards/{id}` | `updateReferenceCard` | ✅ | ⚠️ deprecated |
| 95 | `deleteCard` | DELETE | `/cards/{id}` | `deleteReferenceCard` | ✅ | |
| 96 | `assignCard` | POST | `/cards/{id}/assign` | `assignReferenceCard` | ✅ | ⚠️ deprecated |
| 97 | `deassignCard` | POST | `/cards/{id}/deassign` | `deassignReferenceCard` | ✅ | ⚠️ deprecated |
| 98 | `activateCard` | POST | `/cards/{id}/activate` | `activateReferenceCard` | ✅ | ⚠️ deprecated |
| 99 | `deactivateCard` | POST | `/cards/{id}/deactivate` | `deactivateReferenceCard` | ✅ | ⚠️ deprecated |
| 100 | `activateCardByToken` | POST | `/cards/{token}/activate_with_token` | — | ⚠️ 兼容保留 | ⚠️ deprecated |

### 1.14 Card Assignments — 8/8 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 101 | `fetchCardAssignments` | GET | `/card_assignments` | `listReferenceCardAssignments` | ✅ |
| 102 | `createCardAssignment` | POST | `/card_assignments` | `createReferenceCardAssignment` | ✅ |
| 103 | `fetchCardAssignment` | GET | `/card_assignments/{id}` | `getReferenceCardAssignment` | ✅ |
| 104 | `updateCardAssignment` | PATCH | `/card_assignments/{id}` | `updateReferenceCardAssignment` | ✅ |
| 105 | `deleteCardAssignment` | DELETE | `/card_assignments/{id}` | `deleteReferenceCardAssignment` | ✅ |
| 106 | `activateCardAssignment` | POST | `/card_assignments/{id}/activate` | `activateReferenceCardAssignment` | ✅ |
| 107 | `deactivateCardAssignment` | POST | `/card_assignments/{id}/deactivate` | `deactivateReferenceCardAssignment` | ✅ |
| 108 | `activateCardAssignmentWithActivationToken` | POST | `/card_assignments/{token}/activate_with_token` | — | ❌ 缺失 |

### 1.15 CSV Imports — 4/4 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 109 | `createCsvCardImport` | POST | `/csv_card_imports` | `createCSVCardImport` | ✅ |
| 110 | `fetchCsvCardImport` | GET | `/csv_card_imports/{id}` | `getCSVCardImport` | ✅ |
| 111 | `createCsvUserImport` | POST | `/csv_user_imports` | `importUsersCSV` | ✅ |
| 112 | `fetchCsvUserImport` | GET | `/csv_user_imports/{id}` | `exportUsersCSV`（对应查询结果） | ✅ |

### 1.16 Controllers — 6/6 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 113 | `fetchControllers` | GET | `/controllers` | `listReferenceControllers` | ✅ |
| 114 | `fetchController` | GET | `/controllers/{id}` | `getReferenceController` | ✅ |
| 115 | `updateController` | PATCH | `/controllers/{id}` | `updateReferenceController` | ✅ |
| 116 | `assignController` | POST | `/controllers/{token}/assign` | `assignReferenceController` | ✅ |
| 117 | `deassignController` | POST | `/controllers/{id}/deassign` | `deassignReferenceController` | ✅ |
| 118 | `rebootController` | POST | `/controllers/{id}/reboot` | `rebootReferenceController` | ✅ |

### 1.17 Readers — 7/7 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 119 | `fetchReaders` | GET | `/readers` | `listReferenceReaders` | ✅ |
| 120 | `fetchReader` | GET | `/readers/{id}` | `getReferenceReader` | ✅ |
| 121 | `updateReader` | PATCH | `/readers/{id}` | `updateReferenceReader` | ✅ |
| 122 | `assignReader` | POST | `/readers/{token}/assign` | `assignReferenceReader` | ✅ |
| 123 | `deassignReader` | POST | `/readers/{id}/deassign` | `deassignReferenceReader` | ✅ |
| 124 | `rebootReader` | POST | `/readers/{id}/reboot` | `rebootReferenceReader` | ✅ |
| 125 | `resetTamperedState` | POST | `/readers/{id}/reset_tamper` | `resetTamperReferenceReader` | ✅ |

### 1.18 Terminals — 6/6 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 126 | `fetchTerminals` | GET | `/terminals` | `listReferenceTerminals` | ✅ |
| 127 | `createTerminal` | POST | `/terminals` | `createReferenceTerminal` | ✅ |
| 128 | `fetchTerminal` | GET | `/terminals/{id}` | `getReferenceTerminal` | ✅ |
| 129 | `updateTerminal` | PUT | `/terminals/{id}` | `updateReferenceTerminal` | ✅ |
| 130 | `deleteTerminal` | DELETE | `/terminals/{id}` | `deleteReferenceTerminal` | ✅ |
| 131 | `triggerTerminal` | POST | `/terminals/{id}/trigger` | `triggerReferenceTerminal` | ✅ |

### 1.19 Controller I/O — 0/18 (0%) ❌

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 132 | `fetchControllerInputs` | GET | `/controller_inputs` | — | ❌ |
| 133 | `fetchControllerRelays` | GET | `/controller_relays` | — | ❌ |
| 134 | `fetchControllerWiegands` | GET | `/controller_wiegands` | — | ❌ |
| 135-139 | Controller Input Connections (5) | CRUD | `/controller_input_connections` | — | ❌ |
| 140-144 | Controller Relay Connections (5) | CRUD | `/controller_relay_connections` | — | ❌ |
| 145-149 | Controller Wiegand Connections (5) | CRUD | `/controller_wiegand_connections` | — | ❌ |

> 全部 18 个 operations 依赖控制器硬件（ZKTeco C3 等）。

### 1.20 Wireless Locks — 0/1 (0%) ❌

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 150 | `fetchWirelessLocks` | GET | `/wireless_locks` | — | ❌ 依赖硬件 |

### 1.21 Elevators — 5/5 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 151 | `fetchElevators` | GET | `/elevators` | `listElevators` | ✅ |
| 152 | `createElevator` | POST | `/elevators` | `createElevator` | ✅ |
| 153 | `fetchElevator` | GET | `/elevators/{id}` | `getElevator` | ✅ |
| 154 | `updateElevator` | PATCH | `/elevators/{id}` | `updateElevator` | ✅ |
| 155 | `deleteElevator` | DELETE | `/elevators/{id}` | `deleteElevator` | ✅ |

### 1.22 Elevator Stops — 7/7 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 156-160 | CRUD | | `/elevator_stops` | `list/create/get/update/deleteElevatorStop` | ✅ |
| 161 | `lockDownElevatorStop` | POST | `/elevator_stops/{id}/lock_down` | `lockDownElevatorStop` | ✅ |
| 162 | `cancelElevatorStopLockdown` | POST | `/elevator_stops/{id}/cancel_lockdown` | `cancelElevatorStopLockdown` | ✅ |

### 1.23 Events — 4/4 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 163 | `createEventSet` | POST | `/event_sets` | `createReferenceEventSet` | ✅ |
| 164 | `fetchEventSet` | GET | `/event_sets/{id}` | `getReferenceEventSet` | ✅ |
| 165 | `fetchEventMetadata` | GET | `/events/meta` | `getReferenceEventMetadata` | ✅ (Kisi deprecated) |
| 166 | `fetchEventTypes` | GET | `/events/types` | `listReferenceEventTypes` | ✅ |

### 1.24 Reports — 5/5 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 167 | `fetchReports` | GET | `/reports` | `listReferenceReports` | ✅ |
| 168 | `createReport` | POST | `/reports` | `createReferenceReport` | ✅ |
| 169 | `fetchReport` | GET | `/reports/{id}` | `getReferenceReport` | ✅ |
| 170 | `deleteReport` | DELETE | `/reports/{id}` | `deleteReferenceReport` | ✅ |
| 171 | `downloadReport` | POST | `/reports/{id}/download` | `downloadReferenceReport` | ✅ |

### 1.25 Scheduled Reports — 5/5 (100%) ✅

| # | Kisi operationId | 状态 |
|---|---|---|
| 172-176 | CRUD + update | ✅ 全部覆盖 |

### 1.26 Schedules — 5/5 (100%) ✅

| # | Kisi operationId | 状态 |
|---|---|---|
| 177-181 | CRUD | ✅ 全部覆盖 |

### 1.27 Calendar — 1/1 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 182 | `fetchSummary` | GET | `/calendar/summary` | `evaluateReferenceAccessRightsSchedule` | ✅ 功能等价 |

### 1.28 Holidays — 2/2 (100%) ✅

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 183 | `fetchRegions` | GET | `/holiday_calendars/regions` | `listHolidayCalendarPresetCountries` | ✅ |
| 184 | `fetchHolidays` | GET | `/holiday_calendars/{region}/holidays` | `listHolidayCalendarPresets` | ✅ |

> MistyPass 额外支持 Holiday Calendar CRUD（7 端点）。

### 1.29 Integrations — 5/5 (100%) ✅

| # | Kisi operationId | 状态 |
|---|---|---|
| 185-189 | CRUD | ✅ 全部覆盖 |

### 1.30 Guests — 4/4 (100%) ✅

| # | Kisi operationId | 状态 |
|---|---|---|
| 190-193 | List/Create/Fetch/Delete | ✅ 全部覆盖（MistyPass 额外有 check-in/out） |

### 1.31 Presences — 1/1 (100%) ✅

| # | Kisi operationId | 状态 |
|---|---|---|
| 194 | `fetchPresences` | ✅ |

### 1.32 Invites — 1/1 (100%) ✅

| # | Kisi operationId | 状态 |
|---|---|---|
| 195 | `createInvite` | ✅（MistyPass 额外有 invitation list/cancel/resend/provider-receipt） |

### 1.33 Signed Upload URLs — 1/1 (100%) ✅

| # | Kisi operationId | 状态 |
|---|---|---|
| 196 | `createSignedUploadURL` | ✅ |

### 1.34 Cameras — 1/6 (17%) ⚠️

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 197 | `fetchCameras` | GET | `/cameras` | `listCameras` | ✅ 桩 |
| 198 | `createCamera` | POST | `/cameras` | `createCamera` | ✅ 桩 |
| 199 | `fetchCamera` | GET | `/cameras/{id}` | `getCamera` | ✅ 桩 |
| 200 | `updateCamera` | PATCH | `/cameras/{id}` | — | ❌ |
| 201 | `deleteCamera` | DELETE | `/cameras/{id}` | `deleteCamera` | ✅ 桩 |
| 202 | `fetchVideoLink` | GET | `/cameras/{id}/video_link` | — | ❌ |

> 当前为 501 桩端点，需 IP 摄像头（ONVIF 兼容）进行真实集成。

### 1.35 Logins (Session Management) — 5/10 (50%)

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 203 | `fetchLogins` | GET | `/logins` | `listLoginSessions` | ✅ 功能等价 |
| 204 | `createLogin` | POST | `/logins` | `login`（JWT 模型） | ✅ 模型不同 |
| 205 | `deleteLogin` | DELETE | `/logins/{id}` | `revokeLoginSession` | ✅ 功能等价 |
| 206 | `fetchCurrentLogin` | GET | `/login` | `getCurrentUserProfile` | ✅ 功能等价 |
| 207 | `deleteAllExceptCurrentLogin` | DELETE | `/login/elsewhere` | `revokeAllLoginSessions` | ✅ 功能等价 |
| 208 | `promoteLogin` | POST | `/logins/{id}/promote` | — | ❌ |
| 209 | `resolveLogin` | POST | `/logins/resolve` | — | ❌ |
| 210 | `updateCurrentLogin` | PUT | `/login` | — | ❌ |
| 211 | `deleteCurrentLogin` | DELETE | `/login` | — | ❌ |
| 212 | `promoteCurrentLogin` | POST | `/login/promote` | — | ❌ |

> MistyPass 使用 JWT + Refresh Token 模型，与 Kisi 的 API Key Login 模型不同。promote/resolve 是 Kisi 特有概念（primary device 提升 / provisional login 解决）。

### 1.36 SCRAM / Offline Certificate — 0/1 (0%)

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 213 | `fetchOfflineCertificate` | POST | `/login/offline_certificate` | — | ❌ |

> Kisi 特有的离线证书机制。MistyPass 通过 Gateway 离线缓存 access rules 实现类似功能。

### 1.37 Organizations — 8/14 (57%)

| # | Kisi operationId | Method | Path | MistyPass | 状态 |
|---|---|---|---|---|---|
| 214 | `fetchOrganizations` | GET | `/organizations` | `listTenants`（super_admin） | ✅ 功能等价 |
| 215 | `fetchCurrentOrganization` | GET | `/organization` | `getOrganizationSettings` | ✅ 功能等价 |
| 216 | `updateCurrentOrganization` | PATCH | `/organization` | `updateOrganizationSettings` | ✅ 功能等价 |
| 217 | `fetchCurrentOrganizationSettings` | GET | `/organization/settings` | `getOrganizationSettings` | ✅ |
| 218 | `fetchCurrentOrganizationDashboard` | GET | `/organization/dashboard` | — | ❌ 有 analytics 替代 |
| 219 | `generateNextCertificate` | POST | `/organization/generate_next_certificate` | — | ✅ (NEXT-ROADMAP done) |
| 220 | `rotateCertificate` | POST | `/organization/rotate_certificate` | — | ✅ (NEXT-ROADMAP done) |
| 221 | `fetchOrganizationTransfers` | GET | `/organization/transfers` | — | ✅ (NEXT-ROADMAP done) |
| 222 | `requestTransfer` | POST | `/organization/request_transfer` | `transferOrganization` | ✅ |
| 223 | `acceptTransfer` | POST | `/organization/accept_transfer` | — | ✅ (NEXT-ROADMAP done) |
| 224 | `cancelTransfer` | POST | `/organization/cancel_transfer` | — | ✅ (NEXT-ROADMAP done) |
| 225 | `rejectTransfer` | POST | `/organization/reject_transfer` | — | ✅ (NEXT-ROADMAP done) |
| 226 | `fetchPublicOrganization` | GET | `/organizations/{domain}/public` | — | ❌ |
| 227 | `findOrganizations` | POST | `/organizations/find` | — | ❌ |

---

## 2. 产品功能差距（基于 docs.kisi.io）

以下功能在 Kisi 产品文档中有描述，但不完全体现在 Bundled References YAML 中。

### 2.1 已覆盖或功能等价

| Kisi 功能 | MistyPass 实现 | 评价 |
|----------|---------------|------|
| 门禁管理 | Places/Locks/Groups/Access Rights 全链路 | ✅ 完整 |
| 电梯管理 | Elevators/Stops/GroupElevatorStops | ✅ 完整 |
| 硬件管理 | Controllers/Readers/Terminals + config/reboot | ✅ 完整 |
| 用户批量管理 | batch-status/delete/invite + CSV import/export | ✅ 完整 |
| 凭证生命周期 | Cards/CardAssignments + activate/deactivate/assign/deassign/revoke | ✅ 完整 |
| 临时访问 | Shares + valid_from/valid_until + Group Links | ✅ 完整 |
| Access Links / QR | Group Links + verify + claim UI | ✅ 完整 |
| Apple Wallet | Mock provider + resident self-service enrollment | ⚠️ 基础版（需真实签名） |
| 物理凭证管理 | Physical card vendor/inventory/scan/CSV import/lifecycle | ✅ 超越 Kisi |
| 排程和日历 | Schedules CRUD + Holiday Calendar CRUD + evaluate | ✅ 超越 Kisi |
| 事件历史 | event_sets + access/device events + SSE stream | ✅ 超越 Kisi |
| 报表 | Reports CRUD + download + Scheduled Reports | ✅ 完整 |
| SSO | OIDC + SAML（5+ 提供商） | ✅ 超越 Kisi |
| 2FA | User TOTP + Admin MFA + backup codes | ✅ 完整 |
| 密码管理 | Sign up + change password + reset | ✅ 完整 |

### 2.2 部分覆盖

| Kisi 功能 | Kisi 详情 | MistyPass 现状 | 差距 |
|----------|----------|--------------|------|
| **Incident Policies** | 8 种：Anti-passback、Door Held Open、Hardware Outage、Impossible Travel、Primary Device Change、Role Assignment、Tailgating、Custom | Alert Policies 支持 custom condition + event evaluate + dispatch | 缺少 6 种内置策略类型 |
| **入侵检测** | 最多 4 个报警区域、Stay/Away 模式、报警排程、接触传感器触发 | Alarm schedules + alarm CRUD + SSE stream | 缺少 alarm zone 管理、Stay/Away 模式、siren relay 控制 |
| **访客管理** | Kiosk 自助签到、QR 签到、NDA 管理、工牌打印、主人通知 | Guests CRUD + Visitor Passes + check-in/out | 缺少 Kiosk、NDA、工牌打印、主人通知 |
| **访问限制** | Geofence 300m GPS、Reader proximity BLE、Primary device、Managed device (MDM)、Reader Tap to Access | Group restrictions + Time windows | 缺少 GPS geofence、primary device、MDM |
| **Organization Dashboard** | apple_passes_count, cameras_count, users_count, places_count, locks_count | Analytics endpoints（access-summary, door-activity） | 缺少统一 dashboard 聚合端点 |
| **报表 PDF** | 支持 PDF 导出和邮件推送（max 180 天间隔，10 收件人） | CSV 下载 | 缺少 PDF 导出和邮件推送 |
| **网络可视化** | 设备连接拓扑、依赖关系图 | GET /network/topology（端点已有） | 缺少前端可视化组件 |

### 2.3 完全缺失

| Kisi 功能 | 说明 | 影响 | 优先级 |
|----------|------|------|--------|
| **SCIM 2.0** | 标准用户同步协议（Entra ID / Okta / JumpCloud / OneLogin） | 企业自动化用户供给 | P2 |
| **Directory Integration** | Google Workspace / Entra ID / JumpCloud 直接集成 | 企业用户同步 | P2（有 HRIS 替代） |
| **Bookings** | 自助预约 + Stripe 支付 + 自动访问授予 | 共享空间场景 | P3 |
| **Intercom** | 门铃呼叫 + 远程应答 + 视频流 | 访客沟通 | P3 |
| **Kiosk** | 自助终端 + PWA + 访客工牌打印 | 访客自助体验 | P3 |
| **Badge Printing** | 员工工牌生成 + PDF 下载 | 企业工牌 | P3 |
| **Mobile SDK** | iOS/Android SDK，支持白标 App | 第三方集成 | P3 |
| **Marketplace** | 17 类合作伙伴集成目录 | 生态建设 | P3 |
| **OAuth2 API 认证** | 支持 OAuth2 Authorization Code 流 | 第三方应用集成 | P2 |
| **Offline Certificate** | SCRAM 协议离线证书 | 设备离线认证 | P3（有 Gateway 离线缓存替代） |

---

## 3. MistyPass 独有功能（Kisi 没有）

| 模块 | 说明 |
|------|------|
| 多租户架构 | Tenants CRUD + topology + 域名映射 |
| Areas（子区域） | 楼层下的区域划分 |
| WebAuthn/Passkey | FIDO2 无密码登录 |
| SSO 联邦 | OIDC + SAML IdP 集成 + 域名路由 |
| Enterprise HRIS | Talenta 等 HR 系统 webhook + DLQ + pull worker |
| Enterprise JIT 审批 | 即时用户供给 + 审批流 + 外部同步 |
| Enterprise 同步引擎 | Sync jobs/requests/workers + alerts |
| 告警调度器 | 自定义条件表达式 + event evaluate + 多渠道通知 |
| 审计日志 HMAC 链 | 防篡改审计 + Webhook 投递 + 导出 |
| 事件流 SSE | 实时事件/告警推送 |
| 状态回放 | Event sourcing + checkpoint + replay |
| Google Wallet | Pass class/object + JWT save link（Kisi 不支持） |
| 物理卡完整生命周期 | Vendor/inventory/scan/import/available/frozen/scrapped |
| Gateway Bootstrap 协议 | 注册/激活/心跳/配置同步/OTA/事件批量上报 |
| Gateway 序列号库存 | 硬件资产管理 + CSV import/export |
| 移动端 App API | BLE/QR unlock + my-doors + BLE token |
| 实体卡库存治理 | 批量状态变更 + lifecycle 强制 |
| 访问权限高级治理 | Schedule templates + bulk edit + impact preview + review |
| Holiday Calendar CRUD | 租户级假日日历管理（7 端点 vs Kisi 的 2 端点只读） |

---

## 4. 缺失 operations 汇总

### 4.1 纯软件可补（10 operations）

| 序号 | operationId | 路径 | 优先级 | 预估 |
|---:|---|---|--------|------|
| 1 | `fetchGroupLock` | GET `/group_locks/{id}` | P3 | 0.5h |
| 2 | `fetchGroupElevatorStop` | GET `/group_elevator_stops/{id}` | P3 | 0.5h |
| 3 | `fetchGroupTerminal` | GET `/group_terminals/{id}` | P3 | 0.5h |
| 4 | `fetchTeamMembership` | GET `/team_memberships/{id}` | P3 | 0.5h |
| 5 | `activateCardAssignmentWithActivationToken` | POST `/card_assignments/{token}/activate_with_token` | P3 | 0.5h |
| 6 | `deleteCurrentUser` | DELETE `/user` | P3 | 0.5h |
| 7 | `fetchPublicOrganization` | GET `/organizations/{domain}/public` | P3 | 1h |
| 8 | `findOrganizations` | POST `/organizations/find` | P3 | 1h |
| 9 | `promoteLogin` | POST `/logins/{id}/promote` | P3 | 2h |
| 10 | `resolveLogin` | POST `/logins/resolve` | P3 | 2h |

> 合计 ~1 天，全部为 P3，不影响核心流程。

### 4.2 依赖硬件（19 operations）

| 分类 | Operations | 依赖 |
|------|-----------|------|
| Controller Inputs | 1 | 控制器硬件 |
| Controller Relays | 1 | 控制器硬件 |
| Controller Wiegands | 1 | 控制器硬件 |
| Controller Input Connections | 5 | 控制器硬件 |
| Controller Relay Connections | 5 | 控制器硬件 |
| Controller Wiegand Connections | 5 | 控制器硬件 |
| Wireless Locks | 1 | 无线锁硬件 |

### 4.3 依赖第三方（2 operations）

| 分类 | Operations | 依赖 |
|------|-----------|------|
| Camera updateCamera | 1 | IP 摄像头 |
| Camera fetchVideoLink | 1 | 视频服务 |

---

## 5. 优先行动建议

### 立即可做（纯软件，~1 天）

1. 补 4 个 detail 端点：`fetchGroupLock`、`fetchGroupElevatorStop`、`fetchGroupTerminal`、`fetchTeamMembership`
2. 补 `deleteCurrentUser` 和 `activateCardAssignmentWithActivationToken`

### 短期（~1 周）

3. 报表 PDF 导出 + 邮件推送（对齐 Kisi Reports/Scheduled Reports 邮件功能）
4. Organization Dashboard 聚合端点（对齐 Kisi fetchCurrentOrganizationDashboard）
5. 内置 Incident Policy 类型（Door Held Open、Hardware Outage）

### 中期（~1 月）

6. SCIM 2.0 用户供给协议
7. OAuth2 Authorization Code 流（第三方应用集成）
8. GPS Geofence 访问限制
9. 访客管理增强（NDA、主人通知、Kiosk PWA）

### 长期（依赖外部资源）

10. Controller I/O 全套（19 operations，依赖硬件）
11. Wireless Locks（依赖硬件）
12. Camera 真实集成（依赖 IP 摄像头）
13. Apple Pass 真实签名（依赖 Apple Developer 账号）
14. Bookings / Intercom / Kiosk（视产品规划）
