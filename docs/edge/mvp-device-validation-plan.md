# Edge Controller MVP 设备验证与硬件路线（截至 2026-04-17）

## 0. 读法与分级

- 能力状态标识口径：
  - 统一标识见 `docs/architecture/capability-status-markers.md`。
  - 本文常用：`CONTRACT_READY`（Cloud 合同与脚本可联调）、`PROD_READY`（方向冻结并可进入工程实现）、`BLOCKED_EXTERNAL`（受外部 API/资质阻塞）。
- 状态定义：
  - `已完成`：设备/步骤已完成并可复现。
  - `进行中`：已启动但尚未形成可重复结果。
  - `未完成`：尚未开始或前置条件不足。
- 进度口径：
  - `0-100%` 代表“台架硬件 + 门控链路 + 协议兼容 + 文档沉淀”的综合完成度。
  - 当前聚焦 Controller MVP，不等同于量产级硬件验收。

## 1. 总览（按 PRD v0.2 调整）

| 编号 | 事项 | 当前状态 | 进度 | 外部依赖 | 下一里程碑 |
|---|---|---|---:|---|---|
| D1 | 台架硬件清单冻结与验收归档 | 进行中 | 96% | 无 | 2026-04-18：完成序列号/照片/接口模式归档 |
| D2 | 单门本地闭环（relay + 门磁 + REX + tamper） | 进行中 | 45% | 无 | 2026-04-20：连续运行 30 分钟稳定 |
| D3 | Reader 兼容层 MVP（Wiegand/OSDP） | 进行中 | 40% | 无 | 2026-04-21：形成可复用参数基线与异常清单 |
| D4 | Controller 2D/4D 工程化输入（BOM/I/O/电源防护） | 进行中 | 35% | 无 | 2026-04-22：冻结 EVT 前置规格清单 |
| D5 | Cloud SaaS 对齐（策略包/版本ACK/补传重放） | 进行中 | 80% | 无 | 2026-04-20：完成本地判定优先链路签字 |
| D6 | Partner 模式边界（Integration Hub，不自研 Reader） | 进行中 | 55% | HID Partner 资质仅影响后续联调，不阻塞主线 | 2026-04-24：完成接口边界和数据映射清单 |

综合进度：`58%`（`进行中`）

状态说明：

- `PROD_READY` 产品方向已冻结为“Cloud Access SaaS + Edge Controller（2D/4D）+ Reader 兼容层”，不再把自研 Reader 作为当前优先。
- `CONTRACT_READY` Cloud 侧 `config/authz_cache/events/checkpoint` 合同链路可直接承接实体联调。
- 当前高优先推进项无外部 API 阻塞；HID/Wallet 联调属于后续增强路径。

## 2. 当前台架硬件清单（已到位）

| 序号 | 设备 | 规格/版本 | 用途 | 当前状态 |
|---|---|---|---|---|
| 1 | 香橙派 Zero 3 开发板套件 | 1G，含 USB 拓展板、32G 卡、电源、网线、外壳 | 台架主控（仅用于 MVP 验证） | 已到位（2026-04-16） |
| 2 | 2 路光耦隔离模块 | 2 路，3.3V/5V | 门磁/REX 输入隔离 | 已到位（2026-04-16） |
| 3 | 微雪 USB to RS485 | FT232RNL | RS485 总线实验与 OSDP/私有串口预研 | 已到位（2026-04-16） |
| 4 | 杜邦线（公对母、公对公） | 40P，20cm | 台架临时接线 | 已到位（2026-04-16） |
| 5 | 龙杰 ACS Wallet Mate II | 读卡设备 | 可选读卡输入源（非产品主路线） | 已到位（2026-04-16） |

说明：以上设备用于“台架验证与协议联调”。产品路线不等于“Orange Pi + WalletMate 量产化”。

## 3. 方向校正（基于 PRD v0.2）

### 3.1 硬件方向

1. 不优先自研 Reader，优先做 `Controller 2D/4D`。
2. Reader 侧坚持“兼容层策略”：V1 吃稳 `Wiegand + OSDP`，不做 Wallet 协议终结。
3. 现场主链路是“本地判定 + 本地开门 + 异步上云”，云端不是实时放行阻塞点。
4. 老项目改造优先：保留 Reader，替换/旁挂控制器实现上云。
5. Partner Wallet 模式通过 Integration Hub 对接，不影响控制器主线交付。

### 3.2 云 SaaS 方向（与硬件联动）

1. 多租户与门禁域模型：`organization -> building -> floor -> tenant -> door`。
2. 策略分发：版本化、ACK、重试、回滚、差量下发。
3. 设备运维：注册激活、健康监控、远程配置、OTA。
4. 事件闭环：断网补传、幂等去重、checkpoint 单调保护。
5. Integration Hub：合作伙伴 externalId 映射、凭证生命周期回调、账单/用量对账（V1.5）。

## 4. 推进分支与子项（可并行）

### Branch A：Legacy Retrofit（主战场）

1. A1. Wiegand 输入适配
   - 子项：26/34/37/custom bitstream 解析、keypad bitstream 归一化、LED/Buzzer 驱动口径。
2. A2. 本地放行闭环
   - 子项：本地策略缓存命中、时段/封禁判定、离线 fallback、签名与防重放校验。
3. A3. 单门 I/O 事务闭环
   - 子项：继电器动作、门磁状态、REX、tamper、door-held/timeout 事件统一上报。
4. A4. 运营闭环
   - 子项：异常恢复（断网/抖动/重启）时序模板、现场排障日志模板、审计字段一致性。

### Branch B：Cloud-Native Controller（2D/4D）

1. B1. SKU 与资源模型
   - 子项：2D/4D 门数、reader 上限、每门固定 I/O 资源矩阵。
2. B2. 平台分层
   - 子项：Linux MPU（云连接/OTA/日志）与 MCU（Wiegand/OSDP/继电器/采样）职责边界。
3. B3. 电气与防护
   - 子项：24V 主供电、锁驱 12V/24V、ESD/Surge 保护点、掉电检测与 UPS 输入预留。
4. B4. EVT 前置
   - 子项：BOM 冻结、关键器件替代清单、单门/多门故障域验证项。

### Branch C：Partner-Backed Wallet Mode（增强，不阻塞 V1）

1. C1. Integration Hub 边界
   - 子项：HID Origo 组织/模板映射、externalId 与本地用户模型映射。
2. C2. 生命周期编排
   - 子项：issuance/suspend/revoke/reissue 状态机、callback 回流与审计一致性。
3. C3. 运行策略
   - 子项：功能开关、租户级授权、Partner 用量/计费对账。
4. C4. 风险隔离
   - 子项：Partner API 波动不影响 Controller 本地放行主链路。

## 5. MVP 仍缺的关键硬件（推进 D2/D3 必需）

1. 锁控继电器板（光耦不能直接驱动电锁）。
2. 12V/24V 门锁电源。
3. 门磁。
4. 出门按钮（REX）。
5. 防拆开关（tamper）。
6. 工业级 TF 卡（或等价稳定存储）。
7. 小型 UPS/后备电源（建议）。

## 6. 本轮到下轮执行顺序（无外部依赖）

1. P0：完成 D2 单门闭环接线与 30 分钟稳定运行。
2. P0：完成 D3 `Wiegand + OSDP` 参数基线与异常场景记录。
3. P0：完成脚本化门态链路验收并留档（`legacy-wiegand-poc + door-io-loop`）。
4. P0：把 D2/D3 结果回写到 runbook 与 roadmap（含验收证据路径）。
5. P1：推进 D4（2D/4D 规格与 EVT 前置表）。
6. P1：推进 D6（Integration Hub 数据映射清单），保持“不阻塞 V1 控制器交付”的边界。

## 7. 验收出口（MVP）

1. 单门本地判定链路稳定：断网不影响本地放行与事件排队。
2. `events/batch + retry_subset + checkpoint` 与台架行为一致。
3. Wiegand/OSDP 至少各完成一条可重复实测路径。
4. 文档收口：`接线图 + 参数基线 + 异常处理 + 回归命令 + 证据索引` 齐备。
