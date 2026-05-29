package httpx

import (
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

// Disabling MFA must require valid re-authentication. A wrong password must not
// disable MFA — previously the handler accepted any password because it never
// verified the supplied credential.
func TestDisableUserMFARejectsWrongPassword(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "user-mfa-disable-reauth-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/mfa/user/disable", token, []byte(`{"password":"totally-wrong-password"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password on MFA disable, got %d body=%s", rec.Code, rec.Body.String())
	}
}
