package httpx

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

// The SP-initiated OIDC callback issues a nonce in the authorize request and
// stores it on the auth-state token. The returned ID token's nonce claim must
// match, otherwise a captured/injected ID token could be replayed at the
// callback. A mismatching nonce must be rejected before a session is issued.
func TestEnterpriseOIDCCallbackRejectsNonceMismatch(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "nonce.mismatch@" + emailDomain
	externalID := "sub-nonce-mismatch-001"

	// Seed an active employee so the flow would otherwise succeed (200);
	// the nonce mismatch must be the deciding factor.
	if _, err := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{{
			ExternalID: externalID,
			Email:      email,
			FullName:   "Nonce Mismatch",
			Department: "IT",
			Location:   "Jakarta",
			Status:     "active",
		}},
	); err != nil {
		t.Fatalf("seed active employee: %v", err)
	}

	stateToken, err := s.enterpriseSvc.StartAuthStateToken(
		"tenant_demo_jakarta",
		"oidc",
		email,
		"https://admin.mistypass.local/enterprise/callback",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("start auth state token: %v", err)
	}

	// ID token carries a nonce the server never issued.
	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(
		t, signingKey, issuerURL, clientID, externalID, email,
		map[string]any{"nonce": "attacker-controlled-nonce"},
	)
	rawURL := fmt.Sprintf(
		"/api/v1/enterprise/auth/oidc/callback?state=%s&id_token=%s",
		url.QueryEscape(stateToken.Token),
		url.QueryEscape(idToken),
	)
	request := httptest.NewRequest(http.MethodGet, rawURL, nil)
	recorder := httptest.NewRecorder()

	s.enterpriseOIDCCallback(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for nonce mismatch, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
