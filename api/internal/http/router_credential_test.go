package httpx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestMobileCredentialRegistrationAndRevoke(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "credential-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	// Login as a resident (app user)
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	// Generate a test EC P-256 key pair
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))

	// Register mobile credential
	registerBody, _ := json.Marshal(map[string]any{
		"public_key_pem":  pubPEM,
		"platform":        "android",
		"device_id":       "test_device_samsung_a54",
		"device_model":    "Samsung Galaxy A54",
		"keystore_level":  "tee",
	})
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/credentials/register", token, registerBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var registerResp struct {
		Credential struct {
			ID            string `json:"id"`
			Status        string `json:"status"`
			Platform      string `json:"platform"`
			KeystoreLevel string `json:"keystore_level"`
			PublicKeyPEM  string `json:"public_key_pem"`
		} `json:"credential"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &registerResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if registerResp.Credential.ID == "" {
		t.Fatal("expected credential ID")
	}
	if registerResp.Credential.Status != "active" {
		t.Fatalf("expected active, got %s", registerResp.Credential.Status)
	}
	if registerResp.Credential.Platform != "android" {
		t.Fatalf("expected android, got %s", registerResp.Credential.Platform)
	}
	credID := registerResp.Credential.ID

	// List credentials
	listRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/credentials/mobile", token, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listResp struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	json.Unmarshal(listRec.Body.Bytes(), &listResp)
	if listResp.Total < 1 {
		t.Fatal("expected at least 1 credential in list")
	}

	// Refresh credential
	refreshRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/credentials/mobile/"+credID+"/refresh", token, nil)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on refresh, got %d: %s", refreshRec.Code, refreshRec.Body.String())
	}

	// Self-revoke
	revokeRec := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/app/credentials/mobile/"+credID, token, nil)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on revoke, got %d: %s", revokeRec.Code, revokeRec.Body.String())
	}

	// Verify revoked — refresh should fail
	refreshRec2 := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/credentials/mobile/"+credID+"/refresh", token, nil)
	if refreshRec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 after revoke, got %d", refreshRec2.Code)
	}
}

func TestMobileCredentialAdminRevoke(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "credential-admin-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	// Register as app user
	userToken := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))

	registerBody, _ := json.Marshal(map[string]any{
		"public_key_pem": pubPEM,
		"platform":       "android",
		"device_id":      "admin_test_device",
		"device_model":   "Test Phone",
		"keystore_level": "strongbox",
	})
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/credentials/register", userToken, registerBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Credential struct {
			ID string `json:"id"`
		} `json:"credential"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	credID := resp.Credential.ID

	// Admin revokes the credential
	adminToken := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	revokeRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/credentials/mobile/"+credID+"/revoke", adminToken, nil)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("admin revoke: expected 200, got %d: %s", revokeRec.Code, revokeRec.Body.String())
	}
}

func TestMobileCredentialInvalidKey(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "credential-invalid-key-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	// Try registering with invalid PEM
	registerBody, _ := json.Marshal(map[string]any{
		"public_key_pem": "not-a-valid-pem-key",
		"platform":       "android",
		"device_id":      "bad_device",
		"device_model":   "Test",
		"keystore_level": "tee",
	})
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/credentials/register", token, registerBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid key, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Ensure unused import suppression
var _ = rand.Reader
