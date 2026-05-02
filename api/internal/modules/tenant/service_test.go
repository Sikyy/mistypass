package tenant

import (
	"errors"
	"testing"
)

func TestServiceCreateSuccess(t *testing.T) {
	svc := NewService()

	record, err := svc.Create("  Example Manufacturing  ", "", "  ID-JK  ")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if record.Name != "Example Manufacturing" {
		t.Fatalf("expected trimmed name, got %q", record.Name)
	}
	if record.Type != "company" {
		t.Fatalf("expected default type company, got %s", record.Type)
	}
	if record.HQRegion != "ID-JK" {
		t.Fatalf("expected trimmed region, got %q", record.HQRegion)
	}
	if record.Status != "active" {
		t.Fatalf("expected active status, got %s", record.Status)
	}
	if record.CreatedAt.IsZero() {
		t.Fatalf("expected created_at to be set")
	}
}

func TestServiceCreateRejectsInvalidInput(t *testing.T) {
	svc := NewService()

	if _, err := svc.Create("   ", "company", "ID-JK"); !errors.Is(err, ErrTenantNameRequired) {
		t.Fatalf("expected tenant name required error, got %v", err)
	}

	if _, err := svc.Create("Example", "invalid_type", "ID-JK"); !errors.Is(err, ErrInvalidTenantType) {
		t.Fatalf("expected invalid tenant type error, got %v", err)
	}
}

func TestServiceListReturnsDefaultTenants(t *testing.T) {
	svc := NewService()
	tenants := svc.List()
	if len(tenants) < 2 {
		t.Fatalf("expected at least 2 default tenants, got %d", len(tenants))
	}
	found := false
	for _, t2 := range tenants {
		if t2.ID == "tenant_demo_jakarta" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected tenant_demo_jakarta in default list")
	}
}

func TestServiceCreateAppearsInList(t *testing.T) {
	svc := NewService()
	before := len(svc.List())

	created, err := svc.Create("Test Corp", "studio", "SG")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	after := svc.List()
	if len(after) != before+1 {
		t.Fatalf("expected %d tenants after create, got %d", before+1, len(after))
	}
	found := false
	for _, t2 := range after {
		if t2.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("created tenant not found in list")
	}
}

func TestServiceUpdateStatusLifecycle(t *testing.T) {
	svc := NewService()

	record, err := svc.UpdateStatus("tenant_demo_jakarta", " suspended ")
	if err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	if record.Status != "suspended" {
		t.Fatalf("expected suspended status, got %s", record.Status)
	}

	if _, err := svc.UpdateStatus("tenant_demo_jakarta", "bad"); !errors.Is(err, ErrInvalidTenantStatus) {
		t.Fatalf("expected invalid tenant status error, got %v", err)
	}

	if _, err := svc.UpdateStatus("tenant_missing", "active"); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("expected tenant not found error, got %v", err)
	}
}

func TestServiceStatusFullCycle(t *testing.T) {
	svc := NewService()
	created, err := svc.Create("Lifecycle Corp", "factory", "ID-JK")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Status != "active" {
		t.Fatalf("expected active, got %s", created.Status)
	}

	suspended, err := svc.UpdateStatus(created.ID, "suspended")
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if suspended.Status != "suspended" {
		t.Fatalf("expected suspended, got %s", suspended.Status)
	}

	inactive, err := svc.UpdateStatus(created.ID, "inactive")
	if err != nil {
		t.Fatalf("inactive: %v", err)
	}
	if inactive.Status != "inactive" {
		t.Fatalf("expected inactive, got %s", inactive.Status)
	}

	reactivated, err := svc.UpdateStatus(created.ID, "active")
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if reactivated.Status != "active" {
		t.Fatalf("expected active, got %s", reactivated.Status)
	}
}

func TestServiceNilSafetyGuards(t *testing.T) {
	var svc *Service
	if svc.List() != nil {
		t.Fatal("nil service List should return nil")
	}
	if _, err := svc.Create("x", "company", ""); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("nil service Create should return error, got %v", err)
	}
	if _, err := svc.UpdateStatus("x", "active"); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("nil service UpdateStatus should return error, got %v", err)
	}
}
