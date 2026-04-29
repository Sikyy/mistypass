package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func TestReferencePlaceAdminURLScopeBlocksUnassignedPlace(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-place-admin-scope-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "place.admin.sudirman@mistypass.local")

	placesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places", token, nil)
	if placesRecorder.Code != http.StatusOK {
		t.Fatalf("expected scoped places status 200, got %d body=%s", placesRecorder.Code, placesRecorder.Body.String())
	}
	if !strings.Contains(placesRecorder.Body.String(), "building_demo_001") {
		t.Fatalf("expected assigned place in response, body=%s", placesRecorder.Body.String())
	}
	if strings.Contains(placesRecorder.Body.String(), "building_demo_002") {
		t.Fatalf("expected unassigned place to be filtered, body=%s", placesRecorder.Body.String())
	}

	crossPlaceRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places/building_demo_002?tenant_id=tenant_demo_jakarta", token, nil)
	if crossPlaceRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected cross-place detail status 403, got %d body=%s", crossPlaceRecorder.Code, crossPlaceRecorder.Body.String())
	}

	crossLockRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/locks/door_jkt_014?tenant_id=tenant_demo_jakarta", token, nil)
	if crossLockRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected cross-place lock status 403, got %d body=%s", crossLockRecorder.Code, crossLockRecorder.Body.String())
	}

	crossCreateLockBody := []byte(`{"lock":{"tenant_id":"tenant_demo_jakarta","place_id":"building_demo_002","floor_id":"floor_demo_003","area_id":"area_demo_003","name":"Cross Place Door","gateway_id":"MP-GW-JKT-X","kind":"office","status":"online"}}`)
	crossCreateLockRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/locks", token, crossCreateLockBody)
	if crossCreateLockRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected cross-place lock create status 403, got %d body=%s", crossCreateLockRecorder.Code, crossCreateLockRecorder.Body.String())
	}

	crossShareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"guest@example.com","place_id":"building_demo_002","lock_id":"door_jkt_014","valid_until":"2026-05-01T10:00:00Z"}}`)
	crossShareRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/shares", token, crossShareBody)
	if crossShareRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected cross-place share create status 403, got %d body=%s", crossShareRecorder.Code, crossShareRecorder.Body.String())
	}

	orgToken := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	createCrossUserBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_002","name":"Cross Place Invitee","email":"cross.place.invitee@example.com","role":"employee","status":"inactive"}`)
	createCrossUserRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users", orgToken, createCrossUserBody)
	if createCrossUserRecorder.Code != http.StatusCreated {
		t.Fatalf("expected cross-place user create status 201, got %d body=%s", createCrossUserRecorder.Code, createCrossUserRecorder.Body.String())
	}
	var crossUser struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createCrossUserRecorder.Body.Bytes(), &crossUser); err != nil {
		t.Fatalf("decode cross-place user: %v", err)
	}
	crossInvitationListRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/users/"+crossUser.ID+"/invitations?tenant_id=tenant_demo_jakarta", token, nil)
	if crossInvitationListRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected cross-place user invitations status 403, got %d body=%s", crossInvitationListRecorder.Code, crossInvitationListRecorder.Body.String())
	}
	crossInviteRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/"+crossUser.ID+"/invite", token, []byte(`{"tenant_id":"tenant_demo_jakarta","delivery_method":"email"}`))
	if crossInviteRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected cross-place user invite status 403, got %d body=%s", crossInviteRecorder.Code, crossInviteRecorder.Body.String())
	}
}

func TestReferencePlaceAdminScopeDerivesFromRoleAssignmentsWithoutJWTBuildingIDs(t *testing.T) {
	const jwtSecret = "reference-place-admin-role-assignment-scope-test-secret"
	router, err := NewRouter(config.Config{
		JWTSecret:       jwtSecret,
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	orgToken := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	createTeamAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_002","assignee_type":"Team","assignee_id":"team_operations_jkt"}}`)
	createTeamAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", orgToken, createTeamAssignmentBody)
	if createTeamAssignmentRecorder.Code != http.StatusCreated {
		t.Fatalf("expected team role assignment create status 201, got %d body=%s", createTeamAssignmentRecorder.Code, createTeamAssignmentRecorder.Body.String())
	}

	tokenService := auth.NewService(jwtSecret, "", 0, 0, false)
	login, err := tokenService.LoginByTrustedUser(auth.User{
		ID:       "usr_place_admin_claims_only_001",
		Email:    "place.admin.sudirman@mistypass.local",
		Role:     "building_admin",
		TenantID: "tenant_demo_jakarta",
	})
	if err != nil {
		t.Fatalf("expected trusted login: %v", err)
	}

	placesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places", login.AccessToken, nil)
	if placesRecorder.Code != http.StatusOK {
		t.Fatalf("expected role-assignment scoped places status 200, got %d body=%s", placesRecorder.Code, placesRecorder.Body.String())
	}
	var placesResponse struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(placesRecorder.Body.Bytes(), &placesResponse); err != nil {
		t.Fatalf("decode scoped places: %v", err)
	}
	placeIDs := map[string]bool{}
	for _, place := range placesResponse.Items {
		placeIDs[place.ID] = true
	}
	if !placeIDs["building_demo_001"] || !placeIDs["building_demo_002"] {
		t.Fatalf("expected direct and team role assignment places, got %#v body=%s", placeIDs, placesRecorder.Body.String())
	}
	if placeIDs["building_demo_003"] {
		t.Fatalf("expected factory place to remain out of scope, got %#v body=%s", placeIDs, placesRecorder.Body.String())
	}

	teamScopedPlaceRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places/building_demo_002?tenant_id=tenant_demo_jakarta", login.AccessToken, nil)
	if teamScopedPlaceRecorder.Code != http.StatusOK {
		t.Fatalf("expected team role assignment place status 200, got %d body=%s", teamScopedPlaceRecorder.Code, teamScopedPlaceRecorder.Body.String())
	}
}

func TestReferencePlaceAdminAccessRightsScopeBlocksCrossPlaceWrites(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-place-admin-access-rights-scope-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	orgToken := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	placeToken := referenceAPILogin(t, router, "place.admin.sudirman@mistypass.local")

	crossCreateAssignmentBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_002","assignee_type":"User","assignee_id":"usr_1001","assignee_email":"andri.pratama@mistypass.local"}}`)
	crossCreateAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", placeToken, crossCreateAssignmentBody)
	if crossCreateAssignmentRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected place admin cross-place role assignment create status 403, got %d body=%s", crossCreateAssignmentRecorder.Code, crossCreateAssignmentRecorder.Body.String())
	}

	createAssignmentRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", orgToken, crossCreateAssignmentBody)
	if createAssignmentRecorder.Code != http.StatusCreated {
		t.Fatalf("expected org role assignment create status 201, got %d body=%s", createAssignmentRecorder.Code, createAssignmentRecorder.Body.String())
	}
	var createdAssignment struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createAssignmentRecorder.Body.Bytes(), &createdAssignment); err != nil {
		t.Fatalf("decode cross-place role assignment: %v", err)
	}

	createShareBody := []byte(`{"share":{"tenant_id":"tenant_demo_jakarta","email":"cross.scope.review@example.test","grantee_name":"Cross Scope Review","place_id":"building_demo_002","valid_until":"2099-05-01T10:00:00Z"}}`)
	createShareRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/shares", orgToken, createShareBody)
	if createShareRecorder.Code != http.StatusCreated {
		t.Fatalf("expected org share create status 201, got %d body=%s", createShareRecorder.Code, createShareRecorder.Body.String())
	}
	var createdShare struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createShareRecorder.Body.Bytes(), &createdShare); err != nil {
		t.Fatalf("decode cross-place share: %v", err)
	}

	detailRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/role_assignments/"+createdAssignment.ID+"?tenant_id=tenant_demo_jakarta", placeToken, nil)
	if detailRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected place admin cross-place role assignment detail status 403, got %d body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}

	selectionBody := []byte(`{"tenant_id":"tenant_demo_jakarta","role_assignment_ids":["` + createdAssignment.ID + `"],"share_ids":["` + createdShare.ID + `"]}`)
	previewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/impact_preview", placeToken, selectionBody)
	if previewRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected place admin cross-place impact preview status 403, got %d body=%s", previewRecorder.Code, previewRecorder.Body.String())
	}

	reviewRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access_rights/review", placeToken, selectionBody)
	if reviewRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected place admin cross-place review status 403, got %d body=%s", reviewRecorder.Code, reviewRecorder.Body.String())
	}

	scheduleBody := []byte(`{"tenant_id":"tenant_demo_jakarta","role_assignment_ids":["` + createdAssignment.ID + `"],"share_ids":["` + createdShare.ID + `"],"valid_until":"2099-06-01T10:00:00Z"}`)
	scheduleRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/access_rights/schedule", placeToken, scheduleBody)
	if scheduleRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected place admin cross-place schedule status 403, got %d body=%s", scheduleRecorder.Code, scheduleRecorder.Body.String())
	}
}

func TestReferenceOperatorReadOnlyWriteGuard(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "reference-operator-write-guard-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "ops.jkt.01@mistypass.local")

	readRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/cards?tenant_id=tenant_demo_jakarta", token, nil)
	if readRecorder.Code != http.StatusOK {
		t.Fatalf("expected operator card list status 200, got %d body=%s", readRecorder.Code, readRecorder.Body.String())
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "create team",
			method: http.MethodPost,
			path:   "/api/v1/teams",
			body:   []byte(`{"team":{"tenant_id":"tenant_demo_jakarta","name":"Operator Guard Team","scope":"place","place_id":"building_demo_001"}}`),
		},
		{
			name:   "update role assignment",
			method: http.MethodPatch,
			path:   "/api/v1/role_assignments/ra_demo_place_admin_001",
			body:   []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_001","assignee_type":"User","assignee_id":"usr_1001"}}`),
		},
		{
			name:   "delete share",
			method: http.MethodDelete,
			path:   "/api/v1/shares/vst_2201?tenant_id=tenant_demo_jakarta",
		},
		{
			name:   "create card",
			method: http.MethodPost,
			path:   "/api/v1/cards",
			body:   []byte(`{"card":{"tenant_id":"tenant_demo_jakarta","template_id":"wpt_employee_demo","card_number":"OPERATOR-GUARD-CARD"}}`),
		},
		{
			name:   "create alert policy",
			method: http.MethodPost,
			path:   "/api/v1/alert_policies",
			body:   []byte(`{"policy":{"tenant_id":"tenant_demo_jakarta","category":"custom","name":"Operator Guard","trigger":"door.forced_open","severity":"warning","condition_expression":"event.type == \"door.forced_open\"","enabled":true}}`),
		},
	}

	for _, tc := range cases {
		recorder := referenceAPIRequest(t, router, tc.method, tc.path, token, tc.body)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected operator %s status 403, got %d body=%s", tc.name, recorder.Code, recorder.Body.String())
		}
	}
}
