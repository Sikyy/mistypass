package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

type incidentAlertTestStore struct {
	items map[string][]byte
}

func (s *incidentAlertTestStore) Load(key string, dst any) (bool, error) {
	payload, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return false, err
	}
	return true, nil
}

func (s *incidentAlertTestStore) Save(key string, value any) error {
	if s.items == nil {
		s.items = make(map[string][]byte)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.items[key] = payload
	return nil
}

const roleAssignmentPolicyID = "ap_incident_role_assignment_tenant_demo_jakarta"

func incidentAlertPolicyList(t *testing.T, router http.Handler, token string) []referenceAlertPolicy {
	t.Helper()
	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alert_policies?tenant_id=tenant_demo_jakarta", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected alert policies 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Items []referenceAlertPolicy `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode alert policies: %v", err)
	}
	return resp.Items
}

func findAlertPolicy(items []referenceAlertPolicy, id string) (referenceAlertPolicy, bool) {
	for _, p := range items {
		if p.ID == id {
			return p, true
		}
	}
	return referenceAlertPolicy{}, false
}

func createDemoRoleAssignment(t *testing.T, router http.Handler, token string) string {
	t.Helper()
	body := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_001","assignee_type":"User","assignee_id":"usr_1001","assignee_email":"andri.pratama@mistypass.local"}}`)
	rec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/role_assignments", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected role assignment 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode role assignment: %v", err)
	}
	return created.ID
}

func roleAssignmentNotifications(t *testing.T, router http.Handler, token string) []alertNotification {
	t.Helper()
	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/alert_policies/notifications?tenant_id=tenant_demo_jakarta&policy_id="+roleAssignmentPolicyID, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected notifications 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Items []alertNotification `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode notifications: %v", err)
	}
	return resp.Items
}

func enableRoleAssignmentPolicy(t *testing.T, router http.Handler, token string) {
	t.Helper()
	body := []byte(`{"alert_policy":{"tenant_id":"tenant_demo_jakarta","enabled":true}}`)
	rec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/alert_policies/"+roleAssignmentPolicyID, token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected enable 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIncidentAlertPolicyRoleAssignmentListed(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "incident-list-secret", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	items := incidentAlertPolicyList(t, router, token)
	policy, ok := findAlertPolicy(items, roleAssignmentPolicyID)
	if !ok {
		t.Fatalf("expected role_assignment incident policy in catalog")
	}
	if policy.Trigger != "role_assignment" || policy.Category != "incident" || policy.Enabled {
		t.Fatalf("expected disabled role_assignment incident policy, got %+v", policy)
	}
	if _, ok := findAlertPolicy(items, "ap_incident_door_held_open_tenant_demo_jakarta"); !ok {
		t.Fatalf("expected door_held_open incident policy still listed")
	}
}

func TestIncidentAlertPolicyToggleAndPersist(t *testing.T) {
	store := &incidentAlertTestStore{}
	router, _, err := NewRouter(config.Config{JWTSecret: "incident-toggle-secret", EnableDemoUsers: true}, store)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	enableRoleAssignmentPolicy(t, router, token)

	policy, ok := findAlertPolicy(incidentAlertPolicyList(t, router, token), roleAssignmentPolicyID)
	if !ok || !policy.Enabled {
		t.Fatalf("expected role_assignment policy enabled after PATCH, got %+v ok=%v", policy, ok)
	}

	restored, _, err := NewRouter(config.Config{JWTSecret: "incident-toggle-secret", EnableDemoUsers: true}, store)
	if err != nil {
		t.Fatalf("restored router: %v", err)
	}
	rToken := referenceAPILogin(t, restored, "organization.admin@mistypass.local")
	rp, ok := findAlertPolicy(incidentAlertPolicyList(t, restored, rToken), roleAssignmentPolicyID)
	if !ok || !rp.Enabled {
		t.Fatalf("expected enabled policy to survive restart, got %+v ok=%v", rp, ok)
	}
}

func TestRoleAssignmentAlertSilentWhenDisabled(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "incident-silent-secret", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	_ = createDemoRoleAssignment(t, router, token)
	if n := roleAssignmentNotifications(t, router, token); len(n) != 0 {
		t.Fatalf("expected no notifications when policy disabled, got %d", len(n))
	}
}

func TestRoleAssignmentAlertFiresWhenEnabled(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "incident-fire-secret", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	enableRoleAssignmentPolicy(t, router, token)
	assignmentID := createDemoRoleAssignment(t, router, token)

	notifs := roleAssignmentNotifications(t, router, token)
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	n := notifs[0]
	if n.Trigger != "role_assignment" || n.EventID != assignmentID || n.EventType != "role_assignment.created" {
		t.Fatalf("unexpected notification: %+v (assignment=%s)", n, assignmentID)
	}
}

func TestRoleAssignmentAlertFiresOnUpdate(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "incident-update-secret", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	enableRoleAssignmentPolicy(t, router, token)
	assignmentID := createDemoRoleAssignment(t, router, token)

	updateBody := []byte(`{"role_assignment":{"tenant_id":"tenant_demo_jakarta","role_id":"role_place_admin","applies_to_type":"Place","applies_to_id":"building_demo_001","assignee_type":"User","assignee_id":"usr_1001","assignee_email":"andri.pratama@mistypass.local","valid_until":"2027-01-01T00:00:00Z"}}`)
	rec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/role_assignments/"+assignmentID, token, updateBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var sawUpdate bool
	for _, n := range roleAssignmentNotifications(t, router, token) {
		if n.EventType == "role_assignment.updated" && n.EventID == assignmentID {
			sawUpdate = true
		}
	}
	if !sawUpdate {
		t.Fatalf("expected a role_assignment.updated notification")
	}
}

func TestIncidentAlertPolicyDoorHeldOpenToggleable(t *testing.T) {
	router, _, err := NewRouter(config.Config{JWTSecret: "incident-door-secret", EnableDemoUsers: true}, nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")
	const doorPolicyID = "ap_incident_door_held_open_tenant_demo_jakarta"
	body := []byte(`{"alert_policy":{"tenant_id":"tenant_demo_jakarta","enabled":true}}`)
	rec := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/alert_policies/"+doorPolicyID, token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected door_held_open enable 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	p, ok := findAlertPolicy(incidentAlertPolicyList(t, router, token), doorPolicyID)
	if !ok || !p.Enabled {
		t.Fatalf("expected door_held_open toggleable+enabled, got %+v ok=%v", p, ok)
	}
}
