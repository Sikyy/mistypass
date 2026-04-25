package infrastructure

import (
	"github.com/mistypass/cloud/api/internal/modules/tenant/domain"
)

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_tenant"

type stateSnapshot struct {
	Tenants []domain.Tenant `json:"tenants"`
}

type StateRepository struct {
	store StateStore
}

func NewStateRepository(store StateStore) *StateRepository {
	return &StateRepository{store: store}
}

func (r *StateRepository) LoadTenants(defaults []domain.Tenant) ([]domain.Tenant, error) {
	if r == nil || r.store == nil {
		return domain.CloneTenants(defaults), nil
	}

	var snapshot stateSnapshot
	found, err := r.store.Load(stateKey, &snapshot)
	if err != nil {
		return nil, err
	}
	if !found {
		cloned := domain.CloneTenants(defaults)
		if err := r.store.Save(stateKey, stateSnapshot{Tenants: cloned}); err != nil {
			return nil, err
		}
		return cloned, nil
	}

	return domain.CloneTenants(snapshot.Tenants), nil
}

func (r *StateRepository) SaveTenants(items []domain.Tenant) error {
	if r == nil || r.store == nil {
		return nil
	}
	return r.store.Save(stateKey, stateSnapshot{
		Tenants: domain.CloneTenants(items),
	})
}
