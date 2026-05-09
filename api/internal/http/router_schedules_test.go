package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestSchedulesCRUD(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "schedules-crud-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// create
	createBody := []byte(`{
		"tenant_id": "tenant_demo_jakarta",
		"name": "Weekday Business Hours",
		"description": "Mon-Fri 09:00-18:00",
		"valid_from": "2026-01-01T00:00:00Z",
		"valid_until": "2027-01-01T00:00:00Z",
		"time_windows": [{"start_time": "09:00", "end_time": "18:00", "day_of_week_set": "weekday"}],
		"exception_dates": ["2026-08-17"]
	}`)
	createRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/schedules", token, createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		TimeWindows []struct {
			StartTime    string `json:"start_time"`
			DayOfWeekSet string `json:"day_of_week_set"`
		} `json:"time_windows"`
		ExceptionDates []string `json:"exception_dates"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &created)
	if created.ID == "" || !strings.HasPrefix(created.ID, "sched_") {
		t.Fatalf("expected schedule ID with sched_ prefix, got %s", created.ID)
	}
	if created.Name != "Weekday Business Hours" {
		t.Errorf("expected name=Weekday Business Hours, got %s", created.Name)
	}
	if len(created.TimeWindows) != 1 || created.TimeWindows[0].DayOfWeekSet != "weekday" {
		t.Errorf("unexpected time windows: %+v", created.TimeWindows)
	}
	if len(created.ExceptionDates) != 1 || created.ExceptionDates[0] != "2026-08-17" {
		t.Errorf("unexpected exception dates: %v", created.ExceptionDates)
	}

	// list
	listRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/schedules?tenant_id=tenant_demo_jakarta", token, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}
	if !strings.Contains(listRec.Body.String(), created.ID) {
		t.Errorf("list should contain the created schedule")
	}

	// get
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/schedules/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}

	// update
	updateBody := []byte(`{"name":"Weekday Extended","time_windows":[{"start_time":"06:00","end_time":"22:00","day_of_week_set":"weekday"}]}`)
	updateRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/schedules/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, updateBody)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}
	var updated struct {
		Name        string `json:"name"`
		TimeWindows []struct {
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
		} `json:"time_windows"`
	}
	_ = json.Unmarshal(updateRec.Body.Bytes(), &updated)
	if updated.Name != "Weekday Extended" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}
	if len(updated.TimeWindows) != 1 || updated.TimeWindows[0].EndTime != "22:00" {
		t.Errorf("expected updated time window, got %+v", updated.TimeWindows)
	}

	// delete
	deleteRec := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/schedules/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteRec.Code)
	}

	// verify deleted
	getRec2 := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/schedules/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getRec2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getRec2.Code)
	}

	// check audit
	assertReferenceAuditLog(t, router, token, "schedule_created", "schedule_id="+created.ID)
	assertReferenceAuditLog(t, router, token, "schedule_updated", "schedule_id="+created.ID)
	assertReferenceAuditLog(t, router, token, "schedule_deleted", "schedule_id="+created.ID)
}

func TestScheduleCreateValidation(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "schedules-validation-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// missing name
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/schedules", token, []byte(`{"tenant_id":"tenant_demo_jakarta"}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", rec.Code)
	}
}
