# C 区 Sprint 执行计划（模板化版，2026-04-14）

## 0. 基本信息

- Sprint 名称：`c-zone-production-rollout`
- 时间窗口：`2026-04-14 ~ 2026-05-31`
- 紧急度：`S1`
- 当前状态：`进行中`
- 负责人（Owner）：`Cloud Backend（MistyPass）`
- 协作方：`Edge 固件`、`企业实施`、`QA`、`运维`

## 1. 目标能力（Capability）

- 能力 1：企业身份域名识别、OIDC/SAML 登录与员工同步形成稳定生产链路。
- 能力 2：数字凭证分发（Email/WhatsApp）与实体卡流程具备运营闭环与失败补偿。
- `BLOCKED_EXTERNAL` 能力 3：WalletMate II 读卡验证与云边审计链路可追溯（设备已到位，待接口模式确认与台架窗口）。
- 验收口径：关键链路均具备 API 合同、回归脚本、审计记录与失败重试路径。

## 2. 归属与边界（Ownership / Scope）

- Cloud 负责：
  - 租户/域名解析、IdP 配置校验与登录编排。
  - 员工同步、权限分配、凭证发送与发送回执留档。
  - 审计日志、运营指标、回归脚本与 CI 冒烟编排。
- Edge 负责：
  - 本地授权缓存、本地事件队列执行器、断网补传、读卡器协议适配。
  - WalletMate II 设备侧参数与验卡行为实现。
- Out of Scope：
  - 真实 Google Wallet 发卡写接口生产接入（受 LEI 影响）。
  - WhatsApp Meta 企业号真实联调（外部资质待完成）。
  - MVP 网关设备实体测试（设备已到位，待接口模式确认与测试窗口）。

## 3. 前置依赖（Dependencies）

- 外部依赖：
  - 企业 IdP 测试租户账号与回调域名。
  - Wallet/发卡相关资质（LEI）与 Meta 企业 API 资质。
  - WalletMate II 与台架设备到位（2026-04-16 已到货确认，待接口模式确认）。
- 内部依赖：
  - 网关事件补传与 checkpoint 基线稳定（R1/R2）。
  - Wallet 队列、重试、DLQ、告警发送链路稳定（R3）。
- 阻塞项与回退策略：
  - LEI 或 Meta 资质未满足时，保持 `mock + resend` 路径持续交付，不阻塞主线。

## 4. 交付拆解（Deliverables）

- D1：企业域名识别与登录生产化（Sprint 0/1/2）  
  完成定义：`tenant resolve + idp config/validate + OIDC 登录兑换` 回归通过，审计字段完整。
- D2：员工同步与权限自动分配（Sprint 3）  
  完成定义：新增/变更/离职同步闭环可跑通，失败补偿可追踪。
- D3：凭证下发与发送回执（Sprint 4）  
  完成定义：Email `resend` 稳定可用，WhatsApp `mock` 可回归，发送失败可重试与留档。
- D4：实体卡制作与状态联动（Sprint 4）  
  完成定义：制卡/发卡/挂失/补卡 API 可观测，实体卡与数字卡状态一致性可验证。
- D5：WalletMate II 实机闭环（Sprint 5）  
  完成定义：开门成功、挂起拒绝、撤销拒绝、跨租户拒绝、离线补偿用例齐备（设备已到位，待接口模式确认后执行）。
- D6：上线稳定性与回滚准备（Sprint 6）  
  完成定义：SLO/阈值/容量基线明确，灰度与回滚演练留档。

## 5. 里程碑（Milestones）

1. `M1 2026-04-18`：企业登录与同步链路验收清单冻结。
2. `M2 2026-04-25`：员工同步补偿 + 告警链路稳定回归。
3. `M3 2026-05-02`：凭证下发（`resend + mock`）与失败重试闭环。
4. `M4 2026-05-09`：实体卡流程 API 与状态联动验收。
5. `M5`：WalletMate II 台架验证首轮通过（待接口模式确认与测试窗口后重排日期）。
6. `M6 2026-05-31`：稳定性复核与上线包准备完成。

## 6. 回归与验收（Regression / DoD）

- 回归脚本：
  - `docs/testing/curl-enterprise-sync-access-batch.zsh`
  - `docs/testing/curl-enterprise-sync-worker-alert.zsh`
  - `docs/testing/curl-wallet-job-alert-dispatch.zsh`
  - `docs/testing/curl-wallet-job-alert-dispatch-retry.zsh`
  - `docs/testing/curl-wallet-job-alert-dispatch-resend.zsh`
  - `docs/testing/curl-wallet-job-alert-dispatch-whatsapp.zsh`
  - `docs/testing/curl-gateway-edge-queue-executor-sim.zsh`
- 指标阈值：
  - 登录/同步/告警发送链路失败率与延迟阈值在 CI 与手工回归中可复核。
- DoD：
  - 关键接口、回归脚本、审计日志、运维文档、CI 冒烟全部通过。

## 7. 风险与缓解（Risks）

- 风险 1：外部资质（LEI/Meta）延迟导致真实通道无法验收。  
  缓解：保持 `resend + mock` 路径可发布，真实通道独立排期。
- 风险 2：Edge 设备到位延迟导致实机验证滞后。  
  缓解：先完成 API 合同仿真与台架脚本，设备到位后直接执行实测。
- 风险 3：跨团队文档口径不一致造成交付偏差。  
  缓解：统一使用 `docs/sprints/sprint-template.md` 并维护决策记录。

## 8. 决策记录（Decision Log）

- `2026-04-14`：WhatsApp Meta 真实通道联调暂挂起，主线先走 `resend + mock`。
- `2026-04-14`：Sprint 文档统一采用模板结构（能力/归属/依赖/里程碑）。
- `2026-04-14`：R1 执行器仿真脚本纳入 CI；实体设备链路验证待恢复条件后继续。
- `2026-04-14`：按最新排期要求，WhatsApp API、Google Wallet API、MVP 网关设备测试三类事项统一挂起，待你提供恢复条件后继续。

## 9. 历史 Sprint 分段映射（用于追踪）

- Sprint 0：范围冻结与验收口径建立。
- Sprint 1：企业域名识别增强（含 `tenant resolve`）。
- Sprint 2：企业登录生产化（OIDC/SAML）。
- Sprint 3：员工同步生产化与权限自动分配。
- Sprint 4：凭证下发与实体卡制作。
- Sprint 5：WalletMate II 实机闭环。
- Sprint 6：稳定性与上线准备。
