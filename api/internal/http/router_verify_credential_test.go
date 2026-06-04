package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestVerifyCredentialNFCUID(t *testing.T) {
	router, _, err := NewRouter(config.Config{
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
	router, _, err := NewRouter(config.Config{
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
	router, _, err := NewRouter(config.Config{
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

// TestVerifyCredentialGuestQRDoorIDs verifies a guest's QR token resolves at the
// gate and is gated by the guest's door_ids: an in-list door is allowed, an
// out-of-list door is denied.
func TestVerifyCredentialGuestQRDoorIDs(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "verify-guest-qr-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Create a guest restricted to a single door.
	createBody := []byte(`{
		"tenant_id": "tenant_demo_jakarta",
		"building_id": "building_demo_001",
		"name": "QR Guest",
		"phone": "+628120001",
		"host_name": "Host",
		"door_ids": ["door_jkt_001"]
	}`)
	createRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests", token, createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create guest: %d body=%s", createRec.Code, createRec.Body.String())
	}
	var guest struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &guest)
	if guest.AccessToken == "" {
		t.Fatalf("expected guest access_token, body=%s", createRec.Body.String())
	}

	verify := func(lockID string) (string, string) {
		body := []byte(`{"tenant_id":"tenant_demo_jakarta","lock_id":"` + lockID +
			`","credential_type":"qr_code","credential_data":"` + guest.AccessToken + `"}`)
		rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/verify-credential", token, body)
		if rec.Code != http.StatusOK {
			t.Fatalf("verify %s: %d body=%s", lockID, rec.Code, rec.Body.String())
		}
		var r struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &r)
		return r.Decision, r.Reason
	}

	if d, reason := verify("door_jkt_001"); d != "allow" {
		t.Errorf("expected allow for in-list door, got %s/%s", d, reason)
	}
	if d, reason := verify("door_jkt_999"); d != "deny" || reason != "door_not_in_guest_access" {
		t.Errorf("expected deny/door_not_in_guest_access for out-of-list door, got %s/%s", d, reason)
	}
}
