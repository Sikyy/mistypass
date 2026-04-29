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

func TestDeleteBuildingArchivesPlaceAndRemovesActiveTopology(t *testing.T) {
	svc := NewService()

	building, err := svc.CreateBuilding("tenant_custom", "Archive Hub", "Jl. Archive 1", "ID-JK")
	if err != nil {
		t.Fatalf("CreateBuilding returned error: %v", err)
	}
	floor, err := svc.CreateFloor("tenant_custom", building.ID, "L1")
	if err != nil {
		t.Fatalf("CreateFloor returned error: %v", err)
	}
	area, err := svc.CreateArea("tenant_custom", building.ID, floor.ID, "Lobby")
	if err != nil {
		t.Fatalf("CreateArea returned error: %v", err)
	}
	door, err := svc.CreateDoor("tenant_custom", building.ID, floor.ID, area.ID, "Lobby Door", "gw_archive", "office", "online")
	if err != nil {
		t.Fatalf("CreateDoor returned error: %v", err)
	}

	if err := svc.DeleteBuilding("tenant_custom", building.ID); err != nil {
		t.Fatalf("DeleteBuilding returned error: %v", err)
	}
	if _, err := svc.GetBuilding("tenant_custom", building.ID); !errors.Is(err, ErrBuildingNotFound) {
		t.Fatalf("expected archived building to be hidden from active detail, got %v", err)
	}
	archived, err := svc.GetBuildingIncludingArchived("tenant_custom", building.ID)
	if err != nil {
		t.Fatalf("GetBuildingIncludingArchived returned error: %v", err)
	}
	if archived.Status != "archived" || archived.ArchivedAt == nil {
		t.Fatalf("expected archived building metadata, got %#v", archived)
	}
	for _, item := range svc.ListBuildings("tenant_custom") {
		if item.ID == building.ID {
			t.Fatalf("expected active building list to exclude archived building")
		}
	}
	foundArchived := false
	for _, item := range svc.ListBuildingsIncludingArchived("tenant_custom") {
		if item.ID == building.ID && item.Status == "archived" {
			foundArchived = true
		}
	}
	if !foundArchived {
		t.Fatalf("expected archived building to remain queryable")
	}
	if _, err := svc.GetDoor("tenant_custom", door.ID); !errors.Is(err, ErrDoorNotFound) {
		t.Fatalf("expected archived building door to be removed from active topology, got %v", err)
	}
	topology, err := svc.TopologyByTenant("tenant_custom")
	if err != nil {
		t.Fatalf("TopologyByTenant returned error: %v", err)
	}
	if len(topology.Buildings) != 0 || len(topology.Floors) != 0 || len(topology.Areas) != 0 || len(topology.Doors) != 0 {
		t.Fatalf("expected archived topology to be hidden, got %+v", topology)
	}
}

func TestUpdateAreaKeepsDoorTopologyConsistent(t *testing.T) {
	svc := NewService()

	building, err := svc.CreateBuilding("tenant_custom", "Topology Hub", "Jl. Example 2", "ID-JK")
	if err != nil {
		t.Fatalf("CreateBuilding returned error: %v", err)
	}
	firstFloor, err := svc.CreateFloor("tenant_custom", building.ID, "L1")
	if err != nil {
		t.Fatalf("CreateFloor first returned error: %v", err)
	}
	secondFloor, err := svc.CreateFloor("tenant_custom", building.ID, "L2")
	if err != nil {
		t.Fatalf("CreateFloor second returned error: %v", err)
	}
	area, err := svc.CreateArea("tenant_custom", building.ID, firstFloor.ID, "Lobby")
	if err != nil {
		t.Fatalf("CreateArea returned error: %v", err)
	}
	door, err := svc.CreateDoor("tenant_custom", building.ID, firstFloor.ID, area.ID, "Lobby Door", "gw_01", "office", "online")
	if err != nil {
		t.Fatalf("CreateDoor returned error: %v", err)
	}

	updated, err := svc.UpdateArea("tenant_custom", area.ID, building.ID, secondFloor.ID, "Level 2 Lobby")
	if err != nil {
		t.Fatalf("UpdateArea returned error: %v", err)
	}
	if updated.Name != "Level 2 Lobby" || updated.FloorID != secondFloor.ID {
		t.Fatalf("unexpected updated area: %#v", updated)
	}
	updatedDoor, err := svc.GetDoor("tenant_custom", door.ID)
	if err != nil {
		t.Fatalf("GetDoor returned error: %v", err)
	}
	if updatedDoor.FloorID != secondFloor.ID || updatedDoor.BuildingID != building.ID {
		t.Fatalf("expected door topology to follow updated area, got %#v", updatedDoor)
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
