package gateway

import "testing"

func TestValidateRolloutPhases(t *testing.T) {
	ok := []RolloutPhase{{Percentage: 10}, {Percentage: 50}, {Percentage: 100}}
	if !validateRolloutPhases(ok) {
		t.Fatal("expected valid phases")
	}
	bad := [][]RolloutPhase{
		{},
		{{Percentage: 10}, {Percentage: 50}},                    // last != 100
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
