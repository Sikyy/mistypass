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
