package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func TestBuildingScopeForRequestDerivesPlaceAdminRoleAssignment(t *testing.T) {
	s := &server{accessSvc: access.NewService()}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/places", nil)
	request = withAuthUser(request, auth.User{
		ID:       "usr_place_admin_sudirman_001",
		Email:    "place.admin.sudirman@mistypass.local",
		Role:     "building_admin",
		TenantID: "tenant_demo_jakarta",
	})

	scope, ok := s.buildingScopeForRequest(httptest.NewRecorder(), request)
	if !ok {
		t.Fatalf("expected scope resolution to succeed")
	}
	if _, exists := scope["building_demo_001"]; !exists || len(scope) != 1 {
		t.Fatalf("expected role assignment scope building_demo_001, got %#v", scope)
	}
}

func TestBuildingScopeForRequestDerivesTeamPlaceAdminRoleAssignment(t *testing.T) {
	accessSvc := access.NewService()
	if _, err := accessSvc.CreateRoleAssignment(access.RoleAssignmentInput{
		TenantID:      "tenant_demo_jakarta",
		RoleID:        "role_place_admin",
		AppliesToType: "Place",
		AppliesToID:   "building_demo_002",
		AssigneeType:  "Team",
		AssigneeID:    "team_operations_jkt",
	}); err != nil {
		t.Fatalf("create team place admin role assignment: %v", err)
	}
	s := &server{accessSvc: accessSvc}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/places", nil)
	request = withAuthUser(request, auth.User{
		ID:       "usr_place_admin_sudirman_001",
		Email:    "place.admin.sudirman@mistypass.local",
		Role:     "building_admin",
		TenantID: "tenant_demo_jakarta",
	})

	scope, ok := s.buildingScopeForRequest(httptest.NewRecorder(), request)
	if !ok {
		t.Fatalf("expected scope resolution to succeed")
	}
	if _, exists := scope["building_demo_001"]; !exists {
		t.Fatalf("expected direct place assignment in scope, got %#v", scope)
	}
	if _, exists := scope["building_demo_002"]; !exists {
		t.Fatalf("expected team place assignment in scope, got %#v", scope)
	}
}

func TestBuildingScopeForRequestRequiresRoleAssignmentOrTokenScope(t *testing.T) {
	s := &server{accessSvc: access.NewService()}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/places", nil)
	request = withAuthUser(request, auth.User{
		ID:       "usr_building_admin_missing_scope",
		Email:    "missing.scope@mistypass.local",
		Role:     "building_admin",
		TenantID: "tenant_demo_jakarta",
	})
	recorder := httptest.NewRecorder()

	if _, ok := s.buildingScopeForRequest(recorder, request); ok {
		t.Fatalf("expected scope resolution to fail")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
