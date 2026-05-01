package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestPasswordResetFlow(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "pw-reset-test",
		EnableDemoUsers: true,
		AppEnv:          "development",
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	// Request password reset
	reqBody := []byte(`{"email":"organization.admin@mistypass.local"}`)
	reqRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/password-reset/request", "", reqBody)
	if reqRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", reqRec.Code, reqRec.Body.String())
	}

	var reqResp struct {
		Status     string `json:"status"`
		ResetToken string `json:"reset_token"`
	}
	_ = json.Unmarshal(reqRec.Body.Bytes(), &reqResp)
	if reqResp.ResetToken == "" {
		t.Fatalf("expected reset_token in dev mode response")
	}

	// Confirm with weak password → rejected
	weakBody, _ := json.Marshal(map[string]string{
		"token":        reqResp.ResetToken,
		"new_password": "short",
	})
	weakRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/password-reset/confirm", "", weakBody)
	if weakRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for weak password, got %d body=%s", weakRec.Code, weakRec.Body.String())
	}

	// Need a new token since the previous one was consumed
	reqRec2 := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/password-reset/request", "", reqBody)
	var reqResp2 struct {
		ResetToken string `json:"reset_token"`
	}
	_ = json.Unmarshal(reqRec2.Body.Bytes(), &reqResp2)

	// Confirm with valid password
	confirmBody, _ := json.Marshal(map[string]string{
		"token":        reqResp2.ResetToken,
		"new_password": "NewPass123",
	})
	confirmRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/password-reset/confirm", "", confirmBody)
	if confirmRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", confirmRec.Code, confirmRec.Body.String())
	}

	// Login with new password should work
	loginBody := []byte(`{"email":"organization.admin@mistypass.local","password":"NewPass123"}`)
	loginRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login success with new password, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	// Old password should fail
	oldLoginBody := []byte(`{"email":"organization.admin@mistypass.local","password":"admin123"}`)
	oldLoginRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", oldLoginBody)
	if oldLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with old password, got %d", oldLoginRec.Code)
	}
}

func TestPasswordResetInvalidToken(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "pw-reset-invalid-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	confirmBody := []byte(`{"token":"invalid_token","new_password":"NewPass123"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/password-reset/confirm", "", confirmBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPasswordResetNonexistentEmail(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "pw-reset-nouser-test",
		EnableDemoUsers: true,
		AppEnv:          "development",
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	// Request for nonexistent user → still 200 (no email enumeration)
	reqBody := []byte(`{"email":"nonexistent@example.com"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/password-reset/request", "", reqBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Status     string `json:"status"`
		ResetToken string `json:"reset_token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ResetToken != "" {
		t.Errorf("should not return token for nonexistent user")
	}
}
