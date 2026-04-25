package talenta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
)

func TestPullAdapterPullsPaginatedEmployees(t *testing.T) {
	const (
		clientID     = "talenta-client-001"
		clientSecret = "talenta-secret-001"
	)

	requestCount := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %s", r.Method)
		}
		if got := r.URL.Path; got != "/v2/talenta/v2/employee" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.URL.Query().Get("limit"); got != "1" {
			t.Fatalf("unexpected limit query: %s", got)
		}

		dateHeader := r.Header.Get("Date")
		if dateHeader == "" {
			t.Fatalf("expected Date header")
		}
		expectedAuth := `hmac username="` + clientID + `", algorithm="hmac-sha256", headers="date request-line", signature="` +
			pullSignature(clientSecret, dateHeader, r.Method, r.URL.RequestURI(), r.Proto) + `"`
		if got := r.Header.Get("Authorization"); got != expectedAuth {
			t.Fatalf("unexpected authorization header: %s", got)
		}

		page := r.URL.Query().Get("page")
		var response map[string]any
		switch page {
		case "1":
			response = map[string]any{
				"data": []map[string]any{
					{
						"employment": map[string]any{
							"employee_id":       "EMP-001",
							"employee_number":   "E-001",
							"organization_name": "IT",
							"job_position":      "Engineer",
							"branch":            "Jakarta HQ",
							"employment_status": "active",
							"join_date":         "2026-04-20",
						},
						"personal": map[string]any{
							"first_name":   "Alice",
							"last_name":    "Tan",
							"email":        "alice@pull.local",
							"mobile_phone": "+628111111111",
							"avatar":       "https://cdn.example.com/photos/alice-tan.jpg",
						},
						"leave_info": map[string]any{
							"status": "approved",
							"type":   "Annual Leave",
						},
						"payroll_info": map[string]any{
							"cost_center_name": "CC-IT-01",
						},
					},
				},
				"pagination": map[string]any{
					"current_page": 1,
					"last_page":    2,
				},
			}
		case "2":
			response = map[string]any{
				"data": []map[string]any{
					{
						"employment": map[string]any{
							"employee_id":       "EMP-002",
							"employee_number":   "E-002",
							"organization_name": "Security",
							"job_position":      "Supervisor",
							"branch":            "Bandung",
							"employment_status": "active",
						},
						"personal": map[string]any{
							"first_name":   "Budi",
							"last_name":    "Santoso",
							"email":        "budi@pull.local",
							"mobile_phone": "+628122222222",
						},
					},
				},
				"pagination": map[string]any{
					"current_page": 2,
					"last_page":    2,
				},
			}
		default:
			t.Fatalf("unexpected page query: %s", page)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	adapter := NewPullAdapter()
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	result, err := adapter.Pull(context.Background(), hris.PullInput{
		Connector: enterprise.HRISConnector{
			ID:       "hrc_talenta_001",
			TenantID: "tenant_demo_jakarta",
			Vendor:   "talenta",
		},
		CredentialValue: `{"client_id":"` + clientID + `","client_secret":"` + clientSecret + `","base_url":"` + apiServer.URL + `","page_limit":1}`,
		Mode:            hris.PullModeFull,
		HTTPClient:      apiServer.Client(),
		Now:             now,
	})
	if err != nil {
		t.Fatalf("expected pull success: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected two paginated requests, got %d", requestCount)
	}
	if result.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant id: %s", result.TenantID)
	}
	if result.Source != "hris_talenta" {
		t.Fatalf("unexpected source: %s", result.Source)
	}
	if len(result.Employees) != 2 {
		t.Fatalf("expected two employees, got %d", len(result.Employees))
	}
	if result.Employees[0].ExternalID != "EMP-001" || result.Employees[1].ExternalID != "EMP-002" {
		t.Fatalf("unexpected employee ids: %+v", result.Employees)
	}
	if result.Employees[0].LeaveStatus != "annual_leave" {
		t.Fatalf("expected leave_status annual_leave on first employee, got %s", result.Employees[0].LeaveStatus)
	}
	if result.Employees[0].JoinDate != "2026-04-20" {
		t.Fatalf("expected join_date 2026-04-20 on first employee, got %s", result.Employees[0].JoinDate)
	}
	if result.Employees[0].CostCenter != "CC-IT-01" {
		t.Fatalf("expected cost_center CC-IT-01 on first employee, got %s", result.Employees[0].CostCenter)
	}
	if result.Employees[0].PhotoURL != "https://cdn.example.com/photos/alice-tan.jpg" {
		t.Fatalf("expected photo_url on first employee, got %s", result.Employees[0].PhotoURL)
	}
	if result.RequestID == "" {
		t.Fatalf("expected non-empty request id")
	}
	if result.Mode != hris.PullModeFull {
		t.Fatalf("expected full pull mode, got %s", result.Mode)
	}
}

func TestPullAdapterSupportsIncrementalQueryParams(t *testing.T) {
	const (
		clientID     = "talenta-client-002"
		clientSecret = "talenta-secret-002"
	)

	var capturedQuery string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-010",
						"employee_number":   "E-010",
						"organization_name": "IT",
						"job_position":      "Engineer",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
					},
					"personal": map[string]any{
						"first_name": "Ina",
						"last_name":  "Wijaya",
						"email":      "ina@pull.local",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	lastSuccessAt := time.Date(2026, 4, 22, 9, 30, 0, 0, time.UTC)
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	adapter := NewPullAdapter()
	if !adapter.SupportsIncremental(hris.PullInput{
		CredentialValue: `{"client_id":"` + clientID + `","client_secret":"` + clientSecret + `","base_url":"` + apiServer.URL + `","updated_after_param":"updated_after","updated_before_param":"updated_before","timestamp_format":"rfc3339"}`,
		LastSuccessAt:   &lastSuccessAt,
	}) {
		t.Fatalf("expected adapter to report incremental support")
	}

	result, err := adapter.Pull(context.Background(), hris.PullInput{
		Connector: enterprise.HRISConnector{
			ID:       "hrc_talenta_002",
			TenantID: "tenant_demo_jakarta",
			Vendor:   "talenta",
		},
		CredentialValue: `{"client_id":"` + clientID + `","client_secret":"` + clientSecret + `","base_url":"` + apiServer.URL + `","updated_after_param":"updated_after","updated_before_param":"updated_before","timestamp_format":"rfc3339"}`,
		LastSuccessAt:   &lastSuccessAt,
		Mode:            hris.PullModeIncremental,
		HTTPClient:      apiServer.Client(),
		Now:             now,
	})
	if err != nil {
		t.Fatalf("expected incremental pull success: %v", err)
	}
	if result.Mode != hris.PullModeIncremental {
		t.Fatalf("expected incremental pull mode, got %s", result.Mode)
	}
	if capturedQuery == "" || !strings.Contains(capturedQuery, "updated_after=2026-04-22T09%3A30%3A00Z") || !strings.Contains(capturedQuery, "updated_before=2026-04-22T10%3A00%3A00Z") {
		t.Fatalf("unexpected incremental query: %s", capturedQuery)
	}
}

func TestPullAdapterSupportsIncrementalWithDefaultQueryContract(t *testing.T) {
	const (
		clientID     = "talenta-client-003"
		clientSecret = "talenta-secret-003"
	)

	var capturedQuery string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-011",
						"employee_number":   "E-011",
						"organization_name": "Security",
						"job_position":      "Supervisor",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
					},
					"personal": map[string]any{
						"first_name": "Rani",
						"last_name":  "Putri",
						"email":      "rani@pull.local",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	lastSuccessAt := time.Date(2026, 4, 22, 9, 45, 0, 0, time.UTC)
	now := time.Date(2026, 4, 22, 10, 15, 0, 0, time.UTC)
	adapter := NewPullAdapter()
	if !adapter.SupportsIncremental(hris.PullInput{
		CredentialValue: `{"client_id":"` + clientID + `","client_secret":"` + clientSecret + `","base_url":"` + apiServer.URL + `"}`,
		LastSuccessAt:   &lastSuccessAt,
	}) {
		t.Fatalf("expected adapter to report incremental support via default contract")
	}

	result, err := adapter.Pull(context.Background(), hris.PullInput{
		Connector: enterprise.HRISConnector{
			ID:       "hrc_talenta_003",
			TenantID: "tenant_demo_jakarta",
			Vendor:   "talenta",
		},
		CredentialValue: `{"client_id":"` + clientID + `","client_secret":"` + clientSecret + `","base_url":"` + apiServer.URL + `"}`,
		LastSuccessAt:   &lastSuccessAt,
		Mode:            hris.PullModeIncremental,
		HTTPClient:      apiServer.Client(),
		Now:             now,
	})
	if err != nil {
		t.Fatalf("expected incremental pull success with default contract: %v", err)
	}
	if result.Mode != hris.PullModeIncremental {
		t.Fatalf("expected incremental pull mode, got %s", result.Mode)
	}
	if capturedQuery == "" ||
		!strings.Contains(capturedQuery, "updated_after=2026-04-22T09%3A45%3A00Z") ||
		!strings.Contains(capturedQuery, "updated_before=2026-04-22T10%3A15%3A00Z") {
		t.Fatalf("unexpected default incremental query: %s", capturedQuery)
	}
}

func TestPullAdapterExplicitIncrementalContractOverridesDefaults(t *testing.T) {
	const (
		clientID     = "talenta-client-004"
		clientSecret = "talenta-secret-004"
	)

	var capturedQuery string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-012",
						"employee_number":   "E-012",
						"organization_name": "IT",
						"job_position":      "Analyst",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
					},
					"personal": map[string]any{
						"first_name": "Dewi",
						"last_name":  "Larisa",
						"email":      "dewi@pull.local",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	lastSuccessAt := time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC)
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	adapter := NewPullAdapter()
	result, err := adapter.Pull(context.Background(), hris.PullInput{
		Connector: enterprise.HRISConnector{
			ID:       "hrc_talenta_004",
			TenantID: "tenant_demo_jakarta",
			Vendor:   "talenta",
		},
		CredentialValue: `{"client_id":"` + clientID + `","client_secret":"` + clientSecret + `","base_url":"` + apiServer.URL + `","updated_after_param":"modified_after","updated_before_param":"modified_before","timestamp_format":"datetime"}`,
		LastSuccessAt:   &lastSuccessAt,
		Mode:            hris.PullModeIncremental,
		HTTPClient:      apiServer.Client(),
		Now:             now,
	})
	if err != nil {
		t.Fatalf("expected incremental pull success with explicit override: %v", err)
	}
	if result.Mode != hris.PullModeIncremental {
		t.Fatalf("expected incremental pull mode, got %s", result.Mode)
	}
	if capturedQuery == "" ||
		!strings.Contains(capturedQuery, "modified_after=2026-04-22+09%3A00%3A00") ||
		!strings.Contains(capturedQuery, "modified_before=2026-04-22+10%3A00%3A00") ||
		strings.Contains(capturedQuery, "updated_after=") ||
		strings.Contains(capturedQuery, "updated_before=") {
		t.Fatalf("unexpected explicit incremental query: %s", capturedQuery)
	}
}
