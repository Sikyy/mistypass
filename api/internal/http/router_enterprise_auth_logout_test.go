package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func TestEnterpriseAuthLogoutRevokesAccessAndRefresh(t *testing.T) {
	authSvc := auth.NewService("", "", 0, 0, true)
	login, err := authSvc.Login(auth.LoginRequest{
		Email:    "tenant.admin@sudirman.co",
		Password: "admin123",
	})
	if err != nil {
		t.Fatalf("expected login success: %v", err)
	}

	s := &server{authService: authSvc}
	body, _ := json.Marshal(map[string]string{
		"access_token":  login.AccessToken,
		"refresh_token": login.RefreshToken,
		"tenant_id":     "tenant_demo_jakarta",
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/logout", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	s.enterpriseAuthLogout(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		RevokedAccess  bool   `json:"revoked_access"`
		RevokedRefresh bool   `json:"revoked_refresh"`
		TenantID       string `json:"tenant_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.RevokedAccess || !response.RevokedRefresh {
		t.Fatalf("expected both access and refresh revoked, got %+v", response)
	}
	if response.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant_id: %s", response.TenantID)
	}

	if _, err := authSvc.VerifyAccessToken(login.AccessToken); err != auth.ErrInvalidAccessToken {
		t.Fatalf("expected revoked access token to be invalid, got: %v", err)
	}
	if _, err := authSvc.Refresh(auth.RefreshRequest{RefreshToken: login.RefreshToken}); err != auth.ErrInvalidRefreshToken {
		t.Fatalf("expected revoked refresh token to be invalid, got: %v", err)
	}
}

func TestEnterpriseAuthLogoutTenantMismatch(t *testing.T) {
	authSvc := auth.NewService("", "", 0, 0, true)
	login, err := authSvc.Login(auth.LoginRequest{
		Email:    "tenant.admin@sudirman.co",
		Password: "admin123",
	})
	if err != nil {
		t.Fatalf("expected login success: %v", err)
	}

	s := &server{authService: authSvc}
	body, _ := json.Marshal(map[string]string{
		"access_token": login.AccessToken,
		"tenant_id":    "tenant_demo_factory",
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/logout", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	s.enterpriseAuthLogout(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestEnterpriseAuthLogoutTenantValidationRequiresAccessToken(t *testing.T) {
	authSvc := auth.NewService("", "", 0, 0, true)
	login, err := authSvc.Login(auth.LoginRequest{
		Email:    "tenant.admin@sudirman.co",
		Password: "admin123",
	})
	if err != nil {
		t.Fatalf("expected login success: %v", err)
	}

	s := &server{authService: authSvc}
	body, _ := json.Marshal(map[string]string{
		"refresh_token": login.RefreshToken,
		"tenant_id":     "tenant_demo_jakarta",
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/logout", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	s.enterpriseAuthLogout(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestEnterpriseAuthLogoutRefreshOnly(t *testing.T) {
	authSvc := auth.NewService("", "", 0, 0, true)
	login, err := authSvc.Login(auth.LoginRequest{
		Email:    "tenant.admin@sudirman.co",
		Password: "admin123",
	})
	if err != nil {
		t.Fatalf("expected login success: %v", err)
	}

	s := &server{authService: authSvc}
	body, _ := json.Marshal(map[string]string{
		"refresh_token": login.RefreshToken,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/enterprise/auth/logout", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	s.enterpriseAuthLogout(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		RevokedAccess  bool `json:"revoked_access"`
		RevokedRefresh bool `json:"revoked_refresh"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.RevokedAccess {
		t.Fatalf("expected access token to remain untouched")
	}
	if !response.RevokedRefresh {
		t.Fatalf("expected refresh token to be revoked")
	}
	if _, err := authSvc.VerifyAccessToken(login.AccessToken); err != nil {
		t.Fatalf("expected access token to remain valid, got: %v", err)
	}
	if _, err := authSvc.Refresh(auth.RefreshRequest{RefreshToken: login.RefreshToken}); err != auth.ErrInvalidRefreshToken {
		t.Fatalf("expected refresh token to be invalid after revoke, got: %v", err)
	}
}
