package gateway

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestValidateRolloutPhases(t *testing.T) {
	ok := []RolloutPhase{{Percentage: 10}, {Percentage: 50}, {Percentage: 100}}
	if !validateRolloutPhases(ok) {
		t.Fatal("expected valid phases")
	}
	bad := [][]RolloutPhase{
		{},
		{{Percentage: 10}, {Percentage: 50}}, // last != 100
		{{Percentage: 50}, {Percentage: 50}, {Percentage: 100}}, // not strictly increasing
		{{Percentage: 0}, {Percentage: 100}},                    // 0 out of range
		{{Percentage: 10}, {Percentage: 101}},                   // >100
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

func TestRolloutPhaseZeroSerializes(t *testing.T) {
	b, err := json.Marshal(GatewayOTATask{RolloutID: "rollout_x", RolloutPhase: 0})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"rollout_phase":0`) {
		t.Fatalf("phase-0 task must serialize rollout_phase, got %s", string(b))
	}
}

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
	svc.mu.Lock()
	idx := svc.findRolloutIndexLocked(r.ID, "tenant_demo_jakarta")
	svc.rollouts[idx].PhaseStartedAt = time.Now().UTC().Add(-2 * time.Hour)
	svc.mu.Unlock()
	got, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if got.State != rolloutStatePaused {
		t.Fatalf("want paused via stall, got %s", got.State)
	}
}

func TestRolloutApproveGate(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
	})
	// approve on an active (not awaiting_approval) rollout must conflict.
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

func TestRolloutApproveAdvances(t *testing.T) {
	svc := NewService()
	fw := seedFirmware(t, svc)
	r, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 50}, {Percentage: 100, RequiresApproval: true}},
	})
	reportTask(t, svc, "gw_demo_001", "succeeded") // phase 0 done → next phase requires approval
	got, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if got.State != rolloutStateAwaitingApproval {
		t.Fatalf("want awaiting_approval, got %s", got.State)
	}
	approved, err := svc.ApproveRollout("tenant_demo_jakarta", r.ID, "admin")
	if err != nil || approved.State != rolloutStateCompleted {
		t.Fatalf("approve want completed, got %v %s", err, approved.State)
	}
	if approved.UpdatedBy != "admin" {
		t.Fatalf("want UpdatedBy=admin, got %q", approved.UpdatedBy)
	}
}

// seedJakartaGateway directly appends an extra online gateway to tenant_demo_jakarta so a
// rollout's target can resolve to a real multi-gateway cohort. Direct seeding (mirroring how
// NewService builds s.gateways, and how TestRolloutStallTimeoutCountsAsFailure pokes s.rollouts
// under the lock) keeps the in-package test free of the serial-inventory provisioning that the
// public Register path requires. The id is chosen to sort deterministically after gw_demo_001
// so cohort slicing is predictable: cohortForPhase sorts the target set by ID.
func seedJakartaGateway(t *testing.T, svc *Service, id string) {
	t.Helper()
	if id <= "gw_demo_001" {
		t.Fatalf("seed gateway id %q must sort after gw_demo_001 for deterministic cohorts", id)
	}
	svc.mu.Lock()
	svc.gateways = append(svc.gateways, Gateway{
		ID:             id,
		TenantID:       "tenant_demo_jakarta",
		SerialNumber:   "MP-GW-JKT-" + id,
		BuildingID:     "building_demo_001",
		DeviceCapacity: 8,
		Status:         "online",
		LastSeenAt:     time.Now().UTC(),
	})
	svc.mu.Unlock()
}

func rolloutTaskFor(t *testing.T, svc *Service, rolloutID, gatewayID string) (GatewayOTATask, bool) {
	t.Helper()
	tasks, err := svc.ListOTATasks("tenant_demo_jakarta", gatewayID)
	if err != nil {
		t.Fatalf("list tasks for %s: %v", gatewayID, err)
	}
	for _, task := range tasks {
		if task.RolloutID == rolloutID {
			return task, true
		}
	}
	return GatewayOTATask{}, false
}

// TestRolloutMultiGatewayPhaseProgression drives a 2-phase rollout over a real 2-gateway cohort
// in one tenant: phase 0's cohort (gw_demo_001) must reach terminal before phase 1's cohort
// (the second, higher-sorted gateway) is dispatched. This exercises the advance engine across
// distinct per-phase cohorts — the committed suite otherwise only covers single-gateway (N=1)
// rollouts where every phase shares the one gateway.
func TestRolloutMultiGatewayPhaseProgression(t *testing.T) {
	const second = "gw_demo_jkt_002"
	svc := NewService()
	fw := seedFirmware(t, svc)
	seedJakartaGateway(t, svc, second)

	r, err := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001", second}},
		Phases: []RolloutPhase{{Percentage: 50}, {Percentage: 100}},
	})
	if err != nil {
		t.Fatalf("create rollout: %v", err)
	}

	// Phase 0 (ceil(50%*2)=1) covers only gw_demo_001; the second gateway is gated until phase 1.
	if _, ok := rolloutTaskFor(t, svc, r.ID, "gw_demo_001"); !ok {
		t.Fatal("phase-0 cohort gw_demo_001 should have an OTA task at creation")
	}
	if task, ok := rolloutTaskFor(t, svc, r.ID, second); ok {
		t.Fatalf("phase-1 gateway %s must not have a task before phase 0 completes, got %+v", second, task)
	}

	// Phase 0 succeeds → advance engine should open phase 1 and dispatch the second gateway.
	reportTask(t, svc, "gw_demo_001", "succeeded")
	got, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if got.State != rolloutStateActive || got.CurrentPhase != 1 {
		t.Fatalf("after phase-0 success want active/phase 1, got %s/phase %d", got.State, got.CurrentPhase)
	}
	task, ok := rolloutTaskFor(t, svc, r.ID, second)
	if !ok {
		t.Fatalf("phase-1 task for %s must exist after phase 0 succeeded", second)
	}
	if task.RolloutPhase != 1 {
		t.Fatalf("phase-1 task for %s want RolloutPhase 1, got %d", second, task.RolloutPhase)
	}

	// Phase 1 (final) succeeds → rollout completes.
	reportTask(t, svc, second, "succeeded")
	done, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if done.State != rolloutStateCompleted {
		t.Fatalf("after final phase success want completed, got %s", done.State)
	}
}

// TestRolloutMultiGatewayFailureGateMidPhase verifies the failure gate halts a multi-gateway
// rollout between cohorts: phase 0's gateway fails (100% of a terminal cohort, over the 20%
// threshold) so the rollout pauses and phase 1's gateway is never dispatched.
func TestRolloutMultiGatewayFailureGateMidPhase(t *testing.T) {
	const second = "gw_demo_jkt_002"
	svc := NewService()
	fw := seedFirmware(t, svc)
	seedJakartaGateway(t, svc, second)

	r, err := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001", second}},
		Phases: []RolloutPhase{{Percentage: 50}, {Percentage: 100}}, FailureThresholdPct: 20,
	})
	if err != nil {
		t.Fatalf("create rollout: %v", err)
	}

	// Phase 0's only gateway fails → terminal cohort at 100% failure ≥ 20% → paused at phase 0.
	reportTask(t, svc, "gw_demo_001", "failed")
	got, _ := svc.GetRollout("tenant_demo_jakarta", r.ID)
	if got.State != rolloutStatePaused {
		t.Fatalf("phase-0 failure want paused, got %s", got.State)
	}
	if got.CurrentPhase != 0 {
		t.Fatalf("failure gate must halt at phase 0, got phase %d", got.CurrentPhase)
	}
	if task, ok := rolloutTaskFor(t, svc, r.ID, second); ok {
		t.Fatalf("phase-1 gateway %s must not be dispatched once phase 0 trips the failure gate, got %+v", second, task)
	}
}

func TestScheduleOpenLocked(t *testing.T) {
	base := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC) // 12:00 UTC = 19:00 Asia/Jakarta (UTC+7)
	future := base.Add(time.Hour)
	past := base.Add(-time.Hour)

	if !scheduleOpenLocked(nil, base) {
		t.Fatal("nil schedule must be open")
	}
	if scheduleOpenLocked(&RolloutSchedule{StartAt: &future}, base) {
		t.Fatal("before start_at must be closed")
	}
	if !scheduleOpenLocked(&RolloutSchedule{StartAt: &past}, base) {
		t.Fatal("after start_at (no window) must be open")
	}
	if !scheduleOpenLocked(&RolloutSchedule{WindowStart: "11:00", WindowEnd: "13:00"}, base) {
		t.Fatal("12:00 within 11:00-13:00 UTC must be open")
	}
	if scheduleOpenLocked(&RolloutSchedule{WindowStart: "13:00", WindowEnd: "14:00"}, base) {
		t.Fatal("12:00 outside 13:00-14:00 must be closed")
	}
	night := time.Date(2026, 6, 7, 23, 30, 0, 0, time.UTC)
	if !scheduleOpenLocked(&RolloutSchedule{WindowStart: "22:00", WindowEnd: "05:00"}, night) {
		t.Fatal("23:30 within overnight 22:00-05:00 must be open")
	}
	if scheduleOpenLocked(&RolloutSchedule{WindowStart: "22:00", WindowEnd: "05:00"}, base) {
		t.Fatal("12:00 outside overnight 22:00-05:00 must be closed")
	}
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
		{WindowStart: "02:00"},
		{WindowStart: "2:00", WindowEnd: "05:00"},
		{WindowStart: "24:00", WindowEnd: "05:00"},
		{WindowStart: "02:60", WindowEnd: "05:00"},
		{Timezone: "Mars/Olympus"},
	}
	for i, b := range bad {
		if err := validateRolloutSchedule(b); err != ErrRolloutScheduleInvalid {
			t.Fatalf("bad case %d: want ErrRolloutScheduleInvalid, got %v", i, err)
		}
	}
}

func TestRolloutAbortFromPausedAndAwaiting(t *testing.T) {
	// abort from paused
	svc := NewService()
	fw := seedFirmware(t, svc)
	r1, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 100}},
	})
	if _, err := svc.PauseRollout("tenant_demo_jakarta", r1.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.AbortRollout("tenant_demo_jakarta", r1.ID, "admin"); err != nil || got.State != rolloutStateFailed {
		t.Fatalf("abort from paused: %v %+v", err, got)
	}
	// abort from awaiting_approval
	r2, _ := svc.CreateRollout(CreateRolloutInput{
		TenantID: "tenant_demo_jakarta", FirmwareID: fw.ID,
		Target: RolloutTarget{Kind: "gateways", GatewayIDs: []string{"gw_demo_001"}},
		Phases: []RolloutPhase{{Percentage: 50}, {Percentage: 100, RequiresApproval: true}},
	})
	reportTask(t, svc, "gw_demo_001", "succeeded") // → awaiting_approval
	if got, err := svc.AbortRollout("tenant_demo_jakarta", r2.ID, "admin"); err != nil || got.State != rolloutStateFailed {
		t.Fatalf("abort from awaiting_approval: %v %+v", err, got)
	}
}
