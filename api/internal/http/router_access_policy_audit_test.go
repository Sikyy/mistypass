package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestLegacyAccessPolicyWritesAppendAuditLogs(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "legacy-access-policy-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createBody := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Legacy office access","scope_type":"building","building_id":"building_demo_001","schedule":"weekdays","members":3,"status":"draft"}`)
	createRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/access-policies", token, createBody)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected access policy create status 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created access policy: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created access policy id, body=%s", createRecorder.Body.String())
	}

	updateBody := []byte(`{"name":"Legacy office access active","scope_type":"building","building_id":"building_demo_001","schedule":"weekdays","members":4,"status":"active"}`)
	updateRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/access-policies/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, updateBody)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected access policy update status 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}

	assertReferenceAuditLog(t, router, token, "legacy_access_policy_created", "policy_id="+created.ID, "name=Legacy office access", "scope_type=building", "status=draft", "members=3")
	assertReferenceAuditLog(t, router, token, "legacy_access_policy_updated", "policy_id="+created.ID, "name=Legacy office access active", "scope_type=building", "status=active", "members=4")
}

func TestLegacySpaceAndTemporaryAccessWritesAppendAuditLogs(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "legacy-space-access-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createBuildingBody := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Legacy Audit Place","address":"Audit Street 1","region":"ID-JK"}`)
	buildingRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/buildings", token, createBuildingBody)
	if buildingRecorder.Code != http.StatusCreated {
		t.Fatalf("expected building create status 201, got %d body=%s", buildingRecorder.Code, buildingRecorder.Body.String())
	}
	var createdBuilding struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(buildingRecorder.Body.Bytes(), &createdBuilding); err != nil {
		t.Fatalf("decode created building: %v", err)
	}
	if createdBuilding.ID == "" {
		t.Fatalf("expected created building id, body=%s", buildingRecorder.Body.String())
	}

	createDoorBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","floor_id":"floor_demo_001","area_id":"area_demo_001","name":"Legacy Audit Door","gateway_id":"gw_demo_001","kind":"office","status":"online"}`)
	doorRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/doors", token, createDoorBody)
	if doorRecorder.Code != http.StatusCreated {
		t.Fatalf("expected door create status 201, got %d body=%s", doorRecorder.Code, doorRecorder.Body.String())
	}
	var createdDoor struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(doorRecorder.Body.Bytes(), &createdDoor); err != nil {
		t.Fatalf("decode created door: %v", err)
	}
	if createdDoor.ID == "" {
		t.Fatalf("expected created door id, body=%s", doorRecorder.Body.String())
	}

	createDoorGroupBody := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Legacy Audit Door Group","door_ids":["` + createdDoor.ID + `"]}`)
	doorGroupRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/door-groups", token, createDoorGroupBody)
	if doorGroupRecorder.Code != http.StatusCreated {
		t.Fatalf("expected door group create status 201, got %d body=%s", doorGroupRecorder.Code, doorGroupRecorder.Body.String())
	}
	var createdDoorGroup struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(doorGroupRecorder.Body.Bytes(), &createdDoorGroup); err != nil {
		t.Fatalf("decode created door group: %v", err)
	}
	if createdDoorGroup.ID == "" {
		t.Fatalf("expected created door group id, body=%s", doorGroupRecorder.Body.String())
	}

	createTemporaryAccessBody := []byte(`{"tenant_id":"tenant_demo_jakarta","scope_type":"door","building_id":"building_demo_001","area_id":"area_demo_001","door_id":"` + createdDoor.ID + `","delivery_method":"email_qr","grantee_name":"Legacy Guest","grantee_phone":"+628110000999","grantee_email":"legacy.guest@example.test","valid_until":"2099-05-01T10:00:00Z"}`)
	temporaryAccessRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/temporary-access", token, createTemporaryAccessBody)
	if temporaryAccessRecorder.Code != http.StatusCreated {
		t.Fatalf("expected temporary access create status 201, got %d body=%s", temporaryAccessRecorder.Code, temporaryAccessRecorder.Body.String())
	}
	var createdTemporaryAccess struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(temporaryAccessRecorder.Body.Bytes(), &createdTemporaryAccess); err != nil {
		t.Fatalf("decode created temporary access: %v", err)
	}
	if createdTemporaryAccess.ID == "" {
		t.Fatalf("expected created temporary access id, body=%s", temporaryAccessRecorder.Body.String())
	}

	assertReferenceAuditLog(t, router, token, "legacy_building_created", "building_id="+createdBuilding.ID, "name=Legacy Audit Place", "region=ID-JK")
	assertReferenceAuditLog(t, router, token, "legacy_door_created", "door_id="+createdDoor.ID, "building_id=building_demo_001", "name=Legacy Audit Door", "gateway_id=gw_demo_001")
	assertReferenceAuditLog(t, router, token, "legacy_door_group_created", "door_group_id="+createdDoorGroup.ID, "name=Legacy Audit Door Group", "door_ids="+createdDoor.ID)
	assertReferenceAuditLog(t, router, token, "legacy_temporary_access_created", "temporary_access_id="+createdTemporaryAccess.ID, "scope_type=door", "door_id="+createdDoor.ID, "grantee_email=legacy.guest@example.test")
}
