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

func TestEnterpriseOIDCCallbackJITInactiveReturnsForbidden(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "jit.callback.inactive@" + emailDomain
	externalID := "sub-jit-callback-inactive-001"

	_, err := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: externalID,
				Email:      email,
				FullName:   "JIT Callback Inactive",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "inactive",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed inactive employee success: %v", err)
	}

	stateToken, err := s.enterpriseSvc.StartAuthStateToken(
		"tenant_demo_jakarta",
		"oidc",
		email,
		"https://admin.mistypass.local/enterprise/callback",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("expected start auth state token success: %v", err)
	}

	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(t, signingKey, issuerURL, clientID, externalID, email, map[string]any{"nonce": stateToken.Nonce})
	rawURL := fmt.Sprintf(
		"/api/v1/enterprise/auth/oidc/callback?state=%s&id_token=%s",
		url.QueryEscape(stateToken.Token),
		url.QueryEscape(idToken),
	)
	request := httptest.NewRequest(http.MethodGet, rawURL, nil)
	recorder := httptest.NewRecorder()

	s.enterpriseOIDCCallback(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeInactive.Error())
}

func TestEnterpriseOIDCCallbackJITExternalIDConflictReturnsConflict(t *testing.T) {
	s, issuerURL, clientID, emailDomain, signingKey := newEnterpriseJITOIDCTestServer(t)
	email := "jit.callback.conflict@" + emailDomain

	_, err := s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"manual_sync",
		"qa",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID: "sub-jit-callback-conflict-a",
				Email:      email,
				FullName:   "JIT Callback Conflict",
				Department: "IT",
				Location:   "Jakarta",
				Status:     "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed active employee success: %v", err)
	}

	stateToken, err := s.enterpriseSvc.StartAuthStateToken(
		"tenant_demo_jakarta",
		"oidc",
		email,
		"https://admin.mistypass.local/enterprise/callback",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("expected start auth state token success: %v", err)
	}

	idToken := mustBuildSignedOIDCIDTokenWithExtraClaims(t, signingKey, issuerURL, clientID, "sub-jit-callback-conflict-b", email, map[string]any{"nonce": stateToken.Nonce})
	rawURL := fmt.Sprintf(
		"/api/v1/enterprise/auth/oidc/callback?state=%s&id_token=%s",
		url.QueryEscape(stateToken.Token),
		url.QueryEscape(idToken),
	)
	request := httptest.NewRequest(http.MethodGet, rawURL, nil)
	recorder := httptest.NewRecorder()

	s.enterpriseOIDCCallback(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertResponseError(t, recorder, enterprise.ErrEmployeeExternalIDConflict.Error())
}
