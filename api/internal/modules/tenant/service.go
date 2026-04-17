package tenant

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrTenantNameRequired = errors.New("tenant name is required")
var ErrTenantNotFound = errors.New("tenant not found")
var ErrInvalidTenantStatus = errors.New("invalid tenant status")
var ErrInvalidTenantType = errors.New("invalid tenant type")

type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	HQRegion  string    `json:"hq_region,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_tenant"

type stateSnapshot struct {
	Tenants []Tenant `json:"tenants"`
}

type Service struct {
	mu         sync.RWMutex
	tenants    []Tenant
	stateStore StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		tenants: []Tenant{
			{
				ID:        "tenant_demo_jakarta",
				Name:      "MistyPass Jakarta Demo",
				Type:      "company",
				HQRegion:  "ID-JK",
				Status:    "active",
				CreatedAt: now,
			},
			{
				ID:        "tenant_demo_factory",
				Name:      "Nusantara Manufacturing",
				Type:      "factory",
				HQRegion:  "ID-BT",
				Status:    "active",
				CreatedAt: now,
			},
		},
	}
}

func NewServiceWithStateStore(store StateStore) (*Service, error) {
	svc := NewService()
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) List() []Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Tenant, len(s.tenants))
	copy(items, s.tenants)
	return items
}

func (s *Service) Create(name, tenantType, hqRegion string) (Tenant, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return Tenant{}, ErrTenantNameRequired
	}

	nextType, err := normalizeTenantType(tenantType)
	if err != nil {
		return Tenant{}, err
	}

	id, err := tenantID()
	if err != nil {
		return Tenant{}, err
	}

	record := Tenant{
		ID:        id,
		Name:      trimmed,
		Type:      nextType,
		HQRegion:  strings.TrimSpace(hqRegion),
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	s.tenants = append(s.tenants, record)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Tenant{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateStatus(tenantID, status string) (Tenant, error) {
	id := strings.TrimSpace(tenantID)
	if id == "" {
		return Tenant{}, ErrTenantNotFound
	}

	nextStatus, err := normalizeStatus(status)
	if err != nil {
		return Tenant{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tenants {
		if s.tenants[i].ID != id {
			continue
		}

		s.tenants[i].Status = nextStatus
		if err := s.persistLocked(); err != nil {
			return Tenant{}, err
		}
		return s.tenants[i], nil
	}

	return Tenant{}, ErrTenantNotFound
}

func (s *Service) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return err
	}
	if !found {
		return s.stateStore.Save(stateKey, stateSnapshot{
			Tenants: cloneTenants(s.tenants),
		})
	}

	s.mu.Lock()
	s.tenants = cloneTenants(snapshot.Tenants)
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Tenants: cloneTenants(s.tenants),
	})
}

func cloneTenants(items []Tenant) []Tenant {
	output := make([]Tenant, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func tenantID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "tenant_" + hex.EncodeToString(raw), nil
}

func normalizeStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return "active", nil
	case "suspended":
		return "suspended", nil
	case "inactive":
		return "inactive", nil
	default:
		return "", ErrInvalidTenantStatus
	}
}

func normalizeTenantType(tenantType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(tenantType)) {
	case "", "company":
		return "company", nil
	case "studio":
		return "studio", nil
	case "government":
		return "government", nil
	case "factory":
		return "factory", nil
	case "public_facility":
		return "public_facility", nil
	default:
		return "", ErrInvalidTenantType
	}
}
