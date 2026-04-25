package delivery

import (
	"github.com/mistypass/cloud/api/internal/modules/tenant/application"
	"github.com/mistypass/cloud/api/internal/modules/tenant/domain"
)

type Facade struct {
	service *application.Service
}

func NewFacade(service *application.Service) *Facade {
	return &Facade{service: service}
}

func (f *Facade) List() []domain.Tenant {
	if f == nil || f.service == nil {
		return nil
	}
	return f.service.List()
}

func (f *Facade) Create(name, tenantType, hqRegion string) (domain.Tenant, error) {
	if f == nil || f.service == nil {
		return domain.Tenant{}, domain.ErrTenantNotFound
	}
	return f.service.Create(name, tenantType, hqRegion)
}

func (f *Facade) UpdateStatus(tenantID, status string) (domain.Tenant, error) {
	if f == nil || f.service == nil {
		return domain.Tenant{}, domain.ErrTenantNotFound
	}
	return f.service.UpdateStatus(tenantID, status)
}
