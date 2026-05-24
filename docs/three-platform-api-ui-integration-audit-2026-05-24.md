# 三端 API / UI 联调一致性审计

> 日期：2026-05-24
> 能力状态：CONTRACT_READY
> 仓库范围：
> - 后端与 Web Admin：`/Users/siky/code/MistyPass`
> - iOS：`/Users/siky/code/ios-MistyisletPass`
> - Android：`/Users/siky/code/android-MistyisletPass`

## 1. 总结

三端主路径已经比较完整：登录、组织/场所、门点、远程开门、访客、团队/组、基础凭证、报警、预约、摄像头、报表导出都能在代码层找到对应调用。2026-05-24 已完成第一轮 P0 止血：mobile OpenAPI 扩展、覆盖测试、iOS Wallet tenant query、Android emulator debug base URL 均已落地。

- `docs/openapi.json` 已更新到 479 个路径，其中 `/api/v1/app/*` 为 128 个。
- `docs/openapi-mobile.json` 已更新到 128 个 mobile 路径。
- 后端实际 `/api/v1/app/*` 路由已由 `TestOpenAPIMobileCoverage` 和生成后的 mobile 文档形成回归约束。
- Web Admin API 模块约 270 个路径，存在一批后端已实现但 OpenAPI 未登记的 Admin extension。
- iOS / Android 仍有大量手写路径，后续继续扩展会越来越容易出现参数、命名、HTTP method 和 base URL 不一致。

优先级建议：

1. P0 已完成：补 mobile OpenAPI / `openapi-mobile.json`，让 iOS / Android 有统一契约基线。
2. P0 已完成：修正 iOS Wallet pass status 操作缺 `tenant_id` 的问题。
3. P0 已完成：修正 Android debug base URL，避免模拟器把 `localhost` 指向自身。
4. P1：补 Web Admin 对 Audit Webhook / OAuth2 Clients / Wallet Google Config / batch DLQ 操作的 UI。
5. P1 已启动：统一邮件 provider 和 report schedule 真实发送配置。

## 2. 后端契约状态

| 契约面 | 当前状态 | 风险 |
|---|---|---|
| `docs/openapi.json` | 已更新到 479 个路径，包含 128 个 `/app` 路径 | Admin extension 仍有补录空间 |
| `docs/openapi-mobile.json` | 已更新到 128 个 mobile 路径 | 后续需持续生成并保持 coverage test 通过 |
| `api/internal/http/router.go` | mobile 路由实际已覆盖多模块 | 新增 mobile 路由必须同步 OpenAPI |
| `web-admin/src/lib/api/*` | 前端调用覆盖大量 Admin API | 部分新 Admin extension 未进入 OpenAPI |

### 2.1 需要补入 mobile OpenAPI 的模块

以下后端路由已在第一轮补入 mobile OpenAPI，并由 coverage test 兜底：

- Auth enhanced：`/app/auth/magic-link`、`/app/auth/org-lookup`、`/app/auth/org/{orgId}/methods`、`/app/auth/sso/{orgId}`、`/app/auth/2fa/*`、`/app/auth/register`、`/app/auth/restore-password`。
- Profile：`PATCH /app/me`、`/app/me/avatar`、`/app/me/change-password`、`/app/me/logins`、`/app/me/primary-device`、`/app/devices/apns`。
- Org / Place：`/app/orgs`、`/app/orgs/{orgId}/switch`、`/app/orgs/{orgId}/places`、`/app/orgs/{orgId}/places/search`、settings。
- Place doors：door list/search/unlock/qr-unlock/favorite/lockdown/restrictions/schedules/rename。
- Place admin：users, events, incidents, activity, schedules, zones, cards, credentials, teams, groups, visitor-groups。
- Operational modules：alarms, alarm-schedules, bookings, bookable-spaces, cameras, analytics, reports export, guests。
- Mobile credentials：NFC list/bind/unbind, QR token, APNS registration。

已新增回归测试：

```bash
cd /Users/siky/code/MistyPass/api
go test ./internal/http -run TestOpenAPIMobileCoverage
```

测试从 mobile OpenAPI 文档和 route registry 约束关键 `/app/*` 路由，防止文档再次明显落后。

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
| P0 已推进 | `/api/v1/audit/webhook/config`, `/deliveries`, `/dispatch` | OpenAPI、curl 文档和 `/audit` 页面业务调用已存在 | 下一步接外部 webhook receiver smoke 与失败重试可视化细节 |
| P1 | `/api/v1/oauth2/clients` | OpenAPI 存在，UI 无调用 | 新增 Developer / API Clients 管理页 |
| P1 | `/api/v1/wallet/google/config`, `/validate` | OpenAPI 与计划文档存在，UI 无调用 | Wallet Advanced 中补 Google Wallet provider config |
| P1 | `/api/v1/wallet/jobs/dlq/requeue`, `/cleanup`, `/process`, `/summary` | API 与测试文档存在，UI 目前主要展示 archives/metrics/alerts | Wallet Queue Ops 补批量治理按钮和确认弹窗 |
| P1 已推进 | `/api/v1/report-schedules/{id}/send`, `/api/v1/report-schedules/provider-status` | API 存在；Report schedule UI 已补 “Send now” 行操作和 provider status 状态条 | 下一步接真实 Resend DNS/key smoke 与回执入库 |
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
| P0 已完成 | `Constants.API.walletPassSuspendPath/ActivatePath/RevokePath` | 调 `/api/v1/wallet/passes/{id}/...` 时没有带 `tenant_id`；后端 `changeWalletPassStatus` 从 query 解析 tenant | 方法已增加 `tenantId` 参数，并改为 `...?tenant_id=` |
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
| P0 已完成 | `app/build.gradle.kts` debug `API_BASE_URL` | `http://localhost:8081/api/v1/` 在 Android 模拟器里通常指向模拟器自身，不是 Mac host | 已改为 `http://10.0.2.2:8080/api/v1/` |
| P1 | Device push | Android 只有 `POST /app/devices/register`，后端另有 iOS APNS `/app/devices/apns`；FCM token 注册语义需确认 | 明确 Android FCM 是否复用 register，或新增 `/app/devices/fcm` |
| P1 | Camera cloud token/recordings | 后端存在但 Android 未接 | Camera 真实集成阶段补 UI |
| P1 | Admin detail routes | 同 iOS，列表页多，详情/相关事件/用户 access-rights 较少 | 按运营场景补详情页 |
| P2 | Generated client | Retrofit path 手写，和后端 route 没有编译期约束 | mobile OpenAPI 完成后生成 Retrofit interface 或至少生成 path constants |

## 6. 三端不一致清单

| 领域 | 后端 | Web Admin | iOS | Android | 结论 |
|---|---|---|---|---|---|
| Mobile OpenAPI | 已生成 128 个 `/app` 路径 | 不依赖 | 依赖手写常量 | 依赖手写 Retrofit | P0 已完成，后续推进 generated client |
| Wallet pass status | 需要 `tenant_id` query | 已带 tenant query | 已修 | 移动端暂未主路径调用 | P0 已完成 |
| Android debug base URL | 本地常用 8080/18080 | N/A | dev 可用 localhost | debug 已指向 `10.0.2.2:8080` | P0 已完成 |
| Audit webhook | API 已有 | 无完整 UI | N/A | N/A | P1 补 Web Admin |
| OAuth2 clients | API 已有 | 无 UI | N/A | N/A | P1 补 Web Admin |
| Wallet Google config | API 已有 | 无 UI | 钱包真实发卡未接 | 未接 | P1/P2 视 LEI 和 Google Wallet 条件推进 |
| Report PDF export | 已合并 | 已接 schedule/export | export 默认值需确认 | Android spec 显示兼容 | P1 修移动端默认/文案 |
| Camera cloud | API 已有 | Admin camera 基础页 | 未接 cloud token/recordings | 未接 cloud token/recordings | P1/P2 与真实摄像头排期合并 |
| Email provider | 已新增统一 `MailProvider` + Resend provider | Report Schedule 已有 provider status UI | N/A | N/A | P1 已推进，下一步接 DNS/回执 |

## 7. 推进计划

### Batch A：契约止血（已完成第一轮）

- [x] 补 `docs/openapi-mobile.json` 至后端实际 `/app/*` 覆盖。
- [x] 新增 route coverage test，防止 OpenAPI 再次落后。
- [x] 修 iOS Wallet pass status tenant query。
- [x] 修 Android debug base URL。
- [ ] 对 iOS/Android 登录、门点、开门、报表导出跑一次 simulator/emulator smoke。

### Batch B：后台 UI 补洞（推荐本周做，2-3 天）

- [x] Audit Webhook 页面。
- OAuth2 Clients / Developer API Clients 页面。
- Wallet Google config 页面。
- Wallet DLQ batch governance 操作按钮。
- [x] Report schedule `Send now` provider 错误提示。
- [x] Report schedule provider health/status 只读展示。

### Batch C：移动端 Admin 深水区（推荐 Batch A/B 后做，3-5 天）

- events/incidents detail + related/occurrences。
- user detail/logins/access-rights/share-access。
- camera cloud token/recordings。
- zone detail、holiday regions。
- 统一 report export 格式选择和 PDF 下载体验。

### Batch D：邮件与回执（已启动）

- [x] `MailProvider` 抽象。
- [x] Resend provider 统一到 report schedule 与 Wallet alert sender。
- [ ] Resend 生产配置与 DNS 验收。
- Cloudflare Email Routing/Workers 入站 webhook。
- 邮件回执关联 report schedule / wallet delivery / enterprise alert。
