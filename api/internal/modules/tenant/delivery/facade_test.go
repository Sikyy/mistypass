package delivery

import (
	"errors"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/tenant/application"
	"github.com/mistypass/cloud/api/internal/modules/tenant/domain"
)

func newTestFacade(defaults []domain.Tenant) *Facade {
	svc, _ := application.NewService(nil, defaults, nil)
	return NewFacade(svc)
}

// --- Nil safety ---

func TestFacade_NilFacade_List(t *testing.T) {
	var f *Facade
	items := f.List()
	if items != nil {
		t.Fatalf("expected nil from nil facade, got %v", items)
	}
}

func TestFacade_NilFacade_Create(t *testing.T) {
	var f *Facade
	_, err := f.Create("Test", "company", "")
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound from nil facade, got %v", err)
	}
}

func TestFacade_NilFacade_UpdateStatus(t *testing.T) {
	var f *Facade
	_, err := f.UpdateStatus("t1", "active")
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound from nil facade, got %v", err)
	}
}

func TestFacade_NilService(t *testing.T) {
	f := NewFacade(nil)

	if f.List() != nil {
		t.Fatal("expected nil from nil-service facade List")
	}

	_, err := f.Create("Test", "company", "")
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound from nil-service Create, got %v", err)
	}

	_, err = f.UpdateStatus("t1", "active")
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound from nil-service UpdateStatus, got %v", err)
	}
}

// --- Delegation ---

func TestFacade_List_DelegatesToService(t *testing.T) {
	f := newTestFacade([]domain.Tenant{
		{ID: "t1", Name: "One", Status: "active"},
		{ID: "t2", Name: "Two", Status: "active"},
	})

	items := f.List()
	if len(items) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(items))
	}
}

func TestFacade_Create_DelegatesToService(t *testing.T) {
	f := newTestFacade(nil)

	created, err := f.Create("New Corp", "factory", "ID-JK")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "New Corp" {
		t.Fatalf("expected New Corp, got %s", created.Name)
	}
	if created.Type != "factory" {
		t.Fatalf("expected factory, got %s", created.Type)
	}

	// Should appear in list
	items := f.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 tenant after create, got %d", len(items))
	}
}

func TestFacade_Create_PropagatesValidationErrors(t *testing.T) {
	f := newTestFacade(nil)

	_, err := f.Create("", "company", "")
	if !errors.Is(err, domain.ErrTenantNameRequired) {
		t.Fatalf("expected ErrTenantNameRequired, got %v", err)
	}

	_, err = f.Create("Valid Name", "bogus", "")
	if !errors.Is(err, domain.ErrInvalidTenantType) {
		t.Fatalf("expected ErrInvalidTenantType, got %v", err)
	}
}

func TestFacade_UpdateStatus_DelegatesToService(t *testing.T) {
	f := newTestFacade([]domain.Tenant{
		{ID: "t1", Name: "Corp", Status: "active"},
	})

	updated, err := f.UpdateStatus("t1", "suspended")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != "suspended" {
		t.Fatalf("expected suspended, got %s", updated.Status)
	}
}

func TestFacade_UpdateStatus_PropagatesErrors(t *testing.T) {
	f := newTestFacade([]domain.Tenant{
		{ID: "t1", Name: "Corp", Status: "active"},
	})

	_, err := f.UpdateStatus("nonexistent", "active")
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound, got %v", err)
	}

	_, err = f.UpdateStatus("t1", "bogus_status")
	if !errors.Is(err, domain.ErrInvalidTenantStatus) {
		t.Fatalf("expected ErrInvalidTenantStatus, got %v", err)
	}
}
