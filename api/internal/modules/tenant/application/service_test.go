package application

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/tenant/domain"
)

// memRepo is an in-memory Repository for testing the application service.
type memRepo struct {
	data map[string][]byte
}

func newMemRepo() *memRepo {
	return &memRepo{data: make(map[string][]byte)}
}

func (m *memRepo) LoadTenants(defaults []domain.Tenant) ([]domain.Tenant, error) {
	raw, ok := m.data["tenants"]
	if !ok {
		return domain.CloneTenants(defaults), nil
	}
	var items []domain.Tenant
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *memRepo) SaveTenants(items []domain.Tenant) error {
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	m.data["tenants"] = raw
	return nil
}

// errRepo returns errors for testing error paths.
type errRepo struct {
	loadErr error
	saveErr error
}

func (e *errRepo) LoadTenants(defaults []domain.Tenant) ([]domain.Tenant, error) {
	if e.loadErr != nil {
		return nil, e.loadErr
	}
	return domain.CloneTenants(defaults), nil
}

func (e *errRepo) SaveTenants(_ []domain.Tenant) error {
	return e.saveErr
}

// seqIDGen returns a deterministic ID generator for testing.
func seqIDGen(prefix string) domain.IDGenerator {
	counter := 0
	return func() (string, error) {
		counter++
		return prefix + "_" + string(rune('0'+counter)), nil
	}
}

// failIDGen returns an ID generator that always fails.
func failIDGen() domain.IDGenerator {
	return func() (string, error) {
		return "", errors.New("id generation failed")
	}
}

func fixedTime() time.Time {
	return time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
}

// --- NewService ---

func TestNewService_WithNilRepo(t *testing.T) {
	svc, err := NewService(nil, []domain.Tenant{
		{ID: "d1", Name: "Default"},
	}, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	items := svc.List()
	if len(items) != 1 || items[0].ID != "d1" {
		t.Fatalf("expected 1 default tenant, got %v", items)
	}
}

func TestNewService_LoadsFromRepo(t *testing.T) {
	repo := newMemRepo()
	// Pre-seed the repo
	existing := []domain.Tenant{
		{ID: "existing_1", Name: "From Repo", Status: "active"},
	}
	raw, _ := json.Marshal(existing)
	repo.data["tenants"] = raw

	defaults := []domain.Tenant{
		{ID: "default_1", Name: "Ignored Default"},
	}

	svc, err := NewService(repo, defaults, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	items := svc.List()
	if len(items) != 1 || items[0].ID != "existing_1" {
		t.Fatalf("expected repo data, got %v", items)
	}
}

func TestNewService_RepoLoadError(t *testing.T) {
	repo := &errRepo{loadErr: errors.New("load failed")}
	_, err := NewService(repo, nil, nil)
	if err == nil {
		t.Fatal("expected error when repo load fails")
	}
}

// --- Create ---

func TestCreate_PersistsToRepo(t *testing.T) {
	repo := newMemRepo()
	svc, err := NewService(repo, nil, seqIDGen("test"))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	created, err := svc.Create("Persisted Corp", "factory", "ID-JK")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "Persisted Corp" {
		t.Fatalf("expected Persisted Corp, got %s", created.Name)
	}

	// Verify the repo received the data
	loaded, err := repo.LoadTenants(nil)
	if err != nil {
		t.Fatalf("LoadTenants: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 tenant in repo, got %d", len(loaded))
	}
	if loaded[0].ID != created.ID {
		t.Fatalf("expected %s in repo, got %s", created.ID, loaded[0].ID)
	}
}

func TestCreate_UsesCustomIDGenerator(t *testing.T) {
	svc, err := NewService(nil, nil, func() (string, error) {
		return "custom_id_123", nil
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	created, err := svc.Create("Custom ID Corp", "company", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != "custom_id_123" {
		t.Fatalf("expected custom_id_123, got %s", created.ID)
	}
}

func TestCreate_IDGeneratorFailure(t *testing.T) {
	svc, err := NewService(nil, nil, failIDGen())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.Create("Will Fail", "company", "")
	if err == nil {
		t.Fatal("expected error when ID generator fails")
	}
}

func TestCreate_RepoSaveFailure(t *testing.T) {
	repo := &errRepo{saveErr: errors.New("save failed")}
	svc, err := NewService(repo, nil, seqIDGen("test"))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.Create("Fail Save", "company", "")
	if err == nil {
		t.Fatal("expected error when repo save fails")
	}
}

func TestCreate_EmptyNameRejected(t *testing.T) {
	svc, _ := NewService(nil, nil, nil)

	_, err := svc.Create("", "company", "")
	if !errors.Is(err, domain.ErrTenantNameRequired) {
		t.Fatalf("expected ErrTenantNameRequired, got %v", err)
	}

	_, err = svc.Create("   ", "company", "")
	if !errors.Is(err, domain.ErrTenantNameRequired) {
		t.Fatalf("expected ErrTenantNameRequired for whitespace, got %v", err)
	}
}

func TestCreate_InvalidTypeRejected(t *testing.T) {
	svc, _ := NewService(nil, nil, nil)

	_, err := svc.Create("Valid Name", "bogus_type", "")
	if !errors.Is(err, domain.ErrInvalidTenantType) {
		t.Fatalf("expected ErrInvalidTenantType, got %v", err)
	}
}

func TestCreate_DefaultsTypeToCompany(t *testing.T) {
	svc, _ := NewService(nil, nil, nil)
	created, err := svc.Create("No Type", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Type != "company" {
		t.Fatalf("expected company default type, got %s", created.Type)
	}
}

func TestCreate_TrimsInputFields(t *testing.T) {
	svc, _ := NewService(nil, nil, nil)
	created, err := svc.Create("  Trimmed Corp  ", "  studio  ", "  ID-JK  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "Trimmed Corp" {
		t.Fatalf("expected trimmed name, got %q", created.Name)
	}
	if created.Type != "studio" {
		t.Fatalf("expected studio, got %s", created.Type)
	}
	if created.HQRegion != "ID-JK" {
		t.Fatalf("expected trimmed region, got %q", created.HQRegion)
	}
}

func TestCreate_SetsActiveStatusAndTimestamp(t *testing.T) {
	svc, _ := NewService(nil, nil, nil)
	before := time.Now().UTC()
	created, err := svc.Create("New Corp", "company", "")
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Status != "active" {
		t.Fatalf("expected active, got %s", created.Status)
	}
	if created.CreatedAt.Before(before) || created.CreatedAt.After(after) {
		t.Fatalf("created_at %v not between %v and %v", created.CreatedAt, before, after)
	}
}

// --- UpdateStatus ---

func TestUpdateStatus_PersistsToRepo(t *testing.T) {
	repo := newMemRepo()
	defaults := []domain.Tenant{
		{ID: "t1", Name: "Corp", Status: "active"},
	}
	svc, err := NewService(repo, defaults, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	updated, err := svc.UpdateStatus("t1", "suspended")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != "suspended" {
		t.Fatalf("expected suspended, got %s", updated.Status)
	}

	// Verify persisted
	loaded, err := repo.LoadTenants(nil)
	if err != nil {
		t.Fatalf("LoadTenants: %v", err)
	}
	if loaded[0].Status != "suspended" {
		t.Fatalf("expected suspended in repo, got %s", loaded[0].Status)
	}
}

func TestUpdateStatus_RepoSaveFailure(t *testing.T) {
	repo := &errRepo{saveErr: errors.New("save failed")}
	svc, err := NewService(repo, []domain.Tenant{
		{ID: "t1", Name: "Corp", Status: "active"},
	}, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.UpdateStatus("t1", "suspended")
	if err == nil {
		t.Fatal("expected error when repo save fails")
	}
}

func TestUpdateStatus_EmptyTenantID(t *testing.T) {
	svc, _ := NewService(nil, nil, nil)

	_, err := svc.UpdateStatus("", "active")
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound for empty ID, got %v", err)
	}

	_, err = svc.UpdateStatus("   ", "active")
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound for whitespace ID, got %v", err)
	}
}

func TestUpdateStatus_InvalidStatus(t *testing.T) {
	svc, _ := NewService(nil, []domain.Tenant{
		{ID: "t1", Name: "Corp", Status: "active"},
	}, nil)

	_, err := svc.UpdateStatus("t1", "deleted")
	if !errors.Is(err, domain.ErrInvalidTenantStatus) {
		t.Fatalf("expected ErrInvalidTenantStatus, got %v", err)
	}
}

func TestUpdateStatus_TenantNotFound(t *testing.T) {
	svc, _ := NewService(nil, []domain.Tenant{
		{ID: "t1", Name: "Corp", Status: "active"},
	}, nil)

	_, err := svc.UpdateStatus("nonexistent", "active")
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound, got %v", err)
	}
}

// --- List ---

func TestList_ReturnsClone(t *testing.T) {
	svc, _ := NewService(nil, []domain.Tenant{
		{ID: "t1", Name: "Original", Status: "active"},
	}, nil)

	items := svc.List()
	items[0].Name = "MUTATED"

	fresh := svc.List()
	if fresh[0].Name == "MUTATED" {
		t.Fatal("List returned a reference instead of a clone")
	}
}

func TestList_EmptyDefaults(t *testing.T) {
	svc, _ := NewService(nil, nil, nil)
	items := svc.List()
	if len(items) != 0 {
		t.Fatalf("expected 0 tenants, got %d", len(items))
	}
}

// --- Concurrency ---

func TestConcurrentCreateAndList(t *testing.T) {
	svc, err := NewService(nil, nil, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	var wg sync.WaitGroup
	const goroutines = 20

	// Half create, half list -- no race detector failures
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				svc.Create("Concurrent Corp", "company", "")
			} else {
				svc.List()
			}
		}(i)
	}
	wg.Wait()

	// Verify the expected number of tenants were created
	items := svc.List()
	expected := goroutines / 2 // half of goroutines did creates
	if len(items) != expected {
		t.Fatalf("expected %d tenants, got %d", expected, len(items))
	}
}

func TestConcurrentUpdateStatus(t *testing.T) {
	defaults := make([]domain.Tenant, 10)
	for i := range defaults {
		defaults[i] = domain.Tenant{
			ID:     "t" + string(rune('0'+i)),
			Name:   "Corp",
			Status: "active",
		}
	}

	svc, err := NewService(nil, defaults, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	var wg sync.WaitGroup
	statuses := []string{"active", "suspended", "inactive"}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			idx := n % len(defaults)
			status := statuses[n%len(statuses)]
			svc.UpdateStatus(defaults[idx].ID, status)
		}(i)
	}
	wg.Wait()

	// Just verify no panics and all tenants are present
	items := svc.List()
	if len(items) != len(defaults) {
		t.Fatalf("expected %d tenants, got %d", len(defaults), len(items))
	}
}
