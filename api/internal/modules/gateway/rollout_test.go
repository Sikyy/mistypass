package gateway

import (
	"encoding/json"
	"strings"
	"testing"
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
