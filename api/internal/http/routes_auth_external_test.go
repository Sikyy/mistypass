package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func TestExternalLoginDisabled(t *testing.T) {
	s := &server{
		cfg:         config.Config{ExternalAuthEnabled: false},
		authService: auth.NewService("", "", 0, 0, true),
	}

	requestBytes, _ := json.Marshal(map[string]any{
		"access_token": "ext-token-demo",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/external/login", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.externalLogin(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	}
}

func TestExternalLoginSuccess(t *testing.T) {
	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer ext-valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid token"}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"sub":"casdoor-user-001",
			"email":"external.admin@sudirman.co",
			"role":"tenant_admin",
			"tenant_id":"tenant_demo_jakarta"
		}`))
	}))
	defer userInfoServer.Close()

	s := &server{
		cfg: config.Config{
			ExternalAuthEnabled:     true,
			ExternalAuthProvider:    "casdoor",
			ExternalAuthUserInfoURL: userInfoServer.URL,
			ExternalAuthDefaultRole: "resident",
		},
		authService:            auth.NewService("", "", 0, 0, true),
		externalAuthHTTPClient: userInfoServer.Client(),
	}

	requestBytes, _ := json.Marshal(map[string]any{
		"access_token": "ext-valid-token",
		"provider":     "casdoor",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/external/login", bytes.NewReader(requestBytes))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	s.externalLogin(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var payload auth.LoginResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.User.Email != "external.admin@sudirman.co" {
		t.Fatalf("unexpected login user email: %s", payload.User.Email)
	}
	if payload.User.Role != "tenant_admin" {
		t.Fatalf("unexpected login user role: %s", payload.User.Role)
	}
	if payload.User.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected login user tenant id: %s", payload.User.TenantID)
	}
}
