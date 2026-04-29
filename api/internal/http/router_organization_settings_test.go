package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestOrganizationSettingsGetDefault(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "org-settings-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/organization/settings?tenant_id=tenant_demo_jakarta", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var settings struct {
		TenantID           string `json:"tenant_id"`
		Name               string `json:"name"`
		Timezone           string `json:"timezone"`
		EmailNotifications bool   `json:"email_notifications"`
		EnforceMFA         bool   `json:"enforce_mfa"`
		PasswordPolicy     string `json:"password_policy"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &settings)
	if settings.TenantID != "tenant_demo_jakarta" {
		t.Errorf("expected tenant_demo_jakarta, got %s", settings.TenantID)
	}
	if settings.Name != "Mistyislet" {
		t.Errorf("expected default name=Mistyislet, got %s", settings.Name)
	}
	if settings.Timezone != "Asia/Jakarta" {
		t.Errorf("expected default timezone, got %s", settings.Timezone)
	}
	if !settings.EmailNotifications {
		t.Errorf("expected email notifications enabled by default")
	}
	if settings.PasswordPolicy != "standard" {
		t.Errorf("expected standard password policy, got %s", settings.PasswordPolicy)
	}
}

func TestOrganizationSettingsUpdate(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "org-settings-update-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// update some fields
	updateBody := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Mistyislet Jakarta","timezone":"Asia/Singapore","enforce_mfa":true,"password_policy":"strict","session_timeout_minutes":120}`)
	updateRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/organization/settings", token, updateBody)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}
	var updated struct {
		Name                  string `json:"name"`
		Timezone              string `json:"timezone"`
		EnforceMFA            bool   `json:"enforce_mfa"`
		PasswordPolicy        string `json:"password_policy"`
		SessionTimeoutMinutes int    `json:"session_timeout_minutes"`
		EmailNotifications    bool   `json:"email_notifications"`
	}
	_ = json.Unmarshal(updateRec.Body.Bytes(), &updated)
	if updated.Name != "Mistyislet Jakarta" {
		t.Errorf("expected name=Mistyislet Jakarta, got %s", updated.Name)
	}
	if updated.Timezone != "Asia/Singapore" {
		t.Errorf("expected timezone=Asia/Singapore, got %s", updated.Timezone)
	}
	if !updated.EnforceMFA {
		t.Errorf("expected enforce_mfa=true")
	}
	if updated.PasswordPolicy != "strict" {
		t.Errorf("expected password_policy=strict, got %s", updated.PasswordPolicy)
	}
	if updated.SessionTimeoutMinutes != 120 {
		t.Errorf("expected session_timeout=120, got %d", updated.SessionTimeoutMinutes)
	}
	if !updated.EmailNotifications {
		t.Errorf("expected email_notifications still true (not patched)")
	}

	// verify persistence via GET
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/organization/settings?tenant_id=tenant_demo_jakarta", token, nil)
	var persisted struct {
		Name       string `json:"name"`
		EnforceMFA bool   `json:"enforce_mfa"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &persisted)
	if persisted.Name != "Mistyislet Jakarta" {
		t.Errorf("expected persisted name=Mistyislet Jakarta, got %s", persisted.Name)
	}

	// check audit
	assertReferenceAuditLog(t, router, token, "organization_settings_updated", "name=Mistyislet Jakarta")
}

func TestOrganizationAdvancedOperations(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "org-advanced-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// export audit
	exportRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/organization/export-audit?tenant_id=tenant_demo_jakarta", token, nil)
	if exportRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", exportRec.Code, exportRec.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "organization_audit_export_requested", "")
}
