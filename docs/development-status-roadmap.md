# MistyPass 开发状态与云边分工（截至 2026-04-17）

## 0. 读法与分级

- 紧急度：
  - `S0` 必须优先完成，直接影响当前可交付闭环。
  - `S1` 高优先，需并行推进，影响后续 1-2 个迭代节奏。
  - `S2` 中优先，可在 S0/S1 稳定后集中推进。
  - `S3` 依赖外部条件，当前阻塞。
- 进度口径：`0-100%` 代表“Cloud 侧能力 + 回归脚本 + 文档同步”综合完成度，不代表 Edge 设备端固件侧完成度。

## 1. 计划紧急度总览（按优先级排序）

| 编号 | 事项 | 紧急度 | 当前状态 | 进度 | 外部阻塞 | 下一里程碑 |
|---|---|---|---|---:|---|---|
| R1 | 网关离线可运行闭环（授权缓存 + 补传 + checkpoint） | S0 | 进行中 | 98% | 无（实体设备联调与 Cloud 合同链路并行） | 2026-04-20：Cloud 合同链路稳定性收敛 |
| R2 | PostgreSQL 事件化增量写入与稳定回放 | S0 | 进行中 | 99% | 无 | 2026-04-18：核心模块增量回放基线 |
| R3 | Wallet 队列/重试/DLQ/可观测（不依赖 LEI） | S1 | 进行中 | 97% | WhatsApp API（Meta）事项按当前排期挂起 | 2026-04-20：Resend + Mock 通道收敛 |
| R9 | Encore 增量迁移试点（Control-plane 渐进接入） | S1 | 进行中 | 89% | Meta 真通道对比已挂起（不阻塞 PoC） | 2026-04-25：首批 control-plane PoC 结论 |
| R4 | 文档体系统一（Cloud/Edge 边界 + Sprint 口径） | S1 | 已完成 | 100% | 无 | 2026-04-14：守卫范围与例外策略收口 |
| R5 | 企业 OIDC/SAML 生产化（回调/JIT/会话联动） | S1 | 已完成 | 100% | 无 | 2026-04-15：JIT 回写第二阶段收口 |
| R6 | 协议层强化（统一遥测与告警） | S2 | 进行中 | 68% | 无 | 2026-04-21：完成 WG-Branch-A A2/A3 合同收口与脚本化验收 |
| R8 | Edge Controller MVP 台架验证（2D/4D 方向 + 单门闭环 + 协议兼容） | S1 | 进行中 | 58% | 无（按现有台架持续推进） | 2026-04-20：完成单门本地闭环 30 分钟稳定运行与证据留档 |
| R7 | 真实 Google Wallet 发卡写接口 | S3 | 挂起 | 10% | Google Wallet API 外部条件未满足（LEI）且按当前排期挂起 | 条件恢复后重排 |

## 2. 大项拆解（已完成/未完成）

### R1. 网关离线可运行闭环（S0，进行中 98%）

已完成子项：

- [x] bootstrap 鉴权：`/api/v1/gateway/register` 发放 `device_token`，bootstrap 端点统一校验。
- [x] `config/pull` + `config/applied` 闭环：`authz_cache` 最小集、`policy/status_codes/status`、`rollback_version`、漂移审计。
- [x] `authz_cache` 状态机回归：`AUTHZ_CACHE_MISSING/FRESH/DRIFT/STALE`。
- [x] `events/batch` 部分成功与 `retry_subset` 可重试子集。
- [x] `events/batch` `queue_hint`：`checkpoint_id/acked_increment/status_code/next_action/server_ingested_total`。
- [x] checkpoint 趋势摘要：`/gateways/events/checkpoint/summary` 的 `time_window_trend`。
- [x] checkpoint 单调性保护：`acked_count` 回退返回 `409`，附最新 checkpoint + `next_action`。
- [x] checkpoint 上界保护：`acked_count` 超过服务端队列总量返回 `409`，附 `server_event_total/server_total_source/next_action`。
- [x] 新增 checkpoint 路由级边界回归：`api/internal/http/router_gateway_checkpoint_test.go` 覆盖“多队列单调性隔离（default 冲突不阻塞 priority）”与“default 队列 `event_rows_fallback` 上界冲突分支”。
- [x] 多队列上界口径：优先 `gateway_id + queue` 的 `ingested_total`，`default` 兼容 `event_rows_fallback`。
- [x] `server_ingested_total` 改为“仅统计新创建记录”，纯 dedup 回放不增长。
- [x] 网关侧“本地事件队列执行器”仿真脚本：`docs/testing/curl-gateway-edge-queue-executor-sim.zsh`，覆盖 `queue_hint -> retry_subset -> checkpoint` 决策链、重传去重与 checkpoint 回退保护。
- [x] Edge 实体联调执行 Runbook：`docs/edge/mvp-device-validation-runbook.md`（执行顺序、命令模板、证据留存与通过标准）。
- [x] 新增实体联调一键包装脚本：`docs/testing/run-edge-mvp-validation.zsh`（独立端口执行、自动生成报告与日志索引）。

未完成子项：

- [x] 非 `default` 队列（如 `priority`）的端到端 checkpoint 上界回归脚本（`docs/testing/curl-gateway-event-priority-checkpoint.zsh`，含 `ingested_total` 与 `queue_hint` 联动断言）。
- [x] `queue_ingest_totals` 跨重启恢复专项脚本（`docs/testing/curl-gateway-event-queue-ingest-restart.zsh`，先写入、重启 API、再校验上界判定与进度连续性）。
- [x] 多队列 checkpoint 隔离回归脚本：`docs/testing/curl-gateway-event-multi-queue-isolation.zsh`，验证 default 队列回退冲突不影响 priority 队列继续推进。
- [x] default 队列 fallback 上界回归脚本：`docs/testing/curl-gateway-event-checkpoint-fallback-default.zsh`，验证无 `queue_ingest_total` 时返回 `server_total_source=event_rows_fallback`。
- [x] 上述两条回归已纳入 CI smoke 分组（`.github/workflows/api-smoke.yml`）。
- [ ] 在真实设备进程中落地本地事件队列执行器（与 R8 单门闭环联调并行推进）。

当前风险：

- Edge 本地队列执行器已完成 API 合同层仿真；实体设备侧验证与 R8 联调并行推进，尚待证据留档签字。

### R2. PostgreSQL 事件化增量写入与稳定回放（S0，进行中 99%）

已完成子项：

- [x] `mistypass` 快照 + `mistypass_*` 投影持久化可用。
- [x] 全模块 `upsert + stale row 清理` 已替换全量删插。
- [x] `state/change-log/replay/checkpoint` 基础能力与脚本回归已通过。
- [x] `Save` 幂等去重：同 payload 重复保存不再追加 `mistypass_change_log`（避免无效增长）。
- [x] `Save` 事务化主链路：`mistypass` 快照写入与 change-log 追加同事务提交，投影写入失败可由 replay 链路修复。
- [x] enterprise 员工同步幂等键落地：`tenant_id + external_id` 优先，缺失时回退 `tenant_id + email`，冲突输入按 `rejected` 处理。
- [x] access 批量 upsert 增加跨模块 identity：`sync_source + sync_ref` 匹配优先，支持“同 identity 改 email 不新增”，identity/email 冲突拒绝。
- [x] enterprise -> access 同步链路已透传稳定 identity：`sync_source=enterprise_employee_sync`，`sync_ref` 采用 `external_id` 优先、`email` 回退。
- [x] `Save` 投影主路径切到 change-log replay：保存后按 checkpoint 增量回放驱动投影更新，不再直接以快照 payload 触发表写入。
- [x] 回放失败重试与幂等重放基线：新增故障注入回归（`docs/testing/curl-pg-replay-retry-idempotent.zsh`）与 `internal/state` 重试/幂等测试。
- [x] 回放并发稳定性基线：新增并发 no-op 与 catch-up 指标脚本（`docs/testing/curl-pg-replay-concurrency-baseline.zsh`），并补 `internal/state` 并发回放测试。
- [x] PostgreSQL replay 系列回归已纳入 CI smoke：`replay-retry-idempotent` + `replay-concurrency-baseline`。
- [x] 并发回放 KPI 阈值固化：吞吐下限、no-op p95/max 延迟阈值已在脚本与 CI 参数化落地。
- [x] 多租户 + 多 `state_key` 分层阈值曲线脚本：`docs/testing/curl-pg-replay-multi-state-curve.zsh`（按 level 输出 replay 吞吐与 no-op 延迟曲线）。
- [x] `internal/state` 增补跨 `state_key` 并发回放收敛测试，验证 checkpoint 隔离与最终幂等 no-op。
- [x] 新增长时回归包装脚本：`docs/testing/curl-pg-replay-multi-state-soak.zsh`（循环执行曲线回归、输出 rounds 明细与聚合指标）。
- [x] 新增 nightly workflow：`.github/workflows/api-replay-soak-nightly.yml`（定时执行 soak 并归档 `/tmp/mp_pg_replay_soak/*`）。
- [x] 新增 nightly 跨日复核脚本：`docs/testing/curl-pg-replay-soak-review.zsh`（聚合历史 `metrics.csv`，输出按天与按 `level|state_key` 稳定性报告）。
- [x] nightly workflow 增加历史留档抓取与复核汇总：自动拉取历史 `api-replay-soak-nightly` artifact，生成 `/tmp/mp_pg_replay_soak/review/report.md` 并写入 job summary。
- [x] nightly 历史收集容错增强：当本轮 `metrics.csv` 缺失时，history 收集步骤不再中断（输出 `current_metrics_present=false`），仍可基于历史数据继续 review/signoff。
- [x] nightly 复核口径修正：`curl-pg-replay-soak-review.zsh` 改为全局最值计算 `earliest_day/latest_day`，并新增 `summary.json` 与“接近掉速阈值”预警计数，便于多日稳定性跟踪。
- [x] nightly 复核新增连续覆盖守卫：`curl-pg-replay-soak-review.zsh` 增加 `SOAK_REVIEW_MAX_ALLOWED_MISSING_DAYS`，输出 `coverage_span_days/missing_days_in_span/coverage_ratio`；缺口超阈值标记 `insufficient_coverage`，并纳入 signoff 决策。
- [x] 新增 nightly 签字快照脚本：`docs/testing/curl-pg-replay-soak-signoff.zsh`（读取 `summary.json` 自动输出 `signoff.md` 决策与证据清单）。
- [x] nightly workflow 增加 signoff 快照阶段：自动写入 `review/signoff.md`、`/tmp/mp_pg_replay_soak_signoff.log` 并随 artifact 留档。
- [x] nightly signoff 门禁自动切换：`days_gap_to_min_days==0` 后自动启用 `SOAK_SIGNOFF_FAIL_ON_HOLD=true`，确保达标后非 `ready_for_signoff` 会阻断 job。

未完成子项：

- [ ] 连续多日（>=7 天）nightly 曲线留档与阈值稳定性复核签字（自动汇总链路已上线，待真实 nightly 数据积累后完成结项）。

当前风险：

- 在高并发写入+回放场景，跨模块时序仍依赖现有调用顺序，缺少严格因果链。

### R3. Wallet 队列/重试/DLQ/可观测（S1，进行中 97%）

已完成子项：

- [x] 非真实发卡流程（模板/单发/批发/状态流转/任务重试/审计）可用。
- [x] Google 配置校验支持本地校验 + 可选远端 issuer 探测。
- [x] 批量发卡新增 `execution_mode`：默认 `inline`，可选 `queued`（保留现有回归兼容）。
- [x] 队列执行器 MVP：`POST /api/v1/wallet/jobs/process` 支持 `limit/worker_count/max_retry/backoff` 参数化处理。
- [x] 新增队列回归脚本：`docs/testing/curl-wallet-job-queue-process.zsh`，覆盖 queued 入队 -> worker 处理 -> job 成功收敛。
- [x] DLQ 基础能力：任务处理按可重试与不可重试分流到 `failed/dlq`，并支持 `POST /api/v1/wallet/jobs/{jobID}/dlq/requeue` 人工回补。
- [x] 队列可观测最小视图：`GET /api/v1/wallet/jobs/summary` 返回状态分布、可重试失败数、错误码分布。
- [x] DLQ 回补回归脚本：`docs/testing/curl-wallet-job-dlq-requeue.zsh`，覆盖 `template_inactive -> dlq -> requeue -> success`。
- [x] DLQ 治理第一版：新增 `POST /api/v1/wallet/jobs/dlq/requeue`（批量回补）与 `POST /api/v1/wallet/jobs/dlq/cleanup`（按阈值清理）。
- [x] DLQ 批量治理回归脚本：`docs/testing/curl-wallet-job-dlq-governance.zsh`，覆盖 old/new DLQ、批量回补、清理策略与 summary 可用性。
- [x] DLQ 默认治理参数配置化：`WALLET_DLQ_CLEANUP_DEFAULT_LIMIT`、`WALLET_DLQ_CLEANUP_DEFAULT_OLDER_THAN`、`WALLET_DLQ_ALERT_THRESHOLD`、`WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY`、`WALLET_JOB_METRICS_DEFAULT_WINDOW` 已接入 `config.FromEnv` 并落到 wallet handlers 默认值。
- [x] 新增队列运营指标接口：`GET /api/v1/wallet/jobs/metrics`，提供窗口统计、状态分布与按错误码阈值告警（可 query 覆盖窗口/阈值）。
- [x] DLQ 清理审计归档策略：新增 `GET /api/v1/wallet/jobs/dlq/cleanup/archives`，按租户读取最近清理记录（含 `error_code/older_than_seconds/removed/remaining_dlq/processed_jobs/actor`）。
- [x] 新增阈值告警回归脚本：`docs/testing/curl-wallet-job-metrics-alert.zsh`，覆盖 `jobs/process` 默认重试参数与 `jobs/metrics` 告警阈值生效。
- [x] 管理台 Wallet 运营页第一版：`/wallet` 接入 `jobs/metrics + dlq/cleanup/archives`，支持租户筛选、阈值告警面板与清理归档表格。
- [x] 管理台 Wallet 聚合视图第二版：`/wallet` 新增跨租户聚合统计与风险排行（按 `DLQ + failed` 排序）。
- [x] Wallet 告警订阅策略第一版：新增 `GET/PUT /api/v1/wallet/jobs/alert-subscription`，支持租户级订阅开关、渠道（email/whatsapp）、阈值窗口、冷却与接收组配置。
- [x] 管理台 Wallet 运营页第三版：`/wallet` 新增“告警订阅策略”配置区并接入订阅策略读写。
- [x] 新增告警订阅策略回归脚本：`docs/testing/curl-wallet-job-alert-subscription.zsh`，覆盖默认策略回读、策略更新持久化与非法渠道组合校验。
- [x] 新增队列趋势接口：`GET /api/v1/wallet/jobs/metrics/trend`，支持 `window_seconds/bucket_count/max_retry/dlq_alert_threshold`。
- [x] 管理台 Wallet 运营页第四版：`/wallet` 新增窗口趋势图（时间桶）并支持 `bucket_count` 参数调节。
- [x] 新增趋势回归脚本：`docs/testing/curl-wallet-job-metrics-trend.zsh`，覆盖桶数量、窗口趋势聚合计数与告警联动。
- [x] Wallet 告警发送执行链路（mock）第一版：新增 `POST /api/v1/wallet/jobs/alerts/dispatch` 与 `GET /api/v1/wallet/jobs/alert-notifications`，支持按订阅策略评估、冷却去重与发送结果留档。
- [x] 管理台 Wallet 运营页第五版：`/wallet` 新增“立即评估并发送”操作与“告警发送记录”表格。
- [x] 新增告警发送回归脚本：`docs/testing/curl-wallet-job-alert-dispatch.zsh`，覆盖首次发送、冷却跳过与发送记录查询。
- [x] Wallet 告警发送失败重试策略第一版：新增 `POST /api/v1/wallet/jobs/alert-notifications/{notificationID}/retry`，支持失败记录手动重试、`idempotency_key` 幂等去重与 `attempt/retryable/provider_error` 追踪字段。
- [x] 新增告警发送重试回归脚本：`docs/testing/curl-wallet-job-alert-dispatch-retry.zsh`，覆盖首次发送失败、手动重试成功与重复重试幂等跳过。
- [x] Wallet 告警 Provider 第二版：`email(resend/mock)` + `whatsapp(mock/meta)` 双通道配置接入，统一返回 `channel_results` 回执。
- [x] 新增 Resend 回归脚本：`docs/testing/curl-wallet-job-alert-dispatch-resend.zsh`，覆盖 provider 路径、鉴权头与收件组映射校验。
- [x] 新增 WhatsApp 回归脚本：`docs/testing/curl-wallet-job-alert-dispatch-whatsapp.zsh`（mock）+ `docs/testing/curl-wallet-job-alert-dispatch-whatsapp-meta.zsh`（meta mock server），覆盖 whatsapp-only 订阅、provider 鉴权路径与 `channel_results` 回执校验。
- [x] Wallet 告警发送回归脚本已纳入 CI smoke：`dispatch + dispatch-retry + dispatch-resend + dispatch-whatsapp`（`.github/workflows/api-smoke.yml`）。
- [x] Wallet 告警分发编排拆分试点：新增 `api/internal/modules/wallet/alertdispatch` 纯业务包，`DispatchJobMetricsAlerts` 改为先走编排预判（订阅/通道/冷却/idempotency）再执行 provider 发送。

未完成子项：

- [ ] WhatsApp Meta 企业号联调与生产凭证接入（按当前排期挂起；现阶段以 `mock` 通道和 `meta` 配置能力为主）。

当前风险：

- WhatsApp API（Meta）相关事项已按当前排期挂起；当前仅覆盖 provider 协议与发送编排路径回归。

### R4. 文档体系统一（S1，已完成 100%）

已完成子项：

- [x] Cloud/Edge 硬边界已写入架构与状态文档。
- [x] 网关协议/回归映射文档已覆盖当前接口与脚本。
- [x] 本文档已升级为“紧急度 + 进度 + 已完成/未完成子项”结构。
- [x] Sprint 统一模板已落地：`docs/sprints/sprint-template.md`（能力 + 归属 + 前置依赖 + 里程碑）。
- [x] 首个存量 Sprint 文档已按模板迁移：`docs/sprints/c-zone-sprint-plan.md`。
- [x] 当前 `docs/sprints` 存量文档已完成模板迁移。
- [x] 能力状态标识规范已落地：`docs/architecture/capability-status-markers.md`（`PROD_READY/CONTRACT_READY/SKELETON_ONLY/BLOCKED_EXTERNAL`）。
- [x] 关键文档已接入统一标识：`docs/testing/admin-ui-test-and-api-map.md`、`docs/wallet/google-wallet-issuance-plan.md`、`docs/architecture/gateway-serial-protocol-mobile-plan.md`、`docs/enterprise/indonesia-enterprise-domain-idp-design.md`、`docs/edge/mvp-device-validation-plan.md`。
- [x] 追加补齐统一标识：`docs/architecture/encore-migration-playbook.md`、`docs/enterprise/enterprise-design-spec.md`、`docs/sprints/sprint-template.md`。
- [x] 新增文档状态标识守卫脚本：`docs/testing/check-doc-capability-markers.zsh`，并接入 CI（`.github/workflows/api-smoke.yml`）。
- [x] 新增“已完成功能 × 技术栈”对照文档：`docs/architecture/completed-features-tech-stack.md`（便于对外同步当前可交付能力）。
- [x] 文档标识守卫升级：支持全量递归扫描（`DOC_MAX_DEPTH=0`）与例外策略文件（`docs/testing/doc-capability-marker-ignore.txt`）。
- [x] 新增项目 Wiki 基线：`docs/wiki/internal/*`（研发内用模块手册）+ `docs/wiki/external-api/*`（面向 API 调用开发者/合作企业的对外文档骨架，按 Kisi 风格组织）。
- [x] 内部 Wiki 增补“优先级看板 + 模块深描”：`docs/wiki/internal/priority-board.md`、`docs/wiki/internal/module-deep-dive.md`，用于日常排期同步与模块协作入门。
- [x] 对外文档骨架升级为首批实页：`external-api/getting-started.md`、`authentication.md`、`guides-gateway-offline-workflow.md`、`guides-audit-webhook.md`、`reference-gateway-events-batch.md`、`errors-and-reliability.md`。
- [x] 对外文档补齐第二批高频接入主题：`guides-enterprise-sso-jit.md`、`guides-wallet-queue-ops.md`、`reference-enterprise-auth-start-exchange.md`、`reference-wallet-alert-dispatch.md`。

未完成子项：

- [ ] 无（该项按当前范围已收口，新增例外需走代码评审与注释说明）。

### R5. 企业 OIDC/SAML 生产化（S1，已完成 100%）

已完成子项：

- [x] 企业接入基础：tenant resolve、IdP config/validate、员工同步与幂等补偿。
- [x] `enterprise/auth/exchange` 增加 `sync_mode=jit` 回退：当本地账号不存在时，基于企业员工档案生成受信会话。
- [x] Auth 模块新增受信用户登录能力（`LoginByTrustedUser`），支持 JIT 用户会话刷新与后续 `me/logout` 一致口径。
- [x] 回调编排草案与接口定义文档：`docs/enterprise/oidc-saml-callback-orchestration-draft.md`。
- [x] 新增 `POST /api/v1/enterprise/auth/start`：下发 provider 入口地址（`authorize_url/sso_url`）与短时 `state_token`。
- [x] 新增 `GET /api/v1/enterprise/auth/oidc/callback` 与 `POST /api/v1/enterprise/auth/saml/callback`：消费一次性 `state_token` 并复用统一会话签发（含 JIT 回退）。
- [x] OIDC callback 支持 `code -> id_token` 交换链路（基于 `token_url`，缺省回退 `issuer_url/oauth2/token`）。
- [x] 新增 `POST /api/v1/enterprise/auth/logout`：支持 `access_token/refresh_token` 企业会话联动撤销并返回撤销摘要。
- [x] JIT 目录缺失场景自动建档：本地账号缺失且目录无员工记录时，按租户域名校验后创建 `source=jit_provision` 员工档案并签发会话。
- [x] JIT 属性映射与停用拦截：支持从 OIDC/SAML 身份属性映射 `full_name/department/job_title/location` 并套用权限模板；若员工状态为 `inactive` 则阻断会话签发。
- [x] JIT 会话签发联动收敛：`sync_mode=jit` 下即使本地已有账号，也强制对齐企业员工状态与权限模板（`inactive` 阻断、`active` 同步角色/楼栋范围）。
- [x] 新增 JIT 深属性映射草案：`docs/enterprise/jit-deep-attribute-mapping-draft.md`（字段优先级、冲突约束、停用撤权联动顺序与回归清单）。
- [x] 企业会话签发错误语义收敛：callback/exchange 不再统一返回 `401`，改为 `inactive=403`、`external_id 冲突=409`、输入错误 `400`、其余内部错误 `500`（含单测覆盖）。
- [x] 新增 callback/exchange 路由级状态码回归：`api/internal/http/router_enterprise_exchange_status_test.go` 覆盖 `sync_mode=jit` 下 `inactive=403` 与 `external_id 冲突=409`。
- [x] 新增 OIDC callback 路由级状态码回归：`api/internal/http/router_enterprise_oidc_callback_status_test.go` 覆盖 `sync_mode=jit` 下 `inactive=403` 与 `external_id 冲突=409`。
- [x] 新增 SAML exchange 路由级状态码回归：`api/internal/http/router_enterprise_saml_callback_status_test.go` 覆盖 `sync_mode=jit` 下 `inactive=403` 与 `external_id 冲突=409`。
- [x] 新增 SAML callback 路由级状态码回归：`api/internal/http/router_enterprise_saml_callback_status_test.go` 覆盖 `sync_mode=jit` 下 `inactive=403` 与 `external_id 冲突=409`（含 fixture 时钟固化，避免历史签名样例时间漂移）。
- [x] JIT 会话新增 `employment_status` 阻断：当 OIDC/SAML 身份声明出现 `inactive/terminated/disabled/suspended/deprovisioned`（含布尔 `active=false`）时，优先拒绝会话签发并返回 `403`（`ErrEmployeeInactive`）；新增回归 `router_enterprise_oidc_employment_status_test.go` 与 `router_enterprise_jit_test.go`。
- [x] SCIM/HRIS 目录快照优先级落地（第一阶段）：`ResolveOrProvisionJITEmployee` 对 `source~(scim|hris)` 的员工不再让 callback claims 覆盖已有 `full_name/department/job_title/location`（仅允许填充空字段）；新增服务层回归 `service_jit_provision_test.go` 与路由回归 `router_enterprise_oidc_employment_status_test.go`。
- [x] 停用撤权联动（第一阶段）：`inactive` 拦截时触发同邮箱 refresh session 批量撤销，并写入审计 `enterprise_jit_deprovision_applied`（`provider/email/external_id/employment_status/revoked_refresh`）；新增回归 `router_enterprise_oidc_employment_status_test.go` 与 `auth/service_trusted_user_test.go`。
- [x] 组织审批流门禁（第一阶段）：新增环境开关 `ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED`；开启后目录缺失返回 `403 ErrJITProvisionApprovalRequired`，并落库 `jit_provision_approval(pending)` + 审计 `enterprise_jit_approval_required`，支持 `GET /enterprise/jit-provision-approvals` 查询与 `POST /enterprise/jit-provision-approvals/{approvalID}/review` 审批放行。
- [x] 停用撤权联动（第二阶段）：`inactive` 拦截时在 refresh 批量撤销之外，追加本地 trusted user 最小权限降级（`role -> resident`、清空 `building_scope`），并在 `enterprise_jit_deprovision_applied` 审计中记录 `old_role/new_role/downgraded_local`。
- [x] SCIM/HRIS 深属性映射（第二阶段）：企业员工与 JIT profile 新增 `phone/manager_external_id/employment_status`，统一归一（含布尔 `active` 语义）并纳入目录快照优先级策略（`source~(scim|hris)` 仅填充空字段，不覆盖已有快照字段）。
- [x] 组织审批流跨系统回写编排（第一阶段）：审批记录新增 `external_sync_status/ref/attempt/error`，提供待回写列表 `GET /enterprise/jit-provision-approvals/external-sync-pending` 与回写结果上报 `POST /enterprise/jit-provision-approvals/{approvalID}/external-sync`，并写审计 `enterprise_jit_approval_external_sync_updated`。
- [x] JIT 自动建档策略收口（组织审批流跨系统回写第二阶段）：新增公开回调 `POST /enterprise/jit-provision-approvals/external-sync/callback`（callback token 校验）与后台失败自动重试 worker（`ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_*`）；补齐路由/worker 回归：`router_enterprise_jit_approval_sync_test.go` + `router_test.go`。

未完成子项：

- [ ] 无（当前范围已收口；后续仅剩 enterprise 运营页接入类事项）。

### R6. 协议层强化（S2，进行中 68%）

已完成子项：

- [x] 协议兼容：`wiegand_26/wiegand_34/osdp_v2/rs485/ble`。
- [x] RS485 运行态 telemetry + 阈值告警审计。
- [x] 已新增“印尼老 Wiegand 客户增强网关（Kisi Controller 增强版）”方案，并校正关键口径（云编排 + 网关本地放行闭环）：`docs/architecture/gateway-serial-protocol-mobile-plan.md`。
- [x] `WG-Branch-A/B/C` 已细化为可执行无外部 API 任务拆解（PoC/Pilot/Upgrade）：`docs/architecture/gateway-serial-protocol-mobile-plan.md`。
- [x] `WG-Branch-A` 已新增可执行 PoC 回归脚本 `docs/testing/curl-gateway-legacy-wiegand-poc.zsh` 并本地通过（覆盖 legacy/new 设备默认协议、`probe-legacy` 建议、`events/access` 幂等去重）。
- [x] 已补“跨协议统一遥测字段与告警口径”第一阶段：`/gateways/{gatewayID}/devices/{deviceID}/rs485/telemetry` 返回统一 `telemetry` 视图（`alert_level/line_quality/governance_action/reason_codes/line_policy`），并新增协议级审计 `gateway_protocol_health_alert`。
- [x] 已补“老旧设备探测/降级与线路质量治理策略”第一阶段：`/gateways/{gatewayID}/devices/probe-legacy` 新增治理载荷（`legacy_protocol/upgrade_protocol/offline_fallback/degraded_line_action/line_policy`）。
- [x] 已补 `WG-Branch-A A4` 审计一致性：网关 `access/device/batch` 入站事件统一输出规范审计动作（`grant/deny/tamper/timeout/rex`）与标准化 target 字段模板。
- [x] 已新增 `WG-Branch-A A3` 脚本化基线：`docs/testing/curl-gateway-door-io-loop.zsh` 覆盖单门 `rex/tamper/timeout` 设备事件链路、幂等回放与审计动作一致性。
- [x] 本轮验证通过：`api/go test ./...`、`docs/testing/curl-gateway-serial-protocol.zsh`、`docs/testing/curl-gateway-legacy-wiegand-poc.zsh`、`/bin/zsh docs/testing/curl-gateway-event-retry-subset-mixed.zsh`、`/bin/zsh docs/testing/curl-gateway-event-checkpoint-partial.zsh` 与 `docs/testing/check-doc-capability-markers.zsh`。

未完成子项：

- [ ] `WG-Branch-A A2` 真实边端本地放行闭环（签名缓存 + 本地时钟 + 反重放计数器）仍待设备进程侧落地。
- [ ] `WG-Branch-A A3` 实体单门门态链路（继电器/门磁/REX/防拆）仍待 R8 台架联调完成后回写签字。

### R7. 真实 Google Wallet 发卡写接口（S3，挂起 10%）

已完成子项：

- [x] 前置方案和非真实发卡链路已准备。

未完成子项：

- [ ] LEI 完成且收到恢复指令后，接入真实写接口并做端到端验收。

挂起说明：

- 企业 LEI 申请未完成，且该事项已按当前排期挂起。

### R8. Edge Controller MVP 台架验证（S1，进行中 58%）

已完成子项：

- [x] 台架硬件清单已冻结并同步到文档：香橙派 Zero 3（1G 套件）、2 路光耦、微雪 FT232RNL RS485、杜邦线、龙杰 ACS Wallet Mate II。
- [x] 已按 PRD v0.2 校正硬件方向：当前主线为 `Controller 2D/4D + Reader 兼容层`，不以自研 Reader 作为近期交付前置。
- [x] 台架设计/使用方案已改为 Controller-first 结构，并按“已完成/进行中/未完成 + 进度”管理：
  - `docs/edge/mvp-device-validation-plan.md`
- [x] 实体联调 runbook 已更新为“本地判定优先 + Wiegand/OSDP 兼容 + 单门 I/O 闭环”执行顺序：
  - `docs/edge/mvp-device-validation-runbook.md`
- [x] 已完成 `Legacy Retrofit / Cloud-Native Controller / Partner-Backed Wallet` 三模式边界整理，并与协议分支计划对齐。

进行中子项：

- [ ] 单门本地闭环联调（继电器 + 门磁 + REX + 防拆）与 30 分钟稳定性签字留档。
- [ ] `Wiegand + OSDP` 兼容参数基线与异常分类清单固化（线序/波特率/轮询/超时）。
- [ ] 关键脚本在实体链路执行留档（`legacy-wiegand-poc/door-io-loop/idempotency/retry-subset-mixed/checkpoint-partial`）。
- [ ] Controller 2D/4D 工程化输入整理：门数与 I/O 资源矩阵、电源与防护前置项、EVT 前置 checklist。

未完成子项：

- [ ] 异常场景（断网/串口抖动/重启）恢复时序与阈值固化（含门级故障隔离）。
- [ ] 面向批量部署的物料替代与冗余方案（工业存储/UPS/替代器件）。
- [ ] Partner Wallet 增强模式的数据映射与回调编排清单（Integration Hub 边界，不阻塞 V1）。

### R9. Encore 增量迁移试点（S1，进行中 89%）

已完成子项：

- [x] 输出迁移说明文档：`docs/architecture/encore-migration-playbook.md`（收益/风险/排期/回滚策略）。
- [x] 完成 Wallet 告警编排“可迁移边界”代码拆分：`api/internal/modules/wallet/alertdispatch`。
- [x] 迁移决策口径冻结：仅 `Encore.go` 渐进接入新 control-plane，不做全量重构。
- [x] 首批试点与禁迁边界冻结：首批聚焦 `Webhook/SCIM-HRIS/运营异步任务`，不先迁 `gateway/checkpoint-replay/enterprise-auth-callback` 主链路。
- [x] 落地审计事件 Webhook fan-out PoC（Go 主干内边界版本）：`/audit/webhook/config|deliveries|dispatch` + 模块化投递记录能力（便于后续映射 Encore service）。
- [x] 新增审计事件 Webhook fan-out 回归脚本并接入 CI smoke：`docs/testing/curl-audit-webhook-fanout.zsh` + `.github/workflows/api-smoke.yml`。
- [x] 新增 R9 统一评估基线文档：`docs/architecture/encore-poc-evaluation-baseline.md`（开发效率/可观测性/运维复杂度评分口径 + DoR/DoD）。
- [x] 对外 API 文档第三批（control-plane 边界收敛）：新增 Enterprise 审批回写与 sync worker 告警专题（guide/reference）+ Wallet DLQ 治理专题（guide/reference）。
- [x] 对外 API 文档第四批（资源级 reference 收口）：新增 Enterprise `sync-requests/reconcile`、Wallet `metrics/trend`、`alert-subscription` 参考页，补齐高频运营接口的参数与错误码语义。
- [x] 对外 API 文档第五批（基础资源收口）：新增 `Auth Session/Me`、`Tenant/Space Core`、`Access Core` 参考页，补齐角色矩阵、字段枚举与 scope 错误语义。
- [x] 对外 API 文档第六批（Enterprise 资源收口）：新增 `Tenant Resolve / Domain Mappings`、`IdP Config / Validate`、`Employees / Sync Jobs` 参考页，补齐企业接入基础资源的字段、枚举与幂等语义。
- [x] 对外 API 文档第七批（Wallet/Gateway 资源收口）：新增 `Wallet Jobs Process / Alert Notifications`、`Gateway Serial Inventory / Checkpoint Summary` 参考页，补齐队列处理与库存/checkpoint 运营接口的参数、状态流转与时间窗趋势语义。
- [x] 对外 API 文档第八批（发布口径收口）：新增 `changelog-and-migration` 版本迁移页，并完成 Guide/Reference 双向互链收敛，降低跨页查找与重复解释成本。
- [x] 对外 API 文档第九批（治理规范收口）：新增 `release-process-template` 与 `deprecation-policy`，并同步更新 external README、信息架构与 wiki 入口，形成“变更记录 -> 发布评审 -> 弃用下线”闭环。
- [x] 对外 API 文档第十批（接入排障收口）：新增 `guides-api-token-scope-troubleshooting` 与 `guides-rate-limit-and-429`，补齐最小权限分配、scope 越权排查与 provider `429` 退避实战。

未完成子项：

- [ ] 外部资质完成后，按相同回归口径对比 Meta 真通道 PoC 与主服务行为一致性。
- [ ] 形成 Encore PoC 的开发效率/可观测性/运维复杂度对比结论。
- [ ] 输出 `继续 service-by-service 迁移` 或 `保持现框架` 的决策报告。

## 3. Cloud / Edge 硬边界（冻结）

Cloud 负责：

- 多租户模型、策略编排、审计归档、设备管理编排、集成与运营指标。

Edge 负责：

- 门级实时判定、本地授权缓存、本地事件队列、断网运行、协议适配、设备安全根。

强约束：

- 开门链路不得依赖云端实时 round-trip。
- 云边交互默认“异步 + 幂等 + 最终一致”。

## 4. 回归脚本现状（关键链路）

- `docs/testing/curl-gateway-config-pull-apply.zsh`：通过，覆盖 authz cache 状态机四态。
- `docs/testing/curl-gateway-door-io-loop.zsh`：通过，覆盖单门 `rex/tamper/timeout` 事件链路、幂等回放与设备事件审计动作映射（`gateway_rex_event_recorded/gateway_tamper_event_recorded/gateway_door_timeout_recorded`）。
- `docs/testing/curl-gateway-event-idempotency.zsh`：通过，覆盖单条事件 replay dedup + `queue_progress` 非增长。
- `docs/testing/curl-gateway-event-checkpoint-partial.zsh`：通过，覆盖 partial/retry_subset/checkpoint/summary/两类 409。
- `docs/testing/curl-gateway-event-retry-subset-mixed.zsh`：通过，覆盖 mixed retry + `server_ingested_total` 进度与 dedup 不增长。
- `docs/testing/curl-gateway-event-batch-replay.zsh`：通过，覆盖批量重放去重与列表无重复。
- `docs/testing/curl-gateway-event-priority-checkpoint.zsh`：通过，覆盖 `priority` 队列 `server_ingested_total` + checkpoint 上界 + `server_total_source=queue_ingest_total`。
- `docs/testing/curl-gateway-event-queue-ingest-restart.zsh`：通过，覆盖 `queue_ingest_totals` 跨重启恢复与 checkpoint 上界连续性。
- `docs/testing/curl-gateway-event-multi-queue-isolation.zsh`：通过，覆盖 `default` 队列 checkpoint 回退冲突时 `priority` 队列仍可独立前进（多队列单调性隔离）。
- `docs/testing/curl-gateway-event-checkpoint-fallback-default.zsh`：通过，覆盖 `default` 队列在无 `queue_ingest_total` 时的 checkpoint 上界冲突 fallback（`server_total_source=event_rows_fallback`）。
- `docs/testing/curl-gateway-edge-queue-executor-sim.zsh`：通过，覆盖“本地执行器”决策链（`queue_hint -> retry_subset -> checkpoint`）、重复补传 dedup 与 checkpoint 回退冲突保护。
- `docs/testing/run-edge-mvp-validation.zsh`：通过，覆盖实体联调关键脚本一键串行执行（独立端口隔离）与报告留档（`docs/testing/artifacts/edge-mvp-validation-*.md`）。
- `docs/testing/curl-pg-replay-retry-idempotent.zsh`：通过，覆盖 checkpoint 回放故障注入、失败不推进 checkpoint、修复后重试成功、再次回放幂等 no-op。
- `docs/testing/curl-pg-replay-concurrency-baseline.zsh`：通过，覆盖 checkpoint catch-up 回放吞吐与并发 no-op 回放稳定性（失败率 + p95 延迟）。
- `docs/testing/curl-pg-replay-multi-state-curve.zsh`：通过，覆盖多租户+多 `state_key` 分层压测曲线（delta、吞吐、no-op p95/max、阈值判定）。
- `docs/testing/curl-pg-replay-multi-state-soak.zsh`：通过（最小轮次验证），覆盖多轮曲线执行、趋势守卫、聚合统计与 CSV 留档。
- `docs/testing/curl-pg-replay-soak-review.zsh`：通过（样例数据验证），覆盖跨日汇总、`>=7` 天覆盖度检查、吞吐跌幅守卫与 Markdown 报告输出。
- `docs/testing/curl-pg-replay-soak-signoff.zsh`：通过（样例数据验证），覆盖 review 摘要到签字快照转换、决策门判断与证据清单标准化输出。
- `docs/testing/curl-pg-replay-soak-review-signoff-smoke.zsh`：通过，覆盖连续覆盖守卫的双态回归（`insufficient_coverage -> hold_collect_more_data` 与 `passed -> ready_for_signoff`）。
- `docs/testing/curl-wallet-job-queue-process.zsh`：通过，覆盖 wallet queued 批量任务入队与 `jobs/process` worker 处理闭环。
- `docs/testing/curl-wallet-job-dlq-requeue.zsh`：通过，覆盖 wallet 任务 DLQ 落位、`jobs/summary` 观测与 `dlq/requeue` 回补闭环。
- `docs/testing/curl-wallet-job-dlq-governance.zsh`：通过，覆盖 wallet DLQ 批量回补（`dlq/requeue`）、阈值清理（`dlq/cleanup`）与清理归档查询（`dlq/cleanup/archives`）。
- `docs/testing/curl-wallet-job-metrics-alert.zsh`：通过，覆盖 `jobs/process` 默认 `max_retry` 配置与 `jobs/metrics` 告警阈值触发。
- `docs/testing/curl-wallet-job-alert-subscription.zsh`：通过，覆盖订阅默认值回读、`alert-subscription` 更新持久化与非法渠道组合拦截。
- `docs/testing/curl-wallet-job-metrics-trend.zsh`：通过，覆盖 `metrics/trend` 桶数量、窗口趋势聚合计数与阈值告警联动。
- `docs/testing/curl-wallet-job-alert-dispatch.zsh`：通过，覆盖 `alerts/dispatch` 首次发送、冷却跳过与 `alert-notifications` 记录查询。
- `docs/testing/curl-wallet-job-alert-dispatch-retry.zsh`：通过，覆盖 `alerts/dispatch` 失败注入、`alert-notifications/{notificationID}/retry` 重试成功与重复重试幂等跳过。
- `docs/testing/curl-wallet-job-alert-dispatch-resend.zsh`：通过，覆盖 `resend` provider 发送链路、Authorization 头与 `receiver_groups -> email` 映射。
- `docs/testing/curl-wallet-job-alert-dispatch-whatsapp.zsh`：通过，覆盖 whatsapp-only 订阅发送链路与 `channel_results` 统一回执口径。
- `docs/testing/curl-wallet-job-alert-dispatch-whatsapp-meta.zsh`：通过，覆盖 `meta` provider 请求路径、Authorization 头与 payload 字段校验（本地 mock 验证，非企业号真实联调）。
- `docs/testing/curl-audit-webhook-fanout.zsh`：通过，覆盖 webhook 配置启停、disabled/action-filter 冲突、不可达 endpoint 失败留档与投递记录查询。

## 5. 高优先级排期（2026-04-15 版）

1. `S0`（2026-04-14 ~ 2026-04-18）：推进 R2 多日 nightly soak 曲线留档  
   目标：连续采样并固化稳定阈值，降低环境波动误报。

2. `S0`（2026-04-14 ~ 2026-04-20）：推进 R1 Cloud 合同链路稳定性收敛  
   目标：在不依赖实体设备测试的前提下，持续收敛 `queue_hint/retry_subset/checkpoint` 合同回归与留档。

3. `S1`（2026-04-15 ~ 2026-04-25）：推进 R9 Encore 迁移试点  
   目标：完成 Control-plane 试点 PoC 量化评估，并形成“继续 service-by-service 迁移 / 保持现框架”结论。

4. `S1`（2026-04-15 ~ 2026-04-20）：推进 R3 Wallet 队列与告警收敛  
   目标：在 `resend + mock` 口径下完成运营通道收敛与回归留档，保持 `BLOCKED_EXTERNAL` 边界清晰。

5. `S2`（2026-04-20 ~ 2026-04-26）：推进 R6 协议层强化  
   目标：统一跨协议遥测字段与告警口径，降低边端协议差异带来的运维复杂度。

6. `S1`（持续）：维护 R4/R5 已完成项的回归守卫  
   目标：保持能力标识守卫 + `go test ./...` + 关键脚本口径不回退。

## 5.1 前后端无外部依赖并行高优先（持续）

前端（Web Admin）优先级：

1. `FE-P0`（本轮已完成）：`F1/F4` 角色边界收口签字
   - 子项：`building_admin` 在 `Events/Gateways/Alarms` 的可写边界复核、空状态一致性复核、残余文案口径收口。
   - 验收：`build + smoke + role-boundary + browser e2e(108/108)` 通过，签字记录：`docs/testing/artifacts/web-admin-role-boundary-signoff-20260416.md`。
2. `FE-P0`（本轮已完成）：`F6` 三域结构一次性收口
   - 子项：`directory/policies/grants` 独立页面壳落地、`/access/:section` 兼容重定向落地、共享编排层持续复用。
   - 本轮进展：在既有组件化拆分基础上，新增 `AccessDirectoryPage/AccessPoliciesPage/AccessGrantsPage` 三个独立页面壳，并将 `AccessLegacySectionRedirectPage` 作为 `/:section` 兼容入口；路由层不再把三域直接绑定到同一路由页面实例。
   - 验收：`build + smoke + role-boundary + browser e2e(108/108)` 全通过，F6 转为已完成状态。
3. `FE-P1`（本轮已完成）：`F5` 中文排版与视觉密度收口
   - 子项：按钮/表格/页签密度统一、中英文混排与长字段省略规范化、关键仪表盘视觉层级对齐。
   - 本轮进展：落地 `mp-page-eyebrow/title/description` 与 `mp-kpi-note` 统一标题/说明密度，统一 `button/tabs/card/table` 基线，并新增 `TableCellText` 规范化长字段省略（`max-width + truncate + title`）后覆盖到 `Events/Alarms/Gateways/Wallet/Access` 关键台账。
   - 验收：`build + smoke + role-boundary + browser e2e(108/108)` 全通过，F5 转为已完成状态。
4. `FE-P1`（本轮已收口，持续守护）：`F7` 增量改动回归守护
   - 子项：继续维护 `route + guard + mainflow + interaction + browser e2e`。
   - 本轮进展：已按“分支级集中验证”完成 `build + smoke + role-boundary + browser e2e(108/108) + doc-marker`，并将首轮环境超时抖动重跑收敛为稳定通过。

后端（Cloud/API）优先级：

1. `BE-P0`（立即）：`R6` 印尼 legacy Wiegand 增强网关 PoC 细化落地
   - 子项：`WG-Branch-A/B/C` 任务拆解已完成；`WG-Branch-A` 已补 A4 审计一致性与遥测/探测治理字段，下一步集中推进 A2/A3 实体闭环。
   - 验收：单门 PoC（A）任务闭环、2-4 门 pilot（B）任务可执行、OSDP 升级（C）路径可追溯。
2. `BE-P0`（立即）：`R1` 网关合同链路持续收敛
   - 子项：`queue_hint/retry_subset/checkpoint` 回归脚本留档与告警口径稳定。
   - 本轮验证：`/bin/zsh docs/testing/curl-gateway-event-retry-subset-mixed.zsh` 与 `/bin/zsh docs/testing/curl-gateway-event-checkpoint-partial.zsh` 均通过。
   - 验收：关键脚本持续通过，checkpoint 单调/上界保护无回退。
3. `BE-P0`（立即）：`R8` Controller 单门闭环与协议兼容推进
   - 子项：继电器/门磁/REX/防拆联调、`Wiegand + OSDP` 参数基线固化、实体链路留档。
   - 验收：单门门控连续运行 30 分钟稳定，`legacy-wiegand-poc + idempotency + retry-subset + checkpoint` 留档通过。
4. `BE-P1`（持续）：`R2` nightly 稳定性签字
   - 子项：保持增量回放曲线留档与 `>=7` 天签字。

当前仍依赖外部 API 的事项（不纳入本轮“先解决”范围）：

1. 前端：`F2` 的 `HRIS/SCIM` 返回语义校验；`Wallet/Access` 投递回执字段映射增强。
2. 后端：`R3-WhatsApp API` 真通道；`R7-Google Wallet API`（LEI）。

验证后剩余待推进（高优先级且无外部依赖）：

1. 前端：无新增高优先级阻塞项；仅保留 `F7` 增量回归守护持续执行（`route + guard + mainflow + interaction + browser e2e`）。
2. 后端：`R6/WG-Branch-A` 继续推进 A2/A3（本地放行闭环、门态链路）并补脚本化验收；A4 审计字段一致性已完成。
3. 后端：`R1` 合同链路守护持续执行（`queue_hint/retry_subset/checkpoint` 回归脚本留档）。
4. 边端：`R8` Controller 单门闭环联调（继电器/门磁/REX/防拆 + 30 分钟稳定性留档 + `Wiegand/OSDP` 参数基线）。

## 6. 挂起事项（等待条件恢复）

1. `R3-WhatsApp API`：Meta 企业号真实联调与生产凭证接入  
   恢复条件：你提供可用企业号与 API 条件后恢复推进。

2. `R7-Google Wallet API`：真实发卡写接口接入与验收  
   恢复条件：LEI/外部资质满足且你确认恢复后推进。
