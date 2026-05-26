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
| P0 已推进 | `/api/v1/audit/webhook/config`, `/deliveries`, `/dispatch` | OpenAPI、curl 文档和 `/audit` 页面业务调用已存在；已补 `docs/testing/curl-audit-webhook-receiver.zsh` 并接入 API Smoke，覆盖真实 receiver 成功投递、签名校验、delivery 列表和 500→重试→202 成功闭环 | 下一步补失败重试可视化细节或转 Cloudflare Email Service 真实 DNS/API token smoke |
| P1 已推进 | `/api/v1/oauth2/clients` | OpenAPI 存在；Web Admin 已新增 Developer / API Clients 页面；已补 `docs/testing/curl-oauth2-client-crud.zsh` 并接入 API Smoke，覆盖 `OAUTH2_ENABLED=true` 下创建、列表、编辑、删除与 secret 不回显；已补 `web-admin/e2e/api-clients-e2e.spec.ts` 覆盖 UI 创建、编辑、禁用、删除 payload；已补 `docs/testing/curl-oauth2-protocol.zsh` 覆盖 authorize、JSON/form token、code replay、revoke、scope guard、disabled client guard | 下一步转 Cloudflare Email Service 真实 DNS/API token smoke |
| P1 已推进 | `/api/v1/wallet/google/config`, `/validate` | OpenAPI 与计划文档存在；Web Admin Wallet Advanced 已新增 Google Wallet provider config 保存/验证面板 | 下一步在具备 LEI/Google Wallet 条件后跑真实 issuer/key smoke |
| P1 已推进 | `/api/v1/wallet/jobs/dlq/requeue`, `/cleanup`, `/process`, `/summary`, `/jobs/{jobID}/dlq/requeue` | API 与测试文档存在；Wallet Advanced 已补队列处理、DLQ 重排、DLQ 清理、确认提示、summary 展示、错误码 drill-down 与单条 DLQ 重排；本地 API smoke、Wallet role-boundary e2e、带 DLQ fixture 的 action e2e 通过；Wallet 表单 ref/controlled input warning 已清理 | 下一步转 Cloudflare Email Service 真实 smoke |
| P1 已推进 | `/api/v1/report-schedules/{id}/send`, `/api/v1/report-schedules/provider-status` | API 存在；Report schedule UI 已补 “Send now” 行操作和 provider status 状态条；已补 `docs/testing/curl-report-schedule-resend.zsh` 并接入 API Smoke，覆盖 provider status、send now、Gotenberg PDF、Resend PDF 附件、metadata、idempotency key 与 `report_schedule_sent` 审计 | 下一步接 Cloudflare Email Service provider、真实 DNS/API token smoke 与回执入库 |
| P2 | `/api/v1/uploads/*` | OpenAPI 存在，UI 只在局部功能使用或未形成统一入口 | 归入附件/导入控件，不单独做页面 |
| P2 | `/api/v1/temporary-access` | OpenAPI/旧测试仍有痕迹，当前 UI 已避免直接调旧路径 | 若保留产品能力，应在 Access 下补正式入口；否则标记 deprecated |

### 3.3 Web Admin 已调用的 Admin extension 合同

状态：已推进。`docs/openapi.json` 已补齐 Web Admin 正在调用但此前合同缺失的 Admin extension，并在 `router_openapi_test.go` 加了防回退断言：

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

后续建议：继续把 Web Admin 的动态调用点纳入同类合同测试，避免后台页面先跑起来、OpenAPI 再次落后。

## 4. iOS API 审计

### 4.1 当前一致的部分

- Base URL 按 `APP_ENV` 分 mock/dev/staging/production，production 指向 `https://api.mistyislet.com/api/v1`。
- 主路径覆盖较广：登录、刷新、magic link、org lookup、org/place、门点列表、开门、收藏、lockdown、访客、群组、团队、报警、预约、摄像头、报表导出、个人资料、NFC、APNS。
- iOS 模拟器使用 `http://localhost:8080/api/v1` 通常可访问 Mac 本机服务，适合本地 smoke。

### 4.2 需要修正

| 优先级 | 位置 | 问题 | 建议 |
|---:|---|---|---|
| P0 已完成 | `Constants.API.walletPassSuspendPath/ActivatePath/RevokePath` | 调 `/api/v1/wallet/passes/{id}/...` 时没有带 `tenant_id`；后端 `changeWalletPassStatus` 从 query 解析 tenant | 方法已增加 `tenantId` 参数，并改为 `...?tenant_id=` |
| P1 已完成 | `APIService.exportReport` | 默认 `format = "csv"`，但新 PDF 报表已成为主路径 | [iOS PR #5](https://github.com/Sikyy/IOS-mistypass/pull/5) `714d0f6` 已改默认 PDF，并补模型/常量测试 |
| P1 已推进 | `Constants.API` | 大量路径手写，且有些常量未被 UI 调用 | 已新增 mobile route typed constants 生成器与 drift guard；iOS `Constants.API` 已改为从生成的 `MobileAPIRoutes` 取 path，guard 会校验 iOS generated copy 同步 |
| P1 已推进 | 摄像头 cloud token/recordings | 后端有 `/app/cameras/{id}/cloud-token`、`cloud-recordings`，已补 mock-backed 成功路径测试；iOS 已补 cloud token/recordings 入口、空态和错误态；2026-05-24 route guard 发现 cloud-token method 漂移并在 [iOS PR #7](https://github.com/Sikyy/IOS-mistypass/pull/7) 修为 `GET` | [iOS PR #5](https://github.com/Sikyy/IOS-mistypass/pull/5) `a87b1c5` 已接 UI；后续做真实设备 staging 验收 |
| P2 已推进 | Admin detail routes | events detail/related、incidents detail/occurrences、user detail/logins/access-rights/share-access、zone detail/holiday regions 已补 API smoke；events/incidents/user/zone App 接线已完成 | [iOS PR #5](https://github.com/Sikyy/IOS-mistypass/pull/5) `714d0f6` 已接 event/incident detail APIs；`a87b1c5` 已接 user detail/logins/access-rights/share-access、zone detail/holiday regions 和 camera cloud UI |

## 5. Android API 审计

### 5.1 当前一致的部分

- Retrofit 接口覆盖面和 iOS 基本对齐。
- Android 已接入 favorite doors、events media、admin teams/groups/access-rights、report export、bookings、alarms、camera stream/snapshot。
- `ApiClient` 已把 auth client 与 authenticated client 分开，token refresh 结构合理。

### 5.2 需要修正

| 优先级 | 位置 | 问题 | 建议 |
|---:|---|---|---|
| P0 已完成 | `app/build.gradle.kts` debug `API_BASE_URL` | `http://localhost:8081/api/v1/` 在 Android 模拟器里通常指向模拟器自身，不是 Mac host | 已改为 `http://10.0.2.2:8080/api/v1/` |
| P1 已推进 | Device push | Android 统一使用 `POST /app/devices/register` 注册 FCM token；旧 `/app/devices/fcm` 文档漂移已收口 | 后端已补 FCM HTTP v1 provider、token state store 持久化、`/mobile-push/provider-status` 与 `/mobile-push/smoke`；Android 代码已接 Firebase token 注册，剩余是 Firebase Console / `google-services.json` / service account 配置后跑真实 Xiaomi 15 smoke |
| P1 已推进 | Camera cloud token/recordings | 后端存在，已补 mock-backed 成功路径测试；Android 已补 cloud token/recordings 入口、空态和错误态 | [Android PR #11](https://github.com/Sikyy/Android-mistypass/pull/11) `aa63653` 已接 UI；iOS 真机已看到真实 Hikvision 画面/recordings，Android 后续复用同一 staging camera 做 parity smoke |
| P1 已推进 | Admin detail routes | events detail/related、incidents detail/occurrences、user detail/logins/access-rights/share-access、zone detail/holiday regions 已补 API smoke；events/incidents/user/zone App 接线已完成 | [Android PR #11](https://github.com/Sikyy/Android-mistypass/pull/11) `5b841d5` 已接 event/incident detail APIs；`aa63653` 已接 user detail/logins/access-rights/share-access、zone detail/holiday regions 和 camera cloud UI |
| P2 已推进 | Generated client | Retrofit path 手写，和后端 route 没有编译期约束 | 已新增 Kotlin typed route constants，Android Retrofit 注解已切到 `MobileApiRoutes.*RetrofitPath`；guard 会校验 Android generated copy 同步、method/path 漂移和手写 `/app/*` literal |

## 6. 三端不一致清单

| 领域 | 后端 | Web Admin | iOS | Android | 结论 |
|---|---|---|---|---|---|
| Mobile OpenAPI | 已生成 128 个 `/app` 路径、154 个 mobile operations；events/incidents/users/zones/holiday deep routes 已接 API Smoke，camera cloud 已接 mock-backed API test | 不依赖 | events/incidents/user/zone/camera deep UI 已接 [iOS PR #5](https://github.com/Sikyy/IOS-mistypass/pull/5)；typed route constants 已接入 `Constants.API` | events/incidents/user/zone/camera deep UI 已接 [Android PR #11](https://github.com/Sikyy/Android-mistypass/pull/11)；Retrofit 注解已接入 typed route constants | P1 已推进，route drift guard 会校验主仓库生成物、iOS/Android generated copy、路径和 HTTP method |
| Wallet pass status | 需要 `tenant_id` query | 已带 tenant query | 已修 | 移动端暂未主路径调用 | P0 已完成 |
| Android debug base URL | 本地常用 8080/18080 | N/A | dev 可用 localhost | debug 已指向 `10.0.2.2:8080` | P0 已完成 |
| Audit webhook | API 已有 | 已补 `/audit` 配置与投递 UI | N/A | N/A | P1 已推进 |
| OAuth2 clients | API 已有 | 已补 API Clients 页面 | N/A | N/A | P1 已推进 |
| Admin extension OpenAPI | 已补 SCIM、southbound、Lark、Google Workspace、event snapshots 合同 | 相关页面已调用 | N/A | N/A | P1 已推进 |
| Wallet Google config | API 已有 | 已补 Wallet Advanced 配置/验证 UI | 钱包真实发卡未接 | 未接 | P1 已推进；真实发卡仍视 LEI 和 Google Wallet 条件推进 |
| Report PDF export | 已合并 | 已接 schedule/export | 默认 PDF 已接 [iOS PR #5](https://github.com/Sikyy/IOS-mistypass/pull/5) | 默认 PDF 已接 [Android PR #11](https://github.com/Sikyy/Android-mistypass/pull/11) | P1 已完成，后续只剩模拟器/真机下载体验验收 |
| Camera cloud | API 已有，mock-backed 成功路径已测 | Admin camera 基础页 | 已接 cloud token/recordings UI，真实 Hikvision 画面/recordings 已在 iOS 真机显示 | 已接 cloud token/recordings UI，待复用同一 staging camera 做 Android parity smoke | P1/P2 真摄像头主路径已通过 iOS，Android 只剩 parity |
| Email provider | 已新增统一 `MailProvider` + Resend/Cloudflare provider；路线调整为 Cloudflare Email Service 主通道、Resend fallback | Report Schedule 已有 provider status UI | N/A | N/A | P1 已推进，下一步接 Cloudflare DNS/API token 与回执 |

## 7. 推进计划

### Batch A：契约止血（已完成第一轮）

- [x] 补 `docs/openapi-mobile.json` 至后端实际 `/app/*` 覆盖。
- [x] 新增 route coverage test，防止 OpenAPI 再次落后。
- [x] 修 iOS Wallet pass status tenant query。
- [x] 修 Android debug base URL。
- [x] 生成 Swift/Kotlin typed mobile route constants，并让 route drift guard 校验生成物、路径和 Android Retrofit method。
- [x] route drift guard 已升级为禁止 iOS/Android 生产源码新增手写 `/app/*` route literal，必须经 generated constants / `Constants.API` 使用。
- [x] route drift guard 已升级为强制 iOS/Android 仓库里的 generated route copy 和主仓库生成物 byte-for-byte 一致，避免 app 继续拿旧 path 常量。
- [x] iOS simulator 自动化 smoke：`xcodebuild test` 在 iPhone 17 Pro simulator 跑完 176 个测试，0 failure。
- [x] Android 本地 build/unit smoke：`./gradlew testDebugUnitTest` 与 `./gradlew assembleDebug` 通过。
- [x] Android real-device install smoke：2026-05-25 Xiaomi 15 (`d766dd19`, `24129PN74C/dada`) 已安装 staging APK；首次因旧包签名不一致失败，卸载旧 `com.mistyislet.app` 后安装成功。
- [ ] iOS/Android 登录、门点、开门、报表导出做一次 staging 手工走查；2026-05-25 `staging-api.mistyislet.com` healthz、移动端登录、places、doors、PDF report export API smoke 已通过；Android Xiaomi 15 登录/门点/报表导出已跑通，开门需确认安全门点后再触发；iOS 已开 [IOS-mistypass #12](https://github.com/Sikyy/IOS-mistypass/pull/12) 补 `MistyisletPass-Staging` scheme、APP_ENV 解析和 Debug staging widget Bundle ID，simulator 180 tests / 0 failure，`Siky的iPhone` 真机构建、安装、启动已通过。iOS 人工走查已确认门点列表、报表导出 URL 与真实 Hikvision 画面/recordings；移动端报表导出不发邮件，邮件发送属于 report schedule send path；iOS 通行证误显示 Xiaomi 15 Android BLE 凭证的问题已在 PR #12 中按平台过滤并重新安装真机包，2026-05-26 复开已确认 Xiaomi 15 BLE 通行证不再显示。剩余是选择安全门点测试一次开门、Android FCM 真推送 smoke、Android camera parity smoke。步骤见 `docs/testing/mobile-staging-manual-walkthrough-2026-05-24.md`。

本地可重复脚本：

```bash
cd /Users/siky/code/MistyPass
./docs/testing/mobile-app-smoke.zsh
```

注意：iOS simulator smoke 不要传 `CODE_SIGNING_ALLOWED=NO`。该参数会让 Keychain 测试在模拟器里触发 `OSStatus -34018`，默认 `Sign to Run Locally` 签名下测试可正常通过。

### Batch B：后台 UI 补洞（推荐本周做，2-3 天）

- [x] Audit Webhook 页面。
- [x] Audit Webhook 外部 receiver 成功投递与重试 smoke。
- [x] OAuth2 Clients / Developer API Clients 页面。
- [x] OAuth2 Clients `OAUTH2_ENABLED=true` API CRUD smoke。
- [x] OAuth2 Clients Web Admin 表单动作 e2e。
- [x] OAuth2 authorize/token/revoke 协议 smoke。
- [x] Wallet Google config 页面。
- [x] Wallet DLQ batch governance、错误码 drill-down 与单条重排。
- [x] Report schedule `Send now` provider 错误提示。
- [x] Report schedule provider health/status 只读展示。

### Batch C：移动端 Admin 深水区（推荐 Batch A/B 后做，3-5 天）

- [x] events/incidents detail + related/occurrences 后端 API smoke；iOS/Android App 接线已完成。
- [x] user detail/logins/access-rights/share-access 后端 API smoke；iOS/Android App 接线已完成。
- [x] camera cloud token/recordings mock-backed API test；iOS/Android 已补 cloud token/recordings UI，真实设备待 staging 验收。
- [x] zone detail、holiday regions 后端 API smoke；iOS/Android App 接线已完成。
- [x] 统一 report export 默认 PDF：[iOS PR #5](https://github.com/Sikyy/IOS-mistypass/pull/5)、[Android PR #11](https://github.com/Sikyy/Android-mistypass/pull/11) 已接；下载体验待模拟器/真机验收。

### Batch D：邮件与回执（已启动，Cloudflare-first）

- [x] `MailProvider` 抽象。
- [x] Resend provider 统一到 report schedule 与 Wallet alert sender，保留为 fallback。
- [x] Cloudflare Email Service provider 接入 report schedule、Wallet alert 与 invitation email。
- [x] `REPORT_EMAIL_FROM` 已从 invitation sender 拆出；macmini 可复制 `deploy/env/macmini-staging.example.env` 后在本机填真实 token。
- [x] Mac mini staging env 模板与 Compose env 透传已补齐；`deploy/macmini/update-and-redeploy.zsh` 与 launchd 模板可从 `github/main` 自动 fast-forward 后重部署。
- [x] Report schedule Resend mock smoke 覆盖 PDF 附件、metadata、idempotency key 与发送审计。
- [x] Email inbound webhook 后端入口：`POST /api/v1/webhooks/email/inbound` 已补 HMAC 验签、事件列表、state store 落库与 `email_inbound_event_received` 审计，`docs/testing/curl-email-inbound-webhook.zsh` 已接 API Smoke。
- [x] Cloudflare Email Service 生产 DNS/API token 与真实发信 smoke：2026-05-25 Mac mini staging 已完成 Cloudflare Email Sending 启用、普通邀请邮件 smoke 和 report PDF smoke，并确认真实收件。
- [ ] Wallet alert Cloudflare 真收件 smoke。
- [ ] Cloudflare Email Routing/Workers 生产转发：Worker scaffold 已补到 `deploy/cloudflare/email-inbound-worker/`，`mistypass-email-inbound-worker` 已部署并通过健康检查；`mistyislet.com` 当前存在 SpaceMail MX/SPF 记录，生产域名 Email Routing 地址绑定暂挂，避免误删现有收信记录。
- [ ] 邮件回执关联 report schedule / wallet delivery / enterprise alert。
