package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestScheduleTemplatesIncludeTimeWindows(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "schedule-template-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/access_rights/schedule_templates?tenant_id=tenant_demo_jakarta", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var result struct {
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			TimeWindows []struct {
				StartTime    string `json:"start_time"`
				EndTime      string `json:"end_time"`
				DayOfWeekSet string `json:"day_of_week_set"`
			} `json:"time_windows"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	foundBusinessHours := false
	for _, item := range result.Items {
		if item.ID == "weekdays_business_hours" {
			foundBusinessHours = true
			if len(item.TimeWindows) == 0 {
				t.Errorf("weekdays_business_hours should have time windows")
			} else if item.TimeWindows[0].DayOfWeekSet != "weekday" {
				t.Errorf("expected weekday, got %s", item.TimeWindows[0].DayOfWeekSet)
			}
		}
	}
	if !foundBusinessHours {
		t.Errorf("expected weekdays_business_hours template")
	}
}

func TestScheduleEvaluateEndpoint(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "schedule-eval-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// test active schedule (weekday business hours, evaluated at Tuesday 10:00)
	body := []byte(`{"tenant_id":"tenant_demo_jakarta","valid_from":"2026-01-01T00:00:00Z","valid_until":"2099-12-31T23:59:59Z","time_windows":[{"start_time":"07:00","end_time":"19:00","day_of_week_set":"weekday"}],"evaluate_at":"2026-04-28T10:00:00Z"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/schedule/evaluate", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var eval struct {
		IsActive bool   `json:"is_active"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &eval); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !eval.IsActive {
		t.Errorf("expected active, got reason=%s", eval.Reason)
	}

	// test outside time window (Saturday 10:00)
	body2 := []byte(`{"tenant_id":"tenant_demo_jakarta","valid_from":"2026-01-01T00:00:00Z","valid_until":"2099-12-31T23:59:59Z","time_windows":[{"start_time":"07:00","end_time":"19:00","day_of_week_set":"weekday"}],"evaluate_at":"2026-05-02T10:00:00Z"}`)
	rec2 := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/schedule/evaluate", token, body2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	var eval2 struct {
		IsActive bool   `json:"is_active"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &eval2)
	if eval2.IsActive {
		t.Errorf("expected not active on Saturday")
	}
	if eval2.Reason != "outside_time_window" {
		t.Errorf("expected reason=outside_time_window, got %s", eval2.Reason)
	}
}

func TestScheduleEvaluateExceptionDate(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "schedule-exception-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// evaluate with exception date matching
	body := []byte(`{"tenant_id":"tenant_demo_jakarta","valid_from":"2026-01-01T00:00:00Z","valid_until":"2099-12-31T23:59:59Z","exception_dates":["2026-04-28"],"evaluate_at":"2026-04-28T10:00:00Z"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/schedule/evaluate", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var eval struct {
		IsActive bool   `json:"is_active"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &eval)
	if eval.IsActive {
		t.Errorf("expected not active on exception date")
	}
	if eval.Reason != "exception_date" {
		t.Errorf("expected reason=exception_date, got %s", eval.Reason)
	}
}

func TestHolidayCalendarCRUD(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "holiday-calendar-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// create
	createBody := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Indonesia 2026","country":"ID","entries":[{"date":"2026-01-01","name":"New Year"},{"date":"2026-08-17","name":"Independence Day"}]}`)
	createRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/holiday_calendars", token, createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Entries []struct {
			Date string `json:"date"`
			Name string `json:"name"`
		} `json:"entries"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &created)
	if created.ID == "" || created.Name != "Indonesia 2026" || len(created.Entries) != 2 {
		t.Fatalf("unexpected create result: %s", createRec.Body.String())
	}

	// list
	listRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/holiday_calendars?tenant_id=tenant_demo_jakarta", token, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}
	body := listRec.Body.String()
	if !strings.Contains(body, created.ID) {
		t.Errorf("list should contain created calendar")
	}

	// get
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/holiday_calendars/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}

	// update
	updateBody := []byte(`{"name":"Indonesia 2026 Updated","entries":[{"date":"2026-01-01","name":"New Year"},{"date":"2026-08-17","name":"Independence Day"},{"date":"2026-12-25","name":"Christmas"}]}`)
	updateRec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/holiday_calendars/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, updateBody)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	// evaluate with holiday
	evalBody := []byte(`{"tenant_id":"tenant_demo_jakarta","valid_from":"2026-01-01T00:00:00Z","valid_until":"2099-12-31T23:59:59Z","holiday_calendar_id":"` + created.ID + `","evaluate_at":"2026-08-17T10:00:00Z"}`)
	evalRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/schedule/evaluate", token, evalBody)
	if evalRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", evalRec.Code)
	}
	var eval struct {
		IsActive bool   `json:"is_active"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal(evalRec.Body.Bytes(), &eval)
	if eval.IsActive {
		t.Errorf("expected not active on holiday")
	}
	if !strings.HasPrefix(eval.Reason, "holiday:") {
		t.Errorf("expected reason starting with holiday:, got %s", eval.Reason)
	}

	// delete
	deleteRec := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/holiday_calendars/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteRec.Code)
	}

	assertReferenceAuditLog(t, router, token, "holiday_calendar_created", "calendar_id="+created.ID, "name=Indonesia 2026")
	assertReferenceAuditLog(t, router, token, "holiday_calendar_updated", "calendar_id="+created.ID)
	assertReferenceAuditLog(t, router, token, "holiday_calendar_deleted", "calendar_id="+created.ID)
}

func TestHolidayCalendarPresetCountries(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "preset-countries-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/holiday_calendars/preset_countries", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Items []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Items) < 5 {
		t.Errorf("expected at least 5 preset countries, got %d", len(result.Items))
	}
	found := false
	for _, c := range result.Items {
		if c.Code == "ID" && c.Name == "Indonesia" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Indonesia in preset countries")
	}
}

func TestHolidayCalendarPresets(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "preset-entries-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// test Indonesia presets
	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/holiday_calendars/presets?country=ID&year=2026", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Country     string `json:"country"`
		CountryName string `json:"country_name"`
		Year        int    `json:"year"`
		Entries     []struct {
			Date string `json:"date"`
			Name string `json:"name"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Country != "ID" || result.CountryName != "Indonesia" || result.Year != 2026 {
		t.Errorf("unexpected metadata: %s/%s/%d", result.Country, result.CountryName, result.Year)
	}
	if len(result.Entries) < 10 {
		t.Errorf("expected at least 10 entries for Indonesia, got %d", len(result.Entries))
	}
	foundIndependenceDay := false
	for _, e := range result.Entries {
		if strings.Contains(e.Name, "Kemerdekaan") && strings.HasSuffix(e.Date, "-08-17") {
			foundIndependenceDay = true
		}
	}
	if !foundIndependenceDay {
		t.Errorf("expected Indonesia Independence Day in presets")
	}

	// test unknown country
	rec2 := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/holiday_calendars/presets?country=XX", token, nil)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown country, got %d", rec2.Code)
	}

	// test missing country param
	rec3 := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/holiday_calendars/presets", token, nil)
	if rec3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing country, got %d", rec3.Code)
	}
}

func TestHolidayCalendarCreateWithPreset(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "preset-create-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// create calendar using preset_country
	createBody := []byte(`{"tenant_id":"tenant_demo_jakarta","preset_country":"SG","preset_year":2026}`)
	createRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/holiday_calendars", token, createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Country string `json:"country"`
		Entries []struct {
			Date string `json:"date"`
			Name string `json:"name"`
		} `json:"entries"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &created)
	if created.Country != "SG" {
		t.Errorf("expected country=SG, got %s", created.Country)
	}
	if created.Name != "Singapore 2026" {
		t.Errorf("expected name=Singapore 2026, got %s", created.Name)
	}
	if len(created.Entries) < 10 {
		t.Errorf("expected at least 10 entries from SG presets, got %d", len(created.Entries))
	}
}
