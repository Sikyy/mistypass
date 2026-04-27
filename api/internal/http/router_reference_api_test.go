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
	}
	if err := json.Unmarshal(rolesRecorder.Body.Bytes(), &rolesResponse); err != nil {
		t.Fatalf("decode roles: %v", err)
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

	shareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"guest@example.com","place_id":"building_demo_001","lock_id":"door_jkt_001","valid_until":"2026-05-01T10:00:00Z"}}`)
	shareRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/shares", token, shareBody)
	if shareRecorder.Code != http.StatusCreated {
		t.Fatalf("expected share create status 201, got %d body=%s", shareRecorder.Code, shareRecorder.Body.String())
	}
	if !strings.Contains(shareRecorder.Body.String(), `"role_id":"role_group_access"`) {
		t.Fatalf("expected share to map to group access role, body=%s", shareRecorder.Body.String())
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
