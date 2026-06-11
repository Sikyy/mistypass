# Kisi vs MistyPass 全面差距分析

> 更新日期：2026-06-11（前版 2026-06-10 / 2026-05-01，见 git 历史）
> 本地基准：`Kisi-API-Bundled References.yaml`（OpenAPI 3.1.0, 227 operations）；2026-06-10 实测 api.kisi.io/docs 内嵌 spec **仍为 227 operations / 48 tags，无新增 endpoint**
> 线上基准：docs.kisi.io + getkisi.com/updates（已逐月核对 2025-01 ～ 2026-05 全部月度更新；截至本日无 2026-06 更新）
> 代码基准：HEAD `1a73d20`（2026-06-11）
> 配套：`docs/CODE-REVIEW-2026-06-10.md`（审查报告；其行动清单中的 P1/P2 已于 06-10~06-11 全部修复落地）

### 增量记录（2026-06-11）

审查后两天内完成的销项（均 TDD + 全量测试通过，specs 见 `docs/superpowers/specs/`）：

| 销项 | 实现 | commit |
|---|---|---|
| OAuth2 实际可用（P1-1 修复） | token 经 auth 服务签发 + scope 单点强制 + 限流 + e2e | `0159bb4` |
| 报表调度器（P1-2 修复） | NextRunAt 计算/重算/backfill | `f22cc9a` |
| `activate_with_token` bug + 重复路由 | 参数归一 + 路由清理 | `76d388c` `373eff8` |
| **工牌打印** | `/badges/export`（单/批 PDF/HTML）+ 公开 `/badges/verify`（HMAC 签名 QR） | `446a33f..6988919` |
| **Role-Assignment 内置告警策略** | 角色授予/变更触发告警；顺带把内置 incident 策略修成可切换可触发 | `58cad47` |
| **GPS geofence 服务端强制** | UserGroup 圆心 + 解锁路径强制（location_required/geofence_denied，OR-of-paths） | `e2fec90` |
| **空间分析（后端）** | `/analytics/occupancy` + `/analytics/retention` | `4d3b6fb` |
| **访客 NDA** | 租户模板 + 签署（签名图+哈希+审计）+ check-in 强制 | `ef54f32` |
| **Bookings 支付（Midtrans Snap）** | `price_idr` + pending_payment 占位 + webhook 签名结算 | `1a73d20` |

---

## 0. 总览

### 0.1 API Operation 覆盖率（本版口径修正）

> ⚠️ **口径说明**：前版总览宣称"有效覆盖 189/210 = 90%"，与其自身逐操作表不一致（逐表口径为 174/210 ≈ 83%）。本版统一按**逐操作表严格口径**重算，前后期均按同口径对比。

| 指标 | 2026-05-01（同口径重算） | 2026-06-10 |
|------|---:|---:|
| Kisi 总 operations | 227 | 227（无变化，实测确认） |
| 有效（非废弃）operations | 210 | 210 |
| 已覆盖（有效） | 174 (83%) | **185 (88%)** |
| 注册但有 bug（修复即 +1） | 0 | 1（`activate_with_token`，见 1.2） |
| 已覆盖（含废弃兼容） | 190/227 | **201/227 (89%)** |
| 本期净增覆盖 | — | **+11 operations** |
| 剩余缺口构成 | — | 硬件依赖 19 + Kisi 登录模型特有 4 + SCRAM 1 + bug 1 |
| MistyPass 独有 operations | ~150 | **~250+**（移动管理员 ~80 条 + SCIM + OAuth2 + southbound + wallet DLQ 等） |

**结论：纯软件且有业务价值的 API 缺口已基本清零。** 剩余 25 项中 19 项依赖控制器/无线锁硬件，4 项是 Kisi API-key 登录模型特有概念（promote/current-login，MistyPass 为 JWT 模型，低价值），1 项 SCRAM 有 Gateway 离线缓存替代，1 项是待修的参数名 bug。

另：Kisi 已公告旧版 Cards 操作（assign/deassign/activate/deactivate 等 6 项 deprecated）于 **2025-11-30 落日**（docs.kisi.io/platform/apis/deprecations），MistyPass 对应兼容实现可降级维护。

### 0.2 产品功能覆盖率（基于 docs.kisi.io + 2025-26 月度更新）

| 分类 | Kisi | 05-01 | 06-10 | **06-11** | 备注 |
|------|------|------:|------:|------:|------|
| 门禁管理（Places/Locks/Groups/Rights） | ✅ | 100% | 100% | 100% | |
| 电梯管理 | ✅ | 100% | 100% | 100% | |
| 硬件管理（Controllers/Readers/Terminals） | ✅ | 95% | 95% | 95% | Controller I/O 18 项待硬件 |
| 凭证管理 | ✅ | 90% | 90% | 90% | Wallet 端到端仍缺（见 2.2） |
| 用户管理 / 团队角色 | ✅ | 100% | 100% | 100% | Kisi 新增 Custom Roles 试点（见 2.4） |
| 事件和报表 | ✅ | 85% | 95% | **98%** | 调度器 bug 已修；+占用/留存分析端点 |
| 排程和日历 | ✅ | 100% | 100% | 100% | |
| 集成管理 | ✅ | 100% | 100% | 100% | 生态广度仍差（见 2.4） |
| 告警/Incident Policies | ✅ | 70% | 80% | **85%** | +Role Assignment 策略；内置策略改为可切换可触发 |
| 入侵检测 | ✅ | 40% | 40% | 40% | zones/Stay-Away/siren 仍缺 |
| 访客管理 | ✅ | 30% | 50% | **70%** | +NDA（模板/签署/check-in 强制）；剩 Kiosk |
| 视频监控 | ✅（集成路线） | 10% | 80% | 80% | 5 厂商真集成 + HikConnect 云；缺录像回放管理 |
| 预约/Bookings | ✅ | 0% | 80% | **90%** | +Midtrans Snap 支付；缺必签协议/平面图 |
| SCIM 2.0 | ✅ | 0% | 100% | 100% | 完整服务端 + 管理端 + E2E |
| OAuth2 API 认证 | ✅ | 0% | 形式 100% | **100%** | P1-1 已修：token 可鉴权 + scope 强制 + 限流 + e2e |
| 对讲/Intercom | ✅（自研硬件） | 0% | 0% | 0% | |
| 展台/Kiosk | ✅（自研硬件+软件） | 0% | 0% | **软件版 100%** | /kiosk 自助签到（预约查找/走访登记/NDA 签署/check-in）；硬件一体机形态仍 Kisi 独有 |
| 工牌打印 | ✅ | 0% | 0% | **100%** | /badges/export 单/批 PDF + 公开 verify（QR 核验在职状态） |
| Mobile SDK / 白标 | ✅ | 0% | 0% | 0% | |
| Marketplace | ✅ | 0% | 0% | 0% | |
| 空间分析（Occupancy/Visual/Retention） | ✅（2025-26 新增） | — | 0% | **后端 100%** | occupancy/retention 端点已备；前端可视化待做 |
| Security Agents（自动化安全代理） | ✅（2025-06 新增） | — | 0% | 0% | 见 2.4 |

---

## 1. API Operation 覆盖（重构版：分组汇总 + 缺口明细）

> 完整的逐 operationId 映射表见前版（git 历史 2026-05-01）。本版仅展开**有变化或有缺口**的组；标注"✅ 全覆盖"的组与前版一致。

### 1.1 分组汇总

| 资源组 | 覆盖 | 状态 | 本期变化 |
|---|---|---|---|
| Places | 9/9 | ✅ | |
| Locks | 12/12 | ✅ | |
| Floors | 4/4 | ✅ | |
| Users | 15/15 | ✅ | **+deleteCurrentUser** |
| Groups | 5/5 | ✅ | |
| Group 子资源（locks/zones/elevator stops/terminals） | 16/16 | ✅ | **+3 个 detail 端点** |
| Group Links | 3/3 +扩展 | ✅ | |
| Teams | 5/5 | ✅ | |
| Team Memberships | 4/4 | ✅ | **+detail 端点** |
| Roles + Role Assignments | 7/7 | ✅ | |
| Shares / Members（Kisi 已废弃组） | 10/10 | ✅ | 维持兼容 |
| Cards | 9/10 | ✅ | 唯一未实现项为已废弃的 activateCardByToken（2025-11-30 落日，无需补） |
| Card Assignments | 7/8 | ⚠️ | activate_with_token 已注册但有 bug（见 1.2） |
| CSV Imports | 4/4 | ✅ | |
| Controllers / Readers / Terminals | 19/19 | ✅ | |
| **Controller I/O（inputs/relays/wiegands + connections）** | **0/18** | ❌ | 依赖控制器硬件 |
| **Wireless Locks** | **0/1** | ❌ | 依赖硬件 |
| Elevators + Elevator Stops | 12/12 | ✅ | |
| Events / Reports / Scheduled Reports / Schedules / Calendar / Holidays | 22/22 | ✅ | 报表调度器有 P1 bug（CODE-REVIEW P1-2） |
| Integrations / Guests / Presences / Invites / Signed Upload URLs | 12/12 | ✅ | |
| **Cameras** | **6/6** | ✅ | **从 501 桩 → 真实集成**（见 1.3） |
| **Logins** | **6/10** | ⚠️ | +resolveLogin、+createLogin（Kisi 数字 ID 形态兼容层） |
| **SCRAM Offline Certificate** | **0/1** | ❌ | Gateway 离线缓存为替代方案 |
| Organizations | 14/14 | ✅ | **+dashboard、+public、+find** |

### 1.2 剩余缺口明细（25 项）

| operationId | 路径 | 分类 | 说明 |
|---|---|---|---|
| `activateCardAssignmentWithActivationToken` | POST `/card_assignments/{token}/activate_with_token` | **bug 待修** | 路由注册 `{activationToken}`，处理链读 `chi.URLParam("cardID")` → 必 404（router.go:1229 / routes_reference_gap.go:502）；修复 ~0.5h |
| `promoteLogin` | POST `/logins/{id}/promote` | Kisi 登录模型 | primary device 提升；MistyPass 相近概念为 `/app/me/primary-device` |
| `updateCurrentLogin` / `deleteCurrentLogin` / `promoteCurrentLogin` | PUT/DELETE/POST `/login*` | Kisi 登录模型 | API-key login 模型特有；MistyPass 为 JWT+Refresh，已有会话管理等价物，低价值 |
| `fetchOfflineCertificate` | POST `/login/offline_certificate` | Kisi 特有 | SCRAM 离线证书；Gateway 离线缓存 access rules 为替代 |
| Controller Inputs / Relays / Wiegands + 三组 Connections | 18 个 operations | 硬件依赖 | 待控制器硬件（ZKTeco C3 / southbound 路线） |
| `fetchWirelessLocks` | GET `/wireless_locks` | 硬件依赖 | 待无线锁硬件 |

### 1.3 本期新增覆盖（11 项，均已核实路由注册 + handler）

| operationId | MistyPass 证据 |
|---|---|
| `fetchGroupLock` | router.go:1116 → routes_reference_api.go:1588 |
| `fetchGroupElevatorStop` | router.go:1097 → routes_reference_kisi_full.go:244 |
| `fetchGroupTerminal` | router.go:1101 → routes_reference_kisi_full.go:307 |
| `fetchTeamMembership` | router.go:1175 → routes_reference_api.go:2950 |
| `deleteCurrentUser` | router.go:634 → routes_auth.go:203（真删 + 审计） |
| `fetchPublicOrganization` | router.go:651 → routes_kisi_compat.go:19（⚠️ 与 router.go:635 重复注册，后者覆盖前者，待清理） |
| `findOrganizations` | router.go:636 → routes_organization_settings.go:140 |
| `resolveLogin` | router.go:653 → routes_kisi_compat.go:57（功能等价：接受 resolution_token 按普通登录处理） |
| `updateCamera` | router.go:1259 → routes_cameras.go:60（真实现 + 审计） |
| `fetchVideoLink` | router.go:1260 → routes_cameras.go:98（cameraSvc.GetVideoLink，10s 超时） |
| `fetchCurrentOrganizationDashboard` | router.go:1030 → routes_organization_settings.go:61（聚合 users/places/locks/cards/teams/groups counts） |

---

## 2. 产品功能差距

### 2.1 本期从"缺失/部分"翻转为"已覆盖"

| 功能 | 实现 | 证据 |
|---|---|---|
| 报表 PDF 导出 + 邮件推送 + 定时调度 | `GET /reports/export?format=pdf\|csv\|json`（pdfgen + Gotenberg + Mistyislet 设计语言）；report-schedules CRUD + 定时器 + PDF 附件邮件 + provider-status；手动发送已异步化 | routes_report_export.go / routes_report_schedule.go ⚠️ 调度器 P1 bug 修复后才算闭环 |
| Organization Dashboard 聚合端点 | GET /organization/dashboard | routes_organization_settings.go:61 |
| 网络拓扑前端可视化 | network-topology-page.tsx（452 行，树状渲染 + 协议标签） | web-admin/src/features/network/ |
| SCIM 2.0 | /scim/v2 完整（ServiceProviderConfig/Schemas/Users CRUD+filter）+ 管理端 config/token/test/logs + 前端 + E2E mock | routes_scim.go / routes_scim_admin.go |
| OAuth2 API 认证 | /oauth2/authorize+token+revoke + 客户端 CRUD + 前端 + e2e | routes_oauth2.go ⚠️ P1-1 修复后销项 |
| Bookings | bookable-spaces + bookings CRUD + check-in/out + 移动端 /app/bookings + 前端 | routes_bookings.go（缺支付，见 2.2） |
| 视频监控真集成 | 海康 ISAPI / 大华 / ONVIF / VIVOTEK / 中控 5 个 provider + 抓图/快照/发现/测试 + HikConnect 云绑定 + 移动端 + 前端 | api/internal/modules/camera/、modules/hikconnect/ |
| 访客 QR 直入（Tier 3） | Guest `gqr_` AccessToken（24-72h）+ DoorIDs 白名单 + 扫码开门 + notify_host 字段 | routes_guests.go / routes_app_guests.go |

### 2.2 部分覆盖（更新后）

| Kisi 功能 | Kisi 详情 | MistyPass 现状 | 剩余差距 |
|----------|----------|--------------|------|
| **Incident Policies** | 9 类内置（Anti-passback、Door Held Open、Hardware Outage、Impossible Travel、Primary Device Change、Role Assignment、Tailgating、Custom）+ Security Agents 自动化 | 内置 Door Held Open + Hardware Outage + **Role Assignment（06-11，启用后角色授予/变更即派发告警）**，三者均可经 /alert_policies 切换并持久化（routes_incident_alert_policy.go）；impossible-travel/限频为运行时异常检测；自定义条件引擎 | Anti-passback、Tailgating（需进出双向读卡数据，半硬件）；Security Agents 式自动收权 |
| **入侵检测** | 4 报警区域、Stay/Away、报警排程、siren | Alarm CRUD + AlarmSchedule + 移动端告警 + SSE | alarm zones、Stay/Away 模式、siren relay 控制 |
| **访客管理** | Kiosk（含 Kiosk Pro 硬件）、NDA、工牌打印、主人通知、Guest cards | Guests/Visitor Passes/visitor-groups + QR 直入 + notify_host + **NDA（06-11：租户模板/签名图+哈希/审计/check-in 强制）** + **工牌打印（06-11）** + **Kiosk 自助签到（06-11：/kiosk 全屏流程，浏览器端到端验证）** | 通知端到端验证；Kiosk 硬件一体机形态 |
| **访问限制** | GPS geofence 300m、Reader proximity、Primary device、MDM、Tap to Access、**"开门需物理在场"（2025-12 新增）** | **GPS geofence 已服务端强制（06-11）**：UserGroup 圆心+半径，解锁路径缺坐标→location_required、超范围→geofence_denied（OR-of-paths，geofence.go）；primary-device/MDM 字段可配置 + /app/me/primary-device | primary-device / MDM 的服务端强制；Reader proximity |
| **Bookings** | + Stripe 支付、必签协议（2026-05）、平面图选位、App 内预订 | CRUD + 签到 + 移动端 + **Midtrans Snap 支付（06-11：price_idr、pending_payment 占位、webhook 签名结算）** | 必签协议、平面图选位；Xendit 备选 provider |
| **报表/分析** | + Visual analytics、Daily Occupancy、User Retention（2025-12～2026-04 新增） | access-summary / door-activity / 报表 PDF + **occupancy/retention 端点（06-11）** | 前端空间分析可视化、平面图 widget |
| **目录集成** | Google Workspace / Entra ID / Okta / JumpCloud 直连 | Google Workspace 同步 + Lark 集成 + SCIM（覆盖 Entra/Okta/JumpCloud 场景） | Entra/Okta 直连配置向导 + 文档（功能上 SCIM 已可达） |

### 2.3 仍完全缺失

| Kisi 功能 | 优先级 | 备注 |
|----------|------|------|
| Intercom（含 Intercom Pro 自研硬件 + Web/App 接听） | P3 | 硬件绑定；建议第三方门口机集成路线（见第 4 节） |
| ~~Kiosk（软件版）~~ / Kiosk Pro 硬件一体机 | ~~P3~~ / P3 | **软件版 ✅ 已完成（2026-06-11）**：/kiosk 自助签到 + NDA 签名板；剩硬件一体机形态（打印一体），随硬件策略走 |
| ~~Badge Printing~~ | ~~P3~~ | **✅ 已完成（2026-06-11）**：/badges/export + /badges/verify |
| Mobile SDK（白标） | P3 | 有客户需求再抽取 |
| Marketplace（17 类伙伴目录，健身赛道 Mindbody/Magicline/bsport 持续加码） | P3 | 优先做 2-3 个印尼本地集成 |
| SCRAM Offline Certificate | P3 | Gateway 离线缓存为替代 |
| MotionSense 免操作开门 | P2（移动端体验） | 自研可行（见第 4 节） |

### 2.4 Kisi 2025-26 新增观察清单（前版未收录，需持续跟踪）

| 时间 | Kisi 新增 | 对差距分析的影响 |
|---|---|---|
| 2026-05 | Bookings 必签协议；Intercom Pro 促销（$59/月） | Bookings 子差距 +1 |
| 2026-04 | **Visual Analytics**（空间使用可视化）；Tailgating 事件邮件跟进；Magicline 集成 | 新差距项：空间分析 |
| 2026-03 | **Kiosk Pro 硬件**（签到+打印一体）；App 内 Bookings | Kiosk 差距从软件升级为软硬一体 |
| 2026-01～02 | User Retention 报表；bsport/Mindbody 预约自动授权 | 新差距项：留存分析 |
| 2025-12 | Daily Occupancy 报表；"开门需物理在场"限制；Bookings 平面图 | 新差距项 ×3 |
| 2025-11 | Role Assignment incident policy；Web 端 Intercom 呼叫 | 策略类型 +1 |
| 2025-09 | Bookings 正式 GA + Stripe + 日历邀请；Space Activity Hub | 已收录 |
| 2025-06 | **Security Agents**（凭证共享检测、变更追踪、自动收紧权限） | 新差距项：自动化安全代理 |
| 2025-03～05 | Custom Roles（试点）；Guest cards；独立版访客管理（免费层）；平面图 | 关注 RBAC 自定义角色 |
| 2025-04～07 | Intercom Pro 硬件发售 | 硬件代际更新 |
| API | **227 operations 无变化**；旧 Cards 操作 2025-11-30 落日；**Bookings/Intercom/访客/视频 AI 均无公开 API** | Kisi 的 API 覆盖缺口（可作竞争话术：MistyPass 的 bookings/cameras/visitors 都有 API） |
| Wallet | **仍仅支持 Apple Wallet**（无 Google Wallet） | MistyPass Google Wallet 后端优势仍成立 |

---

## 3. MistyPass 独有功能（Kisi 没有）

前版已列（仍有效）：多租户架构、Areas、WebAuthn/Passkey、SSO 联邦（OIDC+SAML）、Enterprise HRIS（Talenta webhook+DLQ）、JIT 审批、同步引擎、告警调度器（自定义条件表达式）、审计 HMAC 链、SSE 事件流、Event Sourcing 回放、Google Wallet、物理卡完整生命周期、Gateway Bootstrap 协议、序列号库存、移动端 App API、实体卡库存治理、权限高级治理（模板/bulk/impact preview）、Holiday Calendar CRUD。

**本期新增**：

| 模块 | 说明 |
|------|------|
| 摄像头直连集成 | 海康/大华/ONVIF/VIVOTEK/中控 5 provider + HikConnect ISC 云绑定（Kisi 走第三方集成，无直连） |
| Southbound 设备直控 | /gateway/southbound/{provider}/{deviceID}/unlock + sync-users + zkteco push |
| 移动管理员 API | ~80 条 place 管理员路由（users/events/incidents/cards/credentials/analytics/guests…），Kisi 移动端无同级管理能力 |
| Gateway 边缘安全栈 | mTLS ECDSA P-256 + TLS1.3 强制 + 证书续期、auth protocol v2（52B challenge）、nonce 重放缓存、NFC HCE APDU、Wiegand GPIO、Matter over Thread、OSDP v2、BLE TCP 模拟器 |
| Wallet DLQ 治理 | requeue/cleanup/archives + metrics/trend/alert 订阅 + 前端钻取 |
| Cloudflare 双向邮件 | 发送 provider（primary）+ 入站 webhook + Worker |
| FCM 推送 | FCM v1 + service account JWT + provider-status/smoke + APNs 注册端点 |
| Kisi 兼容层 | routes_kisi_compat.go（数字 ID 格式，供客户端按 Kisi 形态直连） |
| 移动凭证管理后台 | /credentials/mobile 列表/吊销 + 前端 |
| 审计 Webhook 管理面 | config + deliveries + dispatch + UI |
| PDF 报表引擎 | pdfgen + Gotenberg + 设计语言 |
| 印尼本地化 + 合规 | id-ID locale + UU PDP 合规文档（compliance-uu-pdp-indonesia.md） |
| OAuth2 客户端体系 | 客户端 CRUD + 三协议端点（修复 P1-1 后即为对 Kisi 的纯增项——Kisi 的 OAuth2 仅限官方集成，未开放第三方客户端自助注册） |
| 基建 | React 19 / Vite 8 / Go 1.25.10、replay-soak 夜间 CI、OpenAPI + 移动路由防漂移 guard |

---

## 4. 差距功能的 Kisi 实现方式（自研 vs 第三方）与对策建议

> 置信度标注：【确认】= 官方文档/月度更新直接证实；【高置信】= 多来源一致推断；【推断】= 单一来源或间接推断。

| 差距功能 | Kisi 的做法 | 置信度 | 建议 MistyPass 路线 |
|---|---|---|---|
| **视频监控** | **第三方集成路线**：无自研摄像头/VMS，靠 Spot AI / Cisco Meraki / Eagle Eye / VIVOTEK VORTEX 集成拉流，tailgating AI 检测也依赖集成方 | 【确认】 | 已反超（5 厂商直连，契合印尼市场海康/大华占有率）。补**录像回放**走海康 ISAPI playback / NVR 接口，**不要自研 VMS** |
| **Intercom** | **自研硬件**（Intercom Pro，2025 发售，$59/月订阅）+ 自研呼叫软件（Web/App 接听） | 【确认】 | 不建议自研硬件。集成 SIP/ONVIF 门口机（Akuvox/海康/大华门禁对讲，印尼易采购），复用 southbound 架构；软件侧加 WebRTC/SIP 接听面板 |
| **Kiosk** | 软件自研（iPad/PWA 签到）+ **2026-03 起自研硬件**（Kiosk Pro 签到打印一体机） | 【确认】 | 软件先行：web-admin 出 kiosk PWA（平板锁定模式）+ 市售 Android 平板 + USB 标签打印机；硬件品牌化后置 |
| **工牌打印** | 自研软件功能（工牌生成 + PDF 下载，配普通打印机） | 【高置信】 | **自研，最快可销项**：复用 pdfgen + Gotenberg 出徽章模板，约数天工作量 |
| **访客 NDA** | 自研轻量电子签（访客流程内上传 NDA + kiosk 上签署），未见接 DocuSign 等第三方 | 【推断】 | 自研：NDA 模板上传 + 签名画板 + 归档进审计链（审计 HMAC 链是现成优势） |
| **Bookings 支付** | 预订模块自研 + **第三方 Stripe** 收款；日历邀请自研 ICS | 【确认】 | 支付接**第三方**，但印尼场景建议 Midtrans/Xendit（QRIS/GoPay/OVO）优先于 Stripe——本地化差异点；ICS 邮件链路已有 |
| **Apple Wallet 员工证** | 走 **Apple 官方企业徽章计划**（Apple Wallet access badge + NFC），需 Apple 合作资质，门槛高 | 【确认】 | 短期做**展示型 pass**（QR 条码双钱包）+ 自有 App BLE/HCE 解锁；Apple Access / Google Smart Tap 的 NFC 钱包凭证均为受限合作计划，列长期。我方 Google Wallet 后端已有——**优先把双端 App 的 wallet 入口放出来端到端打通**，这是 Kisi 没有的现成卖点 |
| **MotionSense 免操作开门** | **自研**（App 后台 BLE ranging + 自家 reader 固件配合，专利特性） | 【高置信】 | 自研可行：App 后台 BLE ranging + gateway RSSI 门限 + 防误触（已有 BLE v2 栈与 Android HCE 基础），Android 先行 |
| **SCIM / 目录集成** | SCIM 自研 + 第三方 IdP（Google/Entra/Okta/JumpCloud）官方 API 直连 | 【确认】 | SCIM 已平手。补 Entra/Okta 的 SCIM 配置向导 + 文档即可对外宣称支持，直连后置 |
| **入侵检测（zones/Stay-Away/siren）** | 自研软件模型 + **自家控制器继电器**驱动 siren | 【高置信】 | 软件模型（zones/arm modes/排程联动）纯自研可先行；siren 物理输出走 gateway GPIO / ZKTeco aux 继电器，半硬件绑定 |
| **Incident Policies / Security Agents** | 自研规则引擎跑在自家事件流上 | 【高置信】 | 自研：alert engine 已有，补内置策略类型。注意 Anti-passback/Tailgating 需要进出双向读卡数据，半硬件绑定；Role-Assignment 监控纯软件可即做 |
| **空间分析（Occupancy/Visual/Retention）** | 自研（基于自家事件数据） | 【高置信】 | 自研：presences/events 数据已有，纯报表开发；可与现有 analytics 端点合并出"空间分析"页 |
| **Custom Roles** | 自研 RBAC（2025-03 起试点） | 【确认】 | 自研；建议提前设计 permission matrix，避免现有固定角色未来迁移成本 |
| **Mobile SDK / 白标** | 自研 SDK | 【确认】 | 后置；有白标客户需求时从现有双端 App 抽取 |
| **Marketplace** | 目录自研 + 伙伴 API 集成（健身赛道密集：Mindbody/Magicline/bsport） | 【确认】 | 后置；优先 2-3 个印尼本地集成（Talenta 已有，可加 Mekari 生态 / Midtrans），比泛目录更有销售价值 |
| **Controller I/O** | **自研控制器硬件**（重资产，亦是其毛利来源） | 【确认】 | 维持第三方控制器（ZKTeco C3 等）+ southbound 直控的轻资产路线，正确且已有架构支撑 |

**战略总结**：Kisi 是"自研硬件（重资产）+ 自研软件 + 选择性第三方（支付/视频/IdP）"；MistyPass 资源与市场（印尼）决定了应走"**软件全自研 + 商品化第三方硬件集成**"——southbound/gateway 架构正是为此而建。纯软件可销项（工牌、NDA、kiosk PWA、空间分析、内置策略、geofence 服务端强制、Bookings 本地支付）合计约 4-6 周工作量；硬件绑定项随硬件采购节奏推进。

---

## 5. 缺失汇总（2026-06-11 更新）

### 5.1 纯软件（约 1 天级）
1. ~~修 `activate_with_token` 参数名 bug + 清理重复注册~~ **✅ 已完成（06-10）**
2. `promoteLogin`（2h，可与 /app/me/primary-device 概念对齐）
3. （可选，低价值）updateCurrentLogin / deleteCurrentLogin / promoteCurrentLogin

### 5.2 纯软件功能块（周级，按销项性价比排序）

已完成（2026-06-10 ~ 06-11）：
- ~~工牌打印~~ ✅ · ~~Role Assignment 内置策略~~ ✅ · ~~Geofence 服务端强制~~ ✅
- ~~空间分析（Occupancy/Retention 后端）~~ ✅ · ~~访客 NDA~~ ✅ · ~~Bookings Midtrans 支付~~ ✅
- ~~locale 补齐（en/id 99 keys + parity 测试）~~ ✅

仍待做：
1. ~~Kiosk PWA~~ ✅ 已完成（06-11，/kiosk 自助签到）
2. 空间分析前端可视化（occupancy/retention 数据已有端点）
3. Bookings 必签协议 + 平面图选位
4. Primary device / MDM 的服务端强制（geofence 已做，模式可复用）
5. Anti-passback / Tailgating 策略（需双向读卡数据，半硬件）
6. 入侵检测软件模型（zones / Stay-Away / 排程联动）
7. Wallet 端到端：双端 App 入口恢复 + Google Wallet 打通 + Apple 展示型 pass（移动端仓库）
8. 访客通知端到端验证

### 5.3 依赖硬件（19 operations + 物理能力）
Controller I/O ×18、Wireless Locks ×1、siren 物理输出、Intercom/Kiosk 硬件形态、Apple/Google NFC 钱包凭证（合作计划门槛）。

### 5.4 前置阻塞（来自代码审查）
- ~~OAuth2 P1-1 与报表调度器 P1-2~~ **均已修复（2026-06-10，commit 0159bb4 / f22cc9a），对应两项已正式销项。**

---

## 6. 优先行动建议（2026-06-11 更新）

1. ~~本周：修 P1-1/P1-2 + 路由 bug~~ **✅ 完成**
2. ~~2 周内：工牌打印、Role-Assignment 策略、geofence 服务端强制、locale 补齐~~ **✅ 完成**
3. ~~1 月内~~ → 已完成：空间分析后端、访客 NDA、Bookings Midtrans 支付；**剩**：Kiosk PWA、Wallet 端到端（移动端）
4. **季度**：入侵检测软件模型、MotionSense 等价免操作开门（Android 先行）、Custom Roles 设计、Entra/Okta SCIM 向导、primary-device/MDM 服务端强制
5. **随硬件**：Controller I/O、siren、门口机对讲集成、Kiosk 硬件
6. **持续跟踪**：Kisi 月度更新（getkisi.com/updates）；其 Security Agents 与空间分析是 2026 主推方向
