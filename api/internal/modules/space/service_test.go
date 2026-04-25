package space

import (
	"errors"
	"testing"
)

func TestCreateTopologySuccess(t *testing.T) {
	svc := NewService()

	building, err := svc.CreateBuilding("tenant_custom", "  New Hub  ", "Jl. Example 1", "ID-JK")
	if err != nil {
		t.Fatalf("CreateBuilding returned error: %v", err)
	}
	if building.TenantID != "tenant_custom" {
		t.Fatalf("unexpected building tenant: %s", building.TenantID)
	}
	if building.Name != "New Hub" {
		t.Fatalf("expected trimmed building name, got %q", building.Name)
	}
	if building.CreatedAt.IsZero() {
		t.Fatalf("expected building created_at to be set")
	}

	floor, err := svc.CreateFloor("tenant_custom", building.ID, "L21")
	if err != nil {
		t.Fatalf("CreateFloor returned error: %v", err)
	}
	if floor.BuildingID != building.ID {
		t.Fatalf("expected floor building %s, got %s", building.ID, floor.BuildingID)
	}

	area, err := svc.CreateArea("tenant_custom", building.ID, floor.ID, "Finance")
	if err != nil {
		t.Fatalf("CreateArea returned error: %v", err)
	}
	if area.FloorID != floor.ID {
		t.Fatalf("expected area floor %s, got %s", floor.ID, area.FloorID)
	}

	door, err := svc.CreateDoor("tenant_custom", building.ID, floor.ID, area.ID, "Main Gate", "gw_01", "office", "online")
	if err != nil {
		t.Fatalf("CreateDoor returned error: %v", err)
	}
	if door.AreaID != area.ID {
		t.Fatalf("expected door area %s, got %s", area.ID, door.AreaID)
	}
	if door.Kind != "office" || door.Status != "online" {
		t.Fatalf("unexpected door kind/status: %s/%s", door.Kind, door.Status)
	}

	topology, err := svc.TopologyByTenant("tenant_custom")
	if err != nil {
		t.Fatalf("TopologyByTenant returned error: %v", err)
	}
	if len(topology.Buildings) != 1 || len(topology.Floors) != 1 || len(topology.Areas) != 1 || len(topology.Doors) != 1 {
		t.Fatalf("unexpected topology counts: %+v", topology)
	}
}

func TestCreateTopologyRejectsTenantOwnershipMismatch(t *testing.T) {
	svc := NewService()

	if _, err := svc.CreateFloor("tenant_demo_factory", "building_demo_001", "Factory Floor"); !errors.Is(err, ErrTenantOwnershipMismatch) {
		t.Fatalf("expected CreateFloor tenant mismatch, got %v", err)
	}

	if _, err := svc.CreateArea("tenant_demo_jakarta", "building_demo_001", "floor_demo_003", "Mixed Area"); !errors.Is(err, ErrTenantOwnershipMismatch) {
		t.Fatalf("expected CreateArea tenant mismatch, got %v", err)
	}

	if _, err := svc.CreateDoor("tenant_demo_jakarta", "building_demo_001", "floor_demo_001", "area_demo_003", "Mixed Door", "gw_02", "office", "online"); !errors.Is(err, ErrTenantOwnershipMismatch) {
		t.Fatalf("expected CreateDoor tenant mismatch, got %v", err)
	}
}

func TestCreateDoorRejectsInvalidKindAndStatus(t *testing.T) {
	svc := NewService()

	if _, err := svc.CreateDoor("tenant_demo_jakarta", "building_demo_001", "floor_demo_001", "area_demo_001", "Door A", "gw_01", "invalid", "online"); !errors.Is(err, ErrInvalidDoorKind) {
		t.Fatalf("expected invalid door kind error, got %v", err)
	}

	if _, err := svc.CreateDoor("tenant_demo_jakarta", "building_demo_001", "floor_demo_001", "area_demo_001", "Door A", "gw_01", "office", "unknown"); !errors.Is(err, ErrInvalidDoorStatus) {
		t.Fatalf("expected invalid door status error, got %v", err)
	}
}
