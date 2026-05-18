package infrastructure

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/tenant/domain"
)

// memStore is a simple in-memory implementation of StateStore for testing.
type memStore struct {
	data map[string][]byte
}

func newMemStore() *memStore {
	return &memStore{data: make(map[string][]byte)}
}

func (m *memStore) Load(key string, dst any) (bool, error) {
	raw, ok := m.data[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

func (m *memStore) Save(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.data[key] = raw
	return nil
}

// errStore always returns errors from Load/Save.
type errStore struct {
	loadErr error
	saveErr error
}

func (e *errStore) Load(key string, dst any) (bool, error) {
	if e.loadErr != nil {
		return false, e.loadErr
	}
	return false, nil
}

func (e *errStore) Save(key string, value any) error {
	return e.saveErr
}

func TestStateRepository_LoadTenants_NoExistingState(t *testing.T) {
	store := newMemStore()
	repo := NewStateRepository(store)
	defaults := []domain.Tenant{
		{ID: "t1", Name: "Default One", Status: "active"},
	}

	loaded, err := repo.LoadTenants(defaults)
	if err != nil {
		t.Fatalf("LoadTenants: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 tenant, got %d", len(loaded))
	}
	if loaded[0].ID != "t1" {
		t.Fatalf("expected t1, got %s", loaded[0].ID)
	}

	// Defaults should have been persisted to the store
	var snapshot stateSnapshot
	found, err := store.Load(stateKey, &snapshot)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if !found {
		t.Fatal("expected defaults to be saved to store")
	}
	if len(snapshot.Tenants) != 1 {
		t.Fatalf("expected 1 persisted tenant, got %d", len(snapshot.Tenants))
	}
}

func TestStateRepository_LoadTenants_ExistingState(t *testing.T) {
	store := newMemStore()
	repo := NewStateRepository(store)

	// Pre-populate the store with existing data
	existing := stateSnapshot{
		Tenants: []domain.Tenant{
			{ID: "existing_1", Name: "Existing Corp", Status: "active"},
			{ID: "existing_2", Name: "Existing Factory", Status: "suspended"},
		},
	}
	if err := store.Save(stateKey, existing); err != nil {
		t.Fatalf("pre-populate store: %v", err)
	}

	// Defaults should be ignored when store already has data
	defaults := []domain.Tenant{
		{ID: "default_1", Name: "Default Ignored", Status: "active"},
	}

	loaded, err := repo.LoadTenants(defaults)
	if err != nil {
		t.Fatalf("LoadTenants: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 tenants from store, got %d", len(loaded))
	}
	if loaded[0].ID != "existing_1" {
		t.Fatalf("expected existing_1, got %s", loaded[0].ID)
	}
}

func TestStateRepository_LoadTenants_ReturnsClone(t *testing.T) {
	store := newMemStore()
	repo := NewStateRepository(store)
	defaults := []domain.Tenant{
		{ID: "t1", Name: "Original", Status: "active"},
	}

	loaded, err := repo.LoadTenants(defaults)
	if err != nil {
		t.Fatalf("LoadTenants: %v", err)
	}

	// Mutating the returned slice should not affect a subsequent load
	loaded[0].Name = "MUTATED"

	reloaded, err := repo.LoadTenants(defaults)
	if err != nil {
		t.Fatalf("second LoadTenants: %v", err)
	}
	if reloaded[0].Name == "MUTATED" {
		t.Fatal("mutation of returned slice affected stored data")
	}
}

func TestStateRepository_SaveTenants(t *testing.T) {
	store := newMemStore()
	repo := NewStateRepository(store)
	now := time.Now().UTC()

	tenants := []domain.Tenant{
		{ID: "t1", Name: "Saved Corp", Type: "factory", HQRegion: "ID-JK", Status: "active", CreatedAt: now},
		{ID: "t2", Name: "Saved Studio", Type: "studio", Status: "suspended", CreatedAt: now},
	}

	if err := repo.SaveTenants(tenants); err != nil {
		t.Fatalf("SaveTenants: %v", err)
	}

	// Verify data round-trips through the store
	var snapshot stateSnapshot
	found, err := store.Load(stateKey, &snapshot)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if !found {
		t.Fatal("expected data in store after save")
	}
	if len(snapshot.Tenants) != 2 {
		t.Fatalf("expected 2 tenants in snapshot, got %d", len(snapshot.Tenants))
	}
	if snapshot.Tenants[0].Name != "Saved Corp" {
		t.Fatalf("expected Saved Corp, got %s", snapshot.Tenants[0].Name)
	}
}

func TestStateRepository_SaveTenants_ClonesInput(t *testing.T) {
	store := newMemStore()
	repo := NewStateRepository(store)

	tenants := []domain.Tenant{
		{ID: "t1", Name: "Before", Status: "active"},
	}

	if err := repo.SaveTenants(tenants); err != nil {
		t.Fatalf("SaveTenants: %v", err)
	}

	// Mutate the input after saving -- should not affect stored data
	tenants[0].Name = "MUTATED"

	var snapshot stateSnapshot
	if _, err := store.Load(stateKey, &snapshot); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if snapshot.Tenants[0].Name == "MUTATED" {
		t.Fatal("mutation of input affected stored data")
	}
}

func TestStateRepository_NilRepository(t *testing.T) {
	var repo *StateRepository

	defaults := []domain.Tenant{
		{ID: "t1", Name: "Default", Status: "active"},
	}

	loaded, err := repo.LoadTenants(defaults)
	if err != nil {
		t.Fatalf("nil repo LoadTenants: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected defaults returned for nil repo, got %d", len(loaded))
	}

	err = repo.SaveTenants(loaded)
	if err != nil {
		t.Fatalf("nil repo SaveTenants should not error: %v", err)
	}
}

func TestStateRepository_NilStore(t *testing.T) {
	repo := NewStateRepository(nil)

	defaults := []domain.Tenant{
		{ID: "t1", Name: "Default", Status: "active"},
	}

	loaded, err := repo.LoadTenants(defaults)
	if err != nil {
		t.Fatalf("nil store LoadTenants: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected defaults returned for nil store, got %d", len(loaded))
	}

	err = repo.SaveTenants(loaded)
	if err != nil {
		t.Fatalf("nil store SaveTenants should not error: %v", err)
	}
}

func TestStateRepository_LoadError(t *testing.T) {
	loadErr := errors.New("disk read failure")
	store := &errStore{loadErr: loadErr}
	repo := NewStateRepository(store)

	_, err := repo.LoadTenants(nil)
	if err == nil {
		t.Fatal("expected error from LoadTenants when store fails")
	}
	if err != loadErr {
		t.Fatalf("expected loadErr, got %v", err)
	}
}

func TestStateRepository_SaveError_OnFirstLoad(t *testing.T) {
	saveErr := errors.New("disk write failure")
	store := &errStore{saveErr: saveErr}
	repo := NewStateRepository(store)

	// When there's no existing state, LoadTenants tries to save defaults
	_, err := repo.LoadTenants([]domain.Tenant{{ID: "t1"}})
	if err == nil {
		t.Fatal("expected error when initial save fails")
	}
	if err != saveErr {
		t.Fatalf("expected saveErr, got %v", err)
	}
}

func TestStateRepository_SaveError_OnSave(t *testing.T) {
	saveErr := errors.New("disk write failure")
	store := &errStore{saveErr: saveErr}
	repo := NewStateRepository(store)

	err := repo.SaveTenants([]domain.Tenant{{ID: "t1"}})
	if err == nil {
		t.Fatal("expected error from SaveTenants when store fails")
	}
	if err != saveErr {
		t.Fatalf("expected saveErr, got %v", err)
	}
}
