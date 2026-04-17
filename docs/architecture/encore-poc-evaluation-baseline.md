# Encore PoC 评估基线（截至 2026-04-14）

当前能力状态：

- `PROD_READY`：Go+Chi 主干上的审计与回归体系可稳定执行，包含 CI smoke、nightly soak、文档标识守卫。
- `CONTRACT_READY`：Audit Webhook fan-out 已形成可联调边界（配置/投递/记录查询/手动派发），可作为 Encore.go 同域试点对照对象。
- `BLOCKED_EXTERNAL`：涉及 Meta 真实企业号通道的“生产通道一致性对比”暂不在本评估首批结论范围。

## 1. 目的与边界

- 目的：给 R9 提供统一的“同口径对比模板”，避免后续 `Go+Chi` 与 `Encore.go` 试点结论不可比。
- 边界：只评估 control-plane 侧服务（Webhook/SCIM-HRIS/异步运营任务）；不纳入 gateway/checkpoint-replay/enterprise auth callback 主链路。

## 2. 评估维度与计分口径

总分 100，三维度加权：

- 开发效率（40 分）
  - 从需求冻结到 API 契约可回归的 lead time（越短越高）。
  - 新增能力所需“胶水代码”占比（路由编排/可观测/脚手架越少越高）。
  - 回归脚本与文档同步成本（新增能力能否低成本纳入现有 CI 口径）。
- 可观测性（35 分）
  - 请求链路可追踪完整度（trace/log/metric 是否默认可用）。
  - 故障定位时间（从失败到定位根因的步骤数与耗时）。
  - 服务依赖与 API 契约可见性（是否自动化、是否跨团队可读）。
- 运维复杂度（25 分）
  - 部署与环境配置复杂度（配置项数量、发布链路步骤）。
  - 回滚路径清晰度（回滚是否可脚本化、是否影响主干链路）。
  - 跨服务治理负担（鉴权、文档、监控、告警的重复建设程度）。

## 3. 当前基线（Go+Chi 审计 Webhook PoC）

基线对象：`api/internal/modules/audit` + `api/internal/http/router.go` 的 webhook 扩展与对应回归资产。

- 实现范围：
  - API：`GET/PUT /api/v1/audit/webhook/config`、`GET /api/v1/audit/webhook/deliveries`、`POST /api/v1/audit/webhook/dispatch`。
  - 回归：`docs/testing/curl-audit-webhook-fanout.zsh`（disabled 冲突、endpoint 不可达失败留档、action filter 冲突）。
  - CI：`.github/workflows/api-smoke.yml` 已纳入 `Audit Webhook Fan-out Regression`。
- 基线观察（用于后续对比，不作为最终结论）：
  - 开发效率：代码改动跨 `module + router + tests + docs + workflow` 五类资产，交付闭环完整但人工编排步骤多。
  - 可观测性：已有投递记录与错误落盘，定位链路依赖现有日志/脚本；跨服务拓扑与追踪仍偏手工。
  - 运维复杂度：当前体系稳定且可控，但新增服务时仍需重复维护 API 文档、回归脚本接入、治理约束。

## 4. Encore.go 对照采样要求

后续每个 Encore.go 试点必须按以下最小样本产出：

- 同能力范围 API（不少于 1 个写接口 + 1 个查询接口 + 1 条异步链路）。
- 同强度回归脚本（至少覆盖成功、幂等/去重、失败补偿三类场景）。
- 同级文档与 CI 接入（roadmap、API map、workflow 步骤）。
- 同窗口观测记录（开发耗时、故障定位耗时、配置/部署步骤数）。

## 5. 决策出口（DoR/DoD）

- DoR（进入决策评审前）：
  - 至少 1 个 Go+Chi 基线样本 + 1 个 Encore.go 对照样本。
  - 两边都具备可重复回归脚本和 CI 记录。
- DoD（形成 R9 结论）：
  - 输出统一评分表与证据链接。
  - 明确结论为二选一：
    - `继续 service-by-service 迁移（Encore.go）`
    - `保持现框架（Go+Chi）`
  - 给出下一阶段执行范围与禁迁边界（保持 gateway/checkpoint-replay/enterprise callback 不先迁）。
