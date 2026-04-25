package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
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

type IDGenerator func() (string, error)

func DefaultIDGenerator() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "tenant_" + hex.EncodeToString(raw), nil
}

func NormalizeStatus(status string) (string, error) {
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

func NormalizeType(tenantType string) (string, error) {
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

func CloneTenants(items []Tenant) []Tenant {
	output := make([]Tenant, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}
