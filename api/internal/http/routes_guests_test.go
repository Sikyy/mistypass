package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

func TestGuestCRUDLifecycle(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "guest-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Create guest
	createBody := []byte(`{
		"tenant_id": "tenant_demo_jakarta",
		"building_id": "building_demo_001",
		"name": "John Doe",
		"email": "john@example.com",
		"phone": "+628123456",
		"company": "Acme Corp",
		"purpose": "Meeting with CTO",
		"host_name": "Jane Admin",
		"host_email": "jane@mistypass.local",
		"expected_at": "2026-05-01T10:00:00Z"
	}`)
	createRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests", token, createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created access.Guest
	_ = json.Unmarshal(createRec.Body.Bytes(), &created)
	if created.ID == "" || created.Name != "John Doe" || created.Status != "expected" {
		t.Fatalf("unexpected guest: %+v", created)
	}

	// List guests
	listRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/guests?tenant_id=tenant_demo_jakarta", token, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}
	var listResp struct {
		Items []access.Guest `json:"items"`
	}
	_ = json.Unmarshal(listRec.Body.Bytes(), &listResp)
	if len(listResp.Items) == 0 {
		t.Fatalf("expected at least 1 guest")
	}

	// Get guest
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/guests/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}

	// Check in
	checkinBody := []byte(`{"tenant_id":"tenant_demo_jakarta","status":"checked_in"}`)
	checkinRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/guests/"+created.ID+"/status", token, checkinBody)
	if checkinRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", checkinRec.Code, checkinRec.Body.String())
	}
	var checkedIn access.Guest
	_ = json.Unmarshal(checkinRec.Body.Bytes(), &checkedIn)
	if checkedIn.Status != "checked_in" || checkedIn.CheckedInAt == "" {
		t.Fatalf("expected checked_in status with timestamp, got %+v", checkedIn)
	}

	// Check out
	checkoutBody := []byte(`{"tenant_id":"tenant_demo_jakarta","status":"checked_out"}`)
	checkoutRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/guests/"+created.ID+"/status", token, checkoutBody)
	if checkoutRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", checkoutRec.Code)
	}
	var checkedOut access.Guest
	_ = json.Unmarshal(checkoutRec.Body.Bytes(), &checkedOut)
	if checkedOut.Status != "checked_out" || checkedOut.CheckedOutAt == "" {
		t.Fatalf("expected checked_out with timestamp, got %+v", checkedOut)
	}

	// Delete
	deleteRec := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/guests/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	// Get after delete → 404
	getRec2 := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/guests/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getRec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getRec2.Code)
	}
}

func TestGuestCreateValidation(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "guest-validation-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// Missing name
	badBody := []byte(`{"tenant_id":"tenant_demo_jakarta","host_name":"Host"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests", token, badBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d", rec.Code)
	}

	// Missing host
	badBody2 := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Visitor"}`)
	rec2 := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/guests", token, badBody2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing host, got %d", rec2.Code)
	}
}
