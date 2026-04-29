package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestVerifyCredentialNFCUID(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "verify-cred-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Verify with known NFC UID (UID-1001 → usr_1001 via physical card inventory → pass wps_demo_1001)
	body := []byte(`{
		"gateway_id": "gw_demo_001",
		"reader_id": "gdv_demo_001",
		"lock_id": "door_jkt_001",
		"tenant_id": "tenant_demo_jakarta",
		"credential_type": "nfc_uid",
		"credential_data": "UID-1001"
	}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/verify-credential", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Decision  string `json:"decision"`
		Reason    string `json:"reason"`
		UserID    string `json:"user_id"`
		UserEmail string `json:"user_email"`
		LockID    string `json:"lock_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Decision != "allow" {
		t.Errorf("expected allow, got decision=%s reason=%s", result.Decision, result.Reason)
	}
	if result.UserID != "usr_1001" {
		t.Errorf("expected usr_1001, got %s", result.UserID)
	}
	if result.LockID != "door_jkt_001" {
		t.Errorf("expected door_jkt_001, got %s", result.LockID)
	}
}

func TestVerifyCredentialUnknownUID(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "verify-unknown-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	body := []byte(`{
		"gateway_id": "gw_demo_001",
		"lock_id": "door_jkt_001",
		"tenant_id": "tenant_demo_jakarta",
		"credential_type": "nfc_uid",
		"credential_data": "UNKNOWN-UID-999"
	}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/verify-credential", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var result struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Decision != "deny" {
		t.Errorf("expected deny, got %s", result.Decision)
	}
	if result.Reason != "credential_not_found" {
		t.Errorf("expected credential_not_found, got %s", result.Reason)
	}
}

func TestVerifyCredentialEmptyCredential(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "verify-empty-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	body := []byte(`{
		"lock_id": "door_jkt_001",
		"tenant_id": "tenant_demo_jakarta",
		"credential_type": "nfc_uid",
		"credential_data": ""
	}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/verify-credential", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var result struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Decision != "deny" || result.Reason != "empty_credential" {
		t.Errorf("expected deny/empty_credential, got %s/%s", result.Decision, result.Reason)
	}
}
