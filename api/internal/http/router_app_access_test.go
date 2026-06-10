package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestAppUnlockDoor(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-unlock-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	// unlock door_jkt_001 — the resident's access is via the geofenced
	// "Common Office Access" group, so coordinates within its radius are required.
	body := []byte(`{"lock_id":"door_jkt_001","latitude":-6.2088,"longitude":106.8456}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/access/unlock", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Decision  string `json:"decision"`
		Reason    string `json:"reason"`
		Status    string `json:"status"`
		LockID    string `json:"lock_id"`
		LockName  string `json:"lock_name"`
		GroupName string `json:"group_name"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Decision != "allow" {
		t.Errorf("expected allow, got %s reason=%s", result.Decision, result.Reason)
	}
	if result.LockID != "door_jkt_001" {
		t.Errorf("expected door_jkt_001, got %s", result.LockID)
	}
	if result.LockName == "" {
		t.Errorf("expected lock name")
	}
}

func TestAppUnlockDoorNoAccess(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-unlock-deny-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	// try to unlock a door that doesn't exist in user's tenant
	body := []byte(`{"lock_id":"door_fct_029"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/access/unlock", token, body)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("expected 404 or 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAppQRUnlock(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-qr-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	// use demo group link secret
	body := []byte(`{"lock_id":"door_jkt_001","qr_token":"gls_demo_common_office"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/access/qr-unlock", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Decision string `json:"decision"`
		LockID   string `json:"lock_id"`
		GroupID  string `json:"group_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Decision != "allow" {
		t.Errorf("expected allow, got %s", result.Decision)
	}
	if result.GroupID == "" {
		t.Errorf("expected group_id")
	}
}

func TestAppQRUnlockInvalidToken(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-qr-invalid-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	body := []byte(`{"lock_id":"door_jkt_001","qr_token":"invalid_token"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/access/qr-unlock", token, body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAppMyDoors(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-mydoors-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/access/my-doors", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Items []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			GatewayStatus string `json:"gateway_status"`
			CanUnlock     bool   `json:"can_unlock"`
			IsFavorite    bool   `json:"is_favorite"`
		} `json:"items"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Pagination.Total == 0 {
		t.Errorf("expected at least 1 accessible door")
	}
	for _, item := range result.Items {
		if item.Name == "" {
			t.Errorf("expected door name for %s", item.ID)
		}
	}
}
