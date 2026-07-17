# OTA 舰队版本可见性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 agent 上报其运行固件版本,云端按网关存储并通过 API 暴露每台版本 + 舰队版本分布(全面对标 Kisi OTA 的子项 #1,地基)。

**Architecture:** agent 把已有的 `agentVersion` 搭在每次 `pullConfig` 请求里上报;服务端在 config/pull handler 顺手存进 `Gateway` 记录的两个新字段;网关列表(返回 `[]Gateway`)自动带出这两个字段,另加一个租户级 `firmware-summary` 端点返回版本分布。纯后端,UI 归子项 #5。

**Tech Stack:** Go、chi、内存 service + `persistLocked()` 状态存储。

设计依据:[2026-06-07-ota-fleet-version-visibility-design.md](../specs/2026-06-07-ota-fleet-version-visibility-design.md)

**测试约定:** 所有 `go` 命令在 `api/` 目录下执行。`gateway.NewService()` 预置 `gw_demo_001` / `tenant_demo_jakarta`。

**范围说明:** spec 4.5 提到 detail 带 `latest_ota_status` —— 本计划**省略**它:每台网关的 OTA 状态已由现有 `GET /gateways/{gatewayID}/ota/tasks` 提供(DRY/YAGNI),无需在 #1 重复。

---

## 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `api/internal/modules/gateway/service.go` | `Gateway` 加两字段;`RecordFirmwareVersion` + `FirmwareSummary` + 两个汇总类型 | 修改 |
| `api/internal/modules/gateway/service_test.go` | service 单测 | 修改 |
| `api/internal/http/router_handlers_gateway.go` | config/pull 请求体加 `firmware_version` + 调 `RecordFirmwareVersion` | 修改 |
| `api/internal/http/routes_gateway_bootstrap_test.go` | config/pull 捕获 firmware 的 http 测试 | 修改 |
| `api/internal/http/routes_gateway_management.go` | `gatewayFirmwareSummary` handler | 修改 |
| `api/internal/http/router.go` | 注册 `GET /gateways/firmware-summary` | 修改 |
| `api/internal/http/routes_gateway_ota_test.go` | summary 端点 http 测试 | 修改 |
| `api/cmd/gateway-agent/agent.go` | `pullConfig` 请求体加 `firmware_version` | 修改 |

---

## Task 1: 数据模型 + service(RecordFirmwareVersion + FirmwareSummary)

**Files:**
- Modify: `api/internal/modules/gateway/service.go`
- Test: `api/internal/modules/gateway/service_test.go`

- [ ] **Step 1: 写失败测试**

在 `api/internal/modules/gateway/service_test.go` 末尾追加:

```go
func TestRecordFirmwareVersion(t *testing.T) {
	svc := NewService()
	if err := svc.RecordFirmwareVersion("tenant_demo_jakarta", "gw_demo_001", "1.4.0"); err != nil {
		t.Fatalf("record: %v", err)
	}
	// empty version must be a no-op (an older agent must not clobber a known version)
	if err := svc.RecordFirmwareVersion("tenant_demo_jakarta", "gw_demo_001", "  "); err != nil {
		t.Fatalf("empty record should be no-op, got %v", err)
	}
	found := false
	for _, g := range svc.List("tenant_demo_jakarta") {
		if g.ID == "gw_demo_001" {
			found = true
			if g.CurrentFirmwareVersion != "1.4.0" {
				t.Fatalf("want 1.4.0, got %q", g.CurrentFirmwareVersion)
			}
			if g.FirmwareReportedAt.IsZero() {
				t.Fatal("FirmwareReportedAt should be set")
			}
		}
	}
	if !found {
		t.Fatal("gw_demo_001 not found")
	}
	if err := svc.RecordFirmwareVersion("tenant_other", "gw_demo_001", "1.5.0"); err != ErrGatewayNotFound {
		t.Fatalf("want ErrGatewayNotFound for wrong tenant, got %v", err)
	}
}

func TestFirmwareSummary(t *testing.T) {
	svc := NewService()
	if err := svc.RecordFirmwareVersion("tenant_demo_jakarta", "gw_demo_001", "1.4.0"); err != nil {
		t.Fatal(err)
	}
	sum := svc.FirmwareSummary("tenant_demo_jakarta")
	if sum.Total < 1 {
		t.Fatalf("expected total>=1, got %d", sum.Total)
	}
	if sum.Reported < 1 {
		t.Fatalf("expected reported>=1, got %d", sum.Reported)
	}
	ok := false
	for _, v := range sum.Versions {
		if v.Version == "1.4.0" && v.Count >= 1 {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected 1.4.0 in versions, got %+v", sum.Versions)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'TestRecordFirmwareVersion|TestFirmwareSummary' -v`
Expected: FAIL(`g.CurrentFirmwareVersion undefined`、`svc.RecordFirmwareVersion undefined` 等)。

- [ ] **Step 3: 给 `Gateway` struct 加两字段**

在 `api/internal/modules/gateway/service.go` 的 `type Gateway struct`(`BoundDoorIDs` 字段那行之后、struct 右括号之前)加入:

```go
	CurrentFirmwareVersion string    `json:"current_firmware_version,omitempty"`
	FirmwareReportedAt     time.Time `json:"firmware_reported_at,omitempty"`
```

- [ ] **Step 4: 写 `RecordFirmwareVersion` + `FirmwareSummary` + 汇总类型**

在 `api/internal/modules/gateway/service.go` 中(紧接 `UpdateGatewayStatus` 函数之后)新增:

```go
// RecordFirmwareVersion stores the running firmware version a gateway reported.
// An empty version is ignored so an older agent that doesn't report can't clobber it.
func (s *Service) RecordFirmwareVersion(tenantID, gatewayID, version string) error {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return ErrGatewayIDRequired
	}
	nextVersion := strings.TrimSpace(version)
	if nextVersion == "" {
		return nil
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.gateways {
		if s.gateways[i].ID != nextGatewayID {
			continue
		}
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			return ErrGatewayNotFound
		}
		s.gateways[i].CurrentFirmwareVersion = nextVersion
		s.gateways[i].FirmwareReportedAt = time.Now().UTC()
		return s.persistLocked()
	}
	return ErrGatewayNotFound
}

// GatewayFirmwareVersionCount is one firmware version and how many gateways run it.
type GatewayFirmwareVersionCount struct {
	Version string `json:"version"`
	Count   int    `json:"count"`
}

// GatewayFirmwareSummary is the firmware-version distribution across a tenant's gateways.
type GatewayFirmwareSummary struct {
	Total    int                           `json:"total"`
	Reported int                           `json:"reported"`
	Versions []GatewayFirmwareVersionCount `json:"versions"`
}

// FirmwareSummary returns the firmware-version distribution for a tenant's gateways,
// sorted by count desc (version asc as a stable tiebreak).
func (s *Service) FirmwareSummary(tenantID string) GatewayFirmwareSummary {
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := map[string]int{}
	summary := GatewayFirmwareSummary{}
	for i := range s.gateways {
		if filterTenantID != "" && s.gateways[i].TenantID != filterTenantID {
			continue
		}
		summary.Total++
		v := strings.TrimSpace(s.gateways[i].CurrentFirmwareVersion)
		if v == "" {
			continue
		}
		summary.Reported++
		counts[v]++
	}
	for v, c := range counts {
		summary.Versions = append(summary.Versions, GatewayFirmwareVersionCount{Version: v, Count: c})
	}
	sort.Slice(summary.Versions, func(a, b int) bool {
		if summary.Versions[a].Count != summary.Versions[b].Count {
			return summary.Versions[a].Count > summary.Versions[b].Count
		}
		return summary.Versions[a].Version < summary.Versions[b].Version
	})
	return summary
}
```

确保 `api/internal/modules/gateway/service.go` 的 import 块包含 `"sort"`(若缺则加入;`strings`/`time` 已有)。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd api && go test ./internal/modules/gateway/ -run 'TestRecordFirmwareVersion|TestFirmwareSummary' -v`
Expected: PASS。

- [ ] **Step 6: 全包回归 + vet + gofmt**

Run: `cd api && go test ./internal/modules/gateway/ && go vet ./internal/modules/gateway/ && gofmt -l internal/modules/gateway/service.go`
Expected: 测试 ok;vet 无输出;gofmt 无输出。

- [ ] **Step 7: 提交**

```bash
git add api/internal/modules/gateway/service.go api/internal/modules/gateway/service_test.go
git commit -m "feat: track per-gateway firmware version + fleet summary"
```

---

## Task 2: config/pull 捕获固件版本 + agent 上报

**Files:**
- Modify: `api/internal/http/router_handlers_gateway.go`
- Modify: `api/cmd/gateway-agent/agent.go`
- Test: `api/internal/http/routes_gateway_bootstrap_test.go`

- [ ] **Step 1: 写失败测试(config/pull 捕获)**

在 `api/internal/http/routes_gateway_bootstrap_test.go` 末尾追加(import 若缺 `bytes`/`encoding/json`/`net/http`/`net/http/httptest`/`testing` 与 `audit`/`gateway` 模块则补上 —— 参考本文件或 `routes_gateway_ota_test.go` 已有 import):

```go
func TestGatewayConfigPullRecordsFirmwareVersion(t *testing.T) {
	svc := gateway.NewService()
	s := &server{
		gatewaySvc:          svc,
		auditSvc:            audit.NewService(),
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "gw_test_token_001"},
	}
	body, _ := json.Marshal(map[string]any{
		"gateway_id":       "gw_demo_001",
		"tenant_id":        "tenant_demo_jakarta",
		"firmware_version": "1.4.0",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/config/pull", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer gw_test_token_001")
	rec := httptest.NewRecorder()
	s.gatewayBootstrapConfigPull(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config/pull expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	recorded := false
	for _, g := range svc.List("tenant_demo_jakarta") {
		if g.ID == "gw_demo_001" && g.CurrentFirmwareVersion == "1.4.0" {
			recorded = true
		}
	}
	if !recorded {
		t.Fatal("firmware version not recorded from config/pull")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/http/ -run TestGatewayConfigPullRecordsFirmwareVersion -v`
Expected: FAIL(固件版本未被记录 → `recorded` 为 false)。

- [ ] **Step 3: handler 捕获 firmware_version**

在 `api/internal/http/router_handlers_gateway.go` 的 `gatewayBootstrapConfigPull` 中,给请求体 struct 加一个字段:

```go
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		CurrentVersion string `json:"current_version"`
		AuthzVersion   string `json:"authz_cache_version"`
		FirmwareVersion string `json:"firmware_version"`
	}
```

在设备鉴权通过之后(`if !s.authorizeGatewayHTTPDeviceRequest(w, r, record.ID) { return }` 这段之后)插入:

```go
	_ = s.gatewaySvc.RecordFirmwareVersion(request.TenantID, request.GatewayID, request.FirmwareVersion)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/http/ -run TestGatewayConfigPullRecordsFirmwareVersion -v`
Expected: PASS。

- [ ] **Step 5: agent 在 pullConfig 上报固件版本**

在 `api/cmd/gateway-agent/agent.go` 的 `pullConfig` 中,把请求体 map 改为包含 `firmware_version`:

```go
	body, _ := json.Marshal(map[string]string{
		"gateway_id":          a.gatewayID,
		"tenant_id":           a.tenantID,
		"current_version":     "",
		"authz_cache_version": a.ruleVersion,
		"firmware_version":    a.agentVersion,
	})
```

- [ ] **Step 6: 构建 + 回归 + 交叉编译**

Run: `cd api && go build ./... && go test ./internal/http/ -run 'OTA|ConfigPull|Firmware' && GOOS=linux GOARCH=arm64 go build -o /tmp/t2 ./cmd/gateway-agent && echo CROSS_OK && rm -f /tmp/t2`
Expected: build OK;相关测试 PASS;打印 `CROSS_OK`。

- [ ] **Step 7: 提交**

```bash
git add api/internal/http/router_handlers_gateway.go api/internal/http/routes_gateway_bootstrap_test.go api/cmd/gateway-agent/agent.go
git commit -m "feat: agent reports firmware version, captured on config pull"
```

---

## Task 3: firmware-summary 端点 + 路由

**Files:**
- Modify: `api/internal/http/routes_gateway_management.go`
- Modify: `api/internal/http/router.go`
- Test: `api/internal/http/routes_gateway_ota_test.go`

- [ ] **Step 1: 写失败测试(summary 端点)**

在 `api/internal/http/routes_gateway_ota_test.go` 末尾追加(该文件已 import `auth`/`gateway`/`json`/`httptest` 等):

```go
func TestGatewayFirmwareSummaryEndpoint(t *testing.T) {
	svc := gateway.NewService()
	if err := svc.RecordFirmwareVersion("tenant_demo_jakarta", "gw_demo_001", "1.4.0"); err != nil {
		t.Fatal(err)
	}
	s := &server{gatewaySvc: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/firmware-summary?tenant_id=tenant_demo_jakarta", nil)
	req = withGatewayMQTTUser(req, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
	rec := httptest.NewRecorder()
	s.gatewayFirmwareSummary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var sum gateway.GatewayFirmwareSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sum.Reported < 1 {
		t.Fatalf("expected reported>=1, got %+v", sum)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/http/ -run TestGatewayFirmwareSummaryEndpoint -v`
Expected: FAIL(`s.gatewayFirmwareSummary undefined`)。

- [ ] **Step 3: 写 handler**

在 `api/internal/http/routes_gateway_management.go` 中(紧接 `listGateways` 函数之后)新增:

```go
func (s *server) gatewayFirmwareSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.gatewaySvc.FirmwareSummary(tenantID))
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/http/ -run TestGatewayFirmwareSummaryEndpoint -v`
Expected: PASS。

- [ ] **Step 5: 注册路由**

在 `api/internal/http/router.go` 中,紧接这一行:

```go
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/{gatewayID}/ota/tasks", s.listGatewayOTATasks)
```

之后加入(静态段 `firmware-summary` 在 chi 中优先于 `{gatewayID}` 参数段,不会被误捕获):

```go
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/firmware-summary", s.gatewayFirmwareSummary)
```

- [ ] **Step 6: 全量构建 + 相关测试 + vet/gofmt**

Run: `cd api && go build ./... && go test ./internal/http/ -run 'Firmware|OTA|ConfigPull' && go vet ./internal/http/ && gofmt -l internal/http/routes_gateway_management.go internal/http/router.go`
Expected: build OK;测试 PASS;vet/gofmt 无输出。

- [ ] **Step 7: 端到端冒烟(可选,确认路由真的挂上)**

Run: `cd api && go test ./internal/http/ 2>&1 | tail -3`
Expected: `ok ... internal/http`(整包绿,确认路由注册没破坏其它路由)。

- [ ] **Step 8: 提交**

```bash
git add api/internal/http/routes_gateway_management.go api/internal/http/router.go api/internal/http/routes_gateway_ota_test.go
git commit -m "feat: add gateways/firmware-summary fleet endpoint"
```

---

## 自检(Self-Review)

**1. Spec 覆盖**
- §4.1 数据模型两字段 → Task 1 Step 3 ✓
- §4.2 agent 上报 → Task 2 Step 5 ✓
- §4.3 服务端捕获 → Task 2 Step 3 ✓
- §4.4 RecordFirmwareVersion + FirmwareSummary + 汇总类型 → Task 1 Step 4 ✓
- §4.5 list 自动带出字段 → Task 1(字段在 `Gateway` 上,`List` 返回 `[]Gateway`,`listGateways` 直接序列化)✓;firmware-summary 端点 → Task 3 ✓;`latest_ota_status` → **有意省略**(已由现有 `GET /gateways/{gatewayID}/ota/tasks` 提供,DRY/YAGNI,见顶部范围说明)。
- §6 错误/边界(空值 no-op、租户隔离)→ Task 1 测试覆盖 ✓
- §7 测试 → Task 1/2/3 各自单测 ✓

**2. 占位符扫描**:无 TODO/TBD;每个代码步骤含完整代码 + 确切命令/预期。

**3. 类型一致性**:`CurrentFirmwareVersion`/`FirmwareReportedAt` 字段名在 struct、Record、Summary、测试中一致;`GatewayFirmwareSummary{Total,Reported,Versions}` 与 `GatewayFirmwareVersionCount{Version,Count}` 在 service 与 http 测试中一致;`RecordFirmwareVersion(tenantID, gatewayID, version) error` 在 service、handler、测试三处签名一致。

---

## 执行交接

计划已就绪。两种执行方式:
1. **Subagent-Driven(推荐)** — 每个 Task 派发新 subagent,任务间两阶段审查。REQUIRED SUB-SKILL: superpowers:subagent-driven-development。
2. **Inline Execution** — 本会话内按 executing-plans 批量执行 + 检查点。REQUIRED SUB-SKILL: superpowers:executing-plans。
