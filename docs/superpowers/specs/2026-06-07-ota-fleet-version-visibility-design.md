# OTA 舰队版本可见性 — 设计文档

> 日期：2026-06-07
> 状态：设计已确认,待出实施计划
> 上层目标：**全面对标 Kisi 的舰队级 OTA 能力**。本文是该程序的**子项 #1（地基)**。

---

## 1. 背景

刚完成的 OTA 签名（PR #145）做齐了**安全核心**(强制签名、验签后才刷、双层回滚),但缺**舰队管理层**。对标 Kisi 级 OTA 拆为 5 个子项:**#1 固件版本可见性(本文)→ #2 固件制品仓库 → #3 舰队批量+灰度发布 → #4 调度+自动更新 → #5 管理 UI**(#6 健壮性桶后置)。#1 是地基:其它子项都要先知道"每台网关跑什么固件版本"。

### 关键现状(已核实)
- **配置版本 ≠ 固件版本**,是两条独立轴:
  - 现有 `desired_version`/`applied_version`(`GatewayConfigSnapshot`,`MarkConfigApplied`,config/pull 的 `in_sync`)追踪的是 **agent 拉取的配置快照版本**。
  - OTA 的**固件/二进制版本**是另一条轴:OTA 任务有 `FirmwareVersion`,agent 运行时有编译期注入的 `agentVersion`。
- **网关设备记录上目前没有"当前固件版本"字段**(`Gateway` struct,`api/internal/modules/gateway/service.go:176`;只有 `Status`/`LastSeenAt`)。
- agent 的 `pullConfig` 请求体只发 `current_version`(配置版本,且当前为 `""`)+ `authz_cache_version`,**不发固件版本**。

→ #1 新增一条**独立的固件版本轴**,不去复用/污染配置版本机制。

---

## 2. 目标与非目标

### 目标
- agent 上报其运行固件版本(`agentVersion`)给云端。
- 云端按网关持久化:当前固件版本 + 上报时间。
- 暴露给管理端:每台网关的当前固件版本 + 上报时间 +(detail)最近一次 OTA 状态;以及一个**舰队版本分布**汇总。

### 非目标(YAGNI / 留给后续子项)
- "是否过期 / 待更新"判定 → 需要 target 版本,留给 **#3 rollout**。
- 固件存储/分发/版本目录 → **#2 仓库**。
- 前端展示 → **#5 UI**(本文纯后端 API)。
- 配置版本(`current_version: ""`)的观测性 → 与固件无关,不在本文范围。

---

## 3. 设计决策:上报通道 = 搭车 pullConfig

agent 把 `agentVersion` 加进**现有 pullConfig 请求体**(`firmware_version` 字段),服务端在处理 config/pull 时顺手存进网关记录。

**理由 / 备选:**
- ✅ 复用每 ~30s 一次的已鉴权轮询;服务端该路径本就在更新网关在线状态;改动最小、数据最新鲜。
- ❌ 专用 `/gateway/firmware/report` 端点:多一个端点 + 每轮多一次请求,相比搭车没必要。
- ❌ 只在注册/OTA 确认时报:省的流量微不足道(搭车几乎免费),且二进制非 OTA 变更时会过期。

---

## 4. 组件设计(4 个单元,纯后端)

### 4.1 数据模型 — `Gateway` struct 新增两字段
`api/internal/modules/gateway/service.go:176` 的 `Gateway` struct 加:
```go
	CurrentFirmwareVersion string    `json:"current_firmware_version,omitempty"`
	FirmwareReportedAt     time.Time `json:"firmware_reported_at,omitempty"`
```
(与配置版本 `desired/applied_version` 互不干扰。)

### 4.2 单元 1 — agent 上报(`api/cmd/gateway-agent/agent.go`)
`pullConfig` 的请求体 map 加一项:`"firmware_version": a.agentVersion`。(一行;`agentVersion` 已由 ldflags 注入。)

### 4.3 单元 2 — 服务端捕获(`api/internal/http/router_handlers_gateway.go`)
`gatewayBootstrapConfigPull` 的 `request` struct 加 `FirmwareVersion string json:"firmware_version"`;在设备鉴权通过后调用:
```go
_ = s.gatewaySvc.RecordFirmwareVersion(request.TenantID, request.GatewayID, request.FirmwareVersion)
```
(失败仅记日志,不影响 config/pull 主流程。)

### 4.4 单元 3 — 存储 + 汇总(`api/internal/modules/gateway/service.go`)
```go
// RecordFirmwareVersion stores the gateway's reported running firmware version.
// Empty version is ignored (an older agent that doesn't report must not clobber it).
func (s *Service) RecordFirmwareVersion(tenantID, gatewayID, version string) error

// FirmwareSummary returns the firmware-version distribution across a tenant's gateways.
func (s *Service) FirmwareSummary(tenantID string) GatewayFirmwareSummary
```
```go
type GatewayFirmwareSummary struct {
	Total    int                       `json:"total"`              // gateways in tenant
	Reported int                       `json:"reported"`           // with a non-empty firmware version
	Versions []GatewayFirmwareVersionCount `json:"versions"`       // sorted desc by count
}
type GatewayFirmwareVersionCount struct {
	Version string `json:"version"`
	Count   int    `json:"count"`
}
```
`RecordFirmwareVersion`:trim version;空 → 直接返回 nil(no-op);按 `tenantID` 定位网关(租户不符 → `ErrGatewayNotFound`);写 `CurrentFirmwareVersion` + `FirmwareReportedAt = now`;`persistLocked()`。

### 4.5 单元 4 — 暴露(HTTP)
- **网关 list/detail**:若列表/详情直接返回 `Gateway` 结构,新增的两个 json 字段自动带出;若中间用了 DTO,在该 DTO 上补这两个字段(实施期核对实际 handler)。detail 额外带 `latest_ota_status`:从 `ListOTATasks(tenant, gw)` 取最新一条的 `Status`(没有则空)。
- **新端点** `GET /api/v1/gateways/firmware-summary`:handler `gatewayFirmwareSummary` → `resolveTenantID` → `s.gatewaySvc.FirmwareSummary(tenant)` → 200 JSON。角色门槛与 `listGatewayOTATasks` 一致(`super_admin`/`tenant_admin`/`operator`/`building_admin`)。路由注册在 `router.go` 网关分组内。

---

## 5. 数据流
```
agent 每 ~30s pullConfig（body 含 firmware_version=agentVersion）
  → 服务端 config/pull handler → RecordFirmwareVersion(tenant, gw, version)
       → gateway.CurrentFirmwareVersion = version; FirmwareReportedAt = now; 持久化
管理端 GET /gateways（list/detail）→ 每台带 current_firmware_version + firmware_reported_at(+ detail latest_ota_status)
管理端 GET /gateways/firmware-summary → {total, reported, versions:[{version,count}...]}
```
OTA 自更新后,新二进制下一轮 pullConfig 自然上报新 `agentVersion` → 版本自动刷新,无需特殊处理。

---

## 6. 错误处理 / 边界
| 情况 | 行为 |
|---|---|
| 空 / 全空白 firmware_version(旧 agent) | `RecordFirmwareVersion` no-op,不覆盖已有版本 |
| 租户不符 | `ErrGatewayNotFound`,config/pull 主流程不受影响(仅记日志) |
| `firmware_reported_at` | 兼作"固件心跳":太久没报可视作掉线/版本陈旧(判定逻辑留给 UI/#3) |
| 租户隔离 | `FirmwareSummary` 仅统计该租户网关 |

---

## 7. 测试
- **service**(`service_test.go`):`RecordFirmwareVersion` 写入两字段;空版本 no-op(不清空已有);租户不符返回 `ErrGatewayNotFound`。`FirmwareSummary` 按版本分组计数、`total`/`reported` 正确、按 count 降序。
- **http**(`internal/http`):config/pull 带 `firmware_version` 后,网关记录/列表能读到该版本;`GET /gateways/firmware-summary` 返回正确分布 + 租户隔离。

---

## 8. 改动文件
**修改**
- `api/internal/modules/gateway/service.go` — `Gateway` 加两字段;`RecordFirmwareVersion` + `FirmwareSummary` + 两个汇总类型
- `api/internal/modules/gateway/service_test.go` — service 测试
- `api/internal/http/router_handlers_gateway.go` — config/pull 请求体加 `FirmwareVersion` + 调 `RecordFirmwareVersion`
- `api/internal/http/routes_gateway_management.go` — `gatewayFirmwareSummary` handler(+ gateway detail 带 `latest_ota_status`)
- `api/internal/http/router.go` — 注册 `GET /gateways/firmware-summary`
- `api/cmd/gateway-agent/agent.go` — `pullConfig` 请求体加 `firmware_version`
- 相应 http 测试文件

---

## 9. 工作量
约 0.5 天(纯后端、改动小)。是后续 #2/#3 的数据地基。
