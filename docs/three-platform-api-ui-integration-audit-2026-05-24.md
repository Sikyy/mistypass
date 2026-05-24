# 三端 API / UI 联调一致性审计

> 日期：2026-05-24
> 能力状态：CONTRACT_READY
> 仓库范围：
> - 后端与 Web Admin：`/Users/siky/code/MistyPass`
> - iOS：`/Users/siky/code/ios-MistyisletPass`
> - Android：`/Users/siky/code/android-MistyisletPass`

## 1. 总结

三端主路径已经比较完整：登录、组织/场所、门点、远程开门、访客、团队/组、基础凭证、报警、预约、摄像头、报表导出都能在代码层找到对应调用。当前最大风险不是“完全没写”，而是契约源不统一：

- `docs/openapi.json` 有 366 个路径，但 `/api/v1/app/*` 只有 16 个。
- `docs/openapi-mobile.json` 也只有同样的 16 个 mobile 路径。
- 后端实际 `/api/v1/app/*` 路由约 100+ 个，包含 auth enhanced、org/place、admin resources、bookings、cameras、alarms、analytics、guests、groups/teams 等。
- Web Admin API 模块约 270 个路径，存在一批后端已实现但 OpenAPI 未登记的 Admin extension。
- iOS / Android 均大量手写路径，后续继续扩展会越来越容易出现参数、命名、HTTP method 和 base URL 不一致。

优先级建议：

1. P0：补 mobile OpenAPI 或生成 `openapi-mobile.json`，让 iOS / Android 不再靠手写常量。
2. P0：修正 iOS Wallet pass status 操作缺 `tenant_id` 的问题。
3. P0：修正 Android debug base URL，避免模拟器把 `localhost` 指向自身。
4. P1：补 Web Admin 对 Audit Webhook / OAuth2 Clients / Wallet Google Config / batch DLQ 操作的 UI。
5. P1：统一邮件 provider 和 report schedule 真实发送配置。

## 2. 后端契约状态

| 契约面 | 当前状态 | 风险 |
|---|---|---|
| `docs/openapi.json` | Admin 合同较全，mobile 只有 16 个 `/app` 路径 | mobile 真实能力缺契约，客户端无法生成 |
| `docs/openapi-mobile.json` | 只有 login/refresh/access/credentials/me/visitor 基础路径 | 与后端实际路由严重落后 |
| `api/internal/http/router.go` | mobile 路由实际已覆盖多模块 | 路由变更不会自动反映到客户端 |
| `web-admin/src/lib/api/*` | 前端调用覆盖大量 Admin API | 部分新 Admin extension 未进入 OpenAPI |

### 2.1 需要补入 mobile OpenAPI 的模块

以下后端路由已存在，但 mobile OpenAPI 未覆盖或覆盖不完整：

- Auth enhanced：`/app/auth/magic-link`、`/app/auth/org-lookup`、`/app/auth/org/{orgId}/methods`、`/app/auth/sso/{orgId}`、`/app/auth/2fa/*`、`/app/auth/register`、`/app/auth/restore-password`。
- Profile：`PATCH /app/me`、`/app/me/avatar`、`/app/me/change-password`、`/app/me/logins`、`/app/me/primary-device`、`/app/devices/apns`。
- Org / Place：`/app/orgs`、`/app/orgs/{orgId}/switch`、`/app/orgs/{orgId}/places`、`/app/orgs/{orgId}/places/search`、settings。
- Place doors：door list/search/unlock/qr-unlock/favorite/lockdown/restrictions/schedules/rename。
- Place admin：users, events, incidents, activity, schedules, zones, cards, credentials, teams, groups, visitor-groups。
- Operational modules：alarms, alarm-schedules, bookings, bookable-spaces, cameras, analytics, reports export, guests。
- Mobile credentials：NFC list/bind/unbind, QR token, APNS registration。

建议新增脚本：

```bash
cd /Users/siky/code/MistyPass/api
go test ./internal/http -run TestOpenAPIMobileCoverage
```

测试应从 router 或 route registry 生成 expected mobile paths，并和 `docs/openapi-mobile.json` 对照。

## 3. Web Admin API / UI 覆盖

### 3.1 已接近完整的模块

| 模块 | 状态 |
|---|---|
| Organization / Places / Spaces / Floors / Areas | API 与 UI 已覆盖主 CRUD |
| Users / Groups / Teams / Access Rights | API 与 UI 已覆盖主流程 |
| Enterprise SSO / SCIM / HRIS / JIT | UI 已进入 Enterprise 页面，API 调用已接入 |
| Wallet templates / passes / deliveries / physical cards / alerts | UI 已大幅接入 |
| Reports / Report schedules / PDF export | PR #94 合并后已具备 PDF 设计语言和发送入口 |
| Southbound | 有独立页面和 API 调用，但 OpenAPI 缺部分 southbound extension |

### 3.2 后端有 API，但 Web Admin 没有完整 UI 的部分

| 优先级 | API | 当前证据 | 建议 |
|---:|---|---|---|
| P0 | `/api/v1/audit/webhook/config`, `/deliveries`, `/dispatch` | OpenAPI 和 curl 文档存在，`web-admin/src` 没有业务调用 | 新增 Integrations -> Audit Webhook 页面 |
| P1 | `/api/v1/oauth2/clients` | OpenAPI 存在，UI 无调用 | 新增 Developer / API Clients 管理页 |
| P1 | `/api/v1/wallet/google/config`, `/validate` | OpenAPI 与计划文档存在，UI 无调用 | Wallet Advanced 中补 Google Wallet provider config |
| P1 | `/api/v1/wallet/jobs/dlq/requeue`, `/cleanup`, `/process`, `/summary` | API 与测试文档存在，UI 目前主要展示 archives/metrics/alerts | Wallet Queue Ops 补批量治理按钮和确认弹窗 |
| P1 | `/api/v1/report-schedules/{id}/send` | API 存在，Report schedule UI 需要确认是否有手动发送按钮 | 补 “Send now” 并显示邮件 provider 状态 |
| P2 | `/api/v1/uploads/*` | OpenAPI 存在，UI 只在局部功能使用或未形成统一入口 | 归入附件/导入控件，不单独做页面 |
| P2 | `/api/v1/temporary-access` | OpenAPI/旧测试仍有痕迹，当前 UI 已避免直接调旧路径 | 若保留产品能力，应在 Access 下补正式入口；否则标记 deprecated |

### 3.3 OpenAPI 缺失但 Web Admin 已调用的 Admin extension

这些不是 UI 缺失，而是合同缺失：

- `/api/v1/enterprise/scim/config`
- `/api/v1/enterprise/scim/token`
- `/api/v1/enterprise/scim/test`
- `/api/v1/enterprise/scim/logs`
- `/api/v1/events/{eventID}/snapshots`
- `/api/v1/gateway/southbound/{provider}/test`
- `/api/v1/gateway/southbound/{provider}/{deviceID}/unlock`
- `/api/v1/gateway/southbound/{provider}/{deviceID}/sync-users`
- `/api/v1/integrations/lark/*`
- `/api/v1/integrations/google-workspace/sync`

建议：把这些 extension 纳入 `docs/openapi.json`，否则后台页面虽然能跑，但 API 文档和外部调用者会看不到。

## 4. iOS API 审计

### 4.1 当前一致的部分

- Base URL 按 `APP_ENV` 分 mock/dev/staging/production，production 指向 `https://api.mistyislet.com/api/v1`。
- 主路径覆盖较广：登录、刷新、magic link、org lookup、org/place、门点列表、开门、收藏、lockdown、访客、群组、团队、报警、预约、摄像头、报表导出、个人资料、NFC、APNS。
- iOS 模拟器使用 `http://localhost:8080/api/v1` 通常可访问 Mac 本机服务，适合本地 smoke。

### 4.2 需要修正

| 优先级 | 位置 | 问题 | 建议 |
|---:|---|---|---|
| P0 | `Constants.API.walletPassSuspendPath/ActivatePath/RevokePath` | 调 `/api/v1/wallet/passes/{id}/...` 时没有带 `tenant_id`；后端 `changeWalletPassStatus` 从 query 解析 tenant | 方法增加 `tenantId` 参数，并改为 `...?tenant_id=` |
| P1 | `APIService.exportReport` | 默认 `format = "csv"`，但新 PDF 报表已成为主路径 | Admin Export UI 若用于 PDF，默认改为 `pdf` 或明确格式选择 |
| P1 | `Constants.API` | 大量路径手写，且有些常量未被 UI 调用 | 等 mobile OpenAPI 补齐后生成 typed client 或生成常量 |
| P1 | 摄像头 cloud token/recordings | 后端有 `/app/cameras/{id}/cloud-token`、`cloud-recordings`，iOS 未接 | 放入 Camera 真实集成排期 |
| P2 | Admin detail routes | events detail/related、incidents detail/occurrences、zone detail、user detail/logins/access-rights/share-access 未完全接 UI | 按移动端 Admin 权限产品范围逐步补 |

## 5. Android API 审计

### 5.1 当前一致的部分

- Retrofit 接口覆盖面和 iOS 基本对齐。
- Android 已接入 favorite doors、events media、admin teams/groups/access-rights、report export、bookings、alarms、camera stream/snapshot。
- `ApiClient` 已把 auth client 与 authenticated client 分开，token refresh 结构合理。

### 5.2 需要修正

| 优先级 | 位置 | 问题 | 建议 |
|---:|---|---|---|
| P0 | `app/build.gradle.kts` debug `API_BASE_URL` | `http://localhost:8081/api/v1/` 在 Android 模拟器里通常指向模拟器自身，不是 Mac host | 改为 `http://10.0.2.2:8080/api/v1/` 或文档要求 `adb reverse tcp:8081 tcp:8080` |
| P1 | Device push | Android 只有 `POST /app/devices/register`，后端另有 iOS APNS `/app/devices/apns`；FCM token 注册语义需确认 | 明确 Android FCM 是否复用 register，或新增 `/app/devices/fcm` |
| P1 | Camera cloud token/recordings | 后端存在但 Android 未接 | Camera 真实集成阶段补 UI |
| P1 | Admin detail routes | 同 iOS，列表页多，详情/相关事件/用户 access-rights 较少 | 按运营场景补详情页 |
| P2 | Generated client | Retrofit path 手写，和后端 route 没有编译期约束 | mobile OpenAPI 完成后生成 Retrofit interface 或至少生成 path constants |

## 6. 三端不一致清单

| 领域 | 后端 | Web Admin | iOS | Android | 结论 |
|---|---|---|---|---|---|
| Mobile OpenAPI | 实际约 100+ `/app` 路由 | 不依赖 | 依赖手写常量 | 依赖手写 Retrofit | P0 补契约 |
| Wallet pass status | 需要 `tenant_id` query | 已带 tenant query | 缺 tenant_id | 移动端暂未主路径调用 | P0 修 iOS |
| Android debug base URL | 本地常用 8080/18080 | N/A | dev 可用 localhost | debug 指向 localhost:8081 | P0 修 Android |
| Audit webhook | API 已有 | 无完整 UI | N/A | N/A | P1 补 Web Admin |
| OAuth2 clients | API 已有 | 无 UI | N/A | N/A | P1 补 Web Admin |
| Wallet Google config | API 已有 | 无 UI | 钱包真实发卡未接 | 未接 | P1/P2 视 LEI 和 Google Wallet 条件推进 |
| Report PDF export | 已合并 | 已接 schedule/export | export 默认值需确认 | Android spec 显示兼容 | P1 修移动端默认/文案 |
| Camera cloud | API 已有 | Admin camera 基础页 | 未接 cloud token/recordings | 未接 cloud token/recordings | P1/P2 与真实摄像头排期合并 |
| Email provider | Resend 路径已存在 | 缺 provider status UI | N/A | N/A | P1 统一 MailProvider |

## 7. 推进计划

### Batch A：契约止血（推荐立即做，1-2 天）

- 补 `docs/openapi-mobile.json` 至后端实际 `/app/*` 覆盖。
- 新增 route coverage test，防止 OpenAPI 再次落后。
- 修 iOS Wallet pass status tenant query。
- 修 Android debug base URL。
- 对 iOS/Android 登录、门点、开门、报表导出跑一次 simulator/emulator smoke。

### Batch B：后台 UI 补洞（推荐本周做，2-3 天）

- Audit Webhook 页面。
- OAuth2 Clients / Developer API Clients 页面。
- Wallet Google config 页面。
- Wallet DLQ batch governance 操作按钮。
- Report schedule `Send now` provider 状态与错误提示。

### Batch C：移动端 Admin 深水区（推荐 Batch A/B 后做，3-5 天）

- events/incidents detail + related/occurrences。
- user detail/logins/access-rights/share-access。
- camera cloud token/recordings。
- zone detail、holiday regions。
- 统一 report export 格式选择和 PDF 下载体验。

### Batch D：邮件与回执（推荐和报表发送一起做，1-2 天）

- `MailProvider` 抽象。
- Resend 生产配置与 DNS 验收。
- Cloudflare Email Routing/Workers 入站 webhook。
- 邮件回执关联 report schedule / wallet delivery / enterprise alert。
