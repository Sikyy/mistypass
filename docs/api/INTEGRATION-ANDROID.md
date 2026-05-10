# Android 客户端联调指南

> **内部文档** — 配合 `openapi.yaml` 在 Stoplight.io 上查阅。
> 最后更新：2026-05-10

---

## 1. 网络层架构

### 1.1 技术栈

| 组件 | 选型 | 说明 |
|------|------|------|
| HTTP 客户端 | OkHttp 4.x | 拦截器架构 |
| REST 框架 | Retrofit 2.x | 注解式接口定义 |
| JSON 序列化 | kotlinx.serialization | `@Serializable` 注解 |
| DI | Hilt (Dagger) | `@Module @InstallIn(SingletonComponent)` |
| Token 存储 | EncryptedSharedPreferences | AES256-GCM 加密 |
| 本地缓存 | Room 2.x | 离线门禁 / 凭证 / 通行记录 |
| 推送 | Firebase Cloud Messaging | FCM token 注册 |

### 1.2 Base URL 配置

通过 `build.gradle.kts` 中 `buildConfigField` 按构建变体设置：

| 环境 | Base URL |
|------|----------|
| debug | `http://10.0.2.2:8080/api/v1` |
| staging | `https://staging-api.mistyislet.com/api/v1` |
| release | `https://api.mistyislet.com/api/v1` |

### 1.3 OkHttp 配置

**两个 OkHttpClient 实例：**

| 实例 | 名称 | 超时 | 拦截器 |
|------|------|------|--------|
| Auth 专用 | `@Named("auth")` | 30s connect / 30s read | Logging 仅 |
| 主客户端 | 默认 | 30s connect / 30s read | AuthInterceptor + Logging |

- **日志级别：** DEBUG 构建 `BODY`，Release `NONE`
- **无证书锁定**（Certificate Pinning）
- **无 SSE / WebSocket** — 使用 FCM 推送代替

### 1.4 JSON 序列化配置

```kotlin
Json {
    ignoreUnknownKeys = true    // 向前兼容
    coerceInputValues = true    // null → 默认值
    encodeDefaults = true       // 序列化默认值
}
```

Converter: `json.asConverterFactory("application/json")`

---

## 2. 认证流程

### 2.1 登录

```
POST /app/auth/login
Body: LoginRequest { email, password }
Response: LoginResponse { access_token, refresh_token, expires_in, user }
```

登录成功后存入 `EncryptedTokenStore`：
- `access_token` (String)
- `refresh_token` (String)
- `expires_at` (Long, 毫秒时间戳)

过期时间计算：`System.currentTimeMillis() + expiresIn * 1000L`

### 2.2 Token 验证

```kotlin
fun isValid(): Boolean =
    accessToken != null && System.currentTimeMillis() < expiresAt - 60_000
```

60 秒提前量，避免在请求过程中 token 过期。

### 2.3 AuthInterceptor 机制

**文件：** `core/network/AuthInterceptor.kt`

**请求阶段：** 附加 `Authorization: Bearer {accessToken}` 头

**401 处理流程：**

```
1. 请求返回 401
2. Mutex 锁获取（防止并发刷新竞争）
3. 检查 token 是否仍然有效（可能被其他线程刷新了）
   - 有效 → 用当前 token 重试
   - 无效 → 继续刷新
4. 使用 auth-only OkHttpClient（无拦截器）调用:
   POST /app/auth/refresh { refresh_token }
5. 成功 → 更新 TokenStore → 重试原请求
6. 失败 → 清除所有 token → 请求不带认证继续（触发重新登录）
```

**重要：** Auth 专用 OkHttpClient 不含 AuthInterceptor，避免循环。

### 2.4 密码重置

```
POST /app/auth/restore-password
Body: RestorePasswordRequest { email }
Response: Unit
```

### 2.5 Magic Link 登录

**流程：**

```
1. 用户输入 email → POST /app/auth/org-lookup?domain={emailDomain}
2. 后端返回 OrgAuthConfig { auth_type, sso_url?, org_name? }
3. 若 auth_type == "magic_link":
   a. 调用 POST /app/auth/magic-link { email }
   b. 后端发送含 token 的 magic link 邮件
   c. 用户点击邮件链接 → 打开 mistyislet://magic-link?token=xxx
   d. DeepLinkHandler 提取 token → 自动调用 POST /app/auth/magic-link/verify { token }
   e. 验证成功返回 LoginResponse → 存入 TokenStore
```

**接口：**

| 步骤 | 方法 | 路径 | 请求/参数 | 响应 |
|------|------|------|-----------|------|
| 组织查询 | GET | `app/auth/org-lookup` | `?domain=example.com` | `OrgAuthConfig` |
| 发送链接 | POST | `app/auth/magic-link` | `MagicLinkRequest { email }` | `MagicLinkResponse { status }` |
| 验证 token | POST | `app/auth/magic-link/verify` | `VerifyMagicLinkRequest { token }` | `LoginResponse` |

**数据模型：**

```kotlin
@Serializable
data class OrgAuthConfig(
    @SerialName("auth_type") val authType: String,   // "password" | "magic_link" | "sso"
    @SerialName("sso_url") val ssoUrl: String? = null,
    @SerialName("org_name") val orgName: String? = null
)

@Serializable
data class MagicLinkRequest(val email: String)

@Serializable
data class MagicLinkResponse(val status: String)

@Serializable
data class VerifyMagicLinkRequest(val token: String)
```

### 2.6 SSO 登录

**流程：**

```
1. 用户输入 email → org-lookup 返回 auth_type == "sso"
2. 使用 Chrome Custom Tabs 打开 sso_url（外部身份提供商页面）
3. SSO 认证完成后回调: https://app.mistyislet.com/sso/callback?token=xxx
4. Android App Links 拦截回调 → DeepLinkHandler 提取 token
5. 调用 POST /app/auth/magic-link/verify { token } 验证（复用同一接口）
6. 验证成功 → 存入 TokenStore
```

**依赖：** `androidx.browser:browser:1.8.0`（Chrome Custom Tabs）

**App Links 要求：** 需要在 `https://app.mistyislet.com/.well-known/assetlinks.json` 部署数字资产链接文件。

### 2.7 LoginViewModel 状态机

登录 UI 使用 `AuthStep` 枚举驱动多步骤流程：

```
EmailInput → (submitEmail) → OrgLookupLoading → 根据 auth_type 分支:
  ├─ "password"    → PasswordInput
  ├─ "magic_link"  → MagicLinkSent
  └─ "sso"         → SSORedirect (打开 Custom Tabs)
```

**文件：** `ui/login/LoginViewModel.kt`

---

## 3. Retrofit 接口清单

### 3.1 AuthApi — 认证 (6 方法)

**文件：** `data/api/AuthApi.kt`

| 方法 | 路径 | 请求体/参数 | 响应 |
|------|------|-------------|------|
| POST | `app/auth/login` | `LoginRequest` | `LoginResponse` |
| POST | `app/auth/refresh` | `RefreshRequest` | `RefreshResponse` |
| POST | `app/auth/restore-password` | `RestorePasswordRequest` | Unit |
| GET | `app/auth/org-lookup` | `?domain=` | `OrgAuthConfig` |
| POST | `app/auth/magic-link` | `MagicLinkRequest` | `MagicLinkResponse` |
| POST | `app/auth/magic-link/verify` | `VerifyMagicLinkRequest` | `LoginResponse` |

### 3.2 UserApi — 用户 (5 方法)

**文件：** `data/api/UserApi.kt`

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/me` | — | `UserInfo` |
| GET | `app/me/logins` | — | `UserLoginListResponse` |
| DELETE | `app/me/logins/{loginId}` | — | Unit |
| POST | `app/me/change-password` | `ChangePasswordRequest` | Unit |
| POST | `app/me/avatar` | `MultipartBody.Part` | `UserInfo` |

### 3.3 AccessApi — 门禁操作 (8 方法)

**文件：** `data/api/AccessApi.kt`

| 方法 | 路径 | 参数 | 响应 |
|------|------|------|------|
| GET | `app/access/my-doors` | — | `ListResponse<AccessibleDoor>` |
| POST | `app/access/unlock` | `UnlockRequest` | `UnlockResponse` |
| POST | `app/access/qr-unlock` | `QRUnlockRequest` | `UnlockResponse` |
| GET | `app/access/ble-token` | — | `BleTokenResponse` |
| GET | `app/access/pin-code` | — | `PinCodeResponse` |
| GET | `app/access/logs` | offset, limit | `ListResponse<AccessLog>` |
| GET | `app/visitor-passes` | offset, limit | `ListResponse<VisitorPass>` |
| POST | `app/visitor-passes` | `CreateVisitorPassRequest` | `VisitorPass` |

### 3.4 PlaceApi — 场所 & 多租户 (16 方法)

**文件：** `data/api/PlaceApi.kt`

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/orgs` | — | `List<Organization>` |
| GET | `app/orgs/{orgId}/places` | — | `List<Place>` |
| GET | `app/places/{placeId}/doors` | — | `ListResponse<AccessibleDoor>` |
| GET | `app/places/{placeId}/doors/search` | ?q= | `ListResponse<AccessibleDoor>` |
| POST | `app/places/{placeId}/doors/{doorId}/unlock` | `UnlockRequest` | `UnlockResponse` |
| PUT | `app/places/{placeId}/doors/{doorId}/favorite` | — | Unit |
| DELETE | `app/places/{placeId}/doors/{doorId}/favorite` | — | Unit |
| POST | `app/places/{placeId}/lockdown` | — | Unit |
| DELETE | `app/places/{placeId}/lockdown` | — | Unit |
| GET | `app/places/{placeId}/visitor-groups` | — | `List<VisitorGroup>` |
| GET | `.../{groupId}/members` | — | `List<VisitorGroupMember>` |
| GET | `.../{placeId}/events/{eventId}/media` | — | `List<EventMedia>` |
| GET | `.../{doorId}/restrictions` | — | `PaginatedResponse<DoorRestriction>` |
| GET | `.../{doorId}/schedules` | — | `PaginatedResponse<DoorSchedule>` |
| POST | `.../{groupId}/cleanup-expired` | — | Unit |

### 3.5 CredentialApi — 钱包凭证 (1 方法)

**文件：** `data/api/CredentialApi.kt`

| 方法 | 路径 | 响应 |
|------|------|------|
| GET | `app/credentials` | `ListResponse<Credential>` |

### 3.6 MobileCredentialApi — BLE 凭证 (4 方法)

**文件：** `data/api/MobileCredentialApi.kt`

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| POST | `app/credentials/register` | `RegisterMobileCredentialRequest` | `Response<RegisterMobileCredentialResponse>` |
| GET | `app/credentials/mobile` | — | `ListResponse<MobileCredential>` |
| DELETE | `app/credentials/mobile/{credentialId}` | — | `Response<RevokeCredentialResponse>` |
| POST | `app/credentials/mobile/{credentialId}/refresh` | — | `Response<RegisterMobileCredentialResponse>` |

**注册 Body：**

```json
{
  "public_key_pem": "-----BEGIN PUBLIC KEY-----\n...",
  "platform": "android",
  "device_id": "<Build.BOARD>_<Build.FINGERPRINT.hashCode>",
  "device_model": "<Build.MANUFACTURER> <Build.MODEL>",
  "keystore_level": "strongbox",
  "attestation_cert_chain": ["base64...", ...]
}
```

### 3.7 AdminApi — 管理后台 (68 方法)

**文件：** `data/api/AdminApi.kt`（384 行）

#### 事件 & 监控

| 方法 | 路径 | 响应 |
|------|------|------|
| GET | `app/places/{placeId}/events` | `PaginatedResponse<AdminEvent>` |
| GET | `app/places/{placeId}/incidents` | `PaginatedResponse<AdminIncident>` |
| GET | `app/places/{placeId}/activity` | `PaginatedResponse<LiveActivityRecord>` |

#### 用户管理

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/places/{placeId}/users` | — | `PaginatedResponse<AdminUser>` |
| POST | `.../{placeId}/users/invite` | `InviteUserRequest` | `AdminUser` |
| PATCH | `.../{userId}/role` | `UserRoleUpdateRequest` | `AdminUser` |
| POST | `.../{userId}/sign-out` | — | Unit |
| DELETE | `.../{userId}` | — | Unit |

#### 组 CRUD

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/places/{placeId}/groups` | — | `PaginatedResponse<AdminGroup>` |
| POST | `.../{placeId}/groups` | `CreateGroupRequest` | `AdminGroup` |
| DELETE | `.../{groupId}` | — | Unit |
| GET | `.../{groupId}/members` | — | `PaginatedResponse<GroupMember>` |
| POST | `.../{groupId}/members` | `AssignMemberRequest` | `GroupMember` |
| DELETE | `.../{memberId}` | — | Unit |
| GET | `.../{groupId}/doors` | — | `PaginatedResponse<GroupDoor>` |
| POST | `.../{groupId}/doors` | `AssignDoorRequest` | `GroupDoor` |
| DELETE | `.../{doorId}` | — | Unit |

#### 团队 CRUD

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/places/{placeId}/teams` | — | `PaginatedResponse<AdminTeam>` |
| POST | `.../{placeId}/teams` | `CreateTeamRequest` | `AdminTeam` |
| DELETE | `.../{teamId}` | — | Unit |
| GET | `.../{teamId}/members` | — | `PaginatedResponse<TeamMember>` |
| POST | `.../{teamId}/members` | `AssignMemberRequest` | `TeamMember` |
| DELETE | `.../{memberId}` | — | Unit |
| GET | `.../{teamId}/access-rights` | — | `PaginatedResponse<TeamAccessRight>` |
| POST | `.../{teamId}/access-rights` | `AssignAccessRightRequest` | `TeamAccessRight` |
| DELETE | `.../{rightId}` | — | Unit |

#### 时间表 CRUD

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/places/{placeId}/schedules` | — | `PaginatedResponse<AdminSchedule>` |
| POST | `.../{placeId}/schedules` | `ScheduleWriteRequest` | Unit |
| PUT | `.../{scheduleId}` | `ScheduleWriteRequest` | Unit |
| DELETE | `.../{scheduleId}` | — | Unit |

#### 区域

| 方法 | 路径 | 响应 |
|------|------|------|
| GET | `app/places/{placeId}/zones` | `PaginatedResponse<AdminZone>` |

#### 告警

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/alarms` | — | `PaginatedResponse<Alarm>` |
| PATCH | `app/alarms/{alarmId}/status` | `AlarmStatusUpdateRequest` | `Alarm` |
| GET | `app/alarm-schedules` | — | `PaginatedResponse<AlarmSchedule>` |
| GET | `app/alarm-schedules/calendar` | ?from, ?to | `PaginatedResponse<AlarmCalendarEntry>` |

#### 访客登记

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/places/{placeId}/guests` | — | `PaginatedResponse<GuestVisit>` |
| POST | `.../{placeId}/guests` | `CreateGuestRequest` | `GuestVisit` |
| PATCH | `.../{guestId}` | `GuestCheckInRequest` | `GuestVisit` |
| DELETE | `.../{guestId}` | — | Unit |

#### 预订

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/bookable-spaces` | — | `PaginatedResponse<BookingSpace>` |
| GET | `app/bookings` | ?space_id | `PaginatedResponse<Booking>` |
| POST | `app/bookings` | `CreateBookingRequest` | `Booking` |
| POST | `app/bookings/{id}/cancel` | — | `Booking` |
| POST | `app/bookings/{id}/check-in` | — | `Booking` |
| POST | `app/bookings/{id}/check-out` | — | `Booking` |

#### 卡片 & 数字凭证

| 方法 | 路径 | 响应 |
|------|------|------|
| GET | `app/places/{placeId}/cards` | `PaginatedResponse<AdminCard>` |
| DELETE | `.../{cardId}` | Unit |
| GET | `app/places/{placeId}/credentials` | `PaginatedResponse<AdminDigitalCredential>` |
| POST | `.../{credentialId}/revoke` | Unit |

#### 摄像头

| 方法 | 路径 | 响应 |
|------|------|------|
| GET | `app/cameras` | `PaginatedResponse<Camera>` |
| GET | `app/cameras/{id}/video-link` | `CameraVideoLink` |
| POST | `app/cameras/{id}/snapshot` | `CameraSnapshotResponse` |
| PATCH | `app/cameras/{id}` | `Camera` (rename) |

#### 数据分析

| 方法 | 路径 | 参数 | 响应 |
|------|------|------|------|
| GET | `.../{placeId}/analytics/summary` | ?days=30 | `AnalyticsSummary` |
| GET | `.../{placeId}/analytics/presence` | ?days=30 | `List<UserPresenceRecord>` |
| GET | `.../{placeId}/analytics/failed-attempts` | ?days=30 | `PaginatedResponse<FailedAttemptEvent>` |
| POST | `.../{placeId}/reports/export` | `ReportExportRequest` | `ReportExportResponse` |

#### 组织设置

| 方法 | 路径 | 请求体 | 响应 |
|------|------|--------|------|
| GET | `app/orgs/{orgId}/settings` | — | `OrgSettings` |
| PUT | `app/orgs/{orgId}/settings` | `OrgSettingsUpdateRequest` | `OrgSettings` |

#### 硬件重命名

| 方法 | 路径 | 请求体 |
|------|------|--------|
| PATCH | `app/cameras/{cameraId}` | `RenameRequest` |
| PATCH | `app/places/{placeId}/doors/{doorId}` | `RenameRequest` |
| PATCH | `app/gateways/{gatewayId}` | `RenameRequest` |

---

## 4. 响应包装器

### 4.1 列表响应

Android 使用两种列表包装：

```kotlin
// 简单列表
@Serializable
data class ListResponse<T>(val items: List<T>)

// 分页列表（后台 admin 接口）
@Serializable
data class PaginatedResponse<T>(val items: List<T>)
```

两者结构相同，区别在语义。后端返回格式：

```json
{ "items": [...], "total": 42 }
```

### 4.2 错误响应

```kotlin
@Serializable
data class ApiError(
    val error: String? = null,
    val message: String? = null,
    val code: Int? = null,
    val status: String? = null
)
```

---

## 5. 错误处理

### 5.1 ApiResult 封装

**文件：** `core/network/ApiResult.kt`

```kotlin
sealed class ApiResult<out T> {
    data class Success<T>(val data: T) : ApiResult<T>()
    data class Error(val code: Int, val message: String) : ApiResult<Nothing>()
    data class Exception(val throwable: Throwable) : ApiResult<Nothing>()
}
```

### 5.2 safeApiCall 包装

所有 Repository 通过 `safeApiCall` 统一异常处理：

| 异常类型 | 映射结果 |
|----------|----------|
| `HttpException` | `ApiResult.Error(code, message)` |
| `SerializationException` | `ApiResult.Error(0, "Service unavailable")` |
| 其他异常 | `ApiResult.Exception(throwable)` |

### 5.3 HTTP 状态码

| 状态码 | 处理 |
|--------|------|
| 200-299 | `ApiResult.Success` |
| 401 | AuthInterceptor 自动 refresh → 重试 |
| 其他 4xx/5xx | `ApiResult.Error` |
| 网络错误 | `ApiResult.Exception` |

---

## 6. Repository 层

### 6.1 架构

```
ViewModel → Repository → Retrofit Api → OkHttp → Backend
                ↕
            Room DB (缓存)
```

### 6.2 Repository 清单

| Repository | Retrofit 接口 | 缓存 | 说明 |
|------------|---------------|------|------|
| `AuthRepository` | AuthApi | 无 | 登录 / refresh / 重置密码 / 组织查询 / Magic Link / SSO |
| `DoorRepository` | AccessApi | Room `CachedDoor` | 门禁列表 + 开锁 + 地理围栏同步 |
| `PlaceRepository` | PlaceApi | 无 | 组织 / 场所 / 场所门禁 / 封锁 |
| `CredentialRepository` | CredentialApi | Room `CachedCredential` | 钱包凭证 |
| `MobileCredentialRepository` | MobileCredentialApi | 无 | BLE 密钥注册 / 吊销 |
| `AccessLogRepository` | AccessApi | Room `CachedAccessLog` | 通行记录 |
| `AdminRepository` | AdminApi | 无 | 68 个管理后台接口 |
| `SelectedPlaceRepository` | DataStore | Preferences | 当前选中组织/场所 |

### 6.3 缓存策略

有 Room 缓存的 Repository 遵循统一模式：

1. `getCached*()` — 返回 `Flow<List<T>>`，实时更新
2. `refresh*()` — 从 API 拉取 → 清除旧缓存 → 插入新数据
3. 无网络时 fallback 到 Room 数据

**Room 数据库：** `mistyislet_db`，3 个表，开启 destructive migration。

---

## 7. Hilt 依赖注入

### 7.1 网络模块

**文件：** `core/network/ApiClient.kt`

```
@Module @InstallIn(SingletonComponent::class)
object ApiClientModule {
    @Provides @Named("auth")   → OkHttpClient (无拦截器)
    @Provides @Named("authRetrofit") → Retrofit (auth 专用)
    @Provides                   → AuthApi
    @Provides                   → OkHttpClient (带 AuthInterceptor)
    @Provides                   → Retrofit (主)
    @Provides                   → AccessApi
    @Provides                   → CredentialApi
    @Provides                   → UserApi
    @Provides                   → MobileCredentialApi
    @Provides                   → PlaceApi
    @Provides                   → AdminApi
}
```

### 7.2 存储模块

| 模块 | 提供 |
|------|------|
| `TokenStoreModule` | `EncryptedTokenStore` |
| `DatabaseModule` | `AppDatabase` + DAO |
| `DataStoreModule` | `DataStore<Preferences>` |

---

## 8. Android 平台特殊事项

### 8.1 FCM 推送注册

**文件：** `core/push/MistyisletMessagingService.kt`

Token 刷新时自动注册：

```
POST app/devices/register (注意：不是 /fcm，是 /register)
Body: {
    "fcm_token": "<token>",
    "device_id": "<Build.BOARD>_<fingerprint_hash>",
    "device_model": "<MANUFACTURER> <MODEL>",
    "platform": "android"
}
```

**推送通道：**

| Channel | 触发事件 |
|---------|----------|
| CHANNEL_SECURITY | `door_held_open` |
| CHANNEL_CREDENTIAL | `credential_updated`, `credential_revoked`, `credential_expiring` |
| CHANNEL_ACCESS | `door_unlocked`, `access_changed`, `access_revoked` |
| CHANNEL_VISITOR | `visitor_arrived` |

### 8.2 BLE 移动凭证

**密钥管理文件：** `core/ble/KeystoreManager.kt`

| 项目 | 规格 |
|------|------|
| 算法 | EC P-256 (secp256r1) |
| 存储 | Android Keystore |
| 安全级别 | 优先 StrongBox (API 28+)，回退 TEE |
| 签名 | SHA256withECDSA |
| 密钥认证 | API 24+ 支持 attestation |
| 用户认证 | 不要求（工厂场景） |

**设备标识生成：**

```kotlin
device_id = "${Build.BOARD}_${Build.FINGERPRINT.hashCode().toUInt()}"
device_model = "${Build.MANUFACTURER} ${Build.MODEL}"
```

### 8.3 QR 码扫描

- CameraX + ML Kit (barcode)
- 扫描 QR → 提取 lockId + qrToken → `POST app/access/qr-unlock`

### 8.4 文件上传

头像使用 Retrofit `@Multipart`：

```kotlin
@Multipart
@POST("app/me/avatar")
suspend fun uploadAvatar(@Part file: MultipartBody.Part): UserInfo
```

### 8.5 无 SSE 流

Android 端**不使用** Server-Sent Events。告警实时推送通过 FCM 实现。
这是与 iOS 的主要差异——iOS 通过 `GET /app/alarms/stream` SSE 接收实时告警。

### 8.6 无 WorkManager

无后台同步任务。所有数据刷新在前台 ViewModel 中触发。

### 8.7 Deep Linking

**文件：** `core/deeplink/DeepLinkHandler.kt`

Android 支持两种 deep link 方案：

#### 自定义 Scheme (`mistyislet://`)

| URI | 目标 |
|-----|------|
| `mistyislet://magic-link?token=xxx` | Magic Link 验证 → 自动登录 |
| `mistyislet://unlock/{doorId}` | 门禁详情 → 开锁 |
| `mistyislet://pass` | 我的通行证 |
| `mistyislet://dashboard` | 管理仪表盘 |
| `mistyislet://profile` | 个人设置 |

#### Android App Links (`https://app.mistyislet.com`)

| URI | 目标 |
|-----|------|
| `https://app.mistyislet.com/sso/callback?token=xxx` | SSO 回调 → 自动登录 |
| `https://app.mistyislet.com/visitor/{token}` | 访客通行证 |

**AndroidManifest 配置：**

```xml
<!-- 自定义 scheme -->
<intent-filter>
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data android:scheme="mistyislet" />
</intent-filter>

<!-- App Links (需要 assetlinks.json) -->
<intent-filter android:autoVerify="true">
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data android:scheme="https" android:host="app.mistyislet.com" />
</intent-filter>
```

**Navigation Compose 集成：** 各 composable route 通过 `navDeepLink { uriPattern = "..." }` 参数声明 deep link，系统自动路由。

**Auth token 处理：** `MainActivity.onNewIntent()` 拦截 magic-link 和 SSO callback intent，通过 `SharedFlow` 传递 token 到 `LoginScreen`，触发自动验证。

**后端要求：** 需要在 `https://app.mistyislet.com/.well-known/assetlinks.json` 部署：

```json
[{
  "relation": ["delegate_permission/common.handle_all_urls"],
  "target": {
    "namespace": "android_app",
    "package_name": "com.mistyislet.app",
    "sha256_cert_fingerprints": ["<release-signing-cert-sha256>"]
  }
}]
```

### 8.8 地理围栏 (Geofencing)

**文件：** `core/geofence/GeofenceManager.kt`, `core/geofence/GeofenceBroadcastReceiver.kt`

**依赖：** `com.google.android.gms:play-services-location:21.3.0`

**机制：**

```
1. DoorRepository.refreshDoors() 拉取门禁列表后 → GeofenceManager.syncGeofences(doors)
2. GeofenceManager 对比当前活跃围栏 vs 新门禁列表，增量更新
3. 用户进入围栏（50m 半径）→ GeofenceBroadcastReceiver 接收 ENTER 事件
4. 发送本地通知 → 点击通知 deep link 到 mistyislet://unlock/{doorId}
```

**参数：**

| 参数 | 值 | 说明 |
|------|------|------|
| 半径 | 50m | GEOFENCE_RADIUS_METERS |
| 触发 | ENTER only | 进入时触发，不监听退出 |
| 上限 | 100 个 | Android 系统限制，超出按距离排序截取最近的 |
| 过滤 | 仅有坐标的门 | `latitude != null && longitude != null` |

**权限：**

```xml
<uses-permission android:name="android.permission.ACCESS_FINE_LOCATION" />
<uses-permission android:name="android.permission.ACCESS_BACKGROUND_LOCATION" />
```

**通知：** 使用已有的 `CHANNEL_ACCESS` 通道，标题/内容通过 `R.string.geofence_notification_title/body` 本地化（支持中文、印尼语）。

**生命周期：**
- 门禁刷新时自动同步围栏
- 用户关闭地理围栏开关 → `GeofenceManager.clearAll()`
- 用户登出 → `GeofenceManager.clearAll()`

**后端要求：** `GET /app/access/my-doors` 的 `AccessibleDoor` 响应需包含 `latitude` 和 `longitude` 字段（可选，Double 类型）。

---

## 9. iOS vs Android 差异对照

| 功能 | iOS | Android |
|------|-----|---------|
| HTTP 客户端 | URLSession | OkHttp + Retrofit |
| JSON 解码 | Codable | kotlinx.serialization |
| Token 存储 | Keychain | EncryptedSharedPreferences |
| 401 防竞争 | Actor (TokenRefreshLock) | Mutex |
| 推送注册 | `POST /app/devices/apns` | `POST /app/devices/register` |
| 推送平台 | APNS | FCM |
| 告警实时 | SSE (`/app/alarms/stream`) | FCM 推送 |
| 密钥存储 | Secure Enclave | Android Keystore (StrongBox/TEE) |
| 离线缓存 | SwiftData | Room |
| Magic Link | ✅ 支持 | ✅ 已实现 (org-lookup + magic-link + verify) |
| SSO 登录 | ✅ 支持 | ✅ 已实现 (Chrome Custom Tabs + App Links 回调) |
| Deep Linking | ✅ Universal Links | ✅ 已实现 (自定义 scheme + App Links) |
| 地理围栏 | ✅ CoreLocation | ✅ 已实现 (GeofencingClient, 50m ENTER) |
| 组织切换 | ✅ `POST /orgs/{id}/switch` | ❌ 未实现 |
| NFC 卡绑定 | ✅ `POST /credentials/nfc` | ❌ 未实现 |
| Wallet Pass 管理 | ✅ suspend/activate/revoke | ❌ 未实现 |
| 组 PATCH (更新) | ✅ `PATCH .../groups/{id}` | ❌ 只有 CREATE/DELETE |
| Bookable Space Status | ✅ `GET .../spaces/{id}/status` | ❌ 未调用 |

---

## 10. 关键源文件索引

| 文件 | 用途 |
|------|------|
| `data/api/AuthApi.kt` | 认证接口 (6 方法) |
| `data/api/UserApi.kt` | 用户接口 (5 方法) |
| `data/api/AccessApi.kt` | 门禁接口 (8 方法) |
| `data/api/PlaceApi.kt` | 场所接口 (16 方法) |
| `data/api/CredentialApi.kt` | 钱包凭证 (1 方法) |
| `data/api/MobileCredentialApi.kt` | BLE 凭证 (4 方法) |
| `data/api/AdminApi.kt` | 管理后台 (68 方法) |
| `core/network/AuthInterceptor.kt` | Token 拦截 & 401 刷新 |
| `core/network/ApiClient.kt` | Hilt 网络模块 |
| `core/network/ApiResult.kt` | 统一结果封装 |
| `core/storage/TokenStore.kt` | 加密 Token 存储 |
| `core/storage/AppDatabase.kt` | Room 数据库 |
| `core/push/MistyisletMessagingService.kt` | FCM 推送服务 |
| `core/ble/BLEAuthClient.kt` | BLE 通信 |
| `core/ble/KeystoreManager.kt` | 密钥管理 |
| `data/repository/AuthRepository.kt` | 认证仓库 (登录/refresh/org-lookup/magic-link/SSO) |
| `data/repository/DoorRepository.kt` | 门禁仓库 (含缓存 + 地理围栏同步) |
| `data/repository/PlaceRepository.kt` | 场所仓库 |
| `data/repository/AdminRepository.kt` | 管理后台仓库 |
| `data/repository/MobileCredentialRepository.kt` | BLE 凭证仓库 |
| `data/repository/AccessLogRepository.kt` | 通行记录仓库 (含缓存) |
| `data/repository/CredentialRepository.kt` | 钱包凭证仓库 (含缓存) |
| `core/deeplink/DeepLinkHandler.kt` | Deep Link 解析 (magic-link / SSO / 导航) |
| `core/geofence/GeofenceManager.kt` | 地理围栏管理 (同步/清除/差量更新) |
| `core/geofence/GeofenceBroadcastReceiver.kt` | 地理围栏事件接收 → 本地通知 |
| `domain/model/ApiModels.kt` | 认证数据模型 (OrgAuthConfig 等) |
| `domain/model/AdminModels.kt` | 管理后台数据模型 (70+) |
