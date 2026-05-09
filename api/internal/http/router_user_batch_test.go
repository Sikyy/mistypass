package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestBatchUpdateUserStatusEndpoint(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "batch-user-status-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// create two users
	user1 := createTestUser(t, router, token, "Batch Status A", "batch.status.a@test.local")
	user2 := createTestUser(t, router, token, "Batch Status B", "batch.status.b@test.local")

	// batch suspend
	body := []byte(`{"tenant_id":"tenant_demo_jakarta","user_ids":["` + user1 + `","` + user2 + `"],"status":"suspended"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/batch-status", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Updated != 2 {
		t.Errorf("expected 2 updated, got %d", result.Updated)
	}
	assertReferenceAuditLog(t, router, token, "users_batch_status_updated", "status=suspended", "updated=2")
}

func TestBatchDeleteUsersEndpoint(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "batch-user-delete-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	user1 := createTestUser(t, router, token, "Batch Delete A", "batch.del.a@test.local")
	user2 := createTestUser(t, router, token, "Batch Delete B", "batch.del.b@test.local")

	body := []byte(`{"tenant_id":"tenant_demo_jakarta","user_ids":["` + user1 + `","` + user2 + `"]}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/batch-delete", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", result.Deleted)
	}
	assertReferenceAuditLog(t, router, token, "users_batch_deleted", "deleted=2")
}

func TestBatchInviteUsersEndpoint(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "batch-user-invite-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	user1 := createTestUser(t, router, token, "Batch Invite A", "batch.inv.a@test.local")

	body := []byte(`{"tenant_id":"tenant_demo_jakarta","user_ids":["` + user1 + `"],"delivery_method":"email"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/batch-invite", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Queued int `json:"queued"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Queued < 1 {
		t.Errorf("expected at least 1 queued, got %d", result.Queued)
	}
	assertReferenceAuditLog(t, router, token, "users_batch_invited", "queued=")
}

func TestExportUsersCSVEndpoint(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "export-users-csv-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/users/export-csv?tenant_id=tenant_demo_jakarta", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/csv") {
		t.Errorf("expected text/csv content type, got %s", contentType)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "id,name,email,") {
		t.Errorf("expected CSV header, got: %s", body[:min(80, len(body))])
	}
	assertReferenceAuditLog(t, router, token, "users_exported_csv", "format=csv")
}

func TestImportUsersCSVEndpoint(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "import-users-csv-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	csvContent := "name,email,role,status\nCSV Import User,csv.import@test.local,employee,active"
	body := []byte(`{"tenant_id":"tenant_demo_jakarta","csv_content":"` + strings.ReplaceAll(csvContent, "\n", "\\n") + `"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/import-csv", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Created int `json:"created"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Created < 1 {
		t.Errorf("expected at least 1 created, got %d", result.Created)
	}
	assertReferenceAuditLog(t, router, token, "users_imported_csv", "created=")
}

func createTestUser(t *testing.T, router http.Handler, token, name, email string) string {
	t.Helper()
	body := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"` + name + `","email":"` + email + `","role":"employee","status":"active"}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create test user: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	return created.ID
}
