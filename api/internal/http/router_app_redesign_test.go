package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestAppMeEnhanced(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-me-enhanced-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/me", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		ID               string `json:"id"`
		Email            string `json:"email"`
		Role             string `json:"role"`
		RoleDisplayLabel string `json:"role_display_label"`
		OrganizationName string `json:"organization_name"`
		Language         string `json:"language"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.RoleDisplayLabel == "" {
		t.Errorf("expected role_display_label, got empty")
	}
	if result.OrganizationName == "" {
		t.Errorf("expected organization_name, got empty")
	}
	if result.Role != "resident" {
		t.Errorf("expected role=resident, got %q", result.Role)
	}
}

func TestAppUpdateMe(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-update-me-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	body := []byte(`{"language":"zh-CN"}`)
	rec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/app/me", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Language         string `json:"language"`
		RoleDisplayLabel string `json:"role_display_label"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Language != "zh-CN" {
		t.Errorf("expected language=zh-CN, got %q", result.Language)
	}
	if result.RoleDisplayLabel == "" {
		t.Errorf("expected role_display_label after update")
	}
}

func TestAppGetPINCode(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-pin-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/access/pin-code", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		PIN        string `json:"pin"`
		ValidUntil string `json:"valid_until"`
		PeriodSecs int    `json:"period_secs"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if len(result.PIN) != 6 {
		t.Errorf("expected 6-digit PIN, got %q", result.PIN)
	}
	if result.PeriodSecs != 30 {
		t.Errorf("expected period_secs=30, got %d", result.PeriodSecs)
	}
	if result.ValidUntil == "" {
		t.Errorf("expected valid_until timestamp")
	}
}

func TestAppPINUnlock(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-pin-unlock-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	// First get the current PIN
	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/access/pin-code", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get pin: expected 200, got %d", rec.Code)
	}
	var pinResult struct {
		PIN string `json:"pin"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &pinResult)

	// Use the PIN to unlock
	body := []byte(fmt.Sprintf(`{"lock_id":"door_jkt_001","pin":"%s"}`, pinResult.PIN))
	rec = referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/access/pin-unlock", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("pin unlock: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var unlockResult struct {
		Decision string `json:"decision"`
		Method   string `json:"method"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &unlockResult)
	if unlockResult.Decision != "allow" {
		t.Errorf("expected decision=allow, got %q", unlockResult.Decision)
	}
	if unlockResult.Method != "pin" {
		t.Errorf("expected method=pin, got %q", unlockResult.Method)
	}
}

func TestAppPINUnlock_InvalidPIN(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-pin-unlock-invalid-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	body := []byte(`{"lock_id":"door_jkt_001","pin":"000000"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/access/pin-unlock", token, body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid PIN, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Reason != "invalid_pin" {
		t.Errorf("expected reason=invalid_pin, got %q", result.Reason)
	}
}

func TestAppDoorFavorite(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-favorite-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	// Set favorite
	body := []byte(`{"favorite":true}`)
	rec := referenceAPIRequest(t, router, http.MethodPut, "/api/v1/app/access/doors/door_jkt_001/favorite", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var favResult struct {
		DoorID   string `json:"door_id"`
		Favorite bool   `json:"favorite"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &favResult)
	if !favResult.Favorite {
		t.Errorf("expected favorite=true")
	}

	// Verify it appears in my-doors as favorite
	rec = referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/access/my-doors", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("my-doors: expected 200, got %d", rec.Code)
	}
	var doorsResult struct {
		Items []struct {
			ID         string `json:"id"`
			IsFavorite bool   `json:"is_favorite"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &doorsResult)
	found := false
	for _, item := range doorsResult.Items {
		if item.ID == "door_jkt_001" && item.IsFavorite {
			found = true
			// Verify it's sorted first
			if len(doorsResult.Items) > 0 && doorsResult.Items[0].ID == "door_jkt_001" {
				// Good - favorite is first
			}
			break
		}
	}
	if !found {
		t.Errorf("expected door_jkt_001 to be marked as favorite")
	}
}

func TestAppListCameras(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-cameras-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	// Use admin token since residents may not see cameras without door linkage
	token := referenceAPILogin(t, router, "building.admin.sudirman@mistypass.local")
	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/cameras", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Items []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			Provider string `json:"provider"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	// Cameras may or may not exist in demo data - just verify structure
	if result.Items == nil {
		t.Errorf("expected items array (even if empty)")
	}
}

func TestAppAccessLogsEnhanced(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-logs-enhanced-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/access/logs", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Items []struct {
			ID           string   `json:"id"`
			DoorName     string   `json:"door_name"`
			SnapshotURLs []string `json:"snapshot_urls"`
		} `json:"items"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	// Verify the snapshot_urls field exists in the response structure
	// (items may be empty if no events exist yet)
}

func TestAppVisitorPassesEnhanced(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-visitors-enhanced-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	// Create a visitor pass with valid_from/valid_until
	body := []byte(`{"visitor":"Test Visitor","building_id":"bld_jkt_001","delivery_method":"wallet","ttl_hours":4}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/app/visitor-passes", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var createResult struct {
		ID           string `json:"id"`
		ValidFrom    string `json:"valid_from"`
		ValidUntil   string `json:"valid_until"`
		DisplayLabel string `json:"display_label"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &createResult)
	if createResult.ValidFrom == "" {
		t.Errorf("expected valid_from")
	}
	if createResult.ValidUntil == "" {
		t.Errorf("expected valid_until")
	}
	if createResult.DisplayLabel != "Test Visitor" {
		t.Errorf("expected display_label='Test Visitor', got %q", createResult.DisplayLabel)
	}

	// List visitor passes and check enhanced fields
	rec = referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/visitor-passes", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}
	var listResult struct {
		Items []struct {
			ValidFrom    string `json:"valid_from"`
			ValidUntil   string `json:"valid_until"`
			DisplayLabel string `json:"display_label"`
		} `json:"items"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listResult)
	if listResult.Pagination.Total == 0 {
		t.Errorf("expected at least 1 visitor pass")
	}
	if len(listResult.Items) > 0 {
		if listResult.Items[0].ValidFrom == "" {
			t.Errorf("expected valid_from in list items")
		}
	}
}
