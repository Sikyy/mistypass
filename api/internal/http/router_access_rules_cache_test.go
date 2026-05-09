package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestGatewayAccessRulesPreview(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "access-rules-preview-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Preview access rules for gw_demo_001 (bound to door_jkt_001, door_jkt_014)
	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/gateways/gw_demo_001/access-rules?tenant_id=tenant_demo_jakarta", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var result struct {
		GatewayID    string   `json:"gateway_id"`
		BoundDoorIDs []string `json:"bound_door_ids"`
		AccessRules  []struct {
			CredentialType string   `json:"credential_type"`
			CredentialData string   `json:"credential_data"`
			UserID         string   `json:"user_id"`
			UserEmail      string   `json:"user_email"`
			LockIDs        []string `json:"lock_ids"`
		} `json:"access_rules"`
		Counts struct {
			AccessRules int `json:"access_rules"`
			Users       int `json:"users"`
			Doors       int `json:"doors"`
		} `json:"counts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if result.GatewayID != "gw_demo_001" {
		t.Errorf("expected gw_demo_001, got %s", result.GatewayID)
	}
	if len(result.BoundDoorIDs) < 2 {
		t.Errorf("expected at least 2 bound doors, got %d: %v", len(result.BoundDoorIDs), result.BoundDoorIDs)
	}

	// Should have NFC UID-1001 → usr_1001 access rule
	foundUID := false
	for _, rule := range result.AccessRules {
		if rule.CredentialType == "nfc_uid" && rule.CredentialData == "UID-1001" {
			foundUID = true
			if rule.UserID != "usr_1001" {
				t.Errorf("expected usr_1001, got %s", rule.UserID)
			}
			if len(rule.LockIDs) == 0 {
				t.Errorf("expected lock_ids for UID-1001")
			}
			hasDoor := false
			for _, id := range rule.LockIDs {
				if id == "door_jkt_001" || id == "door_jkt_014" {
					hasDoor = true
				}
			}
			if !hasDoor {
				t.Errorf("expected door_jkt_001 or door_jkt_014 in lock_ids, got %v", rule.LockIDs)
			}
		}
	}
	if !foundUID {
		t.Errorf("expected access rule for NFC UID-1001, got %d rules", len(result.AccessRules))
	}

	if result.Counts.AccessRules != len(result.AccessRules) {
		t.Errorf("counts mismatch: %d vs %d", result.Counts.AccessRules, len(result.AccessRules))
	}
}

func TestGatewayAccessRulesUnknownGateway(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "access-rules-unknown-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/gateways/gw_nonexistent/access-rules?tenant_id=tenant_demo_jakarta", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
