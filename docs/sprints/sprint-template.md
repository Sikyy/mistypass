# Sprint 模板（能力 + 归属 + 前置依赖 + 里程碑）

## 状态标识用法（必填）

- 在“目标能力”与“里程碑”中使用统一能力状态标识：`PROD_READY | CONTRACT_READY | SKELETON_ONLY | BLOCKED_EXTERNAL`。
- 每个能力点只标一个主状态，若受外部依赖阻塞优先标注 `BLOCKED_EXTERNAL` 并写明恢复条件。

## 0. 基本信息

- Sprint 名称：`<例如：R1-Edge-Queue-Executor>`
- 时间窗口：`<YYYY-MM-DD ~ YYYY-MM-DD>`
- 紧急度：`S0 | S1 | S2 | S3`
- 当前状态：`未开始 | 进行中 | 阻塞 | 已完成`
- 负责人（Owner）：`<主负责人>`
- 协作方：`<模块/团队/角色>`

## 1. 目标能力（Capability）

- `PROD_READY` 能力 1：`<一句话描述可交付能力>`
- `CONTRACT_READY` 能力 2：`<一句话描述可交付能力>`
- `BLOCKED_EXTERNAL` 能力 3（可选）：`<外部依赖未满足时填写>`
- 验收口径：`<功能、稳定性、可观测的最小标准>`

## 2. 归属与边界（Ownership / Scope）

- Cloud 负责：
  - `<API/编排/审计/运维指标等>`
- Edge 负责：
  - `<设备侧判定/本地缓存/本地队列/协议适配等>`
- Out of Scope：
  - `<本 Sprint 明确不做的事项>`

## 3. 前置依赖（Dependencies）

- 外部依赖：
  - `<证书/企业号/第三方 API 资质/硬件到位>`
- 内部依赖：
  - `<上游模块、共享组件、数据模型变更>`
- 阻塞项与回退策略：
  - `<若依赖未满足时的替代路径>`

## 4. 交付拆解（Deliverables）

- D1：`<接口/功能>`  
  完成定义：`<done criteria>`
- D2：`<脚本/测试>`  
  完成定义：`<done criteria>`
- D3：`<文档/运维项>`  
  完成定义：`<done criteria>`

## 5. 里程碑（Milestones）

1. `M1 <日期>`：`<可验证产出>`
2. `M2 <日期>`：`<可验证产出>`
3. `M3 <日期>`：`<可验证产出>`

## 6. 回归与验收（Regression / DoD）

- 回归脚本：
  - `docs/testing/<script-a>.zsh`
  - `docs/testing/<script-b>.zsh`
- 指标阈值：
  - `<成功率/延迟/失败率/吞吐等>`
- DoD：
  - `<功能、性能、审计、文档、CI 全部通过>`

## 7. 风险与缓解（Risks）

- 风险 1：`<描述>`  
  缓解：`<措施>`
- 风险 2：`<描述>`  
  缓解：`<措施>`

## 8. 决策记录（Decision Log）

- `<YYYY-MM-DD>`：`<决策内容 + 原因 + 影响范围>`
