# iOS 客户端联调指南

> **内部文档** — 配合 `openapi.yaml` 在 Stoplight.io 上查阅。
> 最后更新：2026-05-09

---

## 1. 网络层架构

### 1.1 技术栈

| 组件 | 选型 | 说明 |
|------|------|------|
| HTTP 客户端 | `URLSession` | 系统原生，无第三方依赖 |
| JSON 编解码 | `Codable` + `JSONDecoder`/`JSONEncoder` | 自定义 ISO8601 日期策略 |
| Token 存储 | `KeychainService` | `kSecAttrAccessibleWhenUnlockedThisDeviceOnly` |
| SSE 流 | `URLSession.bytes(for:)` | AsyncSequence 逐行解析 |
| 缓存 | SwiftData | 离线门禁 & 事件记录 |

### 1.2 Base URL 配置

通过 Info.plist `APP_ENV` 键或环境变量切换：

| 环境 | Base URL |
|------|----------|
| mock | `http://localhost:4010/api/v1` |
| dev | `http://localhost:8080/api/v1` |
| staging | `https://staging-api.mistyislet.com/api/v1` |
| production | `https://api.mistyislet.com/api/v1` |

**源文件：** `MistyisletPass/Utilities/Constants.swift`

### 1.3 超时配置

| 场景 | Request Timeout | Resource Timeout |
|------|----------------|------------------|
| 普通请求 | 15s | 30s |
| SSE 告警流 | 300s (5min) | 0 (无限) |

### 1.4 请求构建

所有请求在 `APIService.buildRequest()` 中构建：

```
URL = Constants.API.baseURL + path
Method = GET / POST / PUT / PATCH / DELETE
Header: Authorization = Bearer <access_token>  (authenticated=true 时)
Header: Content-Type = application/json  (有 body 时)
Header: Accept = text/event-stream  (SSE 流)
```

**无自定义 User-Agent 头** — iOS 系统限制。

---

## 2. 认证流程

### 2.1 登录

```
POST /app/auth/login
Body: { email, password, mfa_code? }
Response: LoginResponse { access_token, refresh_token, expires_in, user }
```

登录成功后，access_token 和 refresh_token 分别存入 Keychain：
- Key: `com.mistyislet.accessToken`
- Key: `com.mistyislet.refreshToken`

### 2.2 Magic Link 登录

```
POST /app/auth/magic-link       → { status: "sent" }
POST /app/auth/magic-link/verify → LoginResponse
```

### 2.3 组织 SSO 查询

```
GET /app/auth/org-lookup?domain={email_domain}
Response: OrgAuthConfig { auth_type, sso_url, org_name }
```

### 2.4 Token 刷新机制

**触发条件：** 任何请求返回 HTTP 401

**防竞争：** `TokenRefreshLock`（Actor）确保并发 401 只触发一次 refresh

**流程：**

```
1. 请求 A 收到 401
2. refreshLock.refresh() 检查是否已有刷新进行中
3. 若无 → performTokenRefresh()
   - 读取 Keychain 中 refresh_token
   - POST /app/auth/refresh { refresh_token }
   - 成功 → 更新 Keychain 中两个 token → return true
   - 失败 → post NotificationCenter(.sessionExpired) → return false
4. 若成功 → 用新 token 重试原请求（仅一次，不递归）
5. 若失败 → throw APIError.unauthorized
```

### 2.5 退出登录

收到 `.sessionExpired` 通知后，`AuthViewModel` 清除 Keychain 并跳转登录页。

---

## 3. 响应解析模式

### 3.1 列表包装器

大多数列表接口返回统一包装：

```json
{
  "items": [...],
  "total": 42
}
```

iOS 侧解析为 `AdminListResponse<T>`：

```swift
struct AdminListResponse<T: Decodable>: Decodable {
    let items: [T]
    let total: Int?
}
```

### 3.2 特殊包装器

| 接口 | 包装器 |
|------|--------|
| `GET .../doors` | `PlaceDoorListResponse { items: [AccessibleDoor] }` |
| `GET /app/me/logins` | `UserLoginListResponse { items: [UserLogin] }` |
| `POST /app/credentials/register` | `RegisterMobileCredentialResponse { credential }` |

### 3.3 空响应

无返回体的操作（DELETE、部分 POST）解码为内部 `Empty` 类型。

### 3.4 日期解码

自定义 `dateDecodingStrategy`，按优先级尝试：
1. ISO8601 带毫秒（`2026-05-09T08:30:00.000Z`）
2. ISO8601 标准（`2026-05-09T08:30:00Z`）

### 3.5 字段命名

- 后端 JSON：**snake_case**（`lock_id`, `device_model`, `valid_until`）
- Swift 属性：**camelCase**，通过 `CodingKeys` 映射

---

## 4. 错误处理

### 4.1 错误类型

```swift
enum APIError: Error {
    case invalidURL
    case unauthorized           // 401 + refresh 失败
    case serverError(Int, String?)  // 其他 4xx/5xx
    case networkError(Error)    // 网络不可达 / 超时
    case decodingError(Error)   // JSON 解码失败
}
```

### 4.2 HTTP 状态码处理

| 状态码 | 处理 |
|--------|------|
| 200-299 | 正常解码 |
| 401 | 自动 refresh → 重试 → 若仍 401 → `unauthorized` |
| 403 | `serverError(403, msg)` — MFA 或账号禁用 |
| 429 | `serverError(429, msg)` — 频率限制 |
| 其他 | `serverError(code, body)` |

### 4.3 后端错误体

```json
{ "error": "error message here" }
```

iOS 暂未解析结构化错误体，仅将 body 作为原始字符串传入 `serverError`。

---

## 5. 接口调用清单

### 5.1 认证 (Auth) — 无需 Bearer Token

| 方法 | 路径 | 请求体 | 响应 | ViewModel |
|------|------|--------|------|-----------|
| POST | `/app/auth/login` | `LoginRequest` | `LoginResponse` | AuthViewModel |
| POST | `/app/auth/refresh` | `{ refresh_token }` | `AuthTokens` | APIService (内部) |
| POST | `/app/auth/magic-link` | `{ email }` | `MagicLinkResponse` | AuthViewModel |
| POST | `/app/auth/magic-link/verify` | `{ token }` | `LoginResponse` | AuthViewModel |
| GET | `/app/auth/org-lookup?domain=` | — | `OrgAuthConfig` | AuthViewModel |
| POST | `/app/auth/restore-password` | `{ email }` | Empty | AuthViewModel |

### 5.2 用户资料 (Profile)

| 方法 | 路径 | 请求体 | 响应 | ViewModel |
|------|------|--------|------|-----------|
| GET | `/app/me` | — | `UserProfile` | AuthVM / ProfileVM |
| PATCH | `/app/me` | `{ name }` | `UserProfile` | ProfileViewModel |
| POST | `/app/me/avatar` | multipart (file) | `UserProfile` | ProfileViewModel |
| POST | `/app/me/change-password` | `{ current_password, new_password }` | Empty | ProfileViewModel |
| POST | `/app/me/primary-device` | — | Empty | ProfileViewModel |
| GET | `/app/me/logins` | — | `{ items: [UserLogin] }` | ProfileViewModel |
| DELETE | `/app/me/logins/{loginId}` | — | Empty | ProfileViewModel |

### 5.3 组织 & 场所 (Organization)

| 方法 | 路径 | 请求体 | 响应 | ViewModel |
|------|------|--------|------|-----------|
| GET | `/app/orgs` | — | `[Organization]` | OrgViewModel |
| POST | `/app/orgs/{orgId}/switch` | — | `LoginResponse` | OrgViewModel |
| GET | `/app/orgs/{orgId}/places` | — | `[Place]` | OrgViewModel |
| GET | `/app/orgs/{orgId}/places/search?q=` | — | `[Place]` | OrgViewModel |
| GET | `/app/orgs/{orgId}/settings` | — | `OrganizationSettings` | OrgSettingsVM |
| PUT | `/app/orgs/{orgId}/settings` | `OrganizationSettings` | `OrganizationSettings` | OrgSettingsVM |
| PUT | `/app/places/{placeId}/settings` | `{ name, address?, timezone?, capacity? }` | `Place` | PlaceSettingsVM |

### 5.4 门禁操作 (Doors)

| 方法 | 路径 | 请求体 | 响应 | ViewModel |
|------|------|--------|------|-----------|
| GET | `/app/access/my-doors` | — | `[Door]` | DoorsViewModel |
| POST | `/app/access/unlock` | `{ lock_id, ble_token? }` | `UnlockResponse` | DoorsViewModel |
| GET | `/app/access/ble-token` | — | `BleTokenResponse` | — |
| GET | `/app/access/pin-code` | — | `PinCodeResponse` | PinCodeVM |
| GET | `/app/access/logs?offset=&limit=` | — | `[AccessEvent]` | HistoryViewModel |
| GET | `/app/places/{placeId}/doors` | — | `{ items: [AccessibleDoor] }` | PlaceDoorsVM |
| GET | `/app/places/{placeId}/doors/search?q=` | — | `{ items: [AccessibleDoor] }` | PlaceDoorsVM |
| POST | `/app/places/{placeId}/doors/{doorId}/unlock` | `{ lock_id, ble_token? }` | `UnlockResponse` | PlaceDoorsVM |
| PUT | `/app/places/{placeId}/doors/{doorId}/favorite` | — | Empty | PlaceDoorsVM |
| DELETE | `/app/places/{placeId}/doors/{doorId}/favorite` | — | Empty | PlaceDoorsVM |

### 5.5 锁控 & 封锁 (Lockdown)

| 方法 | 路径 | ViewModel |
|------|------|-----------|
| POST | `/app/places/{placeId}/lockdown` | PlaceDoorsVM |
| DELETE | `/app/places/{placeId}/lockdown` | PlaceDoorsVM |
| POST | `/app/places/{placeId}/doors/{doorId}/lockdown` | DoorDetailVM |
| DELETE | `/app/places/{placeId}/doors/{doorId}/lockdown` | DoorDetailVM |

### 5.6 凭证管理 (Credentials)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| POST | `/app/credentials/register` | `RegisterMobileCredentialBody` | `{ credential }` |
| GET | `/app/credentials/mobile` | — | `{ items: [Credential] }` |
| DELETE | `/app/credentials/mobile/{id}` | — | Empty |
| POST | `/app/credentials/nfc` | `{ card_uid, card_type, label }` | `Credential` |
| GET | `/app/credentials/nfc` | — | `[Credential]` |
| DELETE | `/app/credentials/nfc/{id}` | — | Empty |
| POST | `/app/qr-token` | `{ door_id? }` | `QRTokenResponse` |

**移动凭证注册 Body：**

```json
{
  "public_key_pem": "-----BEGIN PUBLIC KEY-----\n...",
  "platform": "ios",
  "device_id": "<device_name>",
  "device_model": "<device_name>",
  "keystore_level": "strongbox",
  "attestation_cert_chain": []
}
```

### 5.7 访客 (Visitors)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/app/visitor-passes` | — | `{ items: [Visitor] }` |
| POST | `/app/visitor-passes` | `CreateVisitorRequest` | `Visitor` |
| GET | `/app/places/{placeId}/visitor-groups` | — | `[VisitorGroup]` |
| POST | `/app/places/{placeId}/visitor-groups` | `{ name, auto_remove_expired }` | `VisitorGroup` |
| GET | `.../{groupId}/members` | — | `[VisitorGroupMember]` |
| POST | `.../{groupId}/cleanup-expired` | — | `{ removed: Int }` |

### 5.8 管理后台 — 用户 (Admin-Users)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/app/places/{placeId}/users` | — | `{ items: [PlaceUser] }` |
| POST | `.../{placeId}/users/invite` | `{ email, role }` | `PlaceUser` |
| PATCH | `.../{placeId}/users/{userId}/role` | `{ role }` | `PlaceUser` |
| POST | `.../{placeId}/users/{userId}/sign-out` | — | Empty |
| DELETE | `.../{placeId}/users/{userId}` | — | Empty |

### 5.9 管理后台 — 组 (Admin-Groups)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/app/places/{placeId}/groups` | — | `{ items: [AccessGroup] }` |
| POST | `.../{placeId}/groups` | `{ name, description }` | `AccessGroup` |
| PATCH | `.../{placeId}/groups/{groupId}` | `{ name, description }` | `AccessGroup` |
| DELETE | `.../{placeId}/groups/{groupId}` | — | Empty |
| GET | `.../{groupId}/members` | — | `{ items: [GroupMember] }` |
| POST | `.../{groupId}/members` | `{ email }` | `GroupMember` |
| DELETE | `.../{groupId}/members/{memberId}` | — | Empty |
| GET | `.../{groupId}/doors` | — | `{ items: [GroupDoor] }` |
| POST | `.../{groupId}/doors` | `{ door_id }` | Empty |
| DELETE | `.../{groupId}/doors/{doorId}` | — | Empty |

### 5.10 管理后台 — 团队 (Admin-Teams)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/app/places/{placeId}/teams` | — | `{ items: [Team] }` |
| POST | `.../{placeId}/teams` | `{ name, description }` | `Team` |
| DELETE | `.../{placeId}/teams/{teamId}` | — | Empty |
| GET | `.../{teamId}/members` | — | `{ items: [TeamMember] }` |
| POST | `.../{teamId}/members` | `{ email }` | `TeamMember` |
| DELETE | `.../{teamId}/members/{memberId}` | — | Empty |
| GET | `.../{teamId}/access-rights` | — | `{ items: [AccessRightAssignment] }` |
| POST | `.../{teamId}/access-rights` | `{ role, scope, scope_id? }` | `AccessRightAssignment` |
| DELETE | `.../{teamId}/access-rights/{rightId}` | — | Empty |

### 5.11 管理后台 — 时间表 (Admin-Schedules)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/app/places/{placeId}/schedules` | — | `{ items: [UnlockSchedule] }` |
| POST | `.../{placeId}/schedules` | ScheduleWriteBody | `UnlockSchedule` |
| PUT | `.../{placeId}/schedules/{id}` | ScheduleWriteBody | `UnlockSchedule` |
| DELETE | `.../{placeId}/schedules/{id}` | — | Empty |
| GET | `.../{doorId}/restrictions` | — | `{ items: [DoorRestriction] }` |
| GET | `.../{doorId}/schedules` | — | `{ items: [UnlockSchedule] }` |

### 5.12 管理后台 — 事件 & 监控 (Admin-Events)

| 方法 | 路径 | 响应 |
|------|------|------|
| GET | `/app/places/{placeId}/events` | `{ items: [AdminEvent] }` |
| GET | `.../{placeId}/incidents` | `{ items: [Incident] }` |
| GET | `.../{placeId}/activity` | `{ items: [UserActivity] }` |
| GET | `.../{placeId}/events/{eventId}/media` | `[EventMedia]` |

### 5.13 管理后台 — 告警 (Admin-Alarms)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/app/alarms` | — | `{ items: [Alarm] }` |
| PATCH | `/app/alarms/{alarmId}/status` | `{ status }` | `Alarm` |
| GET | `/app/alarms/stream` | — | SSE `AlarmStreamEvent` |
| GET | `/app/alarm-schedules` | — | `{ items: [AlarmSchedule] }` |
| GET | `/app/alarm-schedules/calendar?timezone=` | — | `{ items: [AlarmCalendarEntry] }` |

**SSE 流实现：**
- Accept: `text/event-stream`
- 每行 `data: {json}` 解码为 `AlarmStreamEvent`
- 超时 300s，资源无限
- 重连：指数退避 2s→60s，最多 10 次

### 5.14 管理后台 — 访客登记 (Admin-Guests)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/guests` | — | `{ items: [Guest] }` |
| POST | `/guests` | `CreateGuestRequest` | `Guest` |
| PATCH | `/guests/{guestId}/status` | `{ status }` | `Guest` |
| DELETE | `/guests/{guestId}` | — | Empty |

### 5.15 管理后台 — 预订 (Admin-Bookings)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/app/bookable-spaces` | — | `{ items: [BookableSpace] }` |
| GET | `/app/bookable-spaces/{id}/status` | — | `BookableSpaceStatus` |
| GET | `/app/bookings?space_id=` | — | `{ items: [Booking] }` |
| POST | `/app/bookings` | `CreateBookingRequest` | `Booking` |
| POST | `/app/bookings/{id}/cancel` | — | `Booking` |
| POST | `/app/bookings/{id}/check-in` | — | `Booking` |
| POST | `/app/bookings/{id}/check-out` | — | `Booking` |

### 5.16 管理后台 — 卡片 & 凭证 (Admin-Cards)

| 方法 | 路径 | 响应 |
|------|------|------|
| GET | `/app/places/{placeId}/cards` | `{ items: [CardAssignment] }` |
| DELETE | `.../{placeId}/cards/{cardUid}` | Empty |
| GET | `/app/places/{placeId}/credentials` | `{ items: [DigitalCredential] }` |
| PATCH | `/wallet/passes/{passId}/suspend` | Empty |
| PATCH | `/wallet/passes/{passId}/activate` | Empty |
| PATCH | `/wallet/passes/{passId}/revoke` | Empty |

### 5.17 管理后台 — 数据分析 (Admin-Analytics)

| 方法 | 路径 | 参数 | 响应 |
|------|------|------|------|
| GET | `.../{placeId}/analytics/summary` | `?days=30` | `AnalyticsSummary` |
| GET | `.../{placeId}/analytics/presence` | `?days=30` | `{ items: [UserPresenceRecord] }` |
| POST | `.../{placeId}/reports/export` | `{ type, from, to, format }` | `ReportExportResponse` |

### 5.18 管理后台 — 摄像头 (Admin-Cameras)

| 方法 | 路径 | 响应 |
|------|------|------|
| GET | `/app/cameras` | `{ items: [Camera] }` |
| GET | `/app/cameras/{id}/video-link` | `CameraVideoLink` |
| POST | `/app/cameras/{id}/snapshot` | `CameraSnapshot` |
| PATCH | `/app/cameras/{id}` | `Camera` (rename) |

### 5.19 管理后台 — 硬件重命名 (Admin-Hardware)

| 方法 | 路径 | 请求体 |
|------|------|--------|
| PATCH | `/app/places/{placeId}/doors/{doorId}` | `{ name }` |
| PATCH | `/app/gateways/{gatewayId}` | `{ name }` |
| PATCH | `/app/cameras/{cameraId}` | `{ name }` |

### 5.20 管理后台 — 组织设置 (Admin-Settings)

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `/app/orgs/{orgId}/settings` | — | `OrganizationSettings` |
| PUT | `/app/orgs/{orgId}/settings` | `OrganizationSettings` | `OrganizationSettings` |
| PUT | `/app/places/{placeId}/settings` | place body | `Place` |

### 5.21 推送通知注册 (APNS)

| 方法 | 路径 | 请求体 |
|------|------|--------|
| POST | `/app/devices/apns` | `{ device_token, platform: "ios" }` |

---

## 6. iOS 平台特殊事项

### 6.1 Secure Enclave 密钥

- **算法：** EC P-256（secp256r1）
- **存储：** Secure Enclave（不支持时回退软件密钥）
- **导出：** 公钥 PEM 格式 → `POST /app/credentials/register`
- **用途：** BLE 开锁时签名挑战

### 6.2 BLE GATT 协议

UUID 编码规则：ASCII 字符串（6 字符一段，12 hex 位）

| 特征 | UUID | 编码来源 |
|------|------|----------|
| 服务 | `4D495354-5950-4153-532D-424C45415554` | "MISTYPASS-BLEAUT" |
| 挑战 | `4D495354-5950-4153-532D-4348414C4C4E` | "MISTYPASS-CHALLN" |
| 认证响应 | `4D495354-5950-4153-532D-415554485245` | "MISTYPASS-AUTHRE" |
| Reader ID | `4D495354-5950-4153-532D-524541444552` | "MISTYPASS-READER" |
| 结果 | `4D495354-5950-4153-532D-524553554C54` | "MISTYPASS-RESULT" |

连接超时：5 秒

### 6.3 NFC 卡

- DESFire EV3 卡片绑定
- `card_uid` 由 `NFCService` 读取
- `POST /app/credentials/nfc` 上报后端

### 6.4 文件上传

头像上传使用 `multipart/form-data`：
- 字段名：`file`
- 文件名：`avatar.jpg`
- MIME：`image/jpeg`

### 6.5 离线缓存

使用 SwiftData 缓存以下数据（支持离线浏览）：
- `CachedDoor` — 门禁列表
- `CachedAccessEvent` — 通行记录

刷新策略：API 成功后全量替换。

---

## 7. 关键源文件索引

| 文件 | 用途 |
|------|------|
| `Services/APIService.swift` | 全部 HTTP 调用（62+ 方法） |
| `Models/APIRequests.swift` | 请求 body 模型 |
| `Services/KeychainService.swift` | Token 安全存储 |
| `Utilities/Constants.swift` | 路径常量 & Base URL |
| `Services/SecureEnclaveService.swift` | EC P-256 密钥管理 |
| `Services/BLEManager.swift` | BLE GATT 通信 |
| `Services/NFCService.swift` | NFC 卡片读取 |
| `Services/NotificationService.swift` | APNS 注册 |
| `Services/NetworkMonitor.swift` | 网络状态监听 |
| `ViewModels/AuthViewModel.swift` | 登录 / 登出 / token 管理 |
| `ViewModels/DoorsViewModel.swift` | 门禁列表 & 开锁 |
| `ViewModels/PlaceDoorsViewModel.swift` | 场所门禁（多租户） |
| `ViewModels/AlarmsViewModel.swift` | 告警 + SSE 流 |
| `Utilities/AppLogger.swift` | 分类日志（.api 通道） |
