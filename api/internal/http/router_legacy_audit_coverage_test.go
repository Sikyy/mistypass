package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestLegacyUserAndGroupWritesAppendAuditLogs(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "legacy-user-group-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// createUser
	createUserBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"Audit Test User","email":"audit.test@example.test","role":"resident","status":"active"}`)
	userRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users", token, createUserBody)
	if userRecorder.Code != http.StatusCreated {
		t.Fatalf("expected user create status 201, got %d body=%s", userRecorder.Code, userRecorder.Body.String())
	}
	var createdUser struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(userRecorder.Body.Bytes(), &createdUser); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "legacy_user_created", "user_id="+createdUser.ID, "email=audit.test@example.test", "role=resident")

	// createUserGroup
	createGroupBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"Audit Test Group","description":"test","members":[]}`)
	groupRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/user-groups", token, createGroupBody)
	if groupRecorder.Code != http.StatusCreated {
		t.Fatalf("expected user group create status 201, got %d body=%s", groupRecorder.Code, groupRecorder.Body.String())
	}
	var createdGroup struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(groupRecorder.Body.Bytes(), &createdGroup); err != nil {
		t.Fatalf("decode created user group: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "legacy_user_group_created", "group_id="+createdGroup.ID, "name=Audit Test Group")

	// updateUserGroup
	updateGroupBody := []byte(`{"building_id":"building_demo_001","name":"Audit Test Group Updated","description":"updated","members":[]}`)
	updateGroupRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/user-groups/"+createdGroup.ID+"?tenant_id=tenant_demo_jakarta", token, updateGroupBody)
	if updateGroupRecorder.Code != http.StatusOK {
		t.Fatalf("expected user group update status 200, got %d body=%s", updateGroupRecorder.Code, updateGroupRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "legacy_user_group_updated", "group_id="+createdGroup.ID, "name=Audit Test Group Updated")
}

func TestLegacyFloorAndAreaWritesAppendAuditLogs(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "legacy-floor-area-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// createFloor
	createFloorBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","name":"Audit Floor"}`)
	floorRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/floors", token, createFloorBody)
	if floorRecorder.Code != http.StatusCreated {
		t.Fatalf("expected floor create status 201, got %d body=%s", floorRecorder.Code, floorRecorder.Body.String())
	}
	var createdFloor struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(floorRecorder.Body.Bytes(), &createdFloor); err != nil {
		t.Fatalf("decode created floor: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "legacy_floor_created", "floor_id="+createdFloor.ID, "building_id=building_demo_001", "name=Audit Floor")

	// updateFloor
	updateFloorBody := []byte(`{"name":"Audit Floor Updated"}`)
	updateFloorRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/floors/"+createdFloor.ID+"?tenant_id=tenant_demo_jakarta", token, updateFloorBody)
	if updateFloorRecorder.Code != http.StatusOK {
		t.Fatalf("expected floor update status 200, got %d body=%s", updateFloorRecorder.Code, updateFloorRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "legacy_floor_updated", "floor_id="+createdFloor.ID, "name=Audit Floor Updated")

	// createArea
	createAreaBody := []byte(`{"tenant_id":"tenant_demo_jakarta","building_id":"building_demo_001","floor_id":"` + createdFloor.ID + `","name":"Audit Area"}`)
	areaRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/areas", token, createAreaBody)
	if areaRecorder.Code != http.StatusCreated {
		t.Fatalf("expected area create status 201, got %d body=%s", areaRecorder.Code, areaRecorder.Body.String())
	}
	var createdArea struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(areaRecorder.Body.Bytes(), &createdArea); err != nil {
		t.Fatalf("decode created area: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "legacy_area_created", "area_id="+createdArea.ID, "floor_id="+createdFloor.ID, "name=Audit Area")

	// updateArea
	updateAreaBody := []byte(`{"name":"Audit Area Updated"}`)
	updateAreaRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/areas/"+createdArea.ID+"?tenant_id=tenant_demo_jakarta", token, updateAreaBody)
	if updateAreaRecorder.Code != http.StatusOK {
		t.Fatalf("expected area update status 200, got %d body=%s", updateAreaRecorder.Code, updateAreaRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "legacy_area_updated", "area_id="+createdArea.ID, "name=Audit Area Updated")

	// deleteFloor
	deleteFloorRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/floors/"+createdFloor.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteFloorRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected floor delete status 204, got %d body=%s", deleteFloorRecorder.Code, deleteFloorRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "legacy_floor_deleted", "floor_id="+createdFloor.ID, "name=Audit Floor Updated")
}

func TestLegacyAlarmStatusUpdateAppendsAuditLog(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "legacy-alarm-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// list alarms to get an alarm ID
	listRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alarms?tenant_id=tenant_demo_jakarta", token, nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected alarm list status 200, got %d", listRecorder.Code)
	}
	var alarmList struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &alarmList); err != nil {
		t.Fatalf("decode alarm list: %v", err)
	}
	if len(alarmList.Items) == 0 {
		t.Skip("no demo alarms available to test status update audit")
	}

	alarmID := alarmList.Items[0].ID
	updateBody := []byte(`{"status":"acknowledged"}`)
	updateRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/alarms/"+alarmID+"/status?tenant_id=tenant_demo_jakarta", token, updateBody)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected alarm status update status 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "legacy_alarm_status_updated", "alarm_id="+alarmID, "status=acknowledged")
}

func TestWalletPassIssueAppendsAuditLog(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "wallet-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// issue a single pass
	issueBody := []byte(`{"tenant_id":"tenant_demo_jakarta","template_id":"wpt_employee_demo","target_type":"user","target_id":"user_demo_001","actor":"audit-test"}`)
	issueRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/wallet/passes/issue", token, issueBody)
	if issueRecorder.Code != http.StatusCreated {
		t.Fatalf("expected wallet pass issue status 201, got %d body=%s", issueRecorder.Code, issueRecorder.Body.String())
	}
	var issuedPass struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(issueRecorder.Body.Bytes(), &issuedPass); err != nil {
		t.Fatalf("decode issued pass: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "wallet_pass_issued", "pass_id="+issuedPass.ID, "template_id=wpt_employee_demo", "target_type=user")

	// suspend the pass
	suspendRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/wallet/passes/"+issuedPass.ID+"/suspend?tenant_id=tenant_demo_jakarta", token, nil)
	if suspendRecorder.Code != http.StatusOK {
		t.Fatalf("expected wallet pass suspend status 200, got %d body=%s", suspendRecorder.Code, suspendRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "wallet_pass_status_changed", "pass_id="+issuedPass.ID, "status=suspended")
}

func TestWalletPhysicalCardInventoryCreateAppendsAuditLog(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "wallet-phys-audit-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createBody := []byte(`{"tenant_id":"tenant_demo_jakarta","card_number":"AUDIT-CARD-001","uid":"04:AA:BB:CC:DD:EE:FF","actor":"audit-test"}`)
	createRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/wallet/physical-card-inventory", token, createBody)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected inventory create status 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created inventory: %v", err)
	}
	assertReferenceAuditLog(t, router, token, "physical_card_inventory_created", "inventory_id="+created.ID, "card_number=AUDIT-CARD-001")
}
