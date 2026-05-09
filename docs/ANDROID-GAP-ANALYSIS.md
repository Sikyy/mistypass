# Android 功能缺口清单（对标 iOS）

> 基于 2026-05-09 双端全量代码扫描生成
> iOS 源码：`/ios-MistyisletPass/`
> Android 源码：`/android-MistyisletPass/`

---

## 汇总

| 优先级 | 缺口数 | 说明 |
|--------|--------|------|
| P0 关键 | 4 | 直接影响用户核心登录 & 开锁流程 |
| P1 重要 | 7 | 影响管理后台或安全功能 |
| P2 体验 | 6 | 提升用户体验和完整度 |
| P3 锦上添花 | 3 | 未来迭代可做 |
| **合计** | **20** | |

---

## P0 — 关键缺口

### 1. Magic Link 登录

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ 完整 | ❌ 未实现 |
| API | `POST /app/auth/magic-link` + `POST /app/auth/magic-link/verify` | AuthApi 只有 login / refresh / restore-password |
| UI | AuthViewModel 有 emailEntry → magicLinkSent 状态机 | LoginViewModel 只有邮箱+密码 |

**需要做：**
- AuthApi 新增 `requestMagicLink()` 和 `verifyMagicLink()` 两个方法
- LoginViewModel 增加 magic link 流程状态机
- LoginScreen 增加 "发送登录链接" 入口和等待验证 UI
- 处理从邮件跳回 App 的 deep link（`mistyislet://magic-link?token=xxx`）

---

### 2. 组织 SSO / 域名查询

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ 完整 | ❌ 未实现 |
| API | `GET /app/auth/org-lookup?domain={email_domain}` → OrgAuthConfig | AuthApi 无此方法 |
| UI | AuthViewModel.lookupOrganization() → 根据结果走 SSO/SAML 或密码登录 | 无 |

**需要做：**
- AuthApi 新增 `@GET("app/auth/org-lookup")` 方法
- 新增 `OrgAuthConfig` 数据模型（auth_type, sso_url, org_name）
- LoginViewModel 在输入邮箱后自动查询域名 → 判断走 SSO 还是密码
- 如果是 SSO → 打开 CustomTabs 跳转到 sso_url → 回调处理

---

### 3. 地理围栏实际实现

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ GeofenceService 完整实现 | ⚠️ 仅有 UI 开关，无实际功能 |
| 功能 | 50m 圆形围栏、进入/退出通知、最多 20 个、syncGeofences 自动同步 | ProfileViewModel 有 `KEY_GEOFENCE_ENABLED` toggle，但无 GeofencingClient 调用 |

**需要做：**
- 新增 `GeofenceManager` 服务类，使用 Google Play `GeofencingClient`
- 注册 `GeofenceBroadcastReceiver` 处理进入/退出事件
- `DoorRepository` 在刷新门禁列表后调用 `syncGeofences()` 同步围栏
- 进入围栏 → 发送本地通知（提示可开锁）
- 退出围栏 → 清理状态
- 处理 `ACCESS_FINE_LOCATION` + `ACCESS_BACKGROUND_LOCATION` 权限
- 上限 100 个围栏（Android 限制，比 iOS 的 20 个多）

---

### 4. Deep Linking

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ Custom Scheme + Universal Links | ❌ Intent Filter 存在但未配置 |
| iOS 路由 | `mistyislet://unlock/{doorId}` → 门禁页<br>`mistyislet://pass` → 钱包页<br>`mistyislet://dashboard` → 历史页<br>`mistyislet://profile` → 个人页<br>`https://app.mistyislet.com/visitor/{token}` → 访客认证 | 无 |

**需要做：**
- `AndroidManifest.xml` 配置 `<intent-filter>` 和 `<data>` 标签
- Navigation Compose 添加 `NavDeepLink` 注解
- 新增 `DeepLinkHandler` 处理 scheme 和 https 链接
- 处理场景：push 通知点击、邮件链接、Widget 跳转、Magic Link 回调
- App Links 验证（`.well-known/assetlinks.json` 已在后端部署）

---

## P1 — 重要缺口

### 5. SSE 告警实时流

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ `GET /app/alarms/stream` SSE | ❌ 依赖 FCM 推送 |
| 实现 | AlarmsViewModel.startStreaming() + 指数退避重连 | AdminApi 无 stream 方法 |
| 延迟 | ~0ms（实时） | ~2-5s（FCM 延迟） |

**需要做：**
- 选择方案：OkHttp SSE (推荐 `okhttp-sse` 库) 或保持 FCM
- 如果做 SSE：新增 `AlarmStreamManager`，在告警页打开时连接
- 解析 `data: {json}` 行 → `AlarmStreamEvent`
- 重连策略：指数退避 2s→60s，最多 10 次
- 页面离开时断开连接

**或者：** 如果 FCM 延迟可接受（2-5s），可以标记为"不做"，但需在告警页加一个手动刷新按钮。

---

### 6. Wallet Pass 挂起 / 激活

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ suspend / activate / revoke 三个操作 | ⚠️ 只有 revoke |
| API | `PATCH /wallet/passes/{passId}/suspend`<br>`PATCH /wallet/passes/{passId}/activate`<br>`PATCH /wallet/passes/{passId}/revoke` | AdminApi 只有 revokeCredential |

**需要做：**
- AdminApi 新增 `suspendCredential()` 和 `activateCredential()` 方法
- AdminDigitalCredentialsScreen 的凭证操作菜单增加"挂起"和"激活"选项
- 状态联动：挂起的凭证不可用但可恢复，吊销不可恢复

---

### 7. 门禁限制条件 UI

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ 展示 DoorRestriction 列表 | ⚠️ API 已调用但无 UI |
| 数据 | type, latitude, longitude, radiusMeters, isEnabled | PlaceApi 有 `getDoorRestrictions()` |

**需要做：**
- DoorsViewModel 中 `loadDoorRestrictions()` 已经存在
- 新增 `DoorRestrictionsSheet`（BottomSheet），展示限制条件列表
- 每条显示：类型（地理围栏/读卡器距离）、参数、是否启用
- 如有 GPS 限制 → 可在小地图上标注范围

---

### 8. 门禁时间表查看/编辑 UI

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ DoorSchedules 列表展示 | ⚠️ API 已调用但无 UI |
| 数据 | schedule name, type, startTime, endTime, daysOfWeek | PlaceApi 有 `getDoorSchedules()` |

**需要做：**
- DoorsViewModel 中 `loadDoorSchedules()` 已经存在
- 新增 `DoorSchedulesSheet`（BottomSheet），展示该门关联的时间表
- 每条显示：名称、类型标签、时间范围、星期
- 可选：跳转到管理后台时间表编辑页

---

### 9. 门禁重命名

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ `renameDoor(placeId:doorId:name:)` | ❌ AdminApi 有 PATCH 方法但无 UI 入口 |

**需要做：**
- 门禁详情页/长按菜单增加"重命名"选项
- 弹出 AlertDialog 输入新名称
- 调用 `AdminApi.renameDoor(placeId, doorId, RenameRequest(name))` 
- 刷新列表

---

### 10. 组更新（PATCH）

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ `updateGroup(placeId:groupId:name:description:)` PATCH | ❌ 只有 create / delete |
| API | iOS 调用 `PATCH /app/places/{placeId}/groups/{groupId}` | AdminApi 无此方法 |

**需要做：**
- AdminApi 新增 `@PATCH("app/places/{placeId}/groups/{groupId}")` 方法
- AdminGroupDetailScreen 增加"编辑"按钮 → 弹出 EditGroupSheet
- 支持修改组名称和描述

---

### 11. 访客组清理过期成员

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ `cleanupExpiredVisitors(placeId:groupId:)` | ❌ PlaceApi 有 cleanup-expired 但 VisitorsViewModel 未调用 |

**需要做：**
- VisitorsViewModel 增加 `cleanupExpired()` 方法
- 访客组详情页添加"清理过期成员"按钮
- 调用 `POST .../visitor-groups/{groupId}/cleanup-expired`
- 显示清理结果（移除了 N 名过期成员）

---

## P2 — 体验提升

### 12. 触觉反馈

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ HapticService（holdStart / unlockGranted / unlockDenied / buttonTap） | ❌ 无 |

**需要做：**
- 新增 `HapticHelper` 工具类，使用 `HapticFeedbackConstants` 或 `VibrationEffect`
- 开锁按住 → `LONG_PRESS` 反馈
- 开锁成功 → `CONFIRM` 反馈
- 开锁失败 → `REJECT` 反馈
- 按钮点击 → `CLOCK_TICK` 反馈
- Settings 中增加开关

---

### 13. 屏幕亮度控制

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ 展示通行证时自动提高到 100% 亮度 | ❌ 无 |

**需要做：**
- 在 QR 码 / PIN 码展示页，将 `WindowManager.LayoutParams.screenBrightness` 设为 1.0f
- 离开页面时恢复原始亮度
- Settings 中增加 `autoScreenBrightness` 开关

---

### 14. 通行证动态 QR 自动刷新优化

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ WalletView 有完整的自动刷新 + 倒计时 + 过期提示 | ⚠️ 有 30s 刷新但无倒计时 UI |

**需要做：**
- QR 码展示页增加倒计时进度条（环形或线性）
- 过期前 5s 提示即将刷新
- 刷新失败时显示错误状态而非空白

---

### 15. 访客 QR 码分享

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ 生成 QR + 可分享 | ⚠️ 生成 QR 但无分享入口 |

**需要做：**
- 访客 QR 码页增加分享按钮
- 使用 `Intent.ACTION_SEND` 分享 QR 图片或访客链接

---

### 16. 告警日历时区参数

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ `fetchAlarmCalendar(timezone: "Asia/Jakarta")` | ⚠️ 只传 from/to 不传 timezone |
| API | `GET /app/alarm-schedules/calendar?timezone=Asia/Jakarta` | `@Query("from")` + `@Query("to")` |

**需要做：**
- AdminApi 的 `listAlarmCalendar()` 增加 `@Query("timezone")` 参数
- 默认值使用设备时区 `TimeZone.getDefault().id`

---

### 17. Bookable Space 状态查询

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ `GET /app/bookable-spaces/{spaceId}/status` | ❌ 未调用 |

**需要做：**
- AdminApi 新增 `getBookableSpaceStatus(spaceId)` 方法
- 预订空间列表中实时显示每个空间的占用状态
- 用颜色标识：空闲(绿)、使用中(红)、即将空闲(黄)

---

## P3 — 锦上添花

### 18. Lock Screen Widget

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ Lock Screen Widget (circular, rectangular, inline) | ⚠️ 只有 Home Screen Widget |

**需要做：**
- Android 14+ 支持 Lock Screen Widgets（Glance 已支持）
- 扩展现有 `UnlockWidget` 支持 `androidx.glance.appwidget.GlanceLockScreenWidget`
- 提供快速开锁操作

---

### 19. 管理后台搜索增强

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ AdminUsersListView 有搜索 | ⚠️ 部分屏幕有搜索，部分没有 |

**iOS 有搜索但 Android 缺失的屏幕：**
- 用户列表搜索
- 卡片列表搜索
- 数字凭证列表搜索

**需要做：**
- 在对应 AdminScreen 顶部增加 `SearchBar`
- 本地过滤（name / email 匹配）

---

### 20. 主设备标记

| | iOS | Android |
|---|-----|---------|
| 状态 | ✅ `POST /app/me/primary-device` | ❌ UserApi 无此方法 |

**需要做：**
- UserApi 新增 `@POST("app/me/primary-device")`
- ProfileScreen 增加"设为主设备"按钮
- 主设备在多设备场景下优先接收推送

---

## API 端点覆盖率对比

| 端点 | iOS | Android |
|------|-----|---------|
| `POST /app/auth/magic-link` | ✅ | ❌ |
| `POST /app/auth/magic-link/verify` | ✅ | ❌ |
| `GET /app/auth/org-lookup` | ✅ | ❌ |
| `POST /app/orgs/{orgId}/switch` | ✅ | ❌ |
| `PATCH /app/me` (更新资料) | ✅ | ❌ |
| `POST /app/me/primary-device` | ✅ | ❌ |
| `POST /app/credentials/nfc` | ✅ | ✅ (BindCardScreen) |
| `GET /app/credentials/nfc` | ✅ | ❌ |
| `DELETE /app/credentials/nfc/{id}` | ✅ | ❌ |
| `GET /app/alarms/stream` (SSE) | ✅ | ❌ |
| `PATCH .../groups/{groupId}` | ✅ | ❌ |
| `PATCH /wallet/passes/{id}/suspend` | ✅ | ❌ |
| `PATCH /wallet/passes/{id}/activate` | ✅ | ❌ |
| `GET /app/bookable-spaces/{id}/status` | ✅ | ❌ |
| `GET /app/orgs/{orgId}/places/search` | ✅ | ❌ |

---

## 实施建议

### 第一批（P0，预计 3-4 天）
1. Magic Link 登录 — 影响新用户首次登录体验
2. Deep Linking — Magic Link 回调依赖此功能
3. 组织 SSO 查询 — 企业客户必需
4. 地理围栏 — 开关已有，补实现

### 第二批（P1，预计 3-4 天）
5. 门禁限制条件 UI + 时间表 UI — 数据层已就绪，只需 UI
6. 门禁重命名 — API 已有，加 UI 入口
7. 组更新 PATCH — 一个接口 + 一个表单
8. Wallet Pass 挂起/激活 — 两个接口 + 菜单项
9. 访客组清理 — 一个按钮
10. SSE 告警流（或确认 FCM 方案可接受）

### 第三批（P2，预计 2-3 天）
11. 触觉反馈 — 工具类 + 几个调用点
12. 屏幕亮度 — 一个 Activity 属性
13. QR 倒计时 + 分享
14. 告警日历时区 + Space 状态
15. Bookable Space 实时状态

### 第四批（P3，预计 1-2 天）
16. Lock Screen Widget
17. 搜索增强
18. 主设备标记
