# MistyPass 项目全面代码审查报告（第二轮）

> 审查日期：2026-06-10
> 审查范围：后端 (api)、前端 (web-admin)、部署/CI (deploy + .github)、移动端两仓库 (android-MistyisletPass / ios-MistyisletPass，轻量)、Kisi 差距复核
> 基线：上一轮审查 `docs/CODE-REVIEW-2026-05-01.md`（8.1/10），基线 commit `9e008216e`
> 本轮 HEAD：`03536c5`（另含 1 处未提交改动：middleware_request_log 查询参数脱敏）
> 审查方法：6 路并行专项审查（后端 / 前端 / 部署 CI / Kisi 差距核对 / Kisi 线上调研 / 移动端），所有 finding 均经代码逐行核实
> 综合评分：**8.3 / 10**（上轮 8.1）

---

## 一、总览

基线以来 **201 个 commit、501 个文件、+171,141 / −42,656 行**，新增模块：OAuth2 API 客户端、报表 PDF/邮件/调度、Cloudflare 双向邮件、FCM 推送、SCIM 2.0、Bookings、摄像头真集成（5 厂商 + HikConnect 云）、Wallet DLQ 治理、移动管理员 API（约 80 条路由）、southbound 设备直控、Lark 集成、Mac mini staging 部署。

**质量守住的方面**：
- 上一轮全部高危遗留已修复（见第五节）
- `go build` / `go vet` / `tsc --noEmit` 全绿；diff 中无新增 `any`、无 `dangerouslySetInnerHTML`
- 新增移动端 handler 的租户隔离纪律一致（一律使用 token 派生的 `user.TenantID`，不信任请求体 tenant_id）；新 SQL 全部参数化
- 密钥处理总体正确：异步发送统一 `context.Background()+WithTimeout`、多处常数时间比较、前端 secret 不落 localStorage/持久缓存

**失分的方面**：两个"功能交付了但实际不工作"的后端 P1（OAuth2、报表调度器），均为零测试/弱测试区域；iOS 推送断链；i18n 大面积缺失；replay-soak CI 性能守卫被调参掏空。

评分理由：交付量极大且基础纪律保持（+），上轮高危全清（+）；但 P1 数量从 0 升到 4、且两个出自零测试的新功能（−）。

---

## 二、P1（4 项）

### P1-1 OAuth2 整条链路实际不可用，scope 形同虚设
- 位置：`api/internal/http/routes_oauth2.go:594-646`、`api/internal/http/router_middleware.go:692-728`
- 问题：`oauth2IssueAccessToken` 用 `oauth2AccessTokenClaims` 签发 token，该结构体没有 `token_type` 字段；而所有受保护路由走 `withBearerToken → VerifyAccessToken → parseTokenClaims(token,"access")`（auth/service.go:1509 强制 `claims.TokenType == "access"`），OAuth2 token 直接被判 "invalid token type"。授权码换出的 access token **无法访问任何 API**。同时 `scopes` 在全代码库无任何 handler/中间件读取或强制，read/write/admin scope 形同虚设。694 行新代码**零测试**。
- 建议：OAuth2 token 复用 auth 服务签发（带 `token_type:"access"` + client_id/scope claims），或新增专用 OAuth2 验证中间件并实现 `requireScope`；补 token 端点端到端测试。
- 影响：Kisi 差距清单中 "OAuth2 API 认证" 一项在修复前不能算完成。

### P1-2 定时报表调度器对所有新建 schedule 永不触发
- 位置：`api/internal/http/routes_report_schedule.go:617-633, 722-744`
- 问题：`runScheduledReports` 在 `NextRunAt == ""` 时 `continue`；但 `createReportSchedule` / `updateReportSchedule` 从不设置 `NextRunAt`，唯一写入点在 `executeScheduledReport` 内部（只对"已到期"的执行）。新建 schedule 的 `NextRunAt` 恒为空 → 永不进入 due 列表 → **自动定时邮件一封不发**（仅手动 `POST .../send` 可用）。
- 建议：create/update 时按 frequency 计算并写入首个 `NextRunAt`；补一条"新建后到点触发"的测试。

### P1-3 iOS 推送端到端不可用（ios-MistyisletPass 仓库）
- 位置：`MistyisletPass/MistyisletPass.entitlements`（缺 `aps-environment`）；project.yml 无 push capability
- 问题：`NotificationService.swift:18` 注册、`APIService.swift:419` 后端上报、服务端 `/app/devices/apns` 路由全部就绪，但缺 entitlement 导致真机 APNs 注册必然失败，且错误被 `didFailToRegisterForRemoteNotificationsWithError` 静默吞掉。
- 建议：补 entitlement + project.yml capability + 注册失败日志；同时需在 Apple Developer portal 开启 Push 能力并更新 provisioning profile。

### P1-4 前端 99 个新增 locale key 只有中文（印尼市场体验级问题）
- 位置：`web-admin/src/locales/en-US.json`、`id-ID.json`（对照 `zh-CN.json`）
- 问题：基线后新增的 99 个 key（`cameras.*` 48、`visitors.*` 42、`nav.*` 7、`kisi.eventHistory.*` 2）只写入 zh-CN；`i18n.ts:25` 的 `fallbackLng: "zh-CN"` 使英语/印尼语用户在左侧导航 7 个条目及摄像头、访客两个整页看到中文。
- 建议：补齐 en-US / id-ID 两语种；新增 locale key 对齐单测防回归（本次正是因为没有该测试而漏网）。

---

## 三、P2

### 后端
| # | 标题 | 位置 | 说明 |
|---|---|---|---|
| B-1 | 请求日志脱敏可被键名 URL 编码绕过（**未提交改动**） | `api/internal/http/middleware_request_log.go:88-98` | 上传 handler 用 `r.URL.Query()`（解码 `%73ig`→`sig`）读取参数，脱敏却按原始字面量匹配键名；`?%73ig=…` 可让签名 HMAC 原文落日志。**提交前先修**：比对前 `url.QueryUnescape`（或解析为 `url.Values` 重建） |
| B-2 | 移动端创建访客不校验 door_ids 归属 | `api/internal/http/routes_app_guests.go:49-73` | commit 03536c5 将 door_ids 直通 `CreateGuest`，未校验门属于该 place/tenant（对照 `appAdminShareAccess` 有校验）。当前解锁链路尚未消费该字段（惰性元数据），一旦接入授权即成跨租户越权。参考端 `routes_guests.go:82` 同样未校验 |
| B-3 | OAuth2 协议端点无限流 | `api/internal/http/router.go:1408-1413` | `/oauth2/authorize|token|revoke` 不在 `withGlobalAPIRateLimit` 组内也无单独限流；token 端点 bcrypt 校验可被在线暴力/当 CPU 放大面 |
| B-4 | `activate_with_token` 路由参数名错配，必 404 | `router.go:1229` + `routes_reference_gap.go:502` → `routes_reference_api.go:3658` | 路由注册 `{activationToken}`，处理链读 `chi.URLParam(r,"cardID")`，恒取空 ID；无测试覆盖 |
| B-5 | `/organizations/{domain}/public` 重复注册 | `router.go:635` 与 `router.go:651` | 同 method+path 注册两次，Kisi 兼容层 `kisiOrgPublic` 静默覆盖 `getPublicOrganization` |

### 前端
| # | 标题 | 位置 | 说明 |
|---|---|---|---|
| F-1 | Wallet 告警 dispatch/retry 后列表不刷新 | `web-admin/src/features/wallet/hooks/use-wallet-alerts.tsx:329-390` | 成功后仅 `setDispatchSummary`，不 reload 也不合并返回结果；同文件其他操作均有 `loadMetricsAndAlerts`。通知记录卡片状态徽标保持旧值 |
| F-2 | AuditWebhook 面板整段硬编码英文 | `web-admin/src/features/audit/pages/audit-page.tsx:597-724` | 同页其余部分走 `t()`，新增面板全是英文字面量，中英混排 |
| F-3 | 新增 ToggleSwitch 无可访问名称（WCAG 4.1.2） | `audit-page.tsx:639`、`report-schedule-page.tsx:326,423` | 未传 `label` → 无名称 `role="switch"`；同期 `api-clients-page.tsx:308` 是正确示范 |

### 移动端
| # | 标题 | 位置 | 说明 |
|---|---|---|---|
| M-1 | Android 管理台故障时静默展示假数据 | `android …/ui/admin/AdminGatewaysScreen.kt:130-134` 等 10+ 屏 | API Error/Exception 时回退 `AdminDemoData` 且 `_error.value = null`，生产故障显示虚构的门/网关/用户、无任何报错 |
| M-2 | 双端均未接入 create-guest door_ids | iOS `AdminGuestManagementView.swift:410`（硬编码 `[]`）；Android `VisitorPass.kt:7-14`（模型缺字段） | 后端 03536c5 的能力被浪费；双端均无门选择 UI |
| M-3 | 分支管理风险 | — | Android 分支领先 main **61 commits**（含整个 UI parity 重构，且历史有 cherry-pick 重复对）；iOS 领先 7。建议尽快合回 main |

### 部署 / CI
| # | 标题 | 位置 | 说明 |
|---|---|---|---|
| D-1 | replay-soak 性能趋势守卫被调参掏空 | `.github/workflows/api-replay-soak-nightly.yml:56-59`、`docs/testing/curl-pg-replay-multi-state-soak.zsh:87` | 四次放宽叠加：DROP_RATIO_MAX=0.95、LEVEL_MIN_OPS 降至 5、瞬态重试只记录末次延迟（p95 系统性低估）、失败日被从 7 天历史剔除 + `SOAK_REVIEW_STRICT=false`。正确性断言与 noop 延迟上限（4s/7s/10s）仍有效，但作为性能回归守卫已名存实亡。建议 DROP_RATIO 收回 ≤0.7、历史"标记而非删除"、signoff 数据足够后强制 fail-on-hold |
| D-2 | trivy-action 改用可变 `@master` 引用 | `.github/workflows/security-scan.yml:55` | 供应链退化（原 pinned `@0.33.1`）；建议固定 tag 或 SHA |
| D-3 | Mac mini 自动部署无回滚 | `deploy/macmini/update-and-redeploy.zsh:62-80` | 健康检查失败仅 `exit 1`，坏版本继续运行；下个周期因 `before==target` 跳过 compose，反复失败直到人工介入。脚本记录了 `before` SHA 但从未用于回退 |

---

## 四、P3（摘要）

- **后端**：Lark 回调在 `LarkVerificationToken` 未配置时完全跳过校验（可伪造 contact.user.* 事件），且明文 token 用 `==` 而非常数时间比较（`routes_integration_lark.go:30-56`；对照入站邮件 webhook 未配置即 503 的安全默认）；报表导出 `place_id` 过滤未生效——PDF 标题写某栋楼、数据是整租户（`routes_report_export.go:62-110`）；入站邮件 webhook 存在裸 secret 直传的弱认证回退路径、签名路径无 message-id 重放去重（`routes_email_inbound_webhook.go:177-187`）；SCIM tenant 解析的 admin 回退为死代码（`routes_scim.go:143-146`）。
- **前端**：API Clients / Reports 新页整页硬编码英文（与 cameras/visitors 只有中文方向相反，双语策略割裂）；`analytics.ts:94,114` 新端点未 URL 编码；`wallet.ts:130,138` items 解包无 `?? []` 守卫；DLQ 钻取硬截断 10 条且计数徽标误导；`query-keys.ts` 集中工厂仅 1 个消费者（准死代码）。
- **部署**：compose 五处默认凭据简化为 `mistypass-dev`（仍仅绑 127.0.0.1，风险低）；gosec 收窄至 medium/medium（理由合理，建议每月跑一次全量参考）；wrangler@latest / gotenberg:8 未固定版本。
- **移动端**：iOS entitlements 含 macOS 模板残留 `com.apple.security.app-sandbox`；Android targetSdk/compileSdk 仍为 35——"Android 17 readiness" 目前只有服务端仓库的文档，App 本体未动。
- **工作区**：`.claire/` 为外部工具 worktree 残留，建议清理或 gitignore。

**移动端安全面（正向确认）**：Android token 存 EncryptedSharedPreferences AES256-GCM、双端证书 pinning 三枚一致（2027-07 过期）、iOS Keychain `WhenUnlockedThisDeviceOnly`、生产 pin fail-closed、未发现硬编码密钥。

---

## 五、上轮遗留核销

| 上轮问题（2026-05-01） | 现状 | 证据 |
|---|---|---|
| （高）Bootstrap token 全局后备认证 | ✅ 已修复 | `authorizeGatewayBootstrapToken`（router_middleware.go:1218-1241）仅 bootstrap 路由调用，`subtle.ConstantTimeCompare`，未配置返回 503 |
| （高）redistore 零测试 | ✅ 已修复 | `redistore/store_test.go` ~1096 行 |
| （高）Wallet provider 零测试 | ✅ 已修复 | apple/google/alert provider 测试齐备 |
| （中）router.go 等巨型文件 | ✅ 大幅改善 | router.go −8382 行，拆出 middleware/gateway/workers 等；enterprise/wallet/access 按子域拆分 |

---

## 六、测试覆盖评价

- **最好**：前端 OAuth2 API clients（e2e 219 行全流程含 secret 一次性展示断言）；Wallet DLQ（role-boundary e2e + api.test 21/21）。
- **零覆盖的新面**：后端 OAuth2 协议层（694 行，恰好埋了 P1-1）；`activate_with_token`（恰好埋了 B-4）；audit webhook 管理 UI（0 单测 0 e2e）；report mail provider status / Send-now 前端流程。
- **结构性缺口**：组件渲染层无单测（现有 11 个前端测试文件全是 utils/api 纯函数）；无 locale key 对齐测试（P1-4 因此漏网）。规律明显：**本轮全部 P1/P2 几乎都出自零测试区域**。

---

## 七、审查盲区与限制

1. **gateway-agent 真源码未深审**：`api/cmd/gateway-agent/` 基线后 10 commits、27 文件、+5258 行（mTLS ECDSA + TLS1.3、WS 写安全、Wiegand GPIO periph.io、Matter chip-tool、OSDP v2、BLE 会话上限、NFC HCE）。抽查无红旗（MinVersion≥TLS1.2、无 token 落日志、exec 仅限操作员配置路径），已另排专项深审。
2. 移动端为轻量审查（仅 2026-05-04 后变更文件），未跑完整构建。
3. Kisi 线上调研截至 2026-06-10（最新月度更新为 2026-05-28）。

---

## 八、行动清单（建议顺序）

1. 修 4 个 P1：OAuth2 token_type + scope 强制 + 测试；报表调度器 NextRunAt；iOS aps-environment；i18n 99 keys（en/id）
2. 提交 middleware 脱敏改动前修 B-1（QueryUnescape）
3. 短期：B-2 door_ids 归属校验、B-3 /oauth2 限流、B-4/B-5 路由 bug、M-1 Android 假数据回退、M-3 移动端分支合并、D-1 replay-soak 收紧、D-2 trivy 固定、D-3 Mac mini 回滚
4. 中期：P3 批量清理（Lark 校验、报表 place 过滤、前端 i18n 统一、组件层测试 + locale 对齐测试）
5. 专项：gateway-agent 深审

> 配套文档：`docs/kisi-gap-analysis.md`（2026-06-10 重写版）
