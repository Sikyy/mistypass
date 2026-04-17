# 网关序列号、协议兼容与 Misty Access 规划（2026-04-17）

## 0. 云边职责边界（强约束）

- 开门判定链路必须在网关本地闭环，不能依赖云端实时 round-trip。
- 云端负责策略编排、凭证下发、设备管理、审计与运营；网关负责现场实时执行、离线可用与协议适配。
- 云边交互按“异步 + 幂等”设计：策略下发与事件回传允许短暂延迟，但不影响门级实时控制。

边界落地要求：

- 任何“是否放行”的实时判断必须由网关依据本地缓存策略与本地时钟完成。
- 云端不可成为开门动作的同步阻塞依赖；云端仅负责后续审计、对账和策略收敛。

能力状态标识口径：

- 统一标识见 `docs/architecture/capability-status-markers.md`。

## 1. 当前待做清单（按 PRD v0.2 汇总）

### 1.1 P0（当前优先，无外部依赖）

- 控制层 + 云层 + 兼容层主线：
  - 控制器本地判定优先（云端不做开门实时阻塞）。
  - Reader 兼容层吃稳 `Wiegand + OSDP`（含 LED/Buzzer 反馈控制）。
  - `config/pull -> config/applied -> events/batch -> checkpoint` 版本 ACK 与幂等补传持续收敛。
- Cloud SaaS 主链路：
  - 多租户组织层级（`organization -> building -> floor -> tenant -> door`）。
  - 设备运维（注册/激活/健康/OTA/差量配置）。
  - 审计、导出、Webhook 与运维指标闭环。

### 1.2 P1（并行推进）

- `Legacy Retrofit`（印尼老 Wiegand 改造）：
  - 2-4 门集中控制、离线策略包、门级故障隔离。
  - 线路质量治理、探测/降级策略、运维告警口径统一。
- `Cloud-Native Controller`（2D/4D）：
  - SKU 资源矩阵（门数、reader 上限、I/O 固定资源）。
  - 电源、防护、掉电与恢复时序的工程化约束。

### 1.3 P2（增强模式，不阻塞 V1）

- `Partner-Backed Wallet Mode`：
  - Integration Hub 对接 HID Origo（externalId 映射、生命周期 callback、状态回流）。
  - 钱包能力由 Partner Reader 生态提供；控制器持续只处理 `Wiegand/OSDP` 事件。
  - Partner API/资质仅影响增强模式，不阻塞 V1 控制器与 SaaS 主线交付。

## 2. 当前已落地的序列号注册与校验逻辑

### 2.1 网关序列号（`POST /api/v1/gateways/register`）

- 规则：
  - 必填，去空格后转大写。
  - 必须以 `MP-GW-` 开头。
  - 长度范围：`9~64`。
  - 仅允许字符：`A-Z`、`0-9`、`-`。
- 冲突策略：
  - 与现有网关序列号按大小写不敏感判重。
  - 冲突返回 `409 gateway serial_number already registered`。
  - 若序列号未在库存白名单、租户不匹配、已被核销，返回 `409`。
- 错误码：
  - 格式非法返回 `400 serial_number format is invalid`。

### 2.3 序列号库存管理（当前已实现）

- `GET /api/v1/gateways/serial-inventory?tenant_id=...`
- `POST /api/v1/gateways/serial-inventory/import`
- `POST /api/v1/gateways/serial-inventory/import-csv`
- `PATCH /api/v1/gateways/serial-inventory/batch-status`
- `PATCH /api/v1/gateways/serial-inventory/{serialNumber}/status`
- `GET /api/v1/gateways/serial-inventory/export-csv`
- 注册前先入库（`status=available`），注册成功后核销（`status=consumed` + `consumed_gateway_id` + `consumed_at`）。
- 支持库存状态流转：`available`（回库可复用）、`frozen`（冻结不可注册）、`scrapped`（报废不可注册）。
- 支持批量状态流转（冻结/回库/报废），用于生产批次异常快速止损。

### 2.2 下挂设备序列号（`POST /api/v1/gateways/{gatewayID}/devices`）

- 规则：
  - 设备序列号必填。
  - `kind/source/status/protocol` 均做枚举校验。
  - 网关容量上限按 `device_capacity`（4/8）控制。
- 冲突策略：
  - 同一设备序列号不允许跨网关重复注册。
  - 冲突返回 `409 device serial_number already registered on another gateway`。
- 持久化：
  - 设备协议写入 `mistypass_gateway_devices.protocol`，重启后保持一致。

## 3. Wiegand / OSDP / RS485 / BLE 兼容策略（当前实现）

### 3.1 默认协议归一化

- 旧设备（`legacy_integration` 或 `legacy_*`）未显式传协议时，默认 `wiegand_26`。
- 新设备（非 legacy）未显式传协议时，默认 `osdp_v2`。
- 显式支持：`wiegand_26`、`wiegand_34`、`osdp_v2`、`rs485`、`ble`。
- `protocol=rs485` 时支持 `rs485_config`：
  - `baud_rate`：`1200/2400/4800/9600/19200/38400/57600/115200`
  - `parity`：`none/even/odd`
  - `stop_bits`：`1/2`
  - `device_address`：`1..247`
  - `timeout_ms`：`100..5000`
- RS485 运行态监测接口：
  - `POST /api/v1/gateways/{gatewayID}/devices/{deviceID}/rs485/telemetry`
  - 支持上报 `retries/timeouts/collisions/last_error/reset_consecutive_timeouts`
  - 当 `consecutive_timeouts >= 3` 或 `collision_count >= 5` 时写入审计告警 `gateway_rs485_health_alert`
- 非法协议返回 `400 invalid gateway device protocol`。

### 3.2 协议分工建议

- `Wiegand`：用于存量改造接入，重点处理线路噪声、方向性限制、无加密风险。
- `OSDP v2`：新部署默认协议，支持更完整设备管理和安全能力，作为主推线路。
- `RS485`：适用于半双工总线场景（含部分旧设备链路），建议在网关侧统一做超时与重传策略。
- `BLE`：移动端近场能力入口，建议与网关鉴权/时效 token 结合使用。

### 3.2.1 印尼老 Wiegand 客户增强网关（Kisi Controller 增强版）

方案目标：在不强制更换现场读卡器与布线的前提下，把“明文 Wiegand”升级到“短距存量接入 + 网关加密上云 + 可演进 OSDP”。

判断与修正：

1. 可行：网关增加 `Wiegand` 输入与解码链路，先兼容老读卡器，改造成本最低。
2. 可行：网关增加 `RS485/OSDP` 端口用于后续读头升级；但“自动识别”应改为“按设备能力配置/协商”。
3. 需修正：“所有决策在 SaaS 云端”不适合作为默认模式。门控放行必须由网关本地闭环（签名缓存 + 本地时钟 + 反重放），云端负责编排与审计。
4. 可行：`Ethernet/PoE + TLS` 出站链路是主推方案；建议 `mTLS` + 设备独立证书，而不是共享 token。
5. 可行：一台网关管理 `2-4` 门在 PoC/Pilot 阶段可落地，但需按 I/O 隔离、电源预算和故障域评估最终容量。
6. 可行：`tamper/门磁/REX/离线队列` 必须纳入，才能从 demo 提升为可运营原型。

低成本落地建议：

1. 物理部署：网关进带锁机柜，门外仅保留读头与必要状态件；机柜必须接 tamper。
2. 兼容改造：老读头优先短距接入 Wiegand，网关内部完成归一化与上传；高风险线路优先替换为 OSDP。
3. 升级路径：同一网关同时支持 `Wiegand + OSDP`，按门逐步替换读头，避免整站停机。
4. 安全基线：命令签名（`command_id + expires_at + signature`）、离线缓存签名、设备唯一身份证书、事件防重放计数器。

建议分支（可并行）：

1. `WG-Branch-A（PoC）`：单门 Wiegand 接入 + 云端审计闭环 + 本地缓存放行。
2. `WG-Branch-B（Pilot）`：2-4 门集中控制 + 多门离线策略包 + 门级故障隔离。
3. `WG-Branch-C（Upgrade）`：按门替换为 OSDP Secure Channel，并保留 legacy 门的兼容路径。

### 3.2.2 WG 分支任务拆解（无外部 API 依赖）

`WG-Branch-A（PoC）`：

1. A1. 网关输入适配层：Wiegand bitstream -> `credential_ref` 归一化（含校验失败分类）。
2. A2. 本地放行闭环：签名缓存 + 本地时钟 + 反重放计数器（云端仅编排不阻塞）。
3. A3. 单门门态链路：继电器、门磁、REX、防拆事件统一上报。
4. A4. 审计闭环：`grant/deny/timeout/tamper` 全链路审计字段一致性。
5. 当前进展：已新增并通过 `docs/testing/curl-gateway-legacy-wiegand-poc.zsh`，覆盖 legacy/new 设备默认协议、`probe-legacy` 建议和 `events/access` 幂等去重。
6. 当前进展：已新增并通过 `docs/testing/curl-gateway-door-io-loop.zsh`，覆盖单门 `rex/tamper/timeout` 设备事件链路、幂等回放与审计动作映射。
7. 当前进展补充（2026-04-17）：
   - `rs485 telemetry` 已补统一 `telemetry` 视图（`alert_level/line_quality/governance_action/reason_codes/line_policy`）并新增 `gateway_protocol_health_alert` 审计。
   - `probe-legacy` 已补治理载荷（`legacy_protocol/upgrade_protocol/offline_fallback/degraded_line_action/line_policy`），用于老旧线路降级与升级决策。
   - `access/device/batch` 已补规范审计动作与统一 target 模板，覆盖 `grant/deny/tamper/timeout/rex`。

`WG-Branch-B（Pilot）`：

1. B1. 多门聚合模型：`gateway -> doors -> zones -> schedules` 配置下发与回滚。
2. B2. 离线策略包：多门缓存包版本化、签名、过期与灰度切换。
3. B3. 故障隔离：门级故障不扩散（单门线路异常不阻断同网关其他门）。
4. B4. 运行态指标：每门开门成功率、超时率、线路质量与重试基线。

`WG-Branch-C（Upgrade）`：

1. C1. OSDP 升级基线：Secure Channel 参数模板与能力协商字段。
2. C2. 混合接入运行：同网关并存 `Wiegand + OSDP` 的编排与监控。
3. C3. 迁移顺序：按门滚动替换、可回退、零停机策略。
4. C4. 验收门槛：替换后安全指标提升（重放风险、线路明文暴露、tamper 可见性）。

### 3.3 网关配置同步（已落地最小闭环）

- 云端发布：`POST /api/v1/gateways/{gatewayID}/config/publish` 写入目标版本（异步 queued）。
- 网关拉取：`POST /api/v1/gateway/config/pull` 返回 `desired_version` + `bound_door_ids` + `devices`，并附带 `authz_cache`（按绑定门点过滤的 `doors/policies/temporary_access/visitor_passes/users/user_groups`，含 `version + generated_at + expires_at + ttl_seconds + scope + counts`，并提供 `policy`、`status_codes`、`status` 用于本地过期/回滚决策；请求可上报本地 `authz_cache_version` 参与状态判定）。
- 网关回执：`POST /api/v1/gateway/config/applied` 上报已应用版本，并可回执 `authz_cache_version`；云端返回 `authz_cache.version_match + status`，不一致写审计用于漂移追踪，一致则推进下一次下发的 `policy.rollback_version`。
- 该链路满足“云编排、边执行”：网关按本地版本差异决定是否应用，不依赖开门实时链路联机。

### 3.4 网关事件补传与幂等（已落地）

- 网关上报：`POST /api/v1/gateway/events/access`、`POST /api/v1/gateway/events/device`。
- 批量补传：`POST /api/v1/gateway/events/batch`，支持离线队列一次 flush 并返回 `created/deduplicated` 计数。
- 补传决策提示：`events/batch` 响应附带 `queue_hint`（`checkpoint_id + acked_increment + server_ingested_total + status_code + next_action`）和 `retry_subset.queue`，网关可直接按返回动作推进“重试或上报 checkpoint”（`server_ingested_total` 仅统计新创建记录，纯去重回放不增长）。
- 批量补传支持部分成功：坏数据条目进入 `failed`，不阻断有效条目入库，便于网关重试最小化。
- 幂等键优先级：`idempotency_key` -> `request_id` -> `event_id`。
- 重放同一幂等键会返回同一 `event_id` 且 `deduplicated=true`，避免 `/events/access|device` 重复记录。
- 支持 `occurred_at`（RFC3339）回放原始事件时间，满足断网补传对账需求。
- 网关水位回执：`POST /api/v1/gateway/events/checkpoint`（`acked_count` 对同一 `gateway_id + queue` 必须单调不回退；回退返回 `409` 且携带当前服务端 checkpoint 快照与 `next_action`；若 `acked_count` 大于服务端队列已处理总量也返回 `409` + `server_event_total`），管理侧查询：`GET /api/v1/gateways/{gatewayID}/events/checkpoint`。
- 网关补传摘要看板：`GET /api/v1/gateways/events/checkpoint/summary`，用于按网关/队列观察 `lag_count`。

## 4. Misty Access 与 Partner Wallet 协同规划

### 4.1 现有基础

- 已有 App 侧 API：`/api/v1/app/access/doors`、`/api/v1/app/access/ble-token`、`/api/v1/app/access/logs`。
- `CONTRACT_READY` 可作为 BLE 开门与记录回传的第一阶段契约链路。

### 4.2 分阶段推进（按 PRD v0.2 对齐）

1. Phase 1（先可用，云控边执）：
   - BLE token 短时有效、在线校验优先。
   - 网关统一上报移动端开门事件。
2. Phase 2（增强可靠性，离线优先）：
   - 离线策略（短时本地放行 + 恢复后补传事件）。
   - token 轮换与设备绑定策略（防重放）。
3. Phase 3（Partner Wallet 增强）：
   - SaaS 通过 Integration Hub 对接 Partner 凭证生命周期接口。
   - 控制器保持 `Wiegand/OSDP` 兼容层定位，不直接终结 Apple/Google Reader 协议。
   - 同租户可并存 `Legacy Reader 门 + OSDP 门 + Partner Wallet 门`。

## 5. 本轮新增回归与校验

- 新增脚本：
  - `docs/testing/curl-gateway-serial-protocol.zsh`
  - `docs/testing/curl-gateway-serial-inventory-csv.zsh`
  - `docs/testing/curl-gateway-serial-inventory-batch.zsh`
- 覆盖：
  - 网关序列号格式校验、大小写归一判重。
  - 设备协议默认值（legacy -> `wiegand_26`、modern -> `osdp_v2`）。
  - 显式 `ble`/`rs485` 协议注册。
  - RS485 运行态上报（计数累加、连续超时阈值告警审计）。
  - 非法协议拒绝、跨网关设备序列号冲突拒绝。
  - 序列号库存 CSV 入库/导出校验。
  - 序列号批量冻结/回库/报废与注册/挂载阻断校验。
