# MistyPass — 后续实施计划

> 生成日期：2026-04-19
> 前置条件：REFORM-CHECKLIST 76/76 项已全部完成
> 状态说明：⬜ 未启动 | 🔄 进行中 | ✅ 已完成

---

## 一、测试覆盖与质量保障（1-2 周）

**目标：** 补齐自动化测试短板，建立回归防线

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 1.1 | 前端单元测试体系 | 引入 Vitest + Testing Library，配置 `test` script；覆盖核心 hooks（`useAuth`、API client、i18n）和工具函数 | ⬜ |
| 1.2 | 前端组件测试 | 对关键交互组件（登录表单、access-grant-form、gateway 注册流程、wallet 发放流程）编写组件级测试 | ⬜ |
| 1.3 | E2E 测试补齐 | Playwright 已配置但用例待补；覆盖核心流程：登录 → dashboard → 创建租户 → 注册网关 → 发放凭证 → 登出 | ⬜ |
| 1.4 | 后端集成测试 | 补齐 PostgreSQL + Redis 真实依赖的集成测试（当前多为 mock）；重点覆盖 auth 全链路、event replay、webhook 派发 | ⬜ |
| 1.5 | CI 测试流水线统一 | 新增 `ci.yml` 统一编排：typecheck → lint → unit test → build → e2e（前后端并行） | ⬜ |
| 1.6 | 前端 Lint 配置 | 引入 ESLint + Prettier 统一代码风格，CI 中强制检查；配置 `lint-staged` + `husky` pre-commit hook | ⬜ |

---

## 二、生产部署落地（2-3 周）

**目标：** 从开发环境推进到可交付的生产环境

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 2.1 | Fly.io 首次部署 | 按 `production-deployment-evaluation.md` 方案，完成 API + PostgreSQL + Redis 的 Fly.io 部署（`fly.toml`、secrets 管理、region 选择） | ⬜ |
| 2.2 | 前端静态部署 | web-admin 构建产物部署到 Cloudflare Pages / Vercel；配置 API proxy、环境变量、自定义域名 | ⬜ |
| 2.3 | 数据库迁移版本化 | 引入 golang-migrate 或 Atlas，将当前 `DATABASE_AUTO_MIGRATE` 逻辑转为版本化迁移文件；支持 up/down 回滚 | ⬜ |
| 2.4 | Secrets 管理规范 | 生产环境 `JWT_SECRET` / DB 密码 / Redis 密码 / API Key 统一走 Fly secrets 或 Vault；禁止 `.env` 文件提交 | ⬜ |
| 2.5 | 健康检查增强 | `/healthz` 增加 DB/Redis 连通性检查；新增 `/readyz` 就绪探针供编排层使用 | ⬜ |
| 2.6 | CDN 与缓存策略 | 前端静态资源配置 CDN 缓存（immutable hash 文件名 + 长缓存）；API 响应配置合理的 `Cache-Control` | ⬜ |

---

## 三、可观测性运营化（1-2 周）

**目标：** 将已接入的 metrics / traces / logs 转化为可运营的监控体系

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 3.1 | Grafana Dashboard 模板 | 基于已暴露的 Prometheus metrics，制作 API 延迟/错误率/QPS 面板模板（JSON 导出，纳入 `deploy/grafana/`） | ⬜ |
| 3.2 | 告警规则定义 | 定义关键告警：P99 延迟 > 2s、5xx 错误率 > 1%、Redis 连接失败、DB 连接池耗尽、证书即将过期 | ⬜ |
| 3.3 | Trace 采样策略 | 生产环境配置 tail-based sampling：错误请求全采、正常请求 10%；控制 OTLP 导出成本 | ⬜ |
| 3.4 | 业务指标看板 | 新增业务维度 metrics：活跃租户数、日均事件量、网关在线率、凭证发放量、SSE 活跃连接数 | ⬜ |

---

## 四、API 文档与开发者体验（1 周）

**目标：** 降低对接成本，支撑网关固件团队和移动端团队并行开发

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 4.1 | OpenAPI Spec | 基于 Chi 路由 + handler 注释生成 OpenAPI 3.0 spec（推荐 swaggo/swag 或手写 YAML） | ⬜ |
| 4.2 | Swagger UI 集成 | 开发环境暴露 `/docs` 端点渲染交互式 API 文档；生产环境可选关闭 | ⬜ |
| 4.3 | 移动端 SDK 生成 | 基于 OpenAPI spec 为 iOS（Swift）和 Android（Kotlin）生成类型安全 API client | ⬜ |
| 4.4 | API 版本演进策略 | 文档中明确 breaking change 策略：当前 `/api/v1/` 保持稳定，重大变更走 `/api/v2/` 并提供 6 个月并行期 | ⬜ |

---

## 五、性能与安全加固（2-3 周）

**目标：** 建立生产级性能基线 + 安全合规

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 5.1 | 负载测试 | 使用 k6 / vegeta 对核心路径（登录、事件上报 batch、凭证发放、SSE 长连接）建立性能基线 | ⬜ |
| 5.2 | 数据库查询优化 | 基于负载测试结果，对慢查询添加索引、优化 N+1 查询；review sqlc 生成的查询计划 | ⬜ |
| 5.3 | 安全渗透测试 | 对已部署环境执行 OWASP ZAP 自动化扫描 + 手动渗透（重点：认证绕过、越权访问、IDOR） | ⬜ |
| 5.4 | SBOM 生成 | 生成 CycloneDX / SPDX SBOM，满足供应链安全合规要求 | ⬜ |
| 5.5 | 安全响应头 | 前端部署配置 `Content-Security-Policy`、`X-Frame-Options`、`Strict-Transport-Security` 等 | ⬜ |
| 5.6 | Redis 降级演练 | CI 中加入 Redis 断连场景的集成测试，验证内存兜底降级路径正常工作 | ⬜ |
| 5.7 | 前端 Bundle 分析 | 引入 `rollup-plugin-visualizer`，定期检查 bundle 体积；当前已引入多个库，需关注首屏加载性能 | ⬜ |

---

## 六、原生移动端开发（8-12 周）

**目标：** 为住户（resident）和访客提供原生移动端体验，覆盖 BLE 开门、凭证展示、访客通行

### 7.1 iOS / iPadOS App（Swift + SwiftUI）

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 7.1.1 | 项目初始化 | 创建 Xcode 项目（`mistypass-ios/`），配置 SwiftUI App 生命周期、SPM 依赖管理、最低支持 iOS 17 / iPadOS 17 | ⬜ |
| 7.1.2 | 网络层 | 基于 OpenAPI 生成的 Swift client 封装 API 层；实现 token 自动刷新（`URLProtocol` 拦截 401）；支持 SSE 实时流消费 | ⬜ |
| 7.1.3 | 认证模块 | 登录页（品牌渐变背景 + 毛玻璃卡片）；支持密码登录 + 外部认证（ASWebAuthenticationSession）+ Face ID/Touch ID 生物识别快捷登录 | ⬜ |
| 7.1.4 | 首页与凭证展示 | 首页展示用户绑定的门禁凭证卡片（品牌渐变 + 盾牌图标）；支持 Apple Wallet 凭证添加（`.pkpass`） | ⬜ |
| 7.1.5 | BLE 开门 | 集成 Core Bluetooth，实现 BLE token 获取（`/app/access/ble-token`）→ 扫描网关设备 → 加密握手 → 开门指令；近场自动发现 + 手动触发两种模式 | ⬜ |
| 7.1.6 | 访客通行证 | 住户可创建访客通行证（`/app/visitor-passes`）；生成二维码供访客扫码通行；支持有效期设置和撤销 | ⬜ |
| 7.1.7 | 通行记录 | 展示个人通行日志（`/app/access/logs`）；按日期分组、支持筛选 | ⬜ |
| 7.1.8 | 推送通知 | 接入 APNs，支持门禁事件推送（异常告警、访客到达、凭证即将过期） | ⬜ |
| 7.1.9 | iPad 适配 | 利用 SwiftUI `NavigationSplitView` 实现 iPad 三栏布局（sidebar + list + detail）；支持 Stage Manager 多窗口 | ⬜ |
| 7.1.10 | Widget & Live Activity | 锁屏 Widget 显示最近通行状态；Live Activity 显示 BLE 开门进度 | ⬜ |

### 7.2 Android App（Kotlin + Jetpack Compose）

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 7.2.1 | 项目初始化 | 创建 Android 项目（`mistypass-android/`），Jetpack Compose + Material 3 + Hilt DI + Kotlin Coroutines；最低支持 Android 10（API 29） | ⬜ |
| 7.2.2 | 网络层 | 基于 OpenAPI 生成的 Kotlin client 封装 Retrofit/Ktor API 层；OkHttp Interceptor 实现 token 自动刷新；SSE 消费使用 OkHttp EventSource | ⬜ |
| 7.2.3 | 认证模块 | 登录页（品牌渐变背景 + Material 3 卡片）；支持密码登录 + 外部认证（Custom Tabs）+ 生物识别（BiometricPrompt） | ⬜ |
| 7.2.4 | 首页与凭证展示 | 首页展示门禁凭证卡片；支持 Google Wallet 凭证添加（`Save to Google Wallet` button，对接已有 wallet API） | ⬜ |
| 7.2.5 | BLE 开门 | 集成 Android BLE API（`BluetoothLeScanner` + `BluetoothGatt`），实现与 iOS 对等的 BLE 开门流程；处理 Android 12+ 蓝牙权限模型 | ⬜ |
| 7.2.6 | 访客通行证 | 功能对齐 iOS：创建访客通行证、生成二维码、有效期管理、撤销 | ⬜ |
| 7.2.7 | 通行记录 | 功能对齐 iOS：个人通行日志展示、日期分组、筛选 | ⬜ |
| 7.2.8 | 推送通知 | 接入 FCM，支持门禁事件推送；实现通知渠道分类（告警/访客/凭证） | ⬜ |
| 7.2.9 | NFC 开门（可选） | 部分 Android 设备支持 HCE（Host Card Emulation），可作为 BLE 的补充开门方式 | ⬜ |

### 7.3 移动端共享基础设施

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 7.3.1 | BLE 通信协议文档 | 定义网关 ↔ 手机 BLE 通信协议（Service UUID、Characteristic、加密握手流程、指令格式） | ⬜ |
| 7.3.2 | 推送服务后端 | API 新增设备注册端点（`POST /api/v1/app/devices`），存储 APNs/FCM token；事件触发时按用户查找设备并推送 | ⬜ |
| 7.3.3 | 移动端 CI/CD | iOS: Fastlane + GitHub Actions → TestFlight；Android: Fastlane + GitHub Actions → Google Play Internal Testing | ⬜ |
| 7.3.4 | 崩溃与性能监控 | iOS 接入 Sentry / Firebase Crashlytics；Android 同步接入；统一看板 | ⬜ |

---

## 七、官方网站（2-3 周）

**目标：** 建立 MistyPass 品牌官网，与 Web Admin 和移动端保持视觉一致性

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 8.1 | 技术选型 | Next.js（App Router）+ Tailwind + 共享 Design Token；部署到 Vercel / Cloudflare Pages | ⬜ |
| 8.2 | 首页设计 | Hero 区域使用品牌渐变 + 盾牌 3D 动画（Three.js / Spline）；功能亮点卡片、客户案例、CTA | ⬜ |
| 8.3 | 产品介绍页 | 分模块介绍：门禁管理、凭证发放、企业 SSO、网关管理；配合截图/动图展示 | ⬜ |
| 8.4 | 文档中心 | 集成 API 文档（OpenAPI Spec 渲染）、部署指南、SDK 使用说明 | ⬜ |
| 8.5 | 多语言支持 | 复用 i18n 体系（zh-CN / en-US / id-ID），与 Web Admin 翻译 key 共享品牌术语 | ⬜ |
| 8.6 | SEO 与性能 | SSR/SSG 优化、结构化数据（JSON-LD）、Core Web Vitals 达标、OG 社交分享卡片 | ⬜ |

---

## 八、业务功能迭代（持续）

| # | 子项 | 具体措施 | 状态 |
|---|------|----------|------|
| 9.1 | 网关 OTA 全链路联调 | OTA API 已就绪，与固件团队联调：任务下发 → MQTT 推送 → 固件下载 → 状态回报 → 审计闭环 | ⬜ |
| 9.2 | Google Wallet 正式发行 | 完成 LEI 企业认证后，切换 `WALLET_GOOGLE_REMOTE_VALIDATE=true`，上线真实凭证发放 | ⬜ |
| 9.3 | Apple Wallet 凭证 | 实现 `.pkpass` 生成与签名（需 Apple Developer 证书）；对接 iOS App 的 `PassKit` 添加流程 | ⬜ |
| 9.4 | 多租户计费模块 | 设计租户用量计量 + 计费方案（按网关数/凭证发放量/API 调用量）；对接 Stripe / Xendit 支付 | ⬜ |
| 9.5 | 租户自助入驻 | 官网提供自助注册流程：创建租户 → 配置楼宇 → 注册网关 → 邀请管理员 | ⬜ |

---

## 九、修改建议

以下是对当前项目的改进建议，可在上述阶段中穿插执行：

| # | 建议 | 说明 | 建议阶段 |
|---|------|------|----------|
| 10.1 | 数据库迁移无版本管理 | 当前依赖 `DATABASE_AUTO_MIGRATE` 自动建表，生产环境应切换为显式迁移文件（golang-migrate / Atlas），避免 schema drift | 阶段二 |
| 10.2 | 缺少 API 版本演进策略 | 当前所有端点在 `/api/v1/`，建议明确 breaking change 策略，为移动端长期兼容做准备 | 阶段四 |
| 10.3 | Redis 故障降级需演练 | 虽然代码有内存兜底，但建议在 CI 中加入 Redis 断连场景的集成测试，验证降级路径 | 阶段五 |
| 10.4 | 前端 Bundle 体积关注 | 已引入 TanStack Query + Table + RHF + Zod + i18next + Zustand，建议引入 bundle 分析工具定期检查首屏加载 | 阶段五 |
| 10.5 | 前端缺少 Lint 配置 | 建议引入 ESLint + Prettier 统一代码风格，并在 CI 中强制检查 | 阶段一 |
| 10.6 | 移动端 BLE 协议需提前定义 | BLE 通信协议是移动端和网关固件的共同依赖，建议在移动端开发启动前完成协议文档 | 阶段七之前 |
| 10.7 | 考虑 monorepo 管理 | 随着 iOS / Android / 官网项目加入，建议评估是否将所有项目纳入 monorepo（Turborepo / Nx）或保持独立仓库 + 共享 CI | 阶段七启动时 |

---

## 推荐执行顺序

```
阶段一（测试）
    ↓
阶段二（生产部署）+ 阶段四（API 文档）
    ↓
阶段三（可观测性）+ 阶段五（性能安全）
    ↓
阶段六（移动端开发）← 依赖 API 文档 + BLE 协议
    ↓
阶段七（官网）← 依赖产品截图
    ↓
阶段八（业务迭代）← 持续进行
```

> Web Admin UI 重设计、设计语言与角色化整改已独立到 `MISTYPASS-ROLE-BASED-UI-REFORM-PLAN.md`，与上述阶段并行推进。

---

## 进度总览

| 阶段 | 总项数 | ⬜ 未启动 | 🔄 进行中 | ✅ 已完成 |
|------|--------|----------|----------|----------|
| 一、测试质量 | 6 | 6 | 0 | 0 |
| 二、生产部署 | 6 | 6 | 0 | 0 |
| 三、可观测性 | 4 | 4 | 0 | 0 |
| 四、API 文档 | 4 | 4 | 0 | 0 |
| 五、性能安全 | 7 | 7 | 0 | 0 |
| 六、原生移动端 | 22 | 22 | 0 | 0 |
| 七、官方网站 | 6 | 6 | 0 | 0 |
| 八、业务迭代 | 5 | 5 | 0 | 0 |
| **合计** | **60** | **60** | **0** | **0** |
