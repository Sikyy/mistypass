package tenant

import (
	"time"

	"github.com/mistypass/cloud/api/internal/modules/tenant/application"
	"github.com/mistypass/cloud/api/internal/modules/tenant/delivery"
	"github.com/mistypass/cloud/api/internal/modules/tenant/domain"
	"github.com/mistypass/cloud/api/internal/modules/tenant/infrastructure"
)

var ErrTenantNameRequired = domain.ErrTenantNameRequired
var ErrTenantNotFound = domain.ErrTenantNotFound
var ErrInvalidTenantStatus = domain.ErrInvalidTenantStatus
var ErrInvalidTenantType = domain.ErrInvalidTenantType

type Tenant = domain.Tenant

type StateStore = infrastructure.StateStore

type Service struct {
	facade *delivery.Facade
}

func NewService() *Service {
	appService, _ := application.NewService(
		nil,
		defaultTenants(),
		domain.DefaultIDGenerator,
	)
	return &Service{
		facade: delivery.NewFacade(appService),
	}
}

func NewServiceWithStateStore(store StateStore) (*Service, error) {
	repo := infrastructure.NewStateRepository(store)
	appService, err := application.NewService(
		repo,
		defaultTenants(),
		domain.DefaultIDGenerator,
	)
	if err != nil {
		return nil, err
	}
	return &Service{
		facade: delivery.NewFacade(appService),
	}, nil
}

func (s *Service) List() []Tenant {
	if s == nil || s.facade == nil {
		return nil
	}
	return s.facade.List()
}

func (s *Service) Create(name, tenantType, hqRegion string) (Tenant, error) {
	if s == nil || s.facade == nil {
		return Tenant{}, ErrTenantNotFound
	}
	return s.facade.Create(name, tenantType, hqRegion)
}

func (s *Service) UpdateStatus(tenantID, status string) (Tenant, error) {
	if s == nil || s.facade == nil {
		return Tenant{}, ErrTenantNotFound
	}
	return s.facade.UpdateStatus(tenantID, status)
}

func defaultTenants() []domain.Tenant {
	now := time.Now().UTC()
	return []domain.Tenant{
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
	}
}
