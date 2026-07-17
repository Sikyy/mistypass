# OTA 舰队批量 + 灰度发布 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 一个 rollout 编排器:选固件版本 + 一组网关 + 分阶段计划,平台自动分批建 OTA 任务、按成功率自动推进/暂停、阶段可标记需人工审批(对标 Kisi OTA 子项 #3,纯后端,UI 归 #5)。

**Architecture:** Rollout 状态机在 gateway service 的新 `rollouts` 切片(随 stateSnapshot 持久化)。cohort 从 Target 解析网关集合、按累进百分比切片。**反应式推进**:OTA 报告(`UpdateOTATaskStatus`)+ 管理端 GET 时评估当前阶段(终态/失败率/stall 超时)→ 转移状态 / 建下一 cohort。复用 #2 的 `firmware_id`(rollout 任务带 firmware_id,config/pull 自动签下载 URL)。不新增后台 worker。

**Tech Stack:** Go、chi、in-memory service + stateSnapshot 持久化。

设计依据:[2026-06-07-ota-fleet-rollout-design.md](../specs/2026-06-07-ota-fleet-rollout-design.md)

**约定:** `go` 在 `api/` 下;`gateway.NewService()` 预置 `gw_demo_001`(tenant `tenant_demo_jakarta`,building `building_demo_001`)。OTA 状态常量 `gatewayOTATaskStatusQueued/Dispatching/Succeeded/Failed`(实现者用前先 `grep "gatewayOTATaskStatus" service.go` 核实名字)。固件相关复用 #2:`findFirmwareLocked(id, tenant) (GatewayFirmware, bool)`、`GatewayFirmware.Version`。`isValidSHA256Hex` 等已存在。

---

## Task 1: Rollout 类型 + cohort 计算 + 校验(纯逻辑)

**Files:**
- Create: `api/internal/modules/gateway/rollout.go`
- Test: `api/internal/modules/gateway/rollout_test.go`

- [ ] **Step 1: 写失败测试**

Create `api/internal/modules/gateway/rollout_test.go`:

```go
package gateway

import "testing"

func TestValidateRolloutPhases(t *testing.T) {
	ok := []RolloutPhase{{Percentage: 10}, {Percentage: 50}, {Percentage: 100}}
	if !validateRolloutPhases(ok) {
		t.Fatal("expected valid phases")
	}
	bad := [][]RolloutPhase{
		{},
		{{Percentage: 10}, {Percentage: 50}},          // last != 100
		{{Percentage: 50}, {Percentage: 50}, {Percentage: 100}}, // not strictly increasing
		{{Percentage: 0}, {Percentage: 100}},          // 0 out of range
		{{Percentage: 10}, {Percentage: 101}},         // >100
	}
	for i, p := range bad {
		if validateRolloutPhases(p) {
			t.Fatalf("case %d should be invalid", i)
		}
	}
}

func TestCohortForPhase(t *testing.T) {
	mk := func(ids ...string) []Gateway {
		out := make([]Gateway, 0, len(ids))
		for _, id := range ids {
			out = append(out, Gateway{ID: id})
		}
		return out
	}
	all := mk("a", "b", "c", "d") // N=4
	phases := []RolloutPhase{{Percentage: 25}, {Percentage: 50}, {Percentage: 100}}
	// cum: ceil(25%*4)=1, ceil(50%*4)=2, ceil(100%*4)=4
	p0 := cohortForPhase(all, phases, 0)
	if len(p0) != 1 || p0[0].ID != "a" {
		t.Fatalf("phase0 want [a], got %+v", p0)
	}
	p1 := cohortForPhase(all, phases, 1)
	if len(p1) != 1 || p1[0].ID != "b" {
		t.Fatalf("phase1 want [b], got %+v", p1)
	}
	p2 := cohortForPhase(all, phases, 2)
	if len(p2) != 2 || p2[0].ID != "c" || p2[1].ID != "d" {
		t.Fatalf("phase2 want [c d], got %+v", p2)
	}
}

func TestRolloutTargetGateways(t *testing.T) {
	svc := NewService()
	// gw_demo_001 is seeded under tenant_demo_jakarta / building_demo_001.
	allT := svc.rolloutTargetGatewaysLocked("tenant_demo_jakarta", RolloutTarget{Kind: "all"})
	if len(allT) == 0 {
		t.Fatal("expected at least the seeded gateway for 'all'")
	}
	bld := svc.rolloutTargetGatewaysLocked("tenant_demo_jakarta", RolloutTarget{Kind: "building", BuildingID: "building_demo_001"})
	if len(bld) == 0 {
		t.Fatal("expected seeded gateway in building_demo_001")
	}
	none := svc.rolloutTargetGatewaysLocked("tenant_demo_jakarta", RolloutTarget{Kind: "building", BuildingID: "nope"})
	if len(none) != 0 {
		t.Fatalf("expected 0 for unknown building, got %d", len(none))
	}
	exp := svc.rolloutTargetGatewaysLocked("tenant_demo_jakarta", RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}})
	if len(exp) != 1 || exp[0].ID != "gw_demo_001" {
		t.Fatalf("explicit target want [gw_demo_001], got %+v", exp)
	}
	cross := svc.rolloutTargetGatewaysLocked("tenant_other", RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}})
	if len(cross) != 0 {
		t.Fatal("cross-tenant explicit target must resolve empty")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Rollout|Cohort' -v`
Expected: FAIL(`undefined: RolloutPhase`/`cohortForPhase` 等)。

- [ ] **Step 3: 写类型 + 纯逻辑(rollout.go)**

Create `api/internal/modules/gateway/rollout.go`:

```go
package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrRolloutNotFound         = errors.New("gateway rollout not found")
	ErrRolloutFirmwareRequired = errors.New("rollout firmware_id is required")
	ErrRolloutTargetEmpty      = errors.New("rollout target resolves to no gateways")
	ErrRolloutPhasesInvalid    = errors.New("rollout phases must be non-empty, strictly increasing percentages ending at 100 (each 1-100)")
	ErrRolloutThresholdInvalid = errors.New("rollout failure_threshold_pct must be 0-100")
	ErrRolloutStateConflict    = errors.New("rollout action not allowed in current state")
)

const (
	rolloutStatePending               = "pending"
	rolloutStateActive                = "active"
	rolloutStateAwaitingApproval      = "awaiting_approval"
	rolloutStatePaused                = "paused"
	rolloutStateCompleted             = "completed"
	rolloutStateFailed                = "failed"
	defaultRolloutFailureThresholdPct = 20
	rolloutStallWindow                = time.Hour
)

// RolloutTarget selects which gateways a rollout covers.
type RolloutTarget struct {
	Kind       string   `json:"kind"` // "all" | "building" | "gateways"
	BuildingID string   `json:"building_id,omitempty"`
	GatewayIDs []string `json:"gateway_ids,omitempty"`
}

// RolloutPhase is one cumulative-coverage wave.
type RolloutPhase struct {
	Percentage       int  `json:"percentage"`        // cumulative 1-100, strictly increasing, last == 100
	RequiresApproval bool `json:"requires_approval"` // pause for manual approval before entering this phase
}

// GatewayRollout is a phased firmware rollout over a set of gateways.
type GatewayRollout struct {
	ID                  string         `json:"id"`
	TenantID            string         `json:"tenant_id"`
	FirmwareID          string         `json:"firmware_id"`
	FirmwareVersion     string         `json:"firmware_version"`
	Target              RolloutTarget  `json:"target"`
	Phases              []RolloutPhase `json:"phases"`
	FailureThresholdPct int            `json:"failure_threshold_pct"`
	State               string         `json:"state"`
	CurrentPhase        int            `json:"current_phase"`
	PhaseStartedAt      time.Time      `json:"phase_started_at"`
	CreatedBy           string         `json:"created_by,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

func rolloutRecordID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "rollout_" + hex.EncodeToString(b), nil
}

// validateRolloutPhases requires non-empty, each in 1..100, strictly increasing, last == 100.
func validateRolloutPhases(phases []RolloutPhase) bool {
	if len(phases) == 0 {
		return false
	}
	prev := 0
	for _, p := range phases {
		if p.Percentage < 1 || p.Percentage > 100 || p.Percentage <= prev {
			return false
		}
		prev = p.Percentage
	}
	return prev == 100
}

// cohortForPhase returns the gateways newly covered by phaseIdx (cumulative slicing).
func cohortForPhase(all []Gateway, phases []RolloutPhase, phaseIdx int) []Gateway {
	n := len(all)
	cum := func(i int) int {
		if i < 0 {
			return 0
		}
		c := (phases[i].Percentage*n + 99) / 100 // ceil(pct*n/100)
		if c > n {
			c = n
		}
		return c
	}
	start, end := cum(phaseIdx-1), cum(phaseIdx)
	if start > end {
		start = end
	}
	return all[start:end]
}

// rolloutTargetGatewaysLocked resolves a target to a tenant-scoped, ID-sorted gateway set.
// Caller holds s.mu.
func (s *Service) rolloutTargetGatewaysLocked(tenantID string, target RolloutTarget) []Gateway {
	wantIDs := map[string]struct{}{}
	for _, id := range target.GatewayIDs {
		wantIDs[id] = struct{}{}
	}
	var out []Gateway
	for i := range s.gateways {
		if s.gateways[i].TenantID != tenantID {
			continue
		}
		switch target.Kind {
		case "all":
			out = append(out, s.gateways[i])
		case "building":
			if s.gateways[i].BuildingID == target.BuildingID {
				out = append(out, s.gateways[i])
			}
		case "gateways":
			if _, ok := wantIDs[s.gateways[i].ID]; ok {
				out = append(out, s.gateways[i])
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

func cloneGatewayRollouts(in []GatewayRollout) []GatewayRollout {
	if len(in) == 0 {
		return nil
	}
	out := make([]GatewayRollout, len(in))
	copy(out, in)
	return out
}
```

- [ ] **Step 4: 运行测试确认通过 + 回归**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Rollout|Cohort' -v && go test ./internal/modules/gateway/ && go vet ./internal/modules/gateway/ && gofmt -l internal/modules/gateway/rollout.go`
Expected: PASS;全包 ok;vet/gofmt 无输出。

- [ ] **Step 5: 提交**

```bash
git add api/internal/modules/gateway/rollout.go api/internal/modules/gateway/rollout_test.go
git commit -m "feat: add gateway rollout types, cohort math, target resolution"
```

---

## Task 2: 持久化 + OTA 归属 + CreateRollout(建 + start phase 0)

**Files:**
- Modify: `api/internal/modules/gateway/service.go`(`Service`/`stateSnapshot` 加 `rollouts`;`GatewayOTATask` 加 `RolloutID`/`RolloutPhase`;persist/restore 接线)
- Modify: `api/internal/modules/gateway/rollout.go`(CreateRollout/Get/List + helpers)
- Test: `api/internal/modules/gateway/rollout_test.go`

- [ ] **Step 1: 写失败测试**

在 `rollout_test.go` 末尾追加(`strings` 若未 import 则补):

```go
func seedFirmware(t *testing.T, svc *Service) GatewayFirmware {
	t.Helper()
	fw, err := svc.CreateFirmware(CreateFirmwareInput{
		TenantID:  "tenant_demo_jakarta",
		Version:   "1.4.0",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Signature: strings.Repeat("b", 128),
	})
	if err != nil {
		t.Fatalf("seed firmware: %v", err)
	}
	return fw
}

func TestCreateRolloutStartsPhaseZero(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	r, err := svc.CreateRollout(CreateRolloutInput{
		TenantID:   "tenant_demo_jakarta",
		FirmwareID: fw.ID,
		Target:     RolloutTarget{Kind: "all"},
		Phases:     []RolloutPhase{{Percentage: 100}},
		CreatedBy:  "admin@example.com",
	})
	if err != nil {
		t.Fatalf("create rollout: %v", err)
	}
	if r.State != rolloutStateActive || r.CurrentPhase != 0 || r.FirmwareVersion != "1.4.0" {
		t.Fatalf("unexpected rollout: %+v", r)
	}
	if r.FailureThresholdPct != defaultRolloutFailureThresholdPct {
		t.Fatalf("threshold default want 20, got %d", r.FailureThresholdPct)
	}
	// phase-0 cohort (100% of all) should have OTA tasks tagged with the rollout.
	tasks, _ := svc.ListOTATasks("tenant_demo_jakarta", "gw_demo_001")
	tagged := false
	for _, task := range tasks {
		if task.RolloutID == r.ID && task.RolloutPhase == 0 && task.FirmwareID == fw.ID {
			tagged = true
		}
	}
	if !tagged {
		t.Fatal("expected a phase-0 OTA task tagged with the rollout id")
	}
	got, err := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if err != nil || got.ID != r.ID {
		t.Fatalf("get rollout: %v %+v", err, got)
	}
	if _, err := svc.GetRollout("tenant_other", r.ID); err != ErrRolloutNotFound {
		t.Fatalf("cross-tenant get should fail, got %v", err)
	}
}

func TestCreateRolloutValidation(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	base := CreateRolloutInput{TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID, Target: RolloutTarget{Kind: "all"}, Phases: []RolloutPhase{{Percentage: 100}}}
	mut := func(f func(*CreateRolloutInput)) CreateRolloutInput { in := base; f(&in); return in }

	if _, err := svc.CreateRollout(mut(func(in *CreateRolloutInput) { in.FirmwareID = "" })); err != ErrRolloutFirmwareRequired {
		t.Fatalf("want firmware-required, got %v", err)
	}
	if _, err := svc.CreateRollout(mut(func(in *CreateRolloutInput) { in.FirmwareID = "fw_nope" })); err != ErrGatewayFirmwareNotFound {
		t.Fatalf("want firmware-not-found, got %v", err)
	}
	if _, err := svc.CreateRollout(mut(func(in *CreateRolloutInput) { in.Phases = []RolloutPhase{{Percentage: 50}} })); err != ErrRolloutPhasesInvalid {
		t.Fatalf("want phases-invalid, got %v", err)
	}
	if _, err := svc.CreateRollout(mut(func(in *CreateRolloutInput) { in.Target = RolloutTarget{Kind: "building", BuildingID: "nope"} })); err != ErrRolloutTargetEmpty {
		t.Fatalf("want target-empty, got %v", err)
	}
	if _, err := svc.CreateRollout(mut(func(in *CreateRolloutInput) { in.FailureThresholdPct = 150 })); err != ErrRolloutThresholdInvalid {
		t.Fatalf("want threshold-invalid, got %v", err)
	}
}

func TestRolloutPersistsToStateStore(t *testing.T) {
	store := &gatewayMemoryStateStore{}
	first, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("new first: %v", err)
	}
	fw := seedFirmware(t, first)
	r, err := first.CreateRollout(CreateRolloutInput{TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID, Target: RolloutTarget{Kind: "all"}, Phases: []RolloutPhase{{Percentage: 100}}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	restored, err := NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("new restored: %v", err)
	}
	got, err := restored.GetRollout("tenant_demo_jakarta", r.ID)
	if err != nil || got.FirmwareVersion != "1.4.0" {
		t.Fatalf("rollout not restored: %v %+v", err, got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'CreateRollout|RolloutPersists' -v`
Expected: FAIL(`undefined: CreateRolloutInput`/`CreateRollout` 等)。

- [ ] **Step 3: GatewayOTATask 加 rollout 字段 + Service/snapshot 接线(service.go)**

(a) `GatewayOTATask` struct(`FirmwareID` 字段那行之后)加:
```go
	RolloutID         string    `json:"rollout_id,omitempty"`
	RolloutPhase      int       `json:"rollout_phase,omitempty"`
```
(b) `stateSnapshot` struct(`Firmwares` 那行之后)加:
```go
	Rollouts               []GatewayRollout              `json:"rollouts,omitempty"`
```
(c) `Service` struct(`firmwares` 那行之后)加:
```go
	rollouts               []GatewayRollout
```
(d) `NewService()` 结构体字面量(`firmwares: []GatewayFirmware{}` 那行之后)加:
```go
		rollouts:               []GatewayRollout{},
```
(e) `restoreFromStateStore` 中(`s.firmwares = cloneGatewayFirmwares(snapshot.Firmwares)` 那行之后)加:
```go
	s.rollouts = cloneGatewayRollouts(snapshot.Rollouts)
```
(f) 每个构建 `stateSnapshot{...}` 的字面量(restore 里两处 + `persistLocked` 一处,`Firmwares:` 那行之后)各加:
```go
		Rollouts:               cloneGatewayRollouts(s.rollouts),
```

- [ ] **Step 4: CreateRollout / Get / List + helpers(rollout.go)**

在 `rollout.go` 末尾追加:

```go
// CreateRolloutInput carries a new rollout request.
type CreateRolloutInput struct {
	TenantID            string
	FirmwareID          string
	Target              RolloutTarget
	Phases              []RolloutPhase
	FailureThresholdPct int
	CreatedBy           string
}

// CreateRollout validates, persists, and immediately starts phase 0.
func (s *Service) CreateRollout(in CreateRolloutInput) (GatewayRollout, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	fwID := strings.TrimSpace(in.FirmwareID)
	if fwID == "" {
		return GatewayRollout{}, ErrRolloutFirmwareRequired
	}
	if !validateRolloutPhases(in.Phases) {
		return GatewayRollout{}, ErrRolloutPhasesInvalid
	}
	threshold := in.FailureThresholdPct
	if threshold == 0 {
		threshold = defaultRolloutFailureThresholdPct
	}
	if threshold < 0 || threshold > 100 {
		return GatewayRollout{}, ErrRolloutThresholdInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fw, ok := s.findFirmwareLocked(fwID, tenantID)
	if !ok {
		return GatewayRollout{}, ErrGatewayFirmwareNotFound
	}
	all := s.rolloutTargetGatewaysLocked(tenantID, in.Target)
	if len(all) == 0 {
		return GatewayRollout{}, ErrRolloutTargetEmpty
	}

	id, err := rolloutRecordID()
	if err != nil {
		return GatewayRollout{}, err
	}
	now := time.Now().UTC()
	r := GatewayRollout{
		ID:                  id,
		TenantID:            tenantID,
		FirmwareID:          fwID,
		FirmwareVersion:     fw.Version,
		Target:              in.Target,
		Phases:              in.Phases,
		FailureThresholdPct: threshold,
		State:               rolloutStatePending,
		CurrentPhase:        0,
		CreatedBy:           strings.TrimSpace(in.CreatedBy),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	s.rollouts = append([]GatewayRollout{r}, s.rollouts...)
	s.startRolloutPhaseLocked(&s.rollouts[0], 0, all, now)
	if err := s.persistLocked(); err != nil {
		return GatewayRollout{}, err
	}
	return s.rollouts[0], nil
}

// startRolloutPhaseLocked creates OTA tasks for the phase cohort and marks the rollout active.
// Caller holds s.mu.
func (s *Service) startRolloutPhaseLocked(r *GatewayRollout, phase int, all []Gateway, now time.Time) {
	fw, ok := s.findFirmwareLocked(r.FirmwareID, r.TenantID)
	if !ok {
		r.State = rolloutStateFailed
		r.UpdatedAt = now
		return
	}
	for _, gw := range cohortForPhase(all, r.Phases, phase) {
		s.appendRolloutOTATaskLocked(gw, fw, r.ID, phase, now)
	}
	r.CurrentPhase = phase
	r.PhaseStartedAt = now
	r.State = rolloutStateActive
	r.UpdatedAt = now
}

// appendRolloutOTATaskLocked builds a queued OTA task attributed to a rollout phase.
// Firmware sha/sig come from the (already-validated) registry record. Caller holds s.mu.
func (s *Service) appendRolloutOTATaskLocked(gw Gateway, fw GatewayFirmware, rolloutID string, phase int, now time.Time) {
	taskID, err := otaTaskID()
	if err != nil {
		return
	}
	s.otaTasks = append([]GatewayOTATask{{
		ID:                taskID,
		GatewayID:         gw.ID,
		TenantID:          gw.TenantID,
		FirmwareVersion:   fw.Version,
		FirmwareSHA256:    fw.SHA256,
		FirmwareSignature: fw.Signature,
		FirmwareID:        fw.ID,
		Status:            gatewayOTATaskStatusQueued,
		RequestedBy:       "rollout:" + rolloutID,
		UpdatedBy:         "rollout:" + rolloutID,
		RolloutID:         rolloutID,
		RolloutPhase:      phase,
		CreatedAt:         now,
		UpdatedAt:         now,
	}}, s.otaTasks...)
}

// GetRollout returns a tenant-scoped rollout.
func (s *Service) GetRollout(tenantID, id string) (GatewayRollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.findRolloutIndexLocked(strings.TrimSpace(id), strings.TrimSpace(tenantID)); i >= 0 {
		return s.rollouts[i], nil
	}
	return GatewayRollout{}, ErrRolloutNotFound
}

// ListRollouts returns a tenant's rollouts, newest first.
func (s *Service) ListRollouts(tenantID string) []GatewayRollout {
	ft := strings.TrimSpace(tenantID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]GatewayRollout, 0, len(s.rollouts))
	for i := range s.rollouts {
		if ft == "" || s.rollouts[i].TenantID == ft {
			out = append(out, s.rollouts[i])
		}
	}
	return out
}

// findRolloutIndexLocked returns the index of a rollout (optionally tenant-filtered), or -1.
func (s *Service) findRolloutIndexLocked(id, tenantID string) int {
	for i := range s.rollouts {
		if s.rollouts[i].ID == id && (tenantID == "" || s.rollouts[i].TenantID == tenantID) {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 5: 运行测试 + 回归**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Rollout' -v && go test ./internal/modules/gateway/ && go build ./... && go vet ./internal/modules/gateway/ && gofmt -l internal/modules/gateway/rollout.go internal/modules/gateway/service.go`
Expected: 全 PASS;build OK;vet/gofmt clean。(若 `otaTaskID`/`gatewayOTATaskStatusQueued` 名字不符,grep service.go 核实并改。)

- [ ] **Step 6: 提交**

```bash
git add api/internal/modules/gateway/rollout.go api/internal/modules/gateway/service.go api/internal/modules/gateway/rollout_test.go
git commit -m "feat: rollout store + persistence + phase-0 start + OTA attribution"
```

---

## Task 3: 推进引擎 + 反应式挂钩

**Files:**
- Modify: `api/internal/modules/gateway/rollout.go`(`evaluateRolloutPhaseLocked` + `advanceRolloutLocked` + `latestRolloutTaskLocked`;`GetRollout` 触发评估)
- Modify: `api/internal/modules/gateway/service.go`(`UpdateOTATaskStatus` 末尾触发推进)
- Test: `api/internal/modules/gateway/rollout_test.go`

- [ ] **Step 1: 写失败测试**

在 `rollout_test.go` 末尾追加:

```go
func reportTask(t *testing.T, svc *Service, gatewayID, status string) {
	t.Helper()
	tasks, _ := svc.ListOTATasks("tenant_demo_jakarta", gatewayID)
	if len(tasks) == 0 {
		t.Fatalf("no tasks for %s", gatewayID)
	}
	if _, err := svc.UpdateOTATaskStatus("tenant_demo_jakarta", gatewayID, tasks[0].ID, status, "", "agent"); err != nil {
		t.Fatalf("report %s=%s: %v", gatewayID, status, err)
	}
}

func TestRolloutAutoAdvanceAndComplete(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	// single-gateway fleet → two phases both cover the one gateway cumulatively:
	// use one phase at 100 so cohort0 = the gateway; report success → completed.
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
	})
	reportTask(t, svc, "gw_demo_001", "succeeded")
	got, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if got.State != rolloutStateCompleted {
		t.Fatalf("want completed, got %s", got.State)
	}
}

func TestRolloutFailureGatePauses(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}}, FailureThresholdPct: 20,
	})
	reportTask(t, svc, "gw_demo_001", "failed") // 100% failure ≥ 20% → paused
	got, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if got.State != rolloutStatePaused {
		t.Fatalf("want paused, got %s", got.State)
	}
}

func TestRolloutStallTimeoutCountsAsFailure(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
	})
	// Force the phase start into the past so the stall window has elapsed.
	idx := svc.findRolloutIndexLocked(r.ID, "tenant_demo_jakarta")
	svc.rollouts[idx].PhaseStartedAt = time.Now().UTC().Add(-2 * time.Hour)
	// GET triggers evaluation; no report → stall → counted failed → paused (100% fail).
	got, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if got.State != rolloutStatePaused {
		t.Fatalf("want paused via stall, got %s", got.State)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'AutoAdvance|FailureGate|Stall' -v`
Expected: FAIL(GetRollout 不评估;状态停在 active)。

- [ ] **Step 3: 推进引擎(rollout.go)**

在 `rollout.go` 末尾追加:

```go
// latestRolloutTaskLocked finds the newest OTA task for a rollout phase + gateway. Caller holds s.mu.
func (s *Service) latestRolloutTaskLocked(rolloutID string, phase int, gatewayID string) (GatewayOTATask, bool) {
	for i := range s.otaTasks {
		if s.otaTasks[i].RolloutID == rolloutID && s.otaTasks[i].RolloutPhase == phase && s.otaTasks[i].GatewayID == gatewayID {
			return s.otaTasks[i], true
		}
	}
	return GatewayOTATask{}, false
}

// evaluateRolloutPhaseLocked reports whether the current phase is terminal and its failure rate (%).
// A cohort gateway with no terminal task counts as failed once the stall window elapses.
// Empty cohort → terminal with 0% failure. Caller holds s.mu.
func (s *Service) evaluateRolloutPhaseLocked(r *GatewayRollout, all []Gateway, now time.Time) (bool, int) {
	cohort := cohortForPhase(all, r.Phases, r.CurrentPhase)
	total := len(cohort)
	if total == 0 {
		return true, 0
	}
	stalled := now.Sub(r.PhaseStartedAt) > rolloutStallWindow
	failed, terminal := 0, 0
	for _, gw := range cohort {
		task, ok := s.latestRolloutTaskLocked(r.ID, r.CurrentPhase, gw.ID)
		switch {
		case ok && task.Status == gatewayOTATaskStatusSucceeded:
			terminal++
		case ok && task.Status == gatewayOTATaskStatusFailed:
			terminal++
			failed++
		case stalled:
			terminal++
			failed++
		}
	}
	if terminal < total {
		return false, 0
	}
	return true, failed * 100 / total
}

// advanceRolloutLocked drives an active rollout forward as far as it can. Caller holds s.mu.
func (s *Service) advanceRolloutLocked(rolloutIdx int) {
	r := &s.rollouts[rolloutIdx]
	now := time.Now().UTC()
	for r.State == rolloutStateActive {
		all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target)
		terminal, failureRate := s.evaluateRolloutPhaseLocked(r, all, now)
		if !terminal {
			return
		}
		r.UpdatedAt = now
		if failureRate >= r.FailureThresholdPct {
			r.State = rolloutStatePaused
			return
		}
		if r.CurrentPhase >= len(r.Phases)-1 {
			r.State = rolloutStateCompleted
			return
		}
		next := r.CurrentPhase + 1
		if r.Phases[next].RequiresApproval {
			r.State = rolloutStateAwaitingApproval
			return
		}
		s.startRolloutPhaseLocked(r, next, all, now) // creates next cohort; loop re-evaluates (empty cohort auto-skips)
	}
}
```

并把 `GetRollout` 改为**评估后再返回**(替换 Task 2 的 GetRollout 实现):
```go
// GetRollout evaluates the rollout (catches stall timeouts) then returns it, tenant-scoped.
func (s *Service) GetRollout(tenantID, id string) (GatewayRollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.findRolloutIndexLocked(strings.TrimSpace(id), strings.TrimSpace(tenantID))
	if i < 0 {
		return GatewayRollout{}, ErrRolloutNotFound
	}
	s.advanceRolloutLocked(i)
	if err := s.persistLocked(); err != nil {
		return GatewayRollout{}, err
	}
	return s.rollouts[i], nil
}
```

- [ ] **Step 4: 反应式挂钩(service.go `UpdateOTATaskStatus`)**

在 `UpdateOTATaskStatus` 里,把设置状态 + 返回那段(`s.otaTasks[idx].UpdatedAt = now` 之后、`persistLocked` 之前)改为:**先捕获 task(advance 可能 prepend 导致 idx 位移),再触发 rollout 推进**:
```go
	s.otaTasks[idx].Status = nextStatus
	s.otaTasks[idx].ErrorMessage = nextErrorMessage
	if nextUpdatedBy != "" {
		s.otaTasks[idx].UpdatedBy = nextUpdatedBy
	}
	s.otaTasks[idx].UpdatedAt = now

	updatedTask := s.otaTasks[idx] // capture before advanceRolloutLocked may prepend new tasks
	if updatedTask.RolloutID != "" {
		if ri := s.findRolloutIndexLocked(updatedTask.RolloutID, ""); ri >= 0 {
			s.advanceRolloutLocked(ri)
		}
	}

	if err := s.persistLocked(); err != nil {
		return GatewayOTATask{}, err
	}
	return updatedTask, nil
```
(删除原先的 `return s.otaTasks[idx], nil` —— 用上面的 `updatedTask`。)

- [ ] **Step 5: 运行测试 + 回归**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Rollout|AutoAdvance|FailureGate|Stall' -v && go test ./internal/modules/gateway/ && go build ./... && go test ./internal/http/ -run OTA 2>&1 | tail -3`
Expected: 全 PASS(含既有 OTA 测试 —— UpdateOTATaskStatus 改动不破)。

- [ ] **Step 6: 提交**

```bash
git add api/internal/modules/gateway/rollout.go api/internal/modules/gateway/service.go api/internal/modules/gateway/rollout_test.go
git commit -m "feat: rollout advance engine + reactive advance on OTA report/GET"
```

---

## Task 4: 状态转移(approve / pause / resume / abort)

**Files:**
- Modify: `api/internal/modules/gateway/rollout.go`
- Test: `api/internal/modules/gateway/rollout_test.go`

- [ ] **Step 1: 写失败测试**

在 `rollout_test.go` 末尾追加:

```go
func TestRolloutApproveGate(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	// 2 phases over a single gateway: phase0=100 covers it; mark a (non-existent) 2nd? Single gw can't split.
	// Use 'all' with the seeded gateway; phases [50,100] on N=1 → cum(0)=ceil(.5)=1, cum(1)=1 → phase1 cohort empty.
	// Instead assert approve-state-conflict path on a fresh active rollout.
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
	})
	if _, err := svc.ApproveRollout("tenant_demo_jakarta", r.ID, "admin"); err != ErrRolloutStateConflict {
		t.Fatalf("approve on active should conflict, got %v", err)
	}
}

func TestRolloutPauseResumeAbort(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	mkActive := func() GatewayRollout {
		r, _ := svc.CreateRollout(CreateRolloutInput{
			TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
			Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
			Phases: []RolloutPhase{{Percentage: 100}},
		})
		return r
	}
	// pause active → paused; resume paused → active (phase not terminal yet, single gw not reported)
	r := mkActive()
	if got, err := svc.PauseRollout("tenant_demo_jakarta", r.ID, "admin"); err != nil || got.State != rolloutStatePaused {
		t.Fatalf("pause: %v %+v", err, got)
	}
	if _, err := svc.PauseRollout("tenant_demo_jakarta", r.ID, "admin"); err != ErrRolloutStateConflict {
		t.Fatalf("double pause should conflict, got %v", err)
	}
	if got, err := svc.ResumeRollout("tenant_demo_jakarta", r.ID, "admin"); err != nil || got.State != rolloutStateActive {
		t.Fatalf("resume: %v %+v", err, got)
	}
	// abort → failed
	if got, err := svc.AbortRollout("tenant_demo_jakarta", r.ID, "admin"); err != nil || got.State != rolloutStateFailed {
		t.Fatalf("abort: %v %+v", err, got)
	}
	if _, err := svc.AbortRollout("tenant_demo_jakarta", r.ID, "admin"); err != ErrRolloutStateConflict {
		t.Fatalf("abort on failed should conflict, got %v", err)
	}
}

func TestRolloutResumeOverridesFailureGate(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
	})
	reportTask(t, svc, "gw_demo_001", "failed") // → paused (100% fail)
	// resume on a failure-paused, single-phase rollout: terminal phase + last phase → completed (override).
	got, err := svc.ResumeRollout("tenant_demo_jakarta", r.ID, "admin")
	if err != nil || got.State != rolloutStateCompleted {
		t.Fatalf("resume override want completed, got %v %+v", err, got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'ApproveGate|PauseResumeAbort|ResumeOverrides' -v`
Expected: FAIL(`undefined: ApproveRollout` 等)。

- [ ] **Step 3: 状态转移(rollout.go)**

在 `rollout.go` 末尾追加:

```go
// PauseRollout halts an active rollout. Only valid from "active".
func (s *Service) PauseRollout(tenantID, id, actor string) (GatewayRollout, error) {
	return s.transitionRollout(tenantID, id, func(r *GatewayRollout, now time.Time) error {
		if r.State != rolloutStateActive {
			return ErrRolloutStateConflict
		}
		r.State = rolloutStatePaused
		r.UpdatedAt = now
		return nil
	})
}

// ResumeRollout reactivates a paused rollout. If the current phase is already terminal
// (e.g. it was paused by the failure gate), resume forces past it — an explicit operator override.
func (s *Service) ResumeRollout(tenantID, id, actor string) (GatewayRollout, error) {
	return s.transitionRolloutIdx(tenantID, id, func(ri int, now time.Time) error {
		r := &s.rollouts[ri]
		if r.State != rolloutStatePaused {
			return ErrRolloutStateConflict
		}
		r.State = rolloutStateActive
		r.UpdatedAt = now
		all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target)
		terminal, _ := s.evaluateRolloutPhaseLocked(r, all, now)
		if !terminal {
			s.advanceRolloutLocked(ri) // not terminal → no-op, just keep waiting
			return nil
		}
		// Terminal: override the failure gate and move past the current phase.
		if r.CurrentPhase >= len(r.Phases)-1 {
			r.State = rolloutStateCompleted
			return nil
		}
		next := r.CurrentPhase + 1
		if r.Phases[next].RequiresApproval {
			r.State = rolloutStateAwaitingApproval
			return nil
		}
		s.startRolloutPhaseLocked(r, next, all, now)
		s.advanceRolloutLocked(ri)
		return nil
	})
}

// ApproveRollout advances an awaiting-approval rollout into its next phase.
func (s *Service) ApproveRollout(tenantID, id, actor string) (GatewayRollout, error) {
	return s.transitionRolloutIdx(tenantID, id, func(ri int, now time.Time) error {
		r := &s.rollouts[ri]
		if r.State != rolloutStateAwaitingApproval {
			return ErrRolloutStateConflict
		}
		all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target)
		s.startRolloutPhaseLocked(r, r.CurrentPhase+1, all, now)
		s.advanceRolloutLocked(ri)
		return nil
	})
}

// AbortRollout fails a non-terminal rollout; no further tasks are created.
func (s *Service) AbortRollout(tenantID, id, actor string) (GatewayRollout, error) {
	return s.transitionRollout(tenantID, id, func(r *GatewayRollout, now time.Time) error {
		if r.State == rolloutStateCompleted || r.State == rolloutStateFailed {
			return ErrRolloutStateConflict
		}
		r.State = rolloutStateFailed
		r.UpdatedAt = now
		return nil
	})
}

// transitionRollout runs fn against a tenant-scoped rollout under lock, then persists.
func (s *Service) transitionRollout(tenantID, id string, fn func(*GatewayRollout, time.Time) error) (GatewayRollout, error) {
	return s.transitionRolloutIdx(tenantID, id, func(ri int, now time.Time) error {
		return fn(&s.rollouts[ri], now)
	})
}

func (s *Service) transitionRolloutIdx(tenantID, id string, fn func(int, time.Time) error) (GatewayRollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ri := s.findRolloutIndexLocked(strings.TrimSpace(id), strings.TrimSpace(tenantID))
	if ri < 0 {
		return GatewayRollout{}, ErrRolloutNotFound
	}
	if err := fn(ri, time.Now().UTC()); err != nil {
		return GatewayRollout{}, err
	}
	if err := s.persistLocked(); err != nil {
		return GatewayRollout{}, err
	}
	return s.rollouts[ri], nil
}
```

- [ ] **Step 4: 运行测试 + 回归**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Rollout' -v && go test ./internal/modules/gateway/ && go vet ./internal/modules/gateway/ && gofmt -l internal/modules/gateway/rollout.go`
Expected: 全 PASS;vet/gofmt clean。

- [ ] **Step 5: 提交**

```bash
git add api/internal/modules/gateway/rollout.go api/internal/modules/gateway/rollout_test.go
git commit -m "feat: rollout state transitions (approve/pause/resume/abort)"
```

---

## Task 5: HTTP 端点 + 每网关进度

**Files:**
- Create: `api/internal/http/routes_gateway_rollout.go`
- Modify: `api/internal/modules/gateway/rollout.go`(`RolloutGatewayProgress`)
- Modify: `api/internal/http/router.go`(注册 6 路由)
- Test: `api/internal/http/routes_gateway_rollout_test.go`

- [ ] **Step 1: 写失败测试**

Create `api/internal/http/routes_gateway_rollout_test.go`:

```go
package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"context"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func rolloutServer(t *testing.T) (*server, gateway.GatewayFirmware) {
	t.Helper()
	svc := gateway.NewService()
	fw, err := svc.CreateFirmware(gateway.CreateFirmwareInput{
		TenantID: "tenant_demo_jakarta", Version: "1.4.0",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Signature: strings.Repeat("b", 128),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &server{gatewaySvc: svc, cfg: config.Config{UploadStorageDir: t.TempDir(), UploadSigningKey: "k"}}, fw
}

func rolloutReq(method, target string, body any) *http.Request {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, target, bytes.NewReader(b))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	return withGatewayMQTTUser(r, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})
}

func withRolloutIDParam(r *http.Request, id string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add("rolloutID", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
}

func TestCreateAndGetRolloutEndpoint(t *testing.T) {
	s, fw := rolloutServer(t)
	rec := httptest.NewRecorder()
	s.createGatewayRollout(rec, rolloutReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": fw.ID,
		"target":      map[string]any{"kind": "all"},
		"phases":      []map[string]any{{"percentage": 100}},
	}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created gateway.GatewayRollout
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == "" || created.State != "active" {
		t.Fatalf("unexpected created rollout: %+v", created)
	}

	// detail with per-gateway progress
	dRec := httptest.NewRecorder()
	dReq := withRolloutIDParam(rolloutReq(http.MethodGet, "/api/v1/gateways/rollouts/"+created.ID+"?tenant_id=tenant_demo_jakarta", nil), created.ID)
	s.getGatewayRollout(dRec, dReq)
	if dRec.Code != http.StatusOK {
		t.Fatalf("detail expected 200, got %d body=%s", dRec.Code, dRec.Body.String())
	}
	var detail struct {
		Rollout  gateway.GatewayRollout         `json:"rollout"`
		Gateways []gateway.RolloutGatewayStatus `json:"gateways"`
	}
	_ = json.Unmarshal(dRec.Body.Bytes(), &detail)
	if detail.Rollout.ID != created.ID || len(detail.Gateways) == 0 {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	// invalid firmware → 400
	badRec := httptest.NewRecorder()
	s.createGatewayRollout(badRec, rolloutReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": "fw_nope", "target": map[string]any{"kind": "all"}, "phases": []map[string]any{{"percentage": 100}},
	}))
	if badRec.Code != http.StatusNotFound {
		t.Fatalf("unknown firmware expected 404, got %d", badRec.Code)
	}
}

func TestRolloutAbortEndpoint(t *testing.T) {
	s, fw := rolloutServer(t)
	rec := httptest.NewRecorder()
	s.createGatewayRollout(rec, rolloutReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": fw.ID, "target": map[string]any{"kind": "all"}, "phases": []map[string]any{{"percentage": 100}},
	}))
	var created gateway.GatewayRollout
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	aRec := httptest.NewRecorder()
	aReq := withRolloutIDParam(rolloutReq(http.MethodPost, "/api/v1/gateways/rollouts/"+created.ID+"/abort?tenant_id=tenant_demo_jakarta", nil), created.ID)
	s.abortGatewayRollout(aRec, aReq)
	if aRec.Code != http.StatusOK {
		t.Fatalf("abort expected 200, got %d body=%s", aRec.Code, aRec.Body.String())
	}
	var aborted gateway.GatewayRollout
	_ = json.Unmarshal(aRec.Body.Bytes(), &aborted)
	if aborted.State != "failed" {
		t.Fatalf("want failed, got %s", aborted.State)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/http/ -run 'Rollout' -v`
Expected: FAIL(`s.createGatewayRollout undefined` 等)。

- [ ] **Step 3: `RolloutGatewayProgress`(rollout.go)**

在 `rollout.go` 末尾追加:
```go
// RolloutGatewayStatus is one gateway's progress within a rollout.
type RolloutGatewayStatus struct {
	GatewayID              string `json:"gateway_id"`
	Phase                  int    `json:"phase"`      // -1 if not yet in any created cohort
	OTAStatus              string `json:"ota_status"` // queued|dispatching|succeeded|failed|timed_out|pending
	CurrentFirmwareVersion string `json:"current_firmware_version,omitempty"`
}

// RolloutGatewayProgress returns per-gateway progress for a rollout's full target set.
func (s *Service) RolloutGatewayProgress(tenantID, id string) ([]RolloutGatewayStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ri := s.findRolloutIndexLocked(strings.TrimSpace(id), strings.TrimSpace(tenantID))
	if ri < 0 {
		return nil, ErrRolloutNotFound
	}
	r := s.rollouts[ri]
	all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target)
	now := time.Now().UTC()
	out := make([]RolloutGatewayStatus, 0, len(all))
	for _, gw := range all {
		st := RolloutGatewayStatus{GatewayID: gw.ID, Phase: -1, OTAStatus: "pending", CurrentFirmwareVersion: gw.CurrentFirmwareVersion}
		// newest rollout task across phases for this gateway
		for i := range s.otaTasks {
			if s.otaTasks[i].RolloutID == r.ID && s.otaTasks[i].GatewayID == gw.ID {
				st.Phase = s.otaTasks[i].RolloutPhase
				st.OTAStatus = s.otaTasks[i].Status
				if s.otaTasks[i].Status != gatewayOTATaskStatusSucceeded &&
					s.otaTasks[i].Status != gatewayOTATaskStatusFailed &&
					st.Phase == r.CurrentPhase &&
					now.Sub(r.PhaseStartedAt) > rolloutStallWindow {
					st.OTAStatus = "timed_out"
				}
				break
			}
		}
		out = append(out, st)
	}
	return out, nil
}
```

- [ ] **Step 4: HTTP handlers(routes_gateway_rollout.go)**

Create `api/internal/http/routes_gateway_rollout.go`:
```go
package httpx

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func (s *server) createGatewayRollout(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var req struct {
		FirmwareID          string                 `json:"firmware_id"`
		Target              gateway.RolloutTarget  `json:"target"`
		Phases              []gateway.RolloutPhase `json:"phases"`
		FailureThresholdPct int                    `json:"failure_threshold_pct"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rollout, err := s.gatewaySvc.CreateRollout(gateway.CreateRolloutInput{
		TenantID:            tenantID,
		FirmwareID:          req.FirmwareID,
		Target:              req.Target,
		Phases:              req.Phases,
		FailureThresholdPct: req.FailureThresholdPct,
		CreatedBy:           requestActor(r),
	})
	if err != nil {
		writeRolloutError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rollout)
}

func (s *server) listGatewayRollouts(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.gatewaySvc.ListRollouts(tenantID)})
}

func (s *server) getGatewayRollout(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	id := chi.URLParam(r, "rolloutID")
	rollout, err := s.gatewaySvc.GetRollout(tenantID, id)
	if err != nil {
		writeRolloutError(w, err)
		return
	}
	progress, err := s.gatewaySvc.RolloutGatewayProgress(tenantID, id)
	if err != nil {
		writeRolloutError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rollout": rollout, "gateways": progress})
}

func (s *server) rolloutAction(action func(tenantID, id, actor string) (gateway.GatewayRollout, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
		if !ok {
			return
		}
		rollout, err := action(tenantID, chi.URLParam(r, "rolloutID"), requestActor(r))
		if err != nil {
			writeRolloutError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rollout)
	}
}

func (s *server) approveGatewayRollout(w http.ResponseWriter, r *http.Request) {
	s.rolloutAction(s.gatewaySvc.ApproveRollout)(w, r)
}
func (s *server) pauseGatewayRollout(w http.ResponseWriter, r *http.Request) {
	s.rolloutAction(s.gatewaySvc.PauseRollout)(w, r)
}
func (s *server) resumeGatewayRollout(w http.ResponseWriter, r *http.Request) {
	s.rolloutAction(s.gatewaySvc.ResumeRollout)(w, r)
}
func (s *server) abortGatewayRollout(w http.ResponseWriter, r *http.Request) {
	s.rolloutAction(s.gatewaySvc.AbortRollout)(w, r)
}

func writeRolloutError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gateway.ErrRolloutNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, gateway.ErrGatewayFirmwareNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, gateway.ErrRolloutStateConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, gateway.ErrRolloutFirmwareRequired),
		errors.Is(err, gateway.ErrRolloutTargetEmpty),
		errors.Is(err, gateway.ErrRolloutPhasesInvalid),
		errors.Is(err, gateway.ErrRolloutThresholdInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
```

- [ ] **Step 5: 注册路由(router.go)**

在 `router.go` 中,紧接 `Get("/gateways/firmware", s.listGatewayFirmware)` 那两行之后加入:
```go
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/rollouts", s.createGatewayRollout)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/rollouts", s.listGatewayRollouts)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/rollouts/{rolloutID}", s.getGatewayRollout)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/rollouts/{rolloutID}/approve", s.approveGatewayRollout)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/rollouts/{rolloutID}/pause", s.pauseGatewayRollout)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/rollouts/{rolloutID}/resume", s.resumeGatewayRollout)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/rollouts/{rolloutID}/abort", s.abortGatewayRollout)
```

- [ ] **Step 6: 运行测试 + 全量**

Run: `cd api && go test ./internal/http/ -run 'Rollout' -v && go build ./... && go test ./internal/http/ 2>&1 | tail -3 && go vet ./internal/http/ ./internal/modules/gateway/ && go run github.com/securego/gosec/v2/cmd/gosec@v2.22.10 -severity medium -confidence medium -exclude G115 -quiet ./internal/http/... ./internal/modules/gateway/... 2>&1 | tail -3`
Expected: PASS;build OK;http 整包 ok;vet clean;gosec 无新增。

- [ ] **Step 7: 提交**

```bash
git add api/internal/http/routes_gateway_rollout.go api/internal/http/routes_gateway_rollout_test.go api/internal/modules/gateway/rollout.go api/internal/http/router.go
git commit -m "feat: gateway rollout HTTP endpoints + per-gateway progress"
```

---

## 自检(Self-Review)

**1. Spec 覆盖**
- §3 数据模型(RolloutTarget/Phase/GatewayRollout + OTA 任务 RolloutID/Phase)→ Task 1/2 ✓
- §4 状态机(自动推进/失败门/awaiting_approval/pause/resume override/abort)→ Task 3/4 ✓
- §5.1 store + 持久化 → Task 2 ✓;§5.2 cohort → Task 1 ✓;§5.3 推进引擎 → Task 3 ✓;§5.4 OTA 归属 → Task 2 ✓;§5.5 端点 + 每网关进度 → Task 5 ✓;§5 stall → Task 3 ✓
- §7 错误(firmware 400/404、target 空、phases、阈值、409 冲突)→ Task 2/4/5 ✓
- §8 测试 → 各 Task ✓

**2. 占位符扫描**:无 TODO/TBD;代码步骤完整。

**3. 类型一致性**:`GatewayRollout`/`RolloutTarget`/`RolloutPhase`/`CreateRolloutInput`/`RolloutGatewayStatus` 字段全程一致;`advanceRolloutLocked(idx int)`/`evaluateRolloutPhaseLocked(*r, all, now)`/`startRolloutPhaseLocked(*r, phase, all, now)`/`findRolloutIndexLocked(id, tenant)`/`latestRolloutTaskLocked(rollout, phase, gw)` 签名全程一致;状态常量 `rolloutState*` 一致;`appendRolloutOTATaskLocked` 复用 `otaTaskID`/`gatewayOTATaskStatusQueued`;OTA 状态常量名实现者已核实。

**4. 关键风险点**:(a) `UpdateOTATaskStatus` 改动后 idx 位移——已用 `updatedTask` 先捕获;(b) advance 循环空 cohort 自动跳过(避免卡死);(c) resume override 仅当当前阶段终态时强制前进;(d) cohort `ceil` 边界单网关 fleet 已测。

---

## 执行交接(建议 Subagent-Driven)
状态机 + 反应式推进逻辑较密,建议 **superpowers:subagent-driven-development**(每 Task 实现者 + spec/质量两阶段审查)。
