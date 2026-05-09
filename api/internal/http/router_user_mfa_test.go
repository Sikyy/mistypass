package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestUserMFAFullFlow(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "user-mfa-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	// use org admin (who has building_admin-like access via demo)
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// check initial status
	statusRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/auth/mfa/user/status", token, nil)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", statusRec.Code, statusRec.Body.String())
	}
	var status struct {
		Enabled bool `json:"enabled"`
		Pending bool `json:"pending"`
	}
	_ = json.Unmarshal(statusRec.Body.Bytes(), &status)
	if status.Enabled {
		t.Errorf("expected MFA not enabled initially")
	}

	// setup
	setupRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/mfa/user/setup", token, []byte(`{"issuer":"MistyPassTest"}`))
	if setupRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", setupRec.Code, setupRec.Body.String())
	}
	var enrollment struct {
		Secret     string `json:"secret"`
		OTPAuthURL string `json:"otpauth_url"`
	}
	_ = json.Unmarshal(setupRec.Body.Bytes(), &enrollment)
	if enrollment.Secret == "" {
		t.Fatalf("expected a TOTP secret")
	}
	if enrollment.OTPAuthURL == "" {
		t.Fatalf("expected an OTP auth URL")
	}

	// check pending status
	statusRec2 := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/auth/mfa/user/status", token, nil)
	_ = json.Unmarshal(statusRec2.Body.Bytes(), &status)
	if !status.Pending {
		t.Errorf("expected pending after setup")
	}

	// enable with wrong code
	wrongRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/mfa/user/enable", token, []byte(`{"code":"000000"}`))
	if wrongRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong code, got %d", wrongRec.Code)
	}

	// disable before enable should still work (requires re-auth)
	disableRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/mfa/user/disable", token, []byte(`{"password":"admin123"}`))
	if disableRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on disable, got %d body=%s", disableRec.Code, disableRec.Body.String())
	}
	var disabled struct{ Enabled bool `json:"enabled"` }
	_ = json.Unmarshal(disableRec.Body.Bytes(), &disabled)
	if disabled.Enabled {
		t.Errorf("expected disabled")
	}

	// check audit logs
	assertReferenceAuditLog(t, router, token, "user_mfa_setup_started", "user_id=")
	assertReferenceAuditLog(t, router, token, "user_mfa_disabled", "user_id=")
}

func TestUserMFAAvailableForNonAdmin(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "user-mfa-nonadmin-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	// place admin (building_admin) should also be able to use user MFA
	token := referenceAPILogin(t, router, "place.admin.sudirman@mistypass.local")

	statusRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/auth/mfa/user/status", token, nil)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for non-admin user MFA status, got %d body=%s", statusRec.Code, statusRec.Body.String())
	}

	setupRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/mfa/user/setup", token, []byte(`{}`))
	if setupRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for non-admin user MFA setup, got %d body=%s", setupRec.Code, setupRec.Body.String())
	}
}
