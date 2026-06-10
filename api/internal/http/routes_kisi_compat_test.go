package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestKisiOrgPublicLookup(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "kisi-org-public-lookup-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	// Kisi-shape lookup by tenant identifier works without authentication.
	byTenantID := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/organizations/tenant_demo_jakarta/public", "", nil)
	if byTenantID.Code != http.StatusOK {
		t.Fatalf("expected org public lookup 200, got %d body=%s", byTenantID.Code, byTenantID.Body.String())
	}
	var kisiOrg struct {
		ID             int    `json:"id"`
		Domain         string `json:"domain"`
		SSOFlowEnabled *bool  `json:"sso_flow_enabled"`
	}
	if err := json.Unmarshal(byTenantID.Body.Bytes(), &kisiOrg); err != nil {
		t.Fatalf("expected Kisi-shaped org payload (numeric id): %v body=%s", err, byTenantID.Body.String())
	}
	if kisiOrg.ID == 0 || kisiOrg.SSOFlowEnabled == nil {
		t.Fatalf("expected Kisi-shaped org payload, got %s", byTenantID.Body.String())
	}

	// Lookup by the organization's configured primary domain must resolve too —
	// this was only handled by the legacy handler that the Kisi-compat
	// registration shadowed.
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	settingsBody := []byte(`{"tenant_id":"tenant_demo_jakarta","primary_domain":"Acme-Park.example.com"}`)
	settingsRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/organization/settings", token, settingsBody)
	if settingsRecorder.Code != http.StatusOK {
		t.Fatalf("expected organization settings update 200, got %d body=%s", settingsRecorder.Code, settingsRecorder.Body.String())
	}

	byDomain := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/organizations/acme-park.example.com/public", "", nil)
	if byDomain.Code != http.StatusOK {
		t.Fatalf("expected org public lookup by primary domain 200, got %d body=%s", byDomain.Code, byDomain.Body.String())
	}
	var kisiOrgByDomain struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(byDomain.Body.Bytes(), &kisiOrgByDomain); err != nil {
		t.Fatalf("decode org by domain: %v body=%s", err, byDomain.Body.String())
	}
	if kisiOrgByDomain.ID != kisiOrg.ID {
		t.Fatalf("expected domain lookup to resolve the same org, got %d vs %d", kisiOrgByDomain.ID, kisiOrg.ID)
	}
}
