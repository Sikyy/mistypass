package application

import (
	"strings"
	"sync"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/tenant/domain"
)

type Repository interface {
	LoadTenants(defaults []domain.Tenant) ([]domain.Tenant, error)
	SaveTenants(items []domain.Tenant) error
}

type Service struct {
	mu      sync.RWMutex
	repo    Repository
	idGen   domain.IDGenerator
	nowFunc func() time.Time
	tenants []domain.Tenant
}

func NewService(
	repo Repository,
	defaults []domain.Tenant,
	idGen domain.IDGenerator,
) (*Service, error) {
	nextIDGen := idGen
	if nextIDGen == nil {
		nextIDGen = domain.DefaultIDGenerator
	}
	nextNow := func() time.Time {
		return time.Now().UTC()
	}

	items := domain.CloneTenants(defaults)
	if repo != nil {
		loaded, err := repo.LoadTenants(defaults)
		if err != nil {
			return nil, err
		}
		items = domain.CloneTenants(loaded)
	}

	return &Service{
		repo:    repo,
		idGen:   nextIDGen,
		nowFunc: nextNow,
		tenants: items,
	}, nil
}

func (s *Service) List() []domain.Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return domain.CloneTenants(s.tenants)
}

func (s *Service) Create(name, tenantType, hqRegion string) (domain.Tenant, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return domain.Tenant{}, domain.ErrTenantNameRequired
	}
	nextType, err := domain.NormalizeType(tenantType)
	if err != nil {
		return domain.Tenant{}, err
	}
	id, err := s.idGen()
	if err != nil {
		return domain.Tenant{}, err
	}
	record := domain.Tenant{
		ID:        id,
		Name:      trimmedName,
		Type:      nextType,
		HQRegion:  strings.TrimSpace(hqRegion),
		Status:    "active",
		CreatedAt: s.nowFunc(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants = append(s.tenants, record)
	if err := s.persistLocked(); err != nil {
		return domain.Tenant{}, err
	}
	return record, nil
}

func (s *Service) UpdateStatus(tenantID, status string) (domain.Tenant, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return domain.Tenant{}, domain.ErrTenantNotFound
	}
	nextStatus, err := domain.NormalizeStatus(status)
	if err != nil {
		return domain.Tenant{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tenants {
		if s.tenants[i].ID != nextTenantID {
			continue
		}
		s.tenants[i].Status = nextStatus
		if err := s.persistLocked(); err != nil {
			return domain.Tenant{}, err
		}
		return s.tenants[i], nil
	}
	return domain.Tenant{}, domain.ErrTenantNotFound
}

func (s *Service) persistLocked() error {
	if s.repo == nil {
		return nil
	}
	return s.repo.SaveTenants(s.tenants)
}
