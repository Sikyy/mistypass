package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestCurrentUserProfileGetAndUpdate(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "current-user-profile-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	getRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/user", token, nil)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected current user status 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	var initial struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Email      string   `json:"email"`
		Role       string   `json:"role"`
		TenantID   string   `json:"tenant_id"`
		BuildingID []string `json:"building_ids"`
		Language   string   `json:"language"`
	}
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode current user: %v", err)
	}
	if initial.Name != "Organization Admin" || initial.Language != "en-US" {
		t.Fatalf("unexpected initial profile: %#v", initial)
	}

	updateRecorder := referenceAPIRequest(
		t,
		router,
		http.MethodPatch,
		"/api/v1/user",
		token,
		[]byte(`{"name":"Operations Admin","language":"id-ID"}`),
	)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected profile update status 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		TenantID string `json:"tenant_id"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated current user: %v", err)
	}
	if updated.ID != initial.ID || updated.Email != initial.Email || updated.Role != initial.Role || updated.TenantID != initial.TenantID {
		t.Fatalf("profile update changed privileged fields: before=%#v after=%#v", initial, updated)
	}
	if updated.Name != "Operations Admin" || updated.Language != "id-ID" {
		t.Fatalf("unexpected updated profile: %#v", updated)
	}

	meRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/me", token, nil)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("expected legacy me status 200, got %d body=%s", meRecorder.Code, meRecorder.Body.String())
	}
	if !strings.Contains(meRecorder.Body.String(), `"name":"Operations Admin"`) ||
		!strings.Contains(meRecorder.Body.String(), `"language":"id-ID"`) {
		t.Fatalf("expected /me to reflect updated profile, body=%s", meRecorder.Body.String())
	}

	unknownFieldRecorder := referenceAPIRequest(
		t,
		router,
		http.MethodPatch,
		"/api/v1/user",
		token,
		[]byte(`{"name":"Escalated Admin","role":"super_admin"}`),
	)
	if unknownFieldRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unknown privileged field to be rejected, got %d body=%s", unknownFieldRecorder.Code, unknownFieldRecorder.Body.String())
	}

	afterRejectedRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/user", token, nil)
	if afterRejectedRecorder.Code != http.StatusOK {
		t.Fatalf("expected current user after rejected update status 200, got %d body=%s", afterRejectedRecorder.Code, afterRejectedRecorder.Body.String())
	}
	if !strings.Contains(afterRejectedRecorder.Body.String(), `"name":"Operations Admin"`) ||
		strings.Contains(afterRejectedRecorder.Body.String(), `"role":"super_admin"`) {
		t.Fatalf("expected rejected update to leave profile unchanged, body=%s", afterRejectedRecorder.Body.String())
	}

	assertReferenceAuditLog(t, router, token, "auth_user_profile_updated", "user_id="+initial.ID, "changed=name|language")
}

func TestCurrentUserProfileRejectsUnsupportedLanguage(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "current-user-profile-language-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	recorder := referenceAPIRequest(
		t,
		router,
		http.MethodPatch,
		"/api/v1/user",
		token,
		[]byte(`{"language":"fr-FR"}`),
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported language status 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "unsupported language") {
		t.Fatalf("expected unsupported language error, body=%s", recorder.Body.String())
	}
}
