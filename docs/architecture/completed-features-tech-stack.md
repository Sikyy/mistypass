# 已完成功能与技术栈对照（截至 2026-04-15）

当前能力状态：

- `PROD_READY`：网关补传与 checkpoint 合同链路、PostgreSQL 增量回放主路径、文档标识守卫与 CI smoke 已形成稳定回归闭环。
- `CONTRACT_READY`：企业 OIDC/SAML（含 callback、`code -> id_token`、JIT 回退、统一登出）与 Wallet 队列告警链路可联调，部分真实外部通道仍按排期挂起。

## 1. 范围说明

- 本文只整理“已完成并可回归验证”的能力，不包含 `BLOCKED_EXTERNAL` 挂起项（如 Google Wallet 真实发卡、WhatsApp Meta 真实企业号联调、实体设备恢复窗口）。
- 口径来源：`docs/development-status-roadmap.md` + 当前代码主干（`api`、`web-admin`、`.github/workflows`）。

## 2. 功能与技术栈对照

| 功能域 | 已完成能力（当前可用） | 关键接口 / 回归脚本 | 技术栈（核心） |
|---|---|---|---|
| 统一认证与权限 | 管理端与 App 端 JWT 登录、刷新、登出、`me`、RBAC 与租户/楼栋范围约束 | `/api/v1/auth/login` `/auth/refresh` `/auth/logout` `/me` `/auth/users/{userID}/building-scope` | Go 1.22、Chi Router、`github.com/golang-jwt/jwt/v5`、内存会话与撤销表 |
| 企业 SSO 与目录联动 | 域名识别、IdP 配置校验、`auth/start`、OIDC/SAML callback、`auth/exchange`、`auth/logout`、`sync_mode=jit` 回退发会话、JIT 审批门禁 + 审批回写 callback/worker | `/api/v1/enterprise/tenant/resolve` `/enterprise/auth/start` `/enterprise/auth/oidc/callback` `/enterprise/auth/saml/callback` `/enterprise/auth/exchange` `/enterprise/auth/logout` `/enterprise/jit-provision-approvals/*` | Go 模块化服务（enterprise/auth）、OIDC(JWKS+JWT)、SAML(`github.com/crewjam/saml`)、受信会话签发、审计 + 后台 worker |
| 网关离线闭环（Cloud 合同层） | bootstrap、`config/pull+applied`、事件批量补传、`retry_subset`、`queue_hint`、checkpoint 单调/上界保护、趋势摘要 | `/api/v1/gateway/*`、`/api/v1/gateways/events/checkpoint/summary`；`docs/testing/curl-gateway-event-*.zsh` | Go + Chi、队列水位状态机、审计日志、幂等去重 |
| PostgreSQL 增量持久化与回放 | `mistypass` 快照 + `mistypass_*` 投影、change-log 增量回放、checkpoint 回放、并发与多 state_key 基线、nightly soak 汇总 | `/api/v1/state/change-log*`；`docs/testing/curl-pg-replay-*.zsh`；`.github/workflows/api-replay-soak-nightly.yml` | PostgreSQL、`database/sql` + `lib/pq`、状态回放引擎、GitHub Actions |
| Wallet 队列与告警可观测 | queued 执行器、重试/DLQ/治理、metrics/trend、订阅策略、多通道告警分发与重试、发送记录查询 | `/api/v1/wallet/jobs/*`；`docs/testing/curl-wallet-job-*.zsh` | Go wallet + `alertdispatch` 模块、Resend/Mock/WhatsApp(Mock Meta) 适配、可配置阈值与冷却策略 |
| 管理台与联调资产 | 管理台核心页面（租户/空间/权限/网关/事件/告警/Wallet 运营）与后端 API map 对齐；文档状态标识守卫接入 CI | `web-admin` 路由与页面；`docs/testing/admin-ui-test-and-api-map.md`；`docs/testing/check-doc-capability-markers.zsh` | React 18 + Vite + TypeScript、React Router、Tailwind/组件库、GitHub Actions smoke |

## 3. 技术栈速览（按层）

- 后端 API：Go 1.22、Chi、模块化 service（auth/enterprise/gateway/wallet/state）。
- 协议与身份：JWT、OIDC(JWKS 验签)、SAML(Assertion 验签)。
- 数据层：PostgreSQL 快照 + 投影表 + change-log/checkpoint 增量回放。
- 前端：React 18 + TypeScript + Vite + Tailwind（管理台）。
- 回归与交付：Zsh 回归脚本、`go test ./...`、GitHub Actions（smoke + nightly）。
