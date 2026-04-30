package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestReferenceResourceEndpointsExposeKisiStyleModels(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-api-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	rolesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/roles", token, nil)
	if rolesRecorder.Code != http.StatusOK {
		t.Fatalf("expected roles status 200, got %d body=%s", rolesRecorder.Code, rolesRecorder.Body.String())
	}
	if got := rolesRecorder.Header().Get("X-Collection-Range"); !strings.HasPrefix(got, "items 0-") {
		t.Fatalf("expected collection range header, got %q", got)
	}
	var rolesResponse struct {
		Items []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			AppliesTo string `json:"applies_to"`
		} `json:"items"`
		Pagination struct {
			Offset  int  `json:"offset"`
			Limit   int  `json:"limit"`
			Total   int  `json:"total"`
			HasMore bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rolesRecorder.Body.Bytes(), &rolesResponse); err != nil {
		t.Fatalf("decode roles: %v", err)
	}
	if rolesResponse.Pagination.Offset != 0 ||
		rolesResponse.Pagination.Limit != len(rolesResponse.Items) ||
		rolesResponse.Pagination.Total != len(rolesResponse.Items) ||
		rolesResponse.Pagination.HasMore {
		t.Fatalf("expected collection pagination metadata, got %+v items=%d", rolesResponse.Pagination, len(rolesResponse.Items))
	}
	pagedRolesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/roles?limit=1&offset=1", token, nil)
	if pagedRolesRecorder.Code != http.StatusOK {
		t.Fatalf("expected paged roles status 200, got %d body=%s", pagedRolesRecorder.Code, pagedRolesRecorder.Body.String())
	}
	if got := pagedRolesRecorder.Header().Get("X-Collection-Range"); !strings.HasPrefix(got, "items 1-1/") {
		t.Fatalf("expected paged collection range header, got %q", got)
	}
	var pagedRolesResponse struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Pagination struct {
			Offset  int  `json:"offset"`
			Limit   int  `json:"limit"`
			Total   int  `json:"total"`
			HasMore bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(pagedRolesRecorder.Body.Bytes(), &pagedRolesResponse); err != nil {
		t.Fatalf("decode paged roles: %v", err)
	}
	if len(pagedRolesResponse.Items) != 1 ||
		pagedRolesResponse.Pagination.Offset != 1 ||
		pagedRolesResponse.Pagination.Limit != 1 ||
		pagedRolesResponse.Pagination.Total != len(rolesResponse.Items) ||
		pagedRolesResponse.Pagination.HasMore != (len(rolesResponse.Items) > 2) {
		t.Fatalf("expected paged roles metadata/items, got pagination=%+v items=%d total_roles=%d", pagedRolesResponse.Pagination, len(pagedRolesResponse.Items), len(rolesResponse.Items))
	}
	roleIDs := map[string]string{}
	for _, role := range rolesResponse.Items {
		roleIDs[role.ID] = role.AppliesTo
	}
	if roleIDs["role_organization_admin"] != "Organization" {
		t.Fatalf("expected organization admin role, got %v", roleIDs)
	}
	if roleIDs["role_place_admin"] != "Place" {
		t.Fatalf("expected place admin role, got %v", roleIDs)
	}

	assignmentsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/role_assignments", token, nil)
	if assignmentsRecorder.Code != http.StatusOK {
		t.Fatalf("expected role assignments status 200, got %d body=%s", assignmentsRecorder.Code, assignmentsRecorder.Body.String())
	}
	var assignmentsResponse struct {
		Items []struct {
			RoleID        string `json:"role_id"`
			AppliesToType string `json:"applies_to_type"`
			AssigneeEmail string `json:"assignee_email"`
		} `json:"items"`
	}
	if err := json.Unmarshal(assignmentsRecorder.Body.Bytes(), &assignmentsResponse); err != nil {
		t.Fatalf("decode role assignments: %v", err)
	}
	foundOrgAdmin := false
	for _, assignment := range assignmentsResponse.Items {
		if assignment.RoleID == "role_organization_admin" &&
			assignment.AppliesToType == "Organization" &&
			assignment.AssigneeEmail == "organization.admin@mistypass.local" {
			foundOrgAdmin = true
		}
		if strings.Contains(assignment.AssigneeEmail, "resident") {
			t.Fatalf("resident account should not be exposed as management role assignment: %#v", assignment)
		}
	}
	if !foundOrgAdmin {
		t.Fatalf("expected seeded Organization Admin role assignment, got %#v", assignmentsResponse.Items)
	}

	createAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_002","assignee_type":"User","assignee_id":"usr_place_admin_kuningan_001","assignee_email":"place.admin.kuningan@mistypass.local"}}`)
	createAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", token, createAssignmentBody)
	if createAssignmentRecorder.Code != http.StatusCreated {
		t.Fatalf("expected role assignment create status 201, got %d body=%s", createAssignmentRecorder.Code, createAssignmentRecorder.Body.String())
	}
	var createdAssignment struct {
		ID            string `json:"id"`
		RoleID        string `json:"role_id"`
		AppliesToType string `json:"applies_to_type"`
		AssigneeEmail string `json:"assignee_email"`
		ValidUntil    string `json:"valid_until"`
	}
	if err := json.Unmarshal(createAssignmentRecorder.Body.Bytes(), &createdAssignment); err != nil {
		t.Fatalf("decode created role assignment: %v", err)
	}
	if createdAssignment.ID == "" {
		t.Fatalf("expected created role assignment id, body=%s", createAssignmentRecorder.Body.String())
	}
	getAssignmentRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/role_assignments/"+createdAssignment.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getAssignmentRecorder.Code != http.StatusOK {
		t.Fatalf("expected role assignment detail status 200, got %d body=%s", getAssignmentRecorder.Code, getAssignmentRecorder.Body.String())
	}
	if !strings.Contains(getAssignmentRecorder.Body.String(), `"id":"`+createdAssignment.ID+`"`) {
		t.Fatalf("expected role assignment detail to include created assignment, body=%s", getAssignmentRecorder.Body.String())
	}
	updateAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_002","assignee_type":"User","assignee_id":"usr_place_admin_kuningan_001","assignee_email":"place.admin.kuningan.updated@mistypass.local","valid_until":"2026-05-01T10:00:00Z"}}`)
	updateAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/role_assignments/"+createdAssignment.ID, token, updateAssignmentBody)
	if updateAssignmentRecorder.Code != http.StatusOK {
		t.Fatalf("expected role assignment update status 200, got %d body=%s", updateAssignmentRecorder.Code, updateAssignmentRecorder.Body.String())
	}
	if !strings.Contains(updateAssignmentRecorder.Body.String(), `"assignee_email":"place.admin.kuningan.updated@mistypass.local"`) ||
		!strings.Contains(updateAssignmentRecorder.Body.String(), `"valid_until":"2026-05-01T10:00:00Z"`) {
		t.Fatalf("expected updated role assignment fields, body=%s", updateAssignmentRecorder.Body.String())
	}
	deleteAssignmentRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/role_assignments/"+createdAssignment.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteAssignmentRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected role assignment delete status 204, got %d body=%s", deleteAssignmentRecorder.Code, deleteAssignmentRecorder.Body.String())
	}
	deletedAssignmentRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/role_assignments/"+createdAssignment.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletedAssignmentRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted role assignment detail status 404, got %d body=%s", deletedAssignmentRecorder.Code, deletedAssignmentRecorder.Body.String())
	}

	teamsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/teams?scope=place", token, nil)
	if teamsRecorder.Code != http.StatusOK {
		t.Fatalf("expected teams status 200, got %d body=%s", teamsRecorder.Code, teamsRecorder.Body.String())
	}
	var teamsResponse struct {
		Items []struct {
			ID           string `json:"id"`
			ResourceType string `json:"resource_type"`
			Name         string `json:"name"`
			PlaceID      string `json:"place_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(teamsRecorder.Body.Bytes(), &teamsResponse); err != nil {
		t.Fatalf("decode teams: %v", err)
	}
	foundEngineeringTeam := false
	for _, team := range teamsResponse.Items {
		if team.ID == "team_engineering_jkt" &&
			team.ResourceType == "Team" &&
			team.Name == "Engineering Team" &&
			team.PlaceID == "building_demo_001" {
			foundEngineeringTeam = true
		}
	}
	if !foundEngineeringTeam {
		t.Fatalf("expected seeded engineering team, got %#v", teamsResponse.Items)
	}

	membershipsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/team_memberships?team_id=team_engineering_jkt", token, nil)
	if membershipsRecorder.Code != http.StatusOK {
		t.Fatalf("expected team memberships status 200, got %d body=%s", membershipsRecorder.Code, membershipsRecorder.Body.String())
	}
	if !strings.Contains(membershipsRecorder.Body.String(), `"resource_type":"TeamMembership"`) {
		t.Fatalf("expected team membership resource type, body=%s", membershipsRecorder.Body.String())
	}

	createTeamBody := []byte(`{"team":{"tenant_id":"tenant_demo_jakarta","name":"Reference API Team","scope":"place","place_id":"building_demo_001","description":"Created through reference teams"}}`)
	createTeamRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/teams", token, createTeamBody)
	if createTeamRecorder.Code != http.StatusCreated {
		t.Fatalf("expected team create status 201, got %d body=%s", createTeamRecorder.Code, createTeamRecorder.Body.String())
	}
	var createdTeam struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Scope       string `json:"scope"`
		PlaceID     string `json:"place_id"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(createTeamRecorder.Body.Bytes(), &createdTeam); err != nil {
		t.Fatalf("decode created team: %v", err)
	}
	if createdTeam.ID == "" || createdTeam.Name != "Reference API Team" || createdTeam.Scope != "place" || createdTeam.PlaceID != "building_demo_001" {
		t.Fatalf("expected created team fields, got %#v body=%s", createdTeam, createTeamRecorder.Body.String())
	}

	getTeamRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/teams/"+createdTeam.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getTeamRecorder.Code != http.StatusOK {
		t.Fatalf("expected team detail status 200, got %d body=%s", getTeamRecorder.Code, getTeamRecorder.Body.String())
	}
	if !strings.Contains(getTeamRecorder.Body.String(), `"id":"`+createdTeam.ID+`"`) {
		t.Fatalf("expected team detail fields, body=%s", getTeamRecorder.Body.String())
	}

	updateTeamBody := []byte(`{"team":{"tenant_id":"tenant_demo_jakarta","name":"Reference API Team Updated","scope":"place","place_id":"building_demo_002","description":"Updated team"}}`)
	updateTeamRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/teams/"+createdTeam.ID, token, updateTeamBody)
	if updateTeamRecorder.Code != http.StatusOK {
		t.Fatalf("expected team update status 200, got %d body=%s", updateTeamRecorder.Code, updateTeamRecorder.Body.String())
	}
	if !strings.Contains(updateTeamRecorder.Body.String(), `"name":"Reference API Team Updated"`) ||
		!strings.Contains(updateTeamRecorder.Body.String(), `"place_id":"building_demo_002"`) {
		t.Fatalf("expected updated team fields, body=%s", updateTeamRecorder.Body.String())
	}

	createMembershipBody := []byte(`{"team_membership":{"tenant_id":"tenant_demo_jakarta","team_id":"` + createdTeam.ID + `","member_type":"User","member_id":"usr_1001","member_email":"employee@example.com","member_name":"Reference Employee"}}`)
	createMembershipRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/team_memberships", token, createMembershipBody)
	if createMembershipRecorder.Code != http.StatusCreated {
		t.Fatalf("expected team membership create status 201, got %d body=%s", createMembershipRecorder.Code, createMembershipRecorder.Body.String())
	}
	var createdMembership struct {
		ID         string `json:"id"`
		TeamID     string `json:"team_id"`
		MemberID   string `json:"member_id"`
		MemberName string `json:"member_name"`
	}
	if err := json.Unmarshal(createMembershipRecorder.Body.Bytes(), &createdMembership); err != nil {
		t.Fatalf("decode created team membership: %v", err)
	}
	if createdMembership.ID == "" || createdMembership.TeamID != createdTeam.ID || createdMembership.MemberID != "usr_1001" {
		t.Fatalf("expected created team membership fields, got %#v body=%s", createdMembership, createMembershipRecorder.Body.String())
	}

	createdMembershipsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/team_memberships?team_id="+createdTeam.ID, token, nil)
	if createdMembershipsRecorder.Code != http.StatusOK {
		t.Fatalf("expected created team memberships status 200, got %d body=%s", createdMembershipsRecorder.Code, createdMembershipsRecorder.Body.String())
	}
	if !strings.Contains(createdMembershipsRecorder.Body.String(), `"id":"`+createdMembership.ID+`"`) {
		t.Fatalf("expected created membership to be listable, body=%s", createdMembershipsRecorder.Body.String())
	}

	createTeamAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_002","assignee_type":"Team","assignee_id":"` + createdTeam.ID + `"}}`)
	createTeamAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", token, createTeamAssignmentBody)
	if createTeamAssignmentRecorder.Code != http.StatusCreated {
		t.Fatalf("expected team role assignment create status 201, got %d body=%s", createTeamAssignmentRecorder.Code, createTeamAssignmentRecorder.Body.String())
	}
	if !strings.Contains(createTeamAssignmentRecorder.Body.String(), `"assignee_type":"Team"`) ||
		!strings.Contains(createTeamAssignmentRecorder.Body.String(), `"assignee_id":"`+createdTeam.ID+`"`) {
		t.Fatalf("expected team role assignment wrapper, body=%s", createTeamAssignmentRecorder.Body.String())
	}

	deleteMembershipRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/team_memberships/"+createdMembership.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteMembershipRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected team membership delete status 204, got %d body=%s", deleteMembershipRecorder.Code, deleteMembershipRecorder.Body.String())
	}

	deleteTeamRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/teams/"+createdTeam.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteTeamRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected team delete status 204, got %d body=%s", deleteTeamRecorder.Code, deleteTeamRecorder.Body.String())
	}
	deletedTeamRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/teams/"+createdTeam.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletedTeamRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted team detail status 404, got %d body=%s", deletedTeamRecorder.Code, deletedTeamRecorder.Body.String())
	}
}

func TestReferenceResourceEndpointsMapPlacesLocksAndShares(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-api-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	placesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places", token, nil)
	if placesRecorder.Code != http.StatusOK {
		t.Fatalf("expected places status 200, got %d body=%s", placesRecorder.Code, placesRecorder.Body.String())
	}
	if !strings.Contains(placesRecorder.Body.String(), "building_demo_001") {
		t.Fatalf("expected places to include legacy building ids, body=%s", placesRecorder.Body.String())
	}

	placeBody := []byte(`{"place":{"tenant_id":"tenant_demo_jakarta","name":"Reference Place API","address":"Jl. Test 1","region":"ID-JK"}}`)
	createPlaceRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/places", token, placeBody)
	if createPlaceRecorder.Code != http.StatusCreated {
		t.Fatalf("expected place create status 201, got %d body=%s", createPlaceRecorder.Code, createPlaceRecorder.Body.String())
	}
	var createdPlace struct {
		ID       string `json:"id"`
		TenantID string `json:"tenant_id"`
		Name     string `json:"name"`
		Address  string `json:"address"`
		Region   string `json:"region"`
	}
	if err := json.Unmarshal(createPlaceRecorder.Body.Bytes(), &createdPlace); err != nil {
		t.Fatalf("decode created place: %v", err)
	}
	if createdPlace.ID == "" || createdPlace.Name != "Reference Place API" || createdPlace.Region != "ID-JK" {
		t.Fatalf("expected created place fields, got %#v body=%s", createdPlace, createPlaceRecorder.Body.String())
	}
	getPlaceRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places/"+createdPlace.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getPlaceRecorder.Code != http.StatusOK {
		t.Fatalf("expected place detail status 200, got %d body=%s", getPlaceRecorder.Code, getPlaceRecorder.Body.String())
	}
	updatePlaceBody := []byte(`{"place":{"tenant_id":"tenant_demo_jakarta","name":"Reference Place API Updated","address":"Jl. Test 2","region":"ID-BT"}}`)
	updatePlaceRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/places/"+createdPlace.ID, token, updatePlaceBody)
	if updatePlaceRecorder.Code != http.StatusOK {
		t.Fatalf("expected place update status 200, got %d body=%s", updatePlaceRecorder.Code, updatePlaceRecorder.Body.String())
	}
	if !strings.Contains(updatePlaceRecorder.Body.String(), `"name":"Reference Place API Updated"`) ||
		!strings.Contains(updatePlaceRecorder.Body.String(), `"address":"Jl. Test 2"`) {
		t.Fatalf("expected updated place fields, body=%s", updatePlaceRecorder.Body.String())
	}
	lockDownPlaceRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/places/"+createdPlace.ID+"/lock_down?tenant_id=tenant_demo_jakarta", token, nil)
	if lockDownPlaceRecorder.Code != http.StatusOK {
		t.Fatalf("expected place lock_down status 200, got %d body=%s", lockDownPlaceRecorder.Code, lockDownPlaceRecorder.Body.String())
	}
	if !strings.Contains(lockDownPlaceRecorder.Body.String(), `"resource_type":"PlaceAction"`) ||
		!strings.Contains(lockDownPlaceRecorder.Body.String(), `"action":"lock_down"`) {
		t.Fatalf("expected place action response, body=%s", lockDownPlaceRecorder.Body.String())
	}
	cancelPlaceLockdownRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/places/"+createdPlace.ID+"/cancel_lockdown?tenant_id=tenant_demo_jakarta", token, nil)
	if cancelPlaceLockdownRecorder.Code != http.StatusOK {
		t.Fatalf("expected place cancel_lockdown status 200, got %d body=%s", cancelPlaceLockdownRecorder.Code, cancelPlaceLockdownRecorder.Body.String())
	}
	deletePlaceRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/places/"+createdPlace.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletePlaceRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected place delete status 204, got %d body=%s", deletePlaceRecorder.Code, deletePlaceRecorder.Body.String())
	}
	deletedPlaceRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places/"+createdPlace.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletedPlaceRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted place detail status 404, got %d body=%s", deletedPlaceRecorder.Code, deletedPlaceRecorder.Body.String())
	}
	archivedPlaceRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places/"+createdPlace.ID+"?tenant_id=tenant_demo_jakarta&include_archived=true", token, nil)
	if archivedPlaceRecorder.Code != http.StatusOK {
		t.Fatalf("expected archived place detail status 200, got %d body=%s", archivedPlaceRecorder.Code, archivedPlaceRecorder.Body.String())
	}
	if !strings.Contains(archivedPlaceRecorder.Body.String(), `"status":"archived"`) ||
		!strings.Contains(archivedPlaceRecorder.Body.String(), `"archived_at"`) {
		t.Fatalf("expected archived place metadata, body=%s", archivedPlaceRecorder.Body.String())
	}
	archivedPlacesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places?tenant_id=tenant_demo_jakarta&status=archived", token, nil)
	if archivedPlacesRecorder.Code != http.StatusOK {
		t.Fatalf("expected archived places status 200, got %d body=%s", archivedPlacesRecorder.Code, archivedPlacesRecorder.Body.String())
	}
	if !strings.Contains(archivedPlacesRecorder.Body.String(), `"id":"`+createdPlace.ID+`"`) {
		t.Fatalf("expected archived place to be listable with status filter, body=%s", archivedPlacesRecorder.Body.String())
	}
	activePlacesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places?tenant_id=tenant_demo_jakarta", token, nil)
	if activePlacesRecorder.Code != http.StatusOK {
		t.Fatalf("expected active places status 200, got %d body=%s", activePlacesRecorder.Code, activePlacesRecorder.Body.String())
	}
	if strings.Contains(activePlacesRecorder.Body.String(), `"id":"`+createdPlace.ID+`"`) {
		t.Fatalf("expected active places to exclude archived place, body=%s", activePlacesRecorder.Body.String())
	}

	floorBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"Reference Floor API"}`)
	createFloorRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/floors", token, floorBody)
	if createFloorRecorder.Code != http.StatusCreated {
		t.Fatalf("expected floor create status 201, got %d body=%s", createFloorRecorder.Code, createFloorRecorder.Body.String())
	}
	var createdFloor struct {
		ID         string `json:"id"`
		TenantID   string `json:"tenant_id"`
		BuildingID string `json:"building_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(createFloorRecorder.Body.Bytes(), &createdFloor); err != nil {
		t.Fatalf("decode created floor: %v", err)
	}
	if createdFloor.ID == "" || createdFloor.BuildingID != "building_demo_001" || createdFloor.Name != "Reference Floor API" {
		t.Fatalf("expected created floor fields, got %#v body=%s", createdFloor, createFloorRecorder.Body.String())
	}
	getFloorRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/floors/"+createdFloor.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getFloorRecorder.Code != http.StatusOK {
		t.Fatalf("expected floor detail status 200, got %d body=%s", getFloorRecorder.Code, getFloorRecorder.Body.String())
	}
	updateFloorBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"Reference Floor API Updated"}`)
	updateFloorRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/floors/"+createdFloor.ID, token, updateFloorBody)
	if updateFloorRecorder.Code != http.StatusOK {
		t.Fatalf("expected floor update status 200, got %d body=%s", updateFloorRecorder.Code, updateFloorRecorder.Body.String())
	}
	if !strings.Contains(updateFloorRecorder.Body.String(), `"name":"Reference Floor API Updated"`) {
		t.Fatalf("expected updated floor fields, body=%s", updateFloorRecorder.Body.String())
	}
	areaBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","floor_id":"` + createdFloor.ID + `","name":"Reference Area API"}`)
	createAreaRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/areas", token, areaBody)
	if createAreaRecorder.Code != http.StatusCreated {
		t.Fatalf("expected area create status 201, got %d body=%s", createAreaRecorder.Code, createAreaRecorder.Body.String())
	}
	var createdArea struct {
		ID         string `json:"id"`
		BuildingID string `json:"building_id"`
		FloorID    string `json:"floor_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(createAreaRecorder.Body.Bytes(), &createdArea); err != nil {
		t.Fatalf("decode created area: %v", err)
	}
	if createdArea.ID == "" || createdArea.BuildingID != "building_demo_001" || createdArea.FloorID != createdFloor.ID || createdArea.Name != "Reference Area API" {
		t.Fatalf("expected created area fields, got %#v body=%s", createdArea, createAreaRecorder.Body.String())
	}
	getAreaRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/areas/"+createdArea.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getAreaRecorder.Code != http.StatusOK {
		t.Fatalf("expected area detail status 200, got %d body=%s", getAreaRecorder.Code, getAreaRecorder.Body.String())
	}
	updateAreaBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","floor_id":"` + createdFloor.ID + `","name":"Reference Area API Updated"}`)
	updateAreaRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/areas/"+createdArea.ID, token, updateAreaBody)
	if updateAreaRecorder.Code != http.StatusOK {
		t.Fatalf("expected area update status 200, got %d body=%s", updateAreaRecorder.Code, updateAreaRecorder.Body.String())
	}
	if !strings.Contains(updateAreaRecorder.Body.String(), `"name":"Reference Area API Updated"`) {
		t.Fatalf("expected updated area fields, body=%s", updateAreaRecorder.Body.String())
	}
	topologyLockBody := []byte(`{"lock":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_001","floor_id":"` + createdFloor.ID + `","area_id":"` + createdArea.ID + `","name":"Reference Topology Lock","gateway_id":"MP-GW-JKT-TOPO","kind":"office","status":"online"}}`)
	topologyLockRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/locks", token, topologyLockBody)
	if topologyLockRecorder.Code != http.StatusCreated {
		t.Fatalf("expected topology lock create status 201, got %d body=%s", topologyLockRecorder.Code, topologyLockRecorder.Body.String())
	}
	if !strings.Contains(topologyLockRecorder.Body.String(), `"area_id":"`+createdArea.ID+`"`) ||
		!strings.Contains(topologyLockRecorder.Body.String(), `"floor_id":"`+createdFloor.ID+`"`) {
		t.Fatalf("expected topology lock to use created floor and area, body=%s", topologyLockRecorder.Body.String())
	}
	deleteFloorRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/floors/"+createdFloor.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteFloorRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected floor delete status 204, got %d body=%s", deleteFloorRecorder.Code, deleteFloorRecorder.Body.String())
	}
	deletedFloorRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/floors/"+createdFloor.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletedFloorRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted floor detail status 404, got %d body=%s", deletedFloorRecorder.Code, deletedFloorRecorder.Body.String())
	}

	locksRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/locks?place_id=building_demo_001", token, nil)
	if locksRecorder.Code != http.StatusOK {
		t.Fatalf("expected locks status 200, got %d body=%s", locksRecorder.Code, locksRecorder.Body.String())
	}
	if !strings.Contains(locksRecorder.Body.String(), "door_jkt_001") {
		t.Fatalf("expected place-scoped locks to include door_jkt_001, body=%s", locksRecorder.Body.String())
	}
	if strings.Contains(locksRecorder.Body.String(), "door_jkt_014") {
		t.Fatalf("expected place-scoped locks to exclude other places, body=%s", locksRecorder.Body.String())
	}

	lockBody := []byte(`{"lock":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_001","floor_id":"floor_demo_001","area_id":"area_demo_001","name":"Reference Lock API","gateway_id":"MP-GW-JKT-TEST","kind":"office","status":"online"}}`)
	createLockRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/locks", token, lockBody)
	if createLockRecorder.Code != http.StatusCreated {
		t.Fatalf("expected lock create status 201, got %d body=%s", createLockRecorder.Code, createLockRecorder.Body.String())
	}
	var createdLock struct {
		ID         string `json:"id"`
		BuildingID string `json:"building_id"`
		FloorID    string `json:"floor_id"`
		AreaID     string `json:"area_id"`
		Name       string `json:"name"`
		GatewayID  string `json:"gateway_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(createLockRecorder.Body.Bytes(), &createdLock); err != nil {
		t.Fatalf("decode created lock: %v", err)
	}
	if createdLock.ID == "" || createdLock.BuildingID != "building_demo_001" || createdLock.Name != "Reference Lock API" {
		t.Fatalf("expected created lock fields, got %#v body=%s", createdLock, createLockRecorder.Body.String())
	}
	getLockRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/locks/"+createdLock.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getLockRecorder.Code != http.StatusOK {
		t.Fatalf("expected lock detail status 200, got %d body=%s", getLockRecorder.Code, getLockRecorder.Body.String())
	}
	updateLockBody := []byte(`{"lock":{"tenant_id":"tenant_demo_jakarta","name":"Reference Lock API Updated","gateway_id":"MP-GW-JKT-UPDATED","status":"offline"}}`)
	updateLockRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/locks/"+createdLock.ID, token, updateLockBody)
	if updateLockRecorder.Code != http.StatusOK {
		t.Fatalf("expected lock update status 200, got %d body=%s", updateLockRecorder.Code, updateLockRecorder.Body.String())
	}
	if !strings.Contains(updateLockRecorder.Body.String(), `"name":"Reference Lock API Updated"`) ||
		!strings.Contains(updateLockRecorder.Body.String(), `"gateway_id":"MP-GW-JKT-UPDATED"`) ||
		!strings.Contains(updateLockRecorder.Body.String(), `"status":"offline"`) {
		t.Fatalf("expected updated lock fields, body=%s", updateLockRecorder.Body.String())
	}
	for _, action := range []string{"unlock", "lock_down", "cancel_lockdown"} {
		actionRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/locks/"+createdLock.ID+"/"+action+"?tenant_id=tenant_demo_jakarta", token, nil)
		if actionRecorder.Code != http.StatusOK {
			t.Fatalf("expected lock %s status 200, got %d body=%s", action, actionRecorder.Code, actionRecorder.Body.String())
		}
		if !strings.Contains(actionRecorder.Body.String(), `"resource_type":"LockAction"`) ||
			!strings.Contains(actionRecorder.Body.String(), `"action":"`+action+`"`) ||
			!strings.Contains(actionRecorder.Body.String(), `"lock_id":"`+createdLock.ID+`"`) {
			t.Fatalf("expected lock action response for %s, body=%s", action, actionRecorder.Body.String())
		}
	}
	deleteLockRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/locks/"+createdLock.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteLockRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected lock delete status 204, got %d body=%s", deleteLockRecorder.Code, deleteLockRecorder.Body.String())
	}
	deletedLockRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/locks/"+createdLock.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletedLockRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted lock detail status 404, got %d body=%s", deletedLockRecorder.Code, deletedLockRecorder.Body.String())
	}

	groupBody := []byte(`{"group":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_001","name":"Reference Group API","description":"Created through reference groups","member_ids":["usr_1001"]}}`)
	createGroupRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/groups", token, groupBody)
	if createGroupRecorder.Code != http.StatusCreated {
		t.Fatalf("expected group create status 201, got %d body=%s", createGroupRecorder.Code, createGroupRecorder.Body.String())
	}
	var createdGroup struct {
		ID          string   `json:"id"`
		BuildingID  string   `json:"building_id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Members     []string `json:"members"`
	}
	if err := json.Unmarshal(createGroupRecorder.Body.Bytes(), &createdGroup); err != nil {
		t.Fatalf("decode created group: %v", err)
	}
	if createdGroup.ID == "" || createdGroup.BuildingID != "building_demo_001" || createdGroup.Name != "Reference Group API" || len(createdGroup.Members) != 1 {
		t.Fatalf("expected created reference group fields, got %#v", createdGroup)
	}

	getGroupRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/groups/"+createdGroup.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getGroupRecorder.Code != http.StatusOK {
		t.Fatalf("expected group detail status 200, got %d body=%s", getGroupRecorder.Code, getGroupRecorder.Body.String())
	}
	if !strings.Contains(getGroupRecorder.Body.String(), `"name":"Reference Group API"`) {
		t.Fatalf("expected group detail to return created group, body=%s", getGroupRecorder.Body.String())
	}

	updateGroupBody := []byte(`{"group":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_001","name":"Reference Group API Updated","description":"Updated through reference groups","members":["usr_1001","usr_1001"],"login_enabled":false,"geofence_restriction_enabled":true,"geofence_restriction_radius":250,"primary_device_restriction_enabled":true,"reader_restriction_enabled":true,"time_restriction_enabled":true,"tap_to_access_restriction_enabled":false}}`)
	updateGroupRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/groups/"+createdGroup.ID, token, updateGroupBody)
	if updateGroupRecorder.Code != http.StatusOK {
		t.Fatalf("expected group update status 200, got %d body=%s", updateGroupRecorder.Code, updateGroupRecorder.Body.String())
	}
	if !strings.Contains(updateGroupRecorder.Body.String(), `"name":"Reference Group API Updated"`) ||
		!strings.Contains(updateGroupRecorder.Body.String(), `"members":["usr_1001"]`) ||
		!strings.Contains(updateGroupRecorder.Body.String(), `"login_enabled":false`) ||
		!strings.Contains(updateGroupRecorder.Body.String(), `"geofence_restriction_enabled":true`) ||
		!strings.Contains(updateGroupRecorder.Body.String(), `"reader_restriction_enabled":true`) ||
		!strings.Contains(updateGroupRecorder.Body.String(), `"tap_to_access_restriction_enabled":false`) {
		t.Fatalf("expected group update response to normalize fields, body=%s", updateGroupRecorder.Body.String())
	}

	groupLinkBody := []byte(`{"group_link":{"tenant_id":"tenant_demo_jakarta","group_id":"` + createdGroup.ID + `","name":"Reference Visitor Link","email":"visitor@example.com","quick_response_code_type":"online","valid_until":"2099-05-01T10:00:00Z"}}`)
	createGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/group_links", token, groupLinkBody)
	if createGroupLinkRecorder.Code != http.StatusCreated {
		t.Fatalf("expected group link create status 201, got %d body=%s", createGroupLinkRecorder.Code, createGroupLinkRecorder.Body.String())
	}
	var createdGroupLink struct {
		ID                     string `json:"id"`
		ResourceType           string `json:"resource_type"`
		GroupID                string `json:"group_id"`
		GroupName              string `json:"group_name"`
		Name                   string `json:"name"`
		Email                  string `json:"email"`
		LinkEnabled            bool   `json:"link_enabled"`
		QuickResponseCodeType  string `json:"quick_response_code_type"`
		ValidUntil             string `json:"valid_until"`
		Secret                 string `json:"secret"`
		QuickResponseCodeToken string `json:"quick_response_code_token"`
	}
	if err := json.Unmarshal(createGroupLinkRecorder.Body.Bytes(), &createdGroupLink); err != nil {
		t.Fatalf("decode created group link: %v", err)
	}
	if createdGroupLink.ID == "" ||
		createdGroupLink.ResourceType != "GroupLink" ||
		createdGroupLink.GroupID != createdGroup.ID ||
		createdGroupLink.GroupName != "Reference Group API Updated" ||
		createdGroupLink.Email != "visitor@example.com" ||
		createdGroupLink.Secret == "" ||
		createdGroupLink.QuickResponseCodeToken == "" {
		t.Fatalf("expected created group link fields, got %#v body=%s", createdGroupLink, createGroupLinkRecorder.Body.String())
	}

	verifyGroupLinkBody := []byte(`{"tenant_id":"tenant_demo_jakarta","token":"` + createdGroupLink.Secret + `"}`)
	verifyGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/group_links/verify", "", verifyGroupLinkBody)
	if verifyGroupLinkRecorder.Code != http.StatusOK {
		t.Fatalf("expected group link token verify status 200, got %d body=%s", verifyGroupLinkRecorder.Code, verifyGroupLinkRecorder.Body.String())
	}
	var verifiedGroupLink struct {
		Valid      bool   `json:"valid"`
		Status     string `json:"status"`
		VerifiedAt string `json:"verified_at"`
		ClaimedAt  string `json:"claimed_at"`
		GroupLink  struct {
			ID                     string `json:"id"`
			GroupName              string `json:"group_name"`
			LastUsedAt             string `json:"last_used_at"`
			Secret                 string `json:"secret"`
			QuickResponseCodeToken string `json:"quick_response_code_token"`
		} `json:"group_link"`
	}
	if err := json.Unmarshal(verifyGroupLinkRecorder.Body.Bytes(), &verifiedGroupLink); err != nil {
		t.Fatalf("decode verified group link: %v", err)
	}
	if !verifiedGroupLink.Valid ||
		verifiedGroupLink.Status != "valid" ||
		verifiedGroupLink.VerifiedAt == "" ||
		verifiedGroupLink.ClaimedAt == "" ||
		verifiedGroupLink.GroupLink.ID != createdGroupLink.ID ||
		verifiedGroupLink.GroupLink.GroupName != "Reference Group API Updated" ||
		verifiedGroupLink.GroupLink.LastUsedAt == "" ||
		verifiedGroupLink.GroupLink.Secret != "" ||
		verifiedGroupLink.GroupLink.QuickResponseCodeToken != "" {
		t.Fatalf("expected sanitized verified group link response, got %#v body=%s", verifiedGroupLink, verifyGroupLinkRecorder.Body.String())
	}
	if strings.Contains(verifyGroupLinkRecorder.Body.String(), createdGroupLink.Secret) ||
		strings.Contains(verifyGroupLinkRecorder.Body.String(), createdGroupLink.QuickResponseCodeToken) {
		t.Fatalf("expected verified group link response to omit tokens, body=%s", verifyGroupLinkRecorder.Body.String())
	}
	assertReferenceAuditLogActorRole(
		t,
		router,
		token,
		"reference_group_link_claimed",
		"system",
		"system",
		"group_link_id="+createdGroupLink.ID,
		"group_id="+createdGroup.ID,
		"email=visitor@example.com",
	)
	verifyGroupLinkQRRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/group_links/verify?quick_response_code_token="+createdGroupLink.QuickResponseCodeToken, "", nil)
	if verifyGroupLinkQRRecorder.Code != http.StatusOK {
		t.Fatalf("expected group link QR token verify status 200, got %d body=%s", verifyGroupLinkQRRecorder.Code, verifyGroupLinkQRRecorder.Body.String())
	}
	invalidGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/group_links/verify?token=gls_invalid", "", nil)
	if invalidGroupLinkRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected invalid group link token verify status 404, got %d body=%s", invalidGroupLinkRecorder.Code, invalidGroupLinkRecorder.Body.String())
	}

	getGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/group_links/"+createdGroupLink.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getGroupLinkRecorder.Code != http.StatusOK {
		t.Fatalf("expected group link detail status 200, got %d body=%s", getGroupLinkRecorder.Code, getGroupLinkRecorder.Body.String())
	}
	if !strings.Contains(getGroupLinkRecorder.Body.String(), `"name":"Reference Visitor Link"`) ||
		!strings.Contains(getGroupLinkRecorder.Body.String(), `"group_name":"Reference Group API Updated"`) {
		t.Fatalf("expected group link detail to return created link, body=%s", getGroupLinkRecorder.Body.String())
	}

	updateGroupLinkBody := []byte(`{"group_link":{"tenant_id":"tenant_demo_jakarta","name":"Reference Visitor Link Updated","email":"visitor.updated@example.com","link_enabled":false,"quick_response_code_type":"offline","valid_until":"2026-05-02T10:00:00Z"}}`)
	updateGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/group_links/"+createdGroupLink.ID, token, updateGroupLinkBody)
	if updateGroupLinkRecorder.Code != http.StatusOK {
		t.Fatalf("expected group link update status 200, got %d body=%s", updateGroupLinkRecorder.Code, updateGroupLinkRecorder.Body.String())
	}
	if err := json.Unmarshal(updateGroupLinkRecorder.Body.Bytes(), &createdGroupLink); err != nil {
		t.Fatalf("decode updated group link: %v", err)
	}
	if createdGroupLink.Name != "Reference Visitor Link Updated" ||
		createdGroupLink.Email != "visitor.updated@example.com" ||
		createdGroupLink.LinkEnabled ||
		createdGroupLink.QuickResponseCodeType != "offline" ||
		createdGroupLink.ValidUntil != "2026-05-02T10:00:00Z" {
		t.Fatalf("expected updated group link fields, got %#v body=%s", createdGroupLink, updateGroupLinkRecorder.Body.String())
	}
	disabledGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/group_links/verify", "", verifyGroupLinkBody)
	if disabledGroupLinkRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected disabled group link token verify status 403, got %d body=%s", disabledGroupLinkRecorder.Code, disabledGroupLinkRecorder.Body.String())
	}

	groupLinksRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/group_links?group_id="+createdGroup.ID+"&query=visitor", token, nil)
	if groupLinksRecorder.Code != http.StatusOK {
		t.Fatalf("expected group links status 200, got %d body=%s", groupLinksRecorder.Code, groupLinksRecorder.Body.String())
	}
	if !strings.Contains(groupLinksRecorder.Body.String(), `"id":"`+createdGroupLink.ID+`"`) ||
		!strings.Contains(groupLinksRecorder.Body.String(), `"resource_type":"GroupLink"`) {
		t.Fatalf("expected created group link to be listable, body=%s", groupLinksRecorder.Body.String())
	}

	deleteGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/group_links/"+createdGroupLink.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteGroupLinkRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected group link delete status 204, got %d body=%s", deleteGroupLinkRecorder.Code, deleteGroupLinkRecorder.Body.String())
	}
	removedGroupLinksRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/group_links?group_id="+createdGroup.ID, token, nil)
	if removedGroupLinksRecorder.Code != http.StatusOK {
		t.Fatalf("expected group links status 200 after delete, got %d body=%s", removedGroupLinksRecorder.Code, removedGroupLinksRecorder.Body.String())
	}
	if strings.Contains(removedGroupLinksRecorder.Body.String(), `"id":"`+createdGroupLink.ID+`"`) {
		t.Fatalf("expected deleted group link to be absent, body=%s", removedGroupLinksRecorder.Body.String())
	}

	deleteGroupRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/groups/"+createdGroup.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteGroupRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected group delete status 204, got %d body=%s", deleteGroupRecorder.Code, deleteGroupRecorder.Body.String())
	}
	deletedGroupRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/groups/"+createdGroup.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletedGroupRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted group detail status 404, got %d body=%s", deletedGroupRecorder.Code, deletedGroupRecorder.Body.String())
	}

	shareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"guest@example.com","place_id":"building_demo_001","area_id":"area_demo_001","lock_id":"door_jkt_001","grantee_name":"Guest Visitor","grantee_phone":"+628110000001","mobile_model":"iPhone 15","pass_type":"visitor","valid_until":"2026-05-01T10:00:00Z"}}`)
	shareRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/shares", token, shareBody)
	if shareRecorder.Code != http.StatusCreated {
		t.Fatalf("expected share create status 201, got %d body=%s", shareRecorder.Code, shareRecorder.Body.String())
	}
	if !strings.Contains(shareRecorder.Body.String(), `"role_id":"role_group_access"`) {
		t.Fatalf("expected share to map to group access role, body=%s", shareRecorder.Body.String())
	}
	var createdShare struct {
		ID           string `json:"id"`
		Email        string `json:"email"`
		GroupID      string `json:"group_id"`
		RoleID       string `json:"role_id"`
		PlaceID      string `json:"place_id"`
		AreaID       string `json:"area_id"`
		LockID       string `json:"lock_id"`
		GranteeName  string `json:"grantee_name"`
		GranteePhone string `json:"grantee_phone"`
		MobileModel  string `json:"mobile_model"`
		PassType     string `json:"pass_type"`
		AuthorizedAt string `json:"authorized_at"`
		ValidUntil   string `json:"valid_until"`
	}
	if err := json.Unmarshal(shareRecorder.Body.Bytes(), &createdShare); err != nil {
		t.Fatalf("decode created share: %v", err)
	}
	if createdShare.ID == "" {
		t.Fatalf("expected created share id, body=%s", shareRecorder.Body.String())
	}
	if createdShare.AreaID != "area_demo_001" ||
		createdShare.GranteeName != "Guest Visitor" ||
		createdShare.GranteePhone != "+628110000001" ||
		createdShare.MobileModel != "iPhone 15" ||
		createdShare.PassType != "visitor" ||
		createdShare.AuthorizedAt == "" {
		t.Fatalf("expected created share to preserve temporary access fields, got %#v body=%s", createdShare, shareRecorder.Body.String())
	}
	getShareRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/shares/"+createdShare.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getShareRecorder.Code != http.StatusOK {
		t.Fatalf("expected share detail status 200, got %d body=%s", getShareRecorder.Code, getShareRecorder.Body.String())
	}
	if !strings.Contains(getShareRecorder.Body.String(), `"id":"`+createdShare.ID+`"`) {
		t.Fatalf("expected share detail to include created share, body=%s", getShareRecorder.Body.String())
	}
	updateShareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"guest.updated@example.com","group_id":"ug_common_office_jkt","place_id":"building_demo_001","lock_id":"door_jkt_001","valid_until":"2026-05-02T10:00:00Z"}}`)
	updateShareRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/shares/"+createdShare.ID, token, updateShareBody)
	if updateShareRecorder.Code != http.StatusOK {
		t.Fatalf("expected share update status 200, got %d body=%s", updateShareRecorder.Code, updateShareRecorder.Body.String())
	}
	if !strings.Contains(updateShareRecorder.Body.String(), `"email":"guest.updated@example.com"`) ||
		!strings.Contains(updateShareRecorder.Body.String(), `"group_id":"ug_common_office_jkt"`) ||
		!strings.Contains(updateShareRecorder.Body.String(), `"valid_until":"2026-05-02T10:00:00Z"`) {
		t.Fatalf("expected updated share fields, body=%s", updateShareRecorder.Body.String())
	}
	deleteShareRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/shares/"+createdShare.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteShareRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected share delete status 204, got %d body=%s", deleteShareRecorder.Code, deleteShareRecorder.Body.String())
	}
	deletedShareRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/shares/"+createdShare.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletedShareRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted share detail status 404, got %d body=%s", deletedShareRecorder.Code, deletedShareRecorder.Body.String())
	}

	doorGroupsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/door_groups?tenant_id=tenant_demo_jakarta&limit=1", token, nil)
	if doorGroupsRecorder.Code != http.StatusOK {
		t.Fatalf("expected door groups extension status 200, got %d body=%s", doorGroupsRecorder.Code, doorGroupsRecorder.Body.String())
	}
	if got := doorGroupsRecorder.Header().Get("X-Collection-Range"); !strings.HasPrefix(got, "items 0-0/") {
		t.Fatalf("expected door groups extension collection range, got %q", got)
	}
	if !strings.Contains(doorGroupsRecorder.Body.String(), `"pagination"`) ||
		!strings.Contains(doorGroupsRecorder.Body.String(), `"id":`) {
		t.Fatalf("expected door groups extension wrapper payload, body=%s", doorGroupsRecorder.Body.String())
	}

	groupLockBody := []byte(`{"group_lock":{"tenant_id":"tenant_demo_jakarta","group_id":"dg_1001","lock_id":"door_jkt_001"}}`)
	createGroupLockRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/group_locks", token, groupLockBody)
	if createGroupLockRecorder.Code != http.StatusCreated {
		t.Fatalf("expected group lock create status 201, got %d body=%s", createGroupLockRecorder.Code, createGroupLockRecorder.Body.String())
	}
	if !strings.Contains(createGroupLockRecorder.Body.String(), `"id":"dg_1001:door_jkt_001"`) ||
		!strings.Contains(createGroupLockRecorder.Body.String(), `"group_id":"dg_1001"`) ||
		!strings.Contains(createGroupLockRecorder.Body.String(), `"lock_id":"door_jkt_001"`) {
		t.Fatalf("expected group lock wrapper fields, body=%s", createGroupLockRecorder.Body.String())
	}

	groupZonesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/group_zones?group_id=dg_1001&zone_id=area_demo_001", token, nil)
	if groupZonesRecorder.Code != http.StatusOK {
		t.Fatalf("expected group zones status 200, got %d body=%s", groupZonesRecorder.Code, groupZonesRecorder.Body.String())
	}
	if !strings.Contains(groupZonesRecorder.Body.String(), `"id":"dg_1001:area_demo_001"`) ||
		!strings.Contains(groupZonesRecorder.Body.String(), `"zone_id":"area_demo_001"`) ||
		!strings.Contains(groupZonesRecorder.Body.String(), `"place_id":"building_demo_001"`) {
		t.Fatalf("expected group zone wrapper fields, body=%s", groupZonesRecorder.Body.String())
	}

	groupLocksRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/group_locks?group_id=dg_1001&lock_id=door_jkt_001", token, nil)
	if groupLocksRecorder.Code != http.StatusOK {
		t.Fatalf("expected group locks status 200, got %d body=%s", groupLocksRecorder.Code, groupLocksRecorder.Body.String())
	}
	if !strings.Contains(groupLocksRecorder.Body.String(), `"id":"dg_1001:door_jkt_001"`) {
		t.Fatalf("expected created group lock to be listable, body=%s", groupLocksRecorder.Body.String())
	}

	deleteGroupLockRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/group_locks/dg_1001:door_jkt_001?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteGroupLockRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected group lock delete status 204, got %d body=%s", deleteGroupLockRecorder.Code, deleteGroupLockRecorder.Body.String())
	}

	removedGroupLocksRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/group_locks?group_id=dg_1001&lock_id=door_jkt_001", token, nil)
	if removedGroupLocksRecorder.Code != http.StatusOK {
		t.Fatalf("expected group locks status 200 after delete, got %d body=%s", removedGroupLocksRecorder.Code, removedGroupLocksRecorder.Body.String())
	}
	if strings.Contains(removedGroupLocksRecorder.Body.String(), `"id":"dg_1001:door_jkt_001"`) {
		t.Fatalf("expected deleted group lock to be absent, body=%s", removedGroupLocksRecorder.Body.String())
	}
}

func TestReferenceResourceEndpointsMapCardsAndAssignments(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-card-api-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	cardsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/cards?user_id=usr_1001", token, nil)
	if cardsRecorder.Code != http.StatusOK {
		t.Fatalf("expected cards status 200, got %d body=%s", cardsRecorder.Code, cardsRecorder.Body.String())
	}
	if got := cardsRecorder.Header().Get("X-Collection-Range"); !strings.HasPrefix(got, "items 0-") {
		t.Fatalf("expected cards collection range header, got %q", got)
	}
	if !strings.Contains(cardsRecorder.Body.String(), `"resource_type":"Card"`) ||
		!strings.Contains(cardsRecorder.Body.String(), `"status":"activated"`) ||
		!strings.Contains(cardsRecorder.Body.String(), `"credential_kind":"google_wallet"`) ||
		!strings.Contains(cardsRecorder.Body.String(), `"user_id":"usr_1001"`) {
		t.Fatalf("expected card wrapper fields, body=%s", cardsRecorder.Body.String())
	}

	assignmentsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/card_assignments?user_id=usr_1001", token, nil)
	if assignmentsRecorder.Code != http.StatusOK {
		t.Fatalf("expected card assignments status 200, got %d body=%s", assignmentsRecorder.Code, assignmentsRecorder.Body.String())
	}
	if !strings.Contains(assignmentsRecorder.Body.String(), `"resource_type":"CardAssignment"`) ||
		!strings.Contains(assignmentsRecorder.Body.String(), `"assignee_type":"User"`) {
		t.Fatalf("expected card assignment wrapper fields, body=%s", assignmentsRecorder.Body.String())
	}

	createCardBody := []byte(`{"card":{"tenant_id":"tenant_demo_jakarta","template_id":"wpt_employee_demo","uid":"ABCDEF123456"}}`)
	createCardRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards", token, createCardBody)
	if createCardRecorder.Code != http.StatusCreated {
		t.Fatalf("expected card create status 201, got %d body=%s", createCardRecorder.Code, createCardRecorder.Body.String())
	}
	var createdCard struct {
		ID             string `json:"id"`
		Status         string `json:"status"`
		UID            string `json:"uid"`
		Provider       string `json:"provider"`
		CredentialKind string `json:"credential_kind"`
		SaveLink       string `json:"save_link"`
	}
	if err := json.Unmarshal(createCardRecorder.Body.Bytes(), &createdCard); err != nil {
		t.Fatalf("expected created card json: %v body=%s", err, createCardRecorder.Body.String())
	}
	if createdCard.ID == "" || createdCard.Status != "unassigned" || createdCard.UID != "ABCDEF123456" ||
		createdCard.CredentialKind != "physical_card" || createdCard.Provider != "physical_card" || createdCard.SaveLink != "" {
		t.Fatalf("expected unassigned created card with uid, got %+v body=%s", createdCard, createCardRecorder.Body.String())
	}

	cardDetailRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/cards/"+createdCard.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if cardDetailRecorder.Code != http.StatusOK {
		t.Fatalf("expected card detail status 200, got %d body=%s", cardDetailRecorder.Code, cardDetailRecorder.Body.String())
	}
	if !strings.Contains(cardDetailRecorder.Body.String(), `"id":"`+createdCard.ID+`"`) ||
		!strings.Contains(cardDetailRecorder.Body.String(), `"resource_type":"Card"`) ||
		!strings.Contains(cardDetailRecorder.Body.String(), `"uid":"ABCDEF123456"`) {
		t.Fatalf("expected card detail wrapper fields, body=%s", cardDetailRecorder.Body.String())
	}

	createAssignmentBody := []byte(`{"card_assignment":{"tenant_id":"tenant_demo_jakarta","card_id":"` + createdCard.ID + `","assignee_type":"User","assignee_id":"usr_1001"}}`)
	createAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/card_assignments", token, createAssignmentBody)
	if createAssignmentRecorder.Code != http.StatusOK {
		t.Fatalf("expected card assignment create status 200, got %d body=%s", createAssignmentRecorder.Code, createAssignmentRecorder.Body.String())
	}
	if !strings.Contains(createAssignmentRecorder.Body.String(), `"card_id":"`+createdCard.ID+`"`) ||
		!strings.Contains(createAssignmentRecorder.Body.String(), `"status":"activated"`) ||
		!strings.Contains(createAssignmentRecorder.Body.String(), `"user_id":"usr_1001"`) {
		t.Fatalf("expected activated card assignment wrapper, body=%s", createAssignmentRecorder.Body.String())
	}

	assignmentDetailRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/card_assignments/ca_"+createdCard.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if assignmentDetailRecorder.Code != http.StatusOK {
		t.Fatalf("expected card assignment detail status 200, got %d body=%s", assignmentDetailRecorder.Code, assignmentDetailRecorder.Body.String())
	}
	if !strings.Contains(assignmentDetailRecorder.Body.String(), `"id":"ca_`+createdCard.ID+`"`) ||
		!strings.Contains(assignmentDetailRecorder.Body.String(), `"resource_type":"CardAssignment"`) ||
		!strings.Contains(assignmentDetailRecorder.Body.String(), `"card_id":"`+createdCard.ID+`"`) {
		t.Fatalf("expected card assignment detail wrapper fields, body=%s", assignmentDetailRecorder.Body.String())
	}

	deassignRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards/"+createdCard.ID+"/deassign?tenant_id=tenant_demo_jakarta", token, nil)
	if deassignRecorder.Code != http.StatusOK {
		t.Fatalf("expected card deassign status 200, got %d body=%s", deassignRecorder.Code, deassignRecorder.Body.String())
	}
	if !strings.Contains(deassignRecorder.Body.String(), `"status":"unassigned"`) ||
		strings.Contains(deassignRecorder.Body.String(), `"user_id"`) {
		t.Fatalf("expected deassigned card wrapper, body=%s", deassignRecorder.Body.String())
	}

	revokeRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards/"+createdCard.ID+"/revoke?tenant_id=tenant_demo_jakarta", token, nil)
	if revokeRecorder.Code != http.StatusOK {
		t.Fatalf("expected card revoke status 200, got %d body=%s", revokeRecorder.Code, revokeRecorder.Body.String())
	}
	if !strings.Contains(revokeRecorder.Body.String(), `"status":"revoked"`) {
		t.Fatalf("expected revoked card wrapper, body=%s", revokeRecorder.Body.String())
	}

	assignRevokedBody := []byte(`{"card":{"tenant_id":"tenant_demo_jakarta","assignee_type":"User","assignee_id":"usr_1001"}}`)
	assignRevokedRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards/"+createdCard.ID+"/assign", token, assignRevokedBody)
	if assignRevokedRecorder.Code != http.StatusConflict {
		t.Fatalf("expected revoked card assignment status 409, got %d body=%s", assignRevokedRecorder.Code, assignRevokedRecorder.Body.String())
	}

	missingCardRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/cards/missing_card?tenant_id=tenant_demo_jakarta", token, nil)
	if missingCardRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing card detail status 404, got %d body=%s", missingCardRecorder.Code, missingCardRecorder.Body.String())
	}

	missingAssignmentRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/card_assignments/ca_missing_card?tenant_id=tenant_demo_jakarta", token, nil)
	if missingAssignmentRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing card assignment detail status 404, got %d body=%s", missingAssignmentRecorder.Code, missingAssignmentRecorder.Body.String())
	}
}

func TestReferenceResourceEndpointsMapControllersAndReaders(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-hardware-api-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	controllersRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/controllers?place_id=building_demo_001", token, nil)
	if controllersRecorder.Code != http.StatusOK {
		t.Fatalf("expected controllers status 200, got %d body=%s", controllersRecorder.Code, controllersRecorder.Body.String())
	}
	if !strings.Contains(controllersRecorder.Body.String(), `"resource_type":"Controller"`) ||
		!strings.Contains(controllersRecorder.Body.String(), `"device_id":"MP-GW-JKT-0001"`) {
		t.Fatalf("expected controller wrapper fields, body=%s", controllersRecorder.Body.String())
	}
	if strings.Contains(controllersRecorder.Body.String(), "MP-GW-BTN-0001") {
		t.Fatalf("expected place-scoped controllers to exclude factory gateway, body=%s", controllersRecorder.Body.String())
	}

	readersRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/readers?place_id=building_demo_001", token, nil)
	if readersRecorder.Code != http.StatusOK {
		t.Fatalf("expected readers status 200, got %d body=%s", readersRecorder.Code, readersRecorder.Body.String())
	}
	if !strings.Contains(readersRecorder.Body.String(), `"resource_type":"Reader"`) ||
		!strings.Contains(readersRecorder.Body.String(), `"controller_id":"gw_demo_001"`) {
		t.Fatalf("expected reader wrapper fields, body=%s", readersRecorder.Body.String())
	}
	if strings.Contains(readersRecorder.Body.String(), "RD-BTN-100") {
		t.Fatalf("expected place-scoped readers to exclude factory reader, body=%s", readersRecorder.Body.String())
	}

	terminalsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/terminals?place_id=building_demo_001", token, nil)
	if terminalsRecorder.Code != http.StatusOK {
		t.Fatalf("expected terminals status 200, got %d body=%s", terminalsRecorder.Code, terminalsRecorder.Body.String())
	}
	if !strings.Contains(terminalsRecorder.Body.String(), `"resource_type":"Terminal"`) ||
		!strings.Contains(terminalsRecorder.Body.String(), `"reader_id":"gdv_demo_001"`) ||
		!strings.Contains(terminalsRecorder.Body.String(), `"place":{"id":"building_demo_001"`) {
		t.Fatalf("expected terminal wrapper fields, body=%s", terminalsRecorder.Body.String())
	}
	if strings.Contains(terminalsRecorder.Body.String(), "RD-BTN-100") {
		t.Fatalf("expected place-scoped terminals to exclude factory terminal, body=%s", terminalsRecorder.Body.String())
	}

	controllerInventoryBody := []byte(`{"tenant_id":"tenant_demo_jakarta","items":[{"serial_number":"MP-GW-JKT-9001","product_type":"gateway","batch_code":"reference-test","source":"test"}]}`)
	controllerInventoryRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/serial-inventory/import", token, controllerInventoryBody)
	if controllerInventoryRecorder.Code != http.StatusCreated {
		t.Fatalf("expected controller serial inventory import status 201, got %d body=%s", controllerInventoryRecorder.Code, controllerInventoryRecorder.Body.String())
	}
	assignControllerBody := []byte(`{"controller":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_001","device_capacity":4}}`)
	assignControllerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/MP-GW-JKT-9001/assign", token, assignControllerBody)
	if assignControllerRecorder.Code != http.StatusCreated {
		t.Fatalf("expected controller assign status 201, got %d body=%s", assignControllerRecorder.Code, assignControllerRecorder.Body.String())
	}
	var assignedController struct {
		ID           string   `json:"id"`
		ResourceType string   `json:"resource_type"`
		DeviceID     string   `json:"device_id"`
		PlaceID      string   `json:"place_id"`
		LockIDs      []string `json:"lock_ids"`
	}
	if err := json.Unmarshal(assignControllerRecorder.Body.Bytes(), &assignedController); err != nil {
		t.Fatalf("decode assigned controller: %v", err)
	}
	if assignedController.ID == "" || assignedController.ResourceType != "Controller" || assignedController.DeviceID != "MP-GW-JKT-9001" || assignedController.PlaceID != "building_demo_001" {
		t.Fatalf("expected assigned controller wrapper fields, got %#v body=%s", assignedController, assignControllerRecorder.Body.String())
	}

	bindControllerLockBody := []byte(`{"tenant_id":"tenant_demo_jakarta","lock_id":"door_jkt_001"}`)
	bindControllerLockRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/"+assignedController.ID+"/locks", token, bindControllerLockBody)
	if bindControllerLockRecorder.Code != http.StatusOK {
		t.Fatalf("expected controller lock bind status 200, got %d body=%s", bindControllerLockRecorder.Code, bindControllerLockRecorder.Body.String())
	}
	if !strings.Contains(bindControllerLockRecorder.Body.String(), `"lock_ids":["door_jkt_001"]`) {
		t.Fatalf("expected bound lock id in controller wrapper, body=%s", bindControllerLockRecorder.Body.String())
	}

	publishControllerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/"+assignedController.ID+"/config/publish", token, []byte(`{"controller":{"tenant_id":"tenant_demo_jakarta","version":"reference-test-v1"}}`))
	if publishControllerRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected controller config publish status 202, got %d body=%s", publishControllerRecorder.Code, publishControllerRecorder.Body.String())
	}
	if !strings.Contains(publishControllerRecorder.Body.String(), `"command":"update_config"`) ||
		!strings.Contains(publishControllerRecorder.Body.String(), `"status":"queued"`) {
		t.Fatalf("expected controller config ack, body=%s", publishControllerRecorder.Body.String())
	}
	rebootControllerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/"+assignedController.ID+"/reboot?tenant_id=tenant_demo_jakarta", token, nil)
	if rebootControllerRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected controller reboot status 202, got %d body=%s", rebootControllerRecorder.Code, rebootControllerRecorder.Body.String())
	}
	if !strings.Contains(rebootControllerRecorder.Body.String(), `"command":"reboot"`) {
		t.Fatalf("expected controller reboot ack, body=%s", rebootControllerRecorder.Body.String())
	}

	readerInventoryBody := []byte(`{"tenant_id":"tenant_demo_jakarta","items":[{"serial_number":"RD-JKT-9001","product_type":"reader","batch_code":"reference-test","source":"test"}]}`)
	readerInventoryRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/serial-inventory/import", token, readerInventoryBody)
	if readerInventoryRecorder.Code != http.StatusCreated {
		t.Fatalf("expected reader serial inventory import status 201, got %d body=%s", readerInventoryRecorder.Code, readerInventoryRecorder.Body.String())
	}
	assignReaderBody := []byte(`{"reader":{"tenant_id":"tenant_demo_jakarta","controller_id":"` + assignedController.ID + `","protocol":"osdp_v2","status":"online"}}`)
	assignReaderRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/readers/RD-JKT-9001/assign", token, assignReaderBody)
	if assignReaderRecorder.Code != http.StatusCreated {
		t.Fatalf("expected reader assign status 201, got %d body=%s", assignReaderRecorder.Code, assignReaderRecorder.Body.String())
	}
	var assignedReader struct {
		ID           string `json:"id"`
		ResourceType string `json:"resource_type"`
		ControllerID string `json:"controller_id"`
		DeviceID     string `json:"device_id"`
	}
	if err := json.Unmarshal(assignReaderRecorder.Body.Bytes(), &assignedReader); err != nil {
		t.Fatalf("decode assigned reader: %v", err)
	}
	if assignedReader.ID == "" || assignedReader.ResourceType != "Reader" || assignedReader.ControllerID != assignedController.ID || assignedReader.DeviceID != "RD-JKT-9001" {
		t.Fatalf("expected assigned reader wrapper fields, got %#v body=%s", assignedReader, assignReaderRecorder.Body.String())
	}
	terminalDetailRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/terminals/terminal_"+assignedReader.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if terminalDetailRecorder.Code != http.StatusOK {
		t.Fatalf("expected terminal detail status 200, got %d body=%s", terminalDetailRecorder.Code, terminalDetailRecorder.Body.String())
	}
	if !strings.Contains(terminalDetailRecorder.Body.String(), `"resource_type":"Terminal"`) ||
		!strings.Contains(terminalDetailRecorder.Body.String(), `"id":"terminal_`+assignedReader.ID+`"`) ||
		!strings.Contains(terminalDetailRecorder.Body.String(), `"reader_id":"`+assignedReader.ID+`"`) ||
		!strings.Contains(terminalDetailRecorder.Body.String(), `"place":{"id":"building_demo_001"`) {
		t.Fatalf("expected terminal detail wrapper fields, body=%s", terminalDetailRecorder.Body.String())
	}
	missingTerminalRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/terminals/terminal_missing?tenant_id=tenant_demo_jakarta", token, nil)
	if missingTerminalRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing terminal detail status 404, got %d body=%s", missingTerminalRecorder.Code, missingTerminalRecorder.Body.String())
	}
	rebootReaderRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/readers/"+assignedReader.ID+"/reboot?tenant_id=tenant_demo_jakarta", token, nil)
	if rebootReaderRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected reader reboot status 202, got %d body=%s", rebootReaderRecorder.Code, rebootReaderRecorder.Body.String())
	}
	rebootTerminalRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/terminals/terminal_"+assignedReader.ID+"/reboot?tenant_id=tenant_demo_jakarta", token, nil)
	if rebootTerminalRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected terminal reboot status 202, got %d body=%s", rebootTerminalRecorder.Code, rebootTerminalRecorder.Body.String())
	}
	triggerTerminalRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/terminals/terminal_"+assignedReader.ID+"/trigger?tenant_id=tenant_demo_jakarta", token, nil)
	if triggerTerminalRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected terminal trigger status 204, got %d body=%s", triggerTerminalRecorder.Code, triggerTerminalRecorder.Body.String())
	}

	deassignReaderRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/readers/"+assignedReader.ID+"/deassign?tenant_id=tenant_demo_jakarta", token, nil)
	if deassignReaderRecorder.Code != http.StatusOK {
		t.Fatalf("expected reader deassign status 200, got %d body=%s", deassignReaderRecorder.Code, deassignReaderRecorder.Body.String())
	}
	if !strings.Contains(deassignReaderRecorder.Body.String(), `"resource_type":"Reader"`) ||
		!strings.Contains(deassignReaderRecorder.Body.String(), `"device_id":"RD-JKT-9001"`) {
		t.Fatalf("expected deassigned reader wrapper, body=%s", deassignReaderRecorder.Body.String())
	}
	readersAfterDeassignRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/readers?ids="+assignedReader.ID+"&tenant_id=tenant_demo_jakarta", token, nil)
	if readersAfterDeassignRecorder.Code != http.StatusOK {
		t.Fatalf("expected readers after deassign status 200, got %d body=%s", readersAfterDeassignRecorder.Code, readersAfterDeassignRecorder.Body.String())
	}
	if strings.Contains(readersAfterDeassignRecorder.Body.String(), "RD-JKT-9001") {
		t.Fatalf("expected deassigned reader to be removed, body=%s", readersAfterDeassignRecorder.Body.String())
	}

	unbindControllerLockRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/controllers/"+assignedController.ID+"/locks/door_jkt_001?tenant_id=tenant_demo_jakarta", token, nil)
	if unbindControllerLockRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected controller lock unbind status 204, got %d body=%s", unbindControllerLockRecorder.Code, unbindControllerLockRecorder.Body.String())
	}
	deassignControllerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/"+assignedController.ID+"/deassign?tenant_id=tenant_demo_jakarta", token, nil)
	if deassignControllerRecorder.Code != http.StatusOK {
		t.Fatalf("expected controller deassign status 200, got %d body=%s", deassignControllerRecorder.Code, deassignControllerRecorder.Body.String())
	}
	if !strings.Contains(deassignControllerRecorder.Body.String(), `"resource_type":"Controller"`) ||
		!strings.Contains(deassignControllerRecorder.Body.String(), `"device_id":"MP-GW-JKT-9001"`) {
		t.Fatalf("expected deassigned controller wrapper, body=%s", deassignControllerRecorder.Body.String())
	}
	controllersAfterDeassignRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/controllers?ids="+assignedController.ID+"&tenant_id=tenant_demo_jakarta", token, nil)
	if controllersAfterDeassignRecorder.Code != http.StatusOK {
		t.Fatalf("expected controllers after deassign status 200, got %d body=%s", controllersAfterDeassignRecorder.Code, controllersAfterDeassignRecorder.Body.String())
	}
	if strings.Contains(controllersAfterDeassignRecorder.Body.String(), "MP-GW-JKT-9001") {
		t.Fatalf("expected deassigned controller to be removed, body=%s", controllersAfterDeassignRecorder.Body.String())
	}
}

func TestReferenceResourceEndpointsMapEventSetsAndMetadata(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-events-api-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	eventSetBody := []byte(`{"event_set":{"place_id":"building_demo_001","event_type":"access_granted"}}`)
	eventSetRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/event_sets", token, eventSetBody)
	if eventSetRecorder.Code != http.StatusOK {
		t.Fatalf("expected event set status 200, got %d body=%s", eventSetRecorder.Code, eventSetRecorder.Body.String())
	}
	if !strings.Contains(eventSetRecorder.Body.String(), `"status":"finished"`) ||
		!strings.Contains(eventSetRecorder.Body.String(), `"uuid":"evt_1001"`) ||
		!strings.Contains(eventSetRecorder.Body.String(), `"tenant_id":"tenant_demo_jakarta"`) ||
		!strings.Contains(eventSetRecorder.Body.String(), `"area_id":"area_demo_001"`) ||
		!strings.Contains(eventSetRecorder.Body.String(), `"gateway_id":"MP-GW-JKT-0001"`) ||
		strings.Contains(eventSetRecorder.Body.String(), "evt_1004") {
		t.Fatalf("expected filtered event set wrapper, body=%s", eventSetRecorder.Body.String())
	}

	fetchRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/event_sets/event_set_demo?place_id=building_demo_001&event_type=access_granted", token, nil)
	if fetchRecorder.Code != http.StatusOK {
		t.Fatalf("expected fetch event set status 200, got %d body=%s", fetchRecorder.Code, fetchRecorder.Body.String())
	}
	if !strings.Contains(fetchRecorder.Body.String(), `"id":"event_set_demo"`) ||
		!strings.Contains(fetchRecorder.Body.String(), `"object_type":"Lock"`) {
		t.Fatalf("expected fetched event set wrapper, body=%s", fetchRecorder.Body.String())
	}

	metaRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/events/meta", token, nil)
	if metaRecorder.Code != http.StatusOK {
		t.Fatalf("expected event metadata status 200, got %d body=%s", metaRecorder.Code, metaRecorder.Body.String())
	}
	if !strings.Contains(metaRecorder.Body.String(), "object_type_to_action") {
		t.Fatalf("expected event metadata map, body=%s", metaRecorder.Body.String())
	}

	typesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/events/types", token, nil)
	if typesRecorder.Code != http.StatusOK {
		t.Fatalf("expected event types status 200, got %d body=%s", typesRecorder.Code, typesRecorder.Body.String())
	}
	if !strings.Contains(typesRecorder.Body.String(), "access_granted") ||
		!strings.Contains(typesRecorder.Body.String(), "gateway_event") {
		t.Fatalf("expected reference event types, body=%s", typesRecorder.Body.String())
	}
}

func TestReferenceResourceEndpointsMapIntegrations(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-integrations-api-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	recorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/integrations?type=identity_provider&provider=oidc", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected integrations status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"resource_type":"Integration"`) ||
		!strings.Contains(recorder.Body.String(), `"type":"identity_provider"`) ||
		!strings.Contains(recorder.Body.String(), `"provider":"oidc"`) ||
		!strings.Contains(recorder.Body.String(), `"configured":true`) {
		t.Fatalf("expected identity provider integration wrapper, body=%s", recorder.Body.String())
	}

	createBody := []byte(`{"integration":{"tenant_id":"tenant_demo_jakarta","type":"hris","provider":"sunfish","status":"active","sync_mode":"webhook","credential_ref":"vault://tenant_demo_jakarta/hris/sunfish/api_key","webhook_secret_ref":"vault://tenant_demo_jakarta/hris/sunfish/webhook_secret"}}`)
	createRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/integrations", token, createBody)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create integration status 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var createdIntegration referenceIntegration
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createdIntegration); err != nil {
		t.Fatalf("decode created integration: %v", err)
	}
	if createdIntegration.ID == "" || createdIntegration.Type != "hris" || createdIntegration.Provider != "sunfish" || createdIntegration.SyncMode != "webhook" {
		t.Fatalf("expected created hris integration, got %#v", createdIntegration)
	}

	detailRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/integrations/"+createdIntegration.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected integration detail status 200, got %d body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}
	if !strings.Contains(detailRecorder.Body.String(), `"provider":"sunfish"`) {
		t.Fatalf("expected sunfish integration detail, body=%s", detailRecorder.Body.String())
	}

	updateBody := []byte(`{"integration":{"tenant_id":"tenant_demo_jakarta","status":"inactive","sync_mode":"pull"}}`)
	updateRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/integrations/"+createdIntegration.ID, token, updateBody)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update integration status 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	if !strings.Contains(updateRecorder.Body.String(), `"status":"inactive"`) ||
		!strings.Contains(updateRecorder.Body.String(), `"sync_mode":"pull"`) {
		t.Fatalf("expected updated hris integration, body=%s", updateRecorder.Body.String())
	}

	deleteRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/integrations/"+createdIntegration.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected delete integration status 204, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}

func TestReferenceResourceEndpointsMapAlertPolicies(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-alert-policies-api-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	walletRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alert_policies?category=wallet_jobs", token, nil)
	if walletRecorder.Code != http.StatusOK {
		t.Fatalf("expected wallet alert policies status 200, got %d body=%s", walletRecorder.Code, walletRecorder.Body.String())
	}
	if !strings.Contains(walletRecorder.Body.String(), `"resource_type":"AlertPolicy"`) ||
		!strings.Contains(walletRecorder.Body.String(), `"category":"wallet_jobs"`) ||
		!strings.Contains(walletRecorder.Body.String(), `"trigger":"wallet_job_dlq_threshold"`) ||
		!strings.Contains(walletRecorder.Body.String(), `"receiver_groups":["security"]`) {
		t.Fatalf("expected wallet alert policy wrapper fields, body=%s", walletRecorder.Body.String())
	}

	enterpriseRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alert_policies?category=enterprise_sync_worker", token, nil)
	if enterpriseRecorder.Code != http.StatusOK {
		t.Fatalf("expected enterprise alert policies status 200, got %d body=%s", enterpriseRecorder.Code, enterpriseRecorder.Body.String())
	}
	if !strings.Contains(enterpriseRecorder.Body.String(), `"category":"enterprise_sync_worker"`) ||
		!strings.Contains(enterpriseRecorder.Body.String(), `"trigger":"worker_failure_threshold"`) ||
		!strings.Contains(enterpriseRecorder.Body.String(), `"threshold":3`) {
		t.Fatalf("expected enterprise alert policy wrapper fields, body=%s", enterpriseRecorder.Body.String())
	}
}

func TestReferenceAlertPolicyPatchPersistsSubscription(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-alert-policy-patch-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	body := []byte(`{
		"alert_policy": {
			"enabled": false,
			"threshold": 7,
			"window_seconds": 600,
			"cooldown_seconds": 0,
			"channels": {
				"email": false,
				"whatsapp": true
			},
			"receiver_groups": ["ops", "security"]
		}
	}`)
	recorder := referenceAPIRequest(
		t,
		router,
		http.MethodPatch,
		"/api/v1/alert_policies/ap_wallet_jobs_tenant_demo_jakarta",
		token,
		body,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected wallet alert policy patch status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var patched struct {
		Category        string   `json:"category"`
		Enabled         bool     `json:"enabled"`
		Threshold       int      `json:"threshold"`
		WindowSeconds   int64    `json:"window_seconds"`
		CooldownSeconds int64    `json:"cooldown_seconds"`
		ReceiverGroups  []string `json:"receiver_groups"`
		Channels        struct {
			Email    bool `json:"email"`
			WhatsApp bool `json:"whatsapp"`
		} `json:"channels"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patched alert policy: %v", err)
	}
	if patched.Category != "wallet_jobs" ||
		patched.Enabled ||
		patched.Threshold != 7 ||
		patched.WindowSeconds != 600 ||
		patched.CooldownSeconds != 0 ||
		patched.Channels.Email ||
		!patched.Channels.WhatsApp ||
		strings.Join(patched.ReceiverGroups, ",") != "ops,security" {
		t.Fatalf("expected wallet alert policy patch fields, got %#v", patched)
	}

	getRecorder := referenceAPIRequest(
		t,
		router,
		http.MethodGet,
		"/api/v1/alert_policies/ap_wallet_jobs_tenant_demo_jakarta",
		token,
		nil,
	)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected wallet alert policy get status 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	if !strings.Contains(getRecorder.Body.String(), `"threshold":7`) ||
		!strings.Contains(getRecorder.Body.String(), `"enabled":false`) ||
		!strings.Contains(getRecorder.Body.String(), `"receiver_groups":["ops","security"]`) {
		t.Fatalf("expected persisted wallet alert policy fields, body=%s", getRecorder.Body.String())
	}
}

func TestReferenceAlertPolicyCreateAndDeletePersistSubscription(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-alert-policy-create-delete-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	body := []byte(`{
		"alert_policy": {
			"tenant_id": "tenant_demo_jakarta",
			"category": "enterprise_sync_worker",
			"enabled": true,
			"threshold": 5,
			"window_seconds": 900,
			"cooldown_seconds": 60,
			"channels": {
				"email": true,
				"whatsapp": false
			},
			"receiver_groups": ["ops"]
		}
	}`)
	createRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/alert_policies", token, body)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected alert policy create status 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		ID              string   `json:"id"`
		Category        string   `json:"category"`
		Enabled         bool     `json:"enabled"`
		Threshold       int      `json:"threshold"`
		WindowSeconds   int64    `json:"window_seconds"`
		CooldownSeconds int64    `json:"cooldown_seconds"`
		ReceiverGroups  []string `json:"receiver_groups"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created alert policy: %v", err)
	}
	if created.ID != "ap_enterprise_sync_worker_tenant_demo_jakarta" ||
		created.Category != "enterprise_sync_worker" ||
		!created.Enabled ||
		created.Threshold != 5 ||
		created.WindowSeconds != 900 ||
		created.CooldownSeconds != 60 ||
		strings.Join(created.ReceiverGroups, ",") != "ops" {
		t.Fatalf("expected created enterprise alert policy fields, got %#v body=%s", created, createRecorder.Body.String())
	}

	deleteRecorder := referenceAPIRequest(
		t,
		router,
		http.MethodDelete,
		"/api/v1/alert_policies/ap_enterprise_sync_worker_tenant_demo_jakarta",
		token,
		nil,
	)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected alert policy delete status 204, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	getRecorder := referenceAPIRequest(
		t,
		router,
		http.MethodGet,
		"/api/v1/alert_policies/ap_enterprise_sync_worker_tenant_demo_jakarta",
		token,
		nil,
	)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected alert policy get after delete status 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	if !strings.Contains(getRecorder.Body.String(), `"enabled":false`) ||
		!strings.Contains(getRecorder.Body.String(), `"status":"inactive"`) ||
		!strings.Contains(getRecorder.Body.String(), `"threshold":5`) {
		t.Fatalf("expected delete to persist inactive policy, body=%s", getRecorder.Body.String())
	}
}

func TestReferenceDestructiveMutationsAppendAuditLogs(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-destructive-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createPlaceRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/places", token, []byte(`{"place":{"tenant_id":"tenant_demo_jakarta","name":"Audit Place","address":"Jakarta"}}`))
	if createPlaceRecorder.Code != http.StatusCreated {
		t.Fatalf("expected place create status 201, got %d body=%s", createPlaceRecorder.Code, createPlaceRecorder.Body.String())
	}
	var createdPlace struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createPlaceRecorder.Body.Bytes(), &createdPlace); err != nil {
		t.Fatalf("decode created place: %v", err)
	}
	deletePlaceRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/places/"+createdPlace.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletePlaceRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected place delete status 204, got %d body=%s", deletePlaceRecorder.Code, deletePlaceRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_place_deleted", "place_id="+createdPlace.ID, "name=Audit Place")

	createLockBody := []byte(`{"lock":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_001","floor_id":"floor_demo_001","area_id":"area_demo_001","name":"Audit Door","gateway_id":"MP-GW-AUDIT-001","kind":"office","status":"online"}}`)
	createLockRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/locks", token, createLockBody)
	if createLockRecorder.Code != http.StatusCreated {
		t.Fatalf("expected lock create status 201, got %d body=%s", createLockRecorder.Code, createLockRecorder.Body.String())
	}
	var createdLock struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createLockRecorder.Body.Bytes(), &createdLock); err != nil {
		t.Fatalf("decode created lock: %v", err)
	}
	deleteLockRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/locks/"+createdLock.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteLockRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected lock delete status 204, got %d body=%s", deleteLockRecorder.Code, deleteLockRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_lock_deleted", "lock_id="+createdLock.ID, "place_id=building_demo_001")

	createAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_001","assignee_type":"User","assignee_id":"usr_audit_place_admin","assignee_email":"audit.place.admin@example.test"}}`)
	createAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", token, createAssignmentBody)
	if createAssignmentRecorder.Code != http.StatusCreated {
		t.Fatalf("expected role assignment create status 201, got %d body=%s", createAssignmentRecorder.Code, createAssignmentRecorder.Body.String())
	}
	var createdAssignment struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createAssignmentRecorder.Body.Bytes(), &createdAssignment); err != nil {
		t.Fatalf("decode created role assignment: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_role_assignment_created", "role_assignment_id="+createdAssignment.ID, "role_id=role_place_admin", "applies_to_id=building_demo_001")

	updateAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_001","assignee_type":"User","assignee_id":"usr_audit_place_admin","assignee_email":"audit.place.admin@example.test","valid_until":"2099-05-01T10:00:00Z"}}`)
	updateAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/role_assignments/"+createdAssignment.ID, token, updateAssignmentBody)
	if updateAssignmentRecorder.Code != http.StatusOK {
		t.Fatalf("expected role assignment update status 200, got %d body=%s", updateAssignmentRecorder.Code, updateAssignmentRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_role_assignment_updated", "role_assignment_id="+createdAssignment.ID, "role_id=role_place_admin", "applies_to_id=building_demo_001")

	deleteAssignmentRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/role_assignments/"+createdAssignment.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteAssignmentRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected role assignment delete status 204, got %d body=%s", deleteAssignmentRecorder.Code, deleteAssignmentRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_role_assignment_deleted", "role_assignment_id="+createdAssignment.ID, "role_id=role_place_admin")

	createShareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"audit.guest@example.test","place_id":"building_demo_001","valid_until":"2026-05-01T10:00:00Z"}}`)
	createShareRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/shares", token, createShareBody)
	if createShareRecorder.Code != http.StatusCreated {
		t.Fatalf("expected share create status 201, got %d body=%s", createShareRecorder.Code, createShareRecorder.Body.String())
	}
	var createdShare struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createShareRecorder.Body.Bytes(), &createdShare); err != nil {
		t.Fatalf("decode created share: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_share_created", "share_id="+createdShare.ID, "email=audit.guest@example.test", "place_id=building_demo_001")

	updateShareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"audit.guest@example.test","place_id":"building_demo_001","valid_until":"2026-05-02T10:00:00Z"}}`)
	updateShareRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/shares/"+createdShare.ID, token, updateShareBody)
	if updateShareRecorder.Code != http.StatusOK {
		t.Fatalf("expected share update status 200, got %d body=%s", updateShareRecorder.Code, updateShareRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_share_updated", "share_id="+createdShare.ID, "email=audit.guest@example.test", "place_id=building_demo_001")

	deleteShareRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/shares/"+createdShare.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteShareRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected share delete status 204, got %d body=%s", deleteShareRecorder.Code, deleteShareRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_share_deleted", "share_id="+createdShare.ID, "email=audit.guest@example.test")

	createAssignedCardBody := []byte(`{"card":{"tenant_id":"tenant_demo_jakarta","template_id":"wpt_employee_demo","card_number":"AUDIT-CARD-DEASSIGN","user_id":"usr_1001"}}`)
	createAssignedCardRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards", token, createAssignedCardBody)
	if createAssignedCardRecorder.Code != http.StatusCreated {
		t.Fatalf("expected assigned card create status 201, got %d body=%s", createAssignedCardRecorder.Code, createAssignedCardRecorder.Body.String())
	}
	var createdAssignedCard struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createAssignedCardRecorder.Body.Bytes(), &createdAssignedCard); err != nil {
		t.Fatalf("decode assigned card: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_card_created", "card_id="+createdAssignedCard.ID)
	assertReferenceAuditLog(t, router, token, "reference_card_assigned", "card_id="+createdAssignedCard.ID, "target_id=usr_1001")

	deassignCardRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards/"+createdAssignedCard.ID+"/deassign?tenant_id=tenant_demo_jakarta", token, nil)
	if deassignCardRecorder.Code != http.StatusOK {
		t.Fatalf("expected card deassign status 200, got %d body=%s", deassignCardRecorder.Code, deassignCardRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_card_deassigned", "card_id="+createdAssignedCard.ID)

	createRevokedCardBody := []byte(`{"card":{"tenant_id":"tenant_demo_jakarta","template_id":"wpt_employee_demo","card_number":"AUDIT-CARD-REVOKE"}}`)
	createRevokedCardRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards", token, createRevokedCardBody)
	if createRevokedCardRecorder.Code != http.StatusCreated {
		t.Fatalf("expected revocable card create status 201, got %d body=%s", createRevokedCardRecorder.Code, createRevokedCardRecorder.Body.String())
	}
	var createdRevokedCard struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRevokedCardRecorder.Body.Bytes(), &createdRevokedCard); err != nil {
		t.Fatalf("decode revocable card: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_card_created", "card_id="+createdRevokedCard.ID)

	revokeCardRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards/"+createdRevokedCard.ID+"/revoke?tenant_id=tenant_demo_jakarta", token, nil)
	if revokeCardRecorder.Code != http.StatusOK {
		t.Fatalf("expected card revoke status 200, got %d body=%s", revokeCardRecorder.Code, revokeCardRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_card_revoked", "card_id="+createdRevokedCard.ID, "status=revoked")

	deleteAlertPolicyRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/alert_policies/ap_wallet_jobs_tenant_demo_jakarta", token, nil)
	if deleteAlertPolicyRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected alert policy delete status 204, got %d body=%s", deleteAlertPolicyRecorder.Code, deleteAlertPolicyRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_alert_policy_disabled", "category=wallet_jobs")
}

func TestReferenceDestructiveMutationAuditCoversAccessHardwareAndCardStatus(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-destructive-audit-coverage-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createGroupBody := []byte(`{"group":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_001","name":"Audit Access Group","description":"Created for audit coverage","member_ids":["usr_1001"]}}`)
	createGroupRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/groups", token, createGroupBody)
	if createGroupRecorder.Code != http.StatusCreated {
		t.Fatalf("expected group create status 201, got %d body=%s", createGroupRecorder.Code, createGroupRecorder.Body.String())
	}
	var createdGroup struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createGroupRecorder.Body.Bytes(), &createdGroup); err != nil {
		t.Fatalf("decode created group: %v", err)
	}

	createGroupLinkBody := []byte(`{"group_link":{"tenant_id":"tenant_demo_jakarta","group_id":"` + createdGroup.ID + `","name":"Audit Visitor Link","email":"audit.link@example.test","valid_until":"2099-05-01T10:00:00Z"}}`)
	createGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/group_links", token, createGroupLinkBody)
	if createGroupLinkRecorder.Code != http.StatusCreated {
		t.Fatalf("expected group link create status 201, got %d body=%s", createGroupLinkRecorder.Code, createGroupLinkRecorder.Body.String())
	}
	var createdGroupLink struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createGroupLinkRecorder.Body.Bytes(), &createdGroupLink); err != nil {
		t.Fatalf("decode created group link: %v", err)
	}
	deleteGroupLinkRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/group_links/"+createdGroupLink.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteGroupLinkRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected group link delete status 204, got %d body=%s", deleteGroupLinkRecorder.Code, deleteGroupLinkRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_group_link_deleted", "group_link_id="+createdGroupLink.ID, "group_id="+createdGroup.ID)

	createGroupLockRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/group_locks", token, []byte(`{"group_lock":{"tenant_id":"tenant_demo_jakarta","group_id":"dg_1001","lock_id":"door_jkt_001"}}`))
	if createGroupLockRecorder.Code != http.StatusCreated {
		t.Fatalf("expected group lock create status 201, got %d body=%s", createGroupLockRecorder.Code, createGroupLockRecorder.Body.String())
	}
	deleteGroupLockRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/group_locks/dg_1001:door_jkt_001?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteGroupLockRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected group lock delete status 204, got %d body=%s", deleteGroupLockRecorder.Code, deleteGroupLockRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_group_lock_deleted", "group_id=dg_1001", "lock_id=door_jkt_001")

	deleteGroupRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/groups/"+createdGroup.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteGroupRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected group delete status 204, got %d body=%s", deleteGroupRecorder.Code, deleteGroupRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_group_deleted", "group_id="+createdGroup.ID, "name=Audit Access Group")

	createTeamBody := []byte(`{"team":{"tenant_id":"tenant_demo_jakarta","name":"Audit Team","scope":"place","place_id":"building_demo_001","description":"Created for audit coverage"}}`)
	createTeamRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/teams", token, createTeamBody)
	if createTeamRecorder.Code != http.StatusCreated {
		t.Fatalf("expected team create status 201, got %d body=%s", createTeamRecorder.Code, createTeamRecorder.Body.String())
	}
	var createdTeam struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createTeamRecorder.Body.Bytes(), &createdTeam); err != nil {
		t.Fatalf("decode created team: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_team_created", "team_id="+createdTeam.ID, "name=Audit Team", "place_id=building_demo_001")

	updateTeamBody := []byte(`{"team":{"tenant_id":"tenant_demo_jakarta","name":"Audit Team Updated","scope":"place","place_id":"building_demo_001","description":"Updated for audit coverage"}}`)
	updateTeamRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/teams/"+createdTeam.ID, token, updateTeamBody)
	if updateTeamRecorder.Code != http.StatusOK {
		t.Fatalf("expected team update status 200, got %d body=%s", updateTeamRecorder.Code, updateTeamRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_team_updated", "team_id="+createdTeam.ID, "name=Audit Team Updated", "place_id=building_demo_001")

	createMembershipBody := []byte(`{"team_membership":{"tenant_id":"tenant_demo_jakarta","team_id":"` + createdTeam.ID + `","member_type":"User","member_id":"usr_1001","member_email":"audit.member@example.test","member_name":"Audit Member"}}`)
	createMembershipRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/team_memberships", token, createMembershipBody)
	if createMembershipRecorder.Code != http.StatusCreated {
		t.Fatalf("expected team membership create status 201, got %d body=%s", createMembershipRecorder.Code, createMembershipRecorder.Body.String())
	}
	var createdMembership struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createMembershipRecorder.Body.Bytes(), &createdMembership); err != nil {
		t.Fatalf("decode created team membership: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_team_membership_created", "team_membership_id="+createdMembership.ID, "team_id="+createdTeam.ID, "email=audit.member@example.test")

	deleteMembershipRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/team_memberships/"+createdMembership.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteMembershipRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected team membership delete status 204, got %d body=%s", deleteMembershipRecorder.Code, deleteMembershipRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_team_membership_deleted", "team_membership_id="+createdMembership.ID)

	deleteTeamRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/teams/"+createdTeam.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteTeamRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected team delete status 204, got %d body=%s", deleteTeamRecorder.Code, deleteTeamRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_team_deleted", "team_id="+createdTeam.ID)

	controllerInventoryBody := []byte(`{"tenant_id":"tenant_demo_jakarta","items":[{"serial_number":"MP-GW-AUDIT-9001","product_type":"gateway","batch_code":"reference-audit-test","source":"test"}]}`)
	controllerInventoryRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/serial-inventory/import", token, controllerInventoryBody)
	if controllerInventoryRecorder.Code != http.StatusCreated {
		t.Fatalf("expected controller serial inventory import status 201, got %d body=%s", controllerInventoryRecorder.Code, controllerInventoryRecorder.Body.String())
	}
	assignControllerBody := []byte(`{"controller":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_001","device_capacity":4}}`)
	assignControllerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/MP-GW-AUDIT-9001/assign", token, assignControllerBody)
	if assignControllerRecorder.Code != http.StatusCreated {
		t.Fatalf("expected controller assign status 201, got %d body=%s", assignControllerRecorder.Code, assignControllerRecorder.Body.String())
	}
	var assignedController struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(assignControllerRecorder.Body.Bytes(), &assignedController); err != nil {
		t.Fatalf("decode assigned controller: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_controller_assigned", "controller_id="+assignedController.ID, "device_id=MP-GW-AUDIT-9001", "place_id=building_demo_001")

	bindControllerLockRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/"+assignedController.ID+"/locks", token, []byte(`{"tenant_id":"tenant_demo_jakarta","lock_id":"door_jkt_001"}`))
	if bindControllerLockRecorder.Code != http.StatusOK {
		t.Fatalf("expected controller lock bind status 200, got %d body=%s", bindControllerLockRecorder.Code, bindControllerLockRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_controller_lock_bound", "controller_id="+assignedController.ID, "lock_id=door_jkt_001")

	publishControllerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/"+assignedController.ID+"/config/publish", token, []byte(`{"controller":{"tenant_id":"tenant_demo_jakarta","version":"reference-audit-v1"}}`))
	if publishControllerRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected controller config publish status 202, got %d body=%s", publishControllerRecorder.Code, publishControllerRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_controller_config_published", "controller_id="+assignedController.ID, "version=reference-audit-v1")

	rebootControllerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/"+assignedController.ID+"/reboot?tenant_id=tenant_demo_jakarta", token, nil)
	if rebootControllerRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected controller reboot status 202, got %d body=%s", rebootControllerRecorder.Code, rebootControllerRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_controller_reboot_queued", "controller_id="+assignedController.ID)

	readerInventoryBody := []byte(`{"tenant_id":"tenant_demo_jakarta","items":[{"serial_number":"RD-AUDIT-9001","product_type":"reader","batch_code":"reference-audit-test","source":"test"}]}`)
	readerInventoryRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/serial-inventory/import", token, readerInventoryBody)
	if readerInventoryRecorder.Code != http.StatusCreated {
		t.Fatalf("expected reader serial inventory import status 201, got %d body=%s", readerInventoryRecorder.Code, readerInventoryRecorder.Body.String())
	}
	assignReaderBody := []byte(`{"reader":{"tenant_id":"tenant_demo_jakarta","controller_id":"` + assignedController.ID + `","protocol":"osdp_v2","status":"online"}}`)
	assignReaderRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/readers/RD-AUDIT-9001/assign", token, assignReaderBody)
	if assignReaderRecorder.Code != http.StatusCreated {
		t.Fatalf("expected reader assign status 201, got %d body=%s", assignReaderRecorder.Code, assignReaderRecorder.Body.String())
	}
	var assignedReader struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(assignReaderRecorder.Body.Bytes(), &assignedReader); err != nil {
		t.Fatalf("decode assigned reader: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_reader_assigned", "reader_id="+assignedReader.ID, "controller_id="+assignedController.ID, "device_id=RD-AUDIT-9001")

	rebootReaderRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/readers/"+assignedReader.ID+"/reboot?tenant_id=tenant_demo_jakarta", token, nil)
	if rebootReaderRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected reader reboot status 202, got %d body=%s", rebootReaderRecorder.Code, rebootReaderRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_reader_reboot_queued", "reader_id="+assignedReader.ID, "controller_id="+assignedController.ID)

	rebootTerminalRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/terminals/terminal_"+assignedReader.ID+"/reboot?tenant_id=tenant_demo_jakarta", token, nil)
	if rebootTerminalRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected terminal reboot status 202, got %d body=%s", rebootTerminalRecorder.Code, rebootTerminalRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_terminal_reboot_queued", "terminal_id=terminal_"+assignedReader.ID, "controller_id="+assignedController.ID)

	triggerTerminalRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/terminals/terminal_"+assignedReader.ID+"/trigger?tenant_id=tenant_demo_jakarta", token, nil)
	if triggerTerminalRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected terminal trigger status 204, got %d body=%s", triggerTerminalRecorder.Code, triggerTerminalRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_terminal_triggered", "terminal_id=terminal_"+assignedReader.ID, "reader_id="+assignedReader.ID, "controller_id="+assignedController.ID)

	deassignReaderRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/readers/"+assignedReader.ID+"/deassign?tenant_id=tenant_demo_jakarta", token, nil)
	if deassignReaderRecorder.Code != http.StatusOK {
		t.Fatalf("expected reader deassign status 200, got %d body=%s", deassignReaderRecorder.Code, deassignReaderRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_reader_deassigned", "reader_id="+assignedReader.ID, "device_id=RD-AUDIT-9001")

	unbindControllerLockRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/controllers/"+assignedController.ID+"/locks/door_jkt_001?tenant_id=tenant_demo_jakarta", token, nil)
	if unbindControllerLockRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected controller lock unbind status 204, got %d body=%s", unbindControllerLockRecorder.Code, unbindControllerLockRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_controller_lock_unbound", "controller_id="+assignedController.ID, "lock_id=door_jkt_001")

	deassignControllerRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/controllers/"+assignedController.ID+"/deassign?tenant_id=tenant_demo_jakarta", token, nil)
	if deassignControllerRecorder.Code != http.StatusOK {
		t.Fatalf("expected controller deassign status 200, got %d body=%s", deassignControllerRecorder.Code, deassignControllerRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_controller_deassigned", "controller_id="+assignedController.ID, "device_id=MP-GW-AUDIT-9001")

	createCardBody := []byte(`{"card":{"tenant_id":"tenant_demo_jakarta","template_id":"wpt_employee_demo","card_number":"AUDIT-CARD-STATUS"}}`)
	createCardRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards", token, createCardBody)
	if createCardRecorder.Code != http.StatusCreated {
		t.Fatalf("expected card create status 201, got %d body=%s", createCardRecorder.Code, createCardRecorder.Body.String())
	}
	var createdCard struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createCardRecorder.Body.Bytes(), &createdCard); err != nil {
		t.Fatalf("decode status card: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "reference_card_created", "card_id="+createdCard.ID)

	activateCardRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards/"+createdCard.ID+"/activate?tenant_id=tenant_demo_jakarta", token, nil)
	if activateCardRecorder.Code != http.StatusOK {
		t.Fatalf("expected card activate status 200, got %d body=%s", activateCardRecorder.Code, activateCardRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_card_activated", "card_id="+createdCard.ID, "status=activated")

	deactivateCardRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/cards/"+createdCard.ID+"/deactivate?tenant_id=tenant_demo_jakarta", token, nil)
	if deactivateCardRecorder.Code != http.StatusOK {
		t.Fatalf("expected card deactivate status 200, got %d body=%s", deactivateCardRecorder.Code, deactivateCardRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_card_deactivated", "card_id="+createdCard.ID, "status=deactivated")
}

func TestLegacyGatewayHighRiskMutationsAppendAuditLogs(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "legacy-gateway-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	controllerInventoryBody := []byte(`{"tenant_id":"tenant_demo_jakarta","items":[{"serial_number":"MP-GW-LEGACY-AUDIT-9001","product_type":"gateway","batch_code":"legacy-audit-test","source":"test"}]}`)
	controllerInventoryRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/serial-inventory/import", token, controllerInventoryBody)
	if controllerInventoryRecorder.Code != http.StatusCreated {
		t.Fatalf("expected controller serial inventory import status 201, got %d body=%s", controllerInventoryRecorder.Code, controllerInventoryRecorder.Body.String())
	}
	registerGatewayRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/register", token, []byte(`{"tenant_id":"tenant_demo_jakarta","serial_number":"MP-GW-LEGACY-AUDIT-9001","building_id":"building_demo_001","device_capacity":4}`))
	if registerGatewayRecorder.Code != http.StatusCreated {
		t.Fatalf("expected gateway register status 201, got %d body=%s", registerGatewayRecorder.Code, registerGatewayRecorder.Body.String())
	}
	var registeredGateway struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(registerGatewayRecorder.Body.Bytes(), &registeredGateway); err != nil {
		t.Fatalf("decode registered gateway: %v", err)
	}
	if registeredGateway.ID == "" {
		t.Fatalf("expected registered gateway id, body=%s", registerGatewayRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "gateway_registered", "gateway_id="+registeredGateway.ID, "place_id=building_demo_001", "serial_number=MP-GW-LEGACY-AUDIT-9001")

	bindRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/"+registeredGateway.ID+"/bind-door", token, []byte(`{"door_id":"door_jkt_001"}`))
	if bindRecorder.Code != http.StatusOK {
		t.Fatalf("expected gateway door bind status 200, got %d body=%s", bindRecorder.Code, bindRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "gateway_door_bound", "gateway_id="+registeredGateway.ID, "door_id=door_jkt_001", "place_id=building_demo_001")

	publishRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/"+registeredGateway.ID+"/config/publish", token, []byte(`{"version":"legacy-audit-v1"}`))
	if publishRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected gateway config publish status 202, got %d body=%s", publishRecorder.Code, publishRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "gateway_config_published", "gateway_id="+registeredGateway.ID, "version=legacy-audit-v1")

	rebootRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/"+registeredGateway.ID+"/reboot", token, nil)
	if rebootRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected gateway reboot status 202, got %d body=%s", rebootRecorder.Code, rebootRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "gateway_reboot_queued", "gateway_id="+registeredGateway.ID)

	readerInventoryBody := []byte(`{"tenant_id":"tenant_demo_jakarta","items":[{"serial_number":"RD-LEGACY-AUDIT-9001","product_type":"reader","batch_code":"legacy-audit-test","source":"test"}]}`)
	readerInventoryRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/serial-inventory/import", token, readerInventoryBody)
	if readerInventoryRecorder.Code != http.StatusCreated {
		t.Fatalf("expected reader serial inventory import status 201, got %d body=%s", readerInventoryRecorder.Code, readerInventoryRecorder.Body.String())
	}
	registerDeviceRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/"+registeredGateway.ID+"/devices", token, []byte(`{"serial_number":"RD-LEGACY-AUDIT-9001","kind":"reader","source":"mistypass_procured","protocol":"osdp_v2","status":"online"}`))
	if registerDeviceRecorder.Code != http.StatusOK {
		t.Fatalf("expected gateway device register status 200, got %d body=%s", registerDeviceRecorder.Code, registerDeviceRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "gateway_device_registered", "gateway_id="+registeredGateway.ID, "serial_number=RD-LEGACY-AUDIT-9001", "kind=reader", "protocol=osdp_v2")

	unbindRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/gateways/"+registeredGateway.ID+"/unbind-door", token, []byte(`{"door_id":"door_jkt_001"}`))
	if unbindRecorder.Code != http.StatusOK {
		t.Fatalf("expected gateway door unbind status 200, got %d body=%s", unbindRecorder.Code, unbindRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "gateway_door_unbound", "gateway_id="+registeredGateway.ID, "door_id=door_jkt_001", "place_id=building_demo_001")
}

func TestReferenceUsersCRUDEndpoints(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-users-crud-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createUserBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"Reference CRUD User","email":"reference.crud.user@example.com","role":"employee","status":"active","group_ids":["ug_common_office_jkt"]}`)
	createUserRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users", token, createUserBody)
	if createUserRecorder.Code != http.StatusCreated {
		t.Fatalf("expected user create status 201, got %d body=%s", createUserRecorder.Code, createUserRecorder.Body.String())
	}
	var createdUser struct {
		ID         string   `json:"id"`
		BuildingID string   `json:"building_id"`
		Name       string   `json:"name"`
		Email      string   `json:"email"`
		Status     string   `json:"status"`
		GroupIDs   []string `json:"group_ids"`
	}
	if err := json.Unmarshal(createUserRecorder.Body.Bytes(), &createdUser); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	if createdUser.ID == "" || createdUser.Name != "Reference CRUD User" || createdUser.Email != "reference.crud.user@example.com" {
		t.Fatalf("expected created user fields, got %#v body=%s", createdUser, createUserRecorder.Body.String())
	}

	inviteUserBody := []byte(`{"tenant_id":"tenant_demo_jakarta","delivery_method":"email"}`)
	inviteUserRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/"+createdUser.ID+"/invite", token, inviteUserBody)
	if inviteUserRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected user invite status 202, got %d body=%s", inviteUserRecorder.Code, inviteUserRecorder.Body.String())
	}
	if !strings.Contains(inviteUserRecorder.Body.String(), `"resource_type":"UserInvitationDelivery"`) ||
		!strings.Contains(inviteUserRecorder.Body.String(), `"user_id":"`+createdUser.ID+`"`) ||
		!strings.Contains(inviteUserRecorder.Body.String(), `"delivery_method":"email"`) ||
		!strings.Contains(inviteUserRecorder.Body.String(), `"status":"queued"`) {
		t.Fatalf("expected user invitation delivery response, body=%s", inviteUserRecorder.Body.String())
	}
	var createdInvitation struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(inviteUserRecorder.Body.Bytes(), &createdInvitation); err != nil {
		t.Fatalf("decode created user invitation: %v", err)
	}
	if createdInvitation.ID == "" || createdInvitation.Status != "queued" {
		t.Fatalf("expected created invitation id and queued status, got %#v", createdInvitation)
	}
	assertReferenceAuditLog(t, router, token, "reference_user_invitation_sent", "user_id="+createdUser.ID, "email=reference.crud.user@example.com", "delivery_method=email", "status=queued")

	listInvitationsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/users/"+createdUser.ID+"/invitations?tenant_id=tenant_demo_jakarta", token, nil)
	if listInvitationsRecorder.Code != http.StatusOK {
		t.Fatalf("expected user invitations status 200, got %d body=%s", listInvitationsRecorder.Code, listInvitationsRecorder.Body.String())
	}
	if !strings.Contains(listInvitationsRecorder.Body.String(), `"id":"`+createdInvitation.ID+`"`) ||
		!strings.Contains(listInvitationsRecorder.Body.String(), `"resource_type":"UserInvitationDelivery"`) {
		t.Fatalf("expected invitation delivery to be listable, body=%s", listInvitationsRecorder.Body.String())
	}

	receiptBody := []byte(`{"tenant_id":"tenant_demo_jakarta","status":"sent","provider":"mailgun","provider_delivery_id":"mg_ref_invite_001"}`)
	receiptRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/"+createdUser.ID+"/invitations/"+createdInvitation.ID+"/receipt", token, receiptBody)
	if receiptRecorder.Code != http.StatusOK {
		t.Fatalf("expected user invitation receipt status 200, got %d body=%s", receiptRecorder.Code, receiptRecorder.Body.String())
	}
	if !strings.Contains(receiptRecorder.Body.String(), `"status":"sent"`) ||
		!strings.Contains(receiptRecorder.Body.String(), `"provider":"mailgun"`) ||
		!strings.Contains(receiptRecorder.Body.String(), `"provider_delivery_id":"mg_ref_invite_001"`) ||
		!strings.Contains(receiptRecorder.Body.String(), `"delivered_at"`) {
		t.Fatalf("expected invitation receipt fields, body=%s", receiptRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_user_invitation_receipt", "user_id="+createdUser.ID, "status=sent", "provider=mailgun", "provider_delivery_id=mg_ref_invite_001")

	getUserRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/users/"+createdUser.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getUserRecorder.Code != http.StatusOK {
		t.Fatalf("expected user detail status 200, got %d body=%s", getUserRecorder.Code, getUserRecorder.Body.String())
	}
	if !strings.Contains(getUserRecorder.Body.String(), `"id":"`+createdUser.ID+`"`) {
		t.Fatalf("expected user detail to include created user, body=%s", getUserRecorder.Body.String())
	}

	updateUserBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_002","name":"Reference CRUD User Updated","status":"suspended","group_ids":[]}`)
	updateUserRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/users/"+createdUser.ID, token, updateUserBody)
	if updateUserRecorder.Code != http.StatusOK {
		t.Fatalf("expected user update status 200, got %d body=%s", updateUserRecorder.Code, updateUserRecorder.Body.String())
	}
	if !strings.Contains(updateUserRecorder.Body.String(), `"name":"Reference CRUD User Updated"`) ||
		!strings.Contains(updateUserRecorder.Body.String(), `"building_id":"building_demo_002"`) ||
		!strings.Contains(updateUserRecorder.Body.String(), `"status":"suspended"`) {
		t.Fatalf("expected updated user fields, body=%s", updateUserRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_user_status_changed", "user_id="+createdUser.ID, "status=suspended", "previous_status=active")

	placeUsersRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/users?tenant_id=tenant_demo_jakarta&place_id=building_demo_002", token, nil)
	if placeUsersRecorder.Code != http.StatusOK {
		t.Fatalf("expected place users status 200, got %d body=%s", placeUsersRecorder.Code, placeUsersRecorder.Body.String())
	}
	if !strings.Contains(placeUsersRecorder.Body.String(), `"id":"`+createdUser.ID+`"`) {
		t.Fatalf("expected updated user under new place scope, body=%s", placeUsersRecorder.Body.String())
	}

	deleteUserRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/users/"+createdUser.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteUserRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected user delete status 204, got %d body=%s", deleteUserRecorder.Code, deleteUserRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_user_deleted", "user_id="+createdUser.ID, "email=reference.crud.user@example.com", "place_id=building_demo_002")

	deletedUserRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/users/"+createdUser.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deletedUserRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted user detail status 404, got %d body=%s", deletedUserRecorder.Code, deletedUserRecorder.Body.String())
	}
}

func assertReferenceAuditLog(t *testing.T, router http.Handler, token, action string, targetParts ...string) {
	t.Helper()
	assertReferenceAuditLogActorRole(t, router, token, action, "organization.admin@mistypass.local", "tenant_admin", targetParts...)
}

func assertReferenceAuditLogActorRole(t *testing.T, router http.Handler, token, action, actor, role string, targetParts ...string) {
	t.Helper()
	recorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/audit-logs?tenant_id=tenant_demo_jakarta&action="+action+"&limit=1", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected audit log status 200 for %s, got %d body=%s", action, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []struct {
			Action string `json:"action"`
			Target string `json:"target"`
			Actor  string `json:"actor"`
			Role   string `json:"role"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode audit logs for %s: %v", action, err)
	}
	if len(response.Items) == 0 {
		t.Fatalf("expected audit log for action %s, body=%s", action, recorder.Body.String())
	}
	item := response.Items[0]
	if item.Action != action {
		t.Fatalf("expected audit action %s, got %#v", action, item)
	}
	if item.Actor != actor || item.Role != role {
		t.Fatalf("expected audit actor=%s role=%s, got %#v", actor, role, item)
	}
	for _, part := range targetParts {
		if !strings.Contains(item.Target, part) {
			t.Fatalf("expected audit target for %s to contain %q, got %q", action, part, item.Target)
		}
	}
}

func referenceAPILogin(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	body := []byte(`{"email":"` + email + `","password":"admin123"}`)
	recorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/auth/login", "", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if response.AccessToken == "" {
		t.Fatal("expected access token")
	}
	return response.AccessToken
}

func referenceAPIRequest(t *testing.T, router http.Handler, method, path, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
