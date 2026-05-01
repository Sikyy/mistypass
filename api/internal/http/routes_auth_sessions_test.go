package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func TestListLoginSessions(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "sessions-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/auth/sessions", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var sessions []auth.LoginSession
	_ = json.Unmarshal(rec.Body.Bytes(), &sessions)
	if len(sessions) == 0 {
		t.Fatalf("expected at least 1 active session after login")
	}
	if sessions[0].UserID == "" {
		t.Errorf("expected user_id in session")
	}
}

func TestRevokeLoginSession(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "sessions-revoke-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// List sessions to get a session ID
	listRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/auth/sessions", token, nil)
	var sessions []auth.LoginSession
	_ = json.Unmarshal(listRec.Body.Bytes(), &sessions)
	if len(sessions) == 0 {
		t.Fatalf("expected at least 1 session")
	}

	// Revoke specific session
	revokeBody, _ := json.Marshal(map[string]string{"session_id": sessions[0].SessionID})
	revokeRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/sessions/revoke", token, revokeBody)
	if revokeRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", revokeRec.Code, revokeRec.Body.String())
	}

	// Revoke nonexistent → 404
	badBody, _ := json.Marshal(map[string]string{"session_id": "nonexistent"})
	badRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/sessions/revoke", token, badBody)
	if badRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", badRec.Code, badRec.Body.String())
	}
}

func TestRevokeAllLoginSessions(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "sessions-revoke-all-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/sessions/revoke-all", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var result struct {
		Revoked int `json:"revoked"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Revoked == 0 {
		t.Errorf("expected at least 1 revoked session")
	}
}
