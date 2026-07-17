# OTA Rollout 调度(start_at + 维护窗口)Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 #3 的 rollout 加调度:`start_at`(定时启动)+ 每日维护窗口(本地时段 + IANA 时区,支持跨午夜),窗口门控阶段启动;反应式(config/pull 触发),不新增 worker(对标 Kisi OTA 子项 #4)。

**Architecture:** 新增 `scheduled` 状态 + `RolloutSchedule`(内嵌 `GatewayRollout`)。`scheduleOpenLocked` 判定"现在能否启动阶段"。所有"启动阶段"经 `tryStartPhaseLocked`(窗口开→建任务 active;关→停 scheduled 等窗口)。config/pull 调 `EvaluateGatewayScheduledRollouts` 启动到点的调度。入口加 `_ "time/tzdata"` 确保时区库。

**Tech Stack:** Go、`time.LoadLocation`/`time/tzdata`、in-memory service + stateSnapshot 持久化。

设计依据:[2026-06-07-ota-rollout-scheduling-design.md](../specs/2026-06-07-ota-rollout-scheduling-design.md)

**约定:** `go` 在 `api/` 下;`gateway.NewService()` 预置 `gw_demo_001`(tenant `tenant_demo_jakarta`)。复用 #3:`GatewayRollout`/`CreateRollout`/`startRolloutPhaseLocked`/`advanceRolloutLocked`/`evaluateRolloutPhaseLocked`/`rolloutTargetGatewaysLocked`/`findRolloutIndexLocked`/`transitionRolloutIdx`/状态常量 `rolloutState*`(在 `internal/modules/gateway/rollout.go`)。#4 测试可复用 #3 的 `seedFirmware(t, svc)` 助手。

---

## Task 1: Schedule 模型 + scheduleOpenLocked + 校验 + tzdata

**Files:**
- Modify: `api/internal/modules/gateway/rollout.go`(`RolloutSchedule` 类型、`rolloutStateScheduled` 常量、`ErrRolloutScheduleInvalid`、`GatewayRollout.Schedule` 字段、`scheduleOpenLocked`、`validateRolloutSchedule`)
- Modify: `api/cmd/api/main.go`(加 `_ "time/tzdata"`)
- Test: `api/internal/modules/gateway/rollout_test.go`

- [ ] **Step 1: 写失败测试**

在 `rollout_test.go` 末尾追加:

```go
func TestScheduleOpenLocked(t *testing.T) {
	base := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC) // 12:00 UTC = 19:00 Asia/Jakarta (UTC+7)
	future := base.Add(time.Hour)
	past := base.Add(-time.Hour)

	// nil schedule → always open
	if !scheduleOpenLocked(nil, base) {
		t.Fatal("nil schedule must be open")
	}
	// start_at in the future → closed; in the past → open
	if scheduleOpenLocked(&RolloutSchedule{StartAt: &future}, base) {
		t.Fatal("before start_at must be closed")
	}
	if !scheduleOpenLocked(&RolloutSchedule{StartAt: &past}, base) {
		t.Fatal("after start_at (no window) must be open")
	}
	// UTC window containing 12:00
	if !scheduleOpenLocked(&RolloutSchedule{WindowStart: "11:00", WindowEnd: "13:00"}, base) {
		t.Fatal("12:00 within 11:00-13:00 UTC must be open")
	}
	if scheduleOpenLocked(&RolloutSchedule{WindowStart: "13:00", WindowEnd: "14:00"}, base) {
		t.Fatal("12:00 outside 13:00-14:00 must be closed")
	}
	// overnight window 22:00→05:00: 23:30 inside, 12:00 outside
	night := time.Date(2026, 6, 7, 23, 30, 0, 0, time.UTC)
	if !scheduleOpenLocked(&RolloutSchedule{WindowStart: "22:00", WindowEnd: "05:00"}, night) {
		t.Fatal("23:30 within overnight 22:00-05:00 must be open")
	}
	if scheduleOpenLocked(&RolloutSchedule{WindowStart: "22:00", WindowEnd: "05:00"}, base) {
		t.Fatal("12:00 outside overnight 22:00-05:00 must be closed")
	}
	// timezone: 12:00 UTC is 19:00 Jakarta → inside 18:00-20:00 Jakarta, outside same UTC window
	jkt := &RolloutSchedule{WindowStart: "18:00", WindowEnd: "20:00", Timezone: "Asia/Jakarta"}
	if !scheduleOpenLocked(jkt, base) {
		t.Fatal("19:00 Jakarta within 18:00-20:00 Jakarta must be open")
	}
}

func TestValidateRolloutSchedule(t *testing.T) {
	if err := validateRolloutSchedule(nil); err != nil {
		t.Fatalf("nil schedule valid, got %v", err)
	}
	ok := &RolloutSchedule{WindowStart: "02:00", WindowEnd: "05:00", Timezone: "Asia/Jakarta"}
	if err := validateRolloutSchedule(ok); err != nil {
		t.Fatalf("valid schedule rejected: %v", err)
	}
	bad := []*RolloutSchedule{
		{WindowStart: "02:00"},                         // only one bound
		{WindowStart: "2:00", WindowEnd: "05:00"},      // not zero-padded HH:MM
		{WindowStart: "24:00", WindowEnd: "05:00"},     // out of range hour
		{WindowStart: "02:60", WindowEnd: "05:00"},     // out of range minute
		{Timezone: "Mars/Olympus"},                     // bad timezone
	}
	for i, b := range bad {
		if err := validateRolloutSchedule(b); err != ErrRolloutScheduleInvalid {
			t.Fatalf("bad case %d: want ErrRolloutScheduleInvalid, got %v", i, err)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'ScheduleOpen|ValidateRolloutSchedule' -v`
Expected: FAIL(`undefined: RolloutSchedule`/`scheduleOpenLocked` 等)。

- [ ] **Step 3: 实现(rollout.go)**

(a) 在 `const (...)` 块(`rolloutStateFailed` 那行之后)加:
```go
	rolloutStateScheduled = "scheduled"
```
(b) 在错误 `var (...)` 块加:
```go
	ErrRolloutScheduleInvalid = errors.New("rollout schedule is invalid (timezone must be a valid IANA name; window_start/window_end must both be set as HH:MM)")
```
(c) `GatewayRollout` struct(`Phases` 字段之后)加:
```go
	Schedule            *RolloutSchedule `json:"schedule,omitempty"`
```
(d) 在 `import` 块加 `"regexp"`(若未引入)。文件顶部(类型定义附近)加:
```go
// RolloutSchedule defers/gates a rollout: start_at = earliest absolute start; the daily
// local-time window [WindowStart, WindowEnd) (WindowEnd < WindowStart = overnight) gates phase starts.
type RolloutSchedule struct {
	StartAt     *time.Time `json:"start_at,omitempty"`
	WindowStart string     `json:"window_start,omitempty"` // "HH:MM" local 24h
	WindowEnd   string     `json:"window_end,omitempty"`   // "HH:MM" local 24h
	Timezone    string     `json:"timezone,omitempty"`     // IANA, e.g. "Asia/Jakarta"; empty = UTC
}

var rolloutHHMMRe = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// validateRolloutSchedule checks an optional schedule's window format + timezone.
func validateRolloutSchedule(sch *RolloutSchedule) error {
	if sch == nil {
		return nil
	}
	ws, we := strings.TrimSpace(sch.WindowStart), strings.TrimSpace(sch.WindowEnd)
	if (ws == "") != (we == "") {
		return ErrRolloutScheduleInvalid
	}
	if ws != "" && (!rolloutHHMMRe.MatchString(ws) || !rolloutHHMMRe.MatchString(we)) {
		return ErrRolloutScheduleInvalid
	}
	if tz := strings.TrimSpace(sch.Timezone); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return ErrRolloutScheduleInvalid
		}
	}
	return nil
}

// scheduleOpenLocked reports whether a phase may start now under the schedule.
func scheduleOpenLocked(sch *RolloutSchedule, now time.Time) bool {
	if sch == nil {
		return true
	}
	if sch.StartAt != nil && now.Before(*sch.StartAt) {
		return false
	}
	ws, we := strings.TrimSpace(sch.WindowStart), strings.TrimSpace(sch.WindowEnd)
	if ws == "" || we == "" {
		return true // no window → start_at alone gates
	}
	loc := time.UTC
	if tz := strings.TrimSpace(sch.Timezone); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	hhmm := now.In(loc).Format("15:04")
	if ws <= we {
		return ws <= hhmm && hhmm < we
	}
	return hhmm >= ws || hhmm < we // overnight
}
```

- [ ] **Step 4: 入口加 tzdata(main.go)**

在 `api/cmd/api/main.go` 的 import 块加一行(让 `LoadLocation` 在无 OS 时区库的容器里也可用):
```go
	_ "time/tzdata"
```
(放在 stdlib import 区,如 `"time"` 之后或 import 块末尾的空行分组里;`gofmt` 会排序。)

- [ ] **Step 5: 运行测试 + 回归**

Run: `cd api && go test ./internal/modules/gateway/ -run 'ScheduleOpen|ValidateRolloutSchedule' -v && go test ./internal/modules/gateway/ && go build ./... && go vet ./internal/modules/gateway/ && gofmt -l internal/modules/gateway/rollout.go cmd/api/main.go`
Expected: PASS;build OK;vet/gofmt clean。

- [ ] **Step 6: 提交**

```bash
git add api/internal/modules/gateway/rollout.go api/internal/modules/gateway/rollout_test.go api/cmd/api/main.go
git commit -m "feat: rollout schedule model + scheduleOpen + validation + tzdata"
```

---

## Task 2: tryStartPhaseLocked + advance 扩展 + CreateRollout 收 schedule

**Files:**
- Modify: `api/internal/modules/gateway/rollout.go`(`tryStartPhaseLocked`;`advanceRolloutLocked` 扩展;`CreateRollout`/`CreateRolloutInput` + 校验;approve/resume 改 tryStart)
- Test: `api/internal/modules/gateway/rollout_test.go`

- [ ] **Step 1: 写失败测试**

在 `rollout_test.go` 末尾追加:

```go
func TestCreateRolloutScheduledVsActive(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	future := time.Now().UTC().Add(time.Hour)
	// start_at in the future → scheduled, NO phase-0 tasks yet
	r, err := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
		Schedule: &RolloutSchedule{StartAt: &future},
	})
	if err != nil {
		t.Fatalf("create scheduled: %v", err)
	}
	if r.State != rolloutStateScheduled {
		t.Fatalf("want scheduled, got %s", r.State)
	}
	tasks, _ := svc.ListOTATasks("tenant_demo_jakarta", "gw_demo_001")
	for _, task := range tasks {
		if task.RolloutID == r.ID {
			t.Fatal("scheduled rollout must not create phase-0 tasks yet")
		}
	}
	// no schedule → active immediately (existing #3 behavior)
	r2, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
	})
	if r2.State != rolloutStateActive {
		t.Fatalf("no-schedule rollout want active, got %s", r2.State)
	}
}

func TestScheduledRolloutStartsWhenWindowOpens(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	future := time.Now().UTC().Add(time.Hour)
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
		Schedule: &RolloutSchedule{StartAt: &future},
	})
	// "open the window": back-date start_at into the past, then advance via GetRollout.
	past := time.Now().UTC().Add(-time.Hour)
	svc.mu.Lock()
	idx := svc.findRolloutIndexLocked(r.ID, "tenant_demo_jakarta")
	svc.rollouts[idx].Schedule.StartAt = &past
	svc.mu.Unlock()
	got, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if got.State != rolloutStateActive {
		t.Fatalf("want active after window opens, got %s", got.State)
	}
	tasks, _ := svc.ListOTATasks("tenant_demo_jakarta", "gw_demo_001")
	found := false
	for _, task := range tasks {
		if task.RolloutID == r.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("phase-0 tasks should exist after the schedule opens")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'CreateRolloutScheduled|ScheduledRolloutStarts' -v`
Expected: FAIL(`unknown field Schedule`/状态不符)。

- [ ] **Step 3: tryStartPhaseLocked + CreateRolloutInput.Schedule**

(a) 在 `startRolloutPhaseLocked` 之后加:
```go
// tryStartPhaseLocked starts the phase's cohort if the schedule window is open; otherwise parks
// the rollout in "scheduled" at that phase to await the window. Caller holds s.mu.
func (s *Service) tryStartPhaseLocked(r *GatewayRollout, phase int, all []Gateway, now time.Time) {
	if scheduleOpenLocked(r.Schedule, now) {
		s.startRolloutPhaseLocked(r, phase, all, now)
		return
	}
	r.CurrentPhase = phase
	r.State = rolloutStateScheduled
	r.UpdatedAt = now
}
```
(b) `CreateRolloutInput`(`CreatedBy` 之后)加字段:
```go
	Schedule            *RolloutSchedule
```

- [ ] **Step 4: CreateRollout 校验 + 用 tryStart + 存 Schedule**

在 `CreateRollout` 里:
- threshold 校验之后(`s.mu.Lock()` 之前)加:
```go
	if err := validateRolloutSchedule(in.Schedule); err != nil {
		return GatewayRollout{}, err
	}
```
- 记录字面量(`Phases: in.Phases,` 之后)加:
```go
		Schedule:            in.Schedule,
```
- 把 `s.startRolloutPhaseLocked(&s.rollouts[0], 0, all, now)` 改为:
```go
	s.tryStartPhaseLocked(&s.rollouts[0], 0, all, now)
```

- [ ] **Step 5: advanceRolloutLocked 扩展(处理 scheduled + 窗口门控推进)**

把 `advanceRolloutLocked` 整个函数体替换为:
```go
func (s *Service) advanceRolloutLocked(rolloutIdx int) bool {
	r := &s.rollouts[rolloutIdx]
	now := time.Now().UTC()
	all := s.rolloutTargetGatewaysLocked(r.TenantID, r.Target) // target set is immutable under the lock
	changed := false
	for {
		switch r.State {
		case rolloutStateScheduled:
			if !scheduleOpenLocked(r.Schedule, now) {
				return changed // still waiting for the window
			}
			s.startRolloutPhaseLocked(r, r.CurrentPhase, all, now) // window open → start the parked phase
			changed = true
		case rolloutStateActive:
			terminal, failureRate := s.evaluateRolloutPhaseLocked(r, all, now)
			if !terminal {
				return changed
			}
			changed = true
			r.UpdatedAt = now
			if failureRate >= r.FailureThresholdPct {
				r.State = rolloutStatePaused
				return changed
			}
			if r.CurrentPhase >= len(r.Phases)-1 {
				r.State = rolloutStateCompleted
				return changed
			}
			next := r.CurrentPhase + 1
			if r.Phases[next].RequiresApproval {
				r.State = rolloutStateAwaitingApproval
				return changed
			}
			s.tryStartPhaseLocked(r, next, all, now) // window-gated; may park in scheduled
		default:
			return changed
		}
	}
}
```

- [ ] **Step 6: approve/resume 的阶段启动也走 tryStart(窗口是硬约束)**

在 `ResumeRollout` 里把 `s.startRolloutPhaseLocked(r, next, all, now)` 改为 `s.tryStartPhaseLocked(r, next, all, now)`。
在 `ApproveRollout` 里把 `s.startRolloutPhaseLocked(r, r.CurrentPhase+1, all, now)` 改为 `s.tryStartPhaseLocked(r, r.CurrentPhase+1, all, now)`。
(理由:维护窗口是硬约束;approve/resume 只覆盖**失败/审批门**,不覆盖**时间窗口**——窗口关时该阶段落 scheduled 等窗口。)

- [ ] **Step 7: 运行测试 + 回归**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Rollout|Scheduled|CreateRollout' -v && go test ./internal/modules/gateway/ && go build ./... && go vet ./internal/modules/gateway/ && gofmt -l internal/modules/gateway/rollout.go`
Expected: 全 PASS(含 #3 既有 rollout 测试不破);build OK;vet/gofmt clean。

- [ ] **Step 8: 提交**

```bash
git add api/internal/modules/gateway/rollout.go api/internal/modules/gateway/rollout_test.go
git commit -m "feat: window-gated phase starts + scheduled state in advance engine"
```

---

## Task 3: config/pull 触发 + pause/abort advance-only-if-active

**Files:**
- Modify: `api/internal/modules/gateway/rollout.go`(`EvaluateGatewayScheduledRollouts` + `rolloutTargetIncludesLocked`;`PauseRollout`/`AbortRollout` 改 advance-only-if-active)
- Modify: `api/internal/http/router_handlers_gateway.go`(config/pull 调 Evaluate)
- Test: `api/internal/modules/gateway/rollout_test.go` + `api/internal/http/routes_gateway_bootstrap_test.go`

- [ ] **Step 1: 写失败测试**

在 `rollout_test.go` 末尾追加:

```go
func TestEvaluateGatewayScheduledRollouts(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	future := time.Now().UTC().Add(time.Hour)
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
		Schedule: &RolloutSchedule{StartAt: &future},
	})
	// open the window
	past := time.Now().UTC().Add(-time.Hour)
	svc.mu.Lock()
	svc.rollouts[svc.findRolloutIndexLocked(r.ID, "tenant_demo_jakarta")].Schedule.StartAt = &past
	svc.mu.Unlock()
	// a non-target gateway pull does NOT start it
	if err := svc.EvaluateGatewayScheduledRollouts("tenant_demo_jakarta", "gw_other"); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.getRolloutForTest(r.ID); got.State != rolloutStateScheduled {
		t.Fatalf("non-target pull must not start; got %s", got.State)
	}
	// the target gateway pull starts it
	if err := svc.EvaluateGatewayScheduledRollouts("tenant_demo_jakarta", "gw_demo_001"); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.getRolloutForTest(r.ID); got.State != rolloutStateActive {
		t.Fatalf("target pull should start; got %s", got.State)
	}
}

func TestAbortScheduledDoesNotStartIt(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	past := time.Now().UTC().Add(-time.Hour)
	// schedule whose window is OPEN now (start_at in past, no window) but created as scheduled by
	// using a future start then back-dating would race; instead create open and assert active,
	// then test abort on a genuinely-scheduled (future) rollout.
	future := time.Now().UTC().Add(time.Hour)
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
		Schedule: &RolloutSchedule{StartAt: &future},
	})
	// back-date so the window is open, but abort must NOT start it (advance-only-if-active)
	svc.mu.Lock()
	svc.rollouts[svc.findRolloutIndexLocked(r.ID, "tenant_demo_jakarta")].Schedule.StartAt = &past
	svc.mu.Unlock()
	got, err := svc.AbortRollout("tenant_demo_jakarta", r.ID, "admin")
	if err != nil || got.State != rolloutStateFailed {
		t.Fatalf("abort scheduled want failed, got %v %s", err, got.State)
	}
	tasks, _ := svc.ListOTATasks("tenant_demo_jakarta", "gw_demo_001")
	for _, task := range tasks {
		if task.RolloutID == r.ID {
			t.Fatal("abort must not have started the rollout (no tasks)")
		}
	}
}
```
并在 `rollout_test.go` 加一个测试助手(避免 GetRollout 触发 advance 干扰断言):
```go
func (s *Service) getRolloutForTest(id string) (GatewayRollout, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i := s.findRolloutIndexLocked(id, ""); i >= 0 {
		return s.rollouts[i], true
	}
	return GatewayRollout{}, false
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/modules/gateway/ -run 'EvaluateGatewayScheduled|AbortScheduled' -v`
Expected: FAIL(`undefined: EvaluateGatewayScheduledRollouts`/abort 把它启动了)。

- [ ] **Step 3: EvaluateGatewayScheduledRollouts + helper**

在 `rollout.go` 末尾追加:
```go
// EvaluateGatewayScheduledRollouts advances scheduled rollouts in the tenant whose target includes
// the gateway — the reactive "window opened" trigger fired from config/pull. Caller must NOT hold s.mu.
func (s *Service) EvaluateGatewayScheduledRollouts(tenantID, gatewayID string) error {
	ft := strings.TrimSpace(tenantID)
	gw := strings.TrimSpace(gatewayID)
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := range s.rollouts {
		if s.rollouts[i].TenantID != ft || s.rollouts[i].State != rolloutStateScheduled {
			continue
		}
		if !s.rolloutTargetIncludesLocked(s.rollouts[i].TenantID, s.rollouts[i].Target, gw) {
			continue
		}
		if s.advanceRolloutLocked(i) {
			changed = true
		}
	}
	if changed {
		return s.persistLocked()
	}
	return nil
}

// rolloutTargetIncludesLocked reports whether gatewayID is in the rollout's resolved target set.
func (s *Service) rolloutTargetIncludesLocked(tenantID string, target RolloutTarget, gatewayID string) bool {
	for _, gw := range s.rolloutTargetGatewaysLocked(tenantID, target) {
		if gw.ID == gatewayID {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: pause/abort advance-only-if-active**

在 `PauseRollout` 里,把 `s.advanceRolloutLocked(ri)` 那行(无条件)改为:
```go
		if s.rollouts[ri].State == rolloutStateActive {
			s.advanceRolloutLocked(ri) // refresh only an active rollout (never start a scheduled one)
		}
```
在 `AbortRollout` 里,同样把无条件的 `s.advanceRolloutLocked(ri)` 改为:
```go
		if s.rollouts[ri].State == rolloutStateActive {
			s.advanceRolloutLocked(ri)
		}
```

- [ ] **Step 5: config/pull 触发(router_handlers_gateway.go)**

在 `gatewayBootstrapConfigPull` 里,紧接 `_ = s.gatewaySvc.RecordFirmwareVersion(request.TenantID, request.GatewayID, request.FirmwareVersion)` 那行之后加:
```go
	_ = s.gatewaySvc.EvaluateGatewayScheduledRollouts(request.TenantID, request.GatewayID)
```
(best-effort;放在组装 `pendingOTA` 之前,使刚启动的 rollout 任务能在同一次 pull 进入 pending_ota_tasks。)

- [ ] **Step 6: HTTP 触发回归测试**

在 `api/internal/http/routes_gateway_bootstrap_test.go` 末尾追加:
```go
func TestConfigPullStartsScheduledRollout(t *testing.T) {
	svc := gateway.NewService()
	fw, _ := svc.CreateFirmware(gateway.CreateFirmwareInput{
		TenantID: "tenant_demo_jakarta", Version: "1.4.0",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Signature: strings.Repeat("b", 128),
	})
	past := time.Now().UTC().Add(-time.Hour)
	if _, err := svc.CreateRollout(gateway.CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: gateway.RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []gateway.RolloutPhase{{Percentage: 100}},
		Schedule: &gateway.RolloutSchedule{StartAt: &past}, // window already open
	}); err != nil {
		t.Fatal(err)
	}
	s := &server{
		gatewaySvc:          svc,
		gatewayDeviceTokens: map[string]string{"gw_demo_001": "gw_test_token_001"},
		cfg:                 config.Config{UploadStorageDir: t.TempDir(), UploadSigningKey: "k"},
	}
	body, _ := json.Marshal(map[string]any{"gateway_id": "gw_demo_001", "tenant_id": "tenant_demo_jakarta"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/gateway/config/pull", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer gw_test_token_001")
	rec := httptest.NewRecorder()
	s.gatewayBootstrapConfigPull(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config/pull expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		PendingOTATasks []struct {
			FirmwareURL string `json:"firmware_url"`
		} `json:"pending_ota_tasks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.PendingOTATasks) == 0 {
		t.Fatal("config/pull should have started the scheduled rollout and returned its task")
	}
}
```
(确保该测试文件已 import `"time"`;其余 import bytes/json/http/httptest/strings/config/gateway 已在。)

- [ ] **Step 7: 运行测试 + 全量**

Run: `cd api && go test ./internal/modules/gateway/ -run 'Rollout|Scheduled|Abort' -v && go test ./internal/modules/gateway/ && go build ./... && go test ./internal/http/ -run 'ConfigPull|Rollout' -v 2>&1 | tail -15 && go test ./internal/http/ 2>&1 | tail -3`
Expected: 全 PASS。

- [ ] **Step 8: 提交**

```bash
git add api/internal/modules/gateway/rollout.go api/internal/modules/gateway/rollout_test.go api/internal/http/router_handlers_gateway.go api/internal/http/routes_gateway_bootstrap_test.go
git commit -m "feat: config/pull starts scheduled rollouts; pause/abort never start scheduled"
```

---

## Task 4: HTTP create 接受 schedule

**Files:**
- Modify: `api/internal/http/routes_gateway_rollout.go`(create 请求体加 `schedule` + `writeRolloutError` 加 ScheduleInvalid→400)
- Test: `api/internal/http/routes_gateway_rollout_test.go`

- [ ] **Step 1: 写失败测试**

在 `routes_gateway_rollout_test.go` 末尾追加:
```go
func TestCreateRolloutWithSchedule(t *testing.T) {
	s, fw := rolloutTestServer(t)
	rec := httptest.NewRecorder()
	s.createGatewayRollout(rec, rolloutTestReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": fw.ID,
		"target":      map[string]any{"kind": "all"},
		"phases":      []map[string]any{{"percentage": 100}},
		"schedule":    map[string]any{"window_start": "02:00", "window_end": "05:00", "timezone": "Asia/Jakarta"},
	}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create with schedule expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created gateway.GatewayRollout
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Schedule == nil || created.Schedule.WindowStart != "02:00" {
		t.Fatalf("schedule not round-tripped: %+v", created.Schedule)
	}
	// bad timezone → 400
	badRec := httptest.NewRecorder()
	s.createGatewayRollout(badRec, rolloutTestReq(http.MethodPost, "/api/v1/gateways/rollouts?tenant_id=tenant_demo_jakarta", map[string]any{
		"firmware_id": fw.ID, "target": map[string]any{"kind": "all"}, "phases": []map[string]any{{"percentage": 100}},
		"schedule": map[string]any{"timezone": "Mars/Olympus"},
	}))
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad timezone expected 400, got %d", badRec.Code)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/http/ -run TestCreateRolloutWithSchedule -v`
Expected: FAIL(请求体无 schedule 字段;schedule 为 nil)。

- [ ] **Step 3: create handler 接 schedule + 错误映射**

在 `routes_gateway_rollout.go` 的 `createGatewayRollout` 请求体 struct(`FailureThresholdPct` 之后)加:
```go
		Schedule            *gateway.RolloutSchedule `json:"schedule,omitempty"`
```
并在 `CreateRolloutInput{...}` 里(`FailureThresholdPct: req.FailureThresholdPct,` 之后)加:
```go
		Schedule:            req.Schedule,
```
在 `writeRolloutError` 的 400 分支(`ErrRolloutThresholdInvalid` 那组)加一项:
```go
		errors.Is(err, gateway.ErrRolloutScheduleInvalid),
```

- [ ] **Step 4: 运行测试 + 全量 + gosec**

Run: `cd api && go test ./internal/http/ -run 'Rollout' -v && go build ./... && go test ./internal/http/ 2>&1 | tail -3 && go vet ./internal/http/ ./internal/modules/gateway/ && go run github.com/securego/gosec/v2/cmd/gosec@v2.22.10 -severity medium -confidence medium -exclude G115 -quiet ./internal/http/... ./internal/modules/gateway/... 2>&1 | tail -3`
Expected: PASS;build OK;http 整包 ok;vet clean;gosec 无新增。

- [ ] **Step 5: 提交**

```bash
git add api/internal/http/routes_gateway_rollout.go api/internal/http/routes_gateway_rollout_test.go
git commit -m "feat: rollout create endpoint accepts schedule"
```

---

## 自检(Self-Review)

**1. Spec 覆盖**
- §3 数据模型(RolloutSchedule + scheduled 常量 + Schedule 字段)→ Task 1 ✓
- §4 scheduleOpenLocked(start_at/窗口/跨午夜/时区/无调度)→ Task 1 ✓
- §5 tryStartPhaseLocked + advance 扩展(scheduled 分支 + 窗口门控推进)+ CreateRollout tryStart → Task 2 ✓;approve/resume 也 tryStart(窗口硬约束)→ Task 2 Step 6 ✓
- §6 EvaluateGatewayScheduledRollouts + config/pull 触发 → Task 3 ✓
- §7 create 收 schedule;abort scheduled→failed;pause/abort advance-only-if-active(避免误启动)→ Task 3/4 ✓
- §8 校验(tz/半截窗口/HH:MM)→ Task 1(validateRolloutSchedule)+ Task 4(400 映射)✓
- §10 tzdata 入口 → Task 1 Step 4 ✓

**2. 占位符扫描**:无 TODO/TBD;代码步骤完整。

**3. 类型一致性**:`RolloutSchedule` 字段(StartAt/WindowStart/WindowEnd/Timezone)全程一致;`scheduleOpenLocked(sch, now)`/`validateRolloutSchedule(sch)`/`tryStartPhaseLocked(r, phase, all, now)`/`EvaluateGatewayScheduledRollouts(tenant, gw)`/`rolloutTargetIncludesLocked(tenant, target, gw)` 签名一致;`rolloutStateScheduled`/`ErrRolloutScheduleInvalid` 一致;`CreateRolloutInput.Schedule` ↔ create handler `req.Schedule` 一致。

**4. 关键风险点**:(a) 所有"启动阶段"统一经 tryStartPhaseLocked(Create/advance/approve/resume)→ 窗口是硬约束,无绕过口;(b) pause/abort advance-only-if-active → 不会误启动 scheduled;(c) advance 循环 scheduled↔active 切换终止性:CurrentPhase 单调增 + scheduled 窗口关即 return;(d) 时区库 tzdata 入口引入,LoadLocation 全环境可用 + CreateRollout 校验拒坏时区。

---

## 执行交接(建议 Subagent-Driven)
状态机门控 + 时区/窗口判定较细,建议 **superpowers:subagent-driven-development**。
