# MistyPass -> Encore 迁移说明（冻结决策版，2026-04-14）

当前能力状态：

- `CONTRACT_READY`：迁移决策口径已冻结为“Go+Chi 主干保留 + Encore.go 渐进接入新 control-plane 服务”，并已明确首批试点与禁迁边界。
- `BLOCKED_EXTERNAL`：Meta/Google/实体设备三类真实通道验证继续按当前排期挂起，不作为本轮迁移试点阻塞条件。

## 1. 决策摘要（冻结）

- 不做全量重构。
- 如果要用 Encore，只考虑 `Encore.go`。
- 采用“现有 `Go + Chi` 主干保留 + 新 control-plane 服务渐进接入 `Encore.go`”。
- 以下三条不作为第一批迁移对象：
  - 网关离线合同链路（`checkpoint/retry_subset/queue_hint`）。
  - PostgreSQL 增量回放主链路（`change-log/replay/checkpoint`）。
  - 企业 SSO 主链路（`auth/start/callback/exchange`）。

## 2. 方案评分（工程判断，不是官方评分）

评分口径：优先考虑“现有资产复用、重构风险、业务契合度”，因为系统已有多块 `PROD_READY/CONTRACT_READY` 资产。

| 方案 | 综合分 | 现有资产复用 | 重构风险 | 新功能交付速度 | 对当前业务的结论 |
|---|---:|---:|---:|---:|---|
| Encore.go 渐进引入 | 88/100 | 4.5/5 | 4.5/5 | 4.5/5 | 最优。新服务吃到 Encore 平台红利，不动现有 Go 核心资产 |
| 继续 Go + Chi 主干 | 86/100 | 5/5 | 5/5 | 3/5 | 次优。最稳，但文档/可观测/服务治理平台税持续自担 |
| Encore.ts 新起后端主干 | 65/100 | 2/5 | 3/5 | 4/5 | 不建议。会引入第二套后端语言与领域模型 |
| NestJS 重构/新主干 | 62/100 | 2/5 | 3/5 | 4/5 | 不建议。生态成熟，但 Go 资产复用太低 |
| Spring Boot 重构/新主干 | 60/100 | 1.5/5 | 2.5/5 | 3/5 | 不建议。企业能力强，但当前阶段过重 |
| Encore.go 全量重写 | 60/100 | 2/5 | 1.5/5 | 3.5/5 | 明确不建议。对已稳定能力重复支付重构税 |

## 3. Encore.go 渐进引入排第一的原因

- 当前已具备模块化 Go service、PostgreSQL、JWT/OIDC/SAML、队列/DLQ/告警、CI smoke + nightly 等资产，问题更偏“平台治理效率”而非“核心业务能力缺失”。
- Encore 对 API、PostgreSQL、Pub/Sub、Cron、Cache、Auth、自动 tracing/docs 的支持，直接命中 control-plane 侧的工程负担。
- 当前最有价值的能力（checkpoint 单调性、幂等去重、离线补传、增量回放、SSO callback/JIT fallback）是领域壁垒，不是框架壁垒，重写收益低。

## 4. 各方案主要短板

### 4.1 继续 Go + Chi

- 稳定性最佳，但平台税持续自担。
- 系统可见性大量依赖脚本、文档、CI 约束，而非框架原生能力。
- 服务继续拆分后，跨服务文档、追踪、依赖拓扑维护成本上升。

### 4.2 Encore.go

- 优点是平台工程减负，不是替代领域逻辑重写。
- 对 HTTP/API + PostgreSQL + Pub/Sub + Cron + Cache 友好。
- 对设备协议层 / MQTT / Kafka / NATS / 自定义长连接协议，当前未作为第一等公民能力纳入本轮决策，按“外接能力”处理。
- `Cron Jobs` 在本地与 Preview 默认不运行，现有手工触发 API 的回归路径必须保留。

### 4.3 Encore.ts

- 本质问题是 TypeScript + Go 双后端并存。
- 会形成两套 auth/session/policy model、两套错误模型、两套可观测语义。
- 对当前安全敏感、状态敏感业务，不值得为框架切语言。

### 4.4 NestJS

- WebSocket、microservices transport、queues、guards 能力成熟。
- 代价是迁移或长期双栈共存，Go 资产复用低。
- 对当前阶段更像“重建已可用控制平面”，不是补核心短板。

### 4.5 Spring Boot

- 企业认证、授权、治理能力强。
- 当前并非从零建设 enterprise 平台，而是已有 Go 主干与回归闭环。
- 栈切换与工程体量成本过高。

### 4.6 Encore.go 全量重写

- 平台收益会被重构风险吞噬。
- 对已稳定链路（网关/回放/SSO）属于重复投入。

## 5. 按功能域的迁移建议

| 功能域 | 建议 |
|---|---|
| 统一认证与权限 | 保留现有 Go 实现（安全核心，不为框架重写） |
| 企业 OIDC/SAML/JIT | 保留现有 Go 实现（联调复杂，回归价值高） |
| 网关离线闭环/checkpoint/retry_subset | 保留现有 Go 实现（最硬业务核心） |
| PostgreSQL 增量回放/soak | 保留现有 Go 实现（状态机与回放引擎） |
| Wallet 队列/告警编排 | 可作为 Encore.go 候选，优先新增运营与策略能力 |
| 管理台后端 API/审计导出/Webhook/目录同步/SCIM | 最适合 Encore.go，最能获得文档、tracing、Pub/Sub、Cron 红利 |

## 6. 第一批试点与禁迁边界

建议第一批试点：

- 审计事件 fan-out + Webhook 投递。
- 目录同步 / SCIM / HRIS 导入。
- Wallet 告警策略编排的新子服务。
- 报表导出 / 审计归档 / 异步运营任务。

明确不先试：

- gateway 主链路。
- checkpoint/replay 主链路。
- enterprise auth callback 主链路。

## 7. 实施路径（按当前排期）

### Phase A：主线收敛（不改框架）

- 时间：2026-04-14 至 2026-04-20。
- 保持 R1/R2/R5 主链路稳定推进，不引入框架级不确定性。

### Phase B：Encore.go 控制面试点

- 时间：2026-04-21 至 2026-04-25。
- 范围：仅新建 control-plane bounded context，不回切核心链路。
- 要求：保留 `docs/testing/*.zsh` 回归口径，支持双轨验证。
- 当前进展：已在主干落地审计 Webhook fan-out 边界版 PoC（配置/手动分发/投递记录），作为后续 Encore.go 服务映射基线。

### Phase C：决策门

- 时间：2026-04-26。
- 产物：是否继续 service-by-service 迁移的量化报告（开发效率、可观测、运维复杂度、回归成本）。
- 评估口径基线：`docs/architecture/encore-poc-evaluation-baseline.md`（统一评分维度 + DoR/DoD）。

## 8. 回滚与护栏

- 试点期间不改变主 API 外部契约。
- 失败时可直接停用新服务并回退到现有主服务路径。
- 已拆分的边界与测试资产可保留，不引入额外运行时负担。

## 9. 一句话结论

对当前系统，Encore 不是“替换后端主干”的答案，而是“给未来新 control-plane 服务减平台工程负担”的答案。

## 10. 参考

- Encore Cloud Introduction: https://encore.dev/docs/platform/introduction
- Encore.go Auth: https://encore.dev/docs/go/develop/auth
- Encore.go Pub/Sub: https://encore.dev/docs/go/primitives/pubsub
- Encore.go Raw Endpoints: https://encore.dev/docs/go/primitives/raw-endpoints
- Encore Self-Host Docker: https://encore.dev/docs/how-to/self-host
- Encore Cron Jobs: https://encore.dev/docs/primitives/cron-jobs
- NestJS Microservices: https://docs.nestjs.com/microservices/basics/
- NestJS WebSockets: https://docs.nestjs.com/websockets/gateways
- NestJS Authorization: https://docs.nestjs.com/security/authorization
- Spring Security Authorization: https://docs.spring.io/spring-security/reference/features/authorization/index.html
