package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func TestWebAuthnRegisterBeginRequiresOrgToggle(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:             "webauthn-test",
		EnableDemoUsers:       true,
		WebAuthnRPDisplayName: "MistyPass",
		WebAuthnRPID:          "localhost",
		WebAuthnRPOrigins:     []string{"http://localhost:5173"},
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// WebAuthn not enabled for org → should fail
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/register/begin", token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when webauthn disabled, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebAuthnRegisterBeginAfterOrgEnabled(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:             "webauthn-test2",
		EnableDemoUsers:       true,
		WebAuthnRPDisplayName: "MistyPass",
		WebAuthnRPID:          "localhost",
		WebAuthnRPOrigins:     []string{"http://localhost:5173"},
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Enable WebAuthn for org
	updateBody := []byte(`{"tenant_id":"tenant_demo_jakarta","webauthn_enabled":true}`)
	updateRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/organization/settings", token, updateBody)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	// Now register begin should return credential creation options
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/register/begin", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after enabling webauthn, got %d body=%s", rec.Code, rec.Body.String())
	}

	var options map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &options)
	if options["publicKey"] == nil {
		t.Errorf("expected publicKey in response, got %v", options)
	}
}

func TestWebAuthnLoginBeginNoCredentials(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:             "webauthn-login-test",
		EnableDemoUsers:       true,
		WebAuthnRPDisplayName: "MistyPass",
		WebAuthnRPID:          "localhost",
		WebAuthnRPOrigins:     []string{"http://localhost:5173"},
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Enable WebAuthn
	updateBody := []byte(`{"tenant_id":"tenant_demo_jakarta","webauthn_enabled":true}`)
	updateRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/organization/settings", token, updateBody)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateRec.Code)
	}

	// Login begin with no registered passkeys → 401
	loginBody := []byte(`{"email":"organization.admin@mistypass.local"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/login/begin", "", loginBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (no credentials), got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebAuthnListCredentialsEmpty(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:             "webauthn-list-test",
		EnableDemoUsers:       true,
		WebAuthnRPDisplayName: "MistyPass",
		WebAuthnRPID:          "localhost",
		WebAuthnRPOrigins:     []string{"http://localhost:5173"},
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/auth/webauthn/credentials", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var creds []auth.WebAuthnCredential
	_ = json.Unmarshal(rec.Body.Bytes(), &creds)
	if len(creds) != 0 {
		t.Errorf("expected 0 credentials, got %d", len(creds))
	}
}

func TestWebAuthnDeleteCredentialNotFound(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:             "webauthn-delete-test",
		EnableDemoUsers:       true,
		WebAuthnRPDisplayName: "MistyPass",
		WebAuthnRPID:          "localhost",
		WebAuthnRPOrigins:     []string{"http://localhost:5173"},
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/auth/webauthn/credentials/nonexistent", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebAuthnRequiresSSOConfigured(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:             "webauthn-sso-test",
		EnableDemoUsers:       true,
		WebAuthnRPDisplayName: "MistyPass",
		WebAuthnRPID:          "localhost",
		WebAuthnRPOrigins:     []string{"http://localhost:5173"},
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	// Login as factory tenant admin — this tenant has NO SSO/IdP configured
	token := referenceAPILogin(t, router, "tenant.admin@factory.local")

	// Enable webauthn for the factory tenant
	updateBody := []byte(`{"tenant_id":"tenant_demo_factory","webauthn_enabled":true}`)
	updateRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/organization/settings", token, updateBody)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	// Register should be rejected because no SSO is configured
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/webauthn/register/begin", token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (no SSO), got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebAuthnOrgSettingsToggle(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "webauthn-org-toggle",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Default: webauthn_enabled = false
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/organization/settings?tenant_id=tenant_demo_jakarta", token, nil)
	var settings struct {
		WebAuthnEnabled bool `json:"webauthn_enabled"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &settings)
	if settings.WebAuthnEnabled {
		t.Errorf("expected webauthn disabled by default")
	}

	// Enable
	updateBody := []byte(`{"tenant_id":"tenant_demo_jakarta","webauthn_enabled":true}`)
	updateRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/organization/settings", token, updateBody)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateRec.Code)
	}
	_ = json.Unmarshal(updateRec.Body.Bytes(), &settings)
	if !settings.WebAuthnEnabled {
		t.Errorf("expected webauthn enabled after toggle")
	}

	// Verify via GET
	getRec2 := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/organization/settings?tenant_id=tenant_demo_jakarta", token, nil)
	_ = json.Unmarshal(getRec2.Body.Bytes(), &settings)
	if !settings.WebAuthnEnabled {
		t.Errorf("expected webauthn enabled persisted")
	}
}
